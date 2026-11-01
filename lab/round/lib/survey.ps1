# survey.ps1: one listing of the Windows lab VM for a real-install round, the
# counterpart of survey.sh, for Windows PowerShell 5.1. round.sh pipes the
# agent's roots file to it over ssh:
#
#   ssh guest powershell -NoProfile -ExecutionPolicy Bypass -File C:\Users\<user>\lab\survey.ps1
#
# A roots line is a path (`~` for the profile, `<tmp>` for %TEMP%,
# environment variables such as %LOCALAPPDATA% expanded, `/` read as `\`,
# wildcards allowed), optionally after `windows:`; lines for `linux:`,
# `macos:` and `unix:` are skipped. `keychain:<name>` looks the name up in
# Credential Manager (`cmdkey /list`), `process:<name>` counts processes and
# `skip:<path>` keeps a directory's entry but none below it.
# Output is survey.sh's: <kind> <size> <mode> <mtime> <path>, tab-separated,
# sorted by path, then the `top` lines, then the state lines. Reparse points
# are listed with their target and never followed. Nothing is read from a
# file but its directory entry.

[Diagnostics.CodeAnalysis.SuppressMessageAttribute('PSUseCompatibleCommands', '',
    Justification = 'The compatibility profiles list cmdlets; cmdkey is a program.')]
[Diagnostics.CodeAnalysis.SuppressMessageAttribute('PSUseCompatibleCmdlets', '',
    Justification = 'The compatibility profiles list cmdlets; cmdkey is a program.')]
param()

$ErrorActionPreference = 'SilentlyContinue'
$userProfile = $env:USERPROFILE
$tempDir = $env:TEMP.TrimEnd('\')

function Format-Entry($item) {
    $kind = 'file'
    if ($item.Attributes -band [IO.FileAttributes]::ReparsePoint) { $kind = 'link' }
    elseif ($item.PSIsContainer) { $kind = 'dir' }
    $size = '-'
    if (-not $item.PSIsContainer) { $size = $item.Length }
    $mtime = $item.LastWriteTimeUtc.ToString('yyyy-MM-ddTHH:mm:ss.fffZ')
    $name = $item.FullName
    if ($kind -eq 'link' -and $item.Target) { $name = "$name -> $(@($item.Target) -join ';')" }
    "$kind`t$size`t$($item.Mode)`t$mtime`t$name"
}

# The root itself and everything below it; Get-ChildItem -Recurse on 5.1 does
# not descend into a reparse point, so a junction's children are not listed.
function Get-RootListing($root) {
    $self = Get-Item -LiteralPath $root -Force
    if (-not $self) { return }
    Format-Entry $self
    if ($self.PSIsContainer -and -not ($self.Attributes -band [IO.FileAttributes]::ReparsePoint)) {
        Get-ChildItem -LiteralPath $root -Recurse -Force | ForEach-Object { Format-Entry $_ }
    }
}

# A roots line for this platform with `~`, `<tmp>` and %VARIABLES% expanded,
# or $null for a comment, a blank line or another platform's line.
function Resolve-RootLine([string]$line) {
    $text = $line.Trim()
    if ($text -eq '' -or $text.StartsWith('#')) { return $null }
    foreach ($prefix in @('linux:', 'macos:', 'unix:')) {
        if ($text.StartsWith($prefix)) { return $null }
    }
    if ($text.StartsWith('windows:')) { $text = $text.Substring(8) }
    if ($text.StartsWith('keychain:') -or $text.StartsWith('process:')) { return $text }
    $prefix = ''
    if ($text.StartsWith('skip:')) { $prefix = 'skip:'; $text = $text.Substring(5) }
    if ($text -eq '~' -or $text.StartsWith('~/')) { $text = $userProfile + $text.Substring(1) }
    if ($text -eq '<tmp>' -or $text.StartsWith('<tmp>/')) { $text = $tempDir + $text.Substring(5) }
    $prefix + [Environment]::ExpandEnvironmentVariables($text).Replace('/', '\')
}

function Get-ProcessCount([string]$name) {
    @(Get-Process -Name $name -ErrorAction SilentlyContinue).Count
}

$rootLines = @([Console]::In.ReadToEnd() -split "`r?`n")
$entries = @()
$stateLines = @()
$skipPaths = @()
foreach ($line in $rootLines) {
    $root = Resolve-RootLine $line
    if ($null -eq $root) { continue }
    if ($root.StartsWith('keychain:')) {
        $wanted = $root.Substring(9)
        $found = cmdkey /list 2>$null | Select-String -SimpleMatch $wanted
        $state = 'absent'
        if ($found) { $state = 'present' }
        $stateLines += "keychain`t$state`t-`t-`t$wanted"
        continue
    }
    if ($root.StartsWith('skip:')) {
        $skipPaths += $root.Substring(5).TrimEnd('\')
        continue
    }
    if ($root.StartsWith('process:')) {
        $wanted = $root.Substring(8)
        $stateLines += "process`t$(Get-ProcessCount $wanted)`t-`t-`t$wanted"
        continue
    }
    $matched = @(Get-Item -Path $root -Force)
    foreach ($item in $matched) { $entries += Get-RootListing $item.FullName }
}
$entries += Get-RootListing "$userProfile\lab\proj"
$entries | Where-Object {
    $path = ($_ -split "`t")[4]
    -not (@($skipPaths | Where-Object { $path.StartsWith($_ + '\') }).Count)
} | Sort-Object { ($_ -split "`t")[4] } -Unique

$topDirectories = @($userProfile, $env:LOCALAPPDATA, $env:APPDATA, $tempDir,
    "$env:LOCALAPPDATA\Programs", "$env:LOCALAPPDATA\Packages", "$userProfile\.config",
    "$userProfile\.local\share", "$userProfile\.cache")
$topLines = foreach ($directory in $topDirectories) {
    if (Test-Path -LiteralPath $directory) {
        Get-ChildItem -LiteralPath $directory -Force |
            ForEach-Object { "top`t" + (Format-Entry $_) }
    }
}
$topLines | Sort-Object { ($_ -split "`t")[5] } -Unique
$stateLines
