# cursor

Cursor is a closed-source editor forked from VS Code (the Cursor IDE) with a command-line agent, the
Cursor Agent CLI (`agent`, `cursor-agent`), a copy of which the IDE embeds and runs as its agent
worker (on Linux downloaded only when a cloud or remote-host feature is first used). The document
uses Cursor's own names: a composer is one conversation, identified by its `composerId` (a UUID); a
bubble is one message, reasoning block or tool call inside a composer, identified by its `bubbleId`;
the Agents window is the IDE's window given over to agent conversations, and the editor window is
the VS Code-style window with its own chat pane and chat history (on 3.26.13, 2026-10-20, only the
editor window offered Export and Delete); `glass` is the name the IDE's bundle and storage keys give
that agent user interface (`workbench.glass.main.js`, the `cursor/glass.*` keys); `agentKv` is the
key prefix of a content-addressed blob store inside the IDE's database; `yolo` is Cursor's internal
name for auto-run, the setting that runs agent tool calls without asking (`yoloCommandAllowlist`);
and `toolFormer` is its internal name for a tool call's record inside a bubble (`toolFormerData`).
A cloud agent is an agent conversation that runs on a virtual machine of Cursor's against a
repository, not on the host; Cursor's internal name for it is background composer, so its id is
`bc-<uuid>` and its keys carry `bc` or `backgroundComposer`. Cursor Origin is Cursor's own git
hosting service, into which a GitHub repository is mirrored before a cloud agent can use it
([Source-host integration](#source-host-integration)). The analyzer parses the IDE's conversations
([issue 51](https://github.com/seanthegeek/doubleagent/issues/51)) and the files a CLI session
shares with the IDE; the Agent CLI's own chat store is not parsed yet
([issue 235](https://github.com/seanthegeek/doubleagent/issues/235)).

## Source and evidence

Closed source. Evidence, strongest first; the real installs are the evidence for the IDE and for
the CLI's macOS layout, the shipped bundle for the CLI features and platforms no install has
shown:

- Real installs ([Observed](#observed)): the Cursor IDE on macOS, builds 3.24.18, 3.25.9 and
  3.26.13 (2026-10-19 and 2026-10-20), and on Ubuntu (arm64), builds 3.24.18 and 3.25.9 of the
  deb package (2026-10-20); on Windows 11 (arm64), build 3.26.13 of the per-user installer
  (2026-10-21); the standalone Cursor Agent CLI on macOS, build 2026.10.19-6bbf2c4
  (2026-10-20). Read for file names, sizes, key, table and column names,
  value types and enum values only; no value from the install is reproduced, and account ids are
  written `u<id>` and `auth0|user_<id>`, CLI session ids `<sessionId>`.
- L1 shipped bundles. The IDE: Cursor 3.25.9 and 3.26.13 for macOS,
  `Cursor.app/Contents/Resources/app/out/vs/workbench/workbench.glass.main.js`, grepped for
  literal strings, cited below as `workbench.glass.main.js` with the snippet (byte offsets were
  not recorded). The CLI: Cursor Agent CLI `2026.10.01-e373342`, `agent-cli-package.tar.gz` from
  `downloads.cursor.com/lab/.../linux/x64/` (the URL is in the install script
  `https://cursor.com/install`, lines 78-131). Minified webpack chunks keep module names, so
  citations are `<chunk>.js` plus the original module path. The real install ran the newer
  2026.10.19-6bbf2c4, the build the IDE's worker directory held.
- L2 official docs: [Cursor CLI configuration](https://cursor.com/docs/cli/reference/configuration),
  [Cursor CLI parameters](https://cursor.com/docs/cli/reference/parameters) (fetched 2026-10-20),
  [Cursor Origin](https://cursor.com/docs/origin) (fetched 2026-10-21),
  [Cursor rules](https://cursor.com/docs/context/rules),
  [Cursor MCP](https://cursor.com/docs/context/mcp),
  [Cursor hooks](https://cursor.com/docs/agent/hooks),
  [Cursor ignore files](https://cursor.com/docs/context/ignore-files).
- L3 forum.cursor.com [thread 2825](https://forum.cursor.com/t/2825),
  [thread 167641](https://forum.cursor.com/t/167641),
  [thread 169582](https://forum.cursor.com/t/169582),
  [thread 172127](https://forum.cursor.com/t/172127),
  [How to delete a chat in Agent Window](https://forum.cursor.com/t/how-to-delete-a-chat-in-agent-window/163380/5)
  (fetched 2026-10-20).
- L4 write-ups and tools: [Callum-Ward/cursaves](https://github.com/Callum-Ward/cursaves)
  `docs/how-cursor-stores-chats.md`, extracurricular.ai (2026-09-09),
  [getagentseal/codeburn issue 986](https://github.com/getagentseal/codeburn/issues/986),
  [Cosmos-0118/AgentSweep](https://github.com/Cosmos-0118/AgentSweep) `rules/rules.toml`,
  [ShayGus/quota PR 20](https://github.com/ShayGus/quota/pull/20).
- The npm packages `cursor-agent` and `cursor-cli` are unrelated third-party projects (`npm view`),
  not Cursor's CLI.

### Observed

- macOS, Cursor IDE, 2026-10-19 and 2026-10-20
  ([issue 6](https://github.com/seanthegeek/doubleagent/issues/6) and
  [issue 51](https://github.com/seanthegeek/doubleagent/issues/51)): `Cursor.app`, bundle id
  `com.todesktop.230313mzl4w4u92`, 3.24.18 at first launch, 3.25.9 after an in-app update and
  3.26.13 after a second, in the author's macOS lab VM (macOS 26.6.2, Darwin 25.6.0, arm64), one
  user, one project (a git repository with one commit, `README.md` and `calc.py`), the agent pane
  of the Agents window and of the editor window. One step per listing, each diffed against the
  last (times UTC): the first launch, not signed in (2026-10-19 01:00Z), wrote the Electron
  profile of [Cursor IDE](#cursor-ide) and the `~/.cursor` entries of
  [`~/.cursor` as the IDE writes it](#cursor-as-the-ide-writes-it); sign-in (01:08Z) wrote the
  keys of [Credentials](#credentials), the built-in skills and the account's sync store
  ([AgentStores](#agentstores)); the in-app update to 3.25.9 (01:13Z) wrote the first
  [`update-supervisor`](#update-supervisor) directory; opening the project folder (01:16Z, with no
  workspace-trust prompt) wrote its workspace state, `~/.cursor/projects/<sanitized cwd>/`, the
  embedded Agent CLI ([Exclusions](#exclusions)) and a code index inside the repository's `.git`
  ([Project-local files](#project-local-files)); a question with no tool use (01:20Z) wrote the
  first composer ([Record schema](#record-schema)) and its JSONL copy
  ([Agent transcript JSONL](#agent-transcript-jsonl)); a code edit of `calc.py` (01:25Z), applied
  with no prompt, wrote tool bubbles, file-state rows, a retrieval checkpoint and tracking rows
  ([Other stores](#other-stores)); a shell command (01:31Z) ran with no prompt, because a new
  install defaults to the Auto-review run mode
  ([Run mode and standing lists](#run-mode-and-standing-lists)); switching the run mode to
  Allowlist (with Sandbox) (01:56Z) changed two settings; three `curl` commands under it were run
  (02:00Z), skipped (02:04Z) and allowed always (02:06Z), each recording its decision in its tool
  bubble ([Tool bubbles](#tool-bubbles)), the last adding two standing allowlist entries; a plan
  upgrade from free to Pro with an account merge (about 16:14Z to 16:38Z on 2026-10-20, two
  relaunches) and a model switch from `grok-4.6` to `grok-4.7` with no message sent (16:55Z) fell
  in one listing and were mapped by modification time; a question after the switch (17:39Z)
  recorded the new model on its user bubble ([Model rows](#model-rows)); a quit, two relaunches
  and the conversation reopened (17:44Z to 17:48Z) wrote nothing to the conversation
  ([Joins and timestamps](#joins-and-timestamps)); the update to 3.26.13 (17:57Z to 17:58Z) and
  a new conversation with one file read (created 17:56Z, answered 17:59Z) showed the layout of
  [Format versions](#format-versions); archiving the first conversation (18:09Z), exporting it
  to Markdown from the editor window (by 19:02Z), finding no Share there, and deleting it from
  the editor window (19:04Z) are in [What survives a delete](#what-survives-a-delete). The Agents
  window did not render the replies of 17:39Z and 17:59Z, which the store held whole and the
  editor window showed.
- macOS, Cursor Agent CLI 2026.10.19-6bbf2c4, 2026-10-20 ([issue 235](https://github.com/seanthegeek/doubleagent/issues/235)): `agent` and `cursor-agent`,
  the build the IDE's worker directory held, installed with `curl https://cursor.com/install -fsS
  | bash` in the same VM reverted to its clean snapshot (no `Cursor.app`), signed in to the IDE
  round's account (Pro), with the project recreated with one commit. One step per listing, each
  diffed against the last (times UTC): the install and a first start from the home directory,
  not signed in (about 20:05Z), wrote the install directory, `cli-config.json` and an empty
  project entry ([Cursor Agent CLI](#cursor-agent-cli)) and the logs of
  [Scratch directories](#scratch-directories); `/login` in the CLI (20:12Z) wrote the keychain
  items and the identity of [Credentials](#credentials), and the user quit at the
  workspace-trust prompt the CLI showed for the home directory; a start in the project (20:15Z),
  whose trust prompt the user accepted, wrote the trust record
  ([CLI session and trust files](#cli-session-and-trust-files)), a session with no conversation
  and the tracking database; a question with no tool use (20:17Z) wrote a second session with its
  chat store ([CLI chat store](#cli-chat-store)) and a transcript JSONL
  ([Agent transcript JSONL](#agent-transcript-jsonl)); a code edit of `calc.py` (20:21Z),
  applied with no prompt, wrote tool messages and tracking rows
  ([`ai-code-tracking.db`](#ai-code-trackingdb)); a shell command (20:27Z) waited at a
  four-option prompt, answered Allow once (20:33Z), then Skip with a typed instruction (20:36Z)
  and Add to the allow list (20:41Z), the last adding a standing entry
  ([Permission rows](#permission-rows)); a `/model` switch with no message sent (20:43Z) changed
  only `cli-config.json`, and a question after it (20:47Z) recorded the new model
  ([Model rows](#model-rows)); a quit and `agent resume` (20:49Z) wrote nothing to the session
  ([Joins and timestamps](#joins-and-timestamps)); `agent logout` (20:52Z) and the removal of the
  project directory (20:53Z) are in [What survives a delete](#what-survives-a-delete). Nothing
  was written inside the project but the edited file.
- Linux, Cursor IDE, 2026-10-20 ([issue 236](https://github.com/seanthegeek/doubleagent/issues/236)):
  the `cursor` arm64 deb package `3.24.18-1791917266`, upgraded through apt to
  `3.25.9-1792103615`, in the author's Ubuntu 26.x lab VM (Linux 7.0, aarch64, GNOME), one user,
  the macOS round's project (a git repository with one commit, `README.md` and `calc.py`), the
  agent pane of the editor window; the Agents window was not used. One step per listing, each
  diffed against the last (times UTC, the listing's): installing the package (21:19Z) wrote the
  host-level files of [Cursor IDE on Linux](#cursor-ide-on-linux) and nothing under the home;
  the first launch, not signed in (21:25Z), wrote `~/.config/Cursor`, the `~/.cursor` entries
  and the GNOME keyring file; sign-in through the browser (21:32Z), with no keyring prompt, wrote
  the plaintext tokens of [Credentials](#credentials) and the account's sync store under
  `~/.local/state` ([AgentStores](#agentstores)); opening the project folder (with no
  workspace-trust prompt) and the apt upgrade with a relaunch fell in one listing (21:38Z) and
  were split by modification time: the workspace state, the `projects/` entries of
  [`~/.cursor` as the IDE writes it](#cursor-as-the-ide-writes-it) and the repository's code
  index ([Project-local files](#project-local-files)), and no `update-supervisor` directory; a
  question with no tool use (21:42Z) wrote the first composer with its header list inline, its
  transcript JSONL and its sync store, and no Agent CLI copy ([Exclusions](#exclusions)); a
  code edit of `calc.py` (21:44Z), applied with no prompt, wrote a shadow copy under another
  extension directory than on macOS ([Retrieval checkpoints](#retrieval-checkpoints)); a shell
  command (21:47Z) ran with no prompt under the Auto-review default; switching the run mode to
  Allowlist, which has no sandbox variant on Linux
  ([Run mode and standing lists](#run-mode-and-standing-lists)), and sending a command (21:54Z)
  left the call waiting at its prompt with its record already on disk
  ([Tool bubbles](#tool-bubbles)); the prompt was answered Run (21:56Z), and two more commands
  Skip (21:58Z) and "add python3 to the allow list" (22:01Z), the last never reaching the
  standing list; a model switch from Auto to `grok-4.6` and a question (22:07Z) recorded the
  model on the user bubble ([Model rows](#model-rows)); a quit, a relaunch and the conversation
  reopened (22:25Z) wrote nothing to the conversation
  ([Joins and timestamps](#joins-and-timestamps)); adding `python3` in Settings > Agent and
  running the command again (22:31Z) stored the standing entry; archiving (22:36Z), exporting to
  Markdown (22:40Z) and deleting (22:46Z), all from the editor window's chat history, are in
  [What survives a delete](#what-survives-a-delete). Connecting Cursor's GitHub integration to a
  throwaway GitHub account, its GitHub app installed with access to the account's repositories
  (23:09Z), wrote the keys of [Source-host integration](#source-host-integration) and named no
  account; the editor's own GitHub sign-in, through Welcome > Clone from GitHub with the clone
  cancelled (23:20Z; Cursor's Accounts menu has no GitHub entry), named the account in key names
  and stored its encrypted token ([Secrets](#secrets)), and the same listing showed the machine
  enrolled as a remote agent host, written at a time between the two listings that is not known;
  a cloud agent attempted (23:40Z) found no repository in its picker, although the app had access
  to all of them and making the repository public changed nothing, so no cloud conversation was
  created, and it started a download of the Agent CLI copy that was left as two partial tarballs
  ([Cursor IDE on Linux](#cursor-ide-on-linux)). The repository-bound remote session was
  therefore not observed on Linux; the Windows round explained the empty picker and observed it
  ([Source-host integration](#source-host-integration)).
- Windows 11 arm64 (10.0.26300), Cursor IDE 3.26.13 per-user installer, 2026-10-21
  ([issue 236](https://github.com/seanthegeek/doubleagent/issues/236)): in the author's Windows
  lab VM, one user, a project folder `%USERPROFILE%\proj\demo` with `README.md` and `calc.py`
  that was not a git repository, because git was not installed until the step of 13:55Z, so the
  round shows no git-derived field and no code index for it; the agent pane of the editor window,
  and the Agents window for the cloud agent. One step per listing, each diffed against the last
  (times UTC, the listing's): the install and first launch, not signed in (00:45Z), wrote the
  trees of [Cursor IDE on Windows](#cursor-ide-on-windows) and the `~/.cursor` entries of
  [`~/.cursor` as the IDE writes it](#cursor-as-the-ide-writes-it); sign-in through the browser
  (00:50Z), with no credential prompt, wrote the plaintext tokens of
  [Credentials](#credentials) and the account's sync store under `%LOCALAPPDATA%`
  ([AgentStores](#agentstores)); an in-app update (00:56Z) relaunched the app and left the build
  at 3.26.13, writing only an installer log ([`update-supervisor`](#update-supervisor));
  opening the folder (01:10Z), with no workspace-trust prompt, wrote its workspace state and its
  `projects\` entry with the first terminal capture of any round; a question with no tool use
  (01:12Z) wrote the 3.26.13 layout of [Format versions](#format-versions); a code edit of
  `calc.py` (01:15Z), applied with no prompt, wrote a shadow copy
  ([Retrieval checkpoints](#retrieval-checkpoints)); a request for a Python command (01:40Z) on
  a VM without Python made the agent run several commands under the Auto-review default, one of
  them failing ([Tool bubbles](#tool-bubbles)); switching the run mode to Allowlist and sending a
  PowerShell command (01:58Z) left the call waiting at a prompt that offered no allow-list entry,
  and it was answered Run (02:02Z) and, for the same command, Skip (02:08Z); adding the command
  in Settings > Agent (02:10Z) stored the standing entry
  ([Run mode and standing lists](#run-mode-and-standing-lists)); a model switch from Auto to
  `grok-4.6` (02:12Z) and a question after it (02:14Z) are in [Model rows](#model-rows), with
  the agent's injected user turns of [User bubbles](#user-bubbles); a quit, a relaunch and the
  conversation reopened (02:20Z) wrote nothing to the conversation
  ([Joins and timestamps](#joins-and-timestamps)) and removed the terminal captures; exporting
  from the editor window's history (02:25Z; the archive step was skipped) and deleting from it
  (02:28Z) are in [Exported Markdown](#exported-markdown) and
  [What survives a delete](#what-survives-a-delete). Enabling Cursor Origin, mirroring the demo
  repository into it from GitHub at cursor.com, and running a cloud agent on it with a read-only
  task fell in one listing (13:10Z); a quit, a relaunch and the cloud conversation reopened
  (13:21Z), and archiving the cloud agent from the Agents window (13:36Z) are in
  [Cloud agents](#cloud-agents) and [Source-host integration](#source-host-integration); the
  same listing of 13:36Z held what the Agents window created by itself (a new draft in a
  workspace project, two temporary projects, and the first extraction of the embedded Agent CLI,
  which failed to start on arm64; [Cursor IDE on Windows](#cursor-ide-on-windows)). Installing
  Git for Windows 2.56.0.windows.2 and opening the Welcome tab's "Steer from iOS" modal (13:55Z)
  wrote one key ([`ItemTable` keys](#itemtable-keys)); a relaunch, Clone a repository > Clone
  from GitHub with the editor's own GitHub sign-in through the browser, and the clone opened
  (14:03Z) wrote the keys of [Secrets](#secrets) and an empty code index
  ([Project-local files](#project-local-files)). Settings > Cursor Settings showed agent remote
  control on by default with no paired device; pairing the Cursor iOS app to the install through
  "Steer from iOS" (14:31Z, diffed against a baseline listing of 14:18Z) wrote nothing locally,
  and a turn driven from the paired iPhone (14:35Z, closed by the listing of 14:44Z) started the
  embedded agent worker, which exited at once because it has no win32/arm64 native module, so
  the phone could not connect ([Remote control](#remote-control)). The steered turn is a feature
  this VM could not run, for that cause: the worker ran on macOS arm64 and Linux arm64 in the
  earlier rounds, so remote steering from a paired device is an open cell for the macOS VM.
  Installing the Google Calendar plugin from the plugin library (14:58Z), from the Agents
  window with no folder open and before any Google authentication, wrote a marketplace copy, a
  per-project tool manifest, an OAuth attempt record and the OAuth keys of
  [MCP servers and plugins](#mcp-servers-and-plugins); completing the plugin's Google
  authentication through the browser (15:05Z) stored its tokens as ciphertext in `state.vscdb`
  ([Secrets](#secrets)) and replaced the manifest's one tool with the server's nine; the
  plugin's "Try in chat" button, a chat the user stopped and archived, and then the user's own
  chat from the Agents window, which the records show ran as a second cloud agent in a new
  temporary Origin repository and whose request to authorise the MCP server the user accepted
  with a separate Google consent, fell in one listing (15:15Z;
  [Cloud agents](#cloud-agents)); the same calendar question in the demo folder's local chat
  (15:35Z) prompted for each MCP call under Allowlist, each approved, and for none under
  Auto-review, and left no per-call decision ([Permission rows](#permission-rows)). An update
  the app offered (15:56Z) had already been installed in the background, and the restart that
  applied it (16:06Z), the last step of the round, brought the build to 3.26.16 and wrote the
  install's first `update-supervisor` directory ([`update-supervisor`](#update-supervisor));
  3.26.16 itself was not surveyed. No step wrote a Windows Credential Manager entry, a
  scheduled task or a Run key.
- Cursor Agent CLI features the macOS round did not exercise (the `acp-sessions` store,
  `agent worker`, worktrees, plans, hooks, MCP servers (Model Context Protocol servers, which
  give the agent the tools of another service), print mode `-p`), any platform: not
  observed on any real install; bundle-derived ([Cursor Agent CLI](#cursor-agent-cli)).
- Worktree agents, any platform: not observed on any real install. Cloud agents were observed
  on Windows only (3.26.13, 2026-10-21); the attempt on Linux offered no repository.
- The Agent CLI on Windows and on Linux: not observed on any real install
  ([issue 236](https://github.com/seanthegeek/doubleagent/issues/236)). The RPM and AppImage
  builds for Linux and the x64 deb were not installed, and on Windows only the arm64 per-user
  installer was.

### Source reviews

- 2026-10-21: the [Cursor Origin](https://cursor.com/docs/origin) page (L2), for the cloud
  agent's repository list. The document gained Cursor Origin and the reason the Linux round's
  repository picker was empty.
- 2026-10-20: the [Cursor CLI parameters](https://cursor.com/docs/cli/reference/parameters)
  reference (L2), for the CLI's commands. The document gained the command list and the absence of
  a delete, clear, export or share command, and the CLI's half was rewritten from the real
  install.
- 2026-10-20: the IDE bundles 3.25.9 and 3.26.13 for macOS (`workbench.glass.main.js`, the run
  modes, the permission files and the delete commands) and the forum thread
  [How to delete a chat in Agent Window](https://forum.cursor.com/t/how-to-delete-a-chat-in-agent-window/163380/5).
  The document gained the run-mode table, the `permissions.json` and `sandbox.json` file names,
  and the delete surfaces, and the IDE's half was rewritten from the real install.

## Collection

### Per-user storage

The IDE layout below is the macOS install's ([Observed](#observed)); the Linux and Windows
installs differ in the places listed in [Cursor IDE on Linux](#cursor-ide-on-linux) and
[Cursor IDE on Windows](#cursor-ide-on-windows). The CLI's
`auth.json` location differs per OS ([Credentials](#credentials)).

#### Scratch directories

`~/.cursor/worktrees/<repo>/<name>`, the CLI's git worktrees (see
[Cursor Agent CLI](#cursor-agent-cli); excluded in [Exclusions](#exclusions)).

Outside the home, under `$TMPDIR` (`/private/var/folders/.../T/` on macOS), which no catalog line
reaches ([`collectors/docs/coverage.md`](../collectors/docs/coverage.md)):

- `cursor-agent-logs-<uid>/`: the logs of the IDE's worker and of the standalone CLI, one
  `session-<iso time>-<pid>-<n>.log` per process (mode 0600; the CLI's interactive process and
  its worker each write one, with a second, often empty, `-2.log` beside it, and every `agent`
  command such as `agent logout` writes its own) and a `latest.log` symlink to the newest. The
  IDE's were gone between 18:11Z and 19:02Z on 2026-10-20 with no user action, so they are not a
  durable source.
- `cursor-agent-worker-<hash>.sock`: the worker's socket.
- `<user>-cursor-zsh/`: copies of the user's `.zlogin`, `.zprofile`, `.zshenv` and `.zshrc`
  (0.6 to 10 KB each) for the IDE's zsh shell integration, seen on 2026-10-20.

Outside the home under `/private/tmp`: `cursor-askpass-<hex>-<n>.sock`, the CLI's git askpass
socket (0 bytes), a new one per start (2026.10.19-6bbf2c4, 2026-10-20).

The Linux IDE install (3.25.9, 2026-10-20) wrote none of these under `/tmp`: no
`cursor-agent-logs-<uid>/`, since no worker ran, and no copies of the
shell's rc files (the VM's shell was bash, and no bash counterpart of the zsh copies appeared).

On Windows (3.26.13, 2026-10-21) the Agents window opened two temporary folders,
`%LOCALAPPDATA%\Temp\<uuid>`, at 13:31Z and left neither behind; their `projects\` entries
remained ([`~/.cursor` as the IDE writes it](#cursor-as-the-ide-writes-it)). The updater's
`%LOCALAPPDATA%\Temp\cursor-inno-updater-<epoch s>.log` is in
[Cursor IDE on Windows](#cursor-ide-on-windows). `%LOCALAPPDATA%\Temp` is not collected.

#### Cursor IDE

IDE (VS Code fork; the generic layout is in
[User data directory of `research/vscode.md`](vscode.md#user-data-directory)):
`~/Library/Application Support/Cursor/User`, `%APPDATA%\Cursor\User`, `~/.config/Cursor/User` (forum
2825, 169582; cursaves; extracurricular). The chats live in `User/globalStorage/state.vscdb`
([Transcript stores](#transcript-stores)). The file is never vacuumed and reaches 30-125 GB
(forum 167641, 169582, 172127); `state.vscdb.backup` sits beside it.

What the macOS install held under `~/Library/Application Support/Cursor/`, with the catalog's
treatment:

| Path | Holds | Collected |
| --- | --- | --- |
| `User/globalStorage/state.vscdb` (+`-wal`, `-shm`) | the conversations, the settings and the tokens: tables `ItemTable`, `cursorDiskKV` and `composerHeaders` ([Record schema](#record-schema)); 2.9 MB before a quit checkpointed it to 2.2 MB | yes |
| `User/globalStorage/state.vscdb.backup` | a stale copy (forum) | excluded |
| `User/globalStorage/conversation-search.db` (+`-wal`, `-shm`) | the full-text index of the conversations ([`conversation-search.db`](#conversation-searchdb)) | yes |
| `User/globalStorage/storage.json` | VS Code window state (`windowsState`, `backupWorkspaces`, `lastKnownMenubarData`) and device ids; 12 KB, then 69 KB | yes |
| `User/globalStorage/anysphere.cursor-retrieval/checkpoints/<requestId>/` | shadow copies of the files an agent request edited ([Retrieval checkpoints](#retrieval-checkpoints)) | yes |
| `User/globalStorage/anysphere.cursor-agent-host/data.sqlite` | tables `ItemTable` and `cursorDiskKV`, empty throughout the round; first written by 3.25.9 | yes |
| `User/globalStorage/anysphere.cursor-agent-worker/agent-cli/` | the embedded Agent CLI, 608 MB in 453 files: `.local/share/cursor-agent/versions/2026.10.19-6bbf2c4/` (webpack chunks `NNNN.index.js` and binaries of 145-162 MB) and `.local/bin/cursor-agent`, a symlink into it | excluded |
| `User/globalStorage/anysphere.cursor-agent-worker/cursor-agent-worker-<hash10>.*` | the worker's control files: `owner.json` (`{ownerHash}`), `restart.json` (`{ownerHash, roots, version, workerId, workerName}`), `install` (23 bytes of text), `spec`, `<pid>.pid` and `log` | yes |
| `User/globalStorage/anysphere.cursor-agent-worker/worker-data/cursor-agent-worker-<hash>/` | the worker's own data directory in the CLI layout: `projects/<sanitized cwd>/{terminals,mcps}`, `logs/`, `worker.lock` (+`-journal`) | yes |
| `User/workspaceStorage/<id>/` | `workspace.json` and `state.vscdb` per opened folder, the directory name 32 hex digits (an md5 of the folder); `empty-window/` for a window with no folder ([Workspace `state.vscdb`](#workspace-statevscdb)) | yes |
| `User/History/<hex8>/entries.json` | VS Code's local file history | yes |
| `User/settings.json` | user settings (44 bytes) | yes |
| `logs/<launch stamp>/` | one directory per launch ([Logs](#logs)) | yes |
| `AgentStores/cursor_agent_stores/` | the account's and each composer's file-sync stores, 12.6 MB ([AgentStores](#agentstores)) | yes |
| `update-supervisor/<install uuid>/` | one directory per in-app update ([`update-supervisor`](#update-supervisor)); its `cursor-update-supervisor` is a 484 KB binary | yes, the binary excluded |
| `process-monitor/` | `<epoch ms>.log` every four hours (270-330 KB each, 1.4 MB in all) and `pending-uploads.json`: process logs | no: process state is the EDR's |
| `sentry/` | the crash reporter's scope with account and user context (44 KB) | no |
| `Cache`, `Code Cache`, `GPUCache`, `CachedData` (116 MB), `CachedProfilesData`, `CachedExtensionVSIXs` | Chromium and editor caches | no |
| `Cookies`, `DIPS`, `Local State`, `Preferences`, `Network Persistent State`, `Trust Tokens`, `SharedStorage`, `TransportSecurity`, `Crashpad`, `Backups`, `<version>-main.sock` (`3.24-main.sock`, `3.25-main.sock`) | the Chromium profile's state and the main process's socket | no |

Elsewhere under `~/Library`, none of it collected: the `defaults` domain
`com.todesktop.230313mzl4w4u92` (the bundle id); a `Preferences/ByHost/` property list and
`Caches/com.todesktop.230313mzl4w4u92.ShipIt/` (`ShipItState.plist` and logs) written by the
first in-app update, ShipIt being the updater component that `update-supervisor/events.jsonl` names
in its `shipit_*` event types; macOS's recent-documents list for the app,
`com.apple.LSSharedFileList.ApplicationRecentDocuments/<bundle id>.sfl4`, which named the
exported file ([Exported Markdown](#exported-markdown)); and a TCC (macOS's privacy permission
database) microphone grant to the bundle id.

#### Cursor IDE on Linux

Cursor's downloads page (fetched 2026-10-20) offers Linux builds for x64 and arm64 as a deb
package, an RPM package and an AppImage, from
`https://api2.cursor.sh/updates/download/golden/<platform>/cursor/<version>` with `<platform>`
one of `linux-arm64-deb`, `linux-x64-deb`, `linux-arm64-rpm`, `linux-x64-rpm`, `linux-arm64`
and `linux-x64`; an x64 deb refuses to install on arm64. The arm64 deb (3.24.18, then 3.25.9
through apt, 2026-10-20) wrote the tree of [Cursor IDE](#cursor-ide) under `~/.config/Cursor/`:
`User/` with `globalStorage/` (`state.vscdb` and `conversation-search.db` with their sidecars,
`storage.json`, `anysphere.cursor-agent-host/data.sqlite`), `workspaceStorage/`, `History/`
and `snippets/`; `logs/<launch stamp>/` with the macOS log names; `process-monitor/<epoch
ms>.log` from sign-in; `sentry/`; and the Chromium profile (`Backups`, `Cache`, `Cookies`,
`DIPS`, `Trust Tokens`, `Local State`, `Preferences`, `Network Persistent State`,
`SharedStorage`). It also wrote the `~/.cursor` entries of
[`~/.cursor` as the IDE writes it](#cursor-as-the-ide-writes-it). It differs from the macOS
install in these places:

- The account's and the conversations' sync stores are under the XDG state directory,
  `~/.local/state/cursor/agent-stores/cursor_agent_stores/`, not under `~/.config/Cursor`
  ([AgentStores](#agentstores)).
- The Agent CLI copy is not installed at the folder's opening or for local agent turns:
  `User/globalStorage/anysphere.cursor-agent-worker/` did not exist through the local steps,
  where macOS held the 608 MB `agent-cli/` from the folder's opening. It appeared when a cloud
  agent was first attempted (3.25.9, 2026-10-20) with two partial tarballs,
  `agent-cli/cursor-agent-2026.10.19-6bbf2c4-<pid>-<epoch ms>.tar.gz` of 88.6 MB and 35.1 MB,
  not growing after 20 seconds (stalled or abandoned downloads), an empty
  `.local/share/cursor-agent/versions/` tree and no worker control files, logs or pid files
  ([Exclusions](#exclusions)). No worker ran, so there is no `secret://` row for the worker's
  key ([Credentials](#credentials)).
- No `update-supervisor/`: the package updates through apt from the repository its
  post-install step adds, and the upgrade to 3.25.9 rewrote that repository file
  ([`update-supervisor`](#update-supervisor)).
- The shadow copies of edited files are under `User/globalStorage/anysphere.cursor-commits/`
  ([Retrieval checkpoints](#retrieval-checkpoints)).
- The GNOME keyring file `~/.local/share/keyrings/login.keyring` (891 bytes) was created at the
  first launch, before sign-in ([Credentials](#credentials)).
- No sandbox is offered ([Run mode and standing lists](#run-mode-and-standing-lists)).

Outside every home, none of it collected and none of it reachable by a catalog line, the
package's own files (`dpkg -L cursor` lists 18,419 paths): `/usr/share/cursor/` (986 MB: the
212 MB Electron binary `cursor`, `resources/app/` with the bundle `workbench.glass.main.js`,
`bin/{cursor,code-tunnel,cursor-tunnel}`, `locales/`, `chrome-sandbox`,
`chrome_crashpad_handler` and `resources/completions/{bash,zsh}`),
`/usr/share/applications/{cursor.desktop,cursor-url-handler.desktop}`,
`/usr/share/appdata/cursor.appdata.xml`, `/usr/share/bash-completion/completions/cursor`,
`/etc/apparmor.d/cursor-sandbox` (an AppArmor profile for the Chromium sandbox, replaced by the
upgrade), `/etc/sysctl.d/50-cursor.conf` and `/etc/apt/sources.list.d/cursor.sources`, the apt
repository the post-install step adds. They are install evidence, not per-user state. Under
`/run/user/<uid>/` the running app shows only as systemd transient scopes
(`app-cursor-<pid>.scope`, `app-gnome-cursor-<pid>.scope`), process state that is the EDR's.
Elsewhere under the home, not collected: GNOME's recent-files list
`~/.local/share/recently-used.xbel`, which named the exported file
([Exported Markdown](#exported-markdown)).

#### Cursor IDE on Windows

The per-user installer (3.26.13, arm64, 2026-10-21) installs the app under
`%LOCALAPPDATA%\Programs\cursor\` with no administrator rights and registers the Uninstall entry
"Cursor (User)", version 3.26.13; it adds no scheduled task, no Run key and no Credential Manager
entry. The first launch wrote the tree of [Cursor IDE](#cursor-ide) under `%APPDATA%\Cursor\`
(`AppData/Roaming/Cursor`): `User\` with `globalStorage\` (`state.vscdb` and
`conversation-search.db` with their sidecars, `storage.json`), `workspaceStorage\`, `History\`,
`snippets\` and `settings.json`; `logs\<launch stamp>\`; `sentry\`; and the editor's caches
and Chromium profile (`Backups`, `Cache`, `CachedExtensionVSIXs\.trash\anysphere.remote-ssh-1.1.16`,
`CachedProfilesData\__default__profile__\extensions.builtin.cache` of 899 KB, `code.lock`,
`DIPS`, `languagepacks.json`, `Local State`, `Preferences`, `SharedStorage`). It also wrote the
`~/.cursor` entries of [`~/.cursor` as the IDE writes it](#cursor-as-the-ide-writes-it). It
differs from the macOS install in these places:

- Chromium's network state is in a `Network\` subdirectory (`Cookies`, `Trust Tokens`,
  `Network Persistent State`, `NetworkDataMigrated`, and from the first update
  `TransportSecurity`).
- `machineid` (36 bytes) in `%APPDATA%\Cursor\` holds the device id.
- The account's and the conversations' sync stores are under the local application data
  directory, `%LOCALAPPDATA%\Cursor\AgentStores\cursor_agent_stores\`
  (`AppData/Local/Cursor/AgentStores`); `%APPDATA%\Cursor\AgentStores` never existed
  ([AgentStores](#agentstores)).
- The app updates through an installer built with Inno Setup (a third-party installer builder
  for Windows), whose log `%LOCALAPPDATA%\Temp\cursor-inno-updater-<epoch s>.log` is outside
  every catalog line. The update to 3.26.16 also wrote
  `%APPDATA%\Cursor\update-supervisor\win32\<installationId>\`; the earlier in-app update on
  3.26.13 wrote no `update-supervisor\`, and neither wrote a `%LOCALAPPDATA%\cursor-updater`
  ([`update-supervisor`](#update-supervisor)).
- `process-monitor\<epoch ms>.log` from sign-in, as on the other platforms, and
  `User\globalStorage\anysphere.cursor-agent-host\data.sqlite` (4 KB) from the first relaunch.
- The shadow copies of edited files are under `User\globalStorage\anysphere.cursor-commits\`, as
  on Linux ([Retrieval checkpoints](#retrieval-checkpoints)).
- `CachedConfigurations\folder\<id>\configuration.json` (67 bytes,
  `{content{launch, mcp, settings, tasks}}`) holds the cached settings of a remote folder, and
  `User\workspaceStorage\<id>\` with an id of the form `-3a879060`, not 32 hex digits,
  is a cloud agent's remote workspace ([Cloud agents](#cloud-agents)).
- The Agent CLI copy is not extracted at the folder's opening or for local agent turns, as on
  Linux. It was extracted at 13:26Z, while the Agents window was open and no cloud agent was
  being started: `User\globalStorage\anysphere.cursor-agent-worker\` with
  `agent-cli\.local\share\cursor-agent\versions\2026.10.19-6bbf2c4\` (the CLI, its files dated
  2026-10-20T04:38Z, the build time) and the control files
  `cursor-agent-worker-<10 hex>.{install, log, owner.json, restart.json, spec}` (`install` 23
  bytes of text, `owner.json` `{ownerHash}` of 64 hex digits, `restart.json` `{ownerHash, roots,
  version, workerId, workerName}` with one absolute path in `roots` and a 30-character
  `workerName`, `spec` one byte, a number), and `%LOCALAPPDATA%\cursor-compile-cache\`. The
  worker did not start on arm64: its `.log` is a Node 24.5.0 crash, `Error: Failed to
  load native binding for win32/arm64 (expected: ./merkle-tree-napi.win32-arm64-msvc.node)` and
  `The specified module could not be found`, with the minified bundle line echoed; it holds paths
  and no token. Each spawn attempt appends another crash with its echo of the minified source, so
  the log grows: 557 KB after the first, 5.58 MB after the fifth (15:51Z), still under the size
  cap. The extraction also wrote the worker's `secret://` rows
  ([Credentials](#credentials)).
- No sandbox is offered ([Run mode and standing lists](#run-mode-and-standing-lists)).
- `User\globalStorage\mcp-oauth-attempts\<uuid>.json`, from the first plugin install: the
  OAuth attempt of an MCP server, plaintext JSON, one file per server at a time, replaced by a
  new file when the attempt completes or the server reconnects
  ([MCP servers and plugins](#mcp-servers-and-plugins)).

Elsewhere, outside the catalog: the exported file, saved where the user chose
(`%USERPROFILE%\Desktop\export.md`; [Exported Markdown](#exported-markdown)).

#### `~/.cursor` as the IDE writes it

The IDE writes the CLI's data root (macOS install, 3.24.18 to 3.26.13; final sizes):

| Entry | Holds |
| --- | --- |
| `.gitignore` | a managed block, among its entries "Agent transcripts for citation" |
| `agents/`, `plugins/local/` | empty |
| `ai-tracking/ai-code-tracking.db` | lines of code the agent wrote and summaries per conversation, 72 KB ([`ai-code-tracking.db`](#ai-code-trackingdb)) |
| `argv.json` | Electron launch arguments, as in VS Code |
| `extensions/` | `anysphere.remote-ssh-1.1.16` and `extensions.json`, 1.3 MB; excluded |
| `projects/<sanitized cwd>/agent-transcripts/<composerId>/<composerId>.jsonl` | the worker's JSONL copy of each conversation ([Agent transcript JSONL](#agent-transcript-jsonl)) |
| `projects/<name>/mcps/<server>/{INSTRUCTIONS.md,SERVER_METADATA.json,tools/<tool>.json}` | the built-in MCP servers' tool manifests (`cursor-app-control`, `cursor-ide-browser`, `cursor-origin`, `cursor-subscriptions`, and on Windows also `cursor-computer-use`), Cursor's own, rewritten at every launch, 44 KB; the `cursor-origin` tools are in [Source-host integration](#source-host-integration) |
| `projects/<name>/mcps/plugin-<plugin>-<server>/` | an installed plugin's MCP server, in every project the plugin is enabled in, rewritten at every launch (Windows, 3.26.13): `SERVER_METADATA.json` (101 bytes, `{serverIdentifier, serverName}`, the identifier 38 characters); before authentication `STATUS.md` (268 bytes, one line, the server's needs-authentication status) and `tools/mcp_auth.json` (343 bytes, the schema of `mcp_auth`, the one tool offered); after it the server's own tool schemas instead, nine for Google Calendar (`create_event`, `delete_event`, `get_event`, `list_calendars`, `list_events`, `respond_to_event`, `search_events`, `suggest_time`, `update_event`), 164 KB per project |
| `projects/<name>/agent-tools/<uuid>.txt` | a tool's output saved during a turn (Windows, 3.26.13): the `get_mcp_tools` call's, 38 KB of JSON `{mode, namespace, namespaceStatus, tools[{description, inputSchema, tool}]}` with `mode` `namespace`, `namespace` the server's `serverIdentifier`, `namespaceStatus` `ready` and 10 tools, the MCP server's tool manifest as the turn fetched it |
| `plugins/cache/<marketplace>/<plugin>/<40-hex sha>/` | a marketplace copy of each installed plugin (Windows, 3.26.13, marketplace `cursor-public`): `.cache-complete` (0 bytes), `.cursor-plugin/plugin.json` (755 bytes, `{author{email, name}, category, description, displayName, homepage, keywords, license, logo, mcpServers, minClientVersions{cursor}, name, repository, tags, version}`), `mcp.json` (144 bytes, `{mcpServers{<name>{type, url}}}`, `type` `http`), `README.md`, `LICENSE` and `assets/logo.svg`; excluded |
| `projects/<name>/canvases/node_modules/` | type declarations (`@types/react`, `cursor/canvas`, `grok/canvas`), 226 KB per project; excluded |
| `projects/<name>/terminals/<N>.txt` | the output of each terminal while its window lives, plain text with no time but the file's: `1.txt` the integrated terminal's, a large `N` (`401229.txt`) each terminal the agent ran commands in, none for a skipped command; the agent's files are removed at quit (Windows, 3.26.13; [What survives a delete](#what-survives-a-delete)) |
| `projects/empty-window/` | the same for the window with no folder |
| `projects/<epoch ms>/`, `projects/tmp-<uuid>/` | `<epoch ms>/` holds `canvases/` (one also `terminals/`) and `tmp-<uuid>/` `canvases/` and `mcps/`; an `<epoch ms>` directory pairs with `User/workspaceStorage/<same epoch ms>/`, VS Code's id for an untitled window, and `tmp-<uuid>` is a temporary workspace a relaunch created (Linux install, 3.24.18 and 3.25.9) |
| `projects/.agent-data-cleanup-<YYYY-MM-DD>` | an empty marker, written at the first launch, named for a daily cleanup of agent data under `projects/` ([What survives a delete](#what-survives-a-delete)) |
| `skills-cursor/` | 26 built-in skills written at sign-in (automate, autopilot, canvas, create-hook, create-rule, create-skill, create-subagent, deploy-with-vercel, goal, loop, migrate-to-skills, new-repo, onboard, origin, rename-chat, review, review-bugbot, review-security, sdk, share, shell, split-to-prs, statusline, update-cli-config, update-cursor-settings, visualize), 392 KB |
| `statsig-cache.json` | a feature-flag cache from the server, 951 KB; excluded |
| `cli-config.json` | the CLI's settings, 837 bytes, written when the folder was opened ([`cli-config.json`, `permissions.json` and `sandbox.json`](#cli-configjson-permissionsjson-and-sandboxjson)) |
| `agent-cli-state.json` | `{version, workerIdsByDisplayName}`, the worker id per `<cwd> @ <hostname>` |
| `sandbox-policies/` | empty; created about 01:30Z on 2026-10-19; on Linux its modification time moved with a shell command |

On Linux the first launch wrote `.gitignore`, `agents/`, `ai-tracking/ai-code-tracking.db`,
`argv.json` (798 bytes, JSON with comments), `extensions/anysphere.remote-ssh-1.1.16/`,
`plugins/local/`, `sandbox-policies/` and `projects/`, sign-in `skills-cursor/`, and opening the
folder the project's entry, named after the sanitized cwd as on macOS. An IDE-run shell command left
no `terminals/` capture under `projects/<sanitized cwd>/`, and `~/.local/share/cursor-agent` and
`~/.local/bin` stayed absent: the IDE's worker is its own copy of the CLI.

On Windows (3.26.13, 2026-10-21) the first launch wrote the Linux set under `%USERPROFILE%\.cursor\`
(`argv.json` of 798 bytes, `.gitignore` of 985, `agents\`, `ai-tracking\ai-code-tracking.db` of
72 KB, `extensions\`, empty, `plugins\local\`, `projects\.agent-data-cleanup-<YYYY-MM-DD>` and
`projects\empty-window\{canvases\tsconfig.json, mcps\}`), and sign-in `skills-cursor\` with 16
skills and `.sync-manifest.json`. The sanitized name keeps the drive letter, lower-cased, and
replaces every other non-alphanumeric run with `-`: the folder `%USERPROFILE%\proj\demo` became
`projects\c-Users-<user>-proj-demo\`. The other `projects\` entries seen were `<epoch ms>\` for an
untitled window (`canvases`, `terminals`); `<epoch ms>\` for a workspace project the Agents window
created by itself (13:30Z, `canvases` and `terminals\1.txt`, paired with
`User\workspaceStorage\<same epoch ms>\`); and one `C-Users-<user>-AppData-Local-Temp-<uuid>\`
(capital `C`) per temporary folder the Agents window opened, five by the end of the round (two at
13:31Z and two at 13:59Z among them), each with `mcps\` and the
`canvases\node_modules\@types\react` scaffold of 61 entries ([Exclusions](#exclusions)). Opening
the folder wrote the first `terminals\` capture of any round, `1.txt` of 78 bytes, though the
user did not report opening one; the agent's commands then added one file each. `mcps\` and
`skills-cursor\` are rewritten at every launch. The plugin install wrote `plugins\cache\` and
left `plugins\local\` unchanged, with no `plugins\marketplaces\`; no `mcp-approvals.json` or
`mcp-cache.json`, which the bundle names, appeared under `.cursor\` through the MCP steps.

#### Cursor Agent CLI

CLI data root `~/.cursor` (`CURSOR_DATA_DIR` override; `index.js` module
`../cursor-config/dist/paths.js`, function `c`). Config root for `cli-config.json` is
`CURSOR_CONFIG_DIR`, else `$XDG_CONFIG_HOME/cursor`, else `~/.cursor` (same module, function `a`;
docs cli/reference/configuration).

What the macOS install (2026.10.19-6bbf2c4, 2026-10-20) wrote, in the order of its steps
([Observed](#observed)):

| Path | Holds |
| --- | --- |
| `~/.local/share/cursor-agent/versions/<ver>/` | the CLI itself, 614 MB for 2026.10.19-6bbf2c4: webpack chunks `NNNN.index.js`, `node_modules/` and the `cursor-agent` entry point; `.running/<pid>` (0 bytes) while a process of that version runs, one for the interactive process and one for its worker, which can outlive it |
| `~/.local/bin/agent`, `~/.local/bin/cursor-agent` | symlinks to that version's `cursor-agent`; no `~/.local/bin/cursor` |
| `~/.cursor/cli-config.json` | the settings, the model selection and its history, the standing allow and deny lists and, while signed in, the account identity; 837 bytes at the first start, 2,549 at most ([`cli-config.json`, `permissions.json` and `sandbox.json`](#cli-configjson-permissionsjson-and-sandboxjson)) |
| `~/.cursor/statsig-cache.json` | the server's feature-flag cache, 954 KB, from the first start until `agent logout` removed it; excluded |
| `~/.cursor/projects/<sanitized cwd>/` | one per directory the CLI started in; the home's stayed empty; the project's held `.workspace-trusted` and `repo.json` ([CLI session and trust files](#cli-session-and-trust-files)), `worker.log`, `worker.sock` while the worker ran, and `agent-transcripts/<sessionId>/<sessionId>.jsonl` ([Agent transcript JSONL](#agent-transcript-jsonl)) |
| `~/Library/Caches/cursor-compile-cache/v26.8.1-arm64-<hash>-<uid>/` | empty |
| `~/.cursor/agent-cli-state.json` | `{hasClearedLegacyStatsigFields, version}`, 59 bytes, from sign-in |
| `~/.cursor/chats/<md5 of cwd>/<sessionId>/` | one per start of `agent` in the project (the starts in the home wrote none): `meta.json`, and once a prompt is sent `prompt_history.json` and `store.db` (+`-wal`, `-shm`; a 194 KB WAL after one question, a 200 KB `store.db` once a quit checkpointed it) ([CLI chat store](#cli-chat-store)); a start that sent nothing leaves its `meta.json` alone |
| `~/.cursor/ai-tracking/ai-code-tracking.db` | the tracking database the IDE also writes, 72 KB ([`ai-code-tracking.db`](#ai-code-trackingdb)) |
| `~/.cursor/skills-cursor/` | the built-in skills, synced down after sign-in as the IDE does, with `.sync-manifest.json` |

`worker.log` holds the worker's start lines (`[info] runServer socketPath=<projects
dir>/worker.sock`, `[debug] Starting typescript-language-server ...`, `[info] Skipping codebase
telemetry sync`), about 570 bytes per start. The install wrote no `~/.config/cursor/` and no
`~/.cursor/auth.json`, and nothing inside the project but the file the agent edited: no
`.cursor/` and no `.git/cursor/`.

Contents found in the bundle (2026.10.01-e373342); the entries the macOS install did not write are
not observed on any real install:

- `projects/<sanitized cwd>/` where the name is the path with every non-alphanumeric run replaced by
  `-` (`../utils/dist/workspace-paths.js` function `s`; `paths.js` function `d` truncates to 84
  chars plus a 7-hex sha256 suffix, falling back to `/tmp/.cursor` when the home path is too long).
  Inside: `agent-transcripts/` (`*.jsonl`, observed; older `*.txt`), `.workspace-trusted`
  (observed), and, not written by the CLI on the install, `terminals/` (terminal captures),
  `agent-tools/` (tool outputs over 51,380,224 bytes), `agent-notes/shared` and
  `agent-notes/<agent>`, `mcps/`, `mcp-auth.json`, `mcp-approvals.json`, `mcp-cache.json`
  (`index.js` ignore list `!projects/*/...`; `./src/mcp/project-paths.ts`;
  `./src/workspace/approval.tsx`).
- `chats/<md5(resolved cwd)>/<sessionId>/store.db` plus `meta.json` (`7000.index.js`
  `./src/state/index.ts` functions `a`,`i`; `./src/state/chat-session-meta.ts` schema v1 with
  `createdAtMs`, `updatedAtMs`, `cwd`, `hasConversation`, `isSubagent`). `store.db` tables
  `meta(key,value)` and `blobs(id,data)` (codeburn 986). Observed, with the differences in
  [CLI session and trust files](#cli-session-and-trust-files).
- `acp-sessions/<id>/store.db` and `meta.json` (`6136.index.js`), the sessions of `agent acp`.
- `worktrees/<repo>/<name>` (`CURSOR_WORKTREES_ROOT`; `index.js` `./src/project/worktree.ts`),
  `plans/` (`1623.index.js`), `commands/`, `skills/`, `skills-cursor/` (observed),
  `cloud-skills/`, `plugins/{local,marketplaces,local-marketplaces.json}` (`index.js`
  `../cursor-plugins/dist/index.js`), `sandbox-policies/` (`CURSOR_SANDBOX_POLICY_DIR`),
  `ide_state.json` (recently viewed files), `hooks.json`, `hooks/`,
  `managed/active-team-hooks/hooks.json` (`190.index.js`), `mcp.json` (docs),
  `ai-tracking/ai-code-tracking.db` (`2734.index.js`; observed), `extensions/` and `argv.json`
  (dataFolderName convention; AgentSweep 859-872).
- Enterprise hooks: `/Library/Application Support/Cursor/hooks.json`, `/etc/cursor/hooks.json`,
  `C:\ProgramData\Cursor\hooks.json` (`190.index.js`; docs agent/hooks).
- CLI binaries: `~/.local/share/cursor-agent/versions/<ver>/`, symlinks
  `~/.local/bin/{agent,cursor-agent,cursor}` (install script lines 78-131; `2761.index.js`); the
  2026-10-20 install made no `cursor` symlink. Worker mode uses `/opt/cursor` or
  `~/.local/share/cursor-agent` as `--data-dir`.
- Remote SSH server `~/.cursor-server/` (serverDataFolderName convention; `2721.index.js` mentions
  downloading `cursor-server`); its `data/User` holds the remote side's editor state, as in
  [Remote server of `research/vscode.md`](vscode.md#remote-server).

### Credentials

- IDE: `User/globalStorage/state.vscdb` `ItemTable`, the same file as the chats, so it is
  collected unflagged and redacted by key
  ([ShayGus/quota PR 20](https://github.com/ShayGus/quota/pull/20), L4, confirmed on the macOS
  install, 3.24.18 to 3.26.13, on the Linux install, 3.24.18 and 3.25.9, and on the Windows
  install, 3.26.13):
  - tokens: `cursorAuth/accessToken` and `cursorAuth/refreshToken`, 413 bytes each, plaintext
    JWT-shaped text (JSON Web Tokens, the signed bearer-token format); still plaintext on
    3.26.13, although the key `cursor.storage.authTokenEncryptionAtRest` (5 bytes) has existed since 3.25.9;
    plaintext on Linux too (3.24.18 at sign-in, 2026-10-20), where `ItemTable` also holds, from
    the first launch, `encryption.migratedToGnomeLibsecret` (4 bytes, a boolean), VS Code's
    marker for moving its secret storage to libsecret (the Linux secret-service library the
    GNOME keyring serves); plaintext on Windows too (3.26.13 at sign-in, 2026-10-21), with the
    same identity keys beside them and no `secret://` row yet;
  - VS Code secret storage, one row per secret, keys beginning `secret://`:
    `secret://{"extensionId":"anysphere.cursor-agent-worker","key":"cursor-agent-worker.apiKey"}`
    (311 bytes, reissued at 323 bytes by the plan change), the matching `...apiKeyAuthId` row (207
    bytes), and `secret://{"extensionId":"vscode.git","key":"git-ipc-auth-token:<path>"}`, whose
    key holds the workspace-storage path; the values are encrypted with the Electron safeStorage
    key (Electron's keychain-held encryption key for app secrets), the login-keychain item
    `Cursor Safe Storage` / `Cursor Key`; redact every key with the prefix `secret://`; on Linux
    (3.25.9) the rows were `secret://remoteAgentHost.computerKey.auth0|user_<id>` (the
    credential of the machine's enrolment as a remote agent host; not seen on macOS) and
    `secret://{"extensionId":"vscode.git","key":"git-ipc-auth-token:<workspaceStorage path>"}`
    from the first agent turn, and, after the editor's GitHub sign-in,
    `secret://{"extensionId":"vscode.github-authentication","key":"github.auth"}`, the GitHub
    token; no `cursor-agent-worker` rows, since no worker ran; their values are ciphertext
    ([Secrets](#secrets)); on Windows (3.26.13, 2026-10-21) the rows were
    `secret://remoteAgentHost.computerKey.auth0|user_<id>`, present before the editor's GitHub
    sign-in (sign-in itself wrote no `secret://` row; the step that first wrote it was not
    recorded);
    `secret://{"extensionId":"anysphere.cursor-resolver-helper","key":"connectionToken:bc-<uuid>"}`,
    the token the IDE uses to attach to a cloud agent's virtual machine, and
    `secret://{"extensionId":"vscode.git","key":"git-ipc-auth-token:/home/ubuntu/.cursor-server/data/User/workspaceStorage/<id>/vscode.git"}`,
    the git extension's token on that machine, whose remote path is in the key name (a second
    connection appended `-1` to the id), both from the cloud agent's first run and its reopening
    ([Cloud agents](#cloud-agents)); the worker's
    `secret://{"extensionId":"anysphere.cursor-agent-worker","key":"cursor-agent-worker.apiKey"}`
    and `...apiKeyAuthId` rows, from the worker's extraction
    ([Cursor IDE on Windows](#cursor-ide-on-windows)); and, after the editor's GitHub sign-in,
    `secret://{"extensionId":"vscode.github-authentication","key":"github.auth"}`;
  - account identity: `cursorAuth/cachedEmail`, `cursorAuth/cachedScopedProfile` (JSON
    `{authId, displayName}`), `cursorAuth/cachedSignUpType`, `cursorAuth/stripeMembershipAuthId`
    (37 bytes), `cursorAuth/stripeMembershipType` (plain text: `free`, then `premium` for the
    plan the UI calls Pro), `cursorAuth/stripeSubscriptionStatus` (`active`, from the upgrade) and
    `adminSettings.cachedAuthId`;
  - the account id `auth0|user_<id>`, URL-encoded `auth0%7Cuser_<id>`, is part of key names
    (`cloudAgentRepository.agents.*`, `cloudAgentRepository.serverPinnedBcIds.*`,
    `cursor/localLastActivityTimestampMs:*`,
    `agentData.cacheStorage.glass.cloudMcpServers.v1.personal.*`); the numeric account id is the
    `dashboardUserId` of the settings blob ([`ItemTable` keys](#itemtable-keys)) and the
    `u<id>` directory names of [AgentStores](#agentstores). The account merge at the plan change
    added a second numeric id and kept the `auth0|user_<id>` id.
- The IDE's sign-in on macOS added no keychain item. On Linux the sign-in (3.24.18, 2026-10-20,
  through the browser) showed no keyring prompt; the GNOME keyring file
  `~/.local/share/keyrings/login.keyring` had been created at the first launch, before sign-in,
  so by its timing it holds Electron's safeStorage key and not the tokens; its item labels were
  not listed (`secret-tool` was not installed). On Windows no step wrote a Credential Manager
  entry (`cmdkey /list` listed none for Cursor, git or GitHub), and the editor's GitHub sign-in
  wrote no git credential store (no `.gitconfig`, no `.git-credentials`): the editor's git
  extension passes its token through its askpass helper.
- `ItemTable` `glass.lastSignedInAuthId`, present before sign-in on the Linux install, is by its
  name the last signed-in account's id; its value was not read.
- IDE device identifiers: `User/globalStorage/storage.json` `telemetry.devDeviceId` (a UUID),
  `telemetry.machineId` and `telemetry.macMachineId` (64 hex digits each).
- CLI (`index.js` `../cli-credentials/dist/index.js`, factory `d`): macOS uses the login keychain
  via `/usr/bin/security` (services `cursor-access-token`, `cursor-refresh-token`, `cursor-api-key`,
  `cursor-bedrock-*`, account `cursor-user`); every other OS, and macOS with
  `AGENT_CLI_CREDENTIAL_STORE=file`, writes `auth.json` (`accessToken`, `refreshToken`, `apiKey`,
  `bedrockCredentials`) at `$XDG_CONFIG_HOME/cursor/auth.json` or `~/.config/cursor/auth.json` on
  Linux, `~/.cursor/auth.json` on macOS, `%APPDATA%\Cursor\auth.json` on Windows (mode 0600). All
  three paths are flagged. On the macOS install (2026.10.19-6bbf2c4, 2026-10-20) `/login`
  created two login-keychain items, services `cursor-access-token` and `cursor-refresh-token`
  for the account `cursor-user`, and no `cursor-api-key` or `cursor-bedrock-*` item and no
  `auth.json`; `agent logout` ("Logout successful. Authentication tokens removed.") deleted both
  items. The `auth.json` of Linux and Windows is bundle-derived, not observed on any real install.
  Over SSH, with no GUI login session, every `agent` invocation, `--help` included, failed with
  "Your macOS login keychain is locked".
- CLI identity: while signed in, `~/.cursor/cli-config.json` holds `authInfo{authId,
  displayName, email, userId}` (`authId` the 37-character `auth0|user_<id>` id, `userId` the
  numeric id) and `privacyCache{ghostMode, privacyMode, updatedAt}` (macOS install); `agent
  logout` removed both objects. The file holds no token, so it is an identity file collected
  unflagged, its identity keys redacted by name ([Secrets](#secrets)).
- MCP OAuth tokens per project: `~/.cursor/projects/<p>/mcp-auth.json` (`6136.index.js`,
  `8192.index.js`); flagged. The Windows IDE (3.26.13, 2026-10-21) wrote none: it kept a
  plugin's MCP OAuth state in `state.vscdb` `ItemTable`, under keys `mcpOAuth.secret.<b64>`
  (the suffix encodes a scope and a field name;
  [MCP servers and plugins](#mcp-servers-and-plugins)), whose fields are `mcp_tokens` (the
  tokens, from authentication), `mcp_code_verifier`, `mcp_oauth_attempt_client_secret` and
  `mcp_client_information`, every value ciphertext ([Secrets](#secrets)); redact the prefix
  `mcpOAuth.secret.` like `secret://`. The plaintext `mcpOAuth.global.<b64>` keys beside them
  and the attempt files under `User/globalStorage/mcp-oauth-attempts/` hold no secret. A cloud
  agent's MCP grant is held on the server, not on the host.
- `~/.cursor/mcp.json` and project `.cursor/mcp.json` carry server env blocks that often hold
  literal keys (docs recommend `${env:...}`); both are flagged, since the secret glob
  `.cursor/mcp.json` matches in the home and in a collected project.
- `~/.cursor_info` "holds a Cursor authentication token" (AgentSweep 904-910, L4 only, unverified).
- The zsh rc copies of [Scratch directories](#scratch-directories) can hold exported keys; they are
  outside the home and not collected.
- Not credentials, though their names suggest it (macOS install): `canvas-tokens.d.ts` and
  `tokens.d.ts` (TypeScript declarations of design tokens, under `canvases/node_modules` and
  `skills-cursor/canvas/sdk`), `browser_press_key.json` (an MCP tool manifest),
  `workbench.mcp.oauth.log` (MCP connection state changes, no token; on Windows `[MCPService]
  State transition: cursor-origin none → error` four times before Origin was enabled),
  `computer_key.json` (an MCP tool manifest, Windows),
  `GitHub Authentication.*.log` ("Reading sessions from keychain", "Got 0 sessions"; on
  Windows after the editor's sign-in also `Logging in for the following scopes: read:user repo
  user:email workflow`, `Trying device code flow...`, `Login success!` and `Storing 1
  sessions...`, no token) and
  `cursorAuthDebug.*.log` (0 bytes each).

### Exclusions

`.cursor/extensions` (extension payloads), `.cursor/worktrees` (git checkouts),
`.cursor/plugins/marketplaces` (cloned marketplaces), `.cursor/plugins/cache` (the marketplace
copy of each installed plugin, re-downloadable; the installed set stays in `state.vscdb`
`cursor.plugins.installedIds.*` and in the per-project `mcps/plugin-<plugin>-<server>/`
manifests, both collected; Windows, 3.26.13),
`*/Cursor/User/globalStorage/state.vscdb.backup` (stale copy, up to 12 GB per forum 167641),
and `.cursor-server/bin` and `.cursor-server/extensions` (the remote server's binaries and
extensions, beside the collected `.cursor-server/data/User`). From the macOS install, copies of
what exists elsewhere:

- `*/User/globalStorage/anysphere.cursor-agent-worker/agent-cli`: the embedded Agent CLI, 608 MB
  of the install's 610 MB; the worker's control files and `worker-data/` beside it stay collected.
  It applies on Linux too, where the copy is downloaded only when a cloud or remote-host feature
  is first used, and partial tarballs remain when the download does not complete
  ([Cursor IDE on Linux](#cursor-ide-on-linux)), and on Windows, where the extracted copy under
  `AppData/Roaming/Cursor/User/globalStorage/` was 262.7 MB and the worker's crash log
  beside it stays collected ([Cursor IDE on Windows](#cursor-ide-on-windows)).
- `*/Cursor/update-supervisor/*/cursor-update-supervisor`: the 484 KB update helper binary
  (macOS).
- `*/Cursor/update-supervisor/*/inno_updater.exe`: the 3.4 MB Inno Setup updater binary that the
  Windows supervisor keeps from the update to 3.26.16 on; the Linux deb has no
  `update-supervisor`.
- `.cursor/projects/*/canvases/node_modules`: type declarations, 226 KB per project; on Windows
  also in every temporary project the Agents window creates
  ([`~/.cursor` as the IDE writes it](#cursor-as-the-ide-writes-it)).
- `.cursor/statsig-cache.json`: the server's feature flags, 951 KB.

`~/.local/share/cursor-agent` (557 MB per version for the bundle's build, 614 MB for
2026.10.19-6bbf2c4) is not in the catalog. `projects/*/agent-tools` holds, by the bundle,
outputs over 49 MB, but the Windows IDE wrote a 38 KB `get_mcp_tools` output there; it is
collected and left to the size cap. `.cursor/projects/*/mcps` is a candidate exclusion that is
not taken: the tool manifests are copies, rewritten at every launch, Cursor's own for the
built-in servers and downloaded for a plugin's (164 KB per project for Google Calendar), but
they show which MCP servers and tools the agent was offered. On the Windows install
(2026-10-21) `.cursor\projects` held 621 files and 2.6 MB, 383 of the files in the five
temporary projects' `mcps\` manifests and canvases.

### Project-local files

`.cursor/rules/**/*.mdc`, `.cursorrules`, `AGENTS.md`, `CLAUDE.md`, `CLAUDE.local.md` (`index.js`
`LocalCursorRulesService`; docs context/rules), `.cursor/mcp.json`, `.cursor/hooks.json`,
`.cursor/hooks/`, `.cursor/cli.json`, `.cursor/worktrees.json`, `.cursor/commands/`,
`.cursor/skills/`, `.cursor/skills-cursor/`, `.cursor/*.json` generally, `.cursorignore` (docs;
bundle). `.cursorindexingignore` appears in neither the bundle nor current docs.
`.cursor-plugin/marketplace.json` marks a plugin marketplace repo (docs). No per-project database.

The IDE bundle also reads `<root>/.cursor/permissions.json`, searched from the workspace root
down each path component, beside the user-level `~/.cursor/permissions.json` (`dataFolderName ??
".cursor"` + `permissions.json`, `workbench.glass.main.js`, 3.25.9), and the 3.25.9 renderer log
named `~/.cursor/sandbox.json` (absent; the log's error code `ENOPRO`); the install wrote none
of the three. Their fields are in [`cli-config.json`, `permissions.json` and
`sandbox.json`](#cli-configjson-permissionsjson-and-sandboxjson).

Opening the folder (3.25.9, 2026-10-19; on Linux 3.24.18, 2026-10-20) wrote a code index inside
the repository's git directory:
`.git/cursor/crepe/build.lock` and `.git/cursor/crepe/<HEAD sha>/{index.bin, metadata.json,
postings.bin}`, `metadata.json` holding `{git_revision, version}`. `.git/cursor` is not in
`PROJECT_CATALOG`. On Windows (3.26.13, 2026-10-21) the repository cloned through the editor's
Clone from GitHub got `.git\cursor\crepe\` at 14:02:10Z, still empty two minutes later; the
clone's `.git\config` (338 bytes) held `remote.origin.url` and `branch.main.vscode-merge-base`,
the latter written by the editor's git extension. The round's first folder was not a
repository and got no index.

### Where the project path is recorded

- `~/.cursor/chats/*/*/meta.json` key `cwd` (bundle `chat-session-meta.ts`), the absolute path,
  observed on the macOS CLI install and kept after the project directory was removed; the
  `chats/` directory name is the md5 of that path (checked on the install), so it names the
  project only through `meta.json` or a hash of a known path. `~/.cursor/acp-sessions/*/meta.json`
  likewise (bundle).
- `~/.cursor/projects/<sanitized cwd>/.workspace-trusted` key `workspacePath`, the absolute path
  of a folder the CLI was trusted in (macOS install).
- `~/.cursor/projects/<name>` is the sanitized cwd (lossy: `/home/u/a-b` and `/home/u/a_b` collide);
  the IDE names it the same way (`~/.cursor/projects/<sanitized cwd>/`, macOS, Linux and Windows
  installs; on Windows with the drive letter, `c-Users-<user>-proj-demo`). Its `<epoch ms>` and
  `tmp-<uuid>` names hold no path, and the Windows `C-Users-<user>-AppData-Local-Temp-<uuid>`
  names a temporary folder, not a project
  ([`~/.cursor` as the IDE writes it](#cursor-as-the-ide-writes-it)).
- IDE: `User/workspaceStorage/<md5>/workspace.json` `folder` (a `file://` URL).
- IDE, per conversation: `composerHeaders.value.workspaceIdentifier{id, uri{fsPath}}` and the
  same key in `composerData` (`id` is the `User/workspaceStorage/<id>/` directory name, checked on
  the 3.26.13 install), and `trackedGitRepos[].repoPath` in both
  ([`composerHeaders`](#composerheaders), [`composerData`](#composerdata)).
- IDE, per window: `ItemTable` `workspaceMetadata.entries` (`folderUri`, `paths[].uri`,
  `trackedGitRepos[].repoPath`, `workspaceId`) and `cursor/glass.additionalProjects`
  (`workspaceIdentifier.uri.fsPath`), and the path inside key names
  (`glass.localRepoBranchRecency.<abs path>`, `ofsContent:<composerId>:file://<path>`,
  `composer.autoAccept.lastSeenHeadSha` keys, and on Windows
  `cursor.plugins.installedIds.no-team|file:///c%3A/Users/<user>/<repo>`, `[]`, one per opened
  folder beside `...no-workspace`); `history.recentlyOpenedPathsList`; the worker's
  `restart.json` `roots` and `agent-cli-state.json` display names (`<cwd> @ <hostname>`).
- IDE, repository list: `ItemTable` `cloudEnvironmentTemplate.githubRepos`, the cache of the
  repositories a cloud agent may use, filled only once a repository is in Cursor Origin (`[]` on
  the Linux install); a cloud agent's repository is `repoUrl` in
  `cloudAgentRepository.agents.auth0|user_<id>`, and the repository name is part of key names
  ([Cloud-agent keys](#cloud-agent-keys)); and `vscode.github`, which names the open repository
  by its file URI. The GitHub account is named only by the editor's sign-in keys; Cursor's GitHub
  app connection names none locally ([Source-host integration](#source-host-integration)).

## Parsing

### Transcript stores

Relative to the home; the IDE's paths are given for macOS, with `.config/Cursor` (observed on
Linux) and `AppData/Roaming/Cursor` (observed on Windows) as the other platforms' parents.

| Store | Holds | Role |
| --- | --- | --- |
| `Library/Application Support/Cursor/User/globalStorage/state.vscdb` (+`-wal`) | every IDE conversation: `cursorDiskKV` rows per composer and per bubble, `composerHeaders` rows, and settings in `ItemTable` | primary |
| `.cursor/projects/<sanitized cwd>/agent-transcripts/<id>/<id>.jsonl` | the copy of each IDE conversation the worker writes (the id a `composerId`) and of each CLI session (the id its session id): user and assistant text and tool calls; no results or decisions, and no times but the user lines' tags | secondary |
| `.cursor/chats/<md5 of cwd>/<sessionId>/store.db` (+`-wal`), with `meta.json` and `prompt_history.json` | each standalone CLI session: the model messages as content-addressed blobs, the session's state, title and model | observed (macOS, 2026.10.19-6bbf2c4); parser pending, [issue 235](https://github.com/seanthegeek/doubleagent/issues/235) |
| `.cursor/acp-sessions/<id>/store.db` | the sessions of `agent acp` | bundle-derived, not observed on any real install; not in this plan |
| `Library/Application Support/Cursor/User/globalStorage/conversation-search.db` `cloud-cache` rows, with the `cloudAgentRepository.agents.*` record in `state.vscdb` | a cloud agent's conversation text as one block, its title, repository, model, status and times ([Cloud agents](#cloud-agents)) | the only local copy; the conversation's own store is on the agent's virtual machine |

A conversation is a `composerData:<composerId>` row plus one `bubbleId:<composerId>:<bubbleId>`
row per bubble, ordered by the composer's header list
([`composerConvHeaders` and the header list](#composerconvheaders-and-the-header-list)); the
`composerHeaders` table holds one summary row per composer. Opening the app, a window or "new
chat" creates draft composers (`status` `none`, no headers), so composers outnumber
conversations: 11 `composerData` rows for 2 conversations by 3.26.13. Likewise every start of the
CLI creates a session directory, so a start that sent nothing leaves a session with no
conversation ([CLI session and trust files](#cli-session-and-trust-files)).

#### Cloud agents

A cloud agent's conversation is not a local composer (Windows, 3.26.13, 2026-10-21, one agent
with a read-only task on a repository mirrored into Cursor Origin). Running it wrote no
`composerData`, bubble, `composerConvHeaders`, `agentKv` or `composerHeaders` row, no
`cursorDiskKV` key naming `bc-`, no transcript JSONL under `~/.cursor/projects` and no
`ai-code-tracking.db` row. The agent runs on one of Cursor's virtual machines, and the IDE opens
it as a remote workspace whose authority is `background-composer+bc-<uuid>`: the window gets a
`User/workspaceStorage/<id>/` of its own, its `workspace.json` (105 bytes) a `{folder}` with a
`vscode-remote` URL and its `state.vscdb` no composer row
([Cloud-agent keys](#cloud-agent-keys)), and only a `webWorker` extension host log,
since the real extension host runs on the machine. The `vscode.git` secret's key name places a
`.cursor-server` under `/home/ubuntu` on that machine ([Credentials](#credentials)), so the
conversation's own store is there, not on the host.

What the host keeps, all under `User/globalStorage/`:

- in `conversation-search.db`, a `conversations` row with `source` `cloud-cache` and `id` the
  `bc-<uuid>`, whose full-text content holds the title and the conversation's text as one block
  ([`conversation-search.db`](#conversation-searchdb));
- in `state.vscdb` `ItemTable`, the agent's record in the cloud-agent list
  `cloudAgentRepository.agents.<account>` (title, repository URL, model, status numbers, archive
  flag and times), the repository list `cloudEnvironmentTemplate.githubRepos`, the project and
  branch caches, and the pane keys ([Cloud-agent keys](#cloud-agent-keys));
- from the conversation's reopening after a relaunch, `cursorDiskKV`
  `bcCachedDetails:bc-<uuid>`, a protobuf blob with the prompt, a 224-character text, the
  repository, branch and model, and `composerTranscriptHeightTail:bc-<uuid>`
  ([Cloud-agent keys](#cloud-agent-keys)).

A second cloud agent (about 15:08Z) was started from the Agents window with no folder open, by
the user's own chat after the plugin's "Try in chat" button
([MCP servers and plugins](#mcp-servers-and-plugins)); the records show it ran in a new temporary
repository in Cursor Origin rather than on a mirrored one. Its element in the cloud-agent list
(2,101 bytes for two) has `branchName` `main` and a boolean `startedAsNewProject`, keys the first
agent's lacked, and `repoUrl` `origin.cursor.com/git/<namespace>/tmp-<16 hex digits>` (54
characters), where `<namespace>` is the user's Cursor namespace, the account's own space in
Origin; it also got a second remote workspace (`User/workspaceStorage/<id>/`), its own
`connectionToken:bc-<uuid>` secret (2,028 bytes) and `vscode.git` token, a second entry in
`glass.cloudAgentProjects.v1`, and a `cloud-cache` search row (a 21-character title and an
836-character body, not archived). It asked to authorise the plugin's MCP server for itself,
which the user accepted with a second Google consent, distinct from the local plugin's (the user
confirmed it): locally that left only the decision record
`cloudAgentRepository.mcpAuthResolution` and a larger cloud MCP server list
([Cloud-agent keys](#cloud-agent-keys)), and the local `mcpOAuth.*` keys did not change. It then
answered the calendar question with no permission prompt; cloud agents run under their own
policy, not the local run mode.

So a responder can read the conversation's text, title, repository, branch, model, status and
times on the host, but no tool call, tool result or per-turn time. The editor window's chat
history never listed the cloud agent, only the conversations of a locally opened folder; the
Agents window listed it under its repository's project, as does `cursor.com/agents`.

#### What survives a delete

Where delete lives. On 3.26.13 (2026-10-20) the Agents window's per-chat history menu holds Pin,
Rename, Edit Icon, Mark as Unread, Fork, Move to, Copy and Archive (or Unarchive), and its agent
pane's menu has no Delete, Export or Share; the editor window's chat history menu has Export and
Delete, and no Share. The forum says Delete "was intentionally replaced with Archive" in the
Agents window and "is still available in the chat history dropdown in the regular IDE editor
window"
([How to delete a chat in Agent Window](https://forum.cursor.com/t/how-to-delete-a-chat-in-agent-window/163380/5),
L3). The 3.26.13 bundle also has "Delete chat", the command `developer.deleteOldChats`
("Developer: Delete Old Chats...", prompting "Delete chats older than how many days?", logging
`[DeleteOldChats] Starting deletion of N chat(s) older than D days`, calling
`clearLegacyCheckpoints()` and then `deleteComposer`) and an account-level "Delete All Data"
("This will permanently delete all your data. This action cannot be undone."); none of these
three was exercised. A cloud agent's menu in the Agents window (3.26.13, Windows, 2026-10-21)
holds Pin, Rename, Edit Icon, Mark as Unread, Fork, Move to, Copy, Open in Web and Archive, and
no Delete; the editor window does not list cloud agents ([Cloud agents](#cloud-agents)).

Archive (Agents window, 3.26.13, 2026-10-20) removes nothing: every bubble, checkpoint, `agentKv`
blob, transcript JSONL file and retrieval checkpoint stayed. It sets `composerHeaders.isArchived`
to 1 and `isArchived` true in the row's `value`, moves the row's `lastUpdatedAt` (column and
value) to the archive time, and sets `is_archived` 1 and `updated_at` to that time in
`conversation-search.db`; `composerData.isArchived` stays null. The rewrite also moved the
conversation's header list to the 3.26.13 layout ([Format versions](#format-versions)).

Delete (editor window, 3.26.13, 2026-10-20) acted at once, with no confirmation and no mention of
a server copy, and removed rows only, no file:

- removed: `composerData:<composerId>`, every `bubbleId:<composerId>:*` row (37), every
  `checkpointId:<composerId>:*` row (11), `composerConvHeaders:<composerId>:000000`,
  `ofsContent:<composerId>:file://...`, `composerRootWriterDiagnostic:<composerId>`, and the
  composer's `conversation-search.db` `conversations` row;
- left in `state.vscdb`: the `composerHeaders` row, with `isArchived` 1, `lastUpdatedAt` the delete
  time, `createdAt`, `agentLocation`, `workspaceIdentifier`, `totalLinesAdded`,
  `totalLinesRemoved`, `filesChangedCount` and `hasPendingPlan` intact and the `name` and
  `subtitle` keys dropped from `value`; `composerTranscriptHeightTail:<composerId>` (layout, no
  content); and 6 of the 163 `agentKv:blob:*` rows, whose JSON still names the composer id and
  holds the conversation's model messages ([`agentKv` blobs](#agentkv-blobs));
- left under `~/.cursor`: the conversation's transcript JSONL, all 28 lines, 4,276 bytes, with
  every user and assistant text and tool call; and its 2 `ai_code_hashes` rows in
  `ai-code-tracking.db`, which carry the composer id;
- left with no link to the conversation: its `AgentStores` store and `.control/<composerId>/`
  directory (names only, no content), and the code edit's retrieval checkpoint, whose join to the
  conversation was the deleted user bubble's `requestId`, so it remains unattributed shadow
  content;
- left wherever the user saved it: the exported Markdown ([Exported Markdown](#exported-markdown)).

The other conversation and its records were untouched. Deleting the project directory was not
exercised for the IDE.

The editor window on Linux (3.25.9, 2026-10-20) did the same. Archive wrote no file and removed
nothing: it set `composerHeaders.isArchived` to 1 and `isArchived` true in the row's `value`,
moved the row's `lastUpdatedAt` to the archive time and set `is_archived` 1 in
`conversation-search.db`, and left `composerData` untouched (`isArchived` null, `lastUpdatedAt`
the last turn's, the header list inline), so the rewrite of `composerData` at the macOS archive
was the 3.26.13 header migration, not the archive. Export recorded nothing in the app's stores
([Exported Markdown](#exported-markdown)). Delete, with no confirmation, removed rows only and no
file: `composerData:<composerId>`, every `bubbleId:<composerId>:*` row (40), the
`checkpointId:<composerId>:*` rows and the composer's `conversation-search.db` row. It left the
`composerHeaders` row (`isArchived` 1, `lastUpdatedAt` the delete time, `name` dropped from
`value`), `composerTranscriptHeightTailIde:<composerId>`, 8 `agentKv` blobs naming the composer,
the transcript JSONL under `~/.cursor/projects/<sanitized cwd>/agent-transcripts/<composerId>/`,
2 `ai_code_hashes` rows, the store directory
`~/.local/state/cursor/agent-stores/cursor_agent_stores/<composerId>/`, the edit's shadow copy
under `anysphere.cursor-commits/checkpoints/<requestId>/`, now unattributed, and the exported
file.

The editor window on Windows (3.26.13, 2026-10-21) deleted a conversation that had never been
archived, with no confirmation and no file removed, as on the other platforms: gone were
`composerData`, all 46 bubbles, the `composerConvHeaders` chunk and the `conversation-search.db`
row. The `composerHeaders` row survived with `isArchived` 1, which the delete itself set, so a
deleted conversation's header reads archived whether or not it ever was; `lastUpdatedAt` became
the delete time and `name` was dropped from `value`, whose key set is in
[`composerHeaders`](#composerheaders). Also left: `composerTranscriptHeightTailIde:<composerId>`,
11 `agentKv` blobs naming the composer, the transcript JSONL (9,640 bytes, the whole
conversation, the agent's injected user turns included), the store directory
`%LOCALAPPDATA%\Cursor\AgentStores\cursor_agent_stores\<composerId>\`, the edit's shadow copy and
the exported file.

Archiving a cloud agent from the Agents window (3.26.13, Windows, 2026-10-21) removed nothing,
locally or in the cached text: it set `isArchived` from 0 to 1 in the agent's record in
`cloudAgentRepository.agents.<account>`, moving its `rowUpdatedAtMs` to the archive time and
leaving `updatedAt`, `lastReadAtMs` and the status numbers, and set `is_archived` 1 on the
`cloud-cache` row of `conversation-search.db`, leaving its `updated_at` and its 1,971-character
text; `bcCachedDetails:bc-<uuid>` was unchanged. No delete of a cloud agent was exercised, and
the IDE offers none.

Retention: `~/.cursor/projects/.agent-data-cleanup-<YYYY-MM-DD>`, an empty file written at the
first launch, is named for a daily cleanup of agent data under `projects/`; what that cleanup
removes, and whether it reaches the transcript JSONL, was not observed. A quit removes more
(Windows, 3.26.13, 2026-10-21): every terminal capture the agent's commands wrote under
`projects\<p>\terminals\` (five files, 424 to 826 bytes), leaving only the integrated terminal's
`1.txt`, rewritten at the relaunch, so a collection taken after a quit has no command output from
them; the remote workspace's `workspace.json` of a cloud agent, its `state.vscdb` kept; and, as on
Linux, an untitled window's `workspaceStorage\<epoch ms>\`.

The analyzer reads the surviving header, the transcript JSONL, the `agentKv` blobs
and `ai-code-tracking.db`, and for an archived cloud agent the cached text and the agent's
record
([Past-use rows and secondary sources](#past-use-rows-and-secondary-sources);
[Secondary sources](../analyzer/docs/output.md#secondary-sources)).

The Agent CLI has no delete, clear, export or share command: its parameters reference lists `ls`,
`resume`, `create-chat`, `login`, `logout`, `status`, `about`, `models`, `mcp`, `sandbox`,
`worker`, `acp` and `update`
([Cursor CLI parameters](https://cursor.com/docs/cli/reference/parameters), L2, fetched
2026-10-20), so a user removes a session by deleting its `chats/<md5>/<sessionId>/` directory by
hand. On the macOS install (2026.10.19-6bbf2c4, 2026-10-20) `agent logout` removed the two
keychain items, the `authInfo` and `privacyCache` objects of `cli-config.json` and
`statsig-cache.json`, and left every session (`store.db` with the WAL checkpointed into it,
`meta.json`, `prompt_history.json`), the project's `projects/<sanitized cwd>/` entry with its
transcript JSONL, `.workspace-trusted`, `repo.json` and `worker.log`, `ai-code-tracking.db`, the
standing allow list, the model selection and its history, `agent-cli-state.json` and
`skills-cursor/`. Removing the project directory afterwards changed nothing under `~/.cursor`: the
sessions stayed, their `meta.json` `cwd` naming the removed path, and so did the empty session's
`meta.json`, the `projects/` entry, whose name is the readable sanitized path, and the tracking
rows. The analyzer reads the transcript JSONL, `ai-code-tracking.db` and `cli-config.json` of such
a session; the chat store waits for [issue 235](https://github.com/seanthegeek/doubleagent/issues/235).

### Record schema

`state.vscdb` tables (3.24.18 to 3.26.13): `ItemTable(key, value)`, `cursorDiskKV(key, value)`
and `composerHeaders(composerId TEXT PRIMARY KEY, workspaceId TEXT, createdAt INTEGER,
lastUpdatedAt INTEGER, isArchived INTEGER, isSubagent INTEGER, recency INTEGER, checkpointAt
INTEGER, subagentTypeName TEXT, value TEXT)`, indexed on `(workspaceId, isSubagent, isArchived,
recency)` and `(recency, composerId)`. Values are JSON text unless a row says otherwise. The
`cursorDiskKV` key families:

| Key | Value |
| --- | --- |
| `composerData:<composerId>` | the composer ([`composerData`](#composerdata)); also `composerData:empty-state-draft` |
| `composerConvHeaders:<composerId>:<NNNNNN>` | a chunk of the header list, 3.26.13 ([`composerConvHeaders` and the header list](#composerconvheaders-and-the-header-list)) |
| `bubbleId:<composerId>:<bubbleId>` | a user bubble ([User bubbles](#user-bubbles)) |
| `bubbleId:<composerId>:<bubbleId>/s:<n>` | an assistant sub-bubble ([Assistant sub-bubbles](#assistant-sub-bubbles)) |
| `bubbleId:<composerId>:<bubbleId>/t:call-<uuid>-<n>` LF `fc_<uuid>_<n>` | a tool bubble; the key holds a literal line feed ([Tool bubbles](#tool-bubbles)) |
| `checkpointId:<composerId>:<checkpointId>`, `ofsContent:<composerId>:file://<path>`, `composer.content.<sha256>`, `inlineDiff:<workspaceId>:<uuid>`, `composer.autoAccept.lastSeenHeadSha`, `composer.autoAccept.lastSeenHeadTimestamp` | file state of an edit ([File-state rows](#file-state-rows)) |
| `agentKv:blob:<sha256>` | a content-addressed blob ([`agentKv` blobs](#agentkv-blobs)) |
| `composerDraft:<composerId>` | `{richText, text}`, an unsent draft (3.26.13; 35 characters on the install) |
| `composerRootWriterDiagnostic:<composerId>` | `{version, writerInstanceId, rendererRootBlobIdHex, previousRendererRootBlobIdHex, throughWriterSequence, writtenAtMs, writers, writersOmitted, predecessorVerified}`; `rendererRootBlobIdHex` (64 hex) names an `agentKv` blob |
| `bcCachedDetails:bc-<uuid>` | a cloud agent's cached details, protobuf, written when the conversation is reopened ([Cloud-agent keys](#cloud-agent-keys)) |
| `composerTranscriptHeightTail:<composerId>` (`composerTranscriptHeightTailIde:<composerId>` for the editor window's conversation, 3.25.9 on Linux and 3.26.13 on Windows; `composerTranscriptHeightTail:bc-<uuid>` for a cloud agent's), `composerVirtualRowHeights:_recentIds`, `slashMenuItems/v7/local.glass.<workspace>`, `model-nudge.startup-trigger-session`, `model-nudge.startup-session`, `composer.composerHeaders.migratedToTable` | layout, menu and migration state, no conversation content |

#### `composerHeaders`

One row per composer, drafts included (5 rows by 3.26.13: two empty pre-login composers, the two
conversations and one composer the UI archived at once at 17:55:30Z, `status` `none`, no bubbles).
The columns duplicate the value's times; `workspaceId` is 32 hex digits, or `empty-window`.
`value` is JSON with the keys `type` (`head`), `composerId`, `name`, `subtitle`, `createdAt`,
`lastUpdatedAt`, `conversationCheckpointLastUpdatedAt`, `unifiedMode` (`agent`), `forceMode`
(`edit`), `hasUnreadMessages`, `contextUsagePercent`, `totalLinesAdded`, `totalLinesRemoved`,
`hasBlockingPendingActions`, `hasPendingPlan`, `isDraft`, `isWorktree`,
`worktreeStartedReadOnly`, `isSpec`, `isProject`, `isBestOfNSubcomposer`, `numSubComposers`,
`referencedPlans`, `trackedGitRepos[{repoPath, branches}]`, `workspaceIdentifier{id, uri}`,
`agentLocation{type, status, environment{id, uri{fsPath}}}` (`type` `local`, `status` `active`
on both conversations), `agentLocationHistory[{id, reason, timestamp, destination{type},
location}]` (`reason` `created`) and `isArchived`; a deleted conversation's row also had
`filesChangedCount`. The empty-window head has `isEphemeral`, a draft `draftTarget`,
`isArchived` and `hasBeenInSidebar`.

On Windows (3.26.13, 2026-10-21) none of the five rows' `value` had `agentLocation` or
`agentLocationHistory` at the delete step (02:28Z); a conversation created later in the same
folder (15:21Z) had both, `agentLocation{...}` with `type` `local` and one
`agentLocationHistory` entry, so their absence is per conversation, not per platform. The
deleted conversation's surviving `value` (831 bytes) held
`composerId`, `contextUsagePercent`, `conversationCheckpointLastUpdatedAt`, `createdAt`,
`filesChangedCount`, `forceMode`, `hasBlockingPendingActions`, `hasPendingPlan`,
`hasUnreadMessages`, `isArchived`, `isBestOfNSubcomposer`, `isDraft`, `isProject`, `isSpec`,
`isWorktree`, `lastUpdatedAt`, `numSubComposers`, `referencedPlans`, `totalLinesAdded`,
`totalLinesRemoved`, `trackedGitRepos`, `type`, `unifiedMode`, `workspaceIdentifier` and
`worktreeStartedReadOnly`. The two keys are therefore optional, and what decides whether a header
carries them is not known.

#### `composerData`

Keys seen (3.24.18 to 3.26.13): `_v`, `composerId`, `createdAt` and `lastUpdatedAt` (epoch ms
numbers), `lastReadAtMs`, `status` (`none` on a draft, `completed` after a turn), `name` (the
automatic title) and `subtitle`, `unifiedMode` (`agent`), `forceMode` (`edit`),
`conversationState` (an opaque string, 425 to 2,225 characters), `conversationMap` (`{}`: the
bubbles are rows of their own), `fullConversationHeadersOnly` (the header list, inline until
3.26.13), `modelConfig{modelName, maxMode, selectedModels[{modelId, parameters[{id, value}]}]}`
(parameters seen: `effort` `medium`, `context` `256k`), `latestChatGenerationUUID`,
`contextTokenLimit`, `contextTokensUsed`, `contextUsagePercent`,
`promptTokenBreakdown{categories[{id, label, characterCount, estimatedTokens}], maxTokens,
totalUsedTokens}`, `promptContextUsageTree{nodes[{id, parentId, categoryId, kind, label,
characterCount, estimatedTokens, contentAvailable}]}` (`kind` `tool_definition`),
`trackedGitRepos[{repoPath, branches}]`, `workspaceIdentifier{id, uri{fsPath}}`,
`conversationCheckpointLastUpdatedAt`, `blobEncryptionKey` and
`speculativeSummarizationEncryptionKey` (44-character strings), `isAgentic`, `isDraft`,
`isProject`, `isArchived` (null even when archived), `isBestOfNParent`, `isBestOfNSubcomposer`,
`isSpec`, `subComposerIds`, `subagentComposerIds`, `todos`, `queueItems`, `totalLinesAdded`,
`totalLinesRemoved`, `usageData`, `context`, `originalFileStates`, `newlyCreatedFiles` and
`capabilities[{type, data{bubbleDataMap}}]` (7 entries); a draft also has `draftTarget{environment
{id}, type}` (`existing`). A model switch rewrites `modelConfig` in place without moving
`lastUpdatedAt`.

#### `composerConvHeaders` and the header list

The header list is the ordered index of a composer's bubbles, one entry per bubble record:
`{bubbleId, type, grouping{isRenderable, hasText, isShortPlainText, hasThinking, capabilityType,
thinkingDurationMs, turnDurationMs, isKeptFinalAiVisibleOutsideWorkedForGroup,
toolDisplayComputed}, contentHeightHint, createdAt, startedAtMs, completedAtMs}`. `type` is 1 for
a user bubble and 2 for an assistant one; `bubbleId` is the bubble's id with its suffix, so
`<uuid>`, `<uuid>/s:<n>` or `<uuid>/t:call-<uuid>-<n>` LF `fc_<uuid>_<n>`, and is what follows
`bubbleId:<composerId>:` in the record's key. Up to 3.25.9 the list is
`composerData.fullConversationHeadersOnly`; on 3.26.13 it is a JSON array in
`composerConvHeaders:<composerId>:000000`, the six-digit suffix a chunk number (one chunk held 37
entries, 12,471 bytes, so a chunk's capacity is larger than any conversation seen), with
`fullConversationHeadersOnly` `[]`. On the install the list had one entry per bubble record (37
for 37).

#### User bubbles

`bubbleId:<composerId>:<bubbleId>` with `type` 1: `text` (what the user typed), `richText` (the
editor's structured form of the same text), `createdAt` (ISO 8601), `startedAtMs`,
`completedAtMs`, `requestId` (a UUID), `checkpointId` (a UUID), `modelInfo{modelName}`,
`agentMode` and `unifiedMode` (numbers), `tokenCount{inputTokens, outputTokens}`, `isAgentic`,
`isPlanExecution`, `context{...}`, `conversationState`, `contextWindowStatusAtCreation
{percentageRemaining, percentageRemainingFloat, tokenLimit, tokensUsed}` (from the second turn
on), and empty lists such as `toolResults`, `allThinkingBlocks`, `commits` and `deletedFiles`.
`modelInfo.modelName` is the model the turn was sent to (`grok-4.6` on the six turns before the
switch, `grok-4.7` after it).

Not every user bubble is typed (Windows, 3.26.13, 2026-10-21). Of the conversation's first 7
user bubbles, 4 were typed prompts, with `modelInfo{modelName}` (`default` while Auto was
selected) and `agentMode` 1 and 26 to 98 characters of text; 3 had no `modelInfo` and no
`agentMode`, 527 to 689 characters of text, and a `createdAt` within a second of a terminal
command's completion: turns the agent injected, such as a command's output fed back as a user
message, not the person's words. Their header entries are `type` 1 like a typed prompt's (7 of
type 1 and 36 of type 2 for 43 bubbles at that point).

#### Assistant sub-bubbles

One assistant turn is a `bubbleId` uuid whose records carry suffixes: `/s:<n>` for reasoning and
text, `/t:call-<uuid>-<n>` for tool calls, with one counter `n` across both, so the `/s:` indices
skip the slots taken by tool bubbles (`/s:0`, `/s:2`, `/s:3` beside `/t:call-<uuid>-1`). Every
assistant and tool bubble key carries a `/s:` or `/t:` suffix; only user bubbles have none
(3.26.13). A sub-bubble has `type` 2, its own `bubbleId`
(the uuid plus the suffix), `capabilityType`, `text`, `createdAt`, `startedAtMs`, `tokenCount
{inputTokens, outputTokens}`, `codeBlocks` and `requestId` (empty). A reasoning sub-bubble has
`capabilityType` 30, `thinking{text, signature}`, `thinkingDurationMs`, `thinkingStyle` and an
empty `text`; a text sub-bubble has `capabilityType` null, the reply in `text` and, on the last
of a turn, `turnDurationMs`. Assistant records have no `modelInfo`.

#### Tool bubbles

A tool bubble has `capabilityType` 15, `createdAt`, `startedAtMs`, `completedAtMs`, `tokenCount` and
`toolFormerData` with the keys `name`, `params` (a JSON string), `rawArgs` (a JSON string, empty or
absent on `edit_file_v2` and on a terminal command), `result` (a JSON string, 22 to 1,643 characters
seen; an error object when the call was refused), `status` (`completed`, even for a refused call;
`loading` while the call waits at its prompt), `tool` (a number), `toolCallId` (85 characters),
`modelCallId` (85 characters, or empty), `toolCallBinary` (base64), `toolIndex` and
`additionalData`. Names seen: `glob_file_search`, `get_mcp_tools`, `read_file_v2`, `edit_file_v2`,
`run_terminal_command_v2` and, for an MCP server's tool, `mcp-<server>-<tool>`. One bubble holds
both the call and its result.

An MCP call (Windows, 3.26.13, 2026-10-21, the Google Calendar plugin's tools) is a tool bubble
of the same shape: `toolFormerData{name, tool, status, toolCallId, modelCallId, toolIndex,
rawArgs, params, result, additionalData, toolCallBinary}` with `name`
`mcp-google-calendar-list_events` or `mcp-google-calendar-list_calendars` (31 and 34
characters), `tool` 19, `status` `completed`, `rawArgs` a JSON string of 178 to 598 characters,
`params` one of 76 to 308, `result` one of 539 to 4,472, `additionalData{status: "success"}` and
`toolCallBinary` 1,196 to 5,964 characters, a serialised copy of the call whose encoding is not
known. The call that lists a server's tools first is `get_mcp_tools`, `tool` 63, with
`additionalData` `{}`. Under Allowlist the user was asked to approve each MCP call and approved
each, yet no approved MCP bubble has `reviewData`, `selectedOption` or `blockReason`: an MCP
prompt answered Run leaves no per-call record on 3.26.13, unlike a shell command's. Under
Auto-review the same calls were not prompted and have the same shape.

A turn the user stopped (Windows, 3.26.13, the "Try in chat" conversation) left its running
terminal call with `toolFormerData.status` `completed`, `additionalData.status` `rejected`,
`additionalData.reviewData.status` the string `None` (present, not `Done`) and `selectedOption`
the preset `rejectAndTellWhatToDoDifferently`, which nobody chose; the three tool calls after it
(`read_file_v2` and two `glob_file_search`) have `toolFormerData.status` `cancelled` and an
empty `additionalData`.

`additionalData` is absent on a call that ran with no review (a file read); a terminal command
that ran under auto-run has `{status, startedAtMs}`, and so has one that ran because the
standing allow list held it (Linux, 3.25.9, 2026-10-20), which leaves no per-call decision; a
command the user was asked about has:

| Key | Values seen |
| --- | --- |
| `status` | `success` (ran), `rejected` (skipped, or cut off by a Stop), `error` (ran and the command failed; Windows, under auto-run), `pending` (the prompt open; Windows) |
| `startedAtMs` | present when the command ran |
| `blockReason` | why it was held, e.g. `Not in allowlist: curl, head`, or `Shell allowlist is empty` (Windows) |
| `reviewData.selectedOption` | `run` (Run, and the editor window's add-to-allow-list), `skip` (Skip), `allowlistCommands` (the Agents window's Always run), `rejectAndTellWhatToDoDifferently` (the preset while the prompt is open: the skip-and-tell option, never seen chosen; it also stays on a call cut off by a Stop) |
| `reviewData.approvalType` | `user` |
| `reviewData.status` | `Requested` (the prompt open), `Done` (answered), `None` (the turn stopped by the user; Windows) |
| `reviewData.allowlistOptions` | `[{id, label, value, commandName, ancestorIds, tokens, tokenCount, indentLevel}]`, one per command in the line |
| `reviewData.candidatesForAllowlist`, `defaultAllowlistOptionIds`, `selectedAllowlistOptionIds` | the commands offered and preselected (`curl`, `head`) |
| `reviewData.appliedAllowlistOptionIds` | only after an allow-always choice, with either `selectedOption`: the entries the choice named |
| `reviewData.hasClassifierSuggestions`, `reviewData.isShowingInput` | booleans |

The prompt the user answered in the Agents window on macOS offered Skip, Always run and Run
(Allowlist (with Sandbox), 3.25.9, 2026-10-19); the editor window's prompt on Linux offered Run,
Skip, a change of run mode and "add python3 to the allow list", naming the command (Allowlist,
3.25.9, 2026-10-20). The decision records were the same on both: Run wrote `selectedOption`
`run`, `approvalType` `user`, `additionalData.status` `success` and `startedAtMs`; Skip wrote
`skip`, no `startedAtMs` and a refusal object in `result` (70 characters on Linux).

On Windows (3.26.13, editor window, Allowlist, 2026-10-21) the prompt for a `powershell -Command
...` line offered a change of run mode, Skip and Run, and no allow-list entry: the waiting
bubble had `toolFormerData.status` `loading`, `additionalData{status: "pending", blockReason:
"Shell allowlist is empty", reviewData{status: "Requested", selectedOption:
"rejectAndTellWhatToDoDifferently", allowlistOptions: [], candidatesForAllowlist: [],
hasClassifierSuggestions: false}}`. Whether `powershell` was not offered because it is an
interpreter, because of the `-Command` form or because of a 3.26 change is not known; Linux
3.25.9 offered `python3`. Run and Skip then wrote the same records as on macOS and Linux (Skip's
refusal object 70 characters), and a skipped command left no terminal capture. Earlier, under
the Auto-review default, the agent's commands carried `additionalData{status, startedAtMs}` with
`status` `success` three times and `error` once, and no `reviewData`.

A call waiting at its prompt is on disk before any answer (Linux, 3.25.9, editor window,
2026-10-20): `toolFormerData.status` `loading`, and `additionalData{status, blockReason,
reviewData}` with `reviewData{status: "Requested", selectedOption:
"rejectAndTellWhatToDoDifferently", isShowingInput: false, allowlistOptions}`. Its
`selectedOption` is a preset, not an answer; when the user chose Run the same bubble became
`completed` with `reviewData.status` `Done`. The transcript JSONL already held the call's
`tool_use` line.

The allow-always choice has two forms: the Agents window's Always run wrote `selectedOption`
`allowlistCommands` with `appliedAllowlistOptionIds` (macOS, 3.25.9), and the editor window's
"add python3 to the allow list" wrote `selectedOption` `run` with
`candidatesForAllowlist` and `appliedAllowlistOptionIds` both `[<command>]` and
`additionalData.status` `success` (Linux, 3.25.9).

#### File-state rows

An edit (3.25.9) wrote `checkpointId:<composerId>:<checkpointId>` (`{files[{uri{path},
isNewlyCreated, originalModelDiffWrtV0}], activeInlineDiffs, newlyCreatedFolders,
nonExistentFiles}`; a turn with no edit stores `{inlineDiffNewlyCreatedResources,
nonExistentFilesVersion}`), `ofsContent:<composerId>:file://<path>` (the file's text before the
edit, the path in the key), two `composer.content.<sha256>` rows (the file's text after and
before), `inlineDiff:<workspaceId>:<uuid>` (`{composerMetadata{composerId, composerGenerationID,
codeblockId, toolCallId}, diffId, generationUUID, originalTextLines, uri{path}}`) and
`composer.autoAccept.lastSeenHeadSha` (`{<repository path>: <commit sha>}`) with
`composer.autoAccept.lastSeenHeadTimestamp`.

#### `agentKv` blobs

`agentKv:blob:<sha256>` rows are the model calls' content, stored once per hash (11 after the
first question, 163 by the end: 44 JSON, 119 protobuf). The JSON ones are model messages,
`{role, content, providerOptions}` with `role` `system`, `user` or `assistant`, `content` a string
or an array of objects, and an
`id` (40 characters) on assistant messages: the user message is the whole prompt with its
context (30,511 bytes for a 37-character question) and the system message the system prompt
(4,441 bytes). The protobuf ones (first bytes `0A` or `1A`) are not decoded. No key names a
composer: a blob belongs to a conversation when the composer id occurs inside its `content` text,
that is, the prompt names it. Of the 6 rows that named the deleted conversation after its delete,
2 were JSON user messages (30,511 and 31,311 bytes) and 4 protobuf (615 to 653 bytes); 2 other
JSON blobs named the live conversation. They are plaintext, although the composer carries a
`blobEncryptionKey`.

#### `ItemTable` keys

Keys that hold conversation, permission and model evidence (macOS install):

- `src.vs.platform.reactivestorage.browser.reactiveStorageServiceImpl.persistentStorage.applicationUser`:
  the settings blob, 493 KB of JSON; its keys are in
  [Run mode and standing lists](#run-mode-and-standing-lists), plus `aiSettings.modelConfig
  .<surface>{modelName, maxMode, selectedModels}` for the surfaces `composer`, `cmd-k`,
  `background-composer`, `plan-execution`, `quick-agent`, `spec`, `deep-search` and
  `composer-ensemble`; `aiSettings.modelLastUsedAt{<model id>: <ISO time>}` (9 ids, among them
  `grok-4.6` and `grok-4.7`, moved by a relaunch and by the switch on macOS);
  `availableDefaultModels2[{name, serverModelName, idAliases, supports*}]` (46);
  `cursorCreds{backendUrl, agentBackendUrl*, authClientId, authDomain}` (URLs and client ids, no
  secret); `dashboardUserId`; `membershipType` (`free`, then `premium`); `serverPrivacyMode`;
  `teamAdminSettings`; `teamBlocklist`; `mcpServers`; `mcpDisabledTools`;
  `cliDetectionStatus{agentStatus, claudeStatus, codexStatus}` (Cursor checks for other agents'
  CLIs); `authenticationSettings{githubLoggedIn}`; `backgroundComposerState`. The app writes the
  blob late: on Linux (3.25.9, 2026-10-20) `modelLastUsedAt` gained its `grok-4.6` entry at
  22:33:30Z, at a command run after a relaunch, not at the switch (about 22:05Z) or at the
  question sent after it (about 22:06Z), its other entries all carrying the sign-in time; and the
  standing allow list did not show an addition made from the prompt
  ([Run mode and standing lists](#run-mode-and-standing-lists)). A time in the blob, and the
  file's modification time, can therefore follow the action they record by many minutes. On
  Windows (3.26.13, 2026-10-21) the switch from Auto to `grok-4.6` set
  `aiSettings.modelConfig.composer.modelName` and gave `modelLastUsedAt` its `grok-4.6` entry at
  the switch itself (02:11:03.900Z), as on macOS, before any message was sent.
- `glass.agentConversationPanes.<id>` (`{activeTabId, tabAgentIds, version}`),
  `cursor/glass.selectedAgent`, `cursor/glass.lastRealAgent`,
  `glass/cursor.editorPanelVisibility.agent/<id>`, `cursor/glass.editorPanelFullscreen/<id>`:
  which conversation each pane shows.
- `glass.localRepoBranchRecency.<repository path>`: the branches used, `["main"]`.
- `cursor/localLastActivityTimestampMs`, `cursor/localLastActivityClientVersion` (also per
  account, `:auth0%7Cuser_<id>`).
- `cursor/glass.additionalProjects`, `cursor/glass.removedProjects`, `workspaceMetadata.entries`
  ([Where the project path is recorded](#where-the-project-path-is-recorded)).
  `cursor/glass.additionalProjects` is the Agents window's project list: on Windows (3.26.13)
  three entries `{id, lastUsedAt, name, repoUrls, type}` (`id` 39 characters, `type` 4) after the
  cloud agent, and a fourth `{id, lastUsedAt, name, type: "workspace",
  workspaceIdentifier{id}}` when the Agents window created a workspace project by itself, `id`
  the project's epoch-ms name.
- `cursorai/donotchange/{newPrivacyMode2, privacyMode, partnerDataShare,
  newPrivacyModeHoursRemainingInGracePeriod}` and `cursorai/serverConfig` (60 KB of server
  configuration); `adminSettings.cached` (792 bytes before sign-in).
- Written by the editor's own GitHub sign-in (Linux, 3.25.9, 2026-10-20; VS Code's GitHub
  authentication): `github-<login>` (55 bytes, `[{id, name, allowed}]`, the extensions allowed
  to use the GitHub account, the login in the key name), `github-<login>-usages` (137 bytes,
  `[{extensionId, extensionName, lastUsed, scopes}]`: which extension used the account, when as a
  number, and with which scopes), `vscode.github-github` (13 bytes of opaque text that contains
  the login) and `vscode.github-github-repo workflow user:email read:user` (16 bytes of hex; the
  granted scopes in the key name).
- Written by connecting Cursor's GitHub integration (Linux, 3.25.9):
  `cursor/glass.githubConnect.pendingFlow` (273 bytes, `{flowId, state, verifier, expiresAtMs,
  shouldInstall, source}`, `flowId` 32 hex digits, `source` `cloud-agent`), the connect flow's
  record, present at the listing after the connection (23:09Z) and gone by 23:40Z;
  `cloudEnvironmentTemplate.githubRepos` (`[]`, the cache of repositories a cloud agent may use),
  `cloudEnvironmentTemplate.scmTruncatedInstallations` and
  `cloudEnvironmentTemplate.scmBitbucketServerHosts` (both `{}`); `vscode.github`
  (`{branchProtection:<file URI of the open repository>: []}`, VS Code's GitHub extension state
  per repository); `cursor.startupConnectTransport.gate` and
  `cursor.tray.mainProcessCloudRefreshEnabled`. Present before it and still empty:
  `cloudAgentRepository.agents.auth0|user_<id>` (`[]`, the cloud-agent list),
  `cloudAgentRepository.serverPinnedBcIds.auth0|user_<id>` (`[]`) and
  `cursor/glass.runningSessionCloudSetupCta.multiRepoOwnerUserId.auth0|user_<id>` (a number).
- `remoteAgentHost.enrolled.auth0|user_<id>` (a boolean, 4 bytes),
  `remoteAgentHost.displayName.auth0|user_<id>` (6 bytes on Linux; 14 bytes on Windows, the
  machine's host name) and `remoteAgentHost.targetId` (a UUID, 36 bytes): the machine's enrolment
  as a remote agent host for the account, whose credential is
  `secret://remoteAgentHost.computerKey.auth0|user_<id>` ([Credentials](#credentials)); when they
  appear and what pairing a phone leaves are in [Remote control](#remote-control). The Welcome
  tab's "Steer from iOS" button, beside "Clone a repository", wrote
  `cursor/steerFromCloudModalOpened` (a boolean, 4 bytes) when its modal was opened (Windows,
  3.26.13, 2026-10-21); `cursor/glass.steerFromPhonePill.cadence.v1/<account>` (52 bytes,
  `{firstShownAtMs, impressionCount}`), present from the cloud agent's step, is the cadence record
  of the interface's nudge to steer the agent from a phone.
- `cursor/agentAutorunBrandNewDefaultAttempted` (true, set before sign-in),
  `cursor/sandboxSupported`, `cursor/sandboxAutoRunSettingEnabled` (on Linux the first absent
  throughout and the second absent until the run-mode switch wrote it false).
- `cursor.update.sendAuthHeaders` (before sign-in), `cursor.storage.disableSqliteStorageBackup`
  and `cursor.storage.sqliteIncrementalAutoVacuumOnClose`: seen on the Linux install.
- On Windows `cursor/sandboxSupported` was absent throughout, as on Linux; the editor's GitHub
  sign-in wrote the Linux keys (`github-<login>` 55 bytes, `github-<login>-usages` 137,
  `vscode.github-github` 13, `vscode.github-github-repo workflow user:email read:user` 16) and
  `vscode.github` (87 bytes, `{branchProtection:file:///c%3A/Users/<user>/<repo>: [{remote,
  rules}]}`, `remote` `origin`); opening the clone wrote `backgroundComposer.windowBcMapping`
  (`{"1": []}`), `notifications.perSourceDoNotDisturbMode` and the
  `cursor.plugins.installedIds.*` keys of
  [Where the project path is recorded](#where-the-project-path-is-recorded).
- `mcpOAuth.global.<b64>`, `mcpOAuth.secret.<b64>` and the `cursor.plugins.installedIds.*`
  values: a plugin's MCP OAuth state and the installed plugins (Windows, 3.26.13), in
  [MCP servers and plugins](#mcp-servers-and-plugins).

#### Cloud-agent keys

The records of a cloud agent ([Cloud agents](#cloud-agents)) on Windows (3.26.13, 2026-10-21),
in `User/globalStorage/state.vscdb`. `<account>` is the `auth0|user_<id>` account id, which
appears URL-encoded (`auth0%7Cuser_<id>`) inside key names, and `<repo key>` is
`github.com%2F<owner>%2F<repo>`, the repository's name in a key name, or for a temporary Origin
repository `origin.cursor.com%2Fgit%2F<namespace>%2Ftmp-<hex>`. `ItemTable`, JSON unless said:

| Key | Value |
| --- | --- |
| `cloudAgentRepository.agents.<account>` | the cloud-agent list, `[]` before the first agent (1,026 bytes with one): `[{bcId, cloudTargetKey, createdAt, environmentPublicId, hasPendingInteraction, hasStartedVm, isArchived, isKilled, isOwnedByDifferentTeamMember, isUnread, lastMessageActivityAtMs, lastReadAtMs, modelDetails{maxMode, modelName}, name, repoUrl, repoUrls, requestedModel{builtInModel, maxMode, modelId, parameters[{id, value}]}, rowUpdatedAtMs, source, status, transportRepoUrls, updatedAt, usePrivateWorker, workflowStatus, workspaceRootPath}]`; `bcId` 39 characters, `cloudTargetKey` 42, `environmentPublicId` a UUID, `name` the title (36), `repoUrl` `github.com/<owner>/<repo>`, `modelDetails.modelName` and `requestedModel.modelId` 8 characters each, three parameters, `source`, `status` and `workflowStatus` numbers (16, 2 and 2 after the run), `workspaceRootPath` an absolute path on the agent's machine; the second agent's element added `branchName` (`main`) and `startedAsNewProject` (a boolean), and its `repoUrl` was an Origin repository, `origin.cursor.com/git/<namespace>/tmp-<16 hex digits>` |
| `cloudAgentRepository.serverPinnedBcIds.<account>` | `[]` |
| `cloudAgentRepository.mcpAuthResolution` | the user's answer to a cloud agent's request to authorise an MCP server, 310 bytes: `{agentId, authId, nonce, queryId, toolCallId, userDecision, version}`, `agentId` the 39-character `bc-<uuid>`, `authId` 37 characters, `nonce` a UUID, `toolCallId` 85 characters, `userDecision` `accepted`; the meaning of `authId`, `queryId` and `version` is not known |
| `agentData.cacheStorage.glass.cloudMcpServers.v1.personal.<account>` | the account's cloud MCP server list, not JSON, printable; 95 bytes before the cloud agent's MCP authorisation, 381 after |
| `cloudEnvironmentTemplate.githubRepos` | the repository list, `[]` until a repository was in Cursor Origin (127 bytes with one): `[{name, scmRepoNodeId, type, url}]`, `name` `<owner>/<repo>`, `scmRepoNodeId` 12 characters, `type` 7, `url` 34 |
| `glass.cloudAgentProjects.v1` | `[{createdAt, id, isArchived, lastUpdatedAt, name, workspace{id, uri{$mid, authority, external, path, scheme}}}]`, `id` a UUID, `name` `<owner>/<repo>`, `scheme` `vscode-remote`, `authority` 59 characters |
| `glass.cloudAgentProjectMembership.v1` | `{"bc-<uuid>": <project uuid>}` |
| `glass.cloudRepoBranches.<account>.<repo key>` | `{branches[{isDefault, name}], fetchedAtMs, hasMore, isEmptyRepo, page}`, the repository's branches (14) |
| `glass.cloudRepoBranchRecency.<account>.<repo key>` | `[{name, selectedAtMs}]` |
| `agentData.cacheStorage.glass.repoSlashInventory.v2.<repo key>.<account>` | 122 bytes, not JSON |
| `cursor/glass.projectSelector.recentCloudTargets` | `[{githubNodeId, lastSelectedAt, repoUrl, startTargetSelection, templateType}]` |
| `cursor/glass.selectedAgent`, `cursor/glass.lastRealAgent` | the `bc-<uuid>` as a bare string |
| `glass.agentConversationPanes.bc-<uuid>`, `cursor/glass.editorPanelFullscreen/bc-<uuid>`, `glass/cursor.editorPanelVisibility.agent/bc-<uuid>` | the pane state of [`ItemTable` keys](#itemtable-keys) |
| `cursor/glass.tabs.v2/<workspace id>/state.json` | the Agents window's tabs per workspace |

`cursorDiskKV` gained, at the reopening after a relaunch, `bcCachedDetails:bc-<uuid>` (1,783
bytes of protobuf, the only protobuf family beside the `agentKv` blobs) and
`composerTranscriptHeightTail:bc-<uuid>` (`{f: [], h: [7 x [4 x string]], l, t, v}`, the seven
rendered turns' layout). `bcCachedDetails` decoded by wire type alone, with no descriptor (field
numbers, with what each holds):

```text
3 (1,776 bytes)
  1: the agent record: the bc id (39), a fixed64, text 10, the name (36), text 42 (the
     cloudTargetKey), varints, 28 {model (8), varint}, branch (4), text 56, a UUID, a fixed64,
     59 {model (8), 3 x {parameter id, value (4)}}, the repoUrl (34), a UUID, varints
  2: text 40
  3: text 11
  4.7: {1: text of 139 characters, the length of the user's prompt; 2: varint; 57: text 224;
        94: varint}
  22.1.1: {1: the same 139-character text; 2: a UUID; 8: the same 224-character text}
  23: text 38 (two line feeds)
  27: {UUID, UUID, text 36 (one line feed), text 20, varints}
  28: the model and parameters message again
  36: {text 28, text 42, text 4}
  37: 64 hex digits (perhaps the search row's scope)
```

So the blob carries the prompt, a 224-character text (a summary or the reply's start; not the
1,971-character cached text), the repository, branch, model and ids, and not the whole
conversation; its fields need the descriptors of the 3.26.13 bundle to be named.

The remote workspace's `User/workspaceStorage/<id>/state.vscdb` held 13 `ItemTable` keys at the
run (`resource.authority.os.background-composer+bc-<uuid>`, `workbench.view.remote.state`,
`vscode.git`, `cursor/sandboxSupported` among them) and 31 after the reopening, among them
`workbench.backgroundComposer.workspacePersistentData`
(`{cachedInWindowBackgroundComposer{bcId, data{hasLocalChanges, status}, timestamp},
isSideBarExpanded}`); its `composerHeaders` and `cursorDiskKV` stayed empty.

#### Run mode and standing lists

The applicationUser blob's `composerState` holds the run mode per agent mode and the standing
lists. `modes4` has seven entries, `{id, autoRun, fullAutoRun, smartModeAutoRun, autoFix,
shouldAutoApplyIfNoEditTool, thinkingLevel, actionId, description, enabledMcpServers}` (on
Windows 3.26.13 also `enabledTools`; `enabledMcpServers` and `enabledTools` are the mode's MCP
server and tool lists, `[]` after the MCP steps), with
`id` `agent`, `plan`, `chat`, `triage`, `spec`, `debug` and `multitask`; on the new install
`agent`, `triage` and `multitask` had `autoRun` true. No run-mode string is stored: the mode is
derived from the three flags and the `ItemTable` key `cursor/sandboxAutoRunSettingEnabled`
(true after the switch to Allowlist (with Sandbox); `cursor/sandboxSupported` true since the
folder opened). On Linux (3.25.9, 2026-10-20) no sandbox is offered: `cursor/sandboxSupported`
was absent throughout, the new install's default was again Auto-review (`agent` `autoRun` and
`smartModeAutoRun` true, `triage` and `multitask` `autoRun` true), and the switch to Allowlist
set `smartModeAutoRun` false and wrote `cursor/sandboxAutoRunSettingEnabled` false:

| `autoRun` | `smartModeAutoRun` | `fullAutoRun` | sandbox key | Run mode (UI label) | Seen |
| --- | --- | --- | --- | --- | --- |
| false | any | false | any | Ask Every Time | bundle |
| true | false | false | absent or false | Allowlist | after the switch on Linux and Windows |
| true | false | false | true | Allowlist (with Sandbox) | after the switch |
| true | true | false | absent or false | Auto-review | the new-install default (macOS, Linux and Windows) |
| true | true | false | true | Auto-review (with Sandbox) | bundle |
| any | any | true | any | Run Everything | bundle |

The bundle (`workbench.glass.main.js`, 3.25.9) names the modes in two enums: `ASK_EVERY_TIME`
"Ask Every Time" ("Ask for permission before running each operation"), `USE_ALLOWLIST`
"Allowlist" / "Allowlist (with Sandbox)" ("Automatically run operations after you approve them
once"), `SMART_AUTO` "Auto-review" / "Auto-review (with Sandbox)" ("A classifier will run for
each action and decide whether it's safe to execute this command. Allowlists are still
respected.") and `RUN_EVERYTHING` "Run Everything" / "Run Everything (Unsandboxed)"
("Automatically run all operations without asking for permission"); and `ASK_EVERY_TIME`,
`YOLO`, `YOLO_WITH_SANDBOX`, `SMART_AUTO`, `SMART_AUTO_WITH_SANDBOX`, `FULL_YOLO`. The UI sets it
with the "Auto-Run" radio in settings, "Change run mode" in the agent pane, and the slash action
`/run-everything` ("Enable Run Everything for this agent"); an administrator can force it
(`defaultAgentRunMode`, e.g. `allowlist_without_sandbox`, `isAutoRunDisabledByAdmin`, refusals
"Run Everything is disabled by admin settings" and "... by permissions.json").

The standing lists and switches in `composerState`: `yoloCommandAllowlist` (`["curl", "head"]`
after Always run, `[]` before), `yoloCommandDenylist`, `smartAllowlistDenylist`,
`webFetchDomainAllowlist`, `autoApprovedModeTransitions` (all `[]`), and the booleans
`yoloEnableRunEverything` (false), `yoloDeleteFileDisabled` (false), `yoloMcpToolsDisabled`
(false), `yoloOutsideWorkspaceDisabled` (true), `autoApproveModeTransitions` (false),
`autoAcceptWebSearchTool` (false), `autoApplyFilesOutsideContext` (true),
`doNotShowYoloModeWarningAgain`, `doNotShowFullYoloModeWarningAgain`, `useLegacyTerminalTool`,
`playwrightProtection` and `mcpAuthBlocking`. Always run wrote only this list: no file changed.

On Linux (3.25.9, 2026-10-20) the editor window's "add python3 to the allow list" never reached
the list: `yoloCommandAllowlist` stayed `[]` from the choice (22:01Z) through a quit and a
relaunch (22:25Z), no other path in the blob and no other global or workspace `ItemTable` key
named the command, no `permissions.json` or project `.cursor/` held it, and
`yoloCommandDenylist`, `smartAllowlistDenylist` and `webFetchDomainAllowlist` stayed `[]`; the
call's `appliedAllowlistOptionIds` ([Tool bubbles](#tool-bubbles)) was its only record. Whether
the shortcut is held in memory for the session or was lost at the quit before the blob was
written is not known. Adding `python3` in Settings > Agent then stored `["python3"]`. The list
holds the user's own entries only: `ls`, which the settings page showed as a default entry, is
built in and not stored, as `cli-config.json` ships `Shell(ls)`. `User/settings.json` held only
`window.autoDetectColorScheme`: the list is not a VS Code setting.

On Windows (3.26.13, 2026-10-21) no sandbox is offered either: `cursor/sandboxSupported` was
absent, and the switch to Allowlist left `modes4` `agent` at `autoRun` true, `smartModeAutoRun`
false and `fullAutoRun` false and wrote `cursor/sandboxAutoRunSettingEnabled` false. Settings >
Agent persisted the list on 3.26.13 as on 3.25.9: `yoloCommandAllowlist` became
`["powershell"]`. The entry the user tried first, `powershell.exe`, was not in the stored list
and did not match the command line; `powershell`, the command's first token as typed, did, and
the command then ran with no prompt and a record with no `reviewData`. The prompt never offered
an allow-list entry for this command on Windows ([Tool bubbles](#tool-bubbles)), so whether the
prompt's shortcut persists on 3.26.13 was not tested. Before the MCP turn under Auto-review
(15:35Z) the user removed the `powershell` entry in Settings, and the list read `[]` after that
turn; `yoloMcpToolsDisabled` and `mcpAuthBlocking` kept their values.

#### Agent transcript JSONL

`~/.cursor/projects/<sanitized cwd>/agent-transcripts/<composerId>/<composerId>.jsonl`, written by
the worker beside each IDE turn (3.25.9 and 3.26.13). Line shapes:

- `{role: "user", message: {content: [{type: "text", text}]}}`; every user text block is the
  typed text in a fixed wrapper, `<timestamp>Monday, Oct NN, NNNN, N:NN PM
  (UTC-N)</timestamp>` LF `<user_query>Q</user_query>`: weekday name, abbreviated month, day,
  year, a 12-hour time with AM or PM and the UTC offset in parentheses, minute precision, in the
  local time zone (125 characters for a 37-character question); the query inside `<user_query>`
  is the user bubble's `text`;
- `{role: "assistant", message: {content: [...]}}` with blocks `{type: "text", text}` and
  `{type: "tool_use", name, input}`; `name` is a CamelCase tool name, not the snake_case
  `toolFormerData.name` (table below); `input` keys seen: `glob_pattern`;
  `namespace` and `toolName`; `arguments`, `mcpDetails`, `namespace` and `toolName` for an MCP
  call; `path`; `path`, `old_string` and `new_string`; `command`,
  `description` and, for a command that needed a sandbox exception, `required_permissions`
  (`["full_network"]`);
- `{type: "turn_ended", status}` (`success`); on Windows (3.26.13, the stopped "Try in chat"
  turn) also with `error`, `{error, status, type}`.

The user line's tag is the only time in the file; no line carries a model, tool result or
permission decision, so an approved and a skipped command have identical lines. The first
conversation's file grew from 3 to 28 lines over seven turns; the two conversations' files held
32 lines in all, 8 user, 16 assistant and 8 `turn_ended` (one per turn). On Windows (3.26.13,
2026-10-21) the file (`projects\c-Users-<user>-proj-demo\agent-transcripts\<composerId>\`, 300
bytes after the first question, 9,640 at the delete) also held the turns the agent injected as
user messages ([User bubbles](#user-bubbles)).

The transcript's tool names, matched to `toolFormerData.name` by their position in the round's
turns (one install, 3.25.9 and 3.26.13):

| Transcript `name` | `toolFormerData.name` |
| --- | --- |
| `CallDynamicTool` | `mcp-<server>-<tool>` (Windows, 3.26.13) |
| `GetDynamicTools` | `get_mcp_tools` |
| `Glob` | `glob_file_search` |
| `Read` | `read_file_v2` |
| `Shell` | `run_terminal_command_v2` |
| `StrReplace` | `edit_file_v2` |

The standalone CLI writes the same file under the same name, keyed by its session id
(2026.10.19-6bbf2c4, macOS, 2026-10-20): 3 lines (user, assistant, `turn_ended`) and 311 bytes
after one question, the same CamelCase tool names (`GetDynamicTools`, `Glob`, `Read`,
`StrReplace`, `Shell`), and no model. A command the user skipped has the same `tool_use` line as
one that ran, then the assistant's text and `turn_ended` with `status` `success`.

#### CLI chat store

`~/.cursor/chats/<md5 of cwd>/<sessionId>/store.db` (2026.10.19-6bbf2c4, macOS, 2026-10-20) has
two tables, `blobs(id TEXT PRIMARY KEY, data BLOB)` and `meta(key TEXT PRIMARY KEY, value TEXT)`.

`meta` holds one row, key `0`, whose value is hex-encoded JSON (678 hex digits for 339 bytes):
`{agentId, blobEncryptionKey, createdAt, isRunEverything, lastUsedModel, latestRootBlobId, mode,
name}`, with `agentId` a UUID, `blobEncryptionKey` 64 hex digits, `createdAt` a number,
`isRunEverything` a boolean (false throughout, after Add to the allow list too),
`lastUsedModel` the session's model id (`default`, then `grok-4.7` from the first message after
a switch), `latestRootBlobId` the 64-hex id of the root blob, `mode` (`default`) and `name` the
session's title (13 characters). The row is rewritten in place and keeps no history.

`blobs` is content-addressed: `id` is 64 hex digits (a sha256) and `data` 15 to 23,561 bytes.
The session held 11 blobs (4 JSON) after one question, 48 (13) after the edit, 53 (14) with a
permission prompt pending, then 64 (17), 80 (21), 96 (25) and 107 (29) after the later turns.
The blobs are not encrypted, although `meta.0` carries `blobEncryptionKey`. The JSON blobs are
model messages, `{role, content, providerOptions}` with `content` a string or a list of typed
blocks:

| `role` | `content` | Other keys |
| --- | --- | --- |
| `system` | a string, the system prompt (1,954 characters for the first) | `providerOptions.cursor.systemPromptFingerprint{agentType, model, mode, content, featureFlags, mcp, rules, toolNames, promptOptions}`, `agentType` `cli`; a second one was written after the model switch (`model` `grok-4.5` in the first, `grok-4.7` in the second) |
| `user` | a string, the context prompt (22,821 characters after the first question, about 23 KB) | `providerOptions.cursor{requestContextCompleteness{agentSkills, customSubagents, env, gitRepos, gitStatus, mcp, mcpFileSystem, repositoryInfo, rules}, omitCloudWorkerProcedure, useProjectCoordinatorPrompting, userInfoSummarizationEpoch}` |
| `user` | `[{type: "text", text}]`, the typed prompt in the wrapper of [Agent transcript JSONL](#agent-transcript-jsonl) (126 characters) | `providerOptions.cursor.requestId` (a UUID) |
| `assistant` | blocks `{type: "reasoning", text, signature, providerOptions.cursor.modelName}` (`signature` 664 characters, opaque; `modelName` the resolved model id), `{type: "text", text}` and `{type: "tool-call", toolCallId, toolName, args}` (`toolCallId` 85 characters) | `id` (40 characters), `providerOptions.cursor.modelProviderMessageId` |
| `tool` | `[{type: "tool-result", toolCallId, toolName, result, experimental_content[{type: "text", text}]}]`, `result` a string | `id` (the `toolCallId`), `providerOptions.cursor.highLevelToolCallResult{isError, output}` |

The `tool-call` names are the transcript's CamelCase ones, with their `args`: `Glob`
(`glob_pattern`), `GetDynamicTools` (`namespace`, `toolName`), `Read` (`path`), `StrReplace`
(`old_string`, `new_string`, `path`) and `Shell` (`command`, `description`).
`highLevelToolCallResult.output` holds one of:

- for a file tool that ran, `success{path, content, totalLines, readRange{startLine, endLine},
  files, totalFiles, diffString, beforeFullFileContent, afterFullFileContent, linesAdded,
  linesRemoved, fileSize, message}`, an edit's full file text before and after included;
- for a `Shell` call that ran, `success{command, stdout, interleavedOutput, executionTime,
  localExecutionTimeMs}`, with `isBackground`, `sandboxPolicy{type}` and `terminalsFolder` beside
  it in `output` and `isError` false;
- for a call the user skipped, `rejected{command, reason, workingDirectory}`, with `isError` true
  and the block's `result` a 17-character string.

No JSON message has a time key. The instruction the user typed after a skip is a `user` text
message after the tool result. A call waiting at the permission prompt is not yet a message:
while it waited, only the new user message and four protobuf blobs had been written.

The other blobs are protobuf (first byte `0x0A`) and form a hash tree from `latestRootBlobId`
down to the message blobs: they name other blobs by raw 32-byte hashes (field 1 repeated, fields
8 and 10) and hold varints, 36-character ASCII strings (UUIDs) and shorter ASCII strings. One
message type, with a 1,063-character ASCII field 4 and fields 9, 21, 22, 27 and 38, recurs in
nodes of 614 and 1,346 bytes; the 614-byte node's field 5 holds a table of name pairs. The larger
nodes (2.1 to 3.1 KB) are snapshots of the agent's state that embed JSON text, in which occur the
identifiers `pendingToolCallStartedAtMs`, `pendingToolExecutionContracts`, `allowedToolNames`,
`toolIdentifier`, `outerToolName`, `isDynamic`, `summarized_conversation`, `system_prompt`,
`modelName` and `modelProviderMessageId`, the tool names `StrReplace`, `GetDynamicTools`,
`CallDynamicTool`, `TodoWrite`, `GET_MCP_TOOLS` and `STR_REPLACE`, the local time zone's
database name and reasoning signatures. No descriptor has been extracted, so the nodes are not
decoded.

#### CLI session and trust files

Beside `store.db` in `chats/<md5 of cwd>/<sessionId>/` (2026.10.19-6bbf2c4, macOS, 2026-10-20):

- `meta.json`: `{createdAtMs, updatedAtMs, cwd, hasConversation, schemaVersion}` (129 bytes) on
  a start that sent nothing, `hasConversation` false and no `store.db`; `title` (the automatic
  title) is added once a prompt is sent (152 bytes), `hasConversation` true. `cwd` is absolute;
  `updatedAtMs` moves with each turn. The bundle's `isSubagent` key was not seen.
- `prompt_history.json`: a JSON array of the strings the user typed in the session, one per
  prompt, the instruction typed after a skip included (6 after six prompts); no times.

In `~/.cursor/projects/<sanitized cwd>/`:

- `.workspace-trusted` (89 bytes): `{trustedAt, workspacePath}`, an ISO 8601 time and the absolute
  path, written when the user accepted the CLI's workspace-trust prompt; one per project. The CLI
  asked once per folder: started from the home it asked to trust the home, and in the project it
  did not ask again after the first acceptance.
- `repo.json`: `{id}`, a UUID.

### Joins and timestamps

The `composerId` joins `composerHeaders`, `composerData`, every `bubbleId:`,
`checkpointId:`, `ofsContent:`, `composerConvHeaders:`, `composerDraft:` and
`composerRootWriterDiagnostic:` key, the transcript JSONL's directory and file name,
`ai_code_hashes.conversationId`, the `conversation-search.db` `conversations.id`, the
`AgentStores` per-composer store and `.control/<composerId>/`. Bubble order is the header list's;
a tool call and its result are one bubble, keyed by `toolFormerData.toolCallId`; the
`inlineDiff` row names its `composerId` and `toolCallId`. A user bubble's `requestId` names the
retrieval checkpoint (`metadata.json` `agentRequestId`) and `ai_code_hashes.requestId`; its
`checkpointId` names the `checkpointId:` row. The transcript JSONL joins by composer id (its
directory and file name) and, within it, by text: no line has a bubble id or a tool-call id. A
user line's `<timestamp>` tag gives the turn's local time to the minute with its UTC offset, while
the user bubble's `createdAt` has milliseconds; assistant and `turn_ended` lines have no time.

Times: header entries and bubbles have `createdAt` (ISO 8601 with milliseconds) and `startedAtMs`
and `completedAtMs` (epoch ms); `composerData` and `composerHeaders` have `createdAt` and
`lastUpdatedAt` as epoch ms. `lastUpdatedAt` is the last time the composer was touched, not its
last message: on the header row an archive and a delete moved it, a model switch did not; the
3.25.9 archive on Linux left `composerData.lastUpdatedAt` at the last turn, so there the archive
time is on the header row only. The last message's time
is the last header entry's `startedAtMs` or `completedAtMs`. A resume writes nothing to the
conversation: after a quit, two relaunches and the conversation reopened, the bubbles,
`status`, `lastUpdatedAt` and headers were unchanged and `lastReadAtMs` stayed the last bubble's
`startedAtMs`; a relaunch shows only as a `logs/<launch stamp>/` directory and new drafts. The
same held on Linux (3.25.9), where the quit also checkpointed the WAL into `state.vscdb` (3.0 MB)
and removed the `workspaceStorage/<epoch ms>/` of an earlier untitled window, and on Windows
(3.26.13), where the 46 bubbles and `lastUpdatedAt` were unchanged after the reopening.

A cloud agent's `bc-<uuid>` joins its `cloudAgentRepository.agents.<account>` element
(`bcId`), the `conversation-search.db` `conversations.id`, the `bcCachedDetails:` and
`composerTranscriptHeightTail:` keys, the `glass.agentConversationPanes.` and pane keys, the key
of `glass.cloudAgentProjectMembership.v1`, the remote workspace's authority
`background-composer+bc-<uuid>` and the `connectionToken:bc-<uuid>` secret; the repository joins
on `repoUrl` and the repository key names ([Cloud-agent keys](#cloud-agent-keys)). Its times are
the agent record's `createdAt`, `updatedAt`, `lastMessageActivityAtMs`, `lastReadAtMs` and
`rowUpdatedAtMs` (an archive moved only the last) and the search row's `updated_at` (13:09:09Z on
the install, unmoved by the archive). The cached text has no per-turn time
and no boundary between the prompt and the reply, so the cloud conversation cannot be split into
turns locally.

A CLI session's id is its `chats/<md5>/<sessionId>/` directory name, the
`agent-transcripts/<sessionId>/` directory and file name, and `ai_code_hashes.conversationId`;
the md5 is of the absolute cwd (checked on the install), which `meta.json` `cwd` names, and the
transcript's `projects/` directory is the same cwd sanitized. A tool call and its result join on
`toolCallId`, which is also the `tool` message's `id`. Times: `meta.json` `createdAtMs` and
`updatedAtMs` and `meta.0.createdAt` per session; no JSON message has a time; the protobuf state
nodes carry `pendingToolCallStartedAtMs` and varints that may be times; the transcript's user
lines date each prompt to the minute. Message order is the hash tree's, readable only once the
nodes are decoded; until then the transcript's order is the one available. A quit and `agent
resume` wrote nothing to the session (blobs, `meta.json` and `prompt_history.json` unchanged,
no resume marker); the quit checkpointed the WAL into `store.db` (4 KB to 200 KB, the WAL 1.5 MB
to 4 KB).

### Other stores

#### `conversation-search.db`

`User/globalStorage/conversation-search.db`: `conversations(fts_rowid, source, scope, id, title,
branches, updated_at, is_archived, root_fingerprint, cache_fingerprint)` with a CHECK constraint
`source IN ('local','cloud-cache')`, the FTS5 (SQLite's full-text search module) table
`conversation_fts(title, body, branches)` with its shadow tables (`_idx`, `_config`, `_content`,
`_data`, `_docsize`), `conversation_search_candidates(id, updated_at)`,
`conversation_search_reconciliation(cursor, in_progress)` and
`conversation_search_settings(effective_conversation_cap)` (10000). The shadow table
`conversation_fts_content(id, c0, c1, c2)` holds the indexed text, `c0` the title, `c1` the body
and `c2` the branches, and joins `conversations` on `conversations.fts_rowid =
conversation_fts_content.id`. One `conversations` row per composer, drafts included (an empty
title); `updated_at` is the composer's `lastUpdatedAt` in epoch ms, refreshed at launch, and
`is_archived` follows an archive. A `local` row has `scope` `''` and a `root_fingerprint`; a
`cloud-cache` row, a cloud agent's ([Cloud agents](#cloud-agents)), has a `scope` of 64 hex
digits, a 64-character `cache_fingerprint`, `id` the `bc-<uuid>` and `branches` `''`. On the
macOS install the content held the conversation's text (42 characters of body in all after the
first question and its answer). On the Windows install (3.26.13, 2026-10-21) the three `local`
rows of the morning's steps had empty content (title and body 0 characters, before and after the
delete), while the first `cloud-cache` row held a 36-character title and a 1,971-character body
of 21 lines, plain turn text with the user's prompt once and no `**User**` or `**Cursor**`
markers, unchanged by the conversation's reopening and by its archive. Later `local` rows held
text too: the "Try in chat" conversation's a 28-character title and a 252-character body, and
the MCP conversation's a 21-character title and an 806-character body. So some local rows have
no body, and a body is not particular to cloud agents. A delete removes a local row
([What survives a delete](#what-survives-a-delete)).

#### `ai-code-tracking.db`

`~/.cursor/ai-tracking/ai-code-tracking.db`: `ai_code_hashes(hash, source, fileExtension,
fileName, requestId, conversationId, timestamp, model, createdAt)` (2 rows after the edit:
`source` `composer`, `conversationId` the composer id), `scored_commits(commitHash, branchName,
scoredAt INTEGER, linesAdded, linesDeleted, tabLinesAdded, tabLinesDeleted, composerLinesAdded,
composerLinesDeleted, humanLinesAdded, humanLinesDeleted, blankLinesAdded, blankLinesDeleted,
commitMessage, commitDate, v1AiPercentage, v2AiPercentage)` (the line counts INTEGER;
`commitMessage`, `commitDate` and the two percentages TEXT), `tracking_state(key, value)` (one
row, `trackingStartTime`),
`conversation_summaries(conversationId, title, tldr, overview, summaryBullets, model, mode,
updatedAt)`, `tracked_file_content(gitPath, content, conversationId, model, fileExtension,
createdAt)` and `ai_deleted_files(gitPath, composerId, conversationId, model, deletedAt)`. Only
`ai_code_hashes` and `tracking_state` had rows on the install. `ai_code_hashes.timestamp` is
epoch ms (a 13-digit integer); the unit of `createdAt` was not recorded. The standalone CLI
writes the same database with the same tables (2026.10.19-6bbf2c4, 2026-10-20): after its edit,
2 `ai_code_hashes` rows with `source` `cli`, `conversationId` the CLI session id and a model,
one `tracked_file_content` row and one `tracking_state` row. `source` tells the CLI's rows from
the IDE's `composer` rows.

#### Retrieval checkpoints

`User/globalStorage/anysphere.cursor-retrieval/checkpoints/<requestId>/` on the macOS install
(3.25.9) and `User/globalStorage/anysphere.cursor-commits/checkpoints/<requestId>/` on the Linux
install (3.25.9; `metadata.json` 562 bytes, a 32-byte file copy and a 567-byte diff for a
one-function edit), with the same layout, one per agent request that edited a file: `metadata.json`
(`{agentRequestId, workspaceId, startTrackingDateUnixMilliseconds, fileSizeBytes,
requestFiles[{fileUuid, fsPath, gitInfo{gitInfo{branch, commitHash, gitRoot}}}],
deduplicatedGitInfos}`), `files/<fileUuid>` (the file before the edit) and `diffs/<fileUuid>`
(`{fsPath, fileUuid, kind, numLines, fileSizeBytes, gitInfo, diffChanges[{original and modified line
ranges, addedLines, tokenizedAddedLines}]}`). The composer id is not in it. The Windows install
(3.26.13, 2026-10-21) wrote `anysphere.cursor-commits\checkpoints\<requestId>\` too
(`metadata.json` 324 bytes, a 32-byte file copy, a 475-byte diff); the macOS 3.26.13 install's
directory name was not checked, so which builds use which name is not known.

#### Workspace `state.vscdb`

`User/workspaceStorage/<id>/state.vscdb` `ItemTable`, 12 to 18 keys: `aiService.prompts` and
`aiService.generations` (both `[]` throughout the round), `anysphere.cursor-retrieval`,
`cursor/agentLayout.sidebarLocation`, `cursor/sandboxSupported`,
`cursorAuth/workspaceOpenedDate`, `cursorHooks/lastDiskHooksLoadHadHooks`, `history.entries`,
`terminal.integrated.environmentVariableCollectionsV2`, `vscode.git`,
`workbench.view.remote.state` and
`workbench.customize.primitiveSourceSnapshot.{commands,rules,skills,subagents,mcps}.v3`.
`User/workspaceStorage/<id>/workspace.json` is `{folder}` (51 bytes for a local folder on
Windows); a cloud agent's remote workspace is in [Cloud-agent keys](#cloud-agent-keys).

#### `cli-config.json`, `permissions.json` and `sandbox.json`

`~/.cursor/cli-config.json`, the CLI's settings, written by the IDE, which left it unchanged at
its run-mode switch and Always run, and by the standalone CLI, which rewrites it at every start.
Keys at the first start, on both surfaces: `approvalMode` (`allowlist` throughout),
`permissions{allow, deny}` (`allow` `["Shell(ls)"]`, the 9-character entry; `deny` `[]`),
`sandbox{mode, networkAccess}` (`disabled`, `user_config_with_defaults`),
`attribution{attributeCommitsToAgent, attributePRsToAgent}`, `autoAcceptWebSearch`,
`display{mode}` (`zen`), `exploreSubagentModel` (`default`), `hasChangedDefaultModel`, `rewind`,
`steering` and `version`, and from the CLI also `modelSlashCommands` and
`runEverythingSettingsPromptStreak`.

What the CLI added (2026.10.19-6bbf2c4, macOS, 2026-10-20): at sign-in `authInfo` and
`privacyCache` ([Credentials](#credentials)), removed again by `agent logout`; at the first
start in a project `model{aliases, displayModelId, displayName, displayNameShort, maxMode,
modelId}` (`modelId` `default` with `aliases` `["auto"]`, the selection the CLI shows as Auto),
`selectedModel{modelId, parameters[{id, value}]}`, `modelParameters{<modelId>: [{id, value}]}`,
`modelSelectionHistory` (model ids, most recent first, no times),
`serverConfigCache{agentUrlConfig{agentUrl, agentnUrl}, authCacheKey, backendUrl,
cliSandboxDefaultEnabled, serverHttp2Config, updatedAt}` and
`autoReviewAvailabilityCache{authCacheKey, available, backendUrl, updatedAt}`. Add python3 to the
allow list appended `Shell(python3)` to `permissions.allow`, an entry of the form
`Shell(<command name>)`; Allow once and Skip changed nothing. The `/model` switch to "Grok 4.7 256K
High Fast" set `model.modelId` and `displayModelId` to `grok-4.7`, `aliases` to `[]`,
`selectedModel.parameters` to `context` `256k`, `reasoning_effort` `high` and `fast` `true`,
`modelParameters.grok-4.7` to the same three, `modelSelectionHistory` to `["grok-4.7",
"default"]` and `hasChangedDefaultModel` to true. Sizes: 837 bytes at the first start, 1,101
signed in, 2,048 after the start in the project, 2,072 after the allow-list entry, 2,549 after the
switch and 2,285 after `agent logout`.

`permissions.json`, user and project level (bundle-derived, not observed on any real install;
`workbench.glass.main.js`, 3.25.9): `allowlists{mcp, terminal}`, `approvalMode`, and auto-run
instructions `{allowInstructions, blockInstructions}`; the bundle also names `terminalAllowlist`
and `approvalMode` as field sources. `sandbox.json`: name only, from the renderer log.

#### AgentStores

`AgentStores/cursor_agent_stores/` in the app-data directory on macOS, and
`~/.local/state/cursor/agent-stores/cursor_agent_stores/` on Linux (3.24.18 and 3.25.9, 2026-10-20),
holds file-sync stores for agent assets such as skills, one per account (`u<id>/`, from sign-in; a
second after the account merge) and one per conversation (`<composerId>/`, from the first edit on
macOS and from the first question on Linux): `.sync/index.sqlite`, `.sync/root-id` (36 bytes),
`.sync/root-id.lock`, `.sync/sync.lock` (`{acquiredAt, hostname, kind, pid, uid, windowId}`, `kind`
`ide`), `.sync/mount.json` on a conversation's store (`{kind, mountKey, readOnly, version}`, `kind`
`self`), `.sync/tmp/` and `files/` (`files/skills/` on the account's). `index.sqlite` has `meta(key,
value)` (`deletion_armed`, `files_root_birthtime_ms`, `files_root_dev`, `files_root_ino`,
`index_created_ms`, `last_complete_round_ms`, `last_listing_applied_ms`, `remote_version`,
`remote_version_listed_ms`, `root_id`, `schema_version`, `tombstone_cursor_ms`,
`tombstone_last_include_ms`, `tombstone_rounds_since_include`), `files(rel_path, last_synced_sha,
last_seen_etag, size, last_synced_ms, direction, state, tombstone_etag, deleted_at_ms, stat_*)`
(`direction` `pulled` or `pushed`, `state` `live` or `tombstoned`), `pending_deletes` and
`pending_rmdirs`. Every `files` table stayed empty through the macOS round. `.control/<u<id> or
composerId>/pins/<pid>-<uuid>.json` (`{createdAtMs, hostname, pid, purpose, uid, version,
windowId}`, `purpose` `foreground`; 166 bytes on Linux) pins a store per open window or
conversation, and `.store-eviction-last-run` holds a number.

On Windows (3.26.13, 2026-10-21) the stores are under
`%LOCALAPPDATA%\Cursor\AgentStores\cursor_agent_stores\`, from sign-in: `u<id>\.sync\{index.sqlite,
root-id, root-id.lock, sync.lock, tmp\}`, `u<id>\files\skills\`, `.control\u<id>\pins\`,
`.store-eviction-last-run`, and per conversation `<composerId>\files\` and
`.control\<composerId>\pins\`, the latter from the first question, as on Linux. `index.sqlite` was
written at sign-in, as on macOS, and had the same tables, its `files` table empty (36 KB with a
4.1 MB WAL by the cloud agent's step). The pin files carry `sid` (46 characters) where the other
platforms' carry `uid`, `{createdAtMs, hostname, pid, purpose, sid, version, windowId}` (214
bytes, `hostname` 14 characters, `purpose` 10, `windowId` 41), and they accumulate across
launches, one per window and process (7 by the cloud agent's step, 11 an hour later);
`sync.lock` is `{acquiredAt, hostname, kind, pid, sid, windowId}`.

#### `update-supervisor`

`update-supervisor/<install uuid>/`, one per in-app update: `events.jsonl` (17 lines per update;
every line has `installationId`, `phase`, `recordedAt`, `schemaVersion` and `type`, which is one
of `supervisor_started{currentVersion, targetVersion, oldMainPid}`, `phase_changed{from, to,
reason}`, `shipit_state_changed`, `shipit_launchd_changed`, `shipit_install_started`,
`shipit_install_completed`, `shipit_relaunch_progress{line}`, `old_app_exited`,
`bundle_swapped{previousInode, currentInode, version}` and `new_app_ready`), `result.json`
(`{completedAt, details{currentVersion, elapsedMs}, finalPhase, installationId, schemaVersion,
status}`, `finalPhase` `confirmed`, `status` `success`), `app-ready.json` (`{currentVersion, pid,
readyAt}`) and the `cursor-update-supervisor` binary. Together they date each build the install
ran. The Linux deb has none ([Cursor IDE on Linux](#cursor-ide-on-linux)).

On Windows (per-user installer, 2026-10-21) the directory depends on the build the update brings.
The in-app update run on 3.26.13 ran Inno Setup alone: it relaunched the app at 00:52Z with a new
log directory, left the build at 3.26.13 (`package.json` and the `Cursor.exe` product version
both 3.26.13) and wrote only its log, `cursor-inno-updater-<epoch s>.log` (3.5 MB), and no
`update-supervisor\` ([Cursor IDE on Windows](#cursor-ide-on-windows)). The update to 3.26.16
installs in the background and the restart only switches to it. Inno Setup ran at 15:42-15:44Z
while the running app was still 3.26.13: it rewrote the install directory
`%LOCALAPPDATA%\Programs\cursor\` (`resources\app`, `unins000.exe`, `unins000.dat`,
`unins000.msg`, `tools\` and `_\`, Inno's staging directory for the old files), set the
Uninstall entry's version to 3.26.16, and created `%TEMP%\Setup Log 2026-10-21 #001.txt` (0
bytes) and `%TEMP%\vscode-stable-user-arm64-<id>\`, the downloaded installer's extraction; no
scheduled task. The restart that the app offered then (15:59-16:01Z) removed both temporary
entries, rewrote the install directory, grew the Inno log to 4.95 MB and wrote
`update-supervisor\win32\<installationId uuid>\` under `%APPDATA%\Cursor\`, the path the
catalog's `AppData/Roaming/Cursor/update-supervisor` line names, with `app-ready.json` (116
bytes, `{currentVersion, installationId, schemaVersion}`, `currentVersion` `3.26.16`),
`state.json` (104 bytes, `{installationId, schemaVersion, stage}`, `stage` 18 characters) and
`inno_updater.exe` (3,417,896 bytes, its modification time the build time), and nothing else.
So the supervisor came with the update that brought 3.26.16, and the Windows updates of earlier
builds ran Inno Setup alone; the 3.26.16 build itself was not surveyed.

#### Exported Markdown

Export (editor window, 3.26.13, 2026-10-20) writes one Markdown file where the user saves it,
default name `export.md` (1,194 bytes and 89 lines for seven turns): `# <conversation title>`, a
line `_Exported on <M/D/YYYY at HH:MM:SS> <time zone> from Cursor (3.26.13)_`, then alternating
`**User**` and `**Cursor**` blocks separated by `---`. It has no per-turn time, tool call, tool
name, code fence, model or composer id: the prose only. The Linux export (editor window, 3.25.9,
2026-10-20) had the same shape, its line naming `Cursor (3.25.9)` (1,243 bytes and 101 lines for
eight turns, 16 `---` separators). The Windows export (editor window, 3.26.13, 2026-10-21) had it
too: 3,433 bytes, 144 lines with LF line endings, 7 `**User**` and 7 `**Cursor**` blocks, for a
conversation of 5 typed prompts and 3 injected user turns ([User bubbles](#user-bubbles)); how
its blocks map to those turns was not determined. Its save dialog wrote only Chromium state
(`blob_storage`, `Crashpad\settings.dat`, `Local Storage\leveldb\LOG`). Nothing in the app's
stores records an export; macOS's
recent-documents list ([Cursor IDE](#cursor-ide)) and GNOME's recent-files list
([Cursor IDE on Linux](#cursor-ide-on-linux)) named the file.

#### Source-host integration

Cursor connects to GitHub in two ways that store differently (Linux, 3.25.9, 2026-10-20;
Windows, 3.26.13, 2026-10-21):

- Cursor's own GitHub integration installs the Cursor GitHub app on the account for cloud agents
  and keeps the grant on the server: connecting it wrote the connect flow's record and empty
  repository and agent caches, named no account and added no `secret://` row
  ([`ItemTable` keys](#itemtable-keys)). No GitHub token is stored for it.
- The editor's own GitHub sign-in (VS Code's GitHub authentication, reached through Welcome >
  Clone a repository > Clone from GitHub; Cursor's Accounts menu has no GitHub entry) names the
  account in key names, records the scopes in a key name and the extensions that used it, and
  stores an encrypted token ([`ItemTable` keys](#itemtable-keys), [Secrets](#secrets)). On
  Windows it wrote no git credential store ([Credentials](#credentials)).

The repository list is Cursor Origin's, not GitHub's
([Cursor Origin](https://cursor.com/docs/origin), L2, fetched 2026-10-21). Origin is Cursor's
own git hosting, in early beta: a repository is hosted there or mirrored in from GitHub ("Sync
from GitHub" on [Cursor's repository list](https://cursor.com/codebase), "Mirror a GitHub
repository"), with
GitHub the source of truth and Origin the mirror; Origin follows the namespace owner's Privacy
Mode, has a CLI of its own, and cloud agents and automations can be connected to its
repositories. Installing the Cursor GitHub app alone lists no repository in the IDE, which is
why the Linux round's picker was empty: `cloudEnvironmentTemplate.githubRepos` stays `[]` until a
repository is mirrored into Origin, and then holds one entry per repository
([Cloud-agent keys](#cloud-agent-keys)). Origin is also an MCP server
(`logs/<stamp>/mcp-server-cursor-origin.log`: `connecting streamableHttp for "Origin"
(cursor-origin)`, `Successfully connected to streamableHttp server`), whose tool manifests the
IDE writes under `~/.cursor/projects/<p>/mcps/cursor-origin/tools/` (34 files on Windows, among
them `list_repositories`, `list_namespaces`, `get_repository`, `create_repository`,
`sync_mirror`, `list_branches`, `list_commits`, `commit_read`, `compare_commits`,
`get_git_tree`, `get_file_contents`, `grep_contents`, `checks_read`, `list_pull_requests`,
`pull_request_read`, `create_pull_request`, `update_pull_request`, `update_pull_request_base`,
`merge_pull_request`, the pull-request review, comment, review-thread and reviewer tools,
`create_label` and `update_pull_request_labels`).

A repository-bound cloud agent was observed on Windows (3.26.13, 2026-10-21), on a throwaway
repository mirrored into Origin, with a read-only task that opened no pull request. What it left
on the host, and what a responder can read there, is in [Cloud agents](#cloud-agents): the
connected GitHub account is named nowhere by it, the repository is named in the agent's record,
in the repository list and in key names, and the conversation is held locally only as cached
text beside its title, repository, branch, model, status and times; its full store is on the
agent's machine. Archiving it in the Agents window flipped two flags and removed nothing, and
the Agents window offers no Delete ([What survives a delete](#what-survives-a-delete)).

#### MCP servers and plugins

The plugin library is Cursor's catalogue of plugins, packages that add MCP servers to the agent,
opened from the Agents window; each plugin page has a "Try in chat" button, which opens a chat
with a prompt the plugin supplies. On Windows (3.26.13, 2026-10-21) the Google Calendar plugin,
from the `cursor-public` marketplace, was installed from the Agents window with no folder open
(14:58Z). Its manifest names one HTTP MCP server, which signs in to Google through OAuth. The
install wrote, before any authentication:

- the marketplace copy under `~/.cursor/plugins/cache/` and the per-project manifest
  `~/.cursor/projects/empty-window/mcps/plugin-google-calendar-google-calendar/`, its one tool
  `mcp_auth` ([`~/.cursor` as the IDE writes it](#cursor-as-the-ide-writes-it));
- the installed set in `state.vscdb` `ItemTable`, `cursor.plugins.installedIds.<team>|<scope>`
  (`no-team|no-workspace` here), `[{id, sources}]` (38 bytes, `id` 8 digits, `sources`
  `["user"]`); there is one such key per open folder beside it, with the folder's file URI as
  the scope ([Where the project path is recorded](#where-the-project-path-is-recorded)), and the
  demo folder's became 38 bytes at the authentication, the plugin then enabled for it too;
- the OAuth attempt, `User/globalStorage/mcp-oauth-attempts/<uuid>.json` (772 bytes, plaintext):
  `{activeIdentifier, attemptId, authorizationServerUrl, clientInformation{client_id,
  grant_types, redirect_uris, response_types}, createdAtMs, identifier, owner{workspaceId},
  serverUrl, transportServerUrl}`, `activeIdentifier` 82 characters, `attemptId` a UUID,
  `client_id` 72, `grant_types` two strings of 18, `redirect_uris` two URLs, `response_types`
  `["code"]`, `identifier` 67 and `owner.workspaceId` `empty-window`; `clientInformation` is the
  result of OAuth dynamic client registration, in which the app registers itself with the
  authorization server at run time and receives a client id;
- the OAuth keys of `ItemTable`, `mcpOAuth.global.<b64>` (plaintext) and `mcpOAuth.secret.<b64>`
  (ciphertext), whose suffix is URL-safe base64 without padding of `[<scope>] <field>`; the
  scope is `plugin-<plugin>-<server>::mcpScope:profile:<base64 of the profile name>` (profile
  `default`) for the server or `oauth-attempt:<uuid>` for an attempt. The global fields are
  `mcp_server_url` (a 43-byte URL), `mcp_oauth_updated_at_ms` (a number) and the booleans
  `mcp_tokens_canonical_migration_complete` and
  `mcp_client_information_canonical_migration_complete`, beside the key
  `mcpOAuth.global.<base64 of "mcpOAuth.extensionMigrationComplete">` (a boolean). The secret
  fields are `mcp_code_verifier` (the OAuth code verifier, under both the attempt and the server
  scope), `mcp_oauth_attempt_client_secret` and `mcp_client_information`, so a verifier and a
  client secret exist before the user authenticates ([Secrets](#secrets)).

Completing the authentication through the browser (15:05Z) added `mcp_tokens` under the server's
scope, rewrote the server scope's `mcp_code_verifier` and `mcp_client_information` and the
attempt's `mcp_oauth_attempt_client_secret`, removed the first attempt's `mcp_code_verifier`
key, and replaced the attempt file with a new `<uuid>.json` (717 bytes, the same keys without
`authorizationServerUrl`). A reconnect of the server rewrites it again: a third file replaced the
second at 15:22Z. So there is one attempt file per server at a time, and a completed attempt's
record is dropped. The per-project manifests of the empty window and of the open folder lost
`STATUS.md` and `tools/mcp_auth.json` and gained the server's nine tool schemas. No token was
written in plaintext, to a file or to the Credential Manager. The logs
`workbench.mcp.oauth.log` and `MCP Logs` ([Logs](#logs)) were not re-read for these steps.

The "Try in chat" button (about 15:06Z) opened a local agent chat in the empty window with the
plugin's 106-character prompt, model `grok-4.7`; its tool calls were `get_mcp_tools`, two
`glob_file_search`, `read_file_v2` and a `run_terminal_command_v2`, which the user stopped
([Tool bubbles](#tool-bubbles)), and the conversation was archived. Its turn saved the
`get_mcp_tools` output under `~/.cursor/projects/empty-window/agent-tools/`. The user's next chat
from the Agents window ran as a cloud agent in a new temporary Origin repository, and the cloud
agent's MCP authorisation is a separate grant held on the server
([Cloud agents](#cloud-agents)): one install can leave two grants on the Google account, the
local client's tokens in `state.vscdb` and the cloud agent's, of which the host keeps only the
`cloudAgentRepository.mcpAuthResolution` decision record. A responder revoking the access looks
for both.

The calendar question in the demo folder's local chat (15:21Z to 15:35Z, model `grok-4.7`) used
the plugin's tools under two run modes in one conversation: under Allowlist the user was asked to
approve each MCP call and approved each (the turn's calls were `get_mcp_tools`, `list_events`,
a second `get_mcp_tools`, `list_calendars` and three more `list_events`), and after the switch to
Auto-review two `list_events` calls ran with no prompt; no bubble records any of those approvals
([Tool bubbles](#tool-bubbles)). The worker's transcript names the calls `GetDynamicTools` and
`CallDynamicTool` ([Agent transcript JSONL](#agent-transcript-jsonl)). Its `get_mcp_tools` output
was not saved under `agent-tools/` for the demo folder.

#### Remote control

Remote control lets the Cursor iOS app ("Steer from iOS") follow and drive the agent turns of
an IDE install. The switch, agent remote control under Settings > Cursor Settings, was on by
default with no paired device (Windows, 3.26.13, 2026-10-21), and the install was enrolled as a
remote agent host for the account without any step of the user's: the enrolment keys of
[`ItemTable` keys](#itemtable-keys) and the computer key of [Credentials](#credentials). On Linux
(3.25.9, 2026-10-20) the computer key was on disk after the first agent turn and the three
`remoteAgentHost.*` keys were first seen after the editor's GitHub sign-in; on Windows all four
were on disk by the delete step, long before the editor's GitHub sign-in, and the step that
first wrote them is not known.

Pairing the Cursor iOS app to the install (Windows, 3.26.13, 2026-10-21) wrote no key, no file
and no log line: the four keys kept their sizes (14, 4, 36 and 770 bytes), no `ItemTable` or
`cursorDiskKV` key and no file was added, and no log of any launch stamp under
`%APPDATA%\Cursor\logs\` mentions `remoteAgentHost`, `steerFrom`, `paired` or `pairing`; the
listing's other changes were the window state of the clone opened before it. The paired device
is account state on the server, and the enrolment is the only local record of remote control.

A steered turn runs through the embedded agent worker (`cursor-agent-worker`,
[Cursor IDE on Windows](#cursor-ide-on-windows)). On Windows arm64 the phone saw the install but
could not connect: its attempt (14:35Z) opened a new window on the project (`window2_wb3`), and
the worker's log,
`exthost\anysphere.cursor-agent-worker\Cursor Agent Worker.workspaceId-<id>.<stamp>.log`
(1749 bytes), records, with paths and ids redacted,
`[cursor-agent-worker] Activating`, `Resolving installer for windows/arm64`,
`Using cached Cursor Agent 2026.10.19-6bbf2c4`, `CLI install: prod:2026.10.19-6bbf2c4`,
`Installed CLI at <path>\node` (the worker ships its own Node), `Workspace roots: <project path>`
(three times), `Spawning worker "<project path> @ <hostname>" (name=<id>) with endpoint URL`,
`Started daemon "<name>" pid=<n> from <project path>`, `Logs: <path>\<name>.log` (the worker's
own log, which holds the native-module crash of [Cursor IDE on Windows](#cursor-ide-on-windows)),
`Pushed worker id "<uuid>" to browser service` and `Worker exited (code=1, signal=null)`. The
worker's name carries the project path and the host name ([Secrets](#secrets)). A relaunch
(14:37Z) and another spawn attempt, which rewrote the worker's control files (14:44Z), added no
key or file. Whether a steered turn is marked in the bubbles or the transcript is therefore not
known; the worker ran on macOS arm64 and Linux arm64 ([Observed](#observed)).

#### Logs

`logs/<launch stamp>/` (names only): `main.log`, `sharedprocess.log`, `composer.log`,
`conversationSearch.log`, `cursor-sentry-events.log`, `mcpprocess.log`,
`mcp-server-cursor-origin.log`, `cursorAuthDebug`, `cursor.requests`, and per window
`window<n>_wb<m>/`: `renderer.log`, `workbench.mcp.{allowlist,files,oauth}.log`,
`mcp-server-cursor-{origin,app-control,ide-browser,subscriptions}.workbench.log`,
`output_<stamp>/{cursor.hooks.workspaceId-<id>.log, cursor.requestTraces.log,
cursorAuthDebug.workspaceId-<id>.log, tasks.workspaceId-<id>.log}` and
`exthost/anysphere.cursor-{agent-exec,agent-worker,agent-host,always-local,mcp,retrieval,socket}/`
(`Cursor Agent Exec`, `Cursor Plugins`, `Mcp FileSystem Writer`, `Cursor Structured Logs`,
`Filesync`, `Cursor Indexing & Retrieval`, `MCP Logs`, and `Cursor Agent Host{, Network, Network
(debug), Storage}.workspaceId-<id>.<stamp>.log`).

On Windows (3.26.13, 2026-10-21) the round left six launch directories (4.0 MB). The renderer
log of the window that opened a cloud agent (`window2_wb2/renderer.log`) records the attach:
`Invoking resolveAuthority(background-composer)...` and `returned 'Managed(1)'`,
`[WorkspaceConnectionService] Promoted workspace to connected after authority resolved.` with
the remote workspace id, `[MainThreadShellExec] ... hostKind=3, expectedHost=3, isRemote=true,
sandboxSupported=true, linuxKernelVersion=6.12.94+` (a Linux kernel on the agent's machine) and
`shared-process MCP routing disabled for remote workspace`. The GitHub authentication and MCP
OAuth logs are described under [Credentials](#credentials).

### Format versions

- Before 3.0 (L4, cursaves; forum 2825, 167641): the composer list in `ItemTable`
  `composer.composerData` (<=2.6), then `composer.composerHeaders` (3.0+); the legacy chat in
  `ItemTable` `workbench.panel.aichat.view.aichat.chatdata`; `cursorDiskKV`
  `messageRequestContext:*` rows, not seen on the 3.24.18 to 3.26.13 install.
- 3.24.18 and 3.25.9 (observed): the headers in the `composerHeaders` table (`cursorDiskKV`
  `composer.composerHeaders.migratedToTable`, `ItemTable` `composer.composerHeaders.version`), the
  header list inline in `composerData.fullConversationHeadersOnly`. 3.25.9 added
  `anysphere.cursor-agent-host/data.sqlite` and `cursor.storage.authTokenEncryptionAtRest`.
  3.25.9 on Linux (deb, 2026-10-20) wrote the same records as 3.25.9 on macOS: the header list
  inline, no `composerConvHeaders:` row, tool names with `_v2`; its editor window's conversation
  had `composerTranscriptHeightTailIde:<composerId>`. The apt repository
  (`https://downloads.cursor.com/aptrepo`, suite `stable`, component `main`, amd64 and arm64)
  served 3.25.9 when the VM upgraded at 21:35Z on 2026-10-20 and listed 3.26.13 for both
  architectures from its 22:18Z refresh the same day (checked 2026-10-21): the arm64 build is
  not behind, and the repository follows a release within the day.
- 3.26.13 (observed): a new conversation's header list is in
  `composerConvHeaders:<composerId>:<NNNNNN>` chunks, with `fullConversationHeadersOnly` `[]`; an
  older conversation keeps its inline list until the composer is rewritten, and archiving the
  first conversation moved its 37 entries into a chunk. Also new: `composerDraft:<composerId>` and
  `composerTranscriptHeightTail:<composerId>`. Tool names carry a `_v2` suffix on both 3.25.9 and
  3.26.13. 3.26.13 on Windows (per-user installer, 2026-10-21) wrote the same layout as on
  macOS: a new conversation with `fullConversationHeadersOnly` `[]` and one
  `composerConvHeaders:<composerId>:000000` chunk, the user bubble's `modelInfo.modelName`
  `default`, so the parser's two-layout rule holds there; differences in single keys are noted
  where they occur ([`composerHeaders`](#composerheaders), [Tool bubbles](#tool-bubbles)).
- `composerData._v` is 18 on every composer of the 3.26.13 install (12 rows);
  `composerRootWriterDiagnostic.version` carries a version whose value was not recorded.
- Cursor Agent CLI 2026.10.19-6bbf2c4 (observed, macOS, 2026-10-20; the build the IDE's worker
  directory held): the `store.db` layout of [CLI chat store](#cli-chat-store) and a `meta.json`
  with `schemaVersion` and `title` and without the bundle's `isSubagent`; the value of
  `schemaVersion` was not recorded. The bundle of 2026.10.01-e373342 names `chat-session-meta.ts`
  schema v1 and older `*.txt` transcripts.

### Secrets

The tokens, the secret-storage rows and the identity keys of `ItemTable` are listed in
[Credentials](#credentials). Inside the conversation stores: `composerData.blobEncryptionKey` and
`speculativeSummarizationEncryptionKey` (keys for blobs that were stored in plaintext anyway);
`cursorCreds` in the settings blob holds URLs and client ids, no secret; the hostname is in
`agent-cli-state.json` keys and in the `AgentStores` lock and pin files. User content beyond the
messages: the `agentKv` prompts with their context, the FTS index, the `ofsContent:` and
`composer.content.*` file texts, the retrieval checkpoints' file copies and
`tracked_file_content.content`.

On Linux (3.25.9, 2026-10-20) the `secret://` values are VS Code's secret storage encrypted with
the Electron safeStorage key, stored as JSON `{type: "Buffer", data: [...]}`: the editor's GitHub
token `secret://{"extensionId":"vscode.github-authentication","key":"github.auth"}` (743 bytes,
195 data bytes), `secret://remoteAgentHost.computerKey.auth0|user_<id>` (727 bytes, 195 data
bytes) and the `vscode.git` row (318 bytes, 83 data bytes). So the editor's provider tokens are
ciphertext while Cursor's own `cursorAuth/*` JWTs beside them are plaintext. The macOS
`secret://` values were not shaped. `cursor/glass.githubConnect.pendingFlow`'s `state` (88
characters) and `verifier` (43 characters) are the one-time secrets of the GitHub connect flow
(OAuth with PKCE, the proof-key form of the authorization-code flow), present only while the flow
runs. Cursor's GitHub app grant has no local token. The GitHub login is an identity value in the
key names `github-<login>` and `github-<login>-usages` and in the value of `vscode.github-github`
([`ItemTable` keys](#itemtable-keys)).

On Windows (3.26.13, 2026-10-21) the `secret://` values are the same JSON Buffer form, encrypted
with the safeStorage key, which Electron protects with DPAPI (the Windows Data Protection API,
which encrypts data to the user's logon credentials) on Windows: the editor's GitHub token row
was 814 bytes, `secret://remoteAgentHost.computerKey.auth0|user_<id>` 770 bytes; the cloud
agent's `connectionToken:bc-<uuid>` and `git-ipc-auth-token` rows and the worker's `apiKey` and
`apiKeyAuthId` rows were not shaped ([Credentials](#credentials)). Identity values beside them:
the host name in `remoteAgentHost.displayName.<account>`, which the `remote agent host
enrolled:` row prints, in the `AgentStores` pin and lock files, and with the project path in the
embedded worker's name, `<project path> @ <hostname>`, which the worker's log prints
([Remote control](#remote-control)), beside which `restart.json`'s 30-character `workerName` is
identity-bearing too, its form not resolved; the repository's
`<owner>/<repo>` in the `glass.cloudRepoBranches.*`, `glass.cloudRepoBranchRecency.*` and
`agentData.cacheStorage.glass.repoSlashInventory.v2.*` key names and in the cloud-agent records
([Cloud-agent keys](#cloud-agent-keys)); the clone's path in the
`cursor.plugins.installedIds.no-team|file:///...` key names and inside `vscode.github`; and the
editor's GitHub scopes in the key name `vscode.github-github-repo workflow user:email
read:user`. User content beyond the messages: a cloud agent's cached conversation text in
`conversation-search.db` and the prompt inside `bcCachedDetails`.

The MCP OAuth state of a plugin (Windows, 3.26.13, 2026-10-21;
[MCP servers and plugins](#mcp-servers-and-plugins)): every `mcpOAuth.secret.<b64>` value is
safeStorage ciphertext in the Buffer form. Before authentication `mcp_code_verifier` held 74 data
bytes, `mcp_oauth_attempt_client_secret` 66 and `mcp_client_information` 354; after it
`mcp_tokens` was 2,559 bytes and the rewritten verifier, client information and client secret
292, 1,294 and 259. The values are never read; the key names, decoded, say which server holds
tokens. The plaintext beside them is identity, not secret: `mcp_server_url` and the update time
in `mcpOAuth.global.*`, and in the attempt file the 72-character `client_id`, the redirect URIs
and the server URLs. A cloud agent's MCP authorisation is a second credential, on the server:
the host keeps only the decision record `cloudAgentRepository.mcpAuthResolution`, and the local
`mcp_tokens` key did not change when it was granted, so revoking a Google grant on the host's
evidence alone misses the cloud agent's. The user's Cursor namespace, an identity value, is in a
temporary Origin repository's `repoUrl`, `origin.cursor.com/git/<namespace>/tmp-<hex>`, and in
the key names `glass.cloudRepoBranches.<account>.origin.cursor.com%2Fgit%2F<namespace>%2F...`
and `agentData.cacheStorage.glass.repoSlashInventory.v2.origin.cursor.com%2Fgit%2F<namespace>...`
([Cloud-agent keys](#cloud-agent-keys)).

The CLI on macOS keeps its tokens in the login keychain ([Credentials](#credentials)).
`cli-config.json` holds, while signed in, `authInfo` (the email, the display name, the numeric
user id and the `auth0|user_<id>` id) and the 42-character `authCacheKey` of `serverConfigCache` and
`autoReviewAvailabilityCache`, whose derivation is not known. In the chat
store, `meta.0.blobEncryptionKey` (64 hex digits) sits beside blobs that were not encrypted with
it, and the reasoning blocks' `signature` strings are opaque. User content in the chat store: the
context prompts, an edit's `beforeFullFileContent` and `afterFullFileContent`, command output,
and `prompt_history.json`.

### Parser plan

`wants()` by `artifact.rel`, with `(?:Library/Application Support|\.config|AppData/Roaming)/Cursor`
written `<ide>`:

```text
^<ide>/User/globalStorage/state\.vscdb$
^<ide>/User/globalStorage/conversation-search\.db$
^\.cursor/projects/[^/]+/agent-transcripts/[^/]+/[^/]+\.jsonl$
^\.cursor/ai-tracking/ai-code-tracking\.db$
^\.cursor/cli-config\.json$
^\.cursor/permissions\.json$
^<ide>/User/globalStorage/mcp-oauth-attempts/[^/]+\.json$
```

SQLite files are opened through `sqlite_util.py`. In `state.vscdb` the parser reads every
`composerData:` row; a composer with `status` `none` and an empty header list in both layouts is a
draft and yields nothing but its `composerDraft:` row. For the others it walks the header list
(`fullConversationHeadersOnly` when non-empty, else every `composerConvHeaders:<composerId>:*`
chunk in suffix order, concatenated) and reads `bubbleId:<composerId>:<entry bubbleId>` for each
entry.

| Record | `turn_type` and text |
| --- | --- |
| `composerHeaders` row or `composerData` with `name` | `system`, `session name: <name>` at `createdAt` |
| user bubble (`type` 1) | `user`, `text`, at `createdAt` |
| sub-bubble with `thinking.text` | `thinking`, `thinking.text` (only with `include_thinking`) |
| sub-bubble with non-empty `text` | `assistant`, `text`, at `createdAt` (else `startedAtMs`) |
| tool bubble | `tool_use`, `toolFormerData.params` (else `rawArgs`), at `startedAtMs`; then the permission row when `additionalData.reviewData` is present; then `tool_result`, `toolFormerData.result`, prefixed `[rejected] ` when `additionalData.status` is `rejected`, at `completedAtMs` |
| first user bubble with `modelInfo.modelName`; a later one with another | `system`, `model: <id>`; `model: <old> -> <new>` |
| `composerDraft:<composerId>` | `system`, `unsent draft: <text>` at the composer's `createdAt`, or with an empty `timestamp_utc` when it has none |
| `composerHeaders` `isArchived` 1 with its `composerData` present | `system`, `conversation archived` at `lastUpdatedAt` |
| `composerHeaders` row with no `composerData` and not a draft | `system`, `conversation deleted` at `lastUpdatedAt` ([Deletion rows](#deletion-rows)) |
| header entry with no bubble row | `system`, `parser: missing bubble <bubbleId>`, at the entry's `createdAt` |
| settings blob and `cli-config.json`, `permissions.json` | `system` permission and model rows ([Permission rows](#permission-rows), [Model rows](#model-rows)) |
| `ai-code-tracking.db`, `conversation-search.db`, transcript JSONL, `agentKv` blobs | [Past-use rows and secondary sources](#past-use-rows-and-secondary-sources) |
| a cloud agent: the `cloud-cache` row of `conversation-search.db`, its element of `cloudAgentRepository.agents.*`, and `cloudEnvironmentTemplate.githubRepos` | [Sync, relay and origin rows](#sync-relay-and-origin-rows) |
| `cloudAgentRepository.mcpAuthResolution` | a `permission decision:` row ([Permission rows](#permission-rows)) |
| `mcpOAuth.global.*` keys, `cursor.plugins.installedIds.*` keys and `mcp-oauth-attempts/<uuid>.json` | [MCP server and plugin rows](#mcp-server-and-plugin-rows) |

Columns:

| Column | Source |
| --- | --- |
| timestamp_utc | as in the table; epoch ms converted to UTC |
| session_id | the `composerId`; for a cloud agent its `bc-<uuid>` |
| project_path | `composerHeaders.value.workspaceIdentifier.uri.fsPath`, else `composerData.workspaceIdentifier.uri.fsPath`, else the first `trackedGitRepos[].repoPath`; for a cloud agent the record's `repoUrl` (`github.com/<owner>/<repo>`) |
| git_branch | the first entry of the matching `trackedGitRepos[].branches`, which was empty on the install; empty otherwise |
| model | the tracked `modelInfo.modelName` of the turn's user bubble; for a cloud agent's rows its `modelDetails.modelName` |
| tool_name | `toolFormerData.name` |
| tool_use_id | `toolFormerData.toolCallId` |
| source_line | the `cursorDiskKV` rowid, or the 1-based line of a JSONL file, or the rowid of a table row |

#### Permission rows

Per call: a tool bubble whose `additionalData.reviewData` is present and answered yields, after
its `tool_use` row, a `system` row `permission decision: <selectedOption>
outcome=<additionalData.status> approval=<approvalType> reason=<blockReason>`, then
` allowlisted=<entries>` when `appliedAllowlistOptionIds` is non-empty (the `commandName` of each
matching `allowlistOptions` entry, else the ids), with the call's `tool_use_id` and `tool_name`.
The `allowlisted=` suffix marks an allow-always choice whatever `selectedOption` says, since the
Agents window writes `allowlistCommands` and the editor window `run` for it, and it is the
durable record of that choice: on Linux the standing list never gained the entry
([Run mode and standing lists](#run-mode-and-standing-lists)). A bubble whose
`reviewData.status` is neither `Done` nor `None`, or whose `toolFormerData.status` is
`loading`, is a call
still waiting at its prompt when the store was copied: its `selectedOption` is the preset
`rejectAndTellWhatToDoDifferently`, so it yields no decision but `permission prompt pending:
<tool_name> reason=<blockReason> options=<allowlistOptions ids>`, joined on `tool_use_id`, and
no `tool_result` row unless its `result` is non-empty
([issue 237](https://github.com/seanthegeek/doubleagent/issues/237)). On Windows (3.26.13) such
a bubble also had `additionalData.status` `pending`, and its `allowlistOptions` was `[]`, so the
row's `options=` is empty and its `reason=` is `Shell allowlist is empty`. A call that ran under
auto-run (`additionalData` without `reviewData`, whatever its `status`, `error` included) or with
no `additionalData` yields no permission row; the run mode then in force is the standing row
below. The record shapes are in [Tool bubbles](#tool-bubbles).

A call cut off when the user stopped the turn, `reviewData.status` `None` with
`additionalData.status` `rejected`, is neither an answer nor a pending prompt: it yields
`permission decision: stopped by the user (reviewData.status None, additionalData.status
rejected)` beside the call, joined on `tool_use_id`, and its `tool_result` prefixed
`[rejected] `. A call with `toolFormerData.status` `cancelled` (one the stopped turn never ran)
yields its `tool_use` row and no permission row.

An MCP call (`toolFormerData.name` `mcp-<server>-<tool>`, and `get_mcp_tools`) is an ordinary
`tool_use` and `tool_result` pair, its `additionalData` holding only `status`, so it yields no
permission row: an MCP prompt answered Run under Allowlist leaves no per-call record on 3.26.13
(Windows, 2026-10-21), and no `permission decision:` row can exist for an MCP call. The
standing rows below give the run mode and `yoloMcpToolsDisabled` in force. `toolCallBinary` is
not read.

A cloud agent's request to authorise an MCP server, `cloudAgentRepository.mcpAuthResolution`
([Cloud-agent keys](#cloud-agent-keys)), yields `permission decision: <userDecision> mcp auth
for cloud agent <agentId> tool_call=<toolCallId> (from state.vscdb
cloudAgentRepository.mcpAuthResolution, file mtime)` at the file's modification time, with
`session_id` the `agentId` and `tool_use_id` the `toolCallId`; a value that is a list, which no
install has shown, gives one row per element. The cloud agent's own tool calls are on its
machine and prompted for nothing on the install.

Standing, at the modification time of the file, saying so in the text, with empty `session_id`: from
`state.vscdb`'s settings blob, which the app writes late ([`ItemTable` keys](#itemtable-keys)), one
row per entry of `yoloCommandAllowlist`, `yoloCommandDenylist`, `smartAllowlistDenylist`,
`webFetchDomainAllowlist` and `autoApprovedModeTransitions`, `permission setting: <list>=<entry>
(from state.vscdb composerState, file mtime, written late by the app)`; one per `modes4` entry,
`permission setting: run mode <id>=<mode> autoRun=<b> smartModeAutoRun=<b> fullAutoRun=<b>
sandbox=<b>`, the mode derived as in [Run mode and standing lists](#run-mode-and-standing-lists);
and one per boolean switch of that section (on Windows the list entry was
`yoloCommandAllowlist=powershell`). From `cli-config.json`: `approvalMode`, each
`permissions.allow` and `permissions.deny` entry and `sandbox.mode`. From `permissions.json`
(bundle-derived, fixture only): each `allowlists.mcp` and `allowlists.terminal` entry,
`approvalMode`, and each `allowInstructions` and `blockInstructions` entry.

The Agent CLI (2026.10.19-6bbf2c4, 2026-10-20) asks with four options, in this order: Allow once,
Add `<command>` to the allow list, Run everything, and Skip and tell the agent what to do. Its
chat store records no decision as such: a call that ran has a `tool` message with
`highLevelToolCallResult.output.success` and `isError` false, a skipped one
`output.rejected{command, reason, workingDirectory}` and `isError` true, joined to the call on
`toolCallId` ([CLI chat store](#cli-chat-store)). Allow once and Add to the allow list differ only
by the new `permissions.allow` entry `Shell(<command>)` in `cli-config.json`, which the standing
rows above already read. The session's `meta.0.isRunEverything` flag and the folder-trust grant
`.workspace-trusted` (at `trustedAt`) are standing records for [issue 235](https://github.com/seanthegeek/doubleagent/issues/235).

#### Model rows

Per message the model is the user bubble's `modelInfo.modelName`: `model: <id>` at the first user
bubble that names one, `model: <old> -> <new>` at each later user bubble whose model differs;
assistant and tool rows take the tracked model. A user bubble with no `modelInfo`, such as a turn
the agent injected ([User bubbles](#user-bubbles)), neither starts nor changes the tracked model
and yields no `model:` row. A switch in the UI writes no bubble, so it shows
only at the next message. `composerData.modelConfig` is the current selection with no history and
yields no row. The settings blob's `aiSettings.modelLastUsedAt` is the only dated record of a
switch beside the next user bubble, and it is written late: on Linux its entry for the new model
was dated 27 minutes after the switch, at a later command, while on Windows (3.26.13) it was
dated at the switch ([`ItemTable` keys](#itemtable-keys)), so its time is when the app wrote the
blob, at or after the switch. One `system` row per entry,
`model last used: <id> (from state.vscdb aiSettings.modelLastUsedAt, written late by the app)`,
at its time, with empty `session_id`.

A cloud agent records one model per agent: its element of `cloudAgentRepository.agents.*` yields
`model: <modelDetails.modelName>` at its `createdAt`, followed by
` requested=<requestedModel.modelId>` when the requested id differs from the resolved name; the
`model` column of that session's rows carries `modelDetails.modelName`.

For the CLI, `meta.0.lastUsedModel` is the session's current model id (`default`, the Auto
selection, then `grok-4.7`), updated at the first message after a switch; each assistant
`reasoning` block's `providerOptions.cursor.modelName` is the resolved id per message
(`cursor-grok-4.5-high` while Auto was selected, `grok-4.7-high-fast` for `grok-4.7` with
`reasoning_effort` `high` and `fast` `true`); and `cli-config.json` `modelSelectionHistory` lists
the selections, most recent first, with no times. A `/model` switch with no message sent changed
only `cli-config.json`. The plan for [issue 235](https://github.com/seanthegeek/doubleagent/issues/235): `model: <resolved>` at the first assistant message
and `model: <old> -> <new>` on a change, with the selection (`meta.0.lastUsedModel`,
`selectedModel.modelId`) beside the resolved id when they differ.

#### Past-use rows and secondary sources

- Transcript JSONL (secondary; `secondary()` true), `session_id` from the file name and
  `project_path` from the composer when the database has it. A user line yields a `user` row
  whose text is the query inside `<user_query>`, with the wrapper stripped, so it equals the user
  bubble's `text`, and whose `timestamp_utc` is the `<timestamp>` tag converted to UTC with its
  offset, at minute precision; a line whose wrapper does not parse keeps its whole text and an
  empty `timestamp_utc`. An assistant line yields an `assistant` row per text block and a
  `tool_use` row per tool block (`tool_name` the transcript's own CamelCase name, `text` the
  compact JSON of `input`), with an empty `timestamp_utc`, so they match on text alone; the
  mapping to the IDE's names in [Agent transcript JSONL](#agent-transcript-jsonl) was observed on
  one install and is not applied. `turn_ended` lines yield nothing. `secondary_tolerance()` is at
  least 60 seconds, because the tag has minute precision and the bubble's `createdAt` has
  milliseconds. The
  file survives a delete.
- `agentKv` blobs (secondary): a JSON blob whose `content` contains the id of a composer with a
  `composerHeaders` row but no `composerData` (a deleted conversation) yield `user` and
  `assistant` rows from
  `content` (a `system` message as `system prompt: <content>`), with an empty `timestamp_utc`
  and `source_line` the rowid. Blobs of a conversation still present are not yielded: their user
  text is the whole prompt with its context, which repeats the bubbles in a form deduplication
  cannot match. Protobuf blobs yield nothing.
- `ai-code-tracking.db` (not secondary: it repeats no event): `ai_code_hashes`, `system`, `ai
  code: <fileName> source=<source> model=<model> request=<requestId>` at `timestamp`;
  `conversation_summaries`, `system`, `conversation summary: <title>: <tldr>` at `updatedAt`;
  `ai_deleted_files`, `system`, `file deleted by agent: <gitPath> model=<model>` at
  `deletedAt`; `scored_commits`, `system`, `scored commit: <commitHash> branch=<branchName>
  +<linesAdded> -<linesDeleted>` at `scoredAt`; `session_id` the `conversationId`. Only
  `ai_code_hashes` was populated on the install.
- `conversation-search.db` (not secondary): one `system` row per `conversations` row with a
  title, `conversation index: <title> source=<source> archived=<is_archived>` at `updated_at`,
  `session_id` the `id`. A delete removes a local row, so it marks only conversations still
  present. The text is read from the shadow table `conversation_fts_content` directly, joined
  on `fts_rowid`, never through the FTS5 virtual table, whose module may be absent. For a
  `cloud-cache` row with a non-empty body (`c1`) the parser yields one `system` row
  `cached conversation text (conversation-search.db, source=cloud-cache): <c1 in full>` at
  `updated_at`, `session_id` the `bc-<uuid>`: the only local copy of a cloud agent's
  conversation text ([Cloud agents](#cloud-agents)). It is never a `user` or `assistant` row,
  because the body runs the prompt and the reply together with no boundary. Some `local` rows
  have no body ([`conversation-search.db`](#conversation-searchdb)); a non-empty one yields the
  same row with `source=local`.
- The surviving `composerHeaders` row of a deleted conversation gives its `conversation deleted`
  row ([Deletion rows](#deletion-rows)).

The CLI keeps `prompt_history.json` per session, the typed prompts with no times, which repeats
the user messages, and a `meta.json` with no `store.db` for each start that sent nothing
(`hasConversation` false, no `title`), an empty start rather than a lost session; neither is read
yet ([issue 235](https://github.com/seanthegeek/doubleagent/issues/235)). Its transcript JSONL and `ai-code-tracking.db` rows are read as above.

The retrieval checkpoints, `AgentStores`, `update-supervisor`, the logs and an exported Markdown
file yield nothing: no times per turn, or no join to a conversation. The workspace
`aiService.prompts` key stayed `[]` and is not read. The terminal captures
`.cursor/projects/<p>/terminals/<N>.txt` hold the output of the agent's commands, a record the
transcript JSONL lacks, but as plain text with no time, command or call id, and a quit removes
them ([What survives a delete](#what-survives-a-delete)); they are not read.
`bcCachedDetails:bc-<uuid>` is read as a key but not decoded, pending its descriptors, and
yields nothing; the cached text above is the readable copy.

#### Sync, relay and origin rows

The IDE recorded no sync of a local conversation (3.26.13, macOS 2026-10-20 and Windows
2026-10-21): no Share or upload surface, `agentLocation.type` `local` on both macOS
conversations and the key absent on Windows, and every `AgentStores` `files` table empty (those
stores sync files such as skills, not transcripts). The parser yields `session origin: remote
<agentLocation.type>` when `agentLocation.type` is present and not `local` (bundle-derived: only
`local` was seen) and sets `records_origin`, so a local conversation's `origin` reads `local`,
and `records_sync`, so a local conversation's `synced` reads `no`.
Relay: the agent records none. The CLI recorded no sync either (2026.10.19-6bbf2c4,
2026-10-20): it has no share command, and `agent worker start`, which runs a private cloud worker
and is the CLI's remote surface, was not exercised.

A cloud agent is a conversation of remote origin, held by the vendor
([Cloud agents](#cloud-agents)), and yields:

- from the `cloud-cache` row of `conversation-search.db`, `session origin: remote cursor cloud
  agent` at `updated_at`, beside the `conversation index:` row and the cached text of
  [Past-use rows and secondary sources](#past-use-rows-and-secondary-sources);
- from each element of an `ItemTable` key beginning `cloudAgentRepository.agents.` (the suffix,
  the account id in either the `auth0|user_<id>` or the URL-encoded `auth0%7Cuser_<id>` form, is
  not matched; a value that is not a JSON list is skipped), with `session_id` the `bcId` and
  `project_path` the `repoUrl`: `cloud agent: <name> repo=<repoUrl> status=<status>
  workflowStatus=<workflowStatus> archived=<isArchived> killed=<isKilled>
  vmStarted=<hasStartedVm> (from state.vscdb cloudAgentRepository.agents, rowUpdatedAtMs)` at
  `rowUpdatedAtMs`, a field that is absent or empty printed `-`; `source host: github.com
  <owner>/<repo> (cloud agent)` at `createdAt` when the host of `repoUrl`, parsed as a URL with
  or without a scheme, is `github.com`; the `model:` row of [Model rows](#model-rows);
  `session origin: remote cursor cloud agent` at `createdAt` only when no `conversation-search.db`
  row yielded it for the same `bcId`; and `session synced: cursor cloud agent <bcId> (vendor copy;
  from state.vscdb cloudAgentRepository.agents)` at `createdAt`, else `rowUpdatedAtMs`, since the
  vendor holds the conversation;
- for an agent started as a new project in a temporary Origin repository (the second agent of
  [Cloud agents](#cloud-agents)), the same rows, with `project_path` its
  `origin.cursor.com/git/<namespace>/tmp-<hex>` `repoUrl` and no `source host:` row, since the
  URL is not GitHub's; its MCP authorisation is the decision row of
  [Permission rows](#permission-rows);
- from each element of `cloudEnvironmentTemplate.githubRepos`, the repository list the account
  may reach through Cursor Origin, `source host repository: <url or name> type=<type> (from
  state.vscdb cloudEnvironmentTemplate.githubRepos, file mtime)` at the file's modification time,
  with an empty `session_id`.

`bcCachedDetails:bc-<uuid>` yields nothing until it is decoded
([Past-use rows and secondary sources](#past-use-rows-and-secondary-sources)). The `status`,
`workflowStatus` and `source` numbers are printed as stored; their meanings are not known.

Source host ([Source-host integration](#source-host-integration)): one `system` row per
`github-<login>` key of the editor's GitHub sign-in, `source host: github account <login>
scopes=<scopes> (from state.vscdb key github-<login>)`, the login from the key name and the
scopes from the `-usages` entries, at their latest `lastUsed`, else the file's modification time;
and `remote agent host enrolled: <displayName> (from state.vscdb
remoteAgentHost.enrolled.<account>, file mtime; on by default, a paired device leaves no local
record)` when `remoteAgentHost.enrolled.<account>` is true, the name from
`remoteAgentHost.displayName.<account>`, the host name on Windows. The enrolment row is a
standing capability of the install, not a user action and not a relay or a sync; no `remote agent
device paired:` row can exist, because pairing leaves no local record
([Remote control](#remote-control)). Both rows have an empty `session_id` and are added in
analyzer 0.20.1. Only these keys are read: the cached GitHub token, the computer key and the
cloud agent's connection token are never read. The Cursor GitHub app connection records nothing
to yield.

#### Scratch directory rows

None under the home. The `$TMPDIR` directories of [Scratch directories](#scratch-directories), the
CLI's logs among them, and its askpass sockets under `/private/tmp` are not collected.

#### Deletion rows

An archived conversation yields `conversation archived` at `lastUpdatedAt`. A `composerHeaders` row
whose `composerData` is gone, not a draft (it has an entry in `agentLocationHistory`, or
`isArchived` 1), yields `conversation deleted (header kept; time is the header's lastUpdatedAt)`
followed by ` (isArchived is set by the delete)`, at `lastUpdatedAt`, with `project_path` from
the header and `session_id` the composer id, and no `conversation archived` row, since the delete
sets `isArchived` 1 on a conversation that was never archived
([What survives a delete](#what-survives-a-delete)); an archive time, if there was one, is lost,
since the delete overwrote `lastUpdatedAt`. A cloud agent's archive shows as `archived=1` in its
`cloud agent:` row and in its `conversation index:` row; no delete of a cloud agent exists in the
IDE. What survives a delete is in
[What survives a delete](#what-survives-a-delete).

The CLI has no delete command and keeps no header row that outlives a session: a session
directory removed by hand shows only as a transcript JSONL, and tracking rows, whose session id
has no `chats/*/<sessionId>/` directory. The transcript's rows already carry that session id; no
deletion row is yielded for it.

#### Bad-line rows

The transcript JSONL is read with `iter_jsonl`, and each bad line becomes its own `system` row
through `error_rows`. A `cursorDiskKV` or `ItemTable` value that is not JSON (when it should be)
is appended to the errors list with its rowid and key, and a header entry with no bubble row has
its own row (table above). A database that cannot be read, or whose `-wal`, `-shm` or `-journal`
sidecar cannot, is one row `parser: <file> could not be read (<reason>)`, `<file>` the name of the
file that failed; one that is not a SQLite database is `parser: not a SQLite database`, a SQLite
error one `parser: SQLite error: <message>` row. `cli-config.json` or `permissions.json` that is not
JSON is one `parser:` row, and so is an `mcp-oauth-attempts/<uuid>.json` file that is not JSON
(`parser: unreadable <rel>: <reason>`) or not a JSON object (`parser: <rel> is not a JSON object`).
For the CLI parser, a `meta.json`, `prompt_history.json` or `.workspace-trusted` that is not JSON,
or a `meta.0` value that is not hex-encoded JSON, is one `parser:` row for that file, and a blob
that is neither JSON nor protobuf is appended to the errors list with its `id`.

#### Fixtures

Synthetic records in the shapes above, invented values. `state.vscdb`, `cursorDiskKV`:

```json
{"key": "composerData:c1", "value": {"_v": 18, "composerId": "c1", "status": "completed", "name": "Add subtract", "createdAt": 1790000000000, "lastUpdatedAt": 1790000300000, "modelConfig": {"modelName": "grok-4.7", "maxMode": false, "selectedModels": [{"modelId": "grok-4.7", "parameters": [{"id": "context", "value": "256k"}]}]}, "workspaceIdentifier": {"id": "0123456789abcdef0123456789abcdef", "uri": {"fsPath": "/srv/proj"}}, "trackedGitRepos": [{"repoPath": "/srv/proj", "branches": []}], "fullConversationHeadersOnly": [{"bubbleId": "u1", "type": 1, "createdAt": "2026-10-01T10:00:00.000Z", "startedAtMs": 1790000000000, "completedAtMs": 1790000001000}, {"bubbleId": "a1/s:0", "type": 2, "createdAt": "2026-10-01T10:00:01.000Z"}, {"bubbleId": "a1/t:call-t1-1\nfc_t1_1", "type": 2, "createdAt": "2026-10-01T10:00:02.000Z"}, {"bubbleId": "a1/s:2", "type": 2, "createdAt": "2026-10-01T10:00:05.000Z"}, {"bubbleId": "u2", "type": 1, "createdAt": "2026-10-01T10:01:00.000Z"}, {"bubbleId": "a2/t:call-t2-0\nfc_t2_0", "type": 2, "createdAt": "2026-10-01T10:01:01.000Z"}, {"bubbleId": "a2/t:call-t3-1\nfc_t3_1", "type": 2, "createdAt": "2026-10-01T10:01:10.000Z"}, {"bubbleId": "a2/t:call-t4-2\nfc_t4_2", "type": 2, "createdAt": "2026-10-01T10:01:20.000Z"}, {"bubbleId": "a2/t:call-t7-3\nfc_t7_3", "type": 2, "createdAt": "2026-10-01T10:01:40.000Z"}, {"bubbleId": "a2/t:call-t8-4\nfc_t8_4", "type": 2, "createdAt": "2026-10-01T10:01:50.000Z"}]}}
{"key": "bubbleId:c1:u1", "value": {"type": 1, "text": "Add a subtract function", "createdAt": "2026-10-01T10:00:00.000Z", "requestId": "r1", "checkpointId": "k1", "modelInfo": {"modelName": "grok-4.6"}}}
{"key": "bubbleId:c1:a1/s:0", "value": {"type": 2, "bubbleId": "a1/s:0", "capabilityType": 30, "text": "", "thinking": {"text": "Read calc.py first.", "signature": "sig"}}}
{"key": "bubbleId:c1:a1/t:call-t1-1\nfc_t1_1", "value": {"type": 2, "capabilityType": 15, "startedAtMs": 1790000002000, "completedAtMs": 1790000002500, "toolFormerData": {"name": "read_file_v2", "params": "{\"path\":\"/srv/proj/calc.py\"}", "rawArgs": "{\"path\":\"calc.py\"}", "result": "{\"contents\":\"def add(a, b): ...\"}", "status": "completed", "tool": 40, "toolCallId": "tc1", "modelCallId": "mc1", "toolIndex": 0}}}
{"key": "bubbleId:c1:a1/s:2", "value": {"type": 2, "capabilityType": null, "text": "Added subtract().", "turnDurationMs": 5000}}
{"key": "bubbleId:c1:u2", "value": {"type": 1, "text": "Fetch the page with curl", "createdAt": "2026-10-01T10:01:00.000Z", "modelInfo": {"modelName": "grok-4.7"}}}
{"key": "bubbleId:c1:a2/t:call-t2-0\nfc_t2_0", "value": {"type": 2, "capabilityType": 15, "toolFormerData": {"name": "run_terminal_command_v2", "params": "{\"command\":\"curl -s example.test | head\"}", "rawArgs": "", "result": "{\"output\":\"ok\"}", "status": "completed", "toolCallId": "tc2", "additionalData": {"status": "success", "startedAtMs": 1790000061000, "blockReason": "Not in allowlist: curl, head", "reviewData": {"selectedOption": "run", "approvalType": "user", "status": "Done", "allowlistOptions": [{"id": "curl", "commandName": "curl"}, {"id": "head", "commandName": "head"}]}}}}}
{"key": "bubbleId:c1:a2/t:call-t3-1\nfc_t3_1", "value": {"type": 2, "capabilityType": 15, "toolFormerData": {"name": "run_terminal_command_v2", "params": "{\"command\":\"curl -s example.test | head\"}", "result": "{\"error\":\"skipped\"}", "status": "completed", "toolCallId": "tc3", "additionalData": {"status": "rejected", "blockReason": "Not in allowlist: curl, head", "reviewData": {"selectedOption": "skip", "approvalType": "user", "status": "Done"}}}}}
{"key": "bubbleId:c1:a2/t:call-t4-2\nfc_t4_2", "value": {"type": 2, "capabilityType": 15, "toolFormerData": {"name": "run_terminal_command_v2", "params": "{\"command\":\"curl -s example.test | head\"}", "result": "{\"output\":\"ok\"}", "status": "completed", "toolCallId": "tc4", "additionalData": {"status": "success", "startedAtMs": 1790000080000, "blockReason": "Not in allowlist: curl, head", "reviewData": {"selectedOption": "allowlistCommands", "approvalType": "user", "status": "Done", "allowlistOptions": [{"id": "curl", "commandName": "curl"}, {"id": "head", "commandName": "head"}], "appliedAllowlistOptionIds": ["curl", "head"]}}}}}
{"key": "bubbleId:c1:a2/t:call-t7-3\nfc_t7_3", "value": {"type": 2, "capabilityType": 15, "toolFormerData": {"name": "run_terminal_command_v2", "params": "{\"command\":\"python3 -c 'print(1+1)'\"}", "result": "{\"output\":\"2\"}", "status": "completed", "toolCallId": "tc7", "additionalData": {"status": "success", "startedAtMs": 1790000100000, "blockReason": "Not in allowlist: python3", "reviewData": {"selectedOption": "run", "status": "Done", "allowlistOptions": [{"id": "python3", "commandName": "python3"}], "candidatesForAllowlist": ["python3"], "appliedAllowlistOptionIds": ["python3"]}}}}}
{"key": "bubbleId:c1:a2/t:call-t8-4\nfc_t8_4", "value": {"type": 2, "capabilityType": 15, "toolFormerData": {"name": "run_terminal_command_v2", "params": "{\"command\":\"python3 -c 'print(2+2)'\"}", "result": "", "status": "loading", "toolCallId": "tc8", "additionalData": {"blockReason": "Not in allowlist: python3", "reviewData": {"status": "Requested", "selectedOption": "rejectAndTellWhatToDoDifferently", "isShowingInput": false, "allowlistOptions": [{"id": "python3", "commandName": "python3"}]}}}}}
{"key": "composerData:c2", "value": {"composerId": "c2", "status": "completed", "name": "Read the README", "createdAt": 1790001000000, "fullConversationHeadersOnly": []}}
{"key": "composerConvHeaders:c2:000000", "value": [{"bubbleId": "u3", "type": 1, "createdAt": "2026-10-01T10:20:00.000Z"}, {"bubbleId": "a3/s:0", "type": 2, "createdAt": "2026-10-01T10:20:02.000Z"}]}
{"key": "bubbleId:c2:u3", "value": {"type": 1, "text": "What does the README say?", "modelInfo": {"modelName": "grok-4.7"}}}
{"key": "bubbleId:c2:a3/s:0", "value": {"type": 2, "text": "It describes calc.py."}}
{"key": "composerData:d1", "value": {"composerId": "d1", "status": "none", "fullConversationHeadersOnly": []}}
{"key": "composerDraft:d1", "value": {"richText": "{}", "text": "draft not sent"}}
{"key": "agentKv:blob:00ff", "value": {"role": "user", "content": "<context>conversation c3</context>\n<user_query>c3 deleted question</user_query>", "providerOptions": {}}}
{"key": "agentKv:blob:0a0b", "value": "<protobuf bytes, not JSON>"}
```

`composerHeaders` rows: `c1` (`isArchived` 0), `c2` (`isArchived` 1, `value.isArchived` true,
`lastUpdatedAt` 1790002000000) and the deleted `c3`, which has no `composerData:c3`:

```json
{"composerId": "c3", "workspaceId": "0123456789abcdef0123456789abcdef", "createdAt": 1789990000000, "lastUpdatedAt": 1790003000000, "isArchived": 1, "isSubagent": 0, "value": {"type": "head", "composerId": "c3", "createdAt": 1789990000000, "lastUpdatedAt": 1790003000000, "isArchived": true, "agentLocation": {"type": "local", "status": "active"}, "agentLocationHistory": [{"id": "l1", "reason": "created", "timestamp": 1789990000000}], "workspaceIdentifier": {"id": "0123456789abcdef0123456789abcdef", "uri": {"fsPath": "/srv/proj"}}, "totalLinesAdded": 0, "totalLinesRemoved": 0}}
```

`ItemTable` settings blob:

```json
{"composerState": {"modes4": [{"id": "agent", "autoRun": true, "smartModeAutoRun": false, "fullAutoRun": false}], "yoloCommandAllowlist": ["curl", "head"], "yoloCommandDenylist": [], "yoloOutsideWorkspaceDisabled": true}, "aiSettings": {"modelLastUsedAt": {"grok-4.7": "2026-10-01T09:59:00.000Z"}}}
```

with `cursor/sandboxAutoRunSettingEnabled` `true`.
`.cursor/projects/srv-proj/agent-transcripts/c1/c1.jsonl`, the live conversation's copy, whose
user lines match the bubbles `u1` and `u2` (10:00Z and 10:01Z, written at UTC-4):

```json
{"role": "user", "message": {"content": [{"type": "text", "text": "<timestamp>Thursday, Oct 1, 2026, 6:00 AM (UTC-4)</timestamp>\n<user_query>Add a subtract function</user_query>"}]}}
{"role": "assistant", "message": {"content": [{"type": "text", "text": "Added subtract()."}]}}
{"type": "turn_ended", "status": "success"}
{"role": "user", "message": {"content": [{"type": "text", "text": "<timestamp>Thursday, Oct 1, 2026, 6:01 AM (UTC-4)</timestamp>\n<user_query>Fetch the page with curl</user_query>"}]}}
{"role": "assistant", "message": {"content": [{"type": "tool_use", "name": "Shell", "input": {"command": "curl -s example.test | head", "description": "Fetch the page", "required_permissions": ["full_network"]}}]}}
{"type": "turn_ended", "status": "success"}
```

followed by a truncated line written with `write_bad_line`; and
`.cursor/projects/srv-proj/agent-transcripts/c3/c3.jsonl`, the deleted conversation's:

```json
{"role": "user", "message": {"content": [{"type": "text", "text": "<timestamp>Wednesday, Sep 30, 2026, 9:30 PM (UTC-4)</timestamp>\n<user_query>c3 deleted question</user_query>"}]}}
{"role": "assistant", "message": {"content": [{"type": "text", "text": "Answer kept only here."}]}}
{"type": "turn_ended", "status": "success"}
```

`ai-code-tracking.db` `ai_code_hashes`, two rows: (`hash` `h1`, `source` `composer`,
`fileExtension` `py`, `fileName` `/srv/proj/calc.py`, `requestId` `r1`, `conversationId` `c1`,
`timestamp` 1790000003000, `model` `grok-4.6`) and (`hash` `h2`, the same but `conversationId`
`c3`, `requestId` `r9`, `timestamp` 1789990060000), the second naming the deleted conversation.
Noise files
the parser must not want: `.cursor/statsig-cache.json` and
`.cursor/projects/srv-proj/mcps/cursor-ide-browser/SERVER_METADATA.json`. Expected: for `c1`, a
`session name:` row, `model: grok-4.6`, the user row, a thinking row only with `include_thinking`,
the read's `tool_use` and `tool_result` with no permission row, the assistant row, `model:
grok-4.6 -> grok-4.7`, three commands each with `tool_use`, a `permission decision:` row
(`run`, `skip` with a `[rejected] ` result, `allowlistCommands` with `allowlisted=curl,head`)
and `tool_result`, the editor window's allow-always command (`tc7`) with `tool_use`, a
`permission decision: run` row ending `allowlisted=python3` and `tool_result`, and the pending
command (`tc8`) with `tool_use` and `permission prompt pending: run_terminal_command_v2
reason=Not in allowlist: python3 options=python3`, and no `permission decision:` or
`tool_result` row; for `c2`, rows from the chunk and `conversation archived`; for `c3`,
`conversation deleted`, the JSON blob's `user` row (the protobuf blob yields nothing), the
transcript's `user` row (`c3 deleted question` at 2026-10-01T01:30:00.000Z) and `assistant` row,
and its `ai code:` row; for `d1`, `unsent draft: draft not sent`;
standing rows for the two allowlist entries, the `agent` run mode `Allowlist (with Sandbox)` and
the switch; `model last used: grok-4.7`; `c1`'s `ai code:` row; and from `c1`'s transcript two
`user` rows (`Add a subtract function` at 10:00:00.000Z, `Fetch the page with curl` at
10:01:00.000Z) that deduplicate against `u1` and `u2`, an `assistant` row that matches the
bubble's text, a `tool_use` row with `tool_name` `Shell`, then the `parser:` row for the
truncated line.

Two cloud agents. `conversation-search.db` `conversations` row (`fts_rowid` 10, `source`
`cloud-cache`, `scope` and `cache_fingerprint` 64 hex digits, `id` `bc-e1`, `title` `Describe the
repository`, `branches` `''`, `updated_at` 1790020060000, `is_archived` 1) and its
`conversation_fts_content` row (`id` 10, `c0` the title, `c1` a body of two lines, the prompt and
the reply run together, `c2` `''`), beside empty content rows for the local conversations; in
`ItemTable`, the agent list under each spelling of the account id, the second agent listed
only, and the repository list:

```json
{"key": "cloudAgentRepository.agents.auth0|user_01", "value": [{"bcId": "bc-e1", "cloudTargetKey": "github:owner/demo:main:0123", "createdAt": 1790020000000, "environmentPublicId": "e0", "hasPendingInteraction": false, "hasStartedVm": true, "isArchived": true, "isKilled": false, "isOwnedByDifferentTeamMember": false, "isUnread": false, "lastMessageActivityAtMs": 1790020060000, "lastReadAtMs": 1790020090000, "modelDetails": {"maxMode": false, "modelName": "grok-4.7"}, "name": "Describe the repository", "repoUrl": "github.com/owner/demo", "repoUrls": ["github.com/owner/demo"], "requestedModel": {"builtInModel": "", "maxMode": false, "modelId": "grok-4.7", "parameters": [{"id": "context", "value": "256k"}]}, "rowUpdatedAtMs": 1790020120000, "source": 16, "status": 2, "transportRepoUrls": ["https://github.com/owner/demo"], "updatedAt": 1790020060000, "usePrivateWorker": false, "workflowStatus": 2, "workspaceRootPath": "/home/ubuntu/demo"}]}
{"key": "cloudAgentRepository.agents.auth0%7Cuser_02", "value": [{"bcId": "bc-e2", "createdAt": 1790021000000, "hasStartedVm": false, "isArchived": false, "isKilled": false, "modelDetails": {"modelName": "grok-4.7"}, "name": "Summarise the web repository", "repoUrl": "github.com/owner/web", "requestedModel": {"modelId": "default"}, "rowUpdatedAtMs": 1790021120000, "status": 1, "workflowStatus": 1}]}
{"key": "cloudEnvironmentTemplate.githubRepos", "value": [{"name": "owner/demo", "scmRepoNodeId": "R_0", "type": "github", "url": "https://github.com/owner/demo"}, {"name": "owner/web", "scmRepoNodeId": "R_1", "type": "github"}]}
```

with a `bcCachedDetails:bc-e1` protobuf row in `cursorDiskKV` and a
`secret://{"extensionId":"anysphere.cursor-resolver-helper","key":"connectionToken:bc-e1"}` row,
neither of which yields anything. Expected: for `bc-e1`, `conversation index: Describe the
repository source=cloud-cache archived=1`, `session origin: remote cursor cloud agent` at
`updated_at` and the `cached conversation text (conversation-search.db, source=cloud-cache):`
row with the whole body, then `cloud agent: Describe the repository repo=github.com/owner/demo
status=2 workflowStatus=2 archived=true killed=false vmStarted=true (from state.vscdb
cloudAgentRepository.agents, rowUpdatedAtMs)`, `source host: github.com owner/demo (cloud
agent)` and `model: grok-4.7`, with `project_path` `github.com/owner/demo` and the session's
`origin` `remote`; for `bc-e2`, the same rows from the list with `session origin: remote cursor
cloud agent` at its `createdAt` and `model: grok-4.7 requested=default`; and two `source host
repository:` rows with an empty `session_id`, the first naming the URL, the second the name. The
local conversations' empty content rows yield no cached-text row.

MCP servers and plugins, in `ItemTable`, each key suffix the URL-safe base64 of `[<scope>]
<field>`, without padding unless said: for the scope
`plugin-calendar-calendar::mcpScope:profile:ZGVmYXVsdA` (the profile `default` in base64), the
`mcpOAuth.global` fields `mcp_server_url` (`"https://calendar.mcp.example.test/mcp"`),
`mcp_oauth_updated_at_ms` (1790849300000) and `mcp_tokens_canonical_migration_complete`
(`true`); for a second server scope, `plugin-acme-tracker-tracker::mcpScope:profile:ZGVmYXVsdA`,
only `mcp_server_url` (`https://tracker.example.test/mcp`, its key padded), with no tokens and
no update time; for the attempt scope `oauth-attempt:a0000001-0000-4000-8000-000000000001`, a
`mcp_server_url`; the key `mcpOAuth.extensionMigrationComplete` (`true`); the `mcpOAuth.secret`
fields `mcp_tokens` and `mcp_client_information` of the calendar scope and `mcp_code_verifier`
of the attempt scope, each value `{"type": "Buffer", "data": [...]}` with the sentinel
`CURSOR-SECRET-SENTINEL` inside; `cursor.plugins.installedIds.no-team|no-workspace` and
`cursor.plugins.installedIds.no-team|file:///srv/proj`, each `[{"id": "12345678", "sources":
["user"]}]`; and `cloudAgentRepository.mcpAuthResolution` for the cloud agent `bc-e1` above,
its `toolCallId` `toolu_mcp_auth_01` and `fc_mcp_auth_01` joined by a newline:

```json
{"agentId": "bc-e1", "authId": "f0000001-0000-4000-8000-000000000001", "nonce": "f0000002-0000-4000-8000-000000000002", "queryId": "f0000003-0000-4000-8000-000000000003", "toolCallId": "toolu_mcp_auth_01\nfc_mcp_auth_01", "userDecision": "accepted", "version": 1}
```

In `cursorDiskKV`, one MCP tool bubble appended to a local conversation, under the header
`a2/t:call-m1-5` + newline + `fc_m1_5`:

```json
{"type": 2, "capabilityType": 15, "startedAtMs": 1790848910000, "completedAtMs": 1790848911000, "toolFormerData": {"name": "mcp-google-calendar-list_events", "tool": 19, "status": "completed", "toolCallId": "call-m1-5\nfc_m1_5", "modelCallId": "model-call-m1", "toolIndex": 5, "rawArgs": "{\"calendarId\": \"primary\", \"timeMin\": \"2026-10-01T00:00:00Z\"}", "params": "{\"calendarId\": \"primary\"}", "result": "{\"events\": [{\"summary\": \"Standup\"}]}", "additionalData": {"status": "success"}, "toolCallBinary": "Q0FMTEJJTkFSWQ=="}}
```

and `User/globalStorage/mcp-oauth-attempts/0000.json` under the IDE's data directory:

```json
{"activeIdentifier": "plugin-calendar-calendar::mcpScope:profile:default", "attemptId": "a0000002-0000-4000-8000-000000000002", "clientInformation": {"client_id": "client-0123456789abcdef", "grant_types": ["authorization_code", "refresh_token"], "redirect_uris": ["cursor://anysphere.cursor-mcp/oauth/callback"], "response_types": ["code"]}, "createdAtMs": 1790849280000, "identifier": "plugin-calendar-calendar", "owner": {"workspaceId": "empty-window"}, "serverUrl": "https://calendar.mcp.example.test/mcp", "transportServerUrl": "https://calendar.mcp.example.test/mcp"}
```

The settings files' modification time is 2026-10-01T14:00:00Z. Expected, with an empty
`session_id` except where said: `mcp server: plugin-acme-tracker-tracker
url=https://tracker.example.test/mcp (from state.vscdb mcpOAuth.global, file mtime)` at the file
time; `mcp server: plugin-calendar-calendar url=https://calendar.mcp.example.test/mcp (from
state.vscdb mcpOAuth.global, mcp_oauth_updated_at_ms) tokens=stored (ciphertext, not read)` at
10:08:20Z; nothing for the attempt scope or the migration flags; `plugin installed: id=12345678
sources=user scope=file:///srv/proj (from state.vscdb cursor.plugins.installedIds, file mtime)`
with `project_path` `/srv/proj` and the same with `scope=no-workspace`, at the file time; `mcp
oauth attempt: server=https://calendar.mcp.example.test/mcp client_id=client-0123456789abcdef
workspace=empty-window (from mcp-oauth-attempts/0000.json)` at 10:08:00Z; `permission decision:
accepted mcp auth for cloud agent bc-e1 tool_call=<toolCallId> (from state.vscdb
cloudAgentRepository.mcpAuthResolution, file mtime)` with `session_id` `bc-e1`, `tool_use_id`
the `toolCallId`, the agent's `project_path` `github.com/owner/demo` and `model` `grok-4.7`; for
the bubble a `tool_use` row (`tool_name` `mcp-google-calendar-list_events`, the text the
`params`) at 10:01:50Z and a `tool_result` row (the `result`) at 10:01:51Z, and no permission
row; and neither the sentinel nor the `toolCallBinary` value in any row. The attempt file
rewritten as `{"serverUrl": "https://x.test/mcp"}` gives `mcp oauth attempt:
server=https://x.test/mcp client_id=- workspace=- (from mcp-oauth-attempts/0000.json, file
mtime)` at its modification time, cut mid-value the `parser: unreadable` row, and as `[]` the
`parser: ... is not a JSON object` row.

#### MCP server and plugin rows

From the MCP OAuth keys and the plugin records of
[MCP servers and plugins](#mcp-servers-and-plugins), with an empty `session_id`; only key names
are decoded, and no `mcpOAuth.secret.*` value is read:

- for each scope with an `mcpOAuth.global` field `mcp_server_url`, `mcp server: <scope before
  ::> url=<value> (from state.vscdb mcpOAuth.global, mcp_oauth_updated_at_ms)` at that scope's
  `mcp_oauth_updated_at_ms`, or, when the scope has none, the same text ending `(from state.vscdb
  mcpOAuth.global, file mtime)` at the file's modification time, followed by ` tokens=stored
  (ciphertext, not read)` when an `mcpOAuth.secret` key for the same scope has the field
  `mcp_tokens`; the `oauth-attempt:` scopes and the `mcpOAuth.extensionMigrationComplete` key
  yield nothing;
- for each entry of a `cursor.plugins.installedIds.<team>|<scope>` key, `plugin installed:
  id=<id> sources=<sources, comma-separated> scope=<scope> (from state.vscdb
  cursor.plugins.installedIds, file mtime)` at the file's modification time, the scope the
  key's segment after the last `|` (`no-workspace`, or a folder's `file://` URI, whose decoded
  path is the row's `project_path`, a Windows drive in the form VS Code's `fsPath` writes, with
  the drive letter in lower case, `c:\...`, so that it matches the composer headers'
  `workspaceIdentifier.uri.fsPath`);
- for each `User/globalStorage/mcp-oauth-attempts/<uuid>.json`, `mcp oauth attempt:
  server=<serverUrl> client_id=<clientInformation.client_id> workspace=<owner.workspaceId>
  (from mcp-oauth-attempts/<file>)` at `createdAtMs`, or, when it has none, the same text
  ending `(from mcp-oauth-attempts/<file>, file mtime)` at the file's modification time; a file
  that is not JSON is one `parser: unreadable <rel>: <reason>` row and one that is JSON but not
  an object one `parser: <rel> is not a JSON object` row ([Bad-line rows](#bad-line-rows)).

The plugin cache is excluded and not read, and the per-project `mcps/` manifests and
`agent-tools/` outputs yield nothing.

For the CLI parser of [issue 235](https://github.com/seanthegeek/doubleagent/issues/235), one CLI session in `/srv/proj`, under
`.cursor/chats/6da544f59eb1005eef665ccbade87510/s1/` (the directory name the md5 of `/srv/proj`).
`store.db` `meta` row `0`, the hex encoding of
`{"agentId":"a1","blobEncryptionKey":"<64 zeros>","createdAt":1790010000000,"isRunEverything":false,"lastUsedModel":"grok-4.7","latestRootBlobId":"<64 hex>","mode":"default","name":"Run python"}`:

```text
7b226167656e744964223a226131222c22626c6f62456e6372797074696f6e4b6579223a2230303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030222c22637265617465644174223a313739303031303030303030302c22697352756e45766572797468696e67223a66616c73652c226c617374557365644d6f64656c223a2267726f6b2d342e37222c226c6174657374526f6f74426c6f624964223a2261626162616261626162616261626162616261626162616261626162616261626162616261626162616261626162616261626162616261626162616261626162222c226d6f6465223a2264656661756c74222c226e616d65223a2252756e20707974686f6e227d
```

`blobs`, four JSON rows, each `id` the sha256 of its `data` as the builder computes it, beside one
protobuf row (`0a 20` and 32 bytes) the parser must skip:

```json
{"role": "user", "content": [{"type": "text", "text": "<timestamp>Thursday, Oct 1, 2026, 9:00 AM (UTC-4)</timestamp>\n<user_query>Run python3 twice</user_query>"}], "providerOptions": {"cursor": {"requestId": "q1"}}}
{"role": "assistant", "id": "m1", "content": [{"type": "reasoning", "text": "Run it.", "signature": "sig", "providerOptions": {"cursor": {"modelName": "grok-4.7-high-fast"}}}, {"type": "tool-call", "toolCallId": "tc5", "toolName": "Shell", "args": {"command": "python3 -c 'print(1+1)'", "description": "Add"}}, {"type": "tool-call", "toolCallId": "tc6", "toolName": "Shell", "args": {"command": "python3 -c 'print(1+1)'", "description": "Add again"}}], "providerOptions": {"cursor": {"modelProviderMessageId": "p1"}}}
{"role": "tool", "id": "tc5", "content": [{"type": "tool-result", "toolCallId": "tc5", "toolName": "Shell", "result": "2", "experimental_content": [{"type": "text", "text": "2"}]}], "providerOptions": {"cursor": {"highLevelToolCallResult": {"isError": false, "output": {"success": {"command": "python3 -c 'print(1+1)'", "stdout": "2\n", "executionTime": 40}, "isBackground": false, "sandboxPolicy": {"type": "none"}}}}}}
{"role": "tool", "id": "tc6", "content": [{"type": "tool-result", "toolCallId": "tc6", "toolName": "Shell", "result": "User skipped this"}], "providerOptions": {"cursor": {"highLevelToolCallResult": {"isError": true, "output": {"rejected": {"command": "python3 -c 'print(1+1)'", "reason": "skip it", "workingDirectory": "/srv/proj"}}}}}}
```

`meta.json` of the session, `prompt_history.json` beside it, an empty start's `meta.json` in
`.cursor/chats/6da544f59eb1005eef665ccbade87510/s0/` (no `store.db`), and
`.cursor/projects/srv-proj/.workspace-trusted`:

```json
{"createdAtMs": 1790010000000, "updatedAtMs": 1790010120000, "cwd": "/srv/proj", "hasConversation": true, "schemaVersion": 1, "title": "Run python"}
["Run python3 twice", "skip it"]
{"createdAtMs": 1790009900000, "updatedAtMs": 1790009900000, "cwd": "/srv/proj", "hasConversation": false, "schemaVersion": 1}
{"trustedAt": "2026-10-01T12:58:00.000Z", "workspacePath": "/srv/proj"}
```

with `cli-config.json` `permissions.allow` `["Shell(ls)", "Shell(python3)"]` and
`modelSelectionHistory` `["grok-4.7", "default"]`. Expected, by the plan of the issue: a session
`s1` with `project_path` `/srv/proj` from `meta.json`, `session name: Run python`, `model:
grok-4.7-high-fast` (with the selection `grok-4.7` beside it), the `user` row with the wrapper
stripped, a thinking row only with `include_thinking`, two `tool_use` rows (`Shell`, `tc5` and
`tc6`), a `tool_result` for `tc5` and a `[rejected] ` one for `tc6` with its permission row, a
standing row for the trust grant at 12:58Z, a standing row for `isRunEverything` false, the
history's two prompts as secondary `user` rows with empty `timestamp_utc`, nothing for the
protobuf blob, and `s0` reported as an empty start or not at all.

#### Cursor Agent CLI parser

The `cursor` parser as it stands (analyzer 0.20.0) reads a CLI session through the files the CLI
shares with the IDE: the transcript JSONL's user lines (timed to the minute by their tags),
assistant text and `tool_use` blocks with the CamelCase names, every row marked `recovered`,
since no `state.vscdb` holds the session; the `source` `cli` rows of `ai-code-tracking.db` as `ai
code:` rows; and the `permission setting:` rows of `cli-config.json` (`approvalMode`, each
`permissions.allow` entry, `sandbox.mode`). Its `session_id` is the CLI session id, its
`project_path` is empty because `meta.json` is not read, and its origin is `local`. It does not
read `store.db`, `meta.json`, `prompt_history.json` or `.workspace-trusted`.

The plan of [issue 235](https://github.com/seanthegeek/doubleagent/issues/235), in order:

1. Session facts: `meta.json` (`project_path` from `cwd`, the session's times), `meta.0`
   (`model: <lastUsedModel>`, a standing row for `isRunEverything`, the title as `session name:`),
   `prompt_history.json` as a past-use source and `.workspace-trusted` as a standing permission
   row at `trustedAt`.
2. The JSON message blobs: `user`, `assistant`, `thinking`, `tool_use` and `tool_result` rows,
   `model:` rows from the reasoning blocks' resolved model with the selection from
   `cli-config.json` beside it, and permission rows from `output.rejected` against
   `output.success`, joined on `toolCallId`.
3. Order and per-call times from the protobuf state nodes, which needs their descriptors: the
   CLI bundle (`~/.local/share/cursor-agent/versions/<ver>/*.index.js`) is JavaScript, so they
   are in a protobuf-es registry (the JavaScript protobuf runtime's in-code descriptors) rather
   than a binary, to be extracted with `analyzer/tools/proto_descriptors.py` or a variant that
   reads JavaScript and decoded with `doubleagent/protobuf.py`. Until then the rows from the JSON
   blobs take the transcript JSONL's order.

Globs for the CLI files, relative to the home:

```text
^\.cursor/chats/[0-9a-f]{32}/[^/]+/store\.db$
^\.cursor/chats/[0-9a-f]{32}/[^/]+/meta\.json$
^\.cursor/chats/[0-9a-f]{32}/[^/]+/prompt_history\.json$
^\.cursor/projects/[^/]+/\.workspace-trusted$
```

## Confidence

- High, observed on macOS (Cursor IDE 3.24.18 to 3.26.13, one step per listing): the IDE's files
  and key families, the record shapes, the permission and model records, the archive and delete
  results, and the 3.26.13 header chunks.
- High, observed on Linux (Cursor IDE deb 3.24.18 and 3.25.9, one step per listing): the
  `~/.config/Cursor` and `~/.local/state/cursor/agent-stores` layout, the absence of the
  Agent CLI copy until a cloud feature is used, the absence of `update-supervisor`, the plaintext
  tokens beside the encrypted `secret://` values, the source-host keys, the pending-prompt and
  editor-window allow-always records, and the archive and delete results.
- High, observed on Windows (Cursor IDE 3.26.13 per-user installer, arm64, one step per
  listing): the `AppData/Roaming/Cursor` and `AppData/Local/Cursor/AgentStores` layout, the
  updater's files before and at the update to 3.26.16, the plaintext tokens, the decision,
  pending and standing-list records, the delete's result on a conversation never archived, the
  source-host keys, and a
  cloud agent's local records, archive and reopening, the remote-control enrolment and the
  absence of a local record of a paired device, a plugin's install and MCP OAuth records, the
  MCP tool bubbles and the cloud agent's MCP authorisation record; two cloud agents with one turn
  each, on arm64, where the embedded worker did not start.
- Not observed: remote steering from a paired device (the worker does not start on Windows
  arm64); to be observed on macOS.
- High, observed on macOS (Cursor Agent CLI 2026.10.19-6bbf2c4, one step per listing): the CLI's
  files under `~/.cursor` and `~/.local`, the chat store's tables, the `meta.0` keys and the JSON
  message shapes, the keychain items, and what `agent logout` and the project's removal left.
- High (L1 bundle): the CLI entries the install did not write (`acp-sessions/`, worktrees, hooks,
  plans), the `projects/` name truncation, and the CLI's `auth.json` logic on Linux and Windows;
  the IDE's run-mode names and delete commands.
- Medium: `.cursor-server` (convention plus L4).
- Low: `~/.cursor_info` (a single L4 claim).
- Not determined: the schema of the CLI chat store's protobuf nodes (no descriptor extracted), so
  the CLI's message order and per-call times; whether other CLI builds split or encrypt the blobs,
  which 2026.10.19-6bbf2c4 stored whole and in plaintext beside `blobEncryptionKey` (codeburn
  986); the `cursor-api-key` and `cursor-bedrock-*` keychain items (bundle only); the derivation
  of `authCacheKey` and the value of `meta.json` `schemaVersion`; whether the Windows CLI uses
  `%APPDATA%\Cursor\auth.json` or the IDE's directory of the same name; the status of
  `.cursorindexingignore`; the protobuf `agentKv` blobs; the transcript's tool-name mapping beyond
  the five names of one install; the form of a positive UTC offset in the transcript tag, and
  whether the tag truncates or rounds the seconds; whether `messageRequestContext:*` rows still
  appear on any 3.x build; the unit of `ai_code_hashes.createdAt`; subagent composers
  (`isSubagent`, `subagentComposerIds`) and worktree agents; what deleting the project directory
  leaves for the IDE; the meaning of the fields of `bcCachedDetails` (no descriptor extracted)
  and of the cloud agent's `status`, `workflowStatus` and `source` numbers; whether a cloud
  agent's cached text grows with more turns or keeps one block, and whether it can be deleted
  locally or only on `cursor.com/agents`; whether `.git/cursor/crepe` fills later on Windows;
  what decides whether a header carries `agentLocation`; why some local rows of
  `conversation-search.db` have no body; what `toolCallBinary` encodes; the meaning of the
  `authId`, `queryId` and `version` fields of `cloudAgentRepository.mcpAuthResolution`; the
  value of the cloud MCP server list `agentData.cacheStorage.glass.cloudMcpServers.v1.personal.*`;
  how the export's blocks map to
  typed and injected turns; whether the prompt offers an allow-list entry for an interpreter
  command line such as `powershell -Command`; which builds name the shadow copies
  `cursor-retrieval` and which `cursor-commits`; the step that first wrote the Windows
  `remoteAgentHost` keys; whether the editor window's add-to-allow-list shortcut
  is kept in memory for the session or was lost at the quit, and whether other builds persist
  it; what the daily `projects/` cleanup removes; whether the Linux `secret://` values are
  encrypted through libsecret, and what the GNOME keyring item holds; the Agents window, the RPM
  and AppImage builds and the x64 deb on Linux; the Cursor IDE 3.26.16 on Windows, beyond its
  update, and the values `update-supervisor\win32\<installationId>\state.json` `stage` takes;
  and the Agent CLI on Windows and on Linux.
