#!/bin/sh
# survey.sh: one listing of a Linux or macOS lab VM for a real-install round:
# the kind, size, mode, UTC modification time and path of everything under
# the agent's survey roots, never a file's contents. round.sh pipes the
# agent's roots file to it:
#
#   ssh guest sh lab/survey.sh < agents/<agent>/roots > listings/NN-<step>.txt
#
# A roots line is a path (`~` for the home, `<tmp>` for the per-user temp
# directory, globs allowed), optionally after `linux:`, `macos:`, `unix:` or
# `windows:`; `keychain:<service>` and `process:<name>` lines add a
# credential-store presence line and a process count, and a `skip:<path>`
# line keeps a directory's entry but none below it. Output lines are
# tab-separated: <kind> <size> <mode> <mtime> <path>, sorted by path, then
# `top` lines (the entries of the home's and temp directory's top levels, so
# a directory no root covers still shows in a diff), then the state lines.
# Symlinks are listed with their target and never followed.

set -u
TZ=UTC
LC_ALL=C
export TZ LC_ALL
tab=$(printf '\t')
home_dir=${HOME:?}
case $(uname -s) in
  Darwin) platform=macos ;;
  *) platform=linux ;;
esac
temp_dir=${TMPDIR:-}
if [ -z "$temp_dir" ] && [ "$platform" = macos ]; then
  temp_dir=$(getconf DARWIN_USER_TEMP_DIR 2>/dev/null) || temp_dir=
fi
temp_dir=${temp_dir%/}
[ -n "$temp_dir" ] || temp_dir=/tmp

# GNU find prints every field itself, a link's target unquoted; elsewhere
# BSD stat appends " -> <target>" with %SY, one stat call per find batch.
# stat's %N is not used on Linux: GNU and uutils quote the name in it.
if find / -maxdepth 0 -printf '' >/dev/null 2>&1; then
  link_format="%y$tab%s$tab%m$tab%TY-%Tm-%TdT%TH:%TM:%TSZ$tab%p -> %l\n"
  other_format="%y$tab%s$tab%m$tab%TY-%Tm-%TdT%TH:%TM:%TSZ$tab%p\n"
  stat_entries() {
    find "$@" -maxdepth 0 \( -type l -printf "$link_format" \) -o -printf "$other_format" \
      2>/dev/null
  }
  survey_root() {
    [ -e "$1" ] || [ -L "$1" ] || return 0
    find "$1" \( -type l -printf "$link_format" \) -o -printf "$other_format" 2>/dev/null
  }
else
  stat_format="%HT$tab%z$tab%Lp$tab%Sm$tab%N%SY"
  stat_entries() { stat -t '%Y-%m-%dT%H:%M:%SZ' -f "$stat_format" -- "$@" 2>/dev/null; }
  survey_root() {
    [ -e "$1" ] || [ -L "$1" ] || return 0
    find "$1" -exec stat -t '%Y-%m-%dT%H:%M:%SZ' -f "$stat_format" -- {} + 2>/dev/null
  }
fi

# Print a roots line with its platform prefix removed, or nothing when the
# line is a comment, blank or for another platform.
root_for_platform() {
  case $1 in
    '' | '#'*) return 0 ;;
    linux:*) [ "$platform" = linux ] && printf '%s\n' "${1#linux:}" ;;
    macos:*) [ "$platform" = macos ] && printf '%s\n' "${1#macos:}" ;;
    unix:*) printf '%s\n' "${1#unix:}" ;;
    windows:*) return 0 ;;
    *) printf '%s\n' "$1" ;;
  esac
  return 0
}

# Print a roots path with `~` and `<tmp>` expanded. The tilde is held in a
# variable so the patterns match it literally rather than expand it.
tilde=$(printf '\176')
expand_path() {
  case $1 in
    "$tilde") printf '%s\n' "$home_dir" ;;
    "$tilde"/*) printf '%s\n' "$home_dir/${1#"$tilde"/}" ;;
    '<tmp>') printf '%s\n' "$temp_dir" ;;
    '<tmp>/'*) printf '%s\n' "$temp_dir/${1#'<tmp>/'}" ;;
    *) printf '%s\n' "$1" ;;
  esac
}

# Survey one roots path after expanding `~`, `<tmp>` and any glob in it.
# The glob is expanded with IFS set to newline only, so a match holding a
# space stays one word.
expand_root() {
  _er_path=$(expand_path "$1")
  case $_er_path in
    *'*'* | *'?'* | *'['*)
      _er_saved_ifs=$IFS
      IFS='
'
      # shellcheck disable=SC2086 # the glob is meant to expand
      set -- $_er_path
      IFS=$_er_saved_ifs
      for _er_match in "$@"; do survey_root "$_er_match"; done
      ;;
    *) survey_root "$_er_path" ;;
  esac
}

# Map find's kind letters and BSD stat's kind names ("Regular File") to
# file, dir and link, so the listings of both platforms read alike.
normalize() {
  awk -v FS="$tab" -v OFS="$tab" '{
    kind = tolower($1)
    if (kind == "f" || kind ~ /regular/) kind = "file"
    else if (kind == "d" || kind ~ /^directory/) kind = "dir"
    else if (kind == "l" || kind ~ /symbolic link/) kind = "link"
    else if (kind == "s") kind = "socket"
    else if (kind == "p") kind = "fifo"
    else gsub(/ /, "-", kind)
    $1 = kind
    print
  }'
}

# Count the processes whose program, or whose script under an interpreter
# (`node /usr/bin/gemini`), is named $1.
count_processes() {
  ps -A -o args= 2>/dev/null | awk -v want="$1" '{
    program = $1; sub(/.*\//, "", program)
    script = $2; sub(/.*\//, "", script)
    if (program == want || script == want) count++
  } END { print count + 0 }'
}

# Drop the entries below the `skip:` paths of the roots, one per line in
# $1; the skipped directory's own entry stays.
drop_skipped() {
  awk -v FS="$tab" -v skips="$1" 'BEGIN { count = split(skips, skip, "\n") }
  {
    for (index_number = 1; index_number <= count; index_number++)
      if (skip[index_number] != "" && index($5, skip[index_number] "/") == 1) next
    print
  }'
}

roots=$(cat)
skip_paths=$(printf '%s\n' "$roots" | while IFS= read -r line; do
  entry=$(root_for_platform "$line")
  case $entry in
    (skip:*) expand_path "${entry#skip:}" ;;
  esac
done)

{
  printf '%s\n' "$roots" | while IFS= read -r line; do
    path=$(root_for_platform "$line")
    case $path in
      '' | keychain:* | process:* | skip:*) ;;
      *) expand_root "$path" ;;
    esac
  done
  survey_root "$home_dir/lab/proj"
} | normalize | drop_skipped "$skip_paths" | sort -u -t "$tab" -k5

for directory in "$home_dir" "$home_dir/.config" "$home_dir/.local/share" \
  "$home_dir/.local/state" "$home_dir/.local/bin" "$home_dir/.cache" \
  "$home_dir/Library/Application Support" "$home_dir/Library/Caches" "$temp_dir" /tmp; do
  [ -d "$directory" ] || continue
  stat_entries "$directory"/* "$directory"/.[!.]* "$directory"/..?*
done | normalize | sort -u -t "$tab" -k5 | sed "s/^/top$tab/"

printf '%s\n' "$roots" | while IFS= read -r line; do
  entry=$(root_for_platform "$line")
  case $entry in
    keychain:*)
      service=${entry#keychain:}
      if [ "$platform" != macos ]; then
        state='not-checked'
      elif security find-generic-password -s "$service" >/dev/null 2>&1; then
        state=present
      else
        state=absent
      fi
      printf 'keychain\t%s\t-\t-\t%s\n' "$state" "$service"
      ;;
    process:*)
      printf 'process\t%s\t-\t-\t%s\n' "$(count_processes "${entry#process:}")" "${entry#process:}"
      ;;
  esac
done
