# make-project.ps1: create the round's sample project in the Windows lab VM,
# %USERPROFILE%\lab\proj, with the same files, commit and origin as
# make-project.sh writes on Linux and macOS. Windows PowerShell 5.1; files
# are UTF-8 without a byte-order mark and end lines with LF, as on the
# other platforms.

$ErrorActionPreference = 'Stop'
$project = Join-Path $env:USERPROFILE 'lab\proj'
$remote = 'https://github.com/infosecdemo42/logsleuth.git'
if ($env:LAB_REMOTE) { $remote = $env:LAB_REMOTE }
if (Test-Path -LiteralPath $project) { Write-Error "make-project: $project exists"; exit 1 }
New-Item -ItemType Directory -Force -Path $project | Out-Null
Set-Location -LiteralPath $project
$encoding = New-Object System.Text.UTF8Encoding($false)
$readme = "# lab proj`n`n" +
    "A small calculator used to exercise coding agents in the DoubleAgent lab.`n`n" +
    "Run ``python3 calc.py 2 3`` to add two numbers.`n"
$calc = "import sys`n`n`ndef add(a, b):`n    return a - b  # bug: should add`n`n`n" +
    "def main(argv):`n    a, b = (int(x) for x in argv[1:3])`n    print(add(a, b))`n`n`n" +
    "if __name__ == '__main__':`n    main(sys.argv)`n"
[IO.File]::WriteAllText((Join-Path $project 'README.md'), $readme, $encoding)
[IO.File]::WriteAllText((Join-Path $project 'calc.py'), $calc, $encoding)
[IO.File]::WriteAllText((Join-Path $project '.gitignore'), "__pycache__/`n*.pyc`n", $encoding)
git init -q -b main .
git config user.name 'Lab User'
git config user.email 'lab@example.invalid'
git add -A
git commit -q -m 'Initial calculator'
git remote add origin $remote
git log --oneline
