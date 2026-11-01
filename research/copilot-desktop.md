# copilot-desktop

GitHub Copilot Desktop (catalog agent `copilot-desktop`) is GitHub's closed-source desktop app,
shipped as the dpkg package `github` on Linux and as the per-user installer
`GitHub-Copilot-windows-<arch>-setup.exe` on Windows, an NSIS installer (NSIS being the Nullsoft
Scriptable Install System, an installer framework). It is a Tauri app: Tauri is a desktop
framework that draws the user interface in the operating system's web engine (WebKitGTK, the GTK
port of the WebKit browser engine, on Linux; WebView2, Microsoft Edge's Chromium engine packaged
for embedding in other applications, on Windows) and runs the backend as a native Rust program.
The app runs its sessions through a bundled copy of Copilot CLI and shares `~/.copilot` with it:
the session logs under `~/.copilot/session-state/` are Copilot CLI's format, stay with the
`copilot-cli` agent and are read by the `copilot-cli` parser ([`copilot-cli.md`](copilot-cli.md));
nested catalog entries claim the app's own files inside that directory. The `copilot-desktop`
parser reads the app's own SQLite store, `~/.copilot/data.db`, which records the accounts,
repositories, worktrees, workspaces, sessions, activity and schedules around those logs.
[Issue 147](https://github.com/seanthegeek/doubleagent/issues/147) tracks the app.

## Source and evidence

Closed source, so there are no commit links. On Linux the package depends on
`libwebkit2gtk-4.1-0` for its web engine. The app identifier is `com.github.githubapp`. Each
claim names its evidence level:

- **Shipped binary**: `GitHub-Copilot-linux-arm64.deb` (464 MB), dpkg
  package `github` 1.1.29 (arm64 `.deb`), maintainer "you", description "Tauri Copilot
  Application", installed size 1,609,955 KB. `/usr/bin/github` is a
  1,388,701,168-byte ELF aarch64 executable with debug information, not
  stripped (`file`: BuildID
  `337d94e3f5709ec9c61ece90f86958a5d141a9e3`), sha256
  `9f6259e76190ef3fd89561e594a0a6fad0967bddc2faf3b119c8bafa3097ec27`;
  the `.deb` is
  `51117b94e5e278e609d9f568857c86d7b45cbc98d8d09b20497134377c33351b` and
  `/usr/bin/git-credential-copilot` (4,342,424 bytes)
  `5e89734fe81b21dfa32c8b4473e7d055b38586804e23ed03a79353dc35ce32f1`.
  It was read with `strings` and `grep` only, never run by the research.
  Citations are written `github 1.1.29 linux-arm64 /usr/bin/github` with
  the literal snippet, shortened after this paragraph to "binary" and the
  snippet; no byte offsets were recorded,
  so grep for the snippet. Its Rust crates include `github_app`,
  `github_app_desktop_services`, `github_app_storage`,
  `github_app_database`, `github_copilot_sdk` and
  `github_app_git_native`; the paths module is
  `crates/desktop-services/src/platform/paths.rs` (binary). The package also ships the SDK's
  TypeScript declarations (SDK, software development kit: GitHub's library that embeds and drives
  the Copilot CLI) under `/usr/lib/GitHub Copilot/copilot-sdk/`, read with `grep` and `sed` and
  cited as "SDK declarations" with the file and the declaration; `cliVersion.d.ts` names the CLI
  it drives, `COPILOT_CLI_VERSION = "1.0.95-3"`.
- **Dirs crate**: the binary links the Rust `dirs` crate (binary:
  `index.crates.io-1949cf8c6b5b557f/dirs-6.0.0`, with `dirs-5.0.1` and
  `dirs-4.0.0` beside it), whose data directory is
  `~/.local/share` on Linux, `~/Library/Application Support` on macOS and
  `%APPDATA%` (`AppData/Roaming`) on Windows, and whose cache directory is
  `~/.cache` on Linux and `%LOCALAPPDATA%` (`AppData/Local`) on Windows.
  The Windows install confirmed both Windows directories (the data
  directory held only `app-skills/`, and the three tool downloads went to
  `AppData/Local`); the Windows binary carries no `dirs-<version>` string,
  so the crate is inferred there from the paths, not read. The macOS paths
  in this document are derived from the crate's behaviour and the app
  identifier, not observed.
- **Shipped binary (Windows)**: `GitHub-Copilot-windows-arm64-setup.exe`
  (297,454,576 bytes, sha256
  `d63b70c36d61a090a4a455dc8b67c538374328304846035e44b2740781e2a4dd`,
  version resource 1.1.30, product name "GitHub Copilot"), an NSIS
  installer (the install directory holds its `uninstall.exe` and the icons directory
  its `nsis-header.bmp` and `nsis-sidebar.bmp`). It installs per user
  into `AppData/Local/Programs/GitHub Copilot/` (312 files, 520 MB):
  `github.exe` 1.1.30 windows-arm64 (442,207,520 bytes, sha256
  `8e9b4162ee7c0d7c7212c3b34b05c7bd8809af24494ab4560f2fc2d4536288e5`,
  FileVersion 1.1.30, CompanyName "GitHub Inc.") and
  `git-credential-copilot.exe` (289,568 bytes, sha256
  `d538676354421560ede7e3490f0661c24ef8698fbbcd62f6b9ea15ea533d3af6`),
  read on the VM with a string search only, the same way as the Linux build. Citations are
  written `github 1.1.30 windows-arm64 github.exe` with the literal snippet; no
  byte offsets were recorded. The SDK declarations in its install directory's
  `copilot-sdk\` say `COPILOT_CLI_VERSION = "1.0.99-2"`
  (`cliVersion.d.ts`) and `SDK_PROTOCOL_VERSION = 3`
  (`sdkProtocolVersion.d.ts`).
- **Docs**: none fetched.

### Observed

- **Windows**: GitHub Copilot Desktop 1.1.30, the arm64 NSIS installer of the
  **Shipped binary (Windows)** source above, installed per user at 19:48Z on 2026-10-17 on a
  Windows 11 ARM64 Parallels VM (build 10.0.26300.9550, Windows PowerShell 5.1.26100.9549, user
  `sean`, home `C:\Users\sean`), surveyed over SSH for names, sizes, modification times, key names
  and types only; all times are UTC. Exercised: first start at 19:53:23Z (`data.db` created
  19:53:24Z); sign-in by device-code flow (a sign-in under OAuth, the open standard for delegated
  authorisation, in which the app shows a short code that the user enters on github.com in a
  browser) at 19:54:27Z; three MCP servers (Model Context Protocol servers, programs that offer the
  agent extra tools) added from the app's settings at 19:56Z, two of which could not start because
  their commands were not installed; a general chat, session `90825c78…` (created 19:59:11Z, three
  prompts at 20:00:04Z, 20:02:02Z and 20:02:49Z), in which the agent built and tested a small
  Node.js MCP server in the chat's working directory, running `npm install` there; the GitHub
  repository `seanthegeek/doubleagent` added at 20:04:45Z, cloned into `.copilot\repos\doubleagent\`
  at 20:04:42Z and checked out into a git worktree (a second working directory of the same
  repository on its own branch) at 20:04:47Z; and one prompt in a project session, `f60c7c7f…`
  (created 20:04:52Z, prompted 20:05:27Z, one `powershell` call, one checkpoint at 20:05:40Z). The
  user had installed Node.js 24.22.0 and the Visual Studio Build Tools beside the app for the MCP
  work; `AppData/Local/npm-cache` is theirs. Every action ran before the first listing, so the
  times above and the files' modification times are the only mapping from action to file. What
  the app wrote is in [Per-user storage](#per-user-storage): its own files inside and outside
  `.copilot`, the WebView2 user data folder, three tool downloads under `AppData/Local`, seven pool
  and probe sessions it pre-created and never prompted, and, through the bundled Copilot CLI
  1.0.99-2, the session directories, `session-store.db`, `mcp-config.json` and the CLI cache
  `AppData/Local/copilot`
  ([Source and evidence in `copilot-cli.md`](copilot-cli.md#source-and-evidence)). Then, at 20:31Z,
  the user deleted the general chat with the app's own delete action (app log message kind
  `delete_session`) and a listing diff was taken; at 20:35Z the user quit the app from the system
  tray (the notification-area icon, which keeps the app running after its window is closed), no
  `github.exe` or `copilot.exe` process was left, and a second listing was taken. What the delete
  and the quit removed and left is in [What survives a delete](#what-survives-a-delete). Not
  exercised: a scheduled task, a session automation, a denied tool call (allow-all stayed on), a
  model switch, a resumed session, a fork, a side chat, a remote session.
- **Linux**: GitHub Copilot Desktop 1.1.29, the arm64 `.deb` of the **Shipped binary** source
  above, installed at 21:59Z on 2026-10-16 on an Ubuntu 26.04.1 arm64 Parallels VM (home
  `/home/sean`, `/bin/sh` dash) that already held the Copilot CLI 1.0.98 round's `~/.copilot`
  ([Source and evidence in `copilot-cli.md`](copilot-cli.md#source-and-evidence)), surveyed for
  names, sizes, modification times, key names and types only. Exercised: app start at 22:00:30Z;
  sign-in by device-code flow (app log task `auth_device_code_flow`), which modified the GNOME
  keyring at 22:01:14Z ([Credentials](#credentials)); one general chat, session `5d404e14…` (created
  22:01:27Z, one prompt, no tool call, titled by the user); then the user added the GitHub
  repository `seanthegeek/doubleagent`, which the app cloned and checked out into a git worktree
  at 22:03Z, and asked one question in a project session, `79255eab…` (created 22:03:11Z, four
  prompts, `bash` called twice and `create` once; the `create` call wrote a file on the user's
  desktop, outside the worktree, which the CLI's `session-store.db` `session_files` records).
  Every action ran before the first listing, so the times above and the files' modification times
  are the only mapping from action to file; what the app wrote is in
  [Per-user storage](#per-user-storage). Then, at 02:05Z on 2026-10-17 (22:05 on 2026-10-16 in the VM's
  local time; the app log stamps `delete_workspace` 02:05:17Z and `delete_session` 02:05:31Z), the
  user restarted the app and deleted both sessions with the app's own delete action, which sends
  the bundled CLI `sessions.delete`, and a listing diff (names and counts only) was taken. What
  the delete removed and left is in [What survives a delete](#what-survives-a-delete). By then two
  more never-prompted sessions existed, `83277a7e…` (created 22:15Z) and `4c088c7e…` (created
  23:01Z) ([Inside `~/.copilot`](#inside-copilot)), and the restart had created an empty general
  chat, `4e347d3e…` (`chats/2026-10-16/symmetrical-telegram-4e347d3e/`); none of the three was
  exercised. Not exercised: a scheduled task, a session automation, a removed project, a denied
  tool call, a model switch, a remote session, a fork, a side chat.
- macOS was not observed.

### Source reviews

None.

## Collection

### Per-user storage

#### Scratch directories

The app's own working directories, which are not projects the user works in
([Where the project path is recorded](#where-the-project-path-is-recorded)): a general chat's
`~/.copilot/chats/<date>/<slug>-<id8>/`, a project session's worktree under
`~/.copilot/repos/copilot-worktrees/`, the clone under `~/.copilot/repos/`,
`~/.copilot/canvas-catalog-probe/` and, on Windows, the install directory
`AppData/Local/Programs/GitHub Copilot`; the last three were the `cwd` of the pool
and probe sessions the app pre-creates ([Inside `~/.copilot`](#inside-copilot)).

#### Package files

Linux (shipped binary, real install (Linux)):
`/usr/bin/github`, `/usr/bin/git-credential-copilot` (4,342,424 bytes,
the git credential helper, [Credentials](#credentials)), and
`/usr/lib/GitHub Copilot/` (244 MB): `Microsoft.AI.Foundry.Local.Core.so`,
`libonnxruntime.so`, `libonnxruntime-genai.so` and
`libonnxruntime_providers_shared.so` (Foundry Local, Microsoft's runtime
for running models on the local machine, built on ONNX Runtime,
Microsoft's inference engine for models in the ONNX format, Open Neural
Network Exchange), `app-extensions/` (`README.md`, `catalog.json`,
`azure-devops/`), `copilot-extension-host/`, `copilot-sdk/` (TypeScript
declarations, `extension.js` and four Markdown guides under `docs/`; SDK,
software development kit, here GitHub's library that embeds and drives
the Copilot CLI), `icons/`, `native-plugins/`
(`libgithub_app_linux_pulse_plugin.so`), `plugins/` (empty), `sounds/`,
`terminal-integration/` (shell integration scripts for bash, zsh, fish
and PowerShell). Desktop entries `/usr/share/applications/GitHub
Copilot.desktop` (`Exec=github %u`, `StartupWMClass=github`, URL schemes
`github-app`, `ghapp` and `gh`) and, written by the app at first start,
`~/.local/share/applications/com.github.githubapp.desktop` (138 bytes;
not collected, outside every entry).

Windows (shipped binary, real install (Windows)): the per-user
install directory `AppData/Local/Programs/GitHub Copilot/` holds
`github.exe`, `git-credential-copilot.exe`,
`Microsoft.AI.Foundry.Local.Core.dll` (24 MB), `onnxruntime.dll`,
`onnxruntime-genai.dll`, `onnxruntime_providers_shared.dll`,
`uninstall.exe`, and the directories `app-extensions/` (`catalog.json`,
`README.md`, `azure-devops/`), `copilot-extension-host/preloads/`,
`copilot-sdk/` (TypeScript declarations, `index.js`, `extension.js`;
`cliVersion.d.ts` sets `COPILOT_CLI_VERSION = "1.0.99-2"` and
`COPILOT_CLI_USE_NPM_PACKAGE = false`, `sdkProtocolVersion.d.ts` sets
`SDK_PROTOCOL_VERSION = 3`), `icons/`, `plugins/`, `sounds/` and
`terminal-integration/` (the Linux scripts plus `shellIntegration.ps1`).

Unlike the Linux package's empty `plugins/`, the Windows one holds three
built-in plugins: `computer-use/` (`computer-use-mcp.exe`,
`CopilotComputerUse.exe`, `mcp.json`, `plugin.json`,
`github-app-integrity.json`), and `excel-agent/` and `word-agent/` (each
`plugin.json`, `github-app-integrity.json`, two skills under `skills/`
and `com.github.copilot/extensions/`). The install directory is outside
every catalog entry.

The installer registers, under HKCU
(`HKEY_CURRENT_USER`, the registry hive of the signed-in user), the
uninstall key
`HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall\GitHub Copilot`
(DisplayName "GitHub Copilot", DisplayVersion 1.1.30, Publisher "GitHub
Inc.", InstallLocation the directory above, EstimatedSize 508107 KB) and
the URL protocol handlers `HKCU\Software\Classes\github-app`, `ghapp` and
`gh` (each `shell\open\command` runs `github.exe "%1"`), with no Run
(autostart) entry, and the shortcuts `AppData/Roaming/Microsoft/Windows/Start
Menu/Programs/GitHub Copilot.lnk` and `Desktop/GitHub Copilot.lnk`. The
registry is outside the collector's scope.

#### Inside `~/.copilot`

The app resolves `~/.copilot` as Copilot CLI does, through `COPILOT_HOME`,
and an app-only override `COPILOT_HOME_OVERRIDE` (binary:
`github_app_desktop_services::platform::paths::COPILOT_HOME_OVERRIDE`). The
neighbouring string run in the binary lists the names it builds under it:

```text
copilot-worktreesdata.dbattachmentsmedia-cachesession.dbCOPILOT_HOMEchatscrash-reportsfilesplan.mdsession-state
```

Shared with Copilot CLI (real install (Linux)): `session-state/<uuid>/`
holds each app session in Copilot CLI's format (`events.jsonl`,
`workspace.yaml`, `checkpoints/index.md`, `files/`, `research/`,
`.workspace-fork.lock`), written by the bundled CLI; the directory name is
the session id in `data.db` `sessions.id`. A desktop session differs from a
CLI session in `workspace.yaml` `client_name` (`github/autopilot`, where
the CLI writes `github/cli`), `user_named` true, and, for a project
session, `git_root`, `repository`, `host_type` and `branch` after `cwd`;
neither had `mc_last_event_id`. Each directory also held `session.db`
(192 KB SQLite, the per-session workflow and factory store that
[Per-user storage in `copilot-cli.md`](copilot-cli.md#per-user-storage) names
from the bundle; its tables are in
[`session.db` and `hsts-storage.sqlite`](#sessiondb-and-hsts-storagesqlite)),
`inuse.<pid>.hold` (0 bytes) and `inuse.<pid>.lock` (6 bytes). Two more
directories, `session-state/pending-session:<uuid>/files/` and
`session-state/pending-session:draft:<uuid>/files/`, were created empty
at 22:03Z.

`extensions/` (empty) is also a CLI directory: the SDK guide names "the
user's copilot config extensions directory" (shipped binary,
`copilot-sdk/docs/extensions.md`), so it stays with `copilot-cli`.
`logs/process-<epoch ms>-<pid>.log` (six files) are the bundled CLI's
process logs and stay with `copilot-cli` too (lines such as "Starting CLI
in server mode (stdio)", `Created session: <uuid>`, "Installed N native
extension(s) for session",
`Cleaning up session <uuid>: Session deleted via sessions.delete`).
`~/.cache/copilot/` gained `tgrep-serve.lock`, and
`copilot-user-cache.json` grew to 6.5 KB.

On Windows (real install, CLI 1.0.99-2) `.copilot` sits at the top of the
profile (`C:\Users\sean\.copilot`, the `COPILOT_HOME` default under
`USERPROFILE`; `github.exe` and the CLI's `copilot.exe` both carry the
strings `COPILOT_HOME`, and `copilot.exe` also `USERPROFILE`) with the
same layout. The bundled CLI wrote `config.json`, `session-store.db`, 17
process logs, `session-state/`, `extensions/` (empty) and
`installed-plugins/` (empty).

Adding MCP servers from the app's settings
(app log message kind `add_mcp_config_server`) wrote the CLI's
`mcp-config.json` (725 bytes; shape
`mcpServers{<name>{type: "local", command, args[], tools[], env{}}}`; one
server's `env` held a token, [Secrets](#secrets)), `.mcp-writer.binding` (157 bytes JSON,
`lock{file_index, platform: "windows", volume}` and `root{...}`) and
`.mcp-writer.lock` (0 bytes), all collected as `copilot-cli`; the CLI's
cache gained `mcp-tools/<sha256>.json`, the cached tool list of each MCP
server that connected
([Per-user storage in `copilot-cli.md`](copilot-cli.md#per-user-storage)).

`session-state/` held 18 `.session-operation-locks/<uuid>.lock` files but
only 9 session directories: the CLI logs say `Created session` 18 times
and `Session deleted via sessions.delete` 9 times, so the app creates and
deletes sessions of its own. Seven of the nine were pool and probe
sessions the app pre-created and never prompted (app log tasks
`ws_session_warmup_loop`, `session_warmup_prepare`, `warm_cli_acquire`,
`warm_canvas_catalogs`): each held only `workspace.yaml`,
`.workspace-fork.lock`, `checkpoints/index.md` and empty `files/` and
`research/`, with no `events.jsonl` and no `session.db`, and
`workspace.yaml` `client_name` `github/autopilot`, `user_named` false.
Their `cwd` was the app's install directory
`C:\Users\sean\AppData\Local\Programs\GitHub Copilot` (four sessions),
`.copilot/canvas-catalog-probe` (two) or the clone
`.copilot/repos/doubleagent` (one, created after the project session,
with `git_root`, `repository`, `host_type` and `branch`). The two
exercised sessions had `events.jsonl`, `session.db` and, while running,
`inuse.<pid>.hold` and `.lock`, and their `workspace.yaml` the extra keys
`name`, `user_named: true`, `remote_steerable` and the `mc_` keys. No
`pending-session:*` directory, `rewind-file-snapshots/`, `attachments` or
`crash-reports` appeared. On Linux two such sessions existed by the time of
the deletion, `83277a7e…` (created 22:15Z) and `4c088c7e…` (created
23:01Z), both with `client_name` `github/autopilot`, `user_named` false,
no `events.jsonl` and a `cwd` under `.copilot/` (a `canvas-catalog-probe`
`cwd` belongs to one of them).

App-only, all created at 22:00:30Z unless said (real install (Linux)):

| Path | Size | What it holds |
| --- | --- | --- |
| `data.db` | 1.3 MB | the app's own SQLite store (WAL mode, write-ahead log: new pages go to the `-wal` file until a checkpoint copies them into the main file; binary: `PRAGMA journal_mode=WAL;PRAGMA foreign_keys=ON;`): accounts, projects, worktrees, workspaces, sessions, activity, schedules; 110 tables |
| `data.db-wal`, `data.db-shm` | 1.67 MB, small | its WAL and shared-memory index; the WAL is larger than the main file, so both must be collected with it |
| `data.db.open-lock`, `data.db.startup-lock` | 0 bytes | lock files (binary: `db.open-lockdb.startup-lock`) |
| `repo-metadata-cache.db` (+`-wal`, `-shm`) | 4,096 (`-wal` 98,912) | GitHub metadata cache (binary: `repo-metadata-cache.db`); tables in [`repo-metadata-cache.db`](#repo-metadata-cachedb), all empty but one lifecycle row |
| `chats/<date>/<slug>-<first 8 of the session id>/` | empty | one working directory per general chat, the chat session's `cwd` (`chats/2026-10-16/automatic-barnacle-5d404e14`); on Windows the agent built a Node.js project in one (`chats/2026-10-17/refactored-barnacle-90825c78/weather-mcp/` with `package.json`, `package-lock.json`, `test-client.js`, `src/index.js` and `node_modules/`, 3,415 files and 14.8 MB in all, 4 files and 45,644 bytes without `node_modules`) |
| `repos/<name>/` | 20 MB | the clone of each added repository (remote `https://github.com/<login>/<name>.git`; its `.git/config` held no credential helper, and there was no `~/.gitconfig`); its `.git/refs/copilot/checkpoints/` holds the session checkpoint refs ([What survives a delete](#what-survives-a-delete)) |
| `repos/copilot-worktrees/<name>/<branch>/` | not recorded apart from the clone | the git worktree a project session runs in, on a new branch named `<login>-<adjective>-<noun>`; after the deletion round, the pool's pre-created worktree `copilot-prewarm-<adjective>-<noun>` instead |
| `run/single-instance.lock`, `run/single-instance.owner` | 0 and 5 bytes | the single-instance lock and the owning pid (binary: `single-instance.locksingle-instance.ownersingle-instance-pr.locksingle-instance-pr.owner`) |
| `run/ws.port`, `run/ws.release.port` | 11 bytes each (10 on Windows) | the port of the app's local WebSocket server |
| `run/ws.token`, `run/ws.release.token` | 49 bytes each (48 on Windows) | the WebSocket auth token ([Credentials](#credentials)) |
| `media-cache/` | empty | media cache (binary: `"$HOME/.copilot/media-cache/**",`) |
| `canvas-catalog-probe/` | empty | canvas catalog probe (binary: `canvas-catalog-probe`) |
| `logs/github-app.<pid>.log` | 109 KB | the Tauri backend's tracing log: WebSocket server, session warm-up, the device-code sign-in, "wrote run file", "wrote WebSocket auth token", "Scheduler starting for workflows, session automations, and Scheduled tasks", git version detection, canvas catalog warm-up (each a literal in the binary) |

Named in the binary, not observed: `$HOME/.copilot/attachments/**` and
`$HOME/.copilot/workspaces/*/attachments/**` (binary: `"$HOME/.copilot/attachments/**",`,
`"$HOME/.copilot/workspaces/*/attachments/**",`), and `crash-reports`
(binary, in the string run above; also in
`github 1.1.30 windows-arm64 github.exe`: `attachments`,
`crash-reports`). They get no catalog line until an install shows them,
and the Windows install did not: `attachments` under `~/.copilot` is
collected with `copilot-cli` meanwhile.

On Windows the same app-only set was written, with these differences:
`data.db` 1,347,584 bytes and unchanged while the app ran (`-wal`
1,796,352 at 20:08Z, 1,907,592 at 20:32Z); the `run/` files rewritten when
the app restarted its WebSocket server (`ws.port` and `ws.token` at
20:02:41Z and 20:31:33Z), all six still on disk after the app quit;
`run/single-instance.lock`, on which the running app holds a byte-range
lock, so no other process can read it while the app runs;
`repos/<name>/.git/` held
`refs/copilot/checkpoints/<session id>/<seq>/<event id>` for the project
session's one checkpoint and `worktrees/<branch>/` for the session's
worktree and the pool's `copilot-prewarm-<adjective>-<noun>` worktree;
the app log `logs/github-app.5688.log` (193,758 bytes after quit) has the
start line `GitHub Copilot app started pid=N version=N.N.N commit=<hex>
platform=windows arch=aarch64 ...`, `opening database
db_path="C:\\Users\\sean\\.copilot\\data.db"`, `embedded git installed
path=<path>`, `Prepared executable built-in plugin plugin_name=...`
(three times) and `Executable built-in plugin is not bundled for this
platform` (once).

#### Outside `~/.copilot`

| Path | Size | What it holds |
| --- | --- | --- |
| `~/.local/share/com.github.githubapp/` (Linux, observed); `AppData/Roaming/com.github.githubapp` (Windows, observed); `~/Library/Application Support/com.github.githubapp` (macOS, dirs crate, not observed) | 4.6 MB (Linux), 3.0 MB (Windows) | the Tauri data directory; on Windows it held only `app-skills/`, the web view's storage being in the WebView2 folder below |
| `.../app-skills/` | 3.3 MB (Linux), 3.0 MB and 145 files (Windows) | skills bundled with the app, copied at first start (`af`, `agent-merge`, `agentfinder`, `connector-finder`, `create-canvas`, `factory-creator`, `impeccable` with hundreds of `.mjs` scripts, `latex`, `orchestrate`, `pr-stack`, each with `SKILL.md`; 1.1.30 on Windows adds `automation-group-creator`), markers `.app-skills-bundled` and `.app-skills-version` (binary: `app-skillsapp-skills.new.app-skills-version`) |
| `.../localstorage/tauri_localhost_0.localstorage` (+`-wal`, `-shm`) | 4,096 (`-wal` 939,392) | the web view's local storage, SQLite `ItemTable(key, value BLOB)`, with the unsent composer draft and tab lists ([Web view local storage](#web-view-local-storage)) |
| `.../WebKitCache/Version 17/` | 280 KB | WebKit's HTTP cache (`Blobs`, `Records`, `salt`) |
| `.../CacheStorage/salt`, `.../storage/salt`, `.../mediakeys/v1/salt` | 8 bytes each | WebKit's per-origin salts |
| `.../hsts-storage.sqlite` | 12,288 | WebKit's HSTS store (HTTP Strict Transport Security: the hosts that must be reached over HTTPS) |
| `~/.github-copilot-cli/` | 96 KB | Foundry Local's directory, named after the app's Foundry app name `github-copilot-cli` (binary: `Skipping Foundry Local init: rate-limit cool-down recorded by a concurrent attemptgithub-copilot-cli`) |
| `~/.github-copilot-cli/cache/models/foundry.modelinfo.json` | 71 KB | the local-model catalog (44 models); downloaded weights would land under `cache/models/` (none downloaded) |
| `~/.github-copilot-cli/ep/` | empty | not determined |
| `~/.github-copilot-cli/logs/foundry.core<yyyymmdd>.log` | 3,660 | Foundry Local log: `Telemetry initialized for app 'github-copilot-cli' with Device ID: <hex>`, "Found N locally cached models", "Got N models from Azure Foundry catalog" |
| `~/Microsoft/DeveloperTools/.onnxruntime/deviceid` | 36 bytes | a UUID in a directory named `.onnxruntime`, different from the `~/.cache/Microsoft/DeveloperTools/deviceid` of [issue 148](https://github.com/seanthegeek/doubleagent/issues/148); not collected |
| `~/.cache/github-copilot-sdk/cli/1.0.95-3/copilot` | 180 MB (180,095,744 bytes, sha256 `42cf5d50353862c2a04108ae2556df8a32810ee4de8890c472600bd0abd94407`) | the Copilot CLI that runs the app's sessions, downloaded at first start, with the marker `.copilot-cli.ok` (9 bytes; binary: `.copilot-cli.ok`, `github-copilot-sdk`) |
| `~/.cache/github-copilot-git-2.53.0-4/` | 53 MB | a private git build (`bin/`, `etc/`, `libexec/`, `share/`, `ssl/`); the app log says `embedded_git_in_use=true` (binary: `github-copilot-git-`) |
| `~/.cache/copilot-desktop-gh-2.102.0/` | 38 MB | GitHub's `gh` CLI (binary: `copilot-desktop-gh-`) |
| `~/.local/share/keyrings/login.keyring` | not recorded | the GNOME keyring, modified at sign-in ([Credentials](#credentials)) |

No `~/.config` entry was written. The layout of `~/.copilot` is the same
on every OS, as Copilot CLI's is; the Tauri data directory moves with
the dirs crate.

On Windows (real install, 1.1.30) these were written outside `.copilot`:

| Path | Size | What it holds |
| --- | --- | --- |
| `AppData/Local/com.github.githubapp/EBWebView/` | 611 files, 72,614,047 bytes | the WebView2 user data folder (binary: `EBWebView`; `Last Version` says runtime 155.0.4283.62), a Chromium profile; collected as `copilot-desktop`, less its caches ([Exclusions](#exclusions)); itemised below |
| `AppData/Local/github-copilot-sdk/cli/1.0.99-2/copilot.exe` | 158,577,952 bytes, sha256 `b7bc340acf1c82228de2e3504444847f8e18beec1f07f72f260f1bbeb4c3a9b2` | the Copilot CLI that runs the app's sessions, with `.copilot-cli.ok` (9 bytes), downloaded 19:53:31Z to 19:53:51Z; not collected |
| `AppData/Local/github-copilot-git-2.53.0-4/` | 517 files, 114 MB | the embedded git in the MinGW layout (Minimalist GNU for Windows, the GNU toolchain port Git for Windows is built with): `clangarm64/`, `cmd/git.exe`, `etc/`, `usr/`, `LICENSE.txt` and an `.integrity-<sha256>` marker; not collected |
| `AppData/Local/copilot-desktop-gh-2.102.0/gh.exe` | 39,351,608 bytes | GitHub's `gh` CLI; not collected |
| `.github-copilot-cli/` | | Foundry Local's directory, as on Linux: `cache/models/foundry.modelinfo.json` (72,935 bytes, 44 models), empty `ep/`, `logs/foundry.core20261017.log` (3,887 bytes; its cache line names `C:\Users\sean/.github-copilot-cli/cache/models` with mixed separators, and it says "Model ... is not supported on Arm64 currently"); no device id file beside it |
| `AppData/Roaming/Microsoft/Credentials/<32 hex>` | 666 bytes | a file of Credential Manager, the per-user credential vault of Windows, written at the sign-in minute ([Credentials](#credentials)); outside the catalog |

The three downloads under `AppData/Local` have the names Linux keeps
under `~/.cache`. No `AppData/Local/Microsoft/DeveloperTools/`,
`~/Microsoft/`, `~/.cache/`, `.onnxruntime` directory or file named
`deviceid` was in the profile to depth 6; the Microsoft developer-tools
device id of [issue 148](https://github.com/seanthegeek/doubleagent/issues/148)
is the registry value `HKCU\SOFTWARE\Microsoft\DeveloperTools\deviceid`
(a string value, type `REG_SZ`, of 36 characters) on Windows, with a
`HardwareCache` subkey.

The WebView2 folder, per directory (files, bytes), with the subtrees the
catalog excludes marked ([Exclusions](#exclusions)):

| Path under `EBWebView/` | Files, bytes | What it holds |
| --- | --- | --- |
| `Local State` | 1, 10,079 | Chromium's browser-wide state, JSON: `os_crypt{encrypted_key}` (the profile key, wrapped with DPAPI, the Windows Data Protection API, [Credentials](#credentials)), `profile`, `user_experience_metrics{client_id2, machine_id, ...}`, `variations_*`, `updateclientdata` |
| `Default/Local Storage/leveldb/` | `000003.log`, `CURRENT`, `LOCK`, `LOG`, `MANIFEST-000001` | the web view's local storage, a leveldb database (Google's key-value store library, which Chromium uses for DOM storage), with the keys of the Linux WebKit `localstorage` table ([Web view local storage](#web-view-local-storage)) |
| `Default/Session Storage/` | `000003.log` 30, then 234 | session storage, leveldb |
| `Default/Network/` | `Cookies` 20,480 and `Cookies-journal`, `TransportSecurity` 189, `Network Persistent State`, `Reporting and NEL`, `Trust Tokens`, `Device Bound Sessions`, `Sdch Dictionaries`, `SCT Auditing Pending Reports`, `NetworkDataMigrated` | the network stores; `Cookies` had 0 rows after the app quit (it cannot be opened while any `github.exe` runs, Windows error 32, a sharing violation) |
| `Default/History` | 249,856 | browsing history, SQLite: 7 `urls` rows, all `http://tauri.localhost` (the app's own pages), 16 `visits` |
| `Default/Login Data`, `Login Data For Account` | 43,008 each | saved passwords; 0 `logins` rows |
| `Default/Web Data` | 227,328 | autofill: 1 `autofill` row, 13 `autofill_edge_field_values`, 0 `token_service` |
| `Default/Vpn Tokens` | 28,672 | 0 `vpn_token_config` rows |
| `Default/Preferences`, `Default/Secure Preferences` | 8,516 and 9,856 | profile preferences (`profile{name, edge_profile_id, created_by_version, ...}`, `extensions`, `password_manager`, `edge`, `autofill`) and their integrity checks (`protection{macs, super_mac}`, message authentication codes over the settings) |
| `Default/` others | small | `Favicons`, `Top Sites`, `settings_diagnostic.log` (38 KB), `favorites_diagnostic.log`, `DashTrackerDatabase`, `DIPS`, `ServerCertificate`, `shared_proto_db/`, `Extension Rules`, `Extension Scripts`, `Extension State`, `Site Characteristics Database`, `Sync Data/LevelDB`, `EdgeSessions/SessionRestoreLog`, `Sessions/` (empty), `README` |
| `Crashpad/` | 4, 308 | crash reporter state (`metadata`, `settings.dat`, `throttle_store.dat`, `watson_metadata`; empty `attachments/` and `reports/`) |
| `TrustTokenKeyCommitments/`, `OriginTrials/`, `AutoLaunchProtocolsComponent/` | 4, 7,093; 1, 43; 2, 3,807 | downloaded component data |
| `Default/Cache` (excluded) | 30, 7,061,265 | HTTP cache |
| `Default/Code Cache` (excluded) | 322, 92,526 | compiled JavaScript and WebAssembly |
| `Default/GPUCache` (excluded) | 5, 5,837,168 | GPU shader cache |
| `Default/DawnGraphiteCache`, `Default/DawnWebGPUCache` (excluded) | 5 and 557,424 each | WebGPU caches |
| `Default/blob_storage`, `Default/Shared Dictionary` (excluded) | not recorded | blob store and shared compression dictionaries |
| `GrShaderCache`, `ShaderCache`, `GPUPersistentCache` (excluded) | 5, 557,424; 5, 557,424; 3, 0 | shader caches |
| `component_crx_cache`, `extensions_crx_cache` (excluded) | 9, 20,720,619; 1, 13 | downloaded component and extension packages |
| `WidevineCdm` (excluded) | 6, 20,278,481 | the Widevine module (Google's digital-rights-management component for protected media) |
| `Subresource Filter` (excluded) | 15, 11,814,930 | the ad-filter list |
| `hyphen-data`, `PKIMetadata`, `CertificateRevocation` (excluded) | 54, 1,769,902; 4, 266,347; 2, 23,236 | hyphenation dictionaries, certificate metadata and revocation lists |
| `BrowserMetrics-spare.pma` (excluded) | 1,310,720 | metrics scratch file |

Top-level `Last Version`, `lockfile`, `Variations*` and the empty
`MEIPreload/` and `Trust Protection Lists/` are collected too.

### Credentials

- GitHub token: the OS keyring. The binary links `keyring_core` and
  `dbus_secret_service_keyring_store` (binary:
  `index.crates.io-1949cf8c6b5b557f/keyring-core-1.0.0`), the client for
  the Secret Service, the freedesktop.org interface over D-Bus (the Linux desktop's
  inter-process message bus) that the GNOME
  keyring (the GNOME desktop's password store) implements. On the Linux
  install `~/.local/share/keyrings/login.keyring` was modified at
  22:01:14Z, during the sign-in, and no token was found in any file the
  survey read. The keyring is outside the catalog, as it is for Copilot
  CLI. On Windows the token goes to Credential Manager (the per-user
  credential vault of Windows, whose entries are files under
  `AppData/Roaming/Microsoft/Credentials/` encrypted with DPAPI, the Data
  Protection API that ties a key to the user's logon): `github.exe` links
  the same crate with the Windows store (`github 1.1.30 windows-arm64
  github.exe`: `keyring-core-1.0.0`, `keyring-store-1.1.0`,
  `windows-native-keyring-store`, and the imports `CredWriteW`,
  `CredReadW`, `CredDeleteW`, `CredEnumerateW`), and the one credential
  file written that day, `AppData/Roaming/Microsoft/Credentials/<32 hex>`
  (666 bytes), has the sign-in minute, 19:54:27Z (`data.db`
  `accounts.created_at` 19:54:27.147Z, app log task
  `auth_device_code_flow`). The credential's target name is not
  determined: `CredEnumerateW` from the SSH session failed with error 1312
  (no logon session; a key-based SSH logon has no credential vault), and
  `cmdkey /list` printed nothing there. The Credentials directory is
  outside the catalog. The binary has no `storeTokenPlaintext` string. The
  store on macOS is not determined.
- `~/.copilot/data.db` has two columns that can hold the GitHub token, a
  file fallback beside the keyring ([Secrets](#secrets)); both were
  NULL on the Linux and Windows installs. `data.db` stays collected and
  unflagged, as Cursor's `state.vscdb` is, and those two columns are the
  ones to redact.
- `run/ws.token` and `run/ws.release.token` (49 bytes each on Linux, 48
  on Windows, characters of the URL-safe base64 alphabet): the token the app's web view presents to
  the local WebSocket server (binary and app log: "wrote WebSocket auth
  token"). It is a live local credential while the app runs, so
  `.copilot/run/*.token` is a secret glob. `run/ws.port` is not a secret.
  On Windows both were rewritten while the app ran and stayed on disk
  after it quit.
- The storage crate sets mode 0600 on a token file (binary:
  `crates/storage/src/permissions.rs` "Failed to set 0600 on token file");
  the two run tokens were mode 0600 on the install (`-rw-------`), the
  port files too; which other files it covers is not determined.
- `/usr/bin/git-credential-copilot` is a git credential helper (binary
  manifest: `"binaries/git-credential-copilot"`; build note: "`cargo build
  -p git-credential-copilot`) and set $COPILOT_TRAMPOLINE_BIN_DIR"). Its
  strings show a Unix-domain socket client (`std::os::unix::net::UnixStream`),
  which suggests it asks the running app for the credential rather than
  reading a file; not confirmed. The clone's `.git/config` named no
  helper on the install, so how git is pointed at it is not determined.
- The WebKit salts (`CacheStorage/salt`, `storage/salt`,
  `mediakeys/v1/salt`, `WebKitCache/Version 17/salt`) key WebKit's
  per-origin storage names; they are not credentials and stay unflagged.
- `~/.copilot/config.json` stays the Copilot CLI secret glob; the app did
  not change its role.
- `~/.copilot/mcp-config.json`, written by the bundled CLI when MCP
  servers are added from the app, can hold a token in a server's `env`
  ([Secrets](#secrets)); it is the Copilot CLI secret glob
  `.copilot/mcp-config.json`.
- The WebView2 `Local State`, cookie store and saved-password store are
  flagged `secret: true` by the globs
  `AppData/Local/com.github.githubapp/EBWebView/Local State`,
  `.../Default/Network/Cookies*` and `.../Default/Login Data*`, as the
  same Chromium stores of ChatGPT Desktop are, because a cookie store, a
  saved-password store and the key that encrypts them (in `Local State`,
  [Secrets](#secrets)) hold credentials whenever the web view signs
  in. The stores were empty on the install (`Network/Cookies` 0 rows
  after the quit, since the device-code sign-in runs in the system browser
  and leaves no github.com cookie in the app's web view; `Login Data` 0
  `logins` rows; `Web Data` `token_service` 0 rows; `Vpn Tokens` 0 rows).
  Of the other WebView2 files whose names look like credentials,
  `Vpn Tokens`, `Trust Tokens` and `TrustTokenKeyCommitments/keys.json`
  (the public key commitments of Chromium's Trust Tokens component) are
  not credentials.
- `git-credential-copilot.exe` on Windows carries none of the keyring
  strings; `github.exe` has the named pipe prefix `\\.\pipe\copilot-ssh-`
  (a Windows named pipe, a local inter-process channel); what uses it,
  and how the helper reaches the app on Windows, are not determined.

### Exclusions

- `.copilot/repos`, with each clone's `.git` carved out by a nested
  catalog entry: `.copilot/repos` is excluded whole (the clones' working
  trees and the worktrees under `copilot-worktrees`), and the catalog
  line `copilot-desktop|.copilot/repos/*/.git` claims each clone's
  top-level `.git` as its own walk, less the exclusions
  `.copilot/repos/*/.git/objects` (the object store) and
  `.copilot/repos/*/.git/index`. Exclusions can only remove, and their
  globs cross `/`, so no exclusion set could keep a working tree out and
  its `.git` in; the nested entry does it with data alone. Collected as
  `copilot-desktop`: `.git/refs` (the checkpoint refs), `.git/packed-refs`,
  `.git/logs` (the reflog), `.git/HEAD`, `.git/config` and
  `.git/worktrees`; the working tree, dot entries included, gets no row of
  its own beyond the `skipped_excluded` row of `repos`, and `--full`
  collects it. Which repositories were added, where, and on which branch
  is in `data.db` `projects`, `worktrees` and
  `workspace_checkout_bindings`, which are collected. The `.git` holds the
  checkpoint refs of a deleted session, which outlive the session
  ([What survives a delete](#what-survives-a-delete)).
- `.local/share/com.github.githubapp/app-skills`,
  `Library/Application Support/com.github.githubapp/app-skills` and
  `AppData/Roaming/com.github.githubapp/app-skills`: the skills shipped in
  the app, copied at first start; a skill the user added beside them is
  moved aside on a collision (binary: "a user-added app skill collides
  with a newly bundled one and was moved aside"), so the exclusion also
  prunes user-added skills there.
- `.local/share/com.github.githubapp/WebKitCache` and
  `.local/share/com.github.githubapp/CacheStorage`: WebKit's caches.
- The WebView2 user data folder `AppData/Local/com.github.githubapp`
  (Windows), a catalog entry because it holds the
  web view's Local Storage (the composer draft and tab lists that Linux
  keeps in the WebKit `localstorage` table), less its Chromium caches and
  downloaded components:
  `EBWebView/Default/Cache` (7.1 MB on the install), `Default/Code Cache`
  (93 KB), `Default/GPUCache` (5.8 MB), `Default/DawnGraphiteCache` and
  `Default/DawnWebGPUCache` (557 KB each), `Default/blob_storage` and
  `Default/Shared Dictionary` (sizes not recorded), `GrShaderCache` and
  `ShaderCache` (557 KB each), `GPUPersistentCache` (empty),
  `component_crx_cache` (20.7 MB), `extensions_crx_cache` (13 bytes),
  `WidevineCdm` (20.3 MB), `Subresource Filter` (11.8 MB), `hyphen-data`
  (1.8 MB), `PKIMetadata` (266 KB), `CertificateRevocation` (23 KB) and
  `BrowserMetrics-spare.pma` (1.3 MB), all under
  `AppData/Local/com.github.githubapp/EBWebView/`. Of the folder's 611
  files and 72.6 MB, that leaves about 140 files and at most 1.2 MB
  collected, by the survey's per-directory sizes: `Local State`,
  `Preferences`, `Secure Preferences`, Local and Session Storage, the
  `Network/` stores, `History`, `Favicons`, `Top Sites`, `Login Data`,
  `Web Data`, the diagnostic logs, `Crashpad` and the leveldb bookkeeping
  files.
- `.copilot/chats/*/node_modules`: the dependency tree of a project the
  agent builds in a chat's working directory (exclusion globs cross `/`,
  so it reaches `chats/<date>/<slug>/<project>/node_modules`). On the
  Windows install it was 3,411 of the chat directory's 3,415 files and
  14.7 of its 14.8 MB; the four files the agent wrote stay collected.
- The Copilot CLI's marketplace clone under its cache directory,
  `AppData/Local/copilot/marketplaces` (4,560 files, 263,616,505 bytes on
  the Windows install, a shallow git clone of the awesome-copilot
  marketplace, cloned 19:55:19Z to 19:55:34Z, two minutes after the app's
  first start), with `.cache/copilot/marketplaces` and
  `Library/Caches/copilot/marketplaces` beside it; see
  [Exclusions in `copilot-cli.md`](copilot-cli.md#exclusions) and
  [issue 131](https://github.com/seanthegeek/doubleagent/issues/131).
- Not collected, outside every entry: `~/.cache/github-copilot-sdk`
  (the 180 MB CLI binary), `~/.cache/github-copilot-git-<version>` (53 MB
  of git binaries) and `~/.cache/copilot-desktop-gh-<version>` (38 MB, the
  `gh` binary). Their version suffixes record which tool versions the app
  used, and a listing of `~/.cache` shows them without collecting them.
  On Windows the same three are `AppData/Local/github-copilot-sdk`
  (158.6 MB), `AppData/Local/github-copilot-git-<version>` (114 MB) and
  `AppData/Local/copilot-desktop-gh-<version>` (39.4 MB), also outside
  every entry, as is the install directory
  `AppData/Local/Programs/GitHub Copilot` (520 MB).
- Foundry Local model weights would land under
  `.github-copilot-cli/cache/models/`; none were downloaded on the
  install, so no exclusion was added. A host where a local model was
  downloaded would need one.

### Project-local files

None observed. The app writes inside the clone and worktree it manages
under `~/.copilot/repos/`, not inside the user's own repositories. The
SDK guide names `.github/extensions/` as the project extensions
directory (shipped binary, `copilot-sdk/docs/extensions.md`: "The CLI
scans `.github/extensions/` (project)"), a Copilot CLI convention, not
an app file.

### Where the project path is recorded

- `data.db` `projects.main_repo_path` (the clone under
  `~/.copilot/repos/`), with `github_owner` and `github_repo`;
  `worktrees.path` and `branch`; `workspace_checkout_bindings.repo_path`,
  `checkout_path` and `repo_full_name`; `settings.storage_location`
  (`/home/sean/.copilot/repos` on the install, the root under which the app
  clones). SQLite only, so discovery does not read them; the analyzer
  does.
- `session-state/<uuid>/workspace.yaml` `cwd`, as for Copilot CLI: a
  general chat's `cwd` is its empty `~/.copilot/chats/<date>/<slug>-<id8>`
  directory, and a project session's is its worktree under
  `~/.copilot/repos/copilot-worktrees/`. Neither is a project the user
  works in ([Scratch directories](#scratch-directories)). Project
  discovery skips a `workspace.yaml` `cwd` that lies under the same
  home's `.copilot/` directory, in both collectors; the repository the
  user added is identified by `data.db` `projects` and by the project
  session's `workspace.yaml` `repository` key instead.
- The pool and probe sessions the app pre-creates and never prompts
  ([Inside `~/.copilot`](#inside-copilot)) have a `cwd` too, on Windows
  the app's install directory for four of them. Project discovery skips
  the `cwd` of a session directory that has no `events.jsonl`, in both
  collectors: every prompted session on the Linux and Windows installs had
  one, and none of the pool or probe sessions did.
- `events.jsonl` `session.start` `data.context` carries `cwd`, and for a
  project session `branch`, `gitRoot`, `headCommit`, `hostType`
  (`github`), `repository` and `repositoryHost` (`github.com`).
- After a session is deleted ([What survives a delete](#what-survives-a-delete)),
  `session-state/<uuid>/` and the `data.db` `worktrees`, `workspaces` and
  `workspace_checkout_bindings` rows are gone; the `data.db` `projects`
  row and `session-store.db` `sessions` (`cwd`, `repository`, `branch`)
  still name the repository, and the checkpoint refs under the clone's
  `.git/refs/copilot/checkpoints/` still name the session id. The
  analyzer lists the app's own directories under `scratch_directories`
  rather than as projects ([Scratch directory rows](#scratch-directory-rows)).

## Parsing

### Transcript stores

Relative to the home.

| Path | Content | Parse |
| --- | --- | --- |
| `.copilot/data.db` (+`-wal`, `-shm`) | the app's store: accounts, settings, projects, worktrees, workspaces, sessions, activity, checkpoints, schedules | yes, primary, by the `copilot-desktop` parser |
| `.copilot/repos/<name>/.git/refs/copilot/checkpoints/<session id>/<seq>/<event id>` | one loose ref per session checkpoint (a loose ref is a ref kept as its own file under `.git/refs/`, holding a commit hash, rather than a line of `.git/packed-refs`): `<seq>` 20 digits, zero-padded, `<event id>` an event id of the session's `events.jsonl`; outlives the session ([What survives a delete](#what-survives-a-delete)) | yes, by the `copilot-desktop` parser, one `checkpoint ref` row each |
| `.copilot/repos/<name>/.git/logs/` | the clone's reflog (git's log of each change to where a ref points) | no: on the install it held only the clone, `remote set-head`, the pre-created worktree's checkout and reset entries, and no checkpoint ref |
| `.copilot/session-state/<uuid>/events.jsonl`, `workspace.yaml` | each app session's event log and metadata, Copilot CLI's format | yes, by the `copilot-cli` parser ([Transcript stores in `copilot-cli.md`](copilot-cli.md#transcript-stores)) |
| `.copilot/session-state/<uuid>/session.db` | per-session workflow and factory store ([`session.db`](#sessiondb-and-hsts-storagesqlite)) | no; its tables were empty but two |
| `.copilot/repo-metadata-cache.db` (+`-wal`, `-shm`) | GitHub metadata cache | no; empty but one lifecycle row |
| `.copilot/logs/github-app.<pid>.log` | the Tauri backend's tracing log | no: it records the app's own start-up, sign-in and scheduler steps, while the sessions, prompts and tool calls it would date are in the event logs and `data.db` |
| `.copilot/chats/<yyyy-mm-dd>/<slug>/` | a general chat's working directory and whatever the agent writes there (on the Windows install the Node.js project it built, `node_modules` included) | no; deleted with the session on Linux and Windows ([What survives a delete](#what-survives-a-delete)) |
| `.local/share/com.github.githubapp/localstorage/tauri_localhost_0.localstorage` | Linux: the web view's local storage, SQLite `ItemTable(key, value BLOB)` | no: its values are UI state (tabs, caches, theme); the one user text, the unsent composer draft `home-composer-draft:main`, was never sent to the agent and has no time of its own; reading it is left for a later parser change |
| `AppData/Local/com.github.githubapp/EBWebView/Default/Local Storage/leveldb/` (`000003.log`, `CURRENT`, `LOCK`, `LOG`, `MANIFEST-000001`) | Windows: the web view's local storage inside the WebView2 user data folder (`EBWebView/`, a Chromium profile), in leveldb, whose `.log` file holds the newest writes, with the same keys as the Linux `ItemTable` ([Web view local storage](#web-view-local-storage)) | no, for the same reason as the Linux store |
| `.github-copilot-cli/logs/foundry.core<yyyymmdd>.log` | Foundry Local log (Foundry Local: Microsoft's runtime for local models) | no: model catalog fetches and a device id, no use of the agent; on Windows no device id file sat beside it, the Microsoft developer-tools device id being a registry value ([issue 148](https://github.com/seanthegeek/doubleagent/issues/148)) |

Desktop sessions in the event logs are told from CLI sessions by
`workspace.yaml` `client_name` ([Inside `~/.copilot`](#inside-copilot));
the `copilot-cli` parser writes it at the end of the
`session metadata:` row as ` client=<client_name>`.

#### What survives a delete

Real install (Linux), the deletion step of the Linux bullet in [Observed](#observed); names,
counts and key names only. The app's delete removed each session's `session-state/<uuid>/`
directory entire (`events.jsonl`, `workspace.yaml`, `session.db`, `checkpoints/`,
`rewind-file-snapshots/`, `files/`, `research/`, the `inuse.*` files), so the `copilot-cli`
parser has no primary store left for either session; it also removed the general chat's
directory under `chats/`, the project session's worktree and its branch, and in `data.db` the
general chat's `sessions` row and every `workspaces`, `worktrees`, `workspace_session_aliases` and
`workspace_checkout_bindings` row, so the project session's rows lose their `project_path` join
([Joins and timestamps](#joins-and-timestamps)). `session_deletion_intents` stayed empty (0 rows
before and after). These survived:

- `session-store.db`, the Copilot CLI's cross-session store, which the
  CLI calls its chronicle, collected as `copilot-cli` and read by the `copilot-cli` parser as a
  secondary source ([Other stores in `copilot-cli.md`](copilot-cli.md#other-stores)):
  the `sessions` rows of both sessions (`summary` the session name, `cwd`
  under `.copilot/`, the project session with `repository` and `branch`),
  5 `turns` rows holding every prompt and final reply in full (the general
  chat's one turn written at 22:12:15Z, ten minutes after its reply; the
  project session's four at 22:04:43Z, 22:06:30Z, 22:06:50Z and
  22:17:00Z), 1 `session_files` row (the `create` of the file the agent
  wrote), 8 `assistant_usage_events` rows and 5 `search_index` rows, as the CLI's bundle, which
  holds no `DELETE` for them, predicts
  ([`session-store.db` in `copilot-cli.md`](copilot-cli.md#session-storedb)).
- `data.db`: the project session's `sessions` row (title, `session_type`
  `project`, `archived_at` null, `is_running` 0), one `activity_items`
  row for the general chat (`agent_idle`, a 300-character preview of the
  reply, `session_id` set although its session row is gone), all 4
  `session_checkpoints` rows (commit hashes and times), 7
  `session_context_usage` rows, the `projects` row and the
  `reusable_worktree_pool` row (state `available`, the worktree the pool
  pre-creates).
- `data.db-wal`: the deleted rows' pages. The two session ids occur 63
  and 105 times in the WAL and never in `data.db`, whose main file had
  not changed since the app first started.
- The clone's checkpoint refs: four loose refs
  `.git/refs/copilot/checkpoints/79255eab…/0000000000000000000N/<event id>` (N 1 to 4) in
  `repos/<name>/`, one per checkpoint of
  the deleted project session, each holding a commit hash; `<seq>` is a 20-digit zero-padded
  sequence number (1 to 4) and `<event id>` a 36-character UUID that is an event id of the
  session's deleted `events.jsonl`. Each commit has author and committer `Copilot`, subject
  `copilot checkpoint: <session id> @ <event id>`, an empty body and one parent, and is dated at
  the `session_checkpoints` times. Their trees were identical to `main`, because the session's
  only write went outside the worktree; in a session that edits files they would be per-turn
  snapshots of the worktree. The refs appear in no reflog: the reflog held only the clone,
  `remote set-head`, the pre-created worktree's checkout and reset entries. The worktree that
  remains, `repos/copilot-worktrees/<name>/copilot-prewarm-<adjective>-<noun>/`, is the one the
  reusable-worktree pool pre-creates, on a detached HEAD (a checkout of a commit, here `main`'s,
  rather than of a branch); it replaced the deleted session's worktree.
- `logs/github-app.<pid>.log` (160 KB after the round): `deleted local branch ref on hard delete
  workspace_id=<uuid> branch=<login>-improved-happiness repo=<path>`, `permanently deleted
  workspace workspace_id=<uuid>`, `archived workspace workspace_id=<uuid>`, `not parking worktree;
  removing it instead project_id=<uuid> error=reusable worktree pool is full for project <uuid>`,
  `session not found in memory during delete (already evicted) session_id=<uuid>` twice and
  `terminated agent processes owned by ending sessions scope="session_end" ...`, with 22 and 129
  mentions of the two ids; not parsed ([Transcript stores](#transcript-stores)).
- The CLI's `logs/process-<epoch ms>-<pid>.log` (collected as `copilot-cli`), untouched, with
  `Cleaning up session <uuid>: Session deleted via sessions.delete` once
  per deletion (three lines in all, one from an earlier delete); the two
  0-byte `.session-operation-locks/<id>.lock` files (collected as `copilot-cli`), one per deleted
  session; the file the agent created, still on the user's desktop, outside every catalog entry.
- `.local/share/com.github.githubapp/localstorage/tauri_localhost_0.localstorage`: the
  `side-chat-tabs` value still keys its `openSessionIdsByWorkspace` object by both deleted ids
  (empty lists) and the pending-session id; `home-composer-draft:main` is `{state: {promptText,
  selectedProjectId, projectExplicitlySelected}, version}`, empty here. The ids occur nowhere else
  in it, WAL included.

Real install (Windows), the deletion and quit steps of the Windows bullet in [Observed](#observed):
the app's delete of the general chat removed its `session-state/<uuid>/` directory entire
(`events.jsonl`, `workspace.yaml`, `session.db`, `checkpoints/`, `files/`, `research/`,
`.workspace-fork.lock` and the `inuse.*` files), its `data.db` `sessions` row and the whole
`.copilot/chats/2026-10-17/` tree: the chat's working directory, with the Node.js project the agent
had built there (`weather-mcp/`), `node_modules` included. The files an agent writes in a general
chat's directory do not outlive the chat. The project session's `inuse.<pid>.hold` and `.lock`
went too, its CLI process having ended at its 20:15Z shutdown. The chat had no checkpoint, so no
`.git` ref of it existed; the project session's ref stayed. These survived, and differ from Linux
as noted:

- `data.db`: the chat's 3 `activity_items` rows (`agent_idle`, each with
  a reply preview and the deleted session's id);
  `session_deletion_intents` stayed at 0 rows; six `app_state` keys were
  rewritten at the deletion (`sidebar-project-groups`, `right-panel`,
  `cleared-background-tasks`, `session-groups`, `main-content-tabs`,
  `inbox-sections`).
- `data.db-wal`, until the app quit: 1,907,592 bytes, with 220
  occurrences of the chat's id against none in the main file. Quitting
  the app from the tray checkpointed the WAL (`data.db-wal` 0 bytes,
  `data.db` grown from 1,347,584 to 1,433,600 bytes), after which the id
  occurs 23 times in the main file, in the surviving `activity_items`
  rows and in freed pages. So the Linux finding that a deleted session's
  rows linger in `data.db-wal` holds only until the app next quits
  cleanly; after that the main file keeps the activity rows and whatever
  freed pages a later write has not reused.
- `session-store.db`, read by the `copilot-cli` parser: the chat's
  `sessions` row, all 3 `turns` with each prompt and reply in full
  (stamped 20:02:02, 20:02:49 and 20:13:48, the last at the shutdown), 3
  `session_files` rows naming the three files the agent created
  (`tool_name` `create`), 3 `search_index` rows and 8
  `assistant_usage_events` rows. Its WAL was not checkpointed at the
  quit (704,552 bytes, 220 mentions of the id). With the chat directory
  gone, the `turns` and `session_files` rows are what remains of the
  chat's work.
- `.copilot/session-state/.session-operation-locks/<uuid>.lock` (0 bytes, as on Linux); the app
  log `logs/github-app.5688.log`, with 41 lines naming the id, among them
  `github_app::session::manager::lifecycle: session not found in memory during delete (already
  evicted) session_id=<uuid>` and `github_app::session::manager::process_teardown: terminated agent
  processes owned by ending sessions scope="session_end" session_count=1 runtime_outcome="not_live"
  ...` under message kind `delete_session`; the CLI's process log, with one `Cleaning up session
  <uuid>: Session deleted via sessions.delete` line; `run/ws.port` and `run/ws.token`, rewritten at
  the deletion (20:31:33Z).
- The WebView2 local storage
  (`AppData/Local/com.github.githubapp/EBWebView/Default/Local Storage/leveldb/`), whose tab-list
  keys were rewritten, as the Linux `side-chat-tabs` was.

After the quit the six `run/` files, the seven pool session directories, the 18
`.session-operation-locks` files, the app log (193,758 bytes) and `repo-metadata-cache.db-wal`
(98,912 bytes) stayed on disk.

The analyzer reads the surviving `session-store.db` rows through the
`copilot-cli` parser, `data.db` and the checkpoint refs through this one
([Deletion rows](#deletion-rows)); how secondary rows with no primary left are kept is in
[Secondary sources](../analyzer/docs/output.md#secondary-sources).

### Record schema

Every `created_at` and `updated_at` default is
`strftime('%Y-%m-%dT%H:%M:%fZ','now')`, ISO 8601 UTC with milliseconds
(real install (Linux)). `data.db` has 110 tables in 1.1.29 on Linux and
104 in 1.1.30 on Windows (`PRAGMA user_version` 194; not recorded on
Linux); the ones a parser reads, with the row counts observed on the
installs, are below. Columns marked "1.1.30" are in the Windows
install's definitions and not in the Linux list. Column
lists are as the survey printed the `CREATE TABLE` statement; where it
cut the statement, the list ends in "...".

| Table | Columns the parser reads (others named in the definition) | Observed |
| --- | --- | --- |
| `accounts` | `id`, `login`, `name`, `is_default`, `host` (default `github.com`), `token_updated_at`, `created_at`, `updated_at`, `access_token`, `kind` (default `github`), `settings_json`, `parent_account_id` | 1 row on each install: `kind` `github`, `host` `github.com`, `is_default` 1, `access_token` NULL; on Windows `settings_json` set and `parent_account_id` NULL |
| `settings` | `id` (always 1), `last_active_workspace_id`, `storage_location`, `instructions`, `github_access_token`, `github_token_type`, `github_scope`, `github_token_updated_at`; no timestamp column | 1 row on each install: `storage_location` the repos root under `~/.copilot`, `instructions` empty, token columns NULL |
| `model_providers` | `id`, `name`, `created_at`, `updated_at`, `type` (default `openai`), `settings_json`, `account_id` | 1 row: `type` `github_copilot`, `name` `GitHub Copilot` |
| `projects` | `id`, `name`, `container_kind` (`folder`, `repository` or `collection`), `main_repo_path` (unique), `default_branch`, `github_owner`, `github_repo`, `github_account_id`, `created_at`, `last_opened_at`; also `setup_script`, `run_script`, `instructions`, `auto_approve`, `trusted_config_sha256`, `trusted_config_accepted_at`, `branch_prefix`, `sandbox_enabled`, the `reusable_worktrees_*` settings, `checkout_is_managed`, `remote_control_enabled` and more; the definition ends `show_in_sidebar`, `provider_pr_binding_revision`, `agent_owner_session_id`, `copilot_account_id`, `copilot_account_source`, `github_account_source`, `remote_default_branch`, `default_branch_is_explicit` (1.1.30) | 1 row on each install: `container_kind` `repository`, `main_repo_path` under `~/.copilot/repos/`, owner and repository set, `default_branch` `main`; on Windows `checkout_is_managed` 1, `reusable_worktrees_enabled` 1, `auto_approve` 1, `sandbox_enabled` 0, `remote_control_enabled` 0 |
| `worktrees` | `id`, `project_id`, `path`, `branch`, `base_branch`, `created_at` | 1 row on each install: `path` under `~/.copilot/repos/copilot-worktrees/`, `base_branch` `main` |
| `workspaces` | `id`, `project_id`, `worktree_id`, `workspace_type` (`worktree`, `branch`, `folder` or `cloud`), `branch`, `name`, `created_at`, `source_pr_repo_full_name`, `source_pr_number`, `created_pr_repo_full_name`, `created_pr_number`; also `port`, the other `source_pr_*`, `source_issue_*` and `created_pr_*` columns, `last_diff_*`, token totals, `agent_merge_*`, `creator_session_id`, `archived_at`, `session_id`, `created_by`, and `host_id`, `name_source`, `explicitly_opened`, `coordinating_creator_session_id`, `provisional`, `provisional_base_ref`, `is_initialized`, `branch_session_create_scripts_started_at`, `lifecycle_generation`, `coordination_generation`, `initialization_generation`, `notification_recipient_session_id`, `session_bootstrap_config` (1.1.30) and more | 1 row on each install: `workspace_type` `worktree`, `port` 3000; on Windows `name_source` `auto`, `created_by` `user`, `is_initialized` 1 |
| `workspace_session_aliases` | `session_id` (primary key), `workspace_id`, `created_at` | 1 row on each install: the project session to its workspace |
| `workspace_checkout_bindings` | `workspace_id`, `repo_path`, `repo_full_name`, `display_name`, `checkout_kind` (`in_place` or `worktree`), `checkout_path`, `worktree_id` | 1 row on each install: `checkout_kind` `worktree`, `repo_path` the clone, `checkout_path` the worktree (not read by the parser; the worktree row carries the same path) |
| `sessions` | `id`, `title`, `session_type` (default `workspace`), `mode`, `model`, `reasoning_effort`, `created_at`, `remote_control_enabled`, `auto_approve` (default 1), `forked_from_session_id`, `archived_at`, `permission_mode`; also `is_running`, `was_interrupted`, `interruption_reason`, `updated_at`, `enabled_experiments_json`, token and context totals, `remote_session_mode`, `agent`, `provider_id`, `execution_location`, `context_tier`, `title_source`, `sandbox_enabled`, `agent_scope`, `provisional`, and `remote_availability`, `remote_availability_revision`, `remote_create_state`, `remote_prompt_delivery`, `recovered_empty_execution_event_id`, `empty_execution_failure_event_json`, `first_turn_context_delivery`, `lifecycle_generation`, `remote_lifecycle_owned` (1.1.30) and more | 2 rows on each install: `session_type` `general_chat` and `project`, `mode` NULL, `model` `claude-sonnet-5.5`, `auto_approve` 1, `remote_control_enabled` 0; `id` equals the `session-state` directory name; on Windows `reasoning_effort` NULL, `permission_mode` `on`, `title_source` `auto`, `remote_session_mode` `export`, `execution_location` `local`, `provisional` 0, `archived_at` NULL |
| `activity_items` | `id`, `workspace_id`, `session_id`, `activity_type`, `preview`, `is_read`, `created_at`, `updated_at`, `metadata_json` | Linux 6 rows: `agent_idle` ×5 with a `session_id` and a `preview` of 63 to 300 characters (the start of the reply), `workspace_ready` ×1 (31 characters, no session); Windows 5 rows: `agent_idle` ×4 with a `session_id`, `workspace_ready` ×1 without |
| `session_checkpoints` | `session_id`, `event_id`, `commit_sha`, `created_at` | Linux 4 rows for the project session, a 40-hex `commit_sha` each; the worktree's git log showed no new commit, so these are snapshot commits kept outside the branch; Windows 1 row for the project session |
| `my_copilot_scheduled_tasks` | `id`, `revision`, `title`, `prompt`, `enabled`, `execution_mode`, `background_agent_type` (default `general-purpose`), `created_at`, `updated_at`, `archived_at` | 0 rows |
| `my_copilot_scheduled_task_triggers` | `id`, `task_id`, `kind`, `enabled`, `config_version`, `config_json`, `state_version`, `state_json`, `next_run_at`, `last_fired_at`, `created_at`, `updated_at`, `archived_at` | 0 rows |
| `my_copilot_scheduled_task_invocations` | `id`, `task_id`, `trigger_id`, `occurrence_key`, `source`, `scheduled_for`, `task_revision`, `task_title`, `prompt_snapshot`, `trigger_snapshot_json`, `execution_mode`, `parent_session_id`, `runtime_agent_id`, `status`, `claimed_at`, `dispatched_at`, `completed_at`, ... (the survey cut the definition there) | 0 rows |
| `session_automations` | `session_id`, `prompt`, `interval` (`once`, `minutes`, `daily` or `weekly`), `every_minutes`, `schedule_hour`, `schedule_day`, `last_run_at`, `next_run_at`, `updated_at`, `name`, `owner_session_id`, `paused_at`; no `created_at` | 0 rows |
| `session_deletion_intents` | `session_id`, `created_at` | 0 rows; the binary inserts a row (binary: `INSERT OR IGNORE INTO session_deletion_intents (session_id) VALUES (?1)`) and leaves sessions that have one out of its queries (binary: `AND NOT EXISTS (SELECT 1 FROM session_deletion_intents`) |

#### Other `data.db` tables

Other tables observed, not read: `session_context_usage` (8 rows,
`ts_ms`, `current_tokens`, `token_limit` 200000), `app_state` (34
key-value rows; keys include `activeWorkspaceId`,
`copilot-available-models-v2`, `permission-mode-default-on`,
`session-groups`, `telemetry.machine_id`, `crash-reporting.tracking-id`,
`workspace-mru`), `schema_migration_reversals`
([Format versions](#format-versions)), `session_sources` (files and folders
attached to a session, with `onedrive` and `gdrive` providers),
`session_side_chats`, `chat_parent_links`, `remote_environments`, all
empty; and, by name only and empty unless said: `agent_memory_*`,
`agent_merge_*`, `agents`, `builtin_agents`, `canvas_catalog_cache` (1
row), `coordinator_*`, `factory_*`, `inline_review_*`,
`project_checkouts` (1 row), `project_host_trust` (1 row, a 64-hex
`trusted_config_sha256`), `project_trust_revisions` (1 row),
`reusable_worktree_pool` (1 row), `review_thread_*`, `workflow_runs`,
`workflows`, `workspace_archive_*`, `workspace_diff_*`, `workspace_file_*`,
`workspace_pull_request_links` and the rest of the 110.

On Windows (1.1.30, real install (Windows)) the other tables with rows
were `session_context_usage` (10 rows, `token_limit` 200000),
`app_state` (37 rows), `canvas_catalog_cache` (2 rows, `scope`
`project` and `user`), `project_checkouts` (1 row, `root_resource_uri`
a `file:` URI), `project_future_config_trust` (1 row, `project_id`,
`accepted_at`), `project_host_trust` (1 row), `project_trust_revisions`
(1 row), `reusable_worktree_pool` (1 row, `available`), `sqlite_sequence`
and `schema_migration_reversals`. The 37 `app_state` keys:
`activeWorkspaceId`, `canvas-store`, `catalogue-mcp-provenance`,
`cleared-background-tasks`, `copilot-available-models-v2` (23 KB),
`copilot-cloud-available-models-v2` (21 KB),
`copilot-cloud-workflow-metadata` (18 KB), `copilot-context-tier`,
`copilot-onboarding`, `copilot-reasoning-effort`,
`crash-reporting.tracking-id`, `enabled-experiments`,
`exp-assignment-cache`, `exp-first-launch-at`,
`feature-highlight-shown-today`, `feature-highlights`,
`github-read-outbox`, `home-discovery-dismissed`, `inbox-sections`,
`keep-awake-settings`, `lastRoutePath`, `main-content-tabs`,
`mock-connector-library`, `native-first-paint-theme-v1`,
`permission-mode-default-on`, `project-identity-host-revalidation-v1`,
`project-order`, `remote-control-feature-flag-split-v1`,
`remote-environments-legacy-import-completed`,
`remote-feature-flag-disables`, `right-panel`, `session-groups`,
`sidebar-project-groups`, `telemetry.machine_id`, `theme-settings-v2`,
`ui-zoom-level-v2`, `workspace-mru`. Empty: `agent_memory_bindings`,
`agent_memory_owners`, `agent_merge_delivery_origins`,
`agent_merge_permission_deliveries`, `agent_subscriptions`, `agents`,
`automation_group_contexts`, `builtin_agents`, `chat_parent_links`,
`collection_discovery`, `collection_members`,
`coordination_revocations`, `coordinator_agent_reports`,
`coordinator_assignments`, `coordinator_result_deliveries`,
`creator_notification_claims`, `factory_cloud_roles`,
`factory_creation_drafts`, `factory_cursors`, `factory_definitions`,
`factory_evaluations`, `factory_evidence_snapshots`,
`factory_job_settings`, `factory_jobs`, `factory_proposals`,
`factory_session_scopes`, `factory_targets`,
`factory_upgrade_receipts`, `inline_review_comments`,
`inline_review_replies`, `my_copilot_scheduled_task_invocations`,
`my_copilot_scheduled_task_triggers`, `my_copilot_scheduled_tasks`,
`plugin_marketplace_accounts`, `project_account_hints`,
`project_execution_scopes`, `project_host_catalogue_selections`,
`project_overview_summaries`, `project_repository_bindings`,
`project_sandbox_policies`, `project_trusted_config_hashes`,
`provider_models`, `pull_request_tours`, `remote_environment_metadata`,
`remote_environments`, `remote_session_adoption_suppressions`,
`review_thread_legacy_map`, `review_thread_messages`, `review_threads`,
`session_account_projects`, `session_automation_runs`,
`session_automation_runtime_reports`, `session_automations`,
`session_deletion_intents`, `session_managed_mcp_event_display_names`,
`session_mcp_event_sources`, `session_open_canvases`,
`session_side_chats`, `session_sources`, `software_factories`,
`software_factory_assessment_criteria`,
`software_factory_assessment_profiles`,
`software_factory_assessment_results`,
`software_factory_assessment_runs`, `software_factory_decisions`,
`workflow_runs`, `workflows`, `workspace_archive_preservation`,
`workspace_archive_preservation_attempt_worktrees`,
`workspace_archive_preservation_attempts`,
`workspace_archive_recoveries`, `workspace_changes_counts`,
`workspace_detach_notifications`, `workspace_diff_pins`,
`workspace_diff_snapshots`, `workspace_file_draft_sets`,
`workspace_file_drafts`, `workspace_file_edit_advisories`,
`workspace_file_save_journals`, `workspace_parent_links`,
`workspace_pull_request_links`, `workspace_repo_contexts`,
`workspace_side_chats`.

#### The session logs the app's CLI writes

These are Copilot CLI event logs ([Record schema in
`copilot-cli.md`](copilot-cli.md#record-schema)), with these
differences on the install (real install (Linux)):

- `session.start` `producer` `copilot-agent`, `copilotVersion` `1.0.95-3`;
  `context` holds `cwd` only for the general chat, and `branch`, `cwd`,
  `gitRoot`, `headCommit`, `hostType` (`github`), `repository` and
  `repositoryHost` (`github.com`) for the project session.
- `session.model_change`, one per session: `source` `startup`,
  `previousModel` null, `newModel` `claude-sonnet-5.5`, and the reasoning
  and tier keys all null.
- `session.permissions_changed`, one per session, with the keys
  `previousAllowAllPermissions` false, `allowAllPermissions` true,
  `previousAllowAllPermissionMode` `off` and `allowAllPermissionMode`
  `on`, not the schema's `previousMode` and `mode`. The SDK declarations
  shipped with the app name only the schema keys (SDK declarations,
  `generated/session-events.d.ts`: `export interface PermissionsChangedData`
  with `assistedApprovalModel?`, `mode?`, `previousMode?`), and the 1.0.98
  schema does not name the `allowAll*` keys either.
- `user.message` keys `content`, `delivery` (`idle`), `interactionId`,
  `messageId`, `parentAgentTaskId`, `supportedNativeDocumentMimeTypes`,
  `transformedContent`, `turnId`; no `source`, no `agentMode`.
- `tool.execution_start` keys `arguments`, `model`, `shellToolInfo` (new,
  on `bash`), `toolCallId`, `toolName`, `toolTitle`, `turnId`.
- `hook.start` and `hook.end` pairs, 5 in the general chat and 23 in the
  project session (SDK declarations: `HookStartData` `hookInvocationId`,
  `hookType`, `input?`, `parentToolCallId?`; `HookEndData` `error?`,
  `hookInvocationId`, `hookType`, `output?`, `parentToolCallId?`,
  `success`). `hookType` values and their `input` keys: `sessionStart`
  (`cwd`, `initialPrompt`, `sessionId`, `source`, `timestamp`),
  `userPromptSubmitted` (`cwd`, `prompt`, `sessionId`, `timestamp`),
  `userPromptTransformed` (the same plus `transformedPrompt`),
  `preToolUse` and `postToolUse` (keys not recorded; a `postToolUse`
  payload reached 4 KB), `agentStop` (`cwd`, `sessionId`, `stopReason`,
  `stop_hook_active`, `timestamp`, `transcriptPath`), `sessionEnd` (`cwd`,
  `reason`, `sessionId`, `timestamp`). `hook.end` carried
  `hookInvocationId`, `hookType`, `success` and, for all but the tool
  hooks, `output` `{}`.
- No `permission.requested` or `permission.completed` (the app approves
  automatically), no `session.shutdown`, no `session.info`.

On Windows (real install (Windows), bundled CLI 1.0.99-2) the
differences above held, with these changes:

- `session.start` `producer` `copilot-agent`, `copilotVersion`
  `1.0.99-2`.
- `session.permissions_changed`, one per session, with the same
  `allowAll*` keys; the `copilot-cli` parser writes it as `permission
  mode: allow-all off -> on`. `session.model_change` came at the first
  prompt, `source` `startup`, `newModel` `claude-sonnet-5.5`.
- The general chat's 115 lines: 3 `user.message`, 8
  `assistant.message`, 9 `tool.execution_start` and
  `tool.execution_complete` pairs, 27 `hook.start` and `hook.end` pairs,
  1 `system.message`, 8 `session.usage_record`, 3
  `session.usage_checkpoint` and 1 `session.shutdown`. The project
  session's 30 lines held one prompt, one tool call and 7 hook pairs.
- Tool names `powershell` and `create` (the Linux sessions had `bash`).
  Three `create` calls failed with "Parent directory does not exist" and
  were retried. The first `powershell` call exited 1 because Windows
  PowerShell's execution policy refused to run `npm.ps1` ("running
  scripts is disabled on this system"), and the agent switched to
  `npm.cmd`; that is an ordinary failed tool result, not a permission
  decision: no `permission.requested` or `permission.completed` was
  written, and allow-all stayed on.
- `hookType` adds `postToolUseFailure`, the hook for a tool call that
  failed, to the Linux list.
- `session.shutdown` was written, unlike on Linux: at 20:13:47 for the
  chat, ten minutes after its last reply, and at 20:15:43 for the
  project session.
- No `pending-session:*` directory, no `rewind-file-snapshots/`, no
  `attachments` and no `crash-reports` were written.

### Joins and timestamps

- Session: `sessions.id` = the `session-state/<uuid>/` directory name =
  `session.start` `sessionId` = `workspace_session_aliases.session_id` =
  `activity_items.session_id` = `session_checkpoints.session_id`
  (real install (Linux)). The `copilot-desktop` and `copilot-cli` rows of
  one session therefore share `session_id`.
- Project: `workspace_session_aliases.workspace_id` → `workspaces.id`;
  `workspaces.project_id` → `projects.id`, whose `main_repo_path` is the
  `project_path` of the session's rows; `workspaces.worktree_id` →
  `worktrees.id`, and `worktrees.project_id` → `projects.id`. A general
  chat has no alias row and no project. The `copilot-cli` rows of a
  project session carry the worktree path from `workspace.yaml` `cwd`,
  not `main_repo_path`, so the two parsers' `project_path` differ for the
  same session; the join is `session_id`.
- Checkpoint: `session_checkpoints.event_id` names an event of the
  session's log (format not checked against the log's `id`s). Each
  checkpoint ref's path carries the session id and an event id that is
  an `events.jsonl` event id of the session, and its commit is dated at
  the matching `session_checkpoints` time (real install (Linux)). On the
  install each ref's commit hash equalled the row's `commit_sha` and its
  event id the row's `event_id`, for all four checkpoints, so the ref,
  the row and the log event are one record under three names.
- Fork: `sessions.forked_from_session_id` → `sessions.id` (not observed).
- Scheduled task: `my_copilot_scheduled_task_triggers.task_id` and
  `my_copilot_scheduled_task_invocations.task_id` → `my_copilot_scheduled_tasks.id`;
  `invocations.trigger_id` → `triggers.id`; `invocations.parent_session_id`
  → `sessions.id` (schema; not observed).
- Times: every `*_at` column is ISO 8601 UTC with milliseconds and `Z`;
  `session_context_usage.ts_ms` is epoch milliseconds. `settings` has no
  time column, so its row is timestamped by the file's modification time.

### Other stores

Three SQLite files belong to the app (real install (Linux)); the bundled
CLI and WebKit write two more.

#### `data.db`

`~/.copilot/data.db` is in WAL mode, with its lock files beside it
([Inside `~/.copilot`](#inside-copilot)). On both installs the WAL was
larger than the main file, so the parser must read the evidence copy with
its sidecars, through `doubleagent/sqlite_util.py`. Schema in
[Record schema](#record-schema).

#### `repo-metadata-cache.db`

`~/.copilot/repo-metadata-cache.db` (+`-wal`, `-shm`; binary:
`repo-metadata-cache.db`): `github_metadata_account_lifecycle` (1 row;
its schema comment says the credential column is "Hex SHA-256 of the
credential identity in use ... never a token"), `github_metadata_items`,
`_lists`, `_list_pages`, `_org_items`, `_org_lists`, `_org_lifecycle`,
`_repo_lifecycle`, `_repo_backoffs`, `_org_backoffs`,
`_cleanup_obligations`, all empty. Not parsed.

On Windows it had 12 tables and `PRAGMA user_version` 13 (real install
(Windows)).

#### Web view local storage

`~/.local/share/com.github.githubapp/localstorage/tauri_localhost_0.localstorage`
(+`-wal`, `-shm`): WebKit's local storage for the app's page, one table
`ItemTable(key, value BLOB)`, keys `copilot.first-paint-theme.v1`,
`backend-instance-id`, `detail-cache`, `agent-tabs`,
`chat-agent-tabs-source-v1`, `chat-agent-tabs-partition-v1`,
`home-composer-draft:main` (192 bytes), `general-chat-tabs`,
`inbox-cache` (2.5 KB), `side-chat-tabs`, `workspace-tabs`. Not parsed
([Transcript stores](#transcript-stores)). This is the Linux store; on
Windows the web view is WebView2 and the same keys for the origin
`http://tauri.localhost` were in the LevelDB store
`AppData/Local/com.github.githubapp/EBWebView/Default/Local Storage/leveldb/`
(14 records in `000003.log`, several keys rewritten more than once),
also not parsed. Chromium writes DOM storage lazily, so `000003.log` was
30 bytes while the app ran and 6,174 bytes after the deletion.

#### `session.db` and `hsts-storage.sqlite`

Each session directory's `session.db` (192 KB), written by
the bundled CLI, with the tables `factory_agents`,
`factory_credit_charges`, `factory_journal`, `factory_phases`,
`factory_progress`, `factory_runs`, `workflow_agents`,
`workflow_credit_charges`, `workflow_journal`, `workflow_phases`,
`workflow_progress`, `workflow_runs`, `workflow_store_schema` (1 row) and
`github_metadata_*` (the account lifecycle 1 row, the rest empty); and
WebKit's `hsts-storage.sqlite` in the Tauri data directory. Neither is
parsed.

### Format versions

`schema_migration_reversals` records the reversible migrations: 18 rows, the highest `version` 187,
`recorded_by_app_version` `1.1.29` (real install (Linux)). It is the only version marker in
`data.db`, so a parser change for a later app version starts by comparing the highest `version` and
the table definitions with [Record schema](#record-schema). The binary carries older migrations
(binary: `ALTER TABLE github_accounts ADD COLUMN access_token TEXT;`), so earlier builds had other
table names; their shapes are not determined. The bundled CLI was 1.0.95-3 (SDK declarations,
`cliVersion.d.ts`), older than the 1.0.98 of
[Source and evidence in `copilot-cli.md`](copilot-cli.md#source-and-evidence), and its logs carry the
differences listed in
[The session logs the app's CLI writes](#the-session-logs-the-apps-cli-writes).

On Windows, 1.1.30 recorded 25 rows, `version` 170 to 194,
`recorded_by_app_version` `1.1.30`, with `PRAGMA user_version` 194 and
104 tables (real install (Windows)); the columns that differ from the
Linux list are marked in [Record schema](#record-schema). Its bundled CLI
was 1.0.99-2 and its SDK protocol version 3 (SDK declarations of the
Windows build, `cliVersion.d.ts` and `sdkProtocolVersion.d.ts`).

### Secrets

- `accounts.access_token` and `settings.github_access_token` (with
  `github_token_type`, `github_scope` and `github_token_updated_at`) can
  hold a GitHub token; both were NULL on both installs, the token being in
  the OS keyring ([Credentials](#credentials)). The binary also carries an
  older table's migration, `ALTER TABLE github_accounts ADD COLUMN
  access_token TEXT;`, and queries on `github_access_token IS NOT NULL`, so
  a file fallback exists; when it is used is not determined. The parser
  writes only whether each is present (`token_present=`,
  `github_token_present=`), never the value.
- `repo-metadata-cache.db` `github_metadata_account_lifecycle` holds a
  credential identity hash, not a token
  ([`repo-metadata-cache.db`](#repo-metadata-cachedb)).
- `accounts.login` and `projects.github_owner` name the account;
  `app_state` holds `telemetry.machine_id` and
  `crash-reporting.tracking-id` (not read).
- `activity_items.preview` is the start of an agent reply, and the
  scheduled-task and automation `prompt` columns are prompts the user
  wrote, kept verbatim.
- `run/ws.token` and `run/ws.release.token`, the local WebSocket auth
  tokens ([Credentials](#credentials)), are not read.
- `.copilot/mcp-config.json` `mcpServers.<name>.env` can hold a token: on
  the Windows install the `github` server's `env`
  `GITHUB_PERSONAL_ACCESS_TOKEN` held a GitHub personal access token (33
  characters) in plain text. Not read; see
  [Credentials in `copilot-cli.md`](copilot-cli.md#credentials).
- On Windows the WebView2 `Local State` file's `os_crypt.encrypted_key`
  (396 characters) is the key Chromium encrypts cookie values and saved
  passwords with, wrapped with DPAPI so only the user's logon can unwrap
  it; it is not a credential by itself. Not read. `Local State`
  `user_experience_metrics.client_id2` and `Preferences`
  `profile.edge_profile_id` are identifiers, not credentials.

### Parser plan

`wants()` by `artifact.rel`: `^\.copilot/data\.db$`, opened through
`sqlite_util.py`, which reads the WAL sidecar from the evidence copy, and
the checkpoint refs (below). No secondary source. Rows from `data.db`,
each `system`, with `source_file` the database and `source_line` empty,
timestamps converted with `to_utc`:

| Table | Row |
| --- | --- |
| `accounts` | at `created_at`: `account: <login> host=<host> kind=<kind> default=<is_default> token_present=<true if access_token not null>` |
| `settings` | at the file's mtime (say so in the text): `desktop settings: storage_location=<storage_location> instructions=<instructions or -> github_token_present=<bool> github_token_type=<github_token_type or ->` |
| `model_providers` | at `created_at`: `model provider: <name> type=<type>` |
| `projects` | at `created_at`, `project_path` = `main_repo_path`: `project added: <name> kind=<container_kind> path=<main_repo_path> github=<github_owner>/<github_repo> default_branch=<default_branch>` (omit ` github=` when both null) |
| `worktrees` | at `created_at`, `project_path` = the project's `main_repo_path`: `worktree: <path> branch=<branch> base=<base_branch or ->` |
| `workspaces` | at `created_at`, `project_path` as above: `workspace: <name> type=<workspace_type> branch=<branch>` plus ` source_pr=<source_pr_repo_full_name>#<source_pr_number>` and ` created_pr=<created_pr_repo_full_name>#<created_pr_number>` when set |
| `sessions` | at `created_at`, `session_id` = `id`, `model` = `model`, `project_path` via `workspace_session_aliases` -> `workspaces` -> `projects.main_repo_path` (empty for a general chat): `desktop session: <title or -> type=<session_type> model=<model or -> auto_approve=<auto_approve> permission_mode=<permission_mode or -> remote_control=<remote_control_enabled> archived=<true if archived_at>`; and when `forked_from_session_id` is set a second row `forked from session <id>` |
| `activity_items` | at `created_at`, `session_id` when set: `activity <activity_type>: <preview>` |
| `session_checkpoints` | at `created_at`, `session_id`: `checkpoint commit <commit_sha>` |
| `my_copilot_scheduled_tasks` | at `created_at`: `scheduled task: <title> enabled=<enabled> mode=<execution_mode> agent=<background_agent_type> prompt: <prompt>` |
| `my_copilot_scheduled_task_triggers` | at `created_at`: `scheduled task trigger: task=<task_id> kind=<kind> enabled=<enabled> next_run_at=<next_run_at or -> config: <config_json>` |
| `my_copilot_scheduled_task_invocations` | at `claimed_at`, `session_id` = `parent_session_id` when set: `scheduled task run: <task_title> status=<status> source=<source> scheduled_for=<scheduled_for or -> prompt: <prompt_snapshot>` |
| `session_automations` | at `updated_at`, `session_id`: `session automation: <name or -> interval=<interval> every_minutes=<every_minutes or -> next_run_at=<next_run_at or -> prompt: <prompt>` |
| `session_deletion_intents` | at `created_at`, `session_id`: `session deletion requested` |

Rows follow the table order above, then `created_at`.

Checkpoint refs: `wants()` also takes
`^\.copilot/repos/[^/]+/\.git/refs/copilot/checkpoints/([0-9a-f-]{36})/(\d+)/([0-9a-f-]{36})$`
and yields one `system` row per ref file:
`checkpoint ref <seq as an integer>: commit <file content, stripped> event <event id> (file mtime)`,
with `session_id` the first group, `timestamp_utc` the file's
modification time (`file_mtime` from `parsers/base.py`), `project_path`
empty and `source_file` the ref file. No secondary.

#### Permission rows

The app records no per-call decision in `data.db`; its
standing permission state is `sessions.auto_approve` and
`sessions.permission_mode` (and `projects.auto_approve`, not read), on
the `desktop session:` row. In the session logs it is the
`session.permissions_changed` event with the `allowAll*` keys, which the
`copilot-cli` parser writes as `permission mode: allow-all <previous> ->
<new>`; no `permission.requested` or `permission.completed` was written,
since the app approved every call automatically.

#### Model rows

`sessions.model` (and `reasoning_effort`, not read) per session,
on the `desktop session:` row and in its `model` column; the switches are
the logs' `session.model_change` events, which the `copilot-cli` parser
writes as `model:` rows. `data.db` records no per-switch history.

#### Past-use rows and secondary sources

`data.db` is a primary source for the app's own records and repeats no
event of the session logs, so no `secondary()` file.
`activity_items.preview` overlaps the start of an assistant reply in the
log but is shorter text at a different time, so it stays its own row.
The logs' `hook.*` events, which the `copilot-cli` parser writes as
`hook <hookType> start:` and `hook <hookType> end:` rows, repeat each
prompt (`userPromptSubmitted` `prompt`) and the transcript path
(`agentStop` `transcriptPath`).

#### Sync, relay and origin rows

Not stated.

#### Scratch directory rows

Since analyzer 0.18.0
([issue 149](https://github.com/seanthegeek/doubleagent/issues/149))
`inventory.json` lists the app's own working directories
([Scratch directories](#scratch-directories)) under
`scratch_directories`, the working directories that belong to the agent
rather than to the user, not under `projects`: the chat directories, the
clone and the worktree lie under `~/.copilot`, a directory the catalog
names, and the `copilot-cli` parser gives each pool or probe session (a
session the app pre-created and never prompted,
[Inside `~/.copilot`](#inside-copilot)) a `scratch directory:` row naming
its `cwd`
([Scratch directory rows of `copilot-cli.md`](copilot-cli.md#scratch-directory-rows)).
The `copilot-desktop` parser yields no such row of its own.

#### Deletion rows

`session_deletion_intents` would outlive a session the app is deleting,
but it stayed empty through the observed deletion. What a deletion leaves
is in [What survives a delete](#what-survives-a-delete): the Copilot
CLI's `session-store.db` rows (secondary sources of the `copilot-cli`
parser, which then have no primary left and are kept), the `data.db` rows
above that the delete did not remove, and the checkpoint refs, a primary
source of their own. The `checkpoint ref` rows are what remains of a
deleted session's checkpoints in the clone.

#### Bad-line rows

Every column is read as possibly NULL. A database that cannot be opened
gives one `parser: <message>` row. A checkpoint ref file that is empty or
not a 40-hex hash still yields the row with its content as read,
stripped, nothing repaired; one that cannot be read is one row
`parser: <file> could not be read (<reason>)` through `error_rows`, `source_line` 0, with the
session id from its path.

A `data.db` that cannot be read, or whose `-wal`, `-shm` or `-journal` sidecar cannot, is one row
`parser: <file> could not be read (<reason>)`, `<file>` the name of the file that failed; one that
is not a SQLite database is `parser: not a SQLite database`.

#### Fixtures

The rows of the four schedule tables are fixture only, since all four
were empty on the install ([Schedules](#schedules)).

#### Schedules

`my_copilot_scheduled_tasks`, their triggers and invocations, and
`session_automations` are the evidence of scheduled and recurring work
(binary: "Scheduler starting for workflows, session automations, and
Scheduled tasks"); all four were empty on the install.

#### MCP

The MCP servers the user adds from the app
(`.copilot/mcp-config.json`), their failed spawns (the CLI's
`logs/process-*.log`, `MCP server connection failed
{"server_name":"<name>","raw_error":"failed to spawn MCP server process:
program not found"}`) and the cached tool list of each connected server
(`AppData/Local/copilot/mcp-tools/<sha256>.json`,
[Transcript stores in `copilot-cli.md`](copilot-cli.md#transcript-stores)) are
recorded on disk (Windows install) and not yet in the timeline
([issue 150](https://github.com/seanthegeek/doubleagent/issues/150)).

## Confidence

- High: the app-only file set inside `~/.copilot`, its sizes and creation
  times, the Tauri data directory on Linux and Windows, `~/.github-copilot-cli`, the
  three tool download directories (`~/.cache` on Linux, `AppData/Local` on
  Windows), the shared `session-state` format and
  the `client_name` value (real installs (Linux, Windows), with the path names
  confirmed in the binary); the WebView2 user data folder and its contents
  (real install (Windows) and binary); the pool and probe sessions without
  `events.jsonl` (real installs (Linux, Windows) and app log); the NULL
  token columns in `data.db` (real installs (Linux, Windows)) and the token
  in the GNOME keyring on Linux (real install (Linux) and binary); the run
  tokens as live local credentials (binary and app log);
  what a session deleted from the app removes and leaves, the checkpoint
  refs' layout and the WAL checkpoint at quit (real installs (Linux,
  Windows)).
- High: the `data.db` tables of [Record schema](#record-schema) with their column definitions and
  row counts, and the joins of [Joins and timestamps](#joins-and-timestamps) (real installs (Linux,
  Windows)); the differences of the app's session logs from the CLI's (real installs, and the SDK
  declarations for the hook payloads).
- Medium: the token in Windows Credential Manager (binary
  imports and the credential file's modification time, target name not
  read); the macOS data directory (dirs crate, not observed); the macOS
  token store; the shapes of the schedule and automation rows the parser
  writes, which rest on the table definitions alone, all four tables having
  been empty ([Schedules](#schedules)).
- Not determined: when `data.db`'s token columns are
  filled; how git reaches `git-credential-copilot`; what `ep/` holds; where
  `attachments`, `workspaces/*/attachments` and `crash-reports` appear and
  what they hold; whether Foundry Local model weights are downloaded
  without the user asking; whether a CLI session that is never prompted
  writes `events.jsonl` (if not, project discovery no longer finds its
  `cwd`, [Where the project path is recorded](#where-the-project-path-is-recorded)); the sizes of
  the WebView2 `blob_storage` and `Shared Dictionary` directories.
- Not determined, for the parser: the format of `session_checkpoints.event_id` against the log's
  event `id`s, beyond the four checkpoints whose ref paths matched it
  ([Joins and timestamps](#joins-and-timestamps)); whether the `allowAll*` keys of
  `session.permissions_changed` are a schema the SDK declarations will
  name; the table names and shapes of builds before 1.1.29
  ([Format versions](#format-versions)); whether `session_deletion_intents`
  is ever filled, since it stayed empty through both deletions.
