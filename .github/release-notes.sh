#!/bin/sh
# Print the release notes of one version: the body of its "## [VERSION] -
# YYYY-MM-DD" section of the changelog, then the file's link reference
# definitions, so a reference such as [#12] still resolves in the release.
# A relative link target, inline or in a definition, is made absolute against
# BASE_URL, the URL of the changelog's directory at the tagged commit, since
# a release page has no directory of its own. Fails when VERSION is not a version number (1.2.3 or
# 1.2.3-rc.1) or the changelog has no dated, non-empty section for it.
set -eu
version=${1:?usage: release-notes.sh VERSION CHANGELOG BASE_URL}
changelog=${2:?usage: release-notes.sh VERSION CHANGELOG BASE_URL}
base_url=${3:?usage: release-notes.sh VERSION CHANGELOG BASE_URL}
if ! printf '%s\n' "$version" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$'; then
  printf 'release-notes.sh: %s is not a version such as 1.2.3 or 1.2.3-rc.1\n' "$version" >&2
  exit 1
fi
# The awk program keeps the lines after the version's heading up to the next
# "## " heading or the first link reference definition, then prints them with
# every definition of the file. It exits 2 when the heading is missing and 3
# when the section holds nothing.
status=0
awk -v version="$version" -v base="$base_url" '
  function relative(target) {
    return target != "" && target !~ /^[a-z][a-z0-9+.-]*:/ && target !~ /^#/
  }
  function absolute(line,    out, rest, start, target) {
    out = ""
    rest = line
    while ((start = index(rest, "](")) > 0) {
      out = out substr(rest, 1, start + 1)
      rest = substr(rest, start + 2)
      target = substr(rest, 1, index(rest, ")") - 1)
      if (relative(target))
        out = out base
    }
    return out rest
  }
  function definition(line,    at) {
    at = index(line, "]: ")
    if (relative(substr(line, at + 3)))
      return substr(line, 1, at + 2) base substr(line, at + 3)
    return line
  }
  /^\[/ && index($0, "]: ") > 2 {
    definitions = definitions definition($0) "\n"
    if (inside) inside = 0
    next
  }
  /^## / {
    if (inside) { inside = 0; done = 1 }
    heading = "## [" version "] - "
    if (!done && substr($0, 1, length(heading)) == heading &&
        substr($0, length(heading) + 1) ~ /^[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]$/) {
      inside = 1; found = 1
    }
    next
  }
  inside {
    body = body absolute($0) "\n"
    if ($0 != "") nonblank = 1
  }
  END {
    if (!found) exit 2
    if (!nonblank) exit 3
    sub(/^\n+/, "", body)
    sub(/\n+$/, "\n", body)
    printf "%s", body
    if (definitions != "") printf "\n%s", definitions
  }
' "$changelog" || status=$?
case $status in
  0) ;;
  2)
    printf 'release-notes.sh: %s has no "## [%s] - YYYY-MM-DD" section\n' "$changelog" "$version" >&2
    exit 1
    ;;
  3)
    printf 'release-notes.sh: the [%s] section of %s is empty\n' "$version" "$changelog" >&2
    exit 1
    ;;
  *) exit "$status" ;;
esac
