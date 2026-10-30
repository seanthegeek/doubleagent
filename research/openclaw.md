# openclaw

OpenClaw (catalog agent `openclaw`, earlier named Clawdbot and then Moltbot) is a self-hosted
agent whose Gateway (the long-running `openclaw gateway` server process that owns every session)
takes messages from chat channels (Telegram, WhatsApp, Slack, Signal, iMessage and others under
`extensions/`) and runs agents with shell, browser and file tools. It ships as a TypeScript CLI and
Gateway with a Swift macOS app and a container image; the Control UI (its web front end), the TUI
and the mobile and desktop apps are clients of the Gateway, not copies of its store, and OpenClaw
has no vendor account. Its transcript entries are the tree-structured session format of pi, whose
agent core OpenClaw carries as its own `packages/agent-core` (see [pi](pi.md#record-schema)); this
document describes only what OpenClaw adds or changes. The entries live as rows of a per-agent
SQLite database; `.jsonl` files remain only as legacy import sources and archives. Three terms run
through the document: a generation is one transcript of a logical session (a reset or fork starts a
new one under the same session key); `openclaw doctor` is the CLI's repair and migration command;
a SecretRef is a reference to a value held in the environment, a file, a command or a secret
store, not the value itself. The latest version reviewed is the `main` branch at
[`fc3490a`](https://github.com/openclaw/openclaw/commit/fc3490aa24e624f0182ae5e29482611fee5736fb),
package version `2026.10.9`. Parsed: the openclaw parser reads the agent databases, the Gateway's
state database and the legacy and archived session files.

## Source and evidence

[openclaw/openclaw](https://github.com/openclaw/openclaw), MIT, TypeScript with a Swift macOS app,
reviewed at commit
[`fc3490aa24e624f0182ae5e29482611fee5736fb`](https://github.com/openclaw/openclaw/commit/fc3490aa24e624f0182ae5e29482611fee5736fb)
(2026-10-30 UTC, the default branch `main`). Its `package.json` gives version `2026.10.9`, state
schema 36 and agent schema 38
([`package.json:3-20`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/package.json#L3-L20)). That
commit is ahead of the release tag: the tag `v2026.10.9`
([`8b18b55`](https://github.com/openclaw/openclaw/commit/8b18b5578f66eab1501cba4cef2310d88bfb9a41),
2026-10-29) sits on a release branch whose `package.json` gives state schema 20 and agent schema 25
([`package.json:5-9` at the tag](https://github.com/openclaw/openclaw/blob/8b18b5578f66eab1501cba4cef2310d88bfb9a41/package.json#L5-L9)),
and the schema histories list the newest changes as unreleased
([state history](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/docs/reference/database-schemas/state-schema-history.md#L52-L61),
[agent history](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/docs/reference/database-schemas/agent-schema-history.md#L43-L57)).
[Format versions](#format-versions) says what the tag writes differently.

Every claim is source-level at `fc3490a` unless it cites a page under `docs/` or `CHANGELOG/` in
the same checkout; such a page is the vendor's own and is linked at the claim. Facts about older
builds cite
[`146284a`](https://github.com/openclaw/openclaw/commit/146284a289f98a2cf3babbc55cc3bd16c28e604b)
(2026-10-19, version `2026.10.5` on `main`) or
[`3b16db7`](https://github.com/openclaw/openclaw/commit/3b16db73c0b5209aad2ee7c3fb5ddfb12fb208f9)
(2026-10-04, version `2026.9.8`) and appear only in [Format versions](#format-versions).

### Observed

- No real install. The document rests on source and the vendor's pages at `fc3490a` (version
  `2026.10.9` on `main`), with the release tag `v2026.10.9` read for its schema. Every row of the
  [Parser plan](#parser-plan) is source-derived; the public-share, backup-upload, permission,
  relay, pending-input, deletion, PostgreSQL-anchor, GitHub publication and `git remote:` rows
  are fixture only, and no real install has shown the legacy `sessions.json` index or a JSON line
  that is not an object.

### Source reviews

- 2026-10-30: [openclaw/openclaw](https://github.com/openclaw/openclaw) at
  [`fc3490a`](https://github.com/openclaw/openclaw/commit/fc3490aa24e624f0182ae5e29482611fee5736fb)
  (version `2026.10.9`, state schema 36, agent schema 38), with the release tag `v2026.10.9`: the
  document was rewritten from this commit. It adds the local client credential store
  `client/credentials.sqlite`, the legacy identity files, the auth-source and approvals archives,
  the cold archive chunk files, the memory chunk session paths and key tables for every store the
  parser reads; the runtime no longer falls back to `~/.clawdbot`, and the exec socket token and
  device identities moved out of the state database ([Format versions](#format-versions)).
- 2026-10-21: [openclaw/openclaw](https://github.com/openclaw/openclaw) at
  [`146284a`](https://github.com/openclaw/openclaw/commit/146284a289f98a2cf3babbc55cc3bd16c28e604b)
  (version `2026.10.5`): the relay, permission, model-override, past-use, deletion,
  PostgreSQL-anchor and GitHub publication records became parser rows; the credential and
  exclusion sections gained the exec approvals file, the Docker secrets directory and the clone
  exclusion.
- 2026-10-19: [openclaw/openclaw](https://github.com/openclaw/openclaw) at
  [`146284a`](https://github.com/openclaw/openclaw/commit/146284a289f98a2cf3babbc55cc3bd16c28e604b)
  (shared-state schema 28, agent schema 31), against
  [`3b16db7`](https://github.com/openclaw/openclaw/commit/3b16db73c0b5209aad2ee7c3fb5ddfb12fb208f9)
  (state 20, agent 24): added the permission, model override, past-use, relay and deletion
  records, the PostgreSQL engine, Logbook, Docker and `projects/` paths.

## Collection

### Per-user storage

Home is `$OPENCLAW_HOME`, else `HOME`, else `USERPROFILE`
([`home-dir.ts:28-52`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/packages/normalization-core/src/home-dir.ts#L28-L52)).
The state directory is `$OPENCLAW_STATE_DIR`, else `~/.openclaw-<profile>` for a named profile
(`OPENCLAW_PROFILE=work`; the profile `default` keeps `~/.openclaw`), else `~/.openclaw`
([`src/config/state-dir.ts:23-54`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/state-dir.ts#L23-L54);
[`src/cli/profile-utils.ts:26-37`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/cli/profile-utils.ts#L26-L37)).
A remote worker machine keeps prepared workspaces under `~/.openclaw-worker`
([`project-setup-script.ts:202`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/gateway/worker-environments/project-setup-script.ts#L202)).
There is no platform branch: the same tree is used on Linux, macOS and Windows
(`%USERPROFILE%\.openclaw`). Outside the state directory OpenClaw reads one XDG-style file,
`~/.config/openclaw/gateway.env` ([Credentials](#credentials)).

#### Scratch directories

OpenClaw works in directories of its own, all under the state directory: the default agent
workspace `workspace/` and `workspace-<agentId>/` (the transcript header `cwd` defaults to it),
managed git worktrees in `worktrees/`, repositories the Gateway cloned in
`projects/<16 hex>/<name>`, the hex being the first 16 characters of the SHA-256 of the clone URL
([`src/projects/project-clone.ts:72`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/projects/project-clone.ts#L72),
[`:143`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/projects/project-clone.ts#L143)),
and sandbox workspaces in `sandboxes/`
([`agents/sandbox/constants.ts:8`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/agents/sandbox/constants.ts#L8)).
`state/openclaw.sqlite` records which is which
([Where the project path is recorded](#where-the-project-path-is-recorded)). A cloud worker
session's workspace is on the worker, not on this host. The temp root `/tmp/openclaw` (else a
per-user `os.tmpdir()` directory,
[`infra/tmp-openclaw-dir.ts:6`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/infra/tmp-openclaw-dir.ts#L6))
is outside the home and holds the rolling logs and Logbook's capture directory on the capturing
device.

#### Contents of the state directory

The vendor's state layout table
([`docs/openclaw-agent-runtime.md:49-68`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/docs/openclaw-agent-runtime.md#L49-L68))
and the source give:

- `openclaw.json`, the config, JSON5 (JSON with comments and unquoted keys)
  ([`config/paths.ts:35`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/paths.ts#L35),
  [`:241`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/paths.ts#L241);
  [`io.read-helpers.ts:137-142`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/io.read-helpers.ts#L137-L142)),
  `OPENCLAW_CONFIG_PATH` moving it. Siblings: `.bak` and `.bak.<n>` rotation, `.clobbered.<ts>`,
  `.rejected.<ts>` and `.last-good`
  ([`backup-rotation.ts:189`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/backup-rotation.ts#L189),
  [`io.clobber-snapshot.ts:203`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/io.clobber-snapshot.ts#L203),
  [`io.write-rejected.ts:28`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/io.write-rejected.ts#L28),
  [`io.observe-recovery.ts:493`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/io.observe-recovery.ts#L493)).
  Legacy name `clawdbot.json`
  ([`state-migrations.state-dir.ts:333-344`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/infra/state-migrations.state-dir.ts#L333-L344)).
- `state/openclaw.sqlite`, the Gateway's shared state database
  ([`openclaw-state-db.paths.ts:41`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/state/openclaw-state-db.paths.ts#L41);
  schema [`src/state/openclaw-state-schema.sql`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/state/openclaw-state-schema.sql)):
  audit events, inbound channel messages, approvals and exec policy, cron jobs (`cron_jobs`),
  subagent runs (`subagent_runs`), projects, worktrees, backups, GitHub publications and the
  secret store. Its records are in [Other stores](#other-stores).
- `client/credentials.sqlite`, the local client credential store
  ([`client-credential-paths.ts:10-12`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/infra/client-credential-paths.ts#L10-L12)),
  kept local on every database engine
  ([`docs/gateway/postgresql.md:188`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/docs/gateway/postgresql.md#L188)); a credential file
  ([Credentials](#credentials)).
- `agents/<agentId>/agent/openclaw-agent.sqlite`, one database per agent holding sessions,
  transcripts, the memory index and auth profiles
  ([`agent-database-path-discovery.ts:23`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/state/agent-database-path-discovery.ts#L23);
  schema [`src/state/openclaw-agent-schema.sql`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/state/openclaw-agent-schema.sql));
  `agentDir` is configurable per agent
  ([`agents/agent-scope-config.ts:623-634`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/agents/agent-scope-config.ts#L623-L634)).
  Beside it `models.json`, the agent's model catalog
  ([`agents/models-config.ts:248`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/agents/models-config.ts#L248)).
- `agents/<agentId>/sessions/`
  ([`config/sessions/paths.ts:51-61`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/sessions/paths.ts#L51-L61)):
  legacy `sessions.json` and `<sessionId>.jsonl` transcripts awaiting `doctor --fix` import, the
  reset and delete archives, checkpoints, trajectories, rollback copies and the `cold/`
  subdirectory named in [Transcript stores](#transcript-stores). `sessions/` at the state root is
  the older single-agent layout.
- `workspace/` (default agent) and `workspace-<agentId>/` (others), unless
  `OPENCLAW_WORKSPACE_DIR` or config moves them
  ([`agents/workspace-default-path.ts:8-25`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/agents/workspace-default-path.ts#L8-L25),
  [`agent-scope-config.ts:507-531`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/agents/agent-scope-config.ts#L507-L531)).
  A workspace holds `AGENTS.md`, `SOUL.md`, `TOOLS.md`, `IDENTITY.md`, `USER.md`, `BOOTSTRAP.md`,
  `MEMORY.md`
  ([`agents/workspace-bootstrap-policy.ts:11-37`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/agents/workspace-bootstrap-policy.ts#L11-L37)),
  legacy `memory.md`
  ([`memory/root-memory-files.ts:6-8`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/memory/root-memory-files.ts#L6-L8)),
  `memory/` and `skills/<name>/SKILL.md`
  ([`agents/workspace.ts:180`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/agents/workspace.ts#L180),
  [`:206`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/agents/workspace.ts#L206)),
  and is a git repository
  ([`workspace-git.ts:83`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/agents/workspace-git.ts#L83)).
  These are the agent's instructions and memory, prime evidence of prompt injection or
  persistence.
- `credentials/` (`$OPENCLAW_OAUTH_DIR` overrides,
  [`config/paths.ts:315-324`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/paths.ts#L315-L324)), and `identity/`, both of
  credentials ([Credentials](#credentials)).
- `skills/`, managed skills from the marketplace
  ([`skills/loading/workspace-skill-sources.ts:100`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/skills/loading/workspace-skill-sources.ts#L100));
  `~/.agents/skills` is also read
  ([`:150`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/skills/loading/workspace-skill-sources.ts#L150)).
  `extensions/`, installed plugins, and `plugin-skills/`, a generated symlink index
  ([`docs/cli/backup.md:848-850`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/docs/cli/backup.md#L848-L850)).
- `logs/`: `commands.log`
  ([`hooks/bundled/command-logger/handler.ts:19-22`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/hooks/bundled/command-logger/handler.ts#L19-L22)),
  `config-audit.jsonl`
  ([`state-migrations.audit-checkpoints.ts:169`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/infra/state-migrations.audit-checkpoints.ts#L169)),
  and debug traces that hold full prompts: `raw-stream.jsonl`, `anthropic-payload.jsonl`,
  `cache-trace.jsonl`
  ([`embedded-agent-subscribe.raw-stream.ts:19`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/agents/embedded-agent-subscribe.raw-stream.ts#L19),
  [`anthropic-payload-log.ts:85`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/agents/anthropic-payload-log.ts#L85),
  [`cache-trace.ts:89`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/agents/cache-trace.ts#L89)).
- `browser/<profile>/user-data/`: the Chromium profile the agent drives
  ([`extensions/browser/src/browser/chrome.ts:512`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/extensions/browser/src/browser/chrome.ts#L512)).
- `worktrees/`, managed git worktrees
  ([`state-migrations.doctor-discovery.ts:53`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/infra/state-migrations.doctor-discovery.ts#L53)),
  `delivery-queue-media/`
  ([`config/paths.ts:310-311`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/paths.ts#L310-L311)),
  `cron/runs/`, `delivery-queue/` and `sandbox/`, the sandbox registry
  ([`docs/cli/backup.md:780-787`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/docs/cli/backup.md#L780-L787)),
  and `service-env/` ([Credentials](#credentials)).
- Runtime roots the installer manages: `dev/`, `git/`, `npm/`, `npm-runtime/`, `tmp/` and `tools/`
  (managed checkouts, package trees, compiler caches, temporary files and downloaded runtimes such
  as signal-cli and text-to-speech models), each at the top of the state directory
  ([`commands/backup-resource-inventory.ts:78`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/commands/backup-resource-inventory.ts#L78);
  [`docs/cli/backup.md:862-866`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/docs/cli/backup.md#L862-L866)).
  The state code writes its own temporary files under the top-level `tmp/`
  ([`config/paths.ts:302`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/paths.ts#L302),
  [`plugins/plugin-source-capture-path.ts:11`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/plugins/plugin-source-capture-path.ts#L11))
  and its tools under the top-level `tools/`
  ([`skills/lifecycle/install.ts:596`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/skills/lifecycle/install.ts#L596)).
  A directory of one of these names deeper in the tree, such as a workspace's `tmp/` or `tools/`,
  is not one of these roots: it is whatever the agent or the user put in the workspace.
- `control/services.sqlite`: daemon runtime pins, kept locally on every database engine
  ([`docs/reference/database-engines.md:24-28`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/docs/reference/database-engines.md#L24-L28)).
- `plugins/workboard/workboard.sqlite`, the Workboard plugin's task board
  ([`extensions/workboard/src/sqlite-store.ts:31`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/extensions/workboard/src/sqlite-store.ts#L31)).
- With the PostgreSQL engine, `state/openclaw.sqlite` and each `openclaw-agent.sqlite` are small
  SQLite anchors and the stores they name are on a database server
  ([PostgreSQL engine](#postgresql-engine)).

#### Outside the state directory

- Rolling logs `openclaw[-<profile>]-YYYY-MM-DD.log` in `/tmp/openclaw` or a per-user
  `os.tmpdir()` fallback, on Windows under `AppData/Local/Temp`
  ([`logging/log-file-path.ts:43-50`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/logging/log-file-path.ts#L43-L50)).
- macOS app: `~/Library/Application Support/OpenClaw/databases/`, its chat transcript cache
  (`databases/gateway-cache.sqlite` and `databases/client-state.sqlite`,
  [`ClientDatabases.swift:35-36`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/apps/shared/OpenClawKit/Sources/OpenClawChatUI/ClientDatabases.swift#L35-L36)),
  and `OpenClaw/canvas`
  ([`ChatTranscriptCacheSupport.swift:121-126`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/apps/macos/Sources/OpenClaw/ChatTranscriptCacheSupport.swift#L121-L126),
  [`CanvasManager.swift:19-22`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/apps/macos/Sources/OpenClaw/CanvasManager.swift#L19-L22)).
- Persistence: `~/Library/LaunchAgents/ai.openclaw.gateway.plist` and `ai.openclaw.node.plist`,
  systemd user units `~/.config/systemd/user/openclaw-gateway.service` and `openclaw-node.service`,
  the legacy unit `~/.config/systemd/user/clawdbot-gateway.service`, and Windows scheduled tasks
  `OpenClaw Gateway` and `OpenClaw Node` running `<state>/gateway.cmd`
  ([`daemon/constants.ts:4-6`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/daemon/constants.ts#L4-L6),
  [`:28-34`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/daemon/constants.ts#L28-L34);
  [`launchd-service-files.ts:55`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/daemon/launchd-service-files.ts#L55);
  [`systemd-service-files.ts:51`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/daemon/systemd-service-files.ts#L51);
  [`daemon/paths.ts:50-61`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/daemon/paths.ts#L50-L61)).

#### Logbook

Logbook, a plugin off by default that takes periodic screen snapshots from a paired device and
summarises them into a timeline, keeps `logbook/logbook.sqlite` and the image files
`logbook/frames/`
([`extensions/logbook/openclaw.plugin.json:2-11`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/extensions/logbook/openclaw.plugin.json#L2-L11)),
both under the state directory. The frames are screenshots of the user's screen and can be large.

#### Docker

The compose file bind-mounts `~/.openclaw` at `/home/node/.openclaw`, its `workspace` at
`/home/node/.openclaw/workspace`, and a second host directory, `~/.openclaw-auth-profile-secrets`,
at `/home/node/.config/openclaw`
([`docker-compose.yml:46-49`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/docker-compose.yml#L46-L49)),
which is where `gateway.env` is read
([`infra/dotenv-global-core.ts:181-183`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/infra/dotenv-global-core.ts#L181-L183)).
The host directory `.openclaw-auth-profile-secrets` sits beside the state directory, so it
matches the profile-directory form `.openclaw-*`; its contents are credentials
([Credentials](#credentials)).

The setup script can also persist the container's whole home, `/home/node`, in a named Docker
volume chosen with `OPENCLAW_HOME_VOLUME` (the guide's example name is `openclaw_home`), with the
three bind mounts above laid over it
([`scripts/docker/setup.sh:592-602`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/scripts/docker/setup.sh#L592-L602);
[`docs/install/docker/networking-and-storage.md:46-62`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/docs/install/docker/networking-and-storage.md#L46-L62)).
The volume therefore holds no OpenClaw state: it keeps a Claude Code install made inside the
container, `.local/bin/claude`, `.local/share/claude`, `.claude/` and `.claude.json`
([`networking-and-storage.md:100`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/docs/install/docker/networking-and-storage.md#L100)),
which are another agent's files.

#### Legacy names

`~/.clawdbot` is the one legacy state directory the code knows
([`src/config/state-dir.ts:10-12`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/state-dir.ts#L10-L12)). The runtime does not
read it: `openclaw doctor` renames it to `~/.openclaw` when `~/.openclaw` is absent and no
explicit state, home or config path is set, and renames `clawdbot.json` inside it to
`openclaw.json`; it leaves no link behind, and when both directories exist it changes neither
([`state-migrations.state-dir.ts:184-199`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/infra/state-migrations.state-dir.ts#L184-L199),
[`:273-331`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/infra/state-migrations.state-dir.ts#L273-L331);
[`docs/cli/doctor/state-migrations.md:114-128`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/docs/cli/doctor/state-migrations.md#L114-L128)).
A host that never ran doctor after upgrading keeps its state in `~/.clawdbot`, which the current
tool ignores. `~/.moltbot` and `moltbot.json` detection was removed
([`CHANGELOG/2026.2.13.md:119`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/CHANGELOG/2026.2.13.md#L119),
[`CHANGELOG/2026.3.22.md:13`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/CHANGELOG/2026.3.22.md#L13)),
so a `~/.moltbot` on an old host is orphaned state; both legacy directories can hold a `.env`
([Credentials](#credentials)). `CLAWDBOT_*` and `MOLTBOT_*` environment
variables only draw a warning
([`gateway/server-runtime-config.ts:45-47`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/gateway/server-runtime-config.ts#L45-L47)).

### Credentials

Paths are home-relative and the same on Linux, macOS and Windows; `.openclaw` stands for every
state directory form, `.openclaw-<profile>` and the legacy `.clawdbot` and `.moltbot` included
([Legacy names](#legacy-names)). Below each form the credential files are `openclaw.json`,
`clawdbot.json`, `moltbot.json`, `.env`, `credentials/*`, `service-env/*`,
`client/credentials.sqlite`, `identity/*`, `exec-approvals.json`,
`agents/<agentId>/agent/auth-profiles.json`, `agents/<agentId>/agent/auth-state.json`,
`agents/<agentId>/agent/auth.json`, `agents/<agentId>/agent/models.json` and
`browser/<profile>/user-data/<chromium-profile>/Cookies`, each with the copies named below.

- `.openclaw/openclaw.json` holds `gateway.auth.token` and `password`
  ([`config/zod-schema.gateway.ts:254-256`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/zod-schema.gateway.ts#L254-L256)),
  channel bot tokens, provider `apiKey` values, MCP server `env` and `headers`
  ([`config/zod-schema.mcp-server.ts:29-35`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/zod-schema.mcp-server.ts#L29-L35)),
  a GitHub App installation's `privateKey` under `tools.github` or an agent entry's
  `tools.github`
  ([`zod-schema.github-identity.ts:16-43`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/zod-schema.github-identity.ts#L16-L43);
  [`zod-schema.agent-runtime.ts:562`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/zod-schema.agent-runtime.ts#L562),
  [`:629`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/zod-schema.agent-runtime.ts#L629)),
  and with the PostgreSQL engine `database.postgres.connection`, which may be the DSN itself,
  password included
  ([`docs/reference/database-engines.md:90-93`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/docs/reference/database-engines.md#L90-L93)),
  each plaintext or as a SecretRef. Its copies `.openclaw/openclaw.json.bak`,
  `.openclaw/openclaw.json.bak.<n>`, `.openclaw/openclaw.json.clobbered.<ts>`,
  `.openclaw/openclaw.json.rejected.<ts>` and `.openclaw/openclaw.json.last-good`, and the legacy
  `.clawdbot/clawdbot.json`, carry the same.
- `.openclaw/client/credentials.sqlite`, the local client credential store: `device_identities`
  (Ed25519 key pairs, `private_key_pem`), `device_auth_tokens` and `gateway_origin_device_tokens`
  (paired device tokens) and `native_exec_socket_credentials` (`socket_token`, the token of the
  exec approvals socket)
  ([`infra/client-credential-schema.ts:12-54`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/infra/client-credential-schema.ts#L12-L54);
  [`docs/cli/node.md:390-391`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/docs/cli/node.md#L390-L391)).
  The device identity also derives the key that encrypts public share links
  ([`control-ui-public-session-token.ts:259-272`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/gateway/control-ui-public-session-token.ts#L259-L272);
  [`infra/device-identity-store.ts:34-46`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/infra/device-identity-store.ts#L34-L46)).
- Legacy identity files that doctor imports into that store and leaves in place:
  `.openclaw/identity/device.json`, `.openclaw/identity/node-device.json` and
  `.openclaw/identity/share-device.json` (signed device key pairs)
  ([`state-migrations.device-identity-repair.ts:29-31`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/infra/state-migrations.device-identity-repair.ts#L29-L31);
  [`docs/gateway/doctor/state-and-sessions.md:46`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/docs/gateway/doctor/state-and-sessions.md#L46)),
  and `.openclaw/identity/device-auth.json`, `.openclaw/identity/node-device-auth.json` and
  `.openclaw/identity/share-device-auth.json` (device tokens)
  ([`state-migrations.device-auth.ts:32-34`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/infra/state-migrations.device-auth.ts#L32-L34)).
- `.openclaw/exec-approvals.json` (`.openclaw*/exec-approvals.json` across the state directory
  forms), the legacy exec approvals file, whose `socket.token` authenticates to the approvals
  socket
  ([`infra/exec-approvals-config.ts:99-100`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/infra/exec-approvals-config.ts#L99-L100),
  [`:120`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/infra/exec-approvals-config.ts#L120);
  [`infra/exec-approvals-core.ts:232-240`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/infra/exec-approvals-core.ts#L232-L240)).
  Doctor imports it and archives a retired stub's exact bytes as
  `.openclaw/exec-approvals.json.migrated.<sha256>.<unique-id>`
  ([`docs/cli/doctor/state-migrations.md:349`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/docs/cli/doctor/state-migrations.md#L349)),
  which holds the same token.
- Legacy auth sources that doctor imports into the agent database
  (`agents/*/agent/auth-profiles.json` and its siblings under every agent):
  `.openclaw/agents/<agentId>/agent/auth-profiles.json`,
  `.openclaw/agents/<agentId>/agent/auth-state.json`,
  `.openclaw/agents/<agentId>/agent/auth.json` and `.openclaw/credentials/oauth.json`
  ([`agents/auth-profiles/legacy-source-files.ts:19-21`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/agents/auth-profiles/legacy-source-files.ts#L19-L21),
  [`:63-68`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/agents/auth-profiles/legacy-source-files.ts#L63-L68)),
  and their archives `<source>.migrated-<stamp>-<uuid>` and `<source>.sqlite-import.<...>` beside
  them, which keep the original bytes
  ([`commands/doctor-auth-migration-receipts.ts:75`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/commands/doctor-auth-migration-receipts.ts#L75);
  [`legacy-source-files.ts:73-74`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/agents/auth-profiles/legacy-source-files.ts#L73-L74)).
- `.openclaw/agents/<agentId>/agent/models.json` provider entries can carry an `apiKey` that
  doctor migrates
  ([`commands/doctor-model-catalog-credentials.ts:81-95`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/commands/doctor-model-catalog-credentials.ts#L81-L95),
  [`:255-259`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/commands/doctor-model-catalog-credentials.ts#L255-L259)).
- `.openclaw/browser/<profile>/user-data/Default/Cookies`, the cookie store of the Chromium
  profile the agent drives
  ([`extensions/browser/src/browser/chrome.ts:512`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/extensions/browser/src/browser/chrome.ts#L512)),
  holds the session cookies of every site the agent signed into, with its `Cookies-journal`.
- `.openclaw/credentials/` subtrees written by channel plugins: `whatsapp/<accountId>/` session
  keys
  ([`extensions/whatsapp/src/accounts.ts:68`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/extensions/whatsapp/src/accounts.ts#L68)),
  `matrix/`
  ([`extensions/matrix/src/storage-paths.ts:60`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/extensions/matrix/src/storage-paths.ts#L60)),
  `zalouser/`
  ([`extensions/zalouser/src/session-state.ts:41`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/extensions/zalouser/src/session-state.ts#L41)),
  and the browser relay secret `.openclaw/credentials/browser-extension-relay.secret`
  ([`relay-auth.ts:21`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/extensions/browser/src/browser/extension-relay/relay-auth.ts#L21),
  [`:44`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/extensions/browser/src/browser/extension-relay/relay-auth.ts#L44)).
- Legacy state directories keep the credential files of the build that wrote them. Clawdbot
  builds read `.clawdbot/.env`, the state directory's env file, and keep their config in
  `.clawdbot/clawdbot.json`
  ([`src/utils.ts:214-221` at `2026.1.24`](https://github.com/openclaw/openclaw/blob/bcedeb4e1f620a50b6e99f1e2b25cc692f0d7bab/src/utils.ts#L214-L221);
  [`src/infra/dotenv.ts:14-16` at `2026.1.24`](https://github.com/openclaw/openclaw/blob/bcedeb4e1f620a50b6e99f1e2b25cc692f0d7bab/src/infra/dotenv.ts#L14-L16)).
  Moltbot builds read `.moltbot/.env` when `~/.moltbot` exists and `~/.clawdbot` does not, else
  `.clawdbot/.env`, and keep their config in `.moltbot/moltbot.json` or `.clawdbot/moltbot.json`
  ([`src/utils.ts:214-230` at `2026.1.29` Moltbot](https://github.com/openclaw/openclaw/blob/cb4b3f74b54227e5d7d2d4700d5a7a3c0a8a5e8f/src/utils.ts#L214-L230);
  [`src/infra/dotenv.ts:14-16`](https://github.com/openclaw/openclaw/blob/cb4b3f74b54227e5d7d2d4700d5a7a3c0a8a5e8f/src/infra/dotenv.ts#L14-L16);
  [`src/config/paths.ts:19-29`](https://github.com/openclaw/openclaw/blob/cb4b3f74b54227e5d7d2d4700d5a7a3c0a8a5e8f/src/config/paths.ts#L19-L29),
  [`:75-83`](https://github.com/openclaw/openclaw/blob/cb4b3f74b54227e5d7d2d4700d5a7a3c0a8a5e8f/src/config/paths.ts#L75-L83)).
  Each holds what its `.openclaw` counterpart holds.
- `.openclaw/.env`, and `.config/openclaw/gateway.env` when the state directory is the default
  ([`infra/dotenv-global-core.ts:168-185`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/infra/dotenv-global-core.ts#L168-L185);
  [`docs/help/environment.md:27`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/docs/help/environment.md#L27)).
- `.openclaw/service-env/<label>.env` (mode 0600) beside the macOS LaunchAgent holds the service
  secrets
  ([`daemon/launchd-service-files.ts:44-64`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/daemon/launchd-service-files.ts#L44-L64)).
- `.openclaw-auth-profile-secrets/`, the Docker installs' host directory for the container's
  `.config/openclaw` and its `gateway.env` ([Docker](#docker)); every file in it,
  `.openclaw-auth-profile-secrets/*`, is a credential.
  The Docker guide states that OAuth access, refresh and ID tokens are plaintext in SQLite under
  the state directory
  ([`docs/install/docker-vm-runtime.md:46-50`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/docs/install/docker-vm-runtime.md#L46-L50)).
- The two SQLite stores that mix credentials with evidence, `state/openclaw.sqlite` and each
  `openclaw-agent.sqlite` (`agents/*/agent/openclaw-agent.sqlite`), are listed with their
  credential tables in [Secrets](#secrets).
- OpenClaw keeps its own secrets in files and SQLite, but reads Claude Code's macOS keychain item
  `Claude Code-credentials` to reuse that login
  ([`plugin-sdk/provider-auth-claude-compat.ts:15-17`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/plugin-sdk/provider-auth-claude-compat.ts#L15-L17)),
  so a Claude Code token can appear in its auth profiles.
- The managed GitHub identity, the account the Gateway's GitHub tools and publications act as
  ([Source host rows](#source-host-rows)), keeps its OAuth token as a `secret_store_entries` row
  of `state/openclaw.sqlite` named `github-oauth-<32 hex>`
  ([`secret-store-github-names.ts:9`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/secrets/store/secret-store-github-names.ts#L9)).

### Exclusions

The backup command's own skip list is the guide
([`docs/cli/backup.md:848-874`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/docs/cli/backup.md#L848-L874);
[`commands/backup-resource-inventory.ts:161-167`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/commands/backup-resource-inventory.ts#L161-L167)):
the top-level runtime roots `dev/`, `git/`, `npm/`, `npm-runtime/`, `tmp/` and `tools/`,
`extensions/*/node_modules`, `plugin-skills/`, and the agent temporary trees
`agents/<agentId>/agent/**/{tmp,.tmp}/` (`agents/*/agent/**/tmp`, among them `agents/*/agent/tmp`
and `agents/*/agent/.tmp`). The archive also leaves out `sandbox/skills-workspaces/` (generated
copies of the current skill sources) and `cache/control-ui-assets`
([`infra/backup-volatile-filter.ts:7-9`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/infra/backup-volatile-filter.ts#L7-L9);
[`docs/cli/backup.md:787-796`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/docs/cli/backup.md#L787-L796)). The backup command omits the runtime roots only at the
top of the state directory, and says names such as `tmp` and `.tmp` are not blanket exclusions
in custom agent directories
([`docs/cli/backup.md:872-874`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/docs/cli/backup.md#L872-L874));
a workspace's `tmp/` or `tools/` is the agent's own work. Add `worktrees/` (git worktrees of user
repositories) and the Chromium cache subtrees under `browser/*/user-data/`,
`browser/*/user-data/*/Cache`, `browser/*/user-data/*/Code Cache`,
`browser/*/user-data/*/GPUCache` and `browser/*/user-data/*/Service Worker`; do not exclude
`browser/*/user-data` as a whole, since its History and Cookies show what the agent browsed. The
temporary trees deeper in an agent directory are `agents/*/agent/**/tmp` and
`agents/*/agent/**/.tmp`.

The full git clones the Gateway makes when a project is added from a remote URL,
`projects/<16 hex>/<name>`, are the same kind of tree as `worktrees/`. The clone directory's name
is exactly 16 lower-case hex characters
([`src/projects/project-clone.ts:302-318`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/projects/project-clone.ts#L302-L318)),
so the exclusion
`projects/[0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f]`
names it, and nothing else under `projects/` is pruned; which repository each clone is stays in
`state/openclaw.sqlite` `projects` (`origin_url`, `repo_root`). The cold archive directory
`agents/<agentId>/sessions/cold/` holds transcript history and is not excluded
([Transcript stores](#transcript-stores)). Every exclusion applies below each state directory
form, the legacy `.clawdbot` and `.moltbot` included, since a build that used one as its state
directory wrote the same tree there ([Legacy names](#legacy-names)).

### Project-local files

The agent workspace is the main project. Inside user repositories OpenClaw reads
`.openclaw/worktree-setup.sh` when it creates a managed worktree
([`agents/worktrees/repository-setup.ts:81`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/agents/worktrees/repository-setup.ts#L81)),
skills in a workspace's `.agents/skills/`
([`skills/loading/workspace-skill-roots.ts:47`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/skills/loading/workspace-skill-roots.ts#L47)),
and the workspace bootstrap files of
[Contents of the state directory](#contents-of-the-state-directory) when a repository is
configured as a workspace. `/export-trajectory` writes a trajectory bundle into the workspace's
`.openclaw/trajectory-exports/openclaw-trajectory-<session>-<timestamp>/`
([`docs/tools/trajectory.md:28-47`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/docs/tools/trajectory.md#L28-L47)).
A crashed memory migration leaves
`.openclaw-repair/root-memory/`
([`root-memory-files.ts:9`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/memory/root-memory-files.ts#L9)).
No per-project database.

### Where the project path is recorded

- `openclaw.json`: `agents.defaults.workspace` and `agents.entries.<id>.workspace`, `cwd` and
  `agentDir`
  ([`config/zod-schema.agents.ts:30-39`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/zod-schema.agents.ts#L30-L39);
  [`zod-schema.agent-defaults-base.ts:62`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/zod-schema.agent-defaults-base.ts#L62);
  [`zod-schema.agent-entry-base.ts:85-87`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/zod-schema.agent-entry-base.ts#L85-L87)).
  JSON5, so a key may be unquoted. A build that used a legacy state directory kept the same keys
  in `.clawdbot/openclaw.json` or `.clawdbot/clawdbot.json`
  ([`src/config/paths.ts:36-41` at `3b16db7`](https://github.com/openclaw/openclaw/blob/3b16db73c0b5209aad2ee7c3fb5ddfb12fb208f9/src/config/paths.ts#L36-L41)),
  and a Moltbot build in `.moltbot/moltbot.json` or `.clawdbot/moltbot.json`
  ([Credentials](#credentials)).
- `state/openclaw.sqlite` tables `projects` (`repo_root`, `origin_url`, `source` `registered` for a
  repository the user pointed the Gateway at, `cloned` for one it cloned into `projects/`) and
  `worktrees` (`repo_root`, `path`, `owner_kind` `manual|workboard|session`, `owner_id`,
  `removed_at`)
  ([`openclaw-state-schema.sql:1854-1870`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/state/openclaw-state-schema.sql#L1854-L1870),
  [`:1901-1909`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/state/openclaw-state-schema.sql#L1901-L1909)):
  SQLite only. They separate the user's projects from the agent's scratch checkouts.
- The transcript header's `cwd`, in `transcript_events` and in legacy `.jsonl` files
  ([Header](#header)). Headers are never compressed, so a byte grep of the database finds them
  ([`transcript-payload.ts:546-550`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/sessions/transcript-payload.ts#L546-L550)).
- `state/openclaw.sqlite` `worker_session_placements.remote_workspace_dir` names a cloud worker
  session's workspace on the worker, not on this host
  ([`openclaw-state-schema.sql:2152-2175`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/state/openclaw-state-schema.sql#L2152-L2175)).

## Parsing

### Transcript stores

Home-relative, with `.openclaw` replaceable by `.openclaw-<profile>` or, on a host doctor never
migrated, the legacy `.clawdbot`:

- `.openclaw/agents/<agentId>/agent/openclaw-agent.sqlite` with its sidecars
  `openclaw-agent.sqlite-wal` and `openclaw-agent.sqlite-shm`: the live store.
  One `session_nodes` row per logical session, one `session_windows` row per generation, one
  `transcript_events` row per entry ([Database rows](#database-rows)). Its directory, `agentDir`,
  can be moved by config ([Contents of the state directory](#contents-of-the-state-directory)).
- `.openclaw/agents/<agentId>/sessions/<sessionId>.jsonl`: legacy one-file-per-session transcripts
  with their `sessions.json` index, imported into SQLite by `openclaw doctor --fix`, which records
  a receipt and then removes the source
  ([`docs/cli/doctor/state-migrations.md:135`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/docs/cli/doctor/state-migrations.md#L135);
  [`docs/gateway/doctor/state-and-sessions.md:43`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/docs/gateway/doctor/state-and-sessions.md#L43));
  a host that never ran doctor after upgrading still has them. Doctor no longer discovers the
  older single-agent `sessions/` at the state root unless a configured store points there
  ([`docs/gateway/doctor/state-and-sessions.md:62`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/docs/gateway/doctor/state-and-sessions.md#L62)).
- Same directory, archives of reset or deleted sessions:
  `<sessionId>.jsonl.reset.<YYYY-MM-DDTHH-MM-SS[.mmm]Z>[.<32 hex generation>][.zst]` and
  `.jsonl.deleted.<...>`, and the legacy compact backups `.jsonl.bak.<...>`
  ([`config/sessions/transcript-file-names.ts:1-16`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/sessions/transcript-file-names.ts#L1-L16);
  naming
  [`session-accessor.sqlite-archive-artifact.ts:39-80`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/sessions/session-accessor.sqlite-archive-artifact.ts#L39-L80)).
  A `.zst` file is one zstd frame of the JSONL. The canonical copy of each reset or delete archive
  is a blob in `session_transcript_archives`; the file is published from the blob's exact bytes,
  hash-checked
  ([`session-accessor.sqlite-archive-artifact.ts:186-225`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/sessions/session-accessor.sqlite-archive-artifact.ts#L186-L225)).
  When a delete or reset keeps one is in [What survives a delete](#what-survives-a-delete).
- Same directory, `cold/<sha256>.jsonl.zst`: old generations moved to cold storage when
  `session_transcript_cold_archives.storage` is `file`, one zstd file per archive, or per chunk of
  at most 64 MiB uncompressed in `session_transcript_cold_archive_chunks`
  ([`session-cold-storage-codec.ts:36-41`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/sessions/session-cold-storage-codec.ts#L36-L41);
  [`session-cold-storage-prepare.ts:421-433`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/sessions/session-cold-storage-prepare.ts#L421-L433);
  [agent history](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/docs/reference/database-schemas/agent-schema-history.md#L56)).
  With `storage` `sqlite` the bytes are `archive_blob` in the row.
- Also there: `<name>.checkpoint.<uuid>.jsonl` compaction checkpoints, `<id>.trajectory.jsonl`
  runtime trajectories and their `<id>.trajectory-path.json` pointers, `*.migrated[.N]` and
  `*.jsonl.pre-doctor-(branch|openai-codex)-repair-<ts>.bak` rollback copies, and
  `sessions.json.bak.<n>`
  ([`config/sessions/artifacts.ts:19-41`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/sessions/artifacts.ts#L19-L41),
  [`:77-91`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/sessions/artifacts.ts#L77-L91)). Which of them copy a transcript is
  in [Past-use rows and secondary sources](#past-use-rows-and-secondary-sources).

The agent database uses a WAL journal
([`infra/sqlite-wal.ts:216`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/infra/sqlite-wal.ts#L216)),
read through `sqlite_util.py` with its sidecars. An entry of at least 1 KiB and at most 4 MiB is
stored as `event_zstd` (one zstd frame, level 1, with checksum, no dictionary) with `event_json`
NULL when the frame and its `navigation_json` save at least the larger of 64 bytes and a tenth of
the size; a header entry, an entry holding `\u` escapes or a NUL, and every entry on PostgreSQL
stay in `event_json`
([`config/sessions/transcript-payload.ts:500-575`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/sessions/transcript-payload.ts#L500-L575);
[`transcript-payload-read.ts:7`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/sessions/transcript-payload-read.ts#L7);
[`infra/zstd-codec.ts:13-20`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/infra/zstd-codec.ts#L13-L20)).
`event_utf8_bytes` is the decoded length. The parser needs `zstandard`. `archive_blob` in the
archive tables is JSONL, zstd when `encoding` is `zstd`, and always zstd in the cold tables.

#### What survives a delete

`openclaw sessions archive` keeps the transcript and only sets `archivedAt`, `archivedBy` and
`archiveReason` (`manual`, `active-session-cap`, `age-retention`, `stale-dashboard`,
`restart-recovery`) in `entry_json`, mirrored in `session_nodes.archived_at`
([`docs/cli/sessions.md:183-198`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/docs/cli/sessions.md#L183-L198);
[`src/config/sessions/types.ts:304-309`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/sessions/types.ts#L304-L309);
[`gateway-protocol/src/schema/sessions-row.ts:55-61`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/packages/gateway-protocol/src/schema/sessions-row.ts#L55-L61)).
`openclaw sessions delete` removes the session row, its generations, runtime state, bindings and
boards, and keeps a verified `.jsonl.deleted.<ts>` archive with its `session_transcript_archives`
blob (`reason` `deleted`) unless the session was incognito, as a reset keeps a `.jsonl.reset.<ts>`
one
([`docs/cli/sessions.md:224-236`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/docs/cli/sessions.md#L224-L236);
[`session-accessor.sqlite-entry-store.ts:170-186`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/sessions/session-accessor.sqlite-entry-store.ts#L170-L186)).
Age retention later removes a published archive's row and its file in one transaction; when the
file cannot be removed the row stays
([`session-history-archive-pruning.worker.ts:103-152`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/sessions/session-history-archive-pruning.worker.ts#L103-L152)).

The memory index keeps the session's `memory_index_chunks` until `openclaw memory forget`, which
writes one `memory_session_tombstones` row per forgotten session, reason `forgotten` by default
([`extensions/memory-core/src/memory-session-tombstones.ts:89-106`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/extensions/memory-core/src/memory-session-tombstones.ts#L89-L106));
the vendor states that retained deleted-session archives can remain eligible for memory search
until then ([`docs/cli/sessions.md:229-236`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/docs/cli/sessions.md#L229-L236)). Inbound channel
messages in `channel_ingress_events` are pruned by age and count, not by deletion;
`conversation_deliveries` rows go only with their conversation; an uploaded backup keeps the
session; pending inputs cascade away with the session; a public share grant is lost with the
session row ([Other stores](#other-stores),
[Sync, relay and origin rows](#sync-relay-and-origin-rows)). There is no prompt-history file.

The analyzer reads the archive blobs as primary and the published `.jsonl.deleted.*` and
`.jsonl.reset.*` files as secondary. A deleted archive gives an `archive=deleted.<ts>`
session-start row and a `session deleted:` row, a tombstone a `session forgotten from memory:`
row, an archived session a `session archived:` row; the full-text index gives the entries the
transcript no longer holds, the pending inputs the prompts that never reached it, the channel
tables the inbound messages and deliveries, and the backups `backup uploaded:` rows (see
[Secondary sources](../analyzer/docs/output.md#secondary-sources) and
[Deletion rows](#deletion-rows)).

### Record schema

#### Header

Every transcript opens with a header record, the entry shape of
[pi's header](pi.md#header) with OpenClaw's writer
([`config/sessions/transcript-header.ts:6-29`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/sessions/transcript-header.ts#L6-L29);
[`agents/sessions/session-manager-types.ts:7-16`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/agents/sessions/session-manager-types.ts#L7-L16)).
`cwd` defaults to the process working directory when the caller gives none; `inheritedThrough`
lists the entry ids a copied transcript inherited from its source.

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `type` | string | `session` | |
| `version` | number | `3`, `4` | |
| `id` | string | | |
| `timestamp` | string | | |
| `cwd` | string | | |
| `parentSession` | string | | |
| `inheritedThrough` | array | | |

#### Entries

Every other record has `type`, `id`, `parentId` (null at a root), `timestamp` (ISO 8601 UTC from
`toISOString()`) and an optional `appendMode` `side`
([`packages/agent-core/src/harness/types.ts:65-109`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/packages/agent-core/src/harness/types.ts#L65-L109);
[`session-manager-types.ts:23-96`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/agents/sessions/session-manager-types.ts#L23-L96)).
OpenClaw's set differs from pi's: it adds `reset` and the opaque `leaf` record, preserves unknown
records that carry `id` and `parentId`
([`config/sessions/session-entry-codec.ts:190-218`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/sessions/session-entry-codec.ts#L190-L218)),
and adds `supersedesEntryId` to a `message` a branch rewrite re-appended and `__openclaw` and
`tokensAfter` to a `compaction`.

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `type` | string | `message`, `thinking_level_change`, `model_change`, `compaction`, `reset`, `branch_summary`, `custom`, `custom_message`, `label`, `session_info`, `leaf` | |
| `id` | string | | |
| `parentId` | string or null | | |
| `timestamp` | string | | |
| `appendMode` | string | `side` | |
| `message` | object | | |
| `supersedesEntryId` | string | | |
| `thinkingLevel` | string | | |
| `provider` | string | | |
| `modelId` | string | | |
| `summary` | string | | |
| `firstKeptEntryId` | string | | |
| `tokensBefore` | number | | |
| `tokensAfter` | number | | |
| `details` | object or array or string or number or boolean or null | | |
| `fromHook` | boolean | | |
| `__openclaw` | object | | |
| `reason` | string | `new`, `reset`, `idle`, `daily`, `cron-stale` | |
| `fromId` | string | | |
| `customType` | string | | |
| `data` | object or array or string or number or boolean or null | | |
| `content` | string or array | | |
| `display` | boolean | | |
| `excludeFromContext` | boolean | | |
| `targetId` | string or null | | |
| `label` | string | | |
| `name` | string | | |
| `appendParentId` | string or null | | |

#### Message roles

`message.message` is an `AgentMessage`, discriminated by `role`, every one with `timestamp` in
Unix milliseconds except a `compactionSummary` read from older rows, whose `timestamp` may be a
string
([`packages/llm-core/src/types.ts:377-400`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/packages/llm-core/src/types.ts#L377-L400),
[`:525-572`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/packages/llm-core/src/types.ts#L525-L572);
[`packages/agent-core/src/types.ts:368-435`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/packages/agent-core/src/types.ts#L368-L435)).
The roles are pi's ([Message roles in pi](pi.md#message-roles)) less `system`. OpenClaw's user
message adds `synthetic` (host-generated context, not a human turn), `runtimeContext` (injected
runtime context) and `operatorMessage`; its assistant message adds `openclawDelivery`,
`responseAccessProgram`, `turnId`, `endTurn`, `errorCode`, `errorType`, `errorBody` and
`retryAfterMs`. `bashExecution` is a user-run `!` command. `usage` holds `input`, `output`,
`cacheRead`, `cacheWrite`, `totalTokens` and a `cost` object.

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `role` | string | `user`, `assistant`, `toolResult`, `bashExecution`, `custom`, `branchSummary`, `compactionSummary` | |
| `content` | string or array | | |
| `timestamp` | number or string | | |
| `synthetic` | boolean | | |
| `runtimeContext` | object | | |
| `operatorMessage` | object | | |
| `api` | string | | |
| `provider` | string | | |
| `model` | string | | |
| `responseModel` | string | | |
| `responseId` | string | | |
| `turnId` | string | | |
| `usage` | object | | |
| `stopReason` | string | `stop`, `length`, `toolUse`, `error`, `aborted` | |
| `errorMessage` | string | | |
| `errorCode` | string | | |
| `errorBody` | string | | |
| `providerReplay` | object | | |
| `openclawDelivery` | object | | |
| `toolCallId` | string | | |
| `toolName` | string | | |
| `isError` | boolean | | |
| `command` | string | | |
| `output` | string | | |
| `exitCode` | number or null | | |
| `cancelled` | boolean | | |
| `truncated` | boolean | | |
| `fullOutputPath` | string | | |
| `customType` | string | | |
| `display` | boolean | | |
| `summary` | string | | |
| `fromId` | string | | |
| `tokensBefore` | number | | |

`usage`, its `cost`
([`llm-core/src/types.ts:316-347`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/packages/llm-core/src/types.ts#L316-L347))
and `runtimeContext`
([`:384-387`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/packages/llm-core/src/types.ts#L384-L387)) hold:

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `input` | number | | |
| `output` | number | | |
| `cacheRead` | number | | |
| `cacheWrite` | number | | |
| `totalTokens` | number | | |
| `cost` | object | | |
| `total` | number | | |
| `retained` | boolean | | |

`stopReason` values are at
[`llm-core/src/types.ts:364`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/packages/llm-core/src/types.ts#L364); `api` values include
`anthropic-messages`
([`:23`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/packages/llm-core/src/types.ts#L23)).
`responseModel` is the concrete model the stream reported "when different from the requested
`model`"
([`:542`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/packages/llm-core/src/types.ts#L542)).

Content blocks
([`llm-core/src/types.ts:261-314`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/packages/llm-core/src/types.ts#L261-L314)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `type` | string | `text`, `thinking`, `image`, `toolCall` | |
| `text` | string | | |
| `textSignature` | string | | |
| `thinking` | string | | |
| `thinkingSignature` | string | | |
| `redacted` | boolean | | |
| `data` | string | | |
| `mimeType` | string | | |
| `id` | string | | |
| `name` | string | | |
| `arguments` | object | | |
| `thoughtSignature` | string | | |
| `async` | boolean | | |
| `executionMode` | string | `sequential`, `parallel` | |

The shell tool is named `exec`
([`agents/lazy-exec-tool.ts:66`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/agents/lazy-exec-tool.ts#L66)).

#### Database rows

`transcript_events` holds one entry per row, ordered by `seq` from 0 within a generation;
`created_at` is the entry's timestamp in epoch ms, else the write time
([`openclaw-agent-schema.sql:437-482`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/state/openclaw-agent-schema.sql#L437-L482);
[`session-accessor.sqlite-transcript-store.ts:120`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/sessions/session-accessor.sqlite-transcript-store.ts#L120)).

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `session_id` | string | | |
| `seq` | number | | |
| `event_json` | string or null | | |
| `event_zstd` | blob or null | | |
| `event_utf8_bytes` | number or null | | |
| `created_at` | number | | |
| `navigation_json` | string or null | | |
| `message_role` | string or null | | |

`session_windows`, one row per generation
([`openclaw-agent-schema.sql:140-171`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/state/openclaw-agent-schema.sql#L140-L171)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `session_id` | string | | |
| `session_key` | string | | |
| `previous_session_id` | string or null | | |
| `reason` | string or null | `initial`, `reset`, `rollover`, `fork`, `rewind`, `switch`, `recovery`, `compaction` | |
| `session_scope` | string | `conversation`, `shared-main`, `group`, `channel` | |
| `created_at` | number | | |
| `updated_at` | number | | |
| `hook_external_content_source` | string or null | `gmail`, `webhook` | |
| `started_at` | number or null | | |
| `ended_at` | number or null | | |
| `status` | string or null | `running`, `done`, `failed`, `killed`, `timeout` | |
| `chat_type` | string or null | `direct`, `group`, `channel` | |
| `channel` | string or null | | |
| `account_id` | string or null | | |
| `primary_conversation_id` | string or null | | |
| `model_provider` | string or null | | |
| `model` | string or null | | |
| `parent_session_key` | string or null | | |
| `spawned_by` | string or null | | |
| `display_name` | string or null | | |

`session_nodes`, one row per logical session, whose `entry_json` is the session's metadata
([`openclaw-agent-schema.sql:16-51`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/state/openclaw-agent-schema.sql#L16-L51)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `session_key` | string | | |
| `current_session_id` | string | | |
| `entry_json` | string | | |
| `updated_at` | number | | |
| `created_at` | number or null | | |
| `created_via` | string or null | `operator`, `spawn`, `channel`, `cron`, `talk`, `run`, `plugin`, `internal` | |
| `created_actor_type` | string or null | `human`, `agent`, `system` | |
| `created_actor_id` | string or null | | |
| `parent_session_key` | string or null | | |
| `spawned_by` | string or null | | |
| `fork_source_session_id` | string or null | | |
| `fork_source_entry_id` | string or null | | |
| `label` | string or null | | |
| `archived_at` | number or null | | |

The `entry_json` keys a reader uses
([`src/config/sessions/types.ts:76-99`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/sessions/types.ts#L76-L99),
[`:289-309`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/sessions/types.ts#L289-L309),
[`:484-537`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/sessions/types.ts#L484-L537),
[`:611`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/sessions/types.ts#L611),
[`:655-656`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/sessions/types.ts#L655-L656);
[`sessions-row.ts:63-70`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/packages/gateway-protocol/src/schema/sessions-row.ts#L63-L70)).
`toolOverrides` maps `mcpServers` and `skills` names to booleans, `mcpToolsDeny` server names to
denied tools, and `webSearch` to a boolean. `modelOverrideSource` is `auto` for a temporary runtime
fallback, `user` for `/model` or `sessions.patch`, `default` for a configured default.
`modelOverrideFallbackOriginProvider` and `modelOverrideFallbackOriginModel` name the model the
user had picked when a fallback replaced it. `delivery` is `{kind:"none"}`, `{kind:"internal"}` or
`{kind:"external", route, context, origin}`; `route.channel` is the lower-case channel id
([`src/plugin-sdk/channel-route.ts:19-26`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/plugin-sdk/channel-route.ts#L19-L26)).

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `sessionId` | string | | |
| `updatedAt` | number | | |
| `incognito` | boolean | `true` | |
| `archivedAt` | number | | |
| `archivedBy` | object | | |
| `archiveReason` | string | `manual`, `active-session-cap`, `age-retention`, `stale-dashboard`, `restart-recovery` | |
| `toolOverrides` | object | | |
| `elevatedLevel` | string | `off`, `on`, `ask`, `full` | |
| `execHost` | string | | |
| `execNode` | string | | |
| `execCwd` | string | | |
| `providerOverride` | string | | |
| `modelOverride` | string | | |
| `modelOverrideSource` | string | `auto`, `user`, `default` | |
| `modelOverrideFallbackOriginProvider` | string | | |
| `modelOverrideFallbackOriginModel` | string | | |
| `sendPolicy` | string | `allow`, `deny` | |
| `delivery` | object | | |
| `publicShare` | object | | |

`elevatedLevel` values are at
[`auto-reply/thinking.shared.ts:27`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/auto-reply/thinking.shared.ts#L27), and `execHost`
names `sandbox`, `gateway` or `node`
([`infra/exec-approvals-core.ts:13-14`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/infra/exec-approvals-core.ts#L13-L14)).
`archivedBy` is a `{type, id, label}` actor
([`session-entry-provenance.ts:17-19`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/sessions/session-entry-provenance.ts#L17-L19);
[`sessions-row.ts:73-76`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/packages/gateway-protocol/src/schema/sessions-row.ts#L73-L76)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `type` | string | `human`, `agent`, `system` | |
| `id` | string | | |
| `label` | string | | |

The `delivery` `origin` object holds `provider`, `surface`, `chatType`, `from`, `to`,
`accountId`, `threadId`, `nativeChannelId` and `nativeDirectUserId`; `publicShare` is
`{id, sessionId, createdAt}`, `id` 48 lower-case hex characters, `sessionId` the generation it
publishes, `createdAt` epoch ms
([`session-public-share.ts:7-24`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/sessions/session-public-share.ts#L7-L24)).

#### Archive tables

`session_transcript_archives`, the reset and delete archives, and `session_transcript_cold_archives`
with its chunks, the cold tier
([`openclaw-agent-schema.sql:484-542`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/state/openclaw-agent-schema.sql#L484-L542)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `session_id` | string | | |
| `generation` | string | | |
| `session_key` | string | | |
| `reason` | string | `deleted`, `reset` | |
| `encoding` | string | `identity`, `zstd` | |
| `archive_blob` | blob or null | | |
| `archive_sha256` | string | | |
| `archive_name` | string | | |
| `created_at` | number | | |
| `published_at` | number or null | | |
| `storage` | string | `file`, `sqlite` | |
| `archived_at` | number | | |
| `event_count` | number | | |
| `raw_bytes` | number | | |
| `archive_bytes` | number | | |
| `last_seq` | number | | |
| `chunk_index` | number | | |
| `first_seq` | number | | |

#### Cold archive records

A cold archive, decompressed, is JSONL whose records carry `kind`: one `header`, then per
moved transcript row an `event` (the `transcript_events` row), an `identity` (its tree index
row), and `active`, `index` and `fts` rows of the generation's indexes, each under `row`
([`session-cold-storage-codec.ts:78-127`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/sessions/session-cold-storage-codec.ts#L78-L127)).

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `kind` | string | `header`, `event`, `identity`, `active`, `index`, `fts` | |
| `version` | number | `1` | |
| `sessionId` | string | | |
| `generation` | string | | |
| `row` | object | | |
| `seq` | number | | |
| `event_json` | string | | |
| `created_at` | number | | |
| `event_id` | string | | |
| `event_type` | string or null | | |
| `parent_id` | string or null | | |
| `message_idempotency_key` | string or null | | |
| `active_position` | number | | |
| `event_seq` | number | | |
| `message_position` | number or null | | |
| `context_eligible` | number or null | | |
| `indexed_seq` | number | | |
| `leaf_event_id` | string or null | | |
| `needs_rebuild` | number | | |
| `active_event_count` | number | | |
| `active_message_count` | number | | |
| `updated_at` | number | | |
| `text` | string or null | | |
| `message_id` | string or null | | |
| `role` | string or null | | |
| `timestamp` | string or number or null | | |

### Joins and timestamps

- Session: `transcript_events.session_id = session_windows.session_id`, and the header `id` equals
  it. Generations of one conversation share `session_key` and chain through
  `previous_session_id`; `session_nodes.current_session_id` names the live one.
- The session key encodes the origin: `agent:<agentId>:<channel>:<accountId>:direct:<peerId>`,
  `agent:<agentId>:<channel>:direct:<peerId>`, `agent:<agentId>:direct:<peerId>`,
  `agent:<agentId>:<channel>:<peerKind>:<peerId>`, or `agent:<agentId>:main`-style keys
  ([`routing/session-key.ts:179-234`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/routing/session-key.ts#L179-L234)).
  This is how a responder ties a transcript to a Telegram chat or WhatsApp number.
- Relay: a session carried through a messaging platform is recorded three ways besides its key,
  each naming the platform's copy of the conversation: the `session_windows` columns `channel`,
  `account_id`, `chat_type`, `primary_conversation_id` and `hook_external_content_source` (a
  session a Gmail or webhook hook started); `conversations` joined to a generation by
  `session_conversations` ([Other stores](#other-stores)); and the `entry_json` `delivery` route,
  with `session_nodes.created_via` `channel` for a session a channel message created. The channel
  `webchat` is the Control UI itself, not a platform
  ([`utils/message-channel-constants.ts:3`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/utils/message-channel-constants.ts#L3)).
- Tool call to result: `toolCall.id` in an assistant message equals `toolResult.toolCallId` in a
  later `message` entry.
- Tree: `parentId` to `id`; `transcript_event_identities` indexes it
  ([`openclaw-agent-schema.sql:580-590`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/state/openclaw-agent-schema.sql#L580-L590)).
- Subagents: `session_nodes.parent_session_key` and `spawned_by`, and `fork_source_session_id` and
  `fork_source_entry_id` for forks; header `parentSession`.
- Project: header `cwd`. No git branch is recorded on a transcript.
- Approvals: `operator_approvals.source_session_id` and `source_tool_call_id` join an approval to
  the transcript's `toolCall.id` ([Permission records](#permission-records)).
- Times: entry `timestamp` is an ISO 8601 UTC string; message `timestamp`, the database
  `created_at` columns and every `*_ms` column are epoch ms. How copies join the database rows is
  under [Past-use rows and secondary sources](#past-use-rows-and-secondary-sources).

### Other stores

#### Permission records

All in `state/openclaw.sqlite`.

`operator_approvals`, one row per approval request the Gateway raised for an `exec` (shell) call,
a plugin action or a system agent
([`openclaw-state-schema.sql:370-475`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/state/openclaw-state-schema.sql#L370-L475)).
`resolver_kind` names who decided: the paired device, the chat channel, the runtime's automatic
reviewer or the system; `presentation_json` is what the reviewer was shown.

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `approval_id` | string | | |
| `kind` | string | `exec`, `plugin`, `system-agent` | |
| `status` | string | `pending`, `allowed`, `denied`, `expired`, `cancelled` | |
| `decision` | string or null | `allow-once`, `allow-always`, `deny` | |
| `terminal_reason` | string or null | `user`, `timeout`, `malformed-verdict`, `no-route`, `run-aborted`, `gateway-restart`, `storage-corrupt` | |
| `resolver_kind` | string or null | `device`, `channel`, `runtime`, `system` | |
| `resolver_id` | string or null | | |
| `presentation_json` | string | | |
| `requested_by_device_id` | string or null | | |
| `source_agent_id` | string or null | | |
| `source_session_key` | string or null | | |
| `source_session_id` | string or null | | |
| `source_run_id` | string or null | | |
| `source_tool_call_id` | string or null | | |
| `source_tool_name` | string or null | | |
| `created_at_ms` | number | | |
| `expires_at_ms` | number | | |
| `resolved_at_ms` | number or null | | |
| `consumed_at_ms` | number or null | | |

`exec_approvals_config`, the standing exec policy, one row whose `raw_json` is the approvals file
less its socket credentials: `defaults` and per-agent `agents.<id>` with `security`, `ask`,
`askFallback`, `autoAllowSkills`, an `allowlist` and `mcpTools` grants
([`openclaw-state-schema.sql:356-368`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/state/openclaw-state-schema.sql#L356-L368);
[`infra/exec-approvals-sqlite.ts:165-186`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/infra/exec-approvals-sqlite.ts#L165-L186);
[`gateway-protocol/src/schema/exec-approvals.ts:15-75`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/packages/gateway-protocol/src/schema/exec-approvals.ts#L15-L75)).
The exec mode `auto` is `allowlist` with `on-miss` and the automatic reviewer on
([`infra/exec-approvals-core.ts:15-18`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/infra/exec-approvals-core.ts#L15-L18),
[`:100-119`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/infra/exec-approvals-core.ts#L100-L119)).

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `config_key` | string | | |
| `raw_json` | string | | |
| `socket_path` | string or null | | |
| `has_socket_token` | number | | |
| `default_security` | string or null | `deny`, `allowlist`, `full` | |
| `default_ask` | string or null | `off`, `on-miss`, `always` | |
| `default_ask_fallback` | string or null | `deny`, `allowlist`, `full` | |
| `auto_allow_skills` | number or null | | |
| `agent_count` | number | | |
| `allowlist_count` | number | | |
| `updated_at_ms` | number | | |

The `raw_json` keys:

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `version` | number | `1` | |
| `socket` | object | | |
| `defaults` | object | | |
| `agents` | object | | |
| `security` | string | `deny`, `allowlist`, `full` | |
| `ask` | string | `off`, `on-miss`, `always` | |
| `askFallback` | string | `deny`, `allowlist`, `full` | |
| `autoAllowSkills` | boolean | | |
| `allowlist` | array | | |
| `pattern` | string | | |
| `argPattern` | string | | |
| `source` | string | `allow-always` | |
| `mcpTools` | array | | |
| `server` | string | | |
| `tool` | string | | |
| `addedAt` | number | | |

`operator_approval_standing_grants`: an `allow-always` decision on a scheduled (cron) job mints a
grant
([`openclaw-state-schema.sql:512-526`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/state/openclaw-state-schema.sql#L512-L526)).
`channel_pairing_allow_entries` lists the chat senders allowed to talk to the agent per channel
account
([`:760-767`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/state/openclaw-state-schema.sql#L760-L767)).

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `grant_id` | string | | |
| `minted_by_approval_id` | string | | |
| `agent_id` | string | | |
| `cron_job_id` | string | | |
| `operation_binding` | string | | |
| `created_at_ms` | number | | |
| `expires_at_ms` | number or null | | |
| `revoked_at_ms` | number or null | | |
| `last_used_at_ms` | number or null | | |
| `use_count` | number | | |
| `channel_key` | string | | |
| `account_id` | string | | |
| `entry` | string | | |
| `sort_order` | number | | |
| `updated_at` | number | | |

`audit_events` carries `tool_call_id`, `tool_name`, `action`, `status`, `reason_code` and
`actor_type` and `actor_id` per action, with `occurred_at`
([`:100-132`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/state/openclaw-state-schema.sql#L100-L132)); its `kind` and `action` values
are free text in the schema. Per session, the `entry_json` keys `elevatedLevel`, `execHost`,
`execNode`, `execCwd`, `toolOverrides` and `sendPolicy` ([Database rows](#database-rows)) hold
the session's permission state.

#### Past-use tables

Every source is present at the reviewed commit; the parser reads all but the memory index
([Past-use rows and secondary sources](#past-use-rows-and-secondary-sources)):

| Source | Holds | Survives | Joins on | Plan |
| --- | --- | --- | --- | --- |
| `openclaw-agent.sqlite` `session_transcript_fts` (FTS5, the SQLite full-text index; content in its `session_transcript_fts_content` shadow table) and `session_transcript_fts_rows` ([`openclaw-agent-schema.sql:825-847`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/state/openclaw-agent-schema.sql#L825-L847)) | user and assistant text with `session_id`, `message_id`, `role`, `timestamp`, the index behind the `sessions_search` tool ([`docs/concepts/session-search.md:10-13`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/docs/concepts/session-search.md#L10-L13)) | rows are deleted when a generation moves to cold storage ([`session-cold-storage-worker.ts:216-218`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/sessions/session-cold-storage-worker.ts#L216-L218)); `session_transcript_fts_rows` has no foreign key to `session_windows` | `session_id`, entry `id` (`message_id`), text, time | secondary |
| `openclaw-agent.sqlite` `session_pending_inputs` ([`:863-880`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/state/openclaw-agent-schema.sql#L863-L880)) and `session_input_completions` ([`:886-898`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/state/openclaw-agent-schema.sql#L886-L898)) | each accepted prompt as `message_json` with `state` and `accepted_at`, kept "outside the active transcript until its exact turn owns execution" | a cancelled or interrupted prompt never reaches the transcript; the rows cascade away with the session | `session_id`, `session_key`, `consumed_event_id` (the transcript entry once consumed) | primary for rows never consumed, secondary otherwise |
| `openclaw-agent.sqlite` `memory_index_chunks` with `source` `sessions` ([`:674-686`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/state/openclaw-agent-schema.sql#L674-L686); the sources are `memory` and `sessions`, [`memory-host-sdk/src/host/types.ts:5`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/packages/memory-host-sdk/src/host/types.ts#L5)) | chunks of transcript text with `path`, line range and `updated_at` | a session delete, until `openclaw memory forget` | `path`, which is `sessions/<agentId>/<sessionId>.jsonl` for a SQLite session and `sessions/<agentId>/<file name>` for a legacy file ([`session-entry-render.ts:143-146`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/packages/memory-host-sdk/src/host/session-entry-render.ts#L143-L146); [`session-files.ts:276-281`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/packages/memory-host-sdk/src/host/session-files.ts#L276-L281)), and text | secondary |
| `state/openclaw.sqlite` `channel_ingress_events` ([`openclaw-state-schema.sql:1239-1261`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/state/openclaw-state-schema.sql#L1239-L1261)) | each inbound channel message as `payload_json`, with `channel_id`, `account_id`, `status`, `received_at` | pruned by age and count per status (`pending`, `completed`, `failed`), not by session deletion ([`channels/message/ingress-queue.kernel.ts:495-513`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/channels/message/ingress-queue.kernel.ts#L495-L513)) | text and time | secondary |
| `openclaw-agent.sqlite` `conversation_deliveries` ([`:225-248`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/state/openclaw-agent-schema.sql#L225-L248)) | each outbound send or turn to a platform conversation | cascades with the `conversations` row, not with the session | `source_session_key`, time | secondary |

`session_pending_inputs`; `message_json` is a user message (`role` `user`, `content`)
([`session-pending-input-value.ts:8-15`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/sessions/session-pending-input-value.ts#L8-L15)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `input_id` | string | | |
| `session_key` | string | | |
| `session_id` | string | | |
| `run_id` | string | | |
| `message_json` | string | | |
| `state` | string | `queued`, `interrupted`, `cancelled` | |
| `accepted_at` | number | | |
| `consumed_event_id` | string or null | | |

`session_transcript_fts_content`, the FTS5 content table, has the row id `id` and columns `c0` to
`c4`, being `text`, `session_id`, `message_id`, `role` and `timestamp` in the declared order:

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `id` | number | | |
| `c0` | string | | |
| `c1` | string | | |
| `c2` | string or null | | |
| `c3` | string | | |
| `c4` | string or number | | |

`conversations` and `session_conversations`, the platform conversations and their links to
generations
([`openclaw-agent-schema.sql:192-207`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/state/openclaw-agent-schema.sql#L192-L207),
[`:257-267`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/state/openclaw-agent-schema.sql#L257-L267)),
and `conversation_deliveries`, whose outbound text is stored only as `message_hash`; a platform
message that replied to a delivery fills the reply columns
([`conversation-delivery-store.types.ts:24-30`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/sessions/conversation-delivery-store.types.ts#L24-L30)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `conversation_id` | string | | |
| `channel` | string | | |
| `account_id` | string | | |
| `kind` | string | `direct`, `group`, `channel` | |
| `peer_id` | string | | |
| `delivery_target` | string | | |
| `thread_id` | string or null | | |
| `native_channel_id` | string or null | | |
| `native_direct_user_id` | string or null | | |
| `role` | string | `primary`, `participant`, `related` | |
| `first_seen_at` | number | | |
| `last_seen_at` | number | | |
| `operation_id` | string | | |
| `operation_kind` | string | `send`, `turn` | |
| `source_session_key` | string or null | | |
| `message_hash` | string | | |
| `status` | string | `created`, `queued`, `sent`, `suppressed`, `rejected`, `unknown`, `replied` | |
| `platform_message_id` | string or null | | |
| `rejection_error` | string or null | | |
| `reply_message_id` | string or null | | |
| `reply_text` | string or null | | |
| `reply_timestamp` | number or null | | |
| `created_at` | number | | |

`channel_ingress_events`, whose `payload_json` shape differs per channel plugin:

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `queue_name` | string | | |
| `event_id` | string | | |
| `channel_id` | string | | |
| `status` | string | | |
| `payload_json` | string | | |
| `received_at` | number | | |

`memory_session_tombstones`, one row per forgotten session
([`openclaw-agent-schema.sql:715-720`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/state/openclaw-agent-schema.sql#L715-L720)), and
`memory_entry_origins`, which ties a memory entry to the session it came from
([`:705-713`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/state/openclaw-agent-schema.sql#L705-L713)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `session_id` | string | | |
| `agent_id` | string | | |
| `reason` | string | `forgotten` | |
| `created_at` | number | | |
| `entry_key` | string | | |
| `origin_class` | string | `owner`, `agent`, `untrusted`, `system` | |
| `observed_at` | number | | |

No prompt-history file exists: `history.jsonl`, `input-history`, `prompt-history` and
`historyFile` have no match in `src/` at the reviewed commit.

#### Sharing, backup and placement records

- `session_nodes.entry_json` `publicShare` ([Database rows](#database-rows)) is written when the
  session's creator or a Gateway admin turns on Public access in the Control UI (the
  `session.publicShare.set` Gateway method), never for an incognito session, and removed when it is
  turned off
  ([`src/gateway/server-methods/sessions-sharing.ts:426-475`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/gateway/server-methods/sessions-sharing.ts#L426-L475)).
  A replacement entry for the same generation keeps it; a new `sessionId` (a reset or fork) drops
  it
  ([`session-accessor.sqlite-entry-write.ts:206-215`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/sessions/session-accessor.sqlite-entry-write.ts#L206-L215)).
  The public link is the session's normal thread URL `/chat/<agentId>/<sessionRef>`, not a
  secret; links issued earlier as `/share/session` tokens still work, each an encrypted form of
  the grant bound to the installation identity and not stored
  ([`docs/web/urls.md:272-278`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/docs/web/urls.md#L272-L278),
  [`:362-366`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/docs/web/urls.md#L362-L366)).
- `state/openclaw.sqlite` `backup_runs`, one row per `openclaw backup` attempt
  ([`openclaw-state-schema.sql:1843-1849`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/state/openclaw-state-schema.sql#L1843-L1849);
  writers [`src/commands/backup.ts:65-78`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/commands/backup.ts#L65-L78),
  [`src/commands/backup-git.ts:157-168`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/commands/backup-git.ts#L157-L168);
  codec [`backup-run-records.contract.ts:73-121`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/state/backup-run-records.contract.ts#L73-L121)).
  For a Git backup `archive_path` is the repository and `target` the commit; for an offsite archive
  `target` is the storage location name. The table keeps the newest 200 rows plus the newest attempt
  and newest success per kind and target
  ([`docs/cli/backup.md:664-677`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/docs/cli/backup.md#L664-L677)).

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `id` | string | | |
| `created_at` | number | | |
| `archive_path` | string | | |
| `status` | string | `ok`, `failed` | |
| `manifest_json` | string | | |
| `kind` | string | `archive`, `sqlite-snapshot`, `git`, `external` | |
| `target` | string | | |
| `namespace` | string | | |
| `error` | string | | |
| `pushFailed` | boolean | `true` | |
| `bytes` | number | | |
| `location` | object | | |
| `retention` | object | | |

`location` holds `name`, `provider`, `locationId`, `key`, `namespace`, `plaintextBytes` and
`storedBytes`; `retention` holds `kept` and `deleted`.

- `state/openclaw.sqlite` `worker_session_placements`, one row per session whose turns run on a
  paired device or a cloud worker
  ([`openclaw-state-schema.sql:2152-2265`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/state/openclaw-state-schema.sql#L2152-L2265)),
  deleted when the placement is retired
  ([`placement-store.ts:395-413`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/gateway/worker-environments/placement-store.ts#L395-L413)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `session_id` | string | | |
| `agent_id` | string | | |
| `session_key` | string | | |
| `execution_mode` | string or null | `worker-turn`, `remote-exec` | |
| `state` | string | `local`, `requested`, `provisioning`, `syncing`, `starting`, `active`, `draining`, `reconciling`, `reclaimed`, `failed` | |
| `environment_id` | string or null | | |
| `remote_workspace_dir` | string or null | | |
| `last_transcript_ack_cursor` | number or null | | |
| `created_at_ms` | number | | |
| `updated_at_ms` | number | | |

#### Source-host records

`state/openclaw.sqlite` keeps one row per push or pull request the Gateway made for a session in
`github_publication_requests`, `github_personal_publication_requests` and
`github_repository_publication_requests`
([`openclaw-state-schema.sql:2093-2144`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/state/openclaw-state-schema.sql#L2093-L2144),
[`:2441-2513`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/state/openclaw-state-schema.sql#L2441-L2513),
[`:2519-2562`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/state/openclaw-state-schema.sql#L2519-L2562)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `request_id` | string | | |
| `session_id` | string | | |
| `session_key` | string | | |
| `agent_id` | string | | |
| `identity_source` | string | `system-detected`, `system-configured`, `agent-override`, `personal` | |
| `identity_profile_id` | string or null | | |
| `identity_account_id` | number | | |
| `identity_login` | string | | |
| `status` | string | `requested`, `publishing`, `needs_confirmation`, `published`, `failed` | |
| `repository` | string or null | | |
| `branch` | string | | |
| `pull_request_url` | string or null | | |
| `created_at_ms` | number | | |

`projects` ([Where the project path is recorded](#where-the-project-path-is-recorded)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `id` | string | | |
| `display_name` | string | | |
| `repo_root` | string | | |
| `origin_url` | string or null | | |
| `source` | string | `registered`, `cloned` | |
| `created_at_ms` | number | | |
| `updated_at_ms` | number | | |

#### PostgreSQL engine

With `database.engine` `postgres` in `openclaw.json` the shared state, the agent databases
(transcripts included), and the Workboard, Logbook and Team Reports stores live on a PostgreSQL
server named by `database.postgres.connection`; `database.families.agent.engine` moves the agent
databases alone
([`docs/reference/database-engines.md:90-93`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/docs/reference/database-engines.md#L90-L93),
[`:109-120`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/docs/reference/database-engines.md#L109-L120)).
Each local `state/openclaw.sqlite` and `openclaw-agent.sqlite` is then a small SQLite anchor with
one table, `openclaw_engine_anchor`, whose `schema` is the server namespace
`<schemaPrefix>_<store>_<uuid-prefix>`
([`:702-706`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/docs/reference/database-engines.md#L702-L706);
[`src/infra/database-engine/open.ts:128`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/infra/database-engine/open.ts#L128)).
The transcript is then on the database server and no local file holds it; transcript payloads
are plain JSON there, never zstd. A responder who finds an anchor needs the DSN (in
`openclaw.json` or the environment, [Credentials](#credentials)) and the `schema` value to request
the server's copy. `client/credentials.sqlite` and `control/services.sqlite` stay local on every
engine.

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `store_id` | string | | |
| `engine` | string | `postgres` | |
| `schema` | string | | |

#### Auth profile store

`openclaw-agent.sqlite` `auth_profile_store`, one row per store key
([`openclaw-agent-schema.sql:647-651`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/state/openclaw-agent-schema.sql#L647-L651)),
whose `store_json` is `{version, profiles}`, `profiles` mapping each profile id to a credential
([`agents/auth-profiles/types.ts:33-39`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/agents/auth-profiles/types.ts#L33-L39),
[`:161-164`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/agents/auth-profiles/types.ts#L161-L164);
[`credential-schema.ts:44-67`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/agents/auth-profiles/credential-schema.ts#L44-L67)).
Its values are credentials ([Secrets](#secrets)); the parser never reads the table.

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `store_key` | string | | |
| `store_json` | string | | |
| `updated_at` | number | | |
| `version` | number | | |
| `profiles` | object | | |
| `type` | string | `api_key`, `token`, `oauth` | |
| `provider` | string | | |
| `key` | string | | |
| `token` | string | | |
| `access` | string | | |
| `refresh` | string | | |
| `expires` | number | | |

#### Audit, ingress and debug logs

Not transcripts, but they tie actions to sessions: `audit_events`
([Permission records](#permission-records)), the inbound chat messages of `channel_ingress_events`
([Past-use tables](#past-use-tables)) and the debug traces under `logs/` that hold full prompts
([Contents of the state directory](#contents-of-the-state-directory)). `session_windows` also
carries `row_transcript_json`, a cached preview of the session-list row
([`docs/reference/database-schemas/versioning.md:229-236`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/docs/reference/database-schemas/versioning.md#L229-L236)).

### Format versions

- Header `version`: current 4, minimum readable 3; versions 1 and 2 need doctor import and may
  lack `id` and `parentId`; the old `hookMessage` role becomes `custom` with `customType` `hook`
  ([`config/sessions/version.ts:1-4`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/sessions/version.ts#L1-L4);
  [`session-entry-codec.ts:163-188`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/sessions/session-entry-codec.ts#L163-L188)).
  Each database records its schema in `schema_meta.schema_version`
  ([`openclaw-agent-schema.sql:6-14`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/state/openclaw-agent-schema.sql#L6-L14)).
  The repository keeps sample databases of older schemas in `test/fixtures/sqlite/`: agent
  schemas 1, 14, 19, 21, 22 and 24 to 27, state schemas 1, 24 and 35
  ([`test/fixtures/sqlite/README.md`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/test/fixtures/sqlite/README.md)).
- Schema numbers: `3b16db7` (`2026.9.8`) wrote state 20 and agent 24, `146284a` (`2026.10.5` on
  `main`) state 28 and agent 31, `fc3490a` (`2026.10.9` on `main`) state 36 and agent 38. The
  release tag `v2026.10.9` writes state 20 and agent 25
  ([`package.json:5-9` at the tag](https://github.com/openclaw/openclaw/blob/8b18b5578f66eab1501cba4cef2310d88bfb9a41/package.json#L5-L9));
  its agent tables match `main`'s but for the cold archive chunks, and its state database still
  holds `device_identities`, `device_auth_tokens` and `gateway_origin_device_tokens` and has no
  `client/credentials.sqlite`. The vendor's histories describe each schema step
  ([state history](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/docs/reference/database-schemas/state-schema-history.md#L52-L61),
  [agent history](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/docs/reference/database-schemas/agent-schema-history.md#L43-L57)).
- State schema 29 moved the client private identities, token caches and the exec socket
  credentials out of `state/openclaw.sqlite` into `client/credentials.sqlite`
  ([state history](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/docs/reference/database-schemas/state-schema-history.md#L53)); the
  migration strips `socket.path` and `socket.token` from `exec_approvals_config.raw_json`
  ([`client-credential-migration-data.ts:167-205`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/infra/client-credential-migration-data.ts#L167-L205);
  [`client-credential-migration.ts:612`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/infra/client-credential-migration.ts#L612)).
  On `146284a` and the release tag, `raw_json` keeps `socket.token`, and the share-link key is
  derived from the `device_identities` row of `state/openclaw.sqlite`
  ([`control-ui-public-session-token.ts:196-217` at `3b16db7`](https://github.com/openclaw/openclaw/blob/3b16db73c0b5209aad2ee7c3fb5ddfb12fb208f9/src/gateway/control-ui-public-session-token.ts#L196-L217)).
- Agent schema 37 added `session_transcript_cold_archive_chunks`, splitting a cold archive into
  files of at most 64 MiB uncompressed
  ([agent history](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/docs/reference/database-schemas/agent-schema-history.md#L56)).
  `session_windows.row_transcript_json` and `row_transcript_updated_at` were added without a
  version step
  ([`versioning.md:229-236`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/docs/reference/database-schemas/versioning.md#L229-L236)).
  At `146284a` the agent schema already had the `navigation_*` and `message_role` columns of
  `transcript_events`
  ([`openclaw-agent-schema.sql:437-482` at `146284a`](https://github.com/openclaw/openclaw/blob/146284a289f98a2cf3babbc55cc3bd16c28e604b/src/state/openclaw-agent-schema.sql#L437-L482)).
- Legacy state directory: at `3b16db7` the runtime used `~/.clawdbot` when it existed and
  `~/.openclaw` did not, and doctor's rename left `~/.clawdbot` as a symlink (Windows junction) to
  the new directory
  ([`src/config/state-dir.ts:8-43` at `3b16db7`](https://github.com/openclaw/openclaw/blob/3b16db73c0b5209aad2ee7c3fb5ddfb12fb208f9/src/config/state-dir.ts#L8-L43);
  [`state-migrations.state-dir.ts:447-459` at `3b16db7`](https://github.com/openclaw/openclaw/blob/3b16db73c0b5209aad2ee7c3fb5ddfb12fb208f9/src/infra/state-migrations.state-dir.ts#L447-L459)).
  The runtime fallback and the link are gone at `146284a`, the release tag and `fc3490a`
  ([`src/config/state-dir.ts:32-37` at `146284a`](https://github.com/openclaw/openclaw/blob/146284a289f98a2cf3babbc55cc3bd16c28e604b/src/config/state-dir.ts#L32-L37));
  changed between `2026.9.8` and `2026.10.5`.
- Compression: at `3b16db7` an entry was compressed when the frame shrank it by at least 10%
  ([`transcript-payload.ts:215-272` at `3b16db7`](https://github.com/openclaw/openclaw/blob/3b16db73c0b5209aad2ee7c3fb5ddfb12fb208f9/src/config/sessions/transcript-payload.ts#L215-L272));
  at `fc3490a` the frame and its navigation projection must save at least the larger of 64 bytes
  and a tenth ([Transcript stores](#transcript-stores)), as at `146284a`
  ([`transcript-payload.ts:359` at `146284a`](https://github.com/openclaw/openclaw/blob/146284a289f98a2cf3babbc55cc3bd16c28e604b/src/config/sessions/transcript-payload.ts#L359)).
  Changed between `2026.9.8` and `2026.10.5`.
- Archive pruning: at `3b16db7` retention deleted a published archive row only once its file was
  gone
  ([`session-history-archive-pruning.worker.ts:105-127` at `3b16db7`](https://github.com/openclaw/openclaw/blob/3b16db73c0b5209aad2ee7c3fb5ddfb12fb208f9/src/config/sessions/session-history-archive-pruning.worker.ts#L105-L127));
  at `fc3490a` it removes the row and the file together
  ([What survives a delete](#what-survives-a-delete)).
- The file-name patterns moved from `config/sessions/artifacts.ts` to
  `config/sessions/transcript-file-names.ts` between `2026.9.8` and `2026.10.5`; the shell tool's
  name moved from `agents/core-tool-factory-descriptors.ts`
  ([`:20` at `3b16db7`](https://github.com/openclaw/openclaw/blob/3b16db73c0b5209aad2ee7c3fb5ddfb12fb208f9/src/agents/core-tool-factory-descriptors.ts#L20))
  to `agents/lazy-exec-tool.ts`, unchanged.
- The `exec_approvals_config` table and the approval tables are the same at `3b16db7`,
  `146284a` and `fc3490a`. The PostgreSQL engine is absent at `3b16db7`, which has no PostgreSQL
  source file, and experimental at `146284a`.
- Compaction checkpoints: a `<name>.checkpoint.<uuid>.jsonl` was a byte copy of the session file
  taken before compaction
  ([`src/gateway/session-compaction-checkpoints.ts:383-389`](https://github.com/openclaw/openclaw/blob/68145258676f1401a2e42cbaffe6564c2a6213c6/src/gateway/session-compaction-checkpoints.ts#L383-L389)
  at
  [`6814525`](https://github.com/openclaw/openclaw/commit/68145258676f1401a2e42cbaffe6564c2a6213c6),
  the parent of the commit that stopped copying,
  [`c59635a`](https://github.com/openclaw/openclaw/commit/c59635ae970cb70171c72fa422b15dc38319414f));
  `<name>` is `<sessionId>` or, for a session forked from a checkpoint, `<ts>_<sessionId>`
  ([`session-compaction-checkpoints.ts:320-332`](https://github.com/openclaw/openclaw/blob/68145258676f1401a2e42cbaffe6564c2a6213c6/src/gateway/session-compaction-checkpoints.ts#L320-L332)
  at `6814525`). No runtime writes new ones at `fc3490a`; the session store keeps only the
  references
  ([`legacy-compaction-history.ts:3-7`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/sessions/legacy-compaction-history.ts#L3-L7)).
- Pre-doctor repair copies: `<sessionId>.jsonl.pre-doctor-(branch|openai-codex)-repair-<ts>.bak` was
  a byte copy of a legacy transcript taken before `openclaw doctor` rewrote it
  ([`src/commands/doctor-session-transcripts.ts:249-292`](https://github.com/openclaw/openclaw/blob/6559e3f3b94c54e2b1aa06d1fca712c5360455ff/src/commands/doctor-session-transcripts.ts#L249-L292)
  at
  [`6559e3f`](https://github.com/openclaw/openclaw/commit/6559e3f3b94c54e2b1aa06d1fca712c5360455ff),
  the parent of
  [`4273ca9`](https://github.com/openclaw/openclaw/commit/4273ca9dbd9b3e3172f34a7231d41bcd8b5d200c),
  which removed the file-era runtime); the name is still classified at
  [`artifacts.ts:20-21`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/sessions/artifacts.ts#L20-L21). A branch repair kept only
  the active branch, so the backup also holds the abandoned branches.
- Public links: at `3b16db7` the public URL was `<control UI base>/share/session?token=<opaque>`
  ([`docs/web/urls.md:223-268` at `3b16db7`](https://github.com/openclaw/openclaw/blob/3b16db73c0b5209aad2ee7c3fb5ddfb12fb208f9/docs/web/urls.md#L223-L268)); at `fc3490a` it is
  the thread URL ([Sharing, backup and placement records](#sharing-backup-and-placement-records)).
  Changed between `2026.9.8` and `2026.10.9`.
- `toolOverrides.mcpServers` is a map of server names to booleans at `fc3490a`
  ([`sessions-row.ts:63-70`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/packages/gateway-protocol/src/schema/sessions-row.ts#L63-L70)).
- User messages carried `runtimeContextCarrier` and `runtimeContextCarrierRetained` through
  `v2026.9.7`, replaced by `runtimeContext`
  ([`llm-core/src/types.ts:388-397`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/packages/llm-core/src/types.ts#L388-L397)).

### Secrets

`openclaw-agent.sqlite`, the same database as the transcripts: `auth_profile_store.store_json` and
`auth_profile_state.state_json` (provider API keys and OAuth tokens) and `session_suggestions`
([`state/secret-state-tables.ts:57-61`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/state/secret-state-tables.ts#L57-L61);
[`openclaw-agent-schema.sql:647-657`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/state/openclaw-agent-schema.sql#L647-L657));
never emit them.

`state/openclaw.sqlite`: the credential-bearing tables the vendor's redaction list names, among
them `device_auth_tokens`, `device_identities`, `gateway_origin_device_tokens` (on builds before
the client credential store), `mcp_oauth_stores`, `secret_store_entries` (`value`) and
`worker_environment_credentials`, and `channel_ingress_events`
([`state/secret-state-tables.ts:2-23`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/state/secret-state-tables.ts#L2-L23);
[`openclaw-state-schema.sql:2768-2780`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/state/openclaw-state-schema.sql#L2768-L2780)).
`exec_approvals_config.raw_json` holds `socket.token` on builds before the client credential store
([Format versions](#format-versions)).

In records: `thinkingSignature` and `providerReplay.data` are opaque provider state;
`image.data` is base64 media; `toolResult.content` and `bashExecution.output` are raw tool output
and may contain anything the agent read, including `.env` files. Whoever holds the device
identity can mint working `/share/session` token links for every `publicShare` grant. Offsite
backup credentials (`accessKeyId`, `secretAccessKey`) and the encryption passphrase are
configured under `storage.locations` in the config file, as SecretRefs by preference
([`docs/concepts/storage-locations.md:79-91`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/docs/concepts/storage-locations.md#L79-L91)).

### Parser plan

Selection: `*/agent/openclaw-agent.sqlite` under an `openclaw` artifact (plus `-wal`);
`*/sessions/*.jsonl`, `*.jsonl.reset.*`, `*.jsonl.deleted.*` (decompress `.zst`),
`*.checkpoint.<uuid>.jsonl` and `*.jsonl.pre-doctor-*-repair-*.bak`; `state/openclaw.sqlite`.
Skip `*.trajectory.jsonl` and `*.migrated*`. SQL:
`SELECT e.rowid, e.session_id, e.seq, e.event_json, e.event_zstd, e.created_at, w.model, w.channel, w.session_key FROM transcript_events e LEFT JOIN session_windows w USING (session_id) ORDER BY e.session_id, e.seq`.
Then the archive blobs.

| Column | Source |
| --- | --- |
| timestamp_utc | entry `timestamp`; else `message.timestamp` (ms); else `created_at` (ms) |
| session_id | `session_id` / header `id` |
| project_path | header `cwd` of the session |
| git_branch | empty |
| turn_type | role `user` → user; `assistant` text → assistant, `toolCall` block → tool_use, `thinking` block → thinking; `toolResult` → tool_result; `bashExecution` → tool_use (tool `bash`); other entry types and roles → system |
| model | assistant `responseModel` or `model`; `model_change.modelId` |
| tool_name | `toolCall.name` / `toolResult.toolName` |
| tool_use_id | `toolCall.id` / `toolResult.toolCallId` |
| text | text blocks; `toolCall.arguments.command`/`path`/`url` else JSON; tool result text; `summary` for compaction; `reason` for reset |
| source_line | `transcript_events.rowid`, or the JSONL line number |

The file patterns, matched against the home-relative path:

```text
^(?:\.openclaw(?:-[^/]+)?|\.clawdbot|\.moltbot)/agents/[^/]+/agent/openclaw-agent\.sqlite$
^(?:\.openclaw(?:-[^/]+)?|\.clawdbot|\.moltbot)/state/openclaw\.sqlite$
^(?:\.openclaw(?:-[^/]+)?|\.clawdbot|\.moltbot)/(?:agents/[^/]+/)?sessions/(?P<name>[^/]+?)(?:(?P<archive>\.jsonl\.(reset|deleted)\.\d{4}-\d\d-\d\dT\d\d-\d\d-\d\d(?:\.\d{3})?Z(?:\.[0-9a-f]{32})?(?:\.zst)?)|(?P<copy>\.checkpoint\.[0-9a-fA-F-]{36}\.jsonl|\.jsonl\.pre-doctor-(?:branch|openai-codex)-repair-\d{4}-\d\d-\d\dT\d\d-\d\d-\d\d-\d{3}Z\.bak)|\.jsonl)$
^(?:\.openclaw(?:-[^/]+)?|\.clawdbot|\.moltbot)/(?:agents/[^/]+/)?sessions/cold/[0-9a-f]{64}\.jsonl\.zst$
\.trajectory$
```

The last skips a JSONL file whose name, less `.jsonl`, ends in `.trajectory`.

Each session opens with a `system` row
`session start: key=<session_key> <window columns> version=<version> parent_session=<parentSession>`,
the window columns being `channel`, `account_id`, `chat_type`, `reason`, `previous_session_id`,
`parent_session_key`, `spawned_by`, `display_name` and `model_provider` as `<column>=<value>` when
set; an archive blob adds `archive=<reason> name=<archive_name>`, a cold archive
`cold_archive=<archive_name>`, and a copy file `archive=<reason>.<ts>` or `copy=<suffix>`. Entry
types other than `message` give `system` rows: `thinking level: <thinkingLevel>`,
`compaction (tokens_before=<tokensBefore> tokens_after=<tokensAfter>): <summary>`,
`reset: <reason>`, `branch summary: <summary>`, `custom <customType>: <data>`,
`custom message <customType>: <content>`, `label: <label> (entry <targetId>)` and
`session name: <name>`; any other type gives `<type>: <the record as JSON>`. A user message with
`runtimeContext`, or the older `runtimeContextCarrier`, is a `system` row
`runtime context: <text>`, not a human turn. An assistant message whose `stopReason` is `error` or
`aborted`, or that has `errorMessage`, adds `assistant <stopReason>: <errorMessage>`; a
`branchSummary` or `compactionSummary` message is `branch summary: <summary>` or
`compaction summary: <summary>`. A cold archive stored in the database is read from its
`archive_blob`, or, when `session_transcript_cold_archive_chunks` has rows for it, from each
chunk's `archive_blob` in `chunk_index` order; only its `event` records give rows, the session id
coming from the `header`; an `event_json` that does not decode is
`parser: cold archive event seq <seq> unreadable: <reason>`. The rows below rest on source alone;
each subsection says which are fixture only.

#### Permission rows

No transcript entry records a per-call permission decision; `bashExecution.cancelled` stays on the
bash `tool_result` row. The approvals and the standing policy are rows of `state/openclaw.sqlite`
and of each session's `entry_json` ([Permission records](#permission-records)). The parser yields,
all fixture only:

- One row per `operator_approvals` row, at `resolved_at_ms` (`created_at_ms` while pending), with
  `session_id` `source_session_id`, `tool_name` `source_tool_name` and `tool_use_id`
  `source_tool_call_id`, so it sits beside the call it governed:
  `permission decision: <decision, else status> status=<status> kind=<kind> resolver=<resolver_kind> reason=<terminal_reason> approval <approval_id> (from state/openclaw.sqlite operator_approvals, <resolved_at_ms or created_at_ms>)`.
- One `permission setting: <scope>.<setting> (from state/openclaw.sqlite exec_approvals_config, updated_at_ms)`
  row per `security`, `ask`, `askFallback` and `autoAllowSkills` value (`<key>=<value>`), per
  allowlist entry (`allowlist=<pattern>[ argPattern=<argPattern>] source=<source>`) and per MCP
  tool grant (`mcpTools=<server>/<tool> source=<source>`) of `raw_json`'s `defaults` and of each
  `agents.<id>`, `<scope>` being `defaults` or `agents.<id>`. `socket` is never read
  ([Secrets](#secrets)); a `raw_json` that does not decode is one `parser:` row that withholds the
  text, since on older builds the text holds the token.
- `permission setting: standing grant <grant_id> allow-always cron job <cron_job_id> agent <agent_id> binding <operation_binding> approval <minted_by_approval_id> expires=<expires_at_ms> revoked=<revoked_at_ms> uses=<use_count> (from state/openclaw.sqlite operator_approval_standing_grants, created_at_ms)`
  per standing grant.
- `permission setting: channel <channel_key> account <account_id> allowFrom=<entry> (from state/openclaw.sqlite channel_pairing_allow_entries, updated_at)`
  per allowed sender.
- One `permission setting: <key>=<value> (from openclaw-agent.sqlite session_nodes.entry_json, updated_at)`
  row per `elevatedLevel`, `execHost`, `execNode`, `execCwd`, `toolOverrides` (as JSON) and
  `sendPolicy` of a session's `entry_json`, after the start row of its current generation, at
  `session_nodes.updated_at`.

Not read: `audit_events`, whose `kind` and `action` values the schema leaves open.

#### Model rows

`system` row `model: <id> provider=<provider>` at the first model a session names and
`model: <old> -> <new> provider=<provider>` on every later entry whose model differs, with the
`model` column set to the new id. Sources, in order: the `session_windows` `model` and
`model_provider` (the row follows the session-start row, at its time), `model_change` `modelId`
and `provider`, and the assistant `responseModel`, else `model`, with `provider`; when
`responseModel` differs from `model` the row ends ` requested=<model>`. A `model_change` entry
naming the model already in force still yields the plain `model: <id> provider=<provider>` row,
since it records a switch; an assistant message that repeats the current model yields none.

The session's model override ([Database rows](#database-rows)) gives one
`model: <modelOverride> provider=<providerOverride> source=<modelOverrideSource> (from openclaw-agent.sqlite session_nodes.entry_json modelOverride, updated_at)`
row, with ` fallback_from=<provider>/<model>` before the parenthesis when
`modelOverrideFallbackOriginProvider` or `modelOverrideFallbackOriginModel` is set, after the
start row of the session's current generation, at `session_nodes.updated_at`, the `model` column
the override. It records a `/model` the user typed even when no turn followed. The entry keeps
only the current override, so the row names no previous model. Fixture only.

#### Past-use rows and secondary sources

Every source is parsed on every run, and a row of a secondary source is dropped when a primary
store's row records the same event (see
[Secondary sources in `analyzer/docs/output.md`](../analyzer/docs/output.md#secondary-sources)).

- Primary: `openclaw-agent.sqlite`, all of it: `transcript_events`, the cold archives and every
  `session_transcript_archives` blob, whether or not its `published_at` is set (the time the blob
  was also written to the sessions directory as a file). The schema calls that table the
  "canonical cold-tier owner for reclaimed transcript generations", from which "the derived
  .deleted/.reset file may be recreated"
  ([`openclaw-agent-schema.sql:484-485`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/state/openclaw-agent-schema.sql#L484-L485)),
  and doctor "repairs canonical blobs before their reconstructible files"
  ([`state-migrations.transcript-directives-archives.ts:288`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/infra/state-migrations.transcript-directives-archives.ts#L288)),
  so the table, not the file, is the store of record. The legacy plain `<sessionId>.jsonl` is
  primary too: before doctor imports it, it is the only copy.
- Secondary, the published `.jsonl.reset.*` and `.jsonl.deleted.*` files: the same entries as the
  blob. A file survives the database being lost or excluded.
- Secondary, `<name>.checkpoint.<uuid>.jsonl`: a pre-compaction byte copy of the session file
  ([Format versions](#format-versions)). It holds the entries a compaction later summarised, which
  survive in the legacy file or, after import, in `transcript_events`; it is a copy of nothing the
  primary store lost unless that store is gone.
- Secondary, `<sessionId>.jsonl.pre-doctor-(branch|openai-codex)-repair-<ts>.bak`: a byte copy of
  a legacy transcript before doctor rewrote it ([Format versions](#format-versions)); its rows for
  the abandoned branches, which no other store has, are kept.
- Not read: `<id>.trajectory.jsonl`, the trajectory flight recorder (a per-run log of runtime
  events: prompt submitted, tool call, model completed), a different schema whose records carry
  `traceSchema` `openclaw-trajectory` and `schemaVersion`
  ([`docs/tools/trajectory.md:76-107`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/docs/tools/trajectory.md#L76-L107)),
  not a transcript copy; `*.migrated[.N]`
  ([`artifacts.ts:22`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/sessions/artifacts.ts#L22)), for which no writer of a session
  transcript is known (the suffix marks migrated cron logs, pairing stores, auth sources and audit
  logs); `<id>.jsonl.bak.<ts>`, the legacy compact backups
  ([`artifacts.ts:29-36`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/sessions/artifacts.ts#L29-L36)), whose content is not
  recorded; and `memory_index_chunks`.

Joins: the copies are byte copies, so their entries carry the same `id`, `timestamp` and header
`cwd` as the primary store's rows, and the parser takes the session id of every JSONL file from its
header `id` (the name before the suffix only when there is no header), the id
`transcript_events.session_id` and the archive table carry. An archive whose session id is over
96 bytes is named `session-<sha256>`
([`session-accessor.sqlite-archive-artifact.ts:23`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/sessions/session-accessor.sqlite-archive-artifact.ts#L23),
[`:32-37`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/sessions/session-accessor.sqlite-archive-artifact.ts#L32-L37)),
which the header id overrides. Every entry row of a copy therefore pairs with its primary twin at
the same time (a 2-second tolerance is not needed but is the default). The session-start row does
not: a copy's names the file (`archive=<reason>.<ts>` or `copy=<suffix>`) where the database's
carries the `session_windows` and archive columns, so each copy keeps one row showing it exists.

The full-text index and the pending inputs repeat transcript entries inside the primary database,
where the file-level secondary flag cannot mark them, so the parser joins them on the entry id
itself and marks what it keeps `recovered`; all fixture only ([Past-use tables](#past-use-tables)):

- `session_pending_inputs`: a row with no `consumed_event_id` is primary and yields a `user` row
  and
  `pending input: <state>, not consumed by the transcript (input <input_id> run <run_id>; from openclaw-agent.sqlite session_pending_inputs, accepted_at)`,
  at `accepted_at`; a consumed row yields its `user` row, recovered, only when no transcript entry
  of the database has the `consumed_event_id`. A `message_json` that does not decode is its own
  `parser:` row.
- `session_transcript_fts`, read from `session_transcript_fts_content` as a plain table: a `user`
  or `assistant` row (else `system`, `<role>: <text>`) per indexed entry whose
  `(session_id, message_id)` no transcript row carries, recovered.
- `conversation_deliveries` joined to `conversations`, with `session_id` the current generation of
  `source_session_key`:
  `channel delivery: <operation_kind> <status> channel <channel> conversation <conversation_id> platform message <platform_message_id> (from openclaw-agent.sqlite conversation_deliveries, created_at)`,
  with ` error=<rejection_error>` before the parenthesis for a rejected send, and, when a platform
  message replied to the delivery,
  `channel delivery reply: <reply_text> (reply message <reply_message_id>; from openclaw-agent.sqlite conversation_deliveries, reply_timestamp)`.
  The outbound text is stored only as `message_hash`, so the rows repeat no transcript text.
- `channel_ingress_events` of `state/openclaw.sqlite`:
  `channel ingress: <channel_id> account <account_id> status <status>: <payload_json> (from state/openclaw.sqlite channel_ingress_events, received_at)`,
  the payload whole since its shape differs per channel, with no session id; it cannot be matched
  to the transcript.

`memory_index_chunks` is proposed, not read: its `path` names the session
([Past-use tables](#past-use-tables)), so a chunk can be joined to its session by path and to an
entry by text.

#### Sync, relay and origin rows

Sync: OpenClaw keeps no copy of a session on a vendor service. Session content leaves the Gateway
host in three ways, all operator actions; the first two leave a local record
([Sharing, backup and placement records](#sharing-backup-and-placement-records)), and both are
parsed (`records_sync`).

- Public link: `entry_json` `publicShare` marks a session whose user messages and final answers
  anyone with the URL can read, served by the Gateway, opt-in per session
  ([`docs/concepts/multi-user.md:21-36`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/docs/concepts/multi-user.md#L21-L36)).
  The parser yields one `system` row after the generation's session-start row:
  `session synced: public share <publicShare.id> generation <publicShare.sessionId> (from openclaw-agent.sqlite session_nodes.entry_json, publicShare.createdAt)`,
  timestamped by `createdAt`, with `session_id` the generation; a grant whose generation has no
  transcript rows in the database still yields its row. Fixture only. Turning access off removes
  the key, and deleting the session deletes its `session_nodes` row; the `.jsonl.deleted.*` archive
  and its blob have no column for the grant, so no local file says a deleted or unpublished session
  was ever public.
- Backup upload: `openclaw backup create --to <location>` uploads the whole state directory,
  every agent database included, to a configured storage location (a mounted disk, or Cloudflare
  R2, the Cloudflare object store, through a plugin) as
  `backups/<namespace>/<UTC ts>-<8 hex>.tar.gz`, and `openclaw backup git create --push` pushes
  per-table dumps to a Git remote; both are opt-in and can be scheduled with
  `openclaw backup enable`
  ([`docs/cli/backup.md:146-156`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/docs/cli/backup.md#L146-L156),
  [`:176-181`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/docs/cli/backup.md#L176-L181),
  [`:505`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/docs/cli/backup.md#L505)).
  The marker is host-wide, not per session: each `backup_runs` row says every session in the
  uploaded databases at `created_at` left the host. The parser yields one `system` row per
  `backup_runs` row with `status` `ok` and either a `location` or a Git `target` without
  `pushFailed`, with an empty `session_id`, in the agent's own term rather than `session synced:`
  because it marks no one session:
  `backup uploaded: <kind> to <location.name or archive_path> key <location.key or target> namespace <location.namespace or namespace or -> (from state/openclaw.sqlite backup_runs, created_at)`,
  timestamped by `created_at`. A Git backup run without `--push` records the same manifest as a
  pushed one, `target` the commit and no `pushFailed`, so a Git row says a commit was made and may
  have been pushed. Fixture only. Deleting a session does not touch `backup_runs`, and the
  uploaded archives keep the session.
- Remote execution: a session placed on a paired device or a cloud worker sends its workspace and
  turn context to that machine while the Gateway keeps the canonical transcript
  ([`docs/gateway/cloud-sessions.md:10-20`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/docs/gateway/cloud-sessions.md#L10-L20)).
  This is execution, not a copy kept elsewhere, and no row is proposed; `worker_session_placements`
  names the environment while the placement lives.

Not sync: the daily update check and the opt-in anonymous statistics carry no prompt or reply
text
([`docs/gateway/telemetry.md:11-24`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/docs/gateway/telemetry.md#L11-L24));
OpenTelemetry export carries no prompt or response text, and log message bodies only when
`diagnostics.otel.captureContent` is `true`, to the operator's own collector
([`docs/gateway/opentelemetry/privacy-and-trace-context.md:47-56`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/docs/gateway/opentelemetry/privacy-and-trace-context.md#L47-L56)).

Relay (`records_relay`; fixture only): the records are under
[Joins and timestamps](#joins-and-timestamps). After a generation's start row the parser yields
one
`session relayed: <channel> account <account_id> <kind> peer <peer_id> thread <thread_id> role <role> (from openclaw-agent.sqlite session_conversations and conversations, first_seen_at)`
row per `session_conversations` link, at `first_seen_at`; for a generation with no such link,
`session relayed: <channel> account <account_id> <chat_type> (from openclaw-agent.sqlite session_windows, started_at)`
from its window; and for a session with neither, from an `entry_json.delivery` of kind `external`,
`session relayed: <route.channel> account <account> <origin.chatType> from <origin.from> to <origin.to> (from openclaw-agent.sqlite session_nodes.entry_json delivery, updated_at)`.
A window whose `hook_external_content_source` is `gmail`, a session a Gmail hook started, is an
email relay:
`session relayed: gmail hook (from openclaw-agent.sqlite session_windows hook_external_content_source, started_at)`.
The Control UI channel `webchat` and the internal wake sources `heartbeat`, `cron`, `webhook`,
`voice` and `sessions_send` are not platforms and give no row
([`src/utils/message-channel-constants.ts:3`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/utils/message-channel-constants.ts#L3),
[`:14-20`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/utils/message-channel-constants.ts#L14-L20)).
The start row still carries `channel`, `account_id` and `chat_type`.

Origin: the agent records none; no session is created on a vendor server. A PostgreSQL anchor
([PostgreSQL engine](#postgresql-engine)) yields one whole-file row, fixture only:
`parser: agent database is a PostgreSQL anchor (store <store_id>, schema <schema>): transcripts are on the database server`,
or for `state/openclaw.sqlite`
`parser: state database is a PostgreSQL anchor (store <store_id>, schema <schema>): the Gateway state is on the database server`,
so the gap is visible.

#### Scratch directory rows

The [scratch directories](#scratch-directories) are under the state directory, which the
analyzer's catalog rule classes as scratch. Proposed, not yet a row: a `scratch directory:` row per
`state/openclaw.sqlite` `projects` row whose `source` is `cloned` and per `worktrees` row, so a
state directory moved by `OPENCLAW_STATE_DIR` is classed too.

#### Deletion rows

A deleted archive shows as an `archive=deleted.<ts>` session-start row
([What survives a delete](#what-survives-a-delete)), and three rows mark a delete, an archive and
a forget, all fixture only:

- `session deleted: <session_id> archive <archive_name> (from openclaw-agent.sqlite session_transcript_archives reason deleted, created_at)`
  after the rows of each `session_transcript_archives` blob whose `reason` is `deleted`, at
  `created_at`, even when the blob does not decompress.
- `session archived: <archiveReason> by <archivedBy.type>:<archivedBy.id> (from openclaw-agent.sqlite session_nodes.entry_json, archivedAt)`
  after the start row of the session's current generation, at `archivedAt`.
- `session forgotten from memory: <session_id> agent <agent_id> reason=<reason> (from openclaw-agent.sqlite memory_session_tombstones, created_at)`
  per tombstone, with the `cwd` of the session's transcript when the database still holds one.

#### Bad-line rows

A legacy `sessions.json` that cannot be read, does not decode or is not a JSON object yields one
whole-file `system` row in each transcript beside it, after that transcript's line rows, with
`source_line` 0 and the transcript's `session_id` and `cwd`:
`parser: sessions.json unreadable (<reason>)`, `parser: sessions.json unparseable (<reason>)` or
`parser: sessions.json is not a JSON object`; the transcript's own rows are still yielded, without
the session key. A line that is valid JSON but not an object (an array, a string, a number,
`null`), in a JSONL file or an archive blob, is its own row like any other bad line,
`parser: unparseable line (not a JSON object): <line>` (prefixed `archive <name> line <n>: ` inside
a blob), and the lines around it are still yielded. Fixture only.

#### Fixtures

Transcript entries as the writers at `fc3490a` produce them, JSONL form; the SQLite form stores
the same strings in `event_json`. The header comes from `createSessionTranscriptHeader`
([`transcript-header.ts:6-29`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/sessions/transcript-header.ts#L6-L29)), the entries
from the session manager's entry types ([Entries](#entries), [Message roles](#message-roles)):

```json
{"type":"session","version":4,"id":"5c1f9e2a-3b4d-4e6f-8a0b-1c2d3e4f5a6b","timestamp":"2026-10-28T14:02:11.204Z","cwd":"/home/riley/.openclaw/workspace"}
{"type":"message","id":"e91c04aa","parentId":null,"timestamp":"2026-10-28T14:02:12.010Z","message":{"role":"user","content":"how full is the root filesystem?","timestamp":1793196132010}}
{"type":"message","id":"f02d15bb","parentId":"e91c04aa","timestamp":"2026-10-28T14:02:15.433Z","message":{"role":"assistant","content":[{"type":"thinking","thinking":"Run df on the root mount."},{"type":"toolCall","id":"toolu_7Q2","name":"exec","arguments":{"command":"df -h /"}}],"api":"anthropic-messages","provider":"anthropic","model":"claude-opus-4-1","usage":{"input":1210,"output":52,"cacheRead":0,"cacheWrite":0,"totalTokens":1262,"cost":{"input":0,"output":0,"cacheRead":0,"cacheWrite":0,"total":0}},"stopReason":"toolUse","timestamp":1793196135433}}
{"type":"message","id":"a13e26cc","parentId":"f02d15bb","timestamp":"2026-10-28T14:02:16.002Z","message":{"role":"toolResult","toolCallId":"toolu_7Q2","toolName":"exec","content":[{"type":"text","text":"/dev/nvme0n1p2  98G  61G  33G  65% /"}],"isError":false,"timestamp":1793196136002}}
{"type":"model_change","id":"b24f37dd","parentId":"a13e26cc","timestamp":"2026-10-28T14:03:40.118Z","provider":"openrouter","modelId":"openai/gpt-5.1"}
{"type":"reset","id":"c35a48ee","parentId":"b24f37dd","timestamp":"2026-10-28T20:00:00.000Z","reason":"idle"}
```

A `session_nodes.entry_json` with the delivery route, model override, archive and permission keys
the parser reads ([Database rows](#database-rows)), and the `raw_json` of an
`exec_approvals_config` row as `writeExecApprovalsConfigRow` stores it, without `socket`
([`infra/exec-approvals-sqlite.ts:165-186`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/infra/exec-approvals-sqlite.ts#L165-L186)):

```json
{"sessionId":"5c1f9e2a-3b4d-4e6f-8a0b-1c2d3e4f5a6b","updatedAt":1793210400000,"delivery":{"kind":"external","route":{"channel":"telegram","accountId":"default"},"context":{},"origin":{"provider":"telegram","chatType":"direct","from":"telegram:5550001","accountId":"default"}},"providerOverride":"openrouter","modelOverride":"openai/gpt-5.1","modelOverrideSource":"user","elevatedLevel":"ask","execHost":"sandbox","toolOverrides":{"mcpServers":{"github":false}},"sendPolicy":"allow","archivedAt":1793214000000,"archivedBy":{"type":"human","id":"operator:riley"},"archiveReason":"manual"}
{"version":1,"defaults":{"security":"allowlist","ask":"on-miss","askFallback":"deny"},"agents":{"main":{"autoAllowSkills":false,"allowlist":[{"pattern":"/usr/bin/df","source":"allow-always","lastUsedAt":1793196135500}],"mcpTools":[{"server":"github","tool":"search_code","source":"allow-always","addedAt":1793100000000}]}}}
```

#### Source host rows

OpenClaw connects a GitHub account as a managed identity per Gateway or per agent, an OAuth
sign-in or a GitHub App installation, which its bundled GitHub plugin (the `github_read` and
`github_search` tools) and its publication flow use
([`extensions/github/README.md:12-14`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/extensions/github/README.md#L12-L14),
[`:57-59`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/extensions/github/README.md#L57-L59);
[`src/config/zod-schema.github-identity.ts:6-43`](https://github.com/openclaw/openclaw/blob/fc3490aa24e624f0182ae5e29482611fee5736fb/src/config/zod-schema.github-identity.ts#L6-L43)).
The token is local ([Credentials](#credentials)). A GitHub App installation lists the repositories
it may reach as `repositories`, a list of `{id, fullName}`, in that config entry (same lines).
The repository-bound records are the GitHub publication requests
([Source-host records](#source-host-records)). Each yields, fixture only,
`source host: github account <identity_login> id <identity_account_id> identity <identity_source>; publication <status> repository <repository> branch <branch> pull request <pull_request_url> (from state/openclaw.sqlite <table>, created_at_ms)`
at `created_at_ms`, `session_id` the session. Not read: the App installation's `repositories`
list, since the parser reads no `openclaw.json` (JSON5), so it gives no `source host repository:`
rows.

#### Git remote rows

Each `projects` row ([Source-host records](#source-host-records)) whose `origin_url` is set yields,
fixture only, `git remote: <origin_url>` (`system`) at the row's `created_at_ms`, with
`project_path` its `repo_root` and an empty `session_id`: the table names a repository the Gateway
registered or cloned, not a session. Its other columns are not read.

## Confidence

- High: state directory resolution, profiles, database file names, table and column names, the
  archive file naming, the credential tables, service names, the entry and message field names,
  the compression rule and the version constants, all read from source at `fc3490a`.
- Medium: the exclusion list rests on the backup command's skip list rather than each writer;
  `models.json` holding literal keys is inferred from the doctor migration;
  `browser/*/user-data/*/Cache` assumes Chromium's usual `Default/` profile layout; the
  `.moltbot` layout is assumed identical to `.clawdbot`; `cwd` meaning the workspace.
- Released builds: a host running the tag `v2026.10.9` writes the tag's schema, not `main`'s;
  only the tables and constants compared in [Format versions](#format-versions) were read at the
  tag.
- Claims for a real-install round: which build writes `client/credentials.sqlite` and whether the
  legacy `identity/*.json` files stay after import; the sizes of `projects/`, `worktrees/`,
  `tools/` and the `cold/` files; that no state lives in a Gateway clone; what a workspace's own
  `tmp/` and `tools/` hold; that legacy `.jsonl` files are removed after import on every path;
  every fixture-only row above.
- Not determined: the Windows log location beyond `os.tmpdir()`; what the macOS transcript cache
  stores; whether the iOS and Android apps keep transcripts on the phone; the shapes of
  `trajectory_runtime_events`, `acp_parent_stream_events`, `custom.data` per `customType` and
  `channel_ingress_events.payload_json` per channel; whether cold-archived generations keep any
  `transcript_events` rows; whether a session delete removes its `session_transcript_fts_rows`;
  the `audit_events` `kind` and `action` values.
