# Development

This page is for a contributor who builds, tests or changes DoubleAgent.
The rules every change keeps, and the processes for adding an agent or a
parser, are in [AGENTS.md](../AGENTS.md) and the area files it lists; this page says how to set up and
run the work.

## Repository layout

```text
cmd/doubleagent/       the program: command dispatch and exit codes
internal/<package>/    the program's packages, one per concern; docs/internals.md maps them
internal/catalog/catalog.txt  the seven catalog tables, the one place they are written
go.mod, go.sum, vendor/  the Go module, its pinned toolchain and its vendored dependencies
THIRD_PARTY_NOTICES.txt  the licences of the Go distribution and of every vendored module
Makefile               the entry point for building and checking; `make` builds into dist/
dist/                  build output, ignored by git
docs/                  the documentation, one page per topic
CHANGELOG.md           what changed in each version
research/              one document per catalog agent, the research index and survival table
lab/                   the lab VM pipeline for real installs (lab.sh), its step tables and tools
tools/                 the document and research checkers, with their tests under tools/tests/
tools/drift-baseline/  the drift checkers' recorded disagreements, one file per agent
mkdocs/                the documentation site's configuration and build hooks
requirements-docs.txt  the documentation site's Python packages, pinned
.github/               CI (ci.yml, classify-changes.sh), the release build (go-build.sh,
                       release.yml, release-notes.sh), Pages, govulncheck, Dependabot
ruff.toml, pyrightconfig.json, .markdownlint-cli2.jsonc, .shellcheckrc  linter configuration
README.md, SECURITY.md, LICENSE  the GitHub overview, the security policy, Apache 2.0
AGENTS.md, CLAUDE.md   the contributor rules, at the root and in each area directory
```

## Prerequisites

- **Go**, at the version the `go` line of [`go.mod`](../go.mod) names.
  Every Go module the program uses is copied under `vendor/`, so a build
  needs no network.
- **Python 3.11 or later**, only for the document and research checkers
  under `tools/`, the lab pipeline under `lab/`, and the scripts under a
  few packages' `testdata` that build binary test files (SQLite
  databases, zstd frames); no test runs those scripts. The checkers and the
  lab tools use the standard library alone; their own tests need pytest.
- **Node.js**, for `npx` and markdownlint, and **uv**, for the pinned ruff
  and pyright and the documentation site build, as the commands in
  [Quality gates in `AGENTS.md`](../AGENTS.md#quality-gates) run them.

## Build tags

Every `go` command of the project runs with the build tags `noasm` and
`nounsafe`. The zstd decoder DoubleAgent uses for compressed evidence
ships assembly and memory reads through `unsafe` pointers that skip Go's
bounds checks; the two tags leave both out, so compressed data from the
evidence is only ever decoded by bounds-checked Go. Set them once per
shell:

```sh
export GOFLAGS=-tags=noasm,nounsafe
```

Without them, `internal/zstdframe` does not compile, and a build or
`go vet` stops with `undefined: buildWithTagsNoasmAndNounsafeOnly`. A
command that passes `-tags` itself replaces `GOFLAGS`, so it names both
tags beside its own (`-tags dash,noasm,nounsafe`).

## Building

The [`Makefile`](../Makefile) at the repository root is the entry point.
It is written in POSIX make syntax and tested with GNU Make 3.81, the
`make` of macOS, and every target
passes the build tags itself, so it needs no `GOFLAGS`. The binaries go
into `dist/`, which git ignores, named `doubleagent-<os>-<arch>` (with
`.exe` on Windows). A build for this machine:

```sh
make build
dist/doubleagent-darwin-arm64 --version    # the name of this machine's build
```

It prints `doubleagent 0.0.0-dev` followed by the commit, and `-modified`
when the checkout has uncommitted changes. `make` alone builds the six
release targets (macOS, Linux and Windows, each on amd64 and arm64) and
prints the SHA-256 of each:

```sh
make
make VERSION=1.2.3     # the version the binaries report, as a release sets it
```

Both run `.github/go-build.sh`, the one recipe for a release build, which
the CI and release workflows run too. It builds with `CGO_ENABLED=0`,
`-trimpath` and an empty build id, so two builds of one commit are byte
for byte the same on any machine. `make reproducible` builds the six
targets twice in temporary directories and fails unless the hashes agree.

| Target | What it does |
| --- | --- |
| `make` or `make all` | the six release binaries in `dist/`, with their hashes |
| `make build` | this machine's binary in `dist/` |
| `make test` | `go test ./...`, the quick run while working |
| `make race` | the tests under the race detector, as CI runs them |
| `make vet` | `go vet ./...` for Linux, macOS and Windows |
| `make fmt-check` | fails when `gofmt` would change a file, naming it |
| `make staticcheck`, `make govulncheck` | each at the version CI pins; `govulncheck` needs the network |
| `make test-dash`, `make test-nojsonv2` | the two extra test runs of the CI `go` job: the glob tests against dash, which must be installed, and the JSON depth test without `encoding/json` v2 |
| `make check` | the Go gates of the CI `go` job, in its order: `fmt-check`, `vet`, `staticcheck`, `govulncheck`, `race`, `test-dash`, `test-nojsonv2`, `reproducible` |
| `make reproducible` | two builds of the six targets, compared |
| `make clean` | removes `dist/` |

A release is built by the release workflow,
[`.github/workflows/release.yml`](../.github/workflows/release.yml), when
the maintainer pushes a version tag; [Releases](#releases) describes it.

### Releases

A release starts when the maintainer pushes a tag `v<version>`, such as
`v0.1.0` or the pre-release `v0.2.0-rc.1`, after the version's section
`## [<version>] - <YYYY-MM-DD>` is in [`CHANGELOG.md`](../CHANGELOG.md). The
release workflow then:

1. fails unless `main` holds the tagged commit, before anything is built;
2. builds the six targets with that version on an Ubuntu and a macOS
   runner, and fails unless the two runners built the same bytes;
3. checks that the Linux binary reports the version, that the changelog
   has the section, and that `THIRD_PARTY_NOTICES.txt` matches the
   vendored modules;
4. fails when the tag already has a release, draft or published;
5. attests each binary's build provenance, signed through GitHub, which
   `gh attestation verify` checks;
6. creates a draft GitHub Release titled with the version (without the
   `v`), with the changelog section as its notes and the six binaries,
   `SHA256SUMS`, `LICENSE` and `THIRD_PARTY_NOTICES.txt` attached; a
   version with a `-` suffix is marked a pre-release.

The maintainer reads the draft and publishes it by hand. Running the
workflow from the Actions tab with a version is a dry run: it builds,
compares and checks for that version, from the branch it is started on,
and creates no attestation and no release.

[`THIRD_PARTY_NOTICES.txt`](../THIRD_PARTY_NOTICES.txt) holds the licence
of the Go distribution and of each module under `vendor/`, copied from the
files; a test in `internal/version` fails when a module is added, removed
or moved to another version without the file following it.

## Running the tests

```sh
go test ./...
```

CI runs the tests under the race detector, which needs cgo on Linux:

```sh
CGO_ENABLED=1 go test -race ./...
```

The tests never read your home directory or the network, and the
collection tests run `doubleagent collect` only over a fake disk image
they build under a temporary directory. Do the same in any test or hand
run: give `-r` a real image directory built for it, never an empty or
computed value, since a run without a root collects this machine's own
credentials. [Testing](testing.md) describes each kind of test and what
CI runs on each platform.

### Expected-output tests

The collection tests and the analysis commands' tests compare whole
outputs with expected files committed under the packages' `testdata`
directories. After a change that alters an output on purpose, rewrite the
expected files and read the diff before committing it:

```sh
DOUBLEAGENT_UPDATE_GOLDEN=1 go test ./internal/collect ./internal/analyze ./internal/walk
git diff internal/*/testdata docs/timeline-output.md
```

The same run rewrites the example output blocks of
[`docs/timeline-output.md`](timeline-output.md), which a test of
`internal/analyze` compares with the program's output
([The output examples](#the-output-examples)), so the page's diff is read
too.

The collection's expected files differ by platform, so a change to them is
made on Linux, macOS and Windows;
[Expected output of `collect`](testing.md#expected-output-of-collect) says
how.

## Quality gates

CI runs these checks; the exact commands and the pinned versions are in
[Quality gates in `AGENTS.md`](../AGENTS.md#quality-gates), which is the
authority for both. Run the ones a change touches from the repository
root before pushing.

| Check | What it checks |
| --- | --- |
| `gofmt -l cmd internal` | prints nothing: the Go code is formatted |
| `go vet ./...` | the Go toolchain's own static checks |
| `staticcheck` | further static analysis of the Go code |
| `govulncheck` | no known vulnerability in a Go module or the standard library that the code can reach |
| `go test -race ./...` | the tests, under the race detector |
| reproducible build | `.github/go-build.sh` run twice prints the same hashes |
| `tools/check_code_style.py` | names, comments and docstring lengths of every Go, Python, sh and PowerShell file against "Code style" in [AGENTS.md](../AGENTS.md#code-style), each failure printed as `path:line: message` |
| ruff and pyright | the Python of the checkers and the lab |
| markdownlint | every Markdown file |
| `tools/check_doc_links.py` | every relative link and heading fragment in the Markdown files, and the link and term rules of AGENTS.md |
| `tools/check_research_structure.py`, `tools/check_tables_research.py`, `tools/check_parser_research.py` | the research documents, as [Drift checkers](#drift-checkers) describes |
| site build | `mkdocs build --strict`, which fails on a broken page link or a page missing from the site's navigation |

### Installing the linters

shellcheck, shfmt and checkbashisms are not on PyPI or npm: install the
release binaries (CI checks their sha256), and checkbashisms as the single
`scripts/checkbashisms.pl` from the devscripts tag or
`apt install devscripts`. PSScriptAnalyzer, which only the lab's Windows
guest scripts need, installs with
`pwsh -NoProfile -Command 'Install-Module PSScriptAnalyzer -Scope CurrentUser -RequiredVersion 1.25.0 -Force'`,
or from the nupkg of the
[PSScriptAnalyzer release](https://github.com/PowerShell/PSScriptAnalyzer/releases)
on GitHub, unzipped into the user module path, when the PowerShell
Gallery's v2 API fails, which it does intermittently. It runs over the
guest scripts with:

```sh
pwsh -NoProfile -Command '$r = @(Get-ChildItem -Path lab -Recurse -Filter *.ps1 | ForEach-Object { $_.FullName }) | ForEach-Object { Invoke-ScriptAnalyzer -Path $_ -Settings lab/PSScriptAnalyzerSettings.psd1 }; $r | Format-Table -AutoSize; exit @($r).Count'
```

The one `ruff format` commit is listed in `.git-blame-ignore-revs`; enable
it with `git config blame.ignoreRevsFile .git-blame-ignore-revs`.

## Drift checkers

Each agent's research document under [`research/`](../research/README.md)
is the source the catalog (`internal/catalog/catalog.txt`, which
`doubleagent catalog` prints) is written from. A place where the catalog
or a parser, or a document's own outline, disagrees with the rules is
drift, and the checkers under `tools/` report it. CI runs them on every change, a change
to the Markdown documents alone included. Run them from the repository
root:

```sh
python3 tools/check_tables_research.py      # the catalog against the Collection part
python3 tools/check_research_structure.py   # the documents' outline and conventions
python3 tools/check_parser_research.py      # the parsers against the Parsing part; needs go
python3 -m pytest tools/tests -q            # the checkers' own tests
```

| Checker | Compares | What it reports |
| --- | --- | --- |
| `check_tables_research.py` | every line of the catalog's tables with the Collection part of its agent's document (`research/shared.md` for the `shared` and `project` lines) | a line no code span of the document names; a credential file the Credentials section names that no secret glob flags and the Secrets section does not list; a secret glob an exclusion hides; an exclusion or secret glob that no entry reaches; a Docker volume glob the document's volume subsection does not name, or an exclusion below a volume's mount directory without its volume-relative twin; two agents' home entries that differ only by case or can match one path; forbidden wording in a Credentials section; a catalog agent without a document or a row on the [agents page](supported-agents.md), and a row naming no catalog agent; a parsing fact under Collection; a relocation variable line or a startup file glob that its document does not name, or names differently |
| `check_research_structure.py` | each research document with "Research document structure" in [`research/AGENTS.md`](../research/AGENTS.md#research-document-structure) | outline breaches, which always fail, and, against the baseline, an Observed entry without its bold platform label or build, or whose list of earlier builds is malformed or not at its end, two such entries for one platform and product form, a version named without a Format versions entry or inline link, a fact stated twice, documentation named without an inline link |
| `check_parser_research.py` | each registered parser's file patterns, record keys, row texts and problem messages, which `go test ./tools/parserfacts` reads from the Go code, with the code spans and fenced lines of the Parsing part of its agent's document, and the patterns of the files it reads that repeat another store's events with the agent's row of the [survival table](../research/README.md#where-transcripts-live-and-what-survives) | a row text, problem message, file pattern or key the Parsing part does not name; such a pattern the agent's survival table row does not name, or a parser with such patterns and no row |

Each checker prints one line per finding,
`<path>[:<line>]: <kind> [<agent>]: <detail> - <explanation>`, such as:

```text
research/zed.md:154: parsing-fact-under-collection [zed]: folder_paths - a Record schema key belongs in the Parsing part
```

and a summary of the counts on stderr, and exits `1` on a new finding, a
stale baseline line or a malformed one, `0` otherwise.

The drift that existed when the checkers were written is recorded in the
baseline, `tools/drift-baseline/<agent>.txt`, one file per agent
(`unattributed.txt` for a finding that names none) holding one
`<checker>|<kind>|<detail>` line per finding, sorted, each once; every
checker fails on a line out of order (`unsorted:`) or repeated
(`duplicate:`). A finding missing from the baseline is new and fails CI:
fix the catalog line or the document that disagrees. A baseline line that
no finding produces any more is stale and fails too, printed as
`stale: remove "<line>" from tools/drift-baseline/<agent>.txt`, so the
commit that fixes the drift also deletes its line; when an agent is
rebuilt from fresh evidence, its file empties and is deleted.
`--update-baseline` rewrites one checker's lines in every file from the
current findings, sorts the file and drops a repeated line, drops a
malformed line and names it on stderr, and exits `0`
(`check_research_structure.py` still exits `1` on an outline breach). It
may add lines only in the commit that adds or changes a checker's rule,
recording the breaches the new rule finds, and that commit's message body
says why; a finding an edit of yours causes is fixed, never added to the
baseline.

## The output examples

The plain [`detect`](timeline-output.md#detect) output, the
[`detect --json`](timeline-output.md#detect---json) output, the
[`timeline`](timeline-output.md#timeline) output and the
[`inventory.json`](timeline-output.md#inventoryjson),
[`timeline.jsonl`](timeline-output.md#timelinejsonl) and
[`sessions.jsonl`](timeline-output.md#sessionsjsonl) examples in
[`timeline-output.md`](timeline-output.md) are output of the commands over
[the example collection](timeline-output.md#the-example-collection),
`internal/input/testdata/fixtures/example.tar.gz`, each between an
`<!-- example output: <name> -->` and an `<!-- end example output: <name> -->`
comment, with the command line that writes it in a one-line `sh` block
just above. `TestTheOutputPageShowsWhatEachExampleCommandPrints` in
`internal/analyze` runs each of those command lines and fails when a block
differs from what the command prints, or when the page shows another
command line. After a change to the output, rewrite the blocks with

```sh
DOUBLEAGENT_UPDATE_GOLDEN=1 go test ./internal/analyze -run TestTheOutputPageShowsWhatEachExampleCommandPrints
```

on Linux or macOS (the test is skipped on Windows, since the page shows
POSIX paths), and read the page's diff: only the text inside the blocks'
fences changes. The page's other example blocks, such as the fragments
showing variant lines, and the prose around each example are written by
hand.

## Reading a real install

`lab/tools/shape.py` shows the files an agent wrote on a real install, a lab VM or
the author's own machine. It prints the shape of the files and never a value,
so its output can go into a research document or a report. It uses the
standard library only and imports nothing else from the repository, so the
single file can be copied to another machine and run there.

```sh
python3 -I lab/tools/shape.py ~/.config/Agent
python3 -I lab/tools/shape.py --enums --depth 3 settings.json sessions/
python3 -I lab/tools/shape.py --max-size 20000000 --skip logs ~/.agent
```

Each PATH is a file or a directory walked recursively; symlinks are listed
as `symlink, not followed` and never followed. Output is one block per file
in sorted path order, headed `== <path> (<size> bytes, <kind>)` with the path
relative to the PATH argument:

| File | Printed |
| --- | --- |
| `.json`, or any file that parses as one JSON document | the key tree, one key per line; arrays as `[n x <shape of the first element>]` or `[]`; scalars as `bool`, `number`, `null`; strings as `str<uuid>`, `str<hex32>`, `str<hex64>`, `str<iso-date>`, `str<abs-path>`, `str<url>` or `str(len n)`; objects deeper than `--depth` as `{...}` |
| `.jsonl`, `.ndjson` | line count per `type` value (or per top-level key set when there is no `type`), the union of top-level keys per type, the `subtype` breakdown of `system` lines, the content block types of `user` and `assistant` `message.content` lists, and `bad lines <n>` |
| anything else | the size and `not JSON`, plus for a file under 1 KiB the character class `hex`, `base64`, `printable` or `binary` |

It never prints a string value, a line of a JSONL file or file content.
Keys print as they are, except that a UUID inside a key or a path becomes
`<uuid>`, a run of 32 or more hex digits `<hex>`, an e-mail address
`<email>`, and a key that is itself an absolute path or a URL becomes
`<abs-path>` or `<url>`; keys that are equal after that print once with a
count, `<abs-path> (x 3)`. The `type` and `subtype` values of JSONL lines
are printed, with the same redaction, because they are what the summary
counts.

| Option | Default | Effect |
| --- | --- | --- |
| `--max-size BYTES` | 1048576 | a larger file prints its size and `skipped (large)` |
| `--skip NAME` | `Cache`, `Code Cache`, `GPUCache`, `DawnGraphiteCache`, `DawnWebGPUCache`, `blob_storage`, `Crashpad`, `Session Storage`, `Local Storage`, `IndexedDB`, `WebStorage`, `Shared Dictionary`, `node_modules`, `.git` | a directory name not to enter, repeatable and added to the defaults; a skipped directory prints `directory, skipped` |
| `--depth N` | 6 | object nesting depth to print |
| `--enums` | off | a string of at most 16 characters from `a-z`, `0-9`, `_`, `-` prints as `str=<value>`, since such strings are usually enum values; a value under a key whose name contains `token`, `secret`, `key`, `passw`, `cred`, `auth`, `cookie`, `bearer` or `salt` keeps the `str(len n)` form |

The exit status is 0; an unreadable file prints `unreadable (<ErrorClass>)`
and the walk continues. `lab/tests/test_shape.py` checks on synthetic files
that tokens, UUIDs, paths, URLs and prompt text do not reach the output.

## Protobuf and SQLite

Protobuf stores are decoded by `internal/protowire`, a small
schema-driven wire decoder, from field tables written into each parser. For
a closed-source agent the field names come from the descriptors embedded in
its binary. `tools/proto_descriptors.py extract BINARY OUT.json` extracts
them, and `show OUT.json MESSAGE...` and `find OUT.json TEXT` query them. SQLite
stores are read by `internal/sqlitefile`, which reads the file format
itself, runs no SQL and writes nothing, so the evidence copy is never
touched and rows still in the write-ahead log or a rollback journal are
not lost.

## Adding a parser or a catalog agent

A parser is written from the agent's research document and checked against
a real install, whose files are read with the shape tool above, so nothing
from the install leaves the machine. Start from the Parsing part of the
agent's research document, listed in the
[research index](../research/README.md), and follow
[Adding a parser in `internal/parsers/AGENTS.md`](../internal/parsers/AGENTS.md#adding-a-parser): the
package under `internal/parsers/`, its registration, its fixtures and
tests, its growth cases and the pages to update.

A new agent for the catalog starts from its source or its vendor's
documentation, as
[Adding an agent to the catalog in `internal/catalog/AGENTS.md`](../internal/catalog/AGENTS.md#adding-an-agent-to-the-catalog)
and
[Validating catalog entries from source in `research/AGENTS.md`](../research/AGENTS.md#validating-catalog-entries-from-source)
describe; its lines go into `internal/catalog/catalog.txt`, and a fixture
for each into the fake disk image
([The fixture image](testing.md#the-fixture-image)).
[Internals](internals.md) describes how a collection run walks the
catalog.
