#Requires -Version 5.1
<#
.SYNOPSIS
    Remove Moodle CLI from Windows.

.DESCRIPTION
    The binary goes by default. Configuration and stored credentials stay
    unless -Purge is given, because uninstalling is often one step of an
    upgrade and silently discarding a login would be a poor trade to make on
    someone's behalf.

.PARAMETER InstallDir
    Directory the binary was installed into. Defaults to
    %LOCALAPPDATA%\Programs\moodle-cli.

.PARAMETER Purge
    Also remove configuration and anything stored in Credential Manager.

.PARAMETER DryRun
    Report what would be removed and change nothing.

.EXAMPLE
    .\uninstall.ps1

.EXAMPLE
    .\uninstall.ps1 -Purge
#>
[CmdletBinding()]
param(
    [string]$InstallDir = (Join-Path $env:LOCALAPPDATA 'Programs\moodle-cli'),
    [switch]$Purge,
    [switch]$DryRun
)

$ErrorActionPreference = 'Stop'

# The keychain service and the configuration directory are both the project
# name; the command itself is moodle.
$Service = 'moodle-cli'

function Remove-Target {
    param([string]$Path, [string]$Description)
    if (-not (Test-Path -LiteralPath $Path)) { return }
    if ($DryRun) {
        Write-Host "would remove $Description $Path"
        return
    }
    Remove-Item -LiteralPath $Path -Recurse -Force
    Write-Host "removed $Description $Path"
}

function Remove-PathEntry {
    param([string]$Directory)
    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    if (-not $userPath) { return }
    $entries = $userPath -split ';' | Where-Object { $_ -ne '' }
    if ($entries -notcontains $Directory) { return }
    if ($DryRun) {
        Write-Host "would remove $Directory from your user PATH"
        return
    }
    $kept = $entries | Where-Object { $_ -ne $Directory }
    [Environment]::SetEnvironmentVariable('Path', ($kept -join ';'), 'User')
    Write-Host "removed $Directory from your user PATH"
}

# Credentials are in Windows Credential Manager rather than a file, stored
# under a target of the form service:account, so they are enumerated and
# removed by prefix instead of guessed at.
function Remove-StoredCredentials {
    $targets = @()
    foreach ($line in (cmdkey /list 2>$null)) {
        if ($line -match 'Target:\s*(.+)$') {
            $target = $Matches[1].Trim()
            if ($target -like "*$Service`:*") { $targets += $target }
        }
    }
    $targets = $targets | Select-Object -Unique
    if ($targets.Count -eq 0) {
        Write-Host 'no stored credentials found'
        return
    }
    foreach ($target in $targets) {
        if ($DryRun) {
            Write-Host "would remove credential $target"
            continue
        }
        cmdkey /delete:$target | Out-Null
        Write-Host "removed credential $target"
    }
}

Remove-Target -Path $InstallDir -Description 'install directory'
Remove-PathEntry -Directory $InstallDir

if ($Purge) {
    # config.yaml, and credentials.json for anyone who chose the file store.
    Remove-Target -Path (Join-Path $env:APPDATA $Service) -Description 'configuration'
    Remove-StoredCredentials
    Write-Host ''
    Write-Host 'Tokens are not revoked on the Moodle side: the same token is often the'
    Write-Host "one your phone's Moodle app holds. To revoke it, use the site's own"
    Write-Host '"Security keys" page in your profile.'
} else {
    Write-Host ''
    Write-Host 'Configuration and credentials were kept. Pass -Purge to remove them too.'
}
