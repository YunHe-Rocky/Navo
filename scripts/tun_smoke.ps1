param(
    [string]$PackageRoot = (Join-Path $PSScriptRoot "..\release\Navo"),
    [int]$StartupTimeoutSeconds = 20,
    [switch]$Elevated
)

$ErrorActionPreference = "Stop"
$PackageRoot = [System.IO.Path]::GetFullPath($PackageRoot)
$Executable = Join-Path $PackageRoot "navo.exe"

function Test-IsAdministrator {
    $Identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $Principal = [Security.Principal.WindowsPrincipal]::new($Identity)
    return $Principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

# The launcher and the IPC client must run at the same integrity level.
# Elevating only the launcher makes the acceptance client fail on the pipe ACL.
if ($Elevated -and -not (Test-IsAdministrator)) {
    $SmokeLogRoot = Join-Path ([System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot ".."))) ".cache\tun-smoke"
    New-Item -ItemType Directory -Force $SmokeLogRoot | Out-Null
    $FailurePath = Join-Path $SmokeLogRoot "elevated.failure.log"
    Remove-Item -LiteralPath $FailurePath -Force -ErrorAction SilentlyContinue
    $Arguments = @(
        "-NoProfile",
        "-ExecutionPolicy", "Bypass",
        "-File", "`"$PSCommandPath`"",
        "-PackageRoot", "`"$PackageRoot`"",
        "-StartupTimeoutSeconds", $StartupTimeoutSeconds,
        "-Elevated"
    )
    $ElevatedTest = Start-Process `
        -FilePath "$env:SystemRoot\System32\WindowsPowerShell\v1.0\powershell.exe" `
        -ArgumentList $Arguments `
        -Verb RunAs `
        -WindowStyle Hidden `
        -Wait `
        -PassThru
    if (Test-Path -LiteralPath $FailurePath) { Get-Content -LiteralPath $FailurePath | Write-Error }
    exit $ElevatedTest.ExitCode
}

if (-not (Test-Path -LiteralPath $Executable -PathType Leaf)) {
    throw "Packaged launcher not found: $Executable"
}
$ProjectRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot ".."))
$LocalAppData = Join-Path $ProjectRoot ".cache\tun-smoke\localappdata"
$FailurePath = Join-Path $ProjectRoot ".cache\tun-smoke\elevated.failure.log"
New-Item -ItemType Directory -Force $LocalAppData | Out-Null

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
        ".", "Navo.UI.Agent.v1",
        [System.IO.Pipes.PipeDirection]::InOut,
        [System.IO.Pipes.PipeOptions]::Asynchronous
    )
    try {
        $Pipe.Connect(5000)
        $Request = @{
            request_id = "tun-smoke-$([DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds())"
            type = "REQUEST"
            method = $Method
        }
        foreach ($Entry in $Payload.GetEnumerator()) { $Request[$Entry.Key] = $Entry.Value }
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
        $ResponseBody = Read-Exact -Stream $Pipe -Length $ResponseLength
        return ([System.Text.Encoding]::UTF8.GetString($ResponseBody) | ConvertFrom-Json)
    }
    finally {
        $Pipe.Dispose()
    }
}

$Launcher = $null
$LauncherStdout = $null
$LauncherStderr = $null
$InitialNavoPids = @(Get-Process -Name "navo", "navo_app" -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Id)
if ($InitialNavoPids.Count -gt 0) {
    throw "Refusing to run while Navo is already active: $($InitialNavoPids -join ', ')"
}

try {
    $StartInfo = [System.Diagnostics.ProcessStartInfo]::new()
    $StartInfo.FileName = $Executable
    $StartInfo.WorkingDirectory = $PackageRoot
    $StartInfo.UseShellExecute = $false
    $StartInfo.CreateNoWindow = $true
    $StartInfo.RedirectStandardOutput = $true
    $StartInfo.RedirectStandardError = $true
    $PreviousLocalAppData = $env:LOCALAPPDATA
    $env:LOCALAPPDATA = $LocalAppData
    try {
        $Launcher = [System.Diagnostics.Process]::Start($StartInfo)
        $LauncherStdout = $Launcher.StandardOutput.ReadToEndAsync()
        $LauncherStderr = $Launcher.StandardError.ReadToEndAsync()
    }
    finally {
        $env:LOCALAPPDATA = $PreviousLocalAppData
    }

    $Deadline = [DateTime]::UtcNow.AddSeconds($StartupTimeoutSeconds)
    do {
        if ($Launcher.HasExited) { throw "Launcher exited with code $($Launcher.ExitCode)" }
        try {
            $CoreBefore = Invoke-NavoIPC -Method "core.status"
            break
        }
        catch {
            Start-Sleep -Milliseconds 250
        }
    } while ([DateTime]::UtcNow -lt $Deadline)
    if ($null -eq $CoreBefore) { throw "Navo IPC did not become ready" }

    $ProxyBefore = Invoke-NavoIPC -Method "proxy.status"
    $TUNBefore = Invoke-NavoIPC -Method "tun.status"

    $OriginalCoreID = $CoreBefore.payload.core_id
    $RestoreMode = if ($TUNBefore.payload.enabled) { "tun" } elseif ($ProxyBefore.payload.enabled) { "system_proxy" } else { "off" }

    # Startup recovery is asynchronous; do not race a previous capture journal.
    $RecoveryDeadline = [DateTime]::UtcNow.AddSeconds($StartupTimeoutSeconds)
    do {
        $Snapshot = Invoke-NavoIPC -Method "dashboard.snapshot"
        $CaptureState = [string]$Snapshot.payload.capture.state
        if ($CaptureState -notin @("starting_system_proxy", "starting_tun", "stopping", "recovering")) { break }
        Start-Sleep -Milliseconds 250
    } while ([DateTime]::UtcNow -lt $RecoveryDeadline)
    if ($CaptureState -in @("starting_system_proxy", "starting_tun", "stopping", "recovering")) {
        throw "Capture recovery did not settle: $CaptureState"
    }

    $CoreList = Invoke-NavoIPC -Method "core.list"
    if ($CoreList.type -ne "RESPONSE") {
        throw "Core detection failed: $($CoreList | ConvertTo-Json -Compress -Depth 10)"
    }
    $CoreResults = @()
    foreach ($Core in @($CoreList.payload.cores | Where-Object { $_.installed })) {
        $OffBeforeSelect = Invoke-NavoIPC -Method "capture.set" -Payload @{ mode = "off" }
        if ($OffBeforeSelect.type -ne "RESPONSE") { throw "Failed to reset capture before selecting $($Core.id)" }

        $SelectResult = Invoke-NavoIPC -Method "core.select" -Payload @{ core_id = $Core.id }
        if ($SelectResult.type -ne "RESPONSE") {
            throw "Core selection failed for $($Core.id): $($SelectResult | ConvertTo-Json -Compress -Depth 10)"
        }
        $SelectedStatus = Invoke-NavoIPC -Method "core.status"
        if ($SelectedStatus.payload.core_id -ne $Core.id) {
            throw "Selected core mismatch: expected $($Core.id), got $($SelectedStatus.payload.core_id)"
        }

        $SystemProxyStatus = $null
        if ($Core.system_proxy_supported -ne $false) {
            $SystemProxyResult = Invoke-NavoIPC -Method "capture.set" -Payload @{ mode = "system_proxy" }
            if ($SystemProxyResult.type -ne "RESPONSE") {
                throw "System proxy activation failed for $($Core.id): $($SystemProxyResult | ConvertTo-Json -Compress -Depth 10)"
            }
            $SystemProxyStatus = Invoke-NavoIPC -Method "proxy.status"
            $SystemProxyCore = Invoke-NavoIPC -Method "core.status"
            if (-not $SystemProxyStatus.payload.enabled -or $SystemProxyCore.payload.state -ne "running") {
                throw "System proxy did not activate WinINet and core for $($Core.id)"
            }
            $SystemProxyStop = Invoke-NavoIPC -Method "capture.set" -Payload @{ mode = "off" }
            if ($SystemProxyStop.type -ne "RESPONSE") { throw "System proxy rollback failed for $($Core.id)" }
        }

        $TUNResult = Invoke-NavoIPC -Method "capture.set" -Payload @{ mode = "tun" }
        $TUNStatus = Invoke-NavoIPC -Method "tun.status"
        $DataPlaneStatus = $null
        if ($Core.tun_supported) {
            if ($TUNResult.type -ne "RESPONSE") {
                throw "TUN activation failed for $($Core.id): $($TUNResult | ConvertTo-Json -Compress -Depth 10)"
            }
            $TUNCore = Invoke-NavoIPC -Method "core.status"
            if (-not $TUNStatus.payload.enabled -or $TUNCore.payload.state -ne "running") {
                throw "TUN did not activate adapter and core for $($Core.id)"
            }
            $DataPlane = Invoke-WebRequest -Uri "http://www.msftconnecttest.com/connecttest.txt" -UseBasicParsing -TimeoutSec 15
            $DataPlaneStatus = $DataPlane.StatusCode
            if ($DataPlaneStatus -lt 200 -or $DataPlaneStatus -ge 400) {
                throw "TUN data-plane probe returned HTTP $DataPlaneStatus for $($Core.id)"
            }
            if ([string]$DataPlane.Content -notmatch "Microsoft Connect Test") {
                throw "TUN data-plane probe returned unexpected content for $($Core.id)"
            }
        }
        else {
            if ($TUNResult.type -ne "ERROR") { throw "Unsupported core $($Core.id) unexpectedly accepted TUN" }
            if ($TUNStatus.payload.enabled) { throw "Unsupported core $($Core.id) mutated the TUN adapter" }
        }

        $OffAfterCore = Invoke-NavoIPC -Method "capture.set" -Payload @{ mode = "off" }
        if ($OffAfterCore.type -ne "RESPONSE") { throw "Capture rollback failed after testing $($Core.id)" }
        $CoreResults += [pscustomobject]@{
            id = $Core.id
            version = $Core.version
            system_proxy_supported = $Core.system_proxy_supported
            system_proxy_enabled = $SystemProxyStatus.payload.enabled
            tun_supported = $Core.tun_supported
            tun_response = $TUNResult.type
            tun_enabled = $TUNStatus.payload.enabled
            data_plane_status = $DataPlaneStatus
        }
    }

    $RestoreCore = Invoke-NavoIPC -Method "core.select" -Payload @{ core_id = $OriginalCoreID }
    if ($RestoreCore.type -ne "RESPONSE") { throw "Original core restore failed: $OriginalCoreID" }
    $RestoreResult = Invoke-NavoIPC -Method "capture.set" -Payload @{ mode = $RestoreMode }
    if ($RestoreResult.type -ne "RESPONSE") {
        throw "Capture restore failed: $($RestoreResult | ConvertTo-Json -Compress -Depth 10)"
    }
    $JournalPath = Join-Path $LocalAppData "Navo\tun_network_journal.json"
    if (Test-Path -LiteralPath $JournalPath) {
        throw "TUN transaction journal remains after clean rollback: $JournalPath"
    }

    [pscustomobject]@{
        original_core = $OriginalCoreID
        proxy_before = $ProxyBefore.payload
        tun_before = $TUNBefore.payload
        detected_cores = $CoreList.payload.cores
        core_results = $CoreResults
        restore_mode = $RestoreMode
        restore = $RestoreResult
    } | ConvertTo-Json -Depth 10
}
catch {
    $Failure = $_ | Out-String
    if ($null -ne $Launcher -and $Launcher.HasExited) {
        $Failure += "`nLauncher exit code: $($Launcher.ExitCode)"
    }
    if ($null -ne $LauncherStdout -and $LauncherStdout.IsCompleted) {
        $Failure += "`nLauncher stdout:`n$($LauncherStdout.Result)"
    }
    if ($null -ne $LauncherStderr -and $LauncherStderr.IsCompleted) {
        $Failure += "`nLauncher stderr:`n$($LauncherStderr.Result)"
    }
    $Failure | Set-Content -LiteralPath $FailurePath -Encoding UTF8
    throw
}
finally {
    try { $null = Invoke-NavoIPC -Method "service.shutdown" } catch {}
    if ($null -ne $Launcher -and -not $Launcher.HasExited) {
        $Launcher.WaitForExit(15000) | Out-Null
    }
}
