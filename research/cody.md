# cody

Sourcegraph Cody (catalog agent `cody`) is Sourcegraph's coding assistant, shipped as a VS Code
extension (`sourcegraph.cody-ai`), a JetBrains plugin and a `cody` CLI; the last two run the same
TypeScript "agent" in a Node process. Cody keeps all chats of all accounts in one JSON object,
`AccountKeyedChatHistory`, stored under the key `cody-local-chatHistory-v2`. In VS Code chat
history and the access token are not files: that object sits inside the `sourcegraph.cody-ai`
row of the editor's `User/globalStorage/state.vscdb`, which the `vscode` entries already collect,
and the extension's own `globalStorage` folder holds only a local search index. The JetBrains
plugin keeps the same chat history as files under a directory named `Cody-nodejs`, placed by
`env-paths` (the npm library that picks a per-platform data directory); the `cody chat` CLI keeps
it in memory only. Deep Cody is the agentic context mode in which a helper model gathers context
with tools, the terminal included, before the chat model answers. Source is the frozen public
snapshot, so later releases may differ. The `cody` parser reads the history from both stores.

## Source and evidence

[sourcegraph/cody-public-snapshot](https://github.com/sourcegraph/cody-public-snapshot), commit
[`8e20ac6c1460c08b0db581c0204658112a246eda`](https://github.com/sourcegraph/cody-public-snapshot/commit/8e20ac6c1460c08b0db581c0204658112a246eda)
(2025-08-01), VS Code extension version 1.116.0
([`vscode/package.json:3-7`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/package.json#L3-L7)).
The source is frozen: it is the archived copy of the now-private Cody repository, which Sourcegraph
moved to a private repository just after this copy was taken
([`README.md:1-5`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/README.md#L1-L5));
the README names
[`d7fc674`](https://github.com/sourcegraph/cody-public-snapshot/commit/d7fc6741e7893e3f6e29efe58043f1afe08d505f)
as the last Apache-2.0 commit, while the `LICENSE` file at HEAD is still Apache-2.0 text. The
shipped extension may have moved on; every claim below is about the extension as of August 2025.
Home-directory resolution of `env-paths` is cited from its source at the locked version 2.2.1
(`pnpm-lock.yaml`). VS Code storage mechanics are cited from the PearAI fork of VS Code 1.96.4, the
only VS Code checkout read for this document; see [vscode.md](vscode.md) for upstream.

### Observed

- No real install. In particular no real install has shown a Deep Cody confirmation step: the layout
  the parser handles is not observed on any real install, source-derived
  (sourcegraph/cody-public-snapshot at
  [`8e20ac6`](https://github.com/sourcegraph/cody-public-snapshot/commit/8e20ac6c1460c08b0db581c0204658112a246eda);
  [Permission records](#permission-records), [Permission rows](#permission-rows)).

### Source reviews

- 2026-10-21: sourcegraph/cody-public-snapshot at
  [8e20ac6](https://github.com/sourcegraph/cody-public-snapshot/commit/8e20ac6c1460c08b0db581c0204658112a246eda)
  (VS Code extension 1.116.0): the parser plan now reads the standing settings, the model
  preferences and the legacy JetBrains history; the review added the serialised form of that
  history, the JetBrains account files and what they hold, and the source-host finding.
- 2026-10-19: sourcegraph/cody-public-snapshot at
  [`8e20ac6`](https://github.com/sourcegraph/cody-public-snapshot/commit/8e20ac6c1460c08b0db581c0204658112a246eda),
  still the repository's HEAD and unchanged since 2025-08-01, so nothing later than the VS Code
  extension 1.116.0 can be read from source. The review added the Deep Cody confirmation and
  standing-setting records, the JetBrains IDE settings and project files, the legacy JetBrains
  history, `cody-model-preferences`, the telemetry that carries prompts and tool calls, and what a
  delete leaves, and corrected the `cody chat` CLI to in-memory history.

## Collection

### Per-user storage

The layout differs by surface: the VS Code extension keeps its state in the editor's own
storage, the JetBrains plugin and the CLI in an `env-paths` directory, and the JetBrains plugin
also in the IDE's `options/` directory.

#### Scratch directories

None. Deep Cody's terminal tool and the agentic chat's `run_terminal_command` both run in the first
workspace folder
([`vscode/src/commands/context/shell.ts:37`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/commands/context/shell.ts#L37);
[`vscode/src/chat/chat-view/tools/shell.ts:52-69`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/chat/chat-view/tools/shell.ts#L52-L69)),
and no clone, worktree or per-chat directory is created outside the internal `cody bench` evaluation
command, whose `cody-bench-` worktree sits in the system temp directory
([`agent/src/cli/command-bench/testTypecheck.ts:49-55`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/agent/src/cli/command-bench/testTypecheck.ts#L49-L55)).
The `symf/indexroot/<path>` directories are indexes named after real workspaces, not working
directories.

#### VS Code extension

- Global state: `localStorage.setStorage(context.globalState)`
  ([`vscode/src/main.ts:133-136`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/main.ts#L133-L136)).
  VS Code stores an extension's global state as one JSON value under the extension id in `ItemTable`
  of `User/globalStorage/state.vscdb`
  ([PearAI `extensionStorage.ts:159-161`](https://github.com/trypear/pearai-app/blob/d930f0233c14668df9f85c6a78a81828f4251194/src/vs/platform/extensionManagement/common/extensionStorage.ts#L159-L161),
  [`170-175`](https://github.com/trypear/pearai-app/blob/d930f0233c14668df9f85c6a78a81828f4251194/src/vs/platform/extensionManagement/common/extensionStorage.ts#L170-L175)).
  So the row key is `sourcegraph.cody-ai` and its value is a JSON object whose keys include
  `cody-local-chatHistory-v2` (all chat history), `SOURCEGRAPH_CODY_ENDPOINT` (last server),
  `SOURCEGRAPH_CODY_ENDPOINT_HISTORY`, `sourcegraphAnonymousUid`, `cody-model-preferences`,
  `cody-github-repo-metadata`
  ([`vscode/src/services/LocalStorageProvider.ts:29-39`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/services/LocalStorageProvider.ts#L29-L39)).
  History is keyed by `<endpoint>-<username>`
  ([`LocalStorageProvider.ts:393-397`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/services/LocalStorageProvider.ts#L393-L397)),
  so one row can hold several accounts' chats.
- `User/globalStorage/sourcegraph.cody-ai/symf/`: the downloaded `symf` search binary
  `symf-<version>-<arch>-<platform>`
  ([`vscode/src/local-context/download-symf.ts:53-56`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/local-context/download-symf.ts#L53-L56),
  [`118`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/local-context/download-symf.ts#L118))
  and `symf/indexroot/<absolute workspace path>/`, one index per indexed folder; on Windows the
  drive colon is dropped
  ([`vscode/src/local-context/symf.ts:74`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/local-context/symf.ts#L74),
  [`396-411`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/local-context/symf.ts#L396-L411)),
  plus `.tmp/`, `.trash/` and `.failed/<path with / as __>` sentinels
  ([`symf.ts:339`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/local-context/symf.ts#L339),
  [`514-517`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/local-context/symf.ts#L514-L517)).
  Enabled by default (`cody.experimental.symf.enabled`;
  [`vscode/src/extension.node.ts:33-35`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/extension.node.ts#L33-L35)).
- `~/.vscode/cody.json`: user custom commands, in the home, not the user data directory
  ([`vscode/src/commands/services/custom-commands.ts:20`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/commands/services/custom-commands.ts#L20),
  [`46-48`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/commands/services/custom-commands.ts#L46-L48);
  [`vscode/src/commands/types.ts:9-15`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/commands/types.ts#L9-L15)).
  The `vscode` entries do not collect it today.
- Output channel logs go to the editor's `logs/` tree, collected under `vscode`.

#### JetBrains plugin and CLI (agent)

Paths come from `envPaths('Cody')`
([`lib/shared/src/codyPaths.ts:3`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/lib/shared/src/codyPaths.ts#L3)),
which appends `-nodejs` by default
([env-paths `index.js:54-59`](https://github.com/sindresorhus/env-paths/blob/62d4ec8cd42f1a419e00856fab949ca8286773f6/index.js#L54-L59)):

| | data | config | log |
| --- | --- | --- | --- |
| Linux | `~/.local/share/Cody-nodejs` | `~/.config/Cody-nodejs` | `~/.local/state/Cody-nodejs` |
| macOS | `~/Library/Application Support/Cody-nodejs` | `~/Library/Preferences/Cody-nodejs` | `~/Library/Logs/Cody-nodejs` |
| Windows | `%LOCALAPPDATA%\Cody-nodejs\Data` | `%APPDATA%\Cody-nodejs\Config` | `%LOCALAPPDATA%\Cody-nodejs\Log` |

([env-paths `index.js:9-47`](https://github.com/sindresorhus/env-paths/blob/62d4ec8cd42f1a419e00856fab949ca8286773f6/index.js#L9-L47);
XDG variables honoured on Linux only.) The agent sets `globalStorageUri` and `storageUri` to data,
`logUri` to log, and copies extension resources into `<config>/dist`
([`agent/src/agent.ts:120-137`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/agent/src/agent.ts#L120-L137),
[`159-161`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/agent/src/agent.ts#L159-L161),
[`180-189`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/agent/src/agent.ts#L180-L189)).

- `<data>/JetBrains-globalState/`: the JetBrains plugin asks for server-managed global state
  ([`jetbrains/.../CodyAgentService.kt:203`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/jetbrains/src/main/kotlin/com/sourcegraph/cody/agent/CodyAgentService.kt#L203)),
  with client name `JetBrains`
  ([`CodyAgent.kt:136-137`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/jetbrains/src/main/kotlin/com/sourcegraph/cody/agent/CodyAgent.kt#L136-L137)),
  so the agent opens `<data>/<name>-globalState` with `node-localstorage`
  ([`agent/src/agent.ts:1470-1476`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/agent/src/agent.ts#L1470-L1476);
  [`agent/src/global-state/AgentGlobalState.ts:115-121`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/agent/src/global-state/AgentGlobalState.ts#L115-L121)).
  Each key is one file named by its URL-encoded key, holding the JSON value
  ([`AgentGlobalState.ts:134-140`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/agent/src/global-state/AgentGlobalState.ts#L134-L140);
  [node-localstorage 3.0.5 `LocalStorage.js`](https://unpkg.com/node-localstorage@3.0.5/LocalStorage.js)
  `setItem`). The chat history is the file `JetBrains-globalState/cody-local-chatHistory-v2`, quota
  256 MB.
- `<data>/symf/`: the same binary and index as under [VS Code extension](#vs-code-extension).
- `<config>/user-settings.json`: CLI accounts
  ([`agent/src/cli/command-auth/settings.ts:6-36`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/agent/src/cli/command-auth/settings.ts#L6-L36)).
- `~/.cody/commands.json`: custom commands for agent clients
  ([`types.ts:13-14`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/commands/types.ts#L13-L14);
  [`custom-commands.ts:46-47`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/commands/services/custom-commands.ts#L46-L47)).

#### JetBrains IDE settings

The plugin also persists application-level components in the IDE's `options/` directory
(`~/.config/JetBrains/<product>`, `~/Library/Application Support/JetBrains/<product>`,
`%APPDATA%\JetBrains\<product>`, per
[Directories used by the IDE to store settings, caches, plugins and logs](https://www.jetbrains.com/help/idea/directories-used-by-the-ide-to-store-settings-caches-plugins-and-logs.html)),
which the catalog collects as `options/cody*.xml` on each platform:

- `cody.xml`: `GlobalCodySettings.jsonConfig`, a global `cody.*` settings document
  ([`jetbrains/.../config/GlobalCodySettings.kt:10-24`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/jetbrains/src/main/kotlin/com/sourcegraph/config/GlobalCodySettings.kt#L10-L24)).
  The plugin passes the agent the per-project file
  `<project>/.idea/.sourcegraph/cody_settings.json`, or `~/.sourcegraph/cody_settings.json` when the
  project has no base path, with this document as the fallback
  ([`ConfigUtil.kt:152-162`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/jetbrains/src/main/kotlin/com/sourcegraph/config/ConfigUtil.kt#L152-L162),
  [`184-199`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/jetbrains/src/main/kotlin/com/sourcegraph/config/ConfigUtil.kt#L184-L199)),
  so a JetBrains user's `cody.agentic.context.experimentalOptions` and `cody.mcpServers` are in one
  of those three. `~/.sourcegraph` is a catalog line and `.idea/.sourcegraph` a project table line
  (the project table's `.sourcegraph` names the project root's directory). The file is parsed
  as HOCON (Human-Optimized Config Object Notation, the superset of JSON that the Typesafe Config
  library reads), not JSON
  ([`ConfigUtil.kt:175-199`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/jetbrains/src/main/kotlin/com/sourcegraph/config/ConfigUtil.kt#L175-L199)).
- `cody_application_settings.xml`: feature toggles, `anonymousUserId` and
  `shouldAcceptNonTrustedCertificatesAutomatically`
  ([`CodyApplicationSettings.kt:9-25`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/jetbrains/src/main/kotlin/com/sourcegraph/cody/config/CodyApplicationSettings.kt#L9-L25)).
- `cody_accounts_settings.xml`: the pre-agent `CodyAccountsSettings` component, which holds only
  `activeAccountId`, the id of the active account
  ([`auth/deprecated/DeprecatedCodyAccountManager.kt:18-20`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/jetbrains/src/main/kotlin/com/sourcegraph/cody/auth/deprecated/DeprecatedCodyAccountManager.kt#L18-L20),
  [`44-49`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/jetbrains/src/main/kotlin/com/sourcegraph/cody/auth/deprecated/DeprecatedCodyAccountManager.kt#L44-L49),
  [`76-78`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/jetbrains/src/main/kotlin/com/sourcegraph/cody/auth/deprecated/DeprecatedCodyAccountManager.kt#L76-L78)).
- `cody_accounts.xml`: the pre-agent account list, `CodyAccounts`, one `account` element per
  account with `name`, `displayName`, `id` (a UUID) and the server's `url` and
  `customRequestHeaders`
  ([`DeprecatedCodyPersistentAccounts.kt:8-16`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/jetbrains/src/main/kotlin/com/sourcegraph/cody/auth/deprecated/DeprecatedCodyPersistentAccounts.kt#L8-L16);
  [`DeprecatedCodyAccount.kt:14-21`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/jetbrains/src/main/kotlin/com/sourcegraph/cody/auth/deprecated/DeprecatedCodyAccount.kt#L14-L21);
  [`auth/SourcegraphServerPath.kt:10-14`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/jetbrains/src/main/kotlin/com/sourcegraph/cody/auth/SourcegraphServerPath.kt#L10-L14)).
  The `id` is the `accountId` of the [Legacy JetBrains history](#legacy-jetbrains-history), and
  the migration keys the agent store by `<url>-<name>` from this file.

The project-level files are in [Project-local files](#project-local-files).

### Credentials

- VS Code: the access token is stored in SecretStorage three times, under the endpoint URL, under
  `cody.access-token`, and the token source under `<endpoint>cody.access-token.source`
  ([`vscode/src/services/SecretStorageProvider.ts:5-6`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/services/SecretStorageProvider.ts#L5-L6),
  [`97-112`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/services/SecretStorageProvider.ts#L97-L112)).
  These are encrypted `secret://{"extensionId":"sourcegraph.cody-ai","key":...}` rows in
  `state.vscdb`
  ([PearAI `mainThreadSecretState.ts:76-78`](https://github.com/trypear/pearai-app/blob/d930f0233c14668df9f85c6a78a81828f4251194/src/vs/workbench/api/browser/mainThreadSecretState.ts#L76-L78)).
- `cody.experimental.localTokenPath`: a setting that points at a JSON file with a `token` key, read
  instead of SecretStorage
  ([`SecretStorageProvider.ts:48-57`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/services/SecretStorageProvider.ts#L48-L57),
  [`225-235`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/services/SecretStorageProvider.ts#L225-L235)).
  Its location is arbitrary; read `settings.json` for it.
- JetBrains: secrets are client-managed
  ([`CodyAgentService.kt:204`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/jetbrains/src/main/kotlin/com/sourcegraph/cody/agent/CodyAgentService.kt#L204)),
  that is, in the IDE's own password store, not in `Cody-nodejs`. The pre-agent accounts kept
  their tokens there too, in PasswordSafe (the IntelliJ Platform's credential store) under the
  service name `Sourcegraph` and the account id
  ([`DeprecatedCodyAccountManager.kt:67-74`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/jetbrains/src/main/kotlin/com/sourcegraph/cody/auth/deprecated/DeprecatedCodyAccountManager.kt#L67-L74);
  [`DeprecatedCodyAccount.kt:23-24`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/jetbrains/src/main/kotlin/com/sourcegraph/cody/auth/deprecated/DeprecatedCodyAccount.kt#L23-L24)),
  so `cody_accounts_settings.xml` holds no secret and gets no secret glob. `cody_accounts.xml` is
  flagged: each account's `customRequestHeaders` can carry proxy credentials, as the CLI's
  `customHeaders` can.
- CLI: the OS keychain via `security`, Windows Credential Manager or `secret-tool`, no file fallback
  ([`agent/src/cli/command-auth/secrets.ts:8-14`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/agent/src/cli/command-auth/secrets.ts#L8-L14),
  [`204-213`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/agent/src/cli/command-auth/secrets.ts#L204-L213)).
  `user-settings.json` holds `serverEndpoint`, `username` and `customHeaders`, which may carry proxy
  auth.
- `cody.mcpServers.<server>` entries (VS Code `settings.json`, the JetBrains `cody_settings.json`
  files and `cody.xml`) take `headers` for an SSE (server-sent events, the HTTP transport) server
  and `env` for a stdio one, where MCP (Model Context Protocol) servers usually receive their tokens
  ([`vscode/src/chat/chat-view/tools/MCPManager.ts:31-43`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/chat/chat-view/tools/MCPManager.ts#L31-L43)).
  The JetBrains `options/cody.xml`, `~/.sourcegraph/cody_settings.json` and a project's
  `.idea/.sourcegraph/cody_settings.json` are therefore secret globs, like `cody_accounts.xml`;
  the editor's `settings.json` is shared with the editor's own state and is redacted by key
  ([Secrets](#secrets)).

### Exclusions

- The `symf-*` binary in each `symf/` directory.
- `<config>/dist`: copied extension resources.
- `symf/indexroot` index content was not sized; its directory names are
  the indexed workspace paths, which is evidence, so it is not proposed for
  exclusion until a live collection shows it is large.

### Project-local files

`.vscode/cody.json` (VS Code) and `.cody/commands.json` (agent clients) at the first workspace
folder
([`custom-commands.ts:36-39`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/commands/services/custom-commands.ts#L36-L39));
`.sourcegraph/*.rule.md` rules
([`vscode/src/rules/fs-rule-provider.ts:20-26`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/rules/fs-rule-provider.ts#L20-L26)).
No per-project database.

JetBrains project files, both in the project table:

- `.idea/cody_history.xml`: the plugin's pre-agent chat history, which the migration copied into
  the agent store and left in place, so it still holds a chat deleted in the plugin
  ([Legacy JetBrains history](#legacy-jetbrains-history)).
- `.idea/.sourcegraph/cody_settings.json`: the per-project `cody.*`
  settings, permission settings and MCP servers included
  ([JetBrains IDE settings](#jetbrains-ide-settings)).

### Where the project path is recorded

- Chat transcripts carry no workspace field; context items carry file URIs
  (`contextFiles[].uri`), from which the project can be inferred.
- `symf/indexroot/<path>` directory names are indexed workspace roots.
- VS Code `workspaceStorage/*/workspace.json` as usual.
- A JetBrains `.idea/cody_history.xml` or `.idea/.sourcegraph/` names its
  project by where it sits; nothing inside records the path.

## Parsing

### Transcript stores

- VS Code (and forks): `<User>/globalStorage/state.vscdb`, table `ItemTable`, row `key =
  'sourcegraph.cody-ai'`; `value` is the extension's whole global state as JSON
  ([PearAI VS Code fork `extensionStorage.ts:170-175`](https://github.com/trypear/pearai-app/blob/d930f0233c14668df9f85c6a78a81828f4251194/src/vs/platform/extensionManagement/common/extensionStorage.ts#L170-L175)),
  and `value["cody-local-chatHistory-v2"]` is the history
  ([`vscode/src/services/LocalStorageProvider.ts:29`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/services/LocalStorageProvider.ts#L29),
  [`190-212`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/services/LocalStorageProvider.ts#L190-L212)).
  Collected today under `vscode`.
- JetBrains: `<Cody-nodejs data>/JetBrains-globalState/cody-local-chatHistory-v2`, a file whose
  whole content is the JSON-encoded history
  ([`agent/src/global-state/AgentGlobalState.ts:115-121`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/agent/src/global-state/AgentGlobalState.ts#L115-L121),
  [`134-140`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/agent/src/global-state/AgentGlobalState.ts#L134-L140)).
  Home-relative globs: `.local/share/Cody-nodejs/JetBrains-globalState/cody-local-chatHistory-v2`,
  `Library/Application Support/Cody-nodejs/JetBrains-globalState/...`,
  `AppData/Local/Cody-nodejs/Data/JetBrains-globalState/...`.
- The whole history is rewritten on every save
  ([`vscode/src/chat/chat-view/ChatHistoryManager.ts:94-108`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/chat/chat-view/ChatHistoryManager.ts#L94-L108)).
- The `cody chat` CLI writes no history. Its client info asks for no global-state capability
  ([`agent/src/cli/command-chat.ts:141-158`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/agent/src/cli/command-chat.ts#L141-L158)),
  so the agent opens its global state without a directory
  ([`agent/src/agent.ts:1470-1480`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/agent/src/agent.ts#L1470-L1480)),
  which is an in-memory database
  ([`AgentGlobalState.ts:23-33`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/agent/src/global-state/AgentGlobalState.ts#L23-L33)).

Other sources of past use and model state are in [Other stores](#other-stores).

#### What survives a delete

"Delete All Chats" (`cody.chat.history.clear`, with a modal confirmation unless called with
`clear-all-no-confirm`) replaces only the signed-in account's entry in `cody-local-chatHistory-v2`
with an empty chat map, `{chat: {}}`, so other accounts' chats stay in the same value, and
`cody.chat.history.delete` removes one chat key and rewrites the whole value; neither writes a
tombstone or other marker
([`ChatsController.ts:505-531`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/chat/chat-view/ChatsController.ts#L505-L531);
[`LocalStorageProvider.ts:231-241`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/services/LocalStorageProvider.ts#L231-L241),
[`270-276`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/services/LocalStorageProvider.ts#L270-L276)),
so the parser has nothing to yield for a delete ([Deletion rows](#deletion-rows)). The one local
copy a delete or clear does not reach is the JetBrains plugin's legacy, project-local
`<project>/.idea/cody_history.xml` ([Legacy JetBrains history](#legacy-jetbrains-history)), which
the migration copied into the agent store and never deleted; it is also the only record of an
unpaired message the migration dropped. Cody keeps no per-prompt history file and no other copy of
its chats ([Other stores](#other-stores)). The analyzer reads the history value and, as a
secondary source, the legacy XML (see
[Secondary sources](../analyzer/docs/output.md#secondary-sources)).

### Record schema

`AccountKeyedChatHistory`
([`lib/shared/src/chat/transcript/messages.ts:193-204`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/lib/shared/src/chat/transcript/messages.ts#L193-L204);
key built at
[`LocalStorageProvider.ts:393-397`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/services/LocalStorageProvider.ts#L393-L397))
is:

```text
{ "<endpoint>-<username>": { "chat": { "<chatID>": SerializedChatTranscript } } }
```

The endpoint is a URL such as `https://sourcegraph.com/`, so split the key
on the last `-` only with care: usernames may contain hyphens, endpoints
rarely end in one.

`SerializedChatTranscript`
([`lib/shared/src/chat/transcript/index.ts:7-24`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/lib/shared/src/chat/transcript/index.ts#L7-L24)):
`id`, `chatTitle?`, `lastInteractionTimestamp`, `interactions[]` of `{humanMessage,
assistantMessage|null}`. `id` and `lastInteractionTimestamp` are both the chat's **creation** time
as `Date.toUTCString()`, e.g. `"Fri, 03 Oct 2026 10:00:00 GMT"` (UTC, second resolution), despite
the name
([`vscode/src/chat/chat-view/ChatBuilder.ts:97`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/chat/chat-view/ChatBuilder.ts#L97),
[`337-341`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/chat/chat-view/ChatBuilder.ts#L337-L341)).
Chats whose human message errored are not saved
([`ChatBuilder.ts:325-331`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/chat/chat-view/ChatBuilder.ts#L325-L331)).

`SerializedChatMessage`
([`messages.ts:156-171`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/lib/shared/src/chat/transcript/messages.ts#L156-L171);
serialiser
[`index.ts:26-42`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/lib/shared/src/chat/transcript/index.ts#L26-L42)):
`speaker` (`human`, `assistant`, `system`), `text`, `model`, `intent` (`search`, `chat`, `edit`,
`insert`, `agentic`), `manuallySelectedIntent`, `contextFiles` (context items with `type`, `uri`,
`content`, ...), `editorState`, `error` (`ChatError`: `name`, `message`, rate-limit fields),
`search` (`query`, `response`), `agent`, `processes`, `subMessages`, `content`.

Agentic tool use appears two ways:

- `processes[]` (`ProcessingStep`,
  [`messages.ts:92-140`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/lib/shared/src/chat/transcript/messages.ts#L92-L140)):
  `type` (`tool`, `confirmation`, `step`), `id`, `title`, `description`, `content`, `state`
  (`pending`/`success`/`error`), `items`.
- `content[]` (`MessagePart`,
  [`lib/shared/src/sourcegraph-api/completions/types.ts:63-97`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/lib/shared/src/sourcegraph-api/completions/types.ts#L63-L97)):
  `{type:"text",text}`, `{type:"context_file",uri,content?}`,
  `{type:"tool_call",tool_call:{id,name,arguments}}` (assistant),
  `{type:"tool_result",tool_result:{id,content}}` (human).

No per-message timestamp. `model` is on the assistant message.

#### Permission records

The two agentic modes record permission state differently:

- **Deep Cody confirmations.** Deep Cody's terminal tool asks once per command:
  `addConfirmationStep` adds a `processes[]` step of type `confirmation` with `state` `pending`,
  `title` the tool's title and `content` the command
  ([`vscode/src/chat/agentic/CodyTool.ts:166-181`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/chat/agentic/CodyTool.ts#L166-L181);
  [`ProcessManager.ts:14-27`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/chat/agentic/ProcessManager.ts#L14-L27),
  [`46-49`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/chat/agentic/ProcessManager.ts#L46-L49)),
  and the webview's buttons answer it
  ([`vscode/webviews/chat/cells/agenticCell/ApprovalCell.tsx:14-16`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/webviews/chat/cells/agenticCell/ApprovalCell.tsx#L14-L16)).
  A refusal completes the step with `new Error('Command rejected')`, so the saved step has `state`
  `error` and `error` `{name, message: "Command rejected"}`
  ([`ProcessManager.ts:51-61`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/chat/agentic/ProcessManager.ts#L51-L61);
  [`messages.ts:253-260`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/lib/shared/src/chat/transcript/messages.ts#L253-L260)).
  An approval changes nothing on the confirmation step, which keeps `pending`: the only completion
  calls name the tool's or the main step's id
  ([`DeepCody.ts:176`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/chat/agentic/DeepCody.ts#L176),
  [`253`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/chat/agentic/DeepCody.ts#L253),
  [`274`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/chat/agentic/DeepCody.ts#L274)).
  A `pending` step in a saved chat is therefore an approval or a request never answered; source
  cannot tell them apart. The steps are saved on the **human** message: `postStatuses` calls
  `setLastMessageProcesses`, which refuses any message but a human one
  ([`ChatController.ts:943-946`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/chat/chat-view/ChatController.ts#L943-L946);
  [`ChatBuilder.ts:238-247`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/chat/chat-view/ChatBuilder.ts#L238-L247)).
  The command then runs only if no `shell.allow` prefix list excludes it and its first word is not
  on the block list
  ([`vscode/src/commands/context/shell.ts:28`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/commands/context/shell.ts#L28),
  [`40-54`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/commands/context/shell.ts#L40-L54)).
- **Agentic chat has no per-call decision.** The `agentic` intent's tools are `get_file`,
  `run_terminal_command`, `get_diagnostic` and `text_editor`, all enabled by default
  ([`vscode/src/chat/chat-view/tools/index.ts:22-27`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/chat/chat-view/tools/index.ts#L22-L27)).
  `run_terminal_command` runs the command in the first workspace folder with no prompt
  ([`tools/shell.ts:42-69`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/chat/chat-view/tools/shell.ts#L42-L69));
  its schema's `danger` flag says "user will be asked to confirm", but no code reads it
  ([`tools/schema.ts:21-25`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/chat/chat-view/tools/schema.ts#L21-L25);
  grepped `.danger` and `danger:` across `vscode`, `lib` and `agent`). The MCP server schema's
  `autoApprove` list is likewise parsed and never used
  ([`tools/MCPManager.ts:23-29`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/chat/chat-view/tools/MCPManager.ts#L23-L29);
  grepped `autoApprove` across the repository). Tool results go into the next human message's
  `content[]` and `contextFiles[]`, the latter `tool-state` items with a `status`
  ([`AgenticHandler.ts:154-162`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/chat/chat-view/handlers/AgenticHandler.ts#L154-L162);
  [`lib/shared/src/codebase-context/messages.ts:271-296`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/lib/shared/src/codebase-context/messages.ts#L271-L296)).
- **Standing settings**, all in the editor's user `settings.json` (or, for the JetBrains plugin,
  `<project>/.idea/.sourcegraph/cody_settings.json` with the `jsonConfig` string of the IDE's
  `cody.xml` as fallback; see [JetBrains IDE settings](#jetbrains-ide-settings)):
  `cody.agentic.context.experimentalOptions` `shell.allow` (command prefixes, empty allows all) and
  `shell.block` (added to a built-in block list), `cody.agentic.context.experimentalShell`
  (deprecated, "Now configurable via UI"), `cody.chat.agenticContext` (default `true`), and
  `cody.mcpServers.<server>` with `disabled` and `disabledTools`, which the extension itself
  rewrites at the global target when the user toggles a tool
  ([`vscode/package.json:1136-1140`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/package.json#L1136-L1140),
  [`1176-1213`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/package.json#L1176-L1213);
  [`MCPManager.ts:330-370`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/chat/chat-view/tools/MCPManager.ts#L330-L370),
  [`592-599`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/chat/chat-view/tools/MCPManager.ts#L592-L599)).
  The UI toggle that replaced `experimentalShell` is not persisted: Deep Cody's shell configuration
  is a static object rebuilt from the instance's feature flag and the client's shell
  ([`DeepCodyHandler.ts:31-35`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/chat/chat-view/handlers/DeepCodyHandler.ts#L31-L35),
  [`169-172`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/chat/chat-view/handlers/DeepCodyHandler.ts#L169-L172)).

### Joins and timestamps

- Account: the outer key; it names the server and the user that owned
  the chat, which is attribution evidence in its own right.
- Session to project: none recorded; infer from `contextFiles[].uri`
  paths.
- Tool call to result: `tool_call.id` = `tool_result.id`.
- No subagents.
- Legacy JetBrains history to the agent store: the migration made the chat's `updatedAt` the chat
  id and `lastInteractionTimestamp`, and `<server url>-<account name>` of the account whose `id`
  is the XML's `accountId` the account key
  ([`ChatHistoryMigration.kt:30-48`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/jetbrains/src/main/kotlin/com/sourcegraph/cody/config/migration/ChatHistoryMigration.kt#L30-L48)).
  An id that is a date passes the `CODY3538` migration unchanged, since only a UUID is rewritten
  ([`chat-id-migration-CODY3538.ts:12-17`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/agent/src/global-state/migrations/chat-id-migration-CODY3538.ts#L12-L17)),
  so a migrated chat keeps the zone-less `updatedAt` string as its id in both stores. Whether that
  time was the host's local time is not in the snapshot, which no longer has the code that set it.

### Other stores

VS Code: read `ItemTable` from `state.vscdb` through `doubleagent/sqlite_util.py` (WAL sidecars).
The value is UTF-8 JSON text in a BLOB column. History over 50 MB triggers a size warning
([`LocalStorageProvider.ts:40`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/services/LocalStorageProvider.ts#L40);
[`ChatHistoryManager.ts:104`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/chat/chat-view/ChatHistoryManager.ts#L104)),
so expect large values.

Other sources of past use and model state follow. No other mirror, index, backup or archive of
chats exists: the `symf` index holds code, not chats, and export writes only to the path the user
picks ([Sync, relay and origin rows](#sync-relay-and-origin-rows)).

#### Legacy JetBrains history

Secondary. Before the plugin ran the agent, it kept chats in a project-level persistent component,
`HistoryService`, stored as `cody_history.xml`
([`jetbrains/.../history/HistoryService.kt:13-15`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/jetbrains/src/main/kotlin/com/sourcegraph/cody/history/HistoryService.kt#L13-L15)).
The IntelliJ Platform keeps a project-level component's file in the project's `.idea/` directory, so
the path is `<project>/.idea/cody_history.xml` (platform behaviour, not re-read from platform source
here). Each `<chat>` has `internalId`, `updatedAt`, `messages` of `<message>` with `text` and
`speaker` (`HUMAN`, `ASSISTANT`), the deprecated `model` and `accountId`, an `llm` element (`model`,
`title`, `provider`, `tags`, `usage`) and `enhancedContext`
([`history/state/ChatState.kt:13-31`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/jetbrains/src/main/kotlin/com/sourcegraph/cody/history/state/ChatState.kt#L13-L31),
[`MessageState.kt:10-20`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/jetbrains/src/main/kotlin/com/sourcegraph/cody/history/state/MessageState.kt#L10-L20),
[`LLMState.kt:10-18`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/jetbrains/src/main/kotlin/com/sourcegraph/cody/history/state/LLMState.kt#L10-L18)).
The chats sit under `accountData` (`AccountData`, with its own `accountId`), or under the
deprecated top-level `chats` with the chat's own `accountId`
([`HistoryState.kt:6-21`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/jetbrains/src/main/kotlin/com/sourcegraph/cody/history/state/HistoryState.kt#L6-L21),
[`AccountData.kt:6-19`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/jetbrains/src/main/kotlin/com/sourcegraph/cody/history/state/AccountData.kt#L6-L19)).
The plugin's own serialisation test fixes the element form: each scalar is an element named for
the field with a `value` attribute, and each list is the field's element around `<list>`, as in
`<accountData><list><AccountData><accountId value="..."/><chats><list><chat><internalId
value="..."/><messages><list><message><speaker value="HUMAN"/><text value="hi"/></message>...`
([`jetbrains/src/test/.../HistoryStateTest.kt:37-67`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/jetbrains/src/test/kotlin/com/sourcegraph/cody/history/state/HistoryStateTest.kt#L37-L67)),
with `updatedAt` a date and time without a zone (`2024-01-31T01:06:18.524621` in the test). The
test does not show an `llm` element, and the `<project version="4"><component
name="ChatHistory">` wrapper the IDE writes around the state in the file is platform behaviour,
not in this repository. The run-once migration `CodyMigrateChatHistory-v2`
([`config/migration/SettingsMigration.kt:69-71`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/jetbrains/src/main/kotlin/com/sourcegraph/cody/config/migration/SettingsMigration.kt#L69-L71))
copies it into the agent store through `chat/import` with merge on, keyed `<server url>-<account
name>` with `updatedAt` as both the chat id and `lastInteractionTimestamp`, the chat's `llm.model`
on every message, and only complete human and assistant pairs
([`ChatHistoryMigration.kt:17-79`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/jetbrains/src/main/kotlin/com/sourcegraph/cody/config/migration/ChatHistoryMigration.kt#L17-L79)).
Nothing deletes the XML, so it survives a delete or clear of the migrated chat in the agent store,
and it is the only record of a dropped unpaired message. It names the project by where it sits. It
joins the primary store on account key, chat id (`updatedAt`) and message text.

#### cody-model-preferences

A model setting: a global-state key (`sourcegraph.cody-ai` row in VS Code, a file of that name under
`JetBrains-globalState/` for the plugin) holding, per endpoint, `defaults` and `selected`, each a
map from usage (`chat`, `edit`, `autocomplete`, `unlimitedChat`) to model id
([`LocalStorageProvider.ts:37`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/services/LocalStorageProvider.ts#L37),
[`338-344`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/services/LocalStorageProvider.ts#L338-L344);
[`lib/shared/src/models/modelsService.ts:198-219`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/lib/shared/src/models/modelsService.ts#L198-L219),
[`545`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/lib/shared/src/models/modelsService.ts#L545);
[`lib/shared/src/models/types.ts:3-8`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/lib/shared/src/models/types.ts#L3-L8)).
It records the user's current choice, not when it was made. Deep Cody's helper model is chosen at
run time and recorded nowhere
([`DeepCodyHandler.ts:162-167`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/chat/chat-view/handlers/DeepCodyHandler.ts#L162-L167)).

#### cody-chat-memory

Gone: listed as a deprecated key that `clearDeprecatedKeys` sets to null whenever the storage is
attached at activation
([`LocalStorageProvider.ts:42-46`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/services/LocalStorageProvider.ts#L42-L46),
[`63-72`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/services/LocalStorageProvider.ts#L63-L72),
[`374-384`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/services/LocalStorageProvider.ts#L374-L384)).
Its earlier content is not in the snapshot (no other reference found for `cody-chat-memory` or
`CODY_CHAT_MEMORY`); a `state.vscdb` from a build that predates the cleanup may still hold it.

### Format versions

The key suffix `-v2` is the version
([`LocalStorageProvider.ts:28-29`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/services/LocalStorageProvider.ts#L28-L29)).
JetBrains histories imported from the pre-agent plugin had UUID chat ids; the agent migration
rewrites them to `toUTCString()` ids and records `migrated-chat-history-cody-3538`
([`agent/src/global-state/migrations/chat-id-migration-CODY3538.ts:7-41`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/agent/src/global-state/migrations/chat-id-migration-CODY3538.ts#L7-L41)).
Older VS Code keys (`cody-local-chatHistory` without `-v2`) were not traced.

No version drift is visible in source, since the snapshot is HEAD: the parser's keys
(`sourcegraph.cody-ai`, `cody-local-chatHistory-v2`, the message and process fields) are at the
cited lines. Whether builds after August 2025 still write them cannot be read from source, and
they need a real-install round before any parser change.

### Secrets

The access token is not in the history; it is in SecretStorage. Do not treat
`SOURCEGRAPH_CODY_ENDPOINT` as a secret: it is a URL, and the extension refuses to save a token
there
([`LocalStorageProvider.ts:92-100`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/services/LocalStorageProvider.ts#L92-L100)).
`contextFiles[].content` and `tool_result.content` carry file and command output. In the
editor's `User/settings.json`, redact the `headers` and `env` values of each
`cody.mcpServers.<server>` entry.

### Parser plan

Inputs: the `sourcegraph.cody-ai` row of any collected `state.vscdb`
(attribute it to `cody` even though the file is under `vscode`), and
`*/Cody-nodejs/**/JetBrains-globalState/cody-local-chatHistory-v2`; for the settings, model
and secondary rows below, the editor's `User/settings.json`,
`*/Cody-nodejs/**/JetBrains-globalState/cody-model-preferences` and a project's
`.idea/cody_history.xml`.

| Column | Source |
| --- | --- |
| timestamp_utc | chat `id` parsed as RFC 1123 date, for every row of the chat |
| session_id | chat `id` (prefix with the account key if ids collide across accounts) |
| project_path | empty, or the common parent of `contextFiles[].uri.fsPath` |
| git_branch | empty |
| turn_type | `human` → user; `assistant` → assistant; `system` → system; `tool_call` part → tool_use; `tool_result` part → tool_result; `processes[]` with `type:"tool"` → tool_use |
| model | `assistantMessage.model` |
| tool_name | `tool_call.name` or process `title` |
| tool_use_id | `tool_call.id` / `tool_result.id` / process `id` |
| text | `text`; for tool_use the `arguments`; for tool_result its `content` |
| source_line | 1 (single JSON value) |

#### Permission rows

All rows are `system` ([issue 96](https://github.com/seanthegeek/doubleagent/issues/96) and
[issue 97](https://github.com/seanthegeek/doubleagent/issues/97)):

- `permission decision: <decision> confirmation state=<state> error=<error.message> title=<title>`
  for a `processes[]` entry of `type` `confirmation`, which the source describes as "a process that
  prompts the user confirmation"
  ([`messages.ts:76-79`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/lib/shared/src/chat/transcript/messages.ts#L76-L79)),
  with its `description` and `content` on the following lines and `tool_use_id` its `id`. `state`
  and `error.message` are printed as recorded (`pending`, `success`, `error`,
  [`messages.ts:117-124`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/lib/shared/src/chat/transcript/messages.ts#L117-L124)):
  and `<decision>` follows the states traced in
  [Permission records](#permission-records): `rejected` for `state` `error` with `error.message` `Command
  rejected`, and `approved or unanswered` for `pending`, which source cannot split. Any other state
  is written `permission decision: confirmation state=<state>`, with `error=` when the step has an
  `error.message`, and the row does not interpret it
  ([issue 169](https://github.com/seanthegeek/doubleagent/issues/169)).

The agentic chat records no per-call decision, so it yields no row.

The standing settings in [Permission records](#permission-records) are read from the editor's
`User/settings.json`, which the catalog attributes to the editor's agent (the parser lists the
editors in `reads_agents`), as JSONC (JSON with comments and trailing commas, the form VS Code
accepts): one `permission setting: <key>=<value> (from <file>, file mtime)` per leaf of
`cody.agentic.context.experimentalOptions` (`...experimentalOptions.shell.allow=["git","ls"]`),
for `cody.agentic.context.experimentalShell` and `cody.chat.agenticContext`, and for `disabled`
and `disabledTools` of each `cody.mcpServers.<server>`, at the file's modification time with an
empty `session_id`. A server's `headers`, `env`, `url`, `command` and `args` are not read, since
they carry its credentials ([Credentials](#credentials)). A `settings.json` that does not decode
yields `parser: settings.json is not a JSON object` only when its text names a `cody.` key. These
rows are source-derived; not observed on any real install. The JetBrains copies of the same keys
(`cody_settings.json` and the `jsonConfig` of `cody.xml`) are collected but not parsed: they are
HOCON ([JetBrains IDE settings](#jetbrains-ide-settings)), which the analyzer has no reader for.

#### Model rows

All rows are `system` ([issue 96](https://github.com/seanthegeek/doubleagent/issues/96) and
[issue 97](https://github.com/seanthegeek/doubleagent/issues/97)):

- `model: <id>` at the first assistant message of a chat whose `model`
  is set and `model: <old> -> <new>` on every later assistant message
  with a different `model`, before that message's rows, at the chat's
  time. The model is recorded per assistant message, so a change inside
  a chat shows.

`cody-model-preferences` ([cody-model-preferences](#cody-model-preferences)), from the
`sourcegraph.cody-ai` row and from the JetBrains file of that name, gives one `model setting:
<selected|defaults>.<usage>=<id> endpoint=<endpoint> (from <file>, file mtime)` per entry, the
user's `selected` before the server's `defaults`, with ` cody-model-preferences` after the file
name for the database row, the `model` column the id, an empty `session_id` and the file's
modification time, since the value records no time. The legacy XML's chat model (the `model`
inside its `llm` element, else the deprecated `model`) gives `model: <id>` before the chat's
first assistant message, the row the agent store's copy of the chat also yields. These rows are
source-derived; not observed on any real install.

#### Past-use rows and secondary sources

`<project>/.idea/cody_history.xml` ([Legacy JetBrains history](#legacy-jetbrains-history)) is a
secondary source, collected through the project table and read on every run with `secondary()`
true. Per chat it yields `chat: <internalId> | account=<accountId> | messages=<n> (legacy
JetBrains history)`, `model: <id>` before the first assistant message, and a `user` or
`assistant` row per `HUMAN` or `ASSISTANT` message with text, all with the chat's `updatedAt` as
`session_id` (else `internalId`) and as the time, read as UTC like the same string in the agent
store, and the project the file sits in as `project_path`. A migrated chat's complete pairs
match the agent store's rows on session, text and time and are dropped as copies; an unpaired
message the migration skipped, and a chat the agent store no longer holds, are kept. The account
is the XML's account id, which names the agent store's account key only through
`cody_accounts.xml` ([JetBrains IDE settings](#jetbrains-ide-settings)), which the parser does
not join. These rows are source-derived; not observed on any real install. No other source
records past use ([Other stores](#other-stores)).

#### Sync, relay and origin rows

Cody records no sync state. The frozen snapshot has no chat history sync, no share link and no
server copy of a chat: history is kept only in the local `cody-local-chatHistory-v2` value and
deleted there
([`LocalStorageProvider.ts:190-240`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/services/LocalStorageProvider.ts#L190-L240),
[`270-272`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/services/LocalStorageProvider.ts#L270-L272)),
the history commands are clear, delete, rename and export
([`ChatsController.ts:227-233`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/chat/chat-view/ChatsController.ts#L227-L233)),
and export writes a JSON file to a path the user picks in a save dialog
([`ChatsController.ts:401-420`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/chat/chat-view/ChatsController.ts#L401-L420);
the webview's download button,
[`vscode/webviews/chat/downloadChatHistory.ts:11-30`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/webviews/chat/downloadChatHistory.ts#L11-L30)).
The GraphQL queries the client sends name no chat or history operation
([`lib/shared/src/sourcegraph-api/graphql/queries.ts`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/lib/shared/src/sourcegraph-api/graphql/queries.ts)),
and Cody Web, the chat embedded in the Sourcegraph web app, keeps its history in the browser's
IndexedDB rather than on the server
([`web/lib/agent/agent.worker.ts:25`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/web/lib/agent/agent.worker.ts#L25),
[`web/lib/agent/index-db-storage.ts:9`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/web/lib/agent/index-db-storage.ts#L9)).
Searched `share`, `sync`, `upload`, `export`, `cloud`, `remote`, `teleport`, `remoteId` and
`chatHistory` in `vscode/src`, `vscode/webviews`, `lib/shared/src`, `agent/src`, `web/lib` and the
JetBrains plugin's `history/` package. Prompt and reply text does leave the host outside the model
call, with no local marker: the telemetry events `cody.chat-question/submitted` and
`cody.chat-question/executed` carry the prompt (truncated to the 7,000-token chat input budget,
[`lib/shared/src/token/constants.ts:4`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/lib/shared/src/token/constants.ts#L4))
and the chat id (`ChatBuilder.sessionID`, the same `toUTCString()` value,
[`ChatBuilder.ts:97`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/chat/chat-view/ChatBuilder.ts#L97))
as `sessionID` in their `privateMetadata`, the event part that sourcegraph.com exports and private
instances drop unless allowlisted
([`vscode/src/services/telemetry-v2.ts:118-121`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/services/telemetry-v2.ts#L118-L121)),
and `cody.chatResponse` carries the reply, but only when the account's endpoint is sourcegraph.com
(`isDotCom`) or, for the executed event, Sourcegraph's internal instance sourcegraph.sourcegraph.com
(`isS2`)
([`lib/shared/src/telemetry-v2/events/chat-question.ts:45-75`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/lib/shared/src/telemetry-v2/events/chat-question.ts#L45-L75),
[`107-148`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/lib/shared/src/telemetry-v2/events/chat-question.ts#L107-L148);
[`ChatController.ts:1570-1586`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/chat/chat-view/ChatController.ts#L1570-L1586);
[`lib/shared/src/sourcegraph-api/environments.ts:7-35`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/lib/shared/src/sourcegraph-api/environments.ts#L7-L35)).
The events go to the connected instance's GraphQL API
([`GraphQLTelemetryExporter.ts:33-45`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/lib/shared/src/sourcegraph-api/telemetry/GraphQLTelemetryExporter.ts#L33-L45)),
on by default; the hidden setting `cody.telemetry.level` (`all` or `off`, default `all`) is read
([`vscode/src/configuration.ts:95`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/configuration.ts#L95)),
but this document does not trace whether `off` stops these events. A responder with a
sourcegraph.com account in the history key can ask Sourcegraph for the events by the chat id;
nothing local says they were sent, so the parser yields no sync row.

Also leaving the host without a local mark: `cody.runTerminalCommand`
`accepted` and `cody.tool-use` `executed`/`failed` telemetry carry each agentic tool call's
arguments in `privateMetadata`
([`tools/shell.ts:58-66`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/chat/chat-view/tools/shell.ts#L58-L66);
[`AgenticHandler.ts:410-444`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/chat/chat-view/handlers/AgenticHandler.ts#L410-L444)).

#### Scratch directory rows

None: Cody creates no scratch directory ([Scratch directories](#scratch-directories)).

#### Deletion rows

No row: no tombstone or deletion record is written, so the parser has nothing
to yield for a delete; the legacy JetBrains XML is the one local copy
that a delete does not reach ([What survives a delete](#what-survives-a-delete)).

#### Bad-line rows

Each input is one JSON value; the parser recovers a damaged history chat by chat, with one `parser:`
row for what it cannot read and one `parser: unparseable record <N> (<reason>): <record>` row,
carrying the chat's session id, for the first interaction of a chat that does not decode. The legacy
XML is read incrementally, so the chats before a cut or damaged part are yielded, followed by one
`parser: cody_history.xml did not parse; read <n> chat(s) (<error>)` row with the project as
`project_path`. A model preferences file that is not a JSON object is one `parser:
cody-model-preferences is not a JSON object` row.

#### Fixtures

Fixture (`JetBrains-globalState/cody-local-chatHistory-v2`):

```json
{"https://sourcegraph.com/-alice":{"chat":{"Fri, 03 Oct 2026 10:00:00 GMT":{"id":"Fri, 03 Oct 2026 10:00:00 GMT","chatTitle":"Fix flaky test","lastInteractionTimestamp":"Fri, 03 Oct 2026 10:00:00 GMT","interactions":[{"humanMessage":{"speaker":"human","text":"why is test_login flaky?","intent":"chat","contextFiles":[{"type":"file","uri":{"$mid":1,"fsPath":"/home/alice/proj/tests/test_login.py","path":"/home/alice/proj/tests/test_login.py","scheme":"file"},"source":"user"}]},"assistantMessage":{"speaker":"assistant","model":"anthropic::2024-10-22::claude-sonnet-4-latest","text":"It depends on wall-clock time.","intent":"chat"}}]}}}}
```

Agentic variant of an interaction:

```json
{"humanMessage":{"speaker":"human","text":"run the tests","intent":"agentic"},"assistantMessage":{"speaker":"assistant","model":"anthropic::2024-10-22::claude-sonnet-4-latest","text":"Running tests.","content":[{"type":"text","text":"Running tests."},{"type":"tool_call","tool_call":{"id":"toolu_01","name":"run_terminal_command","arguments":"{\"command\":\"pytest -q\"}"}}],"processes":[{"type":"tool","id":"toolu_01","title":"run_terminal_command","content":"pytest -q","state":"success"}]}}
```

Editor `settings.json` keys (from the `package.json` and MCP server schemas cited in
[Permission records](#permission-records) and
[`MCPManager.ts:20-45`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/chat/chat-view/tools/MCPManager.ts#L20-L45)):

```json
{"cody.chat.agenticContext":false,"cody.agentic.context.experimentalOptions":{"shell":{"allow":["git","ls"],"block":["sudo"]}},"cody.mcpServers":{"github":{"command":"npx","env":{"GITHUB_TOKEN":"..."},"disabledTools":["delete_repo"]},"web":{"url":"https://mcp.example/sse","disabled":true}}}
```

`cody-model-preferences`:

```json
{"https://sourcegraph.com/":{"defaults":{"chat":"anthropic::2024-10-22::claude-sonnet-4-latest"},"selected":{"chat":"anthropic::2024-10-22::claude-3-5-haiku-latest"}}}
```

Legacy `.idea/cody_history.xml`, the test's serialisation inside the component wrapper, with an
unpaired last message the migration drops:

```xml
<project version="4">
  <component name="ChatHistory">
    <accountData><list><AccountData>
      <accountId value="VXNlkjoxEFU3NjE=" />
      <chats><list><chat>
        <internalId value="0f8b7034-9fa8-488a-a13e-09c52677008a" />
        <messages><list>
          <message><speaker value="HUMAN" /><text value="hi" /></message>
          <message><speaker value="ASSISTANT" /><text value="hello" /></message>
          <message><speaker value="HUMAN" /><text value="unpaired question" /></message>
        </list></messages>
        <updatedAt value="2024-01-31T01:06:18.524621" />
      </chat></list></chats>
    </AccountData></list></accountData>
  </component>
</project>
```

#### Source host rows

The agent records no source-host integration. GitHub and GitLab appear only as ways to sign in
to sourcegraph.com: the extension opens the instance's OpenID Connect login with
`prompt_auth=github` or `gitlab`, which ends in a Sourcegraph access token
([`vscode/src/services/AuthProviderSimplified.ts:24-37`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/services/AuthProviderSimplified.ts#L24-L37)),
stored as in [Credentials](#credentials); the GitHub or GitLab account is server-side only. The
global-state key `cody-github-repo-metadata` caches, for a day, the github.com names of the
workspace's git remotes with an `isPublic` flag and a fetch time in milliseconds, from an
unauthenticated `HEAD` request to the GitHub API
([`vscode/src/repository/githubRepoMetadata.ts:16-22`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/repository/githubRepoMetadata.ts#L16-L22),
[`85-98`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/repository/githubRepoMetadata.ts#L85-L98);
[`LocalStorageProvider.ts:257-268`](https://github.com/sourcegraph/cody-public-snapshot/blob/8e20ac6c1460c08b0db581c0204658112a246eda/vscode/src/services/LocalStorageProvider.ts#L257-L268)):
the user's own remotes, with no account or token, so not an integration, and not read. Grepped
`github`, `gitlab`, `bitbucket`, `oauth`, `installation`, `repositor`, `pull request` and `clone`
in `vscode/src`, `lib/shared/src`, `agent/src` and the JetBrains plugin at
[8e20ac6](https://github.com/sourcegraph/cody-public-snapshot/commit/8e20ac6c1460c08b0db581c0204658112a246eda).

## Confidence

- High (frozen source): the storage keys and row layout, the key names, transcript and message
  fields, `toUTCString` ids and tool part shapes, the `symf` paths, the custom command files, the
  agent's `Cody-nodejs` layout, the JetBrains global-state directory and the history file in it,
  and the CLI's keychain use.
- High (frozen source): the legacy history's element form inside the state (the plugin's own
  serialisation test) and the JetBrains account files' fields.
- Medium: the serialised form of `uri` (a VS Code URI's JSON form; field set not traced) and the
  model string in the fixture; the legacy history's file path and its component wrapper, and the
  nesting of its `llm` element, which are IntelliJ Platform behaviour not read here (the parser
  takes the first `model` value anywhere under `llm`).
- Not determined: anything after August 2025, including whether shipped builds still use these
  keys, paths and fields and whether the JetBrains plugin passes a `globalStateDir` from the Kotlin
  side (it does not at this commit); the size of a `symf` index; where the Eclipse and Visual
  Studio clients, which are not in this repository, keep state; whether `cody.telemetry.level`
  `off` stops the chat telemetry events; whether the legacy history's zone-less `updatedAt` is
  the host's local time.
