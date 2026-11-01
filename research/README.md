# Research

One document per catalog agent, named after the agent as it appears in
`collect-agent-artifacts.sh --list`, holds everything the project knows
about that agent, in two parts. The Collection part records where the
agent stores state on each OS, which files are credentials, which subtrees
are large or worthless, which files it writes inside projects and where it
records the project path; it is the evidence behind the `CATALOG`,
`PROJECT_CATALOG`, `EXCLUDES`, `SECRET_GLOBS` and `DOCKER_VOLUMES` tables.
The Parsing part describes how the agent records transcripts on disk and
how a parser should read them, and ends with a parser plan mapped to the
timeline columns and synthetic sample records meant to become test
fixtures; it is the evidence behind each parser, and the field names a
parser depends on are repeated in its module docstring under
`analyzer/doubleagent/parsers/`. An agent with nothing to parse has one
paragraph in place of the Parsing sections. Each document ends with the
confidence in each finding, for both parts.

Forks of one codebase were often studied together, because the differences
are what matter, but each agent has its own document. Where two agents
share a lineage, each document says so and links to the other.

Evidence levels, best first: a real install, a per-step round of the agent
on the platform named, recorded as an Observed bullet; source code at a
named commit, each citation linked to the line at that commit, with the
project's own documentation beside it; for an agent without public source,
the vendor's documentation (the documentation site, and the release notes,
samples and plugin types in its official repositories), read first so the
package is read with its terms known, then the shipped package or binary
(`npm pack`, extracted bundles, binary strings, or for Antigravity the
protobuf descriptors embedded in its binary) for the key names, record
shapes and path strings the documentation does not state; vendor forums and
issue trackers; published write-ups; and a vendor's own statement, recorded
with its author and date where it points at nothing a reader can check.
Each document says which level each claim rests on, and where two levels
disagree it records both: a real install settles the question, and before
one the shipped package stands ahead of the page, because it is what ran.
Where a document mentions `scratchpad/...`, that is the
research session's working directory, kept only for the duration of the
research; the cited sources are the repositories, package versions and URLs
named beside it.

The first section of a document carries one Observed bullet per platform and
surface the agent has been run on, naming the build or version, the date,
what was exercised and what each surface wrote. That build is the version
the document's evidence rests on, the paths as much as the record shapes:
line types and key sets change between releases (the rollouts of the
ChatGPT desktop app's bundled Codex 0.163 carried per-turn settings events,
reviewer threads and turn triggers that the document written from the 0.162
install did not list). Before relying on a document against an install,
compare the install's build with the latest Observed bullet for that
platform and surface; when they differ, or no bullet exists for the cell,
the document describes an older build or none, and the install gets a
per-step round
([Real-install rounds in `AGENTS.md`](../AGENTS.md#real-install-rounds))
before its paths or shapes are read from the document as current; the shape
tool's output is diffed against the storage and parser-plan sections before
the parser is changed.

Two checks at the repository root keep the documents honest.
`tools/check_research_links.py` checks every GitHub citation in these
documents: for each link, that the file exists at the cited commit and that
the cited line range lies inside it. It fetches each file from
`raw.githubusercontent.com`, or uses local checkouts with `--repos DIR`
(each named after its repository, with `HEAD` at the cited commit). Pass
file names to check only those. It exits `1` when any link fails.
`tools/check_research_structure.py` checks that every agent document
follows the outline of
[Research document structure in `AGENTS.md`](../AGENTS.md#research-document-structure)
(the headings, their numbers and the fixed subsections) and that prose
lines wrap at 100 characters; with `--before DIR` holding earlier copies of
the documents, it also lists every citation, code span, fenced line, number
and label a rewrite dropped.

## Index

The Collector evidence column names the evidence levels the Collection
part rests on, the real install first where there is one. The Parser column says whether `analyzer/doubleagent/parsers/` has a
parser for the agent, and for an agent without one, its status on the
[agents page](../docs/agents.md), which lists the files each parser reads.
Notes end with the transcript store, where the agent has one.

| Agent | Document | Collector evidence | Parser | Notes |
| --- | --- | --- | --- | --- |
| `agent-zero` | [agent-zero.md](agent-zero.md) | source (agent-zero, a0-install, a0-launcher, a0-connector) | yes | state under the install dir `~/agent-zero/<instance>/usr`, not a dot dir; named Docker volumes `*a0_usr` and `a0-launcher-*-usr` collected since 1.5.0. Store: JSON per chat context, rewritten whole |
| `aider` | [aider.md](aider.md) | source | yes | history files live in the repository, home copies only as a fallback. Store: markdown and readline-style text in the repository |
| `amp` | [amp.md](amp.md) | shipped binary strings, docs | no, backlog | threads are server-side; XDG dirs on every OS |
| `antigravity` | [antigravity.md](antigravity.md) | real installs (CLI 1.2.16 and desktop app 2.23.0, macOS; CLI 1.3.4 and desktop app 2.23.1, Linux; CLI 1.3.5 and desktop app 2.23.1, Windows), binary strings, app bundle, docs | yes | CLI under `.gemini/antigravity-cli`; desktop app under `.gemini/antigravity` plus its `Antigravity` Electron user-data and logs directories; IDE now `.gemini/antigravity-ide` and `Antigravity IDE` app dirs; plugin OAuth token in `plugin_data/<plugin>/token.json`; CLI plugin secret file store `plugin-secrets.json` under the OS configuration directory. Store: SQLite of protobuf blobs per conversation, the same for the CLI (`.gemini/antigravity-cli`) and the desktop app (`.gemini/antigravity`); schema from the descriptors embedded in `agy` and the app's language server; verified on Linux (WSL: CLI; Ubuntu: CLI and app), macOS (CLI and app) and Windows (Windows 11 ARM64: CLI 1.3.5 and desktop app 2.23.1) installs |
| `atlarix` | [atlarix.md](atlarix.md) | vendor statement | no, backlog | closed source; Electron user-data dir (`Library/Application Support/Atlarix`, `AppData/Roaming/Atlarix`, `.config/Atlarix`) shared by the app, CLI, editor extension and ACP agent, plus `~/.atlarix`. Store: SQLite `sqlite.db` |
| `augment` | [augment.md](augment.md) | shipped bundle, docs | no, backlog | `~/.augment` on every OS |
| `autogen-studio` | [autogen-studio.md](autogen-studio.md) | source | no, backlog | `~/.autogenstudio` on every OS; run history and model client API keys in one SQLite database |
| `camel-ai` | [camel-ai.md](camel-ai.md) | source | no, collected only | `~/.camel` is shared with Apache Camel JBang, so only CAMEL's Gmail token and skills dirs are collected |
| `chatgpt-desktop` | [chatgpt-desktop.md](chatgpt-desktop.md) | real install (Windows, macOS), write-ups and press | yes | since 26.1007 one Chromium-based build on both platforms (Windows package `OpenAI.Codex`, macOS bundle id `com.openai.codex`) with no local chat transcript; the former native macOS app encrypted conversations with a Keychain key. Store: no transcript: a ChatGPT conversation's text stays on the server; the app lists the account's conversations at sign-in as `chatgpt` rows of `~/.codex/sqlite/codex-dev.db` `local_thread_catalog` (WAL), while its Codex threads are `codex-cli` rollouts; closed source, from Windows and macOS installs |
| `claude-code` | [claude-code.md](claude-code.md) | real installs (Linux WSL2, Ubuntu, macOS, Windows; clean-VM rounds on Windows, Ubuntu and macOS, October 2026), shipped bundle, docs | yes | `CLAUDE_CONFIG_DIR`; several credential files beyond `.credentials.json`. Store: JSONL per session with `uuid`/`parentUuid` chains, subagent files beside it, prompt history |
| `claude-desktop` | [claude-desktop.md](claude-desktop.md) | real install (MSIX build, Linux deb, macOS), official docs, GitHub issues | yes | encrypted token cache in `config.json`; local-agent sessions. Store: session records (JSON) and an HMAC-chained `audit.jsonl` per Cowork session around Claude Code JSONL transcripts; closed source, from the app bundle; verified on Windows, Linux and macOS installs |
| `cline` | [cline.md](cline.md) | source | yes | `~/.cline/data` shared by VS Code, CLI and JetBrains. Store: JSON arrays per task; SDK session files; SQLite indexes |
| `codex-cli` | [codex-cli.md](codex-cli.md) | real install (macOS and Windows: desktop app and standalone CLI; Linux: standalone CLI), source | yes | rollouts may be zstd-compressed; the ChatGPT desktop app adds `sqlite/codex-dev.db` and `.codex-global-state.json`, which holds ChatGPT resume tokens. Store: JSONL rollouts (`session_meta`, `response_item`, `event_msg`), optionally zstd-compressed |
| `cody` | [cody.md](cody.md) | source (frozen public snapshot, August 2025) | yes | chat history and token are rows in `state.vscdb`; the nested entry claims only the symf index. Store: rows in the editor `state.vscdb`; JetBrains global-state JSON |
| `continue` | [continue.md](continue.md) | source | yes | `~/.continue` only; the repository is read-only after the final 2.0.0 release. Store: JSON per session, session-level timestamps only |
| `copilot` | [copilot.md](copilot.md) | source (vendored language server) | no, collected only | editor OAuth store `github-copilot/apps.json` |
| `copilot-cli` | [copilot-cli.md](copilot-cli.md) | real install (macOS, Linux, Windows), shipped bundle, docs | yes | migrates XDG state and config dirs into `~/.copilot`. Store: JSONL event log per session (`events.jsonl`) with a cross-session SQLite store (WAL) and a prompt history; closed source, from the shipped bundle's event schema and macOS and Linux installs, and on Windows a winget install and the CLI the Copilot Desktop app bundles |
| `copilot-desktop` | [copilot-desktop.md](copilot-desktop.md) | real installs (Linux, Windows), shipped binary strings, dirs crate | yes | Tauri app that shares `~/.copilot` with Copilot CLI; nested entries claim its own store, chat, repository and run files; local WebSocket tokens in `.copilot/run`; WebView2 user data folder in `AppData/Local` on Windows. Store: SQLite app store `~/.copilot/data.db` (WAL) of accounts, projects, worktrees, workspaces, sessions, activity and schedules; the sessions themselves are Copilot CLI event logs; closed source, from the shipped binary and Linux and Windows installs |
| `crewai` | [crewai.md](crewai.md) | source, appdirs source | no, backlog | run database in a data dir named after the project folder, collected by file name; opt-in memory stores beside it are not |
| `crush` | [crush.md](crush.md) | source | yes | sessions only in each project's `.crush/crush.db`. Store: per-project SQLite (WAL), parts as a JSON array |
| `cursor` | [cursor.md](cursor.md) | real installs: macOS (IDE 3.26.13 and Agent CLI 2026.10.19-6bbf2c4, 2026-10-20), Linux (IDE 3.25.9 deb, 2026-10-20), Windows (IDE 3.26.13 per-user installer, arm64, 2026-10-21); bundle for the CLI elsewhere; docs, forum, write-ups | yes | secret globs on the CLI's `auth.json` (three paths), `.cursor/mcp.json` and the per-project `mcp-auth.json`; the IDE's tokens sit in `state.vscdb` beside the chats, collected unflagged; the IDE embeds a 608 MB copy of the Agent CLI on macOS, and downloads it on Linux and Windows when a cloud feature is used (on Windows arm64 it fails to start); excluded, as is the marketplace copy of each installed plugin (`.cursor/plugins/cache`). Store: `state.vscdb` `cursorDiskKV` rows per composer and bubble (SQLite, WAL), with a JSONL copy per conversation under `~/.cursor/projects`; the Agent CLI's sessions in `~/.cursor/chats/<md5>/<id>/store.db`, JSON messages and protobuf nodes as blobs, not parsed |
| `dify` | [dify.md](dify.md) | source | no, backlog | server state in bind mounts beside its compose file (manual step); agent sandbox named volumes and `difyctl` config collected |
| `factory-droid` | [factory-droid.md](factory-droid.md) | shipped binary strings, docs | no, backlog | `.factory/cache` holds the session index |
| `flowise` | [flowise.md](flowise.md) | source | no, backlog | `~/.flowise` on every OS; `encryption.key` decrypts the credentials in `database.sqlite` |
| `gemini-cli` | [gemini-cli.md](gemini-cli.md) | source | yes | slug project dirs; OAuth moved to keyring with encrypted file fallback. Store: JSONL with `$set`/`$patch`/`$rewindTo` operations |
| `goose` | [goose.md](goose.md) | source, crate sources | yes | XDG on macOS; token caches under the config dir; the repository moved to `aaif-goose/goose` (`block/goose` redirects). Store: SQLite (WAL), `content_json` arrays |
| `hermes` | [hermes.md](hermes.md) | source | yes | `~/.hermes` or `AppData/Local/hermes`; profiles are full homes; installer checkout `.env` kept. Store: SQLite (WAL) `state.db`, JSONL fallback transcripts |
| `kilo-code` | [kilo-code.md](kilo-code.md) | source | yes | OpenCode fork; `kilo.db` embeds tokens. Store: OpenCode-style SQLite (`kilo.db`) |
| `kiro` | [kiro.md](kiro.md) | Amazon Q source, Kiro CLI binary strings, docs, issues | yes, Amazon Q CLI only | `.kiro/powers` holds MCP configuration, not just binaries; the Amazon Q Developer CLI repository is frozen. Store: Amazon Q CLI SQLite, one JSON blob per working directory; Kiro CLI unverified |
| `langflow` | [langflow.md](langflow.md) | source, platformdirs source; Desktop from docs only | no, backlog | config dir is the platformdirs cache dir; the OSS `langflow.db` is in site-packages (manual step); `langflow-data` volume |
| `letta` | [letta.md](letta.md) | source (Letta Code; retired V1 server) | yes | `.letta/settings.json` secret glob also flags project settings. Store: JSONL transcripts per agent and conversation, sessions index |
| `little-coder` | [little-coder.md](little-coder.md) | source (little-coder, pi v0.83.0; v0.0.5 for the v0.0.x layout) | yes | launcher for pi; sessions in `~/.pi/agent`, prompt history a nested entry; v0.0.x kept its own `~/.little-coder`. Store: pi session JSONL; prompt history as a JSON array; v0.0.x JSON session files |
| `local-deep-research` | [local-deep-research.md](local-deep-research.md) | source, platformdirs source | no, backlog | history in a SQLCipher database keyed by the login password, so the parser ([issue 56](https://github.com/seanthegeek/doubleagent/issues/56)) needs that password or a plain database |
| `metagpt` | [metagpt.md](metagpt.md) | source | no, collected only | only `~/.metagpt/config2.yaml` (API key) under the home; workspace is relative to the clone or working directory |
| `muse-code` | [muse-code.md](muse-code.md) | real install (Linux), shipped binary strings (Linux and Windows builds), launcher and installer scripts | yes | closed source; XDG dirs under the home on every OS; launcher state in `~/.local/bin` or `AppData/Local/Programs/muse`. Store: event-sourced JSONL per session and subagent; closed source, from the binary's strings, vendor skills and a real install |
| `n8n` | [n8n.md](n8n.md) | source | no, backlog | `~/.n8n` on every OS; computer-use gateway log records every tool call; `n8n_data` and `n8n-data` volumes |
| `nanobot` | [nanobot.md](nanobot.md) | source | yes | `~/.nanobot-<name>` instances; workspace path in `.workspace` markers. Store: JSONL per session key under `sessions/<workspace-id>/` |
| `ollama` | [ollama.md](ollama.md) | real install (Linux), source | yes | desktop app chat database on macOS and Windows; model manifests collected, blobs excluded; `ollama launch chatgpt` writes into `~/.codex`. Store: desktop-app SQLite (WAL) on macOS and Windows; REPL prompt history without times; no server-side transcripts |
| `open-interpreter` | [open-interpreter.md](open-interpreter.md) | source, 0.4.3 sdist | yes, the Codex parser | now a Codex CLI fork with the Codex layout under `~/.openinterpreter`; the legacy Python tool's platformdirs `open-interpreter` and older `Open Interpreter*` dirs collected since 1.10.0. Store: Codex rollout JSONL under `~/.openinterpreter` |
| `openclaw` | [openclaw.md](openclaw.md) | source | yes | formerly Clawdbot and Moltbot; `~/.clawdbot` and `~/.moltbot` are legacy state directories; JSON5 config. Store: SQLite (WAL) per agent with JSON or zstd events, legacy JSONL |
| `opencode` | [opencode.md](opencode.md) | source | yes | XDG roots on every OS; the desktop app shares the CLI's server and database; the repository moved to `anomalyco/opencode` (`sst/opencode` redirects). Store: SQLite (WAL) with JSON columns |
| `openhands` | [openhands.md](openhands.md) | source (OpenHands, software-agent-sdk, OpenHands-CLI) | yes | `agent-canvas/workspaces` and `workspaces` excluded; Docker conversations record container paths. Store: one JSON file per event per conversation, naive local timestamps |
| `pearai` | [pearai.md](pearai.md) | source (pearai-app, pearai-submodule, PearAI-Roo-Code) | yes | VS Code fork; `~/.pearai` is also its Continue fork home. Store: Continue-fork session JSON, Roo-fork task files |
| `pi` | [pi.md](pi.md) | source | yes | same path on every OS; shared with little-coder. Store: JSONL session tree, header line with `cwd` |
| `pydantic-clai` | [pydantic-clai.md](pydantic-clai.md) | source, PyPI wheel | no, backlog | `clai` history and the CLAI 2.0 coding agent's `sessions.db`; `/resume` copies Claude Code and Codex sessions in |
| `qwen-code` | [qwen-code.md](qwen-code.md) | source | yes | second root `QWEN_RUNTIME_DIR`; managed local mode under `managed-local/`. Store: append-only JSONL records with `cwd` and `gitBranch` |
| `qwen-live` | [qwen-live.md](qwen-live.md) | source | no, backlog | `~/.qwen-live` on every OS; a voice daemon that hands tasks to other agents; writes its discovery record into `~/.qwen`. Store: a JSONL call log per daemon run |
| `roo-code` | [roo-code.md](roo-code.md) | source | yes | platform-branched MCP clone dirs. Store: Cline task layout with Roo extensions |
| `shared` | [shared.md](shared.md) | derived from the per-agent documents | no, collected only | cross-agent instruction and skill dirs |
| `shellgpt` | [shellgpt.md](shellgpt.md) | source | yes | chat history in the system temp dir, collected only on Windows. Store: one JSON message array per chat id in the temp dir |
| `tabby` | [tabby.md](tabby.md) | source, JetBrains docs | yes | self-hosted server; Docker `TABBY_ROOT=/data` reached only through a named volume matching `*tabby*`. Store: server SQLite (WAL) `ee/db.sqlite`, event JSON logs |
| `twinny` | [twinny.md](twinny.md) | source | yes | chats in `state.vscdb`; project root inferred from the embeddings manifest. Store: rows in the editor `state.vscdb` |
| `vscode` | [vscode.md](vscode.md) | source (vscode, vscode-copilot-chat) | yes | remote server data dirs hold Copilot Chat state on WSL and SSH hosts. Store: VS Code chat sessions (Copilot Chat): JSONL mutation log, legacy JSON |
| `windsurf` | [windsurf.md](windsurf.md) | official docs, community write-ups | no, backlog | rebrand to Devin in progress; new `.windsurf/transcripts` |
| `zed` | [zed.md](zed.md) | source, crate sources | yes, needs `zstandard` | Flatpak install paths not covered. Store: SQLite with zstd-compressed JSON thread blobs |

Collector version 1.3.0 ([issue 24](https://github.com/seanthegeek/doubleagent/issues/24), commit
[88678a0](https://github.com/seanthegeek/doubleagent/commit/88678a02e7e8deb988c7763b98384cd014d0fff4))
applied the catalog lines these documents proposed, and the proposals were then removed from the
documents: the tables in the scripts record what was adopted, and that commit's history what was
considered.

Collector version 1.4.0 added fifteen agents from the October 2026 backlog
(`agent-zero` to `twinny` above, among them agents that are not coding
agents, such as `hermes`, `openclaw`, `nanobot` and `local-deep-research`)
the same way.

Collector version 1.10.0 ([issue 64](https://github.com/seanthegeek/doubleagent/issues/64)) swept
sixteen agent frameworks, framework studios and self-hosted agent platforms for default per-user
paths. The sweep itself, with the evidence for every framework that got no catalog line (the
AutoGen libraries, Microsoft Agent Framework, Semantic Kernel, TaskWeaver,
LangGraph, the Pydantic AI library, Logfire, Atomic Agents and IntentKit), is
one document, [frameworks.md](frameworks.md), with no row in the table. Each
framework that got a catalog agent (`autogen-studio`, `camel-ai`, `crewai`,
`dify`, `flowise`, `langflow`, `metagpt`, `n8n`, `pydantic-clai`) has its own
document like any other agent, and the legacy Open Interpreter paths were
added to [open-interpreter.md](open-interpreter.md).

## Findings across agents

Collection, from the October 2026 round:

- Remote development hosts matter. `.vscode-server*/data/User`,
  `.cursor-server/data/User` and the Windsurf and Kiro equivalents hold the
  same chat and extension state as the desktop `User` directory, on the WSL
  or SSH host rather than the workstation. All are in the catalog since
  1.3.0.
- Several exclusions swallowed evidence: `.factory/cache` (session index),
  `.kiro/powers` (skills and MCP configuration) and `.kilocode/worktrees`
  (wrong path) were narrowed or corrected in 1.3.0;
  `.local/share/goose/apps` (agent-written output) is not excluded.
- Credentials are moving into OS keyrings with encrypted file fallbacks
  (Gemini CLI, Claude Code, Amp, Factory Droid), so the plaintext files in the
  catalog are increasingly legacy. The fallbacks still appear on headless
  hosts and in WSL, which is where collections usually run.
- Nearly every tool now has a per-project ignore file and a plugin or skill
  marketplace clone directory; the ignore files are project entries and the
  clone directories are exclusions since 1.3.0.

Parsing:

- Only Claude Code, Codex CLI, Qwen Code, Antigravity, Hermes and Zed
  record a git branch, and Zed only at thread start. Every other store
  leaves `git_branch` empty.
- Per-message timestamps are missing in Continue sessions, Zed threads and
  Cline's legacy API history; those parsers inherit session-level times.
- OpenCode, Crush, Goose, Kilo, Antigravity and Zed's sidebar database run
  SQLite in WAL mode, so the `-wal` sidecar must be collected with the
  database. Amazon Q and Zed's `threads.db` use the rollback journal and copy
  cleanly alone.
- Credentials sit inside transcript databases for OpenCode, Kilo and Amazon
  Q; those files are collected unflagged and the keys to redact are listed
  under the `### 12. Secrets` heading of each document.

## Where transcripts live, and what survives

Each row says, for one agent, where its transcripts are stored and
whether they outlive deleting the project, which per-prompt history file
it keeps and what that file survives, where the working directory is
recorded, and what other copies of past use exist and what a delete
leaves. "Not stated" means the agent's document does not say, and the
link in each row opens that document's account of what survives. Parsed,
secondary and not parsed say whether the analyzer reads a file and as
what; [Secondary sources](../analyzer/docs/output.md#secondary-sources) lists the
secondary sources per agent, and
[issue 122](https://github.com/seanthegeek/doubleagent/issues/122)
is the design that shows each event once across them.

| Agent | Transcripts live in | Survives project deletion | Per-prompt history file | Working directory recorded | Second copy |
| --- | --- | --- | --- | --- | --- |
| `agent-zero` | install directory `usr/chats/<ctxid>/chat.json` | yes (projects are `usr/projects/<name>`, a separate place) | none | per chat `data.project` (a name), `data.acp_cwd` for an editor session | The scheduler's `tasks.json` and the Telegram chat map (an entry with no chat marks a deleted chat), secondary; the shadow git of `usr` and the memory store outlive a delete, not parsed. [what survives](agent-zero.md#what-survives-a-delete) |
| `aider` | the project (git root): `.aider.chat.history.md` | no | `<repo>/.aider.input.history`: local time and text, no session; goes with the project; secondary. [what survives](aider.md#what-survives-a-delete) | implicit (the file's location) | `.aider.llm.history` (only with a flag), project-local like the transcript; secondary. [what survives](aider.md#what-survives-a-delete) |
| `antigravity` | home `~/.gemini/antigravity{,-cli}/conversations/<uuid>.db` | yes | CLI `history.jsonl`: prompt, time, workspace and conversation id (absent on a first prompt); secondary. [what survives](antigravity.md#what-survives-a-delete) | per conversation (workspace URIs) | The brain mirror `transcript_full.jsonl`, secondary; the summaries database, one row per conversation. [what survives](antigravity.md#what-survives-a-delete) |
| `chatgpt-desktop` | on OpenAI's servers only; the app's Codex-mode threads are `codex-cli` rollouts | not applicable: the conversations have no project, and their list is under the home | none | none (`cwd` is NULL on the `chatgpt` rows) | The account conversation list, the `chatgpt` rows of `codex-dev.db` `local_thread_catalog`; parsed. [what survives](chatgpt-desktop.md#what-survives-a-delete) |
| `claude-code` | home `~/.claude/projects/<key>/<id>.jsonl`, CLI and VS Code extension alike | yes, every home trace stays (observed on Windows, Linux and macOS) | `~/.claude/history.jsonl`: text, time, project and session, a short paste inline and a long one by hash with its text in `paste-cache/`; kept when retention deletes its transcript; secondary. [what survives](claude-code.md#what-survives-a-delete) | every record (`cwd`, `gitBranch`) | Dated `.claude.json` copies in `backups/` and `jobs/<id>/parent-transcript.jsonl`, not parsed; imported copies in Hermes, Cline and CLAI outlive the transcript. [what survives](claude-code.md#what-survives-a-delete) |
| `claude-desktop` | app data `local-agent-mode-sessions/`; the Code tab uses `~/.claude/projects` | yes | none of its own; the Code tab uses Claude Code's | session record `cwd` and `userSelectedFolders`; JSONL `cwd` per record (a VM path in VM mode) | Per-session `audit.jsonl` (every message, permission events) and the record's `initialMessage`; secondary. [what survives](claude-desktop.md#what-survives-a-delete) |
| `cline` | VS Code `globalStorage` or `~/.cline/data/` | yes | CLI `cache/user_input_history.jsonl`: last 20 unique prompts, no session or cwd; survives a delete; secondary. [what survives](cline.md#what-survives-a-delete) | per task `cwdOnTaskInitialization`, manifest `cwd` | `session-search.db` until the hub next runs, secondary; `logs/hooks.jsonl` survives a delete, secondary. [what survives](cline.md#what-survives-a-delete) |
| `codex-cli` | home `~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl` | yes | `~/.codex/history.jsonl`: session, time and text; survives `thread/delete`; secondary. [what survives](codex-cli.md#what-survives-a-delete) | `session_meta.cwd`, updated per turn | `.codex-global-state.json` keeps a deleted thread's prompts and a deletion marker, secondary; `db-backups/` copies of `state_5.sqlite` secondary; memory rollout summaries parsed. [what survives](codex-cli.md#what-survives-a-delete) |
| `cody` | the editor's `state.vscdb` or `Cody-nodejs/` | yes | none | none (inferred from `contextFiles`) | The JetBrains plugin's `<project>/.idea/cody_history.xml` outlives a delete or clear; secondary. [what survives](cody.md#what-survives-a-delete) |
| `continue` | `~/.continue/sessions/` | yes | CLI `input_history.json`: text and time, no session, up to 1000; survives a delete; secondary. [what survives](continue.md#what-survives-a-delete) | per session `workspaceDirectory` | `dev_data/*.jsonl` events with the session id and `logs/cn.log` survive a delete; parsed, `chatFeedback` secondary. [what survives](continue.md#what-survives-a-delete) |
| `copilot-cli` | home `~/.copilot/session-state/<id>/events.jsonl` | yes | `command-history-state.json`: last 50 prompts, no time, session or cwd; survives a delete; secondary. [what survives](copilot-cli.md#what-survives-a-delete) | `session.start` `context.cwd` (updated by `session.context_changed`); `workspace.yaml` `cwd`; `session-store.db` `sessions.cwd` | `session-store.db` `turns` (prompt and final reply) survive `/session delete` and `/rewind`; secondary. [what survives](copilot-cli.md#what-survives-a-delete) |
| `copilot-desktop` | home `~/.copilot/session-state/<id>/events.jsonl`, written by the bundled Copilot CLI and parsed as `copilot-cli`; the app's own records in `~/.copilot/data.db` | not observed; the session logs and `data.db` live under the home | none of its own observed | `data.db` `projects.main_repo_path`; `worktrees.path`; `workspace.yaml` `cwd` | The CLI's `session-store.db`, `data.db` activity rows and the checkpoint refs survive a delete; parsed. [what survives](copilot-desktop.md#what-survives-a-delete) |
| `crush` | the project: `<project>/.crush/crush.db` | no | none | the database's location; `~/.local/share/crush/projects.json` | A deleted session's sub-agent sessions stay in `crush.db`, parsed; `logs/crush.log` and `stats/index.html` outlive a delete, not parsed. [what survives](crush.md#what-survives-a-delete) |
| `cursor` | home `Cursor/User/globalStorage/state.vscdb`, rows per composer and bubble; the CLI's `~/.cursor/chats/<md5>/<id>/store.db` | yes (under the home; the CLI's sessions survived the project's removal) | IDE none; CLI `prompt_history.json` per session: the typed prompts, no time; not parsed. [what survives](cursor.md#what-survives-a-delete) | per conversation `workspaceIdentifier.uri.fsPath` in `composerHeaders` and `composerData`; `workspaceStorage/<md5>/workspace.json`; CLI `meta.json` `cwd` | The app's delete keeps the `composerHeaders` row, the `agentKv` prompt blobs, the transcript JSONL under `~/.cursor/projects` and the `ai-code-tracking.db` rows; the transcript secondary, the rest parsed. [what survives](cursor.md#what-survives-a-delete) |
| `gemini-cli` | home `~/.gemini/tmp/<slug>/chats/`; a container-sandboxed session leaves none on the host | yes | `tmp/<slug>/logs.json`: prompts with session and time; untouched by delete and retention; secondary. [what survives](gemini-cli.md#what-survives-a-delete) | per project directory (`.project_root`) | `shell_history`, `/chat save` checkpoints and `/bug` exports survive a delete and the 30-day retention, secondary; the shadow git and `memory/` not parsed. [what survives](gemini-cli.md#what-survives-a-delete) |
| `goose` | home `~/.local/share/goose/sessions/sessions.db` | yes | `~/.local/state/goose/history.txt`: last 100 CLI inputs, text only; secondary. The desktop app's `goose-chat-history` in local storage: last 500 prompts of 30 days, with times; not parsed ([issue 368](https://github.com/seanthegeek/doubleagent/issues/368)). [what survives](goose.md#what-survives-a-delete) | per session `working_dir` | `llm_request.<N>.jsonl` (last 10 provider calls) outlive `/clear`, a rewind and a delete; secondary. [what survives](goose.md#what-survives-a-delete) |
| `hermes` | home `~/.hermes/state.db` | yes | `~/.hermes/.hermes_history`: prompts with time, no session; untouched by a delete; secondary. [what survives](hermes.md#what-survives-a-delete) | per session `cwd` (updated) | `state-snapshots/`, `sessions/*.jsonl`, `cron/output/`, `response_store.db` and trajectory files, secondary; delete tombstones parsed. [what survives](hermes.md#what-survives-a-delete) |
| `kilo-code` | home `~/.local/share/kilo/kilo.db` | yes | `~/.local/state/kilo/prompt-history.jsonl`: last 50 TUI prompts, no time, session or directory; survives a delete; secondary. [what survives](kilo-code.md#what-survives-a-delete) | session `directory` and per message `path.cwd` | Project memory session digests survive a delete, parsed; pinned ids in `session.json`, secondary; `retention/state.json` counts pruned sessions. [what survives](kilo-code.md#what-survives-a-delete) |
| `kiro` | home `amazon-q/data.sqlite3`, the latest conversation per working directory only | yes | `~/.aws/amazonq/.cli_bash_history`: no time, session or directory; outlives `/clear`; secondary. [what survives](kiro.md#what-survives-a-delete) | the row key and per message `env_context` | The blob's checkpoint snapshots and tangent-mode main history outlive a rewind; secondary. Its `transcript` array outlives `/clear`; not parsed. [what survives](kiro.md#what-survives-a-delete) |
| `letta` | home `~/.letta/transcripts/` | yes | none | header `cwd` (local backend), else a heuristic | `transcript.jsonl` outlives an agent delete, secondary; `crons.json`, `runs/` and `listener-state/` outlive a server-agent delete, parsed. [what survives](letta.md#what-survives-a-delete) |
| `little-coder` | home `~/.pi/agent/sessions/` (v0.0.x: `~/.little-coder/sessions/`) | yes | `little-coder-prompt-history.json`: last 100 prompts, no time or session; survives a delete; parsed. [what survives](little-coder.md#what-survives-a-delete) | header `cwd` (v0.0.x: none) | The checkpoint directory outlives a delete, parsed; v0.0.x `history.json` and `input_history.txt` outlive pruning, secondary. [what survives](little-coder.md#what-survives-a-delete) |
| `muse-code` | home `~/.local/share/muse/sessions/` | yes | `tui-history.jsonl`: prompt with session and workspace, no time; parsed. [what survives](muse-code.md#what-survives-a-delete) | session `workspace_root` | `.msp-view-v1` view and `session-index.db` (first prompt); what a delete leaves is not stated; not parsed. [what survives](muse-code.md#what-survives-a-delete) |
| `nanobot` | home `~/.nanobot/sessions/<workspace-id>/sessions.sqlite3`; the JSONL files beside it are imported once and then frozen | yes | `~/.nanobot/history/cli_history`: time and prompt, no session; never truncated; secondary. [what survives](nanobot.md#what-survives-a-delete) | a `.workspace` file per directory | WebUI transcript, frozen JSONL, `jsonl-backup/`, sidebar cache, secondary; cron and trigger run records survive a delete, secondary. [what survives](nanobot.md#what-survives-a-delete) |
| `ollama` | app data `db.sqlite` (desktop app) | yes | `~/.ollama/history`: last 100 REPL inputs, text only; parsed. [what survives](ollama.md#what-survives-a-delete) | none | A REPL `/save` lives on as a model blob, which the collector excludes with the weights. [what survives](ollama.md#what-survives-a-delete) |
| `open-interpreter` | home `~/.openinterpreter/sessions/` | yes | `~/.openinterpreter/history.jsonl`: session, time and text; survives `thread/delete`; secondary. [what survives](open-interpreter.md#what-survives-a-delete) | `session_meta` and per turn | `state_5.sqlite`, `thread_history_1.sqlite` and `db-backups/` copies as for Codex, secondary; memory rollout summaries parsed. [what survives](open-interpreter.md#what-survives-a-delete) |
| `openclaw` | home `~/.openclaw/agents/<id>/agent/*.sqlite` | yes | none | header `cwd` | `.deleted` and `.reset` archive files, checkpoints and repair backups, secondary; archive blobs, cold archives, the full-text index and pending inputs, parsed; memory chunks not parsed. [what survives](openclaw.md#what-survives-a-delete) |
| `opencode` | home `~/.local/share/opencode/opencode.db` | yes | `~/.local/state/opencode/prompt-history.jsonl`: last 50 TUI prompts, no time or session; survives a delete; secondary. [what survives](opencode.md#what-survives-a-delete) | session `directory` and 1.x assistant `path.cwd` | Legacy 1.x rows of a migrated database, `opencode-next.db` and pinned ids in `session.json`, secondary; `shell/` output and the desktop `drafts.sqlite` survive a delete, not parsed. [what survives](opencode.md#what-survives-a-delete) |
| `openhands` | home `~/.openhands/conversations/<hex>/` | yes | `projects/<sha256(cwd)>/prompt_history.json`: last 100 prompts per directory, local time; survives a delete; secondary. [what survives](openhands.md#what-survives-a-delete) | per conversation `workspace.working_dir` | A fork, a full copy that survives its source's deletion, parsed as its own conversation. [what survives](openhands.md#what-survives-a-delete) |
| `pearai` | `~/.pearai/sessions/` and `globalStorage` tasks | yes | none | session `workspaceDirectory`; task `workspace` | The PearAI output channel log (every prompt and completion) and `dev_data/chat.jsonl` survive a delete, secondary. [what survives](pearai.md#what-survives-a-delete) |
| `pi` | home `~/.pi/agent/sessions/--<cwd>--/` (or `--session-dir` anywhere) | yes | none | header `cwd` | `pi-debug.log`, secondary, and `crashes.json`, parsed, survive a delete; the project exports, the bug-report zip and `pi-tui-crash.log`, not parsed. [what survives](pi.md#what-survives-a-delete) |
| `qwen-code` | home `~/.qwen/projects/<cwd>/chats/` (runtime base `QWEN_RUNTIME_DIR`) | yes | `~/.qwen/tmp/<sha256(cwd)>/logs.json`: prompts; the TUI's `/delete` purges a session's entries, a daemon delete does not; secondary. [what survives](qwen-code.md#what-survives-a-delete) | every record (`cwd`) | `usage_record.jsonl` keeps a deleted session's id, project and times, no text; secondary. The monthly token usage log, todo files and debug log survive; not parsed. [what survives](qwen-code.md#what-survives-a-delete) |
| `qwen-live` | home `~/.qwen-live/sessions/live-<ms>-<nonce>.jsonl`, one per daemon run | yes, it records no project | none | nowhere per call; `config.json` `defaultCwd` | `memories/<id>/dialogue.db` `turns` keeps each call's dialogue after its log is removed; not parsed. [what survives](qwen-live.md#what-survives-a-delete) |
| `roo-code` | `globalStorage` `tasks/<id>/` or `~/.vscode-mock` | yes | CLI `~/.roo/cli-history.json`: newest 500 lines, text only; outlives a deleted task; secondary. [what survives](roo-code.md#what-survives-a-delete) | per task `workspace` | Up to 3.53.0, the Cloud retry queue keeps a failed upload's message past a delete; one row per request, message not parsed. [what survives](roo-code.md#what-survives-a-delete) |
| `shellgpt` | the system temp directory `chat_cache/<id>` | yes, but lost to temp cleanup instead | none | none | The response cache: newest 100 answers, no prompt, time or session; outlives the chat; secondary. [what survives](shellgpt.md#what-survives-a-delete) |
| `tabby` | home `~/.tabby/ee/db.sqlite` | yes | editor inline-edit command histories (IntelliJ XML, VS Code `state.vscdb`), no times; secondary | none (`git_url` only) | `db.backup-*.sqlite` copies and the `events/*.json` log, secondary; AI-written pages parsed. [what survives](tabby.md#what-survives-a-delete) |
| `twinny` | the editor's `state.vscdb` | yes | none | none in the chats; since 4.3.5 the `agent-changes/` records carry the workspace `root` | The `agent-changes/` records (newest 50) and the log channel survive a delete; secondary. [what survives](twinny.md#what-survives-a-delete) |
| `vscode` | user data `workspaceStorage/<hash>/chatSessions/` | yes | none | `workspace.json` per hash; `workingDirectory` | `chatEditingSessions/` edit states survive a delete, secondary; Copilot Chat's `session-store.db`, hook transcripts and debug logs too, not parsed. [what survives](vscode.md#what-survives-a-delete) |
| `zed` | data directory `threads/threads.db` | yes | none | per thread `folder_paths` | `sidebar_threads` (metadata only); a thread's `created_git_worktrees` entry survives a delete, parsed. [what survives](zed.md#what-survives-a-delete) |

What the table shows:

- Only Aider and Crush keep the transcript itself inside the project, so
  only for those two does deleting the project delete the evidence;
  ShellGPT's chats are lost to temp-directory cleanup instead. Pi's
  exports, Letta's session pointers, OpenHands' hooks file and the Muse
  and Claude Desktop worktrees are project-local too, but none is the
  primary transcript.
- Antigravity is the one agent whose history line is written before its
  session exists: the first prompt's line has no `conversationId`
  (Muse's `.session.jsonl.permission-init`, written before
  `session.jsonl`, is the nearest analogue). Claude Code's and Codex's
  history lines outlive their transcripts rather than precede them.
- Second copies that survive the loss of the primary store exist for
  Antigravity, Claude Desktop, Cline, Hermes, OpenClaw, Tabby, nanobot,
  Letta, Codex and Open Interpreter; the analyzer shows each event once
  across them
  ([issue 122](https://github.com/seanthegeek/doubleagent/issues/122)).
- A transcript can also survive in another agent's store: Hermes Agent,
  Cline's SDK and CLAI import Claude Code and Codex sessions, and the
  copy outlives the original file
  ([claude-code.md](claude-code.md#transcript-stores),
  [codex-cli.md](codex-cli.md#transcript-stores)).
- Retention deletes old transcripts in Claude Code (30 days; its one
  delete command, `claude purge`, removes a whole project), and with no
  delete command in Gemini CLI (30 days by default) and Kilo Code (an
  opt-in pass); in each a prompt history outlives what retention
  removed ([claude-code.md](claude-code.md#what-survives-a-delete)).
