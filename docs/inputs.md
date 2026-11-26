# Inputs and detection

`doubleagent detect` and `doubleagent timeline` read whatever a responder
has: the archive a collection run wrote, that archive already extracted,
or a loose tree such as a copied home directory or a mounted disk image. A
collection carries a manifest, so each file's host, user, home and agent
come from it. A loose tree carries nothing, so the commands find homes and
agents with the catalog (the table of agents and the paths under a home
directory where each keeps its state) and infer the user from the path,
and the output says that they did. A file or directory that cannot be read
is reported, never skipped in silence.

## Inputs

| Input | How it is read |
| --- | --- |
| A file: normally the `<host>_<time>_agent-artifacts.tar.gz` a collection run wrote | Recognised by its first bytes, not its name: a tar archive, plain or compressed with gzip or bzip2, or a zip archive. An xz- or zstd-compressed tar is refused. It is extracted into a new directory (see [The work directory](#the-work-directory)), then read as an extracted collection when it unpacked to `manifest.jsonl` and `fs/` at its top, and as a loose tree otherwise. A file that is neither a tar nor a zip archive is an error. |
| An extracted collection: a directory holding a regular file `manifest.jsonl` and a directory `fs/`, neither a symbolic link | Read through its manifest, as [With a manifest](#with-a-manifest) describes. |
| Anything else | Read as a loose tree: a copied home directory, a directory of homes, a mounted disk image, or one agent directory such as `.claude` copied on its own, as [Without a manifest](#without-a-manifest) describes. A directory with `fs/` but no `manifest.jsonl` is read from `fs/`, with the note `fs/ found without manifest.jsonl; treating fs/ as the root`; one whose `manifest.jsonl` is not a regular file is read as a loose tree with the note `manifest.jsonl is not a regular file; reading the input as a loose tree`. |

An input that is neither a regular file nor a directory, such as a device
or a named pipe, is an error. The input path itself may be a symbolic link,
which is followed because you named it; no link inside the input is.

### Archives

When a file `<archive>.sha256` lies beside the archive, the archive's
SHA-256 digest is compared with the first word of that file (the form
`doubleagent collect` writes) and the result is a note:

| Note | When |
| --- | --- |
| `archive sha256 verified against <name>.sha256` | The digests agree. |
| `WARNING archive sha256 <digest> does not match <name>.sha256 (<expected>)` | They differ: the archive is not the one the collection run hashed. |
| `WARNING archive sha256 not checked: <path>: <reason>` | The `.sha256` file cannot be read. |

Extraction makes only directories and regular files, each file created
new:

- A tar member that is a symbolic link, a hard link, a device or a named
  pipe is not made; the manifest records a link's target. A zip member is
  always written as a plain file. Each file, and each tar directory, gets
  the modification time its member records. A zip member's time comes from
  its NTFS, UNIX or extended timestamp field; one that records only the
  MS-DOS date and time, which hold no time zone, is read as UTC.
- A backslash in a zip member's name always separates folders, whatever
  system made the zip; on Windows it does in a tar member's name too.
  The members whose names hold no backslash are extracted first, then
  those whose names hold one, each group in archive order; on Windows a
  tar holding such a name is read twice. So where a member whose name is a
  path with slashes (`a/b`) and one that reaches it only through a
  backslash (`a\b`, `a\.\b`, a folder `a\b/`, or `a\B` on a file system
  that folds case) meet, the one named with slashes is kept, whatever
  order the archive lists them in; between two that both need a
  backslash, the first in the archive is. Each other member is skipped with the problem line below. A
  later member with the very same name replaces the earlier, as in an
  archive appended to.
- A member name loses its leading slashes and its empty and `.` parts. A
  member whose name would lead out of the extraction directory (it holds a
  `..` part or starts with a drive letter), names no file, or holds a part this
  system cannot use as a file name is skipped, with the problem line
  `<archive>: member <name> not extracted: <reason>`. So is a member whose
  place already holds what another member made, as a file system that
  folds case can give two names one entry.
- A sparse tar member, a member of an unknown type, an encrypted zip member
  and a damaged archive stop the command with
  `error: <archive>: cannot extract: <reason>`.

### With a manifest

- **Host**: `collection.json` `hostname`, unless `--host` names another.
- **User, home and agent**: each file's own manifest row (`user`, `home`,
  `agent`), with the file's original path as `source_file` in the
  timeline. Only rows with status `collected` and type `file` are read;
  the rows of skipped files, symbolic links and directories describe what
  the collection run met and are not offered to the parsers. A manifest
  line that is not a JSON object, or a `collected` file row whose `size`
  is not a JSON integer, is counted in the note
  `<n> manifest rows did not parse` (`row` for one).
- **Projects**: the directories `collection.json` lists under `projects`,
  and the home of any row whose agent is `project`, are project
  directories rather than homes. Their files are the project files the
  collection run gathered there, and `inventory.json` marks those projects
  `collected` ([inventory.json](timeline-output.md#inventoryjson)). A row
  whose agent is `project` is given the agent that owns its path in the
  project part of the catalog, such as `aider` for `.aider.chat.history.md`;
  a file no single agent owns, such as `AGENTS.md`, keeps `project`.
- **Paths**: a backslash in a row's `path`, `home` or `archive_path`
  separates folders only when the row comes from a Windows host (its
  `home` or `path` starts with a drive letter or `\\`); in a row from
  Linux or macOS it is part of the file's name. In a collection that came
  from a zip, a backslash in a row's `archive_path` always separates
  folders, as it does in the zip's member names, so the row names the file
  extraction made; on Windows a backslash in `archive_path` separates
  folders in every collection. A row's `archive_path` loses its empty and
  `.` parts, as a member name does. Where extraction skipped a member
  because its name and the kept member's name one file, only the row whose
  `archive_path`, read with slashes alone, is the kept member's name is
  given the file; the other row is reported `missing from the collection`.
  A file another file names by its path, such as a transcript's saved tool
  output, is read from the kept member, which is the one whose name has
  slashes alone unless no such member was there.
- `--user` names the user of any home whose row gives none.

### Without a manifest

The tree is walked from the top, each directory's entries in byte order of
their names, so one tree gives one result on every file system. A directory
is a home when it holds a match of a catalog entry of a real agent; one
that holds only files many tools read, such as `AGENTS.md`, is a project,
not a home. A home is not entered further, so a project's own `.claude`
directory inside a home is not mistaken for a second user.

- **User**: the directory name below `home`, `Users`, `usr/home` or
  `export/home` in the path below the input, or `root` for `root` and
  `var/root`; otherwise the home directory's own name (unless it is `fs`).
  When the input itself is the home, the user is empty. `--user` fills an
  empty user.
- **Home**: the home's path below the input, with a leading `/`, such as
  `/home/alice`.
- **Host**: empty, shown as `(unknown)`, unless `--host` names one.
- **One agent directory**: when the input's own name matches a catalog
  entry of one path element, such as `.claude` or `.codex`, its parent is
  the only home, with the note `root is a <agent> directory; treating its
  parent as the home`. The user comes from the parent's path by the rules
  above, without the fallback to the directory's name, and only the agent
  directory is read, nothing beside it.

`detect` prints the kind as
`loose (no manifest; user and home inferred from paths)`, and
`detect --json` and `inventory.json` mark each such home `inferred`.
No exclusion applies to a loose tree: every file under a matched catalog
path is attributed and offered to the parsers.

## The work directory

An archive is extracted into a new directory named `doubleagent-` and a
random number, made in the system's temporary directory, or inside the
directory `--work-dir` names, which is created when it is missing. The
extracted directory is removed when the command ends, and so are the
directories made for `--work-dir`, while they are empty. With
`--keep-extracted` it stays, and its path is printed on stderr as
`extracted to: <path>`; an interrupted run removes it anyway. An extracted
collection or a loose tree is read where it is.

An archive needs room for its extracted files in the work directory; give
`--work-dir` on a larger disk when the temporary directory is small.

## What is never followed or written

- **Nothing in the input is changed.** Files are opened read-only, and an
  archive is extracted elsewhere.
- **No symbolic link inside the input is followed.** The walk of a loose
  tree does not enter a linked directory and does not read a linked file;
  a path built from the manifest or from a name in the evidence is refused
  when it would pass through a link, and on Windows through any reparse
  point. A dangling link is therefore skipped like any other link, not
  reported. On Windows, a directory the system redirects elsewhere, such
  as a cloud file placeholder, is not entered and is reported as a
  problem.
- **Writes go only where you point them**: the `timeline` output directory
  (which must be outside the input), the `fleet` CSV, and the work
  directory.

## Unreadable inputs

An input that cannot be opened stops `detect`, `timeline` and `fleet` with
exit code `2` and one line on stderr, `error: <path>: <reason>`, where the
reason is the operating system's message, such as
`no such file or directory` or `permission denied`. The path is the file
the operating system named: the input itself, `manifest.jsonl` inside it,
or the archive being extracted. This covers an input that cannot be
looked at (including one inside a directory you cannot enter), an input
directory that cannot be listed, an archive that cannot be read or
extracted into the work directory, and a `manifest.jsonl` that cannot be
read. A damaged archive gives `error: <path>: cannot extract: <reason>`.
Whatever was extracted is removed before the command exits.

Anything inside an input that cannot be read is a `problem:` line, and the
command carries on:

| Problem line | When |
| --- | --- |
| `<directory>: <reason>` | A directory of a loose tree that cannot be listed, while homes are found or the files under a catalog match are walked. Reported once per directory; nothing under it is attributed. |
| `<original path>: missing from the collection` | A manifest row with status `collected` and type `file` whose archived copy is not a regular file in the collection, as in a truncated or edited archive. The row is not attributed. Any other reason its copy cannot be looked at is printed in place of `missing from the collection`. |
| `<original path>: home <home>: <reason>` | A manifest row whose home cannot be placed inside the collection. |
| `<original path>: a path longer than 4096 bytes, not read`, and `more than 4096 path components, not read` | A manifest row whose path is too long or too deep to be a real file. |
| `manifest.jsonl line <n>: longer than <limit> bytes, not read` | A manifest line too long to be a row. |
| `manifest.jsonl: <n> member names repeat within their JSON objects (<name> <k> times, ...); the last value of each is used`, and the same for `collection.json` | A manifest row or the collection summary gives one member name twice within an object; the last value is the one read. One line per file however many rows repeat a name, or `1 member name repeats within its JSON object (<name>); the last value is used` for a single one. |
| `<archive>: member <name> not extracted: <reason>` | An archive member skipped for its name, as [Archives](#archives) describes. |
| `<original path>: <reason>` | `timeline` only: a parse that failed part-way. Most parsers report a file they cannot read as a `parser:` row in the timeline instead, naming the file and the reason ([Output](timeline-output.md)). |

`detect` prints them after its notes, and `timeline` before its `window:`
line. An unreadable `collection.json` is the note
`collection.json unreadable: <reason>`, and the host is then empty unless
`--host` names one.

## Detection

Detection uses the catalog built into `doubleagent`, the same one a
collection run gathers files by; `doubleagent catalog` prints it and
`doubleagent catalog --agents` lists its agents with `parser` or
`detect only`. Every agent in the catalog is therefore detected, whether or
not a parser reads it. When two entries match the same file, the more
specific one wins, so Antigravity CLI state under `.gemini/antigravity-cli`
is attributed to `antigravity`, not to the enclosing `gemini-cli` entry, in
a loose tree as in a collection. A loose tree is matched by the rules a
collection run uses: ASCII case is ignored, so `.config/Goose` matches the
line `goose|.config/goose`, and a `*` or `?` never matches a name's leading
dot.

### Routing files to parsers

Every file goes to the parsers of the agent the manifest (or, in a loose
tree, the catalog) attributes it to, and to the parsers of other agents
that read that agent's files:

- The Cody, Twinny, Tabby and Roo Code parsers read an editor's own
  `User/globalStorage/state.vscdb`, which the catalog attributes to
  `vscode` or to the editor fork whose `User/` directory holds it
  (`cursor`, `windsurf`, `pearai`, `kiro`, `antigravity`). Their rows come
  out under their own agent, so `--agent cody` selects them.
- The ChatGPT desktop app keeps its list of the account's conversations in
  `.codex/sqlite/codex-dev.db`, which the catalog attributes to `codex-cli`
  because it sits under `.codex`; the `chatgpt-desktop` parser reads it,
  and its rows come out under `chatgpt-desktop`.
- The `ollama` parser reads the profile Ollama writes under a Claude
  Desktop directory, which the catalog attributes to `claude-desktop`.

A file whose agent is `project`, because several agents read it, is
offered to every parser, and the rows of any parser that accepts it are
kept under that parser's agent. A parser that scopes a row to the project,
such as `claude-code` for a project's `.claude/settings.json`, goes by
whether the file's base is a project directory, not by the agent.

A project's git metadata, which a collection run gathers (see
[Git metadata](coverage.md#git-metadata)), is in the same group: its rows
have agent `project`, and no parser wants them. The commands read two of
those files themselves, from a project directory only: the remotes of
`.git/config` and the branch or commit of `.git/HEAD`, which
`detect --json` and `inventory.json` give per project as `remotes` and
`head` ([detect --json](timeline-output.md#detect---json)). Either file
that cannot be read is a `problem:` line and leaves the project's
`remotes` empty. A loose tree has no project directories, so these stay
empty there.
