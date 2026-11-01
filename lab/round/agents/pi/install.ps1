# install.ps1: install pi in the Windows lab VM with the command the
# vendor's README gives under "On Windows", then print the `source:`, `bin:`
# and `version:` lines round.sh records. Windows PowerShell 5.1.

[Diagnostics.CodeAnalysis.SuppressMessageAttribute('PSUseCompatibleCommands', '',
    Justification = 'The compatibility profiles list cmdlets; powershell is a program.')]
[Diagnostics.CodeAnalysis.SuppressMessageAttribute('PSUseCompatibleCmdlets', '',
    Justification = 'The compatibility profiles list cmdlets; powershell is a program.')]
param()

$ErrorActionPreference = 'Stop'
$sourcePage = 'https://github.com/earendil-works/pi/blob/main/packages/coding-agent/README.md#getting-started'
$sourceFetched = '2026-11-01'
"source: $sourcePage (fetched $sourceFetched)"
powershell -c "irm https://pi.dev/install.ps1 | iex"
if ($LASTEXITCODE -ne 0) { Write-Error "install.ps1: the installer exited $LASTEXITCODE"; exit 1 }
$agentDir = Join-Path $env:USERPROFILE '.pi\agent'
# The installer sets the user's PATH in the registry; read it back, since
# this process started before it.
$env:Path = [Environment]::GetEnvironmentVariable('Path', 'User') + ';' +
    [Environment]::GetEnvironmentVariable('Path', 'Machine')
$command = Get-Command pi -ErrorAction SilentlyContinue
$launcher = @(Get-Item -Path "$agentDir\bin\pi*" -ErrorAction SilentlyContinue)
if ($command) { "bin: $($command.Source)" }
elseif ($launcher.Count -gt 0) { "bin: $($launcher[0].FullName)" }
else { Write-Error 'install.ps1: no pi launcher after the installer'; exit 1 }
$versionFile = Join-Path $agentDir 'install\current-version'
if (Test-Path -LiteralPath $versionFile) { "version: $((Get-Content -LiteralPath $versionFile -Raw).Trim())" }
