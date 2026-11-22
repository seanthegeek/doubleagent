# The contributor's entry point for building and checking doubleagent. Each
# target is a thin wrapper: the release build itself is .github/go-build.sh,
# which the CI and release workflows run too. Written in the subset of make
# that GNU make and the BSD make of macOS and the BSDs both read.
#
# Every go command is given the build tags in GOFLAGS by its recipe, so no
# target needs GOFLAGS exported (see "Build tags" in docs/development.md); the
# tests that build the binary themselves read the tags from there. VERSION,
# given in the environment or on the command line (make VERSION=1.2.3), is
# the version the binaries report; without it they report the development
# version.

GO = go
TAGS = noasm,nounsafe
TAGGED = GOFLAGS=-tags=$(TAGS)
# The linter versions of the "go" CI job: the env block of
# .github/workflows/ci.yml and the "Quality gates" table of AGENTS.md give the
# same ones, and the three change together; tools/tests/test_version_pins.py
# fails when they differ.
STATICCHECK_VERSION = 0.8.2
GOVULNCHECK_VERSION = 1.9.0

.PHONY: all build test race vet fmt-check staticcheck govulncheck test-dash \
	test-nojsonv2 check reproducible clean

# The six release binaries (macOS, Linux and Windows on amd64 and arm64) in
# dist/, with the SHA-256 of each printed.
all:
	VERSION="$(VERSION)" sh .github/go-build.sh dist

# The binary for this machine only, as dist/doubleagent-<os>-<arch>.
build:
	VERSION="$(VERSION)" sh .github/go-build.sh dist "$$($(GO) env GOOS)/$$($(GO) env GOARCH)"

# The tests, without the race detector: the quick run while working.
test:
	$(TAGGED) $(GO) test ./...

# The tests under the race detector, as CI runs them; it needs cgo on Linux.
race:
	CGO_ENABLED=1 $(TAGGED) $(GO) test -race -timeout 30m ./...

# go vet for Linux, macOS and Windows, since each has files the others do not
# build.
vet:
	GOOS=linux $(TAGGED) $(GO) vet ./...
	GOOS=darwin $(TAGGED) $(GO) vet ./...
	GOOS=windows $(TAGGED) $(GO) vet ./...

# Fails, naming the files, when gofmt would change any Go file.
fmt-check:
	@unformatted=$$(gofmt -l cmd internal); \
	if [ -n "$$unformatted" ]; then printf 'gofmt: %s\n' "$$unformatted"; exit 1; fi

# staticcheck at the pinned version; go run fetches it on first use.
staticcheck:
	$(TAGGED) $(GO) run honnef.co/go/tools/cmd/staticcheck@v$(STATICCHECK_VERSION) ./...

# Known vulnerabilities the code can reach, at the pinned govulncheck version;
# it needs the network to read the vulnerability database.
govulncheck:
	$(TAGGED) $(GO) run golang.org/x/vuln/cmd/govulncheck@v$(GOVULNCHECK_VERSION) ./...

# The glob package's tests that compare it with dash, which must be installed.
test-dash:
	CGO_ENABLED=1 $(TAGGED) $(GO) test -race -tags dash,$(TAGS) ./internal/glob/

# The JSON depth limit test, in the build without encoding/json v2 where the
# limit applies.
test-nojsonv2:
	GOEXPERIMENT=nojsonv2 $(TAGGED) $(GO) test \
		-run '^TestArraysAndObjectsNestAtMostTenThousandDeep$$' ./internal/jsonvalue/

# The Go gates of the "go" CI job, in its order: formatting, vet, staticcheck,
# govulncheck, the tests under the race detector, the dash and JSON depth
# tests, and the reproducible build.
check: fmt-check vet staticcheck govulncheck race test-dash test-nojsonv2 reproducible

# Builds the six binaries twice in temporary directories and fails unless the
# two lists of hashes are the same.
reproducible:
	@first=$$(mktemp -d) && second=$$(mktemp -d) && \
	VERSION="$(VERSION)" sh .github/go-build.sh "$$first/bin" > "$$first/sums" && \
	VERSION="$(VERSION)" sh .github/go-build.sh "$$second/bin" > "$$second/sums" && \
	cat "$$first/sums" && \
	if cmp "$$first/sums" "$$second/sums"; then status=0; else status=1; fi; \
	rm -rf "$$first" "$$second"; \
	[ "$$status" = 0 ] && echo 'reproducible: the two builds are identical'

# Removes dist/.
clean:
	rm -rf dist
