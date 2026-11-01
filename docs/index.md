# DoubleAgent

Forensic analysis of AI agent use on a system, for incident responders.
It covers agents that execute tools or shell commands on a host and leave
transcripts, configuration or credentials on disk: coding agents such as
Claude Code, Gemini CLI, Antigravity, Codex CLI, Copilot CLI, Cursor,
Windsurf, Aider and VS Code chat extensions; autonomous agents such as
Hermes, Agent Zero and OpenClaw; agent framework studios and CLIs such as
AutoGen Studio, CrewAI and MetaGPT; self-hosted agent and workflow
automation platforms such as n8n, Flowise, Langflow and Dify; voice
daemons that hand coding tasks to other agents, such as Qwen Live; Ollama;
and others. The [agents page](agents.md) lists every one of them with what the
collector gathers and whether the analyzer parses its transcripts.

DoubleAgent has two parts that run in different places. The collectors run
on the host under investigation, or against a mounted disk image, and
gather the on-disk artifacts of every agent for every user into one archive
with a hashed manifest. The analyzer runs on the analyst's workstation,
reads that archive, and turns the transcripts into one timeline of prompts,
tool calls and results. The source, the issue tracker and the releases are
on [GitHub](https://github.com/seanthegeek/doubleagent).

## Collectors

Two single-file scripts with the same catalog (the table of agents and the
paths under a home directory where each keeps its state), manifest schema
and archive layout: a POSIX `sh` collector for macOS, Linux and BSD, and a
Windows PowerShell 5.1 collector. Each depends on nothing beyond its base
system, so it can be uploaded and run once through an EDR remote shell
(CrowdStrike RTR, SentinelOne RemoteOps, Defender Live Response, Palo Alto
Networks Cortex XDR Live Terminal), run locally by a responder, or pointed
at a mounted disk image of any of the three platforms. Output is one
`tar.gz` (a `.zip` from the Windows collector where `tar.exe` is missing)
holding the collected files, a hashed JSONL manifest and a run summary.
With `--inventory` they write nothing and print one JSON line per user and
agent found instead, for a fleet-wide audit through the EDR console.

Start with the [collectors README](../collectors/README.md) for the quick
start and the option table, then:

- [Options](../collectors/docs/options.md): every option, exit code and
  environment variable.
- [Output](../collectors/docs/output.md): the stdout summary, the output
  files, the archive layout and the manifest schema.
- [Coverage](../collectors/docs/coverage.md): what is collected and from
  where, exclusions and credential files.
- [Docker and Podman](../collectors/docs/docker.md): named volumes that
  belong to known agents.
- [Privileges](../collectors/docs/privileges.md): what a run without root
  or Administrator still collects.
- [Inventory mode](../collectors/docs/inventory.md): the fleet audit
  lines.
- [Windows](../collectors/docs/windows.md): archivers, symlinks, locked
  files and long paths.
- [Deployment](../collectors/docs/deployment.md): running through an EDR
  remote shell, console timeouts and runtime.
- [Testing](../collectors/docs/testing.md) and
  [Internals](../collectors/docs/internals.md): for developers changing
  the collectors.

## Analyzer

`doubleagent`, a Python command-line tool that reads a collector archive,
an extracted collection, or any loose directory such as a copied home or a
mounted image, detects which agents left state in it using the collectors'
own catalog, and parses the transcripts it understands into a normalised
JSONL timeline of turns, tool calls and tool results, plus a per-session
summary. It needs Python 3.10 or later and installs with
`pip install ./analyzer`.

Start with the [analyzer README](../analyzer/README.md) for the quick
start, the commands and the option table, then:

- [Inputs](../analyzer/docs/inputs.md): accepted inputs, loose mode and
  how agents are detected.
- [Output](../analyzer/docs/output.md): the `detect` and `timeline`
  output and the `timeline.jsonl` and `sessions.jsonl` schemas.
- [Filtering](../analyzer/docs/filtering.md): time windows and text
  matches.
- [Searching](../analyzer/docs/searching.md): reading large timelines
  with jq, grep, DuckDB, Python and PowerShell.
- [Inventory](../analyzer/docs/inventory.md): merging many hosts'
  inventory lines into one CSV.
- [Options](../analyzer/docs/options.md): every option and exit code.
- [Development](../analyzer/docs/development.md): tests, lint checks and
  adding a parser.

## Research

[Research](../research/README.md) holds one document per agent in two
parts. The Collection part is the evidence behind every catalog entry:
where the agent keeps its state, which files are credentials and what the
collector excludes. The Parsing part is the evidence behind every parser:
how the agent records its transcripts and how the parser reads them. The
index lists every document with its evidence level and parse status, and
which records survive a delete.

## Agent lab

A disposable Docker sandbox for running CLI agents against a scratch home
and exporting it for the collector, so real-install test material can be
produced without installing agents on a workstation. See the
[agent lab README](../lab/README.md).

## Lab VMs

The clean virtual machines that the
[real-install rounds](../AGENTS.md#real-install-rounds) start from: the
development tools their snapshots hold so an agent's use of them can be
recorded, the install commands that rebuild them on Windows, macOS and
Ubuntu, what the host decides, and Docker's requirements in each. See
[Lab VMs](lab-vms.md).

## Contributing

The standards every change must keep, the process for adding an agent or
a parser, and the quality gates CI runs are in
[AGENTS.md](../AGENTS.md).

## License

Copyright 2026 Sean Whalen. Licensed under the Apache License, Version 2.0.
See [LICENSE](../LICENSE).
