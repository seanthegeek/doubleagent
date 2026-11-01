# qwen-live

Qwen Live (catalog agent `qwen-live`) is the voice daemon in the Qwen Code repository
([QwenLM/qwen-code](https://github.com/QwenLM/qwen-code), `packages/qwen-live`), a Node.js
process that holds a spoken conversation with a DashScope realtime model (DashScope is Alibaba
Cloud's model API service; its realtime models, `qwen-omni`, take and return audio) and hands the
coding work the user asks for to other agents. It drives each backend agent either over ACP (the
Agent Client Protocol, a JSON-RPC protocol over a child process's standard input and output) or
over the REST API of `qwen serve`, Qwen Code's daemon; its surfaces are the daemon itself and Live
Host, a macOS overlay app (the "orb") that carries the microphone, screen and camera to it. It
shares its repository and lineage with Qwen Code ([`qwen-code.md`](qwen-code.md)), but keeps its
own data directory, `~/.qwen-live`, and writes one record into Qwen Code's `~/.qwen`. Not parsed
yet: the agent is in the parser backlog ([Parser plan](#parser-plan)).

## Source and evidence

[QwenLM/qwen-code](https://github.com/QwenLM/qwen-code), Apache-2.0, open source, TypeScript,
commit
[`835b080f8567d8612effa248e1fafa11af0e244b`](https://github.com/QwenLM/qwen-code/commit/835b080f8567d8612effa248e1fafa11af0e244b)
(2026-11-01), where `packages/qwen-live/package.json` gives version 0.1.0
([package.json:4](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/package.json#L4)). `packages/qwen-live` is identical at the latest release
tag, `v0.26.5` (commit
[`db0e4e2`](https://github.com/QwenLM/qwen-code/commit/db0e4e24a0d89b211b2a15f4af2be22adac71dfc)),
so the release writes the shapes described here. The package's README
([README.md:33](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/README.md#L33)) installs it by building the monorepo, and it needs Node.js
22.13 or later for the built-in SQLite module. Every claim is from source at the reviewed commit,
read with the package's
[README](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/README.md) there, unless it cites another; paths cited are relative to
`packages/qwen-live` unless they name another package. Read only: nothing was installed or run.

### Observed

- No real install. What stands in is the source review of QwenLM/qwen-code at
  [`835b080`](https://github.com/QwenLM/qwen-code/commit/835b080f8567d8612effa248e1fafa11af0e244b)
  (`qwen-live` 0.1.0, beside release `v0.26.5`); every path and record shape below is
  source-derived.

### Source reviews

- 2026-11-01: QwenLM/qwen-code at
  [`835b080`](https://github.com/QwenLM/qwen-code/commit/835b080f8567d8612effa248e1fafa11af0e244b)
  (`qwen-live` 0.1.0). First review: the data directory, the call log and its record types, the
  memory libraries and their SQLite schema, the configuration and its credentials, the discovery
  record in `~/.qwen` and the debug recordings in the temp directory.

## Collection

### Per-user storage

The data directory is `~/.qwen-live` on every platform (the home directory joined with
`.qwen-live`, with no platform branch), or `$QWEN_LIVE_DATA_DIR` when it is set
([src/config.ts:864-869](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/config.ts#L864-L869); [src/index.ts:30-33](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/index.ts#L30-L33));
the setup wizard, `qwen-live init`, always writes `~/.qwen-live`
([src/init.ts:40-41](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/init.ts#L40-L41)). Directories are created mode 0700 and files 0600.

| Path under `~/.qwen-live` | Holds | Source |
| --- | --- | --- |
| `config.json` | the configuration: the DashScope key, the realtime model and voice, the backends, the default working directory and the memory, visual input and Proactive (background monitor) settings ([Credentials](#credentials)) | [src/config.ts:864-922](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/config.ts#L864-L922) |
| `config.json.tmp` | the wizard's temporary copy, renamed over `config.json` | [src/init.ts:386-391](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/init.ts#L386-L391) |
| `config.json.<uuid>.tmp` | the temporary copy the daemon writes when it saves a language, display or memory setting, renamed over `config.json`, removed when the write fails | [src/language-preferences.ts:38-45](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/language-preferences.ts#L38-L45); [src/visual-preferences.ts:28-42](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/visual-preferences.ts#L28-L42); [src/memory/service.ts:51-85](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/memory/service.ts#L51-L85) |
| `sessions/live-<ms>-<nonce>.jsonl` | the call log of one daemon run, `<ms>` the start time in epoch milliseconds and `<nonce>` the first eight characters of the run's random UUID ([Call log](#call-log)) | [src/daemon.ts:93](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/daemon.ts#L93), [235-238](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/daemon.ts#L235-L238); [src/log/session-log.ts:67-71](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/log/session-log.ts#L67-L71) |
| `sessions/live-<ms>-<nonce>.jsonl.<n>` | the earlier parts of a call log past 32 MiB, `<n>` counting from 1; nothing removes them | [src/log/session-log.ts:26](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/log/session-log.ts#L26), [136-147](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/log/session-log.ts#L136-L147) |
| `memories/<id>/meta.json` | a memory library's id, display name and times; `<id>` is `default` or `lib_<8 hex>` | [src/memory/store.ts:159-187](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/memory/store.ts#L159-L187), [189-211](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/memory/store.ts#L189-L211) |
| `memories/<id>/.meta.<uuid>.tmp` | the temporary copy of `meta.json` before its rename | [src/memory/store.ts:227-245](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/memory/store.ts#L227-L245) |
| `memories/<id>/dialogue.db` | the library's SQLite database in WAL mode, with its `dialogue.db-wal` and `dialogue.db-shm` sidecars ([Memory library](#memory-library)) | [src/memory/store.ts:288-300](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/memory/store.ts#L288-L300) |

The memory root moves with the `memory.dir` setting: a relative path resolves under the data
directory and an absolute one may lie anywhere
([src/memory/config.ts:318-325](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/memory/config.ts#L318-L325)). The README says the same
([README.md:462](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/README.md#L462)).

#### Discovery record in `~/.qwen`

The daemon publishes `~/.qwen/live/daemon.json` (`live/daemon.json` under the home's `.qwen`,
whatever `$QWEN_HOME` says) for Live Host to find it, writing it through a temporary
`.qwen/live/.daemon.<pid>.<uuid>.tmp` beside it under the lock `.qwen/live/.daemon.lock`, and
removes it when it stops
([src/host/discovery.ts:16](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/host/discovery.ts#L16), [86-90](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/host/discovery.ts#L86-L90), [372](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/host/discovery.ts#L372), [505-545](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/host/discovery.ts#L505-L545)).
`qwen serve`'s built-in Live module writes the same file
([cli/src/serve/live/discovery.ts:22](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/serve/live/discovery.ts#L22), [92-94](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/cli/src/serve/live/discovery.ts#L92-L94)),
so the file stays with the `qwen-code` entry that reaches it, and its secret globs are listed in
[Credentials](#credentials). The record's keys are `url`, `token`, `configPath`,
`protocolVersion`, `pid` and `instanceNonce`
([src/host/discovery.ts:33-40](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/host/discovery.ts#L33-L40)).

#### Monitor debug recordings

With `--debug` or `QWEN_LIVE_LOG_LEVEL=debug`, each visual Proactive monitor records the requests
it sends to the model under `qwen-live-monitor-debug/` in the operating system's temporary
directory ([src/proactive/monitor-debug-store.ts:21](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/proactive/monitor-debug-store.ts#L21);
[README.md:376-410](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/README.md#L376-L410)):

| Path under `qwen-live-monitor-debug/` | Holds | Source |
| --- | --- | --- |
| `monitor-<ms>-<uuid>/monitor.json` | the monitor's settings, `format` `qwen-live-monitor-debug-v1` and its creation time | [src/proactive/monitor-debug-store.ts:123-147](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/proactive/monitor-debug-store.ts#L123-L147) |
| `monitor-<ms>-<uuid>/requests/<6 digits>/request.json` | one request: the instructions, the event order, frame hashes, audio offsets and the name of the previous request | [src/proactive/monitor-debug-store.ts:385-451](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/proactive/monitor-debug-store.ts#L385-L451) |
| `monitor-<ms>-<uuid>/requests/<6 digits>/response.json` | the text the model returned, the parsed action or the failure | [src/proactive/monitor-debug-store.ts:348](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/proactive/monitor-debug-store.ts#L348) |
| `monitor-<ms>-<uuid>/requests/<6 digits>/image-<4 digits>.jpg` | each screen or camera frame sent to the model | [src/proactive/monitor-debug-store.ts:395](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/proactive/monitor-debug-store.ts#L395), [442-445](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/proactive/monitor-debug-store.ts#L442-L445) |
| `monitor-<ms>-<uuid>/requests/<6 digits>/input.wav` | the microphone audio sent, mono 16 kHz PCM | [src/proactive/monitor-debug-store.ts:409](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/proactive/monitor-debug-store.ts#L409), [446-449](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/proactive/monitor-debug-store.ts#L446-L449) |

The ten most recently created monitor directories are kept, with no bound on the requests in one
([src/proactive/monitor-debug-store.ts:110](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/proactive/monitor-debug-store.ts#L110), [202](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/proactive/monitor-debug-store.ts#L202)).
On Windows the temporary directory is `AppData/Local/Temp` under the home, which the catalog line
`qwen-live|AppData/Local/Temp/qwen-live-monitor-debug` reaches; on Linux and macOS it is `/tmp`
or `$TMPDIR`, outside every home, a manual step in
[Agents outside dot directories in `collectors/docs/coverage.md`](../collectors/docs/coverage.md#agents-outside-dot-directories).

#### Live Host

Live Host (`packages/live-host`), an Electron app for macOS only, has the identifier
`com.alibaba.qwen-code.live-host` and the product name `Qwen Live Host`
([live-host/electron-builder.yml:1-2](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/live-host/electron-builder.yml#L1-L2);
[README.md:146-148](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/README.md#L146-L148)). It keeps only its overlay position, theme and
language in Electron's user data directory
([live-host/src/main/index.ts:543](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/live-host/src/main/index.ts#L543), [1636](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/live-host/src/main/index.ts#L1636), [1666](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/live-host/src/main/index.ts#L1666)),
`overlay-position.json`, `theme.json` and `language.json`, so no catalog line names it.

#### Backend agents

The setup wizard looks on the `PATH` for Qoder CLI (`qodercli`), Qwen Code (`qwen`), Gemini CLI
(`gemini`), Claude Code (`claude`) and Codex (`codex`) and launches the chosen one in ACP mode,
the last two through `npx` adapter packages
([src/agent-detector.ts:33-83](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/agent-detector.ts#L33-L83);
[README.md:431-438](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/README.md#L431-L438)). Each backend keeps its own sessions in its own
state directory, where its catalog entry collects them; Qoder CLI has no catalog entry.

#### Scratch directories

None known.

### Credentials

| Path | Holds |
| --- | --- |
| `.qwen-live/config.json`, `.qwen-live/config.json.tmp`, `.qwen-live/config.json.<uuid>.tmp` (the files `.qwen-live/config.json*`) | `realtimeApiKey`, the DashScope key; a `qwen-code` backend's `token` and the legacy `serveToken`, `qwen serve` bearer tokens; an ACP backend's `env` values ([src/config.ts:705-783](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/config.ts#L705-L783), [812-819](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/config.ts#L812-L819), [873-876](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/config.ts#L873-L876)) |
| `.qwen/live/daemon.json`, `.qwen/live/.daemon.<pid>.<uuid>.tmp` | `token`, the bearer token of the running daemon's HTTP and WebSocket endpoints ([src/host/discovery.ts:33-40](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/host/discovery.ts#L33-L40)) |

The DashScope key may instead come from `DASHSCOPE_API_KEY` or `QWEN_LIVE_REALTIME_API_KEY`, and
the memory updater's and observer's `apiKeyEnv` names an environment variable rather than holding
a key ([README.md:482](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/README.md#L482)). The keychain is not used. The monitor recordings omit
the connection credentials ([README.md:402](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/README.md#L402)).

### Exclusions

`AppData/Local/Temp/qwen-live-monitor-debug/*/requests/*/image-*.jpg` and
`AppData/Local/Temp/qwen-live-monitor-debug/*/requests/*/input.wav`: the screen or camera frames
and microphone audio of the debug recordings, raw copies of what the user's screen, camera and
microphone captured, unbounded in number. The JSON beside them is collected, so the requests,
their frame hashes and the model's answers stay. `~/.qwen-live` itself is small: call logs grow by
one line per event and a memory library by one row per turn.

### Project-local files

None: the daemon writes nothing into a project.

### Where the project path is recorded

Nowhere per call. `config.json` holds `defaultCwd`, the working directory of sessions the daemon
creates, and an ACP backend's `cwd` ([src/config.ts:769](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/config.ts#L769),
[891-892](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/config.ts#L891-L892)); a call log's `tool.call` records keep the arguments of a
`session_create` call, `cwd` among them, as a JSON string
([src/tools/definitions.ts:74-105](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/tools/definitions.ts#L74-L105)). Project discovery reads
neither: the configuration names one default for every session and is a credential file, and the
key inside a JSON string is escaped. The analyzer reads them.

## Parsing

### Transcript stores

The call log, `~/.qwen-live/sessions/live-<ms>-<nonce>.jsonl`, is one append-only JSONL file per
daemon run, holding every call of that run; the source calls it "the only durable record of a
call" ([src/log/session-log.ts:7-14](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/log/session-log.ts#L7-L14)). It keeps the final text
of what the user said and what the model answered, the tool calls with their arguments, the
permission requests and decisions, and the backend events, but never raw audio or image bytes
(screenshots appear as asset ids). The memory library keeps a second copy of the dialogue
([Memory library](#memory-library)).

#### What survives a delete

The daemon deletes no call log and no memory library: the logs are never pruned, a full log is
renamed to `.jsonl.<n>` and kept, turning memory off keeps the selected library and its data
([README.md:458](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/README.md#L458)), and the store's `deactivateSession`, which drops a call's
search rows and marks it inactive while keeping its `turns`, has no caller at the reviewed commit
([src/memory/store.ts:350-394](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/memory/store.ts#L350-L394)). A call log removed by hand
leaves the call's dialogue in the `turns` table of `dialogue.db`, and the facts consolidated from
it in `ltm_entries` and `stm_items`; a library removed by hand leaves the call logs. The backend
agents' own transcripts hold the tasks handed to them and survive both. Expired short-term items
are marked inactive, not deleted. Nothing is parsed yet; a parser would read the `turns` table as
a secondary source
([Secondary sources in `analyzer/docs/output.md`](../analyzer/docs/output.md#secondary-sources)).

### Record schema

#### Call log

Each line is one object ([src/log/session-log.ts:78-96](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/log/session-log.ts#L78-L96)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `ts` | number | | |
| `seq` | number | | |
| `type` | string | `session.start`, `session.end`, `vad.speech_started`, `vad.speech_stopped`, `transcript.user`, `transcript.assistant`, `response.created`, `response.done`, `response.cancelled`, `tool.call`, `tool.result`, `backend.event`, `inject.context`, `inject.speech`, `permission.request`, `permission.decision`, `playback.started`, `playback.cleared`, `error` | |
| `payload` | object | | |

`ts` is the write time in epoch milliseconds and `seq` counts lines from 1 in the run; the types
are those of [src/log/session-log.ts:28-47](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/log/session-log.ts#L28-L47). The payload keys
by type, from the writers in `src/orchestrator/live-session.ts`:

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `callId` | string | | |
| `epoch` | number | | |
| `mode` | string | `resume`, `new` | |
| `backends` | string | | |
| `model` | string | | |
| `voice` | string | | |
| `phase` | string | `realtime_ready`, `input_committed` | |
| `reason` | string | | |
| `error` | string | | |
| `text` | string | | |
| `direct` | boolean | | |
| `responseId` | string | | |
| `authority` | string | `direct`, `tool_continuation`, `backend_speech`, `proactive`, `proactive_repair` | |
| `status` | string | | |
| `name` | string | | |
| `args` | string | | |
| `argumentChars` | number | | |
| `ok` | boolean | | |
| `receipt` | string | | |
| `kind` | string | | |
| `job` | string | | |
| `chars` | number | | |
| `session` | string | | |
| `jobRef` | string | | |
| `resubscribeInMs` | number | | |
| `sawEvent` | boolean | | |
| `source` | string | `realtime`, `tool_output`, `proactive_tool`, `session_list`, `permission`, `pump`, `proactive_repair`, `proactive_task` | |
| `message` | string | | |
| `requestHandle` | string | | |
| `requestId` | string | | |
| `title` | string | | |
| `decision` | string | `allow`, `deny` | |
| `auto` | boolean | | |
| `outcome` | string | `delivered`, `already_resolved`, `delivery_failed` | |
| `note` | string | | |

- `session.start` opens a call with `callId` (the identifier Live Host gives the call), `epoch`,
  `mode` (whether Live Host resumed a call or started a new one), `backends` (the backend names
  joined by commas), `model` and `voice`, and repeats as
  `{"phase": "realtime_ready"}` once the model connection is up
  ([src/orchestrator/live-session.ts:557-564](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/orchestrator/live-session.ts#L557-L564), [1455](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/orchestrator/live-session.ts#L1455)).
  `session.end` carries the connection's `reason`, and when the call itself ends, `callId` and
  any `error` ([1878](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/orchestrator/live-session.ts#L1878), [3756-3759](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/orchestrator/live-session.ts#L3756-L3759)).
- `transcript.user` and `transcript.assistant` carry `text`, the full final transcript of one
  utterance; a turn the model answered without speech arrives with `direct: true`
  ([1525](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/orchestrator/live-session.ts#L1525), [1538](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/orchestrator/live-session.ts#L1538), [1796-1799](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/orchestrator/live-session.ts#L1796-L1799)).
- `response.created`, `response.done` and `response.cancelled` carry `responseId` and the
  response's `authority` (who asked for it) and, on `response.done`, its `status`
  ([1656-1659](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/orchestrator/live-session.ts#L1656-L1659), [1713-1717](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/orchestrator/live-session.ts#L1713-L1717), [1744-1746](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/orchestrator/live-session.ts#L1744-L1746);
  [src/realtime/realtime-session.ts:150-155](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/realtime/realtime-session.ts#L150-L155)).
- `tool.call` carries the tool `name`, `callId` and `args`, the arguments as the model sent them,
  cut at 2,000 characters; for the two memory tools it carries `argumentChars` instead.
  `tool.result` carries `name`, `callId`, `ok` and `receipt`, the answer given to the model, cut
  at 2,000 characters
  ([1914-1920](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/orchestrator/live-session.ts#L1914-L1920), [1944-1949](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/orchestrator/live-session.ts#L1944-L1949)).
  The tools are `appshot`, `session_list`, `session_create`, `handoff`, `session_monitor`,
  `session_stop`, `respond_permission` and the Proactive tools
  ([src/tools/definitions.ts:20-32](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/tools/definitions.ts#L20-L32)); `handoff` sends a
  `task` to a backend session.
- `inject.context` and `inject.speech` record context fed to the model, by `kind`, `job` and
  length in `chars`, not its text ([546-550](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/orchestrator/live-session.ts#L546-L550)).
- `backend.event` carries the Live session handle `session`, the event `type` and any `jobRef`,
  and a `stream_ended` event `resubscribeInMs` and `sawEvent`
  ([2772-2778](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/orchestrator/live-session.ts#L2772-L2778), [2799-2804](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/orchestrator/live-session.ts#L2799-L2804)).
- `permission.request` carries `requestHandle`, `requestId`, `session` and `title`;
  `permission.decision` carries `requestHandle`, `requestId`, `decision`, `auto`, `outcome` and
  any `note`, and `error` when an automatic answer failed
  ([src/permissions/permission-broker.ts:145-150](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/permissions/permission-broker.ts#L145-L150), [160-167](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/permissions/permission-broker.ts#L160-L167), [306-313](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/permissions/permission-broker.ts#L306-L313)).
- `error` carries `source` and `message`, with the provider's `code`, `kind` and `status` for a
  model error ([658-668](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/orchestrator/live-session.ts#L658-L668), [1836-1846](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/orchestrator/live-session.ts#L1836-L1846)).
- `vad.speech_started`, `vad.speech_stopped` and `playback.cleared` mark speech and playback
  timing, with at most a `phase`, `reason` or `responseId`; `playback.started` is a declared type
  that only the debug output uses at the reviewed commit
  ([804](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/orchestrator/live-session.ts#L804), [1485-1487](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/orchestrator/live-session.ts#L1485-L1487), [1501-1519](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/orchestrator/live-session.ts#L1501-L1519), [1739-1742](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/orchestrator/live-session.ts#L1739-L1742)).

#### Memory library

`dialogue.db` is created from one schema, version 1 in `schema_meta`
([src/memory/schema.ts:9](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/memory/schema.ts#L9), [34-157](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/memory/schema.ts#L34-L157)).
Its `session_id` is the call's `callId`
([src/orchestrator/live-session.ts:1399-1400](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/orchestrator/live-session.ts#L1399-L1400)).
The `turns` table, one row per exchange
([src/memory/session.ts:560-576](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/memory/session.ts#L560-L576)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `id` | number | | |
| `session_id` | string | | |
| `turn_idx` | number | | |
| `user_text` | string | | |
| `user_ts` | string | | |
| `user_epoch` | number | | |
| `asst_text` | string | | |
| `asst_ts` | string | | |
| `asst_epoch` | number | | |
| `interrupted` | number | `0`, `1` | |
| `seg_id` | number or null | | |

`user_ts` and `asst_ts` are local times without a zone, `YYYY-MM-DD HH:MM:SS`, and `user_epoch`
and `asst_epoch` the same moments in epoch seconds
([src/memory/recorder.ts:33-36](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/memory/recorder.ts#L33-L36), [64-65](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/memory/recorder.ts#L64-L65)).
The library's calls, in `library_sessions`:

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `session_id` | string | | |
| `session_name` | string or null | | |
| `first_seen_at` | string | | |
| `last_seen_at` | string | | |
| `n_turns` | number | | |
| `n_segments` | number | | |
| `active` | number | `0`, `1` | |
| `deactivated_at` | string or null | | |
| `deactivate_reason` | string or null | | |

The facts the model kept (`wm_snapshots`, the working memory, one row per edit), the
consolidated long-term profile (`ltm_entries`) and short-term items (`stm_items`), and the visual
observations (`stm_env`):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `seq` | number | | |
| `wm_json` | string | | |
| `ops_json` | string | | |
| `applied_json` | string | | |
| `created_at` | string | | |
| `field` | string | `name`, `occupation_or_role`, `long_term_goals`, `routines`, `appearance`, `preferences`, `interests`, `relationships` | |
| `content` | string | | |
| `updated_at` | string | | |
| `src_session` | string or null | | |
| `created_ts` | number | | |
| `event_date` | string or null | | |
| `expires_at` | string or null | | |
| `expired_at` | string or null | | |

`stm_items.status` is `ongoing` or `upcoming`. The rest are derived: `dialogue_segments` (runs of
turns joined into one `body` for search), the full-text search tables `dialogue_fts` and
`env_fts`, `embeddings` (vectors as a blob), `preload_log` and `updater_log` (what was loaded into
a call and what the consolidation model returned, with its `model`).

#### Library metadata

A library's `meta.json`, which the store writes when it creates the library and rewrites on a
rename (`touchLibrary`, which would also rewrite it, has no caller at the reviewed commit)
([src/memory/store.ts:159-187](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/memory/store.ts#L159-L187), [213-225](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/memory/store.ts#L213-L225)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `version` | number | `1` | |
| `id` | string | | |
| `name` | string | | |
| `created_at` | string | | |
| `updated_at` | string | | |

### Joins and timestamps

A call log's lines are ordered by `seq` and stamped by `ts` in epoch milliseconds, the time the
daemon wrote them. A call is the run of lines from a `session.start` with a `callId` to the
`session.end` with the same `callId`; the file name stem identifies the run. A tool call joins
its result on `callId` (the realtime call id, distinct from the call's `callId`), a permission
request its decision on `requestHandle`. A memory turn joins its call on `session_id` = `callId`
and on the text: `user_text` and `asst_text` repeat the final `transcript.user` and
`transcript.assistant` texts, the library storing only final user transcripts and ordinary replies
([README.md:464](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/README.md#L464)); its times are whole seconds. The backend sessions are known
only by Live's own handles (`session`), so a backend's transcript joins a call only by time and
by the `task` text of a `handoff`, which the backend receives as its prompt.

### Other stores

The memory library is described under [Memory library](#memory-library), the configuration under
[Credentials](#credentials), and the monitor recordings under
[Monitor debug recordings](#monitor-debug-recordings); their records time each request in epoch
milliseconds (`createdAt`, `sentAt`).

### Format versions

One build, 0.1.0, at the reviewed commit; the memory schema is version 1, migrated forward by
`schema_meta` ([src/memory/store.ts:302-326](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/memory/store.ts#L302-L326)), and the
monitor recordings' `format` is `qwen-live-monitor-debug-v1`
([src/proactive/monitor-debug-store.ts:22](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/proactive/monitor-debug-store.ts#L22)).

### Secrets

The call log holds no key ([src/log/session-log.ts:13-14](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/log/session-log.ts#L13-L14)),
but `transcript.user`, `tool.call` `args` and the memory tables hold whatever the user said, a
spoken secret included; the monitor recordings' frames and audio show and say whatever the screen,
camera and microphone caught ([README.md:401-403](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/README.md#L401-L403)).

### Parser plan

Globs (home-relative): `.qwen-live/sessions/live-*.jsonl`, `.qwen-live/sessions/live-*.jsonl.*`
and `.qwen-live/memories/*/dialogue.db`, opened through `sqlite_util`. Not read: `config.json`,
`meta.json` and the monitor recordings.

| Column | Source |
| --- | --- |
| timestamp_utc | `ts`; a memory row's `user_epoch`, `asst_epoch` or `created_at` |
| session_id | the `callId` of the call the line falls in, else the file name stem; a memory row's `session_id` or `src_session` |
| project_path | empty; a `session_create` call's `cwd` stays in its `tool_use` text |
| git_branch | empty |
| turn_type | `transcript.user` → user; `transcript.assistant` → assistant; `tool.call` → tool_use; `tool.result` → tool_result; the other kinds read → system |
| model | `model` of `session.start` |
| tool_name | `name` |
| tool_use_id | `callId` of `tool.call` and `tool.result` |
| text | `text`; `args` or `argumentChars`; `receipt` with `ok`; the system row texts below |

System rows: `call started: <callId> (mode <mode>, backends <backends>)` and
`call ended: <callId> (<reason or error>)`; `backend event: <type> (session <session>)`;
`injected <kind> context: <chars> characters`; `error (<source>): <message>`. Not read:
`vad.speech_started`, `vad.speech_stopped`, `response.created`, `response.done`,
`response.cancelled` and `playback.cleared`, which time speech and playback only.

#### Permission rows

Each `permission.request` gives a `system` row `permission request: <title> (request
<requestId>, session <session>)`, and each `permission.decision` the row
`permission decision: <decision> (auto <auto>, outcome <outcome>)`, both with `tool_use_id`
`requestHandle`. A spoken "allow always" is delivered as an `allow` and kept only in the daemon's
memory as a rule with an expiry, so it has no record of its own; the requests it answers later
appear with `auto: true`
([src/permissions/permission-broker.ts:11-13](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/permissions/permission-broker.ts#L11-L13), [189-190](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/qwen-live/src/permissions/permission-broker.ts#L189-L190)).

#### Model rows

`session.start` names the realtime model of each call: one `system` row `model: <model>` at the
first call naming one, with the voice beside it, and `model: <old> -> <new>` when a later call
names another. The memory consolidation model is in `updater_log.model`, shown in that row's text
and not as a model row.

#### Past-use rows and secondary sources

`dialogue.db` `turns` is secondary to the call log: each row gives a `user` row of `user_text` at
`user_epoch` and an `assistant` row of `asst_text` at `asst_epoch`, joined on `session_id` =
`callId` and the text, so only the turns of a removed call log remain. Because its times are whole
seconds taken when the turn was recorded, `secondary_tolerance()` widens the window by a second
backwards plus the recorder's delay, which a round must measure. `wm_snapshots`, `ltm_entries`,
`stm_items` and `stm_env` are primary: one `system` row per row (`memory edit:`,
`memory fact: <field>:`, `memory item (<status>):`, `visual observation:` with the content) at
its `created_at` or `updated_at`, with `src_session` as the session.

#### Sync, relay and origin rows

The agent records none: the call goes to DashScope but no record marks a remote copy.

#### Scratch directory rows

The agent records none.

#### Deletion rows

The agent records none; a `library_sessions` row with `active` 0 gives a `system` row
`memory call deactivated: <deactivate_reason>` at `deactivated_at`, though no code sets one at the
reviewed commit ([What survives a delete](#what-survives-a-delete)).

#### Bad-line rows

Each call log line that does not decode becomes its own `parser: unparseable line` row, the lines
around it still read. A `dialogue.db` that cannot be opened, its sidecars included, or a call log
that cannot be read is one row `parser: <file> could not be read (<reason>)`.

#### Fixtures

Call log lines as `SessionLog.write` produces them for a call with one user turn, a tool call and
its result, and a permission request answered by voice, derived from the writers cited under
[Call log](#call-log), not from an install:

```json
{"ts":1793433600120,"seq":1,"type":"session.start","payload":{"callId":"call-7f3e","epoch":1,"mode":"voice","backends":"qwen","model":"qwen3.5-omni-plus-realtime","voice":"Tina"}}
{"ts":1793433601004,"seq":2,"type":"session.start","payload":{"phase":"realtime_ready"}}
{"ts":1793433606230,"seq":3,"type":"transcript.user","payload":{"text":"ask qwen to run the tests"}}
{"ts":1793433606910,"seq":4,"type":"tool.call","payload":{"name":"handoff","callId":"fc_01","args":"{\"task\":\"run the tests\"}"}}
{"ts":1793433607015,"seq":5,"type":"tool.result","payload":{"name":"handoff","callId":"fc_01","ok":true,"receipt":"{\"job\":\"job_1\"}"}}
{"ts":1793433612400,"seq":6,"type":"permission.request","payload":{"requestHandle":"perm_1","requestId":"7","session":"s1","title":"Run npm test"}}
{"ts":1793433615800,"seq":7,"type":"permission.decision","payload":{"requestHandle":"perm_1","requestId":"7","decision":"allow","auto":false,"outcome":"delivered"}}
```

## Confidence

- High: the data directory, the file names, the call log's line shape and record types, the
  memory schema and the credential keys (source).
- Medium: what `epoch` of `session.start` counts, which comes from Live Host's protocol; the
  delay between a spoken turn and its `turns` row.
- Not determined: the name of Live Host's user data directory on a real install, whether Electron
  names it by the product name or the package name, and what else Electron keeps there; how large
  call logs and libraries grow in use.
- For a real-install round: a call with a handoff to each backend kind, an approval, a denial and
  an "allow always", a memory edit, a renamed and a new library, a debug run with a visual monitor,
  and a call log removed by hand, with a listing diff after each.
