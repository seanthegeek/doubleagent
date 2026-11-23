# kiro

The catalog agent `kiro` covers the Kiro CLI, a closed-source command-line agent; its
open-source predecessor the Amazon Q Developer CLI, which it continues; and the Kiro IDE, a
VS Code fork. Evidence levels below are L1 (source or shipped binary), L2
(kiro.dev documentation), L3 (vendor issue tracker) and L4 (community source). Paths cited are
relative to the clone; `C=` means `crates/chat-cli/src`. The agent is parsed for the CLI store
only: the `kiro` parser reads the Amazon Q Developer CLI's `data.sqlite3` conversations, `/save`
exports, prompt history, agent configurations and delegate records ([Parser plan](#parser-plan)).

## Source and evidence

- L1 source: [aws/amazon-q-developer-cli](https://github.com/aws/amazon-q-developer-cli) at
  [`15cc8f3cd18c4272925ce1c7053268eedff1ea0a`](https://github.com/aws/amazon-q-developer-cli/commit/15cc8f3cd18c4272925ce1c7053268eedff1ea0a)
  (2026-04-23),
  [`crates/chat-cli/src/util/paths.rs`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/util/paths.rs),
  [`database/mod.rs`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/database/mod.rs),
  [`database/sqlite_migrations/`](https://github.com/aws/amazon-q-developer-cli/tree/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/database/sqlite_migrations),
  [`cli/chat/conversation.rs`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/conversation.rs).
  Cited as `Q paths.rs:N`.
- L1 binary strings: Kiro CLI 2.27.1, `kirocli-x86_64-linux.zip` from
  `desktop-release.q.us-east-1.amazonaws.com/latest/` (BUILD_HASH 7c1a246f, 2026-10-02), extracted
  to `scratchpad/npm/kiro/kirocli/`; the same URL path with `2.27.1/` in place of `latest/` still
  serves the build (161118272 bytes, sha256
  `e68927fe0bffb752746a573010cb68792b0f3e97be3a6ce73792c9cc02f08007`, re-downloaded and read
  without running it). `bin/kiro-cli-chat` is the agent (crate `chat-cli-v2`);
  `bin/kiro-cli` is the autocomplete shell wrapper. Strings in
  `scratchpad/npm/kiro/chat-strings.txt`.
- L2 kiro.dev docs: `reference/settings`, `configuration`, `permissions`,
  `cli/chat/session-management`, `cli/v3/new-features`, `mcp/configuration`,
  `custom-agents/configuration-reference`, `steering`, `hooks`, `powers`,
  `upgrade-guides/migrating-from-q`, `getting-started/authentication`.
- L3 [kirodotdev/Kiro](https://github.com/kirodotdev/Kiro) issues
  [11282](https://github.com/kirodotdev/Kiro/issues/11282),
  [5727](https://github.com/kirodotdev/Kiro/issues/5727),
  [9977](https://github.com/kirodotdev/Kiro/issues/9977),
  [6780](https://github.com/kirodotdev/Kiro/issues/6780), PR
  [5755](https://github.com/kirodotdev/Kiro/pull/5755).
- L4 community source: [junhoyeo/tokscale](https://github.com/junhoyeo/tokscale) `sessions/kiro.rs`,
  [getagentseal/codeburn](https://github.com/getagentseal/codeburn) `docs/providers/kiro.md`,
  wakatime-cli `pkg/ai/kiro.go`, [hnewcity/KiroaaS](https://github.com/hnewcity/KiroaaS) `main.rs`,
  Kiro IDE `product.json` copy in [Bad3r/dotfiles](https://github.com/Bad3r/dotfiles).

The source checkout is workspace version 1.19.7
([`Cargo.toml:11`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/Cargo.toml#L11)),
dual licensed Apache-2.0 and MIT.
[`README.md:4`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/README.md#L4)
says the project is unmaintained except for security fixes and that the product continues as the
closed-source Kiro CLI. Two agent cores exist: `chat-cli` is the shipped `q chat` and the only one
that persists conversations; `crates/agent` is a newer loop with its own camelCase `Message` types
that is never written to disk here and is the most likely shape of what Kiro CLI stores, though that
is inference. Nothing in the repository mentions `.kiro`, `kiro-cli`, `sessions/` or
`workspace-roots`; those catalog paths come from the closed binary's strings and the kiro.dev
documentation, not from source, and their record shapes remain unverified.

### Observed

- No real install. No installed Kiro CLI binary or state existed on the research
  machine; the 2.27.1 binary above was only extracted for its strings.

### Source reviews

- 2026-11-23: aws/amazon-q-developer-cli at
  [15cc8f3](https://github.com/aws/amazon-q-developer-cli/commit/15cc8f3cd18c4272925ce1c7053268eedff1ea0a),
  still the default branch's head, and kkawakam/rustyline at
  [bc06b3e](https://github.com/kkawakam/rustyline/commit/bc06b3eb322df1a535e6e3b64db8c0fee0b1df8c),
  against the Go parser and the catalog: added how a conversation's model is chosen, the agent
  file's full key set and every tool's settings keys, the `/save` naming and the tangent start time,
  and rewrote the Parser plan from the parser's code and row texts.
- 2026-10-21: aws/amazon-q-developer-cli at
  [15cc8f3](https://github.com/aws/amazon-q-developer-cli/commit/15cc8f3cd18c4272925ce1c7053268eedff1ea0a)
  (1.19.7), with kkawakam/rustyline at
  [bc06b3e](https://github.com/kkawakam/rustyline/commit/bc06b3eb322df1a535e6e3b64db8c0fee0b1df8c)
  (v15.0.0, the version `Cargo.lock` pins): added the prompt history's file format, the
  interrupt text, the conversion of a deny-rule prompt into a cancelled tool use, and the
  delegate record's defaults, and turned the permission, past-use and bad-line plans into what
  the parser now reads ([Parser plan](#parser-plan)); the Kiro CLI forms are unchanged.
- 2026-10-19: HEAD of aws/amazon-q-developer-cli is still
  [`15cc8f3`](https://github.com/aws/amazon-q-developer-cli/commit/15cc8f3cd18c4272925ce1c7053268eedff1ea0a),
  the commit cited here; the repository is frozen, so the open-source paths have not moved. Added
  the permission records, the prompt history, the delegate records, the shadow checkpoint
  repository, the copies inside the conversation blob and the absence of any sync
  ([Transcript stores](#transcript-stores), [Record schema](#record-schema),
  [Other stores](#other-stores), [Parser plan](#parser-plan)); the Kiro CLI paths still rest on
  the 2.27.1 binary's strings and the documentation.

## Collection

### Per-user storage

Layout is the same on Linux, macOS and Windows except the platform data dir.

#### Scratch directories

`.aws/amazonq/cli-checkouts/<conversation_id>` is the CLI's per-conversation checkpoint
repository, deleted when the session ends
([Shadow checkpoint repository](#shadow-checkpoint-repository)). It is excluded
([Exclusions](#exclusions)) and never recorded as a project; the CLI has no working directory of
its own.

#### Relocation variables

| Variable | Moves | Replaces | Table agent |
| --- | --- | --- | --- |
| `KIRO_HOME` | the parent of `.kiro`, the Kiro CLI home with its `sessions/` store, so the value stands in for the home itself ([Settings, Environment variables](https://kiro.dev/docs/reference/settings/#environment-variables); kiro 2.27.1 linux-x86_64 `bin/kiro-cli-chat` @99590959 `KIRO_HOME/.kiro/sessions`, in the help text "Defaults to KIRO_HOME/.kiro/sessions") | `.` | `kiro` |

Not relocation variables: `KIRO_API_KEY` (carries a key), `KIRO_LOG_LEVEL` (behaviour only),
`KIRO_TEST_SESSIONS_DIR` and `KIRO_TEST_SETTINGS_PATH` (test only). Not confirmed:
`KIRO_ACP_RECORD_PATH`, said to add a JSONL recording of the agent client protocol wire, is on
no [Settings](https://kiro.dev/docs/reference/settings/) page and in neither the 2.27.1 nor the
3.1.0 binary, which hold `KIRO_RECORD_API_REQUESTS_PATH` and `KIRO_RECORD_API_RESPONSES_PATH`
instead; `Q_CLI_DATA_DIR`
moves the data directory in the unpersisted `crates/agent` core
([`consts.rs:27`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/agent/src/agent/util/consts.rs#L27)),
but the shipped `chat-cli` data path
([`paths.rs:327-331`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/util/paths.rs#L327-L331))
does not read it.

#### Kiro CLI

Kiro CLI home `~/.kiro` (override `KIRO_HOME`, docs reference/settings; binary help "Defaults to
KIRO_HOME/.kiro/sessions"):

- `settings/cli.json`, `settings/permissions.yaml`, `settings/mcp.json`, `settings/lsp.json` (docs;
  binary), `settings.json` mentioned in the binary's prompt text.
- `agents/*.json` (legacy 2.x profiles as `*.json.bak`), `prompts/`, `steering/` with
  `steering/AGENTS.md`, `skills/`, `hooks/`, `workflows/`,
  `powers/installed/<name>/{plugin.json,skills/,mcp.json}` (docs configuration, custom-agents,
  powers, permissions).
- `sessions/cli/<id>.json` header with `session_id`, `cwd` plus `<id>.jsonl` conversation (tokscale
  `:4, 13, 134-135`); `sessions/<workspace-hash>/sess_<uuid>/session.json` (`workspacePaths`,
  `modelId`, `createdAt`) and `messages.jsonl`, written by the IDE and the CLI's v3 session store
  (tokscale `:8-9, 171-179, 569-590`; codeburn; binary `sess_<uuid>`); `session-index/<hash>.jsonl`;
  `workspace-roots/<hash(root)>/.trust-migration.json` with key `root` and `permissions.yaml`
  ([kirodotdev/Kiro issue 11282](https://github.com/kirodotdev/Kiro/issues/11282); docs
  permissions).
- State stores: `memories/` (memory database), `sandbox-state/`, `web-session/`, `cloud-cache/`,
  `sandbox-curl/` (docs permissions). `.default` config marker in `install.sh`.
- Legacy Amazon Q:
  `~/.aws/amazonq/{cli-agents,prompts,mcp.json,cli-checkouts,.cli_bash_history,global_context.json,profiles,knowledge_bases}`
  (Q
  [`paths.rs:59-66`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/util/paths.rs#L59-L66));
  the 2.27.1 binary still carries these names and `.aws/amazonq`. `.cli_bash_history` is the chat
  prompt's history ([Prompt history `.cli_bash_history`](#prompt-history-cli_bash_history)), and
  `cli-agents/<name>.json` are agent configurations holding the standing tool grants and the
  agent's `model` ([Permission records](#permission-records)).
- Platform data dir via `dirs::data_local_dir()`: Linux `~/.local/share/<app>/`, macOS
  `~/Library/Application Support/<app>/`, Windows `%LOCALAPPDATA%\<app>\`, holding `settings.json`
  (Q
  [`paths.rs:307-311`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/util/paths.rs#L307-L311),
  one pretty-printed JSON object of dotted keys,
  [`crates/chat-cli/src/database/settings.rs:200-214`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/database/settings.rs#L200-L214),
  [`:281`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/database/settings.rs#L281),
  such as the default model, the default agent and the checkpoint switch,
  [`settings.rs:126-134`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/database/settings.rs#L126-L134))
  and `data.sqlite3` (Q
  [`paths.rs:327-331`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/util/paths.rs#L327-L331)).
  `<app>` is `amazon-q` in the Q source; `kiro-cli` per community tools (KiroaaS `:523-526`,
  tokscale `:6`). The catalog collects both names on each platform: `.local/share/amazon-q`,
  `Library/Application Support/amazon-q`, `AppData/Local/amazon-q`, `.local/share/kiro-cli`,
  `Library/Application Support/kiro-cli` and `AppData/Local/kiro-cli`. The binary contains
  `data.sqlite3` and `KIRO_TEST_SETTINGS_PATH` in
  `chat-cli-v2/src/util/paths.rs` but the directory literal is not visible; docs say only "SQLite
  database in `~/.kiro/`" (session-management).
- Logs: `$TMPDIR/kiro-log/kiro-chat.log` (binary; docs migrating-from-q), legacy `$TMPDIR/qlog` (Q
  [`paths.rs:143-151`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/util/paths.rs#L143-L151)),
  Windows `%TEMP%\amazon-q\logs`. Outside home.
- Binaries: `~/.local/bin/{kiro-cli,kiro-cli-chat,kiro-cli-term,q,qchat}` (install.sh), Kiro Crew
  via `download.crew.kiro.dev/cli.sh` (binary).

#### Kiro IDE

Kiro IDE (VS Code fork, `nameShort` Kiro, `dataFolderName` `.kiro`, `serverDataFolderName`
`.kiro-server`, L4 product.json): `~/Library/Application Support/Kiro/User`, `~/.config/Kiro/User`
(`$XDG_CONFIG_HOME`), `%APPDATA%\Kiro\User`; remote `~/.kiro-server/data/User` (codeburn; wakatime
`:505-513`). Chat state in `User/globalStorage/kiro.kiroagent/`: `<workspace-hash>/<execution>.chat`
legacy, extensionless `<workspace-hash>/<session>/<execution>` (post Feb 2026),
`workspace-sessions/<base64url(path)>/<uuid>.json` v1 records, `index/` code-search index,
`config.json`, `profile.json`, `.migrations/`, a LanceDB vector index (codeburn;
[kirodotdev/Kiro pull request 5755](https://github.com/kirodotdev/Kiro/pull/5755);
[kirodotdev/Kiro issue 6780](https://github.com/kirodotdev/Kiro/issues/6780): 4.8 GB sessions plus
564 MB LanceDB; [kirodotdev/Kiro issue 9977](https://github.com/kirodotdev/Kiro/issues/9977): 25 GB
across 31 hash dirs). Extensions in `~/.kiro/extensions` share the CLI's home. The catalog also
collects the IDE's logs beside `User`, `.config/Kiro/logs`, `Library/Application Support/Kiro/logs`
and `AppData/Roaming/Kiro/logs`, where a VS Code fork keeps them; no Kiro source or install
confirms the name.

#### Environment variables

Env: `KIRO_HOME`, `KIRO_API_KEY`, `KIRO_LOG_LEVEL`, `KIRO_TEST_SESSIONS_DIR`,
`KIRO_TEST_SETTINGS_PATH` (binary); see [Relocation variables](#relocation-variables) for the
unconfirmed `KIRO_ACP_RECORD_PATH`.

### Credentials

- `data.sqlite3` table `auth_kv(key, value)` holds Builder ID / IAM Identity Center tokens,
  deliberately separate from `state` (Q
  [`database/sqlite_migrations/005_auth_table.sql`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/database/sqlite_migrations/005_auth_table.sql);
  [`mod.rs:163`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/database/mod.rs#L163),[`172`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/database/mod.rs#L172);
  identical migration text in the 2.27.1 binary), so the catalog flags the whole file, with its
  `-journal` sidecar ([SQLite specifics](#sqlite-specifics)), as a secret through
  `*/amazon-q/data.sqlite3*` and `*/kiro-cli/data.sqlite3*`.
- MCP OAuth tokens in `~/.aws/sso/cache/*.json` (Q
  [`paths.rs:314-316`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/util/paths.rs#L314-L316);
  binary `~/.aws/sso/cache`), flagged by `.aws/sso/cache/*.json`; KiroaaS also probes
  `~/.aws/sso/cache/kiro-auth-token.json` (L4), which the same glob flags.
- `KIRO_API_KEY` (`ksk_...`) env var for headless use (docs authentication); no file store
  documented.
- Files that embed keys: `~/.kiro/settings/mcp.json`, `.kiro/settings/mcp.json`,
  `~/.kiro/powers/installed/*/mcp.json`, agent JSON MCP server blocks (docs mcp/configuration,
  permissions), the legacy `~/.aws/amazonq/mcp.json` and project `.amazonq/mcp.json`. All are
  flagged, the agent files as `.kiro/agents/*.json`, `.aws/amazonq/cli-agents/*.json` and a
  project's `.amazonq/cli-agents/*.json`. `~/.kiro/web-session/` by name holds browser session
  state (docs permissions); its files are flagged by `.kiro/web-session/*`.
  Binary string `secrets.json` in `owner_only_sweep.rs` suggests an owner-only
  `~/.kiro/secrets.json`, unverified; the catalog flags `.kiro/secrets.json`.

### Exclusions

`.aws/amazonq/cli-checkouts` (shadow repos, Q
[`paths.rs:62`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/util/paths.rs#L62)),
`.aws/amazonq/knowledge_bases` (Q
[`paths.rs:66`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/util/paths.rs#L66)),
`.kiro/cloud-cache`, `.kiro/sandbox-state`, `.kiro/sandbox-curl` (names only, docs permissions),
`*/User/globalStorage/kiro.kiroagent/index` and `*lance*`
([kirodotdev/Kiro pull request 5755](https://github.com/kirodotdev/Kiro/pull/5755);
[kirodotdev/Kiro issue 6780](https://github.com/kirodotdev/Kiro/issues/6780)), `.kiro/extensions`
(IDE extension payloads). `.kiro/powers` should not be excluded wholesale: powers are skills,
`mcp.json` and knowledge files (docs powers), and `powers/installed/*/mcp.json` can carry
credentials.

### Project-local files

`.kiro/settings/mcp.json`, `.kiro/settings/lsp.json`, `.kiro/agents/`, `.kiro/steering/*.md`,
`.kiro/skills/*/SKILL.md`, `.kiro/hooks/<id>.json`, `.kiro/specs/`, `.kiro/workflows/`,
`.kiro/prompts/`, `.kiroignore`, `AGENTS.md` at root and in subdirectories (docs configuration,
steering, hooks, permissions); legacy
`.amazonq/{cli-agents,prompts,mcp.json,cli-todo-lists,.subagents,rules/**/*.md}` (Q
[`paths.rs:46-51`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/util/paths.rs#L46-L51))
and `AmazonQ.md` (Q
[`conversation.rs:1285`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/conversation.rs#L1285),
[`paths.rs:54`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/util/paths.rs#L54)).
No per-project database. `.amazonq/.subagents/<agent>.json` records the last background task the
`delegate` tool ran for each agent
([Delegate records `.amazonq/.subagents`](#delegate-records-amazonqsubagents)).

### Where the project path is recorded

- `~/.kiro/workspace-roots/<hash>/.trust-migration.json` key `root`
  ([kirodotdev/Kiro issue 11282](https://github.com/kirodotdev/Kiro/issues/11282)).
- `~/.kiro/sessions/*/sess_*/session.json` key `workspacePaths` (array) (tokscale `:178-179`);
  `~/.kiro/sessions/cli/*.json` key `cwd` (tokscale `:135`).
- `data.sqlite3` table `conversations(key, value)` keyed by cwd (Q
  [`mod.rs:402-410`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/database/mod.rs#L402-L410),
  [migration 007](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/database/sqlite_migrations/007_conversations_table.sql));
  2.27.1 adds `conversations_v2(key, conversation_id, value, created_at, updated_at)` (binary
  migration `008_multiple_conversations_per_path`), still keyed by path. SQLite, so left to the
  analyzer.
- IDE `kiro.kiroagent/workspace-sessions/<base64url(path)>/` directory name decodes to the workspace
  path (codeburn; wakatime glob `*/workspace-sessions/*/*.json`). Grep-able without SQLite.

## Parsing

### Transcript stores

`dirs::data_local_dir()/amazon-q/data.sqlite3`
([`C/util/paths.rs:327-331`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/util/paths.rs#L327-L331);
[`crates/agent/src/agent/util/directories.rs:16`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/agent/src/agent/util/directories.rs#L16),[`37-48`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/agent/src/agent/util/directories.rs#L37-L48),
overridable with `Q_CLI_DATA_DIR`,
[`crates/agent/src/agent/util/consts.rs:27`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/agent/src/agent/util/consts.rs#L27)):
that is `~/.local/share/amazon-q/`, `~/Library/Application Support/amazon-q/` and
`%LOCALAPPDATA%\amazon-q\`. Mode is forced to 0600 on Unix
([`C/database/mod.rs:212-223`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/database/mod.rs#L212-L223)).
The `kiro-cli` catalog variants are the unverified rebrand.

Table `conversations`: one row per working directory. The key is the cwd string and the value is
`serde_json::to_string(&ConversationState)`
([`C/database/mod.rs:385-410`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/database/mod.rs#L385-L410)),
written with `INSERT OR REPLACE`
([`mod.rs:470-475`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/database/mod.rs#L470-L475))
after every assistant turn
([`C/cli/chat/conversation.rs:403-422`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/conversation.rs#L403-L422)),
so only the latest conversation in each directory survives. Resume reads the same key
([`C/cli/chat/mod.rs:719-724`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/mod.rs#L719-L724)).

`/save <path>` writes `serde_json::to_string_pretty(&session.conversation)`, the same schema, to the
path exactly as typed, adding no extension and refusing an existing file without `--force`;
`/load` reads the path as typed and, when that fails and the path does not end in `.json`, the
path with `.json` appended
([`C/cli/chat/cli/persist.rs:60-75`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/cli/persist.rs#L60-L75),[`84-90`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/cli/persist.rs#L84-L90)).
`~/.aws/amazonq/.cli_bash_history`
([`paths.rs:63`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/util/paths.rs#L63))
and the shadow checkouts `~/.aws/amazonq/cli-checkouts`
([`paths.rs:62`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/util/paths.rs#L62))
are adjacent evidence, not transcripts ([Other stores](#other-stores)).

#### What survives a delete

A save replaces the directory's row, so only the latest conversation in each directory survives
(above), and the prompt history `.cli_bash_history` outlives `/clear`, the row replacement and a
compaction ([Prompt history `.cli_bash_history`](#prompt-history-cli_bash_history)). The
analyzer reads it as a secondary source, and reads the checkpoint snapshots and the tangent-mode
main history below for the entries `history` no longer holds
([Past-use rows and secondary sources](#past-use-rows-and-secondary-sources); see also
[Secondary sources](../docs/timeline-output.md#secondary-sources)); `transcript` is not read.

Three fields of `ConversationState` repeat history the parser reads from
`history`, and each outlives a change to it:

- `transcript`, up to 10,000 strings: each prompt as `> <text>` and each reply as
  `<text>\n[Tool uses: <names or none>]`
  ([`C/cli/chat/conversation.rs:892-907`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/conversation.rs#L892-L907),
  [`C/cli/chat/consts.rs:6`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/consts.rs#L6)).
  `/clear` resets `history`, `next_message` and `latest_summary` only
  ([`conversation.rs:226-231`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/conversation.rs#L226-L231),
  [`C/cli/chat/cli/clear.rs:50-51`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/cli/clear.rs#L50-L51)),
  and compaction drains `history` and keeps a summary
  ([`conversation.rs:732-741`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/conversation.rs#L732-L741)),
  so after either the texts survive here, without times.
- `checkpoint_manager.checkpoints[]`, when checkpoints are on: each `{tag, timestamp (local time
  with offset), description, history_snapshot, is_turn, tool_name}`, the snapshot a full copy of
  `history` at that turn or tool use, beside the manager's `shadow_repo_path`, `work_tree_path`
  (the working directory), `tag_index`, `current_turn`, `tools_in_turn`, `pending_user_message`,
  `message_locked` and `file_stats_cache`
  ([`C/cli/chat/checkpoint.rs:36-82`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/checkpoint.rs#L36-L82),
  [`conversation.rs:146`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/conversation.rs#L146)).
  `/checkpoint restore` replaces `history` with an earlier snapshot
  ([`conversation.rs:911-922`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/conversation.rs#L911-L922))
  and removes no checkpoint
  ([`checkpoint.rs:207-240`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/checkpoint.rs#L207-L240)),
  so the later checkpoints still hold the turns the rewind discarded.
- `tangent_state`, present while tangent mode is on: the main conversation's `main_history`,
  `main_next_message`, `main_transcript` and `main_latest_summary`, restored when tangent mode ends,
  and `tangent_start_time`, when tangent mode was entered (a `time::OffsetDateTime`, time 0.3.44
  with its `serde` feature per
  [`Cargo.toml:114`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/Cargo.toml#L114)
  and
  [`Cargo.lock:7232-7233`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/Cargo.lock#L7232-L7233);
  its JSON form was not read)
  ([`conversation.rs:149-167`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/conversation.rs#L149-L167),
  [`:238-262`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/conversation.rs#L238-L262)).
  A row saved during a tangent has the side conversation in `history` and the main one only here.

All three join `history` on the entries' own fields (a snapshot entry is
a `HistoryEntry` with the same timestamps and message ids) or, for
`transcript`, on text.

### Record schema

Migrations
([`C/database/mod.rs:67-76`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/database/mod.rs#L67-L76),
SQL in
[`C/database/sqlite_migrations/`](https://github.com/aws/amazon-q-developer-cli/tree/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/database/sqlite_migrations)):

```text
000 CREATE TABLE migrations (id INTEGER PRIMARY KEY, version INTEGER NOT NULL, migration_time INTEGER NOT NULL);
001 CREATE TABLE history (id INTEGER PRIMARY KEY, command TEXT, shell TEXT, pid INTEGER, session_id TEXT, cwd TEXT, time INTEGER, in_ssh INTEGER, in_docker INTEGER, hostname TEXT, exit_code INTEGER);
002 ALTER TABLE history DROP COLUMN in_ssh; DROP COLUMN in_docker;
003 ALTER TABLE history RENAME COLUMN time TO start_time; ADD COLUMN end_time INTEGER; ADD COLUMN duration INTEGER;
004 CREATE TABLE state (key TEXT PRIMARY KEY, value TEXT);
005 CREATE TABLE auth_kv (key TEXT PRIMARY KEY, value TEXT);
006 state recreated with value BLOB
007 CREATE TABLE conversations (key TEXT PRIMARY KEY, value TEXT);
```

`history` has no writer in this repository (legacy shell history from the
Fig lineage); old installs may still hold rows.

Serialised `ConversationState`
([`conversation.rs:106-152`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/conversation.rs#L106-L152);
default serde, snake_case, externally tagged enums): `conversation_id` (random,
[`conversation.rs:109`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/conversation.rs#L109)),
`next_message: UserMessage|null`, `history: [HistoryEntry]`, `valid_history_range: [int,int]`,
`transcript: [string]`, `tools: {"native___"|<mcp server>: [...]}`
([`tools/mod.rs:305-315`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/tools/mod.rs#L305-L315)),
`context_manager`, `checkpoint_manager`, `file_line_tracker`, `context_message_length` (opaque),
`latest_summary: [string, RequestMetadata]|null`, `model` (legacy, 1.13.3 and earlier),
`model_info: {model_name?, description?, model_id, context_window_tokens}`
([`cli/model.rs:26-37`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/cli/model.rs#L26-L37)),
`mcp_enabled`, `tangent_state` (optional,
[`conversation.rs:154-167`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/conversation.rs#L154-L167)).
`tool_manager` and `agents` are `#[serde(skip)]`.

`model_info` is the conversation's model, and each request sends its `model_id`
([`conversation.rs:621`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/conversation.rs#L621)),
which `request_metadata.model_id` records
([`parser.rs:216`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/parser.rs#L216)).
A new conversation takes, in this order: the `--model` option, and the command ends with an error
when the service does not list that model; the active agent's `model`, when the service lists it
(otherwise a warning and the next step); the `chat.defaultModel` setting, when one is saved and the
service lists it; and the service's default model
([`C/cli/chat/mod.rs:379-424`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/mod.rs#L379-L424)).
A name matches a model's `model_name` or `model_id` in any ASCII case, `claude-4-sonnet` read as
`claude-sonnet-4`
([`cli/model.rs:223-240`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/cli/model.rs#L223-L240)),
and the chosen id becomes `model_info`
([`conversation.rs:179-188`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/conversation.rs#L179-L188),
[`:210`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/conversation.rs#L210)).
`/model` replaces `model_info` for the rest of the session
([`cli/model.rs:141-143`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/cli/model.rs#L141-L143)).
A resumed conversation keeps its stored `model_info`, since the model chosen above is applied only
to a new one
([`mod.rs:726-760`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/mod.rs#L726-L760)),
and `/load` keeps the running session's `model_info`, not the file's
([`persist.rs:105`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/cli/persist.rs#L105)).
So the agent file's `model` and `chat.defaultModel` are inputs to that choice; neither is copied
into the conversation under its own name.

`HistoryEntry`
([`conversation.rs:91-97`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/conversation.rs#L91-L97)):
`{user: UserMessage, assistant: AssistantMessage, request_metadata: RequestMetadata|null}`.

`UserMessage`
([`message.rs:53-60`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/message.rs#L53-L60)):
`additional_context`, `env_context: {env_state: {operating_system, current_working_directory
(truncated to 256, consts.rs:3), environment_variables: [{key,value}]}}`
([`message.rs:422-425`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/message.rs#L422-L425),[`542-559`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/message.rs#L542-L559);
[`api_client/model.rs:724-728`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/api_client/model.rs#L724-L728),[`761-764`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/api_client/model.rs#L761-L764)),
`content`, `timestamp: string|null`, `images`. `content`
([`message.rs:62-76`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/message.rs#L62-L76))
is one of `{"Prompt":{"prompt":s}}`,
`{"CancelledToolUses":{"prompt":s|null, "tool_use_results":[...]}}`,
`{"ToolUseResults":{"tool_use_results":[...]}}`. `timestamp` is `chrono::DateTime<FixedOffset>` from
`Local::now().fixed_offset()`
([`conversation.rs:398`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/conversation.rs#L398)),
serialised RFC 3339 with the local offset, for example `2026-04-23T14:17:15.123456789-07:00`
(`chrono/src/datetime/serde.rs:32-46`, v0.4.42 per
[`Cargo.lock:1575-1576`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/Cargo.lock#L1575-L1576)).
Tool-result user messages may carry `null`.

`ToolUseResult`
([`message.rs:332-340`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/message.rs#L332-L340)):
`{tool_use_id, content: [{"Text": string} | {"Json": any}], status: "Success"|"Error"}`
([`message.rs:387-391`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/message.rs#L387-L391);
[`api_client/model.rs:491-495`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/api_client/model.rs#L491-L495)).

`AssistantMessage`
([`message.rs:435-448`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/message.rs#L435-L448)):
`{"Response":{"message_id":s|null, "content":s}}` or
`{"ToolUse":{"message_id":s|null,"content":s,"tool_uses": [AssistantToolUse]}}`. `AssistantToolUse`
([`message.rs:507-519`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/message.rs#L507-L519)):
`{id, name, orig_name, args, orig_args}`; `id` is the backend `tool_use_id`
([`message.rs:531-539`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/message.rs#L531-L539)).

`RequestMetadata`
([`C/cli/chat/parser.rs:687-715`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/parser.rs#L687-L715)):
`request_id`, `message_id`, `request_start_timestamp_ms`, `stream_end_timestamp_ms` (unix ms,
[`parser.rs:693-696`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/parser.rs#L693-L696)),
`time_to_first_chunk: {secs,nanos}|null`, `time_between_chunks`, `user_prompt_length`,
`response_size`, `chat_conversation_type: "NotToolUse"|"ToolUse"|null`
([`telemetry/core.rs:534-539`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/telemetry/core.rs#L534-L539)),
`tool_use_ids_and_names: [[id,name]]`, `model_id`,
`message_meta_tags: ["Compact"|"GenerateAgent"|"TangentMode"]`
([`core.rs:551-558`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/telemetry/core.rs#L551-L558)).

The newer `crates/agent` shape (not persisted here,
[`agent_loop/types.rs`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/agent/src/agent/agent_loop/types.rs)):
`ConversationState {id: uuid, messages: [Message]}`
([`types.rs:147-150`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/agent/src/agent/types.rs#L147-L150));
`Message {id, role: "user"|"assistant", content: [{"text": s} | {"toolUse": {toolUseId, name,
input}} | {"toolResult": {toolUseId, content: [{"text"}|{"json"}|{"image"}], status:
"success"|"error"}} | {"image": ...}], timestamp: unix seconds|null}`
([`types.rs:161-170`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/agent/src/agent/agent_loop/types.rs#L161-L170),[`310-316`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/agent/src/agent/agent_loop/types.rs#L310-L316),[`383-400`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/agent/src/agent/agent_loop/types.rs#L383-L400),
[`402-407`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/agent/src/agent/agent_loop/types.rs#L402-L407),[`426-431`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/agent/src/agent/agent_loop/types.rs#L426-L431),[`445-451`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/agent/src/agent/agent_loop/types.rs#L445-L451)).
Note camelCase and lower-case status.

#### Permission records

A tool call that needs approval waits for `y`, `n` or `t` at the prompt. `y` runs it; `t` runs it
and trusts the tool for the rest of the session through `agents.trust_tools`, in memory only, since
`agents` is `#[serde(skip)]`
([`C/cli/chat/mod.rs:2284-2306`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/mod.rs#L2284-L2306)).
Anything else abandons the pending calls: the next user message is `CancelledToolUses` whose
`prompt` is what the user typed, or, for a bare `n`, the fixed text `I deny this tool request. Ask a
follow up question clarifying the expected action`
([`:2325-2337`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/mod.rs#L2325-L2337)),
and each abandoned call gets a result with status `Error` and the text
`Tool use was cancelled by the user`
([`C/cli/chat/message.rs:120-140`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/message.rs#L120-L140)).
An approval leaves no record of its own: the call is followed by an ordinary `ToolUseResults`
message. A call whose arguments match a deny rule of the agent's `toolsSettings` is rejected without
asking, and the conversation continues with a `Prompt` user message `Tool use with <tool> was
rejected because the arguments supplied were forbidden`
([`:2392-2441`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/mod.rs#L2392-L2441)).
Because the previous assistant message holds the rejected call, the conversation invariants turn
such a `Prompt` into `CancelledToolUses` with the same `prompt` and a cancelled result per call
([`C/cli/chat/conversation.rs:1190-1210`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/conversation.rs#L1190-L1210)),
so the rejection text usually reaches the store as the prompt of a cancelled tool use. An
interrupt (Ctrl+C) during tool execution writes `CancelledToolUses` with the prompt `The user
interrupted the tool execution.`
([`mod.rs:941-950`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/mod.rs#L941-L950)).
`--trust-all-tools` and `/tools trust-all` set `trust_all_tools` on the same skipped field, so
neither reaches the database.

Standing grants are agent configuration files, global `~/.aws/amazonq/cli-agents/<name>.json` and
per project `.amazonq/cli-agents/<name>.json`
([`C/util/paths.rs:46`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/util/paths.rs#L46),
[`:59`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/util/paths.rs#L59)),
camelCase JSON that rejects unknown keys, with `$schema`, `name`, `description`, `prompt`,
`mcpServers`, `tools`, `toolAliases`, `allowedTools` (tools run without asking), `resources`,
`hooks`, `toolsSettings`, `useLegacyMcpJson` and `model` (the agent's model, an input to the choice
in [Record schema](#record-schema))
([`C/cli/agent/mod.rs:130-179`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/agent/mod.rs#L130-L179)).
`toolsSettings` maps a tool to its settings: `allowedCommands`, `deniedCommands`, `denyByDefault`
and `autoAllowReadonly` for `execute_bash`, read as `execute_cmd` on Windows
([`C/cli/chat/tools/execute/mod.rs:197-218`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/tools/execute/mod.rs#L197-L218));
`allowedPaths`, `deniedPaths` and `allowReadOnly` for `fs_read`
([`C/cli/chat/tools/fs_read.rs:105-121`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/tools/fs_read.rs#L105-L121));
`allowedPaths` and `deniedPaths` for `fs_write`
([`C/cli/chat/tools/fs_write.rs:469-477`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/tools/fs_write.rs#L469-L477));
and `allowedServices`, `deniedServices` and `autoAllowReadonly` for `use_aws`
([`C/cli/chat/tools/use_aws.rs:180-191`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/tools/use_aws.rs#L180-L191)).

### Joins and timestamps

All messages are embedded in the one JSON blob; `history[i]` is one
user/assistant pair in turn order, and `next_message` is a pending user
message. `history[i].assistant.ToolUse.tool_uses[].id` matches
`history[i+1].user.content.ToolUseResults.tool_use_results[].tool_use_id`
(or `CancelledToolUses`). `request_metadata.tool_use_ids_and_names`
duplicates the id to name map and `request_metadata.message_id` matches
`assistant.*.message_id`. cwd is the row `key` and, per message,
`user.env_context.env_state.current_working_directory`. No git branch is
recorded. Session id is `conversation_id`.

### Other stores

#### SQLite specifics

rusqlite 0.32 `bundled` via r2d2
([`Cargo.toml:82-89`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/Cargo.toml#L82-L89));
no `PRAGMA` anywhere, so the default rollback journal: no `-wal` or `-shm`, only a transient
`data.sqlite3-journal` during a write. A copied `data.sqlite3` alone is complete unless Kiro CLI
enabled WAL (unverified). Each write replaces the whole blob, so a mid-write copy is the old or the
new conversation, never a partial row. No encryption.

#### Prompt history `.cli_bash_history`

`.cli_bash_history` is the chat prompt's history, not a
shell's. The rustyline editor (version 15, `with-file-history`,
[`Cargo.toml:93`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/Cargo.toml#L93))
loads it at start and appends the session's lines when the input source is dropped
([`C/cli/chat/prompt.rs:575-595`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/prompt.rs#L575-L595),
[`C/cli/chat/input_source.rs:38-67`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/input_source.rs#L38-L67)).
Every line typed at the prompt is added except an empty or blank one, a line that is `y`, `n` or `t`
in either case once trimmed (the approval answers), and a line starting with a space
([`input_source.rs:112-140`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/input_source.rs#L112-L140),
[`prompt.rs:563-567`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/prompt.rs#L563-L567)),
slash commands included. Lines carry no time, session id or directory. It outlives `/clear`, the
per-directory row replacement and a compaction, so it is often the only record of an earlier
conversation's prompts. Secondary: it joins the conversation on prompt text only.

The file format is rustyline 15's `FileHistory`: a file it writes whole starts with the line `#V2`
([`src/history.rs:465-467`](https://github.com/kkawakam/rustyline/blob/bc06b3eb322df1a535e6e3b64db8c0fee0b1df8c/src/history.rs#L465-L467)),
then one entry per line, with a line feed inside an entry written `\n` and a backslash `\\`; an
append adds lines without a header
([`history.rs:488-523`](https://github.com/kkawakam/rustyline/blob/bc06b3eb322df1a535e6e3b64db8c0fee0b1df8c/src/history.rs#L488-L523)).
On load, a first line other than `#V2` is a legacy file whose lines are taken as written, empty
lines are skipped, and a line with any other escape or a trailing backslash is kept unchanged with
a warning; a line that is not UTF-8 fails the whole load
([`history.rs:525-584`](https://github.com/kkawakam/rustyline/blob/bc06b3eb322df1a535e6e3b64db8c0fee0b1df8c/src/history.rs#L525-L584)).
A legacy file is rewritten with the header at the next save
([`history.rs:706-764`](https://github.com/kkawakam/rustyline/blob/bc06b3eb322df1a535e6e3b64db8c0fee0b1df8c/src/history.rs#L706-L764)).

#### Delegate records `.amazonq/.subagents`

`.amazonq/.subagents/<agent>.json` in the working directory
([`C/util/paths.rs:248-250`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/util/paths.rs#L248-L250),
[`C/cli/chat/tools/delegate.rs:577-590`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/tools/delegate.rs#L577-L590))
records the last background task run with the `delegate` tool for each agent: `{agent, task, status
(running|completed|failed), launched_at and completed_at (Unix seconds), pid, exit_code, output,
user_notified, summary, cwd}`
([`delegate.rs:248-291`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/tools/delegate.rs#L248-L291)),
written pretty-printed; `completed_at` is `null` while the task runs, a missing `launched_at`
reads as the Unix epoch, and a missing `cwd` as `Unknown`
([`:275-278`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/tools/delegate.rs#L275-L278),
[`:289-295`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/tools/delegate.rs#L289-L295)).
The default agent is named `q_cli_default`
([`C/cli/agent/mod.rs:74`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/agent/mod.rs#L74)).
The task runs as `q chat --non-interactive --agent <agent> <task>`, with `--trust-all-tools` added
for the default agent
([`:337-346`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/tools/delegate.rs#L337-L346)),
so its own conversation lands in `data.sqlite3` under the same directory key. Primary for the
delegated task's record; collected through the `.amazonq` project entry.

#### Shadow checkpoint repository

The shadow repository
`.aws/amazonq/cli-checkouts/<conversation_id>`
([`C/cli/chat/mod.rs:225-226`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/mod.rs#L225-L226))
is a bare git repository whose work tree is the user's directory, made only when checkpoints are on
(`chat.enableCheckpoint`, off unless set,
[`C/database/settings.rs:132`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/database/settings.rs#L132),
[`C/cli/experiment/experiment_manager.rs:127-137`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/experiment/experiment_manager.rs#L127-L137)),
and deleted when the checkpoint manager is dropped
([`C/cli/chat/checkpoint.rs:349-364`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/checkpoint.rs#L349-L364)).
It is the agent's scratch space, excluded by the catalog.

### Format versions

Migrations 000 to 007; `migrations.version` is the 0-based index, `migration_time` unix seconds
([`mod.rs:448-451`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/database/mod.rs#L448-L451)).
`has_migration` re-checks versions up to 7 because of an off-by-one in early builds
([`mod.rs:532-556`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/database/mod.rs#L532-L556)).
No schema version inside the JSON; `request_metadata` is `#[serde(default)]`
([`conversation.rs:95`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/conversation.rs#L95)),
`model` is kept only for 1.13.3 and earlier
([`conversation.rs:133-136`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/conversation.rs#L133-L136)),
and `model_info`, `file_line_tracker`, `mcp_enabled` and `tangent_state` are defaulted, so a parser
must treat them as optional. Legacy JSON exists only as `/save` exports. Paths still say `amazon-q`,
`.aws/amazonq` and `.amazonq` at this commit; expect the `crates/agent` camelCase schema and
per-session files if Kiro CLI stores `~/.kiro/sessions/`.

### Secrets

`auth_kv` keys `codewhisperer:odic:token`
([`C/auth/builder_id.rs:304`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/auth/builder_id.rs#L304))
and `codewhisperer:odic:device-registration`
([`builder_id.rs:119`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/auth/builder_id.rs#L119));
values are JSON with access and refresh tokens and a client secret. `state` key
`telemetry-cognito-credentials` holds `{access_key_id, secret_key, session_token, expiration}`
([`mod.rs:57`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/database/mod.rs#L57),[`78-84`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/database/mod.rs#L78-L84));
also `api.codewhisperer.profile`, `auth.idc.start-url`, `auth.idc.region`, `telemetryClientId`
([`mod.rs:57-61`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/database/mod.rs#L57-L61)).
Inside conversations, `tool_uses[].args` and tool result `content` hold command lines and output,
and `env_state.environment_variables` can carry environment values. The MCP OAuth token files are in
[Credentials](#credentials).

### Parser plan

Files read, by path relative to the home, or to the project for a project's files; the parser is
offered only files the catalog attributes to `kiro`:

- `.local/share/amazon-q/data.sqlite3`, `Library/Application Support/amazon-q/data.sqlite3` and
  `AppData/Local/amazon-q/data.sqlite3`, and the same three under `kiro-cli/`
  (`.local/share/kiro-cli/data.sqlite3`, `Library/Application Support/kiro-cli/data.sqlite3`,
  `AppData/Local/kiro-cli/data.sqlite3`), with their `-wal` and `-journal` sidecars: each row of
  `conversations` in `rowid` order, its `value` text or a BLOB read as UTF-8, then the legacy
  `history` table. A table name matches in any ASCII case.
- `.aws/amazonq/.cli_bash_history`, secondary.
- `.aws/amazonq/cli-agents/<name>.json` and a project's `.amazonq/cli-agents/<name>.json`.
- A project's `.amazonq/.subagents/<agent>.json`.
- `/save` exports: any other `.json` file outside `.kiro/sessions/` whose first 4,096 bytes, after
  leading ASCII white space, start with `{` and hold `"conversation_id"`. An export saved under a
  name without `.json`, which `/save` allows ([Transcript stores](#transcript-stores)), is not
  read.

Not read: `auth_kv`, `state`, `settings.json`, `transcript`, `.kiro/sessions/`, `.kiro/agents/`.

Each conversation gives, entry by entry through `history`: a model row when the model changes
([Model rows](#model-rows)); the user row; the deny-rule row of a rejected call
([Permission rows](#permission-rows)); one `tool_result` row per `tool_use_results[]` entry, each
followed by its permission row when the message is `CancelledToolUses`; the assistant row; one
`tool_use` row per `tool_uses[]` entry; and `compact: conversation history summarised` when the
entry's `message_meta_tags` hold `Compact`. The rows of the copies follow
([Past-use rows and secondary sources](#past-use-rows-and-secondary-sources)), then
`summary: <text>` for `latest_summary`, at the time and with the `model_id` of its
`RequestMetadata`, then `pending message (not sent): <text>` for a `next_message`, at its
`timestamp`, the text its `prompt` or else its `content` as compact JSON. The legacy `history`
table gives one row per row, `shell history: <command> (exit_code=<n> shell=<s> hostname=<h>
duration=<n>)`, each of the four only when not null or an empty string, at `start_time` (`time`
before migration 003), with its `session_id` and `cwd`.

| Column | Source |
| --- | --- |
| timestamp_utc | user and tool_result rows: `user.timestamp` in UTC, else `request_metadata.request_start_timestamp_ms`; assistant, tool_use and compaction rows: `stream_end_timestamp_ms`, else `request_start_timestamp_ms`; a row with neither takes the time of the conversation's row before it |
| session_id | `conversation_id` |
| project_path | the row `key`, else the first `user.env_context.env_state.current_working_directory` in `history` (an export has no key) |
| git_branch | empty |
| turn_type | user for the `prompt` of a `Prompt` or `CancelledToolUses`; assistant for the `content` of a `Response` or `ToolUse`; tool_use per `tool_uses[]` entry; tool_result per `tool_use_results[]` entry; system for the rest |
| model | the rows of a history entry: `request_metadata.model_id`, else `model_info.model_id`, else the legacy `model`; the summary row: the `model_id` of its `RequestMetadata`; empty on the pending message row and the row that heads a copy |
| tool_name | `tool_uses[].name`, else `orig_name`; a result row takes the name of the call with its `tool_use_id` |
| tool_use_id | `tool_uses[].id`; `tool_use_results[].tool_use_id` |
| text | the prompt; the assistant `content`; for a tool use its `args` as compact JSON in the record's key order, a string as written, after `orig_name=<orig_name> ` when that differs from the name; for a result a `content` string as written, or each `Text` block, each `Json` block as compact JSON and any other block as its text or compact JSON, joined by line feeds, then ` [<status>]` (`[<status>]` alone when there is no text), after `cancelled ` for a `CancelledToolUses` result |

The plan rests on the Amazon Q Developer CLI source; the Kiro CLI forms remain unverified.

#### Permission rows

Source-derived; not observed on any real install. A `CancelledToolUses` result stays a
`tool_result` whose text starts `cancelled` and is followed by a `system` row with the result's
`tool_use_id` and `tool_name`, chosen once per message by its `prompt`
([Permission records](#permission-records)):

| `prompt` | Row text |
| --- | --- |
| the fixed text of a bare `n` | `permission decision: denied by=n` |
| `Tool use with <name> was rejected ...` | `permission decision: denied by=deny-rule tool=<name>` |
| `The user interrupted the tool execution.` | `permission decision: cancelled by=interrupt` |
| any other text | `permission decision: denied by=user-text` |
| none, empty or not a string | `permission decision: cancelled` |

The rejection text in a `Prompt` message gives the deny-rule row after the user row, with
`tool_name` and no `tool_use_id`, since it names the tool, not the call. Each `cli-agents/*.json`
file, global or project, gives one row per `allowedTools` entry, `permission setting:
allowedTools=<entry> (from <file>, file mtime)`, and one per key of each `toolsSettings` tool,
`permission setting: toolsSettings.<tool>.<key>=<value> (from <file>, file mtime)`, or
`permission setting: toolsSettings.<tool>=<value> (from <file>, file mtime)` for a tool whose
settings are not an object, at the file's modification time with no session; `<file>` is the path
relative to the home or project, and a project file's rows carry the project as `project_path`.
An `allowedTools` that is not an array or a `toolsSettings` that is not an object gives no row and
no problem row. Approvals, `t` and `--trust-all-tools` leave nothing to read. Kiro CLI's
`settings/permissions.yaml`, `sandbox-state/` and `workspace-roots/<hash>/.trust-migration.json`
(named in [Per-user storage](#per-user-storage)) have no recorded shape and are not read.

#### Model rows

`system` rows ([issue 96](https://github.com/seanthegeek/doubleagent/issues/96)): `model: <id>`
before the rows of the first history entry that has a model and `model: <old> -> <new>` before the
rows of each entry whose model differs, at the entry's `user.timestamp`, else its
`request_start_timestamp_ms`, else its `stream_end_timestamp_ms`. The model is
`request_metadata.model_id`; an entry without one falls back to `model_info.model_id`, else the
legacy `model` string, and the row then ends ` source=model_info` or ` source=model`, since those
are conversation-level fields serialised with the whole state rather than per-request records.

The agent file's `model` and the `chat.defaultModel` key of `settings.json` choose the model of a
new conversation ([Record schema](#record-schema)), but the parser gives no row for either: from an
agent file it reads only `allowedTools` and `toolsSettings`, and it does not read `settings.json`.

#### Past-use rows and secondary sources

Source-derived; not observed on any real install. By role:

- `.aws/amazonq/.cli_bash_history`, secondary: one `user` row per entry, unescaped as the
  [file format](#prompt-history-cli_bash_history) says when the first line is `#V2` and taken as
  written otherwise, with no timestamp, session or project, so it matches the conversation on text
  alone.
- `checkpoint_manager.checkpoints[].history_snapshot` and `tangent_state.main_history`, secondary
  copies inside the same blob: the parser pairs them itself, since the analyzer's deduplication
  pairs only rows of different files. An entry equal to one the parser already yielded on its
  user `content`, user `timestamp` and `assistant` message is skipped; the others are parsed like
  `history` after its rows and marked `recovered` `yes` (their `system` rows excepted), behind one
  `system` row per copy that contributes any: `checkpoint snapshot: tag=<tag>[ tool=<tool_name>]
  recovered=<n> entries[: <description>]` at the checkpoint's `timestamp`, or `tangent mode: main
  conversation held aside, recovered=<n> entries`, which takes the previous row's time, since
  `tangent_start_time` is not read. One model sequence runs over all the copies, apart from the
  history's.
- `.amazonq/.subagents/<agent>.json`, primary: `delegate task launched: agent=<agent> pid=<pid>[
  trust_all_tools=true]: <task>` at `launched_at` (`trust_all_tools` for `q_cli_default`, which
  the delegate tool runs with `--trust-all-tools`) and, once the task ended (`completed_at` set, or
  a `status` present and not `running`), `delegate task <status>: agent=<agent> exit_code=<code>`,
  the status `ended` when absent, with `summary:` and `output:` lines at `completed_at`. A time
  that is not a positive number is left empty, so a `launched_at` the CLI defaulted to the epoch
  gives no time. `project_path` is `cwd`, or the project when it is empty or `Unknown`.
- `transcript`, secondary with no timestamp, is not read.

#### Sync, relay and origin rows

The agent records none in the open-source CLI. Each request sends the conversation to the service
under its `conversation_id`
([`C/api_client/mod.rs:391-400`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/api_client/mod.rs#L391-L400)),
and nothing uploads or marks a session as stored remotely (not found in `crates/chat-cli/src` at
[`15cc8f3`](https://github.com/aws/amazon-q-developer-cli/commit/15cc8f3cd18c4272925ce1c7053268eedff1ea0a),
grepped for `share_conversation`, `sync_session`, `upload_conversation` and
`remote_session`).

#### Scratch directory rows

`.aws/amazonq/cli-checkouts/<conversation_id>`
([Shadow checkpoint repository](#shadow-checkpoint-repository)) is never a project and is never
recorded as one; the row key is always the user's working directory.

#### Deletion rows

The parser gives none. See [What survives a delete](#what-survives-a-delete): `/clear` and
compaction show as a `history` shorter than `transcript`, which the parser does not read, and a
replaced row survives only in `.cli_bash_history`.

#### Bad-line rows

Source-derived; not observed on any real install. `.cli_bash_history` is the one line-oriented
file: a line that is not UTF-8 becomes `parser: unparseable line (not valid UTF-8): <line>`, after
every entry's row. A line with an escape rustyline does not write is kept as written, as rustyline
keeps it. An agent configuration or delegate record that is not a JSON object is one row at the
file's modification time, `parser: agent configuration is not a JSON object` or `parser: delegate
record is not a JSON object`.

A conversation value that does not parse as JSON is salvaged when it holds `"history": [`: the
`conversation_id` before it and every complete history entry before the damage give their rows,
followed by `parser: truncated conversation, <n> history entries recovered: <reason>`. One with
nothing to salvage, or a JSON value that is not an object, is one row
`parser: unparseable conversation: <reason>`, the reason `not a JSON object` for the latter. A
`history` that is present but not an array is `parser: history is not a JSON array`, and the
conversation then gives no other row; an absent or null `history` gives no row at all.

Another conversation member that is present but of the wrong kind is
`parser: <member> is not a JSON array` (or `object`) with the conversation's `source_line`, after
its rows: `checkpoint_manager.checkpoints`, a checkpoint's `history_snapshot`,
`tangent_state.main_history`, `request_metadata.message_meta_tags`, `tool_uses` and
`tool_use_results` as arrays; `checkpoint_manager`, `tangent_state`, `request_metadata`, `user`,
`user.content` and `assistant` as objects, and the body of a `user.content` or `assistant` variant
as an object (`parser: user.content.<variant> is not a JSON object`,
`parser: assistant.<variant> is not a JSON object`). An absent, null or empty one gives no row.
One of these arrays or `history`, other than `message_meta_tags`, holding entries that are not
objects gives one row for them all at the same place,
`parser: <member> holds <n> entries that are not JSON objects`
(`1 entry that is not a JSON object` for one), the other entries giving their rows. A member that
every history entry or checkpoint holds is reported once for the conversation however many entries
get it wrong: one wrong entry gives the rows above, more give
`parser: <member> is not a JSON object in <n> entries` (or `array`), and the entries of the wrong
kind its arrays hold are counted together over every entry, in one `holds <n> entries` row.
A `model_info`, legacy `model`, `latest_summary` or `next_message` of the wrong kind is passed
over, and a delegate record's field of the wrong kind is written as its JSON text or left empty,
with no problem row.

A `.cli_bash_history`, `/save` export, agent configuration or delegate record that cannot be read
is one row `parser: <file> could not be read (<reason>)`, `<file>` its name. Selection reads a
`.json` file's first bytes to recognise an export; a `.json` file it cannot read is offered as an
export, so it gives that one row too.

A `data.sqlite3` that cannot be read, or whose `-wal` or `-journal` sidecar cannot, is one row
`parser: <file> could not be read (<reason>)`, `<file>` the name of the file that failed; one that
is not a SQLite database is `parser: not a SQLite database`. One that starts with the SQLite header
but cannot be read past it (a truncated copy, a damaged page) is one row
`parser: SQLite error: <message>`, `<message>` SQLite's own, in place of every row of that
database; the other files are still parsed.

#### Fixtures

Fixture, a `data.sqlite3` with one conversation row:

```sql
CREATE TABLE conversations (key TEXT PRIMARY KEY, value TEXT);
INSERT INTO conversations (key, value) VALUES ('/home/alice/proj', '{"conversation_id":"3f2b8c1e-9d7a-4e6b-b1c2-0a9f8e7d6c5b","next_message":null,"history":[{"user":{"additional_context":"","env_context":{"env_state":{"operating_system":"linux","current_working_directory":"/home/alice/proj","environment_variables":[]}},"content":{"Prompt":{"prompt":"list the files here"}},"timestamp":"2026-04-23T14:17:15.123456789-07:00","images":null},"assistant":{"ToolUse":{"message_id":"msg-001","content":"I will list the directory.","tool_uses":[{"id":"tooluse_abc123","name":"execute_bash","orig_name":"execute_bash","args":{"command":"ls -la"},"orig_args":{"command":"ls -la"}}]}},"request_metadata":{"request_id":"req-001","message_id":"msg-001","request_start_timestamp_ms":1777000635200,"stream_end_timestamp_ms":1777000636900,"time_to_first_chunk":{"secs":0,"nanos":400000000},"time_between_chunks":[],"user_prompt_length":19,"response_size":26,"chat_conversation_type":"ToolUse","tool_use_ids_and_names":[["tooluse_abc123","execute_bash"]],"model_id":"claude-sonnet-4","message_meta_tags":[]}},{"user":{"additional_context":"","env_context":{"env_state":{"operating_system":"linux","current_working_directory":"/home/alice/proj","environment_variables":[]}},"content":{"ToolUseResults":{"tool_use_results":[{"tool_use_id":"tooluse_abc123","content":[{"Text":"total 8\nREADME.md"}],"status":"Success"}]}},"timestamp":null,"images":null},"assistant":{"Response":{"message_id":"msg-002","content":"The directory contains README.md."}},"request_metadata":{"request_id":"req-002","message_id":"msg-002","request_start_timestamp_ms":1777000637000,"stream_end_timestamp_ms":1777000638100,"time_to_first_chunk":null,"time_between_chunks":[],"user_prompt_length":0,"response_size":34,"chat_conversation_type":"NotToolUse","tool_use_ids_and_names":[],"model_id":"claude-sonnet-4","message_meta_tags":[]}}],"valid_history_range":[0,2],"transcript":["> list the files here","The directory contains README.md."],"tools":{"native___":[]},"context_manager":null,"context_message_length":null,"latest_summary":null,"model_info":{"model_id":"claude-sonnet-4","model_name":"Claude Sonnet 4"},"file_line_tracker":{},"checkpoint_manager":null,"mcp_enabled":true}');
CREATE TABLE auth_kv (key TEXT PRIMARY KEY, value TEXT);
INSERT INTO auth_kv VALUES ('codewhisperer:odic:token', '{"access_token":"REDACT-ME"}');
```

A `CancelledToolUses` user message after a bare `n`, from the constructor at
[`C/cli/chat/message.rs:120-140`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/message.rs#L120-L140)
(`env_context` shortened):

```json
{"additional_context":"","env_context":{},"content":{"CancelledToolUses":{"prompt":"I deny this tool request. Ask a follow up question clarifying the expected action","tool_use_results":[{"tool_use_id":"tooluse_d1","content":[{"Text":"Tool use was cancelled by the user"}],"status":"Error"}]}},"timestamp":"2026-04-25T09:00:05-07:00","images":null}
```

A checkpoint, from
[`C/cli/chat/checkpoint.rs:74-82`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/checkpoint.rs#L74-L82)
(`history_snapshot` holds `HistoryEntry` objects):

```json
{"tag":"1","timestamp":"2026-04-25T08:59:30-07:00","description":"fix the build","history_snapshot":[],"is_turn":true,"tool_name":null}
```

A global agent configuration `.aws/amazonq/cli-agents/dev.json`:

```json
{"name":"dev","allowedTools":["fs_read","@git"],"toolsSettings":{"execute_bash":{"allowedCommands":["git status"],"deniedCommands":["rm -rf .*"],"autoAllowReadonly":true},"fs_write":{"deniedPaths":["/etc"]}},"model":"claude-sonnet-4"}
```

A delegate record `.amazonq/.subagents/q_cli_default.json`:

```json
{"agent":"q_cli_default","task":"run the tests","status":"completed","launched_at":1777132900,"completed_at":1777132960,"pid":4711,"exit_code":0,"output":"12 passed","user_notified":true,"summary":"All tests pass.","cwd":"/srv/proj"}
```

A `.cli_bash_history` in the `#V2` format, an entry with a line feed on its fourth line:

```text
#V2
list the files here
/clear
fix the\nbuild
```

#### Source host rows

The agent records no source-host integration. `crates/chat-cli/src` and `crates/agent/src` at
[15cc8f3](https://github.com/aws/amazon-q-developer-cli/commit/15cc8f3cd18c4272925ce1c7053268eedff1ea0a),
grepped for `github`, `gitlab`, `bitbucket`, `oauth`, `installation`, `repositor`,
`pull request` and `clone`, name GitHub only in the new-issue URL of the CLI's own repository that
`q issue` builds
([`C/cli/chat/util/issue.rs:74`](https://github.com/aws/amazon-q-developer-cli/blob/15cc8f3cd18c4272925ce1c7053268eedff1ea0a/crates/chat-cli/src/cli/chat/util/issue.rs#L74)),
in release notes and in an MCP test; the OAuth flows are the Builder ID and IAM
Identity Center sign-in ([Secrets](#secrets)). Kiro CLI and the Kiro IDE are closed source and
not reviewed for it.

## Confidence

- High: `~/.kiro` subdirectories, `KIRO_HOME`, legacy `~/.aws/amazonq` and `~/.aws/sso/cache`,
  `auth_kv` and `conversations` tables, `amazon-q` data dir, `$TMPDIR/kiro-log`, project `.kiro`
  files, Kiro IDE app dirs and `kiro.kiroagent`; the Amazon Q store's path, tables, migrations,
  key = cwd, JSON field names and enum tags, chrono RFC 3339 and unix-ms timestamp formats.
- Medium: the Kiro CLI paths `kiro-cli` as the current data-dir name and the
  `sessions/<hash>/sess_*` and `session-index` layouts (L3/L4, binary confirms `sess_<uuid>`
  only), LanceDB directory name, `.kiro-server`; the `dirs` platform mapping (crate docs, not
  read); `tangent_start_time` serialisation.
- Low: `~/.kiro/secrets.json`, `kiro-auth-token.json`.
- Unverified: the record shapes of everything specific to Kiro CLI on disk (the `kiro-cli`
  directories, `.kiro/sessions`, `workspace-roots`), WAL use, the camelCase `agent` schema on
  disk, token key names after the rebrand.
- Not determined: whether the v3 CLI still writes `data.sqlite3` or only `~/.kiro/sessions`
  ("SQLite database in `~/.kiro/`" per docs vs `data.sqlite3` string in the binary), the hash
  function for `workspace-roots` and `sessions` buckets, Windows `KIRO_HOME` default, and the
  `.chat` file schema.
