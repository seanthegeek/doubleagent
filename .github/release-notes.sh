#!/bin/sh
# Print the release notes of one version: the body of its "## [VERSION] -
# YYYY-MM-DD" section of the changelog, then the file's link reference
# definitions, so a reference such as [#12] still resolves in the release.
# A relative link target, inline or in a definition, is made absolute against
# BASE_URL, the URL of the changelog's directory at the tagged commit, since
# a release page has no directory of its own; a target in angle brackets is
# made absolute inside them, a target starting with "/", "#" or a scheme is
# not relative, and code spans, fenced code blocks and HTML comments are
# copied unchanged. An angle-bracket target is one only when no "<" comes
# before its ">" and a ")", space or tab follows it (a definition's may end
# the line); a backslash escape is not read. A fence is recognised at up to
# three spaces of indent only, so one inside a list item is not. Fails when
# VERSION is not a version number (1.2.3 or 1.2.3-rc.1), the changelog has no
# dated, non-empty section for it, or an HTML comment or a fenced code block
# in it never closes, which would hide the rest of the file.
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
# every definition of the file; a line inside a fenced code block or an HTML
# comment is neither.
# It exits 10 when the heading is missing, 11 when the section holds nothing,
# and 12, after saying on standard error which line opened it, when a comment
# or fence never closes; awk itself exits 2 on an error of its own.
status=0
awk -v version="$version" -v base="$base_url" '
  function relative(target) {
    return target != "" && target !~ /^[a-zA-Z][a-zA-Z0-9+.-]*:/ && target !~ /^[#\/]/
  }
  # The target at the start of text, which follows "](" or "]: ": the text
  # inside angle brackets when it opens with "<", else up to the first ")".
  # An angle-bracket target holds no "<" and is followed by one of ends,
  # the characters that may come after it, or by the end of text when ends
  # holds "$"; otherwise there is none ("").
  function target_at(text, ends,    shut, after) {
    if (substr(text, 1, 1) != "<") return substr(text, 1, index(text, ")") - 1)
    shut = index(text, ">")
    if (shut == 0 || index(substr(text, 2, shut - 2), "<") > 0) return ""
    after = substr(text, shut + 1, 1)
    if (after == "" ? index(ends, "$") == 0 : index(ends, after) == 0) return ""
    return substr(text, 2, shut - 2)
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
        target = target_at(substr(line, at), ") \t")
        if (relative(target)) {
          if (substr(line, at, 1) == "<") { out = out "<"; at++ }
          out = out base
        }
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
      opened = NR
      return 1
    }
    if (substr(fence, 1, 1) != mark || run < length(fence)) return 0
    if (substr(text, run + 1) !~ /^[ \t]*$/) return 0
    fence = ""
    return 1
  }
  # Whether line opens or continues an HTML comment, which runs from a line
  # starting "<!--" at up to three spaces of indent to the first line holding
  # "-->" after it; comment is set while the comment is open after line.
  function comment_line(line,    text) {
    if (comment) {
      if (index(line, "-->") > 0) comment = 0
      return 1
    }
    if (line !~ /^ ? ? ?<!--/) return 0
    text = line
    sub(/^ ? ? ?<!--/, "", text)
    if (index(text, "-->") == 0) { comment = 1; opened = NR }
    return 1
  }
  function definition(line,    at, rest) {
    at = index(line, "]: ")
    rest = substr(line, at + 3)
    if (!relative(rest ~ /^</ ? target_at(rest, " \t$") : rest)) return line
    if (rest ~ /^</) return substr(line, 1, at + 2) "<" base substr(rest, 2)
    return substr(line, 1, at + 2) base rest
  }
  {
    was = fence
    toggled = fence == "" && comment_line($0)
    if (!toggled) toggled = fence_line($0)
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
    if (comment || fence != "") {
      printf "release-notes.sh: %s: the HTML comment or fenced code block opened on line %d" \
        " never closes\n", FILENAME, opened | "cat 1>&2"
      close("cat 1>&2")
      exit 12
    }
    if (!found) exit 10
    if (!nonblank) exit 11
    sub(/^\n+/, "", body)
    sub(/\n+$/, "\n", body)
    printf "%s", body
    if (definitions != "") printf "\n%s", definitions
  }
' "$changelog" || status=$?
case $status in
  0) ;;
  10)
    printf 'release-notes.sh: %s has no "## [%s] - YYYY-MM-DD" section\n' "$changelog" "$version" >&2
    exit 1
    ;;
  11)
    printf 'release-notes.sh: the [%s] section of %s is empty\n' "$version" "$changelog" >&2
    exit 1
    ;;
  12) exit 1 ;;
  *) exit "$status" ;;
esac
