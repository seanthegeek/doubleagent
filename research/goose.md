# goose

Catalog agent `goose`, the open-source agent first published by Block and now governed by the
Agentic AI Foundation (AAIF, a Linux Foundation project): a Rust CLI (`goose`) and an Electron
desktop app (`Goose`) that share one set of Rust config, data and state directories and keep their
sessions in one SQLite database, `sessions.db`. Beside them run a gateway (a chat-platform front
end that relays a Telegram chat into a session), roaming (`goose roam`, peer-to-peer remote
control of an agent over ACP, the Agent Client Protocol the desktop app also speaks to its
backend), the scheduler (recipes, Goose's YAML task files, run on a cron schedule) and the
built-in MCP servers (Model Context Protocol tool servers such as the memory and computer
controller extensions). Parsed: the goose parser reads `sessions.db`, the legacy JSONL sessions,
the `llm_request` logs, both `history.txt` files, `permission.yaml`, `roaming_trust.json` and
`schedule.json`. This document rests on version 1.57.0 at commit
[`b23d21c`](https://github.com/aaif-goose/goose/commit/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc).

## Source and evidence

[aaif-goose/goose](https://github.com/aaif-goose/goose) at
[`b23d21c`](https://github.com/aaif-goose/goose/commit/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc)
(committed 2026-10-31), Apache-2.0, source level. The workspace version at that commit is 1.57.0
([`Cargo.toml`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/Cargo.toml#L9),
[`ui/desktop/package.json`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/ui/desktop/package.json#L5)),
and the latest release tag is `v1.57.0` at
[`30bba95`](https://github.com/aaif-goose/goose/commit/30bba95105e59bc3e7df624cf6b4d433690cfe12)
(2026-10-27). The release writes the same shapes as the reviewed commit except the one metadata
key [Format versions](#format-versions) names; `session_manager.rs`, the request log and the
history code are unchanged between them. The repository was `block/goose` and moved to
`aaif-goose/goose`, which its [README](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/README.md#L29) names as part of the Agentic AI
Foundation; `github.com/block/goose` redirects to it, so links to either name are one
repository. Earlier citations kept in [Format versions](#format-versions) name the commits of the
[Source reviews](#source-reviews).

The pinned crates were read at the versions `Cargo.lock` gives: etcetera 0.11.0
([`Cargo.lock`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/Cargo.lock#L3330-L3331)) at
[lunacookies/etcetera `07b3f54`](https://github.com/lunacookies/etcetera/commit/07b3f54999e4f3ff0f027de0144c9349d820ce80)
(tag `v0.11.0`), rustyline 18.0.1 ([`Cargo.lock`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/Cargo.lock#L8591-L8592)) at
[kkawakam/rustyline `cc1aab0`](https://github.com/kkawakam/rustyline/commit/cc1aab036d62aff3dbdc681b08949076ec275030),
rmcp 3.4.1 ([`Cargo.lock`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/Cargo.lock#L8366-L8367)) at
[modelcontextprotocol/rust-sdk `9427a92`](https://github.com/modelcontextprotocol/rust-sdk/commit/9427a929959e665e0d12e9395f674026baf4bd48),
sqlx-sqlite 0.9.0 ([`Cargo.lock`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/Cargo.lock#L9730-L9731)) at
[launchbadge/sqlx `75bc048`](https://github.com/launchbadge/sqlx/commit/75bc0487eb661da811bb7a3c5d158f1bd463fef4),
and serde_yaml 0.9.34 ([`Cargo.lock`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/Cargo.lock#L9069-L9070)). The Electron `userData`
locations are Electron's convention, documented under
[app.getPath](https://www.electronjs.org/docs/latest/api/app#appgetpathname), not goose source.

### Observed

- No real install. What stands in is the source at
  [`b23d21c`](https://github.com/aaif-goose/goose/commit/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc)
  (version 1.57.0) and the crates above. Every row form of [Parser plan](#parser-plan) is
  source-derived and fixture only; no row has been checked against a real install.

### Source reviews

- 2026-10-31: [aaif-goose/goose](https://github.com/aaif-goose/goose) at
  [`b23d21c`](https://github.com/aaif-goose/goose/commit/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc)
  (version 1.57.0, release tag `v1.57.0`). The document was rewritten at that commit. It added the
  cache directory and the computer controller's screenshots, the Claude Code provider's temporary
  MCP configuration, the desktop app's `settings.json` backend secret and its prompt history in
  local storage, the `.agents` skills and recipes roots, the `goose review` project files and the
  `diagnostics_<session_id>.json` export, and the `compactionSummary` and `operationLogs` metadata
  keys; it corrected the elicitation keys to snake case and settled the request logs as a
  secondary source and every credential path per platform.
- 2026-10-21: [aaif-goose/goose](https://github.com/aaif-goose/goose) at
  [`fb9e3c8`](https://github.com/aaif-goose/goose/commit/fb9e3c89b2b4670a0eded99e7567e578ac6e502c)
  (committed 2026-10-19): it added the file formats of [Permission files](#permission-files)
  and [Scheduler and project files](#scheduler-and-project-files), the chunk assembly of
  [Request logs](#request-logs) and the review of [Source host rows](#source-host-rows), and
  the parser plan's permission, scheduled-task and request-log rows now describe what the parser
  writes.
- 2026-10-19: [aaif-goose/goose](https://github.com/aaif-goose/goose) at
  [`fb9e3c8`](https://github.com/aaif-goose/goose/commit/fb9e3c89b2b4670a0eded99e7567e578ac6e502c)
  (committed 2026-10-19), against
  [`591edd4`](https://github.com/block/goose/commit/591edd47cf2cfea4957d720c607cf2a4def8673d)
  and, for sync,
  [`bfcc6ee`](https://github.com/aaif-goose/goose/commit/bfcc6ee6f1ee18d0c4c852a65c49b25f87ee40dc).
  It added the tool-confirmation records of [Record schema](#record-schema), the permission,
  scheduler and project files of [Other stores](#other-stores), the scratch directories, the
  delete table of [What survives a delete](#what-survives-a-delete) and the schema 17 changes of
  [Format versions](#format-versions), and corrected the parser plan's statement that the answer
  to a tool confirmation is not recorded.
- 2026-10-17: [aaif-goose/goose](https://github.com/aaif-goose/goose) at
  [`bfcc6ee`](https://github.com/aaif-goose/goose/commit/bfcc6ee6f1ee18d0c4c852a65c49b25f87ee40dc),
  for sync. It added the gateway pairings of [Joins and timestamps](#joins-and-timestamps) and
  the `session relayed:` row of [Sync, relay and origin rows](#sync-relay-and-origin-rows).

## Collection

### Per-user storage

[`crates/goose/src/config/paths.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/config/paths.rs#L22-L36)
uses `etcetera::choose_app_strategy` with author `Block` and app `goose`;
[`etcetera/src/app_strategy.rs`](https://github.com/lunacookies/etcetera/blob/07b3f54999e4f3ff0f027de0144c9349d820ce80/src/app_strategy.rs#L157-L165)
picks the `Windows` strategy on Windows and `Xdg` everywhere else, macOS included. The
directories are:

| Directory | Linux and macOS | Windows |
| --- | --- | --- |
| config | `.config/goose` | `AppData/Roaming/Block/goose/config` |
| data | `.local/share/goose` | `AppData/Roaming/Block/goose/data` |
| state | `.local/state/goose` | `AppData/Roaming/Block/goose/data` (no state directory, so state falls back to data) |
| cache (built-in MCP servers only) | `.cache/goose` | `AppData/Local/Block/goose/cache` |

The XDG forms honour `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME` and `XDG_CACHE_HOME`
([`etcetera/src/app_strategy/xdg.rs`](https://github.com/lunacookies/etcetera/blob/07b3f54999e4f3ff0f027de0144c9349d820ce80/src/app_strategy/xdg.rs#L197-L216)).
The Windows forms are `%APPDATA%\Block\goose\config` and `\data`, and `%LOCALAPPDATA%` for the
cache ([`etcetera/src/app_strategy/windows.rs`](https://github.com/lunacookies/etcetera/blob/07b3f54999e4f3ff0f027de0144c9349d820ce80/src/app_strategy/windows.rs#L136-L165);
[`base_strategy/windows.rs`](https://github.com/lunacookies/etcetera/blob/07b3f54999e4f3ff0f027de0144c9349d820ce80/src/base_strategy/windows.rs#L190-L201));
the state fallback is
[`paths.rs:32`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/config/paths.rs#L32).
`GOOSE_PATH_ROOT`, when absolute, replaces the first three with `<root>/config`, `<root>/data`
and `<root>/state`, and the home's `.agents` with `<root>/.agents`
([`paths.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/config/paths.rs#L8-L17),
[`40-46`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/config/paths.rs#L40-L46));
the built-in MCP servers ignore it and keep the cache directory
([`goose-mcp/src/lib.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-mcp/src/lib.rs#L14-L18)).
The comment at
[`paths.rs:19-21`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/config/paths.rs#L19-L21)
names `Library/Application Support/Block/goose` as a legacy macOS location; no code at the
reviewed commit reads or migrates it.

#### Scratch directories

A gateway session runs in `gateway/<platform>/<user_id>` under the config directory
([`gateway/handler.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/gateway/handler.rs#L303-L305),
[`801-806`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/gateway/handler.rs#L801-L806)),
so `.config/goose/gateway` and `AppData/Roaming/Block/goose/config/gateway` hold the files a
chat user's session wrote. A GitHub recipe is cloned with `gh repo clone` into the OS temp
directory as `<temp>/<owner>/<repo>`
([`goose-cli/src/recipes/github_recipe.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-cli/src/recipes/github_recipe.rs#L187-L205)),
outside every home; it holds recipe sources, not a session's working directory. A Telegram voice
message is saved to a temporary file in the OS temp directory
([`gateway/telegram.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/gateway/telegram.rs#L698-L701)).

The desktop app's default working directory is the home itself
([`ui/desktop/src/utils/workingDir.ts`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/ui/desktop/src/utils/workingDir.ts#L55-L59));
a scheduled job runs in the scheduler process's current directory
([`scheduler/full.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/scheduler/full.rs#L878-L886)),
and `goose roam share` in its `--cwd` or the directory it was started in
([`commands/roam.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-cli/src/commands/roam.rs#L652-L664)).
The agent creates no worktree.

#### Config directory

`.config/goose` (Windows `AppData/Roaming/Block/goose/config`) holds:

- `config.yaml`, the user configuration, with `config.tmp` written beside it and renamed over it
  on every save
  ([`config/base.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/config/base.rs#L200-L205),
  [`650-673`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/config/base.rs#L650-L673)); also the system files
  `/etc/goose/config.yaml` and `C:\ProgramData\goose\config.yaml`, and any file named in
  `GOOSE_ADDITIONAL_CONFIG_FILES`
  ([`base.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/config/base.rs#L158-L180)).
- `secrets.yaml` and its `secrets.yaml.lock`
  ([`base.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/config/base.rs#L21-L29),
  [`397-399`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/config/base.rs#L397-L399)); see
  [Credentials](#credentials).
- `permission.yaml` and `permission.yaml.lock`, the tool allow, ask and deny lists
  ([`config/permission.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/config/permission.rs#L13-L16),
  [`200-212`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/config/permission.rs#L200-L212);
  [Permission files](#permission-files)).
- `roaming_trust.json`, `roaming_peers.json` and `roaming_node_key`, the roaming allowlist, the
  saved peers and this node's private key
  ([`goose-cli/src/commands/roam.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-cli/src/commands/roam.rs#L36-L46);
  [`goose-roaming/src/identity.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-roaming/src/identity.rs#L17),
  [`113-116`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-roaming/src/identity.rs#L113-L116)).
- `tls/server.pem` and `tls/server.key`, the certificate and key of the local ACP server
  ([`acp/transport/tls.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/acp/transport/tls.rs#L88-L90),
  [`115-118`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/acp/transport/tls.rs#L115-L118)).
- `history.txt`, the CLI's legacy history location
  ([`goose-cli/src/session/mod.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-cli/src/session/mod.rs#L200-L201)).
- `recipes/`, `skills/`, `agents/`, `prompts/`, `custom_providers/` and `memory/` (the memory
  extension's global `<category>.txt` files)
  ([`recipe/local_recipes.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/recipe/local_recipes.rs#L16);
  [`skills/mod.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/skills/mod.rs#L169);
  [`sources.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/sources.rs#L470);
  [`prompt_template.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/prompt_template.rs#L67);
  [`config/declarative_providers.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/config/declarative_providers.rs#L23);
  [`goose-mcp/src/memory/mod.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-mcp/src/memory/mod.rs#L133-L135),
  [`205-214`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-mcp/src/memory/mod.rs#L205-L214)).
- `.goosehints` and `AGENTS.md`, global hints
  ([`hints/load_hints.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/hints/load_hints.rs#L239-L248)),
  and `adversary.md`, the adversary inspector's rules
  ([`security/adversary_inspector.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/security/adversary_inspector.rs#L90)).
- `mcp-apps-cache/`, the default MCP app pages copied out of the binary
  ([`goose_apps/cache.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/goose_apps/cache.rs#L33-L38)).
- `gateway/<platform>/<user_id>/` ([Scratch directories](#scratch-directories)).
- `permissions/tool_permissions.json`, left only by older releases
  ([Permission files](#permission-files)).
- The provider token caches of [Credentials](#credentials).

#### Data directory

`.local/share/goose` (Windows `AppData/Roaming/Block/goose/data`) holds:

- `sessions/sessions.db` with its `-wal` and `-shm` sidecars, and the legacy
  `sessions/<YYYYMMDD_HHMMSS>.jsonl` files
  ([`session/session_manager.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/session/session_manager.rs#L32-L34),
  [`1096-1105`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/session/session_manager.rs#L1096-L1105);
  [`session/legacy.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/session/legacy.rs#L13-L29)).
- `schedule.json` and `scheduled_recipes/`, the scheduled jobs and the copies of their recipes
  ([`scheduler/common.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/scheduler/common.rs#L94-L105)).
- `projects/<slug>.md`, a project name with its working directories
  ([`sources.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/sources.rs#L69-L75)).
- `apps/<name>.html`, pages the model wrote through the apps extension
  ([`agents/platform_extensions/apps.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/agents/platform_extensions/apps.rs#L57),
  [`137`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/agents/platform_extensions/apps.rs#L137);
  [`acp/server/apps.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/acp/server/apps.rs#L204)).
- `models/`, downloaded Whisper dictation weights
  ([`dictation/whisper.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/dictation/whisper.rs#L94)), and
  `model_catalog/`, a cached copy of the `models.dev` model list
  ([`model_catalog.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/model_catalog.rs#L5-L9)).
- `roam/serve.json` and `roam/serve.lock`, the status and lock of a running `goose roam share`
  ([`goose-cli/src/cli.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-cli/src/cli.rs#L1715);
  [`commands/roam.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-cli/src/commands/roam.rs#L616)).

#### State directory

`.local/state/goose` (Windows: the data directory) holds:

- `logs/<component>/<YYYY-MM-DD>/`, the application logs, pruned after two weeks
  ([`logging.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/logging.rs#L116-L145)).
- `logs/llm_request.<N>.jsonl`, the last provider requests in full, written first as
  `logs/llm_request.<uuid>.jsonl` ([Request logs](#request-logs)).
- `history.txt`, the CLI's input history
  ([`goose-cli/src/session/mod.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-cli/src/session/mod.rs#L200)).
- `instance_id` and `telemetry_installation.json`, an installation id, first-seen time and
  session count
  ([`instance_id.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/instance_id.rs#L8-L10);
  [`posthog.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/posthog.rs#L97-L115)).
- `roaming_directory.json`, the peers seen over roaming
  ([`commands/roam.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-cli/src/commands/roam.rs#L36-L38)).
- `codex/images/`, images handed to the Codex CLI provider
  ([`providers/codex.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/providers/codex.rs#L121-L123)).
- `claude-code/system-prompt-<YYYYMMDD>_<random>.txt` and
  `claude-code/mcp-config-<YYYYMMDD>_<random>.json`, temporary files of the Claude Code CLI
  provider, deleted when the provider is dropped
  ([`providers/claude_code.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/providers/claude_code.rs#L585-L619),
  [`662-664`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/providers/claude_code.rs#L662-L664)).

#### Cache directory

`.cache/goose` (Windows `AppData/Local/Block/goose/cache`) is written only by the built-in MCP
servers: `computer_controller/` holds the screenshots the computer controller tool took,
`<prefix>_<YYYYMMDD_HHMMSS>.png`, and `pdf_images/` below it the images it extracted from PDF
files
([`goose-mcp/src/computercontroller/mod.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-mcp/src/computercontroller/mod.rs#L235-L248),
[`383-387`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-mcp/src/computercontroller/mod.rs#L383-L387);
[`pdf_tool.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-mcp/src/computercontroller/pdf_tool.rs#L120-L121)),
and `autovisualiser/` the autovisualiser extension's files
([`goose-mcp/src/autovisualiser/mod.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-mcp/src/autovisualiser/mod.rs#L689-L699)).

#### Shared cross-agent directories

Goose writes agents to `~/.agents/agents` and skills to `~/.agents/skills`, and installs plugins
in `~/.agents/plugins`
([`sources.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/sources.rs#L327-L341);
[`skills/mod.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/skills/mod.rs#L46-L48);
[`plugins/mod.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/plugins/mod.rs#L35-L37)).
It reads, and through the desktop app's source editor may write, agent files under
`~/.goose/agents` and `~/.claude/agents`
([`sources.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/sources.rs#L536-L544));
it reads skills from `~/.claude/skills` and `~/.config/agents/skills`
([`skills/mod.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/skills/mod.rs#L166-L173)),
recipes from `~/.goose/recipes` and `~/.agents/recipes`
([`agents/platform_extensions/summon.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/agents/platform_extensions/summon.rs#L262-L274)),
and global hints from `~/.agents/AGENTS.md`
([`hints/load_hints.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/hints/load_hints.rs#L243-L248)).
Its first-run import reads Claude Desktop's `claude_desktop_config.json` to copy its MCP servers
([`acp/server/onboarding.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/acp/server/onboarding.rs#L180-L199)).

#### Electron desktop app

The desktop app's `productName` is `Goose`
([`ui/desktop/package.json`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/ui/desktop/package.json#L3)),
so its Electron `userData` directory is `.config/Goose` on Linux,
`Library/Application Support/Goose` on macOS and `AppData/Roaming/Goose` on Windows. Under it the
app writes `settings.json`, `logs/startup/`, `logs/main.log`, `recent-dirs.json` and
`recipe_hashes/<sha256>.hash` (the time a recipe was first accepted)
([`ui/desktop/src/main.ts`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/ui/desktop/src/main.ts#L184-L185);
[`utils/logger.ts`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/ui/desktop/src/utils/logger.ts#L6);
[`utils/recentDirs.ts`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/ui/desktop/src/utils/recentDirs.ts#L5);
[`utils/recipeHash.ts`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/ui/desktop/src/utils/recipeHash.ts#L12-L38)).
Its windows use the persistent partition `persist:goose`
([`main.ts`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/ui/desktop/src/main.ts#L1286)),
whose local storage holds the prompt history of
[Desktop prompt history](#desktop-prompt-history). The app runs the same Rust backend over ACP,
which writes the directories above.

Of the paths [issue 23](https://github.com/seanthegeek/doubleagent/issues/23) questioned,
`.config/Goose` is this Electron directory on Linux, derived from `productName` rather than
written as a literal, and `~/.goose` holds `agents/` and `recipes/`, which Goose reads and its
source editor may write ([Shared cross-agent directories](#shared-cross-agent-directories)).

### Credentials

Secrets go to the OS keyring under service `goose`, account `secrets`, as one JSON object
([`config/base.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/config/base.rs#L31-L34)),
unless `GOOSE_DISABLE_KEYRING` is set in the environment or in `config.yaml`, or the keyring fails;
then they go to `.config/goose/secrets.yaml` (Windows
`AppData/Roaming/Block/goose/config/secrets.yaml`), written mode 0600 through a temporary file
renamed over it
([`base.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/config/base.rs#L216-L222),
[`401-420`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/config/base.rs#L401-L420),
[`1171-1191`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/config/base.rs#L1171-L1191);
[`providers/private_file.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/providers/private_file.rs#L240-L257)).
The lock file `.config/goose/secrets.yaml.lock` is not a credential file.

`.config/goose/config.yaml` (Windows `AppData/Roaming/Block/goose/config/config.yaml`) is a
credential file: an MCP extension's entry holds its `envs` and, for a streamable HTTP server,
`headers`
([`agents/extension.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/agents/extension.rs#L155-L222)),
and any key can be set there by hand. Its save copy `.config/goose/config.tmp` (Windows
`AppData/Roaming/Block/goose/config/config.tmp`) is the same content.

Provider token caches are files under the config directory whatever the keyring setting
([`providers/provider_secrets.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/providers/provider_secrets.rs#L109-L152)),
on Linux and macOS:

- `.config/goose/githubcopilot/info.json` and `.config/goose/githubcopilot/*/info.json`
  ([`githubcopilot.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/providers/githubcopilot.rs#L178-L186)).
- `.config/goose/chatgpt_codex/tokens.json`
  ([`chatgpt_codex.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/providers/chatgpt_codex.rs#L332)).
- `.config/goose/gemini_oauth/tokens.json`
  ([`gemini_oauth.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/providers/gemini_oauth.rs#L153)).
- `.config/goose/kimicode/token.json`
  ([`kimicode.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/providers/kimicode.rs#L105)).
- `.config/goose/muse_code/token.json`
  ([`muse_code_def.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/providers/muse_code_def.rs#L248)).
- `.config/goose/huggingface/oauth/tokens.json`
  ([`huggingface_auth.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/providers/huggingface_auth.rs#L19);
  [`goose-local-inference/src/huggingface_auth.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-local-inference/src/huggingface_auth.rs#L10)).
- `.config/goose/databricks/oauth/*.json`, one file per host, client and scope set
  ([`providers/oauth.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/providers/oauth.rs#L95-L118)).
- `.config/goose/xai_oauth/tokens.json`
  ([`xai_oauth.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/providers/xai_oauth.rs#L106)).

On Windows the same files are under `AppData/Roaming/Block/goose/config`:
`AppData/Roaming/Block/goose/config/githubcopilot/info.json`,
`AppData/Roaming/Block/goose/config/githubcopilot/*/info.json`,
`AppData/Roaming/Block/goose/config/chatgpt_codex/tokens.json`,
`AppData/Roaming/Block/goose/config/gemini_oauth/tokens.json`,
`AppData/Roaming/Block/goose/config/kimicode/token.json`,
`AppData/Roaming/Block/goose/config/muse_code/token.json`,
`AppData/Roaming/Block/goose/config/huggingface/oauth/tokens.json`,
`AppData/Roaming/Block/goose/config/databricks/oauth/*.json` and
`AppData/Roaming/Block/goose/config/xai_oauth/tokens.json`. The file
`.config/goose/kimicode/device_id` is not a credential: it holds a random device identifier
([`kimicode.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/providers/kimicode.rs#L197-L206)).

`secrets.yaml` and the token caches of GitHub Copilot, ChatGPT Codex, Gemini, Muse Code,
Databricks and xAI are written through `write_private_file`, which writes the whole content to a
temporary file in the target's directory and renames it over the target
([`providers/private_file.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/providers/private_file.rs#L150-L152),
[`240-257`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/providers/private_file.rs#L240-L257);
[`config/base.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/config/base.rs#L21-L23)). The temporary name is
tempfile's default, `.tmp` and six random letters or digits (tempfile 3.27.0,
[`Cargo.lock`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/Cargo.lock#L10195-L10196), at
[Stebalien/tempfile `5c8fa12`](https://github.com/Stebalien/tempfile/commit/5c8fa12eb584931b4f1bccfde87eb72fbfa7dc61):
[`src/lib.rs`](https://github.com/Stebalien/tempfile/blob/5c8fa12eb584931b4f1bccfde87eb72fbfa7dc61/src/lib.rs#L227-L236), [`195`](https://github.com/Stebalien/tempfile/blob/5c8fa12eb584931b4f1bccfde87eb72fbfa7dc61/src/lib.rs#L195);
[`src/util.rs`](https://github.com/Stebalien/tempfile/blob/5c8fa12eb584931b4f1bccfde87eb72fbfa7dc61/src/util.rs#L7-L19)), so a write cut short leaves a copy of the credential:
`.config/goose/.tmp??????`, `.config/goose/githubcopilot/.tmp??????`,
`.config/goose/githubcopilot/*/.tmp??????`, `.config/goose/chatgpt_codex/.tmp??????`,
`.config/goose/gemini_oauth/.tmp??????`, `.config/goose/muse_code/.tmp??????`,
`.config/goose/databricks/oauth/.tmp??????` and `.config/goose/xai_oauth/.tmp??????` on Linux and
macOS, and
`AppData/Roaming/Block/goose/config/.tmp??????`,
`AppData/Roaming/Block/goose/config/githubcopilot/.tmp??????`,
`AppData/Roaming/Block/goose/config/githubcopilot/*/.tmp??????`,
`AppData/Roaming/Block/goose/config/chatgpt_codex/.tmp??????`,
`AppData/Roaming/Block/goose/config/gemini_oauth/.tmp??????`,
`AppData/Roaming/Block/goose/config/muse_code/.tmp??????`,
`AppData/Roaming/Block/goose/config/databricks/oauth/.tmp??????` and
`AppData/Roaming/Block/goose/config/xai_oauth/.tmp??????` on Windows. The Kimi Code and Hugging Face
caches are written in place
([`kimicode.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/providers/kimicode.rs#L124-L128);
[`huggingface_auth.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/providers/huggingface_auth.rs#L204-L210)).

A custom provider's file, `.config/goose/custom_providers/<id>.json` (Windows
`AppData/Roaming/Block/goose/config/custom_providers/<id>.json`), is a credential file: it is a
`DeclarativeProviderConfig` whose `headers` is a free map of HTTP headers the user supplied
([`goose-providers/src/declarative.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-providers/src/declarative.rs#L155-L177),
[`164`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-providers/src/declarative.rs#L164)), copied from the request when the
provider is created and kept or replaced when it is updated
([`config/declarative_providers.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/config/declarative_providers.rs#L226), [`344-347`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/config/declarative_providers.rs#L344-L347)); only the API key
goes to the secret store, under the name in `api_key_env` ([`195-207`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/config/declarative_providers.rs#L195-L207)). The file
is written through `write_private_file` ([`22-24`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/config/declarative_providers.rs#L22-L24), [`131-136`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/config/declarative_providers.rs#L131-L136)), so
its temporary copy is `.config/goose/custom_providers/.tmp??????` (Windows
`AppData/Roaming/Block/goose/config/custom_providers/.tmp??????`).

Private keys: `.config/goose/roaming_node_key` and
`AppData/Roaming/Block/goose/config/roaming_node_key`, the roaming node's ed25519 key
([`goose-roaming/src/lib.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-roaming/src/lib.rs#L10-L11)), and
`.config/goose/tls/server.key` and `AppData/Roaming/Block/goose/config/tls/server.key`, the local
ACP server's TLS key
([`acp/transport/tls.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/acp/transport/tls.rs#L92-L118)).

The Claude Code provider's temporary MCP configuration holds each MCP server's resolved `env`
values ([`providers/claude_code.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/providers/claude_code.rs#L543-L582)):
`.local/state/goose/claude-code/mcp-config-*.json` on Linux and macOS,
`AppData/Roaming/Block/goose/data/claude-code/mcp-config-*.json` on Windows.

The desktop app's `settings.json` holds `externalGoosed.secret`, the shared secret of an
external backend server
([`ui/desktop/src/utils/settings.ts`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/ui/desktop/src/utils/settings.ts#L6-L12),
[`46`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/ui/desktop/src/utils/settings.ts#L46);
[`main.ts`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/ui/desktop/src/main.ts#L892-L895)):
`.config/Goose/settings.json`, `Library/Application Support/Goose/settings.json` and
`AppData/Roaming/Goose/settings.json`.

The project-local `diagnostics_<session_id>.json` ([Project-local files](#project-local-files))
is a credential file: its `config.configYaml` holds `config.yaml` as read, up to 256 KiB
([`session/diagnostics.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/session/diagnostics.rs#L35-L41),
[`315-328`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/session/diagnostics.rs#L315-L328)).

Recipes and the stored recipe of a session can carry extension entries with `envs` and are
listed in [Secrets](#secrets) as stores that mix credentials with evidence.

### Exclusions

Downloaded or copied subtrees: `.local/share/goose/models`, `.local/share/goose/model_catalog`,
`.local/state/goose/codex`, `.config/goose/mcp-apps-cache`,
`AppData/Roaming/Block/goose/data/models`, `AppData/Roaming/Block/goose/data/model_catalog`,
`AppData/Roaming/Block/goose/data/codex`, `AppData/Roaming/Block/goose/config/mcp-apps-cache`,
and the Electron app's Chromium caches under each `userData` directory, `*/Goose/Cache`,
`*/Goose/Code Cache`, `*/Goose/GPUCache`, `*/Goose/DawnGraphiteCache`,
`*/Goose/DawnWebGPUCache` and `*/Goose/blob_storage`. `apps/` is agent output (pages the model
wrote), and the cache directory's screenshots are the computer controller's own output, so
neither is a copy.

### Project-local files

- `.goosehints` and `AGENTS.md`, walked from the working directory upward; `CONTEXT_FILE_NAMES`
  in `config.yaml` replaces the list
  ([`hints/load_hints.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/hints/load_hints.rs#L10-L24)).
- `.goose/memory/<category>.txt`, the memory extension's project memories
  ([`goose-mcp/src/memory/mod.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-mcp/src/memory/mod.rs#L205-L214)).
- `.goose/recipes` and `.agents/recipes`, read as recipes; `.goose/recipes` is also where the CLI
  lists local recipes
  ([`summon.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/agents/platform_extensions/summon.rs#L255-L258);
  [`recipe/local_recipes.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/recipe/local_recipes.rs#L18)).
- `.agents/agents`, `.goose/agents` and `.claude/agents`, agent files, the first written
  ([`sources.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/sources.rs#L327-L340),
  [`532-536`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/sources.rs#L532-L536)).
- `.agents/skills`, `.goose/skills` and `.claude/skills`, skills, the first written
  ([`skills/mod.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/skills/mod.rs#L50-L54),
  [`321-326`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/skills/mod.rs#L321-L326)).
- `.agents/plugins`, plugins installed for one project
  ([`plugins/mod.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/plugins/mod.rs#L39-L41)).
- `.agents/checks/*.md` and `.agents/REVIEW.md`, read by `goose review`
  ([`goose-cli/src/commands/review/mod.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-cli/src/commands/review/mod.rs#L3)).
- `diagnostics_<session_id>.json`, written by `goose session diagnostics` in the current directory
  unless `-o` names a file: the session with its conversation, the configuration, logs and the
  schedule
  ([`goose-cli/src/commands/session.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-cli/src/commands/session.rs#L400-L424);
  [`session/diagnostics.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/session/diagnostics.rs#L87-L100)).

No per-project database. `.gooseignore` has no reader in `crates/` or `ui/desktop/src`.

### Where the project path is recorded

- `sessions.db` table `sessions` column `working_dir`
  ([`session_manager.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/session/session_manager.rs#L1164-L1170)).
- The legacy JSONL first line, key `working_dir`
  ([`legacy.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/session/legacy.rs#L74)).
- `projects/<slug>.md` front matter key `workingDirs`, a list of paths
  ([`sources.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/sources.rs#L258-L268)).
- `schedule.json` key `recipe_base_dir`, the directory a scheduled recipe came from
  ([`scheduler/common.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/scheduler/common.rs#L182-L186)).
- The desktop app's `recent-dirs.json`, key `dirs`, the last ten working directories
  ([`recentDirs.ts`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/ui/desktop/src/utils/recentDirs.ts#L5-L10)).

## Parsing

### Transcript stores

The stores are under the directories of [Per-user storage](#per-user-storage): on macOS the XDG
ones, not `Library/Application Support`, which holds only the desktop app's Electron directory;
on Windows state falls back to data.

| Store | Linux and macOS | Windows | Unit |
| --- | --- | --- | --- |
| Sessions database | `.local/share/goose/sessions/sessions.db` (+`-wal`, `-shm`) | `AppData/Roaming/Block/goose/data/sessions/sessions.db` | a `sessions` row is a session, a `messages` row one message ([`session_manager.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/session/session_manager.rs#L1162-L1220)) |
| Legacy sessions | `.local/share/goose/sessions/<YYYYMMDD_HHMMSS>.jsonl` | same directory | line 1 metadata, then one message per line ([`legacy.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/session/legacy.rs#L62-L101)) |
| LLM request log | `.local/state/goose/logs/llm_request.<N>.jsonl` (temporary `llm_request.<uuid>.jsonl`) | `AppData/Roaming/Block/goose/data/logs/` | one file per provider call ([`providers/utils.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/providers/utils.rs#L81-L101)) |
| CLI history | `.local/state/goose/history.txt` (old `.config/goose/history.txt`) | `AppData/Roaming/Block/goose/data/history.txt` | rustyline's version 2 format: a `#V2` header, one entry per line, newlines and backslashes escaped ([`goose-cli/src/session/mod.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-cli/src/session/mod.rs#L200-L201); [`rustyline/src/history.rs`](https://github.com/kkawakam/rustyline/blob/cc1aab036d62aff3dbdc681b08949076ec275030/src/history.rs#L461-L465)) |
| Desktop prompt history | local storage of the `persist:goose` partition under the desktop app's `userData` | same | one JSON array under one key ([Desktop prompt history](#desktop-prompt-history)) |

The database is opened with `journal_mode=WAL`, a 30-second `busy_timeout` and foreign keys on,
its directory mode 0700
([`session_manager.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/session/session_manager.rs#L1074-L1093)).
Recent rows live in `sessions.db-wal` until a checkpoint. There is no encryption.

#### What survives a delete

The session store loses messages in the ways below, and `history.txt` (CLI prompts only, the
last 100), the `llm_request` logs (the last 10 provider calls, `LOGS_TO_KEEP`,
[`providers/utils.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/providers/utils.rs#L50),
rotated at [`121-137`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/providers/utils.rs#L121-L137)) and the desktop
prompt history (the last 500 desktop prompts of 30 days) are the local copies left after each:

| Action | What it does to `sessions.db` |
| --- | --- |
| `goose session remove`, the desktop app's delete (an ACP `delete_session` request) | deletes the session's `messages`, `usage_ledger` and `sessions` rows ([`commands/session.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-cli/src/commands/session.rs#L43-L46); [`acp/server/manage_sessions.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/acp/server/manage_sessions.rs#L93-L107); [`session_manager.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/session/session_manager.rs#L2529-L2560)) |
| `/clear` in the CLI | replaces the conversation with an empty one: every `messages` row of the session is deleted, the `sessions` row stays ([`goose-cli/src/session/mod.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-cli/src/session/mod.rs#L1088-L1106); [`session_manager.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/session/session_manager.rs#L2201-L2212)) |
| `goose session --edit` | rewrites the conversation from an external editor the same way; `--fork` copies the session first and edits the copy ([`goose-cli/src/cli.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-cli/src/cli.rs#L2075-L2098)) |
| a rewind (ACP `truncate_session_conversation`, `goose term`, or a fork from a message) | deletes the session's messages from a timestamp or a message id on ([`manage_sessions.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/acp/server/manage_sessions.rs#L178-L195); [`goose-cli/src/commands/term.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-cli/src/commands/term.rs#L337-L346); [`acp/server/fork_session.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/acp/server/fork_session.rs#L37-L44); [`session_manager.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/session/session_manager.rs#L2870-L2912)) |
| archive (ACP `archive_session`) | sets `sessions.archived_at` and deletes nothing; unarchive clears it ([`manage_sessions.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/acp/server/manage_sessions.rs#L236-L262)) |

Compaction is not a loss: it rewrites the older messages' `metadata_json` to hide them from the
model and inserts the summary, so the compacted turns stay in `messages`
([`session_manager.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/session/session_manager.rs#L2250-L2300);
[`context_mgmt/mod.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/context_mgmt/mod.rs#L180-L197)).
No delete touches `permission.yaml`, `gateway_pairings`, `schedule.json`, the gateway working
directory, an export file or a `diagnostics_<session_id>.json` file. Source shows no local record
of a delete.

`goose session remove` leaves `history.txt` alone, so the prompts of a removed session stay in
it. It does not survive many sessions: the editor is built without a history size
([`goose-cli/src/session/mod.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-cli/src/session/mod.rs#L672-L689)),
so rustyline's default of 100 entries applies and a consecutive repeat is not added
([`rustyline/src/config.rs`](https://github.com/kkawakam/rustyline/blob/cc1aab036d62aff3dbdc681b08949076ec275030/src/config.rs#L248-L251);
[`history.rs`](https://github.com/kkawakam/rustyline/blob/cc1aab036d62aff3dbdc681b08949076ec275030/src/history.rs#L264-L278)).

Unpairing a chat user and removing a gateway delete the entries from `gateway_pairings`
([`gateway/manager.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/gateway/manager.rs#L175-L193),
[`331-343`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/gateway/manager.rs#L331-L343))
and leave the `gateway` session row, its name and its working directory; `goose session remove`
deletes the rows and leaves the `gateway_pairings` entry, which then names a session that no
longer exists. The working directory under `.config/goose/gateway/` is never removed by either.
No gateway code deletes chat messages on the platform (no `deleteMessage` call in
`crates/goose/src/gateway/`), so the chat copy on Telegram outlives every local delete.

The analyzer reads `history.txt` and the `llm_request` logs on every run, both as secondary
sources, so a removed, cleared or rewound session's prompts and latest replies are kept where
the database lost them; see
[Past-use rows and secondary sources](#past-use-rows-and-secondary-sources) and
[Secondary sources](../analyzer/docs/output.md#secondary-sources).

### Record schema

#### Sessions database

`sessions.db` is at schema version 17
([`session_manager.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/session/session_manager.rs#L32)).
A new database is created with this statement
([`session_manager.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/session/session_manager.rs#L1146-L1240)):

```sql
CREATE TABLE schema_version (version INTEGER PRIMARY KEY, applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE sessions (
  id TEXT PRIMARY KEY, name TEXT NOT NULL DEFAULT '', description TEXT NOT NULL DEFAULT '',
  user_set_name BOOLEAN DEFAULT FALSE, session_type TEXT NOT NULL DEFAULT 'user',
  working_dir TEXT NOT NULL, created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, extension_data TEXT DEFAULT '{}',
  total_tokens INTEGER, input_tokens INTEGER, output_tokens INTEGER, cache_read_tokens INTEGER,
  cache_write_tokens INTEGER, accumulated_total_tokens INTEGER, accumulated_input_tokens INTEGER,
  accumulated_output_tokens INTEGER, accumulated_cache_read_tokens INTEGER,
  accumulated_cache_write_tokens INTEGER, accumulated_cost REAL, schedule_id TEXT, recipe_json TEXT,
  user_recipe_values_json TEXT, provider_name TEXT, model_config_json TEXT,
  goose_mode TEXT NOT NULL DEFAULT 'auto', archived_at TIMESTAMP, project_id TEXT, parent_session_id TEXT,
  system_prompt_override TEXT, system_prompt_extras_json TEXT, container_id TEXT);
CREATE TABLE messages (
  id INTEGER PRIMARY KEY AUTOINCREMENT, message_id TEXT, session_id TEXT NOT NULL REFERENCES sessions(id),
  role TEXT NOT NULL, content_json TEXT NOT NULL, created_timestamp INTEGER NOT NULL,
  timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP, tokens INTEGER, metadata_json TEXT);
CREATE TABLE usage_ledger (
  id INTEGER PRIMARY KEY AUTOINCREMENT, session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
  created_timestamp INTEGER NOT NULL, model TEXT, input_tokens INTEGER, output_tokens INTEGER,
  total_tokens INTEGER, cache_read_tokens INTEGER, cache_write_tokens INTEGER, cost REAL,
  cost_source TEXT, is_compaction INTEGER DEFAULT 0);
```

The same transaction creates `provider_inventory_entries` and `provider_inventory_models`, the
providers' model lists, which hold no conversation
([`providers/inventory/mod.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/providers/inventory/mod.rs#L1217-L1253)).
A database migrated from schema 9 or earlier also has a `thread_id` column and the tables
`threads` and `thread_messages`, which nothing writes ([Format versions](#format-versions)).

The `sessions` columns a reader uses:

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `id` | string | | |
| `name` | string | | |
| `description` | string | | |
| `user_set_name` | number | `0`, `1` | |
| `session_type` | string | `user`, `scheduled`, `sub_agent`, `hidden`, `terminal`, `gateway`, `acp` | |
| `working_dir` | string | | |
| `created_at` | string | | |
| `updated_at` | string | | |
| `extension_data` | string | | |
| `schedule_id` | string or null | | |
| `recipe_json` | string or null | | |
| `user_recipe_values_json` | string or null | | |
| `provider_name` | string or null | | |
| `model_config_json` | string or null | | |
| `goose_mode` | string | `auto`, `approve`, `smart_approve`, `chat` | |
| `archived_at` | string or null | | |
| `project_id` | string or null | | |
| `parent_session_id` | string or null | | |
| `system_prompt_override` | string or null | | |
| `system_prompt_extras_json` | string or null | | |
| `container_id` | string or null | | |
| `thread_id` | string or null | | |

The token columns (`total_tokens`, `input_tokens`, `output_tokens`, `cache_read_tokens`,
`cache_write_tokens`, their `accumulated_` forms and `accumulated_cost`) are numbers or null.
`session_type` is the `SessionType` enum in snake case
([`session_manager.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/session/session_manager.rs#L48-L71));
`goose_mode` is `GooseMode`: `auto` runs every tool call, `approve` asks before every call,
`smart_approve` asks only for calls judged sensitive and `chat` makes no tool calls
([`goose-provider-types/src/goose_mode.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-provider-types/src/goose_mode.rs#L20-L32)).

The `messages` columns:

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `id` | number | | |
| `message_id` | string or null | | |
| `session_id` | string | | |
| `role` | string | `user`, `assistant` | |
| `content_json` | string | | |
| `created_timestamp` | number | | |
| `timestamp` | string | | |
| `tokens` | number or null | | |
| `metadata_json` | string or null | | |

`role` is written by `role_to_string`
([`session_manager.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/session/session_manager.rs#L844-L849)).
`content_json` is a JSON array of content blocks and `metadata_json` a JSON object, both below.

The `usage_ledger` columns, one row per provider call
([`session_manager.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/session/session_manager.rs#L1037-L1070)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `id` | number | | |
| `session_id` | string | | |
| `created_timestamp` | number | | |
| `model` | string or null | | |
| `input_tokens` | number or null | | |
| `output_tokens` | number or null | | |
| `total_tokens` | number or null | | |
| `cache_read_tokens` | number or null | | |
| `cache_write_tokens` | number or null | | |
| `cost` | number or null | | |
| `cost_source` | string or null | `provider_reported`, `estimated` | |
| `is_compaction` | number | `0`, `1` | |

#### Content blocks

Each element of `content_json` is tagged on `type`, in camel case
([`goose-provider-types/src/conversation/message.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-provider-types/src/conversation/message.rs#L315-L330)).
The text and image blocks are rmcp's
([`rmcp/src/model/content.rs`](https://github.com/modelcontextprotocol/rust-sdk/blob/9427a929959e665e0d12e9395f674026baf4bd48/crates/rmcp/src/model/content.rs#L20-L35),
[`62-73`](https://github.com/modelcontextprotocol/rust-sdk/blob/9427a929959e665e0d12e9395f674026baf4bd48/crates/rmcp/src/model/content.rs#L62-L73)).

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `type` | string | `text`, `image`, `document`, `toolRequest`, `toolResponse`, `toolConfirmationRequest`, `actionRequired`, `thinking`, `redactedThinking`, `systemNotification`, `error` | |
| `text` | string | | |
| `annotations` | object | | |
| `_meta` | object | | |
| `data` | string or object | | |
| `mimeType` | string | | |
| `name` | string | | |
| `id` | string | | |
| `toolCall` | object | | |
| `toolResult` | object | | |
| `metadata` | object | | |
| `toolName` | string | | |
| `arguments` | object | | |
| `prompt` | string or null | | |
| `thinking` | string | | |
| `signature` | string | | |
| `notificationType` | string | `thinkingMessage`, `progressMessage`, `inlineMessage`, `creditsExhausted` | |
| `msg` | string | | |
| `kind` | string | `authentication`, `contextLengthExceeded`, `creditsExhausted`, `other` | |
| `message` | string | | |

By block: `text` holds `text` and optional `annotations` and `_meta`; `image` holds `data` and
`mimeType`; `document` holds `data`, `mimeType` and an optional `name`; `toolRequest` holds
`id`, `toolCall`, an optional `metadata` (provider data) and an optional `_meta`
([`message.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-provider-types/src/conversation/message.rs#L131-L142));
`toolResponse` holds `id`, `toolResult` and an optional `metadata`
([`176-183`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-provider-types/src/conversation/message.rs#L176-L183));
`toolConfirmationRequest`, an older form of the confirmation, holds `id`, `toolName`,
`arguments` and `prompt`
([`186-192`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-provider-types/src/conversation/message.rs#L186-L192));
`actionRequired` holds `data`
([`226-229`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-provider-types/src/conversation/message.rs#L226-L229));
`thinking` holds `thinking` and `signature`, `redactedThinking` holds `data`
([`231-240`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-provider-types/src/conversation/message.rs#L231-L240));
`systemNotification` holds `notificationType`, `msg` and an optional `data`, and `error` holds
`kind` and `message`
([`242-287`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-provider-types/src/conversation/message.rs#L242-L287)).

A text block's `annotations` is rmcp's `Annotations`, whose `audience` lists the roles the block
is meant for, `user` or `assistant`
([`rmcp/src/model/annotated.rs`](https://github.com/modelcontextprotocol/rust-sdk/blob/9427a929959e665e0d12e9395f674026baf4bd48/crates/rmcp/src/model/annotated.rs#L17-L24);
[`model.rs`](https://github.com/modelcontextprotocol/rust-sdk/blob/9427a929959e665e0d12e9395f674026baf4bd48/crates/rmcp/src/model.rs#L2619-L2624)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `audience` | array | | |
| `priority` | number | | |
| `lastModified` | string | | |

`toolCall` and `toolResult` are a result wrapper: `{"status":"success","value":...}` or
`{"status":"error","error":"..."}`
([`goose-provider-types/src/conversation/tool_result_serde.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-provider-types/src/conversation/tool_result_serde.rs#L7-L26)).

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `status` | string | `success`, `error` | |
| `value` | object or array | | |
| `error` | string | | |

A `toolCall` value is rmcp's call
([`rmcp/src/model.rs`](https://github.com/modelcontextprotocol/rust-sdk/blob/9427a929959e665e0d12e9395f674026baf4bd48/crates/rmcp/src/model.rs#L4138-L4150)),
a `toolResult` value rmcp's result
([`model.rs`](https://github.com/modelcontextprotocol/rust-sdk/blob/9427a929959e665e0d12e9395f674026baf4bd48/crates/rmcp/src/model.rs#L3868-L3897));
a `toolResult.value` may also be a bare content array, the older form, which the reader still
accepts
([`tool_result_serde.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-provider-types/src/conversation/tool_result_serde.rs#L134-L170)).
`arguments` is the tool's own parameter object: the developer extension's `shell` tool, which
the model calls as `developer__shell`, takes `command` and an optional `timeout_secs`
([`platform_extensions/developer/shell.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/agents/platform_extensions/developer/shell.rs#L182-L189);
[`developer/mod.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/agents/platform_extensions/developer/mod.rs#L150)).
The result's `content` items are rmcp content blocks tagged on `type` in snake case: `text`,
`image`, `audio`, `resource` or `resource_link`
([`rmcp/src/model/content.rs`](https://github.com/modelcontextprotocol/rust-sdk/blob/9427a929959e665e0d12e9395f674026baf4bd48/crates/rmcp/src/model/content.rs#L252-L264)).

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `name` | string | | |
| `arguments` | object | | |
| `content` | array | | |
| `structuredContent` | object | | |
| `isError` | boolean | | |
| `resultType` | string | | |

`toolRequest._meta` keys Goose writes
([`message.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-provider-types/src/conversation/message.rs#L156-L173);
[`agents/state_machine/ops_tool_approval.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/agents/state_machine/ops_tool_approval.rs#L23)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `goose.executable` | boolean | | |
| `goose.external_dispatch` | boolean | | |
| `goose.toolSummary.title` | string | | |
| `goose.toolCall.providerIndex` | number | | |
| `goose.toolChain.summary` | object | | |

#### Tool confirmations and permission mode

`actionRequired.data` is tagged on `actionType` with four variants
([`message.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-provider-types/src/conversation/message.rs#L194-L220)):
`toolConfirmation`, the question put to the user before a tool runs; `toolConfirmationResponse`,
the answer; `elicitation`, an MCP server's request for input (MCP elicitation: a server asks the
user to fill in a form); and `elicitationResponse`. Only the `toolConfirmation` variant renames
its fields to camel case, so `requested_schema` and `user_data` keep snake case.

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `actionType` | string | `toolConfirmation`, `elicitation`, `elicitationResponse`, `toolConfirmationResponse` | |
| `id` | string | | |
| `toolName` | string | | |
| `arguments` | object | | |
| `prompt` | string or null | | |
| `permission` | string | `always_allow`, `allow_once`, `cancel`, `deny_once`, `always_deny` | |
| `message` | string | | |
| `requested_schema` | object | | |
| `user_data` | object | | |
| `action` | string | `accept`, `decline`, `cancel` | |

`permission` is the `Permission` enum in snake case
([`goose-provider-types/src/permission.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-provider-types/src/permission.rs#L3-L11)),
and `action` rmcp's `ElicitationAction` in lower case
([`rmcp/src/model.rs`](https://github.com/modelcontextprotocol/rust-sdk/blob/9427a929959e665e0d12e9395f674026baf4bd48/crates/rmcp/src/model.rs#L3654-L3664)).
The request is written as an `assistant` message visible to the user only (`userVisible` true,
`agentVisible` false)
([`ops_tool_approval.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/agents/state_machine/ops_tool_approval.rs#L107-L127)),
and the answer as a `user` message visible to neither, added the moment the user answers
([`agents/state_machine/tool_confirmation.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/agents/state_machine/tool_confirmation.rs#L108-L116)).
`id` in both is the `toolRequest.id` it governs. A gateway user answers by typing `approve`,
`approve always`, `deny` or `deny always` (or `yes`, `y`, `no`, `n`) in the chat, which becomes
the same record
([`gateway/handler.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/gateway/handler.rs#L215-L223)).

`goose.executable` in `toolRequest._meta` (the `tool_meta` field) is the approval step's
verdict: `false` for a call the permission check denied or held for confirmation, then the value
the answer allows (`allow_once` and `always_allow` give `true`)
([`ops_tool_approval.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/agents/state_machine/ops_tool_approval.rs#L57-L76),
[`103-110`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/agents/state_machine/ops_tool_approval.rs#L103-L110),
[`271-289`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/agents/state_machine/ops_tool_approval.rs#L271-L289)).
The verdict is patched into the stored `toolRequest` row in place, an `UPDATE` of its
`content_json`
([`session_manager.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/session/session_manager.rs#L2980-L3020)).
A call that runs with no question (mode `auto`, or a tool on an allow list) is not marked. A
denied, cancelled or refused call is answered with a `toolResponse` error holding the fixed text
that begins `The user has declined to run this tool.`
([`agents/tool_execution.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/agents/tool_execution.rs#L151-L153);
[`ops_toolcalling.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/agents/state_machine/ops_toolcalling.rs#L955-L965)).

`sessions.goose_mode` is rewritten in place on a mode change
([`session_manager.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/session/session_manager.rs#L1949),
[`2023-2025`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/session/session_manager.rs#L2023-L2025)),
so only the last mode survives. A scheduled job's session is created in `auto`
([`scheduler/full.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/scheduler/full.rs#L878-L886)),
so its tool calls ran without any confirmation.

#### Message metadata

`metadata_json` is `MessageMetadata` in camel case
([`message.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-provider-types/src/conversation/message.rs#L827-L860)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `userVisible` | boolean | | |
| `agentVisible` | boolean | | |
| `inference` | object | | |
| `outputTokenLimitReached` | boolean | | |
| `steer` | boolean | | |
| `turnContext` | boolean | | |
| `compactionSummary` | boolean | | |
| `usage` | object | | |
| `operationLogs` | array | | |
| `operations` | object | | |

`inference` records the model of an assistant message
([`message.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-provider-types/src/conversation/message.rs#L766-L775)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `provider` | string | | |
| `requestedModel` | string | | |
| `resolvedModel` | string | | |
| `providerSessionId` | string | | |

`usage` is the message's token use
([`message.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-provider-types/src/conversation/message.rs#L777-L800)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `inputTokens` | number | | |
| `outputTokens` | number | | |
| `totalTokens` | number | | |
| `cacheReadTokens` | number | | |
| `cacheWriteTokens` | number | | |
| `cost` | number | | |
| `costSource` | string | `provider_reported`, `estimated` | |
| `elapsedMs` | number | | |
| `timeToFirstTokenMs` | number | | |
| `isCompaction` | boolean | | |

The boolean flags are written only when true, `inference`, `usage` and `operations` only when
present and `operationLogs` only when not empty.

#### Model configuration

`sessions.model_config_json` is `ModelConfig`
([`goose-provider-types/src/model.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-provider-types/src/model.rs#L40-L60));
`context_limit` and `request_headers` are not serialised. The same object is the
`model_config` of a request log's first line, so `model_config.model_name` names the model of
that request.

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `model_name` | string | | |
| `temperature` | number or null | | |
| `max_tokens` | number or null | | |
| `toolshim` | boolean | | |
| `toolshim_model` | string or null | | |
| `request_params` | object | | |
| `reasoning` | boolean | | |
| `supports_vision` | boolean | | |

#### Legacy sessions

A legacy file `<YYYYMMDD_HHMMSS>.jsonl` is read once into the database when `schema_version` does
not exist, then left in place
([`session_manager.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/session/session_manager.rs#L1107-L1128),
[`1285-1398`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/session/session_manager.rs#L1285-L1398)).
Its first line is the session
([`legacy.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/session/legacy.rs#L62-L89)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `description` | string | | |
| `id` | string | | |
| `created_at` | string | | |
| `updated_at` | string | | |
| `extension_data` | object | | |
| `message_count` | number | | |
| `working_dir` | string | | |
| `total_tokens` | number or null | | |
| `input_tokens` | number or null | | |
| `output_tokens` | number or null | | |

Each following line is a message
([`legacy.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/session/legacy.rs#L91-L101);
[`message.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-provider-types/src/conversation/message.rs#L973-L981)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `id` | string or null | | |
| `role` | string | `user`, `assistant` | |
| `created` | number | | |
| `content` | array | | |
| `metadata` | object | | |

A missing first-line key is filled from the file (`id` from the name, the times from the file's
creation and modification times), and the token fields move under `usage`
([`legacy.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/session/legacy.rs#L66-L77);
[`session/import_formats/mod.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/session/import_formats/mod.rs#L171-L197)).
`goose session import` also loads a Goose JSON export or a Claude Code, Codex or Pi JSONL
transcript into the database
([`import_formats/mod.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/session/import_formats/mod.rs#L1-L13);
[`goose-cli/src/cli.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-cli/src/cli.rs#L582-L587)).

#### Request log lines

Each provider call writes a log whose first line is `{model_config, input}`, `input` being the
provider's own request body, then one `{data, usage}` line per message the provider yields and an
`{error}` line on failure
([`goose-provider-types/src/request_log.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-provider-types/src/request_log.rs#L66-L133)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `model_config` | object | | |
| `input` | object | | |
| `data` | object | | |
| `usage` | object or null | | |
| `error` | string | | |

`input` has the provider's request format. In the OpenAI chat format it is `{model, messages}`,
the system message first, and each message's `content` is a string when the message holds only
text, an array of parts when it holds anything else, and null beside tool calls
([`goose-provider-types/src/formats/openai.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-provider-types/src/formats/openai.rs#L203-L213),
[`486-505`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-provider-types/src/formats/openai.rs#L486-L505),
[`1772-1794`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-provider-types/src/formats/openai.rs#L1772-L1794)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `model` | string | | |
| `messages` | array | | |
| `role` | string | | |
| `content` | string or array or null | | |

`usage` is the provider usage
([`goose-provider-types/src/conversation/token_usage.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-provider-types/src/conversation/token_usage.rs#L102-L110)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `input_tokens` | number or null | | |
| `output_tokens` | number or null | | |
| `total_tokens` | number or null | | |
| `cache_read_input_tokens` | number or null | | |
| `cache_write_input_tokens` | number or null | | |

A `data` line is a message in the legacy message shape above. A streaming provider yields one
partial message per chunk, built with `Message::assistant()`, so each line carries the message's
`id`, `role` `assistant`, its own `created` and the chunk's content blocks
([`goose-provider-types/src/formats/anthropic.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-provider-types/src/formats/anthropic.rs#L1155-L1160)).

### Joins and timestamps

`messages.session_id` and `usage_ledger.session_id` equal `sessions.id`. A tool call pairs on
`toolRequest.id` (an `assistant` row) and `toolResponse.id` (a `user` row). The project path is
`sessions.working_dir`; there is no git branch.

- `sessions.id` is `<YYYYMMDD>_<n>`, the UTC date and the next number for that date
  ([`session_manager.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/session/session_manager.rs#L1809-L1825)).
  `messages.message_id` is the message's own id or `msg_<session_id>_<uuid>`
  ([`session_manager.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/session/session_manager.rs#L2172-L2175)).
- `messages.created_timestamp` is epoch seconds (`Message::user()` and `Message::assistant()` use
  `Utc::now().timestamp()`,
  [`message.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-provider-types/src/conversation/message.rs#L1043-L1062));
  a client may write milliseconds, which Goose normalises by dividing a value above
  `10_000_000_000` by 1000
  ([`session_manager.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/session/session_manager.rs#L35),
  [`851-864`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/session/session_manager.rs#L851-L864)).
  A message added to a session is stored with the later of its own time and the newest
  `created_timestamp` already stored for the session, so a reply prepared before a tool ran never
  sorts ahead of it
  ([`session_manager.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/session/session_manager.rs#L2156-L2170)).
  Messages are read back ordered by `created_timestamp`, then `id`
  ([`session_manager.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/session/session_manager.rs#L2124)).
- `messages.timestamp` and `sessions.created_at` are SQLite `CURRENT_TIMESTAMP` text,
  `YYYY-MM-DD HH:MM:SS` in UTC, and `sessions.updated_at` is `datetime('now')` in the same form
  ([`session_manager.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/session/session_manager.rs#L2189-L2192)).
  A legacy-imported session and `archived_at` bind chrono values, which sqlx writes as RFC 3339
  ([`session_manager.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/session/session_manager.rs#L1368-L1369);
  [`sqlx-sqlite/src/types/chrono.rs`](https://github.com/launchbadge/sqlx/blob/75bc0487eb661da811bb7a3c5d158f1bd463fef4/sqlx-sqlite/src/types/chrono.rs#L66-L70)).
- The model is recorded per session in `sessions.provider_name` and `sessions.model_config_json`,
  per assistant message in `metadata_json.inference`, and per provider call in
  `usage_ledger.model`.
- `usage_ledger.created_timestamp` is `strftime('%s','now')`, epoch seconds
  ([`session_manager.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/session/session_manager.rs#L1056)).

#### CLI history

CLI history (`history.txt`) joins `sessions.db` on text alone
([issue 122](https://github.com/seanthegeek/doubleagent/issues/122)). The CLI adds every input
line that is not blank to the rustyline history, slash commands included, before it decides what
the line is, and sends a prompt as the line with surrounding whitespace trimmed
([`goose-cli/src/session/input.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-cli/src/session/input.rs#L192-L208));
an editor-composed prompt is added the same way
([`input.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-cli/src/session/input.rs#L133-L135)).
The file is saved before the message is added to the session
([`session/mod.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-cli/src/session/mod.rs#L808-L815)),
so a prompt's history line, trimmed, equals the `text` block of its `messages` row. The line
records no time, session id or working directory, so the only join is the text, against any
session of the same user. A save writes `.local/state/goose/history.txt` and deletes
`.config/goose/history.txt`; the old file is read only while the new one is absent
([`session/mod.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-cli/src/session/mod.rs#L216-L236)),
so both exist only when the save or that deletion failed. No other code names `history.txt`
(the desktop app included), so the file holds CLI prompts only.

#### Gateway pairings

Gateway pairings join `sessions.db` on session id. `goose gateway start` runs a bot (Telegram
is the one platform implemented, through `https://api.telegram.org`) that relays each message
from a paired chat user into a Goose session and sends the reply back to the chat
([`gateway/telegram.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/gateway/telegram.rs#L13);
[`gateway/mod.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/gateway/mod.rs#L95-L100)).
Pairing a chat user creates one session with `session_type` `gateway`, `name`
`<platform>/<display name or user id>` and `working_dir`
`.config/goose/gateway/<platform>/<user_id>`, then records the pairing under the
`gateway_pairings` key of `config.yaml`
([`gateway/handler.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/gateway/handler.rs#L303-L362);
[`gateway/pairing.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/gateway/pairing.rs#L11),
[`105-120`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/gateway/pairing.rs#L105-L120)).
Each list item
([`pairing.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/gateway/pairing.rs#L15-L21)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `platform` | string | | |
| `user_id` | string | | |
| `display_name` | string or null | | |
| `state` | object | | |

`state` is tagged on its own `state` key
([`gateway/mod.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/gateway/mod.rs#L59-L65)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `state` | string | `unpaired`, `pending_code`, `paired` | |
| `code` | string | | |
| `expires_at` | number | | |
| `session_id` | string | | |
| `paired_at` | number | | |

`paired_at` and `expires_at` are epoch seconds
([`handler.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/gateway/handler.rs#L352-L361)), and `state.session_id`
equals `sessions.id`. A pending pairing code is kept as the secret `gateway_pending_codes`
([`pairing.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/gateway/pairing.rs#L12-L13),
[`172-199`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/gateway/pairing.rs#L172-L199)).
A sample, with the platform id and name invented:

```yaml
gateway_pairings:
- platform: telegram
  user_id: '123456789'
  display_name: alice
  state:
    state: paired
    session_id: 20261017_4
    paired_at: 1792224000
```

#### Request logs

Request logs join `sessions.db` on text and tool call id; their lines are under
[Request log lines](#request-log-lines). `Conversation::push` assembles consecutive messages
with the same `id` into one: a one-block text chunk extends a last text block of the same
audience, a one-block thinking chunk extends a last thinking block that is unsigned or has its
signature, and anything else is appended; the message keeps its first chunk's `created`
([`goose-provider-types/src/conversation.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-provider-types/src/conversation.rs#L75-L119)).
The stored message's `created_timestamp` is never earlier than the log's and can be later by as
long as the tool calls before it ran ([Joins and timestamps](#joins-and-timestamps)). The log
names no session and no working directory, so the join is the assembled text, or the tool call
id, against any session of the same user. A provider call that yields no message id leaves every
chunk its own line and its own time. The logs keep the last 10 calls
([What survives a delete](#what-survives-a-delete)), so a removed, cleared or rewound session's
latest requests and replies may survive there alone.

#### Desktop prompt history

The desktop prompt history joins `sessions.db` on text and time. Each prompt the desktop app
sends is added to the local storage key `goose-chat-history`, a JSON array of
`{content, timestamp}` with `timestamp` in epoch milliseconds, unless it repeats the last entry;
it keeps the last 500 entries of 30 days
([`ui/desktop/src/utils/localMessageStorage.ts`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/ui/desktop/src/utils/localMessageStorage.ts#L1-L67);
[`components/ChatInput.tsx`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/ui/desktop/src/components/ChatInput.tsx#L435)).
It records no session id or working directory.

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `content` | string | | |
| `timestamp` | number | | |

### Other stores

#### Permission files

Standing grants:

| File | Shape | Meaning |
| --- | --- | --- |
| `.config/goose/permission.yaml` (`AppData/Roaming/Block/goose/config/permission.yaml` on Windows), with `permission.yaml.lock` beside it | a YAML map from category to `{always_allow: [], ask_before: [], never_allow: []}`, each a list of tool names; the categories are `user` (the user's choices) and `smart_approve` (the tools the `smart_approve` mode asks about, such as tools an MCP server marks as not read-only) ([`config/permission.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/config/permission.rs#L18-L44), [`91-106`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/config/permission.rs#L91-L106), [`169-195`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/config/permission.rs#L169-L195)) | an `always_allow` answer adds the tool to `user.always_allow`, an `always_deny` answer to `user.never_allow` ([`ops_tool_approval.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/agents/state_machine/ops_tool_approval.rs#L57-L71)); `goose configure` sets any level by hand. The file keeps no time and no session, so its modification time is the last change |
| `.config/goose/roaming_trust.json` | JSON `{"allowed": [<peer key>], "revoked_keys": [<peer key>]}` ([`goose-roaming/src/trust.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-roaming/src/trust.rs#L19-L26)) | the peer keys allowed to drive this host's agent over roaming, each with "goose's full ACP surface" ([`trust.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-roaming/src/trust.rs#L1-L12)) |
| `gateway_configs[].allowed_user_ids` in `.config/goose/config.yaml` | list of chat user ids | the chat users allowed to pair with the gateway ([`gateway/manager.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/gateway/manager.rs#L15-L28); [`gateway/handler.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/gateway/handler.rs#L808-L827)) |

`permission.yaml` is rewritten whole on every change by `serde_yaml::to_string` through a
temporary file
([`config/permission.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/config/permission.rs#L257-L275)),
so each category is a line `<category>:` at the left margin, each level an indented `<level>:`
line, or `<level>: []` when its list is empty, and each tool a block list item `- <tool>` below
its level, quoted only where YAML needs it.

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `user` | object | | |
| `smart_approve` | object | | |
| `always_allow` | array | | |
| `ask_before` | array | | |
| `never_allow` | array | | |

`roaming_trust.json` is pretty-printed JSON written through `roaming_trust.json.tmp-<pid>`
([`trust.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-roaming/src/trust.rs#L78-L86)); its entries are public peer
keys, not secrets.

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `allowed` | array | | |
| `revoked_keys` | array | | |

`permissions/tool_permissions.json` under the config directory, a per-call store of records
`{tool_name, allowed, context_hash, readable_context, timestamp, expiry}` keyed
`<tool>:<hash of the arguments>`
([`permission/permission_store.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/permission/permission_store.rs#L11-L20),
[`41-47`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/permission/permission_store.rs#L41-L47),
[`82`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/permission/permission_store.rs#L82)),
has no writer: `ToolPermissionStore` is only re-exported
([`permission/mod.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/permission/mod.rs#L10)),
so the file exists only where an older release wrote it.

#### Scheduler and project files

`<data>/schedule.json` is a JSON list of scheduled jobs, written pretty-printed whole on every
change
([`scheduler/full.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/scheduler/full.rs#L75-L87);
[`scheduler/common.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/scheduler/common.rs#L166-L187)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `id` | string | | |
| `source` | string | | |
| `cron` | string | | |
| `last_run` | string or null | | |
| `currently_running` | boolean | | |
| `paused` | boolean | | |
| `current_session_id` | string or null | | |
| `process_start_time` | string or null | | |
| `parameters` | array | | |
| `recipe_base_dir` | string or null | | |

`last_run` and `process_start_time` are RFC 3339 times, `source` is the job's recipe copy under
`scheduled_recipes/` (written mode 0600,
[`common.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/scheduler/common.rs#L57-L91)), and `parameters` is a list of
`[name, value]` pairs. A job's sessions have `session_type` `scheduled`, the name
`Scheduled job: <id>` and `sessions.schedule_id` set to the job id
([`full.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/scheduler/full.rs#L878-L886),
[`972`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/scheduler/full.rs#L972)).

`<data>/projects/<slug>.md` is a Markdown file whose YAML front matter holds `name`,
`description` and further properties such as the `workingDirs` list; it groups working
directories under a project name and holds no session content
([`sources.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/sources.rs#L60-L67),
[`92-123`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/sources.rs#L92-L123),
[`720-743`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/sources.rs#L720-L743)).

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `name` | string | | |
| `description` | string | | |
| `workingDirs` | array | | |

Neither file repeats a transcript.

#### Roaming directory

`roaming_directory.json` in the state directory is a pretty-printed list of the peers this host
dealt with
([`goose-roaming/src/directory.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-roaming/src/directory.rs#L17-L42),
[`95`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-roaming/src/directory.rs#L95)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `endpoint_id` | string | | |
| `label` | string or null | | |
| `direction` | string | `inbound`, `outbound` | |
| `agent_id` | string or null | | |
| `first_seen_ms` | number | | |
| `last_seen_ms` | number | | |
| `connected` | boolean | | |

It names no session id. `roaming_peers.json` in the config directory maps a nickname to a saved
peer's connection card, endpoint id, fingerprint and `added_ms`
([`goose-roaming/src/peerbook.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-roaming/src/peerbook.rs#L20-L38)).

### Format versions

- Schema 16 (version 1.53.0, block/goose
  [`591edd4`](https://github.com/block/goose/blob/591edd47cf2cfea4957d720c607cf2a4def8673d/crates/goose/src/session/session_manager.rs#L28))
  lacks the `sessions` columns `system_prompt_override`, `system_prompt_extras_json` and
  `container_id`, which migration 17 adds (version 1.55.0,
  [`bfcc6ee`](https://github.com/aaif-goose/goose/blob/bfcc6ee6f1ee18d0c4c852a65c49b25f87ee40dc/crates/goose/src/session/session_manager.rs#L32);
  [`b23d21c`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/session/session_manager.rs#L1750-L1771)).
- `CURRENT_SCHEMA_VERSION` is recorded in `schema_version`, and migrations 1 to 17 are additive
  `ALTER TABLE ADD COLUMN` steps
  ([`session_manager.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/session/session_manager.rs#L1459-L1777)),
  so an older database lacks, by migration: `user_recipe_values_json` (2), `metadata_json` (3),
  `name` and `user_set_name` (4), `session_type` (5), `provider_name` and `model_config_json`
  (6), `message_id` (7), `goose_mode` (8), `archived_at` and `project_id` (12),
  `accumulated_cost` (13), the cache token columns (14), `parent_session_id` and `usage_ledger`
  (15). Migration 9 renames sessions named `ACP Session` to type `acp`
  ([`1569-1581`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/session/session_manager.rs#L1569-L1581)).
- Migration 10 adds the `thread_id` column and the `threads` and `thread_messages` tables, which
  the create statement of a new database lacks and nothing writes (no `INSERT INTO threads` or
  `INSERT INTO thread_messages` in `crates/`), so only a database migrated from schema 9 or
  earlier has them
  ([`session_manager.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/session/session_manager.rs#L1582-L1634)).
  `threads` holds `id`, `name`, `user_set_name`, `working_dir`, `created_at`, `updated_at`,
  `archived_at` and `metadata_json`; `thread_messages` holds `id`, `thread_id`, `session_id`,
  `message_id`, `role`, `content_json`, `created_timestamp` and `metadata_json`.
- The legacy JSONL sessions predate the database ([Legacy sessions](#legacy-sessions)).
- `metadata_json.operationLogs` was added between 1.55.0
  ([`fb9e3c8`](https://github.com/aaif-goose/goose/commit/fb9e3c89b2b4670a0eded99e7567e578ac6e502c))
  and 1.57.0, and `metadata_json.compactionSummary` between the 1.57.0 tag and
  [`b23d21c`](https://github.com/aaif-goose/goose/commit/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc),
  so a 1.57.0 release does not write `compactionSummary`; a summary written before the flag is
  known by its continuation text
  ([`context_mgmt/mod.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/context_mgmt/mod.rs#L76-L95)).
- `permissions/tool_permissions.json` had no writer at
  [`fb9e3c8`](https://github.com/aaif-goose/goose/commit/fb9e3c89b2b4670a0eded99e7567e578ac6e502c)
  either; the release that last wrote it is not determined.
- A pending gateway pairing code was a `gateway_pending_codes` list in `config.yaml` in older
  builds; current builds move it to the secret of the same name and revoke the old codes
  ([`gateway/pairing.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/gateway/pairing.rs#L122-L137)).

### Secrets

- `llm_request.*.jsonl` holds the entire provider request body (system prompt, all messages,
  tool outputs) and the responses, so anything the user pasted is there; headers and keys are
  not logged, since `ModelConfig.request_headers` is not serialised
  ([`request_log.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-provider-types/src/request_log.rs#L81-L86),
  [`110-113`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-provider-types/src/request_log.rs#L110-L113);
  [`model.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-provider-types/src/model.rs#L56-L59)).
- In `sessions.db`, `recipe_json` can hold a recipe's `extensions` with their `envs` and
  `headers`, and `user_recipe_values_json` the parameters the user entered
  ([`recipe/mod.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/recipe/mod.rs#L43-L84)).
- Recipe files `.config/goose/recipes/*`, `.local/share/goose/scheduled_recipes/*`,
  `AppData/Roaming/Block/goose/config/recipes/*` and
  `AppData/Roaming/Block/goose/data/scheduled_recipes/*` can hold the same extension entries, and
  `schedule.json` `parameters` the values the user entered.
- The keyring entry or `secrets.yaml` ([Credentials](#credentials)) holds, besides provider keys,
  `gateway_platform_config_<type>` (the gateway's platform configuration with the Telegram bot
  token,
  [`gateway/manager.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/gateway/manager.rs#L17-L19)),
  `gateway_pending_codes`, and `oauth_creds_<name>`, an MCP server's OAuth grant
  ([`oauth/persist.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/oauth/persist.rs#L29-L31)).
- `config.yaml` keeps only `{gateway_type, max_sessions, allowed_user_ids}` per gateway under
  `gateway_configs`
  ([`gateway/manager.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/gateway/manager.rs#L21-L28)), and the
  `gateway_pairings` relay marker ([Gateway pairings](#gateway-pairings)); `config.yaml` is a
  credential file, so a `--no-secrets` collection loses both.
- The Langfuse keys (`LANGFUSE_PUBLIC_KEY`, `LANGFUSE_SECRET_KEY`) are read from the environment
  only
  ([`tracing/langfuse_layer.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/tracing/langfuse_layer.rs#L154-L163)).
- The desktop prompt history holds the user's prompts as typed.

### Parser plan

Globs: `.local/share/goose/sessions/sessions.db*`, `.local/share/goose/sessions/*.jsonl`,
`.local/state/goose/logs/llm_request.*.jsonl`, `.local/state/goose/history.txt`,
`.config/goose/history.txt`, `AppData/Roaming/Block/goose/data/sessions/sessions.db*`,
`AppData/Roaming/Block/goose/data/logs/llm_request.*.jsonl`,
`AppData/Roaming/Block/goose/data/history.txt`, and for the standing and scheduled records
`.config/goose/permission.yaml`, `.config/goose/roaming_trust.json`,
`.local/share/goose/schedule.json` and their Windows forms
`AppData/Roaming/Block/goose/config/permission.yaml`,
`AppData/Roaming/Block/goose/config/roaming_trust.json` and
`AppData/Roaming/Block/goose/data/schedule.json`.

Rows: one per `content_json` block, plus one `system` start row per session from `sessions`,
carrying `archived=` for `archived_at` and `schedule=` for `schedule_id`.

| Column | Source |
| --- | --- |
| timestamp_utc | `created_timestamp` (divided by 1000 when above `10_000_000_000`) |
| session_id | `messages.session_id` |
| project_path | `sessions.working_dir` |
| git_branch | empty |
| turn_type | role `user` → user; `assistant` text → assistant; `toolRequest` → tool_use; `toolResponse` → tool_result; `thinking`/`redactedThinking` → thinking; `systemNotification`/`error`/`actionRequired` → system |
| model | `metadata_json.inference.requestedModel`, else `model_config_json.model_name` prefixed with `provider_name/` |
| tool_name | `toolCall.value.name` (responses look the name up by id) |
| tool_use_id | `toolRequest.id` / `toolResponse.id` |
| text | text, `arguments` JSON, result text |

Not read: `usage_ledger`, `provider_inventory_entries`, `provider_inventory_models`, `threads`,
`thread_messages`, `config.yaml`, `secrets.yaml`, `projects/<slug>.md`,
`roaming_directory.json`, `roaming_peers.json`, `recent-dirs.json`, `goose-chat-history`,
`diagnostics_<session_id>.json` and `permissions/tool_permissions.json`.

#### Permission rows

All `system`, from `sessions.db`
([issue 96](https://github.com/seanthegeek/doubleagent/issues/96),
[issue 97](https://github.com/seanthegeek/doubleagent/issues/97),
[issue 209](https://github.com/seanthegeek/doubleagent/issues/209);
[Tool confirmations and permission mode](#tool-confirmations-and-permission-mode)):

- `permission mode: <goose_mode>` after each session's start row; `goose_mode` is one value per
  session row (default `auto`), so there is no change row.
- `permission request: <toolName> | <prompt> | <arguments>` for each `actionRequired` block of
  `actionType` `toolConfirmation`, an empty prompt left out, at the message's
  `created_timestamp`, with `tool_name` the block's `toolName` and `tool_use_id` its `id`.
- `permission decision: <permission> (toolConfirmationResponse)` for each `actionRequired`
  block of `actionType` `toolConfirmationResponse`, `tool_use_id` the block's `id` and
  `tool_name` looked up from the `toolRequest` with that id.
- `permission decision: not executable (goose.executable=false)` after the `tool_use` row of a
  `toolRequest` whose `_meta` holds `goose.executable` `false` and that no
  `toolConfirmationResponse` of the same session answers (a pre-scan of the session's messages
  finds the answers), with the call's `tool_name` and `tool_use_id`. Only `sessions.db` is
  checked; the legacy JSONL sessions and the request logs never carry the key.
- An `elicitation` or `elicitationResponse` block keeps the generic
  `action required: <compact JSON>` row, and an older `toolConfirmationRequest` block its
  `confirmation requested:` row.

Standing ([Permission files](#permission-files)), at the file's modification time with an empty
`session_id`, `<file>` the path relative to the home:

- `permission setting: <category>.<level>=<tool> (from <file>, file mtime)` per tool in
  `permission.yaml`, read without a YAML dependency for the one nesting level `serde_yaml`
  writes; any other line is a bad line ([Bad-line rows](#bad-line-rows)).
- `permission setting: roaming_trust.<allowed or revoked_keys>=<key> (from <file>, file
  mtime)` per peer key in `roaming_trust.json`.
- `gateway_configs[].allowed_user_ids` stays unread with the rest of `config.yaml`, a credential
  file holding nested YAML.

#### Model rows

All `system`, from `sessions.db`
([issue 96](https://github.com/seanthegeek/doubleagent/issues/96) and
[issue 97](https://github.com/seanthegeek/doubleagent/issues/97)):

- `model: <model_config_json.model_name> provider=<provider_name>` after each session's start
  row, at `created_at`.
- `model: <old> -> <new> provider=<inference.provider>` before the rows of a message whose
  `metadata_json.inference.resolvedModel` (else `requestedModel`) differs from the session's
  current model, with ` requested=<requestedModel>` when the two differ. User messages carry no
  `inference` and change nothing.
- The legacy JSONL sessions record no model and the request log has no session id, so neither
  yields model rows.

#### Past-use rows and secondary sources

`sessions.db` is the primary store, and the legacy JSONL sessions are read as primary records
([issue 122](https://github.com/seanthegeek/doubleagent/issues/122)). The `llm_request` logs
are a secondary source
([issue 126](https://github.com/seanthegeek/doubleagent/issues/126)): their response messages
repeat the database's assistant messages, so `secondary()` is True for them and
`secondary_tolerance()` is 600 seconds, because the database stamps a message up to the tool
calls' run time later than the log ([Request logs](#request-logs)). The parser assembles a
streamed reply's chunks by message id before it makes rows, so the assembled text matches the
database row, and a tool call matches on its id; the `llm request` row, which no database row
repeats, is always kept. A log's first line gives one `system` row
`llm request | <N> messages | last user: <text>`, `<N>` the length of `input.messages` and
`<text>` the last `user` message's text, with the model from `model_config.model_name` (else
`input.model`) and the time of the first response line; each assembled response message gives
the rows of its content blocks, and an `{error}` line one `system` row `llm error: <error>`.
Both `history.txt` files are secondary sources: one `user` row per entry, with an empty
`timestamp_utc`, `session_id` and `project_path`, which `dedup.deduplicate` drops when a database
`user` row of the same user has the same text and keeps otherwise (a removed session's prompt, a
slash command). See [CLI history](#cli-history).

One `system` row per `schedule.json` job
([Scheduler and project files](#scheduler-and-project-files)), a configuration record, neither
primary nor secondary, at the file's modification time with an empty `session_id`:
`scheduled task: <id> <cron> paused=<true|false> running=<true|false> source=<source>
last_run=<last_run> current_session=<current_session_id> recipe_base_dir=<recipe_base_dir>
(from <file>, file mtime)`, with ` parameters=<compact JSON>` before `(from` when the job has
parameters and `-` for an absent value. The job's sessions name it in their start row's
`schedule=` and ran in mode `auto`. `projects/<slug>.md` holds no session content and needs no
row.

The desktop prompt history ([Desktop prompt history](#desktop-prompt-history)) repeats the
desktop app's user prompts with millisecond times and would be a secondary source; it lives in
the Electron local storage database, which no parser reads, so it is not parsed.

#### Sync, relay and origin rows

The one session that leaves the host with a local marker is a gateway session, whose whole
conversation is relayed through the Telegram bot and stays in that chat on Telegram's servers.
The marker is the `gateway_pairings` entry in `.config/goose/config.yaml`
(`AppData/Roaming/Block/goose/config/config.yaml` on Windows), written once when the chat user
pairs and removed on unpair or gateway removal, plus the session's own `session_type`
`gateway`, which survives the unpair ([Gateway pairings](#gateway-pairings)). A relayed chat is
a copy on the messaging platform, not a vendor sync, so the parser leaves `records_sync` off and
reads `sessions.db` rather than the pairing: one `system` row after each `gateway` session's
start row, at the session's `created_at`,
`session relayed: gateway <platform> user <user_id> display <display or -> (from sessions.db session_type gateway, created_at)`,
the platform and user id taken from `working_dir`
(`.config/goose/gateway/<platform>/<user_id>`), the display from `name`
(`<platform>/<display name or user id>`, `-` when it is the user id). The `gateway_pairings`
entry repeats the platform and user id and adds `paired_at`, but it is nested YAML inside
`config.yaml`, a credential file, so it is not read; the session row survives an unpair, which
deletes the entry. A session deleted with `goose session remove` leaves only its pairing entry,
naming a missing session id, and so yields no row. Nothing else uploads a session to a Goose or
Block account:

- `goose session export` writes Markdown, JSON, YAML or HTML to a local file or stdout and
  records nothing
  ([`goose-cli/src/cli.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-cli/src/cli.rs#L561-L580);
  [`commands/session.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-cli/src/commands/session.rs#L293-L322)).
  No share, sync, cloud, upload or publish command is in the CLI's command list, and there is
  no share link (no `shareUrl`, `share_url` or `BASE_URL_SHARE` in `crates/` or `ui/`).
- The `sessions` table has no remote, share or sync column ([Sessions database](#sessions-database));
  `container_id` names a local Docker container for extensions.
- Telemetry goes to PostHog (a product-analytics service) only after the user opts in
  (`GOOSE_TELEMETRY_ENABLED: true` in `config.yaml`; `GOOSE_TELEMETRY_OFF` forces it off) and
  carries counts, versions, provider, model and a pattern-redacted error message
  ([`posthog.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/posthog.rs#L15-L55),
  [`335-360`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/posthog.rs#L335-L360),
  [`530-536`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/posthog.rs#L530-L536));
  its only local file is `telemetry_installation.json`. The desktop app's own analytics send
  nothing
  ([`ui/desktop/src/utils/analytics.ts`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/ui/desktop/src/utils/analytics.ts#L1-L17)),
  and its bug-report button opens a GitHub issue form
  ([`components/ui/Diagnostics.tsx`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/ui/desktop/src/components/ui/Diagnostics.tsx#L185-L191)).
- Tracing uploads content with no local marker. Langfuse (an LLM tracing service, self-hosted or
  cloud) is enabled by `LANGFUSE_PUBLIC_KEY` and `LANGFUSE_SECRET_KEY` in the environment,
  default URL `http://localhost:3000`
  ([`langfuse_layer.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/tracing/langfuse_layer.rs#L13));
  OTLP (the OpenTelemetry protocol) export by `OTEL_EXPORTER_OTLP_ENDPOINT` or
  `otel_exporter_otlp_endpoint` in `config.yaml`
  ([`otel/otlp.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/otel/otlp.rs#L226-L266)).
  Both are off by default, and message text enters the spans only when
  `OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT` is `true`
  ([`agents/gen_ai_telemetry.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/agents/gen_ai_telemetry.rs#L9-L10)).
  An endpoint in `config.yaml` is the only trace on disk. No row is proposed.
- Roaming is peer-to-peer remote control, not an upload: `roam share` serves this host's agent
  over ACP through iroh relays (iroh is a peer-to-peer QUIC library) to peers whose keys were
  accepted, and the sessions they drive stay in this host's `sessions.db`; `roam connect`,
  `delegate` and `bridge` drive a peer's agent, whose sessions stay on the peer
  ([`goose-roaming/src/lib.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-roaming/src/lib.rs#L1-L24)).
  The local record is `roaming_directory.json` ([Roaming directory](#roaming-directory)), which
  names no session id, so it is not a sync marker; a session hosted for a peer has
  `session_type` `acp` unless the client names itself
  ([`acp/server/new_session.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/acp/server/new_session.rs#L238-L248)).

#### Scratch directory rows

No parser row is needed: the gateway working directory is classed by the catalog rule, and the
desktop app's default working directory is the home ([Scratch directories](#scratch-directories)).

#### Deletion rows

The agent records no delete, so there is no `session deleted:` row (what each delete removes is
under [What survives a delete](#what-survives-a-delete)); `sessions.archived_at` is already on
the session start row (`archived=`).

#### Bad-line rows

A `sessions.db` that is not a SQLite database is one row `parser: not a SQLite database`, and a
SQLite error while reading it one row `parser: SQLite error: <message>`. The legacy JSONL and the
request logs go through `iter_jsonl` and `error_rows`; `history.txt`
cannot fail to decode (each line is a row). A `permission.yaml` line that is not a category, a
level or a list item is one row,
`parser: unparseable line (not a category, level or list item): <line>`, and the entries before
and after it are still yielded; a `roaming_trust.json` or `schedule.json` that cannot be read,
is not JSON or is not the type Goose writes (an object, a list) is one `parser: <message>` row,
and a `schedule.json` job that is not an object is
`parser: unparseable record <N> (not a JSON object): <record>`. In `sessions.db`
([issue 183](https://github.com/seanthegeek/doubleagent/issues/183)) a `content_json`,
`metadata_json` or `model_config_json` that does not decode, or decodes to another type than
Goose writes (an array for `content_json`, an object for the other two), is one `system` row
through `error_rows`:
`parser: unparseable record <N> (<table>.<column>: <reason>): <stored value>`, the stored text in
full, `<N>` the `messages.id` or the `sessions` rowid (also `source_line`), with the session's
`session_id` and `working_dir` and an empty `timestamp_utc`. A NULL or empty `content_json` gives
the row with reason `empty` and no value. The rest of the session is still yielded: a bad
`model_config_json` row follows the session start row and the session has no `model:` row; a
bad `metadata_json` row precedes the message's content rows, which keep the session's model and
count as visible; a bad `content_json` row stands in for the message's rows.

#### Fixtures

Samples derived from the writers at
[`b23d21c`](https://github.com/aaif-goose/goose/commit/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc),
values invented.

A `sessions` row and its messages, as `create_session` and `add_message` write them
([`session_manager.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/session/session_manager.rs#L1809-L1840),
[`2156-2199`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/session/session_manager.rs#L2156-L2199)):

```sql
INSERT INTO schema_version(version) VALUES (17);
INSERT INTO sessions(id,name,user_set_name,session_type,working_dir,created_at,updated_at,extension_data,provider_name,model_config_json,goose_mode)
VALUES ('20261030_2','Rename helper',0,'user','/home/bob/work','2026-10-30 14:02:11','2026-10-30 14:03:40','{}','openai',
 '{"model_name":"gpt-5.2","temperature":null,"max_tokens":null,"toolshim":false,"toolshim_model":null}','approve');
INSERT INTO messages(message_id,session_id,role,content_json,created_timestamp,metadata_json) VALUES
('msg_20261030_2_5d0c','20261030_2','user','[{"type":"text","text":"rename util.py to helpers.py"}]',1793368931,'{"userVisible":true,"agentVisible":true}'),
('chatcmpl-77','20261030_2','assistant','[{"type":"toolRequest","id":"call_9","toolCall":{"status":"success","value":{"name":"developer__shell","arguments":{"command":"git mv util.py helpers.py"}}},"_meta":{"goose.executable":false}}]',1793368934,
 '{"userVisible":true,"agentVisible":true,"inference":{"provider":"openai","requestedModel":"gpt-5.2","resolvedModel":"gpt-5.2-2026-09-01"}}'),
('msg_20261030_2_77aa','20261030_2','assistant','[{"type":"actionRequired","data":{"actionType":"toolConfirmation","id":"call_9","toolName":"developer__shell","arguments":{"command":"git mv util.py helpers.py"},"prompt":null}}]',1793368934,'{"userVisible":true,"agentVisible":false}'),
('msg_20261030_2_91be','20261030_2','user','[{"type":"actionRequired","data":{"actionType":"toolConfirmationResponse","id":"call_9","permission":"deny_once"}}]',1793368940,'{"userVisible":false,"agentVisible":false}');
INSERT INTO usage_ledger(session_id,created_timestamp,model,input_tokens,output_tokens,total_tokens,cost_source,is_compaction) VALUES ('20261030_2',1793368934,'gpt-5.2',812,40,852,'estimated',0);
```

A legacy file `20250611_093000.jsonl`, as `legacy.rs` reads it:

```jsonl
{"description":"set up venv","working_dir":"/home/bob/old","created_at":"2025-06-11T09:30:00Z","updated_at":"2025-06-11T09:31:02Z","extension_data":{},"message_count":2,"total_tokens":120}
{"id":"m7","role":"user","created":1749634200,"content":[{"type":"text","text":"make a venv"}]}
{"id":"m8","role":"assistant","created":1749634206,"content":[{"type":"text","text":"Run python3 -m venv .venv"}]}
```

A request log with a streamed reply, as `start_log` and the handle's `write` lay it out
([`request_log.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-provider-types/src/request_log.rs#L66-L116)):

```jsonl
{"model_config":{"model_name":"claude-sonnet-4-5","temperature":null,"max_tokens":null,"toolshim":false,"toolshim_model":null},"input":{"model":"claude-sonnet-4-5","messages":[{"role":"user","content":"list the tests"}]}}
{"data":{"id":"msg_01Ab","role":"assistant","created":1793368950,"content":[{"type":"text","text":"There are "}],"metadata":{"userVisible":true,"agentVisible":true}},"usage":null}
{"data":{"id":"msg_01Ab","role":"assistant","created":1793368951,"content":[{"type":"text","text":"four tests."}],"metadata":{"userVisible":true,"agentVisible":true}},"usage":{"input_tokens":30,"output_tokens":6,"total_tokens":36,"cache_read_input_tokens":null,"cache_write_input_tokens":null}}
```

`permission.yaml`, as `serde_yaml::to_string` writes the map:

```yaml
smart_approve:
  always_allow: []
  ask_before:
  - developer__shell
  never_allow: []
user:
  always_allow:
  - developer__text_editor
  ask_before: []
  never_allow:
  - computercontroller__automation_script
```

`roaming_trust.json`, pretty-printed:

```json
{
  "allowed": [
    "3f9c0d7e5a1b2c4d6e8f0a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d"
  ],
  "revoked_keys": []
}
```

A `schedule.json` job:

```json
[
  {
    "id": "weekly-deps",
    "source": "/home/bob/.local/share/goose/scheduled_recipes/weekly-deps.yaml",
    "cron": "0 0 9 * * MON",
    "last_run": "2026-10-27T09:00:00.481233Z",
    "currently_running": false,
    "paused": true,
    "current_session_id": "20261027_1",
    "process_start_time": null,
    "parameters": [["branch", "main"]],
    "recipe_base_dir": "/home/bob/work/recipes"
  }
]
```

#### Source host rows

The agent records no source-host integration. The only GitHub features are the recipe
repository and the Copilot provider. `goose recipe` with `GOOSE_RECIPE_GITHUB_REPO` set (an
`owner/repo` value in `config.yaml`) shells out to `gh auth status`, `gh auth login`, `gh api`
and `gh repo clone`, so the GitHub account and its token are the GitHub CLI's, in its own
configuration outside every Goose directory, and the clone lands in the OS temp directory
([Scratch directories](#scratch-directories))
([`goose-cli/src/recipes/github_recipe.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-cli/src/recipes/github_recipe.rs#L32),
[`104-126`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-cli/src/recipes/github_recipe.rs#L104-L126),
[`281-286`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose-cli/src/recipes/github_recipe.rs#L281-L286)).
The GitHub Copilot provider signs in to a GitHub account by the device flow to buy model
inference, not repository access, and caches its token in `githubcopilot/info.json`
([Credentials](#credentials);
[`providers/githubcopilot.rs`](https://github.com/aaif-goose/goose/blob/b23d21cabd9b4de0c045dcdd731821a3a15c2bfc/crates/goose/src/providers/githubcopilot.rs#L120-L145)).
An MCP server's OAuth grant (a GitHub MCP server's included) is the secret `oauth_creds_<name>`
([Secrets](#secrets)). No repository list is cached and no conversation is bound to a
repository (`crates/` and `ui/desktop/src` searched for `github`, `gitlab`, `bitbucket`,
`oauth`, `installation`, `repositor`, `pull request` and `clone`), so the parser writes no
`source host:` row.

## Confidence

- High: the Rust paths and their derivation, the Windows mapping, the credential files, the
  `sessions.db` statement, the content and metadata keys, WAL, the migrations.
- Medium: the Electron `userData` names (Electron's convention from `productName`, no explicit
  `setPath`) and the place of the `persist:goose` partition's local storage under it
  (`Partitions/goose/Local Storage/leveldb` by Chromium's layout, not stated in goose source).
- Medium: that the stored assistant message is the one `Conversation::push` assembles from the
  logged chunks; the path from the provider stream to `add_message` was not traced, so the
  request-log join rests on the shared assembly rule.
- Not determined: whether a desktop client writes millisecond `created_timestamp` values (only
  the normaliser shows some client does); where the local-inference model downloads land, which
  the `hf_hub` client decides; the release that last wrote `permissions/tool_permissions.json`.
- For the round: the credential files a real install writes per platform and whether
  `secrets.yaml` appears when the keyring works; the gateway session and its `gateway_pairings`
  entry; a tool confirmation's three records and the in-place `goose.executable` patch; the
  request logs' chunk lines against the stored reply; the desktop prompt history's location and
  shape; the computer controller's screenshots; the legacy macOS
  `Library/Application Support/Block/goose` directory on an upgraded host.
