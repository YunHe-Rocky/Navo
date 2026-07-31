param([switch]$VetOnly)

$ErrorActionPreference = "Stop"
$ProjectRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot ".."))
$env:GOPATH = Join-Path $ProjectRoot ".cache\go-path"
$env:GOMODCACHE = Join-Path $env:GOPATH "pkg\mod"
$env:GOCACHE = Join-Path $ProjectRoot ".cache\go-build"
New-Item -ItemType Directory -Force $env:GOMODCACHE, $env:GOCACHE | Out-Null

Push-Location $ProjectRoot
try {
    if (-not $VetOnly) {
        go test ./...
        if ($LASTEXITCODE -ne 0) {
            throw "Go tests failed with exit code $LASTEXITCODE"
        }
    }
    go vet ./...
    if ($LASTEXITCODE -ne 0) {
        throw "go vet failed with exit code $LASTEXITCODE"
    }
}
finally {
    Pop-Location
}
