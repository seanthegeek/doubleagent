# opencode

OpenCode (catalog agent `opencode`) is an open-source coding agent with a terminal user interface
(TUI), a background server that every client starts and shares, and an Electron desktop app that
drives the same server; its repository moved from `sst/opencode` to `anomalyco/opencode`. The 2.x
line, the subject of this document, keeps every session in one SQLite database under the XDG data
directory (the freedesktop.org base-directory layout, `~/.local/share` and its siblings, which
OpenCode uses on every platform); the 1.x line kept the same database with an older table layout,
described under [Format versions](#format-versions). This document rests on OpenCode 2.2.5 at
commit [`bf544cd`](https://github.com/anomalyco/opencode/commit/bf544cd419c5f17535c0d5af929f7ecd41188320). Kilo Code's store is an OpenCode fork and its parser subclasses this one,
so the per-record error rows and the `session_v2` reading apply to Kilo Code's database too
([`kilo-code.md`](kilo-code.md)); Crush began as an OpenCode fork but shares no storage code
([`crush.md`](crush.md)). Letta Code (at
[`2a46615`](https://github.com/letta-ai/letta-code/commit/2a46615d16babb06cf5450b8e57ecb9bdbf2f436))
can drive OpenCode as an external coding agent, starting `opencode serve` in its working directory
([src/agent/subagents/index.ts:123-159](https://github.com/letta-ai/letta-code/blob/2a46615d16babb06cf5450b8e57ecb9bdbf2f436/src/agent/subagents/index.ts#L123-L159),
[src/tools/impl/opencode-server.ts:51-66](https://github.com/letta-ai/letta-code/blob/2a46615d16babb06cf5450b8e57ecb9bdbf2f436/src/tools/impl/opencode-server.ts#L51-L66)),
so a session Letta Code started lands in this store like any other OpenCode session
([Source and evidence in `letta.md`](letta.md#source-and-evidence)). The analyzer parses the 1.x
and 2.x database layouts and the TUI's prompt history, stash and pinned sessions, from source and
fixtures only.

## Source and evidence

Open source, MIT. [anomalyco/opencode](https://github.com/anomalyco/opencode) (the old
`sst/opencode` name redirects to it; default branch `dev`) at [`bf544cd`](https://github.com/anomalyco/opencode/commit/bf544cd419c5f17535c0d5af929f7ecd41188320), committed
2026-10-31, whose packages all give version 2.2.5
([`packages/cli/package.json:4`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/cli/package.json#L4),
[`packages/core/package.json:3`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/package.json#L3)). The latest release tag,
`v2.2.5` of 2026-10-30, points at the release commit `9bbb8ce` on a release branch; its database
schema, migrations and session schemas are the same as at `bf544cd`, so what a released 2.2.5 build
writes is what this document describes. The latest 1.x release tag is `v1.19.5` (commit
`810c0eb`, 2026-10-28). Paths cited are relative to the repository root: the database and session
code is in `packages/core`, the record schemas in `packages/schema`, the CLI and background server
in `packages/cli` and `packages/server`, the TUI in `packages/tui`, the shared paths in
`packages/util` and the desktop app in `packages/desktop`. Every claim is from source at `bf544cd`
unless it names an older commit or the vendor's documentation, which the repository holds under
[`packages/web/src/content/docs`](https://github.com/anomalyco/opencode/tree/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/web/src/content/docs)
and which is linked at its claim. Two older commits are cited in
[Format versions](#format-versions) for shapes `bf544cd` no longer writes:
[`cb07c55`](https://github.com/anomalyco/opencode/commit/cb07c555d4d1ca34430b7857b52f104c3d95e09e) (2.0.32) and [`907b3bc`](https://github.com/sst/opencode/commit/907b3bc518fa48e90e8ec24dd327d13eee71c36c) (the 1.x line).

### Observed

No real install. The source review of [`bf544cd`](https://github.com/anomalyco/opencode/commit/bf544cd419c5f17535c0d5af929f7ecd41188320) (2.2.5) stands in; every row of the
[Parser plan](#parser-plan) is source-derived and its samples are fixture only, and no real install
has shown a `session_share` row, a shared session's events or a `session_part` row.

### Source reviews

- 2026-10-31: anomalyco/opencode at [`bf544cd`](https://github.com/anomalyco/opencode/commit/bf544cd419c5f17535c0d5af929f7ecd41188320) (2.2.5). Re-cited every claim at the new
  commit; added the `session_part`, `blob`, `blob_ref`, `schedule` and `job` tables, the `system`
  message record, the shell output and artifact directories, the persistent terminal daemon's
  registration, the CLI settings file `cli.json`, the desktop app's staged CLI copies, logs and
  storage tables; settled that the desktop app uses the shared background server and its
  database; moved `auth.json`, `mcp-auth.json`, `kv.json`, `tui.json`, the `storage/` JSON tree,
  session sharing and `opencode github` to Format versions as 1.x shapes; corrected the saved
  permission id prefix to `psv_`.
- 2026-10-21: anomalyco/opencode at [`cb07c55`](https://github.com/anomalyco/opencode/commit/cb07c555d4d1ca34430b7857b52f104c3d95e09e) (2.0.32): the parser plan's permission,
  past-use, scratch directory and deletion items were read from source alone, the 1.x prompt
  history and stash shapes at [`907b3bc`](https://github.com/sst/opencode/commit/907b3bc518fa48e90e8ec24dd327d13eee71c36c) were added, the `service.json` files, the desktop
  app and the Windows temp root got catalog lines, and the source-host integration was reviewed.
- 2026-10-19, `dev` at [`cb07c55`](https://github.com/anomalyco/opencode/commit/cb07c555d4d1ca34430b7857b52f104c3d95e09e) (2.0.32): added the 2.0 line throughout: the TUI state
  files, the desktop app, the background server's `service.json`, the permission records, the temp
  root and worktrees, the 2.0 delete and the `session_v2` layout.
- 2026-10-17, session sharing at [`907b3bc`](https://github.com/sst/opencode/commit/907b3bc518fa48e90e8ec24dd327d13eee71c36c) and the default branch at
  [`797d2af`](https://github.com/sst/opencode/commit/797d2afdbf8f0b4f189748373ff1a69bde9e0ff7):
  added the share records and the `session synced:` rows.

## Collection

### Per-user storage

The roots are built in
[`packages/util/src/global-roots.ts:4-19`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/util/src/global-roots.ts#L4-L19)
and [`packages/util/src/global.ts:12-27`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/util/src/global.ts#L12-L27): data, cache,
config and state are `opencode` under `XDG_DATA_HOME` or `~/.local/share`, `XDG_CACHE_HOME` or
`~/.cache`, `XDG_CONFIG_HOME` or `~/.config` and `XDG_STATE_HOME` or `~/.local/state`, with no
platform branch, so the paths are the same on Linux, macOS and Windows (`~` is `%USERPROFILE%` on
Windows); `OPENCODE_CONFIG_DIR` replaces the config root
([`global.ts:79`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/util/src/global.ts#L79)) and `OPENCODE_TEST_HOME` the home
([`global.ts:17`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/util/src/global.ts#L17)). The vendor's
[Storage section of the troubleshooting page](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/web/src/content/docs/troubleshooting.mdx#L23-L37)
gives `~/.local/share/opencode/` on macOS and Linux and `%USERPROFILE%\.local\share\opencode` on
Windows. `opencode debug paths` prints the roots and the database path
([`packages/cli/src/commands/handlers/debug/paths.ts:12`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/cli/src/commands/handlers/debug/paths.ts#L12)).

#### Scratch directories

- `<system temp directory>/opencode`, the temp root
  ([`global-roots.ts:17`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/util/src/global-roots.ts#L17)), created at start
  ([`global.ts:63-75`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/util/src/global.ts#L63-L75)); the environment block tells the
  model to prefer it over `/tmp`, as a directory "pre-created and approved for external access"
  ([`packages/core/src/instructions/builtins.ts:43`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/instructions/builtins.ts#L43)).
  Generated images go to `images/<session id>/image_<id>` under it
  ([`packages/core/src/tool/plugin/image-generate.ts:269`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/tool/plugin/image-generate.ts#L269)),
  and files the desktop or web composer attaches to `uploads/<uuid>/<file name>`
  ([`packages/app/src/composer/attachments/destination.ts:33`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/app/src/composer/attachments/destination.ts#L33)).
  It is `/tmp/opencode` on Linux, under `/var/folders` on macOS, both outside every home, and
  `AppData/Local/Temp/opencode` under the home on Windows, which the catalog line
  `opencode|AppData/Local/Temp/opencode` collects.
- `~/.local/share/opencode/worktree/<first six characters of the project id>/<name>`, the git
  worktrees OpenCode makes for a project
  ([`packages/core/src/worktree/strategies.ts:51`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/worktree/strategies.ts#L51));
  [Where the project path is recorded](#where-the-project-path-is-recorded) names the table that
  lists them.

#### Data directory

`~/.local/share/opencode`:

- `opencode.db` with `opencode.db-wal` and `opencode.db-shm`, or `opencode-<channel>.db` for a
  build of another channel: every session, project, standing permission grant, schedule and stored
  credential, in one SQLite database ([Transcript stores](#transcript-stores)).
- `opencode-next.db`, the database of the pre-release 2.0 builds, opened read-only, imported into
  `opencode.db` and left in place
  ([`packages/core/src/database/v1-migration.bun.ts:799-812`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/database/v1-migration.bun.ts#L799-L812)).
- `auth.json`, the 1.x credential file, read once by a migration into the `credential` table and
  left in place
  ([`packages/core/src/database/migration/20260805200742_import_legacy_credentials.ts:39`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/database/migration/20260805200742_import_legacy_credentials.ts#L39));
  `mcp-auth.json`, the 1.x MCP token file, which 2.2.5 neither reads nor writes
  ([Credentials](#credentials), [Format versions](#format-versions)).
- `snapshot/<project id>/<hash of the worktree path>`, a bare git directory of file checkpoints
  ([`packages/core/src/snapshot.ts:96`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/snapshot.ts#L96)).
- `worktree/`, the scratch worktrees of [Scratch directories](#scratch-directories).
- `repos/`, clones of repositories a config references for context
  ([`packages/core/src/repository-cache.ts:129`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/repository-cache.ts#L129),
  [`global.ts:22`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/util/src/global.ts#L22)).
- `tool-output/tool_<id>`, the full text of a tool result the transcript cut at 2000 lines or
  50 KiB, and binary results the model cannot take inline, deleted after seven days
  ([`packages/core/src/tool-output.ts:13-18`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/tool-output.ts#L13-L18),
  [`:88-92`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/tool-output.ts#L88-L92),
  [`:122-124`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/tool-output.ts#L122-L124)).
- `shell/<project id>/sh_<id>.out`, the combined output of each command the shell tool ran,
  deleted after seven days and beyond the 25 most recent exited commands of a running server
  ([`packages/core/src/shell.ts:32-34`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/shell.ts#L32-L34),
  [`:144`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/shell.ts#L144),
  [`:289`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/shell.ts#L289),
  [`:104-114`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/shell.ts#L104-L114)).
- `artifacts/<id>/`, the files of an HTML or static artifact an agent published with the
  experimental artifacts feature and the built-in local provider
  ([`packages/core/src/artifact.ts:213-233`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/artifact.ts#L213-L233)).
- `log/opencode.log`, or `log/opencode-<channel>.log` for a local build, the CLI and server log,
  appended and compacted in place to its last 25 MB once it passes 50 MB
  ([`packages/util/src/observability/logging.ts:9-11`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/util/src/observability/logging.ts#L9-L11),
  [`:59-62`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/util/src/observability/logging.ts#L59-L62));
  `log/direct/<stamp>-<pid>.jsonl` and `log/direct/latest.json`, run traces written only when
  `OPENCODE_DIRECT_TRACE` is set
  ([`packages/cli/src/mini-host.ts:50-60`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/cli/src/mini-host.ts#L50-L60)).

#### Config directories

`~/.config/opencode`:

- `opencode.json` and `opencode.jsonc`, the global settings
  ([`packages/core/src/config/discovery.ts:12`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/config/discovery.ts#L12),
  [`:116-120`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/config/discovery.ts#L116-L120)), which can hold provider
  keys, MCP server `environment` values and remote MCP server `headers`
  ([Credentials](#credentials)).
- `cli.json`, the TUI's settings (theme, keybindings, display choices)
  ([`packages/cli/src/config/config.ts:31`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/cli/src/config/config.ts#L31)), written
  once from the 1.x `tui.json` here and `kv.json` in the state directory, both left in place
  ([`packages/cli/src/config/migrate.ts:100-112`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/cli/src/config/migrate.ts#L100-L112)).
- `service.json` (or `service-<channel>.json`), the background server's settings
  ([Credentials](#credentials)).
- `AGENTS.md`, global instructions
  ([`packages/core/src/config/plugin/instruction.ts:41`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/config/plugin/instruction.ts#L41)).
- `agent/` or `agents/` and `mode/` or `modes/` (agent definitions,
  [`packages/core/src/config/plugin/agent.ts:22-23`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/config/plugin/agent.ts#L22-L23)),
  `command/` or `commands/`
  ([`packages/core/src/config/plugin/command.ts:153`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/config/plugin/command.ts#L153)),
  `skill/` or `skills/`
  ([`packages/core/src/config/plugin/skill.ts:89-90`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/config/plugin/skill.ts#L89-L90)),
  `plugins/`
  ([`packages/tui/src/plugin/discovery.ts:21`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/tui/src/plugin/discovery.ts#L21)) and
  `themes/`
  ([`packages/tui/src/theme/discovery.ts:7`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/tui/src/theme/discovery.ts#L7)).

`~/.opencode`: the `.opencode` directory of the home, found by the same walk up from the working
directory that finds a project's
([`discovery.ts:78-82`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/config/discovery.ts#L78-L82)), so it is read like a
project `.opencode` directory ([Project-local files](#project-local-files)).
`~/.opencode/bin/opencode` is the curl installer's target
([`install:69`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/install#L69)), and the desktop app installs its bundled CLI there too
([`packages/desktop/src/main/service/desktop-cli.ts:43-46`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/desktop/src/main/service/desktop-cli.ts#L43-L46)).

The configuration discovery also reads `~/.claude` and `~/.agents` as global roots and `.claude`
and `.agents` directories up from the working directory
([`discovery.ts:69-73`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/config/discovery.ts#L69-L73),
[`:82`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/config/discovery.ts#L82)); those directories belong to other agents
and are not this agent's storage.

#### State directory

`~/.local/state/opencode`:

- `prompt-history.jsonl`, the last 50 TUI prompts; `prompt-stash.jsonl`, up to 50 prompts set aside
  unsent; `frecency.jsonl`, up to 1000 files attached to prompts; `session.json`, the pinned
  session ids. Their records are under [TUI state files](#tui-state-files).
- `model.json`, the TUI's recent and favourite models and the variant chosen per model
  ([`packages/tui/src/context/local.tsx:157`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/tui/src/context/local.tsx#L157),
  [`packages/tui/src/model-preference.ts:12-16`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/tui/src/model-preference.ts#L12-L16)).
- `kv.json`, the 1.x TUI settings, read once into `cli.json` and left in place
  ([`migrate.ts:102`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/cli/src/config/migrate.ts#L102)).
- `service.json` (or `service-<channel>.json`), the background server's registration, which every
  client reads to find and authenticate to the server
  ([`packages/cli/src/services/service-config.ts:116-131`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/cli/src/services/service-config.ts#L116-L131),
  [`packages/client/src/effect/service.ts:14-20`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/client/src/effect/service.ts#L14-L20);
  [Credentials](#credentials)).
- `pty/<instance id>/service.json` and `service.lock`, the registration of the persistent terminal
  daemon `opencode-pty` that keeps desktop terminals alive across server restarts
  ([`packages/core/src/persistent-pty/index.ts:152`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/persistent-pty/index.ts#L152),
  [`packages/core/src/persistent-pty/daemon.ts:206-208`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/persistent-pty/daemon.ts#L206-L208),
  [`:449-451`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/persistent-pty/daemon.ts#L449-L451)).
- `locks/`, file locks
  ([`packages/util/src/flock.ts:20`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/util/src/flock.ts#L20)).

#### Cache directory

`~/.cache/opencode`: `bin/`, downloaded tools such as ripgrep and the `opencode-pty` daemon
([`packages/core/src/ripgrep/binary.ts:124`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/ripgrep/binary.ts#L124),
[`packages/core/src/persistent-pty/binary.bun.ts:19-24`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/persistent-pty/binary.bun.ts#L19-L24));
`npm/`, plugin and provider packages installed from npm
([`packages/util/src/npm.ts:139`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/util/src/npm.ts#L139)); `skills/`, skills fetched
from URLs ([`packages/core/src/skill/discovery.ts:106`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/skill/discovery.ts#L106)).

#### OpenCode desktop app

The desktop app's Electron `userData` directory (Electron's per-app data directory) is
`<appData>/<app id>`
([`packages/desktop/src/main/lifecycle/configure.ts:47`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/desktop/src/main/lifecycle/configure.ts#L47)),
the id `ai.opencode.desktop`, `ai.opencode.desktop.beta` or `ai.opencode.desktop.dev` by channel
([`packages/desktop/src/main/constants.ts:17-26`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/desktop/src/main/constants.ts#L17-L26)):
`~/Library/Application Support/ai.opencode.desktop` on macOS, `~/.config/ai.opencode.desktop` on
Linux and `AppData/Roaming/ai.opencode.desktop` on Windows, as the
[desktop reset steps of the troubleshooting page](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/web/src/content/docs/troubleshooting.mdx#L164-L181)
also give. It holds:

- `drafts.sqlite`, the app's own database: prompt drafts and prompt history (`document`), images and
  text chunks they reference (`blob`), everything the renderer persists (`state`) and GUI plugin
  switches (`plugin`)
  ([`packages/desktop/src/main/storage/index.ts:24`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/desktop/src/main/storage/index.ts#L24),
  [`packages/desktop/src/main/storage/schema.ts:3-40`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/desktop/src/main/storage/schema.ts#L3-L40)).
- `default.dat` and `opencode.*.dat`, the older per-namespace store files, copied into
  `drafts.sqlite` once and removed
  ([`packages/desktop/src/main/storage/legacy.ts:15-24`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/desktop/src/main/storage/legacy.ts#L15-L24),
  [`:91-93`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/desktop/src/main/storage/legacy.ts#L91-L93)).
- `cli/<stage>/opencode`, copies of the bundled CLI the app runs
  ([`packages/desktop/src/main/service/desktop-cli.ts:142-144`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/desktop/src/main/service/desktop-cli.ts#L142-L144)).
- `logs/<stamp>/`, one directory per run, with a `network.netlog`, kept seven days, and
  `Crashpad/`, crash dumps
  ([`packages/desktop/src/main/native/logging.ts:10`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/desktop/src/main/native/logging.ts#L10),
  [`:122`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/desktop/src/main/native/logging.ts#L122),
  [`:134`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/desktop/src/main/native/logging.ts#L134),
  [`:184-185`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/desktop/src/main/native/logging.ts#L184-L185)).
- `compile-cache/`, the V8 compile cache
  ([`configure.ts:56`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/desktop/src/main/lifecycle/configure.ts#L56)).

A packaged app keeps no sessions of its own: it starts or adopts the shared background server with
`serve --service`, through the registration in `~/.local/state/opencode`, so its sessions land in
`~/.local/share/opencode/opencode.db` beside the TUI's
([`packages/desktop/src/main/service/background-service.ts:42-46`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/desktop/src/main/service/background-service.ts#L42-L46),
[`:58-72`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/desktop/src/main/service/background-service.ts#L58-L72)); only an
unpackaged development build with `OPENCODE_DESKTOP_ISOLATED_SERVER` moves the registration into
`userData`.

#### System-wide managed config

Outside the home: `/Library/Application Support/opencode` on macOS, `%ProgramData%\opencode` on
Windows and `/etc/opencode` elsewhere, and on macOS the managed preferences
`/Library/Managed Preferences/<user>/ai.opencode.managed.plist` or
`/Library/Managed Preferences/ai.opencode.managed.plist` a device management profile deploys
([`discovery.ts:27-40`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/config/discovery.ts#L27-L40)).

### Credentials

No keychain is used; every credential is in a file or the database.

- `.local/share/opencode/opencode.db` (with its `-wal` and `-shm` files, `opencode-<channel>.db`
  and `opencode-next.db`) stores provider keys and OAuth tokens, MCP server OAuth tokens and the
  OpenCode console account tokens beside the transcripts; it is a store that mixes secrets with
  evidence, and [Secrets](#secrets) names the fields
  ([`packages/core/src/credential/sql.ts:5-14`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/credential/sql.ts#L5-L14),
  [`packages/core/src/mcp/oauth.ts:274-316`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/mcp/oauth.ts#L274-L316)).
  The database is created mode 0600 and its sidecars are tightened to 0600
  ([`packages/core/src/database/database.ts:85-98`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/database/database.ts#L85-L98)).
- `.local/share/opencode/auth.json`, the 1.x provider credential file, holding per provider
  `{type: "oauth", refresh, access, expires, accountId?, enterpriseUrl?}`,
  `{type: "api", key, metadata?}` or `{type: "wellknown", key, token}`
  ([`20260805200742_import_legacy_credentials.ts:11-29`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/database/migration/20260805200742_import_legacy_credentials.ts#L11-L29)),
  stays on disk after 2.x imports it.
- `.local/share/opencode/mcp-auth.json`, the 1.x MCP OAuth token file, which the vendor's
  [MCP servers page](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/web/src/content/docs/mcp-servers.mdx#L242) still names, stays on
  disk; 2.2.5 keeps MCP tokens in the database instead.
- `.config/opencode/opencode.json`, `.config/opencode/opencode.jsonc`, `.opencode/opencode.json`
  and `.opencode/opencode.jsonc` are credential files: a provider's settings pass every key other
  than the named ones to the provider package, so they can hold `apiKey` and `headers`
  ([`packages/schema/src/provider.ts:68-80`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/schema/src/provider.ts#L68-L80)), and an
  MCP server's entry can hold `environment` values or remote `headers`
  ([`packages/schema/src/mcp.ts:26-32`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/schema/src/mcp.ts#L26-L32),
  [`:53-57`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/schema/src/mcp.ts#L53-L57)). The 1.x `.config/opencode/config.json` is
  the same kind of file ([Format versions](#format-versions)).
- `.config/opencode/service.json` (or `service-<channel>.json`, and the older
  `service-<hash>.json` of a non-latest channel, copied forward and left in place) holds the
  background server's `password` (32 random bytes, base64url, kept across restarts), its
  `hostname`, `port`, `cors` and `env`, and, when remote access is on, `remote.route`, the random
  16-hex-character subdomain that is the secret half of the server's public URL; it is written mode
  0600 through a `service.json.<pid>.tmp` file renamed over it
  ([`service-config.ts:15-28`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/cli/src/services/service-config.ts#L15-L28),
  [`:41-44`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/cli/src/services/service-config.ts#L41-L44),
  [`:62-65`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/cli/src/services/service-config.ts#L62-L65),
  [`:94-100`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/cli/src/services/service-config.ts#L94-L100),
  [`:170-192`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/cli/src/services/service-config.ts#L170-L192)).
  Both are flagged by the secret globs `.config/opencode/service*.json` and
  `.local/state/opencode/service*.json`.
- `.local/state/opencode/service.json` (or `service-<channel>.json`), the registration, carries the
  same `password`, written mode 0600
  ([`packages/client/src/effect/service.ts:14-20`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/client/src/effect/service.ts#L14-L20),
  [`:142-148`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/client/src/effect/service.ts#L142-L148);
  [`service-config.ts:91`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/cli/src/services/service-config.ts#L91)).
- `.local/state/opencode/pty/<instance id>/service.json` holds the `token` that authorises a client
  to the persistent terminal daemon's socket
  ([`packages/core/src/persistent-pty/daemon.ts:35-42`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/persistent-pty/daemon.ts#L35-L42)).
- Remote access exposes the server through an OpenTunnel tunnel (a third-party tunnelling client
  library, `@opentunnel/client`) at `https://<route>.<tunnel hostname>`, turned on with
  `opencode service set remote true` or `opencode pair --remote`; the tunnel identity is kept by
  that library's own XDG storage
  ([`packages/cli/src/services/remote-tunnel.ts:29`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/cli/src/services/remote-tunnel.ts#L29),
  [`:68-72`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/cli/src/services/remote-tunnel.ts#L68-L72)), at a path this repository
  does not name.
- `.config/opencode/cli.json`, `.local/state/opencode/kv.json` and `.local/state/opencode/model.json`
  are not credential files: they hold display settings and model choices only.

### Exclusions

Relative to the home: `.local/share/opencode/snapshot`, `.local/share/opencode/worktree`,
`.local/share/opencode/repos`, `.local/share/opencode/tool-output`, `.config/opencode/node_modules`,
`.opencode/node_modules`, `.opencode/bin`, `.cache/opencode` (binaries, package installs and skill
clones) and `.local/share/opencode/bin`, which no reviewed commit writes. The desktop app's V8
compile cache is excluded as `Library/Application Support/ai.opencode.desktop*/compile-cache`,
`AppData/Roaming/ai.opencode.desktop*/compile-cache` and
`.config/ai.opencode.desktop*/compile-cache`. The desktop app's `cli/` directory holds copies of
the CLI executable ([OpenCode desktop app](#opencode-desktop-app)), a downloaded binary of the kind
an exclusion covers. `.local/share/opencode/shell` and `.local/share/opencode/artifacts` hold what
the agent ran and published, not copies, and are not excluded.

### Project-local files

`.opencode/` directories from the working directory up to the filesystem root, read for
`opencode.json` and `opencode.jsonc` and the same `agent/`, `agents/`, `mode/`, `modes/`,
`command/`, `commands/`, `skill/`, `skills/`, `plugins/` and `themes/` subdirectories as the global
config root ([`discovery.ts:78-82`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/config/discovery.ts#L78-L82)); bare
`opencode.json` and `opencode.jsonc` files found on the same walk; and `AGENTS.md` files from the
working directory up to the home or the project root
([`instruction.ts:36-58`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/config/plugin/instruction.ts#L36-L58)). No
per-project database: sessions live in the home database.

### Where the project path is recorded

All in `opencode.db` ([Record schema](#record-schema)):

- `project.worktree`, the absolute root of each project, and `project.sandboxes`, a JSON array of
  absolute paths
  ([`packages/core/src/project/sql.ts:12-29`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/project/sql.ts#L12-L29)).
- `session_v2.directory`, the session's working directory, and `session_v2.path`, its path under
  the project ([`packages/core/src/session/sql.ts:41-42`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/session/sql.ts#L41-L42));
  paths are stored with `/` on Windows
  ([`packages/core/src/database/path.ts:6-9`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/database/path.ts#L6-L9)).
- `worktree.directory`, one row per worktree OpenCode made, and the deprecated
  `project_directory.directory`
  ([`packages/core/src/worktree/sql.ts:7-21`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/worktree/sql.ts#L7-L21),
  [`project/sql.ts:31-47`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/project/sql.ts#L31-L47)).
- `permission.project_id`, which ties a standing grant to a project.
- In a database upgraded from 1.x, the legacy `session.directory`.

Outside SQLite, `~/.local/state/opencode/frecency.jsonl` lists absolute paths of files attached to
prompts ([TUI state files](#tui-state-files)): file paths whose directories point at projects, not
project roots.

## Parsing

### Transcript stores

One SQLite database for every project and every client (TUI, CLI, desktop app):
`~/.local/share/opencode/opencode.db`
([`packages/cli/src/database-path.ts:4-13`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/cli/src/database-path.ts#L4-L13)). The name
is `opencode.db` for the `latest`, `dev`, `beta`, `next` and `prod` channels or when
`OPENCODE_DISABLE_CHANNEL_DB` is `1` or `true`, otherwise `opencode-<channel>.db`; `OPENCODE_DB`
replaces it, a relative value resolving under the data directory. `journal_mode` is WAL and a
passive checkpoint runs only when the database is opened
([`packages/core/src/database/database.ts:39-44`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/database/database.ts#L39-L44)),
so rows written since then may live only in `opencode.db-wal`. No encryption or compression.

A new database is created from the current schema
([`packages/core/src/database/migration.ts:21-47`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/database/migration.ts#L21-L47),
[`schema.gen.ts`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/database/schema.gen.ts)); one upgraded from 1.x keeps its
1.x `session`, `message` and `part` tables beside the 2.x ones ([Format versions](#format-versions)).
A session is one `session_v2` row and its messages are `session_message` rows ordered by `seq`. An
assistant message's content items are inline in its `data`; a message whose items are in
`session_part` rows keeps an empty `content` array, and readers take the items from those rows
([`packages/core/src/session/projector.ts:317-332`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/session/projector.ts#L317-L332),
[`packages/core/src/session/history.ts:103-110`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/session/history.ts#L103-L110)).
At 2.2.5 no code inserts the first `session_part` row of a message: the projector only updates
part rows a newer release made.

#### What survives a delete

Deleting a session (`opencode session delete`, or the clients' delete) interrupts it, cancels its
background jobs, deletes its children first, publishes the durable `session.deleted` event and
removes the session's `event` and `event_sequence` rows
([`packages/core/src/session.ts:391-401`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/session.ts#L391-L401),
[`packages/core/src/bus.ts:706-712`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/bus.ts#L706-L712)). The projector
releases the session's `blob_ref` rows and any `blob` no other session references, then deletes the
`session_v2` row
([`projector.ts:646-651`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/session/projector.ts#L646-L651),
[`packages/core/src/session/blob.ts:480-482`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/session/blob.ts#L480-L482)),
which cascades to `session_message`, `session_part`, `session_inbox`, `instruction_entry`,
`instruction_state` and `schedule` ([`session/sql.ts:86-215`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/session/sql.ts#L86-L215),
[`packages/core/src/schedule/sql.ts:15-17`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/schedule/sql.ts#L15-L17)). No
record of the deletion is left. Committing a revert deletes the session's messages from the revert
point on ([`projector.ts:877-913`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/session/projector.ts#L877-L913)).
Archiving sets `time_archived` and unarchiving clears it
([`projector.ts:712-727`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/session/projector.ts#L712-L727)).

What remains after a delete: the legacy `session`, `message` and `part` rows of a database
upgraded from 1.x, which nothing deletes; the session in `opencode-next.db`, which is only read;
`prompt-history.jsonl` (the last 50 TUI prompts, with no session, project or time); unsent prompts
in `prompt-stash.jsonl`; a pinned session id in `session.json`; full tool outputs under
`tool-output/` and command output under `shell/`, each for seven days; the desktop app's prompt
history in `drafts.sqlite`; and log lines naming the session in `log/opencode.log`. An archived
session keeps every row. The analyzer reads the legacy tables (for a session `session_message`
does not hold), `opencode-next.db`, `prompt-history.jsonl` and `session.json` as secondary sources
and `prompt-stash.jsonl` as a primary record of unsent prompts; it does not read the log,
`shell/` or `drafts.sqlite`, and `tool-output/` is excluded from collection
([Secondary sources](../analyzer/docs/output.md#secondary-sources)).

### Record schema

Times are epoch milliseconds
([`packages/core/src/database/schema.sql.ts`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/database/schema.sql.ts)). No git
branch is recorded; snapshots hold commit hashes. JSON columns are text holding a JSON value.

#### Sessions

`session_v2` ([`session/sql.ts:28-84`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/session/sql.ts#L28-L84)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `id` | string | | |
| `project_id` | string | | |
| `workspace_id` | string or null | | |
| `parent_id` | string or null | | |
| `fork_session_id` | string or null | | |
| `fork_boundary` | string or null | | |
| `slug` | string | | |
| `directory` | string | | |
| `path` | string or null | | |
| `title` | string or null | | |
| `version` | string | | |
| `share_url` | string or null | | |
| `summary_additions` | number or null | | |
| `summary_deletions` | number or null | | |
| `summary_files` | number or null | | |
| `summary_diffs` | string or null | | |
| `metadata` | string or null | | |
| `cost` | number | | |
| `tokens_input` | number | | |
| `tokens_output` | number | | |
| `tokens_reasoning` | number | | |
| `tokens_cache_read` | number | | |
| `tokens_cache_write` | number | | |
| `revert` | string or null | | |
| `permission` | string or null | | |
| `agent` | string or null | | |
| `model` | string or null | | |
| `time_created` | number | | |
| `time_updated` | number | | |
| `time_busy` | number or null | | |
| `time_idle` | number or null | | |
| `time_viewed` | number or null | | |
| `idle_outcome` | string or null | `succeeded`, `failed`, `interrupted` | |
| `time_compacting` | number or null | | |
| `time_archived` | number or null | | |
| `time_suspended` | number or null | | |
| `resume_attempts` | number | | |

`id` starts `ses_`. `model` is a JSON model reference `{id, providerID, variant?}`; `permission` a
JSON rule list ([Permission records](#permission-records)); `fork_session_id` and `fork_boundary`
name the session and message a fork copied from
([`projector.ts:153-179`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/session/projector.ts#L153-L179)).

#### Session messages

`session_message` ([`session/sql.ts:86-105`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/session/sql.ts#L86-L105)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `id` | string | | |
| `session_id` | string | | |
| `type` | string | `agent-switched`, `model-switched`, `location-switched`, `user`, `synthetic`, `system`, `skill`, `shell`, `assistant`, `compaction`, `idle` | |
| `seq` | number | | |
| `time_created` | number | | |
| `time_updated` | number | | |
| `data` | string | | |

`id` starts `msg_`; `type` is the record's type and `data` the JSON record less `id` and `type`
([`projector.ts:481-504`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/session/projector.ts#L481-L504);
[`packages/schema/src/session-message.ts:304-329`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/schema/src/session-message.ts#L304-L329)).
The decoded `data` ([`session-message.ts:31-302`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/schema/src/session-message.ts#L31-L302)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `time` | object | | |
| `metadata` | object | | |
| `agent` | string | | |
| `previous` | string or object | | |
| `model` | object | | |
| `location` | object | | |
| `projectID` | string | | |
| `subpath` | string | | |
| `text` | string | | |
| `files` | array | | |
| `agents` | array | | |
| `skills` | array | | |
| `description` | string | | |
| `skill` | string | | |
| `name` | string | | |
| `shellID` | string | | |
| `command` | string | | |
| `status` | string | `running`, `exited`, `timeout`, `killed`, `completed`, `failed` | |
| `exit` | number | | |
| `output` | object | | |
| `content` | array | | |
| `snapshot` | object | | |
| `finish` | string | | |
| `rawFinish` | string | | |
| `native` | object | | |
| `cost` | number | | |
| `tokens` | object | | |
| `error` | object | | |
| `retry` | object | | |
| `reason` | string | `auto`, `manual` | |
| `summary` | string | | |
| `outcome` | string | `succeeded`, `failed`, `interrupted` | |

Every record has `time.created` and may have `metadata`. By type:

- `agent-switched`: `agent`, `previous` (the earlier agent); `model-switched`: `model`, `previous`
  (each a model reference `{id, providerID, variant?}`,
  [`packages/schema/src/model.ts:19-23`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/schema/src/model.ts#L19-L23));
  `location-switched`: `location` (`{directory, workspaceID?}`), `projectID`, `subpath` and
  `previous` (`{location, projectID?, subpath?}`).
- `user`: `text`, `files`, `agents`, `skills` ([`session-message.ts:76-84`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/schema/src/session-message.ts#L76-L84));
  a stored file's `uri` may be `media:<sha256>`, naming a `blob` row
  ([`packages/core/src/session/stored.ts:10-44`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/session/stored.ts#L10-L44)).
- `synthetic` and `system`: `text` and `description`; `skill`: `skill`, `name`, `text`.
- `shell`: `shellID` (`sh_...`), `command`, `status` (`running`, `exited`, `timeout`, `killed`),
  `exit`, `output` (`{output, cursor, size, truncated}`) and `time.completed`
  ([`session-message.ts:120-133`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/schema/src/session-message.ts#L120-L133);
  [`packages/schema/src/shell.ts:25`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/schema/src/shell.ts#L25),
  [`:77-84`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/schema/src/shell.ts#L77-L84)).
- `assistant`: `agent`, `model`, `content`, `snapshot` (`{start?, end?, files?}`), `finish`,
  `rawFinish`, `native` (provider replay data), `cost`, `tokens`, `error`, `retry`
  (`{attempt, at, error}`) and `time.{created, streamed?, completed?}`
  ([`session-message.ts:213-246`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/schema/src/session-message.ts#L213-L246)).
- `compaction`: `status` (`running`, `completed`, `failed`), `reason`, `summary`, `model`,
  `native`, `cost`, `tokens` and `error`
  ([`session-message.ts:248-289`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/schema/src/session-message.ts#L248-L289)).
- `idle`: `outcome` and `error`, marking the end of a turn
  ([`session-message.ts:291-302`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/schema/src/session-message.ts#L291-L302)).

An `error` is `{type, message, status?, response?, retryAfterMs?}`
([`packages/schema/src/session-error.ts:6-14`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/schema/src/session-error.ts#L6-L14)).

#### Assistant content items

An item of an assistant `content` array, and the `data` of a `session_part` row
([`session-message.ts:135-206`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/schema/src/session-message.ts#L135-L206)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `type` | string | `text`, `reasoning`, `tool` | |
| `id` | string | | |
| `name` | string | | |
| `text` | string | | |
| `executed` | boolean | | |
| `native` | object | | |
| `state` | object | | |
| `time` | object | | |

A `text` item has `text`; a `reasoning` item `text` and `time.{created, completed?}`; a `tool`
item `id` (the call id), `name`, `executed`, `state` and `time.{created, ran?, completed?}`. The
tool `state`:

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `status` | string | `streaming`, `running`, `completed`, `error` | |
| `input` | object or string | | |
| `metadata` | object | | |
| `content` | array | | |
| `error` | object | | |

`input` is the raw string while `streaming`, an object after; `content` items are
`{type: "text", text}` or `{type: "file", uri, mime, name?}`
([`packages/schema/src/tool.ts:67-83`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/schema/src/tool.ts#L67-L83)). The call and its
result are one item, updated in place.

`session_part` ([`session/sql.ts:107-128`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/session/sql.ts#L107-L128)), one
row per item, keyed by message and position:

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `message_id` | string | | |
| `session_id` | string | | |
| `ordinal` | number | | |
| `type` | string | `text`, `reasoning`, `tool` | |
| `call_id` | string or null | | |
| `time_created` | number | | |
| `time_updated` | number | | |
| `data` | string | | |

#### Projects and worktrees

`project` ([`project/sql.ts:12-29`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/project/sql.ts#L12-L29)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `id` | string | | |
| `worktree` | string | | |
| `vcs` | string or null | | |
| `name` | string or null | | |
| `icon_url` | string or null | | |
| `icon_url_override` | string or null | | |
| `icon_color` | string or null | | |
| `time_created` | number | | |
| `time_updated` | number | | |
| `time_initialized` | number or null | | |
| `time_active` | number | | |
| `time_archived` | number or null | | |
| `sandboxes` | string | | |
| `commands` | string or null | | |

`worktree` ([`worktree/sql.ts:7-21`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/worktree/sql.ts#L7-L21)) and the
deprecated `project_directory`, whose `type` is `main`, `root` or `git_worktree`
([`project/sql.ts:31-47`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/project/sql.ts#L31-L47)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `project_id` | string | | |
| `directory` | string | | |
| `strategy` | string or null | | |
| `type` | string or null | `main`, `root`, `git_worktree` | |
| `time_created` | number | | |

### Joins and timestamps

`session_v2.project_id` joins `project.id`; `session_message.session_id` and
`session_part.session_id` join `session_v2.id`; `session_part.message_id` joins
`session_message.id`. Messages order by `seq`, unique per session
([`session/sql.ts:99-104`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/session/sql.ts#L99-L104)), and parts by
`ordinal`. A tool call and its result are one content item; a tool item's `id` is its call id. A
child session (a subagent) names its parent in `parent_id`. Message ids are time-ordered
([`session-message.ts:21-28`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/schema/src/session-message.ts#L21-L28)). Times are epoch
milliseconds; a message's own time is `data.time.created`, also stored as `time_created`. The 1.x
joins are under [Format versions](#format-versions).

### Other stores

#### Permission records

OpenCode asks before a tool call when the rules say `ask`; the user answers `once`, `always` or
`reject` ([`packages/schema/src/permission.ts:42`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/schema/src/permission.ts#L42)). The
`permission.asked` and `permission.replied` events are not kept, so the answer itself is never
stored. What lands on disk:

- Standing grants: one `permission` row per resource pattern of an `always` reply
  ([`packages/core/src/permission.ts:332-337`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/permission.ts#L332-L337);
  [`packages/core/src/permission/sql.ts:7-20`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/permission/sql.ts#L7-L20)),
  applied as `allow` rules to every later session of the project
  ([`permission.ts:151-158`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/permission.ts#L151-L158)). Its `id` starts
  `psv_` ([`packages/schema/src/permission-saved.ts:8-13`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/schema/src/permission-saved.ts#L8-L13)).

  | Key | Type | Values | Since |
  | --- | --- | --- | --- |
  | `id` | string | | |
  | `project_id` | string | | |
  | `action` | string | | |
  | `resource` | string | | |
  | `time_created` | number | | |
  | `time_updated` | number | | |

- The session's rule list: `session_v2.permission`, a JSON array of rules, set at creation and by
  the durable `session.permissions` event
  ([`projector.ts:690-697`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/session/projector.ts#L690-L697)); a migration
  cleared the column for every session copied from 1.x
  ([`packages/core/src/database/migration/20260910120000_clear_v1_session_permission.ts:8`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/database/migration/20260910120000_clear_v1_session_permission.ts#L8)).
  A rule ([`packages/schema/src/permission.ts:56-66`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/schema/src/permission.ts#L56-L66)):

  | Key | Type | Values | Since |
  | --- | --- | --- | --- |
  | `action` | string | | |
  | `resource` | string | | |
  | `effect` | string | `allow`, `deny`, `ask` | |

- Per-call outcomes, in the transcript: a declined call's tool item gets `state.status` `error`
  with `error.type` `aborted` and the message `The user declined this tool call`
  ([`packages/core/src/session/runner/step.ts:212-220`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/session/runner/step.ts#L212-L220));
  a call a `deny` rule blocks, or one the user rejected with feedback, gets `error.type`
  `permission.rejected`
  ([`packages/core/src/session/to-session-error.ts:71-72`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/session/to-session-error.ts#L71-L72)),
  whose message is the rule's `reason` or `Permission denied: <permission>` for a rule and the
  user's text for feedback ([`permission.ts:60-79`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/permission.ts#L60-L79)),
  so a rule and feedback cannot be told apart from the record. An approval leaves no per-call
  record.

#### TUI state files

In `~/.local/state/opencode`, each rewritten at the TUI's start with the lines that still parse:

- `prompt-history.jsonl`, one prompt per line, the last 50
  ([`packages/tui/src/prompt/history.tsx:20-27`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/tui/src/prompt/history.tsx#L20-L27),
  [`:64-104`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/tui/src/prompt/history.tsx#L64-L104)); no session id, project or time.

  | Key | Type | Values | Since |
  | --- | --- | --- | --- |
  | `text` | string | | |
  | `files` | array | | |
  | `agents` | array | | |
  | `skills` | array | | |
  | `pasted` | array | | |
  | `mode` | string | `normal`, `shell` | |

- `prompt-stash.jsonl`, up to 50 prompts set aside unsent, `prompt` a history entry and
  `timestamp` the time it was stashed
  ([`packages/tui/src/prompt/stash.tsx:9-14`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/tui/src/prompt/stash.tsx#L9-L14),
  [`:40-46`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/tui/src/prompt/stash.tsx#L40-L46)); `pop` removes the last entry.

  | Key | Type | Values | Since |
  | --- | --- | --- | --- |
  | `prompt` | object | | |
  | `timestamp` | number | | |

- `session.json`, the ids of the sessions the user pinned, written through a temporary file
  ([`packages/tui/src/context/local.tsx:517-531`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/tui/src/context/local.tsx#L517-L531)).

  | Key | Type | Values | Since |
  | --- | --- | --- | --- |
  | `pinned` | array | | |

- `frecency.jsonl`, one line per file attached to a prompt, `path` resolved against the TUI's
  working directory, up to 1000 paths
  ([`packages/tui/src/prompt/frecency.tsx:8-10`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/tui/src/prompt/frecency.tsx#L8-L10),
  [`:57-61`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/tui/src/prompt/frecency.tsx#L57-L61)).

  | Key | Type | Values | Since |
  | --- | --- | --- | --- |
  | `path` | string | | |
  | `frequency` | number | | |
  | `lastOpen` | number | | |

The past-use reading of these files is under
[Past-use rows and secondary sources](#past-use-rows-and-secondary-sources).

#### Other records of use

| Source | Holds | Survives | Joins on | Plan |
| --- | --- | --- | --- | --- |
| Legacy `session`, `message`, `part` tables of a database upgraded from 1.x ([Format versions](#format-versions)) | the 1.x transcript, copied into `session_message` once | a 2.x delete, which removes only the `session_v2` row and what cascades from it | session id, message id (the copy keeps the 1.x message ids, [`v1-migration.bun.ts:352-364`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/database/v1-migration.bun.ts#L352-L364)) | secondary: read only for a session `session_message` does not hold, its event rows marked `recovered` |
| `opencode-next.db` | a pre-release 2.0 database (`project`, `session`, `session_message`), imported read-only | everything after the import | session id | secondary (`secondary()`), so a session it shares with `opencode.db` is shown once |
| `tool-output/tool_<id>` | full text of a tool result the transcript cut | a delete, for seven days | the path in the transcript marker `[showing lines 1-<n> of <m>; full output saved to <file>]` | not collected (catalog exclusion) |
| `shell/<project id>/sh_<id>.out` | combined output of each shell command | a delete, for seven days | the `shellID` of a `shell` record or the job of a background command | not read |
| desktop app `drafts.sqlite` | prompt drafts and history, `document(key, value)`, kept 30 days or the 100 most recent ([`legacy.ts:5-7`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/desktop/src/main/storage/legacy.ts#L5-L7)) | a delete | not stated | not read |

The migration from 1.x records its progress under the `kv` key `migration.v1-v2`
([`v1-migration.bun.ts:220`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/database/v1-migration.bun.ts#L220)).

#### Schedules and background jobs

`schedule` holds the prompts a user or agent scheduled with `/schedule` (opt-in through
`experimental.schedules`, [`CHANGELOG.md:312`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/CHANGELOG.md#L312)): one row per schedule, the
session it wakes, the prompt and when it fires
([`packages/core/src/schedule/sql.ts:10-45`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/schedule/sql.ts#L10-L45);
[`packages/schema/src/schedule.ts:34-70`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/schema/src/schedule.ts#L34-L70),
[`:114-127`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/schema/src/schedule.ts#L114-L127)).

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `id` | string | | |
| `session_id` | string or null | | |
| `name` | string or null | | |
| `action` | string | | |
| `check` | string or null | | |
| `when` | string | | |
| `tz` | string | | |
| `status` | string | `active`, `paused`, `done` | |
| `next_due_at` | number or null | | |
| `ends_at` | number or null | | |
| `remaining_runs` | number or null | | |
| `run_count` | number | | |
| `checks` | number | | |
| `failures` | number | | |
| `last_fired_at` | number or null | | |
| `last_message_id` | string or null | | |
| `last_outcome` | string or null | `fired`, `late`, `checked`, `expired`, `skipped`, `disabled`, `failed` | |
| `retry_at` | number or null | | |
| `state` | string or null | | |
| `time_created` | number | | |
| `time_updated` | number | | |

`action` is JSON `{type: "prompt", prompt}`; `when` is JSON with `type` `in`, `at`, `every` or
`cron` and the matching key. `job` holds the background shell commands, subagents and image
generations still running, deleted when their result is delivered or the session is deleted
([`packages/core/src/job/sql.ts:7-27`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/job/sql.ts#L7-L27),
[`packages/core/src/job.ts:490-499`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/job.ts#L490-L499));
`session_inbox` holds prompts queued while a session is busy, deleted on delivery or cancellation
([`packages/core/src/session/inbox.ts:352-389`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/session/inbox.ts#L352-L389)).

#### Media blobs

With `experimental.media_blob` on (since 2.2.0, [`CHANGELOG.md:430`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/CHANGELOG.md#L430)),
inline user files and tool `data:` files are moved into `blob` (`hash`, `mime`, `bytes`, `width`,
`height`, `data` as a SQLite BLOB, `time_created`) and the message keeps `media:<hash>`;
`blob_ref` (`session_id`, `message_id`, `hash`, `seq`) records which message uses which blob
([`session/sql.ts:185-215`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/session/sql.ts#L185-L215),
[`blob.ts:336-341`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/session/blob.ts#L336-L341)).

#### Event log

`event` (`id`, `aggregate_id`, `seq`, `created`, `type`, `data`) and `event_sequence`
([`packages/core/src/database/schema.gen.ts:53-70`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/database/schema.gen.ts#L53-L70))
keep durable event payloads only when the bus is configured to, which the hosted (workerd) server
does and the CLI's server does not
([`bus.ts:183-184`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/bus.ts#L183-L184),
[`:208`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/bus.ts#L208);
[`packages/server/src/workerd.ts:71`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/server/src/workerd.ts#L71)); the first 2.x start
on a 1.x database deletes every `event` row
([`v1-migration.bun.ts:556-575`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/database/v1-migration.bun.ts#L556-L575)).

#### Background server files

The settings file `~/.config/opencode/service.json`
([`service-config.ts:15-28`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/cli/src/services/service-config.ts#L15-L28)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `disabled` | boolean | | |
| `remote` | object | | |
| `remoteAccepted` | boolean | | |
| `hostname` | string | | |
| `port` | number | | |
| `password` | string | | |
| `cors` | array | | |
| `env` | object | | |

The registration `~/.local/state/opencode/service.json`
([`packages/client/src/effect/service.ts:142-148`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/client/src/effect/service.ts#L142-L148)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `id` | string | | |
| `version` | string | | |
| `url` | string | | |
| `pid` | number | | |
| `password` | string | | |

### Format versions

How the store changed, newest first. A host may run any of these builds; the parser reads each.

- Between 2.0.32 ([`cb07c55`](https://github.com/anomalyco/opencode/commit/cb07c555d4d1ca34430b7857b52f104c3d95e09e)) and 2.2.5: the `session_part`, `blob`, `blob_ref`,
  `schedule` and `job` tables were added and `session_pending` dropped
  ([`packages/core/src/database/migration/20261029002625_session_part.ts`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/database/migration/20261029002625_session_part.ts),
  [`20261025154125_drop_session_pending.ts`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/database/migration/20261025154125_drop_session_pending.ts));
  `session_v2` gained `time_busy`
  ([`20261029073502_session_time_busy.ts`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/database/migration/20261029073502_session_time_busy.ts));
  the `system` and `idle` message types and the `native` and `rawFinish` keys appear in the
  schema. Blobs are added in 2.2.0 and schedules in 2.2.1
  ([`CHANGELOG.md:381`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/CHANGELOG.md#L381), [`:288`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/CHANGELOG.md#L288)).
- 2.1.0 renamed provider replay data to `native`: text and reasoning items' `state`, assistant and
  compaction `providerState`, and tool results' `resultState`; rows stored earlier keep the old
  keys and are decoded on read with no migration
  ([`CHANGELOG.md:849-866`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/CHANGELOG.md#L849-L866)).
- 2.0 (the pre-release builds' `opencode-next.db`, then `opencode.db` from 2.0 releases): sessions
  live in `session_v2` and `session_message` is the only transcript for new sessions. A database
  created by 2.x has no `session`, `message`, `part`, `session_share` or `todo` table. A database
  upgraded from 1.x gets `session_v2` beside the old tables; the first 2.x start deletes every
  `event` row, then copies each 1.x session into `session_v2` and its `message` and `part` rows
  into `session_message`, leaving the old rows in place
  ([`v1-migration.bun.ts:541-575`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/database/v1-migration.bun.ts#L541-L575),
  [`:610-612`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/database/v1-migration.bun.ts#L610-L612),
  [`:674-698`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/database/v1-migration.bun.ts#L674-L698)). A pre-release
  database's `session` is renamed to `session_v2`
  ([`packages/core/src/database/migration/20260804233008_loose_psylocke.ts:28`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/database/migration/20260804233008_loose_psylocke.ts#L28)).
  The 1.x `auth.json` is imported into `credential`
  ([`20260805200742_import_legacy_credentials.ts:46-88`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/database/migration/20260805200742_import_legacy_credentials.ts#L46-L88)).
  Session sharing is gone: no code writes `session_share` or `share_url`, and the share and
  unshare keybindings read "Deprecated: session sharing was removed" at
  [`797d2af`](https://github.com/sst/opencode/blob/797d2afdbf8f0b4f189748373ff1a69bde9e0ff7/packages/tui/src/config/v1/keybind.ts#L101-L102);
  `session_v2.share_url` is copied from 1.x sessions. `opencode github` is gone from the CLI. The
  configuration file names became `opencode.json` and `opencode.jsonc` only; the 1.x
  `config.json` and the older `config` it migrated from
  ([`packages/opencode/src/config/config.ts:141-142`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/config/config.ts#L141-L142))
  are no longer read, plugins are installed under `~/.cache/opencode/npm` rather than a
  `node_modules` in each config directory
  ([`packages/opencode/src/config/config.ts:309-317`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/config/config.ts#L309-L317)),
  `CLAUDE.md` and `CONTEXT.md` are no longer read as instructions
  ([`packages/opencode/src/session/instruction.ts:65-67`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/session/instruction.ts#L65-L67)),
  and the TUI's prompt files changed shape (below).
- 1.x (at [`907b3bc`](https://github.com/sst/opencode/commit/907b3bc518fa48e90e8ec24dd327d13eee71c36c), still the layout of 1.19.5): one database, `opencode.db`, with these
  tables, whose first shapes the earliest migration at `bf544cd` still creates
  ([`packages/core/src/database/migration/20260127222353_familiar_lady_ursula.ts:9-97`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/database/migration/20260127222353_familiar_lady_ursula.ts#L9-L97)):
  `session`, `message`, `part`, `project`, `permission(project_id, data)` (dropped later,
  [`20260601202201_amazing_prowler.ts:8`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/database/migration/20260601202201_amazing_prowler.ts#L8)),
  `todo`, `session_share` and, from a later migration, `event`. Its records are below.
  `~/.local/share/opencode/mcp-auth.json` held MCP OAuth tokens
  ([`packages/opencode/src/mcp/auth.ts:37`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/mcp/auth.ts#L37)).
  `~/.local/state/opencode/kv.json` held TUI settings such as the share consent `share_consent`
  ([`packages/tui/src/context/kv.tsx:15`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/tui/src/context/kv.tsx#L15)), and
  `~/.config/opencode/tui.json` the TUI configuration
  ([`packages/opencode/src/config/tui.ts:184`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/config/tui.ts#L184)).
  `~/.local/state/opencode/plugin-meta.json` held plugin metadata
  ([`packages/opencode/src/plugin/meta.ts:49`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/plugin/meta.ts#L49)) and
  `~/.cache/opencode/models.json` the models.dev catalogue
  ([`packages/core/src/models-dev.ts:160-164`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/models-dev.ts#L160-L164)).
- Before the database, 1.x kept JSON under `~/.local/share/opencode/storage/`, one file per key:
  `project/<projectID>.json` (`{id, vcs, worktree, time}`), `session/<projectID>/<sessionID>.json`,
  `message/<sessionID>/<msgID>.json`, `part/<msgID>/<partID>.json`, `session_diff/<sessionID>.json`
  and the marker `storage/migration`
  ([`packages/opencode/src/storage/storage.ts:63-65`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/storage/storage.ts#L63-L65),
  [`:81-211`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/storage/storage.ts#L81-L211),
  [`:224-225`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/storage/storage.ts#L224-L225)); before that,
  per project, `~/.local/share/opencode/project/<slug>/storage/session/{info,message,part}/...`
  ([`storage.ts:97`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/storage/storage.ts#L97),
  [`:139`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/storage/storage.ts#L139),
  [`:150`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/storage/storage.ts#L150),
  [`:165`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/storage/storage.ts#L165)). JSON records carry `id`,
  `sessionID` and `messageID` inline, which the database `data` column strips. At `907b3bc` no
  code imports `storage/` into SQLite, `storage/` is still written for `session_diff`
  ([`packages/opencode/src/session/revert.ts:77`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/session/revert.ts#L77)),
  and `opencode import` loads exported JSON into `message` and `part`
  ([`packages/opencode/src/cli/cmd/import.ts:196-225`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/cli/cmd/import.ts#L196-L225)).

#### 1.x sessions, messages and parts

The 1.x `session` table, as the 2.x upgrade reads it
([`v1-migration.bun.ts:674-685`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/database/v1-migration.bun.ts#L674-L685)),
has the `session_v2` columns `id`, `project_id`, `workspace_id`, `parent_id`, `slug`, `directory`,
`path`, `title`, `version`, `share_url`, the `summary_*`, `metadata`, `cost`, the `tokens_*`,
`revert`, `permission`, `agent`, `model`, `time_created`, `time_updated`, `time_compacting` and
`time_archived`; its `permission` rules were `{permission, pattern, action}`. `message` and `part`
([`20260127222353_familiar_lady_ursula.ts:23-41`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/database/migration/20260127222353_familiar_lady_ursula.ts#L23-L41)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `id` | string | | |
| `message_id` | string | | |
| `session_id` | string | | |
| `time_created` | number | | |
| `time_updated` | number | | |
| `data` | string | | |

`message.id` starts `msg_` and `part.id` `prt_`; `message.data` is the 1.x message less `id` and
`sessionID`, `part.data` the part less `id`, `messageID` and `sessionID`
([`v1-migration.bun.ts:227-260`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/database/v1-migration.bun.ts#L227-L260)).
The 1.x message ([`packages/schema/src/v1/session.ts:332-356`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/schema/src/v1/session.ts#L332-L356),
[`:453-485`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/schema/src/v1/session.ts#L453-L485)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `role` | string | `user`, `assistant` | |
| `time` | object | | |
| `agent` | string | | |
| `model` | object | | |
| `format` | object | | |
| `summary` | object or boolean | | |
| `system` | string | | |
| `tools` | object | | |
| `parentID` | string | | |
| `modelID` | string | | |
| `providerID` | string | | |
| `mode` | string | | |
| `path` | object | | |
| `cost` | number | | |
| `tokens` | object | | |
| `error` | object | | |
| `structured` | object | | |
| `variant` | string | | |
| `finish` | string | | |

A user message has `model.{providerID, modelID, variant?}`; an assistant message `parentID` (its
user message), `modelID`, `providerID`, `path.{cwd, root}`, `tokens.{total?, input, output,
reasoning, cache.{read, write}}` and `error` (`{name, data}`). The 1.x part
([`v1/session.ts:87-330`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/schema/src/v1/session.ts#L87-L330),
[`:357-370`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/schema/src/v1/session.ts#L357-L370)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `type` | string | `text`, `subtask`, `reasoning`, `file`, `tool`, `step-start`, `step-finish`, `snapshot`, `patch`, `agent`, `retry`, `compaction` | |
| `text` | string | | |
| `synthetic` | boolean | | |
| `ignored` | boolean | | |
| `time` | object | | |
| `metadata` | object | | |
| `callID` | string | | |
| `tool` | string | | |
| `state` | object | | |
| `snapshot` | string | | |
| `reason` | string | | |
| `cost` | number | | |
| `tokens` | object | | |
| `prompt` | string | | |
| `description` | string | | |
| `agent` | string | | |

A `tool` part's `state.status` is `pending`, `running`, `completed` or `error`; every state has
`input`, `completed` adds `output`, `title`, `metadata` and `time.{start, end}`, and `error` adds
`error` (a string) and `time.{start, end}`
([`v1/session.ts:259-302`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/schema/src/v1/session.ts#L259-L302)); the call and its
result are one part, mutated in place. `text` and `reasoning` parts have `time.{start, end?}`. 1.x
joins: `session.project_id` to `project.id`, `message.session_id` to `session.id`,
`part.message_id` to `message.id`; parts order by `part.id` or `time_created`; an assistant message
names its user message in `parentID`.

The 1.x refusal texts in a tool part's `state.error`
([`packages/core/src/v1/permission.ts:7-25`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/v1/permission.ts#L7-L25)):
"The user rejected permission to use this specific tool call.", the same followed by " with the
following feedback: " and the user's text, or "The user has specified a rule which prevents you
from using this specific tool call." for a `deny` rule. The 1.x CLI kept `always` answers in
memory only
([`packages/opencode/src/permission/index.ts:143-150`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/permission/index.ts#L143-L150)).

`session_message` in a 1.x database was a supplementary projection with the types
`agent-switched`, `model-switched`, `user`, `synthetic`, `system`, `shell`, `assistant` and
`compaction`; a 2.0 migration emptied it with `session_input` and `event`
([`packages/core/src/database/migration/20260622170816_reset_v2_session_state.ts:8-12`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/database/migration/20260622170816_reset_v2_session_state.ts#L8-L12)).

#### 1.x share records

A 1.x session published to OpenCode's web share service had one `session_share` row
([`20260127222353_familiar_lady_ursula.ts:89-97`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/database/migration/20260127222353_familiar_lady_ursula.ts#L89-L97)),
removed with its session by `ON DELETE CASCADE`:

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `session_id` | string | | |
| `id` | string | | |
| `secret` | string | | |
| `url` | string | | |
| `time_created` | number | | |
| `time_updated` | number | | |

`id` is the share id, which the hosted service makes the last eight characters of the session id
([`packages/enterprise/src/core/share.ts:117-122`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/enterprise/src/core/share.ts#L117-L122));
`secret` is a random UUID the service returns and the client sends back with every sync and delete.
`ShareNext` (the 1.x share client module) wrote the row once the service answered, as an upsert
on a second share, and deleted it after the service accepted a delete
([`packages/opencode/src/share/share-next.ts:310-359`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/share/share-next.ts#L310-L359));
the same URL went into `session.share_url`, which `/unshare` set back to NULL
([`packages/opencode/src/share/session.ts:26-37`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/share/session.ts#L26-L37)).
Each change was also a durable `session.updated.1` event in `event`, whose `data.info.share.url`
held the URL and `data.info.time.updated` the time
([`packages/opencode/src/session/session.ts:734-746`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/session/session.ts#L734-L746);
[`packages/core/src/event.ts:336-348`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/event.ts#L336-L348)), so `event`
still held the URL after an unshare. The 1.x `event` rows have `aggregate_id` the session id and
`type` such as `message.updated.1` and `message.part.updated.1`
([`packages/schema/src/event.ts:94-96`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/schema/src/event.ts#L94-L96)).

#### Older TUI state shapes

At [`907b3bc`](https://github.com/sst/opencode/commit/907b3bc518fa48e90e8ec24dd327d13eee71c36c), a `prompt-history.jsonl` line was `{input, mode?, parts}`
([`packages/tui/src/prompt/history.tsx:8-12`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/tui/src/prompt/history.tsx#L8-L12))
and a `prompt-stash.jsonl` line `{input, parts, timestamp}`
([`packages/tui/src/prompt/stash.tsx:9-13`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/tui/src/prompt/stash.tsx#L9-L13)); at
[`cb07c55`](https://github.com/anomalyco/opencode/commit/cb07c555d4d1ca34430b7857b52f104c3d95e09e) a history entry had no `files` attachment `path`. `~/.local/state/opencode/model.json`
held the last-used model only
([`packages/opencode/src/cli/cmd/run/variant.shared.ts:19`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/cli/cmd/run/variant.shared.ts#L19)).

### Secrets

In `opencode.db` (and `opencode-<channel>.db`, `opencode-next.db`):

- `credential.value`, JSON `{type: "oauth", methodID, refresh, access, expires, metadata?}`,
  `{type: "key", key, metadata?}` or `{type: "external", methodID, metadata?}`, the last naming a
  credential managed elsewhere, such as the Azure CLI
  ([`packages/schema/src/credential.ts:31-59`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/schema/src/credential.ts#L31-L59)); a
  row's `integration_id` names the provider or MCP server. A `gitlab` row holds a GitLab OAuth
  token with the `api` scope and its `metadata.instanceUrl`, and a `github-copilot` row a GitHub
  token ([Source host rows](#source-host-rows)).

  | Key | Type | Values | Since |
  | --- | --- | --- | --- |
  | `id` | string | | |
  | `integration_id` | string or null | | |
  | `label` | string | | |
  | `value` | string | | |
  | `connector_id` | string or null | | |
  | `method_id` | string or null | | |
  | `active` | number or null | | |
  | `time_created` | number | | |
  | `time_updated` | number | | |

- `account.access_token` and `account.refresh_token`, `control_account.access_token` and
  `control_account.refresh_token`, with the account's `email` and `url`, the OpenCode console
  account ([`schema.gen.ts:16-39`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/database/schema.gen.ts#L16-L39)).
- `session_share.secret` in a 1.x database, the per-share secret that authorises sync to and
  deletion of the public copy ([1.x share records](#1x-share-records)).
- Tool `input` and output text, `shell` output and `user` text can hold pasted tokens.

### Parser plan

Globs: `.local/share/opencode/opencode*.db{,-wal,-shm}`,
`.local/share/opencode/storage/{session,message,part,project}/**/*.json`,
`.local/share/opencode/project/*/storage/session/**/*.json`, and in `.local/state/opencode`
`prompt-history.jsonl`, `prompt-stash.jsonl` and `session.json`.

Rows: in a database with `session_v2`, one per `session_message` record and assistant content
item; in a 1.x database, one per `part`, plus a `user` row per user message.

| Column | Source |
| --- | --- |
| timestamp_utc | 2.x `data.time.created`; 1.x part `data.time.start`, else `part.time_created` |
| session_id | `session_v2.id`, else `session.id` |
| project_path | `project.worktree`, fallback the session's `directory` |
| git_branch | empty |
| turn_type | `user` → user; `text` → assistant; `reasoning` → thinking; `tool` → tool_use, plus tool_result once completed or failed; `shell` → tool_use and tool_result; 1.x `step-*` → system |
| model | 2.x `model.providerID/id`; 1.x `data.providerID/modelID` |
| tool_name, tool_use_id | 2.x tool item `name`, `id`; 1.x `data.tool`, `data.callID` |
| text | text, input JSON, output or error |

Session headers and `directory` come from `session_v2`, else `session`. The session-start row adds
`fork=<fork_session_id>`, and a session a 2.x database holds only in the 1.x `session` table ends
`(only in the 1.x session table)`. In a database with `session_v2`, the 1.x `message` and `part`
rows are read only for a session `session_message` does not hold. Planned: read an assistant
message's items from `session_part` when its `content` is empty and part rows exist, and the
`system` record's `text`. Not read: `blob`, `blob_ref`, `schedule`, `job`, `session_inbox`,
`instruction_entry`, `instruction_state`, `instruction_blob`, `kv`, `event_sequence`, `account`,
`control_account`, `credential`, `todo`, `frecency.jsonl`, `model.json`, `cli.json`,
`drafts.sqlite`, `shell/` and `log/opencode.log`.

The 2.x record rows: a `shell` record is a `tool_use` row and a `tool_result` row starting
`[timeout]`, `[killed]` or `[exit <code>]` when it did not exit 0; `location-switched` is a
`system` row whose `project_path` stays the session's `directory`; `skill` and `idle` are `system`
rows; a compaction row adds its `status` and error; a tool error shows its `error.type` before the
message. The texts:

```text
location switched: <previous directory> -> <directory> workspace=<id> subpath=<path>
skill <name>: <text>
idle: <outcome>
[error] aborted The user declined this tool call
```

`workspace=` and `subpath=` appear only when set.

#### Permission rows

The message's `agent` (1.x user and assistant, 2.x assistant, 2.x `agent-switched` `agent`) is
tracked as a mode, because an agent (`build`, `plan` or a custom one) carries its own permission
rules ([`packages/schema/src/config/agent.ts:25`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/schema/src/config/agent.ts#L25)),
with these rows at the message's time:

```text
permission mode: agent=<name>
permission mode: agent=<old> -> agent=<new>
```

Per call, after the `tool_result` row of a refused call and with its `tool_name`, `tool_use_id` and
time ([Permission records](#permission-records)):

```text
permission decision: rejected
permission decision: rejected with feedback
permission decision: denied by rule
permission decision: rejected (permission.rejected)
```

`rejected` stands for the 1.x "The user rejected permission to use this specific tool call." and
the 2.x `error.type` `aborted` with "The user declined this tool call"; `rejected with feedback`
for the 1.x text followed by " with the following feedback: "; `denied by rule` for the 1.x "The
user has specified a rule which prevents you from using this specific tool call."; and `rejected
(permission.rejected)` for the 2.x `error.type` `permission.rejected`, which covers a rule and
feedback alike. No approval is recorded. Standing state: one row per `permission` table row at its
`time_created`, `project_path` the project's `worktree` and no session, and one row per rule of a
session's `permission` column (2.x `{action, resource, effect}`, 1.x `{permission, pattern,
action}`) at the session's `time_updated`, a value that is not a JSON array being a `parser:` row:

```text
permission setting: <action> <resource>=allow (from <database> permission, time_created)
permission setting: session <permission> <pattern>=<effect> (from <database> <table>.permission, time_updated)
```

The 1.x table shape `permission(project_id, data)` is skipped, since its `data` is not described.
The project's `sandboxes` array is not read
([issue 98](https://github.com/seanthegeek/doubleagent/issues/98)).

#### Model rows

A `system` row precedes the first message of a session that names a model (1.x user
`model.{providerID, modelID}`, 1.x assistant `providerID` and `modelID`, 2.x assistant
`model.{id, providerID}`), and a change row precedes every later message, or stands for every 2.x
`model-switched` record, whose model differs:

```text
model: <modelID> provider=<providerID>
model: <old> -> <new> provider=<providerID>
```

Planned: add the `variant` of a model reference beside `provider=` when present. No alias beside
a resolved id exists.

#### Past-use rows and secondary sources

The secondary sources of [Other records of use](#other-records-of-use) and
[TUI state files](#tui-state-files): the legacy 1.x tables of an upgraded database (secondary to
`session_message`, a session they alone hold read with its event rows marked `recovered`, since
both stores are one file), `opencode-next.db` (`secondary()`, deduplicated against the database it
was imported into), `prompt-history.jsonl` and `session.json`; `has_secondary` is set.
`prompt-history.jsonl` is returned by `secondary()`: each line is a `user` row with an empty
`timestamp_utc`, `session_id` and `project_path`, or for `mode` `shell` a `tool_use` row with tool
`shell` and no id, which pairs on text with the transcript's command; the prompt is `text`, or
`input` in a 1.x line. `prompt-stash.jsonl`, a primary record of unsent prompts, gives one `system`
row per line at its `timestamp`, from `prompt.text` or the 1.x `input`. `session.json`, returned by
`secondary()`, gives one `system` row per pinned id with that `session_id` at the file's
modification time, so a pinned session with no other row is a trace of a deleted one:

```text
prompt stashed (unsent): <text> (from .local/state/opencode/prompt-stash.jsonl, timestamp)
session pinned: <id> (from .local/state/opencode/session.json pinned, file mtime)
```

#### Sync, relay and origin rows

Sync: 2.x uploads no session to the vendor; only 1.x shared one, with `/share` in the TUI (which
asked once and kept the answer as `share_consent`), `opencode run --share`, or for every new
top-level session when the config set `share: "auto"`, the deprecated `autoshare: true` or
`OPENCODE_AUTO_SHARE`
([`packages/opencode/src/share/session.ts:39-46`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/share/session.ts#L39-L46),
[`packages/opencode/src/cli/cmd/run.ts:161-164`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/cli/cmd/run.ts#L161-L164)).
A shared 1.x session posted every session, message, part, diff and model update with the share
secret to `<base>/api/share/<id>/sync`, the base `https://opncd.ai`, the config's `enterprise.url`
or a signed-in console account's URL
([`packages/opencode/src/share/share-next.ts:179-222`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/share/share-next.ts#L179-L222)).
The parser reads the 1.x share records of a database, never `secret`: one row per `session_share`
row at its `time_created`; for a session with no `session_share` row whose `session.updated.1`
events carry `info.share.url`, a synced row at that event's `info.time.updated` and an unshared
row at the first later event without `share`. The database name in the text is the file's own
(`opencode-<channel>.db` for a channel database):

```text
session synced: share <id> url <url> (from opencode.db session_share, time_created)
session synced: share url <url> (from opencode.db event session.updated.1, info.time.updated)
session unshared (from opencode.db event session.updated.1, info.time.updated)
```

These rows are fixture only. A 2.x database keeps no event payloads
([Event log](#event-log)), so the event-based rows appear only for a 1.x database not yet opened by
2.x. OpenCode Zen, OpenCode's hosted model gateway, is a model provider, so its calls are model API
calls and not sync. Relay: none; no messaging integration exists in `packages/core`,
`packages/cli`, `packages/server` or `packages/tui`. Origin: none; a session imported with
`opencode session import <file or URL>` is written as a new session with the local `version` and
the import time as `time_updated`, and no field records where it came from
([`packages/cli/src/commands/handlers/session/import.ts:16-21`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/cli/src/commands/handlers/session/import.ts#L16-L21);
[`packages/core/src/session/transfer.ts:113`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/session/transfer.ts#L113),
[`:156`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/session/transfer.ts#L156)).

#### Scratch directory rows

A session whose directory is under `~/.local/share/opencode/worktree/` is classed by the catalog
rule. Each row of the `worktree` table, else of the deprecated `project_directory` table with
`type` `git_worktree`, is a `system` row at its `time_created`, with the directory as
`project_path` and no session; a directory in both tables gives one row:

```text
scratch directory: <directory> (git worktree of <project worktree> strategy=<strategy>) (from <database> <table>, time_created)
```

The temp root ([Scratch directories](#scratch-directories)) gives no row.

#### Deletion rows

No row marks a deletion; what a delete leaves is under
[What survives a delete](#what-survives-a-delete). A session with `time_archived` set is a `system`
row at that time, `<table>` being `session_v2` or `session`; an unarchived session has the column
cleared and gets none:

```text
session archived (from <database> <table>, time_archived)
```

#### Bad-line rows

Each `message`, `part`, `session_message` or `event` row whose `data` is not a JSON object is its
own row carrying the stored value in full, with its row's session and project, an empty timestamp
and `source_line` the rowid; each part file of the JSON trees that does not decode is its own row,
`source_file` the part file; a message or session file that does not decode stays one whole-file
`parser:` row; each line of `prompt-history.jsonl` and `prompt-stash.jsonl` that does not decode
is its own row, read before OpenCode's next start drops it; a `session.json` that is not a JSON
object is one `parser:` row:

```text
parser: unparseable record <rowid> (<table>.data: <reason>): <data>
parser: unparseable part <partID> (<reason>): <file text>
parser: unparseable line (<reason>): <line>
```

#### Fixtures

A 2.x session as the projector writes it
([`projector.ts:541-567`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/session/projector.ts#L541-L567),
[`:481-504`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/session/projector.ts#L481-L504)), a standing grant
([`packages/core/src/permission/saved.ts`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/permission/saved.ts)) and a
worktree row, redacted, all in `opencode.db`:

```sql
INSERT INTO project(id,worktree,vcs,time_created,time_updated,time_active,sandboxes) VALUES('4f1c2a9e0d','/home/u/work/api','git',1761900000000,1761900000000,1761900000000,'[]');
INSERT INTO session_v2(id,project_id,slug,directory,title,version,agent,model,time_created,time_updated) VALUES('ses_0a1b2c3d4e5fXYZ','4f1c2a9e0d','quiet-lake','/home/u/work/api','Bump the parser','2.2.5','build','{"id":"gpt-5","providerID":"openai"}',1761900010000,1761900042000);
INSERT INTO session_message VALUES('msg_0a1b2c3d50aaXYZ','ses_0a1b2c3d4e5fXYZ','user',1,1761900010000,1761900010000,'{"time":{"created":1761900010000},"text":"run the unit tests"}');
INSERT INTO session_message VALUES('msg_0a1b2c3d51bbXYZ','ses_0a1b2c3d4e5fXYZ','assistant',2,1761900011000,1761900042000,'{"time":{"created":1761900011000,"completed":1761900042000},"agent":"build","model":{"id":"gpt-5","providerID":"openai"},"content":[{"type":"tool","id":"call_7Qx","name":"shell","state":{"status":"error","input":{"command":"npm test"},"error":{"type":"aborted","message":"The user declined this tool call"}},"time":{"created":1761900012000,"completed":1761900020000}}],"finish":"stop"}');
INSERT INTO session_message VALUES('msg_0a1b2c3d52ccXYZ','ses_0a1b2c3d4e5fXYZ','idle',3,1761900042000,1761900042000,'{"time":{"created":1761900042000},"outcome":"succeeded"}');
INSERT INTO permission VALUES('psv_0a1b2c3d53dd','4f1c2a9e0d','bash','npm run lint',1761900030000,1761900030000);
INSERT INTO worktree VALUES('4f1c2a9e0d','/home/u/.local/share/opencode/worktree/4f1c2a/calm-heron','git',1761900005000);
```

A 1.x session in the legacy tables, as the 1.x writer stored it
([`20260127222353_familiar_lady_ursula.ts:23-41`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/database/migration/20260127222353_familiar_lady_ursula.ts#L23-L41)):

```sql
INSERT INTO message VALUES('msg_09f8e7d6c5b4AAA','ses_09f8e7d6c5b3AAA',1759000001000,1759000001000,'{"role":"user","time":{"created":1759000001000},"agent":"plan","model":{"providerID":"anthropic","modelID":"claude-sonnet-4-5"}}');
INSERT INTO part VALUES('prt_09f8e7d6c5b5AAA','msg_09f8e7d6c5b4AAA','ses_09f8e7d6c5b3AAA',1759000001000,1759000001000,'{"type":"text","text":"where is the retry loop?"}');
```

TUI state lines, one each from `prompt-history.jsonl`, `prompt-stash.jsonl` and `session.json`,
as their writers serialise them (`JSON.stringify` of the entry):

```json
{"text":"!git status","files":[],"agents":[],"skills":[],"pasted":[],"mode":"shell"}
{"prompt":{"text":"later: drop the old index","files":[],"agents":[],"skills":[],"pasted":[]},"timestamp":1761900050000}
{"pinned":["ses_0a1b2c3d4e5fXYZ","ses_09f8e7d6c5b3AAA"]}
```

#### Source host rows

The agent connects no source-host account for repository access, caches no repository list and
keeps no repository-bound remote conversation; it stores source-host tokens for two model
providers. Signing in to the GitLab provider (GitLab Duo) runs an OAuth flow with the `api` scope
and stores the tokens as a `credential` row with `integration_id` `gitlab` and
`metadata.instanceUrl`
([`packages/core/src/plugin/provider/gitlab.ts:20`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/plugin/provider/gitlab.ts#L20),
[`:31`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/plugin/provider/gitlab.ts#L31),
[`:82-86`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/plugin/provider/gitlab.ts#L82-L86)), a token that can reach that
account's repositories though OpenCode uses it for inference; the GitHub Copilot provider's device
flow asks only for `read:user` and stores its token under `github-copilot`
([`packages/core/src/plugin/provider/github-copilot.ts:88`](https://github.com/anomalyco/opencode/blob/bf544cd419c5f17535c0d5af929f7ecd41188320/packages/core/src/plugin/provider/github-copilot.ts#L88)).
Neither row names the account's login. The `repos/` clones of [Data directory](#data-directory)
carry no account. The 1.x `opencode github install` installed the `opencode-agent` GitHub App on a
repository and wrote `.github/workflows/opencode.yml` into it, and `opencode github run` ran inside
GitHub Actions with an app token held in memory
([`packages/opencode/src/cli/cmd/github.handler.ts:284-337`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/cli/cmd/github.handler.ts#L284-L337),
[`:996-1038`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/cli/cmd/github.handler.ts#L996-L1038)); the one trace on a
workstation is that workflow file. Since no file records a login, the parser yields no
`source host:` row.

## Confidence

- High: the paths, table columns, record keys, enum values and timestamp units, read from source
  at `bf544cd`; the desktop app's use of the shared database, from its server start code.
- Medium: the 1.x shapes cited at `907b3bc` hold for 1.19.5, which this review did not read.
- Claims for a real-install round: what a 2.2.5 session writes to `session_message` for each record
  type, and whether any `session_part` row appears; what the desktop app's `drafts.sqlite`
  `document` and `state` rows hold; where the OpenTunnel library keeps its identity; whether a
  session's `directory` can be the temp root; what `log/opencode.log` records of a session.
- Not determined: whether older releases wrote `bin/` under the data directory; whether an
  intermediate 1.x release imported the `storage/` JSON tree into SQLite.
