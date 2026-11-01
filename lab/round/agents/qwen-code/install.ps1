# install.ps1: install Qwen Code in the Windows lab VM with the standalone
# installer the vendor's README gives for Windows under "Installation", then
# print the `source:` and `bin:` lines round.sh records. Windows PowerShell
# 5.1. The version line is left to the first launch, since where the
# installer puts its package on Windows is not yet known.
[Diagnostics.CodeAnalysis.SuppressMessageAttribute('PSAvoidUsingCmdletAliases', '',
    Justification = 'The vendor command is copied as the README prints it.')]
[Diagnostics.CodeAnalysis.SuppressMessageAttribute('PSAvoidUsingInvokeExpression', '',
    Justification = 'The vendor installer is run the way the README runs it.')]
param()

$ErrorActionPreference = 'Stop'
$sourcePage = 'https://github.com/QwenLM/qwen-code/blob/main/README.md#installation'
$sourceFetched = '2026-11-01'
"source: $sourcePage (fetched $sourceFetched)"
irm https://qwen-code-assets.oss-cn-hangzhou.aliyuncs.com/installation/install-qwen-standalone.ps1 | iex
$command = Get-Command qwen -ErrorAction SilentlyContinue
if ($command) { "bin: $($command.Source)" }
