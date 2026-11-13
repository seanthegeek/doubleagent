# Windows

Windows gives the PowerShell collector three problems the sh collector
never meets. A Windows host may lack `tar.exe`, so the collector carries
two more archivers and records which one wrote the archive. NTFS has
reparse points, symbolic links and junctions among them, which must be
recorded without being followed. And a file held open by a running
editor, or a path longer than 260 characters, must still be copied under
Windows PowerShell 5.1 with no host setting changed. Each is handled as
described below, and an analyst reading a Windows archive or manifest
finds the traces of each here.

## Archiver: `tar.exe`, the PowerShell tar writer or `ZipFile`

The script tries up to three archivers in order, and the first that writes
an archive wins:

| Archiver | `capabilities.archiver` | Archive | Used when |
| --- | --- | --- | --- |
| `tar.exe -czf <archive> -C <stage> .` | `tar.exe` | `.tar.gz` | first, when `tar.exe` is on the `PATH` (Windows 10 1803 and later, Windows 11, Server 2019 and later) |
| the PowerShell tar writer in the script | `PowerShell tar writer` | `.tar.gz` | `tar.exe` is not on the `PATH`, exits non-zero or leaves no archive |
| `System.IO.Compression.ZipArchive` | `ZipFile` | `.zip` | the PowerShell tar writer failed too |

Some builds of the built-in `tar.exe` exit `0xC0000005`, an access
violation, when the staging tree holds a name outside the ANSI code page,
such as a CJK or Cyrillic one. This was seen with bsdtar 3.8.8 (`tar.exe`
10.0.26100) on Windows 11, where no option or format makes it store such
names; the `tar.exe` on the GitHub `windows-latest` runner stores them.
When `tar.exe` fails, the log has a line
`tar.exe failed: exit code 0xC0000005` (hex for a negative exit code,
decimal otherwise), its partial archive is removed, and `collection.json`
`notes` gets `tar.exe failed (exit code 0xC0000005); fell back to PowerShell
tar writer`. A failure of the PowerShell tar writer is logged with the
exception's message (`PowerShell tar writer failed: <message>`) and noted in
the same way before the zip is written.

The PowerShell tar writer uses only .NET Framework APIs (`GZipStream` over a
`FileStream`). It writes POSIX ustar headers with the member names
`tar -czf <archive> -C <stage> .` gives (`./`, `./fs/...`,
`./manifest.jsonl`, `./collection.json`, `./collector.log`, directories
ending in `/`), so the archive layout is the same as with `tar.exe` and the
sh collector. A path that is not ASCII, or does not fit the ustar name and
prefix fields, goes in a pax extended header (typeflag `x`) as a UTF-8
`path` record, as does a symlink target (`linkpath`) and a size above 8 GiB
(`size`). It stores directories (mode 0755), regular files (0644) and
symlinks (0777) with uid and gid 0 and each staged item's last write time.
It reads the staged files with shared access in 1 MiB chunks, lists the
staging tree without following reparse points, and stores a symlink's target
string exactly as the link was staged, so unlike `tar.exe` it keeps a
relative target's backslashes and does not add `\\?\` to an absolute one.

The zip is written entry by entry with `/` separators and no `./` prefix
(the .NET Framework `ZipFile.CreateFromDirectory` used up to 1.10.2 wrote
`\`), with an entry for each empty directory.

The summary's `capabilities.archiver` and `archive` name the archiver and
the archive actually written: the summary and the log are rewritten and
restaged before each fallback, so both the copy inside the archive and the
one beside it name the archiver that wrote it.

## Symlinks and reparse points

A reparse point is an NTFS entry whose contents are interpreted by a file
system filter named by its reparse tag; symbolic links and junctions are
the common ones. The collector reads the item's `LinkType`: a reparse point
whose `LinkType` is `SymbolicLink` or `Junction` is a link and gets a
`symlink` row, and any other reparse point, whose `LinkType` is empty, is a
special file with a `skipped_special` row of `type` `other`, never opened
or followed. Windows PowerShell 5.1 exposes no reparse tag, so the kind
cannot be named. The case seen is the AF_UNIX (Unix domain) socket, reparse
tag `IO_REPARSE_TAG_AF_UNIX` (`0x80000023`), which Codex CLI 0.162.1 keeps
under `.codex\app-server-control` and `.codex\app-server-daemon` while
its app-server daemon runs; collector 1.12.0 recorded each as a `symlink`
whose `target` was its own path in extended-length form. When `LinkType`
cannot be read at all, the reparse point is still treated as a link. See
[Special files](output.md#special-files).

Symlinks and junctions are recreated in the staging directory as
symbolic links with the target string exactly as `Target` reports it, as the
sh collector does with `ln -s`, so a relative target stays relative and a
dangling one is kept. A junction is recreated as a directory symlink to the
same target.

`tar.exe` stores the link, not the file or directory it points to
(libarchive archives a symlink reparse point as a link and does not descend
into it,
[`archive_read_disk_windows.c`](https://github.com/libarchive/libarchive/blob/7219b0134d771dc4b51bf86b4d01761b87398b1b/libarchive/archive_read_disk_windows.c#L2046-L2050)
at v3.8.8); the libarchive 3.3.2 `tar.exe` of early Windows 10 builds stores
the link entry without its target text. `tar.exe` stores a relative target
with its backslashes turned into forward slashes and an absolute target with
a `\\?\` prefix; the `tar.exe` on the GitHub `windows-latest` runner writes
that one with forward slashes too, `//?/C:/...`
([`archive_read_disk_windows.c`](https://github.com/libarchive/libarchive/blob/7219b0134d771dc4b51bf86b4d01761b87398b1b/libarchive/archive_read_disk_windows.c#L396-L405)),
so the link in the archive can differ from the manifest row's `target`,
which keeps the original string.

An NTFS hard link is not a reparse point, so each of its paths is an
ordinary file to the collector: every path gets its own `collected` row and
the bytes are staged and archived once per path. Antigravity's `cli.log`,
a hard link to the newest file under its `log/` directory, is collected
twice this way
([Per-user storage in `research/antigravity.md`](../../research/antigravity.md#per-user-storage)).
The sh collector does the same on any file system, since `cp` copies each
path on its own.

### Creating the link

Creating a symlink on Windows needs the `SeCreateSymbolicLinkPrivilege`
(Administrator) or Developer Mode. Windows PowerShell 5.1 has no managed API
for this, so it runs `cmd.exe /d /v:off /c mklink` (with `/D` for a
directory link or a junction), which keeps the target string verbatim and
accepts a target that does not exist;
[Microsoft documents](https://blogs.windows.com/windowsdeveloper/2016/12/02/symlinks-windows-10/)
that `mklink` creates links without elevation in Developer Mode from the
Windows 10 Creators Update (1703). `cmd.exe` expands `%NAME%` even inside
quotes, so a link whose path or target contains `%` or `"` is not passed to
it. PowerShell 7 uses .NET's `CreateSymbolicLink` instead and has neither
limit.

Without the privilege, for a `%` or `"` under 5.1, or for a reparse point
with no single link target, the row keeps `archive_path` empty and `error`
says why (under 5.1, `mklink`'s message and exit code), and
`collection.json` `notes` counts the links that could not be recreated.

### Symlinks and the zip fallback

A `.zip` cannot hold symlinks and `ZipFile` would copy the target's bytes in
their place, possibly from outside the collection, so the zip archive never
contains them:

- When the zip is the archiver from the start, which happens only when the
  `CAC_ARCHIVER` test hook says so (see [testing.md](testing.md)), no link
  is created, every `symlink` row has an empty `archive_path` and the
  `error` `not recreated in the archive: zip cannot store symlinks`, and a
  note says how many symlinks are in the manifest only. Without `tar.exe`
  the PowerShell tar writer comes first and stores the links.
- When `tar.exe` and the PowerShell tar writer both fail after the links
  were staged, they are deleted from
  the staging directory before zipping, the manifest is rewritten so those
  rows have an empty `archive_path` and the `error`
  `not in the archive: tar failed and zip cannot store symlinks`, a note
  records the removal, and the rewritten manifest and summary are what the
  zip and the output directory hold.

### PowerShell 7 on Linux or macOS

Under PowerShell 7 on Linux or macOS the links are made with .NET's
`CreateSymbolicLink`, the first archiver is the system `tar` (still named
`tar.exe` in `capabilities.archiver`), which stores UTF-8 names, and the
same fallbacks and rules apply when it fails or is missing.

### Staged link cleanup

Staged links are deleted one by one before the staging directory is
removed, so the cleanup never descends through a link. If one cannot be
deleted, the staging directory is kept and the log says so, and a zip fall
back is not attempted (the run exits `2` with no archive) rather than
zipping through the link.

## Locked files and long paths

Files held open by a running editor (Cursor's `state.vscdb`, Electron
LevelDB stores) are read with shared access so they still copy. A few
files that a running Electron agent app opens without sharing, or with a
byte range locked, cannot be, and get `error_copy` rows with the
exception message: the Chromium cookie store `Network\Cookies` and its
`Cookies-journal`, and the `LOCK` files of the `Local Storage\leveldb`
and `Session Storage` LevelDB stores (LevelDB is the key-value store
Chromium keeps them in). This was seen on a live Claude Desktop host,
where the cookies are the claude.ai session cookies and flagged secret,
and on a live Antigravity desktop app 2.23.1 on Windows 11 (October
2026), where the 0-byte `presence\<uuid>.lock` of the open conversation
under `.gemini\antigravity` failed the same way. The ChatGPT desktop app
26.1007.9446.0 (October 2026) showed the same classes in its Chromium
profile and partitions, and also held its package's settings hive
`Settings\settings.dat` and its `.LOG1` and `.LOG2` logs, and the
bundled Codex's `.codex\.sqlite-maintenance.lock` and
`.codex\thread-writer-locks\<uuid>.lock` files. The lock files are
empty, so only the cookie store and the settings hive are lost; close the
app or collect from an image to get them.

A file a running process holds open and keeps writing can have a stale
length in its directory entry: NTFS updates the length it lists only
when a handle on the file closes, so `Get-ChildItem` and the `Get-Item`
the collector reads before the copy can report fewer bytes than the
file holds. On the Antigravity host above, the app's
`logs\language_server.log` was listed at 0 bytes while it held 218,747,
and Chromium's `DIPS-wal` at 32 bytes while it held 210,152. The copy
reads the real bytes, and from collector 1.12.0 a `collected` row's
`size` is the number of bytes copied, so it matches the `sha256` and the
archived member; earlier versions wrote the stale listed length. A
`skipped_size` decision, and the size of a `skipped_size`,
`skipped_secret`, `skipped_special`, `symlink`, `error_copy` or excluded row, still use the
listed length (see the `size` field in [output.md](output.md#manifest)).

Every file system call addresses its path in the extended-length form,
`\\?\C:\...` for a drive path and `\\?\UNC\server\share\...` for a UNC
path such as a `-Root` on a network share, so paths longer than 260
characters are enumerated, copied and hashed without `LongPathsEnabled` on
the host, under Windows PowerShell 5.1 and PowerShell 7. This covers both
sides of the copy: the source tree, and the staging directory, whose paths
are longer than the source's by the output directory, the
`.stage-<name>` directory and `fs\<drive>`. The prefix never reaches the
output: the manifest `path`, `archive_path` and `target`, `collection.json`,
the log and stdout keep the plain form, and an exception message quoted in
`error` has the prefix removed. A file that still fails to copy is recorded
as `error_copy`.

The archivers store the long paths as well. `tar.exe` is given the plain
staging path and stores members past 260 characters itself. The PowerShell
tar writer and the `ZipFile` fallback list and read the staged files
through the extended form; the tar writer stores a name too long for the
ustar fields in a pax `path` record, and a zip entry name has no
260-character limit.

A directory that the walk cannot list gets an `error_read` row rather than
being skipped silently: a directory inside a catalog match and, in the
PowerShell collector, a directory that a wildcard segment of a
catalog line has to list (`.local/share/*/flow_states.db*` lists every
directory under `.local/share`), with the agent of the first catalog line
that needed it. The home or project directory itself is reported once, and
a directory at or under a catalog match gets its row from the walk of that
match.

## The registry step

On a live Windows host the relocation step also reads the registry, because
Windows keeps a user's and the machine's environment variables there rather
than in a file ([relocation.md](relocation.md#the-registry)). It reads only
values whose names are relocation variables, in two places:

- The system environment key,
  `HKLM\SYSTEM\CurrentControlSet\Control\Session Manager\Environment`.
- The `Environment` key of the loaded `HKEY_USERS` hive of each collected
  user whose profile has a security identifier starting `S-1-5-21-` or
  `S-1-12-`. `-Users` limits which users these are.

A value is read without expanding it. A `REG_SZ` value is kept as written; a
`REG_EXPAND_SZ` value has its `%NAME%` references replaced from the same key,
the user's profile paths and the system key, and never from the environment of
the process that runs the collector, which belongs to the account that started
it. A drive path with no leftover reference is kept as the hint's `value`.

The step never loads a hive: loading one with `reg load` or a profile loader
writes to the user's `NTUSER.DAT`, which an evidence collection must not do.
A user whose hive is not loaded is listed in `relocation.registry.users_not_loaded`
and in the log. Opening another user's loaded hive is an Administrator action:
a hive the account cannot open is listed in `users_denied` with the message
Windows gave, and a run from an elevated PowerShell reads every loaded hive.
The step does not run in image mode or under PowerShell 7 on Linux or macOS;
`relocation.registry.reason` says which.
