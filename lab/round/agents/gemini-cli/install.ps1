# install.ps1: install Gemini CLI in the Windows lab VM with the command the
# vendor's README gives under "Install globally with npm", then print the
# `source:`, `bin:` and `version:` lines round.sh records. Windows
# PowerShell 5.1; npm's global prefix on Windows is the user's
# %APPDATA%\npm, so no elevation is needed.

$ErrorActionPreference = 'Stop'
$sourcePage = 'https://github.com/google-gemini/gemini-cli/blob/main/README.md#install-globally-with-npm'
$sourceFetched = '2026-11-01'
"source: $sourcePage (fetched $sourceFetched)"
"node: $(node --version)"
npm install -g @google/gemini-cli
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
$prefix = (npm prefix -g).Trim()
"bin: $prefix\gemini.cmd"
# The version comes from npm's record, not `gemini --version`, which
# already creates .gemini and belongs to the first-launch step.
$listing = (npm ls -g --depth=0 '@google/gemini-cli') -join "`n"
if ($listing -match '@google/gemini-cli@(\S+)') { "version: $($Matches[1])" }
