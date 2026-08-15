param(
    [Parameter(Mandatory)]
    [string]$ProfileRoot,
    [Parameter(Mandatory)]
    [string]$OutputPath
)

$ErrorActionPreference = "Stop"
$ProfileRoot = [IO.Path]::GetFullPath($ProfileRoot)
$OutputPath = [IO.Path]::GetFullPath($OutputPath)

function Test-IsAdministrator {
    $Identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $Principal = [Security.Principal.WindowsPrincipal]::new($Identity)
    return $Principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Protect-LogLine {
    param([string]$Line)
    if ($null -eq $Line) { return "" }
    $Value = $Line -replace '(?i)(https?|socks5)://[^@\s/]+@', '$1://***@'
    $Value = $Value -replace '(?i)("(?:password|token|secret|authorization|credential)"\s*:\s*")[^"]*', '$1***'
    return $Value
}

if (-not (Test-IsAdministrator)) { throw "Administrator privileges are required" }
if (-not (Test-Path -LiteralPath $ProfileRoot -PathType Container)) {
    throw "Navo profile does not exist: $ProfileRoot"
}

$OutputDirectory = Split-Path -Parent $OutputPath
New-Item -ItemType Directory -Force $OutputDirectory | Out-Null

$Files = @(Get-ChildItem -LiteralPath $ProfileRoot -Recurse -File -Force -ErrorAction SilentlyContinue)
$Inventory = @($Files | Sort-Object FullName | ForEach-Object {
    [ordered]@{
        path = $_.FullName.Substring($ProfileRoot.Length).TrimStart('\')
        bytes = $_.Length
        modified_at = $_.LastWriteTimeUtc.ToString('o')
    }
})

$ACL = Get-Acl -LiteralPath $ProfileRoot
$LogPattern = '(?i)tun|capture|restart|selfheal|supervisor|adapter|route|nrpt|dns|core|health|fault|error|warn|rollback'
$LogEvidence = [Collections.Generic.List[object]]::new()
foreach ($File in $Files | Where-Object { $_.Extension -in @('.log', '.jsonl') }) {
    $Relative = $File.FullName.Substring($ProfileRoot.Length).TrimStart('\')
    $Lines = @(Get-Content -LiteralPath $File.FullName -Tail 3000 -ErrorAction SilentlyContinue)
    for ($Index = 0; $Index -lt $Lines.Count; $Index++) {
        $Line = [string]$Lines[$Index]
        if ($Line -match $LogPattern) {
            $LogEvidence.Add([ordered]@{
                file = $Relative
                tail_line = $Index + 1
                text = Protect-LogLine $Line
            })
        }
    }
}

$NRPT = @(Get-DnsClientNrptRule -ErrorAction SilentlyContinue |
    Where-Object { [string]$_.Comment -like 'Navo:TUN:*' } |
    Select-Object Namespace, NameServers, Comment)
$Routes = @(Get-NetRoute -PolicyStore ActiveStore -ErrorAction SilentlyContinue |
    Where-Object { $_.DestinationPrefix -in @('0.0.0.0/1', '128.0.0.0/1') } |
    Select-Object DestinationPrefix, InterfaceIndex, InterfaceAlias, NextHop, RouteMetric)
$Adapters = @(Get-NetAdapter -ErrorAction SilentlyContinue |
    Where-Object { $_.Name -eq 'Navo' -or $_.InterfaceDescription -like '*Wintun*' } |
    Select-Object Name, ifIndex, Status, InterfaceDescription)
$Firewall = @(Get-NetFirewallRule -ErrorAction SilentlyContinue |
    Where-Object { $_.DisplayName -like 'Navo TUN IPv6 Block *' } |
    Select-Object DisplayName, Enabled, Direction, Action)

$Result = [ordered]@{
    captured_at = [DateTimeOffset]::UtcNow.ToString('o')
    profile_root = $ProfileRoot
    profile_owner = [string]$ACL.Owner
    profile_sddl = [string]$ACL.Sddl
    files = $Inventory
    log_evidence = @($LogEvidence)
    network = [ordered]@{
        nrpt = $NRPT
        split_routes = $Routes
        adapters = $Adapters
        firewall = $Firewall
    }
}

$Result | ConvertTo-Json -Depth 12 | Set-Content -LiteralPath $OutputPath -Encoding UTF8
Write-Output $OutputPath
