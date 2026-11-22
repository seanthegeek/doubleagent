#!/bin/sh
# make-project.sh: create the round's sample project ~/lab/proj in a Linux or
# macOS lab VM: a README, a calc.py whose add() subtracts, a .gitignore, one
# commit on main and the lab repository as origin, never pushed. Every cell
# gets the same project, so the paths and the bug the prompts name are the
# same on every platform; make-project.ps1 writes the Windows copy.

set -eu
project=$HOME/lab/proj
remote=${LAB_REMOTE:-https://github.com/infosecdemo42/logsleuth.git}
if [ -e "$project" ]; then
  printf 'make-project: %s exists\n' "$project" >&2
  exit 1
fi
mkdir -p "$project"
cd "$project" || exit 1
cat >README.md <<'EOF_README'
# lab proj

A small calculator used to exercise coding agents in the DoubleAgent lab.

Run `python3 calc.py 2 3` to add two numbers.
EOF_README
printf '%s\n' 'import sys' '' '' 'def add(a, b):' '    return a - b  # bug: should add' \
  '' '' 'def main(argv):' '    a, b = (int(x) for x in argv[1:3])' '    print(add(a, b))' \
  '' '' "if __name__ == '__main__':" '    main(sys.argv)' >calc.py
printf '__pycache__/\n*.pyc\n' >.gitignore
git init -q -b main .
git config user.name 'Lab User'
git config user.email 'lab@example.invalid'
git add -A
git commit -q -m 'Initial calculator'
git remote add origin "$remote"
git log --oneline
