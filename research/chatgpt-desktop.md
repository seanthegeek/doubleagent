# chatgpt-desktop

OpenAI's ChatGPT desktop app (catalog agent `chatgpt-desktop`) is a closed-source desktop app: a
Chromium shell hosting a web interface packaged the way an Electron app is (Electron is the desktop
framework that runs a web user interface in a bundled Chromium; the evidence is in
[Source and evidence](#source-and-evidence)). On Windows and macOS it bundles Codex, OpenAI's coding agent,
whose state goes to `~/.codex` and is collected as `codex-cli`; that half of the app is described in
[`codex-cli.md`](codex-cli.md), under
[Codex Desktop](codex-cli.md#codex-desktop-inside-the-chatgpt-app). The app's prompt box, the
composer, has a mode dropdown with two entries (build 26.1013.40934 on macOS, 2026-10-18):
**Codex** mode starts a local Codex thread, whose rollout (Codex's JSONL transcript) goes to
`~/.codex/sessions` and is read by the `codex-cli` parser; the **ChatGPT** entry lists the account's
conversations on chatgpt.com, OpenAI's web chat service. A ChatGPT-mode conversation keeps its text
on OpenAI's servers: on the Windows and macOS installs checked the app keeps no local chat
transcript, and a reply to a ChatGPT conversation from inside the app (build 26.1013.40934 on
macOS, 2026-10-18) left none either ([Transcript stores](#transcript-stores)). The agent is parsed
([Issue 49](https://github.com/seanthegeek/doubleagent/issues/49) tracks the parser): the one local
trace of the account's ChatGPT conversations, the `chatgpt` rows of the app's own database
`~/.codex/sqlite/codex-dev.db` `local_thread_catalog`, is what the `chatgpt-desktop` parser turns
into timeline rows.

## Source and evidence

Closed source, so there are no commit links, and the app's client code is
not in the Codex repository: a search of the Codex source for
`codex-dev.db`, `local_thread_catalog`, `conversation_origin` and
`chatgpt_async_status` finds nothing
([Parser plan in `codex-cli.md`](codex-cli.md#parser-plan)).
Windows and macOS evidence is a **real install** of each (below); every claim about those
installs was read for file names, sizes, table and column names, value types, enum values and
row counts only, and no conversation title or id from an install is reproduced here. The former
native macOS app and the previous Windows package rest on **DFIR write-ups and press** (DFIR is
digital forensics and incident response), and Linux has no app. No official OpenAI
documentation of on-disk paths was found.

The app's build, as installed. On Windows (2026-10-15) the vendor's `ChatGPT Installer.exe`
(813,088 B) installs the app from the Microsoft Store as an MSIX package (the Windows app package
format; each package gets a per-user data directory under `%LOCALAPPDATA%\Packages\<package family
name>`): package `OpenAI.Codex`, version `26.1007.9446.0`, package family name
`OpenAI.Codex_2p2nqsd0c76g0`, display name `ChatGPT`, installed under `C:\Program
Files\WindowsApps\OpenAI.Codex_26.1007.9446.0_arm64__2p2nqsd0c76g0`. The `OpenAI.ChatGPT-Desktop_*`
package family the write-ups describe does not exist on that install. Its `AppxManifest.xml` (the
package's manifest) declares two applications, `App` (`app/ChatGPT.exe`) and
`CodexCoreCommandRunner` (`app/resources/codex-command-runner.exe`), and the capabilities
`runFullTrust`, `unvirtualizedResources`, `packagedServices`, `localSystemServices`,
`internetClient`, `graphicsCaptureProgrammatic` and `graphicsCaptureWithoutBorder`. `app/` holds
`ChatGPT.exe` (3.4 MB, product version `155.0.8059.40`), `chrome.dll` (311 MB, the same version),
`chrome_elf.dll`, `chrome_proxy.exe`, `chrome_pwa_launcher.exe`, `elevation_service.exe`,
`notification_helper.exe`, `Codex.exe` (20 KB) and `owl-shell-runtime.json`. `app/resources/` holds
`app.asar` (567 MB, the web app's archive), `owl-app.ini`, `owl-electron-app.json`, the bundled
Codex as `codex.exe` (282,678,064 B) and a Linux `codex` binary, `codex-code-mode-host.exe`,
`codex-command-runner.exe`, `codex-windows-sandbox-service.exe`, `codex-windows-sandbox-setup.exe`,
`rg.exe` (ripgrep), and the directories `cua_node/`, `plugins/`, `skills/` and `tectonic/`. On macOS
(2026-10-15) `/Applications/ChatGPT.app` (`CFBundleShortVersionString` `26.1007.71326`,
`CFBundleVersion` `20398`) has the bundle identifier **`com.openai.codex`**; its
`Contents/Frameworks/` holds `Codex Framework.framework` (`com.openai.codex.framework`, version
`155.0.8059.40`, and `155.0.8059.80` in build 26.1013.40934) and `Sparkle.framework` 2.9.1 (Sparkle
is the macOS update framework). `Contents/Resources/` holds `app.asar` (567,770,955 B), `codex-cli/`
(the bundled Codex, run from `codex-cli/CodexCLI.app/Contents/MacOS/codex` according to the app
log), `cua_node/`, `plugins/`, `skills/`, `rg`, `owl-app.ini`, `owl-electron-app.json` and
`scripting.sdef`; a dock-tile plug-in `com.openai.codex.dock-tile-plugin` is under
`Contents/PlugIns/`. So the macOS app is the same build as on Windows, not the native Swift app the
2024 write-ups describe, and nothing under `~/Library` carries the old `com.openai.chat` identifier.
Its version string is the `app_version` the Windows build's `codex-dev.db` records. In both 26.1007
builds the bundled Codex reports `originator` `Codex Desktop`, `cli_version` `0.162.0-alpha.17.2`.

The shell is a Chromium build rather than stock Electron: the Windows `app/` directory carries
Chrome's own `chrome.dll`, `chrome_elf.dll`, `chrome_proxy.exe`, `chrome_pwa_launcher.exe`,
`elevation_service.exe` and `notification_helper.exe`, and the macOS framework carries, under the
app's own name, the Chromium version of the Windows `chrome.dll`. The web layer is an
Electron-style app hosted in that shell: an `app.asar` archive, `owl-electron-app.json`,
`[electron-message-handler]` lines in the app log ([Per-user storage](#per-user-storage)) and the
`electron-persisted-atom-state` key of the Codex global state. "owl" is the shell's name in these
file names; nothing more about it was determined.

### Observed

- macOS, 2026-10-18 ([issue 49](https://github.com/seanthegeek/doubleagent/issues/49),
  [issue 149](https://github.com/seanthegeek/doubleagent/issues/149) and
  [issue 160](https://github.com/seanthegeek/doubleagent/issues/160)): ChatGPT `26.1013.40934`
  (bundle id `com.openai.codex`, `Codex Framework` `155.0.8059.80`) with the bundled Codex
  `0.163.0-alpha.10.2` (`originator` `Codex Desktop`), from 2026-10-18 18:15Z to 2026-10-19 00:43Z
  in the author's macOS lab VM of the 2026-10-15 round (macOS 26.6.2 build 25G83, arm64, 64-bit
  Arm, one user, no Python and no Xcode Command Line Tools). Unlike the round of 2026-10-15, the
  user performed one step at a time and a listing of names, sizes and modification times was taken
  and diffed after each. The steps, and what each wrote to the Codex stores under `~/.codex`, are in
  [Source and evidence in `codex-cli.md`](codex-cli.md#source-and-evidence); what they wrote of the app's
  own (times UTC): the installer disk image (848,907 B, downloaded 18:15:19Z) and the first launch
  (18:17:26Z) created every `~/Library` entry of [macOS, com.openai.codex](#macos-comopenaicodex)
  before any sign-in; while idle (18:21Z to 18:31Z) Chromium updated its components and Sparkle
  checked for updates (18:22:31Z); sign-in (18:31:28Z to 18:32:34Z) created the
  `codex-mcp-app-sandbox%3Ashared` storage partition (Chromium's isolated storage for an embedded
  web view) at 18:32:05Z, filled `executor-plugins/codex-app-tools/` and wrote three
  `local_thread_catalog` rows for the account's ChatGPT conversations
  ([Record schema](#record-schema)); adding the project `~/Demo` with the folder picker (18:48:52Z)
  recorded the folder in `Preferences/com.openai.codex.plist`; the relaunch after a quit
  (21:45:36Z) wrote `browser-sidebar-page-states.json`, and each launch made new sockets and a
  temporary directory ([Scratch directories](#scratch-directories)); a reply in ChatGPT mode to an
  existing chatgpt.com conversation (about 22:02Z to 22:15Z) wrote nothing locally but a composer
  draft ([Transcript stores](#transcript-stores)); and the delete of a Codex-mode thread with the
  app's own command (2026-10-19 00:36:02Z) removed its catalog row and left the `chatgpt` rows
  ([What survives a delete](#what-survives-a-delete)). The other steps (a plain question and a
  coding task in Codex mode, a declined and an approved tool call, a model switch with the Approve
  for me mode, in which a reviewer model rather than the user answers approval requests, a resume,
  a turn in Full access, the access level with no sandbox, a scheduled task, and remote control
  from two paired phones) wrote only to `~/.codex`. No step created a launchd agent or daemon or a
  keychain item.
- macOS, 2026-10-15 ([issue 113](https://github.com/seanthegeek/doubleagent/issues/113) and
  [issue 121](https://github.com/seanthegeek/doubleagent/issues/121)): ChatGPT `26.1007.71326`, the
  build above, observed
  2026-10-15 (16:25Z to 17:05Z) in the author's macOS lab VM, macOS 26.6.2 build 25G83, arm64
  (64-bit Arm), one user, no Homebrew, Node.js or Python. The user repeated the Windows round with
  no listing between tasks: install and first launch (16:26Z) created `~/Library/Application
  Support/Codex/` (the Chromium user-data directory, the directory holding a Chromium browser's
  profiles), the other `~/Library` entries of [macOS, com.openai.codex](#macos-comopenaicodex) and
  `~/.codex`; a plain chat and an image generation left no local transcript and three `chatgpt` rows
  in `local_thread_catalog` ([Other stores](#other-stores)); a task in `~/Demo`, a thread in a
  projectless output directory (the working directory the app gives a thread that belongs to no
  project) and a scheduled automation (16:39Z to 16:50Z) wrote Codex rollouts and an automation
  ([Source and evidence in `codex-cli.md`](codex-cli.md#source-and-evidence)), the automation's
  minute (16:44Z) also creating the `codex-browser-app` partition; and the standalone Codex CLI
  (16:55Z to 16:56Z) added its own layout to `~/.codex` ([macOS differences in
  `codex-cli.md`](codex-cli.md#macos-differences)). No launchd agent or daemon, no `~/Applications`
  entry and no login-keychain item with an OpenAI, Codex or ChatGPT identifier appeared
  ([Credentials](#credentials)).
- Windows, 2026-10-15 ([issue 113](https://github.com/seanthegeek/doubleagent/issues/113),
  [issue 114](https://github.com/seanthegeek/doubleagent/issues/114),
  [issue 115](https://github.com/seanthegeek/doubleagent/issues/115) and
  [issue 116](https://github.com/seanthegeek/doubleagent/issues/116)): ChatGPT `26.1007.9446.0`, the
  `OpenAI.Codex`
  package above, observed 2026-10-15 in the author's Parallels lab VM, Windows 11 ARM64 (ARM64 is
  the 64-bit Arm architecture; `ver` reports `10.0.26300.9550`), one user, no WSL (Windows
  Subsystem for Linux) distribution. No listing was taken between the tasks of each survey, so
  which file each task wrote is by modification time only (times UTC). Install and first launch
  (14:09Z to 14:12Z) created the package's data directory, `~\.codex`, the binaries under
  `%LOCALAPPDATA%\OpenAI\Codex` and the runtime under `~\.cache\codex-runtimes`
  ([Per-user storage](#per-user-storage)); onboarding (14:24Z to 14:26Z) set up the Codex sandbox
  ([Codex Desktop in `codex-cli.md`](codex-cli.md#codex-desktop-inside-the-chatgpt-app)) and wrote
  a note on the Desktop ([Project-local files](#project-local-files)); two local Codex threads
  (14:26Z, and 14:30Z to 14:32Z in `~\Desktop\Demo`, which wrote eleven files into
  `Desktop\Demo\hello-worlds\`) wrote rollouts
  ([Source and evidence in `codex-cli.md`](codex-cli.md#source-and-evidence)); a plain chat and an
  image generation in the ChatGPT chat surface left no local transcript
  ([Transcript stores](#transcript-stores)); and a first attempt at an automation failed with an
  unspecified error and wrote nothing. After a first survey (14:48Z) the user subscribed the account
  to ChatGPT Plus (the paid plan; about 15:22Z), which only refreshed `auth.json` and
  `models_cache.json`; created a scheduled automation (15:25Z to 15:26Z), which wrote Codex's
  automation files and in the package rewrote only the `app://` IndexedDB (a second blob),
  `Local State`, `Network\Cookies` and a few network and Sentry files; and connected a demo GitHub
  account with no repositories, which wrote nothing locally ([Credentials](#credentials)). A second
  survey followed at 15:30Z, and the standalone Codex CLI 0.162.1 was installed beside the app at
  15:38Z ([Standalone CLI on Windows in `codex-cli.md`](codex-cli.md#standalone-cli-on-windows)).
  Not exercised: a resumed thread, a denied tool call, an "allow always" grant, a model switch
  within a thread, remote control, an automation run.
- Former native macOS app (`com.openai.chat`): not observed; the evidence is write-ups
  ([Former native macOS app](#former-native-macos-app)).
- Previous Windows package (`OpenAI.ChatGPT-Desktop_2p2nqsd0c76g0`): not observed; the evidence is
  a write-up ([Windows, previous package](#windows-previous-package)).
- Linux: no official app.

### Source reviews

None.

## Collection

### Per-user storage

No environment variable relocates the data on any platform.

#### Scratch directories

The app's own working and temporary directories, apart from the
profile and package directories of the subsections below:

- `~/Documents/Codex/<date>/<slug>/{work,outputs}` (on Windows
  `~\Documents\Codex\<date>\<letter>\`, empty on the install): the
  projectless output directory, the working directory the bundled Codex
  is given for a thread with no project, collected as `codex-cli`
  ([Scratch directories in `codex-cli.md`](codex-cli.md#scratch-directories)).
- Windows: `%LOCALAPPDATA%\Temp\` held 42 `.tmp<6 chars>` directories
  (the sandbox's and plugins' scratch), three `<uuid>.tmp.wav` and four
  `<uuid>.tmp` files; not collected ([Exclusions](#exclusions)).
- macOS, outside `~/Library` and `~/.codex` (ChatGPT 26.1013.40934,
  2026-10-18): per-launch sockets under `/private/tmp/codex-app-tools/` and
  `/private/tmp/codex-browser-use/` (two `<uuid>.sock` files there from
  sign-in), outside every home, so no catalog line can name them, and a
  `com.openai.codex.<6 chars>/` directory made at each launch in the
  user's temporary directory under `/private/var/folders`.

#### Windows, OpenAI.Codex package

Observed. The package's data directory is
`<pkg>` = `%LOCALAPPDATA%\Packages\OpenAI.Codex_2p2nqsd0c76g0`
(`AppData/Local/Packages/OpenAI.Codex_*` in the catalog). The standard
MSIX subdirectories `AC\`, `AppData\`, `LocalState\`, `RoamingState\`
and `TempState\` are empty; `SystemAppData\Helium\` is Windows' own. The
app writes:

| Path under `<pkg>` | What it holds | Catalog |
| --- | --- | --- |
| `LocalCache\Roaming\Codex\web\Codex\Default\` | The Chromium profile (the directory a Chromium browser keeps one user's state in), 682 files, 4.3 MB: `Preferences`, `Secure Preferences`, `History` (no `urls` or `visits` rows), `Top Sites`, `Web Data`, `Login Data` and `Login Data For Account` (no rows), `Network\Cookies`, LevelDB stores (LevelDB is the key-value store Chromium keeps web storage in) under `Local Storage\leveldb`, `Session Storage`, `Sync Data\LevelDB` and others, and `IndexedDB\app_-_0.indexeddb.leveldb` (IndexedDB is the database a web page keeps structured data in) with a 264,598-byte blob: the only origin is `app://`, the scheme the app's UI is served from. No `https_chatgpt.com_0` origin exists, unlike the previous package | collected |
| `...\Default\Partitions\codex-browser-app\`, `...\Default\Partitions\codex-mcp-app-sandbox%3Ashared\` | Two storage partitions (Chromium's isolated storage for an embedded web view), each a full profile of the same shape: the in-app browser the agent drives, and the sandbox in which MCP apps' web pages run (MCP is the Model Context Protocol, through which an agent calls external tools) | collected |
| `LocalCache\Roaming\Codex\web\Codex\Local State` | The user-data directory's state (the user-data directory is the directory holding a Chromium browser's profiles): `os_crypt.encrypted_key`, the profile list, and `variations_google_groups` keyed by the three profile and partition names | collected, secret |
| `...\web\Codex\Last Version`, `statsig-state.json`, `.codex-tpp-local-executor-installation-id` | The Chromium version string; the Statsig stable id (Statsig is a feature-flag service); an installation id (what "tpp" stands for is not determined) | collected |
| `...\web\Codex\` other children | `sentry\` (Sentry, the crash and event reporting service: `scope_v3.json` with 200 breadcrumbs, `session.json`), `Crashpad\` (Chromium's crash reporter, empty), `component_crx_cache\` (59 MB), `DeferredBrowserMetrics\` (4 MB), shader and GPU caches, `first_party_sets.db`, `segmentation_platform\`, `Last Browser`, `lockfile`, and twenty Chromium component directories such as `CertificateRevocation`, `OptimizationHints`, `WasmTtsEngine` (23 MB) and `WidevineCdm` (20 MB): 1,066 files and 169 MB in all with `Default` | not matched |
| `LocalCache\Local\Codex\Logs\<yyyy>\<mm>\<dd>\codex-desktop-<uuid>-<pid>-t<n>-i1-<hhmmss>-0.log` | The app's own log, one line per event: `<ISO 8601 UTC time> <level> <event or [component]> key=value ...`; 1,551 lines on the install, mostly `[AppServerConnection]` and `[electron-message-handler]`, naming the Codex binary paths and the app-server connection | collected |
| `Settings\settings.dat` and its `.LOG1`, `.LOG2`, `roaming.lock` | The package's MSIX settings hive (a registry hive Windows keeps per package) | collected |

Elsewhere in the profile (observed, not collected,
[Exclusions](#exclusions); the output and temporary directories are
under [Scratch directories](#scratch-directories)):

- `%LOCALAPPDATA%\OpenAI\Codex\` (2,371 files, 600 MB):
  `bin\<hash>\` copies of `codex.exe`, `codex-code-mode-host.exe`,
  `codex-command-runner.exe`, `codex-windows-sandbox-setup.exe` and
  `rg.exe` (the same bytes as in the package; the log's
  `copiedRuntimeLocation=true source=copied` says the app copies them
  out), and `runtimes\cua_node\<hash>\`, a Node.js runtime for the
  computer-use agent (CUA, the agent mode that drives the desktop).
- `~\.cache\codex-runtimes\codex-primary-runtime\` (22,306 files,
  1.27 GB): `dependencies\` with a Git for Windows tree, a PowerShell 7,
  Node.js with 201 packages and Python, `plugins\openai-primary-runtime\`
  (the documents, PDF, presentations, spreadsheets and template-creator
  plugins: manifests, `SKILL.md` files and assets) and `runtime.json`.
  The same plugin tree is under `~\.codex\plugins\cache`.
- The install also created the registry key
  `HKLM\Software\OpenAI\Codex\WindowsSandboxService` and a `codex` protocol handler under
  `HKCU\Software\Classes`, beside the Codex sandbox's service and accounts
  ([Codex Desktop in `codex-cli.md`](codex-cli.md#codex-desktop-inside-the-chatgpt-app));
  registry keys, services and accounts are the EDR's job (the endpoint detection and response tool
  the responder already has) and are not collected.
- No `AppData\Roaming\ChatGPT`, `AppData\Roaming\OpenAI`,
  `AppData\Local\ChatGPT`, `AppData\Local\Programs\ChatGPT`,
  `.config` or `.local` directory, and no reparse point (symlink or
  junction) under `.codex`, `.cache`, the package or
  `%LOCALAPPDATA%\OpenAI`.

`~\.codex` is the bundled Codex's home, collected as `codex-cli`; its
desktop-only files, among them the scheduled automations and the
ambient suggestions, are in
[Codex Desktop in `codex-cli.md`](codex-cli.md#codex-desktop-inside-the-chatgpt-app).

#### Windows, previous package

Write-up, not observed. The `OpenAI.ChatGPT-Desktop_2p2nqsd0c76g0`
package of 2024 and 2025, an Electron-style app whose data was under
`LocalCache\Roaming\ChatGPT\`
([garr3ttmjo/Digital-Forensic-Report-Writeups](https://github.com/garr3ttmjo/Digital-Forensic-Report-Writeups)
"ChatGPT Desktop Forensics", examined 2025-01-23;
[arXiv 2505.23938](https://arxiv.org/abs/2505.23938), October 2024
release, no exact paths). The full root is
`%LOCALAPPDATA%\Packages\OpenAI.ChatGPT-Desktop_2p2nqsd0c76g0\LocalCache\Roaming\ChatGPT\`,
with chat history in `IndexedDB\https_chatgpt.com_0.indexeddb.leveldb\`
(persisting between sessions, removed on logout), session ids and
timestamps in `Local Storage\leveldb\`, and the standard Chromium
siblings (garr3ttmjo). The package's `LocalState\` may hold app
settings (MSIX convention, not cited). Its catalog lines stay for
installs that still have it.

#### macOS, com.openai.codex

Observed. Under `~/Library`:

| Path under `~/Library` | What it holds | Catalog |
| --- | --- | --- |
| `Application Support/Codex/Default/` | The Chromium profile, 742 files, 10.9 MB, with the Windows profile's entries except that the cookie store is `Cookies` and `Cookies-journal` directly in the profile, not under `Network/` (`Network Persistent State`, `TransportSecurity`, `Reporting and NEL`, `Trust Tokens` sit in the profile too); `IndexedDB/` with the single `app_-_0` origin (292 KB), `Local Storage/leveldb`, `Session Storage`, and the partitions `Partitions/codex-browser-app` and `Partitions/codex-mcp-app-sandbox%3Ashared` | collected |
| `Application Support/Codex/Local State`, `Last Version`, `statsig-state.json`, `.codex-tpp-local-executor-installation-id`, `browser-sidebar-page-states.json` | The files beside the profile. `Local State` (6,317 B) has the Windows keys except `os_crypt`: on macOS Chromium keeps the cookie key in the login keychain (not observed there), so the file is not flagged. `browser-sidebar-page-states.json` is 25 B (`pages`, `version`) | collected |
| `Application Support/Codex/executor-plugins/codex-app-tools/` | A bundled MCP server for the app's own tools: `.codex-plugin/plugin.json`, `.mcp.json`, `server.mjs` (949,032 B, excluded), `scripts/launch_codex_app_tools_mcp` | collected |
| `Application Support/Codex/artifact-sessions/runtime-generations/` | Empty on the install | collected |
| `Application Support/Codex/` other children | `component_crx_cache/` (58 MB), the twenty Chromium component directories of the Windows build, `BrowserMetrics-spare.pma` (4 MB), `GraphiteDawnCache/` (6 MB), `GPUPersistentCache/`, `Crashpad/`, `sentry/`, `ChromeFeatureState`, `OriginTrials/`, `Safe Browsing/`, `first_party_sets.db`, `segmentation_platform/`, and the `SingletonLock`, `SingletonSocket`, `SingletonCookie` and `RunningChromeVersion` symlinks (Chromium's single-instance lock) | not matched |
| `Application Support/com.openai.codex/production-appcast-bootstrap.json` | 30 B, `backendAppcastEnabled` (bool): the Sparkle appcast (update feed) switch. Collected as `codex-cli` before the macOS round, now as `chatgpt-desktop`, since the bundle id that owns it is the ChatGPT app | collected |
| `Application Support/com.openai.codex.installer/` | `attribution.json` (289 B: `created_at_ms`, `install_id`, `installed_version`, `installer_session_id`, `installer_stable_id`, `installer_version`, `schema_version`) and `attribution-acknowledgment.json` (137 B: `schema_version`, `install_id`, `app_version`, `acknowledged_at_ms`): install identifiers | collected |
| `Logs/com.openai.codex/<yyyy>/<mm>/<dd>/codex-desktop-<uuid>-<pid>-t<n>-i1-<hhmmss>-0.log` | The app's own log, the same `key=value` form as on Windows; three files, 1,593 lines in the largest; components include `[AppServerConnection]`, `[electron-message-handler]`, `[tpp-local-executor]`, `[mcp-app-sandbox]` and `[sparkle]` | collected |
| `Preferences/com.openai.codex.plist`, `com.openai.codex.installer.plist`, `com.openai.sky.CUAService.plist` | The app's preferences (`DockIconPreference`, `LastRunAppBundlePath`, `NSOSPLastRootDirectory`, the open panel's last directory, so the last project folder picked, and the Sparkle `SU*` keys; the file grew from 328 to 1,088 B when a project folder was added on 2026-10-18), the installer's `installer_stable_id`, and the computer-use service's status-item and Statsig stable id keys | collected |
| `HTTPStorages/com.openai.codex/`, `HTTPStorages/com.openai.sky.CUAService/` and the `.binarycookies` beside each | The macOS URL-session stores (`httpstorages.sqlite` with `-wal` and `-shm`) and their cookie files (461 B and 673 B) | collected; cookies secret |
| `Group Containers/2DC432GLL2.com.openai.sky.CUAService/` | The group container (a directory macOS shares between an app and its helpers) of the computer-use service: the socket `IPC/computeruse.sock` and its `IPC/computeruse.sock.lock`, `Library/Application Support/Software/Analytics.db` (one `distinct_id` row), a `Library/Application Scripts` symlink, empty `Caches` and `Preferences` | collected |
| `Caches/Codex/Default/` (`Cache`, `Code Cache`, `Partitions`; 6.2 MB), `Caches/com.openai.codex/`, `Caches/com.openai.sky.CUAService/` | The HTTP cache the app keeps outside the profile on macOS, and the URL caches (`Cache.db`, `fsCachedData/`) | not matched |
| `Application Support/OpenAI/Codex/NativeMessagingHosts/` | Empty; the counterpart of the Windows `%LOCALAPPDATA%\OpenAI\Codex`, which here holds no binaries | not matched |

"sky" is OpenAI's internal name for the computer-use agent, read from
the bundle ids `com.openai.sky.CUAService` (an XPC service, a macOS
helper process) and the copy of `Codex Computer Use.app`
(`com.openai.sky.CUAService` 26.1005.1001425, 168 files, 76.6 MB) that
the app places in `~/.codex/computer-use/`. The `~/.codex` tree matches
Windows' with the differences listed in
[macOS differences in `codex-cli.md`](codex-cli.md#macos-differences).

On ChatGPT 26.1013.40934 (macOS, 2026-10-18) the layout was the same,
with these observations:

- Every `~/Library` entry above existed after the first launch, before
  sign-in, the profile's `Cookies` already 20 KB. The defaults domains
  (macOS preference domains) are `com.openai.codex`,
  `com.openai.codex.installer` and `com.openai.sky.CUAService`; the app
  installs no launchd agent or daemon.
- Before sign-in, while idle, Chromium fetched component updates under
  `Application Support/Codex/` (`WasmTtsEngine` 25 MB among them, none
  matched by a catalog line), and Sparkle's update check wrote
  `Caches/com.openai.codex/`, `HTTPStorages/com.openai.codex` and its
  `.binarycookies`.
- Sign-in created the `codex-mcp-app-sandbox%3Ashared` partition and
  filled `executor-plugins/codex-app-tools/`; the relaunch after a quit
  wrote `browser-sidebar-page-states.json`.

- `Application Support/Codex/sentry/scope_v3.json` (119 KB at first
  launch, beside `session.json` and `queue/`, and 49 to 119 KB over the
  round; not matched by a catalog line) holds `scope.breadcrumbs`
  (200 entries, each `{category, data, level, message, timestamp,
  type}`), `scope.user` (`account_id`, `authMethod`, `id`), `scope.tags`
  (`buildFlavor`, `bundle`, `host`, `sessionId` and others) and
  `event.release`. After the first chat the breadcrumb categories were
  `console` 109, `electron.net` 62 (`data` `{method, status_code, url}`),
  `app_state` 10, `ui.click` 7, `child_process` 3 (`data` `{spawnfile}`),
  `fetch` 3 and `ui.input` 2: a rolling record of the URLs the app
  called, the user's clicks and the processes it spawned, beside the
  account identity. It holds no token. The catalog leaves it out, like
  the rest of the user-data directory's crash-reporting state.
- `Application Support/Codex/artifact-sessions/runtime-generations/`
  holds empty directories created at first launch; collected.
- The profile has no `Cache` directory: the HTTP cache is in
  `Caches/Codex/Default/`, so the
  `Library/Application Support/Codex/Default/Cache` exclusion matched
  nothing. `Caches/com.openai.sky.CUAService/Cache.db` and
  `Caches/com.apple.nsurlsessiond/Downloads/com.openai.codex/` are
  caches too; none is catalogued.
- `component_crx_cache/` (29 MB at first launch) and `GraphiteDawnCache/`
  (5.6 MB) beside the profile are not matched, like the other component
  directories.
- The profile's web storage is `IndexedDB/app_-_0.indexeddb.leveldb`
  (the `app://` origin), `Local Storage`, `Session Storage`,
  `Service Worker`, `Sync Data` and `shared_proto_db`; no directory is
  named after a chatgpt.com origin.

#### Former native macOS app

Write-ups, not observed. The native Swift app, bundle identifier
`com.openai.chat`, kept its data in
`~/Library/Application Support/com.openai.chat/`; conversations were
plain JSON in `conversations-{uuid}/` until 1.2024.171 and are encrypted
since, in `conversations-v2-{uuid}/`
([pvieito.com, ChatGPT unprotected conversations](https://pvieito.com/2024/07/chatgpt-unprotected-conversations);
[9to5mac.com, 2024-07-03 ChatGPT Mac app report](https://9to5mac.com/2024/07/03)).
Its catalog line stays for hosts that still have it.

### Credentials

- Windows, `OpenAI.Codex` (observed): `Network\Cookies` and
  `Cookies-journal` in `Default` and in each partition (the Chromium
  cookie database), and `Login Data`, `Login Data For Account` (no rows
  on the install). They are encrypted with the key in the user-data
  directory's `Local State` `os_crypt.encrypted_key` (372 characters),
  itself wrapped with DPAPI, the Windows Data Protection API, which binds
  a secret to the user account. All are flagged `secret: true`. While
  the app runs it holds a byte-range lock on `Cookies` and
  `Cookies-journal`, on every LevelDB `LOCK` file and on the
  `Settings\settings.dat*` hive, so none of them can be read then.
- The bundled Codex's credentials are in `~\.codex`: `auth.json` (OAuth
  tokens), `.sandbox-secrets\sandbox_users.json` (the passwords of the
  two sandbox accounts, Windows only), `.codex-global-state.json`
  (ChatGPT conversation resume tokens, the account's ids and the prompt
  history) and, on macOS (build 26.1013.40934), the shell snapshots,
  which copy the login shell's exported environment; remote control's
  server token is kept where no search found it. See
  [Credentials in `codex-cli.md`](codex-cli.md#credentials).
- Windows, previous package (write-up): `Network\Cookies` with
  `Local State`; `Local Storage\leveldb` holds session identifiers. No
  plaintext token file reported.
- macOS, `com.openai.codex` (observed): `Default/Cookies*` and
  `Default/Partitions/<partition>/Cookies*`, `Login Data*` in each, and
  `Library/HTTPStorages/com.openai.codex.binarycookies` and
  `com.openai.sky.CUAService.binarycookies` are flagged `secret: true`.
  The Chromium cookie key is not in `Local State` on macOS; the login
  keychain (the macOS credential store) held no item with an OpenAI,
  Codex or ChatGPT identifier (on 2026-10-15 `security dump-keychain`
  listed 17 items, and none after any step of 2026-10-18), so where the
  key lives was not determined.
- macOS, ChatGPT 26.1013.40934: `Trust Tokens` and `Trust Tokens-journal`
  (36,864 B, in `Default` and both partitions) are collected unflagged.
  They are Chromium's Private State Tokens store: anti-fraud tokens that
  websites issue to the browser, which sign no one in to anything, so
  they are not a credential.
- `sentry/scope_v3.json` carries the account id and user id
  (`scope.user`) but no token; it is not collected
  ([Per-user storage](#per-user-storage)).
- Former native macOS app (write-up): the conversation encryption key
  is the Keychain item `com.openai.chat.conversations_v2_cache`
  (pvieito); absent on the 2026 install.
- No config file embeds an API key; the app uses account sign-in.
- Connecting a GitHub account (observed 2026-10-15) wrote no credential
  or configuration on the host: no `.git-credentials`, `.gitconfig`,
  `.ssh`, `gh` configuration or Windows Credential Manager entry
  (`cmdkey /list` held none). The connection
  lives on the ChatGPT account, so the GitHub access the app had is not
  visible in a collection.

### Exclusions

Windows, `OpenAI.Codex`: in `Default` and in every
`Default\Partitions\<partition>`, the Chromium caches `Cache` (13.7 MB in
`Default`), `Code Cache`, `GPUCache` (5.8 MB), `DawnGraphiteCache`,
`DawnWebGPUCache`, `blob_storage` (4.6 MB, one long ASCII line),
`Service Worker\CacheStorage` and `Service Worker\ScriptCache`, and
`Extensions` (a bundled browser extension, 90 files and 400 KB, copied
into each profile; its purpose was not determined). Each is a
`*/Roaming/Codex/web/Codex/Default/...` exclusion and a
`.../Default/Partitions/*/...` one.

Not catalogued, so not collected and not listed in the manifest:

- The other children of `web\Codex\`: the Chromium component
  directories, `component_crx_cache`, `DeferredBrowserMetrics`, the
  shader and GPU caches, `sentry` and `Crashpad` (crash and event
  reporting state, the precedent being the Claude Desktop exclusion
  `*/Claude*/sentry`) and `Last Browser`. The catalog names `Default`
  and four files beside it instead of the whole directory, so these need
  no exclusion each.
- `%LOCALAPPDATA%\OpenAI\Codex`: binaries and a Node.js runtime only.
- `~\.cache\codex-runtimes`: the 1.27 GB runtime (Git, Node.js, Python,
  PowerShell) and plugin manifests also present under `~\.codex`.
- `%LOCALAPPDATA%\Temp` scratch directories and files: no stable name to
  catalogue.

macOS, `com.openai.codex`: the same nine profile caches, in `Default`
and in every `Default/Partitions/<partition>`, as
`Library/Application Support/Codex/Default/...` exclusions;
`executor-plugins/*/server.mjs` (the bundled MCP server's 949 KB
script; its manifest and launcher are collected); and
`.codex/computer-use/*.app`, the 77 MB computer-use app bundle, whose
sibling `config.json` is collected. Not catalogued:

- `Library/Caches/Codex`, `Library/Caches/com.openai.codex` and
  `Library/Caches/com.openai.sky.CUAService`: HTTP and URL caches.
- `Library/Application Support/OpenAI`: empty on the install.
- The component directories, `component_crx_cache`, `sentry`,
  `Crashpad`, `BrowserMetrics-spare.pma` (a 4 MB metrics file) and the
  `Singleton*` symlinks under `Library/Application Support/Codex`: the
  catalog names only the profile and the files and directories beside
  it that the app writes.
- `~/.cache/codex-runtimes/codex-primary-runtime/` (32,625 entries on
  ChatGPT 26.1013.40934) and the sockets under `/private/tmp`.

The sizes of the excluded trees under `~/.codex` on ChatGPT 26.1013.40934
are in [Exclusions in `codex-cli.md`](codex-cli.md#exclusions); under
`Library/Application Support/Codex` the excluded profile and partition
caches present on that install were `blob_storage`, `DawnGraphiteCache`,
`DawnWebGPUCache`, `GPUCache`, `Service Worker/ScriptCache` and
`Extensions`, and nothing large sat outside them.

Previous Windows package: `*/Roaming/ChatGPT/Cache`, `Code Cache`,
`GPUCache`, `DawnGraphiteCache`, `DawnWebGPUCache`. Former native macOS
app: `Library/Caches/com.openai.chat`, which no catalog line names.

### Project-local files

The app writes none of its own into a project. The bundled Codex writes
what Codex writes (see [`codex-cli.md`](codex-cli.md)); the observed
project task wrote only the files it was asked for. Outside projects,
onboarding wrote `Desktop\Note from ChatGPT.txt` (78 B), by the
onboarding task the app's state names `desktop_note`, and a thread with
no project gets `~\Documents\Codex\<date>\<letter>\`.

On macOS (ChatGPT 26.1013.40934, 2026-10-18) the coding task in `~/Demo`
again wrote only the three files it was asked for: no `.codex`, `.git`
or `AGENTS.md`. The projectless output directories were
`~/Documents/Codex/2026-10-18/w/` (the plain chat) and
`~/Documents/Codex/2026-10-18/new-chat/` (the scheduled task's thread),
each with `work/` and `outputs/`; the first stayed, empty, after its
thread was deleted. An approved write outside the project went to
`~/Desktop`, and the write Approve for me allowed went to `~/Documents`
(`~/Documents/gpt-models-guide.md`, 5,042 B), not to the Desktop the
prompt named.

### Where the project path is recorded

Nothing under the package or `~/Library` records a project path. The bundled Codex records each
thread's working directory in its rollouts' `session_meta` `cwd`, which project discovery reads,
and the desktop app records projects in `~/.codex/.codex-global-state.json` (`local-projects`
`rootPaths`, `thread-workspace-root-hints`, `thread-projectless-output-directories`),
`~/.codex/state_5.sqlite` `project_roots` and, for a folder added as a project (macOS, ChatGPT
26.1013.40934, 2026-10-18), a `[projects."<path>"]` table in `~/.codex/config.toml`; see
[Where the project path is recorded in `codex-cli.md`](codex-cli.md#where-the-project-path-is-recorded), which also
says how discovery treats the projectless output directories. On the Windows install both
threads' directories appeared in the rollouts.

## Parsing

### Transcript stores

Relative to the home.

| Path | Content | Parse |
| --- | --- | --- |
| `.codex/sqlite/codex-dev.db` (+`-wal`, `-shm`) | the app's thread catalog and automation store; the `chatgpt` rows of `local_thread_catalog` are the account's conversations | yes, the `chatgpt` rows, by the `chatgpt-desktop` parser (the catalog attributes the file to `codex-cli`) |
| `.codex/sessions/**/rollout-*.jsonl` | the app's Codex-mode threads | yes, by the `codex-cli` parser |
| `.codex/.codex-global-state.json` | the app's UI and account state; on Windows it held a thread binding and a resume token for the ChatGPT chat (below); on macOS no resume-token key had appeared by the end of sign-in | by the `codex-cli` parser for its Codex keys; not by this parser |
| the Chromium profile (`Library/Application Support/Codex` on macOS, the `OpenAI.Codex_*` package on Windows) | the web view's own storage, cookies and logs | no: no conversation text was found there on either install |

There is no ChatGPT-mode transcript on disk on any install checked. On Windows (2026-10-15) a plain
chat and an image generation left, locally, only a thread binding and a resume token in
`~\.codex\.codex-global-state.json`, one title-only `chatgpt` row in `codex-dev.db`, and whatever
the profile's `app://` IndexedDB blob holds (not determined); the browser `History` databases of
the profile and both partitions had no URL rows, and no generated image was written under the
package or `~\.codex`. On macOS (2026-10-15) a plain chat and an image generation likewise left
only `chatgpt` catalog rows. On macOS (build 26.1013.40934, 2026-10-18) a reply in ChatGPT mode to
one of the listed conversations (about 22:02Z to 22:15Z) wrote nothing locally beyond a composer
draft in the global state and model-cache and Chromium housekeeping: no rollout, no new catalog
row, and the three `chatgpt` rows kept their 2026-10-15 `source_updated_at`. In that window
`logs_2.sqlite`, the app server's log, held only `account/read`, `getAuthStatus`, `config/read`
and `configRequirements/read` requests, so the conversation never passed through the local app
server (Codex's local service that owns the threads), and the profile's web storage had no
chatgpt.com origin. The conversation exists only on the server.

#### What survives a delete

The `chatgpt` row is a sign-in snapshot that outlived a reply to its
conversation from inside the app with its times unchanged. A delete of a
Codex-mode thread with the app's own command (macOS, 2026-10-19) removed
that thread's `vscode` row with its rollout; the `chatgpt` rows stayed.
What else the delete removed and left is in
[What survives a delete in `codex-cli.md`](codex-cli.md#what-survives-a-delete). What a
deletion of the conversation on chatgpt.com, or a sign-out, does to the
row was not exercised.

### Record schema

`local_thread_catalog`, column names from `pragma table_info` on the
macOS install (build 26.1013.40934, 2026-10-18), in table order:
`host_id`, `thread_id`, `display_title`, `source_created_at`,
`source_updated_at`, `cwd`, `source_kind`, `source_detail`,
`model_provider`, `git_branch`, `observation_sequence`,
`missing_candidate`, `thread_source`, `source_recency_at`,
`pending_observed_title`, `project_id`, `conversation_origin`,
`trial_conversation_type`, `chatgpt_async_status`. The declared column
types were not recorded.

Values on the `chatgpt` rows (macOS, 2026-10-18):

| Column | Value |
| --- | --- |
| `host_id` | `chatgpt:<workspace uuid>:<user id>` (74 characters): the ChatGPT workspace id and the 29-character ChatGPT user id, which begins `user-`; the `vscode` rows have `local` |
| `thread_id` | a UUID, the chatgpt.com conversation id |
| `display_title` | the conversation's title, set on every row seen (on Windows, 2026-10-15, the plain chat's row had only its title set) |
| `source_created_at`, `source_updated_at`, `source_recency_at` | REAL epoch seconds with a fraction (not milliseconds) |
| `source_kind` | `chatgpt` (`vscode` for a Codex thread) |
| `conversation_origin` | empty or `tpp`; NULL on the 2026-10-15 macOS rows. What `tpp` stands for is not determined; the same token is in the app's `.codex-tpp-local-executor-installation-id` file name on both platforms and in the macOS app log's `[tpp-local-executor]` component ([Per-user storage](#per-user-storage)) |
| `missing_candidate` | 0 |
| `thread_source`, `model_provider`, `cwd`, `git_branch`, `project_id`, `trial_conversation_type`, `chatgpt_async_status` | NULL |
| `source_detail`, `observation_sequence`, `pending_observed_title` | not recorded |

### Joins and timestamps

`thread_id` is the conversation's id on chatgpt.com and joins nothing
else on disk: no rollout, prompt history or log row carries it, because
the conversation never reaches the local app server
([Transcript stores](#transcript-stores)). It is the id to name in a request to OpenAI
for the conversation's content. `host_id` names the ChatGPT account the
list was taken from, as `local_thread_catalog_hosts` does.

The times are the server's times for the conversation as of the
snapshot: `source_created_at` when it was created and
`source_updated_at` when it last changed, both as of sign-in. A later
reply from inside the app did not move them ([Transcript stores](#transcript-stores)),
so `source_updated_at` is a lower bound on the conversation's last use,
not its last use. When the app wrote the row is the sign-in time, which
the row does not record. The three rows written at the 2026-10-18 macOS
sign-in were the account's conversations of the Windows round of
2026-10-15; those of the 2026-10-15 macOS round were not among them, and
why was not determined.

### Other stores

The catalog tables of `codex-dev.db` (macOS, build 26.1013.40934, 2026-10-18, unless a cell says
otherwise; table and column names only):

| Table | Columns | Rows seen |
| --- | --- | --- |
| `local_thread_catalog` | [Record schema](#record-schema) | 3 `chatgpt` at sign-in, plus one `vscode` per Codex thread; on macOS 2026-10-15, 3 `vscode` and 3 `chatgpt` (two with `conversation_origin` NULL, one `tpp`); on Windows 2026-10-15, 5 (3 `vscode`, 2 `chatgpt`, one with `conversation_origin` empty and one `tpp`) |
| `local_thread_catalog_hosts` | `host_id`, `host_kind` | 2: `local` / `local` and the account's `chatgpt:<workspace uuid>:<user id>` / `chatgpt`; the same two on Windows |
| `local_thread_catalog_sync_state` | `host_id`, `watermark_updated_at`, `initial_build_complete`, `observation_sequence`, `last_full_reconciled_at` | 2 (`initial_build_complete` 1; `observation_sequence` 5 and 1) |
| `local_thread_catalog_metadata` | `id`, `catalog_revision` | 1 |
| `local_thread_catalog_scan_checkpoints`, `local_thread_catalog_scan_entries` | not recorded | 0 |

The database's other tables (the scheduled-task store, `live_visualization_suggestions`,
`local_app_server_feature_enablement`, `thread_timeline_ledger` and `codex_schema_migrations`)
are Codex Desktop's and are in
[`sqlite/codex-dev.db` in `codex-cli.md`](codex-cli.md#sqlitecodex-devdb). Only
`local_thread_catalog` is read by the parser.

### Format versions

`codex_schema_migrations` (33 rows on the Windows 26.1007.9446.0 and the macOS 26.1013.40934
installs) carries the app version in `app_version`, with `build_flavor` `prod` on the Windows
install. The same table and the same `chatgpt` row shape were seen on Windows build 26.1007.9446.0
and macOS builds 26.1007.71326 and 26.1013.40934; the `local_thread_catalog_metadata`,
`_scan_checkpoints`, `_scan_entries` and `_sync_state` tables were recorded only on the
26.1013.40934 install, which does not show that the earlier builds lacked them. The parser reads
every column as possibly absent or NULL, so an older table without a column still parses.

### Secrets

`host_id` carries the ChatGPT workspace id and the user id, and
`display_title` the conversation's title, which is user content. No
token is stored in the table. The app's credentials are in
`~/.codex/auth.json`, flagged `secret: true` by the collector; see
[Credentials](#credentials).

### Parser plan

`wants()` by `artifact.rel == ".codex/sqlite/codex-dev.db"`; the
analyzer offers the file, which the catalog attributes to `codex-cli`,
to this parser through `reads_agents = ("codex-cli",)`, the routing
Cody and Twinny use for an editor's `state.vscdb`. The `codex-cli`
parser does not want the file. The database is opened through
`sqlite_util.py`, which reads the WAL sidecar from a scratch copy. Rows
come only from `local_thread_catalog` rows whose `source_kind` is
`chatgpt`, in rowid order; the `vscode` rows are the Codex threads the
rollouts already give.

Per `chatgpt` row, three `system` rows, each with `session_id` the
`thread_id`, `project_path`, `model` and `git_branch` empty,
`source_line` 0, `source_file` the database, and `timestamp_utc` the
row's `source_updated_at` with milliseconds (its `source_created_at`
when that is NULL, none when both are). `-` stands for a NULL or empty
value:

1. `account conversation: <display_title> origin=<conversation_origin>
   created=<source_created_at, UTC> updated=<source_updated_at, UTC>
   host=<host_id> (from .codex/sqlite/codex-dev.db local_thread_catalog)`
2. `session origin: remote chatgpt.com conversation <thread_id> (listed
   at sign-in; its transcript is server-side)`
3. `session synced: chatgpt.com holds conversation <thread_id>, no local
   transcript`

#### Permission rows

None recorded for a ChatGPT-mode conversation; the app's Codex-mode
approvals are in the rollouts and `logs_2.sqlite`, read by the
`codex-cli` parser.

#### Model rows

None recorded; `model_provider` is NULL on every `chatgpt` row.

#### Past-use rows and secondary sources

None read beyond the primary store. The table is the only local record
of these conversations; the Windows global state's resume token for the
ChatGPT chat ([Transcript stores](#transcript-stores)) is not read by this
parser. No secondary source.

#### Sync, relay and origin rows

The rows are themselves the sync marker, copied one way, account to app,
at sign-in; each gets the `session synced:` row. The app's Codex-mode
threads were not uploaded to the account on the macOS install
([Sync, relay and origin rows in `codex-cli.md`](codex-cli.md#sync-relay-and-origin-rows)),
and nothing in this table marks one as uploaded.

The parser sets `records_origin` and `records_sync`, so `sessions.jsonl`
reads `origin` `remote` and `synced` `yes` for these sessions, and
`relayed` `""`. Their `user_turns`, `assistant_turns` and `tool_calls`
are 0, since the text is not on disk. Relay: none.

#### Scratch directory rows

None. The app's projectless output directories are the bundled Codex's working directories, and
the `codex-cli` parser yields their rows
([Scratch directory rows in `codex-cli.md`](codex-cli.md#scratch-directory-rows)).

#### Deletion rows

None: the table records no deletion. What a delete leaves of the row is in
[What survives a delete](#what-survives-a-delete).

#### Bad-line rows

A file that is not SQLite is one `parser: not a SQLite database` row, a
SQLite error one `parser: SQLite error: <message>` row, and a database
without the table one `parser: no local_thread_catalog table` row.

#### Fixtures

The test fixture (`chatgpt_desktop_records()` in `analyzer/tests/fixtures/chatgpt_desktop.py`)
builds `.codex/sqlite/codex-dev.db` with the `local_thread_catalog` columns of
[Record schema](#record-schema) and a `local_thread_catalog_hosts` table, its rows only in the WAL
sidecar as on a running app: one `vscode` row (a Codex thread, which the parser skips), a `chatgpt`
row with a title, an empty `conversation_origin` and REAL times with a fraction, and a `chatgpt`
row with no title and `conversation_origin` `tpp`. Expected: three `system` rows for each
`chatgpt` row, in rowid order, and none for the `vscode` row.

## Confidence

- High, observed on Windows 11 (ChatGPT 26.1007.9446.0): the package name and layout, the profile
  and partition stores, caches and credential stores of [Per-user storage](#per-user-storage) to
  [Exclusions](#exclusions), that the chat surface keeps no local transcript, and that the running
  app holds the cookie stores, LevelDB `LOCK` files and settings hive locked.
- High, observed on macOS 26.6.2 (ChatGPT 26.1007.71326): the bundle id, the app's build and the
  `~/Library` layout, caches and credential stores.
- High, observed on macOS 26.6.2 (ChatGPT 26.1013.40934, one step per listing): what each step of
  [Source and evidence](#source-and-evidence) wrote of the app's own, and that a ChatGPT-mode reply leaves
  nothing local.
- High, write-ups: the former native app's `conversations-v2-*` encryption and the previous Windows
  package's layout.
- Medium: that the shell is a Chromium build hosting an Electron-style app, read from file names,
  versions and log components ([Source and evidence](#source-and-evidence)), not from the binaries'
  code.
- Not determined: where the macOS app keeps its Chromium cookie key; what the `app://` IndexedDB
  blob and `Local Storage` hold; the bundled extension's purpose; what "tpp" and "owl" stand for;
  the app's behaviour on x64 Windows (only ARM64 was observed); which of the user's actions wrote
  the first Codex thread on the 2026-10-15 installs; why the 2026-10-18 sign-in listed the Windows
  round's conversations and not the 2026-10-15 macOS round's; the declared column types of
  `local_thread_catalog`; and, since none was exercised, what a conversation deleted or renamed on
  chatgpt.com, a sign-out or a second sign-in does to a row, `conversation_origin` values other
  than empty, NULL and `tpp`, any `chatgpt_async_status` value, and a `chatgpt` row with `cwd`,
  `model_provider` or `project_id` set.
