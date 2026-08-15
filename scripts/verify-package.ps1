param(
    [Parameter(Mandatory = $true)]
    [string]$PackageRoot,
    [string]$ArchivePath = "",
    [string]$ExpectedVersion = "",
    [switch]$RequireSignature
)

$ErrorActionPreference = "Stop"
$ResolvedRoot = [System.IO.Path]::GetFullPath($PackageRoot)
if (-not (Test-Path -LiteralPath $ResolvedRoot -PathType Container)) {
    throw "Package root does not exist: $ResolvedRoot"
}

function Get-RelativePackagePath {
    param(
        [Parameter(Mandatory = $true)]
        [string]$FullPath
    )

    return $FullPath.Substring($ResolvedRoot.Length + 1).Replace("\", "/")
}

function Assert-SafeRelativePath {
    param(
        [Parameter(Mandatory = $true)]
        [string]$RelativePath
    )

    $Normalized = $RelativePath.Replace("\", "/").Trim()
    if ([string]::IsNullOrWhiteSpace($Normalized) -or
        [System.IO.Path]::IsPathRooted($Normalized) -or
        $Normalized.StartsWith("/", [System.StringComparison]::Ordinal) -or
        $Normalized.Split("/") -contains "..") {
        throw "Unsafe package-relative path: $RelativePath"
    }
    return $Normalized
}

function New-PathSet {
    return ,([System.Collections.Generic.HashSet[string]]::new(
        [System.StringComparer]::OrdinalIgnoreCase
    ))
}

function Assert-EqualPathSets {
    param(
        [Parameter(Mandatory = $true)]
        [System.Collections.Generic.HashSet[string]]$Expected,
        [Parameter(Mandatory = $true)]
        [System.Collections.Generic.HashSet[string]]$Actual,
        [Parameter(Mandatory = $true)]
        [string]$Label
    )

    $Missing = @($Expected | Where-Object { -not $Actual.Contains($_) } | Sort-Object)
    $Unexpected = @($Actual | Where-Object { -not $Expected.Contains($_) } | Sort-Object)
    if ($Missing.Count -gt 0 -or $Unexpected.Count -gt 0) {
        $Details = @()
        if ($Missing.Count -gt 0) {
            $Details += "missing=[$($Missing -join ', ')]"
        }
        if ($Unexpected.Count -gt 0) {
            $Details += "unexpected=[$($Unexpected -join ', ')]"
        }
        throw "$Label file set mismatch: $($Details -join '; ')"
    }
}

function Assert-PEVersion {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Path,
        [Parameter(Mandatory = $true)]
        [string]$Version
    )

    $Info = [System.Diagnostics.FileVersionInfo]::GetVersionInfo($Path)
    $FileVersion = "{0}.{1}.{2}.{3}" -f `
        $Info.FileMajorPart,
        $Info.FileMinorPart,
        $Info.FileBuildPart,
        $Info.FilePrivatePart
    $ProductVersion = "{0}.{1}.{2}.{3}" -f `
        $Info.ProductMajorPart,
        $Info.ProductMinorPart,
        $Info.ProductBuildPart,
        $Info.ProductPrivatePart
    $ExpectedPEVersion = "$Version.0"
    if ($FileVersion -ne $ExpectedPEVersion -or $ProductVersion -ne $ExpectedPEVersion) {
        throw "PE version mismatch for $Path`: file=$FileVersion product=$ProductVersion expected=$ExpectedPEVersion"
    }

    if ($RequireSignature) {
        $Signature = Get-AuthenticodeSignature -LiteralPath $Path
        if ($Signature.Status -ne [System.Management.Automation.SignatureStatus]::Valid) {
            throw "Authenticode signature is not valid for $Path`: $($Signature.Status)"
        }
    }
}

$VersionPath = Join-Path $ResolvedRoot "VERSION"
if (-not (Test-Path -LiteralPath $VersionPath -PathType Leaf)) {
    throw "Package VERSION file is missing"
}
$PackagedVersion = (Get-Content -LiteralPath $VersionPath -Raw).Trim()
if ([string]::IsNullOrWhiteSpace($ExpectedVersion)) {
    $ExpectedVersion = $PackagedVersion
}
if ($ExpectedVersion -notmatch '^\d+\.\d+\.\d+$') {
    throw "Expected version must use MAJOR.MINOR.PATCH: $ExpectedVersion"
}
if ($PackagedVersion -ne $ExpectedVersion) {
    throw "Package VERSION mismatch: packaged=$PackagedVersion expected=$ExpectedVersion"
}

$ChecksumPath = Join-Path $ResolvedRoot "SHA256SUMS.txt"
if (-not (Test-Path -LiteralPath $ChecksumPath -PathType Leaf)) {
    throw "SHA256SUMS.txt is missing"
}

$ManifestHashes = [System.Collections.Generic.Dictionary[string, string]]::new(
    [System.StringComparer]::OrdinalIgnoreCase
)
foreach ($Line in Get-Content -LiteralPath $ChecksumPath) {
    if ([string]::IsNullOrWhiteSpace($Line)) {
        continue
    }
    if ($Line -notmatch '^([0-9a-fA-F]{64})\s{2}(.+)$') {
        throw "Invalid SHA256SUMS entry: $Line"
    }
    $RelativePath = Assert-SafeRelativePath $Matches[2]
    if ($ManifestHashes.ContainsKey($RelativePath)) {
        throw "Duplicate SHA256SUMS path: $RelativePath"
    }
    $ManifestHashes.Add($RelativePath, $Matches[1].ToLowerInvariant())
}
if ($ManifestHashes.Count -eq 0) {
    throw "SHA256SUMS.txt contains no files"
}

$ExpectedFiles = New-PathSet
foreach ($RelativePath in $ManifestHashes.Keys) {
    [void]$ExpectedFiles.Add($RelativePath)
}
[void]$ExpectedFiles.Add("SHA256SUMS.txt")

$ActualFiles = New-PathSet
foreach ($File in Get-ChildItem -LiteralPath $ResolvedRoot -File -Recurse) {
    [void]$ActualFiles.Add((Get-RelativePackagePath $File.FullName))
}
Assert-EqualPathSets -Expected $ExpectedFiles -Actual $ActualFiles -Label "Package directory"

foreach ($Entry in $ManifestHashes.GetEnumerator()) {
    $RelativePath = $Entry.Key.Replace("/", [System.IO.Path]::DirectorySeparatorChar)
    $FullPath = [System.IO.Path]::GetFullPath((Join-Path $ResolvedRoot $RelativePath))
    $RootPrefix = $ResolvedRoot.TrimEnd("\", "/") + [System.IO.Path]::DirectorySeparatorChar
    if (-not $FullPath.StartsWith($RootPrefix, [System.StringComparison]::OrdinalIgnoreCase)) {
        throw "Manifest path escapes package root: $($Entry.Key)"
    }
    if (-not (Test-Path -LiteralPath $FullPath -PathType Leaf)) {
        throw "Manifest file is missing: $($Entry.Key)"
    }
    $ActualHash = (Get-FileHash -LiteralPath $FullPath -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($ActualHash -ne $Entry.Value) {
        throw "SHA-256 mismatch for $($Entry.Key)"
    }
}

$RequiredFiles = @(
    "VERSION",
    "CORE_MANIFEST.json",
    "THIRD_PARTY_NOTICES.md",
    "third_party/sing-box/LICENSE",
    "third_party/mihomo/LICENSE",
    "third_party/xray/LICENSE",
    "third_party/wintun/LICENSE.txt"
)
foreach ($RequiredFile in $RequiredFiles) {
    if (-not $ActualFiles.Contains($RequiredFile)) {
        throw "Required package file is missing: $RequiredFile"
    }
}

$CoreManifestPath = Join-Path $ResolvedRoot "CORE_MANIFEST.json"
$CoreManifest = Get-Content -LiteralPath $CoreManifestPath -Raw | ConvertFrom-Json
if ($CoreManifest.schema_version -ne 1 -or $null -eq $CoreManifest.cores) {
    throw "CORE_MANIFEST.json schema is unsupported"
}
foreach ($Core in $CoreManifest.cores) {
    $RelativePath = Assert-SafeRelativePath ([string]$Core.relative_path)
    $CorePath = Join-Path $ResolvedRoot $RelativePath.Replace("/", "\")
    if (-not (Test-Path -LiteralPath $CorePath -PathType Leaf)) {
        throw "Core binary is missing: $RelativePath"
    }
    $ExpectedHash = ([string]$Core.sha256).ToLowerInvariant()
    if ($ExpectedHash -notmatch '^[0-9a-f]{64}$') {
        throw "Invalid core SHA-256 for $($Core.type)"
    }
    $ActualHash = (Get-FileHash -LiteralPath $CorePath -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($ActualHash -ne $ExpectedHash) {
        throw "Core SHA-256 mismatch for $($Core.type)"
    }
}

$OwnedExecutables = @(
    (Join-Path $ResolvedRoot "navo.exe"),
    (Join-Path $ResolvedRoot "repair.exe"),
    (Join-Path $ResolvedRoot "app_ui\navo_app.exe")
)
foreach ($Executable in $OwnedExecutables) {
    if (-not (Test-Path -LiteralPath $Executable -PathType Leaf)) {
        throw "Navo executable is missing: $Executable"
    }
    Assert-PEVersion -Path $Executable -Version $ExpectedVersion
}

$ArchiveFileCount = 0
if (-not [string]::IsNullOrWhiteSpace($ArchivePath)) {
    $ResolvedArchive = [System.IO.Path]::GetFullPath($ArchivePath)
    if (-not (Test-Path -LiteralPath $ResolvedArchive -PathType Leaf)) {
        throw "Package archive does not exist: $ResolvedArchive"
    }
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $Archive = [System.IO.Compression.ZipFile]::OpenRead($ResolvedArchive)
    try {
        $ArchiveFiles = New-PathSet
        $ExpectedPrefix = [System.IO.Path]::GetFileName($ResolvedRoot) + "/"
        foreach ($Entry in $Archive.Entries) {
            if ([string]::IsNullOrEmpty($Entry.Name)) {
                continue
            }
            $ArchivePathValue = $Entry.FullName.Replace("\", "/")
            if (-not $ArchivePathValue.StartsWith($ExpectedPrefix, [System.StringComparison]::Ordinal)) {
                throw "Archive entry is outside the package root: $ArchivePathValue"
            }
            $RelativePath = Assert-SafeRelativePath $ArchivePathValue.Substring($ExpectedPrefix.Length)
            if (-not $ArchiveFiles.Add($RelativePath)) {
                throw "Duplicate archive path: $RelativePath"
            }
            $DiskPath = Join-Path $ResolvedRoot $RelativePath.Replace("/", "\")
            $DiskFile = Get-Item -LiteralPath $DiskPath
            if ($Entry.Length -ne $DiskFile.Length) {
                throw "Archive length mismatch for $RelativePath`: archive=$($Entry.Length) directory=$($DiskFile.Length)"
            }

            $EntryStream = $Entry.Open()
            $Hasher = [System.Security.Cryptography.SHA256]::Create()
            try {
                $HashBytes = $Hasher.ComputeHash($EntryStream)
                $ArchiveHash = -join @(
                    $HashBytes | ForEach-Object { $_.ToString("x2") }
                )
            }
            finally {
                $Hasher.Dispose()
                $EntryStream.Dispose()
            }

            $DirectoryHash = (
                Get-FileHash -LiteralPath $DiskFile.FullName -Algorithm SHA256
            ).Hash.ToLowerInvariant()
            if ($ArchiveHash -ne $DirectoryHash) {
                throw "Archive SHA-256 mismatch for $RelativePath"
        }
        }
        Assert-EqualPathSets -Expected $ActualFiles -Actual $ArchiveFiles -Label "Package archive"
        $ArchiveFileCount = $ArchiveFiles.Count
    }
    finally {
        $Archive.Dispose()
    }
}

[pscustomobject]@{
    version = $ExpectedVersion
    package_root = $ResolvedRoot
    file_count = $ActualFiles.Count
    manifest_entries = $ManifestHashes.Count
    archive_file_count = $ArchiveFileCount
    signatures_required = [bool]$RequireSignature
    issues_found = 0
} | ConvertTo-Json -Depth 3
