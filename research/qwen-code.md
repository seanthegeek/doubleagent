# qwen-code

Qwen Code (catalog agent `qwen-code`) is Alibaba's open-source coding agent from
[QwenLM/qwen-code](https://github.com/QwenLM/qwen-code), a fork of Gemini CLI
([`gemini-cli.md`](gemini-cli.md)) whose transcript format has fully diverged: Gemini CLI writes a
metadata line and message records patched in place, Qwen Code writes a flat, append-only record
stream with `uuid`, `parentUuid`, `cwd` and `gitBranch` on every line. It still shares with Gemini
CLI the per-project `tmp/<sha256(cwd)>/` directory and, in it, `logs.json`, `shell_history` and the
`checkpoint-<tag>.json` files, in the shapes Gemini CLI wrote before it moved on
([Format versions in `gemini-cli.md`](gemini-cli.md#format-versions)). Besides the terminal
interface (TUI) it has `qwen serve`, a daemon (a long-running background server) that serves the
Web Shell (a browser interface), Qwen Code Desktop (a Tauri app, a desktop shell around a web view,
that wraps the Web Shell), a managed mode of `qwen serve` that runs its sessions through a bundled
Java service, and messaging-platform channels. This document rests on the main branch at commit
[`835b080`](https://github.com/QwenLM/qwen-code/commit/835b080f8567d8612effa248e1fafa11af0e244b)
of 2026-11-01 (package version 0.26.4), beside the release tag `v0.26.5`. Parsed: the
`qwen-code` parser reads the transcripts, `logs.json`, `usage_record.jsonl`, `trustedFolders.json`,
`settings.json` and the channel route files ([Parser plan](#parser-plan)).

## Source and evidence

[QwenLM/qwen-code](https://github.com/QwenLM/qwen-code), Apache-2.0, open source, TypeScript with
a Rust desktop shell and a Java managed-session service, commit
[`835b080f8567d8612effa248e1fafa11af0e244b`](https://github.com/QwenLM/qwen-code/commit/835b080f8567d8612effa248e1fafa11af0e244b)
(2026-11-01). The root `package.json` at that commit gives version 0.26.4
([`package.json:3`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/package.json#L4));
the latest release tag, `v0.26.5` (commit
[`db0e4e2`](https://github.com/QwenLM/qwen-code/commit/db0e4e24a0d89b211b2a15f4af2be22adac71dfc),
published 2026-11-01), differs from it in `packages/core/src` and `packages/cli/src` only in the
language-server client, the MCP client and the lock handling of the OAuth token manager, so the
release writes the shapes described here. Every claim in the Collection and Parsing parts is from
source at the reviewed commit unless it cites another; the repository's own documentation under
`docs/` is cited at the same commit for product behaviour, and the claims about older builds in
[Format versions](#format-versions) cite
[`2c591ec`](https://github.com/QwenLM/qwen-code/commit/2c591ecc08a6fa080342f9b1b9f7f43215178cbb)
(0.24.7, 2026-10-03) or
[`5dcf7c7`](https://github.com/QwenLM/qwen-code/commit/5dcf7c7cb162e825d95d531081f67ee60da3c0e1)
(0.25.3, 2026-10-20). Paths cited are relative to the clone. Read only: nothing was installed or
run.

### Observed

- No real install. What stands in is the source review of QwenLM/qwen-code at
  [`835b080`](https://github.com/QwenLM/qwen-code/commit/835b080f8567d8612effa248e1fafa11af0e244b)
  (0.26.4, beside release `v0.26.5`). Every row of the [Parser plan](#parser-plan) is
  source-derived; the channel route rows rest on the source and the fixtures alone, as no real
  install has shown either route file ([Channel routes](#channel-routes)).

### Source reviews

- 2026-11-01: QwenLM/qwen-code at
  [`835b080`](https://github.com/QwenLM/qwen-code/commit/835b080f8567d8612effa248e1fafa11af0e244b)
  (0.26.4), every citation re-read there. Corrected the MCP OAuth token store (plaintext
  `mcp-oauth-tokens.json` by default, the encrypted file only when forced) and the delete (the
  TUI's `/delete` purges the session's `logs.json` rows); added the managed local mode's data
  directory, the Agent Host credentials, the session organization store, the monthly token usage
  log, the per-session todo files, the GitHub channel's audit and the channels' group history, the
  settings recovery copies, the home-level worktree root and the key tables; recorded the record
  subtypes added after 0.25.3.
- 2026-10-21: QwenLM/qwen-code at [`5dcf7c7`](https://github.com/QwenLM/qwen-code/commit/5dcf7c7cb162e825d95d531081f67ee60da3c0e1) (0.25.3): the per-call
  decision, approval-mode, `settings.json` permission and `usage_record.jsonl` rows moved from
  proposed to parsed, the `ui_telemetry` tool-call event name was corrected to
  `qwen-code.tool_call`, the session registry gained its secret glob, and the source-host review
  of the GitHub and GitLab channels and the pull-request sidecar was added.
- 2026-10-19: HEAD
  [`5dcf7c7`](https://github.com/QwenLM/qwen-code/commit/5dcf7c7cb162e825d95d531081f67ee60da3c0e1)
  (2026-10-20 UTC+8, version 0.25.3) against
  [`2c591ec`](https://github.com/QwenLM/qwen-code/commit/2c591ecc08a6fa080342f9b1b9f7f43215178cbb);
  `storage.ts`, `paths.ts` and the channel runtime had not changed. It added `usage_record.jsonl`,
  the managed auto-memory, the scratch workspaces and worktrees, the delete path, Qwen Code
  Desktop, the per-call and standing permission records, `executionContext` and the record
  subtypes, and the reading of `trustedFolders.json` as JSONC (JSON with comments, trailing commas
  and a byte-order mark allowed).

## Collection

### Per-user storage

The global directory, `getGlobalQwenDir()`, is `~/.qwen` (`QWEN_DIR = '.qwen'`,
[`core/src/utils/paths.ts:14`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/utils/paths.ts#L14)), or `$QWEN_HOME` with `~` expanded
([`core/src/config/storage.ts:193-203`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/config/storage.ts#L193-L203)); with no home
directory it falls back to `.qwen` in the temporary directory. There is no platform branch and no
XDG lookup: the same layout sits at `~/.qwen` on Linux and macOS and `%USERPROFILE%\.qwen` on
Windows. `platformFoldsCase()` changes only path comparisons
([`storage.ts:29-31`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/config/storage.ts#L29-L31)).

A second root, the runtime base directory, defaults to the global directory and moves with
`$QWEN_RUNTIME_DIR` or the `advanced.runtimeOutputDir` setting (`runtimeOutputDir`)
([`storage.ts:166-191`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/config/storage.ts#L166-L191);
[`cli/src/config/config.ts:1863`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/config/config.ts#L1863)). It holds `tmp/`, `projects/`
and `debug/` ([`storage.ts:229-238`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/config/storage.ts#L229-L238),
[`619-633`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/config/storage.ts#L619-L633)) and the runtime directories marked "runtime
base" below.

Files and directories of the global directory (paths relative to it):

| Path | Holds | Source |
| --- | --- | --- |
| `settings.json` | user settings: permissions, providers, channels, MCP servers, environment ([Credentials](#credentials)) | [`storage.ts:209-210`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/config/storage.ts#L209-L210) |
| `installation_id` | an installation identifier | [`storage.ts:213-214`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/config/storage.ts#L213-L214) |
| `google_accounts.json` | path defined, no writer found | [`storage.ts:15`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/config/storage.ts#L15), [`217-218`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/config/storage.ts#L217-L218) |
| `QWEN.md` | the user's global instructions (`~/.qwen/QWEN.md`), written by the memory tool | [`core/src/memory/writeContextFile.ts:243-247`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/memory/writeContextFile.ts#L243-L247); [`core/src/utils/memory-constants.ts:7`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/utils/memory-constants.ts#L7) |
| `memory.md` | path defined, no caller found | [`storage.ts:225-226`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/config/storage.ts#L225-L226) |
| `commands/` | user slash commands | [`storage.ts:221-222`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/config/storage.ts#L221-L222) |
| `agents/` | user subagent definitions | [`core/src/subagents/subagent-manager.ts:1833`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/subagents/subagent-manager.ts#L1833) |
| `skills/` (and `~/.agents/skills`) | user skills | [`storage.ts:17`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/config/storage.ts#L17), [`754-761`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/config/storage.ts#L754-L761) |
| `rules/` | user rules | [`core/src/config/rulesDiscovery.ts:331`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/config/rulesDiscovery.ts#L331) |
| `output-styles/` | user output styles | [`core/src/core/output-style-files.ts:409`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/core/output-style-files.ts#L409) |
| `workflows/` | user saved-workflow scripts | [`storage.ts:670-671`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/config/storage.ts#L670-L671) |
| `extensions/` | installed extensions | [`storage.ts:768-769`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/config/storage.ts#L768-L769) |
| `extension-store/` | extension store downloads | [`core/src/extension/extension-store.ts:523`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/extension/extension-store.ts#L523) |
| `ide/` | editor companion connection files | [`storage.ts:241-245`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/config/storage.ts#L241-L245) |
| `plans/` | plan-mode plan files | [`storage.ts:298-337`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/config/storage.ts#L298-L337) |
| `bin/` | downloaded helper binaries | [`storage.ts:340-341`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/config/storage.ts#L340-L341) |
| `arena/` | parallel agent working copies (the Arena feature) | [`core/src/agents/arena/ArenaManager.ts:133-136`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/agents/arena/ArenaManager.ts#L133-L136) |
| `worktrees/<sessionId>/` | the worktree service's default root for a session's set of worktrees, with `config.json` | [`core/src/services/gitWorktreeService.ts:1061-1074`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/gitWorktreeService.ts#L1061-L1074), [`1432-1449`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/gitWorktreeService.ts#L1432-L1449) |
| `audits/<sha256>/` | code-audit output per project | [`storage.ts:370-371`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/config/storage.ts#L370-L371) |
| `sessions/<pid>.json` | the live session registry ([Session registry](#session-registry)) | [`core/src/services/session-registry.ts:321-334`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/session-registry.ts#L321-L334) |
| `usage_record.jsonl` | one usage summary per session ([usage_record.jsonl](#usage_recordjsonl)) | [`core/src/services/usageHistoryService.ts:139-141`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/usageHistoryService.ts#L139-L141) |
| `file-history/<sessionId>/` | pre-edit copies of the files a session changed, for rewind | [`core/src/services/sessionService.ts:1786-1790`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/sessionService.ts#L1786-L1790); [`core/src/services/fileHistoryService.ts:137`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/fileHistoryService.ts#L137) |
| `model-registry.json` | downloaded model catalog | [`core/src/models/model-catalog.ts:68`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/models/model-catalog.ts#L68) |
| `mcpApprovals.json` | the user's decisions on project-scoped MCP servers, each bound to a hash of the reviewed configuration | [`cli/src/config/mcpApprovals.ts:21-30`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/config/mcpApprovals.ts#L21-L30), [`55`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/config/mcpApprovals.ts#L55) |
| `trustedFolders.json` | folder trust decisions ([Permission lists and trusted folders](#permission-lists-and-trusted-folders)); `QWEN_CODE_TRUSTED_FOLDERS_PATH` moves it | [`cli/src/config/trustedFolders.ts:34-43`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/config/trustedFolders.ts#L34-L43) |
| `.env` | environment values ([Credentials](#credentials)) | [`cli/src/config/environment.ts:186-196`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/config/environment.ts#L186-L196) |
| `artifacts/<id>/` | published build outputs of the artifact tool | [`core/src/tools/artifact/local-publisher.ts:20`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/tools/artifact/local-publisher.ts#L20), [`30`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/tools/artifact/local-publisher.ts#L30) |
| `boards/` | agent team boards | [`core/src/agents/team/board-lock.ts:31-35`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/agents/team/board-lock.ts#L31-L35) |
| `teams/`, `tasks/<team>/` | agent team definitions and task lists | [`core/src/agents/team/teamHelpers.ts:39`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/agents/team/teamHelpers.ts#L39), [`71`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/agents/team/teamHelpers.ts#L71); [`core/src/agents/team/types.ts:186-189`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/agents/team/types.ts#L186-L189) |
| `batch/` | batch-task plans | [`cli/src/commands/batch-task.ts:286`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/commands/batch-task.ts#L286) |
| `tip_history.json` | which startup tips were shown | [`cli/src/services/tips/tipHistory.ts:132`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/services/tips/tipHistory.ts#L132) |
| `source.json` | the install source the usage-statistics logger reports | [`core/src/telemetry/qwen-logger/qwen-logger.ts:375-379`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/telemetry/qwen-logger/qwen-logger.ts#L375-L379) |
| `startup-perf/`, `cpu-profiles/`, `memory-snapshots/` | startup profiles, CPU profiles and heap snapshots, written on request | [`cli/src/utils/startupProfiler.ts:339`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/utils/startupProfiler.ts#L339); [`cli/src/utils/cpuProfiler.ts:281-283`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/utils/cpuProfiler.ts#L281-L283); [`cli/src/utils/memoryDiagnostics.ts:138-140`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/utils/memoryDiagnostics.ts#L138-L140) |
| `updates/npm/` | the managed npm self-update | [`cli/src/utils/managed-npm-update.ts:259`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/utils/managed-npm-update.ts#L259) |
| `locales/` | user locale overrides (the bundled locales are in the install directory) | [`cli/src/i18n/index.ts:47-50`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/i18n/index.ts#L47-L50) |
| `output-language.md` | the user's rule for the language of model output | [`cli/src/i18n/languageUtils.ts:19`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/i18n/languageUtils.ts#L19), [`95-100`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/i18n/languageUtils.ts#L95-L100) |
| `channels/` | channel routes, credentials and state ([Channel routes](#channel-routes), [Credentials](#credentials)); `channels/service.pid` marks the running channel service | [`cli/src/commands/channel/runtime.ts:43-92`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/commands/channel/runtime.ts#L43-L92); [`cli/src/commands/channel/pidfile.ts:55`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/commands/channel/pidfile.ts#L55) |
| `scratch-workspaces/` | the daemon's scratch root ([Scratch directories](#scratch-directories)) | [`cli/src/serve/run-qwen-serve.ts:5394-5396`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/serve/run-qwen-serve.ts#L5394-L5396) |
| `agent-hosts/<sha256>.json` | the daemon's registration with a remote Agent Host server ([Credentials](#credentials)) | [`cli/src/serve/agent-host-client.ts:132-141`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/serve/agent-host-client.ts#L132-L141) |
| `managed-local/` | the managed local mode's data directory ([Managed local mode](#managed-local-mode)) | [`cli/src/serve/managed-local-state.ts:96-97`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/serve/managed-local-state.ts#L96-L97) |
| `desktop-relay/` | the desktop relay's home, used by the node REPL package | [`node-repl/src/desktop-relay/constants.ts:37-39`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/node-repl/src/desktop-relay/constants.ts#L37-L39) |
| `live/daemon.json` | the discovery record the Live Host app reads to reach the voice daemon | [`live-host/src/main/discovery.ts:29-32`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/live-host/src/main/discovery.ts#L29-L32) |
| `browser-use/profiles/` | browser-use discovery sockets, on Windows only | [`browser-use/src/bridge/discovery.ts:37-46`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/browser-use/src/bridge/discovery.ts#L37-L46) |

Directories of the runtime base:

| Path | Holds | Source |
| --- | --- | --- |
| `projects/<sanitized cwd>/` | the transcripts and per-project state ([Transcript stores](#transcript-stores)) | [`storage.ts:619-623`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/config/storage.ts#L619-L623) |
| `tmp/<sha256(cwd)>/` | `logs.json`, `checkpoint-<tag>.json`, `checkpoints/`, `tool-results/`, `shell_history` and `scheduled_tasks.json`, the project's scheduled tasks ([Other stores](#other-stores); [`core/src/services/cronTasksFile.ts:345`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/cronTasksFile.ts#L345), [`485-494`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/cronTasksFile.ts#L485-L494)) | [`storage.ts:625-633`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/config/storage.ts#L625-L633), [`742-744`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/config/storage.ts#L742-L744), [`772-774`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/config/storage.ts#L772-L774) |
| `debug/<sessionId>.txt` | the debug log of a session | [`storage.ts:233-238`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/config/storage.ts#L233-L238) |
| `resources/<sessionId>/` | a managed-engine session's resources | [`core/src/utils/sessionStorageUtils.ts:813-821`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/utils/sessionStorageUtils.ts#L813-L821) |
| `todos/<sessionId>.json` | the todo list of a session (`default.json` without one) | [`core/src/tools/todoWrite.ts:95-102`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/tools/todoWrite.ts#L95-L102) |
| `usage/token-usage-<YYYY-MM>.jsonl` | one record per model response ([Token usage log](#token-usage-log)) | [`core/src/services/tokenUsageService.ts:17-19`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/tokenUsageService.ts#L17-L19), [`191-200`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/tokenUsageService.ts#L191-L200) |
| `insights/insight-<date>.html` | the `/insight` usage report | [`cli/src/services/insight/generators/StaticInsightGenerator.ts:29-41`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/services/insight/generators/StaticInsightGenerator.ts#L29-L41) |
| `session-start-perf/` | session start profiles | [`core/src/core/session-start-profiler.ts:107-109`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/core/session-start-profiler.ts#L107-L109) |
| `sandbox/` | the execution sandbox's state | [`cli/src/config/execution-sandbox-config.ts:28`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/config/execution-sandbox-config.ts#L28) |
| `imported-project-memory/` | auto-memory imported from another project layout | [`core/src/memory/paths.ts:209-213`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/memory/paths.ts#L209-L213) |

In each `projects/<sanitized cwd>/` directory: `chats/` with the transcripts and their sidecars
([Transcript stores](#transcript-stores)), `session-organization.v1.json`
([Session organization](#session-organization)), `workflows/` with completed-run snapshots
`<runId>.json`, resume journals `<runId>/journal.jsonl` and generated scripts under `generated/`
([`storage.ts:673-724`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/config/storage.ts#L673-L724)), and `memory/`, the managed
auto-memory ([Managed auto-memory](#managed-auto-memory)). The user-level auto-memory is
`memories/` under the memory base directory
([`core/src/memory/paths.ts:39`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/memory/paths.ts#L39),
[`335-336`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/memory/paths.ts#L335-L336)). The sanitized name replaces every character
outside `[a-zA-Z0-9]` with `-`, lower-cased on Windows
([`paths.ts:384-391`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/utils/paths.ts#L384-L391)); the temporary directory's name is the
SHA-256 of the project root, lower-cased first on Windows
([`paths.ts:369-373`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/utils/paths.ts#L369-L373)). No shadow git repository is kept for
checkpoints (the fork has no `gitService.ts`). Named only in design documents or code comments,
with no writer found: `paste-cache/`, `computer-use/`, `attachments/`, `otel/`, `log/otel-*` and
`~/.qwen/channels/dingtalk-groups.json` (a proposed list of group conversation ids).

#### Scratch directories

`~/.qwen/scratch-workspaces/` is the managed scratch root `qwen serve` prepares for a session that
has no workspace ([`cli/src/serve/run-qwen-serve.ts:5390-5397`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/serve/run-qwen-serve.ts#L5390-L5397)).
It is excluded ([Exclusions](#exclusions)), so the files a session made there are not collected,
while the transcript that names it is. The worktrees, `<project>/.qwen/worktrees/<slug>` and
`~/.qwen/worktrees/<sessionId>/`, are in [Project-local files](#project-local-files) and the table
above; how the transcripts record them is in
[Scratch workspaces and worktrees](#scratch-workspaces-and-worktrees). The managed local mode
creates its own workspaces under `managed-local/workspaces/`
([Managed local mode](#managed-local-mode)).

#### Managed local mode

`qwen serve --managed` runs sessions through the bundled Java managed-session service on loopback
ports, keeping everything in a private data directory, `~/.qwen/managed-local/` unless
`--managed-data-dir` names another ([`cli/src/commands/serve.ts:355`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/commands/serve.ts#L355);
[Workspaces and persistent data](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/docs/users/managed-local-serve.md#workspaces-and-persistent-data)).
It holds `state.json` (the service's tokens and keys, mode 0600; [Credentials](#credentials)),
`.launcher.lock`, `logs/<service>.log`, `database/` (the embedded H2 database, a Java SQL engine
storing one file per database, `managed.mv.db`), `runtime/`, `harness/` (the runtime base of the
hosted harness process, so it holds that process's own `projects/` transcripts), `worker-home/`
(the `$QWEN_HOME` of the worker processes, a second `.qwen` layout), `workspaces/` and `legacy/`
([`cli/src/serve/managed-local-state.ts:24-37`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/serve/managed-local-state.ts#L24-L37),
[`136-147`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/serve/managed-local-state.ts#L136-L147);
[`cli/src/serve/managed-local-serve.ts:63-104`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/serve/managed-local-serve.ts#L63-L104),
[`208`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/serve/managed-local-serve.ts#L208),
[`283-292`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/serve/managed-local-serve.ts#L283-L292);
[`managed-agent-server/server/src/main/resources/application-managed-local.yml:3`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/managed-agent-server/server/src/main/resources/application-managed-local.yml#L3)).

#### Qwen Code Desktop

Qwen Code Desktop (`packages/desktop`), a Tauri app that wraps the Web Shell served by
`qwen serve`, has the identifier `com.alibaba.qwen-code` and keeps `desktop-state.json` (window and
zoom state) in Tauri's app configuration directory
([`desktop/src-tauri/tauri.conf.json:3-5`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/desktop/src-tauri/tauri.conf.json#L3-L5);
[`desktop/src-tauri/src/desktop_state.rs:177-182`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/desktop/src-tauri/src/desktop_state.rs#L177-L182)),
which Tauri places under the platform configuration directory in a folder named by the identifier
([PathResolver::app_config_dir in the Tauri documentation](https://docs.rs/tauri/2.0.0/tauri/path/struct.PathResolver.html#method.app_config_dir)):
`Library/Application Support/com.alibaba.qwen-code` on macOS, `.config/com.alibaba.qwen-code` on
Linux and `AppData/Roaming/com.alibaba.qwen-code` on Windows. Its sessions are ordinary `~/.qwen`
transcripts. Its Local Control setting shares the live daemon with a phone on the same network
([`desktop/README.md:17`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/desktop/README.md#L17)).

Qwen Live (`packages/qwen-live`), the voice daemon in the same repository, keeps its state in
`~/.qwen-live`, a directory of its own, so it is not described here
([issue 156](https://github.com/seanthegeek/doubleagent/issues/156)).

### Credentials

Every path below is home-relative and the same on every platform (`.qwen/` under the home on
Linux and macOS, under `%USERPROFILE%` on Windows), or under `$QWEN_HOME` when it is set:

| Path | Holds |
| --- | --- |
| `.qwen/oauth_creds.json` (`~/.qwen/oauth_creds.json`) | the Qwen OAuth tokens, plaintext, mode 0600 ([`core/src/qwen/qwenOAuth2.ts:41`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/qwen/qwenOAuth2.ts#L41), [`1090-1094`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/qwen/qwenOAuth2.ts#L1090-L1094), [`1256`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/qwen/qwenOAuth2.ts#L1256); [`core/src/qwen/sharedTokenManager.ts:26-27`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/qwen/sharedTokenManager.ts#L26-L27), [`664`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/qwen/sharedTokenManager.ts#L664), [`704`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/qwen/sharedTokenManager.ts#L704)) |
| `.qwen/mcp-oauth-tokens.json` | MCP servers' OAuth tokens, a plaintext JSON array, mode 0600, the default store ([`core/src/mcp/oauth-token-storage.ts:47-60`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/mcp/oauth-token-storage.ts#L47-L60), [`111-125`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/mcp/oauth-token-storage.ts#L111-L125); [`storage.ts:205-206`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/config/storage.ts#L205-L206)) |
| `.qwen/mcp-oauth-tokens-v2.json`, `.qwen/extension-secrets-v1.json` | the encrypted file store of MCP tokens and extension secrets ([`core/src/mcp/token-storage/file-token-storage.ts:31-35`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/mcp/token-storage/file-token-storage.ts#L31-L35)) |
| `.qwen/settings.json` | `security.auth.apiKey`, the `env` values, channel bot tokens under `channels`, MCP server `env` and `headers` under `mcpServers`, and the Live Voice key ([`cli/src/config/settingsSchema.ts:308`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/config/settingsSchema.ts#L308), [`324-333`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/config/settingsSchema.ts#L324-L333), [`441-451`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/config/settingsSchema.ts#L441-L451), [`3668`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/config/settingsSchema.ts#L3668), [`3705-3712`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/config/settingsSchema.ts#L3705-L3712), [`4404`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/config/settingsSchema.ts#L4404), [`4424-4430`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/config/settingsSchema.ts#L4424-L4430)) |
| `.qwen/settings.json.write-<random>/settings.json.orig` | the copy of the previous `settings.json` a failed write leaves beside it ([`cli/src/utils/write-with-backup.ts:58-77`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/utils/write-with-backup.ts#L58-L77), [`107-137`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/utils/write-with-backup.ts#L107-L137)) |
| `.qwen/.env`, `.env` (`~/.qwen/.env`, `~/.env`) | environment values, provider keys among them ([`cli/src/config/environment.ts:186-196`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/config/environment.ts#L186-L196)) |
| `.qwen/channels/<channel name>-credentials.json` (the files `.qwen/channels/*-credentials.json`) | the QQ bot adapter's `appId` and `appSecret`, mode 0600 ([`channels/qqbot/src/accounts.ts:13-14`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/channels/qqbot/src/accounts.ts#L13-L14), [`43-53`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/channels/qqbot/src/accounts.ts#L43-L53)) |
| `.qwen/channels/weixin/account.json` | the WeChat adapter's `token` (or under `WEIXIN_STATE_DIR`) ([`channels/weixin/src/accounts.ts:21-40`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/channels/weixin/src/accounts.ts#L21-L40)) |
| `.qwen/sessions/<pid>.json`, `.qwen/sessions/<pid>-<8 hex>.json` (the files `.qwen/sessions/*.json`) | the session registry, whose `ipcToken` admits a connection to a running session's socket ([Session registry](#session-registry)) |
| `.qwen/agent-hosts/<sha256>.json` | the daemon's Agent Host `hostId` and `secret`, mode 0600 ([`cli/src/serve/agent-host-client.ts:94-100`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/serve/agent-host-client.ts#L94-L100), [`189-194`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/serve/agent-host-client.ts#L189-L194)) |
| `.qwen/managed-local/state.json` | the managed local mode's `publicToken`, `signingKey`, `harnessToken`, `brokerToken`, `writerBindingKey` and `runtimeCredentialKey`, mode 0600 ([`cli/src/serve/managed-local-state.ts:24-37`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/serve/managed-local-state.ts#L24-L37), [`146-158`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/serve/managed-local-state.ts#L146-L158)) |

The same `settings.json` keys in a project's `.qwen/settings.json` make it a credential file too,
and so does the copy Qwen Code saves of a project settings file it cannot parse,
`.qwen/settings.json.corrupted` in the project
([`cli/src/config/settings.ts:1097`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/config/settings.ts#L1097),
[`1244-1262`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/config/settings.ts#L1244-L1262)).

The MCP OAuth tokens and extension secrets go to the OS keychain under the service
`qwen-code-oauth` only when `QWEN_CODE_FORCE_ENCRYPTED_FILE_STORAGE` is `true`; the hybrid store
then falls back to the encrypted file when no keychain is available or
`QWEN_CODE_FORCE_FILE_STORAGE` is `true`
([`core/src/mcp/token-storage/index.ts:12-14`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/mcp/token-storage/index.ts#L12-L14);
[`core/src/mcp/token-storage/hybrid-token-storage.ts:28-51`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/mcp/token-storage/hybrid-token-storage.ts#L28-L51)).
The encrypted file uses AES-256-GCM with a key from
`scryptSync('qwen-code-oauth', '<hostname>-<username>-qwen-code')`, so it decrypts given the host
name and user name ([`file-token-storage.ts:62-70`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/mcp/token-storage/file-token-storage.ts#L62-L70)).
An extension's `.qwen/extensions/<name>/.qwen-extension-git-credentials.json` (the selector file
`.qwen-extension-git-credentials.json`) is not a credential:
it names which store, the keychain service `Qwen Code Extension Git Credentials` or the encrypted
file, holds the git credential
([`core/src/extension/extension-git-credentials.ts:19-22`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/extension/extension-git-credentials.ts#L19-L22),
[`66-73`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/extension/extension-git-credentials.ts#L66-L73)). `.qwen/oauth_creds.lock` is
not a credential: it holds the owner id of the process refreshing the token
(`oauth_creds.lock`;
[`sharedTokenManager.ts:713`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/qwen/sharedTokenManager.ts#L713),
[`731`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/qwen/sharedTokenManager.ts#L731)). There is no OS keychain for the main Qwen
login. A `modelProviders` entry's `envKey` names the environment variable that holds a provider key,
not the key itself ([`core/src/models/types.ts:86-87`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/models/types.ts#L86-L87)).

The routing, cursor, pairing and history files under `.qwen/channels/` are not credentials: they
hold routing keys, chat and sender ids, IMAP positions and group messages, never a token
([Channel routes](#channel-routes), [Channel state files](#channel-state-files)). The GitHub
channel may use the host's own `gh` login instead of a token in `settings.json`, asking
`gh auth token` at connect and keeping the token in memory
([`channels/github/src/GithubAdapter.ts:76-92`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/channels/github/src/GithubAdapter.ts#L76-L92);
[`channels/github/src/index.ts:13-27`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/channels/github/src/index.ts#L13-L27)).

### Exclusions

- `.qwen/updates`, `.qwen/extension-store`, `.qwen/bin` and `.qwen/extensions/*/node_modules`:
  downloaded packages and binaries.
- `.qwen/arena`, `.qwen/worktrees` and, in a project, `.qwen/worktrees`: git working copies of a
  repository ([Scratch workspaces and worktrees](#scratch-workspaces-and-worktrees)); a project's
  `.qwen/worktrees` lies under the `qwen-code|.qwen` project entry.
- `.qwen/scratch-workspaces` and `.qwen/managed-local/workspaces`: scratch directories.
- `.qwen/artifacts`: published build outputs.
- `.qwen/resources`: managed-engine session resources.
- `.qwen/startup-perf`, `.qwen/session-start-perf`, `.qwen/cpu-profiles` and
  `.qwen/memory-snapshots`: profiles and heap snapshots, large and holding no conversation.
- `.qwen/locales`: user locale overrides.
- `.qwen/sandbox`: the execution sandbox's state.

`file-history/` (pre-edit copies of changed files), `audits/`, `debug/`, `boards/`, `todos/`,
`usage/`, `managed-local/database/`, `managed-local/harness/` and `managed-local/worker-home/` are
evidence and stay collected.

### Project-local files

In a project (the working directory or its git root):

- `QWEN.md`, `AGENTS.md` and `.qwen/QWEN.local.md`, the instruction files
  ([`core/src/utils/memory-constants.ts:7-8`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/utils/memory-constants.ts#L7-L8),
  [`28`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/utils/memory-constants.ts#L28)), and `.qwenignore`
  ([`core/src/utils/qwenIgnoreParser.ts:13`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/utils/qwenIgnoreParser.ts#L13)).
- `.qwen/settings.json`, which also holds the "always allow" rules saved for that project
  ([`storage.ts:648-650`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/config/storage.ts#L648-L650);
  [Permission lists and trusted folders](#permission-lists-and-trusted-folders)), and its
  `.qwen/settings.json.corrupted` copy ([Credentials](#credentials)).
- `.qwen/commands`, `.qwen/workflows`, `.qwen/agents`, `.qwen/skills` (and `.agents/skills`),
  `.qwen/rules`, `.qwen/output-styles` and `.qwen/extensions/qwen-extension.json`
  ([`storage.ts:652-662`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/config/storage.ts#L652-L662),
  [`746-751`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/config/storage.ts#L746-L751);
  [`subagent-manager.ts:1886`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/subagents/subagent-manager.ts#L1886);
  [`rulesDiscovery.ts:339`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/config/rulesDiscovery.ts#L339);
  [`output-style-files.ts:427`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/core/output-style-files.ts#L427)).
- `.qwen/batch`, batch-workflow plans
  ([`cli/src/commands/batch-workflow.ts:486`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/commands/batch-workflow.ts#L486)), and
  `.qwen/loop.md`, the loop skill's task file
  ([`core/src/skills/bundled/loop/loop-task-file.ts:208`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/skills/bundled/loop/loop-task-file.ts#L208)).
- `.qwen/PROJECT_SUMMARY.md` and `.qwen/welcome-back-state.json`, the summary `/summary` writes of
  the conversation and the welcome-back state that offers it at the next start
  ([`core/src/utils/projectSummary.ts:37-45`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/utils/projectSummary.ts#L37-L45)).
- `.qwen/team-memory/`, the team auto-memory, and `.qwen/memory/` (`<project>/.qwen/memory`), the
  auto-memory when `QWEN_CODE_MEMORY_LOCAL=1` is set
  ([`core/src/memory/paths.ts:46`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/memory/paths.ts#L46),
  [`133-146`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/memory/paths.ts#L133-L146),
  [`375-379`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/memory/paths.ts#L375-L379)).
- `.qwen/worktrees/<slug>`, a git worktree the `enter_worktree` tool creates at the repository's
  top level, and `.qwen/worktrees/agent-<7 hex>`, one the agent tool creates for a subagent run
  with worktree isolation
  ([`core/src/tools/enter-worktree.ts:29`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/tools/enter-worktree.ts#L29);
  [`core/src/services/gitWorktreeService.ts:936-958`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/gitWorktreeService.ts#L936-L958),
  [`1936-1945`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/gitWorktreeService.ts#L1936-L1945);
  [`core/src/tools/agent/agent.ts:3529`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/tools/agent/agent.ts#L3529)). Each holds a
  `.qwen-session` marker naming its owning session, kept out of commits by a rule in the
  repository's `.git/info/exclude`
  ([`gitWorktreeService.ts:39-46`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/gitWorktreeService.ts#L39-L46),
  [`265-280`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/gitWorktreeService.ts#L265-L280)).

There is no per-project database.

### Where the project path is recorded

- `projects/<sanitized>/chats/*.jsonl`: `cwd` on every record, the first record's read by the
  session list ([`core/src/services/sessionService.ts:1200-1218`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/sessionService.ts#L1200-L1218)).
  The directory name is lossy (`/` and `.` both become `-`), so the record is the source.
- `trustedFolders.json`: keys are folder paths.
- `tmp/<sha256>` is one-way; match it by hashing discovered paths.
- `sessions/<pid>.json`: `cwd` beside `sessionId` ([Session registry](#session-registry)).
- `usage_record.jsonl`: `project` beside `sessionId` ([usage_record.jsonl](#usage_recordjsonl)).
- `chats/<sessionId>.worktree.json`: `originalCwd`, the project, and `worktreePath`, the worktree
  inside it ([Scratch workspaces and worktrees](#scratch-workspaces-and-worktrees)).
- `chats/<sessionId>.runtime.json`: `work_dir` ([Transcript stores](#transcript-stores)).
- The channel route files, `channels/sessions.json` and `daemon/<hash>/routes.json`: `cwd` and
  `workspaceCwd` per route ([Channel routes](#channel-routes)).

## Parsing

### Transcript stores

Base directory `~/.qwen` or the runtime base directory
([Per-user storage](#per-user-storage)); the per-project stores of Other stores sit in
`~/.qwen/tmp/<sha256(cwd)>/`.

- Session: `~/.qwen/projects/<sanitizeCwd(cwd)>/chats/<sessionId>.jsonl` (under
  `projects/<sanitizeCwd(cwd)>/` of the runtime base), one file per session, created empty and
  appended to ([`core/src/services/chatRecordingService.ts:1424-1431`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/chatRecordingService.ts#L1424-L1431)).
  The session list accepts `/^[0-9a-fA-F-]{32,36}\.jsonl$/`
  ([`sessionService.ts:480`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/sessionService.ts#L480)). An archived session moves
  to `chats/archive/` with its sidecars
  ([`sessionService.ts:968-980`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/sessionService.ts#L968-L980),
  [`3819-3850`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/sessionService.ts#L3819-L3850)).
- Sidecars in the same directory: `<sessionId>.runtime.json`, a snapshot of the live process with
  snake_case keys ([`storage.ts:726-740`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/config/storage.ts#L726-L740);
  [`core/src/utils/runtimeStatus.ts:46-70`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/utils/runtimeStatus.ts#L46-L70));
  `<sessionId>.worktree.json`
  ([Scratch workspaces and worktrees](#scratch-workspaces-and-worktrees)); `<sessionId>.pr.json`,
  the pull requests bound to the session ([Source host rows](#source-host-rows));
  `<sessionId>.ledger.jsonl`, the append-only prompt ledger
  ([`sessionService.ts:982-1003`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/sessionService.ts#L982-L1003),
  [`1054-1067`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/sessionService.ts#L1054-L1067)).
- A managed-engine session uses the same `.jsonl` location and keeps its resources in
  `~/.qwen/resources/<sessionId>/` ([Managed-engine records](#managed-engine-records)).
- Channel routes: `~/.qwen/channels/sessions.json` and the daemon's
  `~/.qwen/channels/daemon/<workspace hash>/routes.json` ([Channel routes](#channel-routes)).
- The managed local mode's harness keeps its transcripts under
  `~/.qwen/managed-local/harness/projects/` and its workers theirs under
  `~/.qwen/managed-local/worker-home/` ([Managed local mode](#managed-local-mode)).
- Past use beyond the transcript: [Other stores](#other-stores).

#### What survives a delete

Removing a session (the TUI's `/delete`, or the daemon's delete route through the session
service) first salvages its usage into `~/.qwen/usage_record.jsonl`, then deletes the transcript,
its worktree and pull-request sidecars, its prompt ledger, its `file-history/<sessionId>/`
backups, its managed resources directory and its entry in `session-organization.v1.json`
([`sessionService.ts:3494-3503`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/sessionService.ts#L3494-L3503),
[`3664-3700`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/sessionService.ts#L3664-L3700),
[`3776-3811`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/sessionService.ts#L3776-L3811),
[`1793-1811`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/sessionService.ts#L1793-L1811)); archiving moves a session to
`chats/archive/` instead. The TUI's `/delete` also purges the session's rows from the project's
`logs.json`; a delete through the daemon or Web Shell does not, as only the TUI's delete hook calls
the purge ([`cli/src/ui/hooks/useDeleteCommand.ts:50-69`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/ui/hooks/useDeleteCommand.ts#L50-L69);
[`core/src/core/logger.ts:516-558`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/core/logger.ts#L516-L558)). Unlike
[Gemini CLI](gemini-cli.md#what-survives-a-delete) there is no age-based retention of sessions:
`sessionRetention` and `maxAge` occur in no session code.

Untouched by any delete: `shell_history`, the `/chat save` checkpoints (`checkpoint-<tag>.json`),
the managed auto-memory documents, the todo file `todos/<sessionId>.json`, the token usage log
`usage/token-usage-<YYYY-MM>.jsonl`, the debug log `debug/<sessionId>.txt` and the session's
`.qwen/PROJECT_SUMMARY.md` in the project. The tool checkpoints in `checkpoints/` are gone after
the next start ([shell_history and checkpoints](#shell_history-and-checkpoints)).
`usage_record.jsonl` keeps a deleted session's summary but no text
([usage_record.jsonl](#usage_recordjsonl)). A session driven from a messaging platform also lives
on that platform; the daemon's `routes.json` stays on disk while the foreground service's
`channels/sessions.json` is deleted on a clean shutdown. No deletion marker is written: the
surviving `usage_record.jsonl` entry, the token usage records and, after a daemon delete, the
`logs.json` prompts are all that show a removed session. The analyzer reads the transcripts in
`chats/` and `chats/archive/`, `logs.json` and `usage_record.jsonl` as secondary sources and both
channel route files, and not `shell_history`, the checkpoints, the token usage log or the memory
documents (see [Secondary sources](../analyzer/docs/output.md#secondary-sources)).

#### Scratch workspaces and worktrees

- `~/.qwen/scratch-workspaces/` ([Scratch directories](#scratch-directories)): a transcript whose
  `cwd` is under it lies under the home catalog entry `.qwen`, so the analyzer's catalog rule lists
  it under `scratch_directories`.
- `<project>/.qwen/worktrees/<slug>` ([Project-local files](#project-local-files)): recorded in
  `chats/<sessionId>.worktree.json` ([Worktree sidecar](#worktree-sidecar)). Records written
  inside it carry the worktree as `cwd`.

### Record schema

#### ChatRecord

Every transcript line is one `ChatRecord`
([`chatRecordingService.ts:374-581`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/chatRecordingService.ts#L374-L581)). The
base fields are set for every record, `provenance` from the record's `type` unless the writer sets
another, `parentUuid` the previous record's `uuid`, `timestamp` an ISO 8601 time with milliseconds
and `version` the CLI version
([`chatRecordingService.ts:1600-1630`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/chatRecordingService.ts#L1600-L1630)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `uuid` | string | | |
| `parentUuid` | string or null | | |
| `sessionId` | string | | |
| `timestamp` | string | | |
| `type` | string | `user`, `assistant`, `tool_result`, `system` | |
| `subtype` | string | `chat_compression`, `memory_catalog_snapshot`, `slash_command`, `ui_telemetry`, `at_command`, `attribution_snapshot`, `notification`, `background_task_completed`, `cron`, `mid_turn_user_message`, `custom_title`, `parent_session`, `session_source`, `session_execution_engine`, `omni_recall`, `session_model`, `session_approval_mode`, `session_workflow_mode`, `rewind`, `agent_bootstrap`, `agent_launch_prompt`, `agent_retry`, `agent_session_ready`, `file_history_snapshot`, `user_text_elements`, `session_artifact_event`, `session_artifact_snapshot`, `session_sources_snapshot`, `branch_checkpoint`, `goal_state`, `goal_runtime`, `goal_turn_end`, `code_mode_tool_result`, `realtime_message`, `turn_result`, `turn_attempt`, `managed_session_header_v1`, `managed_session_event_v1`, `managed_session_commit_v1`, `agent_mention`, `agent_message` | |
| `provenance` | string | `real_user`, `assistant_output`, `tool_result`, `execution_output`, `goal_control`, `goal_runtime`, `system`, `external_agent` | |
| `goalContext` | object | | |
| `backgroundTurn` | object | | |
| `deliveredTurn` | boolean | `true` | |
| `cwd` | string | | |
| `version` | string | | |
| `gitBranch` | string | | |
| `promptId` | string | | |
| `daemonPromptId` | string | | |
| `message` | object | | |
| `usageMetadata` | object | | |
| `model` | string | | |
| `clientInitiatedSkills` | boolean | `true` | |
| `executionContext` | object | | |
| `contextWindowSize` | number | | |
| `toolCallResult` | object | | |
| `systemPayload` | object | | |
| `agentId` | string | | |
| `agentName` | string | | |
| `agentColor` | string | | |
| `isSidechain` | boolean | | |
| `agentRunId` | string | | |
| `agentRound` | number | | |
| `externalInputKind` | string | `message`, `notification` | |
| `externalRecordKey` | string | | |
| `forkedFrom` | object | | |

`message` is the model API's `Content`, `{role, parts}`, `role` `user` or `model`; a part carries
`text`, `thought: true` beside a reasoning `text`, `functionCall` (`id`, `name`, `args`) or
`functionResponse` (`id`, `name`, `response`). `forkedFrom` is `{sessionId, messageUuid}` on every
record `/branch` copied.

The writers, by record kind:

- A user prompt is `type: "user"` with `executionContext`, the user's `message` and, when the
  front end supplied one, a `systemPayload` holding the prompt payload
  ([`chatRecordingService.ts:2544-2576`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/chatRecordingService.ts#L2544-L2576)).
  A message the user sent while tool results were being submitted is the same with subtype
  `mid_turn_user_message` ([`2619-2640`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/chatRecordingService.ts#L2619-L2640)).
- A notification or a cron-fired prompt is `type: "user"`, `provenance: "system"`, subtype
  `notification` or `cron`, with a `displayText` payload
  ([`2648-2660`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/chatRecordingService.ts#L2648-L2660),
  [`2884-2904`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/chatRecordingService.ts#L2884-L2904)). A Goal runtime message
  is `type: "user"`, subtype and `provenance` `goal_runtime`
  ([`2585-2597`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/chatRecordingService.ts#L2585-L2597)).
- An `agent_mention` or `agent_message` record, a message from or to another agent of the same
  workspace, is `type: "user"`, `provenance: "external_agent"` unless it is the user's own mention
  (no `author`, `provenance: "real_user"`); an `agent_message` also carries the author's `agentId`,
  `agentName` and `agentColor`
  ([`2765-2785`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/chatRecordingService.ts#L2765-L2785)).
- An assistant record has `model`, `message` with `role: "model"`, `usageMetadata` and
  `contextWindowSize` ([`2986-3020`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/chatRecordingService.ts#L2986-L3020)).
- A tool result is `type: "tool_result"` with the `functionResponse` in `message` and
  `toolCallResult`, optionally subtype `code_mode_tool_result`
  ([`3251-3262`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/chatRecordingService.ts#L3251-L3262);
  [`306-316`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/chatRecordingService.ts#L306-L316)).
- A `ui_telemetry` record holds `systemPayload.uiEvent`
  ([`3615-3630`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/chatRecordingService.ts#L3615-L3630);
  [Permission records](#permission-records)).
- A `session_model` or `session_approval_mode` record is written only when the value changes
  ([`4004-4025`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/chatRecordingService.ts#L4004-L4025),
  [`4080-4103`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/chatRecordingService.ts#L4080-L4103)).

`executionContext` ([`498-503`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/chatRecordingService.ts#L498-L503),
[`2526-2532`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/chatRecordingService.ts#L2526-L2532)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `modelId` | string | | |
| `authType` | string | | |
| `approvalMode` | string | `plan`, `default`, `auto-edit`, `auto`, `yolo` | |

`toolCallResult`, the tool response with a scheduler status
([`core/src/core/turn.ts:252-262`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/core/turn.ts#L252-L262);
[`core/src/core/coreToolScheduler.ts:626-739`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/core/coreToolScheduler.ts#L626-L739)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `callId` | string | | |
| `responseParts` | array | | |
| `resultDisplay` | string or object | | |
| `error` | object | | |
| `errorType` | string | | |
| `executionStatus` | string | | |
| `contentLength` | number | | |
| `persistedOutputFiles` | array | | |
| `status` | string | `validating`, `scheduled`, `error`, `success`, `executing`, `cancelled`, `awaiting_approval` | |

#### System payloads

`systemPayload` by subtype
([`chatRecordingService.ts:588-1053`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/chatRecordingService.ts#L588-L1053)).
The prompt payload of a user record
([`606-621`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/chatRecordingService.ts#L606-L621)), and the payload of a
`notification`, `cron` or `mid_turn_user_message` record
([`588-604`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/chatRecordingService.ts#L588-L604)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `displayText` | string | | |
| `hookContext` | string | | |
| `inputAnnotations` | array | | |
| `attachmentReferences` | array | | |
| `resourceLinks` | array | | |
| `backgroundTask` | object | | |

`chat_compression` ([`674-691`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/chatRecordingService.ts#L674-L691)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `info` | object | | |
| `compressedHistory` | array | | |
| `contextPrefixLength` | number | | |
| `summaryPrefixLength` | number | | |
| `promptIds` | array | | |
| `directoryContextIndices` | array | | |
| `completedToolCallIds` | array | | |

`slash_command` ([`697-714`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/chatRecordingService.ts#L697-L714)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `phase` | string | `invocation`, `result` | |
| `rawCommand` | string | | |
| `sentToModel` | boolean | | |
| `hiddenInvocation` | boolean | | |
| `outputHistoryItems` | array | | |

`custom_title` ([`738-753`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/chatRecordingService.ts#L738-L753)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `customTitle` | string | | |
| `titleSource` | string | `manual`, `auto` | |

`session_model` ([`772-777`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/chatRecordingService.ts#L772-L777)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `modelId` | string | | |
| `authType` | string | | |
| `baseUrl` | string | | |
| `isRuntime` | boolean | | |

`session_approval_mode` ([`795-799`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/chatRecordingService.ts#L795-L799)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `mode` | string | `plan`, `default`, `auto-edit`, `auto`, `yolo` | |
| `prePlanMode` | string | `plan`, `default`, `auto-edit`, `auto`, `yolo` | |
| `planExecutionMode` | string | `plan`, `default`, `auto-edit`, `auto`, `yolo` | |

`session_workflow_mode` holds `enabled` (boolean), `rewind` holds `truncatedCount` (number) and
`ui_telemetry` holds `uiEvent` (object)
([`779-781`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/chatRecordingService.ts#L779-L781),
[`906-908`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/chatRecordingService.ts#L906-L908),
[`921-924`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/chatRecordingService.ts#L921-L924)).

`turn_result` ([`1033-1053`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/chatRecordingService.ts#L1033-L1053)), times in
epoch milliseconds of the agent's clock:

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `promptId` | string | | |
| `state` | string | `completed`, `cancelled`, `error` | |
| `stopReason` | string | | |
| `error` | object | | |
| `startedAt` | number | | |
| `cancelledAt` | number | | |
| `cancelReason` | string | `user` | |
| `endedAt` | number | | |
| `promptText` | string | | |
| `promptTextTruncated` | boolean | | |
| `resultText` | string | | |
| `resultTruncated` | boolean | | |
| `resultCode` | string | | |
| `originatorClientId` | string | | |

`agent_message` and `agent_mention`
([`core/src/agents/session-agents/contract.ts:75-83`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/agents/session-agents/contract.ts#L75-L83),
[`132-161`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/agents/session-agents/contract.ts#L132-L161)); `author` is `{agentId, name,
color?, ...}`:

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `displayText` | string | | |
| `author` | object | | |
| `runId` | string | | |
| `status` | string | | |
| `mentionedAgentIds` | array | | |
| `mentionedSquadIds` | array | | |
| `error` | string | | |

#### Permission records

Per-call permission: for a tool call the `ui_telemetry` record's `uiEvent` has
`"event.name": "qwen-code.tool_call"` (`logToolCall` replaces the event's own `tool_call` with the
`EVENT_TOOL_CALL` constant before recording it)
([`core/src/telemetry/loggers.ts:176-179`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/telemetry/loggers.ts#L176-L179),
[`321-335`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/telemetry/loggers.ts#L321-L335);
[`core/src/telemetry/constants.ts:11`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/telemetry/constants.ts#L11)). Its keys
([`core/src/telemetry/types.ts:188-224`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/telemetry/types.ts#L188-L224)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `event.name` | string | | |
| `event.timestamp` | string | | |
| `call_id` | string | | |
| `function_name` | string | | |
| `function_args` | object | | |
| `duration_ms` | number | | |
| `status` | string | `success`, `error`, `cancelled` | |
| `decision` | string | `accept`, `reject`, `modify`, `auto_accept` | |
| `prompt_id` | string | | |
| `tool_type` | string | `native`, `mcp` | |
| `mcp_server_name` | string | | |

`decision` is set only when the user was asked: `accept` for allow once, `auto_accept` for any
"allow always" choice, `modify` when edited first, `reject` for a refusal
([`core/src/telemetry/tool-call-decision.ts:9-36`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/telemetry/tool-call-decision.ts#L9-L36)).
The record is written when the call completes, after its result.

Approval mode: every user prompt and `mid_turn_user_message` record carries `executionContext`
([ChatRecord](#chatrecord)); `auto` is the mode in which a model classifier decides each call
([Auto Mode in the vendor's approval mode page](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/docs/users/features/approval-mode.md#4-auto-mode---classifier-driven-approval)).
A daemon session also writes a `session_approval_mode` record whenever the mode changes. No
separate record of the classifier's verdict exists (`classifier` and `denial` do not occur in
`chatRecordingService.ts`); a call it blocks shows only as the tool result.

#### Managed-engine records

A transcript of the managed engine begins with a `system` record of subtype
`managed_session_header_v1` whose payload is `{formatVersion, minimumReader, sessionKey, engine:
"managed", definitionRef, rootSnapshotRef, createdBy, baseTranscriptProof?}`, followed by
`managed_session_event_v1` and `managed_session_commit_v1` records
([`core/src/managed-runtime/managed-session-records.ts:44-46`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/managed-runtime/managed-session-records.ts#L44-L46),
[`396-405`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/managed-runtime/managed-session-records.ts#L396-L405);
[`core/src/utils/sessionStorageUtils.ts:834`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/utils/sessionStorageUtils.ts#L834),
[`897-959`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/utils/sessionStorageUtils.ts#L897-L959)).

#### Worktree sidecar

`chats/<sessionId>.worktree.json`
([`core/src/services/worktreeSessionService.ts:39-80`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/worktreeSessionService.ts#L39-L80)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `slug` | string | | |
| `worktreePath` | string | | |
| `worktreeBranch` | string | | |
| `originalCwd` | string | | |
| `workspaceCwd` | string | | |
| `originalBranch` | string | | |
| `originalHeadCommit` | string | | |
| `supersededBy` | string | | |
| `supersedes` | string | | |

### Joins and timestamps

Session = file = `sessionId` on every record; `cwd` and `gitBranch` on every record. A tool call's
`message.parts[].functionCall.id` joins `toolCallResult.callId`,
`message.parts[].functionResponse.id` and the `ui_telemetry` event's `call_id`. Turns group by
`promptId`, the tree by `parentUuid`, subagents by `isSidechain` and `agentId`, forks by
`forkedFrom`. Record times are ISO 8601 with milliseconds; `usage_record.jsonl`, the session
registry, the channel routes and `turn_result` use epoch milliseconds, and `runtime.json` epoch
seconds. `logs.json` and the token usage log join on `sessionId`; `shell_history` and the
checkpoints carry no session id.

### Other stores

No SQLite store; all stores the CLI writes are JSON or JSONL text. The managed local mode's H2
database is the one binary store ([Managed local mode](#managed-local-mode)).

#### logs.json

`tmp/<sha256(cwd)>/logs.json` (secondary, parsed): a pretty-printed JSON array of the user's
prompts across every session of the project, which backs the prompt history
([`core/src/core/logger.ts:14-27`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/core/logger.ts#L14-L27),
[`204`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/core/logger.ts#L204)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `sessionId` | string | | |
| `messageId` | number | | |
| `timestamp` | string | | |
| `type` | string | `user`, `model_switch` | |
| `message` | string | | |

A `model_switch` entry's `message` is a JSON `ModelSwitchEvent`
([`logger.ts:29-34`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/core/logger.ts#L29-L34)); no code at the reviewed commit writes
one, as nothing passes `MessageSenderType.MODEL_SWITCH`:

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `fromModel` | string | | |
| `toModel` | string | | |
| `reason` | string | `vision_auto_switch`, `manual`, `fallback`, `other` | |
| `context` | string | | |

#### usage_record.jsonl

`~/.qwen/usage_record.jsonl` (secondary, parsed): one JSON object per session, the last record per
`sessionId` winning ([`core/src/services/usageHistoryService.ts:37-78`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/usageHistoryService.ts#L37-L78)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `version` | number | `1` | |
| `sessionId` | string | | |
| `timestamp` | number | | |
| `startTime` | number | | |
| `project` | string | | |
| `durationMs` | number | | |
| `totalLatencyMs` | number | | |
| `models` | object | | |
| `tools` | object | | |
| `files` | object | | |
| `skills` | object | | |

Times are epoch milliseconds. `models` maps a model id to `{requests, inputTokens, outputTokens,
cachedTokens, thoughtsTokens, totalTokens, totalLatencyMs?}`, `tools` is `{totalCalls,
totalSuccess, totalFail, byName}` with `byName` mapping a tool to `{count, success, fail,
totalDurationMs?}`, and `files` is `{linesAdded, linesRemoved}`. A record is appended when a
session with model activity exits and at `/clear`, with `project` the project root
([`usageHistoryService.ts:143-159`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/usageHistoryService.ts#L143-L159);
[`cli/src/llm.tsx:1240-1256`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/llm.tsx#L1240-L1256);
[`cli/src/ui/commands/clearCommand.ts:82-98`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/ui/commands/clearCommand.ts#L82-L98)), and
a session's usage is salvaged into the file before its transcript is deleted, with `project` the
first record's `cwd` ([`usageHistoryService.ts:239-246`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/usageHistoryService.ts#L239-L246),
[`282-345`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/usageHistoryService.ts#L282-L345)). A deleted session survives here
as its id, project, start and duration, the models it used and how often it called each tool.

#### Token usage log

`usage/token-usage-<YYYY-MM>.jsonl` under the runtime base (not parsed): one record per model
response, written from the API response logger
([`core/src/services/tokenUsageService.ts:26-45`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/tokenUsageService.ts#L26-L45),
[`386`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/tokenUsageService.ts#L386);
[`core/src/telemetry/loggers.ts:694`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/telemetry/loggers.ts#L694)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `schemaVersion` | number | `1` | |
| `id` | string | | |
| `timestamp` | string | | |
| `localDate` | string | | |
| `localMonth` | string | | |
| `sessionId` | string | | |
| `model` | string | | |
| `authType` | string | | |
| `source` | string | | |
| `inputTokens` | number | | |
| `outputTokens` | number | | |
| `cachedTokens` | number | | |
| `thoughtsTokens` | number | | |
| `totalTokens` | number | | |
| `apiDurationMs` | number | | |

No delete touches it, so it keeps the time and model of every response of a deleted session.

#### Managed auto-memory

`~/.qwen/projects/<sanitized project>/memory/` by default, under the memory base (a projection,
not parsed): the
documents a background agent extracts from conversations, on by default
(`memory.enableManagedAutoMemory` and `memory.enableManagedAutoDream`,
[`cli/src/config/settingsSchema.ts:2521-2540`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/config/settingsSchema.ts#L2521-L2540);
root at [`core/src/memory/paths.ts:133-165`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/memory/paths.ts#L133-L165)). No session id
or time; it outlives the sessions.

#### shell_history and checkpoints

`shell_history`, `checkpoint-<tag>.json` and `checkpoints/` hold what they hold in Gemini CLI
([Other stores in `gemini-cli.md`](gemini-cli.md#other-stores)): the shell-mode commands with no
time or session id ([`storage.ts:772-774`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/config/storage.ts#L772-L774)), the
`/chat save` histories, each a raw `Content[]` array written by `saveCheckpoint`
([`logger.ts:691-740`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/core/logger.ts#L691-L740)), and the tool checkpoints.
`checkpoints/` is deleted at start unless an execution sandbox is configured
([`cli/src/utils/cleanup.ts:126-136`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/utils/cleanup.ts#L126-L136);
[`cli/src/llm.tsx:589`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/llm.tsx#L589)).

#### Channel routes

`~/.qwen/channels/sessions.json`, written by `qwen channel start`, which drives sessions from a
messaging platform (Telegram, WeChat, QQ, DingTalk, WeCom, Feishu, GitHub, GitLab or email; the
conversation then also lives on that platform's servers)
([`cli/src/commands/channel/runtime.ts:43-45`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/commands/channel/runtime.ts#L43-L45);
[Channels in the vendor's documentation](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/docs/users/features/channels/overview.md#how-it-works)).
It is one JSON object keyed by routing key (`<channelName>:<chatId>` and similar forms by scope),
each value a route
([`channels/base/src/SessionRouter.ts:25-36`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/channels/base/src/SessionRouter.ts#L25-L36),
[`1607-1625`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/channels/base/src/SessionRouter.ts#L1607-L1625)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `sessionId` | string | | |
| `target` | object | | |
| `cwd` | string | | |
| `turns` | number | | |
| `startedAt` | number | | |
| `isolation` | string | `worktree` | |
| `workspaceCwd` | string | | |

`target` ([`channels/base/src/types.ts:180-187`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/channels/base/src/types.ts#L180-L187)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `channelName` | string | | |
| `messageRoute` | string | | |
| `senderId` | string | | |
| `chatId` | string | | |
| `threadId` | string | | |
| `isGroup` | boolean | | |

`startedAt` is epoch milliseconds. The file is rewritten on every routing change and deleted on a
clean shutdown (`clearAll`,
[`SessionRouter.ts:1508-1518`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/channels/base/src/SessionRouter.ts#L1508-L1518);
[`cli/src/commands/channel/start.ts:171`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/commands/channel/start.ts#L171)), so it survives
only a running service or a crash. The daemon-managed form writes the same shape to
`~/.qwen/channels/daemon/<workspace hash>/routes.json`
([`runtime.ts:47-62`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/commands/channel/runtime.ts#L47-L62);
[`cli/src/commands/channel/daemon-worker.ts:665`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/commands/channel/daemon-worker.ts#L665))
and only disposes the router on shutdown
([`daemon-worker.ts:880`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/commands/channel/daemon-worker.ts#L880),
[`895`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/commands/channel/daemon-worker.ts#L895)), so that file stays. The session
transcripts themselves are ordinary `chats/<sessionId>.jsonl` files.

#### Channel state files

Beside the routes, under `~/.qwen/channels/` (not parsed):

- `<channel>-group-history.jsonl`: the group-chat messages a channel kept as context, one
  `{type: "message", key, limit, entry: {senderId, senderName, text, messageId?, timestamp},
  recordedAt}` or `{type: "clear", key, recordedAt}` per line, times in epoch milliseconds
  ([`channels/base/src/ChannelBase.ts:1430-1437`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/channels/base/src/ChannelBase.ts#L1430-L1437);
  [`channels/base/src/group-history-store.ts:12-39`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/channels/base/src/group-history-store.ts#L12-L39),
  [`94-100`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/channels/base/src/group-history-store.ts#L94-L100)).
- `<channel>-pairing.json`, `<channel>-allowlist.json` and `<channel>-groups.json`, the pairing
  requests and approved senders and groups, under a per-workspace scope directory or at the top
  ([`channels/base/src/PairingStore.ts:52-71`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/channels/base/src/PairingStore.ts#L52-L71)).
- `<name>-<hash>-poll-cursor.json`, a polling adapter's position
  ([`channels/base/src/PollingChannelBase.ts:124-135`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/channels/base/src/PollingChannelBase.ts#L124-L135)).
- `cron.json`, the channel loop's scheduled jobs, and, per daemon workspace,
  `observed-contacts.json` and `instances/<label>-<hash>/`
  ([`runtime.ts:64-92`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/commands/channel/runtime.ts#L64-L92)).
- The email adapter's `state.json` (IMAP positions and reply routes, no password,
  [`channels/email/src/state.ts:29-37`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/channels/email/src/state.ts#L29-L37)) and the QQ
  adapter's `<name>-sessions.json`, `<name>-state.json`, `<name>-sessions-backup.json` and
  `<name>-sessions-purged.json` ([`channels/qqbot/src/QQChannel.ts:623-625`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/channels/qqbot/src/QQChannel.ts#L623-L625),
  [`720-733`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/channels/qqbot/src/QQChannel.ts#L720-L733)).
- The GitHub channel's publication audit and queues, per workspace scope
  ([Source host rows](#source-host-rows)).

#### Session registry

`~/.qwen/sessions/<pid>.json` (not parsed): an index of the sessions running now. Each top-level
session writes its record at start and unlinks it on exit; a process that hosts several sessions
(the `qwen --acp` child a daemon spawns) writes one `<pid>-<8 hex>.json` per session
([`core/src/services/session-registry.ts:7-22`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/session-registry.ts#L7-L22),
[`119`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/session-registry.ts#L119),
[`653`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/session-registry.ts#L653)). A record left by a process that died is
removed by the next listing that finds its process gone
([`735-790`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/session-registry.ts#L735-L790)). The record is written with mode
0600 ([`104`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/session-registry.ts#L104),
[`188-236`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/session-registry.ts#L188-L236)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `schemaVersion` | number | | |
| `pid` | number | | |
| `procStart` | string or null | | |
| `pidNs` | number or null | | |
| `sessionId` | string | | |
| `cwd` | string | | |
| `name` | string | | |
| `startedAt` | number | | |
| `qwenVersion` | string or null | | |
| `kind` | string | | |
| `ipcPath` | string | | |
| `ipcToken` | string | | |

`startedAt` is epoch milliseconds; `kind` is the registering surface, which this build writes as
`tui`, `headless`, `serve` or `external` and readers keep whatever they find, `tui` when absent
([`155-160`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/session-registry.ts#L155-L160)); `ipcPath` is the session's
peer-messaging socket and `ipcToken` the token a connection to it must present.

#### Session organization

`projects/<sanitized cwd>/session-organization.v1.json` (not parsed): the Web Shell's session
groups, colours and pins, `{schemaVersion: 1, groups: [{id, name, color, order, createdAt,
updatedAt}], sessions: {<sessionId>: {groupId, color?, pinnedAt?, updatedAt}}}`
([`core/src/services/session-organization-service.ts:27-46`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/session-organization-service.ts#L27-L46),
[`80-97`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/session-organization-service.ts#L80-L97),
[`260`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/session-organization-service.ts#L260)). A deleted session's entry is
removed ([What survives a delete](#what-survives-a-delete)).

#### Permission lists and trusted folders

`settings.json` (`~/.qwen/` and `<project>/.qwen/`) holds the `permissions.allow`,
`permissions.ask` and `permissions.deny` rule lists and `permissions.autoMode` (the AUTO
classifier's hints and limits)
([`cli/src/config/settingsSchema.ts:2755-2810`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/config/settingsSchema.ts#L2755-L2810)),
and `tools.approvalMode`, whose default is `auto`
([`3361-3381`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/config/settingsSchema.ts#L3361-L3381)). An "always allow" choice for the
project or for the user appends its rules to `permissions.allow` of the matching file
([`core/src/core/permission-helpers.ts:184-245`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/core/permission-helpers.ts#L184-L245);
[`cli/src/config/config.ts:2652-2664`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/config/config.ts#L2652-L2664)). The file is read
with comments stripped and a byte-order mark removed
([`cli/src/config/settings.ts:922`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/config/settings.ts#L922)). Qwen Code has no
`auto-saved.toml` policy file, where Gemini CLI keeps its saved grants
([Other stores in `gemini-cli.md`](gemini-cli.md#other-stores)).

`trustedFolders.json` is a JSON object whose keys are folder paths and whose values are a trust
level (`TrustLevel`), `TRUST_FOLDER`, `TRUST_PARENT` or `DO_NOT_TRUST`
([`cli/src/config/trustedFolders.ts:45-49`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/config/trustedFolders.ts#L45-L49);
[Trusted Folders in the vendor's documentation](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/docs/users/configuration/trusted-folders.md#managing-your-trust-settings)).
It is read as JSONC through `parseJsoncObject`, which allows comments and trailing commas and
strips a byte-order mark, and rewritten preserving the comments
([`trustedFolders.ts:176-192`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/config/trustedFolders.ts#L176-L192),
[`281-306`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/config/trustedFolders.ts#L281-L306);
[`cli/src/utils/jsonc-editor.ts:92-108`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/utils/jsonc-editor.ts#L92-L108)).

### Format versions

`version` on every record is the CLI version. The managed-engine header's `formatVersion` is `1`
at the reviewed commit
([`managed-session-records.ts:31`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/managed-runtime/managed-session-records.ts#L31)).
`logs.json` and `checkpoint-*.json` keep the shapes inherited from Gemini CLI
([logs.json](#logsjson), [shell_history and checkpoints](#shell_history-and-checkpoints)).

Changed between 0.25.3
([`5dcf7c7`](https://github.com/QwenLM/qwen-code/commit/5dcf7c7cb162e825d95d531081f67ee60da3c0e1))
and 0.26.4
([`835b080`](https://github.com/QwenLM/qwen-code/commit/835b080f8567d8612effa248e1fafa11af0e244b)):
the record subtypes `memory_catalog_snapshot` and `session_workflow_mode` and the record key
`clientInitiatedSkills` were added
([`chatRecordingService.ts:391-436`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/chatRecordingService.ts#L391-L436),
[`496`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/chatRecordingService.ts#L496)), and the managed local mode with its
`managed-local/` data directory appeared (`openManagedLocalState` is absent at 0.25.3). Paths and
every other record shape are unchanged.

Changed between 0.24.7
([`2c591ec`](https://github.com/QwenLM/qwen-code/commit/2c591ecc08a6fa080342f9b1b9f7f43215178cbb))
and 0.25.3; none of this has been observed on a real install:

- Record subtypes. The `agent_mention` and `agent_message` records, the `turn_attempt` record and
  the `code_mode_tool_result` subtype were added
  ([`chatRecordingService.ts` at 0.25.3, lines 391-435](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/core/src/services/chatRecordingService.ts#L391-L435)),
  as was `goalContext` on every record kind.
- `trustedFolders.json`: 0.24.7 read the file with comments stripped only
  ([`cli/src/config/trustedFolders.ts:162` at 0.24.7](https://github.com/QwenLM/qwen-code/blob/2c591ecc08a6fa080342f9b1b9f7f43215178cbb/packages/cli/src/config/trustedFolders.ts#L162));
  from 0.25.3 it reads it with `parseJsoncObject`, which also allows trailing commas and strips a
  byte-order mark
  ([`cli/src/utils/jsonc-editor.ts:92-107` at 0.25.3](https://github.com/QwenLM/qwen-code/blob/5dcf7c7cb162e825d95d531081f67ee60da3c0e1/packages/cli/src/utils/jsonc-editor.ts#L92-L107)).
- The `ToolCallEvent` class's own `event.name` is the bare `tool_call`
  ([`types.ts:189`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/telemetry/types.ts#L189)); every build reviewed records it as
  `qwen-code.tool_call`, and a build that recorded the event without the rename would have written
  the bare name.

### Secrets

The credential files are in [Credentials](#credentials). Inside the stores:
`session_model.systemPayload.baseUrl` is an endpoint, not a key;
`toolCallResult.responseParts` and `message.parts[].functionResponse.response` hold raw tool output
and `systemPayload.hookContext` holds hook output, either of which may carry a secret a tool
printed. `ipcToken` in a `sessions/<pid>.json` record ([Session registry](#session-registry)) is the
token that admits a connection to the running session's socket
([`session-registry.ts:224-235`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/session-registry.ts#L224-L235)). The managed
local mode's `database/` and `worker-home/` may hold the workers' own copies of credentials; which
was not determined.

### Parser plan

Globs (home-relative): `.qwen/projects/*/chats/*.jsonl`,
`.qwen/projects/*/chats/archive/*.jsonl`, `.qwen/tmp/*/logs.json`, `.qwen/trustedFolders.json`,
`.qwen/settings.json` (also a project's), `.qwen/usage_record.jsonl`,
`.qwen/channels/sessions.json` and `.qwen/channels/daemon/*/routes.json`. Excluded from transcript
parsing: `*.runtime.json`, `*.worktree.json`, `*.pr.json` and `*.ledger.jsonl`.

Rows: one per record; an assistant record also yields a `tool_use` row per `functionCall` part and
a `thinking` row per `thought: true` part.

| Column | Source |
| --- | --- |
| timestamp_utc | `timestamp` |
| session_id | `sessionId` |
| project_path | `cwd` |
| git_branch | `gitBranch` |
| turn_type | user, assistant and tool_result as recorded, except a user record the user did not type (`provenance` other than `real_user`, or subtype `notification`, `cron` or `agent_message`) → system; system → system; functionCall → tool_use; thought part → thinking |
| model | `model` of an assistant record; `modelId` of a `session_model` payload or of `executionContext` for the model rows |
| tool_name | `functionCall.name` / `functionResponse.name` |
| tool_use_id | `functionCall.id` / `toolCallResult.callId` |
| text | text parts, `systemPayload.displayText`, `rawCommand`, `toolCallResult.status` plus `errorType` |

A `notification`, `cron`, `agent_mention`, `agent_message` or `goal_runtime` record the user did
not type is the `system` row `<subtype> (provenance=<provenance> agent=<agentName>
agentId=<agentId>): <text>`, the agent from the record's `agentId` and `agentName` or, on an
agent's `agent_mention`, from `systemPayload.author` (`agentId`, `name`), parts not recorded left
out, `<text>` the message parts or `systemPayload.displayText`. The user's own `agent_mention`
(`provenance: "real_user"`) stays a `user` row. `turn_attempt`, `turn_result` and the other system
subtypes fall to the generic `system` row; a `code_mode_tool_result` record is read as an ordinary
`tool_result`.

#### Permission rows

Shapes in [Permission records](#permission-records) and
[Permission lists and trusted folders](#permission-lists-and-trusted-folders); source-derived, not
observed on any real install:

- `permission decision: <status>` (`system`, the result's `tool_name` and `tool_use_id`) after a
  `tool_result` row whose `toolCallResult.status` is `cancelled` or `awaiting_approval`; the
  `[<status>]` prefix on the result row stays.
- Per call: a `ui_telemetry` record keeps its `ui_telemetry: <JSON>` row; when its `uiEvent` has
  `event.name` `qwen-code.tool_call` or `tool_call` and a `decision`, it is followed by
  `permission decision: <decision> (tool_call ui_telemetry)` with `tool_name` the event's
  `function_name` and `tool_use_id` its `call_id`, which join the call's `tool_use` row. The row
  follows the result, as the record does.
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
  channel tokens beside them are never emitted. `~/.qwen/settings.json` is a secret glob, so a
  `--no-secrets` collection lacks it. A file that is not a JSON object is one
  `parser: .qwen/settings.json is not a JSON object` row.
- Trusted folders: `permission setting: trustedFolders.<path>=<value> (from <file>, file mtime)`
  (`system`, `project_path` the folder, no session id, the file's modification time, line 1), one
  per key of `~/.qwen/trustedFolders.json`, the value printed as recorded. The parser reads the file
  through the `trusted_folder_rows` reader it shares with the `gemini-cli` parser, which strips a
  byte-order mark, comments and trailing commas outside strings. A file that is still not a JSON
  object is one `parser: <file> is not a JSON object` row.

Not read: `permissions.autoMode` (the classifier's hints) and `mcpApprovals.json`.

#### Model rows

The model is recorded per message in an assistant record's `model`, per prompt in
`executionContext.modelId`, and per session (a daemon session's binding) in a `session_model`
record, written on every change.

- A `session_model` record is the row `model: <modelId> auth=<authType>`, or
  `model: <old> -> <modelId> auth=<authType>` when an earlier record of the session named another
  model (`model` column `modelId`).
- An assistant record's `model` yields `model: <id>` before its rows when no earlier
  `session_model` or assistant record of the session named a model, and `model: <old> -> <new>`
  when it differs from the model in force, tracked per `sessionId` with the record's timestamp and
  line.
- `executionContext.modelId` (with `authType`) on each user prompt and `mid_turn_user_message`
  record is the model in force when the prompt was sent and feeds the same tracking, before the
  record's own rows: `model: <modelId> auth=<authType>` or
  `model: <old> -> <modelId> auth=<authType>`, the `auth=` part left out when `authType` is absent.
  Source-derived; not observed on any real install.
- A `logs.json` `model_switch` entry is `model: <fromModel> -> <toModel> reason=<reason>` (`model`
  column `toModel`), printed from the entry, not tracked; the reviewed builds write no such entry
  ([logs.json](#logsjson)).

#### Past-use rows and secondary sources

`logs.json` is parsed as a secondary source
([Secondary sources](../analyzer/docs/output.md#secondary-sources)): each `user` entry repeats a
prompt of the transcript, joined on `sessionId` and text, and outlives a delete made through the
daemon ([What survives a delete](#what-survives-a-delete)).

`usage_record.jsonl` is parsed as a secondary source, source-derived; not observed on any real
install. Each line is one `system` row, `session usage: project <project> duration
<durationMs>ms models <id>,<id> tools <tool>=<count>,<tool>=<count> (from usage_record.jsonl,
startTime)`, `-` for an absent part, at `startTime`, with `session_id` the record's `sessionId` and
`project_path` its `project`. No transcript row repeats it, so it is always kept, and for a deleted
session it is the only row.

Not read: `shell_history`, `checkpoint-<tag>.json`, the managed auto-memory `memory/`, the token
usage log `token-usage-<YYYY-MM>.jsonl`, `todos/<sessionId>.json`, `PROJECT_SUMMARY.md`, the
channels' `<channel>-group-history.jsonl` and the managed local mode's `database/` ([Other
stores](#other-stores)).

#### Sync, relay and origin rows

The inherited Gemini CLI features are covered in
[Sync, relay and origin rows of `gemini-cli.md`](gemini-cli.md#sync-relay-and-origin-rows); Qwen
Code changed them as follows.

- There is no Clearcut logger and no Conseca. Usage statistics go instead to Alibaba Cloud RUM
  (Real User Monitoring) at `gb4w8c3ygj-default-sea.rum.aliyuncs.com`, on by default under
  `privacy.usageStatisticsEnabled`
  ([`core/src/telemetry/qwen-logger/qwen-logger.ts:83`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/telemetry/qwen-logger/qwen-logger.ts#L83),
  [`219`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/telemetry/qwen-logger/qwen-logger.ts#L219);
  [`cli/src/config/settingsSchema.ts:1521-1538`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/config/settingsSchema.ts#L1521-L1538)).
  The prompt event carries its id and length, not its text
  ([`qwen-logger.ts:552-558`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/telemetry/qwen-logger/qwen-logger.ts#L552-L558)), user
  feedback a rating ([`1113-1118`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/telemetry/qwen-logger/qwen-logger.ts#L1113-L1118)),
  and error text is replaced by `***REDACTED***` before an event is queued
  ([`150`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/telemetry/qwen-logger/qwen-logger.ts#L150),
  [`250-264`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/telemetry/qwen-logger/qwen-logger.ts#L250-L264)).
- OpenTelemetry export is off by default (`telemetry.enabled`), and once it is on the prompt text
  is in the exported record unless `telemetry.logPrompts` is set to false
  ([`core/src/config/config.ts:3531-3539`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/config/config.ts#L3531-L3539)).
- There is no Code Assist backend and no `/privacy` opt-in; `chat.qwen.ai` is used for OAuth only
  ([`core/src/qwen/qwenOAuth2.ts:28`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/qwen/qwenOAuth2.ts#L28)).
- `/export [md|html|json|jsonl] [path]` writes `qwen-code-export-<ISO time>.<ext>` to the working
  directory or a path inside it; it sends nothing
  ([`cli/src/ui/commands/exportCommand.ts:85-91`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/ui/commands/exportCommand.ts#L85-L91),
  [`492-512`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/ui/commands/exportCommand.ts#L492-L512);
  [`cli/src/ui/utils/export/utils.ts:10-13`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/ui/utils/export/utils.ts#L10-L13)). `/bug`
  opens a GitHub issue URL and writes no history file
  ([`cli/src/ui/commands/bugCommand.ts:38`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/ui/commands/bugCommand.ts#L38)).
- Channels. A session driven by `qwen channel start` is conducted on a messaging platform, so its
  prompts and replies sit on that platform's servers as well as in the local transcript. The
  marker is the channel routes file of [Channel routes](#channel-routes): an entry names the
  `sessionId` and the platform's `channelName`, `chatId`, `senderId` and `threadId`. The parser
  yields one `system` row per entry, with `session_id` the entry's `sessionId` and `project_path`
  its `cwd`: `session relayed: channel <channelName> chat <chatId or -> sender <senderId or ->
  thread <threadId or -> (from channels/sessions.json, startedAt)`, at `startedAt` when present and
  otherwise at the file's modification time with the text ending `file mtime)`, and
  `channels/daemon/<hash>/routes.json` named in place of `channels/sessions.json` for the daemon
  form. The row is `session relayed:`, not `session synced:`: the copy is on the messaging
  platform, not in a Qwen account, so the parser sets `records_relay` and not `records_sync`. A
  file or an entry that is not a JSON object is one `parser:` row. Fixture only: no real install
  has shown either file. A clean shutdown deletes `sessions.json`, so a session whose entry is gone
  may still have been a channel session; the bot tokens are in the `channels` key of
  `settings.json` or in environment variables it names
  ([Configuration in the vendor's channels page](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/docs/users/features/channels/overview.md#configuration)).
  The sessions that remain on the platform are deleted, if at all, on the platform, and no local
  file records that.
- The Mem0 external-context extension (`integrations/external-context-mem0`, Mem0 being a memory
  service an administrator configures)
  sends the submitted prompt as a search query when its Auto Recall hook is installed, and an
  opt-in `context_remember` tool writes content to that service; both are off unless installed and
  configured
  ([Mem0 in the vendor's documentation](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/docs/users/features/mem0.md)),
  and a write appears in the transcript only as that tool call.
- The Agent Host connection registers the daemon with a remote Agent Host server so that server
  can drive its sessions; the registration file is the only local marker
  ([`cli/src/serve/agent-host-client.ts:94-141`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/serve/agent-host-client.ts#L94-L141)).
  Its rows are not stated, as the parser does not read it.
- Qwen Code Desktop's Local Control shares the live daemon with a phone on the same network
  ([Qwen Code Desktop](#qwen-code-desktop)); whether it leaves a local record of a paired device was
  not determined.

#### Scratch directory rows

A record whose `cwd` lies under `<project>/.qwen/worktrees/`, or a `.worktree.json` sidecar, should
yield `scratch directory: <worktreePath> (enter_worktree <slug>)`. The paths are in
[Scratch workspaces and worktrees](#scratch-workspaces-and-worktrees).

#### Deletion rows

The agent records none: nothing marks a removed session except its surviving `usage_record.jsonl`
entry, its token usage records and, after a daemon delete, its `logs.json` prompts
([What survives a delete](#what-survives-a-delete)).

#### Bad-line rows

Each line of a transcript that does not decode becomes its own `parser: unparseable line` row.
`logs.json`, a JSON array, is decoded record by record: the records before a cut are yielded and
the cut is one `unparseable record <N>` row. A channel routes file or a `trustedFolders.json` that
is not a JSON object is one whole-file `parser:` row, and a routes entry that is not an object is
its own row ([Channel routes](#channel-routes), [Permission rows](#permission-rows)). Each line of
`usage_record.jsonl` that does not decode is its own `parser: unparseable line` row, and the lines
before and after it are still read. A `settings.json` that is not a JSON object is one whole-file
`parser:` row.

A transcript, `usage_record.jsonl`, `logs.json`, channel routes file, `settings.json` or
`trustedFolders.json` that cannot be read is one whole-file row
`parser: <file> could not be read (<reason>)` through `error_rows`, `<file>` its name, distinct
from the `parser: <file> is not a JSON object` of a file that reads but decodes to something else.

#### Fixtures

Transcript records as the writers of [ChatRecord](#chatrecord) produce them at 0.26.4, redacted:
a user prompt (`recordUserMessage`), an assistant turn with a thought, text and a function call
(`recordAssistantTurn`), its tool result (`recordToolResult`), the tool-call telemetry event
(`recordUiTelemetryEvent` through `logToolCall`), a daemon's approval-mode change
(`recordSessionApprovalMode`) and a cron prompt (`recordCronPrompt`):

```json
{"uuid":"7c1d2e3f-0a1b-4c2d-8e3f-4a5b6c7d8e01","parentUuid":null,"sessionId":"3f2e1d0c-9b8a-4766-8554-43322110ffee","timestamp":"2026-10-31T08:15:02.117Z","type":"user","provenance":"real_user","cwd":"/home/user/repo","version":"0.26.4","gitBranch":"feature/x","executionContext":{"modelId":"qwen3-coder-plus","authType":"qwen-oauth","approvalMode":"default"},"message":{"role":"user","parts":[{"text":"list the failing tests"}]},"systemPayload":{"displayText":"list the failing tests","hookContext":""},"promptId":"3f2e1d0c-9b8a-4766-8554-43322110ffee########1"}
{"uuid":"8d2e3f40-1b2c-4d3e-9f40-5b6c7d8e9f02","parentUuid":"7c1d2e3f-0a1b-4c2d-8e3f-4a5b6c7d8e01","sessionId":"3f2e1d0c-9b8a-4766-8554-43322110ffee","timestamp":"2026-10-31T08:15:05.480Z","type":"assistant","provenance":"assistant_output","cwd":"/home/user/repo","version":"0.26.4","gitBranch":"feature/x","model":"qwen3-coder-plus","message":{"role":"model","parts":[{"text":"I should run the test runner.","thought":true},{"text":"Running the suite."},{"functionCall":{"id":"call_9f8e7d","name":"run_shell_command","args":{"command":"npm test"}}}]},"usageMetadata":{"promptTokenCount":812,"candidatesTokenCount":41,"totalTokenCount":853},"contextWindowSize":262144}
{"uuid":"9e3f4051-2c3d-4e4f-a051-6c7d8e9fa003","parentUuid":"8d2e3f40-1b2c-4d3e-9f40-5b6c7d8e9f02","sessionId":"3f2e1d0c-9b8a-4766-8554-43322110ffee","timestamp":"2026-10-31T08:15:19.902Z","type":"tool_result","provenance":"tool_result","cwd":"/home/user/repo","version":"0.26.4","gitBranch":"feature/x","message":{"role":"user","parts":[{"functionResponse":{"id":"call_9f8e7d","name":"run_shell_command","response":{"output":"2 failing"}}}]},"toolCallResult":{"callId":"call_9f8e7d","resultDisplay":"2 failing","status":"success","contentLength":9}}
{"uuid":"af405162-3d4e-4f50-b162-7d8e9fab0004","parentUuid":"9e3f4051-2c3d-4e4f-a051-6c7d8e9fa003","sessionId":"3f2e1d0c-9b8a-4766-8554-43322110ffee","timestamp":"2026-10-31T08:15:19.950Z","type":"system","provenance":"system","subtype":"ui_telemetry","cwd":"/home/user/repo","version":"0.26.4","gitBranch":"feature/x","systemPayload":{"uiEvent":{"event.name":"qwen-code.tool_call","event.timestamp":"2026-10-31T08:15:19.949Z","call_id":"call_9f8e7d","function_name":"run_shell_command","function_args":{"command":"npm test"},"duration_ms":14422,"status":"success","success":true,"decision":"accept","prompt_id":"3f2e1d0c-9b8a-4766-8554-43322110ffee########1","tool_type":"native"}}}
{"uuid":"b0516273-4e5f-4061-8273-8e9fab1c0005","parentUuid":"af405162-3d4e-4f50-b162-7d8e9fab0004","sessionId":"3f2e1d0c-9b8a-4766-8554-43322110ffee","timestamp":"2026-10-31T08:16:40.003Z","type":"system","provenance":"system","subtype":"session_approval_mode","cwd":"/home/user/repo","version":"0.26.4","gitBranch":"feature/x","systemPayload":{"mode":"plan","prePlanMode":"default"}}
{"uuid":"c1627384-5f60-4172-9384-9fab1c2d0006","parentUuid":"b0516273-4e5f-4061-8273-8e9fab1c0005","sessionId":"3f2e1d0c-9b8a-4766-8554-43322110ffee","timestamp":"2026-10-31T09:00:00.012Z","type":"user","provenance":"system","subtype":"cron","cwd":"/home/user/repo","version":"0.26.4","gitBranch":"feature/x","message":{"role":"user","parts":[{"text":"check the nightly build"}]},"systemPayload":{"displayText":"check the nightly build"}}
```

A `usage_record.jsonl` line as `metricsToUsageRecord` writes it
([usage_record.jsonl](#usage_recordjsonl)) and a `channels/sessions.json` file as
`SessionRouter.persist` writes it ([Channel routes](#channel-routes)):

```json
{"version":1,"sessionId":"3f2e1d0c-9b8a-4766-8554-43322110ffee","timestamp":1793433600500,"startTime":1793430902117,"project":"/home/user/repo","durationMs":2698383,"models":{"qwen3-coder-plus":{"requests":6,"inputTokens":5120,"outputTokens":410,"cachedTokens":2048,"thoughtsTokens":96,"totalTokens":5530}},"tools":{"totalCalls":2,"totalSuccess":2,"totalFail":0,"byName":{"run_shell_command":{"count":2,"success":2,"fail":0}}},"files":{"linesAdded":0,"linesRemoved":0}}
{"telegram:5550001":{"sessionId":"4a3b2c1d-0e9f-4877-8665-54433221100f","target":{"channelName":"telegram","senderId":"5550001","chatId":"5550001","isGroup":false},"cwd":"/home/user/repo","turns":3,"startedAt":1793431000000}}
```

#### Source host rows

The agent records no connected source-host account in a file the parser reads. The GitHub and
GitLab channels act as an account through a personal access token in a `channels` entry of
`settings.json` or, for GitHub, through the host's own `gh` login ([Credentials](#credentials));
the authenticated login is looked up at connect and kept in memory (the GitHub channel also writes
it to the daemon's standard error), never in a file
([`channels/github/src/GithubAdapter.ts:593-597`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/channels/github/src/GithubAdapter.ts#L593-L597);
[`channels/gitlab/src/GitlabAdapter.ts:92-98`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/channels/gitlab/src/GitlabAdapter.ts#L92-L98)).

The GitHub channel appends a publication audit per workspace scope,
`~/.qwen/channels/<workspace scope>/<channel>-<hash>-github-audit.jsonl`, beside
`github-pending-deliveries.json` and `github-inbound-tasks.json` of the same prefix
([`GithubAdapter.ts:1027`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/channels/github/src/GithubAdapter.ts#L1027),
[`1948`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/channels/github/src/GithubAdapter.ts#L1948),
[`2064-2076`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/channels/github/src/GithubAdapter.ts#L2064-L2076)). One audit record per reply
([`GithubAdapter.ts:271-290`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/channels/github/src/GithubAdapter.ts#L271-L290)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `at` | string | | |
| `type` | string | `github_publication` | |
| `outcome` | string | `posted`, `suppressed`, `failed`, `posting` | |
| `channel` | string | | |
| `triggerKind` | string | | |
| `repository` | string | | |
| `number` | number | | |
| `sessionId` | string | | |
| `sourceMessageId` | string | | |
| `actor` | string | | |
| `threadId` | string | | |
| `pendingId` | string | | |
| `commentId` | number | | |
| `commentUrl` | string | | |
| `failurePhase` | string | `delivery` | |
| `failureError` | string | | |
| `bodySha256` | string | | |
| `bodyChars` | number | | |

A session that opened or reviewed a pull request keeps `chats/<sessionId>.pr.json`, `{prs: [...]}`
with at most ten entries ([`core/src/services/session-pr-service.ts:15-72`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/services/session-pr-service.ts#L15-L72)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `number` | number | | |
| `url` | string | | |
| `createdAt` | string | | |
| `state` | string | `open`, `merged`, `closed` | |
| `source` | string | `create`, `worktree`, `review` | |
| `issues` | array | | |

Neither file is parsed. No repository list is cached. Grepped for `github`, `gitlab`,
`bitbucket`, `oauth`, `installation`, `repositor`, `pull request` and `clone` at the reviewed
commit.

## Confidence

- High: every `storage.ts`, session, token and settings path; the record field names, status
  values, ISO timestamps and sidecar names.
- Medium: the platform paths of Qwen Code Desktop's configuration directory, which rest on
  [PathResolver::app_config_dir in the Tauri documentation](https://docs.rs/tauri/2.0.0/tauri/path/struct.PathResolver.html#method.app_config_dir)
  rather than a real install; the serialized value of `errorType`.
- Not determined: whether any build writes a `logs.json` `model_switch` entry; whether
  `extension-secrets-v1.json` holds anything besides extension secrets; what `hookContext` holds on
  a real install; what the managed local mode's H2 database and `worker-home/` hold beyond the
  sessions; whether a session registry record outlives a crashed process long enough to be
  collected; a worktree's size and whether anything else lives under `.qwen/worktrees`.
- For a real-install round: the per-call decision, approval-mode, model and usage rows (an
  approval, a denial, an allow always, a model switch and a session deleted from the TUI and from
  the Web Shell, with a listing diff after each), the channel route files and the Desktop app's
  directories.
