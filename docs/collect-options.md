# Collection options

`doubleagent collect [options]` takes every option on this page, the same
on Linux, macOS and Windows. A run with no options collects every user of
the live host into the current directory. `doubleagent collect --help`
prints the short form of the table below.

## Option reference

| Option | Default | Meaning |
| --- | --- | --- |
| `-o`, `--output DIR` | the current directory | Where the archive, its `.sha256` file, the manifest, `collection.json` and the log are written. Created, with any missing parents, when it does not exist. Ignored under `--inventory`, which writes nothing. On an output directory that treats two different names as one (see [Case and Unicode form in the output directory](#case-and-unicode-form-in-the-output-directory)), one of two such files is not staged and its row says why. |
| `-r`, `--root DIR` | none: live mode | The root of a mounted disk image or offline volume, which switches the run to image mode: homes, projects, Docker volumes and startup files are looked for under `DIR`, and every path in the output keeps `DIR` in front. A relative `DIR` is resolved against the current directory, and it must be a directory (a link to one is followed). In image mode nothing behind a link whose target leads out of `DIR` is listed, read or recorded, in any step; the log names each such link once. On Linux and macOS `-r /`, and any spelling that comes to `/` (`-r /tmp/..`), is live mode, and so is `-r ""`. On Windows every `-r` is image mode, a drive root such as `-r C:\` included. |
| `-u`, `--users LIST` | every home found | Comma-separated user names; only their homes are collected. A name is compared with the user name from the password database, or with the directory name for a home found under a home parent such as `/home` or `C:\Users`. Linux and macOS compare names exactly; Windows ignores letter case. The list also limits the per-user Docker and Podman volume directories and the projects found through the users' own agent state. A name that matches no home is logged as `WARNING: -u NAME matches no user home; homes found for: A, B` (or `none`), and a list naming nobody (`-u ,`) as `WARNING: -u lists no names`. |
| `-p`, `--project DIR` | none | An extra project directory whose project-level files are collected, as for a project found in an agent's state. Repeat the option for each directory. `DIR` is a path on the machine running the command and is never put under the root, so in image mode give it with the mount point (`-p /mnt/evidence/srv/app`). A relative path is resolved against the current directory, and a directory given twice is collected once. In image mode a path outside the root is logged `Project DIR: is outside the image root; not walked` and gets no row. A value that is not a directory is logged `WARNING: project given with -p is not a directory: DIR: <reason>` and gets one `error_read` row. Every value is recorded in `collection.json` `options.projects`; only the directories collected from are in `projects`. Ignored with `--no-projects`. |
| `--full` | off | Collect what the default exclusions leave out: model weights, caches, extension and daemon binaries, marketplace clones and the other subtrees under `# excluded unless --full` in `--list`. |
| `--no-secrets` | off | Leave credential files out. Each gets a `skipped_secret` row with its size, instead of being collected and flagged `secret: true`. Startup files collected for a relocation hint are credential files too. |
| `--no-projects` | off | Skip project discovery and the `-p` directories. |
| `--no-docker` | off | Skip the Docker and Podman named volumes and the Docker Desktop check ([Docker volumes](docker.md)). |
| `--max-file-size MB` | `256` | Skip a file larger than this many MiB (1 MiB is 1,048,576 bytes) with a `skipped_size` row. A whole number of MiB, digits only; `0` means no limit. A symlink is never skipped for its size. |
| `--env-scan-home` | off | After the startup files, also search every other regular file of each home and project for the names of the relocation variables ([Search the whole home](relocation.md#search-the-whole-home)). Slow on a large home. It only reads; a file that names a variable is collected like a startup file. |
| `--env-scan-max-size MB` | `1` | With `--env-scan-home`, leave a file larger than this many MiB unsearched and count it in `collection.json` `relocation.files_skipped_size`. A whole number of MiB; `0` means no limit. Recorded even without `--env-scan-home`. |
| `--no-preserve-atime` | off | Open every file the run reads for reading alone. On Windows a run by default opens each file with the right to change its attributes as well, so it can ask Windows not to update the file's last-access time for the run's own read; some security software watches for that right. With this option a read may change a file's access time on disk. The manifest still records each collected file's access time from before the read. It changes nothing on Linux and macOS, where files are always opened for reading alone. See [Access times](deployment.md#access-times). |
| `-k`, `--keep-staging` | off | Keep the staging directory, `.stage-<host>_<time>_agent-artifacts` in the output directory, which holds the archive's contents unpacked. On Windows its `fs` directory is marked case-sensitive, so it can hold two names that differ only in case; open it with tools that respect that setting. |
| `--inventory` | off | Write nothing: walk the catalog, count what a collection would take and print one JSON line for the host and one per user and agent to standard output ([Inventory mode](collect-inventory.md)). |
| `--list` | | Print the catalog tables and exit. |
| `-q`, `--quiet` | off | Print only the final summary. The progress lines still go to the log file. |
| `-V`, `--version` | | Print `doubleagent <version>` and exit. |
| `-h`, `--help` | | Print the usage text and exit. |

Sizes are given and recorded in whole MiB on the command line and in the
log (`max_file_size=256MB`), and in bytes in `collection.json`
(`max_file_size_bytes: 268435456`).

## How the command line is read

Options may be written with one dash or two (`-output DIR`, `--output DIR`),
with the value as the next argument or after `=` (`--output=DIR`, `-o=DIR`).
A switch takes no separate value, but accepts `=true` or `=false`
(`--quiet=false`). `--` ends the options. An option given twice takes its
last value, except `-p`, which adds a directory each time. `collect` takes
no argument that is not an option.

A mistake anywhere on the line stops the run before it reads anything, even
when the line also holds `--help`, `--version` or `--list`. Each mistake is
printed on standard error as one line starting `doubleagent collect: `,
followed by the usage text, and exits `1`:

```text
doubleagent collect: flag provided but not defined: -bogus
doubleagent collect: flag needs an argument: -o
doubleagent collect: unexpected argument: stray
```

A size that is not a whole number of MiB is one line without the usage
text: `Invalid --max-file-size: 1.5` or `Invalid --env-scan-max-size: x`.
On a line without a mistake, `--help` wins over `--version`, and
`--version` over `--list`.

## Exit codes

| Code | Meaning |
| --- | --- |
| `0` | An archive was written, even if some files could not be copied (`error_copy` rows) or some directories could not be read (`error_read` rows). With `--inventory`, the walk ran, even if homes or Docker data were unreadable. Also `--list`, `--version` and `--help`. |
| `1` | A command-line mistake, or a size value that is not a whole number. |
| `2` | The run could not start or could not finish: the root is not a directory (`Root is not a directory: DIR`), the output directory cannot be created (`Cannot create output dir: DIR`), entered (`cd: can't cd to DIR`) or written (`Output dir not writable: DIR`), the staging directory cannot be created (`Cannot create staging dir`), or the archive could not be written (the log ends `FATAL: archive not written`). A failure after the log was opened is logged as a `FATAL:` line. Under `--inventory` only the root is checked. |
| `130` | Interrupted: `INT` or `TERM` on Linux and macOS, Ctrl+C on Windows. The log gets `interrupted`, the staging directory is removed unless `-k` was given, and no archive is left. A program the run had started at that moment (`getent`, `dscl` or `plutil`) is stopped, and none is started after it. |

A message for a run that could not start goes to standard error, before
any log file exists.

## Case and Unicode form in the output directory

Some file systems take two different names for one: names that differ in
letter case on the default macOS volume, on exFAT and FAT drives and on a
Windows volume whose staging directory cannot be marked case-sensitive;
names that differ only in Unicode form (the same accented letter stored as
one character, or as a letter and a combining mark) on every macOS volume;
and, on a Windows volume that makes 8.3 short names, a name shaped like one
(`CLAUDE~1`). When the evidence holds two such names (from a Linux image,
or a case-sensitive directory), the output directory can hold only one of
them. The run never writes one over the other: the second file gets an
`error_copy` row, with no archive path and the error
`the output directory takes its name for another file's, link's or directory's; not staged`,
and a link gets its `symlink` row with that reason after
`not recreated in the archive:`. When the run finds that its output
directory merges names, the log's start lines include
`NOTE: output directory DIR ignores letter case or Unicode form in names; ...`.

To keep both, write the output to a file system that compares names byte
for byte, such as Linux ext4 or XFS; for names differing only in case, a
case-sensitive volume is enough. On Windows the run marks its staging
directory case-sensitive where the file system allows it.

## `--list`

`--list` prints the catalog bundled into the program, the same text as
`doubleagent catalog`, as tables, each under a heading line:

| Heading | Lines |
| --- | --- |
| `# home artifacts (agent\|glob relative to home)` | Where each agent keeps its state under a home. |
| `# project artifacts (relative to each discovered project)` | The project-level files collected from each project. |
| `# excluded unless --full` | The subtrees left out by default, as patterns relative to the home, project or volume. |
| `# credential files (flagged secret, skipped with --no-secrets)` | The patterns that mark a file `secret: true`. |
| `# docker volumes (agent\|volume name glob)` | The Docker and Podman volume names that belong to an agent. |
| `# relocation variables (agent\|VARIABLE\|home-relative path it replaces)` | The environment variables that move an agent's files. |
| `# startup files searched for them (scope\|glob; scope home, system or project)` | The files searched for those variable names. |

Lines starting with `#` inside a table are comments naming the agent or the
source of a line.

## Environment

`doubleagent collect` reads no settings from the environment. To list the
users of a live Linux or macOS host it runs `getent passwd` (reading
`/etc/passwd` when `getent` fails) and, on macOS, `dscl`; in image mode it
reads the image's `etc/passwd` instead. It reads a launchd property list's
values through `plutil` where macOS provides it, live or in image mode. It
looks for those programs in `/usr/bin`, `/bin`, `/usr/sbin`, `/sbin` and
`/usr/local/bin` first, then along `PATH` (skipping a relative entry), and
runs them with `LC_ALL=C`. On Windows it runs none of them. Copying,
hashing, archiving and searching are done by the program itself.

On Windows it reads `COMPUTERNAME` for the host name (falling back to the
DNS host name), `PROCESSOR_ARCHITECTURE` for `collection.json` `platform`,
and, in a live run, `SystemDrive` for the drive whose `Users` directory
holds the profiles (default `C:`) and `ProgramData` for the Docker and
Docker Desktop data.
