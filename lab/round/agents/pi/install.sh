#!/bin/sh
# install.sh: install pi in a Linux or macOS lab VM with the command the
# vendor's README gives under "Getting started", run with sh as its shebang
# names (downloaded to a file first, so a failed download fails the install),
# then print the `source:`, `bin:` and `version:` lines round.sh
# records. Without a terminal the installer asks nothing and takes its
# defaults, so it does not add its launcher directory to PATH; when the
# launcher is not in ~/.local/bin, the PATH line it would have offered is
# appended to the shell's startup file, as a person answering yes gets.

set -eu
source_page='https://github.com/earendil-works/pi/blob/main/packages/coding-agent/README.md#getting-started'
source_fetched='2026-11-01'
printf 'source: %s (fetched %s)\n' "$source_page" "$source_fetched"
installer=$HOME/lab/install-pi.sh
curl -fsSL -o "$installer" https://pi.dev/install.sh
sh "$installer"
if [ -x "$HOME/.local/bin/pi" ]; then
  printf 'bin: %s\n' "$HOME/.local/bin/pi"
elif [ ! -x "$HOME/.pi/agent/bin/pi" ]; then
  printf 'install.sh: neither ~/.local/bin/pi nor ~/.pi/agent/bin/pi exists\n' >&2
  exit 1
else
  case $(uname -s) in
    Darwin) startup=$HOME/.zshrc ;;
    *) startup=$HOME/.bashrc ;;
  esac
  # The text is the installer's own PATH line, written for the person's shell.
  # shellcheck disable=SC2016
  printf '\n# Pi\nexport PATH="%s:$PATH"\n' "$HOME/.pi/agent/bin" >>"$startup"
  printf 'path: added %s to %s\n' "$HOME/.pi/agent/bin" "$startup"
  printf 'bin: %s\n' "$HOME/.pi/agent/bin/pi"
fi
# The version is the managed install's own record, so pi does not run here.
if [ -f "$HOME/.pi/agent/install/current-version" ]; then
  printf 'version: %s\n' "$(cat "$HOME/.pi/agent/install/current-version")"
fi
