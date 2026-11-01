#!/bin/sh
# Classify a change for the CI workflow by the files it touches, so that each
# job runs only when a file it reads changed. Reads the changed paths, one per
# line, on stdin; with the argument "all", ignores them. Prints one
# "<category>=true|false" line per category: sh, sh-lint, powershell, python,
# catalog and all. A path matching no category below, or no path at all, sets
# all, and all sets every category. AGENTS.md ("Validation and testing")
# gives the rule.

all=false
sh_collector=false
sh_lint=false
powershell=false
python=false
catalog=false
[ "${1:-}" = all ] && all=true
count=0

while IFS= read -r path; do
  [ -n "$path" ] || continue
  count=$((count + 1))
  matched=false

  case "$path" in
    # This script and the workflow decide what runs; .gitattributes sets the
    # line endings of every checked-out file.
    .github/workflows/ci.yml | .github/classify-changes.sh | .gitattributes)
      all=true
      matched=true
      ;;
  esac

  # The smoke jobs, the BSD job and the collector's shellcheck job.
  case "$path" in
    collectors/collect-agent-artifacts.sh | collectors/tests/smoke.sh | \
      collectors/tests/catalog-sync.sh | .shellcheckrc)
      sh_collector=true
      matched=true
      ;;
  esac

  # The lint job's shfmt, shellcheck and checkbashisms steps read every file
  # `shfmt -f .` finds: the .sh files and the extensionless hook scripts. An
  # extensionless sh script anywhere else matches no category and runs all.
  case "$path" in
    *.sh | .githooks/*)
      sh_lint=true
      matched=true
      ;;
  esac

  case "$path" in
    collectors/Collect-AgentArtifacts.ps1 | collectors/tests/smoke.ps1 | \
      collectors/PSScriptAnalyzerSettings.psd1 | lab/round/*.ps1)
      powershell=true
      matched=true
      ;;
  esac

  # The analyzer job and the ruff and pyright steps. The analyzer tests also
  # read analyzer/docs/output.md (its generated examples) and docs/agents.md
  # (the collected agents), and run the tools/ checkers' tests and the round
  # pipeline's tests, which read every file of lab/round/.
  case "$path" in
    tools/drift-baseline/*) ;;
    analyzer/docs/output.md | docs/agents.md)
      python=true
      matched=true
      ;;
    *.md) ;;
    analyzer/* | tools/* | lab/round/* | mkdocs/hooks.py | ruff.toml | pyrightconfig.json)
      python=true
      matched=true
      ;;
  esac

  # The catalog drift job, and the analyzer tests that read the sh collector.
  case "$path" in
    collectors/collect-agent-artifacts.sh | collectors/Collect-AgentArtifacts.ps1 | \
      analyzer/doubleagent/catalog.txt | collectors/tests/catalog-sync.sh)
      catalog=true
      matched=true
      ;;
  esac

  # Files only the always-run lint steps read (markdownlint, the document
  # checks, the drift checkers, code style), or no CI job reads at all.
  case "$path" in
    *.md | .markdownlint-cli2.jsonc | tools/drift-baseline/* | .github/ISSUE_TEMPLATE/* | \
      .github/dependabot.yml | .github/workflows/pages.yml | mkdocs/mkdocs.yml | \
      requirements-docs.txt | .git-blame-ignore-revs | .gitignore | LICENSE | \
      lab/.env.example | lab/.gitignore | lab/Dockerfile | lab/compose*.yaml)
      matched=true
      ;;
  esac

  [ "$matched" = true ] || all=true
done

[ "$count" -gt 0 ] || all=true
if [ "$all" = true ]; then
  sh_collector=true
  sh_lint=true
  powershell=true
  python=true
  catalog=true
fi
[ "$sh_collector" = true ] && sh_lint=true

printf 'sh=%s\nsh-lint=%s\npowershell=%s\npython=%s\ncatalog=%s\nall=%s\n' \
  "$sh_collector" "$sh_lint" "$powershell" "$python" "$catalog" "$all"
