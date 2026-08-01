param(
    [string]$PackageRoot = "",
    [int]$StartupTimeoutSeconds = 20
)

$ErrorActionPreference = "Stop"
Add-Type -AssemblyName System.Net.Http
$ProjectRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot ".."))
if ([string]::IsNullOrWhiteSpace($PackageRoot)) {
    $PackageRoot = Join-Path $ProjectRoot "release\Navo"
}
$PackageRoot = [System.IO.Path]::GetFullPath($PackageRoot)
$ExpectedReleaseRoot = [System.IO.Path]::GetFullPath((Join-Path $ProjectRoot "release"))
if (-not $PackageRoot.StartsWith($ExpectedReleaseRoot, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "Smoke package must stay under $ExpectedReleaseRoot"
}

$Executable = Join-Path $PackageRoot "navo.exe"
$UIExecutable = Join-Path $PackageRoot "app_ui\navo_app.exe"
$SingBoxExecutable = Join-Path $PackageRoot "third_party\sing-box\sing-box.exe"
$MihomoExecutable = Join-Path $PackageRoot "third_party\mihomo\mihomo.exe"
$XrayExecutable = Join-Path $PackageRoot "third_party\xray\xray.exe"
foreach ($RequiredFile in @(
    $Executable,
    $UIExecutable,
    $SingBoxExecutable,
    $MihomoExecutable,
    $XrayExecutable
)) {
    if (-not (Test-Path -LiteralPath $RequiredFile -PathType Leaf)) {
        throw "Required package file is missing: $RequiredFile"
    }
}

$SmokeRoot = Join-Path $ProjectRoot ".cache\smoke"
$LocalAppData = Join-Path $SmokeRoot "localappdata"
New-Item -ItemType Directory -Force $LocalAppData | Out-Null
$BaselineChildPIDs = @(
    Get-Process -Name "navo_app", "sing-box", "mihomo", "xray" -ErrorAction SilentlyContinue |
        ForEach-Object Id
)
$ProbeListener = [System.Net.Sockets.TcpListener]::new(
    [System.Net.IPAddress]::Loopback,
    0
)
$ProbeListener.Start()
$ProbePort = ([System.Net.IPEndPoint]$ProbeListener.LocalEndpoint).Port
$ProbeListener.Stop()
$ProbeMarker = "navo-smoke-$([Guid]::NewGuid().ToString('N'))"
[System.IO.File]::WriteAllText((Join-Path $SmokeRoot "index.html"), $ProbeMarker)
$Python = (Get-Command python -ErrorAction Stop).Source
$ProbeStartInfo = [System.Diagnostics.ProcessStartInfo]::new()
$ProbeStartInfo.FileName = $Python
$ProbeStartInfo.Arguments = "-m http.server $ProbePort --bind 127.0.0.1 --directory `"$SmokeRoot`""
$ProbeStartInfo.UseShellExecute = $false
$ProbeStartInfo.CreateNoWindow = $true
$ProbeServer = [System.Diagnostics.Process]::Start($ProbeStartInfo)

function Read-Exact {
    param([System.IO.Stream]$Stream, [int]$Length)
    $Buffer = [byte[]]::new($Length)
    $Offset = 0
    while ($Offset -lt $Length) {
        $Read = $Stream.Read($Buffer, $Offset, $Length - $Offset)
        if ($Read -le 0) {
            throw "Named Pipe closed before a complete frame was read"
        }
        $Offset += $Read
    }
    return $Buffer
}

function Invoke-NavoIPC {
    param(
        [Parameter(Mandatory)][string]$Method,
        [hashtable]$Payload = @{}
    )
    $Pipe = [System.IO.Pipes.NamedPipeClientStream]::new(
        ".",
        "Navo.UI.Agent.v1",
        [System.IO.Pipes.PipeDirection]::InOut,
        [System.IO.Pipes.PipeOptions]::Asynchronous
    )
    try {
        $Pipe.Connect(5000)
        $Request = @{
            request_id = "smoke-$Method-$([DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds())"
            type = "REQUEST"
            method = $Method
        }
        foreach ($Entry in $Payload.GetEnumerator()) {
            $Request[$Entry.Key] = $Entry.Value
        }
        $Body = [System.Text.Encoding]::UTF8.GetBytes(($Request | ConvertTo-Json -Compress -Depth 10))
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
        if ($ResponseLength -eq 0 -or $ResponseLength -gt 10MB) {
            throw "Invalid Named Pipe response length: $ResponseLength"
        }
        $ResponseBody = Read-Exact -Stream $Pipe -Length $ResponseLength
        $Response = [System.Text.Encoding]::UTF8.GetString($ResponseBody) | ConvertFrom-Json
        if ($Response.type -eq "ERROR") {
            throw "$Method failed: $($Response.payload.code): $($Response.payload.message)"
        }
        return $Response
    }
    finally {
        $Pipe.Dispose()
    }
}

$Launcher = $null
try {
    $StartInfo = [System.Diagnostics.ProcessStartInfo]::new()
    $StartInfo.FileName = $Executable
    $StartInfo.WorkingDirectory = $PackageRoot
    $StartInfo.UseShellExecute = $true
    $StartInfo.Verb = "RunAs"
    $PreviousLocalAppData = $env:LOCALAPPDATA
    $env:LOCALAPPDATA = $LocalAppData
    try {
        $Launcher = [System.Diagnostics.Process]::Start($StartInfo)
    }
    finally {
        $env:LOCALAPPDATA = $PreviousLocalAppData
    }

    $Deadline = [DateTime]::UtcNow.AddSeconds($StartupTimeoutSeconds)
    do {
        if ($Launcher.HasExited) {
            throw "Packaged launcher exited during startup with code $($Launcher.ExitCode)"
        }
        try {
            $CoreStatus = Invoke-NavoIPC -Method "core.status"
            break
        }
        catch {
            if ([DateTime]::UtcNow -ge $Deadline) {
                throw
            }
            Start-Sleep -Milliseconds 250
        }
    } while ($true)

    $Results = [ordered]@{}
    foreach ($Method in @(
        "core.status",
        "core.list",
        "proxy.status",
        "tun.status",
        "subscription.list",
        "outbound.list",
        "runtime.status",
        "metrics.current",
        "tray.snapshot"
    )) {
        $Results[$Method] = (Invoke-NavoIPC -Method $Method).payload
    }

    if ($Results["core.status"].state -ne "running") {
        throw "Core is not running after startup"
    }
    if ($Results["tun.status"].enabled -eq $true) {
        throw "TUN unexpectedly enabled in isolated smoke state"
    }
    $TraySnapshot = $Results["tray.snapshot"]
    foreach ($RequiredTrayState in @(
        "core.status",
        "core.list",
        "runtime.status",
        "outbound.list",
        "tun.status",
        "proxy.status",
        "metrics.current",
        "subscription.list"
    )) {
        if ($null -eq $TraySnapshot.$RequiredTrayState) {
            throw "Tray snapshot is missing $RequiredTrayState"
        }
    }

    $RuntimeConfigPath = Join-Path $LocalAppData "Navo\runtime.json"
    $RuntimeConfig = Get-Content -LiteralPath $RuntimeConfigPath -Raw | ConvertFrom-Json
    $MixedInbound = $RuntimeConfig.inbounds |
        Where-Object type -EQ "mixed" |
        Select-Object -First 1
    if (-not $MixedInbound) {
        throw "Generated runtime config has no mixed inbound"
    }
    $ProxyURI = "http://127.0.0.1:$($MixedInbound.listen_port)"
    $HTTPHandler = [System.Net.Http.HttpClientHandler]::new()
    $WebProxy = [System.Net.WebProxy]::new($ProxyURI)
    $WebProxy.BypassProxyOnLocal = $false
    $HTTPHandler.Proxy = $WebProxy
    $HTTPHandler.UseProxy = $true
    $HTTPClient = [System.Net.Http.HttpClient]::new($HTTPHandler)
    $HTTPClient.Timeout = [TimeSpan]::FromSeconds(15)
    try {
        $CoreSwitchResults = [ordered]@{}
        foreach ($CoreID in @("sing-box", "mihomo", "xray", "sing-box")) {
            Invoke-NavoIPC -Method "core.select" -Payload @{ core_id = $CoreID } | Out-Null
            $SelectedCoreStatus = (Invoke-NavoIPC -Method "core.status").payload
            if ($SelectedCoreStatus.state -ne "running" -or $SelectedCoreStatus.core_id -ne $CoreID) {
                throw "Core switch to $CoreID did not reach running state"
            }
            $HTTPResponse = $HTTPClient.GetAsync(
                "http://127.0.0.1:$ProbePort/"
            ).GetAwaiter().GetResult()
            if (-not $HTTPResponse.IsSuccessStatusCode) {
                throw "$CoreID data-plane HTTP returned $([int]$HTTPResponse.StatusCode)"
            }
            $HTTPBody = $HTTPResponse.Content.ReadAsStringAsync().GetAwaiter().GetResult()
            if ($HTTPBody -notmatch [regex]::Escape($ProbeMarker)) {
                throw "$CoreID data-plane returned an unexpected response body"
            }
            $CoreSwitchResults[$CoreID] = @{
                state = $SelectedCoreStatus.state
                status_code = [int]$HTTPResponse.StatusCode
            }
        }
        $Results["core.switch"] = $CoreSwitchResults
    }
    finally {
        $HTTPClient.Dispose()
        $HTTPHandler.Dispose()
    }

    & (Join-Path $PSScriptRoot "exit_via_tray.ps1") -ProcessId $Launcher.Id

    $Results | ConvertTo-Json -Depth 8
}
finally {
    if ($Launcher -and -not $Launcher.HasExited) {
        $Launcher.Kill()
        $Launcher.WaitForExit()
    }
    if ($ProbeServer -and -not $ProbeServer.HasExited) {
        $ProbeServer.Kill()
        $ProbeServer.WaitForExit()
    }
    Start-Sleep -Milliseconds 500
    $Residual = Get-Process -Name "navo_app", "sing-box", "mihomo", "xray" -ErrorAction SilentlyContinue |
        Where-Object { $_.Id -notin $BaselineChildPIDs }
    if ($Residual) {
        throw "Residual child processes remain: $($Residual.Name -join ', ')"
    }
}
