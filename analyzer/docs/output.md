# Output

The analyzer answers two questions about an input, and each has its own
command and output. `detect` says which agents left state in it, for
which users, and whether a parser exists for each, on stdout; nothing is
written. `timeline` reads every transcript it can parse and writes three
files into `-o`: `timeline.jsonl`, one record per turn, tool call, tool
result or system event; `sessions.jsonl`, one record per session; and
`inventory.json`, what was detected and parsed. The JSONL records are the
interface for everything downstream, so their fields, order and types are
fixed and documented in full here, and a record always has every field.

## The example collection

The examples of [detect](#detect), [detect --json](#detect---json),
[timeline](#timeline), [inventory.json](#inventoryjson),
[timeline.jsonl](#timelinejsonl) and [sessions.jsonl](#sessionsjsonl) are
the analyzer's output on a synthetic collection. No collector gathered
it from a host: `analyzer/tools/example_collection.py` writes the agents'
files with the analyzer's test fixtures, the same records the parser
tests read, so the examples show what the parsers make of the record
shapes the research documents describe, from real installs and the
agents' source. The contents are invented. A fixture covers every
platform its agent writes for, and a Linux home holds no macOS
`Library` or Windows `AppData` tree, so the tool leaves those trees out,
except Cursor's editor state, which it moves to its Linux place,
`.config/Cursor`. The tool writes `collection.json`, `manifest.jsonl`
and `collector.log` itself, in the shape
[`collectors/docs/output.md`](../../collectors/docs/output.md) documents,
as the collector of this repository, whose version they carry, would
record `wsdev14`, a Linux workstation, in live mode on 2026-10-15, with
the users `alice` and `bob` and the repository `/srv/proj`. A file whose
fixture sets no modification time has 15:56:10 UTC that day, an hour
before the collection.

- alice's Claude Code files: transcripts of several builds whose tool
  calls are `tool_use` rows joined to their `tool_result` rows on
  `tool_use_id`, with `permission decision:` rows beside the calls,
  `permission mode:` rows (one, from a state record that carries no
  time, with no timestamp), `/model` commands as `slash command:` rows
  and `model: <old> -> <new>` rows on a switch, and scratchpads as
  `scratch directory:` rows (see
  [Row order and system rows](#row-order-and-system-rows)). A session
  bound to Remote Control gives `remote control: bridge session` and
  `session synced: remote control bridge` rows, and the registry file
  of another session's process under `.claude/sessions/`, still
  running, one more `session synced:` row. A session the
  Remote Control server spawned gives
  `session origin: remote (Remote Control, entrypoint sdk-cli)`, its
  `ccr-tip.json` a `session synced: remote event` row, and the project's
  `bridge-pointer.json` a `remote control: environment` row that belongs
  to no session. A cloud session brought to the host with
  `claude --teleport` gives `session origin: remote (teleport, ...)`
  and `session synced: cloud session (teleported: ...)` rows.
  `.claude/history.jsonl` repeats prompts the transcripts hold, which are
  dropped as duplicates and counted on the `dropped:` line (see
  [Secondary sources](#secondary-sources)), and keeps one of a session
  whose transcript is gone, in `/srv/old`, with `recovered` `yes`.
  `.claude/settings.json` gives `permission setting:` rows with an empty
  `project_path`, and `.claude.json` rows for `/srv/proj`.
- alice's Codex CLI files: a rollout with a `git remote:` row naming the
  repository's URL, `model:` and `permission mode: approval=...
  sandbox=...` rows and an `apply_patch` call, its history, the state
  database, whose `threads` rows give `thread record:` rows, and the
  thread history database, whose turns of a thread with no rollout read
  `recovered` `yes`.
- alice's Cursor files: the editor's state database, with
  `source host: github` rows and cloud agents whose sessions read
  `origin` `remote`; the CLI's settings, as `permission setting:` rows;
  and agent transcripts, some of whose rows read `recovered` `yes`, one
  whose last line was cut mid-write, as a copy taken while the agent
  runs can catch it: a `parser: unparseable line` row with an empty
  `timestamp_utc`, so it sorts after every timed row, with every row
  before it kept.
- alice's Aider history files in her home, and an Amp settings file:
  Amp has no parser, so it is under `detected but not parsed`.
- bob's Gemini CLI and Ollama files. A Gemini CLI chat the manifest
  lists as collected is missing from `fs/`, which gives a `problem:`
  line, and the manifest row of an Ollama model manifest is cut short,
  which gives the `manifest rows did not parse` note. The Ollama prompt
  history records no times, so its `user` and `slash command:` rows have
  an empty `timestamp_utc`.
- The files of `/srv/proj`: `.git/HEAD` and `.git/config`, which names
  the remotes `origin` and `upstream`, are shared files, and
  `.claude/settings.local.json` gives `permission setting:` rows with
  `project_path` `/srv/proj`.
- Two excluded directories, alice's `.claude/cache` and bob's
  `.ollama/models/blobs`, are `skipped_excluded` rows of a used install's
  size.

The [timeline.jsonl](#timelinejsonl) example is not the whole run's file
but that of a second, filtered run, `timeline -o out --agent claude-code
--agent codex-cli --agent cursor --match <regex>`, where the regular
expression picks the row forms the examples point out, so the example
stays short enough to read; the [timeline](#timeline) stdout,
[inventory.json](#inventoryjson) and [sessions.jsonl](#sessionsjsonl)
examples are the unfiltered run's. The regular expression is
`TIMELINE_FILTERS` in `analyzer/tools/regenerate_examples.py`.

To write the collection into an empty directory, here `example`, from
the repository root:

```sh
mkdir example
python3 analyzer/tools/example_collection.py example
```

The collection is `example/wsdev14/`, with `collection.json`,
`manifest.jsonl`, `collector.log` and `fs/`, the input for `detect` and
`timeline`. The
examples show its path as `/cases/wsdev14/wsdev14` and the `timeline`
output directory as `out`.

## detect

`detect` prints the input, its kind (`archive`, `collected` for an extracted
collection, or `loose`), the host, any notes, and up to two tables: one
line per user, home and agent outside any project directory, and, when the
input holds project directories, one line per agent of each project. Both
give the number of files, their bytes (from the manifest `size`, or from
disk in loose mode), and whether a parser exists. The notes cover the
archive hash check, unreadable `collection.json`, manifest rows that do not
parse, and how a loose root was interpreted. One `problem:` line follows the
notes for each directory or file inside the input that could not be read
(see [Unreadable inputs](inputs.md#unreadable-inputs)). `no agent artifacts
found` is printed, with exit code `1`, only when both tables are empty.
`--files` adds one `user<TAB>agent<TAB>path` line per attributed file.
Collections made by collectors before 1.6.0 also hold a `live/` system
snapshot, which appears as agent `live` with no user, and `detect only`.

The first table has the columns `user`, `home` (cut to 32 characters),
`agent`, `files`, `bytes` and `parser` (`yes` or `detect only`), in the
order each user, home and agent's first file was met. Its homes are user
homes and Docker volumes, never a project directory.

Files the collector gathered from a project directory (a repository that
agent state referenced, or one given with `-p`) are listed in the second
table under the agent that owns them: a repository's `.claude/` under
`claude-code`, its `.aider.chat.history.md` under `aider`. Collectors
before 1.12.0 recorded every project file with agent `project`; for such a
collection the analyzer names the agent itself, from the project catalog
in its bundled catalog copy (the `# project artifacts` section of
`doubleagent catalog`): the most specific entry whose glob matches the
file's path relative to the project directory, so an old and a new
collection of the same host list the same agents. Nothing is printed about
the re-attribution.

Some project files belong to no single agent, because several agents read
the same file. These are the shared files: the collector records them
with agent `project`, and the tables count them on the project's
`(shared)` line. They are the `project` lines of the project catalog, and
[`research/shared.md`](../../research/shared.md)
and the per-agent research documents cite each reader:

- `AGENTS.md`, the repository instruction file read by Codex CLI, Copilot
  CLI, Amp, OpenCode, Crush, Factory Droid, Kilo Code, Zed and pi, and
  its variants `AGENTS.override.md` (Codex CLI, pi, Hermes) and
  `AGENT.md` (Amp, Zed).
- `.agents/`, the cross-tool skills directory that Codex CLI, Copilot
  CLI, Amp and Muse Code read.
- `.mcp.json`, the project's list of MCP (Model Context Protocol) servers,
  read by Claude Code, Copilot CLI and Muse Code.
- `.github/copilot-instructions.md`, `.github/git-commit-instructions.md`
  and `.github/instructions/`, the GitHub Copilot instruction files, read
  by the Copilot editor extensions, Copilot CLI, Crush and Zed.
- `SOUL.md` and `USER.md`, the persona and user-profile files of an
  OpenClaw or nanobot workspace; Hermes reads `SOUL.md` too.
- The repository's git metadata, from collector 1.19.0: `.git/HEAD`,
  `.git/config`, `.git/FETCH_HEAD`, `.git/packed-refs`, `.git/refs/`,
  `.git/logs/`, `.git/worktrees/` and `.git/info/exclude`, the
  checked-out branch, the remotes, the branches, the reflog and the
  local ignore rules, which say where the code came
  from and where it may have been pushed rather than which agent worked
  there. [Git metadata in `collectors/docs/coverage.md`](../../collectors/docs/coverage.md#git-metadata)
  says what each file holds and what under `.git/` is not collected.

A project file no project catalog entry matches is counted as shared as
well. `project` is not an agent, so it never appears as an agent in
either table, and a shared file cannot say which agent used it: a
repository holding only `AGENTS.md` shows that some agent was set up
there, not which one. The per-agent lines are the evidence of a
particular agent.

The project directories are the homes the analyzer marks as projects: the
paths in `collection.json` `projects` and, for a collection from a
collector before 1.12.0, the `home` of each collected row with agent
`project`. A loose input holds none, so the second table is not printed
there, nor for a collection without project files. The table follows the
first after a blank line, sorted by project path.

This is the output of `detect` on
[the example collection](#the-example-collection). The cut manifest row
gives the `note:` line and bob's missing Gemini CLI chat the `problem:`
line. The first table holds alice's and bob's homes, with `amp` marked
`detect only`; the second holds `/srv/proj`, with a line for each agent
that left files there, a `(shared)` line for its `.git/` files, and the
`remotes:` line its `.git/config` gives:

<!-- generated by analyzer/tools/regenerate_examples.py: detect-text -->
```text
input:   /cases/wsdev14/wsdev14
kind:    collected
host:    wsdev14
note:    1 manifest rows did not parse
problem: /home/bob/.gemini/tmp/6b9af143446a411aefa3edb76e4693799bd7513e7ed8d919962d6713d3d3b972/chats/session-2026-09-30T08-00-b2c3d4e5.json: missing from the collection

user           home                             agent                 files        bytes  parser
alice          /home/alice                      claude-code              23        40040  yes
alice          /home/alice                      codex-cli                 8        59166  yes
alice          /home/alice                      cursor                   11       100667  yes
alice          /home/alice                      aider                     4         1379  yes
alice          /home/alice                      amp                       1           96  detect only
bob            /home/bob                        gemini-cli                9         4598  yes
bob            /home/bob                        ollama                    6          300  yes

project                                  users          agent                 files        bytes  parser
/srv/proj                                alice          claude-code               1           70  yes
                                                        (shared)                  2          486
  remotes: origin git@github.com:alice/proj.git https://mirror.example.com/alice/proj.git (push https://alice:FIXTURE-TOKEN@github.com/alice/proj.git), upstream https://github.com/upstream/proj.git
```
<!-- end generated: detect-text -->

Each project has one line per agent, sorted by agent, then a `(shared)`
line when it holds shared files, with an empty `parser` column. The path
(cut to 40 characters) and `users` (the users whose manifest rows name the
directory, sorted and joined by `,`) are printed on the project's first
line only. A project whose `.git/config` names remotes gets one more
line after its last (since 0.21.0): `  remotes: ` followed, per remote
sorted by name and joined by `, `, by the name and its `url` values
separated by spaces, then ` (push <pushurl> ...)` when the remote sets
push URLs, as in `  remotes: origin git@git.example.com:alice/proj.git
https://mirror.example.com/alice/proj.git (push
https://git.example.com/alice/proj.git), upstream
https://git.example.com/upstream/proj.git`. In the example above it is the
last line, naming the `origin` and `upstream` remotes of `/srv/proj`.

### detect --json

With `--json`, `detect` prints the same data as one JSON object,
indented by two spaces, and exits `0` even when nothing was found. This
example is the output of `detect --json` on
[the example collection](#the-example-collection), the data of the
[detect](#detect) example with each project's `remotes` and `head`. With
`--files` it also holds the `files` key, one entry per attributed file,
which the key table below describes:

<!-- generated by analyzer/tools/regenerate_examples.py: detect-json -->
```json
{
  "input": "/cases/wsdev14/wsdev14",
  "kind": "collected",
  "host": "wsdev14",
  "notes": [
    "1 manifest rows did not parse"
  ],
  "homes": [
    {
      "user": "alice",
      "home": "/home/alice",
      "inferred": false,
      "agents": [
        {
          "agent": "aider",
          "files": 4,
          "bytes": 1379,
          "parser": true
        },
        {
          "agent": "amp",
          "files": 1,
          "bytes": 96,
          "parser": false
        },
        {
          "agent": "claude-code",
          "files": 23,
          "bytes": 40040,
          "parser": true
        },
        {
          "agent": "codex-cli",
          "files": 8,
          "bytes": 59166,
          "parser": true
        },
        {
          "agent": "cursor",
          "files": 11,
          "bytes": 100667,
          "parser": true
        }
      ]
    },
    {
      "user": "bob",
      "home": "/home/bob",
      "inferred": false,
      "agents": [
        {
          "agent": "gemini-cli",
          "files": 9,
          "bytes": 4598,
          "parser": true
        },
        {
          "agent": "ollama",
          "files": 6,
          "bytes": 300,
          "parser": true
        }
      ]
    }
  ],
  "projects": [
    {
      "path": "/srv/proj",
      "users": [
        "alice"
      ],
      "agents": [
        {
          "agent": "claude-code",
          "files": 1,
          "bytes": 70,
          "parser": true
        }
      ],
      "shared_files": 2,
      "shared_bytes": 486,
      "remotes": [
        {
          "name": "origin",
          "urls": [
            "git@github.com:alice/proj.git",
            "https://mirror.example.com/alice/proj.git"
          ],
          "pushurls": [
            "https://alice:FIXTURE-TOKEN@github.com/alice/proj.git"
          ]
        },
        {
          "name": "upstream",
          "urls": [
            "https://github.com/upstream/proj.git"
          ],
          "pushurls": []
        }
      ],
      "head": "main"
    }
  ],
  "problems": [
    "/home/bob/.gemini/tmp/6b9af143446a411aefa3edb76e4693799bd7513e7ed8d919962d6713d3d3b972/chats/session-2026-09-30T08-00-b2c3d4e5.json: missing from the collection"
  ]
}
```
<!-- end generated: detect-json -->

`inferred` is `true` only in loose mode, where `projects` is always
`[]`, so no single run shows both. The `remotes` and `head` of
`/srv/proj` come from its collected `.git/config` and `.git/HEAD`. Bob's
missing Gemini CLI chat is left out of his `gemini-cli` counts and named
under `problems`. `homes` holds the counts of the first table of
`detect` grouped by home, with each home's agents sorted by name, where
the table lists them in the order their first file was met: here
alice's `aider` comes before `claude-code` whatever the manifest's order.
Its keys, in order:

| Key | Type | Meaning |
| --- | --- | --- |
| `input` | string | The input path as given on the command line, as on the `input:` line. |
| `kind` | string | `archive` (an archive the analyzer extracted), `collected` (an extracted collection: `manifest.jsonl` and `fs/`) or `loose` (any other directory, or an archive that did not unpack to a collection). |
| `host` | string | `--host` when given, else `collection.json` `hostname`, else `""`. |
| `notes` | array of strings | The `note:` lines without their prefix, in the order they were made: the archive sha256 check (`archive sha256 verified against <name>`, or a line starting `WARNING`), `collection.json unreadable: <reason>`, `<n> manifest rows did not parse`, `fs/ found without manifest.jsonl; treating fs/ as the root`, `root is a <agent> directory; treating its parent as the home`. `[]` when there are none. |
| `homes` | array of objects | The first table, grouped by home: one per user home and Docker volume, in the order its first collected row was met; project directories are left out (they are `projects`). |
| `homes[].user` | string | The home's owner: the manifest `user`, or in loose mode the user inferred from the path; `--user` when neither names one; `""` when unknown. |
| `homes[].home` | string | The manifest `home`, the path on the source host: a user's home or a Docker volume's `_data`, never a project directory. In loose mode the home's path relative to the input root with a leading `/`, or `""` when the input root is the home or an agent directory; the text table prints the home's path on the analyst's disk there instead. |
| `homes[].inferred` | boolean | `true` in loose mode, where user and home come from the path; `false` when they come from a manifest. |
| `homes[].agents` | array of objects | One per agent with at least one file in this home, sorted by `agent`; `[]` when every file of the home is missing from the collection. Never holds `project`. |
| `homes[].agents[].agent` | string | Catalog agent name. |
| `homes[].agents[].files` | integer | Files attributed to this agent in this home: collected manifest rows of type `file` whose file is present under `fs/`, or files under a matched catalog path in loose mode. |
| `homes[].agents[].bytes` | integer | Their total size: the sum of the manifest `size` values, or of the sizes on disk in loose mode. |
| `homes[].agents[].parser` | boolean | `true` when the analyzer has a parser for this agent, `false` when it is detected only. |
| `projects` | array of objects | The second table: one per project directory the input holds (see [detect](#detect) for which they are), sorted by `path`. `[]` when there is none, always in loose mode. |
| `projects[].path` | string | The project directory as the manifest's `home` records it, the collector's path; in image mode it still starts with the `-r` root. |
| `projects[].users` | array of strings | The distinct users whose rows name the directory, sorted. |
| `projects[].agents` | array of objects | One per agent that owns at least one file in the directory, sorted by `agent`. Never holds `project`; `[]` when the directory holds only shared files, such as a repository with an `AGENTS.md` and nothing else. |
| `projects[].agents[].agent` | string | Catalog agent name. |
| `projects[].agents[].files` | integer | Files of this agent in the directory, counted as for `homes[].agents[].files`. |
| `projects[].agents[].bytes` | integer | Their total manifest `size`. |
| `projects[].agents[].parser` | boolean | `true` when the analyzer has a parser for this agent, `false` when it is detected only. |
| `projects[].shared_files` | integer | The shared files in the directory: files several agents read, which the manifest records with agent `project` rather than an owner (`AGENTS.md`, `.agents/`, `.mcp.json`, `.github/copilot-instructions.md`, `SOUL.md`, the `.git/` metadata; see [detect](#detect) for the full list and their readers). 0 when there is none. |
| `projects[].shared_bytes` | integer | The total manifest `size` of those shared files. |
| `projects[].remotes` | array of objects | The remotes of the directory's `.git/config`, one per `[remote "<name>"]` section that sets a non-empty `url` or `pushurl`, sorted by `name`; `[]` when the collection holds no `.git/config` there (a collector before 1.19.0, a directory that is not a repository, or a linked worktree or submodule whose `.git` is a file). The file is read whole, as git writes it: lines ending at a line feed or a carriage return and line feed, an initial byte-order mark ignored, section and key names in any case, quoted values, `#` and `;` comments, a pair on the same line as its section header, a value continued by a backslash at the end of its line (a backslash that ends a comment continues nothing); a line that is neither a section header nor a `key = value` pair is skipped, as are the keys after an unterminated section header. A value git rejects, a quoted one the file ends inside or one holding an escape git does not define (only `\\`, `\"`, `\n`, `\t` and `\b` are escapes), is left out and listed under `problems` as `<path>: line <n>: unterminated quoted value` or `<path>: line <n>: unsupported escape \q`, so a config cut mid-URL or edited by hand never yields a destination git would not read. The deprecated `[remote.name]` header form lower-cases its name, as git does, so it names the same remote as `[remote "name"]`. A `.git/config` or `.git/HEAD` the analyzer cannot read (not a regular file, a permission or I/O error) leaves `remotes` `[]` and `head` `""` and is listed under `problems` as `<path>: <reason>`. Since 0.21.0. |
| `projects[].remotes[].name` | string | The remote's name, as the section header spells it (`origin`, `upstream`). |
| `projects[].remotes[].urls` | array of strings | Every `url` value of the section, in file order; `[]` when it sets none. Git fetches from the first and, when the section sets no `pushurl`, pushes to every one. A URL may carry credentials in its user part (`https://<user>:<token>@<host>/...`): redact them before sharing the output. |
| `projects[].remotes[].pushurls` | array of strings | Every `pushurl` value of the section, in file order; `[]` when it sets none. Git pushes to every one, so a second value is a second destination of each push. These are the values the file records, not git's fallback to `urls`, and they may carry credentials as `urls` can. |
| `projects[].head` | string | What the directory's `.git/HEAD` names: the checked-out branch (`main` for `ref: refs/heads/main`), the full ref for any other symbolic ref, or the commit hash of a detached HEAD; `""` when the collection holds no `.git/HEAD` there. Last key of the object since 0.21.0. |
| `problems` | array of strings | The `problem:` lines without their prefix, `<path>: <reason>`: each directory or file inside the input that could not be read while it was opened, and each collected manifest row whose file is missing from `fs/` (see [Unreadable inputs](inputs.md#unreadable-inputs)). `[]` when there are none. |
| `files` | array of objects | Only with `--files`: one per attributed file, in the order the manifest lists them (the walk order in loose mode). It mirrors the manifest file by file, so a shared file such as `AGENTS.md` appears here with agent `project`. |
| `files[].user` | string | As `homes[].user`. |
| `files[].agent` | string | The attributed agent: the manifest `agent`, re-attributed from the bundled project catalog for a project file of a collection before 1.12.0; `project` for a shared file (`AGENTS.md`, `.mcp.json`). |
| `files[].path` | string | The file's path on the source host, as the manifest records it. |

Before analyzer 0.12.0 the per-home counts were a flat array `agents`,
one object per user, home and agent with the keys `user`, `home`,
`agent`, `files`, `bytes`, `parser` and `inferred`, which also held the
project directories as rows whose `home` was the directory; `homes` had
no `agents` key and listed the project directories too.

## timeline

`timeline` writes three files into `-o`, creating the directory if needed,
overwriting files of the same name, and writes them even when no rows
were produced:

| File | What it holds | Schema |
| --- | --- | --- |
| `timeline.jsonl` | One record per turn, tool call, tool result or system event, sorted by time. | [timeline.jsonl](#timelinejsonl) |
| `sessions.jsonl` | One record per session, with its first and last timestamp, models and counts. | [sessions.jsonl](#sessionsjsonl) |
| `inventory.json` | What was detected and what was parsed: the input's kind, notes and problems; per home the files, bytes and whether a parser exists for each agent; row and session counts per project and outside any project, whether each project directory was collected and the files each agent left in it; and the lists of agents parsed and not parsed. | [inventory.json](#inventoryjson) |

On stdout it prints the input, host and notes, one `problem:` line per
directory that could not be read and per file whose read error no parser
reported as a row (see
[Unreadable inputs](inputs.md#unreadable-inputs)), and, when the timeline
is filtered, a `window:` line with the resolved UTC bounds, a `match:`
line with the patterns, and a `filtered:` line counting the rows dropped
outside the window, without a timestamp, and not matching. Then comes
the `dropped:` line, always printed, which counts the rows of secondary
sources left out because a primary store, or another secondary source,
records the same event (see
[Secondary sources](#secondary-sources)), with the agents that had any
in parentheses, sorted by name, and no parentheses when the count is 0:

```text
dropped:   3 duplicate rows from secondary sources (antigravity 2, claude-code 1)
dropped:   0 duplicate rows from secondary sources
```

Then come the `agents:` line, the row count with a breakdown by agent and
`turn_type`, and the session count. The breakdown counts the rows written. The
last three line groups name the three files, one each, in this order:

```text
rows:      214 in out/timeline.jsonl
           claude-code    assistant    40
           claude-code    user         50
sessions:  6 in out/sessions.jsonl
inventory: out/inventory.json (homes, projects, scratch directories, counts per agent, problems)
```

`rows:` is followed by one indented line per agent and `turn_type` with
its count, sorted by agent and type, and nothing when there are no rows.
When at least one record of `sessions.jsonl` has `synced` `yes`,
`relayed` `yes`, `origin` `remote` or `recovered` `yes` or `partial`,
the `sessions:` line ends with those counts in parentheses after the
path, in that order, each part present only when its count is greater
than 0 and the parts joined by a comma:
`sessions:  6 in out/sessions.jsonl (1 synced)`,
`sessions:  6 in out/sessions.jsonl (2 relayed)` or
`sessions:  6 in out/sessions.jsonl (1 synced, 2 relayed, 1 remote, 3 recovered)`.
With none it ends at the path, as above. The `synced` part is there
since analyzer 0.14.0, the `relayed`, `remote` and `recovered` parts
since 0.15.0.
The paths are `-o` joined with the file name, as given on the command
line. The exit code is `0` when at least one row was written and `1`
otherwise; the files are written either way. Before analyzer 0.12.0
`timeline` also wrote `detect.json`, named on a `detect:` line before the
`inventory:` line; its content is now in `inventory.json` (see
[inventory.json](#inventoryjson)).

The `agents:` line is always one line. When the input holds at least one
agent it lists them in two to four groups; otherwise it reads
`none were found`:

```text
agents:    parsed: antigravity, claude-code; detected but not parsed: amp
agents:    parsed: claude-code; detected but not parsed: none
agents:    parsed: none; detected but not parsed: amp
agents:    parsed: claude-code; detected but not parsed: none; parser found no rows: antigravity
agents:    parsed: claude-code; detected but not parsed: amp; collected only: copilot
agents:    none were found
```

`parsed` names the agents that produced rows before filtering. `detected but
not parsed` names the agents in the input that have no parser and whose files
hold records a parser is wanted for, the `backlog` rows of the [agents
page](../../docs/agents.md) (the agents `detect` shows without a parser, in
either table, so an agent whose only files are in a project directory counts).
`collected only` names the agents in the input that have no parser because
their files hold nothing to parse, a token store or configuration only: the
`collected` rows of that table. The group follows `detected but not parsed`
and appears only when it has an entry. Before analyzer 0.14.0 those agents
were listed under `detected but not parsed`. `parser found no rows` names the
agents in the input that have a parser which read their files and produced no
rows, such as a data directory holding only an empty database; the group
appears only when it has an entry, and with `--agent` it counts only the
agents selected. Shared files (agent `project`, see [detect](#detect)), which
every parser is offered, are never listed in any group. Each list is sorted
and comma-separated, and `parsed` or `detected but not parsed` reads `none`
when it is empty. `none were found` means the input holds no agent data, where
`detect` prints `no agent artifacts found`. Before analyzer 0.12.0 the line
read `parsed <list>; detected only <list>`, with `none` for an empty list.

This is the output of `timeline -o out` on
[the example collection](#the-example-collection); the files it writes
are the examples under [inventory.json](#inventoryjson),
[timeline.jsonl](#timelinejsonl) and [sessions.jsonl](#sessionsjsonl).
The `note:` and `problem:` lines repeat those of `detect`; the
`dropped:` line counts the secondary-source rows, the history prompts
among them, that a transcript also records; and the `sessions:` line
ends with the synced, remote and recovered counts: the Claude Code
sessions bound to Remote Control, spawned by it or teleported, the
Cursor cloud agents and a Codex CLI thread whose feedback report was
uploaded are synced; the Remote Control spawn, the teleported session
and the cloud agents are remote; and the sessions whose rows come in
whole or in part from secondary sources are recovered:

<!-- generated by analyzer/tools/regenerate_examples.py: timeline-stdout -->
```text
input:     /cases/wsdev14/wsdev14 (collected)
host:      wsdev14
note:      1 manifest rows did not parse
problem:   /home/bob/.gemini/tmp/6b9af143446a411aefa3edb76e4693799bd7513e7ed8d919962d6713d3d3b972/chats/session-2026-09-30T08-00-b2c3d4e5.json: missing from the collection
dropped:   20 duplicate rows from secondary sources (aider 2, claude-code 2, codex-cli 12, cursor 3, gemini-cli 1)
agents:    parsed: aider, claude-code, codex-cli, cursor, gemini-cli, ollama; detected but not parsed: amp
rows:      295 in out/timeline.jsonl
           aider          assistant    2
           aider          system       11
           aider          tool_result  5
           aider          user         5
           claude-code    assistant    6
           claude-code    system       97
           claude-code    tool_result  10
           claude-code    tool_use     10
           claude-code    user         17
           codex-cli      assistant    2
           codex-cli      system       17
           codex-cli      tool_result  4
           codex-cli      tool_use     4
           codex-cli      user         2
           cursor         assistant    4
           cursor         system       56
           cursor         tool_result  4
           cursor         tool_use     5
           cursor         user         6
           gemini-cli     assistant    2
           gemini-cli     system       10
           gemini-cli     tool_result  3
           gemini-cli     tool_use     3
           gemini-cli     user         6
           ollama         system       3
           ollama         user         1
sessions:  22 in out/sessions.jsonl (7 synced, 4 remote, 7 recovered)
inventory: out/inventory.json (homes, projects, scratch directories, counts per agent, problems)
```
<!-- end generated: timeline-stdout -->

## inventory.json

`inventory.json` is one JSON object, UTF-8 without a byte order mark,
indented by two spaces. It is the detection snapshot of the run as well
as its counts: `kind`, `notes`, `homes` and the problems met while the
input was opened are what [`detect --json`](#detect---json) prints for
the same input, so an analyst reads them here later instead of running
`detect` again. Its row and session counts follow what `timeline.jsonl`
and `sessions.jsonl` hold, so with `--since`, `--until` or `--match` they
count the rows and sessions left after filtering, and with `--agent` only
the agents selected. `homes`, `notes`, `problems` and the file counts
(`homes[].agents[].files` and `bytes`, and the project `files`, `bytes`
and `shared_files`) describe the whole input and do not follow the
filters, except that `--agent` changes the parser problems, which come
from the parsers that ran. This example is the `inventory.json` that
`timeline -o out` writes for
[the example collection](#the-example-collection), the run shown under
[timeline](#timeline). Under `projects`, `/srv/proj` has the `remotes`
and `head` of its `.git/` files and, in `recorded_remotes`, the URL
alice's Codex CLI thread recorded; the paths that only rows name, such
as `/srv/old` of alice's prompt history and the GitHub repositories of
the Cursor cloud agents, are `referenced`; and `scratch_directories`
holds the scratchpads alice's Claude Code sessions name, counted there
rather than as projects:

<!-- generated by analyzer/tools/regenerate_examples.py: inventory-json -->
```json
{
  "doubleagent_version": "0.24.0",
  "input": "/cases/wsdev14/wsdev14",
  "kind": "collected",
  "host": "wsdev14",
  "notes": [
    "1 manifest rows did not parse"
  ],
  "problems": [
    "/home/bob/.gemini/tmp/6b9af143446a411aefa3edb76e4693799bd7513e7ed8d919962d6713d3d3b972/chats/session-2026-09-30T08-00-b2c3d4e5.json: missing from the collection"
  ],
  "rows": 295,
  "sessions": 22,
  "parsed": [
    "aider",
    "claude-code",
    "codex-cli",
    "cursor",
    "gemini-cli",
    "ollama"
  ],
  "detected_not_parsed": [
    "amp"
  ],
  "no_rows": [],
  "homes": [
    {
      "user": "alice",
      "home": "/home/alice",
      "inferred": false,
      "agents": [
        {
          "agent": "aider",
          "files": 4,
          "bytes": 1379,
          "parser": true,
          "synced_sessions": 0,
          "relayed_sessions": 0,
          "remote_sessions": 0,
          "recovered_sessions": 2
        },
        {
          "agent": "amp",
          "files": 1,
          "bytes": 96,
          "parser": false,
          "synced_sessions": 0,
          "relayed_sessions": 0,
          "remote_sessions": 0,
          "recovered_sessions": 0
        },
        {
          "agent": "claude-code",
          "files": 23,
          "bytes": 40040,
          "parser": true,
          "synced_sessions": 4,
          "relayed_sessions": 0,
          "remote_sessions": 2,
          "recovered_sessions": 1
        },
        {
          "agent": "codex-cli",
          "files": 8,
          "bytes": 59166,
          "parser": true,
          "synced_sessions": 1,
          "relayed_sessions": 0,
          "remote_sessions": 0,
          "recovered_sessions": 1
        },
        {
          "agent": "cursor",
          "files": 11,
          "bytes": 100667,
          "parser": true,
          "synced_sessions": 2,
          "relayed_sessions": 0,
          "remote_sessions": 2,
          "recovered_sessions": 2
        }
      ]
    },
    {
      "user": "bob",
      "home": "/home/bob",
      "inferred": false,
      "agents": [
        {
          "agent": "gemini-cli",
          "files": 9,
          "bytes": 4598,
          "parser": true,
          "synced_sessions": 0,
          "relayed_sessions": 0,
          "remote_sessions": 0,
          "recovered_sessions": 1
        },
        {
          "agent": "ollama",
          "files": 6,
          "bytes": 300,
          "parser": true,
          "synced_sessions": 0,
          "relayed_sessions": 0,
          "remote_sessions": 0,
          "recovered_sessions": 0
        }
      ]
    }
  ],
  "projects": {
    "/home/alice": {
      "status": "referenced",
      "rows": 23,
      "sessions": 2,
      "agents": {
        "aider": {
          "rows": 23,
          "sessions": 2,
          "turn_types": {
            "assistant": 2,
            "system": 11,
            "tool_result": 5,
            "user": 5
          },
          "files": 0,
          "bytes": 0
        }
      },
      "shared_files": 0,
      "synced": "",
      "relayed": "",
      "origin": "",
      "recovered": "partial",
      "remotes": [],
      "head": "",
      "recorded_remotes": []
    },
    "/srv/old": {
      "status": "referenced",
      "rows": 1,
      "sessions": 1,
      "agents": {
        "claude-code": {
          "rows": 1,
          "sessions": 1,
          "turn_types": {
            "user": 1
          },
          "files": 0,
          "bytes": 0
        }
      },
      "shared_files": 0,
      "synced": "no",
      "relayed": "",
      "origin": "local",
      "recovered": "yes",
      "remotes": [],
      "head": "",
      "recorded_remotes": []
    },
    "/srv/other": {
      "status": "referenced",
      "rows": 1,
      "sessions": 0,
      "agents": {
        "gemini-cli": {
          "rows": 1,
          "sessions": 0,
          "turn_types": {
            "system": 1
          },
          "files": 0,
          "bytes": 0
        }
      },
      "shared_files": 0,
      "synced": "",
      "relayed": "",
      "origin": "",
      "recovered": "",
      "remotes": [],
      "head": "",
      "recorded_remotes": []
    },
    "/srv/proj": {
      "status": "collected",
      "rows": 212,
      "sessions": 16,
      "agents": {
        "claude-code": {
          "rows": 124,
          "sessions": 8,
          "turn_types": {
            "assistant": 6,
            "system": 83,
            "tool_result": 10,
            "tool_use": 10,
            "user": 15
          },
          "files": 1,
          "bytes": 70
        },
        "codex-cli": {
          "rows": 27,
          "sessions": 2,
          "turn_types": {
            "assistant": 2,
            "system": 15,
            "tool_result": 4,
            "tool_use": 4,
            "user": 2
          },
          "files": 0,
          "bytes": 0
        },
        "cursor": {
          "rows": 38,
          "sessions": 3,
          "turn_types": {
            "assistant": 4,
            "system": 19,
            "tool_result": 4,
            "tool_use": 5,
            "user": 6
          },
          "files": 0,
          "bytes": 0
        },
        "gemini-cli": {
          "rows": 23,
          "sessions": 3,
          "turn_types": {
            "assistant": 2,
            "system": 9,
            "tool_result": 3,
            "tool_use": 3,
            "user": 6
          },
          "files": 0,
          "bytes": 0
        }
      },
      "shared_files": 2,
      "synced": "partial",
      "relayed": "",
      "origin": "mixed",
      "recovered": "partial",
      "remotes": [
        {
          "name": "origin",
          "urls": [
            "git@github.com:alice/proj.git",
            "https://mirror.example.com/alice/proj.git"
          ],
          "pushurls": [
            "https://alice:FIXTURE-TOKEN@github.com/alice/proj.git"
          ]
        },
        {
          "name": "upstream",
          "urls": [
            "https://github.com/upstream/proj.git"
          ],
          "pushurls": []
        }
      ],
      "head": "main",
      "recorded_remotes": [
        "git@github.com:alice/proj.git"
      ]
    },
    "/srv/proj2": {
      "status": "referenced",
      "rows": 2,
      "sessions": 0,
      "agents": {
        "claude-code": {
          "rows": 2,
          "sessions": 0,
          "turn_types": {
            "system": 1,
            "user": 1
          },
          "files": 0,
          "bytes": 0
        }
      },
      "shared_files": 0,
      "synced": "",
      "relayed": "",
      "origin": "",
      "recovered": "",
      "remotes": [],
      "head": "",
      "recorded_remotes": []
    },
    "/srv/work": {
      "status": "referenced",
      "rows": 1,
      "sessions": 0,
      "agents": {
        "claude-code": {
          "rows": 1,
          "sessions": 0,
          "turn_types": {
            "system": 1
          },
          "files": 0,
          "bytes": 0
        }
      },
      "shared_files": 0,
      "synced": "",
      "relayed": "",
      "origin": "",
      "recovered": "",
      "remotes": [],
      "head": "",
      "recorded_remotes": []
    },
    "github.com/alice-gh/demo": {
      "status": "referenced",
      "rows": 8,
      "sessions": 1,
      "agents": {
        "cursor": {
          "rows": 8,
          "sessions": 1,
          "turn_types": {
            "system": 8
          },
          "files": 0,
          "bytes": 0
        }
      },
      "shared_files": 0,
      "synced": "yes",
      "relayed": "",
      "origin": "remote",
      "recovered": "no",
      "remotes": [],
      "head": "",
      "recorded_remotes": []
    },
    "github.com/alice-gh/web": {
      "status": "referenced",
      "rows": 5,
      "sessions": 1,
      "agents": {
        "cursor": {
          "rows": 5,
          "sessions": 1,
          "turn_types": {
            "system": 5
          },
          "files": 0,
          "bytes": 0
        }
      },
      "shared_files": 0,
      "synced": "yes",
      "relayed": "",
      "origin": "remote",
      "recovered": "no",
      "remotes": [],
      "head": "",
      "recorded_remotes": []
    }
  },
  "outside_projects": {
    "rows": 40,
    "sessions": 1,
    "agents": {
      "claude-code": {
        "rows": 10,
        "sessions": 0,
        "turn_types": {
          "system": 10
        }
      },
      "codex-cli": {
        "rows": 2,
        "sessions": 0,
        "turn_types": {
          "system": 2
        }
      },
      "cursor": {
        "rows": 24,
        "sessions": 1,
        "turn_types": {
          "system": 24
        }
      },
      "ollama": {
        "rows": 4,
        "sessions": 0,
        "turn_types": {
          "system": 3,
          "user": 1
        }
      }
    },
    "synced": "no",
    "relayed": "",
    "origin": "local",
    "recovered": "no"
  },
  "deduplicated": {
    "aider": 2,
    "claude-code": 2,
    "codex-cli": 12,
    "cursor": 3,
    "gemini-cli": 1
  },
  "collected_only": [],
  "synced_sessions": 7,
  "relayed_sessions": 0,
  "remote_sessions": 4,
  "recovered_sessions": 7,
  "scratch_directories": {
    "/private/tmp/claude-501/-srv-proj/11111111-2222-4333-8444-000000000002/scratchpad": {
      "status": "referenced",
      "rows": 1,
      "sessions": 0,
      "agents": {
        "claude-code": {
          "rows": 1,
          "sessions": 0,
          "turn_types": {
            "system": 1
          },
          "files": 0,
          "bytes": 0
        }
      },
      "shared_files": 0,
      "synced": "",
      "relayed": "",
      "origin": "",
      "recovered": "",
      "remotes": [],
      "head": "",
      "recorded_remotes": []
    },
    "/tmp/claude-1000/-srv-proj/11111111-2222-4333-8444-aaaaaaaaaaaa/scratchpad": {
      "status": "referenced",
      "rows": 1,
      "sessions": 0,
      "agents": {
        "claude-code": {
          "rows": 1,
          "sessions": 0,
          "turn_types": {
            "system": 1
          },
          "files": 0,
          "bytes": 0
        }
      },
      "shared_files": 0,
      "synced": "",
      "relayed": "",
      "origin": "",
      "recovered": "",
      "remotes": [],
      "head": "",
      "recorded_remotes": []
    }
  }
}
```
<!-- end generated: inventory-json -->

Each row is counted once: under the project its `project_path` names,
under `scratch_directories` when that path is a scratch directory, or
under `outside_projects` when it is empty; a session is counted where its
first non-empty `project_path` puts it. `/srv/proj` is `collected`: its
agents are those with rows there (the Claude Code, Codex CLI, Cursor and
Gemini CLI sessions run in it and the project's `permission setting:`
rows) and those with files there. A path that only rows name, such as
`/srv/old` of alice's prompt history, is a `referenced` project with no
files. `outside_projects` holds the rows with no project: the user-level
settings rows, the rows of the Remote Control sidecar files
(`bridge-pointer.json` and `ccr-tip.json`), a Codex CLI automation and
log row, Cursor's editor-wide rows, and bob's Ollama rows.
`scratch_directories` holds the scratchpads alice's sessions name,
`referenced` because the collection holds no files from them.

`no_rows` is `[]`, `detected_not_parsed` lists `amp`, and
`collected_only` is `[]` because no agent of the input is one whose files
hold nothing to parse. `deduplicated` holds the counts of the `dropped:`
line per agent. `synced_sessions`, `remote_sessions` and
`recovered_sessions` are the counts of the `sessions:` line of
[timeline](#timeline), and `relayed_sessions` is 0, since no session here
went through a messaging platform. The `synced`, `relayed`, `origin` and
`recovered` marks of each project follow the rules in the table below: a
project whose sessions are some synced and some not reads `synced`
`partial`, and one whose sessions are some remote and some local reads
`origin` `mixed`, as `/srv/proj` does. Only `/srv/proj` has `remotes` and
`head`, from its `.git/config` and `.git/HEAD`; its `recorded_remotes` is
the URL in the `git remote:` row of alice's Codex CLI thread. Its keys, in
order:

| Key | Type | Meaning |
| --- | --- | --- |
| `doubleagent_version` | string | The analyzer version that wrote the file, as `doubleagent --version` prints it, so the parser set and row conventions behind the counts can be looked up in the changelog. First key since 0.14.0. |
| `input` | string | The input path, as on the `input:` line. |
| `kind` | string | As in [`detect --json`](#detect---json): `archive`, `collected` or `loose`. |
| `host` | string | The host name, `""` when unknown. |
| `notes` | array of strings | As in [`detect --json`](#detect---json): the `note:` lines without their prefix, in the order they were made; `[]` when there are none. |
| `problems` | array of strings | The `problem:` lines of `timeline` without their prefix, `<path>: <reason>`: first the problems met while the input was opened, as in `detect --json`, then each file a parser could not read, by its path on the source host. Not changed by `--since`, `--until` or `--match`; with `--agent`, only the parsers that ran add theirs. `[]` when there are none. |
| `rows` | integer | The number of records in `timeline.jsonl`, after the filters. |
| `sessions` | integer | The number of records in `sessions.jsonl`, after the filters. |
| `parsed` | array of strings | The `parsed` group of the `agents:` line: the agents that produced rows before filtering, sorted. |
| `detected_not_parsed` | array of strings | The `detected but not parsed` group: the agents in the input, outside or inside a project directory, that have no parser, sorted. Since 0.14.0 it excludes the agents in `collected_only`. |
| `no_rows` | array of strings | The `parser found no rows` group, sorted; `[]` when it is empty. |
| `homes` | array of objects | The `homes` array of [`detect --json`](#detect---json) for the same input, element for element, with `synced_sessions`, `relayed_sessions`, `remote_sessions` and `recovered_sessions` added as the last four keys of each agent object: one per user home and Docker volume, in the order its first collected row was met, never a project directory. Not filtered. |
| `homes[].user` | string | As `homes[].user` in `detect --json`: the home's owner, `""` when unknown. |
| `homes[].home` | string | As `homes[].home` in `detect --json`: the manifest `home`, or in loose mode the path relative to the input root with a leading `/`. |
| `homes[].inferred` | boolean | `true` in loose mode, where user and home come from the path; `false` when they come from a manifest. |
| `homes[].agents` | array of objects | One per agent with files in this home, sorted by `agent`; `[]` when every file of the home is missing from the collection. |
| `homes[].agents[].agent` | string | Catalog agent name; never `project`. |
| `homes[].agents[].files` | integer | Files of this agent in this home, counted as in `detect --json`; not filtered. |
| `homes[].agents[].bytes` | integer | Their total size, the manifest `size` or the size on disk in loose mode; not filtered. |
| `homes[].agents[].parser` | boolean | `true` when the analyzer has a parser for this agent, `false` when it is detected only. |
| `homes[].agents[].synced_sessions` | integer | This user's sessions of this agent whose `synced` in `sessions.jsonl` is `yes`, after the filters; 0 when there is none, and always 0 for an agent whose parser records no sync state. Sessions are matched to the home by `user`. Last key of the object in 0.14.0. |
| `homes[].agents[].relayed_sessions` | integer | This user's sessions of this agent whose `relayed` in `sessions.jsonl` is `yes`, after the filters; 0 when there is none, and always 0 for an agent whose parser records no relay state. Sessions are matched to the home by `user`, as for `synced_sessions`. Added in 0.15.0. |
| `homes[].agents[].remote_sessions` | integer | This user's sessions of this agent whose `origin` in `sessions.jsonl` is `remote`, after the filters; 0 when there is none, and always 0 for an agent whose parser records no origin. Sessions are matched to the home by `user`, as for `synced_sessions`. Added in 0.15.0. |
| `homes[].agents[].recovered_sessions` | integer | This user's sessions of this agent whose `recovered` in `sessions.jsonl` is `yes` or `partial`, after the filters; 0 when there is none, and always 0 for an agent whose parser reads no secondary source. Sessions are matched to the home by `user`, as for `synced_sessions`. Last key of the object since 0.15.0. |
| `projects` | object | Keyed by project path, sorted: every distinct non-empty `project_path` of the rows, and every project directory the input holds, except the scratch directories, the agent's own working directories, which are under `scratch_directories` (since 0.18.0, see below), as the collector's path with the image root removed and Windows paths in the canonical form of [`timeline.jsonl`](#timelinejsonl) `project_path` (see below). Windows paths that differ only in case are one project, keyed by the collected directory's spelling when the project is one, else by the first spelling the rows record. `{}` when there is neither. |
| `projects.<path>.status` | string | `collected` when the path is a project directory the input holds, so its project files are in the collection; `referenced` when only rows name it, because the directory was deleted, was out of the collector's reach, was skipped with `--no-projects`, or is recorded in another form (see below). |
| `projects.<path>.rows` | integer | Rows with this `project_path`, or for a Windows path any spelling of it that differs only in case, after the filters. |
| `projects.<path>.sessions` | integer | Sessions whose `project_path` in `sessions.jsonl` is this one, compared as for `rows`, after the filters. |
| `projects.<path>.agents` | object | Keyed by agent name, sorted: every agent with rows in the project or files in its directory, so the keys are the agents used on the project. Never has the key `project`; `{}` for a project with neither. |
| `projects.<path>.agents.<agent>.rows` | integer | This agent's rows with this `project_path`, compared as for `rows`, after the filters; 0 for an agent with files and no rows. |
| `projects.<path>.agents.<agent>.sessions` | integer | This agent's sessions counted under this project, after the filters. |
| `projects.<path>.agents.<agent>.turn_types` | object | Keyed by `turn_type`, sorted: this agent's rows of each type in the project, after the filters, listing only the types that occur; `{}` with no rows. |
| `projects.<path>.agents.<agent>.files` | integer | The files the collection holds for this agent in this directory, not filtered; 0 for an agent with rows and no files there, and for every agent of a `referenced` project. |
| `projects.<path>.agents.<agent>.bytes` | integer | Their total manifest `size`, not filtered. |
| `projects.<path>.shared_files` | integer | The shared files the collection holds in the directory: files several agents read, which the manifest records with agent `project` rather than an owner (`AGENTS.md`, `.agents/`, `.mcp.json`, `.github/copilot-instructions.md`, `SOUL.md`, the `.git/` metadata; see [detect](#detect) for the full list and their readers). Not filtered; 0 for a `referenced` project. |
| `projects.<path>.synced` | string | `yes` when every session counted under the project has `synced` `yes`; `partial` when some do; `no` when none does and at least one agent of the project records sync state; `""` otherwise, and for a project with no session. Since 0.15.0. |
| `projects.<path>.relayed` | string | The same four values from `relayed`, with `no` when at least one agent of the project records relay state. Since 0.15.0. |
| `projects.<path>.origin` | string | `remote` when every session counted under the project has `origin` `remote`; `mixed` when some do; `local` when none does and at least one agent of the project records origin; `""` otherwise, and for a project with no session. Since 0.15.0. |
| `projects.<path>.recovered` | string | `yes` when every session counted under the project has `recovered` `yes`, so the project's primary transcripts are gone and only secondary sources remain; `partial` when some of its sessions are `yes` or `partial`; `no` when none is and at least one agent of the project reads a secondary source; `""` otherwise, and for a project with no session. A `referenced` project whose `recovered` is `yes` is one whose directory and whose agents' primary stores are both gone: what is left of it is the secondary sources. Last keys of the entry from 0.15.0 to 0.20.1: `synced`, `relayed`, `origin` and `recovered`, in that order. Counts of these sessions are kept only at the top level and per home agent. |
| `projects.<path>.remotes` | array of objects | The remotes of the directory's `.git/config`, as `projects[].remotes` in [`detect --json`](#detect---json): `name`, `urls` and `pushurls` per remote, sorted by `name`. `[]` for a `referenced` project and for a collected one whose collection holds no `.git/config`. Not filtered. Since 0.21.0. |
| `projects.<path>.head` | string | The branch or detached commit the directory's `.git/HEAD` names, as `projects[].head` in `detect --json`; `""` for a `referenced` project and when the collection holds no `.git/HEAD`. Not filtered. Since 0.21.0. |
| `projects.<path>.recorded_remotes` | array of strings | The remote URLs the agents recorded for the project: the distinct URLs of the `git remote:` rows (see [Row order and system rows](#row-order-and-system-rows)) after the filters, sorted. A row of a session is counted under the session's project, as the session is, and a row without a session under its own `project_path`. It is filled for a `referenced` project too, so a project whose directory is gone still names the remote its agent saw; `[]` when no row names one. Last keys of the entry since 0.21.0: `remotes`, `head` and `recorded_remotes`, in that order. |
| `outside_projects` | object | The counts for the rows and sessions whose `project_path` is empty. It has no status, no file counts and no `remotes`, `head` or `recorded_remotes`: files outside projects are counted per home under `homes`. |
| `outside_projects.rows` | integer | Rows with an empty `project_path`, after the filters. |
| `outside_projects.sessions` | integer | Sessions with an empty `project_path`, after the filters. |
| `outside_projects.agents` | object | Keyed by agent name, sorted: the agents with such rows or sessions; `{}` when there is none. |
| `outside_projects.agents.<agent>.rows` | integer | This agent's rows with an empty `project_path`, after the filters. |
| `outside_projects.agents.<agent>.sessions` | integer | This agent's sessions with an empty `project_path`, after the filters. |
| `outside_projects.agents.<agent>.turn_types` | object | Keyed by `turn_type`, sorted: this agent's rows of each type, listing only the types that occur. |
| `outside_projects.synced`, `relayed`, `origin`, `recovered` | string | As for a project, by the same rules and in the same order, over the sessions with an empty `project_path` and the agents with such rows or sessions; a Copilot cloud agent task, which records no project, is counted here. Last keys since 0.15.0. |
| `deduplicated` | object | Keyed by agent name, sorted: the agents with rows of a secondary source dropped because a primary store or another secondary source records the same event (see [Secondary sources](#secondary-sources)), the counts of the `dropped:` line. With `--agent`, only the agents selected; not changed by `--since`, `--until` or `--match`, which apply to the rows kept. `{}` when no row was dropped. |
| `deduplicated.<agent>` | integer | The number of this agent's secondary-source rows dropped, never 0. |
| `collected_only` | array of strings | The `collected only` group of the `agents:` line: the agents in the input, outside or inside a project directory, that have no parser because their files hold nothing to parse (the `collected` rows of the [agents page](../../docs/agents.md)), sorted; `[]` when it is empty. Since 0.14.0. |
| `synced_sessions` | integer | The number of records in `sessions.jsonl` whose `synced` is `yes`, after the filters, as for `sessions`; 0 when there is none. Last key in 0.14.0. |
| `relayed_sessions` | integer | The number of records in `sessions.jsonl` whose `relayed` is `yes`, after the filters, as for `sessions`; 0 when there is none. Added in 0.15.0. |
| `remote_sessions` | integer | The number of records in `sessions.jsonl` whose `origin` is `remote`, after the filters, as for `sessions`; 0 when there is none. Added in 0.15.0. |
| `recovered_sessions` | integer | The number of records in `sessions.jsonl` whose `recovered` is `yes` or `partial`, after the filters, as for `sessions`; 0 when there is none. Last key from 0.15.0 to 0.17.0. |
| `scratch_directories` | object | Keyed by path, sorted, and grouped as `projects` is: every `project_path` of the rows, and every project directory the input holds, that is a scratch directory (see below); `{}` when there is none. Last key since 0.18.0. |
| `scratch_directories.<path>.status` | string | As `projects.<path>.status`: `collected` when the collector gathered project files from the directory, `referenced` when only rows name it. Since 0.18.0. |
| `scratch_directories.<path>.rows`, `sessions`, `agents`, `shared_files` | integer, object | As for a project, over the rows and sessions with this `project_path` and the files in the directory; each agent object has the keys of `projects.<path>.agents.<agent>`. Since 0.18.0. |
| `scratch_directories.<path>.synced`, `relayed`, `origin`, `recovered` | string | As for a project, by the same rules and in the same order. Since 0.18.0. |
| `scratch_directories.<path>.remotes`, `head`, `recorded_remotes` | array of objects, string, array of strings | As for a project, by the same rules and in the same order, as the last keys of the entry. Since 0.21.0. |

A row is counted under its own `project_path` and a session under the
session's, the first non-empty `project_path` among its rows, so a session
whose rows name two projects has rows under both and is counted once, under
the first. Rows with no `session_id` belong to no session. The `rows` of
`projects`, `scratch_directories` and `outside_projects` add up to the
top-level `rows`, and their `sessions` to the top-level `sessions`.
Nothing is counted twice: files per home are under `homes`, files per
project directory under `projects` or `scratch_directories`, and rows and
sessions under `projects`, `scratch_directories` and `outside_projects`.

A scratch directory is a working directory that belongs to the agent
rather than to the user: the directory an app creates for a chat that has
no project (the ChatGPT desktop app's `~/Documents/Codex/<date>/<slug>`,
with `work/` and `outputs/` inside), the agent's own state directory or a
clone or worktree it made inside it (Copilot Desktop's
`.copilot/chats/<id>`, `.copilot/repos/<name>` and
`.copilot/canvas-catalog-probe`), or the working directory of a session
the app pre-created and never prompted (Copilot Desktop's pool and probe
sessions, whose `cwd` on Windows is the app's install directory). A
project is a directory the user pointed the agent at. The two are kept
apart because a scratch directory says what the agent did on its own
account, which can still matter to the investigation. A path is a
scratch directory when any of these holds:

1. A parser says so: a `system` row whose `text` starts with
   `scratch directory:` names it in its own `project_path`. The
   `codex-cli` parser writes `scratch directory: <path> (projectless
   thread <id>) (from .codex/.codex-global-state.json
   thread-projectless-output-directories, file mtime)` for each entry of
   the desktop app's map of projectless output directories, and the
   `copilot-cli` parser writes `scratch directory: <cwd> (working
   directory of a session the app pre-created and never prompted: no
   events.jsonl)` for a session directory that holds a `workspace.yaml`
   with a `cwd` and no `events.jsonl`. The `opencode` and `kilo-code`
   parsers write `scratch directory: <directory> (git worktree of
   <project> strategy=<strategy>) (from <database> worktree,
   time_created)` for each worktree the agent made, from the `worktree`
   or `project_directory` table. Since 0.20.1 the `continue` parser
   writes one for a `cn review` worktree, the `zed` parser for a
   thread's sandboxed-terminal temp directory and a git worktree Zed
   created, the `hermes` parser for a git worktree the agent created
   (`<repo>/.worktrees/<name>` on a `hermes/<name>` branch), the
   `openhands` parser for a conversation in the Agent Server's default
   worktree root `/tmp/conversation-worktrees`, and the `agent-zero`
   parser for an editor (ACP) chat's container working directory; each
   paragraph under [Row order and system rows](#row-order-and-system-rows)
   gives the row's text. Since 0.21.0 the `claude-code` parser writes
   `scratch directory: <path> (session scratchpad, from the environment
   attachment)` for the session scratchpad a transcript's `environment`
   attachment names (`%LOCALAPPDATA%\Temp\claude\<slug>\<session>\scratchpad`
   on Windows).
2. The catalog says so: the path is in or under a directory that a home
   entry of the bundled catalog names, in one of the input's homes. The
   path relative to the home is matched segment by segment against each
   entry's glob, so `/home/alice/.copilot/chats/<id>` is under the entry
   `.copilot` and `/home/alice/.config/Cursor/x` under
   `.config/[Cc]ursor`, while `/home/alice/.config/myproject` is under no
   entry. A Windows home is compared ignoring case. The rule needs the
   home's path on the source host, which a collection's manifest gives;
   in loose mode it applies only where the analyzer could infer the
   home's path from the tree, and not at all when the input is a home
   itself.
3. The project catalog says so: the path is under a directory that a
   project entry of one agent in the bundled catalog names, inside
   another directory, with at least one segment below the entry. This is
   the worktree an agent makes inside the user's project:
   `/srv/proj/.claude/worktrees/feat` is under the `claude-code` entry
   `.claude`, and `/srv/proj/.kilo/worktrees/k1`,
   `/srv/proj/.gemini/worktrees/g1`, `/srv/proj/.qwen/worktrees/q1` and
   `/srv/proj/.codex/worktrees/c1` under the `kilo-code`, `gemini-cli`,
   `qwen-code` and `codex-cli` entries, while `/srv/proj/.kilo` itself
   and `/srv/proj/src` are not scratch directories. The entries that
   belong to no single agent (`AGENTS.md`, `.agents`, the agent `project`
   in `--list`) do not count. A Windows path is compared ignoring case.
   The rule needs no home path, so it applies in loose mode as well.
   Since 0.20.1 ([issue 227](https://github.com/seanthegeek/doubleagent/issues/227)).

No rule looks at the collector's list of project directories, so a
scratch directory the collector gathered project files from is listed
under `scratch_directories` with `status` `collected`. A scratch
directory the collector gathers as the agent's own files (the Codex
output directory `Documents/Codex`, a `codex-cli` catalog entry since
collector 1.16.0) is not in `collection.json` `projects`, so it is
listed with `status` `referenced`: its files are counted under `homes`
for `codex-cli`, not as project files. The rows keep
their `project_path` in `timeline.jsonl` and `sessions.jsonl`, and
`detect` and `detect --json`, which read no rows, list every project
directory the input holds without applying either rule.

The project directories the input holds are the paths in `collection.json`
`projects` (the directories the collector gathered project files from,
discovered or given with `-p`) and, for a collection whose
`collection.json` is missing or has no `projects`, the `home` of each
collected manifest row with agent `project`. A loose input holds none:
the analyzer does not look for project directories in a loose tree, so
every key there is `referenced`. Each such directory is listed even when
no row names it, and even when the filters left no rows at all.

Paths are compared and counted as recorded, with two exceptions. First,
for a collection made in image mode (`collection.json` `mode` `image`),
the `root` given with `-r` is removed from the front of each project
directory, so `/mnt/image/srv/proj` in `collection.json` matches the
`/srv/proj` the transcripts on that host recorded; the file counts of two
directories that map to one path are added together. Second, a Windows
path (one that starts with a drive letter, or with `\\` for a network
share) is in the canonical form described under
[`timeline.jsonl`](#timelinejsonl) `project_path`, in the rows and in the
project directories alike, and is compared ignoring case, because Windows
file names are case-insensitive: `c:/Users/sean/demo`,
`C:\Users\sean\demo\` and `C:\USERS\sean\demo` are one project. Its
key is the spelling of the collected directory when the project is one,
so a collected project's rows, files and `status` `collected` stay under
the path `collection.json` names; otherwise it is the first spelling seen
in the rows in timeline order, then in the sessions. The case after the
drive is never changed, since the analyzer cannot know the folder's real
casing. Nothing else is normalised, so one repository recorded in two
forms has two keys: a trailing slash or a different case on a POSIX path,
or a Windows image collected on Linux or macOS, whose transcripts say
`C:\Users\alice\proj` while the collector's path, once the root is
removed, is `/Users/alice/proj`. The collector's form then has `status`
`collected` and the files, and the transcripts' form has the rows and
`status` `referenced`.

A jq query that lists projects by row count:

```sh
jq -r '.projects | to_entries | sort_by(-.value.rows)[] | "\(.value.rows)\t\(.key)"' out/inventory.json
```

The same for scratch directories:

```sh
jq -r '.scratch_directories | to_entries | sort_by(-.value.rows)[] | "\(.value.rows)\t\(.key)"' out/inventory.json
```

One that lists each project with the agents used on it:

```sh
jq -r '.projects | to_entries[] | "\(.key)\t\(.value.agents | keys | join(", "))"' out/inventory.json
```

And one that lists the agents that left files in a project but no rows:

```sh
jq -r '.projects | to_entries[] | .key as $p | .value.agents | to_entries[] | select(.value.rows == 0 and .value.files > 0) | "\($p)\t\(.key)"' out/inventory.json
```

Before analyzer 0.12.0 there was no `inventory.json`. The keys are an
interface like the JSONL fields: new ones are added at the end, the
existing ones keep their names, types and meaning. Analyzer 0.13.0 added
`deduplicated`. Analyzer 0.14.0 added `collected_only` and then
`synced_sessions`, the last key, and `synced_sessions` as the last key of
each `homes[].agents[]` object; from that version `detected_not_parsed`
no longer lists the agents with nothing to parse, which it had held since
0.12.0. Analyzer 0.15.0 added `relayed_sessions`, `remote_sessions`
and `recovered_sessions` after `synced_sessions`, in that order, as the
last keys of the file and of each `homes[].agents[]` object, and
`synced`, `relayed`, `origin` and `recovered` as the last keys of each
`projects.<path>` entry and of `outside_projects`. Analyzer 0.18.0
added `scratch_directories` as the last key of the file and moved the
scratch directories out of `projects` into it. Analyzer 0.21.0 added
`remotes`, `head` and `recorded_remotes` as the last keys of each
`projects.<path>` and `scratch_directories.<path>` entry. The one key added at the front is `doubleagent_version`, in
analyzer 0.14.0, because a reader needs the version before the rest; the
file is one object read by key, not by position.

## JSONL encoding

Both JSONL files are UTF-8 without a byte order mark, one JSON object per
line, each line ended by LF, with no header line. Every object has every
field, in the order listed below. Fields with no value hold the empty string
`""`, never `null`, and every field is a string except the integers
`source_line`, `user_turns`, `assistant_turns` and `tool_calls` and the
array `models`. A line holds no literal line break: newlines inside
`text` are written as `\n`, and U+2028 and U+2029 as `\u2028` and
`\u2029`, because some line readers split on them. Other non-ASCII text is
written as UTF-8, not escaped, so a search for it matches as typed; the
exception is a record whose text holds a lone UTF-16 surrogate, which UTF-8
cannot encode, and which is written entirely with `\uXXXX` escapes.
There is no space after `:` or `,`, the same layout `jq -c` prints, so a
`grep` pattern such as `"agent":"codex-cli"` matches the file and jq output
alike. [searching.md](searching.md) has recipes.

## timeline.jsonl

`timeline.jsonl` fields, in order:

| Field | Type | Meaning |
| --- | --- | --- |
| `timestamp_utc` | string | ISO 8601 UTC with milliseconds, `2026-10-03T16:55:31.123Z`. Times recorded without a zone are taken as UTC. Epoch numbers above 10^11 are read as milliseconds, smaller ones as seconds. Empty when the record has none. |
| `host` | string | `--host` when given, else `collection.json` `hostname`. Empty for loose input without `--host`. |
| `user` | string | From the manifest, inferred from the path, or `--user`. |
| `agent` | string | Catalog agent name of the parser that produced the row. |
| `session_id` | string | The agent's own session or thread identifier. |
| `project_path` | string | Working directory recorded for the turn. A Windows path, one that starts with a drive letter and a separator or with `\\`, is written in one canonical form whichever store recorded it: upper-case drive letter, backslash separators, no repeated separators and no trailing separator except in a drive root (`C:\`), and the extended-length prefix `\\?\` removed (`\\?\UNC\server\share` becomes `\\server\share`), so `c:/Users/sean/demo/` is `C:\Users\sean\demo`. The case after the drive is kept as recorded. Any other path, POSIX paths included, is written as recorded. A `file:` URI is turned into a path by the parser that reads it. |
| `git_branch` | string | Branch recorded for the turn, where the agent logs one. |
| `turn_type` | string | `user`, `assistant`, `tool_use`, `tool_result`, `system`, or `thinking` with `--include-thinking`. |
| `model` | string | Model that produced the turn, where recorded. |
| `tool_name` | string | For `tool_use` rows: the tool. A Codex command run is `shell` when the rollout records it only as a `local_shell_call`, or as a `CommandExecution` item with no call line of the same id, else the call's own name, such as `exec_command`. |
| `tool_use_id` | string | Links a `tool_use` row to its `tool_result`. |
| `text` | string | The event's full text, with leading and trailing whitespace removed and inner line breaks and indentation kept: the prompt, the response, the tool output, or for a tool call its most identifying argument (the Bash command, the edited file, the search pattern, the fetched URL), else the arguments as JSON. It is never truncated, so the timeline is a complete transcript, one event per line. |
| `source_file` | string | Path of the record on the source host, as in the manifest. In loose mode it is the path relative to the input root with a leading `/`, or relative to the home when the input root is the home or an agent directory. |
| `source_line` | integer | Where the record is in that file, so the full record can be read. For line-oriented files (JSONL, Markdown, text) the 1-based line number. For SQLite stores it is the record's `rowid` (Tabby events: the event id), and for JSON documents an array index or `1`. Each parser's module docstring says which. `0` when no position applies. |
| `recovered` | string | `yes` when the row is an event (any `turn_type` but `system`) from a secondary source of its agent (see [Secondary sources](#secondary-sources)) that survived deduplication, so the agent's primary store does not hold the event; since 0.20.0 also the `user` and `assistant` rows the `cursor` parser takes from the `agentKv` prompt blobs of a conversation deleted in the editor, which the editor's database still holds although the conversation's own records are gone; since 0.20.1 also the turns the `kiro` parser recovers from a conversation's checkpoint snapshots or tangent-mode main history, which the conversation's own history no longer holds. `""` for every other row, including every `system` row of a secondary source (an index header, a thread record, a snapshot, a session-start or permission row, a `parser:` problem), which describes the source or the session rather than a turn the primary store lost. Since analyzer 0.15.0. |

The field names each parser relies on are listed in its module docstring
under `doubleagent/parsers/`. Thinking and reasoning blocks are left out
unless `--include-thinking` is passed.

This is the `timeline.jsonl` of a filtered run on
[the example collection](#the-example-collection): `timeline -o out`
with `--agent claude-code --agent codex-cli --agent cursor` and a
`--match` regular expression that keeps the row forms named here, a
matched tool call keeping its result (see
[The example collection](#the-example-collection)); the unfiltered run
is the one shown under [timeline](#timeline). Rows worth following: a
tool call and its result sharing one `tool_use_id`; a
`permission decision:` row beside the call it decided; a
`model: <old> -> <new>` row after alice's `/model`; the
`session origin: remote` rows of the Remote Control and teleported
sessions; the history prompt with `recovered` `yes`; and the
`parser: unparseable line` row at the cut end of alice's Cursor agent
transcript:

<!-- generated by analyzer/tools/regenerate_examples.py: timeline-jsonl -->
```json
{"timestamp_utc":"2026-09-30T10:00:00.000Z","host":"wsdev14","user":"alice","agent":"claude-code","session_id":"99999999-0000-4000-8000-000000000000","project_path":"/srv/old","git_branch":"","turn_type":"user","model":"","tool_name":"","tool_use_id":"","text":"an older prompt whose session file is gone","source_file":"/home/alice/.claude/history.jsonl","source_line":2,"recovered":"yes"}
{"timestamp_utc":"2026-10-01T08:00:00.000Z","host":"wsdev14","user":"alice","agent":"codex-cli","session_id":"aaaaaaaa-bbbb-4ccc-8ddd-000000000006","project_path":"/srv/proj","git_branch":"","turn_type":"system","model":"","tool_name":"","tool_use_id":"","text":"thread record: aaaaaaaa-bbbb-4ccc-8ddd-000000000006 title=remove the old build artifacts rollout=/home/alice/.codex/sessions/2026/10/01/rollout-2026-10-01T08-00-00-aaaaaaaa-bbbb-4ccc-8ddd-000000000006.jsonl name=cleanup model=gpt-5-codex archived","source_file":"/home/alice/.codex/state_5.sqlite","source_line":2,"recovered":""}
{"timestamp_utc":"2026-10-01T09:00:00.000Z","host":"wsdev14","user":"alice","agent":"claude-code","session_id":"","project_path":"","git_branch":"","turn_type":"system","model":"","tool_name":"","tool_use_id":"","text":"permission setting: permissions.allow=Bash(npm test:*) (from .claude/settings.json, file mtime)","source_file":"/home/alice/.claude/settings.json","source_line":1,"recovered":""}
{"timestamp_utc":"2026-10-01T09:00:00.000Z","host":"wsdev14","user":"alice","agent":"claude-code","session_id":"","project_path":"","git_branch":"","turn_type":"system","model":"","tool_name":"","tool_use_id":"","text":"permission setting: permissions.allow=Read(~/notes/**) (from .claude/settings.json, file mtime)","source_file":"/home/alice/.claude/settings.json","source_line":1,"recovered":""}
{"timestamp_utc":"2026-10-01T09:00:00.000Z","host":"wsdev14","user":"alice","agent":"claude-code","session_id":"","project_path":"","git_branch":"","turn_type":"system","model":"","tool_name":"","tool_use_id":"","text":"permission setting: permissions.defaultMode=acceptEdits (from .claude/settings.json, file mtime)","source_file":"/home/alice/.claude/settings.json","source_line":1,"recovered":""}
{"timestamp_utc":"2026-10-01T10:00:06.000Z","host":"wsdev14","user":"alice","agent":"claude-code","session_id":"11111111-2222-4333-8444-555555555555","project_path":"/srv/proj","git_branch":"main","turn_type":"system","model":"","tool_name":"","tool_use_id":"","text":"slash command: <command-name>/model</command-name>\n<command-message>model</command-message>\n<command-args></command-args>","source_file":"/home/alice/.claude/projects/-srv-proj/11111111-2222-4333-8444-555555555555.jsonl","source_line":14,"recovered":""}
{"timestamp_utc":"2026-10-01T10:01:00.000Z","host":"wsdev14","user":"alice","agent":"cursor","session_id":"c0000001-0000-4000-8000-000000000001","project_path":"/srv/proj","git_branch":"","turn_type":"system","model":"grok-4.7","tool_name":"","tool_use_id":"","text":"model: grok-4.6 -> grok-4.7","source_file":"/home/alice/.config/Cursor/User/globalStorage/state.vscdb","source_line":6,"recovered":""}
{"timestamp_utc":"2026-10-01T10:01:01.000Z","host":"wsdev14","user":"alice","agent":"cursor","session_id":"c0000001-0000-4000-8000-000000000001","project_path":"/srv/proj","git_branch":"","turn_type":"tool_use","model":"grok-4.7","tool_name":"run_terminal_command_v2","tool_use_id":"call-t2-0\nfc_t2_0","text":"{\"command\": \"curl -s example.test | head\"}","source_file":"/home/alice/.config/Cursor/User/globalStorage/state.vscdb","source_line":7,"recovered":""}
{"timestamp_utc":"2026-10-01T10:01:01.000Z","host":"wsdev14","user":"alice","agent":"cursor","session_id":"c0000001-0000-4000-8000-000000000001","project_path":"/srv/proj","git_branch":"","turn_type":"system","model":"grok-4.7","tool_name":"run_terminal_command_v2","tool_use_id":"call-t2-0\nfc_t2_0","text":"permission decision: run outcome=success approval=user reason=Not in allowlist: curl, head","source_file":"/home/alice/.config/Cursor/User/globalStorage/state.vscdb","source_line":7,"recovered":""}
{"timestamp_utc":"2026-10-01T10:01:01.000Z","host":"wsdev14","user":"alice","agent":"cursor","session_id":"c0000001-0000-4000-8000-000000000001","project_path":"/srv/proj","git_branch":"","turn_type":"tool_result","model":"grok-4.7","tool_name":"run_terminal_command_v2","tool_use_id":"call-t2-0\nfc_t2_0","text":"{\"output\": \"ok\"}","source_file":"/home/alice/.config/Cursor/User/globalStorage/state.vscdb","source_line":7,"recovered":""}
{"timestamp_utc":"2026-10-01T10:01:10.000Z","host":"wsdev14","user":"alice","agent":"cursor","session_id":"c0000001-0000-4000-8000-000000000001","project_path":"/srv/proj","git_branch":"","turn_type":"tool_use","model":"grok-4.7","tool_name":"run_terminal_command_v2","tool_use_id":"call-t3-1\nfc_t3_1","text":"{\"command\": \"curl -s example.test | head\"}","source_file":"/home/alice/.config/Cursor/User/globalStorage/state.vscdb","source_line":8,"recovered":""}
{"timestamp_utc":"2026-10-01T10:01:10.000Z","host":"wsdev14","user":"alice","agent":"cursor","session_id":"c0000001-0000-4000-8000-000000000001","project_path":"/srv/proj","git_branch":"","turn_type":"system","model":"grok-4.7","tool_name":"run_terminal_command_v2","tool_use_id":"call-t3-1\nfc_t3_1","text":"permission decision: skip outcome=rejected approval=user reason=Not in allowlist: curl, head","source_file":"/home/alice/.config/Cursor/User/globalStorage/state.vscdb","source_line":8,"recovered":""}
{"timestamp_utc":"2026-10-01T10:01:10.000Z","host":"wsdev14","user":"alice","agent":"cursor","session_id":"c0000001-0000-4000-8000-000000000001","project_path":"/srv/proj","git_branch":"","turn_type":"tool_result","model":"grok-4.7","tool_name":"run_terminal_command_v2","tool_use_id":"call-t3-1\nfc_t3_1","text":"[rejected] {\"error\": \"skipped\"}","source_file":"/home/alice/.config/Cursor/User/globalStorage/state.vscdb","source_line":8,"recovered":""}
{"timestamp_utc":"2026-10-01T10:01:20.000Z","host":"wsdev14","user":"alice","agent":"cursor","session_id":"c0000001-0000-4000-8000-000000000001","project_path":"/srv/proj","git_branch":"","turn_type":"tool_use","model":"grok-4.7","tool_name":"run_terminal_command_v2","tool_use_id":"call-t4-2\nfc_t4_2","text":"{\"command\": \"curl -s example.test | head\"}","source_file":"/home/alice/.config/Cursor/User/globalStorage/state.vscdb","source_line":9,"recovered":""}
{"timestamp_utc":"2026-10-01T10:01:20.000Z","host":"wsdev14","user":"alice","agent":"cursor","session_id":"c0000001-0000-4000-8000-000000000001","project_path":"/srv/proj","git_branch":"","turn_type":"system","model":"grok-4.7","tool_name":"run_terminal_command_v2","tool_use_id":"call-t4-2\nfc_t4_2","text":"permission decision: allowlistCommands outcome=success approval=user reason=Not in allowlist: curl, head allowlisted=curl,head","source_file":"/home/alice/.config/Cursor/User/globalStorage/state.vscdb","source_line":9,"recovered":""}
{"timestamp_utc":"2026-10-01T10:01:20.000Z","host":"wsdev14","user":"alice","agent":"cursor","session_id":"c0000001-0000-4000-8000-000000000001","project_path":"/srv/proj","git_branch":"","turn_type":"tool_result","model":"grok-4.7","tool_name":"run_terminal_command_v2","tool_use_id":"call-t4-2\nfc_t4_2","text":"{\"output\": \"ok\"}","source_file":"/home/alice/.config/Cursor/User/globalStorage/state.vscdb","source_line":9,"recovered":""}
{"timestamp_utc":"2026-10-01T10:06:40.000Z","host":"wsdev14","user":"alice","agent":"cursor","session_id":"","project_path":"","git_branch":"","turn_type":"system","model":"","tool_name":"","tool_use_id":"","text":"source host: github account alice-gh scopes=repo,workflow,user:email,read:user (from state.vscdb key github-alice-gh)","source_file":"/home/alice/.config/Cursor/User/globalStorage/state.vscdb","source_line":4,"recovered":""}
{"timestamp_utc":"2026-10-01T11:00:01.000Z","host":"wsdev14","user":"alice","agent":"claude-code","session_id":"11111111-2222-4333-8444-777777777777","project_path":"/srv/proj","git_branch":"main","turn_type":"system","model":"","tool_name":"","tool_use_id":"","text":"permission mode: auto","source_file":"/home/alice/.claude/projects/-srv-proj/11111111-2222-4333-8444-777777777777.jsonl","source_line":4,"recovered":""}
{"timestamp_utc":"2026-10-01T11:00:02.000Z","host":"wsdev14","user":"alice","agent":"claude-code","session_id":"11111111-2222-4333-8444-777777777777","project_path":"/srv/proj","git_branch":"main","turn_type":"tool_use","model":"claude-fable-5-1","tool_name":"Bash","tool_use_id":"toolu_95","text":"cd /srv/proj && crontab -l","source_file":"/home/alice/.claude/projects/-srv-proj/11111111-2222-4333-8444-777777777777.jsonl","source_line":5,"recovered":""}
{"timestamp_utc":"2026-10-01T11:00:03.000Z","host":"wsdev14","user":"alice","agent":"claude-code","session_id":"11111111-2222-4333-8444-777777777777","project_path":"/srv/proj","git_branch":"main","turn_type":"tool_result","model":"","tool_name":"","tool_use_id":"toolu_95","text":"0 3 * * * x","source_file":"/home/alice/.claude/projects/-srv-proj/11111111-2222-4333-8444-777777777777.jsonl","source_line":6,"recovered":""}
{"timestamp_utc":"2026-10-01T11:00:03.000Z","host":"wsdev14","user":"alice","agent":"claude-code","session_id":"11111111-2222-4333-8444-777777777777","project_path":"/srv/proj","git_branch":"main","turn_type":"system","model":"","tool_name":"","tool_use_id":"toolu_95","text":"permission decision: allow source=auto_mode reason=classifier","source_file":"/home/alice/.claude/projects/-srv-proj/11111111-2222-4333-8444-777777777777.jsonl","source_line":6,"recovered":""}
{"timestamp_utc":"2026-10-01T12:00:01.000Z","host":"wsdev14","user":"alice","agent":"claude-code","session_id":"11111111-2222-4333-8444-999999999999","project_path":"/srv/proj","git_branch":"main","turn_type":"system","model":"","tool_name":"","tool_use_id":"","text":"permission mode: auto","source_file":"/home/alice/.claude/projects/-srv-proj/11111111-2222-4333-8444-999999999999.jsonl","source_line":1,"recovered":""}
{"timestamp_utc":"2026-10-01T12:00:01.000Z","host":"wsdev14","user":"alice","agent":"claude-code","session_id":"11111111-2222-5333-8444-000000000001","project_path":"/srv/proj","git_branch":"main","turn_type":"system","model":"","tool_name":"","tool_use_id":"","text":"session origin: remote (Remote Control, entrypoint sdk-cli)","source_file":"/home/alice/.claude/projects/-srv-proj/11111111-2222-5333-8444-000000000001.jsonl","source_line":3,"recovered":""}
{"timestamp_utc":"2026-10-01T12:00:01.000Z","host":"wsdev14","user":"alice","agent":"claude-code","session_id":"11111111-2222-5333-8444-000000000001","project_path":"/srv/proj","git_branch":"main","turn_type":"system","model":"","tool_name":"","tool_use_id":"","text":"permission mode: default","source_file":"/home/alice/.claude/projects/-srv-proj/11111111-2222-5333-8444-000000000001.jsonl","source_line":3,"recovered":""}
{"timestamp_utc":"2026-10-01T12:00:02.000Z","host":"wsdev14","user":"alice","agent":"claude-code","session_id":"11111111-2222-4333-8444-999999999999","project_path":"/srv/proj","git_branch":"main","turn_type":"system","model":"","tool_name":"","tool_use_id":"","text":"permission mode: auto -> plan","source_file":"/home/alice/.claude/projects/-srv-proj/11111111-2222-4333-8444-999999999999.jsonl","source_line":4,"recovered":""}
{"timestamp_utc":"2026-10-01T12:00:02.000Z","host":"wsdev14","user":"alice","agent":"claude-code","session_id":"11111111-2222-5333-8444-000000000001","project_path":"/srv/proj","git_branch":"main","turn_type":"tool_use","model":"claude-fable-5-1","tool_name":"Bash","tool_use_id":"toolu_r1","text":"ls -la","source_file":"/home/alice/.claude/projects/-srv-proj/11111111-2222-5333-8444-000000000001.jsonl","source_line":4,"recovered":""}
{"timestamp_utc":"2026-10-01T12:00:03.000Z","host":"wsdev14","user":"alice","agent":"claude-code","session_id":"11111111-2222-5333-8444-000000000001","project_path":"/srv/proj","git_branch":"main","turn_type":"tool_result","model":"","tool_name":"","tool_use_id":"toolu_r1","text":"calc.py\nREADME.md","source_file":"/home/alice/.claude/projects/-srv-proj/11111111-2222-5333-8444-000000000001.jsonl","source_line":5,"recovered":""}
{"timestamp_utc":"2026-10-01T12:00:03.000Z","host":"wsdev14","user":"alice","agent":"claude-code","session_id":"11111111-2222-5333-8444-000000000001","project_path":"/srv/proj","git_branch":"main","turn_type":"system","model":"","tool_name":"","tool_use_id":"toolu_r1","text":"permission decision: accept source=config reason=subcommandResults","source_file":"/home/alice/.claude/projects/-srv-proj/11111111-2222-5333-8444-000000000001.jsonl","source_line":5,"recovered":""}
{"timestamp_utc":"2026-10-01T12:00:06.000Z","host":"wsdev14","user":"alice","agent":"claude-code","session_id":"11111111-2222-4333-8444-999999999999","project_path":"/srv/proj","git_branch":"main","turn_type":"system","model":"","tool_name":"","tool_use_id":"","text":"slash command: <command-name>/model</command-name>\n<command-message>model</command-message>\n<command-args>opus</command-args>","source_file":"/home/alice/.claude/projects/-srv-proj/11111111-2222-4333-8444-999999999999.jsonl","source_line":6,"recovered":""}
{"timestamp_utc":"2026-10-01T12:00:08.000Z","host":"wsdev14","user":"alice","agent":"claude-code","session_id":"11111111-2222-4333-8444-999999999999","project_path":"/srv/proj","git_branch":"main","turn_type":"system","model":"claude-opus-5-5","tool_name":"","tool_use_id":"","text":"model: claude-fable-5-1 -> claude-opus-5-5 requested=opus","source_file":"/home/alice/.claude/projects/-srv-proj/11111111-2222-4333-8444-999999999999.jsonl","source_line":8,"recovered":""}
{"timestamp_utc":"2026-10-01T12:00:09.000Z","host":"wsdev14","user":"alice","agent":"claude-code","session_id":"11111111-2222-4333-8444-999999999999","project_path":"/srv/proj","git_branch":"main","turn_type":"system","model":"","tool_name":"","tool_use_id":"","text":"permission mode: plan -> acceptEdits","source_file":"/home/alice/.claude/projects/-srv-proj/11111111-2222-4333-8444-999999999999.jsonl","source_line":9,"recovered":""}
{"timestamp_utc":"2026-10-01T12:46:40.000Z","host":"wsdev14","user":"alice","agent":"cursor","session_id":"bc-e0000001-0000-4000-8000-000000000001","project_path":"github.com/alice-gh/demo","git_branch":"","turn_type":"system","model":"grok-4.7","tool_name":"","tool_use_id":"","text":"session synced: cursor cloud agent bc-e0000001-0000-4000-8000-000000000001 (vendor copy; from state.vscdb cloudAgentRepository.agents)","source_file":"/home/alice/.config/Cursor/User/globalStorage/state.vscdb","source_line":12,"recovered":""}
{"timestamp_utc":"2026-10-01T12:47:40.000Z","host":"wsdev14","user":"alice","agent":"cursor","session_id":"bc-e0000001-0000-4000-8000-000000000001","project_path":"github.com/alice-gh/demo","git_branch":"","turn_type":"system","model":"grok-4.7","tool_name":"","tool_use_id":"","text":"session origin: remote cursor cloud agent","source_file":"/home/alice/.config/Cursor/User/globalStorage/conversation-search.db","source_line":10,"recovered":""}
{"timestamp_utc":"2026-10-01T13:00:00.000Z","host":"wsdev14","user":"alice","agent":"claude-code","session_id":"11111111-2222-4333-8444-000000000002","project_path":"/srv/proj","git_branch":"main","turn_type":"system","model":"","tool_name":"","tool_use_id":"","text":"session origin: remote (teleport, cloud scratchpad /tmp/claude-0/-home-user-repo/22222222-3333-4444-8555-666666666666/scratchpad)","source_file":"/home/alice/.claude/projects/-srv-proj/11111111-2222-4333-8444-000000000002.jsonl","source_line":1,"recovered":""}
{"timestamp_utc":"2026-10-01T13:00:01.000Z","host":"wsdev14","user":"alice","agent":"claude-code","session_id":"11111111-2222-4333-8444-000000000002","project_path":"/srv/proj","git_branch":"main","turn_type":"system","model":"","tool_name":"","tool_use_id":"","text":"session origin: remote (teleport, remoteSourced)","source_file":"/home/alice/.claude/projects/-srv-proj/11111111-2222-4333-8444-000000000002.jsonl","source_line":2,"recovered":""}
{"timestamp_utc":"2026-10-01T13:00:01.000Z","host":"wsdev14","user":"alice","agent":"claude-code","session_id":"11111111-2222-4333-8444-000000000002","project_path":"/srv/proj","git_branch":"main","turn_type":"system","model":"","tool_name":"","tool_use_id":"","text":"session synced: cloud session (teleported: remoteSourced records replayed from the vendor's copy)","source_file":"/home/alice/.claude/projects/-srv-proj/11111111-2222-4333-8444-000000000002.jsonl","source_line":2,"recovered":""}
{"timestamp_utc":"2026-10-01T13:00:01.000Z","host":"wsdev14","user":"alice","agent":"claude-code","session_id":"11111111-2222-4333-8444-000000000002","project_path":"/srv/proj","git_branch":"main","turn_type":"system","model":"","tool_name":"","tool_use_id":"","text":"permission mode: default","source_file":"/home/alice/.claude/projects/-srv-proj/11111111-2222-4333-8444-000000000002.jsonl","source_line":2,"recovered":""}
{"timestamp_utc":"2026-10-01T13:00:01.000Z","host":"wsdev14","user":"alice","agent":"claude-code","session_id":"11111111-2222-4333-8444-aaaaaaaaaaaa","project_path":"/tmp/claude-1000/-srv-proj/11111111-2222-4333-8444-aaaaaaaaaaaa/scratchpad","git_branch":"main","turn_type":"system","model":"","tool_name":"","tool_use_id":"","text":"scratch directory: /tmp/claude-1000/-srv-proj/11111111-2222-4333-8444-aaaaaaaaaaaa/scratchpad (session scratchpad, from the environment attachment)","source_file":"/home/alice/.claude/projects/-srv-proj/11111111-2222-4333-8444-aaaaaaaaaaaa.jsonl","source_line":4,"recovered":""}
{"timestamp_utc":"2026-10-01T13:00:02.000Z","host":"wsdev14","user":"alice","agent":"claude-code","session_id":"11111111-2222-4333-8444-000000000002","project_path":"/srv/proj","git_branch":"main","turn_type":"tool_use","model":"claude-opus-5-5","tool_name":"Read","tool_use_id":"toolu_t1","text":"/home/user/repo/README.md","source_file":"/home/alice/.claude/projects/-srv-proj/11111111-2222-4333-8444-000000000002.jsonl","source_line":3,"recovered":""}
{"timestamp_utc":"2026-10-01T13:00:03.000Z","host":"wsdev14","user":"alice","agent":"claude-code","session_id":"11111111-2222-4333-8444-000000000002","project_path":"/srv/proj","git_branch":"main","turn_type":"tool_result","model":"","tool_name":"","tool_use_id":"toolu_t1","text":"# logsleuth","source_file":"/home/alice/.claude/projects/-srv-proj/11111111-2222-4333-8444-000000000002.jsonl","source_line":4,"recovered":""}
{"timestamp_utc":"2026-10-01T13:00:03.000Z","host":"wsdev14","user":"alice","agent":"claude-code","session_id":"11111111-2222-4333-8444-000000000002","project_path":"/srv/proj","git_branch":"main","turn_type":"system","model":"","tool_name":"","tool_use_id":"toolu_t1","text":"permission decision: accept source=config reason=rule {\"decision\":\"accept\",\"reasonType\":\"rule\",\"ruleSource\":\"cliArg\",\"source\":\"config\"}","source_file":"/home/alice/.claude/projects/-srv-proj/11111111-2222-4333-8444-000000000002.jsonl","source_line":4,"recovered":""}
{"timestamp_utc":"2026-10-01T13:00:03.000Z","host":"wsdev14","user":"alice","agent":"claude-code","session_id":"11111111-2222-4333-8444-aaaaaaaaaaaa","project_path":"/srv/proj","git_branch":"main","turn_type":"tool_use","model":"claude-sonnet-5-5","tool_name":"PowerShell","tool_use_id":"toolu_309a","text":"python calc.py 2 3","source_file":"/home/alice/.claude/projects/-srv-proj/11111111-2222-4333-8444-aaaaaaaaaaaa.jsonl","source_line":9,"recovered":""}
{"timestamp_utc":"2026-10-01T13:00:04.000Z","host":"wsdev14","user":"alice","agent":"claude-code","session_id":"11111111-2222-4333-8444-aaaaaaaaaaaa","project_path":"/srv/proj","git_branch":"main","turn_type":"tool_result","model":"","tool_name":"","tool_use_id":"toolu_309a","text":"first line of the listing\nsecond line of the listing","source_file":"/home/alice/.claude/projects/-srv-proj/11111111-2222-4333-8444-aaaaaaaaaaaa.jsonl","source_line":10,"recovered":""}
{"timestamp_utc":"2026-10-01T13:00:04.000Z","host":"wsdev14","user":"alice","agent":"claude-code","session_id":"11111111-2222-4333-8444-aaaaaaaaaaaa","project_path":"/srv/proj","git_branch":"main","turn_type":"system","model":"","tool_name":"","tool_use_id":"toolu_309a","text":"permission decision: accept source=user_temporary reason=- {\"decision\":\"accept\",\"grantScope\":\"once\",\"reasonCode\":\"other\",\"source\":\"user_temporary\"}","source_file":"/home/alice/.claude/projects/-srv-proj/11111111-2222-4333-8444-aaaaaaaaaaaa.jsonl","source_line":10,"recovered":""}
{"timestamp_utc":"2026-10-01T13:00:05.000Z","host":"wsdev14","user":"alice","agent":"claude-code","session_id":"11111111-2222-4333-8444-aaaaaaaaaaaa","project_path":"/srv/proj","git_branch":"main","turn_type":"tool_use","model":"claude-sonnet-5-5","tool_name":"PowerShell","tool_use_id":"toolu_309b","text":"python calc.py 5 7","source_file":"/home/alice/.claude/projects/-srv-proj/11111111-2222-4333-8444-aaaaaaaaaaaa.jsonl","source_line":11,"recovered":""}
{"timestamp_utc":"2026-10-01T13:00:06.000Z","host":"wsdev14","user":"alice","agent":"claude-code","session_id":"11111111-2222-4333-8444-aaaaaaaaaaaa","project_path":"/srv/proj","git_branch":"main","turn_type":"tool_result","model":"","tool_name":"","tool_use_id":"toolu_309b","text":"The user doesn't want to proceed with this tool use.","source_file":"/home/alice/.claude/projects/-srv-proj/11111111-2222-4333-8444-aaaaaaaaaaaa.jsonl","source_line":12,"recovered":""}
{"timestamp_utc":"2026-10-01T13:00:06.000Z","host":"wsdev14","user":"alice","agent":"claude-code","session_id":"11111111-2222-4333-8444-aaaaaaaaaaaa","project_path":"/srv/proj","git_branch":"main","turn_type":"system","model":"","tool_name":"","tool_use_id":"toolu_309b","text":"permission decision: reject source=user_reject reason=- {\"decision\":\"reject\",\"reasonCode\":\"other\",\"source\":\"user_reject\"} toolDenialKind=user-rejected userFeedback=use the test runner instead","source_file":"/home/alice/.claude/projects/-srv-proj/11111111-2222-4333-8444-aaaaaaaaaaaa.jsonl","source_line":12,"recovered":""}
{"timestamp_utc":"2026-10-01T13:00:07.000Z","host":"wsdev14","user":"alice","agent":"claude-code","session_id":"11111111-2222-4333-8444-aaaaaaaaaaaa","project_path":"/srv/proj","git_branch":"main","turn_type":"system","model":"","tool_name":"","tool_use_id":"","text":"session synced: remote control bridge cse_0a1b2c3d4e5f6a7b url https://claude.ai/code/session_0a1b2c3d4e5f6a7b","source_file":"/home/alice/.claude/projects/-srv-proj/11111111-2222-4333-8444-aaaaaaaaaaaa.jsonl","source_line":14,"recovered":""}
{"timestamp_utc":"2026-10-01T13:00:10.000Z","host":"wsdev14","user":"alice","agent":"claude-code","session_id":"11111111-2222-4333-8444-000000000002","project_path":"/private/tmp/claude-501/-srv-proj/11111111-2222-4333-8444-000000000002/scratchpad","git_branch":"main","turn_type":"system","model":"","tool_name":"","tool_use_id":"","text":"scratch directory: /private/tmp/claude-501/-srv-proj/11111111-2222-4333-8444-000000000002/scratchpad (session scratchpad, from the environment attachment)","source_file":"/home/alice/.claude/projects/-srv-proj/11111111-2222-4333-8444-000000000002.jsonl","source_line":7,"recovered":""}
{"timestamp_utc":"2026-10-01T13:00:11.000Z","host":"wsdev14","user":"alice","agent":"claude-code","session_id":"11111111-2222-4333-8444-000000000002","project_path":"/srv/proj","git_branch":"main","turn_type":"system","model":"","tool_name":"","tool_use_id":"","text":"permission mode: default -> auto","source_file":"/home/alice/.claude/projects/-srv-proj/11111111-2222-4333-8444-000000000002.jsonl","source_line":8,"recovered":""}
{"timestamp_utc":"2026-10-01T13:03:20.000Z","host":"wsdev14","user":"alice","agent":"cursor","session_id":"bc-e0000002-0000-4000-8000-000000000002","project_path":"github.com/alice-gh/web","git_branch":"","turn_type":"system","model":"grok-4.7","tool_name":"","tool_use_id":"","text":"session origin: remote cursor cloud agent","source_file":"/home/alice/.config/Cursor/User/globalStorage/state.vscdb","source_line":13,"recovered":""}
{"timestamp_utc":"2026-10-01T13:03:20.000Z","host":"wsdev14","user":"alice","agent":"cursor","session_id":"bc-e0000002-0000-4000-8000-000000000002","project_path":"github.com/alice-gh/web","git_branch":"","turn_type":"system","model":"grok-4.7","tool_name":"","tool_use_id":"","text":"session synced: cursor cloud agent bc-e0000002-0000-4000-8000-000000000002 (vendor copy; from state.vscdb cloudAgentRepository.agents)","source_file":"/home/alice/.config/Cursor/User/globalStorage/state.vscdb","source_line":13,"recovered":""}
{"timestamp_utc":"2026-10-01T14:00:00.000Z","host":"wsdev14","user":"alice","agent":"cursor","session_id":"bc-e0000001-0000-4000-8000-000000000001","project_path":"github.com/alice-gh/demo","git_branch":"","turn_type":"system","model":"grok-4.7","tool_name":"","tool_use_id":"toolu_mcp_auth_01\nfc_mcp_auth_01","text":"permission decision: accepted mcp auth for cloud agent bc-e0000001-0000-4000-8000-000000000001 tool_call=toolu_mcp_auth_01\nfc_mcp_auth_01 (from state.vscdb cloudAgentRepository.mcpAuthResolution, file mtime)","source_file":"/home/alice/.config/Cursor/User/globalStorage/state.vscdb","source_line":29,"recovered":""}
{"timestamp_utc":"2026-10-01T14:00:00.000Z","host":"wsdev14","user":"alice","agent":"cursor","session_id":"","project_path":"","git_branch":"","turn_type":"system","model":"","tool_name":"","tool_use_id":"","text":"permission setting: permissions.allow=Shell(ls) (from .cursor/cli-config.json, file mtime)","source_file":"/home/alice/.cursor/cli-config.json","source_line":1,"recovered":""}
{"timestamp_utc":"2026-10-02T09:00:00.000Z","host":"wsdev14","user":"alice","agent":"codex-cli","session_id":"aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee","project_path":"/srv/proj","git_branch":"feature/x","turn_type":"system","model":"","tool_name":"","tool_use_id":"","text":"git remote: git@github.com:alice/proj.git","source_file":"/home/alice/.codex/sessions/2026/10/02/rollout-2026-10-02T09-00-00-aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee.jsonl","source_line":1,"recovered":""}
{"timestamp_utc":"2026-10-02T09:00:00.000Z","host":"wsdev14","user":"alice","agent":"codex-cli","session_id":"aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee","project_path":"/srv/proj","git_branch":"feature/x","turn_type":"system","model":"","tool_name":"","tool_use_id":"","text":"thread record: aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee title=exfiltrate nothing, just list the home dir rollout=/home/alice/.codex/sessions/2026/10/02/rollout-2026-10-02T09-00-00-aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee.jsonl branch=feature/x model=gpt-5.5-codex","source_file":"/home/alice/.codex/state_5.sqlite","source_line":1,"recovered":""}
{"timestamp_utc":"2026-10-02T09:00:01.000Z","host":"wsdev14","user":"alice","agent":"codex-cli","session_id":"aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee","project_path":"/srv/proj","git_branch":"feature/x","turn_type":"system","model":"gpt-5-codex","tool_name":"","tool_use_id":"","text":"permission mode: approval=on-request sandbox={\"type\":\"workspace-write\"} profile=- network=- reviewer=- profile_id=-","source_file":"/home/alice/.codex/sessions/2026/10/02/rollout-2026-10-02T09-00-00-aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee.jsonl","source_line":2,"recovered":""}
{"timestamp_utc":"2026-10-02T09:00:04.000Z","host":"wsdev14","user":"alice","agent":"codex-cli","session_id":"aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee","project_path":"/srv/proj","git_branch":"feature/x","turn_type":"tool_use","model":"gpt-5-codex","tool_name":"apply_patch","tool_use_id":"call_2","text":"{\"patch\":\"*** Begin Patch\"}","source_file":"/home/alice/.codex/sessions/2026/10/02/rollout-2026-10-02T09-00-00-aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee.jsonl","source_line":12,"recovered":""}
{"timestamp_utc":"2026-10-02T09:00:05.000Z","host":"wsdev14","user":"alice","agent":"codex-cli","session_id":"aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee","project_path":"/srv/proj","git_branch":"feature/x","turn_type":"tool_result","model":"gpt-5-codex","tool_name":"","tool_use_id":"call_2","text":"Done","source_file":"/home/alice/.codex/sessions/2026/10/02/rollout-2026-10-02T09-00-00-aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee.jsonl","source_line":13,"recovered":""}
{"timestamp_utc":"2026-10-02T09:00:08.000Z","host":"wsdev14","user":"alice","agent":"codex-cli","session_id":"aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee","project_path":"/srv/proj","git_branch":"feature/x","turn_type":"system","model":"gpt-5.5-codex","tool_name":"","tool_use_id":"","text":"model: gpt-5-codex -> gpt-5.5-codex","source_file":"/home/alice/.codex/sessions/2026/10/02/rollout-2026-10-02T09-00-00-aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee.jsonl","source_line":19,"recovered":""}
{"timestamp_utc":"2026-10-02T09:00:08.000Z","host":"wsdev14","user":"alice","agent":"codex-cli","session_id":"aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee","project_path":"/srv/proj","git_branch":"feature/x","turn_type":"system","model":"gpt-5.5-codex","tool_name":"","tool_use_id":"","text":"permission mode: approval=on-request sandbox={\"type\":\"workspace-write\"} profile=- network=- reviewer=- profile_id=- -> approval=never sandbox={\"type\":\"danger-full-access\"} profile={\"type\":\"disabled\"} network={\"allowed_domains\":[\"github.com\"],\"denied_domains\":[]} reviewer=- profile_id=-","source_file":"/home/alice/.codex/sessions/2026/10/02/rollout-2026-10-02T09-00-00-aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee.jsonl","source_line":19,"recovered":""}
{"timestamp_utc":"2026-10-07T10:00:00.250Z","host":"wsdev14","user":"alice","agent":"codex-cli","session_id":"aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee","project_path":"/srv/proj","git_branch":"feature/x","turn_type":"system","model":"","tool_name":"","tool_use_id":"","text":"session synced: feedback report uploaded to Sentry, attachments 2 failed false (from logs_2.sqlite logs, ts)","source_file":"/home/alice/.codex/logs_2.sqlite","source_line":3,"recovered":""}
{"timestamp_utc":"2026-10-07T10:01:00.000Z","host":"wsdev14","user":"alice","agent":"codex-cli","session_id":"","project_path":"","git_branch":"","turn_type":"system","model":"","tool_name":"","tool_use_id":"","text":"session synced: feedback report uploaded to Sentry, attachments 0 failed false (from logs_2.sqlite logs, ts)","source_file":"/home/alice/.codex/logs_2.sqlite","source_line":4,"recovered":""}
{"timestamp_utc":"2026-10-15T15:56:10.000Z","host":"wsdev14","user":"alice","agent":"claude-code","session_id":"11111111-2222-5333-8444-000000000001","project_path":"","git_branch":"","turn_type":"system","model":"","tool_name":"","tool_use_id":"","text":"session synced: remote event 33333333-4444-4555-8666-777777777777 at 1790856300000 (from .claude/projects/-srv-proj/11111111-2222-5333-8444-000000000001/ccr-tip.json, file mtime)","source_file":"/home/alice/.claude/projects/-srv-proj/11111111-2222-5333-8444-000000000001/ccr-tip.json","source_line":1,"recovered":""}
{"timestamp_utc":"2026-10-15T15:56:10.000Z","host":"wsdev14","user":"alice","agent":"claude-code","session_id":"","project_path":"","git_branch":"","turn_type":"system","model":"","tool_name":"","tool_use_id":"","text":"remote control: environment env_0123456789abcdefghijkl session 0123456789abcdef0123456789abcdef source=standalone (from .claude/projects/-srv-proj/bridge-pointer.json, file mtime)","source_file":"/home/alice/.claude/projects/-srv-proj/bridge-pointer.json","source_line":1,"recovered":""}
{"timestamp_utc":"2026-10-15T15:56:10.000Z","host":"wsdev14","user":"alice","agent":"claude-code","session_id":"11111111-2222-4333-8444-888888888888","project_path":"/srv/proj","git_branch":"","turn_type":"system","model":"","tool_name":"","tool_use_id":"","text":"session synced: remote control bridge cse_0123456789abcdef url - (from .claude/sessions/4343.json, file mtime)","source_file":"/home/alice/.claude/sessions/4343.json","source_line":1,"recovered":""}
{"timestamp_utc":"2026-10-15T15:56:10.000Z","host":"wsdev14","user":"alice","agent":"claude-code","session_id":"","project_path":"/srv/proj","git_branch":"","turn_type":"system","model":"","tool_name":"","tool_use_id":"","text":"permission setting: permissions.allow=Bash(make lint) (from .claude/settings.local.json, file mtime)","source_file":"/srv/proj/.claude/settings.local.json","source_line":1,"recovered":""}
{"timestamp_utc":"2026-10-15T15:56:10.000Z","host":"wsdev14","user":"alice","agent":"claude-code","session_id":"","project_path":"/srv/proj","git_branch":"","turn_type":"system","model":"","tool_name":"","tool_use_id":"","text":"permission setting: permissions.defaultMode=plan (from .claude/settings.local.json, file mtime)","source_file":"/srv/proj/.claude/settings.local.json","source_line":1,"recovered":""}
{"timestamp_utc":"","host":"wsdev14","user":"alice","agent":"claude-code","session_id":"11111111-2222-4333-8444-aaaaaaaaaaaa","project_path":"/srv/proj","git_branch":"main","turn_type":"system","model":"","tool_name":"","tool_use_id":"","text":"permission mode: default","source_file":"/home/alice/.claude/projects/-srv-proj/11111111-2222-4333-8444-aaaaaaaaaaaa.jsonl","source_line":2,"recovered":""}
{"timestamp_utc":"","host":"wsdev14","user":"alice","agent":"cursor","session_id":"c0000001-0000-4000-8000-000000000001","project_path":"/srv/proj","git_branch":"","turn_type":"system","model":"","tool_name":"","tool_use_id":"","text":"parser: unparseable line (Expecting value: line 1 column 31 (char 30)): {\"type\": \"user\", \"truncated\": tr","source_file":"/home/alice/.cursor/projects/srv-proj/agent-transcripts/c0000001-0000-4000-8000-000000000001/c0000001-0000-4000-8000-000000000001.jsonl","source_line":7,"recovered":""}
```
<!-- end generated: timeline-jsonl -->

### Row order and system rows

Rows are sorted by timestamp, then by source file and line. Rows without a
timestamp sort last. Injected context is typed `system`, not `user`, so
the `user` rows are what the person typed. In Claude Code that is
`isMeta` records, task notifications, slash commands and their output, a
subagent's task, compaction summaries and messages from other agents. In
Codex it is `developer` messages and user-role blocks whose content kind
names harness context rather than `user.*` input, or, in older rollouts
without content kinds, a block that is wholly a harness wrapper such as
`<environment_context>`. These `system` rows start with a lowercase label
naming the source, such as `subagent task:`, `task notification:`,
`slash command:`, `command output:` or `context: <kind>:`, followed by the
full text. Each line a parser cannot decode, such as a transcript's last
line cut off mid-write, is reported as its own `system` row whose
`source_line` is that line's number and whose `text` is
`parser: unparseable line (<reason>): <line>`: `<reason>` is the
decoder's message and `<line>` is the whole line as read (decoded as
UTF-8 with undecodable bytes replaced, less its line terminator), with
nothing repaired, extracted or shortened. The rest of the file is still
read. A file that exists but cannot be opened (a permission denied), a
SQLite database's `-wal`, `-shm` or `-journal` sidecar included, or that
the shared JSONL reader cannot read through (an I/O error), is one
whole-file `system` row naming the file and the
operating system's message, `parser: <file> could not be read (<reason>)`
from the shared JSONL and SQLite readers (a parser's own reader words it
as its research document states), with the file's known session id and
project path and `source_line` 0 when the file could not be opened, else
the last line read; the parser's other files are still read (since
0.23.0).
A line that is valid JSON but not an object, in a format whose
lines are objects, is reported the same way as `parser: unparseable line
(not a JSON object): <line>` (since 0.19.0). A database row or an array
element that does not decode is `parser: unparseable record <N>
(<reason>): <record>`: `<N>` is the row's `rowid` (or the id the table
keys it by), the element's position or its index, `<reason>` names the
table and column when the parser knows them (`(<table>.<column>:
<reason>)`, as in `goose`'s `messages.content_json` or `opencode`'s
`part.data`) and otherwise the decoder's message or `not a JSON object`,
and `<record>` is the stored value in full as text (a binary blob as
hex, as `zed` writes a `zstd` thread blob). Readers of JSON arrays use
the same form, with the record's text from its first character to the
end of the file, and a line that decodes but cannot be applied names what
failed, as VS Code's `parser: mutation not applied (<reason>): <line>`
does. An error that concerns the whole file rather than one line (the
file cannot be read, is not a JSON array, ends inside an array, is not a
SQLite database, or lacks the table the format keeps its records in) is
one `system` row `parser: <message>`, such as `parser: unterminated JSON
array`, `parser: not a SQLite database`, `parser: SQLite error:
<error>` or `parser: no <table> table in the session database`, with
the line number when the error has one and `0` otherwise.

Antigravity's permission state is also typed `system`. A grant recorded on
a step is `permission grant: action=<action> target=<target>
allow=<true|false> scope=<scope>`, or for a file access entry
`permission grant: path=<path> is_directory=<true|false>
allow=<true|false> scope=<scope>`; it follows the step's own rows and
carries the step's `tool_name` and `tool_use_id`, so it joins to the call
it governed. A standing grant from a settings file is
`permission setting: <key>=<value> (from <file>, file mtime)`: its
`timestamp_utc` is the file's modification time, not the time of the
grant, which no file records, and its `session_id` is empty, so it
appears in `timeline.jsonl` but in no `sessions.jsonl` record. The
[`antigravity` subsection of the agents page](../../docs/agents.md#antigravity) lists the
files.

Permission state and the model in use are `system` rows too. The
`claude-code` parser (and `claude-desktop` for the transcripts it reads
with the same reader) writes `permission mode: <mode>` at a session's
first permission mode and `permission mode: <old> -> <new>` on each
change, and `model: <id>` at the first assistant reply and
`model: <old> -> <new>` on each change, with ` requested=<id>` when the
model asked for differs from the one that answered; the `model` column of
a model row is the new model. Each subagent transcript tracks its own.
Standing permission settings are one row per entry, `permission setting:
<key>=<value> (from <file>, file mtime)`: `permissions.allow`, `deny`,
`ask`, `additionalDirectories`, `defaultMode` and
`disableBypassPermissionsMode` from `.claude/settings.json` and
`.claude/settings.local.json` (the user's, or a project's with
`project_path` the project), and `allowedTools`, `hasTrustDialogAccepted`
and `hasClaudeMdExternalIncludesApproved` per project from `.claude.json`
(with `project_path` the project key). Their `timestamp_utc` is the
file's modification time (the manifest `mtime` when there is a
manifest), not the time the entry was added, and their `session_id` is
empty. Two more rows come from the same files, timestamped the same way:
`retention setting: cleanupPeriodDays=<n> (from <file>, file mtime)`
after a settings file's permission rows, the number of days after which
Claude Code's retention sweep deletes a transcript, and `source host:
github status=<status> (from .claude.json, file mtime)` from the
`githubWebConnectionStatusCache` object of `.claude.json` (written by
Claude Code 2.1.310 once the GitHub app is connected; `connected` is the
value seen), with an empty `project_path`. The account and organization
UUIDs beside that status are not printed. A settings file's string `model`
gives `model setting: model=<alias> (from <file>, file mtime)` after its
retention row: the model alias (`fable`, `opus`) `/model` saves as the
default for new sessions, which the VS Code extension's start can also
rewrite, so it is the file's value when collected, not a session's model.
`.claude.json` adds, timestamped the same way, `remote control: spawn mode
<mode> for <path> (from .claude.json, file mtime)` for a project with a
saved Remote Control spawn mode (`same-dir` or `worktree`), with
`project_path` the project, then `remote control: machine id set (from
.claude.json, file mtime)` when Remote Control has been enabled on the
machine (the id is not printed) and `cloud: hasRemoteEnvironment=<value>
(from .claude.json, file mtime)` once a cloud session has been created
from the CLI, both with an empty `project_path`. A Claude Desktop session record adds ` permission_mode=<mode>
effort=<effort>` to its row and one `permission grant: <key> <entry>` or
`permission grant: <key>=<value>` row per session grant, timestamped at
the record's `lastActivityAt` (else `createdAt`), which the text names as
`(as of <field>)`. A per-call decision stays `permission decision:`
(Claude Code) or `permission request:`, `permission response:`,
`permission auto-approved:` and `permission auto-denied:` (the Claude
Desktop audit log). On Claude Code 2.1.309 the decision object also has
`grantScope` (`once` on a one-time approval) and `reasonCode` (`other`,
`unknown`, `subcommandResults`, and on 2.1.310 `bashMiss:no-rule-match`
for a shell command that matched no allow rule), which the row carries in the compact
JSON after `reason=`, as `permission decision: accept
source=user_temporary reason=-
{"decision":"accept","grantScope":"once","reasonCode":"other","source":"user_temporary"}`.

Since 0.21.0 the `claude-code` parser (and `claude-desktop` for the
transcripts it reads with the same reader) also writes these `system`
rows from a session transcript:

| Row text | From | When |
| --- | --- | --- |
| `session surface: <entrypoint>`, then `session surface: <old> -> <new>` | the `entrypoint` key of the file's records: `cli` (the terminal), `claude-vscode` (the VS Code extension), `claude-desktop` (the desktop app's Code tab) | at the first record of each transcript file that carries it, and at each change |
| `prompt trigger: promptSource=<promptSource> turnOrigin=<turnOrigin>` | a prompt's `promptSource` and `turnOrigin`, `-` for a missing key | after a `user` record's rows when `promptSource` is set and not `typed` or `turnOrigin` is set and not `human`: `queued` is a prompt sent from a Remote Control client such as the phone or one typed while a turn ran, `sdk` one from the VS Code extension, from claude.ai to a Remote Control spawn, or the cloud prompt of a teleported session, `system` a task notification, a background shell's completion notice or a `/loop` firing, `suggestion_accepted` a prompt Claude suggested, `turnOrigin=scheduled` a `/loop` firing, `task_notification` a background task's completion |
| `scratch directory: <path> (session scratchpad, from the environment attachment)` | `snapshot.scratchpadDirectory` of an `environment` attachment | once per session and path, with `project_path` the scratchpad, so inventory.json lists it under `scratch_directories` |
| `session continued in: <continuedInSessionId>` | a `continued-in` record | when the session daemon hands a console session to a worker session, whose own file holds the later turns |
| `remote control: bridge session <bridgeSessionId or none> lastSequenceNum=<n>` | a `bridge-session` record (Remote Control) | when the bridge id or sequence number changes; `none` is the detach |
| `session synced: remote control bridge <bridgeSessionId> url <url>` | the first `bridge-session` record of a session that names a bridge id, and the `url` of the `bridge_status` system record that belongs to that bridge (among the status records between the previous and the next `bridge-session` record of another id, the first at or after it, else the nearest before, else `-`); also a `.claude/sessions/<pid>.json` registry file that names a bridge id, as `... url - (from <path>, file mtime)` at the file's modification time | once per session and bridge id: the bridge puts the session on claude.ai, so `sessions.jsonl` `synced` is `yes` for it and `no` for a `claude-code` session without one |
| `session synced: remote event <eventId> at <updatedAt> (from <path>, file mtime)` | a `.claude/projects/<slug>/<session>/ccr-tip.json` file (Claude Code Remote), the last claude.ai event the local side of a Remote Control session processed | one row per file, at the file's modification time, with `session_id` the session directory's name: the session is `synced` `yes` |
| `session synced: cloud session (teleported: remoteSourced records replayed from the vendor's copy)` | the first `remoteSourced` record of a session, a turn of a cloud session replayed by `claude --teleport` | beside the session's `session origin: remote (teleport, remoteSourced)` row; the conversation is held on Anthropic's servers, so the session reads `synced` `yes`, like a Remote Control spawn |
| `remote control: environment <environmentId> session <sessionId> source=<source> (from <path>, file mtime)` | a `.claude/projects/<slug>/bridge-pointer.json` file, the Remote Control environment enabled for the project, which stays after the bridge stops | one row per file, at the file's modification time; `sessionId` is the bridge's, so `session_id` is empty |
| `session origin: remote (Remote Control, entrypoint sdk-cli)` | a transcript whose records have `entrypoint` `sdk-cli` and a prompt with `promptSource` `sdk`: a session started from claude.ai through `claude remote-control` | at the session's first `sdk-cli` record: `sessions.jsonl` `origin` is `remote` |
| `session origin: remote (teleport, remoteSourced)` | a record with `remoteSourced` true: a turn of a cloud session replayed by `claude --teleport` | at the session's first such record |
| `session origin: remote (teleport, cloud scratchpad <path>)` | an `environment` attachment on or before the file's last `remoteSourced` record, in the block a teleport replayed before the local process wrote its own records: the cloud machine's scratchpad, whatever its project directory name | once per session and path, in place of a `scratch directory:` row, since the path is not on this host |
| `session origin: local (VS Code extension)` | a record with `entrypoint` `claude-vscode` | once per session; a `claude-code` session without a `session origin: remote` row reads `origin` `local` |
| `scheduled task fired: <taskId> cron=<cron>` | a `scheduled_task_fire` system record (a `/loop` firing; `-` for a missing key) | in place of the `scheduled_task_fire: <content>` row; the prompt it fires follows as a `user` row with `prompt trigger: promptSource=system turnOrigin=scheduled` |

Each takes its record's timestamp, or the latest one seen in the file for
a record with none (`bridge-session`). The owner account and organization
UUIDs of a `bridge-session` record are not printed. A `/clear` is not a
deletion row: it starts a new transcript file whose first records are the
`/clear` command and its output (`slash command:` and `command output:`
rows, or a `local_command:` row), and leaves the old file as it was,
ending with its `cost-state` line.

A remote URL an agent recorded for the repository it worked in is a
`system` row too, `git remote: <url>`, with the URL exactly as the agent
recorded it after the prefix; inventory.json lists the URLs of these rows
under each project's `recorded_remotes`. A parser writes it once per
session and URL, at the time of the record that names it (since 0.21.0):

- `codex-cli` and `open-interpreter`: the rollout's `session_meta`
  `git.repository_url`, right after the `session start:` row, and the
  `state_5.sqlite` `threads.git_origin_url` at the thread's creation
  time, right after its `thread record:` row. The second is a secondary
  source, so the timeline keeps one of the two when both survive.
- `zed`: the `git_state.remote_url` of the thread's
  `initial_project_snapshot` worktree whose `worktree_path` is the
  thread's project (else of the first worktree, as for the branch), at
  the thread-start time, right after the `thread start:` row.
- `openclaw`: each `projects` row of `state/openclaw.sqlite` that has an
  `origin_url`, at its `created_at_ms`, with `project_path` its
  `repo_root` and no `session_id`, since the table names a repository
  the Gateway registered or cloned, not a session.

The URL can carry credentials in its user part; redact them before
sharing the timeline. That holds for every source, the rollout's
`git.repository_url` included: Codex has removed every password and
every username but the ssh `git` user before storing the value since
late August 2026
([`session_meta` in `research/codex-cli.md`](../../research/codex-cli.md#session_meta)),
but a rollout an earlier build wrote, or one written by a fork such as
Open Interpreter, may hold the URL as the remote had it, and the parser
writes the stored string as it is.

The `codex-cli` and `open-interpreter` parsers (one shared reader) write
`model: <id>` at the first record of a rollout that names a model and
`model: <old> -> <new>` at each later one whose model differs, and one
composite `permission mode: approval=<approval_policy>
sandbox=<sandbox_policy> profile=<permission_profile> network=<network>
reviewer=<approvals_reviewer> profile_id=<active_permission_profile.id>`
row at the first record with any of those fields and `permission mode:
<old> -> <new>` whenever the composite changes. The records are each
`turn_context` and each `thread_settings_applied` event, which the
ChatGPT desktop app writes the moment the user changes the model, the
reviewer or the access level, before any turn, so the row is dated when
the setting changed rather than at the next turn. A settings event
supplies only the thread-level fields, `approval`, `reviewer` and
`profile_id`; `sandbox` and `profile` are the turn's, so a row dated at
a settings event shows them as the last `turn_context` had them, or as
`-` when the access level (`profile_id`) changed, until the next
`turn_context` fills them in, and a settings event before the thread's
first `turn_context` is folded into that turn's row. A
missing field is `-` and an object field is compact JSON, so a value
reads `sandbox={"type":"workspace-write"}`. `reviewer` is `user` when
the user approves each escalated call, and `auto_review` in the desktop
app's "Approve for me" mode, where an automatic reviewer (a guardian
review thread) decides instead; `profile_id` is the desktop app's
access level, `:workspace` or `:danger-full-access` ("Full access", no
sandbox and no approvals), on Codex Desktop 0.163.0-alpha.10.2 inside
ChatGPT 26.1013.40934 (macOS, 2026-10-18). Both rows take the record's
timestamp, and the `model` column of both is the new model. The
`reviewer` and `profile_id` fields were added at the end of the
composite in analyzer 0.16.0, so every `permission mode:` row of these
parsers ends `reviewer=- profile_id=-` when the rollout records
neither.

Each turn of a rollout whose `task_started` record carries
`turn_attribution.turn_trigger` gets a row `turn trigger: <turn_trigger>
turn=<turn_id>` after its `task_started turn=<turn_id>` row. The values
seen with Codex Desktop 0.163.0-alpha.10.2 inside ChatGPT 26.1013.40934
(macOS, 2026-10-18 and 2026-10-19):

| `turn_trigger` | What started the turn |
| --- | --- |
| `composer` | A prompt typed in the desktop app. |
| `remote_ios` | A prompt sent from the ChatGPT iPhone app over remote control (Settings > Connections > Control this Mac or PC): the phone drives the desktop's thread, which stays local. |
| `remote_android` | The same from the ChatGPT Android app. |
| `guardian_review` | The automatic reviewer's turn, in a guardian review thread. |
| `scheduled_task_setup` | The first turn of a scheduled task's thread, when the task is created. |
| `automation_heartbeat_run_now` | A scheduled task run with Run now. |
| `automation_heartbeat_scheduled` | A scheduled task run at its scheduled time. |

Codex records no per-call approval in its rollouts. The desktop app's
are written to `logs_<N>.sqlite`: each user decision is one row
`permission decision: <decision> exec <item id> turn=<turn_id> (from
logs_<N>.sqlite logs, ts)` at the log row's time, with the thread as
`session_id` and the governed item's id as `tool_use_id`, which is the
`tool_use_id` of the call's `tool_use` row; `<decision>` is the log's
own word, `denied` or `approved` on this row. An app-server response to
an approval request (`Decline` or `Accept`) with no such row within two
seconds of it is `permission decision: <decision> request <request id>
(from logs_<N>.sqlite logs, ts)` with no session; on the install
observed those response rows had left the database before it was
collected, while the decision rows above remained. In the "Approve for me" mode the app spawns a guardian review
thread (a rollout with `thread_source` `guardian_review` and the user's
thread as `parent_thread_id`) whose final answer is the verdict: it is
one row `permission decision: <outcome> (auto_review
risk_level=<risk_level> user_authorization=<user_authorization>) exec
<item id> reviewer <review thread id>: <rationale>` at the review's
`task_complete` time, carrying the user's thread as `session_id`, an
empty `model` (the reviewer's model is not the thread's), and
the governed item's id as `tool_use_id` when the log names exactly one
call under review in that window (else `exec -` and an empty
`tool_use_id`); `allow` is the outcome seen. A log row of an approval
whose text the parser cannot read is a `parser: unreadable approval log
row (<target>): <text>` row.

From Codex source, with no real install yet to confirm them, and for
Open Interpreter's home as well where the fork keeps the code: an
`apply_patch` approval in `logs_<N>.sqlite` is `permission decision:
<decision> patch <item id> (from logs_<N>.sqlite logs, ts)`, with the
thread as `session_id` and the `FileChange` item's id as `tool_use_id`;
each record of the `guardian_review_feedback` table of `state_5.sqlite`,
an automatic review that did not allow the action, is a row at the
record's capture time with the reviewed thread as `session_id` and the
governed item as `tool_use_id`. A `denied` record whose decision is the
reviewer's JSON verdict reads exactly as the guardian review thread's
row, so the timeline keeps one of the two; any other reads
`permission decision: <status> (auto_review feedback model=<model>) exec
<item id> turn=<turn id> reviewer <review thread id>: <decision> action:
<reviewed command>`, `<status>` being `denied`, `invalid_decision`,
`missing_decision`, `failed`, `timed_out` or `aborted`. The standing
grants are one row each, `permission setting: <key>=<value> (from
.codex/config.toml, file mtime)` for `approval_policy`,
`approvals_reviewer`, `sandbox_mode`, `default_permissions`,
`projects.<path>.trust_level` (with `project_path` the path) and each MCP
or app tool's `approval_mode`, and `permission setting: <statement> (from
.codex/rules/<file>, file mtime)` for each statement of a rules file
(`prefix_rule(...)` for an "approve and remember" answer,
`network_rule(...)` for a network answer), at the file's modification
time with an empty `session_id`; a project's own `.codex/config.toml` and
`.codex/rules/` give the same rows with the project as `project_path`.
No other key of `config.toml` is shown. A `thread_goal_updated` event is
`thread goal: status=<status> turn=<turn id> objective: <objective>`, and
each `.codex/memories/rollout_summaries/<file>.md` is one row
`memory summary: thread <thread id> rollout=<path> (from <file>,
updated_at): <summary>` at the summary's `updated_at`, with the thread as
`session_id`; it repeats no event, so it is never dropped as a copy.

Each `/feedback` report recorded in `logs_<N>.sqlite`
is one row `session synced: feedback report uploaded to Sentry,
attachments <uploaded_attachments> failed <attachments_failed> (from
logs_<N>.sqlite logs, ts)` at the log row's time, with `-` for a field
the log line lacks and `session_id` the reported thread, empty when no
thread was open. Each row of the `remote_control_enrollments` table of
`state_5.sqlite`, the host's enrollment for remote control, is one row
`remote control: enabled=<0|1> server=<server_id>
environment=<environment_id> host=<server_name>
client=<app_server_client_name> account=<account_id>` at its
`updated_at`, with an empty `session_id`: remote control lets a paired
phone or desktop drive this host's threads, and turning it off keeps the
row with `enabled=0`. It is not a sync: the threads stay on the host.

The `codex-cli` parser also reads the desktop app's
`.codex/.codex-global-state.json`, a secondary source that keeps traces
of a thread the app deleted. Its prompt history gives `user` rows with
an empty `timestamp_utc` and the thread as `session_id`, which pair with
the rollout's prompts on text and are kept, and marked `recovered`, when
the rollout is gone. Its `system` rows are at the file's modification
time and end `(from .codex/.codex-global-state.json <key>, file mtime)`
or `(from .codex/.codex-global-state.json, file mtime)`:
`session deleted: <thread id>` for each thread the app deleted, `thread
description: <description> last_activity=<time>` per thread, `permission
mode (standing): approval=<approvalPolicy> sandbox=<sandboxPolicy>
reviewer=<approvalsReviewer> profile_id=<id>` per thread, `permission
setting: <key>.<host>=<value>` or `permission setting: <key>=<value>`
for the app's agent mode (`agent-mode-by-host-id`,
`preferred-non-full-access-agent-mode-by-host-id`,
`permission-selection-by-host-id:<host>`; `guardian-approvals` is the
"Approve for me" mode), and `remote control: mobile device
connected=<true|false> environment=<id>` when a phone was ever paired.
Since 0.18.0, each entry of the top-level
`thread-projectless-output-directories` map, the directory the app made
for a thread with no project, is also a `scratch directory: <path>
(projectless thread <thread id>)` row whose `project_path` is that
directory (see [`inventory.json`](#inventoryjson)).
A value the file lacks is `-`.

The `gemini-cli` and `qwen-code` parsers write `model: <id>` at a
session's first record that names a model and `model: <old> -> <new>`
on each change, tracked per session id: for Gemini CLI from each gemini
message's `model`, for Qwen Code from the `session_model` record, which
reads `model: <id> auth=<authType>`, and from each assistant record's
`model`; a Qwen Code `logs.json` `model_switch` entry reads
`model: <from> -> <to> reason=<reason>`. A tool call whose status is
`cancelled` or `awaiting_approval` keeps its `[<status>]` prefix on the
`tool_result` row and is followed by `permission decision: <status>`
with the call's `tool_name` and `tool_use_id` (after the `tool_use` row
when there is no result). Each folder in `.gemini/trustedFolders.json`
or `.qwen/trustedFolders.json` is one row
`permission setting: trustedFolders.<path>=<value> (from <file>, file mtime)`,
where `<value>` is the recorded trust level (`TRUST_FOLDER`,
`TRUST_PARENT` or `DO_NOT_TRUST`), with `project_path` the folder, the
file's modification time and an empty `session_id`. Each entry of Qwen
Code's `.qwen/channels/sessions.json` or
`.qwen/channels/daemon/<hash>/routes.json` is one row `session relayed:
channel <channelName> chat <chatId> sender <senderId> thread <threadId>
(from channels/sessions.json, startedAt)`, with `-` for an absent value,
the entry's `sessionId` as `session_id` and its `cwd` as
`project_path`, at `startedAt`, or at the file's modification time with
the text ending `file mtime)`; the daemon form names its `routes.json`.
Since 0.19.0 both parsers read a `trustedFolders.json` with a byte-order
mark, comments or trailing commas, as Qwen Code does. The `gemini-cli`
parser also reads Gemini CLI's other standing permission files, one
`system` row per entry at the file's modification time with an empty
`session_id`: from `.gemini/settings.json` in the home or a project,
`permission setting: <key>=<value> (from <file>, file mtime)` for each
entry of `tools.sandboxAllowedPaths`, `tools.core`, `tools.allowed`,
`tools.confirmationRequired`, `tools.exclude`, `mcp.allowed`,
`mcp.excluded` and `security.allowedExtensions` and each set value of
`general.defaultApprovalMode`, `tools.sandbox`,
`tools.sandboxNetworkAccess`, `security.toolSandboxing`,
`security.disableYoloMode`, `security.disableAlwaysAllow`,
`security.enablePermanentToolApproval`,
`security.autoAddToPolicyByDefault`, `security.blockGitExtensions`,
`security.folderTrust.enabled` and `security.enableConseca`, and
`session retention: general.sessionRetention.<key>=<value> (from <file>,
file mtime)` for each of `enabled`, `maxAge`, `maxCount` and
`minRetention` that is set; from each `.gemini/policies/*.toml` (and
`auto-saved.toml.bak`) in the home or a project, `permission setting:
policy <decision> <toolName> <key>=<value> ... (from <file>, file
mtime)` per rule, its other keys in file order; from
`.gemini/trusted_hooks.json`, `permission setting:
trustedHooks=<name>:<command> (from <file>, file mtime)`; and from
`.gemini/acknowledgments/agents.json`, `permission setting:
acknowledgedAgents.<agent>=<hash> (from <file>, file mtime)`. A
project's file carries the project as `project_path`, and the last two
files the project each entry names. Policy files are read with Python's
`tomllib`, so under Python 3.10 each is one row `parser: <file> not
read: its [[rule]] tables need Python 3.11 or later (tomllib)`. No
`session retention:` row means the defaults: Gemini CLI deletes a
project's sessions older than 30 days each time it starts there, so a
session missing from `chats/` may have expired rather than been deleted,
and its prompts remain in `logs.json`. Gemini CLI's `shell_history`
yields one `user` row per shell-mode command, and a `/chat save`
checkpoint or a `/bug` history export yields a `system` row naming it
(`chat checkpoint: tag <tag> authType=<authType> (from <file>, file
mtime)` at the file's modification time, or `bug report history:
exported (from <file>, time in the file name)` at the time in its name)
and then, for each saved turn, its text as one `user` or `assistant` row
and a `thinking` (when asked for), `tool_use` or `tool_result` row per
thought, call or response part, with no timestamp; all three are
secondary sources (see [Secondary sources](#secondary-sources)). A Qwen Code `user`
record the user did not type is a `system` row `<subtype>
(provenance=<provenance> agent=<agentName> agentId=<agentId>): <text>`,
the parts the record lacks left out: a `cron` or `notification` record
(`cron (provenance=system): ...`), an `agent_message` or `agent_mention`
another workspace agent wrote (`provenance=external_agent`), and a
record of `goal_runtime` provenance; the user's own `agent_mention` stays a `user` row.
A Qwen Code record's `executionContext` (on user prompts) feeds the same
model tracking, as `model: <id> auth=<authType>` or `model: <old> ->
<new> auth=<authType>` (the `auth=` part left out when not recorded),
and an approval-mode tracking per session, `permission mode: <mode>` the
first time and `permission mode: <old> -> <new>` on a change, before the
record's own rows; a `session_approval_mode` record reads `permission
mode: <mode>` or `permission mode: <old> -> <mode>`, followed by
` prePlanMode=<mode>` and ` planExecutionMode=<mode>` when recorded. A
`ui_telemetry` record of a tool call the user was asked about keeps its
`ui_telemetry: <JSON>` row and is followed by `permission decision:
<decision> (tool_call ui_telemetry)`, `<decision>` one of `accept`,
`reject`, `modify` or `auto_accept` (an "allow always" answer), with the
call's `tool_name` and `tool_use_id`. Each entry of `permissions.allow`,
`permissions.ask` and `permissions.deny` in `.qwen/settings.json`, in
the home or a project, is one row `permission setting:
permissions.<list>=<rule> (from .qwen/settings.json, file mtime)`, and
`tools.approvalMode` one row `permission setting:
tools.approvalMode=<mode> (from .qwen/settings.json, file mtime)`, at the
file's modification time with an empty `session_id` (`project_path` the
project for a project's file). Each line of `.qwen/usage_record.jsonl`
is one row `session usage: project <project> duration <ms>ms models
<id>,... tools <tool>=<count>,... (from usage_record.jsonl, startTime)`,
with `-` for an absent part, the record's `sessionId` and `project`, at
its `startTime`; the line is written when a session ends, at `/clear`
and before a session is deleted, so it can be all that is left of one.

The `antigravity` parser writes `model: <id>` before the rows of a
conversation's first step that the `gen_metadata` table names a model for
(`chat_model.response_model`, else `model_display_name`) and
`model: <old> -> <new>` before the rows of each later step whose model
differs, at the step's `created_at`, with the new model in the `model`
column; the format has no switch record, so a change is seen only as a
different model on a later step. A `run_command` or `mcp_tool` call the
user rejected is followed, after its `tool_result` row (which keeps its
`[rejected by user]` marker), by `permission decision: user_rejected` with
the call's `tool_name`, `tool_use_id` and result time, and a
`run_command` result that ran in Antigravity's sandbox ends with
` sandbox=true`.

The `opencode` and `kilo-code` parsers write `model: <modelID>
provider=<providerID>` before the first message of a session that names a
model and `model: <old> -> <new> provider=<providerID>` before each later
message, or V2 `model-switched` record, whose model differs; the `model`
column of the row is the new `<providerID>/<modelID>`. They write
`permission mode: agent=<name>` and `permission mode: agent=<old> ->
agent=<new>` for the message's `agent` and for a V2 `agent-switched`
record, because an OpenCode or Kilo Code agent (`build`, `plan`, a custom
one) is a mode with its own permission ruleset. Both rows take the
message's timestamp and `source_line` and precede its other rows. Since
0.19.0 they read an OpenCode 2.0 database: session-start rows come from
`session_v2` and end with `fork=<id>` for a forked session, and the
`session_message` records of 2.0 add these rows: a `shell` record's
`tool_use` row (tool `shell`) and its `tool_result` row, whose text starts
`[timeout]`, `[killed]` or `[exit <code>]` when the command did not exit
0; `location switched: <old directory> -> <new directory>` with
` workspace=<id>` and ` subpath=<path>` when recorded (the session's
`project_path` stays its `directory`); `skill <name>: <text>`; `idle:
<outcome>` with the error after it when there is one; and `compaction
<reason> <status> <summary>`, all `system` but the shell rows. A session
that a 2.0 database holds only in the 1.x `session` table has a
session-start row ending `(only in the 1.x session table)`, and its rows
come from the 1.x `message` and `part` tables (see
[Secondary sources](#secondary-sources)). The
`crush` parser writes `model: <model> provider=<provider>` and `model:
<old> -> <new> provider=<provider>` the same way, from each message's
`model` and `provider`, where `<model>` is the model that served the
turn: Crush's `prism_model_id` for a Hyper Prism turn, else `model`.
Since 0.19.0 the row ends with ` requested=<model>` when the requested
`model` differs from the served one, and the `model` column reads
`<provider>/<served model>`, or the served model alone when no provider
is recorded. After a `tool_result` row it writes `permission decision:
denied by user` when the result is Crush's `User denied permission`
error, `permission decision: <allow|deny|halt> by hook <names>
reason=<reason>` from the result's hook metadata (` reason=` only when
the hook gave one), and `permission decision: <deny|halt> by hook` for a
hook's block or halt message with no metadata, each with the call's
`tool_name` and `tool_use_id`. Standing entries are `permission setting:
<key>=<value> (from <file>, file mtime)`, one per entry of
`permissions.allowed_tools`, `options.disabled_tools`,
`mcp.<name>.disabled_tools`, `mcp.<name>.enabled_tools` and
`hooks.<event>` in each `crush.json`, of `permissions allow` (as
`permissions.allowed_tools`) and `permissions deny` (as
`options.disabled_tools`) lines in a `crushrc`, where the file reads
`<file> line <n>`, and of the `mcp_enabled_servers` and
`mcp_disabled_servers` tables of `crush.db`, at the file's modification
time with an empty `session_id`. A session bound to an MCP channel gets
`session relayed: channel <server> (from crush.db sessions.channel, as
of updated_at)`, and the first user message a channel pushed for each
source, sender and group `session relayed: channel <source> sender
<sender> group <group> (from crush.db message <id>)`, `sender` and
`group` only when present; the parser records relay state. A sub-agent
session whose parent is gone gives `session deleted: <parent>
(parent_session_id of sub-agent session <id> names no session in
crush.db; time is the sub-agent's updated_at)` with `session_id` the
parent. Each `files` snapshot is `file version: <path> version=<n>
bytes=<length> (from crush.db files)` at its `created_at`, and each
`read_files` entry `file read: <path> (from crush.db read_files,
read_at)` at its `read_at`.
The `opencode` and `kilo-code` parsers write, after a database's
session-start rows, `session synced: share <id> url <url> (from
<database> session_share, time_created)` for each session published to
the share service, at the share's creation time; for a session with no
`session_share` row whose `session.updated.1` events record a share URL
(one shared and then unshared), `session synced: share url <url> (from
<database> event session.updated.1, info.time.updated)` at the first
event that names it and `session unshared (from <database> event
session.updated.1, info.time.updated)` at the first later event without
it, `<database>` being the file name (`opencode.db`, `kilo.db`). The
`kilo-code` parser also writes `session synced: kilo session <id> ingest
<ingestPath> share <url> (from storage/session_share/<sessionID>.json,
file mtime)` for each Kilo cloud session marker, at the file's
modification time, `-` for an absent value.

The `opencode` and `kilo-code` parsers write a `system` row
`permission decision: rejected`, `permission decision: rejected with
feedback`, `permission decision: denied by rule` or `permission
decision: rejected (permission.rejected)` after the `tool_result` row of
a tool call the user or a deny rule refused, with the call's `tool_name`,
`tool_use_id` and result time; an approval leaves no record. After a
database's session-start and sync rows they write `session archived
(from <database> <table>, time_archived)` at the time a session was
archived, `permission setting: session <permission> <pattern>=<effect>
(from <database> <table>.permission, time_updated)` for each rule of a
session's rule list, `scratch directory: <directory> (git worktree of
<project> strategy=<strategy>) (from <database> worktree,
time_created)` (or `project_directory`) for each worktree the agent
made, with that directory as `project_path` and no session, and
`permission setting: <action> <resource>=allow (from <database>
permission, time_created)` for each saved "always" grant, with the
project as `project_path`. From the terminal UI's state directory they
read `prompt-history.jsonl`, a secondary source whose lines are `user`
rows, or `tool_use` rows with tool `shell` for a shell command, with an
empty `timestamp_utc`; `prompt-stash.jsonl`, one `prompt stashed
(unsent): <text> (from <file>, timestamp)` row per prompt set aside; and,
for OpenCode, `session.json`, one `session pinned: <id> (from <file>
pinned, file mtime)` row per pinned session, which outlives a deleted
session. The `kilo-code` parser also writes `permission setting:
permission.<permission>.<pattern>=<action> (from <file>, file mtime)`
(or `permission.<permission>=<action>`) for each rule of the
`permission` key of a global config file in `.config/kilo`, where an
"always" approval writes it; `permission setting: sandbox.<key>=<value>
(from <file>, file mtime)` for each key of a sandbox policy file and
`permission setting: sandbox.enabled=<value>` for a sandbox preference;
`sessions deleted by retention: deleted=<n> scanned=<n>
skippedActive=<n> failed=<n> durationMs=<n> (from retention/state.json,
at)` for the last automatic deletion pass, with no session; and, from
`log/<time>.log`, `remote control: connection enabled connected=<v>`,
`remote control: connection disabled`, `remote control: session started
model=<model> directory=<directory>` and `remote control: session ended
reason=<reason> durationMs=<n> startedAt=<time>`, each ending `(from
log/<file>, <time basis>)`, the session rows with the session's id.
Remote control lets Kilo Cloud drive this host's sessions; it is not a
sync, since the transcript stays in `kilo.db`. These rows are
source-derived; no real install has shown them.

The pi session reader (the `pi` parser, the little-coder sessions in
pi's directory, which stay agent `pi`, and Letta's local-backend
`messages.jsonl`) and the `openclaw` parser write `model: <id>
provider=<provider>` at the first entry of a session that names a model
and `model: <old> -> <new> provider=<provider>` on each change, with
` requested=<model>` when an assistant message's `responseModel` (the
model the provider reported) differs from its `model` (the one asked
for); a `model_change` entry is one of those entries, and one that
names the model already in force, being the user's own switch, still
gets the plain `model: <id> provider=<provider>` row; for `openclaw`
the `session_windows` `model` and `model_provider` start the session at
its start time. `openclaw` writes `session synced: public share <id>
generation <sessionId> (from openclaw-agent.sqlite
session_nodes.entry_json, publicShare.createdAt)` after the start row of
a session published through the Gateway's public link, at the time
access was turned on, and one `backup uploaded: <kind> to <location or
archive path> key <object key or commit> namespace <namespace> (from
state/openclaw.sqlite backup_runs, created_at)` row, with an empty
`session_id`, per successful `openclaw backup` run that uploaded the
state directory to a storage location or committed it to a Git backup
repository whose push did not fail. The `letta` parser adds the same row
after the first
`.letta/sessions.jsonl` line of each CLI session and a change row when a
later line of the session names another model. The `pi` parser reads
`.pi/agent/trust.json`, the project trust decisions pi and little-coder
share, as one `permission setting: <path>=<true|false> (from
.pi/agent/trust.json, file mtime)` row per project, with `project_path`
the project, the file's modification time and an empty `session_id`.

The `openclaw` parser also writes, each text ending in
`(from <file> <table>, <time column>)`, which names the source and the time the row
carries. After the start row of a session's current generation, from the
session's `session_nodes.entry_json` at `updated_at`:
`model: <modelOverride> provider=<providerOverride>
source=<modelOverrideSource>` (with ` fallback_from=<provider>/<model>`
when a fallback replaced the user's choice; the entry keeps no previous
model), one `permission setting: <key>=<value>` per `elevatedLevel`,
`execHost`, `execNode`, `execCwd`, `toolOverrides` and `sendPolicy`, and
`session archived: <archiveReason> by <type>:<id>` at `archivedAt`. One
`session relayed: <channel> account <account> <kind> peer <peer>
thread <thread> role <role>` per chat-platform conversation linked to a
generation, else `session relayed: <channel>
account <account> <chat_type>` from its window, else `session relayed: <channel>
account <account> <chatType> from <from> to <to>` from the session's external
delivery route, and `session relayed: gmail hook` for a session a Gmail
hook started; the Control UI (`webchat`) and the internal wake channels
give none, and `sessions.jsonl` `relayed` reads `no` for a session
without the row. `session deleted: <session_id> archive <archive_name>`
follows each deleted archive's rows, and `session forgotten from
memory: <session_id> agent <agent_id> reason=<reason>` marks each memory
tombstone. A prompt in `session_pending_inputs` that no transcript entry
consumed gives a `user` row and `pending input: <state>, not consumed by
the transcript (input <input_id> run <run_id>; ...)`; one consumed by an
entry the database no longer holds gives its `user` row, and a full-text
index entry the transcript no longer holds its `user`, `assistant` or
`system` row, both marked `recovered`. `channel
delivery: <kind> <status> channel <channel> conversation <id> platform message <id>` and
`channel delivery reply: <text> (reply message <id>; ...)` record the
sends to a platform conversation and the platform's replies. From
`state/openclaw.sqlite`: `permission decision: <decision>
status=<status> kind=<kind> resolver=<resolver_kind>
reason=<terminal_reason> approval <id>` per approval, at its resolution
(its request while pending), with the governed call's `session_id`,
`tool_name` and `tool_use_id`; one `permission
setting: <scope>.<setting>` per default and per-agent exec policy value,
allowlist entry and MCP tool grant (the approvals socket token is never
read), per `allow-always` standing grant (`permission setting: standing
grant <id> ...`) and per allowed channel sender (`permission setting:
channel <channel> account <account> allowFrom=<sender>`); `channel
ingress: <channel> account <account> status <status>: <payload>` per
inbound channel message, the payload whole and with no session; and
`source host: github account <login> id <id> identity <source>;
publication <status> repository <repo> branch <branch> pull
request <url>` per push or pull request the Gateway made on GitHub, with the
session's id. An agent or state database that is only a PostgreSQL
engine anchor is one row, `parser: agent database is a PostgreSQL anchor
(store <store_id>, schema <schema>): transcripts are on the database
server` (`state database ... the Gateway state is on the database
server`).
For a `/bug` report recorded in a session, the `pi` parser writes
`session synced: bug report <id> uploaded to radius.pi.dev
transcript=<true|false> summary=<true|false> (from session entry
pi.bug-report, entry timestamp)` when it was uploaded and `bug report
exported: <id> path=<path> (from session entry pi.bug-report, entry
timestamp)` when it was written to a zip, at the entry's time.
The `pi` parser also writes `permission setting:
defaultProjectTrust=<ask|always|never> (from .pi/agent/settings.json,
file mtime)` when `.pi/agent/settings.json` sets that key, at the file's
modification time with an empty `session_id`; `permission decision:
denied (<reason>)` after the `tool_result` row of a call little-coder's
permission gate refused (an error result reading `edit cancelled by
user`, `command cancelled by user` or starting `shell whitelist:`), with
the call's `tool_name` and `tool_use_id`; one `crash: <kind> <message>
version=<version> sessionFile=<file> (from .pi/agent/crashes.json,
record timestamp)` row per `.pi/agent/crashes.json` record, the stack on
the lines after it, at the record's time, with the record's `cwd` as
`project_path`, the id in its session file name as `session_id` and `-`
for an absent value; and, for `.pi/agent/pi-debug.log`, one `debug
snapshot: /debug wrote <N> message(s) of the open session, which the log
does not name (from .pi/agent/pi-debug.log, Debug output at line)` row
at the time the log was written, before the log's messages, which have
no `session_id` or `project_path`. The `little-coder` parser reads the
v0.0.x releases' `~/.little-coder`: each saved session starts with
`session saved: little-coder v0.0.x session <id> turn_count=<n>
input_tokens=<n> output_tokens=<n> (saved_at <saved_at>, host local
time)`, and every row of the session carries `saved_at`, host local
time written as if it were UTC, and an empty `project_path`; a refused
call is followed by `permission decision: denied (Denied: user rejected
this operation)`; each checkpoint snapshot is `little-coder v0.0.x
checkpoint <id>: turn_count=<n> prompt=<prompt> files=<path> (backup
<name>|did not exist); ... (created_at host local time)`; and
`.little-coder/config.json` gives `session synced: gist <id>`, `session
relayed: telegram chat <id>` and `permission setting:
permission_mode=<mode>`, each ending `(from .little-coder/config.json,
file mtime)`, at the file's modification time with an empty
`session_id`.
The `letta` parser writes `session synced: server conversation
<conversation id> agent <agent id> server <server> (from
.letta/transcripts path, first captured_at)` before the first row of
each transcript whose agent id does not start `agent-local-`, at that
line's time, `<server>` the agent's `baseUrl` in `.letta/settings.json`,
`api.letta.com` when it has none and `-` when the file has no entry
for the agent; and `session synced: last session on <server> agent
<agentId> conversation <conversationId> (from <file>, file mtime)` for
each `sessionsByServer` entry of `.letta/settings.json` and of a
project's `.letta/settings.local.json` that is not the local backend's,
at the file's modification time, with the conversation as `session_id`.
Each `.letta/sessions.jsonl` line, which Letta writes when a session
ends with `/exit` or `/logout`, is `letta session end: agent=<agent id>
exit_reason=<reason> messages=<n> tool_calls=<n> wall_ms=<ms>` at the
time of exit. Since the review of Letta Code 0.34.17 the `letta` parser
also writes, from source: `permission setting:
permissions.<key>=<value> (from <file>, file mtime)` per entry of the
`permissions` object (`mode`, `allow`, `deny`, `ask`, `alwaysAsk`,
`additionalDirectories`) of `.letta/settings.json`,
`.config/letta/settings.json` and a project's `.letta/settings.json` and
`.letta/settings.local.json`, at the file's modification time with an
empty `session_id`; `session relayed: <channel> chat <chatId> type
<chatType> thread <threadId> account <accountId> agent <agentId>
enabled=<enabled> outbound=<outboundEnabled> detached=<detached>
updated=<updatedAt> (from .letta/channels/<channel>/routing.json,
createdAt)` per route, at `createdAt`, with the conversation as
`session_id`; `scheduled task: <id> name=<name> status=<status>
cron=<cron> timezone=<timezone> recurring=<true|false>
scheduled_for=<time> agent=<agent id> conversation=<conversation id>
fire_count=<n> last_fired=<time> last_run=<time> outcome=<outcome>
reason=<reason> error=<error> cancel_reason=<reason> (from
.letta/crons.json, created_at) description: <description> prompt:
<prompt>` per task of `.letta/crons.json`, at its creation time;
`scheduled task run: <task id> action=<action> status=<status>
outcome=<outcome> reason=<reason> agent=<agent id>
conversation=<conversation id> run=<run id> scheduled_for=<time>
fired=<time> missed=<n> summary=<summary> error=<error> (from
.letta/runs/<task id>.jsonl, ts)` per run log line, at its time; and,
for a turn the remote listener did not finish,
`.letta/listener-state/<hash>/<agent>_<conversation>.json`, `interrupted
turn: agent <agent id> conversation <conversation id> run <run id>
tool_calls=<ids> unstarted=<ids> results=<n> working_directory=<dir>
acting_user=<id> (from <file>, file mtime)` followed by `permission
decision: <approve|deny> reason=<reason> (from <file>, interrupted
turn, file mtime)` per recorded decision and a `tool_result` row per
recorded tool result, each with the call's id as `tool_use_id`, at the
file's modification time, with the conversation as `session_id` and
the working directory as `project_path`. A record without a time of its
own takes the file's modification time and its text ends `file mtime)`;
`-` stands for an absent value.

The `cline`, `continue` and `pearai` parsers write the same `model:`
rows, with the `model` column the new model. Cline's SDK sessions write
`model: <id> provider=<provider>` after the session start (or
`sessions.db` index) row, from the manifest's `model` and `provider`, and
`model: <old> -> <new> provider=<provider>` before a message whose
`modelInfo.id` differs; its legacy tasks write `model: <model_id>
model_provider_id=<id> mode=<mode>` and `model: <old> -> <new> ...` from
the `model_usage` entries of `task_metadata.json` at each entry's time,
and `permission mode: <mode>` and `permission mode: <old> -> <new>` from
the same entries' `mode`. Continue writes `model: <chatModelTitle>` after
the session row and `model: <old> -> <new>` before a history item whose
`promptLogs[].modelTitle` differs; PearAI's chat sessions write
`model: <id>` and `model: <old> -> <new>` from each item's
`promptLogs[].completionOptions.model`. These rows inherit the session's
time like every other row of a Continue or PearAI session. Continue
also writes per-call decisions with the call's `tool_use_id` and
`tool_name`: `permission decision: accepted=<accepted>` after each
`dev_data` `toolUsage.jsonl` record's `tool_result` row, at the record's
time and with an empty `session_id` (the record names none), and
`permission decision: canceled reason=user` (the CLI's `Permission
denied by user`), `permission decision: canceled reason=policy`
(`Command blocked by security policy`) or `permission decision:
canceled` after a session tool call whose `toolCallStates[]` status is
`canceled`, and `permission decision: errored reason=policy` after an
`errored` one whose output is the extension's `Security Policy
Violation`, at the session time. A Continue session with a `mode`
writes `permission mode: <mode>` after its `model:` row, and one whose
`workspaceDirectory` ends in `cn-review-<digits>-<digits>` writes
`scratch directory: <workspaceDirectory> (cn review worktree)`, both
at the session time. Each rule of the CLI's `.continue/permissions.yaml`
is `permission setting: <list>=<rule> (from .continue/permissions.yaml,
file mtime)`, `<list>` being `allow`, `ask` or `exclude`, at the file's
modification time; a file in any other YAML layout is one `parser:
unsupported YAML at line <N> (only allow, ask and exclude lists of
strings are read)` row. Each `dev_data` `editOutcome.jsonl` line is
`permission decision: edit accepted=<accepted> file=<filepath> lines
<previousCodeLines> -> <newCodeLines>`, each `editInteraction.jsonl`
line `edit interaction: file=<filepath>` before its prompt (`user`) and
edited file (`assistant`), and each `chatFeedback.jsonl` line `chat
feedback: feedback=<true|false|-> model=<modelTitle>` after its prompt
and reply, all at the line's time and with `-` for an absent value. A
Cline
SDK session whose metadata has the `enterprise.blobUpload` key gets
`session synced: enterprise prompt upload to <adapterType> bucket
<bucket> object sessions/<userDistinctId or unknown>/<session_id>/messages.json
endpoint=<endpoint> region=<region> account=<accountId> (from
<session_id>.json metadata enterprise.blobUpload, started_at)` after its
`model:` row, at the session's start and with `-` for an absent value,
or the same row ending `(from sessions.db sessions.metadata_json
enterprise.blobUpload, started_at)` for a session known only from
`sessions.db`; each item of the VS Code extension's upload queue
`cache/sync-queue.json` is `enterprise prompt upload <status> key
<key> retries <retryCount> (from
saoudrizwan.claude-dev/cache/sync-queue.json, item timestamp)`, prefixed
`session synced: ` only when `<status>` is `synced` (a `pending` or
`failed` item has not reached the bucket), with the item's `taskId` as
`session_id`, at the item's time. A Cline SDK tool result for a call
the runtime did not run is followed by `permission decision: rejected
reason=<reason>` when the reason ends with the runtime's `-- NOT a tool
or system failure. Clarify with user before proceeding.` (written
without it), or `permission decision: blocked reason=<reason>` for
`Tool "<name>" is disabled by policy` and `Tool <name> was blocked by a
runtime hook`, with the result's `tool_use_id`, `tool_name` and time.
Cline's standing settings are one `permission setting: <key>=<value>
(from <file>, file mtime)` row each, at the file's modification time:
`autoApprovalSettings.enabled`, `.maxRequests`, `.favorites` (per entry)
and `.actions.<name>` and `mode` from `.cline/data/globalState.json`;
`toolAutoApprove`, `planActMode`, `disabledTools`, `disabledPlugins` and
`disabledAgentPlugins` (per entry) and `tools.<name>.enabled` from
`.cline/data/settings/global-settings.json`; and
`mcpServers.<name>.autoApprove` (per entry) and
`mcpServers.<name>.disabled` from `settings/cline_mcp_settings.json`.
Each Cline Cloud session in `globalState.json` `cloudSessionStatuses`
gets `session origin: remote cline-cloud session <id> status <status>
(from .cline/data/globalState.json cloudSessionStatuses, observedAt)`
and `session synced: cline-cloud holds session <id>, no local transcript
(from ...)` at its `observedAt`, with the id as `session_id`, and each
record of a `.cline/data/cloud-pending-starts/*.json` file the same pair,
the origin row reading `sandbox start pending account=<account>
endpoint=<endpoint>`, at the file's modification time, so such a session
has `origin` `remote` and `synced` `yes`; every other Cline session has
`origin` `local`. The `cloudTaskTarget` is `cloud task target:
target=<target> repository_id=<id> branch=<branch> (from
.cline/data/globalState.json, file mtime)`. A Cline session imported
from another agent names it on its start row: `imported_from=<tool>
imported_session=<id> imported_path=<path> imported_at=<time>
imported_provider=<provider> imported_model=<model>`. The hook audit log
`.cline/data/logs/hooks.jsonl` gives a `hook audit: <hookName> ...` row
for each `agent_start`, `agent_abort`, `agent_error` and
`session_shutdown` line, with `cline_version=`, `user=`, `agent=`,
`parent_agent=`, `reason=`, `pid=`, `source=` and `error=<name>:
<message>` when present. Continue's `.continue/logs/cn.log`, and the `cn<N>.log`
files it is rotated to, give `agent metadata posted: agent <agentId>
keys <metadataKeys> <status> (from logs/cn.log, log line time,
host-local)` for each metadata post of `cn serve --id`, `<status>`
being `Success`, `Unexpected response`, `Auth required`, `Failed`,
`Error` or `-` when no result line follows, and `storage upload
requested, skipped (from logs/cn.log, log line time, host-local)`, and
for each rule a "don't ask again" answer added to `permissions.yaml`
`permission decision: allow always rule=<rule> (from logs/cn.log, log
line time, host-local)`, all with an empty `session_id` and the log's
host-local time written as if it were UTC. `roo-code` records no
model and writes no such row.

The `aider` parser reads the announcement lines Aider writes to
`.aider.chat.history.md` at startup and after each `/model`,
`/weak-model`, `/editor-model` or `/chat-mode` switch, and after the
`tool_result` row of the blockquote run that holds them writes
`model: <name>` at a session's first announcement and `model: <old> ->
<new>` on each later one that names another main model, the same text
followed by ` (editor model)` or ` (weak model)` for those two roles,
with the new model in the `model` column; a `Model:` line, which means
no separate weak model, sets the weak model to the main one without a
row unless it replaces a separate weak model. Each confirmation line
is `permission decision: <decision> (<question>) answer=<letter>
default=<default>` after the same kind of `tool_result` row, with
`<decision>` `yes`, `no`, `all`, `skip all` or `no, don't ask again`;
a `y` or `n` answer ends `; may be automatic (--yes-always)`, because
Aider writes the same line when `--yes-always` answers for the user.
Both take the run's time, a session header's or the last prompt's.

The `shellgpt` chat files record no model, permission or time. From
`.config/shell_gpt/.sgptrc` the parser reads four keys and no other,
since the file also holds the API key: `DEFAULT_MODEL` is `model:
<value> (configured default, from .config/shell_gpt/.sgptrc, file
mtime)`, with the value in the `model` column, and
`OPENAI_USE_FUNCTIONS`, `DEFAULT_EXECUTE_SHELL_CMD` and
`SHELL_INTERACTION` are each `permission setting: <key>=<value> (from
.config/shell_gpt/.sgptrc, file mtime)`, in file order. A line of that
file with no `=` is `parser: unparseable line <n> (no "=" separator; not
shown, the file holds the API key)`, without the line. Each file in the
function store is `permission setting: function <file name> (from
.config/shell_gpt/functions, file mtime)`; the function calls in the
chat files ran with no confirmation. All of these rows carry the file's
modification time and an empty `session_id`.

The `tabby` parser records no model per message and writes no `model:`
row. From the server's `.tabby/config.toml` it writes one row per
configured model, `model configured: <group> <model_name or model_id>
via <http|local>[ kind=<kind>][ endpoint=<api_endpoint>][
supported_models=<a,b>] (from config.toml [model.<group>.<http|local>],
file mtime)`, `<group>` being `completion`, `chat` or `embedding`, at the
file's modification time; under Python 3.10, which has no `tomllib`, the
one row `parser: config.toml model tables not read: Python 3.11 or later
needed` instead. From the database it writes `page start | title <title>
| author <email (name)> | code_source <id>` for each AI-written page,
with `session_id` `page:<id>`, followed by the page content and its
sections as `assistant` rows; `source host: <host> integration
<display_name> kind=<kind> api_base=<api_base> (account not recorded;
from integrations table, created_at)` for each GitHub or GitLab
integration, `<host>` being `github.com`, `gitlab.com` or the host of a
self-hosted `api_base`; and `source host repository: <git_url>
type=<kind> name=<name> active=<yes|no> (integration <display_name>;
from provided_repositories table, updated_at)` for each repository
cached from one, both at their own times. The editor plugins'
inline-edit command histories, from the IntelliJ plugin's
`intellij-tabby-command-history.xml` and the `edit.recentlyCommand` key
of the `TabbyML.vscode-tabby` row of a VS Code or fork `state.vscdb`,
give `inline edit command history: <N> commands, newest first (from
<source>, no times)` followed by one `user` row per command, newest
first, with an empty `timestamp_utc`.

The `pearai` parser writes per-call decisions on the agent surface (the
Roo Code-based agent pane, as opposed to the chat), from
the Roo Code 3.15.3 result texts of `api_conversation_history.json`: a
tool result that starts `The user denied this operation.` is followed by
`permission decision: denied`, one that starts `The user denied this
operation and provided the following feedback:` by `permission decision:
denied with feedback`, and one that starts `The user approved this
operation and provided the following context:` by `permission decision:
approved with feedback`, each with the call's `tool_name` and
`tool_use_id` at the API message's time; beside a `ui_messages.json` the
API history yields these rows only, with the `tool_use_id` of the last
tool ask of `ui_messages.json` before the message. An approval with no
note leaves no mark. The `pearai.pearai-roo-cline` row of the editor's
`state.vscdb` gives one `task: <task> | tokensIn=<n> tokensOut=<n>
totalCost=<n> number=<n>` row per `taskHistory` entry at the entry's
`ts`, with the task id as `session_id`, ending ` | creator mode:
newProjectType=<type> newProjectPath=<path>` for a task PearAI Creator
started, and one `permission setting: <key>=<value> (from state.vscdb
pearai.pearai-roo-cline, file mtime)` per standing auto-approval key
(`autoApprovalEnabled`, `alwaysAllowReadOnly`, `alwaysAllowExecute`,
`allowedCommands` and the other `alwaysAllow` keys) at the file's
modification time. On the chat surface, the `PearAI` output channel log
gives `model: <id>` and `model: <old> -> <new>` from each request's
`Settings:` block, tracked over the file; a request whose prompt is not in
the `<user>` and `<assistant>` block form (an autocomplete request) is
`model request: <prompt>` and its reply `model completion: <reply>`.
`dev_data/chat.jsonl` gives `user feedback: <true|false> (from
dev_data/chat.jsonl)` after each rated reply, with the line's
`sessionId`, and `dev_data/tokens_generated.jsonl` one `tokens
generated: model=<model> provider=<provider> promptTokens=<n>
generatedTokens=<n>` row per request; all of these rows have an empty
`timestamp_utc`, since neither file records a time.

The `roo-code` parser writes Roo Code's permission state as `system`
rows. In a task's `ui_messages.json`, a tool approval ask marked
`isAnswered` gives `permission decision: approved isAnswered=true (user
or auto-approval)` after the ask's `tool_use` row, with its `tool_use_id`
and `tool_name`: the record does not say whether the user clicked or the
auto-approval settings approved it. In the API history, a `tool_result`
that opens with Roo Code's denial or approval JSON gives `permission
decision: denied` or `permission decision: approved`, with ` feedback=<text>`
when the user typed a note, joined on the call's `tool_use_id` and
`tool_name`; it follows the `tool_result` row when the API history is
the task's only transcript, and is the API history's only row beside a
`ui_messages.json`. Each task row from `history_item.json` or
`tasks/_index.json` is followed by `permission mode: <mode>`, the task's
mode slug (`code`, `architect`, `ask`, `debug` or a custom mode). The
standing settings are `permission setting: <key>=<value> (from <file>,
file mtime)`, one row per scalar or list entry, at the file's modification
time and with an empty `session_id`: the auto-approval switches, command
allow and deny lists and the current `mode` from the extension's global
state (the `RooVeterinaryInc.roo-cline` row of an editor's
`state.vscdb`, named `<file> ItemTable RooVeterinaryInc.roo-cline`) and
the CLI's `.vscode-mock/global-storage/global-state.json`;
`requireApproval`, `dangerouslySkipPermissions` and `mode` from
`.roo/cli-settings.json`; and `mcpServers.<name>.alwaysAllow=<tool>` from
`settings/mcp_settings.json` and a project's `.roo/mcp.json`, with the
project as `project_path`. Releases up to 3.53.0 also signed in to Roo
Code Cloud, the vendor's hosted service, and uploaded each task message
while task sync was on; no file records which tasks were uploaded, so the
parser writes no `session synced:` row. It writes the settings cache that
decided it, `sync setting: user-settings.settings.taskSyncEnabled=<value>`
(`unset (defaults to true)` when the key is absent) and `sync setting:
organization-settings.cloudSettings.<key>=<value>` for `recordTaskMessages`,
`enableTaskSharing`, `allowPublicTaskSharing`, `taskShareExpirationDays`,
`allowMembersViewAllTasks` and `workspaceTaskVisibility`, each ending
`(from <file>, Roo Code Cloud settings cache, file mtime)`, and, for each
upload that failed and was queued for retry in the workspace state,
`cloud upload queued: <event> task=<task id> type=<type>
retries=<retryCount> url=<url> lastError=<error> (Roo Code Cloud retry
queue, from <file> roo.retryQueue, request timestamp)` at the time it was
queued, with the task as `session_id`, ` lastError=<error>` only when one
is recorded, and `-` for an absent value. The queued request's headers and
body are never written.

The `vscode`, `cody` and `twinny` parsers write the same `model: <id>`
and `model: <old> -> <new>` rows, from the request `modelId` (VS Code),
the assistant message `model` (Cody, per chat) and `meta.model` (Twinny,
per conversation), before the rows of the request or message that
names the model. VS Code also writes `permission mode: <mode>` and
`permission mode: <old> -> <new>` from the request's chat mode (`ask`,
`agent`, `edit`, `custom`), then `permission level: <level>` and
`permission level: <old> -> <new>` from the request's permission level
(`default`, `assisted`, `autoApprove`, `autopilot`); when the request
records the model's options, they follow the model id on the model row
as sorted `key=value` pairs (`model: <id> <key>=<value>`), and a change
of options alone is a model change. Per-call decisions are `permission
decision:` rows. In VS Code, a tool call's confirmation follows its
`tool_result` row with the call's `tool_use_id`: `permission decision:
confirmed=true` or `confirmed=false` (before VS Code 1.104), else
`confirmed=<kind>(<number>)`, one of `Denied(0)`,
`ConfirmationNotNeeded(1)`, `Setting(2)`, `LmServicePerTool(3)`,
`UserAction(4)` and `Skipped(5)`, followed by that record's `source`,
`id`, `scope`, `selectedButton` or `selectedButtonKind`, the tool's
origin as `tool_source=<type>` and any `reason` last. A question put to
the user in the chat is `permission decision: confirmation
isUsed=<true|false> buttons=<list> title=<title>` with its message on
the next line, and the request that answering it sent is followed by
`permission decision: confirmation=<button label>`. Cody's confirmation
steps are `permission decision: <decision> confirmation state=<state>
error=<message> title=<title>`, the parts the step lacks left out, with
the step's description and content on the next lines. Since 0.19.0
`<decision>` is `rejected` for a step the user refused (saved as state
`error` with the message `Command rejected`) and `approved or
unanswered` for a `pending` step, because Cody saves an approval and an
unanswered request the same way; any other step reads `permission
decision: confirmation state=<state> ...` with no separate decision
word. Cody's standing settings in the editor's `User/settings.json` are
`permission setting: <key>=<value> (from <file>, file mtime)` rows, one
per leaf of `cody.agentic.context.experimentalOptions` (such as
`cody.agentic.context.experimentalOptions.shell.allow=["git"]`), for
`cody.agentic.context.experimentalShell` and `cody.chat.agenticContext`,
and for `disabled` and `disabledTools` of each `cody.mcpServers.<server>`;
its model preferences are `model setting: <selected|defaults>.<usage>=<id>
endpoint=<endpoint> (from <file>, file mtime)`, from
`JetBrains-globalState/cody-model-preferences` or, ending `(from <file>
cody-model-preferences, file mtime)`, from the `state.vscdb` row, with
the `model` column the id. Both kinds have an empty `session_id` and the
file's modification time, not the time the setting was made. A JetBrains
project's legacy `.idea/cody_history.xml` gives, per chat, `chat:
<internalId> | account=<accountId> | messages=<n> (legacy JetBrains
history)`, `model: <id>` before the first assistant message, and the user
and assistant rows, with the chat's `updatedAt` as `session_id` and time
and the project as `project_path`. VS Code's session-start row ends with `workspace=<path>` when the
chat's workspace is a `.code-workspace` file (since 0.19.0); the
`project_path` is the session's `workingDirectory`, else the
workspace's folder, never that file. Twinny's tool steps saved while waiting for the user's Run or
Skip, or skipped, are `permission decision: waiting` and `permission
decision: skipped`, each with ` approval=<command|change>` when the
step records what was asked, after the step's `tool_result` row with
its `tool_use_id`. Twinny's standing grants are `system` rows with an
empty `session_id` at the modification time of the editor's
`globalStorage/state.vscdb`, each ending ` (from state.vscdb
rjmacarthy.twinny, file mtime)`: `permission mode: agentMode=<true|false>`,
`permission setting: autoRunCommands=<true|false>`, one `permission
setting: alwaysRunCommands=<command>` per command the user chose Always
run for, one `permission setting: alwaysRunMcpTools=<server/tool>` per
MCP tool, and one `permission setting: twinny.teamPolicy.<dotted
key>=<value>` per leaf of the team gateway policy the user accepted (a
list gives one row per entry). Each file a reply changed is `file
changed: <status> <path> +<added> -<removed> reply=<id>`, with
` undone=true` and ` noUndo=true` when set: in the conversation, after
the reply's tool steps, and again from the reply's `agent-changes/`
record with the record's `root` as `project_path`, its `createdAt` as the
time and an empty `session_id`; `reply=<id>` joins the two, which
deduplication cannot pair because the conversation records neither the
project nor the reply's time. Each entry of the Twinny log channel is
`log <level>: <message> (from <file name>, log line time, host-local)`,
its continuation lines (a Debug block's prompt or reply) on the
following lines, with an empty `session_id`, `source_line` the entry's
first line and the log's host-local time written as if it were UTC.
The `rjmacarthy.twinny` row of a workspace's
`workspaceStorage/<id>/state.vscdb` gives `source host repository:
https://github.com/<owner>/<repo> type=review (from workspaceStorage
state.vscdb rjmacarthy.twinny, file mtime)` for the repository the pull
request review tab last listed, at that file's modification time.
The `vscode` parser writes `session synced: remote session
<cloudSessionId> task <cloudTaskId> (from
User/globalStorage/github.copilot-chat/cloudSessions.json, file mtime)`
for each chat session Copilot Chat's session sync uploaded to the
user's GitHub account, with the chat session id as `session_id` and the
file's modification time, and `session synced: cloud agent task
<taskId> application=<application> (from state.vscdb
GitHub.copilot-chat github.copilot.cloudAgent.ownedTasks, recordedAt)`
for each cloud agent task started from the editor, with `session_id`
`copilot-cloud-agent:/task/<taskId>`, at the time it was recorded; `-`
stands for an absent value. The task ran on GitHub's servers from the
start, but GitHub holds its copy as it does an uploaded session's, so
its session's `synced` in `sessions.jsonl` is `yes`. A second row at
the same time and session, `session origin: remote GitHub cloud agent
(from state.vscdb GitHub.copilot-chat
github.copilot.cloudAgent.ownedTasks, recordedAt)`, records where it
began, so its `origin` is `remote`.
VS Code's standing tool approvals are one row per entry, `permission
setting: <key>.<id>=<value> (from User/<path>/state.vscdb, file
mtime)`, for the keys `chat/autoconfirm`, `chat/autoconfirm-post`,
`chat/servers/autoconfirm`, `chat/servers/autoconfirm-post` and
`chat/autoconfirm-combination` of a profile's or a workspace's
`state.vscdb`, `<value>` being `true`, the tool's label or
`{"arguments":...,"label":...}`; `permission setting:
chat.tools.global.autoApprove.optIn=<value>` and one `permission
setting: chat.externalPath.workspaceAllowlist=<path>` per allowed
folder end the same way. Each auto-approve setting of a profile's
`settings.json` (the keys matching `chat.tools.*autoApprov*`) and
`chat.permissions.default` is `permission setting: <key>=<value>`, or
one `permission setting: <key>.<entry>=<value>` per entry of an object,
and `chat.defaultModel` is `model setting: chat.defaultModel=<value>`,
each ending `(from User/<path>/settings.json, file mtime)`. All of them
carry the file's modification time and an empty `session_id`; a
workspace's rows carry the workspace's folder as `project_path`. The
GitHub account VS Code signed in with is `source host: <host> account
<login> extension=<id> scopes=<scopes> (from state.vscdb
github-<login>-usages, lastUsed)`, or `mcp_server=<id>` from
`github-<login>-mcpserver-usages`, one per extension or MCP server that
used the account, at its last use, and each repository picked for a
cloud agent task in the last week is `source host repository: <owner/repo>
type=cloud agent selection (from state.vscdb GitHub.copilot-chat
userSelectedRepositories, timestamp)`, both with an empty `session_id`.
The chat editing state gives `edit state: <Modified|Accepted|Rejected>
<path> request=<requestId> model=<modelId> (from chatEditingSessions
state.json, file mtime)`, with ` deleted=true` before the parenthesis
for a file the edit deleted, one per file of the session's most recent
snapshot, with the chat session id and the workspace's folder, at the
file's modification time.

The `chatgpt-desktop` parser reads the ChatGPT desktop app's list of
the account's ChatGPT conversations, the `chatgpt` rows of
`.codex/sqlite/codex-dev.db` `local_thread_catalog`, which the app
writes at sign-in; a conversation's text stays on OpenAI's servers and
is not on disk. Per conversation it writes three `system` rows with the
conversation id as `session_id`, at the row's `source_updated_at`:
`account conversation: <title> origin=<conversation_origin>
created=<UTC> updated=<UTC> host=<host_id> (from
.codex/sqlite/codex-dev.db local_thread_catalog)`, where `host_id` is
`chatgpt:<workspace uuid>:<user id>` and `-` stands for an
absent value; `session origin: remote chatgpt.com conversation <id>
(listed at sign-in; its transcript is server-side)`; and `session
synced: chatgpt.com holds conversation <id>, no local transcript`. Such
a session's `origin` is `remote`, its `synced` `yes`, and its turn
counts 0. The update time is the conversation's as of sign-in: a reply
from inside the app did not change it.

The `cursor` parser writes `session origin: remote cursor cloud agent`
for a Cursor cloud agent, which runs on Cursor's servers, with the
agent's `bc-<id>` as `session_id` and its repository
(`github.com/<owner>/<repo>`) as `project_path`: at the cached
conversation's `updated_at` when `conversation-search.db` holds one for
the agent, else at the agent's `createdAt` from the
`cloudAgentRepository.agents.<account>` list in `state.vscdb`. Beside it,
at the agent's `createdAt`, `session synced: cursor cloud agent <bcId>
(vendor copy; from state.vscdb cloudAgentRepository.agents)` records
that Cursor holds the conversation. The
session also has `model: <model>` (with ` requested=<id>` when the
requested model id differs), `source host: github.com <owner>/<repo>
(cloud agent)`, `cloud agent: <name> repo=<repoUrl> status=<status>
workflowStatus=<workflowStatus> archived=<isArchived> killed=<isKilled>
vmStarted=<hasStartedVm> (from state.vscdb cloudAgentRepository.agents,
rowUpdatedAtMs)` and, when the search index holds its text, `cached
conversation text (conversation-search.db, source=cloud-cache): <text>`,
one `system` row because the index runs the prompt and the reply
together; when the user answered the agent's request to authorise an
MCP server, the session also has `permission decision: <userDecision>
mcp auth for cloud agent <bcId> tool_call=<toolCallId> (from state.vscdb
cloudAgentRepository.mcpAuthResolution, file mtime)`, with the tool call
id as `tool_use_id`. Its `origin` is `remote` and its `synced` `yes`; every other
Cursor session's `synced` is `no`, since the editor records no sync of a
local conversation.

The `zed`, `goose`, `kiro`, `hermes` and `ollama` parsers write model rows
in the same form. `zed` writes `model: <model> provider=<provider>` once
per thread, after the thread-start row, since a thread blob keeps one
model. `goose` writes `model: <model_name> provider=<provider_name>` after
each session's start row and `model: <old> -> <new> provider=<provider>`
before a message whose recorded model differs, with
` requested=<requestedModel>` when the resolved model is another id.
`kiro` writes `model: <id>` and `model: <old> -> <new>` before the rows of
the history entry whose `request_metadata.model_id` names it, ending
` source=model_info` or ` source=model` when the entry had none and the
conversation-level field stood in; the entries it recovers from a
checkpoint snapshot or the tangent-mode main history track their own
model sequence. `hermes` writes them from the
`session_model_usage` table at each model's first use, with
` provider=<billing_provider>`, or one `model: <id>` from the session row
at its start. Since 0.19.0 a switch made in the Hermes TUI or desktop
app, stored as a message with `display_kind` `model_switch`, is also a
row `model: <old> -> <new> provider=<provider>` at the switch time,
`<old>` being the previous switch's model, else the last model the
`session_model_usage` table first saw before it; `model: <new>` when
neither is known, and `model_switch: <text>` when the stored message
does not parse. Such a switch can therefore appear twice, once from the
stored message and once from the usage table at the model's first use.
`ollama` writes them before the assistant message whose
`model_name` names the model; it also writes one `permission setting:
integrations.<name>.automode=<true|false> (from .ollama/config.json, file
mtime)` row per integration whose launcher configuration holds the key,
and `permission setting: autoModeEnabled=<true|false> (set by Ollama's
Claude Desktop launcher, from <file>, file mtime)` from the Claude Desktop
profile `configLibrary/00000000-0000-4000-8000-000000000114.json` under a
`Claude-3p` base, a file the catalog attributes to `claude-desktop`; both
at the file's modification time with an empty `session_id`. `goose` also writes
`permission mode: <goose_mode>` after each session's start row, and,
for a session the gateway relayed through its Telegram bot, `session
relayed: gateway <platform> user <user_id> display <display name> (from
sessions.db session_type gateway, created_at)`. A tool confirmation in
`sessions.db` is `permission request: <toolName> | <prompt> |
<arguments>` (an empty prompt left out) and its answer `permission
decision: <permission> (toolConfirmationResponse)`, `<permission>` one
of `allow_once`, `always_allow`, `deny_once`, `always_deny` and
`cancel`, both with the tool call's `tool_use_id` and `tool_name`; a
tool call the permission check refused or held, marked
`goose.executable` `false` and never answered, is followed by
`permission decision: not executable (goose.executable=false)`. Its
standing lists are `permission setting: <category>.<level>=<tool> (from
<file>, file mtime)` per tool in `permission.yaml` (`user` or
`smart_approve`, then `always_allow`, `ask_before` or `never_allow`)
and `permission setting: roaming_trust.<allowed|revoked_keys>=<key>
(from <file>, file mtime)` per peer key in `roaming_trust.json`, and
each job in `schedule.json` is `scheduled task: <id> <cron>
paused=<true|false> running=<true|false> source=<recipe file>
last_run=<time> current_session=<session id> recipe_base_dir=<dir>
(from <file>, file mtime)`, with ` parameters=<compact JSON>` before
`(from` when the job has parameters and `-` for an absent value; these
rows carry the file's modification time and an empty `session_id`. `zed`
writes one `permission setting: <key>=<value> (thread blob, as of
updated_at)` row per grant the user gave for the rest of a thread
(`profile`, each `sandbox_grants.write_paths` and `network_hosts` entry,
and each `sandbox_grants` escape flag that is `true`): unlike the
settings-file rows these live in the thread blob, so their `session_id` is
the thread id and their `timestamp_utc` is the thread's `updated_at`, the
blob's last write rather than the time of the grant. It also writes, after
a failed tool result whose text is Zed's denial, `permission decision:
denied` (the user denied the call) or `permission decision: auto_denied
reason=<text>` (a rule denied it), with the call's `tool_name` and
`tool_use_id`; an approval leaves no record. From the user settings files
(`settings.json`, `global_settings.json`) it writes one `permission
setting: <key>=<value> (from <file>, file mtime)` row per entry of
`agent.tool_permissions` and `agent.sandbox_permissions`, per
`agent_servers.<id>.default_mode` and per `session.trust_all_worktrees`,
the key prefixed with the override block or profile it sits in
(`stable.agent.tool_permissions.default`), and from `db.sqlite` one
`permission setting: trusted_worktrees.<path>=true[ host=<user>@<host>]
(from <file> trusted_worktrees, file mtime)` row per trusted project, with
the path as `project_path`. Its other `db.sqlite` rows are `sidebar thread:
title=<title> archived=<true|false> interacted_at=<time|->
main_worktree=<path|->` per native thread, at the sidebar row's
`updated_at`; `unsent draft: <text>` per draft prompt, at the draft's
`updated_at`, or ending ` (from <file> agent_draft_prompts, file mtime)`
when the draft's sidebar row is gone, and `unsent draft: <text> (thread
blob, as of updated_at)` for a thread blob's own draft; `archived worktree:
<worktree> of <main repository> branch=<branch> staged=<hash>
unstaged=<hash> original=<hash> ref=refs/archived-worktrees/<id> (from
<file> archived_git_worktrees, file mtime)` per archived worktree and
linked thread; and `sidebar terminal: <title> cwd=<directory>` per
sidebar terminal at its `created_at`. Its `scratch directory:` rows are
`scratch directory: <path> (sandboxed terminal temp directory of thread
<id>) (thread blob, as of updated_at)` for a macOS thread's sandbox temp
directory and `scratch directory: <worktree> (Zed-created git worktree
of <main repository>) (from <file> created_git_worktrees, created_at)` for each
linked worktree Zed made, at its creation time, with `host=<host>` after
the main repository for a remote one; both have an empty `session_id`. These rows rest on source
alone. `hermes` also
writes one `debug report uploaded: <url> (from pastes/pending.json,
expire_at minus 6 hours)` row per paste.rs link that `hermes debug
share` uploaded and that is still due for deletion, at the upload time,
with an empty `session_id`: the uploaded logs hold recent messages but
name no session. Since 0.20.1 `hermes` also writes, at a session's
`started_at` after its model rows, `session relayed: <source> chat
<chat_id> type <chat_type> thread <thread_id> user <user_id> name
<user_name> display <display_name> (from state.db sessions, started_at)`
for a session carried through a messaging platform, so its `relayed` is
`yes` and every other Hermes session's `no`; `permission setting:
yolo_mode=<true|false> (from state.db sessions.model_config,
started_at)` when the session's `/yolo` bypass (every approval off) is
recorded; and `scratch directory: <cwd> (git worktree the agent created
in <repo>, branch <git_branch>)` for a session run in a worktree Hermes
made under `<repo>/.worktrees/`. The session-start row ends
` | archived=1` for an archived session and ` | imported from <tool>
<path> session <id>` for a session Hermes copied from Claude Code or
Codex CLI. After a `terminal` tool result that records the approval
outcome, a `permission decision: approved (<approval note>)`,
`permission decision: blocked <error>` (with ` user_summary=<summary>`
when a person was asked) or `permission decision: pending
pattern=<pattern_key> smart_denied=<true|false>` row carries the call's
`tool_use_id`. Standing approvals are `permission setting:
command_allowlist=<pattern> (from config.yaml, file mtime)` per pattern
an "always" answer allowed, and `permission setting:
pairing.<platform>.<user id> user_name=<name> (from
pairing/<platform>-approved.json, approved_at)` (or
`platforms/pairing/...`) per messaging user the gateway's owner
approved, at the approval time. `session deleted: <id>
(from state.db state_meta gateway.retired_session.v1, retired_at)`, with
` sweep` after the id when the idle sweep rather than a delete removed
the session, is the deletion record Hermes keeps after the rows are
gone, at the deletion time and with the deleted session's id. `relay
peer: <source> <session_key> generation <n> (from state.db
conversation_generations)`, with no time or session, names a messaging
chat that ever had a conversation; `bot mode room <name> (<room_id>)
seq <seq> <kind> actor <actor kind>:<actor id> (<display_name>): <text>`
is one event of a desktop Bot Mode group chat, at its time; and
`remote backend: <kind> <label> <url or user@host:port> (from
connections.json, file mtime)`, with ` org=`, ` name=` and ` profile=`
when recorded, is a remote, SSH or Hermes Cloud backend the desktop app
was connected to, with no session. In the secondary sources, a prompt
history entry that starts with `/` is `slash command: <entry>`, each
cron run file starts with `cron run: job <job id> (from cron/output,
file name time, host-local)` and a run document with no prompt section
is one row `cron run output: <document>`, each `response_store.db`
response is `api server response <id> status=<status> model=<model>
conversation=<name> (from response_store.db responses, created_at)`, and
each trajectory line is `trajectory: model=<model> completed=<true|false>
(from <file>, timestamp host-local)` before its turns, whose system
prompt is `trajectory system prompt: <text>`. `-` stands for an absent
value.

The `kiro` parser follows each `cancelled` `tool_result` of an Amazon Q
Developer CLI conversation with a row carrying its `tool_use_id` and
`tool_name`: `permission decision: denied by=n` when the user answered
the approval prompt with a bare `n`, `permission decision: denied
by=deny-rule tool=<name>` when a deny rule of the agent configuration
rejected the call, `permission decision: cancelled by=interrupt` for an
interrupt during tool execution, `permission decision: denied
by=user-text` when the user typed something else instead of approving,
and `permission decision: cancelled` when the message has no prompt; a
deny-rule rejection stored as a plain prompt gives the same deny-rule row
with the tool name and no id. Each agent configuration
(`.aws/amazonq/cli-agents/*.json`, or a project's
`.amazonq/cli-agents/*.json` with `project_path` the project) gives
`permission setting: allowedTools=<entry> (from <file>, file mtime)` per
entry and `permission setting: toolsSettings.<tool>.<key>=<value> (from
<file>, file mtime)` per tool setting, with an empty `session_id`. The
turns it recovers from a conversation's checkpoint snapshots or the main
history held aside in tangent mode follow the conversation's own rows,
each group behind `checkpoint snapshot: tag=<tag>[ tool=<tool_name>]
recovered=<n> entries[: <description>]` at the checkpoint's time or
`tangent mode: main conversation held aside, recovered=<n> entries`. A
project's `.amazonq/.subagents/<agent>.json` gives `delegate task
launched: agent=<agent> pid=<pid>[ trust_all_tools=true]: <task>` at the
launch, `trust_all_tools=true` marking the default agent, which the
delegate tool runs with `--trust-all-tools`, and, once the task ended,
`delegate task <status>: agent=<agent> exit_code=<code>` followed by
`summary:` and `output:` lines at its completion, with `project_path` the
task's working directory and no `session_id`.

OpenHands, Agent Zero, nanobot and Muse Code write the same kinds of row.
`openhands` writes `model: <id>` after each conversation's header row, at
the header's time: the model of the first response (`token_usages[]` in
`base_state.json`, joined on `llm_response_id`), else `agent.llm.model`.
It writes `model: <old> -> <new>` before the first event of a response
whose model changed, `model: <old> -> <new> profile=<profile>[
class=<chosen_class>]` after the agent's own switch
(`SwitchLLMObservation`, `ClassifyAndSwitchLLMObservation`), and, on
`base_state.json` at its modification time, `model: <old> -> <new> (from
base_state.json agent.llm, file mtime)` when the file names a model the
events do not reach (a switch over the Agent Server's REST endpoints). A
rejected call's `tool_result` is followed by `permission decision:
rejected rejection_source=<source> <reason>` with the call's `tool_name`
and `tool_use_id`, and an action's `tool_use` text ends with
` security_risk=<value>` when the event has one. Each entry of a security
analyzer's verdict is `permission decision: analyzed risk=<risk>
analyzer=<analyzer> policy=<policy>[ details=<JSON>]` with the analysed
action's `tool_name` and `tool_use_id` (` action_id=<id>` when the action
is not on disk). At `base_state.json`'s modification time it writes
`permission setting: confirmation_policy=<kind>[ threshold=<threshold>][
confirm_unknown=<true|false>] (from base_state.json, file mtime)`,
`permission setting: security_analyzer=<kind> (from base_state.json,
file mtime)` (`none` when no analyzer is set) and `permission setting:
blocked_actions.<action id>=<reason> (from base_state.json, file mtime)`
per hook-blocked action. A conversation whose working directory is under
the Agent Server's default worktree root gets `scratch directory:
<working_dir> (conversation worktree under the Agent Server's default
conversation_worktree_root /tmp/conversation-worktrees)` after its model
row. `agent-zero` writes
`model: <id>` before an agent's first reply and `model: <old> -> <new>`
on a change, tracked per chat and agent number, with ` agent=<n>` for a
subordinate agent. After the chat start row, at the file's modification
time, it also writes `session synced: provider response <response_id>
previous <previous_response_id> model <provider_model_key> (<n> response
ids) agent=<n> (from chat.json agents[].data.responses_state, file
mtime)` for each agent that chained a turn on a response the provider
stored, and for a chat carried by a messaging plugin `session relayed:
telegram bot <bot> chat <chat id> user <user id> @<username>`,
`session relayed: whatsapp chat <chat id> number <number>` or `session
relayed: email thread <thread id> from <sender>`, each ending `(from
chat.json data, file mtime)`. After those, at the file's modification
time and holding the latest value only, an editor (ACP) session gives
`permission mode: <acp_mode> (acp, as of file mtime)`, `model: requested
<acp_model_id> (acp, as of file mtime)` and `scratch directory:
<workdir_path> (acp container transport, from chat.json
data.workdir_path, as of file mtime)`, whose `project_path` is that
directory, while the chat's other rows carry the editor's workspace
`acp_cwd` as their `project_path`; a model chosen for the chat gives
`model: requested preset <name> (chat override, as of file mtime)`, or
`model: requested chat=<provider>/<name> utility=<provider>/<name> (chat
override, as of file mtime)` for a custom choice, never its API key. A
`warning` log item that records a permission decision is followed, at its
time, by `permission decision: blocked tool=<name> profile=<profile>`
(with `tool_name`), `permission decision: denied automatically (provider
MCP approval request)` or `permission decision: <clarify, passed or
terminated> (infection check)`. Standing settings are `permission
setting: tool_access.<key>=<value>`, `permission setting:
orchestrator.<cli>.<key>=<value>` and `permission setting:
<infection_check|orchestrator>=<enabled|disabled>`, each followed by
` profile=<profile>` and ` project=<name>` when the file is scoped and
ending `(from <file>, file mtime)`, with an empty `session_id`. The
scheduler's `tasks.json` gives `scheduled task: <name> type=<type>
state=<state>` at the task's `created_at` and, for a failed run,
`scheduled task failed: <error>` at `last_run`; the Telegram chat map
gives `session deleted: telegram bot <bot> chat <chat id> user <user id>
ctxid <ctxid> (from state.json, no chat.json in the input, file mtime)`
for a mapped chat whose `chat.json` is not in the input; and the A0 CLI
connector's `.agent-zero/.env` gives `session origin: remote agent-zero
instance <host> context <id> (from .agent-zero/.env, file mtime)` for
the last chat it used, so `agent-zero` sessions read `origin` `remote`
or `local`. `nanobot` writes `model: <preset> (preset)` after
the session start row: the value is a model preset of nanobot's
configuration, not a model id. A `/model <name>` switch gives
`model: <old> -> <new> (preset) requested=<name> resolved=<model id>`
after the reply that confirms it; the session records only its last
preset, so in a switched session the first switch reads `model: <new>
(preset) requested=<name> resolved=<model id>` and the start row's
`model:` row is not written. After the session start row, at its time,
nanobot also writes `session relayed: <channel> chat <chat id> (from
session key)` for a session keyed to a messaging channel (Telegram,
Discord, Slack, WhatsApp, email and the other chat services it answers
through), `session relayed: <channel> chat <chat id> (from
metadata.last_channel)` when the session's last delivery route is such a
channel and differs from its key, and `permission mode:
access_mode=<restricted|full> project=<path> (from session metadata
workspace_scope)` for the last workspace scope a WebUI chat set. Its
`config.json` gives `permission setting: <key>=<value> (from <path>,
file mtime)` per standing setting (`tools.restrictToWorkspace`,
`tools.ssrfWhitelist`, `tools.webuiAllowLocalServiceAccess`,
`tools.webuiAllowRemotePackageInstall`, `tools.exec.enable`,
`tools.exec.sandbox`, each `tools.exec.allowPatterns`, `denyPatterns`,
`sandboxRoBinds`, `sandboxRwBinds` and `allowedEnvKeys` entry, and each
`channels.<name>.allowFrom` entry), `<path>` relative to the home and the
key spelled as the file spells it; its `pairing.json` gives `permission
setting: pairing.approved.<channel>=<sender id> (from <path>, file
mtime)` per approved chat sender and `permission setting:
pairing.pending.<channel>=<sender id> expires_at=<time>
delivery_status=<status> (from <path>, created_at)` per pending request,
at its `created_at`. A cron run record gives `cron run: job <job name>
(<job id>) run <run id> status=<status>` and a trigger run record
`trigger run: trigger <trigger name> (<trigger id>) delivery <delivery
id> status=<status> attempts=<n>`, each with ` error=<error>` when the
run failed, beside the run's prompt and reply; the WebUI sidebar cache
gives `webui sidebar entry: key=<key> title=<title> preview=<preview>
model_preset=<preset> updated_at=<time> (from .webui_session_index.json
version <n>)` per cached chat, absent fields left out. `muse-code` writes, per session log,
`permission mode: approval=<approval> reviewer=<reviewer>
profile=<profile id>` followed by ` cause=<cause>` and
` format=<format>` from the log's permission frame,
`permission mode: security_mode=<value>`, and `permission mode: <old> ->
<new>` on a change of either; `model: <id> provider=<provider>` at the
first model a log names and `model: <old> -> <new>` on a change or a
`/model` command; `permission decision: <outcome>
kind=automated_review_completed risk_level=<r> user_authorization=<u>
status=<s> <rationale>` for the automated reviewer's verdict and
`permission decision: <decision> source=<source kind> policy_result=<p>
amendment=<a>` for the decision applied (`-` for a missing value), both
at the record's place in the log, before the call's `tool_result`, and
carrying the call's `tool_name` and `tool_use_id`; and one
`permission setting: projects.<path>.decision=<decision> (from
.config/muse/trust.json, file mtime)` row per trusted project, with
`project_path` the project, the file's modification time and an empty
`session_id`.

The `claude-code` decision row also ends with
` toolDenialKind=<value> userFeedback=<text>` when the same line records
either key, each printed as recorded and `userFeedback` last because it
is free text. A line with `toolDenialKind` and no `permissionDecision` is
a denial that interrupted the turn (Claude Code 2.1.310 wrote it on macOS
for `No` at the permission dialog and for a declined plan, the reason
being the next prompt) and yields `permission decision: reject
(interrupted) toolDenialKind=<value>`; a line with only `userFeedback`
yields `permission decision: - source=- reason=- userFeedback=<text>`. The
`claude-desktop` parser writes `model: <id>` from each audit log's
`system` `init` line and `model: <old> -> <new>` when an `assistant`
line's model changes, from every audit log whether or not the session's
transcript exists; a `model_refusal_fallback` line, whose fields are not
recorded, is `model_refusal_fallback: <compact JSON of its fields>`. A
Cowork session record adds one `permission grant: <key> <entry>`,
`permission grant: <key>.<member>=<value>` or `permission grant:
<key>=<value>` row, with the same `(as of <field>)` time, per entry of
`approvedToolNames`, `webFetchAllowedUrls`, `egressAllowedDomains`,
`userApprovedFileAccessPaths`, `enabledMcpTools` and
`fileDeleteApprovedMounts`. A scheduled task's row ends with
` permission_mode=<permissionMode> model=<model>` (`-` when absent) and
carries the task's model in the `model` column. Each folder in
`remote-session-spaces.json` is `permission setting:
entries.<sessionId>.folders=<folder> (from <file>, file mtime)`, with
`project_path` the folder and `session_id` empty, because the
`<sessionId>` there is the server-side id of a remote Cowork session.

### Secondary sources

Many agents keep a second copy of some of their events beside their
transcript store: a per-prompt history file, a mirror or projection of
the transcript, a full-text index, a backup or snapshot of the store, a
copy kept by a reset, a compaction or a repair. The table below names
them for each agent. The analyzer parses every source on every run,
even when the matching transcript exists, because a copy can survive
the transcript's deletion, and keeps each event once: a row from a
secondary source is dropped when a row from the agent's primary store has

- the same `host`, `user`, `agent`, `turn_type` and `text`; two
  `tool_use` or two `tool_result` rows that both carry a `tool_use_id`
  match on the same `tool_use_id` instead, and never on text when the
  ids differ, because a mirror can record a call in another form than
  the transcript does (Codex CLI's `thread_history_1.sqlite` holds the
  parsed command, the rollout the model's arguments) and a call id is
  unique within a session; a tool row without an id matches on text.
  Claude Code's rows compare a slash command and a prompt in one form on
  both sides: its `history.jsonl` keeps a command as typed (`/rename
  lab-round`) where the transcript has the command in tags (a `slash
  command:` or `local_command:` row), and a paste as the expanded text
  the parser builds from the placeholder, which the transcript's prompt
  wraps in `<pasted_content id="...">` ... `</pasted_content>`; so the
  command matches as the `user` text `/<name>`, plus ` <args>` when its
  `<command-args>` is not empty, and a prompt with the wrapper removed and
  each run of whitespace collapsed to one space (the whitespace itself
  stays, so prompts that differ by a space stay distinct). A command
  line's window is 600 seconds, because a command that opens a picker or
  an overlay (`/model`, `/help`) is recorded in the transcript when it
  completes; the window extends forward only (a record never precedes
  its line by more than the source's own window), it applies against
  transcript records only (two history files keep their own window with
  each other), and within it a command line may not take a record
  written at or after the next line of the same command that could take
  that record (a line of the record's session, a session-less line of its
  project, or a line with neither), so a line whose own record is missing
  stays, under its own time, rather than taking a later line's record. A `/clear` (its history
  line carries the ended session, its record the new one), a `/logout` and
  an `/exit` typed while a turn ran have no record to pair with and stay;
- the same `session_id`, when the secondary row has one; when it has
  none, the same `project_path` (whatever the primary row's
  `session_id`), when it has one; when it has neither, as a text-only
  prompt history does, any primary row of that host, user and agent;
- when the secondary row has a `timestamp_utc`, a `timestamp_utc` on the
  primary row too, at most the source's tolerance apart, the bound
  included; when the secondary row has none, no time constraint, so it
  matches on text and scope alone. A secondary row with a timestamp never
  matches a primary row without one.

The tolerance is per source: 2 seconds, except for the sources that
stamp an event later than the primary store does, whose wider window the
table below gives.

Matching is one-to-one. The secondary rows with a timestamp go first, in
timestamp order, each matched to the nearest primary row within its
tolerance that no earlier secondary row took, so a transcript cut after
three of five identical prompts keeps two of the five history lines.
Then the secondary rows with no timestamp go, those with a `session_id`
first, then those with only a `project_path`, then those with neither,
each group in input order, each matched to the earliest-timestamped
primary row still untaken (primary rows with no timestamp last, in input
order); taking them second keeps an undated row from claiming the
primary row a timestamped one pairs with, and taking the most specific
first lets a session's copy, not a text-only history line, take the
primary row. The primary row always survives, with its own `source_file`
and `source_line`.

Last, the secondary rows that no primary row took are matched against
the other secondary rows, one-to-one, under the same rule (the tolerance
of the row being tested), and a row that matches is dropped and counted
with the others. Its candidates are every secondary row a primary row
took, so a third copy of a primary event collapses onto the copy that
took it and only the primary row is left, and every secondary row tested
before it: the rows with a timestamp in timestamp order, then those with
none in input order. The copy kept of several is therefore the earliest
one with a timestamp, and a copy with no timestamp matches one with a
timestamp, with no time constraint, rather than the reverse. In this pass
only, a row that finds no candidate under its own scope tries the
broader ones, its `project_path` when it has one and then any scope, so
Antigravity's history line (a project, no session) and its brain mirror
line (a session, no project) of the same prompt meet; under a broader
scope a candidate whose `session_id` or `project_path` differs from one
the row records is never taken, so the same prompt typed in two sessions
stays two events. The copies of one event that several backups,
snapshots, archives or mirrors keep are thus shown once, even after the
primary store is gone. A row matches only a row from a different
`source_file`, so two identical prompts in one history file, with a
timestamp or without, are both kept as the two events the file records.
A `system` row whose text starts with `parser:` is never dropped and
never takes a copy, since it describes its own `source_file` (unreadable,
cut off, or holding an unparseable line), so a history file and a
transcript that fail with the same message are two problems, not one
event.

A secondary row with no match is kept, which recovers a prompt whose
transcript was deleted, excluded by size or truncated, and a prompt the
transcript no longer holds (a Gemini CLI rewind). The rule runs after
`--agent` and before `--since`, `--until` and `--match`, and the rows it
drops are counted per agent on the `dropped:` line of stdout and under
`deduplicated` in [inventory.json](#inventoryjson). Before analyzer
0.13.0 both rows were written. Since analyzer 0.15.0 every secondary
event row kept is marked `recovered` `yes` in `timeline.jsonl`; a
`system` row of a secondary source is not, since it describes the
source or the session rather than a lost turn. A parser whose primary
and secondary copies live in one file pairs them itself and marks the
event rows it took from the secondary copy `recovered` `yes` in the same
way: since 0.19.0 the `opencode` parser reads the 1.x
`message` and `part` tables of a database upgraded to OpenCode 2.0 only
for a session its `session_message` table does not hold, matched exactly
on the session id, the `kiro` parser reads a conversation's checkpoint
snapshots and tangent-mode main history only for the turns the
conversation's history no longer holds, matched on the user message and
its time and the assistant message, and the `openclaw` parser reads the
full-text index (`session_transcript_fts_content`) and the consumed
pending inputs of an agent database only for an entry whose session and
entry id the database's transcript does not hold.
[`sessions.jsonl`](#sessionsjsonl)
`recovered` says whether a session was rebuilt from such rows alone
(`yes`) or in part (`partial`):
`jq -c 'select(.recovered == "yes")' out/sessions.jsonl` lists the
sessions whose primary transcript is gone. Two cases read `partial` or
`yes` without anything lost. Deduplication matches on text, so a
secondary source that records an event in another form than the primary
store (an index that compresses a tool call into one line) keeps that
row, and its session reads `partial`: `partial` marks a session to
compare against its secondary sources, not proof that the transcript
was cut. And Letta's server backend keeps the store of record on the
server, so the local transcript is its mirror, a secondary source, and
a server-backend Letta session reads `yes` or `partial` by design,
beside its `session synced: server conversation` row.

| Agent | Secondary source | Primary store |
| --- | --- | --- |
| `agent-zero` | `usr/scheduler/tasks.json` of the install directory or volume (each task's prompt, with no timestamp, matches on text within the task's chat; its last result is stamped when the run ends: a 60-second window); the Telegram plugin's `usr/plugins/_telegram_integration/state.json` (only `system` rows, for a mapped chat that is gone) | `usr/chats/<ctxid>/chat.json` |
| `aider` | `.aider.input.history` and `.aider.llm.history`, in a home or a repository | `.aider.chat.history.md` beside them |
| `antigravity` | `.gemini/antigravity-cli/history.jsonl`; the brain mirror `.gemini/<dir>/brain/<uuid>/.system_generated/logs/transcript_full.jsonl`, else its chunks, else `transcript.jsonl`, else its chunks, `<dir>` each of `antigravity-cli`, `antigravity`, `antigravity-ide` and `antigravity-backup` (600 seconds: the mirror records a step's start, the database row its completion) | `.gemini/<dir>/conversations/<uuid>.db` |
| `claude-code` | `.claude/history.jsonl` (and `.claude-<name>/history.jsonl`; a `[Pasted text #<n> ...]` placeholder is expanded from the line's `pastedContents` or `paste-cache/<contentHash>.txt` when either holds the paste; `/exit` and other commands the transcript does not record never pair) | `.claude/projects/<slug>/*.jsonl` and its subagent files |
| `claude-desktop` | each Cowork session's `local-agent-mode-sessions/<acct>/<org>/[agent/]<session dir>/audit.jsonl` under the app-data directory; the session records `local_<uuid>.json` beside the session dirs and under `claude-code-sessions/` (a Cowork record's first prompt has no timestamp: matches on text within the session) | the session dir's `.claude/projects/<slug>/*.jsonl` and its subagent files |
| `cline` | `.cline/data/db/session-search.db` (no timestamp per message: matches on text and session; a message of content blocks keeps their type names and tool ids, so only plain-string prompts match); `.cline/data/cache/user_input_history.jsonl` (only the last line timestamped: the others match on text alone); `.cline/data/logs/hooks.jsonl` (tolerance one hour; tool rows match on the call id) | `.cline/data/sessions/<id>/<id>.messages.json` |
| `codex-cli` | `.codex/history.jsonl`; `.codex/state_5.sqlite` (first prompt with no timestamp: matches on text alone); `.codex/thread_history_1.sqlite`; `.codex/.codex-global-state.json` (the desktop app's prompt history, no timestamp: matches on text within the thread); `.codex/db-backups/<folder>/state_5.sqlite` (a copy Codex set aside as corrupt, read as `state_5.sqlite`) | the rollouts under `.codex/sessions/` and `.codex/archived_sessions/` |
| `cody` | a JetBrains project's `.idea/cody_history.xml`, the pre-agent plugin's history (the migration's chat id `updatedAt` as session and time; complete pairs match the migrated chat, an unpaired message is recovered) | the `sourcegraph.cody-ai` row of the editor's `state.vscdb` and `Cody-nodejs/[Data/]JetBrains-globalState/cody-local-chatHistory-v2` |
| `continue` | `.continue/input_history.json` (the CLI's prompt history, no session or project: matches on text at any distance in time); `.continue/dev_data/<schema>/chatFeedback.jsonl` (a rated reply's prompt and completion, stamped at the click: matches within the session at any distance in time) | `.continue/sessions/<id>.json` and the `dev_data` `chatInteraction.jsonl` lines |
| `copilot-cli` | `.copilot/session-store.db` (`turns`, the prompt and the final reply of each turn, stamped when the turn ends: a one-hour window; its `sessions` name pairs with the `workspace.yaml` row; `session_files`, `session_refs`, `checkpoints` and `forge_trajectory_events` rows the log has no copy of are always kept); `.copilot/session-state/<id>/rewind-file-snapshots/index.json` (each turn's prompt, a one-hour window); `.copilot/command-history-state.json` (no timestamp, session or project: matches on text alone); each also under the pre-XDG `.local/state/.copilot` | `.copilot/session-state/<id>/events.jsonl` and `workspace.yaml` |
| `cursor` | `.cursor/projects/<project>/agent-transcripts/<composerId>/<composerId>.jsonl`, the agent worker's copy of each conversation (a prompt is stamped to the minute in local time with its UTC offset: a 60-second window; replies and tool calls have no timestamp and match on text within the conversation; the tool calls name the tools differently from the database, so they never pair) | the editor's `User/globalStorage/state.vscdb` `cursorDiskKV` composer and bubble rows |
| `gemini-cli` | `.gemini/tmp/<slug>/logs.json`; `shell_history` (no timestamp or session: matches on text and project, and pairs with the `logs.json` copy of the command), `checkpoint-<tag>.json` from `/chat save` and `bug-report-history-<ms>.json` from `/bug` (no timestamp or session: match on text and project, tool rows on `tool_use_id`) in the same directory; each also under `.cache/.gemini` | `.gemini/tmp/<slug>/chats/*.jsonl` |
| `goose` | `.local/state/goose/history.txt` and `.config/goose/history.txt` (`AppData/Roaming/Block/goose/data/history.txt` on Windows; no timestamp, session or project: matches on text alone); `.local/state/goose/logs/llm_request.<N>.jsonl` (`AppData/Roaming/Block/goose/data/logs/` on Windows; the last 10 provider calls, with no session or project: a 600-second window; streamed chunks of one message are assembled first; the `llm request` row has no twin and is always kept) | `.local/share/goose/sessions/sessions.db` |
| `hermes` | `.hermes/state-snapshots/<id>/state.db` (copies of `state.db`; several snapshots collapse to one copy; collected only with `--full` before collector 1.18.0) and `.hermes/sessions/<id>.jsonl` (no timestamp on some lines: those match on text within the session); since 0.20.1 also `.hermes/.hermes_history` (no session or project: matches on text and time), `.hermes/cron/output/<job>/<time>.md` (no session: matches on text and time), `.hermes/response_store.db` (no timestamp per message: matches on text within the session) and the `trajectory_samples.jsonl` and `failed_trajectories.jsonl` files of a project (no timestamp per turn: matches on text within the project); all but the trajectory files also under `.hermes<suffix>`, `AppData/Local/hermes`, `AppData/Local/hermes<suffix>`, `profiles/<name>/` and, since 0.19.0, the root of a Docker or Podman volume | `.hermes/state.db` |
| `kilo-code` | `.local/state/kilo/prompt-history.jsonl` (no timestamp, session or project: matches on text alone) | `.local/share/kilo/kilo.db` |
| `kiro` | `.aws/amazonq/.cli_bash_history`, the chat prompt history (no timestamp, session or project: matches on text alone); in each conversation, the checkpoint snapshots and the tangent-mode main history, read only for the turns the conversation no longer holds and paired by the parser itself | `data.sqlite3` `conversations` under `amazon-q/` or `kiro-cli/`, and `/save` exports |
| `letta` | `.letta/transcripts/<agent>/<conversation>/transcript.jsonl` (stamped at the end of the turn: a one-hour window) | `.letta/lc-local-backend/conversations/<key>/messages.jsonl` |
| `little-coder` | the v0.0.x `.little-coder/sessions/history.json` and `sessions/mr_sessions/session_latest.json` (the same sessions under the same id and `saved_at`); `.little-coder/input_history.txt` (no timestamp, session or project: matches on text alone) | the v0.0.x `.little-coder/sessions/daily/<date>/session_*.json` and the `/save` files under `sessions/` |
| `muse-code` | `.local/share/muse/tui-history.jsonl` (no timestamp: matches on text alone) | `.local/share/muse/sessions/.../session.jsonl` |
| `nanobot` | `.nanobot*/webui/<key>.jsonl` and `<key>.segments/NNNNNN.jsonl` (tolerance 15 hours: true UTC against the session file's local time, at most 14 hours apart, plus an hour of turn); `.nanobot*/history/cli_history` (no session or project: matches on text and time); when a `sessions.sqlite3` sits in the namespace directory, the namespace's `<key>.jsonl` files and their `jsonl-backup/` copies, which the database imported (since 0.19.0); a workspace's `cron/runs/*.json` and `triggers/runs/*.json` run records (tolerance 15 hours, as for the WebUI transcript: true UTC); `.nanobot*/sessions/<workspace-id>/.webui_session_index.json` (`system` rows only, which never pair) | `.nanobot*/sessions/<workspace-id>/sessions.sqlite3`, else `.nanobot*/sessions/<workspace-id>/<key>.jsonl` |
| `open-interpreter` | `.openinterpreter/history.jsonl`; `.openinterpreter/state_5.sqlite` (first prompt with no timestamp); `.openinterpreter/thread_history_1.sqlite`; `.openinterpreter/db-backups/<folder>/state_5.sqlite` | the rollouts under `.openinterpreter/sessions/` and `.openinterpreter/archived_sessions/` |
| `openclaw` | `.openclaw/agents/<id>/sessions/` copies of a transcript: the published `*.jsonl.reset.*` / `*.jsonl.deleted.*` archive files, `*.checkpoint.<uuid>.jsonl` and `*.jsonl.pre-doctor-*-repair-*.bak` (each copy's session-start row names it and is kept) | `.openclaw/agents/<id>/agent/openclaw-agent.sqlite` (`transcript_events` and the `session_transcript_archives` blobs) and the legacy `sessions/<id>.jsonl` |
| `opencode` | `.local/share/opencode/opencode-next.db`, the pre-release 2.0 database that 2.0 imports and leaves in place; in a database upgraded to 2.0, the 1.x `message` and `part` tables, read only for a session `session_message` does not hold and paired by the parser itself (since 0.19.0); `.local/state/opencode/prompt-history.jsonl` (no timestamp, session or project: matches on text alone); `.local/state/opencode/session.json` (its `session pinned:` rows are `system` rows and never pair) | `.local/share/opencode/opencode.db` (or `opencode-<channel>.db`): `session_v2` and `session_message` |
| `openhands` | `.openhands/projects/<sha256 of cwd>/prompt_history.json`, the CLI's last 100 prompts per working directory (no conversation id: matches on text and time; `project_path` is the working directory of a conversation whose SHA-256 is the directory name) | `.openhands/conversations/<hex>/events/event-*.json` |
| `pearai` | the `PearAI` output channel log `<editor>/logs/<session>/window<N>/exthost/output_logging_<time>/<n>-PearAI.log` (no timestamp, session or project: matches on text alone; a prompt sent with context items carries them and never pairs); `.pearai/dev_data/chat.jsonl` (the rated replies, no timestamp: matches on text within the session) | `.pearai/sessions/<sessionId>.json` |
| `pi` | `.pi/agent/pi-debug.log`, the last `/debug` snapshot of the open session's messages (no session or project; one hour: a message's own time is its start, the session entry's its end) | `.pi/agent/sessions/<cwd>/<time>_<id>.jsonl` |
| `qwen-code` | `.qwen/tmp/<hash>/logs.json`; `.qwen/usage_record.jsonl` (per-session summaries, `system` rows that no transcript row repeats) | `.qwen/projects/<slug>/chats/*.jsonl` |
| `roo-code` | `.roo/cli-history.json` (no timestamp, task or project: matches on text alone) | the task files under `.vscode-mock/global-storage/tasks/` and the editor's `globalStorage/rooveterinaryinc.roo-cline/tasks/` |
| `shellgpt` | the response cache `AppData/Local/Temp/shell_gpt/cache/<md5>`, one answer per file (no timestamp, session or project: matches on text alone; the answer after a function call is cached without the chat file's `> @FunctionCall` marker, so it never pairs and is kept) | the chat files `chat_cache/<id>` |
| `tabby` | `.tabby/ee/db.backup-*.sqlite` and `dev-db.backup-*.sqlite` (copies of the live threads, pages, events and integrations); `.tabby/events/YYYY-MM-DD.json` (its `user` and `assistant` rows pair with `user_events`; its `system` rows name the owner by encoded id, not email, and never pair); the IntelliJ `intellij-tabby-command-history.xml` and the VS Code `state.vscdb` `edit.recentlyCommand` (no timestamp; the server keeps no copy of an inline-edit instruction, so they never pair) | `.tabby/ee/db.sqlite` or `dev-db.sqlite` |
| `twinny` | the editor's `User/workspaceStorage/<id>/rjmacarthy.twinny/agent-changes/<id>.json` (or under `globalStorage/rjmacarthy.twinny/`; `system` rows with the project root and no session, which never pair); the log channel `logs/.../rjmacarthy.twinny/Twinny.log` and `Twinny.log_<n>` (`system` rows with no session, which never pair) | the `rjmacarthy.twinny` row of the editor's `User/globalStorage/state.vscdb` |
| `vscode` | `User/workspaceStorage/<hash>/chatEditingSessions/<id>/state.json`, the chat editing state (only `edit state:` `system` rows, which never pair and are never marked `recovered`; it outlives a deleted chat) | `User/workspaceStorage/<hash>/chatSessions/<id>.jsonl` or `.json` |

The 2 seconds cover Codex's `history.jsonl`, which records whole seconds
beside the rollouts' milliseconds, and Antigravity's history line, under
a second from the database step. Qwen Code's `logs.json` rows have no
project path and match by `sessionId` only. A known miss: a prompt the
agent queued and sent later (Claude Code queues input typed while it
works) can be minutes from its transcript record, and is then shown
twice, the history row at the time it was typed. Collectors before
1.18.0 excluded Hermes's quick snapshots (`state-snapshots/`) unless run
with `--full`, so in their collections the snapshots appear only in a
full collection or loose input.

Some sources have a wider window, because they stamp an event later
than the primary store does or more coarsely. Cursor's agent transcript
stamps a prompt to the minute, so its rows match within 60 seconds. Antigravity's brain mirror records only
when a step started, while the database rows of a response or a tool
result carry its completion, so its rows match within 600 seconds.
Letta's `transcript.jsonl` stamps every line of a turn when the turn
ends, so its window is one hour; a turn longer than that leaves its
earlier lines shown twice. Copilot CLI's session store writes a turn
when it ends, so its `turns` rows and its rewind snapshots match within
one hour, and only against an identical event of the same session.
Cline's hook audit log `logs/hooks.jsonl` stamps a tool call when it
starts or ends and the transcript stamps the message when the turn is
written, so its rows match within one hour; its call rows match on the
call id.
Goose's request logs stamp a message with the time of its first
streamed chunk, while `sessions.db` stores the later of that time and
the session's newest stored message, so a database row can follow its
log copy by as long as the tool calls before it ran; their window is
600 seconds.
nanobot's WebUI transcript stamps events in
true UTC while its session files record host local time, emitted as if
UTC, so its window is 15 hours (the largest UTC offset, 14 hours, plus
an hour of turn); a WebUI row kept because no session row matched
carries true UTC, unlike every other nanobot row. Continue's two
secondary sources have a window of ten years, in effect none: its
session rows carry the session's creation time rather than the prompt's,
so a prompt history entry is later by as long as the session ran, and a
`chatFeedback` line is stamped when the user rated the reply, any time
after its `chatInteraction` line; a history entry still never matches a
session row with no time, as a session the index lost has. Hermes's
`.hermes_history` (the classic CLI's entries) and cron run files record
host local time, emitted as if UTC, against the true UTC of `state.db`,
so their window is 14 hours plus 2 seconds; the TUI's history entries
are true UTC and fall within it as well. nanobot's cron and trigger run
records stamp in true UTC as well and use the same window; their prompt
row carries the text the session stores for the turn, rebuilt from the
record, and a trigger's content or reply longer than the 4,000
characters nanobot keeps in its record is shown twice.

Several sources record no time for an event, and their rows match on
text and scope alone: Muse Code's `tui-history.jsonl`, Goose's
`history.txt`, Kiro's `.cli_bash_history` and Roo Code's
`cli-history.json` record none at all;
Claude Desktop's Cowork session record keeps its first prompt with no
time of its own; Cline's `session-search.db` records none per message,
and its CLI `user_input_history.jsonl` only on its newest line, since
every rewrite stamps every line with the time of that rewrite; Codex
CLI's and Open Interpreter's `state_5.sqlite` hold a thread's first
prompt beside the thread's creation time, which can precede the prompt,
so the prompt row has none. A row with neither a session nor a project,
such as a Goose `history.txt` line, can match a primary row of any
session of the same user, so a prompt typed in two sessions is dropped
once, against the earliest database row.

Some secondary rows never match, because the primary store has no twin
for them or renders the event differently, and are always kept: Tabby's
event-log `system` rows, which name the server user by an encoded id
where the database gives an email, and its editor command histories,
whose inline-edit instructions the server does not store; Cline's `session-search.db` messages
stored as content blocks, which the index keeps with their block type
names and tool ids; a nanobot WebUI prompt whose session copy has
runtime context appended (a quoted reply, an automation template);
Claude Desktop's permission requests, responses and auto-decisions,
`result` lines and `init: cwd=... model=...` rows, which the audit log
alone records (its `model: <id>` row does match the transcript's), and
the session records' `system` rows (the session, its grants, its error);
and the session-start row each OpenClaw copy begins with, which names
the copy.

Two sources that look like second copies are primary. Ollama's
`.ollama/history` is the `ollama run` REPL's readline history, while
its `db.sqlite` holds the desktop app's chats, a different surface, so
both are parsed as primary records. little-coder's prompt history rows
carry agent `little-coder`, while its transcripts are pi sessions under
agent `pi`; the rule pairs rows of one agent only, so it cannot pair
them, and both stay. Continue's `toolUsage.jsonl` is not a secondary
source either: its event rows carry their own time
but no session or project, and the session rows they repeat carry only
the session's creation time, so a match within 2 seconds would be
chance, and both are kept.

One fallback remains: Cline's `api_conversation_history.json` is read only
when the task's `ui_messages.json` is absent, because the two record different
texts for the same turn (the API history wraps the first prompt in `<task>`
tags, and its tool calls are content blocks where the UI messages have one
JSON record per tool), so the rule could not pair them and the task's turns
would be shown twice; see [Parser plan in
`research/cline.md`](../../research/cline.md#parser-plan).

## sessions.jsonl

`sessions.jsonl` has one record per distinct `host`, `user`, `agent` and
`session_id`, built from the records of `timeline.jsonl` that have a
non-empty `session_id`; records without one belong to no session. It is
built after filtering, so it summarises the rows written. It is sorted by
`first_timestamp_utc`, then `agent`, then `session_id`, so sessions with
no timestamp come first. Its fields, in order:

| Field | Type | Meaning |
| --- | --- | --- |
| `host` | string | The `host` of the session's timeline records. |
| `user` | string | The `user` of the session's timeline records. |
| `agent` | string | The `agent` of the session's timeline records. |
| `session_id` | string | The `session_id` of the session's timeline records, never empty. |
| `project_path` | string | The first non-empty `project_path` among the session's records in timeline order, so a Windows path is in the canonical form described under [`timeline.jsonl`](#timelinejsonl); `""` when none has one. A `scratch directory:` row, which names the agent's own directory rather than the session's project, counts only when no other record has a path (since 0.21.0). `inventory.json` counts the session under this path. |
| `first_timestamp_utc` | string | The earliest non-empty `timestamp_utc` among the session's records, `""` when none has one. |
| `last_timestamp_utc` | string | The latest non-empty `timestamp_utc` among the session's records, `""` when none has one. |
| `models` | array of strings | Each distinct non-empty `model` of the session's records, in order of first use in the timeline; `[]` when none was recorded. |
| `user_turns` | integer | The number of the session's records with `turn_type` `user`. |
| `assistant_turns` | integer | The number with `turn_type` `assistant`. |
| `tool_calls` | integer | The number with `turn_type` `tool_use`. |
| `source_file` | string | The `source_file` that contributed the most of the session's records, the first met on a tie, so a transcript wins over a one-line-per-prompt history file; `""` when no record names one. |
| `synced` | string | Whether the session is synced to a remote account, from the agent's own record of it: `yes` when one of the session's records is a `system` row whose `text` starts with `session synced:` (written by the `agent-zero`, `chatgpt-desktop`, `claude-code` (since 0.21.0), `cline`, `codex-cli`, `copilot-cli`, `cursor`, `kilo-code`, `letta`, `little-coder`, `open-interpreter`, `openclaw`, `opencode`, `pi` and `vscode` parsers); `no` when the agent's parser records sync state and no record carries that row; `""` when the agent's parser records no sync state, so nothing is known. A string rather than a boolean, because `false` would say the agent can sync and did not. Since analyzer 0.14.0. |
| `relayed` | string | Whether the session was carried through a messaging platform (Telegram, WhatsApp, email, a GitHub thread), which keeps its own copy of the conversation, from the agent's own record of it: `yes` when one of the session's records is a `system` row whose `text` starts with `session relayed:` (written by the `agent-zero`, `goose` and `qwen-code` parsers, and since analyzer 0.20.1 by the `crush`, `hermes`, `letta`, `little-coder`, `nanobot` and `openclaw` parsers); `no` when the agent's parser records relay state and no record carries that row; `""` when the agent's parser records no relay state, so nothing is known. A relay is not a sync: the copy is on the platform, not in the agent vendor's account, so a session can read `yes` in both fields (an Agent Zero Telegram chat chained on a stored provider response). The `little-coder` rows in both fields come from the v0.0.x `.little-coder/config.json` and name no session, so every little-coder session reads `no` in both. Since analyzer 0.15.0. |
| `origin` | string | Where the session began, from the agent's own record of it: `remote` when one of the session's records is a `system` row whose `text` starts with `session origin: remote` (written by the `vscode` parser, for a Copilot cloud agent task that ran on GitHub's servers and is only listed in the editor, and by the `chatgpt-desktop` parser, for a ChatGPT conversation the app lists from the account; since analyzer 0.20.0 also by the `cursor` parser, for a conversation whose header names an agent location other than `local`, and since analyzer 0.20.1 for a Cursor cloud agent, `session origin: remote cursor cloud agent`, by the `cline` parser, for a Cline Cloud session, and by the `agent-zero` parser, for the chat the A0 CLI connector last used on an Agent Zero instance; since analyzer 0.21.0 by the `claude-code` parser, for a session started from claude.ai through Remote Control or a cloud session brought local with `claude --teleport`); `local` when the agent's parser records origin and no record carries that row; `""` when the agent's parser records no origin, so nothing is known. A remote `vscode`, `chatgpt-desktop` or `cursor` session is also `synced` `yes`, since the vendor holds its copy; since analyzer 0.20.1 every other Cursor session's `synced` is `no`. Since analyzer 0.15.0. |
| `recovered` | string | Whether the session was rebuilt from secondary sources (see [Secondary sources](#secondary-sources)): `yes` when every one of its records in `timeline.jsonl` has `recovered` `yes`, so its primary transcript is gone and only secondary sources hold it; `partial` when some do, so the primary was cut or rewound and a secondary source kept turns it lost; `no` when none does and the agent's parser reads at least one secondary source; `""` when the agent's parser reads no secondary source, so nothing is known. Computed after the filters, over the session's event records only (`system` rows left out), so a secondary source's header, record or `parser:` row never makes a complete session `partial`, and a session with no event record reads `no` or `""`. Because deduplication matches on text, `partial` also results when a secondary source records an event in another form than the primary store does (an index that compresses a tool call into one line), so it marks a session to compare against its secondary sources, not proof of a cut; and a Letta session on the server backend, whose local transcript mirrors the server's store of record, reads `yes` or `partial` by design. Since analyzer 0.15.0. |

This is the `sessions.jsonl` that `timeline -o out` writes for
[the example collection](#the-example-collection), the run shown under
[timeline](#timeline): one record per session of every parsed agent.
The Remote Control spawn and the teleported session read `synced` `yes`
and `origin` `remote`; the session bound to Remote Control and the one
whose process the registry file names read `synced` `yes` but `origin`
`local`, since they began on this host; the sessions whose every turn
comes from a secondary source, among them the one known only from
alice's prompt history, read `recovered` `yes`, and those some of whose
turns do read `partial`. The settings rows have no `session_id`, so they belong to no
session:

<!-- generated by analyzer/tools/regenerate_examples.py: sessions-jsonl -->
```json
{"host":"wsdev14","user":"alice","agent":"claude-code","session_id":"99999999-0000-4000-8000-000000000000","project_path":"/srv/old","first_timestamp_utc":"2026-09-30T10:00:00.000Z","last_timestamp_utc":"2026-09-30T10:00:00.000Z","models":[],"user_turns":1,"assistant_turns":0,"tool_calls":0,"source_file":"/home/alice/.claude/history.jsonl","synced":"no","relayed":"","origin":"local","recovered":"yes"}
{"host":"wsdev14","user":"alice","agent":"cursor","session_id":"c0000003-0000-4000-8000-000000000003","project_path":"/srv/proj","first_timestamp_utc":"2026-10-01T01:30:00.000Z","last_timestamp_utc":"2026-10-01T13:00:00.000Z","models":["grok-4.6"],"user_turns":2,"assistant_turns":1,"tool_calls":0,"source_file":"/home/alice/.cursor/projects/srv-proj/agent-transcripts/c0000003-0000-4000-8000-000000000003/c0000003-0000-4000-8000-000000000003.jsonl","synced":"no","relayed":"","origin":"local","recovered":"yes"}
{"host":"wsdev14","user":"alice","agent":"codex-cli","session_id":"aaaaaaaa-bbbb-4ccc-8ddd-000000000006","project_path":"/srv/proj","first_timestamp_utc":"2026-10-01T08:00:00.000Z","last_timestamp_utc":"2026-10-01T08:00:10.000Z","models":[],"user_turns":1,"assistant_turns":1,"tool_calls":1,"source_file":"/home/alice/.codex/thread_history_1.sqlite","synced":"no","relayed":"","origin":"","recovered":"yes"}
{"host":"wsdev14","user":"bob","agent":"gemini-cli","session_id":"a1b2c3d4-0000-4000-8000-000000000001","project_path":"/srv/proj","first_timestamp_utc":"2026-10-01T09:00:00.000Z","last_timestamp_utc":"2026-10-01T09:00:24.000Z","models":["gemini-2.5-pro","gemini-2.5-flash"],"user_turns":3,"assistant_turns":2,"tool_calls":3,"source_file":"/home/bob/.gemini/tmp/proj/chats/session-2026-10-01T09-00-a1b2c3d4.jsonl","synced":"","relayed":"","origin":"","recovered":"partial"}
{"host":"wsdev14","user":"bob","agent":"gemini-cli","session_id":"c3d4e5f6-0000-4000-8000-000000000003","project_path":"/srv/proj","first_timestamp_utc":"2026-10-01T09:00:30.000Z","last_timestamp_utc":"2026-10-01T09:00:30.000Z","models":[],"user_turns":1,"assistant_turns":0,"tool_calls":0,"source_file":"/home/bob/.gemini/tmp/proj/chats/a1b2c3d4-0000-4000-8000-000000000001/c3d4e5f6-0000-4000-8000-000000000003.jsonl","synced":"","relayed":"","origin":"","recovered":"no"}
{"host":"wsdev14","user":"bob","agent":"gemini-cli","session_id":"a1b2c3d4-0000-4000-8000-0000000000ff","project_path":"/srv/proj","first_timestamp_utc":"2026-10-01T09:01:00.000Z","last_timestamp_utc":"2026-10-01T09:01:00.000Z","models":[],"user_turns":1,"assistant_turns":0,"tool_calls":0,"source_file":"/home/bob/.gemini/tmp/proj/chats/session-2026-10-01T09-00-a1b2c3d4.jsonl","synced":"","relayed":"","origin":"","recovered":"no"}
{"host":"wsdev14","user":"alice","agent":"cursor","session_id":"c0000001-0000-4000-8000-000000000001","project_path":"/srv/proj","first_timestamp_utc":"2026-10-01T09:59:00.000Z","last_timestamp_utc":"2026-10-01T10:05:00.000Z","models":["grok-4.6","grok-4.7"],"user_turns":2,"assistant_turns":1,"tool_calls":5,"source_file":"/home/alice/.config/Cursor/User/globalStorage/state.vscdb","synced":"no","relayed":"","origin":"local","recovered":"partial"}
{"host":"wsdev14","user":"alice","agent":"claude-code","session_id":"11111111-2222-4333-8444-555555555555","project_path":"/srv/proj","first_timestamp_utc":"2026-10-01T10:00:00.000Z","last_timestamp_utc":"2026-10-01T10:00:20.000Z","models":["claude-fable-5-1"],"user_turns":4,"assistant_turns":2,"tool_calls":5,"source_file":"/home/alice/.claude/projects/-srv-proj/11111111-2222-4333-8444-555555555555.jsonl","synced":"no","relayed":"","origin":"local","recovered":"no"}
{"host":"wsdev14","user":"alice","agent":"claude-code","session_id":"11111111-2222-4333-8444-666666666666","project_path":"/srv/proj","first_timestamp_utc":"2026-10-01T10:00:30.000Z","last_timestamp_utc":"2026-10-01T10:00:31.000Z","models":["claude-fable-5-1"],"user_turns":1,"assistant_turns":1,"tool_calls":0,"source_file":"/home/alice/.claude-work/projects/-srv-proj/11111111-2222-4333-8444-666666666666.jsonl","synced":"no","relayed":"","origin":"local","recovered":"no"}
{"host":"wsdev14","user":"alice","agent":"cursor","session_id":"c0000002-0000-4000-8000-000000000002","project_path":"/srv/proj","first_timestamp_utc":"2026-10-01T10:20:00.000Z","last_timestamp_utc":"2026-10-01T12:00:00.000Z","models":["grok-4.7"],"user_turns":2,"assistant_turns":2,"tool_calls":0,"source_file":"/home/alice/.config/Cursor/User/globalStorage/state.vscdb","synced":"no","relayed":"","origin":"local","recovered":"no"}
{"host":"wsdev14","user":"alice","agent":"claude-code","session_id":"11111111-2222-4333-8444-777777777777","project_path":"/srv/proj","first_timestamp_utc":"2026-10-01T11:00:00.000Z","last_timestamp_utc":"2026-10-01T11:00:04.000Z","models":["claude-fable-5-1"],"user_turns":1,"assistant_turns":0,"tool_calls":1,"source_file":"/home/alice/.claude/projects/-srv-proj/11111111-2222-4333-8444-777777777777.jsonl","synced":"no","relayed":"","origin":"local","recovered":"no"}
{"host":"wsdev14","user":"alice","agent":"claude-code","session_id":"11111111-2222-4333-8444-888888888888","project_path":"/srv/proj","first_timestamp_utc":"2026-10-01T12:00:00.000Z","last_timestamp_utc":"2026-10-15T15:56:10.000Z","models":[],"user_turns":0,"assistant_turns":0,"tool_calls":0,"source_file":"/home/alice/.claude/sessions/4343.json","synced":"yes","relayed":"","origin":"local","recovered":"no"}
{"host":"wsdev14","user":"alice","agent":"claude-code","session_id":"11111111-2222-5333-8444-000000000001","project_path":"/srv/proj","first_timestamp_utc":"2026-10-01T12:00:00.000Z","last_timestamp_utc":"2026-10-15T15:56:10.000Z","models":["claude-fable-5-1"],"user_turns":1,"assistant_turns":0,"tool_calls":1,"source_file":"/home/alice/.claude/projects/-srv-proj/11111111-2222-5333-8444-000000000001.jsonl","synced":"yes","relayed":"","origin":"remote","recovered":"no"}
{"host":"wsdev14","user":"alice","agent":"claude-code","session_id":"11111111-2222-4333-8444-999999999999","project_path":"/srv/proj","first_timestamp_utc":"2026-10-01T12:00:01.000Z","last_timestamp_utc":"2026-10-01T12:00:10.000Z","models":["claude-fable-5-1","claude-opus-5-5"],"user_turns":3,"assistant_turns":3,"tool_calls":0,"source_file":"/home/alice/.claude/projects/-srv-proj/11111111-2222-4333-8444-999999999999.jsonl","synced":"no","relayed":"","origin":"local","recovered":"no"}
{"host":"wsdev14","user":"alice","agent":"cursor","session_id":"d0000001-0000-4000-8000-000000000001","project_path":"","first_timestamp_utc":"2026-10-01T12:30:00.000Z","last_timestamp_utc":"2026-10-01T12:30:00.000Z","models":[],"user_turns":0,"assistant_turns":0,"tool_calls":0,"source_file":"/home/alice/.config/Cursor/User/globalStorage/state.vscdb","synced":"no","relayed":"","origin":"local","recovered":"no"}
{"host":"wsdev14","user":"alice","agent":"cursor","session_id":"bc-e0000001-0000-4000-8000-000000000001","project_path":"github.com/alice-gh/demo","first_timestamp_utc":"2026-10-01T12:46:40.000Z","last_timestamp_utc":"2026-10-01T14:00:00.000Z","models":["grok-4.7"],"user_turns":0,"assistant_turns":0,"tool_calls":0,"source_file":"/home/alice/.config/Cursor/User/globalStorage/state.vscdb","synced":"yes","relayed":"","origin":"remote","recovered":"no"}
{"host":"wsdev14","user":"alice","agent":"claude-code","session_id":"11111111-2222-4333-8444-000000000002","project_path":"/srv/proj","first_timestamp_utc":"2026-10-01T13:00:00.000Z","last_timestamp_utc":"2026-10-01T13:00:11.000Z","models":["claude-opus-5-5"],"user_turns":2,"assistant_turns":0,"tool_calls":1,"source_file":"/home/alice/.claude/projects/-srv-proj/11111111-2222-4333-8444-000000000002.jsonl","synced":"yes","relayed":"","origin":"remote","recovered":"no"}
{"host":"wsdev14","user":"alice","agent":"claude-code","session_id":"11111111-2222-4333-8444-aaaaaaaaaaaa","project_path":"/srv/proj","first_timestamp_utc":"2026-10-01T13:00:01.000Z","last_timestamp_utc":"2026-10-01T13:00:11.000Z","models":["claude-sonnet-5-5"],"user_turns":4,"assistant_turns":0,"tool_calls":2,"source_file":"/home/alice/.claude/projects/-srv-proj/11111111-2222-4333-8444-aaaaaaaaaaaa.jsonl","synced":"yes","relayed":"","origin":"local","recovered":"no"}
{"host":"wsdev14","user":"alice","agent":"cursor","session_id":"bc-e0000002-0000-4000-8000-000000000002","project_path":"github.com/alice-gh/web","first_timestamp_utc":"2026-10-01T13:03:20.000Z","last_timestamp_utc":"2026-10-01T13:05:20.000Z","models":["grok-4.7"],"user_turns":0,"assistant_turns":0,"tool_calls":0,"source_file":"/home/alice/.config/Cursor/User/globalStorage/state.vscdb","synced":"yes","relayed":"","origin":"remote","recovered":"no"}
{"host":"wsdev14","user":"alice","agent":"codex-cli","session_id":"aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee","project_path":"/srv/proj","first_timestamp_utc":"2026-10-02T09:00:00.000Z","last_timestamp_utc":"2026-10-14T15:25:52.000Z","models":["gpt-5-codex","gpt-5.5-codex"],"user_turns":1,"assistant_turns":1,"tool_calls":3,"source_file":"/home/alice/.codex/sessions/2026/10/02/rollout-2026-10-02T09-00-00-aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee.jsonl","synced":"yes","relayed":"","origin":"","recovered":"no"}
{"host":"wsdev14","user":"alice","agent":"aider","session_id":"/home/alice/.aider.chat.history.md#2026-10-02 12:00:00","project_path":"/home/alice","first_timestamp_utc":"2026-10-02T12:00:00.000Z","last_timestamp_utc":"2026-10-02T12:00:09.000Z","models":["anthropic/claude-sonnet-4-5","anthropic/claude-haiku-4-5"],"user_turns":1,"assistant_turns":2,"tool_calls":0,"source_file":"/home/alice/.aider.chat.history.md","synced":"","relayed":"","origin":"","recovered":"partial"}
{"host":"wsdev14","user":"alice","agent":"aider","session_id":"/home/alice/.aider.chat.history.md#2026-10-02 13:00:00","project_path":"/home/alice","first_timestamp_utc":"2026-10-02T13:00:00.000Z","last_timestamp_utc":"2026-10-02T13:05:00.000Z","models":["gpt-4o","claude-opus-4-5","claude-sonnet-4-5","claude-haiku-4-5"],"user_turns":4,"assistant_turns":0,"tool_calls":0,"source_file":"/home/alice/.aider.chat.history.md","synced":"","relayed":"","origin":"","recovered":"partial"}
```
<!-- end generated: sessions-jsonl -->

## Compatibility

The two JSONL layouts are a compatibility contract with the analysts and
tooling that consume them. A future version may append new fields after the
existing ones, but existing fields keep their names, order, type and
meaning.
