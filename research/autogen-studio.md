# autogen-studio

AutoGen Studio is Microsoft AutoGen's web UI (`autogenstudio ui`) for building and running
multi-agent teams. It came into the catalog from the agent framework sweep for
[issue #64](https://github.com/seanthegeek/doubleagent/issues/64); the AutoGen libraries and
Microsoft Agent Framework, which write nothing under the home by default, are covered in
[`research/frameworks.md`](frameworks.md). Its parser is in the backlog
([issue 69](https://github.com/seanthegeek/doubleagent/issues/69)).

## Source and evidence

[microsoft/autogen](https://github.com/microsoft/autogen), commit
[`027ecf0a379bcc1d09956d46d12d44a3ad9cee14`](https://github.com/microsoft/autogen/commit/027ecf0a379bcc1d09956d46d12d44a3ad9cee14).
Code is MIT
([LICENSE-CODE](https://github.com/microsoft/autogen/blob/027ecf0a379bcc1d09956d46d12d44a3ad9cee14/LICENSE-CODE)),
docs CC-BY-4.0. The repository ships `autogen-core`, `autogen-agentchat`, `autogen-ext`,
`magentic-one-cli` and `autogen-studio` 0.4.3
([version.py:1](https://github.com/microsoft/autogen/blob/027ecf0a379bcc1d09956d46d12d44a3ad9cee14/python/packages/autogen-studio/autogenstudio/version.py#L1)).
The README marks AutoGen as in maintenance mode and points new users to Microsoft Agent Framework
([README.md:14-25](https://github.com/microsoft/autogen/blob/027ecf0a379bcc1d09956d46d12d44a3ad9cee14/README.md#L14-L25)).
All claims are from source.

AutoGen Studio is in the catalog because its default gallery runs code on the host: a Python
code execution tool built on `LocalCommandLineCodeExecutor(work_dir=".coding")`, relative to
the working directory
([builder.py:433-439](https://github.com/microsoft/autogen/blob/027ecf0a379bcc1d09956d46d12d44a3ad9cee14/python/packages/autogen-studio/autogenstudio/gallery/builder.py#L433-L439)),
and an MCP filesystem workbench rooted at the home directory
([builder.py:562-573](https://github.com/microsoft/autogen/blob/027ecf0a379bcc1d09956d46d12d44a3ad9cee14/python/packages/autogen-studio/autogenstudio/gallery/builder.py#L562-L573)).

### Observed

- No real install: nothing was installed or run.

### Source reviews

- 2026-10-19: HEAD
  [`004749e`](https://github.com/microsoft/autogen/commit/004749e6b7102c8fd317b9b1499ce5807223c278)
  against the previously cited
  [`027ecf0`](https://github.com/microsoft/autogen/commit/027ecf0a379bcc1d09956d46d12d44a3ad9cee14),
  from source only. `python/packages/autogen-studio` is identical at the two commits (an empty
  `git diff`), so every citation still holds. Added the message records, the model, deletion and
  the `.coding` scratch directory ([Per-user storage](#per-user-storage)) and the absence of
  permission and sync state ([Parsing](#parsing)).

## Collection

### Per-user storage

The app directory is `~/.autogenstudio` (`Path.home()`) on every OS, unless
`AUTOGENSTUDIO_APPDIR` or `--appdir` is set
([initialization.py:41-45](https://github.com/microsoft/autogen/blob/027ecf0a379bcc1d09956d46d12d44a3ad9cee14/python/packages/autogen-studio/autogenstudio/web/initialization.py#L41-L45),
[cli.py:58-59](https://github.com/microsoft/autogen/blob/027ecf0a379bcc1d09956d46d12d44a3ad9cee14/python/packages/autogen-studio/autogenstudio/cli.py#L58-L59),
[README.md:130](https://github.com/microsoft/autogen/blob/027ecf0a379bcc1d09956d46d12d44a3ad9cee14/python/packages/autogen-studio/README.md#L130)).

| Path under `~/.autogenstudio` | Holds | Default or opt-in |
| --- | --- | --- |
| `autogen04203.db` | SQLite: tables `team`, `message`, `session`, `run` (task, `team_result`, messages as JSON), `gallery`, `settings`, eval tables. The README's `database.sqlite` name is out of date ([config.py:7](https://github.com/microsoft/autogen/blob/027ecf0a379bcc1d09956d46d12d44a3ad9cee14/python/packages/autogen-studio/autogenstudio/web/config.py#L7), [initialization.py:47-51](https://github.com/microsoft/autogen/blob/027ecf0a379bcc1d09956d46d12d44a3ad9cee14/python/packages/autogen-studio/autogenstudio/web/initialization.py#L47-L51), [db.py:51-170](https://github.com/microsoft/autogen/blob/027ecf0a379bcc1d09956d46d12d44a3ad9cee14/python/packages/autogen-studio/autogenstudio/datamodel/db.py#L51-L170)) | default unless `--database-uri` is set |
| `files/`, `files/user/`, `configs/` | static files, user files, team configs ([initialization.py:53-70](https://github.com/microsoft/autogen/blob/027ecf0a379bcc1d09956d46d12d44a3ad9cee14/python/packages/autogen-studio/autogenstudio/web/initialization.py#L53-L70), [config.py:11](https://github.com/microsoft/autogen/blob/027ecf0a379bcc1d09956d46d12d44a3ad9cee14/python/packages/autogen-studio/autogenstudio/web/config.py#L11)) | default |
| `alembic/`, `alembic.ini` | migration scaffolding, small ([deps.py:142](https://github.com/microsoft/autogen/blob/027ecf0a379bcc1d09956d46d12d44a3ad9cee14/python/packages/autogen-studio/autogenstudio/web/deps.py#L142), [schema_manager.py:43-45](https://github.com/microsoft/autogen/blob/027ecf0a379bcc1d09956d46d12d44a3ad9cee14/python/packages/autogen-studio/autogenstudio/database/schema_manager.py#L43-L45)) | default |
| `temp_env_vars.env` | host, port, app dir, database URI, auth config path; always under the home, even when `--appdir` is set ([cli.py:18-22](https://github.com/microsoft/autogen/blob/027ecf0a379bcc1d09956d46d12d44a3ad9cee14/python/packages/autogen-studio/autogenstudio/cli.py#L18-L22), [cli.py:51-74](https://github.com/microsoft/autogen/blob/027ecf0a379bcc1d09956d46d12d44a3ad9cee14/python/packages/autogen-studio/autogenstudio/cli.py#L51-L74), [lite/studio.py:131-135](https://github.com/microsoft/autogen/blob/027ecf0a379bcc1d09956d46d12d44a3ad9cee14/python/packages/autogen-studio/autogenstudio/lite/studio.py#L131-L135)) | default |
| `.env` | environment variables (API keys) that the user puts there and Studio loads ([initialization.py:72-77](https://github.com/microsoft/autogen/blob/027ecf0a379bcc1d09956d46d12d44a3ad9cee14/python/packages/autogen-studio/autogenstudio/web/initialization.py#L72-L77)) | read if present |

Studio lite mode uses an in-memory database
([lite/studio.py:150](https://github.com/microsoft/autogen/blob/027ecf0a379bcc1d09956d46d12d44a3ad9cee14/python/packages/autogen-studio/autogenstudio/lite/studio.py#L150)).

#### Scratch directories

The gallery's Python code execution tool runs in
`.coding`, created relative to the directory Studio was started from
([builder.py:433](https://github.com/microsoft/autogen/blob/004749e6b7102c8fd317b9b1499ce5807223c278/python/packages/autogen-studio/autogenstudio/gallery/builder.py#L433)).
It is the agent's own working directory, not a user project. Started from
the home, it is `~/.coding`, which no catalog line collects; started
elsewhere, it is outside the home. Nothing in the database records it, so
neither project discovery nor the analyzer can name it.

#### Relocation variables

| Variable | Moves | Replaces | Table agent |
| --- | --- | --- | --- |
| `AUTOGENSTUDIO_APPDIR` | the app directory, so the database, `files/`, `configs/` and `.env` ([initialization.py:41-45](https://github.com/microsoft/autogen/blob/027ecf0a379bcc1d09956d46d12d44a3ad9cee14/python/packages/autogen-studio/autogenstudio/web/initialization.py#L41-L45)) | `.autogenstudio` | `autogen-studio` |
| `AUTOGENSTUDIO_DATABASE_URI` | the database, when the URI names a SQLite file elsewhere ([initialization.py:47-51](https://github.com/microsoft/autogen/blob/027ecf0a379bcc1d09956d46d12d44a3ad9cee14/python/packages/autogen-studio/autogenstudio/web/initialization.py#L47-L51)) | `.autogenstudio/autogen04203.db` | `autogen-studio` |

Not relocation variables: `AUTOGENSTUDIO_AUTH_CONFIG` (names a user-written auth file, carries
secrets), `AUTOGENSTUDIO_TEAM_FILE` (input file, set for the server by the command),
`AUTOGENSTUDIO_HOST` and `AUTOGENSTUDIO_PORT` (behaviour only).

#### Docker

No compose file and no named volume. The Studio Dockerfile sets
`AUTOGENSTUDIO_APPDIR=/home/user/app` inside the image
([Dockerfile:11](https://github.com/microsoft/autogen/blob/027ecf0a379bcc1d09956d46d12d44a3ad9cee14/python/packages/autogen-studio/Dockerfile#L11)),
so no `DOCKER_VOLUMES` line.

#### Records

A run's stream is saved message by message as `message`
rows (`config` JSON, `session_id`, `run_id`) for the types
`TextMessage`, `MultiModalMessage`, `StopMessage`, `HandoffMessage`,
`ToolCallRequestEvent`, `ToolCallExecutionEvent` and
`LLMCallEventMessage`
([connection.py:129-143](https://github.com/microsoft/autogen/blob/004749e6b7102c8fd317b9b1499ce5807223c278/python/packages/autogen-studio/autogenstudio/web/managers/connection.py#L129-L143),
[173-186](https://github.com/microsoft/autogen/blob/004749e6b7102c8fd317b9b1499ce5807223c278/python/packages/autogen-studio/autogenstudio/web/managers/connection.py#L173-L186)),
and the run's final `TaskResult` goes to `run.team_result`.

#### Model

Every model call is logged as an AutoGen `LLMCallEvent`, which
Studio captures and saves as a message with source `llm_call_event`
([teammanager.py:26-37](https://github.com/microsoft/autogen/blob/004749e6b7102c8fd317b9b1499ce5807223c278/python/packages/autogen-studio/autogenstudio/teammanager/teammanager.py#L26-L37),
[types.py:25-26](https://github.com/microsoft/autogen/blob/004749e6b7102c8fd317b9b1499ce5807223c278/python/packages/autogen-studio/autogenstudio/datamodel/types.py#L25-L26)).
Its `content` is the event as JSON (`type: "LLMCall"`, `messages`,
`response`, `prompt_tokens`, `completion_tokens`, `tools`)
([logging.py:42-45](https://github.com/microsoft/autogen/blob/004749e6b7102c8fd317b9b1499ce5807223c278/python/packages/autogen-core/src/autogen_core/logging.py#L42-L45),
[63-64](https://github.com/microsoft/autogen/blob/004749e6b7102c8fd317b9b1499ce5807223c278/python/packages/autogen-core/src/autogen_core/logging.py#L63-L64)),
and for the OpenAI client `response` is the provider's response dump,
whose `model` is the resolved model id
([_openai_client.py:716-724](https://github.com/microsoft/autogen/blob/004749e6b7102c8fd317b9b1499ce5807223c278/python/packages/autogen-ext/src/autogen_ext/models/openai/_openai_client.py#L716-L724)).
The model the user configured is in the `team` row's component JSON and
in `settings.config.default_model_client`, `gpt-4o-mini` by default
([types.py:105-110](https://github.com/microsoft/autogen/blob/004749e6b7102c8fd317b9b1499ce5807223c278/python/packages/autogen-studio/autogenstudio/datamodel/types.py#L105-L110)).

#### Deletion

Deleting a session deletes the `session` row with foreign
keys on, and `run` (`ondelete="CASCADE"` on `session_id`) and `message`
(`ondelete="CASCADE"` on `run_id`) follow it
([sessions.py:58-62](https://github.com/microsoft/autogen/blob/004749e6b7102c8fd317b9b1499ce5807223c278/python/packages/autogen-studio/autogenstudio/web/routes/sessions.py#L58-L62),
[db_manager.py:253-263](https://github.com/microsoft/autogen/blob/004749e6b7102c8fd317b9b1499ce5807223c278/python/packages/autogen-studio/autogenstudio/database/db_manager.py#L253-L263),
[db.py:65](https://github.com/microsoft/autogen/blob/004749e6b7102c8fd317b9b1499ce5807223c278/python/packages/autogen-studio/autogenstudio/datamodel/db.py#L65),
[96](https://github.com/microsoft/autogen/blob/004749e6b7102c8fd317b9b1499ce5807223c278/python/packages/autogen-studio/autogenstudio/datamodel/db.py#L96)).
No other file keeps a copy.

### Credentials

- `.autogenstudio/.env`: secret glob.
- `.autogenstudio/temp_env_vars.env`: secret glob; its database URI can
  embed a Postgres password
  ([README.md:133](https://github.com/microsoft/autogen/blob/027ecf0a379bcc1d09956d46d12d44a3ad9cee14/python/packages/autogen-studio/README.md#L133)).
- The `settings` table in `autogen04203.db` holds environment variables
  typed `secret` and the default model client config with its `api_key`
  ([types.py:88-109](https://github.com/microsoft/autogen/blob/027ecf0a379bcc1d09956d46d12d44a3ad9cee14/python/packages/autogen-studio/autogenstudio/datamodel/types.py#L88-L109)).
  The database also holds the run history, so it is collected unflagged,
  as the catalog does for Cursor's `state.vscdb`; redact the `settings`
  table before sharing.

### Exclusions

None. The directory is small.

### Project-local files

None written by Studio itself; the gallery code executor's `.coding` directory is in
[Scratch directories](#scratch-directories).

### Where the project path is recorded

Nowhere. Studio has no project concept; teams and sessions are rows in
`autogen04203.db`.

## Parsing

AutoGen Studio is `backlog`: its parser is
[issue 69](https://github.com/seanthegeek/doubleagent/issues/69), and nothing it writes is parsed
yet. A parser would read the SQLite `autogen04203.db`: the `message` rows of each run (the
message types under [Records](#records)), joined through the `session` and `run` tables, with
the `LLMCallEventMessage` rows for the model ([Model](#model)). `run.messages` and
`run.team_result` repeat the `message` rows inside the same file, so there is no secondary
source, and the `settings` table holds credentials ([Credentials](#credentials)). The store is
SQLite, so there is no line-oriented file.

Studio records no permission state: the gallery's code executor runs without an approval function
([builder.py:433-434](https://github.com/microsoft/autogen/blob/004749e6b7102c8fd317b9b1499ce5807223c278/python/packages/autogen-studio/autogenstudio/gallery/builder.py#L433-L434));
`approval_func` exists only on AgentChat's `CodeExecutorAgent`
([_code_executor_agent.py:140](https://github.com/microsoft/autogen/blob/004749e6b7102c8fd317b9b1499ce5807223c278/python/packages/autogen-agentchat/src/autogen_agentchat/agents/_code_executor_agent.py#L140)),
and is not found in `autogen-studio` at
[`004749e`](https://github.com/microsoft/autogen/commit/004749e6b7102c8fd317b9b1499ce5807223c278)
(grepped for `approv` and `permission`: only the web auth error class). A human's answer to a
`UserProxyAgent` arrives over the web socket and is stored as an ordinary message
([connection.py:201-224](https://github.com/microsoft/autogen/blob/004749e6b7102c8fd317b9b1499ce5807223c278/python/packages/autogen-studio/autogenstudio/web/managers/connection.py#L201-L224)).
It records no sync, relay or origin state either (grepped for `upload`, `sync` and `share` in
`autogen-studio` at the same commit).

## Confidence

- High: the app directory, the database name and tables, and the credential locations (source).
- Not determined: whether older Studio releases used another database name (the README mentions
  `database.sqlite`).
