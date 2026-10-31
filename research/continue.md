# continue

Continue (catalog agent `continue`) is an AI coding assistant with three surfaces that share one
core and one state directory: the VS Code extension `continue.continue`, the JetBrains plugin (same
core via `binary/`) and the `cn` CLI (`extensions/cli`), which share
[`core/util/history.ts`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/util/history.ts)
([`extensions/cli/src/session.ts:12`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/session.ts#L12),[`50-51`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/session.ts#L50-L51)).
PearAI is a fork of it ([`pearai.md`](pearai.md)). The analyzer parses its sessions, `dev_data`
events, `cn.log`, the CLI's `permissions.yaml` and its prompt history, from source and fixtures
only.

## Source and evidence

Open source. [continuedev/continue](https://github.com/continuedev/continue) at
[`5522c6f44ca0ac3528b37244818fbfa39b5af470`](https://github.com/continuedev/continue/commit/5522c6f44ca0ac3528b37244818fbfa39b5af470),
Apache-2.0
([`LICENSE`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/LICENSE)).
All claims from source. Paths cited are relative to the repository root. The default branch
`main` has not moved since 2026-07-20; the README says the repository "is no longer actively
maintained and is read-only for all users" after a final 2.0.0 release of the VS
Code extension, the CLI and the JetBrains plugin
([`README.md:19`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/README.md#L19),
[`README.md:27-29`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/README.md#L27-L29)),
so this commit is the last source, and whether development continues elsewhere cannot be told
from the clone.

### Observed

- No real install.

### Source reviews

- 2026-10-21: continuedev/continue at
  [`5522c6f`](https://github.com/continuedev/continue/commit/5522c6f44ca0ac3528b37244818fbfa39b5af470)
  (2.0.0, the last commit): the Parser plan now reads `permissions.yaml`, the denial reasons and
  the session mode, `input_history.json`, the `chatFeedback`, `editInteraction` and `editOutcome`
  events and the `cn review` worktrees, all source-derived; it added which prompt log the
  `chatInteraction` and `chatFeedback` lines share ([dev_data event files](#dev_data-event-files))
  and the source-host review of [Source host rows](#source-host-rows).
- 2026-10-19: [continuedev/continue](https://github.com/continuedev/continue) at
  [`5522c6f`](https://github.com/continuedev/continue/commit/5522c6f44ca0ac3528b37244818fbfa39b5af470),
  the commit this document already cited. It added the CLI's `permissions.yaml` shape and writer,
  `input_history.json` and the selected models in `index/globalContext.json`
  ([Other stores](#other-stores)), the `cn review` worktrees of
  [Scratch directories](#scratch-directories), the clear-all delete of
  [What survives a delete](#what-survives-a-delete), and the index bad-line rows of
  [Bad-line rows](#bad-line-rows).

## Collection

### Per-user storage

One global dir on every OS, `~/.continue`, from `os.homedir()`; env `CONTINUE_GLOBAL_DIR` relocates
it (relative values resolve against cwd)
([`core/util/paths.ts:27-35`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/util/paths.ts#L27-L35);
[`extensions/cli/src/env.ts:10-11`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/env.ts#L10-L11)).
No XDG, Library or AppData paths.

Under `~/.continue`:

- `sessions/<sessionId>.json`, `sessions/sessions.json`
  ([`paths.ts:78-112`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/util/paths.ts#L78-L112);
  [`core/util/history.ts:112-120`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/util/history.ts#L112-L120));
  the CLI writes the same dir
  ([`extensions/cli/src/session.ts:50-67`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/session.ts#L50-L67)).
- `config.yaml`, `config.json`, `config.ts`, `tsconfig.json`, `package.json`,
  `types/core/index.d.ts`, `out/config.js`
  ([`paths.ts:114-177`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/util/paths.ts#L114-L177)),
  `.continuerc.json`
  ([`paths.ts:210-225`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/util/paths.ts#L210-L225)),
  `.continueignore`
  ([`paths.ts:58-67`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/util/paths.ts#L58-L67)),
  `sharedConfig.json`
  ([`paths.ts:98-100`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/util/paths.ts#L98-L100)),
  `.env`
  ([`paths.ts:377-378`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/util/paths.ts#L377-L378)),
  `.local`/`.staging` environment markers
  ([`paths.ts:462-468`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/util/paths.ts#L462-L468)).
- `.configs/<hostname>/config.json`, `config.js` for remote configs
  ([`paths.ts:342-375`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/util/paths.ts#L342-L375)).
- `index/index.sqlite`, `index/lancedb/`, `index/autocompleteCache.sqlite`, `index/docs.sqlite`,
  `index/globalContext.json` (selected models and MCP OAuth tokens,
  [index/globalContext.json](#indexglobalcontextjson))
  ([`paths.ts:86-95`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/util/paths.ts#L86-L95),[`326-340`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/util/paths.ts#L326-L340)).
- `dev_data/devdata.sqlite`, `dev_data/<schema>/<event>.jsonl` with events `chatInteraction`,
  `chatFeedback`, `toolUsage`, `tokensGenerated`, `autocomplete`, `editInteraction`, `quickEdit`,
  `editOutcome`, `nextEditOutcome`
  ([`paths.ts:228-249`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/util/paths.ts#L228-L249);
  [`packages/config-yaml/src/schemas/data/index.ts`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/packages/config-yaml/src/schemas/data/index.ts));
  legacy `dev_data/*.jsonl`
  ([`paths.ts:440-461`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/util/paths.ts#L440-L461)).
- `logs/core.log`, `logs/prompt.log`
  ([`paths.ts:385-399`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/util/paths.ts#L385-L399));
  CLI logs in `logs/` too
  ([`extensions/cli/src/util/logger.ts:17`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/util/logger.ts#L17)).
- `.utils/.chromium-browser-snapshots`, `.utils/esbuild`, `.utils/repo_map.txt`
  ([`paths.ts:46-56`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/util/paths.ts#L46-L56),[`433-438`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/util/paths.ts#L433-L438)),
  `.diffs/`
  ([`paths.ts:470-477`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/util/paths.ts#L470-L477)),
  `.migrations/<id>` markers
  ([`paths.ts:293-310`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/util/paths.ts#L293-L310)),
  `node_modules/`.
- Blocks folders shared by IDE and CLI: `rules/`, `prompts/`, `agents/`, `mcpServers/`, `skills/`,
  plus any `<name>/` read by `getGlobalFolderWithName`
  ([`paths.ts:401-406`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/util/paths.ts#L401-L406);
  [`core/config/loadLocalAssistants.ts:108-121`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/config/loadLocalAssistants.ts#L108-L121);
  [`core/context/mcp/json/loadJsonMcpConfigs.ts:43`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/context/mcp/json/loadJsonMcpConfigs.ts#L43);
  [`extensions/cli/src/util/loadMarkdownSkills.ts:98`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/util/loadMarkdownSkills.ts#L98);
  [`extensions/cli/src/systemMessage.ts:93`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/systemMessage.ts#L93)).
- CLI only: `auth.json`, `input_history.json` (the TUI's prompt history,
  [input_history.json](#input_historyjson)), `permissions.yaml` (standing tool grants,
  [permissions.yaml](#permissionsyaml)), `.onboarding_complete`
  ([`extensions/cli/src/auth/authEnv.ts:5`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/auth/authEnv.ts#L5);
  [`util/inputHistory.ts:6`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/util/inputHistory.ts#L6);
  [`permissions/permissionsYamlLoader.ts:12`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/permissions/permissionsYamlLoader.ts#L12);
  [`onboarding.ts:108`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/onboarding.ts#L108)).

#### Scratch directories

`cn review` makes one detached git worktree per review agent at
`<system temp directory>/cn-review-<epoch ms>-<n>`, runs a headless agent in it in the `auto`
permission mode, and removes it in a `finally` block with `git worktree remove --force`
([`extensions/cli/src/commands/review/worktree.ts:15-20`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/commands/review/worktree.ts#L15-L20),
[`99-115`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/commands/review/worktree.ts#L99-L115);
[`extensions/cli/src/commands/review/reviewWorker.ts:116-129`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/commands/review/reviewWorker.ts#L116-L129);
[`extensions/cli/src/commands/review.ts:341`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/commands/review.ts#L341),
[`389-392`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/commands/review.ts#L389-L392)).
The worker changes into the worktree before it starts its services or adds the first message, and a
CLI session takes `process.cwd()` as its `workspaceDirectory` when it is created
([`extensions/cli/src/session.ts:92-106`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/session.ts#L92-L106)),
so the review session's file and `sessions.json` entry name the temp path, and `discover_projects`
lists it as a project. The system temp directory is outside the home on Linux and macOS and is
`AppData/Local/Temp` on Windows; no catalog line collects it, and the worktree is normally gone
before collection. Nothing in the path or the session marks it as the agent's own; the basename
`cn-review-<digits>-<digits>` is the only sign. The extension has no scratch directory: a terminal
command with no workspace runs in the home directory
([`core/tools/implementations/runTerminalCommand.ts:81-87`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/tools/implementations/runTerminalCommand.ts#L81-L87)).

#### Editor extensions

VS Code `globalStorage/continue.continue/`: `<key>.bin` AES-GCM blobs written by the extension's own
secret store, key material in VS Code `context.secrets`
([`extensions/vscode/src/stubs/SecretStorage.ts:7-23`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/vscode/src/stubs/SecretStorage.ts#L7-L23),[`41-58`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/vscode/src/stubs/SecretStorage.ts#L41-L58),[`100`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/vscode/src/stubs/SecretStorage.ts#L100)).

The extension's tool policies are not under `~/.continue`. Its webview keeps `toolSettings` (tool
name to `allowedWithPermission`, the default, `allowedWithoutPermission` or `disabled`),
`toolGroupSettings` (`include` or `exclude`), `ruleSettings` and `reasoningSettings` with
redux-persist (a library that saves the webview's state store) in the webview's `localStorage`,
under the key `persist:root`, one JSON string per slice
([`gui/src/redux/store.ts:36-60`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/gui/src/redux/store.ts#L36-L60),
[`92-99`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/gui/src/redux/store.ts#L92-L99);
[`gui/src/redux/slices/uiSlice.ts:27-34`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/gui/src/redux/slices/uiSlice.ts#L27-L34);
[`gui/src/util/migrateLocalStorage.ts:32-36`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/gui/src/util/migrateLocalStorage.ts#L32-L36)).
Where VS Code or a JetBrains IDE keeps a webview's local storage on disk is outside this source and
not determined; the catalog's `globalStorage/continue.continue` lines do not cover it.

### Credentials

- `~/.continue/auth.json` (CLI): `userId`, `userEmail`, `accessToken`, `refreshToken`, `expiresAt`,
  `organizationId`
  ([`extensions/cli/src/auth/workos.ts:28-40`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/auth/workos.ts#L28-L40)).
- `config.yaml` / `config.json` / `config.ts` / `.configs/*/config.js*` embed provider `apiKey`
  values; `.env` holds the `${{ secrets.X }}` inputs
  ([`paths.ts:377-383`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/util/paths.ts#L377-L383)).
- `index/globalContext.json` key `mcpOauthStorage[server].tokens` and `clientInformation`
  ([`core/util/GlobalContext.ts:50-53`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/util/GlobalContext.ts#L50-L53));
  it shares the `index/` dir with the embedding store.
- The same config files can embed source-host tokens as context provider parameters: the
  GitHub Issues provider (`issue`) takes `githubToken` and a `repos` list of `{owner, repo, type}`
  ([`core/context/providers/GitHubIssuesContextProvider.ts:10-16`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/context/providers/GitHubIssuesContextProvider.ts#L10-L16),
  [`29-33`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/context/providers/GitHubIssuesContextProvider.ts#L29-L33),
  [`84-89`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/context/providers/GitHubIssuesContextProvider.ts#L84-L89)),
  and the deprecated GitLab merge request provider (`gitlab-mr`) a `token` and a `domain`
  ([`core/context/providers/GitLabMergeRequestContextProvider.ts:73`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/context/providers/GitLabMergeRequestContextProvider.ts#L73),
  [`86-98`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/context/providers/GitLabMergeRequestContextProvider.ts#L86-L98)).
- `mcpServers/*.yaml|json` can embed headers and env.
- VS Code: hub session token in `globalStorage/continue.continue/*.bin`,
  decryptable only with the key in the editor's `state.vscdb`.
- No OS keychain use by the core.

### Exclusions

`.continue/index/lancedb`, `.continue/index/*.sqlite`, `.continue/.utils`,
`.continue/node_modules`, `.continue/out`, `.continue/types`,
`.continue/.diffs`, `.continue/.migrations`, `.continue/dev_data/devdata.sqlite`.
Keep `dev_data/**/*.jsonl` (prompts, tool usage), `logs/prompt.log`,
`index/globalContext.json`.

### Project-local files

`.continue/<subdir>/*.yaml|md` for `rules`, `prompts`, `agents`, `mcpServers`, `skills`, `models`,
`context`, `docs`, `data`
([`loadLocalAssistants.ts:22-23`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/config/loadLocalAssistants.ts#L22-L23),[`112-115`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/config/loadLocalAssistants.ts#L112-L115);
[`extensions/vscode/src/activation/activate.ts:42`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/vscode/src/activation/activate.ts#L42);
[`createNewAssistantFile.ts:55`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/config/createNewAssistantFile.ts#L55);
[`promptFiles/index.ts:4`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/promptFiles/index.ts#L4);
[`tools/definitions/createRuleBlock.ts:65`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/tools/definitions/createRuleBlock.ts#L65));
`.continuerc.json`
([`core/config/json/loadRcConfigs.ts:18`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/config/json/loadRcConfigs.ts#L18);
[`loadLocalAssistants.ts:18`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/config/loadLocalAssistants.ts#L18));
`.continueignore`
([`core/core.ts:1287`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/core.ts#L1287)).
No per-project database.

### Where the project path is recorded

`sessions/sessions.json[].workspaceDirectory` and `sessions/<id>.json.workspaceDirectory`
([`history.ts:105-120`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/util/history.ts#L105-L120),[`160-172`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/util/history.ts#L160-L172)).
Empty string for sessions created without a workspace. A `cn review` session names its temporary
worktree here ([Scratch directories](#scratch-directories)).

## Parsing

### Transcript stores

Under the global directory of [Per-user storage](#per-user-storage):

- `sessions/<sessionId>.json`: one session, pretty-printed JSON
  ([`paths.ts:102-103`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/util/paths.ts#L102-L103),
  [`history.ts:131-134`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/util/history.ts#L131-L134)).
- `sessions/sessions.json`: JSON array of session metadata
  ([`paths.ts:106-112`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/util/paths.ts#L106-L112)).
  The extension rewrites the whole array on every save
  ([`core/util/history.ts:168-181`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/util/history.ts#L168-L181)),
  so a crash or a partial copy can leave it cut.

The logs, `dev_data` events, prompt history, exports and settings beside them are in
[Other stores](#other-stores).

#### What survives a delete

The extension's delete (`history/delete`) removes one session file and its `sessions.json` entry
only
([`core/util/history.ts:60-75`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/util/history.ts#L60-L75)),
and its clear-all (`history/clear`) removes the whole `sessions/` directory, the index with it
([`core/util/history.ts:87-89`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/util/history.ts#L87-L89);
[`core/core.ts:331`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/core.ts#L331)).
The CLI has no delete command (not found in `extensions/cli/src`, grepped for
`historyManager.delete` and `deleteSession`); editing an earlier message rewrites the session with
the history cut before it
([`extensions/cli/src/ui/hooks/useChat.ts:762-777`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/ui/hooks/useChat.ts#L762-L777)).

Afterwards the CLI's prompt history `input_history.json` remains, with a
time per prompt but no session id or directory and at most 1000
entries; so do the `dev_data` lines that carry the session's
`sessionId`, `logs/cn.log` and any `/share` Markdown export. No command
deletes the remote copy; a deletion marker is not stated. The
analyzer reads all of these but the `/share` exports: the `chatInteraction`, `toolUsage`,
`editInteraction` and `editOutcome` event files and `cn.log` as primary records, and
`input_history.json` and the `chatFeedback` events as secondary sources, so a deleted session's
prompts and rated replies are shown again
([Past-use rows and secondary sources](#past-use-rows-and-secondary-sources); see
[Secondary sources](../analyzer/docs/output.md#secondary-sources)).

### Record schema

#### Session file

Session
([`core/index.d.ts`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/index.d.ts)).
`Session`
[`:279-290`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/index.d.ts#L279-L290):
`sessionId`, `title`, `workspaceDirectory`, `history: ChatHistoryItem[]`,
`mode?: "chat"|"agent"|"plan"|"background"`, `chatModelTitle?`, `usage?: SessionUsage`
(`promptTokens`, `completionTokens`, `totalCost`,
[`:275-277`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/index.d.ts#L275-L277),[`407-422`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/index.d.ts#L407-L422)).
Key order is forced
([`history.ts:115-129`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/util/history.ts#L115-L129)).

`ChatHistoryItem`
[`:534-545`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/index.d.ts#L534-L545):
`message: ChatMessage`, `contextItems: ContextItemWithId[]`
(`content, name, description, uri?{type,value}, id{providerTitle,itemId}`
[`:459-473`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/index.d.ts#L459-L473)),
`editorState?`, `promptLogs?: PromptLog[]` (`modelTitle, modelProvider, prompt, completion`,
[`:488-493`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/index.d.ts#L488-L493)),
`toolCallStates?: ToolCallState[]`, `reasoning?: {active, text, startAt: number (ms epoch), endAt?}`
([`:527-532`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/index.d.ts#L527-L532);
[`gui/src/redux/slices/sessionSlice.ts:561`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/gui/src/redux/slices/sessionSlice.ts#L561)),
`conversationSummary?`, `appliedRules?`.

`ChatMessage`
[`:440-445`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/index.d.ts#L440-L445)
by `role`
([`:335-340`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/index.d.ts#L335-L340)):
`user` `{content: string | {type:"text",text}|{type:"imageUrl",imageUrl:{url}}[]}`; `assistant`
`{content, toolCalls?: [{id?, type?:"function", function?:{name?, arguments?: string (JSON)}}], usage?}`;
`thinking` `{content, signature?, redactedThinking?, reasoning_details?}`; `system` `{content}`;
`tool` `{content, toolCallId}`. All may carry `metadata?`.

#### Tool call states and denials

`ToolCallState`
[`:516-525`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/index.d.ts#L516-L525):
`toolCallId`, `toolCall: {id, type:"function", function:{name, arguments}}`,
`status: "generating"|"generated"|"calling"| "errored"|"done"|"canceled"`, `parsedArgs`,
`processedArgs?`, `output?: ContextItem[]` (tool result text in `content`).

A denied or blocked call ends as a state whose output says why. In the CLI a refusal at the prompt
is `canceled` with the output text `Permission denied by user`, and a call an `exclude` rule or a
headless `ask` rule stops is `canceled` with `Command blocked by security policy`
([`extensions/cli/src/stream/streamChatResponse.helpers.ts:113-137`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/stream/streamChatResponse.helpers.ts#L113-L137),
[`519-547`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/stream/streamChatResponse.helpers.ts#L519-L547);
[`extensions/cli/src/services/ChatHistoryService.ts:297-327`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/services/ChatHistoryService.ts#L297-L327)).
In the extension a call whose policy is `disabled`, or whose terminal command the security check
refuses, is set to `errored` with one output item named `Security Policy Violation`
([`gui/src/redux/thunks/evaluateToolPolicies.ts:93-117`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/gui/src/redux/thunks/evaluateToolPolicies.ts#L93-L117)).

#### Session mode and CLI sessions

Session `mode` is the extension's mode at the last save, `chat`, `agent` or `plan` (`background` is
in the type but nothing sets it), written from the GUI state on each save
([`gui/src/redux/thunks/session.ts:250-257`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/gui/src/redux/thunks/session.ts#L250-L257);
[`core/index.d.ts:495`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/index.d.ts#L495)).
CLI session files carry no `mode` and no `chatModelTitle`, and their items no `promptLogs`: the
CLI's `Session` is built with `sessionId`, `title`, `workspaceDirectory`, `history` and `usage` only
([`extensions/cli/src/session.ts:92-106`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/session.ts#L92-L106);
[`extensions/cli/src/services/ChatHistoryService.ts:131-147`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/services/ChatHistoryService.ts#L131-L147);
not found in `extensions/cli/src` outside tests, grepped for `chatModelTitle` and `promptLogs`). The
CLI drops `system` messages before saving and copies user content into `editorState`
([`extensions/cli/src/session.ts:211-231`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/session.ts#L211-L231)).

#### sessions.json index

`sessions.json` entry (`BaseSessionMetadata`
[`:292-298`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/index.d.ts#L292-L298)):
`sessionId`, `title`, `dateCreated` (`String(Date.now())`, millisecond epoch as a string,
[`history.ts:171`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/util/history.ts#L171)),
`workspaceDirectory`, `messageCount?` (assistant messages only,
[`:154-156`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/util/history.ts#L154-L156)).

#### dev_data events

`dev_data/0.2.0/chatInteraction.jsonl`
([`schemas/data/chatInteraction/v0.2.0.ts`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/packages/config-yaml/src/schemas/data/chatInteraction/v0.2.0.ts),
[`index.ts:4-19`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/packages/config-yaml/src/schemas/data/chatInteraction/index.ts#L4-L19),
[`base.ts:3-11`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/packages/config-yaml/src/schemas/data/base.ts#L3-L11)):
`eventName`, `schema`, `timestamp` (ISO 8601,
[`log.ts:50`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/data/log.ts#L50)),
`userId`, `userAgent`, `selectedProfileId`, `prompt`, `completion`, `modelName`, `modelTitle`,
`modelProvider`, `sessionId`, `tools?`, `rules?`. `toolUsage.jsonl`
([`toolUsage/v0.2.0.ts:13-19`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/packages/config-yaml/src/schemas/data/toolUsage/v0.2.0.ts#L13-L19)):
`toolCallId`, `functionName`, `functionParams`, `toolCallArgs`, `accepted`, `succeeded`, `output`.
`chatFeedback` adds `feedback`.

#### prompt.log

`prompt.log`
([`core/llm/logFormatter.ts:43-75`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/llm/logFormatter.ts#L43-L75)):
`HH:MM:SS.s [Chat]` (UTC,
[`:29-41`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/llm/logFormatter.ts#L29-L41)),
`Options: {json}`, `Role: user`, text lines prefixed `| `, relative `+N.N` deltas, `Success`,
`PromptTokens: N`; overlapping interactions get a column prefix character from `" |&%#"`
([`:19`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/llm/logFormatter.ts#L19)).
No date, no session id.

### Joins and timestamps

`sessions.json[].sessionId` with `sessions/<sessionId>.json`;
`workspaceDirectory` is in both. Tool call with result:
`assistant.toolCalls[].id` = `toolCallStates[].toolCallId` = the next `tool`
message's `toolCallId`. `chatInteraction.sessionId` joins the session file;
`toolUsage.toolCallId` joins `toolCallStates[].toolCallId`. No git branch is
recorded.

No per-message timestamp exists in session files; the session's time is the `sessions.json`
`dateCreated` ([sessions.json index](#sessionsjson-index)).

### Other stores

#### prompt.log

`logs/prompt.log`: human-readable LLM interaction log
([`paths.ts:397-399`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/util/paths.ts#L397-L399)),
written only by the binary used by JetBrains
([`binary/src/index.ts:35-38`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/binary/src/index.ts#L35-L38)),
not by the VS Code extension. Its line format is in [Record schema](#record-schema), under
[prompt.log](#promptlog).

#### dev_data event files

`dev_data/0.2.0/<eventName>.jsonl`: one JSON object per line per event
([`paths.ts:240-249`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/util/paths.ts#L240-L249),
[`core/data/log.ts:20`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/data/log.ts#L20),[`73-97`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/data/log.ts#L73-L97)).
The events are those named in [Per-user storage](#per-user-storage)
([`packages/config-yaml/src/schemas/data/`](https://github.com/continuedev/continue/tree/5522c6f44ca0ac3528b37244818fbfa39b5af470/packages/config-yaml/src/schemas/data)).
Legacy `dev_data/0.1.0/*.jsonl` and pre-version `dev_data/*.jsonl`
([`paths.ts:441-460`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/util/paths.ts#L441-L460)).

`dev_data/0.2.0/editInteraction.jsonl`, `editOutcome.jsonl` and `chatFeedback.jsonl` are still
written locally at every level
([`core/data/log.ts:71-117`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/data/log.ts#L71-L117);
the 0.2.0 schema table,
[`packages/config-yaml/src/schemas/data/index.ts:117-128`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/packages/config-yaml/src/schemas/data/index.ts#L117-L128)).
`editInteraction` (`prompt`, `completion`, `modelTitle`, `filepath`, written by the VS Code edit
flow,
[`extensions/vscode/src/extension/VsCodeMessenger.ts:236`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/vscode/src/extension/VsCodeMessenger.ts#L236))
and `editOutcome` (`prompt`, `completion`, `accepted`, `previousCode`, `newCode`,
`previousCodeLines`, `newCodeLines`, `lineChange`, `filepath`) record edit requests and whether the
user kept the edit. `editOutcome` is written both when an edit of the VS Code edit flow is accepted
or rejected and when an agent-mode file edit is
([`extensions/vscode/src/extension/EditOutcomeTracker.ts:41-76`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/vscode/src/extension/EditOutcomeTracker.ts#L41-L76);
[`gui/src/util/editOutcomeLogger.ts:184-208`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/gui/src/util/editOutcomeLogger.ts#L184-L208)).
Whether an edit also reaches a session file is not determined, so both are marked primary.
`chatFeedback` (`prompt`, `completion`, `modelTitle`, `feedback`, `sessionId`) is one line per
prompt log of a reply the user rated with the Helpful or Unhelpful button
([`gui/src/components/FeedbackButtons.tsx:20-39`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/gui/src/components/FeedbackButtons.tsx#L20-L39)).
Its `prompt` and `completion` are the prompt log's, the same pair the `chatInteraction` line of that
reply was written with when the reply ended
([`gui/src/redux/thunks/streamNormalInput.ts:229-243`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/gui/src/redux/thunks/streamNormalInput.ts#L229-L243)),
so it repeats that line, at the time of the click, and adds the rating: secondary. In both,
`prompt` is the full prompt sent to the model, not the user's message. `quickEdit` has no 0.2.0
schema, so `logLocalData` throws and nothing is written for it at this commit
([`core/data/log.ts:78-84`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/data/log.ts#L78-L84)).

#### dev_data/devdata.sqlite

`dev_data/devdata.sqlite`, table `tokens_generated`
([`paths.ts:236-238`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/util/paths.ts#L236-L238),
[`core/data/devdataSqlite.ts:16-20`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/data/devdataSqlite.ts#L16-L20)).
Its shape is `tokens_generated(model, provider, tokens_prompt, tokens_generated, timestamp)`
([`devdataSqlite.ts:16-49`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/data/devdataSqlite.ts#L16-L49)).
Excluded ([Exclusions](#exclusions)).

#### Markdown exports

`<YYYYMMDDTHHMMSS>_session.md`: a Markdown export written by the `/share` slash command ("Export the
current chat session to markdown"), or by the history page's share button in VS Code, which asks for
a destination folder first
([`core/commands/slash/built-in-legacy/share.ts:7-16`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/commands/slash/built-in-legacy/share.ts#L7-L16);
[`extensions/vscode/src/commands.ts:383-412`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/vscode/src/commands.ts#L383-L412)).
The file goes to `~/.continue` unless a folder or `outputDir` is given, is named by the host-local
export time, and starts `### [Continue](https://continue.dev) session transcript` and
`Exported: <locale time>`, then one `#### _User_` or `#### _Assistant_` heading per message with the
text as a block quote; it has no session id
([`core/util/historyUtils.ts:41-65`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/util/historyUtils.ts#L41-L65),
[`67-105`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/util/historyUtils.ts#L67-L105)).
Nothing is uploaded: "share" means a local file.

#### cn.log

`logs/cn.log`: the `cn` CLI's log, kept by the winston logging library at level `info`, rotated at
10 MB with five files kept
([`extensions/cli/src/util/logger.ts:16-31`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/util/logger.ts#L16-L31),
[`102-115`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/util/logger.ts#L102-L115)).
The CLI depends on winston `^3.17.0`
([`extensions/cli/package.json:134`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/package.json#L134)),
whose `File` transport, without the `tailable` option the CLI does not set, writes each rotation to
`cn<N>.log` with `N` counting up from 1 in the same directory and deletes the oldest past `maxFiles`
(winston 3.17.0 npm tarball, sha256
`6dc0d9098a6e90ff760e00c54df25d090c7e090e424d935113ef878eb1b2cb84`,
`lib/winston/transports/file.js:651-663` `_getFile`, the snippet `` `${basename}${isRotation}${ext}`
``, and `:674-687` `_checkMaxFilesIncrementing`); so the files are `cn.log`, `cn1.log`, `cn2.log`
and so on.

Each line is `YYYY-MM-DD HH:mm:ss.SSS [<8 hex>] [<level>]: <message>` followed by the call's extra
fields as one JSON object
([`:74-95`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/util/logger.ts#L74-L95)).
The time is host-local with no offset, and the eight hex digits are a random id per process, not the
session id
([`:12-13`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/util/logger.ts#L12-L13)).

In `cn serve --id <agentId>` mode it holds the only local record of what the CLI posted to
Continue's agent API (the sync marker, [Sync, relay and origin](#sync-relay-and-origin-rows)):
`[metadata] POST https://api.continue.dev/agents/<agentId>/metadata` with
`{"agentId":"<agentId>","metadataKeys":[...]}`, then `[metadata] Request body` with
`{"metadata":{...}}` (the posted fields, `summary` text included), then
`[metadata] Success: <status> (<ms>ms) <url>` or an `Unexpected response`,
`Auth required (skipping)`, `Failed` or `Error` line
([`extensions/cli/src/util/metadata.ts:125-181`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/util/metadata.ts#L125-L181)).
The base URL is `CONTINUE_API_BASE` or `https://api.continue.dev/`
([`extensions/cli/src/env.ts:9`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/env.ts#L9)).

#### input_history.json

`input_history.json` (CLI): the TUI's prompt history, a JSON array of `{text, timestamp}` with
`timestamp` in epoch milliseconds, newest first, at most 1000 entries; a prompt typed again is moved
to the front, so only its latest time is kept
([`extensions/cli/src/util/inputHistory.ts:6-12`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/util/inputHistory.ts#L6-L12),
[`53-72`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/util/inputHistory.ts#L53-L72)).
Every prompt and slash command submitted in the TUI is added
([`extensions/cli/src/ui/UserInput.tsx:477`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/ui/UserInput.tsx#L477),
[`585`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/ui/UserInput.tsx#L585)).
It carries no session id or directory, so it joins a session's `user` items on text and time only.
It survives everything that removes a session: the extension's delete and clear-all, a CLI edit that
rewinds the history and a compaction. Secondary.

#### permissions.yaml

`permissions.yaml` (CLI): standing tool grants, three optional lists `allow`, `ask` and `exclude` of
`Tool` or `Tool(<argument pattern>)` entries such as `Bash(npm*)`
([`extensions/cli/src/permissions/permissionsYamlLoader.ts:15-19`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/permissions/permissionsYamlLoader.ts#L15-L19),
[`77-146`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/permissions/permissionsYamlLoader.ts#L77-L146)).
`cn` writes it with empty lists on first use and appends to `allow` on a "Continue + don't ask
again" answer, `Bash(<first word>*)` for a shell command and the tool name otherwise
([`152-179`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/permissions/permissionsYamlLoader.ts#L152-L179);
[`extensions/cli/src/permissions/policyWriter.ts:18-76`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/permissions/policyWriter.ts#L18-L76);
[`extensions/cli/src/ui/components/ToolPermissionSelector.tsx:37-55`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/ui/components/ToolPermissionSelector.tsx#L37-L55);
[`extensions/cli/src/ui/hooks/useChat.ts:789-823`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/ui/hooks/useChat.ts#L789-L823)).
The append also logs `Added policy rule to permissions.yaml: <rule>` at level `info` to
`logs/cn.log`
([`extensions/cli/src/permissions/policyWriter.ts:67`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/permissions/policyWriter.ts#L67)),
the only record of when the grant was made. The file holds no other key and no secret
([`permissionsYamlLoader.ts:77-117`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/permissions/permissionsYamlLoader.ts#L77-L117)).
Primary for permission state.

#### index/globalContext.json

`index/globalContext.json`: `selectedModelsByProfileId.<profile id>.<role>` (the extension's
selected model per role, `chat` among them), `cliSelectedModel` (the CLI's) and
`lastSelectedProfileForWorkspace` keyed by workspace
([`core/util/GlobalContext.ts:18-33`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/util/GlobalContext.ts#L18-L33);
[`extensions/cli/src/util/modelPersistence.ts:7-19`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/util/modelPersistence.ts#L7-L19)).
Standing state with no time but the file's. The file also holds MCP OAuth tokens
([Credentials](#credentials)) and is a flagged secret.

#### Editor output channel

Upstream Continue writes no editor output channel: no TypeScript file in this clone outside
`node_modules` calls `createOutputChannel`. Its PearAI fork does
([Output channel log in `pearai.md`](pearai.md#output-channel-log)).

### Format versions

`sessions.json` entries with `session_id` are an old format and are filtered
([`history.ts:34-36`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/util/history.ts#L34-L36));
`dev_data` schema `0.1.0` became `0.2.0`; no version field inside session files.

### Secrets

`promptLogs[].prompt` and `dev_data/*/chatInteraction.jsonl` `prompt` embed full context, possibly
`.env` contents. `dev_data` `userId` is the Continue hub `userToken`
([`log.ts:64-66`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/data/log.ts#L64-L66)).
Session files hold no API keys; the collector flags `config.yaml`, `config.json` and
`index/globalContext.json`. A `data:` block in `config.yaml` can carry an `apiKey` that the
extension sends as a bearer token to the block's `destination`
([`packages/config-yaml/src/schemas/data/index.ts:71-81`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/packages/config-yaml/src/schemas/data/index.ts#L71-L81),
[`core/data/log.ts:192-194`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/data/log.ts#L192-L194));
`config.yaml` is already flagged. The CLI still names `auth.json` for a Continue Hub (Continue's
hosted account and configuration service) sign-in
([`extensions/cli/src/auth/authEnv.ts:5`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/auth/authEnv.ts#L5)),
but at this commit Hub sign-in is a stub that never saves
([`extensions/cli/src/auth/workos.ts:1-7`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/auth/workos.ts#L1-L7),
[`86-88`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/auth/workos.ts#L86-L88));
an older `cn` may have left one, and `.continue/auth*.json` is a secret glob.

### Parser plan

Globs: `.continue/sessions/*.json` (`sessions.json` is read for
`dateCreated` and yields rows only for what it loses, see [Bad lines](#bad-line-rows)),
`.continue/dev_data/*/chatInteraction.jsonl`,
`.continue/dev_data/*/toolUsage.jsonl`, `.continue/dev_data/*/editInteraction.jsonl`,
`.continue/dev_data/*/editOutcome.jsonl`, `.continue/dev_data/*/chatFeedback.jsonl`,
`.continue/permissions.yaml`, `.continue/input_history.json`, and `.continue/logs/cn.log` with
its rotated `cn<N>.log` files ([Sync, relay and origin](#sync-relay-and-origin-rows)).

Rows: one per `history[]` item (user, assistant, thinking, tool), plus one
`tool_use` per `toolCalls[]`.

| Column | Source |
| --- | --- |
| timestamp_utc | `sessions.json` `dateCreated` (session level; message rows inherit); `reasoning.startAt` if present |
| session_id | `sessionId` |
| project_path | `workspaceDirectory` |
| git_branch | empty |
| turn_type | by `message.role`; `thinking` → thinking; `tool` → tool_result |
| model | `chatModelTitle` or `promptLogs[].modelTitle` |
| tool_name, tool_use_id | `toolCall.function.name`, `toolCallId` |
| text | text, `function.arguments`, output `content` |

#### Permission rows

Per-call decision rows, typed `system`:

- `toolUsage.jsonl` `accepted` yields `permission decision:
  accepted=<accepted>` after the record's `tool_result` row, with its
  `timestamp`, `toolCallId` and `functionName`; the result row keeps its
  `accepted=` and `succeeded=` text.
- A `ToolCallState` whose `status` is `canceled` yields `permission
  decision: canceled reason=user` when its output is `Permission denied by user`,
  `permission decision: canceled reason=policy` when it is `Command blocked by security
  policy`, and `permission decision: canceled` otherwise; an `errored` state with an output
  item named `Security Policy Violation` yields `permission decision: errored reason=policy`
  ([Tool call states and denials](#tool-call-states-and-denials)). Each carries the call's
  `toolCallId` and function name, follows the state's `tool_result` row when there is one, and
  has the session time.
- `editOutcome.jsonl` yields `permission decision: edit accepted=<accepted> file=<filepath>
  lines <previousCodeLines> -> <newCodeLines>` (`-` for a missing value) at the line's
  `timestamp`, with `modelTitle` as the model; the line names no session or tool call.
- The `Added policy rule to permissions.yaml: <rule>` line of `logs/cn.log` and its rotated files,
  written by a "Continue + don't ask again" answer, yields `permission decision: allow always
  rule=<rule> (from logs/cn.log, log line time, host-local)`, with an empty `session_id` and the
  log's host-local time read as UTC, as the other `cn.log` rows
  ([permissions.yaml](#permissionsyaml)).

Standing and session-level rows, typed `system`:

- Each rule of `permissions.yaml` yields `permission setting: <list>=<rule> (from
  .continue/permissions.yaml, file mtime)`, `<list>` being `allow`, `ask` or `exclude`, at the
  file's modification time, with `source_line` the rule's line and no session. The reader takes
  only the shape the CLI writes: top-level `allow`, `ask` and `exclude` keys, each empty (`[]`,
  `null`, `~` or nothing) or followed by `- <rule>` lines, the rule plain, single- or
  double-quoted, with comments and blank lines. Any other construct makes the whole file one
  `parser: unsupported YAML at line <N> (only allow, ask and exclude lists of strings are read)`
  row and no rule row, since a reader of that shape cannot tell what the rest grants.
- A session whose `mode` is set yields `permission mode: <mode>` after the `model:` row, at the
  session time with `source_line` 0
  ([Session mode and CLI sessions](#session-mode-and-cli-sessions)), since `plan` restricts the
  tools. One value per file, so there is no
  `permission mode: <old> -> <new>` row.

All of these are source-derived; not observed on any real install.

The CLI's permission mode (`normal`, `plan` or `auto`, cycled in the TUI or set by `--auto` and
`--readonly`, with `--allow`, `--ask` and `--exclude` overrides) is recorded nowhere on disk: the
switch is logged at `debug`, below the file's `info` level
([`extensions/cli/src/services/ToolPermissionService.ts:285-290`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/services/ToolPermissionService.ts#L285-L290);
[`extensions/cli/src/ui/UserInput.tsx:206-213`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/ui/UserInput.tsx#L206-L213);
[`extensions/cli/src/index.ts:232-239`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/index.ts#L232-L239)).
The extension's `toolSettings` live in the editor's webview storage, which the collector does not
gather ([Editor extensions](#editor-extensions)).

#### Model rows

Model rows, typed `system`: `model: <chatModelTitle>` right after the
session row, and `model: <old> -> <new>` ahead of a `history[]` item whose
`promptLogs[].modelTitle` differs from the model before it. When
`chatModelTitle` is missing (or lost after a cut in a truncated file), the
first `modelTitle` starts the session. Mid-session switching is not
researched: the session format has no switch record, so a change shows
only as a different `modelTitle` between items, and `chatModelTitle` is
one value per file with no record of when it was set.

Read: `chatModelTitle` and `promptLogs[].modelTitle`. CLI
sessions name no model at all ([Session mode and CLI sessions](#session-mode-and-cli-sessions)),
so they get no `model:` row; `globalContext.json` `cliSelectedModel` and
`selectedModelsByProfileId` are standing selections, not read
([index/globalContext.json](#indexglobalcontextjson)).

#### Past-use rows and secondary sources

Primary: `sessions/`, `chatInteraction`, `toolUsage`, `editInteraction`, `editOutcome` and
`cn.log`. `editInteraction` yields `edit interaction: file=<filepath>` (a `system` row with
`modelTitle` as the model), then the `prompt` as `user` and the `completion`, the file after the
edit, as `assistant`, at the line's `timestamp` with no session; `editOutcome` yields only the
decision row of [Permission rows](#permission-rows), since its prompt and completion repeat the
edit request.

Secondary, with `secondary()` true ([Other stores](#other-stores)):

- `input_history.json`: one `user` row per entry at its `timestamp`, with no session or project,
  `source_line` the entry's position in the array.
- `chatFeedback.jsonl`: `user` (`prompt`) and `assistant` (`completion`) rows like a
  `chatInteraction` line, then `chat feedback: feedback=<true|false|-> model=<modelTitle|->`, with
  the line's `sessionId`, the session's `workspaceDirectory` and the line's time.

Both pair with their twins at any distance in time (`secondary_tolerance` is ten years): a
session's rows carry its creation time, not the prompt's, so a history entry stamped at
submission is later by as long as the session ran, and a rating is stamped at the click, any time
after its `chatInteraction` line. A history entry pairs with a session `user` row or a
`chatInteraction` `user` row of the same text; it never pairs a `user` row with no
`timestamp_utc`, as a session the index lost has. A slash command the TUI kept, which no session
records as a message, is kept as a recovered row. All of these are source-derived; not observed on
any real install.

#### Sync, relay and origin rows

Sync: only the hidden `cn serve --id <agentId>` mode sends session content to Continue, and the only
local trace is in `logs/cn.log` ([cn.log](#cnlog)). `cn serve` is an HTTP front end (`/state`,
`/message`, `/permission`, `/exit`) that Continue's hosted agents drive; `--id` is described as
"Upload session snapshots to Continue-managed storage"
([`extensions/cli/src/index.ts:310-326`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/index.ts#L310-L326)).
With an id, after every agent turn and at exit, the CLI posts to `agents/<agentId>/metadata` the
last assistant message cut to 500 characters (`summary`), the `git diff main` line counts
(`additions`, `deletions`), token usage and `isComplete`/`hasChanges`
([`extensions/cli/src/commands/serve.ts:487-494`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/commands/serve.ts#L487-L494);
[`extensions/cli/src/util/exit.ts:97-135`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/util/exit.ts#L97-L135);
[`extensions/cli/src/util/metadata.ts:70-116`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/util/metadata.ts#L70-L116)),
and an unhandled error posts `status: "FAILED"` to `agents/<agentId>/status`
([`extensions/cli/src/index.ts:98-115`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/index.ts#L98-L115)).
The requests carry no credential at this commit
([`extensions/cli/src/util/apiClient.ts:58-83`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/util/apiClient.ts#L58-L83)).

The snapshot upload itself (the full state as JSON and the `git diff main` text, PUT every 30
seconds to presigned storage URLs from `agents/storage/presigned-url`,
[`extensions/cli/src/services/StorageSyncService.ts:221-264`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/services/StorageSyncService.ts#L221-L264),
[`359-414`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/services/StorageSyncService.ts#L359-L414))
never starts, because `serve` passes `accessToken: null` and the service then logs
`Storage sync requested but no Continue API key is available; skipping uploads.`
([`extensions/cli/src/commands/serve.ts:185-198`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/commands/serve.ts#L185-L198);
[`extensions/cli/src/services/StorageSyncService.ts:85-90`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/services/StorageSyncService.ts#L85-L90)).
The session file and `sessions.json` record no agent id: the `isRemote` and `remoteId` fields of the
CLI's session list exist only in memory, are always `false` and absent for local sessions, and
remote sessions are a stub returning none ("Hub integration removed",
[`extensions/cli/src/session.ts:21-26`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/session.ts#L21-L26),
[`441-446`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/session.ts#L441-L446)).

The parser reads
`logs/cn.log` and its rotated `cn<N>.log` files
([cn.log](#cnlog)) and yields one `system` row per
`[metadata] POST` line, `agent metadata posted: agent <agentId> keys
<metadataKeys joined by ,> <status> (from logs/cn.log, log line time,
host-local)`, where `<status>` is `Success`, `Unexpected response`,
`Auth required`, `Failed` or `Error` from the first later `[metadata]`
result line of the same process (the eight hex digits) for the same
URL, and `-` when there is none; and for a `Storage sync requested but
no Continue API key` line `storage upload requested, skipped (from
logs/cn.log, log line time, host-local)`, with the rotated file's name in
place of `cn.log`. Both rows have an empty `session_id`, since the log
names no session, and the line's own time read as if it were UTC, as the
aider parser reads its host-local headers. The `[metadata] Request body`
line is never put in a row. Neither row starts `session synced:`: the
posts are an agent's metadata, not a session's transcript, and name no
local session, so the parser does not set `records_sync`. Both rest on a fixture alone: no real
install has shown either line. A deleted session's `cn.log` lines survive, and no command deletes
the remote copy ([What survives a delete](#what-survives-a-delete)).

The other paths that leave the host carry no transcript and leave no per-session marker: `/share`
writes a local Markdown file ([Markdown exports](#markdown-exports)); `dev_data` events go to remote
destinations only when a `data:` block in `config.yaml` names an `http` or `https` `destination`
(opt-in, no default block), posted as `{name, data, schema, level, profileId}` with the prompt and
completion at level `all`, and nothing local records the post beyond a console error on failure
([`core/data/log.ts:104-117`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/data/log.ts#L104-L117),
[`184-236`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/data/log.ts#L184-L236));
the CLI's OpenTelemetry metrics (counts, cost, lines changed) export only when an
`OTEL_EXPORTER_OTLP_*` or `OTEL_METRICS_EXPORTER` variable is set
([`extensions/cli/src/telemetry/telemetryService.ts:69-84`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/telemetry/telemetryService.ts#L69-L84));
and the extension core has no PostHog or other product analytics client at this commit
(`allowAnonymousTelemetry` survives only as a shared-config flag,
[`core/config/sharedConfig.ts:48-55`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/config/sharedConfig.ts#L48-L55)).
Searched `share`, `sync`, `upload`, `remote`, `remoteId`, `storage`, `telemetry`, `posthog`,
`agents/`, `api.continue.dev` and `userToken` in `core/`, `gui/src`, `extensions/vscode/src` and
`extensions/cli/src`.

The CLI's remote TUI (`startRemoteTUIChat`, which polls a
`cn serve` `/state` endpoint and skips local persistence) has no caller at this commit
([`extensions/cli/src/ui/index.ts:128-131`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/ui/index.ts#L128-L131);
[`extensions/cli/src/services/ChatHistoryService.ts:66-70`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/services/ChatHistoryService.ts#L66-L70)),
and the extension's `ContinueServerClient` fetches configuration and an embeddings cache, not
sessions
([`core/continueServer/stubs/client.ts:33-40`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/continueServer/stubs/client.ts#L33-L40)).
No relay through a messaging platform: not found in `core/`, `extensions/cli/src` or
`extensions/vscode/src` (grepped for `telegram`, `slack` and `webhook`; the one hit is a
`slack-dark` syntax theme name).

#### Scratch directory rows

`cn review` sessions name a temp worktree `cn-review-<epoch ms>-<n>` as `workspaceDirectory`
([Scratch directories](#scratch-directories)). A session whose `workspaceDirectory` ends in a
component `cn-review-<digits>-<digits>` (with `/` or `\` separators) yields `scratch directory:
<workspaceDirectory> (cn review worktree)` after the session's other session-level rows, at the
session time with that directory as `project_path`, so the inventory lists it under
`scratch_directories` rather than `projects`. Source-derived; not observed on any real install.

#### Deletion rows

No deletion row: the source records no deletion marker. What each delete removes and what survives
it (`input_history.json`, `dev_data`,
`cn.log` and `/share` exports) is in [What survives a delete](#what-survives-a-delete).

#### Bad-line rows

`chatInteraction.jsonl`, `toolUsage.jsonl`, `editInteraction.jsonl`, `editOutcome.jsonl` and
`chatFeedback.jsonl` go through `iter_jsonl` and `error_rows`, a file that cannot be read giving one
whole-file row `parser: <file> could not be read (<reason>)`, `<file>` its name.
`input_history.json` is read as a
JSON array: an element cut mid-write is `parser: unparseable record <N> (<reason>): <the text from
that element to the end of the file>`, the entries before it kept; an element that is not an
object with a string `text` is `parser: unparseable record <N> (not an object with a text): <the
element as JSON>` and the walk goes on; `parser: not a JSON array`, `parser: array cut off after
record <N>` and `parser: unparseable data after the JSON array` describe the whole file, and an
unreadable file is `parser: unreadable (<reason>)`, as is an unreadable `permissions.yaml`, whose
other layouts are the one row of [Permission rows](#permission-rows). These are source-derived;
not observed on any real install. `sessions.json` is read by
`read_index` in `continue_dev.py` (the PearAI parser shares it), whose
errors the index's own artifact reports through `error_rows`, once,
with an empty `session_id` and time
([issue 170](https://github.com/seanthegeek/doubleagent/issues/170)): `parser: unreadable
session index: <reason>`, `parser: not a JSON array`, `parser:
unparseable record <N> (<reason>): <the text from that entry to the
end of the file>` for an entry cut mid-write (the entries before it
still time their sessions), `parser: array cut off after record <N>`,
`parser: unparseable record <N> (not an object with a sessionId):
<the entry as JSON>` (an entry keyed by the pre-1.0 `session_id` is
skipped without a row, as the extension filters it) and `parser:
unparseable data after the JSON array`. A session the index lost
keeps its rows with an empty `timestamp_utc`. These damaged-index layouts are not observed on any
real install, source-derived (continuedev/continue at
[`5522c6f`](https://github.com/continuedev/continue/commit/5522c6f44ca0ac3528b37244818fbfa39b5af470));
the index can be cut because the extension rewrites it whole on every save
([Transcript stores](#transcript-stores)).

#### Fixtures

Fixture `sessions/0f1e.json`:

```json
{"sessionId":"0f1e","title":"Fix bug","workspaceDirectory":"/home/u/proj","history":[
 {"message":{"role":"user","content":"rename foo"},"contextItems":[]},
 {"message":{"role":"assistant","content":"","toolCalls":[{"id":"call_1","type":"function","function":{"name":"edit_existing_file","arguments":"{\"filepath\":\"a.py\"}"}}]},"contextItems":[],
  "toolCallStates":[{"toolCallId":"call_1","toolCall":{"id":"call_1","type":"function","function":{"name":"edit_existing_file","arguments":"{\"filepath\":\"a.py\"}"}},"status":"done","parsedArgs":{"filepath":"a.py"},"output":[{"name":"Edit","description":"","content":"ok"}]}]},
 {"message":{"role":"tool","content":"ok","toolCallId":"call_1"},"contextItems":[]}],"chatModelTitle":"GPT-4o"}
```

`sessions.json`:

```json
[{"sessionId":"0f1e","title":"Fix bug","dateCreated":"1759400000000","workspaceDirectory":"/home/u/proj","messageCount":1}]
```

`dev_data/0.2.0/chatInteraction.jsonl`:

```json
{"eventName":"chatInteraction","schema":"0.2.0","timestamp":"2026-10-02T10:00:00.000Z","userId":"","userAgent":"vscode/1.1 (Continue/1.0)","selectedProfileId":"local","prompt":"rename foo","completion":"done","modelName":"gpt-4o","modelTitle":"GPT-4o","modelProvider":"openai","sessionId":"0f1e"}
```

`dev_data/0.2.0/chatFeedback.jsonl` and `editOutcome.jsonl`:

```json
{"eventName":"chatFeedback","schema":"0.2.0","timestamp":"2026-10-02T10:01:30.000Z","userId":"","userAgent":"vscode/1.1 (Continue/1.0)","selectedProfileId":"local","prompt":"rename foo","completion":"done","modelProvider":"openai","modelName":"GPT-4o","modelTitle":"GPT-4o","feedback":true,"sessionId":"0f1e"}
{"eventName":"editOutcome","schema":"0.2.0","timestamp":"2026-10-02T10:02:30.000Z","userId":"","userAgent":"vscode/1.1 (Continue/1.0)","selectedProfileId":"local","modelProvider":"openai","modelName":"GPT-4o","modelTitle":"GPT-4o","prompt":"add a docstring","completion":"def foo(): ...","accepted":false,"previousCode":"def foo(): pass","newCode":"def foo(): ...","previousCodeLines":1,"newCodeLines":2,"lineChange":1,"filepath":"/srv/proj/a.py"}
```

`permissions.yaml`, as the CLI rewrites it after a "don't ask again" answer, and
`input_history.json`:

```yaml
# Continue CLI Permissions Configuration
# This file is managed by the Continue CLI and should not be edited manually.
# Use the TUI to modify permissions interactively.

allow:
  - Read
  - Bash(npm*)
ask: []
exclude: []
```

```json
[{"text": "/model", "timestamp": 1790935260000}, {"text": "rename foo", "timestamp": 1790935201000}]
```

#### Source host rows

The agent records no source-host account. No sign-in to GitHub, GitLab or Bitbucket exists in the
extension or the CLI, no repository list is cached, and no conversation is bound to a remote
repository. The only source-host tokens are context provider parameters a user writes into
`config.yaml` or `config.json` by hand: `githubToken` with a configured `repos` list for the GitHub
Issues provider and `token` with `domain` for the GitLab merge request provider
([Credentials](#credentials)); they name no account login, the files are flagged secrets, and the
parser reads neither, since `config.yaml` needs a YAML reader. `cn checks` reads `GITHUB_TOKEN` from
the environment to look up the current branch's pull request and stores nothing
([`extensions/cli/src/commands/checks.ts:64-75`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/extensions/cli/src/commands/checks.ts#L64-L75)).
Grepped `github`, `gitlab`, `bitbucket`, `oauth`, `installation`, `repositor`, `pull request` and
`clone` in `core/`, `gui/src`, `extensions/vscode/src` and `extensions/cli/src` at
[`5522c6f`](https://github.com/continuedev/continue/commit/5522c6f44ca0ac3528b37244818fbfa39b5af470);
the `oauth` hits are MCP server OAuth ([Credentials](#credentials)).

## Confidence

- High: every `~/.continue` path, `auth.json` fields, `workspaceDirectory`, the `.bin` secret blobs;
  the session, `sessions.json` and `dev_data` schemas (typed source).
- Medium: how the `yaml` library's `YAML.stringify` lays out `permissions.yaml` (sequence indent,
  which rules it quotes); not checked against the library, so the reader takes indented and
  unindented items, plain and quoted.
- Medium: the list of project `.continue/<subdir>` names (the loader accepts any subdir passed by
  callers); the `prompt.log` format (from a doc comment; only the JetBrains binary writes it).
- Not determined: whether the JetBrains plugin writes anything outside `~/.continue` (its
  `binary/` uses the same
  [`core/util/paths.ts`](https://github.com/continuedev/continue/blob/5522c6f44ca0ac3528b37244818fbfa39b5af470/core/util/paths.ts)).
- Not determined: where VS Code or a JetBrains IDE keeps the extension webview's local storage on
  disk ([Editor extensions](#editor-extensions)).
- Not determined: whether an `editInteraction` or `editOutcome` edit also reaches a session file
  ([dev_data event files](#dev_data-event-files)).
- Not determined: whether a model switch in the middle of a session is recorded anywhere beyond a
  changed `modelTitle` between items ([Model rows](#model-rows)).
