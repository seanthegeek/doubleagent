# DoubleAgent analyzer

`doubleagent` is a forensic analyzer of AI agent use on a system. It takes
an archive from the doubleagent collectors, an extracted collection, or any
loose directory tree such as a copied home directory or a mounted disk image,
detects which AI agents left state in it, and parses the transcripts it
knows how to read into one normalised JSONL timeline. Detection uses the
catalog, the collectors' table of agents and the paths under a home
directory where each keeps its state, bundled with the analyzer. It runs
on the analyst's workstation, never on the host under investigation, so
unlike the collectors it may carry dependencies.

It needs Python 3.11 or later. `pip install .` in this directory installs
the `doubleagent` package and the `doubleagent` command with their two
required dependencies: `python-dateutil`, which reads the `--since` and
`--until` times, and `zstandard`, which decompresses Zed threads, Codex and
Open Interpreter `.jsonl.zst` rollouts and OpenClaw's compressed transcript
rows. To run from a checkout without installing, `pip install -r
requirements.txt` and run `python3 -m doubleagent` from this directory.
`--version` prints `doubleagent <version>`. [CHANGELOG.md](CHANGELOG.md) lists what changed in
each version, including changes to the `timeline.jsonl` and `sessions.jsonl`
fields.

## Quick start

```sh
# What is in this collection?
doubleagent detect /cases/host01/host01_20261003T165531Z_agent-artifacts.tar.gz

# Build the timeline
doubleagent timeline /cases/host01/host01_*.tar.gz -o /cases/host01/analysis

# Every Bash command Claude Code ran, from the timeline
jq -r 'select(.agent == "claude-code" and .tool_name == "Bash") | .text' /cases/host01/analysis/timeline.jsonl

# Only 1 October 2026 (UTC), or only the last three days
doubleagent timeline /cases/host01/host01_*.tar.gz -o /cases/host01/oct1 --since 2026-10-01 --until 2026-10-01
doubleagent timeline /cases/host01/host01_*.tar.gz -o /cases/host01/recent --since 3d

# Only events whose text mentions curl or wget, with the paired tool calls and results
doubleagent timeline /cases/host01/host01_*.tar.gz -o /cases/host01/net --match 'curl|wget' -i

# A home directory copied off a host by other means, or a mounted image
doubleagent timeline /mnt/evidence -o /cases/host02/analysis --host host02

# A single agent directory copied on its own, under its original name
doubleagent detect /cases/host03/alice/.claude --user alice

# Fleet inventory: one saved collector --inventory stdout per host in a directory
doubleagent inventory /cases/fleet/inventory -o /cases/fleet/fleet-inventory.csv
```

## Commands

| Command | What it does | Writes | Page |
| --- | --- | --- | --- |
| `detect` | Reports the host, users, homes and agents found in an input, with file counts, bytes and whether a parser exists, in one table for homes and, for a collection, one for project directories. | stdout only; `--json` for one object | [docs/output.md](docs/output.md#detect) |
| `timeline` | Parses every transcript it can read into one timeline, optionally filtered by time or text. | `timeline.jsonl`, `sessions.jsonl` and `inventory.json` in `-o` | [docs/output.md](docs/output.md#timeline), [docs/filtering.md](docs/filtering.md) |
| `inventory` | Merges saved collector `--inventory` output from many hosts into one CSV. | `fleet-inventory.csv`, or the `-o` path | [docs/inventory.md](docs/inventory.md) |
| `catalog` | Prints the bundled catalog, or with `--agents` each agent with `parser` or `detect only`. | stdout only | [docs/inputs.md](docs/inputs.md#detection) |

## Options

```text
doubleagent detect INPUT [--json] [--files] [common options]
doubleagent timeline INPUT -o DIR [--include-thinking] [--agent NAME]...
                         [--since WHEN] [--until WHEN] [--keep-undated] [--match REGEX]... [-i]
                         [common options]
doubleagent inventory [-o FILE] INPUT...
doubleagent catalog [--agents]
doubleagent --version
```

| Option | Meaning |
| --- | --- |
| `--json` | `detect`: print one machine-readable object. |
| `--files` | `detect`: also list every attributed file. |
| `-o, --output` | `timeline`: output directory, required. `inventory`: CSV path. |
| `--include-thinking` | `timeline`: emit thinking and reasoning blocks as `thinking` rows. |
| `--agent NAME` | `timeline`: run only this agent's parsers. Repeatable. |
| `--since WHEN`, `--until WHEN` | `timeline`: keep rows inside this window. |
| `--keep-undated` | `timeline`: with a window, also keep rows without a timestamp. |
| `--match REGEX` | `timeline`: keep rows whose `text` matches, with the paired call or result. Repeatable. |
| `-i, --ignore-case` | `timeline`: make every `--match` case-insensitive. |
| `--agents` | `catalog`: list agents and whether each has a parser. |
| `--host NAME` | Host name to record; overrides `collection.json`. |
| `--user NAME` | User to record for every home whose user is empty. |
| `--work-dir DIR` | Where to extract an archive. |
| `--keep-extracted` | Keep the extracted archive. |

[docs/options.md](docs/options.md) has the full meaning of each option,
which subcommands take it, and the full exit-code table.

| Exit code | Meaning |
| --- | --- |
| `0` | Rows or agents were found, or the command always succeeds (`detect --json`, `catalog`, `--version`). |
| `1` | Nothing was found: no timeline rows, no agent artifacts, or no inventory lines. |
| `2` | The input cannot be opened or extracted, an option value is invalid, or the command line is invalid. |

## Parsers

The [agents page](../docs/agents.md) lists every agent in the catalog
with its status (parsed, a parser backlog, or collected with nothing to
parse), the files each parser reads and the evidence it was validated
against. Thinking and reasoning blocks are left out unless
`--include-thinking` is passed.

## Documentation

| Page | Covers |
| --- | --- |
| [docs/inputs.md](docs/inputs.md) | Accepted inputs, loose mode, unreadable inputs and `problem:` lines, detection from the bundled catalog, routing files to parsers |
| [docs/output.md](docs/output.md) | `detect` output and `--json` keys, `timeline` files and stdout, the `inventory.json`, `timeline.jsonl` and `sessions.jsonl` schemas |
| [docs/filtering.md](docs/filtering.md) | `--since`, `--until`, `--keep-undated` and `--match`, the `WHEN` forms and periods |
| [docs/searching.md](docs/searching.md) | Reading and searching large output with jq, grep, DuckDB, Python and PowerShell |
| [docs/inventory.md](docs/inventory.md) | The `inventory` command, its CSV columns and rollup |
| [docs/options.md](docs/options.md) | Every option with its subcommands, and the exit codes |
| [docs/development.md](docs/development.md) | Running the tests and lint checks, reading a real install, protobuf and SQLite stores, adding a parser |

The evidence behind every parser, with citations and the level each claim
rests on, is one document per agent, listed in the
[research index](../research/README.md).

## License

Copyright 2026 Sean Whalen. Licensed under the Apache License, Version 2.0.
See [LICENSE](../LICENSE).
