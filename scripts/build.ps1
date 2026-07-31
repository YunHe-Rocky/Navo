param(
    [ValidateSet("amd64", "386", "arm64")]
    [string]$Architecture = "amd64",
    [string]$Output = "navo.exe"
)

$ErrorActionPreference = "Stop"
$ProjectRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot ".."))
$env:GOPATH = Join-Path $ProjectRoot ".cache\go-path"
$env:GOMODCACHE = Join-Path $env:GOPATH "pkg\mod"
$env:GOCACHE = Join-Path $ProjectRoot ".cache\go-build"
$OutputPath = [System.IO.Path]::GetFullPath((Join-Path $ProjectRoot $Output))

if (-not $OutputPath.StartsWith($ProjectRoot, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "Output must stay inside the project: $OutputPath"
}

New-Item -ItemType Directory -Force $env:GOMODCACHE, $env:GOCACHE | Out-Null
New-Item -ItemType Directory -Force (Split-Path -Parent $OutputPath) | Out-Null
$env:GOOS = "windows"
$env:GOARCH = $Architecture

Push-Location $ProjectRoot
try {
    go build -trimpath -ldflags "-s -w -H windowsgui" -o $OutputPath ./cmd/navo
    if ($LASTEXITCODE -ne 0) {
        throw "Go build failed with exit code $LASTEXITCODE"
    }
}
finally {
    Pop-Location
}

Write-Host "Built: $OutputPath"
Write-Warning "Launcher-only developer build. Use package.ps1 to build the runnable desktop application."
