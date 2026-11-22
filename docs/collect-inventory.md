# Inventory mode

`doubleagent collect --inventory` answers a fleet question, which users
have used which agents on which hosts, without collecting anything: it
walks what a collection would take and, instead of copying each file,
counts it, then prints one JSON line for the host and one per user and
agent found. Run it on every host through the EDR console and keep what it
prints: standard output is the whole result
([Fleet inventory](deployment.md#fleet-inventory)), and `doubleagent fleet`
merges the saved outputs of many hosts into one CSV
([Fleet inventory](fleet.md)).

Nothing is created, copied, hashed or written on the host, not even a log
file. `-o` is ignored; when it is given, the line
`NOTE: --inventory writes nothing; -o DIR ignored` goes to standard error.
`-k`, `--no-secrets`, `--max-file-size`, `--env-scan-home` and
`--env-scan-max-size` have no effect, and no relocation hint is searched
for.

## What is walked

The walk is the collection walk with the copy step removed, and it honours
`-r`, `-u`, `-p`, `--full`, `--no-docker` and `--no-projects` as a
collection does. Homes are found as for a collection. Every catalog line is
tried against every home, [nested entries](coverage.md#nested-entries)
claim their subtrees from the entry that encloses them, and the default
exclusions prune their subtrees (not with `--full`), so the counts cover
what a collection would take. Regular files are counted with their size and
modification time from the file itself; symlinks, junctions and special
files are neither followed nor counted. Docker and Podman volumes are found
as in [Docker volumes](docker.md), and a volume whose name matches an agent
is walked like a home. The homes are walked last, after the volumes and
project discovery, so the progress lines come in that order.

Project discovery runs, so the number of projects is known, but the
project-level files are not walked. Discovery sources that are credential
files (such as Claude Code's `.claude.json`) are never opened in this mode,
so the project count can be lower than the number of projects a collection
takes from. The other discovery sources are read as in a collection, which
may update their access time ([Access times](deployment.md#access-times)).

## Output

Standard output carries JSON Lines and nothing else: first one `host` line,
then one `agent` line per user and agent, users in the order their homes
were found and each user's agents in the order of their first catalog
match, then the lines of the Docker and Podman volumes under the user
`docker`. Progress lines go to standard error, without times, unless `-q`
is given. The exit code is `0` once the walk has run, also when homes or
Docker data were unreadable. Over a mounted image:

```jsonl
{"type":"host","host":"ir-ws01","collector":"<version>","mode":"image","at":"2026-11-22T05:09:59Z","users_scanned":1,"users_unreadable":0,"docker_volumes":2}
{"type":"agent","host":"ir-ws01","user":"alice","agent":"claude-code","files":19,"bytes":822,"first":"2026-10-01T00:00:00Z","last":"2026-10-01T00:00:00Z","projects":1,"evidence":".claude,.claude.json*"}
{"type":"agent","host":"ir-ws01","user":"alice","agent":"codex-cli","files":2,"bytes":43,"first":"2026-10-01T00:00:00Z","last":"2026-10-01T00:00:00Z","projects":1,"evidence":".codex"}
{"type":"agent","host":"ir-ws01","user":"docker","agent":"agent-zero","files":9,"bytes":210,"first":"2026-10-01T00:00:00Z","last":"2026-10-01T00:00:00Z","projects":0,"evidence":"*a0_usr"}
```

### Host line

One per run, with its keys in this order:

| Field | Type | Meaning |
| --- | --- | --- |
| `type` | string | `host`. |
| `host` | string | The host name, as in `collection.json` `hostname`: the name of the machine running the command, so in image mode the analyst workstation's. |
| `collector` | string | The version of `doubleagent`, as `--version` prints it. |
| `mode` | string | `live`, or `image` with `-r`. |
| `at` | string | The start time, UTC to the second, `2026-11-22T05:09:59Z`. |
| `users_scanned` | number | The homes walked, after `-u`. On Windows a profile logged as `profile not accessible (skipped)` is not counted. |
| `users_unreadable` | number | Of those, the homes that cannot be both listed and entered; their catalog lines that need a listing match nothing. Each is also logged to standard error as `WARNING: home of USER is not readable: HOME`. As root, or as Administrator, this is usually `0`; without, other users' homes are counted here. A directory inside a readable home that cannot be listed is not counted anywhere: its files are missing from the totals. |
| `docker_volumes` | number | The volume directories found, matched or not, as `collection.json` `docker.volumes_found`. `0` with `--no-docker`. Without root the root-owned volume directories cannot be read, so their volumes are not found; a `NOTE: docker:` line on standard error says so. |

### Agent lines

One per user and agent with at least one regular file under its matched
catalog paths, after exclusions and nested claims, with its keys in this
order. A match that holds no file of its own gives no line: an empty
directory, a symlink, an excluded-only tree, or a `~/.gemini` that holds
only the `antigravity-cli` directory claimed by the nested `antigravity`
entry. The same holds for Docker volumes, so `files` is never `0`. The
`shared` catalog lines (directories several agents read) are not
inventoried, and the agent `project` never appears.

| Field | Type | Meaning |
| --- | --- | --- |
| `type` | string | `agent`. |
| `host` | string | As on the host line. |
| `user` | string | The home's user, as in the manifest, or `docker` for a matched Docker or Podman volume. |
| `agent` | string | The catalog agent name; for a volume, the agent its name matched. |
| `files` | number | The regular files under the agent's matched paths after exclusions, with nested entries' subtrees left to their own agent. |
| `bytes` | number | Their total size in bytes. |
| `first`, `last` | string | The earliest and latest modification time of those files, UTC to the second; `""` when no file had a time after 1970. |
| `projects` | number | The project directories found for this user, the same number on each of the user's lines, since a project is not tied to one agent. A project named by several users counts for the first of them in the order their homes were found, as in a collection, and a `-p` directory counts for the user whose home holds it. A project that leads out of the image root is not counted. `0` for `docker` lines and with `--no-projects`. |
| `evidence` | string | The catalog patterns (the part after `agent\|` in `--list`) that matched, in catalog order, joined by commas, so no path below the home is printed. For a volume, the volume name patterns that matched. |

### Escaping

Each line is one JSON object written by Go's `encoding/json`: `"`, `\` and
control characters are escaped, so are U+2028 and U+2029, and `<`, `>` and
`&` are written as they are. A user name that is not valid
UTF-8 is written with U+FFFD in place of each invalid byte.
