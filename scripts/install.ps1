[CmdletBinding()]
param(
    [ValidateSet("install", "update", "uninstall", "doctor")]
    [string]$Command = "install",
    [ValidateSet("stable", "beta", "experimental")]
    [string]$Channel = "stable",
    [string]$Version = "",
    [string]$Prefix = "",
    [switch]$ModifyPath,
    [switch]$VerifySignature,
    [switch]$Quiet
)

$ErrorActionPreference = "Stop"
if ($Channel -eq "experimental") { $Channel = "beta" }
$Repository = if ($env:KMC_GITHUB_REPOSITORY) { $env:KMC_GITHUB_REPOSITORY } else { "marius4lui/kmc" }
$ApiUrl = if ($env:KMC_GITHUB_API_URL) { $env:KMC_GITHUB_API_URL.TrimEnd("/") } else { "https://api.github.com" }
$DownloadUrl = if ($env:KMC_GITHUB_DOWNLOAD_URL) { $env:KMC_GITHUB_DOWNLOAD_URL.TrimEnd("/") } else { "https://github.com" }

function Write-Status([string]$Message) {
    if (-not $Quiet) { Write-Host $Message }
}

function Stop-Installer([string]$Message) {
    throw "kmc installer: $Message"
}

if (-not ($IsWindows -or $env:OS -eq "Windows_NT")) {
    Stop-Installer "install.ps1 supports Windows only; use install.sh on macOS or Linux"
}

$Arch = switch ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString()) {
    "X64" { "amd64" }
    "Arm64" { "arm64" }
    default { Stop-Installer "unsupported architecture: $_" }
}

if ([string]::IsNullOrWhiteSpace($Prefix)) {
    $Prefix = Join-Path ([Environment]::GetFolderPath("LocalApplicationData")) "Programs\kmc"
}
$Prefix = [System.IO.Path]::GetFullPath($Prefix)
$KmcCommand = Join-Path $Prefix "kmc.exe"
$StateFile = Join-Path $Prefix "install.json"

function Resolve-KmcVersion {
    if (-not [string]::IsNullOrWhiteSpace($Version)) {
        if ($Version.StartsWith("v")) { return $Version }
        return "v$Version"
    }
    $Headers = @{ Accept = "application/vnd.github+json" }
    if ($Channel -eq "stable") {
        $Release = Invoke-RestMethod -Headers $Headers -Uri "$ApiUrl/repos/$Repository/releases/latest"
    } else {
        $Release = Invoke-RestMethod -Headers $Headers -Uri "$ApiUrl/repos/$Repository/releases?per_page=100" |
            Where-Object { $_.prerelease -and -not $_.draft } |
            Select-Object -First 1
    }
    if (-not $Release -or [string]::IsNullOrWhiteSpace($Release.tag_name)) {
        Stop-Installer "no $Channel release is available"
    }
    return [string]$Release.tag_name
}

function Add-KmcToPath {
    $UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $PathParts = @($UserPath -split ";" | Where-Object { $_ })
    if ($PathParts -contains $Prefix) { return }
    if (-not $ModifyPath) {
        Write-Status "Add this directory to your user PATH: $Prefix"
        return
    }
    [Environment]::SetEnvironmentVariable("Path", ((@($PathParts) + $Prefix) -join ";"), "User")
    if (($env:Path -split ";") -notcontains $Prefix) { $env:Path = "$Prefix;$env:Path" }
    Write-Status "Added $Prefix to your user PATH. New terminals will pick it up."
}

function Install-Kmc {
    $ResolvedVersion = Resolve-KmcVersion
    $PlainVersion = $ResolvedVersion.TrimStart("v")
    $Asset = "kmc_${PlainVersion}_windows_${Arch}.zip"
    $BaseUrl = "$DownloadUrl/$Repository/releases/download/$ResolvedVersion"
    $TempDir = Join-Path ([System.IO.Path]::GetTempPath()) ("kmc-install-" + [guid]::NewGuid())
    New-Item -ItemType Directory -Path $TempDir | Out-Null
    try {
        $Archive = Join-Path $TempDir $Asset
        $Checksums = Join-Path $TempDir "kmc_checksums.txt"
        Write-Status "Downloading kmc $ResolvedVersion ($Channel) for windows/$Arch..."
        Invoke-WebRequest -UseBasicParsing -Uri "$BaseUrl/$Asset" -OutFile $Archive
        Invoke-WebRequest -UseBasicParsing -Uri "$BaseUrl/kmc_checksums.txt" -OutFile $Checksums
        $ChecksumLine = Get-Content -LiteralPath $Checksums |
            Where-Object { $_ -match "^[0-9a-fA-F]{64}\s+\*?$([regex]::Escape($Asset))$" } |
            Select-Object -First 1
        if (-not $ChecksumLine) { Stop-Installer "checksum for $Asset is missing" }
        $Expected = ($ChecksumLine -split "\s+")[0].ToLowerInvariant()
        $Actual = (Get-FileHash -Algorithm SHA256 -LiteralPath $Archive).Hash.ToLowerInvariant()
        if ($Actual -ne $Expected) { Stop-Installer "SHA-256 verification failed for $Asset" }
        if ($VerifySignature) {
            if (-not (Get-Command gh.exe -ErrorAction SilentlyContinue)) {
                Stop-Installer "gh is required for -VerifySignature"
            }
            & gh.exe attestation verify $Archive --repo $Repository | Out-Null
            if ($LASTEXITCODE -ne 0) {
                Stop-Installer "GitHub artifact attestation verification failed for $Asset"
            }
        }

        $Unpacked = Join-Path $TempDir "unpacked"
        Expand-Archive -LiteralPath $Archive -DestinationPath $Unpacked
        $DownloadedBinary = Join-Path $Unpacked "kmc.exe"
        if (-not (Test-Path -LiteralPath $DownloadedBinary -PathType Leaf)) {
            Stop-Installer "release archive does not contain kmc.exe"
        }
        & $DownloadedBinary --version | Out-Null
        if ($LASTEXITCODE -ne 0) { Stop-Installer "downloaded binary did not pass its version check" }

        New-Item -ItemType Directory -Force -Path $Prefix | Out-Null
        $Staged = Join-Path $Prefix ".kmc.new.$PID.exe"
        Copy-Item -LiteralPath $DownloadedBinary -Destination $Staged -Force
        if (Test-Path -LiteralPath $KmcCommand) {
            $Backup = Join-Path $Prefix ".kmc.backup.$PID.exe"
            [System.IO.File]::Replace($Staged, $KmcCommand, $Backup)
            Remove-Item -LiteralPath $Backup -Force -ErrorAction SilentlyContinue
        } else {
            Move-Item -LiteralPath $Staged -Destination $KmcCommand
        }
        & $KmcCommand channel set $Channel | Out-Null
        if ($LASTEXITCODE -ne 0) {
            Stop-Installer "installed binary could not save the $Channel update channel"
        }
        @{
            version = $ResolvedVersion
            channel = $Channel
            os = "windows"
            arch = $Arch
        } | ConvertTo-Json | Set-Content -LiteralPath $StateFile -Encoding utf8
        Add-KmcToPath
        Write-Status "kmc $ResolvedVersion installed at $KmcCommand"
    } finally {
        Remove-Item -LiteralPath $TempDir -Recurse -Force -ErrorAction SilentlyContinue
    }
}

switch ($Command) {
    { $_ -in "install", "update" } { Install-Kmc }
    "uninstall" {
        if (Test-Path -LiteralPath $KmcCommand) {
            Remove-Item -LiteralPath $KmcCommand -Force
            Write-Status "Removed $KmcCommand"
        } else {
            Write-Status "kmc is not installed at $KmcCommand"
        }
        Remove-Item -LiteralPath $StateFile -Force -ErrorAction SilentlyContinue
    }
    "doctor" {
        Write-Status "Repository: $Repository"
        Write-Status "Platform: windows/$Arch"
        Write-Status "Channel: $Channel"
        Write-Status "Prefix: $Prefix"
        if (Test-Path -LiteralPath $KmcCommand -PathType Leaf) {
            Write-Status "kmc: $((& $KmcCommand --version).Trim()) ($KmcCommand)"
        } elseif ($FoundKmc = Get-Command kmc.exe -ErrorAction SilentlyContinue) {
            Write-Status "kmc: $((& $FoundKmc.Source --version).Trim()) ($($FoundKmc.Source))"
        } else {
            Stop-Installer "kmc is not installed"
        }
        if (Test-Path -LiteralPath $StateFile) { Write-Status "Installer metadata: $StateFile" }
        if (($env:Path -split ";") -contains $Prefix) { Write-Status "PATH: ready" }
        else { Write-Status "PATH: $Prefix is missing from this terminal" }
        if ((Get-Command npm.cmd -ErrorAction SilentlyContinue) -and
            ((& npm.cmd list --global --depth=0 "@marius4lui/kmc" 2>$null) -match "@marius4lui/kmc@")) {
            Write-Status "Legacy npm installation: detected (remove manually with npm uninstall -g @marius4lui/kmc)"
        } else {
            Write-Status "Legacy npm installation: not detected"
        }
    }
}
