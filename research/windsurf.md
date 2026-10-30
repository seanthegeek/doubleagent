# windsurf

Windsurf is a closed-source editor with the VS Code layout whose agent is Cascade; the editor is
being renamed Devin Desktop. The same vendor's Codeium plugins (JetBrains, Eclipse, nvim) share
its `~/.codeium` directory, and the Devin CLI is its command-line agent. Its parser is in the
backlog ([issue 52](https://github.com/seanthegeek/doubleagent/issues/52)).

## Source and evidence

Closed source. docs.windsurf.com now 307-redirects to docs.devin.ai (observed on
`/windsurf/cascade/mcp` and `/troubleshooting/windsurf-common-issues`).

- L2 official docs: docs.devin.ai `desktop/devin-desktop-faq`, `desktop/cascade/mcp`,
  `desktop/cascade/memories`, `desktop/cascade/workflows`, `desktop/cascade/hooks`,
  `desktop/troubleshooting/windsurf-common-issues`, `cli/reference/commands` (credentials.toml
  wording from search snippet of that page).
- L3 vendor tracker:
  [Exafunction/codeium issue 295](https://github.com/Exafunction/codeium/issues/295)
  (`.windsurf\extensions` on Windows).
- L4 community source, read locally:
  [wookat/DevCleaner](https://github.com/wookat/DevCleaner) `conversation.rs`,
  [Cosmos-0118/AgentSweep](https://github.com/Cosmos-0118/AgentSweep) `rules.toml`,
  [vinzdg/codenotch](https://github.com/vinzdg/codenotch) `DevinCredentials.swift`,
  [jlcodes99/cockpit-tools](https://github.com/jlcodes99/cockpit-tools) `windsurf_account.rs`;
  [Exafunction/windsurf.nvim](https://github.com/Exafunction/windsurf.nvim) `lua/codeium/api.lua`.

### Observed

No real install.

### Source reviews

None.

## Collection

### Per-user storage

#### Scratch directories

None known.

#### IDE user data

IDE user data (FAQ): macOS `~/Library/Application Support/Windsurf/` legacy and
`~/Library/Application Support/Devin/` new; Windows `%APPDATA%\Windsurf\` and `%APPDATA%\Devin\`;
Linux `~/.config/Windsurf/` and `~/.config/Devin/`. Each holds the VS Code layout
(`User/settings.json`, `User/globalStorage/`, `Workspaces/`, `argv.json`, `logs/`, `User/History/`).
"Migrate from Windsurf" copies the legacy dir and extensions dir and skips `logs/`, `crashpad/`,
`User/History/`, so both trees can exist with different content. Devin Desktop reads both and writes
to Devin paths.

#### Extensions

Extensions: `~/.windsurf/extensions/` legacy (read-only after migration), `~/.devin/extensions/` new
(FAQ; [Exafunction/codeium issue 295](https://github.com/Exafunction/codeium/issues/295)).

#### Cascade hook transcripts

Cascade transcripts for hooks: `~/.windsurf/transcripts/{trajectory_id}.jsonl` (docs cascade/hooks,
`transcript_path`); the docs name no content field beyond `type` and `status`. Post-rename
`~/.devin/transcripts` is unverified.

#### Codeium directory

Central Codeium dir `~/.codeium/` on every OS (FAQ: "not changing in this release"; Windows
`C:\Users\<u>\.codeium\windsurf\cascade` per troubleshooting page):

- `windsurf/cascade/` conversation history as `<uuid>.pb` protobuf files with no published schema;
  deleting the dir clears chat history (troubleshooting page; DevCleaner
  `conversation.rs:1199-1229`). `ItemTable` keys `cascade.chatdata`, `cascade.conversations`,
  `cascade.*`, `windsurf.cascadeViewContainerId.*` in `state.vscdb` hold view state
  (DevCleaner `:41-85`).
- `windsurf/memories/` auto memories plus `windsurf/memories/global_rules.md` (docs memories);
  `windsurf/global_workflows/*.md` (docs workflows); `windsurf/skills/`; `windsurf/hooks.json` (IDE)
  and `hooks.json` (JetBrains plugin) (docs hooks); `windsurf/mcp_config.json` and
  `mcp_config.json`, `user_settings.pb`, `windsurf/user_settings.pb`, `installation_id` (FAQ;
  AgentSweep 1107-1113).
- `windsurf/database/`, `database/` "local Cascade/Windsurf database", `windsurf/code_tracker/`,
  `windsurf/codemaps/`, `brain/`, `recipes/`, `context_state/`, `windsurf/implicit/`, `implicit/`,
  `windsurf/native_storage_migrations.lock`, `ws-browser/`, `ws-browser-profile/` (AgentSweep
  1078-1149, labels only).
- `windsurf/bin/` language-server and CLI binaries (FAQ); `~/.local/bin/devin` (macOS/Linux),
  `%LOCALAPPDATA%\devin\bin\` (Windows).
- `~/.windsurf-server/data/User` and `~/.devin-server/data/User`: the data directory of the
  editor's remote server on an SSH or container host, by the VS Code remote-server layout the
  fork inherits ([Remote server in `vscode.md`](vscode.md#remote-server)). Both are catalog lines
  by analogy only: no source, bundle or real install has shown them for Windsurf or Devin.
- Plugins (JetBrains, Eclipse, nvim) share `~/.codeium/`: `codeium.log` (plugin logs search
  snippet), language server under a configured manager dir (windsurf.nvim `api.lua:147-167`).

#### Devin CLI

Devin CLI / local agent: `~/.config/devin/mcp_config.json` (`$XDG_CONFIG_HOME/devin/`),
`%APPDATA%\devin\mcp_config.json` (docs cascade/mcp); `~/.local/share/devin/credentials.toml`
(`$XDG_DATA_HOME/devin/`), `%APPDATA%\devin\credentials.toml` (cli/reference/commands; codenotch
`:23-26`). Its session store was not found in docs. On a case-insensitive disk `%APPDATA%\devin`
is the same directory as the Devin IDE's `%APPDATA%\Devin`, so the catalog names the CLI's
`mcp_config.json` and `credentials.toml` there rather than the directory.

#### System-level files

System-level, outside home:
`/Library/Application Support/{Devin,Windsurf}/{rules,workflows,skills,hooks.json}`,
`/etc/{devin,windsurf}/...`, `C:\ProgramData\{Devin,Windsurf}\...` (docs memories, workflows,
hooks).

#### Environment variables

Env: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `WINDSURF_API_KEY` (docs).

### Credentials

- IDE: `User/globalStorage/state.vscdb` `ItemTable` key `windsurfAuthStatus` with JSON `apiKey`,
  `email` (codenotch `:16-20, 73-83`; cockpit-tools `:1133-1148`); multi-account keys
  `codeium.windsurf-windsurf_auth` and `windsurf_auth-%` (cockpit-tools `:1167-1185`). Same file as
  chat view state: collect unflagged, redact by key.
- `~/.codeium/config.json` `{"api_key": ...}` written by the nvim/emacs plugins (windsurf.nvim
  `api.lua:60-90`, config_path default).
- Devin CLI `credentials.toml` field `windsurf_api_key`, persistent and non-expiring (docs;
  codenotch `:86-96`).
- `mcp_config.json` files (all three locations) embed server env and support
  `${file:~/.secrets/...}` indirection (docs cascade/mcp).

### Exclusions

`.codeium/windsurf/bin`, `.codeium/bin`, `.codeium/*/bin` (binaries, FAQ), `.codeium/ws-browser`,
`.codeium/ws-browser-profile` (embedded Chromium, AgentSweep 1078-1085),
`.codeium/windsurf/implicit` (an earlier finding: a multi-GB embedding cache; AgentSweep lists it
unclassified), `.windsurf/extensions`, `.devin/extensions`. `AppData/Local/devin/bin` is outside
every catalog line, so it is not collected and needs no exclusion. The top-level
`.codeium/implicit`, which AgentSweep also lists, is not excluded and is collected with
`.codeium`. Keep `database`, `code_tracker`, `codemaps`, `brain`, `memories`, `cascade`.

### Project-local files

`.devin/rules/*.md` preferred, `.windsurf/rules/*.md` fallback, legacy `.windsurfrules`;
`.devin/workflows/*.md`, `.windsurf/workflows/*.md`; `.devin/skills/`, `.windsurf/skills/`;
`.devin/plans/`, `.windsurf/plans/`; `.devin/hooks.json`, `.windsurf/hooks.json`,
`.devin/hooks.v1.json`; ignore files `.devinignore`, `.windsurfignore`, `.codeiumignore` (FAQ
workspace table; docs memories, workflows, hooks; cli commands). No per-project database.

### Where the project path is recorded

Not determinable from available evidence. Memories are "associated with the workspace they were
created in" (docs memories) but the key is inside the files, and neither the Cascade `.pb` files
([Codeium directory](#codeium-directory)) nor the hook transcripts
([Cascade hook transcripts](#cascade-hook-transcripts)) have a documented field for it. Fall back
to `User/workspaceStorage/*/workspace.json` in the Windsurf and Devin data dirs.

## Parsing

Windsurf is `backlog`: Cascade's history is on disk ([Per-user storage](#per-user-storage)), and the
parser is [issue 52](https://github.com/seanthegeek/doubleagent/issues/52), blocked on format
research because the Cascade `.pb` protobuf files have no published schema; no record schema has
been researched yet.

## Confidence

- High (L2, or two independent L4 sources): the IDE data dirs old and new, the extensions dirs,
  `~/.codeium/windsurf/{cascade,memories,global_workflows,mcp_config.json,hooks.json,bin}`,
  `windsurfAuthStatus`, `credentials.toml` and the project directories.
- Medium: the `.pb` naming (one L4 tool), whether `~/.windsurf/transcripts` still exists after the
  rename, the `implicit` size, and the `database`, `code_tracker` and `codemaps` contents.
- Not determined: the Devin CLI session and log directory; the `.pb` schema; whether
  `~/.devin/transcripts` replaces `~/.windsurf/transcripts`; the `windsurf-next` beta paths; any
  Windows `%LOCALAPPDATA%` use beyond `devin\bin`.
