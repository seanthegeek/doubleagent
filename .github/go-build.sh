#!/bin/sh
# Build the doubleagent binary for every release target into the directory
# given and print one "<sha256>  <file>" line per binary, so two runs of this
# script can be compared for a reproducible build. VERSION in the environment
# is stamped into the binary; a build without it reports a development version.
set -eu
dir=${1:?usage: go-build.sh DIR}
mkdir -p "$dir"
version=${VERSION:-0.0.0-dev}
for target in darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64 windows/arm64; do
  target_os=${target%/*}
  arch=${target#*/}
  out="$dir/doubleagent-$target_os-$arch"
  [ "$target_os" != windows ] || out="$out.exe"
  CGO_ENABLED=0 GOOS=$target_os GOARCH=$arch go build -trimpath \
    -ldflags "-buildid= -X github.com/seanthegeek/doubleagent/internal/version.Version=$version" \
    -o "$out" ./cmd/doubleagent
done
cd "$dir"
if command -v sha256sum >/dev/null 2>&1; then sha256sum doubleagent-*; else shasum -a 256 doubleagent-*; fi
