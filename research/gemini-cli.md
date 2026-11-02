# gemini-cli

Gemini CLI (catalog agent `gemini-cli`) is Google's open-source command-line agent, from
[google-gemini/gemini-cli](https://github.com/google-gemini/gemini-cli), with one surface, the
`gemini` CLI (its VS Code companion extension writes only a connection file outside the home,
[Per-user storage](#per-user-storage)). Qwen Code forked it ([`qwen-code.md`](qwen-code.md)
says how far the fork has drifted), Antigravity keeps its own state inside `~/.gemini`
([`antigravity.md`](antigravity.md)), and Zed's agent panel can drive it
([Driven from Zed](#driven-from-zed)). This document rests on commit
[`6814e00`](https://github.com/google-gemini/gemini-cli/commit/6814e00d6b148c3a1d57244d5d7040f7c541009d)
(version 0.67.0-nightly.20261020), the stable release
[v0.66.0](https://github.com/google-gemini/gemini-cli/releases/tag/v0.66.0) and a real install
of 0.66.0 on Linux ([Observed](#observed)). The agent is parsed: the `gemini-cli` parser reads
the session files, the past-use copies beside them (`logs.json`, `shell_history`, the
`/chat save` checkpoints, the `/bug` history exports, the tool checkpoints and the error
reports), the activity log's records at its default path but not the conversation turns in
its bodies, the
`memory/` files, the clipboard images and the standing permission files
([Parser plan](#parser-plan)).

## Source and evidence

[google-gemini/gemini-cli](https://github.com/google-gemini/gemini-cli) (Apache-2.0), default
branch `main` at commit
[`6814e00d6b148c3a1d57244d5d7040f7c541009d`](https://github.com/google-gemini/gemini-cli/commit/6814e00d6b148c3a1d57244d5d7040f7c541009d)
(2026-10-30). Its root `package.json` gives the version `0.67.0-nightly.20261020.gb041de017`;
the default branch carries nightly builds. The latest stable release, the one
`npm install -g @google/gemini-cli` installs, is tag `v0.66.0` at
[`9704132`](https://github.com/google-gemini/gemini-cli/commit/970413207732f5d4d8c50540c7fc359c44fdaaf2)
(2026-10-27, `package.json` version `0.66.0`); it writes every shape below except the shadow
repository's two exclusion files ([Format versions](#format-versions)). Every claim is source
at that commit (a `packages/...` link) unless it links a page of the repository's `docs/`
directory, the vendor's own pages, read at the same commit, such as
[Session management](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/docs/cli/session-management.md); a shape only an older commit writes is in Format
versions with its commit. A claim about what an install wrote rests on the Linux CLI round of
0.66.0 and names that build; a store no install has shown is source-derived.

### Observed

- **Linux CLI**: Gemini CLI 0.66.0 at `/usr/local/bin/gemini`, installed with the vendor's npm
  command
  ([Install globally with npm](https://github.com/google-gemini/gemini-cli/blob/main/README.md#install-globally-with-npm),
  fetched 2026-11-01) on an Ubuntu lab virtual machine (arm64, GNOME desktop, Node v22.22.1),
  run twice on the same build, 2026-11-01 22:51 UTC to 2026-11-02 01:50 UTC and 2026-11-02
  02:00 to 03:40 UTC, each driven over SSH in a 120 by 40 terminal with a listing of the agent's
  directories (names, sizes, modes and times) after every step and a collection before the
  delete steps, after them and after a reboot. In a sample git project the first run's steps
  were: the first start and the folder trust dialog
  ([Per-user storage](#per-user-storage)); the Google sign-in, which the service refused for a
  personal account, then an API key pasted into the dialog ([Credentials](#credentials)); the `?`
  and `/help` screens; a question answered with no tool call; a web question, shell calls
  approved once, cancelled and allowed for the session, an edit dialog, an MCP server's tool and
  the underspecified prompt "refactor this" ([Permission rows](#permission-rows)); `/model`
  ([Model rows](#model-rows)); `/bug` ([Bug reports](#bug-reports)); `/exit` and
  `gemini --resume latest`; `/clear` ([Transcript stores](#transcript-stores));
  `gemini --worktree lab` with `experimental.worktrees` unset
  ([Project-local files](#project-local-files)); `general.devtools` on
  ([Activity log](#activity-log)); `gemini --list-sessions`, `gemini --delete-session` and the
  project directory removed ([What survives a delete](#what-survives-a-delete)); a start in the
  home ([Per-user storage](#per-user-storage)); `/auth signout` and a sign-in from a terminal on
  the desktop ([Credentials](#credentials)); a 600-second idle period
  ([What survives a delete](#what-survives-a-delete)); and a reboot. The session file's line
  types and keys are in [Session JSONL](#session-jsonl). The first run recorded no reply to its
  pastes, no `/chat save` checkpoint and no model switch. The second run repeated the first-run
  steps with the same results and added: the API-key dialog's text and a second sign-out
  ([Credentials](#credentials)); the refactor prompt cancelled at its first dialog and the edit
  dialog read whole ([Permission rows](#permission-rows)); a short and a long paste
  ([Transcript stores](#transcript-stores)); the `/model` Manual list
  ([Model rows](#model-rows)); `/chat save lab`
  ([/chat save checkpoints](#chat-save-checkpoints)); and `gemini --worktree lab` with
  `experimental.worktrees` set ([Project-local files](#project-local-files)). The second run's
  MCP call is not observed; the first run's is. Not observed in either run: a model switch and
  the names of the permission modes. Neither run typed a shell-mode command.

### Source reviews

- 2026-11-02: the Linux CLI round on 0.66.0 ([Observed](#observed)), with `devtoolsService.ts`,
  `activityLogger.ts` and `nonInteractiveCli.ts` re-read at
  [`6814e00`](https://github.com/google-gemini/gemini-cli/commit/6814e00d6b148c3a1d57244d5d7040f7c541009d)
  and `v0.66.0`, which corrected where the activity log is written.
- 2026-11-01: `main` at
  [`6814e00`](https://github.com/google-gemini/gemini-cli/commit/6814e00d6b148c3a1d57244d5d7040f7c541009d):
  added the activity log's payload keys and the error report's keys, and the session id inside a
  tool checkpoint's `messageId`; corrected the tool checkpoint's `history`, which is the
  interface's display list, not `Content` items.
- 2026-11-01: `main` at
  [`6814e00`](https://github.com/google-gemini/gemini-cli/commit/6814e00d6b148c3a1d57244d5d7040f7c541009d)
  (0.67.0-nightly.20261020) and tag `v0.66.0`: every citation re-read at the new commit; added
  the sandbox approvals file `policies/sandbox.toml`, the extension integrity files, the
  container sandbox's throwaway settings directory, the clipboard images, the error reports in
  the system temporary directory and the shadow repository's exclusion files; settled that the
  ACP session id is Gemini CLI's own session id and that the always-allow priority is an integer;
  dropped the names read from the workstation.
- 2026-10-21:
  [`3be3d44`](https://github.com/google-gemini/gemini-cli/commit/3be3d4424b3c8a760d793ae1bea6cb5180adf27d)
  (0.66.0-nightly.20261013): added the user policy files, the policy rule schema, the
  `.gemini/worktrees` exclusion and the source-host statement.
- 2026-10-19:
  [`3be3d44`](https://github.com/google-gemini/gemini-cli/commit/3be3d4424b3c8a760d793ae1bea6cb5180adf27d)
  against
  [`fb972b2`](https://github.com/google-gemini/gemini-cli/commit/fb972b2f87fe7d5b06d37eac711490162d98de2c)
  (0.64.0-nightly.20260929): added session retention, the standing permission files, the
  past-use stores, the `--worktree` checkout and the model alias.

## Collection

### Per-user storage

The home is `$GEMINI_CLI_HOME` when set, else `os.homedir()`
([`core/src/utils/paths.ts:22-27`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/utils/paths.ts#L22-L27)), and the state
directory is `GEMINI_DIR = '.gemini'` below it
([`paths.ts:13`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/utils/paths.ts#L13);
[`core/src/config/storage.ts:54-60`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/storage.ts#L54-L60)), with
the system temporary directory standing in when no home resolves. User paths have no `win32` or
`darwin` branch and no XDG use, so the layout is `~/.gemini` on Linux and macOS and
`%USERPROFILE%\.gemini` on Windows; the only platform split is the system settings directory
(`/etc/gemini-cli`, `/Library/Application Support/GeminiCli`, `C:\ProgramData\gemini-cli`;
[`storage.ts:174-182`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/storage.ts#L174-L182)), which lies outside
every home.

The runtime directory is `~/.gemini`, except under `SANDBOX=sandbox-exec` (the macOS Seatbelt
sandbox), where it is `~/.cache/.gemini`
([`storage.ts:92-108`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/storage.ts#L92-L108); created in
[`cli/src/utils/sandbox.ts:224`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/utils/sandbox.ts#L224)). Its files are
therefore under either directory on macOS:

| Path below the runtime directory | Holds | Source |
| --- | --- | --- |
| `mcp-oauth-tokens.json`, `a2a-oauth-tokens.json` | MCP and A2A server OAuth tokens ([Credentials](#credentials)) | [`storage.ts:70-76`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/storage.ts#L70-L76) |
| `installation_id` | the install's random id | [`storage.ts:123`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/storage.ts#L123) |
| `google_accounts.json` | the signed-in Google account | [`storage.ts:127`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/storage.ts#L127) |
| `trustedFolders.json` | folder trust decisions; `GEMINI_CLI_TRUSTED_FOLDERS_PATH` moves it | [`storage.ts:131-136`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/storage.ts#L131-L136) |
| `policy_integrity.json` | one hash per policy directory | [`storage.ts:170`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/storage.ts#L170) |
| `projects.json` | the project registry | [`storage.ts:289-292`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/storage.ts#L289-L292) |
| `history/<slug>/` | the checkpoint shadow repository | [`storage.ts:320`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/storage.ts#L320) |
| `tmp/` | the per-project directories below, and the global temporary files | [`storage.ts:195`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/storage.ts#L195) |

Always under `~/.gemini`:

| Path | Holds | Source |
| --- | --- | --- |
| `settings.json` | user settings, MCP servers included | [`storage.ts:78`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/storage.ts#L78) |
| `.env` | environment values, API keys among them | [`cli/src/config/settings.ts:631`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/config/settings.ts#L631) |
| `oauth_creds.json` | the Google sign-in token ([Credentials](#credentials)) | [`storage.ts:255`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/storage.ts#L255) |
| `gemini-credentials.json` | the encrypted keychain fallback | [`core/src/services/fileKeychain.ts:19-20`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/services/fileKeychain.ts#L19-L20) |
| `integrity.key`, `extension_integrity.json` | the extension integrity key's file fallback, and the signed record of installed extensions | [`core/src/config/constants.ts:40-41`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/constants.ts#L40-L41); [`core/src/config/extensions/integrity.ts:49`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/extensions/integrity.ts#L49), [`118`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/extensions/integrity.ts#L118) |
| `commands/`, `skills/`, `agents/` | user slash commands, skills and subagent definitions | [`storage.ts:138`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/storage.ts#L138), [`142`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/storage.ts#L142), [`158`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/storage.ts#L158) |
| `policies/` | `auto-saved.toml` and its `.bak`, other policy files, `sandbox.toml` ([Standing permission files](#standing-permission-files)) | [`storage.ts:28`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/storage.ts#L28), [`150`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/storage.ts#L150), [`247`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/storage.ts#L247); [`core/src/policy/sandboxPolicyManager.ts:104`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/policy/sandboxPolicyManager.ts#L104) |
| `keybindings.json` | key bindings | [`storage.ts:154`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/storage.ts#L154) |
| `acknowledgments/agents.json` | acknowledged project subagents | [`storage.ts:162-168`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/storage.ts#L162-L168) |
| `trusted_hooks.json` | trusted project hooks | [`core/src/hooks/trustedHooks.ts:27-30`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/hooks/trustedHooks.ts#L27-L30) |
| `state.json` | UI state | [`cli/src/utils/persistentState.ts:11`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/utils/persistentState.ts#L11), [`35`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/utils/persistentState.ts#L35) |
| `mcp-server-enablement.json` | which MCP servers are enabled | [`cli/src/config/mcp/mcpServerEnablement.ts:181`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/config/mcp/mcpServerEnablement.ts#L181) |
| `GEMINI.md` | the user's memory file (any configured context name) | [`core/src/tools/memoryTool.ts:90-91`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/tools/memoryTool.ts#L90-L91) |
| `extensions/<name>/` | installed extensions | [`storage.ts:472-478`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/storage.ts#L472-L478); [`cli/src/config/extensions/variables.ts:21`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/config/extensions/variables.ts#L21) |
| `whisper_models/` | downloaded speech-to-text models (`ggml-*.bin`) | [`core/src/voice/whisperModelManager.ts:25-39`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/voice/whisperModelManager.ts#L25-L39) |
| `bin/litert/` | the local Gemma model server binary | [`cli/src/commands/gemma/constants.ts:36`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/commands/gemma/constants.ts#L36) |
| `cli-browser-profile/` | the browser agent's Chromium profile and its consent marker | [`core/src/utils/browserConsent.ts:16`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/utils/browserConsent.ts#L16); [`core/src/agents/browser/browserManager.ts:659-664`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/agents/browser/browserManager.ts#L659-L664) |
| `sandbox-macos-<profile>.sb` | a custom Seatbelt profile the user wrote | [`cli/src/utils/sandbox.ts:150-162`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/utils/sandbox.ts#L150-L162) |

A sibling `~/.agents/skills` is read for skills too
([`storage.ts:62-68`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/storage.ts#L62-L68),
[`146`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/storage.ts#L146)).

The global `tmp/` holds `background-processes/background-<pid>.log`, the output of shell
commands run in the background
([`core/src/services/shellExecutionService.ts:390-412`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/services/shellExecutionService.ts#L390-L412)),
`seen_screen_reader_nudge.json`
([`cli/src/ui/components/Notifications.tsx:30-33`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/ui/components/Notifications.tsx#L30-L33))
and the Gemma server's `litert-server.pid` and `litert-server.log`
([`gemma/constants.ts:39-45`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/commands/gemma/constants.ts#L39-L45)).
`storage.ts` also names `tmp/bin`
([`199-201`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/storage.ts#L199-L201)), but nothing at the reviewed
commit calls that getter; ripgrep ships inside the package
([`core/src/tools/ripGrep.ts:64-69`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/tools/ripGrep.ts#L64-L69)).
Older builds downloaded the ripgrep binary into `tmp/bin` ([Format versions](#format-versions)).

The per-project directory is `tmp/<slug>/`
([`storage.ts:230-234`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/storage.ts#L230-L234)). The slug comes from
`projects.json`: the project directory's basename lowercased, other characters to `-`, a `-N`
suffix on collision
([`core/src/config/projectRegistry.ts:304-316`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/projectRegistry.ts#L304-L316),
[`414-420`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/projectRegistry.ts#L414-L420)). The older directory
named by the SHA-256 of the project path
([`paths.ts:318-319`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/utils/paths.ts#L318-L319)) is copied to the slug
directory on first start and left in place
([`storage.ts:310-324`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/storage.ts#L310-L324);
[`core/src/config/storageMigration.ts:20-52`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/storageMigration.ts#L20-L52)),
so both forms can exist. Its contents:

| Path below `tmp/<slug>/` | Holds | Source |
| --- | --- | --- |
| `chats/session-<YYYY-MM-DDTHH-MM>-<id8>.jsonl`, `chats/<parent session id>/<session id>.jsonl` | session transcripts and subagent transcripts ([Transcript stores](#transcript-stores)) | [`core/src/services/chatRecordingService.ts:781-823`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/services/chatRecordingService.ts#L781-L823) |
| `logs.json` | the prompt history | [`core/src/core/logger.ts:15`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/core/logger.ts#L15), [`148-149`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/core/logger.ts#L148-L149) |
| `checkpoint-<tag>.json` | `/chat save` checkpoints | [`logger.ts:294`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/core/logger.ts#L294) |
| `checkpoints/` | tool checkpoints | [`storage.ts:364`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/storage.ts#L364) |
| `logs/session-<session id>.jsonl` | the activity log's default path, which no caller passes ([Activity log](#activity-log)) | [`cli/src/utils/activityLogger.ts:772-778`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/utils/activityLogger.ts#L772-L778) |
| `tool-outputs/`, `tool-outputs/session-<session id>/` | large tool outputs kept out of the transcript | [`core/src/utils/fileUtils.ts:839-857`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/utils/fileUtils.ts#L839-L857) |
| `plans/`, `tracker/`, `tasks/`, each also under `<session id>/` | plans, task trackers and tasks | [`storage.ts:372-411`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/storage.ts#L372-L411) |
| `memory/` | the memory tool's files ([memory/](#memory)) | [`storage.ts:336`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/storage.ts#L336) |
| `shell_history` | shell-mode commands | [`storage.ts:480`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/storage.ts#L480) |
| `images/clipboard-<epoch ms>.png` | images pasted from the clipboard, removed after an hour | [`cli/src/ui/utils/clipboardUtils.ts:264`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/ui/utils/clipboardUtils.ts#L264), [`283`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/ui/utils/clipboardUtils.ts#L283), [`403`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/ui/utils/clipboardUtils.ts#L403) |
| `compression_state.json` | file summaries the context compressor caches | [`core/src/context/contextCompressionService.ts:58`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/context/contextCompressionService.ts#L58) |
| `bug-report-history-<epoch ms>.json`, `bug-memory-<epoch ms>.heapsnapshot` | `/bug` and `/bug-memory` output ([Bug reports](#bug-reports)) | [`cli/src/ui/commands/bugCommand.ts:85-91`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/ui/commands/bugCommand.ts#L85-L91) |
| `otel/collector*.log` | the local telemetry collector's logs | [`docs/local-development.md:63`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/docs/local-development.md#L63), [`113`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/docs/local-development.md#L113) |
| `.project_root` | the absolute project path ([Where the project path is recorded](#where-the-project-path-is-recorded)) | [`projectRegistry.ts:24`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/projectRegistry.ts#L24), [`372-412`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/projectRegistry.ts#L372-L412) |

`history/<slug>/` is the checkpoint shadow repository, a git repository whose work tree is the
project: `.git`, `.gitconfig` (a fixed author and, when the user's global git configuration sets
one, the `core.excludesFile` path), `.gitconfig_system_empty`, `.gitignore` (a copy of the
project's) and `.git/info/exclude` (a copy of the project's)
([`core/src/services/gitService.ts:105-106`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/services/gitService.ts#L105-L106),
[`138-174`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/services/gitService.ts#L138-L174),
[`328-360`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/services/gitService.ts#L328-L360);
[Checkpointing: How it works](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/docs/cli/checkpointing.md#how-it-works)), plus
`.project_root` ([`storage.ts:293-296`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/storage.ts#L293-L296)).

`extensions/<name>/` holds `gemini-extension.json`, `.gemini-extension-install.json` and `.env`
([`variables.ts:22-24`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/config/extensions/variables.ts#L22-L24)); a git
source is a shallow clone
([`cli/src/config/extensions/github.ts:57`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/config/extensions/github.ts#L57)).

Antigravity, Google's agentic editor and its CLI, keeps `antigravity/`, `antigravity-cli/`,
`antigravity-ide/`, `antigravity-backup/` and `config/` under `~/.gemini` (see
[`antigravity.md`](antigravity.md#data-directories-under-gemini)); Gemini CLI writes none of them, `~/.gemini/config` included (no path at the reviewed commit
joins `config` to the state directory).

Outside the home, the container sandbox (`tools.sandbox` set to `docker` or `podman`) gives the
container a fresh `gemini-sandbox-<random>` directory in the host's system temporary directory as
its `~/.gemini`, holding only a copy of the user settings with secrets removed, and deletes it
when the CLI exits
([`sandbox.ts:532-541`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/utils/sandbox.ts#L532-L541),
[`571-587`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/utils/sandbox.ts#L571-L587),
[`78-97`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/utils/sandbox.ts#L78-L97),
[`132`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/utils/sandbox.ts#L132);
[`storage.ts:104-106`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/storage.ts#L104-L106)): a session run in the
container sandbox writes its transcript, `logs.json` and `shell_history` there, not under the
host's `~/.gemini`. A failed model request writes
`gemini-client-error-<type>-<timestamp>.json` to the system temporary directory, holding the
error and the conversation history sent with the request
([`core/src/utils/errorReporting.ts:26-35`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/utils/errorReporting.ts#L26-L35);
[`core/src/core/turn.ts:440-447`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/core/turn.ts#L440-L447)); on Windows that
directory is `%TEMP%`, by default inside the profile. The VS Code companion extension writes
`gemini/ide/gemini-ide-server-<parent pid>-<port>.json` there, holding the port, the workspace
path and a per-run auth token
([`vscode-ide-companion/src/ide-server.ts:82-91`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/vscode-ide-companion/src/ide-server.ts#L82-L91),
[`362-370`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/vscode-ide-companion/src/ide-server.ts#L362-L370)).

On the Linux install of 0.66.0 the first start in a project created `~/.gemini` (mode 0775) with
`installation_id` (36 bytes), `projects.json` ([projects.json](#projectsjson)), `state.json` and
`state.json.bak` ([state.json](#statejson)), the `.project_root` files of `tmp/<slug>/` and
`history/<slug>/` (19 bytes each) and a session file of about 1 KB. The first screen is the
folder trust dialog ([Permission rows](#permission-rows)); trusting the folder wrote
`trustedFolders.json` (43 bytes, mode 0600) and restarted the CLI
([Transcript stores](#transcript-stores)). Choosing an API key wrote `settings.json`
([Credentials](#credentials)). A start in the home itself was met by the trust dialog again and
put its sessions and `logs.json` under a `tmp/<slug>/` named for the home directory, as for any
project. The sample project's `history/<slug>/` held only `.project_root`, and no
`shell_history`, `checkpoints/`, `memory/`, `images/`, `logs/` or `tool-outputs/` appeared in its
`tmp/<slug>/`.

#### Scratch directories

Gemini CLI has no working directory of its own: it runs in the directory it was started in. The
one directory it creates for work is the `--worktree` checkout inside the project, made only with
`experimental.worktrees` on ([Project-local files](#project-local-files)).

### Credentials

- `.gemini/oauth_creds.json` (`%USERPROFILE%\.gemini\oauth_creds.json` on Windows) holds the
  Google sign-in token. The reviewed commit's loader moves a file it finds into the keychain and
  deletes it
  ([`core/src/code_assist/oauth-credential-storage.ts:16`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/code_assist/oauth-credential-storage.ts#L16),
  [`117-153`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/code_assist/oauth-credential-storage.ts#L117-L153)), so a
  file that no later start has loaded remains. On the Linux install of 0.66.0 the Google sign-in
  wrote it (1800 bytes, mode 0600), and it was gone by the next listing, ten minutes later,
  after the refusal below and the choice of the API-key method.
- The Google sign-in of 0.66.0 over SSH, with no browser, prints "Please visit the following URL
  to authorize the application:", an `accounts.google.com` authorization URL whose redirect is
  `https://codeassist.google.com/authcode`, and "Enter the authorization code:"; a code not
  entered within five minutes ends it ("Authorization timed out after 5 minutes"). For a personal
  Google account the completed sign-in also wrote `google_accounts.json`, and the CLI then
  refused it, naming Gemini Code Assist, Google's coding-assistant service behind the Google
  sign-in: "Failed to sign in. Message: This client is no longer supported for
  Gemini Code Assist for individuals. To continue using Gemini, please migrate to the Antigravity
  suite of products". Google's
  [Gemini Code Assist consumer accounts](https://developers.google.com/gemini-code-assist/docs/deprecations/code-assist-individuals)
  page (last updated 2026-10-09, fetched 2026-11-01) says that from 2026-06-18 the Gemini Code
  Assist for individuals, Google AI Pro and Google AI Ultra tiers are no longer served, Gemini CLI
  included, that those users can move to the Antigravity products, Antigravity CLI among them
  (`agy`, [`antigravity.md`](antigravity.md)), and that Standard and Enterprise subscriptions are
  on a separate deprecation schedule.
- The keychain is `@github/keytar`
  ([`core/src/services/keychainService.ts:176`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/services/keychainService.ts#L176);
  [`core/package.json:97`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/package.json#L97)), with the services
  `gemini-cli-oauth` (Google sign-in and MCP tokens), `gemini-cli-api-key` and
  `gemini-cli-extension-integrity` (account `secret-key`)
  ([`oauth-credential-storage.ts:16`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/code_assist/oauth-credential-storage.ts#L16);
  [`core/src/core/apiKeyCredentialStorage.ts:12`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/core/apiKeyCredentialStorage.ts#L12);
  [`constants.ts:42-43`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/constants.ts#L42-L43)). Under
  `GEMINI_FORCE_FILE_STORAGE=true`, under WSL, or when the native module fails, the fallback is
  `.gemini/gemini-credentials.json` (`FileKeychain`), AES-256-GCM with a key derived with the salt
  `<hostname>-<username>-gemini-cli`, so it decrypts offline given both
  ([`keychainService.ts:22`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/services/keychainService.ts#L22),
  [`112-136`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/services/keychainService.ts#L112-L136);
  [`fileKeychain.ts:19-26`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/services/fileKeychain.ts#L19-L26)).
- On the Linux install of 0.66.0, in an SSH session beside a GNOME desktop session, an API key
  pasted into the authentication dialog went to the GNOME keyring, whose login keyring file
  (`.local/share/keyrings/login.keyring`, a file of the desktop and of no catalog line) was
  rewritten 15 seconds later; neither `.gemini/.env` nor `.gemini/gemini-credentials.json` was
  written. `settings.json` then held only
  `security.auth.selectedType`, and the banner read "Authenticated with gemini-api-key /auth".
  A key entered from a terminal on the desktop rewrote the same keyring file (641 bytes).
- The API-key dialog of 0.66.0 reads "Enter Gemini API Key", "Please enter your Gemini API key.
  It will be securely stored in your system keychain.",
  `You can get an API key from https://aistudio.google.com/app/apikey`,
  "Paste your API key here" and "(Press Enter to submit, Esc to cancel, Ctrl+C to clear stored
  key)". It drops the spaces of typed text: a prompt typed into it by mistake was stored as the
  key, as one word, and every later request failed with an invalid-key error ("API key not
  valid"). Once, `gemini --resume latest` over SSH after `/exit` showed this dialog although the
  key had been entered earlier in the same run, while the keyring file measured 105 bytes
  against 641 before and after; the cause was not determined.
- `/auth` has the subcommands the completion list for `/auth ` shows and `/help` does not
  list: `signin` ("Sign in or change the authentication method") and `signout` ("Sign out and
  clear all cached credentials"). On 0.66.0 `/auth signout` printed "You are now signed out" and
  "Sign in again to continue using Gemini CLI, or exit the application." with `1. Sign in` and
  `2. Exit`, and rewrote `google_accounts.json` (66 and, in the second run, 33 bytes) and
  `settings.json` but left the keyring file unchanged. The stored key outlived it: `/auth` then
  `2. Use Gemini API Key` opened the dialog prefilled with the stored value, Ctrl+C in it left
  the value in place, Escape returned to the method dialog rather than the prompt, and the next
  start, from a terminal on the desktop, came prefilled with it again and reported
  "Authenticated with gemini-api-key". A responder therefore finds a signed-out user's API key
  in the keyring still.
- MCP and A2A tokens go through `HybridTokenStorage` (keychain, else that encrypted file;
  [`core/src/mcp/token-storage/hybrid-token-storage.ts:28-41`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/mcp/token-storage/hybrid-token-storage.ts#L28-L41)),
  and the plain writer of `.gemini/mcp-oauth-tokens.json` (`.cache/.gemini/mcp-oauth-tokens.json`
  under the macOS sandbox) remains
  ([`core/src/mcp/oauth-token-storage.ts:48`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/mcp/oauth-token-storage.ts#L48),
  [`114-117`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/mcp/oauth-token-storage.ts#L114-L117));
  `.gemini/a2a-oauth-tokens.json` (`.cache/.gemini/a2a-oauth-tokens.json`) is its A2A counterpart.
- `.gemini/integrity.key` is the extension integrity key written when the keychain is
  unavailable, mode 0600; whoever holds it can sign a forged `extension_integrity.json`
  ([`integrity.ts:56-104`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/extensions/integrity.ts#L56-L104)).
- `.gemini/.env`, and the `.env` files the CLI searches before it (`<cwd>/.gemini/.env`,
  `<cwd>/.env`, walking up from the working directory) and after it (`~/.env`, a file Gemini
  CLI reads but does not own), carry `GEMINI_API_KEY` or `GOOGLE_API_KEY`
  ([`settings.ts:607-636`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/config/settings.ts#L607-L636);
  [`cli/src/config/auth.ts:22`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/config/auth.ts#L22),
  [`36`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/config/auth.ts#L36)).
- `.gemini/settings.json` and a project's `.gemini/settings.json` hold MCP server `env` and
  `headers` values (`mcpServers.*.env`, `mcpServers.*.headers`)
  ([`cli/src/config/settingsSchema.ts:3053`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/config/settingsSchema.ts#L3053),
  [`3071`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/config/settingsSchema.ts#L3071)). The CLI rewrites either through
  a write copy `.gemini/.settings.json.<pid>.<epoch ms>.<n>.tmp` beside it, renamed over the file;
  when the rename fails the CLI deletes the copy and writes the file directly, so the copy remains
  only when the process stops between writing it and the rename, or when that delete fails
  ([`cli/src/utils/commentJson.ts:23-46`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/utils/commentJson.ts#L23-L46)).
- `.gemini/tmp/<slug>/bug-memory-<epoch ms>.heapsnapshot`, the V8 heap snapshot `/bug-memory`
  writes, is a dump of the CLI process's memory and so holds the access token and API keys in
  use ([Bug reports](#bug-reports)).
- The patterns `*.gemini/settings.json` and `*.gemini/tmp/*/bug-memory-*.heapsnapshot` match the
  settings files and heap snapshots in every home and project.
- `.gemini/google_accounts.json` is not a credential: it names the signed-in Google account's
  email and holds no token
  ([`core/src/utils/userAccountManager.ts:19`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/utils/userAccountManager.ts#L19)).
  After the refused sign-in on 0.66.0 it held `active` as `null` and `old` as an array of one
  string.
- `.gemini/extensions/<name>/.env` holds an extension's settings not marked `sensitive`, which go
  to the keychain instead
  ([Extension reference](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/docs/extensions/reference.md#L204)).
  A setting that leaves `sensitive` unset counts as not sensitive, so an extension that names an
  API key without the flag keeps it in that file
  ([`cli/src/config/extensions/extensionSettings.ts:28-29`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/config/extensions/extensionSettings.ts#L28-L29),
  [`114-126`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/config/extensions/extensionSettings.ts#L114-L126)).

### Exclusions

These subtrees are large or copies of something else; the patterns are home-relative, and
`.gemini/worktrees` also project-relative:

- `.gemini/whisper_models` (speech models), `.gemini/bin` (the Gemma server binary),
  `.gemini/tmp/bin` (the ripgrep binary older builds downloaded), `.gemini/cli-browser-profile`
  (a Chromium profile).
- `.gemini/extensions/*/node_modules` and `.gemini/extensions/*/.git` (an extension's
  dependencies and clone).
- `.gemini/worktrees`, the `--worktree` checkouts inside a project
  ([Project-local files](#project-local-files)).

`history/*/.git` is a shadow copy of the project at each checkpoint: large but evidence of what
the agent changed, so it is kept and left to the size cap.

### Project-local files

| Path in the project | Holds | Source |
| --- | --- | --- |
| `GEMINI.md` (any configured context name) | project memory | [`memoryTool.ts:90-91`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/tools/memoryTool.ts#L90-L91) |
| `.geminiignore` | ignore patterns | [`constants.ts:37`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/constants.ts#L37) |
| `.gemini/settings.json`, `.gemini/.env` | workspace settings and environment ([Credentials](#credentials)) | [`storage.ts:344`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/storage.ts#L344); [`settings.ts:616`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/config/settings.ts#L616) |
| `.gemini/commands`, `.gemini/skills`, `.gemini/agents`, `.gemini/policies` | workspace commands, skills, subagents and policies | [`storage.ts:236-237`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/storage.ts#L236-L237), [`348-362`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/storage.ts#L348-L362) |
| `.gemini/policies/auto-saved.toml` | the workspace's always-allow grants ([Standing permission files](#standing-permission-files)) | [`storage.ts:236-245`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/storage.ts#L236-L245) |
| `.gemini/extensions/gemini-extension.json` | workspace extensions | [`storage.ts:472-478`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/storage.ts#L472-L478) |
| `.gemini/system.md` | a replacement system prompt (`GEMINI_SYSTEM_MD`) | [`core/src/prompts/promptProvider.ts:112`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/prompts/promptProvider.ts#L112) |
| `.gemini/sandbox-macos-<profile>.sb`, `.gemini/sandbox.venv` | a custom Seatbelt profile; the container sandbox's stand-in for the project's virtual environment | [`sandbox.ts:159`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/utils/sandbox.ts#L159), [`875`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/utils/sandbox.ts#L875) |
| `.gemini/worktrees/<name>` | `--worktree` checkouts | [`core/src/services/worktreeService.ts:117-133`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/services/worktreeService.ts#L117-L133) |
| `.agents/skills` | skills shared with other agents | [`storage.ts:356-358`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/storage.ts#L356-L358) |

The repository's `docs/` also names `.gemini/hooks/` and `.gemini/sandbox.Dockerfile`. No
per-project database exists.

`.gemini/worktrees/<name>` is a full git worktree on the new branch `worktree-<name>`, whose
`.git` is a file pointing into the project's own `.git/worktrees/`
([`worktreeService.ts:117-133`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/services/worktreeService.ts#L117-L133)).
`gemini --worktree [name]` (alias `-w`) creates it only when `experimental.worktrees` is on, off
by default
([`cli/src/config/config.ts:141-161`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/config/config.ts#L141-L161),
[`275-276`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/config/config.ts#L275-L276);
[`settingsSchema.ts:2234-2243`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/config/settingsSchema.ts#L2234-L2243)).
Without a name the source names it `<YYYY-MM-DD-HH-MM-SS-mmm>-<4 random characters>`
([`worktreeService.ts:31-41`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/services/worktreeService.ts#L31-L41)), while
[Git Worktrees: How to use Git worktrees](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/docs/cli/git-worktrees.md#how-to-use-git-worktrees)
gives `worktree-a1b2c3d4` as the example. The worktree is the project's base directory: the
worktree root is the parent of the common git directory
([`worktreeService.ts:96-115`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/services/worktreeService.ts#L96-L115)). Nothing
removes it: `maybeCleanup` has no caller, and
[Git Worktrees: How to exit a Git worktree session](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/docs/cli/git-worktrees.md#how-to-exit-a-git-worktree-session)
says the CLI leaves the worktree and branch for the user to remove
([`worktreeService.ts:62-84`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/services/worktreeService.ts#L62-L84),
[`186-233`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/services/worktreeService.ts#L186-L233)). It is a copy of the
repository with the agent's edits in it, so the `.gemini/worktrees` exclusion skips it; what the
agent changed there is in the transcript. How a session runs there is in
[Worktree sessions](#worktree-sessions). On the Linux install of 0.66.0, with
`experimental.worktrees` unset, `gemini --worktree lab` printed the command's help and created no
checkout, which is what the tag's argument check gives for the flag without the setting
([`config.ts` at `v0.66.0`](https://github.com/google-gemini/gemini-cli/blob/970413207732f5d4d8c50540c7fc359c44fdaaf2/packages/cli/src/config/config.ts#L269-L271)).
With `experimental.worktrees` set to `true` in `settings.json` before the start, the same command
created `<project>/.gemini/worktrees/lab`, holding a `.git` file and the project's `calc.py`,
`README.md` and `.gitignore`, and `<project>/.git/worktrees/lab` with `commondir`, `gitdir` and
`HEAD`.

On the same install `gemini mcp add <name> <command> <args...>` run in the project wrote the
project's `.gemini/settings.json` (186 bytes) with `mcpServers.<name>.command` and its `args`.

### Where the project path is recorded

- `.gemini/tmp/<slug>/.project_root` and `.gemini/history/<slug>/.project_root` hold the plain
  absolute path, trimmed
  ([`projectRegistry.ts:247-250`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/projectRegistry.ts#L247-L250),
  [`382-397`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/projectRegistry.ts#L382-L397)), lower-cased on Windows
  ([`95-100`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/projectRegistry.ts#L95-L100)).
- `.gemini/projects.json` maps each absolute path to its slug
  ([`projectRegistry.ts:16-21`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/projectRegistry.ts#L16-L21);
  [Projects registry](#projectsjson)).
- `.gemini/trustedFolders.json` is keyed by folder path
  ([`storage.ts:131`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/storage.ts#L131)).
- A session file carries only the SHA-256 of the project path
  ([`chatRecordingService.ts:582`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/services/chatRecordingService.ts#L582),
  [`856-863`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/services/chatRecordingService.ts#L856-L863)), which matches
  the older hash-named directories and cannot be reversed.
- `.gemini/trusted_hooks.json` and `.gemini/acknowledgments/agents.json` are keyed by absolute
  project path ([Standing permission files](#standing-permission-files)).
- A `.project_root` under `<project>/.gemini/worktrees/` names a scratch worktree, not a separate
  project.

## Parsing

### Transcript stores

Transcripts live in the per-project directory `tmp/<slug>/` of the runtime directory, named by
the slug `projects.json` maps from the project path; the older hash-named directory may sit beside
it ([Per-user storage](#per-user-storage)).

- Session: `tmp/<slug>/chats/session-<YYYY-MM-DDTHH-MM>-<id8>.jsonl`, one file per session, the
  time the session's UTC start to the minute and `<id8>` the first eight characters of the
  session id
  ([`chatRecordingService.ts:799-823`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/services/chatRecordingService.ts#L799-L823);
  prefix
  [`core/src/services/chatRecordingTypes.ts:12`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/services/chatRecordingTypes.ts#L12)).
  A name already taken becomes `session-<YYYY-MM-DDTHH-MM>-<n>-<id8>.jsonl`, `<n>` counting from 1
  ([`819-821`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/services/chatRecordingService.ts#L819-L821)).
- Subagent session: `tmp/<slug>/chats/<parent session id>/<session id>.jsonl`
  ([`787-815`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/services/chatRecordingService.ts#L787-L815)).
- Legacy session: `session-*.json`, one JSON object (a `ConversationRecord`)
  ([`1647-1697`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/services/chatRecordingService.ts#L1647-L1697)); on resume it
  is written again into a sibling `.jsonl`
  ([`732-759`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/services/chatRecordingService.ts#L732-L759)).
- Large tool output kept out of the transcript: `tmp/<slug>/tool-outputs/`, per session under
  `session-<session id>/`, one `<tool>_<id>.txt` per output
  ([`fileUtils.ts:839-857`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/utils/fileUtils.ts#L839-L857)).

On the Linux install of 0.66.0 session files took the `session-<YYYY-MM-DDTHH-MM>-<id8>.jsonl`
form. The restart that follows a folder trust choice ("Gemini CLI is restarting to apply the
trust changes") deleted the first session file and began a new one. `/clear` began a new
session file in the same process and kept the old one, and `gemini --resume latest` appended to
the file of the session it reopened. A short traceback paste and a long release-notes paste,
each sent as one bracketed paste (the terminal's markers around pasted text) followed by a
typed instruction, were each answered with a one-sentence summary; how the session file stores
a paste was not read.

The past-use stores beside the session file are in [Other stores](#other-stores).

#### What survives a delete

`gemini --delete-session <index or id>` and the `x` key of the `/resume` browser
([Session management: Deleting sessions](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/docs/cli/session-management.md#deleting-sessions))
remove the session's chat file, an activity log at the default path `logs/session-<id>.jsonl`
when one exists ([Activity log](#activity-log)), its `tool-outputs/session-<id>/` and its
`tmp/<slug>/<session id>/` directory of plans, trackers and tasks, with its subagent sessions
([`core/src/utils/sessionOperations.ts:58-116`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/utils/sessionOperations.ts#L58-L116),
[`123-169`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/utils/sessionOperations.ts#L123-L169)). There is no remote copy
to offer to delete.

On the Linux install of 0.66.0 `gemini --list-sessions` printed
`Available sessions for this project (<n>):` and, per session, its number, first prompt, age and
session id; `gemini --delete-session 1` printed `Deleted session 1: <prompt> (<age>)` and removed
that session's `chats/session-*.jsonl` (180 KB) and nothing else, the session having no activity
log, tool outputs or session directory; `logs.json`, the `/bug` export and
`history/<slug>/.project_root` stayed. Removing the project directory removed what lay under it
and nothing under `~/.gemini`. A session started and ended with no prompt deleted its own file
(of about 7.3 KB) on exit, through the exit cleanup listed below, each time on that install. A
600-second idle period in a running session wrote nothing. The error reports in `/tmp`
([Error reports](#error-reports)) were gone after a reboot, since Ubuntu clears `/tmp` at boot.

Retention removes more without being asked
([`cli/src/utils/sessionCleanup.ts:102-260`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/utils/sessionCleanup.ts#L102-L260);
[Session management: Session retention](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/docs/cli/session-management.md#session-retention)),
so a collection taken after the agent started again holds less than one taken before:

- Every start runs `cleanupExpiredSessions`
  ([`cli/src/gemini.tsx:803`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/gemini.tsx#L803)).
  `general.sessionRetention.enabled` defaults to true and `maxAge` to `30d`, with an optional
  `maxCount` and a `minRetention` floor of `1d`
  ([`settingsSchema.ts:389-440`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/config/settingsSchema.ts#L389-L440)), so a
  default install keeps about a month of sessions per project. The cleanup also deletes every
  `session-*` file it cannot read, that lacks a `sessionId` or that has no resumable content
  ([`sessionCleanup.ts:302-311`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/utils/sessionCleanup.ts#L302-L311);
  [`cli/src/utils/sessionUtils.ts:266-303`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/utils/sessionUtils.ts#L266-L303)),
  and with each file what the delete command removes
  ([`sessionCleanup.ts:216-222`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/utils/sessionCleanup.ts#L216-L222)). A
  session file truncated badly enough to fail loading is therefore removed by the next start.
- With retention on, every start also deletes files directly in `tool-outputs/` older than
  `maxAge` or beyond `maxCount`
  ([`sessionCleanup.ts:479-560`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/utils/sessionCleanup.ts#L479-L560);
  [`gemini.tsx:508-513`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/gemini.tsx#L508-L513)).
- On exit a session with no resumable content (no real prompt, reply or tool activity) deletes
  its own file
  ([`chatRecordingService.ts:1327-1370`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/services/chatRecordingService.ts#L1327-L1370)).
- Background shell logs are deleted after seven days
  ([`shellExecutionService.ts:390-392`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/services/shellExecutionService.ts#L390-L392);
  [`cli/src/utils/logCleanup.ts:11-43`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/utils/logCleanup.ts#L11-L43)).
- Every start deletes the whole `tmp/<slug>/checkpoints/` directory
  ([Tool checkpoints](#tool-checkpoints)), and pasting an image deletes clipboard images older
  than an hour ([`clipboardUtils.ts:397-418`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/ui/utils/clipboardUtils.ts#L397-L418)).
- A session run in the container sandbox leaves nothing under the host's `~/.gemini`
  ([Per-user storage](#per-user-storage)).

What remains of a deleted or expired session: the per-project prompt history `logs.json`, which
neither the delete command nor retention touches; `shell_history`, the last 100 shell-mode
commands with no time or session id; the `/chat save` checkpoints, removed only by
`/chat delete <tag>`; the shadow git repository under `history/<slug>`; `memory/`; and the
`bug-report-history-*.json` files from `/bug`; and an error report in the system temporary
directory. The agent writes no deletion marker, and the
requests already sent to Code Assist and Clearcut under the session's `sessionId` are server-side
records held by Google, which a responder can only ask Google for
([Sync, relay and origin rows](#sync-relay-and-origin-rows)). The analyzer reads `logs.json`,
`shell_history`, the `/chat save` checkpoints, the `/bug` history exports, the tool checkpoints
and a copied error report as secondary sources
([Secondary sources](../analyzer/docs/output.md#secondary-sources)), and the activity log's
records, `memory/` and the clipboard images as stores of their own, but not the shadow
repository or the turns in the activity log's bodies ([Parser plan](#parser-plan)).

So a session missing from `chats/` on a host whose `settings.json` sets no
`general.sessionRetention` was either deleted by the user or more than 30 days old when the agent
last started in that project; an expired session's prompts are still in `logs.json` and its shell
commands in `shell_history`, and its turns are in a `/chat save` checkpoint or a `/bug` export
when one was taken. The retention rows of the parser ([Deletion rows](#deletion-rows)) name the
window when the file sets one.

#### Worktree sessions

`gemini --worktree [name]` creates the worktree and changes into it before the configuration is
loaded ([`cli/src/utils/worktreeSetup.ts:21-42`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/utils/worktreeSetup.ts#L21-L42);
[`gemini.tsx:497-506`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/gemini.tsx#L497-L506)), so the session's
`.project_root` and slug are the worktree's
([`core/src/config/config.ts:1323`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/config.ts#L1323)).
On the Linux install of 0.66.0 the session of `gemini --worktree lab` got the slug `lab`:
`history/lab/.project_root` and `tmp/lab/.project_root` (41 bytes each), its session file under
`tmp/lab/chats/` (957 bytes at the start, 2.5 KB after one question), `tmp/lab/logs.json` (2 bytes
at the start, 184 after) and an empty `tmp/lab/logs/` directory.

#### Driven from Zed

Zed's agent panel can run Gemini CLI as an external agent over ACP (the Agent Client Protocol,
the JSON-RPC protocol an editor uses to drive another agent); Zed's sidebar then lists the session
([Transcript stores in `zed.md`](zed.md#transcript-stores)), while the transcript stays in Gemini
CLI's own store
([`agent_servers/src/custom.rs:17-19`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/agent_servers/src/custom.rs#L17-L19)).
Gemini CLI makes a new ACP session's id with `randomUUID()` and passes it as the session id of
the configuration that records the transcript, so the id an ACP client holds is the `sessionId`
of the session file
([`cli/src/acp/acpSessionManager.ts:66-77`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/acp/acpSessionManager.ts#L66-L77),
[`426`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/acp/acpSessionManager.ts#L426);
[`cli/src/config/config.ts:594-599`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/config/config.ts#L594-L599),
[`970-975`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/config/config.ts#L970-L975);
[`core/src/config/config.ts:1003`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/config.ts#L1003)).

### Record schema

#### Session JSONL

One JSON object per line, appended with `JSON.stringify`
([`chatRecordingService.ts:942-958`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/services/chatRecordingService.ts#L942-L958));
the types are in
[`chatRecordingTypes.ts`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/services/chatRecordingTypes.ts). The first line is
the metadata record
([`chatRecordingTypes.ts:156-165`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/services/chatRecordingTypes.ts#L156-L165);
writer [`chatRecordingService.ts:856-865`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/services/chatRecordingService.ts#L856-L865)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `sessionId` | string | | |
| `projectHash` | string | | |
| `startTime` | string | | |
| `lastUpdated` | string | | |
| `kind` | string | `main`, `subagent` | |
| `directories` | array | | |
| `summary` | string | | |
| `memoryScratchpad` | object | | |

`directories` is written for a subagent session only. Later lines are one of four kinds,
told apart by their keys
([`chatRecordingService.ts:79-95`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/services/chatRecordingService.ts#L79-L95)).
A message record
([`chatRecordingTypes.ts:44-87`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/services/chatRecordingTypes.ts#L44-L87);
writers [`chatRecordingService.ts:1064-1110`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/services/chatRecordingService.ts#L1064-L1110),
[`1168-1203`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/services/chatRecordingService.ts#L1168-L1203)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `id` | string | | |
| `timestamp` | string | | |
| `type` | string | `user`, `info`, `error`, `warning`, `gemini` | |
| `content` | string or array or object | | |
| `displayContent` | string or array or object | | |
| `toolCalls` | array | | |
| `thoughts` | array | | |
| `tokens` | object or null | | |
| `model` | string | | |

`toolCalls`, `thoughts`, `tokens` and `model` are on `gemini` messages only; `id` is a UUID and
`timestamp` ISO 8601 from `toISOString()`. `content` is a `PartListUnion` (a string, a `Part` or
a `Part[]`). A `gemini` message is written with `thoughts` (often `[]`), `tokens` (`null` until the
usage arrives) and `model`, and written whole again whenever its tokens or tool calls change
([`1086-1110`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/services/chatRecordingService.ts#L1086-L1110),
[`1137-1166`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/services/chatRecordingService.ts#L1137-L1166)).

A `toolCalls[]` element
([`chatRecordingTypes.ts:54-67`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/services/chatRecordingTypes.ts#L54-L67);
built at [`core/src/core/geminiChat.ts:1842-1871`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/core/geminiChat.ts#L1842-L1871),
enriched at [`chatRecordingService.ts:1172-1181`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/services/chatRecordingService.ts#L1172-L1181)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `id` | string | | |
| `name` | string | | |
| `args` | object | | |
| `result` | array or object or string or null | | |
| `status` | string | `validating`, `scheduled`, `error`, `success`, `executing`, `cancelled`, `awaiting_approval` | |
| `timestamp` | string | | |
| `agentId` | string | | |
| `displayName` | string | | |
| `description` | string | | |
| `resultDisplay` | string or object | | |
| `renderOutputAsMarkdown` | boolean | | |

`status` is a `CoreToolCallStatus`
([`core/src/scheduler/types.ts:26-34`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/scheduler/types.ts#L26-L34)).
`args` holds the tool's parameters, named by the tool, such as `file_path` and `dir_path`
([`core/src/tools/definitions/base-declarations.ts:16-17`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/tools/definitions/base-declarations.ts#L16-L17)).
A `result` is the `Part[]` that `convertToFunctionResponse` builds: one `functionResponse` part
(`id`, `name`, `response`), followed by the `fileData` parts (`fileUri`, `mimeType`) and
`inlineData` parts (`data`, `mimeType`) the tool returned, which a model that takes media inside
a function response gets nested as the `functionResponse`'s own `parts` instead
([`core/src/utils/generateContentResponseUtilities.ts:23-35`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/utils/generateContentResponseUtilities.ts#L23-L35),
[`49-182`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/utils/generateContentResponseUtilities.ts#L49-L182);
[`core/src/agent/content-utils.ts:80-89`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/agent/content-utils.ts#L80-L89)).
The same parts can sit in a `user` message's `content`, which records every part of the prompt
([`geminiChat.ts:545-567`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/core/geminiChat.ts#L545-L567)),
an image or PDF read for it as an `inlineData` part
([`core/src/utils/fileUtils.ts:752-768`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/utils/fileUtils.ts#L752-L768)).
The `response` object of a tool's `functionResponse` part
([`core/src/scheduler/tool-executor.ts:446-454`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/scheduler/tool-executor.ts#L446-L454)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `output` | string | | |
| `error` | string | | |

`output` is the tool's text for the model, absent when the tool returned only media; `error`
replaces it for a failed or cancelled call.
An object `resultDisplay` is one of the tool display shapes, among them a file diff (`fileDiff`,
`fileName`, `filePath`, `originalContent`, `newContent`) and a directory listing (`summary`,
`files`)
([`core/src/tools/tools.ts:935-966`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/tools/tools.ts#L935-L966)).

A `thoughts[]` element, a thought summary with the time it arrived
([`core/src/utils/thoughtUtils.ts:7-10`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/utils/thoughtUtils.ts#L7-L10);
[`chatRecordingService.ts:1129-1135`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/services/chatRecordingService.ts#L1129-L1135)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `subject` | string | | |
| `description` | string | | |
| `timestamp` | string | | |

`tokens`
([`chatRecordingTypes.ts:19-26`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/services/chatRecordingTypes.ts#L19-L26);
[`chatRecordingService.ts:1137-1148`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/services/chatRecordingService.ts#L1137-L1148)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `input` | number | | |
| `output` | number | | |
| `cached` | number | | |
| `thoughts` | number | | |
| `tool` | number | | |
| `total` | number | | |

`memoryScratchpad`, the memory tool's working notes for the session
([`chatRecordingTypes.ts:28-39`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/services/chatRecordingTypes.ts#L28-L39)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `version` | number | `1` | |
| `workflowSummary` | string | | |
| `toolSequence` | array | | |
| `touchedPaths` | array | | |
| `validationStatus` | string | `passed`, `failed`, `unknown` | |

The three update records
([`chatRecordingTypes.ts:123-154`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/services/chatRecordingTypes.ts#L123-L154)):
`{"$set": {...}}` merges metadata keys (any key of the metadata table, and the legacy full
`messages` list); `{"$patch": {...}}` changes messages already written; `{"$rewindTo": "<message
id>"}` drops that message and everything after it
([`chatRecordingService.ts:307-337`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/services/chatRecordingService.ts#L307-L337)).
The `$patch` object:

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `id` | string | | |
| `content` | string or array or object | | |
| `toolCalls` | array | | |
| `updates` | array | | |
| `removeIds` | array | | |
| `orderIds` | array | | |

A `$patch` `toolCalls[]` element holds `id` and `result`, and an `updates[]` element `id`,
`content` and `toolCalls`.

The writer records model text only as `content`
([`geminiChat.ts:1808-1812`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/core/geminiChat.ts#L1808-L1812)), tool calls
in `toolCalls`; the reader also accepts `functionCall` and `thought: true` parts inside `content`
([`core/src/utils/sessionUtils.ts:179-185`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/utils/sessionUtils.ts#L179-L185)).
No working directory and no git branch are stored in the session file.

The main session of the Linux install of 0.66.0 (184,533 bytes) used a subset of these keys: the
metadata line held `kind`, `lastUpdated`, `projectHash`, `sessionId` and `startTime`; `user`
messages held `content`, `id`, `timestamp` and `type`, and `gemini` messages those keys with
`model`, `thoughts`, `tokens` and `toolCalls`; `info` and `error` messages appeared, the errors
holding the failed turns of [Error reports](#error-reports), and no `warning` message; all three
update records appeared. Shorter sessions held only `$set` updates.

#### logs.json records

`logs.json` is a JSON array of `LogEntry` objects
([`logger.ts:17-27`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/core/logger.ts#L17-L27)), user prompts only:

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `sessionId` | string | | |
| `messageId` | number | | |
| `timestamp` | string | | |
| `type` | string | `user` | |
| `message` | string | | |

`messageId` counts the session's prompts and `timestamp` is ISO 8601.

#### Checkpoint records

A `/chat save` checkpoint `checkpoint-<tag>.json` is one object
([`logger.ts:29-32`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/core/logger.ts#L29-L32)); a `/bug` history export is
the bare `history` array:

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `history` | array | | |
| `authType` | string | | |

Each `history` item is a `Content`
([`geminiChat.ts:1311-1313`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/core/geminiChat.ts#L1311-L1313)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `role` | string | | |
| `parts` | array | | |

The CLI writes the roles `user` and `model`. Each part carries `text` (with `thought: true` for a
thought), `functionCall` (`id`, `name`, `args`) or `functionResponse` (`id`, `name`, `response`).

A tool checkpoint in `checkpoints/` is a `ToolCallData`
([`core/src/utils/checkpointUtils.ts:15-24`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/utils/checkpointUtils.ts#L15-L24),
[`130-139`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/utils/checkpointUtils.ts#L130-L139)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `history` | array | | |
| `clientHistory` | array | | |
| `commitHash` | string | | |
| `toolCall` | object | | |
| `messageId` | string | | |

`toolCall` holds `name` and `args` (an edit tool's `file_path` among them); `messageId` equals
the prompt id, `prompt_id`, which the interface builds as `<session id>########<prompt count>`
([`cli/src/ui/hooks/useGeminiStream.ts:1884-1886`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/ui/hooks/useGeminiStream.ts#L1884-L1886)),
so it names the session. `clientHistory` is the model history, `Content` items as above;
`history` is the interface's display list, `HistoryItem` objects
([`useGeminiStream.ts:2407-2414`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/ui/hooks/useGeminiStream.ts#L2407-L2414);
[`cli/src/ui/types.ts:162-174`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/ui/types.ts#L162-L174),
[`453`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/ui/types.ts#L453)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `id` | number | | |
| `type` | string | `user`, `gemini`, `gemini_content`, `info`, `error`, `warning` | |
| `text` | string | | |

`type` has further values for display-only items. Only a `replace` or `write_file` call awaiting
approval gets one, and only with `general.checkpointing.enabled` on, off by default
([`useGeminiStream.ts:2389-2397`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/ui/hooks/useGeminiStream.ts#L2389-L2397);
[`core/src/tools/tool-names.ts:167`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/tools/tool-names.ts#L167);
[`cli/src/config/config.ts:1038`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/config/config.ts#L1038);
[`settingsSchema.ts:299-317`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/config/settingsSchema.ts#L299-L317)).

### Joins and timestamps

A session is a file. Its `sessionId` comes from line 1 and is overridden by a later
`$set.sessionId`, which a resume rewrites
([`chatRecordingService.ts:761`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/services/chatRecordingService.ts#L761)).
A tool call and its result are one `toolCalls[]` element (`id`, `result`, `status`); the `id` is
`<tool name>__` before the model's call id, or before `<tool name>_<epoch ms>_<n>` when the model
gives none (`synth_<prompt id>_<epoch ms>_<n>` with context management on)
([`core/src/core/turn.ts:474-482`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/core/turn.ts#L474-L482)),
and the whole `gemini` message is appended again whenever its tool calls change
([`1168-1237`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/services/chatRecordingService.ts#L1168-L1237),
[`1039-1055`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/services/chatRecordingService.ts#L1039-L1055)), so a reader
keeps the last record per `id` and applies `$patch` and `$rewindTo`. Every time is UTC ISO 8601
with milliseconds, except the activity log's epoch milliseconds. The project path is the
`tmp/<slug>/.project_root` content or a reverse lookup in `projects.json`; a hash-named directory
has no reverse map but matches the SHA-256 of a known path. `kind` and the directory nesting link a
subagent session to its parent.

### Other stores

No SQLite store exists. `history/<slug>` is a shadow git repository and is treated as binary.
Each store below is collected under `.gemini`, the error reports excepted, and every one but
`projects.json` and `state.json` is parsed ([Parser plan](#parser-plan)).

#### logs.json

`tmp/<slug>/logs.json`, the prompt history (secondary, parsed): user prompts only, in the form of
[logs.json records](#logsjson-records)
([`logger.ts:229-233`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/core/logger.ts#L229-L233)). Neither the delete command
nor retention ([What survives a delete](#what-survives-a-delete)) touches it (`logs.json` and
`LOG_FILE_NAME` occur in neither
[`sessionOperations.ts`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/utils/sessionOperations.ts) nor
[`sessionCleanup.ts`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/utils/sessionCleanup.ts)), so it keeps
every prompt of an expired or deleted session, with its session id and time, beside the
`/chat save` checkpoints and `/bug` exports that hold the turns of those taken. On the Linux
install of 0.66.0 its entries had the keys of that table (5,071 bytes for the sample project),
and what the delete left is in [What survives a delete](#what-survives-a-delete).

#### shell_history

`tmp/<slug>/shell_history`
([`storage.ts:480-482`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/storage.ts#L480-L482)) (secondary, parsed):
the commands the user ran in shell mode (`!`), plain text, written as the list joined with `\n`
and no final newline, read back by splitting on `\n` or `\r\n`, skipping blank lines and joining a
line whose predecessor ends in an odd number of backslashes to it with a space, the backslash
dropped; oldest first, deduplicated and capped at 100, with no time and no session id
([`cli/src/ui/hooks/useShellHistory.ts:12`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/ui/hooks/useShellHistory.ts#L12),
[`31-71`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/ui/hooks/useShellHistory.ts#L31-L71),
[`89-100`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/ui/hooks/useShellHistory.ts#L89-L100)). The session file repeats
each command inside a synthetic user message `I ran the following shell command: ...`
([`cli/src/ui/hooks/useExecutionLifecycle.ts:44-69`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/ui/hooks/useExecutionLifecycle.ts#L44-L69);
[`geminiChat.ts:1345-1357`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/core/geminiChat.ts#L1345-L1357)), so it joins
the session on text only, and not even there: the session's copy wraps the command in that
sentence. The command is also a `logs.json` prompt, since the prompt is logged before the
shell-mode branch
([`cli/src/ui/hooks/useGeminiStream.ts:1109-1113`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/ui/hooks/useGeminiStream.ts#L1109-L1113);
[`cli/src/ui/components/InputPrompt.tsx:419-421`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/ui/components/InputPrompt.tsx#L419-L421)),
so a `shell_history` line and the `logs.json` entry of the same command are two copies of one
event. It survives deletion and retention, since it is per project, not per session.

#### /chat save checkpoints

`tmp/<slug>/checkpoint-<encodeURIComponent(tag)>.json` from `/chat save`
([`logger.ts:285-295`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/core/logger.ts#L285-L295),
[`329-343`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/core/logger.ts#L329-L343);
[`cli/src/ui/commands/chatCommand.ts:139-142`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/ui/commands/chatCommand.ts#L139-L142);
[Session management: Manual chat checkpoints](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/docs/cli/session-management.md#manual-chat-checkpoints))
(secondary, parsed): a full `history` with no time or session id
([Checkpoint records](#checkpoint-records)), written with `JSON.stringify` of `{history,
authType}` in that order, the tag encoded with `encodeURIComponent`
([`logger.ts:46-48`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/core/logger.ts#L46-L48)). The first item is a `user`
item holding the environment context (the workspace directories and their structure) the CLI
sends before the first prompt
([`core/src/utils/environmentContext.ts:13`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/utils/environmentContext.ts#L13),
[`101-108`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/utils/environmentContext.ts#L101-L108)). Only
`/chat delete <tag>` removes it
([`logger.ts:393`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/core/logger.ts#L393)), so it survives the session's
deletion and expiry; it joins on text. On the Linux install of 0.66.0 `/chat save lab` wrote
`tmp/<slug>/checkpoint-lab.json` (63.1 KB).

#### Tool checkpoints

`tmp/<slug>/checkpoints/<ISO time with : and . replaced>-<file basename>-<tool>.json`
([`checkpointUtils.ts:59-66`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/utils/checkpointUtils.ts#L59-L66),
[`130-141`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/utils/checkpointUtils.ts#L130-L141);
[`storage.ts:364-366`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/storage.ts#L364-L366)) (secondary, parsed):
the `clientHistory` before each file-changing tool call
([Checkpoint records](#checkpoint-records)), with the shadow repository commit at
`history/<slug>`
([`storage.ts:326-330`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/storage.ts#L326-L330);
[`gitService.ts:74-75`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/services/gitService.ts#L74-L75)). The CLI deletes
the whole directory at every start
([`cli/src/utils/cleanup.ts:221-231`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/utils/cleanup.ts#L221-L231); called
from [`gemini.tsx:508-513`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/gemini.tsx#L508-L513)), so only the checkpoints
of the last run in each project exist. The shadow repository is not cleaned. A checkpoint
joins the session on the session id its `messageId` names and on text, its tool items on the
call id; the time in its name is when it was written, in milliseconds.

#### memory/

`tmp/<slug>/memory/` (a projection, parsed as a store of its own): `MEMORY.md` and skills the
memory tool writes, and, under `experimental.autoMemory` (off by default), `.inbox/<kind>/*.patch`
files a background agent extracts from past sessions
([`storage.ts:332-342`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/storage.ts#L332-L342);
[`memoryTool.ts:12`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/tools/memoryTool.ts#L12);
[`settingsSchema.ts:2432-2441`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/config/settingsSchema.ts#L2432-L2441);
[Auto Memory: How Auto Memory works](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/docs/cli/auto-memory.md#how-auto-memory-works)). It holds
no session id or time and outlives the sessions it was drawn from. Although drawn from sessions,
it is primary: its files are the memory tool's and the extraction agent's own summaries, which no
session file repeats, so there is nothing to deduplicate them against. They are text with no
record structure and have no key table.

#### Clipboard images

`tmp/<slug>/images/clipboard-<epoch ms>.<ext>` (parsed as a store of its own): an image pasted
from the clipboard, `png` on Linux and Windows and `png` or `jpg` on macOS, named by the time of
the paste
([`clipboardUtils.ts:258-283`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/ui/utils/clipboardUtils.ts#L258-L283),
[`298`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/ui/utils/clipboardUtils.ts#L298),
[`331-342`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/ui/utils/clipboardUtils.ts#L331-L342)).
The cleanup that runs at each paste deletes a `clipboard-*` file with an extension of
`IMAGE_EXTENSIONS` (`.png`, `.jpg`, `.jpeg`, `.webp`, `.heic`, `.heif`) older than an hour
([`22-29`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/ui/utils/clipboardUtils.ts#L22-L29),
[`397-418`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/ui/utils/clipboardUtils.ts#L397-L418)).
An image is binary with no record structure and no key table; the prompt that pasted it names
its path.

#### Activity log

The file `GEMINI_CLI_ACTIVITY_LOG_TARGET` names, a path outside every catalog line unless the
variable names one under the home; the writer's default path
`tmp/<slug>/logs/session-<session id>.jsonl`
([`activityLogger.ts:772-778`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/utils/activityLogger.ts#L772-L778))
is the one the parser reads (parsed as a store of its own, its records only), and no build has
been seen to write it (when it is written is below): one JSON object per line
([`activityLogger.ts:767-809`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/utils/activityLogger.ts#L767-L809)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `type` | string | `console`, `network` | |
| `payload` | object | | |
| `sessionId` | string | | |
| `timestamp` | number | | |

`timestamp` is epoch milliseconds, the time of the write. A `console` payload is a
`ConsoleLogPayload` with the time it was logged
([`core/src/utils/events.ts:73-76`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/utils/events.ts#L73-L76);
[`activityLogger.ts:754-761`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/utils/activityLogger.ts#L754-L761)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `type` | string | `log`, `warn`, `error`, `debug`, `info` | |
| `content` | string | | |
| `timestamp` | number | | |

A `network` payload is a `NetworkLog`, the request and response of an HTTP call other than to
`127.0.0.1` or `localhost`, the model calls among them
([`activityLogger.ts:102-119`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/utils/activityLogger.ts#L102-L119),
[`302`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/utils/activityLogger.ts#L302),
[`481`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/utils/activityLogger.ts#L481)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `id` | string | | |
| `timestamp` | number | | |
| `method` | string | | |
| `url` | string | | |
| `headers` | object | | |
| `body` | string | | |
| `pending` | boolean | | |
| `chunk` | object | | |
| `response` | object | | |
| `error` | string | | |

One call is several records sharing its random `id`: the request (`method`, `url`, `headers`,
`body`, `pending` true), one `chunk` record per piece of a streamed response, then a last record
with `pending` false and either `response` or `error`
([`334-342`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/utils/activityLogger.ts#L334-L342),
[`365-374`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/utils/activityLogger.ts#L365-L374),
[`381-390`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/utils/activityLogger.ts#L381-L390),
[`436`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/utils/activityLogger.ts#L436)).
A `chunk` object and a `response` object:

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `index` | number | | |
| `data` | string | | |
| `timestamp` | number | | |

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `status` | number | | |
| `headers` | object | | |
| `body` | string | | |
| `durationMs` | number | | |

The request `headers` have `authorization`, `cookie` and `x-goog-api-key` replaced by
`[REDACTED]`, the response `headers` `set-cookie`; no other header and no body is redacted
([`224-258`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/utils/activityLogger.ts#L224-L258)).
A model request's `body` is the request as JSON: `contents`, the `Content[]` history sent, at the
top for the Gemini API and under `request` for Code Assist
([`code_assist/converter.ts:33-51`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/code_assist/converter.ts#L33-L51),
[`163-177`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/code_assist/converter.ts#L163-L177)).
A chat turn, of the main conversation or of a subagent, is a `GeminiChat` request, and
`GeminiChat` is the only caller of the streaming method, `streamGenerateContent` in the URL
([`core/src/core/geminiChat.ts:1220-1227`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/core/geminiChat.ts#L1220-L1227);
[`code_assist/server.ts:118-119`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/code_assist/server.ts#L118-L119);
[`core/src/core/client.ts:429`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/core/client.ts#L429);
[`core/src/agents/local-executor.ts:1115`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/agents/local-executor.ts#L1115)).
A subagent's requests are logged under the parent's `sessionId`, its history opening with its own
first item. The utility requests send a trimmed history plus an internal prompt through
`generateContent` or `countTokens`
([`core/src/core/baseLlmClient.ts:131`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/core/baseLlmClient.ts#L131),
[`240`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/core/baseLlmClient.ts#L240),
[`365`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/core/baseLlmClient.ts#L365);
[`server.ts:205-206`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/code_assist/server.ts#L205-L206),
[`339-341`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/code_assist/server.ts#L339-L341)):
the loop detector's `<original_user_request>` and "Please analyze the conversation history"
prompt
([`core/src/services/loopDetectionService.ts:568-599`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/services/loopDetectionService.ts#L568-L599)),
the classifier router
([`core/src/routing/strategies/classifierStrategy.ts:161-176`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/routing/strategies/classifierStrategy.ts#L161-L176)),
the next-speaker check
([`core/src/utils/nextSpeakerChecker.ts:106-115`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/utils/nextSpeakerChecker.ts#L106-L115))
and compression
([`core/src/context/chatCompressionService.ts:595`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/context/chatCompressionService.ts#L595),
[`620`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/context/chatCompressionService.ts#L620)).
Each chat request repeats the whole history of its chat up to it, under the record's
`sessionId`, its items carrying no time of their own. Every chat history opens with the
environment-context item, a `user` item whose text is a `<session_context>` block (the date, the
platform, the project's temporary directory, optionally the directory tree and the session
memory) that the CLI sends and the session file does not record as a message
([`core/src/utils/environmentContext.ts:68-77`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/utils/environmentContext.ts#L68-L77),
[`84-111`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/utils/environmentContext.ts#L84-L111)).

The response `body` is recorded as received: the joined text of the stream, for a chat request
the server-sent events. The Code Assist reader buffers each `data:` line, less its six-character
`data: ` prefix, until a blank line and joins the buffered lines with a newline into one event's
JSON, ignoring other lines
([`server.ts:497-533`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/code_assist/server.ts#L497-L533)).
Each chunk is a `GenerateContentResponse`, wrapped under `response` with a `traceId` for Code
Assist
([`converter.ts:77-87`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/code_assist/converter.ts#L77-L87),
[`145-155`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/code_assist/converter.ts#L145-L155)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `response` | object | | |
| `traceId` | string | | |
| `candidates` | array | | |

A `candidates[]` element holds `content`, a `Content` of the `model` role whose parts are a piece
of the reply; the chunks' parts, in order, are the reply the next chat request of the chat sends
back as its history item. The reply to a chat's last request is in no later request: for a
deleted session its response `body` is the only copy of that reply.

The logger starts in interactive mode with `general.devtools` on (off by default), and in
headless mode whenever `GEMINI_CLI_ACTIVITY_LOG_TARGET` is set
([`gemini.tsx:774-779`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/gemini.tsx#L774-L779);
[`cli/src/nonInteractiveCli.ts:98-103`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/nonInteractiveCli.ts#L98-L103);
[`settingsSchema.ts:247-255`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/config/settingsSchema.ts#L247-L255)),
but it writes a file only when that variable is set, and then to the path the variable names;
without it the records stay in memory until the DevTools window (opened with F12) takes them
over a WebSocket
([`cli/src/utils/devtoolsService.ts:121-132`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/utils/devtoolsService.ts#L121-L132);
[`activityLogger.ts:767-780`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/utils/activityLogger.ts#L767-L780),
[`1091-1095`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/utils/activityLogger.ts#L1091-L1095)).
The `v0.66.0` tag has the same code
([`devtoolsService.ts` at `v0.66.0`](https://github.com/google-gemini/gemini-cli/blob/970413207732f5d4d8c50540c7fc359c44fdaaf2/packages/cli/src/utils/devtoolsService.ts#L121-L130);
[`activityLogger.ts` at `v0.66.0`](https://github.com/google-gemini/gemini-cli/blob/970413207732f5d4d8c50540c7fc359c44fdaaf2/packages/cli/src/utils/activityLogger.ts#L767-L780),
[`1092`](https://github.com/google-gemini/gemini-cli/blob/970413207732f5d4d8c50540c7fc359c44fdaaf2/packages/cli/src/utils/activityLogger.ts#L1092);
[`nonInteractiveCli.ts` at `v0.66.0`](https://github.com/google-gemini/gemini-cli/blob/970413207732f5d4d8c50540c7fc359c44fdaaf2/packages/cli/src/nonInteractiveCli.ts#L95-L100)).
The default path is the file writer's fallback for an empty path, which neither caller passes.
On the Linux install of 0.66.0, `general.devtools` set to `true` in `settings.json` and a new
session with one question wrote no `logs/` directory, so the records above are source-derived;
the `--worktree` session's `tmp/<slug>/` held an empty `logs/` directory and no file
([Worktree sessions](#worktree-sessions)). The delete command removes a file at the default path
with the session when one exists.

#### Bug reports

`/bug` opens a GitHub new-issue URL carrying the CLI version and the session id, and, when the
history holds more than the initial context item, writes
`tmp/<slug>/bug-report-history-<epoch ms>.json` (secondary, parsed) and asks the user to attach it
by hand
([`bugCommand.ts:63-125`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/ui/commands/bugCommand.ts#L63-L125)). The file is
the conversation in the `exportHistoryToFile` form, the bare `Content[]` array of
`chat.getHistory()` written with `JSON.stringify`, the same items as a `/chat save` checkpoint
([`bugCommand.ts:85-91`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/ui/commands/bugCommand.ts#L85-L91);
[`cli/src/ui/utils/historyExportUtils.ts:61-70`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/ui/utils/historyExportUtils.ts#L61-L70)),
so it repeats the session store's turns with no time or session id of its own. It outlives a
deleted session and says nothing about whether it was uploaded; the epoch milliseconds in its name
are when `/bug` ran. On the Linux install of 0.66.0 `/bug <text>` printed a
`https://github.com/google-gemini/gemini-cli/issues/new?...` URL prefilled with the version and
the text, wrote an export of 86,047 bytes (an array of objects with `role` and `parts[].text`),
printed "Chat History Exported ... Please consider attaching this file", and opened no browser
and submitted nothing. `/bug-memory` writes the V8 heap snapshot
`bug-memory-<epoch ms>.heapsnapshot` to the same directory
([`cli/src/ui/commands/bugMemoryCommand.ts:26-41`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/ui/commands/bugMemoryCommand.ts#L26-L41)),
a dump of the CLI process's memory ([Credentials](#credentials)).

#### Error reports

`gemini-client-error-<type>-<ISO time with : and . replaced by ->.json` in the system temporary
directory, outside every home ([Per-user storage](#per-user-storage)) (secondary, parsed): one
object written with `JSON.stringify(..., null, 2)` when a model request or a chat start fails
([`errorReporting.ts:13-17`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/utils/errorReporting.ts#L13-L17),
[`26-60`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/utils/errorReporting.ts#L26-L60)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `error` | object | | |
| `context` | array or object | | |
| `additionalInfo` | object | | |

`error` holds `message` and, for an `Error`, `stack`; a report whose context cannot be
serialised is written with `error` alone
([`37-50`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/utils/errorReporting.ts#L37-L50),
[`76-81`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/utils/errorReporting.ts#L76-L81)).
`context` is the `Content[]` history: the curated history plus the failed prompt for
`Turn.run-sendMessageStream`
([`core/src/core/turn.ts:439-448`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/core/turn.ts#L439-L448)),
the starting history for `startChat`
([`core/src/core/client.ts:451-456`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/core/client.ts#L451-L456);
[`core/src/agents/local-executor.ts:1126-1131`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/agents/local-executor.ts#L1126-L1131)),
the request for `<context>-api` and `<context>-invalid-content`
([`core/src/core/baseLlmClient.ts:423-436`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/core/baseLlmClient.ts#L423-L436)),
or, for `generateContent-api`, an object of `requestContents` (that history) and
`requestConfig` (the generation settings)
([`client.ts:1239-1246`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/core/client.ts#L1239-L1246)).
The report carries no session id and no project; it repeats the turns sent with the request, so
it joins the session on text, its tool items on the call id. No catalog line reaches the system
temporary directory, so a collection holds a report only when the responder copied it by hand
([`collectors/docs/coverage.md`](../collectors/docs/coverage.md)); the analyzer reads one placed
directly in a home's runtime directory (`.gemini` or `.cache/.gemini`) or in its `tmp/<slug>/`,
which gives the report that directory's project.

On the Linux install of 0.66.0 each failed turn wrote one report to `/tmp`, mode 0664:
`gemini-client-error-Turn.run-sendMessageStream-<time>.json` (4.5 KB to 12.9 KB) and
`gemini-client-error-generateContent-api-<time>.json` (4.5 KB). The turns failed on a new key
from AI Studio (Google's developer console for Gemini API keys) whose project had the
Generative Language API (the Gemini API's Google Cloud service name) disabled for about fifteen
minutes (403 `SERVICE_DISABLED`), then on the free tier's daily quota for `gemini-3.8-flash`
("Usage limit reached for gemini-3.8-flash"), and the session file recorded each as an `error`
message. A prompt sent with an invalid stored key ([Credentials](#credentials)) wrote
`gemini-client-error-generateJson-api-<time>.json` (9.3 KB) and
`gemini-client-error-Turn.run-sendMessageStream-<time>.json` (10.0 KB). What a reboot left of
them is in [What survives a delete](#what-survives-a-delete).

#### projects.json

`projects.json` maps each absolute project path to its slug, a lower-case string of letters,
digits and hyphens
([`projectRegistry.ts:16-22`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/projectRegistry.ts#L16-L22)), written
through a write copy `projects.json.<uuid>.tmp`
([`109-123`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/projectRegistry.ts#L109-L123)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `projects` | object | | |

On the Linux install of 0.66.0 the file held one entry (57 bytes) after the first start in the
sample project and two entries (83 bytes) after a session in the home; the values were not read.

#### state.json

`state.json`, the interface's persistent state
([`persistentState.ts:11`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/utils/persistentState.ts#L11)),
and its copy `state.json.bak` (not parsed): on the Linux install of 0.66.0 both appeared at the
first start with mode 0600 and the keys `startupWarningCounts.home-directory`,
`startupWarningCounts.true-color` and `tipsShown`, and were rewritten at the sign-in and at a
resume. They name no prompt, session or project.

#### Standing permission files

Every file here is parsed ([Permission rows](#permission-rows)).

`policies/auto-saved.toml`, in `~/.gemini/policies/` for a grant the user saved for every project
and in `<project>/.gemini/policies/` for one saved for the workspace
([`storage.ts:28`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/storage.ts#L28),
[`150-152`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/storage.ts#L150-L152),
[`236-249`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/storage.ts#L236-L249)), is written when the user picks
"Allow for all future sessions": a TOML document whose `rule` array holds one table per grant,
written through a write copy `auto-saved.toml.<16 hex>.tmp`
([`core/src/policy/config.ts:684-710`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/policy/config.ts#L684-L710),
[`795-890`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/policy/config.ts#L795-L890)). An unparseable file is copied to
`auto-saved.toml.bak` and replaced by a fresh one
([`824`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/policy/config.ts#L824)), so the `.bak` holds the grants from before.
At the `v0.66.0` tag the dialogs offer that option only with
`security.enablePermanentToolApproval` on, off by default, and `security.disableAlwaysAllow`
off
([`ToolConfirmationMessage.tsx` at `v0.66.0`](https://github.com/google-gemini/gemini-cli/blob/970413207732f5d4d8c50540c7fc359c44fdaaf2/packages/cli/src/ui/components/messages/ToolConfirmationMessage.tsx#L158-L160),
[`338-343`](https://github.com/google-gemini/gemini-cli/blob/970413207732f5d4d8c50540c7fc359c44fdaaf2/packages/cli/src/ui/components/messages/ToolConfirmationMessage.tsx#L338-L343);
[`settingsSchema.ts` at `v0.66.0`](https://github.com/google-gemini/gemini-cli/blob/970413207732f5d4d8c50540c7fc359c44fdaaf2/packages/cli/src/config/settingsSchema.ts#L1864-L1872)).
On the Linux install of 0.66.0, with the setting unset, none of the shell, `WebFetch`, MCP and
edit dialogs offered it ([Permission rows](#permission-rows)), so no install has written
`auto-saved.toml` and the file rests on the reviewed commit's source.
The grant's `decision` is `allow` and its `priority` the integer 950, the fraction of the
always-allow priority scaled to 1000
([`config.ts:69`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/policy/config.ts#L69),
[`84-93`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/policy/config.ts#L84-L93);
[`core/src/policy/types.ts:375-382`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/policy/types.ts#L375-L382)).

The CLI loads every `*.toml` file in a policies directory, not only `auto-saved.toml`
([`core/src/policy/toml-loader.ts:165-175`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/policy/toml-loader.ts#L165-L175)),
so a policy file the user wrote there holds standing rules too. A `rule` table
([`toml-loader.ts:39-70`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/policy/toml-loader.ts#L39-L70);
[`types.ts:10-14`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/policy/types.ts#L10-L14),
[`48-53`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/policy/types.ts#L48-L53)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `toolName` | string or array | | |
| `decision` | string | `allow`, `deny`, `ask_user` | |
| `priority` | number | | |
| `subagent` | string | | |
| `mcpName` | string | | |
| `argsPattern` | string | | |
| `commandPrefix` | string or array | | |
| `commandRegex` | string | | |
| `modes` | array | | |
| `interactive` | boolean | | |
| `toolAnnotations` | object | | |
| `allowRedirection` | boolean | | |
| `allow_redirection` | boolean | | |
| `denyMessage` | string | | |
| `deny_message` | string | | |

`modes` lists approval modes (`default`, `autoEdit`, `yolo`, `plan`); `allow_redirection` and
`deny_message` are older spellings still read. A `safety_checker` array names the checkers that
vet a tool call ([`toml-loader.ts:75-106`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/policy/toml-loader.ts#L75-L106)).

`~/.gemini/policies/sandbox.toml` holds the sandbox's standing grants, written when the user
answers a sandbox expansion request (a command asking for more paths or network access) with
"allow and save"
([`sandboxPolicyManager.ts:18-38`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/policy/sandboxPolicyManager.ts#L18-L38),
[`126-138`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/policy/sandboxPolicyManager.ts#L126-L138),
[`215-241`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/policy/sandboxPolicyManager.ts#L215-L241);
[`core/src/tools/shell.ts:522-534`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/tools/shell.ts#L522-L534);
[Sandboxing in Gemini CLI: Sandbox expansion](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/docs/cli/sandbox.md#sandbox-expansion)): a
`modes` table with `plan`, `default` and `accepting_edits` tables (`network`, `readonly`,
`approvedTools`, `allowOverrides`, `yolo`) and a `commands` table keyed by command name:

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `allowed_paths` | array | | |
| `allow_network` | boolean | | |

The policy loader reads it as a policy file too; it holds no `rule` array.

`settings.json` (home and project), read as JSON with comments stripped
([`settings.ts:958`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/config/settings.ts#L958)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `general.defaultApprovalMode` | string | `default`, `auto_edit`, `plan` | |
| `tools.sandbox` | boolean or string | | |
| `tools.sandboxAllowedPaths` | array | | |
| `tools.sandboxNetworkAccess` | boolean | | |
| `tools.core` | array | | |
| `tools.allowed` | array | | |
| `tools.confirmationRequired` | array | | |
| `tools.exclude` | array | | |
| `mcp.allowed` | array | | |
| `mcp.excluded` | array | | |
| `security.toolSandboxing` | boolean | | |
| `security.disableYoloMode` | boolean | | |
| `security.disableAlwaysAllow` | boolean | | |
| `security.enablePermanentToolApproval` | boolean | | |
| `security.autoAddToPolicyByDefault` | boolean | | |
| `security.blockGitExtensions` | boolean | | |
| `security.allowedExtensions` | array | | |
| `security.folderTrust.enabled` | boolean | | |
| `security.enableConseca` | boolean | | |
| `general.sessionRetention.enabled` | boolean | | |
| `general.sessionRetention.maxAge` | string | | |
| `general.sessionRetention.maxCount` | number | | |
| `general.sessionRetention.minRetention` | string | | |
| `model.name` | string | | |

The approval mode YOLO is only a command-line choice
([`settingsSchema.ts:228-245`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/config/settingsSchema.ts#L228-L245)); the
runtime approval modes are those of `modes` above. `tools.sandbox` is a boolean, a profile path or
a sandbox command such as `docker`
([`settingsSchema.ts:1557-1590`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/config/settingsSchema.ts#L1557-L1590)); the
tool lists are at
[`1669-1719`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/config/settingsSchema.ts#L1669-L1719), the MCP lists at
[`1794-1813`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/config/settingsSchema.ts#L1794-L1813), the `security` keys at
[`1835-1925`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/config/settingsSchema.ts#L1835-L1925) and
[`2007-2016`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/config/settingsSchema.ts#L2007-L2016), retention at
[`389-440`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/config/settingsSchema.ts#L389-L440)
([What survives a delete](#what-survives-a-delete)) and `model.name` at
[`1062-1075`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/config/settingsSchema.ts#L1062-L1075).

`trustedFolders.json` is a JSON object read with comments stripped, keyed by folder path, each
value a `TrustLevel`
([`core/src/utils/trust.ts:18-22`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/utils/trust.ts#L18-L22),
[`95-96`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/utils/trust.ts#L95-L96),
[`106`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/utils/trust.ts#L106)): `TRUST_FOLDER`, `TRUST_PARENT` or
`DO_NOT_TRUST`. The CLI writes it with `JSON.stringify` through a write copy
`trustedFolders.json.tmp.<uuid>`
([`342-351`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/utils/trust.ts#L342-L351);
[Trusted Folders: Managing your trust settings](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/docs/cli/trusted-folders.md#managing-your-trust-settings)).

`trusted_hooks.json` maps a project path to the project hooks the user trusted, each
`<hook name>:<command>`
([`trustedHooks.ts:72`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/hooks/trustedHooks.ts#L72),
[`103-119`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/hooks/trustedHooks.ts#L103-L119); key form at
[`core/src/hooks/types.ts:121-125`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/hooks/types.ts#L121-L125)).
`acknowledgments/agents.json` maps a project path to an object of acknowledged project subagents,
each name to the agent's hash
([`core/src/agents/acknowledgedAgents.ts:13-18`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/agents/acknowledgedAgents.ts#L13-L18)).

`extension_integrity.json` holds a `store` object, each extension name to its `hash` and
`signature`, and a `signature` over the whole store, keyed with `integrity.key` or the keychain
secret
([`core/src/config/extensions/integrityTypes.ts`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/extensions/integrityTypes.ts);
[`integrity.ts:177-186`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/extensions/integrity.ts#L177-L186)).

### Format versions

No file carries a version field. The shapes changed in these ways:

- Hash-named project directories became slug directories; the migration copies them and both
  remain ([Per-user storage](#per-user-storage)).
- The single-object `session-*.json` became `.jsonl`; the full-history `$set.messages` record is
  deprecated
  ([`chatRecordingTypes.ts:150-153`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/services/chatRecordingTypes.ts#L150-L153)).
- `checkpoint-<tag>.json` moved from a bare array to `{history, authType}`
  ([`logger.ts:359-372`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/core/logger.ts#L359-L372)), and older names carry
  the tag unencoded
  ([`logger.ts:312-313`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/core/logger.ts#L312-L313)).
- The shadow repository's `.gitconfig` `[core] excludesFile` entry and its `.git/info/exclude`
  copy were added in commit
  [`0df3daa`](https://github.com/google-gemini/gemini-cli/commit/0df3daa9f90785e2eac7eaef4c997d5b44b7ac6e)
  (2026-10-30), after `v0.66.0`; the `v0.66.0` build writes `.gitconfig` with the author and
  `gpgsign` settings only
  ([`gitService.ts` at `v0.66.0`](https://github.com/google-gemini/gemini-cli/blob/970413207732f5d4d8c50540c7fc359c44fdaaf2/packages/core/src/services/gitService.ts#L138-L148)).
- Before commit
  [`5562023`](https://github.com/google-gemini/gemini-cli/commit/55620235c0c72c3e72c83b39efb02853a1bff494)
  (2026-04-15), which bundled ripgrep into the package, the CLI downloaded the ripgrep binary
  into `tmp/bin` on first use and ran it from there
  ([`ripGrep.ts` at `366f9e4`](https://github.com/google-gemini/gemini-cli/blob/366f9e4766d732235ed729f80e4f26556bc8f7d2/packages/core/src/tools/ripGrep.ts#L46-L92)).
- Between
  [`fb972b2`](https://github.com/google-gemini/gemini-cli/commit/fb972b2f87fe7d5b06d37eac711490162d98de2c)
  (0.64.0-nightly.20260929) and
  [`6814e00`](https://github.com/google-gemini/gemini-cli/commit/6814e00d6b148c3a1d57244d5d7040f7c541009d)
  nothing else changed in the paths or record shapes above: `chatRecordingService.ts` changed
  only resume and exit-cleanup logic before
  [`3be3d44`](https://github.com/google-gemini/gemini-cli/commit/3be3d4424b3c8a760d793ae1bea6cb5180adf27d),
  and `chatRecordingTypes.ts`, `storage.ts`, `projectRegistry.ts`, `logger.ts`, `trust.ts`,
  `sessionOperations.ts`, `sessionCleanup.ts` and the policy files are unchanged from `3be3d44`
  to `6814e00`.

### Secrets

The credential files are in [Credentials](#credentials). `oauth_creds.json` is a Google
`Credentials` object whose `access_token`, `refresh_token`, `scope`, `token_type` and
`expiry_date` the migration moves into the keychain
([`oauth-credential-storage.ts:64-90`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/code_assist/oauth-credential-storage.ts#L64-L90),
[`117-153`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/code_assist/oauth-credential-storage.ts#L117-L153)).
Inside the stores, tool checkpoints
(`clientHistory`), `toolCalls[].result`, the `/chat save` checkpoints, the `/bug` exports, the
error reports and the activity log's `network` payloads hold full tool output and request
bodies, which can include secrets the agent read. In the activity log the request `headers`
other than the three the CLI redacts, any `url` query string (an API key can ride there), and
the request and response `body` of a non-chat request, such as a token refresh's
`refresh_token` and the `access_token` it returns, can hold credentials; the parser prints none
of them ([Past-use rows and secondary sources](#past-use-rows-and-secondary-sources)).
`google_accounts.json` and Clearcut's account email are identities.

### Parser plan

Globs, home-relative, `<base>` the runtime directory (`.gemini`, or `.cache/.gemini` under the
macOS sandbox): `<base>/tmp/<slug>/chats/<path>.jsonl` and `<base>/tmp/<slug>/chats/<path>.json`,
`<path>` a session file name or `<parent session id>/<session id>`;
`<base>/tmp/<slug>/logs.json`, `<base>/tmp/<slug>/shell_history`,
`<base>/tmp/<slug>/checkpoint-<tag>.json`, `<base>/tmp/<slug>/bug-report-history-<ms>.json`,
`<base>/tmp/<slug>/logs/session-<session id>.jsonl` (the activity log's default path, which no
build has been seen to write;
[issue 410](https://github.com/seanthegeek/doubleagent/issues/410)),
`<base>/tmp/<slug>/checkpoints/<time>-<file basename>-<tool>.json` (`<time>` in the
`YYYY-MM-DDTHH-MM-SS_mmmZ` form), `<base>/tmp/<slug>/memory/<name>` (`<name>` any path below
`memory/`), `<base>/tmp/<slug>/images/clipboard-<epoch ms>.<ext>` (`<ext>` one of the
`IMAGE_EXTENSIONS` in lower case), `<base>/gemini-client-error-<type>-<time>.json` and
`<base>/tmp/<slug>/gemini-client-error-<type>-<time>.json` (`<time>` in the
`YYYY-MM-DDTHH-MM-SS-mmmZ` form), and
`<base>/trustedFolders.json`. In the home and in a project: `.gemini/settings.json` and
`.gemini/policies/*.toml` (also `auto-saved.toml.bak`). In the home only:
`.gemini/trusted_hooks.json` and `.gemini/acknowledgments/agents.json`.

Rows: one `system` row per metadata line,
`session start: kind=<kind> projectHash=<projectHash>`, with ` directories=<JSON array>` added
for a subagent session; one `user` row per `user` message, one `system` row
`info: <text>`, `error: <text>` or `warning: <text>` per message of those types; one `assistant`
row per `gemini` message plus one `thinking` row per `thoughts[]` entry, a `gemini` message with
neither text nor tool calls still giving an empty `assistant` row; one `tool_use` per
`toolCalls[]` element and one `tool_result` when `result` is non-null or `status` is `error` or
`cancelled`, sharing the `tool_use_id`, the result text prefixed `[error]` or `[cancelled]` then;
`logs.json` entries as `user` rows when no session file covers that `sessionId`; the secondary
sources and standing files as in the subsections below. A `$set` with a `summary` is a `system`
row `summary: <summary>` at its `lastUpdated`; a `$rewindTo` is
`rewind to <message id>: <n> message(s) dropped` and a `$patch` with `removeIds`
`patch removed <n> message(s)`, each at the latest time seen before it. In a text, an
`inlineData` part reads `[inlineData <mimeType>]` and a `fileData` part `[fileData <fileUri>]`,
and a `functionResponse` part its `response`'s `output` or `error`.

| Column | Source |
| --- | --- |
| timestamp_utc | `timestamp`; `toolCalls[].timestamp`, `thoughts[].timestamp` |
| session_id | line-1 `sessionId`, overridden by a later `$set.sessionId` |
| project_path | `.project_root` or `projects.json`; else `projectHash` |
| git_branch | empty |
| turn_type | `user` to user; `gemini` to assistant; `info`, `error`, `warning` to system; `toolCalls` to tool_use and tool_result; `thoughts` to thinking |
| model | the `gemini` message's `model` |
| tool_name, tool_use_id | `toolCalls[].name`, `toolCalls[].id` |
| text | `content` text; for tool_use `displayName` plus args; `summary` on `$set` |

Not read: `compression_state.json`, `policy_integrity.json`, `extension_integrity.json`,
`mcp-server-enablement.json`, `state.json`, `state.json.bak`, an error report's
`requestConfig` and `additionalInfo`, the activity log's `chunk` records, `body` and
`response.body`, and every `headers` object. A tool
checkpoint's display list is not read either: its turns are the `clientHistory` items in display
form.

#### Permission rows

- `permission decision: <status>` (`system`, the call's `tool_name` and `tool_use_id`) after the
  `tool_result` row of a `toolCalls[]` element whose `status` is `cancelled` or
  `awaiting_approval`, or after its `tool_use` row when it has no result. The `[cancelled]` (and
  `[error]`) prefix on the result row stays. The session file records the call's `status` only;
  which button the user pressed is not recorded.
- `permission setting: trustedFolders.<path>=<value> (from <file>, file mtime)` (`system`,
  `project_path` the folder, no session id, the file's modification time, line 1), one per key of
  `trustedFolders.json`, the value printed as recorded. The same reader serves Qwen Code's
  `~/.qwen/trustedFolders.json`, which Qwen reads as JSONC (JSON with comments, trailing commas
  allowed, a leading byte-order mark dropped;
  [Format versions in `qwen-code.md`](qwen-code.md#format-versions)), so the parser drops a
  byte-order mark, strips comments and drops a comma that only whitespace separates from a
  closing `}` or `]`, outside strings, before decoding; a Gemini CLI file reads as before. A file
  that still does not decode, or is not a JSON object, is one
  `parser: <file> is not a JSON object` row.

The dialogs behind these records on the Linux install of 0.66.0:

- The folder trust dialog, "Do you trust the files in this folder?", lists what the folder
  configures (for the home, "Setting overrides (2): security, general") and offers
  `1. Trust folder (<name>)`, `2. Trust parent folder (<parent>)` and `3. Don't trust`; the first
  option stored a 12-character value under the folder's key in `trustedFolders.json`, the length
  of `TRUST_FOLDER`.
- A shell call asks `Allow execution of [Shell]?`, a `WebFetch` call "Action Required ... Do you
  want to proceed?", and an MCP tool `Allow execution of MCP tool "<tool>" from server
  "<server>"?` ("press Ctrl+O to expand MCP tool details"), each with `1. Allow once`,
  `2. Allow for this session` and `3. No, suggest changes (esc)`. An edit asks "Apply this
  change?" with `1. Allow once`, `2. Allow for this session`, `3. Modify with external editor`
  and `4. No, suggest changes (esc)`; the cursor wraps to the first option after the fourth.
  Allow once applied the edit to `calc.py`, and the agent then ran `python3 calc.py 2 3` through
  a shell dialog.
- `git log --oneline` and `ls -la` ran through the shell tool with no dialog, and so did the file
  searches of the refactor below. Escape on a dialog gave "Request
  cancelled", the command did not run and no dialog remained. On the next request the agent
  retried the cancelled command, and "Allow for this session" let both run. No session-wide
  grant wrote a file: no `policies/` directory appeared. No dialog offered "Allow for all future
  sessions" ([Standing permission files](#standing-permission-files)).
- A question needing a web search was answered through the `WebFetch` tool, behind its
  confirmation dialog; how the call is recorded was not read beyond the shape of the
  [`toolCalls[]` table](#session-jsonl).
- On the underspecified prompt "refactor this" the agent asked nothing and started a multi-tool
  refactor at once, asking only at each tool's dialog; in the second run it ran
  `FindFiles '*test*'`, which found no files, and `FindFiles '**/*'` before its first dialog, for
  `python3 --version`. No sharper prompt was tried.
- The default mode's footer reads "Shift+Tab to accept edits"; the modes the key cycles through
  were not read.

The standing grants below are each a `system` row at the file's modification time, saying so,
line 1, no session id ([Standing permission files](#standing-permission-files)):

- `.gemini/policies/*.toml` and `auto-saved.toml.bak`, in the home and in a project:
  `permission setting: policy <decision> <toolName> <key>=<value> ... (from <file>, file mtime)`,
  one per `rule` table, its other keys in file order, values as recorded (an array as compact
  JSON), `project_path` the project for a workspace file. Read with Python's `tomllib`; a file
  `tomllib` cannot read (the corrupt one a `.bak` preserves) is one
  `parser: unreadable <file>: <reason>` row, and a `rule` key that is not an array one
  `parser: <file> rule is not an array` row. `safety_checker` tables are not read.
- `~/.gemini/policies/sandbox.toml`, which has no `rule` table: one row
  `permission setting: sandbox mode <mode> <key>=<value> ... (from <file>, file mtime)` per
  `modes` table and one row
  `permission setting: sandbox command <name> allowed_paths=<paths> allow_network=<value> (from <file>, file mtime)`
  per `commands` grant, the keys in file order. A project's `sandbox.toml` is an ordinary policy
  file.
- `settings.json`, in the home and in a project: one
  `permission setting: <key>=<value> (from <file>, file mtime)` row per entry of
  `tools.sandboxAllowedPaths`, `tools.core`, `tools.allowed`, `tools.confirmationRequired`,
  `tools.exclude`, `mcp.allowed`, `mcp.excluded` and `security.allowedExtensions`, and per set
  value of `general.defaultApprovalMode`, `tools.sandbox`, `tools.sandboxNetworkAccess` and the
  boolean `security` keys of the settings table. No other key is read, since `mcpServers` holds
  credentials. A file that is not a JSON object is one `parser: <file> is not a JSON object` row.
- `trusted_hooks.json`:
  `permission setting: trustedHooks=<name>:<command> (from .gemini/trusted_hooks.json, file mtime)`
  per entry, `project_path` the project key.
- `acknowledgments/agents.json`:
  `permission setting: acknowledgedAgents.<agent>=<hash> (from .gemini/acknowledgments/agents.json, file mtime)`
  per entry, `project_path` the project key.
- `policy_integrity.json` holds one hash per `<scope>:<identifier>` key (a policy scope and, for
  a project, its path), against which the CLI checks whether a policy directory changed
  ([`core/src/policy/integrity.ts:27-29`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/policy/integrity.ts#L27-L29)).

#### Model rows

- `model: <id>` (`system`, `model` column set) before the first `gemini` message of a session
  whose `model` is set, and `model: <old> -> <new>` before each later `gemini` message whose
  `model` differs, tracked per `sessionId` in replay order, with the message's timestamp and line.
- Per message: each `gemini` message's `model` is the resolved model after routing; `user` and
  `info` messages carry none
  ([`chatRecordingService.ts:1096-1101`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/services/chatRecordingService.ts#L1096-L1101),
  [`1116-1124`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/services/chatRecordingService.ts#L1116-L1124)). Per session:
  no session-level model key exists. Per switch: there is no switch record; the alias the user
  chose is `model.name` in `settings.json`, and `/model set <name> [--persist]` adds the `info`
  message `Model set to <name>`
  ([`cli/src/ui/commands/modelCommand.ts:18-47`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/ui/commands/modelCommand.ts#L18-L47)),
  recorded because `info` items are written to the session
  ([`cli/src/ui/hooks/useHistoryManager.ts:91-123`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/ui/hooks/useHistoryManager.ts#L91-L123)).
- On the Linux install of 0.66.0 `/model` opened "Select Model" with `1. Auto` ("Let Gemini CLI
  decide the best model for the task: gemini-3.1-pro-preview, gemini-3.8-flash") and
  `2. Manual` ("Manually select a model"), the toggle "Remember model for future sessions: false
  (Press Tab to toggle)" and the hint "To use a specific Gemini model on startup, use the
  --model flag"; the footer shows `/model Auto`. Manual opens a numbered list of models, among
  them `5. gemini-3.8-flash`, `6. gemma-4-31b-it` and `7. gemma-4-26b-a4b-it`. The `model` key
  of each `gemini` message is in [Session JSONL](#session-jsonl). No switch was recorded, so a
  switch's records are source-derived.

#### Past-use rows and secondary sources

`logs.json` is a secondary source: its entries become `user` rows when no session file covers
that `sessionId` ([Secondary sources](../analyzer/docs/output.md#secondary-sources)).

Three more secondary sources are read. Their records carry no time, so their rows have an empty
`timestamp_utc`, no session id and the project of their `tmp/<slug>` directory, and they match the
session file on text, tool rows on `tool_use_id` ([Other stores](#other-stores)):

- `shell_history`: one `user` row per command, `source_line` the command's first line. A command
  matches no session row (the session wraps it in a sentence) but pairs with its `logs.json` copy,
  so a timeline shows it once.
- `checkpoint-<tag>.json`: a first `system` row
  `chat checkpoint: tag <tag> authType=<authType> (from <file>, file mtime)` at the file's
  modification time (`-` when the file names no `authType`, the tag decoded from the file name),
  then per `history` item its text parts as one `user` or `assistant` row, a `thinking` row per
  thought part when thinking is asked for, and a `tool_use` or `tool_result` row per
  `functionCall` or `functionResponse` part with its `id` and `name`; `source_line` is the item's
  1-based position. The environment-context item becomes a `user` row of its own.
- `bug-report-history-<epoch ms>.json`: the same item rows after a first `system` row
  `bug report history: exported (from <file>, time in the file name)`, timestamped with the
  milliseconds in the name.

Two more secondary sources repeat the turns sent to the model, matched the same way; their
`Content` items give the item rows above with an empty `timestamp_utc`, the item's 1-based
position as `source_line`, and the session id where the file names one. A first item that is the
environment-context item, its first part's text opening with `<session_context>`
([Activity log](#activity-log)), gives no row, since the session file does not record it:

- `checkpoints/<time>-<file basename>-<tool>.json`, a tool checkpoint
  ([Tool checkpoints](#tool-checkpoints)): a first `system` row
  `tool checkpoint: <tool> <file_path> commit <commitHash> prompt <messageId> (from <file>, time in the file name)`
  at the time in the name, `-` for a value the file lacks, then the `clientHistory` items, all
  with the session id before the `########` of `messageId` and the project of the directory.
- `gemini-client-error-<type>-<time>.json`, a copied error report
  ([Error reports](#error-reports)): a first `system` row
  `error report: <type>: <message> (from <file>, time in the file name)` at the time in the name,
  the `stack` on the lines after it when present, then the items of `context`, or of its
  `requestContents`, with no session id and the project of a `tmp/<slug>` directory it sits in.

Three stores repeat nothing of the session file in the rows read from them and are primary, one
`system` row per record or file with the project of its `tmp/<slug>` directory:

- `logs/session-<session id>.jsonl`, the activity log at its default path, which no build has
  been seen to write; where a build writes it is open work
  ([issue 410](https://github.com/seanthegeek/doubleagent/issues/410);
  [Activity log](#activity-log)). Per line, at the record's `timestamp` and with its
  `sessionId`, a `console` record is `console <type>: <content>`; a `network` record holding a
  `method` is `network request: <method> <url> (request <id>)`, one holding a `response`
  `network response: <method> <url> status <status> in <durationMs> ms (request <id>)` and one
  holding an `error` `network error: <method> <url>: <error> (request <id>)`, the method and URL
  from the request record of the same `id` and `-` for a value the file lacks. `<url>` stops
  before its query string, and a `chunk` record gives no row. The conversation turns in the
  request `body` and the response `body` are not read: which requests form one chat, which items
  the CLI adds itself (the environment context, the compression summary and its acknowledgement,
  retry prompts), the event form of a stored stream and when a reply is repeated by the next
  request are left to the real-install round, which records the log before a parser reads turns
  from it ([issue 410](https://github.com/seanthegeek/doubleagent/issues/410)).
- `memory/<name>`: `memory file: <name> (from <file>, file mtime)` at the file's modification
  time, no session id, the file's whole text on the lines after it ([memory/](#memory)).
- `images/clipboard-<epoch ms>.<ext>`:
  `clipboard image: <file name> <size> bytes (from <file>, time in the file name)` at the
  milliseconds in the name, no session id, the size the manifest's `size` or, in a loose tree,
  the file's; the image itself is not read ([Clipboard images](#clipboard-images)).

#### Sync, relay and origin rows

The agent records none. No command uploads a session, and nothing under `~/.gemini` holds a
remote id, share URL or synced flag:

- `/chat share` (also `/resume share`) writes the conversation to a local `.md` or `.json` file,
  by default `gemini-conversation-<epoch ms>.json` in the working directory
  ([`chatCommand.ts:282-331`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/ui/commands/chatCommand.ts#L282-L331);
  [`historyExportUtils.ts:61-81`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/ui/utils/historyExportUtils.ts#L61-L81)),
  and `/export-session <path>` writes the session record as JSON to the path given
  ([`cli/src/ui/commands/exportSessionCommand.ts:18-76`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/ui/commands/exportSessionCommand.ts#L18-L76)).
  Neither sends anything over the network.
- `/bug` and `/bug-memory` write local files only ([Bug reports](#bug-reports)).
- Code Assist (Google's `cloudcode-pa.googleapis.com` backend, used when the user signs in with a
  Google account,
  [`core/src/code_assist/server.ts:73-74`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/code_assist/server.ts#L73-L74))
  receives the local `sessionId` as `session_id` in every model request
  ([`core/src/code_assist/converter.ts:163-177`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/code_assist/converter.ts#L163-L177)),
  so the transcript's `sessionId` is also the key of the server's request records; that is the
  model call, not sync. Its metrics calls, `recordConversationOffered` and
  `recordConversationInteraction`, carry a trace id, counts, latencies and an accept or reject
  status, no text
  ([`core/src/code_assist/types.ts:311-336`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/code_assist/types.ts#L311-L336);
  [`server.ts:376-412`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/code_assist/server.ts#L376-L412)). The free-tier
  "allow Google to use my data" choice in `/privacy` is a server-side setting,
  `freeTierDataCollectionOptin`, read and written through the API and not stored locally
  ([`cli/src/ui/hooks/usePrivacySettings.ts:153-193`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/ui/hooks/usePrivacySettings.ts#L153-L193)).
  The server's `traceId` becomes the in-memory `responseId`
  ([`converter.ts:145-150`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/code_assist/converter.ts#L145-L150)) and is not
  written to the session file: `responseId`, `traceId`, `remote`, `cloud` and `upload` occur in
  neither `chatRecordingService.ts` nor `chatRecordingTypes.ts`.
- Clearcut, Google's event-logging endpoint `play.googleapis.com/log`, receives usage statistics
  when `privacy.usageStatisticsEnabled` is on, the default
  ([`core/src/telemetry/clearcut-logger/clearcut-logger.ts:244`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/telemetry/clearcut-logger/clearcut-logger.ts#L244),
  [`340-342`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/telemetry/clearcut-logger/clearcut-logger.ts#L340-L342);
  [`settingsSchema.ts:970-976`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/config/settingsSchema.ts#L970-L976)). Each
  event carries the session id and the signed-in account's email, or the `installation_id` when
  there is none
  ([`clearcut-logger.ts:496-501`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/telemetry/clearcut-logger/clearcut-logger.ts#L496-L501),
  [`1157-1162`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/telemetry/clearcut-logger/clearcut-logger.ts#L1157-L1162)).
  The prompt event sends the prompt's length, not its text
  ([`743-751`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/telemetry/clearcut-logger/clearcut-logger.ts#L743-L751)).
  The exception is Conseca, the context-aware security checker that has a model write a tool
  policy from the prompt (`security.enableConseca`, off by default,
  [`settingsSchema.ts:2007-2014`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/config/settingsSchema.ts#L2007-L2014)):
  with it on, its Clearcut events carry the user prompt, the trusted content, the generated policy
  and the verdict rationale whenever `telemetry.logPrompts` is on, its default
  ([`core/src/telemetry/conseca-logger.ts:33-46`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/telemetry/conseca-logger.ts#L33-L46),
  [`85-98`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/telemetry/conseca-logger.ts#L85-L98);
  [`core/src/config/config.ts:1099-1107`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/config.ts#L1099-L1107),
  [`2922-2924`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/config.ts#L2922-L2924)). Prompt text then leaves the
  host with no local record that it did.
- OpenTelemetry export (`telemetry.enabled`, off by default; target `local` by default or `gcp`,
  the user's own Google Cloud project,
  [`config.ts:1099-1107`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/config/config.ts#L1099-L1107);
  [`core/src/telemetry/index.ts:7-12`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/telemetry/index.ts#L7-L12)) puts the
  prompt text in the `user_prompt` log record when `telemetry.logPrompts` is on
  ([`core/src/telemetry/types.ts:222-224`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/telemetry/types.ts#L222-L224)).
  It goes to a collector the user configures, not to a vendor account; a `telemetry.outfile` path
  is a local copy of those records wherever the user put it.
- Remote subagents over A2A (the Agent2Agent protocol, a remote agent the user configures by URL)
  keep the remote `contextId` and `taskId` in an in-memory map only
  ([`core/src/agents/remote-session-invocation.ts:43-57`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/agents/remote-session-invocation.ts#L43-L57),
  [`242-247`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/core/src/agents/remote-session-invocation.ts#L242-L247)). The
  `a2a-server` package, a server the user deploys and not part of the CLI, stores tasks in a
  Google Cloud Storage bucket when `GCS_BUCKET_NAME` is set
  ([`a2a-server/src/http/app.ts:250-256`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/a2a-server/src/http/app.ts#L250-L256)).

#### Scratch directory rows

A session whose project path (its `.project_root`) is `<project>/.gemini/worktrees/<name>` gets
one `system` row `scratch directory: <path> (gemini --worktree <name>)` after the file's
`session start:` rows, at the first one's time and line, `project_path` the worktree
([Worktree sessions](#worktree-sessions)).

#### Deletion rows

No row marks a deletion itself; it leaves no marker to read, and the delete on the Linux install
of 0.66.0 wrote none ([What survives a delete](#what-survives-a-delete)). Each of
`general.sessionRetention.enabled`, `maxAge`, `maxCount` and `minRetention` that a
`settings.json` sets is one `system` row
`session retention: general.sessionRetention.<key>=<value> (from <file>, file mtime)` at the
file's modification time. No row means the defaults: retention on, 30 days
([What survives a delete](#what-survives-a-delete)).

#### Bad-line rows

A session JSONL line that does not decode is a `parser: unparseable line (<reason>): <line>` row,
and the lines around it are kept. A `logs.json` that does not parse is one whole-file row
`parser: unparseable logs.json: <reason>`, and one that parses to something else
`parser: logs.json is not a JSON array`. A legacy JSON session file that does not parse is one
row `parser: unparseable session file: <reason>`, and one that is not an object with a string
`sessionId` `parser: not a ConversationRecord`. In `shell_history` a line that is not valid
UTF-8 is a row `parser: unparseable line (not valid UTF-8): <line>`, the line decoded with
replacement characters, and the commands around it are kept. A `/chat save` checkpoint or `/bug`
export cut mid-write keeps its complete leading items and adds
`parser: unparseable record <N> (<reason>): <text from the cut item to the end>` or
`parser: array cut off after record <N>`; an item that is not an object is
`parser: unparseable record <N> (not a JSON object): <item>`. A file that does not decode and
starts as neither form is one `parser: unparseable chat history (<reason>)` row, and one that
decodes to neither form one
`parser: not a chat history: neither a JSON array nor an object with history` row. A standing
file ([Permission rows](#permission-rows)) that reads but decodes to something else is one
`parser: <file> is not a JSON object` row, at the file's modification time.

In the activity log a line that does not decode is a `parser: unparseable line (<reason>): <line>`
row and one that decodes to something other than an object a
`parser: unparseable line (not a JSON object): <line>` row; an object without a `console` or
`network` `type` and an object `payload` is a
`parser: unparseable line (not an activity log record): <record>` row, the record as compact
JSON with sorted keys. The records around each are kept, and each row has the session id of the
file name. A tool
checkpoint that does not decode is one
`parser: unparseable tool checkpoint (<reason>)` row, one that is not an object
`parser: not a tool checkpoint: not a JSON object`; an error report that does not decode is one
`parser: unparseable error report (<reason>)` row, one without an `error` object
`parser: not an error report: no error object`. In either a `clientHistory` or `context` item that
is not an object is `parser: unparseable record <N> (not a JSON object): <item>`. A `memory/`
file that is not valid UTF-8 is one `parser: <name> is not UTF-8 text` row.

A session file (JSONL or legacy JSON), `logs.json`, `shell_history`, `/chat save` checkpoint,
`/bug` export, activity log, tool checkpoint, error report, `memory/` file, policy file or
standing file (`settings.json`, `trusted_hooks.json`,
`trustedFolders.json`, `acknowledgments/agents.json`) that cannot be read is one whole-file row
`parser: <file> could not be read (<reason>)`, `<file>` its name. A `tmp/<slug>/.project_root` or
`projects.json` that exists but cannot be read gives every file that resolves its project through
it one row `parser: <file> could not be read (<reason>)`, `<file>` the side file's name, with that
file's session id and the project path resolved without the side file; one that is absent or does
not parse gives no row.

#### Fixtures

Session JSONL from `ChatRecordingService` (the metadata line from the start-up writer, a `user`
message from `recordMessage`, a `gemini` message from `recordMessage` and written again by
`recordToolCalls`, then the three update records; `tokens` is `null` until the usage arrives):

```json
{"sessionId":"c4e1f0a2-5b6d-4e7f-8a9b-0c1d2e3f4a5b","projectHash":"3a7bd3e2360a3d29eea436fcfb7e44c735d117c42d1c1835420b6b9942dd4f1b","startTime":"2026-10-30T14:02:11.402Z","lastUpdated":"2026-10-30T14:02:11.402Z","kind":"main"}
{"id":"0b6f3c1e-2d4a-4f5b-9c6d-7e8f9a0b1c2d","timestamp":"2026-10-30T14:02:19.118Z","type":"user","content":[{"text":"which files are in src?"}]}
{"id":"5d2e9a7b-8c1f-4a3e-b6d0-1f2a3b4c5d6e","timestamp":"2026-10-30T14:02:22.740Z","type":"gemini","content":"Checking the directory.","thoughts":[{"subject":"Listing","description":"I will list the src directory.","timestamp":"2026-10-30T14:02:21.903Z"}],"tokens":null,"model":"gemini-2.5-flash"}
{"id":"5d2e9a7b-8c1f-4a3e-b6d0-1f2a3b4c5d6e","timestamp":"2026-10-30T14:02:22.740Z","type":"gemini","content":"Checking the directory.","thoughts":[{"subject":"Listing","description":"I will list the src directory.","timestamp":"2026-10-30T14:02:21.903Z"}],"tokens":{"input":2210,"output":31,"cached":0,"thoughts":44,"tool":0,"total":2285},"model":"gemini-2.5-flash","toolCalls":[{"id":"list_directory__list_directory_1793455342801_0","name":"list_directory","args":{"dir_path":"/srv/proj/src"},"result":[{"functionResponse":{"id":"list_directory__list_directory_1793455342801_0","name":"list_directory","response":{"output":"index.ts\nutil.ts"}}}],"status":"success","timestamp":"2026-10-30T14:02:23.015Z","resultDisplay":{"summary":"Found 2 item(s).","files":["index.ts","util.ts"]},"description":"src","displayName":"ReadFolder","renderOutputAsMarkdown":true}]}
{"$set":{"lastUpdated":"2026-10-30T14:02:23.020Z"}}
{"$patch":{"id":"5d2e9a7b-8c1f-4a3e-b6d0-1f2a3b4c5d6e","toolCalls":[{"id":"list_directory__list_directory_1793455342801_0","result":null}]}}
{"$rewindTo":"5d2e9a7b-8c1f-4a3e-b6d0-1f2a3b4c5d6e"}
```

`logs.json` from `Logger.logMessage`:

```json
[
  {
    "sessionId": "c4e1f0a2-5b6d-4e7f-8a9b-0c1d2e3f4a5b",
    "messageId": 0,
    "type": "user",
    "message": "which files are in src?",
    "timestamp": "2026-10-30T14:02:19.110Z"
  }
]
```

`checkpoint-before%20refactor.json` from `Logger.saveCheckpoint`, written with two-space
indentation (a `/bug` export is the bare `history` array in the same form):

```json
{
  "history": [
    {
      "role": "user",
      "parts": [
        {
          "text": "which files are in src?"
        }
      ]
    },
    {
      "role": "model",
      "parts": [
        {
          "text": "Checking the directory."
        },
        {
          "functionCall": {
            "id": "list_directory__list_directory_1793455342801_0",
            "name": "list_directory",
            "args": {
              "dir_path": "/srv/proj/src"
            }
          }
        }
      ]
    },
    {
      "role": "user",
      "parts": [
        {
          "functionResponse": {
            "id": "list_directory__list_directory_1793455342801_0",
            "name": "list_directory",
            "response": {
              "output": "index.ts\nutil.ts"
            }
          }
        }
      ]
    }
  ],
  "authType": "oauth-personal"
}
```

`shell_history` from `useShellHistory` (no final newline; `make\` continues on the next line):

```text
ls -la
git status
make\
test
```

`policies/auto-saved.toml` from the policy updater:

```toml
[[rule]]
decision = "allow"
priority = 950
toolName = "run_shell_command"
commandPrefix = [ "git", "status" ]
```

`policies/sandbox.toml` from `SandboxPolicyManager.saveConfig`:

```toml
[modes.plan]
network = false
readonly = true
approvedTools = [ ]
allowOverrides = true

[commands.npm]
allowed_paths = [ "/srv/proj/node_modules" ]
allow_network = true
```

`trusted_hooks.json` from `TrustedHooksManager` and `acknowledgments/agents.json` from
`AcknowledgedAgentsService`, both written with two-space indentation:

```json
{
  "/srv/proj": [
    "lint:npm run lint"
  ]
}
```

```json
{
  "/srv/proj": {
    "reviewer": "9f2c4e1a"
  }
}
```

#### Source host rows

The agent records no source-host integration. A grep of the reviewed commit for `github`,
`gitlab`, `bitbucket`, `oauth`, `installation`, `repositor`, `pull request` and `clone` found no
connected account, stored token or cached repository list: the only GitHub token is
`GITHUB_TOKEN`, read from the environment to download extensions and never written
([`cli/src/config/extensions/github_fetch.ts:15-18`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/config/extensions/github_fetch.ts#L15-L18)),
and `/setup-github` writes GitHub Actions workflow and command files into the repository's
`.github/workflows` and `.github/commands` and opens the repository's Actions secrets page in the
browser, keeping nothing under `~/.gemini`
([`cli/src/ui/commands/setupGithubCommand.ts:48-65`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/ui/commands/setupGithubCommand.ts#L48-L65),
[`240-252`](https://github.com/google-gemini/gemini-cli/blob/6814e00d6b148c3a1d57244d5d7040f7c541009d/packages/cli/src/ui/commands/setupGithubCommand.ts#L240-L252)). OAuth in the CLI is
the Google sign-in and the MCP and A2A server tokens ([Credentials](#credentials)).

## Confidence

- High: every path and record key above is read from constants, types and writers at the cited
  lines; on Linux, 0.66.0's session file, `logs.json`, `/bug` export, error reports, trust file
  and settings file match them.
- Medium: the `~/.cache/.gemini` contents, which exist only if the user runs the macOS sandbox,
  and the size of `history/*/.git`.
- Medium: whether a current build ever writes `functionCall` parts into `content` (the reader
  accepts them, the writer read here does not).
- For a later round: that a container-sandboxed session leaves no transcript under the host's
  `~/.gemini`; that `sandbox.toml` appears after an "allow and save" sandbox expansion; that a
  `--worktree` checkout stays after exit and what an edit in it leaves; the `logs.json` and
  `shell_history` copies of one shell-mode command; the keys of a `/chat save` checkpoint; a
  model switch and its `info` message; the permission modes Shift+Tab cycles through and their
  names; an "Allow for all future sessions" grant with `security.enablePermanentToolApproval`
  on; whether any prompt makes the agent ask the user before acting.
- For a later round: the keyring item's name on Linux; whether the keychain migration on load
  or the refused sign-in removed `oauth_creds.json`; why a resume over SSH once asked for the
  key again while the keyring file measured 105 bytes; the keychain item names on macOS and
  Windows; whether the `gemini-credentials.json` fallback is written over SSH with no desktop
  session; the macOS and Windows layouts, which no install has shown.
- For a later round: the activity log written through `GEMINI_CLI_ACTIVITY_LOG_TARGET`: whether
  the Code Assist and Gemini API request bodies reach it as strings, whether a header other than
  the three redacted ones carries a token, and the request and response bodies of a whole
  session (main and subagent chats, utility requests, a compression and a retry), from which the
  turns could be read; a tool checkpoint after an edit with `general.checkpointing.enabled` on.
- Not determined: whether any build writes the activity log to its default path, and which
  writer made the empty `logs/` directory of the `--worktree` session.
- Not determined: how common a `GEMINI_CLI_HOME` relocation is.
- Not determined: the `tmp/<slug>/<session id>/` layout beyond `plans`, `tracker` and `tasks`.
- Not determined: the `a2a-server` and `sdk` packages' own storage.
