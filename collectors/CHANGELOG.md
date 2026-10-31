# Changelog

All notable changes to the collectors are recorded here. The sh and
PowerShell collectors share one version, printed by `--version` and
`-Version` and recorded in `collection.json`. Changes to the manifest
(`manifest.jsonl` fields and status values) and to `collection.json` are
called out, since analysts and the analyzer depend on them.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added

- Hermes' remaining credential stores are flagged `secret: true` in every home and profile (`.hermes*/` and `AppData/Local/hermes*/`): the config writers' `config.yaml.*` copies, `bot-desktop/*`, `webapp/*`, `feishu_comment_pairing.json`, `workspace/meetings/node_token.json` and the OS credential directories `.ssh/*`, `.aws/*`, `.gnupg/*`, `.kube/*`, `.docker/*`, `.azure/*`, `.config/gh/*`, `.config/gcloud/*` and `.envrc`; in a Hermes volume, whose `_data` is the container user's home, the volume-relative `config.yaml.*`, `bot-desktop/*`, `webapp/*`, `feishu_comment_pairing.json`, `workspace/meetings/node_token.json`, `.ssh/*`, `.aws/credentials`, `.aws/config`, `.gnupg/*`, `.kube/*`, `.docker/*`, `.azure/*`, `.config/gh/*`, `.config/gcloud/*`, `.envrc` and the quick snapshots' `state-snapshots/*/config.yaml` and `state-snapshots/*/feishu_comment_pairing.json` ([#326]).

### Fixed

- The volume-relative secret glob `profiles/*` of OpenHands' LLM profiles is narrowed to `profiles/*.json`: it applied in every volume, so in a Hermes volume it flagged every file of every named profile, `state.db` and transcripts included, and `--no-secrets` skipped them; a profile's JSON credential files, such as `profiles/<name>/auth.json`, stay flagged ([#326]).

## [1.20.0] - 2026-10-29

### Added

- Claude Code's `.claude/settings.json` and `.claude/settings.local.json`, in the home and in a project, and a project's `.mcp.json` are flagged `secret: true`: they can hold MCP server `env` and `headers` values, `env.ANTHROPIC_API_KEY` and `apiKeyHelper` ([#248]).
- Gemini CLI's `settings.json`, whose `mcpServers` entries take `env` and `headers` values, is flagged `secret: true` in `.gemini`, `.cache/.gemini` and a project's `.gemini` (glob `*.gemini/settings.json`) ([#317]).
- Antigravity's shared `.gemini/config/mcp_config.json` is flagged `secret: true` ([#248]).
- Copilot CLI's `.copilot/settings.json` (a sandbox proxy password), the MCP secret store `.copilot/mcp-secrets/`, the pre-migration `.config/.copilot/config.json`, `mcp-config*` and `mcp-oauth-config*`, and a project's `.github/copilot/settings.json`, `.github/copilot/settings.local.json` and `.github/mcp.json` are flagged `secret: true` ([#248]).
- A project's `.vscode/mcp.json` is flagged `secret: true`, like the editor's `User/mcp.json` ([#248]).
- Cline's legacy MCP settings file in the editor's extension storage, `*/User/globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json`, is flagged `secret: true` ([#248]).
- Roo Code's MCP settings file (`settings/mcp_settings.json` and its pre-migration name `cline_mcp_settings.json`) in the extension's storage and in the CLI's `.vscode-mock/global-storage/settings/`, and a project's `.roo/mcp.json`, are flagged `secret: true` ([#248]).
- A project's root `opencode.json` and `opencode.jsonc` are flagged `secret: true`, like the `.opencode/` copies ([#248]).
- Amp's `.config/amp/settings.json` and its `settings.jsonc` fallback, and a project's `.amp/settings.json` and `.amp/settings.jsonc`, are flagged `secret: true`: their `amp.mcpServers` entries take `env` and `headers` values ([#248]).
- Factory Droid's `.factory/settings.json`, `.factory/settings.local.json` and `.factory/mcp.json`, in the home and in a project, are flagged `secret: true` ([#248]).
- Kiro's agent files `.aws/amazonq/cli-agents/*.json` and a project's `.amazonq/cli-agents/*.json`, and a project's legacy `.amazonq/mcp.json`, are flagged `secret: true` ([#248]).
- OpenHands' per-conversation `acp/codex/auth.json` is flagged `secret: true` under `.openhands/` and, volume-relative, under `conversations/` and `agent-canvas/` of an OpenHands volume ([#248]).
- little-coder's `.little-coder/mcp.json` of the v0.0.x releases is flagged `secret: true` ([#248]).
- Hermes' `config.yaml`, whose `mcp_servers` entries take `env` and `headers` values, is flagged `secret: true` in every home and profile (`.hermes*/config.yaml`, `AppData/Local/hermes*/config.yaml`) and at a Hermes volume's root ([#248]).
- Langflow's per-user `_mcp_servers_<id>.json` is flagged `secret: true` in the platformdirs cache directory Langflow uses as its config directory (`.cache/langflow`, `Library/Caches/langflow`, `AppData/Local/langflow/langflow/Cache`) and, as `[0-9a-f]*-*-*-*-*/_mcp_servers_*.json`, in a Langflow volume ([#248]).
- CLAI's `.config/pydantic-clai2/mcp.json` and a project's `.clai/mcp_servers.json` are flagged `secret: true` ([#248]).
- Ollama's copies of other agents' settings files, `.ollama/backup/*` and `.ollama/launch/muse-config/muse/settings.json`, are flagged `secret: true`, and so are their volume-relative forms `backup/*` and `launch/muse-config/muse/settings.json` in an Ollama volume, where `ollama launch` run through `docker exec` writes them ([#320]).
- A project's Crush configuration files `crushrc`, `.crushrc`, `crush.json` and `.crush.json` are flagged `secret: true` ([#323]).
- Project discovery reads Crush's Windows project list, `AppData/Local/crush/projects.json`, beside `.local/share/crush/projects.json`; the PowerShell collector maps its `C:\` paths into the image, the sh collector uses only the `/` paths it holds ([#323]).

### Changed

- Hermes' Electron caches are excluded in suffixed user-data directories too (`.config/Hermes*/Cache` and the `Code Cache` and `GPUCache` forms, on every OS), and so is the desktop updater's download cache `AppData/Local/hermes-updater`, which the suffixed-home line reaches ([#324]).
- Hermes' installer checkout is excluded in suffixed homes and profiles too (`.hermes*/hermes-agent/[!.]*`, `.hermes*/hermes-agent/.[!e]*` and the `AppData/Local/hermes*/hermes-agent/` forms), keeping the checkout's `.env` ([#324]).

### Fixed

- Hermes homes and desktop user-data directories with a `HERMES_DATA_DIR_SUFFIX` are collected: the suffix is appended with no separator (`.hermes-channel-build-canary`), so the catalog lines are `.hermes?*`, `AppData/Local/hermes?*` and `Hermes?*` user-data directories instead of `.hermes_*` and `AppData/Local/hermes_*`, and the desktop connection files are flagged in each ([#324]).
- `windsurf|AppData/Roaming/devin` is replaced by the Devin CLI's `AppData/Roaming/devin/mcp_config.json` and `AppData/Roaming/devin/credentials.toml`, so on a case-insensitive disk, where that directory is the Devin IDE's `AppData/Roaming/Devin`, the whole IDE user-data directory is no longer collected under this line, and on a case-sensitive disk only those two CLI files are ([#321]).
- Factory Droid's environment dumps `.factory/temp/env/*` are collected and flagged `secret: true`: the `.factory/temp` exclusion, which hid them, is narrowed to `.factory/temp/[!e]*` ([#322]).

### Removed

- The exclusions no catalog line reaches: Amp's `.cache/amp/runner-desktop`, `.cache/amp/desktop`, `.cache/amp/bin` and `.cache/amp/obelisk-runs`, Ollama's `Library/Caches/ollama` and `Library/Caches/com.electron.ollama`, and Windsurf's `AppData/Local/devin/bin` ([#322], [#321]).

## [1.19.0] - 2026-10-28

### Added

- The GitHub CLI login store, `.config/gh/hosts.yml` and `AppData/Roaming/GitHub CLI/hosts.yml`, is collected under the `shared` catalog agent (the cross-agent block) and flagged as a credential (`secret: true`): it names the GitHub account an agent acted as and holds the token when `gh` had no OS credential store ([#256]).
- The git metadata of every discovered or `-p` project is collected as
  `project`: `.git/HEAD`, `.git/config`, `.git/FETCH_HEAD`,
  `.git/packed-refs`, `.git/refs`, `.git/logs` and `.git/worktrees`, which
  name the checked-out branch, the remotes with their fetch and push URLs,
  the last fetch, the branches and tags, the reflog and the linked
  worktrees; the object store, index, hooks, LFS objects and submodule
  repositories are not collected, each linked worktree's staging area and
  `modules/` are excluded (`.git/worktrees/*/index`,
  `.git/worktrees/*/index.lock`, `.git/worktrees/*/sharedindex.*`,
  `.git/worktrees/*/modules`, a `skipped_excluded` row each), and
  `.git/config` is not flagged `secret` although a remote URL can carry
  a token ([#253]).
- Claude Code's Windows temp root, `AppData/Local/Temp/claude`, with each
  session's `scratchpad/` and the `tasks/<id>.output` files of background
  commands and subagents, is collected as `claude-code` (observed with
  Claude Code 2.1.309 on Windows 11, October 2026) ([#256]).
- A project's `.git/info/exclude`, which Claude Code rewrites with
  patterns for its own files, is collected as `project` ([#256]).
- Claude Code's session daemon files `.claude/daemon/control.key`,
  `.claude/daemon/pipe.key` and `.claude/daemon/roster.json`, and the
  `.claude.json` copies in `.claude/backups/` (`.claude.json.backup.<epoch>`
  and `.claude.json.corrupted.<timestamp>`, glob `.claude/backups/.claude.json.*`),
  are flagged `secret: true` ([#256]).
- The older home-level `.claude.json.backup*` files, which the
  `.claude.json*` catalog line collects and which hold the same account
  and MCP server settings as `.claude.json`, are flagged `secret: true`,
  in the home and in a `~/.claude-<name>` config home ([#256]).
- A `~/.claude-<name>` config home's session keys, session daemon files
  and `.claude.json` backups are collected and flagged `secret: true`
  through catalog lines of their own, `.claude-*/sessions/*.*.key`,
  `.claude-*/daemon/control.key`, `.claude-*/daemon/pipe.key`,
  `.claude-*/daemon/roster.json` and `.claude-*/backups/.claude.json.*`,
  since such a home is collected only by its Claude-specific names and a
  secret glob alone collects nothing ([#256]).
- `.claude/plugins/plugin-directory-cache*`, Claude Code's downloaded copy
  of the plugin directory, is excluded ([#256]).
- Claude Code's `.local/state/claude`, whose `locks/<version>.lock` names
  the process ID, binary path and version of a running session, and
  `.cache/claude`, its `staging/` directory, are collected as
  `claude-code`, at the same paths under the home on Linux and Windows
  (observed with Claude Code 2.1.310 on Ubuntu 26.04, October 2026)
  ([#256]).
- Claude Code's macOS URL handler, `~/Applications/Claude Code URL Handler.app`,
  which registers the `claude-cli:` URL scheme and whose executable is a
  symbolic link to `~/.local/bin/claude` (recorded as a `symlink` row), is
  collected as `claude-code` (observed with Claude Code 2.1.310 on macOS
  26.6, October 2026) ([#256]).
- Claude Code's `.claude/ide/<port>.lock`, the editor connection file that
  holds an `authToken` while an IDE session runs, and
  `.claude/bridge-spawn/<id>/mcp.json`, the Remote Control bridge's
  definition of its remote MCP server with session-bound headers, are
  flagged `secret: true`; a `~/.claude-<name>` config home's copies are
  collected through catalog lines of their own,
  `.claude-*/ide/*.lock` and `.claude-*/bridge-spawn/*/mcp.json`, and
  flagged too ([#256]).

### Changed

- Claude Code's `.claude/paste-cache` is no longer excluded: it holds the
  text of each long pasted prompt, which `history.jsonl` names only by
  hash ([#256]).
- The comment lines of the catalog tables, which `--list` and `-List`
  print, are shorter: each names what its lines cover and the research
  document to read, without issue numbers, dates or build histories. No
  catalog line changed ([#277]).

## [1.18.0] - 2026-10-21

### Added

- Catalog lines and secret globs from the 2026-10-19 source review's
  issues, taken from each agent's source; the
  [real-install rounds](../AGENTS.md#real-install-rounds) that
  confirm them are still owed.
- A project's `.crewai` directory (CrewAI's `last_run.json`, the record
  that a run's trace reached AMP, and the `crewai eval` state) is
  collected as `crewai` ([#223]).
- The CLAI 2.0 state directory `.local/state/pydantic-clai2` (the web
  orchestrator conversation, workspace records and logs) is collected as
  `pydantic-clai`; its workspaces' clones of remote repositories are
  excluded, and each workspace's `token` is flagged `secret: true`
  ([#225]).
- difyctl's `login-pending.yml`, a pending OAuth device login, is
  flagged `secret: true` in `.config/difyctl`, `AppData/Roaming/difyctl`
  and `AppData/Local/difyctl` ([#224]).
- Sourcegraph Cody's JetBrains plugin files are collected: `options/cody*.xml`
  in each JetBrains IDE, `~/.sourcegraph`, and a project's
  `.idea/cody_history.xml` and `.idea/.sourcegraph`; the account list
  `options/cody_accounts.xml` is flagged secret, since its
  `customRequestHeaders` can carry proxy credentials, and so are
  `options/cody.xml`, `~/.sourcegraph/cody_settings.json` and a project's
  `.idea/.sourcegraph/cody_settings.json`, whose `cody.mcpServers` entries
  carry MCP server `headers` and `env` ([#214]).
- The Agent Zero WhatsApp bridge's login state `tmp/whatsapp/session/` is
  flagged secret, in home, Desktop and volume forms; the A0 CLI
  connector's computer-use screen captures in
  `AppData/Local/Temp/_a0_connector` are collected on Windows ([#217]).
- Project discovery reads Agent Zero editor (ACP) chats' `acp_cwd` and
  `acp_additional_directories` ([#217]) and archived Codex threads in
  `~/.codex/archived_sessions` ([#222]).
- A discovered nanobot workspace's `cron/jobs.json`, `cron/runs/`,
  `triggers/triggers.json` and `triggers/runs/` are collected ([#220]).
- The OpenCode desktop app's data directory is collected, with its
  compile cache excluded, and so is the Windows temp root
  `AppData/Local/Temp/opencode`; OpenCode's `service.json` and
  `service-<channel>.json` are flagged secret ([#213]).
- Kilo Code's `kilo-sandbox-policy` and `kilo-sandbox-preference`
  directories are collected ([#221]).
- OpenHands Agent Canvas `mars-credentials.json` (DigitalOcean
  connections) is flagged secret on macOS, Linux and Windows ([#218]).
- Qwen Code's running-session registry `~/.qwen/sessions/<pid>.json`,
  which holds the session's `ipcToken`, is flagged secret ([#230]).
- `~/.roo/cli-credentials.json`, the Roo Code CLI's cloud token in
  releases up to 3.53.0, is flagged secret ([#159]).
- The twinny-server forge plugins' `plugins/<id>/repos.json`, which holds
  saved access tokens, is flagged secret ([#215]).
- The Hermes desktop app's `connection.json` and `connections.json`,
  which hold a remote gateway's token, are flagged secret ([#208]).

- An exclusion for Cursor's plugin cache, `.cursor/plugins/cache`,
  recorded as `skipped_excluded` with its size. It holds the marketplace
  copy of each installed plugin (`<marketplace>/<plugin>/<sha>/` with
  `.cursor-plugin/plugin.json`, `mcp.json`, a README, a licence and
  assets), which can be downloaded again, as the Claude Code and Codex
  plugin caches already excluded can. Which plugins were installed stays
  collected, in `state.vscdb` and in the per-project
  `.cursor/projects/<project>/mcps/plugin-<name>-<server>/` manifests
  ([#236]).
- An exclusion for the copy of Cursor's Windows updater,
  `*/Cursor/update-supervisor/*/inno_updater.exe` (3.4 MB), recorded as
  `skipped_excluded` with its size. Cursor's update to 3.26.16 on
  Windows (October 2026) wrote it into
  `update-supervisor/win32/<installationId>/` beside `app-ready.json` and
  `state.json`, which stay collected; it is a copied binary that the
  macOS supervisor exclusion did not cover ([#236]).

### Changed

- The Linux catalog line for Cursor's agent sync stores now names where
  the Cursor IDE writes them, the XDG state directory
  `.local/state/cursor/agent-stores` (the per-account and
  per-conversation `cursor_agent_stores/<id>/` stores and the
  `.control/<id>/pins/` of the open conversations), in place of
  `.config/Cursor/AgentStores`, which matched nothing. Cursor 3.25.9 from
  the Ubuntu deb (October 2026) wrote the stores there, so collector
  1.17.0 left them out on Linux. `--list` (`-List`) prints the new line
  ([#236]).
- The Windows catalog line for Cursor's agent sync stores now names
  `AppData/Local/Cursor/AgentStores`, where Cursor 3.26.13 (Windows 11,
  October 2026) writes them, in place of
  `AppData/Roaming/Cursor/AgentStores`, which matched nothing, so
  collector 1.17.0 left them out on Windows. `--list` (`-List`) prints
  the new line ([#236]).
- The sh collector tests a file against the credential globs with one
  compiled pattern instead of one test per glob, so a collection whose
  workers run under bash, posh or a BSD sh is faster. The manifest is
  unchanged: the same files are flagged `secret: true` or
  `skipped_secret` ([#242]).
- Gemini CLI `--worktree` checkouts (`.gemini/worktrees`) and Qwen Code
  worktrees (`.qwen/worktrees`) are excluded, in a home and inside a
  project ([#216]).
- OpenClaw's Gateway repository clones (`projects/<16 hex>/<name>`) are
  excluded, and the volume-relative forms of the clone, `plugin-skills`
  and agent `tmp` and `.tmp` exclusions are added ([#219]).
- OpenHands `~/.openhands/workspaces` (Canvas-in-Docker automation runs)
  is excluded by default, like `agent-canvas/workspaces` ([#218]).
- Hermes quick snapshots (`state-snapshots/`) are collected instead of
  excluded; their `.env`, `auth.json` and pairing copies stay flagged, in
  volumes too ([#123]).
- Twinny's per-workspace extension storage
  (`workspaceStorage/<id>/rjmacarthy.twinny`, the `agent-changes/`
  records) is attributed to `twinny` instead of the editor ([#215]).
- Project discovery no longer takes a Codex or Open Interpreter thread
  worktree (`.codex/worktrees`, `.openinterpreter/worktrees`) for a
  project ([#222]).

### Fixed

- OpenClaw's legacy `exec-approvals.json` (the approvals socket token),
  `~/.openclaw-auth-profile-secrets/` and a volume-root
  `exec-approvals.json` are flagged `secret: true` ([#219]).

### Removed

- The secret glob `.config/crush/.crushrc`, which matched a file Crush
  never reads ([#231]).

## [1.17.0] - 2026-10-20

### Added

- Catalog lines for two more Cursor directories in the editor's
  user-data directory (`Library/Application Support/Cursor`,
  `.config/Cursor` and `AppData/Roaming/Cursor`), collected as `cursor`:
  `AgentStores`, the sync stores of the agent's files per account and per
  conversation, with the pins of the open conversations, and
  `update-supervisor`, one directory per in-app update whose
  `events.jsonl`, `result.json` and `app-ready.json` record which build
  ran from when. `--list` (`-List`) prints the six lines ([#6]).

### Changed

- Four Cursor subtrees are excluded by default and recorded as
  `skipped_excluded` with their size, where they were collected before:
  the copy of the Cursor Agent CLI that the IDE's agent worker runs,
  `*/User/globalStorage/anysphere.cursor-agent-worker/agent-cli` (608 MB
  on a macOS install, almost all of that install's collected bytes), the
  update supervisor binary
  `*/Cursor/update-supervisor/*/cursor-update-supervisor`, the canvas type
  declarations `.cursor/projects/*/canvases/node_modules` and the
  feature-flag cache `.cursor/statsig-cache.json`. `--full` (`-Full`)
  still collects them ([#6]).

## [1.16.0] - 2026-10-19

### Added

- A `codex-cli|Documents/Codex` catalog line: the ChatGPT desktop app's
  projectless output directories, `Documents/Codex/<date>/<slug>` with
  `work/` and `outputs/` inside, where the bundled Codex runs a thread
  that has no project, are collected whole and attributed to
  `codex-cli`, so the files the agent wrote there are in the archive;
  exclusions, the size limit and the secret globs apply as everywhere,
  and `--list` (`-List`) and the inventory evidence print the glob
  ([#149]).

### Changed

- Project discovery drops a Codex rollout `cwd` that is
  `<home>/Documents/Codex` or lies under it, so a projectless output
  directory is no longer listed in `collection.json` `projects` or used
  as a project base (before 1.16.0 it was, and only its project-catalog
  files were collected) ([#149]).

## [1.15.0] - 2026-10-19

### Added

- A secret glob for Codex's shell snapshots, `.codex/shell_snapshots/*`
  ([#160]). Codex, and Codex Desktop inside the ChatGPT app, writes a
  `<thread uuid>.<unix ns>.sh` file at each resume and turn of a live
  thread (the two most recent per thread were kept) with the user's
  exported login-shell environment, so an API key the shell exports is
  copied into it. The files are now flagged `secret: true` and skipped
  with `--no-secrets` (`-NoSecrets`), and `--list` (`-List`) prints the
  glob; 1.14.0 collected them unflagged.

## [1.14.0] - 2026-10-18

### Added

- Secret globs for credential files the session sync research found
  collected without the flag ([#153]); each is flagged `secret: true` and
  skipped with `--no-secrets` (`-NoSecrets`), and `--list` (`-List`)
  prints them:
  - nanobot's remote WebUI pairing keys,
    `.nanobot*/webui/remote-pairing/*/identity` (an unencrypted OpenSSH
    private key) and `.nanobot*/webui/remote-pairing/*/exchange` (an
    X25519 private key).
  - Agent Zero's plugin configuration, which holds the Telegram bot token:
    `agent-zero*/usr/plugins/*/config.json`,
    `Desktop/agent-zero*/usr/plugins/*/config.json` and, in a Docker
    volume, `plugins/*/config.json` and `usr/plugins/*/config.json`.
  - Cline's cached organisation remote config, which holds the prompt
    upload bucket's access keys,
    `*/User/globalStorage/saoudrizwan.claude-dev/cache/remote_config_*.json`,
    and its webhook configuration, `Documents/Cline/webhook_config.json`.
  - Gemini CLI's `/bug-memory` heap snapshot of the CLI process,
    `*.gemini/tmp/*/bug-memory-*.heapsnapshot`.
  - little-coder's legacy `.little-coder/config.json`, which holds a
    GitHub token.
  - Qwen Code's channel credentials, `.qwen/channels/*-credentials.json`
    and `.qwen/channels/weixin/account.json`.

### Changed

- Qwen Code's secret glob `.qwen/channels/*.json` is replaced by the two
  credential globs above. It flagged every file under `channels/`,
  including the `sessions.json` and `daemon/<hash>/routes.json` routing
  tables, which `--no-secrets` then left out; those are now collected
  unflagged in both modes ([#153]).
- Cline's VS Code extension `cache/` directory
  (`*/User/globalStorage/saoudrizwan.claude-dev/cache`) is no longer
  excluded. It holds the cached remote config and `sync-queue.json`, the
  queue of task files uploaded or waiting to be uploaded to the
  organisation's storage bucket, and nothing large ([#153]).
- Kilo Code's log directory `.local/share/kilo/log` is no longer
  excluded: its session service lines (`creating session`, `sharing`,
  `unsharing`, `removing share`) are the only local record of a deletion
  whose remote delete failed ([#153]).

## [1.13.0] - 2026-10-17

### Added

- Manifest status `skipped_special` for a special file: neither a regular
  file, a directory nor a symbolic link, such as the Unix domain sockets
  Codex CLI 0.162.1 keeps under `.codex`. The file is never opened or
  copied; the row keeps its size, times and `secret` flag and has an empty
  `archive_path`, `sha256`, `target` and `error`. This is a new status
  value in `manifest.jsonl` ([#120]).
- Manifest `type` values for a `skipped_special` row: `socket`, `fifo`,
  `device` (block or character) and `other`. The sh collector names the
  kind with `test -S`, `-p`, `-b` and `-c`; the PowerShell collector names
  it from `UnixMode` under PowerShell 7.1 and later on Linux and macOS, and
  records a Windows reparse point that is neither a symbolic link nor a
  junction (an AF_UNIX socket) as `other` ([#120]).
- `collection.json` `counts.skipped_special`, between `skipped_secret`
  and `error_copy` in both collectors ([#120]).

- GitHub Copilot Desktop (the Tauri app, dpkg package `github`, 1.1.29
  on a real Linux install) is a new catalog agent, `copilot-desktop`. The
  app runs its sessions through a bundled Copilot CLI that writes them to
  `.copilot/session-state` in the CLI's format, so those stay
  `copilot-cli`; [nested entries](docs/coverage.md#nested-entries) inside
  `.copilot` claim the app's own
  files: `data.db*`, `repo-metadata-cache.db*`, `chats`, `repos`, `run`,
  `media-cache`, `canvas-catalog-probe` and `logs/github-app.*.log`. The
  app's data directory (`.local/share/com.github.githubapp`,
  `Library/Application Support/com.github.githubapp`,
  `AppData/Roaming/com.github.githubapp`) and Foundry Local's
  `.github-copilot-cli` are collected as `copilot-desktop` too ([#147]).
- Exclusions for Copilot Desktop: `.copilot/repos` (the working trees of
  the repositories the app clones and their worktrees; which ones were added
  is recorded in `data.db`), the bundled skills under `app-skills`, and on
  Linux the web view caches `WebKitCache` and `CacheStorage` of the app's
  data directory. Each clone's `.git` directory is its own entry,
  `.copilot/repos/*/.git`, collected as `copilot-desktop` less its object
  store and index (`.copilot/repos/*/.git/objects`, `.copilot/repos/*/.git/index`),
  so the checkpoint refs under `refs/copilot/checkpoints`, `packed-refs`,
  `logs`, `HEAD`, `config` and `worktrees` survive a deleted session in the
  archive ([#147]).
- Secret glob `.copilot/run/*.token`: the app's local WebSocket auth
  tokens `ws.token` and `ws.release.token` ([#147]).
- Copilot Desktop on Windows (1.1.30 on a real install): the WebView2 user
  data folder `AppData/Local/com.github.githubapp`, where the Windows
  build keeps the web view's Local Storage (the composer draft and tab
  lists), is collected as `copilot-desktop` ([#147]).
- Exclusions for that folder's Chromium caches and downloaded components,
  under `AppData/Local/com.github.githubapp/EBWebView/`:
  `Default/Cache`, `Default/Code Cache`, `Default/GPUCache`,
  `Default/DawnGraphiteCache`, `Default/DawnWebGPUCache`,
  `Default/blob_storage`, `Default/Shared Dictionary`, `GrShaderCache`,
  `ShaderCache`, `GPUPersistentCache`, `component_crx_cache`,
  `extensions_crx_cache`, `WidevineCdm`, `Subresource Filter`,
  `hyphen-data`, `PKIMetadata`, `CertificateRevocation` and
  `BrowserMetrics-spare.pma`; at least 71.4 MB of the folder's 72.6 MB on the
  install ([#147]).
- Secret globs for that folder's `Local State` (Chromium's profile key,
  wrapped with DPAPI), `Default/Network/Cookies*` and `Default/Login Data*`,
  as for ChatGPT Desktop's Chromium profile ([#147]).
- Exclusion `.copilot/chats/*/node_modules`: the dependency tree of a
  project the agent builds in a Copilot Desktop chat's working directory
  (3,411 files and 14.7 MB on the Windows install); the files the agent
  wrote there stay collected ([#147]).
- Exclusions `.cache/copilot/marketplaces`,
  `Library/Caches/copilot/marketplaces` and
  `AppData/Local/copilot/marketplaces`: the Copilot CLI's git clones of
  plugin marketplaces (263 MB and 4,560 files on the Windows install).
  The `skipped_excluded` row keeps the size; which marketplaces were
  cloned is inside and is not collected without `--full` ([#131]).

### Changed

- The stdout `skipped:` line ends with the number of special files:
  `skipped:    N excluded, N too large, N secret, N special` ([#120]).
- The sh walk gives every path that is not a directory to the stage
  worker (`find ! -type d`), not only regular files and symlinks, so a
  socket or FIFO under a matched directory gets a `skipped_special` row
  instead of no row at all. `--inventory` still counts regular files only
  ([#120]).
- The PowerShell collector records a Windows reparse point as `symlink`
  only when its `LinkType` is `SymbolicLink` or `Junction`. Up to 1.12.0
  every reparse point was a `symlink`, so an AF_UNIX socket under `.codex`
  got a `symlink` row whose `target` was its own path and was recreated
  in the archive as a link to itself ([#120]).
- Project discovery skips a Copilot CLI session `cwd` (from
  `.copilot/session-state/*/workspace.yaml`) that lies under the same
  home's `.copilot` directory: Copilot Desktop's per-chat directories
  under `.copilot/chats` and its worktrees under `.copilot/repos` were
  listed as projects ([#147]).
- Project discovery reads a Copilot CLI session's `workspace.yaml` `cwd`
  only when the session directory has an `events.jsonl`. Copilot Desktop
  pre-creates pool and probe sessions that are never prompted and have
  none; on Windows their `cwd` is the app's install directory,
  `AppData/Local/Programs/GitHub Copilot`, which was listed as a project
  ([#147]).

## [1.12.0] - 2026-10-15

### Added

- The Antigravity desktop app's Electron user-data directory is collected
  whole as `antigravity`: `Library/Application Support/Antigravity`,
  `.config/Antigravity` and `AppData/Roaming/Antigravity`, which replace
  the pre-2.0 `User` and `logs` lines for the same directories. The 2.x
  app (2.23.0, macOS, checked on a real install) has neither
  subdirectory; on Linux and Windows its logs are in the `logs/` child,
  which the whole-directory line still covers ([#83]).
- `Library/Logs/Antigravity`, the desktop app's macOS logs directory
  (`main.log`, `language_server.log`), is collected as `antigravity`
  ([#83]).
- Antigravity plugin OAuth credentials,
  `.gemini/antigravity*/plugin_data/<plugin>/token.json` (the Google Docs
  plugin writes one), are flagged `secret: true` ([#84]).
- The Antigravity CLI's file secret store `plugin-secrets.json`, which
  CLI 1.3.4 uses for plugin credentials when no OS keychain is available,
  is collected as `antigravity` and flagged `secret: true`:
  `.config/antigravity/plugin-secrets.json` (named in the CLI's own log
  on Linux), `Library/Application Support/antigravity/plugin-secrets.json`
  and `AppData/Roaming/antigravity/plugin-secrets.json`. The secret globs
  accept `Antigravity` as well, since on a case-insensitive filesystem
  the file shares the desktop app's Electron user-data directory
  ([#90]).
- `AppData/Local/antigravity` is collected as `antigravity`: the Windows
  counterpart of `.cache/antigravity`, the plugin marketplace staging
  directory under Go's user cache directory, which the desktop app 2.23.1
  created (empty) on a Windows 11 install. The installer's
  `AppData/Local/antigravity-updater` and the app's program directory
  `AppData/Local/Programs/antigravity` are not matched ([#83]).
- The ChatGPT desktop app's Windows package, renamed `OpenAI.Codex` by
  October 2026 (ChatGPT 26.1007.9446.0 from the Microsoft Store, checked
  on a real Windows 11 install), is collected as `chatgpt-desktop`. The
  app is now a Chromium build, and the collector found none of it,
  since the catalog named only the old `OpenAI.ChatGPT-Desktop_*`
  package. The new lines, under
  `AppData/Local/Packages/OpenAI.Codex_*`, name the browser profile
  `LocalCache/Roaming/Codex/web/Codex/Default` (with its storage
  partitions under `Default/Partitions`) and the files beside it
  (`Local State`, `Last Version`, `statsig-state.json`,
  `.codex-tpp-local-executor-installation-id`) rather than the whole
  user-data directory, so its Chromium component data (about 160 MB on
  the install) is not walked; and the app's own log directory
  `LocalCache/Local/Codex` and the package's `Settings` hive. The old
  package lines stay for older installs ([#113]).
- The new ChatGPT profile's credential stores are flagged
  `secret: true`: `Network/Cookies*` and `Login Data*` in `Default` and
  in each `Default/Partitions/<partition>`, and the user-data
  directory's `Local State`, which holds the DPAPI-wrapped key that
  decrypts them ([#113]).
- `.codex/.codex-global-state.json` and its `.bak` copy, which the Codex
  inside the ChatGPT desktop app writes, are flagged `secret: true`: they
  hold ChatGPT conversation resume tokens and the user's prompt history
  ([#115]).
- The ChatGPT desktop app on macOS is collected as `chatgpt-desktop`.
  Since 26.1007 (26.1007.71326, checked on a real macOS 26.6.2 install)
  it is the same Electron build as on Windows, with the bundle id
  `com.openai.codex`, and none of the earlier `com.openai.chat` paths
  exist, so the collector found none of it. The new lines name the
  Chromium profile `Library/Application Support/Codex/Default` and the
  files beside it (`Local State`, `Last Version`, `statsig-state.json`,
  `.codex-tpp-local-executor-installation-id`,
  `browser-sidebar-page-states.json`), the app's `executor-plugins`
  (a bundled MCP server) and `artifact-sessions` directories,
  `Library/Application Support/com.openai.codex.installer` (install
  identifiers), the log directory `Library/Logs/com.openai.codex`, the
  `com.openai.codex`, `com.openai.codex.installer` and
  `com.openai.sky.CUAService` preferences, the `HTTPStorages` stores and
  binary cookies of `com.openai.codex` and `com.openai.sky.CUAService`
  (the computer-use service), and the computer-use group container
  `Library/Group Containers/2DC432GLL2.com.openai.sky.CUAService`. The
  `com.openai.chat` line stays for the former native app ([#121]).
- The macOS profile's `Cookies*` and `Login Data*`, in `Default` and in
  each `Default/Partitions/<partition>`, and the two `.binarycookies`
  files are flagged `secret: true` ([#121]).

### Changed

- The Antigravity `bin/` exclusion covers every `.gemini` data directory
  (`antigravity`, `antigravity-cli`, `antigravity-ide`,
  `antigravity-backup`), not only the CLI's, so the desktop app's
  binaries in `.gemini/antigravity/bin` get one `skipped_excluded` row
  ([#83]).
- Plugin binaries under `.gemini/antigravity*/plugin_data/<plugin>/bin`
  and the plugin marketplace cache `.gemini/antigravity*/cache/marketplaces`
  are excluded; the rest of `cache/` is still collected ([#83]).
- The Chromium caches in the Antigravity Electron user-data directory are
  excluded: `Cache` (named under each of the three locations, since the
  PowerShell match ignores case and `*/Antigravity/Cache` would also prune
  `.gemini/antigravity/cache`), `Code Cache`, `GPUCache`,
  `DawnGraphiteCache`, `DawnWebGPUCache`, `blob_storage`,
  `Shared Dictionary`, `CachedData` and `Crashpad` ([#83]).
- The Antigravity Electron user-data directory's `Dictionaries`
  (Chromium's spell-check dictionary, 452 KB on Linux), the sidecar SDK
  the desktop app installs under `.gemini/antigravity*/lib/node_modules`,
  and its copy under `.gemini/config/node_modules` are excluded ([#83]).
- Manifest change: files collected from a project directory now carry the
  agent that owns them in the manifest `agent` field (`claude-code` for a
  project's `.claude/` and `CLAUDE.md`, `cursor` for `.cursor/`, `aider`
  for `.aider.chat.history.md`, `little-coder` for `.pi/approved-plan.md`
  inside pi's `.pi/`), taken from the `PROJECT_CATALOG` table that
  `--list` prints. Files several agents read (`AGENTS.md`,
  `AGENTS.override.md`, `AGENT.md`, `.agents/`, `.mcp.json`,
  `.github/copilot-instructions.md`, `.github/git-commit-instructions.md`,
  `.github/instructions/`, `SOUL.md`, `USER.md`) keep the agent
  `project`, which until now every project row had ([#104]).
- Both collectors are reorganised for readability: the same twelve
  sections in the same order, a reader's guide at the top, a comment on
  every function, the long functions split by step, and matching names
  for the sh and PowerShell functions that do the same step (listed in
  [docs/internals.md](docs/internals.md)). Options, `--list` output,
  the manifest, `collection.json`, the log, stdout, the archive and the
  exit codes are unchanged ([#99]).
- The Chromium caches of the ChatGPT desktop profile are excluded, in
  `Default` and in every `Default/Partitions/<partition>`: `Cache`,
  `Code Cache`, `GPUCache`, `DawnGraphiteCache`, `DawnWebGPUCache`,
  `blob_storage`, `Extensions` (a bundled extension's code) and
  `Service Worker/CacheStorage` and `ScriptCache` ([#113]).
- `.codex/plugins/.plugin-appserver`, where the ChatGPT desktop app
  copies the Codex binaries (`codex.exe`, 283 MB, and three helpers of
  7 to 70 MB that were collected), and the empty staging directory
  `.codex/plugins/.remote-plugin-install-staging` are excluded ([#116]).
- `Library/Application Support/com.openai.codex` is attributed to
  `chatgpt-desktop` instead of `codex-cli`: the bundle id that owns it is
  now the ChatGPT app, which bundles Codex ([#121]).
- The Chromium caches of the macOS ChatGPT profile are excluded, in
  `Default` and in every `Default/Partitions/<partition>`, as on Windows,
  and so is the bundled MCP server's script
  `executor-plugins/<plugin>/server.mjs` (949 KB). The computer-use app
  bundle the app copies into `.codex/computer-use/`
  (`Codex Computer Use.app`, 77 MB) is excluded with `.codex/computer-use/*.app`; its
  `config.json` is still collected ([#121]).

### Fixed

- Project discovery in the sh collector now matches `file://` URIs.
  From 1.3.0 the pattern required a slash after `file:///`, so no URI
  ever matched: the Antigravity desktop app's project folders in
  `.gemini/config/projects/*.json` (`folderUri`) were never discovered,
  nor was a `file://` URI in the other files read the same way (the
  Antigravity CLI's `settings.json` and `cache/projects.json`, Kiro's
  `session.json`, Goose's `recent-dirs.json` and Twinny's embedding
  manifests); plain absolute paths in them were unaffected. A Windows
  URI (`file:///C:/x`, also `file:///c%3A/x`) now yields the
  drive-relative `/x`, which image mode prefixes with the root, as the
  PowerShell collector already did ([#91]).
- The sh collector's home patterns (`home/*`, `Users/*`, `usr/home/*`,
  `export/home/*`) are expanded under the root only. Since 1.0.0 the shell
  expanded them against the current directory first, so in image mode,
  run from a directory with a `home` or `Users` subdirectory (`/` on
  macOS), the homes found only by those patterns were lost: a Windows
  image yielded no users, and a Linux image only those in its
  `etc/passwd` ([#105]).
- The sh collector clears its list of claimed paths (`NESTED_CLAIMS`, the
  subtrees that nested catalog entries collect) at startup. The Docker
  and Podman volume walks read it without setting it, so a value in the
  environment the collector was started with pruned the paths it named
  from every matched volume, with no manifest row ([#106]).
- The sh collector tests a catalog match that is a file, not only a
  directory, against the exclusions: an excluded file match now gets one
  `skipped_excluded` row with its size instead of being collected, and
  `--inventory` no longer counts it, as in the PowerShell collector. A
  file named `.local/bin/.muse-update.<suffix>`, matched by the Muse Code
  catalog line `.local/bin/.muse-*` and by the exclusion
  `.local/bin/.muse-update.*`, was collected whole. `--full` still
  collects it ([#107]).
- The PowerShell collector reports an `-OutputDir` that is a file, or lies
  under one, as `Cannot create output dir: <dir>` and exits 2, as the sh
  collector does. `New-Item` neither fails nor creates the directory
  there, so a path under a file went on to print a `Resolve-Path` error,
  a `ProviderPath` error from strict mode and then
  `Output dir not writable: <dir>`, and a path that is a file printed the
  last message alone ([#108]).
- The sh collector no longer expands the exclusion globs, or the claimed
  paths of nested entries, against its working directory. Run from a
  directory holding paths that a wildcard exclusion matches (a home with
  `.claude/plugins/<name>/node_modules`, say), the glob was replaced by
  those paths and stopped matching the collected homes, so their trees
  under it were collected instead of getting one `skipped_excluded` row
  ([#109]).
- The sh collector unsets `CDPATH` at startup. With `CDPATH` in the
  environment, a relative `-r` or `-o` was looked up in the `CDPATH`
  directories first, and `cd` printed the directory it chose, so the
  root or output directory could become a same-named directory elsewhere,
  recorded as a two-line path ([#110]).
- The PowerShell collector records a `collected` row's `size` as the
  number of bytes it copied, the length the `sha256` covers. It used the
  file's length from the directory listing, which NTFS updates only when
  a handle on the file closes, so for a file a running app holds open
  and writes the manifest could say `0` or `32` bytes beside an archived
  copy of 218,747 or 210,152 (Antigravity's `language_server.log` and
  Chromium's `DIPS-wal`), and `collected_bytes` understated the archive.
  Other statuses keep the listing's length ([#111]).
- The PowerShell collector reads project discovery sources with shared
  access and logs `discovery source unreadable: <path>: <message>` when
  one still cannot be read. It used `File.ReadAllText`, which fails on a
  Codex rollout that a running ChatGPT desktop app holds open, and
  dropped the error silently, so a live run on Windows found no project
  although both rollouts named one ([#114]).

## [1.11.0] - 2026-10-12

### Added

- Claude Desktop installed as an MSIX package on Windows (Anthropic's
  Windows download, not a Microsoft Store app) is collected as
  `claude-desktop`: its data directory under
  `AppData/Local/Packages/Claude_*/LocalCache/Roaming/Claude` (and the
  `AnthropicPBC.Claude_*` family), where MSIX virtualisation stores what
  the app reports as `AppData/Roaming/Claude` ([#73]).
- `AppData/Local/Claude` (the MSIX build's logs) and
  `AppData/Local/Claude-Data` (an alternative Windows data directory) are
  collected as `claude-desktop` ([#73]).
- The MSIX build's `config.json` (OAuth token cache), `Cookies*`,
  `host-creds-*.json` and `ccd-session-secrets/*` are flagged
  `secret: true` under both package families ([#73]).
- Claude Desktop's space-link signing key,
  `spaces-present/<acct>/<org>/link-key` under the data directory, is
  flagged `secret: true` (`*/Claude*/spaces-present/*/link-key`) ([#73]).
- Claude Code's per-process session key,
  `.claude/sessions/<pid>.<hash>.key` (and the same under a relocated
  `.claude-*` home), is flagged `secret: true`; the `<pid>.json` beside it
  stays unflagged ([#76]).
- The official Linux `.deb` of Claude Desktop (2.31226.1) was checked
  against the catalog on a real install: the data directory and its logs
  are under `.config/Claude`, already collected as `claude-desktop`, and
  the CLI the app downloads under `claude-code/<version>/<hash>/` is
  excluded by `*/Claude*/claude-code` ([#73]).
- Claude Code's CLI cache directory, which holds per-project MCP server
  logs (`<project slug>/mcp-logs-<server>/<timestamp>.jsonl`), is
  collected as `claude-code` from `Library/Caches/claude-cli-nodejs`
  (macOS), `.cache/claude-cli-nodejs` (Linux) and
  `AppData/Local/claude-cli-nodejs/Cache/*` (Windows). The Windows entry
  names the children of `Cache/` because the PowerShell collector's
  case-insensitive exclusion `*/Claude*/Cache` would otherwise prune the
  directory itself ([#81]).
- The macOS standard build of Claude Desktop (2.31226.1) was checked
  against the catalog on a real install: the data directory, the logs,
  `Partitions/`, `claude-code-vm` (the Linux CLI build the Cowork VM runs)
  and `vm_bundles` behave as documented and nothing new needed collecting;
  the Touch ID WebAuthn metadata secret in `Preferences`, collected
  unflagged, is documented as a key to redact ([#73]).

### Fixed

- The PowerShell collector collects paths longer than 260 characters under
  Windows PowerShell 5.1 without `LongPathsEnabled` on the host: every
  file system call uses the extended-length form (`\\?\`), so a deep source
  file is enumerated and its staged copy is written, where before the copy
  failed as `error_copy` and a source tree past 260 characters was not
  listed. Manifest paths and the archive layout are unchanged ([#77]).
- In the PowerShell collector, a directory that a wildcard catalog segment
  cannot list gets an `error_read` row instead of being skipped silently
  ([#77]).
- `tests/smoke.sh` passes on a macOS host: it strips the trailing slash
  from `$TMPDIR` and resolves the work directory before building the fake
  image, and hashes the backslash-named file from an extracted copy of the
  archive, because bsdtar reads a backslash in a member name as an escape
  ([#78]).
- On macOS and the BSDs, a catalog match directory that cannot be listed or
  entered (an agent's state directory with no read permission) left no row
  in the manifest, only a `find` line in the log; it now gets one
  `error_read` row with the OS error, as on Linux ([#82]).

## [1.10.3] - 2026-10-07

### Added

- The PowerShell collector has its own tar writer, used when `tar.exe` is
  missing or fails: it writes the same `.tar.gz` layout with pax headers
  for non-ASCII and long names, and keeps the staged symlinks.
  `collection.json` `capabilities.archiver` has a new value,
  `PowerShell tar writer`, for an archive it wrote ([#72]).
- `CAC_ARCHIVER`, a test hook for the PowerShell collector, sets the
  archivers to try (`tar`, `writer`, `zip`) ([#72]).

### Fixed

- On Windows, a collection with a file name outside the ANSI code page
  (CJK, Cyrillic) no longer falls back to a zip without saying why:
  some `tar.exe` builds crash on such names, and the collector now logs its exit
  code (`tar.exe failed: exit code 0xC0000005`), adds a `notes` entry to
  `collection.json` for each fallback, and writes the `.tar.gz` with its
  own tar writer ([#72]).
- The PowerShell collector's zip, now the last resort, stores entry names
  with `/` instead of `\` under Windows PowerShell 5.1, so it extracts
  into the `fs/<original path>` layout on Linux and macOS ([#72]).

## [1.10.2] - 2026-10-07

### Changed

- The PowerShell collector's per-file work is faster: the exclusion and
  credential tables are each tested with one regular expression instead of
  a loop over every glob, the staged copy is hashed through the handle that
  wrote it with one SHA-256 object for the run (reopening a file just
  written waited for the antivirus scan on Windows), the owner lookup reads
  only the owner and translates each SID once, staging directories are
  remembered, and the source is no longer read a second time for its
  timestamps. A 5,000-file collection takes about a third of the time under
  Windows PowerShell 5.1 and about half under PowerShell 7 ([#71]).
- The PowerShell collector writes manifest rows itself instead of through
  `ConvertTo-Json`. The rows parse to the same values with the same keys,
  order and types, byte for byte as PowerShell 7 wrote them; under Windows
  PowerShell 5.1, `<`, `>`, `&` and `'` are now written as themselves rather
  than as `\u` escapes ([#71]).
- `collection.json` from the PowerShell collector reports
  `capabilities.hash_tool` as `System.Security.Cryptography.SHA256` instead
  of `Get-FileHash` ([#71]).

## [1.10.1] - 2026-10-07

### Changed

- The sh collector's per-file work forks far fewer processes: manifest rows
  are built without subshells, and files are lstat'ed, copied and hashed in
  chunks of up to 256 with one `stat`, one hasher call and one `cp -p` per
  directory, falling back to per-file calls for a chunk or file the batch
  cannot handle. The manifest is unchanged, rows in the same order; a
  5,000-file collection takes about a tenth of the time it did, and the
  `shasum` path (macOS) about a twentieth ([#71]).
- The sh collector's `find -exec` workers run under `dash` when it is on
  `PATH` and `COLLECTOR_SH` is unset, instead of `sh`; on macOS, whose `sh`
  is bash, this roughly halves the remaining time ([#71]).
- `--inventory` stats each batch of files with one `stat` call ([#71]).

## [1.10.0] - 2026-10-06

### Added

- Catalog agent `autogen-studio`: `~/.autogenstudio`, with `.env` and
  `temp_env_vars.env` flagged as credentials ([#64]).
- Catalog agent `crewai`: `~/.config/crewai`, `~/.crewai`, the `crewai`
  data directory, and CrewAI's run databases (`latest_kickoff_task_outputs.db*`,
  `flow_states.db*`) and `.crewai_user.json` inside the data directory named
  after the project folder (`.local/share/*/...`,
  `Library/Application Support/*/...`), plus `AppData/Local/[Cc]rew[Aa][Ii]`;
  `settings.json` and the `credentials/` token files flagged ([#64]).
- Catalog agent `camel-ai`: `.camel/gmail_token.json` (flagged),
  `.camel/skills` and `.config/camel`, leaving the rest of `~/.camel`,
  which Apache Camel JBang shares, uncollected ([#64]).
- Catalog agent `dify`: the `difyctl` config directories, with `tokens.yml`
  flagged, and the Docker volumes `*dify_agent_local_sandbox_home` and
  `*dify_agent_local_sandbox_workspace` ([#64]).
- Catalog agent `flowise`: `~/.flowise`, with `encryption.key` and
  `*_secret.key` flagged and `vectorstore` excluded ([#64]).
- Catalog agent `langflow`: the config directory (`.cache/langflow`,
  `Library/Caches/langflow`, `AppData/Local/langflow/langflow/Cache`),
  `~/.langflow` and the Langflow Desktop directories; `secret_key`,
  `cache_secret_key` and the Desktop `data/.env` flagged; knowledge bases,
  Desktop virtual environments and caches, and `profile_pictures`
  excluded; the Docker volume `*langflow-data` ([#64]).
- Catalog agent `metagpt`: `~/.metagpt`, with `config2.yaml` and
  `config2.bak` flagged ([#64]).
- Catalog agent `n8n`: `~/.n8n`, `~/.n8n-local-gateway` and
  `~/.n8n-gateway`; `.n8n/config` and `.n8n/ssh/*` flagged; node modules,
  node definitions and SDK templates excluded; the Docker volumes
  `*n8n_data` and `*n8n-data` ([#64]).
- Catalog agent `pydantic-clai`: `~/.pydantic-ai` and
  `~/.config/pydantic-clai2` (CLAI 2.0), with `credentials-*` and the
  Logfire credentials flagged, and the project entry `.clai` ([#64]).
- `open-interpreter`: the legacy Python tool's platformdirs
  `open-interpreter` directories, the older `Open Interpreter*` directories
  and `.cache/open-interpreter`, with `profiles/*` and `config.yaml`
  flagged and the downloaded `models/` excluded ([#64]).
- Volume-relative exclusions `profile_pictures`, `nodes/node_modules`,
  `node-definitions` and `n8n-sdk-templates`, and secret globs
  `secret_key`, `cache_secret_key`, `config` and `ssh/*`, at the end of
  `EXCLUDES` and `SECRET_GLOBS` ([#64]).

## [1.9.0] - 2026-10-06

### Changed

- The Ollama exclusion `.ollama/models` is narrowed to `.ollama/models/blobs`,
  so the model manifests under `.ollama/models/manifests` (an inventory of
  pulled models, with pull times) and the per-blob `models/metadata` files
  are collected; the weights stay `skipped_excluded`. The volume-relative
  `models` exclusion is unchanged, so a Docker Ollama volume's manifests are
  still not collected ([#44]).

### Removed

- The credential pattern `.claude-*/.device-keys.json`, which could never
  match: no catalog line collects that file from a `.claude-<name>` config
  home, and Claude Code keeps it in `~/.claude` whatever `CLAUDE_CONFIG_DIR`
  says. `.claude/.device-keys.json` is still collected and flagged ([#41]).

## [1.8.1] - 2026-10-05

### Fixed

- sh collector: a `tar` that fails inside the `tar | gzip` fallback is now
  detected, logged as `tar | gzip: tar exited N`, and the collector falls
  through to the plain `.tar`, instead of hashing and reporting a truncated
  `.tar.gz` as success ([#40]).

## [1.8.0] - 2026-10-05

Interface changes: the manifest has a new status value, `error_read`;
`collection.json` has new fields `options.no_projects`, `options.projects`
and `counts.error_read`, and the sh collector now writes
`capabilities.archiver`; the PowerShell collector now sets `archive_path`
(and, when the link is not in the archive, `error`) on `symlink` rows; the
stdout `errors:` line now reads `errors:     N (X error_copy, Y error_read)`.

### Added

- `collection.json` `options.no_projects` (boolean) and `options.projects`
  (the `-p`/`--project` or `-Project` values, resolved to absolute paths
  and de-duplicated), and, in the sh collector, `capabilities.archiver`
  (`tar -z`, `tar | gzip` or `tar`) ([#31]).
- Manifest status `error_read`: one `dir` row, with the reason in `error`,
  for a home, project directory or directory inside a walk that cannot be
  listed or entered, and for a `-p`/`-Project` value that is not a
  directory. `collection.json` `counts.error_read` counts them ([#32], [#39]).

### Fixed

- On a case-insensitive filesystem (macOS, Windows), catalog entries that
  differ only by case (`.config/goose` and `.config/Goose`,
  `.config/PearAI/User` and `.config/pearai/User`, `.config/Cursor/User`
  inside `.config/cursor`) no longer collect the same files twice: the first
  catalog line keeps the directory and the later one is logged and skipped,
  also with `--inventory`. On Linux both directories are still collected.
  The sh collector compares device and inode; the PowerShell collector
  compares on-disk names and records paths in their on-disk spelling ([#29]).
- `collection.json` `archive` and `capabilities.archiver` name the archive
  and archiver actually used. Both collectors rewrite the summary and its
  archived copy before each fallback, so they no longer say `.tar.gz` after
  the sh collector's plain `.tar` fallback or `.tar.gz` and `tar.exe` after
  the PowerShell collector's `.zip` fallback ([#31]).
- The PowerShell collector recreates symlinks and other reparse points in
  the archive, with the target string as stored, and sets their
  `archive_path`, as the sh collector does ([#30]). Windows PowerShell 5.1
  makes the link with `cmd.exe`'s `mklink` and skips a link path or target
  containing `%` or `"`, which `cmd.exe` cannot take verbatim; PowerShell
  7 uses .NET. Where Windows refuses to
  create a link (no Administrator or Developer Mode) or the archive is a
  `.zip`, which cannot hold links, the `symlink` row keeps an empty
  `archive_path`, its `error` field says why, and `collection.json`
  `notes` counts them; a late fall back from `tar.exe` to zip removes the
  staged links first and rewrites their manifest rows, so a zip never
  holds a link target's bytes. Manifest change: `error` is now set on such
  `symlink` rows, and `archive_path` on recreated ones.
- A home or a directory inside a home that the collector could not read
  was silent (sh) or only logged; it now has an `error_read` row and a
  `read failed:` log line ([#32]).
- The stdout `errors:` line counts `error_copy` and `error_read` rows
  together and shows each in parentheses, `errors:     3 (1 error_copy,
  2 error_read)`; it counted only `error_copy` before ([#32]).
- The PowerShell collector checks that the output directory is writable
  (a probe file created and removed) and exits 2 with
  `Output dir not writable: <dir>` before collecting, as the sh collector
  does ([#38]).
- A `-p`/`-Project` directory that does not exist was dropped silently; it
  is now logged and gets an `error_read` row. A relative `-p` was ignored
  by the sh collector; both collectors now resolve it against the current
  directory, in image mode too ([#39]).

## [1.7.0] - 2026-10-04

### Added

- Meta's Muse Code CLI as catalog agent `muse-code`, with an evidence
  document under `research/`: `~/.config/muse` and `~/.local/share/muse`
  (the same paths on Windows), `~/.muse`, the launcher's dot files in
  `~/.local/bin/.muse-*`, the Windows install directory
  `AppData/Local/Programs/muse` and the per-session shell sandbox
  `AppData/LocalLow/muse-shell-sandbox-*`. `auth.json` and `settings.json`
  under `.config/muse` are flagged secret. The bundled plugin and skill
  copies, marketplace clones, update staging directories, the Windows
  `muse-bin-*.exe` binary and `.muse/worktrees` are excluded. Projects are
  discovered from `trust.json` and session logs' `workspace_root`, and a
  project's `.muse` directory is collected.
- Claude Code config homes moved with `CLAUDE_CONFIG_DIR` to a
  `~/.claude-<name>` directory such as `~/.claude-work`: `projects/`,
  `file-history/`, `history.jsonl`, `.claude.json*` and the credential
  files inside it are collected as `claude-code`, the credential files and
  `.claude.json` are flagged secret, and its `.claude.json` and
  `history.jsonl` are project discovery sources. The entries name
  Claude-only files, so `~/.claude-code-router`, `~/.claude-mem` and
  `~/.claude-squad` are not collected.
- `collection.json` field `users_with_artifacts`: the users whose home
  produced at least one manifest row. `users` and `homes` still list
  every home scanned.

### Changed

- The stdout `users:` line counts only homes that produced at least one
  manifest row, and the `User` progress line is written only for a home
  with a catalog match.
- The stdout `size:` and `collected:` lines add the size in binary units
  after the byte count, such as `74779093 (71.3 MiB)`.
- The sh collector's final progress line on stderr is `done`; the archive
  path, size and sha256 stay in the log file's `done:` line and on stdout.
  The PowerShell collector now prints the same `done` line.
- PowerShell collector: the internal logging function is renamed from
  `Write-Log` to `Write-CollectorLog` so it no longer shadows a Windows
  built-in command name; output and behaviour are unchanged. Found by
  PSScriptAnalyzer 1.25.0 with `collectors/PSScriptAnalyzerSettings.psd1`.

### Fixed

- sh collector: failed to parse under posh and other pdksh-derived shells,
  which stop a `$(...)` at the `)` of a case pattern. The two such `case`
  statements use the `(pattern)` form, and the shared helper and worker
  programs are read from here-documents without `$(cat <<...)`; the
  programs are byte-identical. The smoke test now passes under posh.

### Removed

- Shell history collection: the `shell-history` catalog agent and its ten
  entries (`.bash_history`, `.zsh_history`, `.zsh_sessions`, `.sh_history`,
  `.history`, `.bash_sessions`, both fish `fish_history` paths and both
  PSReadLine `ConsoleHost_history.txt` paths). Shell history is the EDR's
  job, like process lists and logons. Manifests no longer hold
  `shell-history` rows, and `--list` and `-List` no longer print them.
- General-purpose `.env` files: the `shared|.env` entry for `~/.env` and
  the `project|.env` entry for a `.env` at a discovered project's root.
  They hold whatever secrets the user or project keeps, not only AI keys,
  and are left to the EDR. An agent's own `.env` inside its state
  directory (`~/.codex/.env`, `~/.gemini/.env`, `~/.hermes/.env`, the
  Agent Zero `usr/.env` and the others in `SECRET_GLOBS`) is still
  collected and flagged secret.

## [1.6.0] - 2026-10-04

### Added

- Inventory mode, `--inventory` / `-Inventory`, for fleet audits through
  an EDR console: the collection walk (catalog, exclusions, nested claims,
  Docker volumes, `-u`, `-r`, `--full`, `--no-docker`) with `lstat` only.
  It writes nothing, not even a log; `-o` is ignored with a stderr note.
  Exit code `0` once the walk has run.
- Inventory output on stdout, JSON Lines: one `host` line (`type`, `host`,
  `collector`, `mode`, `at`, `users_scanned`, `users_unreadable`,
  `docker_volumes`).
- Then one `agent` line per user and agent with at least one file after
  exclusions and nested claims (`type`, `host`, `user`,
  `agent`, `files`, `bytes`, `first`, `last`, `projects`, `evidence`);
  `projects` is the user's discovered project count, repeated on each of
  the user's lines, and Docker volumes appear as user `docker`.
- Inventory never opens a credential file: project discovery skips sources
  that match `SECRET_GLOBS`.

### Changed

- PowerShell: the `profile not accessible (skipped)` log line is written
  only for profiles selected by `-Users`.

### Removed

- The live system snapshot. Both collectors no longer write the `live/`
  directory (`system.txt`, `users.txt`, `logins.txt`, `processes.txt`,
  `agent-processes.txt`, `network.txt`, `services.txt`,
  `scheduled-tasks.txt`, and the sh collector's `live/environ/<pid>.txt`
  process environments). The collectors supplement an EDR, which already
  records processes, logins, network connections and services.
- Manifest: the `live` agent name and the `live/<file>` archive paths are
  gone; no row has an empty `path` any more.
- The `--no-live` / `-NoLive` option. Passing it is now a usage error.
- `collection.json`: the `options.no_live` field. The host name stays in
  `hostname`.

## [1.5.0] - 2026-10-04

### Added

- Docker and Podman named volumes are collected ([#28]). Both
  collectors enumerate `var/lib/docker/volumes`,
  `var/lib/containers/storage/volumes` and `ProgramData/Docker/volumes`
  under the root, and the rootless `.local/share/docker/volumes` and
  `.local/share/containers/storage/volumes` under each home, from the
  filesystem only. A volume whose name matches the new `DOCKER_VOLUMES`
  table is collected whole with `user` `docker` and its `_data` directory
  as `home`; exclusion and secret globs apply relative to `_data`.
- New catalog table `DOCKER_VOLUMES` (Agent Zero, Local Deep Research,
  Ollama from source; Tabby, OpenHands, Letta, Hermes, OpenClaw and nanobot
  by inferred name), printed last by `--list` and `-List` under the new
  heading `# docker volumes (agent|volume name glob)`. Consumers of the
  `--list` output that do not know the heading should stop reading there.
- Volume-relative forms of the Agent Zero, Tabby, Ollama, Local Deep
  Research, OpenHands, Hermes, OpenClaw, nanobot and Letta exclusions and
  secret globs, at the end of `EXCLUDES` and `SECRET_GLOBS`.
- `--no-docker` / `-NoDocker` skips the enumeration.
- Manifest: new status value `skipped_unmatched_volume`, one `dir` row
  with its size for each volume that matches no `DOCKER_VOLUMES` line.
- `collection.json`: `options.no_docker`, `counts.skipped_unmatched_volume`,
  a `docker` object (`volumes_found`, `volumes_collected`, `unreadable`,
  `docker_desktop`) and a `notes` list. An unreadable volume directory or a
  volume that cannot be walked is noted there and in a `docker:` line of
  the stdout summary; collection continues and the exit code is unchanged.
  Docker Desktop data (whose Linux volumes are inside a VM disk) is only
  noted.

## [1.4.0] - 2026-10-04

### Added

- Fifteen catalog agents from the October 2026 backlog, each with an
  evidence document under `research/`: `pearai`, `cody`, `twinny`, `tabby`,
  `open-interpreter`, `openhands`, `pi`, `little-coder`, `letta`, `hermes`,
  `openclaw`, `nanobot`, `agent-zero`, `shellgpt` and `local-deep-research`.
  Several of them are general-purpose or chat-channel agents rather than
  coding agents. Sourcegraph Cody and Twinny have nested entries inside
  every VS Code family editor's globalStorage, like Cline and Continue, and
  little-coder's prompt history is a nested entry inside pi's `~/.pi`.
- 82 home catalog entries, 33 project catalog entries, 151 exclusions and
  150 secret globs, among them Codex CLI's `.codex/.env`, which was not
  flagged before.
- Twenty-one more project discovery sources in both collectors: Hermes
  checkpoint project records; Letta's session index and local-backend
  transcripts; pi session headers (also little-coder's), misplaced
  sessions, trusted folders, crash log and experimental session metadata;
  Open Interpreter live and archived rollouts; OpenHands workspaces, Agent
  Canvas conversation metadata and CLI conversation state; PearAI
  sessions; OpenClaw `workspace` and `agentDir` from its JSON5 config,
  legacy `clawdbot.json` included; nanobot workspace markers and instance
  config; Tabby `file://` repositories; and the common directory of the
  files in each Twinny embeddings manifest.
- `[!...]` negated classes in exclusion and secret globs now work in the
  PowerShell collector as they do in the sh collector.

### Fixed

- The PowerShell collector dropped `file:///` URIs of POSIX paths found in
  agent state; they now map to the path.

## [1.3.0] - 2026-10-03

### Added

- 60 home catalog entries, among them the remote server data directories of
  VS Code, Cursor, Windsurf, Devin and Kiro (with their agent extensions
  attributed to the extension), Antigravity IDE and `.gemini/config`, the
  Copilot CLI XDG directories and caches, Windsurf transcripts, the Ollama
  desktop app and Zed under Flatpak.
- 33 project catalog entries, 55 exclusions and 80 secret globs, including
  the Cursor and Windsurf credential files, now flagged `secret: true`.
- Ten more project discovery sources in both collectors.
- `collectors/research/` holds one evidence document per catalog agent.

### Fixed

- Exclusions that hid evidence: Factory Droid's cache exclusion now covers
  only its sounds, Kiro's `powers` directory is collected again, the Kilo
  Code worktree exclusions match the real paths, and Codex database backups
  and Goose apps are collected.

## [1.2.0] - 2026-10-03

### Changed

- A catalog entry nested inside another entry's directory now claims its
  subtree: each file is collected once, under the most specific agent.
  Manifest rows for Antigravity CLI files under `~/.gemini/antigravity-cli`
  now have `agent` `antigravity` instead of `gemini-cli`.

### Added

- Nested entries for the Cline, Roo Code, Kilo Code and Continue extensions
  inside the globalStorage of every VS Code family editor (VS Code, VSCodium,
  Cursor, Windsurf, Antigravity IDE, Kiro), on all three platforms, so their
  files are attributed to the extension rather than the editor.

## [1.1.0] - 2026-10-03

### Added

- `Collect-AgentArtifacts.ps1`, the Windows PowerShell 5.1 collector, with
  the same catalog, manifest and archive layout as the sh collector. It reads
  profiles from the registry and `C:\Users`, copies files held open by
  running editors, takes a CIM live snapshot, and writes a `tar.gz` through
  `tar.exe` or a `.zip` on older hosts.
- Manifest rows written by the Windows collector carry two extra fields,
  `owner` and `attributes`, and record `uid`, `gid`, `ctime` as `0` and
  `mode` as `""`.
- Catalog agents `roo-code`, `kilo-code`, `copilot` (the GitHub Copilot
  editor plugin's `github-copilot` directory) and `shared` (`~/.agents`,
  `~/.env` and `AGENTS.md` shared between agents).
- Nine more project discovery sources, and Crush's per-project `.crush`
  session database.
- Exclusions for embedding indexes, shadow git checkpoints, worktrees,
  daemon binaries and Electron caches.
- Secret flags for the Gemini CLI and Goose credential files that replace
  the OS keychain when it is unavailable.

### Changed

- Catalog entries for Cursor, Windsurf, Continue, Amp, Goose, Zed, Kiro,
  Qwen Code, Cline, Augment, Factory Droid, Crush, OpenCode, ChatGPT Desktop,
  Claude Desktop, Gemini CLI, Codex CLI and Aider were rebuilt from each
  tool's source, shipped bundle or official documentation.
- Zed, Goose, OpenCode, Crush, Kilo Code and Amp are looked for under
  `~/.config` and `~/.local/share` on macOS and Windows images too.

### Fixed

- Continue's `index/globalContext.json` is collected; only the embedding
  index beside it is excluded.

## [1.0.0] - 2026-10-03

### Added

- `collect-agent-artifacts.sh`, a single POSIX sh collector with no
  dependencies beyond the base system, for live macOS, Linux and BSD hosts
  and for mounted disk images (`-r`).
- Collection of Claude Code, Claude Desktop, Gemini CLI, Antigravity, Codex
  CLI, Copilot CLI, Cursor, VS Code, Windsurf, Aider, Ollama and other agent
  state, plus shell histories, for every user, and project files from
  repositories referenced in agent state.
- `manifest.jsonl` with one row per path: `user`, `home`, `agent`, `path`,
  `archive_path`, `type`, `size`, `mtime`, `atime`, `ctime`, `btime`, `uid`,
  `gid`, `mode`, `sha256`, `secret`, `status`, `target`, `error`. Status
  values: `collected`, `symlink`, `skipped_excluded`, `skipped_size`,
  `skipped_secret`, `error_copy`.
- `collection.json` run summary, a `fs/<original path>` archive layout, a
  live system snapshot, and a SHA-256 of the archive.
- Options `-o`, `-r`, `-u`, `-p`, `--full`, `--no-secrets`, `--no-live`,
  `--no-projects`, `--max-file-size`, `-k`, `--list`, `-q`, `-V`, `-h`.
- Default size exclusions for model weights, caches and extension binaries,
  overridden by `--full`; credential files collected and flagged
  `secret: true` by default.

[Unreleased]: https://github.com/seanthegeek/doubleagent/compare/collector-v1.20.0...HEAD
[1.20.0]: https://github.com/seanthegeek/doubleagent/compare/collector-v1.19.0...collector-v1.20.0
[1.19.0]: https://github.com/seanthegeek/doubleagent/compare/collector-v1.18.0...collector-v1.19.0
[1.18.0]: https://github.com/seanthegeek/doubleagent/compare/collector-v1.17.0...collector-v1.18.0
[1.17.0]: https://github.com/seanthegeek/doubleagent/compare/collector-v1.16.0...collector-v1.17.0
[1.16.0]: https://github.com/seanthegeek/doubleagent/compare/collector-v1.15.0...collector-v1.16.0
[1.15.0]: https://github.com/seanthegeek/doubleagent/compare/collector-v1.14.0...collector-v1.15.0
[1.14.0]: https://github.com/seanthegeek/doubleagent/compare/collector-v1.13.0...collector-v1.14.0
[1.13.0]: https://github.com/seanthegeek/doubleagent/compare/collector-v1.12.0...collector-v1.13.0
[1.12.0]: https://github.com/seanthegeek/doubleagent/compare/collector-v1.11.0...collector-v1.12.0
[1.11.0]: https://github.com/seanthegeek/doubleagent/compare/collector-v1.10.3...collector-v1.11.0
[1.10.3]: https://github.com/seanthegeek/doubleagent/compare/collector-v1.10.2...collector-v1.10.3
[1.10.2]: https://github.com/seanthegeek/doubleagent/compare/collector-v1.10.1...collector-v1.10.2
[1.10.1]: https://github.com/seanthegeek/doubleagent/compare/collector-v1.10.0...collector-v1.10.1
[1.10.0]: https://github.com/seanthegeek/doubleagent/compare/collector-v1.9.0...collector-v1.10.0
[1.9.0]: https://github.com/seanthegeek/doubleagent/compare/collector-v1.8.1...collector-v1.9.0
[1.8.1]: https://github.com/seanthegeek/doubleagent/compare/collector-v1.8.0...collector-v1.8.1
[1.8.0]: https://github.com/seanthegeek/doubleagent/compare/collector-v1.7.0...collector-v1.8.0
[1.7.0]: https://github.com/seanthegeek/doubleagent/compare/collector-v1.6.0...collector-v1.7.0
[1.6.0]: https://github.com/seanthegeek/doubleagent/compare/collector-v1.5.0...collector-v1.6.0
[1.5.0]: https://github.com/seanthegeek/doubleagent/compare/collector-v1.4.0...collector-v1.5.0
[1.4.0]: https://github.com/seanthegeek/doubleagent/compare/collector-v1.3.0...collector-v1.4.0
[1.3.0]: https://github.com/seanthegeek/doubleagent/compare/collector-v1.2.0...collector-v1.3.0
[1.2.0]: https://github.com/seanthegeek/doubleagent/compare/collector-v1.1.0...collector-v1.2.0
[1.1.0]: https://github.com/seanthegeek/doubleagent/compare/collector-v1.0.0...collector-v1.1.0
[1.0.0]: https://github.com/seanthegeek/doubleagent/releases/tag/collector-v1.0.0

[#326]: https://github.com/seanthegeek/doubleagent/issues/326
[#324]: https://github.com/seanthegeek/doubleagent/issues/324
[#323]: https://github.com/seanthegeek/doubleagent/issues/323
[#322]: https://github.com/seanthegeek/doubleagent/issues/322
[#321]: https://github.com/seanthegeek/doubleagent/issues/321
[#320]: https://github.com/seanthegeek/doubleagent/issues/320
[#317]: https://github.com/seanthegeek/doubleagent/issues/317
[#277]: https://github.com/seanthegeek/doubleagent/issues/277
[#256]: https://github.com/seanthegeek/doubleagent/issues/256
[#253]: https://github.com/seanthegeek/doubleagent/issues/253
[#248]: https://github.com/seanthegeek/doubleagent/issues/248
[#242]: https://github.com/seanthegeek/doubleagent/issues/242
[#236]: https://github.com/seanthegeek/doubleagent/issues/236
[#231]: https://github.com/seanthegeek/doubleagent/issues/231
[#230]: https://github.com/seanthegeek/doubleagent/issues/230
[#225]: https://github.com/seanthegeek/doubleagent/issues/225
[#224]: https://github.com/seanthegeek/doubleagent/issues/224
[#223]: https://github.com/seanthegeek/doubleagent/issues/223
[#222]: https://github.com/seanthegeek/doubleagent/issues/222
[#221]: https://github.com/seanthegeek/doubleagent/issues/221
[#220]: https://github.com/seanthegeek/doubleagent/issues/220
[#219]: https://github.com/seanthegeek/doubleagent/issues/219
[#218]: https://github.com/seanthegeek/doubleagent/issues/218
[#217]: https://github.com/seanthegeek/doubleagent/issues/217
[#216]: https://github.com/seanthegeek/doubleagent/issues/216
[#215]: https://github.com/seanthegeek/doubleagent/issues/215
[#214]: https://github.com/seanthegeek/doubleagent/issues/214
[#213]: https://github.com/seanthegeek/doubleagent/issues/213
[#208]: https://github.com/seanthegeek/doubleagent/issues/208
[#160]: https://github.com/seanthegeek/doubleagent/issues/160
[#159]: https://github.com/seanthegeek/doubleagent/issues/159
[#153]: https://github.com/seanthegeek/doubleagent/issues/153
[#149]: https://github.com/seanthegeek/doubleagent/issues/149
[#147]: https://github.com/seanthegeek/doubleagent/issues/147
[#131]: https://github.com/seanthegeek/doubleagent/issues/131
[#123]: https://github.com/seanthegeek/doubleagent/issues/123
[#121]: https://github.com/seanthegeek/doubleagent/issues/121
[#120]: https://github.com/seanthegeek/doubleagent/issues/120
[#116]: https://github.com/seanthegeek/doubleagent/issues/116
[#115]: https://github.com/seanthegeek/doubleagent/issues/115
[#114]: https://github.com/seanthegeek/doubleagent/issues/114
[#113]: https://github.com/seanthegeek/doubleagent/issues/113
[#111]: https://github.com/seanthegeek/doubleagent/issues/111
[#110]: https://github.com/seanthegeek/doubleagent/issues/110
[#109]: https://github.com/seanthegeek/doubleagent/issues/109
[#108]: https://github.com/seanthegeek/doubleagent/issues/108
[#107]: https://github.com/seanthegeek/doubleagent/issues/107
[#106]: https://github.com/seanthegeek/doubleagent/issues/106
[#105]: https://github.com/seanthegeek/doubleagent/issues/105
[#104]: https://github.com/seanthegeek/doubleagent/issues/104
[#99]: https://github.com/seanthegeek/doubleagent/issues/99
[#91]: https://github.com/seanthegeek/doubleagent/issues/91
[#90]: https://github.com/seanthegeek/doubleagent/issues/90
[#84]: https://github.com/seanthegeek/doubleagent/issues/84
[#83]: https://github.com/seanthegeek/doubleagent/issues/83
[#82]: https://github.com/seanthegeek/doubleagent/issues/82
[#81]: https://github.com/seanthegeek/doubleagent/issues/81
[#78]: https://github.com/seanthegeek/doubleagent/issues/78
[#77]: https://github.com/seanthegeek/doubleagent/issues/77
[#76]: https://github.com/seanthegeek/doubleagent/issues/76
[#73]: https://github.com/seanthegeek/doubleagent/issues/73
[#72]: https://github.com/seanthegeek/doubleagent/issues/72
[#71]: https://github.com/seanthegeek/doubleagent/issues/71
[#64]: https://github.com/seanthegeek/doubleagent/issues/64
[#44]: https://github.com/seanthegeek/doubleagent/issues/44
[#41]: https://github.com/seanthegeek/doubleagent/issues/41
[#40]: https://github.com/seanthegeek/doubleagent/issues/40
[#39]: https://github.com/seanthegeek/doubleagent/issues/39
[#38]: https://github.com/seanthegeek/doubleagent/issues/38
[#32]: https://github.com/seanthegeek/doubleagent/issues/32
[#31]: https://github.com/seanthegeek/doubleagent/issues/31
[#30]: https://github.com/seanthegeek/doubleagent/issues/30
[#29]: https://github.com/seanthegeek/doubleagent/issues/29
[#28]: https://github.com/seanthegeek/doubleagent/issues/28
[#6]: https://github.com/seanthegeek/doubleagent/issues/6
