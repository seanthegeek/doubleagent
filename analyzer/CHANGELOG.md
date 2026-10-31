# Changelog

All notable changes to the analyzer are recorded here. The version is
printed by `--version`. Changes to the `timeline.jsonl` and
`sessions.jsonl` fields (`timeline.csv` and `sessions.csv` columns up to
0.7.0) are called out, since they are interfaces.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

## [0.22.0] - 2026-10-30

### Added

- The bundled catalog copy gains the catalog lines of collector 1.20.0, so detection follows them: a Hermes home or desktop user-data directory with any `HERMES_DATA_DIR_SUFFIX` (`.hermes?*`, `AppData/Local/hermes?*` and `Hermes?*` under each OS's application data directory, replacing `.hermes_*` and `AppData/Local/hermes_*`) is detected as `hermes` ([#324]); and in `AppData/Roaming/devin` only the Devin CLI's `mcp_config.json` and `credentials.toml` are attributed to `windsurf`, not the Devin IDE's files beside them ([#321]).
- The bundled catalog copy gains the collector's new Hermes secret globs; the secret globs change as in the collector's Unreleased entry ([#326]).
- The bundled catalog copy gains the Goose lines of collector 1.21.0: the cache directory, the Windows config and data directories in place of `AppData/Roaming/Block/goose`, the project `diagnostics_*.json` file, the new Goose secret globs (the custom provider files and the temporary `.tmp??????` copies of its credential files among them) and exclusion, and no secret glob on the request logs ([#326], [#333]).
- The bundled catalog copy gains the OpenClaw lines of collector 1.21.0: the new secret globs and exclusions under `.openclaw`, `.clawdbot` and `.moltbot`, no `openclaw` volume lines, and a `claude-code` line for OpenClaw's `*openclaw_home` volume ([#326]).
- The `kilo-code` parser reads Kilo Code's project memory session digests, `.local/share/kilo/memory/<folder>/sessions/*.md`: one `system` row `memory digest: <topic>: <summary> (from <file>, Updated)` per digest, at its `Updated:` time, with the session id of its heading and the `canonical` project root of the folder's `manifest.json`; a digest without a `## Summary` line, or a `manifest.json` that is not a JSON object, is a `parser:` row ([#326]).
- The bundled catalog copy gains collector 1.21.0's Kilo Code project config secret globs and JetBrains plugin lines, so its `kilo` folder and `options/kilo*.xml` settings are detected as `kilo-code` ([#326]).
- The `opencode` parser reads an assistant message's content items from its `session_part` rows, which OpenCode 2.2.5 updates but only a newer release creates, in `ordinal` order and in place of the message's inline `content`, as OpenCode's own reader does; a `session_part` row whose `data` does not decode is one `parser: unparseable record <rowid> (session_part.data: <reason>): <data>` row ([#194], [#326]).
- The bundled catalog copy gains collector 1.21.0's OpenCode lines: the `pty/*/service.json` and `cli.json*` secret globs, the `service*.json*` globs of the config and state directories and the desktop app's `cli/` exclusions ([#326]).

### Fixed

- The `hermes` parser reads a home with `HERMES_DATA_DIR_SUFFIX` appended with no separator (`.hermes<suffix>` and `AppData/Local/hermes<suffix>`, such as `.hermes-dev`, as Hermes writes it) and the desktop app's suffixed `Hermes<suffix>/connections.json`; it read only `.hermes_<suffix>`, so such a home was detected but not parsed ([#324]).
- The `openclaw` parser reads a cold archive stored in the database in chunks (`session_transcript_cold_archive_chunks`, agent schema 37), chunk by chunk; such generations gave no rows before. A database-stored cold archive or chunk with no bytes is a `parser: cold archive <name> has no stored bytes` row, not silence ([#206], [#326]).
- The `kilo-code` parser reads the pinned session ids of the Kilo Code TUI's `.local/state/kilo/session.json`, as it does OpenCode's, giving one `session pinned:` row per id; it had skipped the file, assuming Kilo's TUI kept none ([#326]).
- A Kilo Code remote control log or pre-fork extension task file that cannot be read is now one `parser: <file> could not be read (<reason>)` row, as is an unreadable memory digest, instead of a `problem:` line ([#326]).
- An OpenCode 1.x session file whose project file (`storage/project/<projectID>.json`) exists but does not decode is now one `parser: project file <projectID>.json could not be read (<reason>)` row instead of a session with no project path and no row saying why; the `kilo-code` parser reads Kilo Code's 1.x JSON trees through the same reader and gives the same row ([#326]).

## [0.21.0] - 2026-10-28

### Added

- The bundled catalog copy gains the Claude Code macOS lines of collector 1.19.0 (the `claude-cli` URL handler bundle under `~/Applications`, the relocated-home `ide/*.lock` and `bridge-spawn/*/mcp.json` entries, and the GitHub CLI `hosts.yml` lines under the `shared` agent), so detection follows them ([#256]).
- The bundled catalog copy has the collector's Claude Code lines from the
  [Windows round](../research/claude-code.md#observed): the Windows temp
  root `AppData/Local/Temp/claude` and the
  project's `.git/info/exclude` are detected, `.claude/paste-cache` is no
  longer excluded, `.claude/plugins/plugin-directory-cache*` is, and the
  session daemon's keys and the `.claude.json` backups are secrets, in
  `.claude` and, with the session keys, in a `~/.claude-<name>` config
  home ([#256]).
- The `claude-code` parser yields `session synced: remote control bridge
  <bridgeSessionId> url - (from <path>, file mtime)` from a
  `.claude/sessions/<pid>.json` registry file that names a bridge id, at
  the file's modification time, so a session whose transcript is gone but
  whose registry file shows Remote Control attached reads `synced` `yes`
  in `sessions.jsonl` rather than `no` ([#256]).
- The URL of a `session synced: remote control bridge` row is the
  `bridge_status` record that belongs to that bridge, the first at or
  after the `bridge-session` record up to the next bridge or detach, else
  the nearest before it back to the previous one, else `-`; a second
  bridge in one transcript took the first bridge's URL before ([#256]).
- The bundled catalog copy has the collector's seven new project lines,
  `.git/HEAD`, `.git/config`, `.git/FETCH_HEAD`, `.git/packed-refs`,
  `.git/refs`, `.git/logs` and `.git/worktrees`, and its exclusion of
  each linked worktree's staging area and `modules/`, so the analyzer attributes a collected
  project's repository files to `project`, the shared project files, and
  `detect` counts them under `(shared)` ([#253]).
- `detect --json` gains `remotes` and `head` as the last two keys of each
  `projects[]` object, new keys of that interface: the remotes of the
  project's `.git/config` (`name`, and `urls` and `pushurls`, every
  `url` and `pushurl` value as the file records them, in file order, so a
  second push destination is kept; sorted by name) and the branch its
  `.git/HEAD` names, or
  the commit of a detached HEAD; `[]` and `""` when the collection holds
  neither file, and also when one of them cannot be read, which is then a
  `problem:` line. The `detect` text output adds a `remotes:` line under
  each project that has any ([#253]).
- `inventory.json` gains `remotes`, `head` and `recorded_remotes` as the
  last three keys of each `projects.<path>` and `scratch_directories.<path>`
  entry, new keys of that interface: `remotes` and `head` as in `detect
  --json` (`[]` and `""` for a `referenced` project), and
  `recorded_remotes`, the sorted distinct URLs of the `git remote:` rows
  of the sessions counted under the project after the filters, so a
  project whose directory is gone still names the remote its agent
  recorded. `outside_projects` has none of the three ([#253]).
- `git remote: <url>` `system` rows, once per session per URL at the time
  of the record that names it: the `codex-cli` and `open-interpreter`
  parsers read the rollout's `session_meta` `git.repository_url` and the
  `state_5.sqlite` `threads.git_origin_url` (a
  [secondary source](docs/output.md#secondary-sources), so the
  timeline keeps one of the two); the `zed` parser reads the thread
  snapshot's `git_state.remote_url`; the `openclaw` parser reads the
  `projects` table's `origin_url` of `state/openclaw.sqlite`, with the
  project path its `repo_root` and no session ([#253]).
- The `claude-code` parser writes `session surface: <entrypoint>` at the
  first record of each transcript file (`cli`, `claude-vscode` for the VS
  Code extension, `claude-desktop`), and `session surface: <old> -> <new>`
  when it changes ([#256]).
- The `claude-code` parser writes `prompt trigger: promptSource=<v>
  turnOrigin=<v>` after a prompt that was not typed by the person at the
  terminal: one sent from a Remote Control client (`queued`), from the VS
  Code extension (`sdk`), a suggestion the person accepted
  (`suggestion_accepted`), a `/loop` firing (`turnOrigin=scheduled`) or a
  background task's completion ([#256]).
- The `claude-code` parser writes `scratch directory: <path> (session
  scratchpad, from the environment attachment)` for the session
  scratchpad a transcript's `environment` attachment names, so
  `inventory.json` lists it under `scratch_directories` ([#256]).
- The `claude-code` parser writes `session continued in: <session id>`
  for a 2.1.309 `continued-in` record, the session daemon's handover of a
  console session to the worker session that holds the later turns, and
  `remote control: bridge session <id or none> lastSequenceNum=<n>` for a
  Remote Control `bridge-session` record ([#256]).
- The `claude-code` parser writes `retention setting:
  cleanupPeriodDays=<n> (from <file>, file mtime)` from a settings file,
  after its `permission setting:` rows, because the period decides which
  transcripts Claude Code's own retention sweep has deleted; a session
  known only from `history.jsonl` is what a swept session leaves
  ([#256]).
- The `claude-code` parser writes `source host: github status=<status>
  (from .claude.json, file mtime)` from the `githubWebConnectionStatusCache`
  that Claude Code 2.1.310 keeps in `.claude.json` once the GitHub app is
  connected; the account and organization UUIDs beside it are not
  printed ([#256]).
- The `claude-code` parser records where a session began, so its sessions
  have `origin` `remote` or `local` in `sessions.jsonl` (and the
  inventory's `remote_sessions` follows): `session origin: remote (Remote
  Control, entrypoint sdk-cli)` for a session started from claude.ai
  through `claude remote-control`, `session origin: remote (teleport,
  remoteSourced)` for a cloud session brought local with `claude
  --teleport`, `session origin: remote (teleport, cloud scratchpad
  <path>)` in place of the scratch directory row for the cloud machine's
  replayed scratchpad, and `session origin: local (VS Code extension)`.
  `claude-desktop` sessions keep `origin` `""` ([#256]).
- The `claude-code` parser reads `projects/<slug>/<session>/ccr-tip.json`
  as `session synced: remote event <eventId> at <updatedAt> (from <file>,
  file mtime)`, so a Remote Control session reads `synced` `yes` after
  its bridge has stopped, and `projects/<slug>/bridge-pointer.json` as
  `remote control: environment <environmentId> session <sessionId>
  source=<source> (from <file>, file mtime)` ([#256]).
- A teleported cloud session (`remoteSourced` records) gets `session synced: cloud session (teleported: remoteSourced records replayed from the vendor's copy)` beside its origin row, so it reads `synced` `yes` like a Remote Control spawn: the conversation is held on Anthropic's servers ([#256]).
- The `claude-code` parser writes `remote control: spawn mode <mode> for
  <path>`, `remote control: machine id set` and `cloud:
  hasRemoteEnvironment=<value>` rows from `.claude.json`, and `model
  setting: model=<alias>` from a settings file's `model`, each `(from
  <file>, file mtime)`; the machine id is not printed ([#256]).

### Changed

- `claude-code` sessions now have `synced` `yes` or `no` in
  `sessions.jsonl` (and the inventory's `synced_sessions` and project
  `synced` follow): a session with a Remote Control bridge gets a
  `session synced: remote control bridge <id> url <url>` row, because the
  bridge puts the session on claude.ai. `claude-desktop` sessions keep
  `synced` `""` ([#256]).
- A `scratch directory:` row no longer decides a session's
  `project_path` in `sessions.jsonl` when another row of the session has
  a path; it is used only when no other row has one ([#256]).
- The `claude-code` parser takes a persisted tool output's file from the
  2.1.309 `toolUseResult.persistedOutputPath` when the line has one tool
  result, and from the `<persisted-output>` stub's text otherwise; the
  file must still be in the session's own `tool-results/` ([#256]).
- A `claude-code` denial recorded with `toolDenialKind` and no decision
  object, which Claude Code 2.1.310 writes on macOS for `No` at the
  permission dialog and for a declined plan, now reads `permission
  decision: reject (interrupted) toolDenialKind=<kind>` instead of
  `permission decision: - source=- reason=- toolDenialKind=<kind>`
  ([#256]).
- A `scheduled_task_fire` record (a `/loop` firing) is now `scheduled task
  fired: <taskId> cron=<cron>` instead of `scheduled_task_fire: <content>`
  ([#256]).
- A `claude-code` `history.jsonl` row now carries the pasted text in place
  of a `[Pasted text #<n> +<m> lines]` placeholder, from the line's
  `pastedContents` or `paste-cache/<contentHash>.txt` when either still
  holds it, as the transcript's prompt does ([#256]).
- The errors the analyzer reports to the analyst share one exception
  class, `doubleagent.errors.DoubleAgentError`: a bad `--since`,
  `--until` or `--match`, an input that is neither a file, a directory
  nor an extractable archive, and a VS Code mutation-log record that
  cannot be applied raise it or a subclass, and none of them is a
  `ValueError`. The messages and exit codes are unchanged ([#277]).
- The package builds with the `hatchling` backend instead of
  `setuptools`; the wheel holds the same `doubleagent` package and
  `catalog.txt`. The test suite runs under pytest, which
  `requirements-dev.txt` installs: `tests/run.sh` passes its arguments to
  `python3 -m pytest`, so a test is named by its pytest node id
  (`tests/test_catalog.py::CatalogTests`) rather than its unittest name
  ([#277]).

### Fixed

- `claude-code` sessions no longer read `recovered` `partial` or `yes`
  because of their own slash commands and pastes: a `history.jsonl` line
  with a command as typed (`/model`, `/help`, `/rename lab-round`) now
  pairs with the transcript's tag-form command record, within 600 seconds
  because a command that opens a picker is recorded when it completes, and
  a line with a paste with the prompt that wraps it in `<pasted_content>`,
  so only lines no transcript records match stay recovered: `/clear`
  (filed under the ended session), `/logout` and an `/exit` typed during
  a turn. The match keeps whitespace (only runs of it collapse), the wider
  window reaches forward only and against transcript records only, a line
  whose own record is missing keeps its own time instead of taking a later
  line's record, and a `paste-cache` file that is there but cannot be read
  gives a `parser:` row naming it ([#256]).
- The `cody` parser reports each chat interaction it cannot decode in a
  damaged chat history as its own `parser: unparseable record <N>
  (<reason>): <record>` row with the chat's session id, beside the rows
  of the chats it recovered; the record was dropped silently before
  ([#280]).
- A relative `--since` or `--until` value past Python's integer
  conversion digit limit (4300 by default) gives the usual out-of-range
  error instead of Python's own conversion message, and every rejected
  bound's message now ends with the accepted forms, the out-of-range
  relative and epoch values included ([#283]).
- Detection recognises the section headings of the bundled catalog by
  their full text, so a comment inside a table that starts with a
  heading's first words (`excluded`, `credential files`) can no longer
  move the lines after it into another table and leave their agents
  undetected ([#285]).
- The `crush` and `gemini-cli` parsers give a `parser:` row, with the
  file's project, for an unreadable Crush `projects.json`, Gemini CLI
  `logs.json` or legacy Gemini CLI session file, so the failure sits in
  the timeline beside the file's other rows; before, the error left the
  parser and the command reported it as a `problem:` line ([#282]).
- The `codex-cli`, `continue`, `copilot-cli`, `goose` and `hermes`
  parsers end a line of the YAML, Starlark, Markdown and log files they
  read, and on Python 3.10 the TOML files (3.11 and later read TOML with
  the standard library), at LF or CRLF only and trim only the whitespace
  the format defines, so a value holding a Unicode line or paragraph
  separator (U+2028, U+2029, U+0085) or a form feed is no longer split
  across lines, and one with a no-break space or another Unicode space at
  its edge is no longer trimmed. On Python 3.10, where the analyzer reads
  TOML itself, `automation.toml` keeps the spaces before a multi-line
  basic string's first line end. It also keeps a line ending in an
  escaped backslash (two backslashes) instead of joining the next line.
  On 3.10 the Codex TOML readers also reject what the standard library's
  reader on 3.11 and later rejects: an unquoted value padded with a
  Unicode space, an integer with a leading zero, a non-ASCII digit or a
  misplaced underscore, and a backslash before a lone space in a string.
  Such a value gives one `parser:` row for `automation.toml`; on 3.10
  `config.toml` leaves the entry out of its `permission setting:` rows
  with no `parser:` row ([#281]).
- The examples of the plain `detect` output, `detect --json`, the
  `timeline` stdout, `inventory.json`, `timeline.jsonl` and
  `sessions.jsonl` in [`docs/output.md`](docs/output.md) are now the
  analyzer's output on a synthetic collection, built by the new
  `tools/example_collection.py` and regenerated by the new
  `tools/regenerate_examples.py`; the collection holds several Claude
  Code sessions, a Remote Control and a teleported one among them, a
  Codex CLI thread, settings, a prompt history that names a deleted
  session, a cut transcript and a cut manifest row, and
  [The example collection](docs/output.md#the-example-collection)
  describes it. Before, the `inventory.json` example showed analyzer
  0.14.0, keys of the `detect --json` and `inventory.json` examples had
  been filled in by hand, the project table under `detect` listed files
  its collection never held, and the `timeline` stdout and the two JSONL
  files had no complete example. `tests/test_examples.py` fails when an
  example is stale ([#269]).

## [0.20.1] - 2026-10-21

### Added

- Source-derived rows for the open-source agents, from the 2026-10-19
  review's issues; each agent's research document marks them
  source-derived until its
  [real-install round](../AGENTS.md#real-install-rounds). The lines below list them
  per agent.
- The `agent-zero` parser writes `permission decision:` rows for tools a
  policy blocked, provider MCP approvals denied automatically and
  Infection Check verdicts, and `permission setting:` rows from the
  `_tool_access` and `_orchestrator` `config.json` files and the
  Infection Check and Orchestrator plugin switches ([#204]).
- The `agent-zero` parser writes an editor (ACP) session's mode, model
  and container working directory (`permission mode:`, `model:
  requested`, `scratch directory:`), takes the workspace `acp_cwd` as the
  project path, and writes the per-chat model override without its API
  key ([#204]).
- The `agent-zero` parser reads `usr/scheduler/tasks.json` and the
  Telegram `state.json` as secondary sources; a mapped Telegram chat that
  is gone gives a `session deleted:` row ([#204]).
- The `agent-zero` parser writes a `session origin: remote` row from the
  A0 CLI connector's last chat, so `origin` reads `remote` or `local` for
  Agent Zero sessions ([#204]).
- The `aider` parser writes `model: <name>` and `model: <old> -> <new>`
  rows from the startup and `/model` announcement lines, with ` (editor
  model)` and ` (weak model)` for those roles, and fills the `model`
  column on them ([#203]).
- The `aider` parser writes a `permission decision:` row for each
  confirmation answer in the chat history; a yes or no notes that it may
  have come from `--yes-always` ([#203]).
- The `cline` parser reads the SDK hook audit log
  `~/.cline/data/logs/hooks.jsonl` as a secondary source, recovering the
  prompts, tool calls, results and final replies of sessions whose
  transcripts are gone, plus `hook audit:` rows for run starts, aborts,
  errors and stale-session shutdowns ([#190]).
- The `cline` parser writes `permission decision: rejected` or `blocked`
  beside SDK tool results the user rejected or the policy blocked, and
  `permission setting:` rows for `autoApprovalSettings` and `mode` in
  `globalState.json`, the tool settings in
  `settings/global-settings.json`, and each MCP server's `autoApprove` and
  `disabled` ([#190]).
- The `cline` parser writes `session origin: remote` and `session
  synced:` rows for the Cline Cloud sessions in `globalState.json`
  `cloudSessionStatuses` and `cloud-pending-starts/*.json`, and a `cloud
  task target:` row for the last cloud task target; a session imported
  from Claude Code, Codex or OpenCode names its source agent, session and
  path on its start row ([#190]).
- The `codex-cli` and `open-interpreter` parsers write `permission
  setting:` rows for the standing grants in `config.toml` and
  `rules/*.rules`, a `thread goal:` row per `thread_goal_updated` event
  and a `memory summary:` row per `memories/rollout_summaries` file, and
  read `db-backups/*/state_5.sqlite`, a copy Codex set aside as corrupt,
  as a secondary source ([#200]).
- The `codex-cli` parser writes `permission decision:` rows for
  `apply_patch` approvals in `logs_<N>.sqlite` and for failed automatic
  reviews in `state_5.sqlite` `guardian_review_feedback` ([#200]).
- The `cody` parser writes `permission setting:` rows for the Cody keys
  of the editor's `User/settings.json` (Deep Cody shell allow and block
  lists, agentic context switches, MCP server and tool toggles) and
  `model setting:` rows from `cody-model-preferences`, in the VS Code
  state row and the JetBrains global-state file ([#197]).
- The `cody` parser reads a JetBrains project's legacy
  `.idea/cody_history.xml` as a secondary source, recovering chats and
  unpaired messages the agent store no longer holds ([#197]).
- The `continue` parser writes a `permission setting:` row for each rule
  in the CLI's `permissions.yaml`, `permission decision: allow always`
  for the `cn.log` "Added policy rule" line, the reason a call was denied
  (`canceled reason=user`, `canceled reason=policy`, `errored
  reason=policy` for a Security Policy Violation), and a `permission
  mode:` row for a session's mode ([#198]).
- The `continue` parser reads the CLI's `input_history.json` and the
  `chatFeedback` events as secondary sources, parses the `editInteraction`
  and `editOutcome` events (`permission decision: edit ...`), and lists a
  `cn review` worktree as a scratch directory ([#198]).
- The `crush` parser writes a `permission decision:` row after a tool
  call the user denied or a `PreToolUse` hook allowed, denied or halted,
  and `permission setting:` rows from every `crush.json`, the `crushrc`
  `permissions allow|deny` lines and the per-project MCP server overrides
  ([#195]).
- The `crush` parser writes `session relayed:` rows for sessions driven
  by an MCP channel, a `session deleted:` row for the missing parent of
  an orphaned sub-agent session, and `file version:` and `file read:`
  rows from the `files` and `read_files` tables ([#195]).
- The `gemini-cli` parser writes `permission setting:` rows from
  `settings.json` (permission, sandbox and MCP lists, in the home and the
  project), the policy files under `.gemini/policies/` (Python 3.11 or
  later; 3.10 gives one `parser:` row), `trusted_hooks.json` and
  `acknowledgments/agents.json`, and `session retention:` rows naming a
  `general.sessionRetention` setting, so an expired session can be told
  from a deleted one ([#186]).
- The `gemini-cli` parser reads `shell_history` and the `/chat save`
  checkpoints as secondary sources ([#186]) and the `/bug` history exports
  `bug-report-history-<ms>.json` too ([#155]).
- The `goose` parser writes `permission setting:` rows for
  `permission.yaml` and `roaming_trust.json` entries at the file's
  modification time, and one `scheduled task:` row per `schedule.json`
  job ([#209]).
- The `hermes` parser writes `session relayed:` rows for messaging-gateway
  sessions, `relay peer:`, Bot Mode room and desktop `remote backend:`
  rows, and `scratch directory:` rows for the git worktrees the agent
  created ([#208]).
- The `hermes` parser writes terminal approval outcomes as `permission
  decision:` rows and `/yolo`, `command_allowlist` and pairing approvals
  as `permission setting:` rows; `state_meta` deletion fences give
  `session deleted:` rows, and start rows mark archived and imported
  sessions ([#208]).
- The `hermes` parser reads `.hermes_history`, the cron run files,
  `response_store.db` and the trajectory files as secondary sources
  ([#208]).
- The `kilo-code` parser reads the config `permission` grants, the
  sandbox policy and preference files, `retention/state.json` and the
  `remote control:` lines of its logs ([#192], [#221]).
- The `kiro` parser writes a `permission decision:` row after each
  cancelled Amazon Q Developer CLI tool call (a bare `n`, a deny rule, an
  interrupt or the user's own words), and one `permission setting:` row
  per `allowedTools` entry and `toolsSettings` key of each global or
  project `cli-agents/*.json` file ([#212]).
- The `kiro` parser reads the prompt history `.aws/amazonq/.cli_bash_history`
  as a secondary source, recovers turns only a conversation's checkpoint
  snapshots or tangent-mode main history still hold, and turns the
  delegate tool's `.amazonq/.subagents/<agent>.json` records into launch
  and completion rows ([#212]).
- The `letta` parser writes `permission setting:` rows from the four
  settings files, `session relayed:` rows from the channel routes, and
  parses the scheduled tasks (`crons.json`), their run logs
  (`runs/*.jsonl`) and interrupted listener turns with their approval
  decisions and tool results ([#202]).
- The `little-coder` parser reads the v0.0.x `~/.little-coder` layout:
  saved sessions, `history.json`, `session_latest.json`,
  `input_history.txt` and checkpoint snapshots, and from its
  `config.json` `session synced: gist`, `session relayed: telegram chat`
  and `permission setting: permission_mode` rows, never reading its
  tokens ([#157]).
- The `little-coder` parser writes `permission decision: denied
  (<reason>)` after a call its permission gate refused ([#201]).
- The `nanobot` parser writes `session relayed:` rows for sessions keyed
  to or routed through a messaging channel, `permission setting:` and
  `permission mode:` rows from `config.json`, `pairing.json` and the
  WebUI workspace scope, and `model: <old> -> <new> (preset)
  requested=... resolved=...` rows for `/model` switches ([#207]).
- The `nanobot` parser reads the cron and trigger run records and the
  WebUI sidebar cache as secondary sources, recovering the scheduled
  turns of deleted sessions ([#207]).
- The `ollama` parser writes a `permission setting:` row for each
  launcher integration's `automode` key in `.ollama/config.json`, and for
  the `autoModeEnabled` key it writes into Claude Desktop's third-party
  profile, at each file's modification time ([#188]).
- The `openclaw` parser writes `session relayed:` rows for sessions
  carried through a chat platform or started by a Gmail hook, `permission
  decision:` rows for Gateway approvals, and `permission setting:` rows
  for the exec approvals policy, standing cron grants, allowed channel
  senders and per-session settings ([#206]).
- The `openclaw` parser writes a `model:` row for a session's model
  override, `channel delivery:` and `channel ingress:` rows, `session
  deleted:`, `session archived:` and `session forgotten from memory:`
  rows, and `source host: github account <login>` rows for the pushes and
  pull requests the Gateway made; it recovers prompts that never reached
  the transcript and entries only the full-text index still holds
  ([#206]).
- The `opencode` and `kilo-code` parsers write `permission decision:`
  rows for refused tool calls, `permission setting:` rows for saved
  grants and session rule lists, `session archived` rows and `scratch
  directory:` rows for the agent's own worktrees ([#194], [#192]).
- The `opencode` and `kilo-code` parsers read the terminal UI's
  `prompt-history.jsonl` as a secondary source and `prompt-stash.jsonl`
  as `prompt stashed (unsent):` rows; `opencode` also reads the pinned
  sessions in `session.json` ([#194], [#192]).
- The `openhands` parser writes a `permission decision: analyzed
  risk=<risk> analyzer=<analyzer> policy=<policy>` row for a security
  analyzer's verdict, joined to the call, and `permission setting:` rows
  from each conversation's `base_state.json` ([#205]).
- The `openhands` parser reads the CLI's `prompt_history.json` as a
  secondary source, and writes a `scratch directory:` row for a
  conversation in the Agent Server's default worktree root,
  `/tmp/conversation-worktrees` ([#205]).
- The `pearai` parser writes `permission decision:` rows for the denial
  and approval texts of the agent surface (the Roo Code-based agent pane,
  as opposed to the chat), one row per `taskHistory`
  entry and per standing auto-approval setting in the editor's
  `state.vscdb`, and one `tokens generated:` row per model request from
  `dev_data/tokens_generated.jsonl` ([#193]).
- The `pearai` parser reads the `PearAI` output channel log and
  `dev_data/chat.jsonl` as secondary sources ([#193]).
- The `pi` parser writes a `permission setting:` row for
  `defaultProjectTrust`, reads `.pi/agent/pi-debug.log` as a secondary
  source, and writes one `crash:` row per `.pi/agent/crashes.json` record
  ([#201]).
- The `qwen-code` parser writes `permission decision: <decision>
  (tool_call ui_telemetry)` rows joined on the call id (the event named
  `qwen-code.tool_call` or `tool_call`), `model:` and `permission mode:`
  rows from `executionContext` and `session_approval_mode` records, and
  `permission setting:` rows from `.qwen/settings.json` ([#187]).
- The `qwen-code` parser reads `~/.qwen/usage_record.jsonl` as a
  secondary source, one `session usage:` row per session summary ([#187]).
- The `roo-code` parser writes `permission decision:` rows for approved
  tool asks and for the denial and approval results in the API history,
  `permission mode:` for each task, and `permission setting:` rows from
  the global state, `.roo/cli-settings.json` and the MCP `alwaysAllow`
  lists ([#191]).
- The `roo-code` parser writes `sync setting:` rows from the Roo Code
  Cloud settings cache and `cloud upload queued:` rows from the cloud
  retry queue of releases up to 3.53.0 ([#159]).
- The `shellgpt` parser writes `permission setting:` rows for three
  `.sgptrc` keys, a `model: ... (configured default, ...)` row for
  `DEFAULT_MODEL` and one row per function-store file, and reads no
  other `.sgptrc` key; it reads the response cache as a secondary source
  ([#189]).
- The `tabby` parser reads the server's AI-written pages as `page:<id>`
  sessions, `model configured:` rows from `.tabby/config.toml` (Python
  3.11 or later), the IntelliJ and VS Code inline-edit command histories
  as secondary sources, and `source host:` and `source host repository:`
  rows from the GitHub and GitLab integrations ([#211]).
- The `twinny` parser writes `permission mode:` and `permission setting:`
  rows for Twinny's standing grants and the accepted team gateway policy
  ([#154]), a `file changed:` row per file each reply changed, reads the
  `agent-changes/` records and the Twinny log as secondary sources, and
  writes a `source host repository:` row for the repository the review
  tab last listed ([#199]).
- The `vscode` parser writes `permission level:` rows, the model's options
  on `model:` rows, `permission setting:` rows for the standing tool
  approvals and the auto-approve and default-permission settings, a
  `model setting:` row for `chat.defaultModel`, and `source host:` and
  `source host repository:` rows; it reads the chat editing state
  `chatEditingSessions/<id>/state.json` as a secondary source (`edit
  state:` rows) ([#210]).
- The `zed` parser writes standing `permission setting:` rows from
  `settings.json`, `global_settings.json` and the `trusted_worktrees`
  table, `permission decision: denied` and `auto_denied` rows,
  `scratch directory:` rows for the macOS sandboxed-terminal temp
  directory and Zed-created worktrees, and `unsent draft:`, `archived
  worktree:`, `sidebar terminal:` and `sidebar thread:` rows ([#196]).

- The `cursor` parser reads Cursor cloud agents, which run on Cursor's
  servers and keep no conversation records in the editor's database.
  Each entry of the `cloudAgentRepository.agents.<account>` list in
  `state.vscdb` becomes a session with the agent's `bc-<id>` as
  `session_id` and its repository (`github.com/<owner>/<repo>`) as
  `project_path`, with `session origin: remote cursor cloud agent`,
  `session synced: cursor cloud agent <bc-id> (vendor copy; ...)`,
  `model: <model>` (with ` requested=<id>` when the requested id
  differs), `source host: github.com <owner>/<repo> (cloud agent)` and
  `cloud agent: <name> repo=... status=... workflowStatus=...
  archived=... killed=... vmStarted=...` rows; such a session's `origin`
  is `remote` and its `synced` `yes` in `sessions.jsonl` and the
  inventory, since Cursor holds the conversation, and every other Cursor
  session's `synced` now reads `no` instead of `""`. The repository list
  `cloudEnvironmentTemplate.githubRepos` gives one `source host
  repository: <url> type=<type>` row per repository, at the file's
  modification time. The encrypted `secret://` connection tokens beside
  them are never read, and the protobuf `bcCachedDetails:<bc-id>` record
  is not decoded yet ([#236]).
- The `cursor` parser reads the text `conversation-search.db` holds for a
  conversation, from its full-text index's `conversation_fts_content`
  table (read as a plain table, so SQLite's fts5 module is not needed):
  one `cached conversation text (conversation-search.db,
  source=<source>): <text>` row per conversation with text, a single
  `system` row because the index runs the prompt and the reply together.
  For a cloud agent's `cloud-cache` row this is the only local copy of the
  conversation's text, and the row also gives the session's `session
  origin: remote cursor cloud agent` row ([#236]).

- The `cursor` parser reads the editor's MCP servers and plugins:
  `mcp server: <server> url=<url> (from state.vscdb mcpOAuth.global,
  mcp_oauth_updated_at_ms)` per MCP server with OAuth state, ending
  ` tokens=stored (ciphertext, not read)` when the editor holds the
  server's OAuth tokens, which are encrypted and never read (the parser
  decodes only the key names of `mcpOAuth.*`); `plugin installed: id=<id>
  sources=<sources> scope=<scope>` per installed plugin, with the folder
  as `project_path` when the plugin is enabled for one; and `mcp oauth
  attempt: server=<url> client_id=<id> workspace=<workspace>` from the
  plaintext `User/globalStorage/mcp-oauth-attempts/<uuid>.json`, the
  OAuth client registration of the server's sign-in ([#236]).
- The `cursor` parser yields `permission decision: <decision> mcp auth
  for cloud agent <bc-id> tool_call=<id>` from
  `cloudAgentRepository.mcpAuthResolution`, the user's answer to a cloud
  agent's request to authorise an MCP server, in the cloud agent's
  session and joined on the tool call. An MCP tool call in a local
  conversation is a `tool_use` row named `mcp-<server>-<tool>`; the
  editor records no per-call decision for it, so it has no `permission
  decision:` row ([#236]).
- The `cursor` parser yields `source host: github account <login>
  scopes=<scopes>` for the editor's own GitHub sign-in (the
  `github-<login>` and `github-<login>-usages` keys of `state.vscdb`), at
  the account's last use, and `remote agent host enrolled: <name>` when
  the editor has enrolled the machine as a remote agent host for the
  Cursor account, at the file's modification time. Neither reads the
  encrypted `secret://` values beside them ([#236]).

### Changed

- The `cursor` parser's `conversation deleted` row ends ` (isArchived is
  set by the delete)` when the kept header is marked archived, since the
  editor sets that flag when it deletes a conversation that was never
  archived ([#236]).
- The `cursor` parser's `remote agent host enrolled:` row now ends
  `file mtime; on by default, a paired device leaves no local record)`:
  the enrolment is on by default, and pairing a phone writes nothing
  on the computer ([#236]).
- The bundled catalog copy follows the collector on Windows: Cursor's
  sync store is read from `AppData/Local/Cursor/AgentStores`, where the
  editor writes it, instead of `AppData/Roaming/Cursor/AgentStores`
  ([#236]).
- The bundled catalog copy follows the collector: the Linux sync-store
  line `.local/state/cursor/agent-stores` replaces
  `.config/Cursor/AgentStores` ([#236]).
- The `cursor` parser's `permission setting:` rows from the editor's
  settings blob now end `(from state.vscdb composerState, file mtime,
  written late by the app)` and its `model last used:` rows `(from
  state.vscdb aiSettings.modelLastUsedAt, written late by the app)`: the
  editor writes that blob some time after the change, so neither time is
  the time of the change ([#237]).
- `inventory.json` lists an agent's own worktree inside a project
  (`<project>/.claude/worktrees/<name>`, and the same under `.kilo`,
  `.gemini`, `.qwen` and `.codex`) under `scratch_directories` instead of
  as a project: a path under a directory that one agent's project catalog
  entry names is a scratch directory, in loose mode too ([#227]).
- `sessions.jsonl` `relayed` now reads `yes` or `no` for `crush`,
  `hermes`, `letta`, `little-coder`, `nanobot` and `openclaw` sessions,
  `synced` for `little-coder` sessions and `origin` for `cline` and
  `agent-zero` sessions, instead of `""`; every little-coder session
  reads `no`, since its rows name no session ([#195], [#208], [#202],
  [#157], [#207], [#206], [#190], [#204]).
- `sessions.jsonl` `recovered` now reads `no`, `partial` or `yes`
  instead of `""` for the agents whose parsers read a secondary source
  since this release: `agent-zero`, `cody`, `continue`, `kilo-code`,
  `kiro`, `little-coder`, `openhands`, `pearai`, `pi`, `shellgpt`,
  `twinny` and `vscode` ([#204], [#197], [#198], [#192], [#212], [#157],
  [#205], [#193], [#201], [#189], [#199], [#210]).
- The `goose` parser writes tool confirmations as `permission request:`
  and `permission decision: <permission> (toolConfirmationResponse)`
  rows joined to the call, in place of the generic `action required:`
  row, and a `permission decision: not executable
  (goose.executable=false)` row for a call the permission check refused
  or held without an answer ([#209]).
- The `goose` parser reads the `llm_request` logs as a secondary source
  matched within 600 seconds, assembling streamed chunks first, so
  replies the database holds appear once and a removed or rewound
  session's replies are marked recovered ([#126]).
- The `letta` parser reads `sessions.jsonl` as exit lines only, and a
  transcript line takes its project from the agent's next exit line
  ([#158]).
- The bundled catalog copy follows the collector's tables: the `.crewai`
  project entry, the CLAI 2.0 state directory, Cody's JetBrains files,
  nanobot's workspace cron and trigger files, the OpenCode desktop app,
  the Kilo sandbox files, the Twinny workspace storage and the Agent Zero
  connector temp directory are attributed; the Gemini CLI and Qwen Code
  worktrees, the CLAI clones, OpenClaw's Gateway clones and OpenHands'
  `workspaces` are excluded; Hermes' `state-snapshots` is no longer
  excluded; and the secret globs change as in collector 1.18.0 ([#223],
  [#224], [#225], [#214], [#220], [#213], [#221], [#215], [#217], [#216],
  [#219], [#218], [#123], [#231], [#230], [#159]).

### Fixed

- The `cursor` parser no longer reads a permission prompt that was still
  open when the editor's database was copied as a decision: such a call
  (`reviewData.status` other than `Done`, or `toolFormerData.status`
  `loading`) yields `permission prompt pending: <tool> reason=<reason>
  options=<allowlist options>` instead of a `permission decision:` row
  carrying the prompt's preset option ([#237]).
- The `cursor` parser's `permission decision:` row ends `allowlisted=`
  for the editor window's "add ... to the allow list", which stores the
  decision as `run` with the added commands, as it does for the Agents
  window's Always run ([#237]).
- The `cursor` parser no longer reads the tool call that was running
  when the user stopped a turn as a pending prompt: such a call
  (`reviewData.status` `None`, `additionalData.status` `rejected`)
  yields `permission decision: stopped by the user (reviewData.status
  None, additionalData.status rejected)` beside its `[rejected] ` result,
  and the calls queued after it (`cancelled`) yield their `tool_use` row
  and no permission row ([#236]).
- An OpenClaw database that is only a PostgreSQL engine anchor yields a
  `parser:` row naming the store instead of nothing ([#206]).
- A Tabby server root collected from a Docker or Podman volume is parsed;
  before, it was detected but not parsed ([#124]).

## [0.20.0] - 2026-10-20

### Added

- A `cursor` parser for the Cursor IDE's agent conversations: the
  composers and bubbles of the editor's `User/globalStorage/state.vscdb`
  (`user`, `assistant`, `thinking` with `--include-thinking`, `tool_use`
  and `tool_result` rows, in both the 3.25.9 inline header list and the
  3.26.13 `composerConvHeaders` chunks), with `session name:`,
  `conversation archived` and `unsent draft:` rows; `model:` rows from
  each user message's model; a `permission decision:` row for each
  command the user was asked about, with the outcome, the reason and the
  commands an Always run added; `permission setting:` rows for the run
  mode of each agent mode, the command, domain and mode-transition lists
  and the auto-run switches, and for `.cursor/cli-config.json` and
  `.cursor/permissions.json`; and `model last used:` rows ([#51]).
- The `cursor` parser recovers a conversation deleted in the editor: its
  surviving `composerHeaders` row gives a `conversation deleted` row, the
  JSON `agentKv` prompt blobs that name it give `user` rows marked
  `recovered`, and the agent worker's transcript copy under
  `.cursor/projects/<project>/agent-transcripts/`, a secondary source
  read on every run, gives its prompts (at the minute the transcript
  records), replies and tool calls. `conversation-search.db` and
  `.cursor/ai-tracking/ai-code-tracking.db` give `conversation index:`,
  `ai code:`, `conversation summary:`, `file deleted by agent:` and
  `scored commit:` rows. Cursor sessions read `origin` `local`, or
  `remote` for a conversation whose header names a non-local agent
  location, and `recovered` `no`, `yes` or `partial` in `sessions.jsonl`
  and the inventory; `cursor` moves from `detected but not parsed` to
  `parsed` ([#51]).
- The bundled catalog copy follows collector 1.17.0: it gains Cursor's
  `AgentStores` and `update-supervisor` directories in the editor's
  user-data directory, so files there are detected as `cursor`, and its
  exclusion section gains Cursor's embedded Agent CLI, update supervisor
  binary, canvas `node_modules` and `statsig-cache.json`, which detection
  does not read ([#6]).

## [0.19.0] - 2026-10-19

### Changed

- The `continue` and `pearai` parsers now want the session index
  `sessions/sessions.json`, so `detect --files` lists it as parsed; it
  yields rows only when it is damaged ([#170]).
- The `vscode` session-start row ends with `workspace=<path>` when the
  workspace is a `.code-workspace` file, and a multi-root session with no
  `workingDirectory` of its own now has an empty `project_path`, where it
  had the `.code-workspace` file's path ([#185]).
- The `goose` parser reports a `content_json` that holds a bare JSON
  string as not a JSON array, since Goose writes an array there ([#183]).
- The `crush` `model:` rows end with ` requested=<model>` when the
  requested model differs from the one that served the turn, and the
  `model` column reads `<provider>/<served model>` ([#167]).
- The `opencode` parser now reads secondary sources, so an OpenCode
  session's `recovered` in `sessions.jsonl` and the inventory reads `no`,
  `yes` or `partial` where it was `""`; `kilo-code` keeps `""`, since
  Kilo Code has no such source ([#165]).
- The `hermes` parser may show one model switch twice: the `model: <old>
  -> <new>` row of the stored switch marker and the row of the session's
  model usage, at different times ([#180]).

### Fixed

- A JSONL line that is valid JSON but not an object (an array, a
  string, a number, `null`) is its own `system` row `parser: unparseable
  line (not a JSON object): <line>` in every parser that reads JSONL
  through the shared reader, instead of being skipped silently ([#228]).
- Ollama: a `.ollama/history` line that is not valid UTF-8 is no longer
  shown as a `user` row with replacement characters; it becomes its own
  `system` row `parser: unparseable line (<reason>): <line>` carrying the
  line as read, and the other lines are still yielded ([#174]).
- `trustedFolders.json` for Gemini CLI and Qwen Code is read as Qwen Code
  reads it: a byte-order mark and trailing commas no longer reduce the
  file to a single "is not a JSON object" row, so the standing trust
  grants appear in the timeline ([#173]).
- Twinny: a message the user sends while a reply runs (a `said` tool
  step, since Twinny 4.3.5) is now a `user` row; it was a `tool_use` row
  named `said` with an empty `tool_result` ([#171]).
- Cody: a Deep Cody confirmation the user refused now reads `permission
  decision: rejected confirmation state=error error=Command rejected
  ...`, and a pending one reads `approved or unanswered`, because Cody
  saves an approval and an unanswered request the same way; any other
  confirmation error is printed as `error=<message>` ([#169]).
- OpenClaw: a legacy `sessions.json` that cannot be read, does not decode
  or is not a JSON object now yields a `parser:` row in each transcript
  beside it instead of silently losing the session key, and a transcript
  or archive line that is valid JSON but not an object is its own
  `parser: unparseable line (not a JSON object): <line>` row instead of
  being dropped ([#177]).
- Qwen Code `cron` and `notification` records, and `agent_message` and
  `agent_mention` records written by another workspace agent, are
  `system` rows naming the subtype, provenance and agent (`cron
  (provenance=system): ...`, `agent_message (provenance=external_agent
  agent=<name> agentId=<id>): ...`) instead of `user` rows that read as
  prompts the user typed; the user's own @-mention stays a `user` row
  ([#172]).
- Crush: a `messages.parts` value that does not decode is carried in full
  in a `parser: unparseable record <rowid> (message <id> parts:
  <reason>): <parts>` row, where only the error was reported ([#166]).
- Crush: the model that served a Hyper Prism turn (`prism_model_id`) is
  named in the `model:` rows and the `model` column, with the requested
  model beside it as `requested=<model>`, where only the requested model
  was shown whenever both were set ([#167]).
- Codex CLI and Open Interpreter: each `thread_history_1.sqlite`
  `thread_items` row whose `item_json` does not decode to a JSON object
  is now its own `parser: unparseable record <rowid> (<reason>):
  <item_json>` row carrying the stored value, with its thread's session
  and project, instead of one row that counted them ([#184]).
- OpenHands: a `meta.json` or `base_state.json` that cannot be read, does
  not decode or is not a JSON object now gives a `parser:` row naming the
  file and the reason, instead of silently dropping the conversation's
  title, model and project ([#176]).
- Agent Zero chats collected from a Docker or Podman volume (`*a0_usr`,
  `a0-launcher-*-usr`), whose paths start at `chats/` under the volume's
  `_data`, are now parsed; they were collected and detected but yielded
  no rows ([#175]).
- Goose: a `sessions.db` `content_json`, `metadata_json` or
  `model_config_json` that does not parse, or is not the array or object
  Goose writes, is now its own `parser: unparseable record <N>
  (<table>.<column>: <reason>): <stored value>` row carrying the stored
  value in full, and the rest of the session is still read; before, the
  content row lost the stored text and bad metadata or model config was
  silently dropped ([#183]).
- VS Code: a chat's project path is now its own `workingDirectory`, then
  the workspace's folder, and never a `.code-workspace` file, so
  Agents-window chats are no longer classed under the editor's
  `agent-sessions.code-workspace` scratch file; the workspace file is
  named in the session-start row as `workspace=<path>` ([#185]).
- VS Code: a `workspace.json` that cannot be read or is not a JSON object
  now gives a `parser:` row instead of being skipped silently ([#185]).
- Zed: a `threads.db` or `db.sqlite` that is not SQLite is reported as
  one `parser: not a SQLite database` row, and a SQLite error as
  `parser: SQLite error: <error>`, instead of yielding nothing ([#168]).
- Zed: a thread blob that does not decode is now `parser: unparseable
  record <rowid> (<reason>): <stored value>`, carrying the `json` text or
  the `zstd` bytes as hex, instead of an unprefixed `thread <id> could
  not be decoded` row ([#168]).
- Zed: `messages[]` items, `Agent` content items, legacy segments and
  tool entries of an unknown variant each become a `parser: unparseable
  record <N> (<reason>): <item>` row instead of being skipped, and a
  `Compaction` of an unknown variant is no longer reported as provider
  native ([#168]).
- Hermes: a model switch made in the TUI or desktop app (a stored message
  with `display_kind` `model_switch`) is now a `system` row `model: <old>
  -> <new> provider=<provider>` at the switch time, instead of a user
  prompt, or a `[rewound]` user prompt once a later switch superseded it
  ([#180]).
- Hermes: a Hermes home collected from a Docker or Podman named volume,
  whose files start at the volume root (`state.db`, `sessions/`,
  `state-snapshots/`, `pastes/`), is now parsed; before, it was collected
  and never read ([#181]).
- Hermes: a `tool_calls` value that is not valid JSON, is not a list or
  holds a non-object entry is now a `parser: unparseable record <id>
  (<reason>): <value>` row carrying the stored value, instead of being
  dropped silently or reported without the value ([#182]).
- Continue: an unreadable or damaged `.continue/sessions/sessions.json`
  (not a JSON array, cut mid-write, an entry without a `sessionId`) now
  yields `parser:` rows from the index file instead of being dropped
  silently, and the entries before a cut still give their sessions a
  time ([#170]).
- PearAI: the same for `.pearai/sessions/sessions.json`, which is read
  through the Continue parser's index reader ([#170]).
- nanobot: sessions stored in `sessions/<workspace-id>/sessions.sqlite3`
  are read, each undecodable record its own `parser:` row, and the JSONL
  files and `jsonl-backup/` copies the database imported are secondary
  sources, so each event shows once and a turn the database lost is
  recovered ([#178]).
- nanobot: the CLI history `history/cli_history` is read under any
  `.nanobot*` state directory, where `--home` moves it, not only under
  `.nanobot` ([#179]).
- OpenCode 2.0 databases are read: session headers and project paths
  from `session_v2`, the `session_message` transcript including the 2.0
  shell, location-switched, skill and idle records; a migrated session
  is no longer shown twice, a session deleted in 2.0 is recovered from
  the 1.x tables left behind, and `opencode-next.db` is a secondary
  source ([#165]).
- OpenCode and Kilo Code: each undecodable message, part,
  `session_message` or event record and each undecodable part file is
  its own `parser:` row carrying the record in full, replacing the "N
  part(s) of message ... could not be decoded" and "N record(s) with
  undecodable data" count rows ([#139]).

## [0.18.0] - 2026-10-19

### Added

- `inventory.json` ends with a `scratch_directories` key, an object
  keyed and shaped like `projects`, that lists the scratch directories:
  the working directories that belong to the agent rather than to the
  user, such as the ChatGPT desktop app's projectless
  `~/Documents/Codex/<date>/<slug>`, a directory under the agent's own
  state directory (`.copilot/chats/<id>`) and the working directory of a
  Copilot Desktop session the app pre-created and never prompted. A path
  is one when a parser's `scratch directory:` row names it, or when it
  lies under a directory a home catalog entry names in one of the
  input's homes ([#149]).
- The `codex-cli` parser yields a `system` row `scratch directory:
  <path> (projectless thread <id>) (from
  .codex/.codex-global-state.json thread-projectless-output-directories,
  file mtime)` for each projectless output directory the ChatGPT desktop
  app records, with that path as `project_path` ([#149]).
- The `copilot-cli` parser yields a `system` row `scratch directory:
  <cwd> (working directory of a session the app pre-created and never
  prompted: no events.jsonl)` after the `session metadata:` row of a
  session directory that holds a `workspace.yaml` with a `cwd` and no
  `events.jsonl` ([#149]).
- The bundled catalog copy gains the collector's `codex-cli|Documents/Codex`
  line, so a home with the ChatGPT desktop app's projectless output
  directories is detected as `codex-cli` and the directories fall under
  `scratch_directories` by the catalog rule ([#149]).

### Changed

- `inventory.json` `projects` no longer lists the scratch directories,
  which move to `scratch_directories`; the `rows` and `sessions` of
  `projects`, `scratch_directories` and `outside_projects` add up to the
  totals. `timeline.jsonl` and `sessions.jsonl` are unchanged ([#149]).
- The `timeline` stdout line for the inventory reads `inventory:
  out/inventory.json (homes, projects, scratch directories, counts per
  agent, problems)` ([#149]).

## [0.17.0] - 2026-10-19

### Added

- A `chatgpt-desktop` parser for the ChatGPT desktop app's list of the
  account's ChatGPT conversations, the `chatgpt` rows of
  `.codex/sqlite/codex-dev.db` `local_thread_catalog`, which the app
  writes at sign-in. A conversation's text stays on OpenAI's servers,
  so each gives three `system` rows at its update time:
  `account conversation: <title> origin=<conversation_origin>
  created=<UTC> updated=<UTC> host=<host_id> (from
  .codex/sqlite/codex-dev.db local_thread_catalog)`, `session origin:
  remote chatgpt.com conversation <id> (listed at sign-in; its
  transcript is server-side)` and `session synced: chatgpt.com holds
  conversation <id>, no local transcript`, so its session reads
  `origin` `remote` and `synced` `yes` in `sessions.jsonl` ([#49]).

### Changed

- `chatgpt-desktop` is no longer reported as `collected only`: with a
  parser it is listed under `parsed`, or under `parser found no rows`
  when its files hold no account conversation ([#49]).

## [0.16.0] - 2026-10-19

### Added

- The `codex-cli` and `open-interpreter` parsers read the rollout's
  `thread_settings_applied` events, which the ChatGPT desktop app writes
  when the user changes the model, the reviewer or the access level, so
  `model:` and `permission mode:` rows are dated when the setting
  changed, not at the next turn ([#160]).
- A turn whose `task_started` record names its trigger gets a `turn
  trigger: <turn_trigger> turn=<id>` row: `composer`, `remote_ios` and
  `remote_android` (a prompt sent from a paired phone over remote
  control), `guardian_review`, `scheduled_task_setup`,
  `automation_heartbeat_run_now` and `automation_heartbeat_scheduled`
  ([#160]).
- The `codex-cli` parser yields the ChatGPT desktop app's per-call
  approvals: a `permission decision: <decision> exec <item id> ...` row
  per user decision recorded in `logs_<N>.sqlite`, joined to the call on
  `tool_use_id`, and a `permission decision: <outcome> (auto_review ...)`
  row per verdict of a guardian review thread (the automatic reviewer of
  the "Approve for me" mode), attributed to the user's thread ([#160],
  [#49]).
- The `codex-cli` and `open-interpreter` parsers write a `remote control:
  enabled=<0|1> server=<id> environment=<id> host=<name> ...` row per
  `state_5.sqlite` `remote_control_enrollments` row ([#160]).
- The `codex-cli` parser reads the desktop app's
  `.codex/.codex-global-state.json` as a secondary source: its prompt
  history as `user` rows (kept and marked `recovered` for a thread whose
  rollout is gone), and `session deleted:`, `thread description:`,
  `permission mode (standing):`, `permission setting:` and `remote
  control: mobile device` rows, so a thread deleted in the app still
  shows its prompts and its deletion ([#160], [#49]).

### Changed

- The `permission mode:` row of the `codex-cli` and `open-interpreter`
  parsers ends with two more fields, `reviewer=<approvals_reviewer>
  profile_id=<active_permission_profile.id>` (`-` when the rollout
  records neither), a change to the row text of `timeline.jsonl`
  ([#160]).

## [0.15.0] - 2026-10-18

### Added

- The `openclaw` parser records sync state: a session published through
  the Gateway's public link gets a `session synced: public share <id>
  generation <sessionId>` row from `session_nodes.entry_json`, so
  `sessions.jsonl` says `synced` `yes` or `no` for OpenClaw sessions, and
  it reads `state/openclaw.sqlite` for one `backup uploaded:` row per
  successful `openclaw backup` upload or Git backup ([#153]).
- The `agent-zero` parser records sync state: a `session synced:
  provider response` row for each agent that chained a turn on a
  response the provider stored, and a `session relayed:` row for a chat
  carried by the Telegram, WhatsApp or email plugin ([#153]).
- The `hermes` parser reads `pastes/pending.json`: one `debug report
  uploaded: <url>` row per paste.rs link of a recent `hermes debug share`
  upload ([#153]).
- The `goose` parser writes a `session relayed: gateway <platform> user
  <user_id>` row for a session the gateway relayed through its Telegram
  bot ([#153]).
- The `codex-cli` and `open-interpreter` parsers read `logs_<N>.sqlite`,
  the tracing log under the agent's home: each `/feedback` report sent to
  Sentry is a `session synced: feedback report uploaded to Sentry, ...`
  row with the reported thread as its session, and both parsers now set
  `synced` in `sessions.jsonl` (`no` for a thread without the row)
  ([#153]).
- The `cline` parser yields a `session synced: enterprise prompt upload
  to ...` row for an SDK session whose manifest or `sessions.db`
  metadata has `enterprise.blobUpload`, naming the organisation's bucket
  and object, and sets `synced` in `sessions.jsonl` ([#153]).
- The `cline` parser reads the VS Code extension's upload queue
  `cache/sync-queue.json`: each queued task file is an `enterprise
  prompt upload <status> ...` row for its task, prefixed `session
  synced: ` when the file was uploaded, without the file content the
  queue holds ([#153]).
- The `continue` parser reads `.continue/logs/cn.log` and its rotated
  `cn<N>.log` files: each `cn serve` metadata post to Continue's agent
  API is an `agent metadata posted:` row with its result, and a skipped
  snapshot upload a `storage upload requested, skipped` row ([#153]).
- The `qwen-code` parser reads the channel routes files
  `.qwen/channels/sessions.json` and
  `.qwen/channels/daemon/<hash>/routes.json`: each session driven from a
  messaging platform is a `session relayed: channel ...` row ([#153]).
- The `opencode` and `kilo-code` parsers write a `session synced: share`
  row for each session published to OpenCode's share service, from the
  database's `session_share` table, and, for a session shared and then
  unshared, `session synced: share url` and `session unshared` rows from
  its `session.updated.1` events; `sessions.jsonl` `synced` is now `yes`
  or `no` for their sessions ([#153]).
- The `kilo-code` parser reads Kilo's cloud session marker,
  `.local/share/kilo/storage/session_share/<sessionID>.json`, as a
  `session synced: kilo session` row ([#153]).
- The `pi` parser writes a `session synced: bug report` row for a `/bug`
  report uploaded to radius.pi.dev and a `bug report exported:` row for
  one written to a zip, from the session's `pi.bug-report` entry;
  `sessions.jsonl` `synced` is now `yes` or `no` for pi sessions ([#153]).
- The `letta` parser writes a `session synced: server conversation` row
  for each transcript of a server-side agent and reads
  `.letta/settings.json` and a project's `.letta/settings.local.json` for
  a `session synced: last session on` row per server; `sessions.jsonl`
  `synced` is now `yes` or `no` for Letta sessions ([#153]).
- The `vscode` parser reads Copilot Chat's
  `User/globalStorage/github.copilot-chat/cloudSessions.json`, a
  `session synced: remote session` row per chat session uploaded to the
  user's GitHub account, and the cloud agent tasks in the extension's
  `state.vscdb` global state, a `session synced: cloud agent task <id>`
  row each, since GitHub holds those sessions too, and a `session
  origin: remote GitHub cloud agent` row beside it; `sessions.jsonl`
  `synced` is now `yes` or `no` for VS Code sessions, `yes` for each
  cloud agent task ([#153]).
- `sessions.jsonl` gains a last field `relayed`, after `synced`, a new
  field of that interface: `yes` when the session has a `session
  relayed:` row, `no` for a session of an agent whose parser records
  relay state (`agent-zero`, `goose`, `qwen-code`) without one, `""` for
  every other agent. `inventory.json` gains two `relayed_sessions` keys,
  new keys of that interface: the last top-level key, after
  `synced_sessions`, counting the sessions whose `relayed` is `yes`, and
  the last key of each `homes[].agents[]` object, after
  `synced_sessions`, counting them per home and agent ([#153]).
- `sessions.jsonl` gains a last field `origin`, after `relayed`, a new
  field of that interface: `remote` when the session has a `session
  origin: remote` row, `local` for a session of an agent whose parser
  records origin (`vscode`) without one, `""` for every other agent.
  `inventory.json` gains two `remote_sessions` keys, new keys of that
  interface: the last top-level key, after `relayed_sessions`, counting
  the sessions whose `origin` is `remote`, and the last key of each
  `homes[].agents[]` object, after `relayed_sessions`, counting them per
  home and agent ([#153]).
- `timeline.jsonl` gains a last field `recovered`, after `source_line`,
  a new field of that interface: `yes` on an event row (any `turn_type`
  but `system`) of a secondary source that survived deduplication, so
  the primary store does not hold the event, `""` on every other row.
  `sessions.jsonl` gains a last field `recovered`, after `origin`, a new
  field of that interface, decided by the session's event rows: `yes`
  when every one is recovered (the primary transcript is gone),
  `partial` when some are, `no` when none is and the agent's parser
  reads a secondary source, `""` for the others.
  `inventory.json` gains `recovered_sessions`, new keys of that
  interface, counting the `yes` and `partial` sessions: the last
  top-level key, after `remote_sessions`; the last key of each
  `homes[].agents[]` object, after `remote_sessions`. Each
  `projects.<path>` entry and `outside_projects` end with `synced` and
  `relayed` (`yes`, `partial`, `no` or `""`), `origin` (`remote`,
  `mixed`, `local` or `""`) and `recovered` (`yes`, `partial`, `no` or
  `""`), summarising their sessions: new keys of that interface, in last
  position.

### Changed

- `sessions.jsonl` `synced` now reads `yes` or `no`, where it read `""`,
  for the sessions of `agent-zero`, `cline`, `codex-cli`, `kilo-code`,
  `letta`, `open-interpreter`, `openclaw`, `opencode`, `pi` and `vscode`,
  whose parsers now record sync state, and `inventory.json`
  `synced_sessions` and the `sessions:` line count their synced sessions.
  The `sessions:` line also counts the relayed sessions, the sessions
  of remote origin and the recovered sessions, ending `(<N> synced, <M>
  relayed, <K> remote, <R> recovered)`, each part shown only when its
  count is greater than 0.
  This changes the values of a `sessions.jsonl` field ([#153]).
- `timeline.jsonl` gains `system` row forms: `session synced:` from the
  parsers above, `session relayed:` (`agent-zero`, `goose`, `qwen-code`),
  `session origin: remote` (`vscode`),
  `backup uploaded:` (`openclaw`), `debug report uploaded:` (`hermes`),
  `agent metadata posted:` and `storage upload requested, skipped`
  (`continue`), `bug report exported:` (`pi`), `session unshared` (`opencode`, `kilo-code`) and `cline`'s
  `enterprise prompt upload <status>` for a queued upload that has not
  gone yet ([#153]).
- The bundled catalog copy follows collector 1.14.0: its credential
  section gains the secret globs of the session sync round and replaces
  Qwen Code's `.qwen/channels/*.json`, and its exclusion section drops
  Cline's VS Code `cache` directory and Kilo Code's `log` directory.
  Detection reads neither section, so it is unchanged ([#153]).

## [0.14.0] - 2026-10-17

### Added

- `inventory.json` begins with a `doubleagent_version` key, the analyzer
  version that wrote it ([#138]).
- `inventory.json` ends with a `collected_only` key and the `agents:`
  line gains a `collected only:` group, naming the detected agents whose
  files hold nothing to parse (`camel-ai`, `chatgpt-desktop`, `copilot`,
  `metagpt`, `shared`). This adds a key to the `inventory.json` interface
  ([#143]).
- A `copilot-cli` parser for GitHub Copilot CLI. It reads each session's
  event log `.copilot/session-state/<id>/events.jsonl`: prompts, replies,
  tool calls and results joined on `toolCallId`, the system prompt, the
  runtime's utility-model calls, `model:` rows from `session.model_change`,
  and `permission request:`, `permission decision:` and `permission mode:`
  rows beside the call they govern ([#50]).
- The parser reads `.copilot/session-state/<id>/workspace.yaml`: a
  `session name:` row and a `session metadata:` row (user-named, forks,
  compactions) at the session's creation time; its `cwd` is the project
  path of rows that record none ([#50]).
- The parser reads `.copilot/session-store.db` (SQLite with WAL sidecars)
  as a secondary source with a one-hour window, since a turn is stamped
  when it ends: the prompt and final reply of each turn, the session name,
  and `file changed:`, `reference:` and `checkpoint` rows, so turns a
  `/rewind` removed from the log, or a deleted session's, are kept ([#50]).
- The parser reads each session's
  `rewind-file-snapshots/index.json` as a secondary source: the prompt of
  each snapshot and a `rewind snapshot:` row naming the files it saved
  ([#50]).
- The parser reads the prompt history `.copilot/command-history-state.json`
  as a secondary source: one `user` row per prompt with no timestamp,
  session or project, matched on text alone ([#50]).
- The parser reads the standing approvals in
  `.copilot/permissions-config.json` (`tool_approvals` and
  `allowed_directories` per location) and the `allowedUrls` and
  `deniedUrls` of `.copilot/settings.json`: one `permission setting:` row
  per entry at the file's modification time ([#50]).
- The parser reads the sign-in line of `.copilot/logs/process-*.log` as a
  `signed in: <host> as <login>` row, and takes the project path of
  database and snapshot rows that record none from
  `.copilot/sidebar-sessions-state/*.json` ([#50]).
- The parser yields a `system` row `session.info <infoType>: <message>`
  (with ` url=` and ` tip=` when present) for each `session.info` notice
  in the event log, so the CLI's own report of a `/model` switch ("Model
  changed from ... to ...") and its Auto routing line keep their rows
  beside the `model:` row ([#133]).
- The bundled catalog copy follows collector 1.13.0: GitHub Copilot
  Desktop is detected as `copilot-desktop` from its own files inside
  `.copilot` (`data.db`, `repo-metadata-cache.db`, `chats`, `repos`,
  `run`, `media-cache`, `canvas-catalog-probe`,
  `logs/github-app.*.log`), its data directory `com.github.githubapp`
  and Foundry Local's `.github-copilot-cli`; its sessions under
  `.copilot/session-state` stay `copilot-cli` ([#147]).
- A `copilot-desktop` parser for the GitHub Copilot Desktop app. It reads
  the app's store `.copilot/data.db` (SQLite with WAL sidecars), one
  `system` row per record: the signed-in account (`account:`, with
  whether a token is stored, never the token), the app settings at the
  file's modification time, the model provider, each repository added
  (`project added:`), worktree and workspace, each session (`desktop
  session:` with its type, model, `auto_approve` and `permission_mode`,
  and `forked from session` when forked), activity items, checkpoint
  commits, scheduled tasks with their triggers and runs, session
  automations and session deletion requests. The app's session logs stay
  with the `copilot-cli` parser ([#147]).
- The `copilot-desktop` parser also reads the checkpoint refs the app
  writes into its clone of a project,
  `.copilot/repos/<repo>/.git/refs/copilot/checkpoints/<session>/<seq>/<event>`:
  one `system` row per ref, `checkpoint ref <seq>: commit <hash> event
  <event id> (file mtime)`, at the file's modification time, with the
  session id from the path. The refs outlive a session the app deleted,
  and their commits are snapshots of the session's worktree ([#147]).
- The `copilot-cli` parser yields `hook <hookType> start: <input>` and
  `hook <hookType> end: success=<b> output=<output>` (with ` error=` when
  present) rows for the `hook.start` and `hook.end` events the desktop
  app's sessions record, joined to the tool call on `parentToolCallId`;
  they were `hook.start: <compact JSON>` rows before ([#147]).
- `sessions.jsonl` gains a last field `synced`, a new field of the
  `sessions.jsonl` interface: `yes` when the session carries a
  `session synced:` row, `no` when the agent's parser records sync state
  (today `copilot-cli`, which also covers Copilot Desktop's sessions) and
  the session has none, `""` when the parser records no sync state
  ([#151]).
- `inventory.json` gains two `synced_sessions` keys, new keys of that
  interface: the last top-level key, the number of sessions whose
  `synced` is `yes`, and the last key of each `homes[].agents[]` object,
  the same count for that user and agent. The `sessions:` line ends with
  `(<N> synced)` when N is greater than 0 ([#151]).
- The `copilot-cli` parser yields a `session synced: remote session <id>
  task <id> last event <id> remote_steerable=<value> (from workspace.yaml,
  file mtime)` row when a session's `workspace.yaml` has a non-empty
  `mc_session_id`, the marker of a session synced to the GitHub account,
  at the file's modification time ([#151]).

### Changed

- The `copilot-cli` parser's `session metadata:` row ends with
  ` client=<client_name>` from `workspace.yaml`, `github/cli` for a CLI
  session and `github/autopilot` for one the desktop app ran ([#147]).
- The `copilot-cli` parser writes the desktop app's
  `session.permissions_changed` form, which carries
  `allowAllPermissionMode` and no `mode`, as `permission mode: allow-all
  <old> -> <new>` instead of `permission mode: - -> -` ([#147]).

- The package metadata (`pyproject.toml` `[project.urls]`) links the
  documentation site, the repository, the issue tracker and this changelog.
- Each line a parser cannot decode is now its own `system` row carrying
  the line in full, `parser: unparseable line (<reason>): <line>`, in
  place of the one row per file `parser: N unparseable line(s), first at
  line M`; a JSON array record that does not decode is likewise
  `parser: unparseable record N (<reason>): <text>`, with the record's
  text to the end of the file, and a whole-file error is one row
  `parser: <message>`. This changes the timeline row text interface
  ([#136]).

### Fixed

- `camel-ai`, `chatgpt-desktop`, `copilot`, `metagpt` and `shared`, whose
  files are a token store, configuration or app state with no transcript,
  are no longer listed as a parser backlog under `detected but not parsed`
  and in `detected_not_parsed`, which changes the meaning of that
  `inventory.json` key; they are listed as `collected only` ([#143]).
- The Claude Code and Antigravity `history.jsonl` readers report
  unparseable lines as `system` rows instead of dropping them silently;
  the valid prompts before and after are still yielded ([#134]).
- A `parser:` row from a secondary source (a history file that could not
  be read, or held an unparseable line) is no longer dropped as a
  duplicate of the same row from the primary store: each describes its
  own file, so both are kept ([#135]).
- The Codex and Open Interpreter `history.jsonl` readers report
  unparseable lines as `system` rows instead of dropping them silently;
  the valid prompts before and after are still yielded ([#125]).

## [0.13.0] - 2026-10-15

### Added

- A `dropped:` line on the `timeline` stdout, always printed before
  `agents:`, counting the rows of secondary sources dropped because a
  primary store, or another secondary source, records the same event,
  with the agents that had any ([#122]).
- `inventory.json` key `deduplicated`, the new last key: the dropped
  rows per agent, `{}` when none ([#122]).
- The `hermes` parser reads the quick snapshots
  `state-snapshots/<id>/state.db` (collected with `--full`) like
  `state.db`, after one `system` row naming the snapshot, as a secondary
  source, so a session pruned or deleted from the live database is
  recovered once however many snapshots hold it ([#122]).
- Cline's `~/.cline/data/db/session-search.db`, the SDK's full-text index
  of each session, is read as a secondary source: one `system` row per
  indexed session at its start, then a `user` or `assistant` row per
  message with no timestamp, so a session whose transcript is gone still
  shows its turns ([#122]).
- Cline's CLI prompt history `~/.cline/data/cache/user_input_history.jsonl`
  is read as a secondary source: one `user` row per prompt, the last one
  at its `ts` and the others with no timestamp, since the CLI restamps
  every line each time it rewrites the file ([#122]).
- OpenClaw: compaction checkpoints (`*.checkpoint.<uuid>.jsonl`) and
  pre-doctor repair backups (`*.jsonl.pre-doctor-*-repair-*.bak`) in the
  sessions directory are parsed as secondary sources; a backup keeps the
  abandoned branches a doctor repair removed ([#122]).
- The `nanobot` parser reads the WebUI display transcript
  (`.nanobot*/webui/<key>.jsonl` and its `.segments/NNNNNN.jsonl`) as a
  secondary source: `user`, `assistant`, `tool_use`, `tool_result`,
  opt-in `thinking` and `system` rows (file edits, fork markers, context
  compactions) at the record's `created_at_ms`, which is true UTC unlike
  the session files' local time, so they pair with session rows within
  15 hours; a WebUI turn whose session file a `/new` emptied is now in
  the timeline ([#122]).
- The `nanobot` parser reads `~/.nanobot/history/cli_history`, the CLI's
  prompt history, as a secondary source: one `user` row per prompt at
  its own local time, with no session or project ([#122]).
- The `codex-cli` and `open-interpreter` parsers read `state_5.sqlite`
  (a `thread record:` system row per thread and its first prompt, with
  no timestamp) and `thread_history_1.sqlite` (the turns and turn items
  Codex projects from a rollout) as secondary sources, so a thread whose
  rollout was removed outside Codex, excluded by size or truncated is
  still in the timeline ([#122]).
- Roo Code: the CLI's input history `~/.roo/cli-history.json` is read,
  one `user` row per entry with no timestamp, task or project, as a
  secondary source that matches the task rows on text alone ([#122]).
- The `antigravity` parser reads the brain mirror of each conversation,
  `brain/<uuid>/.system_generated/logs/transcript_full.jsonl` (else its
  chunks, else the shortened `transcript.jsonl`, else its chunks) under
  every data directory, as a secondary source paired with the
  conversation database within 600 seconds, so a conversation whose
  database is gone or unreadable is still in the timeline ([#122]).

### Changed

- Interface change: the `timeline.jsonl` and `sessions.jsonl`
  `project_path` of a Windows path (a drive letter or a `\\` network
  share) is written in one canonical form, whichever agent and store
  recorded it: upper-case drive letter, backslash separators, no repeated
  or trailing separator (except in a drive root such as `C:\`), and the
  extended-length prefix `\\?\` removed. Antigravity's
  `c:/Users/sean/demo` (from a `file:` URI) and `C:/users/sean/demo2`
  are now `C:\Users\sean\demo` and `C:\users\sean\demo2`. The case
  after the drive is kept as recorded; POSIX paths are unchanged
  ([#112]).
- Interface change: `inventory.json` `projects` groups Windows paths that
  differ only in case into one project, keyed by the spelling of the
  collected project directory when it is one, so its `status` is
  `collected` and its rows, sessions and files are under one key; a
  project the input does not hold is keyed by the first spelling the rows
  record. Project directories from `collection.json` are in the same
  canonical form ([#112]).
- Interface change: `timeline.jsonl` no longer holds a row of a second
  copy that repeats a transcript row or another copy. The secondary
  sources are Aider's `.aider.input.history` and `.aider.llm.history`;
  Antigravity's CLI `history.jsonl` and brain mirror; Claude Code's
  `history.jsonl`; Claude Desktop's Cowork `audit.jsonl`; Cline's
  `session-search.db` and CLI `user_input_history.jsonl`; Codex CLI's
  and Open Interpreter's `history.jsonl`, `state_5.sqlite` and
  `thread_history_1.sqlite`; Gemini CLI's and Qwen Code's `logs.json`;
  Goose's `history.txt`; Hermes's `state-snapshots/<id>/state.db` and
  `sessions/<id>.jsonl` diverts; Letta's `transcript.jsonl`; Muse Code's
  `tui-history.jsonl`; nanobot's WebUI transcript and `cli_history`;
  OpenClaw's reset and deleted archive files, compaction checkpoints and
  pre-doctor repair backups; Roo Code's `cli-history.json`; and Tabby's
  database backups and `events/` log. One of their rows is
  dropped when a primary-store row of the same host, user and agent has
  the same `turn_type` and `text` (two `tool_use` or two `tool_result`
  rows that both carry a `tool_use_id`: the same `tool_use_id` instead),
  the same `session_id` (or, without one, the same `project_path`, or,
  without either, any session or project) and, when the secondary row
  has a `timestamp_utc`, one at most 2 seconds away (a wider window
  where the parser table says so); a row with no timestamp, such as Muse
  Code's, matches on text alone, the session-scoped ones first, so the
  most specific copy takes the primary row. Secondary rows that no
  primary row took are then matched against the copies a primary row
  took and against each other the same way, the timestamped rows first
  in time order and the undated ones after, and looking under a broader
  scope too (the project, then any) when a candidate's session and
  project do not differ from the row's; so three copies of a primary
  event leave only the primary row, copies of one event kept by several
  backups appear once, as the earliest timestamped copy, and of an
  undated and a timestamped copy the timestamped one is kept. A row pairs only with one from a
  different file, so two identical lines in one file both stay.
  Secondary rows with no match are kept, so a prompt whose transcript is
  gone is still in the timeline ([#122]).
- Goose's CLI `history.txt` (both locations) is a secondary source: a
  history row is dropped when a `sessions.db` user row has the same text,
  and kept when none does, such as a prompt of a removed session ([#122]).
- Aider's `.aider.input.history` and `.aider.llm.history` are secondary
  sources of `.aider.chat.history.md`: an input-history prompt the chat
  history records is dropped, since the chat row takes its time from it,
  and one the chat history lacks is kept ([#122]).
- Claude Desktop: each Cowork session's `audit.jsonl` is read in full on
  every run, its prompts, replies and tool calls too, not only when the
  session dir holds no transcript, and is a secondary source of that
  transcript; the audit rows without a session record take the `init`
  line's `cwd` as `project_path`, as the transcript's rows take the
  JSONL `cwd`. The Cowork session record `local_<uuid>.json` is a
  secondary source too, and its `initialMessage` is now always a `user`
  row, with no timestamp, instead of only when neither a transcript nor
  an audit log was collected; a Code-tab record's stays a fallback,
  since its transcript belongs to agent `claude-code` ([#122]).
- The `letta` parser reads `.letta/transcripts/<agent>/<conversation>/transcript.jsonl`
  always, also when the conversation's local-backend `messages.jsonl` was
  collected (it gave one `system` row pointing at it before), as a
  secondary source paired with `messages.jsonl` within one hour, since
  Letta stamps each line at the end of its turn; its `tool_use` text is
  now the command, path or query, as the `messages.jsonl` row shows it,
  instead of the arguments JSON ([#122]).
- The `hermes` parser's `sessions/<id>.jsonl` diverts are a secondary
  source: a message the live `state.db` also holds for that session is
  shown once ([#122]).
- Tabby's pre-migration backups `db.backup-*.sqlite` and the event log
  `events/YYYY-MM-DD.json` are secondary sources: a backup's thread and
  event rows that the live database (or an earlier backup) also holds,
  and an event log prompt or choice that `user_events` also holds, are
  dropped. The backup's leading `system` row now names the backup file
  ([#122]).
- OpenClaw: a published reset or deleted archive blob in
  `session_transcript_archives` is parsed in full instead of one `system`
  row naming its file, and the `.jsonl.reset.*` / `.jsonl.deleted.*` file
  is a secondary source; the session id of a legacy JSONL file is now its
  header `id` ([#122]).

## [0.12.0] - 2026-10-15

### Added

- `timeline` writes `inventory.json` beside `timeline.jsonl` and
  `sessions.jsonl`: the input's `kind`, `notes` and `problems` (the
  problems met while opening the input, then each file a parser could
  not read), the files, bytes and parser of each agent in each user home
  and Docker volume (`homes`, the same array `detect --json` prints), row
  and session counts per project (`projects`) and outside any project
  (`outside_projects`), each split by agent and `turn_type`, and the
  lists of agents parsed, detected but not parsed, and whose parser found
  no rows. The row and session counts follow the filters, as the stdout
  breakdown does; `homes`, `notes`, `problems` and the file counts do not
  ([#103], [#104]).
- Each `inventory.json` project starts with `status`: `collected` when
  the path is a project directory the collection holds
  (`collection.json` `projects`), `referenced` when only timeline rows
  name it. A collected project that no row names is listed with zero
  counts ([#104]).
- Each `inventory.json` project counts the files the collection holds in
  its directory: every agent gains `files` and `bytes`, an agent with
  project files and no rows (a `.cursor/` directory, an agent with no
  parser) is listed with zero rows, so the keys of `agents` are every
  agent used on the project, and `shared_files` counts the files no
  single agent owns (`AGENTS.md`, `.mcp.json`). These counts do not
  follow the filters ([#104]).
- `detect` prints project directories in a table of their own after the
  table of homes: one line per agent of each project with its files,
  bytes and parser, and a `(shared)` line for files such as `AGENTS.md`.
  `detect --json` holds them as `projects` (`path`, `users`, `agents`
  with `agent`, `files`, `bytes`, `parser`, `shared_files`,
  `shared_bytes`). `project` never appears as an agent.
  Collections from collectors before and after 1.12.0 give the same
  lists, and loose input lists no project ([#104]).
- An agent whose only files are in a project directory counts as
  detected on the timeline's `agents:` line and in `inventory.json`
  ([#104]).
- `timeline` prints a line for `inventory.json` after the `sessions:`
  line: `inventory: <path> (homes, projects, counts per agent,
  problems)`. The `rows:` and `sessions:` lines are unchanged ([#103],
  [#104]).
- `docs/output.md` documents every key of `detect --json` and
  `inventory.json` and every field of `sessions.jsonl` in a table with
  its type, and the `timeline.jsonl` table gains a type column ([#103],
  [#104]).
- The bundled catalog copy detects the Antigravity CLI's file secret
  store `plugin-secrets.json` (`.config/antigravity`,
  `Library/Application Support/antigravity`, `AppData/Roaming/antigravity`)
  as `antigravity` ([#90]).
- The bundled catalog copy follows collector 1.12.0: the Antigravity
  desktop app's Electron user-data directory
  (`Library/Application Support/Antigravity`, `.config/Antigravity`,
  `AppData/Roaming/Antigravity`) and its macOS logs directory
  `Library/Logs/Antigravity` are detected as `antigravity` ([#83]).
- The bundled catalog copy detects `AppData/Local/antigravity`, the
  Windows staging directory of the Antigravity desktop app and CLI, as
  `antigravity`, following collector 1.12.0 ([#83]).
- The `antigravity` parser reads the Antigravity desktop app's
  conversations: `conversations/*.db` and `conversation_summaries.db`
  under `.gemini/antigravity` (the desktop app 2.x), and under
  `.gemini/antigravity-ide` and `.gemini/antigravity-backup` (the
  separately installed IDE and the app's migration backup) as well as
  `.gemini/antigravity-cli`. Before, only the CLI's conversations produced
  rows and the app's were collected but silently skipped. Each
  conversation's session start row now names the surface that wrote it,
  `source=<trajectory source>` (`cli` for the CLI, `cascade_client` for the
  desktop app, the number for a value not in the table) and
  `data_dir=<the .gemini subdirectory>` ([#83]).
- Rows from the Antigravity CLI's `history.jsonl` carry the
  conversation's id in `session_id` when the line has `conversationId`,
  which CLI 1.3.x writes; older lines have none and keep `session_id`
  empty. A prompt in the history can now be joined to its conversation
  database ([#89]).
- The `antigravity` parser turns Antigravity's permission state into
  `system` rows. Each grant in a step's `steps.permissions` blob is a
  `permission grant: action=<action> target=<target> allow=<true|false>
  scope=<scope>` row, and each file access entry a `permission grant:
  path=<path> is_directory=<true|false> allow=<true|false> scope=<scope>`
  row, after the step's rows and with its `tool_name` and `tool_use_id`.
  The standing grants in `.gemini/config/config.json`,
  `.gemini/config/projects/*.json`, `.gemini/config/sidecars/*/sidecar.json`
  and the CLI's `.gemini/antigravity-cli/settings.json` (allow lists,
  sidecar access grants, the migration and sandbox flags, trusted
  workspaces) are one `permission setting: <key>=<value> (from <file>,
  file mtime)` row per entry, timestamped by the file's modification time
  ([#92]).
- The `claude-code` parser records the permission mode and the model as
  events: `permission mode: <mode>` at a session's first
  `permission-mode` record or `permissionMode` prompt key and
  `permission mode: <old> -> <new>` on each change, and `model: <id>` at
  the first assistant reply and `model: <old> -> <new>` on each change,
  with ` requested=<requestedModel>` when the requested model differs.
  The `claude-desktop` transcripts, read with the same reader, gain the
  same rows ([#93]).
- The `claude-code` parser reads the standing permission settings: one
  `system` row per `permissions.allow`, `deny`, `ask` and
  `additionalDirectories` entry and one for `defaultMode` and
  `disableBypassPermissionsMode` in `.claude/settings.json` and
  `.claude/settings.local.json` (the user's and, from a collection, a
  discovered project's), and one per `allowedTools` entry and for
  `hasTrustDialogAccepted` and `hasClaudeMdExternalIncludesApproved` of
  each project in `.claude.json`, timestamped by the file's modification
  time and saying so. These files were not read before; no other key in
  them is read ([#93]).
- The `claude-desktop` session-record row ends with
  `permission_mode=<mode> effort=<effort>`, and each per-session grant in
  the record (`alwaysAllowedReasons` and `sessionPermissionUpdates`
  entries, `chromePermissionMode`, `chromeAllowedDomains`,
  `cuAllowedApps`, `cuGrantFlags`, `cuFlagsGrantedAt`) is a
  `permission grant:` row at the record's `lastActivityAt` ([#93]).
- The `codex-cli` and `open-interpreter` parsers record the model and
  the permission policy of each turn as events: `model: <id>` at the
  first `turn_context` that names a model and `model: <old> -> <new>` on
  each change, and `permission mode: approval=<approval_policy>
  sandbox=<sandbox_policy> profile=<permission_profile> network=<network>`
  at the first `turn_context` with a policy and `permission mode: <old> ->
  <new>` on each change (`-` for a missing field). Codex writes no
  per-call approval to its rollouts, so there is no decision row
  ([#96], [#97]).
- The `gemini-cli` and `qwen-code` parsers read `trustedFolders.json`
  (`.gemini/trustedFolders.json`, `.qwen/trustedFolders.json`): one
  `permission setting: trustedFolders.<path>=<TrustLevel> (from <file>,
  file mtime)` row per folder, with `project_path` the folder and the
  file's modification time. A tool call whose status is `cancelled` or
  `awaiting_approval` is followed by a `permission decision: <status>`
  row with its `tool_use_id`; the `[cancelled]` prefix on the result row
  stays. `gemini-cli` writes `model: <id>` at a session's first gemini
  message that names a model and `model: <old> -> <new>` on each change,
  and `qwen-code` writes `model: <id>` from the first assistant record
  when no `session_model` record named one, and a change row when an
  assistant record's model differs ([#96], [#97]).
- The `antigravity` parser writes `model: <id>` before the rows of a
  conversation's first step that `gen_metadata` names a model for and
  `model: <old> -> <new>` before those of every later step on another
  model, and a `permission decision: user_rejected` row after the result
  of a `run_command` or `mcp_tool` call the user rejected, with the call's
  `tool_use_id`; a `run_command` result that ran in the sandbox ends with
  `sandbox=true` ([#96], [#97]).
- The `opencode` and `kilo-code` parsers record the model and the agent
  as events: `model: <modelID> provider=<providerID>` before the first
  message of a session that names a model and `model: <old> -> <new>
  provider=<providerID>` before each message that changes it, and
  `permission mode: agent=<name>` and `permission mode: agent=<old> ->
  agent=<new>` for the message's `agent`, since an OpenCode agent carries
  its own permission ruleset. In a session that only the V2
  `session_message` table holds, the `model-switched` and
  `agent-switched` records, which were printed as raw JSON before, become
  the change rows ([#96], [#97]).
- The `crush` parser records the model as events: `model: <model>
  provider=<provider>` before the first message of a session that names
  one and `model: <old> -> <new> provider=<provider>` before each message
  that changes it ([#96]).
- The `pi` parser reads `.pi/agent/trust.json`, the project trust
  decisions in the agent directory pi and little-coder share: one
  `permission setting: <path>=<true|false> (from .pi/agent/trust.json,
  file mtime)` row per project, with `project_path` the project and
  `timestamp_utc` the file's modification time, not the time of the
  decision. The file was not read before ([#97]).
- The `letta` parser adds a `model: <id> provider=<provider>` row after
  the first `.letta/sessions.jsonl` line of each CLI session that names a
  model, and `model: <old> -> <new> provider=<provider>` when a later
  line of the session names another ([#96]).
- The `continue` and `pearai` chat-session parsers yield a `system` row
  `model: <id>` at a session's first model and `model: <old> -> <new>`
  ahead of a later history item that names a different one: Continue
  starts from `chatModelTitle` (else the first `promptLogs[].modelTitle`)
  and compares each item's `promptLogs[].modelTitle`, PearAI compares each
  item's `promptLogs[].completionOptions.model`. The `continue` parser
  also yields `permission decision: accepted=<value>` after each
  `toolUsage.jsonl` record's `tool_result` row and
  `permission decision: canceled` for a session tool call whose state is
  `canceled` ([#96], [#97]).
- The `vscode` parser writes `model: <id>` and `model: <old> -> <new>`
  rows from the request `modelId`, and `permission mode: <mode>` and
  `permission mode: <old> -> <new>` rows from the request
  `modeInfo.telemetryModeId` (`ask`, `agent`, `edit`, `custom`). A tool
  invocation's `isConfirmed` becomes a `permission decision:
  confirmed=<value>` row after its result (`true` or `false` before VS
  Code 1.104, else the confirmation kind such as `UserAction(4)` or
  `Denied(0)` with its `source`, `scope`, `id`, `selectedButton` or
  `reason`, and `tool_source=<type>`), a `confirmation` response part a
  `permission decision: confirmation isUsed=<bool> buttons=<list>
  title=<title>` row, and a request sent by a confirmation button a
  `permission decision: confirmation=<label>` row ([#96], [#97]).
- The `cody` parser writes `model: <id>` and `model: <old> -> <new>`
  rows from the assistant message `model`, per chat, and turns each
  `processes[]` entry of type `confirmation`, dropped before, into a
  `permission decision: confirmation state=<state> title=<title>` row
  ([#96], [#97]).
- The `twinny` parser writes `model: <id>` and `model: <old> -> <new>`
  rows from `meta.model`, per conversation, and a `permission decision:
  waiting` or `permission decision: skipped` row (with
  ` approval=<command|change>` when recorded) after the result of a tool
  step saved while it waited for the user's Run or Skip, or after the
  user skipped it ([#96], [#97]).
- The `zed` parser yields `model: <model> provider=<provider>` once per
  thread after the thread-start row, and one `permission setting:
  <key>=<value> (thread blob, as of updated_at)` row per thread-scoped
  grant in the thread blob: `profile`, each `sandbox_grants.write_paths`
  and `sandbox_grants.network_hosts` entry, and each of the
  `network_any_host`, `allow_fs_write_all`, `unsandboxed` and
  `sandbox_fallback` flags that is set, with the thread id as
  `session_id` and the thread's `updated_at` as the time ([#96], [#97]).
- The `goose` parser yields `model: <model_name> provider=<provider_name>`
  and `permission mode: <goose_mode>` after each session's start row, and
  `model: <old> -> <new> provider=<provider>` (with
  ` requested=<requestedModel>` when the resolved model differs) before a
  message whose `metadata_json.inference` names another model ([#96],
  [#97]).
- The `kiro` parser yields `model: <id>` before a conversation's first
  history entry and `model: <old> -> <new>` before each entry whose
  `request_metadata.model_id` differs; an entry without one falls back to
  `model_info.model_id` (or the legacy `model`) and the row ends
  ` source=model_info` (or ` source=model`) ([#96]).
- The `hermes` parser yields `model: <id>` and
  `model: <old> -> <new>` rows from the main-loop rows of the
  `session_model_usage` table, at each model's `first_seen` and with
  ` provider=<billing_provider>`, or one `model: <sessions.model>` row at
  `started_at` when the session has none ([#96]).
- The `ollama` parser yields `model: <model_name>` before a chat's first
  assistant message that names a model and `model: <old> -> <new>` before
  each assistant message whose `model_name` differs ([#96]).
- The `openhands`, `agent-zero` and `nanobot` parsers record the model
  as an event: `openhands` writes `model: <agent.llm.model>` after each
  conversation's header row; `agent-zero` writes `model: <id>` before an
  agent's first reply and `model: <old> -> <new>` when
  `metadata.responses.provider_model_key` changes, ending ` agent=<n>`
  for a subordinate agent; `nanobot` writes `model: <preset> (preset)`
  after the session start row, since the session records a model preset
  of its configuration, not a model id ([#96]).
- The `openhands` parser follows a rejected call's `tool_result` with a
  `system` row `permission decision: rejected
  rejection_source=<source> <reason>`, and ends an action's `tool_use`
  text with ` security_risk=<value>`, the security analyzer's rating
  ([#97]).
- The `muse-code` parser records the starting permission profile and
  security mode as `permission mode: approval=<a> reviewer=<r>
  profile=<id>` (with its `cause` and declared `format`) and
  `permission mode: security_mode=<value>`, the automated reviewer's
  verdict as `permission decision: <outcome>
  kind=automated_review_completed ...` with its rationale, the model as
  `model: <id> provider=<provider>` at the first model a session log
  names and `model: <old> -> <new>` on each change, and each trusted
  project in `.config/muse/trust.json`, not read before, as a
  `permission setting: projects.<path>.decision=<decision> (from
  .config/muse/trust.json, file mtime)` row ([#96], [#97]).
- The `claude-code` `permission decision:` row ends with
  `toolDenialKind=<value> userFeedback=<text>` when the line records
  either key, and a line with either key but no `permissionDecision`
  gets the row too ([#97]).
- The `claude-desktop` parser writes `model: <id>` from each audit log's
  `init` line even when the transcript exists, `model: <old> -> <new>`
  in the full audit fallback, and a `model_refusal_fallback:` row; adds
  one `permission grant:` row per entry of `approvedToolNames`,
  `webFetchAllowedUrls`, `egressAllowedDomains`,
  `userApprovedFileAccessPaths`, `enabledMcpTools` and
  `fileDeleteApprovedMounts` in a session record; ends each scheduled
  task row with `permission_mode=<mode> model=<model>`; and reads
  `remote-session-spaces.json`, not read before, as one
  `permission setting:` row per folder granted to a remote Cowork
  session, at the file's modification time ([#96], [#97]).
- The catalog copy detects the ChatGPT desktop app's renamed Windows
  package, `OpenAI.Codex` (its Chromium profile, log directory and
  settings hive), as `chatgpt-desktop` ([#113]).
- The `codex-cli` and `open-interpreter` session start row ends with
  `account=<creator_account_id> user=<creator_user_id>` when the
  rollout's `session_meta` names the ChatGPT account and user that
  created the thread, as Codex Desktop inside the ChatGPT desktop app
  writes them; a rollout without them keeps the old text ([#118]).
- The `codex-cli` parser reads the ChatGPT desktop app's scheduled
  automations, `.codex/automations/<slug>/automation.toml`, not read
  before: one `system` row per file, `scheduled task: <id> name=<name>
  kind=<kind> status=<status> rrule=<rrule> target_thread=<thread id>
  created=<UTC> updated=<UTC> (from <file>, file mtime) prompt:
  <prompt>`, at the file's modification time, with `session_id` the
  thread the automation runs in ([#119]).
- The catalog copy detects the ChatGPT desktop app on macOS (its
  `Library/Application Support/Codex` profile, logs, preferences,
  `HTTPStorages` and computer-use group container) as `chatgpt-desktop`,
  and attributes `Library/Application Support/com.openai.codex` to
  `chatgpt-desktop` instead of `codex-cli` ([#121]).

### Changed

- The bundled catalog copy follows collector 1.12.0's project catalog: a
  project-directory file is detected as the agent that owns it
  (`claude-code` for `.claude/` and `CLAUDE.md`, `cursor` for `.cursor/`,
  `aider` for `.aider.chat.history.md`), and only files several agents
  read (`AGENTS.md`, `AGENTS.override.md`, `AGENT.md`, `.agents/`,
  `.mcp.json`, the shared GitHub Copilot instruction files, `SOUL.md`,
  `USER.md`) stay `project` ([#104]).
- Files from a project directory are attributed to the agent that owns
  them in `detect` and `inventory.json`, not to `project`:
  a repository's `.claude/` is `claude-code`, its
  `.aider.chat.history.md` `aider`. A manifest row with agent `project`,
  as collectors before 1.12.0 wrote for every project file, is
  re-attributed from the bundled project catalog; a file several agents
  read (`AGENTS.md`) stays `project` and is counted as a shared file of
  its project. Project directories are recognised
  from `collection.json` `projects` (or a row with agent `project`), not
  from the agent, so the `claude-code` parser still gives a project's
  settings rows the project's `project_path` ([#104]).
- `detect --json`: the flat key `agents`, one object per user, home and
  agent, is replaced by `homes[].agents`, each home's agents sorted by
  name with `agent`, `files`, `bytes` and `parser` (0.11.0's `homes` had
  no `agents` key); `homes` lists user homes and Docker volumes only,
  since project directories moved to `projects`. Nothing in the object
  is named `outside_projects`. The first table `detect` prints leaves out
  project directories the same way. A script that read `.agents` reads
  `.homes[].agents`, plus `.projects[].agents` for project files
  ([#104]).
- The `agents:` line `timeline` prints reads
  `parsed: <list>; detected but not parsed: <list>`, with `none` for an
  empty list, followed by `; parser found no rows: <list>` when a
  detected agent's parser produced no rows, and `none were found` for an
  input with no agent data. It used to read
  `parsed <list>; detected only <list>`, which said `detected only none`
  when every detected agent was parsed, `parsed none; detected only none`
  for an empty input, and nothing about agents whose parser found no
  rows ([#85]).
- The `qwen-code` model rows take the common form: a `session_model`
  record reads `model: <id> auth=<authType>` (was
  `session_model: <id> auth=<authType>`), and a `logs.json`
  `model_switch` entry reads `model: <from> -> <to> reason=<reason>`
  (was `model_switch: <from> -> <to> (<reason>)`) ([#96]).
- The pi session reader, which parses `pi` sessions, the little-coder
  sessions in pi's directory and Letta's local-backend `messages.jsonl`,
  writes `model: <id> provider=<provider>` at the first entry that names
  a model (a `model_change` or an assistant message) and
  `model: <old> -> <new> provider=<provider>` on each change, with
  ` requested=<model>` when the assistant's `responseModel` differs from
  the `model` asked for. A `model_change` entry that names the model
  already in force, a switch the user made, still gets the plain
  `model: <id> provider=<provider>` row. It replaces
  `model_change: <provider>/<id>`, which appeared only for a
  `model_change` entry, so a session whose model was named only on
  assistant messages had no model row. The `model` column is now
  `responseModel` when the message has one ([#96]).
- The `openclaw` parser writes `model: <id> provider=<provider>` at a
  session's first model (the `session_windows` `model`, else the first
  `model_change` or assistant message) and `model: <old> -> <new>
  provider=<provider>` on each change, with ` requested=<model>` and the
  repeated `model_change` row as above, in place of
  `model change: <provider>/<id>` ([#96]).
- The `cline` parser's model rows take the `model: <id>` and
  `model: <old> -> <new>` form. An SDK session's start row no longer ends
  with `model=<id>`; a `model: <id> provider=<provider>` row follows it
  (also for a `sessions.db` index row), and a message whose
  `modelInfo.id` differs yields `model: <old> -> <new>
  provider=<provider>`. The legacy `model_usage: model_id=...` rows from
  `task_metadata.json` become `model: <id> model_provider_id=<p>
  mode=<m>` at the first entry and on a change of `model_id`, and
  `permission mode: <mode>` and `permission mode: <old> -> <new>` rows
  carry the entry's `mode`; an entry that changes neither has no row
  ([#96], [#97]).
- The `muse-code` approval decision row reads `permission decision:
  <decision> source=<decision_source.kind> policy_result=<p>
  amendment=<a>`, where it read `approval <decision> by <source>`; and the
  `/model` row reads `model: <old> -> <new>`, preceded by `model:
  <previous>` when the log named no model before it, where it read
  `model: <new>` ([#96], [#97]).

### Fixed

- `tools/proto_descriptors.py extract` no longer fails with a `TypeError`
  on Mach-O binaries such as the macOS builds of `agy` and the Antigravity
  desktop app's language server ([#86]).
- The `antigravity` parser reads the path of a `code_action` step that
  deletes a file: `ActionSpecDeleteFile` keeps the path in field 1, where
  the parser had read field 2, so the `tool_result` row lost the path or
  the step could not be decoded ([#88]).
- `tools/shape.py` redacts OpenAI user and organisation ids (`user-` or
  `org-` and 20 or more letters and digits) as `<user-id>` and
  `<org-id>` in keys and paths, a `win32:/...` path key and an absolute
  path after a `<prefix>:` in a key (`local:C:\...`) as `<abs-path>`,
  and the elements of a key that is a JSON-encoded array, which it
  printed whole before: Codex Desktop's `.codex-global-state.json` keys
  state by arrays holding the user id and absolute file paths ([#117]).

### Removed

- `timeline` no longer writes `detect.json`; its `kind`, `notes`,
  `problems` and per-home agent table are in `inventory.json` as `kind`,
  `notes`, `problems` and `homes` ([#104]).

## [0.11.0] - 2026-10-12

### Added

- The bundled catalog copy follows collector 1.11.0: Claude Desktop's
  MSIX data directories under `AppData/Local/Packages`, and
  `AppData/Local/Claude` and `AppData/Local/Claude-Data`, are detected as
  `claude-desktop` ([#73]).
- The bundled catalog copy follows collector 1.11.0: the Claude Code CLI
  cache directory (`Library/Caches/claude-cli-nodejs`,
  `.cache/claude-cli-nodejs`, `AppData/Local/claude-cli-nodejs/Cache/*`), which
  holds per-project MCP server logs, is detected as `claude-code` ([#81]).

- A `claude-desktop` parser: Claude Desktop's Cowork and Code-tab
  transcripts (Claude Code JSONL under the app-data directory, with the
  project path taken from the session record), each Cowork session's
  `audit.jsonl` (permission requests, responses and automatic decisions,
  and every turn when the transcript is missing), session records,
  scheduled tasks and Code-tab worktrees. The rows come out under agent
  `claude-desktop` ([#48]).
- The bundled catalog copy follows collector 1.10.0: the agents
  `autogen-studio`, `camel-ai`, `crewai`, `dify`, `flowise`, `langflow`,
  `metagpt`, `n8n` and `pydantic-clai`, the project entry `.clai`, the
  legacy Open Interpreter directories under `open-interpreter`, and the
  Dify, Langflow and n8n Docker volume patterns are detected; none is
  parsed yet ([#64]).
- Claude Code 2.1.295 permission decisions: a tool-result line that
  records how the call was allowed or denied gets a `system` row after
  its result, `permission decision: <decision> source=<source>
  reason=<reasonType>`, carrying the call's `tool_use_id` ([#76]).
- Claude Code's per-process registry `~/.claude/sessions/<pid>.json` is
  read: one `system` row per process at its start time, `claude-code
  process <pid> <kind> status=<status> entrypoint=<entrypoint>
  bridge=<id or none> host_session=<id or none>`, joined to the session.
  A `bridge=` id means the process was under Remote Control when the
  collection ran; check it against the EDR's process list, since a
  crashed process can leave its file behind ([#80]).
- Claude Desktop Code-tab session rows end with the session's persisted
  Remote Control state, `remote_control=<bool> bridges=<n>
  spawned_remotely=<bool> steered_remotely=<bool>`, and a session that
  was under Remote Control when the app quit to update gets a `remote
  control live at update quit` row at that time ([#80]).

### Fixed

- The detection test for the framework agents passes on a case-insensitive
  filesystem (macOS): the two spellings of CrewAI's Windows directory are
  built in separate homes instead of one ([#75]).

## [0.10.0] - 2026-10-06

### Changed

- The analyzer is now `doubleagent`: `pip install .` installs the
  distribution `doubleagent` (was `coding-agent-analyzer`) with the
  command `doubleagent` (was `analyze-agent-artifacts`), and the Python
  package is `doubleagent` (was `agent_analyzer`), so `python3 -m
  doubleagent` runs it from a checkout. `--version` prints
  `doubleagent <version>`. The output files and their fields are
  unchanged ([#47]).
- `zstandard` is a required dependency, imported at start-up like
  `python-dateutil`. The `system` row that said "zstandard package not
  installed" for each compressed Zed thread, Codex or Open Interpreter
  rollout and OpenClaw row is gone ([#47]).
- `pyproject.toml` reads the version from `doubleagent.VERSION`, so it is
  set in one place ([#47]).

## [0.9.0] - 2026-10-06

### Added

- An `ollama` parser for the Ollama desktop app's chat database
  (`Library/Application Support/Ollama/db.sqlite` on macOS,
  `AppData/Local/Ollama/db.sqlite` on Windows, schema versions 16 to 19)
  and the `ollama run` prompt history `.ollama/history`. Tool calls and
  results are paired by position, with `tool_use_id` set to
  `<chat id>:<tool_calls.id>`; attachments appear as their name and size
  only, and the account and settings tables are not read. History rows
  have no timestamp or session, so they sort last and are dropped by
  `--since` and `--until` unless `--keep-undated` is given ([#43]).

### Changed

- The bundled catalog copy follows the collector: the Ollama exclusion is
  now `.ollama/models/blobs`, so `.ollama/models/manifests` and
  `models/metadata` files appear in collections ([#44]).

### Fixed

- Timestamps with a fraction of one, two, four or five digits, such as
  go-sqlite3's `2026-03-01 09:00:00.5-08:00`, are converted on Python 3.10
  as well; before, they gave an empty `timestamp_utc` there ([#45]).
- `tests/run.sh` runs only the named tests when given a module, class or
  method name (`tests/run.sh test_catalog`,
  `tests/run.sh test_parsers.ClaudeCodeTests.test_rows`); before, it passed
  the name to `unittest discover`, which ignored it and ran the whole suite.
  With no names it still runs discover with the options given, and with
  names it refuses the discover-only options `-p`, `-s` and `-t` ([#34]).

### Removed

- The bundled catalog copy drops the collectors' unused credential pattern
  `.claude-*/.device-keys.json`; detection is unchanged ([#41]).

## [0.8.1] - 2026-10-05

### Added

- `detect` prints `problem:` lines too, and `detect --json` and
  `detect.json` have a new last key `problems`, an array of the problem
  texts; in `detect.json` it also holds the files a parser could not read ([#37]).

### Fixed

- An input that cannot be opened, including a missing path, an archive
  that is not readable, an archive inside a directory that cannot be
  traversed, an input directory that cannot be listed and an unreadable
  `manifest.jsonl`, now prints `error: <path>: <reason>` with the OS
  reason in lower case (`no such file or directory`, `permission denied`)
  and exits 2 under `detect`, `timeline` and `inventory`. Before, it
  printed only the path, or a Python traceback for permission errors. A
  corrupt or truncated archive prints `error: <path>: cannot extract:
  <reason>` instead of a traceback, and the extracted work directory is
  removed on every such error ([#37]).
- A directory inside a loose tree that cannot be listed is now a
  `problem:` line instead of being skipped silently ([#37]).
- A manifest row whose file is absent from the extracted collection is now
  one `problem:` line, `<original path>: missing from the collection` ([#37]).
- Problem lines and the `collection.json unreadable` note name the path
  once: `<path>: permission denied` instead of
  `<path>: [Errno 13] Permission denied: '<path>'`. The Continue and Codex
  `parser: unreadable ...` system rows carry only the reason, since
  `source_file` names the file ([#37]).

## [0.8.0] - 2026-10-04

### Added

- `timeline --since WHEN` and `--until WHEN` keep only the rows inside a
  UTC time window, `--keep-undated` also keeps rows without a timestamp,
  and `--match REGEX` (repeatable, `-i` for case-insensitive) keeps rows
  whose `text` matches, with the paired tool call or result. `WHEN` is an
  ISO 8601 date or time (a space may replace the `T`, and an offset may
  follow after a space), a year or month, an epoch number, `today`,
  `yesterday`, `now`, or a relative time such as `45m`, `36h`, `3d`, `2w`,
  `6mo`, `1y`. A value names a period to its last written unit, so
  `--since 2026-10-01 --until 2026-10-01` is that whole day. The resolved
  window and the dropped-row counts are printed. `sessions.jsonl` is
  summarised from the kept rows.
- `python-dateutil` is a new required dependency, used for `--since` and
  `--until`; reinstall with `pip install -r requirements.txt`.
- The [analyzer README](README.md) has recipes for filtering a written timeline by time
  with jq, DuckDB, Python and PowerShell.

### Changed

- `timeline` writes `timeline.jsonl` and `sessions.jsonl` instead of
  `timeline.csv` and `sessions.csv`: one JSON object per line, UTF-8, LF
  line ends, every field present in the old column order, `""` for no
  value. Interface change: `source_line`, `user_turns`, `assistant_turns`
  and `tool_calls` are JSON integers, and `models` is an array of strings
  instead of a space-separated string. Field names are unchanged.
- The `text` field keeps the event's line breaks and indentation, so tool
  output, diffs and multi-line prompts read as they were written; only
  leading and trailing whitespace is removed. Before, whitespace runs were
  collapsed to one space.
- The [analyzer README](README.md) has a section on reading and searching large JSONL
  timelines with jq, grep, DuckDB, Python and PowerShell.

### Removed

- `--max-text-length`. `text` is always written in full; cut it while
  reading, for example with `.text[0:200]` in jq.
- The `shell-history` catalog agent, following collector 1.7.0, which no
  longer collects shell history. Loose input no longer attributes
  `.bash_history`, `.zsh_history` and the other shell history files to an
  agent.
- The `shared|.env` and `project|.env` catalog entries, following collector
  1.7.0. Loose input no longer attributes a home or project `.env` file to
  `shared` or `project`.

## [0.7.0] - 2026-10-04

### Added

- Claude Code: sessions and `history.jsonl` in a config home moved with
  `CLAUDE_CONFIG_DIR` to a `.claude-<name>` directory (such as
  `~/.claude-work`) are parsed, and the bundled catalog copy detects
  them; `~/.claude-code-router` and other `.claude-` tools are not.
- Claude Code: a tool result stored as a `<persisted-output>` stub now
  carries the full output from the session's `tool-results/*.txt` file;
  when the file is missing, the stub is kept with a
  `[persisted output not found: ...]` note.
- Claude Code: each subagent transcript opens with a `system` row
  `subagent <agent id> of <session id>`.
- Claude Code: `session title:` rows for `ai-title`, `custom-title` and
  `agent-name` records, `cwd changed:` rows for `relocated` records (later
  rows take the new `project_path`), `hook:` rows for hook system messages
  and `edited file:` rows for `edited_text_file` attachments (the path
  only, not the snippet).

### Changed

- Claude Code: the Bash `tool_use` text is the command the model sent,
  with its `cd <dir> &&` prefix, instead of the normalised one.
- Claude Code: other injected context (`isMeta` records) starts with
  `context:`, and `system` records without `content` (`api_error`,
  `memory_saved`, ...) show their fields as JSON after the subtype.
- Open Interpreter rollouts are read by the same code as Codex CLI's, so
  every Codex CLI change below applies to them too.
- Codex CLI: a rollout that holds an ancestor's `session_meta` (a copied
  fork) keeps the file's own session id, project and branch; the ancestor's
  record is a `system` row `copied from ancestor: ...`, and a fork gets a
  `forked from <id>` row.
- Codex CLI: a non-zero exit code from a command's `item_completed` record
  prefixes that call's `tool_result` text as `[exit N]`.
- Codex CLI: a subagent rollout starts with a `system` row
  `subagent <id> of <parent id>` (with its agent path and role), and its
  first prompt, the task from the parent, is a `system` row
  `subagent task: ...` instead of `user`. Messages copied from the parent
  (`inherited_user_message`) are no longer repeated in the subagent's rows.

### Fixed

- Claude Code: prompts typed while a turn was running and delivered as
  `queued_command` attachments are now `user` rows; they were missing.
  The `enqueue` that repeated a delivered prompt's text is now
  `prompt queued: delivered at line N`, keeping the time it was typed;
  an `enqueue` with no delivery in the file is still
  `queue enqueue: <prompt>`.
- Claude Code: a subagent's task, task notifications, slash commands and
  their output, peer and coordinator messages, interruption markers and
  compaction summaries were `user` rows; they are now `system` rows with
  a label (`subagent task:`, `task notification:`, `slash command:`,
  `command output:`, `peer message:`, `coordinator message:`,
  `interrupted:`, `compaction summary:`).
- Claude Code: local API errors (model `<synthetic>`) were `assistant`
  rows; they are now `system` rows `api error: <status> <error>: <text>`.
- Claude Code: empty thinking blocks (signature only) no longer produce
  `[redacted]` thinking rows; only `redacted_thinking` blocks do.
- Claude Code: `pr-link` rows, re-written on every metadata flush, appear
  once per session and URL, and with the session's project and branch,
  like the queue rows.
- Claude Code: tool results that list tools (`tool_reference` blocks) show
  the tool names instead of empty text.
- Codex CLI: user-role messages the harness injects (environment context,
  AGENTS.md and skill instructions, subagent notifications, user shell
  commands) were `user` rows; they are now `system` rows
  `context: <kind>: ...`, by content kind where the rollout records one
  (current releases) and otherwise by the harness's own wrapper around the
  block (`<environment_context>`, `# AGENTS.md instructions`, `<skill>`
  and others), so `user` rows and `user_turns` count only what the person
  typed. A tag typed inside a prompt does not change its type.
- Codex CLI: compaction summaries, messages between agents
  (`inter_agent_communication`, `agent_message`), image generation
  prompts, tool searches, realtime voice transcripts, aborted and
  rolled-back turns, and `item_completed` turn items that no other record
  carries (web searches inside code-mode calls, MCP and dynamic tool calls,
  plans) were dropped; they now produce rows.
- Codex CLI: a reverted thread's rollout
  (`rollout-<time>-<thread id>_<rollout id>.jsonl`) cut before its
  `session_meta` took the rollout id as session id; it now takes the
  thread id.
- Codex CLI: a reasoning item with no summary text was a `thinking` row
  with empty text under `--include-thinking`; it is now skipped.

## [0.6.0] - 2026-10-04

### Added

- Muse Code parser (`muse-code`): session logs under
  `.local/share/muse/sessions/` and their subagent logs (user prompts,
  assistant replies, tool calls and results, approval requests and
  decisions, failed runs; reasoning summaries with `--include-thinking`),
  and the `tui-history.jsonl` prompt history.
- The catalog copy detects `muse-code` (Meta's Muse Code CLI).

### Changed

- Minimum Python raised from 3.9 to 3.10.

### Fixed

- OpenClaw: a `transcript_events` row whose `event_json` is valid JSON
  but not an object (such as `null`) no longer stops the parser with
  `UnboundLocalError`; it becomes a `system` row, "event seq N
  unreadable: not a JSON object", and the session's later events are
  still read.

## [0.5.0] - 2026-10-04

### Added

- `inventory` command: merges the saved stdout of collector 1.6.0
  `--inventory` / `-Inventory` runs (files, or directories of files) into
  one CSV, `fleet-inventory.csv` unless `-o FILE` is given.
- New CSV interface, `fleet-inventory.csv`, columns `host`, `collector`,
  `mode`, `at`, `user`, `agent`, `files`, `bytes`, `first`, `last`,
  `projects`, `evidence`; times normalised to UTC with milliseconds. A host
  with no agents gives one row with the agent columns empty.
- Console junk lines in a capture are skipped and counted on stderr.
- A per-agent rollup on stdout after the CSV: hosts, users and latest
  `last`.
- The catalog copy carries collector 1.5.0's `DOCKER_VOLUMES` table; the
  reader keeps those lines in their own list instead of the secret list.

### Changed

- The [README](README.md) says the `live` pseudo-agent appears only in collections made
  by collectors before 1.6.0, which removed the live snapshot.

## [0.4.0] - 2026-10-04

### Added

- Transcript parsers for fourteen more agents: `tabby`, `openhands`,
  `shellgpt`, `pi`, `little-coder`, `letta`, `hermes`, `agent-zero`,
  `open-interpreter`, `openclaw`, `nanobot`, `cody`, `twinny` and `pearai`,
  for thirty parsed agents in all. little-coder and Letta Code sessions are
  read with the pi parser's reader, and Open Interpreter rollouts with the
  Codex one.
- `reads_agents` routing: a parser can name other catalog agents whose files
  it is also offered. Cody and Twinny use it to read their rows of the
  `state.vscdb` that the catalog attributes to `vscode` or to a fork
  (`cursor`, `windsurf`, `pearai`, `kiro`, `antigravity`); the rows come out
  under `cody` or `twinny`, and `--agent cody` selects them.
- `agent_analyzer/vscode_state.py`, a helper that reads one extension's
  global state out of a VS Code family `state.vscdb` through
  `sqlite_util`, never reading `secret://` rows.
- Detection of the fifteen agents added in collector 1.4.0: `pearai`,
  `cody`, `twinny`, `tabby`, `open-interpreter`, `openhands`, `pi`,
  `little-coder`, `letta`, `hermes`, `openclaw`, `nanobot`, `agent-zero`,
  `shellgpt` and `local-deep-research`, from the regenerated catalog copy.

### Changed

- The Codex parser now also reads archived rollouts under
  `archived_sessions/` and zstd-compressed `.jsonl.zst` rollouts (needs
  `zstandard`; without it each compressed rollout is one `system` row). Its
  rollout reader is now the reusable `RolloutParser` base class.

## [0.3.1] - 2026-10-04

### Fixed

- The `timeline` summary no longer lists `project` among the agents that
  were detected but not parsed.

## [0.3.0] - 2026-10-03

### Added

- Transcript parsers for Gemini CLI, Qwen Code, Kiro (Amazon Q Developer CLI
  conversations), Crush, Goose, Zed, VS Code chat sessions, Continue, Aider,
  OpenCode, Kilo Code, Cline and Roo Code, for sixteen parsed agents in all.
- Dependency on the `zstandard` package for Zed threads; without it each Zed
  thread becomes one undecodable `system` row and everything else runs.

### Changed

- Files the collector found inside a discovered repository (agent `project`
  in the manifest) are offered to every parser, so a repository's
  `.crush/crush.db` and `.aider.*` files are parsed and their rows carry the
  real agent. `--agent` filters on that agent, so `--agent aider` includes
  repository-level Aider files.
- `detect` no longer reports `project` as a parsed agent.

## [0.2.0] - 2026-10-03

### Added

- Antigravity CLI parser for its SQLite conversation databases, decoded from
  the protobuf schemas embedded in the shipped binary. SQLite stores are read
  from a copy taken with their WAL and SHM sidecars, so the evidence copy is
  never opened and uncheckpointed rows are kept.
- Detection of the catalog entries added in collectors 1.2.0 and 1.3.0.

### Changed

- `timeline.csv`: the `summary` column is renamed `text`, in the same
  position, and holds the event's full text by default.
- `--max-text-length N` replaces `--summary-length N` and defaults to `0`,
  no cut; the old default cut at 400 characters.
- In loose mode a nested catalog entry claims its subtree, as in the
  collectors, so Antigravity CLI files under `~/.gemini` are attributed to
  `antigravity`.

## [0.1.0] - 2026-10-03

### Added

- `detect`, `timeline` and `catalog` commands, reading a collector archive,
  an extracted collection, or a loose directory such as a mounted image, a
  copied home or a single agent directory.
- Detection of every agent in the collectors' catalog. Host, user and agent
  come from the manifest when present; for loose input they are inferred
  from paths and marked as inferred.
- Parsers for Claude Code (session transcripts, subagent transcripts,
  `history.jsonl`) and Codex CLI (rollouts, `history.jsonl`).
- `timeline.csv` columns: `timestamp_utc`, `host`, `user`, `agent`,
  `session_id`, `project_path`, `git_branch`, `turn_type`, `model`,
  `tool_name`, `tool_use_id`, `summary`, `source_file`, `source_line`.
- `sessions.csv` columns: `host`, `user`, `agent`, `session_id`,
  `project_path`, `first_timestamp_utc`, `last_timestamp_utc`, `models`,
  `user_turns`, `assistant_turns`, `tool_calls`, `source_file`.
- Options `--host`, `--user`, `--work-dir`, `--keep-extracted`,
  `--summary-length` (default 400), `--include-thinking` and `--agent`;
  `detect --json` and `--files`; `catalog --agents`.

[Unreleased]: https://github.com/seanthegeek/doubleagent/compare/analyzer-v0.22.0...HEAD
[0.22.0]: https://github.com/seanthegeek/doubleagent/compare/analyzer-v0.21.0...analyzer-v0.22.0
[0.21.0]: https://github.com/seanthegeek/doubleagent/compare/analyzer-v0.20.1...analyzer-v0.21.0
[0.20.1]: https://github.com/seanthegeek/doubleagent/compare/analyzer-v0.20.0...analyzer-v0.20.1
[0.20.0]: https://github.com/seanthegeek/doubleagent/compare/analyzer-v0.19.0...analyzer-v0.20.0
[0.19.0]: https://github.com/seanthegeek/doubleagent/compare/analyzer-v0.18.0...analyzer-v0.19.0
[0.18.0]: https://github.com/seanthegeek/doubleagent/compare/analyzer-v0.17.0...analyzer-v0.18.0
[0.17.0]: https://github.com/seanthegeek/doubleagent/compare/analyzer-v0.16.0...analyzer-v0.17.0
[0.16.0]: https://github.com/seanthegeek/doubleagent/compare/analyzer-v0.15.0...analyzer-v0.16.0
[0.15.0]: https://github.com/seanthegeek/doubleagent/compare/analyzer-v0.14.0...analyzer-v0.15.0
[0.14.0]: https://github.com/seanthegeek/doubleagent/compare/analyzer-v0.13.0...analyzer-v0.14.0
[0.13.0]: https://github.com/seanthegeek/doubleagent/compare/analyzer-v0.12.0...analyzer-v0.13.0
[0.12.0]: https://github.com/seanthegeek/doubleagent/compare/analyzer-v0.11.0...analyzer-v0.12.0
[0.11.0]: https://github.com/seanthegeek/doubleagent/compare/analyzer-v0.10.0...analyzer-v0.11.0
[0.10.0]: https://github.com/seanthegeek/doubleagent/compare/analyzer-v0.9.0...analyzer-v0.10.0
[0.9.0]: https://github.com/seanthegeek/doubleagent/compare/analyzer-v0.8.1...analyzer-v0.9.0
[0.8.1]: https://github.com/seanthegeek/doubleagent/compare/analyzer-v0.8.0...analyzer-v0.8.1
[0.8.0]: https://github.com/seanthegeek/doubleagent/compare/analyzer-v0.7.0...analyzer-v0.8.0
[0.7.0]: https://github.com/seanthegeek/doubleagent/compare/analyzer-v0.6.0...analyzer-v0.7.0
[0.6.0]: https://github.com/seanthegeek/doubleagent/compare/analyzer-v0.5.0...analyzer-v0.6.0
[0.5.0]: https://github.com/seanthegeek/doubleagent/compare/analyzer-v0.4.0...analyzer-v0.5.0
[0.4.0]: https://github.com/seanthegeek/doubleagent/compare/analyzer-v0.3.1...analyzer-v0.4.0
[0.3.1]: https://github.com/seanthegeek/doubleagent/compare/analyzer-v0.3.0...analyzer-v0.3.1
[0.3.0]: https://github.com/seanthegeek/doubleagent/compare/analyzer-v0.2.0...analyzer-v0.3.0
[0.2.0]: https://github.com/seanthegeek/doubleagent/compare/analyzer-v0.1.0...analyzer-v0.2.0
[0.1.0]: https://github.com/seanthegeek/doubleagent/releases/tag/analyzer-v0.1.0

[#333]: https://github.com/seanthegeek/doubleagent/issues/333
[#326]: https://github.com/seanthegeek/doubleagent/issues/326
[#324]: https://github.com/seanthegeek/doubleagent/issues/324
[#321]: https://github.com/seanthegeek/doubleagent/issues/321
[#285]: https://github.com/seanthegeek/doubleagent/issues/285
[#283]: https://github.com/seanthegeek/doubleagent/issues/283
[#282]: https://github.com/seanthegeek/doubleagent/issues/282
[#281]: https://github.com/seanthegeek/doubleagent/issues/281
[#280]: https://github.com/seanthegeek/doubleagent/issues/280
[#277]: https://github.com/seanthegeek/doubleagent/issues/277
[#269]: https://github.com/seanthegeek/doubleagent/issues/269
[#256]: https://github.com/seanthegeek/doubleagent/issues/256
[#253]: https://github.com/seanthegeek/doubleagent/issues/253
[#237]: https://github.com/seanthegeek/doubleagent/issues/237
[#236]: https://github.com/seanthegeek/doubleagent/issues/236
[#231]: https://github.com/seanthegeek/doubleagent/issues/231
[#230]: https://github.com/seanthegeek/doubleagent/issues/230
[#228]: https://github.com/seanthegeek/doubleagent/issues/228
[#227]: https://github.com/seanthegeek/doubleagent/issues/227
[#225]: https://github.com/seanthegeek/doubleagent/issues/225
[#224]: https://github.com/seanthegeek/doubleagent/issues/224
[#223]: https://github.com/seanthegeek/doubleagent/issues/223
[#221]: https://github.com/seanthegeek/doubleagent/issues/221
[#220]: https://github.com/seanthegeek/doubleagent/issues/220
[#219]: https://github.com/seanthegeek/doubleagent/issues/219
[#218]: https://github.com/seanthegeek/doubleagent/issues/218
[#217]: https://github.com/seanthegeek/doubleagent/issues/217
[#216]: https://github.com/seanthegeek/doubleagent/issues/216
[#215]: https://github.com/seanthegeek/doubleagent/issues/215
[#214]: https://github.com/seanthegeek/doubleagent/issues/214
[#213]: https://github.com/seanthegeek/doubleagent/issues/213
[#212]: https://github.com/seanthegeek/doubleagent/issues/212
[#211]: https://github.com/seanthegeek/doubleagent/issues/211
[#210]: https://github.com/seanthegeek/doubleagent/issues/210
[#209]: https://github.com/seanthegeek/doubleagent/issues/209
[#208]: https://github.com/seanthegeek/doubleagent/issues/208
[#207]: https://github.com/seanthegeek/doubleagent/issues/207
[#206]: https://github.com/seanthegeek/doubleagent/issues/206
[#205]: https://github.com/seanthegeek/doubleagent/issues/205
[#204]: https://github.com/seanthegeek/doubleagent/issues/204
[#203]: https://github.com/seanthegeek/doubleagent/issues/203
[#202]: https://github.com/seanthegeek/doubleagent/issues/202
[#201]: https://github.com/seanthegeek/doubleagent/issues/201
[#200]: https://github.com/seanthegeek/doubleagent/issues/200
[#199]: https://github.com/seanthegeek/doubleagent/issues/199
[#198]: https://github.com/seanthegeek/doubleagent/issues/198
[#197]: https://github.com/seanthegeek/doubleagent/issues/197
[#196]: https://github.com/seanthegeek/doubleagent/issues/196
[#195]: https://github.com/seanthegeek/doubleagent/issues/195
[#194]: https://github.com/seanthegeek/doubleagent/issues/194
[#193]: https://github.com/seanthegeek/doubleagent/issues/193
[#192]: https://github.com/seanthegeek/doubleagent/issues/192
[#191]: https://github.com/seanthegeek/doubleagent/issues/191
[#190]: https://github.com/seanthegeek/doubleagent/issues/190
[#189]: https://github.com/seanthegeek/doubleagent/issues/189
[#188]: https://github.com/seanthegeek/doubleagent/issues/188
[#187]: https://github.com/seanthegeek/doubleagent/issues/187
[#186]: https://github.com/seanthegeek/doubleagent/issues/186
[#185]: https://github.com/seanthegeek/doubleagent/issues/185
[#184]: https://github.com/seanthegeek/doubleagent/issues/184
[#183]: https://github.com/seanthegeek/doubleagent/issues/183
[#182]: https://github.com/seanthegeek/doubleagent/issues/182
[#181]: https://github.com/seanthegeek/doubleagent/issues/181
[#180]: https://github.com/seanthegeek/doubleagent/issues/180
[#179]: https://github.com/seanthegeek/doubleagent/issues/179
[#178]: https://github.com/seanthegeek/doubleagent/issues/178
[#177]: https://github.com/seanthegeek/doubleagent/issues/177
[#176]: https://github.com/seanthegeek/doubleagent/issues/176
[#175]: https://github.com/seanthegeek/doubleagent/issues/175
[#174]: https://github.com/seanthegeek/doubleagent/issues/174
[#173]: https://github.com/seanthegeek/doubleagent/issues/173
[#172]: https://github.com/seanthegeek/doubleagent/issues/172
[#171]: https://github.com/seanthegeek/doubleagent/issues/171
[#170]: https://github.com/seanthegeek/doubleagent/issues/170
[#169]: https://github.com/seanthegeek/doubleagent/issues/169
[#168]: https://github.com/seanthegeek/doubleagent/issues/168
[#167]: https://github.com/seanthegeek/doubleagent/issues/167
[#166]: https://github.com/seanthegeek/doubleagent/issues/166
[#165]: https://github.com/seanthegeek/doubleagent/issues/165
[#160]: https://github.com/seanthegeek/doubleagent/issues/160
[#159]: https://github.com/seanthegeek/doubleagent/issues/159
[#158]: https://github.com/seanthegeek/doubleagent/issues/158
[#157]: https://github.com/seanthegeek/doubleagent/issues/157
[#155]: https://github.com/seanthegeek/doubleagent/issues/155
[#154]: https://github.com/seanthegeek/doubleagent/issues/154
[#153]: https://github.com/seanthegeek/doubleagent/issues/153
[#151]: https://github.com/seanthegeek/doubleagent/issues/151
[#149]: https://github.com/seanthegeek/doubleagent/issues/149
[#147]: https://github.com/seanthegeek/doubleagent/issues/147
[#143]: https://github.com/seanthegeek/doubleagent/issues/143
[#139]: https://github.com/seanthegeek/doubleagent/issues/139
[#138]: https://github.com/seanthegeek/doubleagent/issues/138
[#136]: https://github.com/seanthegeek/doubleagent/issues/136
[#135]: https://github.com/seanthegeek/doubleagent/issues/135
[#134]: https://github.com/seanthegeek/doubleagent/issues/134
[#133]: https://github.com/seanthegeek/doubleagent/issues/133
[#126]: https://github.com/seanthegeek/doubleagent/issues/126
[#125]: https://github.com/seanthegeek/doubleagent/issues/125
[#124]: https://github.com/seanthegeek/doubleagent/issues/124
[#123]: https://github.com/seanthegeek/doubleagent/issues/123
[#122]: https://github.com/seanthegeek/doubleagent/issues/122
[#121]: https://github.com/seanthegeek/doubleagent/issues/121
[#119]: https://github.com/seanthegeek/doubleagent/issues/119
[#118]: https://github.com/seanthegeek/doubleagent/issues/118
[#117]: https://github.com/seanthegeek/doubleagent/issues/117
[#113]: https://github.com/seanthegeek/doubleagent/issues/113
[#112]: https://github.com/seanthegeek/doubleagent/issues/112
[#104]: https://github.com/seanthegeek/doubleagent/issues/104
[#103]: https://github.com/seanthegeek/doubleagent/issues/103
[#97]: https://github.com/seanthegeek/doubleagent/issues/97
[#96]: https://github.com/seanthegeek/doubleagent/issues/96
[#93]: https://github.com/seanthegeek/doubleagent/issues/93
[#92]: https://github.com/seanthegeek/doubleagent/issues/92
[#90]: https://github.com/seanthegeek/doubleagent/issues/90
[#89]: https://github.com/seanthegeek/doubleagent/issues/89
[#88]: https://github.com/seanthegeek/doubleagent/issues/88
[#86]: https://github.com/seanthegeek/doubleagent/issues/86
[#85]: https://github.com/seanthegeek/doubleagent/issues/85
[#83]: https://github.com/seanthegeek/doubleagent/issues/83
[#81]: https://github.com/seanthegeek/doubleagent/issues/81
[#80]: https://github.com/seanthegeek/doubleagent/issues/80
[#76]: https://github.com/seanthegeek/doubleagent/issues/76
[#75]: https://github.com/seanthegeek/doubleagent/issues/75
[#73]: https://github.com/seanthegeek/doubleagent/issues/73
[#64]: https://github.com/seanthegeek/doubleagent/issues/64
[#51]: https://github.com/seanthegeek/doubleagent/issues/51
[#50]: https://github.com/seanthegeek/doubleagent/issues/50
[#49]: https://github.com/seanthegeek/doubleagent/issues/49
[#48]: https://github.com/seanthegeek/doubleagent/issues/48
[#47]: https://github.com/seanthegeek/doubleagent/issues/47
[#45]: https://github.com/seanthegeek/doubleagent/issues/45
[#44]: https://github.com/seanthegeek/doubleagent/issues/44
[#43]: https://github.com/seanthegeek/doubleagent/issues/43
[#41]: https://github.com/seanthegeek/doubleagent/issues/41
[#37]: https://github.com/seanthegeek/doubleagent/issues/37
[#34]: https://github.com/seanthegeek/doubleagent/issues/34
[#6]: https://github.com/seanthegeek/doubleagent/issues/6
