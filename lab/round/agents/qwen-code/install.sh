#!/bin/sh
# install.sh: install Qwen Code in a Linux or macOS lab VM with the
# standalone installer the vendor's README gives under "Installation", piped
# to bash as the README pipes it, then print the `source:`, `bin:` and
# `version:` lines round.sh records. The installer bundles its own Node and
# puts the `qwen` launcher in ~/.local/bin.

set -eu
source_page='https://github.com/QwenLM/qwen-code/blob/main/README.md#installation'
source_fetched='2026-11-01'
printf 'source: %s (fetched %s)\n' "$source_page" "$source_fetched"
curl -fsSL https://qwen-code-assets.oss-cn-hangzhou.aliyuncs.com/installation/install-qwen-standalone.sh | bash
printf 'bin: %s\n' "$HOME/.local/bin/qwen"
# The version comes from the installed package, not `qwen --version`, which
# may already write ~/.qwen and belongs to the first-launch step.
package=$HOME/.local/lib/qwen-code/package.json
if [ -f "$package" ]; then
  sed -n 's/^[[:space:]]*"version":[[:space:]]*"\([^"]*\)".*/version: \1/p' "$package" | head -n 1
fi
