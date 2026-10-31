# pearai

PearAI (catalog agent `pearai`) is a VS Code fork that ships two AI extensions of its own:
`pearai.pearai`, a fork of Continue (chat, autocomplete, memory), and `PearAI.pearai-roo-cline`, a
fork of Roo Code (the "Agent" view). Editor state follows the VS Code layout in
[`vscode.md`](vscode.md); the extension forks follow [`continue.md`](continue.md) and
[`roo-code.md`](roo-code.md) with renamed directories, and both are older snapshots of their
upstreams, so the record shapes differ in ways a parser must handle. The single most important
fact: **`~/.pearai` is both the editor's `dataFolderName` (extensions) and the Continue fork's
global directory (chat sessions, config, API keys).** PearAI writes two independent transcript
stores, one per bundled extension: chat sessions from its Continue fork in `~/.pearai/sessions`,
and agent tasks from its Roo Code fork in the editor's `globalStorage`. The analyzer parses both,
from source and fixtures only.

## Source and evidence

All claims from source, three repositories, all open source; the editor stores no transcripts of
its own:

- Editor: [trypear/pearai-app](https://github.com/trypear/pearai-app), MIT
  ([`LICENSE.txt`](https://github.com/trypear/pearai-app/blob/d930f0233c14668df9f85c6a78a81828f4251194/LICENSE.txt)),
  commit
  [`d930f0233c14668df9f85c6a78a81828f4251194`](https://github.com/trypear/pearai-app/commit/d930f0233c14668df9f85c6a78a81828f4251194)
  (2025-05-15), the last push. Its tree records the two extensions only in
  [`.gitmodules:1-8`](https://github.com/trypear/pearai-app/blob/d930f0233c14668df9f85c6a78a81828f4251194/.gitmodules#L1-L8),
  with no pinned gitlink, so the extension commits below are their current heads, not the ones a
  given PearAI build shipped.
- Continue fork: [trypear/pearai-submodule](https://github.com/trypear/pearai-submodule),
  Apache-2.0, commit
  [`51eceef62a90c29f712b3a9607ea70a9dca657e9`](https://github.com/trypear/pearai-submodule/commit/51eceef62a90c29f712b3a9607ea70a9dca657e9)
  (2026-06-19), extension `pearai.pearai` 2.0.0
  ([`extensions/vscode/package.json:3-6`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/extensions/vscode/package.json#L3-L6)).
- Roo Code fork: [trypear/PearAI-Roo-Code](https://github.com/trypear/PearAI-Roo-Code), Apache-2.0,
  commit
  [`0b6df736c9b2799c38b9ee629668407521a7bdda`](https://github.com/trypear/PearAI-Roo-Code/commit/0b6df736c9b2799c38b9ee629668407521a7bdda)
  (2026-06-19), extension `PearAI.pearai-roo-cline` 3.15.3, a Roo Code 3.15.3 fork
  ([`package.json:2-7`](https://github.com/trypear/PearAI-Roo-Code/blob/0b6df736c9b2799c38b9ee629668407521a7bdda/package.json#L2-L7)).

### Observed

- No real install.

### Source reviews

- 2026-10-21: trypear/pearai-submodule at
  [51eceef](https://github.com/trypear/pearai-submodule/commit/51eceef62a90c29f712b3a9607ea70a9dca657e9),
  trypear/PearAI-Roo-Code at
  [0b6df73](https://github.com/trypear/PearAI-Roo-Code/commit/0b6df736c9b2799c38b9ee629668407521a7bdda)
  and trypear/pearai-app at
  [d930f02](https://github.com/trypear/pearai-app/commit/d930f0233c14668df9f85c6a78a81828f4251194)
  (extensions 2.0.0 and 3.15.3): the output channel log's record format, the standing
  auto-approval keys, the context providers' source-host tokens and the source-host review were
  added, and the Parser plan now describes the permission, task-list, output-log and `dev_data`
  rows the parser writes.
- 2026-10-19: the heads of all three repositories are still the commits cited here,
  [`51eceef`](https://github.com/trypear/pearai-submodule/commit/51eceef62a90c29f712b3a9607ea70a9dca657e9)
  (pearai-submodule),
  [`0b6df73`](https://github.com/trypear/PearAI-Roo-Code/commit/0b6df736c9b2799c38b9ee629668407521a7bdda)
  (PearAI-Roo-Code) and
  [`d930f02`](https://github.com/trypear/pearai-app/commit/d930f0233c14668df9f85c6a78a81828f4251194)
  (pearai-app): the extension forks have taken no commit since 2026-06-19 and the editor none
  since 2025-05-15. It added the output channel log and the other records of chat use of
  [Other stores](#other-stores), Creator mode's project directory, the agent surface's permission
  records and the agent-task delete.

## Collection

### Per-user storage

The editor, its two extension forks, the import from VS Code and the editor's output channel logs
each have their own layout.

#### Scratch directories

Neither extension keeps a working directory of its own. PearAI Creator, the mode that builds a new
project from a description, creates the project directory, by default
`~/Documents/PearAI/<generated name>`, and opens it as the workspace
([`gui/src/pages/creator/index.tsx:106-114`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/gui/src/pages/creator/index.tsx#L106-L114),
[`extensions/vscode/src/integrations/creator/index.ts:130-160`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/extensions/vscode/src/integrations/creator/index.ts#L130-L160)).
The path picker's default is `~/pearai-projects/`
([`gui/src/components/ui/path-select.tsx:19`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/gui/src/components/ui/path-select.tsx#L19)).
That is the user's new project, reached by project discovery through the task's `workspace`, not a
scratch directory; no catalog line names `Documents/PearAI`.

#### Editor

`product.json` sets `nameShort` `PearAI`, `dataFolderName` `.pearai` and `serverDataFolderName`
`.pearai-server`
([`product.json:2-6`](https://github.com/trypear/pearai-app/blob/d930f0233c14668df9f85c6a78a81828f4251194/product.json#L2-L6),
[`15`](https://github.com/trypear/pearai-app/blob/d930f0233c14668df9f85c6a78a81828f4251194/product.json#L15)).
The user data directory is `getUserDataPath(args, product.nameShort)`
([`src/main.ts:60`](https://github.com/trypear/pearai-app/blob/d930f0233c14668df9f85c6a78a81828f4251194/src/main.ts#L60)),
which resolves to `VSCODE_APPDATA`, else `%APPDATA%`, `~/Library/Application Support` or
`$XDG_CONFIG_HOME`/`~/.config`, joined with the product name
([`src/vs/platform/environment/node/userDataPath.js:63-101`](https://github.com/trypear/pearai-app/blob/d930f0233c14668df9f85c6a78a81828f4251194/src/vs/platform/environment/node/userDataPath.js#L63-L101)):

- `~/.config/PearAI`, `~/Library/Application Support/PearAI`,
  `%APPDATA%\PearAI`, each with the standard `User/` (settings,
  `globalStorage/state.vscdb`, `workspaceStorage/`, `History/`) and
  `logs/`.
- `~/.pearai/extensions/` (installed extensions), `~/.pearai/argv.json` and `~/.pearai/policy.json`
  ([`environmentService.ts:104`](https://github.com/trypear/pearai-app/blob/d930f0233c14668df9f85c6a78a81828f4251194/src/vs/platform/environment/common/environmentService.ts#L104),
  [`149`](https://github.com/trypear/pearai-app/blob/d930f0233c14668df9f85c6a78a81828f4251194/src/vs/platform/environment/common/environmentService.ts#L149),
  [`250`](https://github.com/trypear/pearai-app/blob/d930f0233c14668df9f85c6a78a81828f4251194/src/vs/platform/environment/common/environmentService.ts#L250)).
- Remote server: `~/.pearai-server/`
  ([`src/vs/server/node/server.main.ts:39`](https://github.com/trypear/pearai-app/blob/d930f0233c14668df9f85c6a78a81828f4251194/src/vs/server/node/server.main.ts#L39)),
  with `data/User` as in VS Code.
- Extension `globalStorage` folders are the lowercased extension id
  ([`extHostStoragePaths.ts:90`](https://github.com/trypear/pearai-app/blob/d930f0233c14668df9f85c6a78a81828f4251194/src/vs/workbench/api/common/extHostStoragePaths.ts#L90)):
  `User/globalStorage/pearai.pearai` and `User/globalStorage/pearai.pearai-roo-cline`.

#### Continue fork (`pearai.pearai`)

Global directory is `$CONTINUE_GLOBAL_DIR` or `os.homedir()/.pearai` on every OS
([`core/util/paths.ts:11-21`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/util/paths.ts#L11-L21)).
Layout is Continue's: `sessions/<id>.json` and `sessions/sessions.json`
([`23-53`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/util/paths.ts#L23-L53)),
`config.json`, `config.ts`, `package.json`, `types/`, `tsconfig.json`, `out/config.js`
([`55-144`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/util/paths.ts#L55-L144)),
`.continuerc.json`
([`146-162`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/util/paths.ts#L146-L162)),
`dev_data/devdata.sqlite` and `dev_data/<table>.jsonl`
([`164-178`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/util/paths.ts#L164-L178)),
`.migrations/`
([`203-209`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/util/paths.ts#L203-L209)),
`index/index.sqlite`, `index/lancedb/`, `index/autocompleteCache.sqlite`, `index/docs.sqlite`,
`index/globalContext.json`
([`31-41`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/util/paths.ts#L31-L41),
[`233-247`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/util/paths.ts#L233-L247)),
`.configs/<host>/`
([`249-287`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/util/paths.ts#L249-L287)),
`.env`
([`289-295`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/util/paths.ts#L289-L295)),
`logs/core.log`, `logs/prompt.log`
([`297-311`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/util/paths.ts#L297-L311))
and `.prompts/`
([`313-315`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/util/paths.ts#L313-L315)).
PearAI additions in the same directory:

- `pearai_memories.json`: local "memories" the assistant keeps about the user, written by both
  extensions
  ([`extensions/vscode/src/integrations/mem0/localMemoryService.ts:33-35`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/extensions/vscode/src/integrations/mem0/localMemoryService.ts#L33-L35),
  [`util/pearai/pearaiMemory.ts:1`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/extensions/vscode/src/util/pearai/pearaiMemory.ts#L1);
  [`PearAI-Roo-Code src/utils/memory.ts:23`](https://github.com/trypear/PearAI-Roo-Code/blob/0b6df736c9b2799c38b9ee629668407521a7bdda/src/utils/memory.ts#L23),
  [`39-42`](https://github.com/trypear/PearAI-Roo-Code/blob/0b6df736c9b2799c38b9ee629668407521a7bdda/src/utils/memory.ts#L39-L42)).
- `pearai.log`
  ([`commands.ts:603-604`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/extensions/vscode/src/commands.ts#L603-L604)),
  `.diffs/`
  ([`diff/horizontal.ts:30-32`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/extensions/vscode/src/diff/horizontal.ts#L30-L32)),
  `firstLaunch.flag`, `firstLaunchCreator.flag`
  ([`copySettings.ts:8-12`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/extensions/vscode/src/copySettings.ts#L8-L12)).

#### Import from VS Code

The welcome flow's "import settings" copies VS Code's `settings.json`, `keybindings.json`,
`snippets`, `sync` and `globalStorage/state.vscdb` into a PearAI `User` directory, and every VS Code
extension except AI ones into `~/.pearai/extensions`
([`copySettings.ts:40-102`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/extensions/vscode/src/copySettings.ts#L40-L102),
[`141-145`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/extensions/vscode/src/copySettings.ts#L141-L145)).
The target is built with a lowercase `pearai`
([`copySettings.ts:24-33`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/extensions/vscode/src/copySettings.ts#L24-L33)),
so on Linux it is `~/.config/pearai/User`, a different directory from the `~/.config/PearAI/User`
the editor uses. A `state.vscdb` there is a copy of the user's VS Code state, not PearAI activity.

#### Roo Code fork (`pearai.pearai-roo-cline`)

Storage is the extension's `globalStorage` unless the `roo-cline.customStoragePath` setting moves it
([`src/shared/storagePathManager.ts:11-27`](https://github.com/trypear/PearAI-Roo-Code/blob/0b6df736c9b2799c38b9ee629668407521a7bdda/src/shared/storagePathManager.ts#L11-L27)):
`tasks/<taskId>/`
([`52-57`](https://github.com/trypear/PearAI-Roo-Code/blob/0b6df736c9b2799c38b9ee629668407521a7bdda/src/shared/storagePathManager.ts#L52-L57))
with `api_conversation_history.json`, `ui_messages.json`, `task_metadata.json`;
`settings/custom_modes.json` and `settings/pearai_agent_mcp_settings.json`
([`src/shared/globalFileNames.ts:1-7`](https://github.com/trypear/PearAI-Roo-Code/blob/0b6df736c9b2799c38b9ee629668407521a7bdda/src/shared/globalFileNames.ts#L1-L7);
[`storagePathManager.ts:62-67`](https://github.com/trypear/PearAI-Roo-Code/blob/0b6df736c9b2799c38b9ee629668407521a7bdda/src/shared/storagePathManager.ts#L62-L67));
`cache/`
([`74`](https://github.com/trypear/PearAI-Roo-Code/blob/0b6df736c9b2799c38b9ee629668407521a7bdda/src/shared/storagePathManager.ts#L74));
shadow-git checkpoints in `tasks/<id>/checkpoints` or `checkpoints/<hash>`
([`ShadowCheckpointService.ts:348`](https://github.com/trypear/PearAI-Roo-Code/blob/0b6df736c9b2799c38b9ee629668407521a7bdda/src/services/checkpoints/ShadowCheckpointService.ts#L348),
[`358`](https://github.com/trypear/PearAI-Roo-Code/blob/0b6df736c9b2799c38b9ee629668407521a7bdda/src/services/checkpoints/ShadowCheckpointService.ts#L358)).
The task list is the `taskHistory` key in the editor's global state (`state.vscdb`;
[`ClineProvider.ts:1091`](https://github.com/trypear/PearAI-Roo-Code/blob/0b6df736c9b2799c38b9ee629668407521a7bdda/src/core/webview/ClineProvider.ts#L1091)).
MCP server checkouts go to Roo Code's own directories, `AppData/Roaming/Roo-Code/MCP`,
`Documents/Cline/MCP` (macOS), `.local/share/Roo-Code/MCP`, fallback `~/.roo-code/mcp`
([`ClineProvider.ts:952-973`](https://github.com/trypear/PearAI-Roo-Code/blob/0b6df736c9b2799c38b9ee629668407521a7bdda/src/core/webview/ClineProvider.ts#L952-L973)),
which the catalog already collects as `roo-code` and `cline`.

#### Output channel logs

The editor backs every extension output channel with a file under its `logs/` directory; the chat
extension's `PearAI` channel, `<n>-PearAI.log`, is a prompt log of every model request
([Output channel log](#output-channel-log)). The files are collected by the `.config/PearAI/logs`,
`Library/Application Support/PearAI/logs` and `AppData/Roaming/PearAI/logs` lines.

### Credentials

- PearAI account: the Continue fork stores `pearai-token` and `pearai-refresh` in VS Code
  SecretStorage
  ([`commands.ts:1004-1005`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/extensions/vscode/src/commands.ts#L1004-L1005));
  the Roo fork stores `pearaiApiKey` and `pearaiRefreshKey`
  ([`src/extension.ts:112-113`](https://github.com/trypear/PearAI-Roo-Code/blob/0b6df736c9b2799c38b9ee629668407521a7bdda/src/extension.ts#L112-L113)).
  Both land as encrypted `secret://` rows in `User/globalStorage/state.vscdb` (see
  [vscode.md](vscode.md)); not recoverable offline without the OS keyring. Roo's other provider keys
  go to the same store.
- `~/.pearai/config.json` and `config.ts` model entries carry `apiKey`
  ([`core/index.d.ts:810-814`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/index.d.ts#L810-L814),
  `ModelDescription`); `~/.pearai/.env` is loaded as environment
  ([`paths.ts:289-295`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/util/paths.ts#L289-L295));
  `.configs/<host>/config.json` is a remote config.
- `settings/pearai_agent_mcp_settings.json` in the Roo fork's storage and project
  `.pearai-agent/mcp.json`
  ([`src/services/mcp/McpHub.ts:502-503`](https://github.com/trypear/PearAI-Roo-Code/blob/0b6df736c9b2799c38b9ee629668407521a7bdda/src/services/mcp/McpHub.ts#L502-L503))
  can hold MCP server `env` secrets.
- `config.json` (or `config.ts`) context providers can carry source-host tokens: the `issue`
  provider (GitHub Issues) takes `params.githubToken` for its GitHub API client
  ([`core/context/providers/GitHubIssuesContextProvider.ts:12`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/context/providers/GitHubIssuesContextProvider.ts#L12),
  [`26`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/context/providers/GitHubIssuesContextProvider.ts#L26)),
  and the `gitlab-mr` provider takes `params.token` and `params.domain` (default `gitlab.com`)
  ([`core/context/providers/GitLabMergeRequestContextProvider.ts:74`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/context/providers/GitLabMergeRequestContextProvider.ts#L74),
  [`83-94`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/context/providers/GitLabMergeRequestContextProvider.ts#L83-L94));
  each provider is built from its `contextProviders[]` entry's `params`
  ([`core/config/load.ts:392-402`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/config/load.ts#L392-L402)).
  The `.pearai/config.json` secret glob already flags the file.
- `index/globalContext.json` holds only UI state in this fork
  ([`core/util/GlobalContext.ts:5-16`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/util/GlobalContext.ts#L5-L16)),
  unlike upstream Continue; it need not be flagged.

### Exclusions

- `.pearai/extensions`: extension binaries, the equivalent of the
  `.cursor/extensions` exclusion, and after a VS Code import a full copy of
  the user's VS Code extensions.
- `.pearai-server/bin`, `.pearai-server/extensions` (the catalog collects only
  `.pearai-server/data/User`, so these are never reached).
- Continue fork: `.pearai/index/lancedb`, `.pearai/index/*.sqlite`,
  `.pearai/types`, `.pearai/out`, `.pearai/node_modules`, `.pearai/.diffs`,
  `.pearai/.migrations`, `.pearai/dev_data/devdata.sqlite`, mirroring the
  Continue exclusions. Keep `index/globalContext.json`.
- Roo fork: `*/User/globalStorage/pearai.pearai-roo-cline/checkpoints`,
  `.../tasks/*/checkpoints` and `.../cache`, mirroring Roo Code.

### Project-local files

- Continue fork: `.pearairc.json` at each workspace root
  ([`extensions/vscode/src/ideProtocol.ts:310-322`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/extensions/vscode/src/ideProtocol.ts#L310-L322)),
  `.pearaiignore`
  ([`core/indexing/walkDir.ts:209`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/indexing/walkDir.ts#L209)),
  and `.prompts/*.prompt` slash commands
  ([`core/config/load.ts:185-200`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/config/load.ts#L185-L200);
  [`core/config/promptFile.ts:10`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/config/promptFile.ts#L10)).
- Roo fork renames Roo's `.roo` to `.pearai-agent/` (`rules/`, `rules-<mode>/`,
  `system-prompt-<mode>`, `mcp.json`), `.rooignore` to `.pearai-agent-ignore` and `.roomodes` to
  `.pearai-agent-modes`
  ([`src/shared/constants.ts:1-5`](https://github.com/trypear/PearAI-Roo-Code/blob/0b6df736c9b2799c38b9ee629668407521a7bdda/src/shared/constants.ts#L1-L5);
  [`custom-instructions.ts:161`](https://github.com/trypear/PearAI-Roo-Code/blob/0b6df736c9b2799c38b9ee629668407521a7bdda/src/core/prompts/sections/custom-instructions.ts#L161),
  [`197`](https://github.com/trypear/PearAI-Roo-Code/blob/0b6df736c9b2799c38b9ee629668407521a7bdda/src/core/prompts/sections/custom-instructions.ts#L197);
  [`custom-system-prompt.ts:50`](https://github.com/trypear/PearAI-Roo-Code/blob/0b6df736c9b2799c38b9ee629668407521a7bdda/src/core/prompts/sections/custom-system-prompt.ts#L50)),
  and still reads `.roorules`, `.clinerules` and `.clinerules-<mode>`
  ([`custom-instructions.ts:170`](https://github.com/trypear/PearAI-Roo-Code/blob/0b6df736c9b2799c38b9ee629668407521a7bdda/src/core/prompts/sections/custom-instructions.ts#L170),
  [`213`](https://github.com/trypear/PearAI-Roo-Code/blob/0b6df736c9b2799c38b9ee629668407521a7bdda/src/core/prompts/sections/custom-instructions.ts#L213)).
- No per-project database.

### Where the project path is recorded

- `~/.pearai/sessions/sessions.json` `[].workspaceDirectory` and `sessions/<id>.json`
  `workspaceDirectory`
  ([`core/util/history.ts:118-135`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/util/history.ts#L118-L135);
  [`gui/src/hooks/useHistory.tsx:79-85`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/gui/src/hooks/useHistory.tsx#L79-L85)),
  the first workspace folder as a plain path or empty.
- Roo fork: `taskHistory[].workspace` inside `state.vscdb`
  ([`src/schemas/index.ts:339-352`](https://github.com/trypear/PearAI-Roo-Code/blob/0b6df736c9b2799c38b9ee629668407521a7bdda/src/schemas/index.ts#L339-L352)),
  SQLite only.
- Editor: `User/workspaceStorage/*/workspace.json` as in VS Code.
- Roo fork, Creator mode: `taskHistory[].creatorModeConfig.newProjectPath`, the directory Creator
  mode created for the task
  ([`src/schemas/index.ts:327-331`](https://github.com/trypear/PearAI-Roo-Code/blob/0b6df736c9b2799c38b9ee629668407521a7bdda/src/schemas/index.ts#L327-L331)),
  in `state.vscdb` like the `workspace` key.

## Parsing

### Transcript stores

Chat (Continue fork), `~/.pearai` or `$CONTINUE_GLOBAL_DIR`
([`core/util/paths.ts:11-12`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/util/paths.ts#L11-L12)):

- `.pearai/sessions/<sessionId>.json`: one session, compact JSON
  ([`paths.ts:43-45`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/util/paths.ts#L43-L45);
  [`core/util/history.ts:91-96`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/util/history.ts#L91-L96)).
- `.pearai/sessions/sessions.json`: index array
  ([`paths.ts:47-53`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/util/paths.ts#L47-L53)).

Agent (Roo fork): `api_conversation_history.json`, `ui_messages.json` and `task_metadata.json` in
`<User>/globalStorage/pearai.pearai-roo-cline/tasks/<taskId>/` or under
`roo-cline.customStoragePath` ([Roo Code fork](#roo-code-fork-pearaipearai-roo-cline)).

#### What survives a delete

Deleting a chat session removes its `sessions/<sessionId>.json` file and its `sessions.json` entry
only
([`core/util/history.ts:41-66`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/util/history.ts#L41-L66));
deleting an agent task removes its `taskHistory` entry, its shadow checkpoint repository or branch
and its task directory
([`src/core/webview/ClineProvider.ts:1138-1169`](https://github.com/trypear/PearAI-Roo-Code/blob/0b6df736c9b2799c38b9ee629668407521a7bdda/src/core/webview/ClineProvider.ts#L1138-L1169)).
A per-prompt history file is not stated for either surface. Neither delete touches the other records
of chat use in [Other stores](#other-stores): the `PearAI` output channel log, which holds every
chat prompt and completion with no session id or per-entry time and lasts until the editor drops its
log session directory ([Output channel log](#output-channel-log));
`dev_data/chat.jsonl`, the rated prompts with their `sessionId` and no time;
`dev_data/tokens_generated.jsonl`, which names the models used; the `/share` Markdown exports; and
`pearai_memories.json`, derived from chats but naming none. PearAI's analytics may hold the ids of
tasks deleted locally, but nothing on disk marks a deletion. The analyzer reads the output channel
log and `dev_data/chat.jsonl` as secondary sources of the chat sessions, so a deleted session's
logged prompts and replies and its rated replies are kept, and `dev_data/tokens_generated.jsonl` for
the models used (see [Secondary sources](../analyzer/docs/output.md#secondary-sources) and
[Past-use rows and secondary sources](#past-use-rows-and-secondary-sources)); the `/share` exports
and `pearai_memories.json` are not read. A deleted agent task leaves nothing the analyzer reads,
since its `taskHistory` entry goes with its directory.

### Record schema

#### Chat session

`PersistedSessionInfo`
([`core/index.d.ts:207-213`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/index.d.ts#L207-L213)):
`sessionId`, `title`, `workspaceDirectory`, `history: ChatHistoryItem[]` and
`perplexityHistory: ChatHistoryItem[]`, the second holding the "PearAI Search" (Perplexity)
conversation. The GUI saves one or the other per session
([`gui/src/hooks/useHistory.tsx:43-51`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/gui/src/hooks/useHistory.tsx#L43-L51),
[`79-85`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/gui/src/hooks/useHistory.tsx#L79-L85)).

`ChatHistoryItem`
([`index.d.ts:325-332`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/index.d.ts#L325-L332)):
`message`, `contextItems` (`content`, `name`, `description`, `id`
[`298-306`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/index.d.ts#L298-L306)),
`editorState?`, `modifiers?`, `promptLogs?` (`completionOptions.model`, `prompt`, `completion`;
[`314-318`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/index.d.ts#L314-L318),
[`257-259`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/index.d.ts#L257-L259)),
`citations?` (`url`, `title`).

`ChatMessage`
([`index.d.ts:261-275`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/index.d.ts#L261-L275)):
`role` is only `user`, `assistant` or `system`; `content` is a string or
`[{type:"text",text}|{type:"imageUrl",imageUrl:{url}}]`. **There are no tool calls, `tool` messages
or `thinking` messages** in this fork, unlike current Continue. No per-message timestamp.

`sessions.json` entry (`SessionInfo`,
[`index.d.ts:215-221`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/index.d.ts#L215-L221)):
`sessionId`, `title`, `dateCreated` (`String(Date.now())`, millisecond epoch as a string),
`workspaceDirectory`, `integrationType` (`"continue"` or `"perplexity"`;
[`history.ts:7-20`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/util/history.ts#L7-L20),
[`118-135`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/util/history.ts#L118-L135)).

#### Agent task

Roo 3.15 shape. `api_conversation_history.json` is an array of Anthropic `MessageParam` plus `ts`
(`Date.now()`, ms epoch;
[`src/core/Cline.ts:323-327`](https://github.com/trypear/PearAI-Roo-Code/blob/0b6df736c9b2799c38b9ee629668407521a7bdda/src/core/Cline.ts#L323-L327)).
Tool use is **not** native `tool_use` blocks: the assistant message is one text block whose text
contains XML tool tags such as `<execute_command><command>…</command></execute_command>`
([`Cline.ts:1905-1909`](https://github.com/trypear/PearAI-Roo-Code/blob/0b6df736c9b2799c38b9ee629668407521a7bdda/src/core/Cline.ts#L1905-L1909);
[`src/core/assistant-message/parse-assistant-message.ts:6-30`](https://github.com/trypear/PearAI-Roo-Code/blob/0b6df736c9b2799c38b9ee629668407521a7bdda/src/core/assistant-message/parse-assistant-message.ts#L6-L30)),
and the result is a user message whose first text block is `[<tool> for '<arg>'] Result:` followed
by the output
([`Cline.ts:1271-1276`](https://github.com/trypear/PearAI-Roo-Code/blob/0b6df736c9b2799c38b9ee629668407521a7bdda/src/core/Cline.ts#L1271-L1276),
[`1341-1352`](https://github.com/trypear/PearAI-Roo-Code/blob/0b6df736c9b2799c38b9ee629668407521a7bdda/src/core/Cline.ts#L1341-L1352)).
`ui_messages.json` is `ClineMessage[]`: `ts`, `type` (`ask`/`say`), `ask`, `say`, `text`, `images`,
`partial`, `reasoning`, `conversationHistoryIndex`, `checkpoint`
([`src/schemas/index.ts:868-880`](https://github.com/trypear/PearAI-Roo-Code/blob/0b6df736c9b2799c38b9ee629668407521a7bdda/src/schemas/index.ts#L868-L880)).
`task_metadata.json` holds only `files_in_context`
([`src/core/context-tracking/FileContextTrackerTypes.ts:23-25`](https://github.com/trypear/PearAI-Roo-Code/blob/0b6df736c9b2799c38b9ee629668407521a7bdda/src/core/context-tracking/FileContextTrackerTypes.ts#L23-L25)).
The `taskHistory` entry (`HistoryItem`) is `id`, `number`, `ts`, `task`, `tokensIn`, `tokensOut`,
`cacheWrites`, `cacheReads`, `totalCost`, `size`, `workspace` and
`creatorModeConfig{creatorMode, newProjectType, newProjectPath}`
([`src/schemas/index.ts:327-331`](https://github.com/trypear/PearAI-Roo-Code/blob/0b6df736c9b2799c38b9ee629668407521a7bdda/src/schemas/index.ts#L327-L331),
[`:339-352`](https://github.com/trypear/PearAI-Roo-Code/blob/0b6df736c9b2799c38b9ee629668407521a7bdda/src/schemas/index.ts#L339-L352)),
set for a task started from PearAI Creator ([Scratch directories](#scratch-directories)).

#### Permission records

On the agent surface, a denied tool call is answered in the API history with
the plain text `The user denied this operation.`, or
`The user denied this operation and provided the following feedback:` followed by a `<feedback>`
block, and an approval with a typed note with
`The user approved this operation and provided the following context:` and a `<feedback>` block
([`src/core/prompts/responses.ts:8-14`](https://github.com/trypear/PearAI-Roo-Code/blob/0b6df736c9b2799c38b9ee629668407521a7bdda/src/core/prompts/responses.ts#L8-L14)).
An approval without a note, by the user or by the auto-approve settings, leaves no mark. The
denial and approval texts are pushed as the second text block of the tool result, after
`[<tool> for '<arg>'] Result:`; an approval with a note is followed by a second result block with
the tool's output
([`src/core/Cline.ts:1342-1386`](https://github.com/trypear/PearAI-Roo-Code/blob/0b6df736c9b2799c38b9ee629668407521a7bdda/src/core/Cline.ts#L1342-L1386)).
The `ui_messages.json` record of the call is the `ask` before the answer, and a typed note is a
`say: "user_feedback"` record; neither says which button the user pressed.

The standing auto-approval state is the global-state keys `autoApprovalEnabled`,
`alwaysAllowReadOnly`, `alwaysAllowReadOnlyOutsideWorkspace`, `alwaysAllowWrite`,
`alwaysAllowWriteOutsideWorkspace`, `alwaysAllowBrowser`, `alwaysApproveResubmit`, `alwaysAllowMcp`,
`alwaysAllowModeSwitch`, `alwaysAllowSubtasks`, `alwaysAllowExecute` and `allowedCommands`
([`src/schemas/index.ts:584-597`](https://github.com/trypear/PearAI-Roo-Code/blob/0b6df736c9b2799c38b9ee629668407521a7bdda/src/schemas/index.ts#L584-L597)),
each stored with `globalState.update(key, value)`
([`src/core/config/ContextProxy.ts:119-123`](https://github.com/trypear/PearAI-Roo-Code/blob/0b6df736c9b2799c38b9ee629668407521a7bdda/src/core/config/ContextProxy.ts#L119-L123)),
so they sit in the extension's row of `state.vscdb` beside `taskHistory`; and each MCP server's
`alwaysAllow` list in `settings/pearai_agent_mcp_settings.json` or a project's
`.pearai-agent/mcp.json`
([`src/services/mcp/McpHub.ts:47`](https://github.com/trypear/PearAI-Roo-Code/blob/0b6df736c9b2799c38b9ee629668407521a7bdda/src/services/mcp/McpHub.ts#L47)).

### Joins and timestamps

- Chat session to project: `workspaceDirectory` in both the session and
  `sessions.json`; time from `sessions.json` `dateCreated`.
- `dev_data/chat.jsonl` rows carry `sessionId`.
- Agent task to project: `taskHistory[].workspace` ([Where the project path is recorded](#where-the-project-path-is-recorded)),
  in the editor's `User/globalStorage/state.vscdb` `ItemTable`, key `taskHistory` of the
  extension's global state. The task directory name is `HistoryItem.id`.
- Tool call to result: by position, the assistant XML block followed by the
  next user message's `[tool ...] Result:` block. No ids.

### Other stores

`dev_data/devdata.sqlite` (token counts, excluded by the collector);
`index/*.sqlite` (code index, excluded). The agent's `taskHistory` is in
`state.vscdb`.

#### Output channel log

The chat extension hands its core a log callback that appends every model request to the editor
output channel named `PearAI`
([`extensions/vscode/src/extension/VsCodeExtension.ts:132`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/extensions/vscode/src/extension/VsCodeExtension.ts#L132),
[`:147-155`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/extensions/vscode/src/extension/VsCodeExtension.ts#L147-L155)).
Logging is on unless a caller passes `log: false`
([`core/llm/index.ts:361`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/llm/index.ts#L361)),
and each chat request writes `Settings:`, the context length and completion options (the model among
them), a `####` rule and the full prompt, then `Completion:` and the reply
([`:247-261`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/llm/index.ts#L247-L261),
[`:551-554`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/llm/index.ts#L551-L554),
[`:586-587`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/llm/index.ts#L586-L587)).
The editor, a VS Code fork, backs every extension output channel with a file,
`<User data dir>/logs/<YYYYMMDDTHHMMSS>/window<N>/exthost/output_logging_<time>/<n>-PearAI.log`
([`pearai-app src/vs/workbench/api/common/extHostOutput.ts:121`](https://github.com/trypear/pearai-app/blob/d930f0233c14668df9f85c6a78a81828f4251194/src/vs/workbench/api/common/extHostOutput.ts#L121),
[`:160-170`](https://github.com/trypear/pearai-app/blob/d930f0233c14668df9f85c6a78a81828f4251194/src/vs/workbench/api/common/extHostOutput.ts#L160-L170)),
and keeps the newest 49 log session directories besides the current one
([`src/vs/workbench/contrib/logs/common/logsDataCleaner.ts:24-35`](https://github.com/trypear/pearai-app/blob/d930f0233c14668df9f85c6a78a81828f4251194/src/vs/workbench/contrib/logs/common/logsDataCleaner.ts#L24-L35)).
Each write to the channel is two lines of 74 `=` and then the text
([`VsCodeExtension.ts:147-155`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/extensions/vscode/src/extension/VsCodeExtension.ts#L147-L155)):
a request is `Settings:`, one `key: value` line per setting (`contextLength` first, then the
completion options, `model` among them), a blank line, a line of 44 `#`, a blank line and the
prompt, with no newline added after it; a reply is `Completion:`, a blank line, the reply and two
newlines
([`core/llm/index.ts:247-261`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/llm/index.ts#L247-L261),
[`:586-587`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/llm/index.ts#L586-L587)).
A chat prompt with no template is the messages as `<system>`, `<user>` and `<assistant>` blocks,
each `<role>`, a newline, the content and a blank line
([`:373-384`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/llm/index.ts#L373-L384)),
the context items' contents inside the user message they were sent with; a model with a prompt
template logs the templated text instead
([`:548-550`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/llm/index.ts#L548-L550)).
Autocomplete requests are logged the same way, as `<prefix><FIM><suffix>` or a raw prompt, since
autocomplete does not pass `log: false`
([`:402-406`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/llm/index.ts#L402-L406);
[`core/autocomplete/completionProvider.ts:646-653`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/autocomplete/completionProvider.ts#L646-L653)).
The editor writes the channel's text unformatted, with no time or level prefix
([`extHostOutput.ts:47-49`](https://github.com/trypear/pearai-app/blob/d930f0233c14668df9f85c6a78a81828f4251194/src/vs/workbench/api/common/extHostOutput.ts#L47-L49),
[`:166`](https://github.com/trypear/pearai-app/blob/d930f0233c14668df9f85c6a78a81828f4251194/src/vs/workbench/api/common/extHostOutput.ts#L166)).
The file has no session id and no per-entry time; the log session directory's name and the file's
modification time bound it. It outlives a deleted chat session. `logs/prompt.log` under `~/.pearai`
is written only by the JetBrains binary
([`binary/src/index.ts:34-37`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/binary/src/index.ts#L34-L37)),
which PearAI does not ship as an editor.

#### `dev_data` tables

`.pearai/dev_data/<table>.jsonl`: appended JSON lines
([`core/util/devdata.ts:4-8`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/util/devdata.ts#L4-L8));
tables `chat` (written only when the user clicks feedback;
[`gui/src/components/gui/StepContainer.tsx:87-96`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/gui/src/components/gui/StepContainer.tsx#L87-L96)),
`tokens_generated`
([`core/llm/index.ts:286-291`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/llm/index.ts#L286-L291))
and `autocomplete`.

`dev_data/chat.jsonl` gets one line per prompt log of an item the user rated,
`{...promptLog, feedback, sessionId}`, where a prompt log is
`{completionOptions, prompt, completion}`
([`gui/src/components/gui/StepContainer.tsx:88-97`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/gui/src/components/gui/StepContainer.tsx#L88-L97));
`dev_data/tokens_generated.jsonl` one line per request,
`{model, provider, promptTokens, generatedTokens}`
([`core/llm/index.ts:286-291`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/llm/index.ts#L286-L291)).
Lines are only appended
([`core/util/devdata.ts:4-8`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/util/devdata.ts#L4-L8)),
carry no time, and survive a session delete.

#### `/share` exports

`/share` is a default slash command
([`core/config/default.ts:64`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/config/default.ts#L64))
that writes `<YYYYMMDDTHHMMSS>_session.md`, a Markdown copy of the session, to `~/.pearai` or the
given `outputDir`, never to a server
([`core/commands/slash/share.ts:38-98`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/commands/slash/share.ts#L38-L98)).
The session delete does not remove it.

#### `pearai_memories.json`

`pearai_memories.json`, the assistant's memories about the user, a JSON array of
`{id, content, timestamp, isNew?, isModified?, isDeleted?}`
([`extensions/vscode/src/integrations/mem0/types.ts:1-8`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/extensions/vscode/src/integrations/mem0/types.ts#L1-L8),
[`localMemoryService.ts:33-35`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/extensions/vscode/src/integrations/mem0/localMemoryService.ts#L33-L35)),
written by both extensions; it is derived from chats but names none.

#### Aider integration

The integration with Aider, a separate terminal coding agent, is commented out in full at this
commit
([`extensions/vscode/src/integrations/aider/aiderProcess.ts:121-146`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/extensions/vscode/src/integrations/aider/aiderProcess.ts#L121-L146)),
so PearAI writes no Aider state.

### Format versions

No version field in either store. The Continue fork predates Continue's
tool-calling session format. The Roo fork is Roo Code 3.15.3 and predates every Roo Code change
since ([`roo-code.md`](roo-code.md) describes 3.53.0): it has no native tool calls, no
`history_item.json`, no `isAnswered`, plain-text denials, and a `creatorModeConfig` of its own.

### Secrets

`promptLogs[].prompt` and `dev_data/chat.jsonl` carry full prompts with
context items, which can include file and `.env` contents. Tool results
in `api_conversation_history.json` hold raw command output. No account
tokens are written to these files; they are in SecretStorage.

### Parser plan

Globs (home-relative): `.pearai/sessions/*.json` (read `sessions.json`
as the index through the Continue parser's `read_index`; see [Bad-line rows](#bad-line-rows)),
`*/User/globalStorage/pearai.pearai-roo-cline/tasks/*/{ui_messages,api_conversation_history,task_metadata}.json`
(read through the shared Cline task mapping, with the XML-in-text tool calls handled by the PearAI
parser), `*/User/globalStorage/state.vscdb` (the `pearai.pearai-roo-cline` row),
`*/logs/*/window*/exthost/output_logging_*/*-PearAI.log`, `.pearai/dev_data/chat.jsonl` and
`.pearai/dev_data/tokens_generated.jsonl`.

| Column | Chat session | Agent task |
| --- | --- | --- |
| timestamp_utc | `sessions.json` `dateCreated` for every row | message `ts` |
| session_id | `sessionId` | task directory name |
| project_path | `workspaceDirectory` | `taskHistory[].workspace` when `state.vscdb` is present |
| git_branch | empty | empty |
| turn_type | `message.role`; items from `perplexityHistory` also by role | `role`; XML tool tag → tool_use; `[x] Result:` block → tool_result |
| model | `promptLogs[0].completionOptions.model` | empty |
| tool_name | empty | XML tag name |
| tool_use_id | empty | the assistant message's position in the array |
| text | `message.content` (text parts joined) | text block |

#### Permission rows

Source-derived; not observed on any real install. Agent surface, from the texts in
[Permission records](#permission-records): the `tool_result` row of a result whose first block
after the `Result:` line starts `The user denied this operation.` is followed by a `system` row
`permission decision: denied`, one starting `The user denied this operation and provided the
following feedback:` by `permission decision: denied with feedback`, and one starting `The user
approved this operation and provided the following context:` by `permission decision: approved with
feedback`, with the call's `tool_name` and `tool_use_id` at the API message's `ts`. When
`ui_messages.json` holds the task's transcript, `api_conversation_history.json` is still read for
these rows alone, and the `tool_use_id` is the `ts` of the last tool ask in `ui_messages.json` at or
before the API message, the id the `ui_messages.json` rows give that call. Standing: each
auto-approval key present in the extension's row of `state.vscdb` is one `permission setting:
<key>=<value> (from state.vscdb pearai.pearai-roo-cline, file mtime)` row at the file's
modification time. The MCP servers' `alwaysAllow` lists are not read yet. The chat surface has no
tools and no permission state.

#### Model rows

Model rows, typed `system`, chat sessions only: the first item whose
`promptLogs[].completionOptions.model` names a model yields `model:
<model>` ahead of the item's rows, and a later item naming a different
one yields `model: <old> -> <new>`. Mid-session switching is not
researched: the format has no switch record, so a change shows only as a
different value between items. Agent tasks record no model (the Roo fork
writes no `modelInfo` and no `model_usage`). Source-derived; not observed on any real install: the
output channel log's `model` setting yields `model: <id>` and `model: <old> -> <new>` rows tracked
over the file, with no session, and each `dev_data/tokens_generated.jsonl` line a `system` row
`tokens generated: model=<model> provider=<provider> promptTokens=<n> generatedTokens=<n>` with the
model in the `model` column.

#### Past-use rows and secondary sources

Source-derived; not observed on any real install.

- The `PearAI` output channel log ([Output channel log](#output-channel-log)), secondary: a
  request in the `<role>` block form gives its last `<user>` block as a `user` row and its
  `Completion:` as an `assistant` row; any other request (autocomplete, a templated prompt) gives
  `model request: <prompt>` and `model completion: <reply>` rows. No time, session or project, so
  the rows match the session rows on text alone; a prompt sent with context items carries their
  contents and never pairs with the session's `user` row. A record that does not parse is a
  `parser:` row ([Bad-line rows](#bad-line-rows)).
- `dev_data/chat.jsonl` ([`dev_data` tables](#dev_data-tables)), secondary: per line a `user` row
  from the prompt as above, an `assistant` row from `completion` and `user feedback: <true|false>
  (from dev_data/chat.jsonl)`, with the line's `sessionId` and the session's `workspaceDirectory`
  from `sessions.json`, no time.
- `dev_data/tokens_generated.jsonl`, primary, since no other store repeats it: one `tokens
  generated:` row per line ([Model rows](#model-rows)).
- The agent surface's `taskHistory` entries, primary: one `system` row per entry, `task: <task> |
  tokensIn=<n> tokensOut=<n> totalCost=<n> number=<n>` at the entry's `ts`, the task id as
  `session_id` and `workspace` as `project_path`, so a task whose directory is missing from the
  collection still shows.
- The `/share` Markdown files and `pearai_memories.json` are not transcripts the dedup rule can
  pair and stay unread.

#### Sync, relay and origin rows

Sync: neither surface records sync state, and neither uploads a session, a transcript or a prompt
history beyond the model call. The chat fork keeps upstream Continue's local-only paths
([Sync, relay and origin rows in `continue.md`](continue.md#sync-relay-and-origin-rows)), with these
differences: `/share` writes a local Markdown export, never to a server
([`/share` exports](#share-exports)); the `dev_data` tables are appended locally with no remote
destination ([`dev_data` tables](#dev_data-tables)); and the fork has no `cn` CLI and so no
`serve --id` upload.

What does leave for PearAI's server (`https://server.trypear.ai/pearai-server-api2`,
[`core/util/parameters.ts:38`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/util/parameters.ts#L38))
carries no message text: before every chat request a `GET /anonymousTelemetry` whose headers are the
model, the provider and the event name `streamChat`
([`core/pearaiServer/util.ts:5-28`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/pearaiServer/util.ts#L5-L28);
[`core/llm/index.ts:535-546`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/llm/index.ts#L535-L546)),
on by default and turned off by `allowAnonymousTelemetry: false` in `~/.pearai/config.json`
([`core/config/load.ts:121-122`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/config/load.ts#L121-L122);
[`core/config/profile/doLoadConfig.ts:47-52`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/config/profile/doLoadConfig.ts#L47-L52);
the code comment names the key `sendAnonymousTelemetry`, which nothing reads); and, for PearAI's
hosted models, a `POST /log_tokens` with token counts
([`core/llm/llms/PearAIServer.ts:320-339`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/llm/llms/PearAIServer.ts#L320-L339)).
The PostHog product analytics client is commented out
([`core/util/posthog.ts:36-64`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/util/posthog.ts#L36-L64)),
and team analytics (`TeamAnalytics`, upstream's PostHog, Logstash (Elastic's log pipeline) or
Continue-proxy sink chosen by an `analytics` block in the config) start only when that block is
present and send event names and counts
([`core/control-plane/TeamAnalytics.ts:8-54`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/control-plane/TeamAnalytics.ts#L8-L54);
[`core/config/profile/doLoadConfig.ts:54-59`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/config/profile/doLoadConfig.ts#L54-L59);
[`core/llm/index.ts:262-293`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/llm/index.ts#L262-L293)).

The agent fork has no cloud or share feature (the Roo Code Cloud service of later upstream releases
is absent from `src/services`); its PostHog client is forced on
([`src/services/telemetry/PostHogClient.ts:53`](https://github.com/trypear/PearAI-Roo-Code/blob/0b6df736c9b2799c38b9ee629668407521a7bdda/src/services/telemetry/PostHogClient.ts#L53),
[`86-104`](https://github.com/trypear/PearAI-Roo-Code/blob/0b6df736c9b2799c38b9ee629668407521a7bdda/src/services/telemetry/PostHogClient.ts#L86-L104))
and sends task events keyed by the local task id (`taskId`, the task directory name) with
`source: "user"` or `"assistant"` and no message text
([`src/services/telemetry/TelemetryService.ts:87-101`](https://github.com/trypear/PearAI-Roo-Code/blob/0b6df736c9b2799c38b9ee629668407521a7bdda/src/services/telemetry/TelemetryService.ts#L87-L101);
[`src/core/Cline.ts:1697`](https://github.com/trypear/PearAI-Roo-Code/blob/0b6df736c9b2799c38b9ee629668407521a7bdda/src/core/Cline.ts#L1697)),
so PearAI's analytics may hold the ids of tasks deleted locally, but nothing on disk says so.
Searched `share`, `sync`, `upload`, `remote`, `telemetry`, `posthog`, `SERVER_URL` and `trypear.ai`
in both clones' `core/`, `gui/src`, `extensions/vscode/src`, `src/` and `webview-ui/src`.

#### Scratch directory rows

None. Creator mode's project directory ([Scratch directories](#scratch-directories)) is the user's
new project and appears as the task's `workspace` and `creatorModeConfig.newProjectPath`
([Where the project path is recorded](#where-the-project-path-is-recorded)). Source-derived; not
observed on any real install: the task's `taskHistory` row ends ` | creator mode:
newProjectType=<type> newProjectPath=<path>` for such a task.

#### Deletion rows

The agent records no delete, so there is no deletion row; what each surface's delete removes and
what survives it is in [What survives a delete](#what-survives-a-delete).

#### Bad-line rows

Source-derived; not observed on any real install. Each line of `dev_data/chat.jsonl` and
`dev_data/tokens_generated.jsonl` that does not decode is its own `parser: unparseable line
(<reason>): <line>` row, the lines around it still read. In the output channel log, a write that
starts with neither `Settings:` nor `Completion:`, or a `Settings:` write with no `#` rule (cut off
mid-write), is `parser: unparseable record (<reason>): <record>` at its first line, and text before
the first separator is one such row too; the records around it are still read. The API history read
beside `ui_messages.json` reports its own bad records. A `state.vscdb` that cannot be read, or whose
`-wal`, `-shm` or `-journal` sidecar cannot, is one row
`parser: <file> could not be read (<reason>)`, `<file>` the name of the file that failed; one that
is not a SQLite database is `parser: state.vscdb is not a SQLite database`. A damaged `state.vscdb`,
or an extension row that is not JSON, is one `parser:` row. The chat surface's `sessions.json` index
is read through the Continue parser's index reader (`PearAIParser` keeps a `ContinueParser` for it),
which yields rows for what it loses, with the texts of
[Bad-line rows in `continue.md`](continue.md#bad-line-rows); an unreadable index leaves the sessions
without their `dateCreated` time.

#### Fixtures

Fixture `.pearai/sessions/9b1c2d3e-0000-4000-8000-000000000001.json`:

```json
{"history":[{"message":{"role":"user","content":"why does build.sh fail"},"contextItems":[{"content":"set -e\nmake all","name":"build.sh","description":"/home/alice/proj/build.sh","id":{"providerTitle":"file","itemId":"build.sh"}}]},{"message":{"role":"assistant","content":"The make target is missing."},"contextItems":[],"promptLogs":[{"completionOptions":{"model":"pearai_model"},"prompt":"<user>why does build.sh fail","completion":"The make target is missing."}]}],"perplexityHistory":[],"title":"why does build.sh fail","sessionId":"9b1c2d3e-0000-4000-8000-000000000001","workspaceDirectory":"/home/alice/proj"}
```

`.pearai/sessions/sessions.json`:

```json
[{"sessionId":"9b1c2d3e-0000-4000-8000-000000000001","title":"why does build.sh fail","dateCreated":"1759400000000","workspaceDirectory":"/home/alice/proj","integrationType":"continue"}]
```

Agent `tasks/1759400100000/api_conversation_history.json`:

```json
[{"role":"user","content":[{"type":"text","text":"<task>\nrun the tests\n</task>"}],"ts":1759400100000},{"role":"assistant","content":[{"type":"text","text":"<execute_command>\n<command>npm test</command>\n</execute_command>"}],"ts":1759400103000},{"role":"user","content":[{"type":"text","text":"[execute_command for 'npm test'] Result:"},{"type":"text","text":"12 passing"}],"ts":1759400109000}]
```

A denied call's result in the same file:

```json
{"role":"user","content":[{"type":"text","text":"[execute_command for 'rm -rf build'] Result:"},{"type":"text","text":"The user denied this operation."}],"ts":1759400125000}
```

Output channel log `<n>-PearAI.log`, one chat request and its reply (the `=` lines are 74
characters, the `#` line 44):

```text
==========================================================================
==========================================================================
Settings:
contextLength: 8192
model: pearai_model
maxTokens: 4096

############################################

<user>
add the target

==========================================================================
==========================================================================
Completion:

Added `all:` to the Makefile.

```

`dev_data/chat.jsonl` line:

```json
{"completionOptions":{"model":"pearai_model"},"prompt":"<user>\nadd the target\n\n","completion":"Added `all:` to the Makefile.","feedback":true,"sessionId":"9b1c2d3e-0000-4000-8000-000000000001"}
```

`dev_data/tokens_generated.jsonl` line:

```json
{"model":"pearai_model","provider":"pearai_server","promptTokens":120,"generatedTokens":9}
```

#### Source host rows

Neither extension connects a source-host account, caches a repository list or keeps
repository-bound remote conversations, so the parser writes no `source host:` row. The chat fork's
`issue` and `gitlab-mr` context providers call the GitHub and GitLab APIs with a token from
`config.json` each time an issue or merge request is added as context, and record no account
([Credentials](#credentials)). Searched `github`, `gitlab`, `bitbucket`, `octokit`, `oauth`,
`installation`, `repositor`, `clone` and `githubToken` in `core/`, `gui/src` and
`extensions/vscode/src` of pearai-submodule at
[51eceef](https://github.com/trypear/pearai-submodule/commit/51eceef62a90c29f712b3a9607ea70a9dca657e9)
and in `src/` and `webview-ui/src` of PearAI-Roo-Code at
[0b6df73](https://github.com/trypear/PearAI-Roo-Code/commit/0b6df736c9b2799c38b9ee629668407521a7bdda),
whose only OAuth flows are model providers' (Glama, OpenRouter, Requesty;
[`webview-ui/src/oauth/urls.ts:1-16`](https://github.com/trypear/PearAI-Roo-Code/blob/0b6df736c9b2799c38b9ee629668407521a7bdda/webview-ui/src/oauth/urls.ts#L1-L16)).

## Confidence

- High: editor directory names and remote server directory (product.json and the path resolver);
  the `~/.pearai` layout of the Continue fork; Roo fork storage, file names and renamed project
  files; SecretStorage key names; the chat session and index fields, the absence of tool calls in
  chat sessions, the Roo message shape and the result prefix (all source).
- Medium: Linux `~/.config/pearai/User` from the import flow; it follows from the lowercase path in
  source but whether a Linux build renames the product was not checked.
- Medium: the `<task>` wrapper on the first user message and the model string `pearai_model` in the
  fixture, which are illustrative.
- Not determined: which extension commits a given PearAI build shipped, so older installs may
  differ from the shapes here; the app tree pins no submodule revision, and the extension
  repositories took commits after the app's last push ([Source and evidence](#source-and-evidence)).
- Not determined: whether `console.log` of access tokens in `PearAICredentials`
  ([`core/pearaiServer/PearAICredentials.ts:18-21`](https://github.com/trypear/pearai-submodule/blob/51eceef62a90c29f712b3a9607ea70a9dca657e9/core/pearaiServer/PearAICredentials.ts#L18-L21))
  or `console.dir(data)` at login
  ([`src/extension.ts:110-111`](https://github.com/trypear/PearAI-Roo-Code/blob/0b6df736c9b2799c38b9ee629668407521a7bdda/src/extension.ts#L110-L111))
  reaches a file under `logs/`; if it does, extension host logs contain live tokens.
- Not determined: whether a malformed `sessions.json` index occurs on a real PearAI install.
