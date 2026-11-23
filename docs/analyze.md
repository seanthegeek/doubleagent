# Analysing

The analysis commands of `doubleagent` read what a collection run gathered,
or any directory tree you already have, and tell you which AI agents left
state in it and what those agents did. They run on the analyst's
workstation, never on the host under investigation, and they never change
the input.

| Command | Use it to | Writes |
| --- | --- | --- |
| `doubleagent detect` | See which users and agents an input holds, with file counts and sizes and whether a parser reads each agent. | stdout only; `--json` prints one JSON object |
| `doubleagent timeline` | Parse every transcript it can read into one timeline of prompts, responses, tool calls, tool results and system events, optionally cut to a time window or a text pattern. | `timeline.jsonl`, `sessions.jsonl` and `inventory.json` in the output directory, and a summary on stdout |
| `doubleagent fleet` | Merge the saved outputs of `doubleagent collect --inventory` from many hosts into one CSV. | one CSV, and a summary per agent on stdout |
| `doubleagent catalog` | Print the catalog the commands detect agents with: the table of agents and the paths under a home directory where each keeps its state. | stdout only |

An input to `detect` or `timeline` can be the archive a collection run
wrote, that archive already extracted, or a loose tree such as a copied
home directory or a mounted disk image ([Inputs and detection](inputs.md)).
`doubleagent --version` prints the version, as `doubleagent <version>`.

## Quick start

Options always come before the input. The paths below are an example case
directory holding one collection.

```sh
cd /cases/wsdev14

# Which users and agents does the collection hold?
doubleagent detect wsdev14_20261015T165610Z_agent-artifacts.tar.gz

# Build the timeline into a new directory
doubleagent timeline wsdev14_20261015T165610Z_agent-artifacts.tar.gz analysis

# Every shell command Claude Code ran, from the timeline
jq -r 'select(.agent == "claude-code" and .tool_name == "Bash") | .text' analysis/timeline.jsonl

# Only 1 to 3 October 2026 (UTC), or only the last three days
doubleagent timeline --since 2026-10-01 --until 2026-10-03 wsdev14_20261015T165610Z_agent-artifacts.tar.gz oct1-3
doubleagent timeline --since 3d wsdev14_20261015T165610Z_agent-artifacts.tar.gz recent

# Only rows whose text mentions curl or wget, in any case, with their tool calls and results
doubleagent timeline --match 'curl|wget' -i wsdev14_20261015T165610Z_agent-artifacts.tar.gz net

# A timeline to share, with the credentials the commands identify replaced
doubleagent timeline --no-secrets wsdev14_20261015T165610Z_agent-artifacts.tar.gz shared

# A home directory copied off a host by other means, or a mounted image
doubleagent timeline --host host02 /mnt/evidence /cases/host02/analysis

# One agent directory copied on its own, under its original name
doubleagent detect --user alice /cases/host03/alice/.claude

# A fleet audit: a directory of saved collect --inventory outputs, one per host
doubleagent fleet -o /cases/fleet/fleet-inventory.csv /cases/fleet/inventory
```

The output directory of `timeline` must be new or empty, and outside the
input. `detect` over the example collection prints:

```text
input:   wsdev14_20261015T165610Z_agent-artifacts.tar.gz
kind:    archive
host:    wsdev14
note:    1 manifest row did not parse
problem: /home/bob/.gemini/tmp/6b9af143446a411aefa3edb76e4693799bd7513e7ed8d919962d6713d3d3b972/chats/session-2026-09-30T08-00-b2c3d4e5.json: missing from the collection

user           home                             agent                 files        bytes  parser
alice          /home/alice                      claude-code              23        40040  yes
alice          /home/alice                      codex-cli                 8        59166  yes
alice          /home/alice                      cursor                   11       100667  yes
alice          /home/alice                      aider                     4         1379  yes
alice          /home/alice                      amp                       1           96  detect only
bob            /home/bob                        gemini-cli               17        20363  yes
bob            /home/bob                        ollama                    6          300  yes

project                                  users          agent                 files        bytes  parser
/srv/proj                                alice          claude-code               1           70  yes
                                                        (shared)                  2          486
  remotes: origin git@github.com:alice/proj.git https://mirror.example.com/alice/proj.git (push https://alice:FIXTURE-TOKEN@github.com/alice/proj.git), upstream https://github.com/upstream/proj.git
```

The `note:` and `problem:` lines say what in the input could not be read;
[Unreadable inputs](inputs.md#unreadable-inputs) lists them. The example
collection is synthetic and its archive is damaged on purpose, so these
lines appear; [The example collection](timeline-output.md#the-example-collection)
describes what it holds.

## What comes out

`timeline` prints a summary and writes three files into its output
directory:

| File | What it holds |
| --- | --- |
| `timeline.jsonl` | One JSON record per line for each turn, tool call, tool result or system event, sorted by time. The `text` field holds the full text of the event, never shortened. |
| `sessions.jsonl` | One JSON record per session: first and last time, models, turn and tool-call counts, and whether the session was synced to a vendor's service, relayed through a messaging platform, started remotely or recovered from a second copy. |
| `inventory.json` | What was detected and what was parsed: the input, its notes and problems, the files per home and agent, the rows and sessions per project. |

The fields of each file, and the summary lines, are in
[Output](timeline-output.md). By default the output shows the evidence as
written, live credentials included; `--no-secrets` replaces the ones the
commands identify, and [Secrets](timeline-output.md#secrets) says which
those are and which it cannot find.

## Exit codes

| Exit code | Meaning |
| --- | --- |
| `0` | Something was found: `detect` found agent files (or `--json` was given), `timeline` wrote at least one row, `fleet` wrote at least one row; also `catalog` and `--help`. |
| `1` | Nothing was found: no agent files, no timeline rows, no inventory rows. `timeline` and `fleet` still write their files. |
| `2` | An error, or a mistake on the command line. |
| `130` | The run was interrupted (Ctrl-C, or `SIGTERM`). |

[Options](analyze-options.md) has every option of every command and the
cases behind each exit code.

## Where to go next

| Page | Covers |
| --- | --- |
| [Inputs and detection](inputs.md) | What an input may be, how host, user, home and agent are attributed, the work directory, what is never followed or written, and the lines for what cannot be read. |
| [Options](analyze-options.md) | Every option of `detect`, `timeline`, `fleet` and `catalog`, with defaults and exit codes. |
| [Output](timeline-output.md) | The `detect` tables and `--json` keys, the `timeline` summary, and every field of `inventory.json`, `timeline.jsonl` and `sessions.jsonl`. |
| [Filtering the timeline](filtering.md) | `--since`, `--until`, `--keep-undated`, `--match` and `--agent`, the time forms they take, and what `sessions.jsonl` then describes. |
| [Reading and searching the output](searching.md) | Cutting a large timeline down with jq, grep and DuckDB, and comparing times. |
| [Fleet inventory](fleet.md) | `doubleagent fleet`: its inputs, CSV columns and summary. |
| [Supported agents](supported-agents.md) | Every agent in the catalog, whether a parser reads it, the files each parser reads and the evidence behind it. |

The evidence behind every parser, with citations and the level each claim
rests on, is one document per agent, listed in the
[research index](../research/README.md).

## License

Copyright 2026 Sean Whalen. Licensed under the Apache License, Version 2.0.
See [LICENSE](../LICENSE).
