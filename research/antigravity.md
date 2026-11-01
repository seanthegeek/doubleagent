# antigravity

Google Antigravity is three products under one catalog agent. The **CLI**, `agy`,
is a single Go binary. The **desktop app** (version 2.x, since 2.0) is an
Electron app, a desktop program built on Chromium and Node.js, whose bundle
calls the product "AGY Hub" (its internal name for the agent-first app, and
the `--subclient_type hub` argument it passes to its backend,
[Data directories under `~/.gemini`](#data-directories-under-gemini)); it
describes itself to the user as "redesigned to put agents first". The
**IDE**, the VS Code fork that carried the Antigravity name before 2.0, is now
a separate download named "Antigravity IDE". The CLI and the desktop app run
the same Go backend, which the app calls the language server: it ships it as
`language_server`, the name it gives the process that serves its agent
conversations (the program that owns the conversation stores and runs the
agent), and starts it in standalone mode. Both write the same
SQLite-of-protobuf conversation databases, each into its own data directory
under `~/.gemini/`. The app runs a scheduled task (an automation) as a
sidecar, a long-running helper process the language server supervises. The
CLI and desktop app transcripts are parsed by the `antigravity` parser
([Parser plan](#parser-plan)).

## Source and evidence

Closed source; no public source exists. Evidence levels, tagged per claim:

- **binary**: `~/.local/bin/agy` on the author's WSL (Windows Subsystem for
  Linux) workstation, the Google Antigravity CLI installed 2026-10-03, ELF
  x86-64 (the Linux executable format), 209,625,296 bytes (about 200 MB),
  sha256 `a759ce7c7a235d9b...`, `strings -n 8` saved to
  `scratchpad/agy_strings.txt` (552,251 lines). The embedded changelog's
  newest heading is `## 1.2.15` (`agy_strings.txt:310457`; oldest `## 1.0.0`
  at 311213). Cited as `str:<line>` or by quoted literal. It embeds 1300
  `.proto` descriptors; the relevant packages are `gemini_coder`
  (`third_party/gemini_coder/proto/trajectory.proto`), `exa.cortex_pb`
  (`third_party/jetski/cortex_pb/cortex.proto`), `exa.codeium_common_pb` and
  `exa.jetski_cortex_pb`. The `jetski`, `cascade` and `codeium` names show the
  lineage from Windsurf's Cascade agent. The parser's tables were built from
  these descriptors.
- **descriptors**: the protobuf descriptors embedded in `agy` 1.2.16
  macOS arm64 (271 descriptor files) and in the desktop app's
  `language_server` 2.23.0 (484), extracted with
  `analyzer/tools/proto_descriptors.py extract` and cited by message or
  enum name.
- **bundle**: the desktop app's `Contents/Resources/app.asar` (asar is
  Electron's archive format for an app's code), version 2.23.0, 4,486,221
  bytes, sha256
  `4e99093662d797ecba823e92823d352413e3bf2ca721257df4a255c045df1598`, 738
  packed files with the main-process code under `dist/`. Cited as
  `app.asar:<path>@<byte offset into the asar file>` followed by a literal
  snippet; a reader of another version should grep for the snippet. Nothing
  from the bundle was executed.
- **live**: names and sizes only under `~/.gemini/antigravity-cli`,
  `~/.gemini/config` and `~/.cache/antigravity` on the Linux workstation
  where the ELF binary above was read.
- **observed**: the macOS, Ubuntu and Windows real installs below, read
  for file names, sizes, key names, value types, counts and log-line
  templates with ids redacted only.
- **docs**: [Antigravity CLI install](https://antigravity.google/docs/cli/install/),
  [Antigravity settings](https://antigravity.google/docs/settings/),
  [Antigravity CLI hands-on codelab](https://codelabs.developers.google.com/antigravity-cli-hands-on); **community**:
  jazzyalex.github.io agent-sessions guide, dev.to ndondadaniel2020 restore
  write-up, [Devilaiger/antigravity-chat-history-fix](https://github.com/Devilaiger/antigravity-chat-history-fix),
  [marioalbu08/antigravity-sync](https://github.com/marioalbu08/antigravity-sync),
  [SavFil/antigravity-convo-manager](https://github.com/SavFil/antigravity-convo-manager),
  [damadorPL/agent-memories](https://github.com/damadorPL/agent-memories) (google/antigravity.md),
  discuss.ai.google.dev thread 167445.

The transcript schema rests on the shipped binaries (protobuf descriptors
extracted from `agy` and from the app's `language_server`), confirmed against
a real install on the author's WSL workstation (CLI), on the author's Mac
(CLI and desktop app), in the author's Ubuntu lab VM (CLI and desktop app)
and in the author's Windows 11 lab VM (CLI and desktop app), October 2026.
Extraction:
`tools/proto_descriptors.py extract ~/.local/bin/agy descriptors.json`, then
`show` on the message names of [Record schema](#record-schema).

### Observed

- **Windows, CLI** ([issue 83](https://github.com/seanthegeek/doubleagent/issues/83),
  [issue 89](https://github.com/seanthegeek/doubleagent/issues/89) and
  [issue 90](https://github.com/seanthegeek/doubleagent/issues/90)): `agy` 1.3.5 (`agy --version`)
  at `%LOCALAPPDATA%\agy\bin\agy.exe`, on `PATH`, 184,062,616 B, sha256
  `223953a7d64ad199bbeaae3cf84d634b0ec3917979c7a60feac6ade05d50888b`, installed 2026-10-14T23:56:21Z
  in the author's Parallels lab VM, Windows 11 ARM64 build 26300 (`ver` reports `10.0.26300.9550`;
  ARM64 is the 64-bit Arm architecture) with no WSL distribution installed, and observed the same
  evening (US Eastern daylight time; the UTC times below cross into 2026-10-15), read for file
  names, sizes, key names, value types and log lines with ids redacted only. `%LOCALAPPDATA%` is the
  per-user `AppData\Local` folder, `%APPDATA%` the per-user `AppData\Roaming` folder,
  and `~` is `%USERPROFILE%`, `C:\Users\<user>`. The executable's version
  resource (`VersionInfo.ProductVersion`, the version Windows shows in the
  file's properties) is empty; the embedded changelog's newest heading is
  `## 1.3.4`, with `## 1.3.0` to `## 1.3.4` present. The CLI created
  `~/.gemini/antigravity-cli/` at its first run, 00:22:29Z. Started from
  `<home>\demo2`, typed with a lower-case `users`
  (`C:\users\<user>\demo2`), it ran a coding task that wrote one source file
  per language into ten subdirectories and a `README.md` (00:25:07Z to
  00:27:16Z), which left one conversation, `conversations/<uuid>.db`
  (983,040 B, 00:28:12Z, 38 steps, `trajectory_meta.source` 17), and one
  `history.jsonl` line. The listings were taken after the tasks of both
  surfaces, so the mapping from task to file is by modification time only.
  What 1.3.5 wrote beyond 1.3.4, among it a copy of the app's conversation
  index, is in [CLI data directory](#cli-data-directory), its keyring use in
  [Credentials](#credentials) and its project-path spellings in
  [Where the project path is recorded](#where-the-project-path-is-recorded).

- **Windows, desktop app** ([issue 83](https://github.com/seanthegeek/doubleagent/issues/83),
  [issue 84](https://github.com/seanthegeek/doubleagent/issues/84) and
  [issue 91](https://github.com/seanthegeek/doubleagent/issues/91)): version 2.23.1 (`main.log`:
  `Starting app (v2.23.1) with dynamic port…`; the version resource of `Antigravity.exe` has
  `ProductVersion` `2.23.1.0` and `FileVersion` `2.23.1`), in the same VM, installed per user and
  first launched 2026-10-14T23:56:11Z (program files in
  [Program files and install metadata](#program-files-and-install-metadata)). The user ran, in the
  app: a question with no file writes; a coding task in a project folder `<home>\demo`
  (`C:\Users\sean\demo`), which wrote six files there (a hello world in C, Go, JavaScript, Python
  and Rust, and a `README.md`, 00:01:44Z to 00:03:20Z); a research task that wrote a
  Markdown document to the app's `scratch/` directory (00:10:29Z) and
  started a background task; and a task that set up a scheduled automation
  named `weekly-microsoft-security-bulletins` (`sidecar.json` 00:17:41Z to
  00:20:16Z; [Sidecar logs](#sidecar-logs)). The Google Docs plugin `gdocs`
  0.0.6 was installed from the marketplace between 00:05:45Z and 00:08:51Z,
  when its `token.json` was written. The four conversation databases under
  `~/.gemini/antigravity/conversations/` are 00:03:54Z (622,592 B),
  00:06:55Z (589,824 B), 00:13:30Z (1,441,792 B) and 00:20:53Z
  (6,258,688 B), of 20, 10, 46 and 46 steps (the step counts are in the
  analyzer's session order, which was not matched to the file sizes), all
  `trajectory_meta.source` 1, each with a `brain/<uuid>/` directory, an
  `annotations/<uuid>.pbtxt` and a summaries row; the project file
  `~/.gemini/config/projects/<uuid>.json` is 23:59:53Z. The listings were
  taken after everything (00:29Z) and again after the collection (00:45Z),
  which showed no new file. The app wrote `~/.gemini/antigravity/`, files in
  `~/.gemini/config/`, the Electron user-data directory
  `%APPDATA%\Antigravity\` with its `logs\`, and an empty
  `%LOCALAPPDATA%\antigravity\staging\` (23:56:15Z, before the CLI's first
  run), described in [Per-user storage](#per-user-storage). It created no
  `~/.gemini/antigravity-ide`, `~/.gemini/antigravity-backup`,
  `~/.antigravity`, `~/.config`, `~/.cache` or `~/.antigravity-server` under
  the profile, and no `%APPDATA%\Antigravity IDE`.

- **Linux, CLI** ([issue 83](https://github.com/seanthegeek/doubleagent/issues/83),
  [issue 89](https://github.com/seanthegeek/doubleagent/issues/89) and
  [issue 90](https://github.com/seanthegeek/doubleagent/issues/90)): `agy` 1.3.4 (`agy --version`)
  at `~/.local/bin/agy`, which is not on the non-interactive `PATH`, ELF aarch64, 204,492,984 bytes,
  sha256 `08d619ae3969c33b9cba5eac2a54f4566cbca1b5de68968167cb908ced5b4824`, in the author's Ubuntu
  lab VM (Ubuntu 26.04.1 LTS, aarch64, Linux 7.0.0-38-generic), installed 2026-10-13 21:37 local
  time (EDT, US Eastern daylight time) and observed the same day, read for file names, sizes, key
  names, value types, SQLite schemas and enum values only, with `find`,
  `ls`, `analyzer/tools/shape.py` and a read-only schema script, all Python
  run with `python3 -I`. The embedded changelog's newest heading is
  `## 1.3.3`. A few turns left one conversation
  (`~/.gemini/antigravity-cli/conversations/<uuid>.db`, 622,592 B, modified
  21:43, 26 steps, `trajectory_meta.source` 17 (`cli`) and
  `trajectory_type` 4) and a project folder the agent wrote files into,
  `~/Antigravity/demo/`, inside the app's install directory. The listings
  were taken after the tasks of both surfaces, so the mapping from task to
  file is by modification time only. What 1.3.4 writes beyond 1.2.x is in
  [CLI data directory](#cli-data-directory), and the file secret store its
  log names in [Credentials](#credentials).

- **Linux, desktop app** ([issue 83](https://github.com/seanthegeek/doubleagent/issues/83) and
  [issue 91](https://github.com/seanthegeek/doubleagent/issues/91)): version 2.23.1 (`main.log`:
  `Starting app (v2.23.1) with dynamic port…`), in the same VM, first run 2026-10-13 21:29 local
  time, unpacked by hand from a tarball
  ([Program files and install metadata](#program-files-and-install-metadata)). The user ran a few
  tasks, among them a coding task and a scheduled automation, which left four conversations under
  `~/.gemini/antigravity/conversations/` (655,360 to 1,114,112 B, modified
  21:45, 21:51, 21:55 and 22:00, all `trajectory_meta.source` 1
  (`cascade_client`) and `trajectory_type` 4, of 14, 43, 30 and 38 steps,
  [Conversation database](#conversation-database)), each with a
  `brain/<uuid>/` directory, an `annotations/<uuid>.pbtxt` and a summaries
  row; a project folder `~/demo2/` (21:53 to 21:54) whose project file is
  21:47; the automation's `sidecar.json` (21:58 to 21:59) and its
  `sidecar_data/` (22:00, [Sidecar logs](#sidecar-logs)); the Google Docs
  plugin's token (21:49); and a Markdown report the agent wrote at the home
  root (21:59). No scheduled run fired. The app wrote
  `~/.gemini/antigravity/` (56 MB), files in `~/.gemini/config/` (320 KB),
  and the Electron user-data directory `~/.config/Antigravity/` (8.9 MB)
  with its `logs/` inside it, as the bundle says
  ([Per-user storage](#per-user-storage)). It created no `~/.config/antigravity`
  (lower case), `~/.antigravity`, `~/.antigravity-server`,
  `~/.gemini/antigravity-ide` or `~/.gemini/antigravity-backup`.

- **macOS, CLI** ([issue 83](https://github.com/seanthegeek/doubleagent/issues/83)): two builds on
  the author's Mac (macOS 26.4, Darwin 25.4.0, Apple silicon, arm64). `agy` 1.2.16 (`agy --version`;
  the embedded changelog's newest heading is still `## 1.2.15`) at
  `~/.local/bin/agy`, Mach-O 64-bit arm64 (the macOS executable format),
  175,966,448 bytes, sha256
  `7dca095cfc1df2c057a385ed88a76c7ba98dc103258a80be87a8f42e484cb3aa`,
  installed 2026-10-04, observed 2026-10-13 and read for file names, sizes,
  key names, value types and SQLite schemas only, with
  `analyzer/tools/shape.py`: two conversations were on disk in
  `~/.gemini/antigravity-cli/conversations/`, each a
  `conversations/<uuid>.db` with `trajectory_meta.source` 17 (`cli`,
  `CORTEX_TRAJECTORY_SOURCE_CLI` in the descriptor
  `exa.cortex_pb.CortexTrajectorySource`) and `trajectory_type` 4, of 12 and
  8 steps, each with its `brain/<uuid>/` logs
  ([Transcript stores](#transcript-stores)). `agy` 1.3.6
  (`/Users/sean/.local/bin/agy`, 183,407,440 bytes, sha256
  `827ba22b4aa33557614a57aac2cc13f386e17db6b802febd3932226906ac05ab`),
  observed 2026-10-15: one conversation of 17 steps in a project directory
  the agent was then told to delete. What the delete left is in
  [What survives a delete](#what-survives-a-delete), the mirror's line count
  in [Brain mirror lines](#brain-mirror-lines), and which of the four
  `history.jsonl` lines carry `conversationId` in
  [Joins and timestamps](#joins-and-timestamps).

- **macOS, desktop app** ([issue 83](https://github.com/seanthegeek/doubleagent/issues/83)):
  `/Applications/Antigravity.app`, `CFBundleShortVersionString` and `CFBundleVersion` both `2.23.0`,
  on the same Mac, first launched 2026-10-13 18:28 local time and observed the same
  day (its `app.asar` is the **bundle** source above, its other program
  files in
  [Program files and install metadata](#program-files-and-install-metadata)).
  The user ran, in this order, a plain question-and-answer chat with no
  tool use, a coding task, an automation (a scheduled task the app runs as
  a sidecar, [Shared configuration directory](#shared-configuration-directory))
  and the Google Docs plugin ("gdocs", installed from the official plugin
  marketplace). The directory listings were taken after all four, so which
  conversation belongs to which task is not known, and the order below is
  by modification time only: the four conversation databases under
  `~/.gemini/antigravity/conversations/`, all `trajectory_meta.source` 1
  (`CORTEX_TRAJECTORY_SOURCE_CASCADE_CLIENT`, `cascade_client`) and
  `trajectory_type` 4, are 12 steps (modified 18:37), 24 steps (18:43), 43
  steps (18:50) and 31 steps (19:14), each with a `brain/<uuid>/` directory,
  an `annotations/<uuid>.pbtxt` and a row in `conversation_summaries.db`
  ([Transcript stores](#transcript-stores)); the automation's `sidecar.json` is
  18:41 to 18:42, the plugin's `token.json` 18:46 and the agent's scratch
  document 18:47. No scheduled run fired. The app wrote
  `~/.gemini/antigravity/` (49 MB), files in the shared `~/.gemini/config/`,
  the Electron user-data directory
  `~/Library/Application Support/Antigravity/` (5.3 MB) and
  `~/Library/Logs/Antigravity/` ([Per-user storage](#per-user-storage)); it
  created no `~/.gemini/antigravity-ide`, `~/.gemini/antigravity-backup` or
  `~/.antigravity`, nothing under `~/Library/Caches`, and no LaunchAgent,
  launchd job or cron entry (`~/Library/LaunchAgents` has none).

### Source reviews

None.

## Collection

### Per-user storage

Everything the CLI and the app keep per user sits under `~/.gemini/`; the
Go binary resolves it via `os.UserHomeDir` (binary) so Windows is
`%USERPROFILE%\.gemini` (community: Devilaiger, discuss 167445 quote
`%USERPROFILE%\.gemini\antigravity-cli\brain`). The app's main process
builds the same path from Node's `os.homedir()` (bundle,
[Data directories under `~/.gemini`](#data-directories-under-gemini)).
`os.UserConfigDir`/`os.UserCacheDir`, `XDG_CONFIG_HOME`, `XDG_CACHE_HOME`,
`APPDATA`, `LOCALAPPDATA` also appear (binary), consistent with the
`~/.cache/antigravity/staging` dir (live, observed; empty) used as the
plugin "marketplace cache ... staging directories" (str changelog). On
Windows the same directory is `%LOCALAPPDATA%\antigravity\staging`
(observed, Windows 11 ARM64 build 26300, 2026-10-14; empty), since Go's
`os.UserCacheDir()` is `%LOCALAPPDATA%` there; the desktop app 2.23.1
created it during its first launch, before the CLI's first run. The
catalog line `antigravity|AppData/Local/antigravity` collects it.

#### Scratch directories

The desktop app's `~/.gemini/antigravity/scratch/<name>.md` holds a Markdown
document the agent wrote during a conversation, user-level, not per
conversation ([desktop app data directory](#desktop-app-data-directory));
each `brain/<uuid>/scratch/` was an empty directory on every install
([brain directories](#brain-directories)). Both are collected
([Exclusions](#exclusions)).

#### Program files and install metadata

The program files and install metadata of each install are binaries, not
state, and have no catalog line (observed):

- The CLI is one executable: `~/.local/bin/agy` on macOS and Linux, and
  `%LOCALAPPDATA%\agy\bin\agy.exe` on Windows (docs/cli/install; observed
  with CLI 1.3.5). Builds and hashes are in [Observed](#observed).
- macOS, app 2.23.0: `/Applications/Antigravity.app`, an Electron app
  (`Contents/Frameworks/Electron`, 200,443,040 bytes) with
  `Contents/Resources/app.asar` (the **bundle** of
  [Source and evidence](#source-and-evidence)), its backend
  `Contents/Resources/bin/language_server` (Mach-O arm64, 155,290,912 bytes,
  sha256 `c1dd0c9940f712adf2185e756470752d5f5afee15441e32879e4a0f127e18273`)
  and `Contents/Resources/bin/webm_encoder`.
- Linux, app 2.23.1: downloaded as `~/Downloads/Antigravity.tar.gz` and
  unpacked by hand to `~/Antigravity/` (285 MB), a binary install: the
  Electron binary `antigravity` (220,466,568 B, sha256
  `c543041069aa2e478235447cb636f9f09faab15003862029d226bb14b8dc655b`), a
  setuid-root `chrome-sandbox`, the Chromium libraries and resource packs,
  `resources/app.asar` (4,486,221 B, the size of the macOS 2.23.0 asar,
  with a different hash, sha256
  `b8ec3628df2a12ee9deac5dc06367f238065af146bfe6298fdad6db5f9790d9c`),
  `resources/app-update.yml` (keys `provider`, `url`,
  `updaterCacheDirName`), `resources/bin/language_server` (181,358,800 B,
  sha256 `310ee8e0cba4c4ee05504ac8021cbd3544a79c06cc01c790536361625c5fffb3`)
  and `resources/bin/webm_encoder`. It is not an AppImage (a self-contained
  Linux application package): `main.log` warns
  `APPIMAGE env is not defined, current application is not an AppImage`.
- Windows, app 2.23.1 (Windows 11 ARM64 build 26300, 2026-10-14): the
  per-user installer was left at
  `%LOCALAPPDATA%\antigravity-updater\installer.exe` (155,215,064 B, sha256
  `428ad29354a12654db8be066cbce1fb307dca538b51176a584c1009cb59c3fbc`,
  23:55:21Z) and installed the app to `%LOCALAPPDATA%\Programs\antigravity\`
  (424 files, 574,823,898 B, 23:56:09Z): `Antigravity.exe` (226,382,336 B,
  sha256 `16febcd47a2a6823fe9ec4847b60f2fc766a46c8c355643185326a2879668243`),
  `Uninstall Antigravity.exe`, `resources\app.asar` (4,486,221 B, sha256
  `b8ec3628df2a12ee9deac5dc06367f238065af146bfe6298fdad6db5f9790d9c`,
  byte-identical to the Linux 2.23.1 asar), `resources\app.asar.unpacked\`
  (Node modules, among them `chrome-devtools-mcp`),
  `resources\app-update.yml` (144 B), `resources\elevate.exe` (107,520 B),
  `resources\bin\language_server.exe` (160,643,584 B, sha256
  `32d65353f033600274203137913207c02f636c214b8233c529a2afa0ee55ecf5`),
  `resources\bin\webm_encoder.exe` (12,472,320 B, sha256
  `e78f6cd6292baeb2d322cba9a5e66f2e42aed5589cba175fe4aa5dd6daea90fd`) and
  the usual Electron and Chromium files (`locales\`, `*.pak`, `icudtl.dat`,
  `ffmpeg.dll`, `vulkan-1.dll`, `d3dcompiler_47.dll`,
  `LICENSES.chromium.html`). It also made a Start Menu shortcut,
  `%APPDATA%\Microsoft\Windows\Start Menu\Programs\Antigravity.lnk`, and
  an uninstall entry under
  `HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall` (HKCU is
  `HKEY_CURRENT_USER`, the per-user registry hive) with `DisplayName`
  `Antigravity`, `DisplayVersion` `2.23.1` and no `InstallLocation`. While
  the app ran, six `Antigravity.exe` processes and one
  `language_server.exe` were running: the same backend as on the other
  platforms. The catalog line `AppData/Local/antigravity` names a whole
  path component, so it matches neither `antigravity-updater` nor
  `Programs\antigravity`.

#### Data directories under `~/.gemini`

- `~/.gemini/antigravity/` is the desktop app's data directory on every
  OS. The name is the app name lower-cased without spaces
  (`app.asar:dist/paths.js@4312548`
  `return electron_1.app.getName().toLowerCase().replace(/\s+/g, '');`,
  inside `function getAppDataDirName() {` at @4312439; `antigravity-dev`
  when the app runs unpackaged), joined to the home
  (`app.asar:dist/paths.js@4312621` `function getAppDataDir() {` ...
  `path_1.default.join(os_1.default.homedir(), '.gemini', getAppDataDirName())`).
  The app passes the name to its backend
  (`app.asar:dist/languageServer.js@4242293` `'--app_data_dir',`
  `(0, paths_1.getAppDataDirName)(),`), which owns the directory, and
  starts the backend as subclient "hub"
  (`app.asar:dist/languageServer.js@4241987` `'--subclient_type',` `'hub',`,
  beside `'--override_ide_name', 'antigravity'` and `'--standalone'`).
  Before 2.0 the IDE used the same directory, `app.asar:dist/paths.js@4313608`,
  commented "User data dir for the old IDE (source for copy)":

  ```text
  exports.IDE_OLD_DATA_DIR = path_1.default.join(os_1.default.homedir(), '.gemini', 'antigravity');
  ```

- `~/.gemini/antigravity-cli/` is the CLI's (binary, live, observed on
  macOS, Linux and Windows).
- `~/.gemini/antigravity-ide/` is the separately installed IDE's,
  `app.asar:dist/paths.js@4313784` ("User data dir for the separately
  installed IDE (destination for copy)"):

  ```text
  exports.IDE_NEW_DATA_DIR = path_1.default.join(os_1.default.homedir(), '.gemini', 'antigravity-ide');
  ```

  The app's IDE install wizard copies pre-2.0 IDE data there when
  `antigravity-ide` does not exist and `antigravity` does
  (`app.asar:dist/ideInstall/constants.js@4160197`, `exports.WIZARD_SHOWN_KEY`
  and the comment "2. `~/.gemini/antigravity-ide` does NOT exist 3.
  `~/.gemini/antigravity` DOES exist"); on the observed install `main.log`
  records `[IDE Wizard] <path> not found — user was not migrated, skipping.`
  Bundle-derived, not observed on any real install.
- `~/.gemini/antigravity-backup/` is the backup taken during that copy
  (bundle-derived, not observed on any real install),
  `app.asar:dist/paths.js@4313949`:

  ```text
  exports.IDE_BACKUP_DATA_DIR = path_1.default.join(os_1.default.homedir(), '.gemini', 'antigravity-backup');
  ```

The macOS, Linux (Ubuntu, CLI 1.3.4 and app 2.23.1) and Windows (CLI 1.3.5
and app 2.23.1) installs had `.gemini/antigravity/`,
`.gemini/antigravity-cli/` and `.gemini/config/`, with the same file names
on each, and neither `.gemini/antigravity-ide/` nor
`.gemini/antigravity-backup/`; on Windows under `%USERPROFILE%\.gemini\`
(the user's profile folder, the home on Windows).

#### Desktop app data directory

`~/.gemini/antigravity/` (observed, macOS, app 2.23.0, 49 MB; the
Linux install of 2.23.1 had the same layout at 56 MB, with the additions
listed after the table):

| Path | Observed |
| --- | --- |
| `conversations/<uuid>.db` x4 (688,128 to 1,409,024 B, no sidecar files) | one SQLite database per conversation, the same schema as the CLI's ([Record schema](#record-schema)) |
| `conversation_summaries.db` | the conversation index, the CLI's table plus a full-text search table over titles, workspaces and message bodies ([Summaries database](#summaries-database)) |
| `brain/<uuid>/` x4 | per-conversation logs and tool output ([Brain directories](#brain-directories)) |
| `annotations/<uuid>.pbtxt` x4 (94 to 99 B) | pbtxt is the protobuf text format, a human-readable serialisation of a protobuf message; each holds one `title:` field, the conversation title |
| `scratch/<name>.md` (24,280 B) | a Markdown document the agent wrote during a conversation; user-level, not per conversation |
| `plugin_data/gdocs/token.json` (1,119 B, mode 0600) | the Google Docs plugin's OAuth credential ([Credentials](#credentials)) |
| `plugin_data/gdocs/bin/gdocs-<digits>` (31,938,752 B) | the plugin's binary |
| `bin/webm_encoder` (12,699,266 B), `bin/agentapi` (100 B), `bin/gdocs` | binaries; `bin/gdocs` is a symlink to `~/.gemini/config/plugins/gdocs/skills/gdocs/scripts/gdocs` |
| `cache/marketplaces/antigravity-plugins-official/` (207,829 B) | the plugin marketplace cache: `marketplace.json` (17 plugins, each with a name, version and archive URL and sha256), `meta.json` and `plugins/gdocs/0.0.6/`, a copy of the plugin the user installed |
| `builtin/skills/<name>/SKILL.md` x8, `builtin/.checksum`, `builtin/keep.txt` | bundled skills, the same set as `antigravity-cli/builtin/` |
| `agyhub_summaries_proto.pb` (1,436 B) | the app's counterpart of the CLI's `jetbox_summaries_proto.pb` |
| `antigravity_state.pbtxt` (850 B) | install and onboarding state (keys `installation_uuid`, `post_onboarding`, `agent_onboarding_completed`, `seen_nuxs` (by its name, the new-user-experience tips already shown), `migrate_convos_into_projects`, `migrate_retroactive_projects`, `migrations`); the CLI's counterpart is `jetski_state.pbtxt` |
| `installation_id` (36 B), `presence/<uuid>.lock` x4 (0 B), `crashes/crash_<pid>_<uuid>.log` (0 B) | as in the CLI directory |

Jetski and Jetbox are internal names that appear in the CLI's file names
and keyring service; the binary does not explain them.

Added on Linux (observed, app 2.23.1):

- `sidecar_data/<name>/` (22:00): `logs/sidecar.log` (388 B), the
  scheduler log of the automation named `<name>`
  ([Sidecar logs](#sidecar-logs)), and an empty `data/`.
- `lib/node_modules/sidecar_sdk/` (5 files, 17,508 B: `package.json`,
  `index.mjs`, `preload.js`, `esm_resolver.js`, `register_loader.js`):
  the sidecar SDK, the JavaScript library the app installs for sidecar
  code; the same five files are in `~/.gemini/config/node_modules/sidecar_sdk/`.
  Code, not state, and excluded ([Exclusions](#exclusions)).
- `brain/<uuid>/.system_generated/steps/<idx>/content.md` in one
  conversation ([Transcript stores](#transcript-stores)).
- `conversation_summaries.db` (167,936 B) with the macOS schema
  ([Summaries database](#summaries-database)).
- `scratch/<name>.md` (675 B), `plugin_data/gdocs/token.json` (1,126 B,
  mode 0600) and `plugin_data/gdocs/bin/gdocs-<digits>` (36,320,890 B),
  `bin/webm_encoder` (15,860,003 B), `bin/agentapi` (84 B), the `bin/gdocs`
  symlink and `cache/marketplaces/antigravity-plugins-official/`
  (282,624 B) as on macOS.

On Windows (observed, app 2.23.1, Windows 11 ARM64 build 26300,
2026-10-14) the layout is the same, with these differences:

- `bin/` holds `agentapi.bat` (107 B), `webm_encoder.exe` (12,472,320 B)
  and `gdocs.cmd` (128 B), a `.cmd` launcher where macOS and Linux have
  the `bin/gdocs` symlink.
- No `sidecar_data/` ([Sidecar logs](#sidecar-logs)) and no `lib/`.
- `conversations/<uuid>.db` x4 (589,824 to 6,258,688 B) and
  `conversation_summaries.db` (159,744 B), none with a `-wal` or `-shm`
  sidecar when listed.
- In the brain directories, `messages/undelivered/`, a chunked
  `transcript_full.jsonl` and a 2,682,163 B `content.md`
  ([Transcript stores](#transcript-stores)).
- Other sizes: `agyhub_summaries_proto.pb` 1,417 B,
  `annotations/<uuid>.pbtxt` x4 93 to 96 B, `antigravity_state.pbtxt`
  812 B, `builtin/.checksum` 72 B, `installation_id` 36 B,
  `scratch/<name>.md` 23,305 B, `plugin_data/gdocs/token.json` 1,122 B,
  `plugin_data/gdocs/bin/` 31,200,920 B,
  `cache/marketplaces/antigravity-plugins-official/` 207,829 B
  (`marketplace.json` 17,664 B, `meta.json` 48 B and
  `plugins/gdocs/0.0.6/`), and `crashes/crash_<pid>_<uuid>.log` and
  `presence/<uuid>.lock` x4 at 0 B. The running app holds the open
  conversation's `presence/<uuid>.lock` locked against other readers.

#### Brain directories

`brain/<uuid>/` (observed, app and CLI), one per conversation, holds the
step mirror (`.system_generated/logs/transcript.jsonl`,
`transcript_full.jsonl` and their chunks), the output of each tool step,
and in the app messages addressed to the conversation and background-task
logs. Its layout and sizes per platform are in
[Transcript stores](#transcript-stores) and its line shapes in
[Record schema](#record-schema).

#### CLI data directory

`~/.gemini/antigravity-cli/` (live, observed on macOS, with binary
literal where noted): `antigravity-oauth-token` (1,670 B, see
[Credentials](#credentials)); `bin/webm_encoder` (17 MB on Linux);
`brain/<uuid>/` ([Brain directories](#brain-directories); binary:
`.system_generated/logs/transcript.jsonl`, `transcript_full.jsonl`);
`builtin/skills/<name>/`, `builtin/.checksum`, `keep.txt` (bundled skills; binary also names
`~/.gemini/<app-name>/builtin/sidecars/<id>/` and
`builtin/plugins/<plugin>/sidecars/`); `cache/` with
`default_project_id.txt` (19 B, base64 text), `last_conversations.json`
(an object mapping an absolute workspace path to a conversation UUID) and
`onboarding.json` (three onboarding booleans) (observed), plus
`cache/projects.json` (binary: "stores workspace-to-project mappings in a
centralized `~/.gemini/antigravity-cli/cache/projects.json`"; not present
on the macOS install).

The rest of the directory: `cli.log` symlink to
`log/cli-<YYYYMMDD_HHMMSS>.log` (binary `cli.log`);
`conversation_summaries.db` (+`-wal`, `-shm`; binary
`conversation_summaries.db`, `ALTER TABLE conversation_summaries ...`),
which has only the `conversation_summaries` table; `conversations/<uuid>.db`
(+wal/shm); `crashes/` (empty); `history.jsonl` (binary);
`implicit/<uuid>.pb`; `installation_id` (binary);
`jetbox_summaries_proto.pb` (binary `jetbox_summaries_pb.SummariesState`);
`jetski_state.pbtxt` (binary; keys `installation_uuid`, `post_onboarding`,
`migrations`, observed); `last_check.timestamp`; `presence/<uuid>.lock`;
`settings.json` (docs: `~/.gemini/antigravity-cli/settings.json`; binary
`trustedWorkspaces` tag); `keybindings.json` (docs/settings);
`updater/{update.lock, update_status.json}`; legacy `hooks.json` (binary:
"wrote configurations to `~/.gemini/antigravity-cli/hooks.json` instead of
the shared `~/.gemini/config/hooks.json`").

New in CLI 1.3.x (observed, Linux, 1.3.4): `annotations/<uuid>.pbtxt`
(37 B, one `title:` field, as in the app's directory);
`cache/marketplaces/antigravity-plugins-official/` (`marketplace.json`
and `meta.json`, 32,768 B, excluded); a `conversationId` key on some
`history.jsonl` lines ([Joins and timestamps](#joins-and-timestamps)); and
`settings.json` (156 B) with `permissions` {`allow`, an array of strings}
beside `trustedWorkspaces` (an array of 1 absolute path)
([Permission configuration files](#permission-configuration-files)). The
log directory holds `log/cli-<YYYYMMDD_HHMMSS>.log` files (22,498 and
45,442 B) in glog format (glog is Google's logging library) with
`cli.log` linking to the newest; besides the secret-store line in [Credentials](#credentials),
they name `~/.gemini/config/config.json`, `~/.gemini/extensions` (Gemini
CLI's extension directory, absent on this install) and the workspace
paths.

Observed on Windows (CLI 1.3.5, Windows 11 ARM64 build 26300,
2026-10-14), against 1.3.4 on Linux:

- `cache/conversation_metadata.json` (2,671 B), new: an entry for each of
  the desktop app's four conversations ([Transcript stores](#transcript-stores)).
- `conversation_summaries.db` (40,960 B, no sidecars) holds a row for each
  of the app's four conversations beside its own: CLI 1.3.5 copies the
  app's conversation index into its own
  ([Summaries database](#summaries-database)).
- `cli.log` (41,122 B) is a hard link to `log/cli-20261014_202229.log` on
  NTFS (the Windows file system), not a symlink: `fsutil hardlink list`
  (`fsutil` is the Windows file-system utility) names both paths, so the
  two names hold the same bytes.
- `updater/update.lock` (0 B) and `updater/update_status.json` (59 B;
  `message` string, `success` bool).
- `plugin_data/gdocs/`, an empty directory; the plugin's `token.json` is
  in the app's directory.
- `history.jsonl` (230 B) has one line, without `conversationId`
  ([Joins and timestamps](#joins-and-timestamps)).
- `settings.json` (172 B): `permissions` {`allow`} and `trustedWorkspaces`
  (one absolute Windows path)
  ([Permission configuration files](#permission-configuration-files)).
- Other files: `annotations/<uuid>.pbtxt` (46 B), `bin/agentapi.bat`
  (68 B), `bin/webm_encoder.exe` (12,472,320 B),
  `cache/default_project_id.txt` (19 B), `cache/last_conversations.json`
  (70 B, one absolute path to a UUID), `cache/marketplaces/` (17,712 B),
  `cache/onboarding.json` (111 B; `consumerOnboardingComplete`,
  `enterpriseOnboardingComplete`, `onboardingComplete`, booleans),
  `implicit/<uuid>.pb` (226 B), `installation_id` (36 B),
  `jetbox_summaries_proto.pb` (430 B), `jetski_state.pbtxt` (503 B),
  `last_check.timestamp` (0 B), `presence/<uuid>.lock` (0 B), an empty
  `crashes/`, and the same `builtin/` skills. There is no
  `antigravity-oauth-token` ([Credentials](#credentials)).

#### Shared configuration directory

`~/.gemini/config/`, shared by the CLI and the app (binary changelog,
live names, observed 232 KB on macOS and 320 KB on Linux): `mcp_config.json`, `hooks.json`,
`config.json`, `projects/<uuid>.json`, `skills/<name>/SKILL.md`,
`plugins/<name>/`, `rules/*.md`, `workflows/`, `global_workflows/`,
`sidecars/`, `GEMINI.md`, `AGENTS.md`, `skills.json`, `rules.json`,
`plugins.json`, `agents.json`, `workflows.json`, `.migrated`. The binary
also reads `~/.gemini/GEMINI.md` and `~/.gemini/AGENTS.md`, and names
legacy `~/.gemini/jetski/brain` and per-repo `.antigravitycli` workspace
dirs (both superseded). Observed on macOS:

- `config.json` (657 B): `plugins` {`gdocs` {`installedFrom` {`id`,
  `marketplace`, `version`}}} and `userSettings` {`globalPermissionGrants`
  {`allow`, an array of strings}, `permissionGrantsV2Migrated`,
  `remoteControlHostname`, `sandboxEnabledAtV2Migration`}. On Linux
  (1,356 B, 22:00) it also has `sidecars` {`<name>` {`argumentValues`
  (null), `enabled` (bool), `projectId` (UUID), `resolvedPermissions`
  {`accessGrants`, an array of 2 strings}, `schemaVersion` (number)}},
  one entry per scheduled automation. On Windows (3,332 B, app 2.23.1) it
  has `plugins` as on macOS and `userSettings` {`globalPermissionGrants`
  {`allow`, an array of strings}, `remoteControlHostname`}: no
  `sidecars` key although an automation was saved, and neither
  `permissionGrantsV2Migrated` nor `sandboxEnabledAtV2Migration`. The
  grant counts per install are in
  [Permission configuration files](#permission-configuration-files).
- `plugins/gdocs/`: the installed plugin, `plugin.json`, `README.md`,
  `skills/gdocs/SKILL.md`, nine reference documents and the launcher
  scripts `skills/gdocs/scripts/gdocs` (shell) and `gdocs.cmd`. On
  Windows: `plugin.json` 429 B, `README.md` 154 B, `SKILL.md` 41,701 B,
  `scripts/gdocs` 2,102 B and `scripts/gdocs.cmd` 1,756 B.
- `projects/<uuid>.json` (283 B), written by the app when a project is
  created: `id`, `isWorkspaceOnly`, `name`, `permissionGrants`,
  `projectResources` {`resources`, an array of objects whose `folderUri` is a `file://` URL},
  `settings`; and `projects/default-cli-project.json` (84 B, from the CLI)
  with an empty `projectResources`. The file is pretty-printed: on Linux
  (293 B) the `folderUri` key is followed by a colon, two spaces and the
  quoted `file:///` URL. Linux also had a second shape (382 B, 22:00) with
  no folder: `id`, `name`, `permissionGrants` {`permissionGrants`
  {`allow`, an array of 2 strings}, `v2Migrated`}, an empty
  `projectResources` and `settings` {`artifactReviewMode`}; by its time
  and its grants, the project the scheduled automation runs under (the
  `config.json` `sidecars` entry has a `projectId`). On Windows (243 B,
  app 2.23.1) the project file has `id`, `isWorkspaceOnly`, `name`,
  `projectResources` and an empty `settings`, with no `permissionGrants`,
  and its `folderUri` is `file:///c%3A/Users/<user>/demo`: a lower-case
  drive letter and a percent-encoded colon. `default-cli-project.json`
  (84 B) is as on macOS, and there is no folder-less automation project.
- `sidecars/<name>/sidecar.json` (725 B), the automation the user
  scheduled. A sidecar is the app's name for a long-running helper
  process that its language server supervises; the app starts the server
  with `--enable_sidecars` (`app.asar:dist/languageServer.js@4242552`
  `'--enable_sidecars',`). The file's keys are `builtin` ("schedule", the
  built-in sidecar that runs an agent task on a schedule),
  `display_name`, `description`, `args` (an array of 5 strings), `restart_policy`
  ("always") and `agent_permissions` {`access_grants`, an array of strings,
  counted in [Permission configuration files](#permission-configuration-files)}.
  The Linux `sidecar.json` (791 B) has the same keys, and so does the
  Windows one, `sidecars/weekly-microsoft-security-bulletins/sidecar.json`
  (1,005 B, app 2.23.1). Nothing outside the app starts the schedule: no
  launchd job or cron entry on macOS; no `~/.config/autostart` entry, user
  systemd unit or crontab entry on Linux, where `systemctl --user` showed
  only the running app's transient `app-antigravity-<pid>.scope`; and no
  `Run` key entry, scheduled task or Startup folder entry on Windows. So
  the schedule runs only while the app's language server runs. The
  scheduler's log, and what the Windows install wrote instead of one, are
  in [Sidecar logs](#sidecar-logs).
- `mcp_config.json` and `.migrated`, both empty (also on Windows).
- `node_modules/sidecar_sdk/` (observed, Linux, 5 files, 17,508 B): a copy
  of the sidecar SDK, excluded ([Exclusions](#exclusions)). Absent on
  Windows.

#### Electron user-data directory

Electron keeps each app's own state, Chromium's storage included, in a
per-user directory that `app.getPath('userData')` returns, named after the
app under the per-OS application-data root. The root is
`~/Library/Application Support` on macOS and `%APPDATA%` on Windows
(`app.asar:node_modules/electron-log/src/node/NodeExternalApi.js@2235471`
`return path.join(home, 'Library/Application Support');` and @2235565
`return process.env.APPDATA || path.join(home, 'AppData/Roaming');`) and
`$XDG_CONFIG_HOME` or `~/.config` on Linux
([Electron app API](https://www.electronjs.org/docs/latest/api/app),
`app.getPath('appData')`, fetched 2026-10-13; the bundle snippets cover
macOS and Windows only). So the directory is
`~/Library/Application Support/Antigravity` (observed),
`~/.config/Antigravity` (observed on Linux, app 2.23.1) and
`%APPDATA%\Antigravity` (observed on Windows 11 ARM64 build 26300, app
2.23.1, 2026-10-14). The app stores UI state there
(`app.asar:dist/paths.js@4313059` `function getAppStoragePath() {` ...
`path_1.default.join(electron_1.app.getPath('userData'), 'app_storage.json')`,
"lightweight key-value store for UI state") and a Node wrapper for plugin
scripts (`app.asar:dist/utils.js@4399333` and @4399399):

```text
const binPath = path_1.default.join(userDataPath, 'bin');
const nodeWrapperPath = path_1.default.join(binPath, os === 'win32' ? 'agy-node.cmd' : 'agy-node');
```

Observed on macOS (5.3 MB, created 18:28): `app_storage.json` (1,528 B;
string values under the keys `antigravity-multi-conversation-layout-v3-<uuid>`
x4 and `-index`, `aux-pane-session`, `comments`,
`didAskForNotificationPermission`, `ide-install-wizard-shown`,
`jetski.onboarding.lastLoginIsGcpTos`, `jetski.onboarding.lastLoginUsername`
(the signed-in account name), `lastCreatedProjectId`,
`new-convo-last-selected-project`, `new-convo-magic-workspace-selected`,
`projectsOrder`), `bin/agy-node` (a 152 B `#!/bin/sh` wrapper that runs
the bundled `Antigravity Helper` with `ELECTRON_RUN_AS_NODE=1`),
`Preferences`, `Local State`, `Network Persistent State`, `.updaterId`,
`DevToolsActivePort`, `Cookies` (SQLite, 0 rows), `Local Storage/leveldb/`
(16 KB), `Session Storage/` (16 KB), `DIPS` + `DIPS-wal` (260 KB),
`Trust Tokens`, `declarative_performance_observer.db`, the symlinks
`SingletonCookie`, `SingletonLock`, `SingletonSocket`, and the Chromium
caches `Cache` (3.2 MB), `Code Cache` (52 KB), `GPUCache`,
`DawnGraphiteCache`, `DawnWebGPUCache` (548 KB each), `Shared Dictionary`
(52 KB) and `blob_storage`. There is no `User/` and no `logs/`, the
subdirectories the pre-2.0 catalog lines named.

Observed on Linux (`~/.config/Antigravity/`, 8.9 MB, created 21:29):
`app_storage.json` (1,681 B; the macOS keys plus `aux-pane-v2-session`
and `sidebar_collapsed_sections`), `bin/agy-node` (80 B), `logs/main.log`
(2,128 B) and `logs/language_server.log` (135,703 B), `Cookies`
(20,480 B), `Local Storage/leveldb/` (6 files, 628 B), `Session Storage/`
(6 files, 642 B), `DIPS` (36,864 B) and `DIPS-wal` (115,392 B),
`Trust Tokens` (36,864 B), `declarative_performance_observer.db`
(32,768 B), `DevToolsActivePort`, `Local State` (57 B;
`uninstall_metrics.installation_date2`), `Network Persistent State`
(944 B), `Preferences` (132 B; `electron.media.device_id_salt`,
`spellcheck.dictionaries`, `spellcheck.dictionary`), the `Singleton*`
symlinks, and the caches `Cache` (6,959,104 B), `Code Cache`
(114,688 B), `GPUCache` (614,400 B), `DawnGraphiteCache` and
`DawnWebGPUCache` (307,200 B each), `Shared Dictionary` (65,536 B),
`blob_storage` (8,192 B) and `Crashpad/` (8,192 B; a 36 B `client_id`,
Crashpad being Chromium's crash reporter). New against macOS:
`Dictionaries/en-US-10-1.bdic` (451,968 B), the spell-check dictionary
Chromium downloads in its binary `.bdic` format, excluded
([Exclusions](#exclusions)).

Observed on Windows (`%APPDATA%\Antigravity\`, created 23:56:11Z, app
2.23.1, Windows 11 ARM64 build 26300, 2026-10-14): `app_storage.json`
(1,545 B; the macOS key set, without the two Linux additions),
`bin\agy-node.cmd` (107 B, the Windows name the bundle predicts),
`.updaterId` (36 B), `DevToolsActivePort` (60 B), `Preferences` (132 B;
`electron.media.device_id_salt`, `spellcheck.dictionaries` (one string),
`spellcheck.dictionary`), `Local State` (490 B; `os_crypt`
{`audit_enabled` (bool), `encrypted_key` (a 380-character string)} and
`uninstall_metrics` {`installation_date2`}, see
[Credentials](#credentials)), `logs\main.log` (4,751 B, 51 lines) and
`logs\language_server.log` (218,747 B), `Network\Cookies` (20,480 B) and
`Cookies-journal`, `Network Persistent State` (472 B),
`NetworkDataMigrated` (0 B), `Network\Trust Tokens` (36,864 B)
and its `-journal`, `DIPS` (4,096 B) and `DIPS-wal` (210,152 B),
`declarative_performance_observer.db` (32,768 B) and its `-journal`,
`Local Storage\leveldb\` (`000003.log` 49 B, `CURRENT`, `LOCK`, `LOG`
251 B, `MANIFEST-000001`; LevelDB is the key-value store format Chromium
uses for web storage), `Session Storage\` (`000003.log` 68 B and the
other LevelDB files), `lockfile` (0 B, the Windows form of the
`Singleton*` symlinks, which do not exist here), and the caches
`Cache\Cache_Data` and `Cache\No_Vary_Search` (9,368,215 B in all),
`Code Cache` (22,862 B), `DawnGraphiteCache`, `DawnWebGPUCache` and
`GPUCache` (557,424 B each), `Shared Dictionary` (45,128 B) and an empty
`blob_storage\<uuid>\`. There is no `Crashpad\`, no `Dictionaries\`, no
`CachedData` and no `User\`. While the app runs it holds
`Network\Cookies`, `Cookies-journal` and the `LOCK` files of
`Local Storage\leveldb\` and `Session Storage\` locked against other
readers, and keeps `logs\language_server.log`, `DIPS-wal` and the two
`000003.log` files open while it writes to them.
`%APPDATA%\antigravity` (lower case) is the same directory, as expected
on a case-insensitive filesystem, and there is no
`%APPDATA%\Antigravity IDE`.

#### Logs

Electron's logs directory is `~/Library/Logs/<name>` on macOS
and `<userData>/logs` elsewhere
(`app.asar:node_modules/electron-log/src/main/ElectronExternalApi.js@2216282`
`return this.getPath('logs') || super.getElectronLogPath();` and
`app.asar:node_modules/electron-log/src/node/NodeExternalApi.js@2233256`
`return path.join(this.getAppUserDataPath(appName), 'logs');`), and the
backend's log goes there too (`app.asar:dist/paths.js@4313430`
`function getLsLogPath() {` ...
`path_1.default.join(electron_1.app.getPath('logs'), constants_1.LS_LOG_FILE_NAME)`;
`app.asar:dist/constants.js@4136120`
`exports.LS_LOG_FILE_NAME = 'language_server.log';`). So the logs are in
`~/Library/Logs/Antigravity` (observed), `~/.config/Antigravity/logs`
(observed on Linux) and `%APPDATA%\Antigravity\logs` (observed on
Windows 11 ARM64 build 26300, app 2.23.1, 2026-10-14).

Observed on macOS: `main.log` (1,696 B; lines
`[YYYY-MM-DD HH:MM:SS.mmm] [info]  <message>`, among them
`Starting app (v2.23.0) with dynamic port…`,
`Host bridge server listening on http://127.0.0.1:<port>`,
`LS Logs:     <path>`, `Electron Logs: <path>`, the IDE wizard line of
[Data directories under `~/.gemini`](#data-directories-under-gemini),
`[AutoUpdater] ...` and an hourly `Checking for update`) and
`language_server.log` (160,403 B; lines in the format of glog, Google's
logging library: `ERROR: logging before google.Init: I|E|W<MMDD HH:MM:SS.ffffff> ...`).
The server log names `~/.gemini/config/projects/outside-of-project.json`
174 times (the file does not exist),
`~/.gemini/antigravity-cli/conversations/<uuid>.pb` twice, and the plugin
script, the scratch document and the two `bin/` entries once each.

Observed on Linux: `main.log` has the macOS lines with `v2.23.1`, plus the
AppImage warning
([Program files and install metadata](#program-files-and-install-metadata))
and
`Shutting down language server…`; `language_server.log` names
`~/.gemini/config/projects/outside-of-project.json` 194 times (absent)
and `~/.gemini/antigravity-cli/conversations/<uuid>.pb` once.

Observed on Windows: `main.log` (51 lines) has the
`Starting app (v2.23.1) with dynamic port…` line and, beyond the
templates above (digits and ids redacted):
`Generated new staging user ID: <uuid>`,
`Host bridge server listening on http://127.0.0.1:<port>`,
`[Auto-Restart] Port changed! Reloading all windows with URL: https://127.0.0.1:<port>/`,
`Local: https://127.0.0.1:<port>/`,
`LS Logs: <home>\AppData\Roaming\Antigravity\logs\language_server.log`,
`Electron Logs: <home>\AppData\Roaming\Antigravity\logs\main.log`,
`[IDE Wizard] <home>\.gemini\antigravity not found — user was not migrated, skipping.`,
`Update for version 2.23.1 is not available (latest version: 2.23.1, downgrade is allowed).`,
`[AutoUpdater] Up to date (2.23.1)`, and three WSL errors (see
[WSL](#wsl)). The sidecar lines of `language_server.log` are in
[Sidecar logs](#sidecar-logs).

#### macOS preferences file

`~/Library/Preferences/com.google.antigravity.plist` (916 B, observed)
holds only AppKit window and panel keys (`AppleTextDirection`,
`NSForceRightToLeftWritingDirection`, `NSNavPanelExpandedSizeForOpenMode`,
`NSOSPLastRootDirectory`, `NSTreatUnknownArgumentsAsOpen`) and has no
catalog line.

#### Antigravity IDE

The IDE (community, consistent across four sources):
`~/.gemini/antigravity-ide/` with `conversations/<uuid>.db`,
`brain/<uuid>/` (Markdown artifacts `task.md`, `implementation_plan.md`,
`walkthrough.md`, each with `.metadata.json`, plus
`.system_generated/logs/transcript.jsonl`),
`knowledge/<ki-id>/{metadata.json, artifacts/}`, `knowledge.lock`
(damadorPL); docs/settings calls `~/.gemini/antigravity-ide/` the "local
app data directory". The upgrade leftover `~/.gemini/antigravity-backup/brain`
is reported by jazzyalex, and matches the bundle's backup directory. The
IDE's VS Code-fork app directory is named `Antigravity IDE`:
`%APPDATA%\Antigravity IDE\User\globalStorage\state.vscdb` (Devilaiger,
marioalbu08), `~/.config/Antigravity IDE/User/globalStorage/state.vscdb`
(dev.to); macOS by convention `~/Library/Application Support/Antigravity IDE`.
The IDE installs its program to `/Applications/Antigravity IDE.app`,
`%LOCALAPPDATA%\Programs\Antigravity IDE\` and
`~/.local/share/antigravity-ide` (`app.asar:dist/ideInstall/constants.js@4162112`
`function getIdeInstallPath() {` with `'/Applications/Antigravity IDE.app'`,
`'Programs', 'Antigravity IDE', 'Antigravity IDE.exe'` and
`path.join(os.homedir(), '.local', 'share', 'antigravity-ide')`); those are
binaries, not state, and have no catalog line.

#### WSL

On Windows the app can run its language server inside a WSL
(Windows Subsystem for Linux) distribution, installed at
`~/.antigravity-server/bin/<version>/` in the Linux home
(`app.asar:dist/wsl.js@4411665`
`` return `$HOME/.antigravity-server/bin/${version}`; ``; bundle-derived,
not observed on any real install). The directory holds the server binary;
where that server keeps conversations is not determined. The Windows
install (app 2.23.1, Windows 11 ARM64 build 26300, 2026-10-14, no WSL
distribution installed) shows the app listing WSL distributions at
start-up and for a "connect" menu: `main.log` has twice

```text
[error] Failed to list WSL distros: Error: wsl.exe --list --verbose failed: Command failed: C:\WINDOWS\System32\wsl.exe --list --verbose
```

with the OS text "The Windows Subsystem for Linux is not installed. You
can install by running 'wsl.exe --install'", from `dist\wsl.js`
(`execWsl`, `listWslDistros`), and once `[error] wsl:get-state failed: ...`
from `dist\menu.js` (`addWslConnectMenu`, `wslConnectMenuTemplate`) and
`dist\ipcHandlers.js`. The directory inside a distribution remains
unobserved.

#### System-level settings and environment variables

Enterprise `admin_settings.json` is system-level: `/etc/antigravity/`,
`/Library/Application Support/Antigravity/`, `...\Antigravity\` (binary).

Relocation env vars: none found for the home dir. `ANTIGRAVITY_EXECUTABLE_DATA_DIR`,
`AGY_ACCOUNT`, `AGY_ADC_AUTH`, `ANTIGRAVITY_PROJECT_ID`,
`ANTIGRAVITY_CONVERSATION_ID` exist (binary) but their effect on paths is
unknown.

### Credentials

- Keyring first: `zalando/go-keyring` with Secret Service, service name
  `jetski-standalone-oauth-token` (binary); docs/cli/install: Apple
  Keychain, Linux Secret Service, Windows Credential Manager. File fallback
  when the keyring is "bypassed or unreachable" (binary changelog) is
  `~/.gemini/antigravity-cli/antigravity-oauth-token` (live, 1,670 B; the
  whole literal is not in the strings, so the name is assembled at runtime).
  Flagged by `*.gemini/antigravity-cli/antigravity-oauth-token`. The
  app's `~/.gemini/antigravity/` held no token file on the macOS install
  (observed).
- On Windows the keyring is the Windows Credential Manager (observed, CLI
  1.3.5, Windows 11 ARM64 build 26300, 2026-10-14). `cli.log` has
  `auth.go:NNN] ChainedAuth: authenticated via keyring (effective: keyring)`
  and `keyring.go:NN] keyringAuth: loaded token, expiry=<date> expired=false`,
  and no `Using file-based secret storage` line; there is no
  `antigravity-oauth-token` file and no `plugin-secrets.json` anywhere
  under the profile (searched by name). An ASCII search over the 1.3.5
  binary finds the literals `plugin-secrets.json`,
  `Using file-based secret storage`, `wincred` and `CredWrite`, the last
  two the Go keyring's Windows backend and the Windows API call that
  writes a credential. `cmdkey /list` (the Windows
  command that lists Credential Manager entries), run from an SSH (remote
  shell) session, which is a different logon session, listed no entries, so the
  entry's name is not determined.
- `plugin_data/<plugin>/token.json` under a `.gemini` data directory:
  the Google Docs plugin's OAuth credential (observed,
  `~/.gemini/antigravity/plugin_data/gdocs/token.json`, 1,119 B on macOS
  and 1,126 B on Linux, both mode 0600, and 1,122 B on Windows, app
  2.23.1), its
  keys in [Secrets](#secrets). `*.gemini/antigravity*/plugin_data/*/token.json`
  flags it in every `.gemini/antigravity*` directory
  ([issue 84](https://github.com/seanthegeek/doubleagent/issues/84)).
- `plugin-secrets.json`, the CLI's file store for plugin secrets when no
  OS keychain is usable ([issue 90](https://github.com/seanthegeek/doubleagent/issues/90)). Each CLI
  1.3.4 log on Linux has two lines (observed, home redacted)

  ```text
  I<MMDD HH:MM:SS.ffffff> <n> auth_store.go:136] Using file-based secret storage at ~/.config/antigravity/plugin-secrets.json
  ```

  and the binary carries the literal
  `plugin-secrets.json` beside `keychain get %s: %w` and
  `keychain set %s: %w` (observed, `agy` 1.3.4 linux-arm64, sha256 in
  [Source and evidence](#source-and-evidence)). The file did not exist on
  the install, since nothing was authorised from the CLI, and neither did
  `~/.config/antigravity`. The lower-case directory under `~/.config` is
  Go's `os.UserConfigDir()` on Linux; the macOS and Windows paths,
  `~/Library/Application Support/antigravity/plugin-secrets.json` and
  `%APPDATA%\antigravity\plugin-secrets.json`, are inferred from
  `os.UserConfigDir()`, not observed; the Windows install of CLI 1.3.5
  used the Credential Manager and wrote no such file, though its binary
  carries the literal. On those case-insensitive
  filesystems the directory is the app's Electron user-data directory
  when that exists. The three paths are catalog entries, and
  `.config/[Aa]ntigravity/plugin-secrets.json`,
  `Library/Application Support/[Aa]ntigravity/plugin-secrets.json` and
  `AppData/Roaming/[Aa]ntigravity/plugin-secrets.json` flag them. The
  class accepts either case because whichever product creates the
  directory first fixes its spelling.
- `GEMINI_API_KEY` with `modelProvider: "gemini"` in `settings.json`
  (binary changelog; docs/cli/install); the key is env-only.
- `~/.gemini/config/mcp_config.json` can embed MCP `env` and `headers`
  (binary `McpStdioTransport.EnvEntry`, `McpHttpTransport.HeadersEntry`), so
  `.gemini/config/mcp_config.json` flags it.
- The Electron user-data directory's `Cookies` (SQLite, 0 rows on the
  macOS install, 20,480 B on Linux and Windows, where it is
  `Network\Cookies`), `Trust Tokens` (Chromium's store for
  the Private State Tokens web API, 36,864 B on Linux and Windows) with their
  `-journal` files, and `Local Storage/leveldb/` (16 KB on macOS, 628 B on
  Linux, contents not read) are collected unflagged; whether any of them
  can hold a credential is not determined. The running app holds
  `Network\Cookies` and its journal locked on Windows
  ([Electron user-data directory](#electron-user-data-directory)).
- `Local State` on Windows (490 B) has `os_crypt.encrypted_key`, a
  380-character string: Chromium's key for the cookie store, wrapped with
  DPAPI (the Windows Data Protection API, the per-user encryption the OS
  provides). macOS keeps that key in the Keychain and Linux in the
  Secret Service, so `Local State` holds key material on Windows only
  (the Linux file is 57 B with only `uninstall_metrics`). It
  is collected unflagged: the rest of the file is install metadata, and
  the key is useless without the user's DPAPI context. Treat
  `encrypted_key` as a key to redact before sharing, as
  [Credentials in `claude-desktop.md`](claude-desktop.md#credentials)
  treats Claude Desktop's `metadata_secret`, not as a secret glob.
  `app_storage.json` holds the signed-in account name
  (`jetski.onboarding.lastLoginUsername`), which identifies the user but
  is not a credential.
- IDE `state.vscdb` holds the conversation index
  (`antigravityUnifiedStateSync.trajectorySummaries`, Devilaiger) and, as a
  VS Code fork, probably auth state; collect unflagged.

### Exclusions

Sizes are from the macOS install (observed) unless noted; the Linux
install's are given where they were recorded.

- `.gemini/antigravity/bin`, `.gemini/antigravity-cli/bin`,
  `.gemini/antigravity-ide/bin`, `.gemini/antigravity-backup/bin`: the
  encoder and launcher binaries (app 12,699,366 B in 2 files; CLI
  12,787,712 B on macOS, 17 MB on Linux; on the Linux install of CLI
  1.3.4 and app 2.23.1, 15,872,000 B and 15,876,096 B; on the Windows install of CLI 1.3.5 and app 2.23.1,
  12,472,388 B and 12,472,555 B). One line per directory rather
  than `.gemini/antigravity*/bin`, because `*` in an exclusion crosses
  `/` and that form would also prune any `bin` directory the agent writes
  under `brain/<uuid>/`.
- `.gemini/antigravity*/plugin_data/*/bin`: plugin binaries (the Google
  Docs plugin's, 31,938,752 B; 36,327,424 B on Linux; 31,200,920 B on
  Windows). The plugin's
  `token.json` beside it stays collected.
- `.gemini/antigravity*/cache/marketplaces`: the plugin marketplace cache
  (16 files, 207,829 B; on Linux 282,624 B in the app's directory and
  32,768 B in the CLI's, which has one from 1.3.x; on Windows 207,829 B
  and 17,712 B), a copy of the
  marketplace index and of each downloaded plugin; the installed copy is
  in `~/.gemini/config/plugins/`. The rest of `cache/` is collected.
- `.gemini/antigravity*/brain/*/.system_generated/worktrees` (git
  worktrees of the project, binary).
- `.gemini/antigravity*/lib/node_modules` and `.gemini/config/node_modules`:
  the sidecar SDK the app installs twice (observed on Linux, 5 files and
  17,508 B in each), code rather than state. The `*` crosses `/`, so the
  first also prunes a `lib/node_modules` an agent writes under `brain/`,
  which is a dependency tree as well.
- The Chromium caches in the Electron user-data directory:
  `*/Antigravity/Code Cache`, `*/Antigravity/GPUCache`,
  `*/Antigravity/DawnGraphiteCache`, `*/Antigravity/DawnWebGPUCache`,
  `*/Antigravity/blob_storage`, `*/Antigravity/Shared Dictionary`,
  `*/Antigravity/CachedData`, `*/Antigravity/Crashpad` (observed:
  `Code Cache` 52 KB, `GPUCache`, `DawnGraphiteCache` and
  `DawnWebGPUCache` 548 KB each, `Shared Dictionary` 52 KB; `CachedData`
  and `Crashpad` did not exist on macOS; on Linux `Code Cache` 114,688 B,
  `GPUCache` 614,400 B, `DawnGraphiteCache` and `DawnWebGPUCache`
  307,200 B each, `Shared Dictionary` 65,536 B, `blob_storage` 8,192 B,
  `Crashpad` 8,192 B), `*/Antigravity/Dictionaries` (the spell-check
  dictionary, 451,968 B on Linux), and `Cache` (3.2 MB; 6,959,104 B on
  Linux) as
  `.config/Antigravity/Cache`, `Library/Application Support/Antigravity/Cache`
  and `AppData/Roaming/Antigravity/Cache`. `Cache` is named under each base
  because the PowerShell collector matches without regard to case, and
  `*/Antigravity/Cache` would also prune `.gemini/antigravity/cache`. None
  of the other patterns matches the data directories' lower-case `cache`,
  `crashes` or `bin`. On Windows (observed, app 2.23.1) the set is
  `Cache` 9,368,215 B, `Code Cache` 22,862 B, `DawnGraphiteCache`,
  `DawnWebGPUCache` and `GPUCache` 557,424 B each, `Shared Dictionary`
  45,128 B and `blob_storage` 0 B; there is no
  `Crashpad`, `Dictionaries` or `CachedData`.
- `AppData/Local/agy` (CLI install; `bin\agy.exe` 184,062,616 B on
  Windows) and `.antigravity/extensions`. `.cache/antigravity` (the
  marketplace staging directory) and its Windows form
  `AppData/Local/antigravity` are collected; they were empty on every
  install checked.
- `Antigravity IDE` Electron caches (`Cache`, `CachedData`, `GPUCache`,
  `Code Cache`, by VS Code convention) are outside the collected `User`
  and `logs` subdirectories.

Kept: `.system_generated/subagents` (sub-conversation data), `scratch/`,
`builtin/` (bundled skills, 176 KB in the CLI directory) and the Electron
`Local Storage/`, `Session Storage/` and `DIPS`.

### Project-local files

`.agents/` workspace customizations (`.agents/agents/`, binary changelog;
damadorPL `<project-root>\.agents\`), `.antigravityignore` (binary),
`.antigravity-plugin` (binary), legacy `.antigravitycli/` (binary, removed
in favour of `cache/projects.json`). No per-project database.

### Where the project path is recorded

- `history.jsonl`: `workspace` per prompt (CLI only; the app writes no
  `history.jsonl`), see
  [Transcript stores](#transcript-stores).
  On Windows it is the folder as the user typed it, with a lower-case
  `users` (`C:\users\<user>\demo2`, observed, CLI 1.3.5).
- `settings.json`: `trustedWorkspaces` array (binary json tag; docs/settings).
- `cache/projects.json`: workspace to project map (binary changelog).
- `history.jsonl` also carries `conversationId` on some lines from CLI
  1.3.x (observed), which joins a prompt to its conversation
  ([Joins and timestamps](#joins-and-timestamps)).
  The collectors read only `workspace`.
- `config/projects/<uuid>.json`: `projectResources.resources[].folderUri`,
  a `file://` URL of the project folder (observed on macOS, Linux and
  Windows, written by the app; the folder-less shape in
  [Shared configuration directory](#shared-configuration-directory) has
  none). `Find-Projects` in the PowerShell collector and `discover_projects`
  in the sh collector read these URLs. JSON tags present in the binary:
  `workspace_uris`, `workspace_paths`, `workspace_uri`, `project_id`,
  `display_name`, `cwd`.
- `antigravity-cli/cache/last_conversations.json`: an object whose keys
  are absolute workspace paths (observed). Not read by the collectors.
- On Windows `trustedWorkspaces`, the `history.jsonl` `workspace` and the
  `last_conversations.json` keys are Windows paths (`C:\...`), which the
  sh collector does not use for discovery (see
  [which paths are used in `collectors/docs/coverage.md`](../collectors/docs/coverage.md#which-paths-are-used)).
- The Windows install (observed, CLI 1.3.5 and app 2.23.1, 2026-10-14)
  recorded its two project folders in three spellings:
  `file:///c%3A/Users/<user>/demo` in the app's `file:` URIs, as the
  project file's `folderUri` records it (a percent-encoded colon and a
  lower-case drive letter); a `file:` URI of `C:/users/<user>/demo2` in the CLI
  conversation's `conversation_summaries.workspace_uris`; and the plain
  path `C:\users\<user>\demo2` in the `history.jsonl` `workspace`. The
  lower-case `users` is how the user typed the CLI's working folder. How
  the parser turns them into paths is in
  [Joins and timestamps](#joins-and-timestamps).
- `conversation_summaries.db` (`workspace_uris` column) and the
  per-conversation `*.db`: SQLite only, read by the analyzer.

## Parsing

### Transcript stores

Conversations can be in any of the four data directories of
[Data directories under `~/.gemini`](#data-directories-under-gemini):
`.gemini/antigravity-cli/` (the CLI), `.gemini/antigravity/` (the desktop
app 2.x, and the IDE before 2.0), `.gemini/antigravity-ide/` and
`.gemini/antigravity-backup/`. Only the first two were present on the
macOS, Linux and Windows installs (**observed**); the IDE's two are
bundle-derived, not observed on any real install.

Stores in each data directory (`<dir>` is one of the four above):

| Path | Content |
| --- | --- |
| `<dir>/conversations/<uuid>.db` (+`-wal`, `-shm`) | one SQLite database per conversation, WAL mode; the app's four had no sidecars when listed on macOS or Linux, nor did the app's four and the CLI's one on Windows (**observed**) |
| `<dir>/conversation_summaries.db` | one row per conversation; the app's copy adds a full-text search table, and the CLI 1.3.5 copy on Windows also held a row for each of the app's conversations ([Summaries database](#summaries-database)) |
| `.gemini/antigravity-cli/history.jsonl` | `{display, timestamp (ms), workspace}` per prompt; CLI only, the app writes none (**observed**). From CLI 1.3.x some lines add `conversationId`, the conversation's UUID (**observed**, CLI 1.3.4 on Linux and 1.3.6 on macOS); which lines carry it is in [Joins and timestamps](#joins-and-timestamps) |
| `.gemini/antigravity-cli/cache/conversation_metadata.json` | CLI 1.3.5 (**observed**, Windows, 2,671 B; not seen on macOS with 1.2.16 or Linux with 1.3.4): `conversations` {`<uuid>`: {`is_internal` (bool), `last_modified_time` (ISO 8601 date), `summary` {`AgentName` (empty string), `AppDataDir` (11 characters), `GroupID` (empty string), `ID` (UUID), `Internal` (bool), `Loaded` (bool), `NumSteps` (number), `Preview` (string), `ProjectID` (18 characters), `Title` (string), `UpdatedAt` (ISO 8601 date), `WorkspaceURIs` (null)}}}; its four entries were the desktop app's four conversations, and an 11-character `AppDataDir` is `antigravity`, the app's data directory. The parser does not read it |
| `.gemini/antigravity-cli/cache/last_conversations.json` | {`<absolute path>`: `<uuid>`} (**observed**, Windows, 70 B): by its name, the last conversation per working folder; the parser does not read it |
| `.gemini/antigravity-cli/jetbox_summaries_proto.pb` | `jetbox_summaries_pb.SummariesState`, same summaries as the database; the app's counterpart is `.gemini/antigravity/agyhub_summaries_proto.pb` (1,436 B, **observed**; 1,417 B on Windows) |
| `<dir>/annotations/<uuid>.pbtxt` | seen in the app's directory (**observed**, 94-99 B each on macOS, 93-101 B on Linux, 93-96 B on Windows) and, from CLI 1.3.x, in the CLI's (**observed**, Linux, 37 B; Windows, 46 B): pbtxt ([Desktop app data directory](#desktop-app-data-directory)), `field: value` lines, with a single `title:` field, the conversation title |
| `<dir>/brain/<uuid>/` | per-conversation artifacts and logs, in the next table |
| `.gemini/antigravity-cli/antigravity-oauth-token` | credential, flagged secret by the collector; absent on the Windows install, where the CLI kept its token in the OS keyring ([Credentials](#credentials)) |
| `.gemini/antigravity/plugin_data/<plugin>/token.json` | a plugin's OAuth credential ([Secrets](#secrets)) |
| `<dir>/bin/` | binaries, excluded |
| `.gemini/antigravity/sidecar_data/<name>/logs/sidecar.log`, `sidecar_data/<name>/data/` | the scheduler log of the sidecar `<name>` and an empty `data/` directory (**observed**, Linux; absent on Windows; [Sidecar logs](#sidecar-logs)) |

The brain directory, `<dir>/brain/<uuid>/`, has the same layout for the CLI
and the app (**observed**, macOS, Linux and Windows):

| Path | Content |
| --- | --- |
| `.system_generated/logs/transcript.jsonl` | one JSON line per step, long fields shortened (17,832 to 155,131 B on the app; on Windows 12,720, 17,119, 56,481 and 55,052 B for the app's four conversations) |
| `.system_generated/logs/transcript_full.jsonl` | the same lines with nothing shortened (on Windows 12,357, 17,053, 163,340 and 63,094 B for the same four) |
| `.system_generated/logs/chunks/transcript/0000000N.jsonl`, `chunks/transcript_full/0000000N.jsonl` | the same lines split into chunks of about 100 KB; a line can be cut at a chunk boundary (one macOS app conversation had two `transcript_full` chunks, sizes not recorded, and `shape.py` counted one bad line in each). On Windows the 163,340 B `transcript_full.jsonl` was split into `chunks/transcript_full/00000000.jsonl` of 102,400 B (exactly 100 KiB) and `00000001.jsonl` of 60,940 B |
| `.system_generated/steps/<idx>/output.txt` | the output of each tool step (10 to 4,426 B on macOS, 9 to 8,248 B on Linux) |
| `.system_generated/steps/<idx>/content.md` | Markdown beside that step's `output.txt` (**observed**, Linux: one file of 24,820 B, step 20 of the 30-step app conversation; Windows: one of 2,682,163 B beside a 336 B `output.txt`, step 26 of the app conversation that, by modification time, set up the automation); not seen on macOS |
| `.system_generated/messages/<uuid>.json`, `messages/read.json` | app only, seen in a 43-step conversation on each platform: three messages addressed to the conversation on macOS, two of them tied to a `run_command` tool call; two on Linux, one of them tied to `run_command`; two on Windows (1,400 and 2,010 B, with an 89 B `read.json`), in the conversation that by modification time was the research task; keys in [Brain message files](#brain-message-files) |
| `.system_generated/messages/undelivered/` | an empty directory in that Windows conversation (**observed**, Windows only); by its name, messages not yet delivered to the conversation |
| `.system_generated/tasks/task-<idx>.log` | app only, same conversations: plain-text output of a background task started by step `<idx>` (two on macOS; one on Linux, `task-10.log`, 996 B; one on Windows, `task-7.log`, 988 B); the CLI's brain directories have no `messages/` or `tasks/` |
| `.system_generated/worktrees`, `.system_generated/subagents` | named by the binary (binary): git worktrees of the project, excluded, and sub-conversation data, kept ([Exclusions](#exclusions)) |
| `scratch/`, `.user_uploaded/` | empty directories on every install so far |

#### What survives a delete

What Antigravity's own delete of a conversation removes, in the CLI or the
app, is not stated: no real install has deleted one, and whether the app
removes the conversation database before the brain directory is not
observed. Deleting the project directory removes none of it: on the macOS
CLI 1.3.6 install of 2026-10-15 the conversation database, its
`brain/<uuid>/` mirror, the `history.jsonl` lines and the summaries row
stayed in place, and the whole session parsed after the directory was gone
([Observed](#observed)).

The per-prompt history file, `.gemini/antigravity-cli/history.jsonl`, is the
CLI's alone: each line has the prompt, its time in milliseconds and the
workspace, and from CLI 1.3.x some lines also the conversation id. The
second copies are the brain mirror (`transcript_full.jsonl`,
`transcript.jsonl` and their chunks), which survives what the database
survives and is lost with the data directory; the summaries database with
the app's full-text search table; the protobuf summaries files; the title in
`annotations/<uuid>.pbtxt`; and, on the CLI 1.3.5 install, the CLI's summary
rows and `cache/conversation_metadata.json` entries for the app's
conversations ([Summaries database](#summaries-database)). Whether a compaction or a
rewind rewrites the mirror is not observed, and no deletion marker is
stated. The analyzer reads `history.jsonl` and the brain mirror as secondary
sources and each summaries row as its own `conversation summary` row, and
does not read the annotations, the protobuf summaries, the full-text search
table or the CLI's cache (see
[Secondary sources](../analyzer/docs/output.md#secondary-sources)).

The mirror sits under `brain/<uuid>/`, beside `conversations/<uuid>.db` in
the same data directory. It is a second copy when the database alone is
lost, damaged or excluded: a database whose steps no longer decode, a
`-wal` sidecar the collection missed, a database file the collector skipped
for size, or a conversation the user deleted if the app removes the
database before the brain directory (not observed). The chunked copies
keep the transcript when `transcript_full.jsonl` itself was skipped for
size.

### Record schema

#### Conversation database

```sql
CREATE TABLE trajectory_meta (trajectory_id text, cascade_id text, trajectory_type integer, source integer, PRIMARY KEY (trajectory_id));
CREATE TABLE steps (idx integer, step_type integer, status integer, has_subtrajectory numeric, metadata blob, error_details blob,
                    permissions blob, task_details blob, render_info blob, step_payload blob, step_format integer, PRIMARY KEY (idx));
CREATE TABLE gen_metadata (idx integer, data blob, size integer, PRIMARY KEY (idx));
CREATE TABLE executor_metadata (idx integer, data blob, PRIMARY KEY (idx));
CREATE TABLE parent_references (idx integer, data blob, PRIMARY KEY (idx));
CREATE TABLE trajectory_metadata_blob (id text DEFAULT "main", data blob, PRIMARY KEY (id));
CREATE TABLE battle_mode_infos (idx integer, data blob, PRIMARY KEY (idx));
```

The desktop app's databases have the identical schema (**observed**).
The tables mirror `gemini_coder.Trajectory`: `steps` (field 2),
`generator_metadata` (3), `trajectory_type` (4), `parent_references` (5),
`cascade_id` (6), `metadata` (7), `source` (8), `executor_metadatas` (9),
`battle_mode_infos` (10). `cascade_id` equals the conversation id and the
file name. `source` is a `CortexTrajectorySource` ([Joins and timestamps](#joins-and-timestamps));
`trajectory_type` 4 is `CORTEX_TRAJECTORY_TYPE_CASCADE`.

`steps.step_payload` is a serialized `gemini_coder.Step`:

| Field | Name | Type |
| --- | --- | --- |
| 1 | `type` | `exa.cortex_pb.CortexStepType` (14 USER_INPUT, 15 PLANNER_RESPONSE, 132 GENERIC, 2 FINISH, 23 CHECKPOINT, 28 COMMAND_STATUS, 17 ERROR_MESSAGE, 101 SYSTEM_MESSAGE, ...) |
| 4 | `status` | `CortexStepStatus` (1 PENDING, 2 RUNNING, 3 DONE, 6 CANCELED, 7 ERROR, 9 WAITING, 12 INTERRUPTED) |
| 5 | `metadata` | `CortexStepMetadata` |
| 7..159 | oneof `step` | one message per step kind, field numbers in the parser's `SCHEMA["Step"]`; a protobuf oneof is a group of fields of which at most one is set, so the field present names the step kind |
| 133 | `permissions` | `exa.cortex_pb.TrajectoryPermissions`, also stored in the `steps.permissions` column |
| 148 | `task_details` | `gemini_coder.TaskDetails`, also stored in the `steps.task_details` column |

`steps.metadata` duplicates field 5. `CortexStepMetadata`: 1 `created_at`,
3 `source` (`CortexStepSource`: 2 MODEL, 3 USER_IMPLICIT, 4 USER_EXPLICIT,
5 SYSTEM), 4 `tool_call` (`ChatToolCall`: 1 `id`, 2 `name`, 3
`arguments_json`, 9 `original_name`), 7 `finished_generating_at`, 8
`completed_at`, 11 `generator_model` (enum), 12 `execution_id`, 20
`source_trajectory_step_info`, 26 `internal_metadata.status_transitions`,
30 `tool_summary`, 31 `tool_action`, 32 `started_at`. All timestamps are
`google.protobuf.Timestamp` (seconds, nanos).

Step kinds seen on a real install: `user_input` (19: 1 `query`, 2
`user_response`, 12 `user_config`), `planner_response` (20: 1 `response`,
3 `thinking`, 6 `message_id`, 7 `tool_calls[]`, 12 `stop_reason`, 16
`raw_thinking`) and `generic` (140: 1 `args[]` key/value, 2 `result`
with 1 `result` text and 6 `payload` as `google.protobuf.Any` holding the
typed tool step, here `view_file`). The desktop app's conversations
(**observed**, macOS) also have `system_message` (step type 101, payload
field 114: 1 `message`, 3 `event_type`) and `error_message` (step type 17,
payload field 24: 3 `error` as `CortexErrorDetails` with 1
`user_error_message`, 2 `short_error`, 3 `full_error`). Counts across the
app's four conversations (110 steps): 7 `user_input` (14), 51
`planner_response` (15), 44 `generic` (132), 3 `system_message` (101, all
in the 43-step conversation) and 5 `error_message` (17, all in the 31-step
conversation). Every step had `status` 3 (DONE), `has_subtrajectory` 0 and
`step_format` 0. `steps.permissions` was set on 10 tool steps (52 to 1,174
B) and `steps.task_details` on 2 (239 and 764 B); `error_details` and
`render_info` were empty.

Step kinds other than `user_input`, `planner_response`, `generic`,
`system_message` and `error_message` are decoded from the descriptors but
have not been seen in real data.

The Linux installs (**observed**, CLI 1.3.4 and app 2.23.1) showed the
same three common kinds and `system_message`, and no `error_message`. The
app's four conversations (125 steps): 14 steps (2 `user_input`, 7
`planner_response`, 6 `generic`; `permissions` on none), 43 steps (1, 21,
19 and 2 `system_message`; `permissions` on 6, `task_details` on 1), 30
steps (1, 15, 13; `permissions` on 4) and 38 steps (1, 19, 18;
`permissions` on 9). The CLI's one conversation: 26 steps (2, 13, 11),
`permissions` on 9. In all five, `gen_metadata` had one row per planner
step (7, 21, 15, 19 and 13). The parser reads `permissions` (next) and not
`task_details`.

`steps.permissions` holds an `exa.cortex_pb.TrajectoryPermissions`, the
same message as `gemini_coder.Step` field 133, recording what the user
allowed or denied for that step's tool call. Its fields, from the
descriptors (`third_party/jetski/cortex_pb/cortex.proto`, identical in
`agy` 1.2.16 and `language_server` 2.23.0, macOS arm64):

| Message | Field | Name | Type |
| --- | --- | --- | --- |
| `TrajectoryPermissions` | 1 | `file_access_permissions[]` | `FileAccessPermission` |
| `TrajectoryPermissions` | 2 | `permission_grants[]` | `PermissionGrant` |
| `PermissionGrant` | 1 | `resource` | `PermissionResource` (1 `action`, 2 `target`, both string) |
| `PermissionGrant` | 2 | `allow` | bool |
| `PermissionGrant` | 3 | `scope` | `PermissionScope` |
| `PermissionGrant` | 4 | `user_deny_instruction` | string |
| `PermissionGrant` | 5 | `action` | string |
| `PermissionGrant` | 6 | `target` | string |
| `FileAccessPermission` | 1 | `path` | string |
| `FileAccessPermission` | 2 | `is_directory` | bool |
| `FileAccessPermission` | 3 | `allow` | bool |
| `FileAccessPermission` | 4 | `scope` | `PermissionScope` |
| `FileAccessPermission` | 5 | `from_current_step` | bool |

`exa.cortex_pb.PermissionScope`, how long a grant holds (each name prefixed
`PERMISSION_SCOPE_`): 0 `UNSPECIFIED`, 1 `ONCE`, 2 `CONVERSATION`, 3
`WORKSPACE`, 4 `GLOBAL`, 5 `PROJECT`. Protobuf 3 does not write a false
bool or a zero enum, so a grant with no `allow` is a denial and one with
no `scope` is `UNSPECIFIED`. The column was set on 10 tool steps of the
macOS app's conversations (52 to 1,174 B) and on 28 Ubuntu steps (0, 6, 4
and 9 in the app's four conversations, 9 in the CLI's one; **observed**);
which scope values and which `action` strings occur was not read. On
Windows (**observed**, CLI 1.3.5 and app 2.23.1) the parser yielded 50
`permission grant` rows across the five conversations, three of them for
PowerShell `Get-ChildItem` commands; their scopes were not recorded. The
same five conversations made 72 tool calls: `run_command` 36,
`write_to_file` 21, `view_file` 8, `search_web` 3, `ask_question` 3 and
`read_url_content` 1.

The tool executed by a step is in its `metadata.tool_call`; its `id`
matches the `tool_calls[].id` proposed by the preceding planner step. Tool
payload messages the parser understands: `run_command` (1 `command`, 2
`cwd`, 6 `exit_code`, 21 `combined_output`, 23 `command_line`, 14
`user_rejected`), `command_status`, `shell_exec`, `view_file` (1
`absolute_path_uri`), `list_directory`, `grep_search`, `find`,
`write_to_file` (1 `target_file_uri`, 4 `file_created`), `file_change`,
`code_action` (1 `action_spec` with `command.file`, `create_file.path`,
`delete_file.path`, `sed.path` as `PathScopeItem` 5 `absolute_uri`),
`read_url_content`, `search_web`, `mcp_tool` (1 `server_name`, 2
`tool_call`, 3 `result_string`), `agency_tool_call`, `invoke_subagent` (5
`conversation_id`, 10 `results[].conversation_id`), `ask_question`,
`checkpoint`, `system_message`, `ephemeral_message`, `task_boundary`,
`notify_user`, `error_message`, `finish`, `memory`, `directory_rules`,
`conversation_history`, `send_command_input`.

The paths inside `action_spec` sit at different field numbers per action:
`ActionSpecCommand` 4 `file`, `ActionSpecCreateFile` 2 `path`,
`ActionSpecDeleteFile` 1 `path` and `ActionSpecSed` 3 `path`, each a
`PathScopeItem` (descriptors of `agy` 1.2.16 and `language_server` 2.23.0,
macOS arm64), so `delete_file` does not share the `create_file` layout:
its `path` is field 1, not 2 ([issue 88](https://github.com/seanthegeek/doubleagent/issues/88)). No
`code_action` step has been seen in real data.

`gen_metadata.data` is `CortexStepGeneratorMetadata`: 1 `chat_model`
(`ChatModelMetadata`: 19 `response_model` such as `gemini-3.8-flash`, 21
`model_display_name`, 1 `system_prompt`, 2 `message_prompts[]`), 2
`step_indices[]` (packed). This is the only place the model name appears.
The app's databases have one `gen_metadata` row per planner step
(**observed**); `executor_metadata` had 1-3 rows, `parent_references` and
`battle_mode_infos` none, and `trajectory_metadata_blob` one row of 618-698
B.

Three tool-payload fields carry permission evidence
([issue 97](https://github.com/seanthegeek/doubleagent/issues/97)), all bool and identical in the
`agy` 1.2.16 and `language_server` 2.23.0 descriptors (macOS arm64): `CortexStepRunCommand` 14
`user_rejected` and 34 `ran_in_sandbox`, and `CortexStepMcpTool` 7 `user_rejected`. None of the
three has been seen set in real data, since every observed tool step was a
`generic` step whose typed payload sits in a `google.protobuf.Any` the
parser does not unpack.

`trajectory_metadata_blob.data` is `CortexTrajectoryMetadata`: 1
`workspaces[]` (`CortexWorkspaceMetadata`: 1 `workspace_folder_absolute_uri`,
2 `git_root_absolute_uri`, 4 `branch_name`), 2 `created_at`, 5
`parent_conversation_id`, 6 `root_conversation_id`, 7 `workspace_uris[]`,
17 `nesting_depth`, 18 `project_id`.

#### Brain mirror lines

Brain JSONL line shapes (both `transcript.jsonl` and
`transcript_full.jsonl`, app and CLI, **observed**): every line has
`content`, `created_at`, `source`, `status`, `step_index` and `type`, where
`type` is `USER_INPUT`, `PLANNER_RESPONSE`, `GENERIC`, `SYSTEM_MESSAGE` or
`ERROR_MESSAGE`. `PLANNER_RESPONSE` lines add `thinking`, `tool_calls`,
`input_tokens`, `output_tokens` and `cache_read_tokens`; `ERROR_MESSAGE`
lines add `error`; in `transcript.jsonl` (not `_full`) long `GENERIC` and
`PLANNER_RESPONSE` lines add `truncated_fields`. On Linux (CLI 1.3.4 and app
2.23.1, **observed**) every `GENERIC` line also has an `error` key, which on
macOS appeared only on `ERROR_MESSAGE` lines. The line count equals the
conversation database's step count; the Linux app's four transcripts had
14, 30, 38 and 43 lines and no bad line; the macOS CLI 1.3.6 conversation of
2026-10-15 had 16 `transcript_full.jsonl` lines for 17 database steps
([Observed](#observed)), so the mirror can lag the database by
a step. The token counts appear nowhere the parser reads.

The value forms come from a format description the backend embeds for its
own agent, which is told to read past conversations from these files
(`agy` 1.3.6 macOS arm64, the binary of [Observed](#observed),
sha256 `827ba22b4aa33557614a57aac2cc13f386e17db6b802febd3932226906ac05ab`,
read with `strings` and `grep -b`, never run):

- `agy 1.3.6 darwin-arm64 @54350700` `Transcripts are in JSON Lines (JSONL)
  format.`, one JSON object per step.
- `@54350936` `` `source`: The source of the action (e.g., `USER_EXPLICIT`,
  `MODEL`, `SYSTEM`). ``: the names of `exa.cortex_pb.CortexStepSource`
  without the `CORTEX_STEP_SOURCE_` prefix. The enum also has
  `CORTEX_STEP_SOURCE_USER_IMPLICIT` (`@52896931`), the source of a prompt
  the backend injected rather than one the user typed, which the database
  reader turns into a `system` row; that a mirror line spells it
  `USER_IMPLICIT` is inferred from the other three names, not observed.
- `@54351208` `` `status`: The status of the step (e.g., `DONE`, `ERROR`). ``:
  `CortexStepStatus` names without their prefix, the database's
  `STEP_STATUS` values upper-cased.
- `@54351268` `` `created_at`: The ISO 8601 timestamp of when the step
  occurred. ``: one time per line. The database step has `created_at` and
  `completed_at` ([Conversation database](#conversation-database)); which of them
  the line repeats was not compared on a real install, and the parser
  takes it as `created_at`.
- `@54351541` `` `tool_calls`: An array of tool calls made in this step,
  including their arguments. ``: the element's keys are not given, and
  no real install has recorded them; the parser reads `id`, `name` and
  the arguments as `arguments_json`, `args` or `arguments`, the first
  present, an object or a JSON string (fixture only).
- `@54351847` `` `truncated_fields`: An array of field names that were
  truncated ``, followed by the examples `["content"]`, `["thinking"]`,
  `["tool_calls"]`, and the statement that the key is never in
  `transcript_full.jsonl`.
- `@53937489` `` `<appDataDir>/brain/<conversation-id>/.system_generated/logs/transcript_full.jsonl` ``:
  `<appDataDir>` is the data directory (`antigravity-cli`, `antigravity`,
  and so on), the same name the app passes the language server.

A `GENERIC` line carries no tool name or call id: its `content` is the
tool's output. The database's tool steps on every real install so far were
`generic` steps whose `metadata.tool_call` names the call
([Joins and timestamps](#joins-and-timestamps)); a planner step's calls are run as the
steps after it.

#### Brain message files

`messages/<uuid>.json` (**observed**): `content` (string), `id` (UUID),
`priority` (upper-case enum-like string), `recipient` (UUID),
`renderDetails` {`messageTitle`}, `sender` (a 43-44 character string
containing a UUID), `timestamp` (ISO date), and on two of three
`sourceMetadata` {`tool` {`conversationId` (UUID), `stepIndex` (number),
`toolCall` {`argumentsJson`, `id` (`call_` and digits), `name`
(`run_command`), `thinkingSignature`}}}. `messages/read.json` is
{`<uuid>`: bool} per message. On Linux the two messages had the same keys,
`sender` 44 characters.

### Joins and timestamps

Conversation id = `cascade_id` = file name = `conversation_summaries.
conversation_id` = the `brain/<uuid>/` and `annotations/<uuid>.pbtxt`
names. Subagent conversations are separate databases linked by
`parent_conversation_id` and by `invoke_subagent.results[].conversation_id`.
Git branch is available only when the trajectory metadata records a
workspace `branch_name`; the sample run from a non-git directory left it
empty. Up to CLI 1.2.x `history.jsonl` has no conversation id, so its rows
carry only the workspace. From CLI 1.3.x a line can carry `conversationId`
(**observed**, CLI 1.3.4 Linux, one of two lines), which is the
conversation id above; the parser puts it in the history row's
`session_id` ([issue 89](https://github.com/seanthegeek/doubleagent/issues/89)), so the prompt joins
to its conversation's rows, and a line without it keeps `session_id` empty. A macOS session with CLI
1.3.6 (**observed**, 2026-10-15) shows which lines get it: of four lines
for one conversation, the first prompt, written before the conversation
existed, had no `conversationId`, and the three prompts typed into the
running conversation all carried it. The two earlier observations fit
that pattern (one of two lines on Linux; the single first-prompt line on
Windows, CLI 1.3.5, had none), so a history row with an empty
`session_id` is a conversation's opening prompt, attributed to the
workspace only, and its conversation is the database whose first
`USER_INPUT` step has the same text and a timestamp within a second.

On Windows one folder can reach the parser under several spellings
([Where the project path is recorded](#where-the-project-path-is-recorded)), since the parser
passes a plain path through unchanged and turns a `file:` URI into a path
with `uri_to_path`, which keeps the drive letter's case and uses forward
slashes: on the Windows install (**observed**, CLI 1.3.5 and app 2.23.1,
2026-10-14) its rows carried `c:/Users/sean/demo`, `C:/users/sean/demo2`
and `C:\users\sean\demo2`. The timeline writes every Windows
`project_path` in one canonical form
([`project_path` in `analyzer/docs/output.md`](../analyzer/docs/output.md#timelinejsonl));
the parser itself does not change them.

Which surface wrote a conversation is recorded twice: the data directory
the database sits in, and `trajectory_meta.source`, an
`exa.cortex_pb.CortexTrajectorySource` (each name prefixed
`CORTEX_TRAJECTORY_SOURCE_`; identical in `agy` 1.2.16 and
`language_server` 2.23.0):

```text
0 UNSPECIFIED, 1 CASCADE_CLIENT, 2 EXPLAIN_PROBLEM, 3 REFACTOR_FUNCTION,
4 EVAL, 5 EVAL_TASK, 6 ASYNC_PRR, 7 ASYNC_CF, 8 ASYNC_SL, 9 ASYNC_PRD,
10 ASYNC_CM, 12 INTERACTIVE_CASCADE, 13 REPLAY, 15 SDK, 16 SUBAGENT,
17 CLI, 18 JETBOX, 19 AGENT_API, 20 CIDER, 21 GEMINI_APP, 22 VSCODE,
23 VISUAL_STUDIO
```

The CLI writes 17 (`CLI`) and the desktop app 1 (`CASCADE_CLIENT`)
(**observed**, macOS and Linux). The summaries table's `app_data_dir` column holds
the data-directory name the app passes the language server (`antigravity`
on the app, `antigravity-cli` on the CLI). It also tells a summary row
the CLI 1.3.5 database mirrors from the app (`antigravity`) from the CLI's
own ([Summaries database](#summaries-database)); the parser does not put
`app_data_dir` in the row's text, but the row's `source_file` names the
database it came from. The parser puts
`source=<name>` and `data_dir=<directory>` in each conversation's session
start row. On Windows (**observed**, CLI 1.3.5 and app 2.23.1) the app's four
read `source=cascade_client` and `data_dir=antigravity`, one of them with
the app project's `project_id` and three with `outside-of-project`, and the
CLI's read `source=cli`, `data_dir=antigravity-cli` and
`project_id=default-cli-project`. The conversation title is in `conversation_summaries.title` and
in `annotations/<uuid>.pbtxt` (`title:`); the parser reads the former.

The brain mirror ([Transcript stores](#transcript-stores)) joins the
database by conversation id: the `<uuid>` of `brain/<uuid>/` is the
database's `cascade_id`, and the parser puts it in `session_id` for both.
A mirror line repeats one database step, and the timeline pairs the two
rows on `session_id`, `turn_type` and `text` within 600 seconds (see
[Secondary sources in `analyzer/docs/output.md`](../analyzer/docs/output.md#secondary-sources),
the analyzer's rule for second copies). The mirror records no workspace,
so its rows have no `project_path`; they need none to pair, since the
session id scopes the match. The text joins because the parser renders a
mirror line as the database reader renders the step of the same kind. The
time is the weak part: the line has one `created_at`, while the database
reader stamps an `assistant`, `tool_use` or `tool_result` row with the
step's `completed_at`, so the mirror's row comes first by the time the
model or the tool took, which is what the 600 seconds cover. Rows that do
not pair, and so appear twice when both stores survive: a
`SYSTEM_MESSAGE` whose database step has an `event_type` (the
`task_notification` events of every real install; the database row reads
`task_notification: <message>`, and the line records no event type), a
queued prompt (the database row starts `[queued] `), a field shortened in
`transcript.jsonl` on a row with no `tool_use_id` (a shortened tool call
or result pairs on the id the parser infers for it, since analyzer
0.13.0 pairs tool rows on their ids), a step that ran longer than ten
minutes, and a `GENERIC` line whose `content` differs from the database
step's `generic.result.result` and that has no inferred id, which no
install has compared. A conversation's opening prompt is in
`history.jsonl` without `conversationId`, in the mirror and in the
database; the history line, matched by workspace, can take the database
row first, and the mirror's row, matched by session, then finds the
history row under the broader scope `dedup.deduplicate` tries among
secondary rows, so the prompt is shown once.

### Other stores

#### Summaries database

```sql
CREATE TABLE conversation_summaries (conversation_id text PRIMARY KEY, title text, preview text, step_count integer,
  last_modified_time datetime, workspace_uris text, status text, source text, project_id text, agent_name text,
  parent_conversation_id text, nesting_depth integer, battle_id text, winning_conversation_id text, not_fully_idle numeric,
  killed numeric, last_user_input_time datetime, last_user_input_step_index integer, app_data_dir text, raw_summary blob, group_id text);
```

Times are text like `2026-10-03 16:43:44.002530482+00:00`. `workspace_uris`
is a JSON array of `file://` URIs. `status` is a `CascadeRunStatus` name.
`raw_summary` is `exa.jetski_cortex_pb.CascadeTrajectorySummary` (1
`summary`, 2 `step_count`, 3 `last_modified_time`, 7 `created_time`, 9
`workspaces[]`, 10 `last_user_input_time`, 17 `trajectory_metadata`, 22
`trajectory_type`, 24 `fork_parent_conversation_id`).

The desktop app's `conversation_summaries.db` (**observed**, macOS; 131,072
B with a 32,768 B `-shm` and an empty `-wal`) has the same
`conversation_summaries` table and two more:

- `search_schema_version(version)`, one row, value 3.
- `conversations_fts`, an FTS5 virtual table (FTS5 is SQLite's full-text
  search extension; a virtual table whose rows live in the shadow tables
  `conversations_fts_data`, `_idx`, `_content`, `_docsize` and `_config`;
  the Linux listing showed `conversations_fts_config`, which the macOS
  listing did not record) with the
  columns `conversation_id`, `chunk_index`, `chunk_first_step`,
  `chunk_last_step`, `last_modified_time`, `step_count`,
  `last_step_created_at`, `archived`, `user_step_byte_offsets`,
  `agent_step_byte_offsets`, `title`, `workspace`, `project`, `user_body`
  and `agent_body`, tokenizer `unicode61 remove_diacritics 2`, prefix
  indexes `1 2 3`; 8 rows for 4 conversations. By their names,
  `user_body` and `agent_body` hold the user's and the agent's text of a
  chunk of steps (the values were not read); the parser does not read
  the table.

Its four `conversation_summaries` rows had `status`
`CASCADE_RUN_STATUS_IDLE`, `source` and `agent_name` empty,
`nesting_depth` 0, `parent_conversation_id` and `group_id` empty, `killed`
0, `project_id` and `title` set, `app_data_dir` `antigravity`,
`raw_summary` of 285-395 B and `workspace_uris` as text. The CLI's
summaries database has only `conversation_summaries`.

On Linux (**observed**, app 2.23.1) the app's database was 167,936 B with a
32,768 B `-shm` and an empty `-wal`, the same schema with
`search_schema_version` 3, and four rows like the macOS ones, `raw_summary`
278-390 B. The CLI 1.3.4 summaries database was 20,480 B with only
`conversation_summaries` and one row (`app_data_dir` `antigravity-cli`,
`raw_summary` 386 B).

On Windows (**observed**, CLI 1.3.5 and app 2.23.1) neither summaries
database had a `-wal` or `-shm` sidecar: the app's was 159,744 B, the
CLI's 40,960 B. The CLI's held five rows, its own conversation's and one
for each of the app's four, which carry `app_data_dir` `antigravity`; the
CLI's `cache/conversation_metadata.json` lists the same four
([Transcript stores](#transcript-stores)), and its `cli.log` has a line
from `reconcile_summaries.go` and one from `project_uris_migration.go`.
The parser reads every row of both databases, so each app conversation
yields two `conversation summary` rows, one from each database (9 for 5
conversations on this install). The mirrored row has no run status: the
app's own row reads
`conversation summary | <title> | CASCADE_RUN_STATUS_IDLE | 20 steps` and
the CLI's copy `conversation summary | <title> | 20 steps`. Neither macOS
(CLI 1.2.16) nor Linux (CLI 1.3.4) showed the mirroring; whether it is new
in 1.3.5 or needs the app to have run first on the same profile is not
determined.

#### Permission configuration files

Besides the per-step grants in the conversation databases
([Conversation database](#conversation-database)), Antigravity keeps standing permission
state in four JSON files, which together are its permission configuration. A
grant entry is a tool-call pattern: a tool name followed by its argument in
parentheses. The keys, with what was seen on the 2026-10-13 real installs
(macOS: CLI 1.2.16 and app 2.23.0; Ubuntu: CLI 1.3.4 and app 2.23.1) and
the Windows install of 2026-10-14 (CLI 1.3.5 and app 2.23.1), key names
and counts only, **observed** unless marked:

| File | Keys |
| --- | --- |
| `.gemini/config/config.json` (shared by the CLI and the app) | `userSettings.globalPermissionGrants.allow[]` (3 entries on macOS, 11 on Ubuntu, 14 on Windows), `userSettings.permissionGrantsV2Migrated` (bool), `userSettings.sandboxEnabledAtV2Migration` (bool); per automation (sidecar) `sidecars.<name>.enabled` and `sidecars.<name>.resolvedPermissions.accessGrants[]` (not in the macOS or the Windows file; the Windows file also had neither migration flag) |
| `.gemini/config/projects/<uuid>.json` | `permissionGrants.permissionGrants.allow[]` (2 entries, seen on one Ubuntu project file), `permissionGrants.v2Migrated`; the project folder is `projectResources.resources[].folderUri` |
| `.gemini/config/sidecars/<name>/sidecar.json` | `agent_permissions.access_grants[]` (2 entries on macOS, 3 on Windows) |
| `.gemini/antigravity-cli/settings.json` | `permissions.allow[]` (CLI 1.3.x; 2 entries on Ubuntu, 1 on Windows) and `trustedWorkspaces[]` (the folders the user trusted; a JSON tag in the binary, see [CLI data directory](#cli-data-directory)) |

#### Sidecar logs

`sidecar_data/<name>/logs/sidecar.log` (**observed**, Linux, 388 B,
written when the automation was saved): the plain-text log of the sidecar
named `<name>` (the automation's name, the same as its directory under
`.gemini/config/sidecars/`). With digits and names redacted, its lines
were:

```text
=== sidecar "<name>" started at <timestamp> ===
<date> <time> [schedule] Scheduler started for schedule: "CRON_TZ=<zone> N N * * *"
<date> <time> [schedule] Calculated jitter: NmNN.NNNNNNNNs
<date> <time> [schedule] Next execution scheduled in NNhNNmNNs (at <UTC time>), will run by ...
```

So a scheduled automation's own output lands under
`sidecar_data/<name>/logs/`. The schedule is a cron expression with a time
zone prefix (`CRON_TZ=`). No scheduled run fired on any install, so what
a run writes, and whether it becomes a conversation, is not observed. The
parser does not read the log.

On Windows (**observed**, app 2.23.1) the automation was saved
(`.gemini/config/sidecars/<name>/sidecar.json`, written 00:17:41Z to
00:20:16Z), but no `sidecar_data/` directory existed 25 minutes later and
`config.json` had no `sidecars` key, both of which the Linux install had
within a minute of saving. The language server's log,
`%APPDATA%\Antigravity\logs\language_server.log`, has these lines about it
(digits and names redacted; repeat counts in brackets):

```text
manager.go:NN] Migration [MIGRATION_ID_BUILTIN_SIDECARS_TO_PLUGINS] is enabled   (x3)
manager.go:NN] Migration [MIGRATION_ID_BUILTIN_SIDECARS_TO_PLUGINS] starting
manager.go:NN] Migration [MIGRATION_ID_BUILTIN_SIDECARS_TO_PLUGINS] completed successfully in NN.NNNNms
manager.go:NN] Migration [MIGRATION_ID_BUILTIN_SIDECARS_TO_PLUGINS] already has status MIGRATION_STATUS_...   (x2)
manager.go:NN] Migration [MIGRATION_ID_SIDECAR_USER_CONFIG_BYPASS] is disabled, skipping entirely   (x3)
sidecar_manager.go:NNN] Config changed for sidecar <name>, updating...
sidecar_sdk.go:NN] Using bundled agy-node
```

Whether the sidecar ever started on Windows, and where its log would go
there, is not determined.

### Format versions

The builds observed differ in which files and keys they write; the
protobuf schemas were compared only between the Linux (WSL) build of
[Source and evidence](#source-and-evidence) and the macOS CLI 1.2.16 and app
2.23.0, and the descriptors of CLI 1.3.4 and 1.3.5 and app 2.23.1 were not
extracted.

- `history.jsonl`: up to CLI 1.2.x a line has no conversation id; from CLI
  1.3.x a prompt typed into a running conversation carries
  `conversationId` and a conversation's opening prompt does not (CLI
  1.3.4 on Linux, 1.3.5 on Windows, 1.3.6 on macOS;
  [Joins and timestamps](#joins-and-timestamps)).
- `annotations/<uuid>.pbtxt` appears in the CLI's directory from CLI
  1.3.x, as in the app's ([Transcript stores](#transcript-stores)), and the
  CLI's `settings.json` has `permissions.allow[]` from CLI 1.3.x
  ([Permission configuration files](#permission-configuration-files)).
- CLI 1.3.5 on Windows wrote `cache/conversation_metadata.json` and a
  summary row for each of the app's conversations, which neither CLI
  1.2.16 on macOS nor 1.3.4 on Linux showed
  ([Summaries database](#summaries-database)).
- Brain mirror lines: on Linux (CLI 1.3.4 and app 2.23.1) every `GENERIC`
  line has an `error` key, which on macOS appeared only on
  `ERROR_MESSAGE` lines ([Brain mirror lines](#brain-mirror-lines)).
- Before 2.0 the VS Code-fork IDE wrote `.gemini/antigravity/`, the
  directory the desktop app 2.x now uses; the separately installed IDE's
  directory is `.gemini/antigravity-ide/` ([Transcript stores](#transcript-stores)).

The descriptor comparison: `tools/proto_descriptors.py extract` (after the
fix for [issue 86](https://github.com/seanthegeek/doubleagent/issues/86)) found 271 descriptor files
in `agy` 1.2.16 and 484 in `language_server` 2.23.0; the difference from the Linux count in
[Source and evidence](#source-and-evidence) was not investigated.
`exa.cortex_pb.CortexStepType` (118 values), `CortexStepStatus`,
`CortexTrajectorySource`, `gemini_coder.Step`, `gemini_coder.Trajectory` and
every message in the parser's `SCHEMA` were diffed against the parser's
tables, which were built from the Linux descriptors: field numbers, names
and types match in both macOS binaries, and the two define
`CortexTrajectorySource` identically ([Joins and timestamps](#joins-and-timestamps)).
Nothing the parser reads drifted. The diff did find one error in the tables
themselves, the `delete_file` layout ([Conversation database](#conversation-database)).

### Secrets

`antigravity-oauth-token` (collector secret glob).
`plugin_data/<plugin>/token.json` under the desktop app's data directory
([Credentials](#credentials)) is the Google Docs plugin's OAuth credential
(**observed**): keys `client_id`, `client_secret`, `email`, `scopes` (an
array of 8 URLs), `token` {`access_token`, `expiry` (ISO date),
`refresh_token`, `token_type`} and `updated_at`. `client_secret` and the
`token` values are the credential and `email` names the account. The
collector flags the file `secret: true`
([issue 84](https://github.com/seanthegeek/doubleagent/issues/84)); the parser never reads it.
`ChatModelMetadata.system_prompt` and `message_prompts` in `gen_metadata` hold the full prompt
including file contents; the parser does not read them. Tool outputs in `generic.result`
and `run_command`, the brain directory's `steps/<idx>/output.txt` and
`tasks/*.log` contain whatever the agent read, and so, presumably, do the
FTS5 table's `user_body` and `agent_body`.

The Windows Credential Manager entry of CLI 1.3.5 and the
`os_crypt.encrypted_key` in the desktop app's Windows `Local State` file
are described in [Credentials](#credentials); the parser reads neither.

### Parser plan

Every source below is read on every run; a secondary source is one whose
rows the timeline drops when the primary store yields the same event
([Secondary sources in `analyzer/docs/output.md`](../analyzer/docs/output.md#secondary-sources)).

| Source | Role | Rows |
| --- | --- | --- |
| `<dir>/conversations/<uuid>.db` | primary: the store of record, every step with its payload, model, permissions and both times | session start, user, assistant, tool_use, tool_result, thinking, system, model and permission rows ([Conversation database](#conversation-database)) |
| `<dir>/conversation_summaries.db` | primary: a summary per conversation, not a copy of its events | one `conversation summary` `system` row per conversation ([Summaries database](#summaries-database)) |
| `.gemini/antigravity-cli/history.jsonl` | secondary (2 seconds): the prompts the database also records | one `user` row per line ([Joins and timestamps](#joins-and-timestamps)) |
| `<dir>/brain/<uuid>/.system_generated/logs/transcript_full.jsonl`, else `chunks/transcript_full/*.jsonl` concatenated, else `transcript.jsonl`, else `chunks/transcript/*.jsonl` | secondary (600 seconds): one copy of the database's steps, read in that order of preference, one copy per conversation | user, assistant, tool_use, tool_result, thinking and system rows, as the database reader renders the same step kinds ([Transcript stores](#transcript-stores)) |
| the standing permission files | primary: no other copy | `permission setting` rows ([Permission configuration files](#permission-configuration-files)) |

The four mirror files are copies of one transcript, not four sources: the
shortened `transcript.jsonl` holds the same lines with long fields cut, and
the chunks are the same bytes split at 102,400 bytes. Not read:
`brain/<uuid>/.system_generated/steps/<idx>/output.txt` and `content.md`,
`messages/<uuid>.json` and `tasks/task-<idx>.log` (outputs and messages
the steps already carry, or not yet mapped to a step), `annotations/*.pbtxt`
(the title, also in the summaries database),
`cache/conversation_metadata.json`, `jetbox_summaries_proto.pb` and
`agyhub_summaries_proto.pb` (summaries again), and the app's FTS5 table.

#### Permission rows

The parser decodes the `steps.permissions` column
([Conversation database](#conversation-database)), or Step field 133 of the payload when
the column is empty, and yields after the step's own rows one `system` row
per `permission_grants[]` entry,
`permission grant: action=<action> target=<target> allow=<true|false> scope=<scope>`
(`action` and `target` from fields 5 and 6, else from `resource`; with
`deny_instruction=<text>` appended when `user_deny_instruction` is set), and
one per `file_access_permissions[]` entry,
`permission grant: path=<path> is_directory=<true|false> allow=<true|false> scope=<scope>`
(with `from_current_step=true` appended when set). Each row carries the
step's `tool_name` and `tool_use_id`, so it sits beside the call it
governed, and the step's tool-result time; `<scope>` is the lower-case name
without the prefix, or the number for a value not in the enum.

A step with `user_rejected` set (the three tool-payload fields of
[Conversation database](#conversation-database)) yields, right after its `tool_result`
row (whose text keeps `[rejected by user]`), a `system` row
`permission decision: user_rejected` with the step's `tool_name`,
`tool_use_id` and result time. `ran_in_sandbox` is not a decision: a
`run_command` result with it set ends with ` sandbox=true`.

The `antigravity` parser reads the keys of the
[permission configuration files](#permission-configuration-files) and
nothing else in the files (not `plugins`, `remoteControlHostname`, a
project's `name` or a sidecar's `args`), and never wants `mcp_config.json`,
which can embed MCP credentials, or a plugin's `token.json`. Each list entry
and each flag becomes one `system` row,
`permission setting: <key>=<value> (from <path relative to the home>, file mtime)`,
where `<key>` is the JSON path in the file and a `trustedWorkspaces[]` entry
is `trustedWorkspace=<path>`. The rows are timestamped by the file's
modification time (the manifest's `mtime`, or the file's own without a
manifest), since none of the entries records when it was granted; their
`session_id` is empty, and the project file's rows and each trusted
workspace's row carry the folder as `project_path`. A `deny[]` list beside
each `allow[]` is read the same way; none was seen, so it is fixture only.
On Windows (**observed**, CLI 1.3.5) the one `trustedWorkspaces[]` entry was
an absolute Windows path, not a `file://` URI; the parser accepts both
([issue 92](https://github.com/seanthegeek/doubleagent/issues/92)).

#### Model rows

The parser yields the model as a `system` row
([issue 96](https://github.com/seanthegeek/doubleagent/issues/96)): `model: <id>` before the rows of
the conversation's first step that a `gen_metadata` row lists in `step_indices`, and
`model: <old> -> <new>` before the rows of each later step whose model differs, at the step's
`metadata.created_at`, with the new id in the `model` column. The id is
`chat_model.response_model`, else `model_display_name`. No step kind in
the parser's `SCHEMA` records a model switch. The macOS and Linux rounds
did not record the `response_model` values per conversation; on Windows
(**observed**, CLI 1.3.5 and app 2.23.1) each of the five conversations
yielded one `model` row and no change, with `gemini-3.8-flash-n` in every
one. So a model change within a conversation has not been observed; it
would show only as a different `response_model` on a later planner step.

#### Past-use rows and secondary sources

The conversation database is the store of record. `history.jsonl` (2
seconds) and the brain mirror (600 seconds, one copy per conversation in
the order of preference in the table) are secondary sources; each
summaries row is a `conversation summary` row of its own, so an app
conversation that CLI 1.3.5 also summarises yields two
([Summaries database](#summaries-database)). What each copy survives is in
[What survives a delete](#what-survives-a-delete); how history and mirror
rows pair with the database's, and which rows do not pair, is in
[Joins and timestamps](#joins-and-timestamps).

#### Sync, relay and origin rows

Not stated.

#### Scratch directory rows

The parser yields no `scratch directory:` row: the app's scratch documents
sit inside its data directory ([Scratch directories](#scratch-directories)),
which no row records as a project path.

#### Deletion rows

Not stated.

#### Bad-line rows

A `steps.permissions` blob that does not decode is one `system` row,
`parser: the permissions of step <idx> could not be decoded`, and the
step's other rows stand ([issue 92](https://github.com/seanthegeek/doubleagent/issues/92)). Each
`history.jsonl` and brain mirror line that does not decode is its own `system` row carrying the
line, and the lines around it are still read; a brain chunk can cut a line at a
chunk boundary ([Transcript stores](#transcript-stores)).

A `conversation_summaries.db` or `conversations/<id>.db` that cannot be read, or whose `-wal`,
`-shm` or `-journal` sidecar cannot, is one row `parser: <file> could not be read (<reason>)`,
`<file>` the name of the file that failed; one that is not a SQLite database is
`parser: not a SQLite database`, a conversation database's row carrying the id its file name gives.
One that starts with the SQLite header but cannot be read past it (a truncated copy, a damaged page)
is one row `parser: SQLite error: <message>`, `<message>` SQLite's own, with the same id, and the
other files are still parsed.

A `history.jsonl` that cannot be read is one whole-file row
`parser: history.jsonl could not be read (<reason>)` through `error_rows`. A brain transcript copy
or one of its chunks that cannot be read is one row `parser: <file> could not be read (<reason>)`
at the file the copy is read through (its first chunk for a chunked copy), with the conversation id
its directory names; the chunks that can be read are still joined and parsed. A standing permission
file that cannot be read is one row `parser: <file> could not be read (<reason>)`, `<file>` its
name; one that reads but does not parse, or is not a JSON object, is one row
`parser: unreadable <file>`, `<file>` its home-relative path, at the file's modification time.

#### Fixtures

`analyzer/tests/fixtures/antigravity.py` builds the CLI data directory
(`build_antigravity`, with the step shapes of `antigravity_steps`), the
app's (`build_antigravity_app`, `antigravity_app_steps`, source 1 and
`app_data_dir` `antigravity`), the brain mirror (`build_antigravity_brain`:
a conversation with its database, and one whose database is gone, its line
cut across two chunks) and the permission configuration files
(`build_antigravity_config`). Fixture only, never seen on a real install:
the keys of a mirror line's `tool_calls` elements
([Brain mirror lines](#brain-mirror-lines)) and a `deny[]` permission list
([Permission rows](#permission-rows)).

## Confidence

- High: the CLI layout (live, observed and binary literals); the desktop
  app's data directory `~/.gemini/antigravity` and its contents (bundle and
  observed); the Electron user-data directory name `Antigravity` and the
  macOS logs directory (bundle and observed); the Linux Electron
  user-data directory `~/.config/Antigravity` and its `logs/` (bundle and
  observed, app 2.23.1); the Windows Electron user-data directory
  `%APPDATA%\Antigravity` and its `logs\`, and the Windows staging
  directory `%LOCALAPPDATA%\antigravity\staging` (observed, app 2.23.1 and
  CLI 1.3.5, Windows 11 ARM64 build 26300, 2026-10-14); the Credential
  Manager as the CLI's keyring on Windows (observed log lines); the roles of
  `antigravity-ide` and `antigravity-backup` (bundle); the plugin token's
  location and keys (observed); the keyring service and file fallback;
  `~/.gemini/config` contents, including `projects/<uuid>.json` and
  `sidecars/<name>/sidecar.json` (observed); the CLI's log line naming
  `~/.config/antigravity/plugin-secrets.json` on Linux (observed).
- High: the conversation database and summaries database schemas (observed
  on every install); the protobuf tables the parser reads (descriptors of
  `agy` 1.2.16 and `language_server` 2.23.0, diffed against the parser's
  tables, [Format versions](#format-versions)); the five step kinds of
  [Conversation database](#conversation-database), the brain mirror's line
  keys and the `history.jsonl` keys, including which lines carry
  `conversationId` (observed on all three platforms).
- Medium: the macOS and Windows paths of `plugin-secrets.json` (inferred
  from Go's `os.UserConfigDir()`, not observed; the Windows install used
  the Credential Manager instead) and its contents (never observed);
  `antigravity-ide/conversations` and `brain` (four community sources, no
  real install); the `Antigravity IDE` app-dir name on macOS (inferred from
  the Windows and Linux names); `knowledge/` sub-layout (one source);
  `.cache/antigravity` purpose; the brain mirror's value forms (the format
  description the backend embeds, `agy` 1.3.6, not compared value by value
  on a real install, [Brain mirror lines](#brain-mirror-lines)).
- Not determined: which of the observed app conversations belongs to which
  task, on any of the three platforms (modification times only,
  [Observed](#observed)); what a fired scheduled run writes and whether it
  becomes a conversation ([Sidecar logs](#sidecar-logs)); whether the
  scheduled automation's sidecar ever started on Windows, and where its log
  would go there; whether the Electron `Cookies` or `Local Storage/` can
  hold a credential; where the WSL language server keeps state; the
  `implicit/*.pb` and `presence/` semantics; whether `~/.antigravity` is
  still written by any build; the name of the CLI's Credential Manager
  entry on Windows; whether CLI 1.3.5 copies the app's conversation
  summaries on every platform, or only once the app has run
  ([Summaries database](#summaries-database)).
- Not determined, for the parser: what Antigravity's own delete of a
  conversation removes, and whether a compaction or a rewind rewrites the
  brain mirror ([What survives a delete](#what-survives-a-delete)); the step
  kinds other than the five seen, decoded from the descriptors only; which
  scope values and `action` strings `steps.permissions` holds; whether
  `user_rejected` or `ran_in_sandbox` is ever set on a real install;
  whether a mirror line's `created_at` is the step's `created_at` or its
  `completed_at`; the keys of a mirror line's `tool_calls` elements and a
  `deny[]` permission list (fixture only); a model change within a
  conversation (none observed, [Model rows](#model-rows)); the descriptors
  of CLI 1.3.4 and 1.3.5 and app 2.23.1, which were not extracted, and why
  the Linux binary's descriptor count differs from the macOS ones
  ([Format versions](#format-versions)).
