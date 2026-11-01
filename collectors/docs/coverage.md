# What gets collected

The collectors decide what to take from a table, not from code. The
catalog is the table of agents and the paths under a home directory where
each keeps its state; it is the same table in both collectors and in the
analyzer, and `--list` prints it in full, so a responder can see before a
run which files will be taken, which subtrees are excluded and which files
are flagged as credentials. Every catalog entry is tried
against every home directory, so macOS, Linux and Windows paths coexist and a Windows image
mounted on a Linux workstation is collected correctly. The evidence for every
catalog entry, with citations and the level each claim rests on, is one
document per agent, listed in the [research index](../../research/README.md).

## Editors, extensions and shared directories

Editors are collected through their `User` directories; extension state
inside them goes to the extension's agent.

- **VS Code, VSCodium and their forks:** the `User` directories, which hold
  Copilot Chat sessions and extension state.
- **Cline, Roo Code, Kilo Code, Continue, Sourcegraph Cody and Twinny:**
  their globalStorage is claimed by nested catalog entries, so those files
  are attributed to the extension's agent rather than to the editor,
  whichever editor they sit in. Twinny's per-workspace extension storage,
  `workspaceStorage/<id>/rjmacarthy.twinny` (the `agent-changes/` record of
  each agent-mode reply that changed files), is claimed the same way; the
  `workspace.json` and `state.vscdb` beside it stay the editor's.
- **Sourcegraph Cody's JetBrains plugin** keeps its application settings
  in each JetBrains IDE's `options/` directory: `cody.xml`,
  `cody_application_settings.xml`, `cody_accounts_settings.xml` and
  `cody_accounts.xml` are collected as `cody`; `cody_accounts.xml` is
  flagged secret because a server entry's `customRequestHeaders` can
  carry proxy credentials, and `cody.xml` because its `cody.mcpServers`
  entries carry MCP server `headers` and `env`. Its `cody_settings.json`
  under `~/.sourcegraph` and under a discovered project's
  `.idea/.sourcegraph`, flagged secret for the same reason, and the
  plugin's pre-agent chat history in the project's
  `.idea/cody_history.xml`, are collected as `cody` too.
- **Kilo Code's JetBrains plugin** keeps a `kilo/` folder in each
  JetBrains IDE's system directory (`Library/Caches/JetBrains/<IDE>`,
  `AppData/Local/JetBrains/<IDE>` or `.cache/JetBrains/<IDE>`): from
  collector 1.21.0 it is collected as `kilo-code`, its `machine-id` among
  it, and the plugin's downloaded CLI binaries in `kilo/cli` and
  `kilo/repo-cli` are excluded. Its settings, `kiloLogSettings.xml` and
  `kiloAutocompleteSettings.xml` in the IDE's `options/` directory, are
  collected as `kilo-code` too
  ([JetBrains plugin in `research/kilo-code.md`](../../research/kilo-code.md#jetbrains-plugin)).
- **GitHub Copilot Desktop** shares `~/.copilot` with Copilot CLI: its
  sessions, written there by a bundled Copilot CLI, are collected as
  `copilot-cli`, and nested catalog entries attribute the app's own store,
  chat, repository and run files and its log to `copilot-desktop`. A chat
  directory is collected less any `node_modules` under it
  (`.copilot/chats/*/node_modules`), the dependency tree of a project the
  agent built there. On Windows the app's web view is WebView2 (Microsoft
  Edge's embeddable Chromium), whose user data folder
  `AppData/Local/com.github.githubapp` holds the app's Local Storage (the
  composer draft and tab lists) and is collected as `copilot-desktop` from
  collector 1.13.0, less its Chromium caches and downloaded components
  (`EBWebView/Default/Cache`, `Code Cache`, `GPUCache`, `blob_storage`,
  `component_crx_cache`, `WidevineCdm`, `Subresource Filter` and the
  others listed in `EXCLUDES`). The Copilot CLI's plugin marketplace clone,
  `marketplaces/` in its cache directory (`.cache/copilot`,
  `Library/Caches/copilot`, `AppData/Local/copilot`), is excluded from
  1.13.0; 263 MB on the Windows install observed in October 2026.
- **Claude Code's VS Code extension** keeps no `globalStorage` directory:
  its sessions are Claude Code sessions in `~/.claude`, collected as
  `claude-code`. Its own log,
  `Code/logs/<start>/window1/exthost/Anthropic.claude-code/Claude VSCode.log`,
  is collected as `vscode` by the editor's `logs` entry, and the extension
  directory `.vscode/extensions/anthropic.claude-code-<version>-<platform>`
  (282 MB on Windows, 297 MB on Linux, 273 MB on macOS, each holding a full
  copy of the CLI binary in `resources/native-binary/`, which is the binary
  the extension runs) is not collected, since the `vscode` entry takes only
  `extensions.json` from the extensions directory (observed with 2.1.309 on
  Windows and 2.1.310 on Linux and macOS, October 2026).
- **VS Code 1.143's bundled Copilot agent** writes
  `~/.copilot/logs/.copilot-log.lock` and
  `~/.copilot/logs/process-<epoch>-<pid>.log` at the editor's first start,
  with no Copilot CLI installed (observed on Ubuntu 26.04 and macOS 26.6,
  October 2026).
  `.copilot` is a `copilot-cli` catalog line, so both files are collected
  as `copilot-cli`. The Copilot CLI writes the same two paths itself
  ([Process logs in `research/copilot-cli.md`](../../research/copilot-cli.md#process-logs)),
  so a `copilot-cli` detection whose only files are under `.copilot/logs/`
  may be VS Code's rather than the CLI's; the log's own lines decide (the
  sign-in line names the account, and a session leaves `session-state/`
  beside `logs/`).
- **Skills and configuration files** that several agents read are
  collected too, with agent `shared` in the manifest.
- **`.env` files:** an agent's own `.env` inside its state directory, such
  as `~/.codex/.env` or `~/.hermes/.env`, is collected and flagged
  `secret: true`. General-purpose `.env` files in the home or a project are
  not collected.
- **Shell histories** are not collected: like process lists and logons,
  they are the EDR's job.

## Nested entries

A nested catalog entry wins over the entry that encloses it. When one tool
keeps its state inside another tool's directory, as Antigravity CLI does
under `~/.gemini/antigravity-cli`, those files are collected once,
attributed to the nested agent, and left out of the enclosing agent's walk.

## Case variants

On a case-insensitive filesystem, catalog lines that differ only by case
name one directory, which is collected once by the line listed first. On a
case-sensitive filesystem every variant that exists is collected as before.

The variants in the catalog, where both can exist on Linux:

- `.config/goose` and `.config/Goose`
- `.config/PearAI/User` and `.config/pearai/User`
- `.config/Cursor/User` inside `.config/cursor`

Case-insensitive filesystems are macOS by default, Windows, and a directory
under `/mnt/c` in WSL. The later line collects nothing and leaves one log
line ending `(case-insensitive filesystem), collected there`, with no
manifest row. A case variant of a path inside another match
(`.config/Cursor/User` inside `.config/cursor`) is treated as the nested
entry it is.

How the two collectors decide that two spellings are one directory, and how
the path is spelled in the manifest, differs:

| | `collect-agent-artifacts.sh` | `Collect-AgentArtifacts.ps1` |
| --- | --- | --- |
| Which matches are compared | Matches whose paths are equal once ASCII-lowercased, or where one lowercased path lies inside another. | Every match, after each literal path segment is replaced by its on-disk name. |
| Test for one directory | Same device and inode (`lstat`): `stat -c '%d:%i'` (GNU), `stat -f '%d:%i'` (BSD), or the inode from `ls -di` when neither `stat` works. | Same path string after the on-disk names are taken. The on-disk name of a segment is the single entry that `Directory.GetFileSystemEntries(parent, segment)` returns; when it returns none or several (two names that differ by case on a case-sensitive filesystem), the segment is kept as written. |
| Spelling in `path` and the archive | The first catalog line's spelling (`.config/PearAI/User/...` even when the directory on disk is `pearai`); a nested variant takes the enclosing match's spelling for the shared part (`.config/cursor/User/...`). | The on-disk spelling. |
| Log line for the dropped entry | `[agent] <path>: same file as <kept path> (case-insensitive filesystem), collected there` | `[agent] <path>: catalog line <glob> names the same path as an earlier line (case-insensitive filesystem), collected there` |

Claimed paths of nested entries are compared case-insensitively by the
PowerShell collector on Windows and case-sensitively elsewhere, so pwsh on
Linux keeps `.config/cursor/User` and `.config/Cursor/User` apart. With
`--inventory` (`-Inventory`) the same folding applies, and the dropped line's
glob is left out of the agent's `evidence`.

## Claude Code config homes

A second Claude Code config home named `~/.claude-<name>` (such as
`~/.claude-work`) is collected as `claude-code`, but only by the
Claude-specific names inside it. One outside the home directory, or not
named `.claude-<name>`, is not found; collect it by hand.

Claude Code reads its whole config home from `CLAUDE_CONFIG_DIR` when that
variable is set, and a second account in such a directory is common. The
names collected there:

| Name | Flagged `secret: true` |
| --- | --- |
| `projects/` | no |
| `file-history/` | no |
| `history.jsonl` | no |
| `.claude.json*` | `.claude.json` only |
| `.credentials.json` | yes |
| `hfi-auth.json` | yes |
| `.session_ingress_token` | yes |
| `remote/.oauth_token` | yes |
| `remote/.api_key` | yes |
| `sessions/*.*.key` (the `<pid>.<sha256>.key` session keys) | yes |
| `daemon/control.key`, `daemon/pipe.key`, `daemon/roster.json` | yes |
| `backups/.claude.json.*` | yes |
| `ide/*.lock` | yes |
| `bridge-spawn/*/mcp.json` | yes |

The rest of a relocated home (`settings.json`, `todos/`, `plans/`,
`shell-snapshots/` and the other subdirectories) is not collected, because
a bare `.claude-*` entry would also take other tools' directories such as
`~/.claude-code-router` and `~/.claude-mem`.

## Claude Code CLI cache directory

The Claude Code CLI keeps per-project MCP server logs outside its config
home, in a cache directory that `CLAUDE_CONFIG_DIR` does not move:
`<project slug>/mcp-logs-<server>/<timestamp>.jsonl`, one file per server
per start of the CLI, one JSON object per line with `cwd`, `sessionId`,
`timestamp` and either `debug` or `error`. It is
collected as `claude-code` from these locations, none flagged
`secret: true`:

| Location | Platform |
| --- | --- |
| `Library/Caches/claude-cli-nodejs` | macOS (observed with CLI 2.1.295, October 2026) |
| `.cache/claude-cli-nodejs` | Linux, when `XDG_CACHE_HOME` is unset (observed with CLI 2.1.310, October 2026) |
| `AppData/Local/claude-cli-nodejs/Cache/*` | Windows (observed with CLI 2.1.309, October 2026) |

On Windows the catalog names each project-slug directory under `Cache/`
rather than the directory itself, because the PowerShell collector matches
exclusions without regard to case and the Claude Desktop exclusion
`*/Claude*/Cache` would prune `AppData/Local/claude-cli-nodejs/Cache`
whole; the slug directories are not named `Cache`, so both collectors
collect them. A relocated `XDG_CACHE_HOME` on Linux is not found.

Two more directories outside the config home are collected as
`claude-code` from collector 1.19.0, at the same paths under the home on
Linux, macOS and Windows (observed with CLI 2.1.309 on Windows and 2.1.310
on Linux and macOS, October 2026):

| Location | What it holds |
| --- | --- |
| `.local/state/claude` | `locks/<version>.lock`, a JSON object with `acquiredAt`, `execPath`, `pid` and `version` (138 bytes on Linux and macOS), written when a session of that version starts and removed when it exits; the `locks/` directory is empty otherwise, so a lock in a collection names the process and binary of a session that had not exited. |
| `.cache/claude` | `staging/`, which the native installer creates on every platform; it was empty whenever it was listed. |

On macOS the CLI also writes `~/Applications/Claude Code URL Handler.app`
when its first session exits, a small application bundle (1.1 KB) that
registers the CLI as the handler of `claude-cli:` links with Launch
Services, the macOS registry of which application opens which kind of
link (observed with CLI 2.1.310 on macOS 26.6, October 2026). It is
collected as `claude-code` from collector 1.19.0: `Contents/Info.plist`
names the `claude-cli` URL scheme, and `Contents/MacOS/claude` is a
symbolic link to `~/.local/bin/claude`, recorded as a `symlink` row with
its target. The bundle is outside `~/.local` and `~/.claude`, so it is
evidence that the CLI was installed and run even when those directories
were removed by hand.

## Claude Code temp root and git excludes

The Claude Code CLI gives each session a directory under its temp root,
`<project slug>/<session id>/`, holding `scratchpad/`, the model's
scratch space, and `tasks/`, the `<id>.output` files of background
commands and subagents. On Windows, Claude Code's own cleanup at a later
session start removed the directories of earlier sessions, open ones
included, and the directories outlived a reboot (observed with CLI
2.1.309, October 2026):

| Platform | Temp root | Collected |
| --- | --- | --- |
| Windows | `AppData/Local/Temp/claude` under the home | From collector 1.19.0, as `claude-code`, none flagged `secret: true` |
| Linux, macOS | `/tmp/claude-<uid>` on Linux; `/private/tmp/claude-<uid>` on macOS, also reached as `/tmp/claude-<uid>`, not the per-user temporary directory in `$TMPDIR` | No: outside every home; copy it by hand ([issue 163](https://github.com/seanthegeek/doubleagent/issues/163)) |

On Linux and macOS the temp root holds more than the session directories,
and lasts a shorter time (observed with CLI 2.1.310 on Ubuntu 26.04 and
macOS 26.6, October 2026):

- **One directory per process, not per session.** A new session's
  directory is named by its session id. A resumed session
  (`claude --resume`, `claude --continue`), the `claude agents` console
  and a command that starts no session, such as `claude mcp add` or
  `claude mcp list`, each get a directory of their own under another id,
  so the directories outnumber the sessions in `~/.claude/projects/`. On
  macOS a `/clear` added a directory under the new session's id, and one
  session of the VS Code extension made two. Every level is mode 0700
  except `tasks/`, which is 0775 on Linux and 0755 on macOS and appears at
  the session's first shell command. On macOS a session's directory is
  created when the session starts, before its first prompt; the first
  session after the sign-in made none, and left the temp root itself
  empty.
- **`scratchpad/`** holds files the model writes there for later use, and
  they exist nowhere else: on macOS a shell command the model proposed
  wrote a 31 KB word list to `scratchpad/words-3000.txt` (mode 0644).
- **`tasks/`** holds a background command's output as
  `<9 base-36 characters>.output` (22 bytes each for two `sleep`
  commands on the install observed), and a subagent's as
  `<agent id>.output`, which on Linux and macOS is a symbolic link to the
  subagent's transcript
  `~/.claude/projects/<slug>/<session id>/subagents/agent-<agent id>.jsonl`
  (on Windows it was an empty file).
- **`bash-edit-diff/<19 digits>-<19 digits>-<16 hex>/`**, at the top of
  the temp root, is a git repository Claude Code makes at a shell command
  to see what the command changed: a snapshot commit of the project's
  working tree, with an `objects/info/alternates` file (git's list of
  other object stores to read) naming the project's `.git/objects`. On
  macOS it is a bare repository (one with no working tree) holding
  `HEAD`, `config`, `config.global`, `index`, `info/attributes`,
  `info/exclude` (a copy of the project's), `objects/` and
  `refs/heads/<branch>`. It is removed when the session exits; the empty
  `bash-edit-diff/` stays.
- **The session directories outlive `/exit`** and are removed by
  Claude Code's retention sweep, which deletes a transcript older than
  `cleanupPeriodDays` (30 days unless set) together with its session's
  `scratchpad/` directory. On the install observed the sweep ran 10 to 20
  minutes after a session's last prompt and never in a session that had
  none, so the temp root reaches back about as far as the transcripts; on
  macOS its marker, `~/.claude/.last-cleanup`, was written about 20
  minutes after a session's first prompt.
- **A reboot clears it.** `/tmp` on Ubuntu 26.04 is a tmpfs, a
  file system held in memory, and macOS empties `/private/tmp` at boot, so
  after a reboot nothing of the temp root is left: no scratchpads, task
  outputs or snapshot repositories. On Windows the temp root outlived a
  reboot.

On macOS each CLI process also creates a Unix socket,
`/tmp/cc-socks/<pid>.sock` (mode 0600, in a directory of mode 0700), when
it starts, and removes it when it exits through `/exit`; a process that
ends another way leaves its socket behind, as the native installer's own
run of the CLI did. The socket holds no data and is outside every home,
so it is not collected; its name is the process ID of a CLI process, and
a reboot removes the directory. No `cc-socks` directory was seen in
`/tmp` on Linux.

With `claude --debug` the CLI also writes `~/.claude/debug/<session id>.txt`
and a `debug/latest` symbolic link to it, both kept after the session
exits. They are under the config home and are collected as `claude-code`:
the log as a file, the link as a `symlink` row with its target (see
[Statuses in `output.md`](output.md#statuses)).

When a user grants a standing permission in a project, Claude Code
writes it to the project's `.claude/settings.local.json` and adds
`**/.claude/settings.local.json` to the user's global git exclude file,
`~/.config/git/ignore`, so the grant stays out of commits (observed with
2.1.309 on Windows, October 2026, where it created the file). The
collector does not take that file: it is git's, and a catalog line for
it would mark every home that has one as a Claude Code home. Check it by
hand when it matters which patterns Claude Code added. Claude Code also
rewrites a project's `.git/info/exclude` with patterns for its own
project files (eleven lines naming `claude` after a `/loop` scheduled
task on the install observed); from 1.19.0 that file is collected with
the project's [git metadata](#git-metadata), so its patterns are in the
collection.

## Claude Desktop data directories

Claude Desktop is collected as `claude-desktop` from these locations under
each home; the `*/Claude*/` exclusions (caches, the bundled `claude-code`
binary, `claude-code-vm` (the Linux CLI build that runs inside the Cowork
VM), `vm_bundles` (see [Cowork VM disks](#cowork-vm-disks)), `sentry`) and secret globs
(`config.json`, `Cookies*`, `Local State`, `claude_desktop_config.json`,
`host-creds-*.json`, `ccd-session-secrets/*`, `.audit-key`, the space-link
signing key `spaces-present/*/link-key`) apply in each data directory.

| Location | What it holds |
| --- | --- |
| `Library/Application Support/Claude`, `.config/Claude`, `AppData/Roaming/Claude` | The data directory on macOS, Linux, and an unpackaged Windows install. On Linux (the official `.deb`, observed 2026-10-11) the logs are inside it, in `.config/Claude/logs/`. |
| `AppData/Local/Packages/Claude_*/LocalCache/Roaming/Claude`, `AppData/Local/Packages/AnthropicPBC.Claude_*/LocalCache/Roaming/Claude` | The data directory of the MSIX build (MSIX, the Windows packaged-app format). The app reports it as `AppData\Roaming\Claude`, but MSIX filesystem virtualisation stores it here, and `AppData\Roaming\Claude` does not exist on disk. |
| `AppData/Local/Claude` | The MSIX build's logs (`logs/main.log`, `mcp.log` and the others). |
| `AppData/Local/Claude-Data` | An alternative Windows data directory the app can choose, collected whole including its `<package family>` child. |
| `Library/Logs/Claude` | macOS logs. |
| `Claude-3p` under `Library/Application Support`, `Library/Logs`, `.config`, `AppData/Roaming` and `AppData/Local` | The third-party-inference build. |
| `Claude`, `Documents/Claude` | The user output folders. |

The Code tab's shadow git state (private git index copies, `shadow-of`
and `*-gitdir` directories) lives in the Claude Code CLI's temp root,
`/tmp/claude-<uid>` on Linux, which the app records in
`git-shadow/cli-temp-roots.json` under the data directory. That root is
outside every home and is not collected; the JSON list is (see
[Claude Code temp root and git excludes](#claude-code-temp-root-and-git-excludes)). The CLI
binary the app downloads into `claude-code/<version>/<hash>/` (255 MB on
Linux, 247 MB on Windows) is excluded by `*/Claude*/claude-code` on every
platform and gets one `skipped_excluded` row with its size (on macOS a
`claude.app` bundle per version, 476 MB for two versions observed).

On macOS the data directory also holds `Partitions/launch-preview-static/`
and `Partitions/cowork-file-preview/`, the Chromium profiles of two
Electron session partitions (observed with 2.31226.1, October 2026). They
are collected; the `Cookies*` secret glob crosses `/` and flags the
launch-preview partition's `Cookies` and `Cookies-journal`, and the
`*/Claude*/Cache` and `*/Claude*/Code Cache` exclusions take each
partition's `Cache/` and `Code Cache/`.

`CLAUDE_USER_DATA_DIR`, when set, moves the data directory and the logs
anywhere. That location is not a catalog line and is not collected; when
the variable was set on the host, copy the directory it names by hand.

## Cowork VM disks

Cowork runs its sessions in a virtual machine whose disks live in
`vm_bundles/` under the data directory. The whole directory is excluded
by the `*/Claude*/vm_bundles` glob: by default it yields one
`skipped_excluded` row with its on-disk size and no rows for the files
inside, and `--full` (`-Full`) collects it like any other exclusion,
each file still subject to `--max-file-size`. Observed on macOS, Claude
Desktop 2.31226.1, October 2026 (no `vm_bundles` directory exists on the
MSIX build or on the Linux `.deb` install, where the Cowork VM
was unsupported in a nested-virtualisation guest):

| File | Size | What it is |
| --- | --- | --- |
| `claudevm.bundle/rootfs.img` | 10 GiB (10,737,418,240 bytes) | The guest root filesystem. No user data. |
| `claudevm.bundle/sessiondata.img` | 29 MiB (30,408,704 bytes) | The session disk: every Cowork session's `/sessions/<name>/` working tree outside the mounted host folders. A file the agent wrote only inside the VM and never into a mounted folder is only here. |
| `claudevm.bundle/vmlinuz`, `initrd`, `initrd-micro` | 56 MiB, 27 MiB, 4.3 MiB | The guest kernel and initrds. |
| `claudevm.bundle/efivars.fd` | 128 KiB | Firmware variables. |
| `claudevm.bundle/vmIP`, `machineIdentifier`, `gvisorMacAddress`, `.cowork-adopted`, `.*.origin` | bytes | VM identity and provenance markers. |
| `warm/<sha1>/` | empty | Pre-warmed VM slots. |

Transcripts are never on these disks: the guest's `.claude` directory is
a mount of the host session directory under `local-agent-mode-sessions/`,
which is collected. When the files a Cowork session wrote inside the VM
matter, copy `claudevm.bundle/sessiondata.img` by hand (`cp -p`, then
hash it) rather than collecting the bundle with `--full`, which also
takes the 10 GiB root filesystem and needs `--max-file-size 0` or at
least `10240` to keep it; with the default 256 MiB limit `--full` takes
the session disk and records `rootfs.img` as `skipped_size`.

## Antigravity data directories

Antigravity ships as a CLI, `agy`, and a desktop app. The desktop app
(version 2.x) is an Electron app whose bundle calls the product "AGY Hub";
the VS Code-fork editor that carried the Antigravity name before 2.0 is
now a separate download, Antigravity IDE. Both the CLI and the app run
the same Go backend, which the app calls its language server, and keep
their state under `~/.gemini`. Everything is collected as `antigravity`:

| Location | What it holds |
| --- | --- |
| `.gemini/antigravity` | The desktop app's data directory: `conversations/<uuid>.db` (one SQLite database per conversation), `conversation_summaries.db`, `brain/<uuid>/` (per-conversation transcript logs and tool output), `annotations/`, `scratch/` (documents the agent wrote), `plugin_data/<plugin>/`, `sidecar_data/<name>/logs/sidecar.log` (the scheduler log of each automation) and `antigravity_state.pbtxt`. Observed on macOS with app 2.23.0 and on Linux with app 2.23.1, October 2026; before 2.0 the IDE used this directory. |
| `.gemini/antigravity-cli` | The CLI's data directory, with the same conversation, summaries and `brain/` layout plus `history.jsonl`, `settings.json` and its logs under `log/`. |
| `.gemini/antigravity-ide` | The data directory of the separately installed Antigravity IDE. |
| `.gemini/antigravity-backup` | The backup the desktop app takes when it migrates pre-2.0 IDE data into `antigravity-ide`. |
| `.gemini/config` | Configuration the CLI and the app share: `config.json`, `mcp_config.json`, installed plugins under `plugins/`, project files under `projects/`, and `sidecars/<name>/sidecar.json`, one per automation (a sidecar is the app's name for a helper process its language server runs; the `schedule` kind runs an agent task on a schedule). |
| `Library/Application Support/Antigravity`, `.config/Antigravity`, `AppData/Roaming/Antigravity` | The desktop app's Electron user-data directory, the per-user directory where Electron and its Chromium engine keep the app's own state: `app_storage.json` (UI state, including the last project and the signed-in account name), `Cookies`, `Local Storage/`, `Session Storage/` and the Chromium caches. The macOS path was observed with app 2.23.0 and the Linux and Windows paths with app 2.23.1. On Linux and Windows the logs are inside it, in `logs/` (observed on both). |
| `Library/Logs/Antigravity` | The desktop app's logs on macOS: `main.log` and `language_server.log`. |
| `.cache/antigravity`, `AppData/Local/antigravity` | The plugin marketplace staging directory under Go's per-user cache directory (Linux, and Windows from collector 1.12.0); empty on every install observed. |
| `.config/antigravity/plugin-secrets.json`, `Library/Application Support/antigravity/plugin-secrets.json`, `AppData/Roaming/antigravity/plugin-secrets.json` | The CLI's file store for plugin credentials, used when no OS keychain is available. CLI 1.3.4 names the Linux path in its log; the macOS and Windows paths are inferred from Go's per-OS configuration directory and were not observed. On macOS and Windows the directory is the desktop app's user-data directory when the app is installed, and the file is collected once. |

The exclusions take the binaries and caches out of these directories,
each with one `skipped_excluded` row: `bin/` in each `.gemini` data
directory (the app's `webm_encoder` was 12.7 MB on the observed install),
each plugin's `plugin_data/<plugin>/bin/` (the Google Docs plugin's
binary was 31.9 MB), `cache/marketplaces/` (a copy of the plugin
marketplace and of each plugin downloaded from it), agent worktrees under
`brain/<uuid>/.system_generated/worktrees`, the sidecar SDK (the
JavaScript library the app installs for automation code) in
`.gemini/antigravity*/lib/node_modules` and `.gemini/config/node_modules`,
and the Electron caches `Cache`, `Code Cache`, `GPUCache`,
`DawnGraphiteCache`, `DawnWebGPUCache`, `blob_storage`,
`Shared Dictionary`, `CachedData` and `Crashpad` and the spell-check
`Dictionaries` in the user-data directory. The rest of
`.gemini/antigravity/cache/` is collected: the user-data directory's
`Cache` exclusion names its three locations
(`Library/Application Support/Antigravity/Cache`,
`.config/Antigravity/Cache`, `AppData/Roaming/Antigravity/Cache`), so the
PowerShell collector's case-insensitive match does not prune it.

The CLI's file token `antigravity-oauth-token`, each plugin's OAuth
credential, `plugin_data/<plugin>/token.json` in any `.gemini/antigravity*`
directory, and the CLI's `plugin-secrets.json` are flagged
`secret: true`; the `plugin-secrets.json` patterns accept the directory
spelled `antigravity` or `Antigravity`. The Electron `Cookies` and
`Trust Tokens` databases and `Local Storage/` are collected unflagged;
`Cookies` held no rows on the macOS install. On the observed macOS and
Linux installs a scheduled automation created no launchd job, systemd
unit, autostart entry or cron entry: it runs inside the app's language
server while the app runs, its definition is the `sidecar.json` above,
and its scheduler log is in `.gemini/antigravity/sidecar_data/<name>/logs/`.

## ChatGPT desktop data directories

On Windows the ChatGPT desktop app installs from the Microsoft Store as
an MSIX package (the Windows app package format, whose per-user data
directory is `AppData/Local/Packages/<package family name>`). By October
2026 the package was named `OpenAI.Codex` (ChatGPT 26.1007.9446.0,
observed on Windows 11 ARM64): the app is a Chromium build with OpenAI's
Codex agent bundled, and the bundled Codex keeps its state in `~/.codex`,
collected as `codex-cli`. The older `OpenAI.ChatGPT-Desktop` package is
still collected where it exists. Collected as
`chatgpt-desktop`, with `<pkg>` for `AppData/Local/Packages/OpenAI.Codex_*`:

| Location | What it holds |
| --- | --- |
| `<pkg>/LocalCache/Roaming/Codex/web/Codex/Default` | The Chromium profile: `Preferences`, `History`, `Local Storage/`, `IndexedDB/` (one `app://` origin, the app's own UI), `Network/Cookies`, `Login Data`, and two storage partitions (Chromium's isolated storage for an embedded web view) under `Partitions/`, `codex-browser-app` (the in-app browser the agent drives) and `codex-mcp-app-sandbox%3Ashared`. |
| `<pkg>/LocalCache/Roaming/Codex/web/Codex/Local State`, `Last Version`, `statsig-state.json`, `.codex-tpp-local-executor-installation-id` | The files beside the profile; `Local State` holds the key that decrypts the cookie and login databases. |
| `<pkg>/LocalCache/Local/Codex` | The app's own log, `Logs/<yyyy>/<mm>/<dd>/codex-desktop-*.log`. |
| `<pkg>/Settings` | The package's settings hive, `settings.dat`. |

The rest of the `web/Codex` directory, Chromium's component data, crash
reporting and metrics (about 160 MB on the install), is not named by a
catalog line and gets no manifest row. The profile's caches (`Cache`,
`Code Cache`, `GPUCache`, `DawnGraphiteCache`, `DawnWebGPUCache`,
`blob_storage`, `Service Worker/CacheStorage` and `ScriptCache`) and its
bundled `Extensions` are excluded in `Default` and in each partition.
`Local State`, `Network/Cookies*` and `Login Data*` are flagged
`secret: true`; while the app runs the cookie stores, the LevelDB `LOCK`
files and the settings hive cannot be copied (see
[Locked files and long paths](windows.md#locked-files-and-long-paths)).
In `~/.codex` the app adds `.codex-global-state.json`, which holds
ChatGPT conversation resume tokens and the user's prompts and is
flagged `secret: true`, and copies of the Codex binaries in
`plugins/.plugin-appserver`, which are excluded.

On the install checked, a plain chat and an image generation left no
local transcript: the chat's only local traces are a title and a resume
token, so ChatGPT conversations are server-side. Not collected: the
binary copies and Node.js runtime under `AppData/Local/OpenAI/Codex`, the
1.27 GB runtime under `.cache/codex-runtimes`, and the sandbox's scratch
directories in `AppData/Local/Temp`, which have no stable name. The
evidence is in
[`research/chatgpt-desktop.md`](../../research/chatgpt-desktop.md).

On macOS the app is the same build, a Chromium shell hosting an Electron-style web app (ChatGPT 26.1007.71326,
observed on macOS 26.6.2, October 2026), with the bundle identifier
`com.openai.codex`; the paths of the former native app
(`com.openai.chat`) no longer appear, but they are still collected on
hosts that have them. Collected as `chatgpt-desktop`:

| Location | What it holds |
| --- | --- |
| `Library/Application Support/Codex/Default`, and `Local State`, `Last Version`, `statsig-state.json`, `.codex-tpp-local-executor-installation-id` and `browser-sidebar-page-states.json` beside it | The Chromium profile and the files beside it, as on Windows, except that the cookie store is `Default/Cookies` and `Local State` holds no cookie key. |
| `Library/Application Support/Codex/executor-plugins`, `artifact-sessions` | A bundled MCP server for the app's own tools (its 949 KB `server.mjs` is excluded), and an empty working directory. |
| `Library/Application Support/com.openai.codex`, `Library/Application Support/com.openai.codex.installer` | The Sparkle update switch (collected as `codex-cli` before collector 1.12.0) and the installer's install identifiers. |
| `Library/Logs/com.openai.codex` | The app's own log. |
| `Library/Preferences/com.openai.codex.plist`, `com.openai.codex.installer.plist`, `com.openai.sky.CUAService.plist` | Preferences of the app, its installer and the computer-use service (`com.openai.sky.CUAService`). |
| `Library/HTTPStorages/com.openai.codex`, `com.openai.sky.CUAService` and the `.binarycookies` beside each | URL-session stores and cookies; the cookies are flagged `secret: true`. |
| `Library/Group Containers/2DC432GLL2.com.openai.sky.CUAService` | The computer-use service's group container. |

The same profile caches are excluded, in `Default` and in each partition.
`Library/Caches/Codex`, `Library/Caches/com.openai.codex` and
`Library/Caches/com.openai.sky.CUAService` are caches and are not
collected. In `~/.codex` the app places the computer-use app bundle,
`computer-use/Codex Computer Use.app` (77 MB), which is excluded with
`.codex/computer-use/*.app`. The bundled Codex also writes
`.codex/shell_snapshots/<thread uuid>.<unix ns>.sh`, the user's exported
shell environment for each open thread, which can hold any API key the
login shell exports; the snapshots are flagged `secret: true` from
collector 1.15.0. On ChatGPT 26.1013.40934 (macOS, October 2026) a reply
to a ChatGPT conversation from inside the app left nothing on disk, and
the app's paired-phone remote control left no file of its own: its
local record is a row in `.codex/state_5.sqlite`, two keys in
`.codex/.codex-global-state.json` and rows in the app server's log
database `.codex/logs_2.sqlite`, all collected as `codex-cli`.

A thread with no project runs in the app's projectless output directory,
`Documents/Codex/<date>/<slug>` with `work/` and `outputs/` inside (seen
on macOS and Windows). From collector 1.16.0 that directory is collected
whole as `codex-cli`, so the files the agent wrote for the user are in
the archive, and project discovery no longer takes a thread's working
directory there for a project.

## Cursor data directories

The Cursor IDE, an editor forked from VS Code with an agent pane, keeps
its editor state in a user-data directory, `<data>`:
`Library/Application Support/Cursor` on macOS, `.config/Cursor` on Linux
and `AppData/Roaming/Cursor` on Windows. Its agent also writes to
`~/.cursor`, which is collected whole less the exclusions below. Collected
as `cursor`:

| Location | What it holds |
| --- | --- |
| `<data>/User` | The editor's settings, `globalStorage/state.vscdb`, which holds the agent conversations and the sign-in tokens (see [Credentials](#credentials)), the per-workspace `workspaceStorage`, and the `globalStorage` of Cursor's built-in extensions. |
| `<data>/logs` | One directory of logs per launch. |
| `<data>/AgentStores` on macOS, `.local/state/cursor/agent-stores` on Linux, `AppData/Local/Cursor/AgentStores` on Windows | From collector 1.17.0; the Linux and Windows locations from collector 1.18.0. The sync stores of the agent's files, one per account and one per conversation (`cursor_agent_stores/<id>/.sync/index.sqlite` with `root-id`, `mount.json` and `sync.lock`), and the pins of the open conversations (`.control/<id>/pins/`). The directory names carry the numeric account id. |
| `<data>/update-supervisor` | From collector 1.17.0. One directory per in-app update: on macOS `events.jsonl`, `result.json` and `app-ready.json`, which record which build ran from when; on Windows `win32/<installationId>/` with `app-ready.json` and `state.json` beside the excluded `inno_updater.exe`. |

The `AgentStores` and `update-supervisor` directories were seen on macOS
(Cursor 3.25.9 and 3.26.13, October 2026). On Linux (Cursor 3.25.9 from
the Ubuntu deb, October 2026) the sync stores were in the XDG state
directory, `~/.local/state/cursor/agent-stores/cursor_agent_stores`, and
not under `<data>`; `update-supervisor` was not seen on the deb install,
which is updated through apt, not by the in-app updater, and its Linux
line is kept but unobserved. The IDE on Linux downloads its copy of
the Agent CLI only when a cloud or remote-host feature is first used,
not at folder open or for local agent turns: the
`anysphere.cursor-agent-worker/agent-cli` directory appeared at the
first cloud-agent attempt, holding two partial
`cursor-agent-<version>-<pid>-<epoch ms>.tar.gz` downloads of 88.6 MB
and 35.1 MB, so the `agent-cli` exclusion below applies on Linux as
well.

On Windows (Cursor 3.26.13 from the per-user installer, Windows 11 arm64,
October 2026) the sync stores were in `%LOCALAPPDATA%\Cursor\AgentStores`
(`AppData/Local/Cursor/AgentStores`), not under `<data>`;
`%APPDATA%\Cursor\AgentStores` never existed. The Windows build updates
through an updater made with Inno Setup (a common builder of Windows
installers), and `update-supervisor` appeared only with the update to
3.26.16: an in-app update earlier the same day, while on 3.26.13, left
none. The update to 3.26.16 wrote
`update-supervisor\win32\<installationId>\` with `app-ready.json` and
`state.json`, which are collected, and a copy of `inno_updater.exe`, 3.4
MB, which is excluded by the pattern below. The updater's own log,
`%LOCALAPPDATA%\Temp\cursor-inno-updater-<epoch s>.log`, 4.9 MB, is
outside the catalog roots and is not collected. As on Linux, the IDE extracted its copy of
the Agent CLI only during the cloud-agent steps, not at folder open or
for local agent turns: the `anysphere.cursor-agent-worker/agent-cli`
directory, 262.7 MB, is excluded by the pattern below, and the worker's
bookkeeping beside it,
`cursor-agent-worker-<id>.{install,log,owner.json,restart.json,spec}`, is
collected. On that arm64 install the worker failed to load its native
module, and its `.log` holds the crash; the log grows with each spawn
attempt, from 557 KB after the first to 5.6 MB after five, and is
collected under the size cap. The Agents window also
leaves one scaffold project per launch under `~/.cursor/projects`, named
after a temporary folder it opened and removed
(`C-Users-<user>-AppData-Local-Temp-<uuid>`), with `canvases/` and `mcps/`
and about 60 small files each; their `canvases/node_modules` is excluded
and the `mcps/` tool manifests are collected.

Installing a plugin from Cursor's plugin library (the Google Calendar
plugin, Windows, October 2026) wrote three more kinds of file, two of
them collected. `<data>/User/globalStorage/mcp-oauth-attempts/<uuid>.json`
is collected: a plaintext record of the OAuth client registration made
for the plugin's MCP server (Model Context Protocol, the protocol through
which the agent calls the plugin's tools), with the client id, the
redirect URIs and the server URLs, and no token; Cursor keeps one such
file per server and replaces it at each sign-in or reconnection, and the
tokens themselves are encrypted keys inside `state.vscdb` (see
[Credentials](#credentials)).
`.cursor/projects/<project>/agent-tools/<uuid>.txt` is collected: the
tool manifest of an MCP server that the agent fetched during a turn, 38
KB on that install. The marketplace copy of the plugin under
`.cursor/plugins/cache` is excluded, below.

Excluded from collector 1.17.0, each recorded as `skipped_excluded` with
its size:

| Pattern | What it is |
| --- | --- |
| `*/User/globalStorage/anysphere.cursor-agent-worker/agent-cli` | A copy of the Cursor Agent CLI that the IDE's agent worker runs, 608 MB on the macOS install. The worker's own files beside it (`cursor-agent-worker-<id>.log` and its `.owner.json`, `.restart.json` and `.spec` siblings, and `worker-data/`) are collected. |
| `*/Cursor/update-supervisor/*/cursor-update-supervisor` | The update supervisor binary, 484 KB, in each update directory. |
| `*/Cursor/update-supervisor/*/inno_updater.exe` | From collector 1.18.0. The Windows updater binary, 3.4 MB, in each update directory. |
| `.cursor/projects/*/canvases/node_modules` | TypeScript type declarations for Cursor's canvas feature, about 226 KB per project. The project's `agent-transcripts/` and `mcps/` beside it are collected. |
| `.cursor/statsig-cache.json` | A feature-flag cache downloaded from Cursor's servers, 932 KB. |
| `.cursor/plugins/cache` | From collector 1.18.0. The marketplace copy of each installed plugin, `<marketplace>/<plugin>/<sha>/` with `.cursor-plugin/plugin.json`, `mcp.json`, a README, a licence and assets, about 7 KB for the Google Calendar plugin. Which plugins were installed stays collected, in `state.vscdb` and in the per-project `mcps/plugin-<name>-<server>/` manifests; `.cursor/plugins/local` is collected. |

`<data>/process-monitor`, Cursor's logs of the processes it runs, one
file every four hours, is not collected: process activity is the EDR's to
record (see [Host state is not collected](#host-state-is-not-collected)).
Neither are `<data>/sentry` (crash-reporting state), `<data>/CachedData`,
`<data>/CachedProfilesData` and the Chromium caches; none of them is named
by a catalog line, so they get no manifest row.

Two directories in the per-user temp directory (`$TMPDIR`, under
`/private/var/folders/.../T/` on macOS) are outside every home and are not
collected; copy them by hand when they exist:

| Directory | What it holds |
| --- | --- |
| `cursor-agent-logs-<uid>/` | The agent worker's session logs, `session-<time>-<pid>-1.log`, and a `latest.log` symlink. The worker rotates or removes them: on the macOS install they were gone within an hour. |
| `<user>-cursor-zsh/` | Copies of the user's `.zshrc`, `.zshenv`, `.zprofile` and `.zlogin` that the IDE's terminal uses for shell integration. Like Codex's shell snapshots, they can hold any API key the login shell exports. |

The evidence is in [`research/cursor.md`](../../research/cursor.md).

## Home directory discovery

The collectors discover home directories rather than copy them: each
home found is a base against which every catalog entry is tried, and only
the files the catalog names under it are collected. Discovery draws on
several sources, merges them, and de-duplicates by path. Service accounts
with real homes are included, because agents run as them too, for example
the `ollama` system user.

| | sh collector | PowerShell collector |
| --- | --- | --- |
| Live mode | `getent passwd`, or `/etc/passwd` where there is no `getent`; on macOS also `dscl . -list /Users NFSHomeDirectory`. | The registry `ProfileList`, and the globs below. |
| Image mode | `<root>/etc/passwd`, with every home prefixed with the root. | Globs only, the same six patterns as the sh collector. It does not read `etc/passwd`, so in a Linux image it misses any home outside those directories. |
| Globs | In both modes, `home/*`, `Users/*`, `root`, `var/root`, `usr/home/*` and `export/home/*` under the root, taking the directory name as the user. The patterns are expanded under the root only, whatever directory the collector is run from. | In image mode, the same six patterns; in live mode, `Users\*` and `home\*` on the system drive. |
| Dropped | Homes that do not exist; system paths that are not real homes (`/`, `/bin`, `/sbin`, `/usr`, `/usr/bin`, `/usr/sbin`, `/dev`, `/dev/null`, `/proc`, `/sys`, `/nonexistent`, `/var/empty`); `Shared`, `Public`, `Default`, `Default User` and `All Users` under `/Users`. | Profiles named `Public`, `Default`, `Default User`, `All Users`, `Shared` or `defaultuser0`, and `UMFD-<n>`, `DWM-<n>` or `TEMP`, each also with a `.<suffix>` (`TEMP.HOST`), matched in either mode and without regard to case; a candidate whose path is a file. |

The PowerShell collector logs a profile path that cannot be seen as
`profile not accessible (skipped)`. When its parent directory still lists
it (access denied rather than a deleted profile), it also gets an
`error_read` row, but it is not added to `users` and `homes`.

## Project discovery

Project-level artifacts (`CLAUDE.md`, `.claude/`, `.mcp.json`, `AGENTS.md`,
`.cursorrules`, `.aider.chat.history.md`, Crush's per-project
`.crush/crush.db`, the repository's [git metadata](#git-metadata), ...) are collected from every directory referenced in
agent state that can be read with grep, and from every `-p` directory.

### Sources read

| Agent | Where the project path is read from |
| --- | --- |
| Agent Zero | Editor (ACP) chats: `acp_cwd` and `acp_additional_directories` in `usr/chats/<chat id>/chat.json` under `agent-zero`, `agent-zero/<instance>` or `Desktop/agent-zero`; not read in a Docker volume. The path names a directory on the host that ran the editor. |
| Amp | History. |
| Antigravity | History, trusted workspaces, project cache, and the project files the CLI and the desktop app write in `~/.gemini/config/projects/` (their `file://` folder URLs; the sh collector reads them from 1.12.0). |
| Claude Code | History and project list, in `~/.claude` and `~/.claude-<name>` config homes. |
| Cline | Sessions and task history. |
| Codex | Session rollouts in `~/.codex/sessions` and, from collector 1.18.0, `~/.codex/archived_sessions`; a `cwd` under the home's `Documents/Codex` (projectless output directories) or, from 1.18.0, `.codex/worktrees` (the agent's own git worktrees, excluded) is skipped. |
| Continue | Sessions. |
| Copilot CLI | Session workspaces (`workspace.yaml` `cwd`) of sessions that have an `events.jsonl`. |
| Copilot Desktop | None of its own: its sessions are Copilot CLI session workspaces, and a `cwd` under the home's `.copilot/` (the app's chat directories and worktrees) is skipped there, as is the `cwd` of a session directory with no `events.jsonl` (a pool or probe session the app pre-created and never prompted, whose `cwd` on Windows is the app's install directory; from collector 1.13.0); the repositories it manages are in `data.db`, left to the analyzer. |
| Crush | Project list, `.local/share/crush/projects.json` and, from collector 1.20.0, the Windows `AppData/Local/crush/projects.json`, whose `C:\` paths only the PowerShell collector maps into the image; the sh collector uses the `/` paths it holds. |
| Cursor CLI | Chat and ACP session metadata. |
| Factory Droid | Sessions. |
| Gemini CLI | Project registry, trusted folders and project root markers. |
| Goose | Legacy session files, scheduled jobs' recipe directories, project files' working directories and the desktop app's recent directories. |
| Hermes | Checkpoint project records. |
| Kilo Code | Task indexes and, from collector 1.21.0, the legacy JSON project store (`storage/project/*.json` `worktree`) and the project memory manifests (`memory/*/manifest.json` `canonical`) under `.local/share/kilo`. |
| Kiro CLI | Sessions and workspace roots. |
| Letta | Session index and local-backend transcripts. |
| Muse Code | Trusted projects (`.config/muse/trust.json`) and session logs (`workspace_root`). |
| nanobot | Workspace markers and config. |
| Open Interpreter | Rollouts in `sessions` and `archived_sessions`; a `cwd` under the home's `.openinterpreter/worktrees` is skipped from collector 1.18.0. |
| OpenClaw | Workspace, working and agent directories (`workspace`, `cwd`, `agentDir`) from its JSON5 config and the legacy Clawdbot and Moltbot configs; `cwd` also names an MCP server's working directory. |
| OpenCode | The 1.x JSON project store (`.local/share/opencode/storage/project/*.json`, key `worktree`). |
| OpenHands | Workspaces and conversation metadata. |
| PearAI | Sessions. |
| pi | Session headers, trusted folders, crash log and experimental session metadata (which also cover little-coder). |
| Qwen Code | Chats. |
| Roo Code | Task indexes. |
| Tabby | `file://` repositories. |
| Twinny | The common directory of the files in each embeddings manifest. |
| VS Code family | Workspace storage, including the remote server data directories. |

### Which paths are used

- A referenced path is used only if it is absolute and names an existing
  directory that is not itself a home.
- In image mode it is looked up under the root.
- The sh collector reads POSIX paths only, so it skips drive-letter
  references in a Windows image. The exception is a `file:///c:/...` (or
  `file:///c%3A/...`) URL in the files it reads for any quoted path or
  `file://` URL (the Antigravity, Kiro, Goose and Twinny sources): from
  1.12.0 it becomes the same path under the root with the drive letter
  dropped, as in the PowerShell collector.
- The PowerShell collector maps `C:\...` and `file:///c:/...` references to
  the same path under `-Root`, dropping the drive letter. On a live Windows
  host it ignores POSIX paths.

### What is taken from each project

From each project only the `PROJECT_CATALOG` entries are collected. Their
rows have as agent the catalog agent that owns the file (`claude-code` for
`.claude/` and `CLAUDE.md`, `aider` for `.aider.chat.history.md`, `little-coder`
for `.pi/approved-plan.md` inside pi's `.pi/`), or `project` for a file
several agents read (`AGENTS.md`, `AGENTS.override.md`, `AGENT.md`, `.agents/`,
`.mcp.json`, `.github/copilot-instructions.md`,
`.github/git-commit-instructions.md`, `.github/instructions/`, `SOUL.md`,
`USER.md`, and the [git metadata](#git-metadata) under `.git/`); `--list`
prints the table. Before collector 1.12.0 every project
row had agent `project`. Each row has `home` set to the project directory,
and the user whose state referenced it (the first one, when several did). `-p`
directories join the same list. Each gets as its user the owner of the home
it sits under, or none.

A nanobot workspace outside `.nanobot*` keeps its scheduled jobs
(`cron/jobs.json`) and local triggers (`triggers/triggers.json`) beside
one run record per scheduled or triggered run (`cron/runs/`,
`triggers/runs/`), each with the session key, the prompt and the reply,
which outlives the session it ran in; those four entries are collected from
the workspace, and the trigger delivery queue (`triggers/inbox/`,
`processing/`, `failed/`) is not. The default workspace under `.nanobot/` is
collected whole by the home catalog.

Aider's transcript (`.aider.chat.history.md`, with its input and LLM
histories beside it) and Crush's session database (`.crush/crush.db`) live
only inside the project, so a project deleted before collection leaves
nothing under the home to recover those sessions from. The other agents in
[Secondary sources](../../analyzer/docs/output.md#secondary-sources) keep
their transcript store and its second copy under the home, or together in
the agent's Docker or Podman volume (Hermes, Letta, nanobot and Tabby have
`DOCKER_VOLUMES` lines), where both survive the project.

### Git metadata

From collector 1.19.0, the git metadata of each project, discovered or
given with `-p`, is collected under agent `project`. It says where the
project's code came from, where it may have been pushed, and when the
branches moved:

| Path under the project | What it holds |
| --- | --- |
| `.git/config` | The remotes, each with its fetch URL (`url`) and push URL (`pushurl`, when one is set), the branch tracking settings, and any repository-local `user.email` and credential helper. |
| `.git/HEAD` | The branch checked out at collection time (`ref: refs/heads/<name>`), or a commit hash when no branch is checked out. |
| `.git/FETCH_HEAD` | The branches the last fetch or pull brought in and the URL they came from. |
| `.git/refs/`, `.git/packed-refs` | The local branches, the remote-tracking branches (`refs/remotes/<remote>/`) and the tags, each with the commit it names. |
| `.git/logs/` | The reflog: one line per checkout, commit, merge, pull, reset and remote-tracking update, with the time, the name and email git recorded, and the old and new commit. |
| `.git/info/exclude` | The repository's own ignore patterns, kept outside the tree. Claude Code adds patterns for its project files here, so lines naming `claude` show it ran in the project; the user's global exclude file, where it adds a pattern too, is not collected ([Claude Code temp root and git excludes](#claude-code-temp-root-and-git-excludes)). |
| `.git/worktrees/` | One directory per linked worktree (a second checkout of the same repository): its `HEAD`, its `gitdir` file naming where that checkout is, and its own reflog under `logs/`. Each worktree's staging area (`index`, the `sharedindex.<hash>` files of split-index mode, and an `index.lock` left by an interrupted update) and its `modules/`, the submodule repositories checked out in that worktree, are excluded (`.git/worktrees/*/index`, `.git/worktrees/*/index.lock`, `.git/worktrees/*/sharedindex.*`, `.git/worktrees/*/modules`) and get a `skipped_excluded` row each. |

Nothing else under `.git/` is collected: the object store (`objects/`),
the index, `hooks/`, LFS objects (`lfs/`, Git Large File Storage) and
submodule repositories (`modules/`) match no line and have no manifest
row, like any other project file no line names. The object store and LFS
objects hold the repository's content, which can be large and which the
remote holds as well.

A checkout whose `.git` is a file rather than a directory, a linked
worktree or a submodule, gets none of these files: the file points at a
git directory elsewhere, which is not followed
([issue 234](https://github.com/seanthegeek/doubleagent/issues/234)).

A remote URL can carry credentials in its user part
(`https://<user>:<token>@<host>/<owner>/<repo>.git`). `.git/config` is
collected unflagged all the same, so `--no-secrets` keeps the remotes;
check its `url` and `pushurl` values and redact that part before sharing
the collection.

### Left to the analyzer

Project paths recorded only inside SQLite, and in zstd-compressed rollouts,
are not discovered; they are left to the analyst-side parser:

- Zed
- Goose
- OpenCode
- Kilo Code
- Kiro CLI
- Hermes `state.db` and `projects.db`
- OpenClaw `openclaw.sqlite`
- Tabby `ee/db.sqlite`
- Copilot Desktop `data.db` (`projects.main_repo_path`, `worktrees.path`
  and `workspace_checkout_bindings`: the repositories the app cloned under
  `.copilot/repos`, which the collector excludes)
- the PearAI Roo fork in `state.vscdb`
- Codex and Open Interpreter rollouts compressed with zstd
- CLAI 2.0 `sessions.db` (`conversations.metadata` key `workspace`), whose
  projects' `.clai` directories are collected only when another source or
  `-p` names the project

## Agents outside dot directories

Most agents keep their state in a dot directory; a few do not, and some of
their state must be copied by hand.

- **Agent Zero** lives in its install directory (`~/agent-zero`,
  `~/Desktop/agent-zero`). It is also commonly deployed in containers; see
  [docker.md](docker.md). The A0 CLI connector, a host-side client of
  Agent Zero, writes each computer-use screen capture to
  `_a0_connector/computer_use/<chat id>/` in the system temp directory and
  deletes older captures as new ones arrive. On Windows that is
  `AppData/Local/Temp/_a0_connector`, which the catalog collects; on Linux
  and macOS it is under `/tmp` or `$TMPDIR`, outside any home, so copy
  `_a0_connector` from there by hand (the connector's
  `A0_COMPUTER_USE_HOST_ARTIFACT_ROOT` environment variable moves it). On a
  Windows host without Docker Desktop, the A0 Launcher, Agent Zero's
  instance manager, can import its own WSL distribution under
  `AppData/Local/agent-zero/wsl/Ubuntu`, whose virtual disk holds the
  Docker engine and the instances' named volumes and stays after the
  launcher is uninstalled. Neither collector opens that disk or lists its
  volumes, so collect them from inside the distribution as for
  [Docker Desktop](docker.md#docker-desktop)
  ([Host-side companions in `research/agent-zero.md`](../../research/agent-zero.md#host-side-companions)).
- **Local Deep Research** writes reports under
  `~/Documents/LocalDeepResearch`. It is also commonly deployed in
  containers; see [docker.md](docker.md).
- **ShellGPT** keeps its chat history in the system temp directory (`/tmp`
  on Linux, `/var/folders/.../T` on macOS), which no home-relative catalog
  entry can reach, so on Linux and macOS it is not collected and must be
  copied by hand. On Windows the catalog collects the chat directory
  `AppData/Local/Temp/chat_cache` and the pre-0.9.0 `AppData/Local/Temp/shell_gpt`;
  the current response cache, `AppData/Local/Temp/cache`, is not collected
  because the name is too common in a temp directory, so read `CACHE_PATH`
  from `.sgptrc` and copy that directory by hand
  ([Chat history and response cache in `research/shellgpt.md`](../../research/shellgpt.md#chat-history-and-response-cache)).
- **OpenCode** 2.0 creates a temp root, `opencode` in the system temp
  directory (`/tmp/opencode` on Linux, under `/var/folders` on macOS), and
  tells the model to write its temporary files there. On Linux and macOS
  it is outside every home and must be copied by hand; on Windows the
  catalog collects `AppData/Local/Temp/opencode`
  ([Scratch directories in `research/opencode.md`](../../research/opencode.md#scratch-directories)).
  The OpenCode desktop app keeps its data in `ai.opencode.desktop` (or
  `.beta`, `.dev`) under `Library/Application Support`, `AppData/Roaming`
  or `.config`; it is collected, and its V8 `compile-cache` and, from
  collector 1.21.0, its `cli` copies of the OpenCode executable are excluded
  ([OpenCode desktop app in `research/opencode.md`](../../research/opencode.md#opencode-desktop-app)).
- **Kilo Code** uses `kilo/` in the system temp directory as the
  temporary directory of every command its sandbox confines, and writes
  files that Kilo Cloud sends over remote control to
  `kilo/remote-attachments/<session>/` there, removing them when the
  session's remote handling ends; no catalog line reaches it on any
  platform, so copy it by hand, while a remote session is live for the
  attachments
  ([Scratch directories in `research/kilo-code.md`](../../research/kilo-code.md#scratch-directories)).
- **Gemini CLI** writes `gemini-client-error-<type>-<timestamp>.json` to
  the system temp directory (`/tmp` or `$TMPDIR`, `%TEMP%` on Windows,
  by default `AppData/Local/Temp` in the profile) when a model request
  fails; the file holds the error and the conversation history sent with
  the request. Its VS Code companion extension writes
  `gemini/ide/gemini-ide-server-<parent pid>-<port>.json` there,
  holding the port, the workspace path and a per-run auth token. No
  catalog line reaches either on any platform, so copy them by hand
  ([Per-user storage in `research/gemini-cli.md`](../../research/gemini-cli.md#per-user-storage)).
- **Muse Code** keeps its launcher state beside the launcher; see below.
- **Cursor** writes its agent worker's logs and copies of the user's zsh
  startup files to the temp directory; see
  [Cursor data directories](#cursor-data-directories).
- **Roo Code's CLI** run as `roo --ephemeral` keeps its whole storage,
  tasks included, in `roo-cli-<epoch ms>-<random>` under the system temp
  directory (`$TMPDIR` or `/tmp`, `%TEMP%` on Windows) and removes it when
  the CLI exits, so it survives only a CLI killed before its cleanup ran.
  No catalog entry reaches it; when a `roo --ephemeral` process was
  killed, copy any `roo-cli-*` directory in the temp directory by hand
  ([CLI ephemeral storage in `research/roo-code.md`](../../research/roo-code.md#cli-ephemeral-storage)).
- **Zed** on macOS gives an agent thread whose terminal runs in the OS
  sandbox its own temp directory, `$TMPDIR/zed-agent-terminal-<random>`
  (under `/private/var/folders/.../T/`), kept until the thread is deleted.
  It is outside every home and is not collected; copy it by hand when it
  exists. The thread's record in `threads.db`, which is collected, names
  it ([Scratch directories in `research/zed.md`](../../research/zed.md#scratch-directories)).
  The linked git worktrees Zed creates for a thread sit beside the
  project (`<project parent>/worktrees/<name>/<project>`) and are recorded
  only in SQLite, so their project files are collected only when another
  source or `-p` names them.
- **OpenClaw, Codex CLI, Continue and Gemini CLI** write backups,
  exports or a log to a directory the user chooses or works in; see
  [Files written where the user chooses](#files-written-where-the-user-chooses).

### Muse Code

| Location | Collected |
| --- | --- |
| Linux and macOS, `~/.local/bin` | Only Muse's own dot files (`.local/bin/.muse-*`: `.muse-version`, `.muse-release-info.json`, `.muse-update-checked-at` and the like), since other tools share the directory. The launcher's transient update directory `.muse-update.XXXXXX/`, which holds a candidate binary during an update, is excluded and gets one `skipped_excluded` row with its size. The `muse-bin-<version>` binary next to them is not collected and leaves no manifest row. |
| Windows, `AppData/Local/Programs/muse` | The install directory whole. Its `muse-bin-<version>.exe` and staged `.muse-update.<pid>.exe` binaries are excluded, so their `skipped_excluded` rows record the installed version and size. |
| Anywhere else (`MUSE_INSTALL_DIR`) | Not collected. |

Two Muse Code locations are system-wide rather than per home and must be
copied by hand:

- The enterprise policy: the registry key `HKLM\SOFTWARE\Policies\Muse` on
  Windows, and `.enterprise-defaults.json` and `enterprise-policy.json`,
  read as `/Library/Application Support/muse/` on macOS; the Linux location
  is not known.
- On Windows, the elevated shell-sandbox setup root under `%ProgramData%`.

### Files written where the user chooses

Some agents write a copy of their state, or a record of an upload, to a
directory the user names or the one the agent was started from. No catalog
entry can reach these, so copy them by hand when they exist:

| Agent | File | Where to look |
| --- | --- | --- |
| OpenClaw | `openclaw backup create` archives (`.tar.gz` archives of OpenClaw's state and configuration and, unless `--no-include-workspace` was passed, the workspace), `openclaw backup sqlite create` snapshot repositories and `openclaw backup git` repositories, local copies of what may also have been uploaded offsite or pushed to a Git remote. | The `archive_path` column of the `backup_runs` table in the collected `state/openclaw.sqlite`, one row per backup attempt. An archive made without `--output` goes to the working directory it was run from (the home when that directory is inside the state tree); a repository is wherever `--repository` pointed. |
| Codex CLI | `error.log`, appended to by `codex cloud` (which starts a task on OpenAI's servers), with one `new_task: created id=<task id>` line per task it created. | The directory `codex cloud` was started from; nothing under `~/.codex` records it. |
| Continue | `<YYYYMMDDTHHMMSS>_session.md`, a Markdown copy of a chat written by `/share` or the history page's share button. | `~/.continue` (collected) unless the user picked a folder or set `outputDir`. |
| Gemini CLI | `gemini-conversation-<epoch ms>.json` or `.md` from `/chat share`, and the JSON file `/export-session <path>` writes. | The working directory of the session for `/chat share`; the path given for `/export-session`. |

Details and sources are in
[Other stores in `research/openclaw.md`](../../research/openclaw.md#other-stores),
[Parser plan in `research/codex-cli.md`](../../research/codex-cli.md#parser-plan),
[Other stores in `research/continue.md`](../../research/continue.md#other-stores) and
[Parser plan in `research/gemini-cli.md`](../../research/gemini-cli.md#parser-plan).

## Agent frameworks and self-hosted platforms

Agent frameworks are libraries inside someone else's application, so most
write nothing to a fixed place in the home; the catalog covers the ones
that do (AutoGen Studio, CrewAI, CAMEL, MetaGPT, the Pydantic AI CLIs, the
legacy Open Interpreter Python tool) and the self-hosted platforms Dify,
Flowise, Langflow and n8n, in the home and in their Docker volumes (see
[docker.md](docker.md)). The sweep behind this, with the reason each
framework got a catalog line or not, is
[research/frameworks.md](../../research/frameworks.md).

Some of their state has no fixed home-relative name and is not collected.
Copy it by hand when the framework was in use:

| Framework | State not collected | Where to look |
| --- | --- | --- |
| TaskWeaver | Session transcripts and prompt logs (`workspace/sessions/<id>/`), logs, and `taskweaver_config.json`, which holds the LLM API key. | The project directory: the directory holding `taskweaver_config.json`, which TaskWeaver finds by walking up from where it was started. Nothing under the home records it. |
| LangGraph | `langgraph dev` runs, threads and checkpoints, as Python pickles in `.langgraph_api/`; `langgraph up` keeps them in PostgreSQL. | `.langgraph_api/` in the project directory; the Docker volume `<project dir name>_langgraph-data`, a PostgreSQL data directory, which is recorded as `skipped_unmatched_volume`. Never unpickle the files to read them. |
| Dify | The PostgreSQL database with conversations, messages and agent tool calls, uploaded files, tenant RSA keys and `docker/.env` (`SECRET_KEY`). | The bind mounts under `docker/volumes/` beside Dify's compose file, in wherever Dify was cloned. Only its two agent sandbox named volumes and the `difyctl` config are collected. |
| Langflow (pip or uv install) | `langflow.db` (`langflow-pre.db` for a pre-release), with messages, run history and encrypted credentials. | The installed package directory, `site-packages/langflow/`, inside the virtual environment Langflow runs from, unless the `database_url` or `save_db_in_config_dir` setting moved it (the second puts it in the collected config directory). Langflow Desktop's database is under `.langflow/data` or `AppData/Roaming/com.LangflowDesktop` and is collected. |
| CrewAI (Linux and macOS) | Opt-in agent memory (`memory/`) and knowledge stores (Chroma, `qdrant/`). | The directory that holds a collected `latest_kickoff_task_outputs.db`, which is named after the project folder (`~/.local/share/<project>`, `~/Library/Application Support/<project>`). Only CrewAI's own files in it are collected, because the directory name is not fixed. On Windows the whole `AppData/Local/CrewAI` directory is collected. |
| CrewAI (project files) | A project's `.crewai/` (`last_run.json`, the mark that a run's trace reached AMP, CrewAI's hosted service; `last_eval.json`; `eval_project_state.json`) when no other source names the project, and its opt-in `.checkpoints/` always. | The directory a crew was run from. CrewAI records no project path, so `.crewai/` is collected only from a project that another agent's state or `-p` names; `.checkpoints/` is too generic a name for the project catalog. |
| IntentKit | Agent state in PostgreSQL, Redis and RustFS. It is not in the catalog, because it runs no tools on the host. | Its Compose named volumes, such as `intentkit_postgres_data`, recorded as `skipped_unmatched_volume`, and `.env` in its checkout. |

## Remote development servers

Remote development state is collected: `~/.vscode-server`,
`~/.cursor-server`, `~/.windsurf-server`, `~/.devin-server` and
`~/.kiro-server` keep the same `User` layout as the desktop editor, on the
WSL or SSH host rather than the workstation.

## Host state is not collected

The collectors take files from disk only. Running processes, process
environments, logged-in users, network connections, services and scheduled
tasks are not captured: the collectors supplement an EDR, which already
records that host state. Collectors before 1.6.0 wrote a `live/` snapshot of
it; see [CHANGELOG.md](../CHANGELOG.md). The host name is in the `hostname`
field of `collection.json`.

## Default exclusions

Large, low-value subtrees are skipped by default but still recorded in the
manifest with status `skipped_excluded` and their on-disk size, so the
investigator knows what was there. Pass `--full` to collect them. Skipped
by default:

- model weights
- Electron and editor caches
- extension and daemon binaries
- embedding indexes
- shadow git checkpoints
- agent worktrees, in a home and inside a project: `.claude/worktrees`,
  `.codex/worktrees`, `.gemini/worktrees` (Gemini CLI's `--worktree`),
  `.qwen/worktrees` (Qwen Code's `enter_worktree`), `.kilo/worktrees` and
  the others `--list` prints
- git clones of plugin marketplaces
- Claude Desktop's Cowork VM disk bundle (see [Cowork VM disks](#cowork-vm-disks))

Ollama's model weights are excluded as `.ollama/models/blobs` only, so the
rest of `.ollama/models` is collected: the small JSON manifests under
`models/manifests/<registry>/<namespace>/<model>/<tag>`, whose names and
pre-copy times record which models were pulled and when, and the per-blob
metadata files under `models/metadata`. In a Docker volume the exclusion
is the volume-relative `models`, which also covers Tabby, Hermes and
Local Deep Research volumes, where `models` holds model weights. It stays
whole, so a Docker Ollama volume's `models/manifests` is not collected;
when the pulled models matter, copy `models/manifests` from the volume's
`_data` by hand, or pass `--full` (`-Full`), which also takes the weights.

Hermes' quick snapshots under `state-snapshots/<id>/` are collected:
each holds an older complete copy of `state.db`, which may be the only
record of a session Hermes later pruned or deleted, and Hermes keeps at
most 20 of them (one after a `hermes update`), so their size is bounded by
the database's; a copy larger than `--max-file-size` gets a
`skipped_size` row as any file does. The `.env`, `auth.json` and pairing
files inside a snapshot are flagged as secrets, as their originals are,
also in a Hermes Docker or Podman volume. Hermes' zip backups under
`backups/` stay excluded. Before collector 1.18.0 the snapshots were
excluded too and needed `--full`. The facts are in
[Transcript stores in `research/hermes.md`](../../research/hermes.md#transcript-stores).

Claude Code's `.claude/paste-cache/` is collected from collector 1.19.0;
before it was excluded and needed `--full`. It holds the text of each
long pasted prompt as `<hash>.txt`, which `history.jsonl` names only by
that hash, so once the transcript is gone it is the only copy of the
text. `.claude/plugins/plugin-directory-cache*`, a downloaded copy of the
plugin directory (2.6 MB on the Windows install observed in October
2026), is excluded from 1.19.0.

OpenClaw's exclusions name the top-level directories of `~/.openclaw` and
the legacy `~/.clawdbot` and `~/.moltbot`: `dev`, `git`, `npm`,
`npm-runtime`, `tmp` and `tools` (managed checkouts, packages, temporary
files and downloaded runtimes, the directories `openclaw backup` also
skips), `worktrees`, `plugin-skills`, `sandbox/skills-workspaces` and
`cache/control-ui-assets`. Each matches only the top-level directory, so a
`tmp` or `tools` directory inside an agent workspace (`workspace/`,
`workspace-<agentId>/`) or a skill is collected. A profile's
`~/.openclaw-<profile>` has no `dev`, `git`, `npm`, `npm-runtime`, `tmp`,
`tools` or `worktrees` exclusion, because the `*` of `.openclaw-*` crosses
`/` and such a line would also prune those names inside the profile's
workspace: they are collected, however large. Its other exclusions (the
clones, `plugin-skills`, `sandbox/skills-workspaces`,
`cache/control-ui-assets`, `extensions/*/node_modules`, the agent temporary
trees and the Chromium caches) still match at any depth below the profile
directory. The Gateway's clones of repositories
added from a remote URL, `projects/<16 hex>/<name>`, are excluded by the
clone directory's name alone, so anything else under `projects/` is
collected; which repository each clone is stays in the `projects` table of
the collected `state/openclaw.sqlite`.

## Credentials

Credential files are collected by default and flagged `secret: true`, since
they show which account an agent acted as. Use `--no-secrets` to leave them
out; they are then recorded with status `skipped_secret`. Run `--list` for
the full pattern list.

- **OAuth tokens and keys:** `~/.claude/.credentials.json`,
  `~/.codex/auth.json`, `~/.gemini/gemini-credentials.json`,
  `~/.cursor/auth.json`, `~/.copilot/config.json`,
  `~/.cline/data/settings/providers.json`,
  `~/.local/share/opencode/auth.json`, OpenCode's `service.json` (the
  background server's password) in `~/.config/opencode` and
  `~/.local/state/opencode`, from collector 1.21.0 also their temporary
  `service.json.<pid>.tmp` and `service.json.<id>.tmp` copies, the terminal
  handoff `service.json.pty-handoff` and its temporary copy, the terminal daemon's
  `~/.local/state/opencode/pty/<instance id>/service.json` (a socket token),
  `~/.config/goose/secrets.yaml`,
  `~/.factory/auth.v2.*`, ...
- **Config files that commonly embed API keys or MCP server environments:**
  Continue's `config.yaml`, Zed's `settings.json`, Crush's `crush.json`,
  Claude Code's `.claude.json`, Claude Desktop's
  `claude_desktop_config.json`, Kiro powers' `mcp.json`; from collector
  1.20.0 also Claude Code's and Gemini CLI's `settings.json` in the home and
  in a project, a project's `.mcp.json`, `.vscode/mcp.json` and root
  `opencode.json`, Hermes' `config.yaml` in a home, profile or volume, the
  MCP settings files of Antigravity, Cline and little-coder in the home and
  Langflow's in the home and in a volume, those of Amp, Roo Code, Factory
  Droid, Kiro, Copilot CLI and CLAI in the home and in a project, a
  project's Crush `crushrc`, `.crushrc`, `crush.json` and `.crush.json`, and
  Ollama's copies of other agents' settings files (`backup/` and the Muse
  `settings.json` copy) in the home and in a volume; from collector 1.21.0
  also OpenCode's `~/.config/opencode/cli.json`, whose terminal UI plugin
  options can hold a key.
  `settings.json` copy) in the home and in a volume; Goose's `config.yaml`,
  and from collector 1.21.0 its `config.tmp` save copy, the
  `mcp-config-*.json` files of its Claude Code provider and the
  `diagnostics_<YYYYMMDD>_<n>.json` export in a project, which holds
  `config.yaml` up to 256 KiB.
- **Desktop backend secrets:** the Goose desktop app's `settings.json`,
  which holds the shared secret of an external backend server (from
  collector 1.21.0).
- **Electron `Local State` files** that hold the key for encrypted token
  caches.
- **Desktop connection files:** the Hermes desktop app's `connection.json`
  and `connections.json`, which hold a remote gateway's token, encrypted
  with the operating system's key store or in plain text when that
  encryption is turned off.
- **Qwen Code's daemon and managed local mode:** the Agent Host registration
  `~/.qwen/agent-hosts/<sha256>.json` (a host id and secret), the managed
  local mode's `~/.qwen/managed-local/state.json` (its service tokens and
  signing keys) and the settings, token and session registry files its
  worker processes write under `~/.qwen/managed-local/worker-home/`, and
  the `settings.json.corrupted` and `settings.json.write-<random>/settings.json.orig`
  copies Qwen Code leaves beside a settings file. A managed data directory
  moved with `qwen serve --managed-data-dir` outside `~/.qwen` is not
  collected.
- **Running-session registries:** Qwen Code's `~/.qwen/sessions/<pid>.json`
  (or `<pid>-<8 hex>.json`), one per running session, whose `ipcToken`
  admits a connection to that session's socket; the file is deleted when
  the session exits, so only a collection taken while one runs, or after a
  crash, holds it.
- **Claude Code's session daemon and `.claude.json` backups**, flagged from
  collector 1.19.0: `.claude/daemon/control.key` and `pipe.key`, raw keys
  for the local connection to the daemon that keeps sessions running in
  the background, and `.claude/daemon/roster.json`, which holds each
  worker's terminal and attach auth values; and
  `.claude/backups/.claude.json.backup.<epoch ms>`, the dated copies of
  `.claude.json` Claude Code writes at start and exit, and the
  `.claude.json.corrupted.<timestamp>` copy it keeps of a config it could
  not parse (the glob is `.claude/backups/.claude.json.*`), each with the same
  account and MCP server settings as the flagged `.claude.json`, as do the
  older `.claude.json.backup*` files beside `.claude.json` in the home or
  a `~/.claude-<name>` config home.
- **Claude Code's connection files for a running process**, flagged from
  collector 1.19.0, in the home and in a `~/.claude-<name>` config home:
  `.claude/ide/<port>.lock`, which an editor extension's session writes
  with the `authToken` an editor needs to connect to it, and
  `.claude/bridge-spawn/<id>/mcp.json`, which a Remote Control bridge
  (`claude remote-control`, which lets the user start and drive sessions
  on the host from claude.ai or the Claude mobile app) writes with the
  address and session-bound
  headers of its remote MCP server. Each is removed when its process
  exits, so only a collection taken while one runs, or after a crash,
  holds it (observed with 2.1.310 on macOS 26.6, October 2026).
- **Databases flagged whole:** Kilo Code's `kilo.db` and OpenCode's
  `opencode.db`, which embed tokens in their `account` and `credential`
  tables.

On macOS, Claude Code keeps its sign-in in the login keychain, as a
generic password with service `Claude Code-credentials` and the macOS
user name as account, when the CLI is started from a terminal in the
logged-in desktop session, and then writes no `.credentials.json`. The
collector does not read the keychain, so the collection of such a user
holds no Claude Code token, only the account details in `.claude.json`
and its backups. A CLI started over ssh cannot reach the login keychain
and writes `~/.claude/.credentials.json` instead; the two are not shared,
so a CLI started over ssh after a desktop sign-in is not signed in
(observed with 2.1.310 on macOS 26.6, October 2026).

The GitHub CLI's login store is attributed to the `shared` catalog agent, the cross-agent block,
and flagged as a credential (`secret: true`), since several agents (Claude Code's `/install-github-app` and pull-request status, Copilot) act on GitHub
through `gh`: `~/.config/gh/hosts.yml` on macOS and Linux, `AppData/Roaming/GitHub CLI/hosts.yml`
on Windows. The file names the GitHub login an agent acted as, which is what a responder needs
when work was pushed to a personal account; it holds the OAuth token only when `gh` could not
reach the OS credential store (a process started from an SSH session on macOS wrote a 218-byte
file with the token, a GUI-launched one a 94-byte entry without it, the token then being a
keychain item of service `gh:github.com`). `config.yml` beside it holds preferences only and is
not collected.

Cursor and Windsurf keep their auth tokens inside the same `state.vscdb`
that holds the chat history, so that file is collected unflagged; the keys
to redact are `cursorAuth/*` and `windsurfAuthStatus`, and in Cursor's
`ItemTable` also the `secret://` entries, the editor's secret storage,
which holds the agent worker's API key, and the `mcpOAuth.secret.*`
entries, which hold an MCP server's OAuth tokens, PKCE code verifier
(the one-time proof of a sign-in in progress) and client secret,
encrypted with the operating system's key store.
`ollama launch chatgpt` copies `~/.codex/auth.json`, the Codex
sign-in, into a scratch directory `ollama-codex-models-*` in the system
temp directory while it lists Codex's models, and removes it when the
command finishes. A crash or a kill leaves that copy outside every home,
where no catalog line reaches it; on a host where Ollama launched the
ChatGPT app, look for the directory in the temp directory and copy it by
hand (see [Credentials in
`research/codex-cli.md`](../../research/codex-cli.md#credentials)).
Agent Zero copies a Telegram bot's configuration into each Telegram
chat's `chat.json`, which is collected unflagged; the keys to redact are
`data.telegram_bot_cfg.token` and `data.telegram_bot_cfg.webhook_secret`,
and `api_key` inside `data.chat_model_override`, which a model chosen for
one chat from the A0 CLI connector can carry. Its WhatsApp bridge's login
state, `tmp/whatsapp/session/` of the install directory, is flagged
`secret: true`: whoever holds it can act as the account's linked device.
Roo Code releases up to 3.53.0 kept a retry queue of failed Roo Code Cloud
uploads in the extension's workspace state, the `RooVeterinaryInc.roo-cline`
row of an editor's `User/workspaceStorage/<hash>/state.vscdb` and the CLI's
`.vscode-mock/workspace-storage/<hash>/workspace-state.json`, both collected
unflagged; the key to redact is `roo.retryQueue[].options.headers.Authorization`,
a bearer token. The same releases' CLI saved its cloud token in
`~/.roo/cli-credentials.json`, which is flagged.
The VS Code family's `User/settings.json` holds every editor setting, so it
is collected unflagged; the keys to redact are `twinny.githubToken`, the
`env` values of `twinny.chatToolsMcpServers` entries and the `headers` and
`env` values of `cody.mcpServers` entries.
A project's `.git/config` is collected unflagged and can hold a token
inside a remote URL; see [Git metadata](#git-metadata).

## Pattern matching

Exclusion and credential patterns are matched relative to the directory
being collected, and `*` in them crosses `/`. An exclusion wins over a
credential pattern. The patterns are matched as patterns wherever the
collector is run from: they are never expanded against its working
directory.

- **Base:** a home, a discovered project or a Docker volume's `_data`. The
  same `.claude/worktrees` pattern therefore prunes both
  `~/.claude/worktrees` and a project's `.claude/worktrees`.
- **Matched paths:** a path a catalog entry matches is tested as well as
  every path under it, a file as well as a directory. A matched file that
  an exclusion covers (a file `.local/bin/.muse-update.<suffix>`, which
  the catalog line `.local/bin/.muse-*` and the exclusion
  `.local/bin/.muse-update.*` both match) gets one `skipped_excluded` row
  with its size, and the inventory does not count it.
- **Case:** the sh collector matches with `case` and `find -path`, which are
  case-sensitive. The PowerShell collector converts the patterns to regular
  expressions and matches them, and the catalog globs, without regard to
  case.
- **Classes:** `[...]` classes and `[!...]` negated classes work in both
  collectors, in catalog globs too: CrewAI's Windows directory is the
  single line `AppData/Local/[Cc]rew[Aa][Ii]`, which matches `CrewAI` and
  `crewai` on a case-sensitive filesystem without two lines that differ
  only by case.

An excluded directory is pruned from the walk before any file in it is
looked at, so a credential file inside it is never collected or flagged; it
is covered only by the directory's `skipped_excluded` row. Exclusions are
therefore written so that they do not cover credential files: Hermes'
installer checkout is excluded entry by entry (`.hermes/hermes-agent/[!.]*`
and `.hermes/hermes-agent/.[!e]*`) so that `hermes-agent/.env` is still
collected.
