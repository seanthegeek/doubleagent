# Fleet inventory

`doubleagent fleet` turns a fleet-wide audit into one spreadsheet. Run
`doubleagent collect --inventory` on each host, which writes nothing on the
host and prints one JSON line per user and agent it finds
([Inventory mode](collect-inventory.md)); save each host's stdout to a
file, for example from the EDR console; then merge the files on the
analyst's workstation:

```sh
doubleagent fleet -o /cases/fleet/fleet-inventory.csv /cases/fleet/inventory
```

```text
doubleagent fleet [-o CSV] INPUT [INPUT ...]
```

## Inputs

Each `INPUT` is a file holding one host's saved stdout, or a directory
whose regular files are all read: not recursively, in byte order of their
names, symbolic links and other entries left out. An `INPUT` that is
itself a symbolic link is refused (`<path> is a symlink; give its
target`), and so is one that is neither a file nor a directory. An input
that cannot be read stops the command with exit code `2` and
`error: <path>: <reason>`.

Each file is read whole, as UTF-8: a byte that is not valid UTF-8 becomes
U+FFFD, and a byte order mark at the start of the file or of a line is
dropped. A line ends at LF, at CR LF, or at a lone CR, so a progress line
a console ended with CR does not hide the record after it.

- A line holding a JSON object whose `type` is `host` or `agent`, with
  white space around it allowed, is an inventory record.
- Any other line that is not blank, such as a console banner, a prompt or
  a record cut off mid-line, is skipped and counted, and the count is
  printed on stderr as `skipped <n> non-inventory lines in <file>`
  (`line` when there is one). Blank lines are skipped without being
  counted.
- A `host` line opens a run, and each later `agent` line in the same file
  is a row of that run until the next `host` line, so one file may hold
  several runs.
- An `agent` line before any `host` line gives a row with its own `host`
  and empty `collector`, `mode` and `at`.
- A `host` line with no `agent` line after it gives one row with the four
  host columns filled and the eight agent columns empty, so a host where
  nothing was found is still in the sheet.

Rows are written in input order; nothing is merged or deduplicated, so two
captures of one host give two sets of rows.

## The CSV

The CSV is `fleet-inventory.csv` in the current directory unless `-o`
names another path; missing parent directories are created. It is UTF-8
without a byte order mark, with a header row, fields quoted where they
need it and lines ending in LF. Its columns, in order:

| Column | Meaning |
| --- | --- |
| `host` | The host name from the run's `host` line, or from the `agent` line when no `host` line came before it. |
| `collector` | The version of `doubleagent` that made the inventory, from the `host` line. |
| `mode` | `live`, or `image` for a run against a mounted image, from the `host` line. |
| `at` | The run's start time, from the `host` line, converted to UTC with milliseconds: `2026-10-04T16:31:54.000Z`. |
| `user` | The home's user, or `docker` for a Docker or Podman volume. |
| `agent` | The catalog agent name. |
| `files` | Regular files under the agent's matched paths, after exclusions. |
| `bytes` | Their total size in bytes. |
| `first` | The earliest modification time of those files, converted like `at`. Empty when the line gives none. |
| `last` | The latest modification time, converted like `at`. Empty when the line gives none. |
| `projects` | The user's discovered project count, the same on each of the user's rows: a collection run does not record which agent's files named a project. |
| `evidence` | The catalog globs that matched, comma-separated, as the `agent` line gives them. |

A number is written as the line holds it. A field the line lacks, or holds
as `null`, is left empty. The `host` line's `users_scanned`,
`users_unreadable` and `docker_volumes` are not carried into the CSV.

A text field that begins with `=`, `+`, `-`, `@`, a tab or a carriage
return is written with a `'` in front, so a spreadsheet shows it as text
and runs nothing: the inventory comes from the host under investigation,
which may be hostile. The summary on stdout shows such text as written.

The columns are an interface like the timeline's: new ones are added at
the end, and existing ones keep their names, order and meaning.

### Where the CSV may go

The CSV is refused, before any input is read, when it is a symbolic link,
when it is one of the input files, or when it lies inside an input
directory, and when it cannot be told whether it does (as through a
junction or link on Windows). The check follows where the system puts the
file: on Linux and macOS a `..` after a link leads up from the link's
target, and on Windows a `..` cancels the name written before it.

The CSV is written to a new file beside it, named
`doubleagent-fleet-<random>.partial`, which is then moved into its place,
so an existing CSV is replaced whole, never written into, and its
directory must be writable. A `.partial` file left behind by a run killed
while writing can be deleted. When the CSV cannot be written, the
directories made for it are removed again while empty, unless the
directory part of the name holds a `.` or `..` element.

## Stdout

`fleet` prints the number of rows and distinct hosts and the CSV's path,
then, when any row has an agent, one line per agent: `hosts` counts the
distinct hosts with a row for it, `users` the distinct host and user
pairs (`docker` included), and `latest` is the greatest `last` time. The
lines are sorted by `hosts`, then `users`, both descending, then by agent
name. Over a directory of two saved outputs, `ws01.jsonl` with one run
and `ws02.jsonl` with three runs (of `ws02`, `ws03` and `ws04`), a console
login banner and a record cut off mid-line:

```text
skipped 2 non-inventory lines in /cases/fleet/inventory/ws02.jsonl
rows:   9 from 4 hosts in /cases/fleet/fleet-inventory.csv

agent                   hosts  users  latest
claude-code                 3      4  2026-10-05T09:00:00.000Z
codex-cli                   2      2  2026-10-04T11:45:00.000Z
agent-zero                  1      1  2026-10-03T10:00:00.000Z
cursor                      1      1  2026-10-04T09:30:00.000Z
```

The `skipped` line goes to stderr, the rest to stdout. The CSV of that
run:

```text
host,collector,mode,at,user,agent,files,bytes,first,last,projects,evidence
ws01,1.22.0,live,2026-10-04T16:31:54.000Z,alice,claude-code,6,318,2026-09-15T01:02:03.000Z,2026-10-04T16:31:11.000Z,29,".claude,.claude.json*"
ws01,1.22.0,live,2026-10-04T16:31:54.000Z,alice,codex-cli,1,35,2026-10-01T00:00:00.000Z,2026-10-01T00:00:00.000Z,29,.codex
ws01,1.22.0,live,2026-10-04T16:31:54.000Z,bob,claude-code,0,0,,,0,.claude
ws01,1.22.0,live,2026-10-04T16:31:54.000Z,docker,agent-zero,2,63,2026-10-02T10:00:00.000Z,2026-10-03T10:00:00.000Z,0,*a0_usr
ws02,1.22.0,image,2026-10-05T06:00:00.000Z,carol,cursor,11,100667,2026-09-20T12:00:00.000Z,2026-10-04T09:30:00.000Z,4,".cursor,.config/Cursor"
ws02,1.22.0,image,2026-10-05T06:00:00.000Z,carol,claude-code,19,34293,2026-09-01T08:00:00.000Z,2026-10-03T21:15:00.000Z,4,".claude,.claude.json*"
ws02,1.22.0,image,2026-10-05T06:00:00.000Z,dave,codex-cli,8,59166,2026-09-28T07:00:00.000Z,2026-10-04T11:45:00.000Z,2,.codex
ws03,1.22.0,live,2026-10-05T09:10:00.000Z,,,,,,,,
ws04,1.21.0,live,2026-10-05T09:20:00.000Z,erin,claude-code,3,900,2026-10-01T00:00:00.000Z,2026-10-05T09:00:00.000Z,1,.claude
```

`ws02`'s `host` line gives its start as `2026-10-05T08:00:00+02:00`, which
the `at` column converts to UTC; `ws03` found nothing.

## Exit codes

| Exit code | Meaning |
| --- | --- |
| `0` | At least one row was written. |
| `1` | No `host` or `agent` line was found; the CSV is written with its header only. |
| `2` | An input or the CSV was refused or could not be read or written, or the command line is invalid. |
| `130` | Interrupted; the run stops before its next input file and writes no CSV. |
