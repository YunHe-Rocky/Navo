param(
    [string]$PackageRoot = (Join-Path $PSScriptRoot "..\release\Navo"),
    [string]$OutputDirectory = (Join-Path $PSScriptRoot "..\artifacts\tun-acceptance"),
    [string]$SourceProfile = "",
    [string]$UpstreamProtocol = "",
    [string]$UpstreamServer = "",
    [int]$UpstreamPort = 0,
    [switch]$AutoSelectReachableOutbound,
    [switch]$StopOnFailure,
    [ValidateRange(0, 600)]
    [int]$StabilitySeconds = 30,
    [ValidateSet("full", "golden", "failure", "crash", "lifecycle", "routing")]
    [string]$Suite = "full"
)

$ErrorActionPreference = "Stop"
$Runner = Join-Path $PSScriptRoot "test-tun-elevated.ps1"
$FailurePoints = @(
    "after-endpoint-bypass",
    "after-first-split-route",
    "after-second-split-route",
    "after-nrpt",
    "after-ipv6",
    "during-dataplane"
)
$ParsedUpstream = $null
$IsLoopbackUpstream = $UpstreamServer -eq "localhost" -or
    ([System.Net.IPAddress]::TryParse($UpstreamServer, [ref]$ParsedUpstream) -and [System.Net.IPAddress]::IsLoopback($ParsedUpstream))
if ($IsLoopbackUpstream) {
    # A loopback upstream stays inside the host, so no physical endpoint bypass route exists.
    $FailurePoints = @($FailurePoints | Where-Object { $_ -ne "after-endpoint-bypass" })
}
$GoldenCases = @(
    @{ Core = "sing-box"; FailurePoint = "none"; CrashPoint = "none" },
    @{ Core = "mihomo"; FailurePoint = "none"; CrashPoint = "none" }
)
$FailureCases = @()
foreach ($Point in $FailurePoints) {
    $FailureCases += @{ Core = "sing-box"; FailurePoint = $Point; CrashPoint = "none" }
}
$CrashCases = @()
foreach ($Point in $FailurePoints) {
    $CrashCases += @{ Core = "sing-box"; FailurePoint = "none"; CrashPoint = $Point }
}
$LifecycleCases = @(
    @{ Core = "sing-box"; FailurePoint = "none"; CrashPoint = "none"; Scenario = "repeat" },
    @{ Core = "sing-box"; FailurePoint = "none"; CrashPoint = "none"; Scenario = "tun-proxy-tun" },
    @{ Core = "sing-box"; FailurePoint = "none"; CrashPoint = "none"; Scenario = "tun-off-tun" },
    @{ Core = "sing-box"; FailurePoint = "none"; CrashPoint = "none"; Scenario = "disable-adapter" },
    @{ Core = "sing-box"; FailurePoint = "none"; CrashPoint = "none"; Scenario = "core-crash" }
)
$RoutingCases = @(
    @{ Core = "sing-box"; FailurePoint = "none"; CrashPoint = "none"; Scenario = "routing-modes" },
    @{ Core = "mihomo"; FailurePoint = "none"; CrashPoint = "none"; Scenario = "routing-modes" }
)
$Cases = switch ($Suite) {
    "golden" { $GoldenCases }
    "failure" { $FailureCases }
    "crash" { $CrashCases }
    "lifecycle" { $LifecycleCases }
    "routing" { $RoutingCases }
    default { $RoutingCases + $GoldenCases + $LifecycleCases + $FailureCases + $CrashCases }
}

$Failures = [System.Collections.Generic.List[string]]::new()
foreach ($Case in $Cases) {
    $Arguments = @(
        "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", $Runner,
        "-Core", $Case.Core,
        "-FailurePoint", $Case.FailurePoint,
        "-CrashPoint", $Case.CrashPoint,
        "-Scenario", $(if ($Case.Scenario) { $Case.Scenario } else { "golden" }),
        "-PackageRoot", $PackageRoot,
        "-OutputDirectory", $OutputDirectory,
        "-StabilitySeconds", $StabilitySeconds
    )
    if (-not [string]::IsNullOrWhiteSpace($SourceProfile)) {
        $Arguments += @("-SourceProfile", $SourceProfile)
    }
    if (-not [string]::IsNullOrWhiteSpace($UpstreamServer)) {
        $Arguments += @(
            "-UpstreamProtocol", $UpstreamProtocol,
            "-UpstreamServer", $UpstreamServer,
            "-UpstreamPort", $UpstreamPort
        )
    }
    elseif ($AutoSelectReachableOutbound) {
        $Arguments += "-AutoSelectReachableOutbound"
    }
    & powershell.exe @Arguments
    if ($LASTEXITCODE -ne 0) {
        $Failures.Add("$($Case.Core)/scenario=$(if ($Case.Scenario) { $Case.Scenario } else { 'golden' })/failure=$($Case.FailurePoint)/crash=$($Case.CrashPoint)")
        if ($StopOnFailure) { break }
    }
}

if ($Failures.Count -gt 0) {
    throw "TUN acceptance matrix failed: $($Failures -join ', ')"
}
Write-Output "TUN acceptance matrix passed ($($Cases.Count) cases)"
