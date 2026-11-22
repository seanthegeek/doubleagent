#!/bin/sh
# Classify a change for the CI workflow by the files it touches, so that each
# job runs only when a file it reads changed. Reads the changed paths, one per
# line, on stdin; with the argument "all", ignores them. Prints one
# "<category>=true|false" line per category: sh-lint, python, go and all. A
# path matching no category below, or no
# path at all, sets all, and all sets every category. AGENTS.md ("Validation
# and testing") gives the rule.

all=false
sh_lint=false
python=false
go_tree=false
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

  # The lint job's shfmt, shellcheck and checkbashisms steps read every file
  # `shfmt -f .` finds, with shellcheck's configuration. An extensionless sh
  # script matches no category and runs all.
  case "$path" in
    *.sh | .shellcheckrc)
      sh_lint=true
      matched=true
      ;;
  esac

  # The lint job's ruff and pyright steps and the tests of the tools/
  # checkers and of the round pipeline, which read every file of lab/;
  # the drift checkers' tests also read the catalog, the code style
  # checker's tests the two Go files holding its reviewed long functions,
  # and tools/tests/test_release_notes.py runs the release notes script.
  case "$path" in
    tools/drift-baseline/*) ;;
    *.md) ;;
    tools/* | lab/* | mkdocs/hooks.py | ruff.toml | pyrightconfig.json | \
      internal/catalog/catalog.txt | internal/follow/confine_sites_test.go | \
      internal/follow/guard_lists_test.go | .github/release-notes.sh)
      python=true
      matched=true
      ;;
  esac

  # The Go job: the module, its vendored dependencies, every Go source file,
  # the embedded catalog, the build script the job runs twice, the licence
  # notices a test compares with vendor/, and the two pages Go tests read:
  # the output page's examples (internal/analyze/examples_test.go) and the
  # internals page's root-check sentence (internal/follow/confine_sites_test.go).
  # Every other page sets only the always-run steps below.
  case "$path" in
    go.mod | go.sum | vendor/* | *.go | internal/catalog/catalog.txt | .github/go-build.sh | \
      THIRD_PARTY_NOTICES.txt | docs/timeline-output.md | docs/internals.md)
      go_tree=true
      matched=true
      ;;
  esac

  # Files only the always-run lint steps read (markdownlint, the document
  # checks, the drift checkers, code style), or no CI job reads at all.
  case "$path" in
    *.md | .markdownlint-cli2.jsonc | tools/drift-baseline/* | .github/ISSUE_TEMPLATE/* | \
      .github/dependabot.yml | .github/workflows/pages.yml | mkdocs/mkdocs.yml | \
      requirements-docs.txt | .git-blame-ignore-revs | .gitignore | LICENSE | Makefile)
      matched=true
      ;;
  esac

  [ "$matched" = true ] || all=true
done

[ "$count" -gt 0 ] || all=true
if [ "$all" = true ]; then
  sh_lint=true
  python=true
  go_tree=true
fi

printf 'sh-lint=%s\npython=%s\ngo=%s\nall=%s\n' "$sh_lint" "$python" "$go_tree" "$all"
