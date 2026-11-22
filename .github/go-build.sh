#!/bin/sh
# Build the doubleagent binary for every release target into the directory
# given and print one "<sha256>  <file>" line per binary built, the form
# sha256sum -c reads, so two runs can be compared for a reproducible build and
# the output saved as the release's SHA256SUMS. Targets given after the
# directory (as GOOS/GOARCH) are built in place of the six release targets.
# VERSION in the environment is stamped into the binary; a build without it
# reports a development version. -tags noasm,nounsafe keeps the zstd module's
# assembly and unchecked memory reads out of every binary; internal/zstdframe
# does not compile without them.
set -eu
dir=${1:?usage: go-build.sh DIR [GOOS/GOARCH ...]}
shift
[ "$#" -gt 0 ] || set -- darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64 \
  windows/arm64
mkdir -p "$dir"
version=${VERSION:-0.0.0-dev}

# Print the "<sha256>  <name>" line of the file named $1 in the current
# directory. Git Bash's sha256sum marks a file read in binary mode with a "*"
# before its name; the mark is dropped so every runner prints the same line.
print_hash() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | sed 's/ \*/  /'
  else
    shasum -a 256 "$1"
  fi
}

for target in "$@"; do
  target_os=${target%/*}
  arch=${target#*/}
  name="doubleagent-$target_os-$arch"
  [ "$target_os" != windows ] || name="$name.exe"
  CGO_ENABLED=0 GOOS=$target_os GOARCH=$arch go build -trimpath -tags noasm,nounsafe \
    -ldflags "-buildid= -X github.com/seanthegeek/doubleagent/internal/version.Version=$version" \
    -o "$dir/$name" ./cmd/doubleagent
  (cd "$dir" && print_hash "$name")
done
