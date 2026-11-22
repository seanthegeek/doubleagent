# DoubleAgent

DoubleAgent is a forensic tool for incident responders that shows what AI
agents did on a system. One program, `doubleagent`, collects the on-disk
files of every agent for every user of a host or a mounted disk image into
one archive with a hashed manifest (`doubleagent collect`), and then, on
the analyst's workstation, finds the agents in that archive or in any
directory tree (`doubleagent detect`) and turns their transcripts into one
timeline of prompts, tool calls and results (`doubleagent timeline`). It
supplements the responder's EDR (endpoint detection and response) tool
rather than duplicating it: processes, network connections and the like
are left to the EDR. The source, the issue tracker and the releases are on
[GitHub](https://github.com/seanthegeek/doubleagent).

## Collecting

- [Collecting](collect.md): what a collection run does, the quick start
  for a live host and a mounted image, and the commands to copy.
- [Collection options](collect-options.md): every option of
  `doubleagent collect` and its exit codes.
- [Collection output](collection-output.md): the summary it prints, the
  files it writes, the archive layout and the manifest fields.
- [What gets collected](coverage.md): the catalog's paths, the exclusions
  and the credential files.
- [Relocation warnings](relocation.md): what to do when the log names a
  variable that moves an agent's files.
- [Docker volumes](docker.md): the Docker and Podman named volumes that
  belong to known agents.
- [Privileges](privileges.md): what a run without root or Administrator
  still collects.
- [Windows](windows.md): links and junctions, locked files, long paths and
  the registry on Windows.
- [Deployment through an EDR](deployment.md): running a collection through
  an EDR remote shell, console timeouts and run time.
- [Inventory mode](collect-inventory.md): `collect --inventory`, the
  one-line-per-agent audit for a whole fleet.

## Analysing

- [Analysing](analyze.md): the analysis commands, their quick start and
  what each one prints and writes.
- [Analysis options](analyze-options.md): every option of `detect`,
  `timeline`, `fleet` and `catalog`, and their exit codes.
- [Inputs and detection](inputs.md): the inputs the analysis commands
  accept and how they find the agents in them.
- [Analysis output](timeline-output.md): the output of `detect` and
  `timeline`, and the fields of `timeline.jsonl`, `sessions.jsonl` and
  `inventory.json`.
- [Filtering the timeline](filtering.md): time windows and text matches.
- [Reading and searching the output](searching.md): working through a
  large timeline with common command-line and database tools.
- [Fleet inventory](fleet.md): merging many hosts' inventory lines into
  one CSV with `doubleagent fleet`.

## Reference

- [Agents](agents.md): every agent in the catalog, what is collected for
  it, whether its transcripts are parsed and the evidence behind that.
- [Changelog](CHANGELOG.md): what changed in each version.

## Research

[Research](../research/README.md) holds one document per agent, the
evidence behind its catalog entries and its parser: where the agent keeps
its state on each platform, which files are credentials, how it records
its transcripts and what survives a delete. The index gives each
document's evidence level and parse status.

## Contributing

- [Development](development.md): building, the build tags, the tests and
  the quality checks, and where to start on a parser or a catalog agent.
- [Internals](internals.md): how a collection run works inside.
- [Testing](testing.md): how the project is tested, and what CI runs on
  each platform.
- [Lab VMs](lab-vms.md): the clean virtual machines that agents are
  installed in to watch what they write, and how to rebuild them.
- [Lab](../lab/README.md): the pipeline that installs one agent in a lab
  VM, drives it step by step, lists what it wrote after every step and
  collects the result.
- [AGENTS.md](../AGENTS.md): the standards every change keeps and the
  processes for adding an agent or a parser.

## License

Copyright 2026 Sean Whalen. Licensed under the Apache License, Version 2.0.
See [LICENSE](../LICENSE).
