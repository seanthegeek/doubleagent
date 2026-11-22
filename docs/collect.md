# Collecting

`doubleagent collect` gathers the on-disk state of AI agents (their
transcripts, histories, settings and credential files) for every user of a
host, copies each file into one `.tar.gz` archive, and records every file
it met in a manifest with its timestamps, owner and SHA-256 hash. Run it on
the host under investigation, or on your own workstation against a mounted
disk image of that host. Nothing is parsed during a collection: the
analysis commands (`doubleagent timeline` and the others on the
[Analysing page](analyze.md)) read the archive later, on the analyst's
workstation.

What is looked for comes from the catalog, the table of agents and the
paths under a home directory where each keeps its state
(`doubleagent collect --list` prints it). Every catalog line is tried
against every home directory found, so the macOS, Linux and Windows
locations of an agent are all tried on every host, and a Windows disk
image mounted on a Linux or macOS workstation is collected the same way as
a live Windows host. Besides the homes, a run also collects the
project-level files of the working directories the agents' own state names
(such as a repository's `.claude/` directory or `AGENTS.md`), the Docker and
Podman named volumes that belong to known agents, and every startup file
that names an environment variable able to move an agent's files
elsewhere.

Credential files are collected by default and flagged `secret: true` in the
manifest, because the tokens in them show what an agent could reach;
`--no-secrets` leaves them out. Model weights, caches and downloaded
binaries are left out by default but still listed in the manifest with
their size. Processes, network connections, services and other live host
state are left to the EDR (endpoint detection and response) product.

## Getting the program

`doubleagent` is one executable file with no dependencies, built for each
platform. Download the one for the host's operating system and processor
from the project's
[releases page](https://github.com/seanthegeek/doubleagent/releases),
together with the `SHA256SUMS` file published beside it:

| Host | File |
| --- | --- |
| Linux, x86-64 | `doubleagent-linux-amd64` |
| Linux, 64-bit Arm | `doubleagent-linux-arm64` |
| macOS, Intel | `doubleagent-darwin-amd64` |
| macOS, Apple silicon | `doubleagent-darwin-arm64` |
| Windows, x86-64 | `doubleagent-windows-amd64.exe` |
| Windows, 64-bit Arm | `doubleagent-windows-arm64.exe` |

Check the download against `SHA256SUMS` before you put it on a host under
investigation, and make it executable on Linux and macOS:

```sh
sha256sum -c --ignore-missing SHA256SUMS        # Linux
shasum -a 256 -c --ignore-missing SHA256SUMS    # macOS
chmod +x doubleagent-linux-amd64
```

`doubleagent --version` prints the version. [Deployment](deployment.md)
covers uploading the file through an EDR remote shell and checking its hash
there.

## Quick start

Linux and macOS (the file name is the build you downloaded):

```sh
# Live host, every user; root is needed to read other users' homes
sudo ./doubleagent-linux-amd64 collect -o /var/tmp/ir

# A mounted disk image of a Linux, macOS or Windows host
./doubleagent-linux-amd64 collect -r /mnt/evidence -o /cases/host01

# Two users only, without credential files
sudo ./doubleagent-linux-amd64 collect -u alice,bob --no-secrets -o /var/tmp/ir
```

Windows, from a PowerShell or Command Prompt window opened with **Run as
administrator** so that every profile can be read:

```powershell
# Live host, every profile
.\doubleagent-windows-amd64.exe collect -o C:\ir

# A mounted image or an offline volume
.\doubleagent-windows-amd64.exe collect -r E:\ -o C:\cases\host01
```

The options are the same on every platform. Progress lines go to standard
error as the run reaches each user, agent and project (`-q` turns them
off; they always go to the log file), and when the run ends it prints a
summary on standard output:

```text
archive:    /cases/host01/ir-ws01_20261122T050937Z_agent-artifacts.tar.gz
size:       6260 (6.1 KiB)
sha256:     922c3e2a832b0eb642c0398551918c5f702d8075ef8658313522f8f2f79a0acf
manifest:   /cases/host01/ir-ws01_20261122T050937Z_agent-artifacts.manifest.jsonl
summary:    /cases/host01/ir-ws01_20261122T050937Z_agent-artifacts.collection.json
log:        /cases/host01/ir-ws01_20261122T050937Z_agent-artifacts.log
users:      1
projects:   1
collected:  34 files, 1242 bytes (1.2 KiB)
skipped:    5 excluded, 0 too large, 0 secret, 1 special
errors:     0 (0 error_copy, 0 error_read)
relocation: 2 hints in 2 files searched, 1 resolved; see docs/relocation.md
docker:     2 volumes found, 1 collected, 0 unreadable
```

The exit code is `0` whenever an archive was written, even when some files
could not be read; those are rows in the manifest, counted on the `errors`
line. [Options](collect-options.md#exit-codes) lists the other codes.

## What comes out

Every file a run writes goes into the output directory (`-o`, the current
directory by default), named after the host and the start time in UTC:

| File | What it holds |
| --- | --- |
| `<host>_<time>_agent-artifacts.tar.gz` | The archive: every collected file under `fs/` at its original path, and copies of the manifest, `collection.json` and the log. |
| `<host>_<time>_agent-artifacts.tar.gz.sha256` | The archive's SHA-256, in the form `sha256sum -c` reads. |
| `<host>_<time>_agent-artifacts.manifest.jsonl` | One JSON line per file, link or directory the run met: collected, skipped (and why) or unreadable. |
| `<host>_<time>_agent-artifacts.collection.json` | The run: host, mode, options, users, projects, counts, relocation hints and notes. |
| `<host>_<time>_agent-artifacts.log` | The progress lines, each with a UTC time, ending with the archive's size and hash. |

[Output](collection-output.md) describes every field of each.

## Where to go next

| Page | Read it for |
| --- | --- |
| [Options](collect-options.md) | Every option with its default, the size limits, `--list`, exit codes and messages |
| [Output](collection-output.md) | The stdout summary, the archive layout, every manifest field and status, `collection.json`, the log |
| [What gets collected](coverage.md) | The catalog: homes, projects, nested and case-variant entries, exclusions, credential files |
| [Relocation warnings](relocation.md) | What a relocation hint means and how to collect a store an environment variable moved |
| [Docker volumes](docker.md) | Docker and Podman named volumes, and Docker Desktop |
| [Privileges](privileges.md) | What a run without root or Administrator still collects and records |
| [Windows](windows.md) | Links and other reparse points, locked files, long paths, owners and attributes, the registry |
| [Deployment](deployment.md) | Running through an EDR remote shell, console limits, long runs, access times, disk images |
| [Inventory mode](collect-inventory.md) | `--inventory`: one JSON line per user and agent, for a fleet-wide survey |

The evidence behind every catalog line, with citations and the level each
claim rests on, is one document per agent, listed in the
[research index](../research/README.md). [CHANGELOG.md](CHANGELOG.md)
lists what changed in each version, including changes to the manifest.

## License

Copyright 2026 Sean Whalen. Licensed under the Apache License, Version 2.0.
See [LICENSE](../LICENSE).
