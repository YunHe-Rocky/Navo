param(
    [ValidateSet("sing-box", "mihomo", "xray")]
    [string]$Core = "sing-box",
    [ValidateSet("none", "after-endpoint-bypass", "after-first-split-route", "after-second-split-route", "after-nrpt", "after-ipv6", "during-dataplane")]
    [string]$FailurePoint = "none",
    [ValidateSet("none", "after-endpoint-bypass", "after-first-split-route", "after-second-split-route", "after-nrpt", "after-ipv6", "during-dataplane")]
    [string]$CrashPoint = "none",
    [ValidateSet("golden", "repeat", "tun-proxy-tun", "tun-off-tun", "routing-modes", "system-proxy-routing-modes", "disable-adapter", "core-crash")]
    [string]$Scenario = "golden",
    [ValidateSet("bypass_mainland", "global", "direct")]
    [string]$RuntimeMode = "bypass_mainland",
    [string]$PackageRoot = (Join-Path $PSScriptRoot "..\release\Navo"),
    [string]$OutputDirectory = (Join-Path $PSScriptRoot "..\artifacts\tun-acceptance"),
    [string]$SourceProfile = "",
    [string]$UpstreamProtocol = "",
    [string]$UpstreamServer = "",
    [int]$UpstreamPort = 0,
    [switch]$AutoSelectReachableOutbound,
    [ValidateRange(0, 600)]
    [int]$StabilitySeconds = 0,
    [int]$StartupTimeoutSeconds = 30
)

$ErrorActionPreference = "Stop"
Add-Type -AssemblyName System.Net.Http
$ProjectRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot ".."))
$PackageRoot = [System.IO.Path]::GetFullPath($PackageRoot)
$OutputDirectory = [System.IO.Path]::GetFullPath($OutputDirectory)
$Executable = Join-Path $PackageRoot "navo.exe"
$RunID = [DateTimeOffset]::UtcNow.ToString("yyyyMMddTHHmmssfffZ")
$ProfileRoot = Join-Path $ProjectRoot ".cache\tun-acceptance\$Core-$FailurePoint-$CrashPoint-$RunID"
$LocalAppData = Join-Path $ProfileRoot "localappdata"
$ArtifactBase = Join-Path $OutputDirectory "$RunID-$Core-$FailurePoint-$CrashPoint-$Scenario"

New-Item -ItemType Directory -Force $OutputDirectory | Out-Null
trap {
    $Message = ($_ | Out-String).Trim()
    $Message | Set-Content -LiteralPath "$ArtifactBase.early-error.txt" -Encoding UTF8
    Write-Error $Message
    exit 1
}

function Copy-IsolatedProfile {
    param([Parameter(Mandatory)][string]$Source, [Parameter(Mandatory)][string]$Destination)
    $Source = [System.IO.Path]::GetFullPath($Source)
    if (-not (Test-Path -LiteralPath $Source -PathType Container)) {
        throw "Source profile does not exist: $Source"
    }
    $Target = Join-Path $Destination "Navo"
    New-Item -ItemType Directory -Force $Target | Out-Null
    foreach ($Name in @(
        "credentials.dpapi", "device-state.dat", "runtime_state.json",
        "subscriptions.json", "subscriptions.json.endpoints.dpapi", "upstream_proxies.json"
    )) {
        $Path = Join-Path $Source $Name
        if (Test-Path -LiteralPath $Path -PathType Leaf) {
            Copy-Item -LiteralPath $Path -Destination (Join-Path $Target $Name)
        }
    }
    $State = Join-Path $Source "state"
    if (Test-Path -LiteralPath $State -PathType Container) {
        Copy-Item -LiteralPath $State -Destination $Target -Recurse
    }
}

function Test-IsAdministrator {
    $Identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $Principal = [Security.Principal.WindowsPrincipal]::new($Identity)
    return $Principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Read-Exact {
    param([System.IO.Stream]$Stream, [int]$Length)
    $Buffer = [byte[]]::new($Length)
    $Offset = 0
    while ($Offset -lt $Length) {
        $Read = $Stream.Read($Buffer, $Offset, $Length - $Offset)
        if ($Read -le 0) { throw "Named Pipe closed before a complete response" }
        $Offset += $Read
    }
    return $Buffer
}

function Invoke-NavoIPC {
    param([Parameter(Mandatory)][string]$Method, [hashtable]$Payload = @{})
    $Pipe = [System.IO.Pipes.NamedPipeClientStream]::new(
        ".", "Navo.UI.Agent.v1", [System.IO.Pipes.PipeDirection]::InOut,
        [System.IO.Pipes.PipeOptions]::Asynchronous
    )
    try {
        $Pipe.Connect(5000)
        $Request = @{
            request_id = "tun-acceptance-$([DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds())"
            type = "REQUEST"
            method = $Method
        }
        foreach ($Entry in $Payload.GetEnumerator()) { $Request[$Entry.Key] = $Entry.Value }
        $Body = [System.Text.Encoding]::UTF8.GetBytes(($Request | ConvertTo-Json -Compress -Depth 12))
        $Header = [byte[]]::new(8)
        [BitConverter]::GetBytes([uint32]0x4E564F50).CopyTo($Header, 0)
        [BitConverter]::GetBytes([uint32]$Body.Length).CopyTo($Header, 4)
        $Pipe.Write($Header, 0, $Header.Length)
        $Pipe.Write($Body, 0, $Body.Length)
        $Pipe.Flush()
        $ResponseHeader = Read-Exact -Stream $Pipe -Length 8
        if ([BitConverter]::ToUInt32($ResponseHeader, 0) -ne 0x4E564F50) {
            throw "Invalid Named Pipe response magic"
        }
        $ResponseLength = [BitConverter]::ToUInt32($ResponseHeader, 4)
        if ($ResponseLength -le 0 -or $ResponseLength -gt 10485760) {
            throw "Invalid Named Pipe response length: $ResponseLength"
        }
        $ResponseBody = Read-Exact -Stream $Pipe -Length $ResponseLength
        return ([System.Text.Encoding]::UTF8.GetString($ResponseBody) | ConvertFrom-Json)
    }
    finally {
        $Pipe.Dispose()
    }
}

function Wait-NavoReady {
    param([System.Diagnostics.Process]$Process)
    $Deadline = [DateTime]::UtcNow.AddSeconds($StartupTimeoutSeconds)
    do {
        if ($Process.HasExited) { throw "Navo exited during startup with code $($Process.ExitCode)" }
        try {
            $Status = Invoke-NavoIPC -Method "core.status"
            if ($Status.type -eq "RESPONSE") { return $Status }
        }
        catch {}
        Start-Sleep -Milliseconds 250
    } while ([DateTime]::UtcNow -lt $Deadline)
    throw "Navo IPC did not become ready within $StartupTimeoutSeconds seconds"
}

function Wait-CaptureIdle {
    $Deadline = [DateTime]::UtcNow.AddSeconds($StartupTimeoutSeconds)
    $BusyStates = @("starting_system_proxy", "starting_tun", "stopping", "recovering")
    do {
        try {
            $Status = Invoke-NavoIPC -Method "capture.status"
            if ($Status.type -eq "RESPONSE" -and [string]$Status.payload.state -notin $BusyStates) {
                return $Status
            }
        }
        catch {}
        Start-Sleep -Milliseconds 250
    } while ([DateTime]::UtcNow -lt $Deadline)
    throw "Capture transition did not become idle within $StartupTimeoutSeconds seconds"
}

function Wait-CaptureCommittedOff {
    $Deadline = [DateTime]::UtcNow.AddSeconds(45)
    do {
        try {
            $Status = Invoke-NavoIPC -Method "capture.status"
            if ($Status.type -eq "RESPONSE" -and [string]$Status.payload.committed_mode -eq "off") {
                return $Status
            }
        }
        catch {}
        Start-Sleep -Milliseconds 500
    } while ([DateTime]::UtcNow -lt $Deadline)
    throw "Capture health recovery did not commit off within 45 seconds"
}

function Assert-TUNHealthy {
    param($Activation)
    if ($Activation.type -ne "RESPONSE") {
        throw "TUN activation failed: $($Activation | ConvertTo-Json -Compress -Depth 12)"
    }
    $Status = Invoke-NavoIPC -Method "tun.status"
    if (-not $Status.payload.enabled -or [string]$Status.payload.stage -ne "HEALTH_COMMITTED") {
        throw "TUN was reported before the verified health commit"
    }
    $Verification = $Status.payload.verification
    if (-not $Verification.dns -or -not $Verification.tcp -or -not $Verification.https -or [string]::IsNullOrWhiteSpace([string]$Verification.exit_ip)) {
        throw "Service data-plane verification is incomplete"
    }
    return $Status
}

function Start-Navo {
    param([string]$Failure, [string]$Crash)
    $Previous = @{
        LOCALAPPDATA = $env:LOCALAPPDATA
        NAVO_TUN_TEST_MODE = $env:NAVO_TUN_TEST_MODE
        NAVO_TUN_FAILURE_POINT = $env:NAVO_TUN_FAILURE_POINT
        NAVO_TUN_CRASH_POINT = $env:NAVO_TUN_CRASH_POINT
    }
    $env:LOCALAPPDATA = $LocalAppData
    $env:NAVO_TUN_TEST_MODE = "1"
    $env:NAVO_TUN_FAILURE_POINT = if ($Failure -eq "none") { "" } else { $Failure }
    $env:NAVO_TUN_CRASH_POINT = if ($Crash -eq "none") { "" } else { $Crash }
    try {
        $Info = [System.Diagnostics.ProcessStartInfo]::new()
        $Info.FileName = $Executable
        $Info.WorkingDirectory = $PackageRoot
        $Info.UseShellExecute = $false
        $Info.CreateNoWindow = $true
        return [System.Diagnostics.Process]::Start($Info)
    }
    finally {
        $env:LOCALAPPDATA = $Previous.LOCALAPPDATA
        $env:NAVO_TUN_TEST_MODE = $Previous.NAVO_TUN_TEST_MODE
        $env:NAVO_TUN_FAILURE_POINT = $Previous.NAVO_TUN_FAILURE_POINT
        $env:NAVO_TUN_CRASH_POINT = $Previous.NAVO_TUN_CRASH_POINT
    }
}

function Stop-Navo {
    param([System.Diagnostics.Process]$Process)
    if ($null -eq $Process -or $Process.HasExited) { return }
    try { & (Join-Path $PSScriptRoot "exit_via_tray.ps1") -ProcessId $Process.Id | Out-Null } catch {}
    if (-not $Process.WaitForExit(15000)) {
        $Process.Kill()
        $Process.WaitForExit(5000) | Out-Null
    }
}

function Get-OwnedNavoProcesses {
    $PackagePrefix = ([System.IO.Path]::GetFullPath($PackageRoot)).TrimEnd('\') + '\'
    return @(Get-CimInstance Win32_Process -Filter "Name='navo.exe' OR Name='navo_app.exe'" -ErrorAction SilentlyContinue |
        Where-Object {
            -not [string]::IsNullOrWhiteSpace([string]$_.ExecutablePath) -and
            ([System.IO.Path]::GetFullPath([string]$_.ExecutablePath)).StartsWith($PackagePrefix, [System.StringComparison]::OrdinalIgnoreCase)
        } |
        Sort-Object ProcessId |
        Select-Object ProcessId, Name, ExecutablePath)
}

function Get-DirectPublicIP {
    $Errors = @()
    foreach ($Uri in @("https://api4.ipify.org", "https://ipv4.icanhazip.com")) {
        for ($Attempt = 1; $Attempt -le 3; $Attempt++) {
            $Handler = [System.Net.Http.HttpClientHandler]::new()
            $Handler.UseProxy = $false
            $Client = [System.Net.Http.HttpClient]::new($Handler)
            $Client.Timeout = [TimeSpan]::FromSeconds(10)
            try {
                $Value = $Client.GetStringAsync($Uri).GetAwaiter().GetResult().Trim()
                $Parsed = $null
                if ([System.Net.IPAddress]::TryParse($Value, [ref]$Parsed)) { return $Value }
                throw "invalid response: $Value"
            }
            catch {
                $Errors += "$Uri attempt $Attempt`: $(($_ | Out-String).Trim())"
                if ($Attempt -lt 3) { Start-Sleep -Milliseconds 500 }
            }
            finally {
                $Client.Dispose()
                $Handler.Dispose()
            }
        }
    }
    throw "No-proxy public IP lookup failed: $($Errors -join ' | ')"
}

function Invoke-NoProxyHTTPProbe {
    $Handler = [System.Net.Http.HttpClientHandler]::new()
    $Handler.UseProxy = $false
    $Client = [System.Net.Http.HttpClient]::new($Handler)
    $Client.Timeout = [TimeSpan]::FromSeconds(15)
    try {
        $Response = $Client.GetAsync("https://www.cloudflare.com/cdn-cgi/trace").GetAwaiter().GetResult()
        $Body = $Response.Content.ReadAsStringAsync().GetAwaiter().GetResult()
        if (-not $Response.IsSuccessStatusCode -or [string]::IsNullOrWhiteSpace($Body)) {
            throw "No-proxy HTTPS probe failed with HTTP $([int]$Response.StatusCode)"
        }
        return [pscustomobject]@{ status = [int]$Response.StatusCode; bytes = $Body.Length }
    }
    finally {
        $Client.Dispose()
        $Handler.Dispose()
    }
}

function Invoke-NoProxyExternalSiteProbes {
    param([ValidateSet("bypass_mainland", "global", "direct")][string]$Mode = "bypass_mainland")
    $Results = [ordered]@{}
    $Probes = if ($Mode -eq "direct") { @(
        @{ Name = "baidu"; Uri = "https://www.baidu.com/"; Statuses = @(200) },
        @{ Name = "xiaomi"; Uri = "https://connect.rom.miui.com/generate_204"; Statuses = @(204) }
    ) } else { @(
        @{ Name = "google"; Uri = "https://www.google.com/generate_204"; Statuses = @(204) },
        @{ Name = "github"; Uri = "https://github.com/"; Statuses = @(200) },
        # Cloudflare may reject an unauthenticated automation client with 403;
        # both statuses prove TLS and the expected ChatGPT edge answered.
        @{ Name = "chatgpt"; Uri = "https://chatgpt.com/"; Statuses = @(200, 403) },
        @{ Name = "openai_api"; Uri = "https://api.openai.com/v1/models"; Statuses = @(401) }
    ) }
    foreach ($Probe in $Probes) {
        $LastError = $null
        for ($Attempt = 1; $Attempt -le 3; $Attempt++) {
            # Route churn invalidates pooled sockets; isolate every attempt from stale connections.
            $Handler = [System.Net.Http.HttpClientHandler]::new()
            $Handler.UseProxy = $false
            $Client = [System.Net.Http.HttpClient]::new($Handler)
            $Client.Timeout = [TimeSpan]::FromSeconds(15)
            $Response = $null
            try {
                $Response = $Client.GetAsync(
                    $Probe.Uri,
                    [System.Net.Http.HttpCompletionOption]::ResponseHeadersRead
                ).GetAwaiter().GetResult()
                $Status = [int]$Response.StatusCode
                if (@($Probe.Statuses) -notcontains $Status) {
                    throw "$($Probe.Name) returned HTTP $Status; expected $($Probe.Statuses -join ',')"
                }
                $Results[$Probe.Name] = [ordered]@{
                    uri = $Probe.Uri; status = $Status; attempts = $Attempt
                }
                $LastError = $null
                break
            }
            catch {
                $LastError = ($_ | Out-String).Trim()
                if ($Attempt -lt 3) { Start-Sleep -Seconds 1 }
            }
            finally {
                if ($null -ne $Response) { $Response.Dispose() }
                $Client.Dispose()
                $Handler.Dispose()
            }
        }
        if ($null -ne $LastError) {
            throw "$($Probe.Name) failed after 3 attempts: $LastError"
        }
    }
    return $Results
}

function Get-WinINetProxySnapshot {
    $Path = "HKCU:\Software\Microsoft\Windows\CurrentVersion\Internet Settings"
    $Properties = Get-ItemProperty -LiteralPath $Path -ErrorAction Stop
    return [ordered]@{
        proxy_enable = [int]$Properties.ProxyEnable
        proxy_server = [string]$Properties.ProxyServer
        proxy_override = [string]$Properties.ProxyOverride
        auto_config_url = [string]$Properties.AutoConfigURL
        auto_detect = if ($null -eq $Properties.AutoDetect) { $null } else { [int]$Properties.AutoDetect }
    }
}

function Invoke-SystemProxyApplicationProbes {
	param([ValidateSet("bypass_mainland", "global", "direct")][string]$Mode)
    $Results = [ordered]@{}
    $Probes = if ($Mode -eq "direct") { @(
        @{ Name = "baidu"; Uri = "https://www.baidu.com/"; Statuses = @(200) },
        @{ Name = "xiaomi"; Uri = "https://connect.rom.miui.com/generate_204"; Statuses = @(204) }
    ) } else { @(
        @{ Name = "google"; Uri = "https://www.google.com/generate_204"; Statuses = @(204) },
        @{ Name = "github"; Uri = "https://github.com/"; Statuses = @(200) },
        # Cloudflare may reject an unauthenticated automation client with 403;
        # both statuses prove TLS and the expected ChatGPT edge answered.
        @{ Name = "chatgpt"; Uri = "https://chatgpt.com/"; Statuses = @(200, 403) },
        @{ Name = "openai_api"; Uri = "https://api.openai.com/v1/models"; Statuses = @(401) }
    ) }
    foreach ($Probe in $Probes) {
        $LastError = $null
        for ($Attempt = 1; $Attempt -le 3; $Attempt++) {
            # Proxy remains unset deliberately: HttpClient must consume WinINet's system proxy.
            $Handler = [System.Net.Http.HttpClientHandler]::new()
            $Handler.UseProxy = $true
            $Client = [System.Net.Http.HttpClient]::new($Handler)
            $Client.Timeout = [TimeSpan]::FromSeconds(15)
            $Response = $null
            try {
                $Response = $Client.GetAsync(
                    $Probe.Uri,
                    [System.Net.Http.HttpCompletionOption]::ResponseHeadersRead
                ).GetAwaiter().GetResult()
                $Status = [int]$Response.StatusCode
                if (@($Probe.Statuses) -notcontains $Status) {
                    throw "$($Probe.Name) returned HTTP $Status; expected $($Probe.Statuses -join ',')"
                }
                $Results[$Probe.Name] = [ordered]@{
                    uri = $Probe.Uri; status = $Status; attempts = $Attempt; proxy_source = "wininet"
                }
                $LastError = $null
                break
            }
            catch {
                $LastError = ($_ | Out-String).Trim()
                if ($Attempt -lt 3) { Start-Sleep -Seconds 1 }
            }
            finally {
                if ($null -ne $Response) { $Response.Dispose() }
                $Client.Dispose()
                $Handler.Dispose()
            }
        }
        if ($null -ne $LastError) {
            throw "$($Probe.Name) failed through the WinINet system proxy after 3 attempts: $LastError"
        }
    }
    return $Results
}

function Get-SystemProxyPublicIP {
    $Handler = [System.Net.Http.HttpClientHandler]::new()
    $Handler.UseProxy = $true
    $Client = [System.Net.Http.HttpClient]::new($Handler)
    $Client.Timeout = [TimeSpan]::FromSeconds(15)
    try {
        $Errors = @()
        foreach ($Uri in @("https://api4.ipify.org", "https://ipv4.icanhazip.com")) {
            for ($Attempt = 1; $Attempt -le 3; $Attempt++) {
                try {
                    $Value = $Client.GetStringAsync($Uri).GetAwaiter().GetResult().Trim()
                    $Parsed = $null
                    if ([System.Net.IPAddress]::TryParse($Value, [ref]$Parsed)) { return $Value }
                    throw "invalid response: $Value"
                }
                catch {
                    $Errors += "$Uri attempt $Attempt`: $(($_ | Out-String).Trim())"
                    if ($Attempt -lt 3) { Start-Sleep -Milliseconds 500 }
                }
            }
        }
        throw "System proxy public IP lookup failed: $($Errors -join ' | ')"
    }
    finally {
        $Client.Dispose()
        $Handler.Dispose()
    }
}

function Get-NetworkSnapshot {
    $Routes = @(Get-NetRoute -PolicyStore ActiveStore -ErrorAction SilentlyContinue |
        Where-Object { $_.DestinationPrefix -in @("0.0.0.0/0", "0.0.0.0/1", "128.0.0.0/1", "::/0") } |
        Sort-Object DestinationPrefix, InterfaceIndex, NextHop, RouteMetric |
        Select-Object DestinationPrefix, InterfaceIndex, InterfaceAlias, NextHop, RouteMetric, Protocol)
    $NRPT = @(Get-DnsClientNrptRule -ErrorAction SilentlyContinue |
        Sort-Object Namespace, Comment |
        Select-Object Namespace, NameServers, Comment)
    $Firewall = @(Get-NetFirewallRule -ErrorAction SilentlyContinue |
        Where-Object { [string]$_.DisplayName -like "Navo TUN IPv6 Block *" } |
        Sort-Object DisplayName |
        Select-Object DisplayName, Enabled, Direction, Action)
    $Adapters = @(Get-NetAdapter -IncludeHidden -ErrorAction SilentlyContinue |
        Where-Object { [string]$_.Name -eq "Navo" } |
        Select-Object Name, ifIndex, InterfaceGuid, Status)
    $DNS = @(Get-DnsClientServerAddress -AddressFamily IPv4 -ErrorAction SilentlyContinue |
        Where-Object { [string]$_.InterfaceAlias -ne "Navo" } |
        Sort-Object InterfaceIndex |
        Select-Object InterfaceIndex, InterfaceAlias, ServerAddresses)
    $IPv6Bindings = @(Get-NetAdapterBinding -ComponentID ms_tcpip6 -ErrorAction SilentlyContinue |
        Where-Object { [string]$_.Name -ne "Navo" } |
        Sort-Object Name |
        Select-Object Name, InterfaceDescription, Enabled)
    return [ordered]@{
        routes = $Routes
        nrpt = $NRPT
        firewall = $Firewall
        adapters = $Adapters
        dns = $DNS
        ipv6_bindings = $IPv6Bindings
    }
}

function Get-HostEnvironment {
    $AdapterEnvironment = @(Get-NetAdapter -IncludeHidden -ErrorAction SilentlyContinue |
        Sort-Object ifIndex |
        Select-Object Name, ifIndex, Status, InterfaceDescription, HardwareInterface)
    $Features = [ordered]@{}
    foreach ($Feature in @("Microsoft-Hyper-V-All", "Microsoft-Windows-Subsystem-Linux")) {
        try {
            $Features[$Feature] = [string](Get-WindowsOptionalFeature -Online -FeatureName $Feature -ErrorAction Stop).State
        }
        catch {
            $Features[$Feature] = "unavailable"
        }
    }
    return [ordered]@{ adapters = $AdapterEnvironment; optional_features = $Features }
}

function ConvertTo-CanonicalJSON {
    param($Value)
    return ($Value | ConvertTo-Json -Compress -Depth 12)
}

function Assert-Restored {
    param($Before, $After)
    foreach ($Key in @("routes", "nrpt", "firewall", "adapters", "dns", "ipv6_bindings")) {
        if ((ConvertTo-CanonicalJSON $Before[$Key]) -ne (ConvertTo-CanonicalJSON $After[$Key])) {
            throw "Network rollback mismatch for $Key"
        }
    }
    $Journals = @(Get-ChildItem -Recurse -File $LocalAppData -Filter "tun_network_journal.json" -ErrorAction SilentlyContinue)
    if ($Journals.Count -gt 0) { throw "TUN journal remains after rollback: $($Journals.FullName -join ', ')" }
}

$SystemProxyOnly = $Scenario -eq "system-proxy-routing-modes"
if ($Core -eq "xray" -and -not $SystemProxyOnly) {
    throw "Xray supports System Proxy only; elevated TUN scenarios require sing-box or Mihomo"
}
if (-not $SystemProxyOnly -and -not (Test-IsAdministrator)) { throw "Administrator privileges are required" }
if ($SystemProxyOnly -and ($FailurePoint -ne "none" -or $CrashPoint -ne "none")) {
    throw "System Proxy-only acceptance does not support TUN failure injection"
}
if ($FailurePoint -ne "none" -and $CrashPoint -ne "none") { throw "FailurePoint and CrashPoint are mutually exclusive" }
if ($UpstreamProtocol -notin @("", "http", "https", "socks5")) { throw "Unsupported upstream protocol: $UpstreamProtocol" }
$HasGeneratedUpstream = -not [string]::IsNullOrWhiteSpace($UpstreamServer) -or $UpstreamPort -ne 0 -or -not [string]::IsNullOrWhiteSpace($UpstreamProtocol)
if ($HasGeneratedUpstream -and ([string]::IsNullOrWhiteSpace($UpstreamServer) -or $UpstreamPort -le 0 -or [string]::IsNullOrWhiteSpace($UpstreamProtocol))) {
    throw "UpstreamProtocol, UpstreamServer and UpstreamPort must be provided together"
}
if (-not (Test-Path -LiteralPath $Executable -PathType Leaf)) { throw "Packaged launcher not found: $Executable" }

$ManifestPath = Join-Path $PackageRoot "CORE_MANIFEST.json"
$Manifest = Get-Content -Raw -LiteralPath $ManifestPath | ConvertFrom-Json
$CoreEntry = $Manifest.cores | Where-Object { $_.type -eq $Core } | Select-Object -First 1
if ($null -eq $CoreEntry) { throw "Core is absent from manifest: $Core" }
$CorePath = Join-Path $PackageRoot ($CoreEntry.relative_path -replace '/', '\')
$WintunPath = Join-Path $PackageRoot "third_party\sing-box\wintun.dll"
foreach ($Path in @($CorePath, $WintunPath)) {
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) { throw "Required artifact missing: $Path" }
}
$CoreHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $CorePath).Hash.ToLowerInvariant()
if ($CoreHash -ne [string]$CoreEntry.sha256) { throw "Core hash mismatch for $Core" }
$WintunHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $WintunPath).Hash.ToLowerInvariant()
if ($WintunHash -ne "e5da8447dc2c320edc0fc52fa01885c103de8c118481f683643cacc3220dafce") {
    throw "Wintun hash mismatch"
}

$Existing = @(Get-Process -Name "navo", "navo_app" -ErrorAction SilentlyContinue)
if ($Existing.Count -gt 0) { throw "Refusing to run while Navo is active: $($Existing.Id -join ', ')" }
New-Item -ItemType Directory -Force $LocalAppData, $OutputDirectory | Out-Null
if (-not [string]::IsNullOrWhiteSpace($SourceProfile)) {
    Copy-IsolatedProfile -Source $SourceProfile -Destination $LocalAppData
}
$Before = if ($SystemProxyOnly) { $null } else { Get-NetworkSnapshot }
$WinINetBefore = Get-WinINetProxySnapshot
$HostEnvironment = Get-HostEnvironment
$DirectIPBefore = try { Get-DirectPublicIP } catch { $null }
$Process = $null
$RecoveryProcess = $null
$Result = [ordered]@{
    executed_at = [DateTimeOffset]::UtcNow.ToString("o")
    windows = [Environment]::OSVersion.VersionString
    core = $Core
    core_version = $CoreEntry.version
    core_sha256 = $CoreHash
    wintun_sha256 = $WintunHash
    failure_point = $FailurePoint
    crash_point = $CrashPoint
    scenario = $Scenario
    runtime_mode = $RuntimeMode
    profile_mode = if ($HasGeneratedUpstream) { "generated-upstream" } elseif ([string]::IsNullOrWhiteSpace($SourceProfile)) { "fresh-direct" } else { "isolated-clone" }
    upstream = if ($HasGeneratedUpstream) { "$($UpstreamProtocol)://$($UpstreamServer):$UpstreamPort" } else { $null }
    selected_outbound = $null
    direct_ip_before = $DirectIPBefore
    direct_ip_after = $null
    host_environment = $HostEnvironment
    status = "failed"
    activation = $null
    tun_status = $null
    no_proxy_https = $null
    external_sites = $null
    stability_seconds = $StabilitySeconds
    tun_status_delayed = $null
    external_sites_delayed = $null
    reenabled_tun_status = $null
    external_sites_reenabled = $null
    routing_modes = $null
    routing_rules = $null
    wininet_before = $WinINetBefore
    wininet_owned = $null
    wininet_after = $null
    capture_after = $null
    core_log = $null
    outbound_selection_attempts = @()
    rollback = "not_checked"
    before = $Before
    after = $null
    error = $null
}

try {
    $Process = Start-Navo -Failure $FailurePoint -Crash $CrashPoint
    Wait-NavoReady -Process $Process | Out-Null
    Wait-CaptureIdle | Out-Null
    $Off = Invoke-NavoIPC -Method "capture.set" -Payload @{ mode = "off" }
    if ($Off.type -ne "RESPONSE") { throw "Initial capture reset failed: $($Off | ConvertTo-Json -Compress -Depth 10)" }
    $Selected = Invoke-NavoIPC -Method "core.select" -Payload @{ core_id = $Core }
    if ($Selected.type -ne "RESPONSE") { throw "Core selection failed: $($Selected | ConvertTo-Json -Compress -Depth 10)" }
    if ($HasGeneratedUpstream) {
        $Created = Invoke-NavoIPC -Method "outbound.create" -Payload @{
            name = "TUN acceptance upstream"
            proto = $UpstreamProtocol
            server = $UpstreamServer
            port = $UpstreamPort
            udp_policy = "disabled"
        }
        if ($Created.type -ne "RESPONSE" -or [string]::IsNullOrWhiteSpace([string]$Created.payload.id)) {
            throw "Generated upstream creation failed: $($Created | ConvertTo-Json -Compress -Depth 10)"
        }
        $Routes = Invoke-NavoIPC -Method "outbound.list"
        if ($Routes.type -ne "RESPONSE" -or [string]$Routes.payload.active_id -ne [string]$Created.payload.id) {
            throw "Generated upstream was not committed as active"
        }
    }
    elseif ($AutoSelectReachableOutbound) {
        $Routes = Invoke-NavoIPC -Method "outbound.list"
        $Tests = Invoke-NavoIPC -Method "outbound.testAll"
        if ($Routes.type -ne "RESPONSE" -or $Tests.type -ne "RESPONSE") {
            throw "Outbound discovery failed"
        }
        $RouteByID = @{}
        foreach ($Route in $Routes.payload.outbounds) {
            $RouteByID[[string]$Route.id] = $Route
        }
        $ActiveID = [string]$Routes.payload.active_id
        $Candidates = @($Tests.payload.results | Where-Object reachable | Sort-Object @{
            Expression = { if ([string]$_.id -eq $ActiveID) { 0 } else { 1 } }
        }, latency_ms)
        if ($Candidates.Count -eq 0) {
            throw "No reachable outbound is available in the isolated profile"
        }
        $Chosen = $null
        foreach ($Candidate in $Candidates) {
            $CandidateID = [string]$Candidate.id
            $Selected = Invoke-NavoIPC -Method "outbound.select" -Payload @{ id = $CandidateID }
            if ($Selected.type -ne "RESPONSE" -or [string]$Selected.payload.active_id -ne $CandidateID) {
                $Result.outbound_selection_attempts += [ordered]@{ id = $CandidateID; latency_ms = [int]$Candidate.latency_ms; status = "select_failed" }
                continue
            }
            $PreflightMode = Invoke-NavoIPC -Method "runtime.mode.set" -Payload @{ mode = "global" }
            if ($PreflightMode.type -ne "RESPONSE") {
                throw "Outbound preflight could not select global mode"
            }
            $Preflight = Invoke-NavoIPC -Method "capture.set" -Payload @{ mode = "system_proxy" }
            if ($Preflight.type -eq "RESPONSE") {
                $PreflightStop = Invoke-NavoIPC -Method "capture.set" -Payload @{ mode = "off" }
                if ($PreflightStop.type -ne "RESPONSE") {
                    throw "Outbound preflight could not disable System Proxy"
                }
                $PreflightWinINet = Get-WinINetProxySnapshot
                if ((ConvertTo-CanonicalJSON $WinINetBefore) -ne (ConvertTo-CanonicalJSON $PreflightWinINet)) {
                    throw "Outbound preflight did not restore WinINet"
                }
                $Result.outbound_selection_attempts += [ordered]@{ id = $CandidateID; latency_ms = [int]$Candidate.latency_ms; status = "dataplane_passed" }
                $Chosen = $Candidate
                break
            }
            try { Invoke-NavoIPC -Method "capture.set" -Payload @{ mode = "off" } | Out-Null } catch {}
            $Result.outbound_selection_attempts += [ordered]@{
                id = $CandidateID
                latency_ms = [int]$Candidate.latency_ms
                status = "dataplane_failed"
                error_code = [string]$Preflight.payload.code
            }
        }
        if ($null -eq $Chosen) {
            throw "No outbound passed the real System Proxy data-plane preflight"
        }
        $ChosenRoute = $RouteByID[[string]$Chosen.id]
        $Result.selected_outbound = [ordered]@{
            id = [string]$Chosen.id
            name = [string]$ChosenRoute.name
            source_type = [string]$ChosenRoute.source_type
            latency_ms = [int]$Chosen.latency_ms
        }
    }
    $ModeSet = Invoke-NavoIPC -Method "runtime.mode.set" -Payload @{ mode = $RuntimeMode }
    if ($ModeSet.type -ne "RESPONSE" -or [string]$ModeSet.payload.mode -ne $RuntimeMode) {
        throw "Runtime mode selection failed: $($ModeSet | ConvertTo-Json -Compress -Depth 10)"
    }

    if ($SystemProxyOnly) {
        $Result.routing_modes = [ordered]@{ system_proxy = [ordered]@{} }
        $Result.activation = Invoke-NavoIPC -Method "capture.set" -Payload @{ mode = "system_proxy" }
        if ($Result.activation.type -ne "RESPONSE") {
            throw "System Proxy activation failed: $($Result.activation | ConvertTo-Json -Compress -Depth 12)"
        }
        $Result.wininet_owned = Get-WinINetProxySnapshot
        if ($Result.wininet_owned.proxy_enable -ne 1 -or
            $Result.wininet_owned.proxy_server -notmatch '^127\.0\.0\.1:\d+$' -or
            -not [string]::IsNullOrWhiteSpace($Result.wininet_owned.auto_config_url) -or
            $Result.wininet_owned.auto_detect -notin @($null, 0)) {
            throw "System proxy did not acquire exclusive WinINet ownership: $(ConvertTo-CanonicalJSON $Result.wininet_owned)"
        }
        $Result.routing_rules = Invoke-NavoIPC -Method "runtime.rules.set" -Payload @{
            blacklist = @("google.com", "github.com", "chatgpt.com", "openai.com", "api4.ipify.org", "ipv4.icanhazip.com", "203.0.113.0/24")
            whitelist = @("baidu.com", "qq.com", "api4.ipify.org", "ipv4.icanhazip.com", "198.51.100.0/24")
        }
        if ($Result.routing_rules.type -ne "RESPONSE" -or -not $Result.routing_rules.payload.verified) {
            throw "Customer routing rules failed to apply"
        }
        $ListModeOff = Invoke-NavoIPC -Method "runtime.list_mode.set" -Payload @{ mode = "off" }
        if ($ListModeOff.type -ne "RESPONSE" -or [string]$ListModeOff.payload.mode -ne "off") {
            throw "Routing list mode did not remain explicitly disabled"
        }
        foreach ($Mode in @("bypass_mainland", "global", "direct")) {
            $SetMode = Invoke-NavoIPC -Method "runtime.mode.set" -Payload @{ mode = $Mode }
            $ModeResult = [ordered]@{ set = $SetMode; status = $null; external_sites = $null; application_sites = $null; exit_ip = $null }
            $Result.routing_modes.system_proxy[$Mode] = $ModeResult
            if ($SetMode.type -ne "RESPONSE" -or [string]$SetMode.payload.mode -ne $Mode -or -not $SetMode.payload.verified) {
                throw "System proxy routing mode $Mode failed to apply"
            }
            Start-Sleep -Seconds 1
            $ModeResult.status = Invoke-NavoIPC -Method "runtime.status"
            if ([string]$ModeResult.status.payload.list_mode -ne "off") {
                throw "System proxy route mode $Mode implicitly enabled a routing list"
            }
            $ModeResult.external_sites = $SetMode.payload.sites
            $ModeResult.application_sites = Invoke-SystemProxyApplicationProbes -Mode $Mode
            $ModeResult.exit_ip = Get-SystemProxyPublicIP
            if ($Mode -eq "direct" -and $ModeResult.exit_ip -ne $Result.direct_ip_before) {
                throw "System proxy direct mode changed the public IP: before=$($Result.direct_ip_before) after=$($ModeResult.exit_ip)"
            }
            if ($Mode -ne "direct" -and $ModeResult.exit_ip -eq $Result.direct_ip_before) {
                throw "System proxy $Mode mode did not use the selected outbound"
            }
        }
        $Result.routing_list_modes = [ordered]@{}
        foreach ($ListMode in @("blacklist", "whitelist", "off")) {
            if ($ListMode -eq "blacklist") {
                $null = Invoke-NavoIPC -Method "runtime.mode.set" -Payload @{ mode = "direct" }
            }
            elseif ($ListMode -eq "whitelist") {
                $null = Invoke-NavoIPC -Method "runtime.mode.set" -Payload @{ mode = "global" }
            }
            $SetListMode = Invoke-NavoIPC -Method "runtime.list_mode.set" -Payload @{ mode = $ListMode }
            $ListStatus = Invoke-NavoIPC -Method "runtime.status"
            $ListExitIP = Get-SystemProxyPublicIP
            $Result.routing_list_modes[$ListMode] = [ordered]@{ set = $SetListMode; status = $ListStatus; exit_ip = $ListExitIP }
            if ($SetListMode.type -ne "RESPONSE" -or [string]$ListStatus.payload.list_mode -ne $ListMode) {
                throw "Routing list mode $ListMode failed explicit activation"
            }
            if ($ListMode -eq "blacklist" -and $ListExitIP -eq $Result.direct_ip_before) {
                throw "Blacklist mode did not proxy a matching public-IP domain"
            }
            if ($ListMode -eq "whitelist" -and $ListExitIP -ne $Result.direct_ip_before) {
                throw "Whitelist mode did not directly route a matching public-IP domain"
            }
        }
        $Stop = Invoke-NavoIPC -Method "capture.set" -Payload @{ mode = "off" }
        if ($Stop.type -ne "RESPONSE") { throw "Final System Proxy disable failed" }
        $Result.capture_after = Invoke-NavoIPC -Method "capture.status"
        $Result.wininet_after = Get-WinINetProxySnapshot
        if ((ConvertTo-CanonicalJSON $Result.wininet_before) -ne (ConvertTo-CanonicalJSON $Result.wininet_after)) {
            throw "WinINet rollback mismatch"
        }
    }
    elseif ($CrashPoint -ne "none") {
        try {
            $Result.activation = Invoke-NavoIPC -Method "capture.set" -Payload @{ mode = "tun" }
            if ($Result.activation.type -eq "ERROR") {
                throw "TUN activation failed before crash point: $($Result.activation | ConvertTo-Json -Compress -Depth 10)"
            }
        } catch {
            if (-not $Process.HasExited) { throw }
        }
        if (-not $Process.WaitForExit(15000)) { throw "Crash point did not terminate Navo" }
        if ($Process.ExitCode -ne 91) { throw "Unexpected crash-test exit code: $($Process.ExitCode)" }
        $Process = $null
        $RecoveryProcess = Start-Navo -Failure "none" -Crash "none"
        Wait-NavoReady -Process $RecoveryProcess | Out-Null
        Wait-CaptureIdle | Out-Null
        $Recovered = Invoke-NavoIPC -Method "capture.set" -Payload @{ mode = "off" }
        if ($Recovered.type -ne "RESPONSE") { throw "Crash recovery failed: $($Recovered | ConvertTo-Json -Compress -Depth 10)" }
        $Result.capture_after = Invoke-NavoIPC -Method "capture.status"
    }
    else {
        $Result.activation = Invoke-NavoIPC -Method "capture.set" -Payload @{ mode = "tun" }
        $Result.tun_status = Invoke-NavoIPC -Method "tun.status"
        if ($FailurePoint -eq "none") {
            $Result.tun_status = Assert-TUNHealthy -Activation $Result.activation
            $Result.no_proxy_https = Invoke-NoProxyHTTPProbe
            $Result.external_sites = if ($Scenario -eq "routing-modes") {
                $Result.tun_status.payload.verification.sites
            } else {
                Invoke-NoProxyExternalSiteProbes
            }
            if ($StabilitySeconds -gt 0) {
                Start-Sleep -Seconds $StabilitySeconds
                $Result.tun_status_delayed = Assert-TUNHealthy -Activation $Result.activation
                $Result.external_sites_delayed = if ($Scenario -eq "routing-modes") {
                    $Result.tun_status_delayed.payload.verification.sites
                } else {
                    Invoke-NoProxyExternalSiteProbes
                }
            }
            switch ($Scenario) {
                "repeat" {
                    Assert-TUNHealthy -Activation (Invoke-NavoIPC -Method "capture.set" -Payload @{ mode = "tun" }) | Out-Null
                    $Stop = Invoke-NavoIPC -Method "capture.set" -Payload @{ mode = "off" }
                    if ($Stop.type -ne "RESPONSE") { throw "First TUN disable failed" }
                    $Stop = Invoke-NavoIPC -Method "capture.set" -Payload @{ mode = "off" }
                    if ($Stop.type -ne "RESPONSE") { throw "Repeated TUN disable failed" }
                }
                "tun-proxy-tun" {
                    $Proxy = Invoke-NavoIPC -Method "capture.set" -Payload @{ mode = "system_proxy" }
                    if ($Proxy.type -ne "RESPONSE") { throw "TUN to system proxy transition failed" }
                    Assert-TUNHealthy -Activation (Invoke-NavoIPC -Method "capture.set" -Payload @{ mode = "tun" }) | Out-Null
                    $Stop = Invoke-NavoIPC -Method "capture.set" -Payload @{ mode = "off" }
                    if ($Stop.type -ne "RESPONSE") { throw "Final TUN disable failed" }
                }
                "tun-off-tun" {
                    $Stop = Invoke-NavoIPC -Method "capture.set" -Payload @{ mode = "off" }
                    if ($Stop.type -ne "RESPONSE") { throw "First TUN disable failed" }
                    $Result.reenabled_tun_status = Assert-TUNHealthy -Activation (Invoke-NavoIPC -Method "capture.set" -Payload @{ mode = "tun" })
                    $Result.external_sites_reenabled = Invoke-NoProxyExternalSiteProbes
                    $Stop = Invoke-NavoIPC -Method "capture.set" -Payload @{ mode = "off" }
                    if ($Stop.type -ne "RESPONSE") { throw "Final TUN disable failed" }
                }
                "routing-modes" {
                    $Result.routing_modes = [ordered]@{ tun = [ordered]@{}; system_proxy = [ordered]@{} }
                    $Result.routing_rules = Invoke-NavoIPC -Method "runtime.rules.set" -Payload @{
                        blacklist = @("google.com", "github.com", "chatgpt.com", "openai.com", "api4.ipify.org", "ipv4.icanhazip.com", "203.0.113.0/24")
                        whitelist = @("baidu.com", "qq.com", "api4.ipify.org", "ipv4.icanhazip.com", "198.51.100.0/24")
                    }
                    if ($Result.routing_rules.type -ne "RESPONSE" -or -not $Result.routing_rules.payload.verified) {
                        throw "Customer routing rules failed to apply"
                    }
                    $ListModeOff = Invoke-NavoIPC -Method "runtime.list_mode.set" -Payload @{ mode = "off" }
                    if ($ListModeOff.type -ne "RESPONSE" -or [string]$ListModeOff.payload.mode -ne "off") {
                        throw "Routing list mode did not remain explicitly disabled"
                    }
                    foreach ($Mode in @("bypass_mainland", "global", "direct")) {
                        $SetMode = Invoke-NavoIPC -Method "runtime.mode.set" -Payload @{ mode = $Mode }
                        $ModeResult = [ordered]@{ set = $SetMode; status = $null; external_sites = $null; application_sites = $null; exit_ip = $null }
                        $Result.routing_modes.tun[$Mode] = $ModeResult
                        if ($SetMode.type -ne "RESPONSE" -or [string]$SetMode.payload.mode -ne $Mode -or -not $SetMode.payload.verified) {
                            throw "TUN routing mode $Mode failed to apply"
                        }
                        Start-Sleep -Seconds 1
                        $ModeResult.status = Invoke-NavoIPC -Method "runtime.status"
                        if ([string]$ModeResult.status.payload.list_mode -ne "off") {
                            throw "TUN activation or route mode $Mode implicitly enabled a routing list"
                        }
                        $ModeResult.external_sites = $SetMode.payload.sites
                        $ModeResult.application_sites = Invoke-NoProxyExternalSiteProbes -Mode $Mode
                        $ModeResult.exit_ip = Get-DirectPublicIP
                        if ($Mode -eq "direct" -and $ModeResult.exit_ip -ne $Result.direct_ip_before) {
                            throw "TUN direct mode changed the public IP: before=$($Result.direct_ip_before) after=$($ModeResult.exit_ip)"
                        }
                        if ($Mode -ne "direct" -and $ModeResult.exit_ip -eq $Result.direct_ip_before) {
                            throw "TUN $Mode mode did not use the selected outbound"
                        }
                    }
                    $Result.tun_routing_list_modes = [ordered]@{}
                    foreach ($ListMode in @("blacklist", "whitelist", "off")) {
                        if ($ListMode -eq "blacklist") {
                            $null = Invoke-NavoIPC -Method "runtime.mode.set" -Payload @{ mode = "direct" }
                        }
                        elseif ($ListMode -eq "whitelist") {
                            $null = Invoke-NavoIPC -Method "runtime.mode.set" -Payload @{ mode = "global" }
                        }
                        $SetListMode = Invoke-NavoIPC -Method "runtime.list_mode.set" -Payload @{ mode = $ListMode }
                        $ListStatus = Invoke-NavoIPC -Method "runtime.status"
                        $ListExitIP = Get-DirectPublicIP
                        $Result.tun_routing_list_modes[$ListMode] = [ordered]@{ set = $SetListMode; status = $ListStatus; exit_ip = $ListExitIP }
                        if ($SetListMode.type -ne "RESPONSE" -or [string]$ListStatus.payload.list_mode -ne $ListMode) {
                            throw "TUN routing list mode $ListMode failed explicit activation"
                        }
                        if ($ListMode -eq "blacklist" -and $ListExitIP -eq $Result.direct_ip_before) {
                            throw "TUN blacklist mode did not proxy a matching public-IP domain"
                        }
                        if ($ListMode -eq "whitelist" -and $ListExitIP -ne $Result.direct_ip_before) {
                            throw "TUN whitelist mode did not directly route a matching public-IP domain"
                        }
                    }
                    $Proxy = Invoke-NavoIPC -Method "capture.set" -Payload @{ mode = "system_proxy" }
                    if ($Proxy.type -ne "RESPONSE") { throw "TUN to system proxy transition failed" }
                    $ProxyStatus = Invoke-NavoIPC -Method "runtime.status"
                    if ([string]$ProxyStatus.payload.list_mode -ne "off") {
                        throw "System proxy activation implicitly enabled a routing list"
                    }
                    $Result.wininet_owned = Get-WinINetProxySnapshot
                    if ($Result.wininet_owned.proxy_enable -ne 1 -or
                        $Result.wininet_owned.proxy_server -notmatch '^127\.0\.0\.1:\d+$' -or
                        -not [string]::IsNullOrWhiteSpace($Result.wininet_owned.auto_config_url) -or
                        $Result.wininet_owned.auto_detect -notin @($null, 0)) {
                        throw "System proxy did not acquire exclusive WinINet ownership: $(ConvertTo-CanonicalJSON $Result.wininet_owned)"
                    }
                    foreach ($Mode in @("bypass_mainland", "global", "direct")) {
                        $SetMode = Invoke-NavoIPC -Method "runtime.mode.set" -Payload @{ mode = $Mode }
                        $ModeResult = [ordered]@{ set = $SetMode; status = $null; external_sites = $null; application_sites = $null; exit_ip = $null }
                        $Result.routing_modes.system_proxy[$Mode] = $ModeResult
                        if ($SetMode.type -ne "RESPONSE" -or [string]$SetMode.payload.mode -ne $Mode -or -not $SetMode.payload.verified) {
                            throw "System proxy routing mode $Mode failed to apply"
                        }
                        Start-Sleep -Seconds 1
                        $ModeResult.status = Invoke-NavoIPC -Method "runtime.status"
                        if ([string]$ModeResult.status.payload.list_mode -ne "off") {
                            throw "System proxy route mode $Mode implicitly enabled a routing list"
                        }
                        $ModeResult.external_sites = $SetMode.payload.sites
                        $ModeResult.application_sites = Invoke-SystemProxyApplicationProbes -Mode $Mode
                        $ModeResult.exit_ip = Get-SystemProxyPublicIP
                        if ($Mode -eq "direct" -and $ModeResult.exit_ip -ne $Result.direct_ip_before) {
                            throw "System proxy direct mode changed the public IP after TUN: before=$($Result.direct_ip_before) after=$($ModeResult.exit_ip)"
                        }
                        if ($Mode -ne "direct" -and $ModeResult.exit_ip -eq $Result.direct_ip_before) {
                            throw "System proxy $Mode mode did not use the selected outbound after TUN"
                        }
                    }
                    $Result.routing_list_modes = [ordered]@{}
                    foreach ($ListMode in @("blacklist", "whitelist", "off")) {
                        if ($ListMode -eq "blacklist") {
                            $null = Invoke-NavoIPC -Method "runtime.mode.set" -Payload @{ mode = "direct" }
                        }
                        elseif ($ListMode -eq "whitelist") {
                            $null = Invoke-NavoIPC -Method "runtime.mode.set" -Payload @{ mode = "global" }
                        }
                        $SetListMode = Invoke-NavoIPC -Method "runtime.list_mode.set" -Payload @{ mode = $ListMode }
                        $ListStatus = Invoke-NavoIPC -Method "runtime.status"
                        $ListExitIP = Get-SystemProxyPublicIP
                        $Result.routing_list_modes[$ListMode] = [ordered]@{ set = $SetListMode; status = $ListStatus; exit_ip = $ListExitIP }
                        if ($SetListMode.type -ne "RESPONSE" -or [string]$ListStatus.payload.list_mode -ne $ListMode) {
                            throw "Routing list mode $ListMode failed explicit activation"
                        }
                        if ($ListMode -eq "blacklist" -and $ListExitIP -eq $Result.direct_ip_before) {
                            throw "System proxy blacklist mode did not proxy a matching public-IP domain after TUN"
                        }
                        if ($ListMode -eq "whitelist" -and $ListExitIP -ne $Result.direct_ip_before) {
                            throw "System proxy whitelist mode did not directly route a matching public-IP domain after TUN"
                        }
                    }
                    $Stop = Invoke-NavoIPC -Method "capture.set" -Payload @{ mode = "off" }
                    if ($Stop.type -ne "RESPONSE") { throw "Final routing-mode disable failed" }
                    $Result.wininet_after = Get-WinINetProxySnapshot
                    if ((ConvertTo-CanonicalJSON $Result.wininet_before) -ne (ConvertTo-CanonicalJSON $Result.wininet_after)) {
                        throw "WinINet rollback mismatch"
                    }
                }
                "disable-adapter" {
                    $AdapterIndex = [int]$Result.tun_status.payload.interface_index
                    $Adapter = Get-NetAdapter -InterfaceIndex $AdapterIndex -ErrorAction Stop
                    $Adapter | Disable-NetAdapter -Confirm:$false -ErrorAction Stop
                    $Result.capture_after = Wait-CaptureCommittedOff
                }
                "core-crash" {
                    $CoreProcessName = $Core
                    $CoreProcess = Get-Process -Name $CoreProcessName -ErrorAction Stop |
                        Where-Object { [string]::Equals($_.Path, $CorePath, [StringComparison]::OrdinalIgnoreCase) } |
                        Select-Object -First 1
                    if ($null -eq $CoreProcess) {
                        throw "Navo-owned core process was not found at $CorePath"
                    }
                    Stop-Process -Id $CoreProcess.Id -Force -ErrorAction Stop
                    Start-Sleep -Seconds 12
                    $Capture = Invoke-NavoIPC -Method "capture.status"
                    if ([string]$Capture.payload.committed_mode -eq "tun") {
                        $Result.no_proxy_https = Invoke-NoProxyHTTPProbe
                        $Result.external_sites = Invoke-NoProxyExternalSiteProbes
                        $Stop = Invoke-NavoIPC -Method "capture.set" -Payload @{ mode = "off" }
                        if ($Stop.type -ne "RESPONSE") { throw "Core restart cleanup failed" }
                    }
                    else {
                        $Result.capture_after = Wait-CaptureCommittedOff
                    }
                }
                default {
                    $Stop = Invoke-NavoIPC -Method "capture.set" -Payload @{ mode = "off" }
                    if ($Stop.type -ne "RESPONSE") { throw "TUN disable failed: $($Stop | ConvertTo-Json -Compress -Depth 10)" }
                }
            }
        }
        else {
            if ($Result.activation.type -ne "ERROR") { throw "Injected failure unexpectedly returned success" }
            if ($Result.tun_status.payload.enabled) { throw "Injected failure left TUN running" }
        }
        if ($null -eq $Result.capture_after) {
            $Result.capture_after = Invoke-NavoIPC -Method "capture.status"
        }
    }

    Start-Sleep -Milliseconds 500
    $After = if ($SystemProxyOnly) { $null } else { Get-NetworkSnapshot }
    $Result.after = $After
    if (-not $SystemProxyOnly) { Assert-Restored -Before $Before -After $After }
    $Result.wininet_after = Get-WinINetProxySnapshot
    if ((ConvertTo-CanonicalJSON $Result.wininet_before) -ne (ConvertTo-CanonicalJSON $Result.wininet_after)) {
        throw "WinINet rollback mismatch"
    }
    $Result.direct_ip_after = try { Get-DirectPublicIP } catch { $null }
    $Result.rollback = "passed"
    $Result.status = "passed"
}
catch {
    $Result.error = ($_ | Out-String).Trim()
    try { $Result.core_log = (Invoke-NavoIPC -Method "core.log.tail").payload.lines } catch {}
    try {
        $Cleanup = Invoke-NavoIPC -Method "capture.set" -Payload @{ mode = "off" }
        if ($Cleanup.type -eq "RESPONSE") { Wait-CaptureIdle | Out-Null }
    }
    catch {}
    try {
        $Result.after = if ($SystemProxyOnly) { $null } else { Get-NetworkSnapshot }
        if (-not $SystemProxyOnly) { Assert-Restored -Before $Before -After $Result.after }
        $Result.wininet_after = Get-WinINetProxySnapshot
        if ((ConvertTo-CanonicalJSON $Result.wininet_before) -ne (ConvertTo-CanonicalJSON $Result.wininet_after)) {
            throw "WinINet rollback mismatch after failure"
        }
        $Result.rollback = "passed_after_failure"
    }
    catch {
        $Result.rollback = "failed"
        $Result.error += "`nRollback check: $(($_ | Out-String).Trim())"
    }
}
finally {
    Stop-Navo -Process $Process
    Stop-Navo -Process $RecoveryProcess
    $OwnedResidue = @(Get-OwnedNavoProcesses)
    $Result.process_residue = $OwnedResidue
    if ($OwnedResidue.Count -gt 0) {
        foreach ($OwnedProcess in $OwnedResidue) {
            Stop-Process -Id $OwnedProcess.ProcessId -Force -ErrorAction SilentlyContinue
        }
        Start-Sleep -Milliseconds 500
        $RemainingOwned = @(Get-OwnedNavoProcesses)
        $ResidueMessage = "Owned Navo process residue after shutdown: $($OwnedResidue.ProcessId -join ', ')"
        if ($RemainingOwned.Count -gt 0) {
            $ResidueMessage += "; cleanup failed for: $($RemainingOwned.ProcessId -join ', ')"
        }
        $Result.status = "failed"
        $Result.error = (([string]$Result.error).Trim() + "`n" + $ResidueMessage).Trim()
    }
}

$Result | ConvertTo-Json -Depth 15 | Set-Content -LiteralPath "$ArtifactBase.json" -Encoding UTF8
$Markdown = @"
# Navo TUN acceptance result

- Executed: $($Result.executed_at)
- Windows: $($Result.windows)
- Core: $($Result.core) $($Result.core_version)
- Failure point: $($Result.failure_point)
- Crash point: $($Result.crash_point)
- Direct IP before: $($Result.direct_ip_before)
- Direct IP after: $($Result.direct_ip_after)
- Result: $($Result.status)
- Rollback: $($Result.rollback)
- JSON evidence: $([System.IO.Path]::GetFileName("$ArtifactBase.json"))

## Error

$($Result.error)
"@
$Markdown | Set-Content -LiteralPath "$ArtifactBase.md" -Encoding UTF8
Write-Output ($Result | ConvertTo-Json -Depth 15)
if ($Result.status -ne "passed") { exit 1 }
