# Docker volumes

Several agents are commonly run in containers with their state in a Docker
or Podman named volume rather than in a home directory: Agent Zero's README
runs it with `-v a0_usr:/a0/usr`, Local Deep Research's compose file keeps
its data in `ldr_data`, and Ollama's Docker instructions use a volume named
`ollama`. A walk of the home directories never sees that state, so a
collection run also looks for named volumes on the host, collects the ones
whose names belong to an agent as if each were a home, records the ones it
does not recognise, and says so when Docker Desktop keeps its volumes inside
a virtual machine disk the run cannot reach. `--no-docker` turns the whole
step off.

The volume name patterns are in the table below, and `--list` prints them
under `# docker volumes (agent|volume name glob)`. When several match one
volume name, the pattern with the fewest `*` wins, then the one with the most
literal characters (a `?` or a `[...]` list counts as neither), then the one
whose agent, and then pattern, sorts first, wherever the lines sit in the
catalog file.
So `local-deep-research_ollama_data` goes to `ollama` by `*ollama_data`
rather than to `local-deep-research` by `*local-deep-research*`, and a name
holding both `tabby` and `letta` goes to `letta`, whose `*letta*` is as
specific as `*tabby*` and whose name sorts first:

| Pattern | Agent | Source of the name | Mounted at |
| --- | --- | --- | --- |
| `*a0_usr`, `a0-launcher-*-usr` | `agent-zero` | README `docker run`; A0 Launcher named-volume mode | `/a0/usr` |
| `*openclaw_home` | `claude-code` | OpenClaw's `scripts/docker/setup.sh` and Docker guide (`OPENCLAW_HOME_VOLUME`), which keep a Claude Code install in it | `/home/node`, the container user's home |
| `*dify_agent_local_sandbox_home`, `*dify_agent_local_sandbox_workspace` | `dify` | `docker/docker-compose.yaml` (Compose names them `docker_...`) | `/home/dify`, `/workspace` |
| `*langflow-data` | `langflow` | `docker_example/docker-compose.yml` | `/app/langflow`, the config directory |
| `*ldr_data` | `local-deep-research` | `docker-compose.yml` | `/data` |
| `*n8n-data`, `*n8n_data` | `n8n` | README `docker run`; the `get-n8n.sh` compose file (`n8n_n8n-data`) | `/home/node/.n8n` |
| `*ollama*`, `*ollama_data`, `ollama` | `ollama` | `docs/docker.mdx`; the Local Deep Research compose file (`ollama_data`) | `/root/.ollama` |
| `*hermes*`, `*letta*`, `*local-deep-research*`, `*nanobot*`, `*openhands*`, `*tabby*` | as named | inferred: these tools' documentation bind-mounts the home directory, and a named volume replacing it has the same layout | the tool's state directory |

Dify keeps most of its state, including its PostgreSQL database, in bind
mounts beside its compose file rather than in named volumes, and Langflow's
`langflow-postgres` and LangGraph's `<project>_langgraph-data` volumes are
whole PostgreSQL clusters; none of these is matched
([Agent frameworks and self-hosted platforms](coverage.md#agent-frameworks-and-self-hosted-platforms)).

## Where volumes are found

Volumes are found by reading the file system, never through the `docker`
or `podman` command, so the same step works on a live host and on a disk
image. The volume directories tried are, under the root (`/` live, `-r` in
image mode):

| Directory | Holds the volumes of |
| --- | --- |
| `var/lib/docker/volumes` | Docker run as root |
| `var/lib/containers/storage/volumes` | Podman run as root |
| `ProgramData/Docker/volumes` | Docker for Windows containers (on a live Windows host, under the `ProgramData` directory the environment names) |
| `.local/share/docker/volumes` under each home | Rootless Docker |
| `.local/share/containers/storage/volumes` under each home | Rootless Podman |

`-u` limits the per-home directories to the users named; the system
directories are always tried. The log names each volume directory read, as
`Docker volumes in DIR`.

A volume directory that is a symlink, or has a symlink on the way to it, is
not followed: `collection.json` `notes` gets
`docker: DIR is a symlink and was not followed; collect its target by hand`.
In image mode a link above those directories whose target leads out of the
image root (an image whose `var`, or a home's `.local`, is such a link) is
not looked behind either: the log says
`docker: LINK leads out of the image root; nothing behind it was listed`
once per link, and no note is written.

## Matching and collection

Each volume is a directory `<volume directory>/<name>/_data`; an entry that
is a symlink or not a directory is not a volume and is passed over without
being counted. The volume's name is matched against the patterns above:
letter case counts on Linux and macOS and is ignored on Windows. A matched
volume is collected whole, logged as `  [AGENT] DIR/_data`: its rows have
`user` `docker`, `home` the volume's `_data` directory and `agent` from the
table, and its files are archived under `fs/<original path>` like any
other. Exclusion and credential patterns apply relative to `_data`. The
patterns written for a volume's root, such as `tmp/playwright` and
`secrets.env`, start with `volume|` in the catalog and are tried only
below a volume's `_data`, never under a home or a project; every other
pattern is tried below every home, project and volume.

A volume that matches nothing is not collected. It gets one `dir` row with
the status `skipped_unmatched_volume`, an empty `agent` and its size, so a
database volume or an agent with an unexpected volume name is still
visible.

A volume whose `_data` is missing, or is a symlink (on Windows also a
junction), is counted in `volumes_found`, logged as
`  volume NAME has no _data directory, skipped`, and gets no row. The link
is judged from the link itself, before anything is read through it, so the
outcome never depends on what it points to.

## Unreadable volumes

`/var/lib/docker` is readable only by root; run as root to collect the
system volumes. When a volume directory exists but
cannot be read, a volume cannot be read, or a matched volume cannot be
walked completely, the run carries on with everything else and records it:

| `notes` entry | When |
| --- | --- |
| `docker: DIR is not readable; run as root to collect the volumes under it` | A volume directory, or a directory on the way to it, cannot be entered or listed. |
| `docker: volume NAME (AGENT) at DIR is not readable; run as root to collect it` | A volume, or its `_data`, cannot be listed and entered; `(unmatched)` in place of the agent for a volume whose name matches no pattern. |
| `docker: volume NAME (AGENT) could not be walked completely (N unreadable paths, see collector.log); run as root to collect it` | A matched volume was collected, but some of its directories could not be read; each has an `error_read` row. |

Each is also logged with `NOTE: ` in front, counted in `collection.json`
`docker.unreadable`, and the stdout summary says so:

```text
docker:     3 volumes found, 1 collected, 1 unreadable (run as root to collect)
```

The exit code is still `0` when an archive was written.

## Docker Desktop

Docker Desktop on Windows and macOS keeps Linux volumes inside its virtual
machine disk (the WSL `docker_data.vhdx` on Windows, `Docker.raw` on
macOS), which a file system walk cannot reach. When Docker Desktop data is
found (a `DockerDesktop` directory under `ProgramData`, or a home's
`AppData/Local/Docker` or `Library/Containers/com.docker.docker`), the run
only records the note
`docker: Docker Desktop data at DIR; volumes inside its virtual machine disk are not collected`,
sets `docker.docker_desktop` to `true`, and appends
`; Docker Desktop VM disk not collected` to the summary's `docker:` line.
Collect those volumes from inside the virtual machine, for example with
`docker run --rm -v VOLUME:/v -v "$PWD":/out alpine tar -czf /out/VOLUME.tgz -C /v .`.

## With `--inventory`

[Inventory mode](collect-inventory.md) finds volumes the same way and counts
the files of each matched volume on agent lines of user `docker`. An
unmatched volume gives no line, and the host line's `docker_volumes` is the
number of volumes found, matched or not.
