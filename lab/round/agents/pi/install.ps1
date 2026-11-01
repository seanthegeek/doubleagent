# install.ps1: install pi in the Windows lab VM with the command the
# vendor's README gives under "On Windows", then print the `source:`, `bin:`
# and `version:` lines round.sh records. Windows PowerShell 5.1.

$ErrorActionPreference = 'Stop'
$sourcePage = 'https://github.com/earendil-works/pi/blob/main/packages/coding-agent/README.md#getting-started'
$sourceFetched = '2026-11-01'
"source: $sourcePage (fetched $sourceFetched)"
powershell -c "irm https://pi.dev/install.ps1 | iex"
$agentDir = Join-Path $env:USERPROFILE '.pi\agent'
$command = Get-Command pi -ErrorAction SilentlyContinue
if ($command) { "bin: $($command.Source)" } else { "bin: $agentDir\bin\pi" }
$versionFile = Join-Path $agentDir 'install\current-version'
if (Test-Path -LiteralPath $versionFile) { "version: $((Get-Content -LiteralPath $versionFile -Raw).Trim())" }
