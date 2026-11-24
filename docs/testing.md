# Testing

DoubleAgent is tested with Go's own test tool. Most packages have unit
tests beside their code; on top of those, `doubleagent collect` is run end
to end over a fake disk image and its whole output compared with expected
files on Linux, macOS and Windows, the analysis commands are run over
synthetic collections and compared the same way, and every parser is held
to output and work that grow in step with its input. Every `go` command
needs the build tags [Development](development.md#build-tags) gives:

```sh
export GOFLAGS=-tags=noasm,nounsafe
go test ./...                          # every package
CGO_ENABLED=1 go test -race ./...      # under the race detector, as CI runs it
go test ./internal/parsers/codex       # one package
```

No test reads the developer's home directory or the network, and no test
runs `doubleagent collect` without `-r` on an image it built: a
collection run without a root, or with an empty one, collects the
machine it runs on, credentials included.

## Unit tests

Each package under `internal/` has its tests in `_test.go` files beside
its code, table-driven, with subtest names that read as sentences, and
fixtures built under the test's temporary directory. A parser's package
under `internal/parsers/` tests it over synthetic records shaped like the
ones its research document records, and keeps any binary input it needs
(a SQLite database, a zstd frame, a protobuf blob) under its `testdata`
directory; the scripts beside such a file built it once, and no test or
CI step runs them.

## The fixture image

`internal/collect/imagetest` builds, for tests only, the fake disk image
that `doubleagent collect` is tested against: users' homes holding the
agents' files, credentials, excluded directories, an oversized file,
projects that agents' state names, Docker volumes, startup files that
set relocation variables, links, and names no file system makes easy. A
committed tree or archive could not hold it, since it has names with `"`
and `\`, symbolic links, sockets, named pipes, unreadable directories
and, on Windows, junctions and deny rules; the test replays a manifest
instead, under a temporary directory:
[`image_unix.txt`](../internal/collect/imagetest/testdata/image_unix.txt)
on Linux and macOS and
[`image_windows.txt`](../internal/collect/imagetest/testdata/image_windows.txt)
on Windows.

The image must look like a real disk, so in `image_unix.txt` each home
holds only the paths a real install on one operating system writes
there:

| Home | System | Holds |
| --- | --- | --- |
| `home/alice` | Linux | the agents' files a Linux install writes (dot directories, XDG directories, shell startup files), and the `nested` user's home inside it |
| `home/grace` | Linux | the desktop apps' own directories, `.config/Goose`, `.config/Cursor` and `.config/Devin` (with their caches and `Cookies`), `.config/PearAI` and `.config/Antigravity`, and Atlarix's `.config/Atlarix/Cache`, beside alice's directories whose names differ from them only in letter case |
| `Users/bob` | macOS | `Library`, `Applications` and app bundles, a macOS user's dot directories (among them the Cursor CLI's `.cursor/auth.json`, which only macOS writes there), and links that lead out of the home |
| `Users/heidi` | Windows | `AppData`, the PowerShell profile directories under `Documents`, and `.exe` and `.cmd` files |
| `Users/ivan` | Windows | a second Windows user's `AppData`, among them the lower-case directories (`AppData/Local/crewai`, `AppData/Roaming/devin`) that would share one directory with heidi's capitalised ones |
| `home/dave`, `home/dot`, `home/nl`, `home/nl` followed by a newline, `home/ollama`, `home/u1`, `home/u[1]` | Linux | one shape each: an empty home, links inside and out of a home, names holding a newline or `[`, a service account's home |

`image_windows.txt` follows the same rule, with the Windows home as the
one holding the paths every system shares, since its agents' files name
`C:\` projects:

| Home | System | Holds |
| --- | --- | --- |
| `Users/alice` | Windows | the agents' files a Windows install writes: `AppData`, the PowerShell profile directories, the dot and XDG-style directories an agent uses on Windows too, and the bash startup files Git Bash reads |
| `Users/ivan` | Windows | a second Windows user's `AppData`, among them the lower-case directories (`AppData/Local/crewai`, `AppData/Roaming/devin`) that would share one directory with alice's capitalised ones |
| `home/bob` | Linux | the paths an agent writes only on Linux and macOS (XDG directories with an `AppData` counterpart on Windows, Linux session files, the startup files of shells native Windows does not run, the VS Code remote server), and links that lead out of the home |
| `home/grace` | Linux | the desktop apps' own directories, `.config/Goose`, `.config/Cursor`, `.config/PearAI` and `.config/Antigravity`, and Atlarix's `.config/Atlarix/Cache`, beside bob's directories whose names differ from them only in letter case, and the Linux-only paths whose names bob holds as links |
| `Users/judy` | macOS | `Library`, `Applications` and app bundles, and the dot paths only macOS writes (the Cursor CLI's `.cursor/auth.json`, the Gemini CLI's sandbox directory `.cache/.gemini`) |
| `Users/dave`, `Users/Default`, `Users/Public` | Windows | an empty home and the profiles a collection skips |
| `home/dot`, `home/u1`, `home/u[1]` | Linux | one shape each: links inside and out of a home, a name holding `[` |

No two entries of either manifest differ only in letter case, so the
image is built alike on every file system, and no home holds a `Library`
or `Applications` directory or an app bundle unless it is a macOS home,
or an `AppData` directory, a PowerShell profile directory or a `.exe` or
`.cmd` file unless it is a Windows home. `layout_test.go` in the package
holds the table of homes and their systems, and its tests fail on a
breach; which XDG-style paths an agent also uses on Windows is decided
from its research document, not by the test. A
home holding both spellings of a directory, which a Linux user of a
desktop app and its command-line tool has, is built on purpose under
`root584` by the image's `CaseVariantRoot` method for the `case584` and
`fold32` cases: home `upper` makes the capitalised spelling first and
`lower` the lower-case one, so on a file system that ignores letter case
each pair is one directory spelled as it was made first.

Each line of a manifest is one entry: a path below the image's work
directory, with `/` between names, a kind, its argument, then optional
`name=value` attributes, all single spaces apart:

```text
"root/home/alice/.claude/settings.json" file "{}\n" mode=0600
```

| Part | Holds |
| --- | --- |
| path | a Go double-quoted string, as `strconv.Quote` writes it, so `"` and `\` are escaped with `\`, a byte that is not valid UTF-8 is `\xNN` and a control character its escape (`\n`) |
| kind | `dir`, `file`, `link` (a symbolic link), `dirlink` (a Windows directory symbolic link), `junction` (a Windows junction), `fifo` (a named pipe, Unix only) or `socket` (a Unix domain socket file) |
| argument | for a file its content, as quoted parts or `count*"text"` repeats, so a large file is not stored; for a link or junction its quoted target; nothing for the other kinds |
| `from=work` | the target is a path below the work directory |
| `mode=0700` | the permission bits, on Unix and not for links (default `0644`, `0755` for a directory or socket) |
| `mtime=` | an RFC 3339 modification time (default 2026-10-01T00:00:00Z) |
| `deny=` | the `icacls` rights `RD`, `WD` and `AD`, joined by commas, denied to the current user on the entry alone (Windows only) |

Lines starting with `#` are comments. The entries stay sorted as
`LC_ALL=C sort` orders the lines; the order does not change what is
built, and the package's own tests fail on a line out of order or not
written as the package's formatter writes it. The builder sets every
entry's times and mode, deepest first, so making a child never moves its
directory's time. On Linux and macOS a socket whose path
is longer than `bind` accepts is made through a short link in a new
private directory, under `TMPDIR` or, when that is too long as well, under
`/tmp`.

The tests that compare file modes with expected ones (the expected output
of `collect`, the walk's image of unusual entries and the Docker volume
image test) set the umask to `022` while they run, and the staging copy
test sets its source file's mode itself, so a shell's umask does not
change what they see.

The manifest is edited by hand. When a catalog line gains a fixture, add
the file's line to the home of each operating system the agent's
research document places the path on, in each manifest, in its sorted
place (the builder makes the directories above it),
run `go test ./internal/collect/imagetest`, then rewrite the collection's
expected output on every platform
([Expected output of `collect`](#expected-output-of-collect)): every run
over the image now has a new row. What a single case adds to the image,
or a separate root a case runs over, is built by a method of the image in
`setups_unix.go` or `setups_windows.go`.

## Expected output of `collect`

`TestCollectWritesItsGoldenOutput` in `internal/collect` builds the
program, builds the fixture image and runs `doubleagent collect` over it
once per case, then compares everything the run wrote with the case's
expected files. Every case passes `-r` (the image root unless the case
names another root), then `--inventory`, or `-o`, `-q` and
`--max-file-size 1`, then its own options. The cases are listed in
`golden_cases_unix_test.go` and `golden_cases_windows_test.go`: the
default run, `--no-secrets`, `--full`, `-u`, `--no-docker`,
`--env-scan-home`, `--inventory` and its combinations, extra `-p`
projects, roots with awkward names, relative roots and working
directories, unreadable files and directories, and runs that must fail
before writing anything.

Each case's expected files are under
`internal/collect/testdata/golden/<directory>/<case>/`, one per output:
`stdout.txt`, `stderr.txt` (the log lines a run without `-q` prints there,
without their times, or what a failing run prints), `log.txt` (the log,
named so because git ignores `*.log`), `manifest.jsonl`, `collection.json`,
`archive.txt` (one line per archive member, with its type or hash) and
`exit-code.txt`. An output the run did not write, or an empty stream, has
no file.

| Directory | Read on | Holds |
| --- | --- | --- |
| `unix/` | Linux and macOS | every case's whole expected output |
| `linux/`, `darwin/` | that platform alone | only where that platform's output differs from `unix/`: a file as a difference from `unix/`'s (`<file>.unix-delta`), a whole file, or `<file>.absent` for a file that platform does not write |
| `windows/` | Windows | every Windows case's whole expected output |

A case that collects the main image may keep a file as a line difference
from the default case's file of the same directory, `<file>.delta`, when
that is shorter. Before comparing, the test replaces only what differs
from run to run or machine to machine, and checks the shape of each value
it replaces:

- the output directory, the image's work directory and the run name,
  written `{OUT}`, `{WORK}` and `{NAME}`, in every spelling a path takes;
- the log's timestamps, and its first line's version, host, system and
  account; the archive's size and hash in the log and on stdout; on
  Windows the line saying whether the system updates last-access times;
- `collection.json`'s version, host name, platform, account and start and
  finish times, and the inventory's host;
- in the manifest, the access time (checked only to be a number); the
  change and birth times, the owner ids and the Windows owner, each first
  compared with what the file system reports for the row's path, looked
  at without following a link, since the image outlives the run (zeros
  for a row the format writes without them, and an empty owner where the
  walk gives none); a link's size where its target holds a replaced path;
  and on Linux and macOS a directory's size;
- the order a directory listing gives: manifest rows are sorted, and so
  are runs of project blocks, relocation hints and walk failures in the
  log, and the projects and hints of `collection.json`.

Modes, modification times, sizes, hashes, archive members and every other
text are compared as written. A run of one case takes the subtest's name,
`-run 'TestCollectWritesItsGoldenOutput/<case>$'`, or for a case that runs
beside the others `-run 'TestCollectWritesItsGoldenOutput/shared_image/^<case>$'`.

To accept a change of output, run the test with `DOUBLEAGENT_UPDATE_GOLDEN=1`
on each platform, Linux, macOS and Windows, in turn, on one branch, and
read the diff before committing:

```sh
DOUBLEAGENT_UPDATE_GOLDEN=1 go test ./internal/collect -run 'TestCollectWritesItsGoldenOutput|TestEveryCatalogLineLeavesItsEvidence'
git diff internal/collect/testdata
```

An update keeps what the other platforms expect. On Linux or macOS it
writes `unix/` when the other platform has no files of its own for the
case or nothing is there yet, and otherwise only its own differences, and
it folds both platforms' files into `unix/` when they write the same. A
case directory that names no case of the platform fails the test, and an
update removes it. A case with no expected files fails with a message
naming the variable to set.

## Catalog coverage

`TestEveryCatalogLineLeavesItsEvidence` in `internal/collect` runs the
default, `--no-secrets` and `--full` runs over the fixture image, over it
with the framework volumes, over an image of an agent installed as a
Windows app package (MSIX, the format of Microsoft Store apps), and over
the root of homes holding both spellings of a directory, and needs
evidence of every line of the catalog's tables in what they wrote:

| Table | Evidence |
| --- | --- |
| `CATALOG`, `PROJECT_CATALOG` | a `collected` row of the line's agent on a path the line matches |
| `EXCLUDES` | a `skipped_excluded` row the line explains |
| `SECRET_GLOBS` | a `secret: true` row in a default run, and a `skipped_secret` row under `--no-secrets` |
| `STARTUP_FILES` | a relocation hint read from a file the line matches |
| `RELOCATION_VARS` | a hint naming the line's variable for its agent |
| `DOCKER_VOLUMES` | a row of the line's agent in a volume the line names |

It also fails when the default run collects a path it must not, or when a
run changes the image. A line without evidence is excused only when
[`coverage-uncovered.txt`](../internal/collect/testdata/coverage-uncovered.txt)
lists it with the reason the test derives for this platform:

| Reason | Meaning |
| --- | --- |
| `none` | neither image manifest holds a path the line matches, so no run can show it |
| `unix-only` | only the Linux and macOS manifest does, so Windows runs need no evidence |
| `windows-only` | only the Windows manifest does, so Linux and macOS runs need no evidence |
| `linux-unreached`, `darwin-unreached`, `windows-unreached` | the image holds a path the line matches and no run on that platform reaches it: a more specific catalog entry of another agent, one that lies inside this line's directory, claims it, or it lies outside every catalog entry's walk |

The test fails when the file gives a reason other than the one it
derives. `DOUBLEAGENT_UPDATE_GOLDEN=1` rewrites the reasons this platform
derives and keeps the other platforms' unreached lines, so a catalog line
whose fixture is taken out of one platform's image gets `windows-only` or
`unix-only` written without a failure: read the list's diff after an
update. A line that
leaves `none` or an unreached reason is a missing fixture: add one to the
image ([The fixture image](#the-fixture-image)).

## Expected output of the analysis commands

`TestGoldenOutputOfTheAnalysisCommands` in `internal/analyze` runs
`detect` (plain, `--json`, `--files`), `timeline` and `fleet` over several
inputs, some with options such as `--no-secrets`, `--include-thinking`,
`--since` and `--until` or `--match`, and compares their standard output
and every file they write with the files under
`internal/analyze/testdata/golden/<case>/`. The inputs are the example
collection, `internal/input/testdata/fixtures/example.tar.gz`, read as
the archive it is; a fixture disk image and a fixture home, unpacked as
directories; and the saved `collect --inventory` outputs under
`internal/analyze/testdata/golden-inputs/fleet/`. The runs use a fixed
clock, 2026-10-15T12:00:00Z, and the version `0.0.0-golden`. Only the
working directory's path is replaced, by `{ROOT}`, in every form it is
printed in, and each case names text its standard output must show. An
expected file over 32 KiB is kept gzip-compressed and compared after
decompression; read one with `gunzip -c`. On Windows only the example
archive and `fleet` cases run, since the directory inputs are read with
POSIX paths.

The test also fails on a directory under `internal/analyze/testdata/golden`
that names no case, on a file in a case's directory that the case does
not compare, and on a compressed file whose plain form is beside it, which
the test would never read.
`DOUBLEAGENT_UPDATE_GOLDEN=1 go test ./internal/analyze -run TestGoldenOutputOfTheAnalysisCommands`
rewrites the expected files and removes those; nothing else does. The walk's own
expected rows, log and staging tree for an image of unusual entries
(`internal/walk`, Linux and macOS only, skipped when run as root) are
rewritten the same way.

## Growth tests

`TestEveryParserGrowsInStepWithItsInput` in `internal/growthtests` builds, for
every kind of file each parser reads, a home at a size and at twice that
size in each shape that fits the kind: many records naming one large
value, many records sharing one id, one long value carried by many short
records, deep nesting and many damaged siblings, many small files that
each look at the others, equal join keys, and directories that cannot be
listed. It parses both and counts, never times, what each costs: the rows,
the bytes of every string of every row, the memory allocations and the
bytes allocated. At twice the size, the rows or their bytes may be at most
2.5 times those at the size, and the allocations or bytes allocated at
most 2.5 times plus 2,048 allocations or 256 KiB, the runtime's own work;
and at either size the text of the rows may be at most four bytes per
byte of input plus 16,896 bytes a row. A parse that reads a file again for every
record, or writes a value again into every row, grows with the square of
its input and fails. `TestEveryFileKindHasGrowthCases` fails for a
parser or a kind of file without growth cases. The cases of each parser
are in `internal/growthtests/growth_<package>_test.go`.

## The race detector

CI runs every package's tests with `-race`, which reports two goroutines
touching the same memory without synchronisation. It needs cgo on Linux,
so the test run sets `CGO_ENABLED=1`; the program that ships is always
built with `CGO_ENABLED=0`. The collection tests run the program they
test as a separate process built without the race detector, so their
runs take ordinary time. Under the race detector the growth tests count
only rows and their bytes, since the detector changes what a parse
allocates.

Two more runs check what an ordinary run cannot: on Linux and macOS,
`go test -race -tags dash,noasm,nounsafe ./internal/glob/` compares the
catalog's glob matcher with the `dash` shell's on the forms the tables
hold, with both sides lowered, since the matcher ignores ASCII case, and
`GOEXPERIMENT=nojsonv2 go test -run '^TestArraysAndObjectsNestAtMostTenThousandDeep$' ./internal/jsonvalue/`
checks the JSON reader's own nesting limit in a build where the standard
library's reader has none.

## Reproducible builds

A release build is reproducible or it does not ship: CI runs
`.github/go-build.sh` twice into two directories and fails when the
SHA-256 lists it prints differ:

```sh
sh .github/go-build.sh /tmp/b1 > /tmp/s1 && sh .github/go-build.sh /tmp/b2 > /tmp/s2 && cmp /tmp/s1 /tmp/s2
```

## What CI runs

CI (`.github/workflows/ci.yml`) runs on every pull request and on every
push to `main`, to an integration branch (`go-rewrite`, `go-rewrite-*`)
or to a unit branch (`unit-*`).
[`.github/classify-changes.sh`](../.github/classify-changes.sh) sorts the
files a change touches into categories, and a job or step runs only when
one of its categories is set. A push to `main`, a push whose previous
commit is unknown (a new branch, a force push), a change to the workflow,
the classifier or `.gitattributes`, and a file the classifier does not
know run everything.

| Job | Runs on | When | What it runs |
| --- | --- | --- | --- |
| Base branch is main | Ubuntu | pull requests | fails a pull request whose base is not `main` |
| Classify the change | Ubuntu | always | the classifier |
| Go | Ubuntu, macOS and Windows; Ubuntu alone for a push to a `unit-*` branch | a Go change: `go.mod`, `go.sum`, `vendor/`, any `.go` file, `internal/catalog/catalog.txt`, `.github/go-build.sh`, `THIRD_PARTY_NOTICES.txt` (a test compares it with `vendor/`), or `docs/timeline-output.md` or `docs/internals.md` (Go tests read both pages) | `gofmt`, `go vet`, `staticcheck`, `govulncheck`, `go test -race`, the `dash` glob test (not on Windows), the JSON nesting test, and the reproducible build check |
| Lint | Ubuntu | always; the Python steps for a change under `tools/` or `lab/` (other than a Markdown file or a file under `tools/drift-baseline/`), to `mkdocs/hooks.py`, `ruff.toml`, `pyrightconfig.json`, the catalog, `internal/follow/confine_sites_test.go` or `internal/follow/guard_lists_test.go` (the code style checker's tests read their reviewed long functions), `.github/release-notes.sh` (a test runs it), or the `Makefile` and the root `AGENTS.md` (a test compares their linter versions); the sh steps for an `.sh` file or `.shellcheckrc` | markdownlint, the document link check, the research structure and drift checks and the code style check on every change; ruff, pyright and the tests of the checkers and the lab for a Python change, the tests on Python 3.11 and again on the newest Python release; shfmt, shellcheck and checkbashisms for an sh change |
| Site build (strict) | Ubuntu | always | `mkdocs build --strict` |
| CI result | Ubuntu | always | fails when any job above failed or was cancelled; the one check the ruleset on `main` requires |

A new push to a pull request or to a `unit-*` branch cancels the run
still going for its previous commit; on `main` and the integration
branches a run in progress finishes first. A second workflow,
`.github/workflows/govulncheck.yml`, runs `govulncheck` once a week, on
demand and when `go.mod`, `go.sum` or `vendor/` change, and records what
it finds among the repository's code scanning alerts.
[Validation and testing in `AGENTS.md`](../AGENTS.md#validation-and-testing)
gives the rule each change follows.
