param(
    [int]$StartupTimeoutSeconds = 20,
    [switch]$SingleUI
)

$ErrorActionPreference = "Stop"
$ProjectRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot ".."))
$PackageRoot = Join-Path $ProjectRoot "release\Navo"
$TestLauncher = Join-Path $PackageRoot "navo-lightweight-smoke.exe"
$UIExecutable = [System.IO.Path]::GetFullPath((Join-Path $PackageRoot "app_ui\navo_app.exe"))
$SmokeRoot = Join-Path $ProjectRoot ".cache\lightweight-smoke"
$LocalAppData = Join-Path $SmokeRoot "localappdata"
$GoCache = Join-Path $ProjectRoot ".cache\go-build"
$GoPath = Join-Path $ProjectRoot ".cache\go-path"
$GoModCache = Join-Path $GoPath "pkg\mod"
$BaselinePackageProcessIDs = @(
    Get-Process -Name "navo_app", "sing-box", "mihomo", "xray" -ErrorAction SilentlyContinue |
        Where-Object {
            try {
                $_.Path.StartsWith($PackageRoot, [System.StringComparison]::OrdinalIgnoreCase)
            }
            catch {
                $false
            }
        } |
        ForEach-Object Id
)

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
            request_id = "lightweight-$Method-$([DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds())"
            type = "REQUEST"
            method = $Method
        }
        foreach ($Entry in $Payload.GetEnumerator()) {
            $Request[$Entry.Key] = $Entry.Value
        }
        $Body = [System.Text.Encoding]::UTF8.GetBytes(
            ($Request | ConvertTo-Json -Compress -Depth 10)
        )
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
        $Response = [System.Text.Encoding]::UTF8.GetString($ResponseBody) |
            ConvertFrom-Json
        if ($Response.type -eq "ERROR") {
            throw "$Method failed: $($Response.payload.code): $($Response.payload.message)"
        }
        return $Response
    }
    finally {
        $Pipe.Dispose()
    }
}

function Wait-Until {
    param(
        [Parameter(Mandatory)][scriptblock]$Condition,
        [Parameter(Mandatory)][string]$Failure,
        [int]$TimeoutSeconds = 10
    )
    $Deadline = [DateTime]::UtcNow.AddSeconds($TimeoutSeconds)
    do {
        if (& $Condition) {
            return
        }
        Start-Sleep -Milliseconds 100
    } while ([DateTime]::UtcNow -lt $Deadline)
    throw $Failure
}

function Get-TestUIProcesses {
    return @(
        Get-Process -Name "navo_app" -ErrorAction SilentlyContinue |
            Where-Object {
                try {
                    [System.IO.Path]::GetFullPath($_.Path) -eq $UIExecutable
                }
                catch {
                    $false
                }
            }
    )
}

foreach ($RequiredFile in @(
    (Join-Path $PackageRoot "navo.exe"),
    $UIExecutable,
    (Join-Path $PackageRoot "third_party\sing-box\sing-box.exe")
)) {
    if (-not (Test-Path -LiteralPath $RequiredFile -PathType Leaf)) {
        throw "Required package file is missing: $RequiredFile"
    }
}

New-Item -ItemType Directory -Force $LocalAppData, $GoCache, $GoModCache | Out-Null
$Go = (Get-Command go -ErrorAction Stop).Source
$PreviousGoCache = $env:GOCACHE
$PreviousGoPath = $env:GOPATH
$PreviousGoModCache = $env:GOMODCACHE
$PreviousGoOS = $env:GOOS
$PreviousGoArch = $env:GOARCH
$env:GOCACHE = $GoCache
$env:GOPATH = $GoPath
$env:GOMODCACHE = $GoModCache
$env:GOOS = "windows"
$env:GOARCH = "amd64"
try {
    & $Go build -trimpath -o $TestLauncher ./cmd/navo
    if ($LASTEXITCODE -ne 0) {
        throw "Build lightweight smoke launcher failed with exit code $LASTEXITCODE"
    }
}
finally {
    $env:GOCACHE = $PreviousGoCache
    $env:GOPATH = $PreviousGoPath
    $env:GOMODCACHE = $PreviousGoModCache
    $env:GOOS = $PreviousGoOS
    $env:GOARCH = $PreviousGoArch
}

$Launcher = $null
try {
    $StartInfo = [System.Diagnostics.ProcessStartInfo]::new()
    $StartInfo.FileName = $TestLauncher
    $StartInfo.Arguments = "-silent"
    $StartInfo.WorkingDirectory = $PackageRoot
    $StartInfo.UseShellExecute = $false
    $PreviousLocalAppData = $env:LOCALAPPDATA
    $PreviousEnvFile = $env:NAVO_ENV_FILE
    $env:LOCALAPPDATA = $LocalAppData
    Remove-Item Env:NAVO_ENV_FILE -ErrorAction SilentlyContinue
    try {
        $Launcher = [System.Diagnostics.Process]::Start($StartInfo)
    }
    finally {
        $env:LOCALAPPDATA = $PreviousLocalAppData
        if ($null -eq $PreviousEnvFile) {
            Remove-Item Env:NAVO_ENV_FILE -ErrorAction SilentlyContinue
        }
        else {
            $env:NAVO_ENV_FILE = $PreviousEnvFile
        }
    }

    Wait-Until -TimeoutSeconds $StartupTimeoutSeconds `
        -Failure "Tray-only launcher did not expose the UI pipe" `
        -Condition {
            if ($Launcher.HasExited) {
                throw "Tray-only launcher exited with code $($Launcher.ExitCode)"
            }
            try {
                Invoke-NavoIPC -Method "core.status" | Out-Null
                return $true
            }
            catch {
                return $false
            }
        }

    $InitialUIProcesses = Get-TestUIProcesses
    if ($InitialUIProcesses.Count -gt 1) {
        throw "Startup launched multiple Wails UI processes"
    }
    $TrayAvailableAtStart = $InitialUIProcesses.Count -eq 0

    $Stopwatch = [System.Diagnostics.Stopwatch]::StartNew()
    $Dashboard = (Invoke-NavoIPC -Method "dashboard.snapshot").payload
    $Stopwatch.Stop()
    if ($null -eq $Dashboard.core -or $null -eq $Dashboard.metrics -or $null -eq $Dashboard.ip) {
        throw "Dashboard snapshot is incomplete"
    }
    if ($Dashboard.core.state -ne "stopped") {
        throw "Tray-only startup unexpectedly started a core: $($Dashboard.core.state)"
    }
    if ($Dashboard.capture.committed_mode -ne "off") {
        throw "Tray-only startup did not remain disconnected: $($Dashboard.capture.committed_mode)"
    }
    if ($Stopwatch.Elapsed -gt [TimeSpan]::FromSeconds(3)) {
        throw "Dashboard snapshot blocked for $($Stopwatch.Elapsed.TotalMilliseconds) ms"
    }

    if ($TrayAvailableAtStart) {
        Invoke-NavoIPC -Method "ui.show" | Out-Null
        Wait-Until -Failure "UI did not start on demand" -Condition {
            (Get-TestUIProcesses).Count -eq 1
        }
    }
    $FirstUI = (Get-TestUIProcesses)[0]
    $TrayAvailable = $TrayAvailableAtStart
    $SecondUI = $null
    if ($SingleUI) {
        Wait-Until -Failure "UI process did not create a visible window" -Condition {
            $Process = Get-Process -Id $FirstUI.Id -ErrorAction SilentlyContinue
            return $null -ne $Process -and $Process.MainWindowHandle -ne 0
        }
        Start-Sleep -Seconds 2
        $StableUI = @(Get-TestUIProcesses)
        $StableProcess = Get-Process -Id $FirstUI.Id -ErrorAction SilentlyContinue
        if ($StableUI.Count -ne 1 -or $StableUI[0].Id -ne $FirstUI.Id) {
            throw "UI process was duplicated or replaced during the stability window"
        }
        if ($null -eq $StableProcess -or $StableProcess.MainWindowHandle -eq 0) {
            throw "UI window disappeared during the stability window"
        }
        & (Join-Path $PSScriptRoot "exit_via_tray.ps1") -ProcessId $Launcher.Id
    }
    else {
        Stop-Process -Id $FirstUI.Id -Force
        Wait-Until -Failure "UI process did not exit" -Condition {
            (Get-TestUIProcesses).Count -eq 0
        }
    }
    if (-not $SingleUI -and $TrayAvailable) {
        Invoke-NavoIPC -Method "core.status" | Out-Null
        Invoke-NavoIPC -Method "ui.show" | Out-Null
        Wait-Until -Failure "UI did not restart from tray-only state" -Condition {
            (Get-TestUIProcesses).Count -eq 1
        }
        $SecondUI = (Get-TestUIProcesses)[0]
        if ($SecondUI.Id -eq $FirstUI.Id) {
            throw "UI process was not recreated"
        }
        & (Join-Path $PSScriptRoot "exit_via_tray.ps1") -ProcessId $Launcher.Id
    }
    if (-not $Launcher.WaitForExit(10000)) {
        throw "Launcher did not exit cleanly"
    }
    [pscustomobject]@{
        tray_only_start = $TrayAvailableAtStart
        tray_available = $TrayAvailable
        dashboard_ms = [math]::Round($Stopwatch.Elapsed.TotalMilliseconds, 1)
        first_ui_pid = $FirstUI.Id
        second_ui_pid = if ($null -eq $SecondUI) { 0 } else { $SecondUI.Id }
        launcher_exit_code = $Launcher.ExitCode
    } | ConvertTo-Json
}
finally {
    if ($Launcher -and -not $Launcher.HasExited) {
        $Launcher.Kill()
        $Launcher.WaitForExit(5000) | Out-Null
    }
    foreach ($Process in Get-TestUIProcesses) {
        Stop-Process -Id $Process.Id -Force -ErrorAction SilentlyContinue
    }
    Get-Process -Name "sing-box", "mihomo", "xray" -ErrorAction SilentlyContinue |
        Where-Object {
            $_.Id -notin $BaselinePackageProcessIDs -and
            (
                [string]::IsNullOrEmpty($_.Path) -or
                $_.Path.StartsWith($PackageRoot, [System.StringComparison]::OrdinalIgnoreCase)
            )
        } |
        Stop-Process -Force -ErrorAction SilentlyContinue
    if (Test-Path -LiteralPath $TestLauncher -PathType Leaf) {
        Remove-Item -LiteralPath $TestLauncher -Force
    }
}
