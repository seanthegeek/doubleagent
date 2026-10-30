# opencode

OpenCode (catalog agent `opencode`) is an open-source coding agent with a terminal UI, a background
server and, on the 2.0 line, an Electron desktop app; its repository moved from `sst/opencode` to
`anomalyco/opencode` ([Source and evidence](#source-and-evidence)). It was researched together with
Crush, which began as an OpenCode fork but no longer shares any storage code
([`crush.md`](crush.md)); Kilo Code's current store is an OpenCode fork and shares this schema
([`kilo-code.md`](kilo-code.md)), and Kilo Code's parser subclasses this one, so the per-record
error rows and the `session_v2` reading apply to Kilo Code's database too, should Kilo Code take
OpenCode's 2.0 store. Letta Code (at
[`2a46615`](https://github.com/letta-ai/letta-code/commit/2a46615d16babb06cf5450b8e57ecb9bdbf2f436))
can drive OpenCode as an external coding agent, starting `opencode serve` in its working directory
([src/agent/subagents/index.ts:123-159](https://github.com/letta-ai/letta-code/blob/2a46615d16babb06cf5450b8e57ecb9bdbf2f436/src/agent/subagents/index.ts#L123-L159),
[src/tools/impl/opencode-server.ts:51-66](https://github.com/letta-ai/letta-code/blob/2a46615d16babb06cf5450b8e57ecb9bdbf2f436/src/tools/impl/opencode-server.ts#L51-L66)),
so a session Letta Code started lands in this store like any other OpenCode session
([Source and evidence in `letta.md`](letta.md#source-and-evidence)). The analyzer parses the 1.x and
2.0 database layouts and the terminal UI's prompt history, stash and pinned sessions, from source
and fixtures only.

## Source and evidence

[sst/opencode](https://github.com/sst/opencode), open source, MIT. Every claim is from source.
The repository has moved to [anomalyco/opencode](https://github.com/anomalyco/opencode)
(`sst/opencode` redirects to it; default branch `dev`). Most citations are at one of two commits:

- [`907b3bc`](https://github.com/sst/opencode/commit/907b3bc518fa48e90e8ec24dd327d13eee71c36c),
  the 1.x line. The newest tagged release, v1.19.3 of 2026-10-16, is 52 commits after it on a
  branch other than `dev` and still keeps the 1.x layout (its `session` table is `session`,
  beside the `message` and `part` tables of [Record schema](#record-schema)).
- [`cb07c55`](https://github.com/anomalyco/opencode/commit/cb07c555d4d1ca34430b7857b52f104c3d95e09e),
  the `dev` head, the 2.0 line, whose CLI package is version 2.0.32
  ([`packages/cli/package.json:4`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/cli/package.json#L4))
  and which npm publishes under the `dev` tag only. It is 4742 commits after `907b3bc`.

Unless a sentence says "on the 2.0 line" or cites `cb07c55`, the paths in
[Per-user storage](#per-user-storage), [Credentials](#credentials), [Exclusions](#exclusions),
[Project-local files](#project-local-files) and [Where the project path is recorded](#where-the-project-path-is-recorded), and
the record sections, describe the shipped 1.x release at `907b3bc`;
[Format versions](#format-versions) lists what the 2.0 line changes. At `907b3bc` the schema has
moved out of `packages/opencode` into `packages/core` (tables) and `packages/schema` (JSON
shapes); the 2.0 line moves the code again, to `packages/core`, `packages/cli`, `packages/tui` and
`packages/util`. Paths cited are relative to the repository root. The platform mapping was verified
against the `xdg-basedir` 5.1.0 tarball, the version pinned in
[`packages/core/package.json:127`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/package.json#L127).

### Observed

No real install.

- The 2.0 layout the parser reads since
  [issue 165](https://github.com/seanthegeek/doubleagent/issues/165) (`session_v2` and the 2.0
  `session_message` types) is not observed on any real install, source-derived (at `cb07c55`), and
  the real-install round of 2.0 is still to run.
- No real install has shown a `session_share` row or a shared session's events, so the
  `session synced:` rows are fixture only
  ([Sync, relay and origin](#sync-relay-and-origin-rows)).

### Source reviews

- 2026-10-21: anomalyco/opencode at
  [cb07c55](https://github.com/anomalyco/opencode/commit/cb07c555d4d1ca34430b7857b52f104c3d95e09e)
  (2.0.32): the parser plan's permission, past-use, scratch directory and deletion items are now
  read from source alone, the 1.x prompt history and stash shapes at
  [907b3bc](https://github.com/sst/opencode/commit/907b3bc518fa48e90e8ec24dd327d13eee71c36c) were
  added, the `service.json` files, the desktop app and the Windows temp root got catalog lines, and
  the source-host integration was reviewed ([Source host rows](#source-host-rows)).
- 2026-10-19, `dev` at
  [`cb07c55`](https://github.com/anomalyco/opencode/commit/cb07c555d4d1ca34430b7857b52f104c3d95e09e)
  (2.0.32): added the 2.0 line throughout: the TUI state files and other records of use
  ([Other stores](#other-stores)), the desktop app, the background server's `service.json`
  ([Credentials](#credentials)), the permission records, the temp root and worktrees, the 2.0
  delete and the `session_v2` layout ([Format versions](#format-versions)) that the parser has read
  since [issue 165](https://github.com/seanthegeek/doubleagent/issues/165).
- 2026-10-17, session sharing at
  [`907b3bc`](https://github.com/sst/opencode/commit/907b3bc518fa48e90e8ec24dd327d13eee71c36c) and
  the default branch at
  [`797d2af`](https://github.com/sst/opencode/commit/797d2afdbf8f0b4f189748373ff1a69bde9e0ff7):
  added [Share records](#share-records) and the
  [`session synced:` rows](#sync-relay-and-origin-rows).

## Collection

### Per-user storage

[`packages/core/src/global.ts:3`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/global.ts#L3),[`10-15`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/global.ts#L10-L15)
builds data, cache, config and state as `xdg*/opencode`.
[`xdg-basedir/index.js:7-16`](https://github.com/sindresorhus/xdg-basedir/blob/8cceade858e4da18cb971bf1844f086e9e213563/index.js#L7-L16)
has no platform branch: `XDG_DATA_HOME` or `~/.local/share`, `XDG_CONFIG_HOME` or `~/.config`,
`XDG_STATE_HOME` or `~/.local/state`, `XDG_CACHE_HOME` or `~/.cache`. So the layout is identical on
Linux, macOS and Windows (`%USERPROFILE%\.local\share\opencode` etc.). Home itself comes from
`OPENCODE_TEST_HOME` or `os.homedir()`
([`global.ts:19`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/global.ts#L19)).
On the 2.0 line the global paths are built in
[`packages/util/src/global.ts:12-27`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/util/src/global.ts#L12-L27)
and
[`packages/util/src/global-roots.ts:4-19`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/util/src/global-roots.ts#L4-L19),
which keep the same XDG directories, still with no platform branch, and add a `tmp` root
([Scratch directories](#scratch-directories)).

#### Scratch directories

OpenCode 2.0 creates `<os.tmpdir()>/opencode` at start and tells the model in its environment block
to prefer it over `/tmp`, as a directory "pre-created and approved for external access"
([`packages/util/src/global-roots.ts:17`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/util/src/global-roots.ts#L17),
[`packages/util/src/global.ts:66-72`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/util/src/global.ts#L66-L72),
[`packages/core/src/instructions/builtins.ts:43`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/core/src/instructions/builtins.ts#L43)).
It is `/tmp/opencode` on Linux, under `/var/folders` on macOS and `%TEMP%\opencode`
(`AppData\Local\Temp\opencode`) on Windows, so outside the home on Linux and macOS, where no catalog
line can reach it (the same gap as Claude Code's temp root,
[issue 163](https://github.com/seanthegeek/doubleagent/issues/163)), and under the home on Windows,
where the catalog line `opencode|AppData/Local/Temp/opencode` collects it. The
instruction is about where the model writes temporary files, so the path shows in tool inputs;
whether a session's `directory` can be it was not determined. The worktrees OpenCode makes for a
project are scratch directories inside the data directory (`worktree/` under
[Data directory](#data-directory)); how the analyzer classes a session working in one is under
[Scratch directory rows](#scratch-directory-rows).

#### Data directory

`~/.local/share/opencode` (data):

- `opencode.db` with `-wal`/`-shm`, or `opencode-<channel>.db`: every session, project, standing
  permission grant and stored credential, in one SQLite database
  ([Transcript stores](#transcript-stores), [Credentials](#credentials)).
- `opencode-next.db`, on the 2.0 line: the database of the pre-release 2.0 builds, imported into
  `opencode.db` and left in place ([Other records of use](#other-records-of-use)).
- `storage/` legacy JSON tree: `project/<id>.json`, `session/<projectID>/`, `message/<sessionID>/`,
  `part/<messageID>/`, `session_diff/`, plus a `migration` marker
  ([`packages/opencode/src/storage/storage.ts:122`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/storage/storage.ts#L122),[`143`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/storage/storage.ts#L143),[`154`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/storage/storage.ts#L154),[`169`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/storage/storage.ts#L169),[`192`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/storage/storage.ts#L192),[`224-225`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/storage/storage.ts#L224-L225)).
- `auth.json`
  ([`packages/opencode/src/auth/index.ts:10`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/auth/index.ts#L10)),
  `mcp-auth.json`
  ([`packages/opencode/src/mcp/auth.ts:37`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/mcp/auth.ts#L37)).
- `snapshot/<projectID>/<hash>` bare git dirs for file checkpoints
  ([`packages/opencode/src/snapshot/index.ts:71`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/snapshot/index.ts#L71),
  [`packages/core/src/snapshot.ts:98`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/snapshot.ts#L98)),
  on the 2.0 line still `snapshot/<projectID>/<hash of worktree>`
  ([`packages/core/src/snapshot.ts:96`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/core/src/snapshot.ts#L96)).
- `worktree/<projectID>/` git worktrees
  ([`packages/opencode/src/worktree/index.ts:208`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/worktree/index.ts#L208)),
  on the 2.0 line `worktree/<first six characters of projectID>/<name>`
  ([`packages/core/src/worktree/strategies.ts:51`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/core/src/worktree/strategies.ts#L51)).
- `repos/`
  ([`global.ts:24`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/global.ts#L24)),
  `tool-output/` truncated tool output, pruned unless `OPENCODE_DISABLE_PRUNE`
  ([`packages/opencode/src/tool/truncation-dir.ts:4`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/tool/truncation-dir.ts#L4),
  [`config.ts:596`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/config/config.ts#L596));
  on the 2.0 line `tool-output/tool_<id>` keeps the full text of a tool result the transcript cut
  ([Other records of use](#other-records-of-use)).
- `log/opencode.log` and `log/direct/<stamp>-<pid>.jsonl` run traces
  ([`packages/core/src/observability/logging.ts:49`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/observability/logging.ts#L49),
  [`packages/opencode/src/cli/cmd/run/trace.ts:32`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/cli/cmd/run/trace.ts#L32));
  on the 2.0 line `log/opencode.log`, or `opencode-<channel>.log` for a local build
  ([`packages/util/src/observability/logging.ts:59-62`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/util/src/observability/logging.ts#L59-L62)).

#### Config directories

`~/.config/opencode` (config, overridable with `OPENCODE_CONFIG_DIR`,
[`global.ts:64`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/global.ts#L64)):
`opencode.jsonc`, `opencode.json`, `config.json`, legacy `config` migrated to `config.json`
([`packages/opencode/src/config/config.ts:141-142`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/config/config.ts#L141-L142),[`272-285`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/config/config.ts#L272-L285));
`tui.json(c)`
([`config/tui.ts:184`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/config/tui.ts#L184));
`AGENTS.md`
([`session/instruction.ts:61`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/session/instruction.ts#L61));
`agents/`, `themes/`, `plugins/`, `skills/`; a `.gitignore` plus `package.json` and `node_modules`
created by the background `@opencode-ai/plugin` install in every config dir
([`config.ts:309-317`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/config/config.ts#L309-L317),[`450-460`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/config/config.ts#L450-L460)).

`~/.opencode`: a second config directory discovered with `afs.up` from home to home
([`packages/opencode/src/config/paths.ts:34-38`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/config/paths.ts#L34-L38)),
read for `opencode.json(c)` because it ends in `.opencode`
([`config.ts:439-441`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/config/config.ts#L439-L441));
also gets the `node_modules` install. `~/.opencode/bin/opencode` is the curl installer target
([`install:68`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/install#L68),
recognised at
[`installation/index.ts:175`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/installation/index.ts#L175)).

#### State directory

`~/.local/state/opencode`, collected by the catalog's `.local/state/opencode` entry:

- `plugin-meta.json`
  ([`plugin/meta.ts:49`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/plugin/meta.ts#L49)).
- `model.json`, the last-used model
  ([`cli/cmd/run/variant.shared.ts:19`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/cli/cmd/run/variant.shared.ts#L19));
  on the 2.0 line it holds `recent`, `favorite` and `variant`
  ([`packages/tui/src/model-preference.ts:12-16`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/tui/src/model-preference.ts#L12-L16),
  [`packages/tui/src/context/local.tsx:157`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/tui/src/context/local.tsx#L157)).
- File locks
  ([`global.ts:33`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/global.ts#L33)).
- The TUI's records of use, present at `907b3bc` and on the 2.0 line, whose records are under
  [Other records of use](#other-records-of-use): `prompt-history.jsonl`, the last 50 prompts;
  `prompt-stash.jsonl`, prompts set aside unsent; `frecency.jsonl`, the files attached to prompts;
  `session.json`, the pinned session ids.
- `kv.json`, TUI settings such as the share consent
  ([Sync, relay and origin rows](#sync-relay-and-origin-rows)).
- `service.json` (or `service-<channel>.json`), on the 2.0 line: the background server's
  registration, `{id, version, url, pid, password}`
  ([`packages/cli/src/services/service-config.ts:39-42`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/cli/src/services/service-config.ts#L39-L42),
  [`:115-130`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/cli/src/services/service-config.ts#L115-L130);
  [`packages/client/src/effect/service.ts:142-148`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/client/src/effect/service.ts#L142-L148));
  `~/.config/opencode/service.json` holds its settings ([Credentials](#credentials)).

#### Cache directory

`~/.cache/opencode`: `bin/` LSP and tool binaries
([`global.ts:22`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/global.ts#L22),
[`lsp/server.ts:835`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/lsp/server.ts#L835),[`964`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/lsp/server.ts#L964),[`1049`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/lsp/server.ts#L1049)),
`packages/` npm installs
([`packages/core/src/npm.ts:87`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/npm.ts#L87)),
`skills/` clones
([`skill/discovery.ts:35`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/skill/discovery.ts#L35)),
`models.json` models.dev cache
([`packages/core/src/models-dev.ts:160-164`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/models-dev.ts#L160-L164)).

#### OpenCode desktop app

The OpenCode desktop app (Electron, `packages/desktop`) keeps its own data in Electron's `userData`,
set to `<appData>/<app id>` with the id `ai.opencode.desktop`, `ai.opencode.desktop.beta` or
`ai.opencode.desktop.dev`
([`packages/desktop/src/main/constants.ts:17-26`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/desktop/src/main/constants.ts#L17-L26),
[`packages/desktop/src/main/lifecycle/configure.ts:33`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/desktop/src/main/lifecycle/configure.ts#L33)):
`~/Library/Application Support/ai.opencode.desktop` on macOS, `%APPDATA%\ai.opencode.desktop` on
Windows and `~/.config/ai.opencode.desktop` on Linux. It holds `drafts.sqlite`, whose `document`
table keeps prompt drafts and history and whose `state` table keeps what the renderer persists
([`packages/desktop/src/main/storage/index.ts:22-24`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/desktop/src/main/storage/index.ts#L22-L24),
[`packages/desktop/src/main/storage/schema.ts:3-29`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/desktop/src/main/storage/schema.ts#L3-L29)),
and a `compile-cache` directory
([`configure.ts:42`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/desktop/src/main/lifecycle/configure.ts#L42)).
The catalog collects `ai.opencode.desktop*` under each of the three app data directories, from
source alone. Whether the app's sidecar server writes the same
`~/.local/share/opencode/opencode.db` was not determined.

#### System-wide managed config

System-wide managed config, outside home: `/Library/Application Support/opencode`,
`%ProgramData%\opencode`, `/etc/opencode`
([`config/managed.ts:23-27`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/config/managed.ts#L23-L27)).

### Credentials

- `~/.local/share/opencode/auth.json`, mode 0600: per provider
  `{type:"oauth",refresh,access,expires}` or `{type:"api",key}` or `{type:"wellknown",key,token}`
  ([`auth/index.ts:14-33`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/auth/index.ts#L14-L33),[`79`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/auth/index.ts#L79)).
  `OPENCODE_AUTH_CONTENT` can replace it
  ([`:59`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/auth/index.ts#L59)).
- `~/.local/share/opencode/mcp-auth.json` MCP OAuth tokens
  ([`mcp/auth.ts:37`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/mcp/auth.ts#L37)).
- `opencode*.db*`: table `credential` with JSON `value` column
  ([`packages/core/src/credential/sql.ts:5-14`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/credential/sql.ts#L5-L14)),
  so the transcript database is also a secret store.
- `opencode.json(c)` / `config.json` in both config dirs, and a project's `opencode.json(c)` at
  its root and in `.opencode/`, are flagged: they can carry `provider.<id>.options.apiKey`
  ([`packages/opencode/src/provider/provider.ts:229`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/provider/provider.ts#L229),[`352`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/provider/provider.ts#L352)).
- No keychain use found.
- On the 2.0 line, `~/.config/opencode/service.json` (or `service-<channel>.json`), written mode
  0600, holds the background server's `password` (32 random bytes, base64url, kept across
  restarts), its `hostname`, `port`, `cors` and `env`, and, when remote access is on,
  `remote.route`, the random 16-hex-character subdomain that is the only secret in the public URL
  of the server
  ([`packages/cli/src/services/service-config.ts:14-26`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/cli/src/services/service-config.ts#L14-L26),
  [`:164-175`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/cli/src/services/service-config.ts#L164-L175),
  [`:177-186`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/cli/src/services/service-config.ts#L177-L186)).
  The registration copy `~/.local/state/opencode/service.json` carries the same `password`
  ([Per-user storage](#per-user-storage)). Both are flagged by the secret globs
  `.config/opencode/service*.json` and `.local/state/opencode/service*.json`. Remote access
  exposes the server through an OpenTunnel tunnel (a third-party tunnelling client library,
  `@opentunnel/client`) at `https://<route>.<tunnel hostname>`, turned on with
  `opencode service set remote true` or `opencode pair --remote`
  ([`packages/cli/src/services/remote-tunnel.ts:14-27`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/cli/src/services/remote-tunnel.ts#L14-L27),
  [`packages/cli/src/commands/handlers/pair.ts:21-25`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/cli/src/commands/handlers/pair.ts#L21-L25));
  the tunnel identity is kept by that library under an XDG path this repository does not name
  ([`remote-tunnel.ts:66-70`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/cli/src/services/remote-tunnel.ts#L66-L70)),
  not determined.
- `auth.json` is imported into the `credential` table by a 2.0 migration and left in place
  ([`packages/core/src/database/migration/20260805200742_import_legacy_credentials.ts:39`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/core/src/database/migration/20260805200742_import_legacy_credentials.ts#L39),
  [`:46-50`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/core/src/database/migration/20260805200742_import_legacy_credentials.ts#L46-L50)),
  so both copies stay on disk.

### Exclusions

Relative to the home:

`.local/share/opencode/snapshot`, `.local/share/opencode/worktree`, `.local/share/opencode/repos`,
`.local/share/opencode/tool-output`, `.config/opencode/node_modules`, `.opencode/node_modules`,
`.opencode/bin`, `.cache/opencode` (entire dir is binaries, package installs and skill clones;
`models.json` is public data), and `.local/share/opencode/bin`, which neither reviewed commit
writes ([Data directory](#data-directory), [Confidence](#confidence)).

The 2.0 line adds no excludable subtree under these directories; `worktree` and `snapshot` stay
where the exclusions point. The desktop app's V8 `compile-cache` directory
([OpenCode desktop app](#opencode-desktop-app)) is excluded as
`Library/Application Support/ai.opencode.desktop*/compile-cache`,
`AppData/Roaming/ai.opencode.desktop*/compile-cache` and
`.config/ai.opencode.desktop*/compile-cache`.

### Project-local files

`.opencode/` directories from cwd up to the worktree root
([`config/paths.ts:26-31`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/config/paths.ts#L26-L31))
holding `opencode.json(c)`, `agents/`, `plugins/`, `themes/`, `skills/`, and the generated
`.gitignore`, `package.json`, `node_modules`; `opencode.jsonc` / `opencode.json` found by walking up
([`paths.ts:17-19`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/config/paths.ts#L17-L19));
`AGENTS.md`, `CLAUDE.md`, deprecated `CONTEXT.md`
([`session/instruction.ts:65-67`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/session/instruction.ts#L65-L67)).
No per-project database; sessions live in the home DB.

### Where the project path is recorded

- SQLite table `project`: `worktree` (absolute), `sandboxes` (array), and table `project_directory`
  `directory` with `type` main/root/git_worktree
  ([`packages/core/src/project/sql.ts:7-34`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/project/sql.ts#L7-L34)).
- On the 2.0 line (at `cb07c55`), all in SQLite: `session_v2.directory` and `path`
  (the session's working directory and its path under the project,
  [`packages/core/src/session/sql.ts:22-37`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/core/src/session/sql.ts#L22-L37));
  `project.time_archived` for a project the user archived
  ([`packages/core/src/project/sql.ts:12-29`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/core/src/project/sql.ts#L12-L29));
  the `worktree` table, `project_id`, `directory` (absolute), `strategy` and `time_created`, one row
  per worktree OpenCode made, which replaces the deprecated `project_directory`
  ([`packages/core/src/worktree/sql.ts:6-20`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/core/src/worktree/sql.ts#L6-L20),
  [`project/sql.ts:31-47`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/core/src/project/sql.ts#L31-L47));
  and the `permission` table's `project_id`, which ties standing grants to a project. Outside
  SQLite, `~/.local/state/opencode/frecency.jsonl` lists absolute paths of files attached to prompts
  ([Other records of use](#other-records-of-use)); their directories point at projects but are
  file paths, not project roots.
- Legacy `storage/project/<id>.json` key `worktree`
  ([`storage.ts:122-132`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/storage/storage.ts#L122-L132)).

## Parsing

### Transcript stores

One SQLite file for all projects: `~/.local/share/opencode/opencode.db`
([`packages/core/src/global.ts:3`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/global.ts#L3),[`11`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/global.ts#L11)
uses `xdg-basedir` with no platform branching, so the same path on macOS and Windows;
[`packages/core/src/database/database.ts:49-54`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/database/database.ts#L49-L54)).
The name is `opencode.db` on the latest, beta and prod channels or when
`OPENCODE_DISABLE_CHANNEL_DB` is set, otherwise `opencode-<channel>.db` in the same directory
([`database.ts:48-54`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/database/database.ts#L48-L54));
on the 2.0 line the rule is in
[`packages/cli/src/database-path.ts:4-13`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/cli/src/database-path.ts#L4-L13)
and the `dev` channel also maps to `opencode.db`. `OPENCODE_DB` can relocate it, and a relative
value is joined to the data directory
([`:43-47`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/database/database.ts#L43-L47),
[`database.ts:44-46`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/database/database.ts#L44-L46)).
One `message` row per message, one `part` row per content part. A legacy JSON tree
`~/.local/share/opencode/storage/`
([`packages/opencode/src/storage/storage.ts:224`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/storage/storage.ts#L224))
still exists ([Format versions](#format-versions)).

`journal_mode=WAL`
([`database.ts:27`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/database/database.ts#L27),
also at open
[`sqlite.bun.ts:164`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/database/sqlite.bun.ts#L164));
`wal_checkpoint(PASSIVE)` runs at startup only
([`database.ts:32`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/database/database.ts#L32)).
Rows written since the last checkpoint live only in `opencode.db-wal`, so a copy without the sidecar
loses recent messages. No encryption or compression.

Other records of use, beside this store, are in [Other stores](#other-stores), under
[Other records of use](#other-records-of-use).

#### What survives a delete

In 2.0, deleting a session publishes the durable `session.deleted` and removes the session's events
and sequence
([`packages/core/src/session.ts:378-388`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/core/src/session.ts#L378-L388);
[`packages/core/src/bus.ts:702-714`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/core/src/bus.ts#L702-L714));
the projector deletes the `session_v2` row
([`projector.ts:552-554`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/core/src/session/projector.ts#L552-L554)),
which cascades to `session_message`, `session_pending`, `session_inbox` and the instruction tables
([`packages/core/src/session/sql.ts:79-160`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/core/src/session/sql.ts#L79-L160)),
child sessions first. Nothing deletes the legacy `session`, `message` and `part` rows the upgrade
copied from, and no record of the deletion is left. Archiving sets `time_archived` and unarchiving
clears it
([`projector.ts:615-627`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/core/src/session/projector.ts#L615-L627)).

In 1.x a delete removes the `session` row with its `session_share` and `event` rows
([`session/session.ts:606-623`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/session/session.ts#L606-L623),
[`projector.ts:257-259`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/session/projector.ts#L257-L259),
[`event.ts:514-521`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/event.ts#L514-L521)),
and the `session.deleted` subscriber of ShareNext (the share client module, `share-next.ts`) asks
the share service to delete the public copy
([`share-next.ts:200`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/share/share-next.ts#L200));
whether that request still finds the share row before the cascade removes it is not determined from
source, and no local file records that the remote copy was deleted. What remains: the legacy
`session`, `message` and `part` rows of a database migrated to 2.0, which nothing deletes;
`prompt-history.jsonl`, the last 50 TUI prompts with no session, project or time; unsent prompts in
`prompt-stash.jsonl`; a pinned session id in `session.json`; full tool outputs under `tool-output/`
for seven days; and, for a 1.x share, the `creating share` and `removing share` lines in
`log/opencode.log` ([Sync, relay and origin rows](#sync-relay-and-origin-rows)). An archived
session keeps every row, with `time_archived` set. The analyzer reads the legacy V1 tables, for a
session `session_message` does not hold, `opencode-next.db`, `prompt-history.jsonl` and
`session.json` as secondary sources, and `prompt-stash.jsonl` as a primary record of unsent
prompts; it does not read the log, and `tool-output/` is excluded from collection (see
[Secondary sources](../analyzer/docs/output.md#secondary-sources)).

### Record schema

Git branch is not recorded anywhere. Role: `message.data.role`. Project
path: `project.worktree`; cwd: `session.directory` or assistant
`data.path.cwd`.

Drizzle tables in
[`packages/core/src/session/sql.ts`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/session/sql.ts).

#### `session` and `project`

`session`
([`:22-66`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/session/sql.ts#L22-L66)):
`id` (`ses_...`), `project_id` → `project.id`, `workspace_id`, `parent_id`, `slug`, `directory`
(cwd, stored with `/` on Windows,
[`database/path.ts:5-8`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/database/path.ts#L5-L8)),
`path`, `title`, `version` (OpenCode version), `share_url`, `summary_*`, `metadata` JSON, `cost`,
`tokens_*`, `revert` JSON, `permission` JSON, `agent`, `model` JSON `{id, providerID, variant?}`
([`:52-56`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/session/sql.ts#L52-L56)),
`time_created`, `time_updated` (epoch milliseconds,
[`database/schema.sql.ts:3-10`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/database/schema.sql.ts#L3-L10)
uses `Date.now()`), `time_compacting`, `time_archived`.

`project`
([`project/sql.ts:6-18`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/project/sql.ts#L6-L18)):
`id`, `worktree` (absolute repository root), `vcs`, `name`, `time_*`, `sandboxes` JSON array.

#### `message` and `part`

`message`
([`sql.ts:68-80`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/session/sql.ts#L68-L80);
CREATE in
[`database/migration/20260127222353_familiar_lady_ursula.ts:22-31`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/database/migration/20260127222353_familiar_lady_ursula.ts#L22-L31)):
`id` (`msg_...`), `session_id`, `time_created`, `time_updated` (ms), `data` JSON = the V1 message
minus `id` and `sessionID`
([`session/projector.ts:77-82`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/session/projector.ts#L77-L82)).
`data`
([`packages/schema/src/v1/session.ts`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/schema/src/v1/session.ts)):
user
([`:332-354`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/schema/src/v1/session.ts#L332-L354))
`role:"user"`, `time.created` (ms), `agent`, `model:{providerID, modelID, variant?}`, `format?`,
`summary?`, `system?`, `tools?`; assistant
([`:453-485`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/schema/src/v1/session.ts#L453-L485))
`role:"assistant"`, `time.{created, completed?}`, `parentID` (the user message), `modelID`,
`providerID`, `mode`, `agent`, `path.{cwd, root}`, `cost`,
`tokens.{input, output, reasoning, cache.{read, write}}`, `error?` (`{name, data}`), `finish?`,
`variant?`.

`part`
([`sql.ts:82-98`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/session/sql.ts#L82-L98);
CREATE
[`...ursula.ts:33-42`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/database/migration/20260127222353_familiar_lady_ursula.ts#L33-L42)):
`id` (`prt_...`), `message_id`, `session_id`, `time_created`, `time_updated`, `data` JSON = the part
minus `id`, `messageID` and `sessionID`
([`projector.ts:84-87`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/session/projector.ts#L84-L87)).
`data.type` discriminator
([`v1/session.ts:357-370`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/schema/src/v1/session.ts#L357-L370)):
`text`
([`:102-115`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/schema/src/v1/session.ts#L102-L115):
`text`, `synthetic?`, `ignored?`, `time.{start,end?}`); `reasoning`
([`:118-127`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/schema/src/v1/session.ts#L118-L127):
`text`, `time.{start,end?}`); `tool`
([`:315-322`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/schema/src/v1/session.ts#L315-L322):
`callID`, `tool` (name), `state.status` in `pending|running|completed|error`
([`:259-302`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/schema/src/v1/session.ts#L259-L302)),
`state.input` on all, `completed` adds `output`, `title`, `metadata`, `time.{start,end}`, `error`
adds `error`, `time.{start,end}`); the tool call and its result are the same part, mutated in place;
`step-start`
([`:233-237`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/schema/src/v1/session.ts#L233-L237),
`snapshot?`), `step-finish`
([`:240-256`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/schema/src/v1/session.ts#L240-L256),
`reason`, `cost`, `tokens`), `subtask`
([`:204-217`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/schema/src/v1/session.ts#L204-L217),
`prompt`, `description`, `agent`), `compaction`
([`:195-201`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/schema/src/v1/session.ts#L195-L201)),
`snapshot`, `patch`, `file`, `agent`, `retry`.

#### `session_message`

Newer V2 projection in the same database: `session_message`
([`sql.ts:119-138`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/session/sql.ts#L119-L138)):
`id`, `session_id`, `type` in
`agent-switched|model-switched|user|synthetic|system|shell|assistant|compaction`
([`packages/schema/src/session-message.ts:200-213`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/schema/src/session-message.ts#L200-L213)),
`seq`, `time_created` (ms,
[`projector.ts:203`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/session/projector.ts#L203)),
`data` JSON. Assistant `data`
([`session-message.ts:165-189`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/schema/src/session-message.ts#L165-L189)):
`agent`, `model.{id, providerID}`, `content[]` with `type` in `text|reasoning|tool`
([`:122-157`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/schema/src/session-message.ts#L122-L157));
tool items carry `id`, `name`, `state.status`, `state.input`, `state.content[]`
(`{type:"text",text}`), `state.error.message`, `time.{created, ran?, completed?}`. User `data`:
`text`, `files?`, `agents?`
([`:44-51`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/schema/src/session-message.ts#L44-L51)).
Migration
[`20260622170816_reset_v2_session_state.ts:8-14`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/database/migration/20260622170816_reset_v2_session_state.ts#L8-L14)
wipes `session_message`, `session_input` and `event` on upgrade, so in a 1.x database (one with no
`session_v2` table) `message`/`part` are the durable transcript and `session_message` is
supplementary; in a 2.0 database `session_message` is the transcript
([Format versions](#format-versions)). The two switch records carry the common base
`id`, `metadata?` and `time.created`
([`session-message.ts:24-28`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/schema/src/session-message.ts#L24-L28))
plus `agent` (a string) for `agent-switched`
([`:30-35`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/schema/src/session-message.ts#L30-L35))
and `model` for `model-switched`
([`:37-42`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/schema/src/session-message.ts#L37-L42)),
a `Model.Ref` `{id, providerID, variant?}`
([`packages/schema/src/model.ts:13-18`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/schema/src/model.ts#L13-L18)).
The projector stores `id` and `type` in their own columns and the rest of the record in `data`
([`packages/core/src/session/projector.ts:192-208`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/session/projector.ts#L192-L208)),
so `data` is `{time, agent}` or `{time, model}`.

#### `event`

`event`
([`core/src/event/sql.ts:10-25`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/event/sql.ts#L10-L25)):
`aggregate_id` = session id, `seq`, `type` such as `message.updated.1` and `message.part.updated.1`
([`schema/src/event.ts:94-96`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/schema/src/event.ts#L94-L96)),
`data` JSON with full message or part copies
([`core/src/event.ts:336-348`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/event.ts#L336-L348)).
Also wiped by the reset migration.

### Joins and timestamps

`session.project_id → project.id` (worktree); `message.session_id → session.id`;
`part.message_id → message.id` (`part.session_id` is denormalised). Order parts by `part.id` (ids
are time-ordered,
[`v1/session.ts:17-27`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/schema/src/v1/session.ts#L17-L27))
or `time_created`. Tool call and result are one `part` row; assistant → user through
`data.parentID`. `session_message` joins on `session_id`, ordered by `seq`. Times are epoch
milliseconds ([Record schema](#record-schema)).

### Other stores

#### Other records of use

Cited at `cb07c55`; the TUI files are at the same paths at `907b3bc`, for example
[`packages/tui/src/prompt/history.tsx:53`](https://github.com/anomalyco/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/tui/src/prompt/history.tsx#L53).
What the parser reads of them is under
[Past-use rows and secondary sources](#past-use-rows-and-secondary-sources).

| Source | Holds | Survives | Joins on | Plan |
| --- | --- | --- | --- | --- |
| `~/.local/state/opencode/prompt-history.jsonl` ([`packages/tui/src/prompt/history.tsx:20-23`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/tui/src/prompt/history.tsx#L20-L23), [`:60-75`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/tui/src/prompt/history.tsx#L60-L75), [`:93-100`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/tui/src/prompt/history.tsx#L93-L100), [`:139`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/tui/src/prompt/history.tsx#L139)) | the last 50 TUI prompts, one JSON object per line, `{text, files, agents, skills, pasted, mode?}` with `mode` `normal` or `shell` (at `907b3bc` `{input, mode?, parts}`, [`history.tsx:9-12`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/tui/src/prompt/history.tsx#L9-L12)); no session id, project or time | session delete, project removal; trimmed to 50 and rewritten at each start, dropping lines that do not parse ([`:82-87`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/tui/src/prompt/history.tsx#L82-L87)) | text only | read, secondary, empty `timestamp_utc` |
| `~/.local/state/opencode/prompt-stash.jsonl` ([`packages/tui/src/prompt/stash.tsx:9-14`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/tui/src/prompt/stash.tsx#L9-L14), [`:40`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/tui/src/prompt/stash.tsx#L40)) | up to 50 prompts set aside unsent, `{prompt, timestamp}` (epoch ms), `prompt` a history entry (at `907b3bc` `{input, parts, timestamp}`, [`stash.tsx:9-13`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/tui/src/prompt/stash.tsx#L9-L13)) | everything; a stashed prompt may never reach a transcript | none (text and time when it was later sent) | read, primary for what it holds (unsent prompts), `system` rows |
| `~/.local/state/opencode/session.json` ([`packages/tui/src/context/local.tsx:517-530`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/tui/src/context/local.tsx#L517-L530)) | `{pinned: [<session id>...]}`, ids of sessions the user pinned | session delete (the id stays until the user unpins it) | session id | read, secondary: a pinned id with no session left is a trace of a deleted session |
| Legacy `session`, `message`, `part` tables in a database migrated to 2.0 ([Format versions](#format-versions)) | the 1.x transcript, copied into `session_message` by the migration | deletion of the session in 2.0, which removes only the `session_v2` row and what cascades from it | session id, message id (the copy keeps the V1 ids, [`v1-migration.bun.ts:353`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/core/src/database/v1-migration.bun.ts#L353), [`:404`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/core/src/database/v1-migration.bun.ts#L404)) | secondary: read only for a session `session_message` does not hold, its event rows marked `recovered` by the parser |
| `~/.local/share/opencode/opencode-next.db` ([`packages/core/src/database/v1-migration.bun.ts:702-706`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/core/src/database/v1-migration.bun.ts#L702-L706), [`:846`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/core/src/database/v1-migration.bun.ts#L846)) | a pre-release 2.0 database (`project`, `session`, `session_message`), imported read-only and left in place | everything after the import | session id | secondary (`secondary()`), so a session it shares with `opencode.db` is shown once |
| `~/.local/share/opencode/tool-output/tool_<id>` ([`packages/core/src/tool-output.ts:13-18`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/core/src/tool-output.ts#L13-L18), [`:86-90`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/core/src/tool-output.ts#L86-L90)) | full text of a tool result the transcript cut at 2000 lines or 50 KiB, kept seven days | session delete within the seven days | the path in the transcript marker `[showing lines 1-<n> of <m>; full output saved to <file>]` | secondary; not collected (catalog exclusion) |
| desktop app `drafts.sqlite` in `<appData>/ai.opencode.desktop*` ([`packages/desktop/src/main/storage/schema.ts:3-29`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/desktop/src/main/storage/schema.ts#L3-L29)) | prompt drafts and history, `document(key, value)` | not determined | not determined | not collected; needs a real-install round |

`~/.local/state/opencode/frecency.jsonl` holds one line per file the user attached to a prompt,
`{path, frequency, lastOpen}`, `path` absolute (resolved against the TUI's working directory) and
`lastOpen` in epoch milliseconds, up to 1000 paths
([`packages/tui/src/prompt/frecency.tsx:8-10`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/tui/src/prompt/frecency.tsx#L8-L10),
[`:42`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/tui/src/prompt/frecency.tsx#L42),
[`:61`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/tui/src/prompt/frecency.tsx#L61));
it has no session id and records files, not prompts
([Where the project path is recorded](#where-the-project-path-is-recorded)).

The 2.0 runtime keeps no other mirror: the `event` table holds payloads
only on the hosted (workerd) server, never in the CLI
([Event payloads in 2.0](#event-payloads-in-20)).

#### Share records

A session published to OpenCode's web share service (the `/share` command,
[Sync, relay and origin](#sync-relay-and-origin-rows)) has one `session_share` row
([`packages/core/src/share/sql.ts:5-13`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/share/sql.ts#L5-L13);
CREATE in
[`...ursula.ts:89-97`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/database/migration/20260127222353_familiar_lady_ursula.ts#L89-L97)):
`session_id` (primary key, references `session.id` with `ON DELETE CASCADE`), `id` (the share id;
the hosted service makes it the last eight characters of the session id,
[`packages/enterprise/src/core/share.ts:117-122`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/enterprise/src/core/share.ts#L117-L122)),
`secret` (a random UUID the service returns and the client sends back with every sync and delete, so
a credential for the public copy), `url` (the public page) and `time_created`, `time_updated` (epoch
ms,
[`database/schema.sql.ts:3-10`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/database/schema.sql.ts#L3-L10)).
`ShareNext.create` writes it once the service has answered the create request, as an upsert when the
session is shared again
([`packages/opencode/src/share/share-next.ts:310-336`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/share/share-next.ts#L310-L336)),
and `ShareNext.remove` deletes it after the service accepts the delete
([`:338-359`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/share/share-next.ts#L338-L359)).
The same URL goes into `session.share_url`
([`packages/opencode/src/share/session.ts:26-32`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/share/session.ts#L26-L32),
[`packages/opencode/src/session/session.ts:810-812`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/session/session.ts#L810-L812),
[`core/src/session/projector.ts:56`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/session/projector.ts#L56)),
which `/unshare` sets back to NULL
([`share/session.ts:34-37`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/share/session.ts#L34-L37)).
Each of these changes is also a durable `session.updated.1` event
([`schema/src/v1/session.ts:502-506`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/schema/src/v1/session.ts#L502-L506),
[`:580-587`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/schema/src/v1/session.ts#L580-L587);
[`core/src/event.ts:336-347`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/event.ts#L336-L347))
whose `data.info.share.url` holds the URL and `data.info.time.updated` the time of the change
([`session/session.ts:734-746`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/session/session.ts#L734-L746)),
so the `event` table still holds the URL after an unshare. A session never shared has no
`session_share` row, a NULL `share_url` and no `share` key in its events. Deleting a session removes
its `session_share` row with it ([What survives a delete](#what-survives-a-delete)); the upgrade
migration
[`20260622170816_reset_v2_session_state.ts:11`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/database/migration/20260622170816_reset_v2_session_state.ts#L11)
emptied `event` once.

#### Permission records

OpenCode asks before a tool call when the rules say `ask`; the
user answers `once`, `always` or `reject` (`Permission.Reply`,
[`packages/schema/src/permission.ts:42-54`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/schema/src/permission.ts#L42-L54)).
The `permission.asked` and `permission.replied` events are ephemeral, so the answer itself is never
stored; what lands on disk is:

- Standing grants. The `permission` table, `id` (`per_...`), `project_id` (references `project.id`,
  `ON DELETE CASCADE`), `action` (the tool's permission name), `resource` (the pattern granted) and
  `time_created`, `time_updated` (epoch ms), one row per pattern of an `always` reply
  ([`packages/core/src/permission/sql.ts:7-20`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/core/src/permission/sql.ts#L7-L20);
  [`packages/core/src/permission.ts:299-306`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/core/src/permission.ts#L299-L306)).
  Saved rows are applied as `allow` rules for every later session of the project
  ([`:147-156`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/core/src/permission.ts#L147-L156)).
  The table exists at `907b3bc` with the same columns
  ([`packages/core/src/permission/sql.ts:7-19`](https://github.com/anomalyco/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/permission/sql.ts#L7-L19)),
  but the 1.x CLI there keeps `always` answers in memory only
  ([`packages/opencode/src/permission/index.ts:143-150`](https://github.com/anomalyco/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/permission/index.ts#L143-L150)),
  so on a 1.x install the table is written only by the 2.0 runtime it bundles. An older shape,
  `permission(project_id, data)`, was created by the first migration and dropped by
  [`20260601202201_amazing_prowler.ts:8`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/core/src/database/migration/20260601202201_amazing_prowler.ts#L8)
  ([`20260127222353_familiar_lady_ursula.ts:44-51`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/core/src/database/migration/20260127222353_familiar_lady_ursula.ts#L44-L51)).
- The session's rule list. `session.permission` (`session_v2.permission` in 2.0) is a
  `Permission.Ruleset`, a JSON array of `{action, resource, effect}` with `effect` in
  `allow|deny|ask`
  ([`packages/schema/src/permission.ts:56-67`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/schema/src/permission.ts#L56-L67)),
  set at creation and by the durable `session.permissions` event
  ([`packages/schema/src/session-event.ts:121-129`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/schema/src/session-event.ts#L121-L129);
  [`packages/core/src/session/projector.ts:593-596`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/core/src/session/projector.ts#L593-L596)).
  The 2.0 upgrade copies 1.x sessions with `permission` NULL
  ([`packages/core/src/database/v1-migration.bun.ts:611-628`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/core/src/database/v1-migration.bun.ts#L611-L628))
  and a later migration clears the column for every session
  ([`20260910120000_clear_v1_session_permission.ts:8`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/core/src/database/migration/20260910120000_clear_v1_session_permission.ts#L8)),
  so it holds only rules set after that. The configured rules are the config's `permissions` list
  and each agent's
  ([`packages/schema/src/config.ts:55`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/schema/src/config.ts#L55),
  [`packages/schema/src/config/agent.ts:21`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/schema/src/config/agent.ts#L21)).
- Per-call outcomes, in the transcript. In 1.x a refused call is a tool part with `state.status`
  `error` and `state.error` the refusal's text
  ([`packages/opencode/src/session/processor.ts:185-203`](https://github.com/anomalyco/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/session/processor.ts#L185-L203)):
  "The user rejected permission to use this specific tool call.", the same followed by "with the
  following feedback: " and the user's text, or "The user has specified a rule which prevents you
  from using this specific tool call. ..." for a `deny` rule
  ([`packages/core/src/v1/permission.ts:7-25`](https://github.com/anomalyco/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/v1/permission.ts#L7-L25)).
  In 2.0 the assistant's tool item gets `state.status` `error` with `error.type` `aborted` and
  message "The user declined this tool call" for a plain rejection
  ([`packages/core/src/session/runner/step.ts:207-215`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/core/src/session/runner/step.ts#L207-L215)),
  and `error.type` `permission.rejected` for a `deny` rule or a rejection with feedback
  ([`packages/core/src/session/to-session-error.ts:61-62`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/core/src/session/to-session-error.ts#L61-L62)),
  whose message is the rule's `reason` or `Permission denied: <permission>` for a rule and the
  user's text for feedback
  ([`packages/core/src/permission.ts:66-79`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/core/src/permission.ts#L66-L79)),
  so the two cannot be told apart from the record. An approval leaves no per-call record in either
  version.

#### Event payloads in 2.0

The `event` table keeps payloads only when the bus is configured to persist them, which the hosted
(workerd) server does and the CLI does not
([`packages/core/src/bus.ts:178-179`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/core/src/bus.ts#L178-L179),
[`:203`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/core/src/bus.ts#L203),
[`:908`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/core/src/bus.ts#L908);
[`packages/server/src/workerd.ts:64-69`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/server/src/workerd.ts#L64-L69)),
and the 2.0 upgrade deletes every existing `event` row
([`v1-migration.bun.ts:549-566`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/core/src/database/v1-migration.bun.ts#L549-L566)).

### Format versions

Legacy JSON under `~/.local/share/opencode/storage/`, one file per key: `project/<projectID>.json`
(`{id, vcs, worktree, time}`), `session/<projectID>/<sessionID>.json`,
`message/<sessionID>/<msgID>.json`, `part/<msgID>/<partID>.json`, `session_diff/<sessionID>.json`,
marker `storage/migration` (integer)
([`storage.ts:63-65`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/storage/storage.ts#L63-L65),[`81-211`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/storage/storage.ts#L81-L211),[`224-225`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/storage/storage.ts#L224-L225)).
Before that, per-project
`~/.local/share/opencode/project/<slug>/storage/session/{info,message,part}/...`
([`storage.ts:97`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/storage/storage.ts#L97),[`139`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/storage/storage.ts#L139),[`150`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/storage/storage.ts#L150),[`165`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/storage/storage.ts#L165);
[`packages/web/src/content/docs/troubleshooting.mdx:35-36`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/web/src/content/docs/troubleshooting.mdx#L35-L36)).
JSON records carry `id`, `sessionID` and `messageID` inline (the database `data` column strips
them). The SQLite `migration` table
([`database/migration.ts:30`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/database/migration.ts#L30))
lists applied schema migrations; `data_migration` tracks data moves. At this commit no code imports
`storage/` JSON into SQLite; `storage/` is still written for `session_diff`
([`session/revert.ts:77`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/session/revert.ts#L77)),
so both trees coexist. `opencode import`
([`cli/cmd/import.ts:196-225`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/cli/cmd/import.ts#L196-L225))
loads exported JSON into `message`/`part`.

OpenCode 2.0 (the `dev` branch at
[`cb07c55`](https://github.com/anomalyco/opencode/commit/cb07c555d4d1ca34430b7857b52f104c3d95e09e),
not in a tagged release yet). The code moved to `packages/core`,
`packages/cli`, `packages/tui` and `packages/util`; the database is still `opencode.db` (or
`opencode-<channel>.db`) under the same XDG data directory
([`packages/cli/src/database-path.ts:4-13`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/cli/src/database-path.ts#L4-L13)).
What changes for the parser:

- Sessions live in `session_v2`, same columns as `session` plus `fork_session_id`, `fork_boundary`,
  `time_idle`, `time_viewed`, `idle_outcome`, `resume_attempts`
  ([`packages/core/src/session/sql.ts:22-68`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/core/src/session/sql.ts#L22-L68)).
  A database created by 2.0 is bootstrapped from the current schema
  ([`packages/core/src/database/migration.ts:26-36`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/core/src/database/migration.ts#L26-L36);
  [`schema.gen.ts:184`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/core/src/database/schema.gen.ts#L184)),
  which has no `session`, `message`, `part`, `session_share` or `todo` table. A database upgraded
  from 1.x gets `session_v2` beside the old tables and the upgrade copies each 1.x session into it
  and its `message`/`part` rows into `session_message`, leaving the old rows in place
  ([`packages/core/src/database/migration/20260804233008_loose_psylocke.ts:90`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/core/src/database/migration/20260804233008_loose_psylocke.ts#L90),
  [`:139-140`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/core/src/database/migration/20260804233008_loose_psylocke.ts#L139-L140);
  [`v1-migration.bun.ts:534-700`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/core/src/database/v1-migration.bun.ts#L534-L700)).
  A database from a pre-release 2.0 build has its `session` renamed to `session_v2`
  ([`loose_psylocke.ts:13-38`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/core/src/database/migration/20260804233008_loose_psylocke.ts#L13-L38)).
- `session_message` is the only transcript for new sessions. Its `type` adds `location-switched`
  (`location.{directory, workspaceID}`, `projectID`, `subpath`, `previous`), `skill` (`skill`,
  `name`, `text`) and `idle` (`outcome` in `succeeded|failed|interrupted`, `error`) to the types in
  [Record schema](#record-schema); `agent-switched` and `model-switched` gain `previous`; `shell` is
  now `shellID`, `command`, `status` (`running|exited|timeout|killed`), `exit` and `output`, an
  object `{output, cursor, size, truncated}`; tool items gain `executed` and the `streaming` status;
  assistant gains `snapshot`, `finish`, `retry`, `time.streamed`
  ([`packages/schema/src/session-message.ts:45-125`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/schema/src/session-message.ts#L45-L125),
  [`:213-237`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/schema/src/session-message.ts#L213-L237),
  [`:290-309`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/schema/src/session-message.ts#L290-L309);
  [`packages/schema/src/shell.ts:25`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/schema/src/shell.ts#L25),
  [`:77-84`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/schema/src/shell.ts#L77-L84)).
  Model and agent changes are also projected onto `session_v2.model` and `agent`
  ([`projector.ts:555-571`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/core/src/session/projector.ts#L555-L571)).
- Session sharing is removed
  ([`packages/tui/src/config/keybind.ts:125-127`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/tui/src/config/keybind.ts#L125-L127));
  `session_v2.share_url` remains as a column and is copied from 1.x sessions.
- New tables: `permission` ([Permission records](#permission-records)), `worktree` (one row per
  worktree OpenCode made,
  [Where the project path is recorded](#where-the-project-path-is-recorded)), `kv`,
  `session_pending`, `session_inbox`, `instruction_*`, and `project.time_archived`.
- A session imported with `opencode session import <file or URL>` is written as a new session with
  the local version and the import time as `time_updated`, and no field records where it came from
  ([`packages/cli/src/commands/handlers/session/import.ts:15-22`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/cli/src/commands/handlers/session/import.ts#L15-L22);
  [`packages/core/src/session/transfer.ts:64-126`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/core/src/session/transfer.ts#L64-L126)).

### Secrets

`credential.value` JSON (`{type:"oauth", access, refresh}` or `{type:"key", key}`,
[`schema/src/credential.ts:16-34`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/schema/src/credential.ts#L16-L34)),
`account.access_token` and `refresh_token`, `control_account.*`, `session_share.secret` (the share
service's per-share secret that authorises sync to and deletion of the public copy,
[Share records](#share-records)), plus the legacy files `~/.local/share/opencode/auth.json`
([`auth/index.ts:10`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/auth/index.ts#L10),[`14-35`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/auth/index.ts#L14-L35))
and `mcp-auth.json`. On the 2.0 line a `credential` row whose `integration_id` is `gitlab` holds a
GitLab OAuth token with the `api` scope, and one under `github-copilot` a GitHub token
([Source host rows](#source-host-rows)). Tool `input` and `output` text can contain pasted tokens.

### Parser plan

Globs: `.local/share/opencode/opencode*.db{,-wal,-shm}`,
`.local/share/opencode/storage/{session,message,part,project}/**/*.json`,
`.local/share/opencode/project/*/storage/session/**/*.json`.

Rows: one per `part`, plus a `user` row per user message.

| Column | Source |
| --- | --- |
| timestamp_utc | part `data.time.start`, else `part.time_created` (ms) |
| session_id | `session.id` |
| project_path | `project.worktree`, fallback `session.directory` |
| git_branch | empty |
| turn_type | user message → user; `text` → assistant; `reasoning` → thinking; `tool` pending/running → tool_use; `tool` completed/error → tool_use plus tool_result; `step-*` → system |
| model | `data.providerID/modelID` |
| tool_name, tool_use_id | `data.tool`, `data.callID` |
| text | text, input JSON, output or error |

Per requirement class, the subsections that follow say what the parser does today and what is
planned (detail in [Transcript stores](#transcript-stores), [Other stores](#other-stores) and
[Format versions](#format-versions)).

#### Permission rows

The message's `agent` (V1 user and assistant, V2 assistant, V2 `agent-switched` `agent`) is tracked
the same way as `permission mode: agent=<name>` and `permission mode: agent=<old> -> agent=<new>`,
because an agent (`build`, `plan` or a custom one) is a mode that carries its own permission ruleset
(the agent's `permission: PermissionV1.Ruleset`,
[`packages/opencode/src/agent/agent.ts:44`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/agent/agent.ts#L44)).
Both rows take the message's timestamp.

The per-call outcomes and standing state of [Permission records](#permission-records) are read
since the source review of 2026-10-21, source-derived; not observed on any real install. Per call,
after the `tool_result` row of a refused call and with its `tool_name`, `tool_use_id` and time:
`permission decision: rejected` for the 1.x `state.error` "The user rejected permission to use this
specific tool call." or the 2.0 `error.type` `aborted` with "The user declined this tool call";
`permission decision: rejected with feedback` for the 1.x text followed by " with the following
feedback: "; `permission decision: denied by rule` for the 1.x "The user has specified a rule which
prevents you from using this specific tool call."; and `permission decision: rejected
(permission.rejected)` for the 2.0 `error.type` `permission.rejected`, which covers both a rule and
feedback. No approval is recorded. Standing: one `permission setting: <action> <resource>=allow
(from <database> permission, time_created)` row per `permission` table row at its `time_created`,
`project_path` the project's `worktree` and no session; and one `permission setting: session
<permission> <pattern>=<effect> (from <database> <table>.permission, time_updated)` row per rule of
a session's `permission` column (2.0 `{action, resource, effect}`, 1.x `{permission, pattern,
action}`) at the session's `time_updated`, a value that is not a JSON array being a `parser:` row.
The table's first shape, `permission(project_id, data)`, is skipped, since its `data` is not
described here. The project's `sandboxes` array is not read
([issue 98](https://github.com/seanthegeek/doubleagent/issues/98)).

#### Model rows

Model and mode rows. A `system` row `model: <modelID> provider=<providerID>`
precedes the first message of a session that names a model (V1 user
`model.{providerID, modelID}`, V1 assistant `providerID` and `modelID`, V2
assistant `model.{id, providerID}`), and `model: <old> -> <new>
provider=<providerID>` precedes every later message, or stands for every V2
`model-switched` record, whose model differs; mode rows are under
[Permission rows](#permission-rows).
Planned: add the 2.0 `variant` beside `provider=` when present. No alias
beside a resolved id exists.

#### Past-use rows and secondary sources

`prompt-history.jsonl` (secondary, empty `timestamp_utc`, text
match), `prompt-stash.jsonl` (primary for unsent prompts, `system`
rows at `timestamp`), `session.json` (`system` row per pinned id that
has no session, at the file's modification time), the legacy V1 tables
of a migrated database (secondary to `session_message`), and
`opencode-next.db` (secondary to `opencode.db`). Read: the legacy V1
tables (a session `session_message` does not hold, its event rows
marked `recovered` "yes" by the parser, since both stores are one
file) and `opencode-next.db` (`secondary()`, deduplicated against the
database it was imported into); `has_secondary` is set.

The three `~/.local/state/opencode` files are read since 2026-10-21, source-derived; not observed
on any real install. `prompt-history.jsonl` is returned by `secondary()`: each line is a `user` row
with an empty `timestamp_utc`, `session_id` and `project_path`, or for `mode` `shell` a `tool_use`
row with tool `shell` and no id, which pairs on text with the transcript's command; the prompt is
`text` on the 2.0 line and `input` at `907b3bc`. `prompt-stash.jsonl` gives one `system` row per
line at its `timestamp`, `prompt stashed (unsent): <text> (from
.local/state/opencode/prompt-stash.jsonl, timestamp)`, from `prompt.text` or the 1.x `input`.
`session.json`, returned by `secondary()`, gives one `system` row per pinned id, `session pinned:
<id> (from .local/state/opencode/session.json pinned, file mtime)`, with that `session_id` at the
file's modification time, so a pinned session with no other row is a trace of a deleted one.
The sources and their joins are under [Other records of use](#other-records-of-use).

#### Sync, relay and origin rows

Sync: OpenCode uploads a session only when it is shared. Sharing starts from `/share` in the TUI,
which asks once and remembers the answer as `share_consent` in `~/.local/state/opencode/kv.json`
([`packages/tui/src/routes/session/index.tsx:465-497`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/tui/src/routes/session/index.tsx#L465-L497),
[`packages/tui/src/context/kv.tsx:15`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/tui/src/context/kv.tsx#L15)),
from `opencode run --share`
([`packages/opencode/src/cli/cmd/run.ts:161-164`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/cli/cmd/run.ts#L161-L164),
[`:535-547`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/cli/cmd/run.ts#L535-L547)),
or for every new top-level session when the config sets `share: "auto"` (or the deprecated
`autoshare: true`) or `OPENCODE_AUTO_SHARE` is set
([`packages/core/src/v1/config/config.ts:57-63`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/v1/config/config.ts#L57-L63),
[`config/config.ts:589-591`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/config/config.ts#L589-L591),
[`effect/runtime-flags.ts:17`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/effect/runtime-flags.ts#L17),
[`share/session.ts:39-46`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/share/session.ts#L39-L46)).
With `share` unset sharing is manual; `share: "disabled"` or `OPENCODE_DISABLE_SHARE` blocks it
([`share/session.ts:28`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/share/session.ts#L28),
[`share-next.ts:23`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/share/share-next.ts#L23)).
Once shared, a full copy and then every session, message, part, diff and model update is posted with
the share secret to `<base>/api/share/<id>/sync`
([`share-next.ts:179-199`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/share/share-next.ts#L179-L199),
[`:247-299`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/share/share-next.ts#L247-L299)),
where the base is `https://opncd.ai` or the config's `enterprise.url`, or, for a user signed in to
an OpenCode console account with an active organisation (the `account` table), the account's URL
under `/api/shares` with its bearer token and an `x-org-id` header
([`:206-222`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/share/share-next.ts#L206-L222)).
OpenCode Zen, OpenCode's hosted model gateway, is a model provider, so its calls are model API calls
and not sync; trace and log export over OTLP (OpenTelemetry's wire protocol) goes only to an
endpoint the user sets in `OTEL_EXPORTER_OTLP_ENDPOINT`
([`core/src/flag/flag.ts:16-17`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/flag/flag.ts#L16-L17)).
The parser yields one `system` row per `session_share` row,
`session synced: share <id> url <url> (from opencode.db session_share, time_created)`, timestamped
by `time_created`, never with `secret`; for a session with no `session_share` row whose
`session.updated.1` events carry `info.share.url`,
`session synced: share url <url> (from opencode.db event session.updated.1, info.time.updated)` at
that event's `info.time.updated`, and
`session unshared (from opencode.db event session.updated.1, info.time.updated)` at the first later
event without `share`; the database name in the text is the file's own (`opencode-<channel>.db` for
a channel database). These rows are fixture only ([Observed](#observed)). What a delete of a shared
session removes, and the request to delete the public copy, are under
[What survives a delete](#what-survives-a-delete); what survives it is the log: `creating share` and
`removing share` lines with the `sessionID` go to `~/.local/share/opencode/log/opencode.log` at the
default INFO level
([`share-next.ts:312`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/share/share-next.ts#L312),
[`:340`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/share/share-next.ts#L340);
[`core/src/observability/logging.ts:49-52`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/observability/logging.ts#L49-L52),
[`:59-65`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/observability/logging.ts#L59-L65);
[`core/src/global.ts:23`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/global.ts#L23)).
Separately, a session can be moved to a workspace (the control plane's `workspace` table) served by
another OpenCode instance (`Workspace.sessionWarp`,
[`control-plane/workspace.ts:559`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/control-plane/workspace.ts#L559)),
recorded as `session.workspace_id` and a `workspace` row
([`core/src/control-plane/workspace.sql.ts:6-18`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/core/src/control-plane/workspace.sql.ts#L6-L18));
the one built-in adapter targets a local worktree
([`control-plane/adapters/worktree.ts:92`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/control-plane/adapters/worktree.ts#L92)),
so a remote target comes from a plugin, and none was researched.

On the default branch at
[`797d2af`](https://github.com/sst/opencode/commit/797d2afdbf8f0b4f189748373ff1a69bde9e0ff7) (checked
2026-10-17) the `share/` modules are gone and the share and unshare keybindings read "Deprecated:
session sharing was removed"
([`packages/tui/src/config/v1/keybind.ts:101-102`](https://github.com/sst/opencode/blob/797d2afdbf8f0b4f189748373ff1a69bde9e0ff7/packages/tui/src/config/v1/keybind.ts#L101-L102)),
as at `cb07c55` ([Format versions](#format-versions)); no migration in that tree drops
`session_share`, so a database created by an earlier release can still hold the share rows in
[Share records](#share-records). The 2.0 CLI keeps no event payloads
([Event payloads in 2.0](#event-payloads-in-20)), so the event-based share rows can no longer
appear for a 2.0 session.

Sync: 1.x shares covered; none at `dev`. Relay: none (no messaging
integration; not found in `packages/core`, `packages/cli`,
`packages/server` or `packages/tui` at `cb07c55`, grepped for
`telegram`, `whatsapp` and `slack`). Origin: none; an imported session
carries no source.

#### Scratch directory rows

A session whose directory is under `~/.local/share/opencode/worktree/` is already classed by the
catalog rule. Since 2026-10-21, source-derived; not observed on any real install, each row of the
2.0 `worktree` table, else of the deprecated `project_directory` table with `type`
`git_worktree`, is a `system` row `scratch directory: <directory> (git worktree of <project
worktree> strategy=<strategy>) (from <database> <table>, time_created)` at its `time_created`,
with the directory as `project_path` and no session; a directory in both tables gives one row.
The temp root ([Scratch directories](#scratch-directories)) is collected on Windows only and gives
no row.

#### Deletion rows

No row marks a deletion; what a delete leaves is under
[What survives a delete](#what-survives-a-delete). Since 2026-10-21, source-derived; not observed
on any real install, a session with `time_archived` set is a `system` row `session archived (from
<database> <table>, time_archived)` at that time, `<table>` being `session_v2` or `session`; an
unarchived session has the column cleared and gets none.

#### Bad-line rows

Done ([issue 139](https://github.com/seanthegeek/doubleagent/issues/139)). Each `message`, `part`,
`session_message` or `event` row whose `data` is not a JSON object is its own `parser:
unparseable record <rowid> (<table>.data: <reason>): <data>` row
carrying the stored value in full, with its row's session and project,
an empty timestamp and `source_line` the rowid; each part file of the
JSON trees that does not decode is `parser: unparseable part <partID>
(<reason>): <file text>`, `source_file` the part file. A message or
session file that does not decode stays one whole-file `parser:`
row. Each line of `prompt-history.jsonl` and `prompt-stash.jsonl` that does not decode is its own
`parser: unparseable line (<reason>): <line>` row, read before OpenCode's next start drops it; a
`session.json` that is not a JSON object is one `parser:` row.

#### Fixtures

Fixture rows:

```sql
INSERT INTO project(id,worktree,vcs,time_created,time_updated,sandboxes) VALUES('proj_a1','/home/u/repo','git',1760000000000,1760000000000,'[]');
INSERT INTO session(id,project_id,slug,directory,title,version,cost,tokens_input,tokens_output,tokens_reasoning,tokens_cache_read,tokens_cache_write,time_created,time_updated) VALUES('ses_01','proj_a1','fix-bug','/home/u/repo','Fix bug','1.2.0',0,0,0,0,0,0,1760000001000,1760000005000);
INSERT INTO message VALUES('msg_01','ses_01',1760000001000,1760000001000,'{"role":"user","time":{"created":1760000001000},"agent":"build","model":{"providerID":"anthropic","modelID":"claude-sonnet-4"}}');
INSERT INTO part VALUES('prt_01','msg_01','ses_01',1760000001000,1760000001000,'{"type":"text","text":"list files"}');
INSERT INTO message VALUES('msg_02','ses_01',1760000002000,1760000005000,'{"role":"assistant","time":{"created":1760000002000,"completed":1760000005000},"parentID":"msg_01","modelID":"claude-sonnet-4","providerID":"anthropic","mode":"build","agent":"build","path":{"cwd":"/home/u/repo","root":"/home/u/repo"},"cost":0.01,"tokens":{"input":10,"output":5,"reasoning":0,"cache":{"read":0,"write":0}}}');
INSERT INTO part VALUES('prt_02','msg_02','ses_01',1760000002000,1760000003000,'{"type":"tool","callID":"call_x1","tool":"bash","state":{"status":"completed","input":{"command":"ls"},"output":"a.txt","title":"ls","metadata":{},"time":{"start":1760000002500,"end":1760000003000}}}');
INSERT INTO credential VALUES('cred_01','anthropic','key','my key','{"type":"key","key":"sk-ant-REDACT"}',NULL,NULL,1,1760000000000,1760000000000);
INSERT INTO permission VALUES('per_01','proj_a1','bash','npm test',1760000003000,1760000003000);
INSERT INTO worktree VALUES('proj_a1','/home/u/.local/share/opencode/worktree/proj_a/brave-otter','git',1760000004000);
```

State files on the 2.0 line, one line each:

```json
{"text":"list files","files":[],"agents":[],"skills":[],"pasted":[]}
{"prompt":{"text":"draft: rotate the keys","files":[],"agents":[],"skills":[],"pasted":[]},"timestamp":1760000005000}
{"pinned":["ses_01"]}
```

#### Version drift

Done ([issue 165](https://github.com/seanthegeek/doubleagent/issues/165)), from source alone; the
real-install round of 2.0 is still to run, and its line types and key sets are to be
compared with this plan. Session headers and `directory` come from
`session_v2`, else `session`; the session-start row adds
`fork=<fork_session_id>`, and a session a 2.0 database holds only in
the 1.x `session` table ends `(only in the 1.x session table)`. In a
database with `session_v2`, `session_message` is the transcript and
the 1.x `message` and `part` rows are read only for a session it does
not hold. The 2.0 `shell` record (`shellID`, `status`, `exit`,
`output.output`) is a `tool_use` row and a `tool_result` row starting
`[timeout]`, `[killed]` or `[exit <code>]` when it did not exit 0;
`location-switched` is a `system` row `location switched: <previous
directory> -> <directory>` (with `workspace=` and `subpath=` when
set; `project_path` stays the session's `directory`), `skill` is
`skill <name>: <text>`, `idle` is `idle: <outcome>` with the error
text, and a compaction row adds its `status` and error. A 2.0 tool
error shows its `error.type` before the message (`[error] aborted The
user declined this tool call`).

#### Source host rows

The agent connects no source-host account for repository access, caches no repository list and
keeps no repository-bound remote conversation; it does store source-host tokens for two model
providers. On the 2.0 line (`cb07c55`, the CLI, core, terminal UI and server packages grepped for
`github`, `gitlab`, `bitbucket`, `oauth`, `installation`, `repositor`, `pull request` and
`clone`), signing in to the GitLab model provider (GitLab Duo) runs an OAuth flow with the `api`
scope against the instance and stores the tokens as a `credential` row with `integration_id`
`gitlab` and `metadata.instanceUrl`
([`packages/core/src/plugin/provider/gitlab.ts:19`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/core/src/plugin/provider/gitlab.ts#L19),
[`:30`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/core/src/plugin/provider/gitlab.ts#L30),
[`:425-436`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/core/src/plugin/provider/gitlab.ts#L425-L436);
[`packages/core/src/credential/sql.ts:5-14`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/core/src/credential/sql.ts#L5-L14)),
a token that can reach that GitLab account's repositories though OpenCode uses it for inference;
the GitHub Copilot provider's device flow asks only for `read:user` and stores the token the same
way under `github-copilot`
([`packages/core/src/plugin/provider/github-copilot.ts:49`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/core/src/plugin/provider/github-copilot.ts#L49),
[`:85`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/core/src/plugin/provider/github-copilot.ts#L85),
[`:123-135`](https://github.com/anomalyco/opencode/blob/cb07c555d4d1ca34430b7857b52f104c3d95e09e/packages/core/src/plugin/provider/github-copilot.ts#L123-L135)).
Neither row names the account's login, and the database holding them is flagged as a secret
whole ([Secrets](#secrets)). `packages/core/src/repository.ts` parses a repository reference for
the `repos/` clones of [Data directory](#data-directory), with no account. On the 1.x line
(`907b3bc`)
`opencode github install` installs the `opencode-agent` GitHub App on a repository, checks the
installation against `api.opencode.ai` and writes the workflow file `.github/workflows/opencode.yml`
into the repository
([`packages/opencode/src/cli/cmd/github.handler.ts:145`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/cli/cmd/github.handler.ts#L145),
[`:284-327`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/cli/cmd/github.handler.ts#L284-L327),
[`:337`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/cli/cmd/github.handler.ts#L337));
`opencode github run` runs inside GitHub Actions, exchanges the runner's OIDC token for an app
token held in memory and sets it in the runner checkout's git configuration
([`:996-1019`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/cli/cmd/github.handler.ts#L996-L1019),
[`:1036-1038`](https://github.com/sst/opencode/blob/907b3bc518fa48e90e8ec24dd327d13eee71c36c/packages/opencode/src/cli/cmd/github.handler.ts#L1036-L1038)).
The installation and its grant are server-side, the token lives only on the CI runner, and the
one trace on a workstation is the workflow file in the repository, a project file the catalog
does not name. Since no file records a login, the parser yields no `source host:` row.

## Confidence

- High: every path, table column, JSON field name, timestamp unit and part type, the WAL mode and
  the absence of a git branch, all from source at the cited commits.
- Medium: the 2.0 layout ([Format versions](#format-versions)) rests on the `dev` branch, not a
  tagged release, and on source alone ([Observed](#observed)).
- Not determined: whether older releases wrote `bin/` under the data directory; the catalog
  excludes `.local/share/opencode/bin`, and neither reviewed commit writes it
  ([Exclusions](#exclusions)).
- Not determined: the exact byte layout of `session_message.data` for every type on a real install.
- Not determined: whether an intermediate release had a startup importer from the `storage/` JSON
  tree into SQLite.
