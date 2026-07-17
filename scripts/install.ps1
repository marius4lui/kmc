[CmdletBinding()]
param(
    [ValidateSet("install", "update", "uninstall", "doctor")]
    [string]$Command = "install",
    [string]$Version = "latest",
    [string]$Prefix = "",
    [switch]$ModifyPath,
    [switch]$Quiet
)

$ErrorActionPreference = "Stop"
$PackageName = "@marius4lui/kmc"
$MinimumNodeMajor = 18

function Write-Status([string]$Message) {
    if (-not $Quiet) {
        Write-Host $Message
    }
}

function Stop-Installer([string]$Message) {
    throw "kmc installer: $Message"
}

function Invoke-Npm {
    param([Parameter(ValueFromRemainingArguments = $true)][string[]]$Arguments)
    & npm.cmd @Arguments
    if ($LASTEXITCODE -ne 0) {
        Stop-Installer "npm failed with exit code $LASTEXITCODE"
    }
}

if (-not (Get-Command node.exe -ErrorAction SilentlyContinue)) {
    Stop-Installer "Node.js $MinimumNodeMajor+ is required: https://nodejs.org/"
}
if (-not (Get-Command npm.cmd -ErrorAction SilentlyContinue)) {
    Stop-Installer "npm is required and normally ships with Node.js: https://nodejs.org/"
}

$NodeVersion = (& node.exe --version).Trim()
$NodeMajor = [int]($NodeVersion.TrimStart("v").Split(".")[0])
if ($NodeMajor -lt $MinimumNodeMajor) {
    Stop-Installer "Node.js $MinimumNodeMajor+ is required; found $NodeVersion"
}

if ([string]::IsNullOrWhiteSpace($Prefix)) {
    $Prefix = (& npm.cmd config get prefix).Trim()
    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($Prefix)) {
        Stop-Installer "could not determine the npm global prefix; pass -Prefix PATH"
    }
}

$Prefix = [System.IO.Path]::GetFullPath($Prefix)
$KmcCommand = Join-Path $Prefix "kmc.cmd"

function Add-KmcToPath {
    $UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $PathParts = @($UserPath -split ";" | Where-Object { $_ })
    if ($PathParts -contains $Prefix) {
        return
    }

    if (-not $ModifyPath) {
        Write-Status "Add this directory to your user PATH: $Prefix"
        return
    }

    $NewPath = (@($PathParts) + $Prefix) -join ";"
    [Environment]::SetEnvironmentVariable("Path", $NewPath, "User")
    if (($env:Path -split ";") -notcontains $Prefix) {
        $env:Path = "$Prefix;$env:Path"
    }
    Write-Status "Added $Prefix to your user PATH. New terminals will pick it up."
}

switch ($Command) {
    { $_ -in "install", "update" } {
        Write-Status "Installing $PackageName@$Version into $Prefix ..."
        Invoke-Npm install --global --prefix $Prefix "$PackageName@$Version"
        if (-not (Test-Path -LiteralPath $KmcCommand -PathType Leaf)) {
            Stop-Installer "npm completed, but $KmcCommand was not created"
        }
        Add-KmcToPath
        $InstalledVersion = (& $KmcCommand --version).Trim()
        Write-Status "Installed: $InstalledVersion"
        Write-Status "Run: kmc"
    }
    "uninstall" {
        Write-Status "Removing $PackageName from $Prefix ..."
        Invoke-Npm uninstall --global --prefix $Prefix $PackageName
        Write-Status "kmc was removed from $Prefix"
    }
    "doctor" {
        Write-Status "Node.js: $NodeVersion"
        Write-Status "npm: $((& npm.cmd --version).Trim())"
        Write-Status "Prefix: $Prefix"
        if (Test-Path -LiteralPath $KmcCommand -PathType Leaf) {
            Write-Status "kmc: $((& $KmcCommand --version).Trim()) ($KmcCommand)"
        } elseif ($FoundKmc = Get-Command kmc.cmd -ErrorAction SilentlyContinue) {
            Write-Status "kmc: $((& $FoundKmc.Source --version).Trim()) ($($FoundKmc.Source))"
        } else {
            Stop-Installer "kmc is not installed"
        }
        if (($env:Path -split ";") -contains $Prefix) {
            Write-Status "PATH: ready"
        } else {
            Write-Status "PATH: $Prefix is missing from this terminal"
        }
    }
}
