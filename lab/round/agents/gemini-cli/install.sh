#!/bin/sh
# install.sh: install Gemini CLI in a Linux or macOS lab VM with the command
# the vendor's README gives under "Install globally with npm", then print
# the `source:`, `bin:` and `version:` lines round.sh records.

set -eu
source_page='https://github.com/google-gemini/gemini-cli/blob/main/README.md#install-globally-with-npm'
source_fetched='2026-11-01'
binary=gemini
printf 'source: %s (fetched %s)\n' "$source_page" "$source_fetched"
printf 'node: %s\n' "$(node --version)"
prefix=$(npm prefix -g)
# A Node from the distribution's archive keeps global packages under /usr,
# which only root may write; the vendor's command then runs under sudo.
if [ -w "$prefix/lib/node_modules" ] || [ -w "$prefix" ]; then
  npm install -g @google/gemini-cli
else
  sudo -n npm install -g @google/gemini-cli
fi
printf 'bin: %s\n' "$prefix/bin/$binary"
# The version comes from npm's record, not `gemini --version`, which
# already creates ~/.gemini and belongs to the first-launch step.
npm ls -g --depth=0 @google/gemini-cli | sed -n 's/.*@google\/gemini-cli@/version: /p'
