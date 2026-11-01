# claude-desktop

Claude Desktop (catalog agent `claude-desktop`) is Anthropic's closed-source Electron app. Its
surfaces are the claude.ai chat (a web page loaded from `https://claude.ai`), the merged Chat/Cowork
surface (Cowork is the app's local agent mode, session type `agent`, `dispatch_child`, `scheduled`,
`radar` or `chat`), the Code tab (Claude Code sessions on the host) and scheduled tasks. Each local
surface runs the Claude Code CLI with the Agent SDK, so the transcripts themselves are Claude Code
JSONL, whose record shape is in [`claude-code.md`](claude-code.md) and is not repeated here; what
the desktop adds is a session record per session, an HMAC-chained `audit.jsonl` per Cowork session,
and some indexes. The Windows install observed is an MSIX package (Anthropic's Windows download, not
a Microsoft Store app; MSIX is the Windows packaged-app format, whose filesystem virtualisation
redirects the app's `%APPDATA%` writes under `%LOCALAPPDATA%\Packages\<package family>`). Two builds
share one layout. "3p" is the docs' name for "Claude Desktop on third-party", the enterprise build
whose inference goes to the customer's own provider (Amazon Bedrock, Google Cloud or an inference
gateway) and whose users need no Anthropic account; it keeps its data in `Claude-3p` directories so
it "can coexist" with the standard build on one machine (data-storage doc, "Where data lives"). The
bundle uses the same tag: the Code tab's `CLAUDE_CODE_ENTRYPOINT` is `claude-desktop-3p` on that
build and session records can be `importedFrom` `sibling-3p` or `sibling-1p`
([Transcript stores](#transcript-stores), [Record schema](#record-schema)). The agent is parsed by
the `claude-desktop` parser ([Parser plan](#parser-plan)).

## Source and evidence

Closed Electron app; no public repository, so there are no commit links. Each claim names its
evidence level:

- **Bundle**: `/Applications/Claude.app`, version `2.31226.1` (macOS, `CFBundleShortVersionString`),
  `Contents/Resources/app.asar` sha256
  `e0f0a70911d2d206f17572e82636c80a15173281db4966a44c2558e8f8ae6bb8`. The archive was copied into
  the scratchpad and unpacked with a short reader of the asar header (pickle sizes, JSON header,
  file data at offset); 471 packed files, of which the main-process code is the minified Vite output
  under `.vite/build/`. Citations are written `app.asar:<path>@<byte offset>` followed by a short
  literal snippet; the chunk file names are content hashes and change with every release, so a
  reader of another version should grep for the snippet, not the offset. Nothing from the bundle was
  executed and the app was not launched.
- **Official docs**:
  [User identity and local data](https://claude.com/docs/third-party/claude-desktop/data-storage)
  (fetched 2026-10-03; the data-storage page, also cited for names the bundle confirms),
  [MCP debugging](https://modelcontextprotocol.io/docs/tools/debugging) (log and config paths),
  [Claude Code IAM](https://code.claude.com/docs/en/iam) and
  [Claude Code settings](https://code.claude.com/docs/en/settings) (relation to `~/.claude`).
- **Community/forum**:
  [aaddrick/claude-desktop-debian README](https://github.com/aaddrick/claude-desktop-debian#readme)
  (Linux path),
  [CodeZeno/Claude-Code-Usage-Monitor issue 105](https://github.com/CodeZeno/Claude-Code-Usage-Monitor/issues/105)
  and [yhm138/ai-quota-tray pull request 6](https://github.com/yhm138/ai-quota-tray/pull/6) (token
  cache keys and encryption),
  [blind0wl/dms-ai-usage issue 8](https://github.com/blind0wl/dms-ai-usage/issues/8) (Linux
  `~/.config/Claude/config.json` keys, unanswered). No DFIR write-up found for this app.
- **Real installs**: the six surface and platform cells in [Observed](#observed).

Every shape below marked **bundle** rests on the code that writes or reads it; shapes marked
**observed** were seen on disk.

### Observed

All three installs are Claude Desktop 2.31226.1, read for file names, sizes, key names and value
types only ([issue 73](https://github.com/seanthegeek/doubleagent/issues/73)).

- macOS, Code tab: the author's install of the standard build (`CFBundleShortVersionString` of
  `/Applications/Claude.app`) on macOS 26.4.1 arm64, 2026-10-11. One Code-tab session on a project
  folder (two turns, one `Write` and one `Bash` call) wrote the record
  `claude-code-sessions/<acct>/<org>/local_<uuid>.json` (compact JSON, 19 KB,
  [Code-tab session record](#code-tab-session-record-claude-code-sessionslocal_uuidjson-bundle-observed-on-windows-linux-and-macos))
  and its transcript `~/.claude/projects/-Users-<user>-proj/<cliSessionId>.jsonl` (`entrypoint`
  `claude-desktop`, CLI 2.1.295, 45 lines). The macOS layout in
  [Standard build on macOS](#standard-build-on-macos) and the excluded sizes in
  [Exclusions](#exclusions) come from this install.
- macOS, Chat/Cowork: the same install and day, on a Mac that can run the Cowork VM. A chat asked to
  write files, run before the Code-tab session, wrote only `remote-session-spaces.json` (146 bytes)
  and no local session record
  ([`remote-session-spaces.json`](#remote-session-spacesjson)).
- Linux, Code tab: the official `claude-desktop` `.deb` (arm64) on Ubuntu 26.04.1 LTS aarch64,
  2026-10-11, in a clean VM with nested virtualisation, after sign-in. One Code-tab session on a
  project folder (two turns, one `Bash` call) wrote the record (compact JSON, 17 KB, the same keys
  as the Windows record) and its transcript
  `~/.claude/projects/-home-<user>-proj/<cliSessionId>.jsonl` (`entrypoint` `claude-desktop`, CLI
  2.1.295), beside the CLI's `~/.claude/sessions/<pid>.json` and `<pid>.<sha256>.key`
  ([Joins and timestamps](#joins-and-timestamps), [Credentials](#credentials)). The layout is in
  [Standard build on Linux](#standard-build-on-linux).
- Linux, Chat/Cowork: the same install and day. A read-only chat wrote nothing under
  `local-agent-mode-sessions/`; a chat asked to write a report ran as a remote Cowork session and
  wrote only `remote-session-spaces.json` locally
  ([`remote-session-spaces.json`](#remote-session-spacesjson)). The Cowork VM was unsupported in
  the nested guest ([Standard build on Linux](#standard-build-on-linux)).
- Windows, Code tab: the MSIX package `Claude_pzs8sxrjxfjjc`, version 2.31226.1.0 per
  `Get-AppxPackage`, on Windows 11 build 26300 arm64, 2026-10-10, in a clean VM. One Code-tab
  session wrote the record (compact JSON, 17 KB) and its transcript
  `~/.claude/projects/C--Users-<user>-proj/<cliSessionId>.jsonl` (`entrypoint` `claude-desktop`, CLI
  2.1.295). The MSIX layout is in [Standard build on Windows](#standard-build-on-windows).
- Windows, Chat/Cowork: the same install and day. One chat persisted no local session record; its
  only write was `local-agent-mode-sessions/<acct>/<org>/remote-session-spaces.json`
  ([`remote-session-spaces.json`](#remote-session-spacesjson)).

### Source reviews

- 2026-10-19, Ollama at
  [`6787222`](https://github.com/ollama/ollama/commit/678722224b3d8c2a024a4fa5f530d83ce34db120):
  added the gateway profile `ollama launch claude-desktop` writes into the 3p build's
  `configLibrary/` and the `Claude Nest` Windows directory names to
  [Enterprise (3p) build](#enterprise-3p-build).

## Collection

### Per-user storage

The standard build's paths (docs MCP debugging; data-storage note that 3p "is separate from standard
Claude Desktop") differ per platform; the enterprise (3p) build keeps its own `Claude-3p`
directories.

#### Scratch directories

`git-shadow/scratch/` holds the Code tab's private git index copies, pruned after six hours; the
Claude Code CLI temp roots that `git-shadow/cli-temp-roots.json` lists (`/tmp/claude-<uid>` on
Linux) are outside every home and not collected ([`git-shadow`](#git-shadow)).

#### Standard build on macOS

- Observed 2026-10-11, 2.31226.1: the data directory is `~/Library/Application Support/Claude/` and
  the logs are in `~/Library/Logs/Claude/`. Root files: `ant-device-registry.json`, `ant-did`,
  `buddy-tokens.json`, `ccd-ids.json`, `claude_desktop_config.json`, `config.json`,
  `cowork-enabled-cli-ops.json`, `extensions-blocklist.json`, `fcache`, `git-worktrees.json`,
  `plan-usage-history.json`, `vm-support-probe.json`, `window-state.json`, and the Chromium files
  `Cookies`, `Cookies-journal`, `declarative_performance_observer.db` (+ `-journal`), `DIPS`,
  `DIPS-wal`, `Local State`, `Network Persistent State`, `Preferences`, `SharedStorage`,
  `TransportSecurity`, `Trust Tokens` (+ `-journal`).
- Directories: `claude-code/`, `claude-code-sessions/`, `claude-code-vm/`,
  `local-agent-mode-sessions/`, `Partitions/`, `spaces-present/`, `vm_bundles/`, `sentry/`, and the
  Chromium `blob_storage`, `Cache`, `Code Cache`, `Crashpad`, `DawnGraphiteCache`,
  `DawnWebGPUCache`, `GPUCache`, `IndexedDB`, `Local Storage`, `Session Storage`,
  `Shared Dictionary`, `WebStorage`. Seen on Linux but absent here: `ca-bundle.pem`,
  `os-entry-points.json`, `git-shadow/` and the `Singleton*` symlinks.
- `claude-code/<version>/<12 hex>/claude.app/` is a macOS app bundle (`Contents/_CodeSignature`,
  `CodeResources`, `Info.plist`, `MacOS`) with a `.verified` sibling (64-byte hex text), two
  versions present (2.1.293 and 2.1.295). `claude-code-vm/<version>/claude` is the Linux CLI build
  that runs inside the Cowork VM, not a VM workspace: 255,516,664 bytes, the size of the linux-arm64
  2.1.295 download of the Linux round, beside `.payload` (103-byte JSON `{sha256, size}`),
  `.verified` (64-byte hex) and `claude-code-vm/.sdk-version` (7-byte text).
- `local-agent-mode-sessions/<acct>/<org>/` holds `scheduled-tasks.json`, `rpm/manifest.json` (the
  remote plugin manifest) and, after a file-writing chat, `remote-session-spaces.json`, beside a
  `skills-plugin/<org>/<acct>/` tree; there is no `local_<uuid>.json` and no `audit.jsonl`.
- `Partitions/` holds two Electron session partitions, each a Chromium profile of its own:
  - `Partitions/launch-preview-static/` (`Cache/`, `Code Cache/`, `Cookies`, `Cookies-journal`,
    `DIPS`, `DIPS-wal`, `InterestGroups`, `Local Storage/leveldb/`, `Network Persistent State`,
    `Preferences`, `PrivateAggregation` (+ `-journal`), `Session Storage/`, `Shared Dictionary/`,
    `SharedStorage`, `Trust Tokens` (+ `-journal`), `WebStorage/`,
    `declarative_performance_observer.db` (+ `-journal`)) is the default partition of the built-in
    browser that previews servers a session launches
    (`app.asar:.vite/build/index.chunk-Cyt2WTko.js@3035166`
    ``var vN="persist:launch-preview-static";function yN(e,t,n){let r=t?"persist:":"";return n?`${r}launch-preview-session-${n}`:e?`${r}launch-preview-${e}`:vN}``,
    opened at `@4638925` `a.session.fromPartition("persist:launch-preview-static")`), so
    `launch-preview-<workspace>` and `launch-preview-session-<id>` partitions can exist beside it
    (not observed); it has its own cookie jar (20 KB observed, [Credentials](#credentials)).
  - `Partitions/cowork-file-preview/` (`Cache/`, `Code Cache/`, `Local Storage/leveldb/`,
    `Network Persistent State`, `Preferences`, `Shared Dictionary/`, `SharedStorage`,
    `Trust Tokens` (+ `-journal`)) renders Cowork file previews from a `cowork-file://preview/`
    scheme with the network closed (`@4461852`
    `var R2r="cowork-file",z2r="chrome-extension://mhjfbmdgcfjbbpaeojofohoefgiehjai",B2r="persist:cowork-file-preview"`,
    `@4462016` `let e=a.session.fromPartition(B2r)`, then `@4462156`
    `var W2r="http://255.255.255.255:9"` set as its proxy).
- `~/Library/Application Support/Claude-3p/` also exists on this standard install (dated
  2026-04-14), holding only `claude_desktop_config.json` (56 bytes, keys `_cfprefsMigrated` boolean
  and `enterpriseConfig` empty object).
- `~/Library/Logs/Claude/` holds `main.log` rotated to `main1.log`-`main4.log` at about 10 MB each,
  `claude.ai-web.log` and `claude.ai-web1.log`, `swift.log`, `cowork_vm_swift.log`, `coworkd.log`,
  `vzgvisor.log`, `cowork_vm_node.log`, `ssh.log`, `chrome-native-host.log`, `main-window.log`,
  `unknown-window.log` and `mcp.log`; the five largest of them were about 59 MB together. Their
  line format is in [Logs](#logs).
- Other macOS locations hold no agent state and are not collected:
  `~/Library/Preferences/com.anthropic.claudefordesktop.plist` (AppKit keys only),
  `~/Library/Caches/com.anthropic.claudefordesktop/` (an NSURLCache `Cache.db` and `fsCachedData/`,
  2.2 MB), `~/Library/Caches/com.anthropic.claudefordesktop.ShipIt/` (the Squirrel.Mac updater's
  `ShipItState.plist` and `ShipIt_stderr.log`) and
  `~/Library/HTTPStorages/com.anthropic.claudefordesktop/httpstorages.sqlite`. The docs also name
  `developer_settings.json` in the data directory, not present on this install.

#### Standard build on Windows

- `%APPDATA%\Claude\` with logs in `%APPDATA%\Claude\logs\` (MCP debugging doc). This is the path
  the app sees and what an unpackaged (Squirrel) install uses; the bundle's full list of candidate
  data directories is in [Windows data directories](#windows-data-directories).
- The MSIX build observed on the real install keeps `userData` in
  `%LOCALAPPDATA%\Packages\Claude_pzs8sxrjxfjjc\LocalCache\Roaming\Claude\`: the app logs its paths
  as `C:\Users\<user>\AppData\Roaming\Claude\...` and MSIX filesystem virtualisation redirects them
  (`main.log`: `[MSIX] Filesystem virtualization active`,
  `user_data_location: 'package_localcache'`), so `%APPDATA%\Claude` does not exist on disk and
  every other process, the collector included, sees only the Packages path. The bundle names a
  second package family, `AnthropicPBC.Claude_fnn82j28hfe8t`
  ([Other stores](#other-stores)), not observed.
- Logs on that install are in `%LOCALAPPDATA%\Claude\logs\` (`main.log`, `mcp.log`, `ssh.log`,
  `cowork_vm_node.log`, `claude.ai-web.log`), not under the data directory, whose own `logs\` is
  empty. `%LOCALAPPDATA%\Claude-Data\` (the bundle's `Claude-Data\<family, lowercased>` candidate)
  and `%LOCALAPPDATA%\Claude-3p\` exist and are empty.
- The data directory holds `ant-did`, `ant-device-registry.json`, `buddy-tokens.json`,
  `ccd-ids.json`, `claude_desktop_config.json`, `config.json`, `cowork-enabled-cli-ops.json`,
  `DIPS`, `fcache`, `git-worktrees.json`, `Local State`, `lockfile`, `plan-usage-history.json`,
  `Preferences`, `window-state.json`, and the directories `claude-code\<version>\<hash>\claude.exe`
  (247 MB), `claude-code-sessions\`, `local-agent-mode-sessions\`, `spaces-present\`, `IndexedDB\`,
  `Local Storage\`, `Session Storage\`, `WebStorage\`, `logs\` and the Chromium caches. There is no
  `vm-support-probe.json`.

#### Standard build on Linux

- Official `.deb`, observed 2026-10-11 (`dpkg -l` "Desktop application for Claude.ai"): the package
  installs to `/usr/lib/claude-desktop/` (the Electron binary, `resources/app.asar`,
  `resources/cowork-linux-helper`, `resources/chrome-native-host`) and `/usr/bin/claude-desktop`.
- The data directory is exactly `~/.config/Claude/` (the XDG config dir), and the logs are inside
  it in `logs/` (`main.log`, `mcp.log`, `ssh.log`, `cowork_vm_node.log`, `claude.ai-web.log`);
  `main.log` records `user_data_location: null` and
  `platform: 'linux'`. Nothing was created under `~/.cache`, `~/.local/share` or `~/.local/state`,
  `~/.config/Claude-3p` does not exist, and there is no `~/Claude` or `~/Documents/Claude`.
- Root files after sign-in: `ant-did`, `ant-device-registry.json`, `ca-bundle.pem` (a 731 KB CA
  bundle), `claude_desktop_config.json`, `config.json`, `cowork-enabled-cli-ops.json`,
  `extensions-blocklist.json`, `fcache`, `git-worktrees.json`, `os-entry-points.json`,
  `plan-usage-history.json`, `window-state.json`, the Chromium files (`Cookies`, `Cookies-journal`,
  `DIPS`, `Local State`, `Network Persistent State`, `Preferences`, `Trust Tokens`,
  `declarative_performance_observer.db`) and the symlinks `SingletonCookie`, `SingletonLock`,
  `SingletonSocket` (the last two point at `<host>-<pid>` and a `/tmp/scoped_dir*` socket); after a
  Code session also `buddy-tokens.json`, `ccd-ids.json` and `TransportSecurity`.
- Directories: `claude-code/<version>/<hash>/` with the Linux CLI `claude` (255,516,664 bytes,
  downloaded on first launch from `downloads.claude.ai`, per `main.log`), `.payload` and
  `.verified`; `claude-code-sessions/<acct>/<org>/` (`scheduled-tasks.json`, the Code-tab
  `local_<uuid>.json`); `local-agent-mode-sessions/<acct>/<org>/` (`scheduled-tasks.json`,
  `rpm/manifest.json`, and `remote-session-spaces.json` after a file-writing chat);
  `local-agent-mode-sessions/skills-plugin/<org>/<acct>/`, in the reversed order, with
  `manifest.json`, `.claude-plugin/plugin.json` and `skills/<name>/SKILL.md` (4 MB, downloaded at
  sign-in: Anthropic's shipped skills plus any skill the user created, told apart by
  `manifest.json` `skills[].creatorType`); `spaces-present/<acct 8 hex>/<org 8 hex>/link-key`
  ([Credentials](#credentials)); `git-shadow/` with `cli-temp-roots.json`
  ([`git-shadow`](#git-shadow)); `sentry/`; and the Chromium `Cache`,
  `Code Cache`, `GPUCache`, `DawnGraphiteCache`, `DawnWebGPUCache`, `Dictionaries`, `Crashpad`,
  `IndexedDB`, `Local Storage`, `Session Storage`, `Shared Dictionary`, `WebStorage` and
  `blob_storage`.
- There is no `vm_bundles` or `claude-code-vm`: `cowork_vm_node.log` says
  `workspace VM not supported (status=unsupported)` and `[Bundle:status] rootfs.img missing` in the
  nested-virtualisation guest, while the app still spawns
  `/usr/lib/claude-desktop/resources/cowork-linux-helper`, which listens on
  `/run/user/<uid>/claude-cowork-vm.sock`. `main.log` also records
  `[updater] Linux: in-app updater off (updates via apt)`.

#### Enterprise (3p) build

Data-storage doc table: macOS `~/Library/Application Support/Claude-3p/` +
`~/Library/Logs/Claude-3p/`; Windows `%LOCALAPPDATA%\Claude-3p\` (logs inside; earlier releases used
`%APPDATA%\Claude-3p\`, migrated on first launch); Linux `~/.config/Claude-3p/` (logs inside).

Another agent writes into the 3p directories (Ollama at
[`6787222`](https://github.com/ollama/ollama/commit/678722224b3d8c2a024a4fa5f530d83ce34db120); not
seen on a real install). `ollama launch claude-desktop` writes a gateway profile into the 3p
build's `configLibrary/`, and on Windows looks for that build's data directory as either
`%LOCALAPPDATA%\Claude-3p` or `%LOCALAPPDATA%\Claude Nest-3p`, beside `%LOCALAPPDATA%\Claude` and
`%LOCALAPPDATA%\Claude Nest` for the standard build
([`cmd/launch/claude_desktop.go:644-649`](https://github.com/ollama/ollama/blob/678722224b3d8c2a024a4fa5f530d83ce34db120/cmd/launch/claude_desktop.go#L644-L649)).
"Claude Nest" is a directory name that only the Ollama source shows; no install observed here used
it, and what build it belongs to was not determined. The profile sets `autoModeEnabled`, which is
true unless `integrations.claude-desktop.automode` in `~/.ollama/config.json` turns it off,
`coworkEgressAllowedHosts` (the hosts the Cowork VM may reach) and both telemetry switches off
([`claude_desktop.go:320-345`](https://github.com/ollama/ollama/blob/678722224b3d8c2a024a4fa5f530d83ce34db120/cmd/launch/claude_desktop.go#L320-L345),
[`741-759`](https://github.com/ollama/ollama/blob/678722224b3d8c2a024a4fa5f530d83ce34db120/cmd/launch/claude_desktop.go#L741-L759));
the full key list is in [Project-local files in `ollama.md`](ollama.md#project-local-files).

#### Contents of the application-data directory

From the data-storage doc, documented for 3p; the same layout names appear in standard builds per
the issues cited in [Source and evidence](#source-and-evidence):

- `ant-did` device id; `configLibrary/_meta.json`, `<id>.json`.
- `local-agent-mode-sessions/`: Cowork and Chat history, one `local_<uuid>.json` plus a working dir
  per session, scoped by account and org id; `uploads/`, `outputs/`, `audit.jsonl` (HMAC-chained
  event log) with `.audit-key`; `.../cowork_account_settings.json`; `.../memory/CLAUDE.md` and
  `memory/memory/*.md`; `.../spaces/<projectId>/memory/`.
- `claude-code-sessions/`: Code session records (working folder, settings, title, summary);
  transcripts are in `~/.claude/projects/`.
- `claude-code/`, `claude-code-vm/` (the docs call them the bundled Claude Code binary and VM
  workspace; observed on macOS, `claude-code/` is the host CLI and `claude-code-vm/` the Linux CLI
  build that runs inside the Cowork VM, [Standard build on macOS](#standard-build-on-macos)),
  `vm_bundles/` (VM bundle
  and data disk), `cowork_plugins/`, `IndexedDB/`, `Local Storage/`, `Session Storage/`.
- Logs: `main.log`, `cowork_vm_node.log`, `claude.ai-web.log`, `mcp.log`, `mcp-server-<name>.log`.
- User outputs outside the app dir: `~/Claude/` (legacy `~/Documents/Claude/`).
- `claude_desktop_config.json` (MCP servers, may hold `env` API keys), `developer_settings.json`,
  `config.json` ([Credentials](#credentials)), Electron `Local State`, `Preferences`, `Cookies`,
  `Cache/`, `Code Cache/`,

  `GPUCache/`, `DawnGraphiteCache/`, `DawnWebGPUCache/`, `sentry/` (standard Chromium/Electron
  layout; names from prior catalog, consistent with the OSCrypt `Local State` dependency reported in
  [CodeZeno/Claude-Code-Usage-Monitor issue 105](https://github.com/CodeZeno/Claude-Code-Usage-Monitor/issues/105)).

### Credentials

- `config.json` in the app-data dir, keys `oauth:tokenCacheV2` (live grant, includes `refreshToken`,
  `expiresAt`, `scopes`) and legacy `oauth:tokenCache`; value is base64 OSCrypt `v10` + nonce +
  AES-256-GCM ciphertext, key from `Local State` (DPAPI on Windows)
  ([CodeZeno/Claude-Code-Usage-Monitor issue 105](https://github.com/CodeZeno/Claude-Code-Usage-Monitor/issues/105),
  [yhm138/ai-quota-tray pull request 6](https://github.com/yhm138/ai-quota-tray/pull/6)). Docs:
  "OAuth tokens ... encrypted with the operating system's secure storage (Keychain on macOS, DPAPI
  on Windows)". So `config.json` + `Local State` together are the credential; on Linux the Chromium
  safe-storage key comes from the keyring or the fixed "peanuts" key.
- Transient files (docs): `host-creds-<random-id>.json` (resolved inference bearer token or API key,
  0600), `ccd-session-secrets/<session-id>/` (GCP ADC / AWS files), and the same cloud credential
  files inside each Cowork session working directory. Deleted on quit/session end but recoverable on
  image.
- `claude_desktop_config.json` `mcpServers.*.env` can embed API keys (MCP debugging doc example).
  `configLibrary/<id>.json` can hold an API key saved from the configuration window (docs: not
  encrypted).
- `.audit-key` in each Cowork session dir: the 32-byte key of the `audit.jsonl` HMAC chain, stored
  OS-encrypted ([`audit.jsonl`](#auditjsonl-bundle)).
- `Cookies` (Chromium cookie DB) holds claude.ai session cookies. On macOS the
  `launch-preview-static` partition keeps a cookie jar of its own,
  `Partitions/launch-preview-static/Cookies` and `Cookies-journal`
  ([Standard build on macOS](#standard-build-on-macos)).
- `Preferences` (306 bytes on macOS) has `electron.webauthn.touchid.metadata_secret`, a 44-character
  base64 string: Chromium's Touch ID WebAuthn metadata secret. The other keys are
  `electron.media.device_id_salt`, `migrated_user_scripts_toggle`, `partition.per_host_zoom_levels`
  and `spellcheck.dictionaries`. The file is collected unflagged; treat `metadata_secret` as a key
  to redact before sharing, like the tokens inside the Cursor and Windsurf state databases. It is
  not a secret glob, because the rest of the file is UI state and flagging it would withhold it
  under `--no-secrets`. The macOS `Local State` (57 bytes) holds only `uninstall_metrics`, with no
  `os_crypt` key: the safe-storage key is in the Keychain. The Linux `Local State` holds only
  `os_crypt.portal` (keys `prev_desktop` and `prev_init_success`) and `uninstall_metrics`, so it
  names no key either.
- `spaces-present/<acct 8 hex>/<org 8 hex>/link-key` (51 bytes, binary, created at sign-in; observed
  on Linux): the OS-encrypted HMAC key that signs Cowork space links and folder grants
  ([`spaces-present` link key](#spaces-present-link-key)).
- `~/.claude/sessions/<pid>.<sha256>.key` (139 bytes on Linux, 108 bytes on macOS) beside
  `~/.claude/sessions/<pid>.json` belongs to the Claude Code CLI the Code tab launches (observed
  with CLI 2.1.295, [issue 76](https://github.com/seanthegeek/doubleagent/issues/76)) and is
  described in [`claude-code.md`](claude-code.md).
- Not credentials (real install):
  - `buddy-tokens.json` is a daily token-usage counter (`index.chunk-Cyt2WTko.js@5079810`).
  - `ccd-ids.json` holds a start-id `salt` (`@1191469`).
  - `os-entry-points.json` holds an account and org id and an `enabled` flag.
  - `extensions-blocklist.json` is the downloaded extension blocklist (`@4364464`,
    `saveBlocklists`).
  - `ca-bundle.pem` is a public CA bundle, `TransportSecurity` is Chromium's HSTS list and
    `Trust Tokens` is Chromium's Private State Token store.
  - Observed on macOS: `vm-support-probe.json` (`hardwareKey`, `key`, strings; the cached result of
    the Cowork VM support probe, keyed by OS release and architecture and, for `key`, the app
    version: `app.asar:.vite/build/index.chunk-Cyt2WTko.js@3754369`
    ``Bfr="vm-support-probe.json",AW=null;function Vfr(){return`${s.default.release()}|${process.arch}`}function Hfr(){return`${a.app.getVersion()}|${Vfr()}`}``),
    `cowork-enabled-cli-ops.json` (`ownerAccountId`, a UUID), `plan-usage-history.json`
    (`version`, `samples[]` of `{org, t, u: {fh, sd}}`) and `ant-device-registry.json` (a UUID key
    to a string).

  None is a secret glob.

- On the MSIX build all of the above sit under the Packages data directory
  ([Per-user storage](#per-user-storage)); the secret globs name both package families.

### Exclusions

`*/Claude*/Cache`, `Code Cache`, `GPUCache`, `DawnGraphiteCache`, `DawnWebGPUCache`,
`*/Claude*/claude-code`, `*/Claude*/claude-code-vm`, `*/Claude*/vm_bundles` (the docs call them
the binary, the VM workspace, and the VM bundle and data disk; `claude-code-vm` is in fact the
Linux CLI build the Cowork VM runs, [Standard build on macOS](#standard-build-on-macos)),
`*/Claude*/sentry`. Keep `IndexedDB/`, `Local Storage/`, `Session Storage/` (UI state, recent
folders). `local-agent-mode-sessions/*/uploads` can be large but is evidence. The
`*/Claude*/Cache` and `*/Claude*/Code Cache` globs cross `/` and also take the `Cache/` and
`Code Cache/` of both `Partitions/` profiles; the rest of `Partitions/` is kept.

Sizes of the excluded subtrees observed on macOS (2026-10-11, 2.31226.1):

- `vm_bundles` 10,844,483,584 bytes: `claudevm.bundle/` and `warm/` with two empty `<hex>`
  directories. In `claudevm.bundle/`, `rootfs.img` is the guest root filesystem (10,737,418,240
  bytes, 10 GiB), `sessiondata.img` the session disk (30,408,704 bytes, 29 MiB,
  [Files inside the Cowork VM](#files-inside-the-cowork-vm)), beside a 56 MiB kernel and 31 MiB
  of initrds; the manual step for the session disk is in
  [Cowork VM disks in `collectors/docs/coverage.md`](../collectors/docs/coverage.md#cowork-vm-disks).
- `Cache` 1,217,064,960; `Code Cache` 161,087,488.
- `claude-code` 476,061,696 (the two app bundles, 2.1.293 and 2.1.295).
- `claude-code-vm` 255,528,960 (the Linux CLI and its markers).

On Linux (2026-10-11) `claude-code/` (the downloaded CLI) is 255,541,248 bytes. `git-shadow/` is
small and kept ([`git-shadow`](#git-shadow)), and so is `local-agent-mode-sessions/skills-plugin/`
([Standard build on Linux](#standard-build-on-linux)).

### Project-local files

None written by the desktop app itself. Code sessions run Claude Code, which writes the `.claude/`,
`CLAUDE.md`, `.mcp.json` set
([Project-local files in `claude-code.md`](claude-code.md#project-local-files)). Cowork sessions
mount the working folder into the VM; attached files are hard-linked into `uploads/`.

### Where the project path is recorded

- Code-tab record `claude-code-sessions/<acct>/<org>/local_<uuid>.json` (the docs: "each session's
  working folder"): `cwd`, `originCwd` and `gitAnchorsFolderRealpath` are the project path;
  observed equal to the chosen folder on Windows (2026-10-10), Linux (2026-10-11) and macOS
  (2026-10-11).
- Cowork record `local-agent-mode-sessions/.../local_<uuid>.json`: `userSelectedFolders` (bundle,
  [Cowork session record](#cowork-session-record-local_uuidjson-bundle); `cwd` is a VM path there);
  `spaces/<projectId>/` for Cowork projects (docs) and `spaces.json` `folders[].path`
  ([Other per-org files](#other-per-org-files-bundle-names-no-timeline-value)).
- `Local Storage/` holds "recent folders" (docs) in LevelDB.
- `claude_desktop_config.json` (observed on macOS, 2026-10-11): `coworkUserFilesPath` is an absolute
  path (the Cowork user-files root), and `preferences.remoteChatFolderGrants.<uuid>.folders[]` are
  the absolute folders granted to remote chats (each entry also has `accountUuid` and `orgUuid`).
- `local-agent-mode-sessions/<acct>/<org>/remote-session-spaces.json`: `entries[].folders[]` are the
  absolute folders granted to a remote Cowork session (observed on Windows, Linux and macOS).
- `git-shadow/cli-temp-roots.json` lists Claude Code CLI temp roots, not project paths
  ([`git-shadow`](#git-shadow)).

## Parsing

### Transcript stores

All paths are relative to the app-data directory (`userData`): macOS
`~/Library/Application Support/Claude/`, Linux `~/.config/Claude/`, Windows `%APPDATA%\Claude\` as
the app reports it, which for the MSIX build is stored on disk as
`%LOCALAPPDATA%\Packages\Claude_pzs8sxrjxfjjc\LocalCache\Roaming\Claude\` (observed; MSIX filesystem
virtualisation, [Standard build on Windows](#standard-build-on-windows)); the enterprise build
appends `-3p`. `<acct>` and
`<org>` are the account and organization UUIDs, or their first eight hex digits when the app has
switched to short names (observed: full UUIDs), in:

- `app.asar:.vite/build/index.chunk-Cyt2WTko.js@2238150`
  `function GZt(e,t){let r=n.default.join(a.app.getPath("userData"),uC);return{shortPath:zZt?n.default.join(r,HC(e),HC(t)):null,fullPath:n.default.join(r,e,t)}}`,
  where `HC` keeps the first eight characters of a UUID.

| Path | Format | What it holds | Parse? |
| --- | --- | --- | --- |
| `local-agent-mode-sessions/<acct>/<org>/local_<uuid>.json` | JSON object, pretty-printed (2 spaces), rewritten whole | Cowork or desktop-chat session record ([Record schema](#record-schema)) | yes, metadata and first prompt |
| `local-agent-mode-sessions/<acct>/<org>/agent/local_<uuid>.json` | same | session record when `sessionType` is `agent` | yes |
| `local-agent-mode-sessions/<acct>/<org>/local_<uuid>/` or `.../<8 hex>/` | directory | session storage dir: `outputs/`, `uploads/`, `.claude/`, `audit.jsonl`, `.audit-key` | see below |
| `.../<session dir>/.claude/projects/<slug>/<cliSessionId>.jsonl` | Claude Code JSONL | the Cowork transcript | yes, with the claude-code reader |
| `.../<session dir>/audit.jsonl` | JSONL, append-only, HMAC-chained | every Agent SDK message except stream deltas, plus permission events | yes, every line, a secondary source of the transcript ([Parser plan](#parser-plan)) |
| `local-agent-mode-sessions/<acct>/<org>/agent/local_ditto_<id>[_g<n>]/` | directory per dispatch agent generation | `audit.jsonl` and `.claude/projects/*/*.jsonl` | yes, same readers |
| `claude-code-sessions/<acct>/<org>/local_<uuid>.json` | JSON object, compact, rewritten whole | Code-tab session record; the transcript is in `~/.claude/projects/` | yes, one system row |
| `claude-code-sessions/<acct>/<org>/imported-staging/<cliSessionId>.jsonl` | Claude Code JSONL | a transcript imported from another surface | yes, claude-code reader |
| `local-agent-mode-sessions/<acct>/<org>/scheduled-tasks.json`, `claude-code-sessions/<acct>/<org>/scheduled-tasks.json` | JSON | scheduled task definitions (observed, empty) | optional system rows |
| `git-worktrees.json` | JSON | worktrees the Code tab created (observed, empty) | optional system rows |
| `IndexedDB/https_claude.ai_0.indexeddb.leveldb/`, `Local Storage/leveldb/`, `Session Storage/` | Chromium LevelDB | claude.ai web page state ([Other stores](#other-stores)) | no |
| `Partitions/<name>/` | a Chromium profile of an Electron session partition | browser and file-preview state ([Standard build on macOS](#standard-build-on-macos)) | no |
| `vm-support-probe.json`, `cowork-enabled-cli-ops.json`, `plan-usage-history.json`, `ant-device-registry.json` | JSON | small app state, keys in [Credentials](#credentials) | no |
| `main.log`, `mcp.log`, `mcp-server-<name>.log` | text | app and MCP transport logs; locations per platform in [Per-user storage](#per-user-storage), format in [Logs](#logs) | no |

Evidence for the layout (all bundle):

- Session record path: `app.asar:.vite/build/index.chunk-QEr06WaZ.js@445720`
  ``getSessionFilePath(e){let n=this.getAccountStorageDir();return n?this.sessions.get(e)?.sessionType==="agent"?(0,R.join)(n,t.JU,`${e}.json`):(0,R.join)(n,`${e}.json`):null}``,
  with `t.JU` exported from `index.chunk-Cyt2WTko.js` as `dC="agent"` (`@2215319`,
  `var mJt="local_" ... uC="local-agent-mode-sessions",vJt="spaces-present",dC="agent"`). The
  session id is `local_` plus a lowercase UUID (`index.chunk-QEr06WaZ.js`,
  `registerExternalSession`: `rejected uppercase session id`).
- Session storage dir and its short form: `app.asar:.vite/build/index.chunk-Cyt2WTko.js@2238529`
  `var YZt=/^local_([0-9a-f]{8})-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;function GC(e){let t=YZt.exec(e);return t?t[1]:null}`;
  the directory is `local_<uuid>` or the eight-hex short name, whichever exists
  (`function nQt(e,t){let r=n.default.join(e,t),i=GC(t);...`).
- Contents of the session dir: `app.asar:.vite/build/index.chunk-QEr06WaZ.js@446368`
  `sessionDirPaths(e){return{outputsDir:(0,R.join)(e,"outputs"),claudeDir:(0,R.join)(e,".claude"),hostProcessCwd:t.CU()??(0,R.join)(e,"host-cwd")}}`;
  `uploads` is the sibling of `outputs` (`Zt=g?(0,R.join)((0,R.dirname)(g),"uploads"):void 0`).
- The session's `.claude` dir is the CLI's `CLAUDE_CONFIG_DIR`: `@451774`
  `getClaudeConfigDir(e){let t=this.getSessionStorageDir(e);...let{claudeDir:n}=this.sessionDirPaths(t);...return n}`;
  in VM mode it is mounted into the guest at `/sessions/<vmProcessName>/mnt/.claude` (`@243435`
  ``let Ae=`/sessions/${o}/mnt/.claude` ... G=h?t.NU(r.getClaudeConfigDir(i)):r.getClaudeConfigDir(i),K=h?G:Ae``),
  so the transcript the guest CLI writes lands on the host. One spawn path also sets
  `CLAUDE_CODE_PROJECT_DIR_NAME:t.xW` (`@313032`) with `xW` = `OJt="session"`, so the slug directory
  is literally `session` there; the desktop's own transcript lookup reads every slug directory
  (`app.asar:.vite/build/index.chunk-Cyt2WTko.js@6514954`
  ``s=n.default.join(o,".claude","projects") ... Bb(n.default.join(e,r,`${i.cliSessionId}.jsonl`),e)``),
  so a parser must not assume the slug.
- `audit.jsonl` and `.audit-key` sit in the session dir: `index.chunk-QEr06WaZ.js@441475`
  `auditLog(e,t){...let n=this.getSessionStorageDir(e);...j.i(n,...)` and
  `app.asar:.vite/build/index.chunk-Dx-yqN_-.js@4422` `r.default.join(e,"audit.jsonl")`,
  `var l=".audit-key"`.
- Dispatch agent generations: `index.chunk-Cyt2WTko.js`
  `` function gJt(e,t){let n=`${mJt}ditto_${e}`;return t>0?`${n}_g${t}`:n} ``, and the reader in
  `index.chunk-Dx-yqN_-.js@7384` that lists, per generation dir under `agent/`,
  `` `${e.path}/audit.jsonl` `` and every `.jsonl` under ``${e.path}/.claude/projects``.
- Code-tab tree: `index.chunk-Cyt2WTko.js@1984690`
  `var lb="claude-code-sessions",ub="local_",APt="unknown:lessee",jPt="archived-sessions.idx",MPt="imported-staging"`.
  Code transcripts are read from the session dir's `.claude/projects` and then from `C9()` =
  `<CLAUDE_CONFIG_DIR or ~/.claude>/projects` (`@6486039`
  `function C9(){return n.default.join(jj(),"projects")}`). The Code tab launches the CLI with
  `CLAUDE_CODE_ENTRYPOINT` `claude-desktop` or `claude-desktop-3p` (`@4137317`
  `var mIr=["claude-desktop","claude-desktop-3p"];function hIr(e){return e==="3p"?"claude-desktop-3p":"claude-desktop"}`),
  and Cowork with `CLAUDE_CODE_ENTRYPOINT:"local-agent"` (`index.chunk-QEr06WaZ.js`), which is the
  `entrypoint` value the [claude-code research](claude-code.md) documents on every transcript line.

#### What survives a delete

What the desktop's own session delete or archive removes is not determined from the bundle: a Cowork
session's transcript and its `audit.jsonl` live in the same session dir, so deleting that directory
loses both, and whether the app removes either one on its own is not known. The session records
carry an `isArchived` flag and the Code-tab
tree names an `archived-sessions.idx` index (bundle); what an archive changes on disk is not stated,
and no deletion marker is stated. The desktop keeps no per-prompt history file of its own that this
document names; a Code-tab transcript is a Claude Code file under `~/.claude/projects/`, with that
agent's history file and retention
([What survives a delete in `claude-code.md`](claude-code.md#what-survives-a-delete)). Within a
Cowork session, `audit.jsonl` repeats the transcript's turns and survives the loss of the transcript
alone (a transcript never written, removed, excluded by size or not collected), and the session
record's `initialMessage` keeps the first prompt when both are gone. A chat in the merged
Chat/Cowork surface that ran as a remote Cowork session left only `remote-session-spaces.json`
locally, its transcript being server-side
([`remote-session-spaces.json`](#remote-session-spacesjson)). The analyzer reads
the audit log and the record's `initialMessage` as secondary sources (a Code-tab record's prompt
only when its transcript was not collected) and the records' own `system` rows always; it does not
read the logs, the LevelDB stores or `archived-sessions.idx` (see
[Secondary sources](../analyzer/docs/output.md#secondary-sources)).

#### Files inside the Cowork VM

None of the transcript paths above is under an `EXCLUDES` glob: `*/Claude*/claude-code-vm` holds
only the Linux CLI build that runs inside the Cowork VM, not a VM workspace
([Standard build on macOS](#standard-build-on-macos)), and the guest's `.claude` is the host
session dir, not anything under `vm_bundles`. What `*/Claude*/vm_bundles` drops is
`vm_bundles/claudevm.bundle/sessiondata.img` (observed; the bundle classifies `sessiondata.*` as the
session disk, `index.chunk-Cyt2WTko.js` `e.startsWith("sessiondata.")?"sessionData"`), the guest
disk that holds each `/sessions/<name>/` working tree outside `mnt/` (the janitor logs
``failed to delete VM dir /sessions/${e}``). Files the agent wrote only inside the VM and never into
a mounted folder are therefore not collected by default. The session disk itself is small, while
the rest of `vm_bundles` is almost all the guest root filesystem ([Exclusions](#exclusions)).

### Record schema

#### Cowork session record (`local_<uuid>.json`, bundle)

- Written whole by `writeSessionToDisk` (`app.asar:.vite/build/index.chunk-QEr06WaZ.js@467441`, then
  `t.B4(r,o)`, which is `JSON.stringify(t,null,2)`).
- Read back with only `sessionId` (string), `createdAt` and `lastActivityAt` (numbers) required
  (`index.chunk-Cyt2WTko.js@6512360`
  `return typeof t.sessionId=="string"&&typeof t.createdAt=="number"&&typeof t.lastActivityAt=="number"?t:void 0`).

Keys, in write order; undefined values are omitted by `JSON.stringify`:

| Key | Type | Meaning |
| --- | --- | --- |
| `sessionId` | string | `local_<uuid>`, the desktop id |
| `processName`, `vmProcessName` | string | guest process name; the VM cwd is `/sessions/<vmProcessName>` |
| `cliSessionId` | string (UUID) | the Claude Code session id, the transcript file name |
| `cwd` | string | CLI working directory: a VM path `/sessions/<name>` in VM mode (`@350665` ``U={cwd:`/sessions/${s}` ``), a host path in host-loop mode |
| `userSelectedFolders` | string[] | host folders the user attached; the real project paths |
| `resolvedFolderKinds`, `fileDeleteApprovedMounts`, `folderMountNames` | array, object | folder mount bookkeeping |
| `createdAt`, `lastActivityAt` | number | epoch milliseconds (`Date.now()`) |
| `model` | string | model id |
| `permissionMode` | string | CLI permission mode |
| `isArchived` | boolean | |
| `title` | string | session title |
| `initialMessage` | string | the first prompt as typed |
| `sessionType` | string | `agent`, `dispatch_child`, `scheduled`, `chat`, `radar` (`index.chunk-Cyt2WTko.js@2215319`, `fC`, `jJt`) |
| `parentSessionId`, `dispatchParentOrigin` | string | dispatch child to parent |
| `scheduledTaskId`, `scheduledRunContinued` | string, boolean | run of a scheduled task |
| `spaceId`, `spaceIdSetBy`, `userSelectedProjectUuids` | string, string, string[] | Cowork project ("space") |
| `importedFrom` | string | `claude-ai-chat`, `local-1p-cowork`, `local-1p-chat`, `local-1p-code`, `terminal-cli`, `sibling-1p`, `sibling-3p`, `previous-profile` (`@1984690`, `db={...}`, `YPt`) |
| `hostLoopMode` | boolean or string | CLI runs on the host instead of the VM; type not determined |
| `error`, `errorCategory`, `errorAt`, `errorVersion` | string, string, number, string | last session error; `errorAt` type inferred from the Code record, where it is `Date.now()` |
| `enabledMcpTools`, `remoteMcpServersConfig`, `approvedToolNames`, `webFetchAllowedUrls`, `egressAllowedDomains`, `userApprovedFileAccessPaths` | arrays and objects | tool and network grants |
| `chromePermissionMode`, `chromeAllowedDomains`, `cuAllowedApps`, `cuGrantFlags`, `cuFlagsGrantedAt` | | browser and computer-use grants |
| `systemPrompt`, `systemPromptRendererAppends`, `imagineSystemPrompt`, `memoryGuidelinesTemplate`, `coworkSyspromptMap`, `spSectionPrompts` | strings and objects | the system prompt sent |
| `accountName`, `emailAddress` | string | the signed-in person ([Secrets](#secrets)) |
| `otelConfig`, `orgOtlpContentCapture` | object | telemetry export settings ([Secrets](#secrets)) |
| `pendingNotifications` | string[] | queued system reminders |

The remaining keys (`resumeConfirmed`, `relinkedToSpace`, `spacePlacedByUser`,
`globalMemoryBaselineHash`, `slashCommands`, `servesUiSurface`, `mcqAnswers`, `fsDetectedFiles`,
`chromeTabGroupId`, `chromePermsBeforeUnsupervised`, `effortOverride`, `pinnedSpawnEffort`,
`cuLastScreenshotDims`, `cuSelectedDisplayId`, `withheldConnectorHosts`, `orgCliExecPolicies`,
`memoryEnabled`, `skillsEnabled`, `pluginsEnabled`, `documentFunnelEnabled`, `docxEditingCarveout`,
`frameArtifactsEnabled`, `artifactHostGrant`, `pluginInstallPaths`, `isStarred`,
`outboundCCRRemoteId`, `promptSuggestion`) are UI state with no timeline value.

#### `audit.jsonl` (bundle)

One JSON object per line, appended (`index.chunk-Dx-yqN_-.js@4422`, `t.O4(i,{flags:"a"})`). The
session manager logs:

- every message the Agent SDK query loop yields except stream deltas
  (`index.chunk-QEr06WaZ.js@181477` `o.type!=="stream_event"&&this.delegate.auditLog(n,o)`); the
  loop branches on `type` `system` (subtypes `init`, `commands_changed`, `background_tasks_changed`,
  `dev_intent`, `model_refusal_fallback`), `assistant`, `user`, `result` and `prompt_suggestion`.
  These are the SDK's stream-json messages: `assistant` and `user` carry a `message` object with
  Anthropic `content` blocks (`text`, `tool_use`, `tool_result`, `thinking`), `user` tool results
  may carry `tool_use_result`; `result` carries `subtype`, `duration_ms`, `duration_api_ms`,
  `is_error`, `num_turns`, `session_id`, `uuid` (the desktop's own synthetic result,
  `index.chunk-Cyt2WTko.js`,
  `{type:"result",subtype:"success",duration_ms:0,duration_api_ms:0,is_error:!1,num_turns:0,session_id:e,uuid:...}`).
- each prompt the person sends, built by the desktop before the CLI sees it (`@409599`):
  `{type:"user",uuid:x,session_id:y,parent_tool_use_id:null,client_platform:"desktop_app",...,timestamp:(new Date).toISOString(),...,message:{role:"user",content:b}}`;
  meta notifications injected by the desktop have the same shape with `isSynthetic:true` and no
  `timestamp`.
- permission events, `type` `system` with `subtype` `permission_request` (`uuid`, `session_id`,
  `tool_name`, `tool_input`), `permission_response` (`uuid`, `session_id`, `tool_name`, `decision`,
  `granted`; `decision` is the person's answer `deny`, `once`, `always` or `scheduled` at the dialog
  site (`switch(n){case"deny":` ... `case"once":` ... `case"always":case"scheduled":`), or `once`,
  `always` or `deny` derived from the behavior at the bridge site
  (`r.behavior==="allow"?r.updatedPermissions?"always":"once":"deny"`), and `granted` is whether the
  resulting behavior was `allow`), `permission_auto_approved` and `permission_auto_denied`
  (`session_id`, `tool_name`, and one of `source`, `scheduled_task_id`, `matched_ops`) (`@220582`
  and the other `auditLog(` call sites).

Before writing, the logger adds three keys (`index.chunk-Dx-yqN_-.js@5069`):

- `_audit_timestamp`: `(new Date).toISOString()`, the write time.
- `_audit_redactions`: present only when base64 image or document data of 256 characters or more was
  replaced; an array of JSON Pointers (`/message/content/0/source/data`), and the data itself
  becomes `[base64 omitted: <media type> <n> bytes, sha256 <hex>]`.
- `_audit_hmac`: hex HMAC-SHA256, keyed with the 32-byte session key, of the previous line's
  `_audit_hmac` followed by `JSON.stringify` of the record without `_audit_hmac`; the first line
  chains from the literal string `audit-chain-genesis` (`@795`
  `var s="audit-chain-genesis";function c(e,t,n){return(0,o.createHmac)("sha256",e).update(t).update(n).digest("hex")}`).
  On restart the chain resumes from the last `_audit_hmac` in the file. The key is stored in
  `.audit-key` as `safeStorage.encryptString` of its base64 form (Keychain on macOS, DPAPI on
  Windows); when safe storage is unavailable the lines are written without `_audit_hmac`
  (`[audit] safeStorage unavailable — audit log will not be HMAC-signed`). Verifying the chain
  therefore needs the decrypted key, which needs the user's keychain; without it the chain can only
  be checked for continuity, not authenticity.

The `session_id` inside a line is the CLI's id in the normal case, but not always: the desktop's
synthetic notification falls back to the desktop UUID without `local_` before the CLI id is known
(`let n=e.cliSessionId??e.sessionId.replace("local_","")`), and the permission events take
`session_id` from the session object, which may be unset. Take the session from the directory and
its record, not from the line.

#### Code-tab session record (`claude-code-sessions/.../local_<uuid>.json`, bundle, observed on Windows, Linux and macOS)

Compact JSON written by `writeSessionToDisk`
(`app.asar:.vite/build/index.chunk-BCHP6ll9.js@1339940`, `let i=n.Ct(e),a=JSON.stringify(i)`),
serializer `app.asar:.vite/build/index.chunk-epGqPC4W.js@549559` `function dk(e)`. Keys of use to a
timeline:

| Key | Type | Meaning |
| --- | --- | --- |
| `sessionId` | string | `local_<uuid>` |
| `cliSessionId` | string | joins to `~/.claude/projects/<slug>/<cliSessionId>.jsonl` |
| `cwd`, `originCwd` | string | working folder; `originCwd` is the folder chosen before a worktree |
| `worktreePath`, `worktreeName`, `branch`, `sourceBranch` | string | worktree and git branch |
| `createdAt`, `lastActivityAt`, `lastFocusedAt` | number | epoch ms |
| `model`, `effort`, `permissionMode` | string | |
| `title`, `titleSource`, `previousTitles` | string, string, array | |
| `recap`, `recapAt` | string, number | generated summary; `recapAt` type not determined |
| `postTurnSummary` | object | observed: `status_category`, `status_detail`, `needs_action`, `summarizes_uuid`; not a string |
| `prs` | array | pull requests opened from the session |
| `sshConfig`, `wslConfig`, `sshRemoteTranscriptPath` | object, object, string | remote sessions; the transcript is then on the remote host |
| `error`, `errorCategory`, `errorAt` | string, string, number | `errorAt` is `Date.now()` in this chunk |
| `isArchived`, `scheduledTaskId`, `spaceId`, `forkedFromSessionId`, `dispatchParentId` | | |
| `gitAnchorsFolderRealpath`, `completedTurns`, `titleTurn`, `lastAssistantUuid`, `latestUserFrameAt`, `promptAppendSnapshot`, `toolSurfaceSnapshot`, `remoteMcpServersConfig` | | observed on Windows, not in the bundle reading above; `remoteMcpServersConfig[]` items carry `uuid`, `name`, `url`, `tools[]`; other types not recorded |
| `remoteControlUserEnabled`, `remoteControlUserRequested`, `remoteControlUserToggled`, `remoteControlAutoEligible`, `bridgeSessionIds`, `remoteControlSpawn`, `remoteControlDescendant`, `steeredByRemoteClient`, `peerMessaged`, `remoteControlLiveAtUpdateQuit`, `cloudSessionId` | booleans; string[]; object `{ccrSessionId, detachedAt, rcChild, projectThreadChild}`; object `{at, accountId}`; string | persisted Remote Control state (below); observed on Windows, Linux and macOS: `remoteControlAutoEligible` and `steeredByRemoteClient` `false`, the rest absent |
| `alwaysAllowedReasons`, `sessionPermissionUpdates`, `autoChosenInApp`, `autoModeServerFallbackPrompt`, `chromePermissionMode`, `classifierSummaryEnabled`, `lastSpawnRootDetected`, `postTurnSummaryFor`, `reportFindingsCard`, `spawnSeed` | `[]`, `[]`, then not recorded except `chromePermissionMode` (string), `postTurnSummaryFor` (UUID, the message `postTurnSummary` describes) and `spawnSeed` (`{}`) | observed on macOS 2026-10-11, not in the bundle reading above; `chromePermissionMode` is also a Cowork record key |

Nested shapes observed on macOS (2026-10-11, key names and types only):

- `remoteMcpServersConfig[]`: `{name, tools[], url, uuid}`, each tool
  `{_meta: {anthropic/alwaysLoad}, annotations: {destructiveHint, readOnlyHint, title}, description, inputSchema, name}`.
- `toolSurfaceSnapshot`:
  `{appRunId, appVersion, cliVersion, deferredHash, disallowedToolsKey, familyHashes: {builtin, connector, extension, internal_deferred, internal_inline, plugin_or_user}, recordedAt}`.
- `promptAppendSnapshot`: `{append, cliVersion, settingsKey}`, where `append` is a string (7.8 KB
  observed); its content was not read.
- `postTurnSummary`: `{needs_action, status_category, status_detail, summarizes_uuid}`, as on
  Windows.

Imported records (`finishRegisterExternalSession`, `@1686493`) also carry `indexedAt` (epoch ms),
`importedFrom` and `stagedTranscriptPath`.

#### Remote Control state in the Code-tab record

Remote Control is another device driving the session's CLI through a bridge
([issue 80](https://github.com/seanthegeek/doubleagent/issues/80)); the CLI side is in
[`claude-code.md`](claude-code.md), [Remote Control](claude-code.md#remote-control). The serializer
`dk` (`app.asar:.vite/build/index.chunk-epGqPC4W.js@549559`, keys from `@554789` `bridgeSessionIds`
to `@555190` `peerMessaged`) persists `remoteControlUserEnabled`, `remoteControlUserRequested`,
`remoteControlUserToggled`, `remoteControlAutoEligible`, `bridgeSessionIds[]` (every bridge session
the session was attached to), `remoteControlSpawn` (the session was started from a remote client:
`ccrSessionId`, `detachedAt`, `rcChild`, `projectThreadChild`), `remoteControlDescendant`,
`steeredByRemoteClient` (a remote client steered a turn), `peerMessaged`,
`remoteControlLiveAtUpdateQuit` (`at`, `accountId`) and `cloudSessionId`.

The setters are in `index.chunk-BCHP6ll9.js`:

- `@653921` `e&&(e.steeredByRemoteClient=!0,this.deps.saveSession(e))`
- `@794905` `e.bridgeSessionUrl=n?.session_url,e.bridgeSessionId=r,e.bridgeEpoch=n?.bridge_epoch`
- `@761056`
  `function kM(e){e.remoteControlEnabled=!1,...,e.remoteControlConnecting=void 0,...,e.bridgeSessionId=void 0,e.bridgeSessionUrl=void 0,e.bridgeEpoch=void 0}`,
  which clears the live fields.

The live `bridgeSessionId`, `bridgeSessionUrl` and `remoteControlConnecting` are not persisted, so
whether a session was under Remote Control at collection time comes from the CLI's
`~/.claude/sessions/<pid>.json` (`bridgeSessionId`), joined on the record's `cliSessionId`.
`main.log` records these lines, both in local time with no zone:

- `Enabling remote control for session <id>` (or `Disabling`; `@787391`
  ``t.m4.info(`${h?"Enabling":"Disabling"} remote control for session ${s}${c?" (re-adopted process)":""}`)``)
- then `Remote control enabled: <url>` (`@795095`
  ``t.m4.info(`Remote control enabled: ${AM(n?.session_url)}`)``)

Separately, `[remote-tools-device]` log lines (`index.chunk-Cyt2WTko.js@5123319`) and the
`remoteToolsDeviceName` preference in `claude_desktop_config.json` (`@1437532` default
`remoteToolsDeviceName:""`) record the Desktop's enrolment as a device whose local tools claude.ai
can call, a different feature.

#### `scheduled-tasks.json` (observed top level, bundle entries)

Observed keys: `scheduledTasks` (array, empty here), `recordedSkips` (object),
`sundayAliasBoundaryStamped`, `dayFieldsOrBoundaryStamped` (booleans). The schema adds
`recordedPlaceSkips` and `runRetries`. Each task
(`app.asar:.vite/build/index.chunk-Cyt2WTko.js@5534719`, `bZi=j({...})`): `id`, `displayName`,
`cronExpression` (string) or `fireAt` (number, one-shot), `enabled` (boolean), `filePath` (the
task's `SKILL.md`, which holds the prompt), `model`, `createdAt` (number), `lastRunAt` and
`lastScheduledFor` (strings; `lastScheduledFor` is compared with `toISOString()`), `cwd`,
`userSelectedFolders`, `userSelectedFiles`, `permissionMode`, `useWorktree`, `sourceBranch`,
`spaceId`, `importedFrom`, migration markers. Cowork task files live in
`<cowork user files>/Scheduled/<id>/SKILL.md`, where the user-files root is `~/Claude`,
`~/Documents/Claude` or `userData/cowork-user-files` (`@5464085`
`function zqi(){return(0,n.join)(RC(),"Scheduled")}`, `@2233279`
`function NC(){return n.default.join(a.app.getPath("home"),"Claude")}`), all inside existing catalog
entries; Code-tab tasks live in `<CLAUDE_CONFIG_DIR or ~/.claude>/scheduled-tasks` (claude-code's
tree).

#### `git-worktrees.json` (observed top level, bundle entries)

Observed keys: `schemaVersion` (number), `worktrees` (object), `untrackedDirGc` (`cwds`, `roots`,
`sightings`). Each `worktrees` value (`app.asar:.vite/build/index.chunk-3ucQSJ7U.js@131467`) has
`name`, `path`, `leasedBy` (the desktop session id), `baseRepo`, `branch`, `sourceBranch`,
`createdAt` (`Date.now()`), and optionally `placementRoot`, `originRemote`, `anchors`, `hookBased`,
`status`.

#### Other per-org files (bundle names, no timeline value)

Under `local-agent-mode-sessions/<acct>/<org>/` (`index.chunk-Cyt2WTko.js@2239731` onward):
`spaces.json` (`spaces[]` with `id`, `name`, `projects[].uuid`, `folders[].path`, `instructions`,
`origin`, `createdAt`, `updatedAt`), `remote-session-spaces.json`, `cowork_settings.json`,
`cowork_account_settings.json`, `cowork-gb-cache.json`, `cowork-clientdata-cache.json`,
`cowork-policy-limits-cache.json`, `usage-ledger`, `artifacts/`, `memory/`, `agent/memory/`,
`spaces/<id>/memory/` (Markdown memory files, `@2241798`).

`rpm/manifest.json` (observed: `lastUpdated` epoch ms, `plugins[]`) is the remote plugin manifest.

#### `remote-session-spaces.json`

`remote-session-spaces.json` (observed on Windows, Linux and macOS; bundle
`index.chunk-Cyt2WTko.js@2239915`, beside `scheduled-tasks.json`, `spaces.json`,
`cowork-gb-cache.json` and `cowork-clientdata-cache.json`) maps server-side sessions to folders:
`entries[]`, each with `sessionId` (32 hexadecimal characters, not the `local_<uuid>` form) and
`folders[]` (absolute paths). It is the on-disk trace that a remote Cowork session existed; its
transcript is server-side. On Linux (2026-10-11) a read-only chat of about nine minutes wrote
nothing under `local-agent-mode-sessions/`, and a chat asked to write a report ran as a remote
Cowork session: the agent ran server-side and wrote an `.odt` and a `.pdf` into `~/Documents`
through the app's remote-file bridge (`main.log`: `[remote-file] listed 0 entries`, then
`[remote-file] committed 2 files, 0 rejected`; `claude.ai-web.log`:
`[LOCAL_SESSION] remote_cowork.late_link_offer_verdict`), and the only local write was this file
(147 bytes, written when the folder was granted, `main.log` `Config file written`), with `folders[]`
the granted folder. There was no `local_<uuid>.json`, session directory or `audit.jsonl`, and
nothing in `cowork_vm_node.log` or `mcp.log`. The same test on the author's macOS install of
2.31226.1 (2026-10-11, a Mac that can run the Cowork VM) gave the same result: one
`remote-session-spaces.json` (146 bytes, same shape) and no local session. So on all three platforms
(MSIX build, Linux `.deb`, macOS) a file-writing chat in the merged surface is a server-side session
whose local trace is this file, and the local-agent session layout (`local_<uuid>.json`,
`audit.jsonl`) was not observed on any of the three real installs; it remains bundle-derived.
`scheduled-tasks.json` stayed empty in both per-org directories although the account had a scheduled
task, so tasks set from a chat are server-side too.

### Joins and timestamps

- Desktop session to transcript: record `cliSessionId` names
  `<session dir>/.claude/projects/<any slug>/<cliSessionId>.jsonl` for Cowork, and
  `~/.claude/projects/<slug>/<cliSessionId>.jsonl` (or the session dir, or `imported-staging/`) for
  the Code tab. Record file `local_<uuid>.json` to session dir: same base name, or the first eight
  hex digits of the UUID.
- Code-tab session to the running CLI: the CLI's `~/.claude/sessions/<pid>.json` carries
  `hostSessionId`, the desktop `local_<uuid>` (observed on Linux, CLI 2.1.295).

- Audit line to transcript line: both carry the SDK `uuid` for `assistant` and `user` messages;
  whether the values are equal was not determined (no audit file on disk to compare). The parser
  therefore pairs them as the secondary-source rule of
  [issue 122](https://github.com/seanthegeek/doubleagent/issues/122) does
  ([Parser plan](#parser-plan)): same `turn_type` and text, the same `session_id` (the record's
  `cliSessionId` on both sides, the transcript's file name and JSONL `sessionId`), and times at most
  2 seconds apart. The audit time is the desktop prompt's `timestamp` (set as the desktop sends it)
  or `_audit_timestamp` (set as the query loop yields the message); the transcript time is the CLI's
  record time. How far apart they run on a real install is not measured.
- Audit log to project: the record's `userSelectedFolders[0]`, as for the transcript; with no
  record, the `system` `init` line's `cwd`, which is the CLI's working directory and the value the
  transcript records as its JSONL `cwd` (the guest path `/sessions/<name>` in VM mode, as the
  fixture's `init` line and transcript both have).
- Scheduled run to task: record `scheduledTaskId` = task `id`.
- Worktree to session: worktree `leasedBy` = record `sessionId`.

| Field | Format | Zone |
| --- | --- | --- |
| record `createdAt`, `lastActivityAt`, `lastFocusedAt`, `indexedAt`, `errorAt` | epoch ms number | UTC |
| audit `_audit_timestamp`, desktop user message `timestamp` | ISO 8601 with milliseconds and `Z` | UTC |
| transcript JSONL `timestamp` | as in claude-code | UTC |
| scheduled task `createdAt`, `fireAt` | number, epoch ms assumed from the sibling fields | UTC |
| scheduled task `lastScheduledFor` | ISO string | UTC; `lastRunAt` same type, format not determined |
| worktree `createdAt`, `rpm/manifest.json` `lastUpdated` | epoch ms | UTC |
| `main.log`, `mcp*.log` line prefix | `YYYY-MM-DD HH:MM:SS`, no fraction | **local time, no zone** (`@247348` `function Xde(){...${e.getFullYear()}-${xr(e.getMonth()+1)}-${xr(e.getDate())} ${xr(e.getHours())}...`) |

### Other stores

Desktop chat sessions (`sessionType` `chat`) are local Cowork sessions and are covered by
[Transcript stores](#transcript-stores).

#### claude.ai chats

The main window loads `https://claude.ai` (`index.chunk-Cyt2WTko.js`,
`Redirecting to claude.ai ... loadURL($D()`), so the chat UI and its storage belong to the website,
not the bundle; conversations are server-side. Observed on disk: one IndexedDB origin
`https_claude.ai_0` whose LevelDB holds a database named `react-query-cache` and keys
`store:chat-draft:<uuid>` and `store:pin-state:...`, values wrapped as `vnd.blink-idb-value-wrapper`
with an `.indexeddb.blob/` sidecar; Local Storage keys under `https://claude.ai` include
`react-query-cache-ls`, `LSS-persisted.*` and `intercom.*`. No plaintext `chat_conversations`,
`sender` or `created_at` strings were found in either store, so whether the query cache holds
message bodies was not determined. Reading it would need a LevelDB reader (log and table files,
Snappy blocks), the Chromium IndexedDB key coder, the Blink value wrapper and blob sidecar, and V8
deserialization; that is out of scope for this parser. Unsent drafts (`chat-draft`) are the only
obviously user-typed text.

#### Logs

`main.log` (rotated as `main1.log` to `main4.log`, observed) uses
`${timestamp} [${level}] ${message}` (`index.chunk-Cyt2WTko.js`, winston `printf`). The MCP
transport logs `mcp.log` and `mcp-server-<name>.log` (`@4328284`) record each JSON-RPC message as
`Message from client: method="tools/call" id=5 params` or
`Message from server: id=5 result(2 blocks)`; the summariser (`@4334523` `function oYr(e)`) keeps
only `method`, `id`, an error code and a block count, never the tool name or arguments. With local
times and no tool names, neither is worth `system` rows; the Cowork audit log carries the same calls
with UTC times and names. The formats of the other logs listed per platform in
[Per-user storage](#per-user-storage) were not read.

#### Windows data directories

The standard build on Windows picks its userData from four candidates, chosen in:

- `app.asar:.vite/build/index.chunk-Cyt2WTko.js@837614`
  `function vze(){...return{child:_ze().at(0)??null,dataDir:gze(),squirrel1p:n.join(a.app.getPath("appData"),wu),msix1p:e?mze(e,Tu,"Roaming",wu):null}}`

The candidates are `%LOCALAPPDATA%\Claude-Data\<package family, lowercased>` (`WRe="-Data"`,
families `Claude_pzs8sxrjxfjjc` and `AnthropicPBC.Claude_fnn82j28hfe8t`),
`%LOCALAPPDATA%\Claude-Data`, `%APPDATA%\Claude`, and the MSIX virtualised copy
`%LOCALAPPDATA%\Packages\<family>\LocalCache\Roaming\Claude` (`@836946`
`function mze(e,t,r,i){return n.join(e,"Packages",t,"LocalCache",r,i)}`). Which one the observed
MSIX install used, and what it left in the others, is in
[Standard build on Windows](#standard-build-on-windows). The enterprise build (`Claude-3p`) is
unchanged (`index.pre.js@809294`
`function iQ()`). `CLAUDE_USER_DATA_DIR` moves userData and logs anywhere (`index.pre.js@887429`);
that cannot be a catalog line.

#### `spaces-present` link key

`spaces-present/<acct 8 hex>/<org 8 hex>/link-key` at the userData root
([Credentials](#credentials)) is the HMAC key that signs Cowork space links and folder grants,
declared at:

- `app.asar:.vite/build/index.chunk-Cyt2WTko.js@5407180`
  `var JUi="link-key",YUi="link-key.unreadable",XUi=6048e5,...,QUi=32,$Ui="claude-space-link-v1\0",eWi="claude-space-folder-v1\0"`

The class that holds it computes `createHmac("sha256",this.key)` over `$Ui`, the space id and a
link's `url`, `title` and `provider`, or over `eWi` and a folder path (`macOfFolder`), and checks
with `timingSafeEqual` (`verifies`, `verifiesFolder`). The key is 32 random bytes
(`randomBytes(QUi)`) stored as `safeStorage.encryptString` of their base64 form and read back with
`safeStorage.decryptString`; when safe storage is unavailable the session runs `keyless` and no link
or folder is verified. It is OS-encrypted like `.audit-key`.

#### `git-shadow`

`git-shadow/` at the userData root holds the Code tab's private git state, not transcripts
(`app.asar:.vite/build/index.chunk-BCfH2EWk.js@17777`
`function Ke(){return t.aD()??r.default.join(i.app.getPath("userData"),"git-shadow")}`, created with
`mkdir(Ke(),{recursive:!0,mode:448})`): a `scratch/` child used as the `privateIndexDir` for git
operations, whose entries older than `Je=216e5` ms (six hours) are removed, and
`cli-temp-roots.json` (`index.chunk-Cyt2WTko.js@3642519` `var $rr="cli-temp-roots.json",MH=256`), a
JSON array of up to 256 Claude Code CLI temp roots, read back with path checks; the same code
manages the shadow git dirs (`shadow-of`, `*-gitdir`, `*-gitconfig`, `git-crypt`) inside those
roots. Observed on Linux: one entry, `/tmp/claude-<uid>`, beside a
`cli-temp-roots.json.dirty.copy.<16 hex>` of the same size; that root held per-project slug
directories with empty `scratchpad/` and `tasks/` dirs. The roots are outside every home and are
not collected
([`collectors/docs/coverage.md`](../collectors/docs/coverage.md#claude-desktop-data-directories)).

### Format versions

Only `2.31226.1` was read. Version-sensitive points a reader of another release should re-check: the
short directory names (introduced behind a flag, `zZt`), the session-type list, the
`CLAUDE_CODE_PROJECT_DIR_NAME` slug, and the audit HMAC construction. `git-worktrees.json` has a
`schemaVersion` and migrates older stores keyed by session id to stores keyed by worktree name. The
legacy single `local-sessions.json` at the userData root is migrated into the per-org tree on start.
Older installs may have `audit.jsonl` lines without `_audit_hmac` (safe storage unavailable) and
without `_audit_redactions`.

### Secrets

- Cowork session record: `emailAddress` and `accountName` (identity, not credentials, but personal
  data to redact on sharing); `systemPrompt` and its siblings may embed the person's memory and
  instructions; `otelConfig` may hold OTLP exporter `headers`, which the managed-config schema
  treats as sensitive (`ss("otlpHeaders","no exporter headers will be sent")`, `redact:"presence"`);
  whether the headers are kept in the record was not determined. `remoteMcpServersConfig[].url` may
  embed a token in the query string.
- `audit.jsonl` and the transcripts carry full tool inputs and outputs, which can include any secret
  the agent read; base64 images and documents are hashed, nothing else is redacted.
- Code-tab record: `sshConfig` content was not determined; treat it as possibly naming a key file.
  `bridgeSessionUrl` is runtime-only and never written to the record;
  `remoteControlSpawn.ccrSessionId` is an identifier, not a credential.
- Each session dir is the CLI's `CLAUDE_CONFIG_DIR`, so the CLI may write `.claude.json` there
  (`index.chunk-Cyt2WTko.js` `CLAUDE_CONFIG_DIR?(0,n.join)(jj(),".claude.json")`). The OAuth token
  is passed through the environment and a callback (`getOAuthToken`), so a `.credentials.json` in
  the session dir was not seen in the code path read; not determined.
- `config.json`, `Local State`, `Cookies`, `host-creds-*.json`, `ccd-session-secrets/`,
  `.audit-key` and `link-key` are credential files ([Credentials](#credentials)).

### Parser plan

A `claude-desktop` parser that reuses the claude-code reader for the transcripts (as little-coder
reuses pi's), plus small readers for the session records, `audit.jsonl` and the two index files.
Select by `artifact.rel` relative to the userData root (any of the catalog bases):

```text
TRANSCRIPT_PATTERN       (local-agent-mode-sessions|claude-code-sessions)/[^/]+/[^/]+/(agent/)?[^/]+/\.claude/projects/[^/]+/[^/]+\.jsonl$
STAGED_PATTERN           claude-code-sessions/[^/]+/[^/]+/imported-staging/[^/]+\.jsonl$
AUDIT_PATTERN            local-agent-mode-sessions/[^/]+/[^/]+/(agent/)?[^/]+/audit\.jsonl$
RECORD_PATTERN           (local-agent-mode-sessions|claude-code-sessions)/[^/]+/[^/]+/(agent/)?local_[0-9a-f-]{36}\.json$
SCHEDULED_TASKS_PATTERN  (local-agent-mode-sessions|claude-code-sessions)/[^/]+/[^/]+/scheduled-tasks\.json$
WORKTREE_PATTERN         (^|/)git-worktrees\.json$  (only under a claude-desktop base)
```

Subagent files under a transcript's `<session>/subagents/` follow the claude-code layout and go
through the same reader.

Not parsed: LevelDB stores, logs, `spaces.json`, memory Markdown, `rpm/`, caches,
`claude_desktop_config.json`.

#### Permission rows

The session record row of either kind
([issue 93](https://github.com/seanthegeek/doubleagent/issues/93)) ends with
` permission_mode=<permissionMode> effort=<effort>`, `-` for an absent key (the Cowork key list in
[Record schema](#record-schema) has `effortOverride` and `pinnedSpawnEffort` but no `effort`, so a
Cowork row usually reads `effort=-`). Then the per-session grants, all `system` rows with the
record's session and project:

| Record key | Row text |
| --- | --- |
| `alwaysAllowedReasons[]`, `sessionPermissionUpdates[]` | one row per entry, `permission grant: <key> <entry>` |
| `chromePermissionMode`, `chromeAllowedDomains`, `cuAllowedApps`, `cuGrantFlags`, `cuFlagsGrantedAt` (`chrome` the browser extension, `cu` computer use) | one row per key that holds a value (not absent, `""`, `[]` or `{}`), `permission grant: <key>=<value>` |

An entry or value that is not a string is its compact JSON, and each text ends with
`(as of <field>)`. The record keeps no time per grant, so the rows take `lastActivityAt` (the grant
was held by then; the record is written whole by `writeSessionToDisk`), else `createdAt`, and
`<field>` names which. The real installs showed both lists `[]` and `chromePermissionMode` a
string, and the bundle reading in [Record schema](#record-schema) did not cover these keys, so the
fixture's entries (a rule string, and an object shaped like the CLI's permission update
`{type, rules, behavior, destination}`) are invented ([Confidence](#confidence)).

Cowork grants ([issue 97](https://github.com/seanthegeek/doubleagent/issues/97)), after those rows
and at the same time with the same `(as of <field>)` suffix: `approvedToolNames`,
`webFetchAllowedUrls`, `egressAllowedDomains`, `userApprovedFileAccessPaths`, `enabledMcpTools` and
`fileDeleteApprovedMounts`. [Record schema](#record-schema) types them only as "arrays and objects",
without saying which key is which, so a list is one row per entry,
`permission grant: <key> <entry>`, an object one row per member,
`permission grant: <key>.<member>=<value>`, and any other non-empty value one row,
`permission grant: <key>=<value>`; the fixture's values are invented. Not read:
`remoteMcpServersConfig` (it can carry server credentials) and `chromePermsBeforeUnsupervised`.
`preferences.remoteChatFolderGrants` is not in the session record but in
`claude_desktop_config.json`, a secret-glob file (its `mcpServers.*.env` can embed API keys), so the
parser does not read it and its folder grants get no row.

The audit log's permission events give `permission request:`, `permission response:` and
`permission auto-approved:` rows ([Audit log](#audit-log)), and `remote-session-spaces.json` gives
`permission setting:` rows ([Remote session folders](#remote-session-folders)).

#### Model rows

The audit log's `system` `init` line gives `model: <model>` the first time a session's model is
seen, and an `assistant` line whose `message.model` differs from the session's last one is preceded
by `model: <old> -> <new>`, one tracker per session id in the file
([issue 96](https://github.com/seanthegeek/doubleagent/issues/96) and
[issue 97](https://github.com/seanthegeek/doubleagent/issues/97)); `model_refusal_fallback` lines
are printed as recorded ([Audit log](#audit-log)). The session record and scheduled-task rows fill
the `model` column. The transcripts' model rows are the claude-code reader's
([Model in `claude-code.md`](claude-code.md#model-rows)).

#### Past-use rows and secondary sources

The transcript is the primary store: the CLI writes it and the desktop itself reads it back to show
the session (the lookup in [Transcript stores](#transcript-stores)). `audit.jsonl` is a secondary
source of it ([issue 122](https://github.com/seanthegeek/doubleagent/issues/122)): the desktop
writes the same SDK messages beside it, append-only, so its `user`, `assistant` and tool lines
repeat the transcript's turns. Every line is read on every run, whether or not a transcript exists,
and the timeline's deduplication drops the rows the transcript also yields. What the audit log
survives is in [What survives a delete](#what-survives-a-delete). The permission events, the
`result` lines and the `init` line have no transcript twin and are always kept.

The default 2-second window applies. Rows the window or the text cannot pair, and that the timeline
therefore shows twice: a Bash `tool_use` whose transcript record has `wireToolInputs` (the command
as sent, which the claude-code reader prefers; the audit line has only `input`), a tool output the
transcript persisted to a file, a `user` text the transcript labels (a slash command, a task
notification) and the audit log does not, and a prompt the CLI recorded more than 2 seconds after
the desktop stamped it.

If a Cowork record has `initialMessage`, also yield it as a `user` row, always, with an empty
`timestamp_utc`: the record keeps no time for the prompt (`createdAt` is the session's). The record
is a secondary source ([issue 122](https://github.com/seanthegeek/doubleagent/issues/122)), so the
row drops when the audit log or the transcript holds the same prompt in the same session, and
survives when both are gone; the record's `system` rows have no twin and are always kept. A Code-tab
record's `initialMessage` is yielded at `createdAt` only when the session's transcript was not
collected, because that transcript is a `~/.claude/projects/` file of agent `claude-code` and the
rule pairs rows of one agent only.

#### Sync, relay and origin rows

No sync, relay or origin marker is determined for the desktop's own files. The Code-tab record row
carries the persisted Remote Control state (`remote_control=`,

`bridges=`, `spawned_remotely=`, `steered_remotely=` and the update-quit row, [Session
records](#session-records)); a chat that ran as a remote Cowork session is seen only through
`remote-session-spaces.json` ([Remote session folders](#remote-session-folders)).

#### Scratch directory rows

Not stated; the Code tab's `git-shadow/` and the CLI temp roots it lists are described in
[`git-shadow`](#git-shadow).

#### Deletion rows

Not stated; what the desktop's own delete or archive removes is not determined, and no deletion
marker is stated ([What survives a delete](#what-survives-a-delete)).

#### Bad-line rows

A line that does not parse becomes its own `system` row after the file's other rows, carrying the
line in full (`parser: unparseable line (<decoder message>): <line>`,
[issue 136](https://github.com/seanthegeek/doubleagent/issues/136)), and reading continues. The
transcripts go through the claude-code reader and its bad-line rows
([Bad lines in `claude-code.md`](claude-code.md#bad-line-rows)); a `remote-session-spaces.json` that
is not a JSON object is one whole-file row ([Remote session folders](#remote-session-folders)). The
fixture adds a truncated last line written with `write_bad_line` ([Fixtures](#fixtures)).

A transcript or `audit.jsonl` that cannot be read is one whole-file row
`parser: <file> could not be read (<reason>)` through `error_rows`, `<file>` its name. A session
record, `scheduled-tasks.json`, `git-worktrees.json` or `remote-session-spaces.json` that cannot be
read gives the same row, `source_line` 0 and no time. One that reads but is not a JSON object keeps
its own row at `source_line` 1: `parser: session record is not a JSON object`,
`parser: unreadable scheduled-tasks.json`, `parser: unreadable git-worktrees.json` or
`parser: remote-session-spaces.json is not a JSON object`.

#### Fixtures

Synthetic fixture records:

- `Library/Application Support/Claude/local-agent-mode-sessions/aaaaaaaa-0000-4000-8000-000000000001/bbbbbbbb-0000-4000-8000-000000000002/local_c0c0c0c0-1111-4222-8333-444444444444.json`:

```json
{
  "sessionId": "local_c0c0c0c0-1111-4222-8333-444444444444",
  "processName": "quiet-river-1234",
  "cliSessionId": "d1d1d1d1-5555-4666-8777-888888888888",
  "cwd": "/sessions/quiet-river-1234",
  "userSelectedFolders": ["/srv/proj"],
  "createdAt": 1790762400000,
  "lastActivityAt": 1790762460000,
  "model": "claude-fable-5-1",
  "permissionMode": "default",
  "isArchived": false,
  "title": "Summarise the test failures",
  "vmProcessName": "quiet-river-1234",
  "initialMessage": "why does the test fail?",
  "sessionType": "agent",
  "emailAddress": "user@example.invalid"
}
```

- `.../local_c0c0c0c0-1111-4222-8333-444444444444/.claude/projects/session/d1d1d1d1-5555-4666-8777-888888888888.jsonl`:
  the claude-code fixture lines with `"entrypoint":"local-agent"`,
  `"cwd":"/sessions/quiet-river-1234"` and `"sessionId":"d1d1d1d1-5555-4666-8777-888888888888"`.

`.../local_c0c0c0c0-1111-4222-8333-444444444444/audit.jsonl` (HMAC values are placeholders; a
fixture need not verify):

```json
{"type":"user","uuid":"00000000-0000-4000-8000-0000000000a1","session_id":"d1d1d1d1-5555-4666-8777-888888888888","parent_tool_use_id":null,"client_platform":"desktop_app","timestamp":"2026-10-01T10:00:00.000Z","message":{"role":"user","content":[{"type":"text","text":"why does the test fail?"}]},"_audit_timestamp":"2026-10-01T10:00:00.004Z","_audit_hmac":"0000000000000000000000000000000000000000000000000000000000000001"}
{"type":"system","subtype":"init","session_id":"d1d1d1d1-5555-4666-8777-888888888888","cwd":"/sessions/quiet-river-1234","model":"claude-fable-5-1","_audit_timestamp":"2026-10-01T10:00:01.000Z","_audit_hmac":"0000000000000000000000000000000000000000000000000000000000000002"}
{"type":"assistant","uuid":"00000000-0000-4000-8000-0000000000a2","session_id":"d1d1d1d1-5555-4666-8777-888888888888","parent_tool_use_id":null,"message":{"model":"claude-fable-5-1","role":"assistant","content":[{"type":"tool_use","id":"toolu_a1","name":"Bash","input":{"command":"npm test"}}]},"_audit_timestamp":"2026-10-01T10:00:03.000Z","_audit_hmac":"0000000000000000000000000000000000000000000000000000000000000003"}
{"type":"system","subtype":"permission_request","uuid":"00000000-0000-4000-8000-0000000000a3","session_id":"d1d1d1d1-5555-4666-8777-888888888888","tool_name":"Bash","tool_input":{"command":"npm test"},"_audit_timestamp":"2026-10-01T10:00:03.100Z","_audit_hmac":"0000000000000000000000000000000000000000000000000000000000000004"}
{"type":"system","subtype":"permission_response","uuid":"00000000-0000-4000-8000-0000000000a3","session_id":"d1d1d1d1-5555-4666-8777-888888888888","tool_name":"Bash","decision":"once","granted":true,"_audit_timestamp":"2026-10-01T10:00:05.000Z","_audit_hmac":"0000000000000000000000000000000000000000000000000000000000000005"}
{"type":"user","uuid":"00000000-0000-4000-8000-0000000000a4","session_id":"d1d1d1d1-5555-4666-8777-888888888888","parent_tool_use_id":null,"message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_a1","content":"1 failing"}]},"_audit_timestamp":"2026-10-01T10:00:09.000Z","_audit_hmac":"0000000000000000000000000000000000000000000000000000000000000006"}
{"type":"result","subtype":"success","duration_ms":9000,"duration_api_ms":4000,"is_error":false,"num_turns":2,"session_id":"d1d1d1d1-5555-4666-8777-888888888888","uuid":"00000000-0000-4000-8000-0000000000a5","_audit_timestamp":"2026-10-01T10:00:12.000Z","_audit_hmac":"0000000000000000000000000000000000000000000000000000000000000007"}
{"type":"user","uuid":"00000000-0000-4000-8000-0000000000a6","session_id":"d1d1d1d1-5555-4666-8777-888888888888","parent_tool_use_id":null,"client_platform":"desktop_app","timestamp":"2026-10-01T10:00:20.000Z","message":{"role":"user","content":[{"type":"text","text":"thanks"}]},"_audit_timestamp":"2026-10-01T10:00:20.003Z","_audit_hmac":"0000000000000000000000000000000000000000000000000000000000000008"}
```

The first prompt has no twin in the transcript beside it; the last prompt, `thanks`, and the `init`
line's `model: claude-fable-5-1` repeat the transcript's last `user` record and its first `model:`
row, so the timeline drops those two audit rows
([issue 122](https://github.com/seanthegeek/doubleagent/issues/122)).

The `decision` values are `deny`, `once`, `always` and `scheduled`
([Record schema](#record-schema)). Add a second session dir with only `audit.jsonl` (no
`.claude/projects`) for a session whose transcript is gone, and a truncated last line written with
`write_bad_line`.

- `Library/Application Support/Claude/claude-code-sessions/aaaaaaaa-0000-4000-8000-000000000001/bbbbbbbb-0000-4000-8000-000000000002/local_e2e2e2e2-9999-4aaa-8bbb-cccccccccccc.json`:

```json
{"sessionId":"local_e2e2e2e2-9999-4aaa-8bbb-cccccccccccc","cliSessionId":"11111111-2222-4333-8444-555555555555","cwd":"/srv/proj","originCwd":"/srv/proj","branch":"main","createdAt":1790762400000,"lastActivityAt":1790762520000,"model":"claude-fable-5-1","isArchived":false,"title":"Fix the failing test","permissionMode":"default"}
```

`.../scheduled-tasks.json`:

```json
{"scheduledTasks":[{"id":"daily-report","cronExpression":"0 9 * * 1-5","enabled":true,"filePath":"/home/user/Claude/Scheduled/daily-report/SKILL.md","createdAt":1790762400000,"cwd":"/srv/proj"}],"recordedSkips":{},"sundayAliasBoundaryStamped":true,"dayFieldsOrBoundaryStamped":true}
```

`Library/Application Support/Claude/git-worktrees.json`:

```json
{"schemaVersion":2,"worktrees":{"brave-otter":{"name":"brave-otter","path":"/srv/proj/.claude/worktrees/brave-otter","leasedBy":"local_e2e2e2e2-9999-4aaa-8bbb-cccccccccccc","baseRepo":"/srv/proj","branch":"claude/brave-otter","sourceBranch":"main","createdAt":1790762401000}},"untrackedDirGc":{"cwds":{},"roots":{},"sightings":{}}}
```

`schemaVersion` 2 is the current value in the bundle (`index.chunk-3ucQSJ7U.js`, `Bc=2`); the
worktree path layout is illustrative.

Noise the parser must not want: `local-agent-mode-sessions/<acct>/<org>/spaces.json`,
`cowork_settings.json`, `rpm/manifest.json`, `.audit-key`, and `claude_desktop_config.json` at the
userData root.

#### Transcripts

Rows exactly as the claude-code parser yields them, with `agent` `claude-desktop`. Two overrides,
both read from the sibling session record (`<org dir>/[agent/]local_<uuid>.json`, matched by the
session dir name or its eight-hex prefix):

- `project_path`: the first `userSelectedFolders` entry when present, because the JSONL `cwd` is the
  guest path `/sessions/<name>` in VM mode; else the JSONL `cwd`. Say in the docstring that it is
  taken from the record.
- `session_id`: keep the JSONL `sessionId` (= record `cliSessionId`), so rows join to the file name
  and to the Code-tab record.

#### Audit log

| Audit line | `turn_type` | `text` | other columns |
| --- | --- | --- | --- |
| `system` `permission_request` | `system` | `permission request: <tool_name> <compact tool_input>` | `tool_name`, `timestamp_utc` from `_audit_timestamp` |
| `system` `permission_response` | `system` | `permission response: <tool_name> <decision> granted=<bool>` | `tool_name` |
| `system` `permission_auto_approved` / `_denied` | `system` | `permission auto-approved: <tool_name> (<source or scheduled_task_id>)` | `tool_name` |
| `result` | `system` | `result: <subtype> turns=<num_turns> error=<is_error>` | |
| `system` `init` | `system` | `model: <model>` the first time a session's model is seen ([issue 96](https://github.com/seanthegeek/doubleagent/issues/96) and [issue 97](https://github.com/seanthegeek/doubleagent/issues/97)) | `model` the line's `model`; one tracker per session id in the file |
| `system` `model_refusal_fallback` | `system` | `model_refusal_fallback: <compact JSON of the line less type, subtype, uuid, session_id, timestamp and the _audit_ keys>` | the subtype is in the bundle loop above, but its fields are not recorded, so the parser prints them as they are rather than a `model: <old> -> <new>` row |

The turns, from every audit log: `user` lines with `client_platform` `desktop_app` and no
`isSynthetic` become `user` rows (`text_of(message.content)`), synthetic ones `system`
(`context: notification:`), `assistant` content blocks become `assistant`, `tool_use` (`tool_name` =
`name`, `tool_use_id` = `id`) and `thinking` (opt-in), `user` `tool_result` blocks become
`tool_result` rows joined on `tool_use_id`, `system` `init` becomes one `system` row (after its
`model:` row), and an `assistant` line whose `message.model` differs from the session's last one is
preceded by `model: <old> -> <new>` with `model` the new id. `timestamp_utc` is the line's
`timestamp` when present, else `_audit_timestamp`; `model` is `message.model`. `session_id` is the
record's `cliSessionId`, else the line's `session_id`; `project_path` is the record's
`userSelectedFolders[0]`, else the first `init` line's `cwd`, else the record's `cwd`
([Joins and timestamps](#joins-and-timestamps)); `source_line` is the 1-based line number.

#### Session records

One `system` row per record at `createdAt`: `desktop session: <sessionType> "<title>"` for Cowork,
`desktop code session: "<title>"` for the Code tab, with `session_id` = `cliSessionId`,
`project_path` = `userSelectedFolders[0]` or `originCwd` or `cwd`, `git_branch` = `branch` (Code tab
only), `model`. A Code-tab row's text continues after the title with
`remote_control=<remoteControlUserEnabled>`, `bridges=<len(bridgeSessionIds)>`,
`spawned_remotely=<remoteControlSpawn is a non-empty object>` and
`steered_remotely=<steeredByRemoteClient>`, separated by spaces, absent keys reading `false` and
`0`, always appended. When `remoteControlLiveAtUpdateQuit` is an object with a numeric `at`, a
second `system` row at that time reads `remote control live at update quit`, same session and
project. Not read: `remoteControlUserRequested`, `remoteControlUserToggled`,
`remoteControlAutoEligible`, `remoteControlDescendant`, `peerMessaged`, `cloudSessionId`, the spawn
object's fields and the update-quit `accountId`; `sessions.jsonl` gets no column for them until a
real Remote Control session shows them set. Whether a session was under Remote Control at collection
time is the `claude-code` registry row's `bridge=` value, joined on `cliSessionId`.

If `error` is set, one `system` row `session error: <errorCategory>: <error>` at `errorAt`.
`source_line` is `1`.

#### Scheduled tasks

One `system` row per task at `createdAt`, with `project_path` = `cwd` or `userSelectedFolders[0]`
and `model` the task's `model`; `source_line` is the array index plus one. Its text, with `-` for an
absent key:

- `scheduled task: <id> <cronExpression or fireAt as ISO> enabled=<bool> permission_mode=<permissionMode> model=<model>`

#### Remote session folders

`remote-session-spaces.json`: one `system` row per `entries[].folders[]` entry,
`permission setting: entries.<sessionId>.folders=<folder> (from <file>, file mtime)`
(`entries.folders` when the entry has no `sessionId`), at the file's modification time (the manifest
`mtime` when there is one), with `project_path` the folder and `session_id` empty, because the
entry's `sessionId` is the server-side id, not a CLI session; `source_line` 1. A file that is not a
JSON object is one `system` row `parser: remote-session-spaces.json is not a JSON object`.

#### Worktrees

One `system` row per entry at `createdAt`:
`worktree created: <path> branch=<branch> from=<sourceBranch>`, `project_path` = `baseRepo`,
`git_branch` = `branch`, `session_id` empty (the `leasedBy` value is the desktop id, not the CLI id;
put it in the text).

## Confidence

- High: the macOS layout of the standard build (real install, 2026-10-11: data directory, logs,
  `Partitions/`, `claude-code/`, `claude-code-vm/`, `vm_bundles/` and the exclusion sizes in
  [Exclusions](#exclusions)); the Linux layout (official `.deb`, real install, 2026-10-11: data
  directory and logs under `~/.config/Claude`, no XDG data, cache or state use, the downloaded CLI
  under `claude-code/`); the MSIX build's data directory under `Packages`, its logs in
  `%LOCALAPPDATA%\Claude\logs`, the empty `Claude-Data` and `Claude-3p` directories, and the root
  file names there (real install, 2026-10-10); app-dir and log locations per OS (docs), 3p layout
  and file names (docs), `config.json` token keys and OSCrypt scheme (two independent community
  reports plus the docs statement on secure storage); the Code-tab record, its join to the
  transcript by `cliSessionId` and its project path (real installs on all three platforms);
  `remote-session-spaces.json` as the only local trace of a file-writing chat (all three).
- Medium: the `AnthropicPBC.Claude_fnn82j28hfe8t` family and the `Claude-Data\<family>` data
  directory (bundle only, not observed in use); that every 3p subdirectory name also appears in the
  standard build (docs say the two share a layout but document only 3p); the Linux decryption key
  source (keyring or the fixed key; the observed `Local State` names none,
  [Credentials](#credentials)); the Linux Cowork VM layout (`vm_bundles` was not created in a
  nested-virtualisation guest); the Cowork session record, `audit.jsonl`, imported transcripts and
  `scheduled-tasks.json` entries, which rest on the bundle and the synthetic fixture, since no real
  install wrote a local Cowork session or a scheduled task; the `true` values of the Remote Control
  fields, which rest on the bundle and the fixture, since every real install showed them `false` or
  absent.
- Not determined: where an unpackaged (non-MSIX) Windows install puts its logs today; the IndexedDB
  schema and whether the claude.ai query cache holds message bodies ([claude.ai
  chats](#claudeai-chats));
  which build the `Claude Nest` directories belong to
  ([Enterprise (3p) build](#enterprise-3p-build)); whether audit and transcript `uuid` values are
  equal and how far apart their times run ([Joins and timestamps](#joins-and-timestamps)); the inner
  shape of the `alwaysAllowedReasons[]` and `sessionPermissionUpdates[]` entries and of the Cowork
  grant keys, for which the fixture's values are invented ([Permission rows](#permission-rows));
  what the desktop's delete or archive changes on disk
  ([What survives a delete](#what-survives-a-delete)).
