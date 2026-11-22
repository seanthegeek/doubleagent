# Development

A parser is written from the agent's research document and checked against
a real install, whose files are read with a tool that prints their shape
and never a value, so nothing from the install leaves the machine. Several
agents keep their stores as protobuf or SQLite, and the parsers share one
decoder and one reader for them.

## Testing

The Go tests and gates, and the commands that run them, are under "Quality
gates" in [AGENTS.md](../AGENTS.md#quality-gates). The Python that
remains, the checkers under `tools/` and the lab VM pipeline under
`lab/round/`, is checked with ruff, pyright and
`tools/check_code_style.py`, which checks names, comments and docstring
lengths against "Code style" in [AGENTS.md](../AGENTS.md#code-style),
printing each failure as `path:line: message`. From the repository root:

```sh
uvx ruff@0.16.10 check . && uvx ruff@0.16.10 format --check .
uvx --from pyright==1.1.414 --with pytest pyright
python3 tools/check_code_style.py
```

## Drift checkers

Each agent's research document under [`research/`](../research/README.md)
is the source the collector's catalog tables and the smoke tests are
written from. A place where one of them says something the document does
not is drift, and three checkers under `tools/` report it. CI runs them in
its lint job on every change, a change to the Markdown documents alone
included, and runs the checkers' own tests there too. Run them from the
repository root. The checkers use the standard library alone, and their
own tests need pytest:

```sh
python3 tools/check_tables_research.py      # catalog tables against the Collection part
python3 tools/check_smoke_coverage.py       # catalog tables against both smoke tests
python3 tools/check_research_structure.py   # the documents' outline and conventions
python3 -m pytest tools/tests -q            # the checkers' own tests
```

| Checker | Compares | What it reports |
| --- | --- | --- |
| `check_tables_research.py` | every `CATALOG`, `PROJECT_CATALOG`, `EXCLUDES`, `SECRET_GLOBS` and `DOCKER_VOLUMES` line with the Collection part of its agent's document (`research/shared.md` for the `shared` and `project` lines) | a line no code span names, a credential file the Credentials section names, unless its sentence says it is not one, that no secret glob flags and the Secrets section does not list, a secret glob an exclusion hides, an exclusion or secret glob that no home or project entry reaches and no one Docker volume agent's document names, a Docker volume glob that the document's Per-user storage subsection about the volume (one whose heading or opening paragraph says "volume" and names the volume in a code span: its name, its `DOCKER_VOLUMES` glob or its catalog line) does not name, a missing Docker volume form of an exclusion, two agents' home entries whose first differing segment differs only by case or whose first segments differ and can still match one path, the wording "collected unflagged", "left unflagged", "not flagged", "may embed" or "can embed" in Credentials, a catalog agent without a document or an agents page row, a parser fact under Collection |
| `check_smoke_coverage.py` | every table line with `collectors/tests/smoke.sh` and `collectors/tests/smoke.ps1`, read as text | a secret glob with no `secret: true` or no `skipped_secret` check, an exclusion with no `skipped_excluded` check, a catalog line no fixture file matches, a fixture file whose name the document does not give, a `smoke.ps1` check naming a row by a suffix, an agent with a `secret` case in the frameworks case list and no `--no-secrets` check of `skipped_secret` on one, a check or path the reader cannot follow |
| `check_research_structure.py` | each research document with "Research document structure" in [AGENTS.md](../AGENTS.md#research-document-structure) | outline breaches, which always fail, and, against the baseline, an install entry under `### Observed` without its bold platform label or build, or whose list of earlier builds is malformed or not at its end, two such entries for one platform and product form, a version named without a Format versions entry or inline link, a fact stated twice, documentation named without an inline link |

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
(`duplicate:`). A finding missing
from the baseline is new and fails CI: fix the table, document or smoke
test that disagrees. A baseline line that no finding produces any more
is stale and fails too, printed as
`stale: remove "<line>" from tools/drift-baseline/<agent>.txt`, so the
commit that fixes the drift also deletes its line; when an agent is
rebuilt from fresh evidence, its file empties and is deleted.
`--update-baseline` rewrites one checker's lines in every file from the
current findings, sorts the file and drops a repeated line, drops a
malformed line and names it on stderr, and
exits `0` (`check_research_structure.py` still exits `1` on an outline
breach). It may add lines only in the commit that adds
or changes a checker's rule, recording the breaches the new rule finds,
and that commit's message body says why; a finding an edit of yours causes
is fixed, never added to the baseline.

## The output examples

The plain [`detect`](timeline-output.md#detect) output, the
[`detect --json`](timeline-output.md#detect---json) output, the
[`timeline`](timeline-output.md#timeline) stdout and the
[`inventory.json`](timeline-output.md#inventoryjson),
[`timeline.jsonl`](timeline-output.md#timelinejsonl) and
[`sessions.jsonl`](timeline-output.md#sessionsjsonl) examples in
[`timeline-output.md`](timeline-output.md) are example output of the commands over
[the example collection](timeline-output.md#the-example-collection), each between
an `<!-- example output: <name> -->` and an `<!-- end example output: <name> -->`
comment; its other example blocks, such as the fragments showing variant
lines, are written by hand. The prose around each example says what to
notice in it, and is written by hand.

## Reading a real install

`lab/round/tools/shape.py` shows the files an agent wrote on a real install, a lab VM or
the author's own machine. It prints the shape of the files and never a value,
so its output can go into a research document or a report. It uses the
standard library only and imports nothing else from the repository, so the
single file can be copied to a VM and run there.

```sh
python3 -I lab/round/tools/shape.py ~/.config/Agent
python3 -I lab/round/tools/shape.py --enums --depth 3 settings.json sessions/
python3 -I lab/round/tools/shape.py --max-size 20000000 --skip logs ~/.agent
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
and the walk continues. `lab/round/tests/test_shape.py` checks on synthetic files
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

## Adding a parser

The full checklist, including the fixture, test class, agents table row,
version bump and the lists to update, is "Adding a parser" in
[AGENTS.md](../AGENTS.md#adding-a-parser). Start from the Parsing part
of the agent's research document, listed in the
[research index](../research/README.md), and confirm the record format
against source or a real install.
