# Agents

Every agent in the catalog (the table of agents and the paths under a
home directory where each keeps its state, shared by the collectors and
the analyzer), in alphabetical order of its catalog name, with what
DoubleAgent does with its files. The status is one of three words:

- `parsed`: the analyzer reads the agent's transcripts. "Validated
  against" names the evidence the parser was checked against: source, a
  shipped bundle, or real installs, with their platforms. The agent's
  subsection under [Parsed agents](#parsed-agents) lists the files the
  parser reads and the installs on which they were seen.
- `backlog`: the collector gathers transcripts or other parseable records,
  and a parser is wanted; the row links the issue. Windsurf is blocked on
  format research: its record format is not known well enough to write
  one.
- `collected`: the files are collected and detected but hold nothing to
  parse: a token store, configuration only, or a surface (a CLI, a
  desktop app or an editor extension of the agent) that left no
  local transcript on the installs checked. The analyzer's `timeline`
  command reports these agents as `collected only`.

For a `backlog` or `collected` agent, "Validated against" gives the
evidence level of the Collection part of the agent's research document,
and the agent's subsection under [Collected agents](#collected-agents)
lists what the collector gathers. Notes links the research document and
the open issues. The
agent name in each row links to its subsection.

| Agent | Status | Validated against | Notes |
| --- | --- | --- | --- |
| [`agent-zero`](#agent-zero) | parsed | source; fixture only | [research](../research/agent-zero.md) |
| [`aider`](#aider) | parsed | source; fixture only | [research](../research/aider.md) |
| [`amp`](#amp) | backlog | Shipped binary strings, docs | [research](../research/amp.md); [issue 53](https://github.com/seanthegeek/doubleagent/issues/53); prompt history and thread logs; full threads are server-side |
| [`antigravity`](#antigravity) | parsed | real installs: WSL, macOS, Ubuntu, Windows (October 2026) | [research](../research/antigravity.md) |
| [`augment`](#augment) | backlog | Shipped bundle, docs | [research](../research/augment.md); [issue 55](https://github.com/seanthegeek/doubleagent/issues/55); full conversations in `.augment/sessions/*.json` and a prompt history |
| [`autogen-studio`](#autogen-studio) | backlog | Source | [research](../research/autogen-studio.md); [issue 69](https://github.com/seanthegeek/doubleagent/issues/69); SQLite `autogen04203.db` with the `message`, `session` and `run` tables |
| [`camel-ai`](#camel-ai) | collected | Source | [research](../research/camel-ai.md); no transcripts under the home; history is in the working directory |
| [`chatgpt-desktop`](#chatgpt-desktop) | parsed | real installs: Windows, macOS (October 2026) | [research](../research/chatgpt-desktop.md); [issue 49](https://github.com/seanthegeek/doubleagent/issues/49); ChatGPT conversations are listed only, their text is server-side; Codex threads parse as `codex-cli` |
| [`claude-code`](#claude-code) | parsed | real installs: Windows, Linux, macOS (October 2026) | [research](../research/claude-code.md) |
| [`claude-desktop`](#claude-desktop) | parsed | real installs: Windows (MSIX), Linux, macOS (October 2026) | [research](../research/claude-desktop.md) |
| [`cline`](#cline) | parsed | source; fixture only | [research](../research/cline.md) |
| [`codex-cli`](#codex-cli) | parsed | real installs: Windows, macOS, Ubuntu (October 2026) | [research](../research/codex-cli.md) |
| [`cody`](#cody) | parsed | source; fixture only | [research](../research/cody.md) |
| [`continue`](#continue) | parsed | source; fixture only | [research](../research/continue.md) |
| [`copilot`](#copilot) | collected | Source (the vendored language server) | [research](../research/copilot.md); an OAuth token store with no transcripts; Copilot Chat is parsed as `vscode` |
| [`copilot-cli`](#copilot-cli) | parsed | bundle and real installs: macOS, Linux, Windows (winget 1.0.98 and Copilot Desktop's bundled 1.0.99-2) (October 2026) | [research](../research/copilot-cli.md); [issue 132](https://github.com/seanthegeek/doubleagent/issues/132); the legacy flat `session-state/<id>.jsonl` transcript is not parsed yet |
| [`copilot-desktop`](#copilot-desktop) | parsed | shipped app and real installs: Linux, Windows (October 2026) | [research](../research/copilot-desktop.md); [issue 147](https://github.com/seanthegeek/doubleagent/issues/147); [issue 149](https://github.com/seanthegeek/doubleagent/issues/149): the inventory lists the app's own directories as projects |
| [`crewai`](#crewai) | backlog | Source, appdirs source | [research](../research/crewai.md); [issue 140](https://github.com/seanthegeek/doubleagent/issues/140); `latest_kickoff_task_outputs.db` holds every task's output with its messages |
| [`crush`](#crush) | parsed | source; fixture only | [research](../research/crush.md) |
| [`cursor`](#cursor) | parsed | real installs: macOS IDE and CLI, Linux and Windows IDE (October 2026) | [research](../research/cursor.md); [issue 51](https://github.com/seanthegeek/doubleagent/issues/51); [issue 235](https://github.com/seanthegeek/doubleagent/issues/235); [issue 236](https://github.com/seanthegeek/doubleagent/issues/236); [issue 237](https://github.com/seanthegeek/doubleagent/issues/237); the Agent CLI's own chat store is not parsed yet |
| [`dify`](#dify) | backlog | Source | [research](../research/dify.md); [issue 142](https://github.com/seanthegeek/doubleagent/issues/142); sandbox `shellctl.db` and job logs hold shell jobs; chats in PostgreSQL |
| [`factory-droid`](#factory-droid) | backlog | Shipped binary strings, docs | [research](../research/factory-droid.md); [issue 54](https://github.com/seanthegeek/doubleagent/issues/54); JSONL session transcripts under `.factory/sessions` and a prompt history |
| [`flowise`](#flowise) | backlog | Source | [research](../research/flowise.md); [issue 68](https://github.com/seanthegeek/doubleagent/issues/68); `database.sqlite` with `chat_message` (tool use and agent reasoning) and `execution` |
| [`gemini-cli`](#gemini-cli) | parsed | source; fixture only | [research](../research/gemini-cli.md) |
| [`goose`](#goose) | parsed | source; fixture only | [research](../research/goose.md); [issue 368](https://github.com/seanthegeek/doubleagent/issues/368); the desktop app's local-storage prompt history is not parsed |
| [`hermes`](#hermes) | parsed | source; fixture only | [research](../research/hermes.md); [issue 208](https://github.com/seanthegeek/doubleagent/issues/208); the nested `config.yaml` `approvals` settings are not parsed |
| [`kilo-code`](#kilo-code) | parsed | source; fixture only | [research](../research/kilo-code.md); [issue 192](https://github.com/seanthegeek/doubleagent/issues/192); [issue 221](https://github.com/seanthegeek/doubleagent/issues/221); the session pull request links are not parsed yet |
| [`kiro`](#kiro) | parsed | source; fixture only | [research](../research/kiro.md); [issue 212](https://github.com/seanthegeek/doubleagent/issues/212); Kiro CLI's own sessions and permission files wait for a real install |
| [`langflow`](#langflow) | backlog | Source, platformdirs source; Desktop from docs only | [research](../research/langflow.md); [issue 141](https://github.com/seanthegeek/doubleagent/issues/141); Desktop's `database.db`; the server's `langflow.db` is not collected |
| [`letta`](#letta) | parsed | source; fixture only | [research](../research/letta.md) |
| [`little-coder`](#little-coder) | parsed | source; fixture only | [research](../research/little-coder.md) |
| [`local-deep-research`](#local-deep-research) | backlog | Source, platformdirs source | [research](../research/local-deep-research.md); [issue 56](https://github.com/seanthegeek/doubleagent/issues/56); history and chat in SQLCipher databases keyed from the login password |
| [`metagpt`](#metagpt) | collected | Source | [research](../research/metagpt.md); configuration only; its workspace and `team.json` are under the working directory |
| [`muse-code`](#muse-code) | parsed | binary strings and real install: Linux (October 2026) | [research](../research/muse-code.md) |
| [`n8n`](#n8n) | backlog | Source | [research](../research/n8n.md); [issue 67](https://github.com/seanthegeek/doubleagent/issues/67); `database.sqlite` executions and chats, and the gateway's tool-call log |
| [`nanobot`](#nanobot) | parsed | source; fixture only | [research](../research/nanobot.md) |
| [`ollama`](#ollama) | parsed | source; fixture only | [research](../research/ollama.md) |
| [`open-interpreter`](#open-interpreter) | parsed | source; fixture only | [research](../research/open-interpreter.md); [issue 70](https://github.com/seanthegeek/doubleagent/issues/70); the legacy Python tool's `conversations/*.json` files are collected but not parsed yet |
| [`openclaw`](#openclaw) | parsed | source; fixture only | [research](../research/openclaw.md) |
| [`opencode`](#opencode) | parsed | source; fixture only | [research](../research/opencode.md); [issue 194](https://github.com/seanthegeek/doubleagent/issues/194); [issue 213](https://github.com/seanthegeek/doubleagent/issues/213); the desktop app's `drafts.sqlite` and the `shell/` command output are not parsed yet |
| [`openhands`](#openhands) | parsed | source; fixture only | [research](../research/openhands.md); [issue 205](https://github.com/seanthegeek/doubleagent/issues/205); Agent Canvas browser storage (cloud origin, archive) needs a LevelDB reader |
| [`pearai`](#pearai) | parsed | source; fixture only | [research](../research/pearai.md) |
| [`pi`](#pi) | parsed | source; fixture only | [research](../research/pi.md) |
| [`pydantic-clai`](#pydantic-clai) | backlog | Source, PyPI wheel | [research](../research/pydantic-clai.md); [issue 66](https://github.com/seanthegeek/doubleagent/issues/66); CLAI 2.0 `sessions.db` conversations and the prompt histories |
| [`qwen-code`](#qwen-code) | parsed | source; fixture only | [research](../research/qwen-code.md); [issue 396](https://github.com/seanthegeek/doubleagent/issues/396); the usage log, channel histories, todo files, project summary and managed-local transcripts are not parsed |
| [`roo-code`](#roo-code) | parsed | source; fixture only | [research](../research/roo-code.md) |
| [`shared`](#shared) | collected | Derived from the per-agent documents | [research](../research/shared.md); cross-agent instruction and skill directories with no transcripts |
| [`shellgpt`](#shellgpt) | parsed | source; fixture only | [research](../research/shellgpt.md) |
| [`tabby`](#tabby) | parsed | source; fixture only | [research](../research/tabby.md) |
| [`twinny`](#twinny) | parsed | source; fixture only | [research](../research/twinny.md) |
| [`vscode`](#vscode) | parsed | source; fixture only | [research](../research/vscode.md) |
| [`windsurf`](#windsurf) | backlog | Official docs, community write-ups | [research](../research/windsurf.md); [issue 52](https://github.com/seanthegeek/doubleagent/issues/52); blocked on format research: Cascade's `.pb` files have no published schema |
| [`zed`](#zed) | parsed | source; fixture only | [research](../research/zed.md) |

Files that are not transcripts, such as skills and configuration files,
are detected but not parsed.

## Parsed agents

For each of these agents the analyzer reads the transcripts the collector
gathered and writes their events to the timeline: every prompt the user
typed and every reply, each tool call with its arguments and the result
that came back, thinking blocks when `--include-thinking` is passed, and
the records that say what the user authorised: permission decisions and
standing grants, the model in use and every switch, which sessions
were synced to the vendor's account, and which were relayed through a
messaging platform. Each row carries the host, user,
agent, session, project path and source file it came from, in the same
fields for every agent, so one query works across them all; the fields are
in [`analyzer/docs/output.md`](../analyzer/docs/output.md#timelinejsonl).
Where an agent keeps a second record of its use beside the transcript, a
prompt history, a search index or a mirror, the parser reads that too and
shows each event once, so a session the user deleted is still recovered
from what remains. Each subsection names the files the parser reads and
the installs on which they were seen.

### `agent-zero`

Files read: `usr/chats/<ctxid>/chat.json` (UI log for timestamps, each
agent's history for full text and model, `messages/<n>.txt` for long tool
results; history-only turns without timestamps when the log was trimmed at
1000 items) under `agent-zero`, `agent-zero/<instance>` or
`Desktop/agent-zero`, also the legacy `chats/<ctxid>.json` and
`tmp/chats`, or at the base of an `*a0_usr` or `a0-launcher-*-usr`
Docker or Podman volume (`chats/<ctxid>/chat.json` and the legacy
`chats/<ctxid>.json`); project path is the container path `/a0/usr/projects/<name>`.
From the same file, each agent's `data.responses_state` (a turn chained
on a provider-stored response marks the chat synced) and the chat's
Telegram, WhatsApp and email `data` keys (a relayed chat, so the parser
records relay state and `sessions.jsonl` `relayed` reads `yes` or `no`);
the bot configuration in `data.telegram_bot_cfg`, which holds its token,
is never read into a row. Also from the same file: permission decisions
from `warning` log items (a tool blocked by a policy, a provider MCP
approval denied automatically, an Infection Check verdict), an editor
(ACP) session's mode, model and container working directory, with its
workspace `data.acp_cwd` as the project path, and the per-chat model
override, printed without its API key. Beside the chats, in the same
install or volume: `usr/scheduler/tasks.json` (a
[secondary source](../analyzer/docs/output.md#secondary-sources): each
task's prompt and last result), the Telegram plugin's `state.json` (a
secondary source: a mapped chat that is gone gives a `session deleted:`
row), the `_tool_access` and `_orchestrator` `config.json` files and the
`.toggle-0` and `.toggle-1` switches of `_infection_check` and
`_orchestrator` at every plugin scope (standing permission settings);
and, in the home, the A0 CLI connector's `.agent-zero/.env`, of which only
the last chat's host and id are read (a chat on a remote instance, so the
parser records origin and `sessions.jsonl` `origin` reads `remote` or
`local`).

Validated against: Source at [e3051fb](https://github.com/agent0ai/agent-zero/commit/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec), synthetic fixture; the permission, model, editor-session, scheduler, Telegram map and plugin files from source at [ff2a5b9](https://github.com/agent0ai/agent-zero/commit/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a) and the connector's `.env` from agent0ai/a0-connector at [3741c88](https://github.com/agent0ai/a0-connector/commit/3741c8809f1f99b4efee2ed1a24234a98bf896d4), synthetic fixture.

### `aider`

Files read: `.aider.chat.history.md`, `.aider.input.history`,
`.aider.llm.history`, in a home or a repository (a project directory the
collector gathered); the input and LLM histories are secondary sources of
the chat history; times are host local time, emitted as if UTC. The
chat history's announcement and confirmation lines also give the `model:`
and `permission decision:` rows.

Validated against: Source at [5dc9490](https://github.com/Aider-AI/aider/commit/5dc9490bb35f9729ef2c95d00a19ccd30c26339c), synthetic fixture; the `model:` and `permission decision:` rows from source at the same commit (reviewed 2026-10-21), synthetic fixture.

### `antigravity`

Files read: `conversations/*.db` (SQLite of protobuf steps, with WAL
sidecars when present) and `conversation_summaries.db` under each of the
four data directories `.gemini/antigravity-cli` (the CLI),
`.gemini/antigravity` (the desktop app 2.x), `.gemini/antigravity-ide`
(the separately installed IDE) and `.gemini/antigravity-backup` (the app's
migration backup); `.gemini/antigravity-cli/history.jsonl` (CLI only; a
row's `session_id` is set when the line has `conversationId`, which CLI
1.3.x writes). The session start row names `source=<trajectory source>`
(`cli` for the CLI, `cascade_client` for the desktop app) and
`data_dir=<directory>`. Each grant in a step's `steps.permissions` blob is
a `system` row after the step's rows with its `tool_name` and
`tool_use_id`, `permission grant: action=<action> target=<target>
allow=<bool> scope=<scope>` (`<bool>` is `true` or `false`) or, for a file
access entry, `permission grant: path=<path> is_directory=<bool>
allow=<bool> scope=<scope>`. The standing permission files
`.gemini/config/config.json`, `.gemini/config/projects/*.json`,
`.gemini/config/sidecars/*/sidecar.json` and
`.gemini/antigravity-cli/settings.json` give one `permission setting:
<key>=<value> (from <file>, file mtime)` row per allow-list entry, sidecar
access grant, migration or sandbox flag and trusted workspace, at the
file's modification time; their other keys, `mcp_config.json` and plugin
`token.json` files are not read. The brain mirror under each data
directory, `brain/<uuid>/.system_generated/logs/` (one copy per
conversation: `transcript_full.jsonl`, else its
`chunks/transcript_full/*.jsonl` concatenated, else the shortened
`transcript.jsonl`, else its chunks), gives `user`, `assistant`,
`tool_use`, `tool_result`, `thinking` and `system` rows rendered as the
database's, with the line's `created_at`, the directory's `<uuid>` as
`session_id` and no `project_path`; a secondary source paired within 600
seconds, so a conversation whose database is gone is still in the
timeline. The brain directory's `steps/`, `messages/` and `tasks/` files,
`annotations/*.pbtxt` and the app's FTS5 search table are not read.

Validated against: Protobuf descriptors extracted from the shipped `agy`
binary (Linux x86-64) plus a real install (WSL), October 2026; a macOS
real install of CLI 1.2.16 and the desktop app 2.23.0, 2026-10-13, whose
embedded descriptors match the parser's tables; an Ubuntu 26.04.1
(aarch64) real install of CLI 1.3.4 and the desktop app 2.23.1,
2026-10-13; a Windows 11 ARM64 (build 26300) real install of CLI 1.3.5 and
the desktop app 2.23.1, 2026-10-14; the IDE and backup directories fixture
only (bundle-derived); the permission rows' file names and key names seen
on the macOS, Ubuntu and Windows installs, their values (scopes, actions,
grant patterns) fixture only; the brain mirror's file names and line keys
seen on the macOS, Ubuntu and Windows installs, its value forms from the
format description embedded in `agy` 1.3.6 (macOS arm64), the
`tool_calls[]` element shape fixture only.

### `chatgpt-desktop`

Files read: `.codex/sqlite/codex-dev.db` (SQLite with WAL sidecars), the
ChatGPT desktop app's own database, which the catalog attributes to
`codex-cli` because it sits under `~/.codex`: the rows of its
`local_thread_catalog` table whose `source_kind` is `chatgpt`. Each such
row is one of the account's ChatGPT conversations (a chat on chatgpt.com
or in the app's ChatGPT mode), which the app lists there at sign-in;
the conversation's text stays on OpenAI's servers. Each gives three
`system` rows at the row's `source_updated_at`, with `session_id` the
conversation id: `account conversation:` with the title,
`conversation_origin`, the creation and update times and the account's
`host_id`; `session origin: remote chatgpt.com conversation <id>`; and
`session synced: chatgpt.com holds conversation <id>, no local
transcript`, so the conversation's `origin` is `remote` and its
`synced` is `yes` in `sessions.jsonl`. The `vscode` rows of the table
are the app's Codex-mode threads, whose rollouts under `~/.codex` the
`codex-cli` parser reads. The collector also gathers the app's Chromium
profile (the directory Chromium keeps one user's state in), logs and
preferences, on Windows under the `OpenAI.Codex_*` package and on macOS
under `Library/Application Support/Codex`, `com.openai.codex` and the
computer-use service's group container; no parser reads them.

Validated against: Real installs. ChatGPT 26.1013.40934 on macOS 26.6.2
(2026-10-18): signing in wrote three `chatgpt` rows, the account's
conversations, dated 2026-10-15, before any chat on that machine; a
reply to one of them from inside the app wrote nothing locally and left
the row's times unchanged; deleting a Codex-mode thread with the app's
own command removed only that thread's `vscode` row. ChatGPT
26.1007.9446.0 on Windows 11 (2026-10-15): a plain chat and an image
generation left a title-only `chatgpt` row each and no transcript.
Synthetic fixture.

### `claude-code`

Files read: `.claude/projects/<slug>/<session>.jsonl`, subagent
transcripts under the session directory (each opens with a `subagent <id>
of <session>` row), `tool-results/*.txt` in the session directory when a
result's `<persisted-output>` stub names it, `.claude/history.jsonl` with
`.claude/paste-cache/<contentHash>.txt` to expand its paste placeholders,
and the per-process registry `.claude/sessions/<pid>.json` (one `system`
row per running process at `startedAt`: `claude-code process <pid> <kind>
status=<status> entrypoint=<entrypoint> bridge=<bridgeSessionId or none>
host_session=<hostSessionId or none>`, a `bridge=` value meaning the
process was under Remote Control when the collector ran and giving a
`session synced: remote control bridge <id> url - (from <path>, file
mtime)` row as well; the
`<pid>.<sha256>.key` sibling is never read); a 2.1.295
`permissionDecision` on a tool-result line is a `system` row after its
`tool_result` rows, `permission decision: <decision> source=<source>
reason=<reasonType>`, joined on the first result's `tool_use_id`;
`permission mode: <mode>` at a session's first `permission-mode` record or
`permissionMode` prompt key and `permission mode: <old> -> <new>` on each
change, `model: <id>` at the first assistant reply and `model: <old> ->
<new>` on each change (with ` requested=<requestedModel>` when it
differs), each subagent file tracking its own; standing permission
settings, one `system` row per entry at the file's modification time,
`permission setting: <key>=<value> (from <file>, file mtime)`, from
`permissions.allow`, `deny`, `ask`, `additionalDirectories`, `defaultMode`
and `disableBypassPermissionsMode` in `.claude/settings.json` and
`.claude/settings.local.json` (the user's, and a discovered project's with
`project_path` the project) and from `projects.<path>.allowedTools`,
`hasTrustDialogAccepted` and `hasClaudeMdExternalIncludesApproved` in
`.claude.json`; a `retention setting: cleanupPeriodDays=<n> (from
<file>, file mtime)` row when a settings file sets the retention
period, and a `source host: github status=<status> (from .claude.json,
file mtime)` row from `.claude.json` `githubWebConnectionStatusCache`
(2.1.310 and later; the account and organisation ids are not printed);
no other key of either file being read; the same files
under a `.claude-<name>` config home moved with `CLAUDE_CONFIG_DIR` (such
as `.claude-work/projects/...`). The transcripts also give a `session
surface: <entrypoint>` row per file (`cli`, `claude-vscode` for the VS
Code extension, which writes into the same tree, or `claude-desktop` for
the Code tab), a `prompt trigger: promptSource=<promptSource>
turnOrigin=<turnOrigin>` row after a prompt that was not typed by the
person in the console (a Remote Control prompt from a phone, the
extension's panel, an accepted suggestion, a task notification, a `/loop`
firing), a `session continued in: <continuedInSessionId>` row where a
session was handed to the session daemon's worker, `remote control:
bridge session <id or none> lastSequenceNum=<n>` rows and one `session
synced: remote control bridge <id> url <url>` row per bridge for Remote
Control (the URL from the `bridge_status` line), and a `scratch
directory: <path> (session scratchpad, from the environment attachment)`
row per session; a persisted tool output is found through
`toolUseResult.persistedOutputPath` when the line has one tool result.
The server form of Remote Control (`claude remote-control`) adds the
project's `.claude/projects/<slug>/bridge-pointer.json`, a `remote control:
environment <environmentId> session <sessionId> source=<source> (from
bridge-pointer.json, file mtime)` row, and a session's `ccr-tip.json`, a
`session synced: remote event <eventId> at <updatedAt> (from
ccr-tip.json)` row. A session gets a `session origin:` row: `remote
(Remote Control, entrypoint sdk-cli)` for one that server spawned,
`remote (teleport, remoteSourced)` for a cloud session brought in with
`--teleport`, `local (VS Code extension)` for the extension's. A refusal
recorded without a decision is `permission decision: reject (interrupted)
toolDenialKind=user-rejected`; a `/loop` firing is `scheduled task fired:
<taskId> cron=<cron>`; `settings.json` `model` gives `model setting:
<alias> (from settings.json, file mtime)`; and `.claude.json` gives
`remote control: machine id set`, `remote control: spawn mode <mode> for
<path>` and `cloud: hasRemoteEnvironment=<bool>` rows.

Validated against: Real installs, Claude Code 2.1.286 to 2.1.289 on Linux
(WSL2) and 2.1.295 on macOS (launched from the Claude Desktop Code tab),
and the shipped 2.1.289 package strings, October 2026; the
`sessions/<pid>.json` registry and `permissionDecision` on 2.1.295
installs on Linux and macOS (2026-10-11), with no `bridgeSessionId`
observed (the `bridge=` value is fixture only); the `permission-mode`
record, the `permissionMode` prompt key, `requestedModel` and the `/model`
output text on a 2.1.296 install on macOS (2026-10-13, key names and enum
values only), the settings `permissions` keys and mode names from the
2.1.296 darwin-arm64 binary's strings, the settings and `.claude.json`
rows fixture only. Claude Code 2.1.309 win32-arm64 from the native
installer on Windows 11 arm64, a clean VM, 2026-10-23, one step at a
time: the transcript layout under `projects\C--Users-<user>-...\`, the
`permissionDecision` forms of an automatic accept, a one-time approval,
an allow-always grant (a `PowerShell(...)` rule in the project's
`settings.local.json`) and a denial with a typed reason, the mode and
model changes, subagents, persisted output, compaction, the session
daemon's handover (`continued-in`), Remote Control from the iPhone app
(`bridge-session`, `promptSource` `queued`), `/loop`, the VS Code
extension's session (`entrypoint` `claude-vscode`, `promptSource` `sdk`),
`/clear` and a removed project; the rows new with this round (surface,
trigger, handover, bridge, scratch directory) rest on that install's key
names and are checked against fixtures, and a cloud session was not
offered to the account. Claude Code 2.1.310 linux-arm64 from the native
installer on Ubuntu 26.04.1 arm64, a clean VM, 2026-10-25 and 2026-10-26,
one step at a time: the transcript layout under
`projects/-home-<user>-...`, the same `permissionDecision` forms with the
`reasonCode` `bashMiss:no-rule-match` and an allow-always grant as a
`Bash(python3 *)` rule, the mode and model changes, subagents, persisted
output, compaction, `--resume` and `--continue` appending to the
session's own file with no session daemon, Remote Control from the phone
app, `/loop`, the VS Code extension's session, `/clear`, the retention
sweep deleting a transcript older than `cleanupPeriodDays` and keeping its
`history.jsonl` line, and a removed project; the `source host:` and
`retention setting:` rows rest on that install's key names and are
checked against fixtures. Claude Code 2.1.310 darwin-arm64 from the native
installer on macOS 26.6.2 arm64, a clean VM, 2026-10-26, one step at a
time: the transcript layout under `projects/-Users-<user>-...`, the
keychain item `Claude Code-credentials` of a sign-in from the desktop and
the `.credentials.json` of a session started over ssh, refusals recorded
with `toolDenialKind` and no decision, compaction, `--resume` and
`--continue`, the server form of Remote Control driven from claude.ai
(`bridge-pointer.json`, `ccr-tip.json`, `entrypoint` `sdk-cli`), a cloud
session created with `--cloud` and brought in with `--teleport`
(`remoteSourced`), `/loop`, the VS Code extension's session, `/clear`,
`/logout` and a removed project; the `session origin:`, `session synced:
remote event`, `remote control:`, `scheduled task fired:`, `model
setting:` and `cloud:` rows rest on that install's key names and are
checked against fixtures.

### `claude-desktop`

Files read: Under the app-data directory (`Library/Application
Support/Claude`, `.config/Claude`, `AppData/Roaming/Claude`,
`AppData/Local/Claude`, each also `-3p`, `AppData/Local/Claude-Data` and
its package-family child, and, for the MSIX build (MSIX, the Windows
packaged-app format),
`AppData/Local/Packages/Claude_*/LocalCache/Roaming/Claude` or
`AppData/Local/Packages/AnthropicPBC.Claude_*/LocalCache/Roaming/Claude`):
Cowork and Code-tab transcripts
`{local-agent-mode-sessions,claude-code-sessions}/<acct>/<org>/[agent/]<session
dir>/.claude/projects/<slug>/<cliSessionId>.jsonl` and their subagent
files, read with the `claude-code` reader, with `project_path` from the
session record's `userSelectedFolders[0]` (the JSONL `cwd` is a VM guest
path); `claude-code-sessions/<acct>/<org>/imported-staging/*.jsonl`; each
Cowork session's `audit.jsonl`, a secondary source read in full on every
run whether or not the session dir holds a transcript (every turn, an
`init` row, and permission requests, responses and auto-decisions and
`result` lines as `system` rows; the turns the transcript repeats are
dropped); session records `local_<uuid>.json` (one `system` row at
`createdAt`, on a Code-tab record ending with the persisted Remote Control
state `remote_control=<bool> bridges=<n> spawned_remotely=<bool>
steered_remotely=<bool>` and followed by a `remote control live at update
quit` row when `remoteControlLiveAtUpdateQuit.at` is set, on either kind
ending with `permission_mode=<permissionMode> effort=<effort>` and
followed by one `permission grant:` row per `alwaysAllowedReasons` and
`sessionPermissionUpdates` entry and per `chromePermissionMode`,
`chromeAllowedDomains`, `cuAllowedApps`, `cuGrantFlags` and
`cuFlagsGrantedAt` value, at `lastActivityAt` (else `createdAt`), the
first prompt as a `user` row (on a Cowork record always, with no
timestamp, a secondary source that drops when the transcript or the audit
log has the prompt; on a Code-tab record only when its
`~/.claude/projects` transcript, a `claude-code` file, was not collected),
a `session error:` row, and one `permission grant:` row per entry of the
Cowork grants `approvedToolNames`, `webFetchAllowedUrls`,
`egressAllowedDomains`, `userApprovedFileAccessPaths`, `enabledMcpTools`
and `fileDeleteApprovedMounts`); the audit log's `init` model as a `model:
<id>` row, and `model_refusal_fallback` lines as a `system` row of their
fields; `scheduled-tasks.json` (each task row ending
`permission_mode=<permissionMode> model=<model>`); `git-worktrees.json`;
`local-agent-mode-sessions/<acct>/<org>/remote-session-spaces.json` (one
`permission setting: entries.<sessionId>.folders=<folder> (from <file>,
file mtime)` row per folder granted to a remote Cowork session).

Validated against: App bundle 2.31226.1 (`app.asar`, read, not run); real
installs of 2.31226.1 for the Code-tab record and its transcript: MSIX
build (2026-10-10), Linux `.deb` (2026-10-11) and macOS (2026-10-11),
where every Remote Control flag was `false` or absent, so the `true`
values, the bridge count and the update-quit row are fixture only; Cowork
transcripts, `audit.jsonl`, imported transcripts and scheduled tasks
fixture only; the `permission grant:` rows fixture only (real installs
showed both grant lists empty).

### `cline`

Files read: `<editor>/User/globalStorage/saoudrizwan.claude-dev/` and
`.cline/data/`: `tasks/<id>/ui_messages.json` (preferred),
`api_conversation_history.json` (a fallback, only when `ui_messages.json`
is absent, since its texts wrap the prompt in tags the UI messages lack
and could not be paired with them; inherits the task start time),
`task_metadata.json`, `state/taskHistory.json`; SDK
`.cline/data/sessions/<id>/<id>.json` and `*.messages.json`,
`.cline/data/db/sessions.db`; `.cline/data/db/session-search.db` (a
secondary source: one `system` row per indexed session at its start, then
one row per message with no timestamp); the CLI's
`.cline/data/cache/user_input_history.jsonl` (a secondary source: one
`user` row per prompt, no session or project, only the last line
timestamped, since every rewrite stamps every line with its own time).
An SDK session's `enterprise.blobUpload` metadata key, in the manifest or
in `sessions.db` `metadata_json`, gives a `session synced:` row naming the
organisation's bucket and object (fixture only), and each item of the
VS Code extension's upload queue
`<editor>/User/globalStorage/saoudrizwan.claude-dev/cache/sync-queue.json`
one with its status, task file and retries (fixture only).
The SDK's hook audit log `.cline/data/logs/hooks.jsonl` (a secondary
source: the prompts, tool calls, results and final replies it repeats,
paired with the transcript on the call id, and a `hook audit:` row for
each run start, abort, error and stale-session shutdown); the standing
permission settings in `.cline/data/globalState.json`
(`autoApprovalSettings`, `mode`), `.cline/data/settings/global-settings.json`
and each server's `autoApprove` and `disabled` in
`settings/cline_mcp_settings.json`, one `permission setting:` row each;
a `permission decision:` row beside each SDK tool result the user
rejected or the policy blocked; the Cline Cloud sessions in
`globalState.json` `cloudSessionStatuses` and
`.cline/data/cloud-pending-starts/*.json` (`session origin: remote` and
`session synced:` rows) and the `cloudTaskTarget`; and an imported
session's source agent, session and path on its start row (all fixture
only).

Validated against: Source at [39ff2359](https://github.com/cline/cline/commit/39ff2359f7e08231281539696e48a166ce49270c), synthetic fixture; the hook audit log, the permission rows, the Cline Cloud rows and the import marker from source at [41e75efc](https://github.com/cline/cline/commit/41e75efc53e7abbcb76564b89e6de57f78659b33), synthetic fixture.

### `codex-cli`

Files read: `.codex/sessions/**/rollout-*.jsonl` and `archived_sessions/`,
also as `.jsonl.zst` (needs `zstandard`), `.codex/history.jsonl`,
`.codex/automations/*/automation.toml`, and the rollouts' SQLite
projections `.codex/state_5.sqlite` (`threads`: a thread record and the
first prompt) and `.codex/thread_history_1.sqlite` (`thread_turns`,
`thread_items`), secondary sources, with the `state_5.sqlite`
`remote_control_enrollments` row as a `remote control:` row;
`.codex/logs_<N>.sqlite` (`logs`), whose `/feedback` rows give one
`session synced:` row per report sent to Sentry (fixture only) and whose
approval rows give the ChatGPT desktop app's `permission decision:` rows;
and the desktop app's `.codex/.codex-global-state.json`, a secondary
source whose prompt history, deleted-thread markers, thread descriptions,
standing permissions and remote control flags outlive a deleted thread.
From source (fixture only): `.codex/config.toml` and `.codex/rules/*.rules`
(the standing grants, one `permission setting:` row each), the
`guardian_review_feedback` records of `state_5.sqlite` and the
`PatchApproval` rows of `logs_<N>.sqlite` (`permission decision:` rows),
`.codex/memories/rollout_summaries/*.md` (one `memory summary:` row per
thread) and `.codex/db-backups/*/state_5.sqlite` (a secondary source).

Validated against: Real install, Codex CLI, October 2026; Codex Desktop
0.162.0-alpha.17.2 inside the ChatGPT desktop app, Windows 11, October
2026; Codex CLI 0.162.1 (standalone installer) on Windows 11, October
2026; Codex Desktop 0.162.0-alpha.17.2 inside ChatGPT 26.1007.71326 and
Codex CLI 0.162.1 on macOS 26.6.2, October 2026; Codex CLI 0.162.1
(standalone installer) on Ubuntu 26.04.1, October 2026; Codex Desktop
0.163.0-alpha.10.2 inside ChatGPT 26.1013.40934 on macOS 26.6.2,
2026-10-18 (settings events, turn triggers, approval log rows, a
guardian review thread, remote control from iPhone and Android, a
scheduled task and a deleted thread); archived and
`.zst` rollouts, subagent and fork records from source at [3e23877](https://github.com/openai/codex/commit/3e238776e857eccd3bde6bff3026e2e9798f6524),
synthetic fixture; the standing grants, failed guardian reviews, patch
approvals, thread goals, memory summaries and database backups from source
at [4e7b9e2](https://github.com/openai/codex/commit/4e7b9e285ba4ac6ce18ca452cf9e88d3d75ec857), synthetic fixture; the `state_5.sqlite` and `thread_history_1.sqlite`
table and column names seen on the Windows 11 install, their row shapes
from source at [3e23877](https://github.com/openai/codex/commit/3e238776e857eccd3bde6bff3026e2e9798f6524), synthetic fixture. User-role context (environment,
AGENTS.md, skills) is `system` by its content kind; a subagent row links
each spawned thread to its parent; each automation of the ChatGPT desktop
app is a `scheduled task:` row; turn items that no response item carries
become rows; each turn's trigger (the composer, a paired phone, a
scheduled task, the guardian reviewer) is a `turn trigger:` row.

### `cody`

Files read: The `sourcegraph.cody-ai` row of a VS Code or fork
`User/globalStorage/state.vscdb` (its `cody-local-chatHistory-v2` member)
and the JetBrains file
`Cody-nodejs/[Data/]JetBrains-globalState/cody-local-chatHistory-v2`.
Every row of a chat carries the chat's creation time (its id); no project
path is recorded. The standing permission settings (`cody.*` keys of the
editor's `User/settings.json`), the model preferences
(`cody-model-preferences` in the same database row and
`Cody-nodejs/[Data/]JetBrains-globalState/cody-model-preferences`), and
the JetBrains plugin's pre-agent history `<project>/.idea/cody_history.xml`
as a secondary source.

Validated against: Source at [8e20ac6c](https://github.com/sourcegraph/cody-public-snapshot/commit/8e20ac6c1460c08b0db581c0204658112a246eda), synthetic fixture; the settings, model preferences and legacy JetBrains history from source at the same commit (2026-10-21 review), synthetic fixture.

### `continue`

Files read: `.continue/sessions/<sessionId>.json` (timestamped from
`sessions/sessions.json` `dateCreated`; messages have no time of their
own), `.continue/dev_data/<schema>/chatInteraction.jsonl`,
`toolUsage.jsonl`, `editInteraction.jsonl`, `editOutcome.jsonl` and
`chatFeedback.jsonl` (secondary); `.continue/logs/cn.log` and its rotated `cn<N>.log`
files, whose `cn serve` metadata posts, skipped storage uploads and added
permission rules are `agent metadata posted:`, `storage upload requested,
skipped` and `permission decision: allow always` rows, timestamped in
host-local time (fixture only); the CLI's `.continue/permissions.yaml` and
`.continue/input_history.json` (secondary).

Validated against: Source at [5522c6f](https://github.com/continuedev/continue/commit/5522c6f44ca0ac3528b37244818fbfa39b5af470), synthetic fixture; the standing grants, denial reasons, session mode, prompt history, edit and feedback events and `cn review` scratch directories from the same commit (2.0.0, reviewed 2026-10-21), synthetic fixture.

### `copilot-cli`

Files read: Under `.copilot` (or the pre-XDG `.local/state/.copilot` and
`.config/.copilot`): `session-state/<id>/events.jsonl` (the event log;
tool calls, results and the permission requests and decisions joined on
`toolCallId`; `model:` rows from `session.model_change`;
`session.info <infoType>: <message>` rows, such as the confirmation a
`/model` switch writes; the runtime's utility-model calls as
`utility model <id>:` rows; `hook <hookType> start:` and `end:` rows
for the hook events),
`session-state/<id>/workspace.yaml` (session name and metadata, with the
`client_name` that tells the CLI from the desktop app),
`permissions-config.json` and `settings.json` (`allowedUrls`,
`deniedUrls`; one `permission setting: ... (from <file>, file mtime)` row
per entry at the file's modification time), `logs/process-*.log` (the
sign-in line), and `sidebar-sessions-state/*.json` (a project-path join,
no rows). Secondary sources: `session-store.db` (SQLite with WAL sidecars;
`sessions`, `turns`, `session_files`, `session_refs`, `checkpoints`,
`forge_trajectory_events`; one-hour window, since a turn is stamped when
it ends), `session-state/<id>/rewind-file-snapshots/index.json` (each
turn's prompt and changed files; one-hour window) and
`command-history-state.json` (prompts with no time, session or cwd).

Validated against: Shipped bundle 1.0.98 (darwin-arm64 event schema, CLI
and runtime strings) and real installs of Copilot CLI 1.0.98 on macOS
(2026-10-15) and Ubuntu 26.04 arm64 (2026-10-16), and on Windows 11 ARM64
(2026-10-17) the standalone CLI 1.0.98 installed with winget and the CLI
1.0.99-2 that GitHub Copilot Desktop 1.1.30 bundles, not an npm install,
synthetic fixture; `forge_trajectory_events`, checkpoints, references,
denials, resumes and subagents fixture only (the
[Windows winget round](../research/copilot-cli.md#observed)
exercised a denial, a resume and subagents, but its session was deleted
before collection, so only the shapes were seen). The GitHub
Copilot Desktop app's sessions, which its bundled Copilot CLI writes into
`.copilot/session-state/`, are read by this parser and marked by
`client=github/autopilot` on the `session metadata:` row. The
`workspace.yaml` keys `remote_steerable`, `mc_task_id`, `mc_session_id`
and `mc_last_event_id` mark a session synced to the account (added at a
resume on the Windows winget install, present in every Copilot Desktop
session); since analyzer 0.14.0 the parser yields a `session synced:
remote session <id> task <id> last event <id> remote_steerable=<value>
(from workspace.yaml, file mtime)` row for them, `sessions.jsonl` ends
each record with a `synced` field (`yes`, `no`, or empty when the agent's
parser records no sync state) and `inventory.json` counts them in
`synced_sessions`
([issue 151](https://github.com/seanthegeek/doubleagent/issues/151)).

### `copilot-desktop`

Files read: `.copilot/data.db` (SQLite with WAL sidecars), one `system`
row per record of `accounts` (`account:`, with whether a token is
present, never the token), `settings` (`desktop settings:`, at the file's
modification time), `model_providers`, `projects` (`project added:`, with
`project_path` the clone's `main_repo_path`), `worktrees`, `workspaces`
(with the source and created pull requests), `sessions` (`desktop
session:` with the model, `auto_approve`, `permission_mode`, Remote
Control and archive state, `project_path` through
`workspace_session_aliases` and `workspaces`, and a `forked from session`
row for a fork), `activity_items`, `session_checkpoints`, the scheduled
tasks with their triggers and runs (`my_copilot_scheduled_tasks`,
`my_copilot_scheduled_task_triggers`,
`my_copilot_scheduled_task_invocations`), `session_automations` and
`session_deletion_intents`. The checkpoint refs
`.copilot/repos/<name>/.git/refs/copilot/checkpoints/<session id>/<seq>/<event id>`,
one `checkpoint ref <seq>: commit <hash> event <event id> (file mtime)`
row per ref file, at its modification time; they outlive a session
deleted from the app. The app's session logs under
`.copilot/session-state/` are read by the `copilot-cli` parser and share
`session_id` with these rows.

Validated against: Shipped app (`/usr/bin/github` strings and the bundled
SDK declarations) and a real install of GitHub Copilot Desktop 1.1.29 on
Ubuntu 26.04 arm64 (2026-10-16): the `data.db` tables and columns, and
the account, settings, provider, project, worktree, workspace, session,
activity and checkpoint rows, seen there, synthetic fixture; scheduled
tasks, triggers, runs, session automations, deletion intents, forks and
pull request links fixture only (empty on the install). The checkpoint
refs were seen on the same install after both sessions were deleted
from the app (2026-10-16). A real install of GitHub Copilot Desktop
1.1.30 on Windows 11 ARM64 (2026-10-17), with the bundled CLI
1.0.99-2, showed the same parsed tables (104 tables in all, `PRAGMA
user_version` 194) and gave the same kinds of row, a checkpoint ref
included;
after a general chat was deleted from the app its activity rows
survived, and quitting the app checkpointed the WAL into `data.db`.

### `crush`

Files read: `<project>/.crush/crush.db` (SQLite with WAL sidecars; reached
as a project artifact, or under a home when Crush ran in `~`),
`.local/share/crush/projects.json`, `AppData/Local/crush/projects.json`;
every `crush.json` scope (`.config/crush/crush.json`,
`.local/share/crush/crush.json`, `AppData/Local/crush/crush.json`,
`.crush/crush.json`, a project's `crush.json` and `.crush.json`) for its
standing permission entries only, so provider keys are never emitted; and
the `permissions` lines of `.config/crush/crushrc` and a project's
`crushrc` and `.crushrc`.

Validated against: Source at [ca6ae26](https://github.com/charmbracelet/crush/commit/ca6ae26ce016b980407ce32f012a467ec10c1e2f), synthetic fixture; the permission, relay, deletion and file rows and the standing settings from source at [7f799fb](https://github.com/charmbracelet/crush/commit/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da), with fantasy v0.45.1 at [ec25857](https://github.com/charmbracelet/fantasy/commit/ec258577c7d825ec38cfdf8cbeaaa2f7e197aacd) for the stored hook metadata, synthetic fixture.

### `cursor`

Files read: `User/globalStorage/state.vscdb` under the editor's data
directory (`Library/Application Support/Cursor`, `.config/Cursor` or
`AppData/Roaming/Cursor`; SQLite with WAL sidecars): the
`cursorDiskKV` composer rows (`composerData:<composerId>`, the header
list inline or in `composerConvHeaders:<composerId>:<NNNNNN>` chunks,
and one `bubbleId:<composerId>:<bubbleId>` row per user message,
reasoning or reply sub-bubble and tool call), giving `user`,
`assistant`, `thinking`, `tool_use` and `tool_result` rows, a
`permission decision:` row for each command the user was asked about
(Run, Skip or Always run, joined on the call id), a `permission prompt
pending:` row for a call still waiting at its prompt, `model:` rows from
each user message's model, `session name:`, `conversation archived` and
`unsent draft:` rows; the `composerHeaders` table, whose row outlives a
deleted conversation and gives its `conversation deleted` row, with the
JSON `agentKv:blob:*` prompts that name the deleted composer recovered
as `user` and `assistant` rows; and the `ItemTable` settings blob, one
`permission setting:` row per run mode, allowlist or denylist entry and
switch, at the file's modification time, one `model last used:` row
per model, a `source host: github account <login>` row from the
editor's own GitHub sign-in (`github-<login>` keys, at their last use)
and a `remote agent host enrolled:` row when the machine is enrolled as
a remote agent host, which remote control does by default; for each
cloud agent in a `cloudAgentRepository.agents.*` list a `session origin:
remote cursor cloud agent`, a `session synced:` row, a `cloud agent:`
status row, a `source host:` row for its
repository and a `model:` row, and a `source host repository:` row per
entry of `cloudEnvironmentTemplate.githubRepos`; a `permission
decision:` row for a cloud agent's request to authorise an MCP server
(`cloudAgentRepository.mcpAuthResolution`); an `mcp server:` row per
plugin MCP server from the `mcpOAuth.global.*` keys, saying when its
tokens are stored, and a `plugin installed:` row per
`cursor.plugins.installedIds.*` entry; the cached GitHub token, the
`mcpOAuth.secret.*` values and the cloud agent's connection token are
never read.
`User/globalStorage/conversation-search.db` (`conversation index:`
rows, and for a cloud agent's `cloud-cache` row its origin and its
cached conversation text from `conversation_fts_content`),
`User/globalStorage/mcp-oauth-attempts/<uuid>.json` (`mcp oauth
attempt:` rows),
`.cursor/ai-tracking/ai-code-tracking.db` (`ai code:`,
`conversation summary:`, `file deleted by agent:` and `scored commit:`
rows), `.cursor/cli-config.json` and `.cursor/permissions.json` in the
home or a project (`permission setting:` rows). Secondary source:
`.cursor/projects/<sanitized cwd>/agent-transcripts/<composerId>/<composerId>.jsonl`,
the agent worker's copy of each conversation, which survives a delete
(one-minute window, since its user lines are stamped to the minute). A
standalone Agent CLI session is read through the files it shares with
the IDE: its transcript JSONL under `.cursor/projects`, its `source`
`cli` rows of `ai-code-tracking.db` and `cli-config.json`. Its own chat
store, `.cursor/chats/<md5 of cwd>/<sessionId>/store.db` with
`meta.json` and `prompt_history.json`, is not read yet
([issue 235](https://github.com/seanthegeek/doubleagent/issues/235)).

Validated against: Real install of the Cursor IDE on macOS 26.6.2
(builds 3.24.18, 3.25.9 and 3.26.13, 2026-10-19 and 2026-10-20): the
`state.vscdb` composer, bubble, header and settings records, the 3.26.13
header chunks, the agent transcripts, `conversation-search.db` and the
`ai_code_hashes` rows of `ai-code-tracking.db`, and the records an
archive and a delete left; synthetic fixture. `permissions.json` and a
non-local `agentLocation` (bundle-derived) and rows of the
`conversation_summaries`, `ai_deleted_files` and `scored_commits` tables
(empty on the install) are fixture only. Real install of the
standalone Cursor Agent CLI on macOS 26.6.2 (2026.10.19-6bbf2c4,
2026-10-20): its transcript JSONL, its `ai-code-tracking.db` rows and
the `permissions.allow` entries of `cli-config.json`; its chat store,
observed on that install, is not parsed. Real install of the Cursor IDE
on Ubuntu 26.x arm64 (the deb package 3.24.18, upgraded through apt to
3.25.9, 2026-10-20): `state.vscdb` under `.config/Cursor` with the same
composer, bubble, header and settings records as macOS 3.25.9, a tool
call left waiting at its permission prompt, the editor window's
allow-always form (`selectedOption` `run` with
`appliedAllowlistOptionIds`), the agent transcripts, `ai_code_hashes`
rows, the records an archive and a delete left, and the keys of the
editor's GitHub sign-in and of the remote agent host enrolment; a
repository-bound cloud conversation was not observed there. Real
install of the Cursor IDE on Windows 11 arm64 (the per-user installer
3.26.13, 2026-10-21): `state.vscdb` under `AppData/Roaming/Cursor` with
the same records as macOS 3.26.13, a call waiting at its prompt
(`additionalData.status` `pending`), the standing allow list, the
agent's injected user turns, the delete of a conversation never
archived, the editor's GitHub sign-in, and a cloud agent on a repository
mirrored into Cursor Origin: its `cloud-cache` row and cached text in
`conversation-search.db`, its `cloudAgentRepository.agents` record and
the `cloudEnvironmentTemplate.githubRepos` repository list, archived
from the Agents window; and the Google Calendar plugin's MCP OAuth
keys, installed-plugin keys and attempt file, its MCP tool calls, and a
second cloud agent's MCP authorisation record. The cloud agents'
`bcCachedDetails` protobuf is not decoded. A turn steered from a paired phone is not observed: pairing
wrote nothing locally, and the embedded agent worker does not start on
Windows arm64. The Agent CLI on Linux and Windows is not observed. The CLI's
`acp-sessions/` store rests on the shipped bundle alone, and the parser
does not read it.

### `gemini-cli`

Files read: `.gemini/tmp/<slug>/chats/**/*.jsonl` (with `$set`, `$patch`
and `$rewindTo` replayed), legacy `chats/**/*.json`,
`tmp/<slug>/logs.json`, also under `.cache/.gemini`; project path from
`.project_root` or `projects.json`; `.gemini/trustedFolders.json`, one
`permission setting: trustedFolders.<path>=<value> (from <file>, file
mtime)` row per folder at the file's modification time with `project_path`
the folder; a gemini message's `model` as `model: <id>` at a session's
first and `model: <old> -> <new>` on each change; a tool call with status
`cancelled` or `awaiting_approval` is followed by `permission decision:
<status>`; `tmp/<slug>/shell_history` (the shell-mode commands),
`tmp/<slug>/checkpoint-<tag>.json` (`/chat save`) and
`tmp/<slug>/bug-report-history-<ms>.json` (`/bug`) as secondary sources
with no timestamp; `.gemini/settings.json` (permission, sandbox and
retention keys) and `.gemini/policies/*.toml` (saved and written policy
rules) in the home and in a project, `.gemini/trusted_hooks.json` and
`.gemini/acknowledgments/agents.json`, one `permission setting:` or
`session retention:` row per entry at the file's modification time.

Validated against: Source at [fb972b2](https://github.com/google-gemini/gemini-cli/commit/fb972b2f87fe7d5b06d37eac711490162d98de2c), synthetic fixture; the standing permission files, `shell_history`, the `/chat save` checkpoints and the `/bug` exports from source at [3be3d44](https://github.com/google-gemini/gemini-cli/commit/3be3d4424b3c8a760d793ae1bea6cb5180adf27d) (0.66.0-nightly), synthetic fixture.

### `goose`

Files read: `.local/share/goose/sessions/sessions.db` (SQLite with WAL
sidecars), legacy `sessions/*.jsonl`,
`.local/state/goose/logs/llm_request.*.jsonl` (a secondary source: the
last 10 provider calls, matched within 600 seconds), `history.txt` (a
secondary source, no timestamp: matches on text alone),
`.config/goose/permission.yaml` and `.config/goose/roaming_trust.json`
(standing permission rows), `.local/share/goose/schedule.json` (one
`scheduled task:` row per job), and the `AppData/Roaming/Block/goose/data`
and `AppData/Roaming/Block/goose/config` equivalents. A `sessions.db` session
whose `session_type` is `gateway` was relayed through the gateway's
Telegram bot, so the parser records relay state and `sessions.jsonl`
`relayed` reads `yes` or `no`; `config.yaml` `gateway_pairings` is not
read, nor the desktop app's prompt history in its local storage.

Validated against: source at [b23d21c](https://github.com/aaif-goose/goose/commit/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc) (1.57.0), synthetic fixture; the `sessions.db` fixture keeps the schema 16 columns of [591edd4](https://github.com/block/goose/commit/591edd47cf2cfea4957d720c607cf2a4def8673d). No real install.

### `hermes`

Files read: `state.db` (SQLite with WAL sidecars; `sessions`, `messages`
and `session_model_usage`, rewound or compacted `active = 0` rows kept
with a `[rewound]` prefix; also the `state_meta` deletion fences,
`conversation_generations` and the Bot Mode room log) and these secondary
sources: the quick snapshots `state-snapshots/<id>/state.db` (read like
`state.db` after a `state snapshot <id>` row; collected only with
`--full` before collector 1.18.0), the `sessions/<id>.jsonl` diverts
written when `state.db` was replaced under a running process, the prompt
history `.hermes_history`, the cron run files `cron/output/<job>/<time>.md`,
the API server's `response_store.db`, and the `trajectory_samples.jsonl`
and `failed_trajectories.jsonl` files a run leaves in its project; all
under `.hermes`, `.hermes<suffix>`, `AppData/Local/hermes`,
`AppData/Local/hermes<suffix>` and their `profiles/<name>/` homes, or at
the root of a `*hermes*` Docker or Podman volume, the trajectory files
excepted. Also `pastes/pending.json`, the
paste.rs links of recent `hermes debug share` uploads; the standing
approvals in `config.yaml` (`command_allowlist`) and in the pairing
stores `pairing/<platform>-approved.json` and
`platforms/pairing/<platform>-approved.json`; and the desktop app's
`connections.json` in its `Hermes` or `Hermes<suffix>` userData directory.

Validated against: source; fixture only. Source at [6bb377a](https://github.com/NousResearch/hermes-agent/commit/6bb377a51d5c8e279a7a5e34d717d653d421d064) (after release `v2026.9.24`), the schema 31 shapes at [8b66a51](https://github.com/NousResearch/hermes-agent/commit/8b66a51036c1e20920a17cdd049fdf55c968d683); no real install.

### `kilo-code`

Files read: `.local/share/kilo/kilo*.db` and `opencode-*.db` plus the same
JSON trees under `.local/share/kilo` (OpenCode parser, Kilo paths,
including the `session_share` table and `session.updated.1` events),
the cloud session markers
`.local/share/kilo/storage/session_share/<sessionID>.json` (one
`session synced:` row each), and pre-migration VS Code tasks
`<globalStorage>/kilocode.kilo-code/tasks/<id>/api_conversation_history.json`;
the terminal UI's `.local/state/kilo/prompt-history.jsonl` and `session.json`
(pinned sessions, one `session pinned:` row each; both secondary) and
`prompt-stash.jsonl`; the `permission` key of the global config files in
`.config/kilo`; the sandbox files under `.local/state/kilo-sandbox-policy`
and `.local/state/kilo-sandbox-preference`; `.local/share/kilo/retention/state.json`;
the project memory session digests
`.local/share/kilo/memory/<folder>/sessions/<file>.md` (one `memory digest:`
row each, with the folder's `manifest.json` as the project);
and the remote control lines of `.local/share/kilo/log/*.log`.

Validated against: Source at [0a9faef](https://github.com/Kilo-Org/kilocode/commit/0a9faeff69fff8712da184ee5188bb7985c6f057) (7.8.25), synthetic fixture; no real install. Earlier source reviews: [76bcfd4](https://github.com/Kilo-Org/kilocode/commit/76bcfd40be616a72f4697b3041565f322245b462) (7.8.3) and [87cc35c](https://github.com/Kilo-Org/kilocode/commit/87cc35ce6904465a0eed507a8c75c79185863a22) (7.8.14).

### `kiro`

Files read: Amazon Q CLI `data.sqlite3` (`conversations` and legacy
`history` tables) under `amazon-q/` or `kiro-cli/` in `.local/share`,
`Library/Application Support` or `AppData/Local`; `/save` exports (`.json`
with `conversation_id` and `history`), including each conversation's
checkpoint snapshots and tangent-mode main history for the turns the
conversation no longer holds; the chat prompt history
`.aws/amazonq/.cli_bash_history` (a secondary source with no time,
session or project); the agent configurations
`.aws/amazonq/cli-agents/*.json` and a project's
`.amazonq/cli-agents/*.json` (standing tool grants); a project's
`.amazonq/.subagents/*.json` (delegated background tasks). Kiro CLI
`.kiro/sessions/` files are not parsed until their format is confirmed.

Validated against: Source at [15cc8f3](https://github.com/aws/amazon-q-developer-cli/commit/15cc8f3cd18c4272925ce1c7053268eedff1ea0a), synthetic fixture; the denial rows, standing grants, prompt history, history copies and delegate records from source at the same commit and [kkawakam/rustyline at bc06b3e](https://github.com/kkawakam/rustyline/commit/bc06b3eb322df1a535e6e3b64db8c0fee0b1df8c) (v15.0.0, the prompt history format), synthetic fixture.

### `letta`

Files read: Letta Code
`.letta/lc-local-backend/conversations/<key>/messages.jsonl` (pi session
format), `.letta/transcripts/<agent>/<conversation>/transcript.jsonl`
(project path from `sessions.jsonl` by time, a heuristic; a secondary
source, read beside the conversation's `messages.jsonl` with a one-hour
window because each line is stamped at the end of its turn; a
server-side agent's transcript also gives a `session synced:` row),
`.letta/sessions.jsonl` (one line per session ended with `/exit` or
`/logout`), and the `sessionsByServer` entries of `.letta/settings.json`
and a project's `.letta/settings.local.json` (one `session synced:` row
per server; `agents[]` `baseUrl` names a transcript's server). Since the
2026-10-21 review, source-derived: the `permissions` object of
`.letta/settings.json`, `.config/letta/settings.json` and a project's
`.letta/settings.json` and `.letta/settings.local.json` (one
`permission setting:` row per entry),
`.letta/channels/<channel>/routing.json` (one `session relayed:` row per
route), `.letta/crons.json` and `.letta/runs/<task id>.jsonl` (scheduled
tasks and their runs), and `.letta/listener-state/<hash>/<agent>_<conversation>.json`
(an interrupted listener turn with its approval decisions and tool
results). No other settings key is read. On the server backend the
store of record is on the server and the local transcript is its
mirror, so such a session reads `sessions.jsonl` `recovered` `yes` or
`partial` by design, beside its `session synced: server conversation`
row.

Validated against: Source at [77faf36](https://github.com/letta-ai/letta-code/commit/77faf36e34378946c8e69c0bfaa5595a7be4c326), synthetic fixture; the permission settings, channel routing, scheduled tasks and interrupted turns from source at [2a46615](https://github.com/letta-ai/letta-code/commit/2a46615d16babb06cf5450b8e57ecb9bdbf2f436) (version 0.34.17), synthetic fixture.

### `little-coder`

Files read: `.pi/agent/little-coder-prompt-history.json` (prompts without
timestamps or sessions) and `.little-coder/checkpoints/<session file>/`
(one `system` row per session's pre-edit copies); its transcripts are pi
sessions, parsed as `pi`. From the retired v0.0.x Python releases:
`.little-coder/sessions/daily/<date>/session_*.json` and the `/save`
files under `.little-coder/sessions/` (saved sessions, timed by their
save time in host local time), `sessions/history.json` and
`sessions/mr_sessions/session_latest.json` (secondary copies of them),
`.little-coder/input_history.txt` (secondary, prompts without
timestamps), `.little-coder/checkpoints/<id>/snapshots.json` (one
`system` row per snapshot) and three keys of `.little-coder/config.json`
(the gist sync, the Telegram relay and the permission mode).

Validated against: Source at [89d4fa0](https://github.com/itayinbarr/little-coder/commit/89d4fa0af864230527ab75d12604ed0eb320e6df), synthetic fixture; the v0.0.x layout from source at [424f7a4](https://github.com/itayinbarr/little-coder/commit/424f7a4b9f9214a36a6bcb69e1f3435ca3e68d03) (tag `v0.0.5`), synthetic fixture.

### `muse-code`

Files read: `.local/share/muse/sessions/YYYY/MM/DD/<id>/session.jsonl` and
each subagent's `subagent/<id>/session.jsonl` (event envelopes with
microsecond `recorded_at`; tool calls and results joined on `call_id`;
approval requests and decisions as `system` rows; a subagent takes its
project path and branch from the parent log),
`.local/share/muse/tui-history.jsonl` (prompts without timestamps),
`.config/muse/trust.json` (one `permission setting:` row per trusted
project, at the file's modification time). The approval reviewer's
`approval-review/*.jsonl` (synthetic clock), `tool-outputs/` and the
SQLite indexes are not read.

Validated against: Shipped binary 1.4.2 strings, vendor skills and a real
install (Linux), October 2026, synthetic fixture.

### `nanobot`

Files read: `.nanobot*/sessions/<workspace-id>/sessions.sqlite3`, the
session database of newer builds (tables `sessions` and `messages`, with
its `-wal` sidecar), and `.nanobot*/sessions/<workspace-id>/<key>.jsonl`
(project path from the sibling `.workspace`), legacy `sessions/*.jsonl`
and `.migration-conflicts/`, and `memory/history.jsonl`; times are host
local time, emitted as if UTC. Secondary sources: the session JSONL files
and `jsonl-backup/*.jsonl` copies beside a `sessions.sqlite3`, which the
database imported and which are never written again; the WebUI transcript
`.nanobot*/webui/<key>.jsonl` and `<key>.segments/NNNNNN.jsonl` (true UTC
`created_at_ms`, 15-hour window); `.nanobot*/history/cli_history`
(local time, prompts only); the cron and trigger run records
`cron/runs/*.json` and `triggers/runs/*.json` of a workspace (true UTC,
15-hour window); and the WebUI sidebar cache
`.nanobot*/sessions/<workspace-id>/.webui_session_index.json`. Settings:
`.nanobot*/config.json` (the workspace restriction, the exec sandbox and
its allow and deny lists, the channel `allowFrom` lists; no other key) and
`.nanobot*/pairing.json` (approved and pending chat senders), one
`permission setting:` row per entry.

Validated against: Source at [acdae3d](https://github.com/HKUDS/nanobot/commit/acdae3d0ae2714b6dde672428e921dbf705c096f), synthetic fixture. The session database and the `--home` history
location are fixture only, from source at
[909b277](https://github.com/HKUDS/nanobot/commit/909b277e7a5a7ecd2aebdba9611e5febe86988d9),
which may not be released yet; the relay, model switch, permission, run
record and sidebar cache rows from source at
[909b277](https://github.com/HKUDS/nanobot/commit/909b277e7a5a7ecd2aebdba9611e5febe86988d9),
synthetic fixture.

### `ollama`

Files read: The desktop app's `Library/Application
Support/Ollama/db.sqlite` (macOS) and `AppData/Local/Ollama/db.sqlite`
(Windows), SQLite with WAL sidecars, schema versions 16 to 19: `messages`
with their `tool_calls` (no call id: the n-th `tool` message after an
assistant message pairs with its n-th call, `tool_use_id` is `<chat
id>:<tool_calls.id>`) and attachment names and sizes, never their bytes;
the `users` and `settings` tables are not read. `.ollama/history` (the
`ollama run` prompt history: no timestamp or session, so its rows sort
last and drop out of `--since`/`--until`; the last 100 entries only; a
pasted multi-line prompt becomes several rows; slash commands are `system`
rows). `.ollama/config.json`, for the `automode` key of each `ollama
launch` integration, and the Claude Desktop profile the launcher writes,
`configLibrary/00000000-0000-4000-8000-000000000114.json` under
`Library/Application Support/Claude-3p` or `AppData/Local/Claude-3p`, for
its `autoModeEnabled` key: each a `permission setting:` row at the file's
modification time. No project path is recorded. Model manifests,
`backup/`, `launch/` and `logs/` are not read.

Validated against: Source at v0.35.1 ([b0c1ca4](https://github.com/ollama/ollama/commit/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce)), synthetic fixture; no real
`db.sqlite`, since the desktop app does not exist on Linux; the auto-mode
settings from source at [6787222](https://github.com/ollama/ollama/commit/678722224b3d8c2a024a4fa5f530d83ce34db120) (after v0.40.6), synthetic fixture.

### `open-interpreter`

Files read: Codex rollouts (Codex parser, Open Interpreter paths):
`.openinterpreter/sessions/**/rollout-*.jsonl` and `archived_sessions/`,
also as `.jsonl.zst`, `.openinterpreter/history.jsonl`,
`.openinterpreter/state_5.sqlite` and
`.openinterpreter/thread_history_1.sqlite` (secondary sources, read as for
Codex), and `external_agent_session_imports.json` (one `system` row per
`/import`ed thread, naming the Claude Code or Cursor source file; that
thread's record times are import time), and `.openinterpreter/logs_2.sqlite`
(`/feedback` reports, as for Codex; fixture only). As for Codex, from
source: `.openinterpreter/config.toml` and `.openinterpreter/rules/*.rules`
(standing grants), `.openinterpreter/memories/rollout_summaries/*.md` and
`.openinterpreter/db-backups/*/state_5.sqlite`.

Validated against: Source at [2767e5f](https://github.com/openinterpreter/openinterpreter/commit/2767e5f20d6927500b8f1938c773c61afb823245), synthetic fixture; the standing grants, memory summaries and database backups from source at [ac1fcc8](https://github.com/openinterpreter/openinterpreter/commit/ac1fcc8fe57aba51fcc3c79afd57a0883237fe87), synthetic fixture.

### `openclaw`

Files read: `agents/<id>/agent/openclaw-agent.sqlite` (SQLite with WAL
sidecars: `transcript_events`, zstd `event_zstd` rows need `zstandard`;
every reset/deleted archive blob, published or not, and SQLite cold
archives, whole or in `session_transcript_cold_archive_chunks`), legacy
`agents/<id>/sessions/<id>.jsonl` and `sessions/*.jsonl`,
`sessions/cold/*.jsonl.zst`, and, as secondary
sources, the published reset and deleted archive files `*.jsonl.reset.*` /
`*.jsonl.deleted.*` (optionally `.zst`), compaction checkpoints
`*.checkpoint.<uuid>.jsonl` and pre-doctor repair backups
`*.jsonl.pre-doctor-*-repair-*.bak`, under `.openclaw`,
`.openclaw-<profile>`, `.clawdbot` or `.moltbot`. The session key (channel
and sender) is in each session's start row; `auth_profile_store` is never
read; `*.trajectory.jsonl` and `*.migrated*` are not read. The agent
database's `session_nodes.entry_json` gives each published session's
`publicShare` grant, and `state/openclaw.sqlite` table `backup_runs` the
`openclaw backup` uploads. The same `entry_json` gives the session's
delivery route, model override, permission settings and archive marker;
the agent database's `session_conversations`, `conversations` and
`session_windows` give the relay rows, `session_pending_inputs` the
prompts that never reached the transcript, the full-text index content
table `session_transcript_fts_content` the entries the transcript lost,
`conversation_deliveries` the channel sends and replies,
`session_transcript_archives` the deletions and
`memory_session_tombstones` the sessions forgotten from memory.
`state/openclaw.sqlite` also gives `operator_approvals`,
`exec_approvals_config` (never its socket token),
`operator_approval_standing_grants`, `channel_pairing_allow_entries`,
`channel_ingress_events` and the three GitHub publication request tables.
A PostgreSQL engine anchor in place of either database is one row.

Validated against: Source at [fc3490a](https://github.com/openclaw/openclaw/commit/fc3490aa24e624f0182ae5e29482611fee5736fb) (version 2026.10.9 on
`main`), with the older shapes at [146284a](https://github.com/openclaw/openclaw/commit/146284a289f98a2cf3babbc55cc3bd16c28e604b) and [3b16db7](https://github.com/openclaw/openclaw/commit/3b16db73c0b5209aad2ee7c3fb5ddfb12fb208f9) and the checkpoint and
backup writers, since removed, at [6814525](https://github.com/openclaw/openclaw/commit/68145258676f1401a2e42cbaffe6564c2a6213c6) and [6559e3f](https://github.com/openclaw/openclaw/commit/6559e3f3b94c54e2b1aa06d1fca712c5360455ff); fixture only.

### `opencode`

Files read: `.local/share/opencode/opencode*.db` (SQLite with JSON
columns, WAL sidecars; in a 1.x database V1 `message`/`part`, V2
`session_message` for sessions without V1 rows; in a 2.x database
`session_v2` and `session_message`, with an assistant message's
`session_part` items, and the 1.x tables for sessions that
`session_message` does not hold; `session_share` and the
`session.updated.1` rows of `event` for `session synced:` rows, never
the share `secret`), with `opencode-next.db`, the pre-release 2.0
database, read as a secondary source; legacy JSON
`storage/session/*/*.json` and `storage/message/*/*.json` with their
`part/` files and the `storage/project/` file of a session's project, and
the older `project/<slug>/storage/session/` tree. In each database also
the `permission`, `worktree` and `project_directory` tables and the
session's `permission` and `time_archived` columns; and the terminal UI's
`.local/state/opencode/prompt-history.jsonl` and `session.json` (both
secondary) and `prompt-stash.jsonl`. The desktop app writes its sessions
to the same database.

Validated against: Source at [bf544cd](https://github.com/anomalyco/opencode/commit/bf544cd419c5f17535c0d5af929f7ecd41188320) (2.2.5), synthetic fixture; no real install. Earlier source reviews: [cb07c55](https://github.com/anomalyco/opencode/commit/cb07c555d4d1ca34430b7857b52f104c3d95e09e) (2.0.32) and [907b3bc](https://github.com/sst/opencode/commit/907b3bc518fa48e90e8ec24dd327d13eee71c36c) (1.x).

### `openhands`

Files read: `.openhands/agent-canvas/dev_conversations/<hex>/`,
`.openhands/agent-canvas/conversations/<hex>/` and
`.openhands/conversations/<hex>/`: one `events/event-<idx>-<id>.json` per
event, with the sibling `meta.json` (title, `created_at`, working dir) and
`base_state.json` (model, per-response token usage). Event times are naive
local time, corrected by the offset `meta.json` `created_at` implies, else
emitted as if UTC. Each `base_state.json` also gives the confirmation
policy, security analyzer and hook-blocked actions as standing permission
rows. The CLI's `.openhands/projects/<sha256 of cwd>/prompt_history.json`
is a secondary source. Legacy 0.x `sessions/<sid>/events/<n>.json` is
reported as one `system` row per session, not parsed.

Validated against: Source at [b347047](https://github.com/OpenHands/software-agent-sdk/commit/b347047e2dcdd8f4b2be810aa8bc6632bc756d41) (SDK), synthetic fixture; the security-analyzer verdicts, model switches, standing permission rows and worktree scratch directories from source at [33d8aa2](https://github.com/OpenHands/software-agent-sdk/commit/33d8aa29c3078d074799e2fbf5e847aeb2d71360) (SDK) and the prompt history at [0403f1a](https://github.com/OpenHands/OpenHands-CLI/commit/0403f1a79a9fc62a6c7be21c17d7b51f64245ad2) (CLI), synthetic fixture.

### `pearai`

Files read: `.pearai/sessions/<sessionId>.json` (Continue fork, `history`
and `perplexityHistory`, timestamped from `sessions.json` `dateCreated`)
and `<editor>/User/globalStorage/pearai.pearai-roo-cline/tasks/<id>/` (Roo
Code 3.15 fork through the Cline task mapping; XML tool calls in
`api_conversation_history.json`, also read beside `ui_messages.json` for
the permission decisions; project from `taskHistory` in the sibling
`state.vscdb`), the `pearai.pearai-roo-cline` row of the editor's
`User/globalStorage/state.vscdb` (one row per `taskHistory` entry and per
standing auto-approval setting), the `PearAI` output channel log
`<editor>/logs/<session>/window<N>/exthost/output_logging_<time>/<n>-PearAI.log`
and `.pearai/dev_data/chat.jsonl` (secondary: every chat request's prompt
and reply, and the rated ones with their session), and
`.pearai/dev_data/tokens_generated.jsonl` (the model of each request).

Validated against: Source at [51eceef6](https://github.com/trypear/pearai-submodule/commit/51eceef62a90c29f712b3a9607ea70a9dca657e9) and [0b6df736](https://github.com/trypear/PearAI-Roo-Code/commit/0b6df736c9b2799c38b9ee629668407521a7bdda), synthetic fixture; the permission texts, the task list, the standing settings and the chat use logs from source at the same commits and [d930f023](https://github.com/trypear/pearai-app/commit/d930f0233c14668df9f85c6a78a81828f4251194) (the editor), synthetic fixture.

### `pi`

Files read: `.pi/agent/sessions/<cwd>/<time>_<id>.jsonl` and legacy
`.pi/agent/*.jsonl` (session tree; fork entries copied from a collected
parent file are dropped; a session that looks driven by little-coder gets
a `system` row saying so),
`.pi/agent/experimental/sessions/<id>/meta.json` (the durable
`session.sqlite` beside it is not parsed), `.pi/agent/trust.json` (project
trust decisions, one `permission setting:` row per project),
`.pi/agent/settings.json` (only `defaultProjectTrust`, one `permission
setting:` row), `.pi/agent/pi-debug.log` (secondary, the last `/debug`
snapshot of a session's messages) and `.pi/agent/crashes.json` (one
`crash:` row per recent crash). A session's
`pi.bug-report` entry gives a `session synced:` row for a `/bug` upload
or a `bug report exported:` row for a zip, and a call little-coder's
permission gate refused a `permission decision: denied` row.

Validated against: source at [2c8d1fe](https://github.com/earendil-works/pi/commit/2c8d1feb147f6b3d97e855da803bfe7b19f3c403) (coding-agent 1.4.0), and the older shapes of tag v0.83.0 and of 1.0.2 at [2003871](https://github.com/earendil-works/pi/commit/200387122ca450d6387f033949423114a270b96c); synthetic fixture only, no real install.

### `qwen-code`

Files read: `.qwen/projects/<slug>/chats/<session>.jsonl` and
`chats/archive/`, `.qwen/tmp/<hash>/logs.json`;
`.qwen/trustedFolders.json`, one `permission setting:
trustedFolders.<path>=<value> (from <file>, file mtime)` row per folder at
the file's modification time with `project_path` the folder;
`session_model` as `model: <id> auth=<authType>`, an assistant record's
`model` as `model: <id>` when no earlier record named one and `model:
<old> -> <new>` on each change, the `logs.json` `model_switch` as `model:
<from> -> <to> reason=<reason>`; a tool result with status `cancelled` or
`awaiting_approval` is followed by `permission decision: <status>`;
`.qwen/channels/sessions.json` and `.qwen/channels/daemon/<hash>/routes.json`,
one `session relayed:` row per session driven from a messaging platform
(fixture only), so the parser records relay state and `sessions.jsonl`
`relayed` reads `yes` or `no`. A record's `executionContext` as `model:`
and `permission mode:` rows on change, a `session_approval_mode` record
as `permission mode:`, and a `ui_telemetry` tool call the user answered
as `permission decision: <decision> (tool_call ui_telemetry)`;
`.qwen/settings.json` in the home or a project, one `permission setting:`
row per `permissions.allow`, `ask` and `deny` rule and one for
`tools.approvalMode`; `.qwen/usage_record.jsonl`, one `session usage:`
row per session summary, a secondary source that outlives a deleted
transcript; a session's `chats/<session>.worktree.json` sidecar, one
`scratch directory: <worktreePath> (enter_worktree <slug>)` row naming the
worktree the session works in.

Not read ([issue 396](https://github.com/seanthegeek/doubleagent/issues/396)):
the token usage log `usage/token-usage-<YYYY-MM>.jsonl`, the channels'
`<channel>-group-history.jsonl`, the todo files `todos/<session>.json`,
the project summary `.qwen/PROJECT_SUMMARY.md` and the managed local mode's
transcripts under `.qwen/managed-local/harness/projects/` and
`.qwen/managed-local/worker-home/`.

Validated against: Source at [835b080](https://github.com/QwenLM/qwen-code/commit/835b080f8567d8612effa248e1fafa11af0e244b) (version 0.26.4), synthetic fixture; the shapes of [2c591ec](https://github.com/QwenLM/qwen-code/commit/2c591ecc08a6fa080342f9b1b9f7f43215178cbb) (0.24.7) and [5dcf7c7](https://github.com/QwenLM/qwen-code/commit/5dcf7c7cb162e825d95d531081f67ee60da3c0e1) (0.25.3) still read.

### `roo-code`

Files read: `<editor>/User/globalStorage/rooveterinaryinc.roo-cline/` and
`.vscode-mock/global-storage/`: the Cline task files plus
`tasks/<id>/history_item.json` and `tasks/_index.json`; the CLI's input
history `.roo/cli-history.json` (a secondary source, text only). For
permission state: the `isAnswered` mark in `ui_messages.json`, the denial
and approval JSON in the API history (read for decision rows even beside
`ui_messages.json`), the extension's global state (the
`RooVeterinaryInc.roo-cline` row of a VS Code or fork
`User/globalStorage/state.vscdb`) and the CLI's
`.vscode-mock/global-storage/global-state.json`, `.roo/cli-settings.json`,
and the MCP `alwaysAllow` lists of `settings/mcp_settings.json` and a
project's `.roo/mcp.json`. For Roo Code Cloud (releases up to 3.53.0): the
settings cache in the same global state and the retry queue in the
workspace state (the extension's row of `User/workspaceStorage/<hash>/state.vscdb`
and the CLI's `.vscode-mock/workspace-storage/<hash>/workspace-state.json`);
the queue's bearer token and message bodies are never emitted.

Validated against: Source at [b867ec91](https://github.com/RooCodeInc/Roo-Code/commit/b867ec9145750d0ae1ff7f02d35406e9bf2a0b16), synthetic fixture; the Roo Code Cloud settings cache and retry queue from source at [44f6bba2](https://github.com/RooCodeInc/Roo-Code/commit/44f6bba250235a98a21403df8b2ff64dfb5421c0) (version 3.53.0, the last release with the cloud client), synthetic fixture.

### `shellgpt`

Files read: Files directly under a `chat_cache/` directory:
`AppData/Local/Temp/chat_cache/<id>` and
`AppData/Local/Temp/shell_gpt/chat_cache/<id>`, or a copied Linux or macOS
temp directory given as loose input. One JSON array of OpenAI messages per
chat, `tool_calls` and the pre-1.5.0 `function_call` shape; no timestamp,
model or working directory is recorded, so those columns stay empty.
`.config/shell_gpt/.sgptrc` (four keys only: `DEFAULT_MODEL`,
`OPENAI_USE_FUNCTIONS`, `DEFAULT_EXECUTE_SHELL_CMD`, `SHELL_INTERACTION`;
the API key is never read into a row), the function store
`.config/shell_gpt/functions/*.py` (file names only) and the response
cache `AppData/Local/Temp/shell_gpt/cache/<md5>` (secondary).

Validated against: Source at [a082bd5](https://github.com/TheR1D/shell_gpt/commit/a082bd5327ce0c4ef5a0284d9060e833be9444a6), synthetic fixture; the settings, function store and response cache rows from the same source, synthetic fixture.

### `tabby`

Files read: Server-side: `.tabby/ee/db.sqlite` or `dev-db.sqlite` (SQLite
with WAL sidecars; `threads`, `thread_messages`, `user_events`, the
AI-written `pages` and `page_sections`, the source-host `integrations` and
their cached `provided_repositories`, thread owner from `users.email`;
secret columns never read), the pre-migration backups
`ee/db.backup-YYYYMMDD.sqlite` (may hold threads since deleted;
secondary), the completion event log `.tabby/events/YYYY-MM-DD.json`
(full prompt and generated text; secondary, pairs with `user_events`) and
the configured models in `.tabby/config.toml` (`[model.*]`, with Python
3.11 or later). The same files at the root of a Docker or Podman volume
matching `*tabby*`, without the leading `.tabby/`. Client-side, the
inline-edit command histories: the IntelliJ plugin's
`JetBrains/<product>/options/intellij-tabby-command-history.xml` and the
`edit.recentlyCommand` key of the `TabbyML.vscode-tabby` row of a VS Code
or fork `User/globalStorage/state.vscdb` (both secondary, no times).
Project is the attached repository URL; no local path is recorded.

Validated against: Source at [21b2904](https://github.com/TabbyML/tabby/commit/21b29048d7bcf6b94f9f482f2d0fd05efadfd19f), synthetic fixture; the
pages, source-host, model, command-history and volume rows were added from
the same commit on 2026-10-21, source-derived and not observed on any real
install.

### `twinny`

Files read: The `rjmacarthy.twinny` row of a VS Code or fork
`User/globalStorage/state.vscdb` (`twinny.conversations`, the standing
Always run grants, the agent-mode and auto-run switches and
`twinny.teamPolicy`). Messages have no time of their own: every row
carries the conversation's `updatedAt`. Provider `apiKey`s and the ChatGPT
plan profile in the same value are never emitted. As secondary sources,
the `agent-changes/<id>.json` records under
`User/workspaceStorage/<id>/rjmacarthy.twinny/` (or
`User/globalStorage/rjmacarthy.twinny/`), which carry the project root,
and the extension's log channel, `logs/.../rjmacarthy.twinny/Twinny.log`
and its rotated `Twinny.log_<n>` copies. The `rjmacarthy.twinny` row of
`User/workspaceStorage/<id>/state.vscdb` for the pull request review
tab's GitHub repository.

Validated against: Source at [9339bd10](https://github.com/twinnydotdev/twinny/commit/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6), synthetic fixture; the `said` tool step of Twinny 4.3.5 and later from source at [0337b265](https://github.com/twinnydotdev/twinny/commit/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a) (version 4.3.15), synthetic fixture; the standing grants, team policy, file changes, `agent-changes/` records, log channel and review repository from source at the same commit, synthetic fixture.

### `vscode`

Files read: `User/workspaceStorage/<hash>/chatSessions/*.jsonl` and
`.json` (with the sibling `workspace.json` for the project),
`User/globalStorage/emptyWindowChatSessions/*`,
`User/globalStorage/transferredChatSessions/*.json`, for Code, Code -
Insiders, VSCodium, Positron, Trae and the `.vscode-server*/data` remote
layout. Covers Copilot Chat; its model and participant ids are opaque
strings. Copilot Chat's session sync record
`User/globalStorage/github.copilot-chat/cloudSessions.json` (one
`session synced:` row per uploaded session) and the
`github.copilot.cloudAgent.ownedTasks` key of the `GitHub.copilot-chat`
row in `User/globalStorage/state.vscdb` (one `session synced:` row and
one `session origin: remote GitHub cloud agent` row per cloud agent
task, which began on GitHub's servers, so the parser records origin and
`sessions.jsonl` `origin` reads `remote` or `local`). The standing tool
approvals in each `state.vscdb` (`User/globalStorage/`,
`User/profiles/<id>/globalStorage/` and
`User/workspaceStorage/<hash>/`), the auto-approve, default permission
and default model settings of `User/settings.json` and
`User/profiles/<id>/settings.json`, the GitHub account's usage records
and the repositories picked for cloud agent tasks in
`User/globalStorage/state.vscdb`, and the accept or reject state of each
file a chat edited in
`User/workspaceStorage/<hash>/chatEditingSessions/<id>/state.json`, a
secondary source that survives the chat's deletion.

Validated against: Source at [d7622a5](https://github.com/microsoft/vscode/commit/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e), synthetic fixture; the permission level, model options, standing grants, settings, GitHub account records and chat editing state from source at [4d22f4f](https://github.com/microsoft/vscode/commit/4d22f4f922eb012217ac65672ccce02f59d63706), synthetic fixture.

### `zed`

Files read: `threads/threads.db` (zstd-compressed JSON threads; needs
`zstandard`) and `db/0-<channel>/db.sqlite` (`sidebar_threads`, external-agent
threads and native thread state; `trusted_worktrees`; the `scoped_kv_store`
namespaces `agent_draft_prompts` and `created_git_worktrees`;
`archived_git_worktrees` with `thread_archived_worktrees`;
`sidebar_terminal_threads`) under `.local/share/zed`,
`Library/Application Support/Zed`, `AppData/Local/Zed` or the Flatpak data
dir, and `settings.json` and `global_settings.json` under `.config/zed`,
`AppData/Roaming/Zed` or the Flatpak config dir. No per-message timestamps:
messages carry the thread's `updated_at`.

Validated against: Source at [a846890](https://github.com/zed-industries/zed/commit/a84689073d296dfd39987bc7dd478e43ef76d83a), synthetic fixture; the settings files, the other `db.sqlite` stores, denied calls, scratch directories and draft prompts from source at [9cd14da](https://github.com/zed-industries/zed/commit/9cd14daad768fbfaeeac8789f515f9a5f67669a6), synthetic fixture.

## Collected agents

The collector gathers these agents' files, but no parser reads them, so
nothing from them enters the timeline. `doubleagent detect` still shows
them, and `timeline` names them on its `agents:` line as detected but not
parsed (a `backlog` agent, whose records await a parser) or as collected
only (a `collected` agent, whose files hold nothing to parse). Each
subsection lists what the collector gathers and the evidence the research
rests on.

### `amp`

Files collected: `.local/share/amp/history.jsonl` (prompt history with the
working directory), per-thread logs `.cache/amp/logs/threads/<T-id>.log`
and `cli.log`, the terminal tool's shell history
`.cache/amp/terminal/<threadId>/history`, `.amp/claude-diagnostics/`
(Claude Code session files copied when Amp drives Claude Code), settings
and `secrets.json`; full threads are stored server-side by ampcode.com.

Validated against: Shipped binary strings, docs.

Notes in full: prompt history, per-thread logs and the terminal tool's
shell history; full threads are server-side.

### `augment`

Files collected: `.augment/sessions/<conversationId>.json` (full
conversations), `prompt-history.jsonl`, `session.json` (the OAuth
session), `settings.json`, rules, commands, agents and plans.

Validated against: Shipped bundle, docs.

### `autogen-studio`

Files collected: `.autogenstudio/autogen04203.db` (SQLite with the tables
`team`, `message`, `session` and `run`), `files/`, `configs/`,
`temp_env_vars.env` and `.env`.

Validated against: Source.

### `camel-ai`

Files collected: `.camel/gmail_token.json` (the Gmail toolkit's Google
OAuth token), `.camel/skills` and `.config/camel`.

Validated against: Source.

Notes in full: no transcripts under the home; its `chat_history.json` is
relative to the working directory.

### `copilot`

Files collected: `.config/github-copilot` (`AppData/Local/github-copilot`
on Windows): `apps.json` and the legacy `hosts.json`, the OAuth token
store shared by the editor plugins that embed the Copilot language server
(Neovim, JetBrains, Xcode, Vim).

Validated against: Source (the vendored language server).

### `crewai`

Files collected: The run data directory, named after the project folder
(`.local/share/<project>`, `Library/Application Support/<project>`,
`AppData/Local/CrewAI/<project>`): `latest_kickoff_task_outputs.db` (every
task's output with its messages), `flow_states.db` and
`.crewai_user.json`; `.config/crewai/settings.json`, the
`crewai/credentials/` key and encrypted token, and `.crewai` skills; in a
project that another agent's state or `-p` names, its `.crewai` directory
(`last_run.json`, the record that a run's trace reached AMP, CrewAI's hosted
service, and the `crewai eval` state).

Validated against: Source, appdirs source.

### `dify`

Files collected: The `difyctl` CLI client's `config.yml`, `login.yml`,
`tokens.yml` and `login-pending.yml` (`.config/difyctl`, `AppData/Roaming/difyctl`,
`AppData/Local/difyctl`), and the Docker volumes
`*dify_agent_local_sandbox_home` (the sandbox runtime shellctl's
`shellctl.db` and job output logs) and
`*dify_agent_local_sandbox_workspace`.

Validated against: Source.

Notes in full: the sandbox volume's `shellctl.db` and job logs hold the
shell jobs the agent ran; chats are in PostgreSQL.

### `factory-droid`

Files collected: `.factory/sessions/<projectKey>/<sessionId>.jsonl`
transcripts with their `.settings.json` sidecars,
`cache/session-index/index.db`, `state/history.json` (prompt history),
`missions/`, `logs/`, settings, `mcp.json` and `hooks.json`.

Validated against: Shipped binary strings, docs.

### `flowise`

Files collected: `.flowise/database.sqlite` (`chat_message` with
`usedTools` and `agentReasoning`, `execution`, the encrypted `credential`
table), `encryption.key` and the auth signing secrets, `storage` and
`uploads` (the `vectorstore` indexes are excluded).

Validated against: Source.

### `langflow`

Files collected: The config directory (`.cache/langflow`,
`Library/Caches/langflow`, `AppData/Local/langflow/langflow/Cache`) with
`secret_key` and uploaded files, `.langflow/fs_tool/fs_sandbox` (the
`.langflow/knowledge_bases` stores are excluded), Langflow Desktop's
`.langflow/data` (`database.db`, `.env`) and `com.LangflowDesktop`
directories (from the documentation only), and the Docker volume
`*langflow-data`; the open-source server's `langflow.db`, with the
`message` and `transaction` tables, sits in the installed package and is
not collected.

Validated against: Source, platformdirs source; Desktop from docs only.

Notes in full: Langflow Desktop's `database.db`; the server's
`langflow.db` sits in the installed package and is not collected.

### `local-deep-research`

Files collected: The data directory (`.local/share/local-deep-research`,
`Library/Application Support/local-deep-research`,
`AppData/Local/local-deep-research`): `ldr_auth.db` (the account list),
`encrypted_databases/ldr_user_*.db` (research history and chat), its
backups, the opt-in `research_outputs/` reports and logs, and the pre-1.0
unencrypted `ldr.db`; `Documents/LocalDeepResearch`; the Docker volume
`*ldr_data`.

Validated against: Source, platformdirs source.

Notes in full: research history and chat in per-user SQLCipher (encrypted
SQLite) databases keyed from the login password.

### `metagpt`

Files collected: `.metagpt/config2.yaml` (LLM configuration with
`api_key`) and `config2.bak`.

Validated against: Source.

### `n8n`

Files collected: `.n8n/database.sqlite` (`execution_entity`,
`execution_data`, `chat_hub_sessions`, `chat_hub_messages` and the
encrypted `credentials_entity`), `.n8n/config` (the encryption key), the
event log and crash journal, `.n8n-local-gateway/log` (every computer-use
tool call with its arguments; computer-use is n8n's local tool gateway for
its Assistant), `.n8n-gateway/settings.json`, and the Docker volumes
`*n8n_data` and `*n8n-data`.

Validated against: Source.

Notes in full: `database.sqlite` executions and chat messages, and the
computer-use gateway's log of every tool call.

### `pydantic-clai`

Files collected: `.pydantic-ai/prompt-history.txt` (`clai` v1) and
`.config/pydantic-clai2/` (CLAI 2.0): `sessions.db` (`conversations` with
the messages, `workspace` and `model`), `input-history`, `config.db`,
`credentials-*` and `mcp.json`; `sessions.db` can also hold Claude Code
and Codex sessions imported by `/resume`. The CLAI 2.0 state directory
`.local/state/pydantic-clai2/`: the web orchestrator's `orchestrator.json`
and each workspace's `workspace.json`, `token` and `session.log`, with the
workspaces' clones of remote repositories excluded.

Validated against: Source, PyPI wheel.

### `shared`

Files collected: `.agents` and `.config/agents` (skills and agent
definitions in the cross-tool layout) and `.config/AGENTS.md` (user-level
instructions), read by several agents at once.

Validated against: Derived from the per-agent documents.

### `windsurf`

Files collected: `.codeium/windsurf/cascade/<uuid>.pb` (the history of
Cascade, Windsurf's agent, as protobuf), memories, workflows and
`mcp_config.json`; `.windsurf/transcripts/*.jsonl` (the transcripts
Cascade hands to hooks); the Windsurf and Devin editors' `User/` and
`logs` directories; the Devin CLI's `.config/devin`,
`.local/share/devin/credentials.toml` and, on Windows, `mcp_config.json`
and `credentials.toml` in `AppData/Roaming/devin`.

Validated against: Official docs, community write-ups.

Notes in full: Cascade history as protobuf and hook transcripts; blocked
on format research, the `.pb` files have no published schema.
