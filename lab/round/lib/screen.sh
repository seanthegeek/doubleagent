# shellcheck shell=sh
# screen.sh: readers of a rendered agent screen, sourced by round.sh on the
# analyst machine and by the round pipeline's tests. Each reads the screen
# on stdin, as `drive screen` prints it with the CRs removed.
#
# The box-drawing characters are matched as byte strings under LC_ALL=C, so
# the readers behave the same whatever the host's locale.

# Print the screen with each box-drawing character an agent draws around a
# dialog replaced by one space, so every line keeps its width in columns.
unbox() {
  LC_ALL=C sed -e 's/│/ /g' -e 's/┃/ /g' -e 's/║/ /g' -e 's/╭/ /g' -e 's/╮/ /g' \
    -e 's/╰/ /g' -e 's/╯/ /g' -e 's/┌/ /g' -e 's/┐/ /g' -e 's/└/ /g' -e 's/┘/ /g' \
    -e 's/─/ /g' -e 's/━/ /g' -e 's/═/ /g'
}

# Print the screen unboxed, with the spaces at both ends of a line removed.
strip_box() {
  unbox | sed -e 's/^ *//' -e 's/ *$//'
}

# The awk program of joined_urls, over the raw screen in the C locale, so a
# box-drawing character is a three-byte string and `columns` counts each
# UTF-8 character once. A URL is open (wrapped) when what follows it on its
# line is nothing on a line the full `width` wide, or at most two spaces
# and then a vertical box border, the edge of a dialog box anywhere on the
# screen. The next line's leading run, after any border and spaces, is then
# appended whole, and the URL stays open only when that run is open the
# same way; a URL ending anywhere else is complete.
# shellcheck disable=SC2016 # the awk program's own variables, not the shell's
JOINED_URLS_AWK='
  function columns(text, copy) {
    copy = text
    return length(text) - gsub(/[\200-\277]/, "", copy)
  }
  function open_after(line, end_byte, rest) {
    rest = substr(line, end_byte + 1)
    return (rest == "" && columns(line) == width) || rest ~ /^(  ?)?(│|┃|║)/
  }
  function cut(text) {
    return match(text, /│|┃|║/) ? substr(text, 1, RSTART - 1) : text
  }
  function scan(line, from, rest, start, found) {
    while (match(rest = substr(line, from), /https?:\/\/[^ ]+/)) {
      start = from + RSTART - 1
      found = cut(substr(line, start, RLENGTH))
      from = start + length(found)
      if (open_after(line, from - 1)) { url = found; open = 1; return }
      print found
    }
  }
  function continued(line, start, run) {
    match(line, /^ *((│|┃|║) *)?/)
    start = RLENGTH + 1
    if (!match(substr(line, start), /^[^ ]+/)) return 0
    run = cut(substr(line, start, RLENGTH))
    if (run == "" || index(run, "://") || run ~ /^(─|━|═|╭|╮|╰|╯|┌|┐|└|┘)/) return 0
    url = url run
    return start + length(run)
  }
  {
    from = 1
    if (open) {
      open = 0
      from = continued($0)
      if (from && open_after($0, from - 1)) { open = 1; next }
      if (!from) from = 1
      print url
    }
    scan($0, from)
  }
  END { if (open) print url }'

# Print every http:// or https:// URL on a screen $1 columns wide as one
# line, a URL the terminal or a dialog box wrapped joined over its lines as
# JOINED_URLS_AWK says. A word of prose after a URL that happens to fill its
# line is joined too, so a joined URL is checked against the screen.
joined_urls() {
  LC_ALL=C awk -v width="$1" "$JOINED_URLS_AWK"
}

# Print the first line of the screen that matches the grep -E pattern $1,
# the agent's highlighted-option pattern, after strip_box; nothing when no
# line does.
highlighted_line() {
  strip_box | grep -E -- "$1" | sed -n 1p
}

# Print the live part of the screen, its lines as drawn: those after the
# last line that matches, after strip_box, the grep -E pattern $1 of the
# lines that end the scrollback (an input line, a restart notice), so a
# dialog above the last such line is scrollback, such as a box a restart
# left behind. The whole screen when $1 is empty or matches no line;
# nothing when the last line matches.
live_screen() {
  _ls_screen=$(cat)
  _ls_last=
  [ -z "$1" ] ||
    _ls_last=$(printf '%s\n' "$_ls_screen" | strip_box | grep -n -E -- "$1" |
      tail -n 1 | cut -d: -f1)
  if [ -z "$_ls_last" ]; then
    printf '%s\n' "$_ls_screen"
  else
    printf '%s\n' "$_ls_screen" | tail -n +"$((_ls_last + 1))"
  fi
}
