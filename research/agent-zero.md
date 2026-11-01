# agent-zero

Agent Zero (catalog agent `agent-zero`) is agent0ai's open-source Python agent, usually run in a
Docker container whose install directory is `/a0`; it is driven from its web UI, from messaging
plugins (Telegram, WhatsApp, email), from editors through ACP (the Agent Client Protocol) and from
the host-side A0 CLI connector. Three companions put its data on the host: the A0 Install scripts,
the A0 Launcher (an Electron app that manages instances) and the A0 CLI connector (a host-side
client). Its Orchestrator plugin can drive other coding CLIs, Hermes among them. One JSON file per
chat holds two parallel records of the conversation: the UI log (timestamped, truncated, last 1000
items) and each agent's LLM history (full text, no timestamps, compressed over time). Parsed: the
agent-zero parser reads `chat.json`, its `messages/` files and the legacy per-chat JSON, and beside
them the scheduler's task list, the Telegram chat map, the tool policies, the Orchestrator's
settings, two plugin switches and two keys of the connector's `.env`.

## Source and evidence

[agent0ai/agent-zero](https://github.com/agent0ai/agent-zero) at commit
[`e3051fb584b1a36be2b0a0c90606f1c2c2d356ec`](https://github.com/agent0ai/agent-zero/commit/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec).
The `LICENSE` file at that commit is plain MIT
([`LICENSE:1-4`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/LICENSE#L1-L4)),
not a custom licence. Python, source public. Three companion repositories decide where the data
lands on the host and were also read: [agent0ai/a0-install](https://github.com/agent0ai/a0-install)
at
[`c89b4026e9f16f53389e75537783411fefdd53f1`](https://github.com/agent0ai/a0-install/commit/c89b4026e9f16f53389e75537783411fefdd53f1)
(installer scripts), [agent0ai/a0-launcher](https://github.com/agent0ai/a0-launcher) at
[`b0333d80b5abf38d8481f55b667b0cf23d805bbb`](https://github.com/agent0ai/a0-launcher/commit/b0333d80b5abf38d8481f55b667b0cf23d805bbb)
(Electron launcher, MIT,
[`LICENSE:1`](https://github.com/agent0ai/a0-launcher/blob/b0333d80b5abf38d8481f55b667b0cf23d805bbb/LICENSE#L1)),
and [agent0ai/a0-connector](https://github.com/agent0ai/a0-connector) at
[`76e834f20e50f6e47954c89ee39dd4c5b50d81b7`](https://github.com/agent0ai/a0-connector/commit/76e834f20e50f6e47954c89ee39dd4c5b50d81b7)
(the host-side "A0 CLI", MIT,
[`LICENSE:1`](https://github.com/agent0ai/a0-connector/blob/76e834f20e50f6e47954c89ee39dd4c5b50d81b7/LICENSE#L1)).
All claims are from source or the in-repo docs, read only, not run.

### Observed

- No real install: nothing was installed or run.
- The Docker volume layout of chats (`chats/<ctxid>/chat.json` at the volume base,
  [Transcript stores](#transcript-stores)) is not observed on any real install, source-derived.
- The `session synced:` and `session relayed:` rows are fixture only
  ([Sync, relay and origin](#sync-relay-and-origin-rows)).

### Source reviews

- 2026-10-21: agent0ai/agent-zero at
  [`ff2a5b9`](https://github.com/agent0ai/agent-zero/commit/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a)
  and agent0ai/a0-connector at
  [`3741c88`](https://github.com/agent0ai/a0-connector/commit/3741c8809f1f99b4efee2ed1a24234a98bf896d4),
  as reviewed on 2026-10-19: the proposed permission, model, past-use, deletion, scratch and origin
  rows became parser rows, the scheduler's file shape, the OAuth provider tokens, the model
  override's API key and the source-host statement were added, and the collector lines for the
  WhatsApp session, the ACP workspaces and the connector's Windows screen captures are recorded.
- 2026-10-19: agent0ai/agent-zero at
  [`ff2a5b9`](https://github.com/agent0ai/agent-zero/commit/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a)
  (committed 2026-10-13), against
  [`e3051fb`](https://github.com/agent0ai/agent-zero/commit/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec)
  cited above; a0-install unchanged, HEAD is still
  [`c89b402`](https://github.com/agent0ai/a0-install/commit/c89b4026e9f16f53389e75537783411fefdd53f1);
  a0-launcher at
  [`a69403d`](https://github.com/agent0ai/a0-launcher/commit/a69403d9cc92009addb06fedb5585e60e09a3d2c)
  against
  [`b0333d8`](https://github.com/agent0ai/a0-launcher/commit/b0333d80b5abf38d8481f55b667b0cf23d805bbb);
  agent0ai/a0-connector at
  [`3741c88`](https://github.com/agent0ai/a0-connector/commit/3741c8809f1f99b4efee2ed1a24234a98bf896d4)
  against
  [`76e834f`](https://github.com/agent0ai/a0-connector/commit/76e834f20e50f6e47954c89ee39dd4c5b50d81b7).
  The review added the permission records, the per-chat model override, the editor-session keys
  and the host path they record, the records of past use and what a delete leaves, the WhatsApp
  login state, the launcher's own WSL distribution and the connector's screenshots, and recorded
  the small format drift in [Format versions](#format-versions).

## Collection

### Per-user storage

Agent Zero itself never resolves a home directory. Every path is relative to the install directory,
the parent of `helpers/`
([`helpers/files.py:23`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/files.py#L23),[`575-584`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/files.py#L575-L584)),
shown to the agent as `/a0/...`
([`files.py:618-623`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/files.py#L618-L623)).
In the container that is `/a0`; all user state is under `/a0/usr` and `/a0/tmp`, both gitignored
([`.gitignore:26-39`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/.gitignore#L26-L39)).
Where that lands on the host depends on how it was started:

| Deployment | Host location of `/a0/usr` |
| --- | --- |
| A0 Install (`install.sh`, `install.ps1`) | `~/agent-zero/<container-name>/usr`, override `--data-dir` ([`install.sh:1630`](https://github.com/agent0ai/a0-install/blob/c89b4026e9f16f53389e75537783411fefdd53f1/install.sh#L1630),[`1650-1652`](https://github.com/agent0ai/a0-install/blob/c89b4026e9f16f53389e75537783411fefdd53f1/install.sh#L1650-L1652),[`1830`](https://github.com/agent0ai/a0-install/blob/c89b4026e9f16f53389e75537783411fefdd53f1/install.sh#L1830); [`install.ps1:30`](https://github.com/agent0ai/a0-install/blob/c89b4026e9f16f53389e75537783411fefdd53f1/install.ps1#L30),[`1856`](https://github.com/agent0ai/a0-install/blob/c89b4026e9f16f53389e75537783411fefdd53f1/install.ps1#L1856),[`1874`](https://github.com/agent0ai/a0-install/blob/c89b4026e9f16f53389e75537783411fefdd53f1/install.ps1#L1874)) |
| A0 Launcher, default "host directory" mode | `~/agent-zero/<instance>[-N]/usr` ([`state_store.js:144-149`](https://github.com/agent0ai/a0-launcher/blob/b0333d80b5abf38d8481f55b667b0cf23d805bbb/shell/docker_manager/state_store.js#L144-L149), [`index.js:890-899`](https://github.com/agent0ai/a0-launcher/blob/b0333d80b5abf38d8481f55b667b0cf23d805bbb/shell/docker_manager/index.js#L890-L899)) |
| A0 Launcher, "named volume" mode | Docker volume `a0-launcher-<slug>-usr` ([`index.js:948-951`](https://github.com/agent0ai/a0-launcher/blob/b0333d80b5abf38d8481f55b667b0cf23d805bbb/shell/docker_manager/index.js#L948-L951)) |
| README `docker run` | Docker volume `a0_usr` ([`README.md:92`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/README.md#L92)) |
| `docker/run/docker-compose.yml` | `./agent-zero` beside the compose file, mounted as all of `/a0` ([`docker-compose.yml:5-6`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/docker/run/docker-compose.yml#L5-L6)) |
| Native ("development") run | the git clone, anywhere; the docs example is `~/Desktop/agent-zero` ([`docs/setup/dev-setup.md:34-37`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/docs/setup/dev-setup.md#L34-L37)); native means not `--dockerized` ([`helpers/runtime.py:59-64`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/runtime.py#L59-L64)) |

Named volumes live under the Docker data root (or inside the Docker Desktop
VM), outside any home, and are not reachable by a home-relative catalog;
since collector 1.5.0 the `DOCKER_VOLUMES` table collects `*a0_usr` and
`a0-launcher-*-usr` volumes with usr-relative exclusions and secret globs. Only
the native run has `usr/` and `tmp/` beside the source, a `.venv` or
`.conda`, and a legacy root `.env`.

#### Scratch directories

`usr/workdir` is the working directory of a chat with no project
([`helpers/settings.py:604`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/helpers/settings.py#L604)),
and `usr/projects/<name>` directories, clones included, belong to the agent's own tree; both are
collected under the `agent-zero` entries and the volumes. Chats name them by container path
(`/a0/usr/...`), which the analyzer's catalog rule cannot match to a home path. An editor session
on the `container` transport can name its own working directory inside the container
([Editor sessions](#editor-sessions)). The connector's screenshot directory
([Host-side companions](#host-side-companions)) is the one scratch location outside `usr/`; it is
collected only on Windows, where the system temp directory is inside the home.

#### Contents of usr/

- `chats/<ctxid>/chat.json`, one per chat or scheduled-task context, and
  `chats/<ctxid>/messages/<n>.txt` for tool results of 500 characters or more
  ([`helpers/persist_chat.py:17-19`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/persist_chat.py#L17-L19),[`47-48`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/persist_chat.py#L47-L48),
  [`extensions/python/hist_add_tool_result/_90_save_tool_call_file.py:23-40`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/extensions/python/hist_add_tool_result/_90_save_tool_call_file.py#L23-L40)).
- `settings.json`
  ([`helpers/settings.py:182`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/settings.py#L182)),
  `.env`
  ([`helpers/dotenv.py:17-18`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/dotenv.py#L17-L18)),
  `secrets.env`
  ([`helpers/secrets.py:19`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/secrets.py#L19)).
- `memory/<subdir>/`: LangChain FAISS store, `index.faiss`, `index.pkl` (pickled docstore holding
  the memory text), `embedding.json`, `knowledge_import.json`
  ([`plugins/_memory/helpers/memory.py:180`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/plugins/_memory/helpers/memory.py#L180),[`197`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/plugins/_memory/helpers/memory.py#L197),[`269`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/plugins/_memory/helpers/memory.py#L269),[`550`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/plugins/_memory/helpers/memory.py#L550),[`666-673`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/plugins/_memory/helpers/memory.py#L666-L673)).
- `knowledge/` user knowledge files
  ([`memory.py:676-689`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/plugins/_memory/helpers/memory.py#L676-L689)).
- `projects/<name>/` with metadata in `.a0proj/`: `project.json`, `mcp_servers.json`,
  `instructions/`, `knowledge/`, `skills/`, `memory/`, `secrets.env`, `variables.env`
  ([`helpers/projects.py:11-17`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/projects.py#L11-L17),[`77-86`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/projects.py#L77-L86),[`672-684`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/projects.py#L672-L684);
  [`secrets.py:554`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/secrets.py#L554)).
- `workdir/`, the agent's default working directory
  ([`settings.py:597`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/settings.py#L597)).
- `scheduler/tasks.json`, or since [`ff2a5b9`](https://github.com/agent0ai/agent-zero/commit/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a) `scheduler/tasks.yaml` when only the YAML file exists
  ([`helpers/task_scheduler.py:512-515`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/helpers/task_scheduler.py#L512-L515);
  [`helpers/task_scheduler.py:29`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/task_scheduler.py#L29),[`521`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/task_scheduler.py#L521)).
- `agents/`, `skills/`, `plugins/` user extensions
  ([`helpers/subagents.py:12`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/subagents.py#L12),
  [`helpers/skills.py:91`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/skills.py#L91)).
- `uploads/`, `upload/`, `downloads/`, `email/`
  ([`helpers/migration.py:24-31`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/migration.py#L24-L31)).
- `.time_travel/workspaces/`: shadow git history of `usr` workspaces
  ([`plugins/_time_travel/helpers/time_travel.py:23-24`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/plugins/_time_travel/helpers/time_travel.py#L23-L24)).

Older releases kept chats, scheduler, uploads, settings and secrets under `tmp/`, memory under
`memory/` and the `.env` at the install root; startup moves them into `usr/`
([`migration.py:24-38`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/migration.py#L24-L38),[`95-107`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/migration.py#L95-L107)),
so an image of an un-upgraded install may hold them in the old places.

Also under `usr/`, cited at [`ff2a5b9`](https://github.com/agent0ai/agent-zero/commit/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a):

- Tool policies, the per-profile and per-project permission lists:
  `usr/agents/<profile>/plugins/_tool_access/config.json`,
  `usr/projects/<name>/.a0proj/plugins/_tool_access/config.json` and
  `.a0proj/agents/<profile>/plugins/_tool_access/config.json`
  ([`plugins/_tool_access/README.md:4-9`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/plugins/_tool_access/README.md#L4-L9);
  [Permission rows](#permission-rows)).
- Plugin switches: an empty `.toggle-0` (disabled) or `.toggle-1` (enabled) in a plugin's directory
  under `usr/plugins/`
  ([`helpers/plugins.py:60-61`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/helpers/plugins.py#L60-L61)).
- `plugins/_model_config/presets.yaml`, the model presets a chat can select
  ([`plugins/_model_config/helpers/model_config.py:10`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/plugins/_model_config/helpers/model_config.py#L10),[`99-101`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/plugins/_model_config/helpers/model_config.py#L99-L101)).
- `email/state.json` and `email/attachments/` of the email plugin
  ([`plugins/_email_integration/helpers/handler.py:38-39`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/plugins/_email_integration/helpers/handler.py#L38-L39)),
  `whatsapp/media/` and, under the install's `tmp/`, `whatsapp/media/` and `whatsapp/session/` of
  the WhatsApp bridge
  ([`plugins/_whatsapp_integration/helpers/handler.py:31`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/plugins/_whatsapp_integration/helpers/handler.py#L31),
  [`plugins/_whatsapp_integration/helpers/storage_paths.py:6-11`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/plugins/_whatsapp_integration/helpers/storage_paths.py#L6-L11));
  the session directory is the bridge's login state ([Credentials](#credentials)).
  `tmp/` is beside `usr/` only in a native run or a whole-`/a0` mount; the installer, the launcher
  and the README volume mount `usr` alone, so `tmp/` stays inside the container.

#### Host-side companions

These are home-relative:

- A0 CLI connector: `~/.agent-zero/.env` (instance URL, last chat id and host, remote-exec and
  computer-use flags; since the connector's
  [`3741c88`](https://github.com/agent0ai/a0-connector/commit/3741c8809f1f99b4efee2ed1a24234a98bf896d4)
  `_ENV_DIR` is at
  [`config.py:10-12`](https://github.com/agent0ai/a0-connector/blob/3741c8809f1f99b4efee2ed1a24234a98bf896d4/src/agent_zero_cli/config.py#L10-L12)) and `~/.agent-zero/session_cookies.json`
  ([`a0-connector config.py:8-10`](https://github.com/agent0ai/a0-connector/blob/76e834f20e50f6e47954c89ee39dd4c5b50d81b7/src/agent_zero_cli/config.py#L8-L10),[`15-25`](https://github.com/agent0ai/a0-connector/blob/76e834f20e50f6e47954c89ee39dd4c5b50d81b7/src/agent_zero_cli/config.py#L15-L25)),
  same path on every OS. Its managed browser profiles are under
  `~/.local/share/a0/browser-profiles`, `~/Library/Application Support/A0/Browser Profiles` or
  `%LOCALAPPDATA%\A0\Browser Profiles`
  ([`host_browser_common.py:476-487`](https://github.com/agent0ai/a0-connector/blob/76e834f20e50f6e47954c89ee39dd4c5b50d81b7/src/agent_zero_cli/host_browser_common.py#L476-L487)).
- A0 Launcher: Electron `productName` "Agent Zero Launcher"
  ([`package.json:51`](https://github.com/agent0ai/a0-launcher/blob/b0333d80b5abf38d8481f55b667b0cf23d805bbb/package.json#L51)),
  so `userData` is the Electron default directory of that name; it holds `docker_manager/state.json`
  and `docker_manager/cache`
  ([`state_store.js:13-23`](https://github.com/agent0ai/a0-launcher/blob/b0333d80b5abf38d8481f55b667b0cf23d805bbb/shell/docker_manager/state_store.js#L13-L23)).
  The launcher's Windows installer treats `%APPDATA%\a0-launcher` (the package name, not the product
  name) as the launcher's settings directory and keeps it across a reinstall, renaming it to
  `a0-launcher-installer-backup` while it runs
  ([`packaging/platforms/windows/installer.nsh:55-62`](https://github.com/agent0ai/a0-launcher/blob/a69403d9cc92009addb06fedb5585e60e09a3d2c/packaging/platforms/windows/installer.nsh#L55-L62));
  which of the two names Electron uses at run time was not resolved from source.
- A0 Launcher on Windows, since [`a69403d`](https://github.com/agent0ai/a0-launcher/commit/a69403d9cc92009addb06fedb5585e60e09a3d2c): on a host without Docker
  Desktop the launcher's setup can import its own WSL distribution named `Ubuntu`, with its own
  Docker engine, into `AppData/Local/agent-zero/wsl/Ubuntu`
  ([`shell/docker_adapter/impl/WindowsWslRuntime.mjs:23`](https://github.com/agent0ai/a0-launcher/blob/a69403d9cc92009addb06fedb5585e60e09a3d2c/shell/docker_adapter/impl/WindowsWslRuntime.mjs#L23),[`31`](https://github.com/agent0ai/a0-launcher/blob/a69403d9cc92009addb06fedb5585e60e09a3d2c/shell/docker_adapter/impl/WindowsWslRuntime.mjs#L31),[`353`](https://github.com/agent0ai/a0-launcher/blob/a69403d9cc92009addb06fedb5585e60e09a3d2c/shell/docker_adapter/impl/WindowsWslRuntime.mjs#L353)),
  whose virtual disk holds the Docker engine and the container data and is kept on uninstall
  ([`docs/runtime-troubleshooting.md:38-43`](https://github.com/agent0ai/a0-launcher/blob/a69403d9cc92009addb06fedb5585e60e09a3d2c/docs/runtime-troubleshooting.md#L38-L43)).
  A named volume of that engine is inside the virtual disk, out of reach of the collector's volume
  walk; no catalog line names the directory, and
  [Agents outside dot directories in `collectors/docs/coverage.md`](../collectors/docs/coverage.md#agents-outside-dot-directories)
  gives it as a manual step.
- A0 CLI connector, computer use: each screen capture the connector takes for the agent is written
  to `<system temp>/_a0_connector/computer_use/<ctxid>/<uuid>.png` and older captures are deleted as
  new ones arrive
  ([`computer_use.py:242-247`](https://github.com/agent0ai/a0-connector/blob/3741c8809f1f99b4efee2ed1a24234a98bf896d4/src/agent_zero_cli/computer_use.py#L242-L247),[`1329-1340`](https://github.com/agent0ai/a0-connector/blob/3741c8809f1f99b4efee2ed1a24234a98bf896d4/src/agent_zero_cli/computer_use.py#L1329-L1340),[`1348-1356`](https://github.com/agent0ai/a0-connector/blob/3741c8809f1f99b4efee2ed1a24234a98bf896d4/src/agent_zero_cli/computer_use.py#L1348-L1356)),
  overridable with `A0_COMPUTER_USE_HOST_ARTIFACT_ROOT`
  ([`33`](https://github.com/agent0ai/a0-connector/blob/3741c8809f1f99b4efee2ed1a24234a98bf896d4/src/agent_zero_cli/computer_use.py#L33)).
  The system temp directory is `/tmp` or `$TMPDIR` on Linux and macOS, outside the home, and
  `AppData/Local/Temp` on Windows, inside it; the catalog line
  `agent-zero|AppData/Local/Temp/_a0_connector` collects the Windows form, and the other two are a
  manual step in
  [Agents outside dot directories in `collectors/docs/coverage.md`](../collectors/docs/coverage.md#agents-outside-dot-directories).

### Credentials

- `usr/.env`: provider keys as `API_KEY_<PROVIDER>`, `AUTH_LOGIN`, `AUTH_PASSWORD`, `RFC_PASSWORD`,
  `ROOT_PASSWORD`; settings strip these before writing `settings.json`
  ([`settings.py:540-570`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/settings.py#L540-L570)).
- `usr/secrets.env` and `usr/projects/*/.a0proj/secrets.env`: user secrets the agent references as
  `§§secret(KEY)`
  ([`secrets.py:17-19`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/secrets.py#L17-L19),[`535-556`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/secrets.py#L535-L556)).
- `usr/settings.json` keeps `mcp_servers`, a JSON string of MCP server definitions that can carry
  headers or env keys
  ([`settings.py:93`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/settings.py#L93));
  `mcp_server_token` is blanked
  ([`settings.py:553`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/settings.py#L553)).
- Legacy: root `.env`, `tmp/secrets.env`, `tmp/settings.json`
  ([`migration.py:36-38`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/migration.py#L36-L38));
  the VPS guide still mounts a root `.env`
  ([`docs/setup/vps-deployment.md:176`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/docs/setup/vps-deployment.md#L176)).
- The desktop plugin keeps a profile with `.ssh/` and `.gnupg/` under
  `usr/plugins/_desktop/profiles/agent-zero-desktop/` (paths from a grep of
  string literals; writer not traced).
- `usr/plugins/<plugin>/config.json` holds a plugin's settings, which for the Telegram integration
  are the bot `token` and, in webhook mode, `webhook_secret`, and probably the email integration's
  credentials
  ([`helpers/plugins.py:838-851`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/plugins.py#L838-L851));
  secret globs `agent-zero*/usr/plugins/*/config.json`,
  `Desktop/agent-zero*/usr/plugins/*/config.json` and the volume-relative `plugins/*/config.json`
  (the `a0_usr` volume's base is `usr` itself) and `usr/plugins/*/config.json` from collector
  1.14.0. No other `DOCKER_VOLUMES` layout has a `plugins/<name>/config.json`. A per-project copy
  under `.a0proj/` or an agent profile's directory is not flagged. Each Telegram chat's
  `chat.json` repeats the values ([Secrets](#secrets)).
- Host side: `~/.agent-zero/session_cookies.json` (login cookies per host,
  [`config.py:123-146`](https://github.com/agent0ai/a0-connector/blob/76e834f20e50f6e47954c89ee39dd4c5b50d81b7/src/agent_zero_cli/config.py#L123-L146));
  the launcher's `state.json` stores instance passwords encrypted with Electron `safeStorage`, i.e.
  the OS keychain
  ([`state_store.js:683-688`](https://github.com/agent0ai/a0-launcher/blob/b0333d80b5abf38d8481f55b667b0cf23d805bbb/shell/docker_manager/state_store.js#L683-L688),[`753-761`](https://github.com/agent0ai/a0-launcher/blob/b0333d80b5abf38d8481f55b667b0cf23d805bbb/shell/docker_manager/state_store.js#L753-L761)).
- WhatsApp: the bridge, a Node.js process that signs in as a linked WhatsApp
  device through the Baileys library, keeps that device's login state in `tmp/whatsapp/session/`
  with Baileys' `useMultiFileAuthState`
  ([`plugins/_whatsapp_integration/helpers/storage_paths.py:6-7`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/plugins/_whatsapp_integration/helpers/storage_paths.py#L6-L7),
  [`whatsapp-bridge/bridge.js:46`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/plugins/_whatsapp_integration/whatsapp-bridge/bridge.js#L46),[`108`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/plugins/_whatsapp_integration/whatsapp-bridge/bridge.js#L108)).
  Whoever holds those files can act as the account's linked device; the directory is collected where
  `tmp/` is on the host, flagged by the secret globs `agent-zero*/tmp/whatsapp/session/*`,
  `Desktop/agent-zero*/tmp/whatsapp/session/*` and the volume-relative `tmp/whatsapp/session/*`
  (no other `DOCKER_VOLUMES` layout has a `tmp/whatsapp/`). The bridge had it at [`e3051fb`](https://github.com/agent0ai/agent-zero/commit/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec) already.
- OAuth model providers: the bundled `_oauth` plugin signs in to GitHub Copilot (a GitHub device
  flow with the scope `read:user`), Gemini and xAI Grok and keeps each provider's tokens in
  `usr/plugins/_oauth/<provider>/auth.json` (`github_copilot`, `gemini_api`, `xai_grok`), written
  with mode 0600
  ([`plugins/_oauth/helpers/providers/base.py:151-173`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/plugins/_oauth/helpers/providers/base.py#L151-L173),[`197-216`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/plugins/_oauth/helpers/providers/base.py#L197-L216);
  [`plugins/_oauth/helpers/providers/github_copilot.py:170-177`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/plugins/_oauth/helpers/providers/github_copilot.py#L170-L177),[`519-525`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/plugins/_oauth/helpers/providers/github_copilot.py#L519-L525),[`571`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/plugins/_oauth/helpers/providers/github_copilot.py#L571);
  [`plugins/_oauth/helpers/providers/gemini_api.py:59`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/plugins/_oauth/helpers/providers/gemini_api.py#L59);
  [`plugins/_oauth/helpers/providers/xai_grok.py:60`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/plugins/_oauth/helpers/providers/xai_grok.py#L60)).
  The Copilot file holds the GitHub token as `refresh` and the Copilot token as `access`. The
  files are collected with `usr/` and no secret glob flags them: `plugins/*/config.json` names
  only the configuration file.
- The connector's `.env` also holds `AGENT_ZERO_COMPUTER_USE_RESTORE_TOKEN`
  ([`config.py:22`](https://github.com/agent0ai/a0-connector/blob/3741c8809f1f99b4efee2ed1a24234a98bf896d4/src/agent_zero_cli/config.py#L22)),
  already covered by the `.agent-zero/.env` glob.

### Exclusions

`tmp/memory/embeddings` is an embedding cache
([`memory.py:145-147`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/plugins/_memory/helpers/memory.py#L145-L147)),
`tmp/playwright` holds browser binaries
([`plugins/_browser/helpers/playwright.py:16`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/plugins/_browser/helpers/playwright.py#L16)),
`usr/.time_travel` is shadow git. For native clones: `.venv`, `.conda`, `.git`. Python environments
inside `usr/workdir/*/.venv` occur (the code references one). The connector's and launcher's
Chromium directories are caches and cookie stores.

The `usr/.time_travel` exclusion drops more than file contents: every snapshot commit names the
chat that caused it, so the shadow git is a record of which chat changed which files that outlives
a chat delete ([Records of past use](#records-of-past-use)).
Its size was not measured. The launcher's WSL directory on Windows is a whole Linux disk and is not
a candidate for collection by default.

### Project-local files

Agent Zero "projects" are directories it owns under `usr/projects/<name>` (git clones are made
there,
[`projects.py:121-151`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/projects.py#L121-L151))
with metadata in `.a0proj/`; they are collected with `usr/`. Through the A0 CLI connector the
containerised agent reads and writes files and runs commands in the host's current directory
([`acp.py:314`](https://github.com/agent0ai/a0-connector/blob/76e834f20e50f6e47954c89ee39dd4c5b50d81b7/src/agent_zero_cli/acp.py#L314),
[`session.py:119`](https://github.com/agent0ai/a0-connector/blob/76e834f20e50f6e47954c89ee39dd4c5b50d81b7/src/agent_zero_cli/session.py#L119))
but writes no Agent Zero state file there.

### Where the project path is recorded

`usr/chats/<ctxid>/chat.json` key `data.project` holds the project name
([`projects.py:27`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/projects.py#L27),[`457`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/projects.py#L457));
the path is `usr/projects/<name>`.

A host repository path is recorded too. A chat opened by an editor through ACP, which the
connector's `a0 acp` command serves, stores the editor's workspace on the user's host as
`data.acp_cwd` and any extra folders as `data.acp_additional_directories`
([Editor sessions](#editor-sessions)). The `chat.json` is on the host only
where `usr/` is (installer, launcher host-directory mode, a native run) or in a collected volume,
and the path names a directory on whichever host ran the editor, which need not be the host running
Agent Zero. Project discovery in both collectors reads both keys from the chats of a home install
(not from a Docker volume), and the parser takes `acp_cwd` as the chat's project path
([Parser plan](#parser-plan)). A plain A0 CLI chat records no host path: the connector's working
directory stays on the host side
([`a0-connector session.py:131`](https://github.com/agent0ai/a0-connector/blob/3741c8809f1f99b4efee2ed1a24234a98bf896d4/src/agent_zero_cli/session.py#L131));
no key for it was found in agent-zero's `plugins/_a0_connector` at [`ff2a5b9`](https://github.com/agent0ai/agent-zero/commit/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a)
(grepped for `workspace` and `cwd`).

## Parsing

### Transcript stores

Relative to the install directory (`/a0` in Docker; on the host usually
`~/agent-zero/<instance>/` for installer and launcher setups):

- `usr/chats/<ctxid>/chat.json`, one per chat or task context; background contexts are never saved
  ([`helpers/persist_chat.py:17-19`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/persist_chat.py#L17-L19),[`50-60`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/persist_chat.py#L50-L60),[`95-96`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/persist_chat.py#L95-L96)).
  The file is rewritten whole and atomically
  ([`99-120`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/persist_chat.py#L99-L120)).
  `ctxid` is 8 random alphanumerics
  ([`agent.py:150-158`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/agent.py#L150-L158)).
- `usr/chats/<ctxid>/messages/<n>.txt`: full text of each tool result of 500 characters or more; the
  history record gets a `file` key pointing at it
  ([`extensions/python/hist_add_tool_result/_90_save_tool_call_file.py:23-43`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/extensions/python/hist_add_tool_result/_90_save_tool_call_file.py#L23-L43)).
- Legacy: `usr/chats/<ctxid>.json` (v0.8.0, moved into the folder layout on load,
  [`persist_chat.py:134-140`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/persist_chat.py#L134-L140))
  and `tmp/chats/` (moved to `usr/chats`,
  [`helpers/migration.py:24`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/migration.py#L24)).

The catalog globs are therefore `agent-zero/*/usr/chats/*/chat.json`,
`agent-zero/usr/chats/*/chat.json` and the `Desktop/agent-zero` equivalents. A Docker volume holding
`/a0/usr` (`a0_usr`, `a0-launcher-<slug>-usr`) is collected with its `_data` directory as the base,
so its chats are `chats/<ctxid>/chat.json` relative to that base
([`README.md:92`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/README.md#L92),
[`shell/docker_manager/index.js:948-951`](https://github.com/agent0ai/a0-launcher/blob/b0333d80b5abf38d8481f55b667b0cf23d805bbb/shell/docker_manager/index.js#L948-L951)).
The parser selects both forms ([Parser plan](#parser-plan)). The volume layout has no real
install behind it ([Observed](#observed)).

#### What survives a delete

Deleting a chat removes its folder, `usr/chats/<ctxid>/` with `chat.json` and `messages/`, and the
scheduler tasks bound to it, and asks the provider to delete the chat's stored responses; a reset
rewrites `chat.json` and removes `messages/`
([`api/chat_remove.py:11-26`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/api/chat_remove.py#L11-L26),
[`api/chat_reset.py:13-19`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/api/chat_reset.py#L13-L19)).
The provider call's errors are discarded, so nothing local says whether the remote copy went
([Sync, relay and origin](#sync-relay-and-origin-rows)).

After a delete the
time-travel shadow git of `usr`, the memory store, the received files
(`usr/uploads/`, `usr/email/attachments/` and the WhatsApp media
directories) and the Telegram plugin's `state.json` remain; `state.json`
still maps the Telegram user and chat to the old `ctxid` until that
user writes again, so an entry with no `chat.json` is a deleted
Telegram chat, the only deletion marker. The scheduler's `tasks.json`
outlives a reset of a task's chat but not a delete
([Other stores](#other-stores)). Whether Agent Zero
keeps a per-prompt history file is not stated.

The analyzer reads the scheduler's `tasks.json` and the Telegram
`state.json` as secondary sources, the second giving a deletion row for a
mapped chat that is gone
([Past use and secondary sources](#past-use-rows-and-secondary-sources),
[Secondary sources](../analyzer/docs/output.md#secondary-sources)); it does
not read `tasks.yaml`, the time-travel git, which the collector excludes,
or the memory store, a pickle left to manual analysis.

### Record schema

`chat.json` top level
([`persist_chat.py:179-219`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/persist_chat.py#L179-L219)):
`id`, `name`, `created_at`, `last_message` (ISO 8601 with the user's UTC offset,
[`helpers/localization.py:195-212`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/localization.py#L195-L212)),
`type` (`user` or `task`,
[`agent.py:50-53`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/agent.py#L50-L53)),
`agents`, `streaming_agent`, `agent_profile`, `log`, `data` (includes `project`), `output_data`.

#### Agent history

`agents[]`
([`persist_chat.py:222-232`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/persist_chat.py#L222-L232)):
agent 0 first, then each subordinate. Fields `number`, `agent_profile`, `data`, and `history`, which
is a **JSON string** (compact dump,
[`helpers/history.py:520-522`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/history.py#L520-L522),[`810-811`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/history.py#L810-L811))
that decodes to `{"_cls":"History","counter","bulks":[],"topics":[],"current":{}}`
([`history.py:512-518`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/history.py#L512-L518)).
A `Topic` is `{_cls, summary, messages}`
([`290-295`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/history.py#L290-L295)),
a `Bulk` is `{_cls, summary, records}` of topics or bulks
([`340-345`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/history.py#L340-L345)).
A `Message` is `{_cls:"Message", id, ai, content, metadata, sequence, summary, tokens}`
([`138-149`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/history.py#L138-L149)),
no timestamp. Chronological order is bulks, then topics, then current, messages in list order.

Message content by kind:

- User (`ai=false`): dict from `fw.user_message.md`, `{system_message?, user_message, attachments?}`
  with empty keys removed
  ([`agent.py:746-773`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/agent.py#L746-L773),
  [`prompts/fw.user_message.md`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/prompts/fw.user_message.md#L1-L7)).
- Assistant (`ai=true`): the model's text, which in the default prompt mode is a JSON object
  `{thoughts, headline, tool_name, tool_args}`
  ([`prompts/agent.system.main.communication.md:9-12`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/prompts/agent.system.main.communication.md#L9-L12));
  with native function calling it is `{"tool_name","tool_args"}` rendered from the calls, or
  `parallel_tool_calls` wrapping several
  ([`agent.py:775-795`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/agent.py#L775-L795),
  [`helpers/llm_result.py:197-209`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/llm_result.py#L197-L209)).
  `metadata.responses` carries `response_id`, `provider_model_key`, `usage`
  ([`llm_result.py:227-239`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/llm_result.py#L227-L239)).
- Tool result (`ai=false`): dict `{tool_name, tool_result, file?, ...}`
  ([`agent.py:802-825`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/agent.py#L802-L825));
  the message `id` is the tool's log item id
  ([`helpers/tool.py:56`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/tool.py#L56),[`61-68`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/tool.py#L61-L68)).

#### Log

`log`
([`persist_chat.py:235-247`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/persist_chat.py#L235-L247)):
`{guid, logs, progress, progress_no}`, `logs` being the last 1000 items
([`persist_chat.py:18`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/persist_chat.py#L18)).
Each item
([`helpers/log.py:199-209`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/log.py#L199-L209)):
`no`, `id`, `type`, `heading`, `content`, `kvps`, `timestamp` (float epoch seconds, `time.time()`,
[`log.py:157-162`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/log.py#L157-L162)),
`agentno`. `type` is one of `agent`, `browser`, `code_exe`, `subagent`, `error`, `hint`, `info`,
`progress`, `response`, `tool`, `mcp`, `input`, `user`, `util`, `warning`
([`log.py:44-60`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/log.py#L44-L60)).
Content is cut to 15,000 characters
([`log.py:66`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/log.py#L66),[`124-135`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/log.py#L124-L135)).
Keys passed as keyword arguments are merged into `kvps`
([`log.py:336-340`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/log.py#L336-L340)):

- `user` items: `content` is the message, `id` the history message id
  ([`helpers/message_queue.py:141-147`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/message_queue.py#L141-L147)).
- `agent` items: the streamed generation; `kvps` holds the parsed response (`thoughts`, `headline`,
  `tool_name`, `tool_args`), `step`, `reasoning`
  ([`extensions/python/response_stream/_10_log_from_stream.py:47-69`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/extensions/python/response_stream/_10_log_from_stream.py#L47-L69)).
- `tool` items: `kvps` is the tool arguments plus `_tool_name`; `content` is later set to the result
  ([`helpers/tool.py:56-59`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/tool.py#L56-L59),[`68`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/tool.py#L68)).
- `response` items: the final answer to the user.

#### Remote-copy keys

Keys in `data` and `agents[].data` that record a remote copy. Context and agent data are saved
without the keys that start with `_`
([`helpers/persist_chat.py:194-195`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/persist_chat.py#L194-L195),[`223`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/persist_chat.py#L223)),
so these survive:

- `agents[].data.responses_state`, written after each model reply that carries a provider response
  id
  ([`agent.py:1062-1082`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/agent.py#L1062-L1082)):
  `{response_id, previous_response_id, provider_model_key, history_counter, response_ids: [...]}`,
  all strings but `history_counter` (the history `sequence` of the reply) and `response_ids` (every
  id the chat has had for that model). The Responses API is OpenAI's generation endpoint, which can
  keep each response on the provider's side (`store: true`) so the next turn sends only
  `previous_response_id` and the new input. Agent Zero calls providers through the litellm library,
  and its transport sets `store: true` when the model's `responses_state` is `provider`, which is
  the value when none is set, and `store: false` for `local` and `off`
  ([`helpers/litellm_transport.py:889-904`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/litellm_transport.py#L889-L904),[`1554-1556`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/litellm_transport.py#L1554-L1556)).
  It applies only to a model whose `a0_api_mode` is `responses`; every stock provider in
  [`conf/model_providers.yaml`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/conf/model_providers.yaml#L176-L182)
  is `chat`, which sets the state to `off`
  ([`105-112`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/litellm_transport.py#L105-L112)),
  and the bundled Codex (ChatGPT account) provider is `responses` with `responses_state: local`
  ([`plugins/_oauth/conf/model_providers.yaml:9-10`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/plugins/_oauth/conf/model_providers.yaml#L9-L10)).
  So a key present means the provider returned response ids, not that it stored them; a non-empty
  `previous_response_id` means a turn was chained on a stored response. History compression and
  manual compaction drop `response_id` and `previous_response_id` but keep `response_ids`
  ([`helpers/history.py:736-755`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/history.py#L736-L755),
  [`extensions/python/message_loop_end/_10_organize_history.py:12`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/extensions/python/message_loop_end/_10_organize_history.py#L12),
  [`plugins/_chat_compaction/helpers/compactor.py:149`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/plugins/_chat_compaction/helpers/compactor.py#L149));
  a branched chat drops the whole key
  ([`plugins/_chat_branching/api/branch_chat.py:142-144`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/plugins/_chat_branching/api/branch_chat.py#L142-L144)).
  The same id is also in each assistant message's `metadata.responses`
  ([Agent history](#agent-history)).
- Telegram chats (the bundled `_telegram_integration` plugin, a bot the user runs with a token from
  Telegram): `data.telegram_bot` (bot name), `telegram_chat_id`, `telegram_chat_type`,
  `telegram_user_id`, `telegram_username` and `telegram_bot_cfg`, the bot's whole configuration
  entry
  ([`plugins/_telegram_integration/helpers/constants.py:5-15`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/plugins/_telegram_integration/helpers/constants.py#L5-L15),
  [`plugins/_telegram_integration/helpers/handler.py:380-400`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/plugins/_telegram_integration/helpers/handler.py#L380-L400));
  the chat is named `Telegram: @<username>` or `Telegram: <user id>`
  ([`391-392`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/plugins/_telegram_integration/helpers/handler.py#L391-L392)).
- WhatsApp chats (`_whatsapp_integration`): `data.wa_chat_id`, `wa_sender_name`, `wa_sender_number`,
  `wa_is_group`, `wa_active`, `wa_last_body`, `wa_last_msg_id`
  ([`plugins/_whatsapp_integration/helpers/handler.py:33-39`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/plugins/_whatsapp_integration/helpers/handler.py#L33-L39),[`153-159`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/plugins/_whatsapp_integration/helpers/handler.py#L153-L159)).
- Email chats (`_email_integration`, IMAP in and SMTP out): `data.email_handler`, `email_sender`,
  `email_thread_id`, `email_subject`, `email_message_id`, `email_references`, `email_last_body`
  ([`plugins/_email_integration/helpers/dispatcher.py:14-21`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/plugins/_email_integration/helpers/dispatcher.py#L14-L21),[`269-270`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/plugins/_email_integration/helpers/handler.py#L269-L270)).

#### Editor sessions

Read by the parser ([Parser plan](#parser-plan)). ACP lets an editor drive an agent; the bundled
`_a0_acp` plugin serves it and the host-side connector (`a0 acp`) relays the editor's files and
terminal. A chat an editor
opens gets `data.acp_session` (`true`), `acp_cwd` (the editor's workspace, a path on the user's
host), `acp_additional_directories` (a list of paths), `acp_mode` (`default`, `plan` or `act`,
anything else stored as `default`), `acp_transport` (`connector` or `container`), `acp_model_id`,
`acp_config_options`, and, for the `container` transport with a configured root, `workdir_path` (a
working directory inside the container); the chat is named after the last component of `acp_cwd`
([`plugins/_a0_acp/api/session.py:12-20`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/plugins/_a0_acp/api/session.py#L12-L20),[`41-43`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/plugins/_a0_acp/api/session.py#L41-L43),[`131-150`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/plugins/_a0_acp/api/session.py#L131-L150)).
The keys are written at [`e3051fb`](https://github.com/agent0ai/agent-zero/commit/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec) as well.

#### Model override

Read by the parser ([Model rows](#model-rows)). Choosing a model for one chat stores
`data.chat_model_override`: `{"preset_name": <name>}` for a preset of
`usr/plugins/_model_config/presets.yaml`, a whole model configuration object for a custom choice, or
`null` when cleared
([`plugins/_model_config/api/model_override.py:58`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/plugins/_model_config/api/model_override.py#L58),[`81`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/plugins/_model_config/api/model_override.py#L81),[`92`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/plugins/_model_config/api/model_override.py#L92),
[`plugins/_model_config/helpers/model_config.py:10`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/plugins/_model_config/helpers/model_config.py#L10),[`744-756`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/plugins/_model_config/helpers/model_config.py#L744-L756));
the connector's model switcher writes the same key as `{"chat": {...}, "utility": {...},
"embedding": {...}}`, each slot `{provider, name, api_key?, api_base?}`
([`plugins/_a0_connector/api/v1/model_switcher.py:27-46`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/plugins/_a0_connector/api/v1/model_switcher.py#L27-L46),[`200-211`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/plugins/_a0_connector/api/v1/model_switcher.py#L200-L211)),
so a custom choice can carry an API key ([Secrets](#secrets)).
Only the latest choice is kept, so the history of switches is the per-message `provider_model_key`.

### Joins and timestamps

- Chat to project: `data.project` is a project name; path `/a0/usr/projects/<name>`
  ([`helpers/projects.py:457`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/projects.py#L457)).
  Without a project the working directory is `usr/workdir`.
- Log to history: `log.logs[].id` equals the history `Message.id` for user
  messages and tool results ([Record schema](#record-schema)).
- Tool use to result: one `tool` log item holds both; in history the
  assistant message's `tool_name` and the next tool-result message.
- Parent to subordinate: `agents[n]` is the subordinate of `agents[n-1]`
  ([`persist_chat.py:189-194`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/persist_chat.py#L189-L194));
  log items carry `agentno`.
- Times: log items carry float epoch seconds, history messages none, and `created_at` and
  `last_message` are ISO 8601 with the user's UTC offset ([Log](#log),
  [Agent history](#agent-history)).

### Other stores

Beside `chat.json`, the transcript store, Agent Zero keeps the stores below. Memory is a FAISS
index plus a pickled LangChain docstore (`usr/memory/<subdir>/index.pkl`); reading it means
unpickling untrusted data, so leave it to manual analysis.

#### Records of past use

Cited at [`ff2a5b9`](https://github.com/agent0ai/agent-zero/commit/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a).
The parser reads the scheduler's `tasks.json` and the Telegram `state.json`; the plan marks each
source in [Past use and secondary sources](#past-use-rows-and-secondary-sources). Paths are relative
to the install directory.

| File | Holds | Survives | Joins `chat.json` on | Kind |
| --- | --- | --- | --- | --- |
| `usr/scheduler/tasks.json`, or `tasks.yaml` when only that exists | One object per scheduled, ad hoc or planned task: `uuid`, `context_id`, `name`, `system_prompt`, `prompt`, `attachments`, `project_name`, `created_at`, `updated_at`, `last_run`, `last_result`, `state`, `type` ([`helpers/task_scheduler.py:163-175`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/helpers/task_scheduler.py#L163-L175),[`512-515`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/helpers/task_scheduler.py#L512-L515)) | A reset of the task's chat; not a chat delete, which removes the tasks bound to the chat ([`api/chat_remove.py:24-26`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/api/chat_remove.py#L24-L26)) | `context_id` = `id`; `prompt` repeats the task chat's user message, `last_result` its last answer ([Scheduler tasks](#scheduler-tasks)) | secondary, parsed (`tasks.json` only) |
| `usr/plugins/_telegram_integration/state.json` | `chats`, `"<bot>:<user id>:<chat id>"` to `ctxid` ([Telegram chat map](#telegram-chat-map)) | A chat delete, until that Telegram user writes again | the `ctxid` value = `id` | secondary, parsed (a deletion marker: an entry with no `chat.json`) |
| `usr/.time_travel/workspaces/` | A shadow git repository per workspace of `usr`, snapshotted automatically; each commit message ends with `A0-Time-Travel-Metadata:` and base64 JSON with `context_id`, `project_name`, `trigger`, `timestamp` and `changed_path_hints` ([`plugins/_time_travel/helpers/time_travel.py:22-28`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/plugins/_time_travel/helpers/time_travel.py#L22-L28),[`1192-1194`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/plugins/_time_travel/helpers/time_travel.py#L1192-L1194),[`1206-1220`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/plugins/_time_travel/helpers/time_travel.py#L1206-L1220)) | A chat delete and the deletion of the files it wrote | `context_id` = `id`, and time | secondary; excluded by the collector (`usr/.time_travel`), so not in a collection |
| `usr/memory/<subdir>/index.pkl` | Memory fragments and solutions the agent summarised at the end of each monologue, with `id`, `timestamp` and `area` and no chat id ([`plugins/_memory/helpers/memory.py:475-487`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/plugins/_memory/helpers/memory.py#L475-L487)) | A chat delete or reset | time only | secondary; a pickle, left to manual analysis |
| `usr/email/state.json` | The IMAP message ids already processed per handler ([`plugins/_email_integration/helpers/handler.py:38-39`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/plugins/_email_integration/helpers/handler.py#L38-L39)) | A chat delete | nothing | no events |
| `usr/uploads/`, `usr/email/attachments/`, `usr/whatsapp/media/` and `tmp/whatsapp/media/` | Files received through the web UI, Telegram, email and WhatsApp ([`plugins/_telegram_integration/helpers/constants.py:2`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/plugins/_telegram_integration/helpers/constants.py#L2), [`plugins/_whatsapp_integration/helpers/handler.py:31`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/plugins/_whatsapp_integration/helpers/handler.py#L31), [`plugins/_whatsapp_integration/helpers/storage_paths.py:10-11`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/plugins/_whatsapp_integration/helpers/storage_paths.py#L10-L11)) | A chat delete | file names in `user` log items' `attachments` | evidence files, no events |

The WhatsApp bridge keeps no message store: its only file writes are media
([`plugins/_whatsapp_integration/whatsapp-bridge/bridge.js:311`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/plugins/_whatsapp_integration/whatsapp-bridge/bridge.js#L311),[`323`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/plugins/_whatsapp_integration/whatsapp-bridge/bridge.js#L323)),
and its session directory `tmp/whatsapp/session/` is the linked device's login state
([Credentials](#credentials)).

#### Telegram chat map

Telegram also keeps `usr/plugins/_telegram_integration/state.json`,
`{"chats": {"<bot>:<user id>:<chat id>": "<ctxid>"}}`, written when a Telegram user's first message
creates a chat
([`plugins/_telegram_integration/helpers/constants.py:3`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/plugins/_telegram_integration/helpers/constants.py#L3),
[`plugins/_telegram_integration/helpers/handler.py:57-64`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/plugins/_telegram_integration/helpers/handler.py#L57-L64),[`405-410`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/plugins/_telegram_integration/helpers/handler.py#L405-L410)).
The bot name is the part before the last two colons.

#### Scheduler tasks

`usr/scheduler/tasks.json` is the scheduler's task list as pydantic writes it, `{"tasks": [...]}`,
each task an object with the keys of the [Records of past use](#records-of-past-use) table,
datetimes as ISO 8601 strings, and `type` one of `adhoc`, `scheduled` and `planned`; `state` is
`idle`, `running`, `disabled` or `error`
([`helpers/task_scheduler.py:78-88`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/helpers/task_scheduler.py#L78-L88),[`512-519`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/helpers/task_scheduler.py#L512-L519),[`584-592`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/helpers/task_scheduler.py#L584-L592)).
A scheduled task also holds `schedule` (cron fields), a planned task `plan` (`todo`,
`in_progress`, `done`), and an ad hoc task `token`
([`332`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/helpers/task_scheduler.py#L332),[`422`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/helpers/task_scheduler.py#L422),
[`282-284`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/helpers/task_scheduler.py#L282-L284)).
A run logs the chat's user message as `## Task:\n<prompt>`, or
`## Context:\n<context>\n\n## Task:\n<prompt>` when the task has context
([`945-959`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/helpers/task_scheduler.py#L945-L959)),
and a finished run sets `last_run` and `last_result`, the monologue's answer or `ERROR: <error>`
with `state` `error`
([`249-275`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/helpers/task_scheduler.py#L249-L275)).

### Format versions

No version field. History compression summarises topics into bulks and drops the oldest bulk when
merging fails
([`history.py:600-607`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/history.py#L600-L607)),
so early turns may survive only as summaries in history and only in the log while it is under 1000
items. Older chats used `chat.json`-less `<ctxid>.json` files
([Transcript stores](#transcript-stores)).

Between [`e3051fb`](https://github.com/agent0ai/agent-zero/commit/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec) and [`ff2a5b9`](https://github.com/agent0ai/agent-zero/commit/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a) the drift is small: the `usr/` layout is unchanged, the `chat.json` writer is
unchanged and six lines lower in `helpers/persist_chat.py` (`_serialize_context` at
[`185-253`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/helpers/persist_chat.py#L185-L253)),
`metadata.responses` gains an optional `history_extras` string
([`helpers/llm_result.py:249`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/helpers/llm_result.py#L249)),
and the scheduler may keep `tasks.yaml` instead of `tasks.json`
([Per-user storage](#per-user-storage)). The log, history and tool-result files the parser reads are
the same.

### Secrets

Values from `secrets.env` are masked in history content and log items before storage
([`extensions/python/hist_add_before/_10_mask_content.py:20-28`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/extensions/python/hist_add_before/_10_mask_content.py#L20-L28),
[`log.py:420-435`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/log.py#L420-L435)).
API keys in `usr/.env` are not in that set, so any key the user pastes or a tool prints stays in
clear text. `user_message` and tool `kvps` are the places to look.

A Telegram chat's `data.telegram_bot_cfg` is the bot's configuration entry as the bot loop holds it,
so it carries the bot `token` and, in webhook mode, `webhook_secret`, in clear text in `chat.json`
([`plugins/_telegram_integration/extensions/python/job_loop/_10_telegram_bot.py:57-59`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/plugins/_telegram_integration/extensions/python/job_loop/_10_telegram_bot.py#L57-L59),[`82-89`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/plugins/_telegram_integration/extensions/python/job_loop/_10_telegram_bot.py#L82-L89),
[`plugins/_telegram_integration/default_config.yaml:2-9`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/plugins/_telegram_integration/default_config.yaml#L2-L9)).
The transcript cannot be flagged, so `token` and `webhook_secret` are the keys to redact before
sharing a `chat.json`. The same values are in the plugin's configuration file
([Credentials](#credentials)).

A chat's `data.chat_model_override` can hold an `api_key` in each slot of a custom choice made from
the connector ([Model override](#model-override)), in clear text in `chat.json`; the parser prints
only each slot's provider and name. An ad hoc task's `token` in `usr/scheduler/tasks.json` is a
random 19-digit number the task is created with, whose use was not traced
([Scheduler tasks](#scheduler-tasks)); the parser does not read it.

The WhatsApp bridge's login state is a credential directory, not a field of a store
([Credentials](#credentials)).

### Parser plan

Rows from `log.logs` (timestamped); when `len(log.logs) == 1000`, also emit
history messages that have no matching log id, with an empty timestamp and
a `system` row saying the log was trimmed.

Paths: `chat.json`, its `messages/<n>.txt` and the legacy `<ctxid>.json`
under `<a0>/usr/chats` or `<a0>/tmp/chats` of a home install, and under
`chats/` at the base of a Docker volume mounted at `/a0/usr`
([issue 175](https://github.com/seanthegeek/doubleagent/issues/175),
[Transcript stores](#transcript-stores)); the rows are the same in both forms.
Beside them, in the same two forms: `usr/scheduler/tasks.json` (or the
legacy `tmp/scheduler/tasks.json`), `usr/plugins/_telegram_integration/state.json`,
each `_tool_access/config.json` and `_orchestrator/config.json`, the
`.toggle-0` and `.toggle-1` files of `_infection_check` and `_orchestrator`
at every plugin scope ([Per-user storage](#per-user-storage)), and
`~/.agent-zero/.env` of the connector.

| Column | Source |
| --- | --- |
| timestamp_utc | log `timestamp` (epoch s) |
| session_id | `chat.json` `id` |
| project_path | `data.acp_cwd` for an editor session, else `/a0/usr/projects/<data.project>`, else empty |
| git_branch | empty |
| turn_type | `user` → user; `response` → assistant; `agent` → assistant (`kvps.headline`/`thoughts`), and thinking for `kvps.reasoning`; `tool`, `mcp`, `code_exe`, `browser`, `subagent` → tool_use then tool_result from `content`; others → system |
| model | `metadata.responses.provider_model_key` of the matching assistant message, else empty |
| tool_name | `kvps._tool_name` |
| tool_use_id | log item `id` |
| text | `content`; for tool_use the `code`/`command`/`url` key of `kvps`, else `kvps` as JSON |
| source_line | log item `no` |

The subsections below give one paragraph per class, from source at [`ff2a5b9`](https://github.com/agent0ai/agent-zero/commit/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a); every row
they describe is written by the parser.

#### Permission rows

Every row below is source-derived; not observed on any real install.

Per call. A tool call the profile's policy blocks raises
`Tool "<name>" is blocked for agent profile "<profile>".`, which the agent records as a history
warning and a `warning` log item with the same id
([`helpers/tool_policy.py:197-208`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/helpers/tool_policy.py#L197-L208),
[`extensions/python/_functions/agent/Agent/handle_exception/end/_50_handle_repairable_exception.py:17-22`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/extensions/python/_functions/agent/Agent/handle_exception/end/_50_handle_repairable_exception.py#L17-L22)):
`permission decision: blocked tool=<name> profile=<profile>`. A provider-hosted MCP approval request
on the Responses API is always denied, logged as a `warning` headed
`Responses MCP approval required` with a tool result `mcp_approval_request`
([`agent.py:1325-1346`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/agent.py#L1325-L1346)):
`permission decision: denied automatically (provider MCP approval request)`. The Infection Check
plugin, a reviewer model that audits the agent's output before each tool runs, is disabled by
default (the bundled `plugins/_infection_check/.toggle-0`; a plugin is switched by a `.toggle-0` or
`.toggle-1` file,
[`helpers/plugins.py:60-61`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/helpers/plugins.py#L60-L61),[`505-510`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/helpers/plugins.py#L505-L510));
when enabled, a pass leaves only a progress line, and a question or a stop leaves `warning` items
headed `Infection check: requesting clarification`, `Infection check: clarification passed` or
`Infection check: TERMINATED`, the last also replacing the agent's last history message with a
blocked marker
([`plugins/_infection_check/helpers/checker.py:180-197`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/plugins/_infection_check/helpers/checker.py#L180-L197),[`330-345`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/plugins/_infection_check/helpers/checker.py#L330-L345)):
`permission decision: <clarify, passed or terminated> (infection check)`. Each decision row follows
the `system` row `warning: <heading> | <content>` of the log item it reads, at the item's time;
the blocked row carries the tool's name in `tool_name` and the MCP row `mcp_approval_request`. No
log field joins a decision to the call it governed, so `tool_use_id` stays empty.

Standing. Tool policies are `mode` (`inherit` or `custom`), `default` and `mcp_default` (`allow` or
`block`), `allowed` and `blocked` lists of tool ids
([`plugins/_tool_access/default_config.yaml:1-5`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/plugins/_tool_access/default_config.yaml#L1-L5),
[`helpers/tool_policy.py:27-37`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/helpers/tool_policy.py#L27-L37),[`176-195`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/helpers/tool_policy.py#L176-L195)),
in `usr/agents/<profile>/plugins/_tool_access/config.json`, a project's
`.a0proj/plugins/_tool_access/config.json` and
`.a0proj/agents/<profile>/plugins/_tool_access/config.json`
([`plugins/_tool_access/README.md:4-9`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/plugins/_tool_access/README.md#L4-L9)):
one
`permission setting: tool_access.<key>=<value>[ profile=<profile>][ project=<name>] (from <file>, file mtime)`
row per scalar key and list entry, the profile and project taken from the file's
path, with an empty `session_id` and, for a project's file, `project_path` the project. The
Orchestrator plugin, which tells the agent how to run other coding CLIs on the user's host or in
the container, defaults each to its approval bypass (`bypass_sandbox`,
`permission_mode: bypassPermissions`, `force`, `always_approve`, `yolo`, `auto`,
[`plugins/_orchestrator/default_config.yaml:7-37`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/plugins/_orchestrator/default_config.yaml#L7-L37));
a user override in `usr/plugins/_orchestrator/config.json` gives one
`permission setting: orchestrator.<cli>.<key>=<value> (from <file>, file mtime)` row for each of
those seven keys it holds, and nothing else of the file is read. A `.toggle-1` or `.toggle-0` of
`_infection_check` or `_orchestrator` at any plugin scope gives
`permission setting: <infection_check|orchestrator>=<enabled|disabled>[ profile=<profile>][ project=<name>] (from <file>, file mtime)`.
An editor session's `data.acp_mode` gives `permission mode: <acp_mode> (acp, as of file mtime)`
after the chat start row, at the file's modification time, since only the latest mode is kept.

#### Model rows

Before an `agent` log item, or a history-only
assistant message when the log was trimmed, whose
`metadata.responses.provider_model_key` is new for that agent, a `system`
row `model: <key>` at the agent's first model and `model: <old> -> <new>`
on a change, tracked per chat `id` and agent `number` (`agentno` on log
items), with ` agent=<n>` after it for a subordinate agent. These `model:` rows from
`provider_model_key` are already written.

After the chat start row, at the file's modification time, source-derived; not observed on any
real install:
`model: requested <acp_model_id> (acp, as of file mtime)` from
`data.acp_model_id`, and `model: requested <choice> (chat override, as of
file mtime)` from `data.chat_model_override`, where `<choice>` is
`preset <preset_name>`, or `<slot>=<provider>/<name>` for each slot of a
custom choice (`chat`, `utility`, `embedding`), or the top-level
`<provider>/<name>`, or `custom configuration`; the object itself is never
printed, since it can hold an `api_key`
([Model override](#model-override), [Editor sessions](#editor-sessions)).
A cleared override (`null`) gives no row.

#### Past-use rows and secondary sources

For the sources table in [Records of past use](#records-of-past-use),
source-derived; not observed on any real install. `usr/scheduler/tasks.json` is secondary: per
task, with
`session_id` the `context_id` and `project_path`
`/a0/usr/projects/<project_name>` when set, a `system` row
`scheduled task: <name> type=<type> state=<state>` at `created_at`, a
`user` row `## Task:\n<prompt>` with no timestamp (the text a run logs as
the chat's user message when the task has no context,
[Scheduler tasks](#scheduler-tasks)), and the `last_result` at `last_run`
as an `assistant` row, or as a `system` row
`scheduled task failed: <error>` when the state is `error` and the result
starts `ERROR: `. `last_run` is stamped when the run ends, after the
chat's `response` log item, so the file's matching window is 60 seconds.
`token` and `system_prompt` are not read, and `tasks.yaml` is not read
(YAML, which the analyzer has no reader for). `state.json` of the Telegram
plugin is secondary: one
`session deleted: telegram bot <bot> chat <chat id> user <user id> ctxid <ctxid> (from state.json, no chat.json in the input, file mtime)`
row, with `session_id` the ctxid, for each map entry whose
`chats/<ctxid>/chat.json` or `chats/<ctxid>.json` is not beside it in the
input. The time-travel git and the memory pickle are not parsed.

#### Sync, relay and origin rows

Sync: Agent Zero records no sync state. It has no account, cloud copy, share link or history sync:
the web UI is served by the instance itself, and the features that move chats are local. Backup
builds a zip of `usr/` and sends it to the browser as a download
([`helpers/backup.py:338-432`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/backup.py#L338-L432),
[`api/backup_create.py:35-51`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/api/backup_create.py#L35-L51));
chat export returns the `chat.json` text to the browser
([`api/chat_export.py:5-17`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/api/chat_export.py#L5-L17),
[`helpers/persist_chat.py:155-159`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/persist_chat.py#L155-L159));
the Remote Control tunnels (Serveo, Cloudflare, Microsoft, Tailscale) expose the web UI and persist
nothing
([`helpers/tunnel_manager.py:15-31`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/tunnel_manager.py#L15-L31));
the only call to the vendor's own service is the update check, which posts the version and a hashed
install id, no chat content, and is on by default (`update_check_enabled` in `usr/settings.json`,
[`helpers/update_check.py:5-19`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/update_check.py#L5-L19),
[`helpers/settings.py:627`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/settings.py#L627),
[`extensions/python/user_message_ui/_10_update_check.py:66`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/extensions/python/user_message_ui/_10_update_check.py#L66));
and the host-side A0 CLI connector talks to the user's instance and to GitHub for its own updates
([`a0-connector self_update.py:20-25`](https://github.com/agent0ai/a0-connector/blob/76e834f20e50f6e47954c89ee39dd4c5b50d81b7/src/agent_zero_cli/self_update.py#L20-L25)).
Searched `sync`, `share`, `upload`, `cloud`, `publish`, `remote_id`, `telemetry`, `analytics`,
`agent-zero.ai` and every `https://` host across both repositories. Two features leave a copy of a
chat on a third party's server, and `chat.json` records each
([Remote-copy keys](#remote-copy-keys)); both are implemented and unchanged at the 2026-10-19 HEAD:

- Stored responses. With a model set to `a0_api_mode: responses` and no `responses_state` (or
  `provider`), each reply is stored at the provider; `agents[].data.responses_state` and
  `metadata.responses.response_id` hold the ids. The parser yields one row per agent whose
  `previous_response_id` is non-empty:
  `session synced: provider response <response_id or -> previous <previous_response_id> model <provider_model_key or -> (<n> response ids) agent=<number> (from chat.json agents[].data.responses_state, file mtime)`,
  timestamped by the file's modification time, and sets `records_sync`. Because `local` mode records
  the same ids with `store: false`, an agent whose `previous_response_id` is empty yields no row.
  Fixture only: no real install has shown the key. On chat delete, Agent Zero collects every
  response id in `chat.json` and calls litellm's `delete_responses` for each unless
  `responses_delete_on_chat_delete` is `false` in the model's kwargs or the chat data, then deletes
  the folder
  ([`helpers/persist_chat.py:162-168`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/persist_chat.py#L162-L168),[`390-450`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/persist_chat.py#L390-L450),
  [`agent.py:941-942`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/agent.py#L941-L942));
  the call passes no provider or key and its error list is discarded
  ([`helpers/litellm_transport.py:1481-1490`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/helpers/litellm_transport.py#L1481-L1490)),
  so nothing local says whether the remote copy went.
- Messaging channels. A Telegram, WhatsApp or email chat is carried by that service, which keeps the
  messages; no Discord, Slack or other channel exists at [`ff2a5b9`](https://github.com/agent0ai/agent-zero/commit/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a) (grepped for `discord`,
  `slack`, `mattermost` and `signal-cli`: only an unrelated office helper matches).
  The parser yields one row per chat and channel with the keys:
  `session relayed: telegram bot <telegram_bot> chat <telegram_chat_id> user <telegram_user_id> @<telegram_username or -> (from chat.json data, file mtime)`,
  `session relayed: whatsapp chat <wa_chat_id> number <wa_sender_number or -> (from chat.json data, file mtime)`
  or
  `session relayed: email thread <email_thread_id or -> from <email_sender or -> (from chat.json data, file mtime)`,
  timestamped by the file's modification time, and never prints `telegram_bot_cfg` (it holds the bot
  token, [Secrets](#secrets)). Fixture only. Deleting the chat removes the folder, but
  Telegram's `state.json` still maps the user and chat to the old `ctxid` until that user writes
  again
  ([`plugins/_telegram_integration/helpers/handler.py:376-387`](https://github.com/agent0ai/agent-zero/blob/e3051fb584b1a36be2b0a0c90606f1c2c2d356ec/plugins/_telegram_integration/helpers/handler.py#L376-L387)),
  so a `state.json` entry with no `chat.json` is a deleted Telegram chat.

Origin: none on the instance, but the connector's `~/.agent-zero/.env` names the last chat it
used on a remote instance (`AGENT_ZERO_LAST_CONTEXT_ID` and `AGENT_ZERO_LAST_CONTEXT_HOST`,
[`a0-connector config.py:13-14`](https://github.com/agent0ai/a0-connector/blob/3741c8809f1f99b4efee2ed1a24234a98bf896d4/src/agent_zero_cli/config.py#L13-L14),
written together by
[`347-356`](https://github.com/agent0ai/a0-connector/blob/3741c8809f1f99b4efee2ed1a24234a98bf896d4/src/agent_zero_cli/config.py#L347-L356)). When both are set the parser
yields
`session origin: remote agent-zero instance <AGENT_ZERO_LAST_CONTEXT_HOST> context <AGENT_ZERO_LAST_CONTEXT_ID> (from .agent-zero/.env, file mtime)`
with `session_id` the context id, at the file's modification time, and sets `records_origin`;
source-derived; not observed on any real install. It reads only those two keys of a file the
collector flags as a secret, strips quotes as the connector's own reader does
([`51-66`](https://github.com/agent0ai/a0-connector/blob/3741c8809f1f99b4efee2ed1a24234a98bf896d4/src/agent_zero_cli/config.py#L51-L66)), and puts no other line in a row.

#### Scratch directory rows

A chat with no project runs in `/a0/usr/workdir` (the default `workdir_path`,
[`helpers/settings.py:604`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/helpers/settings.py#L604)),
and the analyzer's catalog rule cannot class a container path. An editor session that sets
`data.workdir_path` gives
`scratch directory: <workdir_path> (acp container transport, from chat.json data.workdir_path, as of file mtime)`
after the chat start row, at the file's modification time, with `project_path` the directory;
source-derived; not observed on any real install.
An editor session's `data.acp_cwd`, the user's project on the host, is the `project_path` of its
other rows. A chat with no project and no editor session gets no scratch row: the working
directory it ran in is the instance's `workdir_path` setting, not a key of the chat. The host
connector's computer-use screenshot directory is described in
[Host-side companions](#host-side-companions).

#### Deletion rows

A chat delete also removes the chat's scheduler tasks;
what survives it is Telegram's `state.json`, the time-travel git, the
memory store and the received files ([What survives a delete](#what-survives-a-delete)).
Nothing local says whether the provider deleted its stored responses
([Sync, relay and origin](#sync-relay-and-origin-rows)). The deletion row is the Telegram map's
`session deleted:` row under
[Past use and secondary sources](#past-use-rows-and-secondary-sources), source-derived; not observed on any real install.

#### Bad-line rows

No file is read line by line. A `chat.json` that does not parse yields a whole-file row plus the
log items the parser can recover. `tasks.json` is a JSON array reader: a task that is not an object
gives `parser: unparseable record <N> (not a JSON object): <record>` and the tasks around it are
still read, and a file that is not JSON or has no `tasks` list gives one
`parser: scheduler tasks file ...` row. `state.json` and the policy files give one whole-file
`parser:` row when they are not JSON objects. The connector's `.env` gives no bad-line rows: it is
a secret file, and the connector's own reader skips a line without `=` too. Any of these files that
cannot be read is one row `parser: <file> could not be read (<reason>)`, `<file>` its name, a
`chat.json` row carrying the chat directory's name as its session id. A message file under
`messages/` that exists but cannot be read gives the same row, naming the message file, at the
`chat.json` that names it, after the chat's other rows, and its tool result falls back to the
history or log text.

#### Fixtures

Fixture `usr/chats/AbCd1234/chat.json`:

```json
{"id":"AbCd1234","name":"List files","created_at":"2026-10-01T14:00:00+02:00","type":"user",
 "last_message":"2026-10-01T14:00:05+02:00","streaming_agent":0,"agent_profile":"agent0",
 "data":{"project":"demo"},"output_data":{},
 "agents":[{"number":0,"agent_profile":"agent0","data":{},
   "history":"{\"_cls\":\"History\",\"counter\":3,\"bulks\":[],\"topics\":[],\"current\":{\"_cls\":\"Topic\",\"summary\":\"\",\"messages\":[{\"_cls\":\"Message\",\"id\":\"u1\",\"ai\":false,\"content\":{\"user_message\":\"list files\"},\"metadata\":{},\"sequence\":0,\"summary\":\"\",\"tokens\":5},{\"_cls\":\"Message\",\"id\":\"a1\",\"ai\":true,\"content\":\"{\\\"thoughts\\\":[\\\"list\\\"],\\\"headline\\\":\\\"Listing\\\",\\\"tool_name\\\":\\\"code_execution_tool\\\",\\\"tool_args\\\":{\\\"runtime\\\":\\\"terminal\\\",\\\"code\\\":\\\"ls\\\"}}\",\"metadata\":{},\"sequence\":0,\"summary\":\"\",\"tokens\":30},{\"_cls\":\"Message\",\"id\":\"t1\",\"ai\":false,\"content\":{\"tool_name\":\"code_execution_tool\",\"tool_result\":\"a.txt\"},\"metadata\":{},\"sequence\":0,\"summary\":\"\",\"tokens\":8}]}}"}],
 "log":{"guid":"6b0f0c1e-0000-4000-8000-000000000001","progress":"","progress_no":3,"logs":[
  {"no":0,"id":"u1","type":"user","heading":"","content":"list files","kvps":{"attachments":[]},"timestamp":1759320001.0,"agentno":0},
  {"no":1,"id":"g1","type":"agent","heading":"A0: Listing","content":"","kvps":{"step":"Writing terminal command... (2)","thoughts":["list"],"headline":"Listing","tool_name":"code_execution_tool","tool_args":{"runtime":"terminal","code":"ls"}},"timestamp":1759320002.0,"agentno":0},
  {"no":2,"id":"t1","type":"tool","heading":"A0: Using tool 'code_execution_tool'","content":"a.txt","kvps":{"runtime":"terminal","code":"ls","_tool_name":"code_execution_tool"},"timestamp":1759320003.0,"agentno":0}]}}
```

Fixture `usr/scheduler/tasks.json` (one task shown; the test fixture also holds an ad hoc task in
state `error` with `last_result` `ERROR: connection refused` and a `token`):

```json
{"tasks":[{"uuid":"TaskUuid01","context_id":"AbCd1234","state":"idle","name":"Nightly listing",
 "system_prompt":"You run on a schedule.","prompt":"list files","attachments":[],
 "project_name":"demo","created_at":"2026-10-01T11:00:00Z","updated_at":"2026-10-01T12:00:05.500000Z",
 "last_run":"2026-10-01T12:00:05.500000Z","last_result":"There is one file, a.txt.",
 "type":"scheduled","schedule":{"minute":"0","hour":"2","day":"*","month":"*","weekday":"*"}}]}
```

Fixture `usr/plugins/_telegram_integration/state.json`, whose second entry has no chat:

```json
{"chats":{"opsbot:123456789:987654321":"TgChat01","opsbot:555:777":"GoneTg01"}}
```

Fixture `usr/agents/developer/plugins/_tool_access/config.json`:

```json
{"mode":"custom","default":"allow","mcp_default":"block","allowed":[],
 "blocked":["code_execution_tool","mcp:github.create_issue"]}
```

Fixture `data` of an editor-session chat, and its `warning` log items' headings: the blocked-tool
message in a formatted error, `Responses MCP approval required`,
`Infection check: clarification passed` and `Infection check: TERMINATED`:

```json
{"project":"demo","acp_session":true,"acp_cwd":"/home/alice/src/webapp",
 "acp_additional_directories":["/home/alice/src/shared"],"acp_mode":"act","acp_transport":"container",
 "acp_model_id":"openai/gpt-5","acp_config_options":{},"workdir_path":"/a0/usr/workdir/acp",
 "chat_model_override":{"chat":{"provider":"openrouter","name":"anthropic/claude-sonnet-4","api_key":"<redacted>"},
  "utility":{"provider":"ollama","name":"qwen3"}}}
```

Fixture `~/.agent-zero/.env`, with keys the parser must not read:

```text
AGENT_ZERO_HOST=http://localhost:50080
AGENT_ZERO_COMPUTER_USE_RESTORE_TOKEN=<redacted>
AGENT_ZERO_LAST_CONTEXT_HOST=https://a0.example.net
AGENT_ZERO_LAST_CONTEXT_ID='RemCtx01'
```

#### Source host rows

The agent records no source-host integration. Grepped `github`, `gitlab`, `bitbucket`, `oauth`,
`installation`, `repositor`, `pull request`, `clone` and `token` across agent-zero at
[`ff2a5b9`](https://github.com/agent0ai/agent-zero/commit/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a)
and a0-connector at
[`3741c88`](https://github.com/agent0ai/a0-connector/commit/3741c8809f1f99b4efee2ed1a24234a98bf896d4).
Cloning a repository as a project takes a token for that one clone, passes it as an HTTP header and
never stores it (`git_token` is a transient key), and the project's `.a0proj/project.json` keeps
the clone URL with any credentials stripped
([`helpers/projects.py:74`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/helpers/projects.py#L74),[`121-151`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/helpers/projects.py#L121-L151);
[`helpers/git.py:433-454`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/helpers/git.py#L433-L454)); the plugin installer's `git_token` is
used the same way
([`plugins/_plugin_installer/api/plugin_install.py:47-53`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/plugins/_plugin_installer/api/plugin_install.py#L47-L53)).
The GitHub Copilot sign-in of the `_oauth` plugin stores a GitHub token, but only to reach Copilot
models, with the scope `read:user`, and records no login ([Credentials](#credentials)). No list of
repositories and no repository-bound remote conversation is kept. A clone URL is the project's own
git remote, not an integration, so the parser writes no `source host:` row.

## Confidence

- High (source): every path under the install directory and its `usr/` layout (constants), the
  installer and launcher defaults, the connector files.
- High (source): the `chat.json` file layout, top-level keys, history and log item shapes,
  timestamp units and masking.
- Medium: the Electron `userData` paths for the launcher (Electron default from `productName`, no
  `setPath` found; [Host-side companions](#host-side-companions)).
- Medium: `Desktop/agent-zero`, which is only the docs example.
- Medium: the exact `type` and `heading` of `code_execution_tool` items (they come from a tool
  subclass not traced; the generic `tool` path is shown).
- Not determined: where native installs are cloned in practice; the catalog cannot find a clone
  outside the two listed paths.
- Not determined: the desktop plugin's credential files beyond the literal paths found.
- Medium (source, no real install): the permission, setting, requested-model, scratch, scheduler,
  Telegram deletion and connector origin rows ([Parser plan](#parser-plan)); the heading and message
  texts they match are literals in source, but no install has shown them on disk.
- Not determined: the content of task-scheduler contexts beyond the logged prompt, what an ad hoc
  task's `token` is used for, and the `responses` metadata for every provider.
