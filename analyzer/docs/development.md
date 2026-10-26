# Development

A change to the analyzer is checked by the unittest suite under `tests/`,
which builds a fake image of synthetic agent state and runs every parser
over it, and by ruff and pyright at the versions CI pins; both run locally
before a push, since CI runs them on every pull request. A parser is
written from the agent's research document and checked against a real
install, whose files are read with a tool that prints their shape and
never a value, so nothing from the install leaves the machine. Several
agents keep their stores as protobuf or SQLite, and the parsers share one
decoder and one opener for them.

## Testing

```sh
cd analyzer
tests/run.sh                 # python3 -m unittest discover -s tests -t tests
tests/run.sh -v -k Glob      # options only: passed to unittest discover
tests/run.sh test_catalog    # one module
tests/run.sh test_parsers.ClaudeCodeTests            # one class
tests/run.sh test_parsers.ClaudeCodeTests.test_rows  # one method
```

`tests/run.sh` changes to the `analyzer/` directory itself, so it can be run
from anywhere. What it runs depends on whether a test name is given:

| Arguments | Command run | Options accepted |
| --- | --- | --- |
| none, or options only | `python3 -m unittest discover -s tests -t tests <options>` | every `unittest discover` option, including `-p`, `-k`, `-v`, `-f` |
| one or more test names, module, `module.Class` or `module.Class.method` | `python3 -m unittest <options> <names>` with `tests/` prepended to `PYTHONPATH`, so the modules import under the same names as under discover | the `unittest` options `-v`, `-q`, `-k`, `-f`, `-c`, `-b`, `--locals`; `-p`, `-s` and `-t` (and their long forms) are discover-only, so the script prints `run.sh: <option> applies only without test names` and exits 2 |

An argument that does not start with `-` and is not the value of `-k`,
`-p`, `-s`, `-t` or `--durations` counts as a test name.

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
`requirements.txt` installed.

## Updating the output examples

The real-run blocks in [`output.md`](output.md) come from a small collected
fixture built from the Windows, Linux and macOS Claude Code records in
`tests/fixtures.py`. After changing the CLI output, regenerate the blocks from
the repository root and check them with the analyzer suite:

```sh
python3 analyzer/tools/regenerate_output_examples.py
cd analyzer && tests/run.sh test_output_examples
```

The script builds an extracted collection from those fixtures, runs `detect`
and `timeline`, and updates the marked JSON, JSONL and stdout blocks. `--check`
runs the same fixture and fails when the documentation is stale.

Lint and type checks, from the repository root, at the versions CI pins
(see "Quality gates" in [AGENTS.md](../../AGENTS.md#quality-gates)):

```sh
uvx ruff@0.16.10 check . && uvx ruff@0.16.10 format --check .
uvx --from pyright==1.1.414 --with zstandard --with python-dateutil pyright
```

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
4. Add fixture records to `tests/fixtures.py` and a test class to
   `tests/test_parsers.py`. Then change the agent's row in the
   [agents page](../../docs/agents.md) to `parsed`, and move
   its subsection to [Parsed agents](../../docs/agents.md#parsed-agents) with the
   files the parser reads and the evidence it was validated against.
