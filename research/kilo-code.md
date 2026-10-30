# kilo-code

Kilo Code (catalog agent `kilo-code`) is an open-source coding agent with four surfaces: a CLI,
terminal UI (TUI) and local server that are an OpenCode fork, so the SQLite store and record shapes
of [`opencode.md`](opencode.md) apply and this document states only what Kilo adds or changes; a VS
Code extension and a JetBrains plugin, both thin clients that start that server and share its
database. The extension began as a fork of Roo Code ([`roo-code.md`](roo-code.md)), itself a fork
of Cline, and still reads the Roo-style task directories that pre-fork installs left, for import
([`packages/kilo-vscode/src/legacy-migration/task-store.ts:6-7`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-vscode/src/legacy-migration/task-store.ts#L6-L7)).
"Kilo Cloud" below is Kilo's web service at `app.kilo.ai`. This document rests on Kilo Code 7.8.25
at commit
[`0a9faef`](https://github.com/Kilo-Org/kilocode/commit/0a9faeff69fff8712da184ee5188bb7985c6f057).
The analyzer parses its `kilo.db` sessions and the cloud session marker, from source and fixtures
only.

## Source and evidence

Open source, MIT. [Kilo-Org/kilocode](https://github.com/Kilo-Org/kilocode) at
[`0a9faef`](https://github.com/Kilo-Org/kilocode/commit/0a9faeff69fff8712da184ee5188bb7985c6f057),
committed 2026-10-29, whose `package.json` files give version 7.8.25
([`packages/opencode/package.json:3`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/package.json#L3)); the tag `v7.8.25`
points at the earlier commit `b3b2f3d`, so this commit is a later, unreleased state of 7.8.25. The
CLI, TUI and server are in `packages/core`, `packages/opencode` and `packages/tui`; the VS Code
extension `kilocode.kilo-code`
([`packages/kilo-vscode/package.json:2`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-vscode/package.json#L2),
[`:11`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-vscode/package.json#L11)) is in `packages/kilo-vscode`, the JetBrains
plugin in `packages/kilo-jetbrains`, and project memory in `packages/kilo-memory`. Every claim is
from source at that commit unless it names an older commit (in [Format versions](#format-versions))
or the vendor's documentation, which the repository holds under `packages/kilo-docs/pages` and
which is linked at its claim.

### Observed

No real install. The source review of
[`0a9faef`](https://github.com/Kilo-Org/kilocode/commit/0a9faeff69fff8712da184ee5188bb7985c6f057)
(7.8.25) stands in; every row of the [Parser plan](#parser-plan) is source-derived and its
samples are fixture only.

### Source reviews

- 2026-10-30: Kilo-Org/kilocode at
  [`0a9faef`](https://github.com/Kilo-Org/kilocode/commit/0a9faeff69fff8712da184ee5188bb7985c6f057)
  (7.8.25). Re-cited every claim; added the session turn markers, the project memory store, the
  daemon state file, the JetBrains plugin, the agent manager state file, the shared agent board
  tables, the legacy extension MCP settings, the hosted sessions and the permission service the CLI
  uses; corrected the claim that Kilo's TUI keeps no `session.json`.
- 2026-10-21: Kilo-Org/kilocode at
  [87cc35c](https://github.com/Kilo-Org/kilocode/commit/87cc35ce6904465a0eed507a8c75c79185863a22):
  the parser plan's permission, past-use, remote control, scratch directory and deletion items are
  now read from source alone, the sandbox directories got catalog lines, the TUI prompt stash and
  the session pull request links were added, and the source-host integration was reviewed
  ([Source host rows](#source-host-rows)).
- 2026-10-19:
  [`87cc35c`](https://github.com/Kilo-Org/kilocode/commit/87cc35ce6904465a0eed507a8c75c79185863a22)
  against
  [`76bcfd4`](https://github.com/Kilo-Org/kilocode/commit/76bcfd40be616a72f4697b3041565f322245b462).
  Added the sandbox policy and preference files, the TUI prompt history, the retention pass,
  remote control, cloud session import, the permission records and the project worktrees.

## Collection

### Per-user storage

The roots come from `xdg-basedir` with no platform branching
([`packages/core/src/global.ts:3`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/global.ts#L3),
[`:12`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/global.ts#L12),
[`:22-26`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/global.ts#L22-L26)), so the paths are the same on Linux, macOS
and Windows (`~` is `%USERPROFILE%` on Windows); `XDG_DATA_HOME`, `XDG_CONFIG_HOME`,
`XDG_CACHE_HOME` and `XDG_STATE_HOME` relocate them. `KILO_TEST_HOME` overrides home
([`global.ts:32`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/global.ts#L32)) and newlines are stripped from `$HOME`
([`global.ts:21`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/global.ts#L21)). The data, cache and state roots are tagged
no-index for Spotlight ([`global.ts:58`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/global.ts#L58)). The vendor's
[Find the local database](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-docs/pages/code-with-ai/agents/session-history.md#L137-L154)
gives the same `~/.local/share/kilo/kilo.db` on every platform.

#### Scratch directories

- `~/.local/share/kilo/worktree/<project id>/`, the agent's worktrees
  ([`packages/opencode/src/worktree/index.ts:210`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/worktree/index.ts#L210)),
  under the data root of [`~/.local/share/kilo`](#localsharekilo).
- `<project>/.kilo/worktrees/<name>`, the worktrees of the extension's agent manager and of the TUI
  ([`packages/opencode/src/kilocode/cli/cmd/tui-worktree.ts:14-17`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/cli/cmd/tui-worktree.ts#L14-L17);
  [Project-local files](#project-local-files)), the agent's own worktree inside the user's project
  and under no home entry. [Where the project path is recorded](#where-the-project-path-is-recorded)
  says which `kilo.db` rows name them.
- `<system temp directory>/kilo/`, the shared temporary root
  ([`packages/core/src/global.ts:28`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/global.ts#L28)): the shell tool tells the
  model to use it, and the sandbox sets `TMPDIR`, `TMP` and `TEMP` to it for every command it
  confines
  ([`packages/opencode/src/kilocode/tool/shell-tmp.ts:17-30`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/tool/shell-tmp.ts#L17-L30);
  [`packages/opencode/src/kilocode/sandbox/policy.ts:256-276`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/sandbox/policy.ts#L256-L276)).
  Its `remote-attachments/<base64url session id>/` holds files sent from Kilo Cloud over remote
  control, removed when the session's remote handling ends
  ([`packages/opencode/src/kilocode/remote-attachments.ts:272-287`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/remote-attachments.ts#L272-L287);
  [Remote control](#remote-control)). In a Kilo Cloud agent session the shell's temporary
  directory is `/tmp/<SESSION_ID>/` instead. These roots are outside the home, so no home entry
  reaches them; collecting them is a manual step.

#### `~/.local/share/kilo`

`~/.local/share/kilo` (data):

- `kilo.db` (SQLite, WAL journal) on release builds and whenever `KILO_DISABLE_CHANNEL_DB` is set,
  which the VS Code extension and the JetBrains plugin set; `kilo-<channel>.db`, or a pre-existing
  `opencode-<channel>.db`, on other channels; `KILO_DB` overrides
  ([`packages/core/src/database/database.ts:32`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/database/database.ts#L32),
  [`:50-66`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/database/database.ts#L50-L66);
  [`packages/core/src/flag/flag.ts:111`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/flag/flag.ts#L111)). Holds sessions,
  messages, parts and projects ([`kilo.db`](#kilodb)).
- `auth.json` ([`packages/opencode/src/auth/index.ts:11`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/auth/index.ts#L11))
  and `mcp-auth.json`
  ([`packages/opencode/src/mcp/auth.ts:40`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/mcp/auth.ts#L40)), the
  credential stores of [Credentials](#credentials).
- `storage/`, the JSON store
  ([`packages/opencode/src/storage/storage.ts:234`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/storage/storage.ts#L234);
  [`packages/opencode/src/kilocode/storage/json-migration.ts:78`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/storage/json-migration.ts#L78)):
  legacy transcripts ([Transcript stores](#transcript-stores)), the cloud session markers
  `storage/session_share/<sessionID>.json` ([Cloud session marker](#cloud-session-marker)) and the
  pull request links `storage/session_pr_link_session/<sessionID>.json`
  ([Source host rows](#source-host-rows)).
- `snapshot/<projectID>/<hash>`, bare git directories for checkpoints
  ([`packages/opencode/src/snapshot/index.ts:108`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/snapshot/index.ts#L108)),
  `worktree/<projectID>/` ([Scratch directories](#scratch-directories)), `tool-output/`, truncated
  tool results
  ([`packages/opencode/src/tool/truncation-dir.ts:4`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/tool/truncation-dir.ts#L4)),
  `repos/` ([`global.ts:37`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/global.ts#L37)), `log/`
  ([`global.ts:36`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/global.ts#L36); [Logs](#logs)), `plans/*.md`
  ([`packages/opencode/src/session/session.ts:416`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/session/session.ts#L416)).
- `retention/state.json`, the counts of the last automatic deletion pass
  ([`packages/opencode/src/kilocode/session/retention.ts:326`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/session/retention.ts#L326);
  [Retention](#retention)).
- `session-export.db` and `session-export-workspace.json`, the store of a switched-off uploader
  ([`packages/opencode/src/kilocode/bootstrap.ts:85-90`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/bootstrap.ts#L85-L90);
  [Sync, relay and origin rows](#sync-relay-and-origin-rows)).
- `memory/<project name>-<first 12 hex of the SHA-1 of the project root>/`, project memory, a
  per-repository store of notes and session digests the user turns on
  ([`packages/kilo-memory/src/storage/paths.ts:90-103`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-memory/src/storage/paths.ts#L90-L103);
  [`packages/opencode/src/kilocode/memory/runtime.ts:19`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/memory/runtime.ts#L19);
  the vendor's [What memory stores](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-docs/pages/customize/context/memory.md#L18-L29)):
  `state.json`, `manifest.json`, `index.kmem`, `project.md`, `environment.md`, `corrections.md`,
  `.gitignore` and `sessions/<ISO time>_<session id>_<hash>.md`
  ([`paths.ts:105-118`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-memory/src/storage/paths.ts#L105-L118);
  [`packages/kilo-memory/src/storage/sessions.ts:73-78`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-memory/src/storage/sessions.ts#L73-L78));
  records in [Project memory](#project-memory).
- `state/`, the state directory itself when `XDG_STATE_HOME` is unset and `~/.local/state/kilo` is
  unusable ([`global.ts:26`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/global.ts#L26);
  [`packages/core/src/kilocode/global.ts:38-72`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/kilocode/global.ts#L38-L72)).

#### `~/.local/state/kilo`

`~/.local/state/kilo` (state):

- `indexing/`, the codebase index
  ([`packages/opencode/src/kilocode/indexing.ts:298`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/indexing.ts#L298)).
- `model.json`
  ([`packages/opencode/src/kilocode/config/model-state.ts:51`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/config/model-state.ts#L51)),
  `plugin-meta.json`
  ([`packages/opencode/src/plugin/meta.ts:49`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/plugin/meta.ts#L49)),
  `background-process/`
  ([`packages/opencode/src/kilocode/background-process/index.ts:212`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/background-process/index.ts#L212)).
- `prompt-history.jsonl`, the last 50 prompts typed into the TUI, shell commands included
  ([`packages/tui/src/prompt/history.tsx:27`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/tui/src/prompt/history.tsx#L27),
  [`:53`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/tui/src/prompt/history.tsx#L53)), and `prompt-stash.jsonl`, up to 50 prompts
  set aside unsent
  ([`packages/tui/src/prompt/stash.tsx:15`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/tui/src/prompt/stash.tsx#L15),
  [`:36`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/tui/src/prompt/stash.tsx#L36)); records in
  [TUI prompt history](#tui-prompt-history).
- `session.json`, the TUI's pinned sessions
  ([`packages/tui/src/context/local.tsx:489`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/tui/src/context/local.tsx#L489)),
  `frecency.jsonl`, the files attached to prompts
  ([`packages/tui/src/prompt/frecency.tsx:42`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/tui/src/prompt/frecency.tsx#L42)), and
  `kv.json`, TUI settings ([`packages/tui/src/context/kv.tsx:15`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/tui/src/context/kv.tsx#L15)),
  the files and record shapes of OpenCode's TUI
  ([Other records of use in `opencode.md`](opencode.md#other-records-of-use)).
- `session-turn/<session id>.<pid>`, one marker per session whose turn is running, holding the host
  name ([`packages/opencode/src/kilocode/session/turn.ts:29-35`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/session/turn.ts#L29-L35),
  [`:54-61`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/session/turn.ts#L54-L61); records in
  [Session turn markers](#session-turn-markers)).
- `daemon.json`, mode 0600, the registration of a background `kilo serve` started by
  `kilo daemon` or `kilo console`
  ([`packages/opencode/src/kilocode/daemon/daemon.ts:97-99`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/daemon/daemon.ts#L97-L99),
  [`:123-125`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/daemon/daemon.ts#L123-L125); the vendor's
  [Daemon lifecycle](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-docs/pages/contributing/architecture/cli-runtime.md#L182-L192)),
  a credential file ([Credentials](#credentials)); `log/daemon.log` in the data root is its log.

Beside it, in the parent of the state directory:

- `~/.local/state/kilo-sandbox-policy/<sha256 of session id>/<sha256 of directory>.json`, one per
  session and directory, mode 0600: the confinement Kilo's sandbox applied to that session's
  commands
  ([`packages/opencode/src/kilocode/sandbox/store.ts:19-31`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/sandbox/store.ts#L19-L31),
  [`:70-80`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/sandbox/store.ts#L70-L80); records in
  [Sandbox files](#sandbox-files)).
- `~/.local/state/kilo-sandbox-preference/<sha256 of directory>.json`, whether the sandbox is on
  for a directory
  ([`packages/opencode/src/kilocode/sandbox/preference.ts:7-34`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/sandbox/preference.ts#L7-L34)).

On the default layout both are siblings of `~/.local/state/kilo`, so the entries
`.local/state/kilo-sandbox-policy` and `.local/state/kilo-sandbox-preference` reach them; with
the fallback state directory `<data>/state` they land in `~/.local/share/kilo` and are collected
with it. The names are hashes, so a file is tied to a session or project only by hashing a known
id or path.

#### `~/.config/kilo`

`~/.config/kilo` (config; `KILO_CONFIG_DIR` adds a directory,
[`packages/core/src/flag/flag.ts:140-141`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/flag/flag.ts#L140-L141)):
`kilo.jsonc`, `kilo.json`, `opencode.jsonc`, `opencode.json`, `config.json` and the legacy
`config/` directory
([`packages/opencode/src/config/config.ts:206-215`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/config/config.ts#L206-L215),
[`:462-492`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/config/config.ts#L462-L492)), `agents/`
([`packages/opencode/src/kilocode/marketplace/paths.ts:76`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/marketplace/paths.ts#L76)),
`tui.jsonc`, `themes/`, `marketplace/`. The global config files carry the standing permission
grants ([Permission rows](#permission-rows)) and the switches for retention, remote control and
the shared agent board
([`packages/core/src/v1/config/config.ts:84-98`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/v1/config/config.ts#L84-L98),
[`:121-123`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/v1/config/config.ts#L121-L123),
[`:131-133`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/v1/config/config.ts#L131-L133)). `KILO_CONFIG` points at an
extra config file and `KILO_CONFIG_CONTENT` inlines one
([`config.ts:766-781`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/config/config.ts#L766-L781),
[`:901-907`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/config/config.ts#L901-L907)).

#### `~/.cache/kilo`

`~/.cache/kilo`: `bin/` ([`global.ts:35`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/global.ts#L35), language servers
such as clangd and zls), `skills/`, `packages/git/`, `commands/`
([`packages/opencode/src/kilocode/skill-remove.ts:25`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/skill-remove.ts#L25);
[`packages/opencode/src/kilocode/plugin/git-source.ts:158`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/plugin/git-source.ts#L158);
[`packages/opencode/src/kilocode/command-files.ts:118`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/command-files.ts#L118)).

#### `~/.kilo` and `~/.kilocode`

`~/.kilo`: `bin/kilo`, the installer's target ([`install:70`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/install#L70)), `skills/`
([`marketplace/paths.ts:81`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/marketplace/paths.ts#L81)) and
`rules/`
([`packages/opencode/src/kilocode/rules-migrator.ts:12`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/rules-migrator.ts#L12));
it and `~/.kilocode` are extra config roots
([`packages/opencode/src/kilocode/config/overlay.ts:279`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/config/overlay.ts#L279)).
`~/.kilocode` holds the legacy `rules/`, `cli/config.json` and
`cli/global/settings/custom_modes.yaml`
([`packages/opencode/src/kilocode/modes-migrator.ts:170`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/modes-migrator.ts#L170));
`~/.kilocodemodes` is read for migration
([`modes-migrator.ts:174`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/modes-migrator.ts#L174)). `~/.agents`
is read for skills
([`packages/opencode/src/kilocode/config/claude-migration.ts:641`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/config/claude-migration.ts#L641)).

#### VS Code extension

The extension starts the CLI server with `KILO_CLIENT=vscode` and `KILO_DISABLE_CHANNEL_DB=true`,
so its sessions are in the same `kilo.db`
([`packages/kilo-vscode/src/services/cli-backend/server-manager.ts:32-38`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-vscode/src/services/cli-backend/server-manager.ts#L32-L38),
[`:153-160`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-vscode/src/services/cli-backend/server-manager.ts#L153-L160)).
Its VS Code `globalStorage/kilocode.kilo-code/` is
`Library/Application Support/Code/User/globalStorage/kilocode.kilo-code` on macOS,
`AppData/Roaming/Code/User/globalStorage/kilocode.kilo-code` on Windows and
`.config/Code/User/globalStorage/kilocode.kilo-code` on Linux
([`packages/opencode/src/kilocode/paths.ts:14-30`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/paths.ts#L14-L30)).
The current extension keeps its own state in VS Code's `globalState`
([`packages/kilo-vscode/src/KiloProvider.ts:1717-1728`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-vscode/src/KiloProvider.ts#L1717-L1728))
and reads there what pre-fork installs left: `tasks/` with `tasks/_index.json`
([`packages/kilo-vscode/src/legacy-migration/task-store.ts:136`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-vscode/src/legacy-migration/task-store.ts#L136)),
`settings/mcp_settings.json`
([`packages/opencode/src/kilocode/mcp-migrator.ts:100`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/mcp-migrator.ts#L100);
[`packages/kilo-vscode/src/services/marketplace/actions.ts:145`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-vscode/src/services/marketplace/actions.ts#L145)),
`settings/custom_modes.yaml`
([`modes-migrator.ts:166`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/modes-migrator.ts#L166)) and
`skills/`; pre-fork installs also leave `checkpoints/` and `cache/` there. On a remote host (WSL,
SSH or a tunnel) the extension runs in VS Code's remote server, whose user data is
`~/.vscode-server/data/User`
([Remote server in `vscode.md`](vscode.md#remote-server)), so its folder there is
`.vscode-server/data/User/globalStorage/kilocode.kilo-code`. The extension also reads
Roo Code's task directories under the sibling `globalStorage/rooveterinaryinc.roo-cline`,
`rooveterinaryinc.roo-code`, `roovscode.roo-cline` and `roovscode.roo-code` to import them
([`packages/kilo-vscode/src/roo-import/service.ts:12-17`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-vscode/src/roo-import/service.ts#L12-L17),
[`:31-37`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-vscode/src/roo-import/service.ts#L31-L37)).

#### JetBrains plugin

The plugin starts the same CLI with `KILO_CLIENT=jetbrains`, and with `KILO_DISABLE_CHANNEL_DB=true`
on release builds, so its sessions share `kilo.db`
([`packages/kilo-jetbrains/backend/src/main/kotlin/ai/kilocode/backend/cli/KiloBackendCliManager.kt:593-620`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-jetbrains/backend/src/main/kotlin/ai/kilocode/backend/cli/KiloBackendCliManager.kt#L593-L620)).
Its own files sit in the IDE's system directory, `Library/Caches/JetBrains/<product><version>`
on macOS, `AppData/Local/JetBrains/<product><version>` on Windows and
`.cache/JetBrains/<product><version>` on Linux
([Directories used by the IDE, system directory](https://www.jetbrains.com/help/idea/directories-used-by-the-ide-to-store-settings-caches-plugins-and-logs.html#system-directory)),
under `kilo/`: `cli/`, the downloaded CLI binaries
([`KiloCliDownloader.kt:35`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-jetbrains/backend/src/main/kotlin/ai/kilocode/backend/cli/KiloCliDownloader.kt#L35)),
`repo-cli/<version>/`, a CLI the plugin bundles
([`KiloRepoCli.kt:21`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-jetbrains/backend/src/main/kotlin/ai/kilocode/backend/cli/KiloRepoCli.kt#L21)),
`cwd/`, an empty working directory for the server
([`KiloBackendCliManager.kt:627-637`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-jetbrains/backend/src/main/kotlin/ai/kilocode/backend/cli/KiloBackendCliManager.kt#L627-L637)),
and `machine-id`, a random UUID sent to the CLI as `KILO_MACHINE_ID`
([`:658-665`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-jetbrains/backend/src/main/kotlin/ai/kilocode/backend/cli/KiloBackendCliManager.kt#L658-L665)).
Its settings are the application settings files `kiloLogSettings.xml` and
`kiloAutocompleteSettings.xml` in the `options/` folder of the IDE's configuration directory,
`Library/Application Support/JetBrains/<product><version>/options/kiloLogSettings.xml` on macOS,
`AppData/Roaming/JetBrains/<product><version>/options/kiloLogSettings.xml` on Windows and
`.config/JetBrains/<product><version>/options/kiloLogSettings.xml` on Linux
([Directories used by the IDE, configuration directory](https://www.jetbrains.com/help/idea/directories-used-by-the-ide-to-store-settings-caches-plugins-and-logs.html#config-directory);
[`KiloLogSettingsService.kt:13-16`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-jetbrains/frontend/src/main/kotlin/ai/kilocode/client/settings/KiloLogSettingsService.kt#L13-L16),
[`KiloAutocompleteSettingsService.kt:11-14`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-jetbrains/frontend/src/main/kotlin/ai/kilocode/client/autocomplete/KiloAutocompleteSettingsService.kt#L11-L14)):
log level and display preferences and autocomplete switches.

### Credentials

- `~/.local/share/kilo/auth.json`, mode 0600, the provider keys and tokens, the Kilo account's
  among them ([`packages/opencode/src/auth/index.ts:15-37`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/auth/index.ts#L15-L37),
  [`:81-90`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/auth/index.ts#L81-L90)); fields in [Secrets](#secrets).
- `~/.local/share/kilo/mcp-auth.json`, MCP OAuth tokens
  ([`packages/opencode/src/mcp/auth.ts:40`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/mcp/auth.ts#L40)).
- `~/.local/state/kilo/daemon.json`, the local server's password and token
  ([`packages/opencode/src/kilocode/daemon/daemon.ts:30-46`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/daemon/daemon.ts#L30-L46)).
- `~/.kilocode/cli/config.json`, the legacy Kilo token, migrated into `auth.json`
  ([`packages/kilo-gateway/src/auth/legacy-migration.ts:5-11`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-gateway/src/auth/legacy-migration.ts#L5-L11)).
- `~/.config/kilo/kilo.jsonc`, `~/.config/kilo/kilo.json`, `~/.config/kilo/opencode.jsonc`,
  `~/.config/kilo/opencode.json` and `~/.config/kilo/config.json`, settings files that hold MCP
  server `env` and `headers` values and provider keys.
- The legacy extension MCP settings,
  `Library/Application Support/Code/User/globalStorage/kilocode.kilo-code/settings/mcp_settings.json`
  on macOS, `AppData/Roaming/Code/User/globalStorage/kilocode.kilo-code/settings/mcp_settings.json`
  on Windows and `.config/Code/User/globalStorage/kilocode.kilo-code/settings/mcp_settings.json`
  on Linux, which hold MCP server `env` values.
- `kilo.db`, and the channel databases `kilo-<channel>.db` and `opencode-<channel>.db`, mix
  credentials with the transcripts, so they are flagged as credential files; the columns are named
  in [Secrets](#secrets).
- No keychain use. `KILO_SERVER_PASSWORD` is passed in the environment only
  ([`packages/core/src/flag/flag.ts:68`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/flag/flag.ts#L68)).
- The JetBrains `machine-id` is an install identifier, not a credential: it authenticates nothing.

### Exclusions

`.local/share/kilo/snapshot`, `.local/share/kilo/worktree`, `.local/share/kilo/repos`,
`.local/share/kilo/tool-output`, `.local/state/kilo/indexing`, `.local/share/kilo/state/indexing`,
`.kilo/bin`, the project `.kilo/worktrees`, and the
JetBrains plugin's downloaded binaries `Library/Caches/JetBrains/*/kilo/cli`,
`Library/Caches/JetBrains/*/kilo/repo-cli`, `AppData/Local/JetBrains/*/kilo/cli`,
`AppData/Local/JetBrains/*/kilo/repo-cli`, `.cache/JetBrains/*/kilo/cli` and
`.cache/JetBrains/*/kilo/repo-cli`. Keep `plans/`, `retention/`, `memory/`, `session-export*` and
`log/`: the logs are bounded and hold the only local record of a session deletion whose remote
delete failed ([Logs](#logs), [What survives a delete](#what-survives-a-delete)). `.cache/kilo`
(binaries, package clones and downloaded skills) has no catalog line and is not collected.

### Project-local files

`.kilo/` and legacy `.kilocode/` at any depth
([`packages/opencode/src/kilocode/permission/config-paths.ts:9-11`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/permission/config-paths.ts#L9-L11);
[`config.ts:812`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/config/config.ts#L812)) with `rules/`, `workflows/`,
`agent/`, `agents/`, `skills/`, `worktrees/`
([`rules-migrator.ts:11`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/rules-migrator.ts#L11);
[`packages/opencode/src/kilocode/workflows-migrator.ts:14`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/workflows-migrator.ts#L14);
[`packages/opencode/src/kilocode/agent/builder.ts:77`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/agent/builder.ts#L77);
[`marketplace/paths.ts:77`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/marketplace/paths.ts#L77),
[`:82`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/marketplace/paths.ts#L82);
[`tui-worktree.ts:17`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/cli/cmd/tui-worktree.ts#L17), added to
`.git/info/exclude`; the rule directories hold Markdown files such as `.kilocode/rules/<name>.md`,
[`rules-migrator.ts:41-48`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/rules-migrator.ts#L41-L48)), `mcp.json`
([`actions.ts:141-142`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-vscode/src/services/marketplace/actions.ts#L141-L142)),
and `agent-manager.json`, the extension's agent manager state
([`packages/kilo-vscode/src/agent-manager/WorktreeStateManager.ts:113`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-vscode/src/agent-manager/WorktreeStateManager.ts#L113),
[`:147`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-vscode/src/agent-manager/WorktreeStateManager.ts#L147); records in
[Agent manager state](#agent-manager-state)); `kilo.json`, `kilo.jsonc`, `opencode.json` and
`opencode.jsonc`
([`packages/opencode/src/cli/cmd/mcp.ts:399-402`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/cli/cmd/mcp.ts#L399-L402));
`.kilocodeignore`
([`packages/opencode/src/kilocode/ignore-migrator.ts:10`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/ignore-migrator.ts#L10)),
`.kilocodemodes`
([`modes-migrator.ts:187`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/modes-migrator.ts#L187)),
`.kilocoderules`
([`rules-migrator.ts:7`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/rules-migrator.ts#L7)) and
`AGENTS.md`. No per-project database.

### Where the project path is recorded

`kilo.db` holds the project paths in its `project` and `project_directory` tables
([`packages/core/src/project/sql.ts:6-35`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/project/sql.ts#L6-L35); columns
in [`project` and `project_directory`](#project-and-project_directory)), which discovery leaves to
the analyzer. The legacy JSON store names each project's root in the `worktree` key of
`storage/project/<id>.json`, which Kilo's migration copies into the `project` table
([`packages/opencode/src/kilocode/storage/json-migration.ts:264-268`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/storage/json-migration.ts#L264-L268)),
and the pre-fork extension's task index `tasks/_index.json` names each task's project in the
`workspace` of its `entries`, as does a task's `tasks/<id>/history_item.json`
([`packages/kilo-vscode/src/legacy-migration/task-store.ts:135-147`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-vscode/src/legacy-migration/task-store.ts#L135-L147),
[`:169-178`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-vscode/src/legacy-migration/task-store.ts#L169-L178),
[`:194-204`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-vscode/src/legacy-migration/task-store.ts#L194-L204)). A worktree the agent created is a
`project_directory` row of type `git_worktree` and an entry of the project's sandboxes, and a
session run in it has the worktree as its directory
([`sql.ts:16`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/project/sql.ts#L16),
[`:20-35`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/project/sql.ts#L20-L35)). Two files outside the database also name
project roots: each project memory folder's `manifest.json`, in its `canonical` key
([`packages/kilo-memory/src/storage/state.ts:48-76`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-memory/src/storage/state.ts#L48-L76)),
and the project's own `.kilo/agent-manager.json`, which names its worktrees and is found only
inside a project already known.

## Parsing

### Transcript stores

`<data>` below is `~/.local/share/kilo` ([Per-user storage](#per-user-storage)). Transcripts are
SQLite, `<data>/kilo.db` or one of the channel names of [`~/.local/share/kilo`](#localsharekilo)
(tables in [`kilo.db`](#kilodb)); the CLI, the TUI, the VS Code extension and the JetBrains plugin
all write there. The legacy JSON store is `<data>/storage/session/<projectID>/<sesID>.json`,
`storage/message/<sesID>/<msgID>.json` and `storage/part/<msgID>/<prtID>.json`, with the older
`storage/session/info/*.json` and `storage/session/message/<ses>/*.json` and the marker
`storage/migration`
([`packages/opencode/src/storage/storage.ts:65`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/storage/storage.ts#L65),
[`:140-170`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/storage/storage.ts#L140-L170),
[`:193-197`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/storage/storage.ts#L193-L197),
[`:234-236`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/storage/storage.ts#L234-L236)). The VS Code globalStorage
`kilocode.kilo-code/tasks/<id>/` holds only pre-migration Roo-style files
([`roo-code.md`](roo-code.md#transcript-stores) gives their layout). Sessions Kilo Cloud hosts are
listed and fetched on demand and stored nowhere locally
([Hosted sessions](#hosted-sessions)).

#### What survives a delete

Deleting a session removes its `kilo.db` rows as OpenCode does
([What survives a delete in `opencode.md`](opencode.md#what-survives-a-delete)), removes the
session's sandbox policy directory
([`packages/opencode/src/session/session.ts:728-729`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/session/session.ts#L728-L729);
[`packages/opencode/src/kilocode/sandbox/policy.ts:579-588`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/sandbox/policy.ts#L579-L588);
[`packages/opencode/src/kilocode/sandbox/store.ts:90-92`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/sandbox/store.ts#L90-L92)),
and asks Kilo's service to delete the remote copy (`DELETE /api/session/<id>`). It removes the
`storage/session_share/<sessionID>.json` marker only when the service accepts; signed out, or on a
network error or an error status, the marker stays, with a `share remove failed` log line in the
last two cases
([`session.ts:728-733`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/session/session.ts#L728-L733),
[`packages/opencode/src/kilocode/session/index.ts:281-284`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/session/index.ts#L281-L284),
[`packages/opencode/src/kilo-sessions/kilo-sessions.ts:1718-1749`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilo-sessions/kilo-sessions.ts#L1718-L1749)).
The opt-in retention pass ([Retention](#retention)) deletes old sessions through the same delete,
then vacuums the database, which removes the deleted rows' remnants, and records its counts, but no
session id, in `retention/state.json`.

What remains after a delete:

- The TUI's `prompt-history.jsonl` ([TUI prompt history](#tui-prompt-history)), which carries no
  time, session or directory and survives a delete, the retention pass and a removed project; a
  pinned id in the TUI's `session.json` stays until the user unpins it.
- The project memory session digests ([Project memory](#project-memory)), which no session delete
  touches: they go only when the user purges memory or when more than the digest limit pile up.
- A stale turn marker of a process that crashed mid-turn, until another Kilo process scans the
  markers ([Session turn markers](#session-turn-markers)).
- The cloud session marker when the remote delete failed, and the log files ([Logs](#logs)) with
  their `creating session`, `removing share` and `remote session started` lines; whether a delete
  touches them is not stated.

The analyzer reads the `kilo.db` sessions, the marker files, the prompt history as a secondary
source, the remote control lines of the logs and `retention/state.json`
([Parser plan](#parser-plan),
[Secondary sources](../analyzer/docs/output.md#secondary-sources)).

#### Remote control

`kilo remote` (`/remote` in the TUI), `KILO_REMOTE=1` or `"remote_control": true` in the config
connects the host to Kilo Cloud, so sessions can be driven, and created, from there
([`packages/core/src/v1/config/config.ts:121-123`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/v1/config/config.ts#L121-L123),
[`packages/opencode/src/kilo-sessions/kilo-sessions.ts:321`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilo-sessions/kilo-sessions.ts#L321),
[`:921`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilo-sessions/kilo-sessions.ts#L921); the vendor's
[Remote Connections](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-docs/pages/code-with-ai/platforms/cli.md#L662-L700)). The
only local record is in the logs ([Logs](#logs)): `remote connection enabled` and
`remote connection disabled`
([`:1236`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilo-sessions/kilo-sessions.ts#L1236),
[`:1275`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilo-sessions/kilo-sessions.ts#L1275)), and for each session the
remote side creates on the host `remote session started` then `remote session ended`
([`packages/opencode/src/kilo-sessions/remote-session-log.ts:37-61`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilo-sessions/remote-session-log.ts#L37-L61),
[`packages/opencode/src/kilo-sessions/remote-sender.ts:1176-1180`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilo-sessions/remote-sender.ts#L1176-L1180);
their fields in [Logs](#logs)). The session itself is an ordinary row of `kilo.db` with nothing
marking its remote creation. Files the remote side attaches that are not text, images or PDF go to
the temporary root of [Scratch directories](#scratch-directories)
([`packages/opencode/src/kilocode/remote-attachments.ts:8-16`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/remote-attachments.ts#L8-L16),
[`:272-287`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/remote-attachments.ts#L272-L287)).

#### Cloud session import

The VS Code extension lists the user's Kilo Cloud sessions and clones one locally on its first
message, and the remote side can ask for the same clone
([`packages/kilo-vscode/src/kilo-provider/handlers/cloud-session.ts:1-6`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-vscode/src/kilo-provider/handlers/cloud-session.ts#L1-L6)).
The export is fetched from the ingest service
([`packages/kilo-gateway/src/cloud-sessions.ts:153-157`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-gateway/src/cloud-sessions.ts#L153-L157))
and stored as a new local session with a fresh id; the cloud session's parent, share and
permission are dropped
([`:257`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-gateway/src/cloud-sessions.ts#L257),
[`:268`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-gateway/src/cloud-sessions.ts#L268),
[`:295-310`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-gateway/src/cloud-sessions.ts#L295-L310)), and no log line names the
cloud id. A cloned session therefore cannot be told from a local one.

#### Hosted sessions

Sessions that run on Kilo Cloud (cloud agents) are reached through a session host provider that
lists, starts and fetches them over HTTP
([`packages/opencode/src/kilocode/session-hosting/index.ts:12-35`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/session-hosting/index.ts#L12-L35));
nothing in `packages/opencode/src/kilocode/session-hosting` writes a file or a database row, so
their transcripts are server-side only. Their availability is a live
`session.host.availability` event
([`packages/schema/src/kilocode/session-host-event.ts:11-19`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/schema/src/kilocode/session-host-event.ts#L11-L19)),
not a durable one.

### Record schema

The shapes below are those of the V1 and V2 records in [`kilo.db`](#kilodb); the columns that
hold them are under [Other stores](#other-stores). Times are epoch milliseconds. No git branch is
recorded; snapshots hold commit hashes.

#### V1 messages and parts

`message.data` is the V1 message less `id` and `sessionID`, and `part.data` the part less `id`,
`sessionID` and `messageID`
([`packages/core/src/session/sql.ts:20-22`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/session/sql.ts#L20-L22)). User
([`packages/schema/src/v1/session.ts:374-400`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/schema/src/v1/session.ts#L374-L400))
and assistant ([`:499-531`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/schema/src/v1/session.ts#L499-L531)) messages:

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `role` | string | `user`, `assistant` | |
| `time` | object | | |
| `agent` | string | | |
| `model` | object | | |
| `system` | string | | |
| `tools` | object | | |
| `editorContext` | object | | |
| `summary` | object or boolean | | |
| `parentID` | string | | |
| `modelID` | string | | |
| `providerID` | string | | |
| `mode` | string | | |
| `path` | object | | |
| `cost` | number | | |
| `tokens` | object | | |
| `error` | object | | |
| `finish` | string | | |
| `variant` | string | | |

`time` holds `created` and, for the assistant, `completed`; the user `model` holds `providerID`,
`modelID` and `variant`; `path` holds `cwd` and `root`. `editorContext` is Kilo's addition. The
assistant's `tokens`
([`:518-529`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/schema/src/v1/session.ts#L518-L529)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `total` | number | | |
| `input` | number | | |
| `output` | number | | |
| `reasoning` | number | | |
| `cache` | object | | |
| `read` | number | | |
| `write` | number | | |

`read` and `write` are the keys of `cache`.

Parts all carry `type`
([`v1/session.ts:402`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/schema/src/v1/session.ts#L402)): `text`
([`:102-115`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/schema/src/v1/session.ts#L102-L115)), `reasoning`
([`:118-127`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/schema/src/v1/session.ts#L118-L127)), `tool`
([`:289-362`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/schema/src/v1/session.ts#L289-L362)), `step-start`, `step-finish`
([`:241-288`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/schema/src/v1/session.ts#L241-L288)), `file`, `agent`, `subtask`,
`compaction`, `retry`, `patch` and `snapshot`.

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `type` | string | `text`, `reasoning`, `tool`, `step-start`, `step-finish`, `file`, `agent`, `subtask`, `compaction`, `retry`, `patch`, `snapshot` | |
| `text` | string | | |
| `synthetic` | boolean | | |
| `ignored` | boolean | | |
| `time` | object | | |
| `callID` | string | | |
| `tool` | string | | |
| `state` | object | | |
| `reason` | string | | |
| `snapshot` | string | | |
| `model` | object | | |
| `cost` | number | | |
| `tokens` | object | | |

A `tool` part's `state` holds `status` (`pending`, `running`, `completed` or `error`), `input`,
and `output`, `title`, `metadata` and `time` (`start`, `end`) once completed, or `error` once
failed; the call and its result are one part, mutated in place. Kilo adds `model` (`providerID`,
`modelID`), `generationID`, `vercelID`, `metrics` and a `time` to `step-finish`.

#### V2 session messages

`session_message.data`
([`packages/schema/src/session-message.ts:45-213`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/schema/src/session-message.ts#L45-L213))
is the record less `id` and `type`, which have their own columns
([`packages/core/src/session/projector.ts:213-229`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/session/projector.ts#L213-L229)).
Every record has `time.created` and an optional `metadata`
([`session-message.ts:24-28`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/schema/src/session-message.ts#L24-L28)); `agent-switched`
adds `agent` and `model-switched` adds `model`, a model reference with `id`, `providerID` and
`variant` ([`:30-42`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/schema/src/session-message.ts#L30-L42);
[`packages/schema/src/model.ts:13-18`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/schema/src/model.ts#L13-L18)), so their `data`
is `{time, agent}` or `{time, model}`.

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `time` | object | | |
| `metadata` | object | | |
| `agent` | string | | |
| `model` | object | | |
| `text` | string | | |
| `files` | array | | |
| `agents` | array | | |
| `callID` | string | | |
| `command` | string | | |
| `output` | string | | |
| `content` | array | | |
| `snapshot` | object | | |
| `finish` | string | | |
| `cost` | number | | |
| `tokens` | object | | |
| `error` | object | | |
| `reason` | string | | |
| `summary` | string | | |
| `recent` | array | | |

The `type` column is one of `agent-switched`, `model-switched`, `user`, `synthetic`, `system`,
`shell`, `assistant` and `compaction`. Assistant `content[]` items are `text` (`id`, `text`),
`reasoning` (`id`, `text`, `time`) and `tool` (`id`, `name`, `state` with `status`, `input`,
`content[]`, `structured`, `error` and `result`, and `time` with `created`, `ran` and `completed`)
([`:122-158`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/schema/src/session-message.ts#L122-L158)).

#### Legacy extension tasks

The pre-fork extension's `tasks/<id>/api_conversation_history.json` is a JSON array of Anthropic
`MessageParam` records extended with Roo Code's keys
([`packages/kilo-vscode/src/legacy-migration/sessions/lib/legacy-types.ts:13-28`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-vscode/src/legacy-migration/sessions/lib/legacy-types.ts#L13-L28)):
`content` is a string or an array of blocks, `text`, `image`, `tool_use` (`id`, `name`, `input`),
`tool_result` (`tool_use_id`, `content`, `is_error`) and `thinking` (`thinking`), the block shapes
of [Legacy task files in `cline.md`](cline.md#legacy-task-files); a record with `type`
`reasoning` holds the model's reasoning in `text`, `summary` (a list) or
`reasoning_content`
([`packages/kilo-vscode/src/legacy-migration/sessions/lib/parts/parts-util.ts:10-43`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-vscode/src/legacy-migration/sessions/lib/parts/parts-util.ts#L10-L43)),
and `isSummary` marks a condensed summary of the earlier conversation.

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `role` | string | `user`, `assistant` | |
| `content` | string or array | | |
| `ts` | number | | |
| `isSummary` | boolean | | |
| `type` | string | `reasoning`, `text`, `image`, `tool_use`, `tool_result`, `thinking` | |
| `summary` | array | | |
| `reasoning_content` | string | | |
| `text` | string | | |
| `id` | string | | |
| `name` | string | | |
| `input` | object | | |
| `tool_use_id` | string | | |
| `is_error` | boolean | | |
| `thinking` | string | | |

The task's history item, `tasks/<id>/history_item.json` or its entry in the `entries` array of
`tasks/_index.json`, is Roo Code's `HistoryItem`
([Task files in `roo-code.md`](roo-code.md#task-files)), of which Kilo reads `id`, `task`,
`workspace`, `ts`, `mode`, `rootTaskId` and `parentTaskId`
([`packages/kilo-vscode/src/legacy-migration/task-store.ts:194-204`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-vscode/src/legacy-migration/task-store.ts#L194-L204);
[`legacy-types.ts:3-11`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-vscode/src/legacy-migration/sessions/lib/legacy-types.ts#L3-L11)).
Roo Code's `status` is there too. `mode` is the task's mode slug: `code`, `architect`, `ask`,
`debug`, `orchestrator` or a custom mode
([`packages/opencode/src/kilocode/rules-migrator.ts:15`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/rules-migrator.ts#L15)).

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `entries` | array | | |
| `task` | string | | |
| `workspace` | string | | |
| `mode` | string | | |
| `status` | string | | |
| `rootTaskId` | string | | |
| `parentTaskId` | string | | |

### Joins and timestamps

`session.id` ← `message.session_id` ← `part.message_id`; `session.directory` is the working
directory and `session.project_id` → `project.worktree`
([`packages/core/src/project/sql.ts:8`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/project/sql.ts#L8)). A V1 `tool` part
carries both call and result in `state`; a V2 assistant `content[]` tool item likewise, joined on
its `id`. Times are epoch milliseconds. A cloud session marker, a pull request link, a sandbox
policy file and a turn marker join on the session id in their file name (hashed for the sandbox);
the prompt history joins on text alone; a memory digest names its session id in its heading.

### Other stores

#### `kilo.db`

`kilo.db` ([`packages/core/src/session/sql.ts`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/session/sql.ts)) has the
`session` ([`:24-71`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/session/sql.ts#L24-L71)), `message`
([`:73-90`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/session/sql.ts#L73-L90)), `part`
([`:92-114`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/session/sql.ts#L92-L114)), `todo`, `session_message`
([`:135-154`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/session/sql.ts#L135-L154)), `session_input` and
`session_context_epoch` tables, the project tables below, `credential`
([`packages/core/src/credential/sql.ts:5-14`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/credential/sql.ts#L5-L14)),
`account`, `account_state` and the legacy `control_account`
([`packages/core/src/account/sql.ts:6-39`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/account/sql.ts#L6-L39)), and the
shared agent board of [Shared agent board](#shared-agent-board). The projector writes V2
`session_message` and V1 `message` and `part` for every event
([`packages/core/src/session/projector.ts:213-230`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/session/projector.ts#L213-L230),
[`:285-300`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/session/projector.ts#L285-L300),
[`:335-345`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/session/projector.ts#L335-L345)), so either set is complete. WAL
journal as in OpenCode; the `-wal` sidecar holds the latest writes.

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `id` | string | | |
| `project_id` | string | | |
| `workspace_id` | string or null | | |
| `parent_id` | string or null | | |
| `slug` | string | | |
| `directory` | string | | |
| `path` | string or null | | |
| `title` | string | | |
| `version` | string | | |
| `share_url` | string or null | | |
| `metadata` | string or null | | |
| `revert` | string or null | | |
| `summary_additions` | number or null | | |
| `summary_deletions` | number or null | | |
| `summary_files` | number or null | | |
| `summary_diffs` | string or null | | |
| `cost` | number | | |
| `tokens_input` | number | | |
| `tokens_output` | number | | |
| `tokens_reasoning` | number | | |
| `tokens_cache_read` | number | | |
| `tokens_cache_write` | number | | |
| `permission` | string or null | | |
| `agent` | string or null | | |
| `model` | string or null | | |
| `time_created` | number | | |
| `time_updated` | number | | |
| `time_compacting` | number or null | | |
| `time_archived` | number or null | | |
| `session_id` | string | | |
| `message_id` | string | | |
| `type` | string | | |
| `seq` | number or null | | |
| `data` | string | | |

The table covers the `session` columns and those of `message`, `part` and `session_message`; the
JSON columns (`model`, `permission`, `revert`, `metadata`, `summary_diffs`, `data`) are JSON text.
The `summary_*`, `cost` and `tokens_*` columns are the session's diff counts, cost and token
totals, `cost` and the token counts 0 by default
([`sql.ts:40-50`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/session/sql.ts#L40-L50)). Kilo adds a
workspace restore status to `revert`
([`sql.ts:51-54`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/session/sql.ts#L51-L54)).

#### `project` and `project_directory`

([`packages/core/src/project/sql.ts:6-35`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/project/sql.ts#L6-L35)); the
`project_directory` row of an agent worktree has `type` `git_worktree`.

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `worktree` | string | | |
| `vcs` | string or null | | |
| `name` | string or null | | |
| `icon_url` | string or null | | |
| `icon_url_override` | string or null | | |
| `icon_color` | string or null | | |
| `time_initialized` | number or null | | |
| `sandboxes` | string | | |
| `commands` | string or null | | |
| `directory` | string | | |
| `strategy` | string or null | | |

The `type` column of `project_directory` holds `main`, `root` or `git_worktree`.

#### Permission tables and columns

`session.permission` is a JSON ruleset, a list of rules with `permission`, `pattern` and `action`
(`allow`, `deny` or `ask`)
([`packages/core/src/session/sql.ts:55`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/session/sql.ts#L55),
[`packages/schema/src/v1/permission.ts:16-25`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/schema/src/v1/permission.ts#L16-L25)).
The `permission` table, with `id`, `project_id`, `action`, `resource` and the creation and update
times, unique per project, action and resource
([`packages/core/src/permission/sql.ts:7-20`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/permission/sql.ts#L7-L20)),
holds the saved allow grants of the newer permission service
([`packages/core/src/permission.ts:116`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/permission.ts#L116),
[`:131-135`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/permission.ts#L131-L135),
[`:250-256`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/permission.ts#L250-L256)). That service is reached only from
`packages/server`'s handler
([`packages/server/src/handlers/permission.ts:1-3`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/server/src/handlers/permission.ts#L1-L3));
the CLI's session processor and its HTTP permission routes use the 1.x service of
`packages/opencode/src/permission`
([`packages/opencode/src/session/processor.ts:10`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/session/processor.ts#L10),
[`:120`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/session/processor.ts#L120);
[`packages/opencode/src/server/routes/instance/httpapi/handlers/permission.ts:3`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/server/routes/instance/httpapi/handlers/permission.ts#L3),
[`:17`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/server/routes/instance/httpapi/handlers/permission.ts#L17)), so
the CLI writes no row of the `permission` table.

#### Shared agent board

The shared agent board lets a main session and its subagents post messages to each other through
the board tool; it is on unless `shared_agent_board` is `false` or
`KILO_EXPERIMENTAL_SHARED_AGENT_BOARD` is false
([`packages/opencode/src/kilocode/board/enabled.ts:1-19`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/board/enabled.ts#L1-L19)).
It is two tables of `kilo.db`, `kilo_board` per root session and `kilo_board_message` per post
([`packages/core/src/kilocode/board/sql.ts:4-37`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/kilocode/board/sql.ts#L4-L37)).
A post's `type` is `INFO`, `ASK`, `RESULT`, `HOLD` or `VETO`
([`packages/opencode/src/kilocode/board/store.ts:59`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/board/store.ts#L59)),
its recipient a session id or `ALL`; each post is also the input of the board tool call it names,
except a subagent steering notice, which the agent posts itself
([`packages/opencode/src/kilocode/session/steering.ts:62-68`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/session/steering.ts#L62-L68)).
Clearing the board only raises `cleared_seq`; the earlier posts stay in the table
([`store.ts:137-168`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/board/store.ts#L137-L168)).

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `root_session_id` | string | | |
| `objective` | string | | |
| `cleared_seq` | number | | |
| `board_root_session_id` | string | | |
| `sender_session_id` | string | | |
| `recipient` | string | | |
| `body` | string | | |
| `reply_to` | string or null | | |
| `source_message_id` | string | | |
| `source_call_id` | string | | |

#### Cloud session marker

`<data>/storage/session_share/<sessionID>.json`, a storage key written as a JSON file
([`packages/opencode/src/storage/storage.ts:64-66`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/storage/storage.ts#L64-L66);
[`packages/opencode/src/kilo-sessions/kilo-sessions.ts:1580-1589`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilo-sessions/kilo-sessions.ts#L1580-L1589)).
`id` and `ingestPath` (the path on Kilo's session ingest service that receives the session's
events) are written when the ingest service registers the session, at session creation for a user
signed in to Kilo
([`:1463-1497`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilo-sessions/kilo-sessions.ts#L1463-L1497));
`url`, `https://app.kilo.ai/s/<share_token>`, is added by a share
([`:1499-1542`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilo-sessions/kilo-sessions.ts#L1499-L1542)) and removed
by an unshare, which keeps the file
([`:1544-1578`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilo-sessions/kilo-sessions.ts#L1544-L1578)). A session
created while signed out, or with `KILO_DISABLE_SESSION_INGEST` set, has no file. The file carries
no time of its own.

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `id` | string | | |
| `ingestPath` | string | | |
| `url` | string | | |

#### TUI prompt history

The TUI keeps the prompts typed into it in `~/.local/state/kilo/prompt-history.jsonl`
([`packages/tui/src/app.tsx:267-273`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/tui/src/app.tsx#L267-L273),
[`packages/tui/src/prompt/history.tsx:53`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/tui/src/prompt/history.tsx#L53)). Each line
is the typed text, whether it was a shell command (`!`), and the attached file and agent parts;
there is no time, session id or directory
([`:9-25`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/tui/src/prompt/history.tsx#L9-L25)). A prompt is appended unless it repeats
the last one, and the file is rewritten with the last 50 whenever it grows past that and at every
start, which drops lines that do not parse
([`:27-42`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/tui/src/prompt/history.tsx#L27-L42),
[`:54-60`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/tui/src/prompt/history.tsx#L54-L60),
[`:83-108`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/tui/src/prompt/history.tsx#L83-L108)). `prompt-stash.jsonl` beside it holds
one line per prompt set aside unsent, rewritten at each start the same way
([`packages/tui/src/prompt/stash.tsx:9-13`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/tui/src/prompt/stash.tsx#L9-L13),
[`:17-30`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/tui/src/prompt/stash.tsx#L17-L30)). These are OpenCode's older shapes; the
current OpenCode TUI writes others
([Other records of use in `opencode.md`](opencode.md#other-records-of-use)). The TUI's
`session.json`, `frecency.jsonl` and `kv.json` keep OpenCode's shapes.

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `input` | string | | |
| `mode` | string | `normal`, `shell` | |
| `parts` | array | | |
| `timestamp` | number | | |

`timestamp`, in epoch milliseconds, is on stash lines only.

#### Sandbox files

A sandbox policy file holds the confinement applied to a session's commands: whether it is on, the
network mode, the allowed hosts, the extra writable paths and a version
([`packages/opencode/src/kilocode/sandbox/store.ts:11-17`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/sandbox/store.ts#L11-L17),
[`:33-49`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/sandbox/store.ts#L33-L49)); `proxy` mode requires at
least one allowed host. A preference file holds the JSON `true` or `false`
([`packages/opencode/src/kilocode/sandbox/preference.ts:25-29`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/sandbox/preference.ts#L25-L29)).
The vendor's
[Session behavior](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-docs/pages/getting-started/settings/sandboxing.md#L180-L188)
says the latest choice becomes the project's default and that settings changes refresh existing
session policies.

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `enabled` | boolean | | |
| `mode` | string | `allow`, `deny`, `proxy` | |
| `allowedHosts` | array | | |
| `writablePaths` | array | | |
| `version` | number | | |

#### Retention

With `retention.enabled: true` in the global config (off by default), a pass at most every 23 hours
deletes sessions older than `retention.maxAgeDays` (default 30) through the normal session delete,
then runs `VACUUM` and truncates the WAL when enough pages are free, which also removes the deleted
rows' remnants from the database files
([`packages/opencode/src/kilocode/session/retention.ts:13-32`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/session/retention.ts#L13-L32),
[`:163-167`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/session/retention.ts#L163-L167),
[`:301-324`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/session/retention.ts#L301-L324);
[`packages/core/src/v1/config/config.ts:84-98`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/v1/config/config.ts#L84-L98)).
It writes `<data>/retention/state.json` for the last pass
([`retention.ts:45-54`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/session/retention.ts#L45-L54),
[`:326`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/session/retention.ts#L326)), and logs
`retention pass complete`, or `retention pass cancelled`, with the same counts
([`:472`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/session/retention.ts#L472)).

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `at` | number | | |
| `scanned` | number | | |
| `deleted` | number | | |
| `skippedActive` | number | | |
| `failed` | number | | |
| `durationMs` | number | | |
| `cancelled` | boolean | | |
| `reclaimedBytes` | number | | |

#### Project memory

Project memory is off until the user runs `/memory on` (or the extension's switch), which creates
the folder ([`packages/kilo-memory/src/storage/state.ts:184-199`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-memory/src/storage/state.ts#L184-L199);
[`packages/kilo-memory/src/schema.ts:156-168`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-memory/src/schema.ts#L156-L168);
the vendor's [Enable memory](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-docs/pages/customize/context/memory.md#L35-L54)).
`project.md`, `environment.md` and `corrections.md` are Markdown notes; `index.kmem` is the index
built from them; `state.json` holds the switches and counters
([`schema.ts:54-64`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-memory/src/schema.ts#L54-L64)); `manifest.json` names the
project ([`state.ts:48-76`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-memory/src/storage/state.ts#L48-L76)).

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `kind` | string | `kilo-memory` | |
| `display` | string | | |
| `canonical` | string | | |
| `folder` | string | | |
| `createdAt` | string | | |
| `enabled` | boolean | | |
| `autoInject` | boolean | | |
| `autoConsolidate` | boolean | | |
| `stats` | object | | |

`kind`, `display`, `canonical` (the project root's absolute path), `folder` and `createdAt` (ISO
8601) are `manifest.json`'s; the rest are `state.json`'s, whose `stats` holds the times of the last
save and recall and the session id last recalled into
([`schema.ts:37-52`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-memory/src/schema.ts#L37-L52)).

A session digest, `sessions/<ISO time with - for :>_<session id>_<hash>.md`, is a redacted summary
of one session written at the end of a turn while auto-save is on, one file per session, replaced
by the next digest of that session; only the newest digests are kept
([`packages/kilo-memory/src/storage/sessions.ts:54-87`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-memory/src/storage/sessions.ts#L54-L87),
[`:120-130`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-memory/src/storage/sessions.ts#L120-L130),
[`:152-163`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-memory/src/storage/sessions.ts#L152-L163)). Its text is:

```text
# Session <session id>

Version: 1
Updated: <ISO time>
Topic: <topic>

## Summary
<summary>
```

`Fallback: true` follows `Updated:` when the summary was made without the model. The vendor's
[Storage and maintenance](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-docs/pages/customize/context/memory.md#L131-L143) says
purge deletes the whole folder and disabling keeps it.

#### Session turn markers

A turn marker is created before a session's turn starts, its modification time refreshed every 20
seconds while the turn runs, and removed when it ends; another Kilo process reads the markers to
tell a live turn from one a crashed process left, and removes stale ones (on the same host, those
whose process is gone or a day old; from another host, a minute without a refresh)
([`packages/opencode/src/kilocode/session/turn.ts:10-20`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/session/turn.ts#L10-L20),
[`:47-83`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/session/turn.ts#L47-L83),
[`:89-121`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/session/turn.ts#L89-L121)). The file holds only the
host name; the session id and process id are in its name.

#### Agent manager state

`<project>/.kilo/agent-manager.json` is the VS Code agent manager's sidebar state: its worktrees
with path, branch, parent branch, creation time and the cached pull request number, URL and state,
the sessions it manages with their worktree, and the sessions closed in it
([`packages/kilo-vscode/src/agent-manager/WorktreeStateManager.ts:32-63`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-vscode/src/agent-manager/WorktreeStateManager.ts#L32-L63),
[`:84-103`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-vscode/src/agent-manager/WorktreeStateManager.ts#L84-L103)).

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `worktrees` | object | | |
| `sessions` | object | | |
| `closedSessions` | object | | |
| `branch` | string | | |
| `parentBranch` | string | | |
| `createdAt` | string | | |
| `prUrl` | string | | |
| `worktreeId` | string or null | | |

#### Logs

`<data>/log/<ISO time>.log`, bounded: the ten newest `????-??-??T??????.log` files are kept and
each rotates at 50 MB
([`packages/core/src/util/log.ts:24`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/util/log.ts#L24),
[`:70-96`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/util/log.ts#L70-L96),
[`:127-141`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/util/log.ts#L127-L141)); a development run writes `dev.log`
([`:84-85`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/util/log.ts#L84-L85)). Each line is
`INFO  <UTC time to the second> +<ms>ms service=<name> <key>=<value>... <message>`, written at the
default level `INFO` ([`:27`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/util/log.ts#L27),
[`:162-189`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/util/log.ts#L162-L189)), so a reader splits the key-value pairs
off the message. The `kilo-sessions` service
([`packages/opencode/src/kilo-sessions/kilo-sessions.ts:99`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilo-sessions/kilo-sessions.ts#L99))
writes `creating session`, `session bootstrap completed`, the two `session bootstrap skipped:`
lines, `sharing`, `unsharing`, `removing share` and `share remove failed`
([`:1465`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilo-sessions/kilo-sessions.ts#L1465),
[`:1471`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilo-sessions/kilo-sessions.ts#L1471),
[`:1475`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilo-sessions/kilo-sessions.ts#L1475),
[`:1494`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilo-sessions/kilo-sessions.ts#L1494),
[`:1518`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilo-sessions/kilo-sessions.ts#L1518),
[`:1558`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilo-sessions/kilo-sessions.ts#L1558),
[`:1722`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilo-sessions/kilo-sessions.ts#L1722),
[`:1734`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilo-sessions/kilo-sessions.ts#L1734)), each with `sessionId`,
and the remote control lines of [Remote control](#remote-control): the connection lines carry
`connected`, `remote session started` carries `sessionID`, `startedAt` (ISO 8601), `model`
(`<providerID>/<id>`) and `directory`, and `remote session ended` carries `sessionID`,
`startedAt`, `durationMs` and `reason`
([`packages/opencode/src/kilo-sessions/remote-session-log.ts:37-61`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilo-sessions/remote-session-log.ts#L37-L61)).
The retention pass writes the lines of [Retention](#retention).

### Format versions

The JSON `storage/` (two internal migrations,
[`packages/opencode/src/storage/storage.ts:83-200`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/storage/storage.ts#L83-L200))
became SQLite through drizzle migrations under `packages/core/src/database/migration/`;
`opencode-*.db` was renamed `kilo-*.db`; `message` and `part` are kept beside `session_message` by
the migration
[`20260714141136`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/database/migration/20260714141136_session-message-legacy-writer-compat.ts),
and the board tables came with
[`20260828074139`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/database/migration/20260828074139_kilocode_board.ts) and
[`20260903104806`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/database/migration/20260903104806_kilocode_board_reset.ts).
Id prefixes: session `ses`, message `msg` or `msg_`, part `prt`
([`packages/schema/src/session-id.ts:5`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/schema/src/session-id.ts#L5),
[`packages/schema/src/v1/session.ts:17`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/schema/src/v1/session.ts#L17),
[`:23`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/schema/src/v1/session.ts#L23),
[`packages/schema/src/session-message.ts:12`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/schema/src/session-message.ts#L12)).
`.kilocodemodes` is read only for migration.

Builds reviewed: 7.8.3 at
[`76bcfd4`](https://github.com/Kilo-Org/kilocode/commit/76bcfd40be616a72f4697b3041565f322245b462),
7.8.14 at
[`87cc35c`](https://github.com/Kilo-Org/kilocode/commit/87cc35ce6904465a0eed507a8c75c79185863a22)
and 7.8.25 at
[`0a9faef`](https://github.com/Kilo-Org/kilocode/commit/0a9faeff69fff8712da184ee5188bb7985c6f057).
Every record shape the earlier reviews cited is the same at 7.8.25. What changed:

- Session turn markers, the hosted sessions and the `session.host.availability` event were added
  between 7.8.14 and 7.8.25; neither 7.8.3 nor 7.8.14 has
  `packages/opencode/src/kilocode/session/turn.ts`.
- The `wellknown` entry type of `auth.json` is present at 7.8.25
  ([`packages/opencode/src/auth/index.ts:31-35`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/auth/index.ts#L31-L35)).
- Project memory, the JetBrains plugin and the board tables are present at 7.8.3, 7.8.14 and
  7.8.25. Project memory's `decisions.jsonl`, an audit log of an earlier build, is written by none
  of them: 7.8.25 deletes it from a folder whose `manifest.json` is its own
  ([`packages/kilo-memory/src/storage/audit.ts:1`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-memory/src/storage/audit.ts#L1);
  [`packages/kilo-memory/src/storage/state.ts:94-116`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-memory/src/storage/state.ts#L94-L116)).
- Pull request links: an earlier CLI wrote per-worktree records under the storage keys
  `session_pr_link_recorded` and `session_pr_link`, which 7.8.25 deletes at startup
  ([`packages/opencode/src/kilo-sessions/pr-link.ts:545-558`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilo-sessions/pr-link.ts#L545-L558)).
- OpenCode 2.0 moved sessions to a `session_v2` table; Kilo Code has no `session_v2` table at
  7.8.25, so OpenCode's reading of it
  ([Format versions in `opencode.md`](opencode.md#format-versions)) applies only once Kilo Code
  takes that store.

### Secrets

- `auth.json`: per provider an `api` entry's `key`, an `oauth` entry's `refresh` and `access`
  (with `expires`, `accountId`, `enterpriseUrl` and `baseURL`), or a `wellknown` entry's `key` and
  `token` ([`packages/opencode/src/auth/index.ts:15-37`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/auth/index.ts#L15-L37)).
- `daemon.json`: `password` and `token`.
- `kilo.db`: `credential.value`, `account.access_token` and `account.refresh_token`, the same in
  `control_account`, and `session.share_url`, a public link to the session; the database is
  collected as evidence and these columns are the redaction targets
  ([issue 23](https://github.com/seanthegeek/doubleagent/issues/23)).
- Project memory files hold text the agent redacted for secret patterns, best effort.

The `storage/session_share/*.json` files hold no credential: the ingest service authenticates with
the Kilo token from `auth.json`
([`packages/opencode/src/kilo-sessions/kilo-sessions.ts:202-213`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilo-sessions/kilo-sessions.ts#L202-L213)).

### Parser plan

Globs:

- `.local/share/kilo/kilo*.db`, `.local/share/kilo/opencode-*.db`
- `.local/share/kilo/storage/{session,message,part}/**/*.json`
- `.local/share/kilo/storage/session_share/*.json`
- `.local/share/kilo/log/*.log`, `.local/share/kilo/retention/state.json`
- `.local/state/kilo/prompt-history.jsonl`, `.local/state/kilo/prompt-stash.jsonl`
- `.config/kilo/{kilo.jsonc,kilo.json,opencode.jsonc,opencode.json,config.json}`, matched as
  `.config/kilo/<config file>`
- `.local/state/kilo-sandbox-policy/*/*.json`, `.local/state/kilo-sandbox-preference/*.json`
- `.local/share/kilo/memory/*/sessions/*.md`
- `User/globalStorage/kilocode.kilo-code/tasks/<id>/api_conversation_history.json` under any VS
  Code user directory, for pre-migration tasks, with the task's `history_item.json` or
  `tasks/_index.json`

Rows: one per V1 `part`, or per V2 `session_message` content item.

| Column | V1 |
| --- | --- |
| timestamp_utc | `part.data.time.start` or `message.time_created` |
| session_id | `session_id` (`parent_id` for subtasks) |
| project_path | `session.directory` or `project.worktree` |
| git_branch | empty |
| turn_type | text → by role; reasoning → thinking; tool → tool_use plus tool_result; step-*/compaction → system |
| model | `assistant.modelID` |
| tool_name, tool_use_id | `part.data.tool`, `callID` |
| text | `text` or `state.title` |

The code is OpenCode's with Kilo's changes, so the per-call, table and column readers are the
shared OpenCode parser's, which the `kilo-code` parser subclasses. Through it, each session
opens with a
`session start: <title> slug=<slug> version=<version> agent=<agent> directory=<directory>`
row, a `file` part is a `file: <filename> <url>` row and a `retry` part a
`retry attempt=<attempt> <error>` row.

A pre-migration task ([Legacy extension tasks](#legacy-extension-tasks)) is the task directory's
session, with the history item's `workspace` as `project_path`. The history item is one
`task: <task> mode=<mode> status=<status> parent=<parentTaskId>` row at the item's `ts` (each
field only when set), `source_line` 0. Each record's rows follow at its array position, timed by
its `ts`, else the item's: the record's text and `image` blocks are one row by its `role` (an
image as `[image]`), placed where the first was; a text block opening `<environment_details>` is a
`system` row; a `tool_use` block is a `tool_use` row with `name` and `id`; a `tool_result` block a
`tool_result` row joined on `tool_use_id`, its text opening `[error] ` when `is_error` is true; a
`thinking` block, and a `reasoning` record's `summary` texts, `reasoning_content` or `text`, a
`thinking` row only with thinking rows on; a record with `isSummary` one
`summary: <text>` row; any other block a `<type>: <block JSON>` `system` row.

#### Permission rows

The message's agent (V1 user and assistant, V2 assistant, V2 `agent-switched`) is tracked the same
way as the model ([Model rows](#model-rows)), as `permission mode: agent=<name>` and
`permission mode: agent=<old> -> agent=<new>`, because an agent (`build`, `plan` or a custom one) is
a mode that carries its own permission ruleset
([`packages/opencode/src/agent/agent.ts:54`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/agent/agent.ts#L54)).

- Per call: a request has `permission`, `patterns`, `always` and the tool's message and call ids,
  and a reply is `once`, `always` or `reject`
  ([`packages/schema/src/v1/permission.ts:27-38`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/schema/src/v1/permission.ts#L27-L38)).
  The `permission.asked` and `permission.replied` events are not durable, so they are never stored
  ([`:66-70`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/schema/src/v1/permission.ts#L66-L70);
  [`packages/schema/src/event.ts:42-52`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/schema/src/event.ts#L42-L52)). What is stored
  is the tool part: a rejected call ends with `state.status` `error` and `state.error`
  `The user rejected permission to use this specific tool call.`, the same text followed by
  ` with the following feedback: <text>`, or, for a call a deny rule stopped,
  `The user has specified a rule which prevents you from using this specific tool call. Here are some of the relevant rules <JSON>`
  ([`packages/core/src/v1/permission.ts:7-27`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/v1/permission.ts#L7-L27);
  [`packages/opencode/src/permission/index.ts:314-333`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/permission/index.ts#L314-L333);
  [`packages/opencode/src/session/processor.ts:299-312`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/session/processor.ts#L299-L312);
  [`packages/tui/src/util/error.ts:125-133`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/tui/src/util/error.ts#L125-L133)). A
  reject also rejects every other pending request of the session; an approval leaves no mark on
  the part. The parser matches these prefixes and writes `permission decision: rejected`,
  `permission decision: rejected with feedback` or `permission decision: denied by rule` after the
  tool's `tool_result` row, with its `tool_name`, `callID` as `tool_use_id` and the result's time.
- Standing, written by the agent: an `always` reply adds an `allow` rule for each of the request's
  patterns to the `permission` key of the first existing global config file among `kilo.jsonc`,
  `kilo.json`, `opencode.jsonc`, `opencode.json` and `config.json` in `~/.config/kilo`
  ([`permission/index.ts:357-365`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/permission/index.ts#L357-L365),
  [`packages/opencode/src/config/config.ts:206-215`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/config/config.ts#L206-L215),
  [`:1147-1153`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/config/config.ts#L1147-L1153)), and the extension's
  selective save writes `allow` and `deny` rules the same way
  ([`permission/index.ts:385-403`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/permission/index.ts#L385-L403)).
  Protected configuration paths are downgraded to `once`
  ([`:338-340`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/permission/index.ts#L338-L340)). "Allow everything" writes
  `*` `*` `allow` to the global config, or appends that rule to the session's ruleset when given a
  session, and turning it off writes `null`
  ([`packages/opencode/src/kilocode/permission/allow-everything.ts:15`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/permission/allow-everything.ts#L15),
  [`:19-35`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/permission/allow-everything.ts#L19-L35),
  [`:41-56`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/permission/allow-everything.ts#L41-L56)). The
  global config therefore records every standing grant the user gave, with the file's modification
  time as the only time. The `permission` key is one action or an object of permission names, each
  one action or an object of patterns to actions, `null` being Kilo's delete sentinel
  ([`packages/core/src/v1/config/permission.ts:5-56`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/v1/config/permission.ts#L5-L56);
  [`permission/index.ts:522-543`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/permission/index.ts#L522-L543)). The
  parser reads each of the five files as JSONC and writes one
  `permission setting: permission.<permission>.<pattern>=<action> (from .config/kilo/<file>, file mtime)`
  row per rule, `permission.<permission>=<action>` for a rule given as one action and
  `permission=<action>` for a key that is one action
  (`permission setting: permission=<action> (from .config/kilo/<file>, file mtime)`), at the
  file's modification time with no session.
- Standing, the `permission` table and `session.permission` column of
  [Permission tables and columns](#permission-tables-and-columns), through the OpenCode reader:
  `permission setting: <action> <resource>=allow (from kilo.db permission, time_created)` per row
  (the CLI writes none) and
  `permission setting: session <permission> <pattern>=<effect> (from kilo.db session.permission, time_updated)`
  per rule.
- Standing, the sandbox: each key of a policy file of [Sandbox files](#sandbox-files) is one
  `permission setting: sandbox.<key>=<value> (from <file>, file mtime)` row, and a preference file
  one `permission setting: sandbox.enabled=<true|false> (from <file>, file mtime)` row, with no
  session, since the file names are hashes. A file that is not the expected JSON is one `parser:`
  row.
- Not read: the project's `kilo.json`, `kilo.jsonc` and `.kilo/` permission keys, and the project's
  `sandboxes` ([issue 98](https://github.com/seanthegeek/doubleagent/issues/98)).

#### Model rows

A `system` row `model: <modelID> provider=<providerID>` precedes the first message of a session
that names a model (V1 user `model`, V1 assistant `providerID` and `modelID`, V2 assistant `model`),
and `model: <old> -> <new> provider=<providerID>` precedes every later message, or stands for every
V2 `model-switched` record, whose model differs. Both these rows and the `permission mode:` rows of
[Permission rows](#permission-rows) take the message's timestamp. Kilo writes V1 and V2 for every
event ([`kilo.db`](#kilodb)) and the parser reads V2 only for a session with no V1 rows, so for
Kilo sessions the change rows come from the V1 messages and the V2 switch records are read only
when V1 is missing.

#### Past-use rows and secondary sources

The OpenCode reader takes `~/.local/state/kilo/prompt-history.jsonl`
([TUI prompt history](#tui-prompt-history)) as a secondary source (`secondary()`, `has_secondary`
set): its lines carry no time, session or directory, so each is a `user` row with an empty
`timestamp_utc` that pairs with a transcript prompt on text alone, and a shell command (`mode`
`shell`) is a `tool_use` row with tool `shell` and no id; what it survives is in
[What survives a delete](#what-survives-a-delete). `prompt-stash.jsonl` is a primary record of
unsent prompts, one `system` row per line at its `timestamp`,
`prompt stashed (unsent): <text> (from .local/state/kilo/prompt-stash.jsonl, timestamp)`. The TUI's
`session.json` and `frecency.jsonl` follow OpenCode's plan
([Past-use rows and secondary sources in
`opencode.md`](opencode.md#past-use-rows-and-secondary-sources)).
The remote control lines of the logs are under
[Sync, relay and origin rows](#sync-relay-and-origin-rows), `retention/state.json` under
[Deletion rows](#deletion-rows).

The project memory session digests ([Project memory](#project-memory)) record past use that
outlives a session delete; the plan is one `system` row per digest,
`memory digest: <topic>: <summary> (from .local/share/kilo/memory/<folder>/sessions/<file>, Updated)`,
at its `Updated:` time with the session id of its heading and the manifest's `canonical` project
root as `project_path`, the summary being every line after `## Summary` and the topic `-` when the
digest has none. They summarise rather than repeat events, so they are a primary record of what
they hold, not a secondary source. Not read: `kilo_board_message` (each post repeats a board tool
call's input), `session-turn/` (a running or crashed turn, no content), `.kilo/agent-manager.json`
and project memory's `project.md`, `environment.md` and `corrections.md` (notes with no session or
time).

#### Sync, relay and origin rows

Sync: the share code is OpenCode's
([Sync, relay and origin rows of `opencode.md`](opencode.md#sync-relay-and-origin-rows)), and Kilo
changed what it calls. Kilo replaced OpenCode's share service with its own cloud sessions: for a
user signed in to Kilo (a `kilo` entry in `auth.json`, or `KILO_API_KEY`,
[`packages/opencode/src/kilo-sessions/kilo-sessions.ts:202-213`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilo-sessions/kilo-sessions.ts#L202-L213)),
every new session is registered with `POST /api/session` on `https://ingest.kilosessions.ai`
(`KILO_SESSION_INGEST_URL` overrides) at creation
([`:248-275`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilo-sessions/kilo-sessions.ts#L248-L275),
[`:683-691`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilo-sessions/kilo-sessions.ts#L683-L691)), and from then on
every message, part, model and diff is posted to its `ingestPath`
([`:783-791`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilo-sessions/kilo-sessions.ts#L783-L791);
[`packages/opencode/src/kilo-sessions/ingest-queue.ts:296-310`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilo-sessions/ingest-queue.ts#L296-L310)),
with session open, close and status records and pull request links
([`ingest-queue.ts:12-62`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilo-sessions/ingest-queue.ts#L12-L62)); the
`session_status` item's transition record is sent to the server and not stored locally. This is
on by default once signed in; only `KILO_DISABLE_SESSION_INGEST` turns it off, and
`KILO_DISABLE_SHARE` blocks only the public link
([`kilo-sessions.ts:278-281`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilo-sessions/kilo-sessions.ts#L278-L281)).
`/share` and `/unshare` reach Kilo's service instead of OpenCode's
([`packages/opencode/src/share/session.ts:25-36`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/share/session.ts#L25-L36)),
whose `ShareNext` initialisation Kilo commented out
([`packages/opencode/src/project/bootstrap.ts:9-11`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/project/bootstrap.ts#L9-L11)),
so the upstream `session_share` table stays empty; `session.share_url` and the
`session.updated.1` events still record the public URL as upstream
([`packages/opencode/src/session/session.ts:1014-1016`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/session/session.ts#L1014-L1016)).
The vendor's
[Where your session data lives](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-docs/pages/collaborate/sessions-sharing.md#L60-L64)
says a share keeps a public copy while the original stays private.

The marker is the `storage/session_share/<sessionID>.json` file
([Cloud session marker](#cloud-session-marker)). The parser yields one `system` row per file,
`session synced: kilo session <id> ingest <ingestPath> share <url or -> (from storage/session_share/<sessionID>.json, file mtime)`,
timestamped by the file's modification time, with the session id from the file name; a file that
is not a JSON object is one `parser:` row. The OpenCode reader also yields the event rows of
[Sync, relay and origin rows of `opencode.md`](opencode.md#sync-relay-and-origin-rows) from
`kilo.db` (`session.share_url` itself is not read). These rows are fixture only: no real install
has shown the marker file. What a delete does to the file and the remote copy is in
[What survives a delete](#what-survives-a-delete). The service's log lines ([Logs](#logs)) are not
read.

A second uploader, session export to
`https://supermassive-black-hole.kiloapps.io/v1/session-export/batch` for personal accounts on free
Kilo gateway models, is compiled in but switched off (`enabled = false`)
([`packages/opencode/src/kilocode/session-export/session-export.ts:35`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/session-export/session-export.ts#L35),
[`worker/endpoint.ts:1`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/session-export/worker/endpoint.ts#L1),
[`eligibility.ts:16-22`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/session-export/eligibility.ts#L16-L22);
[`packages/opencode/src/kilocode/bootstrap.ts:73-91`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/bootstrap.ts#L73-L91)).

Telemetry goes to PostHog, a product analytics service
([`packages/kilo-telemetry/src/client.ts:9`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-telemetry/src/client.ts#L9)), its
level set by `KILO_TELEMETRY_LEVEL`
([`packages/kilo-telemetry/src/telemetry.ts:125-127`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/kilo-telemetry/src/telemetry.ts#L125-L127));
a search of its event definitions for `prompt`, `text`, `content` and `message` found counts and
ids, not prompt text.

Origin and remote control: the parser reads the remote control lines of `log/<time>.log`
([Remote control](#remote-control), in the format of [Logs](#logs)) as `system` rows ending
`(from log/<file>, <time basis>)`: `remote control: connection enabled connected=<true|false>` and
`remote control: connection disabled` at the line's time;
`remote control: session started model=<model> directory=<directory>` at the line's `startedAt`,
with `sessionID` as `session_id` and the directory as `project_path`; and
`remote control: session ended reason=<reason> durationMs=<n> startedAt=<time>` at the line's
time with the session. A line that ends with one of the four messages without its fields is a
`parser: unparseable line` row; every other log line is not read. These are neither sync nor relay
nor origin rows: remote control is a live control channel through which Kilo Cloud sends prompts
and receives the session's events for display
([`packages/opencode/src/kilo-sessions/remote-sender.ts:590-660`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilo-sessions/remote-sender.ts#L590-L660)),
the session it creates runs on the host and is stored in `kilo.db` like any other, and the copy
Kilo keeps is the ingest upload the marker above records, so `records_relay` and `records_origin`
stay unset, as for Codex's remote control. A session cloned from a cloud session records nothing
of its origin ([Cloud session import](#cloud-session-import)), and a hosted session has no local
record ([Hosted sessions](#hosted-sessions)).

#### Scratch directory rows

The agent's worktrees ([Scratch directories](#scratch-directories)) are named by the
`project_directory` rows of type `git_worktree` and by the project's sandboxes
([Where the project path is recorded](#where-the-project-path-is-recorded)). The OpenCode reader
writes one `system` row
`scratch directory: <directory> (git worktree of <project worktree> strategy=<strategy>) (from kilo.db project_directory, time_created)`
per `project_directory` row of type `git_worktree`, with the directory as `project_path` and no
session ([Scratch directory rows of `opencode.md`](opencode.md#scratch-directory-rows)). The
temporary root and its remote attachments are outside the home and give no row.

#### Deletion rows

The retention pass deletes old sessions in bulk and vacuums the database ([Retention](#retention);
[What survives a delete](#what-survives-a-delete)). Its `state.json` is one `system` row at its
`at`,
`sessions deleted by retention: deleted=<n> scanned=<n> skippedActive=<n> failed=<n> durationMs=<n> (from retention/state.json, at)`,
with ` cancelled=true` and ` reclaimedBytes=<n>` before the source when the file has them and `-`
for an absent count, and no `session_id`, since the file names no session. An archived session
(`session.time_archived`) is a `session archived (from kilo.db session, time_archived)` row through
the OpenCode reader.

#### Bad-line rows

Every file the parser reads is covered. Each line of `prompt-history.jsonl` and
`prompt-stash.jsonl` that does not decode is its own `parser: unparseable line (<reason>): <line>`
row, read before Kilo's next start drops it; a log line that names a remote control message
without its fields is `parser: unparseable line (<message> without its fields): <line>`. A file
that does not decode is one row: a global config file
`parser: <file> could not be decoded (<reason>)`, or `parser: <file> is not a JSON object` when
it decodes to something else; a sandbox policy file or a cloud session marker
`parser: <file> is not a JSON object`; a sandbox preference file
`parser: <file> is not true or false`; the retention state
`parser: retention/state.json is not a JSON object`; a memory digest without a `## Summary` line
`parser: <file> is not a memory digest (no ## Summary line)`, and its folder's `manifest.json`
when it is not a JSON object `parser: memory manifest.json is not a JSON object` beside the
digest's row. A pre-migration `api_conversation_history.json` is read record by record: a record
that does not decode is `parser: unparseable record <N> (<reason>): <text from that record on>`,
with every earlier record's rows kept, and a file that is not an array, an array cut off after a
record or data after the array is one `parser: not a JSON array`,
`parser: array cut off after record <n>` or `parser: unexpected data after the array` row.
Through the OpenCode reader a database that cannot be opened is
`parser: database could not be read: <reason>`, a legacy session or message file that does not
decode is `parser: session file could not be decoded` or
`parser: message file could not be decoded`, and each undecodable database record or part file
is its own `parser: unparseable record <rowid> (<table>.data: <reason>): <data>` or
`parser: unparseable part <partID> (<reason>): <text>` row carrying it in full, the reason of a
part file that cannot be read being `unreadable: <reason>`
([Bad-line rows of `opencode.md`](opencode.md#bad-line-rows);
[issue 139](https://github.com/seanthegeek/doubleagent/issues/139)). A database with
`session_v2` ([Format versions](#format-versions)) is read from it, its `session_message`
transcript and the 1.x tables only for a session that `session_message` lacks
([issue 165](https://github.com/seanthegeek/doubleagent/issues/165)).

#### Fixtures

Database rows, as the projector writes them
([`packages/core/src/session/projector.ts:213-229`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/session/projector.ts#L213-L229),
[`:285-300`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/session/projector.ts#L285-L300),
[`:335-345`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/core/src/session/projector.ts#L335-L345)), one per table:

```json
{"_table":"session","record":{"id":"ses_7f3a2c91d4eKq8","project_id":"prj_4b81e0","parent_id":null,"slug":"quiet-harbor","directory":"/srv/proj","title":"Rename the config loader","version":"7.8.25","agent":"code","model":"{\"id\":\"claude-sonnet-4-6\",\"providerID\":\"kilo\"}","permission":null,"time_created":1761700000000,"time_updated":1761700042000}}
{"_table":"message","record":{"id":"msg_7f3a2c92a01Lm2","session_id":"ses_7f3a2c91d4eKq8","time_created":1761700003000,"data":"{\"role\":\"assistant\",\"parentID\":\"msg_7f3a2c92900Ab1\",\"modelID\":\"claude-sonnet-4-6\",\"providerID\":\"kilo\",\"mode\":\"code\",\"agent\":\"code\",\"path\":{\"cwd\":\"/srv/proj\",\"root\":\"/srv/proj\"},\"cost\":0.0127,\"tokens\":{\"input\":5400,\"output\":212,\"reasoning\":0,\"cache\":{\"read\":4100,\"write\":0}},\"time\":{\"created\":1761700003000,\"completed\":1761700011000},\"finish\":\"tool-calls\"}"}}
{"_table":"part","record":{"id":"prt_7f3a2c92b10Zx4","message_id":"msg_7f3a2c92a01Lm2","session_id":"ses_7f3a2c91d4eKq8","time_created":1761700004000,"data":"{\"type\":\"tool\",\"callID\":\"toolu_01QmW\",\"tool\":\"bash\",\"state\":{\"status\":\"error\",\"input\":{\"command\":\"rm -rf build\"},\"error\":\"The user rejected permission to use this specific tool call.\",\"time\":{\"start\":1761700004000,\"end\":1761700006500}}}"}}
{"_table":"session_message","record":{"id":"msg_7f3a2c93c22Pq7","session_id":"ses_7f3a2c91d4eKq8","type":"model-switched","seq":7,"time_created":1761700020000,"data":"{\"time\":{\"created\":1761700020000},\"model\":{\"id\":\"gpt-5.1\",\"providerID\":\"kilo\"}}"}}
```

State files, one record each: a prompt history line and a stash line
([`packages/tui/src/prompt/history.tsx:9-25`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/tui/src/prompt/history.tsx#L9-L25);
[`packages/tui/src/prompt/stash.tsx:9-13`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/tui/src/prompt/stash.tsx#L9-L13)), a global
config after an "always" approval ([`toConfig`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/permission/index.ts#L522-L543)),
a sandbox policy ([`store.ts:70-80`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/sandbox/store.ts#L70-L80)),
a retention state ([`retention.ts:45-54`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/session/retention.ts#L45-L54))
and a cloud session marker
([`kilo-sessions.ts:1580-1589`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilo-sessions/kilo-sessions.ts#L1580-L1589)):

```json
{"input":"!git log --oneline -5","mode":"shell","parts":[]}
{"input":"try the streaming parser later","parts":[],"timestamp":1761700050000}
{"permission":{"bash":{"npm test":"allow","git push *":"deny"},"edit":"ask"}}
{"enabled":true,"mode":"deny","allowedHosts":[],"writablePaths":["/srv/proj/.cache"],"version":2}
{"at":1761786400000,"scanned":118,"deleted":9,"skippedActive":0,"failed":1,"durationMs":2140,"reclaimedBytes":3145728}
{"id":"5d1c0a7e-2f44-4c1e-9a0b-6e2f8c3d9b17","ingestPath":"/api/session/5d1c0a7e-2f44-4c1e-9a0b-6e2f8c3d9b17/ingest","url":"https://app.kilo.ai/s/Zk3pQ9"}
```

Log lines, in the format of [Logs](#logs), from `remote-session-log.ts`:

```text
INFO  2026-10-29T09:14:02 +3ms service=kilo-sessions connected=false remote connection enabled
INFO  2026-10-29T09:15:40 +21ms service=kilo-sessions sessionID=ses_7f3a2c91d4eKq8 startedAt=2026-10-29T09:15:40.118Z durationMs=94200 reason=closed remote session ended
```

#### Source host rows

The agent's source-host integrations are server-side, and the parser yields no `source host:`
row. Kilo Cloud Agents run a remote task on a GitHub, GitLab or plain git repository
(`kilo cloud start --repo <repo> --repo-type github|gitlab|git`,
[`packages/opencode/src/kilocode/cli/cmd/cloud.ts:12-45`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/cli/cmd/cloud.ts#L12-L45);
[`packages/opencode/src/kilocode/cloud/repository.ts:4-25`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilocode/cloud/repository.ts#L4-L25)):
the repository access and its grant belong to the Kilo account on the server, and the cloud
commands write no local file, so the remote conversation is fetched each time
([Hosted sessions](#hosted-sessions)). The GitHub Copilot model provider's device flow asks for
`read:user` and keeps its token in `auth.json` under `github-copilot`
([`packages/opencode/src/plugin/github-copilot/copilot.ts:94-95`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/plugin/github-copilot/copilot.ts#L94-L95),
[`:243`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/plugin/github-copilot/copilot.ts#L243)); GitLab sign-in comes
from the third-party `opencode-gitlab-auth` package
([`packages/opencode/src/plugin/index.ts:17`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/plugin/index.ts#L17)), whose
storage this repository does not show. Neither records the account's login. The CLI alone
(`KILO_CLIENT` `cli`) records the pull request a session owns, from a `gh pr create` or push the
session made or a `kilo pr link <url>`, in `<data>/storage/session_pr_link_session/<sessionID>.json`
with `evidence` in `pr_create`, `push` or `user`
([`packages/opencode/src/kilo-sessions/pr-link.ts:14-41`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilo-sessions/pr-link.ts#L14-L41),
[`:363-373`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/kilo-sessions/pr-link.ts#L363-L373);
[`packages/opencode/src/cli/cmd/pr.ts:205`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/cli/cmd/pr.ts#L205)), and
sends it to the ingest service; the file is not parsed.

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `link` | object | | |
| `platform` | string | | |
| `prUrl` | string | | |
| `prNumber` | number | | |
| `headRef` | string | | |
| `headSha` | string | | |
| `evidence` | string | `pr_create`, `push`, `user` | |

## Confidence

- High: the XDG roots, database names, credential files, project table columns and project
  dotfiles; the drizzle columns and V1 and V2 struct fields (read from source).
- Medium: the serialised key order of V2 `session_message.data`.
- Medium: that the CLI writes no row of the `permission` table; a round should confirm the table
  stays empty after an "always" approval.
- Low: what `globalStorage/kilocode.kilo-code` holds on a current install beyond legacy files.
- For the round: the cloud session marker after sign-in, share, unshare and delete; the remote
  control log lines; a sandbox policy and preference file; a retention pass; a memory digest after
  `/memory on`; a turn marker during a turn; `daemon.json` after `kilo daemon`; the JetBrains
  plugin's `kilo/` system folder and its `options/` files; `.kilo/agent-manager.json`; board rows
  from a subagent run.
- Not determined: where the `opencode-gitlab-auth` package keeps a GitLab sign-in.
- Not determined: the code path that imports the legacy JSON `storage/` into SQLite
  ([`packages/opencode/src/storage/db.ts`](https://github.com/Kilo-Org/kilocode/blob/0a9faeff69fff8712da184ee5188bb7985c6f057/packages/opencode/src/storage/db.ts), not traced),
  and whether an extension import of a Roo Code task keeps any trace of its source.
