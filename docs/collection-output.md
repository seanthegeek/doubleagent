# Collection output

A collection run writes five files into its output directory: the
archive, a file holding the archive's hash, the manifest, the run summary
`collection.json` and the log. It also prints a summary on standard
output. This page describes each of them, field by field, so you can tell
from the files alone what a run collected, what it left out and why, and
what it could not read.

## Stdout summary

When the run ends it prints one `name: value` line each (the example is in
the [quick start](collect.md#quick-start)):

| Line | Value |
| --- | --- |
| `archive:`, `manifest:`, `summary:`, `log:` | The full paths of the archive, the manifest, `collection.json` and the log. |
| `size:` | The archive's size in bytes, then the same size in binary units (`B`, `KiB`, `MiB`, `GiB`, `TiB`, one decimal) in parentheses. |
| `sha256:` | The archive's SHA-256, as in its `.sha256` file. |
| `users:` | The number of homes that produced at least one manifest row, the length of `collection.json` `users_with_artifacts`. A home that was searched and held nothing is not counted, but is listed in `users`. |
| `projects:` | The number of project directories collected from, the length of `collection.json` `projects`. |
| `collected:` | The `collected` rows and their bytes, `N files, N bytes`, then the bytes in binary units. |
| `skipped:` | The `skipped_excluded`, `skipped_size`, `skipped_secret` and `skipped_special` rows, as `N excluded, N too large, N secret, N special`. |
| `errors:` | The `error_copy` and `error_read` rows together, then each count in parentheses. |
| `relocation:` | `N hints in M files searched`, followed by `, K skipped for size` when `--env-scan-home` left K files unread for their size, by `, R resolved` when R hints carry a value, and by `; see docs/relocation.md` when there is a hint ([Relocation warnings](relocation.md)). |
| `docker:` | Only when a Docker or Podman volume directory or Docker Desktop data was found: `N volumes found, N collected, N unreadable`, followed by ` (run as root to collect)` when a volume was unreadable and by `; Docker Desktop VM disk not collected` when Docker Desktop data was found ([Docker volumes](docker.md)). |

A count of one takes the singular noun (`1 file`, `1 byte`, `1 hint`,
`1 volume found`). Unless `-q` is given, the progress lines of the
[log](#log) go to standard error during the run (without their
timestamps), followed by `done` once the archive is complete.

## Output files

Every file a run writes is named `<host>_<UTC start time>_agent-artifacts`
plus a suffix, in the output directory. In the host name, every byte
other than `A`-`Z`, `a`-`z`, `0`-`9`, `.`, `_` and `-` becomes `-`, so a
character of several bytes becomes as many. The host
name is that of the machine running the command, also in image mode.

```text
ir-ws01_20261122T050937Z_agent-artifacts.tar.gz            the archive
ir-ws01_20261122T050937Z_agent-artifacts.tar.gz.sha256     its SHA-256
ir-ws01_20261122T050937Z_agent-artifacts.manifest.jsonl    the manifest (the archive holds a copy)
ir-ws01_20261122T050937Z_agent-artifacts.collection.json   the run summary (the archive holds a copy)
ir-ws01_20261122T050937Z_agent-artifacts.log               the log (the archive holds a copy)
.stage-ir-ws01_20261122T050937Z_agent-artifacts/           the staging directory, only with -k
```

Every file and directory a run writes there is created new. When
something already holds the name of the staging directory, the log, the
manifest, `collection.json` or the `.sha256` file (a file, a directory, a
symbolic link, one whose target does not exist included, or on Windows a
junction), the run never writes through it, leaves it as it was and ends
with exit `2`: before the log is open with its message on standard error
(`Cannot create staging dir: REASON`, `Cannot create manifest: REASON` or
`open log PATH: REASON`), after it with a `FATAL:` line
(`FATAL: collection.json not written: REASON`, or
`FATAL: archive not written: REASON` for the `.sha256` file), the reason
naming the file. Of two runs started in the same
second into one directory, the second therefore stops and leaves the first
one's files as they were, and a link another account put at one of these
names beforehand is never written through.

The run also checks the staging directory it has just made, once it has
opened it, and refuses it when:

- the name no longer holds the directory it opened (another entry was put
  at the name, or the directory was moved or replaced);
- the directory is not empty;
- on Linux and macOS, and on Windows where the volume keeps access lists, a
  probe file cannot be made in it, or the directory's owner is not the
  owner that probe file is given (on a volume that keeps no owners the two
  always agree).

The run then ends with exit `2` and
`Cannot create staging dir: open DIR: is not an empty directory this run can take as its own; the directory made there is left as it is`,
or, when the directory cannot be opened at all, the open's own reason in
place of the first part. The directory the run made is left in the output
directory, since its name may no longer be the run's.

The run keeps its staging directory open from the moment it makes it, and
every staged copy, the archive's walk of it and its removal at the end go
through that open directory, never through its name. On Linux and macOS
an account that may rename entries in the output directory (it can write
the directory, and the directory has no sticky bit) can still move the
staging directory aside while the run works and put a link in its place,
but the run goes on writing into the directory where it was moved, and at
the end empties it there and leaves the link, with a
`staging directory not removed:` line in the log. On Windows the open
directory cannot be renamed or deleted at all while the run works. A link
recreated inside the staging directory that leads out of it is never
written through on either. What stays open to such an account is the
archive's move to its name, described below; give `-o` a directory that
only the account running the collection can write.

On a volume that keeps no owners or permissions of its own, such as a FAT
or exFAT drive, an NTFS drive mounted on Linux with one owner for every
entry, an NFS export that maps
root to another account, or a macOS volume set to ignore ownership,
nothing can be kept from another account that can reach the volume: every
entry there has the same owner, so the run cannot tell its staging
directory from one another account put at the name, and the mode and access
list it asks for are not kept. The run still works there: on Linux and
macOS it still stages through the open directory, and on Windows it still
holds the name, so the directory cannot be renamed or deleted while it
works.

While it is written, the archive is named
`<host>_<UTC start time>_agent-artifacts.tar.gz.partial`, a file the run
creates new: whatever held that name, or the archive's own name, when
archiving began, a link included, is removed first without being followed.
The archive's SHA-256 is taken of the bytes as the run writes them. Once
the archive is complete and its `.sha256` file is written, the run checks
that the `.partial` name still names the file it wrote, not an entry put
there during the run (that is a `FATAL: archive not written:` line naming
the `.partial` file, and the entry is left as it was), and moves it to its
name; only then is the `done:` line logged. An archive under its own name,
with its `.sha256` file beside it, is therefore one whose run finished, and
`sha256sum -c` on the `.sha256` file checks the bytes the run wrote.

What a run leaves in the output directory depends on how it ended
([Exit codes](collect-options.md#exit-codes)):

| Ending | What is left |
| --- | --- |
| Finished, exit `0` | The archive, its `.sha256` file, the manifest, `collection.json` and the log, ending `done:`. |
| Told to stop, exit `130` | The log, ending `interrupted`, and the manifest of what the run did until then; the partial archive, the `.sha256` file and `collection.json` are removed. |
| A fatal error once the log is open, exit `2` | The log, ending `FATAL: ...`, and the manifest. A fatal error while archiving also leaves `collection.json`, which describes an archive that was not written, and no archive or `.partial` file, unless the finished archive could not be moved to its name: then the `.partial` file and its `.sha256` file are left too. |
| Ended without cleaning up (`SIGKILL`, `SIGQUIT`, or Windows ending the process) | The log without a closing line, the manifest and the staging directory; when it was archiving, `collection.json`, and perhaps a `.partial` file and a `.sha256` file. |

The staging directory is removed in every ending but the last, unless `-k`
is given.

A `.partial` file with a `.sha256` file beside it is a complete archive
whose run was ended, or could not move it to its name, after its
`.sha256` file was written. Rename it to the name the `.sha256` file
gives, the same name without `.partial`, and check it with
`sha256sum -c <archive>.sha256` (`shasum -a 256 -c` on macOS). A
`.partial` file without a `.sha256` file may be incomplete and can be
deleted.

The `.sha256` file is one line, `<hex>  <archive file name>`, the form
`sha256sum -c` and `shasum -a 256 -c` read:

```text
922c3e2a832b0eb642c0398551918c5f702d8075ef8658313522f8f2f79a0acf  ir-ws01_20261122T050937Z_agent-artifacts.tar.gz
```

On Linux and macOS the run sets its file creation mask to `077`, so the
output files are readable by the account that ran it only (mode `600`), and
an output directory it creates is mode `700`. On Windows the files beside
the archive take the permissions the output directory passes on. On a
volume that keeps access lists, such as NTFS, the staging directory is made
with its own access list, taking nothing from the output directory, that
gives the account running the collection and the Administrators group full
control, and everything staged inside inherits it, so a staging directory
`-k` leaves is readable by those two only. On a volume that keeps none
(FAT, exFAT) or does not say (a network share), the staging directory is
made as any other directory there, taking what the output directory
passes on.

The staging directory is where the run copies each file before it packs the
archive; it is removed when the run ends, unless `-k` is given.

## Archive layout

The archive is a gzip-compressed tar file in the POSIX (PAX) format,
always `.tar.gz`. Every member name starts with `./`, and each directory's
entries are in byte order:

```text
./                              the archive's top directory
./collection.json
./collector.log                 the log, as it stood when archiving started
./fs/<original path>            the collected files and recreated symlinks
./manifest.jsonl
```

Under `fs/` each file sits at its original path, the image root left off in
image mode (`/mnt/evidence/home/alice/.claude/history.jsonl` is
`fs/home/alice/.claude/history.jsonl`). On a live Windows host the path
starts with the drive letter (`fs/C/Users/alice/.claude/history.jsonl`),
and a path on a network share with the server and share name
(`fs/server/share/...`). The directories above each collected or skipped
file are members too, so the archive can hold empty directories.

A collected file keeps its modification time, to the second, and on Linux
and macOS its permission bits. Owners are stored as numbers only, never as
names: on Linux and macOS the files' own owners when the run ran as root,
the running account otherwise. The manifest holds each file's original
owner and times in any case. A symlink is stored as a link with its target;
nothing it points to is stored in its place.

A path reached through a symlinked directory whose target stays inside the
same home or project is archived where that directory physically is, so the
recreated link in the archive leads to it: with `.gemini -> dotfiles/gemini`
in bob's home, the manifest `path`
`/home/bob/.gemini/antigravity-cli/conversations/a.db` has the `archive_path`
`fs/home/bob/dotfiles/gemini/antigravity-cli/conversations/a.db`, and the
link itself is archived at `fs/home/bob/.gemini`
([Paths through a symlinked directory](coverage.md#paths-through-a-symlinked-directory)).

The archive's copies of `collection.json` and the manifest are the final
ones. Its copy of the log was taken when archiving started, so only the log
beside the archive has the last line, `done:`, with the archive's size and
hash.

## Manifest

The manifest is JSON Lines: one JSON object per line for every file, link
or directory the run met, collected or not. A row from a Linux or macOS run:

```json
{"user":"alice","home":"/mnt/evidence/home/alice","agent":"claude-code","path":"/mnt/evidence/home/alice/.claude/history.jsonl","archive_path":"fs/home/alice/.claude/history.jsonl","type":"file","size":89,"mtime":1790812800,"atime":1790812800,"ctime":1795324177,"btime":1790812800,"uid":501,"gid":0,"mode":"644","sha256":"db176a33220221c786a18e444fed4ea88e4e3f9cf32fcf8d96945ae10151b70a","secret":false,"status":"collected","target":"","error":""}
```

Every row has every field, in this order, an empty value written as `""`,
`0` or `false`. Rows written on Windows have two more fields, `owner` and
`attributes`, after `mode`.

| Field | Meaning |
| --- | --- |
| `user` | The user whose home the path was found under. For a project file, the user whose agent state named the project, or the user whose home holds a `-p` directory (empty for one outside every home). `docker` for a Docker or Podman volume row. Empty for a system file collected for a relocation hint. |
| `home` | The base the path was collected under, including the image root: the home directory, the project directory, or a volume's `_data` directory. Empty for a system file collected for a relocation hint. |
| `agent` | The catalog agent the file belongs to. For a project file, the agent its project catalog line names: the agent that owns it (`claude-code` for a project's `.claude/`), or `project` for a file several agents read (`AGENTS.md`, `.mcp.json`, the repository's [git metadata](coverage.md#git-metadata) and the other `project` lines in `--list`). For a volume row, the agent of the matched volume name. `shared` for a file collected because it names a relocation variable and no catalog line matches it ([Relocation warnings](relocation.md)); such a file is flagged `secret: true`. For the `symlink` row of a symlinked directory a catalog path runs through, the agent whose catalog paths matched through it, or `shared` when paths of more than one agent did or when the link leads out of the image root; a link that the walk of a catalog match records has that match's agent, and a link that only a startup file or a project's own path runs through is `shared` ([Paths through a symlinked directory](coverage.md#paths-through-a-symlinked-directory)). Empty for `skipped_unmatched_volume` and for the `error_read` row of a home, a project or a `-p` value. |
| `path` | The original full path, including the image root in image mode, in the platform's own form (backslashes on Windows). |
| `archive_path` | The archive member that holds the file or link: `fs/` and the path without the image root, or, for a path reached through a symlinked directory inside its base, the place the base physically holds it ([Archive layout](#archive-layout)). It is spelled as the member is named. Set for `collected` rows and for `symlink` rows whose link is in the archive; empty for every other row. |
| `type` | `file`, `symlink` or `dir` (an excluded real directory, an unmatched volume, an `error_read` directory). A `skipped_special` row names the kind of special file instead: `socket`, `fifo`, `device` or `other` ([Special files](#special-files)). |
| `size` | Bytes. For a `collected` row, the number of bytes copied, which is what `sha256` covers. For every other file or link row, an excluded link included, the size before the copy (for a symlink on Linux and macOS, the length of its target). On Windows that size comes from the file's directory entry, which NTFS updates only when a handle on the file closes, so it can be smaller than the file for one a running program holds open and writes ([Windows](windows.md#locked-files-and-long-paths)). For an excluded real directory or an unmatched volume, the size of everything under it: on Linux and macOS the space allocated, as `du -sk` reports it, in bytes; on Windows the sum of the files' sizes. `0` for an `error_read` row. |
| `mtime`, `atime`, `ctime`, `btime` | Modification, access, change and birth (creation) time, in seconds since 1970 UTC, read from the file itself before it was copied, never through a link. `btime` is `0` where the file system keeps no birth time; on Linux it is read through `statx` and is `0` on a kernel without it or under a system-call filter that refuses it. `ctime` is `0` on Windows, which keeps no change time. A time before 1970 is `0`. All four are `0` for an unmatched volume on Linux and macOS, and for the `error_read` row of a `-p` value that does not exist. |
| `uid`, `gid`, `mode` | Linux and macOS: the numeric owner and group and the permission bits, the setuid, setgid and sticky bits included, as an octal string (`"644"`, `"4755"`); `0`, `0` and `"0"` where nothing could be read, as for an unmatched volume. Windows: `0`, `0` and `""`. |
| `owner` | Windows only: the account that owns the path itself (for a link, the link, not its target), as `DOMAIN\name`, the account name alone when it has no domain, the security identifier (`S-1-5-21-...`) when no account matches it, or `""` when it cannot be read. |
| `attributes` | Windows only: the file's attribute names in ascending bit order, joined by `, ` (`Archive`, `Directory, ReparsePoint`, `Hidden, System, Archive`), or the attribute value as a decimal number when it is `0` or holds a bit that has no name in this list. |
| `sha256` | The SHA-256 of the bytes copied into the archive, for `collected` rows only. The hash is taken during the copy, so it matches the archive member even if a running agent kept writing to the source. |
| `secret` | `true` when the path, relative to `home`, matches a credential pattern, or the file was collected for a relocation hint. |
| `status` | What happened to the path ([Statuses](#statuses)). |
| `target` | For a symlink (on Windows also a junction), its target as the link stores it, which may be relative, an excluded link included. Empty when the target cannot be read. |
| `error` | For `error_copy` and `error_read`, why: the operating system's description of the failure, such as `open /home/carol/.claude/x: permission denied`, or on Windows `open C:\Users\carol\.claude\x: Access is denied.` For a `symlink` row with no `archive_path`, `not recreated in the archive: ` and the reason the link could not be made in the staging directory. |

Rows come in the order of the run's steps: the homes, in the order they
were found, each home's catalog matches by agent and then pattern, then the Docker and Podman volumes, the projects, and the files
collected for relocation hints. Within one directory walk, the rows of the
walk are written together when it ends.

The manifest is written by Go's `encoding/json`: `"` and `\` are escaped,
control characters are written as escapes, and so are the characters
U+2028 and U+2029, so every line is one valid JSON object. A file name that
is not valid UTF-8 is written with the character U+FFFD in place of each
byte that is not part of a valid character; the archive member keeps the
name's real bytes. On Windows a name holding a lone surrogate (a half of a
UTF-16 character pair, which Windows allows in a name) is written with
three U+FFFD characters, one per byte of its stored form.

### Statuses

| Status | Meaning |
| --- | --- |
| `collected` | Copied, hashed and archived. |
| `symlink` | A symbolic link, or on Windows a symbolic link or junction, recorded with its target and never followed. The link is recreated in the archive with its target as stored; where it cannot be (on Windows, without the right to create links), the row has no `archive_path` and `error` says why, and the log has `symlink not recreated: <path>: <reason>`. A symlinked directory that a catalog path runs through gets a row too, under the agent whose catalog paths matched through it, or `shared` when paths of more than one agent did or when the link leads out of the image root (so a `~/.config` link that `amp` and `goose` paths pass through gives a `shared` row), unless the walk of a catalog match records the link, in which case it has that match's agent. A symlinked directory whose target leads out of the home or project, or cannot be resolved, is recorded and not walked ([Paths through a symlinked directory](coverage.md#paths-through-a-symlinked-directory)). |
| `skipped_excluded` | Matched a default exclusion (not with `--full`). An excluded real directory is one `dir` row with its own metadata and the size of everything under it; an excluded file is one `file` row with its size. An excluded symlink (on Windows also a junction) is one `symlink` row with the link's own metadata and size and its target, never followed and with no `archive_path`. |
| `skipped_size` | Larger than `--max-file-size`. |
| `skipped_secret` | A credential file left out by `--no-secrets`. |
| `skipped_special` | A special file: neither a regular file, a directory nor a link ([Special files](#special-files)). Never opened; `archive_path`, `sha256`, `target` and `error` are empty. |
| `error_copy` | The file could not be read or copied; `error` says why, and the log has `copy failed: <path>: <reason>`. Also a file the output directory cannot hold beside another of a name it takes for the same one: `the output directory takes its name for another file's, link's or directory's; not staged` ([Case and Unicode form in the output directory](collect-options.md#case-and-unicode-form-in-the-output-directory)). A file reached a second time at the same staged place after it was replaced during the run gets `changed since this run staged it; the earlier row holds its copy`. |
| `skipped_unmatched_volume` | A Docker or Podman volume whose name matches no agent: one `dir` row with its size, `user` `docker`, an empty `agent`, and `home` and `path` both the volume's `_data` directory. |
| `error_read` | A directory that could not be listed or entered: a home, a project, a `-p` value that is not a directory, or a directory inside a walk ([Privileges](privileges.md)). One `dir` row with `size` `0`, no `archive_path` and the reason in `error`; nothing under it is collected. |

### Check order

A path that no exclusion matches is checked first for being a link, then
for being a special file, then against `--no-secrets`, then against
`--max-file-size`. So a link is never skipped for its size, a special file
is `skipped_special` even when it matches a credential pattern under
`--no-secrets` (its `secret` field still says so), and a credential file is
`skipped_secret` under `--no-secrets` whatever its size.

### Special files

A socket, a named pipe (FIFO) or a device is never opened: reading a named
pipe waits for a writer, and reading a device or socket reads something
that is not a file's contents. It gets one `skipped_special` row whose
`type` names the kind:

| Platform | `type` values |
| --- | --- |
| Linux and macOS | `socket` (a Unix domain socket, such as the ones Codex CLI keeps under `.codex`), `fifo` (a named pipe), `device` (a block or character device), else `other` |
| Windows | `other`: a reparse point other than a symlink or junction, such as the Unix domain sockets Codex CLI keeps under `.codex\app-server-daemon` while its daemon runs |

A reparse point is the Windows mechanism behind every kind of file system
link and some special files ([Windows](windows.md#symlinks-and-reparse-points)).
`--inventory` leaves special files out of its file counts.

## Log

The log is a text file with one line per event, each starting with a UTC
time to the second. Unless `-q` is given, the same lines go to standard
error during the run, without the time. When standard error is closed
during a run, those copies are dropped: the log file still gets every line
and the run finishes as usual.

A control character in a line, such as a newline, a carriage return or a
terminal escape in a file name, is written as `\x` and its two
hexadecimal digits (`\x0a` for a newline), in the file and on standard
error alike, so a name can neither end a line nor start a forged one. So
is each byte that is not part of a valid UTF-8 character. The C1 controls
(U+0080 to U+009F) and the characters U+2028 and U+2029 are written as
`\u` and four hexadecimal digits (`\u2028`). The analysis commands print
text to a terminal in the same form, U+2028 and U+2029 aside. A name that
itself holds the text `\x0a` reads the same; the manifest holds every name
exactly.

A run on a macOS workstation over a mounted image:

```text
2026-11-22T05:09:37Z doubleagent <version> starting on ir-ws01 (Darwin 27.2.0) mode=image root=/mnt/evidence uid=501
2026-11-22T05:09:37Z hash=crypto/sha256 max_file_size=256MB full=0 no_secrets=0 no_docker=0 env_scan_home=0 env_scan_max_size=1MB no_preserve_atime=0
2026-11-22T05:09:37Z NOTE: output directory /cases/host01 ignores letter case or Unicode form in names; a file or link whose name it takes for another file's, link's or directory's is not staged and its row says why (an output directory on a file system that compares names byte for byte keeps both)
2026-11-22T05:09:37Z User alice (/mnt/evidence/home/alice)
2026-11-22T05:09:37Z   [claude-code] /mnt/evidence/home/alice/.claude
2026-11-22T05:09:37Z   [claude-code] /mnt/evidence/home/alice/.claude.json
2026-11-22T05:09:37Z   [codex-cli] /mnt/evidence/home/alice/.codex
2026-11-22T05:09:37Z Docker volumes in /mnt/evidence/var/lib/docker/volumes
2026-11-22T05:09:37Z   [agent-zero] /mnt/evidence/var/lib/docker/volumes/a0_usr/_data
2026-11-22T05:09:37Z Project [alice] /mnt/evidence/srv/proj
2026-11-22T05:09:37Z   [claude-code] /mnt/evidence/srv/proj/.claude
2026-11-22T05:09:37Z Relocation hints for alice (/mnt/evidence/home/alice)
2026-11-22T05:09:37Z relocation hint: /mnt/evidence/home/alice/.bashrc:1 names CODEX_HOME (codex-cli)
2026-11-22T05:09:37Z Relocation hints in system files
2026-11-22T05:09:37Z relocation hint: /mnt/evidence/etc/environment.d/agents.conf:1 names CODEX_HOME (codex-cli) = /mnt/evidence/opt/x
2026-11-22T05:09:37Z Archiving
2026-11-22T05:09:37Z done: /cases/host01/ir-ws01_20261122T050937Z_agent-artifacts.tar.gz (6260 bytes, sha256 922c3e2a832b0eb642c0398551918c5f702d8075ef8658313522f8f2f79a0acf)
```

The first line names the version, the host, the system, the mode, the root
(`/` or `\` in live mode) and the account: `uid=<n>` on Linux and macOS,
`run_as=<DOMAIN\user> admin=<0|1>` on Windows. The second names the hash and
the options. On Windows a third line says how files are opened, from the
system's last-access setting ([Access times](deployment.md#access-times)).
Then come the warnings that apply to the whole run.

| Line | When |
| --- | --- |
| `WARNING: not running as root; other users' homes will probably be unreadable` | A live Linux or macOS run that is not root. On Windows: `WARNING: not running as Administrator; other users' profiles will probably be unreadable`. |
| `NOTE: --inventory writes nothing; -o DIR ignored` | `--inventory` with `-o`. |
| `NOTE: output directory DIR ignores letter case or Unicode form in names; ...` | The output directory takes two different names for one ([Case and Unicode form in the output directory](collect-options.md#case-and-unicode-form-in-the-output-directory)). |
| `WARNING: no user home directories found` | No home was found. |
| `WARNING: -u NAME matches no user home; homes found for: A, B` | A `-u` name that matches no home; `WARNING: -u lists no names` for a list naming nobody. |
| `profile not accessible (skipped): PATH` | Windows: a profile the registry lists whose directory cannot be looked at. |
| `User USER (HOME)` | Before the first catalog match of a home; a home with no match gets no line. |
| `  [AGENT] PATH` | One per catalog match, before its files are copied. A directory that several catalog lines reach, as lines differing only in letter case do on a file system that ignores case, has one such line, with PATH spelled as on disk ([Case variants](coverage.md#case-variants)). |
| `read failed: PATH: REASON` | A directory that could not be listed or entered; it has an `error_read` row. |
| `copy failed: PATH: REASON` | A file that could not be copied; it has an `error_copy` row. |
| `symlink not recreated: PATH: REASON` | A link that could not be made in the staging directory. |
| `Docker volumes in DIR`, `  [AGENT] DIR/_data`, `NOTE: docker: ...` | The volume step ([Docker volumes](docker.md)). |
| `Project [USER] PATH` | Before the files of a project; `Project PATH` for a `-p` directory no home holds. It ends `(through the symlinked directory LINK, archived under PATH)` for a project whose own path runs through a link inside a home. |
| `WARNING: project given with -p is not a directory: PATH: REASON` | A `-p` value that is not a directory. |
| `Relocation hints for USER (HOME)`, `Relocation hints in project PATH`, `Relocation hints in system files`, `Relocation hints in the registry` | Before the first hint of each place searched. |
| `relocation hint: FILE:LINE names VARIABLE (AGENT)` | One per hint, followed by ` = VALUE` when the value was read, or `; value not resolved: REASON` ([Relocation warnings](relocation.md#what-the-warning-means)). |
| `sweep skipped: home PATH lies outside the user home roots`, `sweep skipped: project PATH holds a home directory` | With `--env-scan-home`, a home or project that is not searched ([Search the whole home](relocation.md#search-the-whole-home)). |
| `access time not held: PATH: REASON`, `access times not held: N file(s)` | Windows: a file read without the hold on its access time the run wanted ([Access times](deployment.md#access-times)). |
| `Archiving` | The walk is over and the archive is being written, under its `.partial` name. |
| `done: ARCHIVE (SIZE bytes, sha256 HEX)` | The archive is complete and has its name. Written to the log file only, after the archive took its copy of the log. |
| `interrupted` | The run was told to stop before its archive had its name, and exits `130`; a log holds this line or `done:`, never both. |
| `FATAL: ...` | The run stopped and exits `2`; `FATAL: archive not written: REASON` when the archive failed, the reason naming the file, and `FATAL: archive not moved to its name: REASON; it is complete as PARTIAL, with its .sha256 file beside it` when the finished archive could not be given its name. |
| `staging directory not removed: REASON` | The last line, after `done:`, `interrupted` or `FATAL:`, when the staging directory could not be removed at the end: `remove DIR: no longer names the staging directory this run made` when another entry took its name during the run, in which case the entry at the name is left as it is and the directory itself is emptied wherever it was moved; when emptying it failed too, that error follows after `; `. Not written with `-k`. |

In image mode, a link whose target leads out of the image root is named
once by the step that met it, and nothing behind it is read: a line ending
`leads out of the image root; nothing behind it was listed` (or `... was
listed or taken as a home`, `... nothing under it was listed, not walked`,
`... nothing behind it was listed or read`), starting with `user homes:`,
`docker:`, `startup files:`, `Discovery [USER]`, `Project` or, in the
catalog walk, `[shared]`. On Windows a path that the check let through but Windows opened
outside the root is logged with `the root check allowed it but Windows
opened it outside the image root; nothing from it was used`.

No line holds a file's contents, a matched line of a startup file or a
secret.

## collection.json

`collection.json` records the run as one JSON object, indented by two
spaces. From the run whose log is above:

```json
{
  "tool": "doubleagent",
  "version": "<version>",
  "hostname": "ir-ws01",
  "mode": "image",
  "root": "/mnt/evidence",
  "platform": "Darwin 27.2.0 arm64",
  "run_as_uid": "501",
  "started": "2026-11-22T05:09:37Z",
  "finished": "2026-11-22T05:09:37Z",
  "options": {
    "full": false,
    "no_secrets": false,
    "max_file_size_bytes": 268435456,
    "users_filter": "",
    "no_docker": false,
    "no_projects": false,
    "projects": [],
    "env_scan_home": false,
    "env_scan_max_size_bytes": 1048576,
    "no_preserve_atime": false
  },
  "capabilities": {
    "hash_tool": "crypto/sha256",
    "archiver": "archive/tar",
    "grep_text": true,
    "plutil": true
  },
  "users": [
    "alice"
  ],
  "homes": [
    "/mnt/evidence/home/alice"
  ],
  "users_with_artifacts": [
    "alice"
  ],
  "projects": [
    "/mnt/evidence/srv/proj"
  ],
  "counts": {
    "collected": 34,
    "symlink": 1,
    "skipped_excluded": 5,
    "skipped_size": 0,
    "skipped_secret": 0,
    "skipped_special": 1,
    "error_copy": 0,
    "skipped_unmatched_volume": 1,
    "error_read": 0,
    "collected_bytes": 1242
  },
  "docker": {
    "volumes_found": 2,
    "volumes_collected": 1,
    "unreadable": 0,
    "docker_desktop": false
  },
  "relocation": {
    "hints": [
      {
        "user": "alice",
        "source": "/mnt/evidence/home/alice/.bashrc",
        "line": 1,
        "variable": "CODEX_HOME",
        "agent": "codex-cli",
        "value": ""
      },
      {
        "user": "",
        "source": "/mnt/evidence/etc/environment.d/agents.conf",
        "line": 1,
        "variable": "CODEX_HOME",
        "agent": "codex-cli",
        "value": "/mnt/evidence/opt/x"
      }
    ],
    "registry": {
      "read": false,
      "reason": "not Windows",
      "users_read": [],
      "users_not_loaded": [],
      "users_denied": []
    },
    "files_searched": 2,
    "files_skipped_size": 0
  },
  "notes": [],
  "archive": "ir-ws01_20261122T050937Z_agent-artifacts.tar.gz"
}
```

Every key is always written, in this order, an empty list as `[]`.

| Key | Meaning |
| --- | --- |
| `tool`, `version` | `doubleagent` and its version, as `doubleagent --version` prints it. |
| `hostname` | The name of the machine running the command: on Linux and macOS the system's host name, on Windows `COMPUTERNAME`, else the DNS host name. In image mode it is the analyst workstation's name, not the imaged host's; this is the only place the output names the host. |
| `mode` | `live`, or `image` with `-r`. |
| `root` | The image root, absolute. In live mode `/` on Linux and macOS and `\` on Windows. |
| `platform` | The system running the command. Linux and macOS: the kernel name, release and machine, as `uname -s`, `-r` and `-m` print them (`Linux 6.8.0-50-generic x86_64`, `Darwin 27.2.0 arm64`). Windows: `Windows`, the version as major, minor and build, and `PROCESSOR_ARCHITECTURE` (`Windows 10.0.26100 AMD64`). |
| `run_as_uid` | Linux and macOS: the numeric user ID the command ran as, as a string. |
| `run_as`, `run_as_admin` | Windows, in place of `run_as_uid`: the account the command ran as, `DOMAIN\user`, and whether it is a member of the Administrators group with that membership in effect (a window opened with **Run as administrator**). |
| `started`, `finished` | UTC times to the second, `2026-11-22T05:09:37Z`. `finished` is taken when the walk ends, before archiving. |
| `options` | The command line: `full`, `no_secrets`, `max_file_size_bytes` (`0` for no limit), `users_filter` (the `-u` text as given), `no_docker`, `no_projects`, `projects` (every `-p` value, made absolute and listed once, in the order given; a value that is not a directory is listed too, and the values are recorded even with `no_projects`), `env_scan_home`, `env_scan_max_size_bytes` (`0` for no limit; recorded even without `env_scan_home`) and `no_preserve_atime`. |
| `capabilities` | Fixed facts about the program: `hash_tool` is `crypto/sha256` and `archiver` is `archive/tar` (the hash and archive code built into the program); `grep_text` is `true`, since the relocation search reads bytes and finds a name after a NUL byte; `plutil` says whether `plutil`, the property list tool macOS ships, was found, which decides whether a launchd property list's value is read ([Which values are read](relocation.md#which-values-are-read)). |
| `users`, `homes` | Parallel lists of every user and home searched, homes with no agent state included. |
| `users_with_artifacts` | The users whose home produced at least one manifest row other than its own `error_read` row, in the order of `users`. Rows from projects and Docker volumes do not count. The stdout `users:` line is its length. |
| `projects` | The project directories collected from, found or given with `-p`. A `-p` value that is not a directory, and a project left unwalked because it leads out of the image root, are not listed. |
| `counts` | The rows per status (`collected`, `symlink`, `skipped_excluded`, `skipped_size`, `skipped_secret`, `skipped_special`, `error_copy`, `skipped_unmatched_volume`, `error_read`), then `collected_bytes`, the total `size` of the `collected` rows. |
| `docker` | `volumes_found`, `volumes_collected`, `unreadable` and `docker_desktop` ([Docker volumes](docker.md)). |
| `relocation` | The relocation search: `hints`, `registry`, `files_searched` and `files_skipped_size` ([Relocation warnings](relocation.md)). |
| `relocation.hints` | One object per hint, in the order found: `user` (the owner of the home or the project's user; empty for a system file, except a launchd property list under `/Library/LaunchDaemons` that names a plain account in its `UserName`; for a registry value the profile's user, empty for the system key), `source` (the file that names the variable, or the registry key and value name, such as `HKU\S-1-5-21-...\Environment\CLAUDE_CONFIG_DIR`), `line` (the line number; `0` for a registry value and for the value `plutil` read from a property list as a whole), `variable`, `agent` (the agent the variable's catalog line belongs to, or `shared`) and `value` (the absolute path the line assigns when it was read, in image mode with the root in front; `""` otherwise). A variable that two agents list gives one hint per agent. |
| `relocation.registry` | `read` (`true` when the registry was read), `reason` (why not: `not Windows`, or `image mode`; `""` when it was read), `users_read`, `users_not_loaded` and `users_denied` (the users whose `Environment` key was read, whose registry hive was not loaded, and whose loaded hive could not be opened) ([The registry](relocation.md#the-registry)). |
| `relocation.files_searched`, `relocation.files_skipped_size` | The files searched for relocation variables, each counted once (the startup files, and with `--env-scan-home` every other file swept), and the swept files left unread for being over `--env-scan-max-size`. |
| `notes` | Messages about what could not be collected: a Docker or Podman volume directory that is a symlink or unreadable, a volume that could not be read or walked completely, Docker Desktop data. |
| `archive` | The archive's file name. |
