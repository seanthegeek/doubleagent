# pi

pi is the `pi` coding agent CLI from Earendil Works (`@earendil-works/pi-coding-agent`), a
terminal agent with one per-user directory, `~/.pi/agent`, on every OS, where it writes one
append-only JSONL file per session, a tree of entries linked by `id`/`parentId`. Two other
catalog agents reuse its storage and format: `little-coder`, a launcher that runs pi itself and
writes into the same `~/.pi/agent` (see [little-coder.md](little-coder.md)), and Letta Code's
local backend, which writes pi's session format with its own writer into its own directory (see
[letta.md](letta.md)); [Wrapper attribution](#wrapper-attribution) says how to tell them apart.
Catalog agent: `pi`. Parsed: the pi parser reads the session files, `trust.json`, the
`defaultProjectTrust` setting, the debug and crash logs and the experimental session metadata.

## Source and evidence

[earendil-works/pi](https://github.com/earendil-works/pi) (the monorepo for the `pi` coding agent
CLI, `@earendil-works/pi-coding-agent` 1.0.2), MIT licence
([LICENSE:1](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/LICENSE#L1)),
open source, TypeScript. Reviewed commit
[`200387122ca450d6387f033949423114a270b96c`](https://github.com/earendil-works/pi/commit/200387122ca450d6387f033949423114a270b96c)
(2026-10-04). Where the layout or the schema differs, tag `v0.83.0`
([`845d6ff1f6643aba440341cce877ce1c43ebbc39`](https://github.com/earendil-works/pi/commit/845d6ff1f6643aba440341cce877ce1c43ebbc39)),
the version little-coder pins, is cited. All claims from source.

### Observed

- No real install: nothing was installed or run. No real install has shown a `pi.bug-report`
  entry, so the rows built from it are fixture only
  ([Sync, relay and origin](#sync-relay-and-origin-rows)).

### Source reviews

- 2026-10-21: [earendil-works/pi](https://github.com/earendil-works/pi) at
  [`5c1736f`](https://github.com/earendil-works/pi/commit/5c1736f82d201d09664642ab0d88147da94b450e)
  (coding-agent 1.3.0) and tag `v0.83.0`: the parser now reads `defaultProjectTrust`,
  `pi-debug.log`, `crashes.json` and little-coder's permission-gate refusals, so
  [Permission rows](#permission-rows) and
  [Past-use rows and secondary sources](#past-use-rows-and-secondary-sources) say what it writes;
  `pi-debug.log` turned out to exist at v0.83.0 too, and
  [Source host rows](#source-host-rows) was added.
- 2026-10-19: [earendil-works/pi](https://github.com/earendil-works/pi) at [`5c1736f`](https://github.com/earendil-works/pi/commit/5c1736f82d201d09664642ab0d88147da94b450e), the head of
  `main` (coding-agent 1.3.0,
  [packages/coding-agent/package.json:3](https://github.com/earendil-works/pi/blob/5c1736f82d201d09664642ab0d88147da94b450e/packages/coding-agent/package.json#L3)),
  compared file by file with [`2003871`](https://github.com/earendil-works/pi/commit/200387122ca450d6387f033949423114a270b96c). The review added `defaultProjectTrust` and `--approve` to
  [Project trust](#project-trust), `sessionShareDestination` to
  [Sync, relay and origin rows](#sync-relay-and-origin-rows), the current contents of the debug
  and crash logs to [Other stores](#other-stores), `durationMs` and `argumentsError` to
  [Format versions](#format-versions), and the experimental server's removal to
  [Experimental durable sessions](#experimental-durable-sessions).

## Collection

### Per-user storage

One layout on every OS. The agent directory is `$PI_CODING_AGENT_DIR` (with `~` expanded) or
`os.homedir()/.pi/agent`
([`packages/coding-agent/src/config.ts:566-572`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/config.ts#L566-L572);
tilde expansion
[`utils/paths.ts:88-93`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/utils/paths.ts#L88-L93)).
No XDG, no `AppData`, no `Library`: Windows is `C:\Users\<u>\.pi\agent`. The `.pi` name and the
env-var prefix come from `piConfig` in the package's `package.json`
([`config.ts:538-547`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/config.ts#L538-L547),
[`package.json:6-8`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/package.json#L6-L8)),
so a rebranded build (the source comment's example is `TAU_CODING_AGENT_DIR`) uses another directory
and is not covered by the catalog lines.

Under `~/.pi/agent`:

- `sessions/--<cwd>--/<ISO timestamp>_<session id>.jsonl`: one JSONL transcript per session
  ([`config.ts:610-612`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/config.ts#L610-L612);
  [`core/session-manager.ts:589-594`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/session-manager.ts#L589-L594),
  [`1079-1080`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/session-manager.ts#L1079-L1080)).
  The directory name is the cwd with a leading `/` or `\` dropped and every `/`, `\`, `:` replaced
  by `-`: `/home/alice/proj` → `--home-alice-proj--`, `C:\Users\alice\proj` →
  `--C--Users-alice-proj--`. The file name is the ISO time with `:` and `.` replaced by `-`. A file
  is only created once a user or assistant message exists
  ([`1161-1170`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/session-manager.ts#L1161-L1170)).
  The directory can be moved with `--session-dir`, `$PI_CODING_AGENT_SESSION_DIR` or the
  `sessionDir` setting, in that order
  ([`main.ts:688-692`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/main.ts#L688-L692));
  a custom directory is flat, with no per-cwd subdirectory
  ([`session-manager.ts:1755-1757`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/session-manager.ts#L1755-L1757)).
  Sessions written by v0.30.0 sat directly in `~/.pi/agent/*.jsonl` until a migration moved them
  ([`migrations.ts:76-84`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/migrations.ts#L76-L84)).
- `settings.json` (global settings; `packages`, `extensions`, `skills`, `sessionDir`, `httpProxy`,
  default provider and model)
  ([`config.ts:590-592`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/config.ts#L590-L592);
  [`core/settings-manager.ts:133-138`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/settings-manager.ts#L133-L138),
  [`159-180`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/settings-manager.ts#L159-L180)),
  `keybindings.json`
  ([`core/keybindings.ts:379-380`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/keybindings.ts#L379-L380)).
  Since [`5c1736f`](https://github.com/earendil-works/pi/commit/5c1736f82d201d09664642ab0d88147da94b450e) `settings.json` also holds `defaultProjectTrust` ([Project trust](#project-trust))
  and `sessionShareDestination`, where `/share` uploads
  ([Sync, relay and origin rows](#sync-relay-and-origin-rows)).
- `auth.json`, `models.json`, `models-store.json` (dynamically refreshed provider model catalogs)
  ([`config.ts:580-587`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/config.ts#L580-L587);
  [`core/models-store.ts:52`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/models-store.ts#L52)).
- `trust.json`: project trust decisions keyed by absolute path ([Project trust](#project-trust)).
- `mcp.json` (MCP servers), `mcp-auth.json` (MCP OAuth state), `mcp.log`
  ([`extensions/mcp/config.ts:146-148`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/extensions/mcp/config.ts#L146-L148);
  [`extensions/mcp/oauth.ts:137-145`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/extensions/mcp/oauth.ts#L137-L145);
  [`extensions/mcp/index.ts:351`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/extensions/mcp/index.ts#L351)).
  Not present at v0.83.0.
- `crashes.json`, the recent crashes (not present at v0.83.0), and `pi-debug.log`, written by
  `/debug` (present at v0.83.0 as well)
  ([Debug log and crash log](#debug-log-and-crash-log)).
- User resources: `extensions/`, `skills/`, `prompts/` (formerly `commands/`), `themes/`,
  `AGENTS.md`/`CLAUDE.md` global context, `SYSTEM.md`, `APPEND_SYSTEM.md`
  ([`core/resource-loader.ts:978-982`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/resource-loader.ts#L978-L982),
  [`1207`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/resource-loader.ts#L1207),
  [`1221`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/resource-loader.ts#L1221),
  [`242-246`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/resource-loader.ts#L242-L246);
  [`migrations.ts:137-143`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/migrations.ts#L137-L143)).
  Skills are also read from `~/.agents/skills`
  ([`core/trust-manager.ts:187-188`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/trust-manager.ts#L187-L188)),
  already in the catalog as `shared|.agents`.
- Installed packages (`pi install`): `npm/` (an npm root with `package.json` and
  `node_modules/<name>`) and `git/<host>/<path>` checkouts
  ([`core/package-manager.ts:2134`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/package-manager.ts#L2134),
  [`2154-2174`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/package-manager.ts#L2154-L2174));
  temporary checkouts in `tmp/extensions/`
  ([`231-236`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/package-manager.ts#L231-L236)).
- `bin/` (downloaded `fd` and `rg`), legacy `tools/`
  ([`config.ts:595-602`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/config.ts#L595-L602);
  [`migrations.ts:176-185`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/migrations.ts#L176-L185)).

#### Scratch directories

None known: pi creates none under the home; it runs in the user's cwd. At
[`5c1736f`](https://github.com/earendil-works/pi/commit/5c1736f82d201d09664642ab0d88147da94b450e)
its output files go to the system temporary directory through one helper, now mode 0600
([`utils/output-files.ts:14-34`](https://github.com/earendil-works/pi/blob/5c1736f82d201d09664642ab0d88147da94b450e/packages/coding-agent/src/utils/output-files.ts#L14-L34)),
outside every catalog line.

Truncated bash output is spilled to `<tmpdir>/pi-bash-<16 hex>.log`
([`core/bash-executor.ts:68-69`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/bash-executor.ts#L68-L69)),
MCP and codemode output to `pi-mcp-*` and `pi-codemode-*.txt`
([`extensions/mcp/tools.ts:72`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/extensions/mcp/tools.ts#L72);
[`extensions/codemode/execute.ts:263`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/extensions/codemode/execute.ts#L263));
the session records the path in `fullOutputPath`.

#### Experimental stores

Experimental (`PI_EXPERIMENTAL=1`,
[`core/experimental.ts:1-3`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/experimental.ts#L1-L3);
[`experimental/commands.ts:86`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/experimental/commands.ts#L86)),
not at v0.83.0: `experimental/sessions/<id>/{meta.json,session.sqlite}`
([`experimental/server.ts:69-71`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/experimental/server.ts#L69-L71);
[`experimental/session-catalog.ts:5-16`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/experimental/session-catalog.ts#L5-L16)),
`experimental/vacation-sessions/<sha256(cwd)[:24]>/<ms>-<uuid>/session.sqlite`
([`experimental/vacation/sessions.ts:18-55`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/experimental/vacation/sessions.ts#L18-L55)),
and a server directory `$PI_SERVER_DIR` or `~/.pi/server` holding `default-server-id`, lock
directories and Unix sockets
([`experimental/server.ts:51-56`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/experimental/server.ts#L51-L56),
[`85-91`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/experimental/server.ts#L85-L91)).

At [`5c1736f`](https://github.com/earendil-works/pi/commit/5c1736f82d201d09664642ab0d88147da94b450e) these stores are no longer written
([Experimental durable sessions](#experimental-durable-sessions)); they remain only where an
older build ran.

#### Rewrites on open

pi rewrites its own evidence. Opening an older-format session rewrites the whole file in the current
format
([`session-manager.ts:1088-1094`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/session-manager.ts#L1088-L1094)),
and loading a file whose last line has no newline appends one
([`668`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/session-manager.ts#L668)).
Collect before anyone resumes a session on the host. What the session picker's delete removes, and
what survives it, is in [What survives a delete](#what-survives-a-delete).

### Credentials

- `~/.pi/agent/auth.json`, created mode 0600 in a 0700 directory: one object per provider,
  `{type:"api_key", key, env}` or `{type:"oauth", refresh, access, expires, ...}`
  ([`core/auth-storage.ts:24-25`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/auth-storage.ts#L24-L25),
  [`52-60`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/auth-storage.ts#L52-L60);
  [`packages/ai/src/auth/types.ts:17-37`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/ai/src/auth/types.ts#L17-L37)).
  No keychain. The code read writes no backup or second copy of the file; the catalog's
  `.pi/agent/auth.json.*` secret glob guards only against an editor's copy.
- `~/.pi/agent/mcp-auth.json`: MCP client registrations, tokens and pending PKCE verifiers
  ([`extensions/mcp/oauth.ts:137-145`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/extensions/mcp/oauth.ts#L137-L145)).
- `~/.pi/agent/models.json`: custom providers may carry `apiKey` and `headers`
  ([`core/model-config.ts:242-251`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/model-config.ts#L242-L251));
  a value is a literal, an env reference, or `!command` whose stdout is used
  ([`core/resolve-config-value.ts:140-145`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/resolve-config-value.ts#L140-L145)).
  Flag it.
- `~/.pi/agent/mcp.json` and project `.pi/mcp.json`: `headers` such as `Authorization: Bearer ...`
  ([`extensions/mcp/config.ts:14-21`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/extensions/mcp/config.ts#L14-L21)).
  Flag it.
- Legacy: `oauth.json` and `settings.json` `apiKeys` were folded into `auth.json`; the old file is
  renamed `oauth.json.migrated`, not deleted
  ([`migrations.ts:20-60`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/migrations.ts#L20-L60)).
- pi also reads, but does not write, `~/.config/gcloud/application_default_credentials.json`
  ([`packages/ai/src/env-api-keys.ts:60-67`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/ai/src/env-api-keys.ts#L60-L67))
  and the Hugging Face token
  ([`extensions/llama/huggingface.ts:51-55`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/extensions/llama/huggingface.ts#L51-L55));
  they belong to those tools.

### Exclusions

`bin/` holds third-party binaries. `npm/node_modules`, the `git/` checkouts and `tmp/` are installed
extension packages; the package sources stay recorded in `settings.json` `packages` and
`npm/package.json`
([`package-manager.ts:2068`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/package-manager.ts#L2068)),
so exclude only their dependency trees and `.git`. Project `.pi/npm` and `.pi/git` are the same at
project scope
([`2130-2132`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/package-manager.ts#L2130-L2132),
[`2169-2172`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/package-manager.ts#L2169-L2172)).
Sockets in `~/.pi/server` cannot be copied. Sessions themselves stay: images are inline base64 and
can be large, but they are the transcript.

### Project-local files

`<cwd>/.pi/` holds `settings.json`
([`settings-manager.ts:301`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/settings-manager.ts#L301)),
`mcp.json`, `extensions/`, `skills/`, `prompts/`, `themes/`, `SYSTEM.md`, `APPEND_SYSTEM.md`,
`npm/`, `git/`
([`trust-manager.ts:30-39`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/trust-manager.ts#L30-L39);
[`resource-loader.ts:984-988`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/resource-loader.ts#L984-L988)).
Project extensions run with the user's rights once the folder is trusted, so they are evidence.
little-coder adds `.pi/approved-plan.md`
([Project-local files in `little-coder.md`](little-coder.md#project-local-files)).

Context files read from the cwd up: `AGENTS.override.md`, `AGENTS.md`, `AGENTS.MD`, `CLAUDE.md`,
`CLAUDE.MD`
([`resource-loader.ts:183-185`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/resource-loader.ts#L183-L185)),
and `.agents/skills` up to the git root
([`package-manager.ts:467-480`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/package-manager.ts#L467-L480)).

Exports land in the cwd by default: `/export` HTML as `pi-session-<session file basename>.html`
([`core/export-html/index.ts:274-277`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/export-html/index.ts#L274-L277))
and JSONL as `session-<ISO time>.jsonl`
([`core/session-export.ts:37-43`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/session-export.ts#L37-L43)).
No per-project database.

### Where the project path is recorded

- Session header (line 1 of each file), key `cwd`
  ([`session-manager.ts:43-50`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/session-manager.ts#L43-L50));
  the directory name is a lossy copy.
- `trust.json`: every key is an absolute project path ([Project trust](#project-trust)).
- `crashes.json`: `cwd` and `sessionFile` per record
  ([Debug log and crash log](#debug-log-and-crash-log)).
- `experimental/sessions/*/meta.json`: `cwd`
  ([`session-catalog.ts:6-13`](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/experimental/session-catalog.ts#L6-L13)).

## Parsing

### Transcript stores

- `.pi/agent/sessions/--<encoded cwd>--/<ISO time, : and . as ->_<session id>.jsonl`, e.g.
  `2026-10-01T09-00-00-000Z_0199a1b2-....jsonl`, named as in [Per-user storage](#per-user-storage).
  One file per session; forks and branches get new files
  ([session-manager.ts:1077-1080](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/session-manager.ts#L1077-L1080)).
- With `--session-dir`, `PI_CODING_AGENT_SESSION_DIR` or the `sessionDir`
  setting the files sit flat in that directory, anywhere on disk
  ([Per-user storage](#per-user-storage)).
- Legacy: `.pi/agent/*.jsonl` (v0.30.0 bug, moved on next start).
- Exports in project directories: `session-<time>.jsonl` (same format,
  one branch) and `pi-session-*.html` ([Project-local files](#project-local-files)).
- Experimental durable sessions are SQLite, [Other stores](#other-stores).

#### What survives a delete

The session picker's delete runs the `trash` command and falls back to unlinking the session file
([session-selector.ts:650-680](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/modes/interactive/components/session-selector.ts#L650-L680);
unchanged at [`5c1736f`](https://github.com/earendil-works/pi/commit/5c1736f82d201d09664642ab0d88147da94b450e),
[session-selector.ts:650-680](https://github.com/earendil-works/pi/blob/5c1736f82d201d09664642ab0d88147da94b450e/packages/coding-agent/src/modes/interactive/components/session-selector.ts#L650-L680)),
so the file is gone or sits in the user's trash, with any `pi.bug-report` entry inside it; no index
or log repeats that entry, and pi offers no way to delete the remote copy that `/share` or a bug
report uploaded. pi keeps no per-prompt history file (none in pi at [`5c1736f`](https://github.com/earendil-works/pi/commit/5c1736f82d201d09664642ab0d88147da94b450e): a grep of
`packages/coding-agent/src` for `history.jsonl` and `promptHistory` finds none).

What remains is `pi-debug.log`, whose last `/debug` snapshot can hold one session's messages, and
`crashes.json`, with each recent crash's `sessionFile` and `cwd`
([Debug log and crash log](#debug-log-and-crash-log)); both survive the delete.
Whether the delete touches the exports a user wrote into project
directories (`session-<time>.jsonl`, `pi-session-*.html`) is not
stated. pi writes no deletion marker. The analyzer reads the session
files, `trust.json`, `pi-debug.log` as a secondary source, whose messages it keeps when no session
file holds them, and `crashes.json`, whose rows name a deleted session's file and directory (see
[Secondary sources](../analyzer/docs/output.md#secondary-sources)).

### Record schema

Every line is one JSON object. Malformed lines are skipped by pi
([session-manager.ts:616-624](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/session-manager.ts#L616-L624)).

#### Header

Line 1: `type:"session"`, `version` (3; absent in v1), `id` (session id), `timestamp` (ISO 8601 UTC
with ms and `Z`, from `toISOString`), `cwd`, optional `parentSession` (absolute path of the session
file this one was forked or branched from)
([43-50](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/session-manager.ts#L43-L50),
[1061-1069](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/session-manager.ts#L1061-L1069)).
The session id is a UUIDv7
([264-266](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/session-manager.ts#L264-L266))
unless given with `--session-id`
([main.ts:448](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/main.ts#L448),
[session-manager.ts:268-274](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/session-manager.ts#L268-L274)).
There is no producer name or pi version in the header.

#### Entries

`type`, `id` (8 hex characters,
[276-284](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/session-manager.ts#L276-L284)),
`parentId` (null at a root), `timestamp` (ISO, written at append time)
([57-62](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/session-manager.ts#L57-L62),
[1204-1213](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/session-manager.ts#L1204-L1213)).
Types
([64-195](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/session-manager.ts#L64-L195)):

| `type` | Fields |
| --- | --- |
| `message` | `message`: one of the roles below |
| `thinking_level_change` | `thinkingLevel` |
| `model_change` | `provider`, `modelId` |
| `usage` | `kind` (e.g. `cache_warm`), `provider`, `model`, `usage`, `note`; HEAD only |
| `compaction` | `summary`, `firstKeptEntryId`, `tokensBefore`, `details`, `usage`, `fromHook`, `systemMessage` (HEAD only) |
| `branch_summary` | `fromId`, `summary`, `details`, `usage`, `fromHook` |
| `custom` | `customType`, `data`: extension state, not in model context |
| `custom_message` | `customType`, `content` (string or text/image parts), `display`, `details`: extension-injected context |
| `context_edit` | `targetId`, `replacement` (`{content}` or null = omitted from context); HEAD only |
| `label` | `targetId`, `label` (undefined clears) |
| `session_info` | `name` (user-set session title) |

#### Bug-report entry

`/bug` (HEAD only; absent at v0.83.0) appends a `custom` entry with `customType:"pi.bug-report"` to
the session file once a report is uploaded or exported
([modes/interactive/bug-report.ts:213-227](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/modes/interactive/bug-report.ts#L213-L227),
[core/bug-report.ts:17](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/bug-report.ts#L17)).
Its `data` is `id` (the report id, a UUIDv7 made locally,
[core/bug-report.ts:159-160](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/bug-report.ts#L159-L160)),
`createdAt` (ISO), `hint` (the user's description or null), `sessionIncluded` and `summaryIncluded`
(booleans), `delivery` (`upload` or `zip`) and, for `zip`, `path`, the archive
`pi-bug-report-<id>.zip` in the working directory
([core/bug-report.ts:235-243](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/bug-report.ts#L235-L243),
[278-280](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/bug-report.ts#L278-L280)).
The entry's own `timestamp` is the append time
([session-manager.ts:1290-1301](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/session-manager.ts#L1290-L1301)).
A session never reported has no such entry.

#### Message roles

`message.message.role`, every one with `timestamp`
in Unix milliseconds:

- `user`: `content` string or `[{type:"text",text}|{type:"image",data,mimeType}]`
  ([types.ts:542-546](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/ai/src/types.ts#L542-L546),
  [397-417](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/ai/src/types.ts#L397-L417)).
  Images are inline base64.
- `assistant`: `content[]` of `text` (`text`, `textSignature`), `thinking` (`thinking`,
  `thinkingSignature`, `redacted`), `toolCall` (`id`, `name`, `arguments` object,
  `thoughtSignature`, `namespace`); `api`, `provider`, `model`, `responseModel`, `responseId`,
  `thinkingLevel`, `diagnostics`, `usage` (`input`, `output`, `cacheRead`, `cacheWrite`,
  `totalTokens`, `cost{...}`), `stopReason` (`pending|stop|length|toolUse|error|aborted|deferred`),
  `errorMessage`
  ([types.ts:403-452](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/ai/src/types.ts#L403-L452),
  [548-572](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/ai/src/types.ts#L548-L572)).
- `toolResult`: `toolCallId`, `toolName`, `content[]` (text/image), `details` (tool-specific, e.g.
  edit `diff`/`patch`, bash `fullOutputPath`), `usage`, `nestedCalls` (calls a codemode script
  made), `isError`
  ([596-610](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/ai/src/types.ts#L596-L610);
  [tools/edit.ts:70-77](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/tools/edit.ts#L70-L77),
  [tools/bash.ts:66-69](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/tools/bash.ts#L66-L69)).
  An extension that blocks a call makes the agent loop record an error `toolResult` whose text is
  the extension's reason
  ([packages/agent/src/agent-loop.ts:745-753](https://github.com/earendil-works/pi/blob/5c1736f82d201d09664642ab0d88147da94b450e/packages/agent/src/agent-loop.ts#L745-L753),
  [912-917](https://github.com/earendil-works/pi/blob/5c1736f82d201d09664642ab0d88147da94b450e/packages/agent/src/agent-loop.ts#L912-L917));
  little-coder's [permission gate](little-coder.md#permission-gate) is such an extension.
- `system`: `content` (the system prompt on the first one, added instructions later), `sections`,
  `toolsAdded`, `toolsRemoved`
  ([types.ts:524-540](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/ai/src/types.ts#L524-L540)).
  Persisted since the system-message change
  ([agent-session.ts:1126-1133](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/agent-session.ts#L1126-L1133));
  v0.83.0 persisted only user, assistant and toolResult
  ([0.83.0 agent-session.ts:635-641](https://github.com/earendil-works/pi/blob/845d6ff1f6643aba440341cce877ce1c43ebbc39/packages/coding-agent/src/core/agent-session.ts#L635-L641)).
- `bashExecution`: a user `!command`: `command`, `output`, `exitCode`, `cancelled`, `truncated`,
  `fullOutputPath`, `excludeFromContext`
  ([messages.ts:29-40](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/messages.ts#L29-L40);
  [agent-session.ts:3848](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/agent-session.ts#L3848)).
- `custom` (v2 `hookMessage` renamed), `branchSummary`, `compactionSummary`
  ([messages.ts:46-67](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/messages.ts#L46-L67)).

#### Tool arguments

Built-in tool arguments: `bash` `command`, `timeout`
([tools/bash.ts:40-43](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/tools/bash.ts#L40-L43));
`read` `path`, `offset`, `limit`
([read.ts:14-18](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/tools/read.ts#L14-L18));
`write` `path`, `content`
([write.ts:11-14](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/tools/write.ts#L11-L14));
`edit` `path`, `edits[{oldText,newText}]`
([edit.ts:21-37](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/tools/edit.ts#L21-L37));
`grep` `pattern`, `path`, `glob`
([grep.ts:21-24](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/tools/grep.ts#L21-L24));
`find` `pattern`, `path`
([find.ts:26-30](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/tools/find.ts#L26-L30));
`ls` `path`
([ls.ts:11-12](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/tools/ls.ts#L11-L12));
`codemode` `code`
([extensions/codemode/tool.ts:87-90](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/extensions/codemode/tool.ts#L87-L90)).

### Joins and timestamps

- Session: header `id`, also the file-name suffix. Project: header `cwd`.
- Tool use to result: `toolCall.id` = `toolResult.toolCallId`.
- Tree: `parentId` → `id`; the file holds every branch; the active
  path is the walk up from the last entry. `label` and `context_edit`
  point at `targetId`; `compaction.firstKeptEntryId` marks the cut.
- Fork/branch: child header `parentSession` = parent file path. `forkFrom` copies every parent entry
  verbatim, ids and timestamps included
  ([session-manager.ts:1844-1863](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/session-manager.ts#L1844-L1863));
  branching copies the path to the chosen entry
  ([1632-1681](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/session-manager.ts#L1632-L1681)).
  The same entry therefore appears in several files.
- little-coder checkpoints: a directory named after the session file
  ([Checkpoints in
  `little-coder.md`](little-coder.md#checkpoints-little-codercheckpointssession-file-name)).
- Sub-agents: pi has none built in; little-coder's leave no session file
  ([Transcript stores in `little-coder.md`](little-coder.md#transcript-stores)).

### Other stores

The main format has no SQLite or binary store.

#### Experimental durable sessions

The experimental server and `vacation` mode (`PI_EXPERIMENTAL=1`, HEAD only) keep `session.sqlite`
in WAL mode
([packages/durable/src/storage/sqlite/node.ts:181-193](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/durable/src/storage/sqlite/node.ts#L181-L193))
with tables `conversations`, `entries(conversation_id, head, commit_seq, record JSON)`, `tasks`,
`submissions`, `documents`, `document_revisions`
([migrations.ts:10-82](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/durable/src/storage/sqlite/migrations.ts#L10-L82)).
Not parsed in this plan; open through `sqlite_util.py` with sidecars when it is.

At [`5c1736f`](https://github.com/earendil-works/pi/commit/5c1736f82d201d09664642ab0d88147da94b450e)
the CLI no longer has the experimental server: `packages/server`, `packages/client` and
`packages/coding-agent/src/experimental/server.ts` are gone, leaving only
`src/experimental/source-resolver.ts` in the CLI, `coding-agent` no longer depends on
`pi-durable` (grepped its `package.json` and `src` for `durable`, and the tree for `session-relays`
and `radius-relay`), and `PI_EXPERIMENTAL`
([core/experimental.ts:1-3](https://github.com/earendil-works/pi/blob/5c1736f82d201d09664642ab0d88147da94b450e/packages/coding-agent/src/core/experimental.ts#L1-L3))
is read only by the footer and the startup screen
([footer.ts:213](https://github.com/earendil-works/pi/blob/5c1736f82d201d09664642ab0d88147da94b450e/packages/coding-agent/src/modes/interactive/components/footer.ts#L213),
[cli/startup-ui.ts:141](https://github.com/earendil-works/pi/blob/5c1736f82d201d09664642ab0d88147da94b450e/packages/coding-agent/src/cli/startup-ui.ts#L141)).
`experimental/sessions/`, `experimental/vacation-sessions/` and `~/.pi/server` exist only where a
build from before the removal ran.

#### Debug log and crash log

`pi-debug.log`, written by `/debug`
([config.ts:615-617](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/config.ts#L615-L617);
at [`5c1736f`](https://github.com/earendil-works/pi/commit/5c1736f82d201d09664642ab0d88147da94b450e)
[config.ts:653-656](https://github.com/earendil-works/pi/blob/5c1736f82d201d09664642ab0d88147da94b450e/packages/coding-agent/src/config.ts#L653-L656)),
is rewritten by each `/debug`: a first line `Debug output at <ISO time>`, the terminal size, the
rendered screen lines, then `=== Agent messages (JSONL) ===` and the open session's messages, one
JSON object per line, with no session id
([modes/interactive/interactive-mode.ts:6959-6982](https://github.com/earendil-works/pi/blob/5c1736f82d201d09664642ab0d88147da94b450e/packages/coding-agent/src/modes/interactive/interactive-mode.ts#L6959-L6982)),
so it can hold a deleted session's turns. v0.83.0 writes the same file the same way
([config.ts:563-565](https://github.com/earendil-works/pi/blob/845d6ff1f6643aba440341cce877ce1c43ebbc39/packages/coding-agent/src/config.ts#L563-L565),
[interactive-mode.ts:5886-5910](https://github.com/earendil-works/pi/blob/845d6ff1f6643aba440341cce877ce1c43ebbc39/packages/coding-agent/src/modes/interactive/interactive-mode.ts#L5886-L5910)).
Each line is the inner object of a session `message` entry ([Message roles](#message-roles)), with
its own `timestamp` in Unix milliseconds and no entry `id` or `timestamp`. An assistant message's
`timestamp` is set when its stream starts
([packages/ai/src/api/anthropic-messages.ts:585-600](https://github.com/earendil-works/pi/blob/5c1736f82d201d09664642ab0d88147da94b450e/packages/ai/src/api/anthropic-messages.ts#L585-L600)),
while the session file appends the entry, with the append time, at `message_end`
([core/agent-session.ts:1148-1166](https://github.com/earendil-works/pi/blob/5c1736f82d201d09664642ab0d88147da94b450e/packages/coding-agent/src/core/agent-session.ts#L1148-L1166)),
so the two times of one message differ by the length of the reply.

`crashes.json` is a JSON array of the last five crashes (a crash older than seven days is not
announced), each with its own `timestamp`, `message`, `stack`, `sessionFile` and `cwd`
([core/crash-log.ts:6-23](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/crash-log.ts#L6-L23);
at [`5c1736f`](https://github.com/earendil-works/pi/commit/5c1736f82d201d09664642ab0d88147da94b450e)
[core/crash-log.ts:7-19](https://github.com/earendil-works/pi/blob/5c1736f82d201d09664642ab0d88147da94b450e/packages/coding-agent/src/core/crash-log.ts#L7-L19)),
where a record also has `version`, `kind` (`uncaught_exception` or `fatal_error`) and `notified`,
and `stack` and `sessionFile` may be null
([crash-log.ts:124-144](https://github.com/earendil-works/pi/blob/5c1736f82d201d09664642ab0d88147da94b450e/packages/coding-agent/src/core/crash-log.ts#L124-L144)).

#### Project trust

`~/.pi/agent/trust.json` is an object whose keys are absolute project paths and whose values are
`true` or `false`; a `null` decision deletes the key
([trust-manager.ts:28](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/trust-manager.ts#L28),
[214](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/trust-manager.ts#L214),
[232-241](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/trust-manager.ts#L232-L241));
v0.83.0, which little-coder pins, writes the same file
([trust-manager.ts:212](https://github.com/earendil-works/pi/blob/845d6ff1f6643aba440341cce877ce1c43ebbc39/packages/coding-agent/src/core/trust-manager.ts#L212)).
Since [`5c1736f`](https://github.com/earendil-works/pi/commit/5c1736f82d201d09664642ab0d88147da94b450e) the global setting `defaultProjectTrust` in `settings.json` (`ask`, the default,
`always` or `never`; global only) decides an untrusted project's fate when `trust.json` has no
entry
([settings-schema.ts:266-271](https://github.com/earendil-works/pi/blob/5c1736f82d201d09664642ab0d88147da94b450e/packages/coding-agent/src/core/settings-schema.ts#L266-L271),
[settings-defaults.ts:30](https://github.com/earendil-works/pi/blob/5c1736f82d201d09664642ab0d88147da94b450e/packages/coding-agent/src/core/settings-defaults.ts#L30),
[project-trust.ts:78](https://github.com/earendil-works/pi/blob/5c1736f82d201d09664642ab0d88147da94b450e/packages/coding-agent/src/core/project-trust.ts#L78)).
pi has no per-call tool approval of its own: the only `--approve` is the per-run project trust
override
([cli/args.ts:241-244](https://github.com/earendil-works/pi/blob/5c1736f82d201d09664642ab0d88147da94b450e/packages/coding-agent/src/cli/args.ts#L241-L244),
[341-342](https://github.com/earendil-works/pi/blob/5c1736f82d201d09664642ab0d88147da94b450e/packages/coding-agent/src/cli/args.ts#L341-L342)),
which writes nothing.

### Format versions

Header `version`: 1 (no field, no ids), 2 (`id`/`parentId` added), 3 (`hookMessage` role renamed
`custom`)
([session-manager.ts:41](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/session-manager.ts#L41),
[286-345](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/session-manager.ts#L286-L345)).
pi rewrites old files in place on open, so a collected v1/v2 file means nobody resumed it. Within v3
fields were added without a bump: `usage` and `context_edit` entries, `system` role,
`compaction.systemMessage`, `stopReason:"deferred"` (absent at v0.83.0,
[0.83.0 types.ts:391](https://github.com/earendil-works/pi/blob/845d6ff1f6643aba440341cce877ce1c43ebbc39/packages/ai/src/types.ts#L391)).
A parser must ignore unknown types and fields.

At [`5c1736f`](https://github.com/earendil-works/pi/commit/5c1736f82d201d09664642ab0d88147da94b450e) (coding-agent 1.3.0) the session writer (`core/session-manager.ts`),
`core/trust-manager.ts`, `core/bug-report.ts`, `core/crash-log.ts` and the session picker are
byte for byte the same as at [`2003871`](https://github.com/earendil-works/pi/commit/200387122ca450d6387f033949423114a270b96c), so the record shapes the parser reads are unchanged and no
model field changed; assistant messages gained `durationMs` and tool calls `argumentsError`
([packages/ai/src/types.ts:448](https://github.com/earendil-works/pi/blob/5c1736f82d201d09664642ab0d88147da94b450e/packages/ai/src/types.ts#L448),
[600](https://github.com/earendil-works/pi/blob/5c1736f82d201d09664642ab0d88147da94b450e/packages/ai/src/types.ts#L600)),
again without a version bump. The `meta.json` reader serves only installs from before the
experimental server's removal ([Experimental durable sessions](#experimental-durable-sessions)).

### Secrets

Tool arguments and outputs are stored verbatim (`cat .env`, `write`
contents, bash output). `system` messages carry the full system prompt,
including AGENTS.md and skill text. `thinkingSignature`,
`thoughtSignature`, `textSignature` are opaque provider blobs (redacted
thinking is encrypted there). Credentials are in `auth.json` and
`mcp-auth.json`, not in sessions.

### Parser plan

The parser reads `.pi/agent/sessions/*/*.jsonl` and `.pi/agent/*.jsonl`, takes line 1 as the
header and skips files whose first record is not `type:"session"`. Files in a custom session
directory elsewhere on disk are not read; they would be detected by that header shape.

| Column | Source |
| --- | --- |
| timestamp_utc | entry `timestamp`; fall back to `message.timestamp` (ms) |
| session_id | header `id` |
| project_path | header `cwd` |
| git_branch | empty (not recorded) |
| turn_type | `user`; `assistant` text parts; `toolCall` → tool_use; `toolResult` → tool_result; `thinking` → thinking; `bashExecution` → tool_use + tool_result; `system` role, `custom` role, `custom_message`, `model_change`, `thinking_level_change`, `compaction`, `branch_summary`, `context_edit`, `session_info` → system; skip `custom` entries, `label`, `usage` |
| model | assistant `responseModel`, else `model`; `model_change.modelId` |
| tool_name | `toolCall.name`, `toolResult.toolName`; `bash` for `bashExecution` |
| tool_use_id | `toolCall.id` / `toolResult.toolCallId`; entry `id` for `bashExecution` |
| text | text parts joined; tool_use: `command`, `path`, `pattern` or `code`, else JSON; tool_result: text parts; compaction/branch: `summary` |

#### Permission rows

No entry type or message role in [Record schema](#record-schema) records a permission decision. The
standing state is `~/.pi/agent/trust.json` ([Project trust](#project-trust)). One `system` row per
key, `permission setting: <path>=<value> (from .pi/agent/trust.json, file mtime)`, with
`project_path` the key, `session_id` empty, `source_line` 1 and `timestamp_utc` the file's
modification time. A file that is not a JSON object is one `system` row
`parser: .pi/agent/trust.json is not a JSON object`.

The other standing setting, `defaultProjectTrust` in `~/.pi/agent/settings.json`
([Project trust](#project-trust)), is one `system` row `permission setting:
defaultProjectTrust=<value> (from .pi/agent/settings.json, file mtime)` at the file's modification
time, with an empty `session_id`; no row when the key is absent, and no other key of the file is
read. pi records no per-call decision; an extension's block is an error `toolResult` with the
extension's reason as its text ([Message roles](#message-roles)). little-coder's permission gate is
the one such extension whose texts are known
([Permission gate in `little-coder.md`](little-coder.md#permission-gate)): an error `toolResult`
whose text is `edit cancelled by user` or `command cancelled by user` or starts with
`shell whitelist:` is followed by `permission decision: denied (<the text>)`, with the call's
`tool_name` and `tool_use_id` and the result's time. Both rows are source-derived; not observed on
any real install.

#### Model rows

`system` row `model: <id> provider=<provider>` at the
first entry of a session that names a model, and `model: <old> -> <new>
provider=<provider>` on each later entry whose model differs, both with the
`model` column set to the new id:

- `model_change`: `modelId` and `provider`.
- assistant message: `responseModel`, else `model`, and `provider`; when `responseModel` is set and
  differs from `model`, the row ends ` requested=<model>`. `responseModel` is the "concrete model
  reported by the provider when different from the requested `model`"
  ([types.ts:553-554](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/ai/src/types.ts#L553-L554)).

The row precedes the entry's own rows and carries its timestamp. A
`model_change` entry is the user's own switch, so one that names the
model already in force still yields the plain `model: <id>
provider=<provider>` row; an assistant message that repeats the current
model yields none.

#### Past-use rows and secondary sources

Both files of [Debug log and crash log](#debug-log-and-crash-log) are read on every run;
source-derived; not observed on any real install.

- `pi-debug.log`, secondary: one `system` row `debug snapshot: /debug wrote <N> message(s) of the
  open session, which the log does not name (from .pi/agent/pi-debug.log, Debug output at line)`
  at the time on its first line, then the lines after the marker through the session reader, each
  at the message's own `timestamp`, with an empty `session_id` and `project_path`. A row the
  session file also holds is dropped; the window is one hour, because an assistant message's own
  time is its stream start and the entry's is its end. The screen lines before the marker are not
  read.
- `crashes.json`, primary, since it repeats no session event: one `system` row per record,
  `crash: <kind> <message> version=<version> sessionFile=<sessionFile> (from
  .pi/agent/crashes.json, record timestamp)` followed by the stack on the next lines, at the
  record's `timestamp`, with `project_path` the `cwd` and `session_id` the id in the
  `sessionFile` name; `-` for an absent value.

pi keeps no prompt history file ([What survives a delete](#what-survives-a-delete)).

When a file's `parentSession` parent is also in the collection, drop
child entries whose `id` and `timestamp` match a parent entry, so a fork
does not double the timeline.

#### Sync, relay and origin rows

pi keeps no session in a vendor account, but two commands send a
session out, and only one leaves a mark.

- `/share` uploads the current branch on request
  ([slash-commands.ts:27](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/slash-commands.ts#L27),
  [interactive-mode.ts:3190-3194](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/modes/interactive/interactive-mode.ts#L3190-L3194)).
  With Radius credentials (Radius is Earendil Works' hosted model gateway for pi, signed in with
  `/login radius`,
  [docs/providers.md:104-112](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/docs/providers.md#L104-L112))
  it posts the branch as JSONL, plus a trailing `pi.share` entry with the system prompt and tool
  schemas, to `https://radius.pi.dev/v1/artifacts` with `visibility=organization`
  ([session-share.ts:24-47](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/modes/interactive/session-share.ts#L24-L47),
  [98-131](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/modes/interactive/session-share.ts#L98-L131),
  [radius-config.ts:4](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/ai/src/providers/radius-config.ts#L4));
  without them it renders HTML and runs `gh gist create --public=false`, a secret gist in the user's
  GitHub account viewed at `https://pi.dev/session/#<gist id>`
  ([session-share.ts:71-88](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/modes/interactive/session-share.ts#L71-L88),
  [159-203](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/modes/interactive/session-share.ts#L159-L203),
  [config.ts:553-559](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/config.ts#L553-L559)).
  Both write only to a temporary directory that is removed afterwards, and the share URL is only
  shown on screen
  ([session-share.ts:58-60](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/modes/interactive/session-share.ts#L58-L60),
  [89-95](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/modes/interactive/session-share.ts#L89-L95),
  [146](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/modes/interactive/session-share.ts#L146));
  the `pi.share` entry goes into the uploaded copy, never the session file
  ([session-export.ts:9-29](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/session-export.ts#L9-L29)).
  v0.83.0, which little-coder pins, has the gist path only, the same way
  ([0.83.0 interactive-mode.ts:5523-5608](https://github.com/earendil-works/pi/blob/845d6ff1f6643aba440341cce877ce1c43ebbc39/packages/coding-agent/src/modes/interactive/interactive-mode.ts#L5523-L5608)).
  So a shared session leaves no local marker; the gist is found from the GitHub account, the Radius
  artifact from the organization. Since [`5c1736f`](https://github.com/earendil-works/pi/commit/5c1736f82d201d09664642ab0d88147da94b450e) the setting
  `sessionShareDestination` in `settings.json` (`auto`, `gist`, `radius`) chooses where `/share`
  uploads
  ([settings-schema.ts:224-230](https://github.com/earendil-works/pi/blob/5c1736f82d201d09664642ab0d88147da94b450e/packages/coding-agent/src/core/settings-schema.ts#L224-L230),
  [session-share.ts:69-76](https://github.com/earendil-works/pi/blob/5c1736f82d201d09664642ab0d88147da94b450e/packages/coding-agent/src/modes/interactive/session-share.ts#L69-L76)),
  still with no local record of a share; it names the likely destination and could be a `system`
  row beside the trust rows, but a share leaves no mark of its own. The experimental Radius relay
  below is gone at that commit ([Experimental durable sessions](#experimental-durable-sessions)).
- `/bug` (HEAD only) asks whether to include the transcript, then uploads `report.json`,
  `diagnostics.json` and, when chosen, `session.jsonl` and a model-written `summary.md` to
  `https://radius.pi.dev/v1/bug-reports`, with no login needed, or writes them to a zip
  ([modes/interactive/bug-report.ts:113-120](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/modes/interactive/bug-report.ts#L113-L120),
  [core/bug-report.ts:252-272](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/bug-report.ts#L252-L272),
  [bug-report-upload.ts:12-37](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/bug-report-upload.ts#L12-L37)).
  Either way it appends the `pi.bug-report` entry of [Bug-report entry](#bug-report-entry) and
  empties `.pi/agent/crashes.json` when the report carried crashes
  ([modes/interactive/bug-report.ts:226](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/modes/interactive/bug-report.ts#L226),
  [crash-log.ts:21-23](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/crash-log.ts#L21-L23)).
  The parser reads it, the one `custom` entry not skipped, and for `delivery` `upload` yields one
  `system` row
  `session synced: bug report <id> uploaded to radius.pi.dev transcript=<sessionIncluded> summary=<summaryIncluded> (from session entry pi.bug-report, entry timestamp)`,
  at the entry's `timestamp`, with the session's id and `cwd`; a `zip` entry yields
  `bug report exported: <id> path=<path> (from session entry pi.bug-report, entry timestamp)`
  instead, since nothing left the host, and an absent value prints `-`. `hint` is not printed. Both
  rows are fixture only: no real install has shown a `pi.bug-report` entry. The server answers with
  its own `bug_report.id`, which pi shows and does not store
  ([modes/interactive/bug-report.ts:186-189](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/modes/interactive/bug-report.ts#L186-L189)).

The experimental server (`PI_EXPERIMENTAL=1`, HEAD only,
[experimental.ts:2](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/core/experimental.ts#L2))
also opens a live relay through Radius, `/v1/session-relays/<server id>/connect`, when Radius
credentials exist
([server.ts:609-615](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/experimental/server.ts#L609-L615),
[radius-relay.ts:619-623](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/experimental/radius-relay.ts#L619-L623),
[radius-auth.ts:31-55](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/experimental/radius-auth.ts#L31-L55));
it carries traffic to remote clients rather than storing a copy, and its only file is the server id,
`.pi/server/default-server-id`
([server.ts:54-56](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/experimental/server.ts#L54-L56),
[76-112](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/experimental/server.ts#L76-L112)),
which says the host could be reached that way, not that a session was. No telemetry leaves pi: the
telemetry package has no exporter
([telemetry/README.md:11](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/telemetry/README.md#L11)).

#### Scratch directory rows

None in the home. Spilled tool output goes to the system temporary directory and is named in a
`toolResult`'s `details.fullOutputPath` ([Scratch directories](#scratch-directories)).

#### Deletion rows

pi writes no deletion marker; the picker's delete removes the session file with its
`pi.bug-report` entry ([What survives a delete](#what-survives-a-delete)). Covered.

#### Bad-line rows

pi itself skips malformed lines ([Record schema](#record-schema)); the parser does not. Each line
that does not decode is its own `system` row at the end of the session's rows,
`parser: unparseable line (<decoder reason>): <the line in full>`, with the
session's id and `cwd`; a file with no header line gives those rows alone,
with an empty session id. Every line-by-line reader reports bad lines.

A session file that cannot be read is one whole-file row
`parser: <file> could not be read (<reason>)` through `error_rows`, `<file>` its name.

#### Fixtures

Fixture, one session file:

- `.pi/agent/sessions/--home-alice-proj--/2026-10-01T09-00-00-000Z_0199a1b2-7c3d-7e4f-8a5b-6c7d8e9f0a1b.jsonl`

```json
{"type":"session","version":3,"id":"0199a1b2-7c3d-7e4f-8a5b-6c7d8e9f0a1b","timestamp":"2026-10-01T09:00:00.000Z","cwd":"/home/alice/proj"}
{"type":"model_change","id":"1a2b3c4d","parentId":null,"timestamp":"2026-10-01T09:00:00.100Z","provider":"anthropic","modelId":"claude-sonnet-4-5"}
{"type":"message","id":"2b3c4d5e","parentId":"1a2b3c4d","timestamp":"2026-10-01T09:00:01.000Z","message":{"role":"user","content":"fix the failing test","timestamp":1759309201000}}
{"type":"message","id":"3c4d5e6f","parentId":"2b3c4d5e","timestamp":"2026-10-01T09:00:04.000Z","message":{"role":"assistant","content":[{"type":"thinking","thinking":"run the tests first"},{"type":"toolCall","id":"toolu_01","name":"bash","arguments":{"command":"npm test"}}],"api":"anthropic-messages","provider":"anthropic","model":"claude-sonnet-4-5","usage":{"input":900,"output":40,"cacheRead":0,"cacheWrite":0,"totalTokens":940,"cost":{"input":0,"output":0,"cacheRead":0,"cacheWrite":0,"total":0}},"stopReason":"toolUse","timestamp":1759309204000}}
{"type":"message","id":"4d5e6f70","parentId":"3c4d5e6f","timestamp":"2026-10-01T09:00:07.000Z","message":{"role":"toolResult","toolCallId":"toolu_01","toolName":"bash","content":[{"type":"text","text":"1 failing"}],"isError":false,"timestamp":1759309207000}}
{"type":"message","id":"5e6f7081","parentId":"4d5e6f70","timestamp":"2026-10-01T09:00:09.000Z","message":{"role":"bashExecution","command":"git status","output":"M src/a.ts","exitCode":0,"cancelled":false,"truncated":false,"timestamp":1759309209000}}
{"type":"session_info","id":"6f708192","parentId":"5e6f7081","timestamp":"2026-10-01T09:00:10.000Z","name":"test fix"}
this line is cut mid-wri
```

The last line exercises the bad-line `system` row ([Bad lines](#bad-line-rows)).

`pi-debug.log`, the screen lines shortened:

```text
Debug output at 2026-10-01T09:02:00.000Z
Terminal: 120x40
Total lines: 2

=== All rendered lines with visible widths ===
[0] (w=20) "fix the failing test"

=== Agent messages (JSONL) ===
{"role": "user", "content": "fix the failing test", "timestamp": 1790845201000}
{"role": "user", "content": "a prompt whose session file was deleted", "timestamp": 1790845300000}
```

`crashes.json`:

```json
[{"timestamp": "2026-10-01T09:05:00.000Z", "version": "1.3.0", "kind": "uncaught_exception", "message": "Cannot read properties of undefined", "stack": "TypeError: ...", "sessionFile": "/home/alice/.pi/agent/sessions/--srv-proj--/2026-10-01T09-00-00-000Z_0199a1b2-7c3d-7e4f-8a5b-6c7d8e9f0a1b.jsonl", "cwd": "/srv/proj", "notified": true}]
```

#### Wrapper attribution

The header names no producer, so attribution
rests on what the wrapper leaves:

- *little-coder* writes into pi's own directory. The rows stay agent `pi`, as the manifest says, and
  the session gets one `system` row `session driven by little-coder (pi launcher): <why>` when a
  `custom_message` has `customType` starting `lc-`
  ([_shared/inject.ts:49-64](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/.pi/extensions/_shared/inject.ts#L49-L64);
  full list in [little-coder.md](little-coder.md)), when an assistant `provider` is `llamacpp`
  ([llama-cpp-provider/index.ts:172](https://github.com/itayinbarr/little-coder/blob/89d4fa0af864230527ab75d12604ed0eb320e6df/.pi/extensions/llama-cpp-provider/index.ts#L172);
  pi's own built-in is `llama.cpp`,
  [llama/provider.ts:22](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/coding-agent/src/extensions/llama/provider.ts#L22)),
  or when `.little-coder/checkpoints/<this file name>/` exists in the same home.
- *Letta Code* writes the format under `.letta/lc-local-backend/conversations/*/messages.jsonl`, so
  the path already gives `letta`; inside, `message.id` starts `letta-msg-` and
  `message.metadata.agent_id` is set, fields pi never writes
  ([local-store.ts:135-137](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/backend/local/local-store.ts#L135-L137),
  [local-message.ts:26-46](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/backend/local/local-message.ts#L26-L46);
  `manifest.json` `message_format:"pi-session-entry-jsonl"`,
  [local-transcript.ts:24-46](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/backend/local/local-transcript.ts#L24-L46)).
  Share the pi parser, keep the agent from the catalog.
- Otherwise `pi`. An SDK embedder or rebranded build is not
  distinguishable from the file alone.

#### Source host rows

The agent records no source-host integration. `/share` without Radius runs the user's own `gh`
CLI, after `gh auth status`, to create a secret gist
([session-share.ts:79-83](https://github.com/earendil-works/pi/blob/5c1736f82d201d09664642ab0d88147da94b450e/packages/coding-agent/src/modes/interactive/session-share.ts#L79-L83),
[182](https://github.com/earendil-works/pi/blob/5c1736f82d201d09664642ab0d88147da94b450e/packages/coding-agent/src/modes/interactive/session-share.ts#L182)),
so the GitHub token is `gh`'s and pi stores no account or token for it
([Sync, relay and origin rows](#sync-relay-and-origin-rows)). The GitHub Copilot model provider's
OAuth grant, scope `read:user`, is a model credential kept in `auth.json` as `{type, refresh,
access, expires, enterpriseUrl}` with no login
([packages/ai/src/auth/oauth/github-copilot.ts:215-218](https://github.com/earendil-works/pi/blob/5c1736f82d201d09664642ab0d88147da94b450e/packages/ai/src/auth/oauth/github-copilot.ts#L215-L218),
[341-347](https://github.com/earendil-works/pi/blob/5c1736f82d201d09664642ab0d88147da94b450e/packages/ai/src/auth/oauth/github-copilot.ts#L341-L347)), and the
`git/<host>/<path>` package checkouts are installs ([Per-user storage](#per-user-storage)). A
grep of `packages/coding-agent/src` and `packages/ai/src` at
[`5c1736f`](https://github.com/earendil-works/pi/commit/5c1736f82d201d09664642ab0d88147da94b450e)
for `github`, `gitlab`, `bitbucket`, `oauth`, `repositor`, `pull request` and `clone` finds no
repository list and no repository-bound conversation.

## Confidence

- High (source): the agent directory, its override, every file name in
  [Per-user storage](#per-user-storage), the session directory encoding and file naming, credential
  file shapes and project `.pi` contents.
- High (source): the header, entry types, roles, field names and timestamp units of
  [Record schema](#record-schema).
- High: that the layout is the same on Windows (plain `homedir()` join).
- Medium: `system`-role persistence on versions between 0.83.0 and HEAD (taken from the ai
  changelog's 0.86.0 note, not bisected).
- Medium: whether `.pi/mcp.json` holds literal tokens in practice (the documented form is
  `${ENV}` interpolation).
- Not determined: the durable SQLite `record` JSON shape
  ([Experimental durable sessions](#experimental-durable-sessions)).
- Not determined: whether a rebranded pi build (`piConfig.configDir`) is in use anywhere, which
  would need its own catalog line.
