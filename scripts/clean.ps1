$ErrorActionPreference = "Stop"
$ProjectRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot ".."))

$RelativeTargets = @(
    ".cache",
    ".tmp",
    "coverage.out",
    "navo.exe",
    "release",
    "navo_app\build",
    "navo_app\frontend\dist",
    "navo_app\node_modules",
    "navo_app\wailsjs"
)
$RelativeTargets += Get-ChildItem -LiteralPath $ProjectRoot -Directory |
    Where-Object Name -Like "gocache*" |
    ForEach-Object Name

foreach ($RelativeTarget in $RelativeTargets | Select-Object -Unique) {
    $Target = [System.IO.Path]::GetFullPath((Join-Path $ProjectRoot $RelativeTarget))
    $ExpectedPrefix = $ProjectRoot + [System.IO.Path]::DirectorySeparatorChar
    if (-not $Target.StartsWith($ExpectedPrefix, [System.StringComparison]::OrdinalIgnoreCase)) {
        throw "Refusing to clean outside project: $Target"
    }
    if (Test-Path -LiteralPath $Target) {
        Remove-Item -LiteralPath $Target -Recurse -Force
        Write-Host "Removed: $Target"
    }
}
