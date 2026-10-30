# roo-code

Roo Code (catalog agent `roo-code`) is an open-source coding agent shipped as a VS Code extension
and a CLI (`roo`). It is a fork of Cline and keeps Cline's legacy task layout, so the record shapes
in [`cline.md`](cline.md) apply with the differences listed here; PearAI bundles an older fork of
it ([`pearai.md`](pearai.md)). The analyzer parses its tasks and CLI history, from source and
fixtures only.

## Source and evidence

Open source. [RooCodeInc/Roo-Code](https://github.com/RooCodeInc/Roo-Code) at
[`b867ec9145750d0ae1ff7f02d35406e9bf2a0b16`](https://github.com/RooCodeInc/Roo-Code/commit/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16),
Apache-2.0 (clone `scratchpad/repos/roo`). All claims from source. It applies to the VS Code
extension, id `RooVeterinaryInc.roo-cline`
([`src/package.json:2`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/package.json#L2),[`5`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/package.json#L5)),
and the CLI, which is `apps/cli` on top of `packages/vscode-shim`. Paths cited are relative to the
clone. The Roo Code Cloud client of earlier releases is cited at the tag `v3.53.0`,
[`44f6bba250235a98a21403df8b2ff64dfb5421c0`](https://github.com/RooCodeInc/Roo-Code/commit/44f6bba250235a98a21403df8b2ff64dfb5421c0),
the last release that shipped it.

### Observed

No real install.

### Source reviews

- 2026-10-21: [RooCodeInc/Roo-Code](https://github.com/RooCodeInc/Roo-Code) at
  [b867ec9](https://github.com/RooCodeInc/Roo-Code/commit/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16)
  (3.53.0 development tree) and at
  [44f6bba](https://github.com/RooCodeInc/Roo-Code/commit/44f6bba250235a98a21403df8b2ff64dfb5421c0)
  (tag v3.53.0, the last release with the Roo Code Cloud client): added the extension's global
  and workspace state, the Roo Code Cloud settings cache, retry queue and CLI token file of releases
  up to 3.53.0, the source-host statement, and the parser's permission, mode, sync-setting and
  retry-queue rows ([Other stores](#other-stores), [Parser plan](#parser-plan)).
- 2026-10-19: [RooCodeInc/Roo-Code](https://github.com/RooCodeInc/Roo-Code) HEAD is still
  [`b867ec9`](https://github.com/RooCodeInc/Roo-Code/commit/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16)
  (2026-05-15, "Remove roocode.com web app"), the commit this document cites, so nothing has
  drifted. Added the CLI's `--ephemeral` storage, the full `~/.roo/cli-settings.json` key list, the
  permission records and settings, the provider-profile model ids, the worktree root and the
  absence of any Roo Code Cloud client ([Per-user storage](#per-user-storage),
  [Record schema](#record-schema), [Parser plan](#parser-plan)).

## Collection

### Per-user storage

The layouts differ by surface: the VS Code extension, the CLI's VS Code shim, and the
shared `~/.roo`.

#### Scratch directories

The worktree dialog's suggested root, `~/.roo/worktrees/<project>-<suffix>`
([`src/core/webview/worktree/handlers.ts:224-236`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/core/webview/worktree/handlers.ts#L224-L236)),
is the only directory Roo Code chooses for itself as a working directory. It is under the catalog
line `roo-code|.roo` and in `EXCLUDES` (`.roo/worktrees`), so its contents are not collected and the
analyzer classes it as a scratch directory by its catalog rule.

#### VS Code extension

VS Code extension, globalStorage `rooveterinaryinc.roo-cline/` (same layout on every OS; root is
redirectable by the setting `roo-cline.customStoragePath`,
[`src/utils/storage.ts:14-48`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/utils/storage.ts#L14-L48),
[`src/package.json:334`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/package.json#L334)):

- `tasks/<taskId>/api_conversation_history.json`, `ui_messages.json`, `task_metadata.json`,
  `history_item.json`; `tasks/_index.json`
  ([`src/shared/globalFileNames.ts:1-9`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/shared/globalFileNames.ts#L1-L9);
  [`src/core/task-persistence/TaskHistoryStore.ts:24-25`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/core/task-persistence/TaskHistoryStore.ts#L24-L25),[`561-570`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/core/task-persistence/TaskHistoryStore.ts#L561-L570)).
- `tasks/<taskId>/checkpoints/.git` shadow repo per task, or
  `checkpoints/<sha256[:8] of workspace>/` per workspace
  ([`src/services/checkpoints/ShadowCheckpointService.ts:125`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/services/checkpoints/ShadowCheckpointService.ts#L125),[`432-447`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/services/checkpoints/ShadowCheckpointService.ts#L432-L447);
  [`RepoPerTaskCheckpointService.ts:10`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/services/checkpoints/RepoPerTaskCheckpointService.ts#L10)).
- `settings/mcp_settings.json`, `settings/custom_modes.yaml`
  ([`src/utils/storage.ts:63-68`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/utils/storage.ts#L63-L68);
  [`src/services/mcp/McpHub.ts:494`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/services/mcp/McpHub.ts#L494);
  [`src/core/config/CustomModesManager.ts:251`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/core/config/CustomModesManager.ts#L251);
  the old name `cline_mcp_settings.json` is migrated,
  [`src/utils/migrateSettings.ts:26`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/utils/migrateSettings.ts#L26)).
- `cache/<provider>_models.json`
  ([`src/api/providers/fetchers/modelCache.ts:39-46`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/api/providers/fetchers/modelCache.ts#L39-L46)).
- `roo-index-cache-<sha256>.json` per workspace, codebase-index file hashes
  ([`src/services/code-index/cache-manager.ts:24-26`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/services/code-index/cache-manager.ts#L24-L26)).
  Vectors go to Qdrant, not disk.

#### CLI

CLI (`roo`), `~/.vscode-mock/` from `$HOME` or `%USERPROFILE%`
([`packages/vscode-shim/src/utils/paths.ts:8-16`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/packages/vscode-shim/src/utils/paths.ts#L8-L16),[`65-82`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/packages/vscode-shim/src/utils/paths.ts#L65-L82)):

- `global-storage/`: same `tasks/` tree as above, plus `global-state.json` (the extension's global
  state, [Extension global and workspace state](#extension-global-and-workspace-state)),
  `configuration.json`, `secrets.json`
  ([`packages/vscode-shim/src/context/ExtensionContext.ts:102-111`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/packages/vscode-shim/src/context/ExtensionContext.ts#L102-L111);
  [`api/WorkspaceConfiguration.ts:87-88`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/packages/vscode-shim/src/api/WorkspaceConfiguration.ts#L87-L88);
  [`storage/SecretStorage.ts:47`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/packages/vscode-shim/src/storage/SecretStorage.ts#L47)).
- `workspace-storage/<hash>/workspace-state.json` (the extension's workspace state),
  `configuration.json`; hash is a 32-bit string hash of the workspace path, not reversible
  ([`paths.ts:24-32`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/packages/vscode-shim/src/utils/paths.ts#L24-L32),[`72-75`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/packages/vscode-shim/src/utils/paths.ts#L72-L75)).
- `logs/`
  ([`paths.ts:80-82`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/packages/vscode-shim/src/utils/paths.ts#L80-L82)).

#### `~/.roo`

`~/.roo/`
([`src/services/roo-config/index.ts:26-28`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/services/roo-config/index.ts#L26-L28);
[`apps/cli/src/lib/storage/config-dir.ts:5`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/apps/cli/src/lib/storage/config-dir.ts#L5)):
`rules/`, `rules-<mode>/`
([`src/core/webview/webviewMessageHandler.ts:1908`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/core/webview/webviewMessageHandler.ts#L1908)),
`commands/`
([`src/services/command/commands.ts:137`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/services/command/commands.ts#L137)),
`skills/`
([`src/services/skills/SkillsManager.ts:608`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/services/skills/SkillsManager.ts#L608)),
`worktrees/<project>-<suffix>`
([`src/core/webview/worktree/handlers.ts:229-230`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/core/webview/worktree/handlers.ts#L229-L230)),
`cli-history.json`
([`apps/cli/src/lib/storage/history.ts:21`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/apps/cli/src/lib/storage/history.ts#L21),
entries are raw prompts), `cli-settings.json`
([`settings.ts:9`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/apps/cli/src/lib/storage/settings.ts#L9);
`onboardingProviderChoice`, `mode`, `provider`, `model`, `reasoningEffort`,
`consecutiveMistakeLimit`, `requireApproval`, the older `dangerouslySkipPermissions` and `oneshot`,
[`apps/cli/src/types/types.ts:55-73`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/apps/cli/src/types/types.ts#L55-L73);
`requireApproval` is permission state, see [Permission rows](#permission-rows)), and in releases up
to 3.53.0 `cli-credentials.json`, the CLI's Roo Code Cloud token
([Credentials](#credentials)). `~/.agents/skills` is also read
([`roo-config/index.ts:53-55`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/services/roo-config/index.ts#L53-L55);
[`SkillsManager.ts:594`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/services/skills/SkillsManager.ts#L594)).

#### Global MCP servers

Global MCP server install dir is platform-branched: Windows `AppData/Roaming/Roo-Code/MCP`, macOS
`Documents/Cline/MCP` (sic), Linux `.local/share/Roo-Code/MCP`, fallback `~/.roo-code/mcp`
([`src/core/webview/ClineProvider.ts:1547-1562`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/core/webview/ClineProvider.ts#L1547-L1562)).
It holds cloned server code, not evidence of use. No XDG use elsewhere; no environment variable
relocates storage.

#### CLI ephemeral storage

`roo --ephemeral` keeps the whole CLI storage, tasks included, in
`<system temp directory>/roo-cli-<epoch ms>-<random>` and removes it on exit
([Transcripts in CLI ephemeral storage](#transcripts-in-cli-ephemeral-storage)). It is outside
every home, so no catalog line reaches it; a responder who finds a killed `roo --ephemeral`
process collects the temp directory by hand.

### Credentials

- Extension: provider profiles with API keys are stored in VS Code `context.secrets` under
  `roo_cline_config_api_config`
  ([`src/core/config/ProviderSettingsManager.ts:45`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/core/config/ProviderSettingsManager.ts#L45),[`657`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/core/config/ProviderSettingsManager.ts#L657));
  Codex OAuth credentials likewise
  ([`src/integrations/openai-codex/oauth.ts:451`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/integrations/openai-codex/oauth.ts#L451)).
  Those land in the editor's `state.vscdb`, collected by the vscode entries.
- CLI: the same keys serialised to `~/.vscode-mock/global-storage/secrets.json` (mode 0600,
  [`packages/vscode-shim/src/storage/SecretStorage.ts:10-47`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/packages/vscode-shim/src/storage/SecretStorage.ts#L10-L47)),
  flagged by the catalog's secret glob `.vscode-mock/global-storage/secrets.json`.
- Releases up to 3.53.0: the CLI's `roo auth login` saved a Roo Code Cloud token as
  `~/.roo/cli-credentials.json`, `{token, createdAt, userId?, orgId?}`, mode 0600
  ([`apps/cli/src/lib/storage/credentials.ts:6-28`](https://github.com/RooCodeInc/Roo-Code/blob/44f6bba250235a98a21403df8b2ff64dfb5421c0/apps/cli/src/lib/storage/credentials.ts#L6-L28),
  in `~/.roo` by
  [`config-dir.ts:5-9`](https://github.com/RooCodeInc/Roo-Code/blob/44f6bba250235a98a21403df8b2ff64dfb5421c0/apps/cli/src/lib/storage/config-dir.ts#L5-L9)),
  used as the API key of the vendor's `roo` provider
  ([`apps/cli/src/commands/cli/run.ts:172`](https://github.com/RooCodeInc/Roo-Code/blob/44f6bba250235a98a21403df8b2ff64dfb5421c0/apps/cli/src/commands/cli/run.ts#L172),
  [`184`](https://github.com/RooCodeInc/Roo-Code/blob/44f6bba250235a98a21403df8b2ff64dfb5421c0/apps/cli/src/commands/cli/run.ts#L184),
  [`258`](https://github.com/RooCodeInc/Roo-Code/blob/44f6bba250235a98a21403df8b2ff64dfb5421c0/apps/cli/src/commands/cli/run.ts#L258));
  flagged by the secret glob `.roo/cli-credentials.json`. The extension kept its Roo Code Cloud
  sign-in in SecretStorage under `clerk-auth-credentials` (with `-<base URL>` for a non-production
  sign-in service,
  [`packages/cloud/src/WebAuthService.ts:113-115`](https://github.com/RooCodeInc/Roo-Code/blob/44f6bba250235a98a21403df8b2ff64dfb5421c0/packages/cloud/src/WebAuthService.ts#L113-L115),
  [`215`](https://github.com/RooCodeInc/Roo-Code/blob/44f6bba250235a98a21403df8b2ff64dfb5421c0/packages/cloud/src/WebAuthService.ts#L215)).
- `settings/mcp_settings.json` and project `.roo/mcp.json` can embed `env`
  and headers. Both are flagged, the first (and its pre-migration name
  `cline_mcp_settings.json`) in the extension's `globalStorage` and in
  `.vscode-mock/global-storage/settings/`. No keychain use.

### Exclusions

`*/rooveterinaryinc.roo-cline/tasks/*/checkpoints`,
`*/rooveterinaryinc.roo-cline/checkpoints`, `*/rooveterinaryinc.roo-cline/cache`,
`.vscode-mock/global-storage/tasks/*/checkpoints`,
`.vscode-mock/global-storage/checkpoints`, `.roo/worktrees`,
`.local/share/Roo-Code/MCP`, `AppData/Roaming/Roo-Code/MCP`, `.roo-code/mcp`.

### Project-local files

`.roo/rules/`, `.roo/rules-<mode>/`, `.roo/commands/`, `.roo/mcp.json`, `.roo/skills/`
([`roo-config/index.ts:68-80`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/services/roo-config/index.ts#L68-L80);
[`webviewMessageHandler.ts:1901`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/core/webview/webviewMessageHandler.ts#L1901);
[`commands.ts:141`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/services/command/commands.ts#L141);
[`McpHub.ts:379`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/services/mcp/McpHub.ts#L379);
[`SkillsManager.ts:379`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/services/skills/SkillsManager.ts#L379));
`.roomodes`, `.roorules*`, `.rooignore`, `.rooprotected`, legacy `.clinerules*`
([`src/core/protect/RooProtectedController.ts:17-23`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/core/protect/RooProtectedController.ts#L17-L23);
[`src/core/ignore/RooIgnoreController.ts:41`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/core/ignore/RooIgnoreController.ts#L41);
[`src/core/prompts/sections/custom-instructions.ts:229`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/core/prompts/sections/custom-instructions.ts#L229));
`.agents/skills`; `AGENTS.md`. No per-project database.

### Where the project path is recorded

`tasks/_index.json[].workspace` and `tasks/<id>/history_item.json.workspace`
([`src/core/task-persistence/taskMetadata.ts:22`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/core/task-persistence/taskMetadata.ts#L22),[`37`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/core/task-persistence/taskMetadata.ts#L37),[`111`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/core/task-persistence/taskMetadata.ts#L111);
[`TaskHistoryStore.ts:146-149`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/core/task-persistence/TaskHistoryStore.ts#L146-L149)),
in both globalStorage and `~/.vscode-mock/global-storage`. `checkpoints/<hash>` is a truncated
sha256 and not reversible. A task run in a worktree records the worktree's path as its `workspace`,
so `~/.roo/worktrees/<project>-<suffix>` paths appear there.

## Parsing

### Transcript stores

Tasks are one directory per task, `tasks/<taskId>/`, with the files and the `tasks/_index.json`
index listed in [VS Code extension](#vs-code-extension), under the extension's globalStorage
`rooveterinaryinc.roo-cline/` in VS Code and under `~/.vscode-mock/global-storage/` for the CLI
([`apps/cli/src/lib/task-history/index.ts:8`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/apps/cli/src/lib/task-history/index.ts#L8);
[CLI](#cli)). `~/.roo` holds CLI config and global rules only
([`apps/cli/src/lib/storage/config-dir.ts:5`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/apps/cli/src/lib/storage/config-dir.ts#L5)).
The legacy transcript name `claude_messages.json` is read and then deleted on upgrade
([`src/core/task-persistence/apiMessages.ts:73-91`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/core/task-persistence/apiMessages.ts#L73-L91)).

#### What survives a delete

Deleting a task removes it and, by default, its subtasks from `tasks/_index.json`, deletes their
shadow checkpoint repositories or branches, and removes each task directory
([`src/core/webview/ClineProvider.ts:1737-1814`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/core/webview/ClineProvider.ts#L1737-L1814)).
The CLI's `~/.roo/cli-history.json` is not touched: it holds the newest 500 submitted lines with no
time, task id or workspace, and outlives a deleted task directory, a removed workspace and the
extension's task-history pruning, so it is what is left of a deleted CLI task. A `roo --ephemeral`
task is removed with its temporary directory when the CLI exits and survives only a CLI killed
before its cleanup ran. In releases up to 3.53.0 the messages of a task synced to Roo Code Cloud
are held by the service, and a message whose upload failed stays in the workspace state's retry
queue, which nothing in `src/` touches, so a task delete leaves it
([Roo Code Cloud state](#roo-code-cloud-state-releases-up-to-3530)); the analyzer writes one row
per queued request, naming the task, but not the message. Other second copies, project-local
traces and deletion markers are not stated. The analyzer reads `cli-history.json` as a secondary
source whose lines are kept only when no task has the same text (see
[Secondary sources](../analyzer/docs/output.md#secondary-sources)).

#### Transcripts in CLI ephemeral storage

`roo --ephemeral` ("Run without persisting state",
[`apps/cli/src/index.ts:53`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/apps/cli/src/index.ts#L53))
points the whole extension storage, tasks included, at a new
`<system temp directory>/roo-cli-<epoch ms>-<random>` directory
([`apps/cli/src/lib/storage/ephemeral.ts:5-10`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/apps/cli/src/lib/storage/ephemeral.ts#L5-L10),
[`apps/cli/src/agent/extension-host.ts:378-381`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/apps/cli/src/agent/extension-host.ts#L378-L381))
and removes it when the CLI exits
([`:598-601`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/apps/cli/src/agent/extension-host.ts#L598-L601)).
Such a task is outside the home, so no catalog line reaches it, and it survives only a CLI that was
killed before its cleanup ran ([What survives a delete](#what-survives-a-delete)).

### Record schema

#### Task files

`api_conversation_history.json` is the Anthropic `MessageParam[]` shape as in Cline
([`apiMessages.ts:12`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/core/task-persistence/apiMessages.ts#L12)),
extended with `ts` (ms), `isSummary`, `id`, `type:"reasoning"`, `summary`, `encrypted_content`,
`reasoning_details`, `reasoning_content`, `condenseId`, `condenseParent`, `truncationId`,
`truncationParent`, `isTruncationMarker`
([`apiMessages.ts:13-37`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/core/task-persistence/apiMessages.ts#L13-L37)).
Tool calls are mostly XML inside text blocks.

`ui_messages.json`
([`packages/types/src/message.ts:249-274`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/packages/types/src/message.ts#L249-L274)):
`ts, type, ask, say, text, images, partial, reasoning, conversationHistoryIndex, checkpoint{}, progressStatus, contextCondense{cost, prevContextTokens, newContextTokens, summary, condenseId}, contextTruncation, isProtected, apiProtocol, isAnswered`.
Asks
[`:27-40`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/packages/types/src/message.ts#L27-L40);
says
[`:144-174`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/packages/types/src/message.ts#L144-L174)
add `subtask_result`, `checkpoint_saved`, `condense_context`, `codebase_search_result`,
`api_req_retried`. `ClineSayTool`
([`packages/types/src/vscode-extension-host.ts:693-760`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/packages/types/src/vscode-extension-host.ts#L693-L760))
adds `appliedDiff`, `codebaseSearch`, `newTask`, `switchMode`, `finishTask`, `batchFiles[]`;
`ClineApiReqInfo`
([`:780-790`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/packages/types/src/vscode-extension-host.ts#L780-L790))
adds `apiProtocol`. No model name appears anywhere in Roo transcripts; `HistoryItem.apiConfigName`
is a profile name.

`HistoryItem`
([`packages/types/src/history.ts:7-29`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/packages/types/src/history.ts#L7-L29)):
`id, rootTaskId?, parentTaskId?, number, ts, task, tokensIn, tokensOut, totalCost, size?, workspace?, mode?, apiConfigName?, status?, delegatedToId?, childIds?, ...`.
`_index.json` is `{version:1, updatedAt, entries: HistoryItem[]}`
([`TaskHistoryStore.ts:14-18`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/core/task-persistence/TaskHistoryStore.ts#L14-L18)).
`task_metadata.json` holds only `files_in_context`
([`src/core/context-tracking/FileContextTracker.ts:117-135`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/core/context-tracking/FileContextTracker.ts#L117-L135)).

Timestamps are millisecond epochs. No git branch is recorded.

#### Permission records

Every approval goes through `ask`: the `ask` record is written first,
then `checkAutoApproval` decides `approve`, `deny`, `timeout` or `ask` from the settings
([`src/core/task/Task.ts:1313-1334`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/core/task/Task.ts#L1313-L1334)).
An approval of a `tool` ask, by the user or automatically, sets `isAnswered: true` on that ask
record
([`:1489-1502`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/core/task/Task.ts#L1489-L1502),
[`packages/types/src/message.ts:273`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/packages/types/src/message.ts#L273));
an automatic approval is not told apart from a click. A denial reaches the model as a `tool_result`
in `api_conversation_history.json` whose text is JSON:
`{"status":"denied","message":"The user denied this operation."}`,
`{"status":"denied","feedback":...}`, or for an approval with a typed note
`{"status":"approved","feedback":...}`
([`src/core/prompts/responses.ts:8-24`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/core/prompts/responses.ts#L8-L24)).
The approval JSON opens the call's result text, followed by two newlines and the tool's own output,
and a denial with images is a block list whose first text block is the JSON
([`src/core/assistant-message/presentAssistantMessage.ts:469-472`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/core/assistant-message/presentAssistantMessage.ts#L469-L472),
[`491-525`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/core/assistant-message/presentAssistantMessage.ts#L491-L525);
[`responses.ts:99-111`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/core/prompts/responses.ts#L99-L111)).
`ui_messages.json` keeps no mark of a denial: the ask stays without `isAnswered`, and a typed note
becomes a `say: "user_feedback"` record.
`HistoryItem.mode` is the task's mode slug (`code`, `architect`, `ask`, `debug` or a custom mode),
which decides the tool groups the task may use, and `apiConfigName` names the provider profile
([`packages/types/src/history.ts:21-22`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/packages/types/src/history.ts#L21-L22)).

### Joins and timestamps

Directory name = `HistoryItem.id`; workspace = `workspace` in
`history_item.json` or `_index.json`; `rootTaskId` and `parentTaskId` link
subtasks. `ClineMessage.conversationHistoryIndex` indexes into the API
history.

A `cli-history.json` entry joins a task on text only. The first prompt of a task becomes the task's
first `say: "text"` record with the same text
([`src/core/task/Task.ts:1908`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/core/task/Task.ts#L1908)),
and a later message or follow-up answer becomes a `say: "user_feedback"` record
([`Task.ts:2027-2029`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/core/task/Task.ts#L2027-L2029));
both are `user` rows of the task. The API history wraps the first prompt in `<user_message>` tags
([`Task.ts:1935`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/core/task/Task.ts#L1935)),
so a task that has only `api_conversation_history.json` left does not match on text. The file
outlives a deleted task directory, a removed workspace and the extension's task-history pruning,
since nothing but the 500-entry cap removes an entry; it does not record which task or workspace a
line went to.

### Other stores

No SQLite store of its own. The VS Code side keeps its global and workspace state, and its secrets
in SecretStorage, in the editor's `state.vscdb` files.

#### CLI input history

The CLI also keeps an input history, `~/.roo/cli-history.json`: the file name is joined to
`getConfigDir()`
([`apps/cli/src/lib/storage/history.ts:20-22`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/apps/cli/src/lib/storage/history.ts#L20-L22)),
which is `~/.roo` on every platform
([`apps/cli/src/lib/storage/config-dir.ts:5-9`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/apps/cli/src/lib/storage/config-dir.ts#L5-L9)).
The catalog line `roo-code|.roo` collects it. It is the history the interactive prompt steps through
with the arrow keys, not a transcript.

`cli-history.json` is `{version: 1, entries: string[]}`
([`history.ts:9-15`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/apps/cli/src/lib/storage/history.ts#L9-L15)),
written pretty-printed with tabs and trimmed to the newest `MAX_HISTORY_ENTRIES` (500) on every save
([`history.ts:6-7`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/apps/cli/src/lib/storage/history.ts#L6-L7),
[`64-77`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/apps/cli/src/lib/storage/history.ts#L64-L77)).
`addToHistory` appends the trimmed line, skipping an empty one and one equal to the last entry
([`history.ts:90-108`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/apps/cli/src/lib/storage/history.ts#L90-L108));
`loadHistory` keeps only non-blank strings
([`history.ts:44-45`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/apps/cli/src/lib/storage/history.ts#L44-L45)).
Entries are oldest first and carry no time, task id or workspace. The input box adds every submitted
line before acting on it
([`apps/cli/src/ui/components/autocomplete/AutocompleteInput.tsx:180-198`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/apps/cli/src/ui/components/autocomplete/AutocompleteInput.tsx#L180-L198);
[`apps/cli/src/ui/hooks/useInputHistory.ts:83-97`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/apps/cli/src/ui/hooks/useInputHistory.ts#L83-L97)),
so it holds the first prompt of a task, the answers to follow-up questions and later messages, and
CLI commands such as `/new` that the CLI handles itself and never sends to a task
([`apps/cli/src/ui/hooks/useTaskSubmit.ts:56-132`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/apps/cli/src/ui/hooks/useTaskSubmit.ts#L56-L132)).
A prompt given on the command line rather than typed into the input box does not pass through it.

#### Extension global and workspace state

The extension's settings live in its global state. In VS Code that is one `ItemTable` row of the
editor's `User/globalStorage/state.vscdb`, keyed by `<publisher>.<name>`
([microsoft/vscode `extensionStorage.ts:134-141`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/platform/extensionManagement/common/extensionStorage.ts#L134-L141),
[`159-176`](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/platform/extensionManagement/common/extensionStorage.ts#L159-L176)),
here `RooVeterinaryInc.roo-cline`
([`src/package.json:2`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/package.json#L2),[`5`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/package.json#L5)),
whose value is a flat JSON object. The CLI's shim writes the same object to
`~/.vscode-mock/global-storage/global-state.json`, pretty-printed
([`packages/vscode-shim/src/context/ExtensionContext.ts:104`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/packages/vscode-shim/src/context/ExtensionContext.ts#L104);
[`packages/vscode-shim/src/storage/Memento.ts:58-66`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/packages/vscode-shim/src/storage/Memento.ts#L58-L66)).
A settings change from the webview is written key by key with `contextProxy.setValue`, which stores
a global-state key with `globalState.update`
([`src/core/webview/webviewMessageHandler.ts:638-734`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/core/webview/webviewMessageHandler.ts#L638-L734);
[`src/core/config/ContextProxy.ts:331-337`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/core/config/ContextProxy.ts#L331-L337)).
`allowedCommands` and `deniedCommands` are also written to the VS Code settings
`roo-cline.allowedCommands` and `roo-cline.deniedCommands` (the user's `settings.json`); the CLI's
shim keeps those settings in the same global-state file under the prefixed names
([`packages/vscode-shim/src/api/WorkspaceConfiguration.ts:73-78`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/packages/vscode-shim/src/api/WorkspaceConfiguration.ts#L73-L78),
[`140-151`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/packages/vscode-shim/src/api/WorkspaceConfiguration.ts#L140-L151);
[`packages/vscode-shim/src/api/WorkspaceAPI.ts:117`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/packages/vscode-shim/src/api/WorkspaceAPI.ts#L117)).
A non-interactive CLI run sends `autoApprovalEnabled`, every `alwaysAllow*` switch but
`alwaysAllowFollowupQuestions` as `true` and `allowedCommands: ["*"]`, an interactive one
`autoApprovalEnabled: false`, and the run's `mode`, through the same path, so the file holds the
last run's values
([`apps/cli/src/agent/extension-host.ts:221-250`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/apps/cli/src/agent/extension-host.ts#L221-L250),
[`450-451`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/apps/cli/src/agent/extension-host.ts#L450-L451)).
The workspace state is the same kind of row in the editor's
`User/workspaceStorage/<hash>/state.vscdb`, and the CLI's
`~/.vscode-mock/workspace-storage/<hash>/workspace-state.json`
([`ExtensionContext.ts:102`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/packages/vscode-shim/src/context/ExtensionContext.ts#L102)).
The permission keys are listed in [Permission rows](#permission-rows).

#### Roo Code Cloud state (releases up to 3.53.0)

Roo Code Cloud, the vendor's hosted service for task sync and task sharing, was the
`packages/cloud` client through the tag v3.53.0 and is gone at the commit this document cites
([Sync, relay and origin rows](#sync-relay-and-origin-rows)). While the extension was signed in,
it sent every completed `ui_messages.json` record of a task as a `Task Message` event
`{type, properties: {taskId, message, ...}}`
([`src/core/task/Task.ts:1165-1174`](https://github.com/RooCodeInc/Roo-Code/blob/44f6bba250235a98a21403df8b2ff64dfb5421c0/src/core/task/Task.ts#L1165-L1174);
[`packages/types/src/telemetry.ts:24`](https://github.com/RooCodeInc/Roo-Code/blob/44f6bba250235a98a21403df8b2ff64dfb5421c0/packages/types/src/telemetry.ts#L24),
[`219-226`](https://github.com/RooCodeInc/Roo-Code/blob/44f6bba250235a98a21403df8b2ff64dfb5421c0/packages/types/src/telemetry.ts#L219-L226);
[`packages/cloud/src/CloudService.ts:424-426`](https://github.com/RooCodeInc/Roo-Code/blob/44f6bba250235a98a21403df8b2ff64dfb5421c0/packages/cloud/src/CloudService.ts#L424-L426)),
when task sync was on: the organization's `cloudSettings.recordTaskMessages` for a member of an
organization (default false), else the user's `settings.taskSyncEnabled` (default true)
([`packages/cloud/src/CloudSettingsService.ts:269-282`](https://github.com/RooCodeInc/Roo-Code/blob/44f6bba250235a98a21403df8b2ff64dfb5421c0/packages/cloud/src/CloudSettingsService.ts#L269-L282);
[`packages/cloud/src/TelemetryClient.ts:279-283`](https://github.com/RooCodeInc/Roo-Code/blob/44f6bba250235a98a21403df8b2ff64dfb5421c0/packages/cloud/src/TelemetryClient.ts#L279-L283)).
Which messages were sent was kept only in memory, so no file says which tasks were uploaded.
Sharing a task (`organization` or `public` visibility) asked the service for a link and copied it
to the clipboard; a task the service did not hold was first uploaded whole through
`/api/events/backfill`, and the link was not stored
([`packages/cloud/src/CloudShareService.ts:17-30`](https://github.com/RooCodeInc/Roo-Code/blob/44f6bba250235a98a21403df8b2ff64dfb5421c0/packages/cloud/src/CloudShareService.ts#L17-L30);
[`CloudService.ts:315-330`](https://github.com/RooCodeInc/Roo-Code/blob/44f6bba250235a98a21403df8b2ff64dfb5421c0/packages/cloud/src/CloudService.ts#L315-L330);
[`TelemetryClient.ts:195-240`](https://github.com/RooCodeInc/Roo-Code/blob/44f6bba250235a98a21403df8b2ff64dfb5421c0/packages/cloud/src/TelemetryClient.ts#L195-L240)).

Two things in the extension's state survive locally:

- The settings cache, global-state keys `organization-settings`
  (`{version, cloudSettings: {recordTaskMessages, enableTaskSharing, allowPublicTaskSharing,
  taskShareExpirationDays, allowMembersViewAllTasks, workspaceTaskVisibility,
  llmEnhancedFeaturesEnabled}, defaultSettings, allowList, ...}`) and `user-settings`
  (`{features, settings: {taskSyncEnabled, llmEnhancedFeaturesEnabled}, version}`), written after
  each fetch while signed in and removed at sign-out
  ([`CloudSettingsService.ts:25-26`](https://github.com/RooCodeInc/Roo-Code/blob/44f6bba250235a98a21403df8b2ff64dfb5421c0/packages/cloud/src/CloudSettingsService.ts#L25-L26),
  [`169-178`](https://github.com/RooCodeInc/Roo-Code/blob/44f6bba250235a98a21403df8b2ff64dfb5421c0/packages/cloud/src/CloudSettingsService.ts#L169-L178),
  [`284-291`](https://github.com/RooCodeInc/Roo-Code/blob/44f6bba250235a98a21403df8b2ff64dfb5421c0/packages/cloud/src/CloudSettingsService.ts#L284-L291);
  [`packages/types/src/cloud.ts:128-136`](https://github.com/RooCodeInc/Roo-Code/blob/44f6bba250235a98a21403df8b2ff64dfb5421c0/packages/types/src/cloud.ts#L128-L136),
  [`175-184`](https://github.com/RooCodeInc/Roo-Code/blob/44f6bba250235a98a21403df8b2ff64dfb5421c0/packages/types/src/cloud.ts#L175-L184)).
  Its presence means the extension was signed in when it was last written.
- The retry queue, workspace-state key `roo.retryQueue`, an array of `{id, url, options, timestamp,
  retryCount, type, operation?, lastError?}`, `type` one of `api-call`, `telemetry`, `settings`
  and `other`
  ([`packages/cloud/src/retry-queue/RetryQueue.ts:14`](https://github.com/RooCodeInc/Roo-Code/blob/44f6bba250235a98a21403df8b2ff64dfb5421c0/packages/cloud/src/retry-queue/RetryQueue.ts#L14),
  [`62-95`](https://github.com/RooCodeInc/Roo-Code/blob/44f6bba250235a98a21403df8b2ff64dfb5421c0/packages/cloud/src/retry-queue/RetryQueue.ts#L62-L95);
  [`retry-queue/types.ts:1-10`](https://github.com/RooCodeInc/Roo-Code/blob/44f6bba250235a98a21403df8b2ff64dfb5421c0/packages/cloud/src/retry-queue/types.ts#L1-L10)).
  An event that failed on a network error, a 5xx or a 429 is queued with its whole request:
  `options.body` is the JSON event, the task message included, and `options.headers` carries
  `Authorization: Bearer <session token>`
  ([`TelemetryClient.ts:102-153`](https://github.com/RooCodeInc/Roo-Code/blob/44f6bba250235a98a21403df8b2ff64dfb5421c0/packages/cloud/src/TelemetryClient.ts#L102-L153),
  [`157-192`](https://github.com/RooCodeInc/Roo-Code/blob/44f6bba250235a98a21403df8b2ff64dfb5421c0/packages/cloud/src/TelemetryClient.ts#L157-L192)).
  It retries without limit, keeps at most 100 requests (oldest dropped first), removes a request
  when it succeeds, and is cleared when the signed-in user changes or signs out
  ([`RetryQueue.ts:32-40`](https://github.com/RooCodeInc/Roo-Code/blob/44f6bba250235a98a21403df8b2ff64dfb5421c0/packages/cloud/src/retry-queue/RetryQueue.ts#L32-L40),
  [`343-360`](https://github.com/RooCodeInc/Roo-Code/blob/44f6bba250235a98a21403df8b2ff64dfb5421c0/packages/cloud/src/retry-queue/RetryQueue.ts#L343-L360)).

### Format versions

`claude_messages.json` became `api_conversation_history.json` (deleted on read); globalState
`taskHistory` became `history_item.json` plus `_index.json` with `version: 1`
([`TaskHistoryStore.ts:320-360`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/core/task-persistence/TaskHistoryStore.ts#L320-L360)).
`.roo-code` has no references in current source and is legacy only.

### Secrets

The secret `roo_cline_config_api_config` ([Credentials](#credentials)) holds the provider
profiles with their API keys, in the editor's `state.vscdb` or the CLI's `secrets.json`; its shape
is in [Model rows](#model-rows). Transcripts carry file contents and command output. In releases up
to 3.53.0 each `roo.retryQueue` request of the workspace state
([Roo Code Cloud state](#roo-code-cloud-state-releases-up-to-3530)) holds the Roo Code Cloud session
token in `options.headers.Authorization` and a task message in `options.body`; the extension's
state row and the CLI's `workspace-state.json` are collected unflagged, so that key is the one to
redact. An MCP server entry's `env` and `headers` can hold keys
([Credentials](#credentials)).

### Parser plan

Globs:
`<GS>/rooveterinaryinc.roo-cline/tasks/*/{api_conversation_history,ui_messages,history_item}.json`,
`<GS>/rooveterinaryinc.roo-cline/tasks/_index.json`,
`.vscode-mock/global-storage/tasks/**` (same files),
`.roo/cli-history.json`; for permission and Roo Code Cloud state also
`.vscode-mock/global-storage/global-state.json`,
`.vscode-mock/workspace-storage/*/workspace-state.json`, `.roo/cli-settings.json`,
`<GS>/rooveterinaryinc.roo-cline/settings/mcp_settings.json` (and the `.vscode-mock` one), a
project's `.roo/mcp.json`, and the `RooVeterinaryInc.roo-cline` row of an editor's
`User/globalStorage/state.vscdb` and `User/workspaceStorage/*/state.vscdb`, which the catalog
attributes to the editor.

Rows and columns follow the Cline legacy mapping, with `ApiMessage.ts`
available as a fallback timestamp, `condense_context` typed `system`,
`session_id` the directory name, `project_path` from `history_item.workspace`,
and `model` always empty.

#### Permission rows

The parser reads all of the permission state below; source-derived; not observed on any real
install.

- Per call, from [Permission records](#permission-records): a `tool` ask with `isAnswered: true`
  in `ui_messages.json` yields `permission decision: approved isAnswered=true (user or
  auto-approval)` after the ask's `tool_use` row, with its `tool_use_id` (the ask's `ts`) and
  `tool_name`. A `tool_result` in the API history whose text opens with the JSON `status` `denied`
  or `approved` yields `permission decision: <status>`, plus ` feedback=<feedback>` when that key
  is set, with the result's `tool_use_id` and the `tool_use` block's `name`. When the task has no
  `ui_messages.json` the row follows the `tool_result` row; beside one, the API history is read
  for these rows only, since `ui_messages.json` keeps no mark of a denial. A denied ask in
  `ui_messages.json` has no row of its own, and an automatic approval is not told apart from a
  click.
- Standing: the global settings `autoApprovalEnabled`, `alwaysAllowReadOnly`,
  `alwaysAllowReadOnlyOutsideWorkspace`, `alwaysAllowWrite`, `alwaysAllowWriteOutsideWorkspace`,
  `alwaysAllowWriteProtected`, `alwaysAllowMcp`, `alwaysAllowModeSwitch`, `alwaysAllowSubtasks`,
  `alwaysAllowExecute`, `alwaysAllowFollowupQuestions`, `allowedCommands`, `deniedCommands`,
  `commandTimeoutAllowlist`, `allowedMaxRequests` and `allowedMaxCost`
  ([`packages/types/src/global-settings.ts:97-118`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/packages/types/src/global-settings.ts#L97-L118)),
  kept in the extension's global state: the `state.vscdb` row of the extension in VS Code, and
  `~/.vscode-mock/global-storage/global-state.json` for the CLI
  ([`packages/vscode-shim/src/context/ExtensionContext.ts:104`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/packages/vscode-shim/src/context/ExtensionContext.ts#L104)).
  `roo-cline.allowedCommands` and `roo-cline.deniedCommands` are also VS Code settings in the user's
  `settings.json`
  ([`src/package.json:280`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/package.json#L280),
  [`292`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/package.json#L292)).
  Each MCP server in `settings/mcp_settings.json` or a project's `.roo/mcp.json` has an
  `alwaysAllow` list of tool names
  ([`src/services/mcp/McpHub.ts:70`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/services/mcp/McpHub.ts#L70)).
  Writes to Roo Code's own configuration files (`.roo/**`, `.roomodes`, `.rooignore`, `.roorules*`,
  `.clinerules*`, `.vscode/**`, `*.code-workspace`, `AGENTS.md`, `AGENT.md`) are never
  auto-approved; the list is fixed in code, and `.rooprotected` is in it only as a name reserved
  "for future use", not a file Roo Code reads
  ([`src/core/protect/RooProtectedController.ts:7-25`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/core/protect/RooProtectedController.ts#L7-L25)).
  The CLI runs every tool without asking unless `--require-approval` (`-a`,
  [`apps/cli/src/index.ts:36`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/apps/cli/src/index.ts#L36))
  or `requireApproval: true` (or the older `dangerouslySkipPermissions: false`) in
  `~/.roo/cli-settings.json` says otherwise
  ([`apps/cli/src/commands/cli/run.ts:125-128`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/apps/cli/src/commands/cli/run.ts#L125-L128)).
  The parser yields one `permission setting: <key>=<value> (from <file>, file mtime)` row per
  scalar or list entry, at the file's modification time with an empty `session_id`: the keys
  above and the current `mode`
  ([`global-settings.ts:192`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/packages/types/src/global-settings.ts#L192))
  from the extension's global state, `<file>` naming `<state.vscdb path> ItemTable
  RooVeterinaryInc.roo-cline` for the database (the lower-case key is read too), plus
  `roo-cline.allowedCommands` and `roo-cline.deniedCommands` from the CLI's `global-state.json`
  ([Extension global and workspace state](#extension-global-and-workspace-state));
  `requireApproval`, `dangerouslySkipPermissions` and `mode` from `.roo/cli-settings.json`; and
  `mcpServers.<name>.alwaysAllow=<tool>` per tool from `settings/mcp_settings.json` and a
  project's `.roo/mcp.json`, whose rows carry the project as `project_path` when the file came from
  a collected project. Nothing else of an MCP server entry is read. The copies of the command lists
  in the editor's own `settings.json` are not read: the global state holds the same lists. The
  fixed protected-file list is code, not a record.
- Mode: each task row from `history_item.json`, or from `tasks/_index.json` for a task whose
  directory is gone, is followed by `permission mode: <mode>` from `HistoryItem.mode`, at the
  task's `ts`.

#### Model rows

Roo Code records no model in any of these files (`apiConfigName` is a
profile name, not a model id), so the parser yields no `model:` rows.

The provider profiles are stored as one secret,
`roo_cline_config_api_config`,
`{currentApiConfigName, apiConfigs: {<name>: {id, apiProvider, <provider>ModelId or apiModelId, ...}}, modeApiConfigs}`
([`src/core/config/ProviderSettingsManager.ts:45-61`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/core/config/ProviderSettingsManager.ts#L45-L61)),
in the editor's `state.vscdb` or the CLI's `secrets.json`, so a task's `apiConfigName` names a
profile whose model id is known only as it is now, not as it was when the task ran. The CLI's
`~/.roo/cli-settings.json` holds the default `model` ([`~/.roo`](#roo)).
Neither is a per-task record; the parser yields no model row from them.

#### Past-use rows and secondary sources

The task files are the primary store. `cli-history.json` ([CLI input history](#cli-input-history))
is a secondary source ([issue 122](https://github.com/seanthegeek/doubleagent/issues/122)): one
`user` row per entry, with empty `timestamp_utc`, `session_id` and `project_path` because the file
records none, so the timeline's deduplication pairs it with a task's `user` row of the same text and
keeps it only when no task has the text (a prompt whose task directory was deleted, or a CLI command
such as `/new`). A Roo Code Cloud retry-queue request of releases up to 3.53.0 holds a copy of one
task message in its body ([Roo Code Cloud state](#roo-code-cloud-state-releases-up-to-3530)); the
parser writes only the request's metadata row ([Sync, relay and origin
rows](#sync-relay-and-origin-rows)), not the message, because the body sits beside the bearer token.

#### Sync, relay and origin rows

Sync: Roo Code at this commit records no sync state. Cline's enterprise prompt upload and its
manifest marker
([Sync, relay and origin in `cline.md`](cline.md#sync-relay-and-origin-rows)) live in Cline's later
SDK, which Roo Code never took: its tasks keep the forked legacy layout, and its `HistoryItem` has
no share, sync or remote field
([`packages/types/src/history.ts:7-29`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/packages/types/src/history.ts#L7-L29)).
At this commit (extension version 3.53.0,
[`src/package.json:6`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/package.json#L6))
neither the extension nor the CLI holds a client for Roo Code Cloud, the vendor's hosted service
that earlier releases used for task sync and share links: there is no cloud package under
`packages/`; `CloudService`, `@roo-code/cloud`, `shareTask`, `shareUrl`, `posthog` and
`TelemetryService` have no code match in `src/`, `packages/`, `apps/` or `webview-ui/`, and `cloud`
matches only provider names and the documentation site, which redirects its removed Cloud pages,
`task-sync` and `task-sharing` among them
([`apps/docs/docusaurus.config.ts:278-290`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/apps/docs/docusaurus.config.ts#L278-L290)).
What is left is a `// Cloud Sync Tracking` comment above the task's initial status
([`src/core/task/Task.ts:407-409`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/core/task/Task.ts#L407-L409)),
`shareTaskSuccess` in a comment listing unhandled webview message types
([`src/core/webview/webviewMessageHandler.ts:3221`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/core/webview/webviewMessageHandler.ts#L3221)),
and a CLI client for the vendor's tRPC (a TypeScript remote procedure call library) API that defines
only `auth.me` and has no caller
([`apps/cli/src/lib/sdk/client.ts:11-30`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/apps/cli/src/lib/sdk/client.ts#L11-L30)).
`PRIVACY.md` still describes PostHog (a product-analytics service) usage telemetry that excludes
code and prompts
([`PRIVACY.md:13`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/PRIVACY.md#L13)),
but no telemetry code is in the tree.

The releases that shipped Roo Code Cloud end with the tag v3.53.0, which still has
`packages/cloud`; its task sync and sharing are described in
[Roo Code Cloud state](#roo-code-cloud-state-releases-up-to-3530). No file records which tasks
were uploaded or shared, so the parser writes no `session synced:` row and does not set
`records_sync`. It writes what the client kept (source-derived; not observed on any real install):

- From the extension's global state (the `state.vscdb` row or the CLI's `global-state.json`):
  `sync setting: user-settings.settings.taskSyncEnabled=<value>`, `unset (defaults to true)` when
  the cache has no such key, and one
  `sync setting: organization-settings.cloudSettings.<key>=<value>` row each for
  `recordTaskMessages`, `enableTaskSharing`, `allowPublicTaskSharing`, `taskShareExpirationDays`,
  `allowMembersViewAllTasks` and `workspaceTaskVisibility` when set, each ending
  `(from <file>, Roo Code Cloud settings cache, file mtime)` at the file's modification time.
- From the workspace state (an editor's `User/workspaceStorage/<hash>/state.vscdb` row or the CLI's
  `workspace-state.json`), one row per `roo.retryQueue` request: `cloud upload queued: <event
  type, else operation, else -> task=<properties.taskId or -> type=<type> retries=<retryCount>
  url=<url>`, then ` lastError=<lastError>` when set, then `(Roo Code Cloud retry queue, from
  <file> roo.retryQueue, request timestamp)`, at the request's `timestamp` with the task as
  `session_id`: an upload that had not reached the service. The headers and the body are never
  written.

Relay and origin: the agent records none.

#### Scratch directory rows

A task run in the suggested worktree root ([Scratch directories](#scratch-directories)) has its
`workspace` classed by the analyzer's catalog rule; a user-chosen path elsewhere is a project. No
default directory for a task without a workspace was found (grepped `homedir`, `tmpdir` and
`worktrees` in `src/` and `apps/cli/src`).

#### Deletion rows

No deletion marker is known ([What survives a delete](#what-survives-a-delete)), so the parser
yields no deletion row.

#### Bad-line rows

Every file the parser reads is a JSON document or array. A `cli-history.json` cut mid-write yields
the entries before the cut and one `system` row for the cut, `parser: unparseable record <N>
(<reason>): <raw>` or `parser: array cut off after record N`, as the `roo-code` parser's docstring
states. The API history read beside `ui_messages.json` for decisions reports its own bad records the
same way. A `global-state.json`, `workspace-state.json`, `cli-settings.json`, `mcp_settings.json` or
`mcp.json` that is not a JSON object is one `parser: <name> is not a JSON object` row; a
`state.vscdb` that cannot be read, or whose extension row is not JSON, is one `parser:` row that
never echoes the value.

#### Fixtures

```json
{"_file":"tasks/_index.json","record":{"version":1,"updatedAt":1760000005000,"entries":[{"id":"8f1c2b7e-1","number":1,"ts":1760000000000,"task":"fix tests","tokensIn":1200,"tokensOut":80,"totalCost":0.0041,"workspace":"/home/u/proj","mode":"code","status":"completed"}]}}
{"_file":"api_conversation_history.json","record":{"role":"assistant","content":[{"type":"tool_use","id":"toolu_01A","name":"execute_command","input":{"command":"pytest -q"}}],"ts":1760000002000}}
{"_file":"api_conversation_history.json","record":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_01A","content":"3 passed","is_error":false}],"ts":1760000003000}}
{"_file":"api_conversation_history.json","record":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_01B","content":"{\"status\":\"denied\",\"feedback\":\"leave conftest alone\"}"},{"type":"tool_result","tool_use_id":"toolu_01C","content":"{\"status\":\"approved\",\"feedback\":\"ok\"}\n\n[pytest]"}],"ts":1760000005000}}
{"_file":"ui_messages.json","record":{"ts":1760000002000,"type":"ask","ask":"tool","text":"{\"tool\":\"appliedDiff\",\"path\":\"src/util.py\",\"diff\":\"-old\\n+new\"}","isAnswered":true}}
{"_file":"global-state.json","record":{"autoApprovalEnabled":true,"alwaysAllowReadOnly":true,"allowedCommands":["git status","npm test"],"deniedCommands":["rm -rf"],"mode":"architect","roo-cline.allowedCommands":["git status"],"user-settings":{"features":{},"settings":{"taskSyncEnabled":true},"version":3},"organization-settings":{"version":1,"cloudSettings":{"recordTaskMessages":true,"enableTaskSharing":false},"defaultSettings":{},"allowList":{"allowAll":true,"providers":{}}}}}
{"_file":"workspace-state.json","record":{"roo.retryQueue":[{"id":"1760000000500-abc123def","url":"https://app.roocode.com/api/events","options":{"method":"POST","body":"{\"type\":\"Task Message\",\"properties\":{\"taskId\":\"8f1c2b7e-1\",\"message\":{}}}","headers":{"Authorization":"Bearer <token>","Content-Type":"application/json"}},"timestamp":1760000000500,"retryCount":2,"type":"telemetry","lastError":"fetch failed"}]}}
{"_file":"settings/mcp_settings.json","record":{"mcpServers":{"github":{"command":"npx","env":{"GITHUB_TOKEN":"<token>"},"alwaysAllow":["list_issues","get_issue"]}}}}
{"_file":".roo/cli-settings.json","record":{"mode":"code","provider":"anthropic","requireApproval":true}}
```

#### Source host rows

The agent records no source-host integration. Grepped `github`, `gitlab`, `bitbucket`, `oauth`,
`installation`, `repositor`, `pull request`, `clone` and `octokit` in `src/`, `apps/cli/src`,
`packages/` and `webview-ui/src` at
[b867ec9](https://github.com/RooCodeInc/Roo-Code/commit/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16)
(the only OAuth flows sign in to model providers, OpenAI Codex and Qwen Code) and in
`packages/cloud/src` at
[44f6bba](https://github.com/RooCodeInc/Roo-Code/commit/44f6bba250235a98a21403df8b2ff64dfb5421c0)
(no match); the Roo Code Cloud integrations the changelog names, Slack and Linear
([`CHANGELOG.md:312`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/CHANGELOG.md#L312)),
have no client code in either tree.

## Confidence

- High: all globalStorage and `~/.vscode-mock` paths, `_index.json.workspace`, secrets locations;
  file names, layouts and type definitions (read from source).
- Medium: `.roo/worktrees` is only the suggested default offered to the user
  ([`handlers.ts:230`](https://github.com/RooCodeInc/Roo-Code/blob/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16/src/core/webview/worktree/handlers.ts#L230)),
  so it may be elsewhere.
- Medium: the `ItemTable` key of the extension's state row is the case-preserved
  `RooVeterinaryInc.roo-cline` from VS Code's source; no real `state.vscdb` was checked, so the
  parser reads the lower-case form too.
- Not determined: the `checkpoint` object keys; whether the CLI of releases up to 3.53.0 started
  the Roo Code Cloud client, and so whether its `workspace-state.json` can hold a retry queue.
