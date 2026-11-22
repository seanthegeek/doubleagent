#!/bin/sh
# Print the release notes of one version: the body of its "## [VERSION] -
# YYYY-MM-DD" section of the changelog, then the file's link reference
# definitions, so a reference such as [#12] still resolves in the release.
# A relative link target, inline or in a definition, is made absolute against
# BASE_URL, the URL of the changelog's directory at the tagged commit, since
# a release page has no directory of its own; a target starting with "/",
# "#", "<" or a scheme is not relative, and code spans and fenced code blocks
# are copied unchanged. Fails when VERSION is not a version number (1.2.3 or
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
# every definition of the file; a line inside a fenced code block is neither.
# It exits 2 when the heading is missing and 3 when the section holds nothing.
status=0
awk -v version="$version" -v base="$base_url" '
  function relative(target) {
    return target != "" && target !~ /^[a-zA-Z][a-zA-Z0-9+.-]*:/ && target !~ /^[#\/<]/
  }
  # The length of the run of backticks at position at of text.
  function ticks(text, at,    count) {
    count = 0
    while (substr(text, at + count, 1) == "`") count++
    return count
  }
  # The position after the run of exactly count backticks that closes a code
  # span opened before position from, or 0 when none closes it.
  function span_end(text, from, count,    at, run) {
    for (at = from; at <= length(text); at++) {
      if (substr(text, at, 1) != "`") continue
      run = ticks(text, at)
      if (run == count) return at + run
      at += run - 1
    }
    return 0
  }
  function absolute(line,    out, at, run, after, target) {
    out = ""
    at = 1
    while (at <= length(line)) {
      if (substr(line, at, 1) == "`") {
        run = ticks(line, at)
        after = span_end(line, at + run, run)
        if (after == 0) after = at + run
        out = out substr(line, at, after - at)
        at = after
        continue
      }
      if (substr(line, at, 2) == "](") {
        out = out "]("
        at += 2
        target = substr(line, at)
        target = substr(target, 1, index(target, ")") - 1)
        if (relative(target)) out = out base
        continue
      }
      out = out substr(line, at, 1)
      at++
    }
    return out
  }
  # Whether line opens or closes a fenced code block; fence is the open
  # fence ("" outside one), and a closing fence is of its character and at
  # least its length, with nothing after it.
  function fence_line(line,    text, mark, run) {
    text = line
    sub(/^ ? ? ?/, "", text)
    mark = substr(text, 1, 1)
    if (mark != "`" && mark != "~") return 0
    run = 0
    while (substr(text, run + 1, 1) == mark) run++
    if (run < 3) return 0
    if (fence == "") {
      if (mark == "`" && index(substr(text, run + 1), "`") > 0) return 0
      fence = substr(text, 1, run)
      return 1
    }
    if (substr(fence, 1, 1) != mark || run < length(fence)) return 0
    if (substr(text, run + 1) !~ /^[ \t]*$/) return 0
    fence = ""
    return 1
  }
  function definition(line,    at) {
    at = index(line, "]: ")
    if (relative(substr(line, at + 3)))
      return substr(line, 1, at + 2) base substr(line, at + 3)
    return line
  }
  {
    was = fence
    toggled = fence_line($0)
  }
  toggled || was != "" {
    if (inside) {
      body = body $0 "\n"
      nonblank = 1
    }
    next
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
