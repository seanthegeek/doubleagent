# Internals

DoubleAgent is one Go program, `doubleagent`, built from `cmd/doubleagent` and the
packages under `internal/`. This page is the map a contributor reads before changing
it: which package owns each step of a collection run and of an analysis run, and the
design decisions the code depends on that a reader cannot see from one file. The rules
every Go file follows are in [Go standards in `internal/AGENTS.md`](../internal/AGENTS.md#go-standards)
and are linked here, not repeated. What the program writes is described on the reader
pages: [`collection-output.md`](collection-output.md) for a collection,
[`timeline-output.md`](timeline-output.md) for the analysis outputs,
[`coverage.md`](coverage.md) for what is collected and the matching rules a responder
sees.

`go doc ./internal/<package>` prints each package's own description, which is the
authority for its details; `go list ./...` lists the packages.

## Orientation

### A collection run

`doubleagent collect` reads the seven catalog tables, finds the homes of the host or of
a mounted image (`-r`), walks every catalog line in every home, then the Docker and
Podman volumes and the projects the agents' own state names, searches the startup
files for variables that move an agent's files, and writes the staging directory, with
its manifest, log and `collection.json`, into one gzipped tar archive and its SHA-256.
The steps run in this order, each stopping the run on a cancelled context:

1. Options and setup: `cmd/doubleagent` dispatches to `internal/collect`
   (`ParseOptions`, `Run`), which resolves the paths, loads `internal/catalog`, opens
   the log (`internal/runlog`) and the manifest (`internal/manifest`) and creates the
   staging directory.
2. Confinement: on an image run on Windows, `internal/follow` sets up the check that
   every opened handle lies under the image root ([The image root](#the-image-root)).
3. Homes: `internal/users`.
4. Catalog walk in each home: `internal/walk`, through `internal/glob`,
   `internal/fsmeta` and `internal/stage`.
5. Volumes: `internal/volumes`, which hands each matched volume to the walk.
6. Projects: `internal/projects` finds them; the walk collects the project catalog in
   each.
7. Relocation hints: `internal/relocation`.
8. Output: `internal/summary` writes `collection.json` and the stdout summary,
   `internal/archive` the archive and its `.sha256` file; under `--inventory`,
   `internal/inventory` prints JSON lines instead and nothing is staged.

### An analysis run

`doubleagent detect`, `timeline`, `fleet` and `catalog` live in `internal/analyze`.
`timeline`, the one that parses, opens its input, gives each file to the parsers that
want it, settles the rows of repeated ids, drops the rows of
[secondary sources](timeline-output.md#secondary-sources) (files that repeat events a
primary store also records) that a primary store holds, filters, and writes
`timeline.jsonl`, `sessions.jsonl` and `inventory.json`:

1. Input: `internal/input` opens an archive, an extracted collection or a loose tree
   and attributes every file to a host, user, home and agent.
2. Parser selection: the registry, `internal/parsers`, offers each artifact to its agent's
   parsers and to those that read that agent's files; each parser's `Wants` decides.
3. Parsing: each parser under `internal/parsers/<package>` reads its files through the
   kit, `internal/parserkit`, and emits `timeline.Row` values.
4. Repeated ids: `internal/timeline` attaches each group of rows of a repeated id to
   one parent (`Attach`, `Choose`).
5. Deduplication: `internal/dedup`.
6. Sorting and filters: `timeline.SortRows`, then `internal/rowfilter` for `--since`,
   `--until` and `--match`.
7. Shared texts: `timeline.Resolve` writes each long text several rows name in full
   once, in the first row of the output that names it.
8. Secrets and writers: `internal/secrets` replaces what a run withholds where output
   is written; `internal/timeline` writes the JSONL files and `internal/analyze` the
   inventory and the summary.

`detect` runs no parser: it reports the homes, users and agents the input holds.
`fleet` merges saved `collect --inventory` outputs into one CSV, and `catalog` prints
the bundled tables.

## Commands and run state

| Package | What it owns |
| --- | --- |
| `cmd/doubleagent` | Dispatch to a command, the top-level usage, `--version`, and the exit code of each command. |
| `internal/collect` | The `collect` command line, the run's state (`runState`) and the order of its steps, the writers of its log lines, `collection.json` and stdout. |
| `internal/analyze` | The analysis commands: one flag set each, their outputs and summaries, exit codes 0 (found), 1 (nothing found), 2 (an error) and 130 (interrupted). |
| `internal/runlog` | The run log: each line stamped with the injected clock, to the file and to standard error unless `-q`. |
| `internal/hostcmd` | The one place another program runs: `getent`, `dscl` and `plutil`. |
| `internal/version` | The version string, set from the tag at build time. |

A run's state lives in a struct passed down, and the clock, the host programs, the
current directory and the user enumeration are injected (`collect.environment`,
`analyze.Environment`), so a test pins each; [Go standards](../internal/AGENTS.md#errors-state-and-concurrency)
states the rule. `collect` exits 0, 1 (usage), 2 (fatal) or 130 (interrupted, after
logging `interrupted` and removing the staging directory and what it wrote of the
archive; [Signals](#signals)).

`hostcmd` looks for a program in fixed system directories before `PATH`, so a program
of the same name earlier on the operator's `PATH` is not the one run, and runs it in
the C locale. Copying, hashing, archiving, searching and listing are Go; nothing else
is run.

### Signals

A collection or inventory run ends before its archive is written only when it is
told to stop (an interrupt or a termination request) or meets a fatal error, and
both remove the staging directory unless `-k` is given. Losing its terminal or the
reader of its output is neither: the run continues, because the log file is its
record. Writing the archive has one point of no return: the archive is created new
as `<name>.tar.gz.partial` (whatever held that name is removed first, never
followed) and hashed as its bytes are written, its `.sha256` file is created, the
run checks that the partial name still names the file it wrote (`archive.StillNames`),
the context is looked at once more, and the archive is moved to its name. A stop
request before the move ends the run with the partial archive, the `.sha256` file and `collection.json` removed; once the
move has begun nothing looks at the context, so the run logs `done:` and exits `0`,
or, when the move still fails after it is tried again for about a second, exits `2` with the
complete archive left under its partial name beside its `.sha256` file
(`collect.runState.buildArchive`, whose comment states what holds of the partial
archive). The archive writer looks at the context before
each member and each read of a file, so a stop during a long archive takes effect at
once. `collect.catchSignals` sets the handlers from that rule when the run starts
and removes them when it returns; `--list`, `--version` and `--help` return before
it, since they stage nothing, so a closed standard output ends them by `SIGPIPE`.
A signal the run reacts to is caught, never ignored: an ignored signal would be
inherited by `getent`, `dscl` and `plutil`, while a caught one is reset to its
default action when they start.

These are the signals whose default action ends a process on Linux or macOS and that
a program can catch, with what a run does on each:

| Signals | What the run does | Why |
| --- | --- | --- |
| `SIGINT`, `SIGTERM` | Caught: before the archive's move to its name begins, the run logs `interrupted`, removes the staging directory unless `-k` is given, and the partial archive, its `.sha256` file and `collection.json`, and exits `130`; once the move has begun, the run finishes as if no request had come. | A request to stop. |
| `SIGHUP`, `SIGPIPE` | Caught and dropped: the run continues, and a write to the lost terminal or pipe fails and is dropped (or, under `--inventory`, is fatal). | The terminal or the reader of the output went away. |
| `SIGQUIT`, `SIGABRT`, `SIGILL`, `SIGTRAP`, `SIGSYS`, `SIGEMT` (macOS), `SIGSTKFLT` (Linux), and `SIGBUS`, `SIGFPE` or `SIGSEGV` sent by another program | Go's default: a stack dump on standard error and exit `2`, leaving the staging directory and a log with no closing line. | Each is a request for a stack dump or a sign that the program itself is broken; the dump is how a hung run is diagnosed, and a run that is hung could not clean up anyway. The staging directory left behind is readable by its owner only, as everything the run creates is, and is removed by hand. |
| `SIGBUS`, `SIGFPE`, `SIGSEGV` raised by the program's own fault | Go's default: a run-time panic, exit `2`, as above. | A bug in the program. |
| `SIGUSR1`, `SIGUSR2`, `SIGALRM`, `SIGVTALRM`, `SIGXCPU`, `SIGXFSZ`, `SIGIO` and `SIGPWR` (Linux), the real-time signals from 35 (Linux) | Go's default: caught and dropped, so the run continues. | None asks the run to stop. A write past a file size limit then fails, a fatal error; on Linux a run past its hard CPU-time limit is killed by `SIGKILL`. |
| signals 32, 33 and 34 (Linux) | The system's default: the process ends, leaving the staging directory as `SIGKILL` does. | Go leaves them to the C library's threads and sets no handler for them. |
| `SIGPROF` | Go's default: used by the profiler, otherwise no action. | The run never profiles itself. |
| `SIGTSTP`, `SIGTTIN`, `SIGTTOU` | The system's default: the process stops until `SIGCONT`, and does not end. | Job control belongs to the operator's shell. When the shell that stopped the run exits, the system sends it `SIGHUP` and `SIGCONT`, and the run continues. |
| `SIGKILL`, `SIGSTOP` | Cannot be caught. `SIGKILL` leaves the staging directory as `SIGQUIT` does. | |

On Windows, Go turns Ctrl+C and Ctrl+Break into `os.Interrupt`, and the console being
closed (or a logoff or shutdown event, which only services receive) into `SIGTERM`;
both are requests to stop. Windows ends the process once the handler for a close has
had its time ([HandlerRoutine callback function](https://learn.microsoft.com/en-us/windows/console/handlerroutine)
gives the timeouts), so the cleanup may not finish. A write to a closed pipe or console
fails without a signal, so no signal needs dropping there.

## Paths and the file system

| Package | What it owns |
| --- | --- |
| `internal/syspath` | The one path form used inside (forward slashes, drive letter as spelled, no trailing slash), the conversions to the native and extended-length forms, the ASCII-only fold, and `Entries`, the set of paths keyed by file identity. |
| `internal/osfile` | Every `os` function that takes a path, wrapped so that on Windows it gets the extended-length form. |
| `internal/fsmeta` | The metadata a manifest row records, opening without following a link, the directory listing and size, the access-time hold on Windows, and the image-root opener (`Opener`). |
| `internal/evidencepath` | Reading a path as the machine that recorded it wrote it: a Windows path analysed on a Mac, a POSIX path analysed on Windows. |
| `internal/shortname` | For Windows tests only: a directory typed through its 8.3 short name. |

**Paths are bytes.** A path is a `string` holding the bytes the file system gave, and
nothing decodes it as UTF-8 or folds it by Unicode rules
([Go standards](../internal/AGENTS.md#files-programs-and-paths)). A file name that is not valid UTF-8 is
evidence: the archive member keeps the real bytes, and only the manifest's JSON shows
U+FFFD in place of each bad byte. Case folding is ASCII-only, because NTFS keeps apart
names that a Unicode fold would merge, and merging two names loses one of them.

**Windows reaches the file system through `internal/osfile`.** Windows normalises an
ordinary path before using it: a dot or a space that ends a component is dropped. NTFS
stores such names, so through an ordinary path a directory named `x.` is not found, or,
when a neighbour named `x` exists, the neighbour is opened in its place without an
error, and a row would pair one entry's name with another's contents. A path with the
`\\?\` prefix reaches the entry it names and is not limited to 260 characters; `os` adds
the prefix only to long paths, so `osfile` adds it to every absolute path. A test in
`internal/osfile` fails on a direct `os` call in any file of the Windows build. Errors
come back with their paths in native form; an error from a returned `*os.File`'s
methods goes through `syspath.NativeErrors` before it is shown.

**A Windows path from outside a listing is respelled first.** `-r`, `-o`, the
registry's profile paths and project paths named in evidence are respelled as the disk
holds them (`fsmeta.OnDiskPath`, stopping at the first link) before they are compared,
so two spellings of one directory become the same bytes.

**Access times on Windows.** Reading a file can update its last-access time. Where the
system updates it (the `NtfsDisableLastAccessUpdate` setting), the opener asks the file
system to hold the time for the open handle and verifies it; a file read without the
hold is logged once per file and reason (`access time not held:`) and counted at the
end. `--no-preserve-atime` turns the hold off; [`windows.md`](windows.md) has what the
responder sees.

## Catalog

| Package | What it owns |
| --- | --- |
| `internal/catalog` | The seven tables (`CATALOG`, `PROJECT_CATALOG`, `EXCLUDES`, `SECRET_GLOBS`, `DOCKER_VOLUMES`, `RELOCATION_VARS`, `STARTUP_FILES`) as one embedded text file, `catalog.txt`, in the format `collect --list` prints; its tests check the glob rules. |

No path or glob that belongs in a table lives in code. A new agent is new table lines;
[Adding an agent to the catalog in `internal/catalog/AGENTS.md`](../internal/catalog/AGENTS.md#adding-an-agent-to-the-catalog)
lists the lines, tests and documents that go with them.

## Glob matching

`internal/glob` matches the tables' patterns the way a POSIX shell (dash, under
`LC_ALL=C`) does: byte by byte, `*`, `?` and bracket lists with `[!...]` for negation.
On Windows every match ignores ASCII case, pattern and name lowered first. It offers
three entry points, and each table is served by one:

| Table | Function | Matched against |
| --- | --- | --- |
| `CATALOG`, `PROJECT_CATALOG` | `Expand` | Inside the base (a home, a project, a volume's `_data`), one path segment at a time: `*` and `?` do not cross `/`, and a name starting with `.` matches only a segment that starts with a dot. |
| `STARTUP_FILES` | `Expand` | Inside each base of the line's scope; a system glob's leading `/` is dropped so it is taken under the root. Only regular files are kept. |
| `EXCLUDES` | `MatchPath` | The full path, with the base quoted (`QuoteMeta`) so only the glob is a pattern: `*` crosses `/`. |
| `SECRET_GLOBS` | `MatchPath` | The path relative to the base: `*` crosses `/`. |
| `DOCKER_VOLUMES` | `MatchPath` | The volume directory's name; of the matching lines, the one with the fewest `*` wins, then the one with the most literal characters, then the earlier line. |

`MatchSegment` is one step of `Expand`: one directory entry against one segment.

`Expand` sorts each pattern's matches bytewise on Linux and macOS, as the shell sorts a
pathname expansion, and keeps them in directory order on Windows, where it also
respells each literal segment that exists as the disk holds it
(`fsmeta.OnDiskName`). A pattern that matches nothing gives no match. Every match is a
value in a list, never a line of text read back, so a name holding a newline or any other
byte is one match under its whole name, in every table, the homes and the project list
alike (`walk.ExpandCatalog`, `users`, `projects.Discover`, `relocation`). `Expand` asks
the image root's check before it lists a directory and returns each link the check
refused ([The image root](#the-image-root)).

The tables hold only `*`, `?` and closed bracket lists of literal bytes, ranges and
`!`: the shells disagree on character class names, collating and equivalence classes,
backslashes, carets and unclosed brackets, and the
[catalog rules in `internal/catalog/AGENTS.md`](../internal/catalog/AGENTS.md#catalog-rules) and
`internal/catalog`'s tests refuse them.

**A symlinked directory in a catalog glob.** `Expand` follows a symlinked directory in
the middle of a pattern, as a shell's pathname expansion does, though the walk from a
match never descends a link. So before a match is logged, inventoried or walked,
`walk.LinkedParent` tests the directories between the base and the match. When one is
a link whose target leads out of the base, or cannot be resolved, the match is logged
and skipped, and the link itself is recorded once as a `symlink` row, unless a claimed
path's walk records it ([Claimed paths and case variants](#claimed-paths-and-case-variants)).
A link whose target stays inside the base is followed: each such link is recorded once,
and the match's files are staged at their physical path in the base, through the
deepest such link. On an image run a link that leads out of the image root is never
listed behind ([The image root](#the-image-root)).
[Paths through a symlinked directory in `coverage.md`](coverage.md#paths-through-a-symlinked-directory)
has the rule as a responder sees it.

## Users

| Package | What it owns |
| --- | --- |
| `internal/users` | The homes to collect. On Linux and macOS: the password database (`getent passwd` live, else the image's `/etc/passwd`), `dscl` on macOS, then the usual home parents. On Windows: the home parents, and the registry's `ProfileList` on a live run. It drops system and shared homes, applies `-u`, and keeps each home once. |

Each candidate home holding a `.` or `..` component is first replaced by the path the
system reaches for it (`walk.RootCheck.Reached`), so the system-home test, the duplicate
check and `-u` all see the reached path. Two homes that reach one directory are one,
the first kept, and the log names both users:
`user homes: /img/srv/b/frank is the home of frank and of frank2; walked once, as frank's`.

## Catalog walk

| Package | What it owns |
| --- | --- |
| `internal/walk` | `Collector`, which every walk of a run shares; `ExpandCatalog`, the expansion of a table under a base; `CollectCatalog`, `CollectPath` and `CollectTree`; the case folding of matches, the claims of nested matches, `LinkedParent`, and `RootCheck`, the image root's check. |

The walk lists each directory in the order the system gives it
(`(*os.File).ReadDir(-1)` through `fsmeta`), never `filepath.WalkDir` or `os.ReadDir`,
which sort by name and know nothing of the image root. Each match is walked depth
first, each directory before its entries. `EXCLUDES` prunes a path with one
`skipped_excluded` row, sized for a real directory by its disk use and, for a link, typed `symlink` with its target and not followed; a directory that cannot
be listed or entered gets one `error_read` row and is not walked; every file is
`lstat`ed before it is copied, and a symlink is recorded and recreated, never followed.

**Row order is part of the output.** The manifest is written in run order: homes in
enumeration order (the `error_read` rows of homes a parent lists but the run cannot see
first, then each home's own `error_read` row when it cannot be walked, then its catalog
matches in table order), then the volumes, then the projects (first a row for each `-p`
value that is not a directory, then each project in discovery order). Within one walk
the order depends on the platform, and the expected outputs hold it: on Windows each
row is written as the walk reaches its path, so a failed copy's log line follows its
file; on Linux a walk's `skipped_excluded` rows come first, then its `error_read` rows,
then its file rows; on macOS the file rows, then the `error_read` rows, then the
`skipped_excluded` rows (`walk/flush_*.go`, `walk/rows_*.go`). The log keeps each
`copy failed:` line beside its match line
([Standards every change keeps](../AGENTS.md#standards-every-change-keeps)).

**Inventory.** Under `--inventory` the same walk stages nothing: each match of an agent
other than `shared` and each regular file's size and time go to `walk.Records`, which
`internal/inventory` aggregates per user. The homes are walked after project discovery,
so each user's line can count the projects they own.
[`collect-inventory.md`](collect-inventory.md) has the output.

## Claimed paths and case variants

A catalog line inside another line's match (`.gemini/antigravity-cli` inside
`.gemini`) is a [nested entry](coverage.md#nested-entries): it claims its subtree, so
each file is attributed once, to the most specific agent. `CollectCatalog` expands the
whole table under a base before it walks anything, puts every match into a claim set,
and the walk of each match prunes the claimed paths inside it.

Lines that differ only in case can meet one directory on a file system that ignores
case ([Case variants in `coverage.md`](coverage.md#case-variants)). `ExpandCatalog`
folds them on every platform (`foldCaseMatches`). Two matches collide when their
ASCII-lowered paths are equal, or one lies inside the other's directory, and they spell
that shared part differently; when it is one file by device and inode, both take the
spelling the disk holds for it. On Linux and macOS a literal glob segment is looked up,
not listed, and a case-insensitive volume answers a lookup for any spelling, so
`diskSpelling` lists the directory holding each name the two spell differently
(`glob.Names`, which asks the root check and opens through the run's opener as the
expander does) and takes the one entry equal to it once ASCII letters are lowered; on
Windows the expander has already spelled each literal segment from the disk. Then a
match whose path equals an earlier match's is dropped with a log line, so the earliest
line's match survives under the on-disk spelling, and a match inside another keeps its
line and is pruned by the enclosing claim. Only colliding matches are stat'ed, and only
a collision whose two paths are one file costs a directory read, one per differing
name.

The claims, the links already recorded, the paths the manifest has a row for, the homes
and projects kept once, and the paths the relocation sweep leaves out are each a
`syspath.Entries`: a set keyed by the path with ASCII capitals lowered, whose keys keep
the exact spellings. Two paths with equal bytes are one entry. Two that differ only in
ASCII case are one entry when both can be examined and `lstat` gives them one non-zero
identity (volume serial number and file index, or device and inode), or when neither
exists, since two missing paths hold no evidence (`fsmeta.SameEntry`, behind the root
check in `follow.SameEntry`). They are two entries otherwise: one missing and one
present, two identities, an identity of 0, or an error other than not found. So in a
Windows directory marked case-sensitive (`fsutil file setCaseSensitiveInfo`, or one WSL
made) `Alice` and `alice` are two homes, two claims and two rows, while in an ordinary
directory two spellings of one name are one. On Linux and macOS the key is the bytes,
and the set never asks the file system. A test of whether a path lies under a home or
base (`syspath.CutUnder`) trusts a prefix spelled byte for byte, and one differing only
in ASCII case only when the file system says it is the same directory; where several
homes could own a project, the one spelled as the project's path wins.

## The image root

| Package | What it owns |
| --- | --- |
| `internal/follow` | The one place a link in the last component of a path is followed (a stat, a directory or regular-file test, an access check, a listing test, a read of a whole file, a resolution to the physical directory), each after asking the root check; `Confinement`, the Windows second check; and the guard tests that hold every other package to it. |
| `internal/walk` (`RootCheck`) | The root check itself: whether a path, resolved as the operating system will resolve it, stays under the image root. |

In image mode nothing behind a symbolic link (on Windows also a junction) that leads out
of the image root is listed, stat'ed or read: following it would read the analyst's own
workstation. A live run has no root to leave and passes a nil check, and the `follow`
functions then follow as the operating system does. `os.Root` is not used because it
refuses every absolute link and words its refusal plainly; here an absolute link naming
a path under the root is followed, and a refusal names the link and why
(`*follow.OutOfRootError`), which the log prints.

The check judges the path the operating system will open, resolved the way that system
resolves it, so the two can never name different files:

- **Linux and macOS.** The path as given. The kernel resolves each component in turn, a
  link's target included, from where it has physically got to, so a `..` goes up from
  the directory a link really leads to, and so does the check. A component that cannot
  be stat'ed ends the check as the image's own answer, since the kernel fails the whole
  path at the same component.
- **Windows**, as a test measures on the CI runner. Win32 removes `.` and `..` from a
  path by name before the file system sees it, so the check first does the same
  (`syspath.Opened`, the code `Extended` uses): `Users/alice/nope/../j` is judged as
  `Users/alice/j`. A symbolic link's relative target is joined to the path as named up
  to the link, a junction's name included, and its `..` removed by name; the open then
  starts again with that name, and so does the check. A junction is looked through
  without its target taking its name's place; Windows refuses a junction whose target
  holds `.` or `..`, and so does the check. A link's target is read from its reparse
  data, not from the text `os.Readlink` rewrites it into, and only a closed set of forms
  is followed: a symbolic link marked relative whose name is a plain relative path or
  `\x` (rooted on the link's own drive or share); a symbolic link not marked relative
  whose name is `\??\X:\...` or `\??\UNC\server\share\...`; and a junction whose name is
  `\??\X:\...`. Every other target (a volume GUID name, `GLOBALROOT`, a `\Device\` or
  `\GLOBAL??\` name, a `\\.\` path, a drive-relative `X:x`, an empty name, data that
  cannot be parsed) cannot be verified and is refused. A missing component, or one
  whose name no file can have, ends the check as the image's own answer; any other
  component that cannot be stat'ed (access refused) is refused when names follow it,
  since traverse rights may let an open pass through it.

A link whose target cannot be read, and a chain of more than 40 links, count as leaving
the root, since the analyst's host may resolve what the check could not (Windows follows
up to 63 reparse points in a path). A reparse point other than a symlink or junction (a
cloud-file placeholder, a deduplicated file, an AF_UNIX socket, a WSL symlink, an app
execution alias) is refused the same way, since a filter driver on the analyst's machine
may act on it.

Each refusal carries its kind (`follow.Refusal`), and every log line that reports one
words it by that kind after the refused path, through `follow.Reason`; a new caller
uses it rather than wording a refusal of its own. A project or `-p` value that is
refused gets that line only, no `error_read` row and no warning that it is not a
directory, since testing it as a directory would follow the link on the analyst's host.

**Paths with `.` or `..`.** The paths that can carry `..` come from outside a listing:
a discovered project value, a `-p` value, a passwd home, a link's target. Each is
replaced once, before anything else uses it, by the path the system reaches for it
(`walk.RootCheck.Reached`): on Linux and macOS the physical path the root check's own
walk reached, spelled under the root as given; on Windows the path with `.` and `..`
removed by name. Its row, archive path, log line and `collection.json` entry then all
name that one path. A path the check refuses, or one the kernel cannot resolve, is kept
as written, and staging refuses any path that still holds a `.` or `..` component: its
row is `error_copy` with no archive path and the error
`holds a . or .. component, so where it is staged is not known`.

**The second check on Windows.** In image mode on Windows a second check stands behind
the root check and does not depend on its walk being right. Every evidence file the run
opens (a collected file's copy, project discovery's reads, the relocation search and
sweep) and every directory it opens to list (the tree walk, the glob expander,
discovery's name search, the relocation sweep, the listing test of `follow.DirReadError`
and `CanRead`, and the sizing of an excluded directory or an unmatched volume by
`fsmeta.DiskUsageBytes`) goes through the run's opener (`fsmeta.Opener.Open` and
`fsmeta.OpenDir`), which asks the open handle where it is (`GetFinalPathNameByHandle`)
and requires that to be the image root's own place or below it. The place is asked in
the NT device form (`\Device\HarddiskVolume3\...`, `\Device\Mup\server\share\...` for a
network share), which every volume has: the drive-letter form fails for a volume without
a letter and the volume GUID form for a network share. The two places are compared byte
for byte, since both come from the same call; folding case would let in a sibling a
case-sensitive directory keeps apart from the root. A handle that lies elsewhere, or
whose place cannot be asked, is closed and nothing from it is used, and the path is
logged once:
`C:\img\Users\alice\j\notes: the root check allowed it but Windows opened it outside the image root; nothing from it was used`.
When the root's own place cannot be asked, refusing every open would lose the whole
image, so the run goes on with the root check alone and logs
`image root: where C:\img lies cannot be read (<reason>); what is opened under it is checked by the root check alone`.
The check costs one system call per open; a live run has none. The root
check alone covers what opens no handle the run keeps, or reads only a
handle's own name: the stats behind `follow.Stat`, `IsDirectory` and
`IsRegularFile` and the glob expander's existence test (`os.Stat`); the
name lookups of `fsmeta.OnDiskName` and `OnDiskPath`; a candidate home's
parent listing on Windows (`os.ReadDir`, which only says whether the parent
lists the name); `CanEnter`, which opens nothing on Windows;
`follow.PhysicalDir` and `stage.PhysicalDir`, which read a directory handle's
own final path, compared with the root by their callers; and
`follow.ReadFile`, which reads only the Unix `/etc/passwd`. A test in
`internal/follow` (`TestEveryImageSiteIsAdmittedOrDocumented`) fails when a
counted call on an image path is neither admitted by the opener nor named in
this paragraph.

Linux and macOS have no second check: their walk is the kernel's own physical
resolution, compared with the kernel on every shape by a test, while asking an open file
for its path there is not exact (Linux's `/proc/self/fd` names a deleted file with a
suffix and depends on the mount namespace; macOS's `F_GETPATH` gives one of a file's
hard links).

**The guard test.** A test in `internal/follow` denies by default: outside
`internal/follow`, `internal/osfile`, `internal/fsmeta` and `internal/stage`, every
non-test file may import only a reviewed package, and may use an exported name of a
package that can act on a path (`os`, `os/exec`, `path/filepath`, `syscall`,
`x/sys/unix`, `x/sys/windows`, `osfile`, `fsmeta`, `stage` and a few more) only when a
list says it takes no path, or never follows its last component and changes nothing,
or a counted list gives the call's reason per file. A new call of the last kind is
added to the counted list with its reason and, when it touches an image path,
classified in `confine_sites_test.go`. The test sees package-level names only, not a
method or a call through an interface; the package comment of `internal/follow` names
the methods in use and why each is safe.

## Staging and the archive

| Package | What it owns |
| --- | --- |
| `internal/stage` | The staging directory, the tree that becomes the archive: each file's staged path, the copy and its hash, recreated symlinks, the record of what was staged where (`Places`), and the removal of a failed partial copy. |
| `internal/manifest` | `manifest.jsonl`, one JSON object per row through `encoding/json` in the documented field order, and the counts the summary reports. |
| `internal/summary` | `collection.json` and the stdout summary. |
| `internal/archive` | The gzipped tar of the staging directory (`archive/tar`, `compress/gzip`), its SHA-256 and the `.sha256` file. |
| `internal/inventory` | The `--inventory` host and agent lines. |

The fields, statuses and archive layout are an interface,
[documented in `collection-output.md`](collection-output.md).

**Every output is created new.** The names a run writes under `-o` are predictable (the
host and the start time), and a root run may write into a directory another account can
also write. So the staging directory is made with `osfile.Mkdir` (on Windows
`CreateDirectory` with its own access list) and then held open as a `stage.Tree`, its `fs`
directory made through the tree, and the log, the manifest, `collection.json`, the copies staged beside them, the partial
archive and its `.sha256` file are opened with `O_CREAT|O_EXCL` (`collect.createNew`,
`runlog.Open`, `archive.WriteTarGz`, `archive.WriteSidecar`): anything already at the
name, a dangling link included, fails the create and is neither followed nor changed. On
Windows `O_EXCL` becomes `CREATE_NEW` with `FILE_FLAG_OPEN_REPARSE_POINT`
(`syscall/syscall_windows.go`), and `CreateDirectory` refuses a reparse point at the
name. A name taken before the log is open ends the run with its message on standard
error; one taken later with a `FATAL:` line, exit `2` either way. The run removes only
what it made: the staging directory, `collection.json` on an interrupt, the partial
archive and the `.sha256` file on a failure. Only the partial and final archive names
are cleared first. The archive's hash is taken of the bytes as they are written, and the
file's identity from its open handle, so an entry put at the partial name during the run
is neither hashed nor moved.

**The staging directory is reached through its open handle.** `stage.Tree` is the staging
directory opened once, right after it is made, and every staged write, listing, link and
removal (`stage.Stager`, `archive.WriteTarGz`, the run's cleanup) takes a path relative
to it. On Linux and macOS the tree is an `os.Root`, which walks each name from the
directory's own descriptor, so a rename of the directory changes nothing, and refuses a
`..` or a link that leads out of it, which the stager reports as its own refusal. So a
run's names do not each walk from the top, the tree also keeps open the chain of
directories from it down to the parent of the last name it used, each opened as one real
directory from the one above, never through a link; a name in another directory keeps the
part of the chain the two share and opens, or makes, the rest one component at a time,
and a name whose path holds a link is walked from the top as before. On
Windows `os.Root` cleans every name with `GetFullPathName`, which drops a dot or a space
ending a name, so the tree instead holds a handle on the directory that shares reading and
writing but not deletion: Windows then refuses any rename or removal of the directory
while the run works, and the staged paths go through `osfile` under that fixed name as
before. `stage.CreateTree` refuses a directory swapped in between the make and the open:
one that is not empty, is not what the name holds, or has another owner than the one the
volume gives an entry the run creates in it then (a probe file's owner, read from its
handle). On Windows the output directory's volume is asked first whether it keeps access
lists (`GetVolumeInformationByHandle`, `FILE_PERSISTENT_ACLS`): where it does, the staging
directory is made with its own access list and the running account as owner, and the
owners are compared; where it keeps none or does not answer (a share), the directory is
made plainly and no owner is compared. A member's spelling never depends on where the tree
is or was: it is the name the stager created the file by, unless the volume respelled a
component; the path the system gives for the new file (`F_GETPATH` on macOS,
`GetFinalPathNameByHandle` on Windows) is used only for as many trailing components as the
name has, and a candidate differing from the name is taken only when a lookup through the
tree, following no link, finds that very file (`stage.Stager.member`). On Windows the held
handle's own path, never the name, decides whether a resolved directory lies inside. On a
volume that keeps no owners or access lists every entry has
the same owner, so that check cannot refuse anything there, and nothing can be protected
from another account that reaches the volume; the run still stages through the handle on
Linux and macOS and still holds the name on Windows. `Tree.Discard`
empties the tree, closes it and removes the name only when it still holds the tree. One
moment stays open to an account that may rename or delete entries in the output
directory: the one between the archive's last check and its rename, stated on
`collect.runState.placeArchive`. The output directory must be one only the running
account can write. The write probe both commands make of their output directory is a
pair: on Linux and macOS it creates and removes its file below the directory opened once;
on Windows it creates the file by its path, as the run's own files are made, with
`FILE_FLAG_DELETE_ON_CLOSE`, so closing it removes that very file, since every `os.Root`
open there asks for backup intent (`FILE_OPEN_FOR_BACKUP_INTENT`), under which the system
may grant access before it checks the access list, so a creation through it can succeed
where the run's own creations are refused.

**A log line is one line.** `runlog.Printable` writes each byte below 0x20, 0x7F and each
byte that is not valid UTF-8 as `\xNN`, and the C1 controls, U+2028 and U+2029 as
`\uNNNN`, the form `analyze`'s `shown` gives the analysis commands' terminal output
(which leaves U+2028 and U+2029 as they are), in the log file and in its copy on standard
error, so a file name holding a newline, a
carriage return or a terminal escape cannot end a line, forge one or drive the
operator's terminal. The UTF-8 decoding there only chooses how a byte is shown. The
manifest and `collection.json` hold every name exactly, through `encoding/json`.

**Safety does not depend on knowing which names a volume merges.** A volume can take two
names for one: by letter case (the default macOS volume, an exFAT or FAT drive, a Linux
file system that folds case, a Windows file system without the case-sensitive setting),
by Unicode form (every APFS volume, case-sensitive ones included), or on Windows by an
8.3 short name such as `CLAUDE~1`. On Windows the run marks the staging `fs` directory
case-sensitive while it is still empty (`stage.MarkCaseSensitive`), and the directories
made in it inherit the mark, so `fs/.../Alice/x` and `fs/.../alice/x` stay two files;
short names can still merge there. So every staged copy is created exclusively
(`O_CREAT|O_EXCL|O_NOFOLLOW`, `CREATE_NEW` on Windows): when the volume takes a name for
an existing entry's, by whatever rule, the create fails and nothing is opened for write.
`stage.Places` records, keyed by each staged entry's own identity, the source identity
it was made from and what was returned for it. On a refused create the staged path is
looked up:

- the same non-zero source (the same path again, or one source reached through two
  spellings) returns the first copy's member and hash, so both rows name one member;
- an entry made for another staged path, or one the volume reached through another
  spelling (a walk of the staged path, `walkStaged`, finds a component respelled), is a
  collision: a file's row is `error_copy` with
  `the output directory takes its name for another file's, link's or directory's; not staged`,
  and a link keeps its `symlink` row with no archive path and that reason after
  `not recreated in the archive:`;
- a source staged again at the same path after its file was replaced is refused with
  `changed since this run staged it; the earlier row holds its copy`;
- anything else is refused with `not a regular file` for a link or other reparse
  point, or `already exists in the staging directory and is not known to be this source's copy`.

A copy that fails removes the file it created and leaves no record. `walkStaged` lists
only directories it found typed as real directories, so it never lists anything behind
a recreated link, and every write, open or link creation first checks that its directory
resolves inside the staging tree (`requireInside`), since a recreated link may lead to
the source or to the analyst's own files.

Each row's archive path is spelled as the member is named: on Windows and macOS from the
path the system reports for the new file (`GetFinalPathNameByHandle`, `F_GETPATH`); on
Linux and for links from the walk, where the run's probe (`stage.FoldsCase`: a random
lower-case name looked up in upper case, and a precomposed `é` looked up decomposed)
finds that the volume folds names. The probe also decides the run's log line
`NOTE: output directory <dir> ignores letter case or Unicode form in names; ...`.

## Docker and Podman volumes

| Package | What it owns |
| --- | --- |
| `internal/volumes` | The volume directories found on disk under the root (`var/lib/docker/volumes`, `var/lib/containers/storage/volumes`, `ProgramData/Docker/volumes`) and each user's rootless Docker and Podman directories, matched against `DOCKER_VOLUMES`; the `skipped_unmatched_volume` rows; the notes for Docker Desktop data. |

Volumes are found on disk, never through the `docker` program, so a live host and a disk
image are handled alike. A matched volume's `_data` is walked as one match with user
`docker` and `_data` as the base, so `EXCLUDES` and `SECRET_GLOBS` match relative to it.
A failure (an unreadable volumes directory, a volume that cannot be walked) is counted
and noted and never stops the run. [`docker.md`](docker.md) has the rules.

## Project discovery

`internal/projects` reads the project paths each home's agent state names. Its readers
match lines, as `grep` and `sed` would, rather than parse JSON, YAML or TOML, so a
damaged or unusual file still gives the paths it names. Paths that live only in a SQLite
database are left to the analysis commands. Six source groups run per home, in this
order, and the sources inside each in the order `groups.go` writes them, because the
project list, and with it the manifest's row order, follows the order in which paths are
first seen:

| # | Function | Sources |
| --- | --- | --- |
| 1 | `claudeCodeSources` | Claude Code: prompt history and `.claude.json` project keys, also in `CLAUDE_CONFIG_DIR` homes (`.claude-*`). |
| 2 | `cliSessionSources` | Codex (a rollout `cwd` in `sessions` or `archived_sessions`, outside `Documents/Codex` and `.codex/worktrees`), Qwen Code, Cline CLI, Cursor chats, Continue, Goose sessions, OpenCode, Kilo, Crush, Agent Zero chats and their extra directories. |
| 3 | `geminiSources` | Gemini CLI projects, trusted folders and project roots; Antigravity CLI history, settings and project cache; Gemini config projects. |
| 4 | `agentSessionSources`, `moreAgentSessionSources` | Amp, Factory, Kiro, Cursor ACP sessions, Goose recent directories, scheduled jobs and project files (`gooseSources`), Copilot CLI (sessions with an `events.jsonl`, a `cwd` outside `.copilot/`), Hermes, Letta, pi, muse, Open Interpreter (a rollout `cwd` outside `.openinterpreter/worktrees`), OpenHands, PearAI. |
| 5 | `configFileSources` | OpenClaw (with its Clawdbot and Moltbot configs), nanobot, Tabby, Twinny (the deepest directory its indexed files share). |
| 6 | `editorSources` | VS Code-family `User` directories: `workspaceStorage` folders and the Cline, Roo Code and Kilo Code task indexes. |

A new source goes into the group of its kind, at the end unless its place matters, and
moves no existing source. After the groups, `Discover` adds the absolute `-p` values,
puts each path through `walk.RootCheck.Reached`, keeps each path once with its first
non-empty owner, and drops what is not a directory or is a home. A value becomes a path
by the host's rules (`localProjectPath`): on Linux and macOS a value must start with
`/`, and the image root goes before it; on Windows drive paths and `file://` URIs are
read too, and the result is respelled as the disk holds it. Under `--inventory` no file
`SECRET_GLOBS` matches is read.

In image mode discovery looks at nothing behind a link that leads out of the image root.
Before it opens a source, tests one, searches below a starting directory or lets the glob
expander list a directory, it asks the root check, and a refused link is logged once per
home:
`Discovery [alice] /img/home/alice/.copilot: the symlink leads out of the image root; nothing behind it was listed or read`.
The line says only that the link exists and leads out; no name from behind it reaches the
log. [Project discovery in `coverage.md`](coverage.md#project-discovery) lists the sources
for a responder.

## Relocation hints

`internal/relocation` searches the startup files (`STARTUP_FILES`) of each home, of the
system and of each project for the variable names of `RELOCATION_VARS`, which move an
agent's files, logs each hint, records it for `collection.json` and hands each file that
names one back to the walk to collect as agent `shared`, flagged as a credential file.
[`relocation.md`](relocation.md) describes what a responder sees.

| Package | What it owns |
| --- | --- |
| `internal/relocation` | `Search`: the startup files of every home, then the system's, then the registry, then every project's, then under `--env-scan-home` the sweep of every other file of each home and project; the readers of the values; the registry pass. |
| `internal/regtext` | The text of a `REG_SZ` or `REG_EXPAND_SZ` value with its embedded NULs, so a value such as `C:\a<NUL>junk` is refused rather than read as `C:\a`. |

The step evaluates nothing. A name matches as a whole word, in its own case (a shell
names variables case-sensitively), and in any ASCII case in a `.ps1` file, where a name
before a colon is a drive or scope qualifier (`$env:`) and no hit (`BoundedNames`). The
matched text never reaches the log, stdout or a file; the only value that does is one
that passes `AcceptValue`. A value is read only where the file's syntax needs no shell
(`Kind`): `/etc/environment` as pam_env reads it, an `environment.d` `.conf` file as
systemd reads it, and a launchd plist under `LaunchAgents` or `LaunchDaemons` through
`plutil` only, so the value is the one launchd reads. On a live Windows run the registry
pass reads the system environment key and the `Environment` key of each collected
profile whose hive is loaded, never loading one, since loading changes the user's
`NTUSER.DAT`. A startup file is a regular file only: a symlink is never read, nor a file
under a symlinked directory whose target leads out of its base. A path holding a newline
is listed like any other. The sweep never reads the catalog matches, the active
exclusions or the run's own output, and skips a project that is or holds a home.

## Analysis input

| Package | What it owns |
| --- | --- |
| `internal/input` | `Open`: an archive (tar, plain, gzip or bzip2, or zip), extracted first into a new temporary directory named `doubleagent-...`; an extracted collection (`manifest.jsonl`, `fs/` and usually `collection.json`); or a loose tree. The attribution of every file (`Artifact`, `Home`, `Collection`) and the paths parsers may build from it. |

With a manifest, host, user, home and agent come from its rows; without one, homes are
found by matching the catalog against the tree and the user is inferred from the path. A
loose tree is read in name order, so one tree gives one result on every file system.

Evidence names never become paths unchecked. A path built from the evidence's text (a
manifest field, a name a parser asks `Home.PathOf` for) must be plain components, and
each directory on the way down is looked at with `lstat` and a link (on Windows any
reparse point) refused. `os.Root` is not used: it follows a link that stays inside and
bypasses `internal/osfile`'s Windows path form. Extraction makes no link or device,
checks each member name's components, lstats each directory it makes and creates every
file new; a member that cannot be placed is skipped with a problem naming archive and
member. [`inputs.md`](inputs.md) has what an analyst sees.

## Parsers and the parser kit

| Package | What it owns |
| --- | --- |
| `internal/parserkit` | The kit: the `Parser` interface and the optional ones (`AgentReader`, `SecondaryReader`, `ToleranceSetter`, `Marker`, `KindLister`, `HooksDeclarer`), `Base` and its rows, the readers of files, lines, JSON lines, JSON arrays, JSONC and SQLite databases, the path builders, `Problems`, carried values and the secret identifiers. |
| `internal/parsers` | The registry: the parsers in the order that decides ties in the output (`constructors`), the routing of an artifact to the parsers offered it (`For`), and the run of every parser over a collection (`Collect`). |
| `internal/parsers/<package>` | One agent's parser, named for it. |
| `internal/growthtests` | Tests only: the growth test, which holds the output and work of every registered parser to growing in step with its input, and the tests of repeated member names and of carried values that share its helpers. |
| `internal/parserkit/parsertest` | For tests: a parse run as an analysis run does it, returning rows as `timeline.jsonl` writes them. |

A parser owns one catalog agent, lists the kinds of file it reads (`Kinds`, from which
its `Wants` is built) and emits rows from each artifact (`Parse`). A new parser is
registered by one line in `constructors` at its place; no existing line moves, since the
order is part of the output. [Adding a parser in `internal/parsers/AGENTS.md`](../internal/parsers/AGENTS.md#adding-a-parser)
is the checklist; [`development.md`](development.md) the workflow.

**A parser's paths stay inside the collection.** A parser package builds no disk path
itself: outside its tests it calls none of `filepath.Join`, `Dir`, `Clean`, `Abs`,
`Rel`, `EvalSymlinks`, `Glob`, `Walk` and `WalkDir`. It takes every path it opens from
the artifact, from the kit's `Beside`, `Above` or `Under`, from `Home.PathOf`, or from
the `Child` of a `parserkit.Directory`, which `DirectoryAt` gives only for a plain
directory. Those refuse a name that is not one plain component and a link on the way
down, so a name read from the evidence cannot lead a parser out of the collection.
Every kit reader opens a regular file only, without following a link at the last element
and without waiting on a named pipe, and checks that the handle is the file it looked
at. `TestNoParserPackageBuildsAPath` in `internal/follow` fails on a path-building call
in a parser package; the kit is where those calls are made.

**Evidence JSON goes through the kit**, so an object that repeats a member name gives a
`parser:` row, and a repeat is reported once, by the parse of the file that holds it.
The tests that hold parsers to this are named in
[What the kit's tests hold every parser to in `internal/parsers/AGENTS.md`](../internal/parsers/AGENTS.md#what-the-kits-tests-hold-every-parser-to).

## Readers of evidence formats

| Package | What it owns |
| --- | --- |
| `internal/jsonvalue` | A JSON document as an ordered value: keys in document order, numbers as their source text, at most 10,000 levels deep without recursion; the member names an object repeats; `IsEmpty` and `Equal`. |
| `internal/tomldoc` | A TOML 1.1 document with its tables and keys in document order where the module's key list gives it, over `github.com/BurntSushi/toml`. |
| `internal/sqlitefile` | SQLite files read without SQL and without writing, with their `-wal` and `-journal` sidecars. |
| `internal/vscodestate` | Extension global state in a VS Code family `state.vscdb`, never its `secret://` rows. |
| `internal/protowire` | Protocol buffer messages from hand-written schemas of the fields a parser reads. |
| `internal/zstdframe` | zstd data, over the vendored `github.com/klauspost/compress/zstd`, with the window and output limits set once and a cut-off input told apart from a corrupt one. |
| `internal/gitmeta` | A collected repository's remotes and branch, from the text of `.git/config` and `.git/HEAD`; git is never run, since a repository found in evidence could run its hooks or configured programs. |
| `internal/textlines` | Lines ended at LF or CRLF only, and the ASCII sets the formats trim. |
| `internal/timeutil` | Timestamps in the one form the timeline writes, `YYYY-MM-DDTHH:MM:SS.mmmZ`, milliseconds cut, never rounded. |
| `internal/reason` | The short, platform-independent text of a read error for a `parser:` row or a `problem:` line. |

**The SQLite reader is read-only and reads the sidecars.** A database in a collection is
hostile input, and the evidence copy must not change. `internal/sqlitefile` reads pages
and records with bounds-checked Go, executes no SQL (`CREATE TABLE` text is read only for
the columns and their defaults) and never writes, so a database is read in place.
`database/sql` would need a driver: the cgo-free one most used is SQLite's C source
translated to Go and reads memory through `unsafe` throughout, a cgo driver would end the
static cross-compiled build, and either runs the database's own schema text through a SQL
engine. The reader sees what SQLite would see in a copy: a valid rollback journal
(`-journal`) is played back, and a valid write-ahead log (`-wal`) applied up to its last
commit, by the rules the package comment of `internal/sqlitefile` gives. The `-shm`
file, an index SQLite rebuilds, is never read, so it never stands in the way. A parser opens a database only through
`parserkit.OpenDatabase`, which gives the `parser:` row's wording for each failure
(`parserkit.DatabaseError`). Damage gives an error wrapping `sqlitefile.ErrMalformed`, never
a panic.

## Rows, deduplication and output

| Package | What it owns |
| --- | --- |
| `internal/timeline` | `Row`, the session summary, the marker texts of system rows, the groups of a repeated id (`Group`, `Attach`, `Choose`), shared texts (`SharedText`, `Resolve`), the stable sorts and the JSONL writers. |
| `internal/dedup` | Dropping secondary rows a primary row also records, one to one, within each source's time window, then the copies secondary sources share; the hooks an agent declares for a different match form or window. |
| `internal/rowfilter` | `--since`, `--until` and `--match` (Go regular expressions); a row is kept by string comparison of its timestamp. |
| `internal/secrets` | The recorder of what parsers identify as secret, and the replacement, where output is written, under `--no-secrets`. |
| `internal/secrets/urlcredentials` | The credentials of URLs inside longer text, by rules from RFC 3986. |
| `internal/textset` | Finding a fixed list of texts in one pass (an Aho-Corasick automaton), for the secret replacement. |

**Rows are the same in both secrets modes.** A parser returns text as written and only
identifies a secret through the kit (`parserkit.SecretText`, `SecretJSONText`,
`ReaderErrorText`); replacement happens once, in the writers, so grouping, joining,
deduplication, sorting and counting see the same values whatever the setting. The rule
and its limits are in [Analysis standards in `internal/parsers/AGENTS.md`](../internal/parsers/AGENTS.md#analysis-standards).

**Order of the analysis steps matters.** Deduplication runs on the rows as their groups
attach them (`timeline.Attach`), so a row its group leaves out neither takes a secondary
copy nor is taken. The parent each group goes with is chosen before deduplication, as the
first that the filter keeps (`timeline.Choose`), so a planted earlier row carrying a real
record's id cannot move the record's rows out of the window or give them its attributes.
`timeline.Resolve` runs only after filtering, so each shared text is written in full in
the first row of the output that names it.

## Stand-ins for the standard library

Where a package does something the standard library or a well-known module also offers,
its package comment says what it does differently; the reasons in short:

| Package | Stands in for | Why |
| --- | --- | --- |
| `internal/glob` | `path.Match`, `filepath.Match`, `filepath.Glob` | Shell patterns: `*` crosses `/` in `MatchPath`, matching by byte, a leading dot needs a dot, `[!...]` negation, no pattern is an error, ASCII case ignored on Windows, the root check asked before listing, the disk read only through `osfile` and `fsmeta`. |
| `internal/osfile` | `os` path functions | The `\\?\` form for every absolute path on Windows, so a name ending in a dot or space is the entry it names. |
| `internal/syspath` | `path/filepath` | A second, canonical path form kept beside the native one, `.` and `..` resolved by name, ASCII-only comparison keys and identity-aware sets. |
| `internal/fsmeta` | `os.Lstat`, `fs.FileInfo`, `os.Open` | Every field the manifest records on every platform, opening without following a link, and the access-time hold. |
| `internal/follow` | `os.Stat`, `os.ReadFile`, `filepath.EvalSymlinks`, `os.Root` | Following only after the root check, with a refusal that names the link. |
| `internal/walk` | `filepath.WalkDir`, `os.ReadDir` | Directory order rather than name order, and the root check before each directory. |
| `internal/evidencepath` | `path/filepath` | The rules of the machine that wrote the evidence, not of the host. |
| `internal/jsonvalue` | `encoding/json` into `any` | Key order and number text kept. |
| `internal/tomldoc` | `toml.Decode` into a map | Key order kept, and the TOML 1.1 rules on defining a key or table and on a date-time offset's range checked, which the module lets some documents break. |
| `internal/sqlitefile` | `database/sql` and a driver | No SQL, no write, no cgo or `unsafe`. |
| `internal/protowire` | `google.golang.org/protobuf` | No generated code for unpublished schemas; a damaged field costs only itself. |
| `internal/zstdframe` | the vendored `zstd` package used directly | Concurrency, window and output limits and the sorting of errors set once. |
| `internal/textlines` | `bufio.Scanner`, `strings.Split`, Unicode trims | Lines end at LF or CRLF only, ASCII trims, no line-length limit. |
| `internal/textset` | `strings.Contains`, `strings.Replacer` | One pass whatever the number of texts, telling which occur and the longest at each position. |
| `internal/timeutil` | `time.Parse` with `time.RFC3339` | The date forms agents write, and epochs read exactly from their digits. |
| `internal/rowfilter` | `time.ParseDuration`, `time.Parse` | Days, weeks, months and years with calendar arithmetic, and named periods. |
| `internal/reason` | `err.Error()` | One wording of each common condition on every platform. |
| `internal/regtext` | the Win32 string helpers | Embedded NULs kept, so a value is refused rather than shortened. |
| `internal/runlog` | `log` | The project's time layout from an injected clock. |
| `internal/hostcmd` | `os/exec` | A fakeable runner, system directories before `PATH`, the C locale, a bounded wait after a cancel. |
| `internal/secrets/urlcredentials` | `net/url` | Credentials found inside longer text, where `net/url` parses one URL. |

When the reason stops holding, the package goes
([Go standards](../internal/AGENTS.md#standard-library-and-readers)).

## Function pairs

A function whose work differs by platform is written once per platform file, under one
name and with one comment shape, rather than as a `runtime.GOOS` switch: the files end
`_unix.go`, `_linux.go`, `_darwin.go` or `_windows.go`, or carry a build constraint, and
code that is the same everywhere stays in the plain file. `excludedMeta` in
`walk/rows_other.go` and `walk/rows_windows.go`, or `RootCheck.Reached` in
`walk/rootrules_unix.go` and `walk/rootrules_windows.go`, or the methods of `stage.Tree`
in `stage/tree_unix.go` and `stage/tree_windows.go`, are such pairs. A change to one
side is checked against the other in the same commit, and every platform runs its own
side in CI.

## Tests that freeze the output

**The fake disk image.** `internal/collect/imagetest` builds, for tests only, the image
`collect` runs over, by replaying a committed manifest:
`internal/collect/imagetest/testdata/image_unix.txt` on Linux and macOS and
`image_windows.txt` on Windows. A committed tree or archive could not hold it: it has
names holding `"` and `\`, links, sockets, named pipes, shut directories, junctions and
deny rules. Each home holds only one operating system's paths and no two entries differ
only in letter case ([The fixture image](testing.md#the-fixture-image)); a home holding
both spellings of a directory is built on purpose by a fixture method. Each line is a quoted path, a kind, its
argument and attributes; the file is edited by hand, kept sorted, and its tests fail on a
line out of order or not as `Format` writes it. When a catalog line gains a fixture, add
its file line in sorted place.

**Expected collection output.** `TestCollectWritesItsGoldenOutput` in `internal/collect`
builds the program, runs each case over the image (always with `-r` on a root built for
the test) and compares every output with
`internal/collect/testdata/golden/<directory>/<case>/`: `manifest.jsonl`,
`collection.json`, `log.txt`, `stdout.txt`, the archive's member list and the exit code.
`unix/` is read on Linux and macOS, with `linux/<case>/` or `darwin/<case>/` holding only
how that platform differs; `windows/` on Windows. A file of a case may be a delta from
the default case's file (`name.delta`). Paths, times and other values that differ per
run are normalised before the comparison. `internal/walk` has the same kind of test over
an edge image (`TestEdgeImageRowsLogAndStagingTree`, `internal/walk/testdata/`).

**Expected analysis output.** `TestGoldenOutputOfTheAnalysisCommands` in
`internal/analyze` runs `detect`, `timeline` and `fleet` over the inputs under
`internal/analyze/testdata/golden-inputs/` and the committed example collection, and
compares stdout and each output file with `internal/analyze/testdata/golden/<case>/`;
only the working directory is normalised, to `{ROOT}`, and a file over 32 KiB is kept
gzip-compressed (read one with `gunzip -c`).

**Updating them.** `DOUBLEAGENT_UPDATE_GOLDEN=1` rewrites the expected files of every test
the `go test` run includes that compares them, the example blocks of
[`docs/timeline-output.md`](timeline-output.md) among them when `internal/analyze` runs
(`-run` narrows the set), and nothing else does:

```sh
export GOFLAGS=-tags=noasm,nounsafe
DOUBLEAGENT_UPDATE_GOLDEN=1 go test ./internal/collect -run TestCollectWritesItsGoldenOutput
git diff internal/collect/testdata
```

Read every changed line before committing: the diff is the change's effect on the
output. On Linux and macOS this rewrites only the current platform's view; the other
platform's files come from a run there (CI on the other runner, or a lab VM).

**The coverage list.** `TestEveryCatalogLineLeavesItsEvidence` runs the default,
`--no-secrets` and `--full` runs over the images and needs evidence of every table line
in the output, unless `internal/collect/testdata/coverage-uncovered.txt` gives the reason
the test derives for it: `none` (no image holds a path the line matches),
`unix-only` or `windows-only`, or `linux-unreached`, `darwin-unreached` or
`windows-unreached` (the image holds a matching path that no run on that platform
reaches). `DOUBLEAGENT_UPDATE_GOLDEN=1` rewrites this platform's reasons. A new line that
lands under `none` or an `-unreached` reason is a missing fixture, not an answer.

**Growth cases.** Every parser, for every kind of file it lists, ships growth cases in
`internal/growthtests/growth_<package>_test.go`: a function returning a `growthSet` (the
parser's name, its cases, and the kinds that need none, each with a reason), added to
`growthSets()` in registry order. A case names its kind and one of the shapes in
`growth_test.go` (many records naming one large value, a repeated id, one long carried
value, deep nesting and damaged siblings, many files that look at each other, equal join
keys, directories that cannot be listed), and builds the home's files at a size `n`.
`TestEveryParserGrowsInStepWithItsInput` parses each case at `n` and `2n` and fails when
the rows, their bytes, the allocations or the bytes allocated grow by more than the
limits [the parser rules](../internal/parsers/AGENTS.md#what-the-kits-tests-hold-every-parser-to) give;
`TestEveryFileKindHasGrowthCases` fails for a kind with no case and no exemption, and
`TestWantsIsTheUnionOfTheKinds` for a `Wants` that disagrees with the kinds. The measures
are counts, never times, so the result does not depend on the machine.

[`testing.md`](testing.md) has how to run the suites and the checks CI runs.
