# internal/AGENTS.md

The rules for the Go code of `doubleagent`: `cmd/doubleagent` and every
package under `internal/`. The rules every change keeps are in the
[root `AGENTS.md`](../AGENTS.md), whose [Code style](../AGENTS.md#code-style)
holds for Go files with the exemptions below. Two areas have their own
file: [`catalog/AGENTS.md`](catalog/AGENTS.md) for the catalog and
[`parsers/AGENTS.md`](parsers/AGENTS.md) for the parser kit in
`internal/parserkit`, the parsers under `internal/parsers/`, the registry
in `internal/parsers` itself, the tests of the registered parsers together in
`internal/growthtests` and the analysis output.
[`docs/internals.md`](../docs/internals.md) maps the packages.

## Go standards

### Gates and builds

- **Gates.** `gofmt -l cmd internal` prints nothing; `go vet ./...` for
  Linux, macOS and Windows, `staticcheck ./...` and `govulncheck ./...`
  (all with the build tags below) pass at the versions the
  [Quality gates](../AGENTS.md#quality-gates) table pins; `go test -race ./...`
  passes. `make check` runs them in the order of the `go` CI job, which runs
  them on the Ubuntu, macOS and Windows runners, then builds the six release
  targets twice and fails when any hash differs.
- **Build tags `noasm` and `nounsafe`.** Every `go` command of the project
  (build, vet, test, `staticcheck`, `govulncheck`) runs with both tags, so the
  zstd module's assembly and its unchecked memory reads are never built and
  compressed evidence is only decoded by bounds-checked Go. In a shell,
  `export GOFLAGS=-tags=noasm,nounsafe` once; CI sets the same `GOFLAGS` in
  the workflow `env`, the `Makefile` targets and `.github/go-build.sh` pass
  the tags themselves, and a command that passes its own `-tags` replaces
  `GOFLAGS`, so it spells all of them (`-tags dash,noasm,nounsafe`).
  `internal/zstdframe` does not compile without them: a build or vet stops
  with `undefined: buildWithTagsNoasmAndNounsafeOnly`. To see what a build
  links, run
  `go list -tags noasm,nounsafe -deps -f '{{.ImportPath}} {{.GoFiles}} {{.SFiles}}' ./cmd/doubleagent`
  for each `GOOS` and `GOARCH`: no `.s` file and no `unsafe_enabled.go` of
  `github.com/klauspost/compress` may appear.
- **No cgo.** Every build of the binary sets `CGO_ENABLED=0`; a change that
  needs `import "C"` is refused. The tests alone may run with cgo, because
  the race detector needs it on Linux; nothing they build ships.
- **Builds.** `.github/go-build.sh DIR [GOOS/GOARCH ...]` is the one build
  recipe: it builds the six release targets (macOS, Linux and Windows on
  `amd64` and `arm64`), or those named, with
  `CGO_ENABLED=0 go build -trimpath -tags noasm,nounsafe -ldflags=-buildid=`,
  and prints one `<sha256>  <file>` line per binary, the form
  `sha256sum -c` reads. The `Makefile` is the entry point: `make` builds the
  six into `dist/`, which git ignores, `make build` this machine's,
  `make check` the Go gates in CI order and `make reproducible` two builds
  compared. CI runs the script twice per commit and compares the hashes;
  the release workflow builds a pushed `v<version>` tag on an Ubuntu and a
  macOS runner and fails unless the bytes agree. The version is one string
  in `internal/version`, set from the tag (without its `v`) by
  `-ldflags -X`; a development build reports `0.0.0-dev` and the commit.

### Dependencies

- **Standard library first.** A third-party module is added only with a row
  in the table below naming the module, its version, why the standard
  library cannot serve, and whether it uses cgo or assembly or requires
  other modules. `vendor/` is committed, so a build needs no network, and
  `go.mod` pins the toolchain. A module added, removed or moved to another
  version updates `THIRD_PARTY_NOTICES.txt` in the same commit, with its
  licence files verbatim; the tests in `internal/version` fail until it
  does. The development tools (`staticcheck`, `govulncheck`) are not module
  requirements: the workflow's `env` block, the `Makefile` and the gates
  table pin their versions and run them with `go run <module>@<version>`, so
  `go.mod` and `vendor/` hold only what the binary links.

  | Module | Version | Why |
  | --- | --- | --- |
  | `golang.org/x/sys` | v0.50.0 | System calls the standard library does not expose: birth times (`Statx` on Linux, `Lstat` on macOS), `Uname`, and on Windows handles opened without following a reparse point, reparse tags, file owners, final paths, directory symlinks, file times set on a handle and the registry; pure Go. |
  | `github.com/BurntSushi/toml` | v1.6.0 | TOML 1.1 documents with their tables' keys in document order, which `internal/tomldoc` builds on; it has no setting that limits it to TOML 1.0; pure Go with no dependencies. |
  | `github.com/klauspost/compress` | v1.20.4 | zstd decoding (only its `zstd` package and the packages beneath it), which the standard library offers only as `internal/zstd`; no cgo and no requirements, but assembly on `amd64` and `arm64` and `unsafe` memory reads that the `noasm` and `nounsafe` tags leave out, so neither is linked. `internal/zstdframe` runs it in the calling goroutine with a 128 MiB window limit. |

### Files, programs and paths

- **File system calls.** Every file under `internal/` that is built for
  Windows opens, lists, stats, creates and removes paths through
  `internal/osfile`, never through `os` directly: on Windows the wrappers
  pass the extended-length form (`\\?\`), so a name ending in a dot or a
  space is the entry it names and a long path needs no special case, and
  they return errors with the path in native form. A file built only for
  Unix (`_unix.go`, `_linux.go`, `_darwin.go`, or a `!windows` build
  constraint) may call `os` directly, since there the wrappers are the `os`
  calls unchanged. `TestWindowsBuildCallsNoPathFunctionOfOsDirectly` in
  `internal/osfile` fails on a direct call in any file of the Windows build.
  A path in the staging directory goes through `stage.Tree`, never by its
  full name, and on Windows no `os.Root` method is given a collected name
  (the root cleans each name with `GetFullPathName`) or used where its
  answer must match a creation through `osfile` (its creations ask for
  backup intent).
- **Platform code** lives in `_linux.go`, `_darwin.go`, `_windows.go` and
  `_unix.go` files or behind a build constraint, never in a `runtime.GOOS`
  switch; a function written once per platform keeps one name and one
  comment shape, and a change to one side is checked against the others in
  the same commit ([Function pairs](../docs/internals.md#function-pairs)).
  Behaviour is the same on every platform unless a fact of the platform
  requires a difference, and the function's comment names that fact; that
  another program behaved so is no reason. A manifest row's size, times and
  attributes are read from the file itself (its lstat or an open handle),
  never from a directory listing's copy of them, which can lag the file;
  only a file no handle can open is described by its listing entry.
- **Programs run.** `os/exec` is imported by `internal/hostcmd` and test
  files only, and runs only `getent`, `dscl` and `plutil`: the programs that
  give a platform's user list, or a launchd value, that no file gives.
  Copying, hashing, archiving, searching and listing are Go.
- **Symlinks.** Never follow one: `os.Lstat`, never `os.Stat`, on anything
  under a collection base; open with `O_NOFOLLOW` where the platform has it;
  recreate a link with `os.Symlink`. A symlinked directory in the middle of
  a catalog glob is followed only by the glob expander, as a shell does, and
  handled as
  [Paths through a symlinked directory in `docs/coverage.md`](../docs/coverage.md#paths-through-a-symlinked-directory)
  describes.
- **Paths are bytes.** A path is a `string` holding raw bytes: no `[]rune`,
  `strings.ToLower`, `utf8` decoding or `%q` on one; case folding is
  ASCII-only. Windows path keys (`syspath.Fold`) fold ASCII only: NTFS's own
  case table keeps apart pairs any other fold would merge (`ı` and `I`), and
  merging two names loses one of them. The fold is only a lookup key: a set,
  map or claim over paths is a `syspath.Entries`, where two paths differing
  only in ASCII case are one entry only when the file system gives both one
  identity or says neither exists (`follow.SameEntry`), since a directory
  marked case-sensitive holds `Alice` and `alice` apart; a test of whether a
  path lies under a home or base trusts bytes first (`syspath.CutUnder`).
- **Paths from outside a listing.** A Windows path that comes from outside a
  directory listing is respelled as the disk holds it through
  `fsmeta.OnDiskPath` before it is compared, so two spellings of one
  directory become the same bytes, stopping at the first link: `-r` and
  `-o`, the analyst's own host paths, and the live registry's paths from
  their drive; `-p` and values read from the image only below the root in
  image mode. A link target is compared as text, since respelling it could
  look outside the root. Two `PhysicalDir` results are compared byte for
  byte through `stage.Under`.
- **Listings.** Directory listings use `(*os.File).ReadDir(-1)` (directory
  order), and pattern expansion the glob expander (sorted bytewise on every
  platform, each match spelled as the listing gives it); never
  `filepath.WalkDir` or `os.ReadDir` on a collected tree.

### Errors, state and concurrency

- **Errors and exits.** Library code (`internal/`) returns errors wrapped
  with `%w` and the path or item (`fmt.Errorf("stage %s: %w", path, err)`)
  and never calls `os.Exit`, `log.Fatal` or `panic`; a panic in library code
  is a bug. Only `cmd/doubleagent` and a command's run function map errors
  to exit codes: `collect` and the top-level command line use 0, 1 (usage),
  2 (fatal) and 130 (interrupted); the analysis commands (`detect`,
  `timeline`, `fleet`, `catalog`) use 0 (found), 1 (nothing found), 2 (an
  error or a command-line mistake) and 130 (interrupted). A manifest
  `error` field holds the innermost `*fs.PathError` text.
- **State.** No package-level mutable state; a run's state lives in a struct
  passed down. The one exception is the version variable that `-ldflags -X`
  sets. `init` only validates an embedded file. Timestamps come from one
  injectable clock so tests can pin them.
- **Goroutines.** Every goroutine is bounded by a `context.Context` and
  waited for; the caller decides output order, never scheduling. Every loop
  over directories or files checks `ctx.Err()` per directory.
- **Signals.** A `collect` run ends before its archive is written only when
  it is told to stop (an interrupt or a termination request) or meets a
  fatal error; in both cases it removes its staging directory unless `-k`
  is given. Losing its terminal or the reader of its output is neither: the
  run continues, because the log file is its record. So `SIGINT` and
  `SIGTERM` cancel the context, and the run logs `interrupted` and exits
  130; `SIGHUP` and `SIGPIPE` are caught and dropped; every other signal
  keeps Go's default, decided one by one in
  [Signals](../docs/internals.md#signals), which a new handler updates. A
  signal the run reacts to is caught, never ignored, so the programs a run
  starts keep their defaults, except one that was ignored when the program
  started (`signal.Ignored`), which stays ignored. The handlers cover a
  collection and an inventory run and are removed when `collect.Run`
  returns; `--list`, `--version` and `--help` stage nothing and set none.
  The analysis commands set theirs through `collect.CatchSignals` too, and
  on Linux and macOS also catch `SIGPIPE`, so a reader that goes away ends
  them with their own exit code and cleanup.
- **Logging and output.** Log lines go through the run log
  (`internal/runlog`); `fmt.Print*` outside the summary, inventory and usage
  writers is a defect; no file contents, secret or matched line reaches the
  log, and a log line or row exists only for something that changes the
  evidence or what the responder does. A failed write to standard output or
  standard error never ends a
  collection or inventory run: the run drops the failure, since the log
  file is the record, except under `--inventory`, whose standard output is
  its output and whose failed write is fatal. On Linux and macOS the run
  catches the `SIGPIPE` and `SIGHUP` such a loss sends
  (the Signals bullet above); on Windows the write fails
  without a signal.
- **No fixed values, no repeated checks.** An output field or log value
  that can only ever hold one text is not written. A test made before an
  operation that the operation itself repeats (a writability test before a
  create, an existence test before an open) is not made: the operation's
  own error is the report.

### Standard library and readers

- **Standard library first.** Where the standard library does the job, use
  it: `encoding/json` reads and writes JSON (a struct's field order is the
  documented field order, with `SetEscapeHTML(false)`), `encoding/csv`
  writes CSV, `flag` reads each command's command line (one `FlagSet` per
  command, options before the input), and `strings`, `slices`, `path` and
  the like are used in place of a hand-written copy. A file name that is not
  valid UTF-8 is therefore written to the manifest with U+FFFD in place of
  each bad byte; the archive member keeps the real bytes.
- **A package's comment is in its `doc.go`.** The file holds the comment
  (`// Package name ...`, or `// Command name ...` for a `main` package) and
  the package clause, nothing else, at most 80 lines
  ([Short comments](../AGENTS.md#code-style)).
- **A package that stands in for the standard library says why.** Where the
  project has its own package or function for something the standard
  library (or a well-known module) also offers, such as `internal/glob`
  beside `path.Match` and `filepath.Glob`, or `internal/osfile` beside the
  `os` path functions, the package comment names the standard facility and
  says in a few sentences what it does differently that the project needs
  ([Stand-ins for the standard library](../docs/internals.md#stand-ins-for-the-standard-library)).
  A new package of this kind is added only with that explanation, and when
  the reason stops holding the package goes.
- **Readers follow the specification.** A reader for a specified format (a
  repository config file, an INI dialect, TOML, SQLite, protobuf, zstd) is
  written from the specification, with one test per rule: quoting, escapes,
  continuation, comments, header forms, encoding and case. Lines end at LF
  or CRLF only and trims are ASCII only (`internal/textlines`), and an
  untrusted evidence file is read whole, uncapped, in linear time.
- **One way to build a string.** Use `fmt.Sprintf` (or `Errorf`, `Fprintf`)
  when a value needs converting, quoting or padding; when every value is
  already a string, join with `+`, `strings.Join` or a `strings.Builder`,
  and make the error with `errors.New`, never from another error's text,
  which drops it from the chain: wrap it with `%w`, or with `reason.Wrap`
  where the message words it its own way. `tools/check_code_style.py` fails
  on the forms that break this; its comments say which arguments it takes
  as errors.

### Names, comments and size

- **Names and comments.** [Code style](../AGENTS.md#code-style) holds, with
  these Go exemptions: a one- or two-letter receiver; `ctx` for a
  `context.Context`; `ok`; `err` alone or as a word of a longer name
  (`closeErr`, and the `errNotFound` form staticcheck requires of a
  package's error variables); `w` and `r` for an `io.Writer` and an
  `io.Reader` parameter; `i`, `j`, `k` and `n` for an index or count in a
  few lines; `t` and `b` in tests. Every exported identifier has a doc
  comment starting with its name, and every unexported function one
  comment. `tools/check_code_style.py` fails on a function or method, a
  test helper included, with no comment on the line directly above it (a
  `//go:` directive line is passed over); `main`, `init` and the functions
  `go test` runs are exempt. Every special case, cache, option,
  abstraction or log line states in its comment the forensic result, safety
  property, platform fact or measured cost it protects; one kept for its
  cost has its measurement in [`docs/internals.md`](../docs/internals.md).
- **Size.** A function body stays under 40 lines, never over 60. Over 40 is
  only for a per-item loop that a split would give an extra call per file
  or member, a flat list of discovery sources, or one literal. The checker
  fails on a Go function or method, or a function literal a package
  variable is assigned, whose body (the lines between its braces) is over
  60 lines, except the reviewed lists in `internal/follow` that
  `GO_LONG_FUNCTIONS` names, each audited as a whole and read best
  unbroken.
- **The code stands on its own terms.** No package, identifier, comment or
  test under `cmd/` or `internal/` refers to Python or to the earlier
  collector scripts, or explains Go by what another program does; a comment
  states the rule itself, and may cite a numbered exception or difference
  of
  [`docs/differences-from-the-scripts.md`](../docs/differences-from-the-scripts.md).
  No test runs a Python interpreter or takes its expected values from one.
  The scripts under a package's `testdata` that build binary fixtures Go
  cannot build without using the code under test (SQLite databases, zstd
  frames, archives) are the exception, and no test or CI step runs them.
- **Catalog is data.** Code never holds a path or glob that belongs in a
  catalog table ([`catalog/AGENTS.md`](catalog/AGENTS.md)).

### Tests

- **Tests follow the same standards.** Every rule in this file and in
  [Code style](../AGENTS.md#code-style) applies to `_test.go` files as it
  does to package code, in every package.
- **Form.** Table-driven, with `t.Run` names that read as sentences;
  fixtures built under `t.TempDir()`; run with `-race`. A test whose
  subject starts no goroutine runs its independent cases with
  `t.Parallel()` and builds a fixture they share once, and each package's
  tests finish well inside `go test`'s ten-minute default under the race
  detector on the slowest CI runner.
- **Proof.** A test added with a fix is run against the code before the fix
  and fails there, and a test of cost is also run with the cost put back and
  fails; the change's report names both runs. A test named for an
  exclusive claim ("only", "never", "once") proves the negative half too.
  A test needing POSIX permissions or an unprivileged user, or creating a
  symlink, skips with a reason when the platform or the account cannot
  give it.
- **No live runs.** No test reads the developer's home or the network, and
  no test runs `collect` without `-r`: a run without a root collects the
  developer's own credentials. An empty root (`-r ""`) is live mode too,
  exactly as no `-r` at all, and so, on Linux and macOS, is a root that
  comes out as `/` once a relative root is joined to the current directory
  and its `..` components are taken away by name (`-r /`, `-r /tmp/..`,
  `-r ..` run from `/tmp`). A symlink to `/` is not resolved, so it gives
  an image run whose root is the whole workstation, which reads the same
  files, as does `-r C:\` on Windows. A test or a hand run passes `-r` a
  real image directory built for it, never an empty or computed value that
  can come out empty.
- **Expected output.** `internal/collect/imagetest` builds the fixture image
  the end-to-end tests of `collect` run over, and the expected-output tests
  of `collect`, the walk and the analysis commands compare whole outputs
  with files under the packages' `testdata`; `DOUBLEAGENT_UPDATE_GOLDEN=1`
  rewrites them, and the diff is read before it is committed
  ([Testing](../docs/testing.md)). The collection's expected files differ
  by platform, so a change to them is made on Linux, macOS and Windows.
