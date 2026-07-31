param(
    [ValidatePattern('^\d+\.\d+\.\d+$')]
    [string]$Version = "1.0.0"
)

$ErrorActionPreference = "Stop"
$ProjectRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot ".."))
$ReleaseRoot = Join-Path $ProjectRoot "release\Navo"
$InstallerRoot = Join-Path $ProjectRoot "installer"
$CacheRoot = Join-Path $ProjectRoot ".cache\installer"
$StagingRoot = Join-Path $CacheRoot "payload"
$IntermediateRoot = Join-Path $CacheRoot "obj"
$OutputRoot = Join-Path $ProjectRoot "release\installer"
$OutputPath = Join-Path $OutputRoot "Navo-Setup-$Version-x64.msi"
$WixRoot = Join-Path $ProjectRoot ".cache\tools\wix"
$Wix = Join-Path $WixRoot "wix.exe"
$WixVersion = "5.0.2"

$RequiredPayload = @(
    "navo.exe",
    "repair.exe",
    "CORE_MANIFEST.json",
    ".env.example",
    "README.txt",
    "INSTALL_DEPLOY.md",
    "SHA256SUMS.txt",
    "app_ui",
    "third_party"
)

if (-not (Test-Path -LiteralPath $ReleaseRoot -PathType Container)) {
    throw "Release payload is missing. Run scripts/package.ps1 first."
}
foreach ($RelativePath in $RequiredPayload) {
    $Source = Join-Path $ReleaseRoot $RelativePath
    if (-not (Test-Path -LiteralPath $Source)) {
        throw "Required installer payload is missing: $Source"
    }
}
if (Test-Path -LiteralPath (Join-Path $ReleaseRoot ".env")) {
    throw "Refusing to package the developer .env file"
}

if (-not (Test-Path -LiteralPath $Wix -PathType Leaf)) {
    $DotNet = (Get-Command dotnet.exe -ErrorAction Stop).Source
    $env:DOTNET_CLI_HOME = Join-Path $ProjectRoot ".cache\dotnet-home"
    $env:NUGET_PACKAGES = Join-Path $ProjectRoot ".cache\nuget"
    $env:DOTNET_SKIP_FIRST_TIME_EXPERIENCE = "1"
    $env:DOTNET_CLI_TELEMETRY_OPTOUT = "1"
    New-Item -ItemType Directory -Force $WixRoot | Out-Null
    & $DotNet tool install wix --tool-path $WixRoot --version $WixVersion
    if ($LASTEXITCODE -ne 0) {
        throw "Install WiX $WixVersion failed with exit code $LASTEXITCODE"
    }
}

foreach ($GeneratedPath in @($StagingRoot, $IntermediateRoot)) {
    $Resolved = [System.IO.Path]::GetFullPath($GeneratedPath)
    $ExpectedPrefix = [System.IO.Path]::GetFullPath($CacheRoot) + `
        [System.IO.Path]::DirectorySeparatorChar
    if (-not $Resolved.StartsWith($ExpectedPrefix, [System.StringComparison]::OrdinalIgnoreCase)) {
        throw "Refusing to replace installer output outside $CacheRoot"
    }
    if (Test-Path -LiteralPath $Resolved) {
        Remove-Item -LiteralPath $Resolved -Recurse -Force
    }
    New-Item -ItemType Directory -Force $Resolved | Out-Null
}
New-Item -ItemType Directory -Force $OutputRoot | Out-Null

foreach ($RelativePath in $RequiredPayload) {
    Copy-Item -LiteralPath (Join-Path $ReleaseRoot $RelativePath) `
        -Destination $StagingRoot -Recurse
}

$ForbiddenFiles = @(
    Get-ChildItem -LiteralPath $StagingRoot -File -Recurse -Force |
        Where-Object {
            $_.Name -eq ".env" -or
            $_.Extension -in @(".log", ".dmp") -or
            $_.FullName -match '[\\/](data|log)[\\/]'
        }
)
if ($ForbiddenFiles.Count -gt 0) {
    throw "Forbidden runtime files entered installer staging: $($ForbiddenFiles.FullName -join ', ')"
}

& $Wix build (Join-Path $InstallerRoot "Navo.wxs") `
    -arch x64 `
    -d "PayloadDir=$StagingRoot" `
    -d "ProductVersion=$Version" `
    -dcl high `
    -intermediatefolder $IntermediateRoot `
    -pdbtype none `
    -out $OutputPath
if ($LASTEXITCODE -ne 0) {
    throw "WiX build failed with exit code $LASTEXITCODE"
}

$Hash = (Get-FileHash -LiteralPath $OutputPath -Algorithm SHA256).Hash.ToLowerInvariant()
$HashPath = "$OutputPath.sha256"
[System.IO.File]::WriteAllText(
    $HashPath,
    "$Hash  $([System.IO.Path]::GetFileName($OutputPath))`n",
    [System.Text.UTF8Encoding]::new($false)
)

$PayloadBytes = (
    Get-ChildItem -LiteralPath $StagingRoot -File -Recurse |
        Measure-Object Length -Sum
).Sum
$InstallerBytes = (Get-Item -LiteralPath $OutputPath).Length
[pscustomobject]@{
    Installer = $OutputPath
    Version = $Version
    PayloadMiB = [math]::Round($PayloadBytes / 1MB, 2)
    InstallerMiB = [math]::Round($InstallerBytes / 1MB, 2)
    SHA256 = $Hash
} | Format-List
