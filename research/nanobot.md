# nanobot

nanobot (catalog agent `nanobot`) is a small personal-assistant agent with a CLI, a WebUI and a
gateway that connects chat channels (Telegram, Discord, Feishu, WhatsApp, Matrix, QQ, WeChat and
others under `nanobot/channels/`); it is open source, written in Python, and not a fork of another
agent. It stores each conversation as a JSONL file of OpenAI chat-completions messages behind one
metadata line, rewritten whole on every save, and at the 2026-10-19 commit in a SQLite session
database ([Session database](#session-database)). Parsed: the nanobot parser reads the session
database, the session JSONL files, the memory journal, the WebUI transcript, the CLI history, the
cron and trigger run records, the WebUI sidebar cache, and the permission settings of `config.json`
and `pairing.json`.

## Source and evidence

[HKUDS/nanobot](https://github.com/HKUDS/nanobot), commit
[`acdae3d0ae2714b6dde672428e921dbf705c096f`](https://github.com/HKUDS/nanobot/commit/acdae3d0ae2714b6dde672428e921dbf705c096f)
(2026-10-04, PyPI package `nanobot-ai` version `0.3.5`,
[`pyproject.toml:2-3`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/pyproject.toml#L2-L3)),
MIT
([`LICENSE`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/LICENSE)),
open source, Python. Every claim is source-level unless it cites `docs/` in the same checkout.

The session database and the other changes of 2026-10-19 are cited at [`909b277`](https://github.com/HKUDS/nanobot/commit/909b277e7a5a7ecd2aebdba9611e5febe86988d9). Its `pyproject.toml`
still reads `0.3.5`, while the import module names 0.3.6 as the version that introduces the
database
([`session/jsonl_migration.py:1-6`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/session/jsonl_migration.py#L1-L6)),
so the database may not be in a release yet.

### Observed

- No real install. The session database `sessions/<workspace-id>/sessions.sqlite3`, the frozen
  namespace JSONL and `jsonl-backup/` copies beside it, and `history/cli_history` under any
  `.nanobot*` state directory are not observed on any real install, source-derived
  ([SQLite session store](#sqlite-session-store), [Transcript stores](#transcript-stores)).

### Source reviews

- 2026-10-21: HKUDS/nanobot at
  [`909b277`](https://github.com/HKUDS/nanobot/commit/909b277e7a5a7ecd2aebdba9611e5febe86988d9)
  (0.3.5 in `pyproject.toml`), and the `oauth-cli-kit` 0.1.6 wheel it depends on: the relay,
  permission, model switch, run record and sidebar cache plans of [Parser plan](#parser-plan) are
  now in the parser, the run record and sidebar cache shapes are in [Other stores](#other-stores),
  and the GitHub Copilot sign-in is in [Credentials](#credentials) and
  [Source host rows](#source-host-rows).
- 2026-10-19: [HKUDS/nanobot](https://github.com/HKUDS/nanobot) at [`909b277`](https://github.com/HKUDS/nanobot/commit/909b277e7a5a7ecd2aebdba9611e5febe86988d9) against [`acdae3d`](https://github.com/HKUDS/nanobot/commit/acdae3d0ae2714b6dde672428e921dbf705c096f). Source
  only. The review added the SQLite session store and its JSONL import
  ([SQLite session store](#sqlite-session-store), [Session database](#session-database)), the
  `--home` option, the cron and trigger run records and the WebUI sidebar cache
  ([Other stores](#other-stores)), the `pairing.json` shape, the SQLite delete behaviour in
  [What survives a delete](#what-survives-a-delete), and the permission, model and relay plans of
  [Parser plan](#parser-plan).

## Collection

### Per-user storage

All paths derive from `Path.home()`, so the layout is `~/.nanobot` on Linux and macOS and
`%USERPROFILE%\.nanobot` on Windows; there is no XDG, Library or AppData branch. The config file is
`~/.nanobot/config.json` unless `--config` selects another
([`nanobot/config/loader.py:20-39`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/config/loader.py#L20-L39)),
and the instance data directory is the config file's parent
([`nanobot/config/paths.py:20-27`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/config/paths.py#L20-L27)).
The documented multi-instance pattern is `~/.nanobot-<name>/config.json` with its own workspace
([`docs/multiple-instances.md:13-20`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/docs/multiple-instances.md#L13-L20)).
Settings can also come from `NANOBOT_*` environment variables
([`config/schema.py:665-668`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/config/schema.py#L665-L668)).
Since [`909b277`](https://github.com/HKUDS/nanobot/commit/909b277e7a5a7ecd2aebdba9611e5febe86988d9) a global `--home <dir>` option sets the instance root; without it the root is
`~/.nanobot`, and `history/cli_history`, the legacy sessions directory and the default workspace
follow the chosen root
([`config/home.py:19-28`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/config/home.py#L19-L28);
[`config/paths.py:52-72`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/config/paths.py#L52-L72);
[`cli/commands.py:115`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/cli/commands.py#L115)).
The documented form is `--home ~/.nanobot-<name>`
([`docs/multiple-instances.md:3`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/docs/multiple-instances.md#L3),[`12`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/docs/multiple-instances.md#L12)),
which the `.nanobot*` entry covers; any other directory is missed.

Under the data directory:

- `config.json`: providers, channels, MCP servers, agent defaults ([Credentials](#credentials)).
- `sessions/<workspace-id>/`: one directory per workspace, named by a 32-hex id, holding a
  `.workspace` marker file with the workspace path, `<base64url(session key)>.jsonl` transcripts,
  `<...>.checkpoint.json` runtime overlays, `.session-files.lock` and `.migration-conflicts/`
  ([`nanobot/session/manager.py:58-76`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/session/manager.py#L58-L76),[`448-477`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/session/manager.py#L448-L477),[`606-607`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/session/manager.py#L606-L607),[`777-779`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/session/manager.py#L777-L779),[`903-927`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/session/manager.py#L903-L927);
  naming in [Transcript stores](#transcript-stores)). Since [`909b277`](https://github.com/HKUDS/nanobot/commit/909b277e7a5a7ecd2aebdba9611e5febe86988d9) the directory also holds the
  session database `sessions.sqlite3` with its `-wal` and `-shm` sidecars and the `jsonl-backup/`
  copies ([SQLite session store](#sqlite-session-store)) and the WebUI sidebar cache
  `.webui_session_index.json` ([WebUI sidebar cache](#webui-sidebar-cache)), and the `sessions/`
  root holds `.workspace-migration.lock`
  ([`session/location.py:45-48`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/session/location.py#L45-L48)).
- `webui/`: WebUI display transcripts `<key>.jsonl`, `<key>.segments/` with `manifest.json`, legacy
  `<key>.json`, plus `workspace-state.json`, `sidebar-state.json`
  ([`nanobot/webui/transcript.py:172-188`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/webui/transcript.py#L172-L188);
  [`webui/workspaces.py:45`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/webui/workspaces.py#L45);
  [`webui/sidebar_state.py:37`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/webui/sidebar_state.py#L37)).
- `cron/jobs.json` (legacy location,
  [`nanobot/cli/runtime_config.py:195-196`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/cli/runtime_config.py#L195-L196)),
  `logs/`, `media/<channel>/` received attachments
  ([`config/paths.py:30-48`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/config/paths.py#L30-L48)).
- `llm_usage.sqlite3`: provider, model and time of every model call ([Usage database](#usage-database)).
- `pairing.json`: approved chat senders
  ([`nanobot/pairing/store.py:3`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/pairing/store.py#L3),[`33`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/pairing/store.py#L33)).
- `auth/xai.json`, `auth/mcp.json` ([Credentials](#credentials)), `whatsapp-auth/neonize.db`,
  `matrix-store/`, `weixin/`, `mochat/`, `linear/state.sqlite3` channel state
  ([`channels/whatsapp/runtime.py:75`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/channels/whatsapp/runtime.py#L75);
  [`channels/matrix/runtime.py:354`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/channels/matrix/runtime.py#L354);
  [`channels/weixin/runtime.py:378`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/channels/weixin/runtime.py#L378);
  [`channels/mochat/runtime.py:283`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/channels/mochat/runtime.py#L283);
  [`channels/linear/state.py:44`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/channels/linear/state.py#L44)).
- `mcp/<name>/` working directories for MCP presets
  ([`webui/mcp_presets_api.py:563`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/webui/mcp_presets_api.py#L563)),
  `cli-apps/`
  ([`apps/cli/service.py:428`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/apps/cli/service.py#L428)).
- `cache/tiktoken/`, `bin/tui/<version>/`, `run/executables/` ([Exclusions](#exclusions)).

Always under `~/.nanobot` regardless of `--config` (under the `--home` root when it is given):
`history/cli_history` (prompt_toolkit input
history), legacy `sessions/<safe key>.jsonl`, the default `workspace/`, and launchd logs
`logs/<name>.launchd.log`
([`config/paths.py:51-71`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/config/paths.py#L51-L71);
[`gateway/service.py:135-139`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/gateway/service.py#L135-L139)).

Outside the instance root, the two OAuth model providers keep their tokens through the
`oauth-cli-kit` library (a PyPI package with no linked repository; read in its 0.1.6 wheel, sha256
`d4a73beae7333ae642a9e5766ab0a04de53279667fefdf064c23db77a3494e7f`), whose `FileTokenStorage`
writes `<user_data_dir(app_name)>/auth/<token_filename>` unless `OAUTH_CLI_KIT_TOKEN_PATH` names
another file (`oauth_cli_kit/storage.py:29-38`, default `app_name` `oauth-cli-kit` at `:111`).
`user_data_dir` is the platformdirs data directory, `~/.local/share/<app>` (or `$XDG_DATA_HOME`)
on Linux, `~/Library/Application Support/<app>` on macOS and `AppData/Local/<app>` on Windows
([`unix.py:37-45`](https://github.com/tox-dev/platformdirs/blob/bc0405cb9c9439e6923b2dc090f91ad5daaf7dec/src/platformdirs/unix.py#L37-L45);
[`macos.py:28-30`](https://github.com/tox-dev/platformdirs/blob/bc0405cb9c9439e6923b2dc090f91ad5daaf7dec/src/platformdirs/macos.py#L28-L30);
[`windows.py:28-36`](https://github.com/tox-dev/platformdirs/blob/bc0405cb9c9439e6923b2dc090f91ad5daaf7dec/src/platformdirs/windows.py#L28-L36),
platformdirs 4.3.6). The GitHub Copilot provider uses `app_name` `nanobot` and the file
`github-copilot.json`, so `~/.local/share/nanobot/auth/github-copilot.json` on Linux
([`providers/github_copilot_provider.py:38`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/providers/github_copilot_provider.py#L38),[`53-58`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/providers/github_copilot_provider.py#L53-L58));
the OpenAI Codex provider keeps the default `app_name`, so its token is under
`oauth-cli-kit/auth/`
([`providers/openai_codex_provider.py:769`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/providers/openai_codex_provider.py#L769)).
The catalog has no line for either directory, so neither token is collected
([Credentials](#credentials)).

#### Scratch directories

The default workspace (`~/.nanobot/workspace`, or `<home>/workspace` under `--home`,
[`config/home.py:24-28`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/config/home.py#L24-L28)),
the compose file's `api-workspace` ([Docker](#docker)), the MCP preset directories `mcp/<name>/` and
`cli-apps/` are the agent's own working directories, not user projects. All are under
`.nanobot*`, so the catalog collects them and the analyzer's catalog rule classes them as
scratch directories. No temp root
outside the home holds session state: the only `tempfile` directories are transient staging for
skill deletes and marketplace installs and an SSH control-socket directory (`nb-ssh-*`,
[`webui/remote_mux.py:29`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/webui/remote_mux.py#L29)).

#### Workspace

The **workspace** (default `~/.nanobot/workspace`,
[`config/schema.py:117-120`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/config/schema.py#L117-L120))
holds the agent's instructions and memory: `AGENTS.md`, `SOUL.md`, `USER.md`
([`agent/context.py:92`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/agent/context.py#L92),[`194-202`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/agent/context.py#L194-L202)),
`HEARTBEAT.md` (periodic tasks,
[`cli/gateway_runtime.py:629`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/cli/gateway_runtime.py#L629)),
`memory/MEMORY.md`, `memory/history.jsonl`, legacy `memory/HISTORY.md`, cursors `memory/.cursor` and
`.dream_cursor`
([`agent/memory.py:76-91`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/agent/memory.py#L76-L91)),
a dulwich git repository versioning those files
([`agent/memory.py:89-91`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/agent/memory.py#L89-L91);
[`utils/gitstore.py:56`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/utils/gitstore.py#L56)),
`skills/`
([`agent/skills.py:61`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/agent/skills.py#L61)),
`cron/jobs.json`
([`cli/gateway_runtime.py:469`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/cli/gateway_runtime.py#L469))
with the run records `cron/runs/<run id>.json` and the queue `cron/action.jsonl`
([Cron run records](#cron-run-records)), `triggers/` with `triggers.json`, `inbox/`,
`processing/`, `failed/` and `runs/` ([Trigger run records](#trigger-run-records)),
`.nanobot/workspace-id`
([`session/manager.py:529-534`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/session/manager.py#L529-L534))
and `.nanobot/tool-results/` spilled tool output
([`utils/helpers.py:366-368`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/utils/helpers.py#L366-L368),[`639`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/utils/helpers.py#L639)).
Older releases kept `sessions/*.jsonl` inside the workspace; they are copied to the data directory
and removed
([`session/manager.py:806-855`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/session/manager.py#L806-L855)).
A workspace other than the default is a project path
([Where the project path is recorded](#where-the-project-path-is-recorded)), collected only as a
project.

#### Persistence

Persistence: systemd user unit `~/.config/systemd/user/nanobot-gateway.service` or
`~/Library/LaunchAgents/ai.nanobot.gateway.plist`
([`gateway/service.py:25`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/gateway/service.py#L25),[`86-87`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/gateway/service.py#L86-L87),[`135-136`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/gateway/service.py#L135-L136),[`227-236`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/gateway/service.py#L227-L236)).
No Windows service.

#### Docker

The compose file bind-mounts the host's `~/.nanobot` at `/home/nanobot/.nanobot` rather than using
a named volume, and the API service's workspace is `~/.nanobot/api-workspace`
([`docker-compose.yml:13-14`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/docker-compose.yml#L13-L14),[`48-49`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/docker-compose.yml#L48-L49)),
so the home entry collects both and no `DOCKER_VOLUMES` line is needed.

### Credentials

- `config.json`: `providers.<name>.apiKey`
  ([`config/schema.py:194-202`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/config/schema.py#L194-L202)),
  the API server `apiKey`
  ([`:340`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/config/schema.py#L340)),
  MCP server `env`
  ([`:370`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/config/schema.py#L370)),
  among them the GitHub personal access token of the GitHub MCP preset
  ([Source host rows](#source-host-rows)),
  and channel secrets such as Feishu `appSecret` and `verificationToken`
  ([`channels/feishu/config.py:18-20`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/channels/feishu/config.py#L18-L20))
  or Linear `clientSecret`
  ([`channels/linear/config.py:22-23`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/channels/linear/config.py#L22-L23)).
  Keys are camelCase via `to_camel`
  ([`config_base.py:9-15`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/config_base.py#L9-L15)).
  `${VAR}` references are resolved from the environment
  ([`config/loader.py:196`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/config/loader.py#L196)).
- `auth/xai.json` (xAI OAuth,
  [`providers/xai_oauth.py:262`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/providers/xai_oauth.py#L262))
  and `auth/mcp.json` (MCP OAuth,
  [`agent/tools/mcp_oauth.py:102`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/agent/tools/mcp_oauth.py#L102)).
- `whatsapp-auth/neonize.db` is a linked-device WhatsApp session; `matrix-store/`
  holds Matrix session and encryption state.
- `webui/remote-pairing/<uuid>/identity` (an unencrypted OpenSSH private
  key) and `exchange` (a raw X25519 private key), the material of the
  remote WebUI pairing link; secret globs
  `.nanobot*/webui/remote-pairing/*/identity` and
  `.nanobot*/webui/remote-pairing/*/exchange` from collector 1.14.0. The
  `request`, `receipt` and `ssh_route` files beside them and
  `webui/remote-instances.json` stay unflagged.
  Evidence in
  [Secrets](#secrets).
- No keychain use found.
- `github-copilot.json`, the GitHub OAuth token of the GitHub Copilot model provider (scope
  `read:user`), written by `nanobot provider login github-copilot` as `{access, refresh, expires,
  account_id}` with mode 0600, `account_id` being the GitHub `login` of the signed-in account
  ([`providers/github_copilot_provider.py:37`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/providers/github_copilot_provider.py#L37),[`193-204`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/providers/github_copilot_provider.py#L193-L204);
  `oauth_cli_kit/storage.py:56-70` in the wheel named in
  [Per-user storage](#per-user-storage)), and the OpenAI Codex provider's token beside the
  library's own data directory. Both live outside `.nanobot*`, where that section places them, and
  are not collected.
- `pairing.json`, the approved chat senders, is at [`909b277`](https://github.com/HKUDS/nanobot/commit/909b277e7a5a7ecd2aebdba9611e5febe86988d9)
  `{"approved": {<channel>: [<sender_id>...]}, "pending": {<code>: {channel, sender_id, created_at, expires_at, delivery_status}}}`,
  the code an `ABCD-EFGH` value that lives ten minutes and only lets an operator approve that sender
  ([`pairing/store.py:27-33`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/pairing/store.py#L27-L33),[`73-85`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/pairing/store.py#L73-L85),[`130-138`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/pairing/store.py#L130-L138)).
  It is permission evidence, not a credential, and stays unflagged.

### Exclusions

`bin/tui` (downloaded TUI binaries,
[`cli/tui_launcher.py:261`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/cli/tui_launcher.py#L261)),
`run/executables`
([`:248`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/cli/tui_launcher.py#L248)),
`cache/tiktoken`
([`utils/token_encoding.py:20`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/utils/token_encoding.py#L20)).
Nothing else is large by design; `media/` can grow but is evidence.

### Project-local files

A user can point `--workspace` at a repository, which then receives the workspace files of
[Per-user storage](#per-user-storage), notably `.nanobot/` and `memory/`. `AGENTS.md` is read from the
active workspace, `SOUL.md` and `USER.md` from the configured one
([`agent/context.py:194-202`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/agent/context.py#L194-L202)).
No per-project database.

### Where the project path is recorded

- `sessions/<workspace-id>/.workspace`: absolute workspace path, one line
  ([`session/manager.py:606-607`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/session/manager.py#L606-L607)).
- `config.json` `agents.defaults.workspace`
  ([`config/schema.py:117-120`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/config/schema.py#L117-L120),[`188-191`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/config/schema.py#L188-L191),[`428`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/config/schema.py#L428)).
- `webui/workspace-state.json` (WebUI workspace list; keys not traced).
- Per session, `metadata.workspace_scope.project_path` (with `access_mode` `restricted` or `full`):
  the project folder a WebUI chat was pointed at, which can differ from the workspace in
  `.workspace`
  ([`security/workspace_access.py:80-84`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/security/workspace_access.py#L80-L84);
  [`webui/inbound_commands.py:694`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/webui/inbound_commands.py#L694)).
  Each WebUI message copies its scope into the session metadata, so the key holds the last one
  ([`security/workspace_access.py:145-154`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/security/workspace_access.py#L145-L154)).
  Since [`909b277`](https://github.com/HKUDS/nanobot/commit/909b277e7a5a7ecd2aebdba9611e5febe86988d9) it sits in the `metadata` column of `sessions.sqlite3`, so
  `discover_projects` cannot read it; the analyzer can.

## Parsing

### Transcript stores

Home-relative; `.nanobot` may be `.nanobot-<instance>` when `--config` points elsewhere
([`nanobot/config/paths.py:20-27`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/config/paths.py#L20-L27)).

- `.nanobot/sessions/<workspace-id>/<stem>.jsonl`: one file per session key. `<workspace-id>` is 32
  hex characters; `<stem>` is the URL-safe base64 of the session key without padding
  ([`nanobot/session/manager.py:74`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/session/manager.py#L74),[`903-924`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/session/manager.py#L903-L924)),
  so `cli:direct` is `Y2xpOmRpcmVjdA.jsonl`. The sibling `.workspace` file gives the workspace path
  ([`:606-607`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/session/manager.py#L606-L607)).
- Sidecar `<stem>.checkpoint.json`: volatile mid-turn checkpoint, deleted on the next full save
  ([`:60`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/session/manager.py#L60),[`926-927`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/session/manager.py#L926-L927),[`1233-1235`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/session/manager.py#L1233-L1235)).
  Not a transcript.
- Legacy: `.nanobot/sessions/<key with ":" → "_">.jsonl` (global, before per-workspace namespaces)
  and `<workspace>/sessions/*.jsonl` (older still), both migrated into the namespaced directory
  ([`config/paths.py:69-71`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/config/paths.py#L69-L71);
  [`session/manager.py:806-855`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/session/manager.py#L806-L855),[`929-933`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/session/manager.py#L929-L933)).
  Same record format. Conflicting copies are kept in `.migration-conflicts/`
  ([`:777-779`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/session/manager.py#L777-L779)).
- `<workspace>/memory/history.jsonl`: compressed long-term journal, lines
  `{cursor, timestamp: "YYYY-MM-DD HH:MM", content, session_key?}`
  ([`agent/memory.py:311-322`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/agent/memory.py#L311-L322));
  legacy `memory/HISTORY.md`.
- `.nanobot/webui/<safe key>.jsonl` and `<safe key>.segments/NNNNNN.jsonl`: WebUI display
  transcript, schema version 3, a rendering mirror of the session
  ([`webui/transcript.py:1`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/webui/transcript.py#L1),[`31-39`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/webui/transcript.py#L31-L39),[`172-188`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/webui/transcript.py#L172-L188)).
  The directory is the instance's data directory, so `.nanobot-<instance>/webui/` for an instance
  ([`config/paths.py:20-27`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/config/paths.py#L20-L27),[`46-48`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/config/paths.py#L46-L48)).
  Read as a secondary source; record shape in [Record schema](#record-schema).
  The legacy `<safe key>.json` snapshot beside it
  ([`webui/transcript.py:186-188`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/webui/transcript.py#L186-L188))
  is not read: its shape was not traced.
- `.nanobot/history/cli_history`: prompt_toolkit input history, user prompts only
  ([`config/paths.py:64-66`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/config/paths.py#L64-L66)),
  always under `~/.nanobot` whatever instance runs unless `--home` moves it with the instance root
  ([Per-user storage](#per-user-storage)), so an instance run as `--home ~/.nanobot-work` writes
  `.nanobot-work/history/cli_history`. Read as a secondary source; the parser selects
  `history/cli_history` under any `.nanobot*` state directory
  ([issue 179](https://github.com/seanthegeek/doubleagent/issues/179)).

#### What survives a delete

In the JSONL layout of the
[`acdae3d`](https://github.com/HKUDS/nanobot/commit/acdae3d0ae2714b6dde672428e921dbf705c096f)
commit, `/new` empties the session file and leaves the WebUI transcript
([`command/builtin.py:314-346`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/command/builtin.py#L314-L346)),
so the WebUI turns before it survive only there and in the memory journal's summary.
`Session.clear()` in `session/manager.py` empties the messages and the next save rewrites the file
([`:374-380`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/session/manager.py#L374-L380)),
so a `/new` in a chat erases the previous transcript; only `memory/history.jsonl` and the WebUI
mirror keep a trace. Deleting a WebUI chat deletes the session file and the transcript together
([`webui/ws_http.py:1250-1251`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/webui/ws_http.py#L1250-L1251);
[`webui/thread_disk.py:19-31`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/webui/thread_disk.py#L19-L31)).
The CLI history survives `/new` and any session deletion; nothing in the traced source truncates it.

In the SQLite layout of the 2026-10-19 commit, deleting a WebUI chat removes the session's rows and
its child sessions' rows from `sessions.sqlite3`, its WebUI transcript and, when the user confirms,
its scheduled jobs
([`webui/ws_http.py:1363-1378`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/webui/ws_http.py#L1363-L1378);
[`session/manager.py:555-574`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/session/manager.py#L555-L574));
`/new` removes the session's messages and leaves the WebUI transcript. Writes (in
`session/sqlite_store.py`) upsert each changed `messages` row and delete rows past the new length,
so a `/new` or a rewind removes rows rather than rewriting a file
([`:281-335`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/session/sqlite_store.py#L281-L335)).
Deleting a session is one `DELETE FROM sessions`, and the foreign keys cascade to its messages,
checkpoint and pending inputs
([`:53-66`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/session/sqlite_store.py#L53-L66),[`394-396`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/session/sqlite_store.py#L394-L396)).
No `VACUUM`, `auto_vacuum` or `secure_delete` is set (not found in `nanobot/session` or
`nanobot/llm_usage` at `909b277`; grepped for `VACUUM`, `auto_vacuum` and `secure_delete`), so a
deleted session's pages stay in the file's free list or the WAL until reused. What remains: the
memory journal `memory/history.jsonl` (summaries, not copies of turns); the CLI history
`history/cli_history` (each prompt with its time, no session; nothing truncates it); any frozen
namespace JSONL and `jsonl-backup/` copy from the database import; the WebUI sidebar cache
`.webui_session_index.json` until it is rebuilt; `llm_usage.sqlite3` (model and time per call, no
session key); and the run records of scheduled and triggered runs, `cron/runs/<run id>.json` and
`triggers/runs/<delivery id>.json`, each with the prompt and the reply
([`triggers/local_store.py:45-50`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/triggers/local_store.py#L45-L50)).
No deletion marker is written. The analyzer reads the frozen and backup JSONL copies, the WebUI
transcript, the CLI history, the sidebar cache and the cron and trigger run records as secondary
sources and the memory journal as a primary source of summaries
([Past use and secondary sources](#past-use-rows-and-secondary-sources)), and does not read
`llm_usage.sqlite3` (see [Secondary sources](../analyzer/docs/output.md#secondary-sources)). The
run records of a workspace outside `.nanobot*` are collected through the project catalog.

#### SQLite session store

Since [`909b277`](https://github.com/HKUDS/nanobot/commit/909b277e7a5a7ecd2aebdba9611e5febe86988d9),
parsed ([issue 178](https://github.com/seanthegeek/doubleagent/issues/178)):

- `.nanobot/sessions/<workspace-id>/sessions.sqlite3`, with its `-wal` and `-shm` sidecars: the
  session store of record, mode 0600
  ([`session/location.py:31-35`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/session/location.py#L31-L35),[`56`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/session/location.py#L56);
  [`session/sqlite_store.py:95-96`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/session/sqlite_store.py#L95-L96);
  [`session/manager.py:422-437`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/session/manager.py#L422-L437)).
  Primary. Schema in [Session database](#session-database). On first start, every
  `sessions/<workspace-id>/*.jsonl` and `<workspace>/sessions/*.jsonl` is imported in one
  transaction
  ([`session/sqlite_store.py:157-200`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/session/sqlite_store.py#L157-L200),
  [`185-200`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/session/sqlite_store.py#L185-L200);
  [`session/jsonl_migration.py:155-197`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/session/jsonl_migration.py#L155-L197)).
  The namespace's own JSONL files are left where they were and are never written again, so after the
  import they are frozen copies of the sessions as they stood that day; every later turn is only in
  the database. The global pre-namespace `.nanobot/sessions/*.jsonl` files and
  `.migration-conflicts/` are no longer read by nanobot at all (`get_legacy_sessions_dir` is only
  exported, and `migration-conflicts` is not found in nanobot at `909b277`; grepped for
  `get_legacy_sessions_dir` and `migration-conflicts`).
- `.nanobot/sessions/<workspace-id>/jsonl-backup/<name>.jsonl` and `<name>.checkpoint.json`: the
  older `<workspace>/sessions/` files, copied byte for byte with their modification time kept and
  then removed from the workspace
  ([`session/jsonl_migration.py:32`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/session/jsonl_migration.py#L32),[`221-232`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/session/jsonl_migration.py#L221-L232),[`235-291`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/session/jsonl_migration.py#L235-L291)).
  Same record format as the session JSONL; secondary, since every one was imported into the
  database. The file name is the older workspace form, the key with `:` as `_`, so the session id
  comes from its metadata line.

### Record schema

Line 1 is metadata
([`session/manager.py:1207-1220`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/session/manager.py#L1207-L1220)):
`{"_type":"metadata", key, created_at, updated_at, metadata, last_archived, last_consolidated}`. An
optional line 2 is `{"_type":"provider_state", state}`
([`:1221-1226`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/session/manager.py#L1221-L1226)).
Every other line is one message dict
([`:1227-1228`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/session/manager.py#L1227-L1228);
loader treats any line without a known `_type` as a message,
[`:961-987`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/session/manager.py#L961-L987)):

- `role`: `user`, `assistant`, `tool` (and `system` if a caller adds one).
- `content`: string, or a list of OpenAI content blocks (`{type:"text", text}`; inline images are
  replaced on save,
  [`agent/loop.py:2409-2427`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/agent/loop.py#L2409-L2427)).
- `timestamp`: `datetime.now().isoformat()`, **naive local time with no zone**, for example
  `2026-10-01T10:15:02.123456`
  ([`session/manager.py:208-217`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/session/manager.py#L208-L217);
  [`agent/loop.py:2430`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/agent/loop.py#L2430)).
  `created_at` and `updated_at` in the metadata line are the same form
  ([`:177-178`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/session/manager.py#L177-L178)).
- assistant:
  `tool_calls: [{id, type:"function", function:{name, arguments (JSON string)}, extra_content?, provider_specific_fields?}]`
  ([`providers/base.py:94-112`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/providers/base.py#L94-L112)),
  `reasoning_content` (thinking text), `thinking_blocks`
  ([`utils/helpers.py:753-771`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/utils/helpers.py#L753-L771)).
  Empty assistant messages are dropped
  ([`loop.py:2391-2392`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/agent/loop.py#L2391-L2392)).
- tool: `tool_call_id`, `name`, `content`
  ([`agent/runner.py:541-550`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/agent/runner.py#L541-L550)).
  The shell tool is named `exec`
  ([`agent/tools/shell.py:247-248`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/agent/tools/shell.py#L247-L248)).
- Flags: `_hidden_history: true` marks the synthetic user message that stands in for summarised
  history
  ([`session/history_visibility.py:10`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/session/history_visibility.py#L10);
  [`session/manager.py:219-238`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/session/manager.py#L219-L238));
  `_runtime_context` marks injected runtime context on a user message
  ([`runtime_context.py:14`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/runtime_context.py#L14);
  [`loop.py:2428-2429`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/agent/loop.py#L2428-L2429));
  `_command: true` marks the user message and the assistant reply of a slash command such as
  `/model`, which skip the model; the user message is the command as typed (`/model fast`) and the
  reply is the command's output
  ([`agent/loop.py:2143-2155`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/agent/loop.py#L2143-L2155),
  at [`909b277`](https://github.com/HKUDS/nanobot/commit/909b277e7a5a7ecd2aebdba9611e5febe86988d9)).
  A switch's reply is ``Switched model preset to `<preset>`.`` followed by `- Scope: current
  session`, ``- Model: `<model id>` ``, `- Context window: <n>` and `- Max output tokens: <n>`
  lines; a failed switch replies `Could not switch model preset: <reason>`, and `/model` alone
  replies `## Model` with ``- Current model: `<id>` `` and ``- Current preset: `<preset>` ``
  ([`command/builtin.py:384-448`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/command/builtin.py#L384-L448)).

No model is recorded per message. The session's model preset is `metadata._nanobot_model_preset`
([`session/model_selection.py:9`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/session/model_selection.py#L9));
the summary is `metadata._last_summary {text, last_active}` and the last delivery route
`metadata.last_channel`
([`session/keys.py:10`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/session/keys.py#L10),[`38`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/session/keys.py#L38)).

#### Session database

Since [`909b277`](https://github.com/HKUDS/nanobot/commit/909b277e7a5a7ecd2aebdba9611e5febe86988d9) the transcripts are in SQLite. `sessions/<workspace-id>/sessions.sqlite3`
is opened in WAL mode (the write-ahead log, a `-wal` sidecar that holds committed pages not yet
copied into the main file) with `PRAGMA user_version` 1
([`session/sqlite_store.py:44-73`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/session/sqlite_store.py#L44-L73),[`174`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/session/sqlite_store.py#L174)):

| Table | Columns | Holds |
| --- | --- | --- |
| `sessions` | `key` (primary), `created_at`, `updated_at`, `metadata` (JSON text), `provider_state`, `last_archived`, `revision`, `generation`, `title`, `preview`, `visible_updated_at` | the old metadata line, one row per session key; times are `datetime.isoformat()` of naive local time, as in the JSONL |
| `messages` | `session_key`, `position`, `record` (JSON text) | one message per row in order; `record` is exactly the old JSONL message line (`role`, `content`, `timestamp`, `tool_calls`, flags) |
| `checkpoints` | `session_key`, `record` | the mid-turn runtime checkpoint, formerly `<stem>.checkpoint.json` |
| `pending_inputs` | `session_key`, `kind`, `position`, `record` | follow-up messages queued while a turn runs (`kind` `pending_user_followups`) |
| `storage_meta` | `key`, `value` | `jsonl_import` = `complete` after the import, and the pending workspace-source list |

A child session (a subagent or a fork) names its parent in `metadata.parent_session_key`
([`session/types.py:8-9`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/session/types.py#L8-L9);
[`session/sqlite_store.py:404-413`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/session/sqlite_store.py#L404-L413)).

#### WebUI display transcript

The WebUI is nanobot's browser chat interface, served by its gateway over a WebSocket channel; its
chats are sessions with the key `websocket:<chat_id>`
([`webui/session_identity.py:8`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/webui/session_identity.py#L8),[`17-19`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/webui/session_identity.py#L17-L19)).
Every transcript append goes through `webui_session_key(chat_id)`
([`webui/transcript.py:1274-1281`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/webui/transcript.py#L1274-L1281)),
so only WebUI chats have one. The file stem is `SessionManager.safe_key`, the key with `:` replaced
by `_` and unsafe characters by `_`
([`session/manager.py:899-900`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/session/manager.py#L899-L900);
[`utils/helpers.py:372-374`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/utils/helpers.py#L372-L374)),
so `websocket:chat-1` is `websocket_chat-1.jsonl`, unlike the base64 stem of the session file
([`:903-904`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/session/manager.py#L903-L904)).

Layout. Records are appended to the active `<safe key>.jsonl`
([`webui/transcript.py:1075-1086`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/webui/transcript.py#L1075-L1086),[`1118-1127`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/webui/transcript.py#L1118-L1127)).
After a `turn_end`, when the file passes 2 MiB, the older turns move into the next numbered
`<safe key>.segments/NNNNNN.jsonl` and the active file is rewritten with the recent turns only
([`:35-36`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/webui/transcript.py#L35-L36),[`629-668`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/webui/transcript.py#L629-L668),[`671-711`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/webui/transcript.py#L671-L711));
`segments/manifest.json` indexes the segments. The files hold disjoint turns, not copies of each
other: nanobot reads the segments in order, then the active file
([`:1058-1065`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/webui/transcript.py#L1058-L1065)).

Records. One JSON object per line, `event` naming it, `chat_id`, the turn annotations `turn_id`,
`turn_phase` and `turn_seq`
([`:1289-1305`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/webui/transcript.py#L1289-L1305)),
and `created_at_ms`, set to `time.time()` in epoch milliseconds when the record is appended, unless
the caller gave one
([`:1089-1106`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/webui/transcript.py#L1089-L1106)).
This is true UTC, unlike every other nanobot time.

- `user`: `text`, optional `media_paths[]`, `cli_apps[]`, `mcp_presets[]`, `session_mentions[]`
  ([`:1494-1530`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/webui/transcript.py#L1494-L1530)),
  appended when the WebUI channel accepts the message, before the agent turn
  ([`webui/inbound_commands.py:658-669`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/webui/inbound_commands.py#L658-L669)).
- `message`: `text`, optional `kind` (`tool_hint`, `progress`, `reasoning`), `tool_events[]`,
  `media[]`, `latency_ms`
  ([`channels/websocket/runtime.py:1185-1227`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/channels/websocket/runtime.py#L1185-L1227);
  [`webui/transcript.py:2245-2278`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/webui/transcript.py#L2245-L2278)).
  No `kind` is an assistant reply; `tool_hint` and `progress` are breadcrumbs. A tool event is
  `{version, phase, call_id, name, arguments (an object), result, error, files, embeds}`, `phase`
  `start` when the call is made and `end` or `error` when it returns
  ([`utils/progress_events.py:91-102`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/utils/progress_events.py#L91-L102),[`116-143`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/utils/progress_events.py#L116-L143)).
- `stream_end` and `reasoning_end`: the completed text of a streamed answer or reasoning segment in
  `text`. A `stream_end` with `merge_next` keeps its buffer, so the next `stream_end` repeats its
  text
  ([`channels/websocket/runtime.py:1273-1297`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/channels/websocket/runtime.py#L1273-L1297),[`1338-1377`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/channels/websocket/runtime.py#L1338-L1377)).
  `delta` and `reasoning_delta` chunks are not written
  ([`webui/transcript.py:1229-1247`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/webui/transcript.py#L1229-L1247));
  older files may hold them, folded into the end record at `turn_end`
  ([`:325-362`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/webui/transcript.py#L325-L362)).
- `file_edit`: `edits[]` of
  `{version, call_id, tool, path, absolute_path, phase, added, deleted, approximate, status, binary?, operation?}`
  ([`channels/websocket/runtime.py:1304-1321`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/channels/websocket/runtime.py#L1304-L1321);
  [`utils/file_edit_events.py:531-558`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/utils/file_edit_events.py#L531-L558)).
- `turn_end` (`latency_ms`, `usage`, `round_usages`, `context_window_tokens`), `context_compaction`
  (`compaction_id`, `phase`), and `fork_marker`, the boundary where a forked chat starts
  ([`webui/transcript.py:2289-2326`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/webui/transcript.py#L2289-L2326),[`1361-1369`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/webui/transcript.py#L1361-L1369)).

A fork writes the target's transcript from the source's rows or from the truncated session messages,
`user` and `message` only, every record then stamped at the fork
([`:1319-1358`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/webui/transcript.py#L1319-L1358),[`1372-1407`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/webui/transcript.py#L1372-L1407)).

#### CLI input history

`nanobot agent` reads input through a prompt_toolkit `PromptSession` with a `FileHistory` on
`~/.nanobot/history/cli_history`
([`cli/terminal.py:86-95`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/cli/terminal.py#L86-L95),[`182-188`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/cli/terminal.py#L182-L188)).
The format is the one Aider's input history uses ([`aider.md`](aider.md)): an entry is a
`# <local timestamp>` line followed by one `+<line>` per input line. The timestamp is naive host
local time, the same convention as the session files. No session key or workspace is written, and
the file is shared by every CLI session.

#### Memory journal

`memory/history.jsonl` is not a copy of session events. An entry is a summary the model wrote of
archived messages
([`agent/memory.py:1006-1007`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/agent/memory.py#L1006-L1007))
or, when summarising fails, a `[RAW] <n> messages` block of several formatted messages
([`:627-647`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/agent/memory.py#L627-L647),[`649-672`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/agent/memory.py#L649-L672)),
stamped to the minute
([`:311-324`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/agent/memory.py#L311-L324)).

### Joins and timestamps

- Session: file = metadata `key`. Keys are `<channel>:<chat_id>`, or `unified:default`, `heartbeat`,
  `cli:direct`
  ([`session/keys.py:8-9`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/session/keys.py#L8-L9),[`23-27`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/session/keys.py#L23-L27);
  [`cli/agent.py:146`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/cli/agent.py#L146)).
  The channel and chat id identify the external sender.
- Project: `.workspace` in the same directory.
- Tool call to result: `tool_calls[].id` = `tool_call_id`.
- Model: by time, against `.nanobot/llm_usage.sqlite3` `llm_calls.started_at_ms` (UTC epoch ms,
  [`providers/base.py:1283`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/providers/base.py#L1283);
  [`llm_usage/store.py:196-218`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/llm_usage/store.py#L196-L218)),
  which also gives the host's clock offset.
- Subagents: not persisted as separate session files as far as traced.
- WebUI transcript to session file: session key `websocket:` + `chat_id` = the session file's
  metadata `key`; its file is `sessions/<workspace-id>/<base64 of the key>.jsonl`, whose
  `.workspace` gives the project. Tool calls join on `call_id` = `tool_calls[].id`. Text: the WebUI
  `user` text is the typed message, while the session's copy has any runtime context (a quoted
  reply, an automation template) appended
  ([`runtime_context.py:120-145`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/runtime_context.py#L120-L145);
  [`agent/loop.py:693-699`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/agent/loop.py#L693-L699)),
  so those two differ. Time: the session user message is stamped when it is persisted at the start
  of the turn
  ([`agent/loop.py:663-705`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/agent/loop.py#L663-L705);
  [`session/manager.py:208-217`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/session/manager.py#L208-L217)),
  its assistant and tool messages when the turn is saved at its end
  ([`agent/loop.py:2430`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/agent/loop.py#L2430)),
  all in host local time; the WebUI stamps each record in true UTC as it streams. The two differ by
  the host's UTC offset (at most 14 hours) plus up to the turn's length.
- CLI history to session file: text and time only; the CLI's session key is `cli:direct` unless
  `--session` names another
  ([`cli/agent.py:146`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/cli/agent.py#L146)),
  and the file records neither. The entry is written when the prompt is submitted and the session
  user message at the start of the turn, both in local time.
- Run records to session: `session_key` = the session's key. The session does not store the
  record's prompt but the text `Scheduled cron job triggered: <job name>`, a blank line and the
  job's message for a cron run
  ([`cron/bound_runner.py:81-95`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/cron/bound_runner.py#L81-L95);
  [`cron/session_turns.py:18-24`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/cron/session_turns.py#L18-L24)),
  and `Local trigger received: <trigger name, else id>`, a blank line and the delivery's content
  for a trigger run
  ([`triggers/local_runner.py:212-214`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/triggers/local_runner.py#L212-L214)),
  both rebuildable from the record's fields. The record's times are true UTC epoch milliseconds,
  the session's local time, so they differ by the host's UTC offset plus the turn's length.
- Time basis: message and metadata times are naive host local time
  ([Record schema](#record-schema)), the WebUI's `created_at_ms` true UTC; the parser's
  convention is under [Parser plan](#parser-plan).

### Other stores

The parser reads the WebUI sidebar cache and the cron and trigger run records, not the usage
database ([Past-use rows and secondary sources](#past-use-rows-and-secondary-sources)). The sidebar
cache and the run records are cited at [`909b277`](https://github.com/HKUDS/nanobot/commit/909b277e7a5a7ecd2aebdba9611e5febe86988d9).

#### Usage database

`llm_usage.sqlite3` (WAL,
[`nanobot/llm_usage/__init__.py:42`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/llm_usage/__init__.py#L42);
[`store.py:190-222`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/llm_usage/store.py#L190-L222))
holds only call metadata: table `llm_calls` has `started_at_ms` (UTC epoch milliseconds),
`provider`, `model` and `source` per call and no session key
([`llm_usage/store.py:196-218`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/llm_usage/store.py#L196-L218)
at [`909b277`](https://github.com/HKUDS/nanobot/commit/909b277e7a5a7ecd2aebdba9611e5febe86988d9)), so a call joins a session on time alone. It holds no transcript.

#### WebUI sidebar cache

`.nanobot/sessions/<workspace-id>/.webui_session_index.json`: a rebuildable cache of the WebUI
sidebar, `{"version": 8, "sessions": [...]}` with one row per WebUI transcript (title, preview,
model preset, workspace scope)
([`webui/session_list_index.py:1-6`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/webui/session_list_index.py#L1-L6),[`39-40`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/webui/session_list_index.py#L39-L40),[`172-204`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/webui/session_list_index.py#L172-L204)).
Secondary; it holds a preview, not turns, and a row can outlive the transcript until the cache is
rebuilt. Only the rows of WebUI transcripts that have no row in `sessions.sqlite3` are written to
the file
([`:79`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/webui/session_list_index.py#L79),[`198-205`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/webui/session_list_index.py#L198-L205)),
each `{key, created_at, updated_at, title, preview, model_preset, recovery_state,
_workspace_scope_present, _workspace_scope_value, _source: "webui_transcript", file, mtime_ns, size,
webui_activity_mtime_ns, webui_activity_size, webui_activity_files}`, `title` empty, `preview` the
first user text of the transcript, `created_at` the first record's `created_at_ms` and `updated_at`
the newest transcript file's modification time, both as naive local `isoformat()`
([`:402-413`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/webui/session_list_index.py#L402-L413),[`352-356`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/webui/session_list_index.py#L352-L356),[`416-497`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/webui/session_list_index.py#L416-L497)).
The WebUI directory is shared by the instance's workspaces, so a cached row names a chat, not the
workspace of the namespace directory it sits in.

#### Cron run records

`<workspace>/cron/runs/<run id>.json`: one audit record per scheduled run, `job_id`, `job_name`,
`session_key`, `rendered_prompt`, `status`, `response` or `error`, `run_id`, `updated_at_ms` (UTC
epoch milliseconds)
([`cron/service.py:207-208`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/cron/service.py#L207-L208),[`472-474`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/cron/service.py#L472-L474);
[`cron/bound_runner.py:98-146`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/cron/bound_runner.py#L98-L146);
[`utils/run_records.py:19-29`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/utils/run_records.py#L19-L29)),
beside `cron/jobs.json`, whose `state.runHistory[]` keeps `runAtMs`, `status`, `runId` and
`sessionKey`
([`cron/service.py:445-454`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/cron/service.py#L445-L454)),
and `cron/action.jsonl`, a queue of `{action, params}` lines
([`:673-674`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/cron/service.py#L673-L674)).
Secondary: the prompt and the reply are also turns of the bound session, and the run record survives
the session's deletion. The record is rewritten at each status, `queued` before the turn and `ok`
(with `response`) or `error` (with `error`) after it, each write setting `updated_at_ms` to the
time of that write; `prompt_vars` is `{message}`, the job's message; `run_id` is `<job id>:<epoch
milliseconds at the run's start>:<8 hex>`, and the file name is the run id with every character
other than letters, digits, `.`, `_` and `-` replaced by `_`
([`cron/bound_runner.py:81`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/cron/bound_runner.py#L81),[`98-146`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/cron/bound_runner.py#L98-L146);
[`utils/run_records.py:14-29`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/utils/run_records.py#L14-L29)).

#### Trigger run records

`<workspace>/triggers/`: local triggers (`triggers.json`, each bound to a `channel`, `chat_id` and
`session_key`), the delivery queue (`inbox/`, `processing/`, `failed/`, each delivery
`{id, triggerId, content, createdAtMs, attempts, lastError}`) and `runs/<delivery id>.json` audit
records with the content and the `response`
([`triggers/local_store.py:45-50`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/triggers/local_store.py#L45-L50),[`53-90`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/triggers/local_store.py#L53-L90),[`292-310`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/triggers/local_store.py#L292-L310);
[`triggers/local_types.py:121-158`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/triggers/local_types.py#L121-L158)).
Secondary, as for the cron runs; the directory exists at [`acdae3d`](https://github.com/HKUDS/nanobot/commit/acdae3d0ae2714b6dde672428e921dbf705c096f) as well.
A run record is `{kind: "local_trigger", trigger_id, delivery_id, content, created_at_ms,
attempts, last_error?, trigger_name, session_key, channel, chat_id, sender_id, origin_metadata,
status, error?, response?, run_id, updated_at_ms}`, the trigger's fields present only while the
trigger exists, `created_at_ms` the time the delivery was queued, and `content`, `error`,
`last_error` and `response` cut to 4,000 characters
([`triggers/local_store.py:24`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/triggers/local_store.py#L24),[`439-468`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/triggers/local_store.py#L439-L468)).
Its `status` is `processing` when the delivery reaches the agent, then `ok` with the `response`,
or `interrupted`, `retrying` or `error` with the `error`
([`triggers/local_runner.py:55-111`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/triggers/local_runner.py#L55-L111),[`145-166`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/triggers/local_runner.py#L145-L166)).

### Format versions

No version field in session files. The metadata line writes both `last_archived` and the older
`last_consolidated` for downgrade compatibility
([`session/manager.py:1215-1218`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/session/manager.py#L1215-L1218)).
Legacy file names differ ([Transcript stores](#transcript-stores)). At [`909b277`](https://github.com/HKUDS/nanobot/commit/909b277e7a5a7ecd2aebdba9611e5febe86988d9) the session store
moved from JSONL to SQLite ([Session database](#session-database)), `.migration-conflicts/` and the
import of the global pre-namespace `sessions/*.jsonl` are gone
([SQLite session store](#sqlite-session-store)), and the message, metadata and WebUI record shapes
are unchanged; what `Session.clear()` and a `/new` erase is under
[What survives a delete](#what-survives-a-delete).

### Secrets

`provider_state.state` is opaque provider conversation state. Tool `content`
holds raw tool output. `tool_calls[].function.arguments` can contain whatever
the agent passed to a shell. Credentials themselves are not in session files.

Remote WebUI pairing, nanobot's SSH-based link from a local WebUI to a nanobot gateway on another
host the user runs (no vendor relay,
[`webui/remote_pairing.py:1-5`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/webui/remote_pairing.py#L1-L5)),
keeps its material under the instance's `webui/` directory
([`webui/ws_http.py:392-394`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/webui/ws_http.py#L392-L394)):
`remote-pairing/<uuid>/identity` (an unencrypted OpenSSH private key), `exchange` (a raw X25519
private key), `request`, `receipt` and `ssh_route`
([`:208-210`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/webui/remote_pairing.py#L208-L210),[`249-251`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/webui/remote_pairing.py#L249-L251),[`290-299`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/webui/remote_pairing.py#L290-L299)).
`remote-instances.json` lists the saved remote servers by UUID with `name`, `host` (`user@host`),
`port`, `ssh_config`, `identity_file`, `config_path`, `runtime_user`, `local_port` and `pair_id`
([`webui/remote_instances.py:28-34`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/webui/remote_instances.py#L28-L34),[`52-53`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/webui/remote_instances.py#L52-L53),[`98-103`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/webui/remote_instances.py#L98-L103);
[`webui/remote_ssh.py:32-41`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/webui/remote_ssh.py#L32-L41)),
beside `remote-origin-ports.json` and per-server pinned host keys in `remote-hosts/<uuid>`
([`remote_instances.py:70-71`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/webui/remote_instances.py#L70-L71),[`105-125`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/webui/remote_instances.py#L105-L125)).
The catalog collects all of them under `.nanobot*`, and the two private keys are flagged by the
secret globs named in [Credentials](#credentials).

### Parser plan

Globs (home-relative): `.nanobot*/sessions/*/sessions.sqlite3`,
`.nanobot*/sessions/*/*.jsonl`, `.nanobot*/sessions/*.jsonl`,
`.nanobot*/sessions/*/.migration-conflicts/*.jsonl`,
`.nanobot*/sessions/*/jsonl-backup/*.jsonl`, `.nanobot*/sessions/*/.webui_session_index.json`,
`.nanobot*/config.json`, `.nanobot*/pairing.json`, `.nanobot*/**/cron/runs/*.json` and
`.nanobot*/**/triggers/runs/*.json`, and in a project base `cron/runs/*.json` and
`triggers/runs/*.json`. Skip `*.checkpoint.json`. Read `.workspace` from the namespace directory.

| Column | Source |
| --- | --- |
| timestamp_utc | message `timestamp` (naive local; see below) |
| session_id | metadata `key` |
| project_path | `.workspace` file content |
| git_branch | empty |
| turn_type | `user` → user (`_hidden_history` → system); `assistant` → assistant, plus tool_use per `tool_calls[]`, thinking from `reasoning_content`; `tool` → tool_result |
| model | empty, or `metadata._nanobot_model_preset`; in a session with a `/model` switch, empty before the first switch and the switched preset after each |
| tool_name | `tool_calls[].function.name` / tool `name` |
| tool_use_id | `tool_calls[].id` / `tool_call_id` |
| text | `content` text; for tool_use the `command`/`path`/`url` key of the parsed `arguments`, else the arguments string |
| source_line | line number |

Timestamps are host local time ([Joins and timestamps](#joins-and-timestamps)). The collector does
not record the host zone, so the parser follows the aider parser's documented convention (local time
emitted as if UTC, stated in its [subsection of the agents page](../docs/agents.md#aider)) and
lets the analyst correct it; `llm_usage.sqlite3` gives true UTC for the same calls to derive the
offset.

The primary store and the CLI history location are in the parser
([issue 178](https://github.com/seanthegeek/doubleagent/issues/178) and
[issue 179](https://github.com/seanthegeek/doubleagent/issues/179)), and so are the relay,
permission, model switch, run record and sidebar cache rows of the subsections below
([issue 207](https://github.com/seanthegeek/doubleagent/issues/207)).

#### Permission rows

Source-derived; not observed on any real install. From
[`909b277`](https://github.com/HKUDS/nanobot/commit/909b277e7a5a7ecd2aebdba9611e5febe86988d9)
([issue 207](https://github.com/seanthegeek/doubleagent/issues/207)), the parser yields one `system`
row per standing setting of `config.json` at the file's modification time, `permission setting:
<key>=<value> (from <path>, file mtime)`, `<path>` relative to the home (`.nanobot/config.json`),
for `tools.restrictToWorkspace`, `tools.exec.enable`, `tools.exec.sandbox`, each
`tools.exec.allowPatterns`, `denyPatterns`, `sandboxRoBinds`, `sandboxRwBinds` and `allowedEnvKeys`
entry, each `tools.ssrfWhitelist` entry, `tools.webuiAllowLocalServiceAccess` (or its legacy
`allowLocalPreviewAccess`), `tools.webuiAllowRemotePackageInstall` and each
`channels.<name>.allowFrom` entry, a key the file lacks giving no row. nanobot saves the file in
camelCase and accepts snake_case as well
([`config/loader.py:149`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/config/loader.py#L149);
[`config_base.py:12-15`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/config_base.py#L12-L15);
[`config/schema.py:440-449`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/config/schema.py#L440-L449)),
so the row names the key as the file spells it; no other key of the file, which holds API keys
([Credentials](#credentials)), is read
([`config/schema.py:428-458`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/config/schema.py#L428-L458);
[`agent/tools/shell_config.py:11-28`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/agent/tools/shell_config.py#L11-L28);
[`channels/base.py:244-257`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/channels/base.py#L244-L257));
one per `pairing.json` `approved.<channel>[]` sender at the file's modification time,
`permission setting: pairing.approved.<channel>=<sender_id> (from <path>, file mtime)`, and one per
`pending.<code>` entry at its `created_at` (epoch seconds, true UTC), `permission setting:
pairing.pending.<channel>=<sender_id> expires_at=<time> delivery_status=<status> (from <path>,
created_at)`, the delivery status left out when absent and the code itself never shown
([`pairing/store.py:1-5`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/pairing/store.py#L1-L5),[`32-33`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/pairing/store.py#L32-L33),[`73-85`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/pairing/store.py#L73-L85),[`133-138`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/pairing/store.py#L133-L138));
and after the session start row, at its time,
`permission mode: access_mode=<restricted|full> project=<project_path> (from session metadata
workspace_scope)` from `metadata.workspace_scope`, which holds the scope of the last message the
WebUI accepted
([`security/workspace_access.py:11-12`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/security/workspace_access.py#L11-L12),[`80-84`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/security/workspace_access.py#L80-L84),[`212-213`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/security/workspace_access.py#L212-L213);
[`webui/inbound_commands.py:694`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/webui/inbound_commands.py#L694)).
The key was already written at the [Source and evidence](#source-and-evidence) commit. Per-call
decisions: the agent records none. The exec tool's refusals are tool results such as
`Error: Command blocked by deny pattern filter`
([`agent/tools/shell.py:825-835`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/agent/tools/shell.py#L825-L835)),
already `tool_result` rows; no approval prompt exists (not found at `909b277`; grepped for `approv`
in `nanobot/agent` and for `ask_user`, `confirm` and `permission` there, which find only the
in-memory goal permission of `agent/goal_permission.py`).

#### Model rows

When the metadata line has ([issue 96](https://github.com/seanthegeek/doubleagent/issues/96) and
[issue 97](https://github.com/seanthegeek/doubleagent/issues/97)) `metadata._nanobot_model_preset`,
a `system` row `model: <preset> (preset)` follows the session start row at its time; the value
names a model preset of nanobot's configuration, not a model id, and the row says
so. A session has one metadata line, so it names one preset; changes come only from the switch
rows below. Per-call models are in `llm_usage.sqlite3` `llm_calls`, which the parser does not
read.

A `/model <preset>` switch is kept in the transcript as a user and assistant message pair
flagged `_command` (shapes in [Record schema](#record-schema)); the parser keeps both as plain rows
and, source-derived and not observed on any real install
([issue 207](https://github.com/seanthegeek/doubleagent/issues/207)), writes after each reply that
starts ``Switched model preset to `<preset>` `` a `system` row
`model: <old> -> <new> (preset) requested=<name> resolved=<model id>`, `<name>` from the preceding
`_command` user message `/model <name>` and `<model id>` from the reply's ``- Model: `<id>` ``
line ([`command/builtin.py:405-448`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/command/builtin.py#L405-L448)).
The metadata keeps only the last preset, so the first preset of a switched session is not recorded
anywhere: in a session with a switch the parser writes no `model: <preset> (preset)` row at the
start, the first switch reads `model: <new> (preset) requested=<name> resolved=<model id>`, and
the `model` column is empty until it. A failed switch (`Could not switch model preset:`) and the
`/model` status reply give no row.

#### Past-use rows and secondary sources

Sources, primary or secondary ([issue 122](https://github.com/seanthegeek/doubleagent/issues/122);
every one read on every run):

| File | Role | Rows |
| --- | --- | --- |
| `sessions/<workspace-id>/sessions.sqlite3` | primary ([issue 178](https://github.com/seanthegeek/doubleagent/issues/178)) | as above, one session per `sessions` row in `created_at` order, its `messages.record` values in `position` order, `source_line` the row's `rowid`; a value that is not a JSON object is a `parser: unparseable record <position> (<reason>): <record>` or `parser: unparseable session metadata (<reason>): <metadata>` row |
| `sessions/<workspace-id>/<stem>.jsonl` and `jsonl-backup/<name>.jsonl` beside a `sessions.sqlite3` | secondary, tolerance 2 seconds (the same `timestamp` strings) | as above |
| `sessions/<workspace-id>/<stem>.jsonl` with no database beside it, legacy and `.migration-conflicts/` copies | primary | as above |
| `<workspace>/memory/history.jsonl` | primary: summaries, not copies of events | one `system` row per entry, `memory history #<cursor>: <content>` |
| `webui/<safe key>.jsonl`, `webui/<safe key>.segments/NNNNNN.jsonl` | secondary, tolerance 15 hours | `user`; `assistant` (`message` with no `kind`, `stream_end` without `merge_next`); `tool_use` and `tool_result` per tool event; `thinking` (opt-in); `system` for breadcrumbs without tool events, finished file edits, `fork_marker`, `context_compaction` and unknown events; nothing for `delta`, `reasoning_delta`, `turn_end` |
| `.nanobot*/history/cli_history` | secondary, tolerance 2 seconds | one `user` row per entry at its own time, no session or project |
| `<workspace>/cron/runs/<run id>.json` | secondary, tolerance 15 hours (true UTC) | `user` of the text the session stores for the run (`Scheduled cron job triggered: <job_name>`, a blank line, `prompt_vars.message`; else `rendered_prompt`) at the start time inside `run_id`; `assistant` of `response` and `system` `cron run: job <job_name> (<job_id>) run <run_id> status=<status>`, with ` error=<error>`, at `updated_at_ms`; `session_id` `session_key`, `project_path` the workspace |
| `<workspace>/triggers/runs/<delivery id>.json` | secondary, tolerance 15 hours (true UTC) | `user` `Local trigger received: <trigger_name or trigger_id>`, a blank line, `content`, at `created_at_ms`; `assistant` of `response` and `system` `trigger run: trigger <trigger_name> (<trigger_id>) delivery <delivery_id> status=<status> attempts=<n>`, with ` error=<error>`, at `updated_at_ms` |
| `sessions/<workspace-id>/.webui_session_index.json` | secondary; `system` rows, which never pair | one `system` row per entry, `webui sidebar entry: key=<key> title=<title> preview=<preview> model_preset=<preset> updated_at=<updated_at> (from .webui_session_index.json version <n>)`, absent fields left out, at `created_at`, `session_id` the key, no project |

A frozen JSONL keeps the session start row of the import day, whose
`updated_at` differs from the database's, so both start rows are shown;
every message the database still holds is shown once, and a turn it lost
(a `/new`, a rewind, a deleted session) is recovered from the copy. The
WebUI rows' `project_path` comes from the session JSONL of the same key,
so a WebUI chat whose session exists only in the database has none.

The WebUI rows carry `created_at_ms` as true UTC, the session file's rows
local time as if UTC; the 15-hour window is the largest UTC offset (14
hours) plus an hour of turn, for the assistant and tool messages the
session stamps at the turn's end. Known misses, each shown twice: a turn
longer than that hour, a forked chat's transcript (every record stamped at
the fork), and a user message whose session copy carries appended runtime
context. A kept WebUI row keeps its true-UTC time.

Primary store (done): read `sessions/<workspace-id>/sessions.sqlite3` through
`sqlite_util.py` ([Session database](#session-database)): one session
per `sessions` row, `metadata` as the old metadata line, `messages`
ordered by `position` with `record` mapped as the JSONL message lines
are now, and each `record` that does not decode its own
`parser: unparseable record <position> (<reason>): <record>` row. The
namespace's `*.jsonl` files and `jsonl-backup/*.jsonl` then become
secondary (`secondary()` true, default 2-second window: the same
`timestamp` strings), since the import copied every one of them; the
parser makes them secondary only when the database is beside them,
since without it the JSONL is still the store of record. Messages
whose session row is gone still yield rows, with no start row.
`checkpoints` (the mid-turn checkpoint) and `pending_inputs` (queued
follow-up prompts) are not read.

CLI history (done): select `.nanobot*/history/cli_history`, since `--home` moves it.

Run records and sidebar cache (done, source-derived; not observed on any real install;
[issue 207](https://github.com/seanthegeek/doubleagent/issues/207)): the rows in the table above,
from the shapes in [Other stores](#other-stores). The run records are selected under any
`.nanobot*` directory (the default workspace, and the state directory for the legacy cron
location) and, in a project base the project catalog collected, at its root. The prompt row
carries the text the session stores for the turn, rebuilt from the record
([Joins and timestamps](#joins-and-timestamps)), so it pairs with the session's copy; a
trigger's content or reply longer than the record's 4,000-character limit is cut there, pairs
with nothing and is shown twice. The sidebar cache's rows name chats whose session row is gone
or never existed, so they always stay.

#### Sync, relay and origin rows

Sync: nanobot records no sync state, because it uploads no session to a vendor or share service. The
session, the WebUI transcript, the memory journal and the CLI history are written only under the
home and the workspace ([Transcript stores](#transcript-stores)), and the project's own privacy
notes say chat history is stored locally and that the model provider sees the prompts
([`SECURITY.md:208-212`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/SECURITY.md#L208-L212)).
A channel (Telegram, Slack and the other chat services nanobot answers through) keeps its own copy
of the messages it carried, on the channel's servers, as the model provider sees the prompts;
neither is sync. Searched at the [Source and evidence](#source-and-evidence) commit: `sync`,
`share`, `upload`, `cloud`, `telemetry`, `analytics`, `backup`, `export`, `remote`, every `https://`
host outside the provider and channel modules, and the WebUI and TUI front ends for analytics
libraries (none). The paths found and read, none of which carries session content to a vendor:

- Langfuse tracing, an LLM-observability service, is opt-in: with the `langfuse` extra installed
  (`nanobot plugins enable langfuse`) and `LANGFUSE_SECRET_KEY` set in the environment, the
  OpenAI-compatible provider's client is swapped for Langfuse's wrapper, which sends each model
  call's prompt and reply to `LANGFUSE_BASE_URL` (default `https://cloud.langfuse.com`)
  ([`providers/openai_compat_provider.py:624-632`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/providers/openai_compat_provider.py#L624-L632);
  [`webui/settings_capabilities.py:197-205`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/webui/settings_capabilities.py#L197-L205);
  [`pyproject.toml:85-86`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/pyproject.toml#L85-L86);
  [`docs/provider-cookbook.md:529-557`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/docs/provider-cookbook.md#L529-L557)).
  The setting lives only in the process environment, not in `config.json`, and the gateway's systemd
  unit sets no such variable
  ([`gateway/service.py:268-273`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/gateway/service.py#L268-L273)),
  so nothing collected says a session was traced; a responder who finds the `langfuse` package in
  nanobot's Python environment asks the Langfuse project for the traces. Since [`909b277`](https://github.com/HKUDS/nanobot/commit/909b277e7a5a7ecd2aebdba9611e5febe86988d9) the
  Codex provider is wrapped too
  ([`providers/openai_codex_provider.py:59`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/providers/openai_codex_provider.py#L59);
  [`providers/openai_compat_provider.py:595-601`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/providers/openai_compat_provider.py#L595-L601)).
- Remote WebUI pairing connects the local WebUI over SSH to a nanobot gateway on another host the
  user runs; that host's sessions stay on it, in its own `.nanobot`, and the local side keeps only
  the server list and the pairing keys ([Secrets](#secrets)). "Forget" removes a pairing's
  files
  ([`webui/remote_pairing.py:330-339`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/webui/remote_pairing.py#L330-L339))
  and leaves the append-only `remote-origin-ports.json` ledger
  ([`webui/remote_instances.py:105-125`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/webui/remote_instances.py#L105-L125)).
  The parser does not read these files; they point the responder at the other host to collect.
- The update check is a GET of the PyPI release record on request
  ([`webui/version_check.py:1-3`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/webui/version_check.py#L1-L3),[`19`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/webui/version_check.py#L19)),
  the GitHub star invitation is local state only
  ([`webui/star_prompt.py:1`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/webui/star_prompt.py#L1)),
  and the skills marketplace runs `npx skills add` with `DISABLE_TELEMETRY=1`
  ([`webui/skills_marketplace.py:382-388`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/webui/skills_marketplace.py#L382-L388)).
- A WebUI Temporary Chat runs on a non-persistent session and writes no session file or WebUI
  transcript
  ([`webui/temporary_chats.py:40-44`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/webui/temporary_chats.py#L40-L44),[`84-98`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/webui/temporary_chats.py#L84-L98);
  [`session/manager.py:1634-1642`](https://github.com/HKUDS/nanobot/blob/acdae3d0ae2714b6dde672428e921dbf705c096f/nanobot/session/manager.py#L1634-L1642)),
  so such a chat leaves no transcript on disk at all.

Relay: source-derived; not observed on any real install
([issue 207](https://github.com/seanthegeek/doubleagent/issues/207)). The parser puts the session
key and `metadata.last_channel` in its session start row and, after it at its time, writes
`session relayed: <channel> chat <chat_id> (from session key)` for a key whose channel is a
messaging platform, and `session relayed: <channel> chat <chat_id> (from metadata.last_channel)`
when the last route is a messaging platform and differs from the key, as the `agent-zero` and
`goose` parsers do; `records_relay` is set. The channel prefixes at `909b277` are
`dingtalk`, `discord`, `email`, `feishu`, `linear`, `matrix`, `mattermost`, `mochat`, `msteams`,
`napcat`, `qq`, `signal`, `slack`, `telegram`, `wecom`, `weixin`, `whatsapp` and `whatsapp_agent`
(each channel's `name`, for example
[`channels/telegram/runtime.py:589`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/channels/telegram/runtime.py#L589));
`websocket` (the WebUI), `cli`, `api`
([`api/server.py:361`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/api/server.py#L361)),
`heartbeat`, `dream:` and `unified:default` are local, though `unified:default` with a messaging
`last_channel` was relayed
([`session/keys.py:8-38`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/session/keys.py#L8-L38)).
Origin: the agent records none; `nanobot remote` pairs a server with a local WebUI over SSH, "No
public WebUI or relay"
([`cli/remote.py:1`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/cli/remote.py#L1),[`20`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/cli/remote.py#L20)).

#### Scratch directory rows

No row needed. The default workspace and the compose file's `api-workspace` are under
`.nanobot*`, which the catalog rule already classes ([Scratch directories](#scratch-directories));
a `--home` outside the home's `.nanobot*` is neither collected nor classed.

#### Deletion rows

No row; the session's rows are gone from the database, and what survives is the secondary
sources above; the full list, with its citations, is under
[What survives a delete](#what-survives-a-delete).

#### Bad-line rows

Every JSONL file and the CLI history report each bad line (`iter_jsonl` and
`read_input_history`); in the database, each `messages.record` that does not decode is its own
`parser: unparseable record <position> (<reason>): <record>` row and session metadata that does
not decode a `parser: unparseable session metadata (<reason>): <metadata>` row (the sources
table under [Past use and secondary sources](#past-use-rows-and-secondary-sources)).

A `sessions.sqlite3` that cannot be read, or whose `-wal`, `-shm` or `-journal` sidecar cannot, is
one row `parser: <file> could not be read (<reason>)`, `<file>` the name of the file that failed;
one that is not a SQLite database is `parser: not a SQLite database`, each with the workspace's
project path.

#### Fixtures

Fixture, file `.nanobot/sessions/0123456789abcdef0123456789abcdef/dGVsZWdyYW06MTIzNDU2Nzg5.jsonl`,
with `.workspace` containing `/home/alice/.nanobot/workspace`:

```json
{"_type":"metadata","key":"telegram:123456789","created_at":"2026-10-01T10:15:00.000001","updated_at":"2026-10-01T10:15:09.500000","metadata":{"last_channel":"telegram:123456789"},"last_archived":0,"last_consolidated":0}
{"role":"user","content":"what is using port 8080?","timestamp":"2026-10-01T10:15:01.200000"}
{"role":"assistant","content":"","tool_calls":[{"id":"call_abc123","type":"function","function":{"name":"exec","arguments":"{\"command\": \"ss -ltnp | grep 8080\"}"}}],"reasoning_content":"Check listening sockets.","timestamp":"2026-10-01T10:15:04.000000"}
{"role":"tool","tool_call_id":"call_abc123","name":"exec","content":"LISTEN 0 128 *:8080 users:((\"python3\",pid=4242))","timestamp":"2026-10-01T10:15:05.000000"}
{"role":"assistant","content":"python3 (pid 4242) is listening on 8080.","timestamp":"2026-10-01T10:15:09.400000"}
```

A `/model` switch in a session, from
[`command/builtin.py:435-448`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/command/builtin.py#L435-L448)
and
[`agent/loop.py:2143-2155`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/agent/loop.py#L2143-L2155):

```json
{"role":"user","content":"/model fast","_command":true,"timestamp":"2026-10-04T08:01:00"}
{"role":"assistant","content":"Switched model preset to `fast`.\n- Scope: current session\n- Model: `gpt-5.4-mini`\n- Context window: 128000\n- Max output tokens: 8192","_command":true,"timestamp":"2026-10-04T08:01:00"}
```

`pairing.json`, from
[`pairing/store.py:73-85`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/pairing/store.py#L73-L85)
and [`130-160`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/pairing/store.py#L130-L160):

```json
{"approved": {"telegram": ["123456789"]}, "pending": {"ABCD-EFGH": {"channel": "discord", "sender_id": "999", "created_at": 1791187200.5, "expires_at": 1791187800.5, "delivery_status": "sent"}}}
```

A cron run record `cron/runs/job1_1791187200000_ab12cd34.json`, from
[`cron/bound_runner.py:98-146`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/cron/bound_runner.py#L98-L146)
and [`utils/run_records.py:19-29`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/utils/run_records.py#L19-L29):

```json
{"job_id": "job1", "job_name": "morning digest", "session_key": "slack:C42", "prompt_ref": "x", "prompt_vars": {"message": "summarise the inbox"}, "rendered_prompt": "Reminder: summarise the inbox", "status": "ok", "response": "Three new mails.", "run_id": "job1:1791187200000:ab12cd34", "updated_at_ms": 1791187205000}
```

#### Source host rows

The agent records a source-host token but no connected account, so the parser yields no
`source host:` row. Two GitHub credentials exist, neither naming the account in a collected file:

- The GitHub MCP preset of the WebUI runs `ghcr.io/github/github-mcp-server` in Docker with a
  personal access token the user pastes, stored in `config.json` as
  `tools.mcpServers.github.env.GITHUB_PERSONAL_ACCESS_TOKEN`
  ([`webui/mcp_presets.json:331-367`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/webui/mcp_presets.json#L331-L367);
  [`webui/mcp_presets_api.py:1086-1088`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/webui/mcp_presets_api.py#L1086-L1088));
  the collector flags the whole file ([Credentials](#credentials)), and the parser reads no key
  of `tools.mcpServers`. The token's account and the repositories it reaches are server-side
  only.
- The GitHub Copilot model provider's sign-in (`nanobot provider login github-copilot`, a
  device-code OAuth flow with scope `read:user`) serves model calls, not repository access; its
  token and the account's login are in `github-copilot.json` outside `.nanobot*`, which the
  catalog does not collect ([Credentials](#credentials), [Per-user storage](#per-user-storage)).

The bundled `github` skill drives the user's own `gh` CLI and its login
([`skills/github/SKILL.md:1-10`](https://github.com/HKUDS/nanobot/blob/909b277e7a5a7ecd2aebdba9611e5febe86988d9/nanobot/skills/github/SKILL.md#L1-L10)),
which is the GitHub CLI's state, not nanobot's. No repository list is cached and no
repository-bound remote conversation exists: grepped `nanobot/` at
[`909b277`](https://github.com/HKUDS/nanobot/commit/909b277e7a5a7ecd2aebdba9611e5febe86988d9) for
`github`, `gitlab`, `bitbucket`, `oauth`, `installation`, `repositor`, `pull request` and `clone`;
GitLab and Bitbucket appear nowhere, and `installation` and `oauth` also find the Linear channel's
OAuth app, an issue tracker, and the MCP and xAI OAuth stores of [Credentials](#credentials).

## Confidence

- High (source): every path in [Per-user storage](#per-user-storage) to
  [Where the project path is recorded](#where-the-project-path-is-recorded), read from constants in the cited files.
- High (source): file naming, metadata and message field names, naive local timestamps and the
  tool call shape of [Record schema](#record-schema); the WebUI transcript record shape and layout.
- Medium: that `matrix-store/` holds credentials (inferred from its role; not read).
- Medium: `.nanobot-*` as an instance naming convention comes from docs, and any other
  `--config` location is missed.
- Medium: the `thinking_blocks` element shape is provider-defined.
- Medium: that the fixture's WebUI pairing holds on a real install, since no WebUI transcript or
  CLI history was observed.
- Not determined: the contents of `webui/workspace-state.json`, `weixin/` and `mochat/` state.
- Not determined: whether the Docker image, run without the compose file's
  `/home/nanobot/.nanobot` mount ([Docker](#docker)), keeps its state elsewhere, such as
  `/root/.nanobot`.
- Not determined: whether subagent runs are persisted anywhere.
- Not determined: the legacy WebUI `<safe key>.json` snapshot shape.
- Medium: the OAuth token paths outside `.nanobot*`, read in the `oauth-cli-kit` 0.1.6 wheel and
  platformdirs 4.3.6; an install resolves whatever versions satisfy `oauth-cli-kit>=0.1.6,<1.0.0`.
