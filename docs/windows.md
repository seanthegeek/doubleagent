# Windows

A collection on Windows, live or over a Windows disk image, meets things a
Linux or macOS host does not have: reparse points (the mechanism behind
symbolic links, junctions and some special files), files held open by a
running program, paths longer than 260 characters, directories marked
case-sensitive, short 8.3 names, file owners that are accounts rather than
numbers, and environment variables kept in the registry. This page says how
`doubleagent collect` handles each and what an analyst finds of it in the
manifest and the log. The archive itself is the same `.tar.gz` on every
platform.

## Symlinks and reparse points

A reparse point is an NTFS entry whose contents a file system filter
interprets, named by its reparse tag; symbolic links and junctions (an
older kind of directory link) are the common ones. The run reads each
entry's own reparse tag, never following it:

- A symbolic link or a junction gets a `symlink` row with its target as
  stored, and is never followed. It is recreated in the staging directory
  and stored in the archive as a link with that target; a junction is
  recreated as a directory symlink to the same target. A relative target
  stays relative, and a target that does not exist is kept.
- Any other reparse point is a special file: an AF_UNIX (Unix domain)
  socket, such as the ones Codex CLI keeps under
  `.codex\app-server-daemon` while its daemon runs, a cloud-file placeholder
  (a file in a folder that a sync client such as OneDrive manages), a
  deduplicated file, a WSL symlink or an app execution alias. It gets a
  `skipped_special` row of `type` `other` and is never opened, followed or
  copied. A directory that is a reparse point of any kind is not walked.
  Collect a cloud-synced folder by hand if it matters.

The `attributes` field of such a row includes `ReparsePoint`
([Manifest fields on Windows](#manifest-fields-on-windows)), and the
`owner` field of a link is the link's own owner, not its target's.

Creating a symbolic link needs the `SeCreateSymbolicLinkPrivilege`, which
Administrators hold, or Developer Mode, which lets any account create one.
Without either, a link cannot be recreated in the staging directory: its
row keeps the target but has no `archive_path`, its `error` starts
`not recreated in the archive:` with Windows's reason, and the log has
`symlink not recreated: PATH: REASON`. The link's target is still in the
manifest.

### Symlinks and the zip fallback

The archive is a gzip-compressed tar file on Windows as everywhere, never a
`.zip`, which could not hold a link. Every link recreated in the staging
directory is stored in the archive as a link, and nothing it points to is
stored in its place, so its `symlink` row keeps its `archive_path`.

### Hard links and unreadable directories

An NTFS hard link is not a reparse point, so each of its paths is an
ordinary file: every path gets its own `collected` row and is copied and
archived once per path. Antigravity's `cli.log`, a hard link to the newest
file under its `log\` directory, is collected twice this way
([Per-user storage in `research/antigravity.md`](../research/antigravity.md#per-user-storage)).

A directory that the walk cannot list or enter gets an `error_read` row
([Privileges](privileges.md)). A directory that a wildcard in a catalog
line has to list (`.local/share/*/flow_states.db*` lists every directory
under `.local\share`) and cannot gets no row.

## Locked files and long paths

Every file is opened with shared access for reading, writing and deletion,
so a file held open by a running editor or agent (Cursor's `state.vscdb`,
Electron LevelDB stores) still copies. A file that a running program opened
without sharing, or with a byte range locked, cannot be read and gets an
`error_copy` row with Windows's reason. On live hosts this has been the
case for the Chromium cookie store `Network\Cookies` and its
`Cookies-journal`, and the `LOCK` files of the `Local Storage\leveldb` and
`Session Storage` LevelDB stores (LevelDB is the key-value store Chromium
keeps them in), in the profiles of Electron agent apps such as Claude
Desktop, Antigravity and the ChatGPT desktop app; the ChatGPT app also held
its package's settings hive `Settings\settings.dat` with its `.LOG1` and
`.LOG2` logs, and the bundled Codex its `.codex\.sqlite-maintenance.lock`
and `.codex\thread-writer-locks\<uuid>.lock` files. The lock files are
empty, so what is lost is the cookie store (the agent's session cookies,
flagged secret) and the settings hive; close the app, or collect from an
image, to get them.

A file's size, times and attributes come from its entry in its directory's
listing, and its identity from the file itself. NTFS updates a directory
entry later than the file: the size it lists changes only when a handle on
the file closes, so a file a running program holds open and keeps writing
can be listed with fewer bytes than it holds. A `collected` row's `size` is
the number of bytes copied, so it matches the `sha256` and the archive
member. A `skipped_size` decision, and the `size` of every other row, use
the listed size. A directory that just gained a child, or a file written
through another hard link, can show the older values for a while too.

Every file system call addresses its path in the extended-length form,
`\\?\C:\...` for a drive path and `\\?\UNC\server\share\...` for a path on
a network share (a `-r` on a share, for example), so paths longer than 260
characters are listed, copied and hashed without `LongPathsEnabled` set on
the host. This covers the staging directory too, whose paths are longer
than the source's by the output directory, the `.stage-<name>` directory
and `fs\<drive>`. The prefix never reaches the output: the manifest, the
log, `collection.json` and stdout hold the plain form, and an error quoted
in a row has the prefix removed.

## Case-sensitive directories and short names

Windows can mark a single directory case-sensitive, and WSL (the Windows
Subsystem for Linux) marks the directories it creates. Inside one, `Alice`
and `alice` are two entries, and a catalog line or a path given on the
command line matches only its exact spelling. The run treats two spellings
of a path as one entry only when the file system gives both the same
identity, or neither exists, so it keeps two such homes, files or projects
apart, and it marks its own staging directory case-sensitive so both are
archived. Elsewhere letters' case is ignored, as Windows does: in catalog
matches, Docker volume names, `-u` names and registry value names.

A volume that makes 8.3 short names (the `CLAUDE~1` form older programs
use) takes a name shaped like one for the directory it names, marked
case-sensitive or not. When the evidence holds a file whose name the
output directory takes for another's, by letter case or by short name, the
second is not staged
([Case and Unicode form in the output directory](collect-options.md#case-and-unicode-form-in-the-output-directory)).

## Manifest fields on Windows

A row written on Windows has two fields more than one written on Linux or
macOS, after `mode`:

| Field | On Windows |
| --- | --- |
| `owner` | The account owning the path itself, `DOMAIN\name` (a local account's domain is the computer name), the security identifier when no account matches it, or `""` when it cannot be read. |
| `attributes` | The attribute names, such as `Archive`, `Hidden, System, Archive` or `Directory, ReparsePoint`. |
| `uid`, `gid`, `mode` | Always `0`, `0` and `""`: Windows has no numeric owners or permission bits. |
| `ctime` | Always `0`: Windows keeps no change time. `btime` is the creation time. |
| `path`, `home`, `target` | In Windows form, with backslashes. `archive_path` uses `/` and starts `fs/<drive letter>/` on a live host. |

[Output](collection-output.md#manifest) describes every field.

## Access times

Whether reading a file updates its last-access time is a system-wide
setting on Windows. By default a run asks Windows to leave each file's
access time as it was for the run's own read, and logs any file where that
did not hold; `--no-preserve-atime` turns this off.
[Access times](deployment.md#access-times) has the details.

## The registry step

On a live Windows host the relocation step also reads the registry, because
Windows keeps a user's and the machine's environment variables there rather
than in a file ([The registry](relocation.md#the-registry)). It reads only
values whose names are relocation variables, in two places:

- The system environment key,
  `HKLM\SYSTEM\CurrentControlSet\Control\Session Manager\Environment`.
- The `Environment` key of the loaded `HKEY_USERS` hive of each collected
  user whose profile has a security identifier starting `S-1-5-21-` or
  `S-1-12-`. `-u` limits which users these are.

A value is read without running anything. A `REG_SZ` value is kept as
written; a `REG_EXPAND_SZ` value has its `%NAME%` references replaced from
the same key, the user's profile paths and the system key, never from the
environment of the process running the collection, which belongs to the
account that started it. A plain drive path, one with no leftover reference
or stray `%`, double quote, control character or `..` segment, is kept as
the hint's `value`.

The step never loads a hive: loading one with `reg load` or a profile
loader writes to the user's `NTUSER.DAT`, which an evidence collection must
not do. A user whose hive is not loaded is listed in
`relocation.registry.users_not_loaded` and in the log. On a default system
only Administrators can open another user's loaded hive: a hive the account
cannot open is listed in `users_denied` with the message Windows gave, and
a run as Administrator reads every loaded hive. The step does not run in
image mode or on Linux and macOS; `relocation.registry.reason` says which.

## Image mode on Windows

Every `-r` is image mode on Windows, a drive root such as `-r C:\`
included: the run then reads no registry and looks for profiles under the
root's `Users` directory and the Unix home parents, not in the registry's
profile list. In image mode every file and directory the run opens is
checked, after it is opened, to lie under the image root; one that Windows
opened outside it (through a link the path check could not see) is logged
with `the root check allowed it but Windows opened it outside the image
root; nothing from it was used` and is not used.
