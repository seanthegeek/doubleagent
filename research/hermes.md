# hermes

Hermes Agent (catalog agent `hermes`) is Nous Research's open-source Python agent, with a classic
CLI, a TUI (terminal interface), an Electron desktop app, a messaging gateway that drives the agent
from chat platforms, cron jobs and delegated subagents; every front end stores every conversation
in one SQLite store per profile home, in OpenAI chat-completions message shape. It is not a fork
of another agent, but it can import Claude Code and Codex CLI sessions
([Imported sessions](#imported-sessions)), and Agent Zero's Orchestrator plugin can start it
([Started by another agent](#started-by-another-agent)). Parsed: the hermes parser reads
`state.db`, its quick snapshots, the divert JSONL sessions, `pastes/pending.json`, the prompt
history, the cron run archive, `response_store.db`, the trajectory files, the standing approvals
in `config.yaml` and the pairing stores, and the desktop app's `connections.json`.

## Source and evidence

[NousResearch/hermes-agent](https://github.com/NousResearch/hermes-agent), MIT
([`LICENSE`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/LICENSE#L1-L3)),
open source, Python with an Electron desktop app, commit
[`8b66a51036c1e20920a17cdd049fdf55c968d683`](https://github.com/NousResearch/hermes-agent/commit/8b66a51036c1e20920a17cdd049fdf55c968d683).
All claims are from source; the documented home layout in
[`website/docs/user-guide/configuration.md:22-31`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/website/docs/user-guide/configuration.md#L22-L31)
agrees with it. Read only: nothing was installed or run.

### Observed

- No real install. The model-switch marker rows, a malformed `tool_calls` value and a Hermes home
  in a Docker or Podman named volume are not observed on any real install, source-derived
  (NousResearch/hermes-agent at
  [`86fa9e6`](https://github.com/NousResearch/hermes-agent/commit/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872)
  for the markers,
  [`8b66a51`](https://github.com/NousResearch/hermes-agent/commit/8b66a51036c1e20920a17cdd049fdf55c968d683)
  for the volume mount point); the `pastes/pending.json` rows are fixture only
  ([Model-switch markers](#model-switch-markers), [Bad-line rows](#bad-line-rows),
  [Volume homes](#volume-homes), [Debug-share paste list](#debug-share-paste-list)).

### Source reviews

- 2026-10-21: NousResearch/hermes-agent at
  [`86fa9e6`](https://github.com/NousResearch/hermes-agent/commit/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872),
  with prompt-toolkit/python-prompt-toolkit at
  [`d8adbe9`](https://github.com/prompt-toolkit/python-prompt-toolkit/commit/d8adbe9bfcf5f7e95b015d8ab12e2985b9a7822b)
  (3.0.52, the version Hermes pins) for the history file format: the Parser plan's proposals are
  now what the parser does; added the shapes of the cron run documents, the response store's
  history, the Bot Mode room events and the desktop connection registry, the snapshot count after
  `hermes update`, the gateway tokens in the desktop connection files, and the source-host review.
- 2026-10-19: HEAD
  [`86fa9e6ca1de7ee71132d2be0103f3e37d1fa872`](https://github.com/NousResearch/hermes-agent/commit/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872),
  against the
  [`8b66a51036c1e20920a17cdd049fdf55c968d683`](https://github.com/NousResearch/hermes-agent/commit/8b66a51036c1e20920a17cdd049fdf55c968d683)
  of 2026-10-04. Added the permission outcomes and standing settings, the model-switch markers,
  the sources of past use beyond `state.db` ([Other stores](#other-stores)), the trace upload
  and the desktop backends, the scratch directories and worktrees, the deletion tombstones and
  the prune default, and the schema 32 drift ([Format versions](#format-versions)); every catalog
  path is unchanged.

## Collection

### Per-user storage

Home resolution: context override, then `HERMES_HOME`, then the platform default
([`hermes_constants.py:111-118`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_constants.py#L111-L118)):
`~/.hermes` on Linux and macOS, `%LOCALAPPDATA%\hermes` (fallback `~/AppData/Local/hermes`) on
Windows, each with an optional `HERMES_DATA_DIR_SUFFIX` appended
([`hermes_constants.py:51-58`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_constants.py#L51-L58)).
The suffix is appended with no separator: a channel build's `-channel-build-canary` gives
`~/.hermes-channel-build-canary`
([`hermes_constants.py:53-57`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/hermes_constants.py#L53-L57),
[`environment-variables.md:894`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/website/docs/reference/environment-variables.md#L894)),
so a suffixed home is `.hermes<suffix>` or `AppData/Local/hermes<suffix>`.
The Windows desktop app keeps using a legacy `~/.hermes` when `%LOCALAPPDATA%\hermes` does not exist
([`apps/desktop/electron/data-paths.mjs:26-33`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/apps/desktop/electron/data-paths.mjs#L26-L33),[`56-65`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/apps/desktop/electron/data-paths.mjs#L56-L65)),
so both forms occur on Windows. No XDG paths. Every path the catalog names is unchanged at the
2026-10-19 HEAD
([`86fa9e6`](https://github.com/NousResearch/hermes-agent/commit/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872)).

#### Scratch directories

At the 2026-10-19 HEAD, `cache/scratch/<entry>` is the `TMPDIR` of every child process, reaped when
idle
([`hermes_constants.py:957`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/hermes_constants.py#L957),
[`hermes_constants_scratch.py:1-6`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/hermes_constants_scratch.py#L1-L6));
it is under the catalog's `.hermes` entry and collected. `sandboxes/` (or `TERMINAL_SANDBOX_DIR`)
holds the container backends' persistent bind mounts, `sandboxes/docker/<task>` for Docker
([`tools/environments/base.py:189-191`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/tools/environments/base.py#L189-L191),
[`tools/environments/docker.py:856`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/tools/environments/docker.py#L856));
it is excluded ([Exclusions](#exclusions)).

The git worktrees the agent creates are inside the user's repository, not the home:
`<repo>/.worktrees/<name>` on branch `hermes/<name>` for `hermes -w`, `.worktrees/subagent-<id>`
on `hermes-subagent/<name>` for delegated subagents, and `.worktrees/<child id>` for kanban
children
([`hermes_cli/worktree_ops.py:435-441`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/hermes_cli/worktree_ops.py#L435-L441),
[`tools/subagent_worktree.py:81-96`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/tools/subagent_worktree.py#L81-L96),
[`hermes_cli/kanban_db_graph.py:181`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/hermes_cli/kanban_db_graph.py#L181)).
A `-w` session runs with its working directory in the worktree
([`hermes_cli/cli_commands_mixin.py:1189-1197`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/hermes_cli/cli_commands_mixin.py#L1189-L1197)),
so `sessions.cwd` holds the worktree path
([`hermes_cli/worktree_ops.py:822-825`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/hermes_cli/worktree_ops.py#L822-L825)),
and a retained TUI worktree is locked with the reason `hermes session=<id> db=<path>`
([`hermes_cli/worktree_ops.py:805-815`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/hermes_cli/worktree_ops.py#L805-L815)).
The project catalog collects the repository's `.hermes/` files, not `.worktrees/`.

#### Profiles

Named profiles are complete homes under `<root>/profiles/<name>/`
([`hermes_cli/profiles.py:287-297`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_cli/profiles.py#L287-L297));
every path in this section can also appear one level down under `profiles/*/`.

#### Docker

The compose file bind-mounts the host `~/.hermes` to `/opt/data`
([`docker-compose.yml:36-37`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/docker-compose.yml#L36-L37)),
so container deployments still leave state in the host home.

#### Contents of the home

Relative to the home:

- `state.db` with `-wal`/`-shm`: the session and message store, canonical for CLI, gateway and
  desktop
  ([`hermes_state.py:176`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_state.py#L176)).
- `sessions/`: `sessions.json` gateway routing index
  ([`hermes_state_schema.py:1311`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_state_schema.py#L1311)),
  divert transcripts `<id>.jsonl`, legacy snapshots and API error dumps
  ([Transcript stores](#transcript-stores)), and `saved/hermes_conversation_<ts>.{json,md,html}`
  from `/save`
  ([`hermes_cli/cli_session_mixin.py:677-689`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_cli/cli_session_mixin.py#L677-L689)).
- `session-exports/`: default output of `hermes sessions export`
  ([`hermes_cli/sessions_cmd.py:74-76`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_cli/sessions_cmd.py#L74-L76)).
- `config.yaml`, `.env`, `SOUL.md` (agent identity), `skills/`
  ([`hermes_constants.py:1190-1202`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_constants.py#L1190-L1202));
  `memories/MEMORY.md` and `memories/USER.md`
  ([`tools/memory_tool.py:38-40`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/tools/memory_tool.py#L38-L40),
  [`tools/memory_tool_store.py:212`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/tools/memory_tool_store.py#L212)).
- `logs/agent.log`, `errors.log`, `gateway.log`
  ([`AGENTS.md:247-248`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/AGENTS.md#L247-L248)).
- Other SQLite stores the backup code treats as user data: `cron/jobs.json`, `cron/executions.db`,
  `projects.db`, `response_store.db` (API-server conversation history and tool payloads),
  `memory_store.db`, `verification_evidence.db`, `kanban.db`, `kanban/boards`
  ([`hermes_cli/backup.py:1061-1072`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_cli/backup.py#L1061-L1072)).
- `checkpoints/`: one shared shadow git store; `store/projects/<hash16>.json` records `workdir`
  ([`tools/checkpoint_manager.py:13-25`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/tools/checkpoint_manager.py#L13-L25),[`74`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/tools/checkpoint_manager.py#L74)).
- `state-snapshots/<id>/` (quick snapshots of `state.db`, `.env`, `auth.json` and the other
  stores) and `backups/` (zip backups of the home), described in
  [Transcript stores](#transcript-stores)
  ([`hermes_cli/backup.py:1061-1062`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_cli/backup.py#L1061-L1062),[`1079-1084`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_cli/backup.py#L1079-L1084),[`1120-1124`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_cli/backup.py#L1120-L1124)).
- `cache/terminal` (spilled tool results, background-process logs) and `cache/scratch`
  ([Scratch directories](#scratch-directories))
  ([`configuration.md:240-256`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/website/docs/user-guide/configuration.md#L240-L256)).
- `hermes-agent/`: the installer's git checkout and venv
  ([`scripts/install.sh:79`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/scripts/install.sh#L79),
  [`scripts/install.ps1:23-24`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/scripts/install.ps1#L23-L24)).
- `.hermes_history`: the prompt history of the CLI and the TUI
  ([Prompt history](#prompt-history)). Collected; the profile export leaves it out, with
  `state.db` and `response_store.db`
  ([`hermes_cli/profiles.py:241`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/hermes_cli/profiles.py#L241)).
- `cron/output/<job id>/<YYYY-mm-dd_HH-MM-SS>.md`
  ([`cron/jobs.py:110`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/cron/jobs.py#L110)):
  one Markdown file per cron run ([Cron run archive](#cron-run-archive)). Collected.
- `pairing/<platform>-approved.json` and `-pending.json`: the messaging gateway's approved and
  pending users ([Credentials](#credentials),
  [Permission outcomes and settings](#permission-outcomes-and-settings)).

#### Started by another agent

At agent0ai/agent-zero
[`ff2a5b9`](https://github.com/agent0ai/agent-zero/commit/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a),
Agent Zero's Orchestrator plugin, which tells that agent how to drive other coding CLIs on the
host or in its container, defaults Hermes to `yolo: true` and runs
`hermes chat --quiet --source tool -q <task> --yolo`, which bypasses every dangerous-command
approval prompt for the run
([`hermes_cli/_parser.py:307-308`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/hermes_cli/_parser.py#L307-L308);
[`plugins/_orchestrator/default_config.yaml:7-37`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/plugins/_orchestrator/default_config.yaml#L7-L37),
[`plugins/_orchestrator/skills/orchestrator/references/hermes.md:16`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/plugins/_orchestrator/skills/orchestrator/references/hermes.md#L16)).
Such a session is an ordinary session in this home's `state.db`, started by another agent with
approvals bypassed; the plugin's permission settings are in
[Permission rows in `agent-zero.md`](agent-zero.md#permission-rows).

#### Desktop app

Electron `productName` is `Hermes`
([`apps/desktop/package.json:3`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/apps/desktop/package.json#L3)),
so its `userData` is the Electron default `Hermes` directory, overridable by
`HERMES_DESKTOP_USER_DATA_DIR` and otherwise suffixed with `HERMES_DATA_DIR_SUFFIX` like the
home (`Hermes<suffix>`,
[`data-paths.mjs:35-39`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/apps/desktop/electron/data-paths.mjs#L35-L39)).
The app depends on `electron-updater`
([`apps/desktop/package.json:123`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/apps/desktop/package.json#L123)),
whose download cache is named after the package name `hermes` plus `-updater`
([`package.json:2`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/apps/desktop/package.json#L2);
[`appInfo.ts:159-161`](https://github.com/electron-userland/electron-builder/blob/39db76fc9034061769aaf87b02d020d60b00a254/packages/app-builder-lib/src/appInfo.ts#L159-L161))
and placed in `%LOCALAPPDATA%` on Windows
([`AppAdapter.ts:60-72`](https://github.com/electron-userland/electron-builder/blob/39db76fc9034061769aaf87b02d020d60b00a254/packages/electron-updater/src/AppAdapter.ts#L60-L72)):
`AppData/Local/hermes-updater`, beside the home, holding downloaded installers.
It holds `connection.json`, `connections.json`, `active-profile.json` and window state
([`apps/desktop/electron/main.ts:1222`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/apps/desktop/electron/main.ts#L1222),[`1331-1341`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/apps/desktop/electron/main.ts#L1331-L1341));
conversations stay in `state.db`. `connections.json` is the registry of the backends the app
talks to ([Desktop connection registry](#desktop-connection-registry)); both connection files are
credentials ([Credentials](#credentials)).

### Credentials

Hermes keeps its own authoritative list, `PROFILE_CREDENTIAL_PATHS`
([`hermes_cli/profiles.py:155-188`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_cli/profiles.py#L155-L188)),
relative to each profile home: `auth.json` (OAuth provider credentials and credential pool), `.env`
(API keys, platform bot tokens), `auth/google_oauth.json`, `.op.env` (1Password service-account
token), `npmrc`, `.anthropic_oauth.json`, `google_token.json`, `google_client_secret.json`,
`google_oauth_pending.json`, `google_chat_user_tokens/`, `google_chat_user_*.json`,
`slack_tokens.json`, `honcho.json`, `mem0.json`, `webhook_subscriptions.json` (HMAC secrets),
`teams_pipeline_store.json`, `mcp-tokens/`, `vault/` (`vault.key` beside `vault.json.enc`), browser
profiles (`browser-profile`, `browser_auth`, `bot-desktop`, `browser-profiles`, `browser_profiles`,
`chrome-debug`), pairing stores, `whatsapp/session`, `matrix/store` (both also under `platforms/`),
`cache/bws_cache*.json` (Bitwarden plaintext cache), `weixin/accounts`, `.copilot_jwt.json`,
`runtime/photon-sidecar.json`, `proxy/` (CA key), `home/` (the subprocess `HOME` in containers: gh,
git, ssh, npm credentials,
[`hermes_constants.py:695-718`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_constants.py#L695-L718)),
and `backups`, `state-snapshots`. Copies named `auth.json.*` and `.env.bak*` are also credential
stores
([`profiles.py:189-195`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_cli/profiles.py#L189-L195)).

The pairing stores `pairing/<platform>-approved.json` and `-pending.json` stay flagged secret,
since a pending file holds live pairing codes; they are also standing permission state, whose
shape is under [Permission outcomes and settings](#permission-outcomes-and-settings).
`hermes sessions export --format trace --upload` takes a Hugging Face token from
`HF_TOKEN` or three alternate variable names in the environment
([`agent/trace_upload.py:36`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/agent/trace_upload.py#L36),
[`agent/trace_upload.py:170-172`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/agent/trace_upload.py#L170-L172)),
which the setup message tells the user to put in `.env`; `.env` is already flagged.

The installer checkout's own `hermes-agent/.env` is loaded as a fallback
([`cli.py:337-338`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/cli.py#L337-L338),
[`hermes_cli/env_loader.py:455-462`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_cli/env_loader.py#L455-L462)),
so it can hold keys inside an otherwise excluded tree. The documentation puts non-secret settings
in `config.yaml` and secrets in `.env`
([`configuration.md:62-63`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/website/docs/user-guide/configuration.md#L62-L63)),
but an `mcp_servers` entry in `config.yaml` takes `env` values and `headers` such as
`Authorization: "Bearer ..."`
([`mcp.md:318-340`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/website/docs/user-guide/features/mcp.md#L318-L340)),
so `config.yaml` is flagged in every home, profile and volume.
No OS keychain for Hermes' own secrets; secret managers (1Password, command sources) are pluggable.

The desktop app's `connection.json` and `connections.json` hold each remote gateway's token (and
extra gateway headers) as an envelope that Electron's `safeStorage` encrypts with the operating
system's key store, or as `{"encoding": "plain", "value": <token>}` when the app's secret storage
policy is off
([`apps/desktop/electron/main.ts:1349-1354`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/apps/desktop/electron/main.ts#L1349-L1354),
[`apps/desktop/electron/main.ts:8252-8257`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/apps/desktop/electron/main.ts#L8252-L8257),
[`apps/desktop/electron/main.ts:8773-8780`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/apps/desktop/electron/main.ts#L8773-L8780)).
Both, and the `connections.json.corrupt-<time>` copy the app keeps of an unreadable registry
([`apps/desktop/electron/main.ts:8759`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/apps/desktop/electron/main.ts#L8759)),
are flagged as secrets.

A GitHub token, when the user gives one, is `GITHUB_TOKEN` or `GH_TOKEN` in `.env` (or
`GITHUB_APP_ID`, `GITHUB_APP_PRIVATE_KEY_PATH` and `GITHUB_APP_INSTALLATION_ID` for a GitHub App),
used by the Skills Hub, plugin installs, the update check and the webhook platform's
`github_comment` delivery, with the gh CLI's own login (`gh auth token`) as the fallback
([`tools/skills_hub_github.py:72-74`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/tools/skills_hub_github.py#L72-L74),
[`tools/skills_hub_github.py:134-148`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/tools/skills_hub_github.py#L134-L148),
[`hermes_cli/git_credentials.py:11-17`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/hermes_cli/git_credentials.py#L11-L17),
[`hermes_cli/github_api.py:9-14`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/hermes_cli/github_api.py#L9-L14),
[`gateway/platforms/webhook.py:71-72`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/gateway/platforms/webhook.py#L71-L72));
`.env` is already flagged.

`state.db` can contain credentials typed into commands ([Secrets](#secrets)); it is
collected unflagged.

### Exclusions

Hermes' own backup exclusions are the best guide
([`hermes_cli/backup.py:72-92`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_cli/backup.py#L72-L92),[`94`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_cli/backup.py#L94),[`99`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_cli/backup.py#L99)):
`hermes-agent` (code checkout), `models`, `runtimes`, `node` (GGUF models, llama.cpp, Node, "tens to
hundreds of GB",
[`hermes_constants.py:205-209`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_constants.py#L205-L209)),
`installs` and `tools`
([`hermes_cli/home_data_layout.py:5-6`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_cli/home_data_layout.py#L5-L6)),
`checkpoints` (shadow git), live Chromium profiles, `node_modules`, `.venv`, `site-packages`,
`sandboxes`
([`tools/environments/base.py:191`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/tools/environments/base.py#L191)).
Keep `checkpoints/store/projects/*.json` for discovery. The desktop app's `Cache`, `Code Cache`
and `GPUCache` are excluded in its `Hermes` user-data directory and the suffixed ones, and the
updater cache `AppData/Local/hermes-updater` ([Desktop app](#desktop-app)), which the
suffixed-home line `AppData/Local/hermes?*` reaches, is excluded.

`state-snapshots/` and `backups/` hold older copies of `state.db`, so they may be the only record of
a deleted or pruned session ([What survives a delete](#what-survives-a-delete)). The catalog
collects `state-snapshots/`, whose count is bounded ([Transcript stores](#transcript-stores)) and
whose databases the parser reads, and excludes the zip archives under `backups/`; use `--full`
when deletion is suspected and a zip backup may hold more. A snapshot also holds copies of `.env`,
`auth.json` and the pairing stores, which the home-relative secret globs (their `*` crosses `/`)
and the volume-relative `state-snapshots/*/.env`, `state-snapshots/*/auth.json`,
`state-snapshots/*/pairing/*` and `state-snapshots/*/platforms/pairing/*` flag. `cache/` is not
excluded: `cache/terminal` holds tool output. `sandboxes/` stays excluded: a container backend's
persistent workspace can hold whole repositories, and the commands that ran there are in
`state.db`.

The installer checkout is excluded entry by entry rather than as one
directory (`hermes-agent/[!.]*` and `hermes-agent/.[!e]*`), because an
exclusion wins over a secret glob: pruning the whole tree would lose
`hermes-agent/.env`, which is collected and flagged. The exclusion and
secret globs are anchored to `.hermes*/` and `AppData/Local/hermes*/`, so
they cover profiles and suffixed homes without matching unrelated paths.

### Project-local files

- `.hermes.md` or `HERMES.md`, nearest from cwd up to the git root
  ([`agent/prompt_builder.py:140-149`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/agent/prompt_builder.py#L140-L149));
  also reads `AGENTS.override.md`, `AGENTS.md`, `agents.md`, `CLAUDE.md`, `claude.md`,
  `.cursorrules`, `.cursor/rules/*.mdc`
  ([`prompt_builder.py:1676`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/agent/prompt_builder.py#L1676),[`1691`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/agent/prompt_builder.py#L1691),[`1700-1706`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/agent/prompt_builder.py#L1700-L1706)).
- `.hermes/`: `skills/`
  ([`agent/skill_utils.py:496`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/agent/skill_utils.py#L496)),
  `plugins/`
  ([`hermes_cli/plugins_discovery.py:213`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_cli/plugins_discovery.py#L213)),
  `plans/<date>-<slug>.md`
  ([`agent/plan_prompt.py:21`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/agent/plan_prompt.py#L21)),
  `environment.json`
  ([`agent/verify/environment.py:17`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/agent/verify/environment.py#L17));
  `.agents/skills` is also read.
- `trajectory_samples.jsonl` and `failed_trajectories.jsonl` in the cwd when `--save-trajectories`
  is given
  ([`agent/trajectory.py:37-44`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/agent/trajectory.py#L37-L44),
  [`agent/legacy_cli.py:50`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/agent/legacy_cli.py#L50)).
  No per-project database.

### Where the project path is recorded

- `checkpoints/store/projects/<hash16>.json`, key `workdir`
  ([`tools/checkpoint_manager.py:615-619`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/tools/checkpoint_manager.py#L615-L619)).
  Grep-able JSON; only present when checkpoints are enabled.
- `state.db` table `sessions`, columns `cwd`, `git_repo_root`, `git_branch`
  ([`hermes_state_common.py:409-411`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_state_common.py#L409-L411)).
  SQLite only.
- `projects.db` tables `projects.primary_path` and `project_folders.path`
  ([`hermes_cli/projects_db.py:29-49`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_cli/projects_db.py#L29-L49)).
  SQLite only.
- Worktrees the agent creates: a worktree session's `sessions.cwd` is the worktree path under
  `<repo>/.worktrees/`, a scratch directory inside the project
  ([Scratch directories](#scratch-directories)).

## Parsing

### Transcript stores

Globs relative to the user home:

- `.hermes/state.db` (+ `-wal`, `-shm`), and the same under `.hermes/profiles/*/` and
  `AppData/Local/hermes/`
  ([`hermes_state.py:176`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_state.py#L176);
  home resolution in
  [`hermes_constants.py:51-58`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_constants.py#L51-L58)).
  One database holds all sessions; it is canonical for the gateway too
  ([`gateway/session_transcript.py:573-575`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/gateway/session_transcript.py#L573-L575)).
- `.hermes/sessions/<session_id>.jsonl`: written only when `state.db` was replaced under a running
  process; one JSON message dict per line
  ([`hermes_state.py:429-444`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_state.py#L429-L444)).
  Hermes calls this a divert: the batch of messages a live agent or the gateway could not write
  because its database handle was replaced or quarantined is appended here instead
  ([`agent/session_persistence.py:392-397`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/agent/session_persistence.py#L392-L397),
  [`gateway/session_transcript.py:153-177`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/gateway/session_transcript.py#L153-L177)).
  An agent's line has the keys of a `messages` row in that order, `role`, `content`, `tool_name`,
  `tool_calls` (a list), `tool_call_id`, the reasoning keys, `timestamp` and others
  ([`agent/session_persistence.py:224-236`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/agent/session_persistence.py#L224-L236));
  a gateway line is the pending transcript message as the gateway queued it. No `session_id` field
  (it is the file name), no `cwd`, no model.
- `.hermes/state-snapshots/<id>/state.db`: quick snapshots. `/snapshot [label]`,
  `hermes backup --quick`, `hermes update` (label `pre-update`) and the profile repair (label
  `repair-profiles`) copy `state.db` and the other critical state files
  ([`hermes_cli/backup.py:1057-1076`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_cli/backup.py#L1057-L1076))
  into `state-snapshots/<id>/`
  ([`hermes_cli/backup.py:57`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_cli/backup.py#L57),
  [`hermes_cli/cli_commands_mixin.py:826-828`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_cli/cli_commands_mixin.py#L826-L828),
  [`hermes_cli/backup.py:2190-2193`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_cli/backup.py#L2190-L2193),
  [`hermes_cli/update_cmd_maint.py:686-691`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_cli/update_cmd_maint.py#L686-L691),
  [`hermes_cli/sessions_repair_profiles.py:485`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_cli/sessions_repair_profiles.py#L485)).
  `<id>` is the UTC time `%Y%m%d-%H%M%S`, then `-<label>` when one was given, then `-2`, `-3` on a
  collision
  ([`hermes_cli/backup.py:1263-1270`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_cli/backup.py#L1263-L1270));
  the files are staged under `.<id>.<pid>.partial` and renamed into place
  ([`hermes_cli/backup.py:1271`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_cli/backup.py#L1271),
  [`hermes_cli/backup.py:1420`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_cli/backup.py#L1420)),
  and a `manifest.json` records `id`, `timestamp`, `label`, `file_count`, `total_size`, `files`,
  `failed_dbs` and `oversized_skipped`
  ([`hermes_cli/backup.py:1393-1402`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_cli/backup.py#L1393-L1402)).
  A database is copied with SQLite's backup API, so the copy is a consistent, complete database with
  no WAL of its own
  ([`hermes_cli/backup_sqlite.py:31-35`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_cli/backup_sqlite.py#L31-L35),
  [`hermes_cli/backup_sqlite.py:68`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_cli/backup_sqlite.py#L68)).
  By default 20 snapshots are kept, older ones pruned (the `hermes update` caller passes its own
  count), except when a database failed to copy or was skipped as too large
  ([`hermes_cli/backup.py:1079`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_cli/backup.py#L1079),
  [`hermes_cli/backup.py:1422-1430`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_cli/backup.py#L1422-L1430));
  `hermes update` passes a file-size cap, so a large `state.db` can be missing from its snapshot
  ([`hermes_cli/update_cmd_maint.py:690-691`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_cli/update_cmd_maint.py#L690-L691)).
  At the 2026-10-19 HEAD that cap is 1 GiB and the count `hermes update` passes is 1, so an update
  prunes every older snapshot, manual ones included, unless a database failed to copy or was too
  large
  ([`hermes_cli/update_cmd_maint.py:30-34`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/hermes_cli/update_cmd_maint.py#L30-L34),
  [`hermes_cli/backup.py:1457-1465`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/hermes_cli/backup.py#L1457-L1465));
  the files copied are `state.db`, `config.yaml`, `.env`, `auth.json`, the cron, gateway, project,
  response, memory, verification and kanban stores and both pairing directories
  ([`hermes_cli/backup.py:1104-1122`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/hermes_cli/backup.py#L1104-L1122)).
  Collected by default ([Exclusions](#exclusions)).
- `.hermes/backups/pre-update-*.zip` and `pre-migration-*.zip`: full zip backups of the home, 5 of
  each
  ([`hermes_cli/backup.py:1120-1124`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_cli/backup.py#L1120-L1124));
  not parsed.
- `.hermes/sessions/<id>.json`, `session_<id>.json` (legacy snapshots) and
  `request_dump_<id>_<ts>.json` (redacted API request on error)
  ([`hermes_state_sessions.py:1573-1585`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_state_sessions.py#L1573-L1585),
  [`agent/agent_runtime_helpers.py:1628-1633`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/agent/agent_runtime_helpers.py#L1628-L1633)).
  Nothing at the checked commit writes the two legacy snapshot names; the
  only code that names them is `_remove_session_files`, which deletes them
  with a session (an evaluation script checks that the old
  `write_json_snapshots` setting no longer produces any,
  [`evals/session_snapshot_removal.py:12-29`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/evals/session_snapshot_removal.py#L12-L29)),
  so their shape cannot be read from this source and the parser does not
  read them.
- `.hermes/sessions/saved/hermes_conversation_<YYYYmmdd_HHMMSS>.json`
  (`{id, model, started_at, messages}`) from `/save`
  ([`hermes_cli/cli_session_mixin.py:669-689`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_cli/cli_session_mixin.py#L669-L689)).

`state.db` runs in WAL mode by default with a DELETE fallback on filesystems that refuse it
([`hermes_state_wal.py:254-262`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_state_wal.py#L254-L262));
read `state.db-wal` with the main file. FTS5 tables `messages_fts` and `messages_fts_trigram`
([`hermes_state_common.py:785`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_state_common.py#L785),[`905`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_state_common.py#L905))
are indexes; skip them. No compression or encryption.

Session ids are `<YYYYmmdd_HHMMSS>_<hex>` from local time
([`hermes_state_ids.py:56-60`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_state_ids.py#L56-L60)).
At the 2026-10-19 HEAD a session created through the unified runtime
(Hermes' newer single owner of session creation for the local front ends
and cron) is instead `local-` followed by the SHA-256 of its profile,
principal and request ids, and a cron run's request id is
`cron:<job>:<execution>`; runs from before it were
`cron_<job>_<YYYYmmdd_HHMMSS>`
([`hermes_state_keys.py:11-17`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/hermes_state_keys.py#L11-L17),
[`hermes_state_keys.py:71-74`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/hermes_state_keys.py#L71-L74)).
The parser treats the id as an opaque string, so both forms work.

#### What survives a delete

`hermes sessions delete`, and the dashboard and desktop deletes, remove
the session, its whole local reset or compression lineage when it has one,
its delegate children, their `messages` rows and the files under `sessions/`
named after the id (`<id>.json`, `<id>.jsonl`, `session_<id>.json`,
`request_dump_<id>_*.json`)
([`hermes_state_sessions.py:1628-1648`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/hermes_state_sessions.py#L1628-L1648),
[`hermes_state_sessions.py:1701-1770`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/hermes_state_sessions.py#L1701-L1770)).
`hermes sessions prune`, and the 90-day sweep when `sessions.auto_prune`
is turned on, remove ended sessions the same way. At the cited commit
ended sessions idle 90 days were pruned by default and the file may be VACUUMed
([`hermes_cli/config_defaults.py:2276-2285`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_cli/config_defaults.py#L2276-L2285),
[`hermes_state_maintenance.py:421-445`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_state_maintenance.py#L421-L445));
older copies live in `state-snapshots/*/state.db`.
Automatic pruning is off by default at the 2026-10-19 HEAD: `sessions.auto_prune` was `true` at
[`hermes_cli/config_defaults.py:2276`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_cli/config_defaults.py#L2276)
and is `false` now, with `retention_days` still 90 and the user asked
once when the disk runs low
([`hermes_cli/config_defaults.py:2279-2289`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/hermes_cli/config_defaults.py#L2279-L2289)),
so on a current install old sessions stay in `state.db` unless the user
opted in or ran `hermes sessions prune`. Archiving (`hermes sessions
archive`, the sidebar, the idle sweep `sessions.auto_archive`, off by
default) only sets `archived = 1` (and `auto_archived` for the sweep)
and keeps the messages
([`hermes_state_sessions.py:937-942`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/hermes_state_sessions.py#L937-L942),
[`hermes_state_sessions.py:1968-1980`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/hermes_state_sessions.py#L1968-L1980)).

New since the cited commit, every delete path, the prune sweep
included, first writes deletion fences (tombstones) into `state.db` `state_meta`
(a key and JSON value table) that outlive the rows
([`hermes_state_mutation_retirement.py:148-163`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/hermes_state_mutation_retirement.py#L148-L163)):
the key `gateway.retired_session.v1.<session id>` with
`{"retired_at": <epoch seconds>, "own": true}`, plus `"expires": true`
when a sweep rather than a delete removed it
([`hermes_state_mutation_retirement.py:9`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/hermes_state_mutation_retirement.py#L9),
[`hermes_state_mutation_retirement.py:286-300`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/hermes_state_mutation_retirement.py#L286-L300),
[`hermes_state_mutation_retirement.py:307-317`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/hermes_state_mutation_retirement.py#L307-L317)),
and, for each request the session had admitted, a
`gateway.terminal_admission.v1.<admission id>` row with the principal,
request id and digests and the payload emptied
([`hermes_state_mutation_retirement.py:100-140`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/hermes_state_mutation_retirement.py#L100-L140)).
The marker is the only record of when a session was deleted and of its
id; it holds no text. A deleted session's gateway routing entry is
dropped, but its `conversation_generations` row stays
([Conversation generations](#conversation-generations)).

No delete touches `.hermes_history`, the `cron/output/` run
files, `response_store.db`, the `state-snapshots/` copies of `state.db`
(the last 20 by default, each a whole older database), the trajectory
files in the working directory,
the `conversation_generations` row that names the session's platform
chat, or a memory provider's copy; sessions Hermes imported from Claude
Code or Codex CLI stay in `state.db` after their original files are
gone ([Other stores](#other-stores)).

A snapshot is the whole
database at the snapshot time, so it outlives a session pruned after 90
idle days, a session deleted with `hermes sessions delete` (which removes
the rows and the session's files under `sessions/`), a VACUUM, and a
`state.db` replaced, rebuilt or quarantined; it does not see anything
written after it. Its rows join the live database's exactly: the same
`sessions.id`, the same `cwd` and `git_repo_root`, the same
`messages.timestamp` (the backup API copies the pages, not a re-encoding).
Two things make a snapshot row differ from its live twin: a message rewound
or compacted after the snapshot (live `active = 0`, so the parser's
`[rewound]` prefix), and a session whose `end_reason`, `title` or later
model was set after it (the session start row's text). Those rows are kept
from both.

A divert holds messages the replaced database never received, so most
of its lines have no twin in the live database; a line does have one when
the same messages also reached a database the parser reads (a snapshot,
or a new `state.db` that a later flush wrote them to), and then it carries
the same session id and, when the line has one, the same `timestamp`.
Whether Hermes replays a divert into a new database was not determined;
the session-row heal of
[`agent/session_persistence.py:354-375`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/agent/session_persistence.py#L354-L375)
replays the in-memory transcript only when a session row was deleted, a
different failure.

The analyzer reads the snapshots, the `sessions/<id>.jsonl` diverts,
`.hermes_history`, `cron/output/`, `response_store.db` and the trajectory
files as secondary sources (see
[Secondary sources](../analyzer/docs/output.md#secondary-sources)), the
imported sessions as Hermes sessions, and the tombstones and the
`conversation_generations` rows as `system` rows
([Past use and secondary sources](#past-use-rows-and-secondary-sources),
[Deletion](#deletion-rows)). It does not read the copies of
`response_store.db` and the other stores inside a snapshot, or a memory
provider's copy.

### Record schema

Schema in
[`hermes_state_common.py:372-480`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_state_common.py#L372-L480).

#### Sessions table

`sessions`
([`382-447`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_state_common.py#L382-L447)):
`id`, `source` (`cli`, `cron`, `kanban`, `acp`, `api_server`, `subagent`, `tool`, `recovered` or a
messaging platform;
[`hermes_state.py:471-473`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_state.py#L471-L473)),
`user_id`, `session_key`, `chat_id`, `chat_type`, `thread_id`, `display_name`, `origin_json`,
`model`, `model_config`, `system_prompt_hash` (joins `system_prompts.hash`), `parent_session_id`,
`started_at`, `ended_at` (REAL epoch seconds), `end_reason`, token and cost counters, `cwd`,
`git_branch`, `git_repo_root`, `title`, `profile_name`, `archived`, `hidden`, `tool_names`. `cwd`
and `git_branch` are set at creation and updated by `update_session_cwd`
([`hermes_state_sessions.py:326`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_state_sessions.py#L326),[`558-566`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_state_sessions.py#L558-L566)).

#### Messages table

`messages`
([`449-480`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_state_common.py#L449-L480)),
one row per OpenAI message:

| Column | Meaning |
| --- | --- |
| `id` | INTEGER autoincrement, order of writing |
| `session_id` | `sessions.id` |
| `role` | `user`, `assistant`, `tool`, `system` ([`hermes_state_messages.py:644`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_state_messages.py#L644)) |
| `content` | TEXT; list or dict content (multimodal parts) is stored as `"\x00json:"` + JSON ([`hermes_state_messages.py:138-147`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_state_messages.py#L138-L147), prefix at [`hermes_state.py:1612`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_state.py#L1612)) |
| `tool_calls` | JSON list on assistant rows ([`hermes_state_messages.py:266`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_state_messages.py#L266)); each `{id, call_id, response_item_id, type, function: {name, arguments}}` with `arguments` a JSON string ([`agent/chat_completion_helpers.py:1642-1644`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/agent/chat_completion_helpers.py#L1642-L1644)) |
| `tool_call_id`, `tool_name` | on `role='tool'` result rows ([`agent/turn_tool_round.py:115-118`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/agent/turn_tool_round.py#L115-L118)) |
| `timestamp` | REAL, `time.time()` epoch seconds, UTC ([`hermes_state_messages.py:399`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_state_messages.py#L399)) |
| `reasoning`, `reasoning_content` | thinking text; `reasoning_details`, `codex_reasoning_items`, `codex_message_items` are JSON ([`hermes_state_messages.py:278-280`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_state_messages.py#L278-L280)) |
| `finish_reason`, `token_count` | provider values |
| `active`, `compacted` | rewind and compaction set `active=0, compacted=1` instead of deleting ([`hermes_state_messages.py:70`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_state_messages.py#L70)) |
| `api_content` | exact string sent to the API when it differed from `content` ([`hermes_state_messages.py:390-392`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_state_messages.py#L390-L392)) |
| `platform_message_id`, `display_kind`, `display_metadata`, `message_uid`, `tool_call_uid(s)` | gateway and UI bookkeeping |

#### Model usage

Model per message is not stored; use `sessions.model`, or `session_model_usage.model`
([`hermes_state_common.py:482-502`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_state_common.py#L482-L502))
when a session switched models. `session_model_usage` has one row per (`session_id`, `model`,
`billing_provider`, `billing_base_url`, `billing_mode`, `task`), upserted with the token delta of
each API call that reports usage; `first_seen` and `last_seen` are `time.time()` epoch seconds,
`task` is empty for the main agent loop and names the work (`vision`, `compression`,
`title_generation`) for auxiliary calls
([`hermes_state_usage.py:337-365`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_state_usage.py#L337-L365)).
`sessions.model` is overwritten by a mid-session `/model` switch
([`hermes_state_sessions.py:708-735`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_state_sessions.py#L708-L735)),
so it holds the latest model, not the first.

#### Model-switch markers

Model switches made in the TUI, the desktop app or another front end of
the TUI gateway are also durable message rows, at the 2026-10-19 HEAD and
at the cited commit alike: a `role = 'user'` row with
`display_kind = 'model_switch'` and the content `[System: The active
model for this chat has changed to <model> via provider <provider>. From
this point forward, ...]`, timestamped at the switch
([`tui_gateway/server.py:1274`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/tui_gateway/server.py#L1274),
[`tui_gateway/server.py:1291-1331`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/tui_gateway/server.py#L1291-L1331)).
Each switch first deactivates the earlier marker rows of the session
(`active = 0`), which are kept, never deleted
([`hermes_state_messages.py:1064-1070`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/hermes_state_messages.py#L1064-L1070)),
so every switch survives as one row with the model, the provider and
the time; the classic CLI's `/model` writes no row and only updates
`sessions.model`. Other `display_kind` values mark rows that are not
typed input either: `hidden`, `steer`, `internal_notification`,
`async_delegation_complete`, `process_complete` and `failed_turn`
([`hermes_state_detached_results.py:29`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/hermes_state_detached_results.py#L29),
[`agent/prompt_builder.py:586`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/agent/prompt_builder.py#L586),
[`agent/turn_failure_copy.py:55`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/agent/turn_failure_copy.py#L55),
[`gateway/response_filters.py:33`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/gateway/response_filters.py#L33),
[`tools/process_registry_notifications.py:349`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/tools/process_registry_notifications.py#L349)).

#### Permission outcomes and settings

Permission outcomes are in the result row of the shell tool, registered as `terminal`
([`tools/terminal_tool.py:1634`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/tools/terminal_tool.py#L1634)),
whose `content` is JSON. A command that needed approval and got it carries
`"approval": "Command required approval (<description>) and was approved
by the user."` or `"Command was flagged (<description>) and
auto-approved by smart approval."` (smart approval is Hermes' guardian
model that judges a flagged command)
([`tools/terminal_tool.py:1081-1087`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/tools/terminal_tool.py#L1081-L1087),
[`tools/terminal_tool_result.py:268`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/tools/terminal_tool_result.py#L268),
[`tools/terminal_tool_background.py:187-188`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/tools/terminal_tool_background.py#L187-L188));
a refused one is `{"output": "", "exit_code": -1, "error": <message>,
"status": "blocked"}` with `user_summary` when a person was asked, and
one waiting for a gateway `/approve` is `"status": "pending_approval"`
with `command`, `description`, `pattern_key`, `smart_denied` and
`allow_permanent`
([`tools/terminal_tool.py:1062-1079`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/tools/terminal_tool.py#L1062-L1079)).
The refusal message names the outcome: `BLOCKED by smart approval`, a
user denial with the user's `/deny` reason, a timeout ("Silence is not
consent"), or a withdrawn prompt; the outcome enum is `denied`,
`timeout`, `notify_failed`, `cancelled`, `blocked`
([`tools/approval.py:528`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/tools/approval.py#L528),
[`tools/approval.py:790`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/tools/approval.py#L790),
[`tools/approval.py:884-890`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/tools/approval.py#L884-L890)).
The `outcome` key itself is not written to the result.

An "always"
answer appends the command's pattern to `command_allowlist` in the
profile's `config.yaml`
([`tools/approval.py:401-414`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/tools/approval.py#L401-L414),
[`tools/approval.py:470-495`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/tools/approval.py#L470-L495)),
and a session's `/yolo` bypass (all approvals off for that session) is
kept as `yolo_mode: true` in `sessions.model_config` for the CLI, TUI
and desktop and in the gateway's routing entry for messaging sessions
([`tools/approval_yolo.py:1-8`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/tools/approval_yolo.py#L1-L8),
[`agent/agent_init.py:1206-1216`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/agent/agent_init.py#L1206-L1216)).
The other standing settings are in `config.yaml`: `approvals.mode`
(`manual`, `smart` or `off`, default `smart`), `approvals.cron_mode`,
`single_query_mode` and `unattended_mode` (`deny` or `approve`),
`approvals.deny` (globs that block even under `/yolo`) and
`command_allowlist`
([`hermes_cli/config_defaults.py:1680-1718`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/hermes_cli/config_defaults.py#L1680-L1718),
[`tools/approval_context.py:263-280`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/tools/approval_context.py#L263-L280));
the environment variable `HERMES_YOLO_MODE` turns every check off for
the process
([`tools/approval.py:46`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/tools/approval.py#L46)).
The messaging gateway's pairing store, the standing list of platform
users allowed to drive the agent, is `pairing/<platform>-approved.json`,
a JSON object keyed by user id with `{user_name, approved_at}`
(`approved_at` epoch seconds), beside `<platform>-pending.json`
([`gateway/pairing.py:1-7`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/gateway/pairing.py#L1-L7),
[`gateway/pairing.py:353-356`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/gateway/pairing.py#L353-L356),
[`gateway/pairing.py:431-437`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/gateway/pairing.py#L431-L437)).
All of these existed at the cited commit, the pairing shape unchanged; none was examined then.

#### Messaging sessions

A messaging session's row names the platform and the people:
`sessions.source` is the platform (`telegram`, `discord`, `whatsapp`,
`whatsapp_cloud`, `slack`, `signal`, `mattermost`, `matrix`, `email`,
`sms`, `dingtalk`, `feishu`, `wecom`, `wecom_callback`, `weixin`,
`bluebubbles`, `qqbot`, `yuanbao`, `webhook`, `msgraph_webhook`, or a
plugin platform; `api_server` and `local` are not messaging platforms)
([`gateway/config.py:217-242`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/gateway/config.py#L217-L242)),
`user_id`, `chat_id`, `chat_type` (`dm`, `group`, `channel`, `thread`),
`thread_id`, `display_name` (the chat name) and `origin_json`, the
serialized source `{platform, chat_id, chat_name, chat_type, user_id,
user_name, thread_id, chat_topic, ...}`
([`gateway/session.py:68-80`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/gateway/session.py#L68-L80),
[`gateway/session.py:128-131`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/gateway/session.py#L128-L131),
[`gateway/session_recovery.py:336-345`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/gateway/session_recovery.py#L336-L345),
[`hermes_state_gateway.py:236-292`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/hermes_state_gateway.py#L236-L292),
[`run_agent.py:57-75`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/run_agent.py#L57-L75)).
A message that came through the experimental connector relay (a
generic adapter fed by a separate connector service) records the
underlying platform, not `relay`
([`gateway/session.py:99-102`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/gateway/session.py#L99-L102)).

### Joins and timestamps

- Message to session: `messages.session_id = sessions.id`; project path is
  `sessions.cwd` (or `git_repo_root`), branch `sessions.git_branch`.
- Tool use to result: `messages.tool_calls[].id` (or `call_id`) equals a later `role='tool'` row's
  `tool_call_id`. Hermes pairs on `call_id` then `id`, splitting at `|`
  ([`agent/message_sanitization.py:520-528`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/agent/message_sanitization.py#L520-L528)).
- Parent to child: `sessions.parent_session_id`. A parent whose `end_reason='compression'` continues
  in its child; `'branched'` marks a `/branch` fork
  ([`hermes_state_common.py:169-174`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_state_common.py#L169-L174));
  `source='subagent'` rows are delegated tasks.
- System prompt: `sessions.system_prompt_hash = system_prompts.hash`.
- Times: `messages.timestamp`, `sessions.started_at` and `ended_at` are REAL epoch seconds
  ([Record schema](#record-schema)); how snapshot and divert rows join the live rows is under
  [What survives a delete](#what-survives-a-delete).

### Other stores

The stores below sit beside `state.db`. Each says whether the parser reads it. The sources of past
use were listed at the 2026-10-19 HEAD and their shapes read there on 2026-10-21; none of them is
new since the cited commit unless it says so.

#### Prompt history

`.hermes/.hermes_history`: the prompt history of the classic CLI and
of the TUI, shared by both
([`hermes_cli/cli_init_mixin.py:389`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/hermes_cli/cli_init_mixin.py#L389),
[`hermes_cli/cli_tui_mixin.py:2208`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/hermes_cli/cli_tui_mixin.py#L2208),
[`ui-tui/src/lib/history.ts:6-7`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/ui-tui/src/lib/history.ts#L6-L7)).
prompt_toolkit's `FileHistory` format: per entry a blank line, a
`# <timestamp>` line and the entry's lines each prefixed `+`; the TUI
writes the timestamp as UTC `YYYY-MM-DD HH:MM:SS.mmm` and keeps the
last 1000 entries in memory only
([`ui-tui/src/lib/history.ts:71-78`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/ui-tui/src/lib/history.ts#L71-L78)),
while prompt_toolkit 3.0.52, the version Hermes pins, writes
`datetime.now()`, host-local time with microseconds and no zone
([`src/prompt_toolkit/history.py:297-306`](https://github.com/prompt-toolkit/python-prompt-toolkit/blob/d8adbe9bfcf5f7e95b015d8ab12e2985b9a7822b/src/prompt_toolkit/history.py#L297-L306)),
so the number of fraction digits tells the two writers apart. Every
typed line, slash commands included (`/model`, `/yolo`, `/snapshot`),
with no session id or project. Nothing that deletes a session touches
it, so it survives a delete, a prune, a rewind and a removed project.
Joins on text and time only. Secondary; read by the parser.

#### Cron run archive

`cron/output/<job id>/<YYYY-mm-dd_HH-MM-SS>.md`: one Markdown file per
cron run, local time in the name, with a `# Cron Job: <id>` header and
`## Prompt` and `## Response` sections, pruned per job to a configured
count
([`cron/jobs.py:114`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/cron/jobs.py#L114),
[`cron/jobs.py:3384-3395`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/cron/jobs.py#L3384-L3395),
[`cron/scheduler_prompt.py:72-73`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/cron/scheduler_prompt.py#L72-L73)).
The name's time and the document's `**Run Time:**` are
`hermes_time.now()`, Hermes' configured zone or the host's, written
without an offset
([`cron/jobs.py:3384-3392`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/cron/jobs.py#L3384-L3392),
[`hermes_time.py:146-149`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/hermes_time.py#L146-L149)).
A completed run's document is the header (`# Cron Job: <name>`,
`**Job ID:**`, `**Run Time:**`, `**Schedule:**`), then
`**Prompt Characters:** <n>` and `## Prompt` with the prompt, a blank
line, `**Response Characters:** <m>` and `## Response` with the
response and a newline; the two lengths frame the sections, so a
heading quoted inside the prompt is no boundary
([`cron/scheduler.py:2238-2246`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/cron/scheduler.py#L2238-L2246),
[`cron/scheduler.py:2634-2640`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/cron/scheduler.py#L2634-L2640),
[`cron/scheduler_prompt.py:70-80`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/cron/scheduler_prompt.py#L70-L80)).
Older writers left out the frames, and Hermes' own reader then takes
the last `## Response` heading as the boundary
([`cron/scheduler_prompt.py:80-94`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/cron/scheduler_prompt.py#L80-L94));
a failed, blocked or script-gated run is a short document with no
prompt section
([`cron/scheduler.py:2267`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/cron/scheduler.py#L2267),
[`cron/scheduler.py:2303-2308`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/cron/scheduler.py#L2303-L2308)).
Survives the prune or delete of the run's `source='cron'` session.
Joins on the job id and time, not on a session id. Secondary; read by
the parser.

#### API server response store

`.hermes/response_store.db`: the API server's store for the OpenAI Responses
API (`previous_response_id` chaining)
([`gateway/platforms/api_server.py:732`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/gateway/platforms/api_server.py#L732));
its schema was not examined at the cited commit. At the 2026-10-19 HEAD: table `responses`
(`response_id`, `data`, `accessed_at`), whose `data` is JSON
`{response, conversation_history, instructions, session_id}`, the full
cumulative history at that response; table `conversations` (`name`,
`response_id`); it keeps the 100 most recently used responses
([`gateway/platforms/api_server_response_store.py:14`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/gateway/platforms/api_server_response_store.py#L14),
[`gateway/platforms/api_server_response_store.py:21-24`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/gateway/platforms/api_server_response_store.py#L21-L24),
[`gateway/platforms/api_server_response_store.py:58-61`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/gateway/platforms/api_server_response_store.py#L58-L61),
[`gateway/platforms/api_server_openai_routes.py:354-358`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/gateway/platforms/api_server_openai_routes.py#L354-L358)).
`response` is the Responses API envelope (`id`, `object`, `status`,
`created_at` in epoch seconds, `model`, `output`, `usage`), and
`conversation_history` is a list of OpenAI chat-completions message
dicts, the shape of a `messages` row: the earlier history, the turn's
user message and the agent's messages, or the assistant's final text
when the agent returned none
([`gateway/platforms/api_server_openai_routes.py:330-358`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/gateway/platforms/api_server_openai_routes.py#L330-L358),
[`gateway/platforms/api_server_openai_routes.py:1472-1500`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/gateway/platforms/api_server_openai_routes.py#L1472-L1500));
the messages carry no time of their own.
No session delete path names it (grepped `response_store` and
`ResponseStore` in `hermes_state*.py` and `hermes_cli/sessions*.py`:
only the WAL helper), so it outlives a deleted `api_server` session.
Joins on `session_id`. Secondary; read by the parser.

#### Trajectory files

`trajectory_samples.jsonl` and `failed_trajectories.jsonl` in the
working directory (project files, collected by the project catalog):
one line per run in the ShareGPT format (a common layout for conversation training data),
`{conversations, timestamp, model, completed}`, `timestamp` local ISO 8601 without a zone, no
session id
([`agent/trajectory.py:37-41`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/agent/trajectory.py#L37-L41)).
Each `conversations` entry is `{from, value}`: `system` (the tool-calling
system prompt), `human` (the user's message), `gpt` (a reply, or a
tool-call turn in `<tool_call>` markup) and `tool` (the call results in
`<tool_response>` blocks)
([`agent/agent_runtime_helpers.py:170-196`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/agent/agent_runtime_helpers.py#L170-L196)).
Written only with `--save-trajectories`; survives everything. Joins on
text within a project. Secondary; read by the parser.

#### Imported sessions

`hermes_cli/foreign_sessions.py`
reads Claude Code `~/.claude/projects/*/*.jsonl` and Codex CLI
`~/.codex/sessions/**/rollout-*.jsonl` and copies their user and
assistant turns into `state.db` as a new session with `source`
`claude-code` or `codex-cli`, `origin_json`
`{"imported_from": {"tool", "path", "foreign_session_id"}}` and the
title `Imported from <tool>: <first line>`
([`hermes_cli/foreign_sessions.py:26-27`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/hermes_cli/foreign_sessions.py#L26-L27),
[`hermes_cli/foreign_sessions.py:173-178`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/hermes_cli/foreign_sessions.py#L173-L178),
[`hermes_cli/foreign_sessions.py:254-260`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/hermes_cli/foreign_sessions.py#L254-L260)).
The messages are appended at import time, so their `timestamp` is the
import's, not the original turn's. The parser reads these rows as
Hermes sessions (they are in `state.db`), and its start row names the
source tool, the original path and the foreign session id
([Past-use rows and secondary sources](#past-use-rows-and-secondary-sources)); such a session is a
copy of another agent's transcript that outlives the original file.

#### Bot Mode group chats

Bot Mode is the desktop app's roster of
profiles as named bots that share group chats. Tables `hosted_rooms`
and `hosted_room_events` (`room_id`, `seq`, `event_id`, `kind`,
`actor_json`, `payload_json`, `created_at`) are an append-only room log in
the gateway's root `state.db`
([`gateway/hosted_rooms.py:1-5`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/gateway/hosted_rooms.py#L1-L5),
[`gateway/hosted_rooms.py:88-117`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/gateway/hosted_rooms.py#L88-L117)).
The event kinds are fixed per actor: `message.user` for the user,
`message.member` for a member bot, `member.unavailable`,
`room.activity`, `room.stop_requested` and the `turn.*` kinds for the
gateway, and `authority.*`, `room.created`, `room.disbanded`,
`room.members_changed` and `room.renamed` for the system; `actor_json`
is `{kind, id}` with optional `display_name`, `profile` and
`connection_id`
([`gateway/hosted_rooms.py:48-60`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/gateway/hosted_rooms.py#L48-L60)).
A `message.user` payload is `{thread_id, text, attachments?}` and a
`message.member` payload carries the member's turn coordinates and
`text`
([`gateway/hosted_room_discussion.py:199-211`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/gateway/hosted_room_discussion.py#L199-L211),
[`gateway/hosted_room_discussion.py:387-408`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/gateway/hosted_room_discussion.py#L387-L408)).
Whether a room message also becomes a `messages` row was not
determined. Read by the parser as `system` rows.

#### Conversation generations

`conversation_generations` (`source`, `session_key`, `generation`) in
`state.db`: one row per messaging routing peer, deliberately never
deleted, so it names every platform chat (`session_key` is
`agent:<profile>:<platform>:...`) that ever had a conversation after
its sessions are gone
([`hermes_state_common.py:642-669`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/hermes_state_common.py#L642-L669)).
No time column. A record of relay use, not of turns, with no text to
join on; read by the parser as `system` rows.

#### Desktop connection registry

The desktop app's `connections.json` (version 2) is `{version, primary,
launchMode, lastUsed, connections, quarantined?}`; each connection is
`{id, kind, label}` with `kind` one of `local`, `remote` (a `hermes
serve` gateway), `ssh` and `cloud` (Hermes Cloud), `url`, `authMode`,
`token` and `headers` for `remote` and `cloud`, `org` and `name` for
`cloud`, and `host`, `user`, `port`, `keyPath`, `remoteHermesPath` and
`remoteProfile` for `ssh`; exactly one `local` entry is always present,
and an entry that failed to load is kept under `quarantined` as
`{reason, entry}`
([`apps/desktop/electron/connection-registry.ts:19-20`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/apps/desktop/electron/connection-registry.ts#L19-L20),
[`apps/desktop/electron/connection-registry.ts:44-106`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/apps/desktop/electron/connection-registry.ts#L44-L106)).
It records which other Hermes hosts the app drove, not their sessions.
Read by the parser.

#### Debug-share paste list

`.hermes/pastes/pending.json`: the paste.rs links (paste.rs is a public, anonymous paste service)
that `hermes debug share`, the gateway's `/debug` and the dashboard's debug-share endpoint uploaded
and that are still due for deletion
([`hermes_cli/debug.py:38-39`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_cli/debug.py#L38-L39)).
A JSON list of `{"url": <string>, "expire_at": <epoch seconds, float>}`, `expire_at` the upload time
plus 21600 seconds (6 hours), merged by URL keeping the later time
([`hermes_cli/debug.py:156-166`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_cli/debug.py#L156-L166),
[`hermes_cli/debug.py:31`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_cli/debug.py#L31)).
An entry is dropped once its paste is deleted, or 24 hours after `expire_at` when the delete keeps
failing
([`hermes_cli/debug.py:59-89`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_cli/debug.py#L59-L89));
the gateway sweeps it hourly and every `hermes debug` call sweeps it first
([`gateway/run.py:4666-4670`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/gateway/run.py#L4666-L4670),
[`hermes_cli/debug.py:639-641`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_cli/debug.py#L639-L641)).
The file is missing or `[]` when nothing was shared in the last 6 to 30 hours. Links to dpaste.com
(the fallback paste service) are never written, since dpaste.com has no anonymous delete
([`hermes_cli/debug.py:158-160`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_cli/debug.py#L158-L160)).
Read by the parser ([Sync, relay and origin](#sync-relay-and-origin-rows)).

#### RetainDB queue

`.hermes/retaindb_queue.db`: the write-behind queue of the RetainDB
memory provider (a paid cloud memory service), present only when
`memory.provider` is `retaindb`
([`plugins/memory/retaindb/__init__.py:343-345`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/plugins/memory/retaindb/__init__.py#L343-L345)).
Table `pending` (`id`, `user_id`, `session_id`, `messages_json` holding
the turn's user and assistant text, `created_at` ISO 8601 UTC,
`last_error`); a row is deleted as soon as its upload succeeds, so the
table holds only turns not yet uploaded
([`plugins/memory/retaindb/__init__.py:194-198`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/plugins/memory/retaindb/__init__.py#L194-L198),
[`plugins/memory/retaindb/__init__.py:226-241`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/plugins/memory/retaindb/__init__.py#L226-L241)).
Not read by the parser.

### Format versions

`schema_version.version`, currently 31
([`hermes_state_common.py:283`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_state_common.py#L283));
migrations add columns in place
([`hermes_state_schema.py:1018-1059`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_state_schema.py#L1018-L1059)),
so a parser must select columns that exist (`PRAGMA table_info`). Gateway metadata was in
`sessions/sessions.json` before v18
([`hermes_state_schema.py:1308-1311`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_state_schema.py#L1308-L1311)).
The legacy `session_<id>.json` layout was not examined.

Drift to the 2026-10-19 HEAD: `SCHEMA_VERSION` is 32
([`hermes_state_common.py:289`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/hermes_state_common.py#L289)).
`sessions` gained `runtime_revision`, `runtime_generation` and
`transcript_revision`
([`hermes_state_common.py:445-513`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/hermes_state_common.py#L445-L513)),
`messages` and `session_model_usage` are unchanged
([`hermes_state_common.py:515-568`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/hermes_state_common.py#L515-L568)),
and four tables of the unified runtime are new: `runtime_epoch`,
`session_admissions` (each queued, steered or redirected request against
a session, with `payload_json` and `status`), `worker_executions` and
`worker_receipts`
([`hermes_state_common.py:570-622`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/hermes_state_common.py#L570-L622)).
Every file, table, column and constant the parser depends on was checked
at HEAD and is unchanged: `state.db`
([`hermes_state.py:180`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/hermes_state.py#L180)),
the `\x00json:` prefix
([`hermes_state.py:1627`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/hermes_state.py#L1627)),
the divert `sessions/<id>.jsonl`
([`hermes_state.py:434-441`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/hermes_state.py#L434-L441)),
`state-snapshots`
([`hermes_cli/backup.py:58`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/hermes_cli/backup.py#L58))
and `pastes/pending.json` with its 21600-second lifetime
([`hermes_cli/debug.py:32`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/hermes_cli/debug.py#L32),
[`hermes_cli/debug.py:43`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/hermes_cli/debug.py#L43)).
The parser reads `select *`, so the new columns do not affect it, and no drift affects the
parser. The behaviour changes are the deletion fences and the prune default
([What survives a delete](#what-survives-a-delete)) and the `local-` session ids
([Transcript stores](#transcript-stores)).

### Secrets

Tool-call `arguments` are stored unredacted on purpose, so `state.db` can hold credentials typed
into commands
([`agent/chat_completion_helpers.py:1640-1649`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/agent/chat_completion_helpers.py#L1640-L1649));
tool results, `api_content` and `system_prompts.prompt` (which embeds memory and context files) can
carry secrets. `request_dump_*` files are redacted before writing
([`agent_runtime_helpers.py:1629-1633`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/agent/agent_runtime_helpers.py#L1629-L1633)).
`sessions.origin_json` holds messaging user and chat identifiers.

### Parser plan

Inputs, under any Hermes home (a home's `.hermes`,
`.hermes_<suffix>`, `AppData/Local/hermes[_<suffix>]`, any of them
followed by `profiles/<name>`, or the root of a Docker or Podman named
volume's `_data`, which holds the Hermes home itself, see
[Volume homes](#volume-homes)), each marked for the timeline's
deduplication ([issue 122](https://github.com/seanthegeek/doubleagent/issues/122), see
[Secondary sources in `docs/output.md`](../analyzer/docs/output.md#secondary-sources)):

| Source | Kind | Why |
| --- | --- | --- |
| `state.db` (via `sqlite_util`, so the WAL is read) | primary | the store of record for every front end |
| `state-snapshots/<id>/state.db` | secondary | an older copy of `state.db`; read with the same reader after one `system` row `state snapshot <id>: older copy of state.db`, timestamped from the id's UTC time when the id starts with one, else empty; rows pair with the live rows exactly, and the copies several snapshots keep of a pruned session collapse to the first |
| `sessions/<id>.jsonl` | secondary | a divert of messages `state.db` did not take; `session_id` from the file name, no project path (the match is by session id), each line's own `timestamp` or none |
| `.hermes_history` | secondary | the prompt history ([Prompt history](#prompt-history)); one `user` row per entry at its time, a `system` row `slash command: <entry>` for an entry starting `/`, no session or project |
| `cron/output/<job>/<time>.md` | secondary | a cron run's prompt and response ([Cron run archive](#cron-run-archive)); a `system` row `cron run: job <job> (from cron/output, file name time, host-local)`, then the prompt as `user` and the response as `assistant`, or `cron run output: <document>` for a document with no prompt section, all at the file name's time, no session |
| `response_store.db` | secondary | the API server's history per response ([API server response store](#api-server-response-store)); per response by `created_at` a `system` row `api server response <id> status=<status> model=<model> conversation=<name> (from response_store.db responses, created_at)`, then the `conversation_history` messages no earlier response of the session held at the same position, read with the `state.db` message reader, with `data.session_id` and no time |
| `trajectory_samples.jsonl`, `failed_trajectories.jsonl` (project files) | secondary | ShareGPT runs ([Trajectory files](#trajectory-files)); per line a `system` row `trajectory: model=<model> completed=<completed> (from <file>, timestamp host-local)` at its `timestamp`, then `human` as `user`, `gpt` as `assistant`, `tool` as `tool_result`, `system` as `trajectory system prompt: <value>`, with the project directory and no time or session |
| `config.yaml` | settings | `command_allowlist` ([Permission rows](#permission-rows)) |
| `pairing/<platform>-approved.json`, `platforms/pairing/<platform>-approved.json` | settings | the gateway's approved users ([Permission rows](#permission-rows)) |
| `connections.json` in the desktop app's userData directory | settings | the remote backends ([Sync, relay and origin rows](#sync-relay-and-origin-rows)) |
| `sessions/<id>.json`, `session_<id>.json` | not read | no writer at the checked commit, shape unknown |
| `backups/*.zip`, `sessions/saved/*.json`, the stores other than `state.db` inside a snapshot, `connection.json` | not read | archives, exports and copies |

The snapshots, the diverts, the response store and the trajectory files
keep the default 2-second tolerance: their times are copies of the live
ones, not later stamps, or absent. `.hermes_history` and the cron run
files keep host-local times without a zone (the TUI's history entries
excepted), written as if UTC, so their window is 14 hours plus 2
seconds, which spans every zone offset. Rows: one per message; an assistant row
with N `tool_calls` yields one `assistant` row (when `content` is non-empty),
one `thinking` row (when `reasoning`/`reasoning_content` is set), and N
`tool_use` rows. Order by `timestamp, id`. Include `active=0` rows: they are
evidence of rewound or compacted turns.

| Column | Source |
| --- | --- |
| timestamp_utc | `messages.timestamp` (epoch s) |
| session_id | `messages.session_id` |
| project_path | `sessions.cwd`, else `git_repo_root` |
| git_branch | `sessions.git_branch` |
| turn_type | `role`: `user` → user, `assistant` → assistant, `tool` → tool_result, `system` → system; `tool_calls[]` → tool_use; reasoning → thinking |
| model | `sessions.model` |
| tool_name | `tool_calls[].function.name`, or `messages.tool_name` |
| tool_use_id | `tool_calls[].id` / `messages.tool_call_id` |
| text | `content` (strip `\x00json:` and join text parts); for tool_use the `command`/`path` key of `arguments`, else the arguments |
| source_line | `messages.id` |

The subsections that follow are written against HEAD
[`86fa9e6`](https://github.com/NousResearch/hermes-agent/commit/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872);
every row the subsections describe is implemented
([issue 180](https://github.com/seanthegeek/doubleagent/issues/180),
[issue 181](https://github.com/seanthegeek/doubleagent/issues/181),
[issue 182](https://github.com/seanthegeek/doubleagent/issues/182) and
[issue 208](https://github.com/seanthegeek/doubleagent/issues/208)), and all of it rests on source
alone; the per-step real-install round is still owed. `-` stands for an absent value in every row
text.

#### Permission rows

Implemented, source-derived; not observed on any real install. The records are under
[Permission outcomes and settings](#permission-outcomes-and-settings).

- After a `terminal` `tool_result` whose JSON content has `approval`, `permission decision:
  approved (<approval note>)`; with `status` `blocked`, `permission decision: blocked <error>`,
  followed by ` user_summary=<user_summary>` when a person was asked; with `status`
  `pending_approval`, `permission decision: pending pattern=<pattern_key>
  smart_denied=<true|false>`; each with the call's `tool_name`, `tool_use_id` and the result's
  time. A result that is not a JSON object gives no row.
- One `permission setting: yolo_mode=<true|false> (from state.db sessions.model_config,
  started_at)` row per session whose `model_config` has the key, at `started_at`.
- From `config.yaml`, one `permission setting: command_allowlist=<pattern> (from config.yaml,
  file mtime)` row per entry of the top-level block list, at the file's modification time. Only
  that shape is read, the block style of Hermes' round-trip YAML writer (`- <scalar>` items,
  plain, single- or double-quoted)
  ([`hermes_cli/config.py:2064`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/hermes_cli/config.py#L2064),
  [`hermes_yaml.py:89-98`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/hermes_yaml.py#L89-L98));
  any other construct under the key is one whole-file row `parser: config.yaml command_allowlist
  is not a flat list (<line>: <problem>)` after the entries before it. The nested `approvals`
  settings (`mode`, `cron_mode`, `single_query_mode`, `unattended_mode`, `deny`) need a YAML
  reader and are not read.
- From `pairing/<platform>-approved.json` or `platforms/pairing/<platform>-approved.json`
  (the newer location,
  [`gateway/pairing.py:58-59`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/gateway/pairing.py#L58-L59)),
  one row per user at `approved_at`, `permission setting: pairing.<platform>.<user_id>
  user_name=<user_name> (from pairing/<platform>-approved.json, approved_at)`, or `(from
  platforms/pairing/...` for the newer file. The pending files
  hold live pairing codes and are not read. Both files are collected flagged secret, so a
  `--no-secrets` collection gives none of these rows.

#### Model rows

Model rows ([issue 96](https://github.com/seanthegeek/doubleagent/issues/96)), `system`, after each
session's start row: the session's `session_model_usage` rows with `task` empty, ordered by
`first_seen`, give `model: <model>` for the first and
`model: <old> -> <new>` for each later one, at `first_seen`, with
` provider=<billing_provider>` when it is set; auxiliary task rows are
skipped. The table aggregates per model, so a switch back to a model
already used is not visible. A session with no such rows gets one
`model: <sessions.model>` row at `started_at`. The `model` column of the
other rows stays `sessions.model` ([Model usage](#model-usage)).

Model-switch markers ([issue 180](https://github.com/seanthegeek/doubleagent/issues/180)), `system`,
in message order: each `messages` row with `display_kind = 'model_switch'` is one row
`model: <old> -> <model> provider=<provider>` at the row's `timestamp`,
with `model` set to the new model and `source_line` to `messages.id`,
parsed from the marker text
([Model-switch markers](#model-switch-markers); the provider part only when one was set). The
marker names only the new model: `<old>` is the session's previous marker's
model, else the last main-loop `session_model_usage` model whose
`first_seen` is before the marker; with neither, or when it is the new
model, the row is `model: <model>` with the provider suffix. A later
switch sets the marker's `active` to 0, which is no rewind, so the row
has no `[rewound]` prefix, and a marker is never a `user` row. A marker
whose text has another form is a `system` row `model_switch: <text>`.
These rows sit beside the `session_model_usage` rows, which show
the first API call on each model rather than the switch.

Still proposed (model partly done): every other
non-empty `display_kind` on a `user` row becomes a `system` row
prefixed with the kind, so injected notices are not counted as typed
prompts.

#### Past-use rows and secondary sources

Implemented: the snapshots and the diverts are secondary sources (the source table of this
section), and imported sessions are read as Hermes sessions
([What survives a delete](#what-survives-a-delete)).

Implemented, source-derived; not observed on any real install:

- `.hermes_history`, the cron run files, `response_store.db` and the trajectory files are
  secondary sources, with the rows the source table gives. A history entry pairs with the
  session's `user` row on text (the TUI's within 2 seconds, the classic CLI's within the
  host's zone offset); a slash command, which no store repeats, is a `system` row. A cron run's
  prompt and response pair with the `source='cron'` session's turns when the text matches; a
  response-store message and a trajectory turn carry no time and pair on text within the session
  or the project.
- The start row of a session whose `origin_json` has `imported_from` ends ` | imported from
  <tool> <path> session <foreign_session_id>`, since its turns carry the import's time
  ([Imported sessions](#imported-sessions)).
- `conversation_generations`: one `system` row per row, `relay peer: <source> <session_key>
  generation <n> (from state.db conversation_generations)`, with no time and no session, which
  names chats whose sessions are gone.
- `hosted_room_events`: one `system` row per event at its `created_at`, `bot mode room <name>
  (<room_id>) seq <seq> <kind> actor <actor kind>:<actor id> (<display_name>): <text>` for a
  `message.user` or `message.member` event and the payload as compact JSON for the other kinds
  ([Bot Mode group chats](#bot-mode-group-chats)); never a `user` row, since whether a room
  message is also a `messages` row was not determined.

#### Sync, relay and origin rows

Sync: Hermes records no sync state for a session. Nothing uploads a
session, a transcript or a title to the Nous account: the Nous Portal
(Nous Research's account and billing site, `portal.nousresearch.com`)
is used for sign-in, inference and the tool gateway
([`hermes_cli/portal_cli.py:10-20`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_cli/portal_cli.py#L10-L20)),
and a grep of the tree for session upload, sync, share and cloud-session
names found nothing beyond the trace upload and the paths listed next. `hermes sessions delete`
removes the rows and the session's files and calls nothing remote
([`hermes_state_sessions.py:1606-1650`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_state_sessions.py#L1606-L1650)).

One command uploads a session, present at the cited commit and at HEAD: `hermes sessions export
--format trace --upload [--public]` converts a session to Claude Code
JSONL and uploads it to the user's Hugging Face dataset
`<user>/hermes-traces` as `sessions/<session id>.jsonl`, private unless
`--public`, and writes nothing locally on success
([`agent/trace_upload.py:1-5`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/agent/trace_upload.py#L1-L5),
[`agent/trace_upload.py:209-221`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/agent/trace_upload.py#L209-L221),
[`hermes_cli/sessions_cmd.py:463-482`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/hermes_cli/sessions_cmd.py#L463-L482));
the only local traces are the command in the shell's own history and
`HF_TOKEN` in `.env`, so no `session synced:` row is possible.

Content also leaves the host on these opt-in paths, none of which marks a
session:

- `hermes debug share` (CLI), `/debug` (gateway) and the dashboard's
  `POST /api/ops/debug-share` upload a report and the logs, whose own
  notice says they hold "Verbatim content of your recent messages", to
  paste.rs, falling back to dpaste.com
  ([`hermes_cli/debug.py:96-113`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_cli/debug.py#L96-L113),
  [`hermes_cli/debug.py:442-479`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_cli/debug.py#L442-L479),
  [`gateway/slash_commands.py:1213-1243`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/gateway/slash_commands.py#L1213-L1243),
  [`hermes_cli/web_routers/status.py:792-803`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_cli/web_routers/status.py#L792-L803)).
  The CLI asks first unless `--yes` is given. The paste.rs links stay in
  `pastes/pending.json` for 6 to 30 hours
  ([Debug-share paste list](#debug-share-paste-list)), and with redaction on (the default)
  `agent.log` gets the line `hermes debug share: applied force-mode
  redaction to log snapshots before upload`
  ([`hermes_cli/debug.py:451-453`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_cli/debug.py#L451-L453)).
  `hermes debug share --nous` sends the same bundle to Nous's private
  diagnostics storage (an S3 bucket behind `portal.nousresearch.com`,
  kept 14 days) and only prints the view URL; its local trace is the log
  line `hermes debug share --nous: applied force-mode redaction before
  upload`
  ([`hermes_cli/diagnostics_upload.py:1-13`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_cli/diagnostics_upload.py#L1-L13),
  [`hermes_cli/debug.py:578-605`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_cli/debug.py#L578-L605)).
  `hermes debug delete <url>` deletes a paste.rs paste and does not
  touch `pending.json`
  ([`hermes_cli/debug.py:616-636`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_cli/debug.py#L616-L636)).
- An external memory provider, set in `config.yaml` `memory.provider`
  (empty by default;
  [`hermes_cli/config_defaults.py:1325-1328`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_cli/config_defaults.py#L1325-L1328)),
  receives every completed turn's user and assistant text with the
  session id
  ([`agent/memory_manager.py:535-557`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/agent/memory_manager.py#L535-L557)):
  Mem0 (a hosted memory service, or self-hosted) at
  [`plugins/memory/mem0/__init__.py:302-313`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/plugins/memory/mem0/__init__.py#L302-L313),
  RetainDB through `retaindb_queue.db`
  ([`plugins/memory/retaindb/__init__.py:405-412`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/plugins/memory/retaindb/__init__.py#L405-L412),
  [RetainDB queue](#retaindb-queue)), ByteRover and OpenViking (local
  servers with an optional cloud), and the catalog-installed Honcho,
  Hindsight and Supermemory, whose code is not in this repository. The
  local traces are the provider name in `config.yaml`, its settings
  (`mem0.json`, `honcho.json`) and its key in `.env`; no file records
  which turns were sent, and deleting a session does not delete the
  provider's copy.
- The Langfuse plugin (Langfuse is an LLM tracing service, hosted or
  self-hosted), off unless enabled in `plugins.enabled` with keys in
  `.env`, exports prompts, replies and tool payloads, redacted and
  truncated by default, and writes no local file
  ([`plugins/observability/langfuse/__init__.py:1-7`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/plugins/observability/langfuse/__init__.py#L1-L7)).
- Shared metrics (`config.yaml` `telemetry.shared_metrics.enabled` and
  `.send`, both off by default) are counters only, "Never prompts,
  files, paths", kept in `telemetry/shared_metrics/` and sent daily to
  `telemetry.nousresearch.com` when `send` is on
  ([`hermes_cli/config_defaults.py:2337-2349`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_cli/config_defaults.py#L2337-L2349),
  [`hermes_cli/observability/shared_metrics_consent.py:29-36`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_cli/observability/shared_metrics_consent.py#L29-L36),
  [`hermes_cli/observability/shared_metrics.py:191-193`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_cli/observability/shared_metrics.py#L191-L193)).

Implemented: the parser yields no `session synced:`
row and leaves `records_sync` off. It reads `pastes/pending.json` and
yields one `system` row per entry, `debug report uploaded: <url> (from
pastes/pending.json, expire_at minus 6 hours)`, timestamped `expire_at`
minus 21600 seconds with an empty `session_id`, which shows that logs
left the host in the last day and marks no session; bad entries are under
[Bad-line rows](#bad-line-rows). Fixture only: no real install has shown the file.

Relay: a gateway session's messages also live on the messaging platform, which
is the channel the user typed in, not a sync; `sessions.chat_id`,
`origin_json` and `messages.platform_message_id` name them there
([Messaging sessions](#messaging-sessions)). Implemented, source-derived; not observed on any
real install: a session whose `source` is a messaging platform (every gateway platform but
`local` and `api_server`), or that has a `chat_id` and another `source` than those two (a plugin
platform), gets `session relayed: <source> chat <chat_id> type <chat_type> thread <thread_id>
user <user_id> name <user_name> display <display_name> (from state.db sessions, started_at)`
after its model rows, `user_name` from `origin_json` and `display_name`, else `origin_json`
`chat_name`; the parser sets `records_relay`.

Origin: the desktop app's `connections.json` registry lists `local`,
`remote` (a `hermes serve` the user runs), `ssh` and `cloud` (Hermes
Cloud, Nous Research's hosted Hermes) connections; a session on a
remote, SSH or cloud backend lives in that host's `state.db`, and
whether the desktop keeps any local copy was not determined
([`apps/desktop/electron/main.ts:1349-1354`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/apps/desktop/electron/main.ts#L1349-L1354),
[`apps/desktop/electron/connection-registry.ts:1-10`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/apps/desktop/electron/connection-registry.ts#L1-L10),
[`apps/desktop/electron/connection-registry.ts:44-47`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/apps/desktop/electron/connection-registry.ts#L44-L47)).
Implemented, source-derived; not observed on any real install: the parser reads
`connections.json` ([Desktop connection registry](#desktop-connection-registry)) and yields one
`system` row per connection that is not `local`, `remote backend: <kind> <label> <url, or
user@host:port for ssh> (from connections.json, file mtime)` followed by ` org=<org>`,
` name=<name>` and ` profile=<remoteProfile>` when recorded, and `remote backend quarantined by
the app: <reason> (from connections.json, file mtime)` per quarantined entry, with no session
(`session origin:` needs one) and never printing `token` or `headers`. It sets no
`records_origin`. The single-connection `connection.json` is not read.

#### Scratch directory rows

The scratch directories are `cache/scratch/` and `sandboxes/` under the Hermes home and the git
worktrees under `<repo>/.worktrees/` ([Scratch directories](#scratch-directories)).

Implemented: `cache/scratch` and `sandboxes` are
under the catalog's Hermes home, so the analyzer's catalog rule
classes any path under it as a scratch directory. A `.worktrees/<name>` path is not under
a home entry. Implemented, source-derived; not observed on any real install: a session whose
`cwd` is `<repo>/.worktrees/<name>` and whose `git_branch` starts with `hermes/` or
`hermes-subagent/` gets `scratch directory: <cwd> (git worktree the agent created in <repo>,
branch <git_branch>)` after its model rows, at `started_at`. A kanban child's worktree, whose
branch the source does not fix, gets none.

#### Deletion rows

Implemented, source-derived; not observed on any real install (the markers are new since the
cited commit): one `session deleted: <id> (from state.db state_meta gateway.retired_session.v1,
retired_at)` row per marker, at `retired_at`, with ` sweep` after the id when it has `expires`,
and with the marker's id as `session_id`; the start row of an archived session ends
` | archived=1`. The markers are described under
[What survives a delete](#what-survives-a-delete). The `gateway.retired_route.v1.` fences a
multiplexed gateway writes for other profiles are not read.

#### Bad-line rows

`sessions/<id>.jsonl` goes through `iter_jsonl` and `error_rows`. In `pastes/pending.json` a file
that does not parse or is not a JSON list is one `parser:` row, and an entry that is not an object
with a `url` is its own `parser: unparseable record <N>` row carrying the entry.

Source-derived, for the sources of [issue 208](https://github.com/seanthegeek/doubleagent/issues/208): in `state.db` a `sessions.origin_json` or
`model_config`, a `state_meta` fence value, or a `hosted_room_events` `actor_json` or
`payload_json` that does not decode to a JSON object is a row `parser: unparseable record
<rowid> (<table>.<column>: <reason>): <value as stored>`, with the session's id where there is
one, after the file's other rows. In `response_store.db` a `data` value is reported the same way
(`responses.data`), and a history entry that is not an object as `(responses.data
conversation_history <index>: not an object)`. `.hermes_history` goes through the Aider parser's
`read_input_history`, whose bad line is `parser: unparseable line (not a FileHistory line):
<line>`; a trajectory file's line that does not decode, is not an object or has no
`conversations` list is `parser: unparseable line (<reason>): <line>`. A pairing file or
`connections.json` that does not parse is one whole-file row and an entry that is not an object
is its own `unparseable record <N> (not an object)` row; `config.yaml` is under
[Permission rows](#permission-rows).

Malformed `tool_calls` ([issue 182](https://github.com/seanthegeek/doubleagent/issues/182)): a value
that is not valid JSON, is not a list (a lone object is read as a one-call list) or holds an entry
that is not an object gives one row through `error_rows`, after the
message's other rows, `parser: unparseable record <messages.id>
(tool_calls is not valid JSON: <reason>)`, `(tool_calls is not a
list)` or `(tool_calls entry is not an object)`, followed by `: ` and
the value as stored, with the message's `session_id` and project path
and an empty `timestamp_utc`; the entries that are objects still give
`tool_use` rows. A divert line uses the same rule, with `<N>` the line
number and the value written as compact JSON when it is not a string.

#### Fixtures

```sql
CREATE TABLE schema_version (version INTEGER NOT NULL);
INSERT INTO schema_version VALUES (31);
CREATE TABLE sessions (id TEXT PRIMARY KEY, source TEXT NOT NULL, model TEXT,
  parent_session_id TEXT, started_at REAL NOT NULL, ended_at REAL, end_reason TEXT,
  cwd TEXT, git_branch TEXT, git_repo_root TEXT, title TEXT);
CREATE TABLE messages (id INTEGER PRIMARY KEY AUTOINCREMENT, session_id TEXT NOT NULL,
  role TEXT NOT NULL, content TEXT, tool_call_id TEXT, tool_calls TEXT, tool_name TEXT,
  timestamp REAL NOT NULL, finish_reason TEXT, reasoning TEXT, reasoning_content TEXT,
  active INTEGER NOT NULL DEFAULT 1, compacted INTEGER NOT NULL DEFAULT 0);
INSERT INTO sessions VALUES ('20261001_120000_a1b2c3d4','cli','anthropic/claude-sonnet-4',NULL,
  1759320000.0,NULL,NULL,'/home/u/repo','main','/home/u/repo','List files');
INSERT INTO messages (session_id,role,content,tool_call_id,tool_calls,tool_name,timestamp,finish_reason,reasoning) VALUES
 ('20261001_120000_a1b2c3d4','user','list files',NULL,NULL,NULL,1759320001.25,NULL,NULL),
 ('20261001_120000_a1b2c3d4','assistant','',NULL,
  '[{"id":"call_1","call_id":"call_1","response_item_id":null,"type":"function","function":{"name":"terminal","arguments":"{\"command\":\"ls\"}"}}]',
  NULL,1759320002.5,'tool_calls','User wants a listing.'),
 ('20261001_120000_a1b2c3d4','tool','a.txt\nb.txt','call_1',NULL,'terminal',1759320003.0,NULL,NULL),
 ('20261001_120000_a1b2c3d4','assistant','Two files: a.txt, b.txt.',NULL,NULL,NULL,1759320004.0,'stop',NULL);
```

Source-derived samples for the rows of [issue 208](https://github.com/seanthegeek/doubleagent/issues/208), at
[`86fa9e6`](https://github.com/NousResearch/hermes-agent/commit/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872)
(the shapes are cited in [Record schema](#record-schema) and [Other stores](#other-stores)):

```sql
INSERT INTO sessions (id,source,user_id,chat_id,chat_type,display_name,origin_json,model_config,
  started_at,archived) VALUES ('20261001_180000_7e1e9a40','telegram','42','-100123','group',
  'Ops chat','{"platform":"telegram","chat_id":"-100123","chat_name":"Ops chat",
  "chat_type":"group","user_id":"42","user_name":"alice_tg"}','{"yolo_mode": true}',
  1790877600.0,1);
INSERT INTO messages (session_id,role,content,tool_call_id,tool_name,timestamp) VALUES
 ('20261001_180000_7e1e9a40','tool','{"output": "", "exit_code": -1, "error": "BLOCKED by smart
  approval: deletes system files", "status": "blocked"}','call_a','terminal',1790877603.0);
INSERT INTO state_meta VALUES ('gateway.retired_session.v1.20260915_090000_0ddba11',
  '{"retired_at": 1790880000.5, "own": true}');
INSERT INTO conversation_generations VALUES ('telegram','agent:main:telegram:group:-100123',2);
```

```text
.hermes_history:

# 2026-10-01 12:00:01.250
+list files

# 2026-10-01 14:00:05.123456
+/model openai/gpt-5
```

```json
{"42": {"user_name": "alice_tg", "approved_at": 1790856000.0}}
```

#### Volume homes

[Issue 181](https://github.com/seanthegeek/doubleagent/issues/181): the catalog's `hermes|*hermes*`
volume line collects a named volume whole with its `_data` directory as the base.
The container's Hermes home is `/opt/data` ([Docker](#docker)),
so a named volume mounted there holds the Hermes home itself and the
files arrive as `state.db`, `sessions/<id>.jsonl`,
`state-snapshots/<id>/state.db`, `pastes/pending.json` and
`profiles/<name>/...` with no prefix (`state.db`, not `.hermes/state.db`). `wants()` accepts
these as it accepts the home forms, and the rows carry the manifest's `docker` user.

#### Source host rows

The agent records no source-host integration: at
[`86fa9e6`](https://github.com/NousResearch/hermes-agent/commit/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872)
a grep for `github`, `gitlab`, `bitbucket`, `oauth`, `installation`, `repositor`, `pull request`
and `clone` found no connected account, no cached repository list and no repository-bound remote
conversation. A GitHub token, when present, is a variable in `.env` or the gh CLI's own login
([Credentials](#credentials)), and the webhook platform's GitHub routes live in `config.yaml`
`platforms.webhook.extra.routes`
([`gateway/platforms/webhook.py:1-10`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/gateway/platforms/webhook.py#L1-L10));
a webhook-driven session is a relayed session with `source` `webhook`.

## Confidence

- High: home resolution, profile layout, `state.db` and every file named in
  [Per-user storage](#per-user-storage) and [Credentials](#credentials) (source constants).
- High: the backup exclusion list as a guide to size.
- High: table and column names, timestamp unit, content prefix, tool-call shape, joins, WAL.
- Medium: the desktop `userData` path, which follows the Electron default from `productName`
  rather than an explicit `setPath` for stable builds
  ([`apps/desktop/electron/product-identity.ts:26-47`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/apps/desktop/electron/product-identity.ts#L26-L47)).
- Medium: the full set of `role` values (only the four of the [Messages table](#messages-table)
  seen in queries) and of `source` values beyond those listed in the
  [Sessions table](#sessions-table) and [Messaging sessions](#messaging-sessions).
- Not determined: the exact sizes of `state-snapshots` and `cache/scratch` on a real install.
- Not determined: the format of `auth.json` beyond "OAuth provider credentials"
  ([`configuration.md:25`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/website/docs/user-guide/configuration.md#L25)).
- Not determined: Termux installs, which come from APT
  ([`scripts/install.sh:340`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/scripts/install.sh#L340))
  and were not checked.
- Not determined: the legacy `session_<id>.json` schema, which nothing at the cited commit writes
  ([Transcript stores](#transcript-stores)).
- Medium: that a `terminal` result's JSON reaches `messages.content` unchanged; a result Hermes
  spilled or truncated before storing would give no permission decision row.
- Not determined: whether a Bot Mode room message is also a `messages` row, whether a cron run's
  stored prompt equals the session's first `user` message, and the branch of a kanban child's
  worktree.
