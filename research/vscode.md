# vscode

VS Code, VS Code Insiders, VSCodium and the fork family (Positron, Kiro, Cursor, Windsurf and
others) are hosts for AI extensions (catalog agent `vscode`): GitHub Copilot Chat and other chat
participants keep their state in the editor's own user data directory, which this entry collects.
VS Code's built-in chat store is used by GitHub Copilot Chat and by any other chat participant, so
parsing it covers Copilot Chat without reading the extension's own files; the same layout exists
in VSCodium, Cursor, Windsurf and the other forks whose `User` directories the catalog collects.
The `vscode` parser reads that chat store.

## Source and evidence

Evidence levels: L1 is source, L4 community documentation, L5 a read-only names-and-sizes listing
of this workstation's VS Code directories (see [Observed](#observed)). Paths cited are relative to
the [microsoft/vscode](https://github.com/microsoft/vscode) clone.

- L1 source: [microsoft/vscode](https://github.com/microsoft/vscode) at
  [`d7622a529314a4abcefd7ca9e3d7344bc9ccba9e`](https://github.com/microsoft/vscode/commit/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e),
  MIT
  ([`LICENSE.txt`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/LICENSE.txt)),
  open source; [microsoft/vscode-copilot-chat](https://github.com/microsoft/vscode-copilot-chat) at
  [`5863f5a7088958050792b5dccbe8b46c6e13eccc`](https://github.com/microsoft/vscode-copilot-chat/commit/5863f5a7088958050792b5dccbe8b46c6e13eccc);
  [posit-dev/positron](https://github.com/posit-dev/positron) `product.json`
  (raw.githubusercontent); VSCodium `prepare_vscode.sh`.
- L4 community: Kiro IDE `product.json` copy ([Bad3r/dotfiles](https://github.com/Bad3r/dotfiles)
  `opt/kiro/resources/app/product.json`), AgentsView docs for Trae,
  [wookat/DevCleaner](https://github.com/wookat/DevCleaner) `conversation.rs` for Trae keys.

The Copilot Chat extension is open source. Its source is now in microsoft/vscode under
`extensions/copilot/`: the
[microsoft/vscode-copilot-chat](https://github.com/microsoft/vscode-copilot-chat) repository was
archived at
[`5863f5a`](https://github.com/microsoft/vscode-copilot-chat/commit/5863f5a7088958050792b5dccbe8b46c6e13eccc)
with a notice that development moved into microsoft/vscode
([`README.md:1-8`](https://github.com/microsoft/vscode-copilot-chat/blob/5863f5a7088958050792b5dccbe8b46c6e13eccc/README.md#L1-L8)),
so every citation into `extensions/copilot/` reads the extension at a microsoft/vscode commit
([`661e013`](https://github.com/microsoft/vscode/commit/661e013024de5052af476650250dabbebb4c1165)
or [`4d22f4f`](https://github.com/microsoft/vscode/commit/4d22f4f922eb012217ac65672ccce02f59d63706)).
Two of the extension's names run through this document. Chronicle is the extension's internal name
for its session tracking and upload code (the `chronicle` directories and the `/chronicle` chat
command). A cloud agent session, session type `copilot-cloud-agent`, is a task run by the Copilot
coding agent on GitHub's servers, which VS Code lists and renders but does not run
([`chatSessionsService.ts:408-417`](https://github.com/microsoft/vscode/blob/661e013024de5052af476650250dabbebb4c1165/src/vs/workbench/contrib/chat/common/chatSessionsService.ts#L408-L417)).

### Observed

- No real-install round. The L5 evidence is a read-only `du` and `ls` of this workstation's VS
  Code directories (the WSL remote server `~/.vscode-server`, the Windows user data directory
  `AppData/Roaming/Code` and `~/.vscode`), with no build or date recorded: it gives the names and
  sizes in [Dot folder](#dot-folder), [Remote server](#remote-server) and
  [Exclusions](#exclusions), opened no file and exercised nothing. No install has shown the sync
  files ([Sync, relay and origin rows](#sync-relay-and-origin-rows)) or the Agents-window layout
  ([Scratch directory rows](#scratch-directory-rows)).

### Source reviews

- 2026-10-21: microsoft/vscode at
  [`4d22f4f`](https://github.com/microsoft/vscode/commit/4d22f4f922eb012217ac65672ccce02f59d63706):
  added the profile and Agents window `state.vscdb` paths of the standing grants, the serialised
  snapshot fields of the chat editing state, VS Code's GitHub account usage records and Copilot
  Chat's repository selection ([Other stores](#other-stores), [Credentials](#credentials)), and
  turned the planned permission, model, standing-grant and editing-state rows into the rows the
  parser now writes ([Parser plan](#parser-plan), [Source host rows](#source-host-rows)).
- 2026-10-19: microsoft/vscode at
  [`4d22f4f`](https://github.com/microsoft/vscode/commit/4d22f4f922eb012217ac65672ccce02f59d63706),
  with the extension under `extensions/copilot/` read at the same commit (the archived repository
  is unchanged at
  [`5863f5a`](https://github.com/microsoft/vscode-copilot-chat/commit/5863f5a7088958050792b5dccbe8b46c6e13eccc)).
  Added the chat editing sessions, hook transcripts, debug logs, memory files, the chronicle index
  and the standing grants ([Other stores](#other-stores)), `permissionLevel`,
  `modelConfiguration` and `sessionCostInfo` ([Record schema](#record-schema)), the Agents window's
  workspace file ([Scratch directories](#scratch-directories)) and the rows planned for them
  ([Parser plan](#parser-plan)); nothing the parser reads changed
  ([Format versions](#format-versions)).
- 2026-10-17: microsoft/vscode at
  [`661e013`](https://github.com/microsoft/vscode/commit/661e013024de5052af476650250dabbebb4c1165),
  for session sync. Added `cloudSessions.json`, `ownedTasks`, external index entries and the cloud
  agent hand-off ([Record schema](#record-schema), [Other stores](#other-stores)), the sync delete
  path ([What survives a delete](#what-survives-a-delete)) and the sync rows
  ([Sync, relay and origin rows](#sync-relay-and-origin-rows)).
- Initial review, date not recorded: microsoft/vscode at
  [`d7622a5`](https://github.com/microsoft/vscode/commit/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e)
  and microsoft/vscode-copilot-chat at
  [`5863f5a`](https://github.com/microsoft/vscode-copilot-chat/commit/5863f5a7088958050792b5dccbe8b46c6e13eccc):
  the user data layout, the chat session store and its record schema, which the parser is written
  from.

## Collection

### Per-user storage

User data dir resolution
([`src/vs/platform/environment/node/userDataPath.ts:51-68`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/platform/environment/node/userDataPath.ts#L51-L68),
[`80-99`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/platform/environment/node/userDataPath.ts#L80-L99)):
`$VSCODE_PORTABLE/user-data`, then `$VSCODE_APPDATA/<nameShort>`, then `--user-data-dir`, else
Windows `%APPDATA%\<nameShort>` (fallback `%USERPROFILE%\AppData\Roaming`), macOS
`~/Library/Application Support/<nameShort>`, Linux `$XDG_CONFIG_HOME/<nameShort>` or
`~/.config/<nameShort>`. Layout is identical on all three OSes; only the parent differs.

`nameShort` / `dataFolderName` / `serverDataFolderName`: Code-OSS `Code -
OSS`/`.vscode-oss`/`.vscode-server-oss`
([`product.json:2-15`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/product.json#L2-L15));
shipped builds `Code`, `Code - Insiders` with `.vscode`, `.vscode-insiders`, `.vscode-server`;
VSCodium `VSCodium`/`.vscode-oss`/`.vscodium-server`, Insiders `VSCodium -
Insiders`/`.vscodium-insiders`/`.vscodium-server-insiders` (`prepare_vscode.sh:68-76, 96-103`);
Positron `Positron`/`.positron`/`.positron-server` (product.json L1); Kiro
`Kiro`/`.kiro`/`.kiro-server` (L4); Cursor `Cursor`/`.cursor`/`.cursor-server`, Windsurf→Devin
`Windsurf`,`Devin`/`.windsurf`,`.devin`, Antigravity `Antigravity`/`.antigravity`, Trae `Trae`,
`Trae CN`, `TRAE SOLO CN` (L4 AgentsView) follow the same convention; see the per-agent documents.

#### Scratch directories

VS Code chat itself has none; an empty-window chat has no project. The Agents window opens
`User/agent-sessions.code-workspace`
([`environmentService.ts:280-281`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/src/vs/platform/environment/common/environmentService.ts#L280-L281)),
a workspace file inside the collected `User` directory rather than a project. For Copilot CLI
sessions the extension creates git worktrees at `<repo parent>/<repo>.worktrees/<branch>`
([`extensions/git/src/repository.ts:1952-1970`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/extensions/git/src/repository.ts#L1952-L1970));
they sit beside the user's repository wherever it is, so no catalog line can collect them, and they
are recorded in the Copilot CLI's `session-state` (see [`copilot-cli.md`](copilot-cli.md)).

#### User data directory

Inside the user data dir
([`src/vs/platform/environment/common/environmentService.ts`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/platform/environment/common/environmentService.ts)):

- `User/` = appSettingsHome
  ([`:56`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/platform/environment/common/environmentService.ts#L56));
  `User/globalStorage/storage.json`
  ([`:65`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/platform/environment/common/environmentService.ts#L65));
  `User/globalStorage/state.vscdb`
  ([`platform/storage/electron-main/storageMain.ts:285`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/platform/storage/electron-main/storageMain.ts#L285),[`353`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/platform/storage/electron-main/storageMain.ts#L353)),
  SQLite `ItemTable(key TEXT UNIQUE, value BLOB)`
  ([`base/parts/storage/node/storage.ts:343`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/base/parts/storage/node/storage.ts#L343)).
- `User/globalStorage/<extension id lowercased>/` per extension
  ([`workbench/api/common/extHostStoragePaths.ts:90`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/workbench/api/common/extHostStoragePaths.ts#L90)).
  `User/workspaceStorage/<id>/<extension id, case as published>/`
  ([`:83`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/workbench/api/common/extHostStoragePaths.ts#L83)),
  e.g. `GitHub.copilot-chat`.
- `User/workspaceStorage/<id>/state.vscdb` and `workspace.json`
  ([`storageMain.ts:415-416`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/platform/storage/electron-main/storageMain.ts#L415-L416),
  [`472-474`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/platform/storage/electron-main/storageMain.ts#L472-L474)).
  `<id>` = md5 of the folder URI plus inode (Linux) or birthtime (macOS/Windows)
  ([`platform/workspaces/node/workspaces.ts:53-73`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/platform/workspaces/node/workspaces.ts#L53-L73));
  `.code-workspace` ids are md5 of the config path
  ([`:32`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/platform/workspaces/node/workspaces.ts#L32)).
- Chat sessions
  ([`workbench/contrib/chat/common/model/chatSessionStore.ts:72-79`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/workbench/contrib/chat/common/model/chatSessionStore.ts#L72-L79),
  [`152`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/workbench/contrib/chat/common/model/chatSessionStore.ts#L152),
  [`738-749`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/workbench/contrib/chat/common/model/chatSessionStore.ts#L738-L749)):
  `User/workspaceStorage/<id>/chatSessions/<sessionId>.jsonl` (append log,
  `chat.useLogSessionStorage`) or `.json`; empty-window chats in
  `User/globalStorage/emptyWindowChatSessions/`; `User/globalStorage/transferredChatSessions/`;
  legacy `User/workspaceStorage/no-workspace/chatSessions/`.
- `User/History/` local file history with `entries.json`
  ([`environmentService.ts:89`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/platform/environment/common/environmentService.ts#L89);
  [`workingCopyHistoryService.ts:59`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/workbench/services/workingCopy/common/workingCopyHistoryService.ts#L59));
  `User/sync/`
  ([`:71`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/platform/environment/common/environmentService.ts#L71));
  `User/profiles/<id>/`
  ([`platform/userDataProfile/common/userDataProfile.ts:273`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/platform/userDataProfile/common/userDataProfile.ts#L273))
  each with `settings.json`, `keybindings.json`, `tasks.json`, `snippets/`, `prompts/`, `mcp.json`,
  `extensions.json`, `globalStorage/`, `agent-plugins/`
  ([`:196-205`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/platform/userDataProfile/common/userDataProfile.ts#L196-L205));
  `User/agent-sessions.code-workspace`
  ([`environmentService.ts:281`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/platform/environment/common/environmentService.ts#L281)).
- `logs/<yyyymmddThhmmss>/window<N>/exthost/<ext id>/`
  ([`environmentService.ts:73-78`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/platform/environment/common/environmentService.ts#L73-L78);
  [`workbench/services/environment/electron-browser/environmentService.ts:109`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/workbench/services/environment/electron-browser/environmentService.ts#L109));
  `Backups/` hot-exit unsaved buffers
  ([`platform/environment/electron-main/environmentMainService.ts:49`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/platform/environment/electron-main/environmentMainService.ts#L49));
  `Workspaces/` untitled
  ([`environmentService.ts:108`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/platform/environment/common/environmentService.ts#L108)).

#### Dot folder

`~/<dataFolderName>/` holds `extensions/` and `extensions/extensions.json`
([`environmentService.ts:131-147`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/platform/environment/common/environmentService.ts#L131-L147);
[`platform/userDataProfile/node/userDataProfile.ts:43`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/platform/userDataProfile/node/userDataProfile.ts#L43)),
`extensions/.obsolete`
([`extensionManagementService.ts:558`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/platform/extensionManagement/node/extensionManagementService.ts#L558)),
`argv.json`
([`:95-101`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/platform/environment/common/environmentService.ts#L95-L101)),
`policy.json`
([`:274`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/platform/environment/common/environmentService.ts#L274)),
`cli/` tunnel-CLI state migrated from `~/.vscode-cli`
([`cli/src/state.rs:148-152`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/cli/src/state.rs#L148-L152)).
Observed on this host: `~/.vscode/{agent-plugins,argv.json,cli,extensions}` (L5).

#### Remote server

`~/<serverDataFolderName>/`
([`src/vs/server/node/server.main.ts:41-47`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/server/node/server.main.ts#L41-L47),
env `VSCODE_AGENT_FOLDER`, flag `--server-data-dir`) holds `data/User/...` (the layout
of [User data directory](#user-data-directory)), `data/Machine/settings.json`, `data/logs/`,
`extensions/` and `bin/<commit>/`. On WSL and SSH targets this is where Copilot Chat sessions
live; this host has `~/.vscode-server/data/User/globalStorage/github.copilot-chat` at 54 MB (L5).

#### Chat editing, Copilot Chat and grant records

More records of each chat sit inside the catalog's `User` entries; their contents are in
[Other stores](#other-stores):

- `User/workspaceStorage/<id>/chatEditingSessions/<session id>/`: the files each chat request
  edited, before and after, and whether the user kept the edit
  ([Chat editing sessions](#chat-editing-sessions)).
- `User/workspaceStorage/<id>/GitHub.copilot-chat/transcripts/`, `debug-logs/` and
  `memory-tool/memories/`, and `User/globalStorage/github.copilot-chat/memory-tool/memories/`:
  Copilot Chat's hook transcripts, debug logs and memory notes
  ([Hook transcripts](#hook-transcripts), [Debug logs](#debug-logs),
  [Memory tool files](#memory-tool-files)).
- `User/globalStorage/github.copilot-chat/session-store.db` (with `-wal`): Copilot Chat's chronicle
  index of sessions and turns ([Copilot Chat's chronicle index](#copilot-chats-chronicle-index)),
  and `cloudSessions.json` beside it, its sync markers ([Sync state](#sync-state)).
- `chat/autoconfirm` and related rows of each `state.vscdb`: the user's standing "always allow"
  grants ([Standing permission grants](#standing-permission-grants)), and in
  `User/globalStorage/state.vscdb` the GitHub account's usage records and Copilot Chat's
  repository selection ([GitHub account records](#github-account-records)).

#### Other agents' records in these files

Extensions keep their state inside the editor's own files, which the catalog attributes to `vscode`
(or to the fork), so a `vscode` row can hold another agent's evidence; the analyzer's Cody and
Twinny parsers read their `state.vscdb` rows across that attribution, and leave the other files
below unread:

- Cody (`sourcegraph.cody-ai`): its global state is one `ItemTable` row keyed by the extension id in
  `User/globalStorage/state.vscdb`, holding the chat history and `cody-model-preferences`, and its
  standing settings (`cody.agentic.context.experimentalOptions`, `cody.mcpServers`) are keys of
  `User/settings.json` ([Per-user storage in `cody.md`](cody.md#per-user-storage)).
- Twinny (`rjmacarthy.twinny`): the `rjmacarthy.twinny` row of the global and workspace
  `state.vscdb` holds its chats and its standing command grants,
  `User/workspaceStorage/<id>/rjmacarthy.twinny/agent-changes/` its per-reply change records
  ([`extHostStoragePaths.ts:82-91`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/src/vs/workbench/api/common/extHostStoragePaths.ts#L82-L91)),
  `twinny.*` keys of `User/settings.json` its settings and tokens, and
  `logs/<session>/window<n>/exthost/rjmacarthy.twinny/Twinny.log` its log channel
  ([Per-user storage in `twinny.md`](twinny.md#per-user-storage)).

#### Portable mode

Portable installs
([`src/bootstrap-node.ts:225-246`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/bootstrap-node.ts#L225-L246)):
`<app>/data` on Windows/Linux, `<app dir>/<applicationName>-portable-data` on macOS, containing
`user-data/`, `extensions/`, `tmp/`. Env: `VSCODE_PORTABLE`, `VSCODE_APPDATA`, `VSCODE_EXTENSIONS`,
`XDG_CONFIG_HOME`.

### Credentials

- Extension secrets are `ItemTable` rows keyed `secret://<key>` in `User/globalStorage/state.vscdb`,
  encrypted with the OS keyring/safeStorage when available
  ([`platform/secrets/common/secrets.ts:19`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/platform/secrets/common/secrets.ts#L19),
  [`148`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/platform/secrets/common/secrets.ts#L148),
  [`227`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/platform/secrets/common/secrets.ts#L227)).
  Collect the DB unflagged; the values are not recoverable offline without the keyring. Forks that
  patched this store plaintext tokens (Cursor `cursorAuth/*`, Windsurf `windsurfAuthStatus`).
- The GitHub sign-in that Copilot Chat and the cloud agent use is held by VS Code's built-in
  GitHub authentication extension as one of those secret-storage rows, under the key `github.auth`
  (`<authority><path>.ghes.auth` for GitHub Enterprise)
  ([`extensions/github-authentication/src/github.ts:231-235`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/extensions/github-authentication/src/github.ts#L231-L235),
  [`common/keychain.ts:16-18`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/extensions/github-authentication/src/common/keychain.ts#L16-L18)),
  so the token is encrypted like any other; the account's login is stored unencrypted in other
  keys of the same database ([GitHub account records](#github-account-records)).
- Tunnel CLI: keyring first, file fallback `~/<dataFolder>/cli/token.json` or
  `token-<namespace>.json`
  ([`cli/src/auth.rs:178-189`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/cli/src/auth.rs#L178-L189),
  [`386`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/cli/src/auth.rs#L386),
  [`416-417`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/cli/src/auth.rs#L416-L417)).
- Files that can embed API keys: `User/settings.json`, `User/mcp.json`, `User/profiles/*/mcp.json`,
  project `.vscode/mcp.json`. Each `mcp.json` form is flagged; `User/settings.json`
  holds every other editor setting too, so it is collected unflagged and redacted by key
  ([Secrets](#secrets)).
- Copilot's own token store `~/.config/github-copilot` belongs to the `copilot` agent
  ([Credentials in `copilot.md`](copilot.md#credentials)).

### Exclusions

Sizes are as observed on this workstation (L5); the paths come from the code (L1). The Copilot Chat
names (`logContextRecordings`, `workspaceRecordings`, `codebase-external.sqlite`,
`editRecordings`) are unchanged at
[`4d22f4f`](https://github.com/microsoft/vscode/commit/4d22f4f922eb012217ac65672ccce02f59d63706).

- `*/User/globalStorage/ms-dotnettools.vscode-dotnet-runtime` 463 MB (WSL) / 834 MB (Windows):
  bundled .NET runtimes.
- `*/User/globalStorage/yzane.markdown-pdf` 371 MB (bundled Chromium);
  `*/User/globalStorage/ms-edgedevtools.vscode-edge-devtools` 113 MB.
- Copilot Chat recorders: `*/User/globalStorage/github.copilot-chat/logContextRecordings` and
  `workspaceRecordings`
  ([`src/extension/inlineEdits/vscode-node/inlineEditProviderFeature.ts:130`](https://github.com/microsoft/vscode-copilot-chat/blob/5863f5a7088958050792b5dccbe8b46c6e13eccc/src/extension/inlineEdits/vscode-node/inlineEditProviderFeature.ts#L130);
  [`workspaceRecorder/vscode-node/workspaceRecorderFeature.ts:57`](https://github.com/microsoft/vscode-copilot-chat/blob/5863f5a7088958050792b5dccbe8b46c6e13eccc/src/extension/workspaceRecorder/vscode-node/workspaceRecorderFeature.ts#L57));
  keep `editRecordings`
  ([`platform/multiFileEdit/common/editLogService.ts:68`](https://github.com/microsoft/vscode-copilot-chat/blob/5863f5a7088958050792b5dccbe8b46c6e13eccc/src/platform/multiFileEdit/common/editLogService.ts#L68),
  an AI edit log).
- Copilot Chat indexes under workspaceStorage: `GitHub.copilot-chat/codebase-external.sqlite`
  ([`codeSearch/externalIngestIndex.ts:172`](https://github.com/microsoft/vscode-copilot-chat/blob/5863f5a7088958050792b5dccbe8b46c6e13eccc/src/platform/workspaceChunkSearch/node/codeSearch/externalIngestIndex.ts#L172))
  and chunk-embedding caches
  ([`workspaceChunkEmbeddingsIndex.ts:70`](https://github.com/microsoft/vscode-copilot-chat/blob/5863f5a7088958050792b5dccbe8b46c6e13eccc/src/platform/workspaceChunkSearch/node/workspaceChunkEmbeddingsIndex.ts#L70),
  [`embeddingsIndex.ts:76-77`](https://github.com/microsoft/vscode-copilot-chat/blob/5863f5a7088958050792b5dccbe8b46c6e13eccc/src/platform/embeddings/common/embeddingsIndex.ts#L76-L77)).
- Other `ms-*`, `redhat.*`, `eamodio.*`, `golang.*`, `rust-lang.*` globalStorage dirs were 1-20 MB
  here; their binaries live in `extensions/`, which is not collected. `User/History` is evidence;
  leave it to the size cap.

### Project-local files

`.vscode/mcp.json` (already catalogued), `.vscode/settings.json` (holds `chat.*`/`github.copilot.*`
settings), `.github/copilot-instructions.md` (catalogued). Copilot prompt/instruction/agent files
under `.github/` were not re-verified in this pass.

### Where the project path is recorded

`User/workspaceStorage/<id>/workspace.json`: `{"folder": "<file URI>"}` or `{"workspace":
"<.code-workspace URI>"}`
([`storageMain.ts:416`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/platform/storage/electron-main/storageMain.ts#L416),
[`472-474`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/platform/storage/electron-main/storageMain.ts#L472-L474)).
Decode `file:///c%3A/...` on Windows. Chat `.jsonl` records carry no cwd.

A session from the Agents window records its folder as
`workingDirectory` (a URI) in the session object and in its `chat.ChatSessionStore.index` entry;
Copilot Chat's `session-store.db` has `sessions.cwd` and `sessions.repository`
([`sessionStore.ts:194-205`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/extensions/copilot/src/platform/chronicle/node/sessionStore.ts#L194-L205)).
Both are JSON inside SQLite or a session file, so discovery leaves them to the analyzer.

## Parsing

### Transcript stores

[`common/model/chatSessionStore.ts`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/workbench/contrib/chat/common/model/chatSessionStore.ts),
under the user data directory (`~/.config/Code/`, `Library/Application Support/Code/`,
`AppData/Roaming/Code/`, likewise for Insiders and VSCodium):

- Workspace: `User/workspaceStorage/<hash>/chatSessions/<sessionId>.jsonl` (1.109 and later, an
  append log) or `<sessionId>.json` (before 1.109, flat JSON)
  ([`:72-73`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/workbench/contrib/chat/common/model/chatSessionStore.ts#L72-L73),
  [`:731-741`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/workbench/contrib/chat/common/model/chatSessionStore.ts#L731-L741)).
  Log mode is the default (`getValue('chat.useLogSessionStorage') !== false`, registered nowhere
  else).
- Empty window: `User/globalStorage/emptyWindowChatSessions/<sessionId>.jsonl|json`
  ([`:72`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/workbench/contrib/chat/common/model/chatSessionStore.ts#L72));
  older `User/workspaceStorage/no-workspace/chatSessions/*.json`
  ([`:75-76`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/workbench/contrib/chat/common/model/chatSessionStore.ts#L75-L76),
  [`:714-729`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/workbench/contrib/chat/common/model/chatSessionStore.ts#L714-L729)).
- Transferred: `User/globalStorage/transferredChatSessions/<sessionId>.json`
  ([`:78`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/workbench/contrib/chat/common/model/chatSessionStore.ts#L78),
  [`:745-750`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/workbench/contrib/chat/common/model/chatSessionStore.ts#L745-L750)).

One file is one chat session.

#### What survives a delete

The chat view's delete, and clearing all sessions (which does the same for each), remove each
session's `.json` and `.jsonl` file and its index entry
([`chatSessionStore.ts:455-482`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/src/vs/workbench/contrib/chat/common/model/chatSessionStore.ts#L455-L482),
[`:493-498`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/src/vs/workbench/contrib/chat/common/model/chatSessionStore.ts#L493-L498),
[`:500-508`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/src/vs/workbench/contrib/chat/common/model/chatSessionStore.ts#L500-L508));
trimming the index to the newest 400 local sessions drops entries and keeps their files
([The session index](#the-session-index)). A per-prompt history file is not stated. What remains
is the chat editing state ([Chat editing sessions](#chat-editing-sessions)), the hook transcript
and debug logs where they were written ([Hook transcripts](#hook-transcripts),
[Debug logs](#debug-logs)), the memory tool files ([Memory tool files](#memory-tool-files)) and,
unless the sessions view's delete ran, the `session-store.db` turns
([Copilot Chat's chronicle index](#copilot-chats-chronicle-index)) and the `cloudSessions.json`
entry ([Sync state](#sync-state)).

The sessions view's delete of a local session calls
`github.copilot.sessionSync.deleteSessionFromCloud`, which deletes the session's `session-store.db`
rows and its `cloudSessions.json` entry, and sends `DELETE /agents/tasks/<cloudTaskId>` to GitHub
only while sync is still on, so a session deleted after sync was turned off keeps its remote copy
and loses its local marker
([`remoteSessionExporter.ts:656-690`](https://github.com/microsoft/vscode/blob/661e013024de5052af476650250dabbebb4c1165/extensions/copilot/src/extension/chronicle/vscode-node/remoteSessionExporter.ts#L656-L690),
[`cloudSessionApiClient.ts:286-287`](https://github.com/microsoft/vscode/blob/661e013024de5052af476650250dabbebb4c1165/extensions/copilot/src/extension/chronicle/node/cloudSessionApiClient.ts#L286-L287)).
The Delete Session Sync Data command (`github.copilot.sessionSync.deleteSessions`) offers local and
cloud or cloud only; both remove the entry
([`remoteSessionExporter.ts:545-616`](https://github.com/microsoft/vscode/blob/661e013024de5052af476650250dabbebb4c1165/extensions/copilot/src/extension/chronicle/vscode-node/remoteSessionExporter.ts#L545-L616)).
Neither delete path writes a log line: the exporter reports through telemetry only.

The analyzer reads the `cloudSessions.json` entries, which also name sessions deleted here, the
`ownedTasks` list and the editing state, a secondary source whose `edit state:` rows keep the
deleted chat's session id, edited files, request ids and model (see
[Secondary sources](../analyzer/docs/output.md#secondary-sources)); it does not read
`session-store.db`, the hook transcripts, the debug logs, the memory files or the index, which the
plan in [Parser plan](#parser-plan) makes secondary sources.

### Record schema

[`common/model/chatModel.ts`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/workbench/contrib/chat/common/model/chatModel.ts).
Top level `ISerializableChatData3`
[`:2196-2227`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/workbench/contrib/chat/common/model/chatModel.ts#L2196-L2227):
`version: 3`, `sessionId`, `creationDate` (ms epoch), `initialLocation`, `requests:
ISerializableChatRequestData[]`, `responderUsername`, `customTitle?`, `hasPendingEdits?`,
`inputState?`, `repoData?`, `pendingRequests?`, `workingDirectory?` (URI string). Written in that
order by `storageSchema`
([`chatSessionOperationLog.ts:203-216`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/workbench/contrib/chat/common/model/chatSessionOperationLog.ts#L203-L216)).

`ISerializableChatRequestData`
[`:2037-2094`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/workbench/contrib/chat/common/model/chatModel.ts#L2037-L2094),
one user turn plus response: `requestId`, `timestamp?` (ms, request start), `message: string |
{text, parts[]}`
([`requestParser/chatParserTypes.ts:20-23`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/workbench/contrib/chat/common/requestParser/chatParserTypes.ts#L20-L23)),
`variableData: {variables: [{id, name, fullName?, value, range?}]}`
([`attachments/chatVariableEntries.ts:63-75`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/workbench/contrib/chat/common/attachments/chatVariableEntries.ts#L63-L75)),
`agent?: {id, name, fullName?, extensionId, ...}`
([`participants/chatAgents.ts:63-71`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/workbench/contrib/chat/common/participants/chatAgents.ts#L63-L71)),
`modelId?` (user-selected model id,
[`chatServiceImpl.ts:1291`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/workbench/contrib/chat/common/chatService/chatServiceImpl.ts#L1291)),
`modeInfo?: {kind, telemetryModeId: 'ask'|'agent'|'edit'|'custom'|...}`
([`chatModel.ts:394-401`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/workbench/contrib/chat/common/model/chatModel.ts#L394-L401)),
`response: SerializedChatResponsePart[]`, `responseId?`, `responseTimestamp?` (ms), `result?:
{errorDetails?, timings?, metadata?}`, `modelState?`, `vote?`, `followups?`, `usedContext?`,
`contentReferences?`, `codeCitations?`, `promptTokens?`, `completionTokens?`, `latestModelCall?`,
`elapsedMs?`, `editedFileEvents?: [{uri, eventKind}]`, `isSystemInitiated?`, `requestSource?`,
`terminalExecutionId?`, `confirmation?`, deprecated `isHidden?`, `isCanceled?`.

Response parts
([`:2064`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/workbench/contrib/chat/common/model/chatModel.ts#L2064)):
a bare `IMarkdownString` `{value, isTrusted?, ...}` for assistant text
([`chatSessionOperationLog.ts:45`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/workbench/contrib/chat/common/model/chatSessionOperationLog.ts#L45));
objects keyed by `kind`: `"thinking"` `{value?: string|string[], id?, reasoningDurationMs?}`
([`chatService.ts:635-644`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/workbench/contrib/chat/common/chatService/chatService.ts#L635-L644));
`"toolInvocationSerialized"` `{toolCallId, toolId, invocationMessage, pastTenseMessage?, isComplete,
isConfirmed, resultDetails?` (URI[] or `{input: string, output: [...], isError?}`,
[`tools/languageModelToolsService.ts:283-290`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/workbench/contrib/chat/common/tools/languageModelToolsService.ts#L283-L290)),
`resultError?`, `toolSpecificData?` (`{kind:'terminal', commandLine:{original, userEdited?}, cwd?}`
[`chatService.ts:686-700`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/workbench/contrib/chat/common/chatService/chatService.ts#L686-L700);
`{kind:'input', rawInput}`
[`:847-850`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/workbench/contrib/chat/common/chatService/chatService.ts#L847-L850)),
`subAgentInvocationId?}`
([`chatService.ts:1196-1215`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/workbench/contrib/chat/common/chatService/chatService.ts#L1196-L1215));
plus `textEditGroup`, `codeblockUri`, `progressMessage`, `confirmation`, `undoStop`,
`inlineReference` and others
([`chatSessionOperationLog.ts:62-95`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/workbench/contrib/chat/common/model/chatSessionOperationLog.ts#L62-L95)).
Tool call and result are the same object: the call is `toolCallId`, `toolId` and `toolSpecificData`,
the result is `resultDetails`, `resultError` and `isComplete`. No git branch field; `repoData` is
unspecified here.

#### Tool confirmation and permission fields

Permission state on a tool invocation
([`chatService.ts:1204-1210`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/workbench/contrib/chat/common/chatService/chatService.ts#L1204-L1210)):
`isConfirmed` is `ConfirmedReason | boolean | undefined`, the boolean being the pre-1.104 form.
`ConfirmedReason`
([`:880-889`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/workbench/contrib/chat/common/chatService/chatService.ts#L880-L889))
is an object whose `type` is a `ToolConfirmKind`, a `const enum`
([`:870-877`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/workbench/contrib/chat/common/chatService/chatService.ts#L870-L877))
with no explicit values, so TypeScript inlines and serialises the declaration index: `Denied` 0,
`ConfirmationNotNeeded` 1, `Setting` 2, `LmServicePerTool` 3, `UserAction` 4, `Skipped` 5. Beside
`type`: `source` (`user`, `hook` or `riskAssessment`, the actor of a denial or skip, optional) for
`Denied` and `Skipped`; `reason` (string or markdown) for `ConfirmationNotNeeded`; `id` (the
setting) for `Setting`; `scope` (`session`, `workspace` or `profile`) for `LmServicePerTool`;
`selectedButton` and `selectedButtonKind` for `UserAction`. The invocation's own `source` field is a
different thing: the `ToolDataSource` the tool came from, undefined before 1.104, an object with
`type` (`extension`, `mcp`, `user`, `internal`, `external`) and `label`
([`tools/languageModelToolsService.ts:109-134`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/workbench/contrib/chat/common/tools/languageModelToolsService.ts#L109-L134)).

The request's `modeInfo` also carries `permissionLevel`
([`chatModel.ts:397-405`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/src/vs/workbench/contrib/chat/common/model/chatModel.ts#L397-L405)),
the `ChatPermissionLevel` the request ran under: `default` (the auto-approve settings apply),
`assisted` (approval decisions are delegated to a model), `autoApprove` (every tool call approved,
errors retried) and `autopilot` (as `autoApprove`, plus an internal stop hook that continues until
the task is done)
([`constants.ts:204-213`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/src/vs/workbench/contrib/chat/common/constants.ts#L204-L213)).
The field is in the type at [`d7622a5`](https://github.com/microsoft/vscode/commit/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e) as well, so older `.jsonl` files can carry it. Beside
`modelId` each request has `modelConfiguration`, a dictionary of the selected model's options such
as context size or thinking effort
([`chatModel.ts:2157-2158`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/src/vs/workbench/contrib/chat/common/model/chatModel.ts#L2157-L2158)).
The session's `inputState`, the unsent draft, holds `selectedModel` `{identifier, metadata,
modelConfiguration?}` and `permissionLevel`
([`chatModel.ts:2422-2442`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/src/vs/workbench/contrib/chat/common/model/chatModel.ts#L2422-L2442),
[`:2293`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/src/vs/workbench/contrib/chat/common/model/chatModel.ts#L2293)).
Since [`d7622a5`](https://github.com/microsoft/vscode/commit/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e) the session object gained `sessionCostInfo` `{credits, status?}`
([`:2299`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/src/vs/workbench/contrib/chat/common/model/chatModel.ts#L2299),
[`:2027-2030`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/src/vs/workbench/contrib/chat/common/model/chatModel.ts#L2027-L2030)).
`ToolConfirmKind` keeps the same six members in the same order at
[`4d22f4f`](https://github.com/microsoft/vscode/commit/4d22f4f922eb012217ac65672ccce02f59d63706)
([`chatService.ts:881-888`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/src/vs/workbench/contrib/chat/common/chatService/chatService.ts#L881-L888)),
and `ConfirmedReason` the same fields
([`:890-900`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/src/vs/workbench/contrib/chat/common/chatService/chatService.ts#L890-L900)).

A `confirmation` response part is `{kind: 'confirmation', title, message, data, buttons?, isUsed?}`
([`chatService.ts:514-522`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/workbench/contrib/chat/common/chatService/chatService.ts#L514-L522)),
a question a chat participant puts to the user. Clicking a button sends a new request whose prompt
is `<label>: "<title>"` and whose `confirmation` is the button's label, and sets `isUsed`
([`browser/widget/chatContentParts/chatConfirmationContentPart.ts:42-60`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/workbench/contrib/chat/browser/widget/chatContentParts/chatConfirmationContentPart.ts#L42-L60));
the request field is a string
([`chatModel.ts:2084`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/workbench/contrib/chat/common/model/chatModel.ts#L2084)).

#### Cloud agent hand-off

A local chat delegated to the cloud agent ("Continue In" the cloud) leaves one `pullRequest`
response part in the local session, `{kind: 'pullRequest', uri?, command, title, description,
author, linkTag}`
([`chatService.ts:1243-1254`](https://github.com/microsoft/vscode/blob/661e013024de5052af476650250dabbebb4c1165/src/vs/workbench/contrib/chat/common/chatService/chatService.ts#L1243-L1254),
a kind the operation log persists,
[`chatSessionOperationLog.ts:86`](https://github.com/microsoft/vscode/blob/661e013024de5052af476650250dabbebb4c1165/src/vs/workbench/contrib/chat/common/model/chatSessionOperationLog.ts#L86)).
The extension fills it with the task's web URL as `uri` and as the argument of a `vscode.open`
command, and the task title as `title` and `linkTag`, with empty `description` and `author`
([`copilotCloudSessionsProvider.ts:1948-1960`](https://github.com/microsoft/vscode/blob/661e013024de5052af476650250dabbebb4c1165/extensions/copilot/src/extension/chatSessions/vscode-node/copilotCloudSessionsProvider.ts#L1948-L1960)).
Before the task is created the chat history is summarised and sent with it
([`:1873-1876`](https://github.com/microsoft/vscode/blob/661e013024de5052af476650250dabbebb4c1165/extensions/copilot/src/extension/chatSessions/vscode-node/copilotCloudSessionsProvider.ts#L1873-L1876)).

#### JSONL framing

JSONL framing
([`common/model/objectMutationLog.ts:196-215`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/workbench/contrib/chat/common/model/objectMutationLog.ts#L196-L215),
read
[`:340-385`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/workbench/contrib/chat/common/model/objectMutationLog.ts#L340-L385)):
each line is `{"kind":0,"v":<full session>}` (Initial), `{"kind":1,"k":[path...],"v":...}` (Set),
`{"kind":2,"k":[...],"v":[items], "i"?:index}` (Push; `i` truncates from that index) or
`{"kind":3,"k":[...]}` (Delete). Replay in order to rebuild the object; `k` is a path of keys and
array indexes, for example `["requests",3,"response"]`.

### Joins and timestamps

A session joins its project through the sibling `workspace.json` or its own `workingDirectory`
([Where the project path is recorded](#where-the-project-path-is-recorded)). Tool call and result
share one object; `toolCallId` is the join key to extension telemetry. `agent.id` and
`responderUsername` identify the participant (Copilot).

Times are in ms: `creationDate`, the request `timestamp` and `responseTimestamp`
([Record schema](#record-schema)).

### Other stores

`state.vscdb` (`ItemTable(key, value)`,
[`base/parts/storage/node/storage.ts:73`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/base/parts/storage/node/storage.ts#L73))
in `workspaceStorage/<hash>/` and `globalStorage/`
([`storageMain.ts:285`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/platform/storage/electron-main/storageMain.ts#L285),[`353`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/platform/storage/electron-main/storageMain.ts#L353),[`415`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/platform/storage/electron-main/storageMain.ts#L415)).
Keys: `interactive.sessions` is the legacy JSON blob of all sessions from before the file store
([`chatServiceImpl.ts:67`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/workbench/contrib/chat/common/chatService/chatServiceImpl.ts#L67),[`341`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/workbench/contrib/chat/common/chatService/chatServiceImpl.ts#L341),
migrated by
[`chatSessionStore.ts:611-633`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/workbench/contrib/chat/common/model/chatSessionStore.ts#L611-L633));
`chat.ChatSessionStore.index` is `{version:1, entries:{<sessionId>:{sessionId, title,
lastMessageDate (ms), timing:{created, lastRequestStarted, lastRequestEnded}, workingDirectory?,
isEmpty?, isExternal?, ...}}}`
([`:39`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/workbench/contrib/chat/common/model/chatSessionStore.ts#L39),[`780-836`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/workbench/contrib/chat/common/model/chatSessionStore.ts#L780-L836));
`ChatSessionStore.transferIndex`
([`:40`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/workbench/contrib/chat/common/model/chatSessionStore.ts#L40)).

An external session, one whose resource is not a local chat URI (a cloud agent task or another
provider's session), gets no file under `chatSessions/`: the store writes only its index entry,
keyed by the resource URI string rather than a session id
([`chatSessionStore.ts:405-418`](https://github.com/microsoft/vscode/blob/661e013024de5052af476650250dabbebb4c1165/src/vs/workbench/contrib/chat/common/model/chatSessionStore.ts#L405-L418)),
with `isExternal: true`
([`:802-805`](https://github.com/microsoft/vscode/blob/661e013024de5052af476650250dabbebb4c1165/src/vs/workbench/contrib/chat/common/model/chatSessionStore.ts#L802-L805),
[`:875-877`](https://github.com/microsoft/vscode/blob/661e013024de5052af476650250dabbebb4c1165/src/vs/workbench/contrib/chat/common/model/chatSessionStore.ts#L875-L877))
and `inputState`, the unsent draft
([`:813-818`](https://github.com/microsoft/vscode/blob/661e013024de5052af476650250dabbebb4c1165/src/vs/workbench/contrib/chat/common/model/chatSessionStore.ts#L813-L818)).
External entries are exempt from the 400-session trim
([`:441`](https://github.com/microsoft/vscode/blob/661e013024de5052af476650250dabbebb4c1165/src/vs/workbench/contrib/chat/common/model/chatSessionStore.ts#L441)).
A cloud agent entry's key is `copilot-cloud-agent:/task/<taskId>`
([`copilotCloudSessionsProvider.ts:495`](https://github.com/microsoft/vscode/blob/661e013024de5052af476650250dabbebb4c1165/extensions/copilot/src/extension/chatSessions/vscode-node/copilotCloudSessionsProvider.ts#L495),
[`copilotCodingAgentUtils.ts:132-134`](https://github.com/microsoft/vscode/blob/661e013024de5052af476650250dabbebb4c1165/extensions/copilot/src/extension/chatSessions/vscode/copilotCodingAgentUtils.ts#L132-L134)).

#### Sync state

All written by the Copilot Chat extension under its global storage directory
`User/globalStorage/github.copilot-chat/` (`globalStorageUri`, lower-cased extension id) or its
global state:

- `cloudSessions.json`, a JSON object `{<local session id>: {cloudSessionId, cloudTaskId}}`, both
  strings
  ([`cloudSessionIdStore.ts:10-22`](https://github.com/microsoft/vscode/blob/661e013024de5052af476650250dabbebb4c1165/extensions/copilot/src/extension/chronicle/node/cloudSessionIdStore.ts#L10-L22),
  [`cloudSessionTypes.ts:28-31`](https://github.com/microsoft/vscode/blob/661e013024de5052af476650250dabbebb4c1165/extensions/copilot/src/extension/chronicle/common/cloudSessionTypes.ts#L28-L31),
  path from
  [`remoteSessionExporter.ts:277`](https://github.com/microsoft/vscode/blob/661e013024de5052af476650250dabbebb4c1165/extensions/copilot/src/extension/chronicle/vscode-node/remoteSessionExporter.ts#L277)).
  An entry is added when the cloud session is created, at the first agent turn of a session with
  sync on
  ([`remoteSessionExporter.ts:1066-1071`](https://github.com/microsoft/vscode/blob/661e013024de5052af476650250dabbebb4c1165/extensions/copilot/src/extension/chronicle/vscode-node/remoteSessionExporter.ts#L1066-L1071)),
  and the whole object is rewritten on each change
  ([`cloudSessionIdStore.ts:145-156`](https://github.com/microsoft/vscode/blob/661e013024de5052af476650250dabbebb4c1165/extensions/copilot/src/extension/chronicle/node/cloudSessionIdStore.ts#L145-L156)).
  A reconcile with the server, run before a delete or a reindex, adds an entry for every session the
  account's listing returns, so the file can also name sessions uploaded from another machine
  ([`:96-127`](https://github.com/microsoft/vscode/blob/661e013024de5052af476650250dabbebb4c1165/extensions/copilot/src/extension/chronicle/node/cloudSessionIdStore.ts#L96-L127),
  [`remoteSessionExporter.ts:244-258`](https://github.com/microsoft/vscode/blob/661e013024de5052af476650250dabbebb4c1165/extensions/copilot/src/extension/chronicle/vscode-node/remoteSessionExporter.ts#L244-L258)).
  A session never uploaded has no entry; the file is absent until the first upload. The key is the
  chat session id, the same id as `chatSessions/<sessionId>.jsonl` for a local chat: the sidebar's
  delete passes `LocalChatSessionUri.parseLocalSessionId` values to the delete command, which looks
  them up in this file
  ([`agentSessionsActions.ts:844-875`](https://github.com/microsoft/vscode/blob/661e013024de5052af476650250dabbebb4c1165/src/vs/workbench/contrib/chat/browser/agentSessions/agentSessionsActions.ts#L844-L875)).
- `session-store.db`, the chronicle index
  ([Copilot Chat's chronicle index](#copilot-chats-chronicle-index)). Its `sessions.id` is the key
  of `cloudSessions.json`; the extension counts synced sessions by that join
  ([`remoteSessionExporter.ts:172-191`](https://github.com/microsoft/vscode/blob/661e013024de5052af476650250dabbebb4c1165/extensions/copilot/src/extension/chronicle/vscode-node/remoteSessionExporter.ts#L172-L191)).
  It holds no sync flag of its own.
- `github.copilot.cloudAgent.ownedTasks` in the extension's global state, which VS Code keeps as one
  JSON value in the `ItemTable` row keyed `GitHub.copilot-chat` of `User/globalStorage/state.vscdb`
  ([`extHostMemento.ts:117-118`](https://github.com/microsoft/vscode/blob/661e013024de5052af476650250dabbebb4c1165/src/vs/workbench/api/common/extHostMemento.ts#L117-L118),
  [`extensionStorage.ts:175`](https://github.com/microsoft/vscode/blob/661e013024de5052af476650250dabbebb4c1165/src/vs/platform/extensionManagement/common/extensionStorage.ts#L175)):
  `{<taskId>: {recordedAt: <ms>, application?: <string>}}` (or a bare number, the older form), the
  newest 1,000 cloud agent tasks started or messaged from VS Code
  ([`cloudTaskOwnership.ts:8-14`](https://github.com/microsoft/vscode/blob/661e013024de5052af476650250dabbebb4c1165/extensions/copilot/src/extension/chatSessions/vscode-node/cloudTaskOwnership.ts#L8-L14),
  [`:36-63`](https://github.com/microsoft/vscode/blob/661e013024de5052af476650250dabbebb4c1165/extensions/copilot/src/extension/chatSessions/vscode-node/cloudTaskOwnership.ts#L36-L63)).
  `application` is `vscode` for a task created here, else the task's `event_type` or
  `github/autopilot`
  ([`copilotCloudSessionsProvider.ts:2766-2772`](https://github.com/microsoft/vscode/blob/661e013024de5052af476650250dabbebb4c1165/extensions/copilot/src/extension/chatSessions/vscode-node/copilotCloudSessionsProvider.ts#L2766-L2772)).
  Each task is a session held on GitHub's servers that this install started or messaged, so it marks
  a synced session in the sense of the parser plan ([Parser plan](#parser-plan)), and one whose
  origin is remote: it began on the vendor's side and is only listed here.

#### Standing permission grants

"Always allow" choices are kept in `ItemTable` rows, not in the session. `ToolConfirmStore` writes
five keys, each a JSON object keyed by tool id (or MCP server id) whose value is `true`, a label
string, or `{label, arguments}`; older builds wrote a JSON array of ids
([`languageModelToolsConfirmationService.ts:139-190`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/src/vs/workbench/contrib/chat/browser/tools/languageModelToolsConfirmationService.ts#L139-L190),
[`:240-244`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/src/vs/workbench/contrib/chat/browser/tools/languageModelToolsConfirmationService.ts#L240-L244)):
`chat/autoconfirm` (run without approval), `chat/autoconfirm-post` (continue without reviewing the
result), `chat/servers/autoconfirm` and `chat/servers/autoconfirm-post` (the same for every tool of
an MCP server) and `chat/autoconfirm-combination`. Each key exists in workspace scope, the
workspace's `workspaceStorage/<hash>/state.vscdb`, and in profile scope, `globalStorage/state.vscdb`
for the default profile
([`:49-50`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/src/vs/workbench/contrib/chat/browser/tools/languageModelToolsConfirmationService.ts#L49-L50)),
`User/profiles/<id>/globalStorage/state.vscdb` for another profile and
`User/profiles/builtin/agents/globalStorage/state.vscdb` for the Agents window's
([`platform/storage/electron-main/storageMain.ts:285-289`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/src/vs/platform/storage/electron-main/storageMain.ts#L285-L289),
[`platform/userDataProfile/common/userDataProfile.ts:196`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/src/vs/platform/userDataProfile/common/userDataProfile.ts#L196),
[`:402`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/src/vs/platform/userDataProfile/common/userDataProfile.ts#L402),
[`:24`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/src/vs/platform/userDataProfile/common/userDataProfile.ts#L24),
[`:232`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/src/vs/platform/userDataProfile/common/userDataProfile.ts#L232));
a session-scope grant is held in memory only. Each value is written whole when the window saves
its state: an entry with arguments as `{label, arguments}`, else its label or `true`
([`languageModelToolsConfirmationService.ts:177-189`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/src/vs/workbench/contrib/chat/browser/tools/languageModelToolsConfirmationService.ts#L177-L189)). These are the grants whose per-call record is
`LmServicePerTool` with `scope` `workspace` or `profile`. Two more keys:
`chat.tools.global.autoApprove.optIn` (application scope, `true` once the user accepted the warning
for global auto-approve,
[`languageModelToolsService.ts:73`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/src/vs/workbench/contrib/chat/browser/tools/languageModelToolsService.ts#L73),
[`:1526`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/src/vs/workbench/contrib/chat/browser/tools/languageModelToolsService.ts#L1526))
and `chat.externalPath.workspaceAllowlist` (workspace scope, a JSON array of the URI strings of
folders outside the workspace the user allowed,
[`:85-89`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/src/vs/workbench/contrib/chat/common/tools/builtinTools/chatExternalPathConfirmation.ts#L85-L89),
[`chatExternalPathConfirmation.ts:22-30`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/src/vs/workbench/contrib/chat/common/tools/builtinTools/chatExternalPathConfirmation.ts#L22-L30),
[`:58`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/src/vs/workbench/contrib/chat/common/tools/builtinTools/chatExternalPathConfirmation.ts#L58)).
The settings behind the same decisions live in `User/settings.json` (and a profile's):
`chat.tools.global.autoApprove`, `chat.tools.edits.autoApprove`, `chat.tools.urls.autoApprove`,
`chat.tools.eligibleForAutoApproval`
([`constants.ts:72-75`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/src/vs/workbench/contrib/chat/common/constants.ts#L72-L75)),
`chat.permissions.default`
([`:135`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/src/vs/workbench/contrib/chat/common/constants.ts#L135)),
`chat.tools.terminal.enableAutoApprove` and `chat.tools.terminal.autoApprove`
([`terminalChatAgentToolsConfiguration.ts:46-47`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/src/vs/workbench/contrib/terminalContrib/chatAgentTools/common/terminalChatAgentToolsConfiguration.ts#L46-L47)),
and the default model `chat.defaultModel`
([`constants.ts:138`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/src/vs/workbench/contrib/chat/common/constants.ts#L138)).

#### GitHub account records

VS Code records which extension used which account of an authentication provider in application
scope, the default profile's `User/globalStorage/state.vscdb`, unencrypted. For the GitHub provider
(`github`, or `github-enterprise`,
[`extensions/github-authentication/src/github.ts:62-65`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/extensions/github-authentication/src/github.ts#L62-L65))
the account label is the GitHub login
([`githubServer.ts:323`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/extensions/github-authentication/src/githubServer.ts#L323)),
with ` - <authority><path>` appended for GitHub Enterprise
([`github.ts:227`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/extensions/github-authentication/src/github.ts#L227)).
The keys:

- `<provider>-<label>-usages`, a JSON array of `{extensionId, extensionName, scopes?, lastUsed}`
  (ms), one entry per extension that used the account, refreshed on each use
  ([`services/authentication/browser/authenticationUsageService.ts:15-20`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/src/vs/workbench/services/authentication/browser/authenticationUsageService.ts#L15-L20),
  [`:122-144`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/src/vs/workbench/services/authentication/browser/authenticationUsageService.ts#L122-L144)).
- `<provider>-<label>-mcpserver-usages`, the same for MCP servers, `{mcpServerId, mcpServerName,
  scopes?, lastUsed}`
  ([`authenticationMcpUsageService.ts:15-20`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/src/vs/workbench/services/authentication/browser/authenticationMcpUsageService.ts#L15-L20),
  [`:120-142`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/src/vs/workbench/services/authentication/browser/authenticationMcpUsageService.ts#L120-L142)).
- `<provider>-<label>`, the extensions the user allowed to use the account
  ([`authenticationAccessService.ts:134`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/src/vs/workbench/services/authentication/browser/authenticationAccessService.ts#L134)),
  and `<extension id>-<provider>`, the label of the account an extension prefers, in workspace and
  application scope
  ([`authenticationExtensionsService.ts:245-246`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/src/vs/workbench/services/authentication/browser/authenticationExtensionsService.ts#L245-L246),
  [`:276-278`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/src/vs/workbench/services/authentication/browser/authenticationExtensionsService.ts#L276-L278)).

Copilot Chat's global state (the `GitHub.copilot-chat` row) holds `userSelectedRepositories`, a
JSON array of `{name, timestamp}` (ms), each repository the user picked for a cloud agent task,
dropped a week after it was last picked
([`copilotCloudSessionsProvider.ts:403-404`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/extensions/copilot/src/extension/chatSessions/vscode-node/copilotCloudSessionsProvider.ts#L403-L404),
[`:416-419`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/extensions/copilot/src/extension/chatSessions/vscode-node/copilotCloudSessionsProvider.ts#L416-L419),
[`:2265-2290`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/extensions/copilot/src/extension/chatSessions/vscode-node/copilotCloudSessionsProvider.ts#L2265-L2290));
`name` is the `<owner>/<repo>` the repository picker returns
([`:765-782`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/extensions/copilot/src/extension/chatSessions/vscode-node/copilotCloudSessionsProvider.ts#L765-L782)).
The picker's search asks GitHub on each query, and no other repository list was found in the
extension's stored state (`globalState.update` and `workspaceState.update` grepped in
`extensions/copilot/src`).

#### Chat editing sessions

VS Code core keeps them in
`User/workspaceStorage/<hash>/chatEditingSessions/<key>/state.json`, with the file bodies in
`contents/<hash>`
([`chatEditingSessionStorage.ts:22-23`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/src/vs/workbench/contrib/chat/browser/chatEditing/chatEditingSessionStorage.ts#L22-L23),
[`:43-47`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/src/vs/workbench/contrib/chat/browser/chatEditing/chatEditingSessionStorage.ts#L43-L47)).
`<key>` is the chat session id for a local chat, else the SHA-1 of the session resource
([`chatEditingOperations.ts:137-146`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/src/vs/workbench/contrib/chat/browser/chatEditing/chatEditingOperations.ts#L137-L146)).
`state.json` is `{version: 2, recentSnapshot, timeline, initialFileContents}`; `recentSnapshot` is
`{stopId, entries}` (version 1 wrote `{requestId, workingSet, entries}`) and a snapshot entry is
`{resource, languageId, originalHash, currentHash, state, snapshotUri, telemetryInfo: {requestId,
agentId?, command?, modelId?, modeId?, feature?}, isDeleted?}`
([`:242-288`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/src/vs/workbench/contrib/chat/browser/chatEditing/chatEditingSessionStorage.ts#L242-L288)),
`resource` the file's URI string; the writer fills only `requestId`, `agentId`, `command`,
`modelId` and `modeId` of `telemetryInfo`
([`:176-187`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/src/vs/workbench/contrib/chat/browser/chatEditing/chatEditingSessionStorage.ts#L176-L187)).
`state` is the `ModifiedFileEntryState` const enum, serialised as its index: `Modified` 0,
`Accepted` 1, `Rejected` 2
([`chatEditingService.ts:353-357`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/src/vs/workbench/contrib/chat/common/editing/chatEditingService.ts#L353-L357)).
So it says which file each request edited, with which model, whether the user kept or undid the
edit, and the file before and after. It joins the session on session id and `requestId`. It survives
the chat's deletion: only `stop(true)` removes the folder
([`chatEditingSession.ts:492-497`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/src/vs/workbench/contrib/chat/browser/chatEditing/chatEditingSession.ts#L492-L497),
[`chatEditingSessionStorage.ts:205-213`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/src/vs/workbench/contrib/chat/browser/chatEditing/chatEditingSessionStorage.ts#L205-L213)),
and no caller passes `true`: the only call is `stop()`
([`chatEditingServiceImpl.ts:93`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/src/vs/workbench/contrib/chat/browser/chatEditing/chatEditingServiceImpl.ts#L93);
`stop(true)` and `.clearState(` grepped in `src/vs/workbench/contrib/chat`).

#### Copilot Chat's chronicle index

`globalStorage/github.copilot-chat/session-store.db` (with `-wal`), SQLite, is a copy of the
conversation, joined to the sync markers as [Sync state](#sync-state) says:
`sessions(id, cwd, repository, host_type, branch, summary, agent_name, agent_description,
created_at, updated_at)`, `turns(session_id, turn_index, user_message, assistant_response,
timestamp)`, `checkpoints`
(compaction summaries: `title`, `overview`, `history`, `work_done`, `technical_details`,
`important_files`, `next_steps`), `session_files(file_path, tool_name, turn_index, first_seen_at)`,
`session_refs` and an FTS5 table `search_index`
([`sessionStore.ts:194-271`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/extensions/copilot/src/platform/chronicle/node/sessionStore.ts#L194-L271)).
The texts are cut to 1,000 characters for the user message and 5,000 for the response
([`sessionStoreTracking.ts:17-23`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/extensions/copilot/src/extension/chronicle/common/sessionStoreTracking.ts#L17-L23)),
so a match against the session on text needs a prefix rule. Written by default
(`github.copilot.chat.localIndex.enabled`, default `true`,
[`package.json:3227-3233`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/extensions/copilot/package.json#L3227-L3233));
without a global storage directory it falls back to `<OS temp>/copilot-session-store.db`, outside
the home
([`services.ts:280-283`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/extensions/copilot/src/extension/extension/vscode-node/services.ts#L280-L283)).
The tables, the default and the fallback are the same at
[`661e013`](https://github.com/microsoft/vscode/commit/661e013024de5052af476650250dabbebb4c1165)
([`sessionStore.ts:194-250`](https://github.com/microsoft/vscode/blob/661e013024de5052af476650250dabbebb4c1165/extensions/copilot/src/platform/chronicle/node/sessionStore.ts#L194-L250),
[`package.json:3227-3233`](https://github.com/microsoft/vscode/blob/661e013024de5052af476650250dabbebb4c1165/extensions/copilot/package.json#L3227-L3233),
[`services.ts:280-283`](https://github.com/microsoft/vscode/blob/661e013024de5052af476650250dabbebb4c1165/extensions/copilot/src/extension/extension/vscode-node/services.ts#L280-L283)).
Its rows are removed only by the sessions view's delete, which runs `_deleteSessionsFromCloud`
([`remoteSessionExporter.ts:656-669`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/extensions/copilot/src/extension/chronicle/vscode-node/remoteSessionExporter.ts#L656-L669)),
so a chat deleted another way keeps its turns here. `sessions.id` is the chat session id.

#### Hook transcripts

Copilot Chat writes them to
`workspaceStorage/<hash>/GitHub.copilot-chat/transcripts/<sessionId>.jsonl`
([`sessionTranscriptService.ts:58-67`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/extensions/copilot/src/extension/chat/vscode-node/sessionTranscriptService.ts#L58-L67),
[`:88`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/extensions/copilot/src/extension/chat/vscode-node/sessionTranscriptService.ts#L88)):
one JSON object per line, `{type, data, id, timestamp (ISO 8601), parentId}`
([`platform/chat/common/sessionTranscriptService.ts:16-27`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/extensions/copilot/src/platform/chat/common/sessionTranscriptService.ts#L16-L27)),
with types `session.start` (`sessionId`, `version` 1, `producer` `copilot-agent`, `copilotVersion`,
`vscodeVersion`, `startTime`, `context.cwd`), `user.message`, `assistant.turn_start`,
`assistant.message`, `tool.execution_start`, `tool.execution_complete` and `assistant.turn_end`
([`sessionTranscriptService.ts:118-200`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/extensions/copilot/src/extension/chat/vscode-node/sessionTranscriptService.ts#L118-L200)).
Written only when the agent loop has hooks configured
([`toolCallingLoop.ts:1200-1222`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/extensions/copilot/src/extension/intents/node/toolCallingLoop.ts#L1200-L1222)),
the newest 20 per workspace kept
([`sessionTranscriptService.ts:23`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/extensions/copilot/src/extension/chat/vscode-node/sessionTranscriptService.ts#L23),
[`:239-275`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/extensions/copilot/src/extension/chat/vscode-node/sessionTranscriptService.ts#L239-L275)).
Independent of the chat file, so it survives the chat's deletion.

#### Debug logs

Copilot Chat writes them to
`workspaceStorage/<hash>/GitHub.copilot-chat/debug-logs/<sessionId>/main.jsonl`,
`<label>-<childSessionId>.jsonl` for a subagent, and `models.json`
([`chatDebugFileLoggerService.ts:24-30`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/extensions/copilot/src/extension/chat/vscode-node/chatDebugFileLoggerService.ts#L24-L30),
[`:295-296`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/extensions/copilot/src/extension/chat/vscode-node/chatDebugFileLoggerService.ts#L295-L296),
[`:563-564`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/extensions/copilot/src/extension/chat/vscode-node/chatDebugFileLoggerService.ts#L563-L564)):
entries of type `session_start`, `user_message`, `agent_response`, `tool_call` and
`child_session_ref`, up to 100 MB per session and the newest 50 sessions. Off unless the
experiment-controlled setting registered as `chat.chatDebug.fileLogging.enabled` (default `false`)
turns it on
([`configurationService.ts:843`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/extensions/copilot/src/platform/configuration/common/configurationService.ts#L843)).

#### Memory tool files

The memory tool writes to `globalStorage/github.copilot-chat/memory-tool/memories/` (user scope),
`workspaceStorage/<hash>/GitHub.copilot-chat/memory-tool/memories/repo/` and
`.../memory-tool/memories/<sessionId>/`
([`tools.ts:71`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/extensions/copilot/src/extension/tools/vscode-node/tools.ts#L71),
[`:92`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/extensions/copilot/src/extension/tools/vscode-node/tools.ts#L92),
[`:115`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/extensions/copilot/src/extension/tools/vscode-node/tools.ts#L115)):
notes the model wrote with its memory tool, with no time of their own beyond the file's.

#### The session index

Trimming `chat.ChatSessionStore.index` keeps the newest 400 local sessions in the index and
deletes the other entries, not their files
([`chatSessionStore.ts:37`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/src/vs/workbench/contrib/chat/common/model/chatSessionStore.ts#L37),
[`:438-452`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/src/vs/workbench/contrib/chat/common/model/chatSessionStore.ts#L438-L452)),
so `chatSessions/` can hold more files than the index names. An external entry is the only local
record of a cloud or other provider's session title and times; the parser does not read the index.

### Format versions

`version` absent means v1, `2` adds `computedTitle`, `3` adds `customTitle`;
`normalizeSerializableChatData`
([`chatModel.ts:2422-2461`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/workbench/contrib/chat/common/model/chatModel.ts#L2422-L2461))
fills a missing `sessionId` or `creationDate` (defaulting to one year ago). `message` may be a plain
string in old data
([`:2069`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/workbench/contrib/chat/common/model/chatModel.ts#L2069));
`response` may be a string or string[]
([`chatSessionStore.ts:688-699`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/workbench/contrib/chat/common/model/chatSessionStore.ts#L688-L699)).
Storage moved from `state.vscdb` to `.json` to `.jsonl` (1.109 and later); flat `.json` is still
read as a fallback
([`:653-675`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/workbench/contrib/chat/common/model/chatSessionStore.ts#L653-L675)).
`isConfirmed` was boolean before 1.104; `source` was undefined before 1.104
([`chatService.ts:1204-1209`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/workbench/contrib/chat/common/chatService/chatService.ts#L1204-L1209)).

Between [`d7622a5`](https://github.com/microsoft/vscode/commit/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e) and
[`4d22f4f`](https://github.com/microsoft/vscode/commit/4d22f4f922eb012217ac65672ccce02f59d63706)
nothing the parser reads changed: the user data directory layout, the `workspace.json` keys, the
store paths, the mutation-log framing, the request fields, `ToolConfirmKind` and `ConfirmedReason`
are the same, with only line numbers moved, and the session object gained `sessionCostInfo`
([Record schema](#record-schema)). Between [`661e013`](https://github.com/microsoft/vscode/commit/661e013024de5052af476650250dabbebb4c1165) and [`4d22f4f`](https://github.com/microsoft/vscode/commit/4d22f4f922eb012217ac65672ccce02f59d63706), `cloudSessions.json`,
`ownedTasks` and `chat.sessionSync.enabled` did not change.

### Secrets

`variableData.variables[].value` and `usedContext` may hold attached file contents;
`toolSpecificData.rawInput` and `commandLine` may hold command-line secrets. `state.vscdb` stores
extension auth material under other keys (out of scope here; see the Cursor and Windsurf notes in
[`collectors/docs/coverage.md`](../collectors/docs/coverage.md#credentials)). In
`User/settings.json`, an extension's own settings can hold a token, such as an MCP server's
`env` or `headers` values; the keys are in that extension's document
([Secrets in `twinny.md`](twinny.md#secrets), [Secrets in `cody.md`](cody.md#secrets)).

Session sync uses no credential file of its own: each request goes to the `api` endpoint named in
the Copilot token, the short-lived token the extension exchanges for the user's GitHub sign-in
([`cloudSessionApiClient.ts:329-341`](https://github.com/microsoft/vscode/blob/661e013024de5052af476650250dabbebb4c1165/extensions/copilot/src/extension/chronicle/node/cloudSessionApiClient.ts#L329-L341)),
and `cloudSessions.json` holds only ids. Uploaded events pass a filter that replaces known token
values and token patterns
([`secretFilter.ts:117-172`](https://github.com/microsoft/vscode/blob/661e013024de5052af476650250dabbebb4c1165/extensions/copilot/src/extension/chronicle/common/secretFilter.ts#L117-L172)).

### Parser plan

Globs: `**/User/workspaceStorage/*/chatSessions/*.{json,jsonl}`,
`**/User/globalStorage/emptyWindowChatSessions/*.{json,jsonl}`,
`**/User/workspaceStorage/no-workspace/chatSessions/*.json`,
`**/User/globalStorage/transferredChatSessions/*.json`, plus the sibling `workspace.json`; for the
standing grants, settings and account records `**/User/globalStorage/state.vscdb`,
`**/User/profiles/[builtin/]<id>/globalStorage/state.vscdb`,
`**/User/workspaceStorage/*/state.vscdb`, `**/User/settings.json` and
`**/User/profiles/[builtin/]<id>/settings.json`; and
`**/User/workspaceStorage/*/chatEditingSessions/*/state.json`.

Rows: one `user` per request, one `assistant` per contiguous markdown run, one `thinking` per
thinking part, one `tool_use` plus `tool_result` per `toolInvocationSerialized`.

| Column | Source |
| --- | --- |
| timestamp_utc | `timestamp` for the user row, `responseTimestamp` for response parts (ms) |
| session_id | `sessionId` |
| project_path | `workingDirectory`, else `workspace.json` `folder` (file URI to path); never `workspace` |
| git_branch | empty |
| model | `modelId` |
| tool_name, tool_use_id | `toolId`, `toolCallId` |
| text | text, `invocationMessage` or `commandLine.original` |

#### Permission rows

The parser writes these `system` rows
([issue 96](https://github.com/seanthegeek/doubleagent/issues/96) and
[issue 97](https://github.com/seanthegeek/doubleagent/issues/97)):

- `permission mode: <mode>` and `permission mode: <old> -> <new>` from the request
  `modeInfo.telemetryModeId` (`ask`, `agent`, `edit`, `custom`), placed before the request's user
  row and timed by its `timestamp`, as the [model rows](#model-rows) are.
- `permission decision: confirmed=<value>` after a tool invocation's `tool_result` row, with its
  `toolCallId` and `toolId`: `<value>` is `true` or `false` for the boolean form, else the
  `ToolConfirmKind` name and number (`UserAction(4)`), followed by the `ConfirmedReason`'s other
  fields as `key=value` (`reason` last), and `tool_source=<type>` from the invocation's `source`. An
  invocation without `isConfirmed` gets no row.
- `permission decision: confirmation isUsed=<bool> buttons=<list> title=<title>` and the part's
  `message` on the next line, for a `confirmation` response part, at the response time.
- `permission decision: confirmation=<label>` after the user row of a request whose `confirmation`
  is set.

Since the 2026-10-21 review the parser also writes these, source-derived; not observed on any real
install:

- `permission level: <level>` and `permission level: <old> -> <new>` from
  `modeInfo.permissionLevel`, after the request's `permission mode` row and timed the same way.
- From each `state.vscdb` of a profile or a workspace, one row per entry of the five
  `chat/autoconfirm*` keys, `permission setting: <key>.<tool or server id>=<value>` (`<value>`
  `true`, the label, or `{label, arguments}` as compact JSON; an entry of the older array form
  reads `=true`), then `permission setting: chat.tools.global.autoApprove.optIn=<value>` and one
  `permission setting: chat.externalPath.workspaceAllowlist=<path>` per allowed folder, each ending
  `(from User/<path>/state.vscdb, file mtime)`, at the database's modification time, with an empty
  `session_id` and, for a workspace's database, the workspace's folder as `project_path`. A key of
  another JSON type is one `parser: ItemTable value for <key> is not a JSON object or array` row.
- From `User/settings.json` and each profile's `settings.json`, read as JSONC, one row per key
  matching `chat.tools.*autoApprov*` and for `chat.permissions.default`: `permission setting:
  <key>=<value>`, or one `permission setting: <key>.<entry>=<value>` per entry of a non-empty
  object (`chat.tools.terminal.autoApprove` maps command patterns to a decision), each ending
  `(from User/<path>/settings.json, file mtime)`, at the file's modification time.

The keys are under [Standing permission grants](#standing-permission-grants).

#### Model rows

The parser writes `model: <id>` at the first request whose `modelId` is set and
`model: <old> -> <new>` on every later request with a different `modelId`, as `system` rows before
the request's user row, with its `timestamp`
([issue 96](https://github.com/seanthegeek/doubleagent/issues/96) and
[issue 97](https://github.com/seanthegeek/doubleagent/issues/97)). The model is chosen per request,
so a session can change model at any request.

Since the 2026-10-21 review, source-derived; not observed on any real install: the request's
`modelConfiguration` is tracked with its `modelId` and printed after the id as sorted
`key=value` pairs, `model: <id> <key>=<value>` and `model: <old> -> <new>`, so a change of options
alone is a model row too; and `chat.defaultModel` in a profile's `settings.json` is
`model setting: chat.defaultModel=<value> (from User/<path>/settings.json, file mtime)` at the
file's modification time. The session's `inputState.selectedModel` is a draft, not a record of
use, and gets no row.

#### Past-use rows and secondary sources

The parser reads `chatEditingSessions/<key>/state.json` as a secondary source (`secondary()` true,
`has_secondary` set), source-derived; not observed on any real install. Each entry of
`recentSnapshot.entries` is one `system` row, `edit state: <Modified|Accepted|Rejected> <path>
request=<requestId> model=<modelId> (from chatEditingSessions state.json, file mtime)`, with
` deleted=true` before the parenthesis when `isDeleted` is set and `-` for an absent value, at the
file's modification time, with `<key>` as `session_id` (the chat session id for a local chat) and
the sibling `workspace.json`'s folder as `project_path`. The rows record what the chat does not:
whether the user kept or undid each edit. Being `system` rows they never pair with a chat row and
are never marked `recovered`, but they outlive a deleted chat
([What survives a delete](#what-survives-a-delete)). The `timeline` operations and the
`contents/` bodies are not read.

Not read, and planned: `session-store.db` `turns`, `checkpoints` and `session_files` secondary
(truncated texts, own `timestamp`; [issue 161](https://github.com/seanthegeek/doubleagent/issues/161));
`transcripts/<sessionId>.jsonl` secondary (own `timestamp`); `debug-logs/<sessionId>/*.jsonl`
secondary; memory files secondary with an empty timestamp. The stores are described in
[Other stores](#other-stores).

#### Sync, relay and origin rows

Copilot Chat's session sync uploads chat sessions to the user's GitHub account and marks each
uploaded session in `User/globalStorage/github.copilot-chat/cloudSessions.json`
([Other stores](#other-stores)). It runs when `chat.sessionSync.enabled` is on, a core setting
whose default is `false` but which is tagged experimental with `experiment: {mode: 'auto'}`, so an
experiment can turn it on, and which the organisation policy `CopilotSessionSync` can force off
([`chat.shared.contribution.ts:830-851`](https://github.com/microsoft/vscode/blob/661e013024de5052af476650250dabbebb4c1165/src/vs/workbench/contrib/chat/browser/chat.shared.contribution.ts#L830-L851)),
and when `github.copilot.chat.localIndex.enabled` is on, which it is by default
([`remoteSessionExporter.ts:320-366`](https://github.com/microsoft/vscode/blob/661e013024de5052af476650250dabbebb4c1165/extensions/copilot/src/extension/chronicle/vscode-node/remoteSessionExporter.ts#L320-L366)).
A session is uploaded only when the workspace resolves to a GitHub repository not matched by
`chat.sessionSync.excludeRepositories`
([`:905-961`](https://github.com/microsoft/vscode/blob/661e013024de5052af476650250dabbebb4c1165/extensions/copilot/src/extension/chronicle/vscode-node/remoteSessionExporter.ts#L905-L961),
[`sessionIndexingPreference.ts:48-65`](https://github.com/microsoft/vscode/blob/661e013024de5052af476650250dabbebb4c1165/extensions/copilot/src/extension/chronicle/common/sessionIndexingPreference.ts#L48-L65)).
What leaves is a cloud session created at the first agent turn (`POST /agents/sessions` with the
repository's owner and repository ids and the local session id), then the turns as Copilot
CLI-compatible events (`session.start`, `user.message`, `assistant.message`, `tool.execution_start`,
`tool.execution_complete`) posted to `/agents/sessions/<id>/events` about 500 ms after each turn
ends
([`cloudSessionApiClient.ts:152-199`](https://github.com/microsoft/vscode/blob/661e013024de5052af476650250dabbebb4c1165/extensions/copilot/src/extension/chronicle/node/cloudSessionApiClient.ts#L152-L199),
[`remoteSessionExporter.ts:49`](https://github.com/microsoft/vscode/blob/661e013024de5052af476650250dabbebb4c1165/extensions/copilot/src/extension/chronicle/vscode-node/remoteSessionExporter.ts#L49)).
`/chronicle reindex` and the `github.copilot.chronicle.reindex` command also upload past sessions
that have no entry yet
([`sessionReindexer.ts:349-376`](https://github.com/microsoft/vscode/blob/661e013024de5052af476650250dabbebb4c1165/extensions/copilot/src/extension/chronicle/node/sessionReindexer.ts#L349-L376)).
The parser yields one `system` row per `cloudSessions.json` entry, `session synced: remote session
<cloudSessionId> task <cloudTaskId> (from User/globalStorage/github.copilot-chat/cloudSessions.json,
file mtime)`, with the entry's key as `session_id`, timestamped by the file's modification time; an
entry with no local session is still a row, since it names a session deleted here or uploaded from
another machine. The `ownedTasks` list of cloud agent tasks ([Other stores](#other-stores)) is the
other remote record: one `system` row per entry, `session synced: cloud agent task <taskId>
application=<application or -> (from state.vscdb GitHub.copilot-chat
github.copilot.cloudAgent.ownedTasks, recordedAt)` at `recordedAt`, with `session_id`
`copilot-cloud-agent:/task/<taskId>` to match the index entry; the older bare-number form is read as
`recordedAt` with no `application`. A task is a `session synced:` row although nothing was uploaded
from the host: it began on GitHub's servers, and GitHub holds its copy as it holds an uploaded
session's, so for a responder it is a session the vendor can still produce. Its session reads
`synced` `yes`. Only the origin differs, and it is a second row at the same time and session:
`session origin: remote GitHub cloud agent (from state.vscdb GitHub.copilot-chat
github.copilot.cloudAgent.ownedTasks, recordedAt)`. The parser sets `records_origin`, so a task's
session reads `origin` `remote` and every other VS Code session `local`. Both rows are fixture only:
no real install has shown either file. A delete removes the marker
([What survives a delete](#what-survives-a-delete)).
VS Code Settings Sync
does not carry chat sessions. Its resources are settings, keybindings, snippets, prompt files,
tasks, MCP configuration, extensions, global state and profiles
([`userDataSync.ts:169-181`](https://github.com/microsoft/vscode/blob/661e013024de5052af476650250dabbebb4c1165/src/vs/platform/userDataSync/common/userDataSync.ts#L169-L181));
global state carries only keys stored with the `USER` target
([`globalStateSync.ts:296-304`](https://github.com/microsoft/vscode/blob/661e013024de5052af476650250dabbebb4c1165/src/vs/platform/userDataSync/common/globalStateSync.ts#L296-L304)),
and the chat session index and every extension's state are stored with the `MACHINE` target
([`chatSessionStore.ts:425`](https://github.com/microsoft/vscode/blob/661e013024de5052af476650250dabbebb4c1165/src/vs/workbench/contrib/chat/common/model/chatSessionStore.ts#L425),
[`extensionStorage.ts:175`](https://github.com/microsoft/vscode/blob/661e013024de5052af476650250dabbebb4c1165/src/vs/platform/extensionManagement/common/extensionStorage.ts#L175)).
Its local state, `User/sync/<resource>/lastSync<resource>.json` (`{ref, syncData: {version,
machineId?, content}}`) and the timestamped `<yyyymmddThhmmss>.json` backups kept 30 days by default
(`sync.localBackupDuration`)
([`userDataSync.ts:187-189`](https://github.com/microsoft/vscode/blob/661e013024de5052af476650250dabbebb4c1165/src/vs/platform/userDataSync/common/userDataSync.ts#L187-L189),
[`:457-466`](https://github.com/microsoft/vscode/blob/661e013024de5052af476650250dabbebb4c1165/src/vs/platform/userDataSync/common/userDataSync.ts#L457-L466),
[`userDataSyncLocalStoreService.ts:99-125`](https://github.com/microsoft/vscode/blob/661e013024de5052af476650250dabbebb4c1165/src/vs/platform/userDataSync/common/userDataSyncLocalStoreService.ts#L99-L125)),
records that settings, prompt files and MCP configuration were synced, not sessions, and gets no
row. Export Chat (`workbench.action.chat.export`) writes the session as JSON to a file the user
picks, `chat.json` by default, and uploads nothing
([`chatImportExport.ts:23`](https://github.com/microsoft/vscode/blob/661e013024de5052af476650250dabbebb4c1165/src/vs/workbench/contrib/chat/browser/actions/chatImportExport.ts#L23),
[`:42-79`](https://github.com/microsoft/vscode/blob/661e013024de5052af476650250dabbebb4c1165/src/vs/workbench/contrib/chat/browser/actions/chatImportExport.ts#L42-L79));
no share command was found (searched `share`, `shareChat` and `ShareSession` in
`src/vs/workbench/contrib/chat` and `extensions/copilot/src`). Copilot CLI sessions run in VS Code's
agent host pass the same setting to the Copilot SDK as `enableRemoteSessions`
([`copilotAgent.ts:3846`](https://github.com/microsoft/vscode/blob/661e013024de5052af476650250dabbebb4c1165/src/vs/platform/agentHost/node/copilot/copilotAgent.ts#L3846))
and keep their state under `~/.copilot/session-state/`, where the `copilot-cli` parser reads the
sync marker ([Parser plan in `copilot-cli.md`](copilot-cli.md#parser-plan)).
No relay: the agent carries no session through a messaging platform.

#### Scratch directory rows

A chat in the Agents window runs in the workspace `User/agent-sessions.code-workspace`
([`environmentService.ts:280-281`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/src/vs/platform/environment/common/environmentService.ts#L280-L281)),
so its project would be that file, under the catalog's `User` entry, which the analyzer classes as
a scratch directory, while the real project is in `workingDirectory`. The parser takes the
session's `workingDirectory` first, then `workspace.json`'s `folder`, and never the `workspace`
value, which names a `.code-workspace` file rather than a directory (a user's multi-root workspace
file as well as the Agents window's); the workspace file is named in the session-start row as
`workspace=<path>`, and a `workspace.json` that cannot be read or is not a JSON object gives a
`parser: workspace.json not read (<reason>)` or `parser: workspace.json is not a JSON object` row.
That the Agents window's chats are stored under that workspace's hash follows from the store's
rule ([Transcript stores](#transcript-stores)) and was not traced further; it is not observed on any
real install, source-derived. An empty-window chat has no project. The worktrees VS Code makes for
Copilot CLI sessions, `<repo parent>/<repo>.worktrees/<branch>` unless the git extension's global
state holds a `worktreeRoot:<repo>` root
([`extensions/git/src/repository.ts:1952-1970`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/extensions/git/src/repository.ts#L1952-L1970),
[`chatSessionWorktreeServiceImpl.ts:113`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/extensions/copilot/src/extension/chatSessions/vscode-node/chatSessionWorktreeServiceImpl.ts#L113)),
are recorded in `~/.copilot/session-state/<id>/vscode.metadata.json`
([`chatSessionMetadataStoreImpl.ts:186`](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/extensions/copilot/src/extension/chatSessions/copilotcli/vscode-node/chatSessionMetadataStoreImpl.ts#L186)),
which belongs to the `copilot-cli` parser.

#### Deletion rows

The agent writes no deletion marker ([What survives a delete](#what-survives-a-delete)), and the
parser writes no deletion row. An `edit state:` row whose `session_id` has no chat file is a
deleted chat (source-derived; not observed on any real install); once the other records of
[Other stores](#other-stores) are read, a session-store or transcript record with no chat file is
one too.

#### Bad-line rows

A `.jsonl` session line that does not decode, or whose mutation cannot be applied, is a `system`
row through `error_rows`, `parser: unparseable line (<reason>): <line>` or
`parser: mutation not applied (<why>): <line>`; a `.json` session that does not decode is
`parser: file is not valid JSON (<reason>)`; `workspace.json` errors are under
[Scratch directory rows](#scratch-directory-rows). A chat editing `state.json` that does not
decode is `parser: file is not valid JSON (<reason>)` and one that is not a JSON object
`parser: state.json is not a JSON object`, whole-file rows through `error_rows` with the folder's
session id; a `settings.json` that is not a JSON object is
`parser: User/<path>/settings.json is not a JSON object (<reason>)`; a `state.vscdb` that cannot be
read, or whose `-wal`, `-shm` or `-journal` sidecar cannot, is
`parser: <file> could not be read (<reason>)`, `<file>` the name of the file that failed, one that
is not a SQLite database `parser: state.vscdb is not a SQLite database`, and a damaged `state.vscdb`
or a value that is not JSON is `parser: state.vscdb unreadable: <message>` or
`parser: ItemTable value for <key> is not valid JSON (<reason>)`. When they are read, the transcript
and debug-log readers go through `iter_jsonl` and `error_rows`, and `session-store.db` errors are
whole-file `parser:` rows.

#### Fixtures

Fixture `chatSessions/9ab.jsonl`:

```jsonl
{"kind":0,"v":{"version":3,"creationDate":1759400000000,"customTitle":null,"initialLocation":"panel","responderUsername":"GitHub Copilot","sessionId":"9ab","requests":[{"requestId":"request_1","timestamp":1759400001000,"message":{"text":"run tests","parts":[]},"agent":{"id":"github.copilot.default","name":"GitHub Copilot","extensionId":{"value":"GitHub.copilot-chat"}},"modelId":"copilot/gpt-4.1","modeInfo":{"kind":"agent","telemetryModeId":"agent","isBuiltin":true},"variableData":{"variables":[]},"response":[{"kind":"thinking","value":"Need pytest","id":"t1"},{"kind":"toolInvocationSerialized","toolCallId":"call_a","toolId":"run_in_terminal","invocationMessage":"Running command","isComplete":true,"isConfirmed":true,"toolSpecificData":{"kind":"terminal","commandLine":{"original":"pytest"}},"resultDetails":{"input":"pytest","output":[{"type":"embed","value":"3 passed"}]}},{"value":"All tests pass."}],"responseId":"response_1","responseTimestamp":1759400002000,"modelState":{"value":"complete"}}],"workingDirectory":"file:///home/u/proj"}}
{"kind":2,"k":["requests"],"v":[{"requestId":"request_2","timestamp":1759400010000,"message":{"text":"thanks","parts":[]},"variableData":{"variables":[]},"response":[{"value":"You're welcome."}]}]}
```

`workspace.json`: `{"folder": "file:///home/u/proj"}`

The fixture's `modelId`, `agent.id` and `responderUsername` values are invented: they were written
while the extension was taken to be closed source, and have not been checked against its source
([Source and evidence](#source-and-evidence)).

Since the 2026-10-21 review, from the shapes in [Other stores](#other-stores) at
[`4d22f4f`](https://github.com/microsoft/vscode/commit/4d22f4f922eb012217ac65672ccce02f59d63706)
(ids, labels, the login and the `modelConfiguration` key invented): a request's `modeInfo` with
`"permissionLevel":"autoApprove"` and `"modelConfiguration":{"reasoningEffort":"high"}`; a
workspace `state.vscdb` with

```json
{"chat/autoconfirm": {"run_in_terminal": true, "mcp_github_create_issue": {"label": "Create issue", "arguments": "{\"repo\":\"x\"}"}},
 "chat/servers/autoconfirm": ["github"],
 "chat.externalPath.workspaceAllowlist": ["file:///srv/shared"]}
```

the default profile's `state.vscdb` with `"chat/autoconfirm-post": {"read_file": "Read File"}`,
`"chat.tools.global.autoApprove.optIn": true`,
`"github-octocat-usages": [{"extensionId": "GitHub.copilot-chat", "extensionName": "GitHub Copilot
Chat", "scopes": ["read:user", "user:email"], "lastUsed": 1791025100000}]`, the matching
`-mcpserver-usages` entry and `"userSelectedRepositories": [{"name": "octocat/hello-world",
"timestamp": 1791025000000}]` in the `GitHub.copilot-chat` value; a profile `settings.json` (JSONC)
with `chat.tools.terminal.autoApprove`, an empty `chat.tools.edits.autoApprove`,
`chat.permissions.default` and `chat.defaultModel`; and a chat editing `state.json`:

```json
{"version": 2, "initialFileContents": [["file:///srv/proj/test_a.py", "aaaaaaa"]], "timeline": null,
 "recentSnapshot": {"stopId": null, "entries": [
  {"resource": "file:///srv/proj/test_a.py", "languageId": "python", "originalHash": "aaaaaaa", "currentHash": "bbbbbbb", "state": 1, "snapshotUri": "chat-editing-snapshot-text-model:/srv/proj/test_a.py", "telemetryInfo": {"requestId": "request_1", "modelId": "copilot/gpt-4.1", "agentId": "github.copilot.editsAgent"}},
  {"resource": "file:///srv/proj/old.py", "languageId": "python", "originalHash": "aaaaaaa", "currentHash": "bbbbbbb", "state": 2, "snapshotUri": "chat-editing-snapshot-text-model:/srv/proj/old.py", "telemetryInfo": {"requestId": "request_1", "modelId": "copilot/gpt-4.1"}, "isDeleted": true}]}}
```

#### Source host rows

Copilot Chat signs in to GitHub through VS Code's GitHub authentication provider; the token is a
secret-storage row ([Credentials](#credentials)), and the account's login and its uses are plain
keys of `User/globalStorage/state.vscdb` ([GitHub account records](#github-account-records)). The
parser writes, source-derived; not observed on any real install:

- `source host: <host> account <login> extension=<extensionId> scopes=<a,b> (from state.vscdb
  <key>, lastUsed)` per entry of each `github-<login>-usages` key, and the same with
  `mcp_server=<mcpServerId>` per entry of `github-<login>-mcpserver-usages`, at `lastUsed`, with
  an empty `session_id`. `<host>` is `github.com`, or for a `github-enterprise-<label>-usages` key
  the part of the label after ` - `. The `<provider>-<label>` and `<extension id>-<provider>` keys
  name the same login and are not read.
- `source host repository: <owner>/<repo> type=cloud agent selection (from state.vscdb
  GitHub.copilot-chat userSelectedRepositories, timestamp)` per entry of `userSelectedRepositories`,
  at its `timestamp`, with an empty `session_id`.

A cloud agent task bound to a repository is listed from GitHub; locally it leaves only the index
entry and the `ownedTasks` id ([Sync, relay and origin rows](#sync-relay-and-origin-rows)).

## Confidence

- High: user data path order, `User/` layout, `state.vscdb` schema, chat session paths,
  `workspace.json` keys, server `data/User`, Positron and VSCodium names, from source; the field
  set, the JSONL framing and bare `{value}` markdown parts, from source; `cloudSessions.json`,
  `ownedTasks` and the sync delete path, from source at
  [`661e013`](https://github.com/microsoft/vscode/commit/661e013024de5052af476650250dabbebb4c1165),
  though no install has shown them ([Observed](#observed)); the standing grant keys, the GitHub
  account records and the chat editing state, from source at
  [`4d22f4f`](https://github.com/microsoft/vscode/commit/4d22f4f922eb012217ac65672ccce02f59d63706),
  likewise unobserved; the sizes in
  [Per-user storage](#per-user-storage) and [Exclusions](#exclusions), as measured by the
  names-and-sizes listing of one workstation.
- Medium: Trae names
  (L4 docs only); `agent-plugins` location (source puts it under the profile, the listing shows
  `~/.vscode/agent-plugins`); Copilot index exclusions (file names from source, sizes not
  observed).
- Not determined: the actual `modelId`, `agent.id` and `responderUsername` values the Copilot
  extension sets, which the fixture invents and the now public source has not been read for; the
  exact release in which `.jsonl` became the default beyond the `1.109` code comment; Copilot's
  own data in `state.vscdb` keys other than `interactive.sessions` and those under
  [Other stores](#other-stores); the option names `modelConfiguration` holds; whether forks lowercase
  extension ids the same way (Kiro's `kiro.kiroagent` suggests yes); Copilot `.github/`
  customization file names; which `secret://` keys each AI extension writes.
