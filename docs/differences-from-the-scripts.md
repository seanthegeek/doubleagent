# Differences from the scripts

DoubleAgent used to be three programs: a POSIX sh script that collected
on Linux, macOS and BSD (`collect-agent-artifacts.sh`, called the sh
collector below), a Windows PowerShell script that collected on Windows
(`Collect-AgentArtifacts.ps1`, the PowerShell collector), and a Python
package that analysed what they collected (the Python analyzer, with the
commands `detect`, `timeline`, `inventory` and `catalog`). The two
collectors are called the scripts. The single `doubleagent` program
replaces all three: `doubleagent collect` replaces the scripts, and
`doubleagent detect`, `timeline`, `fleet` and `catalog` replace the
Python analyzer.

Where `doubleagent` deliberately does something the earlier programs did
not, the difference has a number. Read this page when you used the
scripts or the Python analyzer and want to know what changed, or when a
comment in the source names one of these numbers. Differences of
`collect` are called exceptions and are numbered from 1; differences of
the analysis commands are called differences and are numbered from 101.
A number is never reused or given a new meaning; a number that was taken
back is listed as withdrawn, and a number this page does not list was
never given out. Where a number's behaviour is the same as one of the
scripts on its own platform, the entry says so: the number keeps its
place so that a reference to it still finds its meaning.

A few words are used throughout. A live run collects from the machine
`doubleagent collect` runs on; a run in image mode is given `-r DIR`, the
root of a mounted disk image, and collects from that image. The staging
directory is where a run copies files before it packs them into the
archive. The manifest is `manifest.jsonl`, one row per file or directory
the run met, with a `status` such as `collected`, `error_copy` or
`skipped_excluded`. A home is a user's home directory, and a project is a
working directory an agent's state names, whose project-level files are
collected too. A symlink is a symbolic link; on Windows a junction is a
second kind of directory link, and both are reparse points, the Windows
mechanism behind every kind of file system link.

## Index

Exceptions of `collect`:
[1](#exception-1), [2](#exception-2), [3](#exception-3),
[4](#exception-4), [5](#exception-5), [6](#exception-6),
[7](#exception-7), [8](#exception-8), [9](#exception-9),
[10](#exception-10), [11](#exception-11), [12](#exception-12),
[13](#exception-13), [14](#exception-14), [15](#exception-15),
[16](#exception-16), [17](#exception-17), [18](#exception-18),
[19](#exception-19), [20](#exception-20), [21](#exception-21),
[22](#exception-22) (withdrawn), [23](#exception-23), [24](#exception-24),
[25](#exception-25), [26](#exception-26), [27](#exception-27),
[28](#exception-28), [29](#exception-29), [30](#exception-30),
[31](#exception-31), [32](#exception-32), [33](#exception-33),
[34](#exception-34), [35](#exception-35), [36](#exception-36),
[37](#exception-37), [38](#exception-38), [39](#exception-39),
[40](#exception-40), [41](#exception-41), [42](#exception-42),
[43](#exception-43), [44](#exception-44), [45](#exception-45),
[46](#exception-46), [47](#exception-47), [48](#exception-48),
[49](#exception-49), [50](#exception-50), [51](#exception-51),
[52](#exception-52), [53](#exception-53), [54](#exception-54),
[55](#exception-55), [56](#exception-56), [57](#exception-57),
[58](#exception-58), [59](#exception-59), [60](#exception-60),
[61](#exception-61), [62](#exception-62), [63](#exception-63),
[64](#exception-64).

Differences of the analysis commands:
[101](#difference-101), [102](#difference-102), [103](#difference-103),
[104](#difference-104) (withdrawn), [105](#difference-105), [106](#difference-106),
[107](#difference-107), [108](#difference-108), [109](#difference-109),
[110](#difference-110), [111](#difference-111), [112](#difference-112),
[113](#difference-113), [114](#difference-114), [115](#difference-115),
[116](#difference-116), [117](#difference-117), [118](#difference-118),
[119](#difference-119), [120](#difference-120), [121](#difference-121),
[122](#difference-122), [130](#difference-130) (withdrawn),
[140](#difference-140), [150](#difference-150), [160](#difference-160),
[170](#difference-170), [210](#difference-210) (withdrawn),
[230](#difference-230), [231](#difference-231), [232](#difference-232),
[240](#difference-240), [241](#difference-241), [250](#difference-250),
[260](#difference-260), [261](#difference-261), [262](#difference-262),
[263](#difference-263), [264](#difference-264), [265](#difference-265),
[266](#difference-266), [267](#difference-267), [268](#difference-268),
[269](#difference-269), [270](#difference-270).

## Running a collection

### Exception 54

The command line is read by Go's `flag` package, with the sh collector's
option names on every platform. Besides the spellings the sh collector
took, `doubleagent collect` accepts a long option with one dash
(`-output DIR`), a value after `=` (`--output=DIR`, `-o=DIR`), a switch
given `=true` or `=false` (`--quiet=false`), `-help`, and `--` to end the
options; the sh collector refused each of these as an unknown option. A
mistake anywhere on the line is a usage error even after `--help`,
`--version` or `--list`, which the sh collector obeyed where they stood.
A usage error is one line, `doubleagent collect: ` and the problem (such
as `flag provided but not defined: -bogus` or `flag needs an argument: -o`),
then the usage text, where the sh collector printed `Unknown option: X` or
the usage alone. Exit codes and the size-option messages are unchanged.

### Exception 56

On Windows `doubleagent collect` takes the same options as on Linux and
macOS (`-o`, `--output`, `-r`, `--root`, `--inventory` and the rest), not
the PowerShell collector's parameters (`-OutputDir`, `-Root`,
`-Inventory`, `-MaxFileSizeMB` and the others). Messages name the options
that way: a bad size value is `Invalid --max-file-size: <value>` or
`Invalid --env-scan-max-size: <value>` where the PowerShell collector
wrote `Invalid -MaxFileSizeMB` or `Invalid -EnvScanMaxSizeMB`, and the
note that `-o` is ignored under `--inventory` names `--inventory` and
`-o`. `-h` and `--help` work on Windows too, and `-p` is repeated for each
project instead of taking a PowerShell array. One program has one command
line, whatever the platform.

### Exception 58

`doubleagent collect` runs no host program except `getent` and `dscl`
(for the list of users) and `plutil` (for a launchd property list): it
copies, hashes, archives and searches files itself, so a missing `tar`,
`find` or hash program is no longer a reason to stop. It reads none of the
scripts' environment variables (`COLLECTOR_SH`, `CAC_NO_BATCH`,
`CAC_ARCHIVER`). It does not change its own `PATH`, as the sh collector
did; it looks for those three programs in `/usr/bin`, `/bin`, `/usr/sbin`,
`/sbin` and `/usr/local/bin` first, then along `PATH`, skipping a relative
`PATH` entry, and runs them with `LC_ALL=C`. On Windows it runs none of
them.

### Exception 59

`doubleagent` is built for Linux, macOS and Windows only, so a FreeBSD or
OpenBSD host cannot be collected live, as the sh collector could. An image
of such a host can still be collected from a Linux or macOS machine with
`-r`: in image mode the homes come from the image's own `etc/passwd` and
the usual home parents, `usr/home` among them.

### Exception 62

A startup file that begins with a UTF-16 or UTF-32 byte-order mark is
decoded before the relocation search looks for variable names in it, on
every platform; any other file is searched as bytes. The sh collector's
`grep` searched every file as bytes, so it found no name in such a file;
the PowerShell collector's reader decoded it, as `doubleagent` does.

### Exception 23

An interrupt (`INT` or `TERM` on Linux and macOS, Ctrl+C on Windows) logs
`interrupted`, removes the staging directory unless `-k` was given, and
exits 130 on every platform. The PowerShell collector had no handler for
it. `doubleagent` also stops a host program it is running at that moment
and starts no other.

### Exception 13

A home whose path holds `:` is collected under its full path. The sh
collector passed homes around as `user:home` lines, so such a home was
split at the colon and lost.

### Exception 14

When `getent passwd` fails on a live host, `doubleagent` reads
`/etc/passwd` instead. The sh collector kept the empty output of the
failed `getent` and found no home from the password database.

### Exception 6

The rows of one walk are written as a group when the walk ends. The sh
collector wrote them in batches, one each time `find -exec` filled its
argument list, so on a very large tree the rows of two walks could be
interleaved differently. The rows and their content are the same; only
their order on such a tree can differ.

### Exception 9

A file is hashed while it is copied, in the same read, rather than by
reading the staged copy again afterwards. The hash covers the same bytes.

### Exception 12

A directory the run can list but not enter keeps its `error_read` row (and,
when it cannot even be listed, a `find:` line in the log), and its
entries get no row, no log line and no inventory record, on every
platform. Under GNU `find` the sh collector wrote some of those entries as
`error_copy` rows; under BSD `find` it wrote none. Nothing in such a
directory can be read, so neither form is evidence.

### Exception 15

A project given with `-p`, or discovered, whose own path runs through a
symlinked directory inside a home is collected where its files physically
are: each file's `path` is the path the project was named by, and its
`archive_path` the place it really is, the rule the home walk already
follows for catalog matches. The project's log line ends
`(through the symlinked directory LINK, archived under PATH)`. The sh
collector staged the files under the recorded path: through a link it had
recreated in the staging directory, where the copy failed with
`error_copy` rows and `copy failed:` lines, or, where no link was
recreated, at the recorded path itself.

### Exception 20

A `*` in a catalog line never matches a name that begins with a dot, on
every platform, because one glob matcher reads the catalog everywhere. The
PowerShell collector turned the globs into regular expressions, whose `*`
also matched a leading dot.

### Exception 43

`doubleagent collect` has the option `--no-preserve-atime`, which neither
script has, and on Windows it decides how to open each file it reads. It
reads `NtfsDisableLastAccessUpdate` once per run: where Windows updates
access times, or the value cannot be read or understood, each file is
opened with the right to change its attributes and the copy asks Windows
to keep the file's access time, falling back to a read-only open when that
open is refused; where Windows does not update them, or the option is
given, files are opened read-only. One log line after the start lines
says which case applied (Windows only). The option is recorded on every
platform as `options.no_preserve_atime` in `collection.json` and
` no_preserve_atime=0` or `=1` at the end of the log's second start line.
On Linux and macOS it changes nothing. The scripts read every file
read-only and could move its access time.

### Exception 48

Two spellings of a path are treated as one only when the file system says
they are one. Wherever the scripts compared paths with letter case folded
(homes, the files a catalog line claims, the paths already given a row,
projects and their owners, the paths the relocation search leaves out),
`doubleagent` treats two paths as one when their bytes are equal, or when
they differ only in ASCII letter case and the file system gives both the
same identity, or neither exists. So in an NTFS directory marked
case-sensitive, `Users/Alice` and `Users/alice` are two homes with two
walks, where the PowerShell collector merged them and lost one. On Linux
and macOS paths are compared as bytes, as before.

### Exception 49

Nothing in the staging directory is ever written over, and a row names the
archive member that holds its file. An output directory can take two
names for one entry: names differing in letter case on the default macOS
volume, exFAT, FAT or a Windows volume whose staging directory cannot be
marked case-sensitive; names differing in Unicode form on every macOS
volume; and Windows short names such as `CLAUDE~1`. `doubleagent` creates
every staged copy and link so that it fails if something is already
there. When it fails, the same source reached a second time shares the
first copy's member and hash; another file is refused, with an
`error_copy` row and the error
`stage <path>: the output directory takes its name for another file's, link's or directory's; not staged`
(for a link, `not recreated in the archive:` and that reason); and a file
replaced since the run staged it gets
`changed since this run staged it; the earlier row holds its copy`. On
Windows the run marks its new staging directory case-sensitive, so names
differing only in case are both kept. A log line,
`NOTE: output directory DIR ignores letter case or Unicode form in names; ...`,
says when the run finds that the output directory merges names. The
scripts overwrote the first copy without a word, which gave wrong hashes,
rows naming no member and members with no row.

### Exception 50

A symlink that cannot be recreated in the staging directory is recorded on
every platform: its row has the error
`not recreated in the archive: <reason>` and no archive path, and the log
has `symlink not recreated: <path>: <reason>`. When a link's target cannot
be read, no link is made. The sh collector ignored a failed `ln -s`,
leaving a row whose archive path named no member, and made a link with an
empty target when the target could not be read.

### Exception 64

A run whose standard output or standard error is closed while it runs (a
pipe whose reader ended, a remote console that detached) finishes: it
drops what it cannot print, writes the archive, removes the staging
directory and exits `0`, and the log file holds every line. On Linux and
macOS `doubleagent` receives the `SIGPIPE` signal so that the write fails
instead of ending the program. A `--inventory` run, whose standard output
is its only output, exits `2`. The sh collector set no handler for
`SIGPIPE` (its traps covered `EXIT`, `INT` and `TERM`), so unless it was
started with the signal ignored, its next write to the closed stream, a
progress line on standard error without `-q` among them, ended it by that
signal, with no `interrupted` line.

## The manifest, the log and collection.json

### Exception 1

The `error` field of a row holds Go's description of the failure, such as
`open /home/carol/.claude/x: permission denied`. The sh collector wrote
`cp`'s or `ls`'s message, which already had two shapes (GNU and BSD).
Nothing reads the field to make a decision, so its wording is free to
follow the program that writes it.

### Exception 2

A control character in a file name is written to the manifest as a JSON
escape, so every line is valid JSON. The sh collector passed such
characters through as raw bytes, which made the line invalid JSON; the
PowerShell collector escaped them already.

### Exception 3

`collection.json` and the log name the tool as `doubleagent` with the
project's single version, where the scripts named `collect-agent-artifacts`
and their own version. `--version` prints `doubleagent <version>`, and the
inventory's `collector` field holds that version. A build that no release
stamped, such as one made from a checkout, reports `0.0.0-dev`, with `+`
and the first 12 hex digits of its commit (and `-modified` for a tree with
uncommitted changes) when the build recorded it.

### Exception 4

`collection.json` keeps its `capabilities` object, but its values describe
`doubleagent`: `hash_tool` is `crypto/sha256`, `archiver` is `archive/tar`,
`grep_text` is always `true` (the search reads bytes, so a name after a
NUL byte is always found), and `plutil` says whether `plutil` was found.
`stat_mode` and `worker_shell` describe programs `doubleagent` does not
use and are gone. In the log's second start line `hash=` names
`crypto/sha256`, and the sh collector's `stat=` and `worker_sh=` words are
gone.

### Exception 5

The archive is written by Go's `archive/tar`: members in sorted order, in
the PAX format, compressed with gzip. There are no fallbacks to other
archivers, so its bytes, size and hash differ from the scripts' archive
for the same files.

### Exception 7

The size of an excluded directory is worked out by `doubleagent` itself.
On Linux and macOS it is what `du -sk` reports, the blocks the files have
allocated, as the sh collector reported by running `du`; on Windows it is
the sum of the files' sizes, as the PowerShell collector reported it. The
two can differ for the same directory on a compressed or sparse file
system. This is each script's behaviour on its own platform.

### Exception 8

The `mode` field is the permission bits with the setuid, setgid and
sticky bits (`mode & 07777`) on every platform. On macOS the sh collector
read the mode with BSD `stat`, which left those three bits out.

### Exception 10

Every line of the log carries a time. The sh collector let `find`'s and
`mkdir`'s own error output into its log without a time; `doubleagent` has
no such programs. A directory that cannot be listed gets
`find: <dir>: <reason>` with a time, without the quotes GNU `find` puts
around the path.

### Exception 11

`relocation.registry.reason` in `collection.json` is `not Windows` on a
live run outside Windows and `image mode` under `-r`, as the PowerShell
collector says it. The sh collector wrote `sh collector`, the name of a
program that no longer exists.

### Exception 17

On Windows `collection.json` `platform` is `Windows`, the version
(major, minor and build) and the processor architecture. The PowerShell
collector added ` PowerShell <version>`.

### Exception 18

`collection.json` is written by Go's `encoding/json` on every platform, so
on Windows it has the same layout as on Linux and macOS. The PowerShell
collector wrote the layout of `ConvertTo-Json`, whose Windows PowerShell
5.1 version also escaped `<`, `>`, `&` and `'`. The values are the same.

### Exception 19

A relocation hint found in a startup file carries the value the file
gives, where a reader for the file exists (`/etc/environment`,
`environment.d` files, launchd property lists). The PowerShell collector
resolved values from the registry only and wrote none for a file. On a
Windows image none of those files is a startup file, so the difference
shows only in an image that holds both kinds.

### Exception 25

The log's two start lines have the same shape on Windows as on Linux and
macOS: the first names the tool, its version, the host, the system, the
mode, the root and the account, the second the hash and the options. The
PowerShell collector's `discovery source unreadable:` and `stat failed:`
lines are not written.

### Exception 27

A time before 1970 is written as `0`, the sh collector's rule, on every
platform. The PowerShell collector wrote a negative number.

### Exception 29

On Windows the `error` field holds Go's description of the failure (such
as `open C:\p: Access is denied.`), not the .NET exception message the
PowerShell collector wrote. This is [exception 1](#exception-1) on
Windows.

### Exception 32

A failure to set a staged copy's times after the copy makes the row
`error_copy` on every platform. The PowerShell collector ignored it.

### Exception 33

The directories above a skipped file are created in the staging
directory, as the sh collector created them, so an empty directory can be
an archive member. The PowerShell collector created a directory only when
it copied a file into it.

### Exception 36

A directory that cannot be listed gets the line `find: <dir>: <reason>` in
the log on Windows too: one rule on every platform, the sh collector's.
The PowerShell collector wrote no such line.

### Exception 42

A `-u` name that matches no home is logged as
`WARNING: -u NAME matches no user home; homes found for: A, B` (or `none`),
and an empty `-u` list as `WARNING: -u lists no names`, on every platform.
Neither script said anything, so a misspelt name silently collected
nothing.

### Exception 51

A file name that is not valid UTF-8 is written to the manifest with the
character U+FFFD in place of each byte that is not part of a valid
character, in `path` and `archive_path`; the archive member keeps the
name's real bytes. The sh collector wrote the raw bytes, which made the
line invalid JSON. On Windows, a name holding a lone surrogate (a half of a
UTF-16 character pair, which Windows allows in a name) is written with
three U+FFFD characters, one per byte of its stored form, where the
PowerShell collector wrote one.

### Exception 52

`collection.json` is indented as `encoding/json` indents it: two spaces,
with every object member and list item on a line of its own. The sh
collector kept small objects and lists on one line. The keys, their order
and the values are the same.

### Exception 53

The characters U+2028 and U+2029 (the Unicode line and paragraph
separators) are written as the escapes `\u2028` and `\u2029` in the
manifest, `collection.json` and the inventory lines. The sh collector
wrote the raw characters.

## Windows

### Exception 21

A directory that a wildcard of a catalog line cannot list gets no
`error_read` row. The PowerShell collector wrote one.

### Exception 22

Withdrawn. It said that in image mode the homes would be listed in the
sh collector's order; `doubleagent` instead lists them in the order the
collector of the platform it runs on used, so there is no difference.

### Exception 24

The archive is always `.tar.gz`, written by Go's `archive/tar` with no
`tar.exe`, no PowerShell writer and no `.zip` fallback, so symlink rows
are never rewritten after archiving and the archiver's fallback notes are
not logged. The PowerShell collector's `tar.exe` was also seen to store one
link without its target; `doubleagent` stores every link's real target.

### Exception 26

The owner of a symlink is the link's own owner. .NET, under the
PowerShell collector, reported the owner of the link's target.

### Exception 28

The size of an excluded directory is the sum of the sizes of its files,
without going through any reparse point. Under Windows PowerShell 5.1,
`Get-ChildItem -Recurse` could descend into junctions and count what they
lead to.

### Exception 30

A home in the ProfileList registry key that cannot be seen still gets its
`error_read` row and `profile not accessible` line, as in the PowerShell
collector. The PowerShell collector's other log wordings
(`no user profile directories found`, `-Project is not a directory`) are
written in `doubleagent`'s words.

### Exception 31

When project paths are read from an agent's files, drive-letter paths are
also accepted as JSON keys and strings (`"C:\\proj"`). Narrowed: the
PowerShell collector's readers interpreted some escape sequences
differently, and those differences are not reproduced.

### Exception 34

A lone surrogate in a file name (a half of a UTF-16 character pair, which
Windows allows in a name) is kept as the bytes that encode it, so the name
can be found and archived. The PowerShell collector replaced it with
U+FFFD. [Exception 51](#exception-51) says how the manifest writes such a
name.

### Exception 35

On Windows a file's times, size and attributes come from its directory
entry, as `Get-ChildItem` read them; the file's identity comes from the
file itself. NTFS updates a directory entry later than the file, so a
directory that just gained a child, or a file written through another
hard link, can show the older values for up to an hour. This keeps the
PowerShell collector's behaviour.

### Exception 37

A ProfileList path that begins with a NUL character is skipped and the
remaining profiles are still read. In the PowerShell collector the empty
path made `Split-Path` fail inside a `try` that ended the whole ProfileList
loop, losing every later profile.

### Exception 38

Names are compared with ASCII letters folded only: registry value names
against the relocation variables, and the profile names that are skipped.
The PowerShell collector's `-eq` and `-contains` folded by the rules of
the culture, so a name differing only in a non-ASCII letter (`ſ` for `S`)
matched there. The cap of 32,767 on a registry value's expansion counts
UTF-8 bytes, where the PowerShell collector counted UTF-16 units, so
non-ASCII text stops expanding sooner. The `-u` list is the one place that
ignores the case of every letter on Windows, since a wrong match there
only adds a home and never drops evidence; on Linux and macOS user names
are compared exactly.

### Exception 39

A registry value that is listed but cannot be read (a name holding a NUL,
which the listing cuts) is recorded with an empty value and the reason
`unreadable`, and the rest of the key is still read. In the PowerShell
collector `GetValueKind` failed, the whole key was given up and the user
was listed as denied, so one planted value could hide every relocation
hint of a registry hive.

### Exception 40

In a directory marked case-sensitive (a per-directory NTFS setting), a
catalog line or a path given on the command line matches only its exact
spelling, as on Linux. The PowerShell collector's lookups could match
another spelling there.

### Exception 41

Two paths that both come from the file system are compared byte for byte.
That relies on the file system returning names as they are stored, which
local NTFS does; on a network share or a file system that does not, a link
inside a home whose target differs from the stored name in letter case is
treated as leading out of the home and is not followed.

### Exception 44

The log lines `access time not held: <path>: <reason>` and
`access times not held: <N> file(s)` are written on Windows when a file
was read without the access-time hold the run wanted
([exception 43](#exception-43)), or the hold misread the time and the
time was put back. Neither script has them.

### Exception 55

Tar's error output is never appended to the log through the shell, so an
output directory whose name holds a wildcard character such as `[` causes
no error. The PowerShell collector appended `tar.exe`'s errors to the log
with `2>>`, which treats the log's path as a wildcard pattern, and so
printed an error record on stderr for an output directory such as
`out [1]`. That was a defect of the PowerShell collector, which
`doubleagent` does not reproduce.

### Exception 57

With `--inventory`, the homes are walked last, after the other steps, as
the sh collector walked them, on Windows too, so the log lines come in that
order. The PowerShell collector walked the homes before the Docker volumes
and project discovery.

### Exception 63

With `--inventory` on Windows, the host line's `users_scanned` counts only
the homes found, and `users_unreadable` only those of them that cannot be
listed and entered. A ProfileList entry logged as
`profile not accessible (skipped)` is counted in neither, where the
PowerShell collector counted it in both. On Linux and macOS the counts are
unchanged.

## Image mode

### Exception 16

In image mode a project whose own path leads out of the image root is not
walked, and the relocation search does not search it: following its links
would read the analyst's own machine. The decision is made one path
component at a time from the root, following each link by its own target
without touching anything outside the image; a path that cannot be shown to
stay inside counts as outside; and it is made before the directory test,
so the result never depends on the analyst's machine. Such a project gets
the log line
`Project [USER] P: under the symlinked directory X, which leads out of the image root; not walked`
and no rows, and is left out of `collection.json` `projects`, the stdout
project count and the inventory's per-user project count. A `-p` path not
under the root is logged `Project P: is outside the image root; not walked`
with no row, where the sh collector looked for it on the analyst's machine
and, finding nothing, wrote a warning and an `error_read` row. A path the
image itself cannot read keeps that warning and row. For a project behind
a link out of the root, the sh collector copied through the recreated link
and failed.

### Exception 45

Project discovery reads no source behind a link that leads out of the
image root. Before it opens a source, tests one, or searches below a
starting directory, `doubleagent` follows the links on its path from the
root and reads nothing past one that leads out. Both scripts read through
such a link, which in image mode read the analyst's own machine. A live
run is unchanged. [Exception 46](#exception-46) widened this rule and
replaced its log line.

### Exception 46

In image mode nothing behind a link that leads out of the image root is
listed, looked up, stat'ed or logged, in any step: the catalog walk,
project discovery, user enumeration, Docker volumes and the relocation
startup files. Each step logs the link once:
`[AGENT] LINK: the symlinked directory leads out of the image root; nothing under it was listed, not walked`
in the catalog walk, which also records the link as one `symlink` row of
that agent unless a match claims it;
`Discovery [USER] LINK: the symlink leads out of the image root; nothing behind it was listed or read`;
`user homes: LINK leads out of the image root; nothing behind it was listed or taken as a home`;
`docker: LINK leads out of the image root; nothing behind it was listed`;
and `startup files: LINK leads out of the image root; nothing behind it was listed`.
Both scripts listed the analyst's directory there. Related rules: a
candidate home that is such a link is no home; a Docker volume's `_data`
that is a link is skipped (`volume V has no _data directory, skipped`)
before anything is read through it, in live mode too; an excluded path
that is such a link gets a `skipped_excluded` row from the link itself; a
reparse point other than a symlink or junction is refused with its own
wording. The check judges the path the system will open, the way that
system resolves it, so the check and the open cannot name different files;
on Windows in image mode every open and listing is checked again after the
open, and a path the system opened outside the root is logged
(`...: the root check allowed it but Windows opened it outside the image root; nothing from it was used`)
and not used. Neither script had such a check.

### Exception 47

A path that comes in holding a `.` or `..` component (a home from the
password database, a discovered project, a `-p` value and, on Windows, a
ProfileList path) is replaced, before anything else uses it, by the path
the operating system reaches: on Linux and macOS the physical path, spelled
under the root as given; on Windows the path with `.` and `..` removed by
name. The row, archive path, log lines and `collection.json` all use that
path, and staging refuses a path that still holds such a component, with
an `error_copy` row and the error
`holds a . or .. component, so where it is staged is not known`. Two homes
or projects that become one path are one, and for two homes a log line
names both users: `user homes: P is the home of A and of B; walked once, as A's`.
The sh collector kept the dotted spelling and staged the copy at
`fs/<dotted path>`, which `mkdir -p` resolved by name: a home
`/srv/dotlink/../frank`, with `srv/dotlink` a link elsewhere, was staged
at `fs/srv/frank`, on top of another user's home of that name, which loses
evidence.

### Exception 60

A project reference that is a drive-letter path (`C:\proj`, `c:/proj`) is
taken as a project only by a run on Windows. A run on Linux or macOS, such
as one over a Windows image mounted there, reads only references that
start with `/`, and, in the sources whose references are URLs, a
`file:///C:/` or `file:///C%3A/` URL, whose path below the drive it maps
under the root. The PowerShell collector, run
under PowerShell 7 on Linux or macOS over a Windows image, mapped every
drive-letter reference. Collect a Windows image from a Windows machine to
have them all.

### Exception 61

Catalog, exclusion and secret globs match ignoring ASCII letter case only
when the run is on Windows; on Linux and macOS they match byte for byte,
whatever the image holds. The PowerShell collector ignored case wherever
it ran, under PowerShell 7 on Linux and macOS too, so a Windows image
mounted there and collected by `doubleagent` matches a path only in the
letter case the catalog gives. Collect a Windows image from a Windows
machine for its matching.

## Analysing: commands and options

### Difference 101

The usage and help text of each command are `doubleagent`'s own, written
for its options. (The comparison with the Python analyzer needed no rule
for this number.)

### Difference 121

Options come before the input: `doubleagent timeline [options] INPUT OUTPUT_DIR`,
`doubleagent detect [options] INPUT`. The Python analyzer also took
options after the input, and `timeline` took its output directory as
`-o DIR`; `doubleagent timeline` takes it as its second argument and has no
`-o`.

### Difference 261

The Python analyzer's `inventory` command, which merged saved
`--inventory` outputs into one CSV, is `doubleagent fleet`. Its input and
output (`-o CSV`, default `fleet-inventory.csv`) are the same.

### Difference 262

`timeline` refuses an output directory that is not empty, that is not a
directory (a symlink counts as not a directory), or that lies inside the
input, before it reads the input, and exits 2. The Python analyzer created the directory if needed
and wrote its files over whatever was there, inside the input too.

### Difference 263

`fleet` writes its CSV to a new file beside it, named
`doubleagent-fleet-*.partial`, then moves it into place, so an existing
CSV is replaced whole and never written into. It refuses a CSV path that
is a symlink, one of the input files, or inside an input directory. The
Python analyzer opened the path and wrote into it, following a symlink.

### Difference 264

A field of the `fleet` CSV that begins with `=`, `+`, `-`, `@`, a tab or a
carriage return is written with a `'` in front, so a spreadsheet shows it
as text instead of running it as a formula. The rollup printed to stdout
keeps the value as written. The Python analyzer wrote every field as it
was.

### Difference 265

By default `detect` and `timeline` show secrets (tokens, keys, passwords,
cookies, the credentials of a URL) as the evidence holds them, since they
help show what an agent could reach. `--no-secrets` replaces each secret
it identifies with one placeholder, `REDACTED` unless
`--redaction-placeholder` names another. The Python analyzer had neither
option: it always withheld the secrets of certain fields, each field in
its own form.
[Difference 230](#difference-230) says how the withheld forms compare.

### Difference 266

An archive given as the input is extracted into a new directory named
`doubleagent-` and a random suffix, under `--work-dir` or the system's
temporary directory. The Python analyzer named it `cac-` and a suffix
under `--work-dir`, or `cac-analyzer-` and a suffix in the temporary
directory.

### Difference 103

`--match` takes a Go regular expression, not a Python one. Go's syntax has
no lookahead, lookbehind or backreferences, and a pattern Go cannot
compile is a usage error. A pattern that both read alike matches the same
rows.

### Difference 104

Withdrawn. It said that `--since` and `--until` do not read ISO week dates
(`2026-W40`) or ordinal dates (`2026-280`) as times. They read both, as the
Python analyzer did, and neither reads digits other than ASCII `0` to `9`,
so there is no difference.

### Difference 140

When `--since` or `--until` cannot be read, the error lists the accepted
forms, and the epoch form is named `epoch seconds or milliseconds`, both
of which are read. The Python analyzer's list said `epoch seconds`.

### Difference 107

A loose tree (a directory with no collection manifest) is read in the
order of the names' bytes, not in the order the file system lists them, so
two runs over the same tree give the same output on every machine. When
two files hold the same record, which of the two equal rows the timeline
keeps can therefore differ from the Python analyzer, which took the file
system's order.

### Difference 267

`timeline --agent NAME` with a name no parser reads is an error,
`error: --agent NAME: no parser reads this agent`, and the run exits 2
before it reads the input. The Python analyzer took any name and parsed
nothing for one no parser reads.

### Difference 268

An archive input that is an xz- or zstd-compressed tar is refused
(`xz-compressed tar archives are not supported`); a tar, plain or
compressed with gzip or bzip2, or a zip is read. The Python analyzer
opened tars with Python's `tarfile`, which also reads xz. A tar member
whose name starts with slashes is extracted with them dropped, and a
member that would lead out of the extraction directory is skipped with a
problem line while the rest is extracted; the Python analyzer's
extraction filter refused such a member by stopping the whole extraction.

### Difference 269

An interrupt (`INT` or `TERM` on Linux and macOS, Ctrl+C on Windows) stops
`detect`, `timeline` and `fleet` with `interrupted` on standard error and
exit code 130, and an extracted archive is removed even under
`--keep-extracted`, so no half-read extraction is left behind. The Python
analyzer stopped with Python's traceback and kept the extraction under
`--keep-extracted`.

### Difference 270

`doubleagent` with no command prints its usage, and with a command it does
not know prints `Unknown command: NAME` and its usage; both exit 1, the usage exit code of every `doubleagent` command
line. The Python analyzer exited 2 for these, as Python's `argparse` does.

## Analysing: output files

### Difference 108

The JSON outputs (`detect --json`, `inventory.json` and the like) are
written by Go's `encoding/json`. The members, their order and the values
are the same; spacing, escaping and the final newline follow
`encoding/json`.

### Difference 109

A lone surrogate (a half of a UTF-16 character pair) in text becomes
U+FFFD, as Go's standard library writes it. (The comparison with the
Python analyzer needed no rule for this number.)

### Difference 110

The `fleet` CSV is written by Go's `encoding/csv` with its default
settings, so its quoting follows that package and each record ends with
LF, where the Python analyzer's `csv` module ended each with CR LF. The
records are the same.

### Difference 111

Lengths and padding in the tables printed to stdout count characters
(Unicode code points), so a column keeps its width with non-ASCII text.
(The comparison with the Python analyzer needed no rule for this number.)

### Difference 112

A path is printed as it was given. (The comparison with the Python
analyzer needed no rule for this number.)

### Difference 113

A boolean is written `true` or `false`, in a row's text and its formatted
fields alike. The Python analyzer wrote some as `True` and `False`.

### Difference 114

A null value is written as nothing. (The comparison with the Python
analyzer needed no rule for this number.)

### Difference 115

A number taken from the evidence keeps its text as the evidence wrote it.
(The comparison with the Python analyzer needed no rule for this number.)

### Difference 116

JSON embedded in a row's text is compact (no spaces), with each object's
keys in the order the evidence gives them. The Python analyzer's spacing
and key order could differ.

### Difference 117

A list or object in a row's text is written as JSON, never in Python's own
notation. (The comparison with the Python analyzer needed no rule for this
number.)

### Difference 118

White space is what Go's `unicode.IsSpace` calls white space, so the
characters U+001C to U+001F are not trimmed from text. (The comparison
with the Python analyzer needed no rule for this number.)

### Difference 119

A timestamp's fraction is cut to the millisecond, never raised to the next
one. The Python analyzer could carry a time such as a file's modification
time or an epoch-seconds value into the next millisecond, so a row's time
can be one millisecond earlier than it was, and, where that changes the
order, the row can sit a little earlier in the timeline.

## Analysing: row text

### Difference 105

Rows of a Zed database's archived worktrees, and the problems found in a
`state.vscdb` database's values, come out in one fixed order (one archived
worktree's links, for example, ordered by thread id). (The comparison with the Python analyzer
needed no rule for this number.)

### Difference 106

A SQLite table that uses FTS5 (SQLite's full-text search) is always
readable, since `doubleagent` reads the database file itself rather than
through a SQLite library built with or without that feature. (The
comparison with the Python analyzer needed no rule for this number.)

### Difference 230

Under `--no-secrets`, a withheld secret is written as the placeholder in
place of the secret, and the rest of the row stays as it is. The Python
analyzer withheld secrets at certain places, each in a form of its own, and
cut a request row's URL at its query; in `doubleagent` every place uses the
one placeholder, and a request row keeps its URL with only the credentials
replaced.
[Difference 265](#difference-265) says what is shown by default.

### Difference 231

A secret the Python analyzer never printed and gave no row has a row of its
own, such as Agent Zero's connector settings from its `.env` file or a
stored API key of Letta. The secret is shown, or replaced by the
placeholder under `--no-secrets`.

### Difference 232

An identity value the Python analyzer left out of a row is shown: an id
that names an account, an organisation or a machine and grants no access,
such as a GitHub account in Claude Code's `.claude.json`. It is not a
secret, so it is shown under `--no-secrets` too.

### Difference 240

The OpenHands parser reads each conversation's `meta.json`, so a damaged
one is reported as a problem row. The Python analyzer did not read it.

### Difference 250

The token of OpenClaw's exec approvals socket is one row of its own,
`permission setting: socket.token=...`, not part of every setting's row.
The Python analyzer never read the token.

### Difference 260

A Gemini CLI network response or error row names its request by the
request's id alone. The Python analyzer repeated the request's method and
URL in every such row; the request's own row still holds them, so they
were written once per response for no new information.

## Analysing: errors and problem rows

### Difference 102

The wording of a library's or the operating system's error, inside
`parser:` rows, `problem:` lines and `error:` messages, is Go's and
`doubleagent`'s own readers', not Python's. For example, a line that is
not JSON is reported as
`parser: unparseable line (invalid JSON at line 1, column 1: invalid character 'x' looking for beginning of value): x`
where the Python analyzer wrote
`parser: unparseable line (Expecting value: line 1 column 1 (char 0)): x`.
Where the row is, and what it holds besides the wording, is unchanged.

### Difference 120

A value that an `error:` message quotes is in Go's double-quoted form
(`"x"`), where the Python analyzer used single quotes (`'x'`).

### Difference 122

A note counting one manifest row that did not parse says
`1 manifest row did not parse`, in the singular.

### Difference 130

Withdrawn, folded into [difference 230](#difference-230). It said that a
reader's error about a file holding secrets gave only where decoding
stopped; `doubleagent` now writes such a reader's detail as the placeholder
under `--no-secrets`.

### Difference 150

The `-shm` file beside a SQLite database (its shared-memory index) is
never read, so it is never reported as unreadable. The Python analyzer
could report an unreadable `-shm` file and read no further.

### Difference 160

A JSON array that is cut off, or has data after its end, is reported in
one wording: `parser: array cut off after record N`, N the records read
whole, and `parser: unexpected data after the array`. The Python analyzer
also wrote `unparseable record N` with Python's reason for an array cut
where a value was due, and `unparseable data after the JSON array`.

### Difference 170

A file that cannot be read as its kind, and a record member of the wrong
kind, are reported in a `parser:` row where the Python analyzer said
nothing: for example `parser: <file> has no session header` for a pi
session file, `parser: diagnostics.assistant holds 1 entry that is not a JSON object`,
a Muse file that is not a session log, and the same for Claude Desktop and
Cursor files. A silent skip would hide a damaged file from the analyst.

### Difference 210

Withdrawn, folded into [difference 170](#difference-170): it covered the
same kind of report for two Cursor files.

### Difference 241

A list whose entries are not all of the kind the format gives is reported
once per list, so a file with two such lists (a pi bug report whose
`diagnostics.crashes` and `diagnostics.assistant` both hold an entry that
is not an object) gets two `parser:` rows.
