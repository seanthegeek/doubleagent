# letta

Letta (formerly MemGPT) has two codebases that matter on a host. Letta Code, the current `letta`
CLI, keeps its state in `~/.letta` and its conversations on the Letta server by default, but
writes two parseable JSONL stores under `~/.letta` and, in local-backend mode, the full message
history in [pi](pi.md)'s session format into its own directory, which the pi parser's reader
reads. The retired Letta V1 Python server (and MemGPT before it) kept messages in SQLite. Catalog
agent: `letta`. Parsed: the letta parser reads the reflection transcripts, the local-backend
`messages.jsonl` files, `sessions.jsonl`, the `sessionsByServer` and `permissions` settings, the
channel routing, the scheduled tasks and their run logs, and the interrupted listener turns; the
V1 `sqlite.db`, the input ledger and the chunk logs are not parsed.

## Source and evidence

Both codebases are Apache-2.0 and open source:

- **Letta Code**, the current product (`npm install -g @letta-ai/letta-code`, command `letta`):
  [letta-ai/letta-code](https://github.com/letta-ai/letta-code) at
  [`77faf36e34378946c8e69c0bfaa5595a7be4c326`](https://github.com/letta-ai/letta-code/commit/77faf36e34378946c8e69c0bfaa5595a7be4c326).
  TypeScript run under Bun or Node.
- **The retired Letta V1 Python server** (`pip install letta`, earlier `pymemgpt`).
  [letta-ai/letta](https://github.com/letta-ai/letta) `main` at
  [`5bcdd177d70fa2b31a754cfcd801e77b2e1ab16a`](https://github.com/letta-ai/letta/commit/5bcdd177d70fa2b31a754cfcd801e77b2e1ab16a)
  is now only a landing page that points to letta-code
  ([README.md:5](https://github.com/letta-ai/letta/blob/5bcdd177d70fa2b31a754cfcd801e77b2e1ab16a/README.md#L5)).
  The V1 source was read on the `archive` branch at
  [`56ba9c25552605eec89de8ed3dc6394b625c1993`](https://github.com/letta-ai/letta/commit/56ba9c25552605eec89de8ed3dc6394b625c1993)
  and at tags 0.11.7
  ([`bf9356ce`](https://github.com/letta-ai/letta/commit/bf9356ceaf8ac4c80f1b418f209ad49b04986885)),
  0.5.5
  ([`1eb20530`](https://github.com/letta-ai/letta/commit/1eb20530d3bcacedf0de3977573ee364d6432848)),
  0.4.1
  ([`e0442bd6`](https://github.com/letta-ai/letta/commit/e0442bd6585f4f17fdb9dca207d2ce17ec9e6e92))
  and 0.3.25, the last MemGPT-named release
  ([`0f6d9fa8`](https://github.com/letta-ai/letta/commit/0f6d9fa8494b6259246e67a4d86202c94e5391ec));
  the V1 SQLite schema is from tag 0.11.7 at
  [`bf9356ceaf8ac4c80f1b418f209ad49b04986885`](https://github.com/letta-ai/letta/commit/bf9356ceaf8ac4c80f1b418f209ad49b04986885).

The message types used by the Letta Code local backend come from `@earendil-works/pi-ai`
([package.json:151](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/package.json#L151)),
read in [earendil-works/pi](https://github.com/earendil-works/pi) at
[`200387122ca450d6387f033949423114a270b96c`](https://github.com/earendil-works/pi/commit/200387122ca450d6387f033949423114a270b96c)
(letta-code pins `^0.99.1`; the pi checkout is newer, so optional fields may differ). All claims are
from source.

### Observed

- No real install. No real install has shown the files behind the `session synced:` rows, so
  those rows are fixture only ([Sync, relay and origin](#sync-relay-and-origin-rows)).

### Source reviews

- 2026-10-21: letta-ai/letta-code at
  [`2a46615`](https://github.com/letta-ai/letta-code/commit/2a46615d16babb06cf5450b8e57ecb9bdbf2f436)
  (0.34.17), with the Letta client SDK letta-ai/letta-node at
  [`fd801c6`](https://github.com/letta-ai/letta-node/commit/fd801c6b44b3ab2c8f5afaa577b7101ee7520f3a)
  (v1.10.2, the version letta-code pins): corrected [sessions.jsonl](#sessionsjsonl) to exit lines
  only, added the shape of an interrupted turn's `results`, the scheduled-task prompt wrapper and
  the legacy `routing.yaml`, and moved the permission settings, channel routing, scheduled tasks,
  run logs and interrupted turns from not read to parsed in the [Parser plan](#parser-plan); added
  [Source host rows](#source-host-rows).
- 2026-10-19: [letta-ai/letta-code](https://github.com/letta-ai/letta-code) at [`2a46615`](https://github.com/letta-ai/letta-code/commit/2a46615d16babb06cf5450b8e57ecb9bdbf2f436), the head
  of `main` (0.34.17,
  [package.json:3](https://github.com/letta-ai/letta-code/blob/2a46615d16babb06cf5450b8e57ecb9bdbf2f436/package.json#L3)),
  compared file by file with [`77faf36`](https://github.com/letta-ai/letta-code/commit/77faf36e34378946c8e69c0bfaa5595a7be4c326) (0.34.2). The review added the permission settings, the
  scheduled-task run logs, the input ledger, channel routing and the OpenCode external agent, and
  recorded in [Format versions](#format-versions) that the shapes the parser reads are unchanged.

## Collection

### Per-user storage

#### Letta Code

Letta Code resolves everything from `os.homedir()` plus `.letta`, on every OS: `~/.letta` on Linux
and macOS, `%USERPROFILE%\.letta` on Windows
([settings-manager.ts:976-980](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/settings-manager.ts#L976-L980)).
Some modules prefer `$HOME` over `os.homedir()`
([cron-file.ts:136-139](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/cron/cron-file.ts#L136-L139)).
A legacy settings path is `$XDG_CONFIG_HOME/letta/settings.json`, falling back to
`~/.config/letta/settings.json` on every OS, Windows included
([permissions/loader.ts:48-62](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/permissions/loader.ts#L48-L62)).

Under `~/.letta`:

- `settings.json`: global settings, including `env`, `sessionsByServer`, per-agent `agents`,
  `preferredBackendMode` and the deprecated `refreshToken`
  ([settings-manager.ts:64-111](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/settings-manager.ts#L64-L111)).
- `sessions.jsonl`: one line per CLI session that ended with `/exit` or `/logout`, session
  metadata ([sessions.jsonl](#sessionsjsonl)).
- `transcripts/<agent>/<conversation>/transcript.jsonl` and `state.json`: a flattened transcript
  appended after every completed turn, kept for memory "reflection"
  ([utils/transcript-paths.ts:15-22](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/utils/transcript-paths.ts#L15-L22),
  [reflection-transcript.ts:911-925](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/cli/helpers/reflection-transcript.ts#L911-L925),
  [use-conversation-loop.ts:1443-1452](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/cli/app/use-conversation-loop.ts#L1443-L1452));
  `transcripts/background/` holds subagent scratch
  ([subagent-launcher.ts:187-194](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/agent/subagents/subagent-launcher.ts#L187-L194)).
  Root overridable with `LETTA_TRANSCRIPT_ROOT`.
- `agents/<agent-id>/memory/`: a git clone of the agent's memory filesystem (MemFS), plus
  `memory-worktrees/`
  ([memory-git.ts:87-96](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/agent/memory-git.ts#L87-L96),
  [sandbox-policy.ts:135-160](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/permissions/sandbox-policy.ts#L135-L160)).
  Agent skills live in `memory/skills`
  ([skills.ts:195-206](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/agent/skills.ts#L195-L206)).
- `lc-local-backend/` (or `$LETTA_LOCAL_BACKEND_DIR`) for the local backend, used when
  `--backend local`, `LETTA_LOCAL_BACKEND_EXPERIMENTAL=1` or `preferredBackendMode: "local"` is set
  ([local-backend-paths.ts:8-26](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/utils/local-backend-paths.ts#L8-L26),
  [index.ts:851-870](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/index.ts#L851-L870)):
  `agents/<b64url id>.json`,
  `conversations/<b64url key>/{conversation.json,manifest.json,messages.jsonl,system-prompt.json}`,
  `memfs/<agent>/memory/`, `providers/auth.json`
  ([local-store.ts:515](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/backend/local/local-store.ts#L515),
  [2566-2574](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/backend/local/local-store.ts#L2566-L2574),
  [local-transcript.ts:82-88](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/backend/local/local-transcript.ts#L82-L88),
  [backend/local/paths.ts:48-53](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/backend/local/paths.ts#L48-L53)).
  Migration backups are `messages.jsonl.pre-pi-backup-*`
  ([transcript-migration.ts:498](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/backend/local/transcript-migration.ts#L498)).
- `projects/<sanitised cwd>/agent-tools/`: spilled oversized tool output per working directory
  ([overflow.ts:31-50](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/tools/impl/overflow.ts#L31-L50)).
- `logs/debug/` (always on, five sessions of 10 MB)
  ([debug.ts:68-70](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/utils/debug.ts#L68-L70)),
  `logs/chunk-logs/<agent>/<session>.jsonl`, truncated stream chunks ([Chunk logs](#chunk-logs)),
  `logs/remote/`
  ([listen-log.ts:24](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/websocket/listen-log.ts#L24)).
- `channels/<channel>/{config.yaml,accounts.json,routing.json,pairing.yaml,targets.json}` for Slack,
  Telegram, Discord, Signal and WhatsApp bridges
  ([channels/config.ts:35-80](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/channels/config.ts#L35-L80)),
  `routing.json` naming the chat each conversation is relayed through
  ([Channel routing](#channel-routing));
  `channels/whatsapp/auth/<account>/`
  ([whatsapp/session.ts:67-69](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/channels/whatsapp/session.ts#L67-L69)).
- `crons.json` scheduled tasks, with `runs/<task id>.jsonl` beside it, one line per run
  ([Scheduled tasks](#scheduled-tasks))
  ([cron-file.ts:121](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/cron/cron-file.ts#L121),
  [141-143](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/cron/cron-file.ts#L141-L143)),
  `workflows/executions/`
  ([journal.ts:39-41](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/tools/workflow/journal.ts#L39-L41)),
  `listener-state/<hash>/` interrupted turns
  ([interrupted-turn-record.ts:38-50](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/websocket/listener/interrupted-turn-record.ts#L38-L50);
  [Interrupted turns](#interrupted-turns)),
  `state/<24 hex>/input-dispositions-v2.json`, the remote listener's ledger of accepted inputs,
  new since [`77faf36`](https://github.com/letta-ai/letta-code/commit/77faf36e34378946c8e69c0bfaa5595a7be4c326) ([Input ledger](#input-ledger)),
  `artifacts/`
  ([artifact-files.ts:44-48](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/tools/impl/artifact-files.ts#L44-L48)),
  `viewers/`
  ([generate-memory-viewer.ts:41](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/web/generate-memory-viewer.ts#L41)),
  `skills/`
  ([skills.ts:191-194](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/agent/skills.ts#L191-L194)),
  `mods/`, `extensions/`, `mod-cache/`
  ([mods/paths.ts:8-16](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/mods/paths.ts#L8-L16),
  [36-37](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/mods/paths.ts#L36-L37)),
  `cache/model-catalog.json`
  ([remote-model-catalog.ts:60-66](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/agent/remote-model-catalog.ts#L60-L66)),
  `bin/` for downloaded ripgrep and fd
  ([ripgrep-manager.ts:71-75](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/tools/impl/ripgrep-manager.ts#L71-L75)).

With the default API backend, agents and messages live on the server (Letta Cloud or a self-hosted
server); the local evidence is `transcripts/`, `sessions.jsonl`, the logs and the memory clones.

#### Scratch directories

Letta Code works in the user's cwd. Its own working copies are the memory clones
`agents/<agent id>/memory/` and their `memory-worktrees/`, inside `~/.letta`, so the catalog
collects them and the analyzer classes a path there as a scratch directory, as it does
`projects/<sanitised cwd>/agent-tools/`. Its external coding agents (Claude Code, Codex and, since
[`77faf36`](https://github.com/letta-ai/letta-code/commit/77faf36e34378946c8e69c0bfaa5595a7be4c326), OpenCode, started as `opencode serve` in the working directory;
[src/agent/subagents/index.ts:123-159](https://github.com/letta-ai/letta-code/blob/2a46615d16babb06cf5450b8e57ecb9bdbf2f436/src/agent/subagents/index.ts#L123-L159),
[src/tools/impl/opencode-server.ts:51-66](https://github.com/letta-ai/letta-code/blob/2a46615d16babb06cf5450b8e57ecb9bdbf2f436/src/tools/impl/opencode-server.ts#L51-L66))
run in the same cwd and write to their own directories, which their own catalog lines collect.

#### Letta V1 server

The Letta V1 server used `~/.letta` too (`LETTA_DIR`,
[archive constants.py:5-6](https://github.com/letta-ai/letta/blob/56ba9c25552605eec89de8ed3dc6394b625c1993/letta/constants.py#L5-L6)):
`config`
([config.py:42](https://github.com/letta-ai/letta/blob/56ba9c25552605eec89de8ed3dc6394b625c1993/letta/config.py#L42)),
`logs/Letta.log`
([log.py:188](https://github.com/letta-ai/letta/blob/56ba9c25552605eec89de8ed3dc6394b625c1993/letta/log.py#L188)),
`tool_execution_dir/`, and `pg_uri` for Letta Desktop
([settings.py:258-269](https://github.com/letta-ai/letta/blob/56ba9c25552605eec89de8ed3dc6394b625c1993/letta/settings.py#L258-L269)).
Up to at least 0.11.7 the default database was `~/.letta/sqlite.db`
([0.11.7 db.py:149](https://github.com/letta-ai/letta/blob/bf9356ceaf8ac4c80f1b418f209ad49b04986885/letta/server/db.py#L149),
[194](https://github.com/letta-ai/letta/blob/bf9356ceaf8ac4c80f1b418f209ad49b04986885/letta/server/db.py#L194));
at the archive head the engine is always Postgres, defaulting to `localhost:5432`
([settings.py:472-478](https://github.com/letta-ai/letta/blob/56ba9c25552605eec89de8ed3dc6394b625c1993/letta/settings.py#L472-L478),
[db.py:21](https://github.com/letta-ai/letta/blob/56ba9c25552605eec89de8ed3dc6394b625c1993/letta/server/db.py#L21)).
0.4–0.5 kept `~/.letta/credentials`
([0.4.1 credentials.py:15](https://github.com/letta-ai/letta/blob/e0442bd6585f4f17fdb9dca207d2ce17ec9e6e92/letta/credentials.py#L15))
and `~/.letta/chroma`
([0.4.1 config.py:67-68](https://github.com/letta-ai/letta/blob/e0442bd6585f4f17fdb9dca207d2ce17ec9e6e92/letta/config.py#L67-L68)).

#### MemGPT

MemGPT (≤0.3.x) used `~/.memgpt`
([0.3.25 constants.py:4](https://github.com/letta-ai/letta/blob/0f6d9fa8494b6259246e67a4d86202c94e5391ec/memgpt/constants.py#L4))
with `credentials`, `config`, `sqlite.db` and `chroma/`
([credentials.py:15](https://github.com/letta-ai/letta/blob/0f6d9fa8494b6259246e67a4d86202c94e5391ec/memgpt/credentials.py#L15),
[config.py:61-72](https://github.com/letta-ai/letta/blob/0f6d9fa8494b6259246e67a4d86202c94e5391ec/memgpt/config.py#L61-L72),
[metadata.py:342-343](https://github.com/letta-ai/letta/blob/0f6d9fa8494b6259246e67a4d86202c94e5391ec/memgpt/metadata.py#L342-L343)).
No Letta release reads `~/.memgpt`; a grep of 0.4.1, 0.11.7 and the archive head finds no reference.

What a delete removes and leaves is in
[What survives a delete](#what-survives-a-delete).

#### Docker volume

A Docker or Podman volume whose name matches `*letta*` is collected with its `_data` directory as
the base. The volume name is inferred; a volume mounted in place of `~/.letta` would hold the
layout of [Letta Code](#letta-code) and [Letta V1 server](#letta-v1-server) at its base.
The volume-relative exclusions are `bin` and `mod-cache` (Letta Code), and `tool_execution_dir`
and `chroma` (Letta V1). The volume-relative secret globs are `pg_uri`, `credentials`,
`lc-local-backend/providers/auth.json`, `channels/*/accounts.json`,
`channels/whatsapp/auth/*`, `agents/*/memory/.git/config` and
`agents/*/memory/.git/letta-credential-helper.cmd`, and the shared `settings.json` glob applies
([Credentials](#credentials)). The layout inside the volume is source-derived; not observed on any
real install.

### Credentials

- Letta Code stores the Letta API key and refresh token in the OS keychain through `Bun.secrets`,
  service `letta-code`, names `letta-api-key` and `letta-refresh-token`
  ([secrets.ts:1-18](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/utils/secrets.ts#L1-L18)).
  When the keychain is unavailable (headless Linux, Node runtime) it writes `refreshToken` and
  `env.LETTA_API_KEY` into `~/.letta/settings.json`
  ([settings-manager.ts:580-592](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/settings-manager.ts#L580-L592),
  [733-750](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/settings-manager.ts#L733-L750)).
  `oauthState` holds a PKCE verifier
  ([settings-manager.ts:104-110](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/settings-manager.ts#L104-L110)).
- `~/.letta/agents/*/memory/.git/config` holds the Letta API key in an inline credential helper on
  Unix; on Windows it is in `.git/letta-credential-helper.cmd`
  ([memory-git.ts:713-721](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/agent/memory-git.ts#L713-L721),
  [memory-git-windows-credentials.ts:15-26](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/agent/memory-git-windows-credentials.ts#L15-L26)).
- `~/.letta/lc-local-backend/providers/auth.json`, mode 0600: provider API keys and OAuth tokens for
  the local backend
  ([local-provider-auth-store.ts:91](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/backend/local/local-provider-auth-store.ts#L91),
  [116-120](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/backend/local/local-provider-auth-store.ts#L116-L120),
  [218](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/backend/local/local-provider-auth-store.ts#L218)).
- `~/.letta/channels/*/accounts.json`: bot tokens inline unless `channelCredentialsStore` resolves
  to `keyring`
  ([accounts.ts:195-217](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/channels/accounts.ts#L195-L217),
  [657-663](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/channels/accounts.ts#L657-L663),
  [credential-store.ts:62-80](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/channels/credential-store.ts#L62-L80)).
  `channels/whatsapp/auth/` is a WhatsApp Web session.
- Letta Code reads `~/.aws/credentials` and `~/.aws/config` for Bedrock
  ([aws-credentials.ts:20-21](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/utils/aws-credentials.ts#L20-L21)).
- V1: `~/.letta/pg_uri` (Postgres URI with password), `~/.letta/credentials` (0.4–0.5) and
  `~/.memgpt/credentials` (provider keys,
  [0.3.25 credentials.py:15-31](https://github.com/letta-ai/letta/blob/0f6d9fa8494b6259246e67a4d86202c94e5391ec/memgpt/credentials.py#L15-L31)).
  `sqlite.db` holds `providers.api_key` next to the messages
  ([0.11.7 orm/provider.py:30-32](https://github.com/letta-ai/letta/blob/bf9356ceaf8ac4c80f1b418f209ad49b04986885/letta/orm/provider.py#L30-L32));
  collect it unflagged, as with Cursor.

### Exclusions

`.letta/bin` (ripgrep and fd binaries), `.letta/mod-cache`. V1: `.letta/tool_execution_dir` (tool
sandboxes and virtualenvs), `.letta/chroma`, `.memgpt/chroma` (embedding stores). Keep
`agents/*/memory/.git`: it carries the memory edit history and the credential helper.

### Project-local files

`<project>/.letta/settings.json` (checked-in hooks and window title) and
`.letta/settings.local.json` (permissions, `sessionsByServer`, last agent)
([settings-manager.ts:982-997](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/settings-manager.ts#L982-L997),
[143-158](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/settings-manager.ts#L143-L158),
[permissions/loader.ts:70-71](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/permissions/loader.ts#L70-L71)).
Project skills are read from `.agents/skills` and the legacy `.skills`
([skills.ts:181-186](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/agent/skills.ts#L181-L186)).
No per-project database.

Both project files, like the user files `~/.letta/settings.json` and
`~/.config/letta/settings.json`, can hold a `permissions` object with the standing rules
([Permission settings](#permission-settings)).

### Where the project path is recorded

- `~/.letta/sessions.jsonl` key `project`, which is `process.cwd()`
  ([AppCoordinator.tsx:403](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/cli/app/AppCoordinator.tsx#L403),
  [3841-3851](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/cli/app/AppCoordinator.tsx#L3841-L3851)).
  Grep-able: `"project":"<path>"`.
- `~/.letta/lc-local-backend/conversations/*/messages.jsonl` first line, key `cwd`
  ([local-transcript.ts:236-246](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/backend/local/local-transcript.ts#L236-L246)).
- `~/.letta/projects/<dir>/`: the cwd with the leading separator dropped and `/`, `\`, `:` and
  whitespace turned into `_` (lossy).
- `listener-state/*/*.json` `workingDirectory` names a working directory
  ([Interrupted turns](#interrupted-turns)), and `additionalDirectories` in a `permissions` object
  names directories the user let the agent reach ([Permission settings](#permission-settings)).

## Parsing

### Transcript stores

| Glob (relative to home) | One file per | Mode |
| --- | --- | --- |
| `.letta/transcripts/*/*/transcript.jsonl` | agent and conversation | every turn, both backends |
| `.letta/lc-local-backend/conversations/*/messages.jsonl` | conversation | local backend only |
| `.letta/sessions.jsonl` | user (one line per session ended with `/exit` or `/logout`) | always |
| `.letta/logs/chunk-logs/*/*.jsonl` | agent and CLI session, last 5 kept | always, truncated |
| `.letta/sqlite.db` | user | V1 server up to at least 0.11.7 |

`transcripts/<agent>/<conversation>/` names are the ids with anything outside `[a-zA-Z0-9._-]`
replaced by `_`
([reflection-transcript.ts:578-581](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/cli/helpers/reflection-transcript.ts#L578-L581),
[911-925](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/cli/helpers/reflection-transcript.ts#L911-L925));
`state.json` and `payload-*.json` sit beside it. Local-backend conversation directories are
base64url of `conversation:<id>`, or `default:<agent id>` for an agent's default conversation
([local-store.ts:283-285](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/backend/local/local-store.ts#L283-L285),
[3262-3266](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/backend/local/local-store.ts#L3262-L3266)),
next to `conversation.json`, `manifest.json` and `system-prompt.json`.

#### Server-side session markers

In the default backend, `api` (Letta Cloud at `https://api.letta.com`, or a self-hosted Letta server
named by `LETTA_BASE_URL`), the agent and its conversations are server objects and the local files
name them by id
([settings-manager.ts:75](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/settings-manager.ts#L75),
[195](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/settings-manager.ts#L195),
[273-281](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/settings-manager.ts#L273-L281);
[cli/subcommands/backend.ts:38](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/cli/subcommands/backend.ts#L38)).
A local-backend agent id starts `agent-local-`; any other agent id is a server id
([agent/agent-id.ts:3-9](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/agent/agent-id.ts#L3-L9)).

- `.letta/transcripts/<agent id>/<conversation id>/`: the two directory
  names are the server's ids, sanitised as above.
- `.letta/sessions.jsonl` `agent_id`: the server agent id; its `session_id` is the CLI's own
  telemetry session id, not a conversation
  ([cli/app/AppCoordinator.tsx:3841-3844](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/cli/app/AppCoordinator.tsx#L3841-L3844)).
- `.letta/settings.json` `sessionsByServer`: an object keyed by the server's host and port
  (`api.letta.com`, `localhost:8283`, or `local:<store path>` for the local backend) whose values
  are `{agentId, conversationId}`, the last session used on that server; `lastSession` and
  `lastAgent` repeat it for server agents
  ([settings-manager.ts:53-56](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/settings-manager.ts#L53-L56),
  [94](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/settings-manager.ts#L94),
  [215-225](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/settings-manager.ts#L215-L225),
  [1231-1274](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/settings-manager.ts#L1231-L1274)).
  The project's `.letta/settings.local.json` has the same keys for that directory
  ([157](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/settings-manager.ts#L157),
  [995-996](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/settings-manager.ts#L995-L996),
  [1365-1405](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/settings-manager.ts#L1365-L1405)).
  Both are rewritten on every start, resume and conversation switch through `persistSession`
  ([1447-1455](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/settings-manager.ts#L1447-L1455);
  [index.ts:2356](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/index.ts#L2356)),
  so they hold one session per server, never a list.
- `.letta/settings.json` `agents[]`: per-agent settings with `agentId` and `baseUrl`, absent for
  Letta Cloud, and `memfs` when the memory filesystem is on
  ([agent-settings.ts:7-19](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/agent-settings.ts#L7-L19)).
  `deviceId` identifies the install to the server
  ([settings-manager.ts:100](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/settings-manager.ts#L100)).
- `.letta/agents/<agent id>/memory/.git/config`: the memory filesystem ("memfs", the agent's memory
  blocks kept as files in a git repository) has remote `origin` at
  `<server>/v1/git/<agent id>/state.git` and is pushed there
  ([agent/memory-git.ts:1-3](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/agent/memory-git.ts#L1-L3),
  [89-96](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/agent/memory-git.ts#L89-L96),
  [228-233](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/agent/memory-git.ts#L228-L233)).
- `.letta/settings.local.json` `listenerEnvName`: the name under which `letta server` or `/listen`
  registered this machine as a remote environment with Letta Cloud (`/v1/environments/register`), so
  the web app can drive local sessions
  ([settings-manager.ts:158](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/settings-manager.ts#L158);
  [websocket/listen-register.ts:110-128](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/websocket/listen-register.ts#L110-L128)).

#### What survives a delete

Deleting a server agent from the `/agents` selector deletes it on its server and touches no local
file
([cli/components/AgentSelector.tsx:486-514](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/cli/components/AgentSelector.tsx#L486-L514)),
so its `transcripts/` directories, its `sessions.jsonl` lines and the settings entries that name it
all remain, and nothing local records that the server copy is gone. Deleting a local-backend agent
removes its conversation directories under `lc-local-backend/` (with `messages.jsonl`) and leaves
`~/.letta/transcripts/` alone
([local-store.ts:481-517](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/backend/local/local-store.ts#L481-L517);
[483-521](https://github.com/letta-ai/letta-code/blob/2a46615d16babb06cf5450b8e57ecb9bdbf2f436/src/backend/local/local-store.ts#L483-L521) at [`2a46615`](https://github.com/letta-ai/letta-code/commit/2a46615d16babb06cf5450b8e57ecb9bdbf2f436)).
The only callers of `deleteConversation` remove internal fork and ephemeral conversations
([agent/subagents/fork-conversation.ts:100](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/agent/subagents/fork-conversation.ts#L100),
[websocket/app-server-openai.ts:326](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/websocket/app-server-openai.ts#L326)).
Letta Code keeps no per-prompt history file. Since a server-agent delete touches no local file, the
chunk logs (the last 5 per agent and session, content cut to 200 characters), the interrupted-turn
records under `listener-state/`, the input ledger under `state/` and the scheduled-task run logs
also remain after it; what a local-backend delete does to them is not stated. The analyzer reads
`transcript.jsonl` as a secondary source (see
[Secondary sources](../analyzer/docs/output.md#secondary-sources)), so a transcript whose
`messages.jsonl` was deleted keeps all its rows, and reads `sessions.jsonl`, `crons.json`, the run
logs, the interrupted-turn records and the channel routing as primary sources; it does not read the
input ledger or parse the chunk logs into turns
([Past use and secondary sources](#past-use-rows-and-secondary-sources)).

### Record schema

#### Reflection transcript

`transcript.jsonl`, appended after each completed turn
([use-conversation-loop.ts:1443-1452](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/cli/app/use-conversation-loop.ts#L1443-L1452),
[reflection-transcript.ts:927-952](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/cli/helpers/reflection-transcript.ts#L927-L952)).
One object per line
([reflection-transcript.ts:54-71](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/cli/helpers/reflection-transcript.ts#L54-L71)):
`kind` in `user|assistant|reasoning|error` with `text`; or `kind:"tool_call"` with `name`,
`argsText`, `resultText`, `resultOk`. Every line has `captured_at` (ISO 8601 UTC string, the same
value for all lines of one turn batch) and optional `source_line_id`, `source_message_id`. No model,
no cwd, no tool call id: a call and its result share one line.

`captured_at` is the time of the append, not of the event: `appendTranscriptDeltaJsonl` takes
`new Date().toISOString()` once and stamps every line of the batch with it
([reflection-transcript.ts:927-946](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/cli/helpers/reflection-transcript.ts#L927-L946)).
Its three callers append a turn's lines only after the turn ends with the `end_turn` stop reason:
the interactive loop
([use-conversation-loop.ts:1423-1452](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/cli/app/use-conversation-loop.ts#L1423-L1452)),
headless mode when the result is a success
([headless.ts:4650-4652](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/headless.ts#L4650-L4652))
and the WebSocket listener's `completeSuccessfulListenerTurn`
([turn-completion.ts:67-73](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/websocket/listener/turn-completion.ts#L67-L73)).
So a user prompt is stamped later than its submission by the whole turn, tool calls and approval
waits included, and a turn that ends in an error or an interrupt is not appended at all. `argsText`
is the tool call's arguments string as streamed, the chunks concatenated
([accumulator.ts:1094](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/cli/helpers/accumulator.ts#L1094),
[1150-1153](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/cli/helpers/accumulator.ts#L1150-L1153)),
so a complete call holds the arguments JSON.

#### Local-backend transcript

`messages.jsonl` (`message_format` `pi-session-entry-jsonl`, `schema_version` 2 in `manifest.json`;
[local-transcript.ts:24-46](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/backend/local/local-transcript.ts#L24-L46)).
Line 1 is the header `{type:"session", version:3, id, timestamp, cwd}`; later lines are
`{type:"message", id, parentId, timestamp, message}` or
`{type:"compaction", id, parentId, timestamp, summary, firstKeptEntryId, tokensBefore, message, details}`
([local-transcript.ts:122-157](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/backend/local/local-transcript.ts#L122-L157),
[236-246](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/backend/local/local-transcript.ts#L236-L246),
[local-store.ts:3036-3061](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/backend/local/local-store.ts#L3036-L3061)).
Entry `id` is 8 hex characters; entry `timestamp` is ISO, taken from `message.metadata.created_at`
or `message.timestamp`
([local-transcript.ts:479-494](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/backend/local/local-transcript.ts#L479-L494)).

`message`
([local-message.ts:26-86](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/backend/local/local-message.ts#L26-L86))
has `id` (prefix `letta-msg-`,
[local-store.ts:135-137](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/backend/local/local-store.ts#L135-L137)),
`role`, `timestamp` (Unix ms), optional `metadata` {`created_at`, `agent_id`, `conversation_id`,
`provider`{`provider_id`,`model_id`,`usage`}}, and:

- `role:"user"`: `content` string or array of `text`/`image` parts
  ([pi types.ts:542-546](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/ai/src/types.ts#L542-L546)).
- `role:"assistant"`: `content` array of `{type:"text",text}`,
  `{type:"thinking",thinking,thinkingSignature?,redacted?}`, `{type:"toolCall",id,name,arguments}`;
  plus `api`, `provider`, `model`, `usage`, `stopReason`
  ([types.ts:397-427](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/ai/src/types.ts#L397-L427),
  [548-571](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/ai/src/types.ts#L548-L571)).
- `role:"toolResult"`: `toolCallId`, `toolName`, `content` (text/image), `isError`
  ([types.ts:596-609](https://github.com/earendil-works/pi/blob/200387122ca450d6387f033949423114a270b96c/packages/ai/src/types.ts#L596-L609)).
  A denied approval is written as an error `toolResult` whose text is the user's reason or
  `Tool execution denied.`
  ([src/backend/local/local-store.ts:1509-1549](https://github.com/letta-ai/letta-code/blob/2a46615d16babb06cf5450b8e57ecb9bdbf2f436/src/backend/local/local-store.ts#L1509-L1549)).

#### sessions.jsonl

`.letta/sessions.jsonl`
([session-history.ts:6-35](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/agent/session-history.ts#L6-L35)):
`agent_id`, `session_id`, `timestamp` (ms), `project`, `model`, `provider`, `usage`, `duration`,
`cost`, `message_count`, `tool_call_count`, `exit_reason`
([48-58](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/agent/session-history.ts#L48-L58),
[116-158](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/agent/session-history.ts#L116-L158)).
Session metadata, not turns. Only `recordSessionEnd` writes the file: `recordSessionStart`
([79-109](https://github.com/letta-ai/letta-code/blob/2a46615d16babb06cf5450b8e57ecb9bdbf2f436/src/agent/session-history.ts#L79-L109))
has no caller at
[`77faf36`](https://github.com/letta-ai/letta-code/commit/77faf36e34378946c8e69c0bfaa5595a7be4c326)
or [`2a46615`](https://github.com/letta-ai/letta-code/commit/2a46615d16babb06cf5450b8e57ecb9bdbf2f436),
and `recordSessionEnd` is called from the `/exit` handler with `exit_reason` `exit_command`
([cli/app/AppCoordinator.tsx:3838-3855](https://github.com/letta-ai/letta-code/blob/2a46615d16babb06cf5450b8e57ecb9bdbf2f436/src/cli/app/AppCoordinator.tsx#L3838-L3855))
and from `/logout` with `logout`
([cli/app/use-submit-handler.ts:1625-1642](https://github.com/letta-ai/letta-code/blob/2a46615d16babb06cf5450b8e57ecb9bdbf2f436/src/cli/app/use-submit-handler.ts#L1625-L1642)).
So the file holds one line per CLI session that ended with `/exit` or `/logout`, stamped with the
time of exit; a session still running, or one that ended any other way (a crash, a killed
process), has no line. `session_id` is the telemetry session id and `project` the process's
working directory.

### Joins and timestamps

- Reflection transcript: agent and conversation from the two directory names.
  Project path: the `sessions.jsonl` line with the same `agent_id` whose
  `timestamp` is the earliest at or after `captured_at`, which is the exit line of the CLI session
  the turn belongs to when that session ended with `/exit` or `/logout`; when no line is later, the
  latest line before it (heuristic; a session id is not stored in the transcript, and a session
  with no exit line is attributed to the next session's project).
- Local backend: session id is the header `id` (conversation id); project path is the header `cwd`;
  `conversation.json` gives `agent_id`
  ([local-types.ts:30-43](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/backend/local/local-types.ts#L30-L43))
  and `lc-local-backend/agents/*.json` the agent `name`, `model`, `system`
  ([local-types.ts:18-28](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/backend/local/local-types.ts#L18-L28)).
  `toolCall.id` joins `toolResult.toolCallId`. Entries chain by `parentId`.
- Subagents: `conversation.json` `parent_agent_id`, `is_subagent`.
- Transcript to local backend, for one conversation. Session id: the transcript's conversation
  directory is the conversation id sanitised to `[a-zA-Z0-9._-]`, and the `messages.jsonl` header
  `id` is the same conversation id
  ([local-transcript.ts:236-246](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/backend/local/local-transcript.ts#L236-L246)).
  Local-backend ids are `local-conv-<n>` or `local-conv-<uuid>`
  ([local-backend.ts:209](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/backend/local/local-backend.ts#L209),
  [local-store.ts:3177-3181](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/backend/local/local-store.ts#L3177-L3181),
  [762](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/backend/local/local-store.ts#L762)),
  or `default` for the agent's default conversation
  ([local-store.ts:3262-3266](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/backend/local/local-store.ts#L3262-L3266)),
  none of which the sanitisation changes, so the two `session_id`s are equal. Project path: the
  header `cwd` against the `sessions.jsonl` heuristic above; equal in the fixture, but the pairing
  is by session id and does not depend on it. Text: the same for user and assistant text, thinking
  and tool results; a tool call's `argsText`, parsed, gives the argument the pi reader shows for the
  matching `toolCall`. Time: the transcript line is later than its `messages.jsonl` entry by up to
  the turn's duration, the prompt by all of it.

### Other stores

Letta Code keeps no SQLite. Channel routing, the scheduled tasks, the interrupted turns, the input
ledger and the permission settings are cited at [`2a46615`](https://github.com/letta-ai/letta-code/commit/2a46615d16babb06cf5450b8e57ecb9bdbf2f436). Which of these stores the parser reads is
in the [Parser plan](#parser-plan).

#### V1 server database

V1 `~/.letta/sqlite.db`
([0.11.7 db.py:149](https://github.com/letta-ai/letta/blob/bf9356ceaf8ac4c80f1b418f209ad49b04986885/letta/server/db.py#L149)),
table `messages`
([orm/message.py:15-61](https://github.com/letta-ai/letta/blob/bf9356ceaf8ac4c80f1b418f209ad49b04986885/letta/orm/message.py#L15-L61)):
`id`, `agent_id`, `role` (`user|assistant|system|tool`), `text`, `content` (JSON parts), `model`,
`tool_calls` (OpenAI-shaped JSON), `tool_call_id`, `tool_returns`, `step_id`, `sequence_id`,
`created_at`
([orm/base.py:16](https://github.com/letta-ai/letta/blob/bf9356ceaf8ac4c80f1b418f209ad49b04986885/letta/orm/base.py#L16)).
Proposed as a later parser; the JSON column encodings were not verified.

#### Chunk logs

The chunk logs, `.letta/logs/chunk-logs/*/*.jsonl`, hold the last 100 server stream chunks with
content cut to 200 characters
([chunk-log.ts:1-12](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/cli/helpers/chunk-log.ts#L1-L12),
[27-30](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/cli/helpers/chunk-log.ts#L27-L30)).
Useful for message ids and timing only; not parsed into turns.

#### Channel routing

`~/.letta/channels/<channel>/routing.json`: `{routes: [...]}`, each route `accountId`, `chatId`,
`chatType`, `threadId`, `agentId`, `conversationId`, `enabled`, `outboundEnabled`, `detached`,
`createdAt` and `updatedAt` (ISO 8601): the messaging chat a conversation is relayed through
([src/channels/config.ts:35](https://github.com/letta-ai/letta-code/blob/2a46615d16babb06cf5450b8e57ecb9bdbf2f436/src/channels/config.ts#L35),
[58-60](https://github.com/letta-ai/letta-code/blob/2a46615d16babb06cf5450b8e57ecb9bdbf2f436/src/channels/config.ts#L58-L60),
[src/channels/types.ts:453-476](https://github.com/letta-ai/letta-code/blob/2a46615d16babb06cf5450b8e57ecb9bdbf2f436/src/channels/types.ts#L453-L476),
[src/channels/routing-store.ts:15-17](https://github.com/letta-ai/letta-code/blob/2a46615d16babb06cf5450b8e57ecb9bdbf2f436/src/channels/routing-store.ts#L15-L17)).
Joins on `conversationId` and `agentId`. The channels' first version named the same JSON file
`routing.yaml`; Letta reads that name only when `routing.json` is absent, links it to the new name
and removes it, so a copy can survive an interrupted migration
([src/channels/config.ts:62-69](https://github.com/letta-ai/letta-code/blob/2a46615d16babb06cf5450b8e57ecb9bdbf2f436/src/channels/config.ts#L62-L69),
[routing-store.ts:27-73](https://github.com/letta-ai/letta-code/blob/2a46615d16babb06cf5450b8e57ecb9bdbf2f436/src/channels/routing-store.ts#L27-L73)).
The built-in channel names are `telegram`, `slack`, `discord`, `custom`, `whatsapp` and `signal`
([src/channels/types.ts:72-79](https://github.com/letta-ai/letta-code/blob/2a46615d16babb06cf5450b8e57ecb9bdbf2f436/src/channels/types.ts#L72-L79)),
and `chatType` is `direct` or `channel`
([87](https://github.com/letta-ai/letta-code/blob/2a46615d16babb06cf5450b8e57ecb9bdbf2f436/src/channels/types.ts#L87)).

#### Scheduled tasks

`~/.letta/crons.json`: `{version, scheduler_owner, scheduler_owners, tasks}`, each task `id`,
`agent_id`, `conversation_id`, `name`, `description`, `cron`, `timezone`, `recurring`, `prompt`,
`status`, `created_at`, `last_fired_at`, `fire_count`, `cancel_reason`, `last_run_at`,
`last_run_outcome` and more
([src/cron/cron-file.ts:62-131](https://github.com/letta-ai/letta-code/blob/2a46615d16babb06cf5450b8e57ecb9bdbf2f436/src/cron/cron-file.ts#L62-L131));
fired, missed and cancelled tasks are dropped after a day. Beside it, `runs/<task id>.jsonl`, one
line per run: `ts` (ms), `jobId`, `action`, `status`, `outcome`, `reason`, `error`, `summary`,
`agentId`, `conversationId`, `runId` and the schedule window
([src/cron/run-log.ts:28-47](https://github.com/letta-ai/letta-code/blob/2a46615d16babb06cf5450b8e57ecb9bdbf2f436/src/cron/run-log.ts#L28-L47),
[76-83](https://github.com/letta-ai/letta-code/blob/2a46615d16babb06cf5450b8e57ecb9bdbf2f436/src/cron/run-log.ts#L76-L83)).
The task's `prompt` is the text a scheduled turn sends, wrapped in a header that names the task,
its schedule and the current time, ending `Prompt: <prompt>`
([src/cron/scheduled-task-prompt.ts:173-189](https://github.com/letta-ai/letta-code/blob/2a46615d16babb06cf5450b8e57ecb9bdbf2f436/src/cron/scheduled-task-prompt.ts#L173-L189),
[src/cron/scheduler.ts:404](https://github.com/letta-ai/letta-code/blob/2a46615d16babb06cf5450b8e57ecb9bdbf2f436/src/cron/scheduler.ts#L404)),
so no transcript line has the bare prompt. `conversation_id` is `default`, `new` (a fresh
conversation per fire) or a conversation id
([cron-file.ts:66-72](https://github.com/letta-ai/letta-code/blob/2a46615d16babb06cf5450b8e57ecb9bdbf2f436/src/cron/cron-file.ts#L66-L72));
`status` is `active`, `paused`, `fired`, `missed` or `cancelled`, `last_run_outcome` `queued`,
`missed`, `failed` or `skipped`, and `cancel_reason` `conversation_not_found` or `expired`
([33-50](https://github.com/letta-ai/letta-code/blob/2a46615d16babb06cf5450b8e57ecb9bdbf2f436/src/cron/cron-file.ts#L33-L50)).
A run log line's `status` is `ok`, `error` or `skipped` and its `action` `finished`
([run-log.ts:26-31](https://github.com/letta-ai/letta-code/blob/2a46615d16babb06cf5450b8e57ecb9bdbf2f436/src/cron/run-log.ts#L26-L31)).

#### Interrupted turns

The file is the following, both ids URL-encoded:

- `~/.letta/listener-state/<first 24 hex of sha256 of the server URL>/<agent id>_<conversation id>.json`

It holds a turn the remote listener did not finish, with `agentId`, `conversationId`, `runId`,
`toolCallIds`, `unstartedToolCallIds`, `results` (the approval results), `workingDirectory` and
`actingUserId`
([src/websocket/listener/interrupted-turn-storage.ts:51-56](https://github.com/letta-ai/letta-code/blob/2a46615d16babb06cf5450b8e57ecb9bdbf2f436/src/websocket/listener/interrupted-turn-storage.ts#L51-L56),
[interrupted-turn-types.ts:5-59](https://github.com/letta-ai/letta-code/blob/2a46615d16babb06cf5450b8e57ecb9bdbf2f436/src/websocket/listener/interrupted-turn-types.ts#L5-L59)).
The transcripts never hold an interrupted turn, so this is its only local trace. Each `results`
entry is one of the Letta client SDK's approval payloads
([src/agent/approval-execution.ts:137-142](https://github.com/letta-ai/letta-code/blob/2a46615d16babb06cf5450b8e57ecb9bdbf2f436/src/agent/approval-execution.ts#L137-L142)):
an `ApprovalReturn`, `{type: "approval", approve, tool_call_id, reason}`, the user's decision on a
call
([letta-node src/resources/agents/messages.ts:319-339](https://github.com/letta-ai/letta-node/blob/fd801c6b44b3ab2c8f5afaa577b7101ee7520f3a/src/resources/agents/messages.ts#L319-L339)),
or a `ToolReturn`, `{type: "tool", status, tool_call_id, tool_return, stdout, stderr}` with
`status` `success` or `error`, the result of a call that ran
([2208-2226](https://github.com/letta-ai/letta-node/blob/fd801c6b44b3ab2c8f5afaa577b7101ee7520f3a/src/resources/agents/messages.ts#L2208-L2226)),
which letta-code may extend with the approval's `reason`. The record has no time of its own. The
`.json.recovery-<24 hex>` sidecars and `.tmp` files beside it are recovery bookkeeping
([recovery-lineage-sidecar.ts:60-77](https://github.com/letta-ai/letta-code/blob/2a46615d16babb06cf5450b8e57ecb9bdbf2f436/src/websocket/listener/recovery-lineage-sidecar.ts#L60-L77),
[interrupted-turn-record.ts:52-53](https://github.com/letta-ai/letta-code/blob/2a46615d16babb06cf5450b8e57ecb9bdbf2f436/src/websocket/listener/interrupted-turn-record.ts#L52-L53)).

#### Input ledger

`~/.letta/state/<24 hex>/input-dispositions-v2.json`, new since
[`77faf36`](https://github.com/letta-ai/letta-code/commit/77faf36e34378946c8e69c0bfaa5595a7be4c326):
the listener's ledger of accepted remote inputs, each entry with `acceptedAt` and `expiresAt` and,
until it has been replayed after a crash, the `queuedInput` itself
([src/websocket/listener/input-disposition-path.ts:5-17](https://github.com/letta-ai/letta-code/blob/2a46615d16babb06cf5450b8e57ecb9bdbf2f436/src/websocket/listener/input-disposition-path.ts#L5-L17),
[types.ts:317-340](https://github.com/letta-ai/letta-code/blob/2a46615d16babb06cf5450b8e57ecb9bdbf2f436/src/websocket/listener/types.ts#L317-L340)).
What `queuedInput` holds was not read further.

#### Permission settings

The standing permission rules are a `permissions` object,
`{mode, allow, deny, ask, alwaysAsk, additionalDirectories}`, the rules being strings in Claude
Code's form (`Bash(npm test)`, `Edit(**)`)
([src/permissions/types.ts:9-21](https://github.com/letta-ai/letta-code/blob/2a46615d16babb06cf5450b8e57ecb9bdbf2f436/src/permissions/types.ts#L9-L21)),
merged from `~/.config/letta/settings.json`, `~/.letta/settings.json` and a project's
`.letta/settings.json` and `.letta/settings.local.json`
([src/permissions/loader.ts:59-72](https://github.com/letta-ai/letta-code/blob/2a46615d16babb06cf5450b8e57ecb9bdbf2f436/src/permissions/loader.ts#L59-L72)).
`mode` is `standard`, `acceptEdits`, `unrestricted` (the default) or `strict`; the legacy
`default`, `bypassPermissions` and `fullAccess` map onto them
([src/permissions/mode.ts:3-35](https://github.com/letta-ai/letta-code/blob/2a46615d16babb06cf5450b8e57ecb9bdbf2f436/src/permissions/mode.ts#L3-L35)).
`/approve-always` appends an `allow` rule to the scope the user picks
([src/cli/app/use-approval-flow.ts:745-756](https://github.com/letta-ai/letta-code/blob/2a46615d16babb06cf5450b8e57ecb9bdbf2f436/src/cli/app/use-approval-flow.ts#L745-L756),
[loader.ts:329-345](https://github.com/letta-ai/letta-code/blob/2a46615d16babb06cf5450b8e57ecb9bdbf2f436/src/permissions/loader.ts#L329-L345)).

### Format versions

Local backend: schema 1 (`pi-ai-message-jsonl`, bare message per line) and schema 2 (session
entries); `letta local-backend migrate-transcripts` converts 1 to 2 and leaves
`messages.jsonl.pre-pi-backup-*`
([local-transcript.ts:24-81](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/backend/local/local-transcript.ts#L24-L81)).
Messages may be re-appended with the same `message.id` as replacement snapshots
([local-transcript.ts:173-182](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/backend/local/local-transcript.ts#L173-L182));
keep the last. Reflection `state.json` `schema_version` is `v3_assistant_steps`
([reflection-transcript.ts:28-38](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/cli/helpers/reflection-transcript.ts#L28-L38)).

Between [`77faf36`](https://github.com/letta-ai/letta-code/commit/77faf36e34378946c8e69c0bfaa5595a7be4c326) (0.34.2) and [`2a46615`](https://github.com/letta-ai/letta-code/commit/2a46615d16babb06cf5450b8e57ecb9bdbf2f436) (0.34.17) the shapes the parser reads did not change:
`src/backend/local/local-message.ts`, `src/cli/helpers/reflection-transcript.ts`,
`src/agent/session-history.ts`, `src/utils/transcript-paths.ts`,
`src/utils/local-backend-paths.ts` and `src/settings-manager.ts` are byte for byte the same,
`local-transcript.ts` changed only how it reads the file, and the model fields of all three
stores are unchanged. The interrupted-turn code moved from `interrupted-turn-record.ts` into
several files, among them `src/websocket/listener/interrupted-turn-storage.ts`, and the channel
gateway modules were removed.

### Secrets

`thinkingSignature` and `textSignature` are opaque provider blobs. Tool arguments and results can
hold pasted keys. V1 `sqlite.db` table `providers` column `api_key`
([0.11.7 orm/provider.py:30-32](https://github.com/letta-ai/letta/blob/bf9356ceaf8ac4c80f1b418f209ad49b04986885/letta/orm/provider.py#L30-L32)).

### Parser plan

Parse `transcript.jsonl` and `messages.jsonl` always
([issue 122](https://github.com/seanthegeek/doubleagent/issues/122)), and `sessions.jsonl`, the
settings files, `channels/*/routing.json`, `crons.json`, `runs/*.jsonl` and the
`listener-state/*/*.json` records.

| Column | transcript.jsonl | messages.jsonl |
| --- | --- | --- |
| timestamp_utc | `captured_at` | entry `timestamp` |
| session_id | conversation directory name | header `id` |
| project_path | `sessions.jsonl` `project` (heuristic) | header `cwd` |
| git_branch | empty | empty |
| turn_type | `user`, `assistant`; `reasoning` → thinking; `error` → system; `tool_call` → one tool_use and one tool_result row | role/part type; `compaction` → system |
| model | empty | `message.model` (`responseModel` when set) |
| tool_name, tool_use_id | `name`, empty | `toolCall.name`/`id`, `toolResult.toolName`/`toolCallId` |
| text | `text`; `argsText` through pi's argument summary when it parses as JSON; `resultText` | text, thinking, the argument summary, result text |

Each `sessions.jsonl` line is one `system` row, `letta session end: agent=<agent_id>
exit_reason=<exit_reason> messages=<message_count> tool_calls=<tool_call_count>
wall_ms=<duration.wall_ms>`, at its `timestamp`, with its `session_id` and `project`; a line
without `exit_reason` or `message_count`, which no caller writes
([sessions.jsonl](#sessionsjsonl)), would read `letta session start: agent=<agent_id>
provider=<provider> model=<model> project=<project>`.

#### Permission rows

None of the transcript shapes in [Record schema](#record-schema) records a permission decision. A
denied call in `messages.jsonl` is an error `toolResult`
([Local-backend transcript](#local-backend-transcript)), shown as a plain `tool_result`.

The parser reads the standing rules, source-derived; not observed on any real install: the
`permissions` object ([Permission settings](#permission-settings)) of `~/.letta/settings.json`, the
legacy `~/.config/letta/settings.json` and a project's `.letta/settings.json` and
`.letta/settings.local.json`, one `system` row per entry, `permission setting:
permissions.<key>=<value> (from <file>, file mtime)`, the form the `claude-code` parser uses: one
per rule of `allow`, `deny`, `ask` and `alwaysAsk`, one per `additionalDirectories` entry and one
for `mode`, printed as recorded (a legacy value is not mapped). The row is at the file's
modification time, with an empty `session_id` and, for a project file, the project as
`project_path`. `<file>` is the path relative to the home or the project.

The interrupted-turn records ([Interrupted turns](#interrupted-turns)) carry the decisions of a
turn the listener did not finish: each `ApprovalReturn` in `results` is a `system` row
`permission decision: <approve|deny> reason=<reason> (from <file>, interrupted turn, file
mtime)` with the `tool_call_id` as `tool_use_id`, source-derived; not observed on any real install
(see [Past-use rows and secondary sources](#past-use-rows-and-secondary-sources)).

#### Model rows

`messages.jsonl` goes through pi's reader, so it gets pi's
`model: <id> provider=<provider>` row at the first assistant
`message.model` of a conversation and `model: <old> -> <new>
provider=<provider>` on each change
([Model in `pi.md`](pi.md#model-rows)). After the first `sessions.jsonl`
line of a `session_id` that names a `model`, a `model: <model> provider=<provider>` row follows,
and a later line of the same `session_id` with another `model` gets a `model: <old> -> <new>
provider=<provider>` row; since a session has at most one line
([sessions.jsonl](#sessionsjsonl)), the change row does not occur in practice.
`transcript.jsonl`, the scheduled tasks and the interrupted turns record no model.

#### Past-use rows and secondary sources

In local-backend mode `messages.jsonl` is the store of record, the local backend's own message
store, with models, tool ids and per-message times; it is primary. `transcript.jsonl` copies each
completed turn of the same conversation and is a secondary source: its rows are dropped when a
`messages.jsonl` row of the same conversation has the same turn type and text within one hour, the
window a turn's end-of-turn stamp needs (see [Record schema](#record-schema)); a turn longer than an
hour leaves its earlier lines shown twice. What only the transcript keeps survives the rule:
deleting a local-backend agent leaves `~/.letta/transcripts/` alone
([What survives a delete](#what-survives-a-delete)),
and in the default server-backend mode the transcript is the only local copy of the conversation.
`messages.jsonl` alone holds the turns that ended in an error or an interrupt. `sessions.jsonl` is
primary, one `system` row per line, and gives the transcript's project attribution
([Joins and timestamps](#joins-and-timestamps)).

The scheduled tasks, their runs and the interrupted turns are read as primary sources,
source-derived; not observed on any real install. None repeats a transcript event: a scheduled
turn's message wraps the prompt in a header ([Scheduled tasks](#scheduled-tasks)), and an
interrupted turn is never appended to a transcript. `-` stands for an absent value.

- `crons.json`: one `system` row per task, `scheduled task: <id> name=<name> status=<status>
  cron=<cron> timezone=<timezone> recurring=<recurring> scheduled_for=<UTC> agent=<agent_id>
  conversation=<conversation_id> fire_count=<fire_count> last_fired=<UTC> last_run=<UTC>
  outcome=<last_run_outcome> reason=<last_run_reason> error=<last_run_error>
  cancel_reason=<cancel_reason> (from .letta/crons.json, created_at) description: <description>
  prompt: <prompt>`, at `created_at` (the file's modification time, ending `file mtime)`, when it
  has none), with `conversation_id` as `session_id`, empty for `new`. A file that is not a JSON
  object is one `parser:` row. Fired, missed and cancelled tasks leave the file after a day, so the
  run logs outlast them.
- `runs/<task id>.jsonl`: one `system` row per line, `scheduled task run: <jobId> action=<action>
  status=<status> outcome=<outcome> reason=<reason> agent=<agentId>
  conversation=<conversationId> run=<runId> scheduled_for=<UTC> fired=<UTC> missed=<missedCount>
  summary=<summary> error=<error> (from <file>, ts)`, at `ts`, with `conversationId` as
  `session_id`.
- `listener-state/<24 hex>/<agent id>_<conversation id>.json`: one `system` row `interrupted turn:
  agent <agentId> conversation <conversationId> run <runId> tool_calls=<toolCallIds>
  unstarted=<unstartedToolCallIds> results=<n> working_directory=<workingDirectory>
  acting_user=<actingUserId> (from <file>, file mtime)` (ids comma-separated), then per `results`
  entry the `permission decision:` row of [Permission rows](#permission-rows) for an
  `ApprovalReturn`, or a `tool_result` row for a `ToolReturn` whose text is `tool_return`
  (`[error] ` before it when `status` is `error`), with the `tool_call_id` as `tool_use_id`. All
  are at the file's modification time, since the record has none, with `conversationId` as
  `session_id` and `workingDirectory` as `project_path`. The `.json.recovery-*` sidecars and `.tmp`
  files are not read. A file that is not a JSON object is one `parser:` row.

The input ledger is not read (what `queuedInput` holds was not reviewed), and the chunk logs are not
parsed into turns ([Other stores](#other-stores)).

#### Sync, relay and origin rows

In the default `api` backend the conversation is server state by design, not an upload: Letta Code
creates, reads and deletes the agent and its conversations through the server's API
([backend/backend.ts:376](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/backend/backend.ts#L376),
[481-524](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/backend/backend.ts#L481-L524)),
the server keeps every message, and the Letta Chat web app opens the same conversation at
`https://chat.letta.com/chat/<agent id>?conversation=<conversation id>`
([cli/helpers/app-urls.ts:14-37](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/cli/helpers/app-urls.ts#L14-L37)).
It is on unless the user ran `letta backend local`, which sets `preferredBackendMode` to `local` in
`.letta/settings.json`
([cli/subcommands/backend.ts:38](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/cli/subcommands/backend.ts#L38),
[65](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/cli/subcommands/backend.ts#L65)).
The local markers are the ids listed under
[Server-side session markers](#server-side-session-markers). The parser yields, fixture only (no
real install has shown these files):

- per `transcripts/<agent id>/<conversation id>/` whose agent id does
  not start `agent-local-`, one `system` row `session synced:
  server conversation <conversation id> agent <agent id> server <server>
  (from .letta/transcripts path, first captured_at)`, at the first
  `captured_at` of its `transcript.jsonl`, with `<server>` the
  matching `agents[]` entry's `baseUrl`, `api.letta.com` when that
  entry has none, and `-` when there is no entry;
- per `sessionsByServer` entry whose key does not start `local:`, in
  `.letta/settings.json` and in each project's
  `.letta/settings.local.json`, one `system` row `session synced: last
  session on <key> agent <agentId> conversation <conversationId> (from
  <file>, file mtime)`, at the file's modification time, with
  `session_id` the conversation id and, for the project file,
  `project_path` the project.

Memfs pushes the agent's memory to the server's git endpoint (see
[Server-side session markers](#server-side-session-markers)); a commit there is memory, not a
transcript, and is not a row. Telemetry is on unless `LETTA_CODE_TELEM` is `0` or `false` or
`DO_NOT_TRACK` is `1`, goes to `/v1/metadata/telemetry`, and carries counts, tool names and lengths,
not prompt text; error events, sent for Letta Cloud users only, add `recent_chunks` and
`debug_log_tail`, which can hold fragments of streamed output; the queue is in memory and leaves no
file
([telemetry/index.ts:37-112](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/telemetry/index.ts#L37-L112),
[392-405](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/telemetry/index.ts#L392-L405),
[776-800](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/telemetry/index.ts#L776-L800),
[478-480](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/telemetry/index.ts#L478-L480);
[backend/api/metadata.ts:100](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/backend/api/metadata.ts#L100)).
`/feedback` sends the user's message with the agent id, telemetry session id and last run id, and no
transcript
([cli/app/use-feedback-handler.ts:78-94](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/cli/app/use-feedback-handler.ts#L78-L94)).

Relay is recorded in `channels/<channel>/routing.json`, or a leftover legacy `routing.yaml` holding
the same JSON ([Channel routing](#channel-routing)). The parser reads both, source-derived; not
observed on any real install, sets `records_relay`, and yields per route one `system` row
`session relayed: <channel> chat <chatId> type <chatType> thread <threadId> account <accountId>
agent <agentId> enabled=<enabled> outbound=<outboundEnabled> detached=<detached>
updated=<updatedAt> (from <file>, createdAt)`, at `createdAt` (the file's modification time,
ending `file mtime)`, when it has none), with `conversationId` as `session_id`, `<channel>` the
directory name and `-` for an absent value. A file without a `routes` list is one `parser:` row.
`accounts.json` beside it holds the bot tokens ([Credentials](#credentials)) and is not read. No
remote-origin listing found.

#### Scratch directory rows

Classed by the catalog rule: the memory clones and their `memory-worktrees/` under
`~/.letta/agents/<agent id>/` and `~/.letta/projects/<sanitised cwd>/agent-tools/` sit under the
`.letta` entry ([Scratch directories](#scratch-directories)); none outside the home.

#### Deletion rows

Covered: no local file records a server-side delete
([What survives a delete](#what-survives-a-delete)).

#### Bad-line rows

Covered for the files read: each undecodable line of `transcript.jsonl`, `messages.jsonl`,
`sessions.jsonl` and `runs/*.jsonl` is its own `parser:` row, and a settings, routing, `crons.json`
or interrupted-turn file that is not JSON is one row for the file.

A `transcript.jsonl`, `messages.jsonl`, `sessions.jsonl` or `runs/*.jsonl` file, a settings file,
`routing.json`, `crons.json` or an interrupted-turn file that cannot be read is one whole-file row
`parser: <file> could not be read (<reason>)` through `error_rows`, `<file>` its name, a settings
file's row carrying its project path. `~/.letta/settings.json` read for a transcript's sync row
gives no row there when it cannot be read (the server is `-`); its own rows report it.

#### Fixtures

- `.letta/transcripts/agent-1a2b/conv-9f/transcript.jsonl`:

```json
{"kind":"user","text":"list the repo","captured_at":"2026-10-01T12:00:05.120Z","source_line_id":"ui-1"}
{"kind":"tool_call","name":"Bash","argsText":"{\"command\":\"ls\"}","resultText":"README.md","resultOk":true,"captured_at":"2026-10-01T12:00:05.120Z","source_line_id":"ui-2"}
{"kind":"assistant","text":"One file: README.md","captured_at":"2026-10-01T12:00:05.120Z","source_line_id":"ui-3","source_message_id":"message-77"}
```

- `.letta/lc-local-backend/conversations/Y29udmVyc2F0aW9uOmxvY2FsLWNvbnYtMQ/messages.jsonl`:

```json
{"type":"session","version":3,"id":"local-conv-1","timestamp":"2026-10-01T12:00:00.000Z","cwd":"/home/alice/repo"}
{"type":"message","id":"a1b2c3d4","parentId":null,"timestamp":"2026-10-01T12:00:01.000Z","message":{"id":"letta-msg-1","role":"user","content":"list the repo","timestamp":1759320001000}}
{"type":"message","id":"b2c3d4e5","parentId":"a1b2c3d4","timestamp":"2026-10-01T12:00:02.000Z","message":{"id":"letta-msg-2","role":"assistant","content":[{"type":"thinking","thinking":"use ls"},{"type":"toolCall","id":"call_1","name":"Bash","arguments":{"command":"ls"}}],"api":"anthropic-messages","provider":"anthropic","model":"claude-sonnet-4-5","usage":{"input":10,"output":5,"cacheRead":0,"cacheWrite":0,"totalTokens":15,"cost":{"input":0,"output":0,"cacheRead":0,"cacheWrite":0,"total":0}},"stopReason":"toolUse","timestamp":1759320002000}}
{"type":"message","id":"c3d4e5f6","parentId":"b2c3d4e5","timestamp":"2026-10-01T12:00:03.000Z","message":{"id":"letta-msg-3","role":"toolResult","toolCallId":"call_1","toolName":"Bash","content":[{"type":"text","text":"README.md"}],"isError":false,"timestamp":1759320003000}}
```

- `.letta/sessions.jsonl`, the exit line `/exit` writes:

```json
{"agent_id":"agent-1a2b","session_id":"s-1","timestamp":1759320060000,"project":"/home/alice/repo","model":"claude-sonnet-4-5","provider":"anthropic","usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0,"cached_input_tokens":0,"cache_write_tokens":0,"reasoning_tokens":0,"steps":0},"duration":{"api_ms":900,"wall_ms":60000},"cost":{"type":"hosted"},"message_count":4,"tool_call_count":1,"exit_reason":"exit_command"}
```

- `.config/letta/settings.json` (or any of the four settings files), the `permissions` object:

```json
{"permissions":{"mode":"acceptEdits","allow":["Bash(npm test)"],"deny":["Bash(rm -rf:*)"],"additionalDirectories":["/srv/shared"]}}
```

- `.letta/channels/telegram/routing.json`:

```json
{"routes":[{"accountId":"bot-1","chatId":"123456789","chatType":"direct","threadId":null,"agentId":"agent-1a2b","conversationId":"conv-9f","enabled":true,"outboundEnabled":true,"createdAt":"2026-10-01T12:20:00.000Z","updatedAt":"2026-10-01T12:21:00.000Z"}]}
```

- `.letta/crons.json` (one task, abridged to the keys the parser reads) and
  `.letta/runs/a1b2c3d4.jsonl`:

```json
{"version":1,"scheduler_owner":null,"scheduler_owners":{},"tasks":[{"id":"a1b2c3d4","agent_id":"agent-1a2b","conversation_id":"conv-9f","name":"nightly tests","description":"run the suite","cron":"0 2 * * *","timezone":"UTC","recurring":true,"prompt":"run the test suite and report failures","status":"active","created_at":"2026-10-01T12:10:00.000Z","last_fired_at":"2026-10-02T02:00:00.000Z","fire_count":1,"cancel_reason":null,"last_run_at":"2026-10-02T02:00:00.000Z","last_run_outcome":"queued","last_run_reason":"scheduled_time_matched","last_run_error":null,"scheduled_for":null}]}
```

```json
{"ts":1790906400000,"jobId":"a1b2c3d4","action":"finished","status":"ok","outcome":"queued","reason":"scheduled_time_matched","summary":"nightly tests","agentId":"agent-1a2b","conversationId":"conv-9f","runId":"run-1","firedAt":"2026-10-02T02:00:00.000Z"}
{"ts":1790992800000,"jobId":"a1b2c3d4","action":"finished","status":"skipped","outcome":"missed","reason":"scheduler_inactive","summary":"missed","agentId":"agent-1a2b","conversationId":"conv-9f","missedCount":1}
```

- `.letta/listener-state/0123456789abcdef01234567/agent-1a2b_conv-9f.json`:

```json
{"agentId":"agent-1a2b","conversationId":"conv-9f","runId":"run-2","toolCallIds":["call_7","call_8"],"unstartedToolCallIds":["call_8"],"results":[{"type":"tool","status":"error","tool_call_id":"call_7","tool_return":"permission denied","stdout":[],"stderr":[]},{"type":"approval","approve":false,"tool_call_id":"call_8","reason":"not on main"}],"requestOtid":"otid-1","workingDirectory":"/srv/proj","actingUserId":"user-1"}
```

#### Source host rows

The agent records no source-host integration. A grep of letta-code at
[`2a46615`](https://github.com/letta-ai/letta-code/commit/2a46615d16babb06cf5450b8e57ecb9bdbf2f436)
for `github`, `gitlab`, `bitbucket`, `oauth`, `installation`, `repositor`, `pull request` and
`clone` finds the `WatchPR` tool, which polls a github.com pull request through the user's own
locally authenticated `gh` CLI
([src/tools/descriptions/WatchPR.md:11](https://github.com/letta-ai/letta-code/blob/2a46615d16babb06cf5450b8e57ecb9bdbf2f436/src/tools/descriptions/WatchPR.md#L11),
[src/tools/impl/github-pull-request-watch.ts:414](https://github.com/letta-ai/letta-code/blob/2a46615d16babb06cf5450b8e57ecb9bdbf2f436/src/tools/impl/github-pull-request-watch.ts#L414))
and keeps no token, account or repository list of its own, and GitHub Copilot as a model
provider, whose OAuth token
([src/types/provider-oauth-config.ts:18](https://github.com/letta-ai/letta-code/blob/2a46615d16babb06cf5450b8e57ecb9bdbf2f436/src/types/provider-oauth-config.ts#L18))
is a model credential in `providers/auth.json`
([Credentials](#credentials)), not a repository grant. The memfs git remote is Letta's own server
([Server-side session markers](#server-side-session-markers)).

## Confidence

- High (source): every Letta Code path, the keychain fallback, the credential helper, the
  local-backend layout, the line shapes, field names and timestamp units.
- High (source): V1 `~/.letta/sqlite.db` at 0.11.7 and `~/.memgpt` at 0.3.25.
- Medium: pi-ai optional fields at the pinned version, since the pi checkout is newer
  ([Source and evidence](#source-and-evidence)); the `api` value in the fixture is illustrative.
- Medium: the release in which V1 dropped SQLite (between 0.11.7 and the archive head, not
  bisected).
- Not determined: the V1 `content` and `tool_calls` JSON encodings
  ([V1 server database](#v1-server-database)).
- Not determined: whether reflection transcripts are written on Windows when auto-reflection is
  off (`LETTA_ENABLE_WINDOWS_AUTO_REFLECTION`,
  [reflection-settings.ts:9](https://github.com/letta-ai/letta-code/blob/77faf36e34378946c8e69c0bfaa5595a7be4c326/src/reflection-settings.ts#L9),
  exists, but the append path at the cited loop is unconditional).
- Not determined: the Letta Desktop app's Electron data directory (not in either repository).
- Not determined: the contents of `workflows/executions` and `reflection-arena`, and whether mods
  install `node_modules`.
