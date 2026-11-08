# amp

Amp is a closed-source coding agent, first published by Sourcegraph as `@sourcegraph/amp` and now as
`@ampcode/cli`, whose command-line binary `amp` also drives desktop runners and an orb sandbox (the
binary's name for its sandbox environment). Its parser is in the backlog
([issue 53](https://github.com/seanthegeek/doubleagent/issues/53)).

## Source and evidence

- Closed source. npm `@sourcegraph/amp` is now an alias whose only dependency is `@ampcode/cli`
  (its `package.json` says "Renamed to @ampcode/cli"). `@ampcode/cli`
  0.0.1791060199-g5bacb8 is a wrapper (`install.cjs`, `cli-wrapper.cjs`) over per-platform packages;
  the Linux binary is `@ampcode/cli-linux-x64/amp`, a bun-compiled ELF (128 MB). Evidence level
  **B** is strings from that binary: `strings -n 8` (cited as `amp.strings`) and `strings -el`
  UTF-16 (`amp.strings16`); the UTF-16 table holds the minified application code and is where the
  path joins live. The cited binary is `amp` from npm `@ampcode/cli-linux-x64`
  0.0.1791060199-g5bacb8 (linux-x64), 133764576 bytes, sha256
  `9eb4311859a492a675c4a7387fc4ad263e799cbdcb5dc9b8b392f804f387f04b`; a citation `amp @<offset>`
  is a 0-based byte offset into that file, where the quoted UTF-16 text starts.
- Evidence level **D** is the official docs, fetched as Markdown from
  `https://ampcode.com/docs/markdown/<page>` (index `https://ampcode.com/llms.txt`).
- Evidence level **S** is the install script `https://ampcode.com/install.sh`, cited as
  `amp-install.sh`; the copy read (fetched 2026-11-06) is 13614 bytes, sha256
  `83b0a61b09cb653b92e47e039e3aba8bda60c62c0eea6e1467c1b1b12c258cc9`.
- Thread transcripts are server-side: the bug-report bundler writes "The full thread is stored
  server-side and can be fetched by a site admin" (`amp.strings` near `thread/full.json`); docs say
  remote MCP definitions and threads are "stored by ampcode.com" (`customize_mcp.md:15,301`). No
  local thread transcript store was found in either strings table; only per-thread *logs*
  ([Per-user storage](#per-user-storage)).

### Observed

No real install.

### Source reviews

None.

## Collection

### Per-user storage

Path resolution in the CLI code has no platform branching, so the layout is identical on Linux,
macOS and Windows (B, `amp.strings16`):

```text
fzt=process.env.XDG_DATA_HOME||join(home,".local/share")
dzt=process.env.XDG_CONFIG_HOME||join(home,".config")
tg =join(fzt,"amp")                    # data dir
DL =join(dzt,"amp","settings.json")    # settings default
F6t=join(process.env.XDG_CACHE_HOME??join(home,".cache"),"amp"), us=F6t   # cache/log dir
```

One auxiliary module (ripgrep download) differs: data = `~/.local/share/amp` even when
`XDG_DATA_HOME` is set on win32/darwin, and cache = `%LOCALAPPDATA%\amp` on win32 (`La=...` in
`amp.strings`). Only `<that cache>/bin/<rg>` is written there (`It.join(Pt,"bin")`).

#### Scratch directories

None known.

#### Relocation variables

Each `amp @<offset>` below is the 0-based offset of the variable name in the binary named in
[Source and evidence](#source-and-evidence), and the snippet is its minified UTF-16 text. The data directory is that path plus `amp`.

| Variable | Moves | Replaces | Table agent |
| --- | --- | --- | --- |
| `AMP_HOME` | the installer home ([install.sh lines 5-6](https://ampcode.com/install.sh): `AMP_HOME="${AMP_HOME:-$HOME/.amp}"`) | `.amp` | `amp` |
| `XDG_DATA_HOME` | the data directory (`amp @124386424` `fzt=process.env.XDG_DATA_HOME\|\|gl.join(PUe(),".local/share")`) | `.local/share` | `shared` |
| `XDG_CONFIG_HOME` | the config directory (`amp @124386546` `dzt=process.env.XDG_CONFIG_HOME\|\|gl.join(PUe(),".config")`) | `.config` | `shared` |
| `XDG_CACHE_HOME` | the cache and log directory (`amp @124244258` `F6t=IW.join(process.env.XDG_CACHE_HOME??IW.join(gUe(),".cache"),"amp")`; logs under it, `amp @124994480` `KGe=Yp.join(us,"logs")`) | `.cache` | `shared` |

Not relocation variables: `AMP_API_KEY` (carries a key).
Not confirmed: `AMP_SETTINGS_FILE` and `AMP_LOG_FILE`, named in the strings tables but in no
documentation page checked.

#### Configuration directory

Config `~/.config/amp/` (B+D `cli_settings.md:10-14`):

- `settings.json` or `settings.jsonc` (`.jsonc` fallback string "Settings file not found, falling
  back to .jsonc"); overridable by `--settings-file` / `AMP_SETTINGS_FILE`.
- `AGENTS.md` and also `~/.config/AGENTS.md`, `~/.config/AGENT.md` (`customize_agents-md.md:38`,
  `"~/.config/AGENT.md"` in code).
- `skills/` (`join(home,".config","amp","skills")`), `plugins/*.ts` (`customize_plugins`),
  `obelisk-app.env` (orb sandbox only).
- Shared agent dirs also read: `~/.config/agents/skills/`, `~/.agents/skills/`
  (`customize_skills.md:36-38`).

#### Data directory

Data `~/.local/share/amp/` (B):

- `secrets.json` (`Fnt(H)=join(H.dataDir,VCr)`, `VCr="secrets.json"`), `accounts.json`
  (`join(H,"accounts.json")` in class `DGe`), `accounts/<identityKey>-<server>/secrets.json` for
  non-legacy multi-account (`join(dirname(H),"accounts",ED.directoryName,basename(H))`;
  `dataDir:join(tg,"accounts")`).
- `history.jsonl` prompt history, entries `{text, cwd}` (`rgr=join(Dit,"history.jsonl")`,
  `EiH(H){return H.text.length+(H.cwd?.length??0)}`; migrates from `history.json`).
- `device-id.json`, `guest-id`, `net/` (Amp Net sockets), `oauth/locks`,
  `orb-services/ad-hoc-services.json`, `runner/`, `daemon/`, `ide/` (IDE auth tokens: schema has
  `authToken`).

#### Cache directory

Cache `~/.cache/amp/` (B+D):

- `logs/cli.log` (10 MB rotate, 2 files), `logs/threads/<T-id>.log` per-thread logs, 7-day prune
  (`KGe=join(us,"logs"),LV=join(KGe,"threads"),JQ=join(KGe,"cli.log")`, `Y_$=604800000`),
  `logs/headless.log`, `logs/orb-services/`, `logs/orb-provider/`.
- `terminal/<threadId>/history` terminal-tool shell history (`iO(us,"terminal",threadId)`).
- `bin/rg` downloaded ripgrep, `pids/`, `portal-speed-test/latest.json`,
  `runner-desktop/<runner-id>/` compositor state (`cli_runners.md:84`), `desktop/` (orb desktop
  logs), `obelisk-runs/`.

#### Installer home

Installer home `~/.amp/` (`AMP_HOME`, S `amp-install.sh:5-6`; B `join(home,".amp")`):

- `bin/amp`, `bin/amp-desktop-helper/<ver>/`, `bin/waymote/<ver>/` (`cli_runners.md:58,78`),
  `signing-key.pub`, `amp-install-signature.minisign`.
- `oauth/<server>-client.json` MCP OAuth client registrations
  (`join(HOME||USERPROFILE,".amp","oauth")`, `getClientInfoPath`).
- `claude-diagnostics/<T-id>/sessions/` copies of Claude Code session files when Amp drives Claude
  Code as an external agent (`join(home,".amp","claude-diagnostics",thread,"sessions")`).

#### System-wide files

System-wide (not per-user): `/etc/ampcode/`, `/Library/Application Support/ampcode/`,
`%ProgramData%\ampcode\` holding `managed-settings.json` and `AGENTS.md` (`cli_settings.md:261-263`,
`customize_agents-md.md:28`).

### Credentials

- Primary store is the OS keyring via `@napi-rs/keyring` (keyring-rs 3.6.3; service
  `amp.cli.<kind>`, account `amp.cli.account`). File fallback `~/.local/share/amp/secrets.json`
  (logger child `secrets.file`) and per-account `accounts/*/secrets.json`. Secret keys: `apiKey`,
  `mcp-oauth-client-secret`, `mcp-oauth-token`, `oauth-refresh-token` (`lzt=[...]`). (B)
- `accounts.json` lists saved accounts (email, server) — identity evidence, not a token.
- `~/.amp/oauth/*-client.json`: OAuth client info; `clientSecret` is stripped to secret storage
  before writing (`let{clientSecret:t,...r}=e`). (B)
- `~/.local/share/amp/ide/*` IDE connection records include `authToken` (`amp @124393700`
  `AAH=zpH.join(Oo,"ide")`, `amp @124393488` `ideName:R(),authToken:R(),pid:GH()`), so the
  secret glob `.local/share/amp/ide/*` flags every file there from collector 1.22.0. (B)
- `settings.json` can embed MCP server `env`/headers (`amp.mcpServers`, `customize_mcp.md:244-245`),
  so `settings.json` and its `settings.jsonc` fallback are flagged (`.config/amp/settings.json*`).
  A project's `.amp/settings.json` is the workspace scope of the same `amp.mcpServers` setting,
  whose servers need approval before they run (D
  [Amp MCP documentation](https://ampcode.com/docs/markdown/customize/mcp#mcp-loading-order), fetched 2026-10-29,
  `customize/mcp.md:333,341`), so `.amp/settings.json*` flags it too.
  `AMP_API_KEY` env var.
- No `secrets.json` under `.config/amp` occurs anywhere in the binary; the file fallback is the
  data directory's.

### Exclusions

`.amp/bin`. The catalog reaches only `logs` and `terminal` under `.cache/amp` and only `logs`
under `AppData/Local/amp`, so `.cache/amp/runner-desktop`, `desktop`, `bin`, `obelisk-runs` and
`pids`, and `AppData/Local/amp/bin`, are not collected and need no exclusion.

### Project-local files

`.amp/settings.json` or `.amp/settings.jsonc` searched upward (`join(o,".amp","settings.json")`),
`.amp/plugins/*.ts`, `.amp/portals/*.json`, `.amp/portal-proxy.mjs`, `.amp/live-sync.pid`,
`.amp/terminal/history` (legacy), `.amp/cache/ios-simulator-derived-data`, `.amp/services.yaml`,
`.amp/obelisk.yaml`; `AGENTS.md` (fallback `AGENT.md`, `CLAUDE.md`) at every level up to `$HOME`
(`customize_agents-md.md:36-44`); `.agents/skills/`. Commits carry an `Amp-Thread-ID` trailer
(`threads.md:136`).

### Where the project path is recorded

`~/.local/share/amp/history.jsonl`: JSONL, key `cwd` per entry (B).
`~/.cache/amp/logs/threads/<T-id>.log` lines carry a thread id; their cwd content is unverified.
No local thread index.

## Parsing

Amp is `backlog`: the parser is [issue 53](https://github.com/seanthegeek/doubleagent/issues/53).
The full threads are server-side ([Source and evidence](#source-and-evidence)), so a parser would
read what stays local: the prompt history, the per-thread and CLI logs, the terminal tool's shell
history and the Claude Code session copies ([Per-user storage](#per-user-storage)). No record schema
has been researched yet.

## Confidence

- High: the config, data and cache resolution, `settings.json(c)`, `secrets.json`,
  `accounts.json`, `history.jsonl`, `logs/cli.log`, `logs/threads`, `~/.amp/bin`, `~/.amp/oauth`,
  the keyring use and the server-side threads.
- Medium: the `history.jsonl` `cwd` key (inferred from size accounting and type), the
  `accounts/<dir>/secrets.json` naming and the `ide/` contents.
- Low: whether `AppData/Local/amp` ever holds anything but `bin`.
- Not determined: the exact JSON schema of `secrets.json` and `accounts.json`; whether
  `claude-diagnostics` is created outside orb sandboxes; the Windows keyring behaviour
  (Credential Manager through keyring-rs is assumed).
