# Development

A change to the analyzer is checked by the test suite under `tests/`,
which builds a fake image of synthetic agent state and runs every parser
over it, and by ruff and pyright at the versions CI pins; both run locally
before a push, since CI runs them on every pull request. A parser is
written from the agent's research document and checked against a real
install, whose files are read with a tool that prints their shape and
never a value, so nothing from the install leaves the machine. Several
agents keep their stores as protobuf or SQLite, and the parsers share one
decoder and one opener for them.

## Testing

The suite runs under pytest, which the development requirements add to
the analyzer's own:

```sh
cd analyzer
python3 -m pip install -r requirements-dev.txt
tests/run.sh                                            # the whole suite
tests/run.sh tests/test_catalog.py                      # one module
tests/run.sh tests/parsers/test_claude_code.py          # one parser's tests
tests/run.sh tests/parsers/test_claude_code.py::ClaudeCodeTests     # one class
tests/run.sh tests/parsers/test_claude_code.py::ClaudeCodeTests::test_rows  # one method
tests/run.sh -k Glob -x                                 # pytest options
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
`tests/fixtures/<agent>.py` (the Continue builders in `continue_dev.py`,
since `continue` is a Python keyword), with the helpers they share in
`tests/fixtures/common.py`. `tests/fixtures/__init__.py` re-exports every
builder and value, so a test imports them with `from fixtures import ...`,
and holds `build_home`, which runs the per-agent builders listed in
`HOME_BUILDERS`, in order, to write one user's home.

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
4. Add fixture records to `tests/fixtures/<agent>.py`, with the home
   builder in `HOME_BUILDERS` of `tests/fixtures/__init__.py`, and a test
   class to `tests/parsers/test_<parser>.py`. Then change the agent's row in the
   [agents page](../../docs/agents.md) to `parsed`, and move
   its subsection to [Parsed agents](../../docs/agents.md#parsed-agents) with the
   files the parser reads and the evidence it was validated against.
