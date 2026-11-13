#!/bin/sh
# Copyright 2026 Sean Whalen
# SPDX-License-Identifier: Apache-2.0
#
# Fails if the artifact tables embedded in collect-agent-artifacts.sh and
# Collect-AgentArtifacts.ps1 differ, or if the analyzer's bundled copy or the
# Go binary's embedded copy of the --list output is stale. All of them must stay a
# single source of truth.
HERE=$(cd "$(dirname "$0")" && pwd)
SH_COLLECTOR="$HERE/../collect-agent-artifacts.sh"
PS_COLLECTOR="$HERE/../Collect-AgentArtifacts.ps1"
ANALYZER_COPY="$HERE/../../analyzer/doubleagent/catalog.txt"
GO_COPY="$HERE/../../internal/catalog/catalog.txt"
fail=0

sh_table() { # sh_table NAME -> lines of the sh table (blank lines dropped)
  if [ "$1" = SECRET_GLOBS ]; then
    sed -n "/^SECRET_GLOBS='/,/'\$/p" "$SH_COLLECTOR" | sed "1s/^SECRET_GLOBS='//; \$s/'\$//"
  else
    sed -n "/^$1='\$/,/^'\$/p" "$SH_COLLECTOR" | sed '1d;$d'
  fi | grep -v '^$'
}
ps_table() { # ps_table NAME -> lines of the PowerShell here-string
  sed -n "/^\$$1 = @'/,/^'@/p" "$PS_COLLECTOR" | sed '1d;$d' | tr -d '\r' | grep -v '^$'
}

for table in CATALOG PROJECT_CATALOG EXCLUDES SECRET_GLOBS DOCKER_VOLUMES RELOCATION_VARS \
  STARTUP_FILES; do
  sh_lines=$(sh_table "$table"); ps_lines=$(ps_table "$table")
  line_count=$(printf '%s\n' "$sh_lines" | grep -c .)
  if [ "$line_count" -lt 5 ]; then printf 'FAIL %s: could not extract table from sh (%s lines)\n' "$table" "$line_count"; fail=1; continue; fi
  if [ "$sh_lines" = "$ps_lines" ]; then
    printf 'ok   %s (%s lines)\n' "$table" "$line_count"
  else
    printf 'FAIL %s differs:\n' "$table"; fail=1
    printf '%s\n' "$sh_lines" >"${TMPDIR:-/tmp}/cac-sync-sh.$$"; printf '%s\n' "$ps_lines" >"${TMPDIR:-/tmp}/cac-sync-ps.$$"
    diff "${TMPDIR:-/tmp}/cac-sync-sh.$$" "${TMPDIR:-/tmp}/cac-sync-ps.$$" | head -20
    rm -f "${TMPDIR:-/tmp}/cac-sync-sh.$$" "${TMPDIR:-/tmp}/cac-sync-ps.$$"
  fi
done

# The sh collector joins SECRET_GLOBS into one case pattern
# (compile_secret_pattern), with only spaces escaped, so a glob holding a
# character the pattern syntax or eval would read (|, parentheses, &, ;, <, >,
# $, a backtick, a quote or a backslash), or starting with #, which begins a
# comment after the | before it, would break every secret test.
bad=$(sh_table SECRET_GLOBS | grep -e '[|()&;<>$`"'"'"'\\]' -e '^#')
if [ -z "$bad" ]; then printf 'ok   SECRET_GLOBS free of case-pattern specials\n'
else printf 'FAIL SECRET_GLOBS entries with characters the compiled case pattern cannot hold:\n%s\n' "$bad"; fail=1; fi

# The --list / -List outputs must also match when a PowerShell is available.
PWSH=
if command -v pwsh >/dev/null 2>&1; then PWSH=pwsh; fi
if [ -n "$PWSH" ]; then
  if [ "$("$SH_COLLECTOR" --list)" = "$($PWSH -NoProfile -File "$PS_COLLECTOR" -List | tr -d '\r')" ]; then printf 'ok   --list output identical under %s\n' "$PWSH"
  else printf 'FAIL --list output differs under %s\n' "$PWSH"; fail=1; fi
fi

# The analyzer detects agents with a verbatim copy of the --list output.
if [ -f "$ANALYZER_COPY" ]; then
  if [ "$("$SH_COLLECTOR" --list)" = "$(cat "$ANALYZER_COPY")" ]; then printf 'ok   analyzer catalog.txt matches --list\n'
  else
    printf 'FAIL analyzer catalog.txt is stale; run: collectors/collect-agent-artifacts.sh --list > analyzer/doubleagent/catalog.txt\n'; fail=1
  fi
fi

# The Go binary embeds a verbatim copy of the --list output, compared byte for
# byte (command substitution would drop trailing newlines).
if [ -f "$GO_COPY" ]; then
  if "$SH_COLLECTOR" --list | cmp -s - "$GO_COPY"; then printf 'ok   internal/catalog/catalog.txt matches --list\n'
  else
    printf 'FAIL internal/catalog/catalog.txt is stale; run: collectors/collect-agent-artifacts.sh --list > internal/catalog/catalog.txt\n'; fail=1
  fi
  if cmp -s "$GO_COPY" "$ANALYZER_COPY"; then printf 'ok   internal/catalog/catalog.txt matches the analyzer copy\n'
  else
    printf 'FAIL internal/catalog/catalog.txt differs from analyzer/doubleagent/catalog.txt\n'; fail=1
  fi
fi

[ "$fail" = 0 ] && echo "CATALOGS IN SYNC"
exit "$fail"
