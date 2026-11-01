# twinny

Twinny (catalog name `twinny`) is an open-source AI coding extension for VS Code (extension id
`rjmacarthy.twinny`); its chats live inside the editor's own SQLite state database, not in a file
of its own. Its repository also ships the `twinny-node` peer-to-peer GPU sharing CLI and the
`twinny-server` team gateway. The `twinny` parser reads the chats from the editor's state
database.

## Source and evidence

[twinnydotdev/twinny](https://github.com/twinnydotdev/twinny) at
[`9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6`](https://github.com/twinnydotdev/twinny/commit/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6),
version 4.3.5, MIT
([package.json:32](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/package.json#L32)).
Open source, TypeScript. All claims from source. Citations name this commit unless they say
otherwise; facts new since 4.3.5 cite
[`0337b26`](https://github.com/twinnydotdev/twinny/commit/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a)
(version 4.3.15,
[package.json:5](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/package.json#L5)).
How VS Code stores an extension's `globalState` was checked against
[microsoft/vscode](https://github.com/microsoft/vscode) at the commit used by [vscode.md](vscode.md),
and VS Code's extension storage and log-channel paths at
[`4d22f4f`](https://github.com/microsoft/vscode/commit/4d22f4f922eb012217ac65672ccce02f59d63706).
The repository holds three programs: the VS Code extension, the `twinny-node`
peer-to-peer GPU sharing CLI
([package.json:61-63](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/package.json#L61-L63))
and the `twinny-server` team gateway
([packages/twinny-server/package.json:28-30](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/packages/twinny-server/package.json#L28-L30)).

### Observed

- No real install.

### Source reviews

- 2026-10-21: twinnydotdev/twinny at
  [0337b26](https://github.com/twinnydotdev/twinny/commit/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a)
  (4.3.15), with microsoft/vscode at
  [4d22f4f](https://github.com/microsoft/vscode/commit/4d22f4f922eb012217ac65672ccce02f59d63706)
  for the log file format: the parser plan now reads the standing grants, the team policy, the
  reply's `changes`, the `agent-changes/` records, the log channel and the review tab's repository;
  it corrected the team policy, which is kept in `globalState`, not in memory only, and added the
  log's line format and rotation and the source-host finding.
- 2026-10-19: [twinnydotdev/twinny](https://github.com/twinnydotdev/twinny) at
  [`0337b26`](https://github.com/twinnydotdev/twinny/commit/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a)
  (version 4.3.15, committed 2026-10-16), against
  [`9339bd1`](https://github.com/twinnydotdev/twinny/commit/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6)
  (4.3.5). Between the two commits 160 files changed; every path in this document and every key
  and field the parser reads are still used. It added the records new since 4.3.5 (the per-reply
  file-change records, the log channel, the standing grants, the ChatGPT plan keys and the message
  fields of [Record schema](#record-schema)) and the `user` row for a step typed mid-reply.
- 2026-10-17: [twinnydotdev/twinny](https://github.com/twinnydotdev/twinny) at
  [`0337b26`](https://github.com/twinnydotdev/twinny/commit/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a),
  for sync. It added the finding of [Sync, relay and origin rows](#sync-relay-and-origin-rows)
  that Twinny records no sync state.

## Collection

### Per-user storage

Extension id `rjmacarthy.twinny` (publisher `rjmacarthy`, name `twinny`,
[package.json:2](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/package.json#L2),
[33](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/package.json#L33);
[src/protocol/types.ts:118](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/protocol/types.ts#L118)).
The editor's `User/globalStorage/rjmacarthy.twinny/` directory is already collected under `vscode`;
what Twinny actually keeps is split three ways.

Layout is the same on Linux, macOS and Windows; only the editor's `User` directory moves (see
[vscode.md](vscode.md)). No environment overrides for `~/.twinny` in the extension; the server's
paths can be changed in its config (`auth.keysFile`, `auth.licenseFile`, `usage.dir`,
`recording.dir`,
[config.ts:312-330](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/gateway/config.ts#L312-L330)).

#### Scratch directories

Twinny has none. Agent mode works in the first workspace folder only
([src/extension/chat/index.ts:494-497](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/extension/chat/index.ts#L494-L497)),
and its one temporary file, a copy of the repository's index named `twinny-index-<uuid>` in the
system temporary directory, is deleted as soon as the tree is snapshotted
([src/extension/chat/command-snapshot.ts:62-80](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/extension/chat/command-snapshot.ts#L62-L80));
nothing for the catalog to collect or for discovery to drop.

#### VS Code `globalState`

Not files: chat history, the provider list and the active providers are
`context.globalState` keys
([src/common/constants/storage.ts:1-21](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/common/constants/storage.ts#L1-L21)):
`twinny.conversations` (every conversation, keyed by id), `twinny.active-conversation`,
`twinny.inference-providers`, `twinny.active-chat-provider`, `-fim-provider`,
`-embeddings-provider`, P2P device and trusted-peer lists, team share state
([src/extension/chat/conversation-history.ts:80-104](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/extension/chat/conversation-history.ts#L80-L104);
[src/extension/providers/store.ts:47-64](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/extension/providers/store.ts#L47-L64)).
VS Code persists an extension's global state as one JSON string in the profile `state.vscdb`
`ItemTable`, keyed by the extension id
([extHostMemento.ts:118](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/workbench/api/common/extHostMemento.ts#L118);
[extensionStorage.ts:159-176](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/platform/extensionManagement/common/extensionStorage.ts#L159-L176)).
So Twinny's chats live in `User/globalStorage/state.vscdb`, row `key = 'rjmacarthy.twinny'`, which
the `vscode` entry collects. Per-workspace UI state (`chatMessage` draft, `contextItems`,
`selection`, `embeddingsUpdatedAt`,
[storage.ts:35-47](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/common/constants/storage.ts#L35-L47);
[src/extension/webview/base.ts:301-322](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/extension/webview/base.ts#L301-L322))
goes to `workspaceStorage/<id>/state.vscdb` under the same key.

#### Extension global storage

`globalStorage/rjmacarthy.twinny/` holds only:

- `twinny-providers.json`, the provider list including `apiKey`, written only when the setting
  `twinny.providerStorageLocation` is `file` (default `globalState`)
  ([store.ts:37-41](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/extension/providers/store.ts#L37-L41),
  [141-171](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/extension/providers/store.ts#L141-L171);
  [src/common/constants/misc.ts:29](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/common/constants/misc.ts#L29);
  [package.json:699-708](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/package.json#L699-L708)).
- `p2p-host.lock` and `team-share.lock`, `{pid, since}` window locks
  ([src/extension/p2p/host.ts:40](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/extension/p2p/host.ts#L40),
  [83-88](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/extension/p2p/host.ts#L83-L88);
  [src/extension/team/share.ts:28](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/extension/team/share.ts#L28),
  [86-90](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/extension/team/share.ts#L86-L90);
  [src/extension/utils/window-lock.ts:26-52](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/extension/utils/window-lock.ts#L26-L52)).

No chat, embedding or symmetry data is written there, except
`agent-changes/` when no folder is open ([Since 4.3.5](#since-435)).

#### Since 4.3.5

Since 4.3.5, at
[`0337b26`](https://github.com/twinnydotdev/twinny/commit/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a):

- `workspaceStorage/<id>/rjmacarthy.twinny/agent-changes/<uuid>.json`, one record per agent-mode
  reply that changed files, with the text of each file (up to 1 MiB) before the reply
  ([src/extension/chat/index.ts:210-221](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/extension/chat/index.ts#L210-L221);
  [src/extension/chat/reply-changes.ts:25-47](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/extension/chat/reply-changes.ts#L25-L47),
  [211-259](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/extension/chat/reply-changes.ts#L211-L259)),
  or `globalStorage/rjmacarthy.twinny/agent-changes/` with no folder open; the record is in
  [Per-reply file-change records](#per-reply-file-change-records). Nested `twinny` catalog entries
  (`<User dir>/workspaceStorage/*/rjmacarthy.twinny` for each editor layout) claim the workspace
  copy from the editor's `User` directory; the `workspace.json` and `state.vscdb` beside it stay the
  editor's (`vscode` or the fork).
- Standing permission grants and the current `twinny.selectedModel`, as keys of the extension's
  `globalState` value (the `rjmacarthy.twinny` row of `globalStorage/state.vscdb`), and the
  `twinny.chatTools*` editor settings in the user `settings.json`
  ([package.json:645-720](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/package.json#L645-L720));
  the keys and values are in [Standing permission grants](#standing-permission-grants). Both files
  are collected as agent `vscode`.
- The ChatGPT plan sign-in's profile, `twinny.chatgpt-plan.profile` (`clientId`, `subject`, `email`,
  `name` of the signed-in OpenAI account) and `twinny.chatgpt-plan.host-id`, also in the
  `globalState` value
  ([src/extension/chatgpt-plan/session.ts:11-13](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/extension/chatgpt-plan/session.ts#L11-L13),
  [52-57](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/extension/chatgpt-plan/session.ts#L52-L57),
  [397-402](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/extension/chatgpt-plan/session.ts#L397-L402)).
- The **Twinny** log channel, `<editor
  logs>/<session>/window<n>/exthost/rjmacarthy.twinny/Twinny.log`, one line per chat request and
  the prompts and replies at Debug level, and the copies it is rotated to, `Twinny.log_1` to
  `Twinny.log_6` ([Twinny log channel](#twinny-log-channel)); collected as agent `vscode` under the
  editor's `logs` entries.
- The pull request review tab's owner and repository, in the extension's per-workspace state (the
  `rjmacarthy.twinny` row of `workspaceStorage/<id>/state.vscdb`); the keys are in
  [Pull request review repository](#pull-request-review-repository).

#### `~/.twinny`

`~/.twinny` from `os.homedir()` on every OS (no platform branching):

- `templates/*.hbs`: editable prompt templates, defaults copied in on first run
  ([src/index.ts:186](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/index.ts#L186);
  [src/extension/templates/provider.ts:18](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/extension/templates/provider.ts#L18),
  [68-80](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/extension/templates/provider.ts#L68-L80),
  [157](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/extension/templates/provider.ts#L157)).
- `embeddings/<workspace name>/`: a LanceDB table `chunks` plus `manifest.json` per workspace; the
  name is `vscode.workspace.name` with characters outside `[A-Za-z0-9_.-]` replaced by `_`
  ([src/extension/embeddings/index.ts:85-91](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/extension/embeddings/index.ts#L85-L91);
  [database.ts:9-11](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/extension/embeddings/database.ts#L9-L11),
  [102-104](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/extension/embeddings/database.ts#L102-L104);
  [src/extension/utils.ts:369-376](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/extension/utils.ts#L369-L376)).
- `node/identity.json` (P2P key seed, mode 0600) and `node/trusted-peers.json`, written by
  `twinny-node`
  ([src/node/config.ts:1-24](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/node/config.ts#L1-L24),
  [51-55](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/node/config.ts#L51-L55)).
- `server/`, the `twinny-server` data directory
  ([src/gateway/config.ts:209-222](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/gateway/config.ts#L209-L222)):
  `format.json`, `keys.json`, `license`, `invites.json`, `plugins.json`, `usage/`, `recordings/`,
  `audit/`, `plugins/`
  ([src/gateway/data-format.ts:18-21](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/gateway/data-format.ts#L18-L21)).
  `usage/YYYY-MM-DD.jsonl` per-request records
  ([usage.ts:41](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/gateway/usage.ts#L41),
  [83](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/gateway/usage.ts#L83));
  `audit/YYYY-MM.jsonl` admin actions
  ([audit.ts:3](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/gateway/audit.ts#L3),
  [101-102](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/gateway/audit.ts#L101-L102);
  [serve.ts:354](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/gateway/serve.ts#L354));
  `recordings/recordings.sqlite` or `recordings/jsonl/YYYY-MM-DD.jsonl`, full prompts and replies,
  off by default
  ([src/gateway/recording/store.ts:1-11](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/gateway/recording/store.ts#L1-L11),
  [553-556](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/gateway/recording/store.ts#L553-L556);
  [config.ts:216-222](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/gateway/config.ts#L216-L222));
  `plugins/<id>/` plugin state
  ([plugins/host.ts:492-494](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/gateway/plugins/host.ts#L492-L494)).
  The server's own configuration is `twinny.gateway.json` in the directory it was started from
  ([src/gateway/init.ts:19](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/gateway/init.ts#L19),
  [150](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/gateway/init.ts#L150)).

### Credentials

- Provider API keys, in plain text: in `state.vscdb` (`rjmacarthy.twinny` →
  `twinny.inference-providers`) or in `globalStorage/rjmacarthy.twinny/twinny-providers.json`; the
  field is in [Secrets](#secrets).
- Team gateway tokens: VS Code secret storage, `twinny.gateway-token.<providerId>`
  ([src/extension/providers/credentials.ts:14](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/extension/providers/credentials.ts#L14),
  [59-61](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/extension/providers/credentials.ts#L59-L61));
  P2P identity and host seeds also secret storage
  ([storage.ts:11-13](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/common/constants/storage.ts#L11-L13);
  [src/extension/p2p/runtime.ts:261-265](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/extension/p2p/runtime.ts#L261-L265);
  [host.ts:336-340](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/extension/p2p/host.ts#L336-L340)).
  These are encrypted `secret://` rows in `state.vscdb` (see [vscode.md](vscode.md)).
- `twinny.githubToken` setting, a GitHub PAT (personal access token), in the editor
  `settings.json`
  ([package.json:693-698](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/package.json#L693-L698)):
  the token the pull request review sends to GitHub ([Source host rows](#source-host-rows)). It
  shares the file with every other editor setting, so it cannot be a secret glob.
- `~/.twinny/node/identity.json`: the `twinny-node` key seed.
- ChatGPT plan sign-in (new since 4.3.5): access, refresh and id tokens in VS Code secret storage
  under `twinny.chatgpt-plan.credentials`, an encrypted `secret://` row; the account's email and
  subject in plain `globalState` ([Since 4.3.5](#since-435))
  ([src/extension/chatgpt-plan/session.ts:11-13](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/extension/chatgpt-plan/session.ts#L11-L13),
  [59-66](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/extension/chatgpt-plan/session.ts#L59-L66),
  [484](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/extension/chatgpt-plan/session.ts#L484)).
  The profile is redacted by key ([Secrets](#secrets)): `state.vscdb` also holds the chats, so it
  cannot be a secret glob.
- `twinny.chatToolsMcpServers` in the user `settings.json` can carry tokens in each server's `env`
  (the setting's own example is a `GITHUB_TOKEN`,
  [package.json:684-720](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/package.json#L684-L720)).
- Exported provider lists: the "Export providers" command writes the providers, `apiKey` included,
  to a file the user picks, named `twinny-providers.json` by default
  ([src/extension/providers/manager.ts:690-704](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/extension/providers/manager.ts#L690-L704));
  only the copy in `globalStorage/rjmacarthy.twinny/` is in a known place.
- `twinny-server`: `license` (a licence token), `keys.json` (SHA-256 of client keys, not the keys,
  [src/gateway/keys.ts:4-7](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/gateway/keys.ts#L4-L7)),
  `plugins/<id>/settings.json` with git clone tokens for the context plugin
  ([src/gateway/plugins/context.ts:59-60](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/gateway/plugins/context.ts#L59-L60),
  [214](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/gateway/plugins/context.ts#L214),
  [236-241](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/gateway/plugins/context.ts#L236-L241)),
  and `plugins/<forge>/repos.json` with the forge plugins' saved access tokens
  ([Source host rows](#source-host-rows)).
  Backend API keys are read from environment variables named in the config (`apiKeyEnv`,
  [config.ts:58](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/gateway/config.ts#L58),
  [237](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/gateway/config.ts#L237)).

### Exclusions

- `~/.twinny/embeddings/*/chunks.lance`: the LanceDB vector table of source
  chunks; size grows with the indexed workspace. Keep `manifest.json`.
- `~/.twinny/server/plugins/*/repos/*/checkout` (shallow git clones) and `.../index`
  (`manifest.json`, `chunks.json`, `vectors.bin`)
  ([context.ts:271-273](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/gateway/plugins/context.ts#L271-L273),
  [290-295](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/gateway/plugins/context.ts#L290-L295),
  [336-338](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/gateway/plugins/context.ts#L336-L338)).

### Project-local files

None written. Twinny reads each workspace's `.gitignore` to filter files
([src/extension/embeddings/indexer.ts:85-94](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/extension/embeddings/indexer.ts#L85-L94))
and has no rules or instructions file of its own.

### Where the project path is recorded

- `~/.twinny/embeddings/<workspace>/manifest.json`: `files` is keyed by the absolute path of every
  indexed file
  ([database.ts:45-51](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/extension/embeddings/database.ts#L45-L51);
  [indexer.ts:123-133](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/extension/embeddings/indexer.ts#L123-L133),
  [313](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/extension/embeddings/indexer.ts#L313)).
  The workspace root is a common prefix of those keys, not a field.
- `workspaceStorage/<id>/rjmacarthy.twinny/agent-changes/*.json` (since 4.3.5): key `root`, the
  absolute workspace root agent mode worked in
  ([src/extension/chat/reply-changes.ts:42-47](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/extension/chat/reply-changes.ts#L42-L47),
  [161](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/extension/chat/reply-changes.ts#L161));
  a grep-able `"root":"<path>"` in one-line JSON.
- Conversations carry no workspace path. The workspace link is the
  `workspaceStorage/<id>/workspace.json` that holds Twinny's per-workspace
  state.

## Parsing

### Transcript stores

- `<editor User dir>/globalStorage/state.vscdb`, `ItemTable` row whose `key` is `rjmacarthy.twinny`.
  Its `value` is one JSON object holding every `globalState` key of the extension
  ([extHostMemento.ts:118](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/workbench/api/common/extHostMemento.ts#L118);
  [extensionStorage.ts:159-176](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/platform/extensionManagement/common/extensionStorage.ts#L159-L176)).
  Inside it, `twinny.conversations` is an object of conversations keyed by id, and
  `twinny.active-conversation` a copy of the open one
  ([src/common/constants/storage.ts:1](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/common/constants/storage.ts#L1),
  [6](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/common/constants/storage.ts#L6);
  [src/extension/chat/conversation-history.ts:80-104](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/extension/chat/conversation-history.ts#L80-L104)).
  One row holds all conversations. Home-relative globs: `.config/*/User/globalStorage/state.vscdb`,
  `Library/Application Support/*/User/globalStorage/state.vscdb`,
  `AppData/Roaming/*/User/globalStorage/state.vscdb`,
  `.*-server*/data/User/globalStorage/state.vscdb`, all collected as agent `vscode`.
- Optional `twinny-server` recordings (`~/.twinny/server/recordings/`), off by
  default, cover requests from every team member; see [Other stores](#other-stores).

#### What survives a delete

Deleting a conversation removes its key from `twinny.conversations` and clears
`twinny.active-conversation` when it was open, and "clear all" writes `{}`
([conversation-history.ts:157-178](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/extension/chat/conversation-history.ts#L157-L178);
at 4.3.5 the same code is at
[conversation-history.ts:150-156](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/extension/chat/conversation-history.ts#L150-L156)
and
[:167-170](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/extension/chat/conversation-history.ts#L167-L170));
both touch only the extension's `globalState` row, and neither touches `agent-changes/` or the log.
A per-prompt history file is not stated. What remains is the per-reply file-change records under the
workspace's `agent-changes/` directory, the newest 50 kept, each with the project root, the time and
the text of every changed file before the reply, and the Twinny log channel's lines, which join a
conversation by time and text only and last until VS Code removes the log session. A `twinny-server`
gateway whose administrator turned on recording keeps its own copy of each request on the gateway
host; whether a delete reaches it is not stated, and nothing on the developer's machine says a
request was recorded. No deletion marker is stated. The analyzer reads the `rjmacarthy.twinny` row,
and the `agent-changes/` records and the log channel as secondary sources, so a deleted
conversation's file changes, project root and log lines stay in the timeline; the gateway
recordings are not read (see [Secondary sources](../analyzer/docs/output.md#secondary-sources)).

### Record schema

`Conversation`
([src/common/types.ts:127-135](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/common/types.ts#L127-L135)):
`id` (uuid v4,
[src/webview/chat.tsx:494](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/webview/chat.tsx#L494)),
`title`, `messages[]`, `pinnedTitle?`, `updatedAt?` (Unix ms, set on every save,
[conversation-history.ts:97](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/extension/chat/conversation-history.ts#L97)).

`messages[]` are `ChatCompletionMessage`: the OpenAI-style `ChatCompletionMessageParam` from
`fluency.js` (`role`, `content`) plus Twinny fields
([types.ts:50-76](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/common/types.ts#L50-L76)):

- `role`: `user` or `assistant`
  ([src/common/constants/misc.ts:8-11](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/common/constants/misc.ts#L8-L11);
  user turns
  [chat.tsx:496-503](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/webview/chat.tsx#L496-L503),
  replies
  [src/extension/chat/generation.ts:151-157](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/extension/chat/generation.ts#L151-L157)).
  The system prompt sits only in the extension's in-memory turn list
  ([src/extension/chat/index.ts:192-195](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/extension/chat/index.ts#L192-L195));
  saves come from the webview's message list
  ([src/webview/hooks/useConversationHistory.ts:44](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/webview/hooks/useConversationHistory.ts#L44)).
- `content`: string. Reasoning models' output stays inline as `<think>...</think>` or
  `<thinking>...</thinking>`; the UI splits it
  ([src/webview/utils.ts:116-121](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/webview/utils.ts#L116-L121)).
- `id?`, `images?`, `prompt?` (what was actually sent when the UI shows a
  shorter text), `context?` (workspace search hits used),
  `toolNotes?` (tool inputs and outputs carried to the next turn).
- `meta?` `ReplyMeta`
  ([types.ts:104-125](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/common/types.ts#L104-L125)):
  `model`, `provider`, `durationMs`, `completionTokens`, `promptTokens`, `contextWindow`, `stopped`,
  `withheld[]` (counts of credentials the secret shield masked).
- `toolSteps?[]` `ToolStepView`
  ([types.ts:79-101](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/common/types.ts#L79-L101)):
  `id`, `name`, `summary`, `args` (string map), `output`, `status`
  (`running|waiting|done|failed|skipped|stopped`), `command`, `approval`.

Fields added between 4.3.5 and 4.3.15
([types.ts:76-165](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/common/types.ts#L76-L165)
at
[`0337b26`](https://github.com/twinnydotdev/twinny/commit/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a)): `changes` on an assistant message (`{id, files[{path, status, added, removed, undone?,
noUndo?}]}`, the view of the record in
[Per-reply file-change records](#per-reply-file-change-records)); `toolSteps[].said` (`true` on a
step whose `name` is `said` and whose `summary` is a message the user typed while the reply ran,
[`src/extension/chat/tool-steps.ts:101-104`](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/extension/chat/tool-steps.ts#L101-L104));
`toolSteps[].mcpTool` (`server/tool`, a call to a tool of an MCP server, Model Context Protocol, the
standard by which an external program offers tools to a model) and `noAlways` (the team gateway asks
for every command), both cleared when the step finishes
([`:79-97`](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/extension/chat/tool-steps.ts#L79-L97));
`meta.outOfSteps` (the reply used every tool step `twinny.chatToolsMaxSteps` allows) and
`meta.instructions` (relative paths of the `AGENTS.md` files agent mode followed).

### Joins and timestamps

Conversation `id` is the session. Tool steps are embedded in the assistant message that used them,
so there is no call/result join. No project path is stored with a conversation; the per-workspace
state ([VS Code `globalState`](#vs-code-globalstate)) and the `workspace.json` beside it
([Where the project path is recorded](#where-the-project-path-is-recorded)) name the folder, and an
`agent-changes/` record joins through `changes.id`
([Per-reply file-change records](#per-reply-file-change-records)). The conversation list itself is
global.

There is no per-message timestamp; only the conversation's `updatedAt`.

### Other stores

#### `state.vscdb`

`state.vscdb` is VS Code's SQLite `ItemTable(key, value)`, with a `-wal` sidecar. The value is
UTF-8 JSON text. VS Code warns above 512 KB per extension
([extensionStorage.ts:47](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/platform/extensionManagement/common/extensionStorage.ts#L47),
[163-165](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/platform/extensionManagement/common/extensionStorage.ts#L163-L165)),
so long histories are common in that one row.

#### Per-reply file-change records

Secondary, new since 4.3.5: when agent mode changes files, the extension keeps one JSON file per
reply under the workspace's extension storage, `agent-changes/<uuid>.json`
([src/extension/chat/index.ts:210-221](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/extension/chat/index.ts#L210-L221);
[src/extension/chat/reply-changes.ts:211-228](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/extension/chat/reply-changes.ts#L211-L228)).
The extension's workspace storage is `<User dir>/workspaceStorage/<id>/rjmacarthy.twinny/`, and the
global `globalStorage/rjmacarthy.twinny/` when no folder is open
([extHostStoragePaths.ts:82-91](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/src/vs/workbench/api/common/extHostStoragePaths.ts#L82-L91)).
A record is `{id, root, createdAt, files[]}`, each file `{path, status, added, removed, undone?,
noUndo?, before, afterHash}`
([reply-changes.ts:35-47](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/extension/chat/reply-changes.ts#L35-L47)):
`root` is the absolute workspace root, `createdAt` epoch ms at the end of the reply, `status`
`modified`, `created` or `deleted`, `before` the whole file text before the reply (files up to 1
MiB, text only,
[`:25-26`](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/extension/chat/reply-changes.ts#L25-L26))
and `undone` set when the user put it back
([`:238-245`](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/extension/chat/reply-changes.ts#L238-L245)).

The newest 50 records are kept and older ones deleted by modification time
([`:28`](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/extension/chat/reply-changes.ts#L28),
[`:247-259`](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/extension/chat/reply-changes.ts#L247-L259)).
A record joins its conversation through the assistant message's `changes.id`
([src/common/types.ts:76-101](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/common/types.ts#L76-L101)),
and survives the conversation's deletion and "clear all", which touch only `globalState`. It is the
only Twinny record that names the project path.

#### Twinny log channel

The **Twinny** log channel (secondary, line-oriented): a VS Code log output channel
([src/common/logger.ts:93-97](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/common/logger.ts#L93-L97)),
which VS Code writes to `<extension host logs>/rjmacarthy.twinny/Twinny.log`
([extHostOutput.ts:148-150](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/src/vs/workbench/api/common/extHostOutput.ts#L148-L150))
under the editor's `logs/` directory, collected as agent `vscode`. Each entry is one line
`YYYY-MM-DD HH:MM:SS.mmm [<level>] <message>`, the time in host local time with no zone and the
level one of `trace`, `debug`, `info`, `warning` and `error`
([fileLog.ts:62-101](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/src/vs/platform/log/common/fileLog.ts#L62-L101));
a message with line breaks continues on the following lines. When the file has passed 5 MB at a
flush, VS Code moves its content to `Twinny.log_1`, then `_2` and so on up to `_6`, and starts
again at `_1`
([fileLog.ts:14](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/src/vs/platform/log/common/fileLog.ts#L14),
[41-44](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/src/vs/platform/log/common/fileLog.ts#L41-L44),
[78-81](https://github.com/microsoft/vscode/blob/4d22f4f922eb012217ac65672ccce02f59d63706/src/vs/platform/log/common/fileLog.ts#L78-L81)). With `twinny.enableLogging` on
(the default,
[package.json:633-638](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/package.json#L633-L638))
each chat request is one line `Chat → <model> (<provider label>) · <n> messages, <n> chars` and each
reply one line with its duration
([src/extension/chat/generation.ts:188-207](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/extension/chat/generation.ts#L188-L207),
[`:150-153`](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/extension/chat/generation.ts#L150-L153));
at the channel's Debug level the messages and the reply are written as blocks, a title line
(`Chat messages`, `Chat reply`, `FIM #<n> raw` and others) followed by the text with every line
indented four spaces and cut at 8,000 characters
([logger.ts:46-47](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/common/logger.ts#L46-L47),
[77-87](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/common/logger.ts#L77-L87),
[140-144](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/common/logger.ts#L140-L144)),
after the secret shield's redaction. Other Info lines record agent activity in the same channel,
among them `Chat command: <command>`, `Chat command skipped: <command>` and `Chat created
<path>`
([src/extension/chat/tool-sinks.ts:756-761](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/extension/chat/tool-sinks.ts#L756-L761);
[src/extension/edit/service.ts:390](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/extension/edit/service.ts#L390)). A line joins its conversation by time and text only (no
conversation id) and survives the conversation's deletion until VS Code removes the log session.

#### `twinny-server` recordings

`twinny-server` recordings, when enabled: SQLite `recordings.sqlite` (WAL) with `recordings(id, ts,
at, key, route, alias, model, provider, outcome, ended, ms, prompt_tokens, completion_tokens,
request, response, preview, thread)`, or JSONL day files with the same `RecordingRecord` fields
([src/gateway/recording/store.ts:25-50](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/gateway/recording/store.ts#L25-L50),
[336-355](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/gateway/recording/store.ts#L336-L355),
[369-370](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/gateway/recording/store.ts#L369-L370),
[207-211](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/gateway/recording/store.ts#L207-L211)).
`id` is `<ms, zero-padded to 14>-<hex>`
([:99-100](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/gateway/recording/store.ts#L99-L100));
`key` is the client key's name, which attributes the request to a team member.

#### Standing permission grants

In the same `rjmacarthy.twinny` value, under keys prefixed `twinny-global-context-` (the prefix the
webview gives every global value): `alwaysRunCommands` (array of normalised commands the user chose
Always run for), `alwaysRunMcpTools` (array of `server/tool` names), `autoRunCommands` (boolean, the
composer's auto-run switch: every command runs unasked) and `agentMode` (boolean, tools on)
([src/common/constants/storage.ts:25-35](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/common/constants/storage.ts#L25-L35);
[src/extension/chat/index.ts:162-192](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/extension/chat/index.ts#L162-L192),
[`:466-491`](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/extension/chat/index.ts#L466-L491)).
The command `twinny.forgetAlwaysRunCommands` clears the first list
([src/index.ts:517-526](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/index.ts#L517-L526)).
Until the switches are first used, the editor settings `twinny.chatTools` (boolean),
`twinny.chatToolsCommands` (`ask`, `allow`, `off`) and `twinny.chatToolsEdits` (`apply`, `review`)
decide
([package.json:645-683](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/package.json#L645-L683));
those are in `settings.json`, which the `vscode` parser owns. An Always run choice leaves no mark on
the step itself, which ends `done` like any other
([tool-steps.ts:169-176](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/extension/chat/tool-steps.ts#L169-L176)),
so the list is the only record of it.

The policy a developer accepted when connecting to a team gateway is kept in the same value under
`twinny.teamPolicy` until they leave the team: `{url, policy, providerIds, fetchedAt}`, `url` the
gateway, `providerIds` the provider entries the connection added and `fetchedAt` a time string
([src/extension/providers/policy.ts:20-49](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/extension/providers/policy.ts#L20-L49);
[src/common/team.ts:8-14](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/common/team.ts#L8-L14)).
`policy` holds `teamOnly`, `lockDefaults`, `recording[]`, `peers[]`, `systemPrompt` and `agent`,
whose `off`, `commands` (`ask` or `off`), `edits` (`review`), `mcp` (`off`) and `blockedPaths[]`
override the developer's own switches and Always run lists
([src/protocol/types.ts:44-100](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/protocol/types.ts#L44-L100)).
The extension also keeps the policy in force in memory
([src/common/team-policy.ts:79-89](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/common/team-policy.ts#L79-L89)),
reloaded from the stored value.

#### Pull request review repository

The review tab keeps the GitHub owner and repository it last listed pull requests for in the
extension's per-workspace state, `twinny-workspace-context-reviewOwner` and
`twinny-workspace-context-reviewRepo` (strings) in the `rjmacarthy.twinny` row of
`workspaceStorage/<id>/state.vscdb`; it fills them from the workspace's GitHub remote when both
are empty, and lists that repository's open pull requests as soon as both are set
([src/webview/review.tsx:25-62](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/webview/review.tsx#L25-L62);
[src/common/constants/storage.ts:46-47](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/common/constants/storage.ts#L46-L47);
[src/extension/webview/base.ts:362-375](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/extension/webview/base.ts#L362-L375)).
The list comes from `api.github.com` each time and is not cached
([src/extension/review/service.ts:179-197](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/extension/review/service.ts#L179-L197)).

### Format versions

No version field on conversations. `updatedAt` is absent on conversations from older builds
([types.ts:133-134](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/common/types.ts#L133-L134));
`contextItems` was previously `contextFiles`
([storage.ts:38](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/common/constants/storage.ts#L38)).

### Secrets

The same `rjmacarthy.twinny` JSON holds `twinny.inference-providers` and the
`twinny.active-*-provider` objects, each with `apiKey`
([types.ts:255-272](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/common/types.ts#L255-L272)).
Redact `apiKey` wherever it appears in the value. `twinny.chatgpt-plan.profile` names the signed-in
OpenAI account (`email`, `subject`, `name`) and is an identity value to redact; the parser never
reads it. Gateway tokens and P2P seeds are separate encrypted `secret://`
rows. In the editor's `User/settings.json`, redact `twinny.githubToken` and the `env` values of
each `twinny.chatToolsMcpServers` entry.

### Parser plan

Select `state.vscdb` (and `-wal`) under any `User/globalStorage/`, read the
row `key = 'rjmacarthy.twinny'`, parse JSON, iterate
`twinny.conversations`. Attribute rows to `twinny`, not `vscode`. Also select
`User/workspaceStorage/<id>/state.vscdb` (the same row, for
[Source host rows](#source-host-rows)),
`User/workspaceStorage/<id>/rjmacarthy.twinny/agent-changes/*.json` and
`User/globalStorage/rjmacarthy.twinny/agent-changes/*.json`, and
`logs/.../rjmacarthy.twinny/Twinny.log` with its rotated `Twinny.log_<n>` copies
([Past-use rows and secondary sources](#past-use-rows-and-secondary-sources)).

| Column | Source |
| --- | --- |
| timestamp_utc | the conversation's `updatedAt` (ms) on every row of it; a record's `createdAt`; the log line's time; the file's modification time for standing and source-host rows |
| session_id | conversation `id`; empty for the other sources |
| project_path | empty; an `agent-changes/` record's `root` |
| git_branch | empty |
| turn_type | `role`; `<think>` block → thinking; each `toolSteps[]` → tool_use plus tool_result, except a `said: true` step → user |
| model | `meta.model` (assistant) |
| tool_name, tool_use_id | `toolSteps[].name`, `toolSteps[].id` |
| text | `content` with think block removed; tool_use: `command` or `args` JSON; tool_result: `output`; a `said` step: `summary` |
| source_line | 0 (one SQLite row or JSON file); the log entry's first line number |

Model and permission rows are all `system`
([issue 96](https://github.com/seanthegeek/doubleagent/issues/96) and
[issue 97](https://github.com/seanthegeek/doubleagent/issues/97)).

#### Permission rows

- `permission decision: <status>`, with ` approval=<approval>` when the step records it, after the
  `tool_result` row of a `toolSteps[]` entry whose `status` is `waiting` or `skipped`, with the
  step's `id` and `name`. `approval` (`command` or `change`) is what the user is asked to approve,
  not the answer
  ([types.ts:87-92](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/common/types.ts#L87-L92)).
  A step that asks sets `status` `waiting` with `command` and `approval`; the user's Run or Skip
  returns it to `running` and clears both, a Skip (or a stop that answers every waiting step with
  Skip) records the step as skipped, and `finish` then saves it as `status` `skipped`; a step still
  running or waiting when the reply ends is saved `stopped`
  ([src/extension/chat/tool-steps.ts:76-93](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/extension/chat/tool-steps.ts#L76-L93),
  [131-176](https://github.com/twinnydotdev/twinny/blob/9339bd108e60781ec4d693fbec3f0bc7a0d7d3e6/src/extension/chat/tool-steps.ts#L131-L176)).
  So `waiting` is a prompt saved before it was answered, `skipped` is a step the user (or a stop)
  did not let run, and an approved step is indistinguishable from one that needed no approval: both
  end `done` or `failed`. A `stopped` step may or may not have been waiting, so it gets no decision
  row.
- Standing grants and the team policy
  ([Standing permission grants](#standing-permission-grants)), source-derived; not observed on any
  real install. Each is a `system` row with an empty `session_id`, at the modification time of
  `state.vscdb`, its text ending ` (from state.vscdb rjmacarthy.twinny, file mtime)`:
  `permission mode: agentMode=<true|false>`, `permission setting: autoRunCommands=<true|false>`,
  one `permission setting: alwaysRunCommands=<command>` per kept command and one
  `permission setting: alwaysRunMcpTools=<server/tool>` per kept tool, then one
  `permission setting: twinny.teamPolicy.<dotted key>=<value>` per leaf of `twinny.teamPolicy`
  (`url`, `policy.teamOnly`, `policy.agent.commands`, ..., `fetchedAt`), a list giving one row per
  entry. The value holds only the current switches, so `agentMode` is one row for the state at
  the file's time, not a row per change. The editor settings `twinny.chatTools*` are in
  `settings.json`, which the `vscode` parser owns, and are not read here.
- `node/trusted-peers.json`, the `twinny-node` trusted-peer list named in
  [Per-user storage](#per-user-storage),
  is not read: its shape is not recorded.

#### Model rows

`model: <id>` at the first assistant message of a conversation whose `meta.model` is set and
`model: <old> -> <new>` on every later assistant message with a different one, before that
message's rows. The model is recorded per reply, so a change inside a conversation shows. That
value is the provider entry's configured `modelName`, set before the request
([generation.ts:112](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/extension/chat/generation.ts#L112)),
not a model the server reports, and `meta.provider` is overwritten with the backend's name when a
team gateway routes the request
([`:123-126`](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/extension/chat/generation.ts#L123-L126)),
so for a team pool it names the teammate's machine that answered. The current selection is also
kept as `twinny-global-context-twinny.selectedModel` and in `twinny.active-chat-provider`
([src/extension/providers/store.ts:92-100](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/extension/providers/store.ts#L92-L100));
neither is history.

#### Past-use rows and secondary sources

The `agent-changes/` records ([Per-reply file-change records](#per-reply-file-change-records)) are
secondary for the reply they belong to, but the only source of its file changes and project path;
the log channel ([Twinny log channel](#twinny-log-channel)) is secondary and line-oriented. Both
are read on every run and marked secondary, source-derived; not observed on any real install.

- Each assistant message's `changes` gives one `system` row per file, `file changed: <status>
  <path> +<added> -<removed> reply=<changes.id>`, with ` undone=true` and ` noUndo=true` when the
  entry sets them, after the message's tool steps, in the conversation.
- Each `agent-changes/` record gives the same row per file, with `project_path` the record's
  `root`, the time its `createdAt` and an empty `session_id`, since the record names no
  conversation; `reply=<id>` is the join to the conversation's `changes.id`. The conversation
  records neither the project nor the reply's time, so deduplication cannot pair the two copies and
  both are shown. `before` and `afterHash` are never emitted.
- Each entry of `Twinny.log` and its rotated copies gives one `system` row, `log <level>: <first
  line> (from <file name>, log line time, host-local)`, followed on the next lines by the entry's
  continuation lines with the four-space block indent removed, with an empty `session_id` and
  `source_line` the entry's first line. The log time has no zone and is emitted as if it were UTC.
  A log line joins its conversation by time and text only; every entry is kept, the Debug blocks'
  prompts and replies among them, since a deleted conversation's text may survive nowhere else.

#### Sync, relay and origin rows

Twinny records no sync, relay or origin state. Conversations are written and deleted only in the
extension's `globalState`
([conversation-history.ts:100-110](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/extension/chat/conversation-history.ts#L100-L110),
[158-178](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/extension/chat/conversation-history.ts#L158-L178)),
and the extension never calls `setKeysForSync`, the call that would let VS Code's Settings Sync (the
editor's own upload of settings and extension state to the user's Microsoft or GitHub account) carry
a `globalState` key to other devices
([vscode.d.ts:8445-8457](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vscode-dts/vscode.d.ts#L8445-L8457);
[extHostMemento.ts:113-114](https://github.com/microsoft/vscode/blob/d7622a529314a4abcefd7ca9e3d7344bc9ccba9e/src/vs/workbench/api/common/extHostMemento.ts#L113-L114));
the P2P host says so in its own words, "global state is not synced"
([src/extension/p2p/host.ts:5-8](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/extension/p2p/host.ts#L5-L8)).

The features whose names suggest sharing move compute, not chats: team share and P2P (peer-to-peer)
hosting lend this machine's local model server to a team gateway or to paired devices
([src/extension/team/share.ts:1-9](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/extension/team/share.ts#L1-L9);
[src/extension/team/sharer.ts:1-9](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/extension/team/sharer.ts#L1-L9);
[host.ts:1-3](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/extension/p2p/host.ts#L1-L3)),
and their state keys `twinny.teamShare`, `twinny.teamPluginsSeen` and `twinny.teamNudge` hold the
share switch and server, the plugin ids already announced and a first-run notice flag
([src/common/constants/storage.ts:18-23](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/common/constants/storage.ts#L18-L23));
a team-pool reply names the teammate's machine only in `meta.provider` ([Model rows](#model-rows)).
The OpenAI Responses adapters, including the ChatGPT plan sign-in, send `store: false`, so the
provider keeps no server-side conversation
([src/extension/inference/adapters/openai-responses.ts:54](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/extension/inference/adapters/openai-responses.ts#L54);
[src/extension/inference/adapters/chatgpt-plan.ts:192](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/extension/inference/adapters/chatgpt-plan.ts#L192)).
Searched `sync`, `upload`, `share`, `publish`, `export`, `telemetry`, `analytics`, `sentry`,
`posthog`, `record` and `CONVERSATION_STORAGE_KEY` in `src/extension`, `src/common` and
`src/webview`; no telemetry client was found.

The one server copy is the team's own: when chat runs through a `twinny-server` gateway, the request
is the model call, and the gateway keeps it only if its administrator turns on recording
(`recording.chat`, default `false`, and only with the licence's `recording` feature,
[src/gateway/config.ts:147-148](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/gateway/config.ts#L147-L148),
[216-223](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/gateway/config.ts#L216-L223)),
grouped by a `thread` id the gateway derives from the key name and the first user message, not one
the extension sends
([src/gateway/recording/store.ts:125-135](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/gateway/recording/store.ts#L125-L135)).
The extension sends only the workspace name in `X-Twinny-Workspace`
([src/protocol/client.ts:83](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/protocol/client.ts#L83),
[252](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/protocol/client.ts#L252)),
and nothing on the developer's machine says a request was recorded, so the parser yields no sync
row; the gateway host's `recordings/` directory is the place to look
([Other stores](#other-stores)).

#### Scratch directory rows

None: Twinny has no scratch directory ([Scratch directories](#scratch-directories)).

#### Deletion rows

What a delete removes and leaves is in [What survives a delete](#what-survives-a-delete); the
plan names no deletion row.

#### Bad-line rows

The `rjmacarthy.twinny` value and each `agent-changes/` record are single JSON values: one that does
not decode is one whole-file `parser:` row (`parser: agent-changes record is not valid JSON
(<reason>)` for a record). The log channel is line-oriented: a line that is neither an entry
header nor a continuation of an open entry (indented or empty) is a bad line,
`parser: unparseable line (no log line header): <line>`, and the entries before and after it
are still yielded; a continuation that follows a bad line is a bad line too, since it has no entry
to join.

An agent-changes record or a Twinny log channel file that cannot be read is one row
`parser: <file> could not be read (<reason>)`, `<file>` its name, with `source_line` 0 and an empty
time; the log lines read before a failure are kept.

A `state.vscdb` that cannot be read, or whose `-wal`, `-shm` or `-journal` sidecar cannot, is one
row `parser: <file> could not be read (<reason>)`, `<file>` the name of the file that failed; one
that is not a SQLite database is `parser: state.vscdb is not a SQLite database`, a damaged one
`parser: state.vscdb unreadable: <message>`; a value that is not JSON is one row that never echoes
the value.

#### Fixtures

Fixture, the `value` of the `rjmacarthy.twinny` row:

```json
{"twinny.conversations":{"7d1c2b3a-1111-4000-8000-000000000001":{"id":"7d1c2b3a-1111-4000-8000-000000000001","title":"List the tests","updatedAt":1759999205000,"messages":[{"role":"user","content":"which tests fail?"},{"role":"assistant","content":"<think>run the suite</think>Two tests fail.","meta":{"model":"qwen2.5-coder:7b","provider":"Ollama","durationMs":5200},"toolSteps":[{"id":"step-1","name":"run_command","summary":"ran `npm test`","args":{"command":"npm test"},"output":"2 failing","status":"done"}]}]}},"twinny.active-conversation":{"id":"7d1c2b3a-1111-4000-8000-000000000001"},"twinny.inference-providers":{"p1":{"id":"p1","label":"Ollama","provider":"ollama","type":"chat","modelName":"qwen2.5-coder:7b","apiHostname":"localhost","apiPort":11434,"apiKey":"REDACTME"}}}
```

Since 4.3.5, keys of the same value (standing grants and a team policy) and an assistant message
with `changes`, from the types cited in [Standing permission grants](#standing-permission-grants)
and [Record schema](#record-schema):

```json
{"twinny-global-context-agentMode":true,"twinny-global-context-autoRunCommands":false,"twinny-global-context-alwaysRunCommands":["npm test"],"twinny-global-context-alwaysRunMcpTools":["github/list_issues"],"twinny.teamPolicy":{"url":"https://gateway.example.invalid","policy":{"teamOnly":true,"agent":{"commands":"ask","blockedPaths":[".env"]}},"providerIds":["team-0a1b-chat"],"fetchedAt":"2026-10-03T09:00:00.000Z"}}
{"role":"assistant","content":"Fixed both.","meta":{"model":"qwen2.5-coder:7b"},"changes":{"id":"5f0e9d8c-2222-4000-8000-000000000001","files":[{"path":"src/a.ts","status":"modified","added":3,"removed":1}]}}
```

An `agent-changes/<id>.json` record
([Per-reply file-change records](#per-reply-file-change-records)):

```json
{"id":"5f0e9d8c-2222-4000-8000-000000000001","root":"/srv/proj","createdAt":1791029098000,"files":[{"path":"src/a.ts","status":"modified","added":3,"removed":1,"undone":true,"before":"export const a = 1\n","afterHash":"<sha256 hex>"}]}
```

`Twinny.log` entries ([Twinny log channel](#twinny-log-channel)):

```text
2026-10-03 14:00:01.250 [info] Chat → qwen2.5-coder:7b (Ollama) · 2 messages, 41 chars
2026-10-03 14:00:01.251 [debug] Chat messages
    [user]
    which tests fail?
2026-10-03 14:00:06.400 [info] Chat ← 5.1s · 15 chars
```

The workspace state value of the review tab
([Pull request review repository](#pull-request-review-repository)):

```json
{"twinny-workspace-context-reviewOwner":"acme","twinny-workspace-context-reviewRepo":"widgets"}
```

#### Steps typed mid-reply

A `toolSteps[]` entry with `said: true` (since 4.3.5; `name` `said`, `id` `said.<id>`, the message
in `summary`, `status` `done`) is a message the user sent while the reply ran, and the parser emits
it as a `user` row with the `summary` as its text, in its place among the reply's steps, with no
`tool_name`, `tool_use_id` or `model`
([`src/extension/chat/tool-steps.ts:101-104`](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/extension/chat/tool-steps.ts#L101-L104),
[`src/common/types.ts:129-133`](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/common/types.ts#L129-L133));
before the fix for [issue 171](https://github.com/seanthegeek/doubleagent/issues/171) it was a
`tool_use` named `said` and an empty `tool_result`. `changes` is read
([Past-use rows and secondary sources](#past-use-rows-and-secondary-sources));
`mcpTool`, `meta.outOfSteps` and `meta.instructions` are not read. The `said` step layout is not
observed on any real install, source-derived (twinnydotdev/twinny at
[`0337b26`](https://github.com/twinnydotdev/twinny/commit/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a)).

#### Source host rows

The extension connects to GitHub only for its pull request review, with the personal access token
of the `twinny.githubToken` editor setting ([Credentials](#credentials)) sent as a bearer token,
or none
([src/extension/review/service.ts:154-177](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/extension/review/service.ts#L154-L177)).
No account login is stored: the extension never asks GitHub who the token is. The repository the
review tab last listed is in the workspace state
([Pull request review repository](#pull-request-review-repository)); the pull request list is
fetched each time, and a review becomes an ordinary conversation whose first user message begins
`**Code review: PR #<number> <title>**`
([:199-218](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/extension/review/service.ts#L199-L218),
[:303](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/extension/review/service.ts#L303);
[src/extension/review/diff.ts:255-260](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/extension/review/diff.ts#L255-L260)).
The parser yields `source host repository: https://github.com/<owner>/<repo> type=review (from
workspaceStorage state.vscdb rjmacarthy.twinny, file mtime)` when both keys are set, a `system` row
with an empty `session_id` at the modification time of that `state.vscdb`; source-derived; not
observed on any real install. It never reads `settings.json`.

On a `twinny-server` gateway host, the GitHub, GitLab, Gitea and Bitbucket plugins
([src/gateway/plugins/index.ts:18-21](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/gateway/plugins/index.ts#L18-L21))
keep each forge's watched repositories and saved access tokens in
`~/.twinny/server/plugins/<id>/repos.json`, `{version, repos[], tokens[], settings}`, a repository
`{id, fullName, auth, tokenId, addedAt, addedBy, autoReview, autoPost, autoTriage}` and a token
`{id, token, label, login, addedAt, addedBy, replacedAt}`, `login` being the account the host said
the token belongs to
([src/gateway/plugins/forge.ts:109-143](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/gateway/plugins/forge.ts#L109-L143),
[518-526](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/gateway/plugins/forge.ts#L518-L526);
[src/gateway/plugins/pulls.ts:33-35](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/gateway/plugins/pulls.ts#L33-L35),
[165](https://github.com/twinnydotdev/twinny/blob/0337b265bf3b9f8f00ff45d597ae1f2e99ef230a/src/gateway/plugins/pulls.ts#L165)).
The collector flags the file as a secret (`.twinny/server/plugins/*/repos.json`). The parser does
not read it: a self-hosted forge's address is in the plugin's `settings`, which this review did
not trace, so the `source host:` and `source host repository:` rows for it are left for the
gateway's own round.

## Confidence

- High (source): the extension id, the `globalState` key names, the provider file name and
  setting, the `~/.twinny` subpaths, the gateway data directory layout, the secret storage keys,
  the row key, the `Conversation` and message fields, the roles and the `updatedAt` unit.
- High (VS Code source): `globalState` stored in `state.vscdb` under the extension id.
- Medium: `chunks.lance` as the on-disk name of the LanceDB table (LanceDB's naming convention, not
  visible in Twinny's source); `content` is always a string in saved chats (the `fluency.js` type
  also allows part arrays; Twinny writes strings at the cited sites).
- Not determined: sizes of a real index; whether `invites.json` holds reusable secrets; how the
  JetBrains or other editors (Twinny is VS Code only at this commit) would differ; how
  `toolSteps[].args` names the command for each tool; whether other editors running the extension
  (VSCodium, Cursor) store it identically (they share the VS Code storage code, so likely).
