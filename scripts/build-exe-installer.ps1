param(
    [ValidatePattern('^\d+\.\d+\.\d+$')]
    [string]$Version = "1.0.0"
)

$ErrorActionPreference = "Stop"
$ProjectRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot ".."))
$OutputRoot = Join-Path $ProjectRoot "release\installer"
$MsiName = "Navo-Setup-$Version-x64.msi"
$MsiPath = Join-Path $OutputRoot $MsiName
$SetupPath = Join-Path $OutputRoot "Navo-Setup-$Version-x64_setup.exe"
$CacheRoot = Join-Path $ProjectRoot ".cache\installer\bootstrapper"
$BootstrapperTemplate = Join-Path $ProjectRoot "installer\bootstrapper\main.go.tmpl"
$WinresConfig = Join-Path $ProjectRoot "installer\bootstrapper\winres.json"
$BootstrapperSource = Join-Path $CacheRoot "main.go"
$PayloadPath = Join-Path $CacheRoot "payload.msi"

if (-not (Test-Path -LiteralPath $MsiPath -PathType Leaf)) {
    throw "MSI is missing: $MsiPath. Run scripts/build-installer.ps1 first."
}
if (-not (Test-Path -LiteralPath $BootstrapperTemplate -PathType Leaf)) {
    throw "Bootstrapper template is missing: $BootstrapperTemplate"
}
$GoWinres = (Get-Command go-winres.exe -ErrorAction SilentlyContinue).Source
if (-not $GoWinres) {
    $GoWinres = Join-Path $env:USERPROFILE "go\bin\go-winres.exe"
}
if (-not (Test-Path -LiteralPath $GoWinres -PathType Leaf)) {
    throw "go-winres.exe is required to embed the setup icon"
}

if (Test-Path -LiteralPath $CacheRoot) {
    $Resolved = [System.IO.Path]::GetFullPath($CacheRoot)
    $ExpectedPrefix = [System.IO.Path]::GetFullPath((Join-Path $ProjectRoot ".cache")) +
        [System.IO.Path]::DirectorySeparatorChar
    if (-not $Resolved.StartsWith($ExpectedPrefix, [System.StringComparison]::OrdinalIgnoreCase)) {
        throw "Refusing to replace temporary files outside the project cache"
    }
    Remove-Item -LiteralPath $Resolved -Recurse -Force
}
New-Item -ItemType Directory -Force $CacheRoot | Out-Null

Copy-Item -LiteralPath $BootstrapperTemplate -Destination $BootstrapperSource -Force
Copy-Item -LiteralPath $MsiPath -Destination $PayloadPath -Force
try {
    & $GoWinres make `
        --in $WinresConfig `
        --arch amd64 `
        --out (Join-Path $CacheRoot "rsrc") `
        --product-version $Version `
        --file-version $Version
    if ($LASTEXITCODE -ne 0) {
        throw "go-winres failed with exit code $LASTEXITCODE"
    }

    $env:CGO_ENABLED = "0"
    $env:GOOS = "windows"
    $env:GOARCH = "amd64"
    $env:GOCACHE = Join-Path $ProjectRoot ".cache\go-build"
    Push-Location $CacheRoot
    try {
        & go build -trimpath -ldflags "-s -w -H windowsgui" -o $SetupPath .
        if ($LASTEXITCODE -ne 0 -or -not (Test-Path -LiteralPath $SetupPath -PathType Leaf)) {
            throw "Go failed to generate setup.exe (exit code $LASTEXITCODE)"
        }
    }
    finally {
        Pop-Location
    }
}
finally {
    Remove-Item -LiteralPath $PayloadPath -Force -ErrorAction SilentlyContinue
}

$Hash = (Get-FileHash -LiteralPath $SetupPath -Algorithm SHA256).Hash.ToLowerInvariant()
$HashPath = "$SetupPath.sha256"
[System.IO.File]::WriteAllText(
    $HashPath,
    "$Hash  $([System.IO.Path]::GetFileName($SetupPath))`n",
    [System.Text.UTF8Encoding]::new($false)
)

[pscustomobject]@{
    Installer = $SetupPath
    Version = $Version
    InstallerMiB = [math]::Round((Get-Item -LiteralPath $SetupPath).Length / 1MB, 2)
    SHA256 = $Hash
} | Format-List
