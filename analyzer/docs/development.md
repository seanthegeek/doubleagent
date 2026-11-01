# Development

A change to the analyzer is checked by the test suite under `tests/`,
which builds a fake image of synthetic agent state and runs every parser
over it, and by ruff and pyright at the versions CI pins; both run
locally before a push, since CI runs them on every pull request that
changes Python. A parser is written from the agent's research document
and checked against a real install, whose files are read with a tool
that prints their shape and never a value, so nothing from the install
leaves the machine. Several agents keep their stores as protobuf or
SQLite, and the parsers share one decoder and one opener for them.

## Testing

The suite runs under pytest, which the development requirements add to
the analyzer's own:

```sh
cd analyzer
python3 -m pip install -r requirements-dev.txt
tests/run.sh                                                      # the whole suite
tests/run.sh tests/test_catalog.py                                # one module
tests/run.sh tests/parsers/test_claude_code.py                    # one parser's tests
tests/run.sh tests/parsers/test_claude_code.py::ClaudeCodeTests   # one class
tests/run.sh tests/parsers/test_claude_code.py::ClaudeCodeTests::test_rows  # one method
tests/run.sh -k Glob -x                                           # pytest options
```

Each parser's tests are one module, `tests/parsers/test_<parser>.py`, named
after the parser's module under `doubleagent/parsers/`. The shared readers
have no module of their own: `cline_legacy.py` is tested through the Cline
and Roo Code tests, and `codex_state.py` through the Codex tests in
`test_codex.py`.
`tests/parsers/base.py` holds the `ParserBase` class the test classes
extend, which builds the fixture home for each test, and the helpers
several modules use; `tests/parsers/test_shared.py` holds the tests of the
helpers the parsers share and of the outputs and routing across parsers.
The fixture builders are one module per catalog agent,
`tests/fixtures/<agent>.py`, with the hyphens of the catalog name as
underscores (`codex_cli.py` holds the fixtures that `test_codex.py` reads)
and the Continue builders in `continue_dev.py`, since `continue` is a
Python keyword; the helpers they share are in `tests/fixtures/common.py`.
`tests/fixtures/__init__.py` imports and re-exports every builder and
value, so a test imports them with `from fixtures import ...` and a new
name is added to its import list; it also holds `build_home`,
which runs the per-agent builders listed in `HOME_BUILDERS`, in order, to
write one user's home.

`tests/run.sh` changes to the `analyzer/` directory and runs
`python3 -m pytest` with its arguments, so it can be run from anywhere and
takes every pytest option and node id. The `[tool.pytest.ini_options]` table
of `pyproject.toml` sets `testpaths` to `tests` and puts `tests/` on the
import path, so `python3 -m pytest` run from `analyzer/` is the same. Most
tests are `unittest.TestCase` classes, which pytest collects as they are;
newer ones, such as `test_check_code_style.py`, are plain pytest functions
using fixtures such as `tmp_path`.

The suite builds a fake image with two Linux users and a Windows profile
tree, each holding synthetic state for every parsed agent in the exact shapes
the parsers were validated against, and checks detection in every input mode,
each parser's rows, the JSONL output, and the bundled catalog against the
collector's `--list`. When `sh` and `tar` are available it also runs the sh
collector on the fake image and analyses the resulting archive end to end.
`test_inventory.py` feeds `inventory` captured outputs of three hosts, one
with console junk lines and one with a `host` line only, and checks the
columns, the default file name, `-o`, the rollup and the empty-host row;
with `sh` available it also runs the collector's `--inventory` on the fake
image and reads that. Tests that need the collector script, `sh` and `tar`,
or permission to create symlinks are skipped, with the reason, when it is
missing. CI runs the suite on Python 3.10, 3.11, 3.12, 3.13 and 3.14 with
`requirements-dev.txt` installed.

Lint, type and style checks, from the repository root, at the versions CI
pins (see "Quality gates" in [AGENTS.md](../../AGENTS.md#quality-gates)).
ruff's pydocstyle rules require a Google-style docstring on every public
module, class, function and method outside the test methods, and
`tools/check_code_style.py` checks names, comments and docstring lengths
against "Code style" in [AGENTS.md](../../AGENTS.md#code-style), printing
each failure as `path:line: message`:

```sh
uvx ruff@0.16.10 check . && uvx ruff@0.16.10 format --check .
uvx --from pyright==1.1.414 --with zstandard --with python-dateutil pyright
python3 tools/check_code_style.py
```

## Drift checkers

Each agent's research document under [`research/`](../../research/README.md)
is the source the collector's catalog tables, the smoke tests, the
parsers and the test fixtures are written from. A place where one of them
says something the document does not is drift, and five checkers under
`tools/` report it. CI runs them in its lint job on every change, a change
to the Markdown documents alone included, and runs the checkers' own
tests beside the analyzer suite. Run them from the repository root.
`check_fixture_schema.py` imports the fixtures package, so it needs
`analyzer/requirements.txt` installed; the other checkers use the
standard library alone, and their own tests need pytest from
`analyzer/requirements-dev.txt`:

```sh
python3 tools/check_tables_research.py      # catalog tables against the Collection part
python3 tools/check_smoke_coverage.py       # catalog tables against both smoke tests
python3 tools/check_parser_research.py      # parsers against the Parsing part
python3 tools/check_fixture_schema.py       # test fixtures against the Parsing part
python3 tools/check_research_structure.py   # the documents' outline and conventions
python3 -m pytest tools/tests -q            # the checkers' own tests
```

| Checker | Compares | What it reports |
| --- | --- | --- |
| `check_tables_research.py` | every `CATALOG`, `PROJECT_CATALOG`, `EXCLUDES`, `SECRET_GLOBS` and `DOCKER_VOLUMES` line with the Collection part of its agent's document (`research/shared.md` for the `shared` and `project` lines) | a line no code span names, a credential file the Credentials section names, unless its sentence says it is not one, that no secret glob flags and the Secrets section does not list, a secret glob an exclusion hides, an exclusion or secret glob that no home or project entry reaches and no one Docker volume agent's document names, a Docker volume glob that the document's Per-user storage subsection about the volume does not name, a missing Docker volume form of an exclusion, two agents' home entries whose first differing segment differs only by case or whose first segments differ and can still match one path, the wording "collected unflagged", "left unflagged", "not flagged", "may embed" or "can embed" in Credentials, a catalog agent without a document or an agents page row, a parser fact under Collection |
| `check_smoke_coverage.py` | every table line with `collectors/tests/smoke.sh` and `collectors/tests/smoke.ps1`, read as text | a secret glob with no `secret: true` or no `skipped_secret` check, an exclusion with no `skipped_excluded` check, a catalog line no fixture file matches, a fixture file whose name the document does not give, a `smoke.ps1` check naming a row by a suffix, an agent with a `secret` case in the frameworks case list and no `--no-secrets` check of `skipped_secret` on one, a check or path the reader cannot follow |
| `check_parser_research.py` | each parser in `parsers.ALL`, read with Python's `ast` module, with the Parsing part of its agent's document | a row text, `parser:` message or record key no code span or fenced line gives, a file pattern that neither appears unescaped nor has a path or glob span outside Fixtures shaped like it, a `records_sync`, `records_relay`, `records_origin` or `has_secondary` flag the Parser plan contradicts, a file or key a `Not read:` list names that the parser reads, a module docstring fact the document lacks |
| `check_fixture_schema.py` | the files `build_home` leaves on disk, so not a record a later builder overwrites, and the fixture modules' docstrings with the Parsing part of each agent's document | a builder without a `Source:` line, a module that names no document, a key, table, column or enum value the document lacks (and so do the documents of the agents whose fixture modules it imports from), a value whose type a key table contradicts, a project path the module does not declare, another agent's builder called without a `Lineage:` line, a Fixtures sample identical to a fixture record |
| `check_research_structure.py` | each research document with "Research document structure" in [AGENTS.md](../../AGENTS.md#research-document-structure) | outline breaches, which always fail, and, against the baseline, an install entry under `### Observed` without its bold platform label or build, or whose list of earlier builds is malformed or not at its end, two such entries for one platform and product form, a version named without a Format versions entry or link, a fact stated twice, documentation named without a link |

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
`<checker>|<kind>|<detail>` line per finding, sorted. A finding missing
from the baseline is new and fails CI: fix the table, document, parser or
fixture that disagrees. A baseline line that no finding produces any more
is stale and fails too, printed as
`stale: remove "<line>" from tools/drift-baseline/<agent>.txt`, so the
commit that fixes the drift also deletes its line; when an agent is
rebuilt from fresh evidence, its file empties and is deleted.
`--update-baseline` rewrites one checker's lines in every file from the
current findings, drops a malformed line and names it on stderr, and
exits `0` (`check_research_structure.py` still exits `1` on an outline
breach). It may add lines only in the commit that adds
or changes a checker's rule, recording the breaches the new rule finds,
and that commit's message body says why; a finding an edit of yours causes
is fixed, never added to the baseline.

## Branch coverage

A parser's fixture holds the present and the absent form of every key the
parser branches on, so the suite takes each branch both ways.
`tools/check_branch_coverage.py` measures that for the parser modules a
change touches, with [coverage.py](https://coverage.readthedocs.io/) in
branch mode at the version `requirements-dev.txt` pins. CI runs it in the
analyzer job on its newest Python, against the pull request's base branch.
From the repository root, with `requirements-dev.txt` installed:

```sh
python3 tools/check_branch_coverage.py                   # parser lines changed since origin/main
python3 tools/check_branch_coverage.py --base main       # against another ref
python3 tools/check_branch_coverage.py --all             # every parser module, every line
```

It finds the modules under `doubleagent/parsers/` that differ between the
working tree and the merge base of `--base` with HEAD, so uncommitted
edits count, and the untracked modules git does not ignore, each of whose
lines counts as changed. When there is none it prints
`no parser module changed against <ref>` on stdout and exits 0. Otherwise
it runs the whole suite once from `analyzer/` under
`coverage run --branch`, measuring only those modules, and prints one
line per finding on stdout, for a line the change added or modified:

| Line | Meaning |
| --- | --- |
| `<file>:<line>: branch to <line> never taken` | the suite never went from this line to that one |
| `<file>:<line>: branch to exit never taken` | the suite never left the function from this line |
| `<file>:<line>: pragma: no branch without a reason` | the line carries `# pragma: no branch` with no `: <reason>` after it |
| `<file>:<line>: pragma: no cover without a reason` | the line carries `# pragma: no cover`, which drops its whole block from measurement, with no `: <reason>` after it |
| `<file>:1: not measured: the suite never imports it` | no test imports the changed module |

A branch is reported on the line where it starts, the first line of a
statement that spans several, and a change to any line of a statement's
header, such as the second line of an `if` condition, checks the branches
of that first line. A branch no fixture can reach, such as a guard
against a value the format never writes, carries
`# pragma: no branch: <reason>` on that line, which coverage.py and the
check both accept. The summary goes to stderr, and the exit status is 0
with no finding, 1 with one, and 2 when git, the suite or coverage.py
fails, with the failing step's output.

## Regenerating the output examples

The plain [`detect`](output.md#detect) output, the
[`detect --json`](output.md#detect---json) output, the
[`timeline`](output.md#timeline) stdout and the
[`inventory.json`](output.md#inventoryjson),
[`timeline.jsonl`](output.md#timelinejsonl) and
[`sessions.jsonl`](output.md#sessionsjsonl) examples in
[`output.md`](output.md) are generated, not hand-written; its other
example blocks, such as the fragments showing variant lines, are written
by hand. `example_collection.py` and `regenerate_examples.py` under
`tools/` make them.

`example_collection.py` builds the synthetic collection that
[The example collection](output.md#the-example-collection) describes:
host `wsdev14`, the users `alice` and `bob` and the repository
`/srv/proj`, with fixed times and ids. No collector gathered it: the
module writes its `collection.json` and `manifest.jsonl` in the
collector's documented shape, and every file's contents are invented,
in the record shapes of the test fixtures and the research documents'
samples, built from real installs (Claude Code 2.1.310 on Linux and
macOS, Codex CLI 0.162.1 on Linux) and the agents' source.
To write it to disk, give it an empty directory; it writes
`<directory>/wsdev14/` with `collection.json`, `manifest.jsonl`,
`collector.log` and `fs/`. From the repository root:

```sh
mkdir example
python3 analyzer/tools/example_collection.py example
```

`regenerate_examples.py` builds the same collection in a temporary
directory, runs `detect`, `detect --json --files` and `timeline -o` on it
in the same Python process, and writes the output between the comments
that enclose each block,
`<!-- generated by analyzer/tools/regenerate_examples.py: <name> -->`
and `<!-- end generated: <name> -->`. The collection's temporary path is
printed as `/cases/wsdev14/wsdev14` and the `timeline` output directory
as `out`; no other value of the output is changed. The script changes
only the text between the comments, and replaces the file in one rename,
so a failed write leaves it whole. From the `analyzer/` directory:

```sh
python3 tools/regenerate_examples.py                     # rewrite the blocks in docs/output.md
python3 tools/regenerate_examples.py --check             # print a diff per stale block
python3 tools/regenerate_examples.py --document FILE     # work on FILE instead of docs/output.md
```

A rewrite that succeeds exits 0. `--check` writes nothing and exits 1
when a block differs from a fresh run, 0 when every block matches. In
either mode the script exits 2 and leaves the document unchanged when
its marker comments are not each block's begin and end comment, in order
and once each, when the document cannot be read or is not UTF-8 with LF
line ends, when a `detect` or `timeline` run fails, or when the new file
cannot be written. A `--document` that is a symbolic link is followed,
so its target is rewritten.

`tests/test_examples.py` fails on a stale block and on each of these
errors. A change that shows in the collection's output leaves the suite
failing until the script is run, such as a change to the rows of the
`claude-code`, `codex-cli`, `aider`, `ollama`, `cursor` or `gemini-cli`
parsers, to the lines `detect` or `timeline` prints, to the keys of
`detect --json` or `inventory.json`, to `example_collection.py` itself,
or a `VERSION` bump, since `inventory.json` names the analyzer version.
The prose around each example says what to notice in it, and is written
by hand.

## Reading a real install

`tools/shape.py` shows the files an agent wrote on a real install, a lab VM or
the author's own machine. It prints the shape of the files and never a value,
so its output can go into a research document or a report. It uses the
standard library only and imports nothing from `doubleagent`, so the single
file can be copied to a VM and run there.

```sh
python3 -I tools/shape.py ~/.config/Agent
python3 -I tools/shape.py --enums --depth 3 settings.json sessions/
python3 -I tools/shape.py --max-size 20000000 --skip logs ~/.agent
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
and the walk continues. `tests/test_tools_shape.py` checks on synthetic files
that tokens, UUIDs, paths, URLs and prompt text do not reach the output.

## Protobuf and SQLite

Protobuf stores are decoded by `doubleagent/protobuf.py`, a small
schema-driven wire decoder, from field tables written into each parser. For
a closed-source agent the field names come from the descriptors embedded in
its binary. `tools/proto_descriptors.py extract BINARY OUT.json` extracts
them, and `show OUT.json MESSAGE...` and `find OUT.json TEXT` query them. SQLite
stores are copied together with their `-wal` and `-shm` sidecars into a
scratch directory before being opened, so the evidence copy is never touched
and rows still in the write-ahead log are not lost.

## Adding a parser

The full checklist, including the fixture, test class, agents table row,
version bump and the lists to update, is "Adding a parser" in
[AGENTS.md](../../AGENTS.md#adding-a-parser). In short:

1. Start from the Parsing part of the agent's research document, listed in
   the [research index](../../research/README.md). Confirm the record
   format against source or a real install and write the field names into
   the module docstring, as the existing parsers do.
2. Subclass `Parser` in `doubleagent/parsers/<agent>.py`: set `agent` to
   the catalog name, implement `wants` on `artifact.rel` and `parse` yielding
   `Row` objects. Use `iter_jsonl` so a truncated line is reported, not fatal.
   Timestamps go through `to_utc`; text through `compact`, which strips
   leading and trailing whitespace and keeps inner line breaks.
3. Register it in `doubleagent/parsers/__init__.py`. A parser that must
   read a file the catalog attributes to another agent sets `reads_agents`.
4. Add fixture records to `tests/fixtures/<agent>.py`, derived from the
   research document's samples, each builder's docstring naming the
   document heading its records come from in a line
   `Source: research/<agent>.md#<anchor>; build <version>`, with the home
   builder in `HOME_BUILDERS` of `tests/fixtures/__init__.py` and each new
   name in that file's imports, and a test class to
   `tests/parsers/test_<parser>.py`. Then change the agent's row in the
   [agents page](../../docs/agents.md) to `parsed`, and move
   its subsection to [Parsed agents](../../docs/agents.md#parsed-agents) with the
   files the parser reads and the evidence it was validated against.
