# copilot-cli

GitHub Copilot CLI (`copilot`, catalog agent `copilot-cli`) is GitHub's closed-source terminal
coding agent. It writes one event log per session, `events.jsonl`, under
`~/.copilot/session-state/<session id>/`, and keeps copies of the conversation in a cross-session
SQLite database and a prompt history file. The GitHub Copilot Desktop app runs its sessions through
a bundled Copilot CLI and writes them into the same `~/.copilot/session-state/` directory, in this
format, with `workspace.yaml` `client_name` `github/autopilot` where the CLI writes `github/cli`;
those sessions stay `copilot-cli`, and the app's own files and store are the `copilot-desktop`
agent's ([`copilot-desktop.md`](copilot-desktop.md)). The CLI shares no format with the VS Code
Copilot Chat extension ([`vscode.md`](vscode.md)) or the `copilot` catalog agent. The
`copilot-cli` parser reads the event logs, the database, the prompt history and the CLI-written
Copilot Desktop sessions.

## Source and evidence

Closed source: `github/copilot-cli` holds issues and a changelog only, so there are no commit links.
Evidence is the **shipped bundle**; each claim names its evidence level:

- **Bundle** 1.0.91: `npm pack @github/copilot` 1.0.91 (5 KB loader, `buildMetadata.gitCommit
  216810c5`) → `npm pack @github/copilot-linux-x64@1.0.91`, a 178 MB Node single-executable
  (`copilot`). Its SEA blob (SEA is a Node single executable application: a Node binary with a
  script and assets appended in a `__NODE_SEA` blob; magic 0x143da20 at 0x67a01b8) holds
  `sea-loader.js` (122 KB, `scratchpad/npm/cp-sea-loader.js`) and one asset `copilot.tgz` (69.7 MB,
  `scratchpad/npm/cp-asset-copilot.tgz`), extracted to `scratchpad/npm/cp-pkg/package/`: `app.js`
  (7.6 MB JS), `prebuilds/linux-x64/cli-native.node` (8 MB) and `runtime.node` (122 MB, Rust).
  Citations to it: "app.js" with the surrounding identifiers, "sea-loader" (same code appears in
  cc-loader), "native"/"runtime" for byte-string hits.
- **Bundle** 1.0.98: `@github/copilot-darwin-arm64` 1.0.98 (loader package `@github/copilot` 1.0.98,
  `buildMetadata.gitCommit` `45ef16251c8`), the `copilot` executable, sha256
  `b1a76d9eec3c88e8545ee96834189ab08c3d75ef65500a60925187fa6c33fdf1`, 167,681,584 bytes, copied from
  a Homebrew npm install on 2026-10-15. It has the SEA layout of 1.0.91 plus a one-byte field after
  the blob flags: the blob magic `0x0143da20` is at file offset `0x5888000`; flags `0x19` (no
  warning, assets, exec arguments); `sea-loader.js` (124,974 bytes, sha256
  `a9bc83c428713c66388718e554df0be3f9aa92f3e79df17fe47dbadb2e9e7eaf`); one asset `copilot.tgz` at
  `0x58a6877` (70,377,164 bytes, sha256
  `26c6165cb45bce1b37a3a8d6bbcd484e2d938cf513d5970cbdf9ef0da8b92a70`); six Node exec arguments
  (`--no-warnings`, `--expose-gc`, ...). `app.js` is now a 42 KB bootstrap (sha256
  `c5f37d9040e5d866e01e4266c83d87d08ff3ab0804d6a0a0dc863b4414196a7b`) that imports `cli-main.js`
  (8,193,186 bytes, sha256 `151c2e2a37ddb2514fc5cdbd5c473f9aeeb1a74d59e5062f3921a46d12a8f1c2`), the
  minified CLI. The state paths, the event store and the session database are in, and written by,
  `prebuilds/darwin-arm64/runtime.node`, a Rust Node addon (97,100,816 bytes, sha256
  `e5bb26efbdff6cc21a607d3fee0fb9efa933ebeb3d7fe97332ee1a7f0793a02e`). The package also ships the
  event schema as JSON Schema, `schemas/session-events.schema.json` (990,556 bytes, sha256
  `81c4a1eb9f0f3bd21a68219f30aaa774fd7c041eafa76c62ff691ac53efae1ed`, 159 event types with a
  description per field) and as TypeScript, `copilot-sdk/generated/session-events.d.ts` (sha256
  `a2f3897149e0ec1aee24c23dbed3632b44916971d06f5c82768ead98981eaa37`). Citations, for collection
  paths and records alike, are `copilot 1.0.98 darwin-arm64 <file>@<byte offset>` with a literal
  snippet, shortened after this paragraph to `cli-main.js@<n>`, `runtime.node@<n>` and `schema@<n>`
  (the schema file, at the definition's `"title"`). Offsets change with every build; grep for the
  snippet. The binary was read with `strings`, `grep` and Python, never executed.
- **Docs**: [GitHub Copilot CLI configuration
  directory](https://docs.github.com/en/copilot/reference/copilot-cli-reference/cli-config-dir-reference)
  (the Copilot CLI configuration directory reference, fetched 2026-10-15) for the directory layout
  and `permissions-config.json`.
- **Local**: names-only listing of `~/.copilot` and `~/.cache/copilot`.

### Observed

- Windows, winget, standalone CLI 1.0.98, 2026-10-17: installed with `winget install GitHub.Copilot`
  (winget is the Windows Package Manager; `GitHub.Copilot` is the package id) on a Windows 11 ARM64
  Parallels VM (build 10.0.26300.9550, user `sean`, home `C:\Users\sean`) reverted to a clean
  snapshot that had never run Copilot Desktop; all times UTC; names, sizes, key names and types
  only, with a names-only listing diff after every step. The install (listing 21:44Z) wrote the
  winget package directory, a user `PATH` entry, the HKCU uninstall key (HKCU is
  `HKEY_CURRENT_USER`, the registry hive of the signed-in user) and the installer download ([Outside
  the catalog](#outside-the-catalog)); no `.copilot`, no `AppData\Local\copilot`, no credential file
  and no `DeveloperTools` registry key yet. The first start, `copilot` in `C:\Users\sean\demo` at
  21:46:48 (listing 21:48Z), wrote the extracted package
  `AppData\Local\copilot\pkg\win32-arm64\1.0.98\` (181,983,762 bytes),
  `AppData\Local\copilot\marketplaces\.auto-update.lock` and `.copilot\` with the Linux layout
  ([Inside `~/.copilot`](#inside-copilot)): `config.json`, `command-history-state.json` (43 bytes),
  `open-sessions-state.json` (188 bytes), `sidebar-sessions-state\<sha256 of cwd>.json` (125 bytes),
  `logs\process-1792273608752-2868.log` and the session directory with `workspace.yaml` (212 bytes),
  `checkpoints\index.md` (172 bytes) and the process's `inuse.2868` pair. The user logged in from
  inside the session (`/login`, the device-code flow, in which the user types a one-time code into
  github.com in a browser), completing at 21:48:02; while the code was pending the process log
  looped, about 280,000 lines in 74 seconds and four rotations at 10 MB ([Process
  logs](#process-logs)). The login rewrote `config.json` (418 bytes), stored the token in Windows
  Credential Manager (Windows' per-user credential store; [Credentials](#credentials)), created the
  registry device id ([Outside the
  catalog](#outside-the-catalog)) and wrote in `AppData\Local\copilot\` `copilot-user-cache.json`
  (3,309 bytes), `exp-cache.json` (10,904 bytes), `memory-enablement-cache-config.json` (351 bytes),
  `managed-settings.<sha256>.order.json` (36 bytes) and `managed-settings.<sha256>.fence` (36
  bytes), each with a `.lock`, and an empty `managed-settings\` directory ([Package and cache
  directory](#package-and-cache-directory)). The first prompt (21:50:55, listing 22:02Z), a task
  that read web pages, created `events.jsonl` (1,216,194 bytes by 22:01:14), `session-store.db`
  (4,096 bytes) with `-wal` (1,268,992 bytes) and `-shm`, and `rewind-file-snapshots\tracking.json`
  (29 bytes), and grew `command-history-state.json` to 114 bytes and `workspace.yaml` to 251 bytes
  (the `name` key added). One tool call denied and one approved with "allow always" (listing 22:06Z;
  the two steps share a diff) wrote `permissions-config.json` at 22:06:06 (142 bytes, one `write`
  grant for `demo`; [`permissions-config.json`](#permissions-configjson)), grew `events.jsonl` by
  750 KB, the WAL to 1,590,352 bytes and `command-history-state.json` to 190 bytes, and the approved
  `apply_patch` call wrote `demo\ai-coding-tools-review.md`. A `/model` switch to Auto (listing
  22:08Z) grew `events.jsonl` by 870 bytes and `command-history-state.json` by 14 bytes and rewrote
  `exp-cache.json` at the same size, nothing else. Quitting at 22:08:31 wrote
  `rewind-file-snapshots\index.json` (1,204 bytes), grew `events.jsonl` by 8 KB to 1,973,737 bytes
  and removed the `inuse.2868` pair; the log then held 55 `assistant.message`, 109 tool calls, 79
  permission requests with their completions, 6 `user.message`, 4 `session.model_change`, 12 of each
  of the eight `model.*` types and 2 of each of four `subagent.*` types ([Event
  types](#event-types)). Resuming at 22:08:50 (`copilot` again, the session picked from the resume
  picker; listing 22:09Z) wrote a new process log `process-1792274929863-4428.log` (36 KB, no loop),
  rewrote `workspace.yaml` from 251 to 376 bytes with the sync keys `remote_steerable`,
  `mc_task_id`, `mc_session_id` and `mc_last_event_id` (Mission Control ids; Mission Control is
  GitHub's service that mirrors a session's events for remote viewing and steering;
  [`workspace.yaml`](#workspaceyaml)), rewrote `open-sessions-state.json` (373 bytes) and the
  sidebar file (169 bytes), touched `config.json` at the same size, and added
  `managed-settings\<sha256>.json` (252 bytes) to the cache, with
  `managed-settings.<sha256>.order.json` at 101 bytes and `memory-enablement-cache-config.json` at
  532. At 22:11:04 the user started `copilot` again in `demo` (a new session whose directory held
  `workspace.yaml` of 212 bytes and a 1,162-byte `events.jsonl` with `session.start`,
  `session.model_change` and `session.shutdown` only, and no `session.db` or
  `rewind-file-snapshots`) and deleted the first session from it with `/session delete <id> --yes
  --remote`, after trying `/session`, `/session delete-all`, `/session delete <id>` and `/session
  delete copilot <id>`; the user agreed to delete the synced copy too, and quit at 22:16:32 (listing
  22:17Z). What the delete removed and left is in [What survives a delete](#what-survives-a-delete);
  `mcp-config.json` was never written. An npm install of the standalone CLI on Windows is not
  observed ([issue 133](https://github.com/seanthegeek/doubleagent/issues/133)).
- Windows, through Copilot Desktop, bundled CLI 1.0.99-2, 2026-10-17: Copilot CLI 1.0.99-2 as the
  bundled CLI of GitHub Copilot Desktop 1.1.30 (`copilot-sdk/cliVersion.d.ts` `COPILOT_CLI_VERSION =
  "1.0.99-2"`, `COPILOT_CLI_USE_NPM_PACKAGE = false`), downloaded by the app to
  `AppData\Local\github-copilot-sdk\cli\1.0.99-2\copilot.exe` (158,577,952 bytes, sha256
  `b7bc340acf1c82228de2e3504444847f8e18beec1f07f72f260f1bbeb4c3a9b2`), on a Windows 11 ARM64
  Parallels VM (build 10.0.26300.9550, home `C:\Users\sean`); names, sizes, key names and types
  only; the round is in [Source and evidence in
  `copilot-desktop.md`](copilot-desktop.md#source-and-evidence). The CLI wrote the Linux layout
  under `C:\Users\sean\.copilot` ([Per-user storage](#per-user-storage)): `config.json` (131 bytes,
  one key; [Credentials](#credentials)), `session-store.db` (4096 bytes, `-wal` 609,792 then 704,552
  bytes, `-shm`), `logs/.copilot-log.lock` and 17 `logs/process-<epoch ms>-<pid>.log` (1.7 KB to 47
  KB), `session-state/`, empty `extensions/` and empty `installed-plugins/`. Adding three MCP
  servers (Model Context Protocol servers, programs that offer the agent extra tools) from the app
  wrote `mcp-config.json` (725 bytes), `.mcp-writer.binding` (157 bytes) and `.mcp-writer.lock` (0
  bytes) ([Transcript stores](#transcript-stores); one server's `env` held a token,
  [Credentials](#credentials)); the two servers whose commands were not installed failed to spawn,
  six and five times ([Process logs](#process-logs)), and the one that connected left one tool cache
  per session in `AppData\Local\copilot\mcp-tools\`. The cache directory `AppData\Local\copilot`
  also held `copilot-user-cache.json` (3,309 bytes), `managed-settings/<sha256>.json` (252 bytes),
  `tgrep-serve.lock`, the marketplace clone (4,560 files, 263,616,505 bytes) and the package
  `pkg/win32-arm64/1.0.99-2/` (255 files, 191,966,761 bytes)
  ([Package and cache directory](#package-and-cache-directory)). The general chat's `events.jsonl`
  (115 lines) held 3 `user.message`, 8 `assistant.message`, 9 tool calls (`powershell` and
  `create`), 27 `hook.start` and `hook.end` pairs, 8 `session.usage_record`, 3
  `session.usage_checkpoint`, 1 `system.message` and 1 `session.shutdown`; the project session's (30
  lines) one prompt, one `powershell` call, 7 hook pairs and a `session.shutdown`
  ([Event types](#event-types)). Three `create` calls failed ("Parent directory does not exist") and
  the first `powershell` call exited 1 because Windows PowerShell's execution policy refused
  `npm.ps1`; both are ordinary failed tool results. `session-store.db` had 17 tables at
  `schema_version` 8: `sessions` 2, `turns` 4 (3 for the chat, 1 for the project session),
  `session_files` 3 (`tool_name` `create`), `search_index` 4 (`source_type` `turn`) and
  `assistant_usage_events` 10 (`model` `claude-sonnet-5.5`, `initiator` `user` and `agent`,
  `api_endpoint` `/v1/messages`, `reasoning_effort` `medium`, `finish_reason` `stop` and
  `tool_calls`), with `checkpoints`, `dynamic_context_items`, `forge_skill_proposals`,
  `forge_trajectory_events` and `session_refs` empty ([`session-store.db`](#session-storedb)). The
  app also created seven sessions that were never prompted
  ([Transcript stores](#transcript-stores)).
- Linux, npm, 1.0.98, 2026-10-16: installed with `sudo npm install -g @github/copilot` on an Ubuntu
  26.04.1 arm64 Parallels VM (kernel 7.0.0-38-generic, home `/home/sean`); names, sizes, key names
  and types only, read with `analyzer/tools/shape.py`. Exercised: `copilot login`, then two
  interactive sessions, the first in `~/demo` (created 21:01:19.518Z, prompts at 21:02:40Z and
  21:05:34.923Z, shut down 21:16:01Z, all UTC; 194 lines) and the second in `~/secret` (created
  21:16:26.715Z, one prompt at 21:17:18Z, shut down 21:19:27Z; 35 lines), after which the user
  removed the project with `rm -r secret/`. Every action ran before the first listing, so no
  per-action listing diff exists; the times above and the files' modification times are the only
  mapping from action to file. `copilot login` wrote a process log (105 bytes) and put the token in
  the GNOME keyring ([Credentials](#credentials)); the first session start wrote one file outside
  the catalog ([Outside the catalog](#outside-the-catalog)); each session wrote its
  `session-state/<uuid>/` directory, a process log (16 KB and 15 KB), `session-store.db` rows (in a
  1.37 MB WAL beside a 4096-byte main file), a `sidebar-sessions-state` file and its prompts in
  `command-history-state.json`. `~/.copilot` held the macOS file set except `settings.json`, which
  was not written, and `~/.cache/copilot` the names macOS keeps in `~/Library/Caches/copilot`, with
  the package under `pkg/linux-arm64/1.0.98/` ([Per-user storage](#per-user-storage)); there was no
  `~/.config/.copilot` or `~/.local/state/.copilot`. Session 1 called `bash` 2 times, `apply_patch`
  2 (11 files, then 1), `view` 3, `glob` 1, `web_fetch` 18, `web_search` 7 and
  `fetch_copilot_cli_documentation` 1, with 20 permission requests (`kind` `shell` 2, `url` 17,
  `write` 1, each carrying `permissionMode` `manual` and `agentMode` `interactive`): 19 `approved`
  and one `approved-for-location` for the write, all `decisionSource` `human_response`, and one
  automatic model resolution. Session 2 had one `write` request approved and a model switch the user
  typed: `/model`, choosing Auto, at 21:19:20Z ([Event types](#event-types)). What the project's
  removal left is in [What survives a delete](#what-survives-a-delete). Not exercised: a denial, a
  resume, a rewind, a session deleted with the CLI's own command, a subagent, a compaction, plan or
  autopilot mode, a scheduled task.
- macOS, Homebrew npm, 1.0.98, 2026-10-15: the author's workstation; names, sizes, key names and
  types only, read with `analyzer/tools/shape.py` and a schema dump. Three interactive sessions in
  `manual` permission mode (52, 168 and 64 lines, none bad), with `bash`, `view`, `apply_patch`,
  `web_fetch`, `web_search` (an MCP tool), `ask_user` and `fetch_copilot_cli_documentation` calls;
  17 permission prompts approved once (`approved`) or for the project (`approved-for-location`, a
  command and a write approval); one automatic model resolution per session; three normal shutdowns;
  one session whose `turns` rows were re-indexed ([Other stores](#other-stores)). They wrote the
  `~/.copilot` file set of [Per-user storage](#per-user-storage), `settings.json` and
  `permissions-config.json` included, the session store's rows in a 1.7 MB `-wal` beside a 4 KB main
  file, and `~/Library/Caches/copilot` with the package `pkg/darwin-arm64/1.0.98/`; there was no
  `~/.config/.copilot`, `~/.local/state/.copilot` or `~/.cache/copilot`. Not exercised: a denial, a
  model switch the user typed, a resumed session, a rewind, a deleted session, a subagent, a
  compaction.

### Source reviews

- 2026-10-19: microsoft/vscode at
  [`4d22f4f`](https://github.com/microsoft/vscode/commit/4d22f4f922eb012217ac65672ccce02f59d63706),
  for the Copilot CLI sessions that VS Code's Copilot Chat extension runs in a git worktree; added
  the worktree to [Scratch directories](#scratch-directories) and the `vscode.metadata.json` file VS
  Code writes into the session directory to
  [its subsection under Other stores](#vscodemetadatajson).

## Collection

### Per-user storage

Config home: `configDir ?? process.env.COPILOT_HOME ?? join(homedir(), ".copilot")` (app.js
`function dot(e)`, `dQe`; native: "Set the configuration directory (default: ~/.copilot).
Deprecated: use COPILOT_HOME env var instead"). Same on all OSes; docs confirm `~/.copilot` and
`COPILOT_HOME`. On Windows that is `%USERPROFILE%\.copilot`: the Copilot Desktop-bundled
`copilot.exe` 1.0.99-2 carries the strings `COPILOT_HOME`, `USERPROFILE` and the named pipe prefix
`\\.\pipe\copilot-taskbar-`, and no keyring strings.

XDG handling: on start the CLI *migrates* state out of XDG dirs into `~/.copilot` (app.js
`function CEn(e,t)`): from `$XDG_STATE_HOME/.copilot/` the entries `session-state`,
`session-store.db`, `command-history-state`, `command-history-state.json`, `installed-plugins`; from
`$XDG_CONFIG_HOME/.copilot/` the entries `config.json`, `config`, `mcp-config`, `lsp-config`,
`permissions-config`, `copilot-instructions.md`, `mcp-oauth-config`, `hooks`. So older installs with
XDG set may still have `$XDG_STATE_HOME/.copilot` and `$XDG_CONFIG_HOME/.copilot` (default
`~/.local/state/.copilot`, `~/.config/.copilot`), and symlinks are left behind (`symlinkSync` in the
same function).

VS Code writes into `~/.copilot` without the Copilot CLI. On Ubuntu 26.04.1 arm64 (2026-10-26),
VS Code 1.143.1's first start, before any extension was installed and with no Copilot CLI on the
host, wrote `~/.copilot/logs/.copilot-log.lock` and `logs/process-<epoch>-<pid>.log`, the logs of
the Copilot agent that VS Code 1.143 bundles ([Process logs](#process-logs)); it was seen during
a Claude Code round on that host ([Observed in `claude-code.md`](claude-code.md#observed)). The
`copilot-cli|.copilot` catalog line collects them under this agent, so a `~/.copilot` that holds
only `logs/`, with no `session-state/`, may be VS Code's rather than evidence that the Copilot CLI
ran.

#### Scratch directories

VS Code's Copilot Chat extension can run a Copilot CLI session in a git worktree it creates at
`<repo parent>/<repo>.worktrees/<branch>`, unless the git extension's global state holds a
`worktreeRoot:<repo>` root
([`extensions/git/src/repository.ts:1952-1970`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/extensions/git/src/repository.ts#L1952-L1970),
[`chatSessionWorktreeServiceImpl.ts:113`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/extensions/copilot/src/extension/chatSessions/vscode-node/chatSessionWorktreeServiceImpl.ts#L113)),
and records it in `session-state/<id>/vscode.metadata.json`
([`vscode.metadata.json`](#vscodemetadatajson)), at microsoft/vscode
[`4d22f4f`](https://github.com/microsoft/vscode/commit/4d22f4f922eb012217ac65672ccce02f59d63706);
not seen on a real install. Such a worktree is a scratch directory of the agent, not the user's
project. It sits beside the user's repository, outside every catalog line, so it is not
collected, and `discover_projects` and `Find-Projects` do not read `vscode.metadata.json`; see
[`vscode.md`](vscode.md).

#### Inside `~/.copilot`

The docs table plus the bundle:

- `session-state/<sessionId>/` session history: `events.jsonl` (runtime: "Computes the absolute path
  to a session's persisted events.jsonl file"; app.js crash report copies `e.sessionFilePath` as
  `events.jsonl`), `workspace.yaml` (app.js `loadSessionFsWorkspace`, keys `id`, `created_at`,
  `updated_at`, `name`, `user_named`, `summary_count`; `workspaceLoadPersistedSessionCwd` reads cwd
  from it), `plan.md`, `autopilot-objective.json` (app.js `copySessionFsOptionalFile`), checkpoints
  and files subdirs (`/session checkpoints|files|plan` commands). Observed at 1.0.98 (macOS and
  Linux): `checkpoints/index.md` (a Markdown table of compaction checkpoints), `files/` ("Persistent
  storage for session artifacts", `runtime.node@82945561`), `research/`,
  `rewind-file-snapshots/index.json` and `tracking.json` (file snapshots for `/rewind`,
  `runtime.node@82575983` `rewind-file-snapshotstracking.json`), `.workspace-fork.lock`; beside the
  session directories `.session-operation-locks/<uuid>.lock` (`runtime.node@82828676`). On the
Windows winget install (1.0.98) a running process also kept `inuse.<pid>.hold` (0 bytes) and
`inuse.<pid>.lock` (5 bytes) in the session directory, its in-use markers, removed when it quit;
`rewind-file-snapshots/tracking.json` was written at the first prompt and `index.json` at the quit.
The bundle
  also names `session-state/<id>/session.db`, a per-session SQLite of the agent's todo tables
  (`cli-main.js@149653` `this.join(this.sessionStatePath,"session.db")`; not observed), and the
  legacy flat transcript `session-state/<id>.jsonl` that 1.0.98 no longer reads
  (`cli-main.js@1654639` `Rwr=".jsonl"`).
- `session-store.db` SQLite "cross-session data such as checkpoint indexing and search" (docs;
  runtime SQL `FROM checkpoints ... session_id, checkpoint_number`), in WAL mode with its rows in
  the `-wal` sidecar on every install, so the sidecars must be collected with it; tables, writers
  and sizes in [`session-store.db`](#session-storedb).
- `command-history-state.json`, the Ctrl+R prompt history
  ([`command-history-state.json`](#command-history-statejson)). `open-sessions-state.json`
  open-session tracker (`runtime.node@79300674` `open-sessions`): `{}` with no session open, and
  `{schemaVersion, openedAt, refreshedAt, working}` with one open (373 bytes after the resume on the
  Windows winget install, `{}` again after the delete). `sidebar-sessions-state/<sha256 of
  cwd>.json` `{schemaVersion, cwd, sessionIds}` (real install; the file name is the sha256 of `cwd`,
  verified; `runtime.node@87698768` `sidebar-sessions`); on the Windows winget install a resume
  from the resume picker added a second id that got an operation lock and a sidebar entry but never
  a directory (the picker appears to start a fresh session before switching to the chosen one).
  `servers/` and `installed-plugins.lock` (observed, empty); `extensions/` (observed empty on
  Windows through Copilot Desktop's bundled 1.0.99-2).
- Docs-only at 1.0.98 (not observed): `mcp-secrets/` ("Local fallback storage and
  index for MCP secret placeholders"; `cli-main.js@5455460` `huo="mcp-secrets"`), `plugin-data/`
  (`cli-main.js@1688706`), `providers.json` (the BYOK, bring-your-own-key, provider and model
  registry; `cli-main.js@4314272` `jwr="providers.json"`, overridable with
  `COPILOT_PROVIDERS_CONFIG`).
- `logs/` process logs (native: "Set log file directory (default: ~/.copilot/logs/)"); local
  `logs/process-<epoch>-<pid>.log` and `logs/.copilot-log.lock`, rotated at 10 MB; a login left
  pending on Windows 1.0.98 wrote about 45 MB of them ([Process logs](#process-logs)). The logs
  stay collected and are not excluded ([Exclusions](#exclusions)), because the sign-in lines that
  name the account are in them. `ide/<uuid>.lock` IDE discovery lock files (app.js `QB` scans
  `Gi(e,"state")/ide` for `.lock`; local `ide/`). `state/` for the session store default path
  (app.js `sessionStoreDefaultPath(Gi(e,"state"))`).
- `config.json` "automatically managed application state (authentication, installed plugins...)"
  (docs), `settings.json` user settings (docs; runtime "Settings migration ... between config.json
  and settings.json"; written on the macOS install, absent on the Linux one), `mcp-config.json`
  (native help text; observed on Windows with `.mcp-writer.binding` and `.mcp-writer.lock` beside
  it, the writer's lock and its binding to the file's volume and file index), `mcp-oauth-config`
  (app.js `JDn="mcp-oauth-config"`), `permissions-config.json`, `lsp-config`,
  `command-history-state/`, `installed-plugins/<marketplace>/<plugin>` (docs; runtime
  "installed-plugins root"), `copilot-instructions.md`, `instructions/*.instructions.md`,
  `agents/*.agent.md`, `skills/<name>/SKILL.md`, `hooks/` (docs; app.js help list
  `$HOME/.copilot/copilot-instructions.md`, `$HOME/.copilot/instructions/**/*.instructions.md`).

#### Package and cache directory

- Package cache for auto-update (sea-loader `Xi()`): `$COPILOT_PKG_CACHE_HOME/pkg`,
  `$COPILOT_CACHE_HOME/pkg`, `~/Library/Caches/copilot/pkg` (macOS), `%LOCALAPPDATA%\copilot\pkg`
  (Windows), `$XDG_CACHE_HOME/copilot/pkg` or `~/.cache/copilot/pkg` (Linux), `$COPILOT_HOME/pkg`,
  `~/.copilot/pkg`; extraction marker `.extraction-complete`. Each version directory holds
  `.extraction-complete`, `cli-main.js`, `prebuilds/<platform>/runtime.node` and `schemas/`
  (`prebuilds/linux-arm64/runtime.node` in `pkg/linux-arm64/1.0.98/` on Linux); on Windows
  `pkg/win32-arm64/1.0.98/` was 181,983,762 bytes (winget) and `pkg/win32-arm64/1.0.99-2/` 255 files
  and 191,966,761 bytes (Copilot Desktop's bundled CLI), with the `inuse.<pid>.hold` and `.lock`
  pairs of five processes. Local `~/.cache/copilot/copilot-user-cache.json` (runtime literal
  `copilot-user-cache`). On macOS 1.0.98 `~/Library/Caches/copilot`, and on Linux 1.0.98
  `~/.cache/copilot` with the same names, also held `exp-cache.json` (the ExP experiment-assignment
  cache; ExP is Microsoft's experimentation service), `managed-settings/` ("cache for enterprise
  managed settings, safe to delete", `runtime.node@79814215`; on the Windows winget install also
  `managed-settings.<sha256>.fence`, 36 bytes, beside the `.order.json` and `.lock`, contents not
  read), `memory-enablement-cache-config.json` and `marketplaces/`, each with `.lock` files.
- Five of these files are JSONC, not JSON (Linux install): `config.json`, `copilot-user-cache.json`,
  `exp-cache.json`, `memory-enablement-cache-config.json` and `managed-settings/<hex>.json` start
  with one or two `//` comment lines ("Disposable cache for ..., safe to delete. Managed
  automatically." and "User settings belong in settings.json.") and then hold plain JSON, which is
  why the macOS survey could not parse them. Shapes observed on Linux: `copilot-user-cache.json` is
  `copilotUserCache{<key>: {schemaVersion, generation, retrievedAt, response}}`, where `response` is
  the Copilot user entitlement response (`login`, `copilot_plan`, `access_type_sku`,
  `organization_login_list`, `organization_list`, `enterprise_list`,
  `endpoints{api, origin-tracker, proxy, telemetry, exp}`,
  `quota_snapshots{chat, completions, premium_interactions}` with entitlement and remaining counts,
  `quota_reset_date`, and feature flags such as `chat_enabled`, `is_mcp_enabled`,
  `codex_agent_enabled`, `cloud_session_storage_enabled`, `cli_remote_control_enabled`; on
  Windows, 3,309 bytes through Copilot Desktop's bundled 1.0.99-2, also `analytics_tracking_id`,
  `assigned_date`, `can_signup_for_limited`, `copilotignore_enabled`, `restricted_telemetry`,
  `enterprise_usage_telemetry`, `is_staff`, `te`, `token_based_billing`, `can_upgrade_plan` and
  `quota_reset_date_utc`);
  `exp-cache.json` is
  `expAssignmentsCache{"v3:<hex>": {schemaVersion, retrievedAt, response{Features[...], ...}}}`
  (feature names such as `anthropic_tool_search_enabled`, `openai_toolsearch_enabled`,
  `gpt-default`); `managed-settings/<hex>.json` is
  `{schemaVersion, retrievedAtMs, account, response, ignoredPaths[]}` (`response` null, on Windows
  too, 252 bytes), beside
  `managed-settings.<hex>.order.json` `{lastAppliedOrder, nextOrder}`;
  `memory-enablement-cache-config.json` is
  `memoryEnablementCache{"v1:<hex>": {schemaVersion, retrievedAt, enabled}}`.
- `marketplaces/.auto-update.lock` was the only file in `marketplaces/` on macOS and Linux. On
  Windows (CLI 1.0.99-2, bundled with Copilot Desktop) the same directory held `freshness.json`
  (98 bytes), `github-awesome-copilot.lock` (0 bytes) and `github-awesome-copilot/`, a shallow git
  clone of the awesome-copilot plugin marketplace (4,533 files and 160 MB outside its `.git`),
  263 MB in all; it is excluded from collector 1.13.0 ([Exclusions](#exclusions), [issue
  131](https://github.com/seanthegeek/doubleagent/issues/131)).
- Also on Windows (bundled 1.0.99-2): `tgrep-serve.lock`, and `mcp-tools/<sha256>.json`, one file
  per MCP server that connected in a session (two files of 20,383 and 20,384 bytes, written at
  19:59:20Z and 20:04:57Z by the two sessions that connected, both naming `playwright`), shape
  `{schemaVersion, serverName, tools: [{name, description, annotations{destructiveHint,
  openWorldHint, readOnlyHint, title}, inputSchema{...}}], updatedAt}`, the cached tool list of a
  connected MCP server (25 tools); collected.

#### Outside the catalog

- Outside `~/.copilot` and the cache, not in the catalog: the first session start on Linux wrote
  `~/.cache/Microsoft/DeveloperTools/deviceid` (36 bytes, a UUID), the device id Microsoft's
  developer tools share ([issue 148](https://github.com/seanthegeek/doubleagent/issues/148)); on
  Windows the login of the winget install created the registry value
  `HKCU\SOFTWARE\Microsoft\DeveloperTools\deviceid` instead (a string value, type `REG_SZ`, of 36
  characters), as Copilot Desktop's did, and no file; neither wrote
  `AppData\Local\Microsoft\DeveloperTools` or `~/.cache` on Windows. The registry is outside the
  collector's scope.
- The winget install puts the executable in
  `AppData/Local/Microsoft/WinGet/Packages/GitHub.Copilot_Microsoft.Winget.Source_8wekyb3d8bbwe/`
  (`copilot.exe`, 154,351,392 bytes, file date 2026-10-14, version 1.0.98, and winget's own 16 KB
  database `GitHub.Copilot_Microsoft.Winget.Source_8wekyb3d8bbwe.db`), outside every catalog entry,
  like the npm global directory on Linux; the program holds no user state. The install also adds a
  user `PATH` entry for that directory, the HKCU uninstall key
  `GitHub.Copilot_Microsoft.Winget.Source_8wekyb3d8bbwe` (DisplayName `Copilot CLI`,
  DisplayVersion `v1.0.98`, InstallLocation that directory) and leaves the installer download under
  `AppData\Local\Temp\WinGet\GitHub.Copilot.v1.0.98`.
- System policy: `/etc/github-copilot/managed-settings.json`, `/etc/github-copilot/policy.d`
  (runtime).

### Credentials

- GitHub token: OS keyring by default (runtime vendors `keyring-core` and
  `zbus-secret-service-keyring-store`; env `COPILOT_DISABLE_KEYTAR`), with `storeTokenPlaintext`
  setting "Store auth token in plaintext (less secure)" (runtime settings schema) which puts it in
  `config.json` ("authentication" per docs). On the Linux install `copilot login` wrote the token to
  the GNOME keyring (`~/.local/share/keyrings/login.keyring`, modified at the minute of the login),
  as the runtime's keyring-store strings predict, and `config.json` held no token: its keys were
  `loggedInUsers[{host, login, kind}]`, `lastLoggedInUser{host, login}`, `firstLaunchAt`,
  `appTipShown` and a fifth key of 20 or more characters whose name was not recorded; the Windows
  winget install named it: `reasoningSummariesCleanupDone` (418-byte `config.json` after the login,
  the same five keys). Through Copilot Desktop's bundled 1.0.99-2 on Windows `config.json` was 131
  bytes with the one key `firstLaunchAt`. On the winget install the login wrote two Windows
  Credential Manager files under `AppData/Roaming/Microsoft/Credentials/` (634 and 666 bytes, the
  only ones in the profile besides one of 2026-10-10 under `AppData/Local/Microsoft/Credentials/`)
  in the same second, so the token went
  to Credential Manager as Copilot Desktop's did; the target name was not determined (unreadable
  from the SSH session), and `config.json` held no token. Keep `config.json` a secret glob for the
  plaintext fallback.
- `copilot-user-cache.json` holds no token (Linux and Windows installs; shape in
  [Package and cache directory](#package-and-cache-directory)): it names the login, plan,
  organizations and quotas, and stays collected and unflagged, which answers
  [issue 130](https://github.com/seanthegeek/doubleagent/issues/130) for that file. The process log
  names the login ([Process logs](#process-logs)).
- Env precedence `COPILOT_GITHUB_TOKEN`, `GH_TOKEN`, `GITHUB_TOKEN`; falls back to `gh auth token`
  (app.js `Lgo`, runtime "`gh auth token` stdout collection"), so `~/.config/gh/hosts.yml` is a
  related credential.
- `mcp-oauth-config` MCP OAuth store (`copilot-mcp-oauth-store-key-v1`); `mcp-config.json` can embed
  headers/tokens, and on the Windows install it did: the `github` server's `env` held
  `GITHUB_PERSONAL_ACCESS_TOKEN`, a 33-character GitHub personal access token in plain text, written
  when the user added the server from Copilot Desktop's settings; `.copilot/mcp-config.json` is a
  secret glob. `settings.json`
  `sandbox.userPolicy.network.proxy.password`; `.copilot/settings.json` is flagged.
- `mcp-secrets/`, the local fallback store for MCP secret placeholders
  ([Inside `~/.copilot`](#inside-copilot)), is flagged
  (`.copilot/mcp-secrets/*`).
- The pre-migration copies under `~/.config/.copilot` of `config.json`, `mcp-config` and
  `mcp-oauth-config` are flagged like the `~/.copilot` ones (`.config/.copilot/config.json`,
  `.config/.copilot/mcp-config*`, `.config/.copilot/mcp-oauth-config*`).
- A project's `.github/copilot/settings.json`, `.github/copilot/settings.local.json`,
  `.github/mcp.json` and `.mcp.json` ([Project-local files](#project-local-files)) can hold
  MCP server `env` and `headers` values and are flagged.
- The CLI does not use `~/.config/github-copilot/{hosts,apps}.json` (no hit in app.js, native or
  runtime).

### Exclusions

`.copilot/pkg`, `.cache/copilot/pkg`, `Library/Caches/copilot/pkg`, `AppData/Local/copilot/pkg`
(extracted ~180 MB packages per version; 192 MB for `win32-arm64/1.0.99-2`),
`.copilot/installed-plugins` (marketplace copies; keep manifests if desired). Keep `logs/`,
`session-state/`, `session-store.db`.

From collector 1.13.0 also `.cache/copilot/marketplaces`, `Library/Caches/copilot/marketplaces` and
`AppData/Local/copilot/marketplaces`: the git clone of each plugin marketplace the CLI reads (263 MB
and 4,560 files on the Windows install;
[issue 131](https://github.com/seanthegeek/doubleagent/issues/131)). The `skipped_excluded` row
keeps the directory's size, but which marketplaces were cloned, and when, is inside it and is lost;
`--full` collects it.

### Project-local files

`.github/copilot-instructions.md`, `.github/instructions/*.instructions.md`,
`.github/git-commit-instructions.md`, `AGENTS.md`, `CLAUDE.md`, `GEMINI.md`, `.claude/rules/**/*.md`
(read; app.js help list), `.github/copilot/settings.json`, `.github/copilot/settings.local.json`
(app.js `LO`), `.mcp.json`, `.github/mcp.json` (native help), `.github/lsp.json`, `.github/hooks/`,
`.github/skills/`, `.github/agents/` (app.js). No per-project database.

### Where the project path is recorded

`session-state/<id>/workspace.yaml` `cwd`, the persisted session cwd (app.js
`workspaceLoadPersistedSessionCwd(FP(Wp(e,t),"workspace.yaml"))`; the key name is not visible in
the JS, which leaves it to the runtime, so it was read from the installs); the file's other keys,
the sync marker among them, are in [`workspace.yaml`](#workspaceyaml).

`permissions-config.json` is "organized by project location" (docs): its `locations` keys are
project paths, the git root or cwd ([`permissions-config.json`](#permissions-configjson)).

`sidebar-sessions-state/*.json` `cwd` maps a project path to its session ids, and outlives the
project: the removed `~/secret` kept its file on Linux. `events.jsonl` `session.start`
`data.context.cwd` and `session-store.db` `sessions.cwd` carry it too
([Joins and timestamps](#joins-and-timestamps)).

## Parsing

### Transcript stores

Relative to the home; `~/.copilot` unless `COPILOT_HOME` moves it.
Installs that predate the XDG migration keep the same names under
`.local/state/.copilot` (session state, store, history) and
`.config/.copilot` (config, permissions)
([Per-user storage](#per-user-storage)).

| Path | Content | Parse |
| --- | --- | --- |
| `.copilot/session-state/<uuid>/events.jsonl` | the session event log | yes, primary |
| `.copilot/session-state/<uuid>/workspace.yaml` | session metadata: name, cwd, counts | yes |
| `.copilot/session-state/<uuid>/rewind-file-snapshots/index.json` | per-turn file snapshots for `/rewind`, with the prompt | yes, secondary |
| `.copilot/session-state/<uuid>/rewind-file-snapshots/tracking.json` | `{schema, tracking}` | no |
| `.copilot/session-state/<uuid>/checkpoints/index.md` | Markdown table of compaction checkpoints | no |
| `.copilot/session-state/<uuid>/session.db` | per-session SQLite of the agent's own todo tables (bundle; not observed) | no |
| `.copilot/session-state/<uuid>/files/`, `research/`, `plan.md` | agent artifacts (empty on every install) | no |
| `.copilot/session-state/<uuid>.jsonl` | legacy flat transcript, refused by 1.0.98 | no, [Format versions](#format-versions) |
| `.copilot/session-store.db` (+`-wal`, `-shm`) | cross-session SQLite ("chronicle" store) | yes, secondary |
| `.copilot/command-history-state.json` | prompt history for Ctrl+R | yes, secondary |
| `.copilot/permissions-config.json` | standing approvals per project | yes |
| `.copilot/settings.json` | user settings, JSON with comments (written on macOS, absent on the Linux install) | yes, `allowedUrls`/`deniedUrls` |
| `.copilot/sidebar-sessions-state/<sha256 of cwd>.json` | `{schemaVersion, cwd, sessionIds}` | yes, as a join |
| `.copilot/logs/process-<epoch ms>-<pid>.log` | process log | yes, sign-in line only |
| `.copilot/open-sessions-state.json` | open-session tracker (`{}` observed) | no |
| `.copilot/mcp-config.json` | MCP server configuration, `mcpServers{<name>{type, command, args[], tools[], env{}}}` (`type` `local` on the Windows install); `env` can hold a token in plain text (a GitHub personal access token on the Windows install) | no ([MCP](#mcp)) |
| `.copilot/.mcp-writer.binding`, `.copilot/.mcp-writer.lock` | the lock of the writer of `mcp-config.json`: `lock{file_index, platform, volume}` (`platform` `windows`), `root{...}` (Windows install) | no |
| `AppData/Local/copilot/mcp-tools/<sha256>.json` | the cached tool list of a connected MCP server, `{schemaVersion, serverName, tools[{name, description, annotations{destructiveHint, openWorldHint, readOnlyHint, title}, inputSchema}], updatedAt}`, one per session that connected (Windows install; not observed on macOS or Linux) | no ([MCP](#mcp)) |

`session-state/<uuid>/` is created at session start (`runtime.node@83194924`
`Workspace initialized`); `files/` is "Persistent storage for session
artifacts" (`runtime.node@82945561`); `session.db` is the per-session SQL
database (`cli-main.js@149653`
`this.join(this.sessionStatePath,"session.db")`).

A session the Copilot Desktop app pre-created and never prompted holds only `workspace.yaml`
(`client_name` `github/autopilot`, `user_named` false), `.workspace-fork.lock`, an empty
`checkpoints/index.md` table and empty `files/` and `research/`, with no `events.jsonl` or
`session.db` (seven such sessions on Windows, bundled 1.0.99-2, 2026-10-17).

#### What survives a delete

`/session delete <id>` (with `--remote`, the synced copy on the account as well; for a synced
session the CLI asks "has been synced to your account. Would you also like to delete the synced
session data") removes the session directory whole: `events.jsonl`, `workspace.yaml` with the
`mc_` keys, both `rewind-file-snapshots` files, `checkpoints/`, `files/` and `research/`, and the
id from the sidebar file, and leaves `open-sessions-state.json` `{}` (Windows winget install,
1.0.98, 2026-10-17; [Observed](#observed)). A session deleted from the GitHub Copilot Desktop
app is a `sessions.delete` call to the CLI the app bundles, which removes the same
directory; a `/rewind` prunes the discarded turns' file snapshots.

What remains: the `sessions`, `turns`, `session_files`, `search_index`
and `assistant_usage_events` rows of `session-store.db`, for which the
bundle holds no `DELETE` ([`session-store.db`](#session-storedb)); the session's 0-byte
`.session-operation-locks/<id>.lock`; the slash commands and prompts in
`command-history-state.json`, newest first, at most 50 by default, with no time, session or
directory (after the winget round's delete, 10 entries in 450 bytes, the seven slash commands among
them); the process logs, which record nothing about the delete; `permissions-config.json`; and the
files the session wrote. With the remote copy deleted too, no local file says the session was ever
synced, since the `mc_` keys lived only in `workspace.yaml`. What is gone with `events.jsonl` and
kept nowhere else: the tool calls and their permission decisions, a denial, the `session.info`
output of a `/model` switch, the resume, and every event of the prompts the store did not keep.

Removing the project directory (`rm -r secret/` on Linux, 1.0.98, 2026-10-16) removes nothing
under `~/.copilot`: the session's `events.jsonl`, its `session-store.db` rows,
`rewind-file-snapshots/index.json` (which still names the file under the removed directory),
its `command-history-state.json` entries and its sidebar file all survive, and together hold the
whole session.

The store's rows that outlived a delete on the real installs:

- Copilot Desktop, Linux install of 2026-10-16: two deleted app sessions kept their `sessions`,
  `turns`, `session_files`, `assistant_usage_events` and `search_index` rows.
- Copilot Desktop, Windows install of 2026-10-17: the deleted general chat kept its `sessions`
  row, 3 `turns`, 3 `session_files`, 3 `search_index` and 8 `assistant_usage_events` rows, and
  the WAL holding them was not checkpointed when the app quit
  ([What survives a delete in `copilot-desktop.md`](copilot-desktop.md#what-survives-a-delete)).
- The standalone CLI's own command, `/session delete <id> --yes --remote`, locally and from the
  account (Windows winget install, 1.0.98, 2026-10-17): the session kept its `sessions` row (`cwd`
  set, `repository` null, a 32-character `summary`), 3 `turns` (prompts of 63, 18 and 42
  characters, replies of 4,346, 1,704 and 30), 1 `session_files` row (`apply_patch`), 3
  `search_index` rows and 55 `assistant_usage_events` rows (`model` `claude-sonnet-5`,
  `api_endpoint` `/v1/messages`; the later session's `gpt-6.1-sol` rows use `ws:/responses`), with
  `session_refs`, `checkpoints` and `forge_trajectory_events` empty; the WAL's size and time were
  unchanged since 22:06, before the delete. Its `events.jsonl` had held 6 `user.message` lines,
  and the store kept only 3 turns, the last three (stamped 22:03:42, 22:04:58 and 22:05:54); why
  the first three are missing is not determined.

The analyzer reads `session-store.db`, `command-history-state.json` and
`rewind-file-snapshots/index.json` as secondary sources (see
[Secondary sources](../analyzer/docs/output.md#secondary-sources)), so a session whose directory
was deleted still yields its store rows; of the process logs it reads the sign-in line only, and
`search_index` and `assistant_usage_events` give no rows ([Parser plan](#parser-plan)).

### Record schema

#### Envelope

Every line is `{id, parentId, timestamp, type, data}` (observed, 284 of 284 on macOS and every line
on Linux; `schema@1133`). `id` is a UUID v4; `timestamp` is ISO 8601 UTC with milliseconds and `Z`;
`type` names the event; `data` is its payload. Two optional envelope keys are in the schema and were
not observed: `agentId`, set on events from a subagent ("Absent for events from the root/main
agent", `schema@1503`), and `ephemeral`, which marks events that are "not persisted to the session
event log on disk" (`schema@1331`): 83 of the 159 types are ephemeral (streaming deltas,
`session.idle`, `session.title_changed`, `assistant.usage`, `session.snapshot_rewind`, the
`model.call_*` family) and never reach the file. A parser skips unknown types.

`parentId` is "the chronologically preceding event in the session ... Null
for the first event" (`schema@1133`). Observed: the previous line on 258
lines, `null` on `session.start` and on `model.turn_started`, and on 3 of
9 `model.model_call_started` an id absent from the file. The dangling ids
are ephemeral events, which take a place in the chain but are not
written; the `model.*` events come from the Rust runtime's own publisher
(`runtime.node@80033248` `Failed to publish native model event`) and start
a chain of their own. A rewind truncates the file instead of branching it
([Joins and timestamps](#joins-and-timestamps)), so the chain carries no tree and a
parser ignores it.

#### Event types

Observed types with the keys a parser reads (all named in the schema
unless marked):

| `type` | `data` keys | Schema |
| --- | --- | --- |
| `session.start` | `sessionId` (= directory name), `version` (1), `producer` (`copilot-agent`), `copilotVersion`, `startTime` (session creation, not the line time), `selectedModel`, `context{cwd, gitRoot, repository, hostType, branch, headCommit}` (`cwd` alone on Linux, outside a git repository), `alreadyInUse`, `remoteSteerable`, `contextTier` | `@801015` `StartData`; `@987057` `WorkingDirectoryContext` |
| `session.model_change` | `previousModel`, `newModel`, `cause` (`initial_resolution` observed; `rate_limit_auto_switch` per the schema), `source` (`automatic` observed; `model_picker` observed on Linux for a typed `/model` choosing Auto, with `previousModel`, `newModel` `auto`, `previousReasoningEffort` `medium`, `reasoningEffort` and `contextTier` null; `startup` observed through Copilot Desktop's bundled 1.0.99-2 on Windows at the first prompt; also `model_command`, `settings_command`, `config_command`, `agent`, `plan_mode`, `managed_settings`, `repo_settings`, `sdk`, ...), `reasoningEffort`, `previousReasoningEffort`, `providerId` | `@460110` `ModelChangeData`, `@462830` `ModelChangeSource` |
| `session.info` | `infoType` ("Category of informational message": `model`, `fork`, `remote`, `tip`, `warning`, `managed_plugins`, `managed_plugins_complete`, `relay_attachments_dropped`, `canvas`, `hook_progress_transient` in `cli-main.js`, offset not recorded), `message` ("Human-readable informational message for display in the timeline"), `url` and `tip` (optional). Emitted persisted (`this.emit("session.info", ...)`) and ephemeral (`emitEphemeral`) in `cli-main.js` of the 1.0.98 linux-arm64 package (offset not recorded). Observed on Linux right after the `model_picker` change: `infoType` `model`, `Model changed from GitHub Copilot • GPT-6.1 Sol (medium) to GitHub Copilot • Auto for this session` and `Auto routing profile change to Balance requested.` | `schema` `InfoEvent` / `InfoData` of the 1.0.98 linux-arm64 package, offset not recorded ("Informational message for timeline display with categorization"; `additionalProperties: false`) |
| `system.message` | `content` (the full system prompt, 36 KB observed), `role` (`system`, `developer`), `interactionId`, `metadata.variables` (`gitCommitTrailer{clientName, coauthorEnabled, sessionId}` real install and `runtime.node@81992124`) | `@829319` |
| `user.message` | `content` (the prompt as typed), `transformedContent` (the prompt with injected context), `messageId`, `interactionId`, `turnId`, `delivery` (`idle`, `steering`, `queued`), `agentMode` (`interactive`, `plan`, `autopilot`, `shell`), `source` (absent or `user` for typed input; `skill-<name>`, `agent-<id>`, `command-`, `schedule-`, `autopilot-` prefixes), `isAutopilotContinuation`, `attachments`, `responsesReasoning` | `@959453` |
| `assistant.turn_start` / `assistant.turn_end` | `turnId` ("typically a stringified turn number"), `interactionId`, `model`, `parentToolCallId` | `@63598` |
| `assistant.message` | `messageId`, `originatingMessageId` (= the prompt's `messageId`), `content` (empty when the message only calls tools), `model`, `providerId`, `turnId`, `phase` (`final_answer` observed), `reasoningText` (readable reasoning summary), `reasoningOpaque` and `encryptedContent` (opaque provider blobs), `reasoningBlocks{provider, blocks[{type, id, summary[{type, text}], encrypted_content}]}`, `toolRequests[]`, `apiCallId`, `serverTools`, `rte`, `chunkIndex`/`chunkCount`, `parentToolCallId` | `@29947` |
| `toolRequests[]` | `toolCallId` (`call_...`; `custom_...` for grammar tools such as `apply_patch`), `name`, `arguments` (object, or a string for `type` `custom`), `type` (`function`, `custom`), `toolTitle`, `intentionSummary`, `mcpServerName`, `mcpToolName` | `@41785` |
| `tool.execution_start` | `toolCallId`, `toolName`, `toolTitle`, `arguments`, `model`, `turnId`, `mcpServerName`, `mcpToolName`, `parentToolCallId` | `@899827` |
| `tool.execution_complete` | `toolCallId`, `success`, `result{content, detailedContent, structuredContent, contents}`, `error{code, message}`, `shellExecution{exitCode}`, `isUserRequested`, `toolTelemetry`, `model`, `turnId`, `interactionId`, `rte` | `@873844` |
| `permission.requested` | `requestId`, `permissionRequest{kind, toolCallId, intention, ...}`, `promptRequest{kind, ...}` (the prompt shown), `permissionMode` (`manual`, `assisted`, `allow-all`), `agentMode`, `ownerSessionId`, `resolvedByHook` | `@565075`, `@561612` `PermissionRequest`, `@520733` `PermissionMode` |
| `permission.completed` | `requestId`, `toolCallId`, `decisionSource` (`human_response` observed; `assisted_approval`, `host_policy`, `unattended_fallback`, `authorization_carry_forward`), `result{kind, ...}` | `@596459` `PermissionResult`, `@504557` |
| `session.usage_record` | `usage{model, inputTokens, outputTokens, reasoningTokens, cacheReadTokens, cost, duration, finishReason, initiator, interactionType, apiEndpoint, transport, provider, isByok, isAuto, accounting{sequence, sourceSessionId, usageId}, apiCallId}` | `@81773` `AssistantUsageData` |
| `session.usage_checkpoint`, `session.shutdown` | `accountingSnapshot`, `shutdownType` (`routine`, `error`), `errorReason`, `sessionStartTime` (epoch ms), `codeChanges{filesModified, linesAdded, linesRemoved}`, `modelMetrics`, `currentModel`, `eventsFileSizeBytes` | `@765167`, `@771298` |
| `model.turn_started`, `model.model_call_started`, `model.turn_ended` | `kind`, `model`, `modelInfo{...}`, `timestampMs`, `turn` | not in the schema; `cli-main.js@1147661` |
| `model.messages_snapshot` | `kind`, `messages[{role, content}]` (the utility prompt) | not in the schema |
| `model.captured_assignment_context` | `kind`, `assignmentContext` (string) | not in the schema |
| `model.message`, `model.response` | `callId`, `kind`, `message{role, content, apiCallId, outputTokens, refusal}` / `response{...}`, `modelCall{model, api_endpoint, initiator, transport, request_id, ...}`, `turn` | not in the schema |
| `model.model_call_success` | `callId`, `modelCall`, `copilotUsage{token_details, total_nano_aiu}`, `quotaSnapshots`, `provider`, `modelCallDurationMs`; on the Windows winget install (1.0.98) also `turn`, `ttftMs`, `outputTtftMs`, `interTokenLatencyMs` and `modelCall{model, api_id, api_endpoint, request_id, client_request_id, service...}` | not in the schema |
| `subagent.configured`, `subagent.selected`, `subagent.started`, `subagent.completed` | keys not recorded; 2 of each on the Windows winget install (1.0.98), from a session's web-reading task, seen on no other install | `subagent.started` and `.completed` are in the schema (list below); the other two not checked |

Glosses: CAPI is the Copilot API, the GitHub endpoint that serves the
models; `interactionId` is its id for one prompt's run. `rte` is the
"treatment/eligibility signal returned by the Copilot API in the
`X-GitHub-Copilot-Request-TE` response header" (`schema@29947`). A nano AIU
is a billing unit ("nano-AI units cost", `ShutdownData`).

On the Windows winget install (1.0.98) `session.start` carried `contextTier`, `context{cwd}`,
`alreadyInUse` and `remoteSteerable` beside the common keys, and `system.message`
`metadata{variables{gitCommitTrailer, sessionId, clientName, coauthorEnabled}}`. Under Copilot
Desktop's allow-all mode (bundled 1.0.99-2, Windows) no `permission.requested` or
`permission.completed` was written. There the general chat's `session.shutdown` was written ten
minutes after its last reply (20:13:47); the Linux app sessions wrote none.

`permissionRequest.kind` values: `shell` (`fullCommandText`, `commands[{identifier, readOnly}]` plus
`allowMatchIdentifier` (real install and `runtime.node@87055448`), `commandSegments`,
`possiblePaths`, `possibleUrls`, `hasWriteFileRedirection`, `canOfferSessionApproval`, `shellType`
(real install and `runtime.node@87056164`)), `write` (`fileName`, `diff`, `newFileContents`, and
`fileEdits` and `canOfferSessionApproval` on the Linux install), `read` (`path`), `url` (`url`, and
`requestSandboxBypass` and `requestSandboxBypassReason` on the Linux install), `mcp` (`serverName`,
`toolName`, `args`), `memory`, `custom-tool`, `hook`, `workflow`, `extension-management`,
`extension-permission-access`, `extension-env-access` (`schema@583572`, `@594037`, `@587312`).
`promptRequest.kind` adds `commands` (in place of `shell`) and `path`. On the Windows winget
install (1.0.98) the 79 requests had four key sets: 72 of the `url` form (`kind`, `toolCallId`,
`intention`, `url`, `requestSandboxBypass`, `requestSandboxBypassReason`, then `promptRequest`;
the web fetches), 2 of the shell form (`fullCommandText`, `shellType`, `intention`, `commands`,
with `identifier` and `readOnly`, ...), 2 of the write form (`fileName`, `diff`,
`newFileContents`, `canOfferSessionApproval`, `fileEdits`) and 3 with a key beginning `redirecte`
(the rest of its name was cut in the listing; not determined).

`result.kind` values (`schema@596459`): `approved`;
`approved-for-session` and `approved-for-location` (with `approval{kind,
commandIdentifiers | serverName, toolName | toolName | operation |
extensionName}`, and `locationKey`, the git root or cwd, for the location
form, `@485843`); `approved-read-only-for-session` (`directories`);
`cancelled` (`reason`); `denied-by-rules` (`rules[{kind, argument}]`);
`denied-no-approval-rule-and-could-not-request-from-user`;
`denied-interactively-by-user` (`feedback`, `forceReject`);
`denied-by-content-exclusion-policy` (`path`, `message`);
`denied-by-permission-request-hook` (`message`, `interrupt`). Only the
first and the location form were observed; on Linux the location form
carried `approval{kind: write}` and the project path as `locationKey`. The `result.kind` of the
winget round's denial was not recorded. That round's listing showed `permission.completed` as
`{requestId, toolCallId, result{kind, decisionSource}, agentId}`, `agentId` on some records only,
with `decisionSource` inside `result` where the Linux records and the parser have it beside
`result`; whether that is the shape or the listing's nesting is not determined.

The eight `model.*` types are the runtime's native model events, listed in
`cli-main.js@1147861` (`captured_assignment_context:!0,...,turn_started:!0`)
and kept out of the CLI's own timeline (`cli-main.js@1158330`
`if(!($M(c)||Ree(c)))return t(c)`). Each set is one call to a utility
model, not the session model. Observed: three sets per session, one to
`gpt-4o-mini` and two to `gpt-5.4-nano` or `gpt-5.6-luna-utility`. The
first is the session-name generator: the CLI calls
`modelGenerateSessionName` with the first prompt, stores the answer as the
session's name and logs it (`cli-main.js@7310874`
``await e.name.setAuto({summary:O.value}),s.info(`Session named: "${O.value}"`)``;
prompt `runtime.node@90556512` `Generate a session title for this
message:`). The other two are the task-analytics classifier, a telemetry
feature that labels each turn (`runtime.node@83390256`
`judgeModelPreferredgpt-5.6-luna-utility`, `@83388829` `gpt-5.4-nano`,
`@83390335` `copilot_cli_task_analytics`). `assignmentContext` is the ExP
assignment string (the string
names the experiment treatments the client is in, `runtime.node@79937841`
`ExP assignment response AssignmentContext must be a string`).

Schema types a parser should handle although none was observed: `session.resume` (`resumeTime`,
`selectedModel`, `context`), `session.error` (`errorType`, `message`, `statusCode`), `abort`
(`reason`: `user_initiated`, `remote_command`, `user_abort`, `autopilot_credit_limit`),
`session.compaction_complete` (`summaryContent`, `checkpointNumber`, `checkpointPath`),
`session.context_changed` (`WorkingDirectoryContext`), `session.permissions_changed`
(`previousMode`, `mode`), `session.mode_changed` (`previousMode`, `newMode`),
`session.auto_mode_resolved` (`chosenModel`, `routingMethod`; Auto is the model setting that lets
the service choose), `session.task_complete` (`summary`), `session.handoff`,
`session.schedule_created` (`prompt`, `cron`, `at`), `system.notification` (`content`, `kind`),
`assistant.reasoning` (`reasoningId`, `content`), `tool.user_requested` (`toolCallId`, `toolName`,
`arguments`; a tool the user ran), `permission.carriedForward`, `human_response.recorded`,
`subagent.failed` (`toolCallId`, `agentName`, `model`, the keys the schema gives `.started` and
`.completed` too, which were observed on Windows), `skill.invoked` (`name`, `path`), `hook.start` /
`hook.end`. The hook pairs were observed on Windows through Copilot Desktop's bundled 1.0.99-2, with
`hookType` `sessionStart`, `userPromptSubmitted`, `userPromptTransformed`, `preToolUse`,
`postToolUse`, `postToolUseFailure` (not seen on Linux), `agentStop` and `sessionEnd`.

### Joins and timestamps

- Session: `session.start.data.sessionId` = the directory name = `turns`,
  `sessions.id`, `sidebar-sessions-state` `sessionIds[]` and
  `workspace.yaml` `id`.
- Tool call: `toolRequests[].toolCallId` = `tool.execution_start` and
  `tool.execution_complete` `toolCallId` = `permissionRequest.toolCallId`
  = `permission.completed.toolCallId`. `requestId` joins the two
  permission events.
- Prompt to reply: `user.message.messageId` =
  `assistant.message.originatingMessageId`; `interactionId` groups the
  prompt, its turns and its tools. `turnId` counts agent-loop turns
  (22 observed for 7 prompts) and is not `turns.turn_index`, which counts
  prompts.
- Utility call: `model.*` `callId` joins message, response and success.
- Rewind: a snapshot's `eventId` is the id of the `user.message` it
  precedes (real install, equality not checked). `/rewind` truncates
  `events.jsonl` at that event ("Truncates persisted session history to a
  specific event", `runtime.node@83147234`; restore and truncation are
  "Not crash-atomic", `@83147760`), so discarded turns are gone from the
  log and survive only in the stores of
  [Other stores](#other-stores).
- Project: `session.start.data.context.cwd`, updated by
  `session.context_changed` and `session.resume`; `workspace.yaml` `cwd`;
  `sessions.cwd`; `sidebar-sessions-state` `cwd`, whose file name is the
  sha256 of the cwd (real install, verified).

On the Windows winget install (1.0.98) one session held 12 of each
`model.*` type beside its 55 `assistant.message`, so that session made 12
utility calls, not the three per session seen on macOS; what each was for
was not determined.

Every timestamp in the log is UTC with `Z`. `session.shutdown.sessionStartTime`
is epoch milliseconds; `timestampMs` on native model events likewise.

### Other stores

#### `session-store.db`

`session-store.db` runs in WAL mode (`runtime.node@80402520` `PRAGMA journal_mode = WAL`); on the
macOS install the data was in the 1.7 MB WAL and the main file was 4 KB (1.37 MB and 4096 bytes on
Linux, 609,792 then 704,552 bytes beside 4096 on Windows through Copilot Desktop),
so the WAL must be read. `schema_version` 38 observed on macOS and 8 on Linux, both on 1.0.98, and 8
on Windows with the Copilot Desktop app's bundled 1.0.99-2 ([Format versions](#format-versions)).
The runtime writes it as each turn ends and rebuilds it from `events.jsonl` with
`/chronicle reindex` ("chronicle" is the CLI's name for its session-history commands: `standup`,
`reindex`, `skills`, `tips`; `runtime.node@80541016` `src/runtime/src/session/store_reindex.rs`).
Column meanings come from the runtime's own description for its SQL tool (`runtime.node@80212161`):

| Table | Written by | Meaning |
| --- | --- | --- |
| `sessions(id, cwd, repository, host_type, branch, summary, created_at, updated_at)` | `@80389716` `INSERT INTO sessions ... ON CONFLICT(id) DO UPDATE SET cwd = COALESCE(excluded.cwd, cwd)` | one row per session; `summary` a 27 to 39 character title (observed) |
| `turns(session_id, turn_index, user_message, assistant_response, timestamp)` | `@80389302` `INSERT INTO turns ... ON CONFLICT(session_id, turn_index) DO UPDATE SET user_message = COALESCE(...)`; next index from `SELECT MAX(turn_index)` | one row per prompt: the typed prompt and the final reply |
| `session_files(session_id, file_path, tool_name, turn_index, first_seen_at)` | `@80389148` `INSERT OR IGNORE INTO session_files` | files changed, `tool_name` `edit`, `create` or `apply_patch` |
| `session_refs(session_id, ref_type, ref_value, turn_index, created_at)` | `@80388999` `INSERT OR IGNORE INTO session_refs` | `ref_type` `commit`, `pr` or `issue` (`@80212254`) |
| `checkpoints(session_id, checkpoint_number, title, overview, history, work_done, technical_details, important_files, next_steps, created_at)` | `@80390542` | compaction checkpoints |
| `search_index` (FTS5, SQLite's full-text index) | `@80390397` | `source_type` `turn`, `checkpoint_*` or `workspace_artifact`; `source_id` `<session>:turn:<n>`; `content` = prompt, newline, reply (observed) |
| `assistant_usage_events` | `@80401153` | one row per model call: model, tokens, `total_nano_aiu`, `initiator`, `finish_reason`, `token_details_json`; on Linux 28 rows with `model` `gpt-6.1-sol`, `copilot_usage_model` null, `initiator` `agent` or `user`, `api_endpoint` `ws:/responses` or `/responses`, `reasoning_effort` `medium`, `finish_reason` `stop` or `tool_calls` |
| `forge_trajectory_events(session_id, tool_call_id, turn_index, event_type, command, output, exit_code, event_key, event_value, created_at)` | `@80402086` | tool commands and outputs collected for Forge, the feature that proposes skills from repeated workflows (`/chronicle skills create`); `event_type` values not found as literals |
| `forge_skill_proposals`, `dynamic_context_items` | | skill proposals and per-repository context; no session content (both present and empty on Linux, beside `search_index` and its five FTS5 shadow tables) |

Observed: `turns` 9 rows on macOS, `user_message` equal to a
`user.message` `content` and `assistant_response` equal to an
`assistant.message` `content` or null; `timestamp` 4 to 293 seconds after
the matching `user.message` line. On one macOS session the first two
prompts appear again at `turn_index` 2 and 3 after a turn whose response
is null. The Linux round explains the repeat from the timestamps (6
`turns` rows; prompt lengths 111 and 114 characters in session 1, 90 in
session 2). Session 1 has `(0, p1, r1)` at 21:05:34.927Z, `(1, p2, null)`
at 21:08:11.565Z, `(2, p1, r1)` at 21:08:12.119Z and `(3, p2, r2)` at
21:16:01.831Z; session 2 has `(0, p1, null)` at 21:18:16.638Z and `(1, p1,
r1)` at 21:19:27.956Z. So the complete row of a turn is written when the
next prompt arrives (21:05:34.927Z is 4 ms after prompt 2) or at shutdown
(21:16:01.831Z, 21:19:27.956Z), under `MAX(turn_index) + 1`; a
prompt-only row with a null response is written mid-turn, 58 to 157
seconds after the prompt, during a model call and aligned with no
transcript event; and the row written on the next prompt repeats the
previous turn under a new index when a prompt-only row sits between
them. No delete, rewind or reindex was involved. The parser's secondary
deduplication drops both kinds of copy (17 rows on the Linux collection).
The Windows install (bundled 1.0.99-2) wrote no prompt-only row: one
complete row per prompt, 3 for the general chat and 1 for the project
session, the chat's last stamped at its shutdown, each with its reply.
The string `duplicate_turn` sits beside the reindex messages
(`runtime.node@80355318`); a reindex, which derives turns from the log and
merges with `COALESCE`, was not run. Other Linux rows: `sessions` 2
(`cwd` set; `repository`, `host_type` and `branch` null, since neither
directory was a git repository; `summary` the generated session name),
`session_files` 13 (`tool_name` `apply_patch`), `session_refs`,
`checkpoints` and `forge_trajectory_events` 0, `search_index` 6
(`source_type` `turn`, `source_id` `<session>:turn:<n>`, `content` the
prompt, a newline and the reply, or the prompt alone when the reply is
null). The bundle holds no `DELETE` for `sessions`,
`turns`, `session_files`, `session_refs` or `checkpoints` (only for
`search_index` rows it rewrites and `dynamic_context_items`), so these
rows outlive a rewind and a deleted session directory; what deletes left on the installs is in
[What survives a delete](#what-survives-a-delete).

#### `command-history-state.json`

`command-history-state.json` `{commandHistory: [...]}`: prompts, newest
first, a repeat moved to the front, slash commands included (the
Linux file held 4 entries, `/model` newest, then the three prompts), at
most `commandHistoryMaxSize` (50
by default, 1 to 1000) (`cli-main.js@1109188`
`this.commandHistory.unshift(t),this.commandHistory=this.commandHistory.slice(0,this.maxSize)`;
`@1108158` `yNt=50,Dze=1e3`). No time, session or cwd. The docs name a
`command-history-state/` directory instead; 1.0.98 wrote the file.

#### `workspace.yaml`

`workspace.yaml` (real install key order): `id`, `cwd`, `client_name`,
`name` (the generated or `/rename` name), `user_named`, `summary_count`
(compactions), `fork_count`, `created_at`, `updated_at`,
`remote_steerable`, `mc_task_id`, `mc_session_id`, `mc_last_event_id`;
on Linux `client_name` was `github/cli` on both sessions, `user_named`
false, `summary_count` and `fork_count` 0, `remote_steerable` false. On
the Windows winget install (1.0.98) the file was written at session
start without `name` and the last four keys (212 bytes), gained `name`
at the first prompt and `remote_steerable` and the `mc_` keys at a
resume, with `updated_at` unchanged; the `mc_` keys are the sync marker
of [Sync, relay and origin](#sync-relay-and-origin-rows).
The struct has 19 fields (`runtime.node@82945317` `struct WorkspaceMetadata
with 19 elements`); `repository`, `host_type`, `branch`,
`mc_environment_id` and `chronicle_sync_dismissed` are among the rest
(`runtime.node@87588572`). The `mc_` keys are Mission Control ids
(`runtime.node@79041460` `enroll published application sessions in Mission Control`); every
Copilot Desktop session carries them. They live only in `workspace.yaml`
([What survives a delete](#what-survives-a-delete)).

#### `rewind-file-snapshots/index.json`

`rewind-file-snapshots/index.json` `{schema, snapshots[{snapshotId,
eventId, timestamp, userMessage, fileCount, linesAdded, linesRemoved,
backupFiles, files{<hex>: {path, linesAdded, linesRemoved, preimage,
postimage}}}]}`, with `preimage{kind}` and `postimage{kind,
contentHash (sha256 hex), encodedPath{data, encoding}, resolvedPath}`
(real installs; the removed `~/secret` project's snapshot still names
its file under `/home/sean/secret`; `runtime.node@82575983`
`rewind-file-snapshotstracking.json`). A rewind prunes the discarded
turns' snapshots (`runtime.node@82545128` `Failed to prune a
withdrawn turn's file snapshot`).

#### `permissions-config.json`

`permissions-config.json` `{locations: {<abs path>: {tool_approvals:
[{kind, ...}], allowed_directories: [...]}}}`; `kind` `commands`
(`commandIdentifiers`), `read`, `write`, `mcp` (`serverName`,
`toolName`), `mcp-sampling`, `memory`, `custom-tool`,
`extension-management`, `workflow`, `extension-permission-access`,
`extension-env-access` (docs; `runtime.node@82849044`; `runtime.node@82849229`
`tool_approvalsallowed_directories`). Keyed by git root or cwd (docs). Observed two locations with
`write` entries on macOS, one, `~/demo`, with one `write` entry and no `allowed_directories` key
on Linux, and on Windows (winget, 1.0.98) one `write` entry for `demo` (142 bytes) written by an
"allow always"; the denial in the same step wrote no standing rule.

#### `settings.json`

`settings.json`: `allowedUrls` and `deniedUrls`, "URLs or domains allowed
without prompting" (`runtime.node@79085397`); observed four
`allowedUrls` on macOS; not written on the Linux install. JSON with
comments (docs).

#### Process logs

`logs/process-*.log`: `<ISO ms Z> [LEVEL] <message>`. `Signed in to
<host> as <login>.` (`cli-main.js@7556306`) names the account; `Session
named: "<name>"`, `Starting Copilot CLI: <version>`
(`cli-main.js@8163060`), `Workspace initialized: <uuid>`. Also seen on
Linux (ids redacted): `Node.js version: v<n>`, `Login status unknown`,
`Workspace initialized: <uuid> (checkpoints: 0)`, `Registering
foreground session: <uuid>` and `Unregistering ...`, `Remote session
access enabled`, `Session indexing debug: SESSION_INDEXING=true,
repository=undefined`, `[rust:model_bindings::api_resolver] Using
default model: gpt-6.1-sol`, `[managedSettings] device MDM: no policy
present on this device` (MDM is mobile device management, an
organisation's policy push), `[managedSettings] self-fetch starting for
account https://github.com/<login>`, `[rust:rmcp::service] Service
initialized as client` for the `github-mcp-server` remote MCP server,
`Memory enablement check: enabled` and `Starting graceful shutdown of
CLI server`. The login appears twice, in the sign-in line and the
managed-settings line. On the Windows winget install (1.0.98) also
`Remote session access disabled` (before the login) and `enabled` (a
resumed process), `Unregistering foreground session: <uuid>`, `Closing
session <uuid>`, `Closed interrupted sub-agent records on resume
{"closed":0,"dangling":0}`, `Destroying 0 active sessions` and
`[rust:rmcp::service] Service initialized as client` with no MCP server
configured, and in the resumed process `Using default model: gpt-6.1-sol`; no line records a
session deleted with `/session delete`, and the sync a resume made is logged nowhere.

A log rotates at 10 MB to `process-<epoch ms>-<pid>.<n>.log`. On the Windows winget install
(1.0.98) a `/login` left pending looped: while the device code waited the CLI wrote about 280,000
lines in 74 seconds, four rotations (`process-1792273608752-2868.1.log` to `.4.log`, 10,472,282 to
10,483,404 bytes and 65,093 to 65,615 lines each, seven message kinds) and a live log of 3,068,006
bytes at the login, about 45 MB in all. Each rotated file holds about 9,700 each of `[WARNING]
[rust:copilot_runtime::auth::gh]` lines saying the CLI failed to run `gh auth token`
(`"error":"program path has no file name"`; there was no `gh` on the VM), three
`[managedSettings]` lines (`effective policy resolved: source=none`, `device MDM: no policy
present on this device`, `server policy fetch skipped: no authenticated GitHub host available`),
`[INFO] Remote session access disabled` and `[rust:lsp::session_service] Resolved LSP telemetry
consent` with `"telemetry_level":"off","source":"auth_change"` (LSP is the Language Server
Protocol), and about 7,500 `[INFO] [rust:otel] OTel session policy applied` (OTel is
OpenTelemetry, the telemetry framework): a pending login re-resolves the auth state in a tight loop.
After the login the live log grew by 3 KB in fifteen minutes; the Linux and macOS 1.0.98 logs were
105 bytes to 47 KB.

Through Copilot Desktop's bundled 1.0.99-2 on Windows the logs also hold `MCP server connection
failed {"server_name":"<name>","raw_error":"failed to spawn MCP server process: program not
found"}` for a configured server whose command is not installed, and `Best-effort plugin
preparation skipped 1 item.`
The parser's pattern (`process-\d+-\d+\.log`, [Parser plan](#parser-plan))
does not select the rotated files, so a sign-in line that rotated out
would not be read ([issue 152](https://github.com/seanthegeek/doubleagent/issues/152));
on that install every sign-in line was in an unrotated log.

#### `vscode.metadata.json`

`session-state/<id>/vscode.metadata.json` is written by VS Code, not by the CLI, for a session
VS Code's Copilot Chat extension runs in a git worktree
([Scratch directories](#scratch-directories)).
Its entries carry `workspaceFolder`, `worktreeProperties` and `additionalWorkspaces`
([`chatSessionMetadataStoreImpl.ts:170-186`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/extensions/copilot/src/extension/chatSessions/copilotcli/vscode-node/chatSessionMetadataStoreImpl.ts#L170-L186),
microsoft/vscode at
[`4d22f4f`](https://github.com/microsoft/vscode/commit/4d22f4f922eb012217ac65672ccce02f59d63706));
not seen on a real install. The VS Code side is in [`vscode.md`](vscode.md).

### Format versions

`session.start.data.version` 1 and `producer` `copilot-agent` on every
observed log. Several fields are "Absent on historical events"
(`permissionMode`, `ownerSessionId`) or absent "on completions recorded
before this field existed" (`decisionSource`, `schema@504557`), so older
logs lack them. Earlier builds wrote a flat `session-state/<session
id>.jsonl`; 1.0.98 refuses to resume one (`cli-main.js@1698895` `uses a
legacy flat JSONL transcript format that is no longer supported`;
`cli-main.js@1654639` `Rwr=".jsonl"`) and the reindex skips it. Its
record shape is not in this bundle. The schema allows no extra keys
(`additionalProperties: false`), yet the install carried `shellType` and
`allowMatchIdentifier`, so the file is a superset of the published
schema. `session-store.db` migrates through `schema_version`: 38 on
the macOS install and 8 on the Linux install, both 1.0.98 and both with
the tables of [Other stores](#other-stores); neither number is
explained (the Windows install's bundled 1.0.99-2 also had 8);
`rewind-file-snapshots` and `sidebar-sessions-state` carry
their own `schema` and `schemaVersion` numbers.

### Secrets

`system.message.content` is the full system prompt with the user's custom
instructions; `transformedContent` repeats each prompt with injected
context. Tool arguments and results are verbatim (shell output, fetched
pages), and a `write` permission request carries the `diff` and
`newFileContents` of the file. `encryptedContent`, `reasoningOpaque` and
`reasoningBlocks[].encrypted_content` are opaque provider blobs
("Session-bound and stripped on resume", `schema@29947`); `apiCallId` (500
characters observed) is the provider's response id, and `usageId`,
`request_id` and `service_request_id` are request ids, none a credential.
`model_call_success.quotaSnapshots` records the account's plan
entitlements. The log's sign-in line names the GitHub login. No token was
seen in a session file; where the token is kept, and the `mcpServers.<name>.env` values of
`mcp-config.json` that can hold one in plain text, are in [Credentials](#credentials). The parser
reads neither `config.json` nor `mcp-config.json`.

### Parser plan

`wants()` by `artifact.rel`, with `(?:\.copilot|\.local/state/\.copilot|\.config/\.copilot)`
written `<home>`:

```text
^<home>/session-state/[0-9a-f-]{36}/events\.jsonl$
^<home>/session-state/[0-9a-f-]{36}/workspace\.yaml$
^<home>/session-state/[0-9a-f-]{36}/rewind-file-snapshots/index\.json$
^<home>/session-store\.db$
^<home>/command-history-state\.json$
^<home>/permissions-config\.json$
^<home>/settings\.json$
^<home>/sidebar-sessions-state/[0-9a-f]{64}\.json$
^<home>/logs/process-\d+-\d+\.log$
```

Rows follow the log order, timestamped by the line's `timestamp`.

| Record | `turn_type` and text |
| --- | --- |
| `session.start` | `system`, `session start: copilot <copilotVersion> producer=<producer> cwd=<context.cwd>`; fills `session_id`, `project_path`, `git_branch` (`context.branch`) |
| `session.resume` | `system`, `session resumed`; updates cwd and branch from `context` |
| `session.context_changed` | no row; updates `project_path` and `git_branch` |
| `system.message` | `system`, `system prompt: <content>` (`role` `developer`: `developer message:`), never shortened |
| `user.message`, no `agentId`, `source` absent or `user`, not `isAutopilotContinuation` | `user`, `content` |
| `user.message`, other | `system`, `<label>: <content>`: `auto-continuation` for `isAutopilotContinuation`, `subagent task` with `agentId`, otherwise the `source` value |
| `assistant.message` | `assistant` with `content` when non-empty; then `thinking` with `reasoningText` (only with `include_thinking`; else the summaries joined from `reasoningBlocks.blocks[].summary[].text`); then one `tool_use` per `toolRequests[]` |
| `assistant.reasoning` | `thinking`, `content` (only with `include_thinking`) |
| `tool.execution_start` | no row when an earlier `toolRequests[]` had the `toolCallId`; otherwise `tool_use` (a user-requested or subagent call) |
| `tool.user_requested` | `tool_use`, prefixed `user requested: ` |
| `tool.execution_complete` | `tool_result`: the longer of `result.detailedContent` and `result.content`, `detailedContent` on a tie (it is the complete text, but a successful skill invocation puts the concise text there: one observed record had 25 characters in `detailedContent` beside 8091 in `content`); prefixed `[exit N] ` when `shellExecution.exitCode` is non-zero; `error.message` appended on a new line when `success` is false |
| `permission.requested` | `system`, `permission request: <permissionRequest.kind> <fullCommandText, url, fileName, path or serverName/toolName> intention=<intention>`, `tool_use_id` `permissionRequest.toolCallId`; first `permissionMode` and `agentMode` give `permission mode: <permissionMode> agent=<agentMode>`, a change gives `permission mode: <old> -> <new>` |
| `permission.completed` | `system`, `permission decision: <result.kind> source=<decisionSource or ->`, then ` location=<locationKey>`, ` approval=<compact JSON of approval>`, ` directories=...`, ` feedback=<feedback>`, ` rules=<compact JSON>`, ` reason=<reason>`, ` path=<path>`, ` message=<message>` when present; `tool_use_id` `toolCallId` |
| `permission.carriedForward` | `system`, `permission decision: carried-forward source=<decisionSource>` |
| `session.permissions_changed` / `session.mode_changed` | `system`, `permission mode: <previousMode> -> <mode>` / `agent mode: <previousMode> -> <newMode>`; the Copilot Desktop app's form (`previousAllowAllPermissions`, `allowAllPermissions`, `previousAllowAllPermissionMode`, `allowAllPermissionMode`, no `mode` and no `previousMode`; real installs, Linux, bundled CLI 1.0.95-3, and Windows, 1.0.99-2) is `permission mode: allow-all <previousAllowAllPermissionMode or -> -> <allowAllPermissionMode or ->`, and the tracked mode becomes `allow-all <allowAllPermissionMode>` |
| `session.model_change` | `system`, `model: <newModel>` at the first, `model: <old> -> <newModel>` on a change, kept when equal; followed by ` cause=<cause> source=<source> effort=<reasoningEffort>` for the keys present |
| `session.auto_mode_resolved` | `system`, `model: <old> -> <chosenModel> (auto, <routingMethod>)` |
| `session.info` | `system`, `session.info <infoType>: <message>`, then ` url=<url>` and ` tip=<tip>` for the keys present, at the line's `timestamp`; `model` the tracked model, `tool_name` and `tool_use_id` empty |
| `model.message` | `system`, `utility model <modelCall.model>: <message.content>`, `model` that model; never changes the session model |
| other `model.*` | skip (the prompt snapshot, the ExP string and call bookkeeping) |
| `session.compaction_complete` | `system`, `compaction summary: <summaryContent>` |
| `abort`, `session.error`, `session.warning` | `system`, `<type>: <reason or message>` |
| `session.task_complete`, `system.notification`, `session.schedule_created`, `skill.invoked`, `subagent.*`, `session.handoff` | `system`, `<type>: ` and the main text field, else compact JSON |
| `hook.start` / `hook.end` | `system`, `hook <hookType> start: <compact JSON of input, or {}>` / `hook <hookType> end: success=<true or false> output=<compact JSON of output, or {}>`, then ` error=<compact JSON>` when `error` is present; `-` for a missing `hookType` or a non-boolean `success`; `tool_use_id` `parentToolCallId` when present, `tool_name` empty; never shortened |
| `session.shutdown` | `system`, `session end: <shutdownType> <errorReason> files modified: <codeChanges.filesModified>` |
| `assistant.turn_*`, `session.usage_record`, `session.usage_checkpoint`, unknown types | skip |

Other files:

| File | Rows |
| --- | --- |
| `session-store.db` `turns` | `user` from `user_message` and `assistant` from `assistant_response` when not null; `timestamp`, `session_id`, `project_path` and `git_branch` from `sessions`; `source_line` the rowid |
| `session-store.db` `sessions` | `system`, `session name: <summary>` at `created_at` |
| `session-store.db` `session_files` / `session_refs` | `system`, `file changed: <file_path> tool=<tool_name>` at `first_seen_at` / `reference: <ref_type> <ref_value>` at `created_at` |
| `session-store.db` `checkpoints` | `system`, `checkpoint <n>: <title>` and each non-null column on its own line, at `created_at` |
| `session-store.db` `forge_trajectory_events` | `tool_use` (`command`) and `tool_result` (`output`, `[exit N]`) joined on `tool_call_id` (empty table observed; unverified) |
| `search_index`, `assistant_usage_events`, `forge_skill_proposals`, `dynamic_context_items` | none: concatenated copies, accounting, or no session |
| `command-history-state.json` | `user` per entry, empty `timestamp_utc`, `session_id` and `project_path`; `source_line` the 1-based index (newest first) |
| `rewind-file-snapshots/index.json` | `user` from `userMessage` at `timestamp`; and `system`, `rewind snapshot: <fileCount> files +<linesAdded> -<linesRemoved>: <path>, ...` |
| `workspace.yaml` | `system`, `session name: <name>` and `session metadata: user_named=<user_named> forks=<fork_count> compactions=<summary_count> client=<client_name>` at `created_at` (`client=` empty when the key is absent; `github/cli` for the CLI, `github/autopilot` for the Copilot Desktop app); `cwd` is the session's project fallback; when the session directory holds no `events.jsonl` and `cwd` is set (a session the Copilot Desktop app pre-created and never prompted, its pool and probe sessions), one more `system` row after the metadata row with the same columns, `scratch directory: <cwd> (working directory of a session the app pre-created and never prompted: no events.jsonl)`, so inventory.json lists the `cwd` under `scratch_directories`, the working directories that belong to the agent rather than to the user, not under `projects` ([issue 149](https://github.com/seanthegeek/doubleagent/issues/149)) |
| `permissions-config.json` | one `system` row per `tool_approvals` and `allowed_directories` entry: `permission setting: locations.<path>.tool_approvals=<compact JSON> (from .copilot/permissions-config.json, file mtime)`, `project_path` the location, timestamp the file's mtime, empty `session_id` |
| `settings.json` | one `system` row per `allowedUrls` and `deniedUrls` entry, `permission setting: allowedUrls=<url> (from .copilot/settings.json, file mtime)`; strip comments before parsing |
| `sidebar-sessions-state/*.json` | none; `cwd` per `sessionIds[]` fills `project_path` of database and snapshot rows that lack one |
| `logs/process-*.log` | `system`, `signed in: <host> as <login>` at the line's time; `session_id` from `Workspace initialized: <uuid>` when the file names exactly one |

Columns:

| Column | Source |
| --- | --- |
| timestamp_utc | line `timestamp`; database columns as stored |
| session_id | the directory name (= `session.start.sessionId`); `session_id` columns |
| project_path | latest `context.cwd`, else `workspace.yaml` `cwd`, else `sessions.cwd`, else `sidebar-sessions-state` |
| git_branch | latest `context.branch`, else `sessions.branch`; empty when neither is set |
| model | as in [Model](#model-rows) |
| tool_name | `toolRequests[].name` or `toolName`; on results and permission rows, the name remembered for the `toolCallId` |
| tool_use_id | `toolCallId` |
| text | coverage tables; a `tool_use` text is the `arguments` string, else `command`, `url`, `query`, `path` or `input`, else compact JSON |
| source_line | 1-based line, or array index, or rowid |

#### Permission rows

Per-call decisions: `permission.requested`, `permission.completed` and
`permission.carriedForward` give the rows of the table above, joined to the call on
`tool_use_id`, and `session.permissions_changed` and `session.mode_changed` give the mode
changes; the record shapes are in [Event types](#event-types). Standing grants: one row per
`permissions-config.json` entry and per `settings.json` `allowedUrls` and `deniedUrls` entry, at
the file's modification time ([`permissions-config.json`](#permissions-configjson),
[`settings.json`](#settingsjson)).

#### Model rows

The `model` column is `assistant.message.model` or the tool event's `model`, else the model tracked
from `session.model_change`. `session.model_change` and `session.auto_mode_resolved` give the
`model:` rows of the table above, and `model.message` a `utility model` row that never changes the
session model.

A `/model` command the user types is not a `user.message`: the log keeps
only its `session.model_change` and `session.info` lines, and the command
itself survives only as an entry of `command-history-state.json`, which
no other source repeats, so deduplication keeps it as an undated `user`
row (Linux install).

#### Past-use rows and secondary sources

`secondary()` is true for `session-store.db`, `command-history-state.json`
and `rewind-file-snapshots/index.json`. `secondary_tolerance()` is 3600
seconds for the first and the last: `turns.timestamp` is written at the end
of the turn (4 to 293 seconds after the prompt observed, and a turn with
many tools can run longer), and the snapshot time was not measured against
the prompt's, so both get the one-hour window `letta` uses.

The `sessions` row and the `workspace.yaml` row share the `session name:`
form so the database copy drops when the two agree; the metadata is a row
of its own so that the name rows can match. The `workspace.yaml` rows are
not secondary.

What a delete leaves for these sources to rebuild is in [What survives a
delete](#what-survives-a-delete).

#### Sync, relay and origin rows

The sync keys of [`workspace.yaml`](#workspaceyaml), `remote_steerable` and the three `mc_` keys,
mark a session synced to the account through Mission Control (Copilot Desktop sessions on Linux
1.1.29 and Windows 1.1.30 carry them too). When `mc_session_id` is
non-empty the parser yields one `system` row after the `session
metadata:` row, `session synced: remote session <mc_session_id> task
<mc_task_id or -> last event <mc_last_event_id or ->
remote_steerable=<remote_steerable or -> (from workspace.yaml, file
mtime)`, timestamped by the file's modification time, a key absent or
empty printed `-`; the keys can be present as `null` on a session that
is not synced, and then no row is yielded. The parser sets
`records_sync`, so the session's `sessions.jsonl` record says `synced`
`yes` when it has that row and `no` when it has none (Copilot Desktop
sessions included, since they are `copilot-cli` rows), and
`inventory.json` counts the synced sessions in `synced_sessions`, at the
top level and in each `homes[].agents[]` entry. A session deleted
locally and remotely leaves no marker anywhere on disk
([What survives a delete](#what-survives-a-delete)).

`/chronicle reindex` also uploads the sessions to the account (docs: "Reindexing also syncs
your session data to your account").

Relay and origin: not stated.

#### Scratch directory rows

A session the Copilot Desktop app pre-created and never prompted gives a `scratch directory:` row
from its `workspace.yaml` (the `workspace.yaml` row of the table above). The VS Code worktree of
[Scratch directories](#scratch-directories) sits outside every catalog line, so the analyzer's
catalog rule does not class it as a scratch directory; the parser does not read
`vscode.metadata.json`.

#### Deletion rows

The process logs record nothing about a delete, so no row marks one; what a delete leaves and what
the analyzer rebuilds from it is in [What survives a delete](#what-survives-a-delete).

#### Bad-line rows

`events.jsonl` is read with `iter_jsonl`; each bad line is its own
`system` row, `parser: unparseable line (<reason>): <line>` with the line
in full ([issue 136](https://github.com/seanthegeek/doubleagent/issues/136)), and reading continues.

A `session-store.db` that cannot be read, or whose `-wal`, `-shm` or `-journal` sidecar cannot, is
one row `parser: <file> could not be read (<reason>)`, `<file>` the name of the file that failed;
one that is not a SQLite database is `parser: not a SQLite database`.

An `events.jsonl` that cannot be read is one whole-file row
`parser: events.jsonl could not be read (<reason>)` through `error_rows`.

#### Fixtures

Sample fixture
`.copilot/session-state/5e55a0a1-0000-4000-8000-000000000001/events.jsonl`
(synthetic; start, model, system prompt, prompt, utility call, reasoning
and a `bash` request, a location approval, a failed run, an `apply_patch`
request approved once, a final reply, usage, the Linux round's `/model`
switch to Auto with its two `session.info` lines, shutdown, then a cut
line):

```json
{"id":"e0000000-0000-4000-8000-000000000001","parentId":null,"timestamp":"2026-10-01T09:00:00.000Z","type":"session.start","data":{"sessionId":"5e55a0a1-0000-4000-8000-000000000001","version":1,"producer":"copilot-agent","copilotVersion":"1.0.98","startTime":"2026-10-01T08:59:59.000Z","context":{"cwd":"/srv/proj","gitRoot":"/srv/proj","branch":"main"},"alreadyInUse":false,"remoteSteerable":false,"contextTier":null}}
{"id":"e0000000-0000-4000-8000-000000000002","parentId":"e0000000-0000-4000-8000-000000000001","timestamp":"2026-10-01T09:00:00.100Z","type":"session.model_change","data":{"newModel":"gpt-6.1-sol","cause":"initial_resolution","source":"automatic","reasoningEffort":"medium","contextTier":null}}
{"id":"e0000000-0000-4000-8000-000000000003","parentId":"e0000000-0000-4000-8000-000000000002","timestamp":"2026-10-01T09:00:01.000Z","type":"system.message","data":{"content":"You are the GitHub Copilot CLI.\nFollow the user's instructions.","role":"system","interactionId":"1a1a1a1a-0000-4000-8000-000000000001","metadata":{"variables":{"gitCommitTrailer":{"clientName":"copilot-cli","coauthorEnabled":true,"sessionId":"5e55a0a1-0000-4000-8000-000000000001"}}}}}
{"id":"e0000000-0000-4000-8000-000000000004","parentId":"e0000000-0000-4000-8000-000000000003","timestamp":"2026-10-01T09:00:01.200Z","type":"user.message","data":{"content":"fix the failing test","transformedContent":"<current_datetime>2026-10-01T09:00:01Z</current_datetime>\nfix the failing test","messageId":"2b2b2b2b-0000-4000-8000-000000000001","interactionId":"1a1a1a1a-0000-4000-8000-000000000001","parentAgentTaskId":"3c3c3c3c-0000-4000-8000-000000000001","turnId":"0","delivery":"idle","responsesReasoning":{"effort":"medium","initialEffort":"medium","model":"gpt-6.1-sol"},"supportedNativeDocumentMimeTypes":[]}}
{"id":"e0000000-0000-4000-8000-000000000005","parentId":"e0000000-0000-4000-8000-000000000004","timestamp":"2026-10-01T09:00:01.300Z","type":"assistant.turn_start","data":{"interactionId":"1a1a1a1a-0000-4000-8000-000000000001","turnId":"0"}}
{"id":"e0000000-0000-4000-8000-000000000006","parentId":null,"timestamp":"2026-10-01T09:00:01.400Z","type":"model.turn_started","data":{"kind":"turn_started","model":"gpt-4o-mini","modelInfo":{"id":"gpt-4o-mini","name":"GPT-4o mini","object":"model","version":"gpt-4o-mini-2024-07-18","provider":{"id":"copilot","kind":"copilot","label":"GitHub Copilot"}},"timestampMs":1790845201400,"turn":0}}
{"id":"e0000000-0000-4000-8000-000000000007","parentId":"e0000000-0000-4000-8000-000000000006","timestamp":"2026-10-01T09:00:01.410Z","type":"model.model_call_started","data":{"kind":"model_call_started","model":"gpt-4o-mini","modelInfo":{"id":"gpt-4o-mini","name":"GPT-4o mini","object":"model","version":"gpt-4o-mini-2024-07-18","provider":{"id":"copilot","kind":"copilot","label":"GitHub Copilot"}},"timestampMs":1790845201410,"turn":0}}
{"id":"e0000000-0000-4000-8000-000000000008","parentId":"e0000000-0000-4000-8000-000000000007","timestamp":"2026-10-01T09:00:01.420Z","type":"model.messages_snapshot","data":{"kind":"messages_snapshot","messages":[{"role":"system","content":"Generate a session title for this message."},{"role":"user","content":"fix the failing test"}]}}
{"id":"e0000000-0000-4000-8000-000000000009","parentId":"e0000000-0000-4000-8000-000000000008","timestamp":"2026-10-01T09:00:01.430Z","type":"model.captured_assignment_context","data":{"kind":"captured_assignment_context","assignmentContext":"exp-a:1;exp-b:0;"}}
{"id":"e0000000-0000-4000-8000-000000000010","parentId":"e0000000-0000-4000-8000-000000000009","timestamp":"2026-10-01T09:00:02.000Z","type":"model.message","data":{"callId":"4d4d4d4d-0000-4000-8000-000000000001","kind":"message","message":{"apiCallId":"chatcmpl-0001","content":"Fix failing test","outputTokens":4,"providerId":"copilot","refusal":null,"role":"assistant"},"modelCall":{"api_endpoint":"/chat/completions","api_id":"chatcmpl-0001","client_request_id":"req-0001","initiator":"agent","model":"gpt-4o-mini","request_id":"5f5f5f5f-0000-4000-8000-000000000001","rte":false,"service_request_id":"6a6a6a6a-0000-4000-8000-000000000001","transport":"http"},"turn":0}}
{"id":"e0000000-0000-4000-8000-000000000011","parentId":"e0000000-0000-4000-8000-000000000010","timestamp":"2026-10-01T09:00:02.010Z","type":"model.response","data":{"callId":"4d4d4d4d-0000-4000-8000-000000000001","kind":"response","modelCall":{"api_id":"chatcmpl-0001","model":"gpt-4o-mini"},"response":{"content":"Fix failing test","providerId":"copilot","refusal":null,"role":"assistant"},"turn":0}}
{"id":"e0000000-0000-4000-8000-000000000012","parentId":"e0000000-0000-4000-8000-000000000011","timestamp":"2026-10-01T09:00:02.020Z","type":"model.model_call_success","data":{"callId":"4d4d4d4d-0000-4000-8000-000000000001","kind":"model_call_success","modelCall":{"api_endpoint":"/chat/completions","api_id":"chatcmpl-0001","client_request_id":"req-0001","initiator":"agent","model":"gpt-4o-mini","request_id":"5f5f5f5f-0000-4000-8000-000000000001","rte":false,"service_request_id":"6a6a6a6a-0000-4000-8000-000000000001","transport":"http"},"modelCallDurationMs":590,"modelDisplayName":"GPT-4o mini","provider":{"id":"copilot","kind":"copilot","label":"GitHub Copilot"}}}
{"id":"e0000000-0000-4000-8000-000000000013","parentId":"e0000000-0000-4000-8000-000000000012","timestamp":"2026-10-01T09:00:02.030Z","type":"model.turn_ended","data":{"kind":"turn_ended","model":"gpt-4o-mini","modelInfo":{"id":"gpt-4o-mini","name":"GPT-4o mini","object":"model","version":"gpt-4o-mini-2024-07-18","provider":{"id":"copilot","kind":"copilot","label":"GitHub Copilot"}},"timestampMs":1790845202030,"turn":0}}
{"id":"e0000000-0000-4000-8000-000000000014","parentId":"e0000000-0000-4000-8000-000000000013","timestamp":"2026-10-01T09:00:03.000Z","type":"assistant.message","data":{"content":"","messageId":"7b7b7b7b-0000-4000-8000-000000000001","interactionId":"1a1a1a1a-0000-4000-8000-000000000001","originatingMessageId":"2b2b2b2b-0000-4000-8000-000000000001","model":"gpt-6.1-sol","providerId":"copilot","turnId":"0","reasoningText":"Run the tests first.","reasoningOpaque":"rs_AAAA","encryptedContent":"gAAAAA","reasoningBlocks":{"provider":"openai-responses","blocks":[{"type":"reasoning","id":"rs_AAAA","summary":[{"type":"summary_text","text":"Run the tests first."}],"encrypted_content":"gAAAAA"}]},"apiCallId":"resp_0001","rte":false,"serverTools":{"provider":"openai-responses"},"toolRequests":[{"type":"function","name":"bash","toolCallId":"call_01","toolTitle":"Run shell command","arguments":{"command":"npm test","description":"Run the test suite"},"intentionSummary":"Run the test suite"}]}}
{"id":"e0000000-0000-4000-8000-000000000015","parentId":"e0000000-0000-4000-8000-000000000014","timestamp":"2026-10-01T09:00:03.100Z","type":"permission.requested","data":{"requestId":"8c8c8c8c-0000-4000-8000-000000000001","ownerSessionId":"5e55a0a1-0000-4000-8000-000000000001","agentMode":"interactive","permissionMode":"manual","permissionRequest":{"kind":"shell","toolCallId":"call_01","intention":"Run the test suite","fullCommandText":"npm test","shellType":"bash","commands":[{"identifier":"npm test","readOnly":false,"allowMatchIdentifier":"npm test"}],"commandSegments":[{"identifier":"npm test","fullCommandText":"npm test"}],"possiblePaths":[],"possibleUrls":[],"hasWriteFileRedirection":false,"canOfferSessionApproval":true},"promptRequest":{"kind":"commands","toolCallId":"call_01","intention":"Run the test suite","fullCommandText":"npm test","commandIdentifiers":["npm test"]}}}
{"id":"e0000000-0000-4000-8000-000000000016","parentId":"e0000000-0000-4000-8000-000000000015","timestamp":"2026-10-01T09:00:06.000Z","type":"permission.completed","data":{"requestId":"8c8c8c8c-0000-4000-8000-000000000001","toolCallId":"call_01","decisionSource":"human_response","result":{"kind":"approved-for-location","locationKey":"/srv/proj","approval":{"kind":"commands","commandIdentifiers":["npm test"]}}}}
{"id":"e0000000-0000-4000-8000-000000000017","parentId":"e0000000-0000-4000-8000-000000000016","timestamp":"2026-10-01T09:00:06.100Z","type":"tool.execution_start","data":{"toolCallId":"call_01","toolName":"bash","toolTitle":"Run shell command","arguments":{"command":"npm test","description":"Run the test suite"},"model":"gpt-6.1-sol","turnId":"0"}}
{"id":"e0000000-0000-4000-8000-000000000018","parentId":"e0000000-0000-4000-8000-000000000017","timestamp":"2026-10-01T09:00:08.000Z","type":"tool.execution_complete","data":{"toolCallId":"call_01","success":false,"result":{"content":"1 failing","detailedContent":"1 failing\n  date.test.js: expected 2026"},"error":{"code":"failure","message":"Command exited with code 1"},"shellExecution":{"exitCode":1},"toolTelemetry":{"metrics":{},"properties":{}},"model":"gpt-6.1-sol","interactionId":"1a1a1a1a-0000-4000-8000-000000000001","turnId":"0","rte":false}}
{"id":"e0000000-0000-4000-8000-000000000019","parentId":"e0000000-0000-4000-8000-000000000018","timestamp":"2026-10-01T09:00:08.100Z","type":"assistant.turn_end","data":{"turnId":"0"}}
{"id":"e0000000-0000-4000-8000-000000000020","parentId":"e0000000-0000-4000-8000-000000000019","timestamp":"2026-10-01T09:00:08.200Z","type":"assistant.turn_start","data":{"interactionId":"1a1a1a1a-0000-4000-8000-000000000001","turnId":"1"}}
{"id":"e0000000-0000-4000-8000-000000000021","parentId":"e0000000-0000-4000-8000-000000000020","timestamp":"2026-10-01T09:00:10.000Z","type":"assistant.message","data":{"content":"","messageId":"7b7b7b7b-0000-4000-8000-000000000002","interactionId":"1a1a1a1a-0000-4000-8000-000000000001","originatingMessageId":"2b2b2b2b-0000-4000-8000-000000000001","model":"gpt-6.1-sol","providerId":"copilot","turnId":"1","apiCallId":"resp_0002","rte":false,"toolRequests":[{"type":"custom","name":"apply_patch","toolCallId":"custom_0000000000000000000000000001","toolTitle":"Apply patch","arguments":"*** Begin Patch\n*** Update File: /srv/proj/date.test.js\n-2025\n+2026\n*** End Patch\n","intentionSummary":"Update the fixture year"}]}}
{"id":"e0000000-0000-4000-8000-000000000022","parentId":"e0000000-0000-4000-8000-000000000021","timestamp":"2026-10-01T09:00:10.100Z","type":"permission.requested","data":{"requestId":"8c8c8c8c-0000-4000-8000-000000000002","ownerSessionId":"5e55a0a1-0000-4000-8000-000000000001","agentMode":"interactive","permissionMode":"manual","permissionRequest":{"kind":"write","toolCallId":"custom_0000000000000000000000000001","intention":"Update the fixture year","fileName":"/srv/proj/date.test.js","diff":"-2025\n+2026\n","canOfferSessionApproval":true},"promptRequest":{"kind":"write","toolCallId":"custom_0000000000000000000000000001","intention":"Update the fixture year","fileName":"/srv/proj/date.test.js"}}}
{"id":"e0000000-0000-4000-8000-000000000023","parentId":"e0000000-0000-4000-8000-000000000022","timestamp":"2026-10-01T09:00:12.000Z","type":"permission.completed","data":{"requestId":"8c8c8c8c-0000-4000-8000-000000000002","toolCallId":"custom_0000000000000000000000000001","decisionSource":"human_response","result":{"kind":"approved"}}}
{"id":"e0000000-0000-4000-8000-000000000024","parentId":"e0000000-0000-4000-8000-000000000023","timestamp":"2026-10-01T09:00:12.100Z","type":"tool.execution_start","data":{"toolCallId":"custom_0000000000000000000000000001","toolName":"apply_patch","toolTitle":"Apply patch","arguments":{"input":"*** Begin Patch\n*** Update File: /srv/proj/date.test.js\n-2025\n+2026\n*** End Patch\n"},"model":"gpt-6.1-sol","turnId":"1"}}
{"id":"e0000000-0000-4000-8000-000000000025","parentId":"e0000000-0000-4000-8000-000000000024","timestamp":"2026-10-01T09:00:12.500Z","type":"tool.execution_complete","data":{"toolCallId":"custom_0000000000000000000000000001","success":true,"result":{"content":"Modified 1 file: /srv/proj/date.test.js"},"toolTelemetry":{"metrics":{},"properties":{}},"model":"gpt-6.1-sol","interactionId":"1a1a1a1a-0000-4000-8000-000000000001","turnId":"1","rte":false}}
{"id":"e0000000-0000-4000-8000-000000000026","parentId":"e0000000-0000-4000-8000-000000000025","timestamp":"2026-10-01T09:00:12.600Z","type":"assistant.turn_end","data":{"turnId":"1"}}
{"id":"e0000000-0000-4000-8000-000000000027","parentId":"e0000000-0000-4000-8000-000000000026","timestamp":"2026-10-01T09:00:12.700Z","type":"assistant.turn_start","data":{"interactionId":"1a1a1a1a-0000-4000-8000-000000000001","turnId":"2"}}
{"id":"e0000000-0000-4000-8000-000000000028","parentId":"e0000000-0000-4000-8000-000000000027","timestamp":"2026-10-01T09:00:15.000Z","type":"assistant.message","data":{"content":"The fixture year was stale; it now reads 2026.","messageId":"7b7b7b7b-0000-4000-8000-000000000003","interactionId":"1a1a1a1a-0000-4000-8000-000000000001","originatingMessageId":"2b2b2b2b-0000-4000-8000-000000000001","model":"gpt-6.1-sol","providerId":"copilot","turnId":"2","phase":"final_answer","apiCallId":"resp_0003","rte":false,"toolRequests":[]}}
{"id":"e0000000-0000-4000-8000-000000000029","parentId":"e0000000-0000-4000-8000-000000000028","timestamp":"2026-10-01T09:00:15.100Z","type":"assistant.turn_end","data":{"turnId":"2"}}
{"id":"e0000000-0000-4000-8000-000000000030","parentId":"e0000000-0000-4000-8000-000000000029","timestamp":"2026-10-01T09:00:15.200Z","type":"session.usage_record","data":{"usage":{"model":"gpt-6.1-sol","inputTokens":900,"outputTokens":40,"initiator":"agent","interactionType":"conversation-agent","apiEndpoint":"ws:/responses","transport":"websocket","isByok":false,"accounting":{"sequence":1,"sourceSessionId":"5e55a0a1-0000-4000-8000-000000000001","usageId":"u-0001"}}}}
{"id":"e0000000-0000-4000-8000-000000000031","parentId":"e0000000-0000-4000-8000-000000000030","timestamp":"2026-10-01T09:00:20.000Z","type":"session.model_change","data":{"previousModel":"gpt-6.1-sol","newModel":"auto","previousReasoningEffort":"medium","reasoningEffort":null,"contextTier":null,"source":"model_picker"}}
{"id":"e0000000-0000-4000-8000-000000000032","parentId":"e0000000-0000-4000-8000-000000000031","timestamp":"2026-10-01T09:00:20.395Z","type":"session.info","data":{"message":"Model changed from GitHub Copilot • GPT-6.1 Sol (medium) to GitHub Copilot • Auto for this session","infoType":"model"}}
{"id":"e0000000-0000-4000-8000-000000000033","parentId":"e0000000-0000-4000-8000-000000000032","timestamp":"2026-10-01T09:00:20.396Z","type":"session.info","data":{"message":"Auto routing profile change to Balance requested.","infoType":"model"}}
{"id":"e0000000-0000-4000-8000-000000000034","parentId":"e0000000-0000-4000-8000-000000000033","timestamp":"2026-10-01T09:00:30.000Z","type":"session.shutdown","data":{"shutdownType":"routine","sessionStartTime":1790845199000,"totalApiDurationMs":9000,"eventsFileSizeBytes":20000,"codeChanges":{"filesModified":["/srv/proj/date.test.js"],"linesAdded":1,"linesRemoved":1},"modelMetrics":{},"currentModel":"gpt-6.1-sol"}}
{"id":"e0000000-0000-4000-8000-000000000035","parentId":"e0000000-0000-4000-8000-000000000034","timestamp":"2026-10-01T09:01:00.000Z","type":"user.message","data":{"content":"and run
```

`session-store.db`:

```sql
INSERT INTO sessions VALUES ('5e55a0a1-0000-4000-8000-000000000001', '/srv/proj', NULL, NULL, NULL, 'Fix failing test', '2026-10-01T09:00:00.000Z', '2026-10-01T09:00:30.000Z');
INSERT INTO turns (session_id, turn_index, user_message, assistant_response, timestamp) VALUES ('5e55a0a1-0000-4000-8000-000000000001', 0, 'fix the failing test', 'The fixture year was stale; it now reads 2026.', '2026-10-01T09:00:15.200Z');
INSERT INTO turns (session_id, turn_index, user_message, assistant_response, timestamp) VALUES ('5e55a0a1-0000-4000-8000-000000000001', 1, 'and run lint', NULL, '2026-10-01T09:01:05.000Z');
INSERT INTO session_files (session_id, file_path, tool_name, turn_index, first_seen_at) VALUES ('5e55a0a1-0000-4000-8000-000000000001', '/srv/proj/date.test.js', 'apply_patch', 0, '2026-10-01T09:00:12.500Z');
```

`.copilot/session-state/5e55a0a1-0000-4000-8000-000000000001/workspace.yaml`:

```yaml
id: 5e55a0a1-0000-4000-8000-000000000001
cwd: /srv/proj
client_name: copilot-cli
name: Fix failing test
user_named: false
summary_count: 0
fork_count: 0
created_at: 2026-10-01T09:00:00.000Z
updated_at: 2026-10-01T09:00:30.000Z
remote_steerable: false
mc_task_id: null
mc_session_id: null
mc_last_event_id: null
```

`.copilot/command-history-state.json`:

```json
{"commandHistory":["and run lint","/model","fix the failing test"]}
```

`.copilot/permissions-config.json`:

```json
{"locations":{"/srv/proj":{"tool_approvals":[{"kind":"write"},{"kind":"commands","commandIdentifiers":["npm test"]}]}}}
```

`.copilot/settings.json`:

```json
{
  // user settings
  "allowedUrls": ["https://registry.example.org"]
}
```

`.copilot/session-state/5e55a0a1-0000-4000-8000-000000000001/rewind-file-snapshots/index.json`
(the snapshot taken before the first prompt, whose `eventId` is line 4's
`id`; the file key is a synthetic digest and the images are left out):

```json
{"schema":1,"snapshots":[{"snapshotId":"9d9d9d9d-0000-4000-8000-000000000001","eventId":"e0000000-0000-4000-8000-000000000004","timestamp":"2026-10-01T09:00:01.150Z","userMessage":"fix the failing test","fileCount":1,"linesAdded":1,"linesRemoved":1,"backupFiles":[],"files":{"a1a1a1a1a1a1a1a1":{"path":"/srv/proj/date.test.js","linesAdded":1,"linesRemoved":1,"preimage":null,"postimage":null}}}]}
```

`.copilot/sidebar-sessions-state/<sha256>.json`, where `<sha256>` is
`6b9af143446a411aefa3edb76e4693799bd7513e7ed8d919962d6713d3d3b972`, the sha256 of `/srv/proj`:

```json
{"schemaVersion":1,"cwd":"/srv/proj","sessionIds":["5e55a0a1-0000-4000-8000-000000000001"]}
```

`.copilot/logs/process-1790845199000-4242.log`:

```text
2026-10-01T08:59:59.100Z [INFO] Starting Copilot CLI: 1.0.98
2026-10-01T08:59:59.400Z [INFO] Signed in to https://github.com as octo-user.
2026-10-01T08:59:59.900Z [INFO] Workspace initialized: 5e55a0a1-0000-4000-8000-000000000001
```

Noise the parser must not want: `.copilot/config.json`,
`.copilot/session-state/5e55a0a1-0000-4000-8000-000000000001/rewind-file-snapshots/tracking.json`
and `.copilot/session-state/5e55a0a1-0000-4000-8000-000000000001/checkpoints/index.md`.

Expected rows from `events.jsonl`: `system` session start (line 1, with
`project_path` `/srv/proj` and `git_branch` `main`); `system` `model:
gpt-6.1-sol cause=initial_resolution source=automatic effort=medium`
(line 2); `system` system prompt (line 3); `user` `fix the failing test`
(line 4); `system` `utility model gpt-4o-mini: Fix failing test` (line
10); `thinking` `Run the tests first.` with `include_thinking` (line 14);
`tool_use` `bash` `call_01` text `npm test` (line 14); `system`
`permission request: shell npm test intention=Run the test suite` and
`permission mode: manual agent=interactive` (line 15); `system`
`permission decision: approved-for-location source=human_response
location=/srv/proj approval={"commandIdentifiers":["npm test"],"kind":"commands"}`
(line 16); `tool_result` `call_01` text `[exit 1] 1 failing` and the
detail, then `Command exited with code 1` (line 18); `tool_use`
`apply_patch` (line 21); the write request and `permission decision:
approved source=human_response` (lines 22, 23); `tool_result` (line 25);
`assistant` reply (line 28); `system` `model: gpt-6.1-sol -> auto
source=model_picker` (line 31); `system` `session.info model: Model
changed from GitHub Copilot • GPT-6.1 Sol (medium) to GitHub Copilot •
Auto for this session` and `session.info model: Auto routing profile
change to Balance requested.` (lines 32, 33, `model` `auto`); `system`
session end (line 34); the bad-line `system` row (line 35). Six secondary rows drop: the snapshot's
prompt pairs with line 4 (the nearest in time), and the `turns` row 0
prompt, which loses that one-to-one match, collapses onto the snapshot's
copy; the row 0 reply pairs with line 28; the `sessions` row's `session
name: Fix failing test` pairs with the `workspace.yaml` row; the history's
`fix the failing test` pairs with line 4, and its `and run lint` collapses
onto the `turns` row 1, the only other copy of a prompt the cut log lost.
Kept: `turns` row 1 (`and run lint`), the history's `/model` (an undated
`user` row, in no other source), `file changed:
/srv/proj/date.test.js tool=apply_patch` and `rewind snapshot: 1 files +1
-1: /srv/proj/date.test.js`.

#### MCP

The configured servers (`mcp-config.json`), their failed spawns (the process log's
`MCP server connection failed` lines) and the cached tool lists
(`AppData/Local/copilot/mcp-tools/<sha256>.json`) are recorded on disk (Windows install) and not yet
in the timeline ([issue 150](https://github.com/seanthegeek/doubleagent/issues/150)).

## Confidence

- High, from the docs and the bundle with real installs on macOS and Linux (1.0.98) and Windows
  (winget 1.0.98 and Copilot Desktop's bundled 1.0.99-2): the config home and `COPILOT_HOME`, the
  directory names, the XDG migration list, the package cache locations (`~/Library/Caches/copilot`,
  `~/.cache/copilot`, `AppData/Local/copilot`), the log directory, the keyring and plaintext token
  model (the GNOME keyring on Linux, Windows Credential Manager on Windows), the
  `session-state/<id>/` file set, the `workspace.yaml` keys (cwd in `cwd`) and the sync keys a
  resume adds, the fifth `config.json` key, the `session-store.db` schema and its SQL, the
  `permissions-config.json` shape and the registry device id on Windows.
- High, real install on Windows: `mcp-config.json` holding a plain-text token under a server's
  `env`, the `mcp-tools/<sha256>.json` shape and the marketplace clone's size.
- High, schema and real installs agree: the envelope, the observed event types and keys, the
  permission result kinds, the `toolCallId` join, the history file's order, the WAL; the
  session-name generator and the task-analytics classifier as the three utility calls per session on
  macOS; the `turns` write pattern behind the repeated rows (complete row at the next prompt or at
  shutdown, prompt-only row mid-turn; Linux), the `session.info` shape and the `model_picker`
  change; the subagent event types (observed on Windows, keys not recorded).
- Medium: `state/ide` against `ide/` (every install shows `ide/` at the top level; the code joins
  through a "state" resolver); `session.db`, `plan.md` and `mcp-secrets/` (bundle or docs, not
  observed); the `config.json` token field name under `storeTokenPlaintext` (not observed); the
  unobserved event types, the denial and session-approval shapes, `forge_trajectory_events` and the
  snapshot's timing (bundle only).
- Not determined: why `session-store.db` reports `schema_version` 8 on Linux and Windows and 38 on
  macOS; the Credential Manager target name; what the `managed-settings.<sha256>.fence` file holds;
  what the 12 utility calls of the winget session were for; why the store kept 3 of 6 turns; whether
  `permission.completed` nests `decisionSource` in `result` on Windows; the legacy flat transcript's
  shape; the layout of an npm-installed CLI on Windows ([issue
  133](https://github.com/seanthegeek/doubleagent/issues/133)).
