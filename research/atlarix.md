# atlarix

Atlarix is a closed-source coding agent from NorahLabs with a desktop app (macOS, Windows,
Linux), a CLI (`atlarix` on npm, Homebrew), an extension for VS Code, Cursor, Kiro and Windsurf
that bundles the CLI, an Agent Client Protocol agent for Zed and JetBrains IDEs (`atlarix acp`)
and a Chrome extension. The desktop app, the CLI, the editor extension and the ACP agent write
one shared tree, so they are one catalog agent. Its parser is in the backlog
([issue 599](https://github.com/seanthegeek/doubleagent/issues/599)).

## Source and evidence

Closed source. Evidence, best first:

1. The shipped package (level: shipped package). npm `atlarix` 15.19.5 (`atlarix-15.19.5.tgz`,
   8,697,179 bytes, sha256 `d6a583b8ff9ae5a459deb2bca4da6907ed0c2a4f199576b950391388fe551390`)
   is a Node wrapper, `bin/atlarix.js`, that runs the compiled CLI from the per-platform package
   its `optionalDependencies` name, and falls back to a bundled Node build
   (`dist-headless/atlarix-headless.mjs`) for `atlarix run` alone. Three platform packages, each
   15.19.5, were read:
   - `atlarix-darwin-arm64` (tarball 47,812,654 bytes, sha256
     `4dc1dda82ddf96b6da8004a821f14cedc03e0815904b4cbfd4ddd59bbfbcddcb`): `bin/atlarix`, a Mach-O
     arm64 executable, 105,452,720 bytes, sha256
     `8e2874ccef136b6a66afef6ae77eaad8b6bc076de49b24bdf6db6041cd3d2f9c`.
   - `atlarix-linux-x64` (tarball 60,664,439 bytes, sha256
     `274ab98e318f65edba0070aabc3d1f9285bc7e7dc19d6ba05cdb138f754c4423`): `bin/atlarix`, an ELF
     x86-64 executable, 129,557,984 bytes, sha256
     `51a8ccbf43e9b54002d4956d9e9e727d699301e7c6548ef9389df1fd6b6bf045`.
   - `atlarix-windows-x64` (tarball 62,609,516 bytes, sha256
     `edcba4f9e61605a03205721979d77e03bc8cfa7d79f47fabc397d88a3e62ee5c`): `bin/atlarix.exe`, a
     PE32+ executable, 131,143,168 bytes, sha256
     `11292d5251a1a86f4565bd4353e457bb39db6b663fc1ccb271ea2c06b959ec6e`.

   Each binary is a Bun build (Bun compiles a JavaScript program and its runtime into one
   executable) holding the application's JavaScript as plain text, so its path code reads as
   source. The three builds carry the same path code and differ in the platform name Bun wrote
   in as a default. The desktop app itself (an Electron app) is not in the package: the CLI
   shares its code and its data directory, and what only the app writes rests on the vendor
   statement.
2. The vendor's public release repository, AmariahAK/atlarix-releases, at
   [`01a1f03`](https://github.com/AmariahAK/atlarix-releases/commit/01a1f03411d29365420f1f4f77fddfb8dd91f8f4)
   (level: vendor documentation).
3. The vendor's documentation site (level: vendor documentation), read through its index
   [llms.txt](https://www.atlarix.dev/llms.txt) and its full text
   [llms-full.txt](https://www.atlarix.dev/llms-full.txt) (both fetched 2026-11-23, index dated
   2026-11-21). It names few paths; each page is linked at the claim it supports.
4. Vendor statement by Amariah Abishai (NorahLabs), 2026-10-27, for 15.11.1:
   [issue 60, comment 6430492071](https://github.com/seanthegeek/doubleagent/issues/60#issuecomment-6430492071).
5. Vendor statement by Amariah Abishai (NorahLabs), merged 2026-10-27: the vendor's pull request
   [pull request 271](https://github.com/seanthegeek/doubleagent/pull/271), which wrote the first
   version of this page from the statement above and adds claims the statement does not make;
   the page names it as "the vendor's pull request" where a claim rests on it alone.
6. The vendor's check of the macOS directory names against an installed copy, 2026-10-27.

### Observed

No real install.

### Source reviews

- 2026-11-23, npm `atlarix` 15.19.5 (`darwin-arm64/atlarix`, `linux-x64/atlarix`,
  `windows-x64/atlarix.exe`): every path checked against the binaries; the data directory, its
  files and `~/.atlarix` confirmed, more file names, the developer-mode data folder, the copies
  written while saving, the project-local files and six relocation variables added; each claim
  attributed to the vendor statement or the vendor's pull request; `~/.atlarix/tool-outputs/`
  dropped, as the package does not name it.

## Collection

### Per-user storage

Citations in this part are to npm `atlarix` 15.19.5: `atlarix @<offset>` is a byte offset into
`darwin-arm64/atlarix`, and `linux-x64/atlarix` and `windows-x64/atlarix.exe` are named where
the platform build matters.

#### Data directory

Shared by the desktop app, the CLI, the editor extension and the ACP agent (vendor statement;
the documentation says the CLI shares the app's sessions, models, keys and MCP servers, and that
chats from the editor show in the app and the CLI:
[Atlarix in your editor](https://www.atlarix.dev/docs/guides/editors)).
The CLI resolves it on each platform as the app's Electron user-data folder:

- macOS: `~/Library/Application Support/Atlarix/`
  (`atlarix @71928253`: `path3.join(home,"Library","Application Support","Atlarix")`)
- Windows: `%APPDATA%\Atlarix\`, `AppData/Roaming/Atlarix` when `APPDATA` is unset
  (`windows-x64/atlarix.exe @96305532`:
  `path3.join(env2.APPDATA??path3.join(home,"AppData","Roaming"),"Atlarix")`); the vendor
  statement gives the same folder for the Microsoft Store build
- Linux: `$XDG_CONFIG_HOME/Atlarix/`, default `~/.config/Atlarix/`
  (`linux-x64/atlarix @91636042`:
  `path3.join(env2.XDG_CONFIG_HOME??path3.join(home,".config"),"Atlarix")`)

Each build names its own platform as the default (`atlarix @71928149`,
`linux-x64/atlarix @91635770`, `windows-x64/atlarix.exe @96305340`:
`desktopUserDataDir(platform="darwin"`, `"linux"`, `"win32"`). The interactive CLI uses that
folder unless `ATLARIX_USER_DATA_DIR` names another (`atlarix @71928525`:
`const dir=process.env.ATLARIX_USER_DATA_DIR??desktopUserDataDir();process.env.ATLARIX_USER_DATA_DIR=dir`).
Unattended runs (`atlarix run`) refuse the shared folder and, without `ATLARIX_USER_DATA_DIR`,
make a new one named `atlarix-headless-ud-` plus random characters in the system temporary
directory (`atlarix @83800641`:
`process.env.ATLARIX_USER_DATA_DIR=fs101.mkdtempSync(path119.join(os40.tmpdir(),"atlarix-headless-ud-"))`).

What it holds:

- `sqlite.db` (`atlarix @74331887`: `path17.join(getUserDataPath(),"sqlite.db")`), with
  `sqlite.db-wal` and `sqlite.db-shm` (`atlarix @74330977`:
  `for(const extra of["","-wal","-shm"])`): the sessions (vendor statement) and their messages
  (the vendor's pull request).
- `backups/before-update/sqlite-<timestamp>.db`: copies of `sqlite.db` taken before a database
  update (`atlarix @74331250`: `path17.join(getUserDataPath(),"backups","before-update")`;
  `atlarix @74331348`: ``const dest=path17.join(dir,`sqlite-${new Date()``).
- `user-settings.json` and `app-state.json` (`atlarix @72588695`:
  `SETTINGS_FILE="user-settings.json",APP_STATE_FILE="app-state.json"`):
  settings ([Credentials](#credentials)) and app state.
- `supabase-auth.enc` and `supabase-auth-cli.enc`: the sign-in sessions of the app and of the
  CLI, and from older builds a plaintext `.json` of the same stem
  ([Credentials](#credentials)). While a CLI process holds the CLI's session it keeps
  `supabase-auth-cli.lock` beside it, with `supabase-auth-cli.lock.takeover` while one process
  takes over another's stale lock (`atlarix @72379735`:
  `withFileLock(fallbackEncryptedPath().replace(/\.enc$/,"")+".lock"`; `atlarix @72375088`:
  ``const guard=`${lockPath}.takeover` ``); the lock files hold no credential.
- Copies left by an interrupted save: `user-settings.json`, `app-state.json` and the other JSON
  files the CLI saves are written to `<name>.<process id>.tmp` and renamed over the file
  (`atlarix @72578772`: ``const tmp=`${filePath}.${process.pid}.tmp` ``), and the session files
  to `<name>.<process id>.<milliseconds>.tmp` (`atlarix @72374459`:
  ``const tmp=`${file}.${process.pid}.${Date.now()}.tmp` ``); a crash between the two steps
  leaves the copy.
- `shadow-snapshots/`, one subdirectory per project: copies of project files taken before the
  agent edits them, for undo (the vendor's pull request), kept 14 days (`atlarix @74307593`:
  `SHADOW_ROOT_DIR="shadow-snapshots"`; `atlarix @74307678`: `SNAPSHOT_RETENTION_DAYS=14`).
- `logs/`: the app and CLI logs (vendor statement); the CLI's log is `logs/atlarix-cli.log`
  (`atlarix @71925687`:
  `path2.join(process.env.ATLARIX_USER_DATA_DIR??path2.join(os2.homedir(),".atlarix"),"logs","atlarix-cli.log")`).
- `client-seen.json` (`atlarix @81273572`: `path.join(getUserDataPath(),"client-seen.json")`).
- `quit-ended-jobs/` (`atlarix @83185949`: `path104.join(getUserDataPath(),"quit-ended-jobs")`).
- `cache/`: downloaded model and MCP registry lists (`atlarix @74235221`:
  `path11.join(getUserDataPath(),"cache","models.json")`).
- `core-session.key` ([Credentials](#credentials)), `terminal-scopes.json` (`atlarix @79074456`:
  `path80.join(getUserDataPath(),"terminal-scopes.json")`) and `usage-pending/`
  (`atlarix @80675409`: `path90.join(getUserDataPath(),"usage-pending")`).
- `atlarix-auto-billing.json`, written only by the desktop app (`atlarix @75628692`:
  `process.versions.electron)return null;try{return path24`).
- `.atlarix/ATLARIX.md`: the user's global instructions (`atlarix @76644754`:
  `path40.join(getUserDataPath(),ATLARIX_DIR2,ATLARIX_MD)`; `atlarix @76646182`:
  `ATLARIX_DIR2=".atlarix",ATLARIX_MD="ATLARIX.md"`).
- On Windows, `Local State`, which the CLI reads for the key that decrypts the stored secrets,
  and creates with a new key when it is missing ([Credentials](#credentials)).
- The desktop app's Chromium profile files and caches: `Cache`, `Code Cache`, `GPUCache`,
  `DawnGraphiteCache`, `DawnWebGPUCache`, `blob_storage`, `Crashpad` and the rest of an
  Electron user-data folder (the vendor's pull request; the desktop app is not in the package).

#### Home directory `~/.atlarix`

On every OS (`atlarix @71929940`:
`return path5.join(userData,"atlarix-home")}return path5.join(home,".atlarix")`). It moves with
`ATLARIX_HOME_DIR`, or to `atlarix-home` inside the folder `ATLARIX_USER_DATA_DIR` names when
that is not the data directory ([Relocation variables](#relocation-variables)). It holds:

- `projects/<slug>/memory/`: project memory notes, Markdown files with a `MEMORY.md` index
  (`atlarix @76661632`:
  `path42.join(globalAtlarixRoot(),"projects",workspaceSlug(workspacePath),"memory")`;
  `atlarix @76666537`: `MEMORY_INDEX_FILE="MEMORY.md"`), and `projects/<slug>/.workspace`
  ([Where the project path is recorded](#where-the-project-path-is-recorded)). The slug is the
  project's absolute path with separators turned to `-`, cut to its last 60 characters, then
  `-` and a hash (`atlarix @76661149`: `function workspaceSlug(workspacePath)`).
- `logs/cli.log` (`atlarix @83564966`: `path113.join(atlarixHomeDir(),"logs")`;
  `atlarix @83565085`: `path113.join(dir,"cli.log")`).
- The CLI's small state files: `cli-state.json` (`atlarix @81157151`:
  `process.env.ATLARIX_CLI_STATE_FILE||path97.join(atlarixHomeDir(),"cli-state.json")`) and
  `update-check.json` (`atlarix @83569991`:
  `stampFile:path114.join(atlarixHomeDir(),"update-check.json")`).
- `permissions.json`: the user's global permission rules (`atlarix @76459172`:
  `path28.join(dir,"permissions.json")`).
- `edicts.json`, the user's own hooks ("edicts" in Atlarix's terms), and the plugins ("silks"):
  `silks/` with `silks/.staging/`, `silks.json` and `dev-silks/` (`atlarix @76436408`:
  `path25.join(atlarixHomeDir(),"silks")`; `atlarix @76436548`:
  `path25.join(pluginsRootDir(),".staging")`; `atlarix @76436625`:
  `path25.join(atlarixHomeDir(),"silks.json")`; `atlarix @76436700`:
  `path25.join(atlarixHomeDir(),"edicts.json")`; `atlarix @76436778`:
  `path25.join(atlarixHomeDir(),"dev-silks")`), with `mod-runner/` and `mod-store/<plugin>.json`
  for plugin code (`atlarix @78767393`: `path61.join(atlarixHomeDir(),"mod-runner"`;
  `atlarix @78778832`: ``path61.join(atlarixHomeDir(),"mod-store",`${plugin}.json`)``).
- `attachments/` (`atlarix @79596289`: `path85.join(atlarixHomeDir(),"attachments")`).
- `chrome/`: the Chrome extension's bridge ([Chrome extension](#chrome-extension)).
- `bin/`: helper binaries the CLI unpacks from itself (`atlarix @75605583`:
  `path20.join(atlarixHomeDir(),"bin")`), and on macOS and Linux the CLI the install script puts
  there (`atlarix @81170952`: `path98.join(home,".atlarix","bin","atlarix")`).
- `skills/`: a cache of the skills that ship with the CLI (`atlarix @71930422`:
  `path6.join(atlarixHome,"skills")`).
- `blueprints/`, named only in a database migration comment, unexplained
  (`atlarix @71961543`: `blueprint state lives in ~/.atlarix/blueprints/`).
- `dev-data/`: the data folder of the CLI's developer mode, which the interactive CLI and
  `atlarix acp` enter when started with `--provider-url` and without `ATLARIX_USER_DATA_DIR`
  (`atlarix @83827340` and `atlarix @83836035`:
  `const devUrl=flag2("--provider-url");await prepareCliData(devUrl)`; `atlarix @83830971`:
  `process.env.ATLARIX_USER_DATA_DIR=path123.join(os41.homedir(),".atlarix","dev-data")`). The
  CLI writes every file of the [data directory](#data-directory) to the folder that variable
  names (`atlarix @72388703`:
  `if(process.env.ATLARIX_USER_DATA_DIR){return process.env.ATLARIX_USER_DATA_DIR}`), so
  `sqlite.db`, `user-settings.json`, the session files, `core-session.key`, `cache/` and
  `shadow-snapshots/` can be here, with the copies of an interrupted save. The developer branch
  does not open the Windows key store, so no source shows a `Local State` here and no secret
  glob names one. That branch does not set `ATLARIX_AUTH_SESSION_STEM`, which only
  `enterDesktopDataMode` sets (`atlarix @71928748`:
  `process.env.ATLARIX_AUTH_SESSION_STEM??="supabase-auth-cli"`; `atlarix @83830883`:
  `async function prepareCliData(devUrl){if(devUrl){`, whose developer branch does not call it),
  so the session file here has the default stem (`atlarix @72379122`:
  `return process.env.ATLARIX_AUTH_SESSION_STEM||"supabase-auth"`): `supabase-auth.enc`, and
  `supabase-auth.json` in older builds, not `supabase-auth-cli.*`. The secret globs for the folder
  are `.atlarix/dev-data/supabase-auth*.enc*`,
  `.atlarix/dev-data/supabase-auth*.json*`, `.atlarix/dev-data/user-settings.json*` and
  `.atlarix/dev-data/core-session.key` ([Credentials](#credentials)).

The vendor's pull request also names `tool-outputs/` here, for large tool results; the 15.19.5
package has no such string.

The CLI also reads skills from `~/.agents/skills/`, a folder other agents share
(`atlarix @76438791`: `path25.join(path25.dirname(atlarixHomeDir()),".agents")`;
`atlarix @76437934`: `path25.join(userAgentSkillsParentDir(),"skills")`).

#### Windows install folder

The Windows install script's folder, `%LOCALAPPDATA%\Atlarix\bin` (`AppData/Local/Atlarix/bin`),
holds the CLI binary (`atlarix @83568201`:
`env2.ATLARIX_INSTALL_DIR,path114.join(home,".atlarix","bin"),env2.LOCALAPPDATA?path114.join(env2.LOCALAPPDATA,"Atlarix","bin")`).

#### Chrome extension

Keeps its own state in the browser's extension storage, not in the data directory (vendor
statement): its sign-in, conversations and notes (the vendor's pull request). Its messages and
replies also sync to the user's Atlarix account
([Chrome extension](https://www.atlarix.dev/extension)). The CLI
connects to it through a native messaging host (a program a browser extension may start): a
wrapper `atlarix-native-host` (`.cmd` on Windows) and
`host.env` in `~/.atlarix/chrome/` (`atlarix @79218987`: `path83.join(home,".atlarix","chrome")`;
`atlarix @79219140`: `"atlarix-native-host.cmd":"atlarix-native-host"`; `atlarix @79219238`:
`path83.join(bridgeHome(home),"host.env")`), and a manifest `dev.atlarix.bridge.json` in each
Chromium-family browser's `NativeMessagingHosts` folder, such as
`~/.config/google-chrome/NativeMessagingHosts/` on Linux, or in `~/.atlarix/chrome/` on Windows
(`atlarix @79223712`: ``path83.join(target,`${BRIDGE_HOST_NAME}.json`)``; `atlarix @79224402`:
`BRIDGE_HOST_NAME="dev.atlarix.bridge"`; `atlarix @79219876`:
`path83.join(config,"google-chrome",suffix)`). On Windows the CLI also copies the relay program
the wrapper starts into `~/.atlarix/chrome/` as `atlarix-bridge-<12 hex digits>.exe`, the first
12 hex digits of the program's SHA-256, by way of a `.<process id>.tmp` copy, and deletes older
copies (`windows-x64/atlarix.exe @103600714`:
``const name=contentTag?`atlarix-bridge-${contentTag}.exe`:"atlarix-bridge.exe"``;
`windows-x64/atlarix.exe @103600926`:
`createHash6("sha256").update(bytes).digest("hex").slice(0,12)`;
`windows-x64/atlarix.exe @103603346`:
`if(platform==="win32"){try{relay=await placeWindowsRelay(opts.relayPath,home)}`).

#### Editor extension

Nothing of its own beyond two small flags in the editor's `globalState`: it runs its bundled
CLI, which uses the [data directory](#data-directory) (vendor statement). For Zed and JetBrains
IDEs the ACP agent is registered in the editor's own settings, `~/.config/zed/settings.json` or
`~/.jetbrains/acp.json` ([Atlarix in your editor](https://www.atlarix.dev/docs/guides/editors));
the Zed file is collected as `zed`, and no catalog line reaches `~/.jetbrains/acp.json`.

#### Scratch directories

`atlarix-scratch/<slug>/` in the system temporary directory, one per project, which shell
commands the agent runs get as their temporary directory (`atlarix @76666998`:
`path43.join(os14.tmpdir(),AGENT_SCRATCH_ROOT_NAME,workspaceSlug(workspacePath))`;
`atlarix @76667191`: ``AGENT_SCRATCH_ROOT_NAME=`${ATLARIX_TEMP_PREFIX}scratch` ``;
`atlarix @76667082`: `ATLARIX_TEMP_PREFIX="atlarix-"`), and the `atlarix-headless-ud-*` folders
of unattended runs ([Data directory](#data-directory)).

#### Relocation variables

| Variable | Moves | Replaces | Table agent |
| --- | --- | --- | --- |
| `ATLARIX_USER_DATA_DIR` | the data directory, for the CLI and for `atlarix run` (`atlarix @71928525`, `atlarix @83800641`); when it names another folder, also the home directory, to `atlarix-home` inside it (`atlarix @71929940`) | `.config/Atlarix`, `Library/Application Support/Atlarix`, `AppData/Roaming/Atlarix` | `atlarix` |
| `ATLARIX_HOME_DIR` | the home directory `~/.atlarix` (`atlarix @71929744`: `if(env2.ATLARIX_HOME_DIR)return env2.ATLARIX_HOME_DIR`) | `.atlarix` | `atlarix` |
| `ATLARIX_CLI_STATE_FILE` | the CLI's state file (`atlarix @81157151`) | `.atlarix/cli-state.json` | `atlarix` |
| `ATLARIX_SKILLS_DIR` | the cache of bundled skills (`atlarix @71931617`: `process.env.ATLARIX_SKILLS_DIR)return process.env.ATLARIX_SKILLS_DIR`) | `.atlarix/skills` | `atlarix` |
| `ATLARIX_HEADLESS_OUTPUT_FILE` | the file `atlarix run` also writes its final reply to, the environment form of `--output-file` ([Headless CLI](https://www.atlarix.dev/docs/guides/headless-cli): every flag has an `ATLARIX_HEADLESS_*` equivalent), with no default (`atlarix @83818398`: `const outputFile=arg("output-file")??process.env.ATLARIX_HEADLESS_OUTPUT_FILE`) | | `atlarix` |
| `ATLARIX_HEADLESS_TRAJECTORY_FILE` | the file `atlarix run` writes its full trace to, the environment form of `--trajectory-file`, with no default (`atlarix @83818476`: `const trajectoryFile=arg("trajectory-file")??process.env.ATLARIX_HEADLESS_TRAJECTORY_FILE`) | | `atlarix` |
| `XDG_CONFIG_HOME` | the parent of the Linux data directory `Atlarix/` (`linux-x64/atlarix @91636042`) | `.config` | `shared` |

The Windows data directory follows `APPDATA` (`windows-x64/atlarix.exe @96305532`). The variable
`ATLARIX_AUTH_SESSION_STEM` renames the sign-in session file inside the data directory rather
than moving it (`atlarix @72379129`: `process.env.ATLARIX_AUTH_SESSION_STEM||"supabase-auth"`).

### Credentials

The data directory is `Library/Application Support/Atlarix` on macOS, `AppData/Roaming/Atlarix`
on Windows and `.config/Atlarix` on Linux, below the home, and in developer mode
`.atlarix/dev-data` on every platform ([Home directory `~/.atlarix`](#home-directory-atlarix)).
The files below are in it, `Local State` on Windows only; the secret globs flag each in all four
folders, `Local State` in `AppData/Roaming/Atlarix` alone. Each glob except
the one for `core-session.key` ends in `*`, so it also flags the copy an interrupted save leaves
(`<name>.<process id>.tmp`, or `<name>.<process id>.<milliseconds>.tmp` for the session files;
[Data directory](#data-directory)). The session globs are `supabase-auth*.enc*` and
`supabase-auth*.json*`, not `supabase-auth*`, which would also flag the lock files beside the
session, which hold no credential.

- `supabase-auth.enc` and `supabase-auth-cli.enc`: the sign-in sessions, the app's under the
  default stem and the CLI's under its own (`atlarix @72379549`:
  ``path8.join(app.getPath("userData"),`${sessionFileStem()}.enc`)``;
  `atlarix @71928748`: `process.env.ATLARIX_AUTH_SESSION_STEM??="supabase-auth-cli"`;
  `atlarix @83810896`: `path120.join(dir,"supabase-auth-cli.enc")`). Older builds kept the
  session in plaintext as `supabase-auth.json` (or `supabase-auth-cli.json`) in the same folder,
  which current builds re-encrypt and remove (`atlarix @72379431`:
  ``path8.join(app.getPath("userData"),`${sessionFileStem()}.json`)``; `atlarix @72381361`:
  `Migrated legacy supabase-auth.json and removed file`).
- `user-settings.json`: the user's own provider API keys, encrypted with the OS key store
  (vendor statement; the documentation says keys stay on the computer, shared by the app, the
  CLI and the editors, and that the OS keychain keeps the saved keys and the sign-in:
  [Atlarix in your editor](https://www.atlarix.dev/docs/guides/editors),
  [Diagnostics](https://www.atlarix.dev/docs/help/diagnostics)); without a key store they are stored unencrypted in the same file
  (`atlarix @72587505`:
  `Provider keys are stored unencrypted in ${getSettingsFilePath()} (no OS keyring found)`).
- `core-session.key`: a random per-install key from which the session identifiers sent to
  Atlarix's service are derived (`atlarix @74375679`:
  `path18.join(getUserDataPath(),"core-session.key")`). The CLI creates it in one write that
  fails if the file exists, with no temporary copy (`atlarix @74375953`:
  `fs13.writeFileSync(file,made,{flag:"wx",mode:384})`), so its glob names the file alone.
- On Windows, `AppData/Roaming/Atlarix/Local State`: holds the key, protected by the user's
  Windows login, that decrypts the files above (`atlarix @83575394`:
  `path115.join(userDataDir,"Local State")`). The CLI creates the file with a new key when it is
  missing or holds none, through a `.<process id>.tmp` copy (`atlarix @83576163`:
  `if(!opts.create)return null;const key=crypto8.randomBytes(32)`; `atlarix @83576568`:
  ``const tmp=`${file}.${process.pid}.tmp` ``). On macOS that key is the login keychain item
  `Atlarix Safe Storage` (`atlarix @83574050`: ``const service=`${appName} Safe Storage` ``), on
  Linux an entry the CLI reads with `secret-tool` (`atlarix @83576833`:
  `secretToolPassword(appName)`).
- `sqlite.db` and its copies under `backups/before-update/`: the CLI's lookup of stored
  provider keys reads the database as well as `user-settings.json` (`atlarix @83812039`:
  `path121.join(dir,"sqlite.db")`), and the vendor's pull request says it holds MCP server
  credentials, encrypted the same way. The database also holds the sessions, so no secret glob
  covers it; reading the stored keys is left to the parser
  ([issue 599](https://github.com/seanthegeek/doubleagent/issues/599)).

### Exclusions

- `.atlarix/bin`: helper binaries and the CLI binary. The glob also prunes a `.atlarix/bin`
  inside a discovered project; no source names one there. The same holds for every pattern
  that starts `.atlarix/` (`chrome/atlarix-bridge*`, `dev-data/cache`, `dev-data/shadow-snapshots`
  and the `dev-data` secret globs): no source names those folders inside a project.
- `.atlarix/chrome/atlarix-bridge*`: the relay program the Windows CLI copies for the Chrome
  extension, and its temporary copy ([Chrome extension](#chrome-extension)).
- `Library/Application Support/Atlarix/shadow-snapshots`,
  `AppData/Roaming/Atlarix/shadow-snapshots`, `.config/Atlarix/shadow-snapshots` and
  `.atlarix/dev-data/shadow-snapshots`: copies of project files.
- `Library/Application Support/Atlarix/cache`, `AppData/Roaming/Atlarix/cache`,
  `.config/Atlarix/cache` and `.atlarix/dev-data/cache`: downloaded model and registry lists.
- The desktop app's Chromium caches in the three platform data directories, by the names the
  vendor's pull request gives: `Code Cache`, `Crashpad`, `DawnGraphiteCache`, `DawnWebGPUCache`,
  `GPUCache` and `blob_storage` under `Library/Application Support/Atlarix`,
  `AppData/Roaming/Atlarix` and `.config/Atlarix`, and `.config/Atlarix/Cache`. `Cache` and
  `cache` are two directories only where letter case counts, as on Linux; the folders on
  macOS and Windows ignore case, so there the `cache` line prunes `Cache` too, and only
  `.config/Atlarix` has a line for each. On a macOS volume formatted to tell case apart,
  `Library/Application Support/Atlarix/Cache` is collected.

`~/.atlarix/skills`, the cache of bundled skills, is collected: an exclusion applies inside each
discovered project as well as each home, so `.atlarix/skills` would also skip a repository's own
`.atlarix/skills/` ([Project-local files](#project-local-files)), which the user writes. The
Windows install folder `AppData/Local/Atlarix/bin` is not collected, as no catalog line reaches
it.

### Project-local files

In the repository:

- `.atlarix/rules/` (`atlarix @76643205`: `path39.join(workspacePath,".atlarix","rules")`).
- `.atlarix/skills/`
  ([`CONTRIBUTING.md:19`](https://github.com/AmariahAK/atlarix-releases/blob/01a1f03411d29365420f1f4f77fddfb8dd91f8f4/CONTRIBUTING.md#L19);
  `atlarix @71980185`: `.atlarix/skills/*.md`).
- `.atlarix/agents/`, the user's own sub-agents, committed with the repository
  ([Sub-agents and delegation](https://www.atlarix.dev/docs/guides/sub-agents); `atlarix @78825713`:
  `path65.join(workspacePath,".atlarix","agents")`).
- `.atlarix/cloud-setup.sh`, which the cloud agent runs before it starts, and
  `.github/workflows/atlarix.yml`, the workflow its GitHub App adds
  ([Cloud agent](https://www.atlarix.dev/docs/guides/cloud-agent)); neither name is in the
  package, as both serve the cloud agent, which runs in the repository's GitHub Actions. The
  workflow file is not collected, as no `PROJECT_CATALOG` line names it.
- `.atlarix/atlarix.md` and `.atlarix/memory.md` (`atlarix @76644371`:
  `ATLARIX_DIR=".atlarix",CANONICAL_MD="atlarix.md",MEMORY_MD="memory.md"`); the CLI also reads
  `AGENTS.md` and `CLAUDE.md` (`atlarix @76644226`:
  `safeRead(path39.join(workspacePath,"CLAUDE.md"))`).
- `.atlarix/ATLARIX_PLAN.md` and `.atlarix/plans/archive/` (`atlarix @80802535`:
  `PLAN_FILE=".atlarix/ATLARIX_PLAN.md"`; `atlarix @74280686`:
  `path14.join(".atlarix","plans","archive")`).
- `.atlarix/permissions.json` (`atlarix @76458978`:
  `path28.join(workspacePath,".atlarix","permissions.json")`).
- `.atlarix/images/` (`atlarix @80609830`: `path89.resolve(workspace,".atlarix","images")`).
- `relics/`, the documents the agent makes to share (plain files in the project's `relics/`
  folder: [llms.txt](https://www.atlarix.dev/llms.txt)), with `.atlarix/relics.json` and, from older
  builds, `.atlarix/relics/` (`atlarix @79154104`: `RELICS_DIR="relics"`; `atlarix @79154575`:
  `OLD_RELICS_DIR=path81.join(".atlarix","relics");RECORD_FILE=path81.join(".atlarix","relics.json")`).
  `relics/` at the repository root is not collected: it has no `PROJECT_CATALOG` line, as a
  directory of that name in a repository is not Atlarix's alone. The two under `.atlarix/` are.
- `.atlarixignore`, at the root or in `.atlarix/` (`atlarix @74281442`:
  `if(norm===".atlarixignore")return true;if(norm===".atlarix/.atlarixignore")return true`).

The vendor statement places per-project hooks under `.atlarix/`; the package names no hooks file
there. The hooks files it reads are `~/.atlarix/edicts.json` and a plugin's `edicts.json` or
`hooks/hooks.json` (`atlarix @76466912`:
`path30.join(root,"edicts.json"),path30.join(root,"hooks","hooks.json")`).

### Where the project path is recorded

- `sqlite.db`, table `apps` (`atlarix @73468349`: `apps=sqliteTable("apps"`).
- `~/.atlarix/projects/<slug>/.workspace`, which the CLI writes with the project's absolute path
  (`atlarix @76666591`: `WORKSPACE_STAMP_FILE=".workspace"`; `atlarix @76664045`:
  `path42.join(path42.dirname(target),WORKSPACE_STAMP_FILE)`).

## Parsing

Atlarix is `backlog`: its sessions and messages are in `sqlite.db`
([Per-user storage](#per-user-storage)), and the parser is
[issue 599](https://github.com/seanthegeek/doubleagent/issues/599).

## Confidence

- High (shipped package): the data directory on each OS for the CLI, the file and directory
  names cited in it and in `~/.atlarix`, the project-local names and the relocation variables.
- Vendor statement only: that the desktop app, the editor extension and the ACP agent use the
  same data directory, the Microsoft Store build's folder, `logs/` as the app's log folder, and
  per-project hooks under `.atlarix/`.
- The vendor's pull request only: the messages in `sqlite.db`, the MCP server credentials in
  it, `shadow-snapshots/` as the undo store, the Chrome extension's sign-in, conversations and
  notes, and the Chromium cache names of the desktop app's data directory.
- Not determined: the desktop app's Electron user-data files beyond the cache names, as the
  package does not contain the app; whether the CLI writes `Local State` to
  `~/.atlarix/dev-data/`; what `~/.atlarix/blueprints/` holds; where the per-folder MCP servers of
  [MCP servers](https://www.atlarix.dev/docs/guides/mcp-servers) and the activity counts of
  [Your activity](https://www.atlarix.dev/docs/guides/your-activity) are kept on disk, which
  those pages do not name.
- Not verified on a real install: every path, and what each surface writes step by step.
