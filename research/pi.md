# pi

pi is the `pi` coding agent CLI from Earendil Works (`@earendil-works/pi-coding-agent`), a terminal
agent with one per-user directory, `~/.pi/agent`, on every OS, where it writes one append-only
JSONL file per session, a tree of entries linked by `id` and `parentId`. Its only surface is the
CLI (interactive terminal UI, print and RPC modes of one binary); the monorepo's other packages are
libraries. Two other catalog agents reuse its storage or format: little-coder, a launcher that runs
pi itself and writes into the same `~/.pi/agent` ([little-coder.md](little-coder.md)), and Letta
Code's local backend, which writes pi's session format with its own writer into its own directory
([letta.md](letta.md)); [Wrapper attribution](#wrapper-attribution) says how to tell them apart.
Radius, a name that runs through this document, is Earendil Works' hosted model gateway for pi.
This document rests on coding-agent 1.4.0 at commit
[`2c8d1fe`](https://github.com/earendil-works/pi/commit/2c8d1feb147f6b3d97e855da803bfe7b19f3c403).
Catalog agent: `pi`. Parsed: the pi parser reads the session files, `trust.json`, the
`defaultProjectTrust` setting, the debug and crash logs and the older builds' experimental session
metadata.

## Source and evidence

[earendil-works/pi](https://github.com/earendil-works/pi), the monorepo of the `pi` CLI, MIT
licence
([LICENSE:1](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/LICENSE#L1)),
TypeScript. The repository was called `pi-mono` before; its migration code still links issues
under that name
([migrations.ts:74-82](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/migrations.ts#L74-L82)).
Reviewed commit
[`2c8d1fe`](https://github.com/earendil-works/pi/commit/2c8d1feb147f6b3d97e855da803bfe7b19f3c403)
(2026-11-01, the head of `main`), whose `packages/coding-agent/package.json` gives version 1.4.0
([package.json:3](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/package.json#L3)).
The latest release tag is `v1.4.0`
([`4a08025`](https://github.com/earendil-works/pi/commit/4a080252f06caf63a525d7873c5d94d3c23887e0),
2026-10-22); the commit is ahead of it, and [Format versions](#format-versions) lists what the tag's
build writes differently. Tag `v0.83.0`
([`845d6ff`](https://github.com/earendil-works/pi/commit/845d6ff1f6643aba440341cce877ce1c43ebbc39)),
the version little-coder pins, and the commits of the earlier reviews,
[`2003871`](https://github.com/earendil-works/pi/commit/200387122ca450d6387f033949423114a270b96c)
(1.0.2) and
[`5c1736f`](https://github.com/earendil-works/pi/commit/5c1736f82d201d09664642ab0d88147da94b450e)
(1.3.0), are cited only for shapes the reviewed commit no longer writes.

Evidence levels: every path, key and record shape is from source at the reviewed commit unless a
sentence cites an older commit. The vendor's pages under `packages/coding-agent/docs/`
([index.md](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/docs/index.md)) are cited for product behaviour.
The managed install layout comes from the shipped installer scripts,
[the pi.dev shell installer](https://pi.dev/install.sh) (sha256
`ff4ed7b1ea1c058f33249a44093863b50cf95224ab4ff654e722fcbbbdaa31df`) and
[the pi.dev PowerShell installer](https://pi.dev/install.ps1) (sha256
`4667e968e8ddd0b51211eab57250a975d0f40fff3d8492981c61a1a7ca6b46c1`), both fetched 2026-11-01 and
read, not run; they are not in the repository.

### Observed

- No real install. Nothing was installed or run. What stands in is source at
  [`2c8d1fe`](https://github.com/earendil-works/pi/commit/2c8d1feb147f6b3d97e855da803bfe7b19f3c403)
  (coding-agent 1.4.0) and the installer scripts above; every row of the
  [Parser plan](#parser-plan) is source-derived, and the `pi.bug-report`, crash, debug-snapshot and
  permission rows are fixture only.

### Source reviews

- 2026-11-01: [earendil-works/pi](https://github.com/earendil-works/pi) at
  [`2c8d1fe`](https://github.com/earendil-works/pi/commit/2c8d1feb147f6b3d97e855da803bfe7b19f3c403)
  (coding-agent 1.4.0, ahead of tag `v1.4.0`) and the pi.dev installers: every citation re-read at
  the new commit; added the managed install under `install/`, the terminal UI's crash and redraw
  logs, `mcp.log.1`, the HTML export's embedded copy of every entry, the `deviceId`, `trackingId`
  and install-telemetry settings, the `powershell` and `tool_search` tools, and the newer message
  keys; corrected the bug report's crash-log handling (it deletes `crashes.json`).
- 2026-10-21: [earendil-works/pi](https://github.com/earendil-works/pi) at
  [`5c1736f`](https://github.com/earendil-works/pi/commit/5c1736f82d201d09664642ab0d88147da94b450e)
  (coding-agent 1.3.0) and tag `v0.83.0`: the Parser plan gained `defaultProjectTrust`,
  `pi-debug.log`, `crashes.json` and little-coder's permission-gate refusals.
- 2026-10-19: [earendil-works/pi](https://github.com/earendil-works/pi) at
  [`5c1736f`](https://github.com/earendil-works/pi/commit/5c1736f82d201d09664642ab0d88147da94b450e),
  compared file by file with
  [`2003871`](https://github.com/earendil-works/pi/commit/200387122ca450d6387f033949423114a270b96c):
  added `defaultProjectTrust`, `--approve`, `sessionShareDestination`, the debug and crash logs,
  `durationMs`, `argumentsError` and the experimental server's removal.

## Collection

### Per-user storage

One layout on every OS. The agent directory is `$PI_CODING_AGENT_DIR` (with `~` expanded) or
`os.homedir()/.pi/agent`
([config.ts:630-636](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/config.ts#L630-L636);
tilde expansion
[utils/paths.ts:132-137](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/utils/paths.ts#L132-L137)).
There is no XDG, `AppData` or `Library` form: on Windows it is `C:\Users\<user>\.pi\agent`, as the
Windows installer also assumes
([install.ps1:666-668](https://pi.dev/install.ps1)). The `.pi` name and the environment variable
prefix come from `piConfig` in the package's `package.json`
([config.ts:602-611](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/config.ts#L602-L611),
[package.json:6-8](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/package.json#L6-L8)),
so a rebranded build (the source comment's example is `TAU_CODING_AGENT_DIR`) uses another
directory. The vendor's
[Agent directory table](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/docs/configuration.md#agent-directory)
lists the user-facing files.

Under `~/.pi/agent`:

- `sessions/--<cwd>--/<ISO timestamp>_<session id>.jsonl`: one JSONL transcript per session
  ([config.ts:674-676](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/config.ts#L674-L676);
  [session-manager.ts:589-594](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/session-manager.ts#L589-L594),
  [1077-1080](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/session-manager.ts#L1077-L1080)).
  The directory name is the cwd with a leading `/` or `\` dropped and every `/`, `\` and `:`
  replaced by `-`: `/home/dana/webapp` gives `--home-dana-webapp--`, `C:\Users\dana\webapp` gives
  `--C--Users-dana-webapp--`. The file name is the ISO time with `:` and `.` replaced by `-`. A file
  is created only once a user or assistant message exists
  ([session-manager.ts:1161-1170](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/session-manager.ts#L1161-L1170)),
  and `--no-session` writes none
  ([main.ts:364-365](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/main.ts#L364-L365)).
  The directory moves with `--session-dir`, `$PI_CODING_AGENT_SESSION_DIR` or the `sessionDir`
  setting, in that order
  ([main.ts:702-706](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/main.ts#L702-L706));
  a custom directory is flat, with no per-cwd subdirectory
  ([session-manager.ts:1765-1767](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/session-manager.ts#L1765-L1767)).
  Session files written by v0.30.0 sat directly in `~/.pi/agent/*.jsonl` until a migration moved
  them
  ([migrations.ts:84-129](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/migrations.ts#L84-L129)).
- `settings.json`, the global settings
  ([config.ts:654-656](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/config.ts#L654-L656);
  schema
  [settings-schema.ts:205-470](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/settings-schema.ts#L205-L470)):
  default provider and model, `packages`, `extensions`, `skills`, `sessionDir`, `httpProxy`, the
  standing project trust default `defaultProjectTrust` ([Project trust](#project-trust)), the
  `/share` destination `sessionShareDestination`
  ([Sync, relay and origin rows](#sync-relay-and-origin-rows)), and two identifiers: `deviceId`, a
  UUID made the first time a provider's sign-in needs one
  ([settings-manager.ts:1090-1101](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/settings-manager.ts#L1090-L1101)),
  and `trackingId`, a UUID made when analytics is first enabled
  ([1079-1088](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/settings-manager.ts#L1079-L1088)).
- `keybindings.json`
  ([keybindings.ts:337-338](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/keybindings.ts#L337-L338)).
- `auth.json` and `models.json`
  ([config.ts:644-651](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/config.ts#L644-L651)),
  described under [Credentials](#credentials), and `models-store.json`, the provider model
  catalogs pi refreshes itself
  ([models-store.ts:52](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/models-store.ts#L52)).
- `trust.json`: project trust decisions keyed by absolute path ([Project trust](#project-trust)).
- `mcp.json` (MCP servers), `mcp-auth.json` (MCP OAuth state), and `mcp.log` with its rotated copy
  `mcp.log.1`, the log messages MCP servers send, appended one line per message and rotated past
  5 MB
  ([mcp/config.ts:147-149](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/extensions/mcp/config.ts#L147-L149);
  [mcp/oauth.ts:140-148](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/extensions/mcp/oauth.ts#L140-L148);
  [mcp/log.ts:1-10](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/extensions/mcp/log.ts#L1-L10),
  [26-32](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/extensions/mcp/log.ts#L26-L32),
  [50-55](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/extensions/mcp/log.ts#L50-L55);
  [mcp/index.ts:370](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/extensions/mcp/index.ts#L370)).
  Token refreshes are serialised with lock directories `mcp-auth-refresh-<16 hex>.lock` beside it
  ([mcp/oauth.ts:172-185](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/extensions/mcp/oauth.ts#L172-L185)).
- `crashes.json`, the recent crashes, and `pi-debug.log`, written by `/debug`
  ([Debug log and crash log](#debug-log-and-crash-log)).
- `pi-tui-crash.log`, written by the terminal UI when a rendered line is wider than the terminal,
  just before it stops: the time, the width and every rendered screen line, so it can hold
  transcript text
  ([tui-main-screen.ts:522-536](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/tui/src/tui-main-screen.ts#L522-L536);
  the agent directory is its log directory,
  [interactive-mode.ts:635](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/modes/interactive/interactive-mode.ts#L635)).
  `pi-tui-debug.log` holds one `fullRender:` reason line per full redraw, only while
  `PI_TUI_DEBUG_REDRAW=1`
  ([tui-main-screen.ts:326-333](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/tui/src/tui-main-screen.ts#L326-L333)).
- User resources: `extensions/`, `skills/`, `prompts/` (formerly `commands/`), `themes/`, the global
  context file (`AGENTS.override.md`, `AGENTS.md`, `AGENTS.MD`, `CLAUDE.md` or `CLAUDE.MD`),
  `SYSTEM.md` and `APPEND_SYSTEM.md`
  ([resource-loader.ts:1022-1026](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/resource-loader.ts#L1022-L1026),
  [211-213](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/resource-loader.ts#L211-L213),
  [270-274](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/resource-loader.ts#L270-L274),
  [1251](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/resource-loader.ts#L1251),
  [1265](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/resource-loader.ts#L1265);
  [migrations.ts:137-143](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/migrations.ts#L137-L143)).
  A `hooks/` directory or a `tools/` directory holding more than `fd` and `rg` is a leftover of
  older builds that pi only warns about
  ([migrations.ts:222-250](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/migrations.ts#L222-L250)).
  Skills are also read from `~/.agents/skills`
  ([trust-manager.ts:194-195](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/trust-manager.ts#L194-L195)),
  a directory pi shares with other agents and does not own.
- Installed packages (`pi install`): `npm/`, an npm root with its own `package.json`, `.gitignore`
  and `node_modules/<name>`, and `git/<host>/<path>` checkouts, each with its dependencies installed
  inside it
  ([package-manager.ts:2203-2221](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/package-manager.ts#L2203-L2221),
  [2225-2232](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/package-manager.ts#L2225-L2232),
  [2268-2273](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/package-manager.ts#L2268-L2273),
  [2293-2316](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/package-manager.ts#L2293-L2316),
  [2026-2034](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/package-manager.ts#L2026-L2034)).
  Temporary checkouts go to `tmp/extensions/` ([Scratch directories](#scratch-directories)).
  On macOS and Linux pi marks the npm root with extended attributes that tell Dropbox and the
  macOS file provider to skip it
  ([utils/paths.ts:169-184](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/utils/paths.ts#L169-L184)).
- `bin/`, holding the `fd` and `rg` binaries pi downloads and, for a managed install, the `pi`
  launcher script; legacy `tools/` held `fd` and `rg` before a migration moved them to `bin/`
  ([config.ts:659-666](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/config.ts#L659-L666);
  [migrations.ts:177-200](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/migrations.ts#L177-L200);
  [install.sh:1136-1197](https://pi.dev/install.sh)).
- `install/`, the managed install the pi.dev installer creates (unless `PI_LEGACY_INSTALL=1`):
  `managed-install.json` (`kind` `pi-managed-install`, `schemaVersion` 1, `layout` `releases-v1`,
  `entrypoint`), `current-version`, `staging/` and `releases/<version>/`, each release a full npm
  install of pi with its `node_modules`
  ([install.sh:830](https://pi.dev/install.sh), [912-913](https://pi.dev/install.sh),
  [1100-1115](https://pi.dev/install.sh),
  [1160-1187](https://pi.dev/install.sh), [1282](https://pi.dev/install.sh);
  [install.ps1:1130](https://pi.dev/install.ps1);
  [package-manager-cli.ts:58-82](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/package-manager-cli.ts#L58-L82),
  [131-140](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/package-manager-cli.ts#L131-L140)).
  The installer's Node.js runtime goes to `~/.local/share/pi-node` (or `$XDG_DATA_HOME/pi-node`) and
  on Windows `%LOCALAPPDATA%\pi-node`, outside the agent directory
  ([install.sh:434-437](https://pi.dev/install.sh); [install.ps1:612](https://pi.dev/install.ps1)).

Files guarded by the `proper-lockfile` library get a transient `<file>.lock` directory beside them
while a process holds the lock (`auth.json`, `models-store.json`, `mcp-auth.json`, `trust.json`,
`settings.json`)
([auth-storage.ts:80-93](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/auth-storage.ts#L80-L93);
[settings-manager.ts:7](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/settings-manager.ts#L7);
[trust-manager.ts:4](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/trust-manager.ts#L4)).

Opening an older-format session rewrites the whole file in the current format
([session-manager.ts:1088-1094](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/session-manager.ts#L1088-L1094)),
and loading a file whose last line has no newline appends one
([session-manager.ts:664-668](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/session-manager.ts#L664-L668)),
so resuming a session on the host changes its evidence.

#### Scratch directories

The one scratch directory under the home is `~/.pi/agent/tmp/extensions/`, mode 0700, where
`pi -e npm:<pkg>` and `pi -e git:<repo>` keep temporary package checkouts
([package-manager.ts:254-259](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/package-manager.ts#L254-L259),
[2316-2323](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/package-manager.ts#L2316-L2323));
it holds copies of published packages.

Everything else pi spills goes to the system temporary directory, outside the home, mode 0600,
through one helper that names files `<prefix>-<16 hex><extension>`
([utils/output-files.ts:14-34](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/utils/output-files.ts#L14-L34)):
truncated shell output as `pi-bash-*.log` and `pi-powershell-*.log`
([tools/bash.ts:424](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/tools/bash.ts#L424),
[tools/powershell.ts:46](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/tools/powershell.ts#L46);
`!command` output,
[bash-executor.ts:66](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/bash-executor.ts#L66)),
MCP resources as `pi-mcp-*`
([mcp/tools.ts:71](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/extensions/mcp/tools.ts#L71))
and codemode output as `pi-codemode-*`
([codemode/execute.ts:395](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/extensions/codemode/execute.ts#L395),
[424](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/extensions/codemode/execute.ts#L424));
the session names such a file in the tool result ([Message roles](#message-roles)). A pasted
clipboard image is saved as
`pi-clipboard-<uuid>.<ext>` and left there
([interactive-mode.ts:3234-3240](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/modes/interactive/interactive-mode.ts#L3234-L3240)).
`/share` and the external editor use temporary directories they remove
([session-share.ts:58](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/modes/interactive/session-share.ts#L58);
[external-editor.ts:15](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/modes/interactive/external-editor.ts#L15)).

#### Experimental stores of older builds

Builds up to 1.0.2 (reviewed at
[`2003871`](https://github.com/earendil-works/pi/commit/200387122ca450d6387f033949423114a270b96c))
wrote these with `PI_EXPERIMENTAL=1`; the reviewed commit writes none of them
([Format versions](#format-versions)):

- `~/.pi/agent/experimental/sessions/<id>/meta.json` and `session.sqlite`
  ([experimental/server.ts:69-71](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/experimental/server.ts#L69-L71);
  [experimental/session-catalog.ts:5-16](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/experimental/session-catalog.ts#L5-L16)).
- `~/.pi/agent/experimental/vacation-sessions/<sha256(cwd)[:24]>/<ms>-<uuid>/session.sqlite`
  ([experimental/vacation/sessions.ts:18-55](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/experimental/vacation/sessions.ts#L18-L55)).
- A server directory, `$PI_SERVER_DIR` or `~/.pi/server`, holding `default-server-id`, lock
  directories and Unix sockets
  ([experimental/server.ts:51-56](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/experimental/server.ts#L51-L56),
  [85-91](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/experimental/server.ts#L85-L91)).

### Credentials

pi uses no OS keychain: a grep of `packages/coding-agent/src` and `packages/ai/src` at the reviewed
commit for `keytar`, `keychain` and `libsecret` finds nothing. Every path below is the same
relative to the home on macOS, Linux and Windows.

- `.pi/agent/auth.json`, created mode 0600 in a 0700 directory: one object per provider,
  `{type:"api_key", key, env}` or `{type:"oauth", refresh, access, expires, ...}`
  ([auth-storage.ts:36-37](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/auth-storage.ts#L36-L37),
  [64-79](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/auth-storage.ts#L64-L79),
  [107](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/auth-storage.ts#L107);
  [packages/ai/src/auth/types.ts:13-37](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/ai/src/auth/types.ts#L13-L37)).
  pi writes it in place, with no backup, temporary or rename copy; its lock directory
  `auth.json.lock` holds no data.
- `.pi/agent/mcp-auth.json`: MCP client registrations, tokens and pending PKCE verifiers, keyed
  by server name and URL
  ([mcp/oauth.ts:126-148](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/extensions/mcp/oauth.ts#L126-L148)).
- `.pi/agent/models.json`: custom providers carry `apiKey` and `headers`
  ([model-config.ts:89-108](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/model-config.ts#L89-L108));
  a value is a literal, an environment reference, or `!command` whose output is used
  ([resolve-config-value.ts:138-150](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/resolve-config-value.ts#L138-L150)).
- `.pi/agent/mcp.json`, and `.pi/mcp.json` inside a project: MCP server `env` and `headers`, such
  as `Authorization: Bearer ...`
  ([mcp-servers.ts:85](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/mcp-servers.ts#L85),
  [153](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/mcp-servers.ts#L153);
  [mcp/config.ts:14-21](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/extensions/mcp/config.ts#L14-L21)).
- `.pi/agent/oauth.json`, the OAuth store of builds before `auth.json`, and
  `.pi/agent/oauth.json.migrated`, the same file renamed by the migration that copied it into
  `auth.json`
  ([migrations.ts:20-60](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/migrations.ts#L20-L60)).
- `.pi/agent/settings.json` mixes settings with a legacy `apiKeys` object; it is listed in
  [Secrets](#secrets).

`models-store.json` is not a credential file: it holds provider model catalogs, though pi writes
it through the same 0600 file backend as `auth.json`
([models-store.ts:46-55](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/models-store.ts#L46-L55)).
pi also reads, and does not write, `~/.config/gcloud/application_default_credentials.json`
([packages/ai/src/env-api-keys.ts:60-67](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/ai/src/env-api-keys.ts#L60-L67))
and the Hugging Face token under `~/.cache/huggingface/token`
([llama/huggingface.ts:51-55](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/extensions/llama/huggingface.ts#L51-L55));
they belong to those tools.

### Exclusions

Copied, not made by pi, and large:

- `.pi/agent/bin`: downloaded `fd` and `rg` binaries and the managed `pi` launcher.
- `.pi/agent/tools/fd*` and `.pi/agent/tools/rg*`: the same binaries (`fd`, `rg`, `fd.exe`,
  `rg.exe`) where an older build left them in legacy `tools/` and no later start moved them
  ([migrations.ts:177-200](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/migrations.ts#L177-L200)).
- `.pi/agent/install`: the managed install, a full copy of pi per release.
- `.pi/agent/npm/node_modules`: installed npm packages; the sources stay recorded in `settings.json`
  `packages` and `npm/package.json`
  ([package-manager.ts:2207-2211](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/package-manager.ts#L2207-L2211)).
- `.pi/agent/git/*/*/node_modules` and `.pi/agent/git/*/*/.git` (the dependencies and history of
  each `git/<host>/<path>` checkout; a path may have more segments). The checkout's own files are
  extension code that ran with the user's rights.
- `.pi/agent/tmp`: temporary package checkouts ([Scratch directories](#scratch-directories)).
- The same at project scope: `.pi/npm/node_modules`, `.pi/git/*/*/node_modules` and
  `.pi/git/*/*/.git`
  ([package-manager.ts:2229-2230](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/package-manager.ts#L2229-L2230),
  [2310-2311](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/package-manager.ts#L2310-L2311)).

- `.pi/server/*.sock`: the Unix sockets older builds' experimental server left in `~/.pi/server`,
  `<server id>.sock`, `control-<server id>.sock` and `server-<server id>-<nonce>.sock`
  ([packages/client/src/unix.ts:16](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/client/src/unix.ts#L16);
  [experimental/server.ts:582-585](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/experimental/server.ts#L582-L585)),
  which cannot be copied.

Session files stay whole: images
are inline base64 and can be large, but they are the transcript.

### Project-local files

`<cwd>/.pi/` holds `settings.json`
([settings-manager.ts:194](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/settings-manager.ts#L194)),
`mcp.json`, `extensions/`, `skills/`, `prompts/`, `themes/`, `SYSTEM.md` and `APPEND_SYSTEM.md`
([trust-manager.ts:30-39](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/trust-manager.ts#L30-L39);
[resource-loader.ts:1028-1032](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/resource-loader.ts#L1028-L1032),
[1246](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/resource-loader.ts#L1246),
[1260](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/resource-loader.ts#L1260)),
and project-scope packages in `.pi/npm` and `.pi/git`; the vendor's
[Project `.pi` directory table](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/docs/configuration.md#project-pi-directory)
lists them. These load only once the folder is trusted ([Project trust](#project-trust)), and
project extensions then run with the user's rights, so they are evidence. little-coder adds
`.pi/approved-plan.md`
([Project-local files in `little-coder.md`](little-coder.md#project-local-files)).

Context files are read from the cwd upwards: `AGENTS.override.md`, `AGENTS.md`, `AGENTS.MD`,
`CLAUDE.md` and `CLAUDE.MD`
([resource-loader.ts:211-213](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/resource-loader.ts#L211-L213)),
and `.agents/skills` up to the git root
([package-manager.ts:490-503](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/package-manager.ts#L490-L503)).

Exports and reports land in the cwd by default:

- `/export` HTML as `pi-session-<session file basename>.html`
  ([export-html/index.ts:276-279](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/export-html/index.ts#L276-L279)),
  holding the session in base64 ([HTML export](#html-export)).
- `/export` JSONL as `session-<ISO time>.jsonl`, the time's `:` and `.` replaced by `-`, so the
  name matches `session-[0-9][0-9][0-9][0-9]-*.jsonl`
  ([session-export.ts:32-45](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/session-export.ts#L32-L45)).
- A `/bug` report exported as `pi-bug-report-<id>.zip`, `<id>` the report id
  ([modes/interactive/bug-report.ts:202](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/modes/interactive/bug-report.ts#L202);
  [core/bug-report.ts:249-280](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/bug-report.ts#L249-L280)),
  holding `report.json`, `diagnostics.json` and, when chosen, `session.jsonl` and `summary.md`.

pi keeps no per-project database.

### Where the project path is recorded

- Session header (line 1 of each file), key `cwd`
  ([session-manager.ts:43-50](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/session-manager.ts#L43-L50));
  the session directory name is a lossy copy.
- `trust.json`: every key is an absolute project path ([Project trust](#project-trust)).
- `crashes.json`: `cwd` and `sessionFile` per record
  ([Debug log and crash log](#debug-log-and-crash-log)).
- `experimental/sessions/*/meta.json` of older builds: `cwd`
  ([session-catalog.ts:6-13](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/experimental/session-catalog.ts#L6-L13)).

## Parsing

### Transcript stores

- `.pi/agent/sessions/--<encoded cwd>--/<ISO time, : and . as ->_<session id>.jsonl`, for example
  `2026-10-30T14-05-11-204Z_019a3c5e-2f71-7b40-9d1e-5b6c7a8e9f01.jsonl`, named as
  [Per-user storage](#per-user-storage) says. One file per session; a fork or a branch gets a new
  file
  ([session-manager.ts:1077-1080](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/session-manager.ts#L1077-L1080)).
  The vendor's
  [Session File Format page](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/docs/session-format.md#file-location)
  describes the same layout.
- With `--session-dir`, `PI_CODING_AGENT_SESSION_DIR` or the `sessionDir` setting the files sit
  flat in that directory, anywhere on disk.
- Legacy: `.pi/agent/*.jsonl`, the v0.30.0 location, moved on the next start.
- Copies in project directories: `session-<time>.jsonl` (same format, one branch, re-parented into
  a chain) and `pi-session-*.html` ([Project-local files](#project-local-files)).
- The experimental durable sessions of older builds are SQLite ([Other stores](#other-stores)).

#### What survives a delete

The `/resume` picker deletes a session with Ctrl+D: pi runs the `trash` command and falls back to
unlinking the file
([session-selector.ts:640-675](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/modes/interactive/components/session-selector.ts#L640-L675);
the vendor's
[Deleting Sessions section](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/docs/session-format.md#deleting-sessions)),
so the file is gone or sits in the user's trash, with any `pi.bug-report` entry inside it. No index
or log repeats that entry, and pi offers no way to delete the remote copy that `/share` or a bug
report uploaded. pi keeps no per-prompt history file: a grep of `packages/coding-agent/src` at the
reviewed commit for `history.jsonl` and `promptHistory` finds none.

What remains after a delete:

- `pi-debug.log`, whose last `/debug` snapshot can hold one session's messages
  ([Debug log and crash log](#debug-log-and-crash-log)).
- `crashes.json`, with each recent crash's `sessionFile` and `cwd`.
- `pi-tui-crash.log`, the screen lines of the last terminal UI crash.
- Forks and branches: a child file repeats the parent's entries ([Joins and timestamps](#joins-and-timestamps)).
- Exports in project directories: a `pi-session-*.html` holds every entry of the session, a
  `session-*.jsonl` one branch, and a `pi-bug-report-*.zip` a `session.jsonl` when the transcript
  was included. The picker's delete does not touch them.
- little-coder's checkpoints, named after the session file
  ([Checkpoints in
  `little-coder.md`](little-coder.md#checkpoints-little-codercheckpointssession-file-name)).

pi writes no deletion marker. The analyzer reads the session files, `trust.json`, `pi-debug.log`
as a secondary source, whose messages it keeps when no session file holds them, and
`crashes.json`, whose rows name a deleted session's file and directory (see
[Secondary sources](../analyzer/docs/output.md#secondary-sources)).

### Record schema

Every line is one JSON object; pi skips a line that does not parse
([session-manager.ts:616-624](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/session-manager.ts#L616-L624))
and ignores a file whose first parsed record is not a header with a string `id`
([664-667](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/session-manager.ts#L664-L667)).
Types are from
[session-manager.ts:41-198](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/session-manager.ts#L41-L198),
[packages/ai/src/types.ts:422-657](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/ai/src/types.ts#L422-L657)
and
[messages.ts:28-52](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/messages.ts#L28-L52).

#### Header

Line 1
([session-manager.ts:43-50](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/session-manager.ts#L43-L50),
[1060-1076](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/session-manager.ts#L1060-L1076)).
The session id is a UUIDv7
([264-266](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/session-manager.ts#L264-L266))
unless given with `--session-id`, which accepts letters, digits, `.`, `_` and `-`
([main.ts:448](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/main.ts#L448),
[session-manager.ts:268-274](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/session-manager.ts#L268-L274)).
The header names no producer and no pi version.

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `type` | string | `session` | |
| `version` | number | `2`, `3` | |
| `id` | string | | |
| `timestamp` | string | | |
| `cwd` | string | | |
| `parentSession` | string | | |

`version` is absent in a version 1 file; `timestamp` is ISO 8601 UTC with milliseconds and `Z`;
`parentSession` is the absolute path of the session file this one was forked or branched from.

#### Entries

Every later line has `type`, `id` (8 hex characters, collision-checked,
[276-284](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/session-manager.ts#L276-L284)),
`parentId` (null at a root) and `timestamp`, the ISO time of the append
([57-62](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/session-manager.ts#L57-L62),
[1214-1223](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/session-manager.ts#L1214-L1223)).
An entry whose write fails is dropped from memory, so no later entry names it as parent
([1191-1206](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/session-manager.ts#L1191-L1206)).

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `type` | string | `message`, `thinking_level_change`, `model_change`, `usage`, `compaction`, `branch_summary`, `custom`, `custom_message`, `context_edit`, `label`, `session_info` | |
| `id` | string | | |
| `parentId` | string or null | | |
| `timestamp` | string | | |
| `message` | object | | |
| `thinkingLevel` | string | | |
| `provider` | string | | |
| `modelId` | string | | |
| `kind` | string | | |
| `model` | string | | |
| `usage` | object | | |
| `note` | string | | |
| `summary` | string | | |
| `firstKeptEntryId` | string | | |
| `tokensBefore` | number | | |
| `details` | object | | |
| `fromHook` | boolean | | |
| `systemMessage` | object | | |
| `fromId` | string | | |
| `customType` | string | | |
| `data` | object | | |
| `content` | string or array | | |
| `display` | boolean | | |
| `targetId` | string | | |
| `replacement` | object or null | | |
| `label` | string | | |
| `name` | string | | |

Which keys each type carries:

| `type` | Keys |
| --- | --- |
| `message` | `message`: one of the roles in [Message roles](#message-roles) |
| `thinking_level_change` | `thinkingLevel` |
| `model_change` | `provider`, `modelId` |
| `usage` | `kind` (any string, for example `cache_warm`), `provider`, `model`, `usage`, `note` |
| `compaction` | `summary`, `firstKeptEntryId`, `tokensBefore`, `details`, `usage`, `fromHook`, `systemMessage` |
| `branch_summary` | `fromId`, `summary`, `details`, `usage`, `fromHook` |
| `custom` | `customType`, `data`: extension state, not in model context |
| `custom_message` | `customType`, `content` (string or text and image parts), `display`, `details`: extension-injected context |
| `context_edit` | `targetId`, `replacement` (`{content}`, or null to omit the target from context) |
| `label` | `targetId`, `label` (absent clears it) |
| `session_info` | `name`, the user-set session title |

#### Bug-report entry

`/bug` appends a `custom` entry with `customType` `pi.bug-report` once a report is uploaded or
exported
([modes/interactive/bug-report.ts:213-227](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/modes/interactive/bug-report.ts#L213-L227),
[core/bug-report.ts:17](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/bug-report.ts#L17),
[235-243](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/bug-report.ts#L235-L243)).
Its `data`:

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `id` | string | | |
| `createdAt` | string | | |
| `hint` | string or null | | |
| `sessionIncluded` | boolean | | |
| `summaryIncluded` | boolean | | |
| `delivery` | string | `upload`, `zip` | |
| `path` | string | | |

`id` is the report id, a UUIDv7 made locally
([core/bug-report.ts:155-160](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/bug-report.ts#L155-L160));
`hint` is the user's description; `path`, for `zip` only, is the archive in the working directory
([Project-local files](#project-local-files)). The entry's own `timestamp` is the append time
([session-manager.ts:1300-1311](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/session-manager.ts#L1300-L1311)).
A session never reported has no such entry.

#### Message roles

`message.role` and the keys of each role; every message has `timestamp` in Unix milliseconds
([types.ts:561-583](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/ai/src/types.ts#L561-L583),
[585-616](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/ai/src/types.ts#L585-L616),
[640-657](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/ai/src/types.ts#L640-L657);
[messages.ts:28-52](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/messages.ts#L28-L52)).
The agent persists `system`, `user`, `assistant` and `toolResult` messages at their end, writes an
extension's `custom` message as a `custom_message` entry
([agent-session.ts:1163-1184](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/agent-session.ts#L1163-L1184)),
and appends `bashExecution` messages for the user's `!command`
([agent-session.ts:3974-3980](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/agent-session.ts#L3974-L3980)).
A `message` entry with role `custom` comes from a version 2 file's `hookMessage`
([session-manager.ts:316-334](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/session-manager.ts#L316-L334)).
The roles `branchSummary` and `compactionSummary` are built in memory from the `branch_summary`
and `compaction` entries
([messages.ts:100-121](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/messages.ts#L100-L121))
and appear only in the debug log.

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `role` | string | `system`, `user`, `assistant`, `toolResult`, `bashExecution`, `custom`, `branchSummary`, `compactionSummary` | |
| `timestamp` | number | | |
| `content` | string or array | | |
| `sections` | object | | |
| `toolsAdded` | array | | |
| `toolsRemoved` | array | | |
| `api` | string | | |
| `provider` | string | | |
| `model` | string | | |
| `responseModel` | string | | |
| `responseId` | string | | |
| `providerThinkingLevel` | string | | |
| `thinkingLevel` | string | | |
| `diagnostics` | array | | |
| `usage` | object | | |
| `stopReason` | string | `pending`, `stop`, `length`, `toolUse`, `error`, `aborted`, `deferred` | |
| `deferred` | object | | |
| `errorMessage` | string | | |
| `rawStopReason` | string | | |
| `endTurn` | boolean | | |
| `durationMs` | number | | |
| `toolCallId` | string | | |
| `toolName` | string | | |
| `details` | object | | |
| `nestedCalls` | object | | |
| `isError` | boolean | | |
| `command` | string | | |
| `output` | string | | |
| `exitCode` | number | | |
| `cancelled` | boolean | | |
| `truncated` | boolean | | |
| `fullOutputPath` | string | | |
| `excludeFromContext` | boolean | | |
| `customType` | string | | |
| `display` | boolean | | |
| `summary` | string | | |
| `fromId` | string or null | | |
| `tokensBefore` | number | | |

Per role:

- `system`: `content` (the system prompt on the first, added instructions later), `sections`
  (named prompt sections, a null value removes one), `toolsAdded`, `toolsRemoved`.
- `user`: `content`, a string or an array of text and image parts; images are inline base64.
- `assistant`: `content` (text, thinking and tool-call parts), `api`, `provider`, `model` (the
  requested model), `responseModel` (the model the provider reported, which may equal `model`),
  `responseId`, `providerThinkingLevel`, `thinkingLevel`, `diagnostics`, `usage`, `stopReason`,
  `deferred`, `errorMessage`, `rawStopReason`, `endTurn`; `timestamp` is when the request started
  and `durationMs` how long the response took.
- `toolResult`: `toolCallId`, `toolName`, `content` (text and image parts), `details`
  (tool-specific, such as the edit tool's `diff` and `patch` or the shell tools' `fullOutputPath`,
  [tools/edit.ts:68-75](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/tools/edit.ts#L68-L75),
  [tools/bash.ts:66-69](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/tools/bash.ts#L66-L69)),
  `usage`, `nestedCalls`, `isError`, `durationMs`. An extension that blocks a call makes the agent
  loop record an error `toolResult` whose text is the extension's reason, or
  `Tool execution was blocked` when it gives none
  ([packages/agent/src/agent-loop.ts:755-763](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/agent/src/agent-loop.ts#L755-L763),
  [922-927](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/agent/src/agent-loop.ts#L922-L927));
  little-coder's [permission gate](little-coder.md#permission-gate) is such an extension.
- `bashExecution`: `command`, `output`, `exitCode`, `cancelled`, `truncated`, `fullOutputPath`,
  `excludeFromContext` (set for a `!!` command).
- `custom`: `customType`, `content`, `display`, `details`.
- `branchSummary` (`summary`, `fromId`) and `compactionSummary` (`summary`, `tokensBefore`).

#### Content parts

The parts of `content`
([types.ts:422-457](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/ai/src/types.ts#L422-L457)):

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
| `namespace` | string | | |
| `argumentsError` | string | | |

A `text` part has `text` and `textSignature`; a `thinking` part `thinking`, `thinkingSignature` and
`redacted`; an `image` part `data` (base64) and `mimeType`; a `toolCall` part `id`, `name`,
`arguments`, `thoughtSignature`, `namespace` and `argumentsError` (set, with `arguments` empty, when
the model's arguments were not a JSON object).

#### Usage

`usage` on assistant messages, tool results, `usage`, `compaction` and `branch_summary` entries
([types.ts:459-487](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/ai/src/types.ts#L459-L487)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `input` | number | | |
| `output` | number | | |
| `cacheRead` | number | | |
| `cacheWrite` | number | | |
| `cacheWrite1h` | number | | |
| `reasoning` | number | | |
| `totalTokens` | number | | |
| `cost` | object | | |
| `totalFromProvider` | boolean | | |

`cost` holds `input`, `output`, `cacheRead`, `cacheWrite` and `total`, and on the reviewed commit
`totalFromProvider`, true when `total` is the provider's billed cost.

#### Nested calls

`nestedCalls` on a tool result records the calls a codemode script made: `calls` and `complete`,
each call with the keys below; results are not recorded
([types.ts:619-638](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/ai/src/types.ts#L619-L638)).

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `calls` | array | | |
| `complete` | boolean | | |
| `id` | string | | |
| `name` | string | | |
| `arguments` | object | | |
| `argumentsBytes` | number | | |
| `status` | string | `ok`, `error`, `unfinished` | |
| `durationMs` | number | | |
| `error` | string | | |

#### Tool arguments

Built-in tool arguments:

- `bash` and `powershell`: `command`, `timeout`
  ([tools/bash.ts:40-43](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/tools/bash.ts#L40-L43);
  [tools/powershell.ts:39-47](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/tools/powershell.ts#L39-L47)).
- `read`: `path`, `offset`, `limit`
  ([tools/read.ts:14-18](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/tools/read.ts#L14-L18)).
- `write`: `path`, `content`
  ([tools/write.ts:11-14](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/tools/write.ts#L11-L14)).
- `edit`: `path`, `edits` (`oldText`, `newText`)
  ([tools/edit.ts:21-37](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/tools/edit.ts#L21-L37)).
- `grep`: `pattern`, `path`, `glob`
  ([tools/grep.ts:21-24](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/tools/grep.ts#L21-L24)).
- `find`: `pattern`, `path`
  ([tools/find.ts:26-30](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/tools/find.ts#L26-L30)).
- `ls`: `path`
  ([tools/ls.ts:11-12](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/tools/ls.ts#L11-L12)).
- `codemode`: `code`
  ([codemode/tool.ts:78-82](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/extensions/codemode/tool.ts#L78-L82)).
- `tool_search`: `query`, `limit`
  ([tool-search/tool.ts:20](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/extensions/tool-search/tool.ts#L20),
  [252-257](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/extensions/tool-search/tool.ts#L252-L257)).

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `command` | string | | |
| `timeout` | number | | |
| `path` | string | | |
| `offset` | number | | |
| `limit` | number | | |
| `content` | string | | |
| `edits` | array | | |
| `oldText` | string | | |
| `newText` | string | | |
| `pattern` | string | | |
| `glob` | string | | |
| `code` | string | | |
| `query` | string | | |

### Joins and timestamps

- Session: header `id`, also the file-name suffix. Project: header `cwd`.
- Tool use to result: `toolCall.id` equals `toolResult.toolCallId`.
- Tree: `parentId` points at `id`; the file holds every branch, and the active path is the walk up
  from the last entry. `label` and `context_edit` point at `targetId`; `compaction.firstKeptEntryId`
  marks the cut.
- Fork and branch: the child header's `parentSession` is the parent file's path. A fork copies every
  parent entry verbatim, ids and timestamps included
  ([session-manager.ts:1848-1880](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/session-manager.ts#L1848-L1880));
  a branch copies the path to the chosen entry
  ([1642-1691](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/session-manager.ts#L1642-L1691)).
  The same entry therefore appears in several files.
- Time basis: entry `timestamp` is ISO with milliseconds at append; `message.timestamp` is Unix
  milliseconds. An assistant message's `timestamp` is set when its stream starts
  ([packages/ai/src/api/anthropic-messages.ts:587-602](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/ai/src/api/anthropic-messages.ts#L587-L602)),
  while the entry is appended at `message_end`
  ([agent-session.ts:1163-1181](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/agent-session.ts#L1163-L1181)),
  so the two times of one assistant message differ by the length of the reply.
- little-coder checkpoints: a directory named after the session file
  ([Checkpoints in
  `little-coder.md`](little-coder.md#checkpoints-little-codercheckpointssession-file-name)).
- Sub-agents: pi has none built in; little-coder's leave no session file
  ([Transcript stores in `little-coder.md`](little-coder.md#transcript-stores)).

### Other stores

The main format has no SQLite or binary store.

#### Debug log and crash log

`pi-debug.log` in the agent directory
([config.ts:679-681](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/config.ts#L679-L681))
is rewritten by each `/debug`: a first line `Debug output at <ISO time>`, the terminal size, the
rendered screen lines, then the marker `=== Agent messages (JSONL) ===` and the open session's
messages, one JSON object per line, with no session id
([interactive-mode.ts:7043-7067](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/modes/interactive/interactive-mode.ts#L7043-L7067)),
so it can hold a deleted session's turns. Each line is a message as in
[Message roles](#message-roles), with its own `timestamp` in Unix milliseconds and no entry `id` or
`timestamp`; the in-memory roles `branchSummary` and `compactionSummary` can appear there.

`crashes.json` is a JSON array of the last five crashes; a crash older than seven days is not
announced
([crash-log.ts:7-23](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/crash-log.ts#L7-L23),
[124-145](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/crash-log.ts#L124-L145),
[147-160](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/crash-log.ts#L147-L160)).
A bug report that carried crashes deletes the file
([crash-log.ts:164-170](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/crash-log.ts#L164-L170);
[modes/interactive/bug-report.ts:226](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/modes/interactive/bug-report.ts#L226)).

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `timestamp` | string | | |
| `version` | string | | |
| `kind` | string | `uncaught_exception`, `fatal_error` | |
| `message` | string | | |
| `stack` | string or null | | |
| `sessionFile` | string or null | | |
| `cwd` | string | | |
| `notified` | boolean | | |

#### HTML export

A `pi-session-*.html` export embeds the session as base64-encoded JSON with `header`, `entries`
(every entry of every branch), `leafId`, `systemPrompt`, `tools` and `renderedTools`
([export-html/index.ts:130-138](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/export-html/index.ts#L130-L138),
[161](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/export-html/index.ts#L161),
[265-272](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/export-html/index.ts#L265-L272)),
so it is a full copy of the session file's entries at export time.

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `header` | object | | |
| `entries` | array | | |
| `leafId` | string or null | | |
| `systemPrompt` | string | | |
| `tools` | array | | |
| `renderedTools` | object | | |

#### Project trust

`~/.pi/agent/trust.json` is an object whose keys are absolute project paths and whose values are
`true` or `false`; pi drops a leading byte-order mark before parsing it, as it does for
`settings.json`
([trust-manager.ts:113](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/trust-manager.ts#L113);
[settings-manager.ts:380](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/settings-manager.ts#L380)).
A `null` decision deletes the key
([trust-manager.ts:28](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/trust-manager.ts#L28),
[221-223](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/trust-manager.ts#L221-L223),
[239-252](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/trust-manager.ts#L239-L252)).
A decision applies to the folder and everything below it: pi walks up from the cwd to the nearest
key
([trust-manager.ts:50-62](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/trust-manager.ts#L50-L62)).
A decision is saved from the trust prompt, or by an extension answering the `project_trust` event
with `remember`
([project-trust.ts:56-71](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/project-trust.ts#L56-L71)).
When `trust.json` has no entry, `defaultProjectTrust` in the global `settings.json` (`ask`, the
default, `always` or `never`) decides
([settings-schema.ts:271-276](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/settings-schema.ts#L271-L276),
[settings-defaults.ts:30](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/settings-defaults.ts#L30),
[project-trust.ts:73-90](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/project-trust.ts#L73-L90));
the vendor's
[How Pi chooses a trust decision section](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/docs/security.md#how-pi-chooses-a-trust-decision)
gives the same order. pi has no per-call tool approval of its own: `--approve` (`-a`) and
`--no-approve` (`-na`) override project trust for one run
([cli/args.ts:241-244](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/cli/args.ts#L241-L244),
[341-342](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/cli/args.ts#L341-L342))
and write nothing.

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `defaultProjectTrust` | string | `ask`, `always`, `never` | |
| `sessionShareDestination` | string | `auto`, `gist`, `radius` | |

#### Experimental durable sessions

Builds up to 1.0.2 kept experimental sessions ([Experimental stores of older
builds](#experimental-stores-of-older-builds)) in `session.sqlite`, in WAL mode
([packages/durable/src/storage/sqlite/node.ts:181-193](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/durable/src/storage/sqlite/node.ts#L181-L193)),
with tables `durable_metadata`, `record_ids`, `conversations`, `entries`, `tasks`, `submissions`,
`documents` and `document_revisions`
([migrations.ts:10-82](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/durable/src/storage/sqlite/migrations.ts#L10-L82)).
Not parsed; the `entries` table, which would hold the turns:

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `id` | number | | |
| `conversation_id` | number | | |
| `head` | number or null | | |
| `commit_seq` | number | | |
| `record` | string | | |

Each session directory's `meta.json`
([session-catalog.ts:6-13](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/experimental/session-catalog.ts#L6-L13)):

| Key | Type | Values | Since |
| --- | --- | --- | --- |
| `id` | string | | |
| `createdAt` | number | | |
| `cwd` | string | | |
| `path` | string | | |

### Format versions

Header `version`: 1 (no field, no ids), 2 (`id` and `parentId` added), 3 (the `hookMessage` role
renamed `custom`)
([session-manager.ts:41](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/session-manager.ts#L41),
[286-345](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/session-manager.ts#L286-L345);
the vendor's
[Session Version section](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/docs/session-format.md#session-version)).
pi rewrites old files on open, so a collected version 1 or 2 file means nobody resumed it. Within
version 3 keys are added without a bump; a parser must ignore unknown types and keys.

- Tag `v0.83.0`
  ([`845d6ff`](https://github.com/earendil-works/pi/commit/845d6ff1f6643aba440341cce877ce1c43ebbc39)),
  which little-coder pins, persists only `user`, `assistant` and `toolResult` messages
  ([agent-session.ts:635-641](https://github.com/earendil-works/pi/blob/845d6ff1f6643aba440341cce877ce1c43ebbc39/packages/coding-agent/src/core/agent-session.ts#L635-L641)),
  has no `deferred` stop reason
  ([types.ts:391](https://github.com/earendil-works/pi/blob/845d6ff1f6643aba440341cce877ce1c43ebbc39/packages/ai/src/types.ts#L391))
  and writes no `usage` or `context_edit` entry and no `compaction.systemMessage`; `system`
  messages are persisted since 0.86.0, which moved the system prompt and tools into transcript
  system messages
  ([packages/ai/CHANGELOG.md:312-317](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/ai/CHANGELOG.md#L312-L317)). Its assistant
  messages lack `providerThinkingLevel`, `thinkingLevel`, `deferred`, `endTurn` and `durationMs`,
  its tool calls `namespace` and `argumentsError`, and its tool results carry `addedToolNames` in
  place of `nestedCalls` and `durationMs`
  ([types.ts:360-430](https://github.com/earendil-works/pi/blob/845d6ff1f6643aba440341cce877ce1c43ebbc39/packages/ai/src/types.ts#L360-L430)).
  It writes no `crashes.json`, `mcp.json`, `mcp-auth.json` or `mcp.log`, has no `/bug` and no
  Radius `/share`, and has no `powershell` tool. Its terminal UI wrote redraw lines into
  `pi-debug.log` itself while `PI_DEBUG_REDRAW=1` and its crash screen to `pi-crash.log`
  ([tui.ts:332](https://github.com/earendil-works/pi/blob/845d6ff1f6643aba440341cce877ce1c43ebbc39/packages/tui/src/tui.ts#L332),
  [1331-1337](https://github.com/earendil-works/pi/blob/845d6ff1f6643aba440341cce877ce1c43ebbc39/packages/tui/src/tui.ts#L1331-L1337),
  [1527](https://github.com/earendil-works/pi/blob/845d6ff1f6643aba440341cce877ce1c43ebbc39/packages/tui/src/tui.ts#L1527)),
  so its `pi-debug.log` can mix `[<ISO time>] fullRender:` lines with a `/debug` snapshot, which
  it writes the same way
  ([config.ts:563-565](https://github.com/earendil-works/pi/blob/845d6ff1f6643aba440341cce877ce1c43ebbc39/packages/coding-agent/src/config.ts#L563-L565),
  [interactive-mode.ts:5886-5910](https://github.com/earendil-works/pi/blob/845d6ff1f6643aba440341cce877ce1c43ebbc39/packages/coding-agent/src/modes/interactive/interactive-mode.ts#L5886-L5910)).
  The redraw and crash logs became `pi-tui-debug.log` and `pi-tui-crash.log`, changed between
  0.83.0 and 1.0.2. `trust.json` is written the same way
  ([trust-manager.ts:212](https://github.com/earendil-works/pi/blob/845d6ff1f6643aba440341cce877ce1c43ebbc39/packages/coding-agent/src/core/trust-manager.ts#L212)),
  and `/share` has only the gist path
  ([interactive-mode.ts:5523-5608](https://github.com/earendil-works/pi/blob/845d6ff1f6643aba440341cce877ce1c43ebbc39/packages/coding-agent/src/modes/interactive/interactive-mode.ts#L5523-L5608)).
- 1.0.2
  ([`2003871`](https://github.com/earendil-works/pi/commit/200387122ca450d6387f033949423114a270b96c))
  writes the experimental stores of
  [Experimental durable sessions](#experimental-durable-sessions) with `PI_EXPERIMENTAL=1`, and its
  experimental server opened a live relay through Radius, `/v1/session-relays/<server id>/connect`,
  whose only file is `.pi/server/default-server-id` in the home
  ([experimental/server.ts:609-615](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/experimental/server.ts#L609-L615),
  [76-112](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/experimental/server.ts#L76-L112);
  [experimental/radius-relay.ts:619-623](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/experimental/radius-relay.ts#L619-L623)).
  Its assistant messages and tool results have no `durationMs` and its tool calls no
  `argumentsError`; both changed between 1.0.2 and 1.3.0.
- 1.3.0
  ([`5c1736f`](https://github.com/earendil-works/pi/commit/5c1736f82d201d09664642ab0d88147da94b450e))
  no longer has the experimental server or the `vacation` mode: `packages/server`,
  `packages/client` and `packages/coding-agent/src/experimental/server.ts` are gone,
  `src/experimental/source-resolver.ts`, a source-loading helper, is all that is left of
  `src/experimental/` in the CLI, as at the reviewed commit, `coding-agent` no longer depends on
  `pi-durable` (its `package.json` names no `durable` package), and `PI_EXPERIMENTAL`
  ([experimental.ts:1-3](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/experimental.ts#L1-L3))
  is read only by the footer and the startup screen
  ([footer.ts:213](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/modes/interactive/components/footer.ts#L213),
  [cli/startup-ui.ts:141](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/cli/startup-ui.ts#L141)).
  The experimental stores exist only where 1.0.2 or older ran.
- Tag `v1.4.0`
  ([`4a08025`](https://github.com/earendil-works/pi/commit/4a080252f06caf63a525d7873c5d94d3c23887e0))
  differs from the reviewed commit in what it writes as follows: its usage `cost` has no
  `totalFromProvider`; its type says `responseModel` is set only when it differs from `model`, where
  the reviewed commit's says it may equal `model`
  ([types.ts:584](https://github.com/earendil-works/pi/blob/4a080252f06caf63a525d7873c5d94d3c23887e0/packages/ai/src/types.ts#L584);
  [types.ts:485](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/ai/src/types.ts#L485),
  [591](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/ai/src/types.ts#L591));
  its `settings.json` has no `extensionChangelogVersions` or `selfUpdateCommand`
  ([settings-schema.ts:209-213](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/settings-schema.ts#L209-L213),
  [286-292](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/settings-schema.ts#L286-L292));
  and its `trust.json` compares keys case-sensitively, so on Windows one folder can hold several
  keys differing in case, which the reviewed commit replaces with one on its next save
  ([trust-manager.ts:45-48](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/trust-manager.ts#L45-L48),
  [239-252](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/trust-manager.ts#L239-L252)).
  The session writer, the bug-report entry and the crash log are otherwise the same.

### Secrets

- Tool arguments and outputs are stored verbatim (`cat .env`, `write` contents, shell output);
  `system` messages carry the full system prompt, including context-file and skill text.
- `thinkingSignature`, `thoughtSignature` and `textSignature` are opaque provider data; redacted
  thinking is encrypted there.
- `.pi/agent/settings.json`: a legacy `apiKeys` object of provider keys survives when `auth.json`
  already existed at migration time, since the migration runs only when `auth.json` is absent
  ([migrations.ts:27-56](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/migrations.ts#L27-L56));
  `httpProxy` is a proxy URL, which may carry a user name and password
  ([settings-schema.ts:406-408](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/settings-schema.ts#L406-L408));
  `deviceId` and `trackingId` are installation identities. `deviceId` is sent to OpenAI's ChatGPT
  sign-in as the agent host id
  ([packages/ai/src/auth/oauth/openai-chatgpt.ts:226-237](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/ai/src/auth/oauth/openai-chatgpt.ts#L226-L237));
  a bug report drops both identities from the settings it attaches
  ([core/bug-report.ts:61-64](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/bug-report.ts#L61-L64)).
- Session files hold no credential; credentials are in the files [Credentials](#credentials)
  names.

### Parser plan

The parser reads `.pi/agent/sessions/*/*.jsonl` and `.pi/agent/*.jsonl`, takes line 1 as the
header and skips files whose first record is not `type` `session`. Files in a custom session
directory elsewhere on disk are not read; they would be recognised by that header shape. It also
reads `.pi/agent/trust.json`, `.pi/agent/settings.json`, `.pi/agent/pi-debug.log`,
`.pi/agent/crashes.json` and `.pi/agent/experimental/sessions/*/meta.json`.

| Column | Source |
| --- | --- |
| timestamp_utc | entry `timestamp`; fall back to `message.timestamp` (ms) |
| session_id | header `id` |
| project_path | header `cwd` |
| git_branch | empty (not recorded) |
| turn_type | `user`; `assistant` text parts; `toolCall` to tool_use; `toolResult` to tool_result; `thinking` to thinking; `bashExecution` to tool_use and tool_result; `system` role, `custom` role, `custom_message`, `model_change`, `thinking_level_change`, `compaction`, `branch_summary`, `context_edit`, `session_info` to system; `custom` entries other than `pi.bug-report`, `label` and `usage` skipped |
| model | assistant `responseModel`, else `model`; `model_change.modelId` |
| tool_name | `toolCall.name`, `toolResult.toolName`; `bash` for `bashExecution` |
| tool_use_id | `toolCall.id` and `toolResult.toolCallId`; entry `id` for `bashExecution` |
| text | text parts joined; tool_use: `command`, `path`, `pattern`, `code` or `query`, else the arguments as JSON (the shared reader also tries `file_path` and `url`, which pi's own tools do not use); tool_result: text parts; compaction and branch summary: `summary` |

The `system` rows other than those of the subsections below, `<...>` a record value:

- the header, at its `timestamp`:
  `session start: version=<version, 1 when absent> cwd=<cwd>`, ending ` parentSession=<path>`
  for a fork or branch;
- `fork: <N> entries copied from the parent session <path> omitted; they are in the parent's rows`,
  after it, when entries were dropped (see
  [Past-use rows and secondary sources](#past-use-rows-and-secondary-sources));
- `thinking_level_change: <thinkingLevel>`, `compaction: tokensBefore=<tokensBefore>: <summary>`,
  `branch_summary from <fromId>: <summary>`, `custom_message <customType>: <content text>`,
  `context_edit target=<targetId>: <replacement text>` (`omitted from context` for a null
  replacement) and `session_info: name=<name>`;
- `system prompt: <content text>`, `custom <customType>: <content text>`,
  `branchSummary: <summary>` and `compactionSummary: <summary>` for those message roles;
- `assistant stopReason=<stopReason>: <errorMessage>` after an assistant message whose
  `stopReason` is `error` or `aborted`;
- `experimental durable session created: cwd=<cwd> (transcript in session.sqlite, not parsed)`
  per older-build `meta.json`, at its `createdAt`, with the session id its directory names.

An error `toolResult` row's text starts `[error] `; a `bashExecution` result row ends with
`[exit <exitCode>, cancelled, truncated]`, naming only what applies.

Not read: `mcp.log`, `mcp.log.1`, `pi-tui-crash.log`, `pi-tui-debug.log`, `models-store.json`,
`keybindings.json`, the project exports `pi-session-*.html` and `session-*.jsonl`, and
`pi-bug-report-*.zip`.

#### Permission rows

No entry type or message role records a permission decision. The standing state is
`.pi/agent/trust.json` ([Project trust](#project-trust)): one `system` row per key,
`permission setting: <path>=<value> (from .pi/agent/trust.json, file mtime)`, with `project_path`
the key, `session_id` empty, `source_line` 1 and `timestamp_utc` the file's modification time. A
file that is not a JSON object is one `system` row
`parser: .pi/agent/trust.json is not a JSON object`.

The other standing setting, `defaultProjectTrust` in `.pi/agent/settings.json`, is one `system` row
`permission setting: defaultProjectTrust=<value> (from .pi/agent/settings.json, file mtime)` at the
file's modification time, with an empty `session_id`; no row when the key is absent, and no other
key of the file is read. pi records no per-call decision; an extension's block is an error
`toolResult` with the extension's reason as its text ([Message roles](#message-roles)).
little-coder's permission gate is the one such extension whose texts are known
([Permission gate in `little-coder.md`](little-coder.md#permission-gate)): an error `toolResult`
whose text is `edit cancelled by user` or `command cancelled by user` or starts with
`shell whitelist:` is followed by `permission decision: denied (<the text>)`, with the call's
`tool_name` and `tool_use_id` and the result's time.

#### Model rows

Where the model is recorded: per message in the assistant's `model` (requested), `responseModel`
(reported by the provider) and `provider`; per switch in a `model_change` entry (`provider`,
`modelId`), written when the user or an extension changes the model; per session nowhere, since the
header names no model and the `settings.json` defaults `defaultProvider` and `defaultModel` are not
read. A `usage` entry also names `provider` and `model` and is skipped.

A `system` row `model: <id> provider=<provider>` at the first entry of a session that names a
model, and `model: <old> -> <new> provider=<provider>` on each later entry whose model differs,
both with the `model` column set to the new id:

- `model_change`: `modelId` and `provider`.
- assistant message: `responseModel`, else `model`, and `provider`; when `responseModel` is set and
  differs from `model`, the row ends ` requested=<model>`.

The row precedes the entry's own rows and carries its timestamp. A `model_change` entry is the
user's own switch, so one that names the model already in force still yields the plain
`model: <id> provider=<provider>` row; an assistant message that repeats the current model yields
none.

#### Past-use rows and secondary sources

Both files of [Debug log and crash log](#debug-log-and-crash-log) are read on every run:

- `pi-debug.log`, secondary: one `system` row
  `debug snapshot: /debug wrote <N> message(s) of the open session, which the log does not name (from .pi/agent/pi-debug.log, Debug output at line)`
  at the time on its first line, then the lines after the marker through the session reader, each
  at the message's own `timestamp`, with an empty `session_id` and `project_path`. A row the
  session file also holds is dropped; the window is one hour, because an assistant message's own
  time is its stream start and the entry's is its end. The screen lines before the marker are not
  read.
- `crashes.json`, primary, since it repeats no session event: one `system` row per record,
  `crash: <kind> <message> version=<version> sessionFile=<sessionFile> (from .pi/agent/crashes.json, record timestamp)`
  followed by the stack on the next lines, at the record's `timestamp`, with `project_path` the
  `cwd` and `session_id` the id in the `sessionFile` name; `-` for an absent value.

pi keeps no prompt history file ([What survives a delete](#what-survives-a-delete)). The project
exports and the bug-report zip are copies of past sessions that the parser does not read.

When a file's `parentSession` parent is also in the collection, child entries whose `id` and
`timestamp` match a parent entry are dropped, so a fork does not double the timeline.

#### Sync, relay and origin rows

pi keeps no session in a vendor account, but two commands send a session out, and only one leaves
a mark.

- `/share` uploads the current branch on request
  ([slash-commands.ts:27](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/slash-commands.ts#L27),
  [interactive-mode.ts:3298-3302](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/modes/interactive/interactive-mode.ts#L3298-L3302)).
  `sessionShareDestination` (`auto`, the default, `gist` or `radius`) chooses where
  ([settings-schema.ts:229-235](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/settings-schema.ts#L229-L235),
  [session-share.ts:69-76](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/modes/interactive/session-share.ts#L69-L76);
  the vendor's
  [Export or share a session section](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/docs/sessions.md#export-or-share-a-session)).
  With Radius credentials (`/login radius`,
  [docs/providers.md:104-112](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/docs/providers.md#L104-L112))
  it posts the branch as JSONL, plus a trailing `pi.share` entry with the system prompt and tool
  schemas, to `https://radius.pi.dev/v1/artifacts` with `visibility=organization`
  ([session-share.ts:24-47](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/modes/interactive/session-share.ts#L24-L47),
  [105-138](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/modes/interactive/session-share.ts#L105-L138);
  [radius-config.ts:5](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/ai/src/providers/radius-config.ts#L5));
  otherwise it renders HTML and runs `gh gist create --public=false`, a secret gist in the user's
  GitHub account viewed at `https://pi.dev/session/#<gist id>`
  ([session-share.ts:78-95](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/modes/interactive/session-share.ts#L78-L95),
  [166-210](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/modes/interactive/session-share.ts#L166-L210);
  [config.ts:617-623](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/config.ts#L617-L623)).
  Both write only to a temporary directory removed afterwards, and the share URL is only shown on
  screen
  ([session-share.ts:58-60](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/modes/interactive/session-share.ts#L58-L60),
  [96-102](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/modes/interactive/session-share.ts#L96-L102),
  [153](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/modes/interactive/session-share.ts#L153));
  the `pi.share` entry goes into the uploaded copy, never the session file
  ([session-export.ts:8-29](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/session-export.ts#L8-L29)).
  A shared session therefore leaves no local marker, and the setting names only the likely
  destination; the gist is found from the GitHub account, the Radius artifact from the
  organization.
- `/bug` asks whether to include the transcript, then uploads `report.json`, `diagnostics.json`
  and, when chosen, `session.jsonl` and a model-written `summary.md` to
  `https://radius.pi.dev/v1/bug-reports`, with no login needed, or writes them to a zip
  ([modes/interactive/bug-report.ts:113-120](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/modes/interactive/bug-report.ts#L113-L120),
  [core/bug-report.ts:252-272](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/bug-report.ts#L252-L272),
  [bug-report-upload.ts:12-37](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/bug-report-upload.ts#L12-L37);
  the vendor's
  [Report a bug section](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/docs/sessions.md#report-a-bug)).
  Either way it appends the entry of [Bug-report entry](#bug-report-entry). The parser reads it,
  the one `custom` entry not skipped, and for `delivery` `upload` yields one `system` row
  `session synced: bug report <id> uploaded to radius.pi.dev transcript=<sessionIncluded> summary=<summaryIncluded> (from session entry pi.bug-report, entry timestamp)`,
  at the entry's `timestamp`, with the session's id and `cwd`; a `zip` entry yields
  `bug report exported: <id> path=<path> (from session entry pi.bug-report, entry timestamp)`
  instead, since nothing left the host, and an absent value prints `-`. `hint` is not printed. The
  server answers with its own `bug_report.id`, which pi shows and does not store
  ([modes/interactive/bug-report.ts:186-189](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/modes/interactive/bug-report.ts#L186-L189)).

No session relay or remote-origin session exists at the reviewed commit; the Radius relay of 1.0.2
is in [Format versions](#format-versions). Telemetry carries no session: the install ping sends
only the version to `https://pi.dev/api/report-install` after an update, unless
`enableInstallTelemetry` is false or `PI_TELEMETRY` or `PI_OFFLINE` stops it
([interactive-mode.ts:1446-1462](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/modes/interactive/interactive-mode.ts#L1446-L1462);
[core/telemetry.ts:12](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/telemetry.ts#L12);
the vendor's
[Updates, telemetry, and warnings table](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/docs/settings.md#updates-telemetry-and-warnings)),
and the telemetry package has no exporter
([telemetry/README.md:13](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/telemetry/README.md#L13)).

#### Scratch directory rows

The agent records none in the home beyond the package checkouts of
[Scratch directories](#scratch-directories). Spilled tool output goes to the system temporary
directory and is named in a `toolResult`'s `details.fullOutputPath`.

#### Deletion rows

pi writes no deletion marker; the picker's delete removes the session file with its
`pi.bug-report` entry ([What survives a delete](#what-survives-a-delete)).

#### Bad-line rows

pi itself skips malformed lines ([Record schema](#record-schema)); the parser does not. Each line
that does not decode is its own `system` row at the end of the session's rows,
`parser: unparseable line (<decoder reason>): <the line in full>`, with the session's id and
`cwd`; a file with no header line gives those rows alone, with an empty session id. Every
line-by-line reader reports bad lines.

A session file, `crashes.json`, `pi-debug.log`, `trust.json`, `settings.json` or an experimental
session's `meta.json` that cannot be read is one whole-file row
`parser: <file> could not be read (<reason>)` through `error_rows`, `<file>` its name. A
`trust.json`, `settings.json` or `meta.json` that is not a JSON object, a leading byte-order mark
dropped as pi drops it ([Project trust](#project-trust)), is one whole-file row
`parser: <path> is not a JSON object`, `<path>` relative to the home. A `crashes.json` element
that is not an object is `parser: unparseable record <N> (not a JSON object): <element>`, and a cut
array `parser: unparseable record <N> (<reason>)`. A `pi-debug.log` that cannot be opened gives no
`debug snapshot` row, and a `meta.json` row carries the session id its directory names.

#### Fixtures

A session file as the session writer
([session-manager.ts:1060-1076](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/session-manager.ts#L1060-L1076),
[1214-1223](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/session-manager.ts#L1214-L1223))
writes it, redacted, at
`.pi/agent/sessions/--home-dana-webapp--/2026-10-30T14-05-11-204Z_019a3c5e-2f71-7b40-9d1e-5b6c7a8e9f01.jsonl`;
the last line is a cut write for the bad-line row:

```json
{"type":"session","version":3,"id":"019a3c5e-2f71-7b40-9d1e-5b6c7a8e9f01","timestamp":"2026-10-30T14:05:11.204Z","cwd":"/home/dana/webapp"}
{"type":"model_change","id":"a41c09e2","parentId":null,"timestamp":"2026-10-30T14:05:11.310Z","provider":"openai","modelId":"gpt-5.1-codex"}
{"type":"message","id":"b7d2f018","parentId":"a41c09e2","timestamp":"2026-10-30T14:05:19.877Z","message":{"role":"user","content":"why does the login form reject valid emails?","timestamp":1793369119870}}
{"type":"message","id":"c93e6a71","parentId":"b7d2f018","timestamp":"2026-10-30T14:05:26.442Z","message":{"role":"assistant","content":[{"type":"thinking","thinking":"check the validator"},{"type":"toolCall","id":"call_7Qm2","name":"grep","arguments":{"pattern":"isValidEmail","path":"src"}}],"api":"openai-responses","provider":"openai","model":"gpt-5.1-codex","responseModel":"gpt-5.1-codex","usage":{"input":1830,"output":64,"cacheRead":0,"cacheWrite":0,"totalTokens":1894,"cost":{"input":0.0023,"output":0.0006,"cacheRead":0,"cacheWrite":0,"total":0.0029}},"stopReason":"toolUse","timestamp":1793369120015,"durationMs":6390}}
{"type":"message","id":"d05b7c3e","parentId":"c93e6a71","timestamp":"2026-10-30T14:05:26.611Z","message":{"role":"toolResult","toolCallId":"call_7Qm2","toolName":"grep","content":[{"type":"text","text":"src/validate.ts:12: export function isValidEmail"}],"isError":false,"timestamp":1793369126604,"durationMs":41}}
{"type":"message","id":"e2a8f4d6","parentId":"d05b7c3e","timestamp":"2026-10-30T14:06:02.128Z","message":{"role":"bashExecution","command":"npm test -- validate","output":"1 failed","exitCode":1,"cancelled":false,"truncated":false,"timestamp":1793369162120}}
{"type":"custom","customType":"pi.bug-report","data":{"id":"019a3c61-0b4e-7c22-8f10-2d9e4a6b7c58","createdAt":"2026-10-30T14:09:40.512Z","hint":null,"sessionIncluded":true,"summaryIncluded":false,"delivery":"upload"},"id":"f61c3b90","parentId":"e2a8f4d6","timestamp":"2026-10-30T14:09:41.033Z"}
{"type":"session_info","id":"07e9d2a5","parentId":"f61c3b90","timestamp":"2026-10-30T14:10:03.450Z","name":"email validation"}
{"type":"message","id":"18fa4c
```

`.pi/agent/crashes.json`, as `recordCrash` writes it
([crash-log.ts:42-45](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/crash-log.ts#L42-L45),
[124-145](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/crash-log.ts#L124-L145)),
after the crash was announced:

```json
[
  {
    "timestamp": "2026-10-30T15:22:47.019Z",
    "version": "1.4.0",
    "kind": "fatal_error",
    "message": "EPIPE: broken pipe, write",
    "stack": null,
    "sessionFile": "/home/dana/.pi/agent/sessions/--home-dana-webapp--/2026-10-30T14-05-11-204Z_019a3c5e-2f71-7b40-9d1e-5b6c7a8e9f01.jsonl",
    "cwd": "/home/dana/webapp",
    "notified": true
  }
]
```

`.pi/agent/trust.json`, as `ProjectTrustStore` writes it, keys sorted
([trust-manager.ts:133-143](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/trust-manager.ts#L133-L143)):

```json
{
  "/home/dana/downloads/untrusted-repo": false,
  "/home/dana/webapp": true
}
```

`.pi/agent/pi-debug.log`, as `/debug` writes it, the screen lines shortened:

```text
Debug output at 2026-10-30T14:11:30.006Z
Terminal: 140x48
Total lines: 2

=== All rendered lines with visible widths ===
[0] (w=44) "why does the login form reject valid emails?"

=== Agent messages (JSONL) ===
{"role":"user","content":"why does the login form reject valid emails?","timestamp":1793369119870}
{"role":"user","content":"a prompt from a session file since deleted","timestamp":1793369400310}
```

#### Wrapper attribution

The header names no producer, so attribution rests on what the wrapper leaves:

- little-coder writes into pi's own directory, through pi itself. The rows stay agent `pi`, as the
  manifest says, and the session gets one `system` row
  `session driven by little-coder (pi launcher): <why>` when a `custom_message` has a `customType`
  starting `lc-`, when an assistant `provider` is `llamacpp`, or when
  `.little-coder/checkpoints/<this file name>/` exists in the same home; the markers are listed in
  [little-coder markers in `little-coder.md`](little-coder.md#little-coder-markers). pi's own
  built-in local provider is named `llama.cpp`
  ([llama/provider.ts:24](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/extensions/llama/provider.ts#L24)),
  and pi never writes an `lc-` custom type itself. little-coder also writes
  `.pi/agent/little-coder-prompt-history.json` in pi's directory
  ([Shared with pi in `little-coder.md`](little-coder.md#shared-with-pi-piagent)).
- Letta Code writes the format under `.letta/lc-local-backend/conversations/*/messages.jsonl`, so
  the path already gives `letta`; inside, `message.id` starts `letta-msg-` and
  `message.metadata.agent_id` is set, keys pi's writer never sets
  ([Local-backend transcript in `letta.md`](letta.md#local-backend-transcript)). It shares the pi
  reader and keeps the agent from the catalog.
- Otherwise `pi`. An SDK embedder or a rebranded build cannot be told apart from the file alone.

#### Source host rows

The agent records no source-host integration. `/share` without Radius runs the user's own `gh`
CLI, after `gh auth status`, to create a secret gist
([session-share.ts:79-83](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/modes/interactive/session-share.ts#L79-L83),
[182](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/modes/interactive/session-share.ts#L182)),
so the GitHub token is `gh`'s and pi stores no account or token for it. The GitHub Copilot model
provider's OAuth grant, scope `read:user`, is a model credential kept in `auth.json` as
`{type, refresh, access, expires, enterpriseUrl}` with no login
([packages/ai/src/auth/oauth/github-copilot.ts:216-219](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/ai/src/auth/oauth/github-copilot.ts#L216-L219),
[335-341](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/ai/src/auth/oauth/github-copilot.ts#L335-L341)),
and the `git/<host>/<path>` package checkouts are installs. A grep of `packages/coding-agent/src`
and `packages/ai/src` at the reviewed commit for `gitlab`, `bitbucket`, `repositor` and
`pull request` finds only package sources, Hugging Face model names and a source-loading helper,
no repository list and no repository-bound conversation.

## Confidence

- High: the agent directory, its override, every file name of
  [Per-user storage](#per-user-storage), the session directory encoding and file naming, the
  credential file shapes and the project `.pi` contents, all from source at one commit.
- High: the header, entry types, roles, key names and timestamp units of
  [Record schema](#record-schema).
- High: that the layout is the same on Windows (a plain `homedir()` join, matched by the Windows
  installer).
- Medium: the managed install layout, read from the installer scripts as served on one day, which
  are outside the repository and can change without a commit.
- Medium: when `durationMs` and `argumentsError` first appeared between 1.0.2 and 1.3.0, and when
  the redraw and crash logs were renamed between 0.83.0 and 1.0.2; the history was not bisected.
- Medium: whether `.pi/mcp.json` and `models.json` hold literal tokens in practice (the documented
  form is `${ENV}` interpolation).
- Not determined: the experimental durable `record` JSON shape.
- Not determined: whether a rebranded pi build (`piConfig.configDir`) is in use anywhere, which
  would need its own catalog line.
- For a real-install round: that a session file appears only after the first prompt; the
  `pi-debug.log` and `crashes.json` contents; a `pi.bug-report` entry for an upload and a zip;
  `pi-tui-crash.log`; the managed install under `install/` and `bin/pi` on each platform;
  `trust.json` after a trust prompt; `deviceId` after a ChatGPT sign-in; and the Windows paths.
