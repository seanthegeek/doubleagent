# ollama

Ollama (catalog agent `ollama`) runs language models locally: a model server and CLI on every
OS, a terminal REPL (`ollama run`), a launcher for other agents (`ollama launch`, which points
Claude Code, Claude Desktop, the ChatGPT desktop app's Codex, Qwen Code, Droid and others at the
local server) and, on macOS and Windows only, a desktop chat app. Versions `v0.31.2` to
`v0.34.0` shipped a built-in agent that was later removed. Only the desktop app keeps
transcripts, in one SQLite database. The REPL keeps a prompt history without times. The server
keeps nothing, and keeps no copy of what a launched agent sends or receives
([Overlap with launched agents](#overlap-with-launched-agents)). The agent is parsed: the
`ollama` parser reads the desktop app's database, the REPL history and the auto-mode setting
the launcher keeps for Claude Desktop ([Permission rows](#permission-rows)).

## Source and evidence

[ollama/ollama](https://github.com/ollama/ollama), open source, MIT, Go, at tag `v0.35.1`, commit
[`b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce`](https://github.com/ollama/ollama/commit/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce)
(2026-10-01), the latest release and the version installed on the workstation used for the local
check under [Observed](#observed); shallow clone at `scratchpad/repos/ollama-0351`. The code was
only read, never built or run. The first version of this document cited
[`42e911bc3d05798cad729cb474bf62f378cb2e26`](https://github.com/ollama/ollama/commit/42e911bc3d05798cad729cb474bf62f378cb2e26)
(2026-10-02), two commits after `v0.35.1` that change only one documentation page, so every line
cited here was re-checked at `v0.35.1` and is unchanged. The removed built-in agent
([Built-in agent](#built-in-agent-v0312-to-v0340)) was read at
[`b68b112bd8868d6278250d7d4bdfafa5cbf035c8`](https://github.com/ollama/ollama/commit/b68b112bd8868d6278250d7d4bdfafa5cbf035c8),
its last state, from the history of the same checkout. The SQLite driver is
[mattn/go-sqlite3](https://github.com/mattn/go-sqlite3) v1.14.24
([`go.mod:12`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/go.mod#L12)),
read at
[`846fea6c1443e8cc366fc1966fe078d7f825f6a9`](https://github.com/mattn/go-sqlite3/commit/846fea6c1443e8cc366fc1966fe078d7f825f6a9).
The HTTP router is gin v1.10.0
([`go.mod:8`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/go.mod#L8)),
read at
[`75ccf94d605a05fe24817fc2f166f6f2959d5cea`](https://github.com/gin-gonic/gin/commit/75ccf94d605a05fe24817fc2f166f6f2959d5cea).
All claims below are evidence level **A** (source, file:line) unless marked **D** (repo docs
`docs/*.mdx`) or **L** (local layout check, `ls ~/.ollama`, names only).

### Observed

- CLI and server, Linux: Ollama 0.35.1 systemd install on the workstation (service user `ollama`,
  home `/usr/share/ollama`), checked in October 2026 by listing names only (L); the same request
  check was run earlier against 0.30.7. The user's `~/.ollama` held the CLI files of
  [CLI and server](#cli-and-server) but no `launch/`, `logs/` or `chatgpt-ollama/`; its
  `config.json` keys and `backup/` copy are under [Launch configuration](#launch-configuration),
  the five-entry `history` under [`.ollama/history`](#ollamahistory). One request each to
  `/api/chat`, `/v1/chat/completions` and `/v1/messages` with a unique marker wrote no file and
  left the marker in no file and no journal line ([Server logs](#server-logs)).
- CLI and server, macOS and Windows: no real install.
- Desktop app, macOS and Windows: no real install; the app does not exist on Linux, so
  `db.sqlite` ([Transcript stores](#transcript-stores)) rests on source alone.

### Source reviews

- 2026-10-21: ollama/ollama at
  [`6787222`](https://github.com/ollama/ollama/commit/678722224b3d8c2a024a4fa5f530d83ce34db120)
  (`main` after `v0.40.6`): the macOS app's Claude Desktop setting, the macOS-only launch and
  the restore that removes the key went into
  [Claude Desktop third-party mode](#claude-desktop-third-party-mode); the parser now reads
  `.ollama/config.json` and the Claude Desktop profile for the auto-mode setting
  ([Permission rows](#permission-rows)), and [Source host rows](#source-host-rows) was added.
- 2026-10-19: HEAD
  [`6787222`](https://github.com/ollama/ollama/commit/678722224b3d8c2a024a4fa5f530d83ce34db120)
  of `main` (the latest release tag was `v0.40.6`, 2026-10-16; no release `v0.36` to `v0.39` was
  tagged), compared with `v0.35.1`. It added `models/manifests-v2/` and the
  [Scratch directories](#scratch-directories) to [Per-user storage](#per-user-storage), the
  [Claude Desktop third-party mode](#claude-desktop-third-party-mode) profile to
  [Project-local files](#project-local-files), the
  standing `automode` setting to [Permission rows](#permission-rows), the unchanged parser inputs
  to [Format versions](#format-versions) and the plan for history lines that are not UTF-8 to
  [Bad-line rows](#bad-line-rows); the citations at
  [`b68b112`](https://github.com/ollama/ollama/commit/b68b112bd8868d6278250d7d4bdfafa5cbf035c8)
  name files of the removed agent and are meant to be absent at HEAD.

## Collection

### Per-user storage

#### Scratch directories

At [`6787222`](https://github.com/ollama/ollama/commit/678722224b3d8c2a024a4fa5f530d83ce34db120),
Ollama has none under the home. Two temporary
directories go to the system temp directory and are removed when
the command finishes: `ollama-qwen-install-*`, a shim for the Qwen
Code installer
([`cmd/launch/qwen.go:143-150`](https://github.com/ollama/ollama/blob/678722224b3d8c2a024a4fa5f530d83ce34db120/cmd/launch/qwen.go#L143-L150)),
and `ollama-codex-models-*`, a scratch `CODEX_HOME` into which
`ollama launch chatgpt` copies `~/.codex/auth.json` and
`models_cache.json` to list Codex's native models
([`cmd/launch/codex_app.go:1480-1495`](https://github.com/ollama/ollama/blob/678722224b3d8c2a024a4fa5f530d83ce34db120/cmd/launch/codex_app.go#L1480-L1495)).
A crash leaves a copy of the Codex credential in the temp directory,
outside every catalog line. The desktop app's `settings.working_dir`
is the only working directory it records
([Where the project path is recorded](#where-the-project-path-is-recorded)).

#### CLI and server

All OSes, `os.UserHomeDir()`. On the Linux install of [Observed](#observed), `~/.ollama/` held
`backup/config.json.<epoch>`, `cache/model-recommendations.json`, `config.json`, `history`,
`id_ed25519`, `id_ed25519.pub`, `models/blobs/` (6 blobs, 1.9 GB) and
`models/manifests/registry.ollama.ai/library/llama3.2/`, and no `launch/`, `logs/` or
`chatgpt-ollama/` directory.

- `~/.ollama/models/` —
  [`envconfig/config.go:111-123`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/envconfig/config.go#L111-L123)
  (`OLLAMA_MODELS` override, default `$HOME/.ollama/models`);
  `models/manifests/<host>/<ns>/<model>/<tag>` and `models/blobs/sha256-<digest>`
  ([`manifest/paths.go:30`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/manifest/paths.go#L30),[`49`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/manifest/paths.go#L49),[`61`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/manifest/paths.go#L61));
  `models/metadata/sha256-<digest>.json`, one small JSON file per blob with its GGUF key-value
  metadata, arrays longer than 4096 elements (tokenizer vocabularies) recorded by key only
  ([`server/gguf_metadata.go:26-38`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/server/gguf_metadata.go#L26-L38),[`101-110`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/server/gguf_metadata.go#L101-L110));
  at `v0.35.1` no other directory is created under `models/` (the three `envconfig.Models()` joins
  above are the only ones outside tests), and since
  [`6787222`](https://github.com/ollama/ollama/commit/678722224b3d8c2a024a4fa5f530d83ce34db120) `models/manifests-v2/` sits beside the legacy `manifests/`, the same kind of small
  manifest file
  ([`manifest/paths.go:23-24`](https://github.com/ollama/ollama/blob/678722224b3d8c2a024a4fa5f530d83ce34db120/manifest/paths.go#L23-L24));
  transient `models/blobs/.ollama-split-*`
  ([`llm/model_split.go:21-23`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/llm/model_split.go#L21-L23)).
  There is no `OLLAMA_HOME` variable; only `OLLAMA_MODELS` relocates data (grep of
  [`envconfig/config.go`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/envconfig/config.go#L315-L336)
  lines 315-336).
- `~/.ollama/history` — REPL readline history, mode 0600
  ([`readline/history.go:41-52`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/readline/history.go#L41-L52));
  disabled by `OLLAMA_NOHISTORY`
  ([`envconfig/config.go:224`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/envconfig/config.go#L224),[`333`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/envconfig/config.go#L333)).
- `~/.ollama/id_ed25519`, `id_ed25519.pub` — generated on first serve
  ([`cmd/cmd.go:2120-2126`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/cmd.go#L2120-L2126)),
  used to sign registry/cloud requests
  ([`auth/auth.go:19`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/auth/auth.go#L19),[`53-59`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/auth/auth.go#L53-L59)).
- `~/.ollama/server.json` — `disable_ollama_cloud`
  ([`envconfig/config.go:382-404`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/envconfig/config.go#L382-L404),
  [`app/store/cloud_config.go:15-30`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/store/cloud_config.go#L15-L30)).
- `~/.ollama/config.json` — `ollama launch` integration configuration, which integrations were set
  up with which model
  ([`cmd/config/config.go:88`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/config/config.go#L88);
  keys under [Launch configuration](#launch-configuration)). Legacy `~/.ollama/config/config.json`
  ([`:96`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/config/config.go#L96)).
- `~/.ollama/onboarding-v1.completed` — on macOS and Windows, an empty marker of when onboarding
  was first completed
  ([`internal/onboarding/state.go:12-29`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/internal/onboarding/state.go#L12-L29);
  [Launch configuration](#launch-configuration)).
- `~/.ollama/chatgpt-ollama/` — on macOS, an `electron-data/` profile directory and a `chatgpt.pid`
  file named by the desktop app's ChatGPT integration, normally empty
  ([`cmd/launch/codex_app.go:38-40`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/codex_app.go#L38-L40);
  [Launch state](#launch-state)).
- `~/.ollama/backup/<integration>/<file>.<epoch>` — copies of the config files `ollama launch`
  overwrote, other agents' and Ollama's own
  ([`cmd/internal/fileutil/files.go:54-65`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/internal/fileutil/files.go#L54-L65),
  [`76-88`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/internal/fileutil/files.go#L76-L88);
  [Launch state](#launch-state)).
- `~/.ollama/cache/model-recommendations.json`
  ([`server/model_recommendations.go:310-314`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/server/model_recommendations.go#L310-L314));
  no `cache/show` is written
  ([`server/model_show_cache.go:49-50`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/server/model_show_cache.go#L49-L50)).
- `~/.ollama/launch/{muse-config, dsh, codex-app-restore.json, chatgpt-session-start}`
  ([`cmd/launch/muse.go:218`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/muse.go#L218),
  [`deepseek_harness.go:503`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/deepseek_harness.go#L503),
  [`codex_app.go:2653`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/codex_app.go#L2653),
  [`codex_app_profile.go:86`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/codex_app_profile.go#L86)):
  integration settings and launch markers ([Launch state](#launch-state)).
- `~/.ollama/logs/codex-proxy.log` — activity log of the Codex desktop proxy, under the home of the
  account the server runs as
  ([`server/codex_proxy.go:31`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/server/codex_proxy.go#L31);
  [Codex proxy log](#codex-proxy-log)).
- Linux systemd install: service user `ollama`, home `/usr/share/ollama`, so data is
  `/usr/share/ollama/.ollama/`; on the install of [Observed](#observed) some of its files were
  readable only by that account ([`scripts/install.sh:200`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/scripts/install.sh#L200),[`222`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/scripts/install.sh#L222);
  D
  [`docs/faq.mdx:236`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/docs/faq.mdx#L236),[`398`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/docs/faq.mdx#L398)).
  Docker: `/root/.ollama` (D
  [`docs/docker.mdx:4`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/docs/docker.mdx#L4)).
  `OLLAMA_DEBUG_LOG_REQUESTS` writes request bodies to a temp dir `ollama-request-logs-*`, outside
  the home ([Server logs](#server-logs))
  ([`server/inference_request_log.go:25-34`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/server/inference_request_log.go#L25-L34)).
- Skills of the removed built-in agent (`v0.31.2` to `v0.34.0`): `SkillsDir()` returned
  `$OLLAMA_SKILLS` when set, else `$XDG_CONFIG_HOME/ollama/skills` when `XDG_CONFIG_HOME` was set,
  else `~/.ollama/skills`; the bundled `skill-creator/SKILL.md` and skills imported from
  `~/.codex/skills`, `~/.claude/skills` or `~/.pi/agent/skills` were written there
  ([`agent/skills.go:79-92`](https://github.com/ollama/ollama/blob/b68b112bd8868d6278250d7d4bdfafa5cbf035c8/agent/skills.go#L79-L92),[`250-270`](https://github.com/ollama/ollama/blob/b68b112bd8868d6278250d7d4bdfafa5cbf035c8/agent/skills.go#L250-L270),[`296-316`](https://github.com/ollama/ollama/blob/b68b112bd8868d6278250d7d4bdfafa5cbf035c8/agent/skills.go#L296-L316),
  at [`b68b112`](https://github.com/ollama/ollama/commit/b68b112bd8868d6278250d7d4bdfafa5cbf035c8)).
  It also read, never wrote, `~/.agents/skills`, `<project>/.agents/skills` and
  `<project>/.ollama/skills`
  ([`skills.go:200-210`](https://github.com/ollama/ollama/blob/b68b112bd8868d6278250d7d4bdfafa5cbf035c8/agent/skills.go#L200-L210),[`625-650`](https://github.com/ollama/ollama/blob/b68b112bd8868d6278250d7d4bdfafa5cbf035c8/agent/skills.go#L625-L650)).
  The default is inside `~/.ollama`, which `ollama|.ollama` already collects. The XDG form is used
  only when the variable is set, which no distribution does by default, and neither the code nor
  `docs/` at that commit names a value for it (grep of `docs/` for `XDG` and `skills` finds only
  Hermes pages), so no `.config/ollama` catalog line is added; no catalog entry claims
  `.config/ollama` today. No Go code at `v0.35.1` names a `skills` directory (grep of non-test
  `*.go` files for `"skills"`), so an upgrade leaves the directory in place.

#### Desktop app, macOS

The desktop app's code is under `app/`.

- `~/.ollama/logs/server.log`, `app.log`, `upgrade.log`
  ([`app/server/server_unix.go:20`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/server/server_unix.go#L20),
  [`app/cmd/app/app_darwin.go:81`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/cmd/app/app_darwin.go#L81),
  [`app/updater/updater_darwin.go:86`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/updater/updater_darwin.go#L86);
  D
  [`docs/macos.mdx:25-28`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/docs/macos.mdx#L25-L28)).
- `~/Library/Application Support/Ollama/db.sqlite` — the chat database: chats, messages, tool calls,
  attachments, settings and the cached account ([`db.sqlite` in Record schema](#dbsqlite))
  ([`internal/onboarding/app_state.go:26-34`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/internal/onboarding/app_state.go#L26-L34),
  [`app/store/database.go:65-155`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/store/database.go#L65-L155)).
  WAL mode
  ([`database.go:31`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/store/database.go#L31)),
  so `db.sqlite-wal`/`-shm` sidecars.
- `~/Library/Application Support/Ollama/config.json` (legacy app config,
  [`app/store/store.go:209`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/store/store.go#L209)),
  `ollama.pid`
  ([`server_unix.go:19`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/server/server_unix.go#L19)),
  `~/Library/LaunchAgents/com.ollama.ollama.plist`
  ([`app_darwin.go:82`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/cmd/app/app_darwin.go#L82)).
  Low value: `~/Library/Caches/ollama`, `~/Library/Caches/com.electron.ollama`,
  `~/Library/WebKit/com.electron.ollama`,
  `~/Library/Saved Application State/com.electron.ollama.savedState` (D
  [`docs/macos.mdx:38-42`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/docs/macos.mdx#L38-L42)).

#### Desktop app, Windows

- `%LOCALAPPDATA%\Ollama\{db.sqlite, server.log, server-#.log, app.log, config.json, ollama.pid, updates\}`
  ([`app_state.go:33`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/internal/onboarding/app_state.go#L33),
  [`app/server/server_windows.go:18-19`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/server/server_windows.go#L18-L19),
  [`app/cmd/app/app_windows.go:39`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/cmd/app/app_windows.go#L39),
  [`store.go:207`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/store/store.go#L207),
  [`app/ollama.iss:120`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/ollama.iss#L120);
  D
  [`docs/windows.mdx:66-71`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/docs/windows.mdx#L66-L71)).
  Binaries in `%LOCALAPPDATA%\Programs\Ollama`. Models, history, keys stay in
  `%USERPROFILE%\.ollama`
  ([`ollama.iss:139`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/ollama.iss#L139),[`260-261`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/ollama.iss#L260-L261)).
- Linux has no desktop database (`AppDatabasePath` returns "" by default,
  [`app_state.go:35`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/internal/onboarding/app_state.go#L35)).

### Credentials

- `~/.ollama/id_ed25519` is the only credential: Ollama cloud "sign-in" binds this public key to the
  account
  ([`app/auth/connect.go:14-25`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/auth/connect.go#L14-L25),
  [`cmd/cmd.go:1012-1026`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/cmd.go#L1012-L1026));
  no token file is written. `id_ed25519.pub` identifies the device.
- No keychain use anywhere in the tree (grep `keychain|Keychain` in cmd, auth, server, app: none).
- `~/.ollama/config.json` and `server.json` hold no Ollama secret. `~/.ollama/backup/*` holds
  copies of other tools' settings files, which can hold API keys, and
  `launch/muse-config/muse/settings.json` is a copy of Muse's flagged `settings.json`
  ([Launch state](#launch-state)); both are flagged (`.ollama/backup/*`,
  `.ollama/launch/muse-config/muse/settings.json`). The Docker image starts the server
  ([`Dockerfile:341-342`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/Dockerfile#L341-L342))
  but ships the CLI, which the documentation runs with `docker exec`
  ([`docs/docker.mdx:89`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/docs/docker.mdx#L89)),
  and `ollama launch` offers to install a missing integration
  ([`cmd/launch/muse.go:69`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/muse.go#L69)),
  so both copies can land in `/root/.ollama`, the volume; the volume-relative `backup/*` and
  `launch/muse-config/muse/settings.json` flag them there.
- `db.sqlite` holds the account's identity and user-uploaded files; the columns are in
  [Secrets](#secrets).
- `ollama launch chatgpt` creates `~/.codex/auth.json` when none exists, holding the fixed sentinel
  `{"OPENAI_API_KEY": "ollama-local-codex", "auth_mode": "apikey"}`, which is not an OpenAI
  credential ([Project-local files](#project-local-files)). The existing `.codex/auth.json` secret
  glob flags it like a real one.

### Exclusions

- `.ollama/models/blobs`: model weights (multi-GB `sha256-<digest>` files), the transient
  `.ollama-split-*` hard-link directories, and any `/save`d REPL session layer, see
  [Transcript stores](#transcript-stores)
  ([`manifest/paths.go:52-72`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/manifest/paths.go#L52-L72),
  [`llm/model_split.go:21-23`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/llm/model_split.go#L21-L23)).
  The rest of `models/` is collected: `manifests/<host>/<namespace>/<model>/<tag>` (and
  `manifests-v2/` since [`6787222`](https://github.com/ollama/ollama/commit/678722224b3d8c2a024a4fa5f530d83ce34db120)), small JSON files whose names and pre-copy times are an
  inventory of the pulled models and when each tag was pulled, and `metadata/*.json`, one small
  file per blob ([Per-user storage](#per-user-storage)).
- `AppData/Local/Ollama/updates`. `Library/Caches/ollama`, `Library/Caches/com.electron.ollama`
  and `Library/WebKit/com.electron.ollama` are outside every catalog line, so they are not
  collected and need no exclusion.
- Docker volumes: the volume-relative exclusion is `models`, whole, so a Docker Ollama volume's
  `models/manifests` is not collected
  ([`collectors/docs/coverage.md`](../collectors/docs/coverage.md)).

### Project-local files

None. Ollama writes nothing into repositories. `ollama launch` edits other agents' configs under
their own homes
([`cmd/launch/droid.go:60`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/droid.go#L60),
[`qwen.go:381`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/qwen.go#L381),
…); for Claude Code it writes nothing and sets `ANTHROPIC_BASE_URL` and `ANTHROPIC_AUTH_TOKEN` in
the environment instead
([`claude.go:69-71`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/claude.go#L69-L71)).

#### ChatGPT desktop app

`ollama launch chatgpt` (macOS and Windows only,
[`codex_app.go:885-892`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/codex_app.go#L885-L892))
points the ChatGPT desktop app's Codex at Ollama's proxy by writing into `~/.codex`. The path is
always `<home>/.codex`; `CODEX_HOME` is not consulted
([`codex.go:234-240`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/codex.go#L234-L240)).
In order
([`codex_app.go:101-147`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/codex_app.go#L101-L147)):

- `~/.ollama/launch/codex-app-restore.json` saves Codex's previous root values
  ([Per-user storage](#per-user-storage)).
- `~/.codex/ollama-launch-codex-routing.json`, the Ollama-only routing allow-list, and
  `~/.codex/ollama-launch-models.json`, the combined native and Ollama model picker catalog
  ([`internal/proxy/codex_desktop.go:32-38`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/internal/proxy/codex_desktop.go#L32-L38);
  [`codex_app.go:952-958`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/codex_app.go#L952-L958),[`966-1011`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/codex_app.go#L966-L1011),[`1111-1155`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/codex_app.go#L1111-L1155)).
- `~/.codex/auth.json`, only when no such file exists, with `OPENAI_API_KEY` set to the sentinel
  `ollama-local-codex` and `auth_mode` `apikey`, mode 0600; `--restore` removes it only if it still
  holds that sentinel
  ([`codex_app.go:561-631`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/codex_app.go#L561-L631);
  [`codex_desktop.go:40-42`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/internal/proxy/codex_desktop.go#L40-L42)).
  An `auth.json` with that value means the ChatGPT integration was set up while no Codex login
  existed.
- `~/.codex/config.toml`: root `model` set to the chosen model, `model_catalog_json` to the picker
  catalog, `openai_base_url` to `<ollama host>/api/codex/v1`, `profile` and `model_provider`
  removed, and the `[desktop]` reasoning efforts extended
  ([`codex_app.go:312-354`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/codex_app.go#L312-L354)).

The two catalogs and `config.toml` go through `WriteWithBackup` with the integration name
`codex-app`, so their previous versions land in `~/.ollama/backup/codex-app/`
([Per-user storage](#per-user-storage)). It reads, without writing, `~/.codex/models_cache.json` and
`sessions/`
([`codex_app.go:1447-1466`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/codex_app.go#L1447-L1466),[`1511`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/codex_app.go#L1511);
[`codex_app_profile.go:54-79`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/codex_app_profile.go#L54-L79)).
The Codex CLI integration, `ollama launch codex`, writes `~/.codex/model.json` and
`~/.codex/ollama-launch.config.toml` instead and leaves `config.toml` alone
([`codex.go:209-232`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/codex.go#L209-L232),[`250-268`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/codex.go#L250-L268),[`675-688`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/codex.go#L675-L688)).
All of these are under the `codex-cli|.codex` catalog line; [codex-cli.md](codex-cli.md) lists them.

#### Claude Desktop third-party mode

At
[`6787222`](https://github.com/ollama/ollama/commit/678722224b3d8c2a024a4fa5f530d83ce34db120),
`ollama launch claude-desktop` writes a gateway profile for Claude
Desktop's third-party mode (the build that runs inference through a
provider the user names) into
`Library/Application Support/Claude-3p/configLibrary/00000000-0000-4000-8000-000000000114.json`
on macOS, beside `_meta.json` and `claude_desktop_config.json` in the
same base
([`cmd/launch/claude_desktop.go:22-24`](https://github.com/ollama/ollama/blob/678722224b3d8c2a024a4fa5f530d83ce34db120/cmd/launch/claude_desktop.go#L22-L24),
[570-583](https://github.com/ollama/ollama/blob/678722224b3d8c2a024a4fa5f530d83ce34db120/cmd/launch/claude_desktop.go#L570-L583),
[634-635](https://github.com/ollama/ollama/blob/678722224b3d8c2a024a4fa5f530d83ce34db120/cmd/launch/claude_desktop.go#L634-L635)),
and the Windows equivalent under `AppData/Local/Claude-3p` or
`Claude Nest-3p`
([644-649](https://github.com/ollama/ollama/blob/678722224b3d8c2a024a4fa5f530d83ce34db120/cmd/launch/claude_desktop.go#L644-L649)).
The profile sets `inferenceProvider` `gateway`,
`inferenceGatewayBaseUrl` to the local server, `inferenceGatewayApiKey`
to the literal `ollama` (not a credential), `coworkEgressAllowedHosts`,
both telemetry switches off, and `autoModeEnabled`
([741-759](https://github.com/ollama/ollama/blob/678722224b3d8c2a024a4fa5f530d83ce34db120/cmd/launch/claude_desktop.go#L741-L759)).
`autoModeEnabled` comes from `integrations.claude-desktop.automode` in
`~/.ollama/config.json` and is true when that key is unset
([320-345](https://github.com/ollama/ollama/blob/678722224b3d8c2a024a4fa5f530d83ce34db120/cmd/launch/claude_desktop.go#L320-L345);
[`cmd/config/config.go:19-24`](https://github.com/ollama/ollama/blob/678722224b3d8c2a024a4fa5f530d83ce34db120/cmd/config/config.go#L19-L24)),
so a Claude Desktop launched through Ollama runs with auto mode on
unless the user turned it off. Writing `autoModeEnabled` into the
profile is new since `v0.35.1` (`autoModeEnabled` does not occur in
`cmd/launch/claude_desktop.go` at that tag). The macOS desktop app's
Claude Desktop settings toggle saves the `automode` key with
`SaveIntegrationAutoMode` and, when Claude Desktop already uses the
Ollama profile, rewrites the profile with the new value
([`app/cmd/app/app_darwin.go:1497-1522`](https://github.com/ollama/ollama/blob/678722224b3d8c2a024a4fa5f530d83ce34db120/app/cmd/app/app_darwin.go#L1497-L1522);
[`cmd/config/config.go:217-235`](https://github.com/ollama/ollama/blob/678722224b3d8c2a024a4fa5f530d83ce34db120/cmd/config/config.go#L217-L235)).
The profile is written on macOS only: `claudeDesktopSupported` refuses
every other OS
([`cmd/launch/claude_desktop.go:425-430`](https://github.com/ollama/ollama/blob/678722224b3d8c2a024a4fa5f530d83ce34db120/cmd/launch/claude_desktop.go#L425-L430)),
while `ollama launch claude-desktop --restore` runs on macOS and Windows
([432-437](https://github.com/ollama/ollama/blob/678722224b3d8c2a024a4fa5f530d83ce34db120/cmd/launch/claude_desktop.go#L432-L437))
and deletes `autoModeEnabled` with the other gateway keys from the
profile
([796-815](https://github.com/ollama/ollama/blob/678722224b3d8c2a024a4fa5f530d83ce34db120/cmd/launch/claude_desktop.go#L796-L815)).
Each write goes through `WriteWithBackup`
([955-965](https://github.com/ollama/ollama/blob/678722224b3d8c2a024a4fa5f530d83ce34db120/cmd/launch/claude_desktop.go#L955-L965)),
so the profile's modification time is the last time the launcher changed
it. The catalog collects the
`Claude-3p` bases; `AppData/Local/Claude Nest-3p` is not a catalog
line.

### Where the project path is recorded

None for the CLI. The desktop app stores a single `settings.working_dir` in `db.sqlite`
([`database.go:75`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/store/database.go#L75)).
`~/.ollama/history` lines are raw prompts without cwd.

## Parsing

### Transcript stores

| Store | Path | Written by | Unit |
| --- | --- | --- | --- |
| Desktop chat database | macOS `Library/Application Support/Ollama/db.sqlite`, Windows `AppData/Local/Ollama/db.sqlite`, plus `-wal` and `-shm` | the desktop app only ([`internal/onboarding/app_state.go:26-37`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/internal/onboarding/app_state.go#L26-L37)); the store package is built only for Windows and macOS ([`app/store/database.go:1`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/store/database.go#L1)) | `chats` row = conversation, `messages` row = one message |
| REPL prompt history | `.ollama/history` | `ollama run` interactive mode, as the invoking user ([`readline/history.go:40-53`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/readline/history.go#L40-L53)) | one line per entered input, last 100 kept |
| Saved REPL session | a model layer `application/vnd.ollama.image.messages` under the server's `models/blobs` | only on an explicit `/save <model>` in the REPL ([`cmd/interactive.go:254-277`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/interactive.go#L254-L277), [`create/manifest.go:252-263`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/create/manifest.go#L252-L263)) | a JSON array of `api.Message` |
| Launch state | `.ollama/config.json`, `.ollama/backup/`, `.ollama/launch/` | `ollama launch` | configuration only, see [Launch state](#launch-state) |

Linux has no desktop app and therefore no `db.sqlite`. Before `v0.34.2` the store's default
path on any other OS was `.ollama/db.sqlite`, but the package does not
build there. The saved REPL session lives in the models directory of the
server process (for a systemd install,
`/usr/share/ollama/.ollama/models`). Its manifest sits under `models/manifests`, but the message
layer sits with the weights in `models/blobs`, which [Exclusions](#exclusions) excludes, so the
saved messages are a manual follow-up, not a parser input.

#### What survives a delete

A chat deleted in the desktop app is deleted only locally, with a WAL
checkpoint after it
([`ui.go:1415-1439`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/ui/ui.go#L1415-L1439);
[`database.go:823-832`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/store/database.go#L823-L832));
its messages, tool calls and attachments go with the `chats` row through
`ON DELETE CASCADE` ([Record schema](#record-schema)). The REPL prompt
history `.ollama/history` is a separate record that a desktop chat never
reaches, since only the REPL's readline writes it: the last 100 inputs,
slash commands included, with no time, session or working directory. A
REPL session saved with `/save <model>` lives on as a model layer under
the server's `models/blobs`, and an `ollama push` of it leaves only a
`[GIN]` line in the server log. A deletion marker is not stated. The
analyzer reads `.ollama/history` as `user` rows with no timestamp
([Past use and secondary sources](#past-use-rows-and-secondary-sources)); the saved session
layer stays a manual step (above), and the server logs are not read.

#### Built-in agent (v0.31.2 to v0.34.0)

`v0.35.1` has no agent loop of its own: `ollama run` is the REPL above. Older releases had one. Up
to `v0.31.1`, `ollama run --experimental` ran an agent loop with a bash tool that persisted nothing
but the readline history. From `v0.31.2` to `v0.34.0`, a built-in agent with bash and file tools
replaced it; it was removed in commit
[`c16bf9892a560a11c208618968966246e485540c`](https://github.com/ollama/ollama/commit/c16bf9892a560a11c208618968966246e485540c)
(2026-09-11), first absent from `v0.34.1`. At its last state, commit
[`b68b112bd8868d6278250d7d4bdfafa5cbf035c8`](https://github.com/ollama/ollama/commit/b68b112bd8868d6278250d7d4bdfafa5cbf035c8),
its prompt history was rebuilt in memory from the session's messages
([`cmd/tui/chat/input.go:375-383`](https://github.com/ollama/ollama/blob/b68b112bd8868d6278250d7d4bdfafa5cbf035c8/cmd/tui/chat/input.go#L375-L383))
and its only writes were skills ([CLI and server](#cli-and-server)),
a temporary working-directory file for the bash tool
([`agent/tools/bash.go:85`](https://github.com/ollama/ollama/blob/b68b112bd8868d6278250d7d4bdfafa5cbf035c8/agent/tools/bash.go#L85))
and, on an explicit `/save <file>`, the raw request JSON in the current working directory
([`cmd/tui/chat/debug.go:28-48`](https://github.com/ollama/ollama/blob/b68b112bd8868d6278250d7d4bdfafa5cbf035c8/cmd/tui/chat/debug.go#L28-L48)).
There is no session store from those releases to parse. At
[`6787222`](https://github.com/ollama/ollama/commit/678722224b3d8c2a024a4fa5f530d83ce34db120) the REPL's decision mode for yes/no classifier models (`cmd/decide.go`) keeps nothing
on disk beyond the readline history.

### Record schema

#### `db.sqlite`

Created by `database.init`, schema version 19
([`database.go:17`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/store/database.go#L17),[`64-155`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/store/database.go#L64-L155)).
Tables a parser needs:

```sql
CREATE TABLE chats (
  id TEXT PRIMARY KEY, title TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, browser_state TEXT);
CREATE TABLE messages (
  id INTEGER PRIMARY KEY AUTOINCREMENT, chat_id TEXT NOT NULL, role TEXT NOT NULL,
  content TEXT NOT NULL DEFAULT '', thinking TEXT NOT NULL DEFAULT '',
  stream BOOLEAN NOT NULL DEFAULT 0, model_name TEXT,
  model_cloud BOOLEAN, model_ollama_host BOOLEAN,           -- deprecated
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  thinking_time_start TIMESTAMP, thinking_time_end TIMESTAMP, tool_result TEXT,
  FOREIGN KEY (chat_id) REFERENCES chats(id) ON DELETE CASCADE);
CREATE TABLE tool_calls (
  id INTEGER PRIMARY KEY AUTOINCREMENT, message_id INTEGER NOT NULL, type TEXT NOT NULL,
  function_name TEXT NOT NULL, function_arguments TEXT NOT NULL, function_result TEXT,
  FOREIGN KEY (message_id) REFERENCES messages(id) ON DELETE CASCADE);
CREATE TABLE attachments (
  id INTEGER PRIMARY KEY AUTOINCREMENT, message_id INTEGER NOT NULL,
  filename TEXT NOT NULL, data BLOB NOT NULL,
  FOREIGN KEY (message_id) REFERENCES messages(id) ON DELETE CASCADE);
CREATE TABLE users (name TEXT NOT NULL DEFAULT '', email TEXT NOT NULL DEFAULT '',
  plan TEXT NOT NULL DEFAULT '', cached_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP);
```

`settings` is a single row (`id = 1`) holding `device_id`, `working_dir`, `selected_model`,
`schema_version`, `onboarding_version`, `claude_desktop_used`, `codex_desktop_used` and UI
preferences
([`database.go:65-95`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/store/database.go#L65-L95),
[`86`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/store/database.go#L86),
[`92-93`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/store/database.go#L92-L93));
the two `*_used` booleans are described under [Launch configuration](#launch-configuration).
Migrations 16 to 19 add only `onboarding_version`, `claude_desktop_used` and `codex_desktop_used` to
`settings`
([`database.go:563-604`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/store/database.go#L563-L604));
a database last opened by a release before `v0.34.0` has a lower `schema_version` and lacks those
columns, with the same transcript tables. A parser should not require any `settings` column.

- `chats.id`: a UUIDv7 string created when the UI posts to chat `new`
  ([`app/ui/ui.go:717-726`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/ui/ui.go#L717-L726)).
  `chats.title` is set by the UI
  ([`ui.go:1404`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/ui/ui.go#L1404)).
  `browser_state` is JSON for the built-in browser tool's pages.
- `messages.role`: `user`, `assistant` or `tool`; `system` is accepted when
  replayed but the UI does not create one
  ([`ui.go:805`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/ui/ui.go#L805),[`836`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/ui/ui.go#L836),[`1113`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/ui/ui.go#L1113),[`1173`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/ui/ui.go#L1173),[`1889-1918`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/ui/ui.go#L1889-L1918)).
- `messages.content`: the text. For a `tool` row it is the text sent back to the model, falling back
  to the JSON result
  ([`ui.go:1162-1175`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/ui/ui.go#L1162-L1175));
  a failed tool writes `Error: <err>`
  ([`ui.go:1111-1118`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/ui/ui.go#L1111-L1118)).
- `messages.thinking`: reasoning text accumulated from the stream
  ([`ui.go:1268`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/ui/ui.go#L1268)),
  with `thinking_time_start`/`thinking_time_end`.
- `messages.model_name`: the model chosen in the UI, NULL when empty
  ([`database.go:1044-1047`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/store/database.go#L1044-L1047)).
- `messages.tool_result`: JSON of the tool's structured result on `tool` rows, or the JSON `null`
  when the browser tool ran
  ([`ui.go:1140-1161`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/ui/ui.go#L1140-L1161);
  marshalled at
  [`database.go:1049-1056`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/store/database.go#L1049-L1056)).
- `tool_calls`: one row per call on the `assistant` message that requested it. `type` is `function`,
  `function_arguments` is the arguments as a JSON string
  ([`ui.go:1083-1092`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/ui/ui.go#L1083-L1092);
  [`store.go:90-99`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/store/store.go#L90-L99)).
  `function_result` is written only when `Function.Result` is set
  ([`database.go:1187-1211`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/store/database.go#L1187-L1211)),
  which the UI never does, so expect NULL.
- The tools the desktop app can run are web search, web fetch and the browser tools
  (`browser.search`, `browser.open`, `browser.find`)
  ([`ui.go:946-952`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/ui/ui.go#L946-L952));
  there is no shell or file tool.
- `attachments`: user-supplied files as BLOBs on the `user` message
  ([`database.go:1178-1185`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/store/database.go#L1178-L1185)).

#### `.ollama/history`

Plain text, one entry per line, no header, no timestamps, no working directory, no escaping. `Save`
writes every buffered entry with `fmt.Fprintln` to `history.tmp` and renames it over the file
([`history.go:125-150`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/readline/history.go#L125-L150));
only the last 100 entries are kept
([`history.go:24-31`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/readline/history.go#L24-L31),[`92-99`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/readline/history.go#L92-L99)).
Each line submitted at the prompt is added, including slash commands such as `/bye` and `/set`
([`readline/readline.go:317-325`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/readline/readline.go#L317-L325)).
A pasted multi-line block is joined with `\n` into one entry
([`readline.go:319-322`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/readline/readline.go#L319-L322))
and so is written as several physical lines; a `"""` block arrives line by line from the prompt, so
each of its lines is a separate entry
([`cmd/interactive.go:171-199`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/interactive.go#L171-L199)).
On load, lines are trimmed and empty ones skipped
([`history.go:62-78`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/readline/history.go#L62-L78)),
so the file cannot tell a multi-line prompt from several prompts. `OLLAMA_NOHISTORY` or
`/set nohistory` stops saving
([`envconfig/config.go:223-224`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/envconfig/config.go#L223-L224);
[`cmd/interactive.go:128-130`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/interactive.go#L128-L130),[`288-295`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/interactive.go#L288-L295)).
The history is the invoking user's, written from `os.UserHomeDir()`, whatever account runs the
server. `readline/history.go` and `readline/readline.go` are unchanged since `v0.30.7`. On the
Linux install of [Observed](#observed) the file held five entries, four of them slash commands.

#### Tool calls and results

There is no call id. An `assistant` message carries one or more `tool_calls` rows (ordered by
`tool_calls.id`); the results follow as `role = 'tool'` messages in the same order. The tool name of
a `tool` row is not stored (`Message.ToolName` has no column,
[`store.go:44`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/store/store.go#L44)
against
[`database.go:1030-1034`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/store/database.go#L1030-L1034));
the UI recovers it on reload from the last call of the nearest earlier assistant message
([`ui.go:1343-1358`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/ui/ui.go#L1343-L1358)).
A parser should pair the n-th `tool` message after an assistant message with that message's n-th
`tool_calls` row and synthesise `tool_use_id` as `<chat_id>:<tool_calls.id>`.

Thinking: `messages.thinking`, non-empty only for thinking models; emit as
`thinking` rows only with `--include-thinking`.

### Joins and timestamps

#### Session and project attribution

- Session: `messages.chat_id = chats.id`.
- Project: none per chat. `settings.working_dir` is one global value
  ([`store.go:147-148`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/store/store.go#L147-L148))
  and the desktop tools do not use the file system; leave `project_path` empty.
- Model: `messages.model_name` on `assistant` rows.
- Git branch: none.
- `.ollama/history`: no session, no project, no model.
- Tool call to result: by position, no call id
  ([Tool calls and results](#tool-calls-and-results)).

#### Timestamps

- `db.sqlite`: `created_at`, `updated_at`, `thinking_time_*` and `chats.created_at` are bound from
  Go `time.Time` values made with `time.Now()`
  ([`store.go:65-73`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/store/store.go#L65-L73),[`115-122`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/store/store.go#L115-L122)).
  go-sqlite3 binds a `time.Time` as text in its first format, `2006-01-02 15:04:05.999999999-07:00`
  ([`sqlite3.go:228-232`](https://github.com/mattn/go-sqlite3/blob/846fea6c1443e8cc366fc1966fe078d7f825f6a9/sqlite3.go#L228-L232),[`1979-1981`](https://github.com/mattn/go-sqlite3/blob/846fea6c1443e8cc366fc1966fe078d7f825f6a9/sqlite3.go#L1979-L1981)):
  local time with a numeric offset, fractional seconds with trailing zeros trimmed, for example
  `2026-03-01 09:00:00.123456-08:00`. Convert with the offset. A `CURRENT_TIMESTAMP` default
  (`YYYY-MM-DD HH:MM:SS`, UTC) would only appear for a row inserted without a value, which the code
  never does.
- Order messages by `messages.id`. `saveChat` deletes and re-inserts every message of a chat on each
  save
  ([`database.go:789-809`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/store/database.go#L789-L809)),
  so `id` values are not stable across saves but the order is preserved; the `created_at` values are
  carried over from the in-memory message.
- `.ollama/history`: none. File mtime is the time of the last entry only.

### Other stores

The stores beside the desktop database and the REPL history are the server's logs and the
launch configuration, which also answer whether Ollama repeats launched agents' turns.

#### Overlap with launched agents

Question: if `ollama launch claude|opencode|hermes|openclaw --model <local
model>` was used, would an Ollama parser repeat rows the `claude-code`,
`opencode`, `hermes` or `openclaw` parsers already produce? **No.**
Ollama keeps no copy of what a launched agent sends or receives; the
agent's own transcript is the only record, and an Ollama parser reading
the stores in [Transcript stores](#transcript-stores) cannot produce those turns. The same holds for
the integrations added since `v0.30.7`, `chatgpt` (the Codex desktop app),
`muse` and `dsh`; the subsections below take each store in turn.

The launched agent's own transcript is where the model shows: Claude Code
`message.model`, OpenCode `providerID/modelID` (provider `ollama`), Hermes
`sessions.model`, OpenClaw `responseModel`
(see [claude-code.md](claude-code.md), [opencode.md](opencode.md),
[hermes.md](hermes.md), [openclaw.md](openclaw.md); for `chatgpt` and
`muse`, [codex-cli.md](codex-cli.md) and [muse-code.md](muse-code.md)); the
requests under [Server logs](#server-logs) confirm the server echoes the requested local
model name.

Bottom line: an Ollama parser built on [Parser plan](#parser-plan) never duplicates
launched-agent rows. The only Ollama files that touch launched sessions
are configuration (attribution of integration to model) and a
metadata-only proxy log for the ChatGPT integration. The exceptions are
opt-in and outside the parser's inputs: `OLLAMA_DEBUG_LOG_REQUESTS`
request bodies in a temp directory and `OLLAMA_DEBUG=2` server logs would
repeat prompt or output content, and a parser should not read them.

#### Server logs

The API handlers persist nothing under any home. The routes that `ollama launch` points agents at,
`/api/chat`, `/api/generate`, `/v1/chat/completions`, `/v1/completions`, `/v1/responses` and
`/v1/messages`, all end in `ChatHandler` or `GenerateHandler`
([`server/routes.go:2072-2092`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/server/routes.go#L2072-L2092)),
as do `/v1/responses/compact` and `/v1/systemone`, which are new since `v0.30.7` and write no file
either. None of the server, `middleware`, `openai` or `anthropic` packages imports the desktop
`app/store` package (its only importers are under `app/`), so the server never writes `db.sqlite`;
`.ollama/history` is written only by the REPL's readline ([Record schema](#record-schema)). The
server's file writes outside the models directory are the model-recommendation cache, the
[Codex proxy log](#codex-proxy-log) and, only when `OLLAMA_DEBUG_LOG_REQUESTS` is set, request
bodies in a fresh temporary directory `ollama-request-logs-*`
([`server/inference_request_log.go:25`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/server/inference_request_log.go#L25),[`33-47`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/server/inference_request_log.go#L33-L47),[`104-123`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/server/inference_request_log.go#L104-L123)),
outside any home. What does reach a log is the server's stdout and stderr: `gin.Default()`
([`routes.go:2030`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/server/routes.go#L2030))
adds gin's request logger, which writes one
`[GIN] <local time> | <status> | <latency> | <client IP> | <method> "<path>"` line per request
([`gin.go:221-226`](https://github.com/gin-gonic/gin/blob/75ccf94d605a05fe24817fc2f166f6f2959d5cea/gin.go#L221-L226),
[`logger.go:141-161`](https://github.com/gin-gonic/gin/blob/75ccf94d605a05fe24817fc2f166f6f2959d5cea/logger.go#L141-L161));
gin runs in debug mode unless `GIN_MODE` says otherwise
([`routes.go:96`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/server/routes.go#L96),[`106-115`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/server/routes.go#L106-L115)).
Only at `OLLAMA_DEBUG=2` (trace,
[`envconfig/config.go:199-212`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/envconfig/config.go#L199-L212))
does the server log model output text
([`routes.go:2987`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/server/routes.go#L2987),[`3004`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/server/routes.go#L3004))
and `/v1/messages` responses
([`middleware/anthropic.go:64`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/middleware/anthropic.go#L64),[`75`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/middleware/anthropic.go#L75)).
For a systemd install that output goes to the journal; for the macOS desktop app it goes to
`.ollama/logs/server.log`
([`app/server/server_unix.go:20`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/server/server_unix.go#L20),
[`app/server/server.go:242`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/server/server.go#L242),[`288-294`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/server/server.go#L288-L294)).
That log is not a transcript and is not parsed.

On the Linux install of [Observed](#observed) (0.35.1, and 0.30.7 before it), a marker file was
created and one request each was sent to `/api/chat`, `/v1/chat/completions` and `/v1/messages`
with a unique marker string in the prompt. Afterwards nothing under `~/.ollama` or
`/usr/share/ollama` was newer than the marker file, and nothing new under `/tmp` came from Ollama.
The marker string was not found in any readable file under those three trees, nor in
`journalctl -u ollama`, which held only the three `[GIN]` request lines and llama-server slot and
sampler lines without prompt text. All three responses named the requested local model.

#### Codex proxy log

`codex-proxy.log` is metadata only. It exists since `v0.34.0`. The Codex desktop proxy, mounted at
`/api/codex/*` on the server's own listener
([`routes.go:1997`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/server/routes.go#L1997),[`2045`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/server/routes.go#L2045)),
writes to `.ollama/logs/codex-proxy.log` under the home of the *server* process
([`server/codex_proxy.go:19-33`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/server/codex_proxy.go#L19-L33)),
so for a systemd install it lands in `/usr/share/ollama/.ollama/logs`, not the user's home. Its only
writer is `logActivity`, one line per request:
`<RFC 3339 local time> route=<openai|chatgpt|ollama|none> model="<name>" method=<m> path=<p> status=<n> duration=<d> result=<ok|canceled|...>`
([`internal/proxy/codex_desktop.go:481-516`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/internal/proxy/codex_desktop.go#L481-L516)).
No prompt, completion or tool call is written. It does not repeat Codex session content. A parser
may skip it; if read, one `system` row per line (`codex proxy: <route> <model> <status> <result>`)
is the most it supports, with no session id. The Claude Desktop gateway in the same package writes
no file.

#### Launch state

`launch/` and `backup/` hold configuration, not conversations.
`.ollama/launch/` holds:

- `codex-app-restore.json`: the Codex app's previous `profile`, `model`,
  `model_provider`, `model_catalog_json`, `openai_base_url` and desktop
  reasoning-effort values, each with a `had_*` flag
  ([`cmd/launch/codex_app.go:2476-2489`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/codex_app.go#L2476-L2489),[`2617-2627`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/codex_app.go#L2617-L2627),[`2648-2654`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/codex_app.go#L2648-L2654)).
  `v0.30.7` wrote the first four pairs only.
- `muse-config/muse/settings.json` (since `v0.32.11`): a private
  `XDG_CONFIG_HOME` for Muse that holds provider settings while Muse's
  sessions stay in its own data directory
  ([`muse.go:24-34`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/muse.go#L24-L34),[`211-219`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/muse.go#L211-L219)).
- `dsh/` (since `v0.32.11`): DeepSeek Harness settings only, its sessions
  untouched
  ([`deepseek_harness.go:33-36`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/deepseek_harness.go#L33-L36),[`498-504`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/deepseek_harness.go#L498-L504)).
- `chatgpt-session-start` (since `v0.34.0`): a single RFC 3339 UTC time,
  see [Launch configuration](#launch-configuration)
  ([`codex_app_profile.go:28-33`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/codex_app_profile.go#L28-L33),[`81-87`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/codex_app_profile.go#L81-L87)).

On macOS, the desktop app's ChatGPT integration
([`app/cmd/app/codex_app_darwin.go:201`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/cmd/app/codex_app_darwin.go#L201),
[`codex_app.go:700-722`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/codex_app.go#L700-L722))
also makes `.ollama/chatgpt-ollama/` with an `electron-data/` directory and, when it finds a
matching process, a `chatgpt.pid` file
([`codex_app.go:38-39`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/codex_app.go#L38-L39),[`916-938`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/codex_app.go#L916-L938),[`1909-1926`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/codex_app.go#L1909-L1926),[`1976-1988`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/codex_app.go#L1976-L1988)).
`v0.35.1` uses them only to detect and stop a ChatGPT process started with that profile by an
earlier build
([`codex_app.go:792-800`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/codex_app.go#L792-L800),[`1953-1965`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/codex_app.go#L1953-L1965));
no release from `v0.34.0` to `v0.35.1` starts ChatGPT with `--user-data-dir` pointing there, so the
directory should be empty. If an `electron-data/` with content is found, it is a ChatGPT Electron
profile and may hold that app's state; it is not an Ollama transcript.

`backup/` holds the previous version of every file `ollama launch` (or the config writer) overwrote:
`backup/<integration>/<file name>.<unix seconds>`, or `backup/<file name>.<unix seconds>` when no
integration is given (Ollama's own `config.json`, Codex `config.toml`), five per file
([`cmd/internal/fileutil/files.go:17-19`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/internal/fileutil/files.go#L17-L19),[`46-71`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/internal/fileutil/files.go#L46-L71),[`76-94`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/internal/fileutil/files.go#L76-L94)).
The ChatGPT integration still backs up under `backup/codex-app/`. The backed-up files are the
agents' configs: Hermes `config.yaml`
([`hermes.go:268-311`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/hermes.go#L268-L311)),
OpenClaw `openclaw.json`
([`openclaw.go:652-663`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/openclaw.go#L652-L663),[`747`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/openclaw.go#L747)),
OpenCode's `.local/state/opencode/model.json` recent-model list
([`opencode.go:208-214`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/opencode.go#L208-L214),[`216-280`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/opencode.go#L216-L280)).
They can hold API keys from those configs, but no conversation content, session id or working
directory. Claude Code gets nothing on disk: it is started with `--model <model>` and
`ANTHROPIC_BASE_URL`, `ANTHROPIC_AUTH_TOKEN=ollama` and the `ANTHROPIC_DEFAULT_*_MODEL` variables in
its environment
([`claude.go:20-27`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/claude.go#L20-L27),[`52-87`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/claude.go#L52-L87)).
One launch edits another agent's session state: for OpenClaw, `clearSessionModelOverride` rewrites
`.openclaw/agents/main/sessions/sessions.json` without a backup, setting each session's `model` to
the launched model and dropping `modelOverride`/`providerOverride`
([`openclaw.go:757-792`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/openclaw.go#L757-L792)),
unchanged since `v0.30.7`. That changes OpenClaw's own evidence (the session index's model field),
not Ollama's.

#### Launch configuration

There is no per-launch record. `config.json` holds
`integrations{<name>: {models[], aliases{}, onboarded, automode}}`, `last_model`, `last_selection`
and, on Linux, `onboarding_version`, which records that the welcome flow was completed, with no
time field
([`cmd/config/config.go:19-34`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/config/config.go#L19-L34),
[`36-81`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/config/config.go#L36-L81)).
On the Linux install of [Observed](#observed), `config.json` had `integrations` (empty),
`last_model` and `last_selection`, and no `onboarding_version` key;
`backup/config.json.<epoch>` had `integrations` and `last_selection`.
A `codex-app` entry from an older release is copied to `chatgpt` on the next load
([`launch.go:1525-1555`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/launch.go#L1525-L1555)).
The file is rewritten only when something changes: `SaveIntegration` when the model chosen for an
integration differs from the saved one
([`launch.go:734-739`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/launch.go#L734-L739)),
`SetLastModel` when the REPL model changes
([`launch.go:717-720`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/launch.go#L717-L720)),
`SetLastSelection` from the menu
([`cmd/cmd.go:2324-2327`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/cmd.go#L2324-L2327)),
and `WriteWithBackup` skips a write whose bytes are unchanged
([`files.go:84-87`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/internal/fileutil/files.go#L84-L87)).
So the file says which integrations were configured with which local model, the
`backup/config.json.<epoch>` names give the times the configuration changed (the epoch is when the
old version was replaced), and the mtime gives the last change. Repeated launches with the same
model leave no trace in Ollama's files. Three newer markers add a little:

- `launch/chatgpt-session-start` is rewritten each time the macOS desktop app applies Ollama models
  to ChatGPT, not on a CLI `ollama launch chatgpt`
  ([`codex_app.go:653-662`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/codex_app.go#L653-L662),[`700-722`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/codex_app.go#L700-L722)).
  Ollama then counts prompts by reading, never writing, Codex's `sessions/*.jsonl` newer than that
  time
  ([`codex_app_profile.go:54-79`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/codex_app_profile.go#L54-L79),[`111-142`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/launch/codex_app_profile.go#L111-L142)).
- `settings.claude_desktop_used` and `settings.codex_desktop_used` in `db.sqlite` are set once
  Claude Desktop or ChatGPT has been connected through the desktop app
  ([`store.go:180-185`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/store/store.go#L180-L185)),
  booleans without a time.
- On macOS and Windows, `.ollama/onboarding-v1.completed` is an empty file
  created once (mode 0600, `O_EXCL`, never rewritten), so its mtime is when onboarding was first
  completed in the app or CLI
  ([`internal/onboarding/state.go:12-29`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/internal/onboarding/state.go#L12-L29),[`50-75`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/internal/onboarding/state.go#L50-L75);
  [`cmd/config/config.go:36-60`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/config/config.go#L36-L60)).

### Format versions

What a reader with an older install sees differently, by the release
that introduced the change (each is described at `v0.35.1` in the section
that covers it):

- `v0.31.2` to `v0.34.0` shipped a built-in agent that replaced the
  `ollama run --experimental` loop and was removed before `v0.34.1`
  ([Transcript stores](#transcript-stores)). It kept no transcript.
- `v0.32.0` renamed the `codex-app` integration to `chatgpt`; `v0.32.11`
  added `muse` and `dsh` (DeepSeek Harness); `v0.33.0` added the
  per-integration `automode` key; `v0.34.0` added the Codex desktop proxy
  and its `logs/codex-proxy.log`; `v0.34.2` added the onboarding marker
  ([Other stores](#other-stores)).
- The desktop database went from schema version 16 to 17 in `v0.32.15`,
  18 in `v0.33.0` and 19 in `v0.34.0`. Each step adds one `settings`
  column; the transcript tables did not change ([Record schema](#record-schema)).
- The saved REPL session layer moved from `server/create.go` to
  `create/manifest.go`, same media type and content.
- From `v0.35.1` to
  [`6787222`](https://github.com/ollama/ollama/commit/678722224b3d8c2a024a4fa5f530d83ce34db120) `app/store/`, `readline/` and `internal/onboarding/` did not change, and `app/ui/ui.go`'s
  diff touches only an error message and a request header, not the chat writes; models gained
  `models/manifests-v2/` ([Per-user storage](#per-user-storage)).

### Secrets

Sensitive columns: `users.name`, `users.email` and `users.plan` identify the Ollama account
([`database.go:1383-1397`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/store/database.go#L1383-L1397));
`attachments.data` holds whole user files; `settings.device_id` is a stable device identifier. The
parser must not emit any of them in full: an attachment becomes its `filename` and byte length, and
the `users` table is not read.

Sign-in (`ollama signin`, or the app's connect page) binds the device's public key to the ollama.com
account in the browser and writes no token
([`cmd/cmd.go:1012-1030`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/cmd/cmd.go#L1012-L1030);
[`server/routes.go:248-257`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/server/routes.go#L248-L257)).
The desktop app caches the account at each start in the `users` row (`name`, `email`, `plan`,
`cached_at`)
([`app/cmd/app/app.go:345`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/cmd/app/app.go#L345);
[`ui.go:464-492`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/ui/ui.go#L464-L492)),
and nothing outside tests calls `ClearUser`
([`app/store/store.go:584-590`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/store/store.go#L584-L590)),
so the row survives a sign-out
([`routes.go:2407-2438`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/server/routes.go#L2407-L2438)).
That is account state, not sync state; the parser does not read it (above).

### Parser plan

Globs (relative to the home): `Library/Application Support/Ollama/db.sqlite`,
`AppData/Local/Ollama/db.sqlite` (with their `-wal`; open through
`sqlite_util.py`), `.ollama/history`, `.ollama/config.json`, and
`configLibrary/00000000-0000-4000-8000-000000000114.json` under
`Library/Application Support/Claude-3p` and `AppData/Local/Claude-3p`, the Claude Desktop
profile, which the catalog attributes to `claude-desktop` and the parser reads through
`reads_agents` ([Permission rows](#permission-rows)). Do not read `.ollama/logs/*`,
`.ollama/backup/**`, `.ollama/launch/**`,
`.ollama/chatgpt-ollama/**` or `.ollama/onboarding-v*.completed` for rows.

`db.sqlite`: join `messages` to `chats` on `chat_id`, order by `chat_id`,
`messages.id`. One row per message, plus one `tool_use` row per
`tool_calls` row and one `thinking` row where `thinking` is non-empty.
Read schema versions 16 to 19 the same way.

| Column | Source |
| --- | --- |
| timestamp_utc | `messages.created_at` parsed with its offset (`YYYY-MM-DD HH:MM:SS[.f]±HH:MM`), converted to UTC; `tool_use` rows take their message's time |
| session_id | `chats.id` |
| project_path | empty |
| git_branch | empty |
| turn_type | `user` → user; `assistant` with content → assistant; each `tool_calls` row → tool_use; `tool` → tool_result; non-empty `thinking` → thinking (opt-in); `system` → system |
| model | `messages.model_name` on assistant, thinking and tool_use rows |
| tool_name | `tool_calls.function_name`; on tool_result rows, the paired call's name |
| tool_use_id | `<chat_id>:<tool_calls.id>`, paired by position ([Tool calls and results](#tool-calls-and-results)) |
| text | `content`; for tool_use the `url` or `query` argument when present, else `function_arguments`; for user rows with attachments, append `[attachment: <filename>, <n> bytes]` per row, never the BLOB |
| source_line | `messages.id` (or `tool_calls.id` for tool_use rows) |

An assistant message with empty `content`, no thinking and no tool calls (the placeholder written
while the model is being pulled,
[`ui.go:832-841`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/ui/ui.go#L832-L841))
yields no row. A chat with `title` yields nothing extra; the title is not a turn.

#### Permission rows

The desktop app asks before no tool call and the REPL has no tools, so there is no per-call
decision and no per-call row. The standing setting is `.ollama/config.json`
`integrations.<name>.automode`, a boolean
([`cmd/config/config.go:19-24`](https://github.com/ollama/ollama/blob/678722224b3d8c2a024a4fa5f530d83ce34db120/cmd/config/config.go#L19-L24)).
For `claude-desktop` it becomes `autoModeEnabled` in the Claude Desktop profile Ollama writes, true
when the key is unset
([`cmd/launch/claude_desktop.go:320-345`](https://github.com/ollama/ollama/blob/678722224b3d8c2a024a4fa5f530d83ce34db120/cmd/launch/claude_desktop.go#L320-L345),
[741-759](https://github.com/ollama/ollama/blob/678722224b3d8c2a024a4fa5f530d83ce34db120/cmd/launch/claude_desktop.go#L741-L759));
who writes each is under [Claude Desktop third-party mode](#claude-desktop-third-party-mode).
The parser yields, source-derived; not observed on any real install, one `system` row per key,
at the file's modification time, with empty `session_id`:

- `.ollama/config.json`, for each `integrations.<name>` object that holds `automode`:
  `permission setting: integrations.<name>.automode=<true|false> (from .ollama/config.json, file mtime)`.
  An absent key, which the launcher reads as true, gives no row.
- The Claude Desktop profile, when it holds `autoModeEnabled`:
  `permission setting: autoModeEnabled=<true|false> (set by Ollama's Claude Desktop launcher, from <path relative to the home>, file mtime)`.
  A profile that `--restore` stripped gives no row.

A file of either kind that is not a JSON object becomes one `system` row
`parser: <path relative to the home> is not a JSON object` (or `is not readable JSON (<error>)`).
The rows say what Ollama configured for the other agent; that agent's own parser reads what it
did with it. The profile's other keys (`coworkEgressAllowedHosts`, the telemetry switches) are not
read for rows.

#### Model rows

Model rows ([issue 96](https://github.com/seanthegeek/doubleagent/issues/96)), `system`:
`model: <model_name>` before the rows of a chat's first assistant message that names a model, and
`model: <old> -> <new>` before the rows of each later assistant message
whose `model_name` differs, with the message's time and `source_line`.
`config.json` `last_model` is not read for rows.
`last_model` and each integration's `models[]` are configuration, not use.

#### Past-use rows and secondary sources

`.ollama/history`: one `user` row per non-empty line, `timestamp_utc` and
`session_id` empty, `source_line` the line number, text the line. Lines
starting with `/` become `system` rows labelled `slash command:`. The
rows carry no time, so they sort last and drop out of any time-window
filter; the parser docstring must say that the file keeps only the last
100 entries and splits pasted multi-line prompts across lines.
The saved REPL session layer stays a manual step ([Transcript stores](#transcript-stores)).

#### Sync, relay and origin rows

Sync: Ollama records no sync state, because it has no session sync. No release keeps a server copy
of a chat: the desktop app's chat routes (list, get, post, delete, create, rename) are served from
`db.sqlite` by the app's own local handler
([`app/ui/ui.go:286-291`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/ui/ui.go#L286-L291)),
and a chat turn goes to the local server through `inferenceClient`
([`ui.go:431-433`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/ui/ui.go#L431-L433),[`827`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/ui/ui.go#L827),[`1001`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/ui/ui.go#L1001)).
What leaves the host is the model call. A cloud model is one whose name ends in `:cloud` or `-cloud`
([`internal/modelref/modelref.go:98-115`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/internal/modelref/modelref.go#L98-L115));
for it the local server forwards the whole request, every message of the conversation, to
`https://ollama.com:443` signed with the device key `.ollama/id_ed25519`
([`server/routes.go:2656-2664`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/server/routes.go#L2656-L2664);
[`server/cloud_proxy.go:29`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/server/cloud_proxy.go#L29),[`180-220`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/server/cloud_proxy.go#L180-L220),[`372-386`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/server/cloud_proxy.go#L372-L386)),
the same for the REPL, `ollama launch` agents and the desktop app. "Ollama Turbo" is the earlier
name of that cloud service; its trace is the `settings.turbo_enabled` column
([`app/store/database.go:375-380`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/store/database.go#L375-L380)),
which the app now reads only to move a Turbo user's selected model to its cloud name and then sets
to false
([`app/ui/app/src/hooks/useSelectedModel.ts:63-74`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/ui/app/src/hooks/useSelectedModel.ts#L63-L74),[`119-122`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/ui/app/src/hooks/useSelectedModel.ts#L119-L122)).
Ollama's documentation says cloud prompts and responses are processed and not stored or logged
([`docs/faq.mdx:161`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/docs/faq.mdx#L161),
[`docs/cloud.mdx:90`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/docs/cloud.mdx#L90));
the server side is not in the repository, so that is the vendor's statement, not a source finding.
The local evidence of cloud use is the model name: `messages.model_name` with the `:cloud` or
`-cloud` suffix, which the `model:` rows already carry; the `model_cloud` column is deprecated and
never written
([`database.go:115`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/store/database.go#L115),[`1044-1047`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/store/database.go#L1044-L1047)).
The desktop app's web search and web fetch tools send the query or URL to
`ollama.com/api/web_search` and `/api/web_fetch`
([`app/tools/web_search.go:110`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/tools/web_search.go#L110),
[`app/tools/web_fetch.go:96`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/tools/web_fetch.go#L96));
the `tool_use` rows show them. Cloud use is turned off by `OLLAMA_NO_CLOUD` or
`disable_ollama_cloud: true` in `.ollama/server.json`, the desktop app's cloud toggle
([`envconfig/config.go:237-238`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/envconfig/config.go#L237-L238);
[`app/store/cloud_config.go:15-42`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/store/cloud_config.go#L15-L42)),
on by default. Sign-in writes no token and is account state, not sync state
([Secrets](#secrets)). The anonymous-usage `settings.survey` flag is stored but read by
nothing
([`store.go:132-135`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/app/store/store.go#L132-L135)),
and there is no telemetry or crash-report client in `app/`, `cmd/`, `server/` or `internal/`
(searched `telemetry`, `analytics`, `sentry`, `posthog`, `crash`, `upload`).

One path does upload a transcript, with no local marker: a REPL session saved with `/save <model>`
becomes a model whose `application/vnd.ollama.image.messages` layer holds the conversation
([Transcript stores](#transcript-stores)), and `ollama push` of that model uploads every layer, the
messages layer included, to the registry
([`server/images.go:960-1024`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/server/images.go#L960-L1024)).
Push writes nothing locally; only the server log's `[GIN]` line for `POST "/api/push"` records it,
without the model name. No `session synced:` row is proposed. Relay and origin: the agent records
none.

#### Scratch directory rows

None under the home; the two temporary directories outside it are in
[Scratch directories](#scratch-directories).

#### Deletion rows

No row: a chat deleted in the desktop app is deleted only locally and leaves no marker
([What survives a delete](#what-survives-a-delete)).

#### Bad-line rows

In `.ollama/history`, a line that is not valid UTF-8 becomes, after the file's other rows, one
`system` row `parser: unparseable line (<decode error>): <line>` carrying
the line as read less its terminator, with the same empty fields and its line number, reported
through `error_rows` ([issue 174](https://github.com/seanthegeek/doubleagent/issues/174);
[issue 136](https://github.com/seanthegeek/doubleagent/issues/136) sets the rule). Until that change
the parser decoded the file with `errors="replace"`, so such a line became a `user` row with
replacement characters and no `parser:` row. Such a line has not been seen on any real install:
`Save` writes each entry with `fmt.Fprintln` and checks no encoding
([`history.go:125-150`](https://github.com/ollama/ollama/blob/b0c1ca4f7549d7acdfa52a7dcffc934bc63a43ce/readline/history.go#L125-L150)).
A `db.sqlite` that is not a database becomes one `system` row ([Fixtures](#fixtures)).

#### Fixtures

Fixture `Library/Application Support/Ollama/db.sqlite`:

```sql
CREATE TABLE settings (id INTEGER PRIMARY KEY CHECK (id = 1), device_id TEXT NOT NULL DEFAULT '', working_dir TEXT NOT NULL DEFAULT '', selected_model TEXT NOT NULL DEFAULT '', onboarding_version INTEGER NOT NULL DEFAULT 0, claude_desktop_used BOOLEAN NOT NULL DEFAULT 0, codex_desktop_used BOOLEAN NOT NULL DEFAULT 0, schema_version INTEGER NOT NULL DEFAULT 19);
INSERT INTO settings (id, device_id) VALUES (1, '0190a000-0000-7000-8000-00000000d001');
CREATE TABLE chats (id TEXT PRIMARY KEY, title TEXT NOT NULL DEFAULT '', created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, browser_state TEXT);
CREATE TABLE messages (id INTEGER PRIMARY KEY AUTOINCREMENT, chat_id TEXT NOT NULL, role TEXT NOT NULL, content TEXT NOT NULL DEFAULT '', thinking TEXT NOT NULL DEFAULT '', stream BOOLEAN NOT NULL DEFAULT 0, model_name TEXT, model_cloud BOOLEAN, model_ollama_host BOOLEAN, created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, thinking_time_start TIMESTAMP, thinking_time_end TIMESTAMP, tool_result TEXT);
CREATE TABLE tool_calls (id INTEGER PRIMARY KEY AUTOINCREMENT, message_id INTEGER NOT NULL, type TEXT NOT NULL, function_name TEXT NOT NULL, function_arguments TEXT NOT NULL, function_result TEXT);
CREATE TABLE attachments (id INTEGER PRIMARY KEY AUTOINCREMENT, message_id INTEGER NOT NULL, filename TEXT NOT NULL, data BLOB NOT NULL);
CREATE TABLE users (name TEXT NOT NULL DEFAULT '', email TEXT NOT NULL DEFAULT '', plan TEXT NOT NULL DEFAULT '', cached_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP);
INSERT INTO users VALUES ('Alice Example', 'alice@example.invalid', 'free', '2026-03-01 08:59:00-08:00');
INSERT INTO chats VALUES ('0190a000-0000-7000-8000-00000000c001', 'Release notes', '2026-03-01 09:00:00.5-08:00', NULL);
INSERT INTO messages (id, chat_id, role, content, thinking, model_name, created_at, updated_at, tool_result) VALUES
 (1, '0190a000-0000-7000-8000-00000000c001', 'user', 'summarise https://example.invalid/notes', '', NULL, '2026-03-01 09:00:00.5-08:00', '2026-03-01 09:00:00.5-08:00', NULL),
 (2, '0190a000-0000-7000-8000-00000000c001', 'assistant', '', 'need to fetch the page', 'gemma4:e4b', '2026-03-01 09:00:01.25-08:00', '2026-03-01 09:00:02-08:00', NULL),
 (3, '0190a000-0000-7000-8000-00000000c001', 'tool', 'Release 2.0 adds X.', '', NULL, '2026-03-01 09:00:03-08:00', '2026-03-01 09:00:03-08:00', '{"title":"Notes","content":"Release 2.0 adds X."}'),
 (4, '0190a000-0000-7000-8000-00000000c001', 'assistant', 'Release 2.0 adds X.', '', 'gemma4:e4b', '2026-03-01 09:00:04.125-08:00', '2026-03-01 09:00:05-08:00', NULL);
INSERT INTO tool_calls VALUES (1, 2, 'function', 'web_fetch', '{"url":"https://example.invalid/notes"}', NULL);
INSERT INTO attachments VALUES (1, 1, 'notes.txt', X'68656c6c6f');
```

Expected rows: user (17:00:00.500Z), thinking (opt-in, 17:00:01.250Z),
tool_use `web_fetch` with id `0190a000-0000-7000-8000-00000000c001:1` and
text `https://example.invalid/notes`, tool_result with the same id
(17:00:03.000Z), assistant (17:00:04.125Z, model `gemma4:e4b`). The user
row's text ends with `[attachment: notes.txt, 5 bytes]`; the `users` row
and the BLOB appear nowhere in the output. Truncation case: a
`db.sqlite` that is not a database becomes one `system` row. A second
fixture with the `settings` table at schema version 16 (without the three
newer columns) must give the same rows.

Fixture `.ollama/config.json`, whose `claude-desktop` integration holds `automode` and whose
`claude` integration does not:

```json
{"integrations":{"claude":{"models":["gemma4:e4b"]},"claude-desktop":{"models":["gemma4:e4b"],"automode":true}},"last_model":"gemma4:e4b","last_selection":"claude"}
```

Expected: one row,
`permission setting: integrations.claude-desktop.automode=true (from .ollama/config.json, file mtime)`.

Fixture
`Library/Application Support/Claude-3p/configLibrary/00000000-0000-4000-8000-000000000114.json`,
the keys `writeClaudeDesktopGatewayProfile` sets, as `json.MarshalIndent` writes them, with a
`_meta.json` beside it that the parser must not want:

```json
{
  "autoModeEnabled": true,
  "chatTabEnabled": true,
  "coworkEgressAllowedHosts": [
    "*"
  ],
  "deploymentDisplayName": "Ollama",
  "disableDeploymentModeChooser": true,
  "disableEssentialTelemetry": true,
  "disableNonessentialTelemetry": true,
  "inferenceGatewayApiKey": "ollama",
  "inferenceGatewayAuthScheme": "bearer",
  "inferenceGatewayBaseUrl": "http://127.0.0.1:11435",
  "inferenceProvider": "gateway"
}
```

Expected: one row, `permission setting: autoModeEnabled=true (set by Ollama's Claude Desktop
launcher, from <that path>, file mtime)`. With the key removed from either file, no row.

Fixture `.ollama/history`, with noise beside it that the parser must not want:
`.ollama/backup/config.json.1772355600` and an
empty `.ollama/onboarding-v1.completed`:

```text
why is the sky blue
/set nohistory
/bye
```

#### Source host rows

The agent records no source-host integration. ollama/ollama at
[`6787222`](https://github.com/ollama/ollama/commit/678722224b3d8c2a024a4fa5f530d83ce34db120)
was searched for `github`, `gitlab`, `bitbucket`, `oauth`, `installation`, `repositor`,
`pull request` and `clone` in `app/`, `cmd/` and `server/`: the only sign-in is the ollama.com
device-key binding of [Secrets](#secrets) (`ollama://connect`), and the GitHub names are the
launch registry's links and the Copilot integrations' configuration, not an account Ollama holds.

## Confidence

- High: the paths in [Per-user storage](#per-user-storage) to [Project-local files](#project-local-files), from
  source with line numbers; of them only the Linux CLI and server layout was confirmed on a real
  install ([Observed](#observed)).
- High: the DDL, write paths, timestamp binding, the tool pairing rule, the history format, the
  absence of server-side persistence (source plus the request check on 0.30.7 and 0.35.1), the
  `config.json` and backup shapes, the `codex-proxy.log` line format, and the release in which
  each change since `v0.30.7` appeared (from the tag history).
- Medium: whether the WAL sidecars are present depends on the app's state at collection.
- Medium: that the UI never sets `tool_calls.function_result` (no writer found in `app/`); that
  the built-in agent of `v0.31.2` to `v0.34.0` left no session store (read at its last commit
  only, not at each release).
- Not determined: a real `db.sqlite`, because the desktop app does not exist on Linux; whether the
  macOS app also writes `db.sqlite` into `~/.ollama` on non-standard installs (code says no); what
  fills `chats.browser_state` in detail; whether a Windows build of the app writes `server.log`
  with the same gin lines (the path differs, [Desktop app, Windows](#desktop-app-windows)).
- Not determined: whether Claude Desktop itself rewrites `autoModeEnabled` in the profile Ollama
  wrote (its own settings window edits `configLibrary` entries), so the profile row's "set by
  Ollama's Claude Desktop launcher" rests on Ollama's source alone.
- Not determined: the size of `~/.ollama/launch/*` and `repl` artifacts of other `ollama launch`
  integrations; what a pre-release build may have left in
  `.ollama/chatgpt-ollama/electron-data`.
