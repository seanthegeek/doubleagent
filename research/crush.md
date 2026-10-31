# crush

Crush (catalog agent `crush`) is Charm's terminal coding agent. It began as an OpenCode fork
([`opencode.md`](opencode.md)) but no longer shares any storage code with it: Crush is Go, with sqlc
and goose migrations, and keeps one database per project where OpenCode keeps one per user
([OpenCode's data directory](opencode.md#data-directory)); it has no share service
([OpenCode's share records](opencode.md#1x-share-records)), no permission JSON on the session
([OpenCode's permission records](opencode.md#permission-records)), no agent or mode recorded per
message ([OpenCode's messages](opencode.md#session-messages)), and file history in a `files` table
instead of git snapshots. Hyper is Charm's hosted model gateway, and its Prism router picks a model
per request. The analyzer parses the per-project `crush.db`, every `crush.json` scope and the
`crushrc` shell configs, from source and fixtures only.

## Source and evidence

[charmbracelet/crush](https://github.com/charmbracelet/crush), open source, at
[`ca6ae26ce016b980407ce32f012a467ec10c1e2f`](https://github.com/charmbracelet/crush/commit/ca6ae26ce016b980407ce32f012a467ec10c1e2f),
Functional Source License 1.1 with MIT future license
([`LICENSE.md`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/LICENSE.md)).
All claims from source. Paths cited are relative to the clone. Between [`ca6ae26`](https://github.com/charmbracelet/crush/commit/ca6ae26ce016b980407ce32f012a467ec10c1e2f) and [`7f799fb`](https://github.com/charmbracelet/crush/commit/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da) the
lines of `internal/config/load.go` moved about 50 lines up; every path it names is unchanged.

### Observed

No real install.

### Source reviews

- 2026-10-21: charmbracelet/crush at
  [`7f799fb`](https://github.com/charmbracelet/crush/commit/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da),
  with charmbracelet/fantasy at
  [`ec25857`](https://github.com/charmbracelet/fantasy/commit/ec258577c7d825ec38cfdf8cbeaaa2f7e197aacd)
  (v0.45.1, the version `go.mod` pins): the fantasy copy settled that the hook metadata reaches the
  stored result, the dead `~/.config/crush/.crushrc` went from Confidence to
  [Per-user storage](#per-user-storage), and the parser plan's permission, relay, deletion, file and
  bad-line subsections now describe rows the parser writes; a source-host subsection was added.
- 2026-10-19:
  [`7f799fb`](https://github.com/charmbracelet/crush/commit/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da),
  the default branch that day, against
  [`ca6ae26`](https://github.com/charmbracelet/crush/commit/ca6ae26ce016b980407ce32f012a467ec10c1e2f).
  Paths, migrations, columns and part keys were unchanged; the review added the permission
  outcomes, synthetic results, model columns, channels, sub-agent ids
  and MCP overrides in [Record schema](#record-schema), the data-directory files of
  [Other stores](#other-stores), the scratch directories and the parser plan's requirement
  subsections.
- 2026-10-17:
  [`12f96b4`](https://github.com/charmbracelet/crush/commit/12f96b4acd8f36312befbbf6fe7459da9ca07a72),
  a check for sync state. It found none and added the telemetry, Hyper and client-server notes of
  [Sync, relay and origin rows](#sync-relay-and-origin-rows) and the delete behaviour of
  [What survives a delete](#what-survives-a-delete).

## Collection

### Per-user storage

Home comes from `os.UserHomeDir()` and the config root is `XDG_CONFIG_HOME` or `~/.config` on every
OS, Windows included
([`internal/home/home.go:12`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/home/home.go#L12),[`26-31`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/home/home.go#L26-L31)).

Config: `~/.config/crush/crush.json`, or `$CRUSH_GLOBAL_CONFIG/crush.json`
([`internal/config/load.go:1212-1216`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/config/load.go#L1212-L1216)),
with an optional sibling shell config `crushrc`
([`load.go:1219-1231`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/config/load.go#L1219-L1231)).
A `.crushrc` in the config directory is never read: the global list holds the system config, the
global `crush.json`, its `crushrc` sibling and the data `crush.json`, and the names `.crushrc`,
`crushrc`, `.crush.json` and `crush.json` are searched only from the working directory up to the
project boundary
([`load.go:946-975`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/config/load.go#L946-L975)).
Global context files default to `~/.config/crush/CRUSH.md` and `~/.config/AGENTS.md`
([`load.go:578-582`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/config/load.go#L578-L582)).

Commands are read from `~/.config/crush/commands`, `~/.crush/commands` and `<data dir>/commands`
([`internal/commands/commands.go:124-133`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/commands/commands.go#L124-L133)).
Skills from `$CRUSH_SKILLS_DIR`, `~/.config/crush/skills`, `~/.config/agents/skills`
([`load.go:1366-1372`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/config/load.go#L1366-L1372));
at [`7f799fb`](https://github.com/charmbracelet/crush/commit/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da)
also `~/.agents/skills` and Claude Code's `~/.claude/skills`
([`internal/config/load.go:1312-1322`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/config/load.go#L1312-L1322);
[`shared.md`](shared.md)).
System config `/etc/crush/crush.json` on Unix, none on Windows
([`internal/config/config_unix.go:7`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/config/config_unix.go#L7),
[`config_windows.go:8`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/config/config_windows.go#L8)).

Data: `GlobalConfigData()` is `$CRUSH_GLOBAL_DATA/crush.json`, `$XDG_DATA_HOME/crush/crush.json`,
`%LOCALAPPDATA%\crush\crush.json` on Windows, else `~/.local/share/crush/crush.json`
([`load.go:1259-1281`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/config/load.go#L1259-L1281)).
The directory also holds `projects.json`
([`internal/projects/projects.go:14`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/projects/projects.go#L14),[`31-32`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/projects/projects.go#L31-L32))
and a `locks/` directory
([`internal/config/store.go:1102`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/config/store.go#L1102)).
It is "writable machine state and must never be executed as Bash"
([`load.go:947-949`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/config/load.go#L947-L949)).
There is no session database here; sessions live in each project.

Cache: `$CRUSH_CACHE_DIR`, `$XDG_CACHE_HOME/crush`, `%LOCALAPPDATA%\crush\cache` on Windows, else
`~/.cache/crush`
([`load.go:1235-1250`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/config/load.go#L1235-L1250)),
containing only `server-<host>/crush.log` for client-server mode
([`internal/cmd/server.go:50`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/cmd/server.go#L50),
[`internal/cmd/root.go:508`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/cmd/root.go#L508),[`627`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/cmd/root.go#L627)).

Environment variables: `CRUSH_GLOBAL_CONFIG`, `CRUSH_GLOBAL_DATA`, `CRUSH_CACHE_DIR`,
`CRUSH_SKILLS_DIR`, `CRUSH_HYPER_API_KEY`, `XDG_*`.

At [`7f799fb`](https://github.com/charmbracelet/crush/commit/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da) the data `crush.json` also records the chosen models and recent
models, and any `crush.json` and any `crushrc` can carry standing permission state; both are
described in [crush.json and crushrc](#crushjson-and-crushrc).

`crush server` loads its configuration with the data directory itself (`~/.local/share/crush`,
Windows `%LOCALAPPDATA%\crush`) as its working directory
([`internal/cmd/server.go:40`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/cmd/server.go#L40),
[`internal/config/load.go:1228-1235`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/config/load.go#L1228-L1235));
whether that ever creates a `.crush` database there was not determined from source. The catalog
collects the whole directory either way.

#### Scratch directories

Crush makes two working directories of its own, both inside the project data directory:
`.crush/crush-fetch-*`, the working directory of the web-fetch sub-agent, created with
`os.MkdirTemp` in the data directory and removed when the fetch ends
([`internal/agent/agentic_fetch_tool.go:102-106`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/agent/agentic_fetch_tool.go#L102-L106),
[`:142`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/agent/agentic_fetch_tool.go#L142)),
and `.crush/shell-output`, oversized command output kept seven days and capped at 256 MB
([`internal/shell/truncate.go:29-43`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/shell/truncate.go#L29-L43)).
Both are collected with `.crush`. With no data directory the spill falls back to the system temp
directory
([`:145-151`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/shell/truncate.go#L145-L151)),
which no catalog line reaches; every caller found passes the data directory. Sessions record no
working directory, so none of these can be mistaken for a project
([Scratch directory rows](#scratch-directory-rows)).

### Credentials

- `~/.local/share/crush/crush.json` (Windows `%LOCALAPPDATA%\crush\crush.json`): the TUI writes
  provider keys and OAuth tokens here. `SetProviderAPIKey`
  ([`store.go:569`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/config/store.go#L569))
  resolves the global scope to `globalDataPath`
  ([`store.go:330-341`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/config/store.go#L330-L341));
  `ProviderConfig` has `api_key`
  ([`config.go:102`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/config/config.go#L102))
  and `oauth`
  ([`config.go:106`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/config/config.go#L106));
  MCP entries persist `oauth_token`
  ([`config.go:273-275`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/config/config.go#L273-L275),
  orphan cleanup
  [`config.go:326-328`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/config/config.go#L326-L328));
  the Hyper provider token is refreshed in this file
  ([`load.go:109`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/config/load.go#L109)).
- `~/.config/crush/crush.json` can hold the same `api_key` fields literally or as `$ENV` references.
- `~/.config/crush/crushrc` and project `crushrc`/`.crushrc` are shell scripts that typically export
  keys; each is flagged.
- A project's `crush.json` and `.crush.json` are config scopes with the same `api_key` fields
  ([Project-local files](#project-local-files)); both are flagged.
- Project `.crush/crush.json` is the workspace scope and `SetProviderAPIKey` can target it
  ([`store.go:333-337`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/config/store.go#L333-L337)).
- Crush also reads `~/.config/github-copilot/apps.json` (`%LOCALAPPDATA%\github-copilot\apps.json`)
  for Copilot OAuth
  ([`internal/oauth/copilot/disk.go:32-34`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/oauth/copilot/disk.go#L32-L34))
  and probes `~/.aws/credentials` and `~/.aws/login`
  ([`load.go:1118-1121`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/config/load.go#L1118-L1121)).
  No keychain use.
- No new credential file at
  [`7f799fb`](https://github.com/charmbracelet/crush/commit/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da).
  The project lock file `.crush/crush.lock` is not one
  ([Data directory files](#data-directory-files)).

### Exclusions

Nothing large under home. In projects, `.crush/crush-fetch-*` are temporary fetch directories
([`internal/agent/agentic_fetch_tool.go:102`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/agent/agentic_fetch_tool.go#L102))
and `.crush/shell-output` holds spilled tool output
([`internal/shell/truncate.go:29`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/shell/truncate.go#L29)), bounded at
256 MB and seven days since [`7f799fb`](https://github.com/charmbracelet/crush/commit/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da) ([Scratch directories](#scratch-directories)); both are
evidence of what the agent fetched and ran, so neither is an exclusion. `~/.cache/crush` holds only
the client-server log, and `stats/index.html` is one small page. Nothing new to exclude at
[`7f799fb`](https://github.com/charmbracelet/crush/commit/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da).

### Project-local files

Data dir is the closest `.crush` between cwd and the git root, else `<cwd>/.crush`
([`config.go:25`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/config/config.go#L25),
[`load.go:590-593`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/config/load.go#L590-L593)).
Contents: `crush.db` with `-wal`/`-shm`
([`internal/db/connect.go:20`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/db/connect.go#L20),[`93`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/db/connect.go#L93)),
`crush.json` workspace overrides
([`load.go:58`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/config/load.go#L58),
[`store.go:97`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/config/store.go#L97)),
`logs/crush.log`
([`root.go:311`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/cmd/root.go#L311)),
`commands/`, `shell-output/`, `crush-fetch-*`, and a `.gitignore` containing `*`
([`root.go:295-297`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/cmd/root.go#L295-L297)),
so git never sees it.

Config files searched from cwd up to the git root: `.crushrc`, `crushrc`, `.crush.json`,
`crush.json`
([`load.go:962-970`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/config/load.go#L962-L970)).

Context files: `.github/copilot-instructions.md`, `.cursorrules`, `.cursor/rules/`, `CLAUDE.md`,
`CLAUDE.local.md`, `GEMINI.md`, `gemini.md`, `crush.md`, `crush.local.md`, `Crush.md`,
`Crush.local.md`, `CRUSH.md`, `CRUSH.local.md`, `AGENTS.md`, `agents.md`, `Agents.md`
([`config.go:29-45`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/config/config.go#L29-L45)).

At
[`7f799fb`](https://github.com/charmbracelet/crush/commit/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da)
the data directory also holds `crush.lock`, the lock that records the last start
([`internal/db/datadirlock.go:23`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/db/datadirlock.go#L23)),
an empty `init` flag marking the project initialised
([`internal/config/init.go:15`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/config/init.go#L15))
and, after `crush stats`, `stats/index.html`, a usage summary
([`internal/cmd/stats.go:232`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/cmd/stats.go#L232));
their contents are in [Data directory files](#data-directory-files). The log is now at
[`internal/cmd/root.go:324`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/cmd/root.go#L324).

Ignore file `.crushignore`
([`internal/fsext/ls.go:141`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/fsext/ls.go#L141),[`168`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/fsext/ls.go#L168)).

### Where the project path is recorded

`projects.json` next to the data `crush.json` (`~/.local/share/crush/projects.json`, Windows
`%LOCALAPPDATA%\crush\projects.json`): `projects[].path`, `projects[].data_dir`,
`projects[].last_accessed`
([`projects.go:18-25`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/projects/projects.go#L18-L25)),
registered on every start
([`root.go:302`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/cmd/root.go#L302)).
At
[`7f799fb`](https://github.com/charmbracelet/crush/commit/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da)
a registration moves the project to the front of the list instead of sorting by `last_accessed`, so
the order is most recently opened first
([`internal/projects/projects.go:84-96`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/projects/projects.go#L84-L96));
the keys are unchanged
([`:17-21`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/projects/projects.go#L17-L21)).
`crush stats` itself crawls for `.crush/crush.db`
([`internal/cmd/stats.go:247-265`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/cmd/stats.go#L247-L265)),
confirming the layout.

## Parsing

### Transcript stores

Per project: `<project>/.crush/crush.db`
([`internal/config/config.go:25`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/config/config.go#L25)
default `.crush`;
[`internal/db/connect.go:93`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/db/connect.go#L93)
`crush.db`). The data directory is found by walking up from the cwd
([`internal/config/load.go:590-593`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/config/load.go#L590-L593))
and can be overridden by `data_directory`
([`config.go:470`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/config/config.go#L470)).
The global registry `projects.json` maps each project to its data directory
([Where the project path is recorded](#where-the-project-path-is-recorded)). One `messages` row per
message; parts are a JSON array in the `parts` column.

#### What survives a delete

`crush session delete` and the TUI's delete remove one session's `messages`, `files` and `sessions`
rows in one transaction, and its `read_files` rows by foreign key, with `secure_delete` zeroing the
freed pages
([`internal/session/session.go:144-168`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/session/session.go#L144-L168);
[`20260127000000_add_read_files_table.sql:7`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/db/migrations/20260127000000_add_read_files_table.sql#L7);
[`internal/db/connect.go:25`](https://github.com/charmbracelet/crush/blob/12f96b4acd8f36312befbbf6fe7459da9ca07a72/internal/db/connect.go#L25)).
At [`12f96b4`](https://github.com/charmbracelet/crush/commit/12f96b4acd8f36312befbbf6fe7459da9ca07a72) the same delete is one query and two cascades
([`internal/db/sql/sessions.sql:79-81`](https://github.com/charmbracelet/crush/blob/12f96b4acd8f36312befbbf6fe7459da9ca07a72/internal/db/sql/sessions.sql#L79-L81),
[`20250424200609_initial.sql:32`](https://github.com/charmbracelet/crush/blob/12f96b4acd8f36312befbbf6fe7459da9ca07a72/internal/db/migrations/20250424200609_initial.sql#L32),
[`20250424200609_initial.sql:56`](https://github.com/charmbracelet/crush/blob/12f96b4acd8f36312befbbf6fe7459da9ca07a72/internal/db/migrations/20250424200609_initial.sql#L56)).

The deleted session's sub-agent sessions stay in `crush.db` with their messages, since
`parent_session_id` has no foreign key and the delete names one id, so a `parent_session_id` that
names no session is the trace of a deleted parent. Crush keeps no per-prompt history file and no
second copy of the transcript; `logs/crush.log` and the `stats/index.html` snapshot, which still
counts the deleted session, outlive the delete. The `session deleted` telemetry event goes to the
vendor without a session id
([`internal/session/session.go:173`](https://github.com/charmbracelet/crush/blob/12f96b4acd8f36312befbbf6fe7459da9ca07a72/internal/session/session.go#L173),
[`internal/event/all.go:28-30`](https://github.com/charmbracelet/crush/blob/12f96b4acd8f36312befbbf6fe7459da9ca07a72/internal/event/all.go#L28-L30)),
and nothing on disk records it. The analyzer reads the database, the sub-agent sessions included,
and writes a `session deleted:` row for each parent a sub-agent session names that is gone
([Deletion rows](#deletion-rows)); it reads neither the log nor the stats page (see
[Secondary sources](../analyzer/docs/output.md#secondary-sources)).

### Record schema

[`internal/db/migrations/20250424200609_initial.sql`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/db/migrations/20250424200609_initial.sql).

#### sessions

`sessions`
([`:4-14`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/db/migrations/20250424200609_initial.sql#L4-L14)
plus ALTERs): `id` (uuid; task sub-sessions use the tool call id,
[`internal/session/session.go:103`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/session/session.go#L103),[`117`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/session/session.go#L117)),
`parent_session_id`, `title`, `message_count`, `prompt_tokens`, `completion_tokens`, `cost`,
`updated_at`, `created_at` (epoch seconds, `strftime('%s','now')`,
[`internal/db/sql/sessions.sql:22-23`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/db/sql/sessions.sql#L22-L23)),
`summary_message_id`, `todos` JSON, `channel`.

#### messages and parts

`messages`
([`:47-57`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/db/migrations/20250424200609_initial.sql#L47-L57)
plus ALTERs;
[`internal/db/models.go:29-44`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/db/models.go#L29-L44)):
`id` uuid
([`internal/message/message.go:206`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/message/message.go#L206)),
`session_id`, `role` in `assistant|user|system|tool`
([`internal/message/content.go:20-27`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/message/content.go#L20-L27)),
`parts` JSON, `model`, `provider`
([`20250627000000_add_provider_to_messages.sql`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/db/migrations/20250627000000_add_provider_to_messages.sql)),
`created_at`, `updated_at`, `finished_at` (seconds, copied from the `finish` part's `time`,
[`message.go:435-439`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/message/message.go#L435-L439)),
`is_summary_message`, `prism_model_id`, `prism_model_name`, `prism_*_savings`.

`parts` is `[{"type":...,"data":{...}}]`
([`message.go:613-629`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/message/message.go#L613-L629)).
Types and `data` keys
([`content.go`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/message/content.go)):
`text`: `text`, `hidden?`
([`:70-74`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/message/content.go#L70-L74));
`reasoning`: `thinking`, `signature`, `thought_signature`, `tool_id`, `responses_data`,
`started_at`, `finished_at` (seconds;
[`:55-63`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/message/content.go#L55-L63));
`tool_call`: `id`, `name`, `input` (JSON string), `provider_executed`, `finished`
([`:109-115`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/message/content.go#L109-L115));
`tool_result`: `tool_call_id`, `name`, `content`, `data` (base64 media), `mime_type`, `metadata`,
`is_error`
([`:119-127`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/message/content.go#L119-L127));
`finish`: `reason` in `end_turn|max_tokens|tool_use|canceled|error|content_filter|unknown`, `time`
(seconds), `message?`, `details?`
([`:33-49`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/message/content.go#L33-L49),[`131-136`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/message/content.go#L131-L136));
`shell_command`: `command`, `output`, `exit_code`
([`:142-146`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/message/content.go#L142-L146));
`image_url`: `url`, `detail`; `binary`: untagged Go fields `Path`, `MIMEType`, `Data`
([`:93-97`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/message/content.go#L93-L97)).

#### Other tables

`files` (session file snapshots: `path`, `content`,
`version`), `read_files`, `mcp_enabled_servers`, `mcp_disabled_servers`. No
project path, cwd or git branch column anywhere.

The columns and part keys above are unchanged at
[`7f799fb`](https://github.com/charmbracelet/crush/commit/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da), which the subsections that follow cite.

#### Permission outcomes in tool results

The permission service keeps every decision in memory: the per-session grant ("allow for session",
`GrantPersistent`), the allow list and the skip flag behind `--yolo`
([`internal/permission/permission.go:165-197`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/permission/permission.go#L165-L197)),
and none of it is written to a table. A call the user denies returns `User denied permission` as an
error result that ends the turn
([`internal/agent/tools/tools.go:78-84`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/agent/tools/tools.go#L78-L84)),
stored as a `tool_result` part with `is_error: true`
([`internal/agent/agent.go:2155-2172`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/agent/agent.go#L2155-L2172)).

A `PreToolUse` hook (a user command configured under `hooks` in `crush.json`) that denies or halts a
call returns `Tool call blocked by hook. Reason: <reason>` or
`Turn halted by hook. Reason: <reason>`, and every hooked result carries
`{"hook": {"hook_count", "decision", "halt", "reason", "input_rewrite", "hooks": [{"name", "matcher", "decision", ...}]}}`
in its `metadata`, `decision` being `allow`, `deny` or `none`
([`internal/agent/hooked_tool.go:62-73`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/agent/hooked_tool.go#L62-L73),
[`:98-140`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/agent/hooked_tool.go#L98-L140),
[`internal/hooks/hooks.go:26-40`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/hooks/hooks.go#L26-L40),
[`:56-65`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/hooks/hooks.go#L56-L65)).
The `hook` object reaches the stored result: fantasy copies the tool response's `Metadata` to the
result's `ClientMetadata` on both the error and the success path
([`agent.go:851`](https://github.com/charmbracelet/fantasy/blob/ec258577c7d825ec38cfdf8cbeaaa2f7e197aacd/agent.go#L851),
[`:859`](https://github.com/charmbracelet/fantasy/blob/ec258577c7d825ec38cfdf8cbeaaa2f7e197aacd/agent.go#L859)),
and Crush stores `ClientMetadata` as the part's `metadata`, a JSON string
([`internal/agent/agent.go:2155-2160`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/agent/agent.go#L2155-L2160)).

#### Synthetic results

`internal/agent/settle.go`, new since
[`ca6ae26`](https://github.com/charmbracelet/crush/commit/ca6ae26ce016b980407ce32f012a467ec10c1e2f),
settles tool calls that never got an answer (a cancel, a crash, a truncated or refused response)
with an error `tool_result` in a new `tool` message whose text is one of a fixed set
(`Error: user cancelled assistant tool calling`,
`tool call was interrupted and did not produce a result...` and four others), and on reopening a
session after a dead process gives the unfinished assistant message a `finish` of reason `error`,
message `Interrupted`, backdated to the message's `updated_at`
([`internal/agent/settle.go:14-24`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/agent/settle.go#L14-L24),
[`:140-170`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/agent/settle.go#L140-L170)).
The synthetic result's message is created at repair time, so it can be stamped long after the call.

#### Model columns

`messages.model` and `provider` are the configured model of the turn
([`internal/agent/agent.go:557-562`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/agent/agent.go#L557-L562));
`prism_model_id` and `prism_model_name` are the model that actually served it when the turn went
through Hyper's Prism router, from its response headers
([`:1085`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/agent/agent.go#L1085),
[`:1914-1929`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/agent/agent.go#L1914-L1929);
[`20260902000000_add_prism_model_to_messages.sql:3-8`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/db/migrations/20260902000000_add_prism_model_to_messages.sql#L3-L8)).
So `model` is the request and `prism_model_id` the resolved id. The Prism layout is not observed
on any real install, source-derived. No per-session model and no switch record exist; the plan and
code modes and yolo mode the TUI toggles are in memory only
([`internal/ui/model/ui.go:4762-4790`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/ui/model/ui.go#L4762-L4790)),
and the reasoning effort and thinking flag reach only telemetry.

#### Channels (relay)

An MCP server that declares the `claude/channel` capability pushes events into the active session as
a user message whose text is `<channel source="<server>" <meta attributes>>content</channel>`
([`internal/agent/tools/mcp/channel.go:20-36`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/agent/tools/mcp/channel.go#L20-L36),
[`:124-150`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/agent/tools/mcp/channel.go#L124-L150);
[`internal/agent/agent.go:1550-1551`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/agent/agent.go#L1550-L1551)),
`sessions.channel` holds the server of the latest channel-started turn and is cleared by the next
local turn
([`internal/agent/coordinator.go:436-465`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/agent/coordinator.go#L436-L465)),
and at the end of such a turn Crush sends the reply through one of the server's tools, chosen by
`mcp.<name>.channel_reply` in `crush.json` or discovered from the tool list (the latter only in yolo
mode); the `sender` and `group` meta attributes name the recipient
([`internal/agent/channelreply.go:23-40`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/agent/channelreply.go#L23-L40),
[`:245-307`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/agent/channelreply.go#L245-L307)).
The rendered element is the turn's prompt as it stands, so the stored user text starts
`<channel source=`
([`internal/backend/channels.go:75-96`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/backend/channels.go#L75-L96)).
The source names a Signal MCP server as the model case, so a channel session can be carried through
a messaging platform. The automatic reply is not a stored tool call: only the log records it
([Data directory files](#data-directory-files)).

#### Sub-agent sessions

A sub-agent's session id is `<message id>$$<tool call id>` with `parent_session_id` set
([`internal/session/session.go:116-128`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/session/session.go#L116-L128),
[`:427-430`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/session/session.go#L427-L430);
[`internal/agent/coordinator.go:1667-1668`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/agent/coordinator.go#L1667-L1668)).
`parent_session_id` has no foreign key
([`20250424200609_initial.sql:4-14`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/db/migrations/20250424200609_initial.sql#L4-L14)).

#### Repository-scoped MCP overrides

`mcp_enabled_servers` and `mcp_disabled_servers` hold one `name` each, an MCP server the user turned
on or off for this project, overriding `crush.json`
([`20260928000000_add_mcp_disabled_servers.sql:3-5`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/db/migrations/20260928000000_add_mcp_disabled_servers.sql#L3-L5);
[`internal/session/session.go:403-415`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/session/session.go#L403-L415)).

### Joins and timestamps

`messages.session_id → sessions.id`; `sessions.parent_session_id` for sub-agents. `tool_call` parts
live in the assistant message; the result is a separate `role='tool'` message with a `tool_result`
part
([`internal/agent/agent.go:1032-1038`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/agent/agent.go#L1032-L1038)),
joined on `tool_result.data.tool_call_id = tool_call.data.id`. Project path is the parent directory
of the `.crush` holding the database, or `projects.json` `projects[].path` / `data_dir`.

### Other stores

#### SQLite journal and deletes

`journal_mode=WAL`
([`connect.go:20`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/db/connect.go#L20))
with Go defaults (auto-checkpoint at 1000 pages), so a copy without `crush.db-wal` loses recent
messages. `secure_delete=ON`
([`connect.go:25`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/db/connect.go#L25))
zeroes deleted rows. Writes are debounced about 33 ms
([`message.go:21`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/message/message.go#L21))
and serialised on one connection
([`connect.go:142`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/db/connect.go#L142)).
No encryption or compression.

#### Data directory files

Other files in the data directory, at
[`7f799fb`](https://github.com/charmbracelet/crush/commit/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da); none repeats the transcript, so none is a secondary source:

- `logs/crush.log`, JSON lines from Go's `slog` at INFO level, rotated at 10 MB and kept 30 days
  ([`internal/cmd/root.go:324`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/cmd/root.go#L324),
  [`internal/log/log.go:23-44`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/log/log.go#L23-L44)).
  It is the only local record of a reply Crush routed back to a channel
  (`Routed reply to originating channel`,
  [`internal/agent/channelreply.go:302-306`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/agent/channelreply.go#L302-L306))
  and of the `permissions allow|deny` lines a `crushrc` ran
  ([`internal/shellconfig/permissions.go:56`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/shellconfig/permissions.go#L56),
  [`:76`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/shellconfig/permissions.go#L76)).
  Not parsed; it survives a session delete.
- `stats/index.html`, written by `crush stats` ([Project-local files](#project-local-files)): totals,
  usage by day, model, hour and tool, never message text
  ([`internal/cmd/stats.go:62-73`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/cmd/stats.go#L62-L73)).
  A snapshot at generation time, so it still counts sessions deleted since.
- `crush.lock`, the data-directory lock, rewritten at each start with `pid`, `version` and
  `started_at`
  ([`internal/db/datadirlock.go:29-33`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/db/datadirlock.go#L29-L33),
  [`:89-103`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/db/datadirlock.go#L89-L103)):
  the Crush version and time of the last start in that project. It holds no credential.

#### crush.json and crushrc

Standing permission state, at
[`7f799fb`](https://github.com/charmbracelet/crush/commit/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da):
`permissions.allowed_tools` (tools or `tool:action` pairs that skip the prompt),
`options.disabled_tools` (tools hidden from the agent), `mcp.<name>.disabled_tools` and
`enabled_tools`, and `hooks` (the `PreToolUse` commands) in any `crush.json` scope
([`internal/config/config.go:224-225`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/config/config.go#L224-L225),
[`:431-433`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/config/config.go#L431-L433),
[`:471`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/config/config.go#L471),
[`:865-869`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/config/config.go#L865-L869);
[`internal/app/app.go:103-106`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/app/app.go#L103-L106)),
plus the same two lists set by `permissions allow <tool>...` and `permissions deny <tool>...` lines
in a `crushrc` shell config
([`internal/shellconfig/permissions.go:10-78`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/shellconfig/permissions.go#L10-L78)),
and the per-project `mcp_enabled_servers` and `mcp_disabled_servers` tables. `--yolo` (`-y`) is a
command-line flag held in memory
([`internal/cmd/root.go:61`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/cmd/root.go#L61),
[`:301`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/cmd/root.go#L301)),
never written. The `crush.json` files and `crushrc` are collected and parsed
([Permission rows](#permission-rows)).

The global data `crush.json` keeps the user's current `models.large` and `models.small` and up to
five `recent_models` per type
([`internal/config/config.go:850-854`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/config/config.go#L850-L854),
[`internal/config/store.go:521-547`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/config/store.go#L521-L547)),
a preference, not a per-session record.

### Format versions

Goose migrations only (12 files, the same set at [`ca6ae26`](https://github.com/charmbracelet/crush/commit/ca6ae26ce016b980407ce32f012a467ec10c1e2f) and [`7f799fb`](https://github.com/charmbracelet/crush/commit/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da)); no JSON era, no version
field; `goose_db_version` table. The one record shape added between those commits is the
[synthetic result](#synthetic-results).

### Secrets

`crush.json` `providers.<name>.api_key`
([`config.go:102`](https://github.com/charmbracelet/crush/blob/ca6ae26ce016b980407ce32f012a467ec10c1e2f/internal/config/config.go#L102))
at `~/.config/crush/`, `~/.local/share/crush/` and `<project>/.crush/`. `reasoning.signature` and
`thought_signature` are opaque provider blobs. Tool input and output can contain pasted tokens.

### Parser plan

Globs: `.local/share/crush/projects.json`,
`AppData/Local/crush/projects.json`, project-level `.crush/crush.db{,-wal,-shm}`, every `crush.json`
scope (`.config/crush/crush.json`, `.local/share/crush/crush.json`,
`AppData/Local/crush/crush.json`, `.crush/crush.json`, a project's `crush.json` and `.crush.json`),
and `.config/crush/crushrc` and a project's `crushrc` and `.crushrc`.

Rows: one per part in `parts`.

| Column | Source |
| --- | --- |
| timestamp_utc | `messages.created_at` (s); tool_call and tool_result use `finished_at` or `finish.time` |
| session_id | `sessions.id` |
| project_path | directory containing `.crush`, or `projects.json` `path` |
| git_branch | empty |
| turn_type | `user`/`assistant` text; `reasoning` → thinking; `tool_call` → tool_use; `tool_result` → tool_result; role `system`, `finish`, `shell_command` → system |
| model | `provider`/`prism_model_id` when set, else `provider`/`model` |
| tool_name, tool_use_id | `data.name`, `data.id` or `data.tool_call_id` |
| text | text, input JSON, output |

#### Permission rows

Parsed; source-derived; not observed on any real install.

Per call (primary). After a `tool_result` row, with its `tool_name` and `tool_use_id`, the parser
writes `permission decision: denied by user` when the error content is `User denied permission`;
`permission decision: <allow|deny|halt> by hook <names> reason=<reason>` from `metadata.hook`, where
`halt` wins over the decision, `<names>` are the hooks whose own decision (or halt) matches, and
` reason=` is present only when the hook gave one; and `permission decision: <deny|halt> by hook`
for an error result that starts `Tool call blocked by hook.` or `Turn halted by hook.` but carries
no hook metadata
([Permission outcomes in tool results](#permission-outcomes-in-tool-results)). A hook decision of
`none` without a halt writes nothing. A prompt the user approved, a session grant, the allow list
and yolo mode leave nothing, so the absence of a denial is not an approval.

Standing (primary). One `permission setting: <key>=<value> (from <file>, file mtime)` row per entry
at the file's modification time, with an empty `session_id`
([crush.json and crushrc](#crushjson-and-crushrc)): from every `crush.json` scope the entries of
`permissions.allowed_tools`, `options.disabled_tools`, `mcp.<name>.disabled_tools`,
`mcp.<name>.enabled_tools` and each hook under `hooks.<event>` (the hook object as compact JSON);
from a `crushrc`, each tool on a `permissions allow` line as `permissions.allowed_tools=<tool>` and
on a `permissions deny` line as `options.disabled_tools=<tool>`, with `(from <file> line <n>, file
mtime)`; from `crush.db`, `mcp_enabled_servers=<name>` and `mcp_disabled_servers=<name>` at the
database file's modification time. The `crushrc` reader takes only lines whose first word is
`permissions`, as written, so a line inside a condition that never ran still yields its rows. A
workspace `.crush/crush.json` and a project file carry the project as `project_path`; no other
`crush.json` key is read, so provider keys and tokens never reach a row.

#### Model rows

Primary, parsed. The served model of a message is `prism_model_id` when set,
else `model`. A `system` row `model: <served> provider=<provider>`
precedes the first message of a session that names a model, and
`model: <old served> -> <new served> provider=<provider>` every later
message of the session whose served model, requested model or provider
differs, with the message's `created_at`; ` requested=<model>` follows
when the requested model differs from the served one (a Hyper Prism
turn: `model: claude-sonnet-4-5 provider=hyper requested=<model>`). A
message with no `prism_model_id` that repeats the session's last provider
and requested model is not a change, so a Hyper step without Prism
headers does not flip the row back to the request. `tool` messages store
an empty model and are skipped.

No per-session model or switch record exists ([Model columns](#model-columns)), and the
`models.large`, `models.small` and `recent_models` preferences of the data `crush.json` are not a
per-session record ([crush.json and crushrc](#crushjson-and-crushrc)).

#### Past-use rows and secondary sources

Crush keeps none that repeats the transcript: no prompt history file (the TUI's prompt history is a
query over `messages`,
[`internal/ui/model/history.go:18-30`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/ui/model/history.go#L18-L30)),
no search index, no export. Evidence of other kinds: `files` (each version of every file the agent
wrote, per session) and `read_files` (each file read, per session, with `read_at`), both deleted
with their session, are parsed, as `file version: <path> version=<n> bytes=<length> (from crush.db
files)` at the snapshot's `created_at` (the content stays in the database) and `file read: <path>
(from crush.db read_files, read_at)` at `read_at`, source-derived; not observed on any real install.
Neither repeats a transcript event, so neither is secondary. `logs/crush.log` and
`stats/index.html` ([Data directory files](#data-directory-files)), which outlive the session, are
not parsed. Compaction (`is_summary_message`)
adds a summary message and keeps the earlier ones; the model reads from the summary onward
([`internal/db/sql/messages.sql:79`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/db/sql/messages.sql#L79)),
so nothing is lost locally.

#### Sync, relay and origin rows

Sync: Crush records no sync state, at
[`12f96b4`](https://github.com/charmbracelet/crush/commit/12f96b4acd8f36312befbbf6fe7459da9ca07a72)
and again at
[`7f799fb`](https://github.com/charmbracelet/crush/commit/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da).
No command shares, exports, publishes or uploads a session: the subcommands are `run`, `dirs`,
`projects`, `update-providers`, `logs`, `logout`, `schema`, `login`,
`stats` and `session`
([`internal/cmd/root.go:67-78`](https://github.com/charmbracelet/crush/blob/12f96b4acd8f36312befbbf6fe7459da9ca07a72/internal/cmd/root.go#L67-L78);
unchanged at [`7f799fb`](https://github.com/charmbracelet/crush/commit/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da),
[`internal/cmd/root.go:68-79`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/cmd/root.go#L68-L79)),
plus `server` and `models` registered in their own files, and `session` has only `list`, `show`,
`last`, `delete` and `rename`
([`internal/cmd/session.go:34-97`](https://github.com/charmbracelet/crush/blob/12f96b4acd8f36312befbbf6fe7459da9ca07a72/internal/cmd/session.go#L34-L97)).
No migration adds a remote id, share URL, synced flag or sync time to any table, at either commit
(the twelve files under `internal/db/migrations/`; the newest `sessions` column, `channel`, is
[`20260921000000_add_channel_to_sessions.sql:2`](https://github.com/charmbracelet/crush/blob/12f96b4acd8f36312befbbf6fe7459da9ca07a72/internal/db/migrations/20260921000000_add_channel_to_sessions.sql#L2)).
Searched `share`, `publish`, `upload`, `cloud`, `sync`, `teleport`, `backup`, `export`, `remote_id`,
`share_url`, `synced` and `gist` over the Go and SQL sources. What does leave the host, besides the
model calls:

- Usage telemetry to PostHog (a hosted product-analytics service) at `https://data.charm.land`
  ([`internal/event/event.go:17`](https://github.com/charmbracelet/crush/blob/12f96b4acd8f36312befbbf6fe7459da9ca07a72/internal/event/event.go#L17)),
  on by default and off when `CRUSH_DISABLE_METRICS` or `DO_NOT_TRACK` is true or
  `options.disable_metrics` is set in `crush.json`
  ([`internal/cmd/root.go:905-916`](https://github.com/charmbracelet/crush/blob/12f96b4acd8f36312befbbf6fe7459da9ca07a72/internal/cmd/root.go#L905-L916),
  [`internal/config/config.go:475`](https://github.com/charmbracelet/crush/blob/12f96b4acd8f36312befbbf6fe7459da9ca07a72/internal/config/config.go#L475)).
  The events are named actions (`session created`, `session deleted`, `prompt sent` and others,
  [`internal/event/all.go:24-83`](https://github.com/charmbracelet/crush/blob/12f96b4acd8f36312befbbf6fe7459da9ca07a72/internal/event/all.go#L24-L83));
  the prompt and token events carry the session id, provider, model, reasoning effort, thinking
  mode, yolo mode (the setting that skips permission prompts), token counts and cost, never prompt
  or reply text
  ([`internal/agent/event.go:26-50`](https://github.com/charmbracelet/crush/blob/12f96b4acd8f36312befbbf6fe7459da9ca07a72/internal/agent/event.go#L26-L50)).
  The sender id is a hashed machine id recomputed at each start, not stored
  ([`internal/event/identifier.go:20-28`](https://github.com/charmbracelet/crush/blob/12f96b4acd8f36312befbbf6fe7459da9ca07a72/internal/event/identifier.go#L20-L28)),
  and a Hyper login aliases it to the Hyper account's user id
  ([`internal/oauth/hyper/device.go:104-105`](https://github.com/charmbracelet/crush/blob/12f96b4acd8f36312befbbf6fe7459da9ca07a72/internal/oauth/hyper/device.go#L104-L105));
  a TUI crash or panic sends the error type and message
  ([`internal/event/event.go:100`](https://github.com/charmbracelet/crush/blob/12f96b4acd8f36312befbbf6fe7459da9ca07a72/internal/event/event.go#L100),
  [`internal/log/log.go:68`](https://github.com/charmbracelet/crush/blob/12f96b4acd8f36312befbbf6fe7459da9ca07a72/internal/log/log.go#L68)).
  Nothing on disk records what was sent.
- Hyper (`https://hyper.charm.land`,
  [`internal/agent/hyper/provider.go:45`](https://github.com/charmbracelet/crush/blob/12f96b4acd8f36312befbbf6fe7459da9ca07a72/internal/agent/hyper/provider.go#L45))
  is a model provider; its calls are model API calls, not a server copy of the session.
- Client-server mode (`crush server`, the TUI attached with `--host`) keeps the database with the
  server process, on a Unix socket or named pipe by default and TCP when asked
  ([`internal/server/server.go:72-86`](https://github.com/charmbracelet/crush/blob/12f96b4acd8f36312befbbf6fe7459da9ca07a72/internal/server/server.go#L72-L86),
  [`internal/cmd/server.go:23`](https://github.com/charmbracelet/crush/blob/12f96b4acd8f36312befbbf6fe7459da9ca07a72/internal/cmd/server.go#L23));
  it is self-hosted, with no vendor service.
- MCP channels
  ([`internal/agent/tools/mcp/channel.go:20-36`](https://github.com/charmbracelet/crush/blob/12f96b4acd8f36312befbbf6fe7459da9ca07a72/internal/agent/tools/mcp/channel.go#L20-L36))
  push messages from an MCP server into a session; `sessions.channel` names that server. The flow is
  inbound, so it marks no upload.

The parser plan proposes no sync row.

Relay (primary, parsed; source-derived; not observed on any real install). A session whose
`sessions.channel` is set, or whose user text starts with `<channel source=`, was driven by an MCP
channel ([Channels (relay)](#channels-relay)). The parser sets `records_relay` and writes `session
relayed: channel <server> (from crush.db sessions.channel, as of updated_at)` after the session
start of a bound session, at its `updated_at`, and `session relayed: channel <source> sender
<sender> group <group> (from crush.db message <id>)` after the first user message of a session for
each distinct source, sender and group, with `sender` and `group` present only when the element
carries them. Whether the channel is a messaging platform is only in the server's name and the
`<channel>` attributes. A channel turn's automatic reply is in the log alone.

Origin: none; every session is created by the local process.

#### Scratch directory rows

The [scratch directories](#scratch-directories) appear only in the web-fetch sub-agent's tool
inputs. Since sessions record no working directory and the parser takes the project from the
database's location, no scratch path can become a project; no `scratch directory:` row is needed.

#### Deletion rows

Parsed; source-derived; not observed on any real install. A session whose `parent_session_id`
names no session is the trace of a deleted parent
([What survives a delete](#what-survives-a-delete)). The parser writes one row per such parent,
`session deleted: <parent> (parent_session_id of sub-agent session <id> names no session in
crush.db; time is the sub-agent's updated_at)`, with `session_id` the deleted parent, so the parent
appears as a session of its own; the delete time is not recorded, and the sub-agent's `updated_at`
is only a time the parent still existed before. The log, the stats page and the telemetry event
`session deleted` (no session id) outlive the delete and are not read.

#### Bad-line rows

A `crushrc` is read line by line for its `permissions` lines: such a line that does not split
into shell words (an unclosed quote) is one row `parser: unparseable line (<reason>): <line>` with
`source_line` its line number, and the file's other lines are still read; other lines are not
records and are skipped. `projects.json` that does not parse or cannot be read is one whole-file
`parser:` row, as is a `crush.json` that is not a JSON object
(`parser: <file> is not a JSON object`) or cannot be read
(`parser: <file> could not be read: <reason>`); a `messages.parts` value that does not decode
(not JSON, or JSON that is not an array) is one `system` row
`parser: unparseable record <rowid> (message <id> parts: <reason>): <parts>`, carrying the stored
value in full, with the message's session and project, an empty timestamp and `source_line` set to
the rowid; the session's other messages are still yielded
([issue 166](https://github.com/seanthegeek/doubleagent/issues/166)).

#### Fixtures

Fixture rows:

```sql
INSERT INTO sessions(id,title,message_count,prompt_tokens,completion_tokens,cost,updated_at,created_at) VALUES('6f1c0001','Fix bug',3,10,5,0.01,1760000005,1760000001);
INSERT INTO messages(id,session_id,role,parts,model,provider,created_at,updated_at,finished_at,is_summary_message) VALUES
('m1','6f1c0001','user','[{"type":"text","data":{"text":"list files"}},{"type":"finish","data":{"reason":"stop","time":0}}]','claude-sonnet-4','anthropic',1760000001,1760000001,NULL,0),
('m2','6f1c0001','assistant','[{"type":"tool_call","data":{"id":"call_x1","name":"bash","input":"{\"command\":\"ls\"}","provider_executed":false,"finished":true}},{"type":"finish","data":{"reason":"tool_use","time":1760000003}}]','claude-sonnet-4','anthropic',1760000002,1760000003,1760000003,0),
('m3','6f1c0001','tool','[{"type":"tool_result","data":{"tool_call_id":"call_x1","name":"bash","content":"a.txt","data":"","mime_type":"","metadata":"","is_error":false}},{"type":"finish","data":{"reason":"stop","time":0}}]','','',1760000003,1760000003,NULL,0);
```

`projects.json`:

```json
{"projects":[{"path":"/home/u/repo","data_dir":"/home/u/repo/.crush","last_accessed":"2026-10-01T12:00:00Z"}]}
```

Permission, relay, deletion and file records, invented values in the shapes above (a hook denial
shown; the user denial is the content `User denied permission` with `is_error: true` and an empty
`metadata`):

```json
{"type":"tool_result","data":{"tool_call_id":"call_d2","name":"bash","content":"Tool call blocked by hook. Reason: no rm","metadata":"{\"hook\":{\"hook_count\":1,\"decision\":\"deny\",\"reason\":\"no rm\",\"hooks\":[{\"name\":\"guard\",\"matcher\":\"bash\",\"decision\":\"deny\"}]}}","is_error":true}}
```

```text
<channel source="signal" group="g-77" sender="+15550100">deploy status?</channel>
```

A sub-agent session `m9$$call_t1` with `parent_session_id` `6f1c0099` and no such session; a
`crush.json` with `permissions.allowed_tools`, `options.disabled_tools`, `mcp.docs.disabled_tools`,
`mcp.docs.enabled_tools` and one `hooks.PreToolUse` entry beside a provider `api_key`; a `crushrc`:

```sh
permissions allow view 'bash:ls'
permissions deny sourcegraph
```

#### Source host rows

The agent records no source-host integration. Searched `github`, `gitlab`, `bitbucket`, `oauth`,
`installation`, `repositor`, `pull request` and `clone` over the Go sources at
[`7f799fb`](https://github.com/charmbracelet/crush/commit/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da):
the only GitHub account Crush holds is the Copilot OAuth grant it reads from
`github-copilot/apps.json` to reach the Copilot models
([Credentials](#credentials)), a model-provider credential, not repository access; the `gh` CLI is
only probed on `PATH` to word the tool descriptions
([`internal/agent/tools/tools.go:86-93`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/agent/tools/tools.go#L86-L93)),
and GitHub's MCP endpoint appears only in a list of servers that keep no MCP session
([`internal/config/config.go:628-631`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/config/config.go#L628-L631)),
so a GitHub MCP server is an ordinary user-configured MCP entry. No repository list and no
repository-bound remote conversation is stored.

## Confidence

- High: all paths and file names, the table columns, the part JSON field names, the timestamp units,
  WAL, the tool result as a separate message and the absence of a git branch (source).
- Medium: the Hyper Prism columns (`prism_model_id`, `prism_model_name`) are not observed on any
  real install, source-derived ([Model columns](#model-columns)).
- Medium: the permission, relay, deletion, file and standing-setting rows rest on source alone; the
  hook, channel and Hyper surfaces are unexercised, and a real-install round is still owed.
- Not determined: the `binary` part key casing in shipped builds (inferred from an untagged Go
  struct).
