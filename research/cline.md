# cline

Cline (catalog agent `cline`) is an open-source coding agent with a VS Code extension
(`saoudrizwan.claude-dev`), a CLI and a closed-source JetBrains plugin, all writing through its
`sdk/` packages. It was researched together with Roo Code and Kilo Code: Roo Code forked Cline and
still shares its task layout ([`roo-code.md`](roo-code.md)), PearAI bundles a fork of Roo Code
([`pearai.md`](pearai.md)), and Kilo Code has since moved to an OpenCode-derived store
([`kilo-code.md`](kilo-code.md)). The analyzer parses Cline's legacy tasks, its SDK sessions and
their secondary stores, its permission settings and decisions and its Cline Cloud state, from
source and fixtures only.

## Source and evidence

Open source. [cline/cline](https://github.com/cline/cline) at
[`39ff2359f7e08231281539696e48a166ce49270c`](https://github.com/cline/cline/commit/39ff2359f7e08231281539696e48a166ce49270c),
Apache-2.0 (clone `scratchpad/repos/cline`). It applies to the VS Code extension
(`saoudrizwan.claude-dev`), the CLI and JetBrains, all through the `sdk/` packages. Everything below
is from source; the JetBrains plugin is closed
([`README.md:131`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/README.md#L131))
and only its shared data dir is known. Paths cited are relative to the clone.

### Observed

No real install.

### Source reviews

- 2026-10-21: cline/cline at
  [`41e75ef`](https://github.com/cline/cline/commit/41e75efc53e7abbcb76564b89e6de57f78659b33)
  (4.1.25), with the release tags from
  [v3.0.0](https://github.com/cline/cline/commit/7fa399c6af3995c1628f0240de894dedb84b9053) to
  [v4.0.0](https://github.com/cline/cline/commit/1e88a708bde949cf92808c1638d8fb869dc55e05) for the
  share-task question: the parser now reads the hook audit log, the rejected and blocked SDK calls,
  the standing permission settings, the Cline Cloud state and the import marker, the Parser plan
  says so, and [Sync, relay and origin rows](#sync-relay-and-origin-rows) records that no release
  shipped a share-task feature; the `prompt_submit` audit line and the suffix on a missing-callback
  rejection were added.
- 2026-10-19: [cline/cline](https://github.com/cline/cline) at [`41e75ef`](https://github.com/cline/cline/commit/41e75efc53e7abbcb76564b89e6de57f78659b33) against
  [`39ff235`](https://github.com/cline/cline/commit/39ff2359f7e08231281539696e48a166ce49270c). Added Cline Cloud and its local state, imported sessions, the permission
  records and settings, the hook audit log and the `.claude/skills` project directory
  ([Per-user storage](#per-user-storage), [Project-local files](#project-local-files),
  [Transcript stores](#transcript-stores), [Other stores](#other-stores),
  [Parser plan](#parser-plan)). Of the 78 files the document cites, 19 changed and none moved or
  disappeared; the files whose shapes the parser reads are unchanged.

## Collection

### Per-user storage

One layout on every OS; home is `$HOME`, else `%USERPROFILE%`, else `%HOMEDRIVE%%HOMEPATH%`, else
`os.homedir()`
([`sdk/packages/shared/src/storage/paths.ts:84-103`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/shared/src/storage/paths.ts#L84-L103)).

#### Scratch directories

`~/.cline/data/workspaces/chat` (sessions without a
project, and imported sessions with no recorded directory) and
`~/.cline/worktrees/` (CLI task worktrees) are the agent's own working
directories. Both are under the catalog line `cline|.cline` and both are
in `EXCLUDES`, so their contents are not collected, and the analyzer
classes them as scratch directories by its catalog rule. No temp root
outside the home was found (grepped `tmpdir` and `chat-workspace` in
`sdk/packages/shared/src/storage` and `apps/cli/src/utils`).

#### `~/.cline`

`~/.cline`
([`paths.ts:151-160`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/shared/src/storage/paths.ts#L151-L160),
env `CLINE_DIR`):

- `data/`
  ([`paths.ts:179-185`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/shared/src/storage/paths.ts#L179-L185),
  env `CLINE_DATA_DIR`), shared by CLI, VS Code and JetBrains
  ([`.clinerules/storage.md:3`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/.clinerules/storage.md#L3)):
  - `globalState.json` (extension state, with the permission settings of
    [Permission rows](#permission-rows) and the Cline Cloud keys of [Cline Cloud](#cline-cloud)),
    `secrets.json` (mode 0600), `workspaces/<hash>/workspaceState.json`
    ([`apps/vscode/src/shared/storage/storage-context.ts:108-143`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/vscode/src/shared/storage/storage-context.ts#L108-L143));
    standalone hosts default the workspace dir to `data/workspace` unless `WORKSPACE_STORAGE_DIR` is
    set
    ([`apps/vscode/src/standalone/vscode-context.ts:18-21`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/vscode/src/standalone/vscode-context.ts#L18-L21)).
  - `workspaces/chat/`: the real working directory for sessions started without a project
    ([`sdk/packages/shared/src/storage/chat-workspace-paths.ts:10-16`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/shared/src/storage/chat-workspace-paths.ts#L10-L16)).
  - `sessions/<id>/<id>.json`, `<id>.messages.json`, `<id>.compaction.json`,
    `sessions/sessions.index.json`
    ([`paths.ts:187-193`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/shared/src/storage/paths.ts#L187-L193),
    env `CLINE_SESSION_DATA_DIR`;
    [`sdk/packages/core/src/services/session-artifacts.ts:63-94`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/core/src/services/session-artifacts.ts#L63-L94);
    [`session/services/file-session-service.ts:63`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/core/src/session/services/file-session-service.ts#L63)).
  - `tasks/<taskId>/` legacy task dirs and `state/taskHistory.json`
    ([`apps/vscode/src/sdk/legacy-state-reader.ts:42-75`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/vscode/src/sdk/legacy-state-reader.ts#L42-L75)).
  - `db/sessions.db`, `db/session-search.db` (FTS copy of transcripts), `db/connectors.db`,
    `db/cron.db`, `db/tasks.db`
    ([`paths.ts:240-281`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/shared/src/storage/paths.ts#L240-L281),
    env `CLINE_DB_DATA_DIR`;
    [`services/storage/sqlite-session-store.ts:30-45`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/core/src/services/storage/sqlite-session-store.ts#L30-L45);
    [`session/search/session-history-search.ts:196`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/core/src/session/search/session-history-search.ts#L196)).
  - `teams/teams.db`
    ([`paths.ts:195-201`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/shared/src/storage/paths.ts#L195-L201);
    [`services/storage/sqlite-team-store.ts:210`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/core/src/services/storage/sqlite-team-store.ts#L210)).
  - `cloud-pending-starts/<pid>-<uuid>.json`, the ids of Cline Cloud sandboxes being started
    ([Cline Cloud](#cline-cloud)).
  - `settings/providers.json`, `global-settings.json` (permission settings,
    [Permission rows](#permission-rows)), `cline_mcp_settings.json`,
    `remote-environments.json`, `composio/<sha256>.json`
    ([`paths.ts:424-446`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/shared/src/storage/paths.ts#L424-L446);
    [`remote/remote-environments.ts:209`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/core/src/remote/remote-environments.ts#L209);
    [`extensions/composio/composio-tools-extension.ts:63`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/core/src/extensions/composio/composio-tools-extension.ts#L63)).
  - `connectors/settings.json`, `connectors/<name>/...`
    ([`paths.ts:203-238`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/shared/src/storage/paths.ts#L203-L238);
    [`apps/cli/src/connectors/base.ts:221`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/cli/src/connectors/base.ts#L221)).
  - `logs/hooks.jsonl` (hook audit, written by default and only appended to,
    [Hook audit log](#hook-audit-log); env `CLINE_HOOKS_LOG_PATH`), `logs/hub-daemon.log`,
    `logs/<name>.log`, `logs/connectors/<channel>/<key>.log`, `logs/code.log`
    ([`paths.ts:910-920`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/shared/src/storage/paths.ts#L910-L920);
    [`hooks/hook-file-hooks.ts:647`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/core/src/hooks/hook-file-hooks.ts#L647);
    [`hub/daemon/index.ts:120`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/core/src/hub/daemon/index.ts#L120);
    [`apps/cli/src/commands/doctor.ts:206`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/cli/src/commands/doctor.ts#L206)).
  - `cache/user_input_history.jsonl` (CLI prompt history with `ts`), `cache/feature-flags.json`
    ([`apps/cli/src/utils/input-history.ts:8`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/cli/src/utils/input-history.ts#L8),[`66`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/cli/src/utils/input-history.ts#L66);
    [`utils/feature-flags.ts:23`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/cli/src/utils/feature-flags.ts#L23)).
  - `checkpoint-scratch/<sha>/index`, `pathspec` (git index files, reaped after 14 days;
    [`sdk/packages/core/src/hooks/checkpoint-hooks.ts:19-48`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/core/src/hooks/checkpoint-hooks.ts#L19-L48)).
  - `agent-plugins/`
    ([`extensions/agent-plugin/loader.ts:1067`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/core/src/extensions/agent-plugin/loader.ts#L1067)),
    `locks/hub/production.json`
    ([`hub/discovery/workspace.ts:43`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/core/src/hub/discovery/workspace.ts#L43)),
    `diagnostics/`
    ([`apps/examples/desktop-app/sidecar/diagnostics.ts:134`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/examples/desktop-app/sidecar/diagnostics.ts#L134)).
- `rules/`, `workflows/`, `skills/<name>/SKILL.md`, `plugins/`, `schedules/`, `tasks/`,
  `cron/{reports,events}`, `worktrees/`
  ([`paths.ts:299-305`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/shared/src/storage/paths.ts#L299-L305),[`348-349`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/shared/src/storage/paths.ts#L348-L349),[`563-617`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/shared/src/storage/paths.ts#L563-L617);
  [`services/marketplace.ts:251`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/core/src/services/marketplace.ts#L251);
  [`apps/cli/src/utils/worktree.ts:20`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/cli/src/utils/worktree.ts#L20)).

#### Documents folder

`~/Documents/Cline/{Rules,Workflows,MCP,Hooks,Agents,Plugins}`
([`apps/vscode/src/core/storage/disk.ts:55-97`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/vscode/src/core/storage/disk.ts#L55-L97);
[`paths.ts:162-177`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/shared/src/storage/paths.ts#L162-L177)).
Documents is resolved via PowerShell `MyDocuments` on Windows and `xdg-user-dir DOCUMENTS` on Linux
([`documents-path.ts:6-40`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/vscode/src/core/storage/documents-path.ts#L6-L40)),
so it may sit under `%OneDrive%\Documents` or, on headless Linux, at `~/Cline/Rules`
([`paths.ts:538-575`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/shared/src/storage/paths.ts#L538-L575)).

#### Shared `~/.agents`

`~/.agents/skills`, `~/.agents/plugins`, `~/.agents/AGENTS.md`
([`paths.ts:509-513`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/shared/src/storage/paths.ts#L509-L513),[`517-518`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/shared/src/storage/paths.ts#L517-L518),[`630-632`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/shared/src/storage/paths.ts#L630-L632))
are shared cross-agent.

#### VS Code extension

VS Code extension `saoudrizwan.claude-dev` globalStorage, the root of the extension's legacy tasks
([`apps/vscode/src/hosts/vscode/vscode-to-file-migration.ts:25-32`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/vscode/src/hosts/vscode/vscode-to-file-migration.ts#L25-L32);
whether the extension still writes new tasks there rather than to the SDK store of
[Transcript stores](#transcript-stores) is not determined):
`tasks/<id>/{api_conversation_history,ui_messages,context_history,task_metadata,settings}.json`,
`state/taskHistory.json`, `checkpoints/`, `cache/remote_config_<org>.json`,
`cache/cline_recommended_models.json`
([`disk.ts:17-36`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/vscode/src/core/storage/disk.ts#L17-L36),[`51-52`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/vscode/src/core/storage/disk.ts#L51-L52),[`186-194`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/vscode/src/core/storage/disk.ts#L186-L194);
[`apps/vscode/src/utils/storage.ts:11-12`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/vscode/src/utils/storage.ts#L11-L12)).
MCP settings for the extension now live in `~/.cline/data/settings/`
([`apps/vscode/src/sdk/SdkController.ts:309-312`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/vscode/src/sdk/SdkController.ts#L309-L312)).

#### CLI binary

CLI binary: `npm install -g cline` platform package, `bin/cline`
([`apps/cli/DISTRIBUTION.md:52`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/cli/DISTRIBUTION.md#L52),[`240`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/cli/DISTRIBUTION.md#L240));
no home-relative binary dir. Hub daemon runs as `cline --cline-hub-daemon`
([`apps/cli/src/commands/doctor.test.ts:218`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/cli/src/commands/doctor.test.ts#L218)).

### Credentials

- `~/.cline/data/settings/providers.json`: `apiKey`, `auth.apiKey`, `auth.accessToken`,
  `auth.refreshToken` per provider
  ([`sdk/packages/core/src/services/storage/provider-settings-manager.ts:78-85`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/core/src/services/storage/provider-settings-manager.ts#L78-L85),[`275`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/core/src/services/storage/provider-settings-manager.ts#L275)).
- `~/.cline/data/secrets.json`: 46 keys incl. `clineApiKey`, `awsSecretKey`,
  `openai-codex-oauth-credentials`, `ocaRefreshToken`, `mcpOAuthSecrets`
  ([`apps/vscode/src/shared/storage/state-keys.ts:313-361`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/vscode/src/shared/storage/state-keys.ts#L313-L361)).
- `~/.cline/data/connectors/settings.json`, `db/connectors.db` (connector credentials,
  [`paths.ts:248-258`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/shared/src/storage/paths.ts#L248-L258)).
- `settings/cline_mcp_settings.json` and `settings/composio/*.json` can embed `headers`/`env` and
  connected-account ids; `cline_mcp_settings.json` is flagged under `~/.cline/data` and in the
  extension's `globalStorage/saoudrizwan.claude-dev/settings/`
  ([`apps/vscode/src/services/mcp/schemas.ts:42-143`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/vscode/src/services/mcp/schemas.ts#L42-L143)).
- `<globalStorage>/saoudrizwan.claude-dev/cache/remote_config_<organization id>.json`,
  the cached organisation remote config, holds the prompt upload bucket's
  `accessKeyId` and `secretAccessKey`, and the LG CNS webhook integration
  (LG CNS is the LG group's IT services company) writes
  `~/Documents/Cline/webhook_config.json` with a bearer `webhook_token`;
  the catalog's secret globs
  `*/User/globalStorage/saoudrizwan.claude-dev/cache/remote_config_*.json`
  and `Documents/Cline/webhook_config.json` flag them. Evidence in [Secrets](#secrets).
- VS Code extension: secrets migrated out of `context.secrets` into `secrets.json`
  ([`vscode-to-file-migration.ts:4-15`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/vscode/src/hosts/vscode/vscode-to-file-migration.ts#L4-L15)).
  No keychain use.

### Exclusions

`.cline/data/checkpoint-scratch`, `.cline/worktrees`, `.cline/data/workspaces/chat`
(agent working tree), `.cline/data/agent-plugins`, `.cline/plugins`,
`*/User/globalStorage/saoudrizwan.claude-dev/checkpoints`. Keep
`.cline/data/db/session-search.db*`: a searchable copy of every transcript, not a cache.

The extension's `cache/` directory is not excluded. At the commit in
[Source and evidence](#source-and-evidence) it holds only `remote_config_<organization id>.json`, a
credential ([Credentials](#credentials)), and `sync-queue.json`, the enterprise upload queue
([Enterprise upload queue](#enterprise-upload-queue)), which is evidence of what was uploaded;
neither is a cache in size or kind.
`cline_recommended_models.json` is still named in `GlobalFileNames` but nothing at this commit
writes it.

### Project-local files

`.clinerules` (file or dir) with `workflows/`, `hooks/`, `skills/`;
`.cline/{rules,skills,workflows,plugins,tasks,cron,<remote-config plugin>}`; `.clineignore`;
`AGENTS.md`; also reads `.cursorrules`, `.cursor/rules`, `.windsurfrules`, `.claude/skills` (listed
among the project skill directories from
[`41e75ef`](https://github.com/cline/cline/commit/41e75efc53e7abbcb76564b89e6de57f78659b33),
[`sdk/packages/shared/src/storage/paths.ts:42`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/sdk/packages/shared/src/storage/paths.ts#L42),
[`:472-477`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/sdk/packages/shared/src/storage/paths.ts#L472-L477)),
`.agents/skills`
([`disk.ts:17-36`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/vscode/src/core/storage/disk.ts#L17-L36);
[`paths.ts:309-314`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/shared/src/storage/paths.ts#L309-L314),[`357`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/shared/src/storage/paths.ts#L357),[`461-470`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/shared/src/storage/paths.ts#L461-L470),[`528-535`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/shared/src/storage/paths.ts#L528-L535),[`595-617`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/shared/src/storage/paths.ts#L595-L617);
[`sdk/packages/shared/src/remote-config/paths.ts:11-19`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/shared/src/remote-config/paths.ts#L11-L19);
[`apps/vscode/src/core/ignore/ClineIgnoreController.ts:41`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/vscode/src/core/ignore/ClineIgnoreController.ts#L41)).
No per-project database.

### Where the project path is recorded

- Legacy: `state/taskHistory.json[].cwdOnTaskInitialization`
  ([`apps/vscode/src/shared/HistoryItem.ts:13`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/vscode/src/shared/HistoryItem.ts#L13)),
  both under globalStorage and `~/.cline/data`.
- SDK: `sessions/<id>/<id>.json` keys `cwd`, `workspace_root`
  ([`sdk/packages/core/src/session/models/session-manifest.ts:6-28`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/core/src/session/models/session-manifest.ts#L6-L28));
  `db/sessions.db` table `sessions` columns `cwd`, `workspace_root`
  ([`sdk/packages/shared/src/db/sqlite-db.ts:188-273`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/shared/src/db/sqlite-db.ts#L188-L273)).
- Imported sessions: `metadata.importedFrom.sourcePath`, the path of the other agent's transcript
  the session was copied from
  ([`sdk/packages/core/src/services/session-import/service.ts:20-28`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/sdk/packages/core/src/services/session-import/service.ts#L20-L28)).
- Cline Cloud: `globalState.json` `cloudTaskTarget.repositoryId` and
  `branch`, a GitHub repository id rather than a local path; the
  repository URL of a Cloud task is not stored locally.

## Parsing

### Transcript stores

Legacy task layout, one directory per task. Root is VS Code `globalStorage/saoudrizwan.claude-dev/`
for the extension
([`apps/vscode/src/core/storage/disk.ts:190-194`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/vscode/src/core/storage/disk.ts#L190-L194))
or `~/.cline/data/` for CLI and JetBrains
([`apps/vscode/src/sdk/legacy-state-reader.ts:24-29`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/vscode/src/sdk/legacy-state-reader.ts#L24-L29));
env `CLINE_DATA_DIR` or `CLINE_DIR` overrides. Files
([`disk.ts:17-36`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/vscode/src/core/storage/disk.ts#L17-L36),
[`legacy-state-reader.ts:42-72`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/vscode/src/sdk/legacy-state-reader.ts#L42-L72)):
`tasks/<taskId>/api_conversation_history.json` (JSON array), `ui_messages.json` (JSON array),
`context_history.json`, `task_metadata.json`, `settings.json`; index `state/taskHistory.json` (JSON
array of `HistoryItem`); `globalState.json`, `secrets.json`, `settings/cline_mcp_settings.json`.
`taskId` is a `Date.now()` millisecond string (`HistoryItem.id`).

SDK session layout, current on all hosts: `~/.cline/data/sessions/<sessionId>/`
([`sdk/packages/shared/src/storage/paths.ts:179-192`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/shared/src/storage/paths.ts#L179-L192))
holding `<sessionId>.json` (manifest), `<sessionId>.messages.json`, `<sessionId>.compaction.json`
([`sdk/packages/core/src/services/session-artifacts.ts:75-94`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/core/src/services/session-artifacts.ts#L75-L94));
subagent transcripts in the same directory as `<agentId>.messages.json` or
`<agentId>__<teamTaskId>.messages.json`
([`:34-57`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/core/src/services/session-artifacts.ts#L34-L57),[`132-144`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/core/src/services/session-artifacts.ts#L132-L144)).
`sessionId` is `${Date.now()}_${nanoid(5)}`
([`session/services/persistence-service.ts:109`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/core/src/session/services/persistence-service.ts#L109)).
Fallback index `sessions/sessions.index.json`
([`session/services/file-session-service.ts:63`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/core/src/session/services/file-session-service.ts#L63)).
SQLite in `~/.cline/data/db/` ([Other stores](#other-stores)). Hook audit
`~/.cline/data/logs/hooks.jsonl` ([Hook audit log](#hook-audit-log)).

`<sessionId>.compaction.json` is the model-facing view after compaction,
`{version, updated_at, conversation_id?, source_message_count, source_prefix_hash?, source_last_message_key?, messages, system_prompt?}`
([`sdk/packages/core/src/session/models/session-compaction.ts:25-34`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/sdk/packages/core/src/session/models/session-compaction.ts#L25-L34));
the canonical history in `<sessionId>.messages.json` is left unchanged
([`session-manifest-store.ts:299`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/sdk/packages/core/src/session/stores/session-manifest-store.ts#L299)),
so the sidecar holds no turn the messages file lacks, only the summary that replaced the older
turns.

#### What survives a delete

A delete through the SDK hub removes the session's manifest, its messages file and its `sessions.db`
row and leaves no marker
([`session/services/persistence-service.ts:586-630`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/core/src/session/services/persistence-service.ts#L586-L630);
[Sync, relay and origin](#sync-relay-and-origin-rows)). The VS Code extension's delete of a legacy
task removes its entry from `state/taskHistory.json` and the whole task directory, and its delete of
a Cloud session asks the control plane to delete it and writes nothing locally
([`apps/vscode/src/sdk/legacy-state-reader.ts:160-177`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/apps/vscode/src/sdk/legacy-state-reader.ts#L160-L177),
[`apps/vscode/src/sdk/sdk-task-history.ts:662-667`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/apps/vscode/src/sdk/sdk-task-history.ts#L662-L667)).
None of them touches the per-prompt history file `cache/user_input_history.jsonl`, which keeps the
last 20 distinct prompts with no session, directory or model and only the last line's own time, the
hook audit log `logs/hooks.jsonl`, which keeps the tool calls, their inputs and outputs and the
final reply of sessions whose transcripts are gone, or the enterprise upload queue; an uploaded
prompt is never deleted from the organisation's bucket. `session-search.db` keeps a deleted
session's turns until the hub next runs, at its start and every five minutes
([`session-search.db`](#session-searchdb)), and a Cloud session deleted on the server drops out of
`globalState.json` at the next listing. A session the SDK imported from Claude Code, Codex or
OpenCode is a copy of that agent's transcript under a new id. The analyzer reads
`user_input_history.jsonl`, `session-search.db` and `logs/hooks.jsonl` as secondary sources, the
upload queue and `cloudSessionStatuses` for their own rows, and an imported session as any SDK
session with its source named on the start row (see
[Secondary sources](../analyzer/docs/output.md#secondary-sources)).

#### Imported sessions

The SDK can import sessions of other agents: `claude-code`, `codex` and `opencode`
([`sdk/packages/core/src/services/session-import/types.ts:3-8`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/sdk/packages/core/src/services/session-import/types.ts#L3-L8)).
Each imported session becomes an ordinary SDK session with a new id, `source` `desktop`, `metadata`
`title`, `git.branch` when the source recorded one, a history origin of mode `import`, and, once the
import has finished,
`metadata.importedFrom{tool, sourceSessionId, sourcePath, importedAt, sourceProvider, sourceModel}`
([`service.ts:20-28`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/sdk/packages/core/src/services/session-import/service.ts#L20-L28),
[`:302-341`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/sdk/packages/core/src/services/session-import/service.ts#L302-L341),
[`:349-366`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/sdk/packages/core/src/services/session-import/service.ts#L349-L366));
a session whose source recorded no working directory gets the chat workspace as its `cwd`
([`:277-283`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/sdk/packages/core/src/services/session-import/service.ts#L277-L283)).
At this commit the only caller is the example desktop app
([`apps/examples/desktop-app/sidecar/commands.ts:2509`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/apps/examples/desktop-app/sidecar/commands.ts#L2509)),
not the CLI or the VS Code extension. The import service was already present at the cited commit.

#### Cline Cloud

Cline Cloud is the vendor's hosted service that runs a task in a sandbox on its servers against a
GitHub repository; it was added after the cited commit. Its sessions have ids starting `ses-`
([`apps/vscode/src/shared/cloud/cloud-sessions.ts:102-104`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/apps/vscode/src/shared/cloud/cloud-sessions.ts#L102-L104)),
live in the vendor's control plane (the server that lists and manages them), and are merged into the
extension's task list from it; nothing of them is written to the local SDK store
([`apps/vscode/src/sdk/sdk-task-history.ts:47-58`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/apps/vscode/src/sdk/sdk-task-history.ts#L47-L58),
[`:654-667`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/apps/vscode/src/sdk/sdk-task-history.ts#L654-L667)).
The `HistoryItem` fields `executionTarget` (`local` or `cloud`), `cloudStatus`, `cloudRepoUrl`,
`cloudBranch` and `cloudUsageAvailable`
([`apps/vscode/src/shared/HistoryItem.ts:29-35`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/apps/vscode/src/shared/HistoryItem.ts#L29-L35))
are filled from that listing, not stored. What is stored locally:

- `~/.cline/data/globalState.json`
  ([`apps/vscode/src/shared/storage/storage-context.ts:104-110`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/apps/vscode/src/shared/storage/storage-context.ts#L104-L110))
  key `cloudSessionStatuses`, `{<ses- id>: {status, observedAt}}`, the last settled status seen for
  each Cloud session (`completed`, `failed`, `cancelled`, `idle`, `unknown` or `expired`;
  `provisioning` and `running` are never kept) with `observedAt` in epoch ms
  ([`state-keys.ts:101`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/apps/vscode/src/shared/storage/state-keys.ts#L101),
  [`cloud-sessions.ts:15-38`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/apps/vscode/src/shared/cloud/cloud-sessions.ts#L15-L38)).
  An id is dropped when the account's listing no longer has it, so a session deleted on the server
  leaves the file at the next listing
  ([`apps/vscode/src/sdk/sdk-cloud-session-coordinator.ts:310-325`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/apps/vscode/src/sdk/sdk-cloud-session-coordinator.ts#L310-L325),
  [`:411-417`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/apps/vscode/src/sdk/sdk-cloud-session-coordinator.ts#L411-L417)).
- The same file's `cloudTaskTarget`, `{target: "local"|"cloud", repositoryId?, branch?}`, the user's
  last choice of where new tasks run
  ([`state-keys.ts:99`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/apps/vscode/src/shared/storage/state-keys.ts#L99),
  [`cloud-sessions.ts:46-52`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/apps/vscode/src/shared/cloud/cloud-sessions.ts#L46-L52)),
  and the setting `cloudSessionsEnabled`, default `false`
  ([`state-keys.ts:284`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/apps/vscode/src/shared/storage/state-keys.ts#L284)),
  which applies only when the `ext-cloud-sessions` feature flag is on or `CLINE_CLOUD_SESSIONS`
  forces it
  ([`apps/vscode/src/services/cloud/cloudSessionsFeature.ts:5-32`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/apps/vscode/src/services/cloud/cloudSessionsFeature.ts#L5-L32)).
- `~/.cline/data/cloud-pending-starts/<pid>-<uuid>.json`, a JSON array of
  `{sessionId, account?, endpoint?}` for each sandbox the extension host has created and not yet
  sent its first prompt to, so a later host can delete it if this one exits first; the file is
  removed when the array empties
  ([`apps/vscode/src/sdk/cloud-pending-starts.ts:5-12`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/apps/vscode/src/sdk/cloud-pending-starts.ts#L5-L12),
  [`:33-44`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/apps/vscode/src/sdk/cloud-pending-starts.ts#L33-L44),
  [`:103-111`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/apps/vscode/src/sdk/cloud-pending-starts.ts#L103-L111);
  [`apps/vscode/src/sdk/SdkController.ts:730`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/apps/vscode/src/sdk/SdkController.ts#L730)).

### Record schema

#### Legacy task files

`api_conversation_history.json` is `Anthropic.MessageParam[]`
([`disk.ts:155`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/vscode/src/core/storage/disk.ts#L155)):
`{role: "user"|"assistant", content: string | [{type:"text",text} | {type:"tool_use",id,name,input} | {type:"tool_result",tool_use_id,content,is_error?} | {type:"thinking",thinking} | {type:"image",source:{type:"base64",media_type,data}}]}`
(block handling
[`apps/vscode/src/sdk/legacy-task-handling.ts:13-45`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/vscode/src/sdk/legacy-task-handling.ts#L13-L45)).
No timestamps. Legacy tool calls are mostly XML inside text blocks, not `tool_use`.

`ui_messages.json` is `ClineMessage[]`
([`apps/vscode/src/shared/ExtensionMessage.ts:178-208`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/vscode/src/shared/ExtensionMessage.ts#L178-L208)):
`ts` (ms, also the identity), `type: "ask"|"say"`, `ask?`, `say?`, `text?`, `reasoning?`, `images?`,
`files?`, `partial?`, `conversationHistoryIndex?`, `lastCheckpointHash?`,
`modelInfo?: {modelId, providerId, mode}`
([`shared/messages/metrics.ts`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/vscode/src/shared/messages/metrics.ts)).
Asks
[`:210-228`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/vscode/src/shared/ExtensionMessage.ts#L210-L228)
(followup, plan_mode_respond, act_mode_respond, command, command_output, completion_result, tool,
api_req_failed, resume_task, use_mcp_server, new_task, condense, ...); says
[`:230-267`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/vscode/src/shared/ExtensionMessage.ts#L230-L267)
(task, error, api_req_started, api_req_finished, text, reasoning, completion_result, user_feedback,
command, command_output, tool, browser_action*, mcp_server_request_started, mcp_server_response,
checkpoint_created, hook_status, subagent, compaction, ...). `text` is JSON for `say`/`ask` `"tool"`
(`ClineSayTool {tool, path, diff, content, regex, filePattern}`,
[`:269-294`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/vscode/src/shared/ExtensionMessage.ts#L269-L294)),
`"api_req_started"`
(`ClineApiReqInfo {request, tokensIn, tokensOut, cacheWrites, cacheReads, cost, cancelReason}`,
[`:370-379`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/vscode/src/shared/ExtensionMessage.ts#L370-L379))
and `"use_mcp_server"` (`{serverName, type, toolName, arguments, uri}`,
[`:346-352`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/vscode/src/shared/ExtensionMessage.ts#L346-L352)).

`HistoryItem`
([`apps/vscode/src/shared/HistoryItem.ts:1-26`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/vscode/src/shared/HistoryItem.ts#L1-L26)):
`id, ulid?, ts, task, tokensIn, tokensOut, cacheWrites?, cacheReads?, totalCost, size?, cwdOnTaskInitialization?, modelId?, apiProvider?, isLegacy?`.

`task_metadata.json`
([`core/context/context-tracking/ContextTrackerTypes.ts:28-32`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/vscode/src/core/context/context-tracking/ContextTrackerTypes.ts#L28-L32)):
`files_in_context[{path, record_state, record_source, cline_read_date, cline_edit_date}]`,
`model_usage[{ts, model_id, model_provider_id, mode}]`,
`environment_history[{ts, os_name, os_version, host_name, cline_version}]`.

#### SDK manifest

SDK manifest `<id>.json`
([`sdk/packages/core/src/session/models/session-manifest.ts:6-28`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/core/src/session/models/session-manifest.ts#L6-L28)):
`version:1, session_id, source, pid, started_at (ISO 8601), ended_at?, exit_code?, status (idle|running|pending|completed|failed|cancelled, shared/src/session/records.ts:1-15), interactive, provider, model, cwd, workspace_root, team_name?, enable_tools, enable_spawn, enable_teams, prompt?, metadata?{title,...}, messages_path?`.

#### Enterprise upload marker

The manifest's `metadata` object, also stored as the `sessions.db` column `metadata_json`
([`session-manifest.ts:25`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/core/src/session/models/session-manifest.ts#L25),
[`session/services/session-service.ts:167-170`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/core/src/session/services/session-service.ts#L167-L170)),
carries the enterprise prompt-upload marker. Remote config is the organisation settings document the
Cline account API serves to a signed-in member of an enterprise organisation, at
`/api/v1/users/me/remote-config`
([`sdk/packages/core/src/account/cline-account-service.ts:98-102`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/core/src/account/cline-account-service.ts#L98-L102)).
When its `enterpriseTelemetry.promptUploading` block is enabled with access keys for an S3 (Amazon's
object store), R2 (Cloudflare's S3-compatible store) or Azure Blob bucket
([`sdk/packages/shared/src/remote-config/schema.ts:148-174`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/shared/src/remote-config/schema.ts#L148-L174),
[`remote-config/blob-storage.ts:91-127`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/shared/src/remote-config/blob-storage.ts#L91-L127)),
the session's metadata gets the key `enterprise.blobUpload`
([`:56-57`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/shared/src/remote-config/blob-storage.ts#L56-L57)):
`{version: 1, storage: {adapterType: "s3"|"r2"|"azure", bucket, region?, endpoint?, accountId?}, userDistinctId?}`,
the target with the access keys stripped
([`:79-89`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/shared/src/remote-config/blob-storage.ts#L79-L89),
[`:129-160`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/shared/src/remote-config/blob-storage.ts#L129-L160)).

The CLI writes it with `updateSession` right after the session starts
([`apps/cli/src/connectors/session-runtime.ts:248-258`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/cli/src/connectors/session-runtime.ts#L248-L258),
[`apps/cli/src/runtime/run-zen.ts:110-119`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/cli/src/runtime/run-zen.ts#L110-L119),
[`apps/cli/src/utils/enterprise.ts:167-186`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/cli/src/utils/enterprise.ts#L167-L186)),
without `userDistinctId`; the VS Code host adds it, with `userDistinctId` set to the remote-config
token's subject, to the start input
([`apps/vscode/src/sdk/vscode-session-host.ts:150-153`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/vscode/src/sdk/vscode-session-host.ts#L150-L153),
[`sdk/packages/core/src/remote-config/integration.ts:74-108`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/core/src/remote-config/integration.ts#L74-L108)),
which the runtime merges into the manifest at start
([`local-runtime-host.ts:630-634`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/core/src/runtime/host/local-runtime-host.ts#L630-L634)).
A session started without such a remote config has no `enterprise.blobUpload` key. The CLI hands its
session store an uploader
([`apps/cli/src/session/session.ts:66`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/cli/src/session/session.ts#L66)),
and every time that store rewrites `<sessionId>.messages.json` it PUTs the same bytes to the object
`sessions/<userDistinctId or unknown>/<sessionId>/messages.json` in that bucket
([`session/stores/session-manifest-store.ts:238-248`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/core/src/session/stores/session-manifest-store.ts#L238-L248),
[`blob-storage.ts:367-414`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/shared/src/remote-config/blob-storage.ts#L367-L414));
a failed upload is logged at debug level only and leaves the key in place. `keyPrefix` is read back
from the stored metadata but not used in the object name.

#### SDK messages file

SDK `<id>.messages.json`
([`services/session-data.ts:334-359`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/core/src/services/session-data.ts#L334-L359),[`281-294`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/core/src/services/session-data.ts#L281-L294)):
`{version:1, updated_at (ISO), agent:"lead"|"subagent"|"teammate", sessionId, taskType?, origin{source, mode, sessionId, parentThreadId?, subagent?, version?, trigger?}, system_prompt?, messages: MessageWithMetadata[]}`.
Message
([`sdk/packages/shared/src/llms/messages.ts:130-165`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/shared/src/llms/messages.ts#L130-L165)):
`role: "user"|"assistant"`, `content: string|ContentBlock[]`, `id?`, `agent?`, `sessionId?`,
`metadata?` (keys seen at call sites: `displayOnly`, `displayRole`, `kind`, `userRunSpan`,
`checkpoint`), `modelInfo?{id, provider, family?}`,
`metrics?{inputTokens, outputTokens, cacheReadTokens, cacheWriteTokens, cost}`, `ts?` (ms epoch,
[`session-data.ts:196`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/core/src/services/session-data.ts#L196)).
Blocks
([`messages.ts:19-125`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/shared/src/llms/messages.ts#L19-L125)):
`text{text}`, `tool_use{id, call_id?, name, input}`,
`tool_result{tool_use_id, name, content, is_error?}`, `thinking{thinking, signature?}`,
`redacted_thinking{data}`, `image{data, mediaType}`, `file{content, path}`, `media`.

Timestamps are millisecond epochs except the SDK manifest and payload
(`started_at`, `updated_at` ISO). No git branch is recorded.

### Joins and timestamps

Legacy: directory name = `HistoryItem.id`; workspace = `cwdOnTaskInitialization` in
`state/taskHistory.json`; `ClineMessage.conversationHistoryIndex` indexes into
`api_conversation_history.json`; `tool_use.id` pairs with `tool_result.tool_use_id` within the array
(rare, since most tool traffic is XML text). SDK: `messages.json.sessionId` = manifest
`session_id` = `sessions.session_id`; workspace = manifest `cwd` or `workspace_root`; `tool_use.id`
pairs with `tool_result.tool_use_id`. `session-search.db` `session_search.session_id` =
`sessions.session_id` = the session directory name, and a message document's `ordinal` is the
message's 0-based position in the lead `<sessionId>.messages.json`
([`session-search.db`](#session-searchdb)). VS Code merges the legacy and SDK lists
([`apps/vscode/src/sdk/sdk-task-history.ts:413-424`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/vscode/src/sdk/sdk-task-history.ts#L413-L424)),
stamping `metadata.legacyTask`.

### Other stores

#### `sessions.db` and `teams.db`

`~/.cline/data/db/sessions.db`
([`paths.ts:240-249`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/shared/src/storage/paths.ts#L240-L249);
[`shared/src/db/sqlite-db.ts:188-273`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/shared/src/db/sqlite-db.ts#L188-L273)):
`sessions(session_id, source, pid, started_at, ended_at, exit_code, status, status_lock, interactive, provider, model, cwd, workspace_root, team_name, enable_*, parent_session_id, parent_agent_id, agent_id, conversation_id, is_subagent, prompt, metadata_json, transcript_path, hook_path, messages_path, updated_at)`,
`subagent_spawn_queue`, `schedules(prompt, system_prompt, ...)`, `schedule_executions`.
`db/session-search.db`: `indexed_sessions` plus FTS5
`session_search(session_id, document_id, ordinal, role, started_at, workspace_root, title, content)`
([`session/search/session-history-search.ts:123-153`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/core/src/session/search/session-history-search.ts#L123-L153),[`196`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/core/src/session/search/session-history-search.ts#L196)),
a full-text copy of every transcript, described in [`session-search.db`](#session-searchdb).
`teams/teams.db`
([`services/storage/sqlite-team-store.ts:210-330`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/core/src/services/storage/sqlite-team-store.ts#L210-L330),[`431-445`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/core/src/services/storage/sqlite-team-store.ts#L431-L445)):
`team_events(payload_json)`, `team_tasks`, `team_runs`, `team_mailbox(data_json)`,
`team_mission_log(data_json)`.

#### `session-search.db`

It sits beside `sessions.db` in the directory `resolveDbDataDir()` names, `~/.cline/data/db` unless
`CLINE_DB_DATA_DIR` or `CLINE_DATA_DIR` moves it
([`paths.ts:179-185`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/shared/src/storage/paths.ts#L179-L185),
[`240-246`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/shared/src/storage/paths.ts#L240-L246),
[`session-history-search.ts:196`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/core/src/session/search/session-history-search.ts#L196));
nothing writes it under an editor's `globalStorage`. FTS5 is SQLite's full-text search extension;
the table is created with five `UNINDEXED` columns (stored, not searchable) and three indexed ones
([`:143-153`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/core/src/session/search/session-history-search.ts#L143-L153)),
and
`indexed_sessions(session_id, source_revision, indexed_at, document_count, index_version, title)`
records one row per indexed session
([`:123-142`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/core/src/session/search/session-history-search.ts#L123-L142)).
The SDK's hub server builds and maintains it
([`hub-server-transport.ts:283-287`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/core/src/hub/server/hub-server-transport.ts#L283-L287)).
Per session
([`session-history-search.ts:402-465`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/core/src/session/search/session-history-search.ts#L402-L465)):

- one document `<sessionId>:metadata`, `ordinal` -1, `role` `session`, `title` the session's
  `metadata.title` or else its `prompt`, whitespace collapsed and cut to 240 characters
  ([`:102-109`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/core/src/session/search/session-history-search.ts#L102-L109),
  [`prompt/format.ts:195-206`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/shared/src/prompt/format.ts#L195-L206)),
  and `content` the session's `prompt`
  ([`:423-432`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/core/src/session/search/session-history-search.ts#L423-L432));
- one document `<sessionId>:<n>` per message of the lead transcript with any text, `ordinal` the
  message's 0-based index in the array (so a message with no text leaves a gap), `role` the
  message's `role` or `unknown`
  ([`:97-100`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/core/src/session/search/session-history-search.ts#L97-L100),
  [`:434-448`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/core/src/session/search/session-history-search.ts#L434-L448)).
  The messages are read from the session's `messages_path`
  ([`local-runtime-host.ts:1559-1570`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/core/src/runtime/host/local-runtime-host.ts#L1559-L1570)),
  the lead `<sessionId>.messages.json`; subagent transcripts are not indexed;
- `started_at` and `workspace_root` on every document are the session's (`SessionRecord.startedAt`,
  an ISO 8601 string, and `workspaceRoot`,
  [`shared/src/session/records.ts:34-55`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/shared/src/session/records.ts#L34-L55)).
  No document records a time of its own.

A document's `content` is every string value found by walking the message's `content` (or the
message, when it has none), object keys in order, skipping the keys `data`, `image`, `images`,
`reasoning`, `thinking` and `metadata`, joined with line breaks, trimmed, and capped at 128 KiB
([`:14`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/core/src/session/search/session-history-search.ts#L14),
[`:54-95`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/core/src/session/search/session-history-search.ts#L54-L95)).
A plain-string `content` is therefore indexed as itself: a user prompt with no attached image or
file is a string
([`user-input-builder.ts:10-37`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/core/src/runtime/orchestration/user-input-builder.ts#L10-L37)).
A block array is indexed with the block `type` names, tool ids, tool names and tool input strings in
it (`[{"type":"text","text":"Fixed."}]` becomes `text` and `Fixed.` on two lines), and the text of
`thinking` blocks is left out. Numbers and booleans are left out.

What it survives. The hub reconciles the index when it starts and every five minutes while it runs
([`:13`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/core/src/session/search/session-history-search.ts#L13),
[`:204-216`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/core/src/session/search/session-history-search.ts#L204-L216)):
a session no longer listed in `sessions.db` is deleted from the index, and a session whose
`updated_at`, messages file size or mtime changed is re-indexed from the file as it now is, so a
deleted messages file leaves only the metadata document
([`:350-400`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/core/src/session/search/session-history-search.ts#L350-L400)).
Deleting a session through the hub removes it at once
([`:242-257`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/core/src/session/search/session-history-search.ts#L242-L257)).
The index therefore holds a session's turns after its transcript is gone only until the hub next
runs: a session deleted by hand, by a cleanup script or by an attacker while no Cline process ran is
still in it at collection time. It holds no compaction or rewind history of its own, since each
re-index replaces a session's documents.

#### CLI prompt history

The CLI's prompt history for up-arrow recall is `~/.cline/data/cache/user_input_history.jsonl`
([`apps/cli/src/utils/input-history.ts:7-9`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/cli/src/utils/input-history.ts#L7-L9)).
Each line is `{"prompt": <string>, "ts": <epoch ms>}`. When the terminal UI submits a prompt that is
not a queued delivery
([`apps/cli/src/tui/hooks/use-prompt-input-controller.ts:335-337`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/cli/src/tui/hooks/use-prompt-input-controller.ts#L335-L337),
[`use-input-history.ts:109-116`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/cli/src/tui/hooks/use-input-history.ts#L109-L116)),
the CLI reads the file, removes any earlier copy of the trimmed prompt, appends it, keeps the last
20 (`MAX_HISTORY`) and rewrites the whole file, stamping every line with that write's `Date.now()`
([`input-history.ts:5`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/cli/src/utils/input-history.ts#L5),
[`:37-72`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/cli/src/utils/input-history.ts#L37-L72)).
So the lines are oldest first, each prompt once, and only the last line's `ts` is its own prompt's
time; the others carry the time of the last write. No session id, working directory or model is
recorded. Malformed lines are skipped when the CLI reads it
([`:11-30`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/cli/src/utils/input-history.ts#L11-L30))
and dropped at the next rewrite.

#### Enterprise upload queue

The VS Code extension keeps its enterprise prompt upload queue, the task files waiting for or done
with an upload to the organisation's bucket, in `<GS>/saoudrizwan.claude-dev/cache/sync-queue.json`
([`apps/vscode/src/shared/services/worker/sync.ts:27-29`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/vscode/src/shared/services/worker/sync.ts#L27-L29)):
`{items: {<taskId>/<key>: {id, taskId, key, data, timestamp, status, retryCount, lastError}}}`,
where `key` is the task file's name (such as `api_conversation_history.json`), `data` the file's
whole content, `timestamp` the enqueue time in epoch ms and `status` `pending`, `synced` or `failed`
([`queue.ts:9-31`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/vscode/src/shared/services/worker/queue.ts#L9-L31),
[`157-170`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/vscode/src/shared/services/worker/queue.ts#L157-L170)).
An upload marks its item `synced`
([`212-220`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/vscode/src/shared/services/worker/queue.ts#L212-L220));
a synced item older than seven days and a failed one past its retries or seven days old are removed
([`302-315`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/vscode/src/shared/services/worker/queue.ts#L302-L315),
[`325-338`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/vscode/src/shared/services/worker/queue.ts#L325-L338);
[`worker.ts:201-204`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/vscode/src/shared/services/worker/worker.ts#L201-L204)),
so the file holds the recent uploads only.

#### Hook audit log

The SDK writes a hook audit log, `~/.cline/data/logs/hooks.jsonl` (or `CLINE_HOOKS_LOG_PATH`), for
every session that has no runtime hooks configured, which is the default
([`sdk/packages/core/src/services/local-runtime-bootstrap.ts:447-454`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/sdk/packages/core/src/services/local-runtime-bootstrap.ts#L447-L454)).
Each line is appended, never rewritten
([`sdk/packages/core/src/hooks/hook-file-hooks.ts:629-650`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/sdk/packages/core/src/hooks/hook-file-hooks.ts#L629-L650)),
and carries `ts` and `timestamp` (both ISO 8601), `clineVersion`, `taskId` (the conversation id),
`sessionContext.rootSessionId` (the session id), `workspaceRoots`, `userId` (`CLINE_USER_ID`, else
`USER`), `agent_id` and `parent_agent_id`
([`:234-254`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/sdk/packages/core/src/hooks/hook-file-hooks.ts#L234-L254)),
plus a `hookName`: `agent_start`; `tool_call` with `tool_call{id, name, input}`; `tool_result` with
`tool_result` (the call record) and
`postToolUse{toolName, parameters, result, success, executionTimeMs}`; `agent_end` with
`turn.outputText`; `agent_abort` with `reason`; or `agent_error` with `error{name, message, stack}`
([`:652-730`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/sdk/packages/core/src/hooks/hook-file-hooks.ts#L652-L730));
or `prompt_submit` with `userPromptSubmit.prompt`, the text blocks of each user message that is not
an injected hook-context block
([`:734-756`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/sdk/packages/core/src/hooks/hook-file-hooks.ts#L734-L756),
[`:568-575`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/sdk/packages/core/src/hooks/hook-file-hooks.ts#L568-L575)).
The `tool_result` call record is `{id, name, input, output, error?, durationMs, startedAt, endedAt}`
([`:54-65`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/sdk/packages/core/src/hooks/hook-file-hooks.ts#L54-L65)).
The session store appends `session_shutdown` lines `{ts, hookName, reason, sessionId, pid, source}`
for stale sessions it finds
([`sdk/packages/core/src/session/stores/session-manifest-store.ts:320-341`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/sdk/packages/core/src/session/stores/session-manifest-store.ts#L320-L341)).
No code in `hook-file-hooks.ts` truncates, rotates or deletes the file, and no session delete
([What survives a delete](#what-survives-a-delete)) touches it, so it keeps the tool calls, their
inputs and outputs and the final reply of sessions whose transcripts are gone.

### Format versions

Three generations coexist on disk: VS Code globalStorage tasks, `~/.cline/data/tasks` with
`state/taskHistory.json`, and `~/.cline/data/sessions/<id>/`. The VS Code to file migration of state
and secrets uses the sentinel `__migrationVersion`
([`hosts/vscode/vscode-to-file-migration.ts:11-31`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/vscode/src/hosts/vscode/vscode-to-file-migration.ts#L11-L31));
task files are not moved. SDK files carry `version: 1`. At [`41e75ef`](https://github.com/cline/cline/commit/41e75efc53e7abbcb76564b89e6de57f78659b33) the `task_progress` say
type of `ui_messages.json` is retired and dropped on read. Legacy tasks resumed in the SDK get
`metadata.legacyTask: true`. `sessions.db` has ALTER-based column migrations
([`sqlite-db.ts:275-340`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/shared/src/db/sqlite-db.ts#L275-L340)).

### Secrets

`secrets.json` keys
([`apps/vscode/src/shared/storage/state-keys.ts`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/vscode/src/shared/storage/state-keys.ts)
SECRETS_KEYS):
`apiKey, clineApiKey, clineAccountId, openRouterApiKey, awsAccessKey, awsSecretKey, awsSessionToken, openAiApiKey, geminiApiKey, ..., mcpOAuthSecrets, openai-codex-oauth-credentials, ocaRefreshToken`.
Also `messages.json.system_prompt`, `manifest.prompt`, `schedules.system_prompt`,
`session-search.db` content, `images[]` base64, and file contents and command output in every
transcript. `logs/hooks.jsonl` repeats tool inputs and outputs and the final replies, and its
`userId` is the `CLINE_USER_ID` or `USER` value; a `cloud-pending-starts/*.json` record's `account`
names the Cline account scope (user and organisation) the sandbox was started under
([`apps/vscode/src/sdk/cloud-pending-starts.ts:5-12`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/apps/vscode/src/sdk/cloud-pending-starts.ts#L5-L12)).
Neither is a credential; the parser prints both.

Two files that the upload features use hold credentials and are collected without the secret flag.
The VS Code extension caches each organisation's remote config, `promptUploading` access keys
(`accessKeyId`, `secretAccessKey`) included, as
`<GS>/saoudrizwan.claude-dev/cache/remote_config_<organizationId>.json`
([`apps/vscode/src/core/storage/disk.ts:35`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/vscode/src/core/storage/disk.ts#L35),
[`:186-188`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/vscode/src/core/storage/disk.ts#L186-L188),
[`:248-252`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/vscode/src/core/storage/disk.ts#L248-L252),
[`core/storage/remote-config/sdk-control-plane.ts:198`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/vscode/src/core/storage/remote-config/sdk-control-plane.ts#L198));
the CLI fetches it on each session start and keeps no copy
([`apps/cli/src/utils/enterprise.ts:37-73`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/cli/src/utils/enterprise.ts#L37-L73)).
The LG CNS webhook integration (named for LG CNS, the LG group's IT services company, in the source
directory `lg-cns-integration`) writes `~/Documents/Cline/webhook_config.json` with `webhook_url`,
`webhook_token` (a bearer token) and `created_at`
([`apps/vscode/src/services/lg-cns-integration/webhook-hooks.ts:11-30`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/vscode/src/services/lg-cns-integration/webhook-hooks.ts#L11-L30)).

### Parser plan

Globs (home-relative; `<GS>` is each VS Code `User/globalStorage`):
`<GS>/saoudrizwan.claude-dev/tasks/*/{api_conversation_history,ui_messages,task_metadata}.json`,
`<GS>/saoudrizwan.claude-dev/state/taskHistory.json`, the same under
`.cline/data/`, `.cline/data/sessions/*/*.json`,
`.cline/data/sessions/*/*.messages.json`, `.cline/data/db/sessions.db`,
`.cline/data/db/session-search.db`, `.cline/data/cache/user_input_history.jsonl`,
`<GS>/saoudrizwan.claude-dev/cache/sync-queue.json`, `.cline/data/logs/hooks.jsonl`,
`.cline/data/globalState.json`, `.cline/data/settings/global-settings.json`,
`settings/cline_mcp_settings.json` under `.cline/data/` or `<GS>/saoudrizwan.claude-dev/`, and
`.cline/data/cloud-pending-starts/*.json`.

Rows: one per `ui_messages` entry for legacy tasks (preferred over the API
history because it has `ts`), one per content block of SDK messages.

| Column | Legacy | SDK |
| --- | --- | --- |
| timestamp_utc | `ClineMessage.ts` ms | `message.ts` ms, else manifest `started_at` |
| session_id | directory name | `sessionId` |
| project_path | `cwdOnTaskInitialization` | manifest `cwd` |
| git_branch | empty | empty |
| turn_type | say `task`/`user_feedback` → user; `text`/`completion_result` → assistant; `reasoning` → thinking; `tool`/`command`/`use_mcp_server` → tool_use; `command_output`/`mcp_server_response` → tool_result; `error`/`api_req_*` → system | text → by role; tool_use; tool_result; thinking |
| model | `modelInfo.modelId` or `task_metadata.model_usage` | `modelInfo.id` or manifest `model` |
| tool_name | `ClineSayTool.tool` / `use_mcp_server.toolName` | `tool_use.name` |
| tool_use_id | `ts` | `tool_use.id` / `tool_use_id` |
| text | `text` | first text or tool input |

#### Permission rows

Legacy: the first `task_metadata.json` `model_usage[]` entry's `mode` yields a `system` row
`permission mode: <mode>` and a later entry whose `mode` differs yields `permission mode: <old> ->
<new>`, so an entry that changes only the mode still has a row ([Model](#model-rows)).

The rest is read from [`41e75ef`](https://github.com/cline/cline/commit/41e75efc53e7abbcb76564b89e6de57f78659b33),
source-derived; not observed on any real install:

- Per call, SDK sessions. A call the policy does not auto-approve asks the host; a call that is not
  run gets a `tool_result` block with `is_error` true and content `{error: <reason>}`, persisted as
  that object's JSON string
  ([`sdk/packages/agents/src/agent-runtime.ts:2475-2492`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/sdk/packages/agents/src/agent-runtime.ts#L2475-L2492),
  [`:2555-2559`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/sdk/packages/agents/src/agent-runtime.ts#L2555-L2559),
  [`sdk/packages/core/src/runtime/config/agent-message-codec.ts:296-303`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/sdk/packages/core/src/runtime/config/agent-message-codec.ts#L296-L303),
  [`sdk/packages/core/src/session/persisted-tool-result-content.ts:9-24`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/sdk/packages/core/src/session/persisted-tool-result-content.ts#L9-L24)).
  Every approval that comes back not approved gets the suffix
  ` -- NOT a tool or system failure. Clarify with user before proceeding.`
  ([`sdk/packages/shared/src/llms/tools.ts:95-105`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/sdk/packages/shared/src/llms/tools.ts#L95-L105)),
  and its reason starts with `This tool call was rejected by the user and not executed.`,
  `The user denied this edit. The file was NOT modified...` (with `<feedback>` when the user typed
  some) or `Tool execution was cancelled because the user sent a follow-up message.`
  ([`apps/vscode/src/sdk/tool-approval-denial.ts:4-25`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/apps/vscode/src/sdk/tool-approval-denial.ts#L4-L25)),
  is `Tool approval request timed out`
  ([`sdk/packages/core/src/runtime/tools/tool-approval.ts:101`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/sdk/packages/core/src/runtime/tools/tool-approval.ts#L101)),
  or is the runtime's own `Tool "<name>" requires approval but no approval callback is configured`
  or `Tool "<name>" approval request failed: <error>`
  ([`agent-runtime.ts:2502-2541`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/sdk/packages/agents/src/agent-runtime.ts#L2502-L2541)).
  A call skipped without asking has no suffix: `Tool "<name>" is disabled by policy`, or
  `Tool <name> was blocked by a runtime hook` unless the hook gave a reason of its own
  ([`agent-runtime.ts:2469`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/sdk/packages/agents/src/agent-runtime.ts#L2469),
  [`:2481`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/sdk/packages/agents/src/agent-runtime.ts#L2481)).
  An approved call, by the user or by policy, leaves no mark. The parser yields a `system` row
  after the `tool_result` row, with its `tool_use_id`, `tool_name`, time and model:
  `permission decision: rejected reason=<reason without the suffix>` for a reason with the
  suffix, and `permission decision: blocked reason=<reason>` for the two fixed skip reasons. A
  hook's own reason matches neither and yields no row.
- Standing, CLI and SDK: `~/.cline/data/settings/global-settings.json` (`CLINE_GLOBAL_SETTINGS_PATH`
  overrides,
  [`sdk/packages/shared/src/storage/paths.ts:439-445`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/sdk/packages/shared/src/storage/paths.ts#L439-L445))
  keys `toolAutoApprove` (boolean), `planActMode` (`plan` or `act`), `disabledTools` (names),
  `tools.<name>.enabled`, `disabledPlugins` and `disabledAgentPlugins`
  ([`sdk/packages/core/src/services/global-settings.ts:61-74`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/sdk/packages/core/src/services/global-settings.ts#L61-L74)).
  The CLI auto-approves every tool unless a flag or `toolAutoApprove: false` says otherwise
  ([`apps/cli/src/main.ts:880-894`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/apps/cli/src/main.ts#L880-L894),
  [`apps/cli/src/runtime/interactive/approvals.ts:39-52`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/apps/cli/src/runtime/interactive/approvals.ts#L39-L52)).
  The parser reads every key of this list, one row per value or list entry; `telemetryOptOut`,
  `autoUpdateEnabled`, the compaction keys and `tuiTheme` are not permissions and are not read.
- Standing, VS Code and JetBrains: `~/.cline/data/globalState.json`, a flat JSON object keyed by
  state key
  ([`apps/vscode/src/shared/storage/ClineFileStorage.ts:92`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/apps/vscode/src/shared/storage/ClineFileStorage.ts#L92)),
  key `autoApprovalSettings`,
  `{version, enabled, favorites, maxRequests, actions: {readFiles, readFilesExternally, editFiles, editFilesExternally, executeSafeCommands, executeAllCommands, useBrowser, useMcp}, enableNotifications}`,
  whose default approves reads, edits, all commands, the browser and MCP
  ([`apps/vscode/src/shared/AutoApprovalSettings.ts:1-44`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/apps/vscode/src/shared/AutoApprovalSettings.ts#L1-L44),
  [`apps/vscode/src/shared/storage/state-keys.ts:265-267`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/apps/vscode/src/shared/storage/state-keys.ts#L265-L267)),
  and key `mode` (`plan` or `act`,
  [`:286`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/apps/vscode/src/shared/storage/state-keys.ts#L286)).
  The parser reads `enabled`, `maxRequests`, each `favorites` entry and each `actions.<name>`
  (`version` and `enableNotifications` are not permissions), and `mode`; no other key of the file.
- Standing, MCP: each server in `settings/cline_mcp_settings.json` has `autoApprove`, a list of tool
  names run without asking, and `disabled`
  ([`apps/vscode/src/services/mcp/schemas.ts:10`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/apps/vscode/src/services/mcp/schemas.ts#L10),
  [`:17-19`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/apps/vscode/src/services/mcp/schemas.ts#L17-L19)).
  The file is under `~/.cline/data/settings/` and, from extension builds before the move to
  `~/.cline/data`, under the extension's globalStorage `settings/`
  ([`apps/vscode/src/core/storage/disk.ts:128-129`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/apps/vscode/src/core/storage/disk.ts#L128-L129),
  [`:190-194`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/apps/vscode/src/core/storage/disk.ts#L190-L194));
  the parser reads both, `autoApprove` and `disabled` only, never the server's command, `env`,
  `headers` or `oauth`.

Each standing entry is one `permission setting: <key>=<value> (from <file>, file mtime)` row, as
the `claude-code` parser writes them, at the file's modification time with no session or project:
`autoApprovalSettings.actions.readFiles=true`, `mode=act`, `toolAutoApprove=false`,
`disabledTools=<name>`, `tools.<name>.enabled=false`, `mcpServers.<server>.autoApprove=<tool>`,
`mcpServers.<server>.disabled=false`. A file that is not a JSON object is one `parser:` row.

#### Model rows

All typed `system`:

- Legacy: the task's first `task_metadata.json` `model_usage[]` entry
  yields `model: <model_id> model_provider_id=<model_provider_id>
  mode=<mode>`, and a later entry whose `model_id` differs yields
  `model: <old> -> <new>` with the same pairs, at the entry's `ts`.
- SDK: the manifest `model` and `provider` (or the `sessions.db` row of a
  session whose manifest is gone) yield `model: <model>
  provider=<provider>` at `started_at`, and the session start row no
  longer carries `model=`.
- SDK: a message whose `modelInfo.id` differs from the model before it
  yields `model: <old> -> <new> provider=<modelInfo.provider>` ahead of
  its rows. The lead transcript starts from the manifest `model`; a
  subagent transcript starts from its own first `modelInfo.id`.
- `ui_messages.json` `modelInfo` and `HistoryItem.modelId` fill the
  `model` column only.

#### Past-use rows and secondary sources

Sources, primary or secondary (the analyzer drops a row of a secondary
source that a primary store records too,
[issue 122](https://github.com/seanthegeek/doubleagent/issues/122)):

- Primary: `ui_messages.json`, `api_conversation_history.json` (when no
  `ui_messages.json` is beside it), `task_metadata.json`,
  `state/taskHistory.json`, the SDK manifest and `*.messages.json`, and
  `sessions.db` (only for sessions whose manifest is gone).
- Secondary: `session-search.db`. One `system` row per indexed session at
  its `started_at`, `session search index: title=<title>
  workspace_root=<workspace_root> prompt=<metadata content>
  document_count=<n> indexed_at=<indexed_at>`; then one row per message
  document in `ordinal` order, `user` or `assistant` by `role`, `system`
  for any other, `source_line` `ordinal + 1` (the message's position in
  the transcript, as the messages reader numbers it), and no
  `timestamp_utc`, since the index records none per message.
  `session_id` is the index's `session_id`, which is the session
  directory name the messages reader uses; `project_path` is the
  manifest's `cwd` (else `workspace_root`) when the manifest is on disk,
  as the messages reader takes it, and the document's `workspace_root`
  otherwise. A plain-string message pairs with its transcript row on
  text and session; a block-array message's text carries the block type
  names and tool ids and never equals a transcript row, so it is kept
  whether or not the transcript survives. When the SQLite build lacks
  FTS5, one `system` row says so and `indexed_sessions` is read alone,
  each session row without a time.
- Secondary: `cache/user_input_history.jsonl`
  ([CLI prompt history](#cli-prompt-history)). One `user` row per line, with no
  `session_id` or `project_path`; the last line at its `ts`, every other
  line with no `timestamp_utc`, and every line undated when the file has a
  bad line, since the last line read may then not be the newest. An
  undated row matches a transcript prompt of any Cline session on text
  alone; the dated one matches within 2 seconds. It survives the deletion
  of the sessions it came from, holding their last 20 distinct prompts.
  When a prompt is in this file and in `session-search.db` too, whichever
  secondary row the duplicate rule reaches first takes the transcript row,
  and in the fixture the index's copy is the one left over.
- Secondary: `logs/hooks.jsonl` ([Hook audit log](#hook-audit-log)), source-derived; not observed
  on any real install. One row per line, at its `ts`, with `session_id`
  `sessionContext.rootSessionId` (else `taskId`, else the shutdown line's
  `sessionId`), the session directory name the messages reader uses, and
  `project_path` the first `workspaceRoots` entry: `prompt_submit` a `user`
  row with the prompt; `tool_call` a `tool_use` row with `tool_name`, the
  call id as `tool_use_id` and the input summarised as the transcript's
  `tool_use` row is; `tool_result` a `tool_result` row with the call id and
  `postToolUse.result`, or `[error] <error>` when the record has an
  `error`; `agent_end` an `assistant` row with `turn.outputText`. The tool
  rows pair with the transcript's on the call id, the prompt and the reply
  on text, within one hour, since a call line is stamped when the call
  starts or ends and the transcript message when the turn is written. Every
  other line is a `system` row `hook audit: <hookName>` with
  `cline_version=`, `user=`, `agent=`, `parent_agent=`, `reason=`, `pid=`
  and `source=` when present and `error=<name>: <message>` (never the
  stack); no transcript row repeats them, so they stay. What it survives is
  in [What survives a delete](#what-survives-a-delete).

The API history is not a second copy of `ui_messages.json` that the rule could pair, so it stays a
fallback for a task without `ui_messages.json`. The two record different texts for the same turn:
the API history wraps the first prompt as `<task>\n...\n</task>` (with environment details in real
tasks) where `ui_messages.json` holds the prompt as typed, as the extension's own sample-task
generator writes them
([`apps/vscode/src/dev/commands/tasks.ts:95-117`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/vscode/src/dev/commands/tasks.ts#L95-L117),
[`:172-177`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/vscode/src/dev/commands/tasks.ts#L172-L177)).
Its tool calls are content blocks or XML inside text, where `ui_messages.json` has one `say`/`ask`
record per tool with JSON text, and its records carry no time. In the fixture the task's API user
row is `<task>fix tests</task>` beside the UI row `fix tests | [1 image(s)]`, and its tool rows have
no UI twin with the same text.

`<sessionId>.compaction.json` repeats turns the messages file keeps, and
stays unread. An imported session is read as any SDK session, and its
start row names the source, source-derived; not observed on any real
install: `metadata.importedFrom` adds `imported_from=<tool>
imported_session=<sourceSessionId> imported_path=<sourcePath>
imported_at=<importedAt> imported_provider=<sourceProvider>
imported_model=<sourceModel>`, so the copy of another agent's transcript
is not read as Cline's own work.

#### Sync, relay and origin rows

Sync: the SDK manifest `metadata` key `enterprise.blobUpload` (and the same
object in `sessions.db` `metadata_json`,
[Enterprise upload marker](#enterprise-upload-marker)) marks
a session whose transcript is uploaded to the organisation's own S3, R2 or
Azure bucket under an enterprise remote config. It is written once, at session
start, and the upload repeats on every rewrite of the messages file, so the
remote copy is the transcript as last written; the organisation's remote
config turns it on, not a local setting.
The parser yields one `system` row after the session start row (and its
`model:` row), `session
synced: enterprise prompt upload to <adapterType> bucket <bucket> object
sessions/<userDistinctId or unknown>/<session_id>/messages.json
endpoint=<endpoint or -> region=<region or -> account=<accountId or -> (from
<session_id>.json metadata enterprise.blobUpload, started_at)`, timestamped by
the manifest's `started_at`, since the key is written at the start of the
session. For a session whose manifest is gone, the `sessions.db` row
yields the same text ending `(from sessions.db sessions.metadata_json
enterprise.blobUpload, started_at)` at that row's `started_at`. The
parser sets `records_sync`, so an SDK session without the key is `synced`
`no`. Fixture only: no real install has shown the key.

For the VS Code
extension's legacy tasks the marker is the upload queue
`cache/sync-queue.json` ([Enterprise upload queue](#enterprise-upload-queue)): the parser
yields one `system` row per item, with `session_id` the `taskId` and
`project_path` the task's `cwdOnTaskInitialization` from
`state/taskHistory.json`, at the item's `timestamp`: for a `synced`
item `session synced: enterprise prompt upload synced key <key> retries
<retryCount> (from saoudrizwan.claude-dev/cache/sync-queue.json, item
timestamp)`, and for a `pending` or `failed` one the same text without
the `session synced: ` prefix (`enterprise prompt upload <status> key
...`), since that file has not reached the bucket, so a task with only
such items is `synced` `no`; `-` for a value absent or empty. It never
prints `data` (the file's content) or
`lastError`. A file that is not a JSON object with an `items` object is
one `parser:` row. Fixture only: no real install has shown the file.

A delete through the hub leaves no marker on disk
([What survives a delete](#what-survives-a-delete)), and nothing deletes the uploaded object: the
bucket adapter has only
`write`
([`blob-storage.ts:28-30`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/shared/src/remote-config/blob-storage.ts#L28-L30)).
The CLI also reports each upload as a telemetry event `enterprise.prompt_upload_succeeded` or
`enterprise.prompt_upload_failed` with the session id and bucket
([`apps/cli/src/utils/enterprise.ts:75-115`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/cli/src/utils/enterprise.ts#L75-L115)),
which leaves the host and is not written locally.

Other paths by which session content leaves the host record nothing the parser can read. The LG CNS
webhook is installed by opening a `vscode://` link to the extension's `/lg-task` path with
`prompt-file`, `webhook-url` and `webhook-token`
([`apps/vscode/src/services/uri/SharedUriHandler.ts:98-121`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/vscode/src/services/uri/SharedUriHandler.ts#L98-L121));
it writes `webhook_config.json` ([Secrets](#secrets)) and three hook scripts, `TaskStart`,
`PostToolUse` and `TaskComplete` (`.ps1` on Windows), into `~/Documents/Cline/Hooks/`
([`:32-59`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/vscode/src/services/lg-cns-integration/webhook-hooks.ts#L32-L59),
[`apps/vscode/src/core/storage/disk.ts:88-97`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/vscode/src/core/storage/disk.ts#L88-L97)),
which POST every task start, tool call (tool name and parameters) and task completion to the webhook
([`:230-322`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/vscode/src/services/lg-cns-integration/webhook-hooks.ts#L230-L322)).
Those files are configuration, not per-task markers; the hooks are already in the collected
`Documents/Cline` tree. Cloud handoff, the SDK's move of a local session into a Cline cloud sandbox
with its transcript as the seed, defines a `metadata.handoff` object (`toCloudSessionId`,
`handedOffAt`, `status`, `innerSessionId`, `dashboardUrl`, `fingerprint`)
([`sdk/packages/core/src/services/cloud-handoff/types.ts:13-20`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/core/src/services/cloud-handoff/types.ts#L13-L20),
[`cloud-handoff/metadata.ts:98-132`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/core/src/services/cloud-handoff/metadata.ts#L98-L132)),
but no host in the repository starts a handoff or writes that object: the only caller of the cloud
session controller is the example desktop app
([`apps/examples/desktop-app/sidecar/cloud-sessions.ts:31-53`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/examples/desktop-app/sidecar/cloud-sessions.ts#L31-L53)),
whose cloud sessions run and are stored server-side. Model-call tracing to Langfuse (an LLM tracing
service) is configured from the environment only: direct `LANGFUSE_*` credentials, or a host
OpenTelemetry relay whose spans carry prompt and reply text only with `CLINE_TRACE_RECORD_CONTENT`
set
([`sdk/packages/llms/src/services/langfuse-telemetry.ts:146-163`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/llms/src/services/langfuse-telemetry.ts#L146-L163),
[`:198-268`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/sdk/packages/llms/src/services/langfuse-telemetry.ts#L198-L268)).
The task history (`HistoryItem`) has no share, sync or remote field
([`apps/vscode/src/shared/HistoryItem.ts:1-26`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/vscode/src/shared/HistoryItem.ts#L1-L26)),
and no share-task feature or `shareUrl` exists at this commit (searched `share`, `shareUrl`,
`shareTask` in `apps/` and `sdk/`). No earlier release shipped one either: the same search finds
nothing at the release tags v3.0.0, v3.10.0, v3.20.0, v3.30.0, v3.40.0, v3.60.0, v3.80.0, v3.89.2
and v4.0.0 (from [`7fa399c`](https://github.com/cline/cline/commit/7fa399c6af3995c1628f0240de894dedb84b9053),
December 2024, to [`1e88a70`](https://github.com/cline/cline/commit/1e88a708bde949cf92808c1638d8fb869dc55e05),
June 2026), and the extension's changelog, which runs back to 0.0.6, names no share or share-link
feature, so a Cline install of any release holds no share marker to parse.

Origin: a Cline Cloud session ([Cline Cloud](#cline-cloud))
originated on the vendor's servers and has no local transcript. The parser
yields two `system` rows per `cloudSessionStatuses` entry of
`globalState.json`, at its `observedAt` with the id as `session_id`:
`session origin: remote cline-cloud session <ses- id> status <status>
(from .cline/data/globalState.json cloudSessionStatuses, observedAt)` and
`session synced: cline-cloud holds session <ses- id>, no local transcript
(from .cline/data/globalState.json cloudSessionStatuses, observedAt)`,
since the vendor holds the session. Each record of a
`cloud-pending-starts/*.json` file yields the same pair at the file's
modification time, the origin row reading `sandbox start pending
account=<account> endpoint=<endpoint>` in place of the status and both
ending `(from <file>, file mtime)`; a file that is not a JSON array is one
`parser:` row and a record without a `sessionId` one row each. The
`cloudTaskTarget` is one `cloud task target: target=<target>
repository_id=<repositoryId> branch=<branch> (from
.cline/data/globalState.json, file mtime)` row. The parser sets
`records_origin`, so every other Cline session's `origin` is `local`. All
of it is source-derived; not observed on any real install. A session
deleted on the server leaves `cloudSessionStatuses` at the next listing,
so the rows show only the sessions the account still listed when the file
was last written.

#### Scratch directory rows

The scratch directories ([Scratch directories](#scratch-directories)) are
`~/.cline/data/workspaces/chat`, the working directory of a session started without a project and
of an imported session with no recorded directory
([`sdk/packages/shared/src/storage/chat-workspace-paths.ts:2`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/sdk/packages/shared/src/storage/chat-workspace-paths.ts#L2)),
and `~/.cline/worktrees/<task id>/<folder>`, the CLI's task worktrees
([`apps/cli/src/utils/worktree.ts:19-21`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/apps/cli/src/utils/worktree.ts#L19-L21),
[`:99-104`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/apps/cli/src/utils/worktree.ts#L99-L104)),
are recorded as the manifest `cwd` and `workspace_root`; both are under the catalog entry `.cline`,
so the analyzer's catalog rule classes them and the parser need not write `scratch directory:` rows.

#### Deletion rows

No delete writes a marker ([What survives a delete](#what-survives-a-delete)), so the parser
yields no deletion row.

#### Bad-line rows

Every file the parser reads is covered. `logs/hooks.jsonl` is read line by line with each bad line
its own `system` row; a settings file or `globalState.json` that is not a JSON object, and a
pending-start file that is not a JSON array, is one `parser:` row. How a bad line changes the
history rows and the upload queue rows is under
[Past-use rows and secondary sources](#past-use-rows-and-secondary-sources) and
[Sync, relay and origin](#sync-relay-and-origin-rows).

A `sessions.db` or `session-search.db` that cannot be read, or whose `-wal`, `-shm` or `-journal`
sidecar cannot, is one row `parser: <file> could not be read (<reason>)`, `<file>` the name of the
file that failed; one that is not a SQLite database is
`parser: sessions.db is not a SQLite database` or
`parser: session-search.db is not a SQLite database`.

A `logs/hooks.jsonl` or `cache/user_input_history.jsonl` that cannot be read is one whole-file row
`parser: <file> could not be read (<reason>)` through `error_rows`, `<file>` its name. So is a task
file, a `taskHistory.json`, a session manifest or messages file, `sync-queue.json`,
`globalState.json`, a settings file or a pending-start file that cannot be read, in place of the
row for a file of the wrong shape, with the task's or session's id where the path gives one.

#### Fixtures

Fixtures (invented content, real field names):

```json
{"_file":"ui_messages.json","record":{"ts":1760000000000,"type":"say","say":"tool","text":"{\"tool\":\"readFile\",\"path\":\"src/app.py\",\"content\":\"/home/u/proj/src/app.py\"}","modelInfo":{"modelId":"claude-sonnet-4-5","providerId":"anthropic","mode":"act"}}}
{"_file":"ui_messages.json","record":{"ts":1760000001000,"type":"say","say":"api_req_started","text":"{\"request\":\"<task>fix tests</task>\",\"tokensIn\":1200,\"tokensOut\":80,\"cacheWrites\":0,\"cacheReads\":900,\"cost\":0.0041}"}}
{"_file":"api_conversation_history.json","record":{"role":"assistant","content":[{"type":"tool_use","id":"toolu_01A","name":"execute_command","input":{"command":"pytest -q"}}]}}
{"_file":"api_conversation_history.json","record":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_01A","content":"3 passed","is_error":false}]}}
{"_file":"state/taskHistory.json","record":{"id":"1760000000000","ulid":"01JAX3Q8M4","ts":1760000000000,"task":"fix tests","tokensIn":1200,"tokensOut":80,"totalCost":0.0041,"cwdOnTaskInitialization":"/home/u/proj","modelId":"claude-sonnet-4-5","apiProvider":"anthropic"}}
{"_file":"sessions/<id>/<id>.json","record":{"version":1,"session_id":"1760000000000_k3x9q","source":"cli","pid":4242,"started_at":"2026-10-03T09:00:00.000Z","status":"completed","interactive":true,"provider":"anthropic","model":"claude-sonnet-4-5","cwd":"/home/u/proj","workspace_root":"/home/u/proj","enable_tools":true,"enable_spawn":false,"enable_teams":false,"prompt":"fix tests","metadata":{"title":"fix tests"},"messages_path":"/home/u/.cline/data/sessions/1760000000000_k3x9q/1760000000000_k3x9q.messages.json"}}
{"_file":"sessions/<id>/<id>.messages.json","record":{"version":1,"updated_at":"2026-10-03T09:00:05.000Z","agent":"lead","sessionId":"1760000000000_k3x9q","origin":{"source":"cli","mode":"user","sessionId":"1760000000000_k3x9q"},"messages":[{"id":"m1","role":"user","content":"fix tests","ts":1760000000000},{"id":"m2","role":"assistant","content":[{"type":"thinking","thinking":"run the suite"},{"type":"tool_use","id":"call_1","name":"bash","input":{"command":"pytest -q"}}],"modelInfo":{"id":"claude-sonnet-4-5","provider":"anthropic"},"metrics":{"inputTokens":1200,"outputTokens":80,"cost":0.0041},"ts":1760000002000},{"id":"m3","role":"user","content":[{"type":"tool_result","tool_use_id":"call_1","name":"bash","content":"3 passed","is_error":false}],"ts":1760000003000}]}}
{"_file":"logs/hooks.jsonl","record":{"ts":"2026-10-03T09:00:02.500Z","clineVersion":"4.1.25","timestamp":"2026-10-03T09:00:02.500Z","taskId":"1760000000000_k3x9q","sessionContext":{"rootSessionId":"1760000000000_k3x9q"},"workspaceRoots":["/home/u/proj"],"userId":"u","agent_id":"agent_lead","hookName":"tool_call","iteration":1,"tool_call":{"id":"call_1","name":"bash","input":{"command":"pytest -q"}},"preToolUse":{"toolName":"bash","parameters":{"command":"pytest -q"}}}}
{"_file":"logs/hooks.jsonl","record":{"ts":"2026-10-03T09:10:00.000Z","hookName":"session_shutdown","reason":"stale_process","sessionId":"1760000000000_k3x9q","pid":4100,"source":"cli"}}
{"_file":"sessions/<id>/<id>.messages.json","record":{"id":"m5","role":"user","content":[{"type":"tool_result","tool_use_id":"call_2","name":"web_fetch","content":"{\"error\":\"Tool \\\"web_fetch\\\" is disabled by policy\"}","is_error":true}],"ts":1760000004000}}
{"_file":"globalState.json","record":{"autoApprovalSettings":{"version":1,"enabled":true,"favorites":[],"maxRequests":20,"actions":{"readFiles":true,"editFiles":true,"executeSafeCommands":false,"executeAllCommands":true,"useBrowser":true,"useMcp":true},"enableNotifications":false},"mode":"act","cloudTaskTarget":{"target":"cloud","repositoryId":812345678,"branch":"main"},"cloudSessionStatuses":{"ses-01JCLOUDDONE":{"status":"completed","observedAt":1760000100000}}}}
{"_file":"settings/global-settings.json","record":{"telemetryOptOut":false,"autoUpdateEnabled":true,"planActMode":"plan","toolAutoApprove":false,"disabledTools":["browser_action"],"tools":{"web_fetch":{"enabled":false}}}}
{"_file":"settings/cline_mcp_settings.json","record":{"mcpServers":{"github":{"command":"npx","autoApprove":["list_issues"],"disabled":false}}}}
{"_file":"cloud-pending-starts/<pid>-<uuid>.json","record":[{"sessionId":"ses-01JCLOUDWAIT","account":"user:usr_0f9e8d7c","endpoint":"https://api.cline.bot"}]}
{"_file":"sessions/<id>/<id>.json","record":{"metadata":{"title":"port the parser","importedFrom":{"tool":"claude-code","sourceSessionId":"5f0c1d2e-0000-4000-8000-00000000c1a0","sourcePath":"/home/u/.claude/projects/-home-u-proj/5f0c1d2e.jsonl","importedAt":"2026-10-03T09:20:00.000Z","sourceProvider":"anthropic","sourceModel":"claude-opus-4-5"}}}}
```

#### Source host rows

Cline Cloud runs a task against a GitHub repository through the vendor's GitHub App: connecting
opens the App's install page in the browser
([`apps/vscode/src/core/controller/cloud/connectGitHub.ts:5-10`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/apps/vscode/src/core/controller/cloud/connectGitHub.ts#L5-L10),
[`apps/vscode/src/services/cloud/CloudSessionsService.ts:166-187`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/apps/vscode/src/services/cloud/CloudSessionsService.ts#L166-L187)),
so the grant is server-side only, held by the App installation on GitHub and the Cline account,
and no GitHub token or login is written locally; the extension signs in to the Cline account
with the `secrets.json` credentials ([Credentials](#credentials)). The list of repositories the
App can reach comes from the Cline API
([`CloudSessionsService.ts:146-164`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/apps/vscode/src/services/cloud/CloudSessionsService.ts#L146-L164))
and is cached in memory for 60 seconds, never on disk
([`apps/vscode/src/sdk/sdk-cloud-task-target.ts:50`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/apps/vscode/src/sdk/sdk-cloud-task-target.ts#L50),
[`:218-249`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/apps/vscode/src/sdk/sdk-cloud-task-target.ts#L218-L249)).
The repository-bound conversations are the Cline Cloud sessions, stored server-side; locally there
are only their ids and statuses and the user's last choice of repository id and branch
([Cline Cloud](#cline-cloud)), which the parser reads as the origin, sync and `cloud task target:`
rows of [Sync, relay and origin rows](#sync-relay-and-origin-rows). With no account on disk the
parser writes no `source host:` row. A GitHub MCP server's token in
`settings/cline_mcp_settings.json` `env` is a credential of that server, not an integration of
Cline's (grepped `github`, `gitlab`, `bitbucket`, `oauth`, `installation`, `repositor`, `clone`
in `apps/vscode/src` and `sdk/packages` at
[`41e75ef`](https://github.com/cline/cline/commit/41e75efc53e7abbcb76564b89e6de57f78659b33)).

## Confidence

- High: every `~/.cline/data` path, secrets key list, globalStorage layout, discovery keys; file
  names, layouts, `ClineMessage`, `HistoryItem`, manifest and `MessageWithMetadata` fields,
  `sessions.db` columns.
- Medium: whether `Documents/Cline/Agents|Plugins` is written by the extension (the SDK only reads
  them); exact `metadata` keys in SDK messages (collected from call sites, not a schema).
- Low: JetBrains plugin's own storage beyond `~/.cline/data` and `WORKSPACE_STORAGE_DIR`
  ([`storage-context.ts:58-63`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/vscode/src/shared/storage/storage-context.ts#L58-L63));
  not determinable without the closed plugin.
- Not determined: whether the VS Code extension now writes new tasks to `~/.cline/data/sessions`
  rather than globalStorage (code reads both; `legacyExtensionStorageDir` is the globalStorage
  path,
  [`SdkController.ts:505`](https://github.com/cline/cline/blob/39ff2359f7e08231281539696e48a166ce49270c/apps/vscode/src/sdk/SdkController.ts#L505)).
