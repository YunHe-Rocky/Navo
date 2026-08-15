param(
    [ValidateSet("sing-box", "mihomo")]
    [string]$Core = "sing-box",
    [string]$PackageRoot = (Join-Path $PSScriptRoot "..\release\Navo")
)

$ErrorActionPreference = "Stop"
$ProjectRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot ".."))
$env:NAVO_RUN_ELEVATED_TUN_TESTS = "1"
$env:NAVO_TUN_TEST_PACKAGE_ROOT = [System.IO.Path]::GetFullPath($PackageRoot)
$env:NAVO_TUN_TEST_CORE = $Core
$env:NAVO_TUN_FAILURE_POINT = "none"
$env:NAVO_TUN_CRASH_POINT = "none"
$env:GOPATH = Join-Path $ProjectRoot ".cache\go-path"
$env:GOMODCACHE = Join-Path $env:GOPATH "pkg\mod"
$env:GOCACHE = Join-Path $ProjectRoot ".cache\go-build"
$env:TEMP = Join-Path $ProjectRoot ".cache\tmp"
$env:TMP = $env:TEMP
New-Item -ItemType Directory -Force $env:GOMODCACHE, $env:GOCACHE, $env:TEMP | Out-Null

Push-Location $ProjectRoot
try {
    & go test ./internal/network -run '^TestElevatedTUNAcceptance$' -count=1 -v
    if ($LASTEXITCODE -ne 0) {
        throw "Elevated Go TUN integration test failed with exit code $LASTEXITCODE"
    }
}
finally {
    Pop-Location
}
