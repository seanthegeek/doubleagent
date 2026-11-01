# qwen-code

Qwen Code (catalog agent `qwen-code`), from [QwenLM/qwen-code](https://github.com/QwenLM/qwen-code),
is a fork of Gemini CLI ([gemini-cli.md](gemini-cli.md)), but its transcript
format has fully diverged: Gemini writes a metadata line plus message records
with `$set`/`$patch`/`$rewindTo` operations, Qwen writes a flat,
Claude-Code-style record stream with `uuid`, `parentUuid`, `cwd` and
`gitBranch` on every line. What it still shares with Gemini CLI is the per-project
`tmp/<sha256(cwd)>/` directory, which Gemini CLI has since replaced with slug directories and keeps
only as a legacy copy
([Transcript stores in `gemini-cli.md`](gemini-cli.md#transcript-stores)), and in it `logs.json`,
`shell_history` and the `checkpoint-<tag>.json` files, which Qwen still writes in the raw-array
shape Gemini CLI has left behind
([Format versions in `gemini-cli.md`](gemini-cli.md#format-versions)). Besides the CLI
it has a Web Shell served by `qwen serve`, Qwen Code Desktop, a Tauri app that wraps that Web
Shell ([Qwen Code Desktop](#qwen-code-desktop)), and messaging-platform channels. Paths cited are
relative to the clone. The agent is parsed: the `qwen-code` parser reads the transcripts,
`logs.json`, `usage_record.jsonl`, `trustedFolders.json`, `settings.json` and the channel route
files ([Parser plan](#parser-plan)).

## Source and evidence

[QwenLM/qwen-code](https://github.com/QwenLM/qwen-code), commit
[`2c591ecc08a6fa080342f9b1b9f7f43215178cbb`](https://github.com/QwenLM/qwen-code/commit/2c591ecc08a6fa080342f9b1b9f7f43215178cbb)
(2026-10-03, version `0.24.7`), Apache-2.0. Open source. All claims are source-level (`packages/...`
file:line) unless marked `docs/` (repo documentation, used for directories whose writer lives in
code not grepped here). The layout has grown far beyond the v1.1 catalog (channels, boards, arena,
workflows, audits).

### Observed

- No real install. `~/.qwen` does not exist on this workstation, so no live check. The channel
  route files are fixture only: no real install has shown either file
  ([Channel routes](#channel-routes)).

### Source reviews

- 2026-10-21: QwenLM/qwen-code at [`5dcf7c7`](https://github.com/QwenLM/qwen-code/commit/5dcf7c7cb162e825d95d531081f67ee60da3c0e1) (0.25.3): the per-call
  decision, approval-mode, `settings.json` permission and `usage_record.jsonl` rows moved from
  proposed to parsed, the `ui_telemetry` tool-call event name was corrected to
  `qwen-code.tool_call`, the session registry gained its secret glob, and the source-host review
  of the GitHub and GitLab channels and the pull-request sidecar was added
  ([Source host rows](#source-host-rows)).
- 2026-10-19: HEAD
  [`5dcf7c7`](https://github.com/QwenLM/qwen-code/commit/5dcf7c7cb162e825d95d531081f67ee60da3c0e1)
  (2026-10-20 UTC+8, version 0.25.3) against the commit above; `storage.ts`, `paths.ts` and the
  channel runtime had not changed. It added `usage_record.jsonl`, the managed auto-memory, the
  scratch workspaces and worktrees, the delete path, Qwen Code Desktop, the per-call and standing
  permission records, `executionContext` and the record subtypes of
  [Format versions](#format-versions), and the reading of `trustedFolders.json` as JSONC (JSON with
  comments, trailing commas and a byte-order mark allowed).

## Collection

### Per-user storage

`QWEN_DIR = '.qwen'`
([`core/src/utils/paths.ts:14`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/utils/paths.ts#L14)).
`getGlobalQwenDir()` returns `$QWEN_HOME` (with `~` expansion) else `os.homedir()/.qwen`
([`core/src/config/storage.ts:193-203`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/config/storage.ts#L193-L203)).
No platform branch for user paths; `platformFoldsCase()` only affects comparisons
([30-32](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/config/storage.ts#L30-L32)).
Same layout on Linux, macOS, `%USERPROFILE%\.qwen` on Windows; no XDG.

A second root, the runtime base dir, defaults to the global dir but is relocated by
`$QWEN_RUNTIME_DIR`
([`storage.ts:172-191`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/config/storage.ts#L172-L191)).
It holds `tmp/`, `projects/`, `debug/`
([229-238](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/config/storage.ts#L229-L238))
and `resources/<sessionId>`
([`core/src/utils/sessionStorageUtils.ts:813`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/utils/sessionStorageUtils.ts#L813)).

Global dir files (all `storage.ts` unless noted): `settings.json`
([209](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/config/storage.ts#L209)),
`installation_id`
([213](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/config/storage.ts#L213)),
`google_accounts.json`
([217](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/config/storage.ts#L217),
defined, no writer found), `commands/`
([221](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/config/storage.ts#L221)),
`memory.md`
([225](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/config/storage.ts#L225),
no callers found; docs use `~/.qwen/QWEN.md`,
[`docs/users/features/memory.md:30`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/docs/users/features/memory.md#L30)),
`ide/`
([241-245](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/config/storage.ts#L241-L245)),
`plans/`
([325](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/config/storage.ts#L325)),
`bin/`
([340](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/config/storage.ts#L340)),
`arena/`
([344](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/config/storage.ts#L344);
[`core/src/agents/arena/ArenaManager.ts:136`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/agents/arena/ArenaManager.ts#L136)),
`audits/<sha256>/`
([371](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/config/storage.ts#L371)),
`workflows/`
([670](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/config/storage.ts#L670)),
`extensions/`
([768](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/config/storage.ts#L768)),
`skills/` plus `~/.agents/skills`
([17](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/config/storage.ts#L17),
[758](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/config/storage.ts#L758)),
`rules/`
([`core/src/config/rulesDiscovery.ts:331`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/config/rulesDiscovery.ts#L331)),
`sessions/<pid>.json` live session registry, one record per running session
([Session registry](#session-registry)),
([`core/src/services/session-registry.ts:322`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/services/session-registry.ts#L322),[`332`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/services/session-registry.ts#L332)),
`usage_record.jsonl`, one usage summary per session that outlives the session
([usage_record.jsonl](#usage_recordjsonl);
[`core/src/services/usageHistoryService.ts:140`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/services/usageHistoryService.ts#L140)),
`model-registry.json`
([`core/src/models/model-catalog.ts:64`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/models/model-catalog.ts#L64)),
`artifacts/`
([`core/src/tools/artifact/local-publisher.ts:30`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/tools/artifact/local-publisher.ts#L30)),
`file-history/` edit backups
([`core/src/services/sessionService.ts:3317`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/services/sessionService.ts#L3317),[`4274`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/services/sessionService.ts#L4274)),
`boards/`
([`core/src/agents/team/board-lock.ts:32-35`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/agents/team/board-lock.ts#L32-L35)),
`batch/`
([`cli/src/commands/batch-task.ts:283`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/cli/src/commands/batch-task.ts#L283)),
`tip_history.json`
([`cli/src/services/tips/tipHistory.ts:132`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/cli/src/services/tips/tipHistory.ts#L132)),
`startup-perf/`
([`cli/src/utils/startupProfiler.ts:339`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/cli/src/utils/startupProfiler.ts#L339)),
`source.json`
([`core/src/telemetry/qwen-logger/qwen-logger.ts:372-376`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/telemetry/qwen-logger/qwen-logger.ts#L372-L376)),
`channels/{service.pid,sessions.json,cron.json}`
([`cli/src/commands/channel/pidfile.ts:55`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/cli/src/commands/channel/pidfile.ts#L55),
[`runtime.ts:44`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/cli/src/commands/channel/runtime.ts#L44),[`89`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/cli/src/commands/channel/runtime.ts#L89)),
`updates/npm/`
([`cli/src/utils/managed-npm-update.ts:257`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/cli/src/utils/managed-npm-update.ts#L257)),
`scratch-workspaces/`
([`cli/src/serve/run-qwen-serve.ts:5231`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/cli/src/serve/run-qwen-serve.ts#L5231)),
`extension-store/`
([`core/src/extension/extension-store.ts:523`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/extension/extension-store.ts#L523)),
`debug/<sessionId>.txt`
([233-238](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/config/storage.ts#L233-L238)),
`trustedFolders.json`
([`cli/src/config/trustedFolders.ts:33-41`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/cli/src/config/trustedFolders.ts#L33-L41),
override `QWEN_CODE_TRUSTED_FOLDERS_PATH`), `.env`
([`cli/src/config/environment.ts:192-195`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/cli/src/config/environment.ts#L192-L195)).
Named in the docs only, with no writer found in the code at the first commit: `desktop-relay/`,
`computer-use/`, `browser-use/`, `todos/`, `paste-cache/`, `output-styles/`, `output-language.md`,
`log/otel-*`, `locales/` (user locale overrides; the bundled `locales` is in the install dir,
[`cli/src/i18n/index.ts:47`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/cli/src/i18n/index.ts#L47)).

`projects/<sanitizeCwd(cwd)>/`
([619-623](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/config/storage.ts#L619-L623);
non-alphanumerics replaced by `-`,
[`paths.ts:388-391`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/utils/paths.ts#L388-L391)):
`chats/<uuid>.jsonl`
([`sessionService.ts:459`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/services/sessionService.ts#L459),[`948`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/services/sessionService.ts#L948);
[`chatRecordingService.ts:1305`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/services/chatRecordingService.ts#L1305)),
`chats/<id>.worktree.json`
([`cli/src/serve/routes/session-pr-backfill.ts:314`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/cli/src/serve/routes/session-pr-backfill.ts#L314)),
`workflows/` run snapshots and `generated/` scripts
([679-723](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/config/storage.ts#L679-L723)),
`memory/`, the managed auto-memory (below).

`tmp/<sha256(cwd)>/`
([625-629](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/config/storage.ts#L625-L629);
[`paths.ts:368-372`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/utils/paths.ts#L368-L372)):
`logs.json`, `checkpoint-<tag>.json`
([`core/src/core/logger.ts:14`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/core/logger.ts#L14),[`204`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/core/logger.ts#L204),[`693`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/core/logger.ts#L693)),
`checkpoints/`, `tool-results/`
([633](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/config/storage.ts#L633)),
`shell_history`
([773](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/config/storage.ts#L773)),
`attachments/`, `scheduled_tasks.json`, `otel/` (docs). No shadow git repository exists in this fork
(`gitService.ts` absent).

The `memories/` and `memory/` directories, named in the docs only at the first commit, are written
by code at
[`5dcf7c7`](https://github.com/QwenLM/qwen-code/commit/5dcf7c7cb162e825d95d531081f67ee60da3c0e1):
`projects/<sanitized project>/memory/` holds the managed auto-memory, on by default, and
`memories/` the user-level form
([`core/src/memory/paths.ts:19`](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/core/src/memory/paths.ts#L19),
[34](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/core/src/memory/paths.ts#L34),
[116-148](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/core/src/memory/paths.ts#L116-L148);
documents in [Managed auto-memory](#managed-auto-memory)).

#### Scratch directories

`~/.qwen/scratch-workspaces/` is the managed scratch root `qwen serve` (the daemon behind the Web
Shell and the desktop app) prepares for a session that has no workspace
([`cli/src/serve/run-qwen-serve.ts:5285-5292`](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/cli/src/serve/run-qwen-serve.ts#L5285-L5292)).
It is excluded ([Exclusions](#exclusions)), so the files a session made there are not collected,
while the transcript that names it is. The per-project worktrees,
`<project>/.qwen/worktrees/<slug>`, are in [Project-local files](#project-local-files); how the
transcripts record both is in [Scratch workspaces and worktrees](#scratch-workspaces-and-worktrees).

#### Qwen Code Desktop

At
[`5dcf7c7`](https://github.com/QwenLM/qwen-code/commit/5dcf7c7cb162e825d95d531081f67ee60da3c0e1),
Qwen Code Desktop (`packages/desktop`), a Tauri app that wraps the Web
Shell served by `qwen serve`, has the identifier `com.alibaba.qwen-code` and keeps
`desktop-state.json` (window and zoom state) in its own config directory
([`desktop/src-tauri/tauri.conf.json:3-5`](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/desktop/src-tauri/tauri.conf.json#L3-L5);
[`desktop/src-tauri/src/desktop_state.rs:177-182`](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/desktop/src-tauri/src/desktop_state.rs#L177-L182));
its sessions are ordinary `~/.qwen` transcripts. That directory and the webview's own storage are
not in the catalog; their platform paths were not determined from source. Its Local Control setting
shares the live daemon with a phone on the same network
([`desktop/README.md:17`](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/desktop/README.md#L17));
whether the app or that setting leaves a local record of a paired device was not determined.

### Credentials

- `oauth_creds.json` plaintext, mode 0600, plus `oauth_creds.lock`
  ([`core/src/qwen/sharedTokenManager.ts:26-27`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/qwen/sharedTokenManager.ts#L26-L27),[`643`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/qwen/sharedTokenManager.ts#L643);
  [`qwenOAuth2.ts:41`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/qwen/qwenOAuth2.ts#L41),[`1094`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/qwen/qwenOAuth2.ts#L1094)).
  No OS keychain for the main login.
- `mcp-oauth-tokens-v2.json` and `extension-secrets-v1.json`: AES-GCM with
  `scryptSync('qwen-code-oauth', '<hostname>-<username>-qwen-code')`
  ([`core/src/mcp/token-storage/file-token-storage.ts:30-38`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/mcp/token-storage/file-token-storage.ts#L30-L38));
  decryptable given hostname and username. Legacy `mcp-oauth-tokens.json` path still defined
  ([`storage.ts:205-206`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/config/storage.ts#L205-L206)).
- `extensions/<name>/.qwen-extension-git-credentials.json` is a selector file; the secret goes to
  the keyring service `Qwen Code Extension Git Credentials`
  ([`core/src/extension/extension-git-credentials.ts:19-22`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/extension/extension-git-credentials.ts#L19-L22),[`66-67`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/extension/extension-git-credentials.ts#L66-L67),[`152`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/extension/extension-git-credentials.ts#L152)).
- `settings.json` `modelProviders[].apiKey` and `envKey`
  ([`core/src/models/modelsConfig.ts:725`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/models/modelsConfig.ts#L725),[`926`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/models/modelsConfig.ts#L926));
  `.env` in `~/.qwen/.env` or `~/.env`
  ([`environment.ts:192-195`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/cli/src/config/environment.ts#L192-L195),[`224-239`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/cli/src/config/environment.ts#L224-L239)).
  Channel credentials under `channels/`: the QQ bot adapter writes `appId` and `appSecret` to
  `channels/<channel name>-credentials.json`
  ([`channels/qqbot/src/accounts.ts:13-15`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/channels/qqbot/src/accounts.ts#L13-L15),[`44-52`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/channels/qqbot/src/accounts.ts#L44-L52)),
  and the WeChat adapter keeps its `token` in `channels/weixin/account.json` (or under
  `WEIXIN_STATE_DIR`,
  [`channels/weixin/src/accounts.ts:21-40`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/channels/weixin/src/accounts.ts#L21-L40));
  the secret globs `.qwen/channels/*-credentials.json` and `.qwen/channels/weixin/account.json`
  match those two. The other routing, cursor, pairing and history files under `channels/` hold no
  credential:
  `sessions.json` and `daemon/<hash>/routes.json` (the routing tables a parser reads,
  [`cli/src/commands/channel/runtime.ts:43-60`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/cli/src/commands/channel/runtime.ts#L43-L60)),
  `cron.json`, the email adapter's per-workspace `state.json` (IMAP positions and reply routes, no
  password,
  [`channels/email/src/state.ts:29-36`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/channels/email/src/state.ts#L29-L36))
  and the polling adapters' `<name>-<hash>-poll-cursor.json`
  ([`channels/base/src/PollingChannelBase.ts:128-135`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/channels/base/src/PollingChannelBase.ts#L128-L135)).
  The `~/.qwen/channels/dingtalk-groups.json` named in a design document is a proposed store of
  group conversation ids, not a credential, and no writer exists at this commit. Platform bot tokens
  set in `settings.json` are covered by that file's glob; among them the GitHub and GitLab
  channels' personal access tokens, the `token` field of a `channels` entry
  ([`channels/github/src/index.ts:13-20`](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/channels/github/src/index.ts#L13-L20), [`channels/gitlab/src/index.ts:14-21`](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/channels/gitlab/src/index.ts#L14-L21)), which the
  GitHub channel may replace with the token of the host's own `gh` login
  ([`channels/github/src/GithubAdapter.ts:76-90`](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/channels/github/src/GithubAdapter.ts#L76-L90); [Source host rows](#source-host-rows)).
- `sessions/<pid>.json`, the session registry ([Session registry](#session-registry)): its
  `ipcToken` admits a connection to a running session's socket, so the secret glob
  `.qwen/sessions/*.json` flags every record.

### Exclusions

`.qwen/updates`, `.qwen/extension-store`, `.qwen/bin`,
`.qwen/scratch-workspaces`, `.qwen/extensions/*/node_modules`, `.qwen/arena`
(parallel agent working copies), `.qwen/artifacts` (published build
outputs), `.qwen/resources`, `.qwen/startup-perf`, `.qwen/locales` (user locale overrides) and
`.qwen/sandbox` (no writer found at either commit reviewed). `.qwen/worktrees`, the agent's git
worktrees inside a project ([Scratch workspaces and worktrees](#scratch-workspaces-and-worktrees)),
is excluded as a copy of the repository. Keep `file-history/`
(pre-edit file copies: evidence), `audits/`, `debug/`, `boards/`.

### Project-local files

`QWEN.md`
([`core/src/utils/memory-constants.ts:7`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/utils/memory-constants.ts#L7)),
`.qwenignore`
([`core/src/utils/qwenIgnoreParser.ts:13`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/utils/qwenIgnoreParser.ts#L13)),
`.qwen/settings.json`
([648](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/config/storage.ts#L648)),
`.qwen/commands`
([652](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/config/storage.ts#L652)),
`.qwen/workflows`
([661](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/config/storage.ts#L661)),
`.qwen/extensions/qwen-extension.json`
([746-750](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/config/storage.ts#L746-L750)),
`.qwen/skills` and `.agents/skills`
([17](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/config/storage.ts#L17)),
`.qwen/batch`
([`cli/src/commands/batch-workflow.ts:486`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/cli/src/commands/batch-workflow.ts#L486)).
No per-project database.

At
[`5dcf7c7`](https://github.com/QwenLM/qwen-code/commit/5dcf7c7cb162e825d95d531081f67ee60da3c0e1),
`.qwen/settings.json` in a project also holds the "always allow" rules the user saved for that
project, under `permissions.allow`
([Permission lists in settings.json](#permission-lists-in-settingsjson)).
`.qwen/worktrees/<slug>` is a scratch directory: a git worktree the `enter_worktree` tool creates
([`core/src/tools/enter-worktree.ts:29`](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/core/src/tools/enter-worktree.ts#L29)).
No exclusion covers it, so the `qwen-code|.qwen` project entry collects each worktree checkout in
full. `.qwen/memory/` holds the auto-memory when `QWEN_CODE_MEMORY_LOCAL=1` is set
([`core/src/memory/paths.ts:116-148`](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/core/src/memory/paths.ts#L116-L148)).

### Where the project path is recorded

- `projects/<sanitized>/chats/*.jsonl`: first record has `cwd`
  ([`sessionService.ts:154`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/services/sessionService.ts#L154),[`2757`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/services/sessionService.ts#L2757),[`2835`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/services/sessionService.ts#L2835),[`3975`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/services/sessionService.ts#L3975)).
  The directory name is lossy (`/` and `.` both become `-`), so the record is the source.
- `trustedFolders.json`: keys are folder paths.
- `tmp/<sha256>` is one-way; match it by hashing discovered paths.
- `sessions/<pid>.json`: `cwd` beside `sessionId`, for a session that is running or that exited
  without removing its record ([Session registry](#session-registry)).
- `usage_record.jsonl`: `project` beside `sessionId`
  ([`core/src/services/usageHistoryService.ts:37-43`](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/core/src/services/usageHistoryService.ts#L37-L43)),
  kept after the session's deletion ([usage_record.jsonl](#usage_recordjsonl)).
- `chats/<sessionId>.worktree.json`: `originalCwd`, the project, and `worktreePath`, the scratch
  worktree inside it ([Scratch workspaces and worktrees](#scratch-workspaces-and-worktrees)).

## Parsing

### Transcript stores

Base directory `~/.qwen`, overridable by `$QWEN_HOME`
([`core/src/config/storage.ts:193-202`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/config/storage.ts#L193-L202))
or the `runtimeOutputDir` setting
([128-130](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/config/storage.ts#L128-L130)).

- Session: `~/.qwen/projects/<sanitizeCwd(cwd)>/chats/<sessionId>.jsonl`, one file per session
  ([storage.ts:619-623](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/config/storage.ts#L619-L623);
  [`utils/paths.ts:388-392`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/utils/paths.ts#L388-L392):
  every character outside `[a-zA-Z0-9]` becomes `-`, Windows lower-cased;
  [`services/chatRecordingService.ts:1305`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/services/chatRecordingService.ts#L1305),[`1318-1321`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/services/chatRecordingService.ts#L1318-L1321)).
  `sessionId` is a UUID; listing accepts `/^[0-9a-fA-F-]{32,36}\.jsonl$/`
  ([`services/sessionService.ts:459`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/services/sessionService.ts#L459)).
  Archived sessions move to `chats/archive/`
  ([sessionService.ts:949-951](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/services/sessionService.ts#L949-L951)).
- Sidecars in the same directory: `<sessionId>.runtime.json`
  ([storage.ts:733-740](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/config/storage.ts#L733-L740);
  snake_case keys, `startedAt` epoch seconds,
  [`utils/runtimeStatus.ts:46-60`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/utils/runtimeStatus.ts#L46-L60)),
  `<sessionId>.worktree.json`
  ([`worktreeSessionService.ts:38`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/services/worktreeSessionService.ts#L38)),
  `<sessionId>.ledger.jsonl` prompt ledger
  ([sessionService.ts:1043-1046](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/services/sessionService.ts#L1043-L1046)).
- `~/.qwen/tmp/<sha256(cwd)>/`: `logs.json`, `checkpoint-<tag>.json`, `checkpoints/`,
  `shell_history`, `tool-results/`
  ([storage.ts:625-633](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/config/storage.ts#L625-L633),[742-744](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/config/storage.ts#L742-L744),[772-774](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/config/storage.ts#L772-L774);
  [`core/logger.ts:14`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/core/logger.ts#L14),[`203-204`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/core/logger.ts#L203-L204),[`693`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/core/logger.ts#L693);
  [`paths.ts:368-373`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/utils/paths.ts#L368-L373)).
- Managed-engine sessions: same `.jsonl` location plus `~/.qwen/resources/<sessionId>/`
  ([`utils/sessionStorageUtils.ts:806-816`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/utils/sessionStorageUtils.ts#L806-L816)).
- Channel routes: `~/.qwen/channels/sessions.json` and the daemon's
  `~/.qwen/channels/daemon/<workspace hash>/routes.json`; see [Channel routes](#channel-routes).
- Past use beyond the transcript: see [Other stores](#other-stores).

#### What survives a delete

At
[`5dcf7c7`](https://github.com/QwenLM/qwen-code/commit/5dcf7c7cb162e825d95d531081f67ee60da3c0e1),
removing a session first salvages its usage into `~/.qwen/usage_record.jsonl`
([`sessionService.ts:3347-3358`](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/core/src/services/sessionService.ts#L3347-L3358),
[3440-3452](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/core/src/services/sessionService.ts#L3440-L3452)),
then deletes the transcript, its worktree and pull-request sidecars, its prompt ledger, its
file-history backups and its managed resources directory `resources/<sessionId>`
([`sessionService.ts:3539-3558`](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/core/src/services/sessionService.ts#L3539-L3558));
archiving moves a session to `chats/archive/` instead. Unlike
[Gemini CLI](gemini-cli.md#what-survives-a-delete) there is no age-based retention:
`sessionRetention` and `maxAge` occur in no session code. The per-project `logs.json` under
`~/.qwen/tmp/<sha256(cwd)>/`, which holds the user's prompts and model switches, is not touched, and
neither are `shell_history`, the `/chat save` checkpoints (`checkpoint-<tag>.json`) or the managed
auto-memory documents, which carry no session id or time. The tool checkpoints in `checkpoints/`
are gone after the next start ([shell_history and checkpoints](#shell_history-and-checkpoints)).
`usage_record.jsonl` keeps a deleted session's summary but no text
([usage_record.jsonl](#usage_recordjsonl)). A session driven from a messaging platform also lives
on that platform, and the daemon's `routes.json` stays on disk while the foreground service's
`channels/sessions.json` is deleted on a clean shutdown. No deletion marker is written: the
surviving `usage_record.jsonl` entry and the `logs.json` prompts are all that show a removed
session. The analyzer reads the transcripts in `chats/` and `chats/archive/`, `logs.json` and
`usage_record.jsonl` as secondary sources and both channel route files, and not `shell_history`,
the checkpoints or the memory documents (see
[Secondary sources](../analyzer/docs/output.md#secondary-sources)).

#### Scratch workspaces and worktrees

- `~/.qwen/scratch-workspaces/` ([Scratch directories](#scratch-directories)): a transcript whose
  `cwd` is under it lies under the home catalog entry `.qwen`, so the analyzer's catalog rule lists
  it under `scratch_directories`.
- `<project>/.qwen/worktrees/<slug>` ([Project-local files](#project-local-files)): recorded in
  `chats/<sessionId>.worktree.json` as `{slug, worktreePath, worktreeBranch, originalCwd,
  workspaceCwd?, originalBranch, originalHeadCommit, supersededBy?, supersedes?}`
  ([`core/src/services/worktreeSessionService.ts:38-80`](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/core/src/services/worktreeSessionService.ts#L38-L80)).
  Records written inside it carry the worktree as `cwd`. No rule classes it as scratch.

### Record schema

#### ChatRecord

Every line is a `ChatRecord`
([`services/chatRecordingService.ts:336-521`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/services/chatRecordingService.ts#L336-L521)):
`uuid`, `parentUuid: string|null`, `sessionId`, `timestamp` (ISO 8601,
[1490](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/services/chatRecordingService.ts#L1490)),
`type: 'user'|'assistant'|'tool_result'|'system'`, `subtype?` (36 values,
[354-389](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/services/chatRecordingService.ts#L354-L389),
including `chat_compression`, `slash_command`, `custom_title`, `session_model`, `rewind`,
`turn_result`, `branch_checkpoint`, `managed_session_header_v1`), `provenance?`
([270-276](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/services/chatRecordingService.ts#L270-L276)),
`cwd`, `version` (CLI version,
[1501](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/services/chatRecordingService.ts#L1501)),
`gitBranch?`
([1502-1510](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/services/chatRecordingService.ts#L1502-L1510)
via `getGitBranch(cwd)`), `promptId?`, `daemonPromptId?`, `message?: Content` (`{role, parts}`),
`usageMetadata?`, `model?`, `contextWindowSize?`,
`toolCallResult?: Partial<ToolCallResponseInfo> & {status?}`, `systemPayload?`, `agentId?`,
`agentName?`, `isSidechain?`, `agentRunId?`, `agentRound?`, `forkedFrom?: {sessionId, messageUuid}`.
Base fields are set at
[1474-1504](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/services/chatRecordingService.ts#L1474-L1504);
`parentUuid` is the previous record's uuid.

- user: `message = {role:'user', parts:[{text}]}`, optional `systemPayload: UserPromptRecordPayload
  {displayText, hookContext, attachmentReferences?, resourceLinks?}`
  ([542-557](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/services/chatRecordingService.ts#L542-L557)),
  `promptId`
  ([2281-2304](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/services/chatRecordingService.ts#L2281-L2304)).
- assistant: `model`, `message = {role:'model', parts}` whose parts carry `text`, `thought: true`
  text, and `functionCall {id, name, args}`; `usageMetadata`, `contextWindowSize`
  ([2575-2611](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/services/chatRecordingService.ts#L2575-L2611)).
- tool_result: `message = {role:'user', parts:[{functionResponse:{id, name, response}}]}` plus
  `toolCallResult {callId, responseParts, resultDisplay, error, errorType, executionStatus,
  contentLength, status}`
  ([`core/turn.ts:219-246`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/core/turn.ts#L219-L246);
  status values
  [`core/coreToolScheduler.ts:710`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/core/coreToolScheduler.ts#L710),
  the same seven strings as Gemini CLI's `CoreToolCallStatus`,
  [Record schema in `gemini-cli.md`](gemini-cli.md#record-schema))
  ([2741-2840](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/services/chatRecordingService.ts#L2741-L2840)).
- system payloads of interest: `ChatCompressionRecordPayload {info, compressedHistory: Content[],
  promptIds?}`
  ([610-621](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/services/chatRecordingService.ts#L610-L621)),
  `SlashCommandRecordPayload {phase, rawCommand, sentToModel?}`
  ([628-644](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/services/chatRecordingService.ts#L628-L644)),
  `SessionModelRecordPayload {modelId, authType, baseUrl?}`
  ([703-708](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/services/chatRecordingService.ts#L703-L708)),
  `TurnResultRecordPayload {promptId, state, startedAt?, endedAt, promptText?, resultText?}`
  ([949-965](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/services/chatRecordingService.ts#L949-L965)),
  `RewindRecordPayload {truncatedCount}`
  ([837-839](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/services/chatRecordingService.ts#L837-L839)).

#### logs.json and checkpoints

`logs.json` adds `type: 'model_switch'` with `message` = JSON
`ModelSwitchEvent {fromModel, toModel, reason, context?}`
([logger.ts:16-33](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/core/logger.ts#L16-L33)).
`checkpoint-<tag>.json` is a raw `Content[]` array
([logger.ts:726](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/core/logger.ts#L726)),
the shape Gemini CLI wrote before it moved to `{history, authType}`
([Format versions in `gemini-cli.md`](gemini-cli.md#format-versions)).

#### Permission records

These records are present at
[`5dcf7c7`](https://github.com/QwenLM/qwen-code/commit/5dcf7c7cb162e825d95d531081f67ee60da3c0e1).
`CRS` below is
[`core/src/services/chatRecordingService.ts`](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/core/src/services/chatRecordingService.ts)
at that commit.

Per-call permission. A `system` record with subtype `ui_telemetry` holds `systemPayload.uiEvent`
([CRS 3124-3140](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/core/src/services/chatRecordingService.ts#L3124-L3140);
written for every telemetry event,
[`core/src/telemetry/loggers.ts:172-175`](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/core/src/telemetry/loggers.ts#L172-L175)).
For a tool call the event has `"event.name": "qwen-code.tool_call"` (`logToolCall` replaces the
event's own `tool_call` with the `EVENT_TOOL_CALL` constant before recording it,
[`core/src/telemetry/loggers.ts:317-331`](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/core/src/telemetry/loggers.ts#L317-L331);
[`core/src/telemetry/constants.ts:11`](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/core/src/telemetry/constants.ts#L11)),
`call_id`, `function_name`, `function_args`, `status` (`success`, `error`, `cancelled`) and
`decision`, set only when the user was asked: `accept` (allow once), `auto_accept` (any "allow
always" choice), `modify` (edited first) or `reject`
([`core/src/telemetry/types.ts:182-245`](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/core/src/telemetry/types.ts#L182-L245);
[`core/src/telemetry/tool-call-decision.ts:9-36`](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/core/src/telemetry/tool-call-decision.ts#L9-L36)).

Approval mode. Every user prompt record, and every `mid_turn_user_message`, carries
`executionContext: {modelId, authType?, approvalMode}`
([CRS 475-479](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/core/src/services/chatRecordingService.ts#L475-L479),
[2329-2335](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/core/src/services/chatRecordingService.ts#L2329-L2335),
[2356-2359](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/core/src/services/chatRecordingService.ts#L2356-L2359),
[2420-2422](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/core/src/services/chatRecordingService.ts#L2420-L2422)),
`approvalMode` one of `plan`, `default`, `auto-edit`, `auto`, `yolo`
([`core/src/config/approval-mode.ts:7-13`](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/core/src/config/approval-mode.ts#L7-L13)).
`auto`, the AUTO mode in which a model classifier decides each call, is not among Gemini CLI's
approval modes ([Other stores in `gemini-cli.md`](gemini-cli.md#other-stores)).
A daemon session also writes a `session_approval_mode` record,
`systemPayload {mode, prePlanMode?, planExecutionMode?}`, whenever the mode changes
([CRS 749-754](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/core/src/services/chatRecordingService.ts#L749-L754),
[3516-3545](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/core/src/services/chatRecordingService.ts#L3516-L3545)).

### Joins and timestamps

Session = file = `sessionId` on every record; `cwd` and `gitBranch` on every record (the lister
reads the first record,
[sessionService.ts:1184-1197](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/services/sessionService.ts#L1184-L1197)).
Tool call `message.parts[].functionCall.id` joins `tool_result.toolCallResult.callId` and
`message.parts[].functionResponse.id`. Turns group by `promptId`, the tree by `parentUuid`,
subagents by `isSidechain` and `agentId`, forks by `forkedFrom`.

### Other stores

No SQLite or binary store; all stores are JSON or JSONL text.

The past-use stores beside the transcript are described at
[`5dcf7c7`](https://github.com/QwenLM/qwen-code/commit/5dcf7c7cb162e825d95d531081f67ee60da3c0e1);
of them `logs.json` and `usage_record.jsonl` are parsed.

#### logs.json

`logs.json` (secondary, parsed): user prompts and model switches. The session delete does not touch
it (`logs.json` and `getProjectTempDir` do not occur in
[`core/src/services/sessionService.ts`](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/core/src/services/sessionService.ts)).

#### usage_record.jsonl

`~/.qwen/usage_record.jsonl` (secondary, parsed): one JSON object per session, `{version: 1,
sessionId, timestamp, startTime, project, durationMs, totalLatencyMs?, models: {<model id>:
{requests, inputTokens, outputTokens, cachedTokens, thoughtsTokens, totalTokens}}, tools:
{totalCalls, totalSuccess, totalFail, byName: {<tool>: {count, success, fail}}}, files: {linesAdded,
linesRemoved}, skills?}`, with epoch-millisecond times, last record per `sessionId` winning
([`core/src/services/usageHistoryService.ts:37-77`](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/core/src/services/usageHistoryService.ts#L37-L77),
[140](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/core/src/services/usageHistoryService.ts#L140)).
A record is appended when a session exits with model activity and at `/clear`, with `project` the
project root
([`cli/src/llm.tsx:1200-1216`](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/cli/src/llm.tsx#L1200-L1216),
[`cli/src/ui/commands/clearCommand.ts:80-98`](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/cli/src/ui/commands/clearCommand.ts#L80-L98)),
and a session's usage is salvaged into this file before its transcript is deleted, with `project`
the first record's `cwd`
([`core/src/services/usageHistoryService.ts:239-281`](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/core/src/services/usageHistoryService.ts#L239-L281))
([What survives a delete](#what-survives-a-delete)), so a deleted session survives here as its id,
project, start and duration, the models it used and how often it called each tool. Joins on
`sessionId`.

#### Managed auto-memory

`~/.qwen/projects/<sanitized project>/memory/` (a projection, not parsed): the managed auto-memory
documents a background agent extracts from conversations, on by default
(`memory.enableManagedAutoMemory` and `memory.enableManagedAutoDream`,
[`cli/src/config/settingsSchema.ts:2457-2476`](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/cli/src/config/settingsSchema.ts#L2457-L2476);
root at
[`core/src/memory/paths.ts:116-148`](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/core/src/memory/paths.ts#L116-L148),
or `<project>/.qwen/memory` with `QWEN_CODE_MEMORY_LOCAL=1`). No session id or time; outlives the
sessions.

#### shell_history and checkpoints

`shell_history`, `checkpoint-<tag>.json` and `checkpoints/` hold what they hold in Gemini CLI
([Other stores in `gemini-cli.md`](gemini-cli.md#other-stores)): the shell-mode commands with no
time or session id, the `/chat save` histories and the tool checkpoints (shell history at
[`core/src/config/storage.ts:772-774`](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/core/src/config/storage.ts#L772-L774));
`checkpoints/` is deleted at start unless an execution sandbox is configured
([`cli/src/utils/cleanup.ts:126-136`](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/cli/src/utils/cleanup.ts#L126-L136);
[`cli/src/llm.tsx:587`](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/cli/src/llm.tsx#L587)).

#### Channel routes

`~/.qwen/channels/sessions.json`, written by `qwen channel start`, which drives sessions from a
messaging platform (Telegram, WeChat, QQ, DingTalk, WeCom, Feishu, GitHub, GitLab or email; the
conversation then also lives on that platform's servers)
([`cli/src/commands/channel/runtime.ts:43-45`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/cli/src/commands/channel/runtime.ts#L43-L45)).
It is one JSON object keyed by routing key (`<channelName>:<chatId>` and similar forms by scope),
each value `{sessionId, target: {channelName, messageRoute?, senderId, chatId, threadId?, isGroup?},
cwd, turns?, startedAt?, isolation?, workspaceCwd?}`, `startedAt` epoch milliseconds
([`channels/base/src/SessionRouter.ts:25-36`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/channels/base/src/SessionRouter.ts#L25-L36),
[1559-1593](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/channels/base/src/SessionRouter.ts#L1559-L1593);
[`channels/base/src/types.ts:180-187`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/channels/base/src/types.ts#L180-L187)).
It is rewritten on every routing change and deleted on a clean shutdown (`clearAll`,
[SessionRouter.ts:1461-1469](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/channels/base/src/SessionRouter.ts#L1461-L1469);
[`cli/src/commands/channel/start.ts:158`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/cli/src/commands/channel/start.ts#L158)),
so it survives only a running service or a crash. The daemon-managed form writes the same shape to
`~/.qwen/channels/daemon/<workspace hash>/routes.json`
([runtime.ts:47-62](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/cli/src/commands/channel/runtime.ts#L47-L62);
[`cli/src/commands/channel/daemon-worker.ts:656-662`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/cli/src/commands/channel/daemon-worker.ts#L656-L662))
and only disposes the router on shutdown
([877](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/cli/src/commands/channel/daemon-worker.ts#L877),
[892](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/cli/src/commands/channel/daemon-worker.ts#L892)),
so that file stays. The session transcripts themselves are ordinary `chats/<sessionId>.jsonl` files.

#### Session registry

`~/.qwen/sessions/<pid>.json` (not parsed): an index of the sessions running now. Each top-level
session writes its record at start and unlinks it on exit; a process that hosts several sessions
(the `qwen --acp` child a daemon spawns) writes one `<pid>-<8 hex>.json` per session
([`core/src/services/session-registry.ts:7-20`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/services/session-registry.ts#L7-L20),
[321-333](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/services/session-registry.ts#L321-L333),
[486-491](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/services/session-registry.ts#L486-L491),
[624-653](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/services/session-registry.ts#L624-L653)).
A record left by a process that died is removed by the next listing that finds its process gone
([741-786](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/services/session-registry.ts#L741-L786)).
The record, written with mode 0600, is `{schemaVersion, pid, procStart, pidNs, sessionId, cwd, name,
startedAt, qwenVersion, kind?, ipcPath?, ipcToken?}`, `startedAt` epoch milliseconds, `kind` the
registering surface (`tui` when absent), `ipcPath` the session's peer-messaging socket and
`ipcToken` the token a connection to it must present
([104](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/services/session-registry.ts#L104),
[186-236](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/services/session-registry.ts#L186-L236)).
Joins on `sessionId`.

#### Permission lists in settings.json

At
[`5dcf7c7`](https://github.com/QwenLM/qwen-code/commit/5dcf7c7cb162e825d95d531081f67ee60da3c0e1),
`settings.json` (`~/.qwen/` and `<project>/.qwen/`) holds
`permissions.allow`, `permissions.ask` and `permissions.deny` rule lists and `permissions.autoMode`
(the AUTO classifier's hints and limits)
([`cli/src/config/settingsSchema.ts:2691-2740`](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/cli/src/config/settingsSchema.ts#L2691-L2740)),
and `tools.approvalMode`, whose default is `auto`
([3286-3300](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/cli/src/config/settingsSchema.ts#L3286-L3300)).
An "always allow" choice for the project or for the user appends its rules to `permissions.allow` of
the matching file
([`core/src/core/permission-helpers.ts:194-243`](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/core/src/core/permission-helpers.ts#L194-L243);
[`cli/src/config/config.ts:2624-2636`](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/cli/src/config/config.ts#L2624-L2636)).
The file is read with comments stripped and a byte-order mark removed
([`cli/src/config/settings.ts:921`](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/cli/src/config/settings.ts#L921)). Qwen Code has no `auto-saved.toml` policy file, where Gemini CLI keeps its saved grants
([Other stores in `gemini-cli.md`](gemini-cli.md#other-stores)).

### Format versions

`version` on every record is the CLI version. Managed-engine transcripts begin with
`subtype:'managed_session_header_v1'`
(`{formatVersion, minimumReader, sessionKey, engine:'managed', ...}`,
[`managed-runtime/managed-session-records.ts:220-229`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/managed-runtime/managed-session-records.ts#L220-L229))
followed by `managed_session_event_v1` and `commit_v1` records
([209-241](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/managed-runtime/managed-session-records.ts#L209-L241));
detector
[`sessionStorageUtils.ts:827`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/utils/sessionStorageUtils.ts#L827),[`920-958`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/utils/sessionStorageUtils.ts#L920-L958).
`logs.json` and `checkpoint-*.json` keep the formats inherited from Gemini CLI
([logs.json and checkpoints](#logsjson-and-checkpoints)).

Between the first commit and
[`5dcf7c7`](https://github.com/QwenLM/qwen-code/commit/5dcf7c7cb162e825d95d531081f67ee60da3c0e1)
(version 0.25.3) the paths did not change and the records did (`CRS` as in
[Permission records](#permission-records)); none of this has been observed on a real install:

- Record subtypes. `notification` and `cron` records (`type: "user"`,
  `provenance: "system"`,
  [CRS 2655-2690](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/core/src/services/chatRecordingService.ts#L2655-L2690))
  and the new `agent_mention` and `agent_message` records (`type: "user"`,
  `provenance: "external_agent"` unless a user mention, with `agentId` and `agentName`; messages
  from another agent of the same workspace,
  [CRS 2541-2585](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/core/src/services/chatRecordingService.ts#L2541-L2585))
  are parsed as `system` rows ([issue 172](https://github.com/seanthegeek/doubleagent/issues/172)):
  `<subtype> (provenance=<provenance> agent=<agentName>
  agentId=<agentId>): <text>`, the agent from the record's `agentId` and `agentName` or, on an
  agent's `agent_mention`, from `systemPayload.author` (`agentId`, `name`), parts not recorded left
  out, `<text>` the message parts or `systemPayload.displayText`. A `type: "user"` record is a
  `user` row only when its `provenance` is absent or `real_user` and its subtype is not
  `notification`, `cron` or `agent_message`, so the user's own `agent_mention`
  (`provenance: "real_user"`) stays a `user` row and a `goal_runtime` record
  (`provenance: "goal_runtime"`) becomes a `system` row too. New `turn_attempt` records and
  `turn_result.cancelReason`
  ([CRS 998](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/core/src/services/chatRecordingService.ts#L998),
  [3713-3734](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/core/src/services/chatRecordingService.ts#L3713-L3734))
  fall to the generic `system` row. A tool result may carry subtype `code_mode_tool_result`
  ([CRS 403](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/core/src/services/chatRecordingService.ts#L403));
  the parser reads it as an ordinary `tool_result`.
- `trustedFolders.json`: Qwen now reads the file with
  `parseJsoncObject`, which allows trailing commas and strips a byte-order mark
  ([`cli/src/utils/jsonc-editor.ts:24`](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/cli/src/utils/jsonc-editor.ts#L24),
  [92-107](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/cli/src/utils/jsonc-editor.ts#L92-L107);
  [`cli/src/config/trustedFolders.ts:172-182`](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/cli/src/config/trustedFolders.ts#L172-L182));
  how the parser reads it is in [Permission rows](#permission-rows).

### Secrets

`~/.qwen/oauth_creds.json`
([storage.ts:16](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/config/storage.ts#L16);
[`qwen/qwenOAuth2.ts:41`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/qwen/qwenOAuth2.ts#L41)),
`mcp-oauth-tokens.json`
([206](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/config/storage.ts#L206)),
`.qwen-extension-git-credentials.json`
([`extension/extension-git-credentials.ts:20`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/extension/extension-git-credentials.ts#L20)).
`session_model.systemPayload.baseUrl` is an endpoint, not a key.
`tool_result.toolCallResult.responseParts` and `message.parts[].functionResponse.response` hold raw
tool output; `UserPromptRecordPayload.hookContext` is hook stdout. `ipcToken` in a
`sessions/<pid>.json` record ([Session registry](#session-registry)) is the token that admits a
connection to the running session's socket
([`services/session-registry.ts:235`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/services/session-registry.ts#L235)).

### Parser plan

Globs (home-relative): `.qwen/projects/*/chats/*.jsonl`,
`.qwen/projects/*/chats/archive/*.jsonl`, `.qwen/tmp/*/logs.json`, `.qwen/trustedFolders.json`,
`.qwen/settings.json` (also a project's), `.qwen/usage_record.jsonl` and the channel route files.
Exclude
`*.runtime.json`, `*.worktree.json` and `*.ledger.jsonl` from transcript
parsing.

Rows: one per record; an assistant record additionally yields a `tool_use`
row per `functionCall` part and a `thinking` row per `thought:true` part.

| Column | Source |
| --- | --- |
| timestamp_utc | `timestamp` |
| session_id | `sessionId` |
| project_path | `cwd` |
| git_branch | `gitBranch` |
| turn_type | user/assistant/tool_result as-is, except a user record the user did not type (`provenance` other than `real_user`, or subtype `notification`, `cron` or `agent_message`) → system; system → system; functionCall → tool_use; thought part → thinking |
| model | `model` |
| tool_name | `functionCall.name` / `functionResponse.name` |
| tool_use_id | `functionCall.id` / `toolCallResult.callId` |
| text | text parts, `systemPayload.displayText`, `rawCommand`, `toolCallResult.status` plus `errorType` |

#### Permission rows

Permission rows ([issue 96](https://github.com/seanthegeek/doubleagent/issues/96) and
[issue 97](https://github.com/seanthegeek/doubleagent/issues/97)), added with the model rows in
[Model](#model-rows):

- `permission decision: <status>` (`system`, the result's `tool_name` and
  `tool_use_id`) after a `tool_result` row whose `toolCallResult.status`
  is `cancelled` or `awaiting_approval`; the `[<status>]` prefix on the
  result row stays.
- `permission setting: trustedFolders.<path>=<value> (from <file>, file mtime)` (`system`,
  `project_path` the folder, no session id, the file's modification time, line 1), one per key of
  `~/.qwen/trustedFolders.json`. The file is a JSON object read with comments stripped
  ([`cli/src/config/trustedFolders.ts:162`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/cli/src/config/trustedFolders.ts#L162))
  and rewritten preserving them
  ([259](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/cli/src/config/trustedFolders.ts#L259));
  keys are folder paths and values a `TrustLevel`: `TRUST_FOLDER`, `TRUST_PARENT` or `DO_NOT_TRUST`
  ([44-48](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/cli/src/config/trustedFolders.ts#L44-L48),
  [60-61](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/cli/src/config/trustedFolders.ts#L60-L61)).
  Qwen now also accepts trailing commas and a byte-order mark ([Format versions](#format-versions)),
  and the parser reads the file through the `trusted_folder_rows` reader it shares with the
  `gemini-cli` parser, which strips a byte-order mark, comments and trailing commas outside
  strings. The value is printed as recorded. A file that is still not a JSON object is one
  `parser: <file> is not a JSON object` row.

The following rows are source-derived; not observed on any real install (shapes in
[Permission records](#permission-records) and
[Permission lists in settings.json](#permission-lists-in-settingsjson)):

- Per call: a `ui_telemetry` record keeps its `ui_telemetry: <JSON>` row; when its `uiEvent` has
  `event.name` `qwen-code.tool_call` or `tool_call` and a `decision`, it is followed by
  `permission decision: <decision> (tool_call ui_telemetry)` with `tool_name` the event's
  `function_name` and `tool_use_id` its `call_id`, which join the call's `tool_use` row. Both
  spellings are matched: `logToolCall` records `qwen-code.tool_call`
  ([`core/src/telemetry/loggers.ts:317-331`](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/core/src/telemetry/loggers.ts#L317-L331)),
  while the `ToolCallEvent` class's own default is the bare `tool_call`
  ([`core/src/telemetry/types.ts:214`](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/core/src/telemetry/types.ts#L214)),
  which an older build that recorded the event without the rename may have written. The
  record is written when the call completes, so the row follows the result. No separate record of
  the AUTO classifier's verdict was found (`classifier` and `denial` do not occur in `CRS`); a
  call it blocks shows only as the tool result.
- Approval mode: `permission mode: <approvalMode>` at the first record whose `executionContext`
  names one and `permission mode: <old> -> <new>` on each change, tracked per session, before the
  record's own rows. A `session_approval_mode` record is `permission mode: <mode>` or
  `permission mode: <old> -> <mode>` in the same tracking, followed by ` prePlanMode=<mode>` and
  ` planExecutionMode=<mode>` when recorded.
- Standing rules: one `permission setting: permissions.<list>=<rule> (from .qwen/settings.json,
  file mtime)` row per `allow`, `ask` and `deny` entry and one `permission setting:
  tools.approvalMode=<mode> (from .qwen/settings.json, file mtime)`, at the file's modification
  time, line 1, no session id, and `project_path` the project for a project's
  `.qwen/settings.json`. The file is read as JSONC; no other key is read, so the provider keys and
  channel tokens beside them are never emitted. `permissions.autoMode` (the classifier's hints)
  is not read. `~/.qwen/settings.json` is a secret glob, so a `--no-secrets` collection lacks it.
  A file that is not a JSON object is one `parser: .qwen/settings.json is not a JSON object` row.

#### Model rows

- A `session_model` record is the row `model: <modelId> auth=<authType>`,
  or `model: <old> -> <modelId> auth=<authType>` when an earlier record
  of the session named another model (`model` column `modelId`).
- An assistant record's `model` yields `model: <id>` before its rows when
  no earlier `session_model` or assistant record of the session named a
  model, and `model: <old> -> <new>` when it differs from the model in
  force, tracked per `sessionId` with the record's timestamp and line.
- A `logs.json` `model_switch` entry is
  `model: <fromModel> -> <toModel> reason=<reason>` (`model` column
  `toModel`), printed from the entry, not tracked.
- `executionContext.modelId` (with `authType`) on each user prompt and `mid_turn_user_message`
  record is the model in force when the prompt was sent and feeds the same tracking, before the
  record's own rows: `model: <modelId> auth=<authType>` or `model: <old> -> <modelId>
  auth=<authType>`, the `auth=` part left out when `authType` is absent. Source-derived; not
  observed on any real install.

#### Past-use rows and secondary sources

`logs.json` is parsed as a secondary source
([Secondary sources](../analyzer/docs/output.md#secondary-sources)).

`usage_record.jsonl` is parsed as a secondary source, source-derived; not observed on any real
install. Each line is one `system` row, `session usage: project <project> duration
<durationMs>ms models <id>,<id> tools <tool>=<count>,<tool>=<count> (from usage_record.jsonl,
startTime)`, `-` for an absent part, at `startTime`, with `session_id` the record's `sessionId`
and `project_path` its `project`. No transcript row repeats it, so it is always kept, and for a
deleted session it is the only row ([What survives a delete](#what-survives-a-delete)).

Proposed, not parsed: `shell_history` and `checkpoint-<tag>.json` as proposed for Gemini CLI
([Parser plan in `gemini-cli.md`](gemini-cli.md#parser-plan));
the auto-memory directory after a real-install round ([Other stores](#other-stores)).

#### Sync, relay and origin rows

The inherited Gemini CLI features are covered in
[Sync, relay and origin rows of `gemini-cli.md`](gemini-cli.md#sync-relay-and-origin-rows); Qwen
Code changed them as follows, at the commit in [Source and evidence](#source-and-evidence).

- The Clearcut logger and Conseca are gone (neither name occurs in `packages/core/src` or
  `packages/cli/src`). Usage statistics go instead to Alibaba Cloud RUM (Real User Monitoring) at
  `gb4w8c3ygj-default-sea.rum.aliyuncs.com`, on by default under `privacy.usageStatisticsEnabled`
  ([`core/src/telemetry/qwen-logger/qwen-logger.ts:80`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/telemetry/qwen-logger/qwen-logger.ts#L80),
  [215-217](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/telemetry/qwen-logger/qwen-logger.ts#L215-L217);
  [`cli/src/config/settingsSchema.ts:1461-1466`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/cli/src/config/settingsSchema.ts#L1461-L1466)).
  The prompt event carries its id and length, not its text
  ([qwen-logger.ts:549-556](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/telemetry/qwen-logger/qwen-logger.ts#L549-L556)),
  user feedback a rating
  ([1044-1053](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/telemetry/qwen-logger/qwen-logger.ts#L1044-L1053)),
  and error text is replaced by `***REDACTED***` before an event is queued
  ([146-147](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/telemetry/qwen-logger/qwen-logger.ts#L146-L147),
  [247-262](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/telemetry/qwen-logger/qwen-logger.ts#L247-L262)).
  OpenTelemetry export is as in Gemini CLI
  ([Sync, relay and origin rows of `gemini-cli.md`](gemini-cli.md#sync-relay-and-origin-rows)): off
  by default, prompt text in the record when
  `telemetry.logPrompts` is on
  ([`core/src/config/config.ts:3398-3405`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/config/config.ts#L3398-L3405);
  [`core/src/telemetry/loggers.ts:282-283`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/telemetry/loggers.ts#L282-L283)).
- There is no Code Assist backend and no `/privacy` opt-in; `chat.qwen.ai` is used for OAuth only
  ([`core/src/qwen/qwenOAuth2.ts:28-31`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/core/src/qwen/qwenOAuth2.ts#L28-L31)).
- `/export [md|html|json|jsonl] [path]` writes `qwen-code-export-<ISO time>.<ext>` to the working
  directory or the path given; it sends nothing
  ([`cli/src/ui/commands/exportCommand.ts:78-100`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/cli/src/ui/commands/exportCommand.ts#L78-L100),
  [492-500](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/cli/src/ui/commands/exportCommand.ts#L492-L500);
  [`cli/src/ui/utils/export/utils.ts:10-13`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/cli/src/ui/utils/export/utils.ts#L10-L13)).
  `/bug` opens a GitHub issue URL and writes no history file
  ([`cli/src/ui/commands/bugCommand.ts:33-47`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/cli/src/ui/commands/bugCommand.ts#L33-L47)).
- New: channels. A session driven by `qwen channel start` is conducted on a messaging platform, so
  its prompts and replies sit on that platform's servers as well as in the local transcript. The
  marker is the channel routes file in [Channel routes](#channel-routes): an entry names the
  `sessionId` and the platform's `channelName`, `chatId`, `senderId` and `threadId`. The parser
  yields one `system` row per entry, with `session_id` the entry's `sessionId` and `project_path`
  its `cwd`: `session relayed: channel <channelName> chat <chatId or -> sender <senderId or ->
  thread <threadId or -> (from channels/sessions.json, startedAt)`, at `startedAt` when present and
  otherwise at the file's modification time with the text ending `file mtime)`, and
  `channels/daemon/<hash>/routes.json` named in place of `channels/sessions.json` for the daemon
  form. The row is `session relayed:`, not `session synced:`: the copy is on the messaging platform,
  not in a Qwen account, so the parser does not set `records_sync`. A file or an entry that is not a
  JSON object is one `parser:` row. Fixture only: no real install has shown either file. A clean
  shutdown deletes `sessions.json`, so a session whose entry is gone may still have been a channel
  session; the bot tokens are in the `channels` key of `settings.json` or in environment variables
  it names
  ([`docs/users/features/channels/overview.md:28-36`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/docs/users/features/channels/overview.md#L28-L36),
  [254-258](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/docs/users/features/channels/overview.md#L254-L258)).
  The sessions that remain on the platform are deleted, if at all, on the platform, and no local
  file records that.
- New: the Mem0 external-context extension
  (`integrations/external-context-mem0`, Mem0 being a memory service
  an administrator configures) sends the submitted prompt as a search
  query when its Auto Recall hook is installed, and an opt-in
  `context_remember` tool writes content to that service; both are
  off unless an administrator installs and configures them, and a
  write appears in the transcript only as that tool call. No other
  local marker records it.
- Qwen Live (`packages/qwen-live`, a voice daemon that drives coding sessions) keeps its own logs
  and memory under `~/.qwen-live`
  ([`qwen-live/src/config.ts:867-869`](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/qwen-live/src/config.ts#L867-L869));
  it streams audio to the DashScope realtime model (Alibaba Cloud's model API), which is the model
  API, not sync.
- Not reviewed in depth: the Qwen Code Desktop app and its Local Control setting, which shares the
  live daemon with a phone on the same network ([Qwen Code Desktop](#qwen-code-desktop)); whether
  either leaves a local record of a paired device was not determined.

#### Scratch directory rows

A record whose `cwd` lies under
`<project>/.qwen/worktrees/`, or a `.worktree.json` sidecar, should
yield `scratch directory: <worktreePath> (enter_worktree <slug>)`. The paths are in
[Scratch workspaces and worktrees](#scratch-workspaces-and-worktrees).

#### Deletion rows

No row; nothing marks a removed session except its
surviving `usage_record.jsonl` entry and `logs.json` prompts. See
[What survives a delete](#what-survives-a-delete).

#### Bad-line rows

Each line of a transcript that does not decode becomes its own `parser: unparseable line` row.
`logs.json`, a JSON array, is decoded record by record: the records before a cut are yielded and the
cut is one `unparseable record <N>` row. A channel routes file or a `trustedFolders.json` that is
not a JSON object is one whole-file `parser:` row, and a routes entry that is not an object is its
own row ([Channel routes](#channel-routes), [Permission rows](#permission-rows)).
Each line of `usage_record.jsonl` that does not decode is its own `parser: unparseable line` row,
and the lines before and after it are still read. A `settings.json` that is not a JSON object is
one whole-file `parser:` row. A `settings.json` or `trustedFolders.json` that cannot be read is one
`parser: unreadable <file>: <reason>` row, distinct from the `parser: <file> is not a JSON object`
of a file that reads but decodes to something else.

A transcript or `usage_record.jsonl` that cannot be read is one whole-file row
`parser: <file> could not be read (<reason>)` through `error_rows`, `<file>` its name.

#### Fixtures

Fixture:

```json
{"uuid":"0f0e0d0c-3333-4000-8000-000000000010","parentUuid":null,"sessionId":"e5f6a7b8-4444-4000-8000-000000000011","timestamp":"2026-10-01T10:00:00.000Z","type":"user","provenance":"real_user","cwd":"/home/alice/proj","version":"0.24.7","gitBranch":"main","promptId":"e5f6a7b8-4444-4000-8000-000000000011########1","message":{"role":"user","parts":[{"text":"run the tests"}]},"systemPayload":{"displayText":"run the tests","hookContext":""}}
{"uuid":"1a1b1c1d-5555-4000-8000-000000000012","parentUuid":"0f0e0d0c-3333-4000-8000-000000000010","sessionId":"e5f6a7b8-4444-4000-8000-000000000011","timestamp":"2026-10-01T10:00:03.000Z","type":"assistant","provenance":"assistant_output","cwd":"/home/alice/proj","version":"0.24.7","gitBranch":"main","model":"qwen3-coder-plus","message":{"role":"model","parts":[{"text":"Plan: run npm test.","thought":true},{"text":"Running tests."},{"functionCall":{"id":"call_abc123","name":"run_shell_command","args":{"command":"npm test"}}}]},"usageMetadata":{"promptTokenCount":200,"candidatesTokenCount":30,"totalTokenCount":230},"contextWindowSize":131072}
{"uuid":"2b2c2d2e-6666-4000-8000-000000000013","parentUuid":"1a1b1c1d-5555-4000-8000-000000000012","sessionId":"e5f6a7b8-4444-4000-8000-000000000011","timestamp":"2026-10-01T10:00:09.000Z","type":"tool_result","provenance":"tool_result","cwd":"/home/alice/proj","version":"0.24.7","gitBranch":"main","message":{"role":"user","parts":[{"functionResponse":{"id":"call_abc123","name":"run_shell_command","response":{"output":"12 passing"}}}]},"toolCallResult":{"callId":"call_abc123","status":"success","resultDisplay":"12 passing","errorType":null}}
{"uuid":"3c3d3e3f-7777-4000-8000-000000000014","parentUuid":"2b2c2d2e-6666-4000-8000-000000000013","sessionId":"e5f6a7b8-4444-4000-8000-000000000011","timestamp":"2026-10-01T10:00:10.000Z","type":"system","subtype":"custom_title","provenance":"system","cwd":"/home/alice/proj","version":"0.24.7","gitBranch":"main","systemPayload":{"customTitle":"Run tests","titleSource":"auto"}}
```

Permission, approval-mode and usage records at
[`5dcf7c7`](https://github.com/QwenLM/qwen-code/commit/5dcf7c7cb162e825d95d531081f67ee60da3c0e1)
(a user record's `executionContext`, a `ui_telemetry` tool-call decision, a daemon's
`session_approval_mode` and a `usage_record.jsonl` line), shapes from
[Permission records](#permission-records) and [usage_record.jsonl](#usage_recordjsonl):

```json
{"uuid":"p1","parentUuid":null,"sessionId":"e5f6a7b8-4444-4000-8000-000000000044","timestamp":"2026-10-03T09:00:00.000Z","type":"user","provenance":"real_user","cwd":"/srv/proj","version":"0.25.3","executionContext":{"modelId":"qwen3-coder-plus","authType":"qwen-oauth","approvalMode":"default"},"message":{"role":"user","parts":[{"text":"clean the build"}]}}
{"uuid":"p3","parentUuid":"p2","sessionId":"e5f6a7b8-4444-4000-8000-000000000044","timestamp":"2026-10-03T09:00:09.000Z","type":"system","subtype":"ui_telemetry","cwd":"/srv/proj","version":"0.25.3","systemPayload":{"uiEvent":{"event.name":"qwen-code.tool_call","event.timestamp":"2026-10-03T09:00:09.000Z","call_id":"call_p1","function_name":"run_shell_command","status":"success","decision":"auto_accept","prompt_id":"p#1"}}}
{"uuid":"p4","parentUuid":"p3","sessionId":"e5f6a7b8-4444-4000-8000-000000000044","timestamp":"2026-10-03T09:01:00.000Z","type":"system","subtype":"session_approval_mode","cwd":"/srv/proj","version":"0.25.3","systemPayload":{"mode":"plan","prePlanMode":"default"}}
{"version":1,"sessionId":"e5f6a7b8-4444-4000-8000-000000000055","timestamp":1790935200000,"startTime":1790931600000,"project":"/srv/proj","durationMs":3600000,"models":{"qwen3-coder-plus":{"requests":4,"inputTokens":100,"outputTokens":20,"cachedTokens":0,"thoughtsTokens":0,"totalTokens":120}},"tools":{"totalCalls":3,"totalSuccess":3,"totalFail":0,"byName":{"run_shell_command":{"count":2,"success":2,"fail":0},"read_file":{"count":1,"success":1,"fail":0}}},"files":{"linesAdded":4,"linesRemoved":1}}
```

#### Source host rows

The agent records no connected source-host account in a file the parser reads. The GitHub and
GitLab channels act as an account through a personal access token in a `channels` entry of
`settings.json` or, for GitHub, through the host's own `gh` login
([Credentials](#credentials)); the authenticated login is looked up at connect and kept in memory
(the GitHub channel also writes it to the daemon's standard error), never in a file
([`channels/github/src/GithubAdapter.ts:563-603`](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/channels/github/src/GithubAdapter.ts#L563-L603),
[`channels/gitlab/src/GitlabAdapter.ts:69-98`](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/channels/gitlab/src/GitlabAdapter.ts#L69-L98)).
The GitHub
channel appends a publication audit per workspace,
`~/.qwen/channels/<workspace scope>/<channel>-<hash>-github-audit.jsonl`, whose records name the
`repository`, issue or pull request `number`, `sessionId`, `actor` and `commentUrl` of each reply
it posted ([`GithubAdapter.ts:271-290`](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/channels/github/src/GithubAdapter.ts#L271-L290),
[`2064-2075`](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/channels/github/src/GithubAdapter.ts#L2064-L2075));
and a session that opened or reviewed a pull request keeps `chats/<sessionId>.pr.json`,
`{prs: [{number, url, createdAt, state?, source?, issues?}]}`
([`core/src/services/session-pr-service.ts:15-43`](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/core/src/services/session-pr-service.ts#L15-L43)).
Neither is parsed yet. No repository list is cached. Grepped for `github`, `gitlab`,
`bitbucket`, `oauth`, `installation`, `repositor`, `pull request` and `clone` at
[`5dcf7c7`](https://github.com/QwenLM/qwen-code/commit/5dcf7c7cb162e825d95d531081f67ee60da3c0e1).

## Confidence

- High: every `storage.ts`, session, token and settings path; the record field names, status
  values, ISO timestamps and sidecar names.
- Medium: the directories named only in the docs (`channels/<scope>/...`, `log/otel-*`,
  `paste-cache`); that `~/.qwen/QWEN.md` rather than `memory.md` is the live global memory file
  (the code defines `memory.md`, the docs and the memory tool's users point at `QWEN.md`); the
  unit of `turn_result.startedAt` and `endedAt` (numbers, inferred milliseconds); the serialised
  value of `errorType`.
- Not determined: whether `.qwen/sandbox` ever existed; whether `extension-secrets-v1.json` is
  used by anything other than MCP extension secrets; the exact serialisation of a `logs.json`
  `model_switch` entry; what `hookContext` holds.
