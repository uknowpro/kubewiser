#requires -Version 5.1
[CmdletBinding()]
param(
    [string]$BinaryPath = (Join-Path $PSScriptRoot 'kubewiser.exe'),
    [string]$InstallDir
)
$ErrorActionPreference = 'Stop'
if ($env:OS -ne 'Windows_NT') { throw 'This installer supports Windows only.' }
$source = (Resolve-Path -LiteralPath $BinaryPath).Path
$hash = (Get-FileHash -LiteralPath $source -Algorithm SHA256).Hash
if (-not $InstallDir) {
    $InstallDir = Join-Path $env:LOCALAPPDATA ('kubewiser\versions\' + $hash.Substring(0,16))
}
$InstallDir = [IO.Path]::GetFullPath($InstallDir)
if ($InstallDir.Contains(';')) { throw 'The installation path cannot contain a semicolon.' }
$primary = Join-Path $InstallDir 'kubewiser.exe'
$alternate = Join-Path $InstallDir 'kubewise.exe'
$oldUserPath = [Environment]::GetEnvironmentVariable('Path','User')
$created = @()
try {
    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    foreach ($path in @($primary,$alternate)) {
        if (Test-Path -LiteralPath $path) {
            $item = Get-Item -LiteralPath $path
            if ($item.PSIsContainer -or ($item.Attributes -band [IO.FileAttributes]::ReparsePoint)) {
                throw "Refusing to replace an existing directory or symbolic link: $path"
            }
            if ((Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash -ne $hash) {
                throw "Existing file differs; select another -InstallDir: $path"
            }
        }
    }
    if (-not (Test-Path -LiteralPath $primary)) {
        Copy-Item -LiteralPath $source -Destination $primary
        $created += $primary
    }
    if (-not (Test-Path -LiteralPath $alternate)) {
        New-Item -ItemType HardLink -Path $alternate -Target $primary | Out-Null
        $created += $alternate
    } else {
        # Verify that an existing second name really is a hardlink, not another copy.
        $targets = @((Get-Item -LiteralPath $alternate).Target)
        if (-not ($targets | Where-Object { [IO.Path]::GetFullPath($_) -ieq $primary })) {
            throw 'Existing kubewise.exe is not the expected hardlink. Choose another -InstallDir.'
        }
    }
    if ((Get-FileHash -LiteralPath $primary -Algorithm SHA256).Hash -ne $hash) {
        throw 'Installed executable checksum mismatch.'
    }
    $entries = @($oldUserPath -split ';' | Where-Object { $_ -and $_.TrimEnd('\') -ine $InstallDir.TrimEnd('\') })
    $newUserPath = (@($InstallDir) + $entries) -join ';'
    [Environment]::SetEnvironmentVariable('Path',$newUserPath,'User')
    $processEntries = @($env:Path -split ';' | Where-Object { $_ -and $_.TrimEnd('\') -ine $InstallDir.TrimEnd('\') })
    $env:Path = (@($InstallDir) + $processEntries) -join ';'
} catch {
    foreach ($path in $created) { Remove-Item -LiteralPath $path -Force -ErrorAction SilentlyContinue }
    throw
}
Write-Host "Installed one executable with two hardlink names in $InstallDir"
Write-Host 'kubewiser and kubewise are available in this PowerShell and child CMD processes.'
Write-Host 'Restart existing terminal applications to pick up the saved user PATH.'
Write-Host 'No shell profile, telemetry, download, or system PATH changes were made.'
