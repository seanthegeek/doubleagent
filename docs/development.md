# Development

This page is for a contributor who builds, tests or changes DoubleAgent.
The rules every change keeps, and the processes for adding an agent or a
parser, are in [AGENTS.md](../AGENTS.md); this page says how to set up and
run the work.

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

A development build for this machine:

```sh
CGO_ENABLED=0 go build -o doubleagent ./cmd/doubleagent
./doubleagent --version
```

It prints `doubleagent 0.0.0-dev` followed by the commit. The release
builds come from `.github/go-build.sh`, which builds every release target
(macOS, Linux and Windows, each on amd64 and arm64) into the directory
given, as `doubleagent-<os>-<arch>` (with `.exe` on Windows), and prints
the SHA-256 of each:

```sh
sh .github/go-build.sh /tmp/doubleagent-builds
```

It builds with `CGO_ENABLED=0`, `-trimpath` and an empty build id, so two
builds of one commit are byte for byte the same. `VERSION` in the
environment sets the version the builds report.

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
git diff internal/*/testdata
```

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
| `tools/check_research_structure.py`, `tools/check_tables_research.py` | the research documents, as [Drift checkers](#drift-checkers) describes |
| site build | `mkdocs build --strict`, which fails on a broken page link or a page missing from the site's navigation |

## Drift checkers

Each agent's research document under [`research/`](../research/README.md)
is the source the catalog (`internal/catalog/catalog.txt`, which
`doubleagent catalog` prints) is written from. A place where the catalog
or a document's own outline disagrees with the rules is drift, and two
checkers under `tools/` report it. CI runs them on every change, a change
to the Markdown documents alone included. Run them from the repository
root:

```sh
python3 tools/check_tables_research.py      # the catalog against the Collection part
python3 tools/check_research_structure.py   # the documents' outline and conventions
python3 -m pytest tools/tests -q            # the checkers' own tests
```

| Checker | Compares | What it reports |
| --- | --- | --- |
| `check_tables_research.py` | every line of the catalog's tables with the Collection part of its agent's document (`research/shared.md` for the `shared` and `project` lines) | a line no code span of the document names; a credential file the Credentials section names that no secret glob flags and the Secrets section does not list; a secret glob an exclusion hides; an exclusion or secret glob that no entry reaches; a Docker volume glob the document's volume subsection does not name, or an exclusion below a volume's mount directory without its volume-relative twin; two agents' home entries that differ only by case or can match one path; forbidden wording in a Credentials section; a catalog agent without a document or a row on the [agents page](agents.md), and a row naming no catalog agent; a parsing fact under Collection; a relocation variable line or a startup file glob that its document does not name, or names differently |
| `check_research_structure.py` | each research document with "Research document structure" in [AGENTS.md](../AGENTS.md#research-document-structure) | outline breaches, which always fail, and, against the baseline, an Observed entry without its bold platform label or build, or whose list of earlier builds is malformed or not at its end, two such entries for one platform and product form, a version named without a Format versions entry or inline link, a fact stated twice, documentation named without an inline link |

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
[Adding a parser in `AGENTS.md`](../AGENTS.md#adding-a-parser): the
package under `internal/parser/`, its registration, its fixtures and
tests, its growth cases and the pages to update.

A new agent for the catalog starts from its source or its vendor's
documentation, as
[Adding an agent to the catalog in `AGENTS.md`](../AGENTS.md#adding-an-agent-to-the-catalog)
and
[Validating catalog entries from source in `AGENTS.md`](../AGENTS.md#validating-catalog-entries-from-source)
describe; its lines go into `internal/catalog/catalog.txt`, and a fixture
for each into the fake disk image
([The fixture image](testing.md#the-fixture-image)).
[Internals](internals.md) describes how a collection run walks the
catalog.
