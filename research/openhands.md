# openhands

OpenHands is an open-source coding agent, now split across several MIT-licensed repositories, all
open source; `All-Hands-AI/OpenHands` redirects to `OpenHands/OpenHands`. Its surfaces are Agent
Canvas (a React front end with a local-stack launcher, also shipped as an Electron app), the Agent
Server and the SDK behind it, the `openhands` terminal CLI, and the pre-Canvas Python app, whose
legacy layouts share `~/.openhands`. Catalog agent `openhands`. Current OpenHands (Agent Canvas and
the `openhands` CLI) persists conversations through the Software Agent SDK as one JSON file per
event; the legacy 1.x and 0.x apps used other layouts ([Format versions](#format-versions)). Parsed:
the openhands parser reads the SDK event files with their `meta.json` and `base_state.json`.

## Source and evidence

- [OpenHands/OpenHands](https://github.com/OpenHands/OpenHands) at
  [`a6bba78ffd5a8b31620770f52383b1a2c0477fcd`](https://github.com/OpenHands/OpenHands/commit/a6bba78ffd5a8b31620770f52383b1a2c0477fcd)
  (2026-10-03): now only Agent Canvas, the React frontend plus the local-stack launcher; backend,
  conversations and events belong to the SDK
  ([`AGENTS.md:31-35`](https://github.com/OpenHands/OpenHands/blob/a6bba78ffd5a8b31620770f52383b1a2c0477fcd/AGENTS.md#L31-L35)).
- [OpenHands/software-agent-sdk](https://github.com/OpenHands/software-agent-sdk) at
  [`b347047e2dcdd8f4b2be810aa8bc6632bc756d41`](https://github.com/OpenHands/software-agent-sdk/commit/b347047e2dcdd8f4b2be810aa8bc6632bc756d41):
  the SDK and the Agent Server, which owns every file under `~/.openhands` that matters.
- [OpenHands/OpenHands-CLI](https://github.com/OpenHands/OpenHands-CLI) at
  [`954f2ba646e8d749261a8f2b2b7e3031fa39be9f`](https://github.com/OpenHands/OpenHands-CLI/commit/954f2ba646e8d749261a8f2b2b7e3031fa39be9f):
  the `openhands` terminal CLI, "no longer actively maintained"
  ([`README.md:31-35`](https://github.com/OpenHands/OpenHands-CLI/blob/954f2ba646e8d749261a8f2b2b7e3031fa39be9f/README.md#L31-L35))
  but still installed in the field.
- The pre-Canvas Python app, read at tags `1.11.0`
  ([`11ca68ab2e15dcd85c21e4d7d3409e7a259369ac`](https://github.com/OpenHands/OpenHands/commit/11ca68ab2e15dcd85c21e4d7d3409e7a259369ac))
  and `0.62.0`
  ([`7fbb48c40679afd674970966b96185657d92a487`](https://github.com/OpenHands/OpenHands/commit/7fbb48c40679afd674970966b96185657d92a487)),
  for legacy layouts that share `~/.openhands`.

All claims are from source.

### Observed

- No real install.

### Source reviews

- 2026-10-21: OpenHands/software-agent-sdk at
  [`33d8aa2`](https://github.com/OpenHands/software-agent-sdk/commit/33d8aa29c3078d074799e2fbf5e847aeb2d71360),
  OpenHands/OpenHands-CLI at
  [`0403f1a`](https://github.com/OpenHands/OpenHands-CLI/commit/0403f1a79a9fc62a6c7be21c17d7b51f64245ad2)
  and OpenHands/OpenHands at
  [`8d39305`](https://github.com/OpenHands/OpenHands/commit/8d39305538c1d17a30cbe477e57a0e18fcee7d95):
  the Parser plan now describes the permission, model, prompt-history and scratch rows the parser
  writes from source, the `mars-credentials.json` secret globs and the `.openhands/workspaces`
  exclusion are in the tables, and [Source host rows](#source-host-rows) states the GitHub
  repository search and its token.
- 2026-10-19: SDK at
  [`33d8aa2`](https://github.com/OpenHands/software-agent-sdk/commit/33d8aa29c3078d074799e2fbf5e847aeb2d71360)
  (previously
  [`b347047`](https://github.com/OpenHands/software-agent-sdk/commit/b347047e2dcdd8f4b2be810aa8bc6632bc756d41)
  and
  [`fd26808`](https://github.com/OpenHands/software-agent-sdk/commit/fd268089e257bee4ad704ce4fdc6466bc756c71c)),
  Agent Canvas at
  [`8d39305`](https://github.com/OpenHands/OpenHands/commit/8d39305538c1d17a30cbe477e57a0e18fcee7d95)
  (previously
  [`a6bba78`](https://github.com/OpenHands/OpenHands/commit/a6bba78ffd5a8b31620770f52383b1a2c0477fcd)),
  CLI at
  [`0403f1a`](https://github.com/OpenHands/OpenHands-CLI/commit/0403f1a79a9fc62a6c7be21c17d7b51f64245ad2)
  (previously
  [`954f2ba`](https://github.com/OpenHands/OpenHands-CLI/commit/954f2ba646e8d749261a8f2b2b7e3031fa39be9f);
  unchanged since 2026-10-09), from source with no install. It added the permission and model
  records of [Record schema](#record-schema), the Agent Canvas browser storage of
  [Other stores](#other-stores), the DigitalOcean credential store, the scratch directories and the
  delete behaviour, and corrected an earlier statement that the Canvas front end is not in these
  repositories.
- 2026-10-17: SDK at
  [`fd26808`](https://github.com/OpenHands/software-agent-sdk/commit/fd268089e257bee4ad704ce4fdc6466bc756c71c)
  and CLI at
  [`0403f1a`](https://github.com/OpenHands/OpenHands-CLI/commit/0403f1a79a9fc62a6c7be21c17d7b51f64245ad2),
  for sync and upload. It added the finding of
  [Sync, relay and origin rows](#sync-relay-and-origin-rows) that nothing local records a synced
  conversation.

## Collection

### Per-user storage

One root, `~/.openhands`, on every OS: Python `Path.home() / ".openhands"`
([`openhands-sdk/openhands/sdk/utils/path.py:32-59`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/utils/path.py#L32-L59))
and Node `homedir()`
([`scripts/dev-safe.mjs:80-102`](https://github.com/OpenHands/OpenHands/blob/a6bba78ffd5a8b31620770f52383b1a2c0477fcd/scripts/dev-safe.mjs#L80-L102)),
so `%USERPROFILE%\.openhands` on Windows. `OH_PERSISTENCE_DIR` replaces it (SDK); the CLI uses
`OPENHANDS_PERSISTENCE_DIR` and `OPENHANDS_CONVERSATIONS_DIR`
([`openhands_cli/locations.py:5-23`](https://github.com/OpenHands/OpenHands-CLI/blob/954f2ba646e8d749261a8f2b2b7e3031fa39be9f/openhands_cli/locations.py#L5-L23));
the 1.x app also honoured `FILE_STORE_PATH`
([`openhands/app_server/config.py:75-89`](https://github.com/OpenHands/OpenHands/blob/11ca68ab2e15dcd85c21e4d7d3409e7a259369ac/openhands/app_server/config.py#L75-L89)).

#### Scratch directories

OpenHands has three, all chosen by the agent rather than the user:

- `~/.openhands/agent-canvas/workspaces/<conversation hex>/`, the working directory of a Canvas
  conversation opened on no project
  ([`scripts/dev-safe.mjs:648-649`](https://github.com/OpenHands/OpenHands/blob/8d39305538c1d17a30cbe477e57a0e18fcee7d95/scripts/dev-safe.mjs#L648-L649),
  [`703`](https://github.com/OpenHands/OpenHands/blob/8d39305538c1d17a30cbe477e57a0e18fcee7d95/scripts/dev-safe.mjs#L703)).
  It also holds automation run workspaces when Canvas runs natively
  ([`dev-safe.mjs:649`](https://github.com/OpenHands/OpenHands/blob/a6bba78ffd5a8b31620770f52383b1a2c0477fcd/scripts/dev-safe.mjs#L649);
  [`scripts/dev-with-automation.mjs:1064-1069`](https://github.com/OpenHands/OpenHands/blob/a6bba78ffd5a8b31620770f52383b1a2c0477fcd/scripts/dev-with-automation.mjs#L1064-L1069);
  [`src/api/agent-server-config.ts:203-211`](https://github.com/OpenHands/OpenHands/blob/a6bba78ffd5a8b31620770f52383b1a2c0477fcd/src/api/agent-server-config.ts#L203-L211)).
  Excluded by default ([Exclusions](#exclusions)).
- `~/.openhands/workspaces/<run>/`, automation runs under Canvas-in-Docker ([Automation
  workspaces](#automation-workspaces)). Excluded by default ([Exclusions](#exclusions)).
- `/tmp/conversation-worktrees/<dir>/`, the git worktree the Agent Server makes for a conversation
  started with worktree isolation
  ([`openhands-agent-server/openhands/agent_server/config.py:286-293`](https://github.com/OpenHands/software-agent-sdk/blob/33d8aa29c3078d074799e2fbf5e847aeb2d71360/openhands-agent-server/openhands/agent_server/config.py#L286-L293);
  the same default at
  [`b347047`](https://github.com/OpenHands/software-agent-sdk/commit/b347047e2dcdd8f4b2be810aa8bc6632bc756d41):
  [`config.py:256-281`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-agent-server/openhands/agent_server/config.py#L256-L281)).
  Outside the home, so no catalog line can collect it; the conversation records it as
  `workspace.working_dir`
  ([Where the project path is recorded](#where-the-project-path-is-recorded)), and Canvas records
  the choice in its browser storage ([Other stores](#other-stores)).

The first two are under the `.openhands` catalog entry, so the analyzer's catalog rule classes them
as scratch directories; the third is not.

#### Agent Canvas

Agent Canvas (`agent-canvas`, `npm install -g @openhands/agent-canvas`, or the Electron app) runs
the Agent Server with `OH_PERSISTENCE_DIR=~/.openhands` and its own state in
`~/.openhands/agent-canvas` (`OH_CANVAS_SAFE_STATE_DIR`)
([`scripts/dev-safe.mjs:643-703`](https://github.com/OpenHands/OpenHands/blob/a6bba78ffd5a8b31620770f52383b1a2c0477fcd/scripts/dev-safe.mjs#L643-L703),
[`817-821`](https://github.com/OpenHands/OpenHands/blob/a6bba78ffd5a8b31620770f52383b1a2c0477fcd/scripts/dev-safe.mjs#L817-L821);
[`bin/agent-canvas.mjs:157-166`](https://github.com/OpenHands/OpenHands/blob/a6bba78ffd5a8b31620770f52383b1a2c0477fcd/bin/agent-canvas.mjs#L157-L166)):

- `agent-canvas/dev_conversations/<conversation hex>/` when run natively,
  `agent-canvas/conversations/<hex>/` when Canvas runs in its Docker image
  ([`docker/entrypoint.sh:174-178`](https://github.com/OpenHands/OpenHands/blob/a6bba78ffd5a8b31620770f52383b1a2c0477fcd/docker/entrypoint.sh#L174-L178);
  [`config/defaults.json:23-26`](https://github.com/OpenHands/OpenHands/blob/a6bba78ffd5a8b31620770f52383b1a2c0477fcd/config/defaults.json#L23-L26)).
  Each holds `meta.json`, `base_state.json`, `events/event-NNNNN-<id>.json` and `bash_events/`
  ([Where the project path is recorded](#where-the-project-path-is-recorded), and
  [Transcript stores](#transcript-stores)).
- `agent-canvas/bash_events/`: commands typed in the Canvas terminal, one file per event named
  `<YYYYMMDDHHMMSSffffff>_<kind>_...`
  ([`openhands-agent-server/openhands/agent_server/bash_service.py:61-72`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-agent-server/openhands/agent_server/bash_service.py#L61-L72)).
- `agent-canvas/workspaces/<conversation hex>/`: the scratch working directories of
  [Scratch directories](#scratch-directories)
  ([`dev-safe.mjs:703`](https://github.com/OpenHands/OpenHands/blob/a6bba78ffd5a8b31620770f52383b1a2c0477fcd/scripts/dev-safe.mjs#L703)).
- `agent-canvas/logs/agent-canvas.%DATE%.log`
  ([`scripts/logger.mjs:23-26`](https://github.com/OpenHands/OpenHands/blob/a6bba78ffd5a8b31620770f52383b1a2c0477fcd/scripts/logger.mjs#L23-L26),[`64`](https://github.com/OpenHands/OpenHands/blob/a6bba78ffd5a8b31620770f52383b1a2c0477fcd/scripts/logger.mjs#L64));
  `agent-canvas/storage/`
  ([`dev-with-automation.mjs:600-608`](https://github.com/OpenHands/OpenHands/blob/a6bba78ffd5a8b31620770f52383b1a2c0477fcd/scripts/dev-with-automation.mjs#L600-L608));
  `agent-canvas/tmux/` sockets
  ([`dev-safe.mjs:682-698`](https://github.com/OpenHands/OpenHands/blob/a6bba78ffd5a8b31620770f52383b1a2c0477fcd/scripts/dev-safe.mjs#L682-L698)).
- `automation/automations.db`, SQLite, the automation service's schedules and run history
  ([`config/defaults.json:26`](https://github.com/OpenHands/OpenHands/blob/a6bba78ffd5a8b31620770f52383b1a2c0477fcd/config/defaults.json#L26);
  [`dev-with-automation.mjs:1047-1048`](https://github.com/OpenHands/OpenHands/blob/a6bba78ffd5a8b31620770f52383b1a2c0477fcd/scripts/dev-with-automation.mjs#L1047-L1048)).

#### Agent Server stores

Directly under `~/.openhands`
([`openhands-agent-server/openhands/agent_server/persistence/store.py:802-822`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-agent-server/openhands/agent_server/persistence/store.py#L802-L822)):
`settings.json`
([`store.py:306-314`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-agent-server/openhands/agent_server/persistence/store.py#L306-L314)),
`secrets.json`
([`444-450`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-agent-server/openhands/agent_server/persistence/store.py#L444-L450)),
`workspaces.json`
([`725-742`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-agent-server/openhands/agent_server/persistence/store.py#L725-L742)),
`provider-connections/provider_connections.json`
([`895-913`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-agent-server/openhands/agent_server/persistence/store.py#L895-L913);
[`openhands-sdk/openhands/sdk/llm/provider_connection_store.py:49-50`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/llm/provider_connection_store.py#L49-L50)),
`profiles/<name>.json` LLM profiles
([`openhands-sdk/openhands/sdk/llm/llm_profile_store.py:28`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/llm/llm_profile_store.py#L28)),
`agent-profiles/`
([`openhands-sdk/openhands/sdk/profiles/agent_profile_store.py:29`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/profiles/agent_profile_store.py#L29)),
`auth/`
([`openhands-sdk/openhands/sdk/llm/auth/credentials.py:21-28`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/llm/auth/credentials.py#L21-L28)),
`memory/MEMORY.md`
([`openhands-sdk/openhands/sdk/context/memory.py:23`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/context/memory.py#L23)),
`skills/`, `skills/installed/`, `microagents/`
([`openhands-agent-server/openhands/agent_server/skills_service.py:6-10`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-agent-server/openhands/agent_server/skills_service.py#L6-L10)),
`plugins/installed/`
([`openhands-sdk/openhands/sdk/plugin/installed.py:23`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/plugin/installed.py#L23)),
`cache/skills/`, `cache/plugins/`
([`openhands-sdk/openhands/sdk/skills/fetch.py:20`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/skills/fetch.py#L20);
[`openhands-sdk/openhands/sdk/plugin/fetch.py:20`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/plugin/fetch.py#L20)).

#### Docker and Podman volume

In a Docker or Podman volume the catalog's `openhands|*openhands*` line matches, the LLM profiles
`profiles/<name>.json` are flagged by the volume-relative glob `profiles/*.json`
([`openhands-sdk/openhands/sdk/llm/llm_profile_store.py:28`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/llm/llm_profile_store.py#L28)).
A save writes the profile first to a temporary `profiles/tmp<random>.tmp` and then renames it, so
an interrupted save leaves that file holding the profile; `profiles/tmp*.tmp` flags it
([`llm_profile_store.py:240-249`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/llm/llm_profile_store.py#L240-L249)).

#### Docker runtime

The Docker runtime (`OH_CONVERSATION_RUNTIME=docker`, one container per conversation,
[`README.md:107-117`](https://github.com/OpenHands/OpenHands/blob/a6bba78ffd5a8b31620770f52383b1a2c0477fcd/README.md#L107-L117))
keeps `runtime-control/<hex>.json` (per-conversation identity) and `runtime-data/<hex>/persistence/`
under `~/.openhands`
([`openhands-agent-server/openhands/agent_server/docker_runtime/provisioning.py:54-68`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-agent-server/openhands/agent_server/docker_runtime/provisioning.py#L54-L68)).
The container gets the host conversation directory, that persistence directory (as its `HOME`) and
the host workspace as bind mounts
([`docker_runtime/registry.py:40-42`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-agent-server/openhands/agent_server/docker_runtime/registry.py#L40-L42),[`354-404`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-agent-server/openhands/agent_server/docker_runtime/registry.py#L354-L404)),
so transcripts and the container user's dotfiles land on the host. Only what the agent writes
outside those mounts (container `/tmp`, packages it installs) stays in the container and is lost
with it. Canvas-in-Docker likewise bind-mounts host `~/.openhands` to `/home/openhands/.openhands`
([`README.md:93-101`](https://github.com/OpenHands/OpenHands/blob/a6bba78ffd5a8b31620770f52383b1a2c0477fcd/README.md#L93-L101)).
Git worktrees go outside the home ([Scratch directories](#scratch-directories)).

#### Electron app

Product name `OpenHands Agent Canvas`
([`electron/package.json:3`](https://github.com/OpenHands/OpenHands/blob/a6bba78ffd5a8b31620770f52383b1a2c0477fcd/electron/package.json#L3)),
so Electron's default user data directory (`~/Library/Application Support/OpenHands Agent Canvas`,
`~/.config/OpenHands Agent Canvas`, `%APPDATA%\OpenHands Agent Canvas`) holds the frontend's
localStorage; the backend state is the same `~/.openhands`. At Canvas
[`8d39305`](https://github.com/OpenHands/OpenHands/commit/8d39305538c1d17a30cbe477e57a0e18fcee7d95)
it also holds `mars-credentials.json`, the store of DigitalOcean connections for Managed Agents
sessions (MARS, DigitalOcean's name for its hosted agent sandboxes: a microVM that runs its own
Agent Server with its own disk and conversations, so those conversations stay on DigitalOcean;
the file's fields are in [Credentials](#credentials))
([`electron/main.mjs:773-777`](https://github.com/OpenHands/OpenHands/blob/8d39305538c1d17a30cbe477e57a0e18fcee7d95/electron/main.mjs#L773-L777);
[`src/api/mars/AGENTS.md:3`](https://github.com/OpenHands/OpenHands/blob/8d39305538c1d17a30cbe477e57a0e18fcee7d95/src/api/mars/AGENTS.md#L3)).
The localStorage keys that matter (`openhands-backends`, `openhands-last-conversation-by-backend`,
`openhands-agent-server-conversation-metadata`, `archived-conversations`) are listed in
[Other stores](#other-stores).

#### Automation workspaces

The automation service unpacks each run into `AUTOMATION_WORKSPACE_BASE`: `~/.openhands/workspaces`
under Canvas-in-Docker, `~/.openhands/agent-canvas/workspaces` natively
([`docker/entrypoint.sh:334-336`](https://github.com/OpenHands/OpenHands/blob/8d39305538c1d17a30cbe477e57a0e18fcee7d95/docker/entrypoint.sh#L334-L336);
[`scripts/dev-with-automation.mjs:1066-1075`](https://github.com/OpenHands/OpenHands/blob/8d39305538c1d17a30cbe477e57a0e18fcee7d95/scripts/dev-with-automation.mjs#L1066-L1075)).
The Docker path is the same at
[`a6bba78`](https://github.com/OpenHands/OpenHands/commit/a6bba78ffd5a8b31620770f52383b1a2c0477fcd).

#### CLI

Files of the CLI
([`openhands_cli/locations.py:5-54`](https://github.com/OpenHands/OpenHands-CLI/blob/954f2ba646e8d749261a8f2b2b7e3031fa39be9f/openhands_cli/locations.py#L5-L54)):
`conversations/<hex>/` (SDK layout, plus `TASKS.json`,
[`tui/panels/plan_side_panel.py:107`](https://github.com/OpenHands/OpenHands-CLI/blob/954f2ba646e8d749261a8f2b2b7e3031fa39be9f/openhands_cli/tui/panels/plan_side_panel.py#L107)),
`projects/<sha256 of cwd>/prompt_history.json`, `agent_settings.json`, `mcp.json`, `cli_config.json`
([`stores/cli_settings.py:60-63`](https://github.com/OpenHands/OpenHands-CLI/blob/954f2ba646e8d749261a8f2b2b7e3031fa39be9f/openhands_cli/stores/cli_settings.py#L60-L63)),
`cloud/api_key.txt`, `cache/acp/`
([`acp_impl/utils/resources.py:43`](https://github.com/OpenHands/OpenHands-CLI/blob/954f2ba646e8d749261a8f2b2b7e3031fa39be9f/openhands_cli/acp_impl/utils/resources.py#L43)),
`hooks.json`
([`acp_impl/agent/local_agent.py:180`](https://github.com/OpenHands/OpenHands-CLI/blob/954f2ba646e8d749261a8f2b2b7e3031fa39be9f/openhands_cli/acp_impl/agent/local_agent.py#L180)).
`openhands serve` runs the legacy app image with `~/.openhands` mounted at `/.openhands`
([`gui_launcher.py:125-140`](https://github.com/OpenHands/OpenHands-CLI/blob/954f2ba646e8d749261a8f2b2b7e3031fa39be9f/openhands_cli/gui_launcher.py#L125-L140)).

#### Legacy app

The legacy app uses the same root. 1.x: `openhands.db` SQLite
([`openhands/app_server/services/db_session_injector.py:192`](https://github.com/OpenHands/OpenHands/blob/11ca68ab2e15dcd85c21e4d7d3409e7a259369ac/openhands/app_server/services/db_session_injector.py#L192)),
`[<user_id>/]v1_conversations/<hex>/<event hex>.json`
([`conversation_paths.py:12`](https://github.com/OpenHands/OpenHands/blob/11ca68ab2e15dcd85c21e4d7d3409e7a259369ac/openhands/app_server/conversation_paths.py#L12),[`38-55`](https://github.com/OpenHands/OpenHands/blob/11ca68ab2e15dcd85c21e4d7d3409e7a259369ac/openhands/app_server/conversation_paths.py#L38-L55);
[`event/event_service_base.py:89`](https://github.com/OpenHands/OpenHands/blob/11ca68ab2e15dcd85c21e4d7d3409e7a259369ac/openhands/app_server/event/event_service_base.py#L89)),
`settings.json`, `secrets.json`
([`settings/file_settings_store.py:15`](https://github.com/OpenHands/OpenHands/blob/11ca68ab2e15dcd85c21e4d7d3409e7a259369ac/openhands/app_server/settings/file_settings_store.py#L15);
[`secrets/file_secrets_store.py:15`](https://github.com/OpenHands/OpenHands/blob/11ca68ab2e15dcd85c21e4d7d3409e7a259369ac/openhands/app_server/secrets/file_secrets_store.py#L15)),
`analytics_id.txt`
([`analytics/oss_install_id.py:21`](https://github.com/OpenHands/OpenHands/blob/11ca68ab2e15dcd85c21e4d7d3409e7a259369ac/openhands/analytics/oss_install_id.py#L21)).
0.x: `sessions/<sid>/` with `events/<n>.json`, `event_cache/<start>-<end>.json`, `metadata.json`,
`init.json`, `agent_state.pkl`, `conversation_stats.pkl`, `llm_registry.json`, or
`users/<id>/conversations/<sid>/`
([`openhands/storage/locations.py:1-42`](https://github.com/OpenHands/OpenHands/blob/7fbb48c40679afd674970966b96185657d92a487/openhands/storage/locations.py#L1-L42);
[`openhands/events/event_store.py:159`](https://github.com/OpenHands/OpenHands/blob/7fbb48c40679afd674970966b96185657d92a487/openhands/events/event_store.py#L159));
default root `~/.openhands`
([`openhands/core/config/openhands_config.py:73`](https://github.com/OpenHands/OpenHands/blob/7fbb48c40679afd674970966b96185657d92a487/openhands/core/config/openhands_config.py#L73)).

#### Paths inside the sandbox

Paths inside the sandbox and not on the host: in the 0.x app, the runtime container's `/workspace`
unless `SANDBOX_VOLUMES` mounted a host directory
([`gui_launcher.py:161-170`](https://github.com/OpenHands/OpenHands-CLI/blob/954f2ba646e8d749261a8f2b2b7e3031fa39be9f/openhands_cli/gui_launcher.py#L161-L170));
in Canvas-in-Docker, everything except `~/.openhands` and `PROJECTS_PATH`.

### Credentials

- `~/.openhands/agent-canvas/secret-key.txt`: `OH_SECRET_KEY`, the master key
  ([`dev-safe.mjs:90-102`](https://github.com/OpenHands/OpenHands/blob/a6bba78ffd5a8b31620770f52383b1a2c0477fcd/scripts/dev-safe.mjs#L90-L102);
  [`docker/entrypoint.sh:185-196`](https://github.com/OpenHands/OpenHands/blob/a6bba78ffd5a8b31620770f52383b1a2c0477fcd/docker/entrypoint.sh#L185-L196)).
  Secret fields in `settings.json`, `secrets.json`, `profiles/*.json`, `meta.json` and
  `base_state.json` are Fernet tokens keyed by SHA-256 of it
  ([`openhands-sdk/openhands/sdk/utils/cipher.py:73-80`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/utils/cipher.py#L73-L80);
  [`pydantic_secrets.py:48-78`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/utils/pydantic_secrets.py#L48-L78)),
  and are written in plaintext when no key is configured
  ([`store.py:372-383`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-agent-server/openhands/agent_server/persistence/store.py#L372-L383)).
  Collecting the key with the stores makes every secret readable.
- `~/.openhands/agent-canvas/api-key.txt`: the local backend session key
  ([`dev-safe.mjs:80-85`](https://github.com/OpenHands/OpenHands/blob/a6bba78ffd5a8b31620770f52383b1a2c0477fcd/scripts/dev-safe.mjs#L80-L85)).
- `provider-connections/*` and `auth/<vendor>_oauth.json`, beside the stores of the first bullet
  ([`credentials.py:73-75`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/llm/auth/credentials.py#L73-L75)).
- `runtime-control/<hex>.json`: per-conversation `api_key` and `encryption_key`, themselves
  encrypted with the master key
  ([`provisioning.py:22-45`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-agent-server/openhands/agent_server/docker_runtime/provisioning.py#L22-L45)).
- CLI: `agent_settings.json` saved with `expose_secrets: True`, so the LLM API key is plaintext
  ([`stores/agent_store.py:474-476`](https://github.com/OpenHands/OpenHands-CLI/blob/954f2ba646e8d749261a8f2b2b7e3031fa39be9f/openhands_cli/stores/agent_store.py#L474-L476));
  `cloud/api_key.txt`, the OpenHands Cloud account key in plain text (mode 0600), written by
  `openhands login` and read by `openhands cloud` and `openhands acp --cloud`
  ([`auth/token_storage.py:19-25`](https://github.com/OpenHands/OpenHands-CLI/blob/954f2ba646e8d749261a8f2b2b7e3031fa39be9f/openhands_cli/auth/token_storage.py#L19-L25);
  [`19-37`](https://github.com/OpenHands/OpenHands-CLI/blob/0403f1a79a9fc62a6c7be21c17d7b51f64245ad2/openhands_cli/auth/token_storage.py#L19-L37)
  at
  [`0403f1a`](https://github.com/OpenHands/OpenHands-CLI/commit/0403f1a79a9fc62a6c7be21c17d7b51f64245ad2)),
  flagged by the `.openhands/cloud/*` secret glob;
  `mcp.json` holds MCP server definitions, which may embed keys; it is flagged.
- Legacy: `.jwt_secret` and `.keys` in the root
  ([`openhands/app_server/utils/encryption_key.py:51-57`](https://github.com/OpenHands/OpenHands/blob/11ca68ab2e15dcd85c21e4d7d3409e7a259369ac/openhands/app_server/utils/encryption_key.py#L51-L57);
  [`openhands/core/config/utils.py:35`](https://github.com/OpenHands/OpenHands/blob/7fbb48c40679afd674970966b96185657d92a487/openhands/core/config/utils.py#L35)).
- A conversation driving Codex through ACP reads `~/.codex/auth.json`
  ([`openhands-sdk/openhands/sdk/agent/acp_file_credentials.py:32`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/agent/acp_file_credentials.py#L32),[`65-68`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/agent/acp_file_credentials.py#L65-L68));
  older conversations may hold `acp/codex/auth.json` in their own directory
  ([`event_service.py:338-339`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-agent-server/openhands/agent_server/event_service.py#L338-L339)),
  flagged by `.openhands/*/acp/codex/auth.json` and, in a volume,
  `conversations/*/acp/codex/auth.json` and `agent-canvas/*/acp/codex/auth.json`.
- Agent Canvas Electron app: `<user data directory>/mars-credentials.json`, the
  DigitalOcean connections (`connections[]` with `kind` `oauth` or `pat`, `label`, `teamName`,
  `expiresAt` and the token, and a `deviceId`); only the token is encrypted, with Electron
  `safeStorage` (the OS keychain), and is written as `null` when the keychain is unavailable, so the
  connection then lasts only for the session; the file is mode 0600
  ([`scripts/mars-credentials.mjs:18`](https://github.com/OpenHands/OpenHands/blob/8d39305538c1d17a30cbe477e57a0e18fcee7d95/scripts/mars-credentials.mjs#L18),
  [`42-49`](https://github.com/OpenHands/OpenHands/blob/8d39305538c1d17a30cbe477e57a0e18fcee7d95/scripts/mars-credentials.mjs#L42-L49),
  [`77-88`](https://github.com/OpenHands/OpenHands/blob/8d39305538c1d17a30cbe477e57a0e18fcee7d95/scripts/mars-credentials.mjs#L77-L88),
  [`118-135`](https://github.com/OpenHands/OpenHands/blob/8d39305538c1d17a30cbe477e57a0e18fcee7d95/scripts/mars-credentials.mjs#L118-L135),
  [`195-210`](https://github.com/OpenHands/OpenHands/blob/8d39305538c1d17a30cbe477e57a0e18fcee7d95/scripts/mars-credentials.mjs#L195-L210)).
  Flagged by the three secret globs
  `Library/Application Support/OpenHands Agent Canvas/mars-credentials.json`,
  `.config/OpenHands Agent Canvas/mars-credentials.json` and
  `AppData/Roaming/OpenHands Agent Canvas/mars-credentials.json`.
- A GitHub, GitLab or Bitbucket token the Agent Server uses to clone and to search the user's
  repositories is a custom secret in `secrets.json` (`custom_secrets.<name>.secret`, under one of
  the names `github_token`, `GITHUB_TOKEN`, `GH_TOKEN` or `github` for the search), encrypted like
  the other secret fields of the first bullet
  ([Source host rows](#source-host-rows)).
- The Canvas `Local Storage` holds each backend's API key in plain JSON
  ([Secrets](#secrets)).

No keychain use found, apart from the `safeStorage` encryption above.

### Exclusions

`.openhands/cache` (git clones of skill and plugin sources), and
`.openhands/agent-canvas/workspaces`, which can hold whole repositories the agent cloned. It is also
the agent's work product for scratch conversations, so it is excluded by default like
`.claude/worktrees` and `.codex/worktrees`, recorded as `skipped_excluded` with its size, and
collected with `--full`. `.openhands/agent-canvas/tmux` holds sockets only. Keep `bash_events`,
`runtime-data` (container `HOME`, may hold shell history) and `logs`.

`.openhands/workspaces` (automation run workspaces under Canvas-in-Docker,
[Per-user storage](#per-user-storage)) holds the same kind of content as `agent-canvas/workspaces` and is
excluded the same way; `workspaces.json` beside it is a file of its own and is collected.

### Project-local files

`<project>/.openhands/`: `hooks.json`
([`openhands-agent-server/openhands/agent_server/hooks_service.py:36`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-agent-server/openhands/agent_server/hooks_service.py#L36)),
`skills/`, `microagents/`, `memory/MEMORY.md`, `setup.sh`, `pre-commit.sh`
([`skills_service.py:6-10`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-agent-server/openhands/agent_server/skills_service.py#L6-L10);
[`memory.py:23`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/context/memory.py#L23);
[`openhands/app_server/app_conversation/app_conversation_service_base.py:66-74`](https://github.com/OpenHands/OpenHands/blob/11ca68ab2e15dcd85c21e4d7d3409e7a259369ac/openhands/app_server/app_conversation/app_conversation_service_base.py#L66-L74)).
Also read: `.agents/skills/`
([`app_conversation_router.py:1364`](https://github.com/OpenHands/OpenHands/blob/11ca68ab2e15dcd85c21e4d7d3409e7a259369ac/openhands/app_server/app_conversation/app_conversation_router.py#L1364)),
`AGENTS.md`, `.cursorrules` (skills service, above). No per-project database.

### Where the project path is recorded

- `workspaces.json` `workspaces[].path` and `workspaceParents[].path`
  ([`openhands-agent-server/openhands/agent_server/persistence/models.py:615-636`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-agent-server/openhands/agent_server/persistence/models.py#L615-L636)).
- Each conversation's `meta.json` and `base_state.json`: `workspace.working_dir`
  ([`openhands-sdk/openhands/sdk/conversation/request.py:105`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/conversation/request.py#L105);
  [`openhands-sdk/openhands/sdk/workspace/base.py:43`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/workspace/base.py#L43)).
  Under Canvas-in-Docker these are container paths (`/projects/<name>`), not host paths.
- CLI `projects/<sha256>/` names are hashes of the path
  ([`locations.py:34-42`](https://github.com/OpenHands/OpenHands-CLI/blob/954f2ba646e8d749261a8f2b2b7e3031fa39be9f/openhands_cli/locations.py#L34-L42));
  not reversible, but checkable against a candidate path; the analyzer checks each
  conversation's `workspace.working_dir`
  ([Past-use rows and secondary sources](#past-use-rows-and-secondary-sources)).
- Agent Canvas `localStorage` (Electron `Local Storage`)
  `openhands-agent-server-conversation-metadata`: per conversation id, `selected_workspace` (the
  folder the user attached), `selected_repository`, `selected_branch` and `workspace_mode`
  (`local_repo` or `new_worktree`)
  ([`src/api/conversation-metadata-store.ts:4-56`](https://github.com/OpenHands/OpenHands/blob/8d39305538c1d17a30cbe477e57a0e18fcee7d95/src/api/conversation-metadata-store.ts#L4-L56)).
  LevelDB, so not grep-able by `discover_projects`.
- A worktree conversation's `workspace.working_dir` is under `/tmp/conversation-worktrees/` by
  default, a scratch directory ([Scratch directories](#scratch-directories)), not the user's
  project. Whether the conversation files also name the repository the worktree was made
  from was not traced.

## Parsing

### Transcript stores

One directory per conversation, named by the conversation UUID as 32 hex digits
([`openhands-sdk/openhands/sdk/conversation/base.py:331-345`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/conversation/base.py#L331-L345)),
under:

- `.openhands/agent-canvas/dev_conversations/<hex>/` (Canvas, native) and
  `.openhands/agent-canvas/conversations/<hex>/` (Canvas in Docker);
- `.openhands/conversations/<hex>/` (CLI).

Inside
([`conversation/persistence_const.py:4-10`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/conversation/persistence_const.py#L4-L10)):

- `events/event-<idx, 5+ digits>-<event id>.json`, one compact JSON object per event
  ([`conversation/event_store.py:225-232`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/conversation/event_store.py#L225-L232),[`313-318`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/conversation/event_store.py#L313-L318)),
  plus an empty `.eventlog-len-<n>.marker`
  ([`event_store.py:27`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/conversation/event_store.py#L27)).
- `base_state.json`: `ConversationState` (agent with its LLM config, workspace, stats, secret
  registry)
  ([`conversation/state.py:82-225`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/conversation/state.py#L82-L225),
  [`435-447`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/conversation/state.py#L435-L447)).
- `meta.json` (Agent Server only): `StoredConversation`
  ([`openhands-agent-server/openhands/agent_server/event_service.py:276-297`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-agent-server/openhands/agent_server/event_service.py#L276-L297)).
- `bash_events/` for terminal sessions bound to the conversation
  ([`agent_server/dependencies.py:72-80`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-agent-server/openhands/agent_server/dependencies.py#L72-L80));
  the global `.openhands/agent-canvas/bash_events/` has the same files.
- `TASKS.json` (CLI plan panel,
  [`openhands_cli/tui/panels/plan_side_panel.py:107`](https://github.com/OpenHands/OpenHands-CLI/blob/954f2ba646e8d749261a8f2b2b7e3031fa39be9f/openhands_cli/tui/panels/plan_side_panel.py#L107)).

The CLI also keeps `.openhands/projects/<sha256(cwd)>/prompt_history.json`
([`openhands_cli/locations.py:34-47`](https://github.com/OpenHands/OpenHands-CLI/blob/954f2ba646e8d749261a8f2b2b7e3031fa39be9f/openhands_cli/locations.py#L34-L47)),
described in [CLI prompt history](#cli-prompt-history).

#### What survives a delete

The Agent Server's delete, which Agent Canvas uses, notifies webhooks with status `DELETING` and
removes the conversation directory and, once no other conversation uses it, the worktree directory;
the workspace and the worktree's branches in the user's repository stay, and children and forks keep
their now dangling `parent_conversation_id` or `forked_from_conversation_id`
([`agent_server/conversation_service.py:1774-1838`](https://github.com/OpenHands/software-agent-sdk/blob/33d8aa29c3078d074799e2fbf5e847aeb2d71360/openhands-agent-server/openhands/agent_server/conversation_service.py#L1774-L1838);
[`local_storage.py:26-37`](https://github.com/OpenHands/software-agent-sdk/blob/33d8aa29c3078d074799e2fbf5e847aeb2d71360/openhands-agent-server/openhands/agent_server/local_storage.py#L26-L37);
the same delete at SDK
[`fd26808`](https://github.com/OpenHands/software-agent-sdk/commit/fd268089e257bee4ad704ce4fdc6466bc756c71c):
[`agent_server/conversation_service.py:1774-1838`](https://github.com/OpenHands/software-agent-sdk/blob/fd268089e257bee4ad704ce4fdc6466bc756c71c/openhands-agent-server/openhands/agent_server/conversation_service.py#L1774-L1838)).
Canvas also deletes the hidden planning conversation and the metadata entry, and for a cloud backend
the remote conversation; its archive only adds the id to `archived-conversations`. The CLI has no
delete; `openhands logout` removes `cloud/api_key.txt`
([`auth/token_storage.py:55-61`](https://github.com/OpenHands/OpenHands-CLI/blob/0403f1a79a9fc62a6c7be21c17d7b51f64245ad2/openhands_cli/auth/token_storage.py#L55-L61)).
What remains: the CLI's `prompt_history.json`, the last 100 prompts per working directory with text
and local time and no conversation ([CLI prompt history](#cli-prompt-history)); a fork, which is a
deep copy of its source's events and state
([`agent_server/conversation_service.py:1952-1995`](https://github.com/OpenHands/software-agent-sdk/blob/33d8aa29c3078d074799e2fbf5e847aeb2d71360/openhands-agent-server/openhands/agent_server/conversation_service.py#L1952-L1995))
and joins on `meta.json` `forked_from_conversation_id`;
the global `agent-canvas/bash_events/` record of terminal commands; and Canvas browser-storage keys
with no message text, such as `openhands-last-conversation-by-backend`
([Other stores](#other-stores)). Nothing local records a deleted conversation. The analyzer reads a
fork as its own conversation and the prompt history as a secondary source (see
[Secondary sources](../analyzer/docs/output.md#secondary-sources)), so a deleted CLI
conversation's prompts are still in the timeline; it does not read the global terminal record or
the browser storage ([Parser plan](#parser-plan)).

### Record schema

Every event has `kind` (the Python class name,
[`openhands-sdk/openhands/sdk/utils/models.py:202-205`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/utils/models.py#L202-L205)),
`id` (UUID string), `timestamp`, `source` in `agent|user|environment|hook`, `parent_id`
([`event/base.py:24-40`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/event/base.py#L24-L40);
[`event/types.py:4-5`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/event/types.py#L4-L5)).
`timestamp` is `datetime.now().isoformat()`: **naive local time** with microseconds and no zone
([`base.py:28-31`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/event/base.py#L28-L31)).
The parser needs the host's zone to convert it; `meta.json` `created_at`/`updated_at` are UTC with
offset
([`agent_server/models.py:120-121`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-agent-server/openhands/agent_server/models.py#L120-L121);
[`agent_server/utils.py:53-55`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-agent-server/openhands/agent_server/utils.py#L53-L55))
and can anchor the offset. Files are written with `exclude_none`.

Kinds:

- `MessageEvent`: `source` user or agent, `llm_message` (`role` `user|system|assistant|tool`,
  `content` list of `{type:"text",text}` or `{type:"image",image_urls}`, `tool_calls?`,
  `reasoning_content?`, `thinking_blocks?`), `activated_skills`, `extended_content`, `sender`
  ([`event/llm_convertible/message.py:25-60`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/event/llm_convertible/message.py#L25-L60);
  [`llm/message.py:174-176`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/llm/message.py#L174-L176),
  [`205-207`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/llm/message.py#L205-L207),
  [`219-245`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/llm/message.py#L219-L245)).
- `ActionEvent`: `thought` (text list), `reasoning_content`, `thinking_blocks`
  (`{type:"thinking",thinking,signature}` or `redacted_thinking`), `action` (object with its own
  `kind`, for example `TerminalAction` with `command`), `tool_name`, `tool_call_id`, `tool_call`
  (`id`, `name`, `arguments` JSON string, `origin`), `llm_response_id`, `security_risk`, `summary`
  ([`event/llm_convertible/action.py:24-80`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/event/llm_convertible/action.py#L24-L80);
  [`llm/message.py:25-40`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/llm/message.py#L25-L40),
  [`123-147`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/llm/message.py#L123-L147);
  [`openhands-tools/openhands/tools/terminal/definition.py:86-89`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-tools/openhands/tools/terminal/definition.py#L86-L89)).
  Tool names are the snake-cased class name without `_tool`, so `terminal`, `file_editor`
  ([`tool/tool.py:397`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/tool/tool.py#L397)).
- `ObservationEvent`: `tool_name`, `tool_call_id`, `action_id`, `observation` (own `kind`, `content`
  list, `is_error`; `TerminalObservation` adds `command`, `exit_code`)
  ([`event/llm_convertible/observation.py:17-40`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/event/llm_convertible/observation.py#L17-L40);
  [`tool/schema.py:367-382`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/tool/schema.py#L367-L382);
  [`terminal/definition.py:146-152`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-tools/openhands/tools/terminal/definition.py#L146-L152)).
- `UserRejectObservation` (`rejection_reason`, `rejection_source`, `action_id`) and
  `AgentErrorEvent` (`error`, `classification`)
  ([`observation.py:86-147`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/event/llm_convertible/observation.py#L86-L147)).
- `SystemPromptEvent`: `system_prompt`, `tools`, `dynamic_context`
  ([`event/llm_convertible/system.py:12-35`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/event/llm_convertible/system.py#L12-L35)).
- `ACPToolCallEvent` (Claude Code, Codex, Gemini driven over ACP): `tool_call_id`, `title`,
  `status`, `tool_kind`, `raw_input`, `raw_output`, `content`, `is_error`
  ([`event/acp_tool_call.py:46-65`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/event/acp_tool_call.py#L46-L65)).
- `HookExecutionEvent` (`hook_event_type`, `hook_command`, `exit_code`, `stdout`, `stderr`,
  `blocked`;
  [`event/hook_execution.py:22-80`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/event/hook_execution.py#L22-L80)),
  `Condensation` and `CondensationSummaryEvent` (`summary`;
  [`event/condenser.py:11-37`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/event/condenser.py#L11-L37),
  [`120-126`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/event/condenser.py#L120-L126)),
  `ConversationStateUpdateEvent` (`key`, `value`;
  [`event/conversation_state.py:18-35`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/event/conversation_state.py#L18-L35)),
  `PauseEvent`, `InterruptEvent`
  ([`event/user_action.py:7-32`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/event/user_action.py#L7-L32)).

`meta.json`: `id`, `title`, `created_at`, `updated_at`, `forked_from_conversation_id`, `workspace`
(`kind`, `working_dir`), `parent_conversation_id`, `secrets`, `initial_message`
([`agent_server/models.py:102-143`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-agent-server/openhands/agent_server/models.py#L102-L143);
[`conversation/request.py:93-156`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/conversation/request.py#L93-L156)).
`base_state.json`: `id`, `agent.llm.model`
([`llm/llm.py:272`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/llm/llm.py#L272);
[`agent/base.py:117`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/agent/base.py#L117)),
`workspace.working_dir`
([`workspace/base.py:43`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/workspace/base.py#L43)),
`execution_status`, `stats.usage_to_metrics`.

Bash events (`bash_events/<YYYYMMDDHHMMSSffffff>_<kind>_[<command id>_]<event id>`, no extension,
indented JSON;
[`agent_server/bash_service.py:61-85`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-agent-server/openhands/agent_server/bash_service.py#L61-L85)):
`BashCommand` (`id`, `timestamp` UTC, `command`, `cwd`), `BashOutput` (`command_id`, `order`,
`exit_code`, `stdout`, `stderr`), `BashError`
([`agent_server/models.py:616-660`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-agent-server/openhands/agent_server/models.py#L616-L660)).

#### Permission records

At SDK
[`33d8aa2`](https://github.com/OpenHands/software-agent-sdk/commit/33d8aa29c3078d074799e2fbf5e847aeb2d71360):

- `SecurityAnalysisEvent` (new since
  [`b347047`](https://github.com/OpenHands/software-agent-sdk/commit/b347047e2dcdd8f4b2be810aa8bc6632bc756d41);
  `source` `environment`): `analyzer` (the analyzer's class name, for example
  `LLMSecurityAnalyzer`), `policy` (the confirmation policy's class name: `AlwaysConfirm`,
  `NeverConfirm` or `ConfirmRisky`), `risks` (a map from `ActionEvent` id to `UNKNOWN`, `LOW`,
  `MEDIUM` or `HIGH`) and `details` (analyzer-specific detail per action id)
  ([`event/security.py:11-39`](https://github.com/OpenHands/software-agent-sdk/blob/33d8aa29c3078d074799e2fbf5e847aeb2d71360/openhands-sdk/openhands/sdk/event/security.py#L11-L39)).
  It is written once per batch of pending actions whenever an analyzer is configured, before the
  policy decides whether to ask
  ([`agent/agent.py:1130-1140`](https://github.com/OpenHands/software-agent-sdk/blob/33d8aa29c3078d074799e2fbf5e847aeb2d71360/openhands-sdk/openhands/sdk/agent/agent.py#L1130-L1140),
  [`1164-1190`](https://github.com/OpenHands/software-agent-sdk/blob/33d8aa29c3078d074799e2fbf5e847aeb2d71360/openhands-sdk/openhands/sdk/agent/agent.py#L1164-L1190)).
- An approval writes no event: the Agent Server's `ConfirmationResponseRequest` (`accept`, `reason`)
  resumes the run on `accept` and calls `reject_pending_actions` otherwise
  ([`agent_server/models.py:438-442`](https://github.com/OpenHands/software-agent-sdk/blob/33d8aa29c3078d074799e2fbf5e847aeb2d71360/openhands-agent-server/openhands/agent_server/models.py#L438-L442);
  [`agent_server/event_service.py:2015-2029`](https://github.com/OpenHands/software-agent-sdk/blob/33d8aa29c3078d074799e2fbf5e847aeb2d71360/openhands-agent-server/openhands/agent_server/event_service.py#L2015-L2029)),
  so an approved action shows only as an `ActionEvent` followed by its `ObservationEvent`. A
  `PreToolUse` hook's block is a `UserRejectObservation` with `rejection_source` `hook`
  ([`agent/agent.py:360-368`](https://github.com/OpenHands/software-agent-sdk/blob/33d8aa29c3078d074799e2fbf5e847aeb2d71360/openhands-sdk/openhands/sdk/agent/agent.py#L360-L368)).
- `base_state.json` `confirmation_policy` (`kind` `AlwaysConfirm`, `NeverConfirm` or `ConfirmRisky`
  with `threshold`), `security_analyzer` (`kind`, or absent) and `blocked_actions` (action id to the
  hook's reason)
  ([`conversation/state.py:123-127`](https://github.com/OpenHands/software-agent-sdk/blob/33d8aa29c3078d074799e2fbf5e847aeb2d71360/openhands-sdk/openhands/sdk/conversation/state.py#L123-L127),
  [`152-157`](https://github.com/OpenHands/software-agent-sdk/blob/33d8aa29c3078d074799e2fbf5e847aeb2d71360/openhands-sdk/openhands/sdk/conversation/state.py#L152-L157);
  [`security/confirmation_policy.py:27-61`](https://github.com/OpenHands/software-agent-sdk/blob/33d8aa29c3078d074799e2fbf5e847aeb2d71360/openhands-sdk/openhands/sdk/security/confirmation_policy.py#L27-L61)).
  A field assignment rewrites the file
  ([`state.py:596-610`](https://github.com/OpenHands/software-agent-sdk/blob/33d8aa29c3078d074799e2fbf5e847aeb2d71360/openhands-sdk/openhands/sdk/conversation/state.py#L596-L610)),
  so it holds the policy in force at the last write, not its history. The CLI's "always proceed" and
  "risk based" answers set `NeverConfirm` or `ConfirmRisky` on the conversation through the same
  call
  ([`tui/core/confirmation_flow_controller.py:38-48`](https://github.com/OpenHands/OpenHands-CLI/blob/0403f1a79a9fc62a6c7be21c17d7b51f64245ad2/openhands_cli/tui/core/confirmation_flow_controller.py#L38-L48);
  [`tui/core/confirmation_policy_service.py:25-29`](https://github.com/OpenHands/OpenHands-CLI/blob/0403f1a79a9fc62a6c7be21c17d7b51f64245ad2/openhands_cli/tui/core/confirmation_policy_service.py#L25-L29);
  [`acp_impl/confirmation.py:116-128`](https://github.com/OpenHands/OpenHands-CLI/blob/0403f1a79a9fc62a6c7be21c17d7b51f64245ad2/openhands_cli/acp_impl/confirmation.py#L116-L128)),
  and `--always-approve` or headless mode starts it at `NeverConfirm`
  ([`tui/textual_app.py:729-735`](https://github.com/OpenHands/OpenHands-CLI/blob/0403f1a79a9fc62a6c7be21c17d7b51f64245ad2/openhands_cli/tui/textual_app.py#L729-L735)).
  The Agent Server's default for new conversations is `settings.json`
  `conversation_settings.confirmation_mode` (default `false`) and `security_analyzer` (default
  `llm`)
  ([`sdk/settings/model.py:1175-1201`](https://github.com/OpenHands/software-agent-sdk/blob/33d8aa29c3078d074799e2fbf5e847aeb2d71360/openhands-sdk/openhands/sdk/settings/model.py#L1175-L1201),
  [`1221-1232`](https://github.com/OpenHands/software-agent-sdk/blob/33d8aa29c3078d074799e2fbf5e847aeb2d71360/openhands-sdk/openhands/sdk/settings/model.py#L1221-L1232);
  [`agent_server/persistence/models.py:150-183`](https://github.com/OpenHands/software-agent-sdk/blob/33d8aa29c3078d074799e2fbf5e847aeb2d71360/openhands-agent-server/openhands/agent_server/persistence/models.py#L150-L183)).

#### Model records

At SDK
[`33d8aa2`](https://github.com/OpenHands/software-agent-sdk/commit/33d8aa29c3078d074799e2fbf5e847aeb2d71360):

- Model switches. The agent's own `switch_llm` tool loads a saved LLM profile and returns a
  `SwitchLLMObservation` with `profile_name`, `reason` and `active_model`; the
  `classify_and_switch_llm` tool returns a `ClassifyAndSwitchLLMObservation` with `chosen_class`,
  `model` and `active_model`
  ([`tool/builtins/switch_llm.py:43-57`](https://github.com/OpenHands/software-agent-sdk/blob/33d8aa29c3078d074799e2fbf5e847aeb2d71360/openhands-sdk/openhands/sdk/tool/builtins/switch_llm.py#L43-L57),
  [`136-147`](https://github.com/OpenHands/software-agent-sdk/blob/33d8aa29c3078d074799e2fbf5e847aeb2d71360/openhands-sdk/openhands/sdk/tool/builtins/switch_llm.py#L136-L147);
  [`tool/builtins/classify_and_switch_llm.py:57-75`](https://github.com/OpenHands/software-agent-sdk/blob/33d8aa29c3078d074799e2fbf5e847aeb2d71360/openhands-sdk/openhands/sdk/tool/builtins/classify_and_switch_llm.py#L57-L75)).
  Both tools were already present at
  [`b347047`](https://github.com/OpenHands/software-agent-sdk/commit/b347047e2dcdd8f4b2be810aa8bc6632bc756d41).
  A switch through the Agent Server's `switch_profile` or `switch_llm` endpoints writes no event; it
  replaces `agent.llm` in `base_state.json`
  ([`agent_server/conversation_router.py:586-651`](https://github.com/OpenHands/software-agent-sdk/blob/33d8aa29c3078d074799e2fbf5e847aeb2d71360/openhands-agent-server/openhands/agent_server/conversation_router.py#L586-L651);
  [`conversation/impl/local_conversation.py:1673-1743`](https://github.com/OpenHands/software-agent-sdk/blob/33d8aa29c3078d074799e2fbf5e847aeb2d71360/openhands-sdk/openhands/sdk/conversation/impl/local_conversation.py#L1673-L1743)).
  Canvas says so itself: "manual `/model` switches go through REST and leave no observation in
  history"
  ([`src/api/conversation-metadata-store.ts:38-48`](https://github.com/OpenHands/OpenHands/blob/8d39305538c1d17a30cbe477e57a0e18fcee7d95/src/api/conversation-metadata-store.ts#L38-L48)).
  The CLI's ACP `set_session_model` is a no-op
  ([`acp_impl/agent/base_agent.py:331-339`](https://github.com/OpenHands/OpenHands-CLI/blob/0403f1a79a9fc62a6c7be21c17d7b51f64245ad2/openhands_cli/acp_impl/agent/base_agent.py#L331-L339)).
- Model per response. `base_state.json` `stats.usage_to_metrics.<usage id>` is a full `Metrics`
  object (not the snapshot form) with `model_name`, `costs[].model`, `response_latencies[]`
  (`model`, `response_id`) and `token_usages[]` (`model`, `response_id`)
  ([`llm/utils/metrics.py:21-59`](https://github.com/OpenHands/software-agent-sdk/blob/33d8aa29c3078d074799e2fbf5e847aeb2d71360/openhands-sdk/openhands/sdk/llm/utils/metrics.py#L21-L59),
  [`76-130`](https://github.com/OpenHands/software-agent-sdk/blob/33d8aa29c3078d074799e2fbf5e847aeb2d71360/openhands-sdk/openhands/sdk/llm/utils/metrics.py#L76-L130);
  [`conversation/state.py:430-451`](https://github.com/OpenHands/software-agent-sdk/blob/33d8aa29c3078d074799e2fbf5e847aeb2d71360/openhands-sdk/openhands/sdk/conversation/state.py#L430-L451)).
  `response_id` matches the `llm_response_id` of the `ActionEvent` and agent `MessageEvent` that the
  response produced
  ([`event/llm_convertible/message.py:36-41`](https://github.com/OpenHands/software-agent-sdk/blob/33d8aa29c3078d074799e2fbf5e847aeb2d71360/openhands-sdk/openhands/sdk/event/llm_convertible/message.py#L36-L41);
  [`action.py:58-61`](https://github.com/OpenHands/software-agent-sdk/blob/33d8aa29c3078d074799e2fbf5e847aeb2d71360/openhands-sdk/openhands/sdk/event/llm_convertible/action.py#L58-L61)),
  so the model of each turn can be recovered even after a REST switch. `meta.json` also has
  `metrics` (a snapshot with `model_name`), `forked_from_event_id` and `launched_agent_profile`
  ([`agent_server/models.py:115-146`](https://github.com/OpenHands/software-agent-sdk/blob/33d8aa29c3078d074799e2fbf5e847aeb2d71360/openhands-agent-server/openhands/agent_server/models.py#L115-L146)).

### Joins and timestamps

Session to project: `meta.json` or `base_state.json` `workspace.working_dir` (a container path under
Canvas-in-Docker). Tool call to result: `ActionEvent.tool_call_id` =
`ObservationEvent.tool_call_id`; also `ObservationEvent.action_id` = `ActionEvent.id`. Parent to
child conversation: `meta.json` `parent_conversation_id`, `forked_from_conversation_id`. Event
order: the `idx` in the file name; `parent_id` forms a tree when the user navigated back and
branched.

Event `timestamp` is naive local time and `meta.json` times are UTC; see
[Record schema](#record-schema).

### Other stores

None for current transcripts. Legacy 1.x kept conversation metadata in `.openhands/openhands.db`
([`openhands/app_server/services/db_session_injector.py:192`](https://github.com/OpenHands/OpenHands/blob/11ca68ab2e15dcd85c21e4d7d3409e7a259369ac/openhands/app_server/services/db_session_injector.py#L192)).
Legacy 0.x wrote `agent_state.pkl` and `conversation_stats.pkl` (Python pickle; never unpickle
evidence)
([`openhands/storage/locations.py:29-38`](https://github.com/OpenHands/OpenHands/blob/7fbb48c40679afd674970966b96185657d92a487/openhands/storage/locations.py#L29-L38)).
`.openhands/automation/automations.db` is SQLite (automation schedules and runs), outside this
parser's scope.

#### CLI prompt history

`.openhands/projects/<sha256 of cwd>/prompt_history.json` is a JSON array of the last 100 prompts
per working directory, `text` and `timestamp` (naive local `datetime.now().isoformat()`), oldest
first
([`stores/prompt_history.py:11-15`](https://github.com/OpenHands/OpenHands-CLI/blob/0403f1a79a9fc62a6c7be21c17d7b51f64245ad2/openhands_cli/stores/prompt_history.py#L11-L15),
[`59-95`](https://github.com/OpenHands/OpenHands-CLI/blob/0403f1a79a9fc62a6c7be21c17d7b51f64245ad2/openhands_cli/stores/prompt_history.py#L59-L95)).
It names no conversation, and its project is only a hash of the path; it survives the removal of a
conversation directory and joins on text and time.

#### Agent Canvas browser storage

At Canvas
[`8d39305`](https://github.com/OpenHands/OpenHands/commit/8d39305538c1d17a30cbe477e57a0e18fcee7d95)
the front end keeps its own records in browser `localStorage`: in the Electron app that is the
Chromium `Local Storage/leveldb` directory (LevelDB, the key-value file format Chromium stores it
in) under the app's user data directory, which the catalog collects; in a browser it is the browser
profile, outside the catalog. The analyzer has no LevelDB reader for these keys. Each value is JSON:

| Key | Holds | Survives |
| --- | --- | --- |
| `openhands-backends` | every backend the front end talks to: `id`, `name`, `host`, `apiKey`, `kind` (`local` or `cloud`), and for a DigitalOcean session `marsSessionId` and `marsConfigId` ([`src/api/backend-registry/types.ts:4-22`](https://github.com/OpenHands/OpenHands/blob/8d39305538c1d17a30cbe477e57a0e18fcee7d95/src/api/backend-registry/types.ts#L4-L22); [`storage.ts:13-14`](https://github.com/OpenHands/OpenHands/blob/8d39305538c1d17a30cbe477e57a0e18fcee7d95/src/api/backend-registry/storage.ts#L13-L14), [`95-101`](https://github.com/OpenHands/OpenHands/blob/8d39305538c1d17a30cbe477e57a0e18fcee7d95/src/api/backend-registry/storage.ts#L95-L101)) | the backend's removal |
| `openhands-last-conversation-by-backend` | backend id (with `::<org id>` for an organisation) to the last conversation opened on it, a remote id for a cloud backend ([`last-conversation-store.ts:10-18`](https://github.com/OpenHands/OpenHands/blob/8d39305538c1d17a30cbe477e57a0e18fcee7d95/src/api/backend-registry/last-conversation-store.ts#L10-L18)) | the conversation's deletion |
| `openhands-agent-server-conversation-metadata` | conversation id to `selected_repository`, `selected_branch`, `git_provider`, `selected_workspace`, `workspace_mode` (`local_repo` or `new_worktree`), `active_profile` and `stamped_at` (the LLM profile last chosen and when), `plugins`, `local_planning_conversation_id` ([`src/api/conversation-metadata-store.ts:4-56`](https://github.com/OpenHands/OpenHands/blob/8d39305538c1d17a30cbe477e57a0e18fcee7d95/src/api/conversation-metadata-store.ts#L4-L56)) | removed on delete ([`agent-server-conversation-service.api.ts:1047-1081`](https://github.com/OpenHands/OpenHands/blob/8d39305538c1d17a30cbe477e57a0e18fcee7d95/src/api/conversation-service/agent-server-conversation-service.api.ts#L1047-L1081)) |
| `archived-conversations` | `archivesByBackendId`: backend id to the conversation ids the user archived ([`src/stores/archived-conversations-store.ts:4-8`](https://github.com/OpenHands/OpenHands/blob/8d39305538c1d17a30cbe477e57a0e18fcee7d95/src/stores/archived-conversations-store.ts#L4-L8), [`75-78`](https://github.com/OpenHands/OpenHands/blob/8d39305538c1d17a30cbe477e57a0e18fcee7d95/src/stores/archived-conversations-store.ts#L75-L78)) | archive is a hide; the conversation directory stays |

None of these holds message text, so none repeats an event; they mark a conversation's origin,
workspace choice, archive state and the profile the user picked.

### Format versions

- Current SDK: as above. The 5-digit index is a minimum
  ([`persistence_const.py:6-10`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/conversation/persistence_const.py#L6-L10)).
  Every path, the event file layout and every field the parser reads are the same at SDK
  [`33d8aa2`](https://github.com/OpenHands/software-agent-sdk/commit/33d8aa29c3078d074799e2fbf5e847aeb2d71360),
  Canvas
  [`8d39305`](https://github.com/OpenHands/OpenHands/commit/8d39305538c1d17a30cbe477e57a0e18fcee7d95)
  and CLI
  [`0403f1a`](https://github.com/OpenHands/OpenHands-CLI/commit/0403f1a79a9fc62a6c7be21c17d7b51f64245ad2)
  as at the earlier commits; new at those commits are `mars-credentials.json`
  ([Credentials](#credentials)) and the `SecurityAnalysisEvent` kind
  ([Permission records](#permission-records)).
- Legacy 1.x app: `.openhands/[<user_id>/]v1_conversations/<hex>/<event hex>.json`
  ([`openhands/app_server/conversation_paths.py:12`](https://github.com/OpenHands/OpenHands/blob/11ca68ab2e15dcd85c21e4d7d3409e7a259369ac/openhands/app_server/conversation_paths.py#L12);
  [`event/event_service_base.py:89`](https://github.com/OpenHands/OpenHands/blob/11ca68ab2e15dcd85c21e4d7d3409e7a259369ac/openhands/app_server/event/event_service_base.py#L89)),
  SDK events stored by id rather than index (inferred: the app server relays SDK events; field
  shapes not re-read).
- Legacy 0.x: `.openhands/sessions/<sid>/events/<n>.json` with top-level `id`, `timestamp`,
  `source`, `message`, `cause`, `action` or `observation`, `tool_call_metadata`, `llm_metrics`, and
  `args`/`extras`
  ([`openhands/events/serialization/event.py:15-25`](https://github.com/OpenHands/OpenHands/blob/7fbb48c40679afd674970966b96185657d92a487/openhands/events/serialization/event.py#L15-L25);
  [`openhands/storage/locations.py:1-18`](https://github.com/OpenHands/OpenHands/blob/7fbb48c40679afd674970966b96185657d92a487/openhands/storage/locations.py#L1-L18)).
  A different parser.

### Secrets

`base_state.json` `agent.llm.api_key`, `aws_secret_access_key`
([`llm/llm.py:277`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/llm/llm.py#L277),
[`327`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/llm/llm.py#L327))
and `secret_registry`, and `meta.json` `secrets`, are Fernet ciphertext when the server has
`OH_SECRET_KEY` (in `.openhands/agent-canvas/secret-key.txt`), else redacted as `**********`
([`conversation/state.py:435-447`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/conversation/state.py#L435-L447);
[`utils/pydantic_secrets.py:48-78`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/utils/pydantic_secrets.py#L48-L78)).
`thinking_blocks[].signature`, `responses_reasoning_item.encrypted_content` are opaque blobs.
Terminal output in observations and bash events can contain pasted tokens.

The Canvas `Local Storage` key `openhands-backends` ([Other stores](#other-stores)) stores each
backend's `apiKey` in plain JSON, an OpenHands Cloud key for a cloud backend
([`src/api/backend-registry/types.ts:4-22`](https://github.com/OpenHands/OpenHands/blob/8d39305538c1d17a30cbe477e57a0e18fcee7d95/src/api/backend-registry/types.ts#L4-L22);
[`storage.ts:95-101`](https://github.com/OpenHands/OpenHands/blob/8d39305538c1d17a30cbe477e57a0e18fcee7d95/src/api/backend-registry/storage.ts#L95-L101)).
It shares the LevelDB files with the conversation metadata, so it is collected unflagged and the
key is the one to redact.

### Parser plan

Globs: `.openhands/agent-canvas/dev_conversations/*/events/event-*.json`,
`.openhands/agent-canvas/conversations/*/events/event-*.json`,
`.openhands/conversations/*/events/event-*.json`; read the sibling `meta.json` and `base_state.json`
and every event file once per conversation. The same directories' `base_state.json` is also read
as an artifact of its own for the standing rows, and
`.openhands/projects/*/prompt_history.json` as a secondary source. Optional: `bash_events/*`. One
row per event; an `ActionEvent` also yields a `thinking` row for `thought`/`reasoning_content`.
Detect and report legacy `sessions/*/events/*.json` without parsing.

| Column | Source |
| --- | --- |
| timestamp_utc | event `timestamp` (naive local; convert with the host zone, else mark) |
| session_id | conversation directory name (hex) or `meta.json` `id` |
| project_path | `workspace.working_dir` |
| git_branch | empty |
| turn_type | `MessageEvent` by `llm_message.role`; `ActionEvent`, `ACPToolCallEvent` → tool_use; `ObservationEvent`, `UserRejectObservation` → tool_result; others → system |
| model | the model in force at the event ([Model rows](#model-rows)), else `base_state.json` `agent.llm.model` |
| tool_name, tool_use_id | `tool_name`, `tool_call_id` |
| text | message text; `action.command` or `tool_call.arguments`; observation `content[].text`; `error`; `summary` |

#### Permission rows

Written ([issue 96](https://github.com/seanthegeek/doubleagent/issues/96) and
[issue 97](https://github.com/seanthegeek/doubleagent/issues/97)):

- `permission decision: rejected rejection_source=<rejection_source> <rejection_reason>`: a `system`
  row immediately after the `UserRejectObservation`'s `tool_result` row, with its `tool_name` and
  `tool_call_id`, from `rejection_source` (`user` or `hook`; `-` when missing) and
  `rejection_reason`.
- ` security_risk=<value>` at the end of an `ActionEvent`'s `tool_use` text, from `security_risk`.

An approval writes nothing, so none can be shown. Also written, from the records of
[Permission records](#permission-records), source-derived; not observed on any real install:

- `permission decision: analyzed risk=<risk> analyzer=<analyzer> policy=<policy>[
  details=<JSON>]`: one `system` row per entry of a `SecurityAnalysisEvent`'s `risks`, at the
  event's time, with the `tool_name` and `tool_call_id` (as `tool_use_id`) of the `ActionEvent`
  whose `id` is the key; ` action_id=<id>` takes their place when no event file of the
  conversation has that id, and ` details=` carries the entry of `details` for that action as
  compact JSON when there is one.
- Standing state, from `base_state.json` read as an artifact of its own, at its modification time:
  `permission setting: confirmation_policy=<kind>[ threshold=<threshold>][
  confirm_unknown=<true|false>] (from base_state.json, file mtime)`,
  `permission setting: security_analyzer=<kind> (from base_state.json, file mtime)` (`none` when
  the key is absent, which with `exclude_none` means no analyzer), and one
  `permission setting: blocked_actions.<action id>=<reason> (from base_state.json, file mtime)`
  per entry, with the blocked action's `tool_name` and `tool_use_id` when its `ActionEvent` is on
  disk. The values are the state at the last write, not its history.

#### Model rows

Written ([issue 96](https://github.com/seanthegeek/doubleagent/issues/96) and
[issue 97](https://github.com/seanthegeek/doubleagent/issues/97)): `model: <id>`, a `system` row
after the conversation's header row, at the header's time, once per conversation.

From the records of [Model records](#model-records), source-derived; not observed on any real
install:

- The `<id>` of that first row is the model of the conversation's first response, from
  `stats.usage_to_metrics.*.token_usages[]` joined on the event's `llm_response_id`, when one is
  recorded before any switch; else `base_state.json` `agent.llm.model`, which holds only the
  current model. With no response joined and a switch first, the first row is the switch's.
- `model: <old> -> <new>` before the first event of a response whose model differs from the one
  in force (a switch through the REST endpoints between two responses shows only this way).
- `model: <old> -> <active_model> profile=<profile>[ class=<chosen_class>]` after a successful
  `SwitchLLMObservation` (`profile_name`) or `ClassifyAndSwitchLLMObservation` (`model`, the
  profile name, and `chosen_class`), at the observation's time.
- `model: <old> -> <new> (from base_state.json agent.llm, file mtime)` on `base_state.json` at its
  modification time, when `agent.llm.model` differs from the last model the events show: a REST
  switch after the last response.
- The `model` column of each event row is the model in force at that event.

#### Past-use rows and secondary sources

- `.openhands/projects/<sha256 of cwd>/prompt_history.json` (CLI; secondary,
  [CLI prompt history](#cli-prompt-history)), source-derived; not observed on any real install:
  one `user` row per entry, `text` as the row text, `timestamp` emitted as if UTC (as the CLI's
  conversations are, since the CLI writes no `meta.json` to anchor the zone), an empty
  `session_id`, and as `project_path` the `workspace.working_dir` of a conversation under the same
  `.openhands` whose SHA-256 equals the directory name, else empty. It joins the CLI conversation's
  `user` rows on text and time; what only it holds is kept.
- A fork's directory ([What survives a delete](#what-survives-a-delete)): primary, parsed as its
  own conversation.
- The Canvas browser-storage keys ([Other stores](#other-stores)) hold no message text and repeat
  no event; not read.
- The global `agent-canvas/bash_events/` (terminal commands typed in Canvas) is a record of its
  own, not a copy, and is not read.

The agent keeps no search index or summaries database of conversations: not found in
software-agent-sdk at
[`33d8aa2`](https://github.com/OpenHands/software-agent-sdk/commit/33d8aa29c3078d074799e2fbf5e847aeb2d71360)
(grepped for `sqlite3`, `fts5` and `search_index`, and for `summar`, whose only hits under
`openhands-agent-server/` are profile-store listings; the one `sqlite` hit is a system library in
the Docker image).

#### Sync, relay and origin rows

Sync: OpenHands records no sync state on disk. Two features send work to OpenHands Cloud (the
vendor's hosted service at `app.all-hands.dev`, which runs each conversation in a sandbox of its
own), and neither writes a local file that names the remote conversation:

- `openhands cloud -t <task>` (or `-f <file>`) posts the task text and the GitHub or GitLab
  `owner/repo` and branch of the working directory's `origin` remote to `/api/v1/app-conversations`,
  then prints the remote conversation id and its browser URL
  ([`openhands_cli/cloud/conversation.py:54-133`](https://github.com/OpenHands/OpenHands-CLI/blob/0403f1a79a9fc62a6c7be21c17d7b51f64245ad2/openhands_cli/cloud/conversation.py#L54-L133);
  [`auth/api_client.py:109-112`](https://github.com/OpenHands/OpenHands-CLI/blob/0403f1a79a9fc62a6c7be21c17d7b51f64245ad2/openhands_cli/auth/api_client.py#L109-L112);
  [`argparsers/cloud_parser.py:16-45`](https://github.com/OpenHands/OpenHands-CLI/blob/0403f1a79a9fc62a6c7be21c17d7b51f64245ad2/openhands_cli/argparsers/cloud_parser.py#L16-L45)).
  The id goes to stdout only; the CLI's only file writes are the API key, `cli_config.json`, the
  prompt history and the SDK's local conversation store, which this command does not open
  ([`auth/token_storage.py:27-37`](https://github.com/OpenHands/OpenHands-CLI/blob/0403f1a79a9fc62a6c7be21c17d7b51f64245ad2/openhands_cli/auth/token_storage.py#L27-L37);
  [`stores/cli_settings.py:56-62`](https://github.com/OpenHands/OpenHands-CLI/blob/0403f1a79a9fc62a6c7be21c17d7b51f64245ad2/openhands_cli/stores/cli_settings.py#L56-L62);
  [`stores/prompt_history.py:92-93`](https://github.com/OpenHands/OpenHands-CLI/blob/0403f1a79a9fc62a6c7be21c17d7b51f64245ad2/openhands_cli/stores/prompt_history.py#L92-L93)).
  A cloud-backed conversation store is a stub that raises `NotImplementedError`
  ([`conversations/store/cloud.py:10-50`](https://github.com/OpenHands/OpenHands-CLI/blob/0403f1a79a9fc62a6c7be21c17d7b51f64245ad2/openhands_cli/conversations/store/cloud.py#L10-L50)).
- `openhands acp --cloud` (ACP, the Agent Client Protocol an editor such as Zed uses to drive the
  CLI as its agent) runs every session as an SDK `RemoteConversation` in an OpenHands Cloud
  workspace: the events are kept by the agent server inside the cloud sandbox, and the call that
  builds the conversation passes no `persistence_dir`, unlike the local agent, so nothing is written
  under `.openhands/conversations/`
  ([`acp_impl/agent/remote_agent.py:186-258`](https://github.com/OpenHands/OpenHands-CLI/blob/0403f1a79a9fc62a6c7be21c17d7b51f64245ad2/openhands_cli/acp_impl/agent/remote_agent.py#L186-L258);
  [`acp_impl/agent/local_agent.py:188`](https://github.com/OpenHands/OpenHands-CLI/blob/0403f1a79a9fc62a6c7be21c17d7b51f64245ad2/openhands_cli/acp_impl/agent/local_agent.py#L188);
  [`argparsers/acp_parser.py:32-46`](https://github.com/OpenHands/OpenHands-CLI/blob/0403f1a79a9fc62a6c7be21c17d7b51f64245ad2/openhands_cli/argparsers/acp_parser.py#L32-L46)).
  The only local file a remote conversation can leave is an LLM completion log, and only when
  `log_completions` is set on the agent's LLM
  ([`sdk/conversation/impl/remote_conversation.py:1251-1282`](https://github.com/OpenHands/software-agent-sdk/blob/fd268089e257bee4ad704ce4fdc6466bc756c71c/openhands-sdk/openhands/sdk/conversation/impl/remote_conversation.py#L1251-L1282)).
  The session id the editor holds is the remote conversation id; the editor's own state is outside
  this catalog entry.

Both need `openhands login`, which stores the account key in `.openhands/cloud/api_key.txt`
([Credentials](#credentials)) and pulls the account's settings down rather than pushing anything up
([`auth/login_command.py:31`](https://github.com/OpenHands/OpenHands-CLI/blob/0403f1a79a9fc62a6c7be21c17d7b51f64245ad2/openhands_cli/auth/login_command.py#L31);
[`auth/api_client.py:307-347`](https://github.com/OpenHands/OpenHands-CLI/blob/0403f1a79a9fc62a6c7be21c17d7b51f64245ad2/openhands_cli/auth/api_client.py#L307-L347)).
Neither has a local delete: the CLI has no conversation delete command, and the remote copy is
managed in the web app, whose code is not in these repositories.

Other traffic that leaves the host beyond the model call, none of which carries a session marker:

- The critic (an OpenHands model that scores whether the agent's run succeeded) is on by default
  (`critic.enable_critic` in `.openhands/cli_config.json`) for models served through the OpenHands
  provider, and sends the conversation's messages and tools to the vendor's LLM proxy at
  `llm-proxy.app.all-hands.dev/vllm`
  ([`stores/cli_settings.py:23-26`](https://github.com/OpenHands/OpenHands-CLI/blob/0403f1a79a9fc62a6c7be21c17d7b51f64245ad2/openhands_cli/stores/cli_settings.py#L23-L26);
  [`stores/agent_store.py:91-130`](https://github.com/OpenHands/OpenHands-CLI/blob/0403f1a79a9fc62a6c7be21c17d7b51f64245ad2/openhands_cli/stores/agent_store.py#L91-L130);
  [`sdk/critic/impl/api/critic.py:58-105`](https://github.com/OpenHands/software-agent-sdk/blob/fd268089e257bee4ad704ce4fdc6466bc756c71c/openhands-sdk/openhands/sdk/critic/impl/api/critic.py#L58-L105)).
  Each critic result is also sent to PostHog (a product analytics service) as a `critic_inference`
  event, and each feedback button press other than dismiss as `critic_feedback`, with the raw local
  conversation id as the PostHog identity, the score, the agent model and the critic's `event_ids`,
  but no message text, with no consent check
  ([`tui/utils/critic/feedback.py:29-79`](https://github.com/OpenHands/OpenHands-CLI/blob/0403f1a79a9fc62a6c7be21c17d7b51f64245ad2/openhands_cli/tui/utils/critic/feedback.py#L29-L79),
  [`228-270`](https://github.com/OpenHands/OpenHands-CLI/blob/0403f1a79a9fc62a6c7be21c17d7b51f64245ad2/openhands_cli/tui/utils/critic/feedback.py#L228-L270);
  [`tui/widgets/richlog_visualizer.py:332-345`](https://github.com/OpenHands/OpenHands-CLI/blob/0403f1a79a9fc62a6c7be21c17d7b51f64245ad2/openhands_cli/tui/widgets/richlog_visualizer.py#L332-L345)).
  The `posthog` package arrives through `browser-use`, so it is present in a source install
  ([`uv.lock:263-290`](https://github.com/OpenHands/OpenHands-CLI/blob/0403f1a79a9fc62a6c7be21c17d7b51f64245ad2/uv.lock#L263-L290)).
- The Agent Server's product telemetry is off by default (consent `unset` is not consent, the
  exporter defaults to `none`), allowlists scalar fields so no prompt or reply text can be sent, and
  pseudonymises conversation ids; the consent is kept in `.openhands/settings.json`
  `misc_settings.telemetry.consent` (`granted`, `denied` or `unset`)
  ([`agent_server/telemetry/policy.py:160-216`](https://github.com/OpenHands/software-agent-sdk/blob/fd268089e257bee4ad704ce4fdc6466bc756c71c/openhands-agent-server/openhands/agent_server/telemetry/policy.py#L160-L216);
  [`agent_server/config.py:151-172`](https://github.com/OpenHands/software-agent-sdk/blob/fd268089e257bee4ad704ce4fdc6466bc756c71c/openhands-agent-server/openhands/agent_server/config.py#L151-L172);
  [`agent_server/telemetry/models.py:1-16`](https://github.com/OpenHands/software-agent-sdk/blob/fd268089e257bee4ad704ce4fdc6466bc756c71c/openhands-agent-server/openhands/agent_server/telemetry/models.py#L1-L16);
  [`agent_server/telemetry/factory.py:112-124`](https://github.com/OpenHands/software-agent-sdk/blob/fd268089e257bee4ad704ce4fdc6466bc756c71c/openhands-agent-server/openhands/agent_server/telemetry/factory.py#L112-L124);
  [`agent_server/persistence/store.py:821-829`](https://github.com/OpenHands/software-agent-sdk/blob/fd268089e257bee4ad704ce4fdc6466bc756c71c/openhands-agent-server/openhands/agent_server/persistence/store.py#L821-L829)).
- Agent Server webhooks post events and conversation info to a configured URL; the list is empty by
  default and is set by whoever launches the server, not stored per conversation
  ([`agent_server/config.py:91-105`](https://github.com/OpenHands/software-agent-sdk/blob/fd268089e257bee4ad704ce4fdc6466bc756c71c/openhands-agent-server/openhands/agent_server/config.py#L91-L105),
  [`322-325`](https://github.com/OpenHands/software-agent-sdk/blob/fd268089e257bee4ad704ce4fdc6466bc756c71c/openhands-agent-server/openhands/agent_server/config.py#L322-L325)).

The trajectory export (`GET /download-trajectory/<id>`, Agent Canvas's download) is a download to
the user, not an upload: it zips the conversation directory with credential fields redacted into a
temporary `<hex>.zip` beside it and deletes the zip once the response is sent
([`agent_server/file_router.py:179-216`](https://github.com/OpenHands/software-agent-sdk/blob/fd268089e257bee4ad704ce4fdc6466bc756c71c/openhands-agent-server/openhands/agent_server/file_router.py#L179-L216),
[`937-960`](https://github.com/OpenHands/software-agent-sdk/blob/fd268089e257bee4ad704ce4fdc6466bc756c71c/openhands-agent-server/openhands/agent_server/file_router.py#L937-L960)).
No `session synced:` row is proposed. Searched: `cloud`, `share`, `upload`, `sync`, `export`,
`trajectory`, `feedback`, `posthog`, `telemetry`, `webhook`, `remote`, `app_conversation_id`,
`sandbox_id`, `all-hands.dev` in `openhands_cli/`, `openhands-sdk/`, `openhands-agent-server/` and
`openhands-workspace/`. The OpenHands Cloud server is not in these repositories, so a share or sync
feature of its own is not covered here. The Agent Canvas front end is in
[OpenHands/OpenHands](https://github.com/OpenHands/OpenHands); its origin records follow.

Origin: not yet written. A Canvas backend of `kind` `cloud` lists OpenHands Cloud conversations,
and a backend with `marsSessionId` is a DigitalOcean Managed Agents session
([Electron app](#electron-app))
([`src/api/mars/AGENTS.md:3`](https://github.com/OpenHands/OpenHands/blob/8d39305538c1d17a30cbe477e57a0e18fcee7d95/src/api/mars/AGENTS.md#L3),
[`13`](https://github.com/OpenHands/OpenHands/blob/8d39305538c1d17a30cbe477e57a0e18fcee7d95/src/api/mars/AGENTS.md#L13);
[`src/api/cloud/conversation-service.api.ts:70-104`](https://github.com/OpenHands/OpenHands/blob/8d39305538c1d17a30cbe477e57a0e18fcee7d95/src/api/cloud/conversation-service.api.ts#L70-L104)).
Their transcripts never reach `~/.openhands`; the only local traces are the `localStorage` keys of
[Other stores](#other-stores). A `session origin: remote openhands-cloud` or
`remote mars session <id>` row would need a LevelDB reader.

Relay: the agent records none. No messaging relay was found in OpenHands at
[`8d39305`](https://github.com/OpenHands/OpenHands/commit/8d39305538c1d17a30cbe477e57a0e18fcee7d95),
software-agent-sdk at
[`33d8aa2`](https://github.com/OpenHands/software-agent-sdk/commit/33d8aa29c3078d074799e2fbf5e847aeb2d71360)
or OpenHands-CLI at
[`0403f1a`](https://github.com/OpenHands/OpenHands-CLI/commit/0403f1a79a9fc62a6c7be21c17d7b51f64245ad2)
(grepped for `telegram`, `slack`, `discord`, `whatsapp` and `jira`; the last is only a cloud
feature flag).

#### Scratch directory rows

The analyzer's catalog rule already classes `.openhands/agent-canvas/workspaces/<hex>` and
`.openhands/workspaces/<run>` (both under the `.openhands` entry). A worktree conversation's
`workspace.working_dir` under `/tmp/conversation-worktrees/<dir>`
([Scratch directories](#scratch-directories);
[`agent_server/local_storage.py:14-23`](https://github.com/OpenHands/software-agent-sdk/blob/33d8aa29c3078d074799e2fbf5e847aeb2d71360/openhands-agent-server/openhands/agent_server/local_storage.py#L14-L23))
is outside every home, so the parser writes
`scratch directory: <working_dir> (conversation worktree under the Agent Server's default
conversation_worktree_root /tmp/conversation-worktrees)` after the conversation's model row, at the
header's time, with the `working_dir` as `project_path`; `/private/tmp/conversation-worktrees/`,
the same root as macOS resolves it, counts too. Source-derived; not observed on any real install.
The root is configurable, so a non-default root cannot be recognised from the conversation's files.

#### Deletion rows

The agent records no delete, so the parser writes no deletion row: nothing local records a deleted
conversation ([What survives a delete](#what-survives-a-delete)).

#### Bad-line rows

No file is read line by line, and an event file that does not decode gives one
`parser: event file could not be decoded (<reason>)` row. A `meta.json` or `base_state.json` that
exists but cannot be read, does not decode or is not a JSON object is read as empty and gives one
whole-file `system` row on the conversation's first event file, after the header and model rows:
`parser: <file name> could not be decoded (<reason>)`,
`parser: <file name> could not be read (<reason>)` or `parser: <file name> is not a JSON object`,
with an empty timestamp and the session id and `project_path` the other file still gives. The
header then lacks what the file held (title, model, time basis). A missing file is not an error,
since the CLI writes no `meta.json`. A `prompt_history.json` that cannot be read is one whole-file
row `parser: <file name> could not be read (<reason>)` through `error_rows`. The cut-file shape is
not observed on any real install, source-derived (OpenHands/software-agent-sdk at
[`33d8aa2`](https://github.com/OpenHands/software-agent-sdk/commit/33d8aa29c3078d074799e2fbf5e847aeb2d71360)).

`prompt_history.json` is read as a JSON array element by element: a record that does not decode
(a file cut mid-write) is one `parser: unparseable record <N> (<reason>): <the text from that
record to the end of the file>` row after the rows of the records before it, an element that is
not an object is `parser: unparseable record <N> (not a JSON object): <the element>`, and a file
that is not an array or cannot be read is one whole-file `parser:` row.

#### Fixtures

Fixture (`.openhands/conversations/0f0e0d0c111140008000000000000001/`):

`meta.json`:

```json
{"id":"0f0e0d0c-1111-4000-8000-000000000001","title":"List files","created_at":"2026-10-01T10:00:00Z","updated_at":"2026-10-01T10:00:05Z","workspace":{"kind":"LocalWorkspace","working_dir":"/home/alice/proj"}}
```

```json
{"kind":"MessageEvent","id":"6b1d5c2e-0000-4000-8000-000000000001","timestamp":"2026-10-01T12:00:01.000000","source":"user","llm_message":{"role":"user","content":[{"type":"text","text":"list the files"}]},"activated_skills":[],"extended_content":[]}
{"kind":"ActionEvent","id":"6b1d5c2e-0000-4000-8000-000000000002","timestamp":"2026-10-01T12:00:03.000000","source":"agent","thought":[{"type":"text","text":"I will run ls."}],"action":{"kind":"TerminalAction","command":"ls"},"tool_name":"terminal","tool_call_id":"call_1","tool_call":{"id":"call_1","name":"terminal","arguments":"{\"command\":\"ls\"}","origin":"completion"},"llm_response_id":"resp_1","security_risk":"LOW"}
{"kind":"ObservationEvent","id":"6b1d5c2e-0000-4000-8000-000000000003","timestamp":"2026-10-01T12:00:04.000000","source":"environment","tool_name":"terminal","tool_call_id":"call_1","action_id":"6b1d5c2e-0000-4000-8000-000000000002","observation":{"kind":"TerminalObservation","content":[{"type":"text","text":"README.md"}],"is_error":false,"command":"ls","exit_code":0}}
```

(files `events/event-00000-6b1d5c2e-...0001.json` and so on, one object each).

The permission and model records, from the shapes of [Permission records](#permission-records) and
[Model records](#model-records) (a worktree conversation, so its `working_dir` is under
`/tmp/conversation-worktrees/`):

```json
{"kind":"SecurityAnalysisEvent","id":"8a2f0c1d-0000-4000-8000-000000000002","timestamp":"2026-10-03T11:00:02.500000","source":"environment","analyzer":"LLMSecurityAnalyzer","policy":"ConfirmRisky","risks":{"8a2f0c1d-0000-4000-8000-000000000001":"LOW"},"details":{"8a2f0c1d-0000-4000-8000-000000000001":{"rationale":"runs the test suite"}}}
{"kind":"ObservationEvent","id":"8a2f0c1d-0000-4000-8000-000000000005","timestamp":"2026-10-03T11:00:05.000000","source":"environment","tool_name":"switch_llm","tool_call_id":"call_w2","action_id":"8a2f0c1d-0000-4000-8000-000000000004","observation":{"kind":"SwitchLLMObservation","content":[{"type":"text","text":"Switched LLM profile to 'fast'"}],"is_error":false,"profile_name":"fast","reason":"cheaper","active_model":"openai/gpt-5-mini"}}
```

`base_state.json`, the fields beside `agent.llm.model`:

```json
{"confirmation_policy":{"kind":"ConfirmRisky","threshold":"HIGH","confirm_unknown":true},"security_analyzer":{"kind":"LLMSecurityAnalyzer"},"blocked_actions":{"8a2f0c1d-0000-4000-8000-000000000006":"rm is blocked by hook"},"stats":{"usage_to_metrics":{"agent":{"model_name":"litellm_proxy/claude-sonnet-4-5","token_usages":[{"model":"litellm_proxy/claude-sonnet-4-5","prompt_tokens":10,"completion_tokens":5,"response_id":"resp_w1"}]}}}}
```

`.openhands/projects/<sha256 of /srv/proj>/prompt_history.json`:

```json
[{"text":"list the files","timestamp":"2026-10-02T08:30:01.000000"},{"text":"and the hidden files?","timestamp":"2026-10-02T08:45:00.000000"}]
```

#### Source host rows

The agent records no connected source-host account, so the parser writes no `source host:` row.
The Agent Server searches the user's GitHub repositories (`/git/repositories/search` on its
API router, GitHub only) with a token it reads from its secrets store under the names
`github_token`, `GITHUB_TOKEN`, `GH_TOKEN` or `github`, and uses the `github_token`,
`gitlab_token` or `bitbucket_token` secret to clone; it never asks the host who the token
belongs to, and returns the repository list (`id`, `full_name`, `git_provider`, `is_public`,
`pushed_at`, `main_branch`) without storing it
([`agent_server/git_provider_service.py:26-31`](https://github.com/OpenHands/software-agent-sdk/blob/33d8aa29c3078d074799e2fbf5e847aeb2d71360/openhands-agent-server/openhands/agent_server/git_provider_service.py#L26-L31),
[`54-87`](https://github.com/OpenHands/software-agent-sdk/blob/33d8aa29c3078d074799e2fbf5e847aeb2d71360/openhands-agent-server/openhands/agent_server/git_provider_service.py#L54-L87),
[`211-220`](https://github.com/OpenHands/software-agent-sdk/blob/33d8aa29c3078d074799e2fbf5e847aeb2d71360/openhands-agent-server/openhands/agent_server/git_provider_service.py#L211-L220);
[`agent_server/git_router.py:35`](https://github.com/OpenHands/software-agent-sdk/blob/33d8aa29c3078d074799e2fbf5e847aeb2d71360/openhands-agent-server/openhands/agent_server/git_router.py#L35),
[`129-150`](https://github.com/OpenHands/software-agent-sdk/blob/33d8aa29c3078d074799e2fbf5e847aeb2d71360/openhands-agent-server/openhands/agent_server/git_router.py#L129-L150);
[`openhands-sdk/openhands/sdk/workspace/repo.py:46-50`](https://github.com/OpenHands/software-agent-sdk/blob/33d8aa29c3078d074799e2fbf5e847aeb2d71360/openhands-sdk/openhands/sdk/workspace/repo.py#L46-L50)).
The token is a custom secret in `.openhands/secrets.json` (`custom_secrets.<name>.secret`),
Fernet ciphertext under the master key or plaintext without one
([`agent_server/persistence/store.py:575-586`](https://github.com/OpenHands/software-agent-sdk/blob/33d8aa29c3078d074799e2fbf5e847aeb2d71360/openhands-agent-server/openhands/agent_server/persistence/store.py#L575-L586);
[`agent_server/persistence/models.py:519-527`](https://github.com/OpenHands/software-agent-sdk/blob/33d8aa29c3078d074799e2fbf5e847aeb2d71360/openhands-agent-server/openhands/agent_server/persistence/models.py#L519-L527)),
and the file is flagged ([Credentials](#credentials)); a conversation's `meta.json` `secrets` can
hold the same names, as the fixture's `GITHUB_TOKEN` does. Agent Canvas keeps the last three
repositories the user picked, with `git_provider` and `full_name`, under the `localStorage` key
`home-store` (`recentRepositories`, `lastSelectedProvider`)
([`src/stores/home-store.ts:6-9`](https://github.com/OpenHands/OpenHands/blob/8d39305538c1d17a30cbe477e57a0e18fcee7d95/src/stores/home-store.ts#L6-L9),
[`60-63`](https://github.com/OpenHands/OpenHands/blob/8d39305538c1d17a30cbe477e57a0e18fcee7d95/src/stores/home-store.ts#L60-L63);
[`src/types/git.d.ts:58-67`](https://github.com/OpenHands/OpenHands/blob/8d39305538c1d17a30cbe477e57a0e18fcee7d95/src/types/git.d.ts#L58-L67)),
and the repository bound to each conversation under
`openhands-agent-server-conversation-metadata` ([Agent Canvas browser
storage](#agent-canvas-browser-storage)); both are LevelDB in the Electron app, which the analyzer
does not read. An OpenHands Cloud conversation started on a repository (`openhands cloud`, a
Canvas cloud backend) is stored server-side only
([Sync, relay and origin rows](#sync-relay-and-origin-rows)). Grepped for `github`, `gitlab`,
`bitbucket`, `provider_tokens`, `git_provider`, `repositor` and `oauth` in software-agent-sdk at
[`33d8aa2`](https://github.com/OpenHands/software-agent-sdk/commit/33d8aa29c3078d074799e2fbf5e847aeb2d71360),
OpenHands-CLI at
[`0403f1a`](https://github.com/OpenHands/OpenHands-CLI/commit/0403f1a79a9fc62a6c7be21c17d7b51f64245ad2)
and OpenHands at
[`8d39305`](https://github.com/OpenHands/OpenHands/commit/8d39305538c1d17a30cbe477e57a0e18fcee7d95).

## Confidence

- High: every path in [Per-user storage](#per-user-storage) and [Credentials](#credentials) under
  `~/.openhands` (constants and env defaults in the cited files), the Fernet scheme, the
  Docker-runtime bind mounts, the CLI and legacy layouts at the cited commits.
- High: the conversation directory layout, file naming, the `kind` discriminator, the event field
  names, the naive local `timestamp`, and the `security_risk` values `UNKNOWN`, `LOW`, ...
  ([`security/risk.py:13-21`](https://github.com/OpenHands/software-agent-sdk/blob/b347047e2dcdd8f4b2be810aa8bc6632bc756d41/openhands-sdk/openhands/sdk/security/risk.py#L13-L21)).
- Medium: the Electron user data directory (Electron convention from `productName`; the app does
  not set it explicitly); `agent-canvas/storage` contents (created for the automation file store,
  not traced); whether `secrets.json` in the legacy root used the same field names; the 1.x
  `v1_conversations` event shape.
- Not determined: Agent Server file logs (`LOG_DIR` defaults to a relative `logs`, so the cwd of
  the server); the automation service's tarballs under `storage/`; whether a Canvas install leaves
  a uv tool environment worth collecting (uv caches are outside this tool's directories); whether
  any surface writes a zone-aware event timestamp; the full set of action and observation `kind`
  values, which grows with each tool.
