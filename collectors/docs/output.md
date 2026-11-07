# Output

What a collection writes: the stdout summary, the files in the output
directory, the archive layout, the manifest and `collection.json`.

## Stdout summary

When the run finishes, the collector prints a summary on stdout (the sample
is in the [README](../README.md#quick-start)):

- `archive`, `manifest`, `summary`, `log`: the paths of the four main output
  files.
- `size` and `sha256` are the archive's; `size` is in bytes, followed by the
  same size in binary units (`B`, `KiB`, `MiB`, `GiB`, `TiB`, one decimal).
- `users` counts the homes that produced at least one manifest row, the
  `users_with_artifacts` list in `collection.json`; homes that were scanned
  and held nothing are left out of the count but stay in `users` there.
- `projects` counts the project directories collected from.
- `collected` counts the manifest rows with status `collected` and their
  bytes, followed by the same total in binary units.
- `skipped` counts the `skipped_excluded`, `skipped_size`,
  `skipped_secret` and `skipped_special` rows, in that order, as
  `N excluded, N too large, N secret, N special` (the `special` count
  since collector 1.13.0).
- `errors` is the number of `error_copy` and `error_read` rows together,
  followed by each of the two counts in parentheses.
- `relocation` is the number of relocation hints and the number of files
  searched for them, as `relocation: N hints in M files searched`, followed
  by `, K skipped for size` when the `--env-scan-home` sweep left K files
  unread for their size, and by `; see collectors/docs/relocation.md` when N
  is above 0. It is printed after `errors` and before `docker`; see
  [relocation.md](relocation.md).
- The `docker:` line appears only when a Docker or Podman volume directory
  or Docker Desktop data was found; see [docker.md](docker.md).

Progress lines (each user, catalog match, project and relocation hint) go
to stderr unless `-q` is given. They always go to the log. A `User` line is
written only for a home with at least one catalog match, just before that
match. A hint line is `relocation hint: <file>:<line> names <VARIABLE> (<agent>)`,
and the first hint of a base in the startup file search, and again in the
`--env-scan-home` sweep, is preceded by the base's header:
`Relocation hints for <user> (<home>)`, `Relocation hints in system files` or
`Relocation hints in project <path>`. The line of the file that matched is
never logged. The first log lines name the options, `env_scan_home=<0|1>`
and `env_scan_max_size=<MB>MB` among them. The last progress line is `done`;
the log file's `done:` line also records the
archive path, size in bytes and sha256.

A project discovery source (a file the collector reads project paths
from, listed in [internals.md](internals.md#project-discovery)) that the
PowerShell collector cannot read gets the line
`discovery source unreadable: <path>: <message>`, with the .NET exception
message, on stderr unless `-q` is given and always in the log; discovery
goes on with the next source and the file itself is still collected or
reported in the manifest like any other. The reader opens the file with
the same shared access as the copy, so a rollout a running agent holds
open is read. The sh collector reads these sources with `grep`, discards
its error messages, and logs nothing for an unreadable source.

## Output files

The output directory receives five files named
`<host>_<UTC time>_agent-artifacts`. In the host name, characters other
than `A-Za-z0-9._-` become `-`:

```text
host_20261003T165531Z_agent-artifacts.tar.gz          the archive
host_20261003T165531Z_agent-artifacts.tar.gz.sha256   its hash, as "<hex>  <file name>" (sha256sum format)
host_20261003T165531Z_agent-artifacts.manifest.jsonl  the manifest (the archive holds a copy)
host_20261003T165531Z_agent-artifacts.collection.json the run summary (the archive holds a copy)
host_20261003T165531Z_agent-artifacts.log             the log (the archive holds a copy)
.stage-host_20261003T165531Z_agent-artifacts/         the staging directory, only with -k
```

The sh collector sets `umask 077`, so the files and directories it creates
(an output directory it has to create included) are readable by the user
who ran it only, mode `600` and `700`. The PowerShell collector sets no
permissions of its own; its files get the output directory's defaults.

## Archiver fallbacks

The sh collector writes the archive with `tar -czf`. If that fails, it pipes
`tar` through `gzip`. The pipe counts as failed when either `tar` or `gzip`
exits non-zero; `tar`'s exit status is passed through a temporary
`.tar-status-<name>` file in the output directory, outside the staging
directory, which is removed before archiving goes on, and a `tar` failure
there is logged as `tar | gzip: tar exited N`. If the pipe fails, or `gzip`
is not on the `PATH`, it writes an uncompressed `.tar` instead.

The PowerShell collector writes a `.tar.gz` with `tar.exe` (the system
`tar` under PowerShell 7 off Windows). If `tar.exe` is missing, exits
non-zero or leaves no archive, it writes the `.tar.gz` with its own
PowerShell tar writer, with the same member names; if that fails too, it
writes a `.zip`. Each failure is logged (`tar.exe failed: exit code N`, hex
for a negative code such as `0xC0000005`) and added to `notes`; see
[windows.md](windows.md).

Both collectors rewrite `collection.json` and its staged copy before each
fallback, so its `archive` and `capabilities.archiver` fields name the
archive that was written. The log is copied into the archive before
archiving starts, so only the copy outside has the final `done` line with
the archive's size and hash.

## Archive layout

Inside the archive (tar members start with `./`, from either collector;
zip entries from the PowerShell collector have no prefix and use `/`):

```text
fs/<original path>          collected files and recreated symlinks, mirroring
                            the source filesystem
manifest.jsonl
collection.json
collector.log
```

On a live Windows host the archive path is `fs/<drive letter>/<path>`, for
example `fs/C/Users/alice/.claude/history.jsonl`. In image mode it is
relative to the root, as on other platforms.

## Manifest

Each manifest row is one JSON object:

```json
{"user":"alice","home":"/home/alice","agent":"claude-code",
 "path":"/home/alice/.claude/projects/-srv-proj/s1.jsonl",
 "archive_path":"fs/home/alice/.claude/projects/-srv-proj/s1.jsonl",
 "type":"file","size":97,"mtime":1791046385,"atime":1791046385,"ctime":1791046385,
 "btime":1791046385,"uid":1000,"gid":1000,"mode":"644","sha256":"...",
 "secret":false,"status":"collected","target":"","error":""}
```

| Field | Meaning |
| --- | --- |
| `user` | Owner of the home the path was found under. For a project file, the user whose agent state referenced the project (empty for a `-p` directory outside every home). `docker` for volume rows. |
| `home` | The base the path was collected relative to: the home directory, the project directory, or a volume's `_data`, including the `-r` root. |
| `agent` | Catalog agent name. For a file found through the project catalog, the agent the `PROJECT_CATALOG` line names: the agent that owns the file (`claude-code` for a project's `.claude/`), or `project` for a file several agents read (`AGENTS.md`, `.mcp.json`, the repository's [git metadata](coverage.md#git-metadata) under `.git/` and the other `project` lines in `--list`); before collector 1.12.0 every project row had `project`. For a volume row, the `DOCKER_VOLUMES` agent of the matched volume. Empty for `skipped_unmatched_volume` and for the `error_read` row of a home, project or `-p` directory. `shared` for a file collected because it names a relocation variable (a startup file, or under `--env-scan-home` any file) and that no catalog line matches; it has `secret: true`, `skipped_secret` under `--no-secrets`, and for a system file an empty `user` and `home`. See [relocation.md](relocation.md). |
| `path` | The original full path, including the `-r` root in image mode. |
| `archive_path` | Where the file is in the archive: `fs/<original path>`. Set for `collected` rows and for `symlink` rows whose link is in the archive: always from the sh collector, and from the PowerShell collector when the link was recreated and the archive is a tar (see [windows.md](windows.md#symlinks-and-reparse-points)). Empty otherwise. |
| `type` | `file`, `symlink`, or `dir` (an excluded directory, an unmatched volume, or an `error_read` directory). A `skipped_special` row names the kind of special file instead: `socket` (a Unix domain socket), `fifo` (a named pipe), `device` (a block or character device) or `other`; see [Special files](#special-files). |
| `size` | Bytes. For a `collected` row from the PowerShell collector, the number of bytes copied, the length of the archived copy that `sha256` covers; for every other file and symlink row, and for every row from the sh collector, the length `lstat` or the directory listing reported before the copy. On Windows that pre-copy length is the file's directory entry, which NTFS updates only when a handle on the file closes, so it can be stale for a file a running process holds open and writes: a `skipped_size` decision and a `skipped_excluded` file row use it as it was. `0` for an `error_read` row. For any other `dir` row, the size of everything under it: `du -sk` × 1024 from the sh collector, the sum of file lengths from the PowerShell collector. |
| `mtime`, `atime`, `ctime`, `btime` | Epoch seconds, read before the copy. `btime` (creation) is `0` where the platform cannot report it, and `ctime` is `0` from the PowerShell collector. All four are `0` for an excluded directory from the sh collector, for an unmatched volume, for an `error_read` row of a `-p` value that does not exist, and on a host with neither GNU nor BSD `stat` (where `uid`, `gid` and `mode` are `0` too). |
| `uid`, `gid`, `mode` | Numeric owner and group, and the permission bits as an octal string (`"644"`), from `lstat`. `0`, `0` and `""` from the PowerShell collector. |
| `owner`, `attributes` | PowerShell collector only: the owning account name (or SID when the name cannot be resolved) and the attribute list, such as `Archive, ReparsePoint`. |
| `sha256` | SHA-256 of the staged copy, set only for `collected` rows. The sh collector leaves it empty when it found no hash tool (`capabilities.hash_tool` is `none`). |
| `secret` | `true` when the path, relative to `home`, matched a credential pattern. |
| `status` | See below. |
| `target` | For a symlink, its target as stored. The PowerShell collector joins several reparse point targets with `;`. |
| `error` | For `error_copy`, why the copy failed: `cp`'s message or the .NET exception. For `error_read`, why the directory could not be read: the first line of `ls`'s message (sh, for example `ls: cannot open directory '/home/carol': Permission denied`) or the .NET exception message. For a PowerShell `symlink` row with no `archive_path`, why the link is not in the archive, starting `not recreated in the archive:` or `not in the archive:`. |

Timestamps come from `lstat` on the original file (sh), or from the item's
times before the copy (PowerShell). The copy of each file is hashed after
staging, so the hash matches the bytes in the archive even if a running
agent appended to the source afterwards. The PowerShell collector reads the
staged copy back through the handle that wrote it, before closing it, and
then sets the copy's timestamps.

The PowerShell collector writes each row itself, with the fields in the
order above. Strings escape `"` and `\`, control characters below U+0020
(`\b`, `\t`, `\n`, `\f` and `\r`, the rest as `\u00xx`) and U+0085, U+2028
and U+2029; everything else is written as UTF-8, and a lone surrogate in a
Windows file name becomes U+FFFD. Up to 1.10.1 it used `ConvertTo-Json`,
which under Windows PowerShell 5.1 also wrote `<`, `>`, `&` and `'` as `\u`
escapes; the parsed values are the same.

### Statuses

| Status | Meaning |
| --- | --- |
| `collected` | Copied, hashed and archived. |
| `symlink` | A symbolic link (on Windows a symbolic link or junction: a reparse point whose `LinkType` is `SymbolicLink` or `Junction`), recorded with its target and never followed. Both collectors recreate the link in the archive with its target as stored. The PowerShell collector cannot where Windows refuses to create it or where the archive is a `.zip`; the row then has no `archive_path` and `error` says why. |
| `skipped_excluded` | Matched an `EXCLUDES` pattern. An excluded directory is one `dir` row with its total size; an excluded file, found in a walk or matched by a catalog entry itself, is one `file` row with its size. An excluded symlink is not a `symlink` row: it gets the same `file` row, or a `dir` row when it points to a directory. |
| `skipped_size` | Larger than `--max-file-size`. |
| `skipped_secret` | A credential file left out by `--no-secrets`. |
| `skipped_special` | A special file: neither a regular file, a directory nor a symbolic link, such as the Unix domain sockets Codex CLI keeps under `.codex`. Never opened or copied; `type` names the kind, `size` and the times are as `lstat` (sh) or `Get-Item` (PowerShell) reported them, `archive_path`, `sha256`, `target` and `error` are empty, and `secret` is set as for any path. See [Special files](#special-files). Since collector 1.13.0. |
| `error_copy` | Could not be read or copied; see `error`. |
| `skipped_unmatched_volume` | A Docker or Podman volume that matched no `DOCKER_VOLUMES` line: one `dir` row with its size, `user` `docker`, an empty `agent`, and `home` and `path` both the volume's `_data`. |
| `error_read` | A directory that could not be listed or entered: a home, a project directory, a `-p` value that is not a directory, or a directory inside a walk (see [privileges.md](privileges.md)). One `dir` row with size `0`, no `archive_path` and the reason in `error`; nothing under it is collected. |

### Check order

A path that was not excluded is checked first for being a symlink, then
for being a special file, then for `--no-secrets`, then for size. A
symlink is therefore never skipped for size, a special file is
`skipped_special` even when it matches a secret pattern under
`--no-secrets` (its `secret` field still says so), and a credential file
is `skipped_secret` under `--no-secrets` whatever its size.

### Special files

A socket, FIFO (named pipe) or device is never opened, because reading
one can block the collector (a FIFO waits for a writer) or read something
that is not a file's contents; it gets one `skipped_special` row. How each
collector names the kind in `type`:

| Collector | How the kind is found | `type` values |
| --- | --- | --- |
| sh | Every path the walk finds that is not a directory goes to the stage worker (`find ! -type d`). After the symlink test, a path that exists and is not a regular file (`test -f`) is tested with `test -S`, `-p`, then `-b` or `-c`. | `socket`, `fifo`, `device`, else `other` |
| PowerShell on Windows | A reparse point whose `LinkType` is neither `SymbolicLink` nor `Junction`. Windows PowerShell 5.1 exposes no reparse tag, so the kind cannot be named; the case seen is the AF_UNIX (Unix domain) socket, reparse tag `IO_REPARSE_TAG_AF_UNIX` (`0x80000023`), which collector 1.12.0 recorded as a `symlink` whose target was its own path. A reparse point whose `LinkType` cannot be read is still recorded as a `symlink`. | `other` |
| PowerShell 7 on Linux and macOS | The first character of the item's `UnixMode` property (PowerShell 7.1 and later): `s`, `p`, `b` or `c`. Without `UnixMode` the collector cannot tell a special file from a regular one and behaves as before 1.13.0: it tries the copy, which fails for a socket (an `error_copy` row) and can wait for a writer on a FIFO. PowerShell 7.6.6 on Linux reports a FIFO's `UnixMode` with a leading `-`, as for a regular file, so the collector copies it and the run waits until something opens the FIFO for writing (seen October 2026). | `socket`, `fifo`, `device` |

Up to collector 1.12.0 the sh walk passed only regular files and symlinks
to the stage worker, so a socket or FIFO under a matched directory had no
manifest row at all. `--inventory` and `-Inventory` leave the special files
they recognise out of the file counts.

## collection.json

`collection.json` records the run:

| Field | Meaning |
| --- | --- |
| `tool`, `version` | `collect-agent-artifacts` and the collector version. |
| `hostname` | The host name, the only record of it in the output. sh: `hostname`, else `uname -n`. PowerShell: `COMPUTERNAME`, else the DNS host name. Always read from the machine running the collector, so in image mode (`-r`) it is the analyst workstation's name, not the imaged host's. |
| `mode` | `live` or `image`. |
| `root` | The `-r` root. In live mode, `/` (sh) or `\` (PowerShell). |
| `platform` | sh: `uname -s`, `-r` and `-m`. PowerShell: `Windows <OS version> <PROCESSOR_ARCHITECTURE> PowerShell <version>`; off Windows the prefix is still `Windows`, the version is .NET's `OSVersion` (the kernel version on Linux) and the architecture is empty when `PROCESSOR_ARCHITECTURE` is unset (`Windows 6.6.114.1  PowerShell 7.6.6` under PowerShell 7 on Linux). |
| `run_as_uid` | sh only: the numeric uid the collector ran as, as a string. |
| `run_as`, `run_as_admin` | PowerShell only: the account the collector ran as, and whether it was elevated. |
| `started`, `finished` | UTC to the second, `2026-10-03T16:55:31Z`. `finished` is taken before archiving. |
| `options` | `full`, `no_secrets`, `max_file_size_bytes` (`0` means no limit), `users_filter` (the `-u` string), `no_docker`, `no_projects` (boolean, `--no-projects` or `-NoProjects`), `projects` (array of the `-p`/`--project` or `-Project` values in the order given, resolved against the current directory to absolute paths and de-duplicated, by both collectors, `[]` when none). A value that is not a directory is still listed here. They are recorded even with `no_projects`, which ignores them. After `projects` come `env_scan_home` (boolean, `--env-scan-home` or `-EnvScanHome`) and `env_scan_max_size_bytes` (`0` means no limit; recorded even without `env_scan_home`). |
| `capabilities` | sh: `hash_tool` (`sha256sum`, `shasum`, `sha256`, `openssl` or `none`), `stat_mode` (`gnu`, `gnu0` for GNU `stat` without birth time, `bsd` or `none`), `worker_shell`, `archiver` (`tar -z` for `tar -czf`, `tar \| gzip` for the pipe, `tar` for the uncompressed `.tar`) and `grep_text` (`true` when `grep -a` reads a file holding a NUL byte as text, so a relocation variable named in such a file is found; with `false` the relocation search finds no hint in a file `grep` reports as binary). PowerShell, which reads every file as text and has no `grep_text`: `hash_tool` (`System.Security.Cryptography.SHA256`; `Get-FileHash` up to 1.10.1) and `archiver` (`tar.exe` for `tar.exe` or the system `tar`, `PowerShell tar writer` for the script's own tar writer, added after 1.10.2, or `ZipFile` for the zip). `archiver` names the one that wrote the archive. |
| `users`, `homes` | Parallel lists of every user and home scanned, including homes with no agent state. |
| `users_with_artifacts` | The users whose home produced at least one manifest row, in the order of `users`. Rows from discovered projects and Docker volumes do not count. The stdout `users:` line is its length. |
| `projects` | The project directories collected from, discovered or given with `-p`. A `-p` value that is not a directory is left out and has an `error_read` row instead. |
| `counts` | Rows per status (`collected`, `symlink`, `skipped_excluded`, `skipped_size`, `skipped_secret`, `skipped_special`, `error_copy`, `skipped_unmatched_volume`, `error_read`) and `collected_bytes`. `skipped_special` since collector 1.13.0. |
| `docker` | `volumes_found`, `volumes_collected`, `unreadable` and `docker_desktop`; see [docker.md](docker.md). |
| `relocation` | `hints`, `files_searched` and `files_skipped_size`; see [relocation.md](relocation.md). `hints` is an array of objects with `user` (the owner of the home or the project's user, empty for a system file), `source` (the path of the file that names the variable), `line` (its line number), `variable`, `agent` (the catalog agent the table line belongs to, or `shared`) and `value` (always `""`; the collector never evaluates a value). A variable that two agents list gives one hint per agent. `files_searched` counts the files handed to the search, the startup files and, with `--env-scan-home`, the files of the sweep, which leaves out the startup files already searched, so each file counts once; `files_skipped_size` counts the sweep's files over `--env-scan-max-size`. |
| `notes` | Messages about what could not be collected, such as an unreadable or symlinked Docker volume directory, Docker Desktop data, or (PowerShell) symlinks that are not in the archive and why, and (PowerShell) each archiver that failed and the one that was used instead, such as `tar.exe failed (exit code 0xC0000005); fell back to PowerShell tar writer`. |
| `archive` | The file name of the archive written: `.tar.gz`, `.tar` after the sh collector's uncompressed fallback, or `.zip` from the PowerShell collector after both `tar.exe` and its PowerShell tar writer failed. |
