# shellcheck shell=sh
# screen.sh: readers of a rendered agent screen, sourced by round.sh on the
# analyst machine and by the round pipeline's tests. Each reads the screen
# on stdin, as `drive screen` prints it with the CRs removed.
#
# The box-drawing characters are matched as byte strings under LC_ALL=C, so
# the readers behave the same whatever the host's locale.

# Print the screen with the box-drawing characters an agent draws around a
# dialog replaced by spaces and the spaces at both ends of a line removed.
strip_box() {
  LC_ALL=C sed -e 's/│/ /g' -e 's/┃/ /g' -e 's/║/ /g' -e 's/╭/ /g' -e 's/╮/ /g' \
    -e 's/╰/ /g' -e 's/╯/ /g' -e 's/┌/ /g' -e 's/┐/ /g' -e 's/└/ /g' -e 's/┘/ /g' \
    -e 's/─/ /g' -e 's/━/ /g' -e 's/═/ /g' -e 's/^ *//' -e 's/ *$//'
}

# Print every http:// or https:// URL on the screen as one line. A URL the
# terminal wrapped goes on over the lines after it until one that is blank,
# starts with a space-free word followed by a space, is a prompt line
# (starting with >, $, # or %) or starts another URL; a URL followed by a
# space on its own line is complete.
joined_urls() {
  strip_box | LC_ALL=C awk '
    function finish() { if (url != "") print url; url = ""; open = 0 }
    {
      line = $0
      if (open && line != "" && line !~ /^[^ ]+ / && line !~ /^[>$#%]/ &&
        line !~ /https?:\/\//) {
        url = url line
        next
      }
      finish()
      if (match(line, /https?:\/\/[^ ]+/)) {
        url = substr(line, RSTART, RLENGTH)
        open = (RSTART + RLENGTH - 1 == length(line))
      }
    }
    END { finish() }'
}

# Print the first line of the screen that matches the grep -E pattern $1,
# the agent's highlighted-option pattern, after strip_box; nothing when no
# line does.
highlighted_line() {
  strip_box | grep -E -- "$1" | sed -n 1p
}
