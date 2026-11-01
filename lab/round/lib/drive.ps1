# drive.ps1: the guest half of a round's terminal driver on Windows, the
# counterpart of drive.sh for Windows PowerShell 5.1, with the same verbs:
#
#   drive.ps1 SESSION start COLS ROWS DIR   < the command line; DIR below the profile
#   drive.ps1 SESSION keys KEY[,KEY...]
#   drive.ps1 SESSION type                  < text, typed without Enter
#   drive.ps1 SESSION paste FILE            FILE (below the lab directory) as one paste
#   drive.ps1 SESSION screen | wait-idle QUIET MAX | wait-exit MAX | log | stop
#   drive.ps1 warmup selftest
#
# tui-test has no Windows ARM64 build, so the x86-64 binary runs under
# Windows 11's emulation, and the agent runs in a Windows PowerShell 5.1
# started by it.
param(
    [Parameter(Mandatory = $true)][string]$Session,
    [Parameter(Mandatory = $true)][string]$Verb,
    [Parameter(ValueFromRemainingArguments = $true)][string[]]$Rest
)

$ErrorActionPreference = 'Stop'
$labDir = Join-Path $env:USERPROFILE 'lab'
$tui = Join-Path $labDir 'tui-test\tui-test.exe'
# tui-test keeps a session's asciicast only when its config names a
# recording directory and turns tracing on; relative directories resolve
# beside the file.
$config = Join-Path $labDir 'tui-test\tui-test.toml'
if (-not (Test-Path -LiteralPath $tui)) { Write-Error "drive: $tui is missing"; exit 2 }

function Read-StandardInput { [Console]::In.ReadToEnd() }

function Start-AgentSession([string]$cols, [string]$rows, [string]$directory) {
    $commandLine = (Read-StandardInput).Trim()
    if ($commandLine -eq '') { Write-Error 'drive: no command line on stdin'; exit 2 }
    $runArguments = @('--session', $Session, 'run', '--config', $config, '--cols', $cols,
        '--rows', $rows, '--cwd', $directory, '--',
        'powershell.exe', '-NoLogo', '-Command', $commandLine)
    & $tui @runArguments
    exit $LASTEXITCODE
}

function Send-Keys([string]$keyList) {
    foreach ($key in $keyList.Split(',')) {
        & $tui --session $Session key press $key
        if ($LASTEXITCODE -ne 0) { exit 1 }
        Start-Sleep -Seconds 1
    }
}

function Send-Paste([string]$file) {
    $path = Join-Path $labDir $file
    if (-not (Test-Path -LiteralPath $path)) { Write-Error "drive: no paste file $path"; exit 2 }
    $content = [IO.File]::ReadAllText($path).Replace("`r`n", "`r").Replace("`n", "`r")
    $escape = [char]27
    & $tui --session $Session write "$escape[200~$content$escape[201~"
    exit $LASTEXITCODE
}

# Succeed once the screen has not changed for $quiet seconds while the
# marker matches and the busy pattern does not; give up after $limit.
function Wait-Idle([int]$quiet, [int]$limit) {
    $patterns = @((Read-StandardInput) -split "`r?`n")
    $marker = ''
    $busy = ''
    if ($patterns.Count -gt 0) { $marker = $patterns[0] }
    if ($patterns.Count -gt 1) { $busy = $patterns[1] }
    $still = 0
    $last = $null
    for ($second = 0; $second -lt $limit; $second++) {
        $screen = (& $tui --session $Session text 2>$null) -join "`n"
        if ($screen -eq $last) { $still++ } else { $still = 0; $last = $screen }
        $ready = $true
        if ($marker -ne '' -and $screen -notmatch $marker) { $ready = $false }
        if ($busy -ne '' -and $screen -match $busy) { $ready = $false }
        if ($ready -and $still -ge $quiet) { exit 0 }
        Start-Sleep -Seconds 1
    }
    exit 1
}

function Stop-AgentSession {
    foreach ($attempt in @('key:Ctrl+C', 'key:Ctrl+C', 'kill:')) {
        $kind, $value = $attempt.Split(':')
        if ($kind -eq 'kill') { & $tui --session $Session kill 2>$null | Out-Null; break }
        & $tui --session $Session key press $value 2>$null | Out-Null
        & $tui --session $Session wait exit --timeout 5000 2>$null | Out-Null
        if ($LASTEXITCODE -eq 0) { break }
    }
    & $tui --session $Session close 2>$null | Out-Null
}

switch ($Verb) {
    'start' { Start-AgentSession $Rest[0] $Rest[1] (Join-Path $env:USERPROFILE $Rest[2]) }
    'keys' { Send-Keys $Rest[0] }
    'type' {
        & $tui --session $Session type ((Read-StandardInput).TrimEnd("`r", "`n"))
        exit $LASTEXITCODE
    }
    'paste' { Send-Paste $Rest[0] }
    'screen' { & $tui --session $Session text; exit $LASTEXITCODE }
    'wait-idle' { Wait-Idle ([int]$Rest[0]) ([int]$Rest[1]) }
    'wait-exit' {
        & $tui --session $Session wait exit --timeout ([int]$Rest[0] * 1000)
        exit $LASTEXITCODE
    }
    'log' { & $tui --session $Session get-recording --config $config; exit $LASTEXITCODE }
    'stop' { Stop-AgentSession }
    'selftest' {
        & $tui --version
        $settings = "[recording]`ndirectory = `"recordings`"`n`n" +
            "[trace]`nmode = `"on`"`ndirectory = `"traces`"`n"
        [IO.File]::WriteAllText($config, $settings, (New-Object System.Text.UTF8Encoding($false)))
        $readyCommand = "Write-Output 'drive ready'; Start-Sleep -Seconds 30"
        $readyArguments = @('--session', $Session, 'run', '--config', $config, '--cols', '80',
            '--rows', '24', '--', 'powershell.exe', '-NoLogo', '-Command', $readyCommand)
        & $tui @readyArguments
        & $tui --session $Session expect text 'drive ready' --timeout 10000 | Out-Null
        $code = $LASTEXITCODE
        & $tui --session $Session close 2>$null | Out-Null
        exit $code
    }
    default { Write-Error "drive: unknown verb $Verb"; exit 2 }
}
