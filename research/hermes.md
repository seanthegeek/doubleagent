# hermes

Hermes Agent (catalog agent `hermes`) is Nous Research's open-source Python agent, with a classic
CLI, a TUI (terminal interface), an Electron desktop app, the Webapp (the desktop workspace served
to a browser by `hermes webapp`), a messaging gateway that drives the agent from chat platforms,
cron jobs and delegated subagents; every front end stores every conversation in one SQLite store
per profile home, `state.db`, in OpenAI chat-completions message shape. It is not a fork of another
agent, but it imports Claude Code and Codex CLI sessions into its own store
([Imported sessions](#imported-sessions)), and Agent Zero's Orchestrator plugin can start it
([Started by another agent](#started-by-another-agent)). This document rests on the main branch at
commit
[`6bb377a`](https://github.com/NousResearch/hermes-agent/commit/6bb377a51d5c8e279a7a5e34d717d653d421d064)
of 2026-10-30, after release `v2026.9.24`. Parsed: the hermes parser reads `state.db`, its quick
snapshots, the divert JSONL sessions, `pastes/pending.json`, the prompt history, the cron run
archive, `response_store.db`, the trajectory files, the standing approvals in `config.yaml` and the
pairing stores, and the desktop app's `connections.json`.

## Source and evidence

[NousResearch/hermes-agent](https://github.com/NousResearch/hermes-agent), MIT
([`LICENSE`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/LICENSE#L1-L3)), open source, Python with an Electron desktop app, commit
[`6bb377a51d5c8e279a7a5e34d717d653d421d064`](https://github.com/NousResearch/hermes-agent/commit/6bb377a51d5c8e279a7a5e34d717d653d421d064)
(2026-10-30). The repository carries no version in `pyproject.toml` (`0.0.0`); the release the
commit follows is the latest tag, `v2026.9.24`, which `hermes_cli/__init__.py` names as
`__release_date__`
([`hermes_cli/__init__.py:5`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/__init__.py#L5)). Every claim in the Collection and
Parsing parts is from source at that commit unless it cites another; the vendor's documentation
pages in `website/docs/` are cited at the same commit for product behaviour, and the claims about
older builds in [Format versions](#format-versions) cite
[`8b66a51`](https://github.com/NousResearch/hermes-agent/commit/8b66a51036c1e20920a17cdd049fdf55c968d683)
(2026-10-04) or
[`86fa9e6`](https://github.com/NousResearch/hermes-agent/commit/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872)
(2026-10-19). The prompt history format rests on prompt-toolkit/python-prompt-toolkit at
[`d8adbe9`](https://github.com/prompt-toolkit/python-prompt-toolkit/commit/d8adbe9bfcf5f7e95b015d8ab12e2985b9a7822b),
the desktop updater cache on electron-userland/electron-builder at
[`39db76f`](https://github.com/electron-userland/electron-builder/commit/39db76fc9034061769aaf87b02d020d60b00a254)
and the Orchestrator plugin on agent0ai/agent-zero at
[`ff2a5b9`](https://github.com/agent0ai/agent-zero/commit/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a).
Read only: nothing was installed or run.

### Observed

- No real install. What stands in is the source review of NousResearch/hermes-agent at
  [`6bb377a`](https://github.com/NousResearch/hermes-agent/commit/6bb377a51d5c8e279a7a5e34d717d653d421d064)
  (after `v2026.9.24`). Every row of the [Parser plan](#parser-plan) is source-derived; the
  `pastes/pending.json` rows, the model-switch marker rows, a malformed `tool_calls` value and a
  Hermes home in a Docker or Podman named volume rest on the source and the fixtures alone
  ([Debug-share paste list](#debug-share-paste-list),
  [Model-switch markers](#model-switch-markers), [Bad-line rows](#bad-line-rows),
  [Docker and Podman volumes](#docker-and-podman-volumes)).

### Source reviews

- 2026-10-30: NousResearch/hermes-agent at
  [`6bb377a`](https://github.com/NousResearch/hermes-agent/commit/6bb377a51d5c8e279a7a5e34d717d653d421d064),
  every citation re-read there. Added the Webapp and its credential directory, the opt-in vault key
  in the OS keychain, the config backend seam and the managed `/etc/hermes` scope, the
  `gui.log`, the browser uploads, the emergency, malformed and retired-WAL copies of `state.db`,
  the `session_meta` role, the local policy record that holds a classic worktree session's tree,
  the aging of sweep deletion markers and the key tables; recorded the CLI importer's change to
  plain `cli` sessions and the move of the Mem0 and Langfuse plugins to the plugin catalog.
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
  path was unchanged.

## Collection

### Per-user storage

Home resolution: context override, then `HERMES_HOME`, then the platform default
([`hermes_constants.py:194-201`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_constants.py#L194-L201)): `~/.hermes` on Linux and
macOS, `%LOCALAPPDATA%\hermes` (fallback `~/AppData/Local/hermes`) on Windows, each with an
optional `HERMES_DATA_DIR_SUFFIX` appended
([`hermes_constants.py:57-64`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_constants.py#L57-L64)). The suffix is appended with no
separator: a channel build's `-channel-build-canary` gives `~/.hermes-channel-build-canary`, and
an explicit `HERMES_HOME` is never suffixed
([`environment-variables.md:886`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/website/docs/reference/environment-variables.md#L886)), so a
suffixed home is `.hermes<suffix>` or `AppData/Local/hermes<suffix>`. The desktop app resolves the
same default and on Windows keeps using a legacy `~/.hermes` when `%LOCALAPPDATA%\hermes` does not
exist and no suffix is set
([`apps/desktop/electron/data-paths.mjs:26-33`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/apps/desktop/electron/data-paths.mjs#L26-L33),
[`57-64`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/apps/desktop/electron/data-paths.mjs#L57-L64)), so both forms occur on Windows. No XDG
paths.

A system-wide managed scope, `/etc/hermes/config.yaml`, is an overlay applied on top of each
profile's own `config.yaml`
([`hermes_cli/managed_scope.py:31`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/managed_scope.py#L31),
[`hermes_cli/config_backend.py:17-18`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/config_backend.py#L17-L18)); it is outside
every home. With `HERMES_CONFIG_BACKEND=remote` a profile's settings come from a remote
configuration service instead of its `config.yaml`
([`hermes_cli/config_backend.py:1-14`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/config_backend.py#L1-L14),
[`plugins/config_backends/remote/__init__.py:1-2`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/plugins/config_backends/remote/__init__.py#L1-L2)).

#### Scratch directories

`cache/scratch/<entry>` is the `TMPDIR` of every child process, reaped when idle
([`hermes_constants.py:978`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_constants.py#L978),
[`hermes_constants_scratch.py:1-6`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_constants_scratch.py#L1-L6)); it is under the
catalog's `.hermes` entry and collected. `sandboxes/` (or `TERMINAL_SANDBOX_DIR`) holds the
container backends' persistent bind mounts, `sandboxes/docker/<task>` for Docker
([`tools/environments/base.py:187-191`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/tools/environments/base.py#L187-L191),
[`tools/environments/docker.py:856`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/tools/environments/docker.py#L856)); it is excluded
([Exclusions](#exclusions)).

The git worktrees the agent creates are inside the user's repository, not the home:
`<repo>/.worktrees/<name>` on branch `hermes/<name>` for `hermes -w`, `.worktrees/subagent-<id>`
on `hermes-subagent/<name>` for delegated subagents, and `.worktrees/<child id>` for kanban
children
([`hermes_cli/worktree_ops.py:396-402`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/worktree_ops.py#L396-L402),
[`tools/subagent_worktree.py:81-96`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/tools/subagent_worktree.py#L81-L96),
[`hermes_cli/kanban_db_graph.py:181`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/kanban_db_graph.py#L181)). A `--tui -w`
session's `sessions.cwd` lies inside its tree
([`hermes_cli/worktree_ops.py:863-885`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/worktree_ops.py#L863-L885)); a classic `-w`
session's row holds the repository root, and its tree is the `cwd` of the session's local policy
record ([`hermes_cli/main_resume_args.py:28-38`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/main_resume_args.py#L28-L38);
[Local policy records](#local-policy-records)). A retained TUI worktree is locked with the reason
`hermes session=<id> db=<path>`
([`hermes_cli/worktree_ops.py:769-779`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/worktree_ops.py#L769-L779)). The startup
sweep archives an idle dirty or unpushed tree to the git ref `refs/hermes/archive/<tree>` and
removes the directory
([`hermes_cli/worktree_ops.py:1111-1122`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/worktree_ops.py#L1111-L1122)). The project
catalog collects the repository's `.hermes/` files, not `.worktrees/`.

#### Profiles

Named profiles are complete homes under `<root>/profiles/<name>/`
([`hermes_cli/profiles.py:288-298`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/profiles.py#L288-L298)); every path in this
section can also appear one level down under `profiles/*/`.

#### Docker and Podman volumes

The compose file bind-mounts the host `~/.hermes` to `/opt/data`, the container's `HERMES_HOME`
([`docker-compose.yml:36-37`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/docker-compose.yml#L36-L37),
[`Dockerfile:491`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/Dockerfile#L491)), so a compose deployment leaves its state in the host home.
The image declares `/opt/data` a volume
([`Dockerfile:540`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/Dockerfile#L540)), and the Docker guide suggests a named volume
`hermes-data` for it
([`docker.md:252`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/website/docs/user-guide/docker.md#L252)); the catalog's volume line
`hermes|*hermes*` matches it. Such a volume's `_data` directory is the Hermes home itself: its
files are `state.db`, `sessions/<id>.jsonl`, `state-snapshots/<id>/state.db`,
`pastes/pending.json` and `profiles/<name>/...` with no `.hermes/` prefix. The image's runtime
user `hermes` has `/opt/data` as its login home
([`Dockerfile:178`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/Dockerfile#L178)), so a credential a tool writes to its own home inside
the container (`.ssh/<file>`, `.config/gh/<file>`) lands in `_data` too. The home-relative
exclusions and secret globs are anchored to `.hermes*/`, so each has a volume-relative twin
matched against `_data`. The excluded subtrees ([Exclusions](#exclusions)) in a volume are
`hermes-agent/[!.]*`, `hermes-agent/.[!e]*`, `models`, `runtimes`, `node`, `installs`, `tools`,
`sandboxes`, `checkpoints/store/objects`, `checkpoints/store/indexes`, `checkpoints/legacy-*`,
`backups`, `browser-profile`, `browser-profiles`, `browser_profiles`, `chrome-debug`,
`bot-desktop/browser-profile`, `plugins/*/node_modules` and `plugins/*/.venv`. The credential
files ([Credentials](#credentials)) in a volume are flagged by `.env`, `.env.bak*`, `.op.env`,
`npmrc`, `auth.json`, `auth.json.*`, `auth/*`, `.anthropic_oauth.json`, `.copilot_jwt.json`,
`google_*.json`, `google_chat_user_tokens/*`, `slack_tokens.json`, `honcho.json`, `mem0.json`,
`webhook_subscriptions.json`, `teams_pipeline_store.json`, `mcp-tokens/*`, `vault/*`,
`browser_auth/*`, `bot-desktop/*`, `webapp/*`, `pairing/*`, `platforms/pairing/*`,
`feishu_comment_pairing.json`, `whatsapp/session/*`, `platforms/whatsapp/session/*`,
`matrix/store/*`, `platforms/matrix/store/*`, `cache/bws_cache*.json`,
`workspace/meetings/node_token.json`, `weixin/accounts/*`, `runtime/photon-sidecar.json`,
`proxy/*`, `home/*`, `config.yaml`, `config.yaml.*`, `.ssh/*`, `.aws/credentials`, `.aws/config`,
`.gnupg/*`, `.kube/*`, `.docker/*`, `.azure/*`, `.config/gh/*`, `.config/gcloud/*` and `.envrc`; the
AWS twin names its credential files instead, `.aws/credentials`, `.aws/config` and the AWS
CLI's assumed-role credential cache `.aws/cli/cache/*.json`
([Using an IAM role in the AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-role.html#cli-configure-role-cache)),
since a bare `.aws/*` would also flag Kiro's `.aws/amazonq/` transcripts in a home; the SSO
token cache `.aws/sso/cache/*.json`
([Configuring IAM Identity Center authentication with the AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-sso.html#cli-configure-sso-login))
is flagged by Kiro's line of the same form, and a file such as `.aws/amazonq/<file>` stays
unflagged. The credential copies inside a snapshot are flagged by
`state-snapshots/*/.env`, `state-snapshots/*/auth.json`, `state-snapshots/*/config.yaml`,
`state-snapshots/*/feishu_comment_pairing.json`, `state-snapshots/*/pairing/*` and
`state-snapshots/*/platforms/pairing/*`.
A named profile in a volume, `profiles/<name>/`, is a whole Hermes home with its own quick snapshots
([`hermes_cli/backup.py:1127-1129`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/backup.py#L1127-L1129)),
so each credential glob above has a profile form, whose `*` also crosses into the profile's
`state-snapshots/<id>/`: `profiles/*/.env`, `profiles/*/.env.bak*`, `profiles/*/.op.env`,
`profiles/*/npmrc`, `profiles/*/auth.json`, `profiles/*/auth.json.*`, `profiles/*/auth/*`,
`profiles/*/.anthropic_oauth.json`, `profiles/*/.copilot_jwt.json`, `profiles/*/google_*.json`,
`profiles/*/google_chat_user_tokens/*`, `profiles/*/slack_tokens.json`, `profiles/*/honcho.json`,
`profiles/*/mem0.json`, `profiles/*/webhook_subscriptions.json`,
`profiles/*/teams_pipeline_store.json`, `profiles/*/mcp-tokens/*`, `profiles/*/vault/*`,
`profiles/*/browser_auth/*`, `profiles/*/pairing/*`, `profiles/*/platforms/pairing/*`,
`profiles/*/whatsapp/session/*`, `profiles/*/platforms/whatsapp/session/*`,
`profiles/*/matrix/store/*`, `profiles/*/platforms/matrix/store/*`,
`profiles/*/cache/bws_cache*.json`, `profiles/*/weixin/accounts/*`,
`profiles/*/runtime/photon-sidecar.json`, `profiles/*/proxy/*`, `profiles/*/home/*`,
`profiles/*/config.yaml`, `profiles/*/config.yaml.*`, `profiles/*/bot-desktop/*`,
`profiles/*/webapp/*`, `profiles/*/feishu_comment_pairing.json`,
`profiles/*/workspace/meetings/node_token.json`, `profiles/*/.ssh/*`, `profiles/*/.aws/credentials`,
`profiles/*/.aws/config`, `profiles/*/.aws/sso/cache/*.json`, `profiles/*/.aws/cli/cache/*.json`,
`profiles/*/.gnupg/*`, `profiles/*/.kube/*`, `profiles/*/.docker/*`,
`profiles/*/.azure/*`, `profiles/*/.config/gh/*`, `profiles/*/.config/gcloud/*` and
`profiles/*/.envrc`. OpenHands' volume-relative `profiles/*.json` also flags a named profile's
`.json` files, such as `profiles/<name>/pastes/pending.json`, since its `*` crosses `/`, so
`--no-secrets` skips them; `state.db` and the `sessions/<id>.jsonl` transcripts are unaffected.

#### Contents of the home

Relative to the home:

- `state.db` with `state.db-wal` and `state.db-shm`: the session and message store, canonical for CLI, TUI, gateway,
  desktop and Webapp ([`hermes_state.py:181`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state.py#L181)).
- `sessions/`: `sessions.json`, the legacy mirror of the gateway routing index whose primary copy
  is the `gateway_routing` table of `state.db`
  ([`gateway/session_persistence.py:26-30`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/gateway/session_persistence.py#L26-L30),
  [`598-600`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/gateway/session_persistence.py#L598-L600)); the divert transcripts
  `<id>.jsonl`, the legacy snapshots and the API error dumps
  ([Transcript stores](#transcript-stores)); and `saved/hermes_conversation_<ts>.{json,md,html}`
  from `/save`
  ([`hermes_cli/cli_session_mixin.py:183-199`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/cli_session_mixin.py#L183-L199)).
- `session-exports/`: default output of `hermes sessions export`
  ([`hermes_cli/sessions_cmd.py:98-100`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/sessions_cmd.py#L98-L100)).
- `config.yaml`, `.env`, `skills/`
  ([`hermes_constants.py:1269-1281`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_constants.py#L1269-L1281)), `SOUL.md` (agent
  identity); `memories/MEMORY.md` and `memories/USER.md`
  ([`tools/memory_tool.py:38-40`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/tools/memory_tool.py#L38-L40),
  [`tools/memory_tool_store.py:303`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/tools/memory_tool_store.py#L303)).
- `logs/agent.log`, `errors.log`, `gateway.log` and `gui.log`
  ([`hermes_logging.py:397-400`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_logging.py#L397-L400)).
- Other stores the quick snapshot treats as user data: `cron/jobs.json`, `cron/executions.db`,
  `gateway_state.json`, `channel_directory.json`, `channel_aliases.json`, `processes.json`,
  `gateway/discord_message_recovery.db`, `projects.db`, `response_store.db` (API-server
  conversation history and tool payloads), `memory_store.db`, `verification_evidence.db`,
  `kanban.db`, `kanban/boards`
  ([`hermes_cli/backup.py:1106-1121`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/backup.py#L1106-L1121)).
- `checkpoints/`: one shared shadow git store; `store/projects/<hash16>.json` records `workdir`
  ([`tools/checkpoint_manager.py:13-25`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/tools/checkpoint_manager.py#L13-L25),
  [`74`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/tools/checkpoint_manager.py#L74)).
- `state-snapshots/<id>/` (quick snapshots of `state.db`, `.env`, `auth.json` and the other
  stores) and `backups/` (zip backups of the home), described in
  [Transcript stores](#transcript-stores)
  ([`hermes_cli/backup.py:1106-1107`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/backup.py#L1106-L1107),
  [`1124-1129`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/backup.py#L1124-L1129),
  [`1157-1161`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/backup.py#L1157-L1161)).
- `state.db.pre-update-emergency-<time>-<pid>.bak`, `state.db.malformed-backup-<time>` and
  `state.db.retired-wal-<time>-<pid>/`: older copies of `state.db` beside it
  ([State database copies](#state-database-copies)).
- `cache/terminal/<artifact>` (spilled tool results, background-process logs) and `cache/scratch`
  ([Scratch directories](#scratch-directories))
  ([`configuration.md:242-258`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/website/docs/user-guide/configuration.md#L242-L258)).
- `uploads/web-<hex>-<name>`: files a browser attached to a chat
  ([`hermes_cli/staged_uploads.py:1-5`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/staged_uploads.py#L1-L5)).
- `plugins/`: user-installed plugins, `plugins/.install-metadata.json` among them
  ([`hermes_cli/plugins_discovery.py:210`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/plugins_discovery.py#L210),
  [`hermes_cli/plugins_cmd_git.py:34`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/plugins_cmd_git.py#L34)).
- `hermes-agent/`: the installer's git checkout and venv, the repository's own
  `hermes-agent/.env.example` template among its files
  ([`.env.example`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/.env.example#L1),
  [`scripts/install.sh:97`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/scripts/install.sh#L97),
  [`scripts/install.ps1:28-29`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/scripts/install.ps1#L28-L29)).
- `.hermes_history`: the prompt history of the CLI and the TUI
  ([Prompt history](#prompt-history)). Collected; the profile export leaves it out, with
  `state.db` and `response_store.db`
  ([`hermes_cli/profiles.py:246`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/profiles.py#L246)).
- `cron/output/<job id>/<YYYY-mm-dd_HH-MM-SS>.md` ([`cron/jobs.py:119`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/cron/jobs.py#L119)):
  one Markdown file per cron run ([Cron run archive](#cron-run-archive)). Collected.
- `pairing/<platform>-approved.json` and `-pending.json`: the messaging gateway's approved and
  pending users ([Credentials](#credentials),
  [Permission outcomes and settings](#permission-outcomes-and-settings)).
- `webapp/`: the Webapp's browser credential `session-token`, `native-sign-in-tokens.json` and
  `sign-in-registration.json`
  ([`gateway/webapp_listener.py:32`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/gateway/webapp_listener.py#L32),
  [`hermes_cli/webapp_sign_in.py:49`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/webapp_sign_in.py#L49),
  [`hermes_cli/webapp_access.py:32`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/webapp_access.py#L32)).

#### Started by another agent

At agent0ai/agent-zero
[`ff2a5b9`](https://github.com/agent0ai/agent-zero/commit/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a),
Agent Zero's Orchestrator plugin, which tells that agent how to drive other coding CLIs on the
host or in its container, defaults Hermes to `yolo: true` and runs
`hermes chat --quiet --source tool -q <task> --yolo`, which bypasses every dangerous-command
approval prompt for the run
([`hermes_cli/_parser.py:307-308`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/_parser.py#L307-L308);
[`plugins/_orchestrator/default_config.yaml:7-37`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/plugins/_orchestrator/default_config.yaml#L7-L37),
[`plugins/_orchestrator/skills/orchestrator/references/hermes.md:16`](https://github.com/agent0ai/agent-zero/blob/ff2a5b9c9c4e490ba38dd7c313e779510bb2071a/plugins/_orchestrator/skills/orchestrator/references/hermes.md#L16)).
Such a session is an ordinary session in this home's `state.db`, started by another agent with
approvals bypassed; the plugin's permission settings are in
[Permission rows in `agent-zero.md`](agent-zero.md#permission-rows).

#### Desktop app

Electron `productName` is `Hermes`
([`apps/desktop/package.json:3`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/apps/desktop/package.json#L3)), so its `userData` is the
[Electron default](https://www.electronjs.org/docs/latest/api/app#appgetpathname) `Hermes`
directory under the per-user application data folder: `Library/Application Support/Hermes` on
macOS, `AppData/Roaming/Hermes` on Windows and `.config/Hermes` on Linux. It is overridable by
`HERMES_DESKTOP_USER_DATA_DIR` and otherwise suffixed with `HERMES_DATA_DIR_SUFFIX` like the home
(`.config/Hermes<suffix>`, `Library/Application Support/Hermes<suffix>` and
`AppData/Roaming/Hermes<suffix>`,
[`data-paths.mjs:35-39`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/apps/desktop/electron/data-paths.mjs#L35-L39)). The app depends on
`electron-updater`
([`apps/desktop/package.json:126`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/apps/desktop/package.json#L126)), whose download cache is
named after the package name `hermes` plus `-updater`
([`package.json:2`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/apps/desktop/package.json#L2);
[`appInfo.ts:159-161`](https://github.com/electron-userland/electron-builder/blob/39db76fc9034061769aaf87b02d020d60b00a254/packages/app-builder-lib/src/appInfo.ts#L159-L161))
and placed in `%LOCALAPPDATA%` on Windows
([`AppAdapter.ts:60-72`](https://github.com/electron-userland/electron-builder/blob/39db76fc9034061769aaf87b02d020d60b00a254/packages/electron-updater/src/AppAdapter.ts#L60-L72)):
`AppData/Local/hermes-updater`, beside the home, holding downloaded installers.

The `userData` directory holds `active-profile.json`, `connection.json`, `connections.json`,
`desktop-installation.json`, `updates.json`, `window-state.json`, `backend-ownership.json` and
`managed-ssh-update-recovery.json`
([`apps/desktop/electron/main.ts:1248`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/apps/desktop/electron/main.ts#L1248),
[`1320-1330`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/apps/desktop/electron/main.ts#L1320-L1330)); conversations stay in `state.db`.
`connections.json` is the registry of the backends the app talks to
([Desktop connection registry](#desktop-connection-registry)); both connection files are
credentials ([Credentials](#credentials)).

### Credentials

Hermes keeps its own list of credential stores, `PROFILE_CREDENTIAL_PATHS`
([`hermes_cli/profiles.py:156-189`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/profiles.py#L156-L189)), relative to each
profile home. Each path below sits under every Hermes home form of
[Per-user storage](#per-user-storage) (`.hermes/`, `.hermes<suffix>/`, `AppData/Local/hermes/`,
`AppData/Local/hermes<suffix>/` and `profiles/<name>/` inside any of them), as
`.hermes/auth.json` does on Linux and macOS and the same name under `AppData/Local/hermes/` does
on Windows:

| Path | Holds |
| --- | --- |
| `auth.json` | OAuth provider credentials and the credential pool |
| `.env` | API keys, platform bot tokens |
| `config.yaml` | MCP server `env` values and `headers` (below) |
| `config.yaml.bak-<stamp>`, `config.yaml.bak.<n>`, `config.yaml.corrupt.<time>`, `config.yaml.bak-pre-migrate-<time>` (the files `config.yaml.*`) | the config writers' copies of `config.yaml`; the glob also takes any other `config.yaml.*` a user names, which Hermes' own list leaves out |
| `auth/google_oauth.json`, `google_token.json`, `google_client_secret.json`, `google_oauth_pending.json` | Google OAuth |
| `google_chat_user_tokens/`, `google_chat_user_client_secret.json`, `google_chat_user_oauth_pending.json`, `google_chat_user_token.json` (the files `google_chat_user_*.json`), `google_chat_user_oauth_pending/` | Google Chat user OAuth |
| `.op.env` | 1Password service-account token |
| `npmrc` | npm registry auth |
| `.anthropic_oauth.json` | Anthropic OAuth tokens |
| `slack_tokens.json` | Slack tokens |
| `honcho.json`, `mem0.json` | memory provider keys |
| `webhook_subscriptions.json`, `teams_pipeline_store.json` | webhook shared secrets |
| `mcp-tokens/` | MCP OAuth tokens |
| `vault/` | `vault.key` beside `vault.json.enc`, or the `vault.keyref` pointer |
| `browser-profile`, `browser_auth`, `bot-desktop/<file>`, `browser-profiles`, `browser_profiles`, `chrome-debug` | browser cookies and logins |
| `webapp/session-token`, `webapp/native-sign-in-tokens.json`, `webapp/sign-in-registration.json` | the Webapp browser credential and sign-in tokens |
| `pairing/`, `platforms/pairing/`, `feishu_comment_pairing.json` | gateway pairing stores |
| `whatsapp/session/<file>`, `platforms/whatsapp/session/<file>`, `matrix/store/<file>`, `platforms/matrix/store/<file>` | messaging sessions |
| `cache/bws_cache.json`, `cache/bws_cache.enc.json` (both `cache/bws_cache*.json`) | Bitwarden cache |
| `workspace/meetings/node_token.json` | Google Meet node secret |
| `weixin/accounts` | WeChat bot tokens |
| `.copilot_jwt.json` | exchanged Copilot API token |
| `runtime/photon-sidecar.json` | Photon sidecar token |
| `proxy/ca.key` beside `proxy/ca.crt`, and the proxy tokens | iron-proxy CA key and tokens |
| `home/<file>` | the subprocess `HOME` in containers: gh, git, ssh and npm credentials |
| `backups`, `state-snapshots` | copies of the stores above |
| `.ssh/<file>`, `.aws/<file>`, `.gnupg/<file>`, `.kube/<file>`, `.docker/<file>`, `.azure/<file>`, `.config/gh/<file>`, `.config/gcloud/<file>`, `.envrc` | OS credential directories inside a profile home |

The subprocess `HOME` is `home/` only in containers
([`hermes_constants.py:765-788`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_constants.py#L765-L788)), and the proxy's CA files are
`proxy/ca.crt` and `proxy/ca.key`
([`agent/proxy_sources/iron_proxy.py:149-151`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/agent/proxy_sources/iron_proxy.py#L149-L151),
[`262`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/agent/proxy_sources/iron_proxy.py#L262)); the Webapp's three files are named at
[`gateway/webapp_listener.py:32`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/gateway/webapp_listener.py#L32),
[`hermes_cli/webapp_sign_in.py:49`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/webapp_sign_in.py#L49) and
[`hermes_cli/webapp_access.py:32`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/webapp_access.py#L32); the Bot Mode desktop keeps
`bot-desktop/lease.json` and `bot-desktop/sandbox.json` beside its browser profile
([`tools/bot_desktop/lease.py:79`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/tools/bot_desktop/lease.py#L79),
[`tools/bot_desktop/sandbox_host.py:67`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/tools/bot_desktop/sandbox_host.py#L67)); the OS credential directories
come from
[`agent/file_safety.py:210`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/agent/file_safety.py#L210)
([`hermes_cli/profiles.py:153-154`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/profiles.py#L153-L154)). Copies named `auth.json.*`
and `.env.bak*`, like the `config.yaml` copies in the table, are also credential stores
([`hermes_cli/profiles.py:191-198`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/profiles.py#L191-L198)). The
Google Chat pending grants are `google_chat_user_oauth_pending.json` or
`google_chat_user_oauth_pending/<email>.json`
([`plugins/platforms/google_chat/oauth.py:10`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/plugins/platforms/google_chat/oauth.py#L10)).

`config.yaml` is a credential file: the documentation puts non-secret settings in `config.yaml`
and secrets in `.env`
([`configuration.md:62-63`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/website/docs/user-guide/configuration.md#L62-L63)), but an
`mcp_servers` entry takes `env` values and `headers` such as `Authorization: "Bearer ..."`
([`mcp.md:318-340`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/website/docs/user-guide/features/mcp.md#L318-L340)). The installer
checkout's own `hermes-agent/.env` is loaded as a fallback
([`cli.py:339-340`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/cli.py#L339-L340),
[`hermes_cli/env_loader.py:652-659`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/env_loader.py#L652-L659)), so it holds keys
inside an otherwise excluded tree. The pairing stores are credentials, since a
`<platform>-pending.json` file holds live pairing codes; they are also standing permission state
([Permission outcomes and settings](#permission-outcomes-and-settings)).

`hermes sessions export --format trace --upload` takes a Hugging Face token from `HF_TOKEN` or
three alternate variable names in the environment
([`agent/trace_upload.py:36`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/agent/trace_upload.py#L36),
[`agent/trace_upload.py:170-172`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/agent/trace_upload.py#L170-L172)), which the setup message
tells the user to put in `.env`. A GitHub token, when the user gives one, is `GITHUB_TOKEN` or
`GH_TOKEN` in `.env` (or `GITHUB_APP_ID`, `GITHUB_APP_PRIVATE_KEY_PATH` and
`GITHUB_APP_INSTALLATION_ID` for a GitHub App), used by the Skills Hub, plugin installs, the
update check and the webhook platform's `github_comment` delivery, with the gh CLI's own login
(`gh auth token`) as the fallback
([`tools/skills_hub_github.py:73-75`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/tools/skills_hub_github.py#L73-L75),
[`tools/skills_hub_github.py:135-149`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/tools/skills_hub_github.py#L135-L149),
[`hermes_cli/git_credentials.py:11-17`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/git_credentials.py#L11-L17),
[`hermes_cli/github_api.py:9-14`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/github_api.py#L9-L14),
[`gateway/platforms/webhook.py:73-74`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/gateway/platforms/webhook.py#L73-L74)).

The OS keychain holds a Hermes secret in one opt-in case: with `vault.key_storage: keychain` the
vault key is the keychain item of service `Hermes Agent vault key` (macOS Keychain, Windows
Credential Manager, Secret Service or KWallet), and `vault/` keeps only the pointer
`vault.keyref`
([`agent/vault_keychain.py:1-22`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/agent/vault_keychain.py#L1-L22)). Secret managers
(1Password, command sources) are pluggable.

The desktop app's `connection.json` and `connections.json` hold each remote gateway's token (and
extra gateway headers) as an envelope that Electron's `safeStorage` encrypts with the operating
system's key store, or as `{"encoding": "plain", "value": <token>}` when the app's secret storage
policy is off
([`apps/desktop/electron/main.ts:1320-1325`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/apps/desktop/electron/main.ts#L1320-L1325),
[`apps/desktop/electron/main.ts:8075-8080`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/apps/desktop/electron/main.ts#L8075-L8080),
[`apps/desktop/electron/main.ts:8596-8603`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/apps/desktop/electron/main.ts#L8596-L8603)). On
macOS they are `Library/Application Support/Hermes/connection.json` and
`Library/Application Support/Hermes/connections.json`, on Linux `.config/Hermes/connection.json`
and `.config/Hermes/connections.json`, and on Windows the same two names under
`AppData/Roaming/Hermes/`, each also under `Hermes<suffix>`. The
`connections.json.corrupt-<time>` copy the app keeps of an unreadable registry is a credential
file too
([`apps/desktop/electron/main.ts:8582`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/apps/desktop/electron/main.ts#L8582)).

`state.db` mixes credentials typed into commands with the transcript; the fields are named in
[Secrets](#secrets).

### Exclusions

Hermes' own backup exclusions are the best guide
([`hermes_cli/backup.py:77-97`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/backup.py#L77-L97),
[`99`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/backup.py#L99), [`104`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/backup.py#L104)): `hermes-agent` (code
checkout), `models`, `runtimes`, `node` (GGUF models, llama.cpp, Node, "tens to hundreds of GB",
[`hermes_constants.py:288-292`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_constants.py#L288-L292)), `installs` and `tools`
([`hermes_cli/home_data_layout.py:5-6`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/home_data_layout.py#L5-L6)), `checkpoints`
(shadow git), live Chromium profiles, `node_modules`, `.venv`, `site-packages`, `sandboxes`
([`tools/environments/base.py:191`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/tools/environments/base.py#L191)).

The catalog excludes, under every Hermes home: `models`, `runtimes`, `node`, `installs`, `tools`
and `sandboxes`; `backups`; the live browser profiles `browser-profile`, `browser-profiles`,
`browser_profiles`, `chrome-debug` and the Bot Mode desktop's `bot-desktop/browser-profile`
([`tools/bot_desktop/browser.py:28-31`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/tools/bot_desktop/browser.py#L28-L31)); a plugin's
dependency trees `plugins/<name>/node_modules` and `plugins/<name>/.venv`
([`hermes_cli/plugins_cmd_catalog.py:476`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/plugins_cmd_catalog.py#L476)); and the
shadow git store's `checkpoints/store/objects`, `checkpoints/store/indexes` and the archived
pre-v2 repositories `checkpoints/legacy-<timestamp>`
([`tools/checkpoint_manager.py:13-25`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/tools/checkpoint_manager.py#L13-L25)), which keeps
`checkpoints/store/projects/<hash16>.json` for discovery. The desktop app's `Cache`, `Code Cache`
and `GPUCache` are excluded in its `Hermes` user-data directory and the suffixed ones, and the
updater cache `AppData/Local/hermes-updater` ([Desktop app](#desktop-app)), which the
suffixed-home line `AppData/Local/hermes?*` reaches, is excluded.

`state-snapshots/` and `backups/` hold older copies of `state.db`, so they may be the only record
of a deleted or pruned session ([What survives a delete](#what-survives-a-delete)). The catalog
collects `state-snapshots/`, whose count is bounded ([Transcript stores](#transcript-stores)) and
whose databases the parser reads, and excludes the zip archives under `backups/`; use `--full`
when deletion is suspected and a zip backup may hold more. A snapshot also holds copies of `.env`,
`auth.json`, `config.yaml`, `feishu_comment_pairing.json` and the pairing stores, which the
home-relative secret globs (their `*` crosses `/`) and the volume-relative globs of
[Docker and Podman volumes](#docker-and-podman-volumes) flag. `cache/` is not excluded:
`cache/terminal` holds tool output. `sandboxes/` stays excluded: a container backend's persistent
workspace can hold whole repositories, and the commands that ran there are in `state.db`.

The installer checkout is excluded entry by entry rather than as one directory
(`hermes-agent/[!.]*` and `hermes-agent/.[!e]*`), because an exclusion wins over a secret glob:
pruning the whole tree would lose `hermes-agent/.env`, which is collected and flagged. The
exclusion and secret globs are anchored to `.hermes*/` and `AppData/Local/hermes*/`, so they cover
profiles and suffixed homes without matching unrelated paths.

### Project-local files

- `.hermes.md` or `HERMES.md`, nearest from cwd up to the git root
  ([`agent/prompt_builder.py:140-149`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/agent/prompt_builder.py#L140-L149)); also reads
  `AGENTS.override.md`, `AGENTS.md`, `agents.md`, `CLAUDE.md`, `claude.md`, `.cursorrules`,
  `.cursor/rules/*.mdc`
  ([`prompt_builder.py:1605`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/agent/prompt_builder.py#L1605),
  [`1620`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/agent/prompt_builder.py#L1620), [`1629-1635`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/agent/prompt_builder.py#L1629-L1635)).
- `.hermes/`: `skills/` ([`agent/skill_utils.py:513`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/agent/skill_utils.py#L513)), `plugins/`
  ([`hermes_cli/plugins_discovery.py:214`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/plugins_discovery.py#L214)),
  `plans/<date>-<slug>.md` ([`agent/plan_prompt.py:21`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/agent/plan_prompt.py#L21)),
  `environment.json`
  ([`agent/verify/environment.py:17`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/agent/verify/environment.py#L17)); `.agents/skills` is
  also read.
- `trajectory_samples.jsonl` and `failed_trajectories.jsonl` in the cwd when
  `--save-trajectories` is given
  ([`agent/trajectory.py:37-44`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/agent/trajectory.py#L37-L44),
  [`agent/legacy_cli.py:50`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/agent/legacy_cli.py#L50)). No per-project database.

### Where the project path is recorded

- `checkpoints/store/projects/<hash16>.json`, key `workdir`
  ([`tools/checkpoint_manager.py:617-621`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/tools/checkpoint_manager.py#L617-L621)).
  Grep-able JSON; only present when checkpoints are enabled.
- `state.db` table `sessions`, columns `cwd`, `git_repo_root`, `git_branch`
  ([`hermes_state_common.py:490-492`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_common.py#L490-L492)). SQLite only.
- `state.db` table `state_meta`, the `policy.cwd` of each `gateway.local_policy.v1:<session id>`
  value, the tree of a classic `-w` session ([Local policy records](#local-policy-records)).
  SQLite only.
- `projects.db` tables `projects.primary_path` and `project_folders.path`
  ([`hermes_cli/projects_db.py:29-49`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/projects_db.py#L29-L49)). SQLite only.
- Worktrees the agent creates are scratch directories inside the project
  ([Scratch directories](#scratch-directories)).

## Parsing

### Transcript stores

Globs relative to the user home:

- `.hermes/state.db` (+ `-wal`, `-shm`), and the same under `.hermes/profiles/*/` and
  `AppData/Local/hermes/` ([`hermes_state.py:181`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state.py#L181); home resolution in
  [Per-user storage](#per-user-storage)). One database holds all sessions; it is canonical for
  the gateway too
  ([`gateway/session_transcript.py:631-633`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/gateway/session_transcript.py#L631-L633)).
- `.hermes/sessions/<session_id>.jsonl`: written only when `state.db` was replaced under a running
  process; one JSON message dict per line
  ([`hermes_state.py:434-448`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state.py#L434-L448)). Hermes calls this a divert: the
  batch of messages a live agent or the gateway could not write because its database handle was
  replaced or quarantined is appended here instead
  ([`agent/session_persistence.py:466-471`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/agent/session_persistence.py#L466-L471),
  [`gateway/session_transcript.py:203-227`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/gateway/session_transcript.py#L203-L227)). An
  agent's line has the keys of a `messages` row in that order
  ([`agent/session_persistence.py:242-254`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/agent/session_persistence.py#L242-L254);
  [Messages table](#messages-table)); a gateway line is the pending transcript message as the
  gateway queued it, and a value that is not a dict is written as `{"content": <text>}`. No
  `session_id` field (it is the file name), no `cwd`, no model.
- `.hermes/state-snapshots/<id>/state.db`: quick snapshots. `/snapshot [label]`,
  `hermes backup --quick`, `hermes update` (label `pre-update`) and the profile repair (label
  `repair-profiles`) copy `state.db` and the other critical state files
  ([`hermes_cli/backup.py:1102-1121`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/backup.py#L1102-L1121)) into
  `state-snapshots/<id>/`
  ([`hermes_cli/backup.py:62`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/backup.py#L62),
  [`hermes_cli/cli_commands_mixin.py:591-593`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/cli_commands_mixin.py#L591-L593),
  [`hermes_cli/backup.py:2035-2038`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/backup.py#L2035-L2038),
  [`hermes_cli/update_cmd_maint.py:672-677`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/update_cmd_maint.py#L672-L677),
  [`hermes_cli/sessions_repair_profiles.py:530`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/sessions_repair_profiles.py#L530)).
  `<id>` is the UTC time `%Y%m%d-%H%M%S`, then `-<label>` when one was given, then `-2`, `-3` on a
  collision ([`hermes_cli/backup.py:1300-1307`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/backup.py#L1300-L1307)); the files
  are staged under `.<id>.<pid>.partial` and renamed into place
  ([`hermes_cli/backup.py:1308`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/backup.py#L1308),
  [`hermes_cli/backup.py:1457`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/backup.py#L1457)), with a `manifest.json`
  ([Snapshot manifest](#snapshot-manifest)). A database is copied with SQLite's backup API, so
  the copy is a consistent, complete database with no WAL of its own
  ([`hermes_cli/backup_sqlite.py:142-146`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/backup_sqlite.py#L142-L146),
  [`hermes_cli/backup_sqlite.py:186`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/backup_sqlite.py#L186)). By default 20
  snapshots are kept, older ones pruned, except when a database failed to copy or was skipped as
  too large ([`hermes_cli/backup.py:1124`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/backup.py#L1124),
  [`hermes_cli/backup.py:1459-1467`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/backup.py#L1459-L1467)). `hermes update` passes
  a count of 1 and a 1 GiB file-size cap, so an update prunes every older snapshot, manual ones
  included, unless a database failed to copy or was too large, and a larger `state.db` is missing
  from its snapshot
  ([`hermes_cli/update_cmd_maint.py:30-34`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/update_cmd_maint.py#L30-L34),
  [`hermes_cli/update_cmd_maint.py:676-677`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/update_cmd_maint.py#L676-L677)). The
  files copied are `state.db`, `config.yaml`, `.env`, `auth.json`, the cron, gateway, project,
  response, memory, verification and kanban stores and both pairing directories
  ([`hermes_cli/backup.py:1106-1121`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/backup.py#L1106-L1121)). Collected by default
  ([Exclusions](#exclusions)).
- `.hermes/backups/pre-update-*.zip` and `pre-migration-*.zip`: full zip backups of the home
  ([`hermes_cli/backup.py:1157-1161`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/backup.py#L1157-L1161),
  [`hermes_cli/backup.py:1213-1227`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/backup.py#L1213-L1227)); not parsed.
- `.hermes/sessions/<id>.json`, `session_<id>.json` (legacy snapshots) and
  `request_dump_<id>_<ts>.json` (redacted API request on error)
  ([`hermes_state_sessions.py:1630-1642`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_sessions.py#L1630-L1642),
  [`agent/agent_runtime_helpers.py:1589-1594`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/agent/agent_runtime_helpers.py#L1589-L1594)).
  Nothing writes the two legacy snapshot names; the only code that names them is
  `_remove_session_files`, which deletes them with a session (an evaluation script checks that
  the old `write_json_snapshots` setting no longer produces any,
  [`evals/session_snapshot_removal.py:12-29`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/evals/session_snapshot_removal.py#L12-L29)), so
  their shape cannot be read from this source and the parser does not read them.
- `.hermes/sessions/saved/hermes_conversation_<YYYYmmdd_HHMMSS>.json` (`{id, model, started_at,
  messages}`) from `/save`
  ([`hermes_cli/cli_session_mixin.py:750-756`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/cli_session_mixin.py#L750-L756),
  [`183-199`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/cli_session_mixin.py#L183-L199)).

`state.db` runs in WAL mode by default with a DELETE fallback on filesystems that refuse it
([`hermes_state_wal.py:254-262`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_wal.py#L254-L262)); read `state.db-wal` with
the main file. FTS5 tables `messages_fts` and `messages_fts_trigram`
([`hermes_state_common.py:953`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_common.py#L953),
[`1073`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_common.py#L1073)) are indexes; skip them. No compression or encryption.

Session ids are `<YYYYmmdd_HHMMSS>_<hex>` from local time
([`hermes_state_ids.py:56-60`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_ids.py#L56-L60)). A session created through the
unified runtime (Hermes' single owner of session creation for the local front ends and cron) is
instead `local-` followed by the SHA-256 of its profile, principal and request ids, and a cron
run's request id is `cron:<job>:<execution>`; earlier runs were `cron_<job>_<YYYYmmdd_HHMMSS>`
([`hermes_state_keys.py:11-17`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_keys.py#L11-L17),
[`hermes_state_keys.py:71-74`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_keys.py#L71-L74)). The parser treats the id as an
opaque string, so both forms work.

#### What survives a delete

`hermes sessions delete` and the terminal session browser remove the session, its delegate
children, their `messages` rows and the files under `sessions/` named after the id (`<id>.json`,
`<id>.jsonl`, `session_<id>.json`, `request_dump_<id>_*.json`); the dashboard, desktop and
Webapp deletes pass `include_compression_chain` and remove the whole compression chain with it
([`hermes_state_sessions.py:1629-1649`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_sessions.py#L1629-L1649),
[`hermes_state_sessions.py:1702-1730`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_sessions.py#L1702-L1730),
[`hermes_cli/sessions_cmd.py:672`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/sessions_cmd.py#L672),
[`hermes_cli/web_routers/sessions.py:488`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/web_routers/sessions.py#L488)).
`hermes sessions prune`, and the sweep when `sessions.auto_prune` is turned on, remove ended
sessions inactive for `retention_days` (90) the same way and may VACUUM the file; automatic
pruning is off by default, and the user is asked once when the disk holding `state.db` runs low
([`hermes_cli/config_defaults.py:2258-2268`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/config_defaults.py#L2258-L2268),
[`hermes_state_maintenance.py:478-502`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_maintenance.py#L478-L502)), so old
sessions stay in `state.db` unless the user opted in or ran `hermes sessions prune`; older copies
live in `state-snapshots/*/state.db`. Archiving
(`hermes sessions archive`, the sidebar, the idle sweep `sessions.auto_archive`, off by default)
only sets `archived = 1` (and `auto_archived` for the sweep) and keeps the messages
([`hermes_state_sessions.py:953-958`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_sessions.py#L953-L958),
[`hermes_state_sessions.py:1969-1981`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_sessions.py#L1969-L1981)).

Every delete path, the prune sweep included, first writes deletion fences (tombstones) into
`state.db` `state_meta` that outlive the rows
([`hermes_state_mutation_retirement.py:148-163`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_mutation_retirement.py#L148-L163),
[`hermes_state_mutation_retirement.py:307-317`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_mutation_retirement.py#L307-L317)),
removes the session's `gateway_routing` entry, and turns each request the session had admitted
into a terminal admission tombstone with the payload emptied
([`hermes_state_mutation_retirement.py:100-140`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_mutation_retirement.py#L100-L140));
the shapes are under [Deletion fences](#deletion-fences). A user delete's marker is permanent; a
sweep's marker and the admission tombstones are reclaimed 7 days after `retired_at`
([`hermes_state_mutation_retirement.py:400-417`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_mutation_retirement.py#L400-L417)).
The marker is the only record of when a session was deleted and of its id; it holds no text. A
deleted session's `conversation_generations` row stays
([Conversation generations](#conversation-generations)).

No delete touches `.hermes_history`, the `cron/output/` run files, `response_store.db`, the
`state-snapshots/` copies of `state.db` (the last 20 by default, each a whole older database), the
other copies of [State database copies](#state-database-copies), the trajectory files in the
working directory, the `conversation_generations` row that names the session's platform chat, or
a memory provider's copy; sessions Hermes imported from Claude Code or Codex CLI stay in
`state.db` after their original files are gone ([Other stores](#other-stores)).

A snapshot is the whole database at the snapshot time, so it outlives a session pruned after 90
idle days, a session deleted with `hermes sessions delete`, a VACUUM, and a `state.db` replaced,
rebuilt or quarantined; it does not see anything written after it. Its rows join the live
database's exactly: the same `sessions.id`, the same `cwd` and `git_repo_root`, the same
`messages.timestamp` (the backup API copies the pages, not a re-encoding). Two things make a
snapshot row differ from its live twin: a message rewound or compacted after the snapshot (live
`active = 0`, so the parser's `[rewound]` prefix), and a session whose `end_reason`, `title` or
later model was set after it (the session start row's text). Those rows are kept from both.

A divert holds messages the replaced database never received, so most of its lines have no twin
in the live database; a line does have one when the same messages also reached a database the
parser reads (a snapshot, or a new `state.db` that a later flush wrote them to), and then it
carries the same session id and, when the line has one, the same `timestamp`. Whether Hermes
replays a divert into a new database was not determined; the session-row heal of
[`agent/session_persistence.py:428-449`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/agent/session_persistence.py#L428-L449) replays the
in-memory transcript only when a session row was deleted, a different failure.

The analyzer reads the snapshots, the `sessions/<id>.jsonl` diverts, `.hermes_history`,
`cron/output/`, `response_store.db` and the trajectory files as secondary sources (see
[Secondary sources](../analyzer/docs/output.md#secondary-sources)), the imported sessions as
Hermes sessions, and the tombstones and the `conversation_generations` rows as `system` rows
([Past-use rows and secondary sources](#past-use-rows-and-secondary-sources),
[Deletion rows](#deletion-rows)). It does not read the copies of `response_store.db` and the
other stores inside a snapshot, the copies of [State database copies](#state-database-copies), or
a memory provider's copy.

### Record schema

Schema in [`hermes_state_common.py:451-562`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_common.py#L451-L562);
`schema_version.version` is 32 ([`hermes_state_common.py:289`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_common.py#L289)).
Type names below are the JSON types of the stored values: SQLite `TEXT` is `string`, `REAL` and
`INTEGER` are `number`, and a nullable column adds `null`.

#### Sessions table

`sessions` ([`hermes_state_common.py:461-529`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_common.py#L461-L529)); token and
cost counters, compression bookkeeping and the handoff columns are omitted.

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `id` | string | | |
| `runtime_revision` | number | | schema 32 |
| `runtime_generation` | number | | schema 32 |
| `source` | string | | |
| `created_source` | string or null | | |
| `user_id` | string or null | | |
| `session_key` | string or null | | |
| `chat_id` | string or null | | |
| `chat_type` | string or null | | |
| `thread_id` | string or null | | |
| `display_name` | string or null | | |
| `origin_json` | string or null | | |
| `model` | string or null | | |
| `model_config` | string or null | | |
| `system_prompt_hash` | string or null | | |
| `parent_session_id` | string or null | | |
| `started_at` | number | | |
| `ended_at` | number or null | | |
| `end_reason` | string or null | | |
| `message_count` | number | | |
| `cwd` | string or null | | |
| `git_branch` | string or null | | |
| `git_repo_root` | string or null | | |
| `title` | string or null | | |
| `last_activity_at` | number or null | | |
| `profile_name` | string or null | | |
| `archived` | number | `0`, `1` | |
| `auto_archived` | number | `0`, `1` | |
| `pinned` | number | `0`, `1` | |
| `hidden` | number | `0`, `1` | |
| `tool_names` | string or null | | |
| `transcript_revision` | number | | schema 32 |

`source` values: the automation sources
([`hermes_state.py:477-479`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state.py#L477-L479)), `local` for the unified runtime, the
importer's tool names ([Imported sessions](#imported-sessions)) and the messaging platforms
([Messaging sessions](#messaging-sessions)). `started_at` and `ended_at` are epoch seconds. `cwd`
and `git_branch` are set at creation and updated by `update_session_cwd`
([`hermes_state_sessions.py:373`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_sessions.py#L373),
[`567-575`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_sessions.py#L567-L575)). `system_prompt_hash` joins
`system_prompts.hash`. `end_reason` is an open set: besides `compression` and `branched`
([Joins and timestamps](#joins-and-timestamps)) the writers set reset reasons such as
`session_reset` and `session_switch`, close reasons such as `agent_close`, `ws_orphan_reap` and
`startup_orphan_reap`
([`hermes_state_common.py:179-201`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_common.py#L179-L201)), and front-end reasons
such as `cli_close`, `tui_close` and `user_exit`. `chat_type` takes the platform adapters' values
(`dm`, `group`, `channel`, `thread` among them). `archived`, `auto_archived`, `pinned` and
`hidden` are flags, `0` or `1`
([`hermes_state_portability.py:75`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_portability.py#L75),
[`722`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_portability.py#L722)).

#### Messages table

`messages` ([`hermes_state_common.py:531-562`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_common.py#L531-L562)), one row per
OpenAI message:

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `id` | number | | |
| `session_id` | string | | |
| `role` | string | | |
| `content` | string or null | | |
| `tool_call_id` | string or null | | |
| `tool_calls` | string or null | | |
| `tool_name` | string or null | | |
| `effect_disposition` | string or null | | |
| `timestamp` | number | | |
| `token_count` | number or null | | |
| `finish_reason` | string or null | | |
| `reasoning` | string or null | | |
| `reasoning_content` | string or null | | |
| `reasoning_details` | string or null | | |
| `codex_reasoning_items` | string or null | | |
| `codex_message_items` | string or null | | |
| `platform_message_id` | string or null | | |
| `active` | number | `0`, `1` | |
| `compacted` | number | `0`, `1` | |
| `api_content` | string or null | | |
| `display_kind` | string or null | | |
| `display_metadata` | string or null | | |
| `message_uid` | string or null | | |
| `tool_call_uid` | string or null | | |
| `tool_call_uids` | string or null | | |

- `role`: the four OpenAI roles, plus `session_meta`, a bookkeeping row the gateway writes first
  in a new messaging session, which every reader filters out
  ([`hermes_state_messages.py:611-616`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_messages.py#L611-L616),
  [`gateway/run_turn_persistence.py:473-483`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/gateway/run_turn_persistence.py#L473-L483)).
- `content`: list or dict content (multimodal parts) is stored as `"\x00json:"` + JSON, and a
  literal string starting with that prefix is stored JSON-encoded behind it
  ([`hermes_state_messages.py:140-160`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_messages.py#L140-L160), prefix at
  [`hermes_state.py:1629`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state.py#L1629)). The JSON is a list of OpenAI content
  parts ([Content parts](#content-parts)).
- `tool_calls`: a JSON list on assistant rows
  ([`hermes_state_messages.py:278`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_messages.py#L278)), its entries under
  [Tool call entries](#tool-call-entries).
- `tool_call_id`, `tool_name`: on `role='tool'` result rows
  ([`agent/turn_tool_round.py:118-121`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/agent/turn_tool_round.py#L118-L121)).
- `timestamp`: `time.time()` epoch seconds, UTC
  ([`hermes_state_messages.py:415`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_messages.py#L415)); a message written without
  one gets the write time
  ([`hermes_state_messages.py:665-671`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_messages.py#L665-L671)).
- `reasoning`, `reasoning_content`: thinking text; `reasoning_details`, `codex_reasoning_items`
  and `codex_message_items` are JSON
  ([`hermes_state_messages.py:294-296`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_messages.py#L294-L296)).
- `active`, `compacted`: rewind and compaction set `active=0, compacted=1` instead of deleting
  ([`hermes_state_messages.py:73`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_messages.py#L73)).
- `api_content`: the exact string sent to the API when it differed from `content`
  ([`hermes_state_messages.py:406-408`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_messages.py#L406-L408)).
- `platform_message_id`, `display_metadata`, `message_uid`, `tool_call_uid(s)`: gateway and UI
  bookkeeping; `display_kind` is under [Model-switch markers](#model-switch-markers).

#### Tool call entries

Each entry of a `tool_calls` list, as the agent builds it, with `type` taken from the model
provider's tool call and `arguments` the model's JSON text
([`agent/chat_completion_helpers.py:1609-1611`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/agent/chat_completion_helpers.py#L1609-L1611)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `id` | string | | |
| `call_id` | string | | |
| `response_item_id` | string or null | | |
| `type` | string | | |
| `function` | object | | |
| `name` | string | | |
| `arguments` | string | | |

`name` and `arguments` are the keys of `function`.

#### Content parts

A part list holds OpenAI chat-completions content parts: text parts and image parts, whose `type`
is `image_url` (with an `image_url` object holding the `url`), `image` or `input_image`
([`agent/session_persistence.py:46`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/agent/session_persistence.py#L46),
[`agent/image_routing.py:562`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/agent/image_routing.py#L562)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `type` | string | `text`, `image_url`, `image`, `input_image` | |
| `text` | string | | |
| `image_url` | object | | |

The agent's own turn writer stores a part list's text projection, each image as `[screenshot]`
([`agent/session_persistence.py:122-133`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/agent/session_persistence.py#L122-L133)); a
gateway line of a divert keeps the message as the gateway queued it, parts included.

#### Divert lines

A line of `sessions/<id>.jsonl` ([Transcript stores](#transcript-stores)) is the message dict
JSON-encoded as a whole, so `tool_calls` is the list itself rather than a JSON string, and an
agent's line carries its booleans `_compressed_summary` and `observed`
([`agent/session_persistence.py:242-256`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/agent/session_persistence.py#L242-L256),
[`hermes_state.py:434-448`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state.py#L434-L448)). The keys the parser reads:

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `role` | string | | |
| `content` | string, array, object or null | | |
| `tool_calls` | array or null | | |
| `tool_call_id` | string or null | | |
| `tool_name` | string or null | | |
| `timestamp` | number or null | | |
| `reasoning` | string or null | | |
| `reasoning_content` | string or null | | |
| `display_kind` | string or null | | |

#### Model usage

Model per message is not stored; use `sessions.model`, or `session_model_usage.model`
([`hermes_state_common.py:564-584`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_common.py#L564-L584)) when a session switched
models. One row per (`session_id`, `model`, `billing_provider`, `billing_base_url`,
`billing_mode`, `task`), upserted with the token delta of each API call that reports usage
([`hermes_state_usage.py:362-390`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_usage.py#L362-L390)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `session_id` | string | | |
| `model` | string | | |
| `billing_provider` | string | | |
| `billing_base_url` | string | | |
| `billing_mode` | string | | |
| `task` | string | | |
| `api_call_count` | number | | |
| `first_seen` | number or null | | |
| `last_seen` | number or null | | |

`task` is empty for the main agent loop and names the work for auxiliary calls (`vision`,
`compression`, `title_generation`, `background_review`, `moa_reference`, `moa_aggregator` and
`plugin_side_question` among them); `first_seen` and
`last_seen` are `time.time()` epoch seconds. `sessions.model` is overwritten by a mid-session
`/model` switch
([`hermes_state_sessions.py:717-744`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_sessions.py#L717-L744)), so it holds the
latest model, not the first.

#### Model-switch markers

Model switches made in the TUI, the desktop app or another front end of the TUI gateway are also
durable message rows: a `role = 'user'` row with `display_kind = 'model_switch'` and the content
`[System: The active model for this chat has changed to <model> via provider <provider>. From
this point forward, ...]`, timestamped at the switch
([`tui_gateway/server.py:1152`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/tui_gateway/server.py#L1152),
[`tui_gateway/server.py:1169-1209`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/tui_gateway/server.py#L1169-L1209)). Each switch first
deactivates the earlier marker rows of the session (`active = 0`), which are kept, never deleted
([`hermes_state_messages.py:1086-1092`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_messages.py#L1086-L1092)), so every switch
survives as one row with the model, the provider and the time; the classic CLI's `/model` writes
no row and only updates `sessions.model`. Other `display_kind` values, `hidden`, `steer`,
`internal_notification`, `async_delegation_complete`, `process_complete` and `failed_turn` among
them, mark rows that are not typed input either
([`hermes_state_detached_results.py:29`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_detached_results.py#L29),
[`agent/prompt_builder.py:583`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/agent/prompt_builder.py#L583),
[`agent/turn_failure_copy.py:74`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/agent/turn_failure_copy.py#L74),
[`gateway/response_filters.py:33`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/gateway/response_filters.py#L33),
[`tools/process_registry_notifications.py:350`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/tools/process_registry_notifications.py#L350)).

#### Permission outcomes and settings

Permission outcomes are in the result row of the shell tool, registered as `terminal`
([`tools/terminal_tool.py:1727`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/tools/terminal_tool.py#L1727)), whose `content` is a JSON
object
([`tools/terminal_tool.py:1044-1050`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/tools/terminal_tool.py#L1044-L1050),
[`tools/terminal_tool.py:1101-1126`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/tools/terminal_tool.py#L1101-L1126),
[`tools/terminal_tool_result.py:269`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/tools/terminal_tool_result.py#L269),
[`tools/terminal_tool_background.py:188-189`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/tools/terminal_tool_background.py#L188-L189)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `output` | string | | |
| `exit_code` | number | | |
| `error` | string or null | | |
| `status` | string | | |
| `user_summary` | string | | |
| `approval` | string | | |
| `approval_pending` | boolean | `true` | |
| `command` | string | | |
| `description` | string | | |
| `pattern_key` | string | | |
| `smart_denied` | boolean | | |
| `allow_permanent` | boolean | | |

A command that needed approval and got it carries
`"approval": "Command required approval (<description>) and was approved by the user."`
or `"Command was flagged (<description>) and auto-approved by smart approval."` (smart approval
is Hermes' guardian model that judges a flagged command). A refused one is
`{"output": "", "exit_code": -1, "error": <message>, "status": "blocked"}` with `user_summary`
when a person was asked, and one waiting for a gateway `/approve` has
`"status": "pending_approval"` with the command, its description, `pattern_key`, `smart_denied`
and `allow_permanent`. The refusal message names the outcome: `BLOCKED by smart approval`, a user
denial with the user's `/deny` reason, a timeout ("Silence is not consent"), or a withdrawn
prompt; the outcome enum is `denied`, `timeout`, `notify_failed`, `cancelled`, `blocked`
([`tools/approval.py:538`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/tools/approval.py#L538),
[`tools/approval.py:800`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/tools/approval.py#L800),
[`tools/approval.py:894-900`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/tools/approval.py#L894-L900)). The `outcome` key itself is not
written to the result.

An "always" answer appends the command's pattern to `command_allowlist` in the profile's
`config.yaml`
([`tools/approval.py:411-424`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/tools/approval.py#L411-L424),
[`tools/approval.py:480-505`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/tools/approval.py#L480-L505)), and a session's `/yolo` bypass (all
approvals off for that session) is kept as `yolo_mode: true` in `sessions.model_config` for the
CLI, TUI and desktop and in the gateway's routing entry for messaging sessions
([`tools/approval_yolo.py:1-8`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/tools/approval_yolo.py#L1-L8),
[`agent/agent_init.py:1117-1127`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/agent/agent_init.py#L1117-L1127)). The other standing settings
are in `config.yaml`: `approvals.mode` (`manual`, `smart` or `off`, default `smart`),
`approvals.cron_mode`, `single_query_mode` and `unattended_mode` (`deny` or `approve`, default
`deny`), `approvals.deny` (globs that block even under `/yolo`) and `command_allowlist`
([`hermes_cli/config_defaults.py:1652-1688`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/config_defaults.py#L1652-L1688),
[`tools/approval_context.py:263-280`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/tools/approval_context.py#L263-L280)); the environment
variable `HERMES_YOLO_MODE` turns every check off for the process
([`tools/approval.py:46`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/tools/approval.py#L46)). The messaging gateway's pairing store, the
standing list of platform users allowed to drive the agent, is `pairing/<platform>-approved.json`
beside `<platform>-pending.json`
([`gateway/pairing.py:1-7`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/gateway/pairing.py#L1-L7),
[`gateway/pairing.py:353-356`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/gateway/pairing.py#L353-L356),
[`gateway/pairing.py:432-438`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/gateway/pairing.py#L432-L438)), a JSON object keyed by user id:

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `user_name` | string | | |
| `approved_at` | number | | |

`approved_at` is epoch seconds.

#### Messaging sessions

A messaging session's row names the platform and the people: `sessions.source` is the platform
(`telegram`, `discord`, `whatsapp`, `whatsapp_cloud`, `slack`, `signal`, `mattermost`, `matrix`,
`email`, `sms`, `dingtalk`, `feishu`, `wecom`, `wecom_callback`, `weixin`, `bluebubbles`,
`qqbot`, `yuanbao`, `webhook`, `msgraph_webhook`, `relay` (the experimental connector relay), or a
plugin platform such as `irc`, `line`, `teams` or `google_chat`; `api_server` and `local` are not
messaging platforms)
([`gateway/config.py:217-262`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/gateway/config.py#L217-L262)), and the row carries `user_id`, `chat_id`,
`chat_type`, `thread_id`, `display_name` (the chat name) and `origin_json`, the serialized source
([`gateway/session.py:69-81`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/gateway/session.py#L69-L81),
[`gateway/session.py:133-136`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/gateway/session.py#L133-L136),
[`gateway/session_recovery.py:338-347`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/gateway/session_recovery.py#L338-L347),
[`hermes_state_gateway.py:236-292`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_gateway.py#L236-L292),
[`run_agent.py:57-75`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/run_agent.py#L57-L75)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `platform` | string | | |
| `chat_id` | string | | |
| `chat_name` | string or null | | |
| `chat_type` | string | | |
| `user_id` | string or null | | |
| `user_name` | string or null | | |
| `thread_id` | string or null | | |
| `chat_topic` | string or null | | |

A message that came through the experimental connector relay (a generic adapter fed by a
separate connector service) records the underlying platform, not `relay`
([`gateway/session.py:100-103`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/gateway/session.py#L100-L103)).

#### Deletion fences

`state_meta` is a key and JSON value table, columns `key` and `value`
([`hermes_state_common.py:640-643`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_common.py#L640-L643)). A delete writes the key
`gateway.retired_session.v1.<session id>`
([`hermes_state_mutation_retirement.py:9`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_mutation_retirement.py#L9),
[`hermes_state_mutation_retirement.py:286-300`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_mutation_retirement.py#L286-L300)),
and a multiplexed gateway writes `gateway.retired_route.v1.` fences for other profiles
([`hermes_state_mutation_retirement.py:14`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_mutation_retirement.py#L14)). The
value:

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `retired_at` | number | | |
| `own` | boolean | `true` | |
| `expires` | boolean | `true` | |

`retired_at` is epoch seconds; `"expires": true` is present when a sweep rather than a delete
removed the session. Each admitted request becomes a
`gateway.terminal_admission.v1.<admission id>` row with the principal, request id and digests
and the payload emptied
([`hermes_state_mutation_retirement.py:100-140`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_mutation_retirement.py#L100-L140)).

#### Local policy records

A session created through the unified runtime has a creation receipt in `state_meta` under
`gateway.local_policy.v1:<session id>`, whose `policy.cwd` is the directory its tools run in
([`hermes_state_local.py:9`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_local.py#L9),
[`hermes_state_local.py:14-31`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_local.py#L14-L31),
[`hermes_state_local.py:390-401`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_local.py#L390-L401)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `profile_id` | string | | |
| `principal_id` | string | | |
| `request_id` | string | | |
| `session_id` | string | | |
| `route` | string | | |
| `policy` | object | | |

### Joins and timestamps

- Message to session: `messages.session_id = sessions.id`; project path is `sessions.cwd` (or
  `git_repo_root`), branch `sessions.git_branch`.
- Tool use to result: `messages.tool_calls[].id` (or `call_id`) equals a later `role='tool'`
  row's `tool_call_id`. Hermes pairs on `call_id` then `id`, splitting at `|`
  ([`agent/message_sanitization.py:539-547`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/agent/message_sanitization.py#L539-L547)).
- Parent to child: `sessions.parent_session_id`. A parent whose `end_reason='compression'`
  continues in its child; `'branched'` marks a `/branch` fork
  ([`hermes_state_common.py:170-175`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_common.py#L170-L175)); `source='subagent'`
  rows are delegated tasks.
- System prompt: `sessions.system_prompt_hash = system_prompts.hash`.
- Times: `messages.timestamp`, `sessions.started_at` and `ended_at` are REAL epoch seconds
  ([Record schema](#record-schema)); how snapshot and divert rows join the live rows is under
  [What survives a delete](#what-survives-a-delete).

### Other stores

The stores below sit beside `state.db`. Each says whether the parser reads it.

#### Prompt history

`.hermes/.hermes_history`: the prompt history of the classic CLI and of the TUI, shared by both
([`hermes_cli/cli_init_mixin.py:415`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/cli_init_mixin.py#L415),
[`hermes_cli/cli_tui_mixin.py:2204`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/cli_tui_mixin.py#L2204),
[`ui-tui/src/lib/history.ts:6-7`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/ui-tui/src/lib/history.ts#L6-L7)). prompt_toolkit's
`FileHistory` format: per entry a blank line, a `# <timestamp>` line and the entry's lines each
prefixed `+`; the TUI writes the timestamp as UTC `YYYY-MM-DD HH:MM:SS.mmm` and keeps the last
1000 entries in memory only
([`ui-tui/src/lib/history.ts:71-78`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/ui-tui/src/lib/history.ts#L71-L78)), while prompt_toolkit
3.0.52, the version Hermes pins, writes `datetime.now()`, host-local time with microseconds and no
zone
([`src/prompt_toolkit/history.py:297-306`](https://github.com/prompt-toolkit/python-prompt-toolkit/blob/d8adbe9bfcf5f7e95b015d8ab12e2985b9a7822b/src/prompt_toolkit/history.py#L297-L306)),
so the number of fraction digits tells the two writers apart. Every typed line, slash commands
included (`/model`, `/yolo`, `/snapshot`), with no session id or project. Nothing that deletes a
session touches it, so it survives a delete, a prune, a rewind and a removed project. Joins on
text and time only. Secondary; read by the parser.

#### Cron run archive

`cron/output/<job id>/<YYYY-mm-dd_HH-MM-SS>.md`: one Markdown file per cron run, local time in the
name, with a `# Cron Job: <id>` header and `## Prompt` and `## Response` sections, pruned per job
to a configured count
([`cron/jobs.py:119`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/cron/jobs.py#L119),
[`cron/jobs.py:3353-3364`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/cron/jobs.py#L3353-L3364),
[`cron/scheduler_prompt.py:72-73`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/cron/scheduler_prompt.py#L72-L73)). The name's time and the
document's `**Run Time:**` are `hermes_time.now()`, Hermes' configured zone or the host's, written
without an offset
([`cron/jobs.py:3353-3361`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/cron/jobs.py#L3353-L3361),
[`hermes_time.py:147-150`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_time.py#L147-L150)). A completed run's document is the header
(`# Cron Job: <name>`, `**Job ID:**`, `**Run Time:**`, `**Schedule:**`), then
`**Prompt Characters:** <n>` and `## Prompt` with the prompt, a blank line,
`**Response Characters:** <m>` and `## Response` with the response and a newline; the two lengths
frame the sections, so a heading quoted inside the prompt is no boundary
([`cron/scheduler.py:2219-2227`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/cron/scheduler.py#L2219-L2227),
[`cron/scheduler.py:2622-2628`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/cron/scheduler.py#L2622-L2628),
[`cron/scheduler_prompt.py:70-80`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/cron/scheduler_prompt.py#L70-L80)). Older writers left out
the frames, and Hermes' own reader then takes the last `## Response` heading as the boundary
([`cron/scheduler_prompt.py:90-104`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/cron/scheduler_prompt.py#L90-L104)); a failed, blocked or
script-gated run is a short document with no prompt section
([`cron/scheduler.py:2248`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/cron/scheduler.py#L2248),
[`cron/scheduler.py:2284-2289`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/cron/scheduler.py#L2284-L2289)). Survives the prune or delete of
the run's `source='cron'` session. Joins on the job id and time, not on a session id. Secondary;
read by the parser.

#### API server response store

`.hermes/response_store.db`, one per profile home the API server serves: the API server's store
for the OpenAI Responses API (`previous_response_id` chaining)
([`gateway/platforms/api_server_response_store.py:61`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/gateway/platforms/api_server_response_store.py#L61),
[`gateway/platforms/api_server.py:1689`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/gateway/platforms/api_server.py#L1689)). Table
`responses` (`response_id`, `data`, `accessed_at`) and table `conversations` (`name`,
`response_id`); it keeps the 100 most recently used responses
([`gateway/platforms/api_server_response_store.py:17`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/gateway/platforms/api_server_response_store.py#L17),
[`gateway/platforms/api_server_response_store.py:34-37`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/gateway/platforms/api_server_response_store.py#L34-L37),
[`gateway/platforms/api_server_response_store.py:72-75`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/gateway/platforms/api_server_response_store.py#L72-L75)).
`data` is a JSON object, the full cumulative history at that response
([`gateway/platforms/api_server_openai_routes.py:392-396`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/gateway/platforms/api_server_openai_routes.py#L392-L396)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `response` | object | | |
| `conversation_history` | array | | |
| `instructions` | string or null | | |
| `session_id` | string or null | | |

`response` is the Responses API envelope (`id`, `object`, `status`, `created_at` in epoch
seconds, `model`, `output`, `usage`), and `conversation_history` is a list of OpenAI
chat-completions message dicts, the shape of a `messages` row: the earlier history, the turn's
user message and the agent's messages, or the assistant's final text when the agent returned none
([`gateway/platforms/api_server_openai_routes.py:361-389`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/gateway/platforms/api_server_openai_routes.py#L361-L389),
[`gateway/platforms/api_server_openai_routes.py:1532-1560`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/gateway/platforms/api_server_openai_routes.py#L1532-L1560));
the messages carry no time of their own. No session delete path names it (a grep for
`response_store` and `ResponseStore` in `hermes_state*.py` and `hermes_cli/sessions*.py` finds
only the WAL helper), so it outlives a deleted `api_server` session. Joins on `session_id`.
Secondary; read by the parser.

#### Trajectory files

`trajectory_samples.jsonl` and `failed_trajectories.jsonl` in the working directory (project
files, collected by the project catalog): one line per run in the ShareGPT format (a common layout
for conversation training data), no session id
([`agent/trajectory.py:37-41`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/agent/trajectory.py#L37-L41)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `conversations` | array | | |
| `timestamp` | string | | |
| `model` | string | | |
| `completed` | boolean | | |

`timestamp` is local ISO 8601 without a zone. Each `conversations` entry is `{from, value}`:
`system` (the tool-calling system prompt), `human` (the user's message), `gpt` (a reply, or a
tool-call turn in `<tool_call>` markup) and `tool` (the call results in `<tool_response>` blocks)
([`agent/agent_runtime_helpers.py:170-196`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/agent/agent_runtime_helpers.py#L170-L196)). Written
only with `--save-trajectories`; survives everything. Joins on text within a project. Secondary;
read by the parser.

#### Imported sessions

`hermes_cli/foreign_sessions.py` reads Claude Code `~/.claude/projects/*/*.jsonl` (or under
`CLAUDE_CONFIG_DIR`) and Codex CLI `~/.codex/sessions/**/rollout-*.jsonl` (or under `CODEX_HOME`)
and copies their user and assistant turns into `state.db` as a new session, with tool calls
reduced to `[ran tool: <name>]` text
([`hermes_cli/foreign_sessions.py:27-28`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/foreign_sessions.py#L27-L28),
[`hermes_cli/foreign_sessions.py:174-179`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/foreign_sessions.py#L174-L179),
[`77`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/foreign_sessions.py#L77), [`170`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/foreign_sessions.py#L170)). Two
paths write it:

- `hermes sessions import` and `hermes --resume @claude` or `@codex` write a `source = 'cli'`
  session with no `origin_json`, the original `cwd` only when it exists on this host, and the
  title `Imported from <tool>: <first line>`
  ([`hermes_cli/foreign_sessions.py:232-266`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/foreign_sessions.py#L232-L266),
  [`hermes_cli/main_resume_args.py:8-25`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/main_resume_args.py#L8-L25)); the title
  is the only mark of the import.
- The desktop importer writes `source` `claude-code` or `codex-cli`, `origin_json`
  `{"imported_from": {"tool", "path", "foreign_session_id"}}` and `profile_name`
  ([`hermes_cli/foreign_sessions_browser.py:74-96`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/foreign_sessions_browser.py#L74-L96),
  [`hermes_state_portability.py:181-207`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_portability.py#L181-L207)).

The turns carry no time, so their `timestamp` is the import's
([`hermes_state_messages.py:665-671`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_messages.py#L665-L671)). The parser reads
these rows as Hermes sessions (they are in `state.db`), and its start row names the source tool,
the original path and the foreign session id when `origin_json` has them
([Past-use rows and secondary sources](#past-use-rows-and-secondary-sources)); such a session is a
copy of another agent's transcript that outlives the original file.

#### Bot Mode group chats

Bot Mode is the desktop app's roster of profiles as named bots that share group chats. Tables
`hosted_rooms` and `hosted_room_events` are an append-only room log in the gateway's root
`state.db`
([`gateway/hosted_rooms.py:1-5`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/gateway/hosted_rooms.py#L1-L5),
[`gateway/hosted_rooms.py:88-117`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/gateway/hosted_rooms.py#L88-L117)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `room_id` | string | | |
| `seq` | number | | |
| `event_id` | string | | |
| `kind` | string | | |
| `actor_json` | string | | |
| `payload_json` | string | | |
| `created_at` | number | | |

The event kinds are fixed per actor: `message.user` for the user, `message.member` for a member
bot, `member.unavailable`, `room.activity`, `room.stop_requested` and the `turn.*` kinds for the
gateway, and `authority.*`, `room.created`, `room.disbanded`, `room.members_changed` and
`room.renamed` for the system; `actor_json` is `{kind, id}` with optional `display_name`,
`profile` and `connection_id`
([`gateway/hosted_rooms.py:48-60`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/gateway/hosted_rooms.py#L48-L60)). A `message.user` payload
is `{thread_id, text, attachments?}` and a `message.member` payload carries the member's turn
coordinates and `text`
([`gateway/hosted_room_discussion.py:211-223`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/gateway/hosted_room_discussion.py#L211-L223),
[`gateway/hosted_room_discussion.py:400-421`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/gateway/hosted_room_discussion.py#L400-L421)).
Whether a room message also becomes a `messages` row was not determined. Read by the parser as
`system` rows.

#### Conversation generations

`conversation_generations` in `state.db`: one row per messaging routing peer, deliberately never
deleted, so it names every platform chat (`session_key` is `agent:<profile>:<platform>:...`) that
ever had a conversation after its sessions are gone
([`hermes_state_common.py:658-685`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_common.py#L658-L685)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `source` | string | | |
| `session_key` | string | | |
| `generation` | number | | |

No time column. A record of relay use, not of turns, with no text to join on; read by the parser
as `system` rows.

#### Desktop connection registry

The desktop app's `connections.json` (version 2): exactly one `local` entry is always present, and
an entry that failed to load is kept under `quarantined` as `{reason, entry}`
([`apps/desktop/electron/connection-registry.ts:19-20`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/apps/desktop/electron/connection-registry.ts#L19-L20),
[`apps/desktop/electron/connection-registry.ts:44-106`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/apps/desktop/electron/connection-registry.ts#L44-L106)).
The registry:

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `version` | number | `2` | |
| `primary` | string | | |
| `launchMode` | string | `last-used`, `primary` | |
| `lastUsed` | string | | |
| `connections` | array | | |
| `quarantined` | array | | |

Each connection:

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `id` | string | | |
| `kind` | string | `local`, `remote`, `ssh`, `cloud` | |
| `label` | string | | |
| `url` | string | | |
| `authMode` | string | `token`, `oauth` | |
| `token` | object | | |
| `headers` | object | | |
| `org` | string | | |
| `name` | string | | |
| `host` | string | | |
| `user` | string | | |
| `port` | number | | |
| `keyPath` | string | | |
| `remoteHermesPath` | string | | |
| `remoteProfile` | string | | |

`remote` is a `hermes serve` gateway and `cloud` is Hermes Cloud; `url`, `authMode`, `token` and
`headers` belong to `remote` and `cloud`, `org` and `name` to `cloud`, and the rest to `ssh`. It
records which other Hermes hosts the app drove, not their sessions. Read by the parser.

#### Debug-share paste list

`.hermes/pastes/pending.json`: the paste.rs links (paste.rs is a public, anonymous paste service)
that `hermes debug share`, the gateway's `/debug` and the dashboard's debug-share endpoint
uploaded and that are still due for deletion
([`hermes_cli/debug.py:42-43`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/debug.py#L42-L43)). A JSON list of entries, merged by
URL keeping the later time
([`hermes_cli/debug.py:180-195`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/debug.py#L180-L195),
[`hermes_cli/debug.py:32`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/debug.py#L32)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `url` | string | | |
| `expire_at` | number | | |

`expire_at` is epoch seconds, the upload time plus 21600 seconds (6 hours). An entry is dropped
once its paste is deleted, or 24 hours after `expire_at` when the delete keeps failing
([`hermes_cli/debug.py:73-103`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/debug.py#L73-L103)); the gateway sweeps it hourly and
every `hermes debug` call sweeps it first
([`gateway/run.py:4341-4345`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/gateway/run.py#L4341-L4345),
[`hermes_cli/debug.py:669-671`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/debug.py#L669-L671)). The file is missing or `[]`
when nothing was shared in the last 6 to 30 hours. Links to dpaste.com (the fallback paste
service) are never written, since dpaste.com has no anonymous delete
([`hermes_cli/debug.py:182-184`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/debug.py#L182-L184)). Read by the parser
([Sync, relay and origin rows](#sync-relay-and-origin-rows)).

#### Snapshot manifest

Each `state-snapshots/<id>/manifest.json`
([`hermes_cli/backup.py:1430-1439`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/backup.py#L1430-L1439)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `id` | string | | |
| `timestamp` | string | | |
| `label` | string or null | | |
| `file_count` | number | | |
| `total_size` | number | | |
| `files` | object | | |
| `failed_dbs` | array | | |
| `oversized_skipped` | array | | |

Not read by the parser, which takes the snapshot time from `<id>`.

#### State database copies

Three other writers leave older copies of `state.db` beside it in the home:

- `state.db.pre-update-emergency-<UTC time>-<pid>.bak`: a whole copy the desktop updater's
  pre-flight makes before an update, checked with `PRAGMA quick_check`; the two newest are kept
  ([`hermes_cli/backup_sqlite.py:221-255`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/backup_sqlite.py#L221-L255),
  [`hermes_cli/backup.py:134-143`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/backup.py#L134-L143)).
- `state.db.malformed-backup-<YYYYmmdd_HHMMSS>`: a raw copy of a damaged database made before a
  repair, host-local time in the name; three are kept
  ([`hermes_state_repair.py:47`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_repair.py#L47),
  [`hermes_state_repair.py:504-509`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_repair.py#L504-L509)).
- `state.db.retired-wal-<YYYYmmdd-HHMMSS>-<pid>/`: the frames of a WAL whose file was deleted or
  replaced under a live writer, captured as `state.db-wal`, the main image `state.db` (or only its
  header, `state.db.header`, above 512 MiB) and a `manifest.json` with `captured_at`, `trigger`
  and `database`
  ([`hermes_state_dbfile.py:396-416`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_dbfile.py#L396-L416),
  [`hermes_state_dbfile.py:540-591`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_dbfile.py#L540-L591)).

A database that is not SQLite at all is moved aside as `state.db.zeroed-<time>-<pid>.bak` or
`state.db.notadb-<time>-<pid>.bak`
([`hermes_state_dbfile.py:695-718`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_dbfile.py#L695-L718)). Not read by the parser.

#### RetainDB queue

`.hermes/retaindb_queue.db`: the write-behind queue of the RetainDB memory provider (a paid cloud
memory service), present only when `memory.provider` is `retaindb`
([`plugins/memory/retaindb/__init__.py:344-346`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/plugins/memory/retaindb/__init__.py#L344-L346)).
Table `pending`; a row is deleted as soon as its upload succeeds, so the table holds only turns not
yet uploaded
([`plugins/memory/retaindb/__init__.py:194-198`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/plugins/memory/retaindb/__init__.py#L194-L198),
[`plugins/memory/retaindb/__init__.py:226-241`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/plugins/memory/retaindb/__init__.py#L226-L241)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `id` | number | | |
| `user_id` | string | | |
| `session_id` | string | | |
| `messages_json` | string | | |
| `created_at` | string | | |
| `last_error` | string or null | | |

`messages_json` holds the turn's user and assistant text; `created_at` is ISO 8601 UTC. Not read
by the parser.

### Format versions

`schema_version.version` was 31 at
[`8b66a51`](https://github.com/NousResearch/hermes-agent/commit/8b66a51036c1e20920a17cdd049fdf55c968d683)
([`hermes_state_common.py:283`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_state_common.py#L283)) and is 32 at
[`86fa9e6`](https://github.com/NousResearch/hermes-agent/commit/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872)
and
[`6bb377a`](https://github.com/NousResearch/hermes-agent/commit/6bb377a51d5c8e279a7a5e34d717d653d421d064)
([`hermes_state_common.py:289`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_common.py#L289)); the tables did not change
between the last two. Migrations add columns in place
([`hermes_state_schema.py:1061-1102`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_schema.py#L1061-L1102)), so a parser must
select columns that exist (`PRAGMA table_info`); the parser reads `select *`, so new columns do
not affect it, and `SCHEMA_VERSION` is the constant that names the version. Gateway metadata
was in `sessions/sessions.json` before v18 ([`hermes_state_schema.py:1339-1342`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_schema.py#L1339-L1342)). The
legacy `session_<id>.json` layout was not examined.

Schema 32 added to `sessions` the columns `runtime_revision`, `runtime_generation` and
`transcript_revision` (missing at
[`hermes_state_common.py:382-447`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_state_common.py#L382-L447)), and four tables of the
unified runtime: `runtime_epoch`, `session_admissions` (each queued, steered or redirected request
against a session, with `payload_json` and `status`), `worker_executions` and `worker_receipts`
([`hermes_state_common.py:586-638`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_common.py#L586-L638)); `messages` and
`session_model_usage` are unchanged. The `local-` session ids, the deletion fences and the prune
default changed between `8b66a51` and `86fa9e6`: `sessions.auto_prune` was `true` with
`retention_days` 90 at
[`hermes_cli/config_defaults.py:2276-2285`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/hermes_cli/config_defaults.py#L2276-L2285) and
is `false` now ([What survives a delete](#what-survives-a-delete)).

Changed between `86fa9e6` and `6bb377a`:

- The CLI importer wrote `source` `claude-code` or `codex-cli` with `origin_json.imported_from`
  ([`hermes_cli/foreign_sessions.py:254-260`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/hermes_cli/foreign_sessions.py#L254-L260)); it
  now writes a plain `cli` session through the gateway, and only the desktop importer keeps the
  tool name and `origin_json` ([Imported sessions](#imported-sessions)).
- The Mem0 memory provider and the Langfuse plugin were part of the repository
  ([`plugins/memory/mem0/__init__.py:302-313`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/plugins/memory/mem0/__init__.py#L302-L313),
  [`plugins/observability/langfuse/__init__.py:1-7`](https://github.com/NousResearch/hermes-agent/blob/8b66a51036c1e20920a17cdd049fdf55c968d683/plugins/observability/langfuse/__init__.py#L1-L7));
  they are now catalog-installed plugins
  ([`plugin-catalog/mem0.yaml:1-10`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/plugin-catalog/mem0.yaml#L1-L10),
  [`plugin-catalog/langfuse.yaml:1-9`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/plugin-catalog/langfuse.yaml#L1-L9)).
- `webapp` joined `PROFILE_CREDENTIAL_PATHS` with the Webapp
  ([`hermes_cli/profiles.py:154-183`](https://github.com/NousResearch/hermes-agent/blob/86fa9e6ca1de7ee71132d2be0103f3e37d1fa872/hermes_cli/profiles.py#L154-L183)).

Every other file, table, column and constant the parser depends on is unchanged from
`86fa9e6`: `state.db`, the `\x00json:` prefix, the divert `sessions/<id>.jsonl`
([`hermes_state.py:435-442`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state.py#L435-L442)), `state-snapshots` and
`pastes/pending.json` with its 21600-second lifetime
([`hermes_cli/debug.py:43`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/debug.py#L43)).

### Secrets

Tool-call `arguments` are stored unredacted on purpose, so `state.db` holds credentials typed
into commands
([`agent/chat_completion_helpers.py:1607-1616`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/agent/chat_completion_helpers.py#L1607-L1616));
tool results, `messages.api_content` and `system_prompts.prompt` (which embeds memory and context
files) carry secrets too. `request_dump_*` files are redacted before writing
([`agent_runtime_helpers.py:1590-1594`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/agent/agent_runtime_helpers.py#L1590-L1594)).
`sessions.origin_json` holds messaging user and chat identifiers. The same holds for the copies
of `state.db` in `state-snapshots/` and [State database copies](#state-database-copies).

### Parser plan

Inputs, under any Hermes home (a home's `.hermes`, `.hermes<suffix>`, `AppData/Local/hermes`,
`AppData/Local/hermes<suffix>`, any of them followed by `profiles/<name>`, or the root of a Docker
or Podman named volume's `_data`, which holds the Hermes home itself, see
[Docker and Podman volumes](#docker-and-podman-volumes)), each marked for the timeline's
deduplication (see
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
| `sessions/<id>.json`, `session_<id>.json` | not read | no writer, shape unknown |
| `backups/*.zip`, `sessions/saved/*.json`, the stores other than `state.db` inside a snapshot, `connection.json` | not read | archives, exports and copies |

Not read: `state.db.pre-update-emergency-*.bak`, `state.db.malformed-backup-*`,
`state.db.retired-wal-*`, `state-snapshots/<id>/manifest.json`, `retaindb_queue.db`,
`gateway_routing`, `session_admissions`.

The snapshots, the diverts, the response store and the trajectory files keep the default
2-second tolerance: their times are copies of the live ones, not later stamps, or absent.
`.hermes_history` and the cron run files keep host-local times without a zone (the TUI's history
entries excepted), written as if UTC, so their window is 14 hours plus 2 seconds, which spans every
zone offset. Rows: one per message; an assistant row with N `tool_calls` yields one `assistant`
row (when `content` is non-empty), one `thinking` row (when `reasoning`/`reasoning_content` is
set), and N `tool_use` rows. Order by `timestamp, id`. Include `active=0` rows: they are evidence
of rewound or compacted turns.

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
| text | `content` (strip `\x00json:`, join text parts, an image part as `[image]`); for tool_use the first of the `command`, `path`, `file_path`, `url` and `query` keys of `arguments`, else the arguments |
| source_line | `messages.id` |

`-` stands for an absent value in every row text below. `wants()` accepts the files of a Hermes
home at the root of a volume's `_data` as it accepts the home forms, and those rows carry the
manifest's `docker` user ([Docker and Podman volumes](#docker-and-podman-volumes)).

#### Permission rows

The records are under [Permission outcomes and settings](#permission-outcomes-and-settings).

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
  ([`hermes_cli/config.py:1954`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/config.py#L1954),
  [`hermes_cli/config_backend.py:465-468`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/config_backend.py#L465-L468),
  [`hermes_yaml.py:89-98`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_yaml.py#L89-L98)); any other construct under the key is one
  whole-file row `parser: config.yaml command_allowlist is not a flat list (<line>: <problem>)`
  after the entries before it. The nested `approvals` settings (`mode`, `cron_mode`,
  `single_query_mode`, `unattended_mode`, `deny`) need a YAML reader and are not read, and a
  profile served by the remote config backend has no file to read.
- From `pairing/<platform>-approved.json` or `platforms/pairing/<platform>-approved.json` (the
  newer location, [`gateway/pairing.py:58-59`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/gateway/pairing.py#L58-L59)), one row per user
  at `approved_at`, `permission setting: pairing.<platform>.<user_id> user_name=<user_name> (from
  pairing/<platform>-approved.json, approved_at)`, or `(from platforms/pairing/...` for the newer
  file. The pending files hold live pairing codes and are not read. Both files are flagged
  secret, so a `--no-secrets` collection gives none of these rows.

#### Model rows

`system` rows after each session's start row: the session's `session_model_usage` rows with
`task` empty, ordered by `first_seen`, give `model: <model>` for the first and
`model: <old> -> <new>` for each later one, at `first_seen`, with ` provider=<billing_provider>`
when it is set; auxiliary task rows are skipped. The table aggregates per model, so a switch back
to a model already used is not visible. A session with no such rows gets one
`model: <sessions.model>` row at `started_at`. The `model` column of the other rows stays
`sessions.model` ([Model usage](#model-usage)).

Model-switch markers, `system`, in message order: each `messages` row with
`display_kind = 'model_switch'` is one row `model: <old> -> <model> provider=<provider>` at the
row's `timestamp`, with `model` set to the new model and `source_line` to `messages.id`, parsed
from the marker text ([Model-switch markers](#model-switch-markers); the provider part only when
one was set). The marker names only the new model: `<old>` is the session's previous marker's
model, else the last main-loop `session_model_usage` model whose `first_seen` is before the
marker; with neither, or when it is the new model, the row is `model: <model>` with the provider
suffix. A later switch sets the marker's `active` to 0, which is no rewind, so the row has no
`[rewound]` prefix, and a marker is never a `user` row. A marker whose text has another form is a
`system` row `model_switch: <text>`. These rows sit beside the `session_model_usage` rows, which
show the first API call on each model rather than the switch.

Proposed, not implemented: every other non-empty `display_kind` on a `user` row becomes a
`system` row prefixed with the kind, so injected notices are not counted as typed prompts, and a
`session_meta` row is skipped like Hermes' own readers skip it.

#### Past-use rows and secondary sources

- The snapshots, the diverts, `.hermes_history`, the cron run files, `response_store.db` and the
  trajectory files are secondary sources, with the rows the source table gives. A history entry
  pairs with the session's `user` row on text (the TUI's within 2 seconds, the classic CLI's
  within the host's zone offset); a slash command, which no store repeats, is a `system` row. A
  cron run's prompt and response pair with the `source='cron'` session's turns when the text
  matches; a response-store message and a trajectory turn carry no time and pair on text within
  the session or the project.
- Imported sessions are read as Hermes sessions
  ([What survives a delete](#what-survives-a-delete)). The start row of a session whose
  `origin_json` has `imported_from` ends ` | imported from <tool> <path> session
  <foreign_session_id>`, since its turns carry the import's time
  ([Imported sessions](#imported-sessions)); a CLI import has no `origin_json` and gets no such
  suffix.
- `conversation_generations`: one `system` row per row, `relay peer: <source> <session_key>
  generation <n> (from state.db conversation_generations)`, with no time and no session, which
  names chats whose sessions are gone.
- `hosted_room_events`: one `system` row per event at its `created_at`, `bot mode room <name>
  (<room_id>) seq <seq> <kind> actor <actor kind>:<actor id> (<display_name>): <text>` for a
  `message.user` or `message.member` event and the payload as compact JSON for the other kinds
  ([Bot Mode group chats](#bot-mode-group-chats)); never a `user` row, since whether a room
  message is also a `messages` row was not determined.

#### Sync, relay and origin rows

Sync: Hermes records no sync state for a session. Nothing uploads a session, a transcript or a
title to the Nous account: the Nous Portal (Nous Research's account and billing site,
`portal.nousresearch.com`) is used for sign-in, inference and the tool gateway
([`hermes_cli/portal_cli.py:10-20`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/portal_cli.py#L10-L20)), and a grep of the tree
for session upload, sync, share and cloud-session names found nothing beyond the trace upload and
the paths listed next. `hermes sessions delete` removes the rows and the session's files and
calls nothing remote
([`hermes_state_sessions.py:1702-1746`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_state_sessions.py#L1702-L1746)).

One command uploads a session: `hermes sessions export --format trace --upload [--public]`
converts a session to Claude Code JSONL and uploads it to the user's Hugging Face dataset
`<user>/hermes-traces` as `sessions/<session id>.jsonl`, private unless `--public`, and writes
nothing locally on success
([`agent/trace_upload.py:1-5`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/agent/trace_upload.py#L1-L5),
[`agent/trace_upload.py:209-221`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/agent/trace_upload.py#L209-L221),
[`hermes_cli/sessions_cmd.py:497-516`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/sessions_cmd.py#L497-L516)); the only local
traces are the command in the shell's own history and `HF_TOKEN` in `.env`, so no
`session synced:` row is possible.

Content also leaves the host on these opt-in paths, none of which marks a session:

- `hermes debug share` (CLI), `/debug` (gateway) and the dashboard's
  `POST /api/ops/debug-share` upload a report and the logs, whose own notice says they hold
  "Verbatim content of your recent messages", to paste.rs, falling back to dpaste.com
  ([`hermes_cli/debug.py:120-137`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/debug.py#L120-L137),
  [`hermes_cli/debug.py:470-507`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/debug.py#L470-L507),
  [`gateway/slash_commands.py:1274-1304`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/gateway/slash_commands.py#L1274-L1304),
  [`hermes_cli/web_routers/status.py:867-878`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/web_routers/status.py#L867-L878)).
  The CLI asks first unless `--yes` is given. The paste.rs links stay in `pastes/pending.json` for
  6 to 30 hours ([Debug-share paste list](#debug-share-paste-list)), and with redaction on (the
  default) `agent.log` gets the line `hermes debug share: applied force-mode redaction to log
  snapshots before upload`
  ([`hermes_cli/debug.py:479-481`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/debug.py#L479-L481)). `hermes debug share
  --nous` sends the same bundle to Nous's private diagnostics storage (an S3 bucket behind
  `portal.nousresearch.com`, kept 14 days) and only prints the view URL; its local trace is the
  log line `hermes debug share --nous: applied force-mode redaction before upload`
  ([`hermes_cli/diagnostics_upload.py:1-13`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/diagnostics_upload.py#L1-L13),
  [`hermes_cli/debug.py:606-633`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/debug.py#L606-L633)). `hermes debug delete <url>`
  deletes a paste.rs paste and does not touch `pending.json`
  ([`hermes_cli/debug.py:646-666`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/debug.py#L646-L666)).
- An external memory provider, set in `config.yaml` `memory.provider` (empty by default; RetainDB
  and ByteRover bundled, Honcho, Hindsight, Supermemory, Mem0, OpenViking and Holographic from the
  plugin catalog;
  [`hermes_cli/config_defaults.py:1309-1312`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/config_defaults.py#L1309-L1312)),
  receives every completed turn's user and assistant text with the session id, passed through
  Hermes' secret redactor first
  ([`agent/memory_manager.py:441-453`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/agent/memory_manager.py#L441-L453)): Mem0 (a hosted
  memory service, or self-hosted;
  [`plugin-catalog/mem0.yaml:1-10`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/plugin-catalog/mem0.yaml#L1-L10)), RetainDB through
  `retaindb_queue.db`
  ([`plugins/memory/retaindb/__init__.py:406-413`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/plugins/memory/retaindb/__init__.py#L406-L413),
  [RetainDB queue](#retaindb-queue)), ByteRover and OpenViking (local servers with an optional
  cloud), and the other catalog-installed providers, whose code is not in this repository. The
  local traces are the provider name in `config.yaml`, its settings (`mem0.json`, `honcho.json`)
  and its key in `.env`; no file records which turns were sent, and deleting a session does not
  delete the provider's copy.
- The Langfuse plugin (Langfuse is an LLM tracing service, hosted or self-hosted), installed from
  the plugin catalog and off unless listed in `plugins.enabled`
([`hermes_cli/plugins.py:1341`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/plugins.py#L1341)), exports one trace per turn with content sanitized by
  default and writes no local file
  ([`plugin-catalog/langfuse.yaml:1-9`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/plugin-catalog/langfuse.yaml#L1-L9)).
- Shared metrics (`config.yaml` `telemetry.shared_metrics.enabled` and `.send`, both off by
  default) are counters only, "Never prompts, files, paths", kept in `telemetry/shared_metrics/`
  and sent daily to `telemetry.nousresearch.com` when `send` is on
  ([`hermes_cli/config_defaults.py:2319-2331`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/config_defaults.py#L2319-L2331),
  [`hermes_cli/observability/shared_metrics_consent.py:33-40`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/observability/shared_metrics_consent.py#L33-L40),
  [`hermes_cli/observability/shared_metrics.py:212-214`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/observability/shared_metrics.py#L212-L214)).

The parser yields no `session synced:` row and leaves `records_sync` off. It reads
`pastes/pending.json` and yields one `system` row per entry, `debug report uploaded: <url> (from
pastes/pending.json, expire_at minus 6 hours)`, timestamped `expire_at` minus 21600 seconds with
an empty `session_id`, which shows that logs left the host in the last day and marks no session;
bad entries are under [Bad-line rows](#bad-line-rows).

Relay: a gateway session's messages also live on the messaging platform, which is the channel the
user typed in, not a sync; `sessions.chat_id`, `origin_json` and `messages.platform_message_id`
name them there ([Messaging sessions](#messaging-sessions)). A session whose `source` is a
messaging platform (every gateway platform but `local` and `api_server`), or that has a `chat_id`
and another `source` than those two (a plugin platform), gets `session relayed: <source> chat
<chat_id> type <chat_type> thread <thread_id> user <user_id> name <user_name> display
<display_name> (from state.db sessions, started_at)` after its model rows, `user_name` from
`origin_json` and `display_name`, else `origin_json` `chat_name`; the parser sets
`records_relay`.

Origin: the desktop app's `connections.json` registry lists `local`, `remote` (a `hermes serve`
the user runs), `ssh` and `cloud` (Hermes Cloud, Nous Research's hosted Hermes) connections; a
session on a remote, SSH or cloud backend lives in that host's `state.db`, and whether the desktop
keeps any local copy was not determined
([`apps/desktop/electron/main.ts:1320-1325`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/apps/desktop/electron/main.ts#L1320-L1325),
[`apps/desktop/electron/connection-registry.ts:1-10`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/apps/desktop/electron/connection-registry.ts#L1-L10),
[`apps/desktop/electron/connection-registry.ts:44-47`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/apps/desktop/electron/connection-registry.ts#L44-L47)).
The parser reads `connections.json`
([Desktop connection registry](#desktop-connection-registry)) and yields one `system` row per
connection that is not `local`, `remote backend: <kind> <label> <url, or user@host:port for ssh>
(from connections.json, file mtime)` followed by ` org=<org>`, ` name=<name>` and
` profile=<remoteProfile>` when recorded, and `remote backend quarantined by the app: <reason>
(from connections.json, file mtime)` per quarantined entry, with no session (`session origin:`
needs one) and never printing `token` or `headers`. It sets no `records_origin`. The
single-connection `connection.json` is not read.

#### Scratch directory rows

The scratch directories are `cache/scratch/` and `sandboxes/` under the Hermes home and the git
worktrees under `<repo>/.worktrees/` ([Scratch directories](#scratch-directories)).

`cache/scratch` and `sandboxes` are under the catalog's Hermes home, so the analyzer's catalog
rule classes any path under it as a scratch directory. A `.worktrees/<name>` path is not under a
home entry: a session whose `cwd` is `<repo>/.worktrees/<name>` and whose `git_branch` starts with
`hermes/` or `hermes-subagent/` gets `scratch directory: <cwd> (git worktree the agent created in
<repo>, branch <git_branch>)` after its model rows, at `started_at`. A classic `-w` session, whose
row holds the repository root, and a kanban child's worktree, whose branch the source does not
fix, get none.

#### Deletion rows

One `session deleted: <id> (from state.db state_meta gateway.retired_session.v1, retired_at)` row
per marker, at `retired_at`, with ` sweep` after the id when it has `expires`, and with the
marker's id as `session_id`; the start row of an archived session ends ` | archived=1`. The
markers are described under [Deletion fences](#deletion-fences). The `gateway.retired_route.v1.`
fences are not read.

#### Bad-line rows

A `state.db`, a snapshot or a `response_store.db` that cannot be read, or whose `-wal`, `-shm` or
`-journal` sidecar cannot, is one row `parser: <file> could not be read (<reason>)`, `<file>` the
name of the file that failed; one that is not a SQLite database is `parser: not a SQLite database`,
one SQLite cannot read is `parser: SQLite error: <message>`, and a `response_store.db` with no
`responses` table is `parser: no responses table`; a `state.db` with no `messages` table gives no
rows.

`sessions/<id>.jsonl` goes through `iter_jsonl` and `error_rows`. A `sessions/<id>.jsonl`,
`pastes/pending.json`, `config.yaml`, pairing file, `connections.json`, cron run file,
`trajectory_samples.jsonl` or `failed_trajectories.jsonl` that cannot be read is one whole-file row
`parser: <file> could not be read (<reason>)`, `<file>` its name, with `source_line` 0 and no time;
a trajectory file's row carries its project as `project_path`, and a cron run file keeps its
`cron run:` row beside it. In `pastes/pending.json` a file that does not decode as JSON or is not
a JSON list is one `parser:` row, and an entry that is not an object with a `url` is its own
`parser: unparseable record <N>` row carrying the entry.

In `state.db` a `sessions.origin_json` or `model_config`, a `state_meta` fence value, or a
`hosted_room_events` `actor_json` or `payload_json` that does not decode to a JSON object is a row
`parser: unparseable record <rowid> (<table>.<column>: <reason>): <value as stored>`, with the
session's id where there is one, after the file's other rows. In `response_store.db` a `data`
value is reported the same way (`responses.data`), and a history entry that is not an object as
`(responses.data conversation_history <index>: not an object)`. `.hermes_history` goes through
the Aider parser's `read_input_history`, whose bad line is `parser: unparseable line (not a
FileHistory line): <line>`; a trajectory file's line that does not decode, is not an object or has
no `conversations` list is `parser: unparseable line (<reason>): <line>`. A pairing file or
`connections.json` that does not decode as JSON is one whole-file row and an entry that is not an
object is its own `unparseable record <N> (not an object)` row; `config.yaml` is under
[Permission rows](#permission-rows).

Malformed `tool_calls`: a value that is not valid JSON, is not a list (a lone object is read as a
one-call list) or holds an entry that is not an object gives one row through `error_rows`, after
the message's other rows, `parser: unparseable record <messages.id> (tool_calls is not valid JSON:
<reason>)`, `(tool_calls is not a list)` or `(tool_calls entry is not an object)`, followed by
`: ` and the value as stored, with the message's `session_id` and project path and an empty
`timestamp_utc`; the entries that are objects still give `tool_use` rows. A divert line uses the
same rule, with `<N>` the line number and the value written as compact JSON when it is not a
string.

#### Fixtures

A `state.db` reduced to the columns the parser reads, as the schema at `6bb377a` writes them
([Sessions table](#sessions-table), [Messages table](#messages-table)); the assistant row's
`tool_calls` follows the writer of
[`agent/chat_completion_helpers.py:1609-1611`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/agent/chat_completion_helpers.py#L1609-L1611)
and the tool result the terminal envelope of
[`tools/terminal_tool.py:1044-1050`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/tools/terminal_tool.py#L1044-L1050):

```sql
CREATE TABLE schema_version (version INTEGER NOT NULL);
INSERT INTO schema_version VALUES (32);
CREATE TABLE sessions (id TEXT PRIMARY KEY, source TEXT NOT NULL, model TEXT,
  parent_session_id TEXT, started_at REAL NOT NULL, ended_at REAL, end_reason TEXT,
  cwd TEXT, git_branch TEXT, git_repo_root TEXT, title TEXT, archived INTEGER NOT NULL DEFAULT 0);
CREATE TABLE messages (id INTEGER PRIMARY KEY AUTOINCREMENT, session_id TEXT NOT NULL,
  role TEXT NOT NULL, content TEXT, tool_call_id TEXT, tool_calls TEXT, tool_name TEXT,
  timestamp REAL NOT NULL, finish_reason TEXT, reasoning TEXT, reasoning_content TEXT,
  active INTEGER NOT NULL DEFAULT 1, compacted INTEGER NOT NULL DEFAULT 0,
  display_kind TEXT);
INSERT INTO sessions VALUES ('20261029_093015_5c7e19a2','cli','openai/gpt-5.1',NULL,
  1793266215.0,NULL,NULL,'/home/u/work/site','fix-nav','/home/u/work/site','Count lines',0);
INSERT INTO messages (session_id,role,content,tool_call_id,tool_calls,tool_name,timestamp,
  finish_reason,reasoning,display_kind) VALUES
 ('20261029_093015_5c7e19a2','user','how many lines in index.html?',NULL,NULL,NULL,
  1793266216.4,NULL,NULL,NULL),
 ('20261029_093015_5c7e19a2','assistant','',NULL,
  '[{"id":"call_9f","call_id":"call_9f","response_item_id":null,"type":"function","function":{"name":"terminal","arguments":"{\"command\":\"wc -l index.html\"}"}}]',
  NULL,1793266217.9,'tool_calls','Count with wc.',NULL),
 ('20261029_093015_5c7e19a2','tool','{"output": "212 index.html", "exit_code": 0, "error": null}',
  'call_9f',NULL,'terminal',1793266218.6,NULL,NULL,NULL),
 ('20261029_093015_5c7e19a2','user','[System: The active model for this chat has changed to anthropic/claude-opus-4.1 via provider openrouter. From this point forward, ...]',
  NULL,NULL,NULL,1793266230.0,NULL,NULL,'model_switch'),
 ('20261029_093015_5c7e19a2','assistant','index.html has 212 lines.',NULL,NULL,NULL,
  1793266231.2,'stop',NULL,NULL);
```

A messaging session with a refused command, a deletion marker and a routing peer, from the
writers of [Messaging sessions](#messaging-sessions),
[Permission outcomes and settings](#permission-outcomes-and-settings),
[Deletion fences](#deletion-fences) and [Conversation generations](#conversation-generations):

```sql
INSERT INTO sessions (id,source,user_id,chat_id,chat_type,display_name,origin_json,model_config,
  started_at,archived) VALUES ('20261028_201500_b41c0de7','discord','88213','55102','channel',
  'build-alerts','{"platform":"discord","chat_id":"55102","chat_name":"build-alerts",
  "chat_type":"channel","user_id":"88213","user_name":"ops_kim"}','{"yolo_mode": false}',
  1793218500.0,0);
INSERT INTO messages (session_id,role,content,tool_call_id,tool_name,timestamp) VALUES
 ('20261028_201500_b41c0de7','tool','{"output": "", "exit_code": -1, "error": "Command denied:
  recursive delete. Use the approval prompt to allow it, or rephrase the command.", "status":
  "blocked", "user_summary": "rm -rf build/"}','call_c2','terminal',1793218533.5);
INSERT INTO state_meta VALUES ('gateway.retired_session.v1.20261020_081200_77aa01fe',
  '{"retired_at": 1793229100.25, "expires": true, "own": true}');
INSERT INTO conversation_generations VALUES ('discord','agent:main:discord:channel:55102',3);
```

`.hermes_history`, one TUI entry (UTC, milliseconds) and one classic CLI entry (host-local,
microseconds), from the writers of [Prompt history](#prompt-history):

```text

# 2026-10-29 09:30:16.400
+how many lines in index.html?

# 2026-10-29 10:02:44.918203
+/snapshot before-refactor
```

`pairing/discord-approved.json`, from
[`gateway/pairing.py:432-438`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/gateway/pairing.py#L432-L438):

```json
{"88213": {"user_name": "ops_kim", "approved_at": 1793210400.0}}
```

`pastes/pending.json`, from
[`hermes_cli/debug.py:180-195`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/hermes_cli/debug.py#L180-L195):

```json
[{"url": "https://paste.rs/Xq3vT", "expire_at": 1793289600.5}]
```

#### Source host rows

The agent records no source-host integration: at
[`6bb377a`](https://github.com/NousResearch/hermes-agent/commit/6bb377a51d5c8e279a7a5e34d717d653d421d064)
a grep for `github`, `gitlab`, `bitbucket`, `oauth`, `installation`, `repositor`, `pull request`
and `clone` found no connected account, no cached repository list and no repository-bound remote
conversation. A GitHub token, when present, is a variable in `.env` or the gh CLI's own login
([Credentials](#credentials)), and the webhook platform's GitHub and GitLab routes live in
`config.yaml` `platforms.webhook.extra.routes`
([`gateway/platforms/webhook.py:1-10`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/gateway/platforms/webhook.py#L1-L10)); a webhook-driven
session is a relayed session with `source` `webhook`.

## Confidence

- High: home resolution, profile layout, `state.db` and every file named in
  [Per-user storage](#per-user-storage) and [Credentials](#credentials) (source constants).
- High: the backup exclusion list as a guide to size.
- High: table and column names, timestamp unit, content prefix, tool-call shape, joins, WAL.
- Medium: the desktop `userData` path, which follows the Electron default from `productName`
  rather than an explicit `setPath` for stable builds
  ([`apps/desktop/electron/product-identity.ts:26-47`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/apps/desktop/electron/product-identity.ts#L26-L47)).
- Medium: the full set of `source` values beyond those listed in the
  [Sessions table](#sessions-table) and [Messaging sessions](#messaging-sessions).
- Medium: that a `terminal` result's JSON reaches `messages.content` unchanged; a result Hermes
  spilled or truncated before storing would give no permission decision row.
- For the round: the sizes of `state-snapshots`, `cache/scratch` and the state database copies on
  a real install; whether a classic `-w` session's `sessions.cwd` is the repository root and its
  tree only in the local policy; what a `session_meta` row stores in `content`; whether a CLI
  import of a Claude Code or Codex CLI session is marked anywhere but its title.
- For the round: the Webapp's files beyond `webapp/` and `uploads/`, and whether a session driven
  from the Webapp differs from a desktop one in `state.db`.
- Not determined: the format of `auth.json` beyond "OAuth provider credentials"
  ([`configuration.md:25`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/website/docs/user-guide/configuration.md#L25)).
- Not determined: Termux installs, which come from APT
  ([`scripts/install.sh:439`](https://github.com/NousResearch/hermes-agent/blob/6bb377a51d5c8e279a7a5e34d717d653d421d064/scripts/install.sh#L439)) and were not checked.
- Not determined: the legacy `session_<id>.json` schema, which nothing writes
  ([Transcript stores](#transcript-stores)).
- Not determined: whether a Bot Mode room message is also a `messages` row, whether a cron run's
  stored prompt equals the session's first `user` message, the branch of a kanban child's
  worktree, and what a profile served by the remote config backend keeps on disk.
