# codex-cli

OpenAI's Codex CLI (catalog agent `codex-cli`), the Rust `codex-rs` workspace, writes one
append-only JSONL transcript per thread, which Codex calls a rollout, under `~/.codex`: each line an
envelope around a tagged item, plus a global prompt history. The same code runs behind the VS Code
extension and, as Codex Desktop, inside the ChatGPT desktop app (see
[`chatgpt-desktop.md`](chatgpt-desktop.md)); the standalone CLI and the desktop app share one
`~/.codex` and one login. Open Interpreter is a fork that keeps the format byte for byte under
`~/.openinterpreter` ([`open-interpreter.md`](open-interpreter.md), which records only what
differs), and Ollama's `ollama launch` writes files into `~/.codex`
([Files written by Ollama](#files-written-by-ollama)). The agent is parsed by
`doubleagent/parsers/codex.py` (`CodexParser`, built on the shared `RolloutParser`). The TUI is the
CLI's terminal user interface.

## Source and evidence

[openai/codex](https://github.com/openai/codex), Apache-2.0, open source, shallow clone at
`scratchpad/repos/codex`, HEAD
[`3e238776e857eccd3bde6bff3026e2e9798f6524`](https://github.com/openai/codex/commit/3e238776e857eccd3bde6bff3026e2e9798f6524)
(2026-10-03). Paths below are relative to `codex-rs/`. All claims are **source** (file:line under
`codex-rs/`) unless marked **local** (the names-only listing of `~/.codex` on the development host
under [Observed](#observed)), **observed** (the real installs under [Observed](#observed)) or
**web** (community write-ups, used only for the desktop app); every schema claim is from source
unless it is marked local or observed. A citation at the later commit
[`4e7b9e2`](https://github.com/openai/codex/commit/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857) marks a
fact found or changed after `3e23877` ([Source reviews](#source-reviews)).

### Observed

- macOS, Codex Desktop inside ChatGPT 26.1013.40934
  ([issue 49](https://github.com/seanthegeek/doubleagent/issues/49), the ChatGPT desktop parser,
  [issue 149](https://github.com/seanthegeek/doubleagent/issues/149), and
  [issue 160](https://github.com/seanthegeek/doubleagent/issues/160), the resume and sync question):
  ChatGPT `26.1013.40934` (bundle identifier `com.openai.codex`,
  the build after the 26.1007.71326 of the 2026-10-15 round) with the bundled Codex
  `0.163.0-alpha.10.2` (`originator` `Codex Desktop`), from 2026-10-18 18:15Z to 2026-10-19 00:43Z,
  in the macOS lab VM of the 2026-10-15 bullet below (macOS 26.6.2, build 25G83, arm64, one user).
  The user performed one step at a time and a listing of names, sizes and modification times was
  diffed after each, so each file is mapped to the step that wrote it; files were read for names,
  sizes, key names, value types, enum values and row counts only. Times are UTC; the listings of the
  first five steps were taken in the VM's local zone, Eastern Daylight Time (UTC-4), and converted.
  Every thread was started in the composer's Codex mode (the composer is the app's prompt box; its
  Codex mode starts a local Codex thread with a rollout, its ChatGPT mode lists the account's
  chatgpt.com conversations). The steps: install (first launch 18:17:26Z) and sign-in (18:31:28Z to
  18:32:34Z), which added `auth.json`, `models_cache.json` and the first shell snapshot to a
  `~/.codex` that already held the databases and the global state ([macOS, ChatGPT
  26.1013.40934](#macos-chatgpt-26101340934)); a plain question with no tools (18:40:09Z to
  18:40:31Z), whose `cwd` was the app's projectless output directory (the directory the app creates
  under `~/Documents/Codex/<date>/` for a thread that belongs to no project)
  `~/Documents/Codex/2026-10-18/w` ([Scratch directories](#scratch-directories)) and whose prompt
  went to the global state, not to `history.jsonl`; the project `~/Demo` added through the folder
  picker (18:48:52Z), which wrote a trusted-project table to `config.toml`, and a coding task in it
  (18:49:06Z to 18:50:51Z) that wrote three files there and nothing else in the project; in that
  thread, a tool call the user declined and one the user approved (19:03:50Z to 19:05:03Z), whose
  decisions are only in [`logs_2.sqlite`](#logs_2sqlite); a model switch (21:25:09Z), the **Approve
  for me** mode (21:25:28Z), in which an automatic reviewer the app calls the guardian answers
  approval requests instead of the user, and a prompt (21:26:40Z to 21:27:32Z) whose write the
  reviewer allowed in a thread of its own ([Guardian review threads](#guardian-review-threads)), the
  file going to `~/Documents` rather than the Desktop the prompt named; a quit (about 21:38Z),
  relaunch (21:45:36Z) and resume of the Demo thread (21:46:06Z) with one prompt (21:47:36Z to
  21:47:45Z), which appended to the same rollout ([Joins and timestamps](#joins-and-timestamps)); a turn in
  **Full access**, the access level that runs commands with no sandbox and no approvals (set
  21:52:30Z, turn 21:53:27Z, set back by hand at 21:53:38Z); a reply in ChatGPT mode to an existing
  ChatGPT conversation (about 22:02Z to 22:15Z), which wrote nothing under `~/.codex`
  ([Transcript stores in `chatgpt-desktop.md`](chatgpt-desktop.md#transcript-stores)); a scheduled
  task of the kind the app calls a heartbeat (each run ends with the model deciding whether to
  notify the user), created at 23:18:38Z, run with Run now (23:19:26Z to 23:19:30Z) and once on
  schedule (23:45:49Z to 23:45:53Z) ([Automations](#automations)); remote control, which pairs a
  phone with the desktop's app server (Codex's local service that owns the threads), enabled at
  23:50:17Z, with a prompt from an iPhone (23:59:14Z to 23:59:31Z), connections turned off
  (00:05:42Z) and on (00:09:25Z), the iPhone revoked (00:14:36Z), an Android phone paired
  (00:23:10Z) and a prompt from it (00:29:14Z to 00:29:16Z) ([`state_5.sqlite` remote control
  enrollments](#state_5sqlite-remote-control-enrollments)); and the plain question's thread deleted
  with the app's own command at 00:36:02Z ([What survives a delete](#what-survives-a-delete)). The
  rollouts are in [Observed rollouts](#observed-rollouts) and the record shapes they showed in
  [Record schema](#record-schema) and [Other stores](#other-stores). Exercised here for the first
  time on any platform: a resumed thread, a denied and an approved tool call, a model switch within
  a thread, an automatic approval, scheduled-task runs, remote control and a delete. Not exercised:
  an "always allow" grant (the dialog of build 26.1013.40934 offered none for a command), a subagent
  the agent spawned itself, a heartbeat `NOTIFY` decision, an `automation_runs` row and an archived
  thread.
- Ubuntu, standalone CLI ([issue 113](https://github.com/seanthegeek/doubleagent/issues/113) and
  [issue 118](https://github.com/seanthegeek/doubleagent/issues/118)): Ubuntu 26.04.1 LTS, Codex CLI
  0.162.1 standalone
  (`codex --version` prints `codex-cli 0.162.1`), 2026-10-15, 17:12Z to 17:22Z (UTC), in the
  author's Ubuntu lab VM (aarch64, kernel 7.0.0-38-generic, one user; no Node.js, npm, snap or deb
  package of Codex, and no ChatGPT app, which has no Linux build), read for file names, sizes, key
  names, value types and line counts only. The CLI was installed with its shell installer, logged
  in through the browser flow, which wrote `auth.json` and `log/codex-login.log`
  ([Credentials](#credentials)), and given two turns in `~/demo`, which wrote seven files there, one
  rollout ([Observed rollouts](#observed-rollouts)) and two `history.jsonl` lines. The tree holds
  none of the desktop app's files ([Standalone CLI on Linux](#standalone-cli-on-linux)).
- macOS, Codex Desktop inside ChatGPT 26.1007.71326 and standalone CLI
  ([issue 113](https://github.com/seanthegeek/doubleagent/issues/113),
  [issue 118](https://github.com/seanthegeek/doubleagent/issues/118),
  [issue 120](https://github.com/seanthegeek/doubleagent/issues/120) and
  [issue 121](https://github.com/seanthegeek/doubleagent/issues/121)): ChatGPT `26.1007.71326`
  (bundle id `com.openai.codex`; the app's build is in [Source and evidence in
  `chatgpt-desktop.md`](chatgpt-desktop.md#source-and-evidence)) with the bundled Codex
  `0.162.0-alpha.17.2` (`originator` `Codex Desktop`), then Codex CLI `0.162.1` (`originator`
  `codex-tui`) from its shell installer, 2026-10-15 (16:25Z to 17:05Z, UTC), in the author's macOS
  lab VM, macOS 26.6.2 build 25G83, arm64, one user. The user repeated the Windows round with no
  listing between tasks; the files were read for file names, sizes, key names, value types and row
  counts only. The round wrote four rollouts ([Observed rollouts](#observed-rollouts)): a project
  task in `~/Demo` (16:39Z to 16:40Z); the target thread of a scheduled automation created at
  16:44Z, in `~/Documents/Codex/2026-10-15/new-chat` ([Automations](#automations)); a thread in the
  projectless output directory `~/Documents/Codex/2026-10-15/e` (16:50Z, with a
  `codex_app.load_workspace_dependencies` tool call; which user action it was is not determined);
  and the CLI's one prompt in `~/demo2` (16:55Z to 16:56Z), which the CLI also wrote to
  `history.jsonl`. A plain chat and an image generation left no rollout ([Transcript stores in
  `chatgpt-desktop.md`](chatgpt-desktop.md#transcript-stores)). The `~/.codex` tree is the Windows
  one with the differences in [macOS differences](#macos-differences).
- Windows, standalone CLI ([issue 113](https://github.com/seanthegeek/doubleagent/issues/113) and
  [issue 120](https://github.com/seanthegeek/doubleagent/issues/120)): Codex CLI `0.162.1`
  (`originator` `codex-tui`), installed at 15:38Z on 2026-10-15 (run until 15:46Z) in the
  VM of the Codex Desktop bullet below, beside the ChatGPT desktop app, with the PowerShell
  installer OpenAI documents for Windows (the exact command was not recorded). No npm, Node.js,
  winget or scoop is on the host. The user gave it one prompt from a PowerShell console in a new
  folder, `C:\users\<user>\demo-cli` (`~\demo-cli`, typed in lower case). The CLI shared the
  desktop app's `~\.codex` and its login: `auth.json` was not rewritten, no login was asked for and
  no second credential file appeared. It wrote one rollout, whose `source` is `vscode` like the
  desktop app's ([Observed rollouts](#observed-rollouts),
  [Joins and timestamps](#joins-and-timestamps)), one `history.jsonl` line, which the desktop app
  had not written, and the files in [Standalone CLI on Windows](#standalone-cli-on-windows). The
  sandbox failed to start a process during the run (`.sandbox/setup_error.json` `code`
  `helper_unknown_error`; the daemon's stderr log records a failed process creation and a failed
  file write in the project folder), so some tool calls failed; what the folder ended up holding was
  not surveyed. The console's own command history is in PSReadLine's `ConsoleHost_history.txt`,
  which is shell history, the job of the EDR (the endpoint detection and response tool), and not
  collected.
- Windows, Codex Desktop inside ChatGPT 26.1007.9446.0
  ([issue 113](https://github.com/seanthegeek/doubleagent/issues/113),
  [issue 114](https://github.com/seanthegeek/doubleagent/issues/114),
  [issue 115](https://github.com/seanthegeek/doubleagent/issues/115),
  [issue 116](https://github.com/seanthegeek/doubleagent/issues/116) and
  [issue 118](https://github.com/seanthegeek/doubleagent/issues/118)): the
  ChatGPT desktop app `26.1007.9446.0` (the Microsoft Store package `OpenAI.Codex`, an MSIX
  package, the Windows app package format; see [`chatgpt-desktop.md`](chatgpt-desktop.md)) bundles
  Codex, which identifies itself in its rollouts as `originator` `Codex Desktop`, `cli_version`
  `0.162.0-alpha.17.2`; the desktop app's own database records `app_version` `26.1007.71326`.
  Observed 2026-10-15 in the author's Parallels lab VM, Windows 11 ARM64 (the 64-bit Arm
  architecture; `ver` reports `10.0.26300.9550`), one user, times in UTC, read for file names,
  sizes, key names, value types, enum values and row counts only; no listing was taken between
  tasks, so the mapping from task to file is by modification time only. The bundled Codex uses the
  standard `CODEX_HOME`, `~\.codex` (`~` is `%USERPROFILE%`), created at first launch
  ([Codex Desktop inside the ChatGPT app](#codex-desktop-inside-the-chatgpt-app)). The user ran two
  local threads, one in the app's default output directory for a thread with no project,
  `~\Documents\Codex\2026-10-15\d` (14:26:37Z to 14:27:19Z), and one in a project folder on the
  Desktop (14:30:05Z to 14:32:47Z), which wrote eleven files there; a ChatGPT chat with an image
  generation, which left no rollout
  ([Transcript stores in `chatgpt-desktop.md`](chatgpt-desktop.md#transcript-stores)); and a first
  attempt at a scheduled automation, which failed in the app and wrote nothing. Later the same day
  (survey at 15:30Z) the user subscribed the account to ChatGPT Plus (the paid plan), which only
  refreshed the token in `auth.json` and grew `models_cache.json` from 5 to 9 models; created a
  scheduled automation that saved, which wrote `automations/` and at once a third rollout, its
  target thread ([Automations](#automations)); and connected a GitHub account, which wrote nothing
  locally (the connection is held by the ChatGPT account on the server). The rollouts are in
  [Observed rollouts](#observed-rollouts). Not exercised: a resumed thread, a denied tool call, an
  "allow always" grant, a model switch within a thread, subagents, an automation run; the macOS
  round of 2026-10-18 exercised all of them but the grant and a subagent the agent spawned itself.
- Local: the real install on the development host, Codex 0.160.0 (`session_meta.cli_version`), one
  rollout of 20 lines, a 1-line `history.jsonl` and a 1-line `session_index.jsonl`, plus
  scratchpad copies of `state_5.sqlite` and `thread_history_1.sqlite`. Only record types, key
  paths, value types and enum values were tabulated; no content was read into this document. The
  one rollout is `history_mode: "paginated"` and has no `git` block, so the legacy event set and
  `git` are source only. The databases were opened only from scratchpad copies, and only for table
  names, row counts and key names.

### Source reviews

- 2026-10-21: [openai/codex](https://github.com/openai/codex) at
  [`4e7b9e2`](https://github.com/openai/codex/commit/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857),
  the commit of the 2026-10-19 review; source only. The parser now reads the standing grants, the
  `PatchApproval` log lines, the `guardian_review_feedback` records, the `thread_goal_updated`
  events, the memory rollout summaries and the `db-backups/` copies of `state_5.sqlite`, and
  project discovery reads `archived_sessions/` and skips the thread worktrees; the review corrected
  the `thread_goal_updated` key names to camelCase, located the network policy in
  `permission_profile` ([Format versions](#format-versions)) and added the
  [Source host rows](#source-host-rows).
- 2026-10-19: [openai/codex](https://github.com/openai/codex) at
  [`4e7b9e2`](https://github.com/openai/codex/commit/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857), the
  head of `main` that day, diffed file by file against
  [`3e23877`](https://github.com/openai/codex/commit/3e238776e857eccd3bde6bff3026e2e9798f6524);
  source only. It added the standing grants and per-call approval records
  ([Standing grants and approval records](#standing-grants-and-approval-records)), the memories,
  goals and backup stores ([Other past-use stores](#other-past-use-stores)), the thread worktrees
  as scratch directories ([Scratch directories](#scratch-directories)) and the reach of
  `thread/delete` ([What survives a delete](#what-survives-a-delete)), and recorded the format
  drift in [Format versions](#format-versions), which the parser handles.

## Collection

### Per-user storage

Home resolution: `CODEX_HOME` if set (must exist, canonicalised) else `dirs::home_dir()/.codex`
([utils/home-dir/src/lib.rs:13-56](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/utils/home-dir/src/lib.rs#L13-L56)).
No OS branching: Linux, macOS and Windows all use `~/.codex` / `%USERPROFILE%\.codex`. The macOS
desktop app uses the same `~/.codex` for sessions (web:
[openai/codex issue 49797](https://github.com/openai/codex/issues/49797), jazzyalex write-up) and
keeps only Sparkle update state in `~/Library/Application Support/com.openai.codex`
([cli/src/doctor/updates.rs:586](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/cli/src/doctor/updates.rs#L586)).
Since 26.1007 the bundle id `com.openai.codex` is the ChatGPT app, which bundles Codex (observed on
macOS, 2026-10-15), so that directory is collected as `chatgpt-desktop`
([issue 121](https://github.com/seanthegeek/doubleagent/issues/121)).

Inside `$CODEX_HOME`:

- `sessions/YYYY/MM/DD/rollout-<YYYY-MM-DDThh-mm-ss>-<thread-uuid>.jsonl` transcripts
  ([rollout/src/lib.rs:86](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/rollout/src/lib.rs#L86),
  [rollout_file_name.rs:51-67](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/rollout/src/rollout_file_name.rs#L51-L67),
  [recorder.rs:1733-1734](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/rollout/src/recorder.rs#L1733-L1734),
  [list.rs:905-916](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/rollout/src/list.rs#L905-L916));
  may be zstd-compressed `.jsonl.zst`
  ([rollout/src/compression.rs:29](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/rollout/src/compression.rs#L29),[68](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/rollout/src/compression.rs#L68),[98](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/rollout/src/compression.rs#L98)).
  `archived_sessions/` same layout
  ([rollout/src/lib.rs:87](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/rollout/src/lib.rs#L87)).
  `session_index.jsonl`
  ([rollout/src/session_index.rs:20](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/rollout/src/session_index.rs#L20)).
  Local confirms `sessions/2026/10/03/rollout-*.jsonl`.
- `history.jsonl` prompt history
  ([message-history/src/lib.rs:52](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/message-history/src/lib.rs#L52),[86](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/message-history/src/lib.rs#L86)).
- SQLite: `state_5.sqlite` (threads, metadata), `logs_2.sqlite`, `goals_1.sqlite`,
  `memories_1.sqlite`, `queue_1.sqlite`, `thread_history_1.sqlite`
  ([state/src/sqlite.rs:34-39](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/state/src/sqlite.rs#L34-L39)),
  `agent_message_board_1.sqlite`
  ([ext/agent-message-board/src/local.rs:55](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/ext/agent-message-board/src/local.rs#L55);
  at `4e7b9e2` in `ext/agent-message-board/src/local/pool.rs`,
  [16](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/ext/agent-message-board/src/local/pool.rs#L16),
  same file name), with `-wal`/`-shm` sidecars (local). Recovery copies in `db-backups/sqlite-<n>/`
  ([state/src/runtime/recovery.rs:16](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/state/src/runtime/recovery.rs#L16)).
- `memories/` (`MEMORY.md`, `rollout_summaries/*.md`), `memories_v2/`, `memories_extensions/`
  ([memories/read/src/lib.rs:14](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/memories/read/src/lib.rs#L14),
  [memories/write/src/control.rs:5-7](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/memories/write/src/control.rs#L5-L7),[56-63](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/memories/write/src/control.rs#L56-L63)).
  At `4e7b9e2` the memories pipeline, off by default, also writes `memories/raw_memories.md` and
  `memories/memory_summary.md` and the databases `memories_1.sqlite` and `memories_v2_1.sqlite`
  ([state/src/sqlite.rs:43](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/state/src/sqlite.rs#L43));
  what they hold is in [Other past-use stores](#other-past-use-stores). Not seen on a real install.
  Collected.
- `log/` (`codex-tui.log`, `codex-login.log`)
  ([core/src/config/mod.rs:4093](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/core/src/config/mod.rs#L4093),
  [tui/src/lib.rs:281](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/tui/src/lib.rs#L281),
  [cli/src/login.rs:85](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/cli/src/login.rs#L85)).
  Daemon logs in `app-server-daemon/`
  ([feedback/src/daemon_logs.rs:18](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/feedback/src/daemon_logs.rs#L18));
  `app-server-control/` socket dir
  ([app-server-transport/src/transport/mod.rs:53](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/app-server-transport/src/transport/mod.rs#L53)).
- `shell_snapshots/`
  ([core/src/shell_snapshot.rs:107](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/core/src/shell_snapshot.rs#L107)),
  `skills/` with bundled `skills/.system/`
  ([skills/src/lib.rs:57](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/skills/src/lib.rs#L57)),
  `plugins/` and `plugins/cache/`
  ([core-plugin-common/src/installed.rs:11](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/core-plugin-common/src/installed.rs#L11)),
  `packages/` managed binaries
  ([app-server-daemon/src/managed_install.rs:52](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/app-server-daemon/src/managed_install.rs#L52)),
  `worktrees/`
  ([worktree/src/settings.rs:44](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/worktree/src/settings.rs#L44)),
  `visualizations/`
  ([tui/src/inline_visualization.rs:93](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/tui/src/inline_visualization.rs#L93)),
  `.tmp/`
  ([rollout/src/maintenance.rs:27](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/rollout/src/maintenance.rs#L27),
  [core-plugins/src/startup_sync.rs:171](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/core-plugins/src/startup_sync.rs#L171);
  `.tmp/bundled-marketplaces/`), `tmp/arg0/`
  ([arg0/src/lib.rs:363](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/arg0/src/lib.rs#L363)),
  `cache/`
  ([tui/src/pets/ambient.rs:158](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/tui/src/pets/ambient.rs#L158)),
  `installation_id`
  ([core/src/installation_id.rs:17](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/core/src/installation_id.rs#L17)),
  `version.json`
  ([cli/src/doctor/updates.rs:35](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/cli/src/doctor/updates.rs#L35)),
  `.sandbox_migration`
  ([execpolicy/src/sandbox_migration.rs:10](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/execpolicy/src/sandbox_migration.rs#L10)),
  `config.toml`
  ([config/src/lib.rs:52](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/config/src/lib.rs#L52)),
  `rules/` exec policy
  ([core/src/exec_policy.rs:54](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/core/src/exec_policy.rs#L54)),
  `hooks.json`, `agents/`.
- Windows sandbox: `.sandbox/` (logs `.sandbox/logs/sandbox.<date>.log`, `setup_error.json`),
  `.sandbox-bin/`, `.sandbox-secrets/`
  ([windows-sandbox-rs/src/spawn_prep.rs:108](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/windows-sandbox-rs/src/spawn_prep.rs#L108),
  [logging.rs:240-251](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/windows-sandbox-rs/src/logging.rs#L240-L251),
  [setup.rs:196](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/windows-sandbox-rs/src/setup.rs#L196),
  [uninstall_windows.rs:132-137](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/windows-sandbox-rs/src/uninstall_windows.rs#L132-L137)).
- Local only (not grepped in source): `models_cache.json`, `thread-writer-locks/`,
  `tui-thread-reference-capabilities/`, `.sqlite-maintenance.lock`; the first two are described
  under [Codex Desktop inside the ChatGPT app](#codex-desktop-inside-the-chatgpt-app).

#### Scratch directories

- `Documents/Codex/<date>/<slug>/{work,outputs}` (`~\Documents\Codex\<date>\<slug>\` on Windows):
  the ChatGPT desktop app's projectless output directory, the working directory the bundled Codex is
  given for a thread with no project, which the thread's rollout `cwd` names; `outputs/` holds the
  files the agent wrote for the user. Collected whole by the `codex-cli|Documents/Codex` catalog
  line ([issue 149](https://github.com/seanthegeek/doubleagent/issues/149)); project discovery drops
  a rollout `cwd` in it
  ([Where the project path is recorded](#where-the-project-path-is-recorded)). Observed on Windows
  (ChatGPT 26.1007.9446.0, 2026-10-15;
  [Per-user storage in `chatgpt-desktop.md`](chatgpt-desktop.md#per-user-storage)) and on macOS
  (2026-10-15 and 2026-10-18; [Observed](#observed)); not seen on the Ubuntu install, which ran the
  standalone CLI only. The Windows install has not yet had a per-step round, so which action creates
  the directory is known only from the macOS listings.
- `<CODEX_HOME>/worktrees/<first 4 hex of a uuid>/<repository name>` (`.codex/worktrees/...` by
  default): the git worktrees Codex creates for a thread, unless `config.toml` `[desktop]
  git-worktree-root` names another absolute path (source at `4e7b9e2`, not seen on a real install)
  ([worktree/src/settings.rs:12-13](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/worktree/src/settings.rs#L12-L13),
  [43-56](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/worktree/src/settings.rs#L43-L56),
  [worktree/src/paths.rs:13-30](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/worktree/src/paths.rs#L13-L30));
  `[desktop] worktree-auto-cleanup-enabled`, on by default, lets Codex remove them
  ([settings.rs:62-70](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/worktree/src/settings.rs#L62-L70)).
  The default location is under the `.codex/worktrees` exclusion, so its files are not collected.
  Project discovery drops a rollout `cwd` under `<home>/.codex/worktrees` as it drops one under
  `Documents/Codex` (`find_cli_session_projects`, collector 1.18.0,
  [issue 222](https://github.com/seanthegeek/doubleagent/issues/222)); a thread that ran under a
  configured root outside the home still has that worktree discovered as a project and its project
  files collected. The analyzer classes a worktree under `.codex` as a scratch directory and one
  under a configured root as a project. The thread that owns a worktree is in `codex-thread.json`
  (`version`, `ownerThreadId`) inside the source repository's per-worktree git directory
  ([worktree/src/metadata.rs:19-27](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/worktree/src/metadata.rs#L19-L27),
  [88-91](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/worktree/src/metadata.rs#L88-L91)),
  which no catalog line names.

#### Files written by Ollama

Written by Ollama, not by Codex (Ollama source at `v0.35.1`, commit
[`b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce`](https://github.com/ollama/ollama/commit/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce);
detail in [ollama.md](ollama.md), [Project-local files](ollama.md#project-local-files)). Ollama
always uses `<home>/.codex` and ignores `CODEX_HOME`
([`cmd/launch/codex.go:234-240`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/codex.go#L234-L240)).

- `ollama launch chatgpt` (the ChatGPT desktop app's Codex, macOS and Windows) writes
  `ollama-launch-codex-routing.json` and `ollama-launch-models.json`
  ([`internal/proxy/codex_desktop.go:32-38`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/internal/proxy/codex_desktop.go#L32-L38),
  [`cmd/launch/codex_app.go:952-958`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/codex_app.go#L952-L958)),
  rewrites `config.toml` (root `model`, `model_catalog_json`, `openai_base_url` pointing at
  `<ollama host>/api/codex/v1`; `profile` and `model_provider` removed; `[desktop]` reasoning
  efforts extended,
  [`codex_app.go:312-354`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/codex_app.go#L312-L354)),
  and creates `auth.json` when none exists ([Credentials](#credentials)). The previous `config.toml`
  and catalogs are copied to `~/.ollama/backup/codex-app/`, which the `ollama` catalog line
  collects.
- `ollama launch codex` (the CLI) writes `model.json` and `ollama-launch.config.toml` and leaves
  `config.toml` alone
  ([`codex.go:209-232`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/codex.go#L209-L232),[`250-268`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/codex.go#L250-L268)).

All of these sit under `.codex` and are collected by the `codex-cli|.codex` line; none is excluded.

#### Codex Desktop inside the ChatGPT app

Observed on the Windows install of [Source and evidence](#source-and-evidence) (ChatGPT
26.1007.9446.0, 2026-10-15), not in the source at
[3e23877](https://github.com/openai/codex/commit/3e238776e857eccd3bde6bff3026e2e9798f6524) unless a
citation says otherwise. Names, types and counts only; no value from the install is recorded.
`~\.codex` held 9,155 files, most of them plugin and skill copies under `.tmp\`, `plugins\cache\`
and `skills\.system\`. The desktop app adds:

- `.desktop-created` (0 B, written at first launch) and `.codex-provisioning-<16 hex>.guard` (0 B):
  markers that the desktop app created and provisioned this home.
- `installation_id` (36 B), `.sandbox_migration` (3 B), `.sqlite-maintenance.lock` (0 B, byte-range
  locked while the app runs).
- `.codex-global-state.json` and `.codex-global-state.json.bak` (61,390 B each, 123,250 B after a
  Plus subscription and an automation): the desktop app's interface and account state, with the
  user's prompts, each thread's standing permissions and ChatGPT conversation resume tokens
  ([`.codex-global-state.json`](#codex-global-statejson)). Flagged secret
  ([Credentials](#credentials)).
- `sqlite/codex-dev.db` with a WAL sidecar (write-ahead log, where SQLite holds recent writes): the
  desktop app's own thread catalog and automation store
  ([`sqlite/codex-dev.db`](#sqlitecodex-devdb)). Collected.
- `state_5.sqlite`, `thread_history_1.sqlite` and `logs_2.sqlite`, the source-described stores, with
  the tables and columns this build adds ([Other stores](#other-stores)).
- `plugins/.plugin-appserver/` (`codex.exe`, 282,678,064 B, and `codex-code-mode-host.exe`,
  `codex-command-runner.exe`, `codex-windows-sandbox-setup.exe`, 7 to 70 MB): copies of the
  package's binaries, excluded ([issue 116](https://github.com/seanthegeek/doubleagent/issues/116)).
  `plugins/.remote-plugin-install-staging/` (empty) is excluded with it.
- `.tmp/bundled-marketplaces/openai-bundled/` (`.materialization-key`, `plugins/`, 274 MB) and
  `.tmp/plugins/` (60 plugin directories, 57 MB; the whole `.tmp/` was gone by the second survey, so
  it is a temporary tree), the marketplace sync the source cites
  ([core-plugins/src/startup_sync.rs:171](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/core-plugins/src/startup_sync.rs#L171));
  `cache/remote_plugin_catalog/<16 hex>.json` (36 MB), `cache/codex_apps_tools/` and
  `cache/codex_apps_server_info/`; `plugins/cache/` with `openai-bundled`, `openai-curated-remote`
  and `openai-primary-runtime` (149 MB). All under existing exclusions.
- `automations/` (after a scheduled automation was created in the app): `.run-jitter-salt` (37 B)
  and one directory per automation, named by a slug made from the name the user typed (user
  content), holding `automation.toml` (383 B) ([Automations](#automations)). Collected.
- `ambient-suggestions/<40 hex>/ambient-suggestions.json` (139 B): the app's suggested tasks for a
  project root ([Ambient suggestions](#ambient-suggestions)). Collected.
- `computer-use/config.json` (152 B: `accentColor`, `direction`, `locale`, `strings`),
  `vendor_imports/skills-curated-cache.json` (1,267 B: `fetchedAt`, `skills`), `models_cache.json`
  (177,581 B: `client_version`, `etag`, `fetched_at`, `identity`, `models` with each model's
  instructions template and confirmation policies for browser and computer use): collected.
- `thread-writer-locks/` (`.coordination.lock` and one `<thread uuid>.lock` per thread, 0 B,
  byte-range locked while the app runs; [Thread locks and shell
  snapshots](#thread-locks-and-shell-snapshots)), `project-metadata-locks/` (empty),
  `visualizations/<yyyy>/<mm>/<dd>/<thread uuid>/` (empty, excluded), `code-review-plugin/` and
  `pets/` (empty).
- `.sandbox/` (`deny_read_acl_state.json`, `sandbox.<date>.log`, `sandbox-<16 hex>.log`,
  `setup_marker.json`, excluded), `.sandbox-bin/` (empty, excluded) and
  `.sandbox-secrets/sandbox_users.json` ([Credentials](#credentials)), and
  `tmp/arg0/codex-arg0<6 chars>/` (`apply_patch.bat`, `applypatch.bat`, excluded).
- `config.toml` as the app writes it: `notify`, a `[desktop]` table (`followUpQueueMode`,
  `conversationDetailMode`, `ambient-suggestions-enabled`), `marketplaces.openai-bundled` and
  `marketplaces.openai-primary-runtime` (`source_type` `local`),
  `plugins.<id>@openai-bundled.enabled` and `plugins.<id>@openai-primary-runtime.enabled` entries,
  an `mcp_servers.node_repl` server with its `env`, and `windows.sandbox` `elevated`.

The Windows sandbox, which the source describes
([setup.rs:196](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/windows-sandbox-rs/src/setup.rs#L196)),
was set up during onboarding: it created two enabled local accounts, `CodexSandboxOffline` and
`CodexSandboxOnline`, whose passwords are in `.sandbox-secrets/sandbox_users.json`, and the package
runs a service `CodexSandboxService.OpenAI.Codex`. The accounts and the service are the EDR's view
and are not collected.

Outside `~/.codex` the app keeps copies of the binaries in `%LOCALAPPDATA%\OpenAI\Codex` and a
1.27 GB runtime in `~/.cache/codex-runtimes`, neither collected; see [Exclusions in
`chatgpt-desktop.md`](chatgpt-desktop.md#exclusions).

#### Standalone CLI on Windows

Observed with Codex CLI 0.162.1 from the PowerShell installer
([Source and evidence](#source-and-evidence)); names, types and sizes only.

- `~\.codex\packages\standalone\`: `install.lock` (0 B), `auto-update-version` (31 B, one token),
  `current` (an NTFS junction, a directory reparse point, to
  `releases\0.162.1-aarch64-pc-windows-msvc`) and that release: `bin\codex.exe` (282,434,864 B),
  `bin\codex-code-mode-host.exe` (69,820,208 B), `codex-package.json` (216 B: `entrypoint`,
  `layoutVersion`, `pathDir`, `resourcesDir`, `target`, `variant`, `version`), `codex-path\rg.exe`,
  and `codex-resources\` with `codex-command-runner.exe`, `codex-windows-sandbox-setup.exe` and
  `voice\` (`bin\codex-voice-host.exe` with GStreamer, GLib, Opus and PCRE2 libraries,
  `manifest.json`, `runtime.json`, `sources.json`, `windows-crt.json`, licences). File times inside
  the release are the build's (2026-10-09). The `packages/` directory the source names
  ([app-server-daemon/src/managed_install.rs:52](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/app-server-daemon/src/managed_install.rs#L52)).
- `~\.codex\packages\app-server-daemon\`: the same layout with the same binaries, fetched by the
  daemon's updater.
- `%LOCALAPPDATA%\Programs\OpenAI\Codex\bin`: a junction to
  `~\.codex\packages\standalone\current\bin`, which the installer adds to the user `Path` value in
  `HKCU\Environment` (the current user's environment variables in the registry). No Start Menu entry
  and no Uninstall registry key.
- `~\.codex\app-server-daemon\`: the CLI runs the app server as a daemon that outlives the TUI.
  `daemon.lock`, `daemon.pid.lock`, `daemon-updater.pid.lock` (0 B), `daemon.pid` (328 B, JSON:
  `executableIdentity.{digest, path_digest}` as arrays of 32 numbers, `pid`, `processStartTime`),
  `daemon-updater.pid` (52 B: `pid`, `processStartTime`), `daemon.stderr.log` (Rust `tracing` lines
  with ANSI colour codes, `<ISO 8601 time> ERROR codex_core::tools::router: error=...`),
  `daemon-updater.stderr.log` (JSON lines with `timestampMs`, `processPid`, `stage`
  (`installer_fetch`, `installer_run`), `outcome`, `elapsedMs`, `failure`, or `event`
  `package_selected` with `details.{release, trigger}`), `daemon-updater.sock` and an empty
  `workdir\`. `~\.codex\app-server-control\` holds `app-server-control.sock` and
  `app-server-startup.lock` (0 B). The two `.sock` files are Unix domain sockets.
- `cap_sid` (419 B, JSON): `readonly`, `workspace`, `workspace_by_cwd` (a map from a `c:\...` path
  to a string) and `writable_root_by_path` (two `c:\...` paths to strings): capability SIDs by
  workspace path (SIDs are Windows security identifiers; a capability SID has the form
  `S-1-15-3-...`), inferred from the key names and value lengths, not from a value. Collected, not
  flagged.
- `version.json` (105 B: `dismissed_version`, `last_checked_at`, `latest_version`), the CLI's update
  check, cited in the source list above; `history.jsonl` (one line: `session_id`, `text`, `ts`), the
  CLI's prompt history, which the desktop app had not written;
  `tui-thread-reference-capabilities\<thread uuid>` (0 B, one per TUI thread);
  `node_repl\active_execs\` (empty), the scratch of the `node_repl` MCP server that `config.toml`
  configures (MCP is the Model Context Protocol, through which an agent calls external tools).
- `.sandbox\` gained `setup_error.json` (77 B: `code`, `message`) and a longer `sandbox.<date>.log`;
  `.sandbox-bin\` gained the sandbox's own copies of `codex.exe` and
  `codex-command-runner-0.162.0-alpha.17.2.exe` (289,784,416 B together).
- `config.toml` was rewritten at install (43 B longer; its keys were not re-read). `auth.json`,
  `.codex-global-state.json` and the desktop app's automation were not changed by the CLI.

#### macOS differences

Observed on the macOS install of [Source and evidence](#source-and-evidence); names, types and sizes
only. Everything else in
[Codex Desktop inside the ChatGPT app](#codex-desktop-inside-the-chatgpt-app) and
[Standalone CLI on Windows](#standalone-cli-on-windows) was present in the same form (`auth.json`,
`config.toml`, `.codex-global-state.json`, the SQLite stores, `sqlite/codex-dev.db`, `automations/`,
`ambient-suggestions/`, `vendor_imports`, `history.jsonl`, `version.json`, `app-server-daemon/`,
`app-server-control/`).

- `plugins/.plugin-appserver/codex-cli/` (337,674,240 B): on macOS the copied binaries sit in a
  `codex-cli` subdirectory; the `.codex/plugins/.plugin-appserver` exclusion covers it.
- `computer-use/Codex Computer Use.app` (bundle id `com.openai.sky.CUAService`, version
  26.1005.1001425, 168 files, 76,615,353 B), the computer-use agent's app, beside
  `computer-use/config.json`, excluded ([Exclusions](#exclusions)).
- `ipc/ipc.sock` (a Unix domain socket) and an empty `shell_snapshots/` (2026-10-15; on 2026-10-18
  it held the two most recent snapshots of each live thread, see
  [macOS, ChatGPT 26.1013.40934](#macos-chatgpt-26101340934)), macOS only.
- No `.sandbox`, `.sandbox-bin` or `.sandbox-secrets`: the Windows sandbox accounts have no macOS
  counterpart. The macOS sandbox is Seatbelt (Apple's sandbox profile mechanism), which left no file
  that was observed. `.sandbox_migration` is present.
- `app-server-daemon/daemon.pid` (472 B) and `daemon-updater.pid` (194 B) carry a `processIdentity`
  object (`bootId`, a uuid, `startSeconds`, `startMicroseconds`, `uniqueId`) that the Windows files
  lack.
- `app-server-control/app-server-control.sock` is a real symlink here, to
  `/private/tmp/codex-daemon-<uid>/<64 hex>`; `app-server-daemon/daemon-updater.sock` and
  `ipc/ipc.sock` are sockets.
- The standalone CLI: `packages/standalone/releases/0.162.1-aarch64-apple-darwin/` and
  `packages/app-server-daemon/releases/0.162.1-aarch64-apple-darwin/` (`bin/`, `codex-path/`,
  `codex-resources/`, `codex`, `codex-package.json`), each with a `current` symlink, 676,267,646 B
  in all, under the `.codex/packages` exclusion. The installer adds the symlinks
  `~/.local/bin/codex` and `~/.local/bin/codex-code-mode-host` into
  `packages/standalone/current/bin`, which no catalog line names, and a block in `~/.zprofile`
  between `# >>> Codex installer >>>` and `# <<< Codex installer <<<` that prepends `~/.local/bin`
  to `PATH`; shell startup files are not collected. The daemon processes ran from the
  `app-server-daemon` release.

#### macOS, ChatGPT 26.1013.40934

Observed on the macOS install of 2026-10-18 ([Source and evidence](#source-and-evidence)), bundled
Codex `0.163.0-alpha.10.2`; names, types and sizes only. Everything in
[Codex Desktop inside the ChatGPT app](#codex-desktop-inside-the-chatgpt-app) and
[macOS differences](#macos-differences) was present, with these additions and observations:

- Before sign-in `~/.codex` already held `.codex-global-state.json` (1,282 B), `config.toml`
  (2,114 B), `installation_id` (36 B), `.sqlite-maintenance.lock`, `ipc/ipc.sock`,
  `tmp/arg0-macos/codex-arg0*/`, `vendor_imports/skills-curated-cache.json`, `.tmp/`, `plugins/`,
  `skills/.system/`, `computer-use/Codex Computer Use.app`, `sqlite/codex-dev.db`, `state_5.sqlite`
  (every table empty but `thread_sections`, 1 row, and the bookkeeping tables), `logs_2.sqlite`,
  `goals_1.sqlite`, `memories_1.sqlite` and `queue_1.sqlite`; no `auth.json`, `sessions/` or
  `history.jsonl`. Sign-in wrote `auth.json` (4,299 B), `models_cache.json` (399,712 B, 9 models),
  `.sandbox_migration` (3 B), the first shell snapshot and the first `ambient-suggestions` file, and
  grew the global state to 110,327 B and `config.toml` to 2,380 B. No `history.jsonl` or
  `version.json` was written all round: the desktop app keeps prompts in the global state's
  `prompt-history`. `thread_history_1.sqlite` was created with the first thread, and
  `computer-use/config.json` (152 B) with it.
- `shell_snapshots/<thread uuid>.<unix ns>.sh` (2,135 to 2,299 B): the user's login-shell
  environment replayed for the sandbox, written by the source's
  [core/src/shell_snapshot.rs:107](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/core/src/shell_snapshot.rs#L107)
  ([Thread locks and shell snapshots](#thread-locks-and-shell-snapshots)). Flagged secret
  ([Credentials](#credentials)).
- `thread-writer-locks/<uuid>.lock`, also for prewarmed reviewer sessions that never wrote anything
  ([Thread locks and shell snapshots](#thread-locks-and-shell-snapshots)).
- `automations/.run-jitter-salt` (37 B) and `automations/<slug>/automation.toml` (356 B; the slug is
  the kebab-case of the task's name) ([Automations](#automations)).
- A second `ambient-suggestions/<40 hex>/` directory when `~/Demo` was added, one per project root
  ([Ambient suggestions](#ambient-suggestions)).
- `cache/remote_plugin_catalog/<16 hex>.json` (38,747,166 B, downloaded at each launch),
  `cache/codex_apps_server_info/<40 hex>.json` (189 B) and `cache/codex_apps_tools/<40 hex>.json`
  (825,347 B): under the excluded `.codex/cache`, which was 39.6 MB at collection.
- `config.toml` gains a `[projects."<path>"]` table with `trust_level` `trusted` when a project
  folder is added ([Standing grants and approval records](#standing-grants-and-approval-records)).
  Neither the model nor the permission mode (Approve for me, Full access) is written to it.
- `state_5.sqlite` on this build also has `guardian_review_feedback`,
  `external_agent_config_imports`, `thread_spawn_edges` and `thread_read_receipts`, and one
  `remote_control_enrollments` row once remote control is enabled
  ([`state_5.sqlite` remote control enrollments](#state_5sqlite-remote-control-enrollments)).
- `.codex-global-state.json` grew from 1,282 B before sign-in to 120,989 B by the remote-control
  step ([`.codex-global-state.json`](#codex-global-statejson)).
- Outside `~/.codex`: `~/.cache/codex-runtimes/codex-primary-runtime/` (32,625 entries), and sockets
  under `/private/tmp/codex-browser-use/` and `/private/tmp/codex-app-tools/`, replaced at each
  launch; none is collected.

#### Standalone CLI on Linux

Observed on the Ubuntu install of [Source and evidence](#source-and-evidence); names, types and
sizes only.

- `~/.codex/packages/standalone/` and `~/.codex/packages/app-server-daemon/` (816,297,770 B
  together, under the `.codex/packages` exclusion): `install.lock`, `auto-update-version` (34 B), a
  `current` symlink to `releases/0.162.1-aarch64-unknown-linux-musl` and that release (`bin/`, a
  `codex` symlink into `bin`, `codex-path/`, `codex-resources/`, `codex-package.json`). The target
  triple says the Linux build is statically linked against musl, a C library.
- `~/.local/bin/codex`, a symlink to `~/.codex/packages/standalone/current/bin/codex` (no
  `codex-code-mode-host` link, unlike macOS), and a block in `~/.bashrc` between
  `# >>> Codex installer >>>` and `# <<< Codex installer <<<` that prepends `~/.local/bin` to
  `PATH`, as macOS got in `~/.zprofile`. Neither is collected. No `~/.config/codex`,
  `~/.local/share/codex`, `~/.local/state/codex` or `~/.cache/codex*` exists: the CLI uses
  `~/.codex` only, and `~/.cache/codex-runtimes` belongs to the desktop app.
- The daemon runs from the `app-server-daemon` release and listens on a socket
  `/tmp/codex-daemon-<uid>/<64 hex>` with a `.lock` beside it;
  `~/.codex/app-server-control/app-server-control.sock` is a symlink to it.
  `app-server-daemon/daemon.pid` (423 B) and `daemon-updater.pid` (145 B) carry
  `processIdentity.{bootId, startTicks}`; the parser reads neither. `daemon.stderr.log` held one
  `rmcp::transport::streamable_http_client` error line, and `daemon-updater.stderr.log` the JSON
  lines seen on Windows.
- The Linux sandbox is bubblewrap (a tool that runs a process in its own namespaces):
  `/tmp/codex-bwrap-synthetic-mount-targets-<uid>/lock` appeared with the first turn.
- `log/codex-login.log` (544 B, mode 0600): four `tracing` lines of the browser login, from
  `codex_cli::login: starting browser login flow` through the `codex_login::server` callback line
  (`has_code`, `has_state`, `has_error`, `state_valid`) and the token exchange's issuer, token
  endpoint and redirect URI, to `oauth token exchange succeeded`. Source-cited above
  ([cli/src/login.rs:85](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/cli/src/login.rs#L85)).
- Otherwise the source-described files: `auth.json`, `config.toml` (42 B, holding only
  `tui.screen_reader_detection_done`), `installation_id`, `version.json`, `.sandbox_migration`,
  `.sqlite-maintenance.lock`, `history.jsonl` (one line per turn), `session_index.jsonl`, the SQLite
  stores with WAL sidecars, `models_cache.json`, `cache/`, `plugins/cache/openai-curated-remote` and
  `plugins/.remote-plugin-install-staging`, `skills/.system`, `tmp/`, an empty `shell_snapshots/`,
  `thread-writer-locks/.coordination.lock` and `tui-thread-reference-capabilities/<thread uuid>`. A
  fresh CLI's `~/.codex` has none of the desktop app's or Windows' files: no `sqlite/codex-dev.db`,
  `.codex-global-state.json`, `.desktop-created`, `automations`, `ambient-suggestions`,
  `computer-use`, `visualizations`, `vendor_imports`, `plugins/.plugin-appserver`, `.tmp`,
  `cap_sid`, `node_repl` or `.sandbox*`.

### Credentials

- `auth.json` with `OPENAI_API_KEY` and OAuth tokens
  ([login/src/auth/storage.rs:47-53](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/login/src/auth/storage.rs#L47-L53),[163](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/login/src/auth/storage.rs#L163)).
  Store mode `AuthCredentialsStoreMode`: `file` (default), `keyring`, `auto`, `ephemeral`
  ([config/src/types.rs:116-124](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/config/src/types.rs#L116-L124)).
  Keyring via `codex-keyring-store`. As Codex Desktop writes it (observed): `OPENAI_API_KEY`
  `null`, `auth_mode`, `last_refresh` and `tokens` (`access_token`, `id_token`, `refresh_token`,
  `account_id`). `ollama launch chatgpt` creates `auth.json` when the file does not exist, mode
  0600, with `{"OPENAI_API_KEY": "ollama-local-codex", "auth_mode": "apikey"}`; the key is a fixed
  local sentinel that Ollama's proxy refuses to forward to OpenAI, not a credential, and
  `ollama launch chatgpt --restore` deletes the file only while it still holds that value
  ([Ollama `cmd/launch/codex_app.go:561-631`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/codex_app.go#L561-L631),
  [`internal/proxy/codex_desktop.go:40-42`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/internal/proxy/codex_desktop.go#L40-L42)).
  The `.codex/auth.json` secret glob flags it either way; a flagged `auth.json` with that value
  means the integration was set up while no Codex login existed.
- On Linux (observed, Ubuntu 26.04.1, Codex CLI 0.162.1 standalone, 2026-10-15) the browser login
  wrote the OAuth tokens to `auth.json` (4,302 B) at the moment of the token exchange; the GNOME
  keyring was not used (`~/.local/share/keyrings` held only the stock `login.keyring` and
  `user.keystore`). `log/codex-login.log` names the login endpoints and the callback's flags, never
  a token, so it is not a credential ([Standalone CLI on Linux](#standalone-cli-on-linux)).
- `.credentials.json`: MCP OAuth token fallback when keyring unavailable
  ([rmcp-client/src/oauth.rs:17](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/rmcp-client/src/oauth.rs#L17),[857](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/rmcp-client/src/oauth.rs#L857);
  [config/src/types.rs:138](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/config/src/types.rs#L138)).
- `secrets/` gateway OAuth and local secrets, age-encrypted (`gateway_oauth.age`, `local.age`,
  `gateway_oauth.lock`)
  ([login/src/gateway_auth_storage.rs:20-24](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/login/src/gateway_auth_storage.rs#L20-L24),
  [secrets/src/local.rs:166](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/secrets/src/local.rs#L166),[587-591](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/secrets/src/local.rs#L587-L591)).
  At `4e7b9e2` the lock file is written by `login/src/oauth_session_storage.rs`
  ([21-25](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/login/src/oauth_session_storage.rs#L21-L25)),
  which replaced `gateway_auth_storage.rs`; the directory and file names are unchanged.
- `.sandbox-secrets/` Windows sandbox tokens
  ([setup.rs:196](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/windows-sandbox-rs/src/setup.rs#L196)).
  Observed with Codex Desktop: `sandbox_users.json` (828 B) with `offline` and `online` objects,
  each `{username, password}`, and `version`: the passwords of the two sandbox accounts. Flagged by
  `.codex/.sandbox-secrets/*`.
- `.codex-global-state.json` and its `.bak` (observed, Codex Desktop only): ChatGPT conversation
  resume tokens, the account's ids and the user's prompts ([Secrets](#secrets)). Flagged by
  `.codex/.codex-global-state.json*`
  ([issue 115](https://github.com/seanthegeek/doubleagent/issues/115)).
- `shell_snapshots/<thread uuid>.<unix ns>.sh` (observed on macOS, Codex Desktop 0.163.0-alpha.10.2
  inside ChatGPT 26.1013.40934, 2026-10-18): the user's exported login-shell environment replayed
  for the sandbox, so any API key the login shell exports is copied into the file; the names
  exported on the clean lab VM included no key-like name
  ([Thread locks and shell snapshots](#thread-locks-and-shell-snapshots)). Flagged by
  `.codex/shell_snapshots/*` ([issue 160](https://github.com/seanthegeek/doubleagent/issues/160)).
- Remote control's server token (observed on macOS, ChatGPT 26.1013.40934, 2026-10-18 and
  2026-10-19): the app refreshes it when "Allow connections" is turned back on, and
  `logs_2.sqlite` records the refresh, never the token. `state_5.sqlite`
  `remote_control_enrollments` has no token column and its `websocket_url` no query string,
  `auth.json` was not rewritten after sign-in, and a search of the login keychain for service or
  account names containing `openai`, `codex` or `chatgpt` was empty after every step, so where the
  token is kept was not determined. The enrollment row holds the account id and the Mac's host
  name but no secret.
- `config.toml` can embed `experimental_bearer_token` per provider
  ([model-provider-info/src/lib.rs:149-157](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/model-provider-info/src/lib.rs#L149-L157));
  `env_key` names an env var instead.
- A copy of `auth.json` outside the home (Ollama source at
  [`6787222`](https://github.com/ollama/ollama/commit/678722224b3d8c2a024a4fa5f530d83ce34db120)):
  `ollama launch chatgpt` copies `~/.codex/auth.json` and `models_cache.json` into a scratch
  `CODEX_HOME`, `ollama-codex-models-*` in the system temp directory, to list Codex's native models,
  and removes it when the command finishes
  ([`cmd/launch/codex_app.go:1480-1495`](https://github.com/ollama/ollama/blob/678722224b3d8c2a024a4fa5f530d83ce34db120/cmd/launch/codex_app.go#L1480-L1495)).
  A crash leaves that copy of the Codex credential in the temp directory, which no catalog line
  covers; see [Per-user storage in `ollama.md`](ollama.md#per-user-storage), and
  [Credentials in `collectors/docs/coverage.md`](../collectors/docs/coverage.md#credentials), which
  names it as a manual step.

At `4e7b9e2` no new credential file was found: the quoted file names ending `.age`, `.json`,
`.lock`, `.toml` or `.env` in `codex-rs/login`, `codex-rs/secrets`, `codex-rs/rmcp-client` and
`codex-rs/cli` differ between the two commits only by a configuration file name, not a credential
store.

### Exclusions

`.codex/packages`, `.codex/cache`, `.codex/.tmp`, `.codex/tmp`, `.codex/plugins/cache`,
`.codex/plugins/.plugin-appserver` and `.codex/plugins/.remote-plugin-install-staging` (Codex
Desktop, [issue 116](https://github.com/seanthegeek/doubleagent/issues/116)),
`.codex/skills/.system`, `.codex/worktrees`, `.codex/.sandbox`,
`.codex/.sandbox-bin`, `.codex/visualizations`, `.codex/app-server-daemon/packages` (if present).
`.codex/computer-use/*.app` (the computer-use app bundle on macOS, 77 MB,
[issue 121](https://github.com/seanthegeek/doubleagent/issues/121)).

On the Windows standalone install `.codex/packages` (794,201,900 B, the standalone and daemon
releases) and `.codex/.sandbox-bin` (289,784,416 B) fall under these exclusions, so the binaries
need no new one; `AppData/Local/Programs/OpenAI/Codex/bin` is a junction into `.codex/packages`
and is not catalogued. On Linux the daemon's socket directory `/tmp/codex-daemon-<uid>/` and the
bubblewrap sandbox's `/tmp/codex-bwrap-synthetic-mount-targets-<uid>/` are outside the home and
named only by the user id, so no catalog line can hold them.

`.codex/db-backups` is a copy of the state DBs, small, and may hold rows deleted from the live DB:
collect rather than exclude.

On ChatGPT 26.1013.40934 (macOS, 2026-10-19) the excluded trees under `.codex` measured `cache`
39.6 MB (of which `remote_plugin_catalog/` 38.7 MB), `plugins/cache` 67 MB,
`plugins/.plugin-appserver` 341 MB, `.tmp` 123 MB and `computer-use/Codex Computer Use.app` 78 MB,
beside the small `plugins/.remote-plugin-install-staging`, `skills/.system`, `tmp` and
`visualizations`; nothing new and large sat outside them. `Library/Application
Support/com.openai.codex` is updater state only, collected as `chatgpt-desktop`
([issue 121](https://github.com/seanthegeek/doubleagent/issues/121)).

### Project-local files

`AGENTS.md`, `AGENTS.override.md`
([core/src/agents_md.rs:43-45](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/core/src/agents_md.rs#L43-L45)),
`.codex/config.toml`, `.codex/agents/`, `.codex/hooks.json`, `.codex/hooks/`, `.codex/rules/`,
`.codex/skills/`
([external-agent-migration/src/scope.rs:59-62](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/external-agent-migration/src/scope.rs#L59-L62),
[detect/mod.rs:86](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/external-agent-migration/src/detect/mod.rs#L86),[167](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/external-agent-migration/src/detect/mod.rs#L167),[270](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/external-agent-migration/src/detect/mod.rs#L270);
[exec-server/src/discoverV2/capability_locations.rs:72](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/exec-server/src/discoverV2/capability_locations.rs#L72)),
`.agents/skills`
([external-agent-migration/src/service.rs:467](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/external-agent-migration/src/service.rs#L467)),
`generated_images/` in cwd
([ext/image-generation/src/tool.rs:332](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/ext/image-generation/src/tool.rs#L332)).
No per-project database.

### Where the project path is recorded

- First line of every rollout: `SessionMeta` with `cwd: PathBuf`, `timestamp`, `id`, `originator`,
  `cli_version`, `source`, `runtime_workspace_roots`
  ([protocol/src/protocol.rs:3123-3153](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/protocol.rs#L3123-L3153)).
  Compressed rollouts need zstd before grep. Project discovery reads it in `sessions/` and, from
  collector 1.18.0, in `archived_sessions/`, where an archived thread's rollout moves
  ([What survives a delete](#what-survives-a-delete)). It drops a `cwd` that is
  `<home>/Documents/Codex` or lies under it, the desktop app's projectless output directories,
  which the `codex-cli|Documents/Codex` catalog line collects instead, and, from 1.18.0, a `cwd`
  under `<home>/.codex/worktrees`, the thread worktrees
  ([Scratch directories](#scratch-directories);
  [issue 149](https://github.com/seanthegeek/doubleagent/issues/149),
  [issue 222](https://github.com/seanthegeek/doubleagent/issues/222)).
- `state_5.sqlite` `threads` `cwd`, which the analyzer reads
  ([`state_5.sqlite` threads](#state_5sqlite-threads)). `session_index.jsonl` holds titles only,
  no path ([`session_index.jsonl`](#session_indexjsonl)).
- `config.toml` `[projects."<path>"]` tables, each with `trust_level` `trusted` or `untrusted`
  ([Standing grants and approval records](#standing-grants-and-approval-records)); on macOS
  (ChatGPT 26.1013.40934, 2026-10-18) adding a project folder in the app wrote one.
- Codex Desktop (observed): `state_5.sqlite` `project_roots` `path`; `sqlite/codex-dev.db`
  `local_thread_catalog` `cwd`; and in `.codex-global-state.json`, `local-projects` `rootPaths`,
  `thread-workspace-root-hints` and `thread-projectless-output-directories`. On the Windows install
  both rollouts' `cwd` values named the same directories, so the rollouts are enough for discovery.

## Parsing

### Transcript stores

Paths relative to the home; `$CODEX_HOME` replaces `.codex` when set.

- `.codex/sessions/YYYY/MM/DD/rollout-<YYYY-MM-DDThh-mm-ss>-<thread id>.jsonl`,
  one per thread, the directory date and file-name time in **local** time
  ([rollout/src/recorder.rs:1722-1734](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/rollout/src/recorder.rs#L1722-L1734),
  [rollout_file_name.rs:62-73](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/rollout/src/rollout_file_name.rs#L62-L73)),
  while every timestamp inside is UTC: on the macOS install of 2026-10-18, at UTC-4, a thread
  started at 18:40:09Z is `rollout-2026-10-18T14-40-09-<uuid>.jsonl` and one started at 23:18:38Z is
  `sessions/2026/10/18/rollout-2026-10-18T19-18-38-<uuid>.jsonl`. The parser takes every time from
  the records, never from the file name. `thread/revert` writes a new file `rollout-<time>-<thread
  id>_<rollout id>.jsonl` that keeps the thread id and points back at its prefix
  ([recorder.rs:95-104](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/rollout/src/recorder.rs#L95-L104),
  [rollout_file_name.rs:40-47](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/rollout/src/rollout_file_name.rs#L40-L47)).
  Subagent threads get their own rollout in the same tree. **Parser reads.**
- `.codex/archived_sessions/`, same layout
  ([rollout/src/lib.rs:86-87](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/rollout/src/lib.rs#L86-L87)).
  **Parser reads.**
- `*.jsonl.zst`: zstd level 3, written by a background worker for rollouts idle seven days, in both
  trees
  ([compression.rs:29](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/rollout/src/compression.rs#L29),
  [362-363](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/rollout/src/compression.rs#L362-L363),
  [500-501](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/rollout/src/compression.rs#L500-L501)).
  The worker runs only with the `local_thread_store_compression` feature, which is under development
  and off by default
  ([features/src/lib.rs:1212-1217](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/features/src/lib.rs#L1212-L1217),
  [core/src/thread_manager.rs:461-503](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/core/src/thread_manager.rs#L461-L503)).
  **Parser reads** (needs `zstandard`).
- `.codex/history.jsonl`: one line per submitted prompt, all threads
  ([Other stores](#other-stores)). **Parser reads.**
- `.codex/session_index.jsonl`: thread renames ([Other stores](#other-stores)). Not read.
- `.codex/automations/<slug>/automation.toml`: a scheduled automation of
  the ChatGPT desktop app ([Other stores](#other-stores)). **Parser reads.**
- `.codex/state_5.sqlite`, `.codex/thread_history_1.sqlite`: thread metadata
  and a projection of paginated rollouts ([Other stores](#other-stores)).
  **Parser reads**, as secondary sources
  ([issue 122](https://github.com/seanthegeek/doubleagent/issues/122)).
- **Copies held by other agents.** Three agents copy these transcripts into stores of their own,
  where the copy outlives the original file (from those agents' source; none seen on a real
  install):
  - Hermes Agent's `hermes_cli/foreign_sessions.py` reads Codex's
    `~/.codex/sessions/**/rollout-*.jsonl` and appends their user and assistant turns to its
    `state.db` as a new session whose `origin_json.imported_from` names the tool, the source path
    and the foreign session id, stamped with the import time
    ([`hermes_cli/foreign_sessions.py:254-260`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/hermes_cli/foreign_sessions.py#L254-L260);
    [Transcript stores in `hermes.md`](hermes.md#transcript-stores)).
  - Cline's SDK session import makes an ordinary Cline session with a new id and
    `metadata.importedFrom{tool, sourceSessionId, sourcePath, importedAt, sourceProvider, sourceModel}`;
    at the reviewed commit only Cline's example desktop app calls it
    ([`sdk/packages/core/src/services/session-import/service.ts:349-366`](https://github.com/cline/cline/blob/41e75efc53e7abbcb76564b89e6de57f78659b33/sdk/packages/core/src/services/session-import/service.ts#L349-L366);
    [Transcript stores in `cline.md`](cline.md#transcript-stores)).
  - Pydantic AI's CLAI 2.0 (its command-line chat client) `/resume` copies them into its
    `sessions.db`
    ([`codex_sessions.py:35-37`](https://github.com/pydantic/pydantic-ai/blob/0ba01f92be3fc361f89970502cd6e7dd3611aec3/src/pydantic_clai2/pydantic_clai2/runtime/codex_sessions.py#L35-L37);
    [Other agents' sessions in `pydantic-clai.md`](pydantic-clai.md#other-agents-sessions)).
- **Indexed by Zed.** Zed's agent panel can run this agent as an external agent over ACP (the Agent
  Client Protocol, the JSON-RPC protocol Zed uses to drive another agent); its threads sidebar then
  lists the session in `sidebar_threads` with `agent_id` `codex-acp` and the ACP session id in
  `session_id`, while the transcript stays in this agent's own store
  ([`agent_servers/src/custom.rs:17-19`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/agent_servers/src/custom.rs#L17-L19);
  [Transcript stores in `zed.md`](zed.md#transcript-stores)). Whether that id equals the session id this
  agent records was not determined.

#### Observed rollouts

The rollouts of the real installs of [Source and evidence](#source-and-evidence), by line type; none
had a bad line, and every one was `history_mode` `paginated`.

| Install | Thread | Size, lines | Line types |
| --- | --- | --- | --- |
| macOS, Codex Desktop 0.163.0-alpha.10.2, 2026-10-18 | plain question, projectless | 190,766 B, 21 | `session_meta` 1, `event_msg` 6 (`task_started`, `thread_settings_applied`, `item_completed` 2, `token_count`, `task_complete`), `response_item` 11 (`additional_tools` 1, `message` with role `developer` 6, `user` 3, `assistant` 1), `world_state` 1, `turn_context` 1, `token_usage_record` 1 |
| same | coding task in `~/Demo`, first turn | 205,378 B, 39 | `session_meta` 1, `turn_context` 1, `world_state` 1, `response_item` 18 (developer 6, user 3, assistant 2, `additional_tools` 1, `custom_tool_call` 3, `custom_tool_call_output` 3), `event_msg` 14 (`item_completed` 7: `CommandExecution` 3, one `failed` with `exit_code` 1, `FileChange` 1, `AgentMessage` 2 with `phase` `commentary`, `UserMessage` 1; `token_count` 4, `task_started`, `thread_settings_applied`, `task_complete`), `token_usage_record` 4 |
| same | guardian review thread | 105,545 B, 17 | `session_meta`, `task_started`, 5 `response_item`, `world_state`, `turn_context`, `thread_settings_applied`, `item_completed` `Reasoning` and `AgentMessage`, `token_usage_record`, `token_count`, `task_complete` |
| same | scheduled task's thread | 42 lines after Run now, 62 after the scheduled run | not tabulated |
| Ubuntu, CLI 0.162.1, 2026-10-15 | two turns in `~/demo`, `sessions/2026/10/15/rollout-2026-10-15T13-16-43-<uuid>.jsonl` | 440,924 B, 108 | `session_meta` 1, `turn_context` 2 (one per turn), `world_state` 1, `response_item` 41, `event_msg` 49, `token_usage_record` 14 |
| macOS, Codex Desktop 0.162.0-alpha.17.2 and CLI 0.162.1, 2026-10-15 | four threads under `sessions/2026/10/15/` | 348 KB in all | not tabulated |
| Windows, CLI 0.162.1, 2026-10-15 | one prompt in `~\demo-cli`, `rollout-2026-10-15T11-41-45-<uuid>.jsonl` | 106,200 B, 47 | `session_meta` 1, `turn_context` 1, `world_state` 1, `response_item` 20, `event_msg` 18, `token_usage_record` 6 |
| Windows, Codex Desktop 0.162.0-alpha.17.2, 2026-10-15 | projectless thread | 37 lines | `session_meta` 1, `turn_context` 1, `world_state` 1, `response_item` 17, `event_msg` 13, `token_usage_record` 4 |
| same | project task on the Desktop | 32 lines | `session_meta` 1, `turn_context` 1, `world_state` 1, `response_item` 15, `event_msg` 11, `token_usage_record` 3 |
| same | automation's target thread | 83,579 B | one user turn, one assistant message, no tool call ([Automations](#automations)) |
| development host, Codex 0.160.0 | one thread | 20 lines | tabulated by type only ([Record schema](#record-schema)) |

The models seen were `gpt-6.1-sol` (the 2026-10-18 macOS threads before the switch, the Ubuntu and
Windows CLI threads and the three 2026-10-15 macOS threads other than the automation's),
`gpt-5.6-terra` (the 2026-10-15 automation threads on Windows and macOS, chosen by the app, and the
2026-10-18 Demo thread after the switch), `gpt-6-luna` (the two first Windows desktop threads)
and `codex-auto-review` (the guardian review thread).

#### What survives a delete

A `thread/delete` request (the TUI and the desktop app both go through the app server) refuses a
thread whose rollout a fork still references, then removes, for the thread and its spawned
subagents: the rollout under `sessions/` and under `archived_sessions/`, both the `.jsonl` and the
`.jsonl.zst` form, and the thread's `session_index.jsonl` lines
([app-server/src/request_processors/thread_delete.rs:31-63](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/app-server/src/request_processors/thread_delete.rs#L31-L63),
[thread-store/src/local/delete_thread.rs:95-136](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/thread-store/src/local/delete_thread.rs#L95-L136),
[204-281](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/thread-store/src/local/delete_thread.rs#L204-L281));
its `thread_history_1.sqlite` rows
([thread-store/src/local/thread_history.rs:242](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/thread-store/src/local/thread_history.rs#L242));
its agent message boards
([core/src/thread_manager.rs:484-496](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/core/src/thread_manager.rs#L484-L496));
and the `logs_2.sqlite` rows whose `thread_id` column is the thread, its queued items in
`queue_1.sqlite`, its memories `stage1_outputs` row, its goal in `goals_1.sqlite`, and its
`state_5.sqlite` rows in `thread_dynamic_tools`, `thread_spawn_edges`, `thread_read_receipts` and
`threads`
([state/src/runtime/threads.rs:1166-1216](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/state/src/runtime/threads.rs#L1166-L1216))
(source at `4e7b9e2`). An archive (`thread/archive`) instead moves each rollout into
`archived_sessions/` under its own file name, without the date directories
([thread-store/src/local/archive_thread.rs:67-130](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/thread-store/src/local/archive_thread.rs#L67-L130)).

Neither delete handler touches `history.jsonl` (grepped
[`delete_thread.rs`](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/thread-store/src/local/delete_thread.rs)
and
[`thread_delete.rs`](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/app-server/src/request_processors/thread_delete.rs)
for `history.jsonl` and `message_history`), which keeps one line per prompt typed into the TUI with
the thread id and its time in seconds ([`history.jsonl`](#historyjsonl)) and is then the one place
such a prompt survives. A delete also leaves the memories files until the next consolidation, the
`db-backups/` copies of a database Codex found corrupt
([Other past-use stores](#other-past-use-stores)), and log rows that name the thread only in their
text.

The desktop app's own delete (macOS, build 26.1013.40934, 2026-10-19; the dialog mentioned no
remote copy) was a hard delete: no `archived_sessions/` directory was created. It removed the
rollout, its `session_index.jsonl` line, its `state_5.sqlite` `threads` and
`thread_read_receipts` rows, its `thread_history_1.sqlite` rows, its `sqlite/codex-dev.db`
`local_thread_catalog` row and the global state's `thread-client-id-v1:local%3A<thread id>` key;
`logs_2.sqlite` has the `fs/remove` and `thread/delete` requests of that second. It left the
global-state keys that name the thread (its `prompt-history` entry with the prompts,
`thread-descriptions-v1`, `thread-user-activity-times-v1`, `heartbeat-thread-permissions-by-id`,
`client-thread-bindings-v1`, and the top-level `projectless-thread-ids`,
`thread-projectless-output-directories` and `thread-workspace-root-hints`) and added
`codex-writing-block-deleted-thread-v1:<uuid>` set to `true`, the deletion marker; it also left the
empty projectless output directory `~/Documents/Codex/2026-10-18/w` and at least one
`logs_2.sqlite` row carrying the thread id. Source deletes only the log rows whose `thread_id`
column matches, so which rows survived (rows with the id only in their message text, rows written
after the delete, or a build that did not yet delete log rows) was not determined.

Hermes Agent, Cline's SDK and Pydantic AI's CLAI hold copies that outlive the original rollout
([Transcript stores](#transcript-stores)). The analyzer reads the archived rollouts as primary,
`history.jsonl`, the global state and the `db-backups/` copies of `state_5.sqlite` as secondary
sources (the marker as a `session deleted:` row), and the memory rollout summaries as a source of
their own, since they repeat no event (see
[Secondary sources](../analyzer/docs/output.md#secondary-sources)).

### Record schema

Each line is a `RolloutLine`: `timestamp`, optional `ordinal`, and the item
flattened beside them
([history/src/lib.rs:356-367](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/history/src/lib.rs#L356-L367),
[recorder.rs:2062-2091](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/rollout/src/recorder.rs#L2062-L2091)).
The item serialises as `{"type": <snake_case variant>, "payload": {...}}`;
`response_item` lines may also carry a sibling `metadata` object
([history/src/rollout_payload.rs:30-72](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/history/src/rollout_payload.rs#L30-L72)).
Codex's own reader requires `timestamp` and treats `ordinal` as optional
([rollout/src/lib.rs:52-74](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/rollout/src/lib.rs#L52-L74)).
`ordinal` is written only in paginated rollouts, counting from 0 or from the
`history_base` the file continues
([rollout/src/ordinal.rs:22-44](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/rollout/src/ordinal.rs#L22-L44)).

Line types
([history/src/lib.rs:209-227](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/history/src/lib.rs#L209-L227));
which ones are written is decided in
[rollout/src/policy.rs:22-50](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/rollout/src/policy.rs#L22-L50):

| `type` | `payload` | Seen locally |
| --- | --- | --- |
| `session_meta` | thread header, below | yes |
| `turn_context` | per-turn settings, below | yes |
| `response_item` | a Responses API item ([Message content and tool calls](#message-content-and-tool-calls)) | yes |
| `event_msg` | a UI event, tagged by its own `type` (below) | yes |
| `token_usage_record` | `thread_id`, `turn_id`, `session_id`, `root_turn_id`, `response_id`, `usage`, `turn_token_usage`, `thread_token_usage` ([protocol.rs:2263-2273](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/protocol.rs#L2263-L2273)) | yes |
| `world_state` | `full`, `state` (model-visible environment snapshot or patch, [protocol.rs:3272-3278](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/protocol.rs#L3272-L3278)) | yes |
| `compacted` | `message` (the compaction summary), `replacement_history`, window ids ([history/src/lib.rs:285-306](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/history/src/lib.rs#L285-L306), [rollout_payload.rs:238-266](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/history/src/rollout_payload.rs#L238-L266)) | no |
| `inter_agent_communication` | `author`, `recipient`, `other_recipients`, `content` (plain text), `encrypted_content`, `trigger_turn` ([protocol.rs:806-822](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/protocol.rs#L806-L822)) | no |
| `inter_agent_communication_metadata` | `trigger_turn` | no |
| `retained_context` | verified answers and delivered messages for the guardian reviewer ([history/src/retained_context.rs:145-162](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/history/src/retained_context.rs#L145-L162)) | no |
| `security_risk_score` | `scores`, `call_id`, `action`, `sampled_at` ([security_risk.rs:12-25](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/security_risk.rs#L12-L25)) | no |
| `realtime_item` | `id`, `realtime_session_id`, tagged content incl. `transcript_segment` (`role`, `text`); paginated only ([realtime.rs:6-31](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/realtime.rs#L6-L31)) | no |

#### `session_meta`

`session_meta` (line 1;
[protocol.rs:3117-3198](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/protocol.rs#L3117-L3198)):
`id` (this thread), `session_id` (the root thread's id; filled from `id` when absent in old files,
[3243-3269](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/protocol.rs#L3243-L3269)),
`forked_from_id`, `parent_thread_id`, `timestamp`, `cwd`, `runtime_workspace_roots`, `originator`
(`codex-tui` locally), `cli_version`, `source` (`cli`, `vscode`, `exec`, `mcp`, a custom string, or
`{"subagent": ...}`;
[2824-2837](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/protocol.rs#L2824-L2837),
[2905-2923](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/protocol.rs#L2905-L2923)),
`thread_source` (`user`, `subagent`, `guardian_review`, `memory_consolidation` or a feature name;
[2843-2849](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/protocol.rs#L2843-L2849)),
`agent_nickname`, `agent_role`, `agent_path`, `model_provider`, `base_instructions`,
`dynamic_tools`, `memory_mode`, `history_mode` (`legacy` or `paginated`), `history_base`,
`context_window`, and the account fields `creator_user_id`, `creator_account_id`. Beside them, `git`
(`commit_hash`, `branch`, `repository_url`;
[3436-3452](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/protocol.rs#L3436-L3452))
is written only when `cwd` is inside a git repository
([recorder.rs:1978-1990](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/rollout/src/recorder.rs#L1978-L1990)).
`repository_url` is a `SanitizedGitUrl`, a remote URL with its credentials removed before it is
stored: every password, and every username other than the ssh `git` user, is dropped
([sanitized_git_url.rs:11](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/sanitized_git_url.rs#L11),
[73-98](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/sanitized_git_url.rs#L73-L98)).
The type arrived in commit
[1bc02ae](https://github.com/openai/codex/commit/1bc02aea5065dd5d1df540ecc9625794ad7a3172)
of 2026-08-25, so a rollout an earlier build wrote, and one a fork without that commit wrote
(Open Interpreter is such a fork), can hold the URL with its credentials; the parser writes the
stored string as it is.

Observed on Codex Desktop (Windows, 0.162.0-alpha.17.2, 2026-10-15; macOS, 0.163.0-alpha.10.2,
2026-10-18) and in the Windows standalone CLI's rollout: `id`, `session_id`, `timestamp`, `cwd`,
`runtime_workspace_roots` (1 entry), `originator`, `cli_version`, `source` `vscode`,
`thread_source` `user`, `model_provider` `openai`, `base_instructions` (`text`, of 21,769
characters on macOS, and `provenance` with `type` `model` and `model`), `history_mode`
`paginated`, `context_window` (`window_id`), `creator_user_id` (29 characters) and
`creator_account_id` (36 characters, a UUID). Every top-level key is in the source list above;
`base_instructions.provenance` is observed, not checked against source at
[3e23877](https://github.com/openai/codex/commit/3e238776e857eccd3bde6bff3026e2e9798f6524). No
real install had a `git` block, as no thread ran in a repository. A guardian review thread adds
`parent_thread_id` (absent on user threads) and `multi_agent_version` `disabled`
([Guardian review threads](#guardian-review-threads)).

#### `turn_context`

`turn_context` (once per user turn and after mid-turn compaction;
[protocol.rs:3296-3358](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/protocol.rs#L3296-L3358)):
`turn_id`, `root_turn_id`, `cwd`, `workspace_roots`, `current_date`, `timezone`, `approval_policy`,
`sandbox_policy`, `permission_profile`, `network`, `model`, `collaboration_mode` (with its own
`settings.model`), `effort`, `summary`, `personality` and more. The four policy fields:
`approval_policy` is a kebab-case string, `untrusted`, `on-request` (the default, also read from the
old `on-failure`) or `never`, or an object `{"granular": {...}}` for per-flow controls
([protocol.rs:986-1009](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/protocol.rs#L986-L1009));
`sandbox_policy` is an object tagged by `type`: `danger-full-access`, `read-only`,
`external-sandbox` or `workspace-write`, the last three with fields of their own
([protocol.rs:1072-1120](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/protocol.rs#L1072-L1120));
`permission_profile`, optional, is an object tagged by `type`: `managed` (`file_system`, `network`),
`disabled` or `external` (`network`)
([models.rs:421-435](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/models.rs#L421-L435));
`network`, optional, is `{allowed_domains, denied_domains}`, two lists of strings
([protocol.rs:3291-3294](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/protocol.rs#L3291-L3294)).

Observed on Codex Desktop (Windows, 2026-10-15; macOS, 2026-10-18): `turn_id`, `root_turn_id`,
`cwd`, `workspace_roots` (2), `current_date`, `timezone`, `approval_policy`, `sandbox_policy`
(`type` `workspace-write`, `writable_roots`, 2 on macOS, `network_access`,
`exclude_tmpdir_env_var`, `exclude_slash_tmp`), `permission_profile` (`type` `managed`,
`file_system` with `type` `restricted` and 13 `entries`, `network` `restricted`), `model`
(`gpt-6-luna` on Windows, `gpt-6.1-sol` on macOS), `collaboration_mode` (`mode` `default`,
`settings` with `model`, `reasoning_effort`, `medium` on Windows, and `developer_instructions`, of
1,288 characters on macOS), `effort` (`medium` on Windows, `low` on macOS), and on Windows `summary`
`detailed` and `personality` `friendly`. `approval_policy` was the granular object
(`mcp_elicitations`, `request_permissions`, `rules`, `sandbox_approval`, `skill_approval`; on
Windows `request_permissions` and `mcp_elicitations` true, the rest false) in the first thread of
each install and the string `on-request` in later ones. Observed and not among the fields listed
above from [3e23877](https://github.com/openai/codex/commit/3e238776e857eccd3bde6bff3026e2e9798f6524):
`disabled_plugin_ids` (empty), `approvals_reviewer` `user`, `active_permission_profile` (`id`
`:workspace`), `file_system_sandbox_policy` (`kind` `restricted`, 13 `entries` on Windows, 18 on
macOS), `comp_hash`, `multi_agent_version` `v2`, `realtime_active` and `cyber_access_program`
`standard`; the drift to `4e7b9e2` in [Format versions](#format-versions) lists those the struct has
there. Full access and the guardian thread give other values
([`event_msg`](#event_msg), [Guardian review threads](#guardian-review-threads)).

#### `world_state`

`world_state.payload` is `full` and `state`, the model-visible environment snapshot or a patch to it
([protocol.rs:3272-3278](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/protocol.rs#L3272-L3278)).
The keys observed on Codex Desktop (macOS, 0.163.0-alpha.10.2, 2026-10-18): `tools` (the
`mcp__codex_apps__*` tools `github`, `pets`, `hotline`, `sites`, `search_service`,
`safety_settings`, `plugin_management` and `codex_document_control`, plus `mcp__code_review` and
`mcp__node_repl`), `top_level_tools` (`clock.*`, `collaboration.*` with `spawn_agent` and
`send_message`, `functions.exec`, `functions.request_user_input`, `functions.wait`,
`mcp__cua_repl.*`), `permissions` (`approved_command_prefixes`, `instructions`), `environments`
(`current_date`, `timezone`, `filesystem`), `host_skills`, `multi_agent_mode` (`mode`
`explicitRequestOnly`) and `git_attribution`. The parser skips the line
([Parser plan](#parser-plan)).

#### `event_msg`

`event_msg` (tagged enum,
[protocol.rs:1352-1358](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/protocol.rs#L1352-L1358)).
Persisted in every mode
([policy.rs:127-139](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/rollout/src/policy.rs#L127-L139)):
`task_started` (`turn_id`, `root_turn_id`, `trace_id`, `started_at` epoch s, `model_context_window`,
`collaboration_mode_kind`), `task_complete` (`turn_id`, `last_agent_message`, `error`, `started_at`,
`completed_at`, `duration_ms`, `time_to_first_token_ms`)
([2152-2197](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/protocol.rs#L2152-L2197);
the wire names are kept from v1,
[1407-1418](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/protocol.rs#L1407-L1418)),
`turn_aborted` (`turn_id`, `reason`, `error`, times;
[4243-4263](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/protocol.rs#L4243-L4263)),
`thread_rolled_back` (`num_turns`;
[3692-3696](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/protocol.rs#L3692-L3696)),
`token_count`, `thread_goal_updated`, `thread_settings_applied`, and `item_completed` (below).
Legacy-mode rollouts also keep `user_message` (`message`, `images`), `agent_message` (`message`,
`phase`), `agent_reasoning` and `agent_reasoning_raw_content` (`text`), `entered_review_mode`,
`exited_review_mode`, `patch_apply_end`, `context_compacted`, `mcp_tool_call_end`, `web_search_end`,
`image_generation_end` and `sub_agent_activity`
([policy.rs:141-158](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/rollout/src/policy.rs#L141-L158);
[protocol.rs:2518-2544](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/protocol.rs#L2518-L2544),
[2637-2645](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/protocol.rs#L2637-L2645)).
These repeat what the `response_item` lines carry.

`item_completed` (`thread_id`, `turn_id`, `item`, `started_at_ms`,
`completed_at_ms`;
[protocol.rs:1947-1960](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/protocol.rs#L1947-L1960))
carries a `TurnItem` tagged by PascalCase `type`: `UserMessage`,
`AgentMessage`, `Reasoning`, `CommandExecution` (`command`, `cwd`,
`aggregated_output`, `exit_code`, `duration`), `McpToolCall` (`server`,
`tool`, `arguments`, `result`), `FileChange` (`changes`, `stdout`,
`stderr`), `WebSearch`, `Extension` (`kind` `web.search`, `clock.sleep` or
`image_gen.generation`), `Plan`, `CollabAgentToolCall` (`prompt`, `model`,
receiver thread ids) and others
([items.rs:42-78](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/items.rs#L42-L78),
[243-281](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/items.rs#L243-L281),
[337-422](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/items.rs#L337-L422),
[443-486](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/items.rs#L443-L486);
[ext/items/src/lib.rs:32-45](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/ext/items/src/lib.rs#L32-L45)).
Paginated rollouts persist every completed item, with command output and
MCP results cut to 64 KiB; legacy rollouts keep only `FunctionCallOutput`,
`Plan`, `clock.sleep` and completed `SubAgentActivity` items
([policy.rs:16-19](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/rollout/src/policy.rs#L16-L19),
[227-283](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/rollout/src/policy.rs#L227-L283)).
Locally the paginated rollout held `UserMessage`, `Extension`
(`web.search`, with `query`, `action` and `results`) and `AgentMessage`
items.

Observed on Codex Desktop (macOS, 0.163.0-alpha.10.2, 2026-10-18):

- `item_completed` items: `CommandExecution` with `id`, `process_id`, `cwd`, `parsed_cmd`,
  `source`, `status`, `aggregated_output`, `exit_code` and `duration` (`secs`, `nanos`);
  `FileChange` with `changes` keyed by the absolute paths, `status` `completed`, `stdout` and
  `stderr`; `AgentMessage` with `phase`; `UserMessage` with `client_id`, `content` and `id` (the
  runs of a scheduled task have no `client_id`); `Extension` with `kind` `web.search` (`action`,
  `id`, `kind`, `query`, `results`, `type`); `Reasoning` (`id`, `raw_content`, `summary_text`).
- `task_started.turn_attribution`: `turn_id`, `turn_trigger`, `parent_turn_id`,
  `initiating_agent_path` and `root_turn_id` (in source from `4e7b9e2`,
  [Format versions](#format-versions)). The `turn_trigger` values of this build: `composer` (a prompt
  typed in the app), `remote_ios` and `remote_android` (a prompt from a paired phone, whose
  `turn_context` has the shape of a composer turn; the trigger is the only mark in the rollout that
  a turn came from a phone), `guardian_review`, `scheduled_task_setup`,
  `automation_heartbeat_run_now` and `automation_heartbeat_scheduled` ([Automations](#automations)).
  `task_started.model_context_window` went from 475000 to 258400 at the model switch.
- `token_count.rate_limits`: `limit_id` (`codex`), `plan_type` (`plus`), `primary`, `secondary`,
  `credits`, `limit_name`, `individual_limit`, `spend_control_reached` and
  `rate_limit_reached_type`, so every rollout names the account's plan.
- `thread_settings_applied`, the thread's settings, appended the moment the user changes one, before
  any prompt, and again before the next turn: `thread_settings` with 14 keys,
  `active_permission_profile`, `approval_policy`, `approvals_reviewer`, `collaboration_mode`, `cwd`,
  `disabled_plugin_ids`, `model`, `model_provider_id`, `permission_profile`, `personality`,
  `reasoning_effort`, `reasoning_summary`, `runtime_workspace_roots` and `service_tier`, and no
  `sandbox_policy`. Opening a thread after a restart appends one with 11 keys (the 14 less
  `personality`, `reasoning_summary` and `service_tier`;
  [Joins and timestamps](#joins-and-timestamps)). The model switch set `thread_settings.model` to
  `gpt-5.6-terra` (the three earlier events said `gpt-6.1-sol`), and the next `turn_context`
  followed. Approve for me set `approvals_reviewer` `user` -> `auto_review`, with `approval_policy`
  still `on-request`, `permission_profile` `managed` and `active_permission_profile` `id`
  `:workspace`; a third, identical event preceded the turn. Full access set `approval_policy`
  `never`, `approvals_reviewer` `user`, `active_permission_profile` `id` `:danger-full-access` and
  `permission_profile` `{"type": "disabled"}`, and the turn's `turn_context` added `sandbox_policy`
  `danger-full-access` and `file_system_sandbox_policy` null; setting it back appended an event with
  the earlier values (`on-request`, `auto_review`, `:workspace`, `managed`). The app does not turn
  Full access off by itself: it stays a thread setting until changed. `state_5.sqlite` `threads`
  (`model`, `approval_mode`) and the global state keep only the latest values, so a model switch and
  a full-access turn are recorded only in the rollout and `logs_2.sqlite`.
- No approval event. A write inside the workspace ran in the sandbox with no request, and a declined
  or approved call left only its `CommandExecution` (`failed` or `completed`) and no change to
  `turn_context`; the decisions are in [`logs_2.sqlite`](#logs_2sqlite). The declined call's output
  was the text `Script error: exec_command failed: CreateProcess { message: "Rejected(\"rejected
  by user\")" }`, after the same `curl` had failed in the sandbox with `exit_code` 6, the network
  sandbox (`network_access` false).

Everything else on the `EventMsg` enum (deltas, begin events, approvals,
`exec_command_end`, errors, warnings) is transient and never written
([policy.rs:160-223](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/rollout/src/policy.rs#L160-L223)).

#### Message content and tool calls

`response_item.payload` is a `ResponseItem` tagged by snake_case `type`
([protocol/src/models.rs:1010-1256](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/models.rs#L1010-L1256));
all variants but `compaction_trigger` and unknown ones are written
([policy.rs:78-100](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/rollout/src/policy.rs#L78-L100)).
Most variants carry an optional `id` and
`internal_chat_message_metadata_passthrough` (`turn_id`, `create_time` in
fractional epoch seconds, `content_item_kinds`;
[models.rs:954-994](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/models.rs#L954-L994)).

- `message`: `role` (`user`, `assistant`, `developer`; the harness's own instructions go in as
  `developer`), `content[]` of `input_text`, `input_image` (`image_url` or `file_id`), `input_audio`
  or `output_text` blocks, each with `text` where textual
  ([876-896](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/models.rs#L876-L896)),
  and `phase` (`commentary` or `final_answer`;
  [938-952](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/models.rs#L938-L952),
  [1021-1036](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/models.rs#L1021-L1036)).
  A `role: "user"` message is not always typed by the user: the harness injects `AGENTS.md`
  instructions, `<environment_context>`, skill text, subagent notifications, turn-aborted notices
  and user shell commands as user-role messages
  ([core/src/context/contextual_user_message.rs:22-37](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/core/src/context/contextual_user_message.rs#L22-L37)).
  Current versions label each content block in `content_item_kinds`; typed input is `user.text`, the
  environment block `environments.environment_context`
  ([contextual_user_message.rs:51-82](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/core/src/context/contextual_user_message.rs#L51-L82),
  [core/src/context/world_state/environment.rs:200-203](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/core/src/context/world_state/environment.rs#L200-L203)).
  Locally the first turn had three `developer` messages and two `user` messages, of which one was
  `environments.environment_context` and one `user.text`.
- `reasoning`: `summary[]` of `summary_text` (`text`), `content[]` of `reasoning_text` or `text`,
  `encrypted_content`
  ([1048-1060](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/models.rs#L1048-L1060),
  [1982-1993](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/models.rs#L1982-L1993)).
  `content` is written only when it holds `reasoning_text` parts or is null
  ([1624-1631](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/models.rs#L1624-L1631)).
- `function_call`: `name`, `namespace` (MCP and plugin tools), `arguments`
  (a JSON **string**), `call_id`
  ([1074-1093](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/models.rs#L1074-L1093)).
- `custom_tool_call`: `status`, `call_id`, `name`, `namespace`, `input` (free text)
  ([1134-1151](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/models.rs#L1134-L1151)).
  Code mode's `exec` tool
  ([code-mode-protocol/src/lib.rs:52](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/code-mode-protocol/src/lib.rs#L52))
  and the freeform `apply_patch` arrive this way; the one local call was `exec`.
- `local_shell_call`: `call_id`, `status`, `action` (`type: "exec"`, `command` array, `timeout_ms`,
  `working_directory`, `env`, `user`)
  ([1061-1073](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/models.rs#L1061-L1073),
  [1937-1950](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/models.rs#L1937-L1950)).
- `function_call_output`, `custom_tool_call_output`: `call_id`, `name`, `output`, which is a string
  or an array of `input_text`, `input_image`, `input_audio`, `encrypted_content` items
  ([1114-1133](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/models.rs#L1114-L1133),
  [1155-1169](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/models.rs#L1155-L1169),
  [2096-2118](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/models.rs#L2096-L2118),
  [2250-2263](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/models.rs#L2250-L2263)).
  Locally an array of two `input_text` items.
- `web_search_call`: `status`, `action` (`search` with `query`/`queries`, `open_page` with `url`,
  `find_in_page` with `url`, `pattern`)
  ([1191-1204](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/models.rs#L1191-L1204),
  [1952-1980](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/models.rs#L1952-L1980)).
  No output item.
- `tool_search_call` (`call_id`, `execution`, `arguments` object) and `tool_search_output`
  (`call_id`, `status`, `tools[]`)
  ([1094-1108](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/models.rs#L1094-L1108),
  [1170-1182](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/models.rs#L1170-L1182)).
- `agent_message`: `author`, `recipient`, `content[]` of `input_text` or `encrypted_content`
  ([906-911](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/models.rs#L906-L911),
  [1037-1047](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/models.rs#L1037-L1047)).
- `image_generation_call`: `status`, `revised_prompt`, `result` (image data)
  ([1214-1226](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/models.rs#L1214-L1226)).
- `additional_tools` (tool definitions), `compaction` and `context_compaction`
  (`encrypted_content`), `configuration_update`: no readable conversation content
  ([1013-1020](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/models.rs#L1013-L1020),
  [1227-1253](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/models.rs#L1227-L1253)).

The `metadata` sibling on a `response_item` line is harness bookkeeping:
`client_authored`, `retained_source`, `user_input_order`,
`inherited_user_message` (copied from a parent thread), `mcp_attribution`
and others
([history/src/lib.rs:55-135](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/history/src/lib.rs#L55-L135)).

Observed on Codex Desktop (macOS, 2026-10-18): messages carry
`internal_chat_message_metadata_passthrough`; the line `metadata` keys were `client_authored`,
`mcp_attribution`, `retained_source` and `user_input_order`; the assistant message has `phase` and
`status`; the coding task's `custom_tool_call` lines had `call_id`, `id`, `input`, `name` and
`status`. Tool names seen on the real installs: `exec`, `shell`, `web_search` and `apply_patch`;
`node_repl.js`, a tool of the `node_repl` MCP server the desktop app's `config.toml` configures
(Windows CLI); `codex_app.load_workspace_dependencies` (macOS desktop, 2026-10-15); and
`request_user_input_async` (Ubuntu CLI), the CLI asking the user a question mid-turn.

#### Guardian review threads

In the Approve for me mode (Codex Desktop, macOS, 0.163.0-alpha.10.2, 2026-10-18) an approval
request opens no dialog: the app starts a reviewer thread with its own rollout
([Observed rollouts](#observed-rollouts)). Codex calls the reviewer the guardian (`thread_source`
`guardian_review`) and the mode auto-review (`approvals_reviewer` `auto_review`); the app's global
state calls the mode `guardian-approvals`. The reviewer's `session_meta` has `source`
`{"subagent": {"other": "guardian"}}`, `thread_source` `guardian_review`, `parent_thread_id` the
governed thread, the same `cwd` and two `runtime_workspace_roots`; its `turn_context` has `model`
`codex-auto-review`, `approval_policy` `never`, `approvals_reviewer` `auto_review` and
`sandbox_policy` `read-only`; `turn_attribution.turn_trigger` is `guardian_review`.
`task_complete.last_agent_message`, and the `AgentMessage` item, is a JSON string with `risk_level`
(`low`), `user_authorization` (`high`), `outcome` (`allow`) and `rationale` (free text): the
automatic decision for the call. The parent rollout then shows the governed `FileChange`
completing (21:27:27Z on the install) with no approval event of its own; `logs_2.sqlite` joins the
two ([`logs_2.sqlite`](#logs_2sqlite)). The reviewer thread has a `state_5.sqlite` `threads` row
(`thread_source` `guardian_review`, `model` `codex-auto-review`, `approval_mode` `never`,
`has_user_event` 0), items in `thread_history_1.sqlite` and a `session_index.jsonl` line named
`Guardian review`, but no `thread_spawn_edges` row. The app also prewarms reviewer sessions that
write no rollout ([Thread locks and shell snapshots](#thread-locks-and-shell-snapshots)).

### Joins and timestamps

- **Timestamps.** Line `timestamp` and `session_meta.payload.timestamp`: UTC
  `YYYY-MM-DDThh:mm:ss.mmmZ`, the line one taken at write time
  ([recorder.rs:927-933](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/rollout/src/recorder.rs#L927-L933),
  [2077-2082](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/rollout/src/recorder.rs#L2077-L2082));
  local values all had three fraction digits and `Z`. `task_*` `started_at`/`completed_at`: epoch
  seconds; `item_completed` `*_at_ms`: epoch ms; `create_time`: fractional epoch seconds;
  `history.jsonl` `ts`: epoch seconds.
- **Ids.** `session_meta.payload.id` is the thread id and the file-name
  UUID; `session_id` is the root thread's id, so a subagent's rollout has
  `id` ≠ `session_id`. Reverted rollouts end `<thread id>_<rollout id>`.
- **Surface.** `session_meta.payload.originator` is the key that tells
  the surfaces apart: `codex-tui` for the CLI and `Codex Desktop` for
  the ChatGPT desktop app. `source` was `vscode` for both on the Windows
  install observed on 2026-10-15 (desktop 0.162.0-alpha.17.2, standalone
  CLI 0.162.1), so it does not separate them. The two surfaces write
  one `.codex` tree, its sessions and its login, and their rollouts have
  the same envelope and `session_meta` keys, differing in `originator`
  and `cli_version`; the parser reads both alike.
- **Account and user.** `session_meta.payload.creator_account_id` (a
  UUID) and `creator_user_id` (the ChatGPT user id, 29 characters) name
  who created the thread; Codex Desktop fills them (observed on Windows,
  2026-10-15). The same pair is in the `state_5.sqlite` `threads`
  columns of the same names, and the user id, with the ChatGPT workspace
  id, in the `codex-dev.db` `local_thread_catalog_hosts` host id
  `chatgpt:<workspace uuid>:<user id>` ([Other stores](#other-stores)).
- **Project.** `session_meta.payload.cwd`, updated per turn by
  `turn_context.payload.cwd`; `workspace_roots` lists extra roots.
- **Branch.** `session_meta.payload.git.branch`, set once at thread
  creation, absent outside a git repository.
- **Model.** `turn_context.payload.model`; `model_provider` on
  `session_meta`. No per-message model. Codex Desktop also appends
  `event_msg` `thread_settings_applied` (`model`, `reasoning_effort`
  and the permission fields) the moment the user changes a setting,
  before the next turn ([`event_msg`](#event_msg)); `state_5.sqlite`
  `threads.model` keeps only the latest.
- **Resume.** Resuming a thread appends to its rollout: no new file, no
  second `session_meta` and no resume event (Codex Desktop, macOS,
  2026-10-18). The one mark is the 11-key `thread_settings_applied` the
  app appends when the thread is opened ([`event_msg`](#event_msg));
  `logs_2.sqlite` has the `thread/resume` requests (21:46:06Z and
  21:46:11Z on the install) with `app_server.client_name` `Codex Desktop`
  and `app_server.client_version` `26.1013.40934`. The model switch and
  the permission mode survived the restart.
- **Subagents.** Each spawned agent has its own rollout whose
  `session_meta` has `parent_thread_id`, `agent_nickname`, `agent_role`,
  `agent_path`, `thread_source: "subagent"` and `source:
  {"subagent": {"thread_spawn": {parent_thread_id, depth, ...}}}`;
  messages between agents are `inter_agent_communication` lines.
  Codex Desktop's guardian reviewer has no `thread_spawn` and no
  `thread_spawn_edges` row ([Guardian review threads](#guardian-review-threads)).
- **Forks.** `forked_from_id` on the child. A copied fork re-appends the
  parent's records into the child file at fork time, so they carry the
  fork's line timestamps; a referenced (paginated) fork keeps them in the
  parent file behind `history_base` (`thread_id`, `end_ordinal_exclusive`,
  `end_byte_offset`)
  ([core/src/session/mod.rs:1672-1695](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/core/src/session/mod.rs#L1672-L1695),
  [protocol.rs:3101-3115](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/protocol.rs#L3101-L3115)).
  Codex's own metadata sync only trusts a `session_meta` whose `id` is the
  file's thread, which implies copied files can hold an ancestor's
  `session_meta` too
  ([thread-store/src/thread_metadata_sync.rs:246-256](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/thread-store/src/thread_metadata_sync.rs#L246-L256)).
- **Joins.** Tool call to result: `call_id`, equal on the call and its
  `*_output`. Turn: `turn_id` on `turn_context`, `task_started`,
  `task_complete`, `item_completed` and in each item's metadata passthrough.
  An `item_completed` item that a code-mode `exec` call produced (the local
  `web.search`) has its own id, not the call's `call_id`, so nothing joins it
  to the call except its position between the call and its output (local).

### Other stores

#### `history.jsonl`

Each line is `{session_id, ts, text}`, `ts` epoch **seconds**, `session_id` the thread id, one line
per prompt typed into the TUI
([message-history/src/lib.rs:61-66](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/message-history/src/lib.rs#L61-L66),
[127-138](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/message-history/src/lib.rs#L127-L138)).
Written unless `history.persistence = "none"`
([config/src/types.rs:211-219](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/config/src/types.rs#L211-L219));
trimmed oldest-first when `history.max_bytes` is set
([message-history/src/lib.rs:191-195](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/message-history/src/lib.rs#L191-L195)).
It can outlive the rollout it names (deleted or ephemeral threads), so it is the one place such a
prompt survives. Local: one line, those three keys, 10-digit `ts`. The standalone CLI wrote one
line per turn with those keys on every real install (Windows and macOS, 2026-10-15; Ubuntu,
2026-10-15, two lines and 370 bytes); Codex Desktop writes none and keeps its prompts in the global
state's `prompt-history` ([`.codex-global-state.json`](#codex-global-statejson)).

#### `session_index.jsonl`

One `{id, thread_name, updated_at}` line is appended per rename, newest wins, `updated_at` RFC 3339
UTC with nanoseconds
([rollout/src/session_index.rs:24-48](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/rollout/src/session_index.rs#L24-L48)).
Titles only; local shape matches. Codex Desktop also lists the guardian review thread, as
`Guardian review`, and a scheduled task's thread, under the task's name (macOS, build 26.1013.40934,
2026-10-18; [Source and evidence](#source-and-evidence)).

#### `state_5.sqlite` threads

`state_5.sqlite` `threads`: `id`, `rollout_path`, `created_at`, `cwd`, `title`, `git_sha`,
`git_branch`, `git_origin_url` and later `first_user_message`, `preview`
([state/migrations/0001_threads.sql:1-19](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/state/migrations/0001_threads.sql#L1-L19),
[0007_threads_first_user_message.sql:1](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/state/migrations/0007_threads_first_user_message.sql#L1),
[0032_threads_preview.sql:1](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/state/migrations/0032_threads_preview.sql#L1));
`thread_spawn_edges` links parent and child threads
([0021_thread_spawn_edges.sql:1-5](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/state/migrations/0021_thread_spawn_edges.sql#L1-L5)).
A row whose rollout is gone still holds the first prompt. The file name is fixed beside the other
databases
([state/src/sqlite.rs:34-39](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/state/src/sqlite.rs#L34-L39)).
The columns the parser reads ([issue 122](https://github.com/seanthegeek/doubleagent/issues/122)),
each only when the table has it:

- `id`, the thread id: the rollout's `session_meta.id`, so the rows join
  the rollout's on `session_id`.
- `created_at` in epoch seconds and `created_at_ms` in epoch ms
  ([0025_thread_timestamps_millis.sql:1-2](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/state/migrations/0025_thread_timestamps_millis.sql#L1-L2)),
  both the thread's creation time
  ([state/src/runtime/threads.rs:666-670](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/state/src/runtime/threads.rs#L666-L670)).
- `git_origin_url`, the repository's remote URL when the thread's `cwd` was in a git
  repository, read for the `git remote:` row
  ([issue 253](https://github.com/seanthegeek/doubleagent/issues/253)).
- `cwd`, `title`, `git_branch`, `rollout_path`, `archived`, `model` (the
  last turn's, set from each `turn_context`;
  [state/src/extract.rs:122-130](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/state/src/extract.rs#L122-L130)),
  `name` (a rename;
  [0041_threads_name.sql:1](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/state/migrations/0041_threads_name.sql#L1)),
  `source` and `thread_source`
  ([0030_threads_thread_source.sql:1](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/state/migrations/0030_threads_thread_source.sql#L1)).
- `first_user_message`: the first `UserMessage` event's text, trimmed and with the
  `## My request for Codex:` prefix removed, or `[Image]` or `[Audio]` for a prompt with no text
  ([state/src/extract.rs:184-196](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/state/src/extract.rs#L184-L196),
  [protocol/src/protocol.rs:145-150](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/protocol.rs#L145-L150),
  [2614-2635](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/protocol.rs#L2614-L2635)).
  No column records when the prompt was typed: `created_at` is the thread's creation, which the TUI
  does before the first prompt.

Not read: `sandbox_policy` and `approval_mode` (the latest
`turn_context` policy, which the rollout's `permission mode:` rows
already carry), `preview` (the first prompt again, or a thread goal's
objective), the token and recency bookkeeping. What it survives: a
rollout deleted, moved or truncated outside Codex, or excluded by the
collector's size limit. Codex's own thread delete removes the rollout
and the database rows together
([thread-store/src/local/delete_thread.rs:66-92](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/thread-store/src/local/delete_thread.rs#L66-L92)),
so it survives no delete made in Codex. Whether a rollback or a revert
rewrites `first_user_message` was not determined.

Observed on Codex Desktop (Windows, 2026-10-15, 2 rows): the columns `id`, `rollout_path`,
`created_at`, `updated_at`, `source`, `model_provider`, `cwd`, `title`, `sandbox_policy`,
`approval_mode`, `tokens_used`, `has_user_event`, `archived`, `archived_at`, `git_sha`,
`git_branch`, `git_origin_url`, `cli_version`, `first_user_message`, `agent_nickname`,
`agent_role`, `memory_mode`, `model`, `reasoning_effort`, `agent_path`, `created_at_ms`,
`updated_at_ms`, `thread_source`, `preview`, `recency_at`, `recency_at_ms`, `history_mode`,
`name`, `is_pinned`, `thread_section_id`, `section_position`, `section_entered_at_ms`,
`project_id`, `originator`, `daybreak_enabled`, `creator_user_id` and `creator_account_id`.
`approval_mode` held the approval policy: the first thread's a JSON object `{"granular":
{"sandbox_approval": false, "rules": false, "skill_approval": false, "request_permissions": true,
"mcp_elicitations": true}}`, the same object as its `turn_context.approval_policy`, the second's
the string `on-request`; `sandbox_policy` held JSON of 2,389 and 1,725 characters. Beside the tables
the source describes, the database held `projects` (1 row), `project_roots` (1; `project_id`,
`position`, `path`), `project_idempotency_keys`, `thread_sections` (1),
`remote_control_enrollments` (0) and `external_agent_config_imports` (0); on macOS (2026-10-18)
also `guardian_review_feedback`, `thread_spawn_edges` and `thread_read_receipts`. Adding a project
folder wrote a `projects` and a `project_roots` row, and a model switch overwrote `threads.model`
(macOS, 2026-10-18).

#### `thread_history_1.sqlite`

`thread_history_1.sqlite`: `thread_turns`, `thread_items` (`item_json`,
the app-server's camelCase `ThreadItem`), `thread_realtime_items`, keyed
by `rollout_ordinal`
([state/thread_history_migrations/0001_thread_history.sql:1-38](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/state/thread_history_migrations/0001_thread_history.sql#L1-L38)).
A projection of paginated rollouts; local item types were `userMessage`,
`webSearch`, `agentMessage`, the same three items as the rollout's
`item_completed` lines. Nothing found that is not in the rollout.
Codex Desktop on Windows (observed 2026-10-15,
[Source and evidence](#source-and-evidence)) added `reasoning`, `commandExecution` and
`fileChange` items, also all in its rollouts: `thread_items` held 13 rows
(`agentMessage` 4, `commandExecution` 2, `fileChange` 1, `reasoning` 2,
`userMessage` 2, `webSearch` 2) and `thread_turns` 2 rows, both `completed`.
On macOS (2026-10-18) the file was created with the first thread (`thread_items`
2, `thread_turns` 1), a call the user declined is a `commandExecution`
`failed` with exit 1, and the `item_json` keys were `userMessage`
(`clientId`, `content`, `id`, `text`, `text_elements`, `type`),
`agentMessage` (`delivery`, `id`, `memoryCitation`, `phase`, `questions`,
`text`, `type`), `commandExecution` (`aggregatedOutput`, `command`,
`commandActions`, `cwd`, `durationMs`, `exitCode`, `id`, `pluginId`,
`processId`, `scriptPath`, `source`, `status`, `type`) and `fileChange`
(`changes`, `diff`, `id`, `kind`, `path`, `status`, `type`). Record shapes
from source ([issue 122](https://github.com/seanthegeek/doubleagent/issues/122)):

- What is projected: one rollout line at a time, `task_started`,
  `task_complete` and `turn_aborted` into `thread_turns` and each
  `item_completed` into `thread_items`; every other line type
  projects nothing
  ([app-server-protocol/src/protocol/thread_history_projection.rs:21-99](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/app-server-protocol/src/protocol/thread_history_projection.rs#L21-L99)).
  `history_mode` `legacy` rollouts are not projected
  ([line 1-4](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/app-server-protocol/src/protocol/thread_history_projection.rs#L1-L4)).
- `thread_turns`: `status` `inProgress`, `completed`, `interrupted` or `failed`, `started_at` and
  `completed_at` in epoch seconds (the turn events' fields;
  [protocol/src/protocol.rs:2189-2192](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/protocol.rs#L2189-L2192)),
  `duration_ms`, `error_json`; `rollout_end_ordinal` from migration 0003
  ([0003_turn_rollout_positions.sql:1-3](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/state/thread_history_migrations/0003_turn_rollout_positions.sql#L1-L3);
  written at
  [thread-store/src/local/thread_history.rs:316-351](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/thread-store/src/local/thread_history.rs#L316-L351)).
  The two events are the rollout's `task_started` and `task_complete` lines
  ([protocol/src/protocol.rs:1407-1418](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/protocol.rs#L1407-L1418)).
- `thread_items`: `created_at_ms` is the item's `started_at_ms`, or the
  rollout line's `timestamp` when the event has none
  ([thread_history.rs:431-438](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/thread-store/src/local/thread_history.rs#L431-L438),
  [thread_history_materialization.rs:245-252](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/thread-store/src/local/thread_history_materialization.rs#L245-L252));
  `started_at_ms` and `completed_at_ms` from migration 0007
  ([0007_thread_item_lifecycle_timestamps.sql:1-2](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/state/thread_history_migrations/0007_thread_item_lifecycle_timestamps.sql#L1-L2))
  are the `item_completed` event's, when the item started and when it
  completed
  ([core/src/session/mod.rs:2546-2601](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/core/src/session/mod.rs#L2546-L2601)).
  `item_type` is `item_json.type`
  ([0002_thread_items_item_type.sql:1-5](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/state/thread_history_migrations/0002_thread_items_item_type.sql#L1-L5)).
- `item_json` is `ThreadItem`, tagged by `type` in camelCase
  ([app-server-protocol/src/protocol/v2/item.rs:232-429](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/app-server-protocol/src/protocol/v2/item.rs#L232-L429)),
  converted from the rollout's turn item
  ([869-1063](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/app-server-protocol/src/protocol/v2/item.rs#L869-L1063)):
  `userMessage` (`content[]` of `UserInput`, `text` inputs with `text`;
  [v2/turn.rs:425-459](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/app-server-protocol/src/protocol/v2/turn.rs#L425-L459)),
  `agentMessage` (`text`, the content blocks concatenated, `phase`), `reasoning` (`summary[]`,
  `content[]`, strings), `commandExecution` (`command` a display string, `cwd`, `status`,
  `aggregatedOutput`, `exitCode`, `durationMs`), `fileChange` (`changes[]` of `path`, `kind`,
  `diff`;
  [v2/item.rs:1137-1144](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/app-server-protocol/src/protocol/v2/item.rs#L1137-L1144)),
  `mcpToolCall` (`server`, `tool`, `arguments`, `result`, `error`), `dynamicToolCall` (`tool`,
  `arguments`, `contentItems`), `functionCallOutput`, `plan` (`text`), `subAgentActivity`,
  `webSearch` (`id`, `query`, `action`, `results`;
  [ext/items/src/web_search.rs:11-22](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/ext/items/src/web_search.rs#L11-L22)),
  `imageGeneration` (`revisedPrompt`, `status`, `savedPath`, and the image itself in `result`),
  `sleep` (`durationMs`), `hookPrompt`, `collabAgentToolCall`, `imageView`, `enteredReviewMode`,
  `exitedReviewMode`, `contextCompaction`.

The two files join the rollout on the thread id, which is the rollout
rows' `session_id`; the projection has no `cwd`, so its project path is
taken from the `threads` row of the same thread. What it survives is
what `state_5.sqlite` survives; it never holds a legacy rollout's
thread.

#### `logs_2.sqlite`

`logs_2.sqlite` `logs`: Codex's own tracing log, one row per log
event of the app server and the TUI (`ts` and `ts_nanos` epoch seconds
and nanoseconds, `level`, `target`, `feedback_log_body` the formatted
message and fields, `thread_id`, `process_uuid`;
[state/logs_migrations/0002_logs_feedback_log_body.sql:3-16](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/state/logs_migrations/0002_logs_feedback_log_body.sql#L3-L16),
file name at
[state/src/sqlite.rs:34](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/state/src/sqlite.rs#L34)).
Every target is kept at trace level unless listed
([state/src/log_db.rs:61-63](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/state/src/log_db.rs#L61-L63)),
and an event's `thread_id` field fills the `thread_id` column
([log_db.rs:572-579](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/state/src/log_db.rs#L572-L579)).
Rows older than 10 days, or past a 64 MiB budget, are pruned
([state/src/runtime/logs_maintenance.rs:10-11](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/state/src/runtime/logs_maintenance.rs#L10-L11)),
and Codex's thread delete removes a thread's rows
([state/src/runtime/threads.rs:1170-1173](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/state/src/runtime/threads.rs#L1170-L1173),
called from
[thread-store/src/local/delete_thread.rs:139-152](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/thread-store/src/local/delete_thread.rs#L139-L152)).
The one row that marks an upload of a thread: a `/feedback` report
sent to Sentry (an error-reporting service OpenAI uses for Codex
feedback) logs `feedback event uploaded to Sentry` at `INFO` with the
fields `thread_id` (the reported thread's id, or
`no-active-thread-<uuid>` when none was open), `uploaded_attachments`
(a count), `attachments_failed` (a boolean) and `elapsed_ms`
([feedback/src/lib.rs:749-755](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/feedback/src/lib.rs#L749-L755),
[298-300](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/feedback/src/lib.rs#L298-L300)),
under the target `codex_feedback` (the `codex-feedback` crate). It is
written after the report event is accepted and its attachments are
sent, whether or not the user agreed to upload logs, so
`uploaded_attachments` is what tells the two apart. The TUI's
`log/codex-tui.log` does not carry it: its default filter keeps only
`codex_core`, `codex_tui`, `codex_rmcp_client` and
`codex_realtime_webrtc`
([tui/src/startup_orchestration.rs:817-820](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/tui/src/startup_orchestration.rs#L817-L820)).
The parser reads this row and the approval rows below, and no other
row of the database
([Parser plan](#parser-plan)); a body field may
come in any order, and the first logs migration named the body column
`message`. Not seen on a real install: the feedback row is read from
a synthetic fixture only.

Codex Desktop writes the per-call permission decisions here and
nowhere else (macOS, build 26.1013.40934, 2026-10-18;
[Source and evidence](#source-and-evidence)); `ts` was integer epoch seconds. On Windows (2026-10-15) the
`logs` table held 2,156 rows from the app server.

- A user's decision in the approval dialog:
  `codex_app_server::outgoing_message` at `INFO`, `thread_id` empty,
  `<- response: CommandExecutionRequestApproval { request_id:
  Integer(<n>), response: CommandExecutionRequestApprovalResponse {
  decision: Decline } }` (or `Accept`; at 19:04:01Z and 19:05:00Z on the
  install), and in the same second
  `codex_core::session::handlers` at `DEBUG`, `thread_id` set,
  `Submission thread_id=<uuid> submission_id=<uuid>
  operation="exec_approval" ... op=ExecApproval { id: "exec-<uuid>",
  turn_id: Some("<uuid>"), decision: ... }`. A rejected call adds a
  `codex_core::tools::router` `ERROR` row. The `exec-<uuid>` id is
  the `id` of the governed `CommandExecution` or `FileChange` item
  (the rollout's `item_completed` line and the `thread_items` row),
  not the rollout's `call_id` (`call_<hex>`), so the join to the
  call is on the item id. The two rows word one decision differently:
  `denied` and `approved` on the handlers row, `Decline` and `Accept`
  on the response row. The response rows, read in the live database
  at 19:04Z, were no longer in the copy collected at 00:43Z (the
  `logs` table is pruned: it held 1,000 rows before sign-in and 2,944
  at collection, the oldest from 18:31:47Z), while both handlers rows
  were; the handlers row is the one that lasts.
- An automatic decision in the Approve for me mode (auto-review):
  `codex_guardian_v2::async_scorer::approval` at `DEBUG`,
  `decide_request{approval_id="<uuid>"}: reviewing approval
  decision_source="synchronous_assessment" reason=Policy`, and rows
  inside the span `guardian_review{review_id="<uuid>"
  thread_id=<parent uuid> tool_call_id="exec-<uuid>"}`, whose
  `tool_call_id` is the governed item's id; the app-server events
  `item/autoApprovalReview/started`, `guardianWarning` and
  `item/autoApprovalReview/completed`. No
  `CommandExecutionRequestApproval` or `ExecApproval` row is written
  for it; the verdict itself is in the guardian thread's rollout.
- A settings change: `operation="thread_settings"` with
  `op=ThreadSettings { requested_settings: ThreadSettingsOverrides {
  approval_policy: ..., approvals_reviewer: ..., sandbox_policy: ...,
  permission_profile: ... } }`, values seen `Some(OnRequest)`,
  `Some(Never)`, `Some(AutoReview)`, `Some(User)`,
  `Some("managed")`, `Some("disabled")` and `None`. The model is not
  in it; the rollout's `thread_settings_applied` is the model record.
- Every app-server request by method (`thread/start`,
  `thread/resume`, `thread/fork`, `thread/delete`, `fs/remove`,
  `turn/start`, `command/exec`, `fs/readFile`, `gitDiffToRemote`,
  `remoteControl/*` and more), with `app_server.client_name` and
  `app_server.client_version` in its span: `Codex Desktop`
  `26.1013.40934`, `codex_chatgpt_ios_remote` `1.2026.279`,
  `codex_chatgpt_android_remote` `dev`. A paired phone can run a
  command on the host (`command/exec`) and read a file
  (`fs/readFile`) outside any turn; those requests are recorded only
  here.

#### `state_5.sqlite` remote control enrollments

`state_5.sqlite` `remote_control_enrollments`: one row per remote
control enrollment, the link that lets a ChatGPT client drive this
host's app server (`codex remote-control`; `websocket_url`,
`account_id`, `app_server_client_name`, `server_id`, `environment_id`,
`server_name`, `updated_at`, and `remote_control_enabled` from
migration 0037;
[state/migrations/0024_remote_control_enrollments.sql:1-10](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/state/migrations/0024_remote_control_enrollments.sql#L1-L10),
[0037_remote_control_enrollments_enabled.sql:1-2](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/state/migrations/0037_remote_control_enrollments_enabled.sql#L1-L2)).
It names the host's enrollment, not a thread. Empty on the Windows
install of [Source and evidence](#source-and-evidence). On macOS (build 26.1013.40934,
2026-10-18 and 2026-10-19) enabling remote control in the app
(Settings > Connections > Control this Mac or PC) wrote one row:
`websocket_url` the relay's websocket address on the chatgpt.com
host (56 characters, no query string), `account_id`,
`app_server_client_name` `Codex Desktop`, `server_id`
(`srv_e_<hex>`), `environment_id` (`env_e_<hex>`), `server_name`
the Mac's host name (`<name>.local`), `updated_at` and
`remote_control_enabled` 1. Turning "Allow connections" off kept the
row and set `remote_control_enabled` to 0 with a new `updated_at`;
turning it on again reused the row, same ids, with 1. Revoking a
paired phone and pairing a second phone changed nothing in it: the
list of paired devices is held on the server. The row holds no
token ([Credentials](#credentials)). The log rows (targets
`codex_app_server_transport::transport::remote_control::enroll`,
`::websocket` and `::client_tracker`, and the app-server requests):
`remoteControl/enable`, `persisted remote control enrollment: ...`,
`connected to app-server remote control websocket`, status changes
between `Connecting`, `Connected` and `Disabled` (with
`connection_end_reason=Disabled`), `remoteControl/disable`,
`remoteControl/pairing/start`, `remoteControl/client/list` (polled
while the Connections page is open), `remoteControl/client/revoke`
(no device name in the row), the event
`remoteControl/status/changed` when a phone connects, and
`client_tracker` `... connection closed` when it leaves. The first
pairing also wrote two top-level keys of `.codex-global-state.json`
([`.codex-global-state.json`](#codex-global-statejson)). **Parser reads** the row
([Parser plan](#parser-plan)).

#### `sqlite/codex-dev.db`

`sqlite/codex-dev.db`: written only by the ChatGPT desktop app (observed on Windows, ChatGPT
26.1007.9446.0, 2026-10-15, and on macOS, 26.1007.71326 and 26.1013.40934; not in the Codex source
this document cites, and the app's code is not in the repository, see
[Sync, relay and origin rows](#sync-relay-and-origin-rows)). It is the app's own thread catalog
and automation store. The collector gathers it through the `codex-cli|.codex` catalog line, under
`codex-cli`.

- The thread catalog: `local_thread_catalog`, one row per thread the app lists, with
  `local_thread_catalog_hosts`, `_sync_state`, `_metadata` and two scan tables beside it. The local
  Codex threads are rows with `source_kind` `vscode`, `host_id` `local`, `thread_source` `user`
  and the thread's `cwd`, `project_id` null even for a thread in a project (macOS, 2026-10-18); the
  account's ChatGPT conversations are rows with `source_kind` `chatgpt` and the host id
  `chatgpt:<workspace uuid>:<user id>` (the user id is the 29-character ChatGPT user id, which
  begins `user-`). Their columns, values and the other catalog tables are in
  [Record schema in `chatgpt-desktop.md`](chatgpt-desktop.md#record-schema) and
  [Other stores in `chatgpt-desktop.md`](chatgpt-desktop.md#other-stores). Each Codex-mode thread
  added a `vscode` row, and the app's delete removed the deleted thread's row. The
  `chatgpt-desktop` parser reads the `chatgpt` rows (since analyzer 0.17.0). The `codex-cli`
  parser does not read the file: the automation is read from `automation.toml`, and a run would be
  in `automation_runs`, which no real install has shown filled.
- `automations` (`id`, `name`, `prompt`, `status`, `next_run_at`, `last_run_at`, `cwds`, `rrule`,
  `model`, `reasoning_effort`, `target_type`, `project_id`, `kind`, `target_thread_id`,
  `execution_environment`, `account_id`, `user_id` and more) and `automation_runs` (`thread_id`,
  `automation_id`, `status`, `thread_title`, `source_cwd`, `inbox_title`, `inbox_summary`,
  `archived_user_message`, `archived_assistant_message` and more), with `inbox_items`: the
  scheduled-task store, both empty after a first automation failed to save on Windows. The rows of
  the saved automations are in [Automations](#automations).
- `live_visualization_suggestions`, `local_app_server_feature_enablement` and
  `thread_timeline_ledger` (`host_id`, `thread_id`, `sequence`, `record_id`, `payload_json`),
  present on Windows and on macOS; on macOS (2026-10-18) they held 0, 1 and 0 rows, the ledger
  empty through every step.
- `codex_schema_migrations` (33 rows with `app_version`, `build_flavor` and `build_commit`), which
  carries the app version (`app_version` `26.1007.71326`, `build_flavor` `prod` on Windows).

The chat's text is not in it. Neither `conversation_origin` nor `chatgpt_async_status` appears in
the Codex source; `conversation_origin` `tpp` shares its token with the package's
`.codex-tpp-local-executor-installation-id` file name, and what it denotes is not determined
([Record schema in `chatgpt-desktop.md`](chatgpt-desktop.md#record-schema)).

#### Automations

`automations/<slug>/automation.toml`: one directory per automation of the ChatGPT desktop app,
named by a slug of the name the user typed (the kebab-case of the task's name on macOS; observed on
Windows, 2026-10-15; not in the Codex source this document cites), beside
`automations/.run-jitter-salt` (37 bytes, base64-class text). The app calls this kind of task a
heartbeat (`kind` `heartbeat`): each run ends with a `<heartbeat>` block in which the model decides
whether to notify the user. The file is flat TOML (383 bytes on Windows, 356 bytes on macOS
2026-10-18): `version` integer (1), `id` string (the slug), `kind` string (`heartbeat`), `name`
string, `prompt` string (what the automation sends), `status` string (`ACTIVE`), `rrule` string
(an iCalendar recurrence rule, the scheduling syntax of RFC 5545), `target_thread_id` string (the
thread the automation runs in), `created_at` and `updated_at` integers (epoch milliseconds).
**Parser reads** ([issue 119](https://github.com/seanthegeek/doubleagent/issues/119)). The `rrule`
values seen:
`RRULE:FREQ=WEEKLY;BYHOUR=9;BYMINUTE=0;BYDAY=MO,TU,WE,TH,FR` (Windows, a weekday schedule),
`FREQ=WEEKLY;BYHOUR=9;BYMINUTE=0;BYDAY=MO` (macOS, 2026-10-15) and
`RRULE:FREQ=WEEKLY;BYHOUR=19;BYMINUTE=45;BYDAY=SU,MO,TU,WE,TH,FR,SA` for a task the user set to run
daily (macOS, 2026-10-18; the hour is local time). The user reported that scheduled tasks are not
tied to a project and run only on an interval (macOS, build 26.1013.40934).

Creating the automation creates its target thread at once, with a rollout, a `thread-writer-locks`
lock, a `visualizations` directory and a `session_index.jsonl` line under the task's name, its `cwd`
in a new `~/Documents/Codex/<date>/new-chat/` output directory. On Windows that thread
(`rollout-2026-10-15T11-25-49-<uuid>.jsonl`, 83,579 bytes, 15:25:50Z, `cwd`
`~\Documents\Codex\2026-10-15\new-chat`) had `thread_source` `user`, `source` `vscode`, `originator`
`Codex Desktop`, approval `on-request` and the model `gpt-5.6-terra`, which the app chose, and held
one user turn (the automation's prompt), one assistant message and no tool call, `task_started` to
`task_complete` in 9,969 ms. On macOS (2026-10-18) its `session_meta` said nothing about an
automation (`source` `vscode`, `thread_source` `user`, `cwd`
`~/Documents/Codex/2026-10-18/new-chat`); only `turn_attribution.turn_trigger` marks the turns:
`scheduled_task_setup` for the setup turn (the agent acknowledges the schedule, no tool call),
`automation_heartbeat_run_now` for Run now and `automation_heartbeat_scheduled` for the scheduled
run, whose user messages have no `client_id`. Both runs reported into that thread, the file's
`target_thread_id`: the scheduled run's `task_complete.last_agent_message` ends in a `<heartbeat>`
block with `<decision>DONT_NOTIFY</decision>` and a `<message>`, and the Run now report in a
`<heartbeat>` block with an `<automation_id>`. The vendor page [Scheduled tasks | ChatGPT
Learn](https://learn.chatgpt.com/docs/automations) (read 2026-10-18) says "Standalone scheduled
tasks start a new chat for each scheduled run and report results in **Scheduled**"; neither run here
started a new chat.

The same automation is a `codex-dev.db` `automations` row. On Windows: `status` `ACTIVE`, `kind`
`heartbeat`, `rrule` as in the file, `next_run_at` and `next_run_nominal_at` set (epoch ms),
`last_run_at` null, `cwds` an empty JSON array, `target_thread_id` set, `auto_archive` 0 and every
other column null (`model`, `reasoning_effort`, `target_type`, `project_id`,
`execution_environment`, `local_environment_config_path`, `plugin_template_id`,
`notification_policy`, `account_id`, `user_id`, `installation_id`, `legacy_automation_id`). On
macOS (2026-10-18): empty strings for `target_type`, `execution_environment`,
`notification_policy`, `model` and `reasoning_effort`, `cwds` `[]`, `project_id` and
`legacy_automation_id` null, `account_id`, `user_id` and `installation_id` set, `last_run_at` the
last run, `next_run_nominal_at` the scheduled minute and `next_run_at` that minute moved by a
jitter seeded from `.run-jitter-salt`. `automation_runs` (one row per run, linking `thread_id` and
`automation_id`) and `inbox_items` stayed empty on every install: before any run on Windows and on
macOS 2026-10-15, and after a Run now and a scheduled run whose decision was `DONT_NOTIFY` on
macOS 2026-10-18. A run that decides to notify was not observed.

#### Ambient suggestions

`ambient-suggestions/<40 hex>/ambient-suggestions.json`: the desktop app's suggestions for a
project root, one directory per root (observed on Windows, 2026-10-15, 139 bytes, and on macOS,
2026-10-18, a second directory when a project was added): `currentSuggestionIds` (empty list),
`generatedAtMs` number, `projectRoot` absolute path, `suggestions` (empty list). Governed by
`desktop.ambient-suggestions-enabled` in `config.toml`. Not read; a file holding suggestions has
not been observed.

#### `.codex-global-state.json`

`.codex-global-state.json` (and its `.bak`): the ChatGPT desktop app's state, one JSON object
(observed on Windows, 2026-10-15, and on macOS, build 26.1013.40934, 2026-10-18 and 2026-10-19;
not in the Codex source this document cites). The key `electron-persisted-atom-state` holds a JSON
object, not a string, of the app's interface state; it and the top level name threads by id. The
keys that record use of a thread:

- `prompt-history`: a `global` list and one list per thread id, each entry `{markdown, document}`
  (the prompt as Markdown and as an editor document tree with `type` `doc`). It is the prompt
  history of Codex Desktop, which writes no `history.jsonl`.
- `thread-descriptions-v1` (`{<thread id>: string}`), `thread-user-activity-times-v1` (keyed
  `["local","<thread id>"]`, a number, the last activity in epoch ms) and
  `client-thread-bindings-v1` (`client-new-thread:<uuid>` to a thread id; on Windows also a
  `local-chatgpt:<uuid>` key, binding a ChatGPT chat).
- `heartbeat-thread-permissions-by-id`: per thread id `activePermissionProfile` (`{id, extends}` or
  null), `approvalPolicy` (a string or the granular object), `approvalsReviewer`, `sandboxPolicy`
  (`type`, `writableRoots`, `networkAccess`, `excludeTmpdirEnvVar`, `excludeSlashTmp`): each
  thread's standing permission state, in camelCase (`workspaceWrite` here, `workspace-write` in the
  rollout). A one-off approval leaves it unchanged (the Demo thread kept one writable root after
  the approved write to the Desktop), and Full access was never stored in it.
- `agent-mode-by-host-id` (`{local: "guardian-approvals"}`),
  `permission-selection-by-host-id:local` (`{kind: "agent-mode", agentMode}`) and
  `preferred-non-full-access-agent-mode-by-host-id`: the permission mode the app starts threads in
  (the scheduled task's new thread started in auto-review). Only `guardian-approvals` was observed.
  The mode is kept here, not in `config.toml`.
- `composer-recent-model-configurations-v1` (`[{model, reasoningEffort, serviceTier}]`, most
  recent first) and `composer-model-picker-selection-mode-v1`.
- `codex-writing-block-deleted-thread-v1:<thread id>`, `true`: written when the thread is deleted
  in the app, absent for a live thread ([What survives a delete](#what-survives-a-delete)).
- `chatgpt-conversation-resume-tokens-v1` (Windows): keyed by a JSON array of the account's user id
  and a ChatGPT conversation id, each value `{expiresAtMs, promptMessageId, token, turnTraceId}`,
  the token 419 characters. No such key had appeared on macOS by the end of sign-in.
- Top level: `projectless-thread-ids` (a list of thread ids),
  `thread-projectless-output-directories` and `thread-workspace-root-hints`
  (`{<thread id>: path}`), `thread-project-assignments`, `selected-project`,
  `app-server-projects-migration-by-host`, `desktop-first-seen-at-ms`, and, from the first
  remote-control pairing, `electron-local-remote-control-environment-id` (a string) and
  `codex-mobile-has-connected-device` (a boolean).
- Projects and files: `local-projects` (`{<uuid>: {createdAt, id, name, rootPaths, updatedAt}}`),
  `recent-workspace-files-v3` (one entry per file a thread wrote, keyed
  `["local","client-new-thread:<uuid>","<path>"]`; on Windows the last element spells a Windows
  path as `win32:/mnt/c/...`) and `composer-prompt-drafts-v2` (an unsent draft per composer
  surface).
- `external-agent-import-discovery:claude-code`, `:claude-cowork` and `:cursor` (`detectedAtMs`,
  `requestKey`, `result` with `items` and `unsupportedProjects`), written at sign-in: the app looks
  for Claude Code, Claude Cowork and Cursor state to import (empty results on both installs).
- Other keys seen on Windows: the import-discovery keys in the form
  `external-agent-import-discovery:claude-code,claude-cowork` and `...:cursor`,
  `mcp-global-entrypoints-v2`,
  `electron:onboarding-welcome-v2-role-state` (`completedConversationalOnboardingTaskId`, `roles`,
  `workMode`), `cloud-pet-artwork-cache` (`accountId`, `userId`), and, after a Plus subscription and
  an automation, `chatgpt-work-model-settings-v1`, `fast-mode-personalized-estimate`,
  `seen-model-upgrade-list` and a `thread-reference-capability:<uuid>` per new thread.

The ChatGPT account's user id appears inside key names: the resume tokens' keys,
`chatgpt-work-models-v1/<user id>/<workspace uuid>` and the keys of
`new-task-execution-by-account-project` (`{kind: "cloud"}`). The file outlives a deleted thread
with its prompts ([What survives a delete](#what-survives-a-delete)). The collector flags it
`secret: true` and collects it by default. **Parser reads**, as a secondary source
([Parser plan](#parser-plan)).

#### Thread locks and shell snapshots

`thread-writer-locks/` (`.coordination.lock` and one `<thread id>.lock` per open thread, 0 bytes)
and `shell_snapshots/<thread id>.<unix ns>.sh` (macOS, build 26.1013.40934, 2026-10-18): working
files of the running app. The locks of threads that were not open were gone after a quit and
relaunch. At the resume (21:46:13Z) and when the scheduled task was created (23:18:39Z to
23:18:41Z) the app prewarmed a guardian review session, a reviewer warmed up in advance
(`startup_prewarm` with the model `codex-auto-review` in `logs_2.sqlite`), whose id got a 0-byte
lock and no rollout, `state_5.sqlite` `threads` row, `session_index.jsonl` line or
`thread_history_1.sqlite` rows: a lock is not a session. A shell snapshot (2,135 to 2,299 bytes, 43
lines: a `# Snapshot file` header, 29 `export NAME=...` lines, 2 aliases, `unalias -a`, `setopt`
lines), named by the thread id and a Unix time in nanoseconds, replays the user's exported
login-shell environment for the sandbox. One is written at each resume and turn of a thread while
older ones are removed: the two most recent per thread remained at the end of the round, and a
quit removed every snapshot of threads not reopened. The exported names on the clean lab VM
included `BROWSER_USE_ACCESS_VERIFYING_IDENTITY`, `CODEX_TASK_WORKSPACE_VERIFYING_IDENTITY`,
`CODEX_APP_TOOLS_PIPE_PATH`, `SSH_AUTH_SOCK`, `HOME`, `USER` and `TMPDIR`, and no key-like name; on
a workstation an API key the login shell exports is copied here, and the collector flags the files
`secret: true` ([Credentials](#credentials)). The parser reads neither.

#### Standing grants and approval records

From source at
[`4e7b9e2`](https://github.com/openai/codex/commit/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857); not
seen on a real install unless a bullet says so. All are collected by the `codex-cli|.codex` line;
the rows the parser makes of them are in [Permission rows](#permission-rows).

- `rules/default.rules`: when the user answers an approval prompt
  with "approve and remember" (`ReviewDecision::ApprovedExecpolicyAmendment`,
  [protocol/src/protocol.rs:4269-4304](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/protocol/src/protocol.rs#L4269-L4304)),
  Codex appends one Starlark line (Starlark is the Python-like
  configuration language the exec policy is written in)
  `prefix_rule(pattern=["<token>", ...], decision="allow")` to
  `<CODEX_HOME>/rules/default.rules`, the exec policy file
  ([core/src/exec_policy.rs:54-56](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/core/src/exec_policy.rs#L54-L56),
  [core/src/session/handlers.rs:181-193](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/core/src/session/handlers.rs#L181-L193),
  [core/src/exec_policy.rs:466-490](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/core/src/exec_policy.rs#L466-L490),
  [874-876](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/core/src/exec_policy.rs#L874-L876),
  [execpolicy/src/amend.rs:65-80](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/execpolicy/src/amend.rs#L65-L80)),
  and a network allow or deny answer appends
  `network_rule(host=..., protocol=..., decision="allow"|"deny",
  justification=...)`
  ([amend.rs:85-124](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/execpolicy/src/amend.rs#L85-L124)).
  A line carries no time and no thread; the file's modification time
  is the last grant. Every other `*.rules` file under `rules/` is
  loaded too
  ([exec_policy.rs:681](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/core/src/exec_policy.rs#L681)).
  The file holds no credential. The same `rules/` directory in a project's `.codex/` is a project
  file ([Project-local files](#project-local-files)), loaded as a policy layer like the home's
  ([exec_policy.rs:670-684](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/core/src/exec_policy.rs#L670-L684)).
- `config.toml`: the standing policy keys `approval_policy`,
  `approvals_reviewer`, `sandbox_mode` and `default_permissions`
  ([config/src/config_toml.rs:195](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/config/src/config_toml.rs#L195),
  [200](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/config/src/config_toml.rs#L200),
  [224](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/config/src/config_toml.rs#L224),
  [239](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/config/src/config_toml.rs#L239));
  `[projects."<path>"] trust_level`, `trusted` or `untrusted`
  ([config/src/config_toml.rs:473](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/config/src/config_toml.rs#L473),
  [core/src/config/mod.rs:2464](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/core/src/config/mod.rs#L2464),
  [protocol/src/config_types.rs:642-645](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/protocol/src/config_types.rs#L642-L645)),
  the one seen on a real install: adding a project folder in the desktop app wrote `trusted`
  (macOS, ChatGPT 26.1013.40934, 2026-10-18); and, for an MCP tool call the user approved for good
  (`ApprovedMcpPolicyAmendment`), `approval_mode = "approve"` under
  `mcp_servers.<server>.tools.<tool>`,
  `plugins.<plugin>.mcp_servers.<server>.tools.<tool>` or, for a
  ChatGPT app connector, `apps.<connector>.tools.<tool>`, in the project's own `.codex/config.toml`
  when the server is configured there
  ([mcp_tool_call.rs:2460-2470](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/core/src/mcp_tool_call.rs#L2460-L2470),
  [2507-2530](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/core/src/mcp_tool_call.rs#L2507-L2530))
  ([core/src/mcp_tool_call.rs:2394-2411](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/core/src/mcp_tool_call.rs#L2394-L2411),
  [2428-2457](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/core/src/mcp_tool_call.rs#L2428-L2457),
  [2472-2490](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/core/src/mcp_tool_call.rs#L2472-L2490)).
  "Approve for this session" (`ApprovedForSession`) is kept in memory
  only and leaves no file.
- `state_5.sqlite` `guardian_review_feedback` (`id`, `thread_id`,
  `record` BLOB; migration 0060, not at
  [`3e23877`](https://github.com/openai/codex/commit/3e238776e857eccd3bde6bff3026e2e9798f6524),
  [state/migrations/0060_guardian_review_feedback.sql:1-5](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/state/migrations/0060_guardian_review_feedback.sql#L1-L5)):
  one row per automatic review (the desktop app's Approve for me
  reviewer) that did not allow the action: `record` is a JSON object
  `reviewed_thread_id`, `reviewed_turn_id`, `target_item_id`,
  `reviewer_thread_id`, `model`, `status` (`denied`,
  `invalid_decision`, `missing_decision`, `failed`, `timed_out`,
  `aborted`), `decision` (the reviewer's verdict text), `action` (the
  reviewed command or patch), `instructions`, `history` (the
  reviewer's own response items) and `context_omitted`
  ([ext/guardian-reviewer/src/feedback.rs:17-30](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/ext/guardian-reviewer/src/feedback.rs#L17-L30),
  [57-85](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/ext/guardian-reviewer/src/feedback.rs#L57-L85),
  [core/src/guardian/feedback.rs:6-44](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/core/src/guardian/feedback.rs#L6-L44),
  [state/src/runtime/guardian_feedback.rs:33-50](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/state/src/runtime/guardian_feedback.rs#L33-L50)).
  An allowed review writes no row, and nothing is written when
  feedback is turned off or the thread is ephemeral. The `id` is a UUIDv7 made when the record is
  stored, so its first 48 bits are the capture time in epoch milliseconds, and the table keeps at
  most 8 records per reviewed thread and 64 in all
  ([guardian_feedback.rs:9-28](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/state/src/runtime/guardian_feedback.rs#L9-L28)). The row outlives
  the reviewer thread; its `thread_id` references `threads(id)` with
  `ON DELETE CASCADE`, so it is meant to go with the reviewed thread. The table was present, with no
  row read, on the macOS install of 2026-10-18 ([Source and evidence](#source-and-evidence)).
- Log lines of the other answers: beside
  `op=ExecApproval`, the core's `Submission` debug line
  ([core/src/session/handlers.rs:487-495](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/core/src/session/handlers.rs#L487-L495))
  prints `PatchApproval { id: "<id>", decision: <word> }` for an `apply_patch`
  approval, the word as for `ExecApproval` ([Format versions](#format-versions)) and the id the
  governed `FileChange` item's, which the app server submits
  ([app-server/src/bespoke_event_handling.rs:1990-1993](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/app-server/src/bespoke_event_handling.rs#L1990-L1993)),
  and `RequestPermissionsResponse { id, scope,
  strict_auto_review }` for a permissions request
  ([protocol/src/protocol.rs:769-776](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/protocol/src/protocol.rs#L769-L776),
  [803](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/protocol/src/protocol.rs#L803)).
- Started by another agent with approvals bypassed (Agent Zero source at
  [`ff2a5b9`](https://github.com/agent0ai/agent-zero/commit/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a)):
  Agent Zero's Orchestrator plugin, which tells that agent how to drive other coding CLIs on the
  host or in its container, defaults Codex to `bypass_sandbox: true` and runs
  `codex exec --skip-git-repo-check --dangerously-bypass-approvals-and-sandbox`
  ([`plugins/_orchestrator/default_config.yaml:7-37`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/plugins/_orchestrator/default_config.yaml#L7-L37),
  [`plugins/_orchestrator/README.md:86`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/plugins/_orchestrator/README.md#L86)).
  Such a thread is an ordinary rollout in Codex's own store; the plugin's permission settings are in
  [Parser plan in `agent-zero.md`](agent-zero.md#parser-plan).

#### Other past-use stores

From source at
[`4e7b9e2`](https://github.com/openai/codex/commit/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857); not
seen on a real install beyond the `goals_1.sqlite` and `memories_1.sqlite` files listed in
[macOS, ChatGPT 26.1013.40934](#macos-chatgpt-26101340934). Each is derived from rollouts; which
the parser reads is in [Past-use rows and secondary sources](#past-use-rows-and-secondary-sources).

- Memories, when the `memories` feature is on (off by default,
  [features/src/lib.rs:1303-1305](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/features/src/lib.rs#L1303-L1305)):
  `memories_1.sqlite` (and `memories_v2_1.sqlite`) `stage1_outputs`,
  one row per thread with `thread_id`, `raw_memory`,
  `rollout_summary`, `rollout_slug` and `source_updated_at`
  ([state/memory_migrations/0001_memories.sql:1-12](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/state/memory_migrations/0001_memories.sql#L1-L12),
  [state/src/sqlite.rs:114-119](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/state/src/sqlite.rs#L114-L119)),
  and the consolidated files under `memories/` in the Codex home
  ([memories/write/src/control.rs:4-8](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/memories/write/src/control.rs#L4-L8)):
  `rollout_summaries/<stem>.md`, each headed `thread_id:`,
  `updated_at:` (the thread's last update the summary covers, RFC 3339), `rollout_path:`, `cwd:`
  and, when the thread has one, `git_branch:` lines, then a blank line and a model-written summary
  of the thread, `raw_memories.md`
  with the same header per thread, `MEMORY.md` and
  `memory_summary.md`
  ([memories/write/src/lib.rs:39-40](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/memories/write/src/lib.rs#L39-L40),
  [storage.rs:110-136](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/memories/write/src/storage.rs#L110-L136),
  [workspace.rs:87](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/memories/write/src/workspace.rs#L87),
  [101](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/memories/write/src/workspace.rs#L101)).
  A thread delete removes its `stage1_outputs` row
  ([state/src/runtime/memory_versions.rs:39-51](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/state/src/runtime/memory_versions.rs#L39-L51)),
  and its summary file goes only when the next consolidation prunes
  the files it no longer keeps
  ([storage.rs:80-108](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/memories/write/src/storage.rs#L80-L108)),
  so until then the summary names a thread whose rollout is gone.
  Joins on `thread_id`, `cwd` and `rollout_path`.
- Thread goals: `goals_1.sqlite` `thread_goals` (`thread_id`,
  `objective`, `status`, `token_budget`, `created_at_ms`,
  `updated_at_ms`;
  [state/goals_migrations/0001_thread_goals.sql:1-17](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/state/goals_migrations/0001_thread_goals.sql#L1-L17)),
  the objective the user set for a thread, also persisted in the
  rollout as `event_msg` `thread_goal_updated`, whose keys are camelCase (`threadId`, `turnId`,
  and `goal` with `threadId`, `objective`, `status`, `tokenBudget`, `tokensUsed`,
  `timeUsedSeconds`, `createdAt`, `updatedAt`; `status` one of `active`, `paused`, `blocked`,
  `usageLimited`, `budgetLimited`, `complete`;
  [rollout/src/policy.rs:131-139](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/rollout/src/policy.rs#L131-L139),
  [protocol/src/protocol.rs:4206-4257](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/protocol/src/protocol.rs#L4206-L4257)).
  Removed with the thread.
- `db-backups/sqlite-<timestamp>-<n>/`: a database Codex found
  corrupt, renamed aside with its sidecars before a fresh one is
  built
  ([state/src/runtime/recovery.rs:16](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/state/src/runtime/recovery.rs#L16),
  [181-200](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/state/src/runtime/recovery.rs#L181-L200),
  [241-250](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/state/src/runtime/recovery.rs#L241-L250)).
  Only the corrupt database and its sidecars move, so the folder holds one of `state_5.sqlite`,
  `logs_2.sqlite`, `thread_history_1.sqlite` or another runtime database. A backed-up
  `state_5.sqlite` keeps the `threads` rows of its time, deleted threads included.

### Format versions

- **History mode.** `session_meta.history_mode` (`legacy` default, `paginated`;
  [protocol.rs:775-782](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/protocol.rs#L775-L782)).
  The app server starts non-ephemeral threads paginated when the store supports it
  ([app-server/src/request_processors/thread_processor.rs:1462-1465](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/app-server/src/request_processors/thread_processor.rs#L1462-L1465));
  the local TUI thread was paginated. Paginated files add `ordinal` and `item_completed` items and
  drop the legacy `user_message`/`agent_message` events. A background legacy-to-paginated rewrite
  exists behind `background_paginated_rollout_migration`, off by default
  ([features/src/lib.rs:1224-1229](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/features/src/lib.rs#L1224-L1229),
  [thread-store/src/local/rollout_migration.rs:1-9](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/thread-store/src/local/rollout_migration.rs#L1-L9)).
- **Compression and archive.** `.jsonl.zst` and `archived_sessions/`
  ([Transcript stores](#transcript-stores)); Codex materialises a compressed file back to `.jsonl`
  before appending to it
  ([compression.rs:104-105](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/rollout/src/compression.rs#L104-L105)),
  so either form may be present.
- **Flat layout.** Older releases wrote `sessions/rollout-<time>-<uuid>.jsonl` without date
  directories (the example in
  [recorder.rs:82](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/rollout/src/recorder.rs#L82));
  the current lister walks only `YYYY/MM/DD`
  ([list.rs:900-920](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/rollout/src/list.rs#L900-L920)).
- **Renames kept for compatibility.** `session_meta` without `session_id`,
  `agent_type` for `agent_role`, `turn_started`/`turn_complete` aliases,
  `compaction_summary` for `compaction`; `TurnContextItem.summary` is
  written only for older readers.
- **Not determined at this commit:** whether pre-envelope rollouts (no
  `type`/`payload` wrapper) exist on disk from very early releases; the
  current reader rejects lines without `timestamp`.

- **Drift from
  [`3e23877`](https://github.com/openai/codex/commit/3e238776e857eccd3bde6bff3026e2e9798f6524) to
  [`4e7b9e2`](https://github.com/openai/codex/commit/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857).**
  Unchanged: the line envelope and `RolloutItem` set
  ([history/src/lib.rs:215-231](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/history/src/lib.rs#L215-L231)),
  `SessionMeta`
  ([protocol/src/protocol.rs:3256-3331](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/protocol/src/protocol.rs#L3256-L3331)),
  the `ResponseItem` variants, the database file names
  ([state/src/sqlite.rs:41-46](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/state/src/sqlite.rs#L41-L46)),
  `sessions/` and `archived_sessions/`
  ([rollout/src/lib.rs:86-87](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/rollout/src/lib.rs#L86-L87))
  and the `/feedback` marker message
  ([feedback/src/lib.rs:755](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/feedback/src/lib.rs#L755)).
  Changed:
  - `TurnContextItem` dropped `network` (the allowed and denied domain
    lists), `workspace_roots`, `current_date`, `timezone`,
    `personality` and `multi_agent_mode` (present at
    [protocol.rs:3314-3346 at `3e23877`](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/protocol.rs#L3314-L3346))
    and gained `root_turn_id`, `disabled_plugin_ids`,
    `file_system_sandbox_policy`, `comp_hash`, `realtime_active` and
    `cyber_access_program`; `summary` is optional
    ([protocol.rs:3428-3468](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/protocol/src/protocol.rs#L3428-L3468)).
    The parser's `permission mode:` row prints `network=-` for a
    rollout of this version. The network policy is now the `network` field of a `managed` or
    `external` `permission_profile`
    ([protocol/src/models.rs:424-440](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/protocol/src/models.rs#L424-L440)),
    which the composite already prints inside `profile=`.
  - `task_started` gained `turn_attribution` (`turn_id`,
    `turn_trigger`, `parent_turn_id`, `initiating_agent_path`,
    `root_turn_id`) and `root_turn_id`
    ([protocol.rs:2302-2325](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/protocol/src/protocol.rs#L2302-L2325),
    [protocol/src/turn_input.rs:80-86](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/protocol/src/turn_input.rs#L80-L86)):
    the field the desktop app's 0.163.0-alpha rollouts already carried
    and the parser reads.
  - The `op=ExecApproval` debug line now has its own format,
    `ExecApproval { id, turn_id, decision: <word> }`, the word one of
    `approved`, `approved_with_amendment`, `approved_for_session`,
    `approved_mcp_policy_amendment`,
    `approved_with_network_policy_allow`,
    `denied_with_network_policy_deny`, `denied`, `timed_out`, `abort`
    ([protocol.rs:759-767](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/protocol/src/protocol.rs#L759-L767),
    [4323-4339](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/protocol/src/protocol.rs#L4323-L4339)),
    where the derived format printed the enum (`Denied { rejection:
    ... }`). The parser's pattern takes the first word either way.
  - `event_msg` `elicitation_abandoned` is new and transient
    ([rollout/src/policy.rs:199](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/rollout/src/policy.rs#L199)).
  - New tables: `guardian_review_feedback` ([Other stores](#other-stores)),
    `thread_read_receipts`
    ([state/migrations/0061_thread_read_state.sql:1-8](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/state/migrations/0061_thread_read_state.sql#L1-L8)),
    and `thread_turns.root_turn_id`
    ([state/thread_history_migrations/0008_thread_turn_root.sql:1](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/state/thread_history_migrations/0008_thread_turn_root.sql#L1)).
    No column the parser selects was removed.

### Secrets

The credential files under `~/.codex` (`auth.json`, `.credentials.json`, `secrets/`,
`.sandbox-secrets/`) are listed in [Credentials](#credentials). Inside
the stores this document describes:

- `session_meta.payload.creator_account_id` and `creator_user_id`, and the `state_5.sqlite`
  `threads` columns of the same names, name the ChatGPT account and user
  ([Joins and timestamps](#joins-and-timestamps)); `token_count.rate_limits` names the account's
  plan ([`event_msg`](#event_msg)).
- `.codex-global-state.json`: `chatgpt-conversation-resume-tokens-v1`, the ChatGPT conversation
  resume tokens (a credential); the user's prompts in `prompt-history`; and the account's ids, in
  values (`cloud-pet-artwork-cache` `accountId` and `userId`) and inside key names (the resume
  tokens' keys, `chatgpt-work-models-v1/<user id>/<workspace uuid>` and the keys of
  `new-task-execution-by-account-project`)
  ([`.codex-global-state.json`](#codex-global-statejson)).
- `config.toml` can hold an MCP server's `env` and headers and a provider's bearer token
  ([Credentials](#credentials)); the parser prints only the standing permission keys of
  [Permission rows](#permission-rows) from it. `guardian_review_feedback` `record` `action` is the
  reviewed command, which can carry a pasted key, as tool arguments can.
- `shell_snapshots/<thread id>.<unix ns>.sh` and remote control's server token are in
  [Credentials](#credentials); `state_5.sqlite` `remote_control_enrollments` holds no token, only
  the account id and the host name.

### Parser plan

`CodexParser` wants
`^\.codex/(?:archived_)?sessions/(?:.*/)?rollout-[^/]*\.jsonl(?:\.zst)?$`,
`.codex/history.jsonl`, the stores of [Other stores](#other-stores) the table below names, and,
source-derived, `.codex/config.toml`, `^\.codex/rules/[^/]+\.rules$`,
`^\.codex/memories/rollout_summaries/[^/]+\.md$` and
`^\.codex/db-backups/[^/]+/state_5\.sqlite$`. Until `session_meta` is read, the session id is
the thread id in the file name: the last UUID, or the first of a reverted
rollout's `<thread id>_<rollout id>`. The parser reads the whole file before
emitting rows, so that an `item_completed` line can be checked against every
`response_item` id and `call_id` in the file.

| Column | Source |
| --- | --- |
| timestamp_utc | line `timestamp`; `session_meta.payload.timestamp` for the start, subagent and fork rows; `item_completed` `started_at_ms` / `completed_at_ms` for the rows built from a turn item; history `ts` (s) |
| session_id | the file's first `session_meta.payload.id`, else its `session_id`, else the file-name thread id; a later `session_meta` never changes it; history `session_id` |
| project_path | first `session_meta.payload.cwd`, replaced by each `turn_context.payload.cwd` |
| git_branch | first `session_meta.payload.git.branch` |
| model | `turn_context.payload.model` (empty before the first `turn_context`) |
| turn_type | coverage table below |
| tool_name | `name`; `shell` for `local_shell_call` and `CommandExecution`, `web_search` for `web_search_call` and web-search turn items, `tool_search`, `<server>.<tool>` for `McpToolCall`, `apply_patch` for `FileChange` |
| tool_use_id | `call_id` (call falls back to `id`); a turn item's `id` |
| text | message: `content[].text` joined, prefixed `<role>: ` for developer; user-role context `context: <kind>: <text>`; tool_use: `arguments` or `input`, `action.command` joined, `action.query`, `arguments` JSON; tool_result: `output` string or `[].text` joined, prefixed `[exit N]` when the matching `CommandExecution` item has a non-zero `exit_code`; thinking: `summary[].text`, else `content[].text` |

| Record | Row |
| --- | --- |
| first `session_meta` | `system` ("session start: originator version provider cwd", then ` account=<creator_account_id> user=<creator_user_id>` with only the keys present, [issue 118](https://github.com/seanthegeek/doubleagent/issues/118)); plus, right after it and at the same time, `system` `git remote: <git.repository_url>` when the [`session_meta`](#session_meta) records one ([issue 253](https://github.com/seanthegeek/doubleagent/issues/253)); plus `system` `subagent <id> of <parent> [agent_path= agent_role= agent_nickname=]` when `parent_thread_id`, `source.subagent.thread_spawn.parent_thread_id`, or a `session_id` other than `id` on a non-fork names a parent; plus `system` `forked from <id>` for `forked_from_id` |
| later `session_meta` | `system` `copied from ancestor: session start ... id= cwd=` when its id differs; none when it repeats the file's own |
| `turn_context` | sets project and model; `system` `model: <model>` at the first one that names a model and `model: <old> -> <new>` at each later one whose `model` differs; `system` `permission mode: approval=<approval_policy> sandbox=<sandbox_policy> profile=<permission_profile> network=<network> reviewer=<approvals_reviewer> profile_id=<active_permission_profile.id>` at the first one with any of the fields and `permission mode: <old> -> <new>` on every change to the composite (a missing field is `-`, an object compact JSON); both at the line's `timestamp`, before the turn's rows, keyed per session |
| `event_msg` `thread_settings_applied` (Codex Desktop) | the same `model:` and `permission mode:` tracking as `turn_context`, at the event's `timestamp`, so a change the user makes between turns (a model switch, Approve for me, Full access and its revert) is dated when it was made rather than at the next turn. The event is the thread's settings, not a turn's: it carries `model`, `approval_policy`, `approvals_reviewer` and `active_permission_profile`, no `sandbox_policy`, and a `permission_profile` without the roots a turn adds, so only the three thread-level fields come from it; `sandbox` and `profile` stay as the last `turn_context` had them, or read `-` once the access level changed until the next `turn_context`, and an event before the thread's first `turn_context` is folded into that turn's row ([issue 160](https://github.com/seanthegeek/doubleagent/issues/160)) |
| `response_item` with `metadata.inherited_user_message` | none (copied from the parent thread) |
| `response_item` `message` user | per `content_item_kinds` run: `user` for `user.*`, `unknown` or empty kinds; `system` `context: <kind>: ...` for any other kind; without a kinds list as long as `content`, per block: `system` `context: <marker name>: ...` for a block that is wholly one of the harness wrappers below, else `user`. In a subagent file the first prompt is `system` `subagent task: ...`; later ones stay `user` (below) |
| `response_item` `message` assistant / developer | `assistant` / `system` (`developer: ...`); empty text skipped |
| `response_item` `function_call`, `custom_tool_call`, `local_shell_call`, `web_search_call`, `tool_search_call` | `tool_use` |
| `response_item` `function_call_output`, `custom_tool_call_output`, `local_shell_call_output`, `tool_search_output` (tool names) | `tool_result` |
| `response_item` `agent_message`, `inter_agent_communication` | `system` `agent message <author> -> <recipients>: ...` (`subagent task:` for a subagent's first prompt); an `inter_agent_communication` whose `id` a response item carries is skipped |
| `response_item` `image_generation_call` | `system` `image generation: <revised_prompt> status=...` |
| `response_item` `reasoning` | `thinking` with `--include-thinking` when it has text, else skipped |
| `response_item` `additional_tools`, `compaction`, `context_compaction`, `configuration_update` | skipped (no readable content) |
| `event_msg` `task_started`, `task_complete` | `system`; `task_started turn=<id>` as before (so the `thread_history_1.sqlite` copy still pairs with it), followed by `turn trigger: <turn_trigger> turn=<id>` when `turn_attribution.turn_trigger` is set (values in [`event_msg`](#event_msg); a `remote_*` turn came from a paired phone) |
| `event_msg` `task_complete` of a guardian review thread (`thread_source` `guardian_review`) | `system` `permission decision: <outcome> (auto_review risk_level=<risk_level> user_authorization=<user_authorization>) ...` from the JSON verdict in `last_agent_message`, naming the reviewer thread and carrying the `rationale` in full, at the line's `timestamp`, with `session_id` the governed thread (`session_meta.parent_thread_id`) |
| `event_msg` `thread_goal_updated` | `system` `thread goal: status=<goal.status> turn=<turnId> objective: <goal.objective>`, `-` for a missing status or turn (source-derived) |
| `event_msg` `turn_aborted` | `system` `turn aborted: <reason> [error message]` |
| `event_msg` `thread_rolled_back` | `system` `rolled back <n> turns`; earlier rows are not changed |
| `event_msg` `item_completed` `UserMessage`, `AgentMessage`, `Reasoning`, `HookPrompt`, `ContextCompaction` | skipped (repeat a response item or `compacted`) |
| `event_msg` `item_completed`, other items whose `id` a response item carries as `id` or `call_id` | skipped; a `CommandExecution` exit code still marks the call's `tool_result` |
| `event_msg` `item_completed` `CommandExecution`, `McpToolCall`, `DynamicToolCall`, `FileChange`, `WebSearch`, `Extension` `web.search` not carried | `tool_use` + `tool_result` on the item `id` |
| `event_msg` `item_completed` `FunctionCallOutput` not carried | `tool_result` |
| `event_msg` `item_completed` `Plan`, `SubAgentActivity`, image generation, `clock.sleep`, others not carried | `system` (`plan: ...`, `subagent activity: ...`, `image generation: ...`, `sleep: N ms`, `item <type>: <JSON>`) |
| `event_msg` legacy `user_message`, `agent_message`, `agent_reasoning`, and the rest | skipped |
| `compacted` | `system` `compaction summary: <message>`, or `compaction: replacement history of N items` |
| `realtime_item` `transcript_segment` | `user` or `assistant` by `role` |
| `realtime_item` other types | `system` `realtime: <type> <JSON>` |
| `token_usage_record`, `world_state`, `inter_agent_communication_metadata`, `retained_context`, `security_risk_score` | skipped |
| bad line, in a rollout or in `history.jsonl` | one `system` row per bad line at the end of the file's rows, `parser: unparseable line (<decoder reason>): <the line in full>`, `source_line` the bad line, `session_id` the rollout's session (empty for `history.jsonl`) |
| cut or corrupt zstd frame | one `system` row `parser: zstd: <message>` at the end, `source_line` the line after the last one decoded |
| `history.jsonl` line | `user` |
| `state_5.sqlite` `threads` row (secondary) | `system` `thread record: <id> title=<title> rollout=<rollout_path>`, then ` name=`, ` branch=`, ` model=` (the last turn's) and ` archived` when set, at `created_at_ms` (else `created_at`); then `system` `git remote: <git_origin_url>` at the same time when the column is set, the same text as the rollout's row ([issue 253](https://github.com/seanthegeek/doubleagent/issues/253)); then `first_user_message` as `user` with no timestamp, `system` `subagent task: ...` in a subagent's thread |
| `thread_history_1.sqlite` `thread_turns` row (secondary) | `system` `task_started turn=<id>` at `started_at`; `task_complete turn=<id> duration_ms=<n>` at `completed_at` for a completed or failed turn; an interrupted turn has no closing row (its reason is only in the rollout) |
| `thread_history_1.sqlite` `thread_items` row (secondary) | `userMessage` `user`, `agentMessage` `assistant`, `reasoning` `thinking` (opt-in), at `completed_at_ms`; `hookPrompt`, `contextCompaction` none; any other item renamed to the turn-item form and rendered as the `item_completed` rows above, at `started_at_ms` and `completed_at_ms` (else `created_at_ms`); an `item_json` that does not decode to a JSON object is one `system` row per record at the end of the file's rows, `parser: unparseable record <rowid> (<reason>): <item_json as stored>`, `source_line` the rowid, `session_id` and `project_path` the record's thread's ([issue 184](https://github.com/seanthegeek/doubleagent/issues/184)) |
| `automations/<slug>/automation.toml` (Codex only) | one `system` row `scheduled task: <id> name=<name> kind=<kind> status=<status> rrule=<rrule> target_thread=<target_thread_id> created=<created_at> updated=<updated_at> (from <file>, file mtime) prompt: <prompt>` at the file's modification time, `session_id` the target thread; a file that does not parse as TOML is one `system` row ([issue 119](https://github.com/seanthegeek/doubleagent/issues/119)) |
| `logs_2.sqlite` `logs` row `codex_core::session::handlers` `op=ExecApproval` or `codex_app_server::outgoing_message` `CommandExecutionRequestApproval` (Codex Desktop) | `system` `permission decision: <decision> exec <item id> turn=<turn id> (from logs_<N>.sqlite logs, ts)` at `ts` from the handlers row (`denied`, `approved` seen), `session_id` the row's thread id, `tool_use_id` the governed item's id (`exec-<uuid>`, [Other stores](#other-stores)); a response row (`Decline`, `Accept`) with no handlers row within two seconds is `permission decision: <decision> request <n> (from logs_<N>.sqlite logs, ts)` with no session ([issue 160](https://github.com/seanthegeek/doubleagent/issues/160)) |
| `logs_2.sqlite` `logs` row `codex_core::session::handlers` `op=PatchApproval` | `system` `permission decision: <decision> patch <item id> (from logs_<N>.sqlite logs, ts)` at `ts`, `session_id` the row's thread id, `tool_use_id` the governed `FileChange` item's id (source-derived) |
| `state_5.sqlite` `guardian_review_feedback` row | `system` at the `id`'s UUIDv7 time, `session_id` `reviewed_thread_id`, `tool_use_id` `target_item_id`: for a `denied` record whose `decision` is the reviewer's JSON verdict, the guardian review thread's text `permission decision: <outcome> (auto_review risk_level=<risk_level> user_authorization=<user_authorization>) exec <target_item_id> reviewer <reviewer_thread_id>: <rationale>`, so the two pair; for any other, `permission decision: <status> (auto_review feedback model=<model>) exec <target_item_id> turn=<reviewed_turn_id> reviewer <reviewer_thread_id>: <decision> action: <action>`, `-` for an empty value; a `record` that is not a JSON object is `parser: unparseable record <rowid> (<reason>): <record>` (source-derived) |
| `config.toml` (the home's, or a project's `.codex/config.toml`) | one `system` row per standing key, `permission setting: <key>=<value> (from .codex/config.toml, file mtime)`, for `approval_policy`, `approvals_reviewer`, `sandbox_mode`, `default_permissions`, `projects.<path>.trust_level` (`project_path` the path) and each `approval_mode` under `mcp_servers.<server>.tools.<tool>`, `plugins.<plugin>.mcp_servers.<server>.tools.<tool>` and `apps.<connector>.tools.<tool>`, at the file's modification time, `session_id` empty; no other key is printed; a file that does not parse as TOML is one `parser: unreadable config.toml: <reason>` row (source-derived) |
| `rules/*.rules` | one `system` row per Starlark statement, `permission setting: <statement> (from .codex/rules/<file>, file mtime)`, the statement joined onto one line with comments dropped, at the file's modification time, `source_line` its first line, `session_id` empty; a statement still open at the end of the file is `parser: unterminated rule: <its lines>` (source-derived) |
| `memories/rollout_summaries/<stem>.md` | one `system` row `memory summary: thread <thread_id> rollout=<rollout_path> (from <file>, updated_at): <summary>` at `updated_at`, `session_id` the thread, `project_path` `cwd`, `git_branch` `git_branch`; a file with no `thread_id:` header is `parser: memory summary has no thread_id header` (source-derived) |
| `db-backups/sqlite-<time>-<n>/state_5.sqlite` (secondary) | the rows of `state_5.sqlite` above, from the copy (source-derived) |
| `state_5.sqlite` `remote_control_enrollments` row | `system` `remote control: enabled=<0\|1> server=<server_id> environment=<environment_id> host=<server_name> client=<app_server_client_name> account=<account_id>` at `updated_at`, `session_id` empty ([issue 160](https://github.com/seanthegeek/doubleagent/issues/160)) |
| `.codex-global-state.json` (secondary, Codex Desktop) | `user` for each `prompt-history` entry of a thread, with an empty `timestamp_utc` (matched on text and session), `session_id` the thread id; `system` `session deleted: <id>` for each `codex-writing-block-deleted-thread-v1:<id>` key and `system` `permission mode (standing): ...` for each `heartbeat-thread-permissions-by-id` entry, at the file's modification time; `system` `thread description: ...` for each `thread-descriptions-v1` entry; one `system` `remote control: mobile device connected=<bool> environment=<id>` row from the two top-level keys; one `system` `scratch directory: <path> (projectless thread <id>) (from <file> thread-projectless-output-directories, file mtime)` row for each `thread-projectless-output-directories` entry, `session_id` the thread id and `project_path` the path itself, so inventory.json lists the directory under `scratch_directories` ([issue 149](https://github.com/seanthegeek/doubleagent/issues/149)); a file that does not decode is one `parser:` row ([issue 160](https://github.com/seanthegeek/doubleagent/issues/160)) |

#### Permission rows

Permission state is the per-turn `turn_context` policy above and, on Codex Desktop, every
`thread_settings_applied` event and the standing per-thread entries of `.codex-global-state.json`.
The rollout holds no per-call decision: approval requests and their answers are transient events
that the rollout never stores
([policy.rs:160-223](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/rollout/src/policy.rs#L160-L223)),
so a rollout shows what the policy allowed, not what the user approved. Codex Desktop logs the
user's answers in `logs_2.sqlite` and records an automatic answer as the verdict of a guardian
review thread (macOS, build 26.1013.40934, 2026-10-18; [Other stores](#other-stores)); the parser
yields a `permission decision:` row from each, joined to the governed call on its item id. The
approval dialog of that build offered no standing "always allow" grant for a command.

The standing grants and the other approval records of
[Standing grants and approval records](#standing-grants-and-approval-records) give the rows of the
table above, source-derived; not observed on any real install. Each statement of a `rules/*.rules`
file and each standing key of `config.toml` (the policy defaults, a project's `trust_level`, a
tool's `approval_mode`) is one `permission setting:` row at the file's modification time, which is
the time of the last grant, not of each; a project's own `.codex/config.toml` and `.codex/rules/`
give the same rows with that project as `project_path`. On Python 3.10, which has no `tomllib`,
`config.toml` is read for its single-line values under every table header, which holds every key
these rows need. Each `PatchApproval` log line is a `permission decision: ... patch <item id>` row
joined to the `apply_patch` call on the item id, like `ExecApproval`. Each
`guardian_review_feedback` record is a `permission decision:` row at its capture time, joined to the
governed call on `target_item_id`: a denied verdict reads exactly as the guardian review thread's
row, so `dedup.deduplicate` keeps one of the two when both exist, and a review that failed, timed
out or gave no readable verdict, which the review thread does not record as a verdict, is its own
row naming the status and the reviewed action. `RequestPermissionsResponse` lines are not read:
their `scope` is a debug print whose shape the source does not fix.

#### Model rows

The model is recorded per turn, in `turn_context.model`, and on Codex Desktop also at each change,
in `thread_settings_applied`; there is no per-message model, so the model rows follow the turns and
the settings events.

#### Past-use rows and secondary sources

Sources, by role ([issue 122](https://github.com/seanthegeek/doubleagent/issues/122)). The rollouts
are the primary store: Codex writes every other file from them. `history.jsonl`, `state_5.sqlite`,
`thread_history_1.sqlite` and Codex Desktop's `.codex-global-state.json`
are secondary sources, read on every run; the
timeline drops a row of theirs that a rollout row of the same session
repeats (same `turn_type` and `text`, time within 2 seconds, or any time
for a row with none) and keeps the rest. How each joins:

| Source | session_id | project_path | Time |
| --- | --- | --- | --- |
| `history.jsonl` | `session_id` | none | `ts`, whole seconds |
| `state_5.sqlite` thread record | `threads.id` | `threads.cwd` | `created_at_ms`, the thread's creation |
| `state_5.sqlite` first prompt | `threads.id` | `threads.cwd` | none: matches on text and session |
| `thread_history_1.sqlite` | `thread_id` | the same thread's `threads.cwd` from the `state_5.sqlite` beside it, else empty | the item's or turn's own, as the rollout row it repeats |
| `.codex-global-state.json` prompt history | the thread id in the key | none | none: matches on text and session |

The projection's rows take the times of the rollout rows they repeat:
a message at `completed_at_ms`, when the response item is written; a
tool item at its start and end, as the rollout reader stamps an
`item_completed` item; a turn at its `started_at` and `completed_at`,
under a second from the `task_started` and `task_complete` lines. The
fixture pairs all of them within the default 2 seconds. A tool call
made through a function call has a rollout `tool_use` text of the
model's arguments (`{"cmd": ...}` for `exec_command`, the patch for
`apply_patch`) and a projection text of the parsed command or the
changed paths, so the two never pair on text; `dedup.deduplicate` pairs
`tool_use` and `tool_result` rows that both carry a `tool_use_id` on
that id instead (analyzer 0.13.0). Known misses, each shown twice: a web
search longer than 2 seconds whose rollout also holds the
`web_search_call` response item, stamped at the end; on Open
Interpreter, an `agentMessage`, stamped at its start because the fork
has no migration 0007. A thread's first prompt is in three secondary
sources (`history.jsonl`, `first_user_message`, the projection's
`userMessage`); one pairs with the rollout's prompt, and
`dedup.deduplicate` collapses the other two onto that copy, the undated
`first_user_message` included, so only the rollout's row is kept
(fixture: `FIXTURE_DEDUPLICATED` in `tests/test_cli.py`).

The stores of [Other past-use stores](#other-past-use-stores), source-derived; not observed on
any real install:

- `db-backups/sqlite-<time>-<n>/state_5.sqlite`: secondary, read as `state_5.sqlite` is; its rows
  that the live database or a rollout repeats are dropped, and a deleted thread's record and first
  prompt are kept.
- `memories/rollout_summaries/<stem>.md`: one `memory summary:` row per file, joined on
  `thread_id` and `cwd`, at `updated_at`. It is read on every run as a source of its own, not a
  secondary one: the summary is the model's text about the thread, which repeats no event, so
  nothing could pair with it, and marking it recovered would mislabel every live thread. It
  outlives a thread delete until the next consolidation.
- `thread_goal_updated`: primary, in the rollout, one `thread goal:` row per event.
- Not read: `raw_memories.md` (one file for many threads, the raw memories' body has no fixed
  shape), `MEMORY.md` and `memory_summary.md` (consolidated across threads),
  `memories_1.sqlite` `stage1_outputs` and `goals_1.sqlite` `thread_goals` (each removed with the
  thread, repeating what the summary file and the goal events hold), and the other databases under
  `db-backups/`.

#### Sync, relay and origin rows

Codex has no session sync. The thread store is local only: the two backends are `Local` (rollouts
and SQLite) and `InMemory`
([core/src/config/mod.rs:599-607](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/core/src/config/mod.rs#L599-L607)),
and the old `experimental_thread_store_endpoint` for a remote store is now refused at load
([3251-3254](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/core/src/config/mod.rs#L3251-L3254)).
The thread metadata sync of [Joins and timestamps](#joins-and-timestamps) copies rollout facts into
`state_5.sqlite` and calls no server
([thread-store/src/thread_metadata_sync.rs:34-38](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/thread-store/src/thread_metadata_sync.rs#L34-L38)).
`codex resume` reopens a local rollout
([cli/src/main.rs:206-207](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/cli/src/main.rs#L206-L207)),
and the app server's `thread/*` methods have no share or upload method. What does leave the host,
beyond the model API call:

- **`/feedback`** ("send logs to maintainers";
  [tui/src/slash_command.rs:73](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/tui/src/slash_command.rs#L73),
  [93](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/tui/src/slash_command.rs#L93))
  sends a report to OpenAI's Sentry project
  ([feedback/src/lib.rs:60-61](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/feedback/src/lib.rs#L60-L61)).
  The TUI asks "Upload logs?" and lists the files
  ([tui/src/bottom_pane/feedback_view.rs:252](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/tui/src/bottom_pane/feedback_view.rs#L252),
  [282-285](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/tui/src/bottom_pane/feedback_view.rs#L282-L285));
  on yes the open thread's rollout
  ([tui/src/app/background_requests.rs:623-627](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/tui/src/app/background_requests.rs#L623-L627))
  and the rollouts of its subagent threads are attached
  ([app-server/src/request_processors/feedback_processor.rs:179-196](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/app-server/src/request_processors/feedback_processor.rs#L179-L196)),
  packed into `rollouts.tar.gz` in a system temporary directory
  ([feedback/src/lib.rs:840-853](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/feedback/src/lib.rs#L840-L853)).
  At `4e7b9e2` a report with logs also attaches the thread's failed automatic reviews as
  `auto-review-failures.jsonl`.
  On no, the report carries the category, the note and tags only, and the TUI says "Feedback
  recorded (no logs)"
  ([feedback_view.rs:54-58](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/tui/src/bottom_pane/feedback_view.rs#L54-L58)).
  Opt-in per report; the whole flow is on by default and turned off by `[feedback] enabled = false`
  in `config.toml`
  ([config/src/types.rs:233-236](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/config/src/types.rs#L233-L236),
  [core/src/config/mod.rs:4498-4502](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/core/src/config/mod.rs#L4498-L4502),
  [feedback_processor.rs:62-66](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/app-server/src/request_processors/feedback_processor.rs#L62-L66)).
  The marker is the `logs_2.sqlite` row of [Other stores](#other-stores). The parser yields one
  `system` row per such log row (`target` `codex_feedback` and the message
  `feedback event uploaded to Sentry`; the other `codex_feedback` rows report attachment failures
  and are not uploads),
  `session synced: feedback report uploaded to Sentry, attachments <uploaded_attachments> failed <attachments_failed> (from logs_2.sqlite logs, ts)`,
  `-` for a field the body lacks, at `ts` plus `ts_nanos`, with `session_id` the body's `thread_id`
  (empty for a `no-active-thread-` id) and the project and branch of that thread's `threads` row;
  the parser sets `records_sync`, so a thread without the row is `synced` `no` in `sessions.jsonl`.
  A report with 0 attachments sent no rollout. Fixture only: no real install has shown the row. The
  row survives the rollout's loss outside Codex for up to 10 days; Codex's own thread delete removes
  it with the thread, Codex offers no delete of the Sentry copy, and nothing local records one.
- **`codex cloud`** (experimental;
  [cli/src/main.rs:227-229](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/cli/src/main.rs#L227-L229))
  creates a Codex Cloud task, a run on OpenAI's servers against a GitHub branch: the typed prompt,
  the environment id and the branch, plus a local diff only when `CODEX_STARTING_DIFF` is set
  ([cloud-tasks-client/src/http.rs:332-372](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/cloud-tasks-client/src/http.rs#L332-L372)).
  It sends no local thread; the task's transcript is server-side, and `codex apply` fetches a task's
  diff back
  ([chatgpt/src/get_task.rs:37-38](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/chatgpt/src/get_task.rs#L37-L38)).
  Its only local trace is an `error.log` that it appends to in the directory it was started from,
  not under `~/.codex`
  ([cloud-tasks/src/util.rs:18-28](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/cloud-tasks/src/util.rs#L18-L28),
  [cloud-tasks-client/src/http.rs:911-920](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/cloud-tasks-client/src/http.rs#L911-L920)),
  with lines `[<RFC 3339 time>] new_task: created id=<task id> env=<env id> prompt_chars=<n>`
  ([http.rs:374-381](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/cloud-tasks-client/src/http.rs#L374-L381)),
  the GitHub origins it matched
  ([cloud-tasks/src/env_detect.rs:51](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/cloud-tasks/src/env_detect.rs#L51))
  and the `account_id` it authenticated as
  ([cloud-tasks/src/lib.rs:103](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/cloud-tasks/src/lib.rs#L103)).
  No catalog line collects that file; no row is proposed until it is.
- **`codex remote-control`** (experimental;
  [cli/src/main.rs:176-177](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/cli/src/main.rs#L176-L177),
  [cli/src/remote_control_cmd.rs:53-62](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/cli/src/remote_control_cmd.rs#L53-L62))
  enrolls the host's app server with the ChatGPT backend
  (`wham/remote/control/environments`;
  [app-server-transport/src/transport/remote_control/clients.rs:268-270](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/app-server-transport/src/transport/remote_control/clients.rs#L268-L270))
  so a ChatGPT client can drive it over a websocket. Threads stay in
  the local store and the remote client reads them live through the
  app server. Observed with the ChatGPT app's remote connections
  (Settings > Connections > Control this Mac or PC; build
  26.1013.40934, 2026-10-18 and 2026-10-19, [Source and evidence](#source-and-evidence)),
  it leaves two records: the host-level `remote_control_enrollments`
  row of [Other stores](#other-stores), which the parser yields as a
  `remote control:` row, and a per-turn mark, `turn_attribution.turn_trigger`
  `remote_ios` or `remote_android` on a turn sent from a paired phone,
  which the parser's `turn trigger:` row carries; the
  phone's own connection is in `logs_2.sqlite`. The vendor page
  [Remote connections | ChatGPT Learn](https://learn.chatgpt.com/docs/remote-connections)
  (read 2026-10-18) says "Remote connections let you access work running
  on another device or machine", "Your phone sends prompts, approvals,
  and follow-up messages to ChatGPT" and "Repository files and local
  documents come from the connected host"; it does not say whether a
  transcript is copied to the account. The relay carries control, not a copy of
  the thread, so the parser marks no session synced or relayed for it.
- **`--remote <ADDR>`** points the TUI at an app server the user runs elsewhere
  ([cli/src/main.rs:962-974](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/cli/src/main.rs#L962-L974)).
  The rollout is written on that server's host, but the TUI still appends each prompt to the local
  `history.jsonl`
  ([tui/src/app/thread_routing.rs:557-566](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/tui/src/app/thread_routing.rs#L557-L566)),
  so a `history.jsonl` session with no local rollout may be such a thread. Not a vendor upload.
- **Analytics** go to `<chatgpt base URL>/codex/analytics-events/events`
  ([analytics/src/client.rs:154](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/analytics/src/client.rs#L154))
  as events keyed by thread id
  ([analytics/src/events.rs:257](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/analytics/src/events.rs#L257));
  no prompt or reply text field was found (searched `prompt`, `text`, `user_message` in
  `analytics/src`). Turned off by `[analytics] enabled = false`
  ([config/src/types.rs:223-228](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/config/src/types.rs#L223-L228)).
  OpenTelemetry export, which can carry prompts and replies, is off by default and goes only to an
  exporter the user configures
  ([config/src/types.rs:597-611](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/config/src/types.rs#L597-L611),
  [647-657](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/config/src/types.rs#L647-L657)).
  Image attachments are kept inline in the rollout; the one attachment store uploads nothing
  ([attachment-store/src/lib.rs:160-170](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/attachment-store/src/lib.rs#L160-L170)).

The ChatGPT desktop app's own sync state is outside this source: its
client code is not in the repository (searched
`chatgpt-conversation-resume-tokens`, `resume-tokens`,
`conversation_origin`, `chatgpt_async_status`, `codex-dev.db`,
`local_thread_catalog` and `electron-persisted-atom-state` across the
clone; the only matches are unrelated token-usage replay code). The
`chatgpt` rows of `codex-dev.db` `local_thread_catalog` and the
resume tokens in `.codex-global-state.json` are the app's index of
ChatGPT conversations that live on the server, copied one way, account
to app, as a sign-in snapshot; the `chatgpt-desktop` parser reads them
([Parser plan in `chatgpt-desktop.md`](chatgpt-desktop.md#parser-plan)).
Whether a local Codex thread is ever uploaded as a ChatGPT conversation
was not determined from source; on macOS (build 26.1013.40934,
2026-10-18) it was not: the app's Codex-mode threads did not appear in
the Codex area of chatgpt.com (a user report, no listing), and the
delete dialog offered no remote copy to remove. The vendor page
[Codex Cloud](https://learn.chatgpt.com/docs/cloud) (read 2026-10-18)
describes server-side tasks, where "Each task has its own workspace and
can keep working while your computer is asleep", and says nothing about
uploading local threads. So the parser marks no Codex-mode thread
synced.

Codex records no relay and no remote-origin session, at `3e23877` and at `4e7b9e2`.

#### Scratch directory rows

Codex Desktop's projectless output directory is a scratch directory: the parser yields a
`scratch directory:` row for each `thread-projectless-output-directories` entry of
`.codex-global-state.json` (the table above), so inventory.json lists the directory under
`scratch_directories` ([issue 149](https://github.com/seanthegeek/doubleagent/issues/149)).

A rollout `cwd` under `<home>/.codex/worktrees` is classed as a scratch directory by the catalog
rule (source at `4e7b9e2`); one under a configured `git-worktree-root` is not, and no row marks it.
The worktree's `codex-thread.json` (`version`, `ownerThreadId`) sits in the source repository's git
directory for that worktree
([worktree/src/metadata.rs:19-27](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/worktree/src/metadata.rs#L19-L27),
[88-91](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/worktree/src/metadata.rs#L88-L91)),
outside every catalog path.

#### Deletion rows

The desktop app's deletion marker already gives a `session deleted:` row. A delete through
`thread/delete` (the TUI, the app server) leaves no marker; what it removes and what survives it is
in [What survives a delete](#what-survives-a-delete), and the survivors that matter here are
`history.jsonl`, the memory rollout summaries and the `db-backups/` copies of `state_5.sqlite`,
which the parser reads; none of them records the delete itself.

#### Bad-line rows

A bad line in a rollout or in `history.jsonl`, a cut or corrupt zstd frame, a
`thread_history_1.sqlite` row whose `item_json` does not decode, a `guardian_review_feedback`
`record` that is not a JSON object and a rules statement still open at the end of its file each
give the rows in the table above (the last two source-derived).

Until [issue 184](https://github.com/seanthegeek/doubleagent/issues/184) a `thread_history_1.sqlite`
row with an unreadable `item_json` gave one summary row
(`parser: <n> thread_items row(s) with unreadable item_json, first at rowid <n>`); it is now one
row per record with the value in full, as the table above gives (fixture only, no real install has
shown a corrupt row).

A `state_5.sqlite` (or its copy under `db-backups/`), `thread_history_1.sqlite` or `logs_<N>.sqlite`
that cannot be read, or whose `-wal`, `-shm` or `-journal` sidecar cannot, is one row
`parser: <file> could not be read (<reason>)`, `<file>` the name of the file that failed; one that
is not a SQLite database is `parser: not a SQLite database`. The rollout parse, which reads
`state_5.sqlite` and the log databases beside it for thread records and guardian review calls, gives
no row of its own for such a database: the database's own parse reports it.

A `rollout-*.jsonl` or `rollout-*.jsonl.zst` rollout, a `history.jsonl`, an `automation.toml`, a
`config.toml`, a `rules/*.rules` file, a `memories/rollout_summaries/*.md` file or the
`.codex-global-state.json` that cannot be read is one whole-file row
`parser: <file> could not be read (<reason>)` through `error_rows`, `<file>` its name, with
`source_line` 0 and an empty `timestamp_utc`; a rollout carries the session id its file name gives,
and a project's `config.toml` or rules file carries the project as `project_path`. A rollout that
fails while being read gives that row at the last line read, after the rows of the lines before it.

#### Fixtures

Expected rows from the sample below: `system` session start, `system`
`task_started`, `system` `developer: ...`, `system`
`context: environments.environment_context: ...`, `user` "run the tests",
`tool_use` `exec_command` (`call_1`), `tool_result` `[exit 1] 1 failing`
(the `CommandExecution` item has id `call_1`, so it adds only the exit
code), `thinking` "one test fails" with `--include-thinking`, `assistant`,
`system` `task_complete`, `system`
`agent message /root -> /root/tester: rerun only the failing test`, and
a `system` row carrying the cut last line in full. The `UserMessage` item and the
`token_usage_record` give no row.

Sample rollout, paginated shape, at
`.codex/sessions/2026/10/02/rollout-2026-10-02T11-00-00-0199b2c3-1111-7aaa-8bbb-000000000001.jsonl`:

```json
{"timestamp":"2026-10-02T09:00:00.000Z","ordinal":0,"type":"session_meta","payload":{"session_id":"0199b2c3-1111-7aaa-8bbb-000000000001","id":"0199b2c3-1111-7aaa-8bbb-000000000001","timestamp":"2026-10-02T09:00:00.000Z","cwd":"/srv/proj","originator":"codex-tui","cli_version":"0.150.0","source":"cli","thread_source":"user","model_provider":"openai","history_mode":"paginated","git":{"commit_hash":"0a1b2c3d","branch":"main"}}}
{"timestamp":"2026-10-02T09:00:01.000Z","ordinal":1,"type":"event_msg","payload":{"type":"task_started","turn_id":"0199b2c3-2222-7aaa-8bbb-000000000001","started_at":1790931601,"model_context_window":272000,"collaboration_mode_kind":"default"}}
{"timestamp":"2026-10-02T09:00:01.100Z","ordinal":2,"type":"response_item","payload":{"type":"message","id":"m1","role":"developer","content":[{"type":"input_text","text":"<permissions instructions>sandboxed</permissions instructions>"}]}}
{"timestamp":"2026-10-02T09:00:01.200Z","ordinal":3,"type":"turn_context","payload":{"turn_id":"0199b2c3-2222-7aaa-8bbb-000000000001","cwd":"/srv/proj","approval_policy":"on-request","sandbox_policy":{"type":"workspace-write"},"model":"gpt-5.5-codex","summary":"auto"}}
{"timestamp":"2026-10-02T09:00:01.300Z","ordinal":4,"type":"response_item","payload":{"type":"message","id":"m2","role":"user","content":[{"type":"input_text","text":"<environment_context>\n<cwd>/srv/proj</cwd>\n</environment_context>"}],"internal_chat_message_metadata_passthrough":{"turn_id":"0199b2c3-2222-7aaa-8bbb-000000000001","content_item_kinds":["environments.environment_context"]}}}
{"timestamp":"2026-10-02T09:00:01.400Z","ordinal":5,"type":"response_item","payload":{"type":"message","id":"m3","role":"user","content":[{"type":"input_text","text":"run the tests"}],"internal_chat_message_metadata_passthrough":{"turn_id":"0199b2c3-2222-7aaa-8bbb-000000000001","content_item_kinds":["user.text"]}},"metadata":{"client_authored":true,"user_input_order":0}}
{"timestamp":"2026-10-02T09:00:01.500Z","ordinal":6,"type":"event_msg","payload":{"type":"item_completed","thread_id":"0199b2c3-1111-7aaa-8bbb-000000000001","turn_id":"0199b2c3-2222-7aaa-8bbb-000000000001","item":{"type":"UserMessage","id":"u1","content":[{"type":"text","text":"run the tests","text_elements":[]}]},"started_at_ms":1790931601500,"completed_at_ms":1790931601500}}
{"timestamp":"2026-10-02T09:00:03.000Z","ordinal":7,"type":"response_item","payload":{"type":"function_call","id":"fc1","name":"exec_command","arguments":"{\"cmd\":\"npm test\"}","call_id":"call_1"}}
{"timestamp":"2026-10-02T09:00:06.000Z","ordinal":8,"type":"response_item","payload":{"type":"function_call_output","call_id":"call_1","output":"1 failing"}}
{"timestamp":"2026-10-02T09:00:06.100Z","ordinal":9,"type":"event_msg","payload":{"type":"item_completed","thread_id":"0199b2c3-1111-7aaa-8bbb-000000000001","turn_id":"0199b2c3-2222-7aaa-8bbb-000000000001","item":{"type":"CommandExecution","id":"call_1","command":["npm","test"],"cwd":"/srv/proj","parsed_cmd":[],"source":"agent","status":"failed","aggregated_output":"1 failing","exit_code":1},"started_at_ms":1790931603000,"completed_at_ms":1790931606000}}
{"timestamp":"2026-10-02T09:00:07.000Z","ordinal":10,"type":"response_item","payload":{"type":"reasoning","id":"rs1","summary":[{"type":"summary_text","text":"one test fails"}],"encrypted_content":"gAAAA"}}
{"timestamp":"2026-10-02T09:00:08.000Z","ordinal":11,"type":"response_item","payload":{"type":"message","id":"m4","role":"assistant","phase":"final_answer","content":[{"type":"output_text","text":"One test fails in src/a.test.ts."}]}}
{"timestamp":"2026-10-02T09:00:08.100Z","ordinal":12,"type":"token_usage_record","payload":{"thread_id":"0199b2c3-1111-7aaa-8bbb-000000000001","turn_id":"0199b2c3-2222-7aaa-8bbb-000000000001","session_id":"0199b2c3-1111-7aaa-8bbb-000000000001","root_turn_id":"0199b2c3-2222-7aaa-8bbb-000000000001","response_id":"resp_1","usage":{"input_tokens":900,"cached_input_tokens":0,"output_tokens":40,"reasoning_output_tokens":10,"total_tokens":940}}}
{"timestamp":"2026-10-02T09:00:08.200Z","ordinal":13,"type":"event_msg","payload":{"type":"task_complete","turn_id":"0199b2c3-2222-7aaa-8bbb-000000000001","last_agent_message":"One test fails in src/a.test.ts.","started_at":1790931601,"completed_at":1790931608,"duration_ms":7200}}
{"timestamp":"2026-10-02T09:00:09.000Z","ordinal":14,"type":"inter_agent_communication","payload":{"author":"/root","recipient":"/root/tester","other_recipients":[],"content":"rerun only the failing test","trigger_turn":true}}
this line is cut mid-wri
```

`history.jsonl`:
`{"session_id":"0199b2c3-1111-7aaa-8bbb-000000000001","ts":1790931601,"text":"run the tests"}`

Source-derived samples (codex at `4e7b9e2`), each giving the row of the table above:

- `rules/default.rules`: `prefix_rule(pattern=["git", "status"], decision="allow")` and
  `network_rule(host="example.com", protocol="https", decision="deny", justification="no")`.
- `config.toml`: `approval_policy = "on-request"`, `[projects."/srv/proj"]` `trust_level =
  "trusted"`, `[mcp_servers.github.tools.create_issue]` `approval_mode = "approve"`.
- A `thread_goal_updated` line:
  `{"timestamp":"2026-10-02T09:00:01.500Z","type":"event_msg","payload":{"type":"thread_goal_updated","threadId":"0199b2c3-1111-7aaa-8bbb-000000000001","turnId":"t1","goal":{"threadId":"0199b2c3-1111-7aaa-8bbb-000000000001","objective":"keep the tests green","status":"active","tokensUsed":0,"timeUsedSeconds":0,"createdAt":1790931601,"updatedAt":1790931601}}}`.
- A `logs_2.sqlite` body: `Submission thread_id=<thread> submission_id=<uuid>
  operation="patch_approval" op=PatchApproval { id: "<item id>", decision: approved_for_session }`.
- A `guardian_review_feedback` row: `id` a UUIDv7, `record`
  `{"reviewed_thread_id":"<thread>","reviewed_turn_id":"t3","target_item_id":"<item id>","reviewer_thread_id":"<review thread>","model":"codex-auto-review","status":"timed_out","decision":null,"action":"rm -rf build","action_truncated":false,"instructions":null,"history":[],"context_omitted":false}`.
- `memories/rollout_summaries/<stem>.md`: the lines `thread_id: <thread>`,
  `updated_at: 2026-10-02T09:00:10+00:00`, `rollout_path: <path>`, `cwd: /srv/proj`,
  `git_branch: main`, a blank line, then the summary.

#### Injected user-role context

Rollouts without content kinds are classified per content block by the wrappers Codex itself
recognises as injected user-role context
([core/src/context/contextual_user_message.rs:22-37](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/core/src/context/contextual_user_message.rs#L22-L37)).
As in Codex's own check, the block must begin with the open marker after leading whitespace and end
with the close marker, ignoring ASCII case
([context-fragments/src/fragment.rs:116-130](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/context-fragments/src/fragment.rs#L116-L130)),
so a tag a person types inside a prompt leaves it `user`:

| Marker name | Open … close | Source |
| --- | --- | --- |
| `agents_md_instructions` | `# AGENTS.md instructions` … `</INSTRUCTIONS>` | [core/src/context/user_instructions.rs:23-25](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/core/src/context/user_instructions.rs#L23-L25) |
| `environment_context` | `<environment_context>` … `</environment_context>` | [protocol/src/protocol.rs:120-121](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/protocol/src/protocol.rs#L120-L121), [core/src/context/world_state/environment.rs:524-529](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/core/src/context/world_state/environment.rs#L524-L529) |
| `agent_message_board_notification` | `<agent_message_board_notification>` … closing tag | [core/src/context/agent_message_board_notification.rs:19-25](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/core/src/context/agent_message_board_notification.rs#L19-L25) |
| `skill` | `<skill>` … `</skill>` | [ext/skills/src/fragments.rs:89-91](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/ext/skills/src/fragments.rs#L89-L91) |
| `user_shell_command` | `<user_shell_command>` … closing tag | [core/src/context/user_shell_command.rs:44](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/core/src/context/user_shell_command.rs#L44) |
| `turn_aborted` | `<turn_aborted>` … closing tag | [core/src/context/turn_aborted.rs:34](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/core/src/context/turn_aborted.rs#L34) |
| `subagent_notification` | `<subagent_notification>` … closing tag | [core/src/context/subagent_notification.rs:34-36](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/core/src/context/subagent_notification.rs#L34-L36) |
| `recommended_plugins` | `<recommended_plugins>` … closing tag | [core/src/context/recommended_plugins_instructions.rs:43](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/core/src/context/recommended_plugins_instructions.rs#L43) |

#### Subagent input and copied forks

Later `role: "user"` messages in a subagent's rollout stay `user`: the parent's v1 `send_input` tool
submits plain user input to the child
([core/src/agent/control.rs:134-143](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/core/src/agent/control.rs#L134-L143)),
and a person can also type into a subagent thread, because the TUI's `/subagents` picker makes it
the active thread and composer input is sent to the active thread
([tui/src/multi_agents.rs:1-5](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/tui/src/multi_agents.rs#L1-L5),
[tui/src/app/thread_routing.rs:475-497](https://github.com/openai/codex/blob/3e238776e857eccd3bde6bff3026e2e9798f6524/codex-rs/tui/src/app/thread_routing.rs#L475-L497)).
The rollout records both alike.

A copied fork has no boundary beyond the ancestor's `session_meta` and the
per-message `inherited_user_message` flag, so the other copied records stay
in the fork's file under the fork's session id.

#### Source host rows

The agent records no source-host integration in a file the collector gathers (source at
[`4e7b9e2`](https://github.com/openai/codex/commit/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857),
grepped for `github`, `gitlab`, `bitbucket`, `oauth`, `installation`, `repositor`, `pull request`,
`clone` and `cloud`). A Codex Cloud task runs in a server-side environment bound to a GitHub
repository, which `codex cloud` looks up from the local repository's git origins
(`wham/environments/by-repo/github/<owner>/<repo>`;
[cloud-tasks/src/env_detect.rs:50-63](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/cloud-tasks/src/env_detect.rs#L50-L63)), so
the connected GitHub account, its grant, the repository list and the task conversations are
server-side only; the one local trace is the `error.log` of
[Sync, relay and origin rows](#sync-relay-and-origin-rows), outside every catalog path. The network
proxy's credential broker substitutes a `GH_TOKEN` or `GITHUB_TOKEN` from the environment into
requests to GitHub hosts and stores nothing
([network-proxy/src/credential_broker/providers/github.rs:13-20](https://github.com/openai/codex/blob/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857/codex-rs/network-proxy/src/credential_broker/providers/github.rs#L13-L20)).
A GitHub MCP server's configuration in `config.toml` is an MCP server, not a Codex integration.

## Confidence

- High: everything in [Per-user storage](#per-user-storage) to
  [Where the project path is recorded](#where-the-project-path-is-recorded) cited to source, which the local listing
  matches; the file layout and naming, the envelope, the written type set and the legacy and
  paginated split, the `session_meta`, `turn_context` and `ResponseItem` field names, timestamp
  formats and units, and the `history.jsonl` and `session_index.jsonl` shapes (source, and the
  local tabulation agrees for every type it contains); the `state_5.sqlite` and
  `thread_history_1.sqlite` columns and `item_json` shapes, from the migrations and structs at
  [3e23877](https://github.com/openai/codex/commit/3e238776e857eccd3bde6bff3026e2e9798f6524), whose
  table and column names agree with the Windows install.
- High, observed: the desktop-only files and tables in
  [Codex Desktop inside the ChatGPT app](#codex-desktop-inside-the-chatgpt-app) and the credentials
  they add (Windows, Codex Desktop 0.162.0-alpha.17.2); [Standalone CLI on
  Linux](#standalone-cli-on-linux) (Ubuntu 26.04.1, Codex CLI 0.162.1);
  [macOS differences](#macos-differences) (macOS 26.6.2, Codex Desktop and Codex CLI 0.162.1); the
  layout in [Standalone CLI on Windows](#standalone-cli-on-windows) (Codex CLI 0.162.1); and
  [macOS, ChatGPT 26.1013.40934](#macos-chatgpt-26101340934) with the records of
  [Record schema](#record-schema) and [Other stores](#other-stores) it showed (Codex Desktop
  0.163.0-alpha.10.2, one step per listing).
- Medium: that copied forks can contain an ancestor's `session_meta` (inferred from the metadata
  sync guard, not from a file); the exact payload of `item_completed` for item kinds not seen
  locally (read from the structs, not from files); the `AgentPath` string form in the sample
  (`/root/...` is illustrative); `cap_sid` holding capability SIDs (inferred from key names and
  value lengths); the pairing of the `state_5.sqlite` and `thread_history_1.sqlite` times with the
  rollout's, since the rows rest on a synthetic fixture and no real install's values were read.
- Not determined: pre-envelope rollouts from the earliest releases; whether
  `local_shell_call_output` exists (the parser lists it but `ResponseItem` has no such variant at
  `3e23877`; it is harmless); how often the harness emits `agent_message` response items versus
  `inter_agent_communication` lines; what `tui-thread-reference-capabilities/<thread uuid>` records
  beyond its 0-byte name; the purpose of `.codex-provisioning-*.guard` beyond its name; where
  remote control's server token is kept; what `chatgpt_async_status` and `conversation_origin`
  `tpp` mean; which `logs_2.sqlite` rows survive the desktop app's delete.
