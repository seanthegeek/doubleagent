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

# Print every http:// or https:// URL on a screen $1 columns wide as one
# line. A URL is wrapped, and so open, only when its line is the full width
# and the URL runs to the right edge or to a box border's padding (at most
# two columns before it); the next line's leading space-free run is then
# appended whole, and the URL stays open only when that run reaches the
# edge the same way. A URL ending anywhere else is complete, whatever the
# line after it holds; a prose word after a full-width URL line is joined
# too, so a joined URL is checked against the screen.
joined_urls() {
  unbox | LC_ALL=C awk -v width="$1" '
    function columns(text, copy) {
      copy = text
      return length(text) - gsub(/[\200-\277]/, "", copy)
    }
    function at_edge(line, end_byte) {
      return columns(line) == width && substr(line, end_byte + 1) ~ /^ *$/ &&
        columns(line) - columns(substr(line, 1, end_byte)) <= 2
    }
    function scan(line, from, rest, start) {
      while (match(rest = substr(line, from), /https?:\/\/[^ ]+/)) {
        start = from + RSTART - 1
        from = start + RLENGTH
        if (at_edge(line, from - 1)) { url = substr(line, start, RLENGTH); open = 1; return }
        print substr(line, start, RLENGTH)
      }
    }
    {
      from = 1
      if (open) {
        open = 0
        if (match($0, /[^ ]+/) && index(substr($0, RSTART, RLENGTH), "://") == 0) {
          url = url substr($0, RSTART, RLENGTH)
          from = RSTART + RLENGTH
          if (at_edge($0, from - 1)) { open = 1; next }
        }
        print url
      }
      scan($0, from)
    }
    END { if (open) print url }'
}

# Print the first line of the screen that matches the grep -E pattern $1,
# the agent's highlighted-option pattern, after strip_box; nothing when no
# line does.
highlighted_line() {
  strip_box | grep -E -- "$1" | sed -n 1p
}
