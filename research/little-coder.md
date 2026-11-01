# little-coder

little-coder is a launcher plus a fixed set of extensions for the [pi](pi.md) coding agent: a
terminal CLI that spawns pi's own CLI and writes its sessions into pi's directory, `~/.pi/agent`,
beside a few files of its own, so its transcripts are pi session files written by pi itself. The
retired v0.0.x releases were a separate Python implementation with their own state in
`~/.little-coder`. Catalog agent: `little-coder`. Parsed: the pi parser reads the sessions under
agent `pi` and names little-coder in a `system` row, and the little-coder parser reads the files
little-coder writes itself, those of the v0.0.x releases included; this document covers what
little-coder adds and how to tell its sessions from plain pi sessions, and the full pi record
schema is in [`pi.md`](pi.md).

## Source and evidence

[itayinbarr/little-coder](https://github.com/itayinbarr/little-coder) at
[`89d4fa0af864230527ab75d12604ed0eb320e6df`](https://github.com/itayinbarr/little-coder/commit/89d4fa0af864230527ab75d12604ed0eb320e6df),
version 1.20.0, Apache-2.0
([package.json:3](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/package.json#L3),
[10](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/package.json#L10)).
Open source, TypeScript and Node. All claims from source. little-coder depends on the pi coding
agent (`@earendil-works/pi-coding-agent` `^0.83.0`,
[package.json:41](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/package.json#L41));
it spawns pi's own CLI from its `node_modules`
([bin/little-coder.mjs:98-136](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/bin/little-coder.mjs#L98-L136),
[505-509](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/bin/little-coder.mjs#L505-L509)).
pi's storage was therefore read at [earendil-works/pi](https://github.com/earendil-works/pi) tag
`v0.83.0`,
[`845d6ff1f6643aba440341cce877ce1c43ebbc39`](https://github.com/earendil-works/pi/commit/845d6ff1f6643aba440341cce877ce1c43ebbc39).

The retired v0.0.x Python implementation, which kept its own state in `~/.little-coder` and had a
Gist sync command, was read at tag `v0.0.5`,
[`424f7a4b9f9214a36a6bcb69e1f3435ca3e68d03`](https://github.com/itayinbarr/little-coder/commit/424f7a4b9f9214a36a6bcb69e1f3435ca3e68d03)
(its `pyproject.toml` says version 4.0.0), for sync on 2026-10-17 and for its whole layout on
2026-10-21 ([The v0.0.x layout](#the-v00x-layout-little-coder)).

### Observed

- No real install.

### Source reviews

- 2026-10-21: itayinbarr/little-coder at [`424f7a4`](https://github.com/itayinbarr/little-coder/commit/424f7a4b9f9214a36a6bcb69e1f3435ca3e68d03) (tag `v0.0.5`) and
  [`89d4fa0`](https://github.com/itayinbarr/little-coder/commit/89d4fa0af864230527ab75d12604ed0eb320e6df):
  the v0.0.x layout is described in [The v0.0.x layout](#the-v00x-layout-little-coder) and
  [v0.0.x session files](#v00x-session-files) and parsed, with its gist sync, Telegram relay and
  permission rows; the permission gate's refusals are now `permission decision:` rows; and
  [Source host rows](#source-host-rows) was added.
- 2026-10-19: [`89d4fa0`](https://github.com/itayinbarr/little-coder/commit/89d4fa0af864230527ab75d12604ed0eb320e6df)
  is still the head of `main` and still pins pi `^0.83.0`, so no path changed. The review added
  the permission gate and the phase-model switches to [Record schema](#record-schema), the
  `/deep-research` directory to [Scratch directories](#scratch-directories) and `/clear` to
  [What survives a delete](#what-survives-a-delete).

## Collection

### Per-user storage

#### Shared with pi: `~/.pi/agent`

The launcher and pi both use `PI_CODING_AGENT_DIR` (with `~` expansion) or `homedir()/.pi/agent` on
every OS
([little-coder.mjs:24-35](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/bin/little-coder.mjs#L24-L35);
pi
[packages/coding-agent/src/config.ts:491-496](https://github.com/earendil-works/pi/blob/845d6ff1f6643aba440341cce877ce1c43ebbc39/packages/coding-agent/src/config.ts#L491-L496),
[515-521](https://github.com/earendil-works/pi/blob/845d6ff1f6643aba440341cce877ce1c43ebbc39/packages/coding-agent/src/config.ts#L515-L521)).
In it pi keeps `settings.json`, `auth.json`, `models.json`, `themes/`, `tools/`, `bin/` (fd, rg),
`prompts/`, `pi-debug.log` and `sessions/`
([config.ts:523-566](https://github.com/earendil-works/pi/blob/845d6ff1f6643aba440341cce877ce1c43ebbc39/packages/coding-agent/src/config.ts#L523-L566)).
Sessions are `sessions/--<cwd with / \ : replaced by ->--/<ISO timestamp>_<uuid>.jsonl`
([packages/coding-agent/src/core/session-manager.ts:476-481](https://github.com/earendil-works/pi/blob/845d6ff1f6643aba440341cce877ce1c43ebbc39/packages/coding-agent/src/core/session-manager.ts#L476-L481),
[950-953](https://github.com/earendil-works/pi/blob/845d6ff1f6643aba440341cce877ce1c43ebbc39/packages/coding-agent/src/core/session-manager.ts#L950-L953)),
overridable by `PI_CODING_AGENT_SESSION_DIR`
([config.ts:495-496](https://github.com/earendil-works/pi/blob/845d6ff1f6643aba440341cce877ce1c43ebbc39/packages/coding-agent/src/config.ts#L495-L496)).
little-coder sessions and plain pi sessions share this directory and cannot be told apart by path.
little-coder itself writes:

- `settings.json`: merges `quietStartup: true` and `lastChangelogVersion` on each interactive launch
  ([little-coder.mjs:400-466](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/bin/little-coder.mjs#L400-L466));
  a v1.9.0 keybinding is removed from `keybindings.json`
  ([:468-490](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/bin/little-coder.mjs#L468-L490)).
- `little-coder-prompt-history.json`: the last 100 prompts typed
  ([Transcript stores](#transcript-stores) gives the format).

#### Checkpoints: `~/.little-coder/checkpoints/<session file name>/`

Before a `write` or `edit` tool call, the first time per session, a copy of the target file's
previous contents is saved under a name derived from its path (non `[A-Za-z0-9._-]` characters
become `_`, last 200 characters), or an empty `<name>.absent` if it did not exist
([.pi/extensions/checkpoint/index.ts:6-9](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/.pi/extensions/checkpoint/index.ts#L6-L9),
[25-58](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/.pi/extensions/checkpoint/index.ts#L25-L58)).
The directory is the session file's basename, so it matches a `sessions/` JSONL file
([:63-65](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/.pi/extensions/checkpoint/index.ts#L63-L65)).
`homedir()` on every OS. The extension takes the basename by splitting the session path on `/`
only
([checkpoint/index.ts:64](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/.pi/extensions/checkpoint/index.ts#L64)),
so a Windows session path with backslashes would not be split.

#### The v0.0.x layout: `~/.little-coder`

The retired Python releases kept everything in `Path.home()/.little-coder` on every OS
([config.py:6-14](https://github.com/itayinbarr/little-coder/blob/424f7a4b9f9214a36a6bcb69e1f3435ca3e68d03/config.py#L6-L14)):

- `config.json`: the settings, saved whole by every command that changes one
  ([config.py:16-38](https://github.com/itayinbarr/little-coder/blob/424f7a4b9f9214a36a6bcb69e1f3435ca3e68d03/config.py#L16-L38), [62-66](https://github.com/itayinbarr/little-coder/blob/424f7a4b9f9214a36a6bcb69e1f3435ca3e68d03/config.py#L62-L66)), among them
  `model`, `permission_mode` (`auto`, the default, `accept-all` or `manual`), the per-provider
  `<provider>_api_key`, `gist_token`, `cloudsave_auto` and `cloudsave_last_gist_id`
  ([Sync, relay and origin rows](#sync-relay-and-origin-rows)) and `telegram_token` and
  `telegram_chat_id` ([little_coder.py:2586-2595](https://github.com/itayinbarr/little-coder/blob/424f7a4b9f9214a36a6bcb69e1f3435ca3e68d03/little_coder.py#L2586-L2595)).
- `input_history.txt`: the REPL's readline history, up to 1000 entries, written at exit
  ([config.py:8](https://github.com/itayinbarr/little-coder/blob/424f7a4b9f9214a36a6bcb69e1f3435ca3e68d03/config.py#L8), [little_coder.py:3376-3389](https://github.com/itayinbarr/little-coder/blob/424f7a4b9f9214a36a6bcb69e1f3435ca3e68d03/little_coder.py#L3376-L3389)).
- `sessions/daily/<YYYY-MM-DD>/session_<HHMMSS>_<id>.json`, written at exit and pruned to the
  newest `session_daily_limit` (10 by default) of the day; the same session as
  `sessions/mr_sessions/session_latest.json`, overwritten at each exit; and appended to
  `sessions/history.json`, which keeps the newest `session_history_limit` (200 by default)
  ([little_coder.py:978-1033](https://github.com/itayinbarr/little-coder/blob/424f7a4b9f9214a36a6bcb69e1f3435ca3e68d03/little_coder.py#L978-L1033)). `/save` writes
  `sessions/session_<YYYYMMDD_HHMMSS>_<id>.json`, or a name or path the user gives
  ([966-976](https://github.com/itayinbarr/little-coder/blob/424f7a4b9f9214a36a6bcb69e1f3435ca3e68d03/little_coder.py#L966-L976)). [v0.0.x session files](#v00x-session-files) gives
  the format.
- `checkpoints/<session id>/snapshots.json` and `checkpoints/<session id>/backups/<sha256 of the
  path, 16 hex>@v<N>`, the copies of files the agent edited
  ([checkpoint/store.py:1-8](https://github.com/itayinbarr/little-coder/blob/424f7a4b9f9214a36a6bcb69e1f3435ca3e68d03/checkpoint/store.py#L1-L8)); a directory untouched for 30 days is removed at
  each start ([290-309](https://github.com/itayinbarr/little-coder/blob/424f7a4b9f9214a36a6bcb69e1f3435ca3e68d03/checkpoint/store.py#L290-L309)), and `/checkpoint clear` removes
  the running session's ([2740-2743](https://github.com/itayinbarr/little-coder/blob/424f7a4b9f9214a36a6bcb69e1f3435ca3e68d03/little_coder.py#L2740-L2743)).
- `memory/`, `skills/`, `agents/`, `plugins/` with `plugins.json`, and `mcp.json`
  ([memory/store.py:20](https://github.com/itayinbarr/little-coder/blob/424f7a4b9f9214a36a6bcb69e1f3435ca3e68d03/memory/store.py#L20); [skill/loader.py:30-33](https://github.com/itayinbarr/little-coder/blob/424f7a4b9f9214a36a6bcb69e1f3435ca3e68d03/skill/loader.py#L30-L33); [multi_agent/subagent.py:160](https://github.com/itayinbarr/little-coder/blob/424f7a4b9f9214a36a6bcb69e1f3435ca3e68d03/multi_agent/subagent.py#L160); [plugin/store.py:15-16](https://github.com/itayinbarr/little-coder/blob/424f7a4b9f9214a36a6bcb69e1f3435ca3e68d03/plugin/store.py#L15-L16); [mcp/config.py:34-35](https://github.com/itayinbarr/little-coder/blob/424f7a4b9f9214a36a6bcb69e1f3435ca3e68d03/mcp/config.py#L34-L35)).

The catalog line `little-coder|.little-coder` collects the whole directory, `config.json` flagged
as a secret ([Credentials](#credentials)).

#### Configuration: `~/.config/little-coder/`

`~/.config/little-coder/`, or `$XDG_CONFIG_HOME/little-coder`, using
`HOME` or `USERPROFILE`, so `~/.config` on macOS and Windows too:

- `models.json`, the user's provider override (`LITTLE_CODER_MODELS_FILE` wins)
  ([little-coder.mjs:46-56](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/bin/little-coder.mjs#L46-L56);
  [.pi/extensions/llama-cpp-provider/config.ts:75-82](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/.pi/extensions/llama-cpp-provider/config.ts#L75-L82)).
- `extensions/`, user extensions loaded on every launch (`LITTLE_CODER_EXTENSIONS_DIR` wins)
  ([bin/user-extensions.mjs:17-22](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/bin/user-extensions.mjs#L17-L22),
  [37-51](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/bin/user-extensions.mjs#L37-L51)).

#### Update check: `~/.cache/little-coder/version-check.json`

`~/.cache/little-coder/version-check.json` (or `$XDG_CACHE_HOME`): update check timestamp and latest
version
([bin/update-check.mjs:33-37](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/bin/update-check.mjs#L33-L37)).

#### Package install

The package itself (`.pi/extensions/`, `AGENTS.md` used as the system prompt, `skills/`, bundled
`models.json`) lives in the global npm or bun install, not the home
([little-coder.mjs:163](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/bin/little-coder.mjs#L163),
[314](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/bin/little-coder.mjs#L314)).

#### Scratch directories

None known under the home. little-coder works in the user's cwd. Its one directory of its own,
`/deep-research`'s `<tmpdir>/deep-research-*`, is in the system temporary directory and removed when
the research ends
([.pi/extensions/deep-research/index.ts:167-176](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/.pi/extensions/deep-research/index.ts#L167-L176),
[257](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/.pi/extensions/deep-research/index.ts#L257));
no catalog line can hold it, and its research agents run with `--no-session`, so no record
names it. `~/.little-coder/checkpoints/` holds copies of
project files, not a working directory. What a delete leaves (the checkpoints and the prompt
history) is in [What survives a delete](#what-survives-a-delete).

### Credentials

- `~/.pi/agent/auth.json`, pi's credential store, mode 0600
  ([packages/coding-agent/src/core/auth-storage.ts:21](https://github.com/earendil-works/pi/blob/845d6ff1f6643aba440341cce877ce1c43ebbc39/packages/coding-agent/src/core/auth-storage.ts#L21),
  [31](https://github.com/earendil-works/pi/blob/845d6ff1f6643aba440341cce877ce1c43ebbc39/packages/coding-agent/src/core/auth-storage.ts#L31)).
  Belongs to pi.
- `~/.config/little-coder/models.json`: `providers.<name>.apiKey` is meant to name an environment
  variable, but a value that names none is used as a literal key
  ([config.ts:4-14](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/.pi/extensions/llama-cpp-provider/config.ts#L4-L14),
  [294-300](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/.pi/extensions/llama-cpp-provider/config.ts#L294-L300)).
  Flag it.
- `~/.little-coder/config.json` of the v0.0.x Python releases holds a GitHub token, a Telegram bot
  token and provider API keys ([Secrets](#secrets) names the keys); secret glob
  `.little-coder/config.json` from collector 1.14.0.
- `~/.little-coder/mcp.json` of the same releases takes MCP servers in Claude Code's `.mcp.json`
  format, an SSE server with `headers` such as `Authorization: Bearer`
  ([mcp/config.py:1-21](https://github.com/itayinbarr/little-coder/blob/424f7a4b9f9214a36a6bcb69e1f3435ca3e68d03/mcp/config.py#L1-L21)); flagged
  (`.little-coder/mcp.json`).
- No keychain use.

### Exclusions

None. Checkpoints are copies of project files and can be large on a busy
host, but they are pre-edit evidence; keep them.

### Project-local files

- `.pi/approved-plan.md`, written by the plan mode in the session's cwd
  ([.pi/extensions/plan-mode/index.ts:20](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/.pi/extensions/plan-mode/index.ts#L20),
  [740-742](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/.pi/extensions/plan-mode/index.ts#L740-L742)).
- Reads `AGENTS.md`, then `CLAUDE.md`, from the project
  ([.pi/extensions/project-context/discover.ts:21-27](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/.pi/extensions/project-context/discover.ts#L21-L27));
  pi's own project context loading is turned off with `--no-context-files`, and project
  `.pi/extensions` load only with `--with-pi-extensions`
  ([little-coder.mjs:222-228](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/bin/little-coder.mjs#L222-L228),
  [365-372](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/bin/little-coder.mjs#L365-L372)).
- The v0.0.x releases kept tasks in `<cwd>/.little-coder/tasks.json`
  ([task/store.py:24-25](https://github.com/itayinbarr/little-coder/blob/424f7a4b9f9214a36a6bcb69e1f3435ca3e68d03/task/store.py#L24-L25)) and read project skills from `<cwd>/.little-coder/skills`
  ([skill/loader.py:30-33](https://github.com/itayinbarr/little-coder/blob/424f7a4b9f9214a36a6bcb69e1f3435ca3e68d03/skill/loader.py#L30-L33)) and MCP servers from `<cwd>/.mcp.json`
  ([mcp/config.py:35](https://github.com/itayinbarr/little-coder/blob/424f7a4b9f9214a36a6bcb69e1f3435ca3e68d03/mcp/config.py#L35)).
- `/deep-research` writes its report into the cwd
  ([.pi/extensions/deep-research/index.ts:366-367](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/.pi/extensions/deep-research/index.ts#L366-L367)).

### Where the project path is recorded

The pi session header line: `{"type":"session",...,"cwd":...}`
([session-manager.ts:32-39](https://github.com/earendil-works/pi/blob/845d6ff1f6643aba440341cce877ce1c43ebbc39/packages/coding-agent/src/core/session-manager.ts#L32-L39),
[936-944](https://github.com/earendil-works/pi/blob/845d6ff1f6643aba440341cce877ce1c43ebbc39/packages/coding-agent/src/core/session-manager.ts#L936-L944)),
and the encoded session directory name. Checkpoint file names encode absolute paths, but lossily
(`/` becomes `_`). The v0.0.x session files record no working directory; their checkpoint
snapshots name each backed-up file by its absolute path
([v0.0.x session files](#v00x-session-files)).

## Parsing

### Transcript stores

- `.pi/agent/sessions/--<encoded cwd>--/<ISO timestamp with - for : and .>_<uuid>.jsonl`, one file
  per session, appended per entry, shared with plain pi; the naming, its citations and the
  override are in [Shared with pi](#shared-with-pi-piagent).
- `.pi/agent/little-coder-prompt-history.json`: JSON array of up to 100 prompt strings, oldest
  first, no timestamps or session ids
  ([.pi/extensions/prompt-history/index.ts:23-26](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/.pi/extensions/prompt-history/index.ts#L23-L26),
  [38-40](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/.pi/extensions/prompt-history/index.ts#L38-L40),
  [52-57](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/.pi/extensions/prompt-history/index.ts#L52-L57)).
- `.little-coder/checkpoints/<session file name>/<mangled path>`: pre-edit file copies, not
  transcripts, but they join to a session by name
  ([.pi/extensions/checkpoint/index.ts:25-29](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/.pi/extensions/checkpoint/index.ts#L25-L29);
  the naming is in [Checkpoints](#checkpoints-little-codercheckpointssession-file-name)).

- v0.0.x: the saved session files under `.little-coder/sessions/`, `input_history.txt` and the
  checkpoint snapshots of [The v0.0.x layout](#the-v00x-layout-little-coder).

Sub-agents run pi with `--no-session`, so they leave no file
([.pi/extensions/subagent/spawn.ts:14](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/.pi/extensions/subagent/spawn.ts#L14),
[318-322](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/.pi/extensions/subagent/spawn.ts#L318-L322)).

#### What survives a delete

little-coder has no delete of its own: `/clear` is an alias for pi's new-session command and deletes
nothing
([.pi/extensions/clear-command/index.ts:18-23](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/.pi/extensions/clear-command/index.ts#L18-L23)).
A session file is removed only through pi's own session picker
([What survives a delete in `pi.md`](pi.md#what-survives-a-delete)), which leaves two things behind:
the session's checkpoint directory `.little-coder/checkpoints/<session file name>/`, whose name
still joins its pre-edit file copies to the session, and the prompt history
`little-coder-prompt-history.json`, the last 100 prompts with no time and no session id
([.pi/extensions/prompt-history/index.ts:26](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/.pi/extensions/prompt-history/index.ts#L26)).
No deletion marker is written. The analyzer reads both: the prompt history as `user` rows with no
timestamp and no session ([Past use and secondary sources](#past-use-rows-and-secondary-sources)),
and each checkpoint directory as one `system` row naming the session and the files
([Parser plan](#parser-plan)).

The v0.0.x releases delete nothing a user asks for: `/clear` empties the conversation in memory
([little_coder.py:934-938](https://github.com/itayinbarr/little-coder/blob/424f7a4b9f9214a36a6bcb69e1f3435ca3e68d03/little_coder.py#L934-L938)). What goes is pruning: a day's daily copies beyond
the limit and `history.json` sessions beyond its limit, so a session pruned from `daily/` stays in
`history.json`, and one pruned from both may still be the last `session_latest.json`; checkpoint
directories older than 30 days go at the next start
([The v0.0.x layout](#the-v00x-layout-little-coder)).
`input_history.txt` keeps the prompts of pruned sessions, and the gist named in `config.json`
keeps the last upload on GitHub. The analyzer reads every one of these copies, the daily and
`/save` files as primary and `session_latest.json`, `history.json` and `input_history.txt` as
secondary ([Secondary sources](../analyzer/docs/output.md#secondary-sources)).

### Record schema

pi session format version 3
([session-manager.ts:30](https://github.com/earendil-works/pi/blob/845d6ff1f6643aba440341cce877ce1c43ebbc39/packages/coding-agent/src/core/session-manager.ts#L30)).
Line 1 is the header `{type:"session", version, id, timestamp (ISO), cwd, parentSession?}`
([:32-39](https://github.com/earendil-works/pi/blob/845d6ff1f6643aba440341cce877ce1c43ebbc39/packages/coding-agent/src/core/session-manager.ts#L32-L39)).
Every other line has `type`, `id`, `parentId`, `timestamp` (ISO)
([:46-51](https://github.com/earendil-works/pi/blob/845d6ff1f6643aba440341cce877ce1c43ebbc39/packages/coding-agent/src/core/session-manager.ts#L46-L51))
and is one of `message`, `thinking_level_change`, `model_change` (`provider`, `modelId`),
`compaction`, `branch_summary`, `custom`, `custom_message`, `label`, `session_info`
([:53-153](https://github.com/earendil-works/pi/blob/845d6ff1f6643aba440341cce877ce1c43ebbc39/packages/coding-agent/src/core/session-manager.ts#L53-L153)).

`message.message`
([packages/ai/src/types.ts:393-431](https://github.com/earendil-works/pi/blob/845d6ff1f6643aba440341cce877ce1c43ebbc39/packages/ai/src/types.ts#L393-L431)):

- `user`: `content` string or `text`/`image` parts, `timestamp` (Unix ms).
- `assistant`: `content[]` of `text`, `thinking` (`thinking`), `toolCall` (`id`, `name`, `arguments`
  object)
  ([:338-366](https://github.com/earendil-works/pi/blob/845d6ff1f6643aba440341cce877ce1c43ebbc39/packages/ai/src/types.ts#L338-L366));
  `provider`, `model`, `usage`, `stopReason` (`stop|length|toolUse|error|aborted`,
  [:391](https://github.com/earendil-works/pi/blob/845d6ff1f6643aba440341cce877ce1c43ebbc39/packages/ai/src/types.ts#L391)),
  `errorMessage`, `timestamp` (ms).
- `toolResult`: `toolCallId`, `toolName`, `content[]`, `isError`, `timestamp`.
- pi's own roles `bashExecution` (`command`, `output`, `exitCode`), `custom`, `branchSummary`,
  `compactionSummary`
  ([packages/coding-agent/src/core/messages.ts:29-67](https://github.com/earendil-works/pi/blob/845d6ff1f6643aba440341cce877ce1c43ebbc39/packages/coding-agent/src/core/messages.ts#L29-L67)).

Extension messages become `custom_message` entries with `customType`, `content`, `display`,
`details`
([session-manager.ts:135-141](https://github.com/earendil-works/pi/blob/845d6ff1f6643aba440341cce877ce1c43ebbc39/packages/coding-agent/src/core/session-manager.ts#L135-L141);
[packages/coding-agent/src/core/agent-session.ts:624-633](https://github.com/earendil-works/pi/blob/845d6ff1f6643aba440341cce877ce1c43ebbc39/packages/coding-agent/src/core/agent-session.ts#L624-L633)).

#### little-coder markers

Its extensions tag what they inject: `lc-skills`, `lc-knowledge`, `lc-project-context`
([.pi/extensions/_shared/inject.ts:49-64](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/.pi/extensions/_shared/inject.ts#L49-L64);
[skill-inject/index.ts:393](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/.pi/extensions/skill-inject/index.ts#L393);
[knowledge-inject/index.ts:169](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/.pi/extensions/knowledge-inject/index.ts#L169);
[project-context/index.ts:96](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/.pi/extensions/project-context/index.ts#L96)),
`lc-bg-shell` with `details.id`
([bg-shell/index.ts:134-135](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/.pi/extensions/bg-shell/index.ts#L134-L135)),
`lc-plan`, `lc-approved-plan`
([plan-mode/index.ts:394](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/.pi/extensions/plan-mode/index.ts#L394),
[817](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/.pi/extensions/plan-mode/index.ts#L817)),
`lc-research`
([deep-research/index.ts:334](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/.pi/extensions/deep-research/index.ts#L334)).
Injection can instead go into the system prompt (`injectMode` `system`), which leaves no entry
([inject.ts:60-62](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/.pi/extensions/_shared/inject.ts#L60-L62)).
Assistant `provider` is commonly `llamacpp`, little-coder's local provider
([.pi/extensions/llama-cpp-provider/index.ts:172](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/.pi/extensions/llama-cpp-provider/index.ts#L172);
[models.json:2-4](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/models.json#L2-L4)).

#### Permission gate

The `permission-gate` extension decides each shell and edit call by the mode in
`LITTLE_CODER_PERMISSION_MODE` (`auto`, the default, `accept-all` or `manual`) and the extra
allowed command prefixes in `LITTLE_CODER_BASH_ALLOW`
([.pi/extensions/permission-gate/index.ts:5-19](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/.pi/extensions/permission-gate/index.ts#L5-L19),
[268-272](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/.pi/extensions/permission-gate/index.ts#L268-L272));
both are environment variables, so no file holds the standing state. In `manual` mode the user
is asked per call: edits offer Apply, Deny and "Apply all (this session)", the last kept in
memory
([permission-gate/manual-edit-confirm.ts:5-37](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/.pi/extensions/permission-gate/manual-edit-confirm.ts#L5-L37)),
shell commands a yes or no. A refusal blocks the call with a reason, `edit cancelled by user`,
`command cancelled by user`, or in `auto` mode a text starting `shell whitelist:`
([index.ts:288](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/.pi/extensions/permission-gate/index.ts#L288),
[303](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/.pi/extensions/permission-gate/index.ts#L303),
[315](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/.pi/extensions/permission-gate/index.ts#L315),
[330](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/.pi/extensions/permission-gate/index.ts#L330),
[354](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/.pi/extensions/permission-gate/index.ts#L354)),
and pi 0.83.0 records a blocked call as a `toolResult` with that reason as its text and
`isError` true
([packages/agent/src/agent-loop.ts:636-642](https://github.com/earendil-works/pi/blob/845d6ff1f6643aba440341cce877ce1c43ebbc39/packages/agent/src/agent-loop.ts#L636-L642),
[756-761](https://github.com/earendil-works/pi/blob/845d6ff1f6643aba440341cce877ce1c43ebbc39/packages/agent/src/agent-loop.ts#L756-L761)).
An approval leaves no record beyond the call's normal result.

#### v0.0.x session files

A saved session ([little_coder.py:1331-1348](https://github.com/itayinbarr/little-coder/blob/424f7a4b9f9214a36a6bcb69e1f3435ca3e68d03/little_coder.py#L1331-L1348)) is one JSON object written with
`indent=2`: `session_id` (8 hex characters, a fresh random value at each save, so the daily copy,
`session_latest.json` and the `history.json` entry of one exit share it, and nothing else does),
`saved_at` (`YYYY-MM-DD HH:MM:SS`, host local time with no zone, the save time), `messages`,
`turn_count`, `total_input_tokens`, `total_output_tokens`. `history.json` is
`{total_turns, sessions: [<session>, ...]}` ([1013-1028](https://github.com/itayinbarr/little-coder/blob/424f7a4b9f9214a36a6bcb69e1f3435ca3e68d03/little_coder.py#L1013-L1028)).

`messages` hold the neutral format
([providers.py:254-258](https://github.com/itayinbarr/little-coder/blob/424f7a4b9f9214a36a6bcb69e1f3435ca3e68d03/providers.py#L254-L258)): `{role: "user", content, images?}`
([agent.py:88-93](https://github.com/itayinbarr/little-coder/blob/424f7a4b9f9214a36a6bcb69e1f3435ca3e68d03/agent.py#L88-L93)), `{role: "assistant", content, tool_calls: [{id, name,
input}]}` ([207-211](https://github.com/itayinbarr/little-coder/blob/424f7a4b9f9214a36a6bcb69e1f3435ca3e68d03/agent.py#L207-L211)) and `{role: "tool", tool_call_id, name, content}`
([243-248](https://github.com/itayinbarr/little-coder/blob/424f7a4b9f9214a36a6bcb69e1f3435ca3e68d03/agent.py#L243-L248)). No message has a time, a model or a working directory.
A call the user refused is recorded as a `tool` message whose content is `Denied: user rejected
this operation` ([226-233](https://github.com/itayinbarr/little-coder/blob/424f7a4b9f9214a36a6bcb69e1f3435ca3e68d03/agent.py#L226-L233)); whether to ask follows `permission_mode`
([252-270](https://github.com/itayinbarr/little-coder/blob/424f7a4b9f9214a36a6bcb69e1f3435ca3e68d03/agent.py#L252-L270)), and answering `a` sets `accept-all` for the rest of the
session in memory, which the next save of `config.json` keeps
([little_coder.py:585-597](https://github.com/itayinbarr/little-coder/blob/424f7a4b9f9214a36a6bcb69e1f3435ca3e68d03/little_coder.py#L585-L597)). Prompts that arrive through the Telegram bridge
enter the same conversation with no marker ([2495-2510](https://github.com/itayinbarr/little-coder/blob/424f7a4b9f9214a36a6bcb69e1f3435ca3e68d03/little_coder.py#L2495-L2510)).

A snapshot in `checkpoints/<session id>/snapshots.json` is `{id, session_id, created_at (naive
local ISO), turn_count, message_index, user_prompt_preview (the first 80 characters of the
prompt), token_snapshot, file_backups: {<absolute path>: {backup_filename (null when the file did
not exist), version, backup_time}}}` ([checkpoint/types.py:40-63](https://github.com/itayinbarr/little-coder/blob/424f7a4b9f9214a36a6bcb69e1f3435ca3e68d03/checkpoint/types.py#L40-L63),
[116-196](https://github.com/itayinbarr/little-coder/blob/424f7a4b9f9214a36a6bcb69e1f3435ca3e68d03/checkpoint/store.py#L116-L196)). Its session id is the REPL's runtime id, made at
start ([3450-3457](https://github.com/itayinbarr/little-coder/blob/424f7a4b9f9214a36a6bcb69e1f3435ca3e68d03/little_coder.py#L3450-L3457)), which is not the id of any saved session
file.

#### Model switches

The `phase-model` extension switches the model at the plan and implementation phases through
`pi.setModel`
([.pi/extensions/phase-model/index.ts:169](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/.pi/extensions/phase-model/index.ts#L169)),
and pi 0.83.0's `setModel` appends a `model_change` entry
([packages/coding-agent/src/core/agent-session.ts:1578-1586](https://github.com/earendil-works/pi/blob/845d6ff1f6643aba440341cce877ce1c43ebbc39/packages/coding-agent/src/core/agent-session.ts#L1578-L1586)).

### Joins and timestamps

Session id: header `id`, also in the file name. Tree: `parentId` → `id`.
Tool use to result: `toolCall.id` = `toolResult.toolCallId`. Project path:
header `cwd`. Forked sessions: header `parentSession`. Checkpoint directory
name = session file basename.

v0.0.x: session id `session_id`, shared only by the copies of one save; tool use to result
`tool_calls[].id` = `tool_call_id`; no project path; the only time is the session's `saved_at`,
host local time to the second, which the analyzer emits as if it were UTC because the host's
offset is not recorded. A `/load` or `/resume` carries an earlier session's messages into the
next save under a new id, so one turn can appear in several primary files. The checkpoint
snapshots' session id and the gist's uploaded session id each match no saved file.

### Other stores

little-coder keeps no database, index or log beside the prompt history and the checkpoints of
[Transcript stores](#transcript-stores). Images are base64 inside `image` parts
([types.ts:354-358](https://github.com/earendil-works/pi/blob/845d6ff1f6643aba440341cce877ce1c43ebbc39/packages/ai/src/types.ts#L354-L358)).

#### v0.0.x input history

`input_history.txt` is the readline history of [The v0.0.x layout](#the-v00x-layout-little-coder),
one entry per line as Python's `readline.write_history_file` writes it, with no time, session or
project.

### Format versions

pi header `version` (absent in v1 files, 3 now,
[session-manager.ts:30-35](https://github.com/earendil-works/pi/blob/845d6ff1f6643aba440341cce877ce1c43ebbc39/packages/coding-agent/src/core/session-manager.ts#L30-L35)).
little-coder pins pi 0.83.x; a release that moves to pi 1.x should be re-checked against pi's
research. The v0.0.x releases were a separate Python program with their own formats
([v0.0.x session files](#v00x-session-files)); the move to the pi launcher left `~/.little-coder`
in place.

### Secrets

Session files store tool inputs and outputs verbatim; `thinkingSignature` and `thoughtSignature` are
opaque provider blobs
([types.ts:344-366](https://github.com/earendil-works/pi/blob/845d6ff1f6643aba440341cce877ce1c43ebbc39/packages/ai/src/types.ts#L344-L366)).
Credentials are in `auth.json`, not in sessions.

The v0.0.x Python releases kept, in `~/.little-coder/config.json`, the provider keys as
`<provider>_api_key` ([config.py:29-36](https://github.com/itayinbarr/little-coder/blob/424f7a4b9f9214a36a6bcb69e1f3435ca3e68d03/config.py#L29-L36)), the Telegram bot token as
`telegram_token` and a GitHub personal access token with the `gist` scope as `gist_token`
([cloudsave.py:9-12](https://github.com/itayinbarr/little-coder/blob/424f7a4b9f9214a36a6bcb69e1f3435ca3e68d03/cloudsave.py#L9-L12),
[little_coder.py:1371-1384](https://github.com/itayinbarr/little-coder/blob/424f7a4b9f9214a36a6bcb69e1f3435ca3e68d03/little_coder.py#L1371-L1384),
[config.py:6-7](https://github.com/itayinbarr/little-coder/blob/424f7a4b9f9214a36a6bcb69e1f3435ca3e68d03/config.py#L6-L7)).

### Parser plan

The pi parser reads `.pi/agent/sessions/*/*.jsonl`; the rows stay agent `pi`, as the manifest
says, and a session with any `custom_message` whose `customType` starts with `lc-`, an assistant
`provider` of `llamacpp`, or a checkpoint directory named after its file gets one `system` row
`session driven by little-coder (pi launcher): <why>`. The little-coder parser reads
`little-coder-prompt-history.json` as `user` rows with no timestamp and no session, and each
checkpoint directory as one `system` row naming the session and the files.

| Column | Source |
| --- | --- |
| timestamp_utc | entry `timestamp` (ISO); `message.timestamp` (ms) as fallback |
| session_id | header `id` |
| project_path | header `cwd` |
| git_branch | empty |
| turn_type | `user`, `assistant`; `toolCall` → tool_use; `toolResult` → tool_result; `thinking` → thinking; `custom_message`, `model_change`, `compaction`, `bashExecution` → system |
| model | assistant `provider`/`model`; a `model: <id> provider=<provider>` `system` row at the first assistant message that names a model and `model: <old> -> <new> provider=<provider>` on each change, from pi's reader ([Model in `pi.md`](pi.md#model-rows)) |
| tool_name, tool_use_id | `toolCall.name`/`id`; `toolResult.toolName`/`toolCallId` |
| text | text parts; tool_use: `arguments.command` or `path`, else JSON; tool_result: text parts |

#### Permission rows

The refusals of the [permission gate](#permission-gate) are on disk as error `toolResult` entries
whose text is `edit cancelled by user`, `command cancelled by user` or starts `shell whitelist:`.
The pi parser follows each one's `tool_result` row with `permission decision: denied (<the
text>)`, carrying the call's `tool_name` and `tool_use_id`, under agent `pi`
([Permission rows in `pi.md`](pi.md#permission-rows)). There is no standing state on disk to read:
the mode and the allow list are environment variables.

v0.0.x: a refused call's `tool` message `Denied: user rejected this operation` is followed by
`permission decision: denied (Denied: user rejected this operation)` with the call's `tool_name`
and `tool_use_id`, and `config.json` `permission_mode` is one `system` row `permission setting:
permission_mode=<value> (from .little-coder/config.json, file mtime)` at the file's modification
time ([v0.0.x session files](#v00x-session-files)). All these rows are source-derived; not observed
on any real install.

#### Model rows

Covered through pi's `model_change` rows: the [phase-model switches](#model-switches) become
`model_change` entries, which pi's reader turns into `model:` rows
([Model in `pi.md`](pi.md#model-rows)). The v0.0.x session files record no model; `config.json`
`model` is the configured default, not a session's, and is not read.

#### Past-use rows and secondary sources

The prompt history is read on every run, as above; it is primary, since its rows (agent
`little-coder`) cannot pair with the pi sessions' (agent `pi`). The checkpoints of
[Transcript stores](#transcript-stores) are covered past use.

v0.0.x, source-derived; not observed on any real install: the daily and `/save` session files are
primary; `session_latest.json` and `history.json`, which repeat them under the same session id and
`saved_at`, and `input_history.txt` (`user` rows with no time, session or project, matched on text
alone) are secondary. Each session gives a `session saved: little-coder v0.0.x session <id>
turn_count=<n> input_tokens=<n> output_tokens=<n> (saved_at <saved_at>, host local time)` row and
its messages, all at `saved_at`, with an empty `project_path`. Each checkpoint snapshot is one
`system` row `little-coder v0.0.x checkpoint <id>: turn_count=<n> prompt=<user_prompt_preview>
files=<path> (backup <name>|did not exist); ... (created_at host local time)` at `created_at`,
with the snapshot's session id; the copies under `backups/` are not read.

#### Sync, relay and origin rows

On the pi it pins (0.83.x) little-coder inherits pi's `/share`, which uploads a session as a secret
GitHub gist through `gh` and leaves no local marker; pi's `/bug` upload and its `pi.bug-report`
entry arrived after 0.83.0
([0.83.0 interactive-mode.ts:5523-5608](https://github.com/earendil-works/pi/blob/845d6ff1f6643aba440341cce877ce1c43ebbc39/packages/coding-agent/src/modes/interactive/interactive-mode.ts#L5523-L5608);
[Sync, relay and origin in `pi.md`](pi.md#sync-relay-and-origin-rows)). little-coder's own
extensions add no upload: a search of `bin/` and `.pi/extensions/` for `gist`, `upload`, `share`,
`telemetry` and `fetch(` finds only the npm update check
([bin/update-check.mjs:106](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/bin/update-check.mjs#L106))
and the research and browser tools. The v0.0.x Python releases had `/cloudsave`, opt-in:
`/cloudsave setup <token>` stores the token, `/cloudsave` or `/cloudsave push [desc]` uploads the
conversation as a private gist described `[little-coder] <desc or time>` holding
`little-coder_session_<time>.json`, and `/cloudsave auto on` uploads at `/exit`
([cloudsave.py:73-101](https://github.com/itayinbarr/little-coder/blob/424f7a4b9f9214a36a6bcb69e1f3435ca3e68d03/cloudsave.py#L73-L101),
[little_coder.py:1386-1400](https://github.com/itayinbarr/little-coder/blob/424f7a4b9f9214a36a6bcb69e1f3435ca3e68d03/little_coder.py#L1386-L1400),
[1443-1459](https://github.com/itayinbarr/little-coder/blob/424f7a4b9f9214a36a6bcb69e1f3435ca3e68d03/little_coder.py#L1443-L1459),
[1471-1486](https://github.com/itayinbarr/little-coder/blob/424f7a4b9f9214a36a6bcb69e1f3435ca3e68d03/little_coder.py#L1471-L1486)).
Each upload records only the newest gist id, `cloudsave_last_gist_id` in
`~/.little-coder/config.json`, and later pushes update that same gist in place, so the key names one
gist for every upload since the first
([little_coder.py:1451-1457](https://github.com/itayinbarr/little-coder/blob/424f7a4b9f9214a36a6bcb69e1f3435ca3e68d03/little_coder.py#L1451-L1457));
the uploaded `session_id` is a fresh random 8-hex value that matches no local session file
([little_coder.py:1331-1336](https://github.com/itayinbarr/little-coder/blob/424f7a4b9f9214a36a6bcb69e1f3435ca3e68d03/little_coder.py#L1331-L1336)).
The automatic path passes `existing_gist_id=`, a keyword `upload_session` does not take, so it fails
with a `TypeError` before any upload
([little_coder.py:1477-1480](https://github.com/itayinbarr/little-coder/blob/424f7a4b9f9214a36a6bcb69e1f3435ca3e68d03/little_coder.py#L1477-L1480),
[cloudsave.py:73-78](https://github.com/itayinbarr/little-coder/blob/424f7a4b9f9214a36a6bcb69e1f3435ca3e68d03/cloudsave.py#L73-L78)).
The parser reads `cloudsave_last_gist_id` and no other secret key of the file, and writes one
`system` row `session synced: gist <cloudsave_last_gist_id> (from .little-coder/config.json, file
mtime)`, at the file's modification time, with an empty session id.

The v0.0.x Telegram bridge (`/telegram <bot token> <chat id>`) carries prompts from one Telegram
chat into the running session and sends the replies back
([little_coder.py:2344](https://github.com/itayinbarr/little-coder/blob/424f7a4b9f9214a36a6bcb69e1f3435ca3e68d03/little_coder.py#L2344), [2586-2595](https://github.com/itayinbarr/little-coder/blob/424f7a4b9f9214a36a6bcb69e1f3435ca3e68d03/little_coder.py#L2586-L2595)), so
`config.json` `telegram_chat_id` gives one `system` row `session relayed: telegram chat
<telegram_chat_id> (from .little-coder/config.json, file mtime)`, with an empty session id;
`telegram_token` is not read. Neither row names a saved session: the gist's upload has a session id
of its own and the bridge marks no message, so the parser leaves `records_sync` and
`records_relay` unset rather than call every other session unsynced. Both rows are
source-derived; not observed on any real install.

The current releases sync nothing of their own, and no session is relayed or of remote origin.

#### Scratch directory rows

None in the home. `/deep-research`'s `<tmpdir>/deep-research-*` directory is removed when done
and no record names it ([Scratch directories](#scratch-directories)).

#### Deletion rows

No marker; see [What survives a delete](#what-survives-a-delete). The v0.0.x releases write none
either; their pruning is described there.

#### Bad-line rows

Covered: the session files go through pi's reader
([Bad lines in `pi.md`](pi.md#bad-line-rows)), and the prompt history reader reports a bad record.
A v0.0.x session file or `history.json` cut mid-write yields the messages or sessions before the
cut and one `parser: unparseable record <N> (<reason>): <the rest>` row; a `snapshots.json` cut
the same way does too, and a `config.json` that is not a JSON object is one `parser:` row. A
v0.0.x session file, `history.json`, `snapshots.json` or the prompt history that cannot be read is
one whole-file row `parser: <file> could not be read (<reason>)` through `error_rows`, `<file>` its
name.

A pi session file a checkpoint directory names that cannot be read gives the checkpoint row without
the `session file not collected` clause, followed by one row
`parser: <file> could not be read (<reason>)`, `<file>` the session file's name.

#### Fixtures

Fixture:

```json
{"type":"session","version":3,"id":"0b9e2f4c-1111-4000-8000-000000000001","timestamp":"2026-10-01T09:00:00.000Z","cwd":"/home/alice/proj"}
{"type":"custom_message","id":"a1","parentId":null,"timestamp":"2026-10-01T09:00:01.000Z","customType":"lc-skills","content":"## Skill: edit\nUse the edit tool.","display":false}
{"type":"message","id":"a2","parentId":"a1","timestamp":"2026-10-01T09:00:02.000Z","message":{"role":"user","content":"fix the failing test","timestamp":1759309202000}}
{"type":"message","id":"a3","parentId":"a2","timestamp":"2026-10-01T09:00:05.000Z","message":{"role":"assistant","content":[{"type":"thinking","thinking":"run tests first"},{"type":"toolCall","id":"call_1","name":"bash","arguments":{"command":"npm test"}}],"api":"openai-completions","provider":"llamacpp","model":"qwen3.6-35b-a3b","usage":{"input":900,"output":40,"cacheRead":0,"cacheWrite":0},"stopReason":"toolUse","timestamp":1759309205000}}
{"type":"message","id":"a4","parentId":"a3","timestamp":"2026-10-01T09:00:09.000Z","message":{"role":"toolResult","toolCallId":"call_1","toolName":"bash","content":[{"type":"text","text":"1 failing"}],"isError":false,"timestamp":1759309209000}}
```

`little-coder-prompt-history.json`: `["fix the failing test","now run lint"]`

v0.0.x, `.little-coder/sessions/daily/2026-09-20/session_101500_a1b2c3d4.json` (also the content of
`session_latest.json` and the second entry of `history.json`):

```json
{"session_id": "a1b2c3d4", "saved_at": "2026-09-20 10:15:00", "messages": [{"role": "user", "content": "list the files"}, {"role": "assistant", "content": "", "tool_calls": [{"id": "toolu_l1", "name": "Bash", "input": {"command": "ls"}}]}, {"role": "tool", "tool_call_id": "toolu_l1", "name": "Bash", "content": "README.md"}, {"role": "assistant", "content": "One file.", "tool_calls": [{"id": "toolu_l2", "name": "Write", "input": {"file_path": "/srv/proj/b.py", "content": "x"}}]}, {"role": "tool", "tool_call_id": "toolu_l2", "name": "Write", "content": "Denied: user rejected this operation"}], "turn_count": 1, "total_input_tokens": 1200, "total_output_tokens": 60}
```

`.little-coder/config.json`, the tokens replaced:

```json
{"model": "ollama/qwen3.5", "permission_mode": "manual", "gist_token": "<redacted>", "cloudsave_last_gist_id": "aa11bb22cc33dd44ee55ff6677889900", "telegram_token": "<redacted>", "telegram_chat_id": 424242}
```

`.little-coder/checkpoints/c0ffee12/snapshots.json`, second entry:

```json
{"id": 2, "session_id": "c0ffee12", "created_at": "2026-09-20T10:14:30.500000", "turn_count": 1, "message_index": 5, "user_prompt_preview": "list the files", "token_snapshot": {"input": 1200, "output": 60}, "file_backups": {"/srv/proj/a.py": {"backup_filename": "0123456789abcdef@v1", "version": 1, "backup_time": "2026-09-20T10:14:30.500000"}}}
```

#### Source host rows

The current releases record no source-host integration: a search of `bin/` and `.pi/extensions/`
at [`89d4fa0`](https://github.com/itayinbarr/little-coder/commit/89d4fa0af864230527ab75d12604ed0eb320e6df)
for `github`, `gitlab`, `bitbucket`, `oauth`, `repositor`, `pull request` and `clone` finds the
update check and the research tools only; pi's own finding is in
[Source host rows in `pi.md`](pi.md#source-host-rows). The v0.0.x releases kept a GitHub personal
access token, `gist_token` in `~/.little-coder/config.json` ([Secrets](#secrets)), to upload
sessions as gists; `/cloudsave setup` checks it against `GET /user` and shows the login without
storing it ([cloudsave.py:61-70](https://github.com/itayinbarr/little-coder/blob/424f7a4b9f9214a36a6bcb69e1f3435ca3e68d03/cloudsave.py#L61-L70), [little_coder.py:1371-1384](https://github.com/itayinbarr/little-coder/blob/424f7a4b9f9214a36a6bcb69e1f3435ca3e68d03/little_coder.py#L1371-L1384)),
so no account is on disk and no `source host:` row is written. The token is flagged with the whole
file and never read; no repository list or repository-bound conversation is stored.

## Confidence

- High (source): every path in [Per-user storage](#per-user-storage), the environment overrides,
  the checkpoint naming, the prompt history format, and that the sessions are pi's, with pi's
  file locations, header and entry fields and message roles read at its `v0.83.0` tag; a later
  little-coder that moves to pi 1.x may differ.
- High (source): the `lc-*` custom types of [little-coder markers](#little-coder-markers), and
  that sub-agents leave no session file.
- Medium: that every little-coder session contains an `lc-` entry; with injection set to
  `system` mode and no plan, bg-shell or research use, a session may carry none, and then only
  `provider: "llamacpp"` hints at it.
- Medium: the checkpoint directory name on Windows
  ([Checkpoints](#checkpoints-little-codercheckpointssession-file-name)).
- Not determined: the exact `usage` sub-fields beyond those shown
  ([types.ts:368-380](https://github.com/earendil-works/pi/blob/845d6ff1f6643aba440341cce877ce1c43ebbc39/packages/ai/src/types.ts#L368-L380)).
- Not determined: real sizes of `checkpoints/`.
- Medium: the v0.0.x `input_history.txt` encoding, which is that of the readline library Python
  links on the host, not stated by little-coder's source.
- Not determined: whether pi project files beyond `.pi/approved-plan.md` are written when
  `--with-pi-extensions` is used; that is pi's behaviour ([pi](pi.md)).
