# Inputs and detection

The analyzer reads whatever a responder has: an archive from either
collector, a collection already extracted, or a loose tree such as a
copied home directory or a mounted disk image. A collection carries a
manifest, so every file's host, user, home and agent come from it. A loose
tree carries nothing, so the analyzer finds homes and agents with the
collectors' own catalog and infers the user from the path, and its output
says that it did. A file or directory it cannot read is reported as a
problem, never skipped in silence.

## Inputs

| Input | How it is read |
| --- | --- |
| Any file, normally `*_agent-artifacts.tar.gz` or `.zip` from either collector | Recognised as zip or tar by its content (any compression `tarfile` reads), extracted to a temporary directory (or `--work-dir`), then read as an extracted collection. If it does not unpack to `manifest.jsonl` and `fs/`, it is read as a loose tree. A `.sha256` sidecar next to the archive is verified and the result reported as a note. Symlinks, hard links and device files inside the archive are not materialised (their targets are in the manifest), and members with absolute paths, `..` or a drive letter are skipped. A file that is neither tar nor zip is an error. |
| Extracted collection: a directory holding `manifest.jsonl` and `fs/` | Host name from `collection.json` `hostname` unless `--host` is given. User, home and agent for every file come from the manifest. Only rows with status `collected` and type `file` are read. |
| Anything else | Treated as a loose tree: a copied home directory, a mounted disk image, another collector's output, or one agent directory such as `.claude`. Homes and agents are discovered from the catalog, and the user is inferred from the path. A directory with `fs/` but no `manifest.jsonl` is read from `fs/`. |

In a collection, the bases listed in `collection.json` `projects`, and
the base of any manifest row with agent `project`, are project
directories rather than homes: their files are the collector's project
catalog files, and `inventory.json` marks those paths `collected` (see
[inventory.json](output.md#inventoryjson)). A manifest row with agent
`project`, which every row from a project directory had before collector
1.12.0, is re-attributed from the bundled project catalog to the agent
that owns the file (see [detect](output.md#detect)).

In loose mode the analyzer walks the tree and treats a directory as a home
when at least one catalog entry for a real agent matches under it. A
directory that holds only shared files such as `AGENTS.md` is a
project, not a home. A home is never nested inside another, so project-level
`.claude` directories inside a home are not mistaken for a second user. The
user is taken from `home/<user>`, `Users/<user>`, `usr/home/<user>`,
`export/home/<user>`, `root` or `var/root` in the path, falling back to the
directory name, and left empty when the input root itself is the home. When
the input is itself an agent directory, its name must match a one-segment
catalog entry such as `.claude` or `.codex`. Its parent is then the only
home, the user comes from the parent's path by the same conventions (with no
directory-name fallback), and `source_file` is relative to that parent.
Pass `--user` and `--host` to fill in what the path cannot say. Loose-mode
output is marked `inferred` in `detect --json` because none of this comes
from a manifest. Exclusions do not apply in loose mode: every file under a
matched catalog path is attributed and offered to the parsers.

## Unreadable inputs

An input that cannot be opened stops `detect`, `timeline` and `inventory`
with exit code `2` and one line on stderr, `error: <path>: <reason>`, where
the reason is the operating system's message in lower case, such as
`no such file or directory` or `permission denied`. The path is the file
the operating system named, so it is the input itself, or `manifest.jsonl`
inside it, or the archive member being extracted. This covers an input that
cannot be stat'ed (including one inside a directory the analyst cannot
traverse), an input directory that cannot be listed, an archive that cannot
be read or written to the work directory while it is extracted, and a
`manifest.jsonl` that cannot be read. An archive whose content is corrupt
gives `error: <path>: cannot extract: <reason>`. An extracted archive is
removed before the command exits.

Anything inside an input that cannot be read is a `problem:` line and the
command carries on:

| Problem line | When |
| --- | --- |
| `<directory>: <reason>` | A directory in a loose tree that cannot be listed, while homes are discovered or files under a catalog match are walked. Reported once per directory; nothing under it is attributed. |
| `<original path>: missing from the collection` | A manifest row with status `collected` and type `file` whose `archive_path` is not a regular file in the extracted collection, for example in a truncated or edited archive. The row is not attributed. Any other reason the file cannot be stat'ed is printed instead. |
| `<original path>: <reason>` | `timeline` only: a read error a parser let escape. Most parsers report a file they cannot read as a `parser:` row in the timeline naming the file and the reason instead (the shared JSONL and SQLite readers write `<file> could not be read (<reason>)`; see [Output](output.md)). |

`detect` prints them after the notes, and `timeline` before its `window:`
line. Symlinks are never followed, so a dangling symlink in a loose tree is
skipped like any other symlink rather than reported. An unreadable
`collection.json` is a note, `collection.json unreadable: <reason>`, and an
unreadable `.sha256` sidecar is the note `WARNING archive sha256 not
checked: <path>: <reason>`.

## Detection

Detection uses the collectors' own catalog. `doubleagent/catalog.txt` is a
verbatim copy of `collect-agent-artifacts.sh --list`, made with:

```sh
collectors/collect-agent-artifacts.sh --list > analyzer/doubleagent/catalog.txt
```

Every agent the collectors know is therefore detected, whether or not a
parser exists for it. When two entries match the same file, the more specific
one wins, so Antigravity CLI state under `.gemini/antigravity-cli` is
attributed to `antigravity`, not to the enclosing `gemini-cli` entry, in both
loose mode and the collectors' manifests. `doubleagent catalog --agents`
lists the agents and which have parsers.

### Routing files to parsers

Every file goes to the parsers of the agent the manifest (or, for loose
input, the catalog) attributes it to, plus any parser that names that agent
in its `reads_agents` attribute. `secret://` rows are never read.

Files the collector finds inside a project directory through its project
catalog have the project directory as their manifest `home`, which the
analyzer marks as a project base (see [Inputs](#inputs)). From collector
1.12.0 they carry the agent the project catalog names for them, a
repository's `.aider.chat.history.md` `aider` and its `.crush/crush.db`
`crush`, and go to that agent's parsers like any other file, so
`--agent aider` and `--agent crush` include them; the rows of older
collectors are re-attributed as [Inputs](#inputs) describes. A file the
catalog leaves to agent `project`, because several agents read it
(`AGENTS.md`), is offered to every parser, and the rows of any parser that
accepts it are kept under that parser's agent. A parser that scopes a row
to the project, such as `claude-code` for a project's
`.claude/settings.json`, reads the project base marker, not the agent.

A project's git metadata, collected from collector 1.19.0 (see
[Git metadata in `collectors/docs/coverage.md`](../../collectors/docs/coverage.md#git-metadata)),
is in the same group: its rows have agent `project`, and no parser
wants them. The analyzer reads two of those files itself, from a project
base only: the remotes of `.git/config` and the branch or commit of
`.git/HEAD`, which `detect --json` and `inventory.json` give per project
as `remotes` and `head` (see [detect --json](output.md#detect---json)).
One of the two files the analyzer cannot read is a `problem:` line, as
for any other unreadable file (see [Unreadable inputs](#unreadable-inputs)),
and leaves the project's `remotes` empty. A loose input has no project
bases, so these stay empty there.

Cody and Twinny keep their chats as rows of the editor's own `state.vscdb`,
which the catalog attributes to `vscode` or to the fork whose `User/`
directory holds it (`cursor`, `windsurf`, `pearai`, `kiro`, `antigravity`);
both parsers list all six editors in `reads_agents`, so that file is offered
to them too and the rows come out under `cody` or `twinny`, and
`--agent cody` selects them.
The ChatGPT desktop app keeps its list of the account's ChatGPT
conversations in `.codex/sqlite/codex-dev.db`, which the catalog
attributes to `codex-cli` because it sits under `.codex`; the
`chatgpt-desktop` parser lists `codex-cli` in `reads_agents`, so its
rows come out under `chatgpt-desktop`, and `--agent chatgpt-desktop`
selects them.
