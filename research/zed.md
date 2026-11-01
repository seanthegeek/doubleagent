# zed

Zed (catalog agent `zed`) is a code editor whose built-in agent panel runs threads with the
native Zed Agent and drives external agents over ACP, the Agent Client Protocol, the JSON-RPC
protocol Zed uses to drive an external agent (Claude Agent, Gemini CLI, Codex) as a child process.
It was researched together with Goose ([`goose.md`](goose.md)). The analyzer parses the panel's
threads in `threads.db`, the agent tables and key-value namespaces of `db.sqlite` and the standing
permission entries of the user settings files, from source and fixtures only.

## Source and evidence

[zed-industries/zed](https://github.com/zed-industries/zed), open source, at
[`a84689073d296dfd39987bc7dd478e43ef76d83a`](https://github.com/zed-industries/zed/commit/a84689073d296dfd39987bc7dd478e43ef76d83a)
(blobless partial clone); agent crates GPL-3.0-or-later
([`crates/agent/Cargo.toml:6`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/agent/Cargo.toml#L6),
[`crates/agent_ui/Cargo.toml:6`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/agent_ui/Cargo.toml#L6)).
Rust; sqlez (Zed's own SQLite wrapper), zstd crate, agent-client-protocol-schema 1.9.1. Platform
mapping verified against the `dirs` 6.0.0 crate (the version in `Cargo.lock`). Flatpak sandbox
paths are Flatpak documentation level. Paths cited are relative to the clone.

### Observed

No real install. Every layout the parser handles (`threads.db`, the thread blob of
[Record schema](#record-schema) and `sidebar_threads`) and the bad-record rows of
[Bad-line rows](#bad-line-rows) rest on source at
[`a846890`](https://github.com/zed-industries/zed/commit/a84689073d296dfd39987bc7dd478e43ef76d83a)
and [`9cd14da`](https://github.com/zed-industries/zed/commit/9cd14daad768fbfaeeac8789f515f9a5f67669a6)
and on a synthetic fixture.

### Source reviews

- 2026-10-21: zed-industries/zed at
  [`9cd14da`](https://github.com/zed-industries/zed/commit/9cd14daad768fbfaeeac8789f515f9a5f67669a6):
  the plan lines of [Parser plan](#parser-plan) for standing permissions, denied calls, scratch
  directories, draft prompts, archived worktrees, sidebar terminals and the native sidebar rows now
  describe what the parser does, source-derived
  ([issue 196](https://github.com/seanthegeek/doubleagent/issues/196)). The review found that a
  project's `.zed/settings.json` cannot hold the permission keys, added the override blocks of a
  user settings file, the remaining automatic denial texts, the draft prompts' ACP block shape and
  the [Source host rows](#source-host-rows) statement.
- 2026-10-19, [zed-industries/zed](https://github.com/zed-industries/zed) at
  [`9cd14da`](https://github.com/zed-industries/zed/commit/9cd14daad768fbfaeeac8789f515f9a5f67669a6)
  (the default branch that day): the paths of [Per-user storage](#per-user-storage) and the shapes
  the parser reads were unchanged, with line numbers moved by one or two
  ([Format versions](#format-versions)). The review added the permission settings and project trust,
  the denied-call results, the other `db.sqlite` tables, the scratch directories, the delete
  behaviour and the external agents' ids, with the plan lines of [Parser plan](#parser-plan)
  that read them, and the bad-record rows the parser now yields
  ([issue 168](https://github.com/seanthegeek/doubleagent/issues/168)).
- 2026-10-17, [zed-industries/zed](https://github.com/zed-industries/zed) at
  [`56934ec`](https://github.com/zed-industries/zed/commit/56934ec0560bce8929025497411de7f5322fb6ca):
  a sync check that added [Sync, relay and origin rows](#sync-relay-and-origin-rows),
  [Telemetry log and crash reports](#telemetry-log-and-crash-reports) and the clipboard actions of
  [Format versions](#format-versions), whose citations are to that commit.

## Collection

### Per-user storage

`APP_NAME` is `Zed`, lowercased for XDG paths
([`crates/paths/src/paths.rs:18-21`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/paths/src/paths.rs#L18-L21)).

- Config
  ([`paths.rs:122-139`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/paths/src/paths.rs#L122-L139)):
  Windows `dirs::config_dir()/Zed` = `%APPDATA%\Zed`
  ([`dirs/src/win.rs:8`](https://github.com/dirs-dev/dirs-rs/blob/00511bdc252f491a72ac89f6d6a2463406266fb2/src/win.rs#L8));
  Linux/FreeBSD `$FLATPAK_XDG_CONFIG_HOME` or `$XDG_CONFIG_HOME`/`~/.config`, then `/zed`
  ([`dirs/src/lin.rs:9`](https://github.com/dirs-dev/dirs-rs/blob/00511bdc252f491a72ac89f6d6a2463406266fb2/src/lin.rs#L9));
  macOS `~/.config/zed`.
- Data
  ([`paths.rs:144-165`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/paths/src/paths.rs#L144-L165)):
  macOS `~/Library/Application Support/Zed`; Linux `$FLATPAK_XDG_DATA_HOME` or
  `dirs::data_local_dir()` = `$XDG_DATA_HOME` or `~/.local/share`, then `/zed`
  ([`lin.rs:11-12`](https://github.com/dirs-dev/dirs-rs/blob/00511bdc252f491a72ac89f6d6a2463406266fb2/src/lin.rs#L11-L12));
  Windows `dirs::data_local_dir()/Zed` = `%LOCALAPPDATA%\Zed`
  ([`win.rs:11`](https://github.com/dirs-dev/dirs-rs/blob/00511bdc252f491a72ac89f6d6a2463406266fb2/src/win.rs#L11)).
- State
  ([`paths.rs:169-190`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/paths/src/paths.rs#L169-L190)):
  macOS `~/.local/state/Zed` (capital Z), Linux `~/.local/state/zed`, Windows `%LOCALAPPDATA%\Zed`.
  No caller outside the paths crate uses it today.
- Temp/cache
  ([`paths.rs:193-218`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/paths/src/paths.rs#L193-L218)):
  macOS `~/Library/Caches/Zed`, Windows `%LOCALAPPDATA%\Zed`, Linux `~/.cache/zed`.
- Logs
  ([`paths.rs:228-255`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/paths/src/paths.rs#L228-L255)):
  macOS `~/Library/Logs/Zed`, elsewhere `<data>/logs`; files `Zed.log`, `Zed.log.old`. macOS crash
  reports in `~/Library/Logs/DiagnosticReports`
  ([`:264-268`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/paths/src/paths.rs#L264-L268)).
- `--user-data-dir <dir>`
  ([`crates/cli/src/main.rs:524`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/cli/src/main.rs#L524),[`969`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/cli/src/main.rs#L969))
  makes `<dir>` the data dir and `<dir>/config` the config dir
  ([`paths.rs:103-125`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/paths/src/paths.rs#L103-L125)).
- Flatpak builds use `FLATPAK_XDG_*`
  ([`paths.rs:131`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/paths/src/paths.rs#L131),[`153`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/paths/src/paths.rs#L153),[`177`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/paths/src/paths.rs#L177),[`209`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/paths/src/paths.rs#L209));
  app ids `dev.zed.Zed`, `dev.zed.ZedPreview`, `dev.zed.ZedNightly`, `dev.zed.ZedDev`
  ([`script/flatpak/bundle-flatpak:15-33`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/script/flatpak/bundle-flatpak#L15-L33)),
  so sandboxed state is `~/.var/app/dev.zed.Zed*/{config,data,cache}/zed`.

Config dir files: `settings.json`, `global_settings.json`, `settings_backup.json`, `keymap.json`,
`keymap_backup.json`, `tasks.json`, `debug.json`, `AGENTS.md`
([`paths.rs:278-326`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/paths/src/paths.rs#L278-L326)),
`themes/`, `snippets/`
([`:372-380`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/paths/src/paths.rs#L372-L380)),
`development_credentials`
([`crates/zed_credentials_provider/src/zed_credentials_provider.rs:116`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/zed_credentials_provider/src/zed_credentials_provider.rs#L116)),
`git/config`. On macOS only, `prompts/` (holding the LMDB `prompts-library-db.0.mdb`,
[`crates/prompt_store/src/prompt_store.rs:28`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/prompt_store/src/prompt_store.rs#L28)),
`prompt_overrides/` and `embeddings/` sit under config; elsewhere under data
([`paths.rs:386-433`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/paths/src/paths.rs#L386-L433)).

Data dir: `threads/threads.db` agent threads
([`crates/agent/src/db.rs:444-446`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/agent/src/db.rs#L444-L446));
`db/0-<channel>/db.sqlite` where channel is `dev`, `nightly`, `preview`, `stable` or `global`
([`crates/db/src/db.rs:138`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/db/src/db.rs#L138),[`148-166`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/db/src/db.rs#L148-L166);
[`crates/release_channel/src/lib.rs:218-221`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/release_channel/src/lib.rs#L218-L221));
`extensions/`, `remote_extensions/` (with `uploads/`), `languages/`, `debug_adapters/`,
`external_agents/`, `copilot/`, `prettier/`, `remote_servers/`, `devcontainer/`, `hang_traces/`,
`server_state/`
([`paths.rs:222-242`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/paths/src/paths.rs#L222-L242),[`348-483`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/paths/src/paths.rs#L348-L483));
`node/` runtime
([`crates/node_runtime/src/node_runtime.rs:637`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/node_runtime/src/node_runtime.rs#L637),[`1031`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/node_runtime/src/node_runtime.rs#L1031));
`build_timings/`
([`crates/zed/src/reliability.rs:465`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/zed/src/reliability.rs#L465));
`zed-<channel>.sock`
([`crates/zed/src/zed/open_listener.rs:413`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/zed/src/zed/open_listener.rs#L413));
`logs/` on Linux and Windows. No `conversations/` directory exists in current source.

`db.sqlite` also holds the agent panel's other tables and key-value namespaces, listed in
[Other db.sqlite tables](#other-dbsqlite-tables). Global agent skills are read from
`~/.agents/skills/<skill>/SKILL.md`, a directory shared with other agents and not created by Zed
([`crates/agent/src/agent.rs:616-638`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/agent/src/agent.rs#L616-L638)).

#### Scratch directories

Zed has two, and the catalog collects neither:

- On macOS only, a thread whose terminal runs in the OS sandbox gets `tempfile`'s
  `$TMPDIR/zed-agent-terminal-<random>` (under `/var/folders`, outside the home), kept for the
  thread's life as its `TMPDIR` and removed when the thread is deleted; the thread blob in
  `threads.db` records it as `sandboxed_terminal_temp_dir`
  ([`crates/agent/src/thread.rs:1540-1566`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/agent/src/thread.rs#L1540-L1566);
  [`crates/agent/src/db.rs:680-700`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/agent/src/db.rs#L680-L700)).
  The directory is a writable path in the Seatbelt sandbox (macOS's per-process sandbox profiles);
  Linux uses bubblewrap's private `/tmp` and Windows the WSL sandbox's, so neither gets one
  ([`agent.rs:3851-3870`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/agent/src/agent.rs#L3851-L3870)).
- A thread started in a new worktree runs in a linked git worktree (a second checkout made with
  `git worktree add`) that Zed creates under the `git.worktree_directory` setting, by default
  `../worktrees` relative to the repository, giving
  `<project parent>/worktrees/<worktree name>/<project>`; the setting must stay inside the project
  or its parent
  ([`crates/project/src/git_store.rs:6332-6349`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/project/src/git_store.rs#L6332-L6349),
  [`10845-10898`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/project/src/git_store.rs#L10845-L10898);
  [`assets/settings/default.json:1885-1901`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/assets/settings/default.json#L1885-L1901)).
  The worktree path becomes the thread's `folder_paths`, with the real project in
  `sidebar_threads.main_worktree_paths`. It is user project content beside the project, so no
  catalog line can name it; where it is recorded is in
  [Where the project path is recorded](#where-the-project-path-is-recorded).

### Credentials

Production builds store credentials only in the OS keychain
([`zed_credentials_provider.rs:56-64`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/zed_credentials_provider/src/zed_credentials_provider.rs#L56-L64),[`69`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/zed_credentials_provider/src/zed_credentials_provider.rs#L69)).
On the Dev channel, unless `ZED_DEVELOPMENT_USE_KEYCHAIN` is set, they go to
`<config>/development_credentials` as JSON
([`:50-62`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/zed_credentials_provider/src/zed_credentials_provider.rs#L50-L62),[`116-120`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/zed_credentials_provider/src/zed_credentials_provider.rs#L116-L120)).
`settings.json` (and its `settings_backup.json` copy and `global_settings.json`) can embed `api_key`
and `custom_headers` for LM Studio style providers
([`crates/settings_content/src/language_model.rs:310-312`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/settings_content/src/language_model.rs#L310-L312))
and `agent_servers` environment. `db.sqlite` and `threads.db` hold no tokens.

The GitHub Copilot model provider runs its own GitHub OAuth device flow with Zed's client id and
the `read:user` scope
([`crates/copilot_chat/src/copilot_oauth.rs:20-23`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/copilot_chat/src/copilot_oauth.rs#L20-L23))
and keeps the token through the same credentials provider under the URL
`https://<oauth domain>/copilot-agent`, so in the keychain, or in `development_credentials` on the
Dev channel
([`crates/copilot_chat/src/copilot_chat.rs:50-52`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/copilot_chat/src/copilot_chat.rs#L50-L52),
[`562-571`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/copilot_chat/src/copilot_chat.rs#L562-L571),
[`678`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/copilot_chat/src/copilot_chat.rs#L678)).

The `settings.json` keys `agent.tool_permissions` and `agent.sandbox_permissions` hold rules and
paths, not secrets, and the `scoped_kv_store` namespaces read at
[`9cd14da`](https://github.com/zed-industries/zed/commit/9cd14daad768fbfaeeac8789f515f9a5f67669a6) hold no token.

### Exclusions

Under either data root: `extensions`, `remote_extensions`, `node`, `languages`, `copilot`,
`debug_adapters`, `external_agents`, `prettier`, `remote_servers`, `devcontainer`, `embeddings`,
`server_state`, `hang_traces`, plus `build_timings`. `~/.cache/zed` and `~/Library/Caches/Zed` are
temp. `prompts/*.mdb` is the user prompt library, small, keep.

### Project-local files

`.zed/settings.json`, `.zed/tasks.json`, `.zed/debug.json`
([`paths.rs:487-531`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/paths/src/paths.rs#L487-L531));
rules files read by the agent: `.rules`, `.cursorrules`, `.windsurfrules`, `.clinerules`,
`.github/copilot-instructions.md`, `AGENT.md`, `AGENTS.md`, `CLAUDE.md`, `GEMINI.md`
([`crates/prompt_store/src/prompts.rs:22-32`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/prompt_store/src/prompts.rs#L22-L32)).
No per-project database.

### Where the project path is recorded

- `threads.db` table `threads` columns `folder_paths`, `folder_paths_order`
  ([`crates/agent/src/db.rs:469-470`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/agent/src/db.rs#L469-L470),[`545`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/agent/src/db.rs#L545)).
- `db.sqlite` table `sidebar_threads` columns `folder_paths`, `folder_paths_order`
  ([`crates/agent_ui/src/thread_metadata_store.rs:1375-1383`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/agent_ui/src/thread_metadata_store.rs#L1375-L1383))
  and table `workspaces` columns `local_paths_array`, `local_paths_order_array`
  ([`crates/workspace/src/persistence.rs:731-733`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/workspace/src/persistence.rs#L731-L733)).
- `sidebar_threads.main_worktree_paths` and `main_worktree_paths_order`: the main repository of a
  thread whose `folder_paths` is a linked worktree
  ([`crates/agent_ui/src/thread_metadata_store.rs:1387-1388`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/agent_ui/src/thread_metadata_store.rs#L1387-L1388)).
- `archived_git_worktrees` columns `worktree_path` and `main_repo_path`
  ([`thread_metadata_store.rs:1390-1398`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/agent_ui/src/thread_metadata_store.rs#L1390-L1398)).
- `scoped_kv_store` namespace `created_git_worktrees`, key `local`, a newline, then the worktree
  path: the worktrees Zed made, which are scratch rather than projects
  ([`crates/git_ui_core/src/created_worktrees.rs:34-50`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/git_ui_core/src/created_worktrees.rs#L34-L50)).
- `trusted_worktrees.absolute_path`: every project the user trusted
  ([`crates/workspace/src/persistence.rs:986-991`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/workspace/src/persistence.rs#L986-L991)).
- `sidebar_terminal_threads.working_directory` and `folder_paths`
  ([`crates/agent_ui/src/terminal_thread_metadata_store.rs:483-500`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/agent_ui/src/terminal_thread_metadata_store.rs#L483-L500)).

Every entry after the first is in `db.sqlite`. All are SQLite columns, which the collector's
project discovery does not read.

## Parsing

### Transcript stores

`<data>` is the data directory of [Per-user storage](#per-user-storage).

| Store | Path | Unit |
| --- | --- | --- |
| Threads database | `<data>/threads/threads.db` | `threads` row = one thread; `data` BLOB = zstd(JSON) of the whole thread including all messages ([`crates/agent/src/db.rs:444-457`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/agent/src/db.rs#L444-L457),[`535`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/agent/src/db.rs#L535)) |
| Sidebar metadata | `<data>/db/0-{stable,preview,nightly,dev}/db.sqlite`, table `sidebar_threads` | one row per thread, including external-agent threads ([`crates/db/src/db.rs:138`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/db/src/db.rs#L138),[`166`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/db/src/db.rs#L166); [`release_channel/src/lib.rs:216-223`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/release_channel/src/lib.rs#L216-L223); [`agent_ui/src/thread_metadata_store.rs:1375-1465`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/agent_ui/src/thread_metadata_store.rs#L1375-L1465)) |

No separate text-thread or `contexts/*.json` store exists at this commit.

An external agent's transcript is in no Zed store: Zed asks the agent to replay it over the ACP
`session/load` request
([`agent_servers/src/acp/v2.rs:1173`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/agent_servers/src/acp/v2.rs#L1173)),
so it lives wherever that agent keeps its sessions. The ids of the three built-in external agents,
the `sidebar_threads.agent_id` values, are `gemini`, `claude-acp` and `codex-acp`
([`agent_servers/src/custom.rs:17-19`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/agent_servers/src/custom.rs#L17-L19)).

#### What survives a delete

Deleting a thread (the archive view's delete, or the native connection's `delete_session`) removes
its `threads` row, every subagent row whose `parent_id` chain leads to it and the macOS sandbox temp
directory its blob names
([`crates/agent/src/db.rs:680-754`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/agent/src/db.rs#L680-L754);
[`agent.rs:3613-3625`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/agent/src/agent.rs#L3613-L3625)),
then its `sidebar_threads` row and its draft prompt, then the archived-worktree rows and their
`refs/archived-worktrees/<id>` ref once no other thread links to them
([`crates/agent_ui/src/threads_archive_view.rs:814-854`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/agent_ui/src/threads_archive_view.rs#L814-L854)).
"Delete all" empties `threads`
([`db.rs:756-789`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/agent/src/db.rs#L756-L789)),
and whether the sidebar rows of a thread removed that way outlive it was not determined.

Archiving only sets `sidebar_threads.archived`; for a thread in a Zed-created worktree it also
removes the worktree directory after saving its state as commits
([`crates/agent_ui/src/thread_metadata_store.rs:859-878`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/agent_ui/src/thread_metadata_store.rs#L859-L878)).
A per-prompt history file is not stated. The `created_git_worktrees` entry of a thread's Zed-created
worktree survives the deletion, and whether a sidebar terminal row does was not determined. An
external agent's thread lives in that agent's own store, and Zed asks the agent to delete it over
ACP `session/delete` only when the agent offers it
([`crates/agent_servers/src/acp/v1.rs:285-300`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/agent_servers/src/acp/v1.rs#L285-L300)),
so otherwise it stays there. A copy already sent with a rating stays with Zed
([Sync, relay and origin](#sync-relay-and-origin-rows)). Nothing local records a deletion. The
analyzer reads `threads.db` and every store of [Other db.sqlite tables](#other-dbsqlite-tables), so
a deleted thread's surviving `created_git_worktrees` entry still shows; none of these stores
repeats a thread's messages, so none is a secondary source (see
[Secondary sources](../analyzer/docs/output.md#secondary-sources)).

### Record schema

#### threads table

`threads.db`
([`crates/agent/src/db.rs:450-485`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/agent/src/db.rs#L450-L485)):

```sql
CREATE TABLE threads (id TEXT PRIMARY KEY, summary TEXT NOT NULL, updated_at TEXT NOT NULL,
  data_type TEXT NOT NULL, data BLOB NOT NULL);
ALTER TABLE threads ADD COLUMN parent_id TEXT;
ALTER TABLE threads ADD COLUMN folder_paths TEXT; ALTER TABLE threads ADD COLUMN folder_paths_order TEXT;
ALTER TABLE threads ADD COLUMN created_at TEXT;
```

`id` is the ACP session id, a UUID v4 string
([`thread.rs:1430`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/agent/src/thread.rs#L1430);
`#[serde(transparent)]` newtype,
[`agent-client-protocol-schema-1.9.1/src/v1/mod.rs:48-51`](https://github.com/agentclientprotocol/agent-client-protocol/blob/7e87dc205a7325bd07d0249fd20bb7486ee6ba95/agent-client-protocol-schema/src/v1/mod.rs#L48-L51)).
`parent_id` is the subagent parent
([`db.rs:514-517`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/agent/src/db.rs#L514-L517)).
`summary` is the title. `updated_at` and `created_at` are `to_rfc3339()`, for example
`2026-01-02T03:04:05.123456789+00:00`
([`db.rs:513`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/agent/src/db.rs#L513),[`542`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/agent/src/db.rs#L542)).
`folder_paths` is a `PathList`, sorted paths joined by `\n`; `folder_paths_order` is
comma-separated display indices
([`crates/util/src/path_list.rs:130-146`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/util/src/path_list.rs#L130-L146)).
`data_type` is `zstd` or `json`
([`db.rs:363-367`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/agent/src/db.rs#L363-L367)).

#### Thread blob

Blob JSON is `DbThread` flattened plus `"version":"0.3.0"`
([`db.rs:53-89`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/agent/src/db.rs#L53-L89),[`189`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/agent/src/db.rs#L189),[`505-510`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/agent/src/db.rs#L505-L510)):
`title, messages[], updated_at (RFC 3339 "Z"), detailed_summary, initial_project_snapshot{worktree_snapshots:[{worktree_path, git_state:{remote_url, head_sha, current_branch, diff}}], timestamp}, cumulative_token_usage, request_token_usage{<user_msg_id>:{input_tokens, output_tokens, cache_creation_input_tokens, cache_read_input_tokens}}, model{provider, model}, profile, subagent_context{parent_thread_id, depth}, speed, thinking_enabled, thinking_effort, draft_prompt, ui_scroll_position, sandboxed_terminal_temp_dir, sandbox_grants{write_paths, network_hosts, ...}`
([`agent.rs:89-93`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/agent/src/agent.rs#L89-L93);
[`project/src/telemetry_snapshot.rs:33-45`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/project/src/telemetry_snapshot.rs#L33-L45);
[`legacy_thread.rs:45-49`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/agent/src/legacy_thread.rs#L45-L49);
[`language_model_core.rs:525-535`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/language_model_core/src/language_model_core.rs#L525-L535)).
The blob holds one `model` per thread, overwritten on a switch, and no message records a model.

`sandbox_grants` is `DbSandboxGrants`, the sandbox escalations the user approved "for the rest of
this thread" (the "Allow for this thread" prompt option): `write_paths` (granted write paths, each
covering its subtree), `network_hosts` (host patterns such as `*.npmjs.org`), and the booleans
`network_any_host`, `allow_fs_write_all`, `unsandboxed` and `sandbox_fallback` (commands allowed to
run unsandboxed because the OS sandbox could not be created), all `#[serde(default)]`
([`db.rs:84-122`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/agent/src/db.rs#L84-L122)).
A `write_paths` entry is a bare path string when no canonical target was resolved (and in legacy
rows), else an object `{requested, resolved, on_windows_fs?}`
([`granted_write_path.rs:64-87`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/settings/src/granted_write_path.rs#L64-L87)).
`profile` is the agent profile id the thread ran under
([`db.rs:69`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/agent/src/db.rs#L69)).

#### Messages

`messages[]` are externally tagged serde enums with no renames
([`thread.rs:202-208`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/agent/src/thread.rs#L202-L208),[`287-301`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/agent/src/thread.rs#L287-L301),[`743-759`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/agent/src/thread.rs#L743-L759)):

- `{"User":{"id":"<uuid>","content":[{"Text":"..."},{"Mention":{"uri":...,"content":"..."}},{"Image":{"source":"<base64 png>"}}]}}`
- `{"Agent":{"content":[{"Text":"..."},{"Thinking":{"text":"...","signature":null}},{"RedactedThinking":"..."},{"ToolUse":{"id":"toolu_1","name":"terminal","raw_input":"{...}","input":{"type":"json","value":{...}},"is_input_complete":true,"thought_signature":null}}],"tool_results":{"toolu_1":{"tool_use_id":"toolu_1","tool_name":"terminal","is_error":false,"content":[{"Text":"..."}],"output":{...}}},"reasoning_details":null}}`
  ([`language_model_core.rs:592-629`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/language_model_core/src/language_model_core.rs#L592-L629);
  [`request.rs:67-76`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/language_model_core/src/request.rs#L67-L76),[`148-152`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/language_model_core/src/request.rs#L148-L152);
  a legacy single-value `content` is accepted,
  [`request.rs:104-145`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/language_model_core/src/request.rs#L104-L145))
- `"Resume"`, `{"Compaction":{"Summary":"..."}}` or `{"Compaction":{"ProviderNative":{...}}}`.

#### Tool results of denied calls

At
[`9cd14da`](https://github.com/zed-industries/zed/commit/9cd14daad768fbfaeeac8789f515f9a5f67669a6)
an approval leaves no record in the thread. A denial fails the tool, and the error text becomes the
`tool_results` entry with `is_error` true
([`thread.rs:3958`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/agent/src/thread.rs#L3958),
[`5501-5512`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/agent/src/thread.rs#L5501-L5512)):
`Permission to run tool denied by user` for the user's denial
([`thread.rs:6905`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/agent/src/thread.rs#L6905)),
and for an automatic denial by the rules `Blocked by global default: deny`,
`Command blocked by security rule for <tool> tool`, `<tool> tool is disabled` or the built-in
`Blocked by built-in security rule`
([`crates/agent/src/tool_permissions.rs:12-13`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/agent/src/tool_permissions.rs#L12-L13),
[`292-298`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/agent/src/tool_permissions.rs#L292-L298),
[`396-430`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/agent/src/tool_permissions.rs#L396-L430)),
and three more automatic texts: `The terminal command could not be approved because ...` for a
command with shell substitutions, `The <tool> tool cannot run because <n> regex pattern(s) failed to
compile. ...` and `The <shell> shell does not support "always allow" patterns ...`
([`tool_permissions.rs:14-17`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/agent/src/tool_permissions.rs#L14-L17),
[`318-323`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/agent/src/tool_permissions.rs#L318-L323),
[`447-460`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/agent/src/tool_permissions.rs#L447-L460)).
The error becomes one `Text` content item holding the error's message
([`crates/language_model_core/src/request.rs:251-255`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/language_model_core/src/request.rs#L251-L255)).
A call cut off by the user is `Tool canceled by user`
([`thread.rs:72`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/agent/src/thread.rs#L72)).

#### sidebar_threads and archived worktrees

`db.sqlite` → `sidebar_threads` (final shape,
[`thread_metadata_store.rs:1424-1437`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/agent_ui/src/thread_metadata_store.rs#L1424-L1437),[`1459-1464`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/agent_ui/src/thread_metadata_store.rs#L1459-L1464)):

```sql
CREATE TABLE sidebar_threads(thread_id BLOB PRIMARY KEY, session_id TEXT, agent_id TEXT, title TEXT NOT NULL,
  updated_at TEXT NOT NULL, created_at TEXT, folder_paths TEXT, folder_paths_order TEXT, archived INTEGER DEFAULT 0,
  main_worktree_paths TEXT, main_worktree_paths_order TEXT, remote_connection TEXT) STRICT;
ALTER TABLE sidebar_threads ADD COLUMN interacted_at TEXT; ALTER TABLE sidebar_threads ADD COLUMN title_override TEXT;
CREATE TABLE archived_git_worktrees(id INTEGER PRIMARY KEY, worktree_path TEXT NOT NULL, main_repo_path TEXT NOT NULL,
  branch_name TEXT, staged_commit_hash TEXT, unstaged_commit_hash TEXT, original_commit_hash TEXT) STRICT;
CREATE TABLE thread_archived_worktrees(thread_id BLOB NOT NULL, archived_worktree_id INTEGER NOT NULL
  REFERENCES archived_git_worktrees(id), PRIMARY KEY (thread_id, archived_worktree_id)) STRICT;
```

`thread_id` is 16 raw UUID bytes
([`sqlez/src/bindable.rs:360-370`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/sqlez/src/bindable.rs#L360-L370));
`session_id` equals `threads.id`; `agent_id` NULL means the native Zed Agent, otherwise an external
ACP agent whose transcript is not in `threads.db`
([`thread_metadata_store.rs:1499-1500`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/agent_ui/src/thread_metadata_store.rs#L1499-L1500),[`1725-1727`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/agent_ui/src/thread_metadata_store.rs#L1725-L1727);
[`agent.rs:2742`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/agent/src/agent.rs#L2742));
times are RFC 3339
([`:1729-1740`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/agent_ui/src/thread_metadata_store.rs#L1729-L1740));
`remote_connection` is JSON, the SSH, WSL or Docker remote-development target, not a cloud id
([`remote_client.rs:1361-1367`](https://github.com/zed-industries/zed/blob/56934ec0560bce8929025497411de7f5322fb6ca/crates/remote/src/remote_client.rs#L1361-L1367));
`interacted_at` is the last user send.

### Joins and timestamps

`sidebar_threads.session_id = threads.id` (plus `threads.parent_id` for
subagents). Tool pairing inside one `Agent` message: `content[].ToolUse.id`
with the `tool_results` key or `tool_use_id`. Project path:
`threads.folder_paths` split on `\n`, fallback
`initial_project_snapshot.worktree_snapshots[].worktree_path`; branch from
`git_state.current_branch` or `archived_git_worktrees.branch_name` through
`thread_archived_worktrees`.

There are no per-message timestamps, only the thread's `updated_at` and
`created_at` and `initial_project_snapshot.timestamp`. Git branch is
`initial_project_snapshot.worktree_snapshots[].git_state.current_branch`,
captured at thread start.

### Other stores

#### SQLite journals and compression

`db.sqlite`: `PRAGMA journal_mode=WAL; synchronous=NORMAL`
([`crates/db/src/db.rs:129-133`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/db/src/db.rs#L129-L133);
[`sqlez/src/thread_safe_connection.rs:61-62`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/sqlez/src/thread_safe_connection.rs#L61-L62)),
so collect the `-wal` sidecar. `threads.db` is opened with plain `Connection::open_file` and no
journal pragma
([`sqlez/src/connection.rs:65`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/sqlez/src/connection.rs#L65)),
so it uses the default rollback journal and a `threads.db-journal` may exist mid-write. Blobs are
`zstd::encode_all(json, 3)` / `decode_all`
([`db.rs:178-184`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/agent/src/db.rs#L178-L184),[`535`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/agent/src/db.rs#L535),[`660`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/agent/src/db.rs#L660)):
standard zstd frames, no dictionary, content size present. In Python,
`zstandard.ZstdDecompressor().decompress(blob)`, falling back to `stream_reader` if the size header
is missing. Rows with `data_type='json'` are raw UTF-8. No encryption.

#### Other db.sqlite tables

Other per-user stores at
[`9cd14da`](https://github.com/zed-industries/zed/commit/9cd14daad768fbfaeeac8789f515f9a5f67669a6),
all in the same `db/0-<channel>/db.sqlite`, since every sqlez domain (Zed's name for one group of
tables with its migrations) opens that file
([`crates/db/src/db.rs:138`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/db/src/db.rs#L138),
[`166`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/db/src/db.rs#L166);
[`crates/db/src/kvp.rs:20-41`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/db/src/kvp.rs#L20-L41);
[`crates/agent_ui/src/thread_metadata_store.rs:1469`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/agent_ui/src/thread_metadata_store.rs#L1469);
[`crates/workspace/src/persistence.rs:986-991`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/workspace/src/persistence.rs#L986-L991),
[`1078`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/workspace/src/persistence.rs#L1078)).
They are the tables `trusted_worktrees`, `sidebar_terminal_threads` and `archived_git_worktrees`
and the key-value table `scoped_kv_store`, whose namespaces include `agent_draft_prompts` and
`created_git_worktrees`. The parser reads every one of them, each as a primary source, as
[Past-use rows and secondary sources](#past-use-rows-and-secondary-sources) describes.

| Source | Table and key | What it holds | Survives | Joins on | Plan |
| --- | --- | --- | --- | --- | --- |
| Draft prompts | `scoped_kv_store(namespace, key, value)`, namespace `agent_draft_prompts`, key the sidebar `thread_id` ([`kvp.rs:31-36`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/db/src/kvp.rs#L31-L36); [`agent_ui/src/draft_prompt_store.rs:25-58`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/agent_ui/src/draft_prompt_store.rs#L25-L58)) | the unsent prompt of a draft thread, a JSON array of ACP `ContentBlock`s, internally tagged by `type` (`text` with `text`, `image`, `audio`, `resource_link` with `name` and `uri`, `resource` with an embedded `resource`) ([`agent-client-protocol-schema/src/v2/content.rs:84-111`](https://github.com/agentclientprotocol/agent-client-protocol/blob/e2d83349bc9e28a4059ea3acefa1c91c966f74e2/agent-client-protocol-schema/src/v2/content.rs#L84-L111), schema 1.12.0, the version in `Cargo.lock` at `9cd14da`); the key is the `thread_id` UUID in hyphenated form ([`thread_metadata_store.rs:41-44`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/agent_ui/src/thread_metadata_store.rs#L41-L44)); a draft is a `sidebar_threads` row with `session_id` NULL ([`draft_prompt_store.rs:1-5`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/agent_ui/src/draft_prompt_store.rs#L1-L5)) | nothing: deleted with the sidebar row ([`thread_metadata_store.rs:1141-1164`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/agent_ui/src/thread_metadata_store.rs#L1141-L1164)) | `key` to `sidebar_threads.thread_id` | primary, read: text no other store holds |
| Zed-created worktrees | `scoped_kv_store` namespace `created_git_worktrees`, key `<host>` newline `<worktree path>` (`local` for this machine), value `{created_at_seconds, created_at_subsec_nanos}` ([`git_ui_core/src/created_worktrees.rs:34-73`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/git_ui_core/src/created_worktrees.rs#L34-L73)) | each linked git worktree Zed made for a thread, with its creation time | the thread's deletion; removed only by `forget_created_worktree` ([`created_worktrees.rs:142`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/git_ui_core/src/created_worktrees.rs#L142)) | worktree path to `sidebar_threads.folder_paths` | primary, read: the scratch marker |
| Archived worktrees | `archived_git_worktrees`, `thread_archived_worktrees` ([Record schema](#record-schema)) | for a thread archived from a Zed-created worktree: the worktree and main repository paths, the branch, and the staged, unstaged and original commit ids; the worktree directory is removed and the WIP commits stay in the main repository under `refs/archived-worktrees/<id>` ([`agent_ui/src/thread_worktree_archive.rs:77-79`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/agent_ui/src/thread_worktree_archive.rs#L77-L79), [`499-607`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/agent_ui/src/thread_worktree_archive.rs#L499-L607)) | the worktree's removal; dropped with the ref when the last thread linked to it is deleted ([`thread_worktree_archive.rs:823-912`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/agent_ui/src/thread_worktree_archive.rs#L823-L912)) | `thread_archived_worktrees.thread_id` to `sidebar_threads.thread_id` | primary, read |
| Sidebar terminals | `sidebar_terminal_threads(terminal_id, title, custom_title, created_at, working_directory, folder_paths, folder_paths_order, main_worktree_paths, main_worktree_paths_order, remote_connection)` ([`agent_ui/src/terminal_thread_metadata_store.rs:483-500`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/agent_ui/src/terminal_thread_metadata_store.rs#L483-L500)) | a terminal tab opened in the threads sidebar: title, start time (RFC 3339, [`terminal_thread_metadata_store.rs:519`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/agent_ui/src/terminal_thread_metadata_store.rs#L519)) and working directory, no output | not determined | `folder_paths` | primary, read, metadata only |
| Native sidebar rows | `sidebar_threads` rows with `agent_id` NULL ([Record schema](#record-schema)) | title, `title_override`, `archived`, `interacted_at` and `main_worktree_paths` of a native thread | deleted with the thread | `session_id` to `threads.id` | primary, read as thread state: it repeats no message of `threads.db` and is deleted with the thread, so it is not a secondary source |

The `trusted_worktrees(trust_id, absolute_path, user_name, host_name)`
table of the same file is permission state, not past use
([Permission settings and project trust](#permission-settings-and-project-trust)).

#### Permission settings and project trust

At
[`9cd14da`](https://github.com/zed-industries/zed/commit/9cd14daad768fbfaeeac8789f515f9a5f67669a6)
the user's `settings.json` (and `global_settings.json`, which parses the same way) holds
`agent.tool_permissions.default` and, per tool name (`terminal`, `edit_file`, `fetch`,
`mcp:<server>:<tool>` and so on), `tools.<tool>.default` and the regex lists `always_allow`,
`always_deny` and `always_confirm` of `{pattern, case_sensitive}`, with the modes `allow`, `deny`
and `confirm`
([`crates/settings_content/src/agent.rs:397-412`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/settings_content/src/agent.rs#L397-L412),
[`1067-1140`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/settings_content/src/agent.rs#L1067-L1140);
[`assets/settings/default.json:1235-1275`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/assets/settings/default.json#L1235-L1275)),
and `agent.sandbox_permissions` holds `allow_all_hosts`, `network_hosts[]`, `allow_fs_write_all`,
`allow_unsandboxed` and `write_paths[]` (a path or `{requested, resolved}`)
([`agent.rs:1013-1065`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/settings_content/src/agent.rs#L1013-L1065)).
The same keys can also sit in a user file's release-channel blocks (`dev`, `nightly`, `preview`,
`stable`), its platform blocks (`macos`, `linux`, `windows`) and each `profiles.<name>.settings`,
each a whole settings object of its own
([`crates/settings_content/src/settings_content.rs:459-469`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/settings_content/src/settings_content.rs#L459-L469),
[`501-514`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/settings_content/src/settings_content.rs#L501-L514)).
A project's `.zed/settings.json` cannot hold them: it is parsed as `ProjectSettingsContent`, which
has no `agent`, `agent_servers` or `session` key
([`crates/settings_content/src/project.rs:44`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/settings_content/src/project.rs#L44);
[`crates/settings/src/settings_store.rs:1104-1113`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/settings/src/settings_store.rs#L1104-L1113)).

Zed writes these itself: an "Always allow" or "Always deny" choice on a tool prompt (option ids
`always_allow:<tool>`, `always_deny:<tool>`, `always_allow_mcp:<tool>`, `always_deny_mcp:<tool>`)
appends the command's sub-patterns to `always_allow` or `always_deny`, or sets the tool's `default`
([`crates/agent/src/thread.rs:6899-7005`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/agent/src/thread.rs#L6899-L7005)),
and "Allow always" on a sandbox escalation writes `sandbox_permissions`
([`thread.rs:6368-6440`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/agent/src/thread.rs#L6368-L6440),
[`6658-6673`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/agent/src/thread.rs#L6658-L6673)).
The older boolean `agent.always_allow_tool_actions` is migrated to
`tool_permissions.default: "allow"` and no longer read
([`crates/migrator/src/migrations/m_2026_02_04/settings.rs:7`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/migrator/src/migrations/m_2026_02_04/settings.rs#L7)).
An external agent's own mode (Claude Agent's permission mode, for one) is
`agent_servers.<id>.default_mode`
([`agent.rs:854-859`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/settings_content/src/agent.rs#L854-L859)).

Project trust, which gates a project's settings, its language and MCP servers and its
`.agents/skills`, is one `trusted_worktrees` row per trusted path, with `user_name` and `host_name`
for a remote host, in `db.sqlite`, rewritten whole on every change
([`crates/workspace/src/persistence.rs:986-991`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/workspace/src/persistence.rs#L986-L991),
[`2590-2632`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/workspace/src/persistence.rs#L2590-L2632);
[`crates/agent/src/agent.rs:1309-1317`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/agent/src/agent.rs#L1309-L1317)),
and `session.trust_all_worktrees` (default `false`) skips the check
([`crates/settings_content/src/project.rs:498-503`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/settings_content/src/project.rs#L498-L503)).

#### Telemetry log and crash reports

Telemetry (`telemetry.metrics`, default `true`,
[`settings_content.rs:610-633`](https://github.com/zed-industries/zed/blob/56934ec0560bce8929025497411de7f5322fb6ca/crates/settings_content/src/settings_content.rs#L610-L633))
sends events to `/telemetry/events` and appends each one to `telemetry.log` in the logs directory
(`Library/Logs/Zed` or `<data>/logs`, recreated empty at every launch,
[`telemetry.rs:214-223`](https://github.com/zed-industries/zed/blob/56934ec0560bce8929025497411de7f5322fb6ca/crates/client/src/telemetry.rs#L214-L223),
[`284-286`](https://github.com/zed-industries/zed/blob/56934ec0560bce8929025497411de7f5322fb6ca/crates/client/src/telemetry.rs#L284-L286),
[`637-680`](https://github.com/zed-industries/zed/blob/56934ec0560bce8929025497411de7f5322fb6ca/crates/client/src/telemetry.rs#L637-L680)).
The agent panel's events carry ids, model, status and token counts, not text, for example
`Agent Thread Completion`
([`thread.rs:3039-3049`](https://github.com/zed-industries/zed/blob/56934ec0560bce8929025497411de7f5322fb6ca/crates/agent/src/thread.rs#L3039-L3049));
the inline assistant's `Inline Assistant Rated` event, sent on its thumbs buttons, carries the
prompt, the completion and the selected text
([`inline_prompt_editor.rs:640-651`](https://github.com/zed-industries/zed/blob/56934ec0560bce8929025497411de7f5322fb6ca/crates/agent_ui/src/inline_prompt_editor.rs#L640-L651)),
so a `telemetry.log` from the current launch can hold inline-assistant text, never panel-thread
text.

`telemetry.diagnostics` (default `true`) uploads crash minidumps (a process memory snapshot, `*.dmp`
beside a `*.json` metadata file in the logs directory) to Sentry, the crash service, and deletes
both once the upload succeeds
([`reliability.rs:51-54`](https://github.com/zed-industries/zed/blob/56934ec0560bce8929025497411de7f5322fb6ca/crates/zed/src/reliability.rs#L51-L54),
[`259-295`](https://github.com/zed-industries/zed/blob/56934ec0560bce8929025497411de7f5322fb6ca/crates/zed/src/reliability.rs#L259-L295));
no thread file is attached, though the memory image may hold whatever the panel had loaded.

### Format versions

Blob `version` `"0.3.0"` is current
([`db.rs:189`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/agent/src/db.rs#L189));
`"0.2.0"` and `"0.1.0"` are
`SerializedThread {version, summary, updated_at, messages:[{id:int, role, segments:[{type:"text",text}|{type:"thinking",text,signature?}| {type:"RedactedThinking",data}], tool_uses:[{id,name,input}], tool_results:[{tool_use_id,is_error,content,output}], context, creases, is_hidden}], initial_project_snapshot, cumulative_token_usage, request_token_usage:[...], detailed_summary_state, model{provider,model}, profile}`
([`legacy_thread.rs:23-164`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/agent/src/legacy_thread.rs#L23-L164));
no `version` key means `LegacySerializedThread` with `messages[].text`
([`:166-205`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/agent/src/legacy_thread.rs#L166-L205)).
Upgrade logic
[`db.rs:195-330`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/agent/src/db.rs#L195-L330).
Shared and exported threads use `SharedThread` version `"1.0.0"`
([`db.rs:130-141`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/agent/src/db.rs#L130-L141)).
At [`9cd14da`](https://github.com/zed-industries/zed/commit/9cd14daad768fbfaeeac8789f515f9a5f67669a6) the shapes the parser reads are unchanged
from `a846890`, with line numbers moved by one or two: the `threads` DDL
([`db.rs:446-487`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/agent/src/db.rs#L446-L487)),
blob `version` `0.3.0`
([`db.rs:189`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/agent/src/db.rs#L189)),
`DbThread` and `DbSandboxGrants`
([`db.rs:53-122`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/agent/src/db.rs#L53-L122)),
the message enums
([`thread.rs:202-217`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/agent/src/thread.rs#L202-L217),
[`287-301`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/agent/src/thread.rs#L287-L301),
[`743-759`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/agent/src/thread.rs#L743-L759)),
`LanguageModelToolUse` and `LanguageModelToolResult`
([`crates/language_model_core/src/language_model_core.rs:593-602`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/language_model_core/src/language_model_core.rs#L593-L602);
[`crates/language_model_core/src/request.rs:68-76`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/language_model_core/src/request.rs#L68-L76))
and the `sidebar_threads` migrations
([`thread_metadata_store.rs:1372-1465`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/agent_ui/src/thread_metadata_store.rs#L1372-L1465)).

`sidebar_threads` is migrated in place
([`thread_metadata_store.rs:1373-1465`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/agent_ui/src/thread_metadata_store.rs#L1373-L1465));
cross-channel import copies rows between `0-<channel>` databases
([`thread_import.rs`](https://github.com/zed-industries/zed/blob/a84689073d296dfd39987bc7dd478e43ef76d83a/crates/agent_ui/src/thread_import.rs)).

`SharedThread` is used only by the agent panel's two debugging actions, `CopyThreadToClipboard` (the
thread as base64 JSON on the clipboard) and `LoadThreadFromClipboard`
([`agent_ui.rs:332-335`](https://github.com/zed-industries/zed/blob/56934ec0560bce8929025497411de7f5322fb6ca/crates/agent_ui/src/agent_ui.rs#L332-L335);
[`agent_panel.rs:3792-3826`](https://github.com/zed-industries/zed/blob/56934ec0560bce8929025497411de7f5322fb6ca/crates/agent_ui/src/agent_panel.rs#L3792-L3826),
[`3852-3929`](https://github.com/zed-industries/zed/blob/56934ec0560bce8929025497411de7f5322fb6ca/crates/agent_ui/src/agent_panel.rs#L3852-L3929));
nothing is sent over the network. A thread loaded that way is saved to `threads.db` under a new UUID
session id
([`agent_panel.rs:3894`](https://github.com/zed-industries/zed/blob/56934ec0560bce8929025497411de7f5322fb6ca/crates/agent_ui/src/agent_panel.rs#L3894)),
its title prefixed `🔗 ` and with no `initial_project_snapshot`, `profile` or `sandbox_grants`
([`db.rs:153-172`](https://github.com/zed-industries/zed/blob/56934ec0560bce8929025497411de7f5322fb6ca/crates/agent/src/db.rs#L153-L172)),
so the prefix marks a thread pasted in from elsewhere rather than run on this host.

### Secrets

Blob fields `sandbox_grants.write_paths`/`network_hosts`, `draft_prompt`,
`Mention.content` (inlined file contents), `initial_project_snapshot.git_state.diff`
(full diff), and `remote_connection` (host and user) in `sidebar_threads`. No
API keys were found in thread storage; the settings files that can hold them are in
[Credentials](#credentials).

### Parser plan

Globs: `{Library/Application Support/Zed,.local/share/zed,AppData/Local/Zed}/threads/threads.db*`,
`.../db/0-*/db.sqlite*`, and `{.config/zed,AppData/Roaming/Zed}/{settings,global_settings}.json`
(with the Flatpak `.var/app/dev.zed.Zed*/{data,config}/zed` forms). Requires the `zstandard`
package.

Rows: one per message content item and per `tool_results` entry; one
`system` row per `sidebar_threads` entry with `agent_id` set (external agent,
no transcript); the `system` rows of the subsections below.

| Column | Source |
| --- | --- |
| timestamp_utc | thread `updated_at` for all messages (approximate); `initial_project_snapshot.timestamp` for the thread-start row |
| session_id | `threads.id` |
| project_path | first of `folder_paths.split("\n")` |
| git_branch | `git_state.current_branch` |
| turn_type | `User.Text/Mention/Image` → user; `Agent.Text` → assistant; `Thinking`/`RedactedThinking` → thinking; `ToolUse` → tool_use; `tool_results[*]` → tool_result; `Resume`/`Compaction` → system |
| model | `model.provider/model.model` |
| tool_name, tool_use_id | `ToolUse.name` / `tool_name`, `ToolUse.id` / `tool_use_id` |
| text | text, `arguments` JSON, result text; `Mention.uri` for mentions |

The thread start is followed, at its time, by `git remote: <url>` (`system`), the
`git_state.remote_url` of the thread blob's `initial_project_snapshot` worktree whose
`worktree_path` is the thread's project, else of the first worktree, as for `git_branch`
([Record schema](#record-schema); [issue 253](https://github.com/seanthegeek/doubleagent/issues/253)).
Source-derived; not observed on any real install.

Model and permission rows ([issue 96](https://github.com/seanthegeek/doubleagent/issues/96) and
[issue 97](https://github.com/seanthegeek/doubleagent/issues/97)), all `system`, come after the
thread-start row and before the first message. The rows added for
[issue 196](https://github.com/seanthegeek/doubleagent/issues/196) rest on source at
[`9cd14da`](https://github.com/zed-industries/zed/commit/9cd14daad768fbfaeeac8789f515f9a5f67669a6)
alone and a fixture.

#### Permission rows

Thread grants: `permission setting: <key>=<value> (thread blob, as of updated_at)`, one
row per `profile`, per `sandbox_grants.write_paths[]` entry (a string, or
the object printed as compact JSON), per `sandbox_grants.network_hosts[]`
entry, and per `sandbox_grants` boolean (`network_any_host`,
`allow_fs_write_all`, `unsandboxed`, `sandbox_fallback`) that is `true`.
The grants live in the thread blob, so `session_id` is the thread id and
`timestamp_utc` the thread's `updated_at` (the blob's last write; the
moment of the grant is not recorded).

Standing rules, from
[Permission settings and project trust](#permission-settings-and-project-trust), source-derived;
not observed on any real install. The parser reads `settings.json` and `global_settings.json` in
the config directory as JSON with comments and trailing commas (Zed parses them with
`parse_json_with_comments`,
[`settings_content.rs:450-457`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/settings_content/src/settings_content.rs#L450-L457))
and writes one `permission setting: <key>=<value> (from <file>, file mtime)` row per leaf of
`agent.tool_permissions` and `agent.sandbox_permissions` (each list entry its own row, an
`always_allow`, `always_deny` or `always_confirm` rule `{pattern, case_sensitive}` as compact
JSON), per `agent.always_allow_tool_actions` (the legacy boolean, should a file still hold it),
`agent_servers.<id>.default_mode` and `session.trust_all_worktrees`, in the top level and in each
override block and profile, whose name prefixes the key (`stable.agent.tool_permissions.default`),
at the file's modification time with an empty `session_id`. A project's `.zed/settings.json` is not
read, since it cannot hold these keys. Each `trusted_worktrees` row of `db.sqlite` is
`permission setting: trusted_worktrees.<path>=true[ host=<user_name>@<host_name>] (from <file>
trusted_worktrees, file mtime)`, `project_path` the path.

Per call, from [Tool results of denied calls](#tool-results-of-denied-calls), source-derived; not
observed on any real install: a failed `tool_result` (`is_error` true) whose whole text is
`Permission to run tool denied by user` is followed by `permission decision: denied`, and one whose
text is an automatic denial by `permission decision: auto_denied reason=<text>`, each with the
call's `tool_name` and `tool_use_id`. The sandbox-escalation prompt's denial writes the same user
text
([`thread.rs:6393`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/agent/src/thread.rs#L6393),
[`6398`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/agent/src/thread.rs#L6398)).
An approval, once or always, leaves no record in the thread and has no row; an "Always allow" or
"Always deny" choice shows only as the settings rows it wrote.

#### Model rows

Covered: `model: <model.model> provider=<model.provider>` once per thread at the
thread-start time, from the blob's `model`. The blob holds one `model`
per thread, so a switch inside a thread is not recorded and there is no
change row; no message records a model ([Thread blob](#thread-blob)).

#### Past-use rows and secondary sources

Every store of [Other db.sqlite tables](#other-dbsqlite-tables) is read as a primary source, all
`system` rows, source-derived; not observed on any real install:

- A draft prompt is `unsent draft: <text>` at its sidebar row's `updated_at`, with its first
  folder as `project_path` and no `session_id` (a draft has none), or, when the sidebar row is gone,
  `unsent draft: <text> (from <file> agent_draft_prompts, file mtime)` at the file's modification
  time. The text joins the ACP blocks: `text` as written, `resource_link` and `resource` as
  `[mention] <uri>`, `image` and `audio` as markers. The thread blob's own `draft_prompt` (same
  shape) is `unsent draft: <text> (thread blob, as of updated_at)` after the thread's messages.
- An archived worktree is `archived worktree: <worktree_path> of <main_repo_path> branch=<b>
  staged=<hash> unstaged=<hash> original=<hash> ref=refs/archived-worktrees/<id> (from <file>
  archived_git_worktrees, file mtime)`, one per thread linked to it with that thread's
  `session_id`, `project_path` the main repository; the archive time is not recorded.
- A sidebar terminal is `sidebar terminal: <custom_title or title> cwd=<working_directory>` at its
  `created_at`, with its first folder as `project_path` and no `session_id`.
- A native `sidebar_threads` row with a `session_id` is `sidebar thread: title=<title>
  archived=<true|false> interacted_at=<time|-> main_worktree=<path|->` at its `updated_at`, with
  the thread id: the archive flag, the last user send and the main repository of a thread run in a
  linked worktree, none of which `threads.db` holds.

None of these repeats a message of `threads.db`, so the parser has no secondary source.

#### Sync, relay and origin rows

Zed records no sync state; no thread is synced to the account, shared by link or kept on a server as
a copy of `threads.db`. Thread text does leave the host in one place beyond the model call: rating a
thread. A thumbs-up or thumbs-down on a native Zed Agent thread (the buttons, or the `/helpful` and
`/not-helpful` commands in the message box,
[`completion_provider.rs:205-210`](https://github.com/zed-industries/zed/blob/56934ec0560bce8929025497411de7f5322fb6ca/crates/agent_ui/src/completion_provider.rs#L205-L210))
posts the whole thread, the `DbThread` that `threads.db` stores, serialised as JSON, with the
session id, parent session id, rating and organisation id to `/client/feedback/agent_thread`; the
comment box that follows a thumbs-down posts the thread again with the comment to
`/client/feedback/agent_thread_comments`
([`thread_view.rs:69-123`](https://github.com/zed-industries/zed/blob/56934ec0560bce8929025497411de7f5322fb6ca/crates/agent_ui/src/conversation_view/thread_view.rs#L69-L123),
[`125-165`](https://github.com/zed-industries/zed/blob/56934ec0560bce8929025497411de7f5322fb6ca/crates/agent_ui/src/conversation_view/thread_view.rs#L125-L165);
[`cloud_api_client.rs:272-297`](https://github.com/zed-industries/zed/blob/56934ec0560bce8929025497411de7f5322fb6ca/crates/cloud_api_client/src/cloud_api_client.rs#L272-L297);
[`cloud_api_types.rs:107-123`](https://github.com/zed-industries/zed/blob/56934ec0560bce8929025497411de7f5322fb6ca/crates/cloud_api_types/src/cloud_api_types.rs#L107-L123);
the payload is
[`agent.rs:3552-3567`](https://github.com/zed-industries/zed/blob/56934ec0560bce8929025497411de7f5322fb6ca/crates/agent/src/agent.rs#L3552-L3567)).
The tooltip says so: "Rating the thread sends all of your current conversation to the Zed team"
([`thread_view.rs:7560`](https://github.com/zed-industries/zed/blob/56934ec0560bce8929025497411de7f5322fb6ca/crates/agent_ui/src/conversation_view/thread_view.rs#L7560)).

Rating needs a signed-in Zed account
([`cloud_api_client.rs:182-194`](https://github.com/zed-industries/zed/blob/56934ec0560bce8929025497411de7f5322fb6ca/crates/cloud_api_client/src/cloud_api_client.rs#L182-L194)),
happens only on a click, and is offered while the `agent.enable_feedback` setting is `true`, its
default
([`agent.rs:352-355`](https://github.com/zed-industries/zed/blob/56934ec0560bce8929025497411de7f5322fb6ca/crates/settings_content/src/agent.rs#L352-L355);
[`default.json:1294-1295`](https://github.com/zed-industries/zed/blob/56934ec0560bce8929025497411de7f5322fb6ca/assets/settings/default.json#L1294-L1295)),
unless the organisation's configuration sets `is_agent_thread_feedback_enabled` false
([`thread_view.rs:7650-7663`](https://github.com/zed-industries/zed/blob/56934ec0560bce8929025497411de7f5322fb6ca/crates/agent_ui/src/conversation_view/thread_view.rs#L7650-L7663)).
External ACP agents' threads cannot be rated: only the native connection implements the
`AgentTelemetry` hook that supplies the thread
([`agent.rs:3450-3452`](https://github.com/zed-industries/zed/blob/56934ec0560bce8929025497411de7f5322fb6ca/crates/agent/src/agent.rs#L3450-L3452)).
Nothing on disk records that a thread was rated: the rating is held in the view's memory
([`thread_view.rs:63-66`](https://github.com/zed-industries/zed/blob/56934ec0560bce8929025497411de7f5322fb6ca/crates/agent_ui/src/conversation_view/thread_view.rs#L63-L66)),
`DbThread` has no field for it
([`db.rs:53-89`](https://github.com/zed-industries/zed/blob/56934ec0560bce8929025497411de7f5322fb6ca/crates/agent/src/db.rs#L53-L89)),
and the request bypasses the telemetry queue, so it is not in `telemetry.log`
([Telemetry log and crash reports](#telemetry-log-and-crash-reports)). So this is an upload with no
local marker, and no row is proposed; a copy already sent with a rating stays with Zed after a
delete, and nothing local says so.

The Zed-hosted model provider receives the thread id with each request
([`thread.rs:4124`](https://github.com/zed-industries/zed/blob/56934ec0560bce8929025497411de7f5322fb6ca/crates/agent/src/thread.rs#L4124);
[`cloud_llm_client.rs:220-228`](https://github.com/zed-industries/zed/blob/56934ec0560bce8929025497411de7f5322fb6ca/crates/cloud_llm_client/src/cloud_llm_client.rs#L220-L228)),
which is the model call, not sync. The collaboration server has a `SharedThreadId` type that nothing
uses
([`ids.rs:96-106`](https://github.com/zed-industries/zed/blob/56934ec0560bce8929025497411de7f5322fb6ca/crates/collab/src/db/ids.rs#L96-L106)),
the collaboration protocol has no thread message (`crates/proto/proto/ai.proto` carries only agent
and context-server commands), and the `remote_connection` column of `sidebar_threads` is not a cloud
id ([sidebar_threads and archived worktrees](#sidebar_threads-and-archived-worktrees)). At
[`9cd14da`](https://github.com/zed-industries/zed/commit/9cd14daad768fbfaeeac8789f515f9a5f67669a6)
the rating upload is still the only one: a grep for `share_thread`, `ShareThread`, `sync_thread`,
`upload_thread` and `cloud_thread` finds no other. The agent records no relay and no origin.

#### Scratch directory rows

Source-derived; not observed on any real install. Both
[Scratch directories](#scratch-directories) are marked with a `system` row that names the
directory in its own `project_path`:

- A thread blob's `sandboxed_terminal_temp_dir` is `scratch directory: <path> (sandboxed terminal
  temp directory of thread <id>) (thread blob, as of updated_at)`, after the grant rows, at the
  thread's `updated_at` (when the directory was made is not recorded) and with no `session_id`, so
  the thread stays counted under its project.
- A `created_git_worktrees` entry is `scratch directory: <worktree path> (Zed-created git
  worktree[ of <main>][ host=<host>]) (from <file> created_git_worktrees, created_at)` at the
  recorded creation time, with no `session_id`; `<main>` is the `main_worktree_paths` of a
  `sidebar_threads` row whose `folder_paths` holds the worktree, else the `main_repo_path` of its
  `archived_git_worktrees` row, and `host` appears for a remote worktree. The thread run in the
  worktree keeps the worktree as its `project_path`, since the two files are parsed apart, so its
  rows count under the scratch directory.

The macOS temp directory is outside the home and not collected
([Scratch directories](#scratch-directories)); the row is the only trace the analyzer shows.

#### Deletion rows

The agent records none: nothing local records a deletion. What a delete removes and what
survives is in [What survives a delete](#what-survives-a-delete).

#### Bad-line rows

Implemented ([issue 168](https://github.com/seanthegeek/doubleagent/issues/168)). The parser reads
no file line by line; its bad records are whole files, `threads` rows and items
inside a thread blob. A `settings.json` or `global_settings.json` that cannot be read is one row
`parser: <file> could not be read (<reason>)`, `<file>` its name, with `source_line` 0 and an empty
time. A database file that cannot be read, or whose `-wal`, `-shm` or `-journal`
sidecar cannot, is one row `parser: <file> could not be read (<reason>)`, `<file>` the name of the
file that failed; one that is not a SQLite database is `parser: not a SQLite database`, and a SQLite
error while reading one `parser: SQLite error: <error>` row. A `data` blob that does not decode to a
JSON object is `parser: unparseable record <rowid> (<reason>): <stored value>`, the stored value
being a `json` row's text or a `zstd` row's compressed bytes as lowercase hex (a truncated frame can
decompress to nothing), the reason then ending `stored zstd blob as hex`.

A `messages[]` item of no known variant, a `Compaction` with
neither `Summary` nor `ProviderNative`, an `Agent` content item of no
known variant, a content or legacy `segments[]` item of no known type,
and a `tool_uses[]` or `tool_results` entry that is not an object is
`parser: unparseable record <N> (<reason>): <item as JSON>`, N the
message's 1-based position in `messages[]` and the reason naming the
item (`unknown message variant`, `unknown Agent content variant,
content[1]`, `Agent tool_results entry <id> is not an object`). Every
such row is `system`, carries the thread's `session_id` and
`project_path`, an empty `timestamp_utc` and the `threads` rowid as
`source_line`, and follows the thread's other rows. An unknown `User`
content object stays in the prompt text as JSON.

A `scoped_kv_store` value that does not decode (a draft prompt that is not JSON, a worktree
record without `created_at_seconds`) is `parser: unparseable record <rowid> (<namespace>[ <key>]:
<error type>: <reason>): <stored value>` at the end of the file's rows, with an empty
`timestamp_utc` and the store's rowid as `source_line`; a settings file that does not decode to a
JSON object is one `parser: <file> is not a JSON object` row, and an empty one yields nothing.
These are source-derived; not observed on any real install.

#### Fixtures

`threads.db` (`data` is the zstd of the JSON below,
`data_type='zstd'`):

```sql
INSERT INTO threads(id,parent_id,folder_paths,folder_paths_order,summary,updated_at,data_type,data,created_at)
VALUES ('6f1c0b2e-1111-4bbb-8ccc-000000000001',NULL,'/home/alice/proj','0','Fix flaky test','2026-03-01T09:01:05.000000000+00:00','zstd',X'<zstd>','2026-03-01T09:00:00.000000000+00:00');
```

```json
{"version":"0.3.0","title":"Fix flaky test","updated_at":"2026-03-01T09:01:05Z",
 "initial_project_snapshot":{"worktree_snapshots":[{"worktree_path":"/home/alice/proj","git_state":{"remote_url":"git@github.com:alice/proj.git","head_sha":"abc123","current_branch":"main","diff":null}}],"timestamp":"2026-03-01T09:00:00Z"},
 "model":{"provider":"anthropic","model":"claude-sonnet-4-5"},
 "messages":[
  {"User":{"id":"9a7d6c5b-2222-4ddd-9eee-000000000002","content":[{"Text":"run the tests"},{"Mention":{"uri":"file:///home/alice/proj/README.md","content":""}}]}},
  {"Agent":{"content":[{"Thinking":{"text":"use pytest","signature":null}},{"ToolUse":{"id":"toolu_01","name":"terminal","raw_input":"{\"command\":\"pytest -q\"}","input":{"type":"json","value":{"command":"pytest -q"}},"is_input_complete":true,"thought_signature":null}},{"Text":"All 3 tests pass."}],
   "tool_results":{"toolu_01":{"tool_use_id":"toolu_01","tool_name":"terminal","is_error":false,"content":[{"Text":"3 passed"}],"output":null}},"reasoning_details":null}},
  "Resume"]}
```

`db.sqlite`:

```sql
INSERT INTO sidebar_threads(thread_id,session_id,agent_id,title,updated_at,created_at,folder_paths,folder_paths_order,archived)
VALUES (randomblob(16),'6f1c0b2e-1111-4bbb-8ccc-000000000001',NULL,'Fix flaky test','2026-03-01T09:01:05+00:00','2026-03-01T09:00:00+00:00','/home/alice/proj','0',0);
INSERT INTO sidebar_threads(thread_id,session_id,agent_id,title,updated_at,folder_paths,folder_paths_order)
VALUES (randomblob(16),'ext-claude-code-sess-1','claude-code','External thread','2026-03-01T10:00:00+00:00','/home/alice/proj','0');
```

The other `db.sqlite` stores, as `9cd14da` creates them (a draft, a thread run in a Zed-created
worktree and archived, project trust and a sidebar terminal):

```sql
INSERT INTO sidebar_threads(thread_id,session_id,agent_id,title,updated_at,folder_paths,folder_paths_order,archived,main_worktree_paths,interacted_at)
VALUES (X'0b5e2d7a44444ddd9eee000000000004',NULL,NULL,'New Thread','2026-03-01T11:00:00+00:00','/home/alice/proj','0',0,NULL,NULL),
       (X'<16 bytes>','6f1c0b2e-3333-4bbb-8ccc-000000000003',NULL,'Fix tests in a worktree','2026-03-01T12:00:00+00:00','/home/alice/worktrees/fix-tests/proj','0',1,'/home/alice/proj','2026-03-01T11:59:00+00:00');
INSERT INTO scoped_kv_store(namespace,key,value) VALUES
 ('agent_draft_prompts','0b5e2d7a-4444-4ddd-9eee-000000000004','[{"type":"text","text":"refactor the parser"},{"type":"resource_link","name":"main.py","uri":"file:///home/alice/proj/main.py"}]'),
 ('created_git_worktrees','local' || char(10) || '/home/alice/worktrees/fix-tests/proj','{"created_at_seconds":1772362800,"created_at_subsec_nanos":500000000}');
INSERT INTO trusted_worktrees(absolute_path,user_name,host_name) VALUES
 ('/home/alice/proj',NULL,NULL), ('/home/alice/remote','alice','build.example');
INSERT INTO archived_git_worktrees VALUES
 (1,'/home/alice/worktrees/fix-tests/proj','/home/alice/proj','fix-tests','a1b2c3','d4e5f6','0a0b0c');
INSERT INTO thread_archived_worktrees VALUES (X'<16 bytes>',1);
INSERT INTO sidebar_terminal_threads(terminal_id,title,created_at,working_directory,folder_paths,folder_paths_order)
VALUES ('7c1d0e2f-6666-4fff-8aaa-000000000006','zsh','2026-03-01T10:30:00+00:00','/home/alice/proj','/home/alice/proj','0');
```

The thread blob's 9cd14da fields: `"sandboxed_terminal_temp_dir":
"/private/var/folders/xy/abc/T/zed-agent-terminal-Ab12Cd"`, `"draft_prompt":
[{"type":"text","text":"and then lint"}]`, and a denied call's result
`{"tool_use_id":"toolu_02","tool_name":"terminal","is_error":true,
"content":[{"Text":"Permission to run tool denied by user"}]}`.

`settings.json` (JSON with comments):

```json
{"agent": {"tool_permissions": {"default": "confirm", "tools": {"terminal": {
   "always_allow": [{"pattern": "^cargo test"}],
   "always_deny": [{"pattern": "^rm -rf", "case_sensitive": false}]}}},
  "sandbox_permissions": {"network_hosts": ["*.npmjs.org"], "allow_unsandboxed": false}},
 "agent_servers": {"claude-acp": {"type": "registry", "default_mode": "plan"}},
 "session": {"trust_all_worktrees": false},
 "stable": {"agent": {"tool_permissions": {"default": "allow"}}}}
```

#### Source host rows

The agent records no source-host integration. A grep of the agent, agent UI and ACP server crates
at [`9cd14da`](https://github.com/zed-industries/zed/commit/9cd14daad768fbfaeeac8789f515f9a5f67669a6)
for `github`, `gitlab`, `bitbucket`, `oauth`, `installation`, `repositor`, `pull request` and
`clone` finds no connected source-host account, no cached repository list and no
repository-bound remote conversation: the hits are the local repository a thread runs in, context
server (MCP) extension installs and host patterns in sandbox examples. The one GitHub token
Zed holds is the GitHub Copilot model provider's `read:user` grant, a model credential kept in
the keychain ([Credentials](#credentials)), not a grant to act on repositories; a project's own
git remote, which the thread blob's `git_state.remote_url` records, is not an integration.

## Confidence

- High: every path from `paths.rs` and the `dirs` crate; the credential provider's behaviour; the
  DDL, blob and enum shapes, zstd framing, path derivation, WAL settings and version handling, all
  from source with no real install ([Observed](#observed)).
- Medium: the Flatpak `~/.var/app/<id>/{config,data}` layout (Flatpak convention; Zed only reads the
  `FLATPAK_XDG_*` variables the runtime sets).
- Medium: the `MentionUri` variant encoding, and the legacy `Role` spelling in 0.2.0 blobs.
- Not determined: whether any legacy `conversations/` data survives on old installs.
- Not determined: whether a thread's sidebar rows outlive "Delete all", and whether a sidebar
  terminal row outlives its thread ([What survives a delete](#what-survives-a-delete)).
