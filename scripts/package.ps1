param(
    [ValidateSet("amd64")]
    [string]$Architecture = "amd64",
    [ValidatePattern("^Navo(?:-[A-Za-z0-9._-]+)?$")]
    [string]$OutputName = "Navo"
)

$ErrorActionPreference = "Stop"
$ProjectRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot ".."))
$ReleaseParent = [System.IO.Path]::GetFullPath((Join-Path $ProjectRoot "release"))
$ReleaseRoot = [System.IO.Path]::GetFullPath((Join-Path $ReleaseParent $OutputName))
$UIRoot = Join-Path $ProjectRoot "navo_app"
$UICache = Join-Path $ProjectRoot ".cache\npm"
$ToolsRoot = Join-Path $ProjectRoot ".cache\tools"
$BuildTemp = Join-Path $ProjectRoot ".cache\tmp"
$Wails = Join-Path $ToolsRoot "wails.exe"
$GoWinResCommand = Get-Command go-winres.exe -ErrorAction SilentlyContinue
$GoWinRes = if ($null -ne $GoWinResCommand) {
    $GoWinResCommand.Source
}
else {
    Join-Path $env:USERPROFILE "go\bin\go-winres.exe"
}
if (-not (Test-Path -LiteralPath $GoWinRes -PathType Leaf)) {
    throw "go-winres v0.3.3 is required; install it with: go install github.com/tc-hib/go-winres@v0.3.3"
}

if (-not $ReleaseRoot.StartsWith($ReleaseParent, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "Refusing to package outside $ReleaseParent"
}

$Go = (Get-Command go -ErrorAction Stop).Source
$Npm = (Get-Command npm.cmd -ErrorAction Stop).Source
$env:GOPATH = Join-Path $ProjectRoot ".cache\go-path"
$env:GOMODCACHE = Join-Path $env:GOPATH "pkg\mod"
$env:GOCACHE = Join-Path $ProjectRoot ".cache\go-build"
$env:TEMP = $BuildTemp
$env:TMP = $BuildTemp
$env:GOOS = "windows"
$env:GOARCH = $Architecture
New-Item -ItemType Directory -Force $env:GOMODCACHE, $env:GOCACHE, $UICache, $ToolsRoot, $BuildTemp | Out-Null
$WailsSource = Join-Path $env:GOMODCACHE "github.com\wailsapp\wails\v2@v2.12.0"

$RequiredThirdPartyFiles = @(
    (Join-Path $ProjectRoot "third_party\sing-box\sing-box.exe"),
    (Join-Path $ProjectRoot "third_party\mihomo\mihomo.exe"),
    (Join-Path $ProjectRoot "third_party\xray\xray.exe")
)
foreach ($RequiredFile in $RequiredThirdPartyFiles) {
    if (-not (Test-Path -LiteralPath $RequiredFile -PathType Leaf)) {
        throw "Required core executable is missing: $RequiredFile"
    }
}

# Release output is untouched until every source gate passes.
Push-Location $ProjectRoot
try {
    & $Go test ./...
    if ($LASTEXITCODE -ne 0) {
        throw "Go test gate failed with exit code $LASTEXITCODE"
    }
    & $Go vet ./...
    if ($LASTEXITCODE -ne 0) {
        throw "Go vet gate failed with exit code $LASTEXITCODE"
    }
}
finally {
    Pop-Location
}

Push-Location $UIRoot
try {
    & $Npm --cache $UICache ci
    if ($LASTEXITCODE -ne 0) {
        throw "UI dependency installation failed with exit code $LASTEXITCODE"
    }
    & $Npm --cache $UICache test
    if ($LASTEXITCODE -ne 0) {
        throw "UI test gate failed with exit code $LASTEXITCODE"
    }
    & $Npm --cache $UICache run typecheck
    if ($LASTEXITCODE -ne 0) {
        throw "UI typecheck gate failed with exit code $LASTEXITCODE"
    }
    & $Npm --cache $UICache run build
    if ($LASTEXITCODE -ne 0) {
        throw "UI production build failed with exit code $LASTEXITCODE"
    }
}
finally {
    Pop-Location
}

if (Test-Path -LiteralPath $ReleaseRoot) {
    Remove-Item -LiteralPath $ReleaseRoot -Recurse -Force
}
New-Item -ItemType Directory -Force $ReleaseRoot | Out-Null

if (-not (Test-Path -LiteralPath $WailsSource -PathType Container)) {
    throw "Wails v2.12.0 source is missing from the local module cache"
}
if (-not (Test-Path -LiteralPath $Wails -PathType Leaf)) {
    Push-Location $WailsSource
    try {
        & $Go build -trimpath -o $Wails ./cmd/wails
        if ($LASTEXITCODE -ne 0) {
            throw "Wails CLI build failed with exit code $LASTEXITCODE"
        }
    }
    finally {
        Pop-Location
    }
}

Push-Location $ProjectRoot
try {
    & $Go build -trimpath -ldflags "-s -w -H windowsgui" `
        -o (Join-Path $ReleaseRoot "navo.exe") ./cmd/navo
    if ($LASTEXITCODE -ne 0) {
        throw "Navo launcher build failed with exit code $LASTEXITCODE"
    }
    & $GoWinRes patch --no-backup `
        --in (Join-Path $ProjectRoot "cmd\navo\winres\admin.json") `
        (Join-Path $ReleaseRoot "navo.exe")
    if ($LASTEXITCODE -ne 0) {
        throw "Navo launcher manifest patch failed with exit code $LASTEXITCODE"
    }

    & $Go build -trimpath -ldflags "-s -w" `
        -o (Join-Path $ReleaseRoot "repair.exe") ./cmd/repair
    if ($LASTEXITCODE -ne 0) {
        throw "Repair tool build failed with exit code $LASTEXITCODE"
    }
}
finally {
    Pop-Location
}

Push-Location $UIRoot
try {
    & $Wails build -clean -platform "windows/$Architecture" -o navo_app.exe
    if ($LASTEXITCODE -ne 0) {
        throw "Wails desktop build failed with exit code $LASTEXITCODE"
    }
}
finally {
    Pop-Location
}

$UIOutput = Join-Path $UIRoot "build\bin\navo_app.exe"
if (-not (Test-Path -LiteralPath $UIOutput -PathType Leaf)) {
    throw "Wails output is missing: $UIOutput"
}

$PackagedUI = Join-Path $ReleaseRoot "app_ui"
New-Item -ItemType Directory -Force $PackagedUI | Out-Null
Copy-Item -LiteralPath $UIOutput -Destination (Join-Path $PackagedUI "navo_app.exe")
Copy-Item -LiteralPath (Join-Path $UIRoot "assets\tray_icon.ico") `
    -Destination (Join-Path $PackagedUI "tray_icon.ico")
Copy-Item -LiteralPath (Join-Path $ProjectRoot "third_party") `
    -Destination (Join-Path $ReleaseRoot "third_party") -Recurse
Copy-Item -LiteralPath (Join-Path $ProjectRoot ".env.example") `
    -Destination (Join-Path $ReleaseRoot ".env.example")
Copy-Item -LiteralPath (Join-Path $ProjectRoot "CORE_MANIFEST.json") `
    -Destination (Join-Path $ReleaseRoot "CORE_MANIFEST.json")
Copy-Item -LiteralPath (Join-Path $ProjectRoot "docs\INSTALL_DEPLOY.md") `
    -Destination (Join-Path $ReleaseRoot "INSTALL_DEPLOY.md")

$Readme = @(
    "Navo",
    "",
    "Start: double-click navo.exe and approve the Windows UAC prompt.",
    "Administrator access is required because this package runs the TUN/core host in-process.",
    "Requirement: Microsoft Edge WebView2 Runtime.",
    "Configuration and runtime state are stored locally under %LOCALAPPDATA%\Navo.",
    "Logs: %LOCALAPPDATA%\Navo\log\navo.log",
    "Data: %LOCALAPPDATA%\Navo\",
    "Repair: run repair.exe check before applying any repair action."
)
[System.IO.File]::WriteAllLines(
    (Join-Path $ReleaseRoot "README.txt"),
    $Readme,
    [System.Text.UTF8Encoding]::new($false)
)

$HashLines = Get-ChildItem -LiteralPath $ReleaseRoot -File -Recurse |
    Where-Object Name -ne "SHA256SUMS.txt" |
    Sort-Object FullName |
    ForEach-Object {
        $RelativePath = $_.FullName.Substring($ReleaseRoot.Length + 1).Replace("\", "/")
        $Hash = (Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
        "$Hash  $RelativePath"
    }
[System.IO.File]::WriteAllLines(
    (Join-Path $ReleaseRoot "SHA256SUMS.txt"),
    $HashLines,
    [System.Text.UTF8Encoding]::new($false)
)

Write-Host "Package ready: $ReleaseRoot"
