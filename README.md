# DoubleAgent

DoubleAgent is a forensic tool for incident responders who need to know
what an AI agent did on a system. It covers agents that execute tools or
shell commands on a host and leave transcripts, configuration or
credentials on disk: coding agents such as Claude Code, Gemini CLI,
Antigravity, Codex CLI, Copilot CLI, Cursor, Windsurf, Aider and VS Code
chat extensions; autonomous agents such as Hermes, Agent Zero and
OpenClaw; agent framework studios and CLIs such as AutoGen Studio, CrewAI
and MetaGPT; self-hosted agent and workflow automation platforms such as
n8n, Flowise, Langflow and Dify; voice daemons that hand coding tasks to
other agents, such as Qwen Live; Ollama; and others. The
[agents page](docs/agents.md) lists every one of them.

DoubleAgent supplements the responder's EDR (endpoint detection and
response) tool and does not duplicate it: process lists, logged-on users,
network connections, services, scheduled tasks and shell histories are
left to the EDR. What DoubleAgent adds is the agents' own files and a
timeline of what each agent was asked and did.

## One program

DoubleAgent is one program, `doubleagent`, with these commands:

- `doubleagent collect` archives the on-disk files of AI agents for every
  user of a host, or of a mounted disk image, with a hashed manifest.
- `doubleagent detect` lists the homes, users and agents a collection, or
  any directory tree, holds.
- `doubleagent timeline` parses the transcripts in a collection into one
  timeline of prompts, tool calls and results, with a summary per session.
- `doubleagent fleet` merges the saved output of many hosts'
  `collect --inventory` runs into one CSV.
- `doubleagent catalog` prints the bundled catalog: the agents, and the
  paths under a home directory where each keeps its state.

`doubleagent --help` and `doubleagent <command> --help` print each
command's options.

## Quick start

Get the `doubleagent` build for the host's operating system and
architecture from the
[releases page](https://github.com/seanthegeek/doubleagent/releases). The
builds are named `doubleagent-<os>-<arch>`, with `.exe` after the
Windows ones; until a release is published,
build them from a checkout as [Development](docs/development.md#building)
describes.

On the host under investigation, as root or from an elevated prompt so
every user's home can be read:

```sh
doubleagent collect -o /var/tmp/ir
```

Or, on an analyst workstation, against a disk image mounted at
`/mnt/evidence`:

```sh
doubleagent collect -r /mnt/evidence -o /cases/host01
```

Either run writes `<host>_<time>_agent-artifacts.tar.gz` into the output
directory, with its SHA-256 (`.sha256`), the manifest, the run summary
and the log beside it. On
the analyst's workstation, turn the archive into a timeline:

```sh
doubleagent timeline /cases/host01/<host>_<time>_agent-artifacts.tar.gz /cases/host01/timeline
```

The output directory then holds `timeline.jsonl`, `sessions.jsonl` and
`inventory.json`. By default these show the credentials found in the
evidence as the evidence holds them; `--no-secrets` withholds the ones
the [output page](docs/timeline-output.md) lists.

## What it reads and what it never does

`doubleagent collect` reads the agents' files under every home directory
and in the projects their state names, the Docker and Podman volumes that
belong to known agents, and the startup files that can move an agent's
files elsewhere. It copies each file into a staging directory under `-o`,
records its metadata as found before the copy, hashes the copy, packs
everything into one `.tar.gz` and removes the staging directory. It
parses no transcript.

- It never modifies, moves or deletes anything outside the output
  directory.
- It never follows a symbolic link: it records each one with its target
  and stores the link itself in the archive.
- It records everything it skipped (excluded directories, oversized
  files, credential files under `--no-secrets`) in the manifest, with a
  status and a size.
- It prompts for nothing and reads nothing from standard input, so it can
  be uploaded and run once through an EDR remote shell.

The analysis commands (`detect`, `timeline`, `fleet`) run wherever the
analyst runs them, normally a workstation, over an archive, an extracted
collection, or any directory such as a copied home or a mounted image.
They write only what they are asked for (the timeline's output
directory, the fleet CSV) and, for an archive, the directory they extract
it into, which is removed afterwards unless `--keep-extracted` is given;
they never follow a symbolic link in the evidence.

## Platforms

Builds are made for macOS, Linux and Windows, each on x86-64 (amd64) and
arm64. Each build collects its own host, and any build can collect a
mounted disk image of a macOS, Linux or Windows system, because every
catalog entry is tried against every home directory.

## Documentation

The [documentation site](https://seanthegeek.net/doubleagent/) holds:

- [Collecting](docs/collect.md): running `doubleagent collect`, its
  options and its output.
- [Analysing](docs/analyze.md): running the analysis commands, their
  inputs and their output.
- [Agents](docs/agents.md): every agent in the catalog with what is
  collected and whether its transcripts are parsed.
- [Research](research/README.md): the evidence behind every catalog entry
  and parser, one document per agent.
- [Development](docs/development.md): building, testing and changing
  DoubleAgent.

A vulnerability in DoubleAgent itself is reported privately as
[SECURITY.md](SECURITY.md) describes. Bugs and feature requests go
through the issue forms on GitHub.

## License

Copyright 2026 Sean Whalen. Licensed under the Apache License, Version 2.0.
See [LICENSE](LICENSE).
