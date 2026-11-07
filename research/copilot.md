# copilot

The catalog agent `copilot` is the GitHub Copilot OAuth token store shared by editor plugins:
the closed-source `@github/copilot-language-server` that the Copilot plugins for Vim, Neovim,
JetBrains and Xcode embed writes it. The GitHub Copilot CLI does not use it (see
[`research/copilot-cli.md`](copilot-cli.md)), and Copilot Chat in VS Code is parsed as `vscode`.
It is collected only, with no parser: the server keeps no transcripts.

## Source and evidence

Source/bundle: [github/copilot.vim](https://github.com/github/copilot.vim) at
[a12fd5672110c8aa7e3c8419e28c96943ca179be](https://github.com/github/copilot.vim/commit/a12fd5672110c8aa7e3c8419e28c96943ca179be)
(2026-01-09). The plugin vendors the closed `@github/copilot-language-server` 1.408.0
([`copilot-language-server/package.json`](https://github.com/github/copilot.vim/blob/a12fd5672110c8aa7e3c8419e28c96943ca179be/copilot-language-server/package.json))
as a minified bundle
[`copilot-language-server/dist/main.js`](https://github.com/github/copilot.vim/blob/a12fd5672110c8aa7e3c8419e28c96943ca179be/copilot-language-server/dist/main.js)
(7.2 MB, sha256 `580e594aa8305d73e5af1f6e54124ccfc35f2aed6e5af3cb7b57546049e8b81f`); the
token-store code is in that bundle (readable JS, cited by function name, and as `@<n>` for a byte
offset into it). The language server is closed source, so everything here rests on that bundle.
[`autoload/copilot/client.vim:514-515`](https://github.com/github/copilot.vim/blob/a12fd5672110c8aa7e3c8419e28c96943ca179be/autoload/copilot/client.vim#L514-L515)
names the bundle.

### Observed

- No real install. Confirmed names-only that `~/.config/github-copilot` is absent on this host
  (no editor Copilot installed), so no local confirmation.

### Source reviews

- 2026-10-19: HEAD of github/copilot.vim was still
  [`a12fd56`](https://github.com/github/copilot.vim/commit/a12fd5672110c8aa7e3c8419e28c96943ca179be),
  vendoring the same language server 1.408.0. The review found `versions.json` and `oauth.json`
  in the token store, correcting the earlier statement that `versions.json` is not written
  ([Token store files](#token-store-files)), the server's cache directory
  ([Cache directory](#cache-directory)), and that the server keeps no permission, model, sync or
  conversation state on disk ([Parsing](#parsing)).

## Collection

### Per-user storage

[`main.js`](https://github.com/github/copilot.vim/blob/a12fd5672110c8aa7e3c8419e28c96943ca179be/copilot-language-server/dist/main.js)
`function Alt()` ("getXdgConfigPath"):

```text
XDG_CONFIG_HOME set and absolute → $XDG_CONFIG_HOME/github-copilot
win32                            → %USERPROFILE%\AppData\Local\github-copilot
else                             → $HOME/.config/github-copilot
```

So macOS and Linux use the XDG config dir `~/.config/github-copilot` (no Library path), Windows uses
`AppData\Local`. The only relocation variable is `XDG_CONFIG_HOME`.

#### Scratch directories

None known. The server keeps a rebuildable workspace index in its cache directory
([Cache directory](#cache-directory)), which is not a working directory.

#### Relocation variables

| Variable | Moves | Replaces | Table agent |
| --- | --- | --- | --- |
| `XDG_CONFIG_HOME` | the parent of the token store `github-copilot`, when absolute (`copilot-language-server 1.408.0 dist/main.js @5032324` `function Alt()`) | `.config` | `shared` |
| `XDG_CACHE_HOME` | the parent of the cache directory `github-copilot`, when absolute (`copilot-language-server 1.408.0 dist/main.js @6043359` `function wEt()`) | `.cache` | `shared` |

#### Token store files

Files written there (same function's caller in
[`main.js`](https://github.com/github/copilot.vim/blob/a12fd5672110c8aa7e3c8419e28c96943ca179be/copilot-language-server/dist/main.js)):

- `apps.json`: current store, object keyed `"<host>:<githubAppId>"` →
  `{user, oauth_token, githubAppId}`.
- `hosts.json`: legacy store keyed by host → `{user, oauth_token}`; on startup entries are copied
  into `apps.json` and `hosts.json` is deleted (`writeFile(join(l,"apps.json"))` then
  `rm(join(l,"hosts.json"),{force:true})`).
- `versions.json` and `oauth.json`: the literal
  `versions.json` is not in the bundle, but the store is a `FilePersistenceManager` that writes each
  setting to `<directory>/<setting>.json` (`copilot-language-server 1.408.0 dist/main.js @5030498`
  `static{a(this,"FilePersistenceManager")}`, `@5030696` ``let s=`${this.directory}/${r}.json` ``),
  and two settings use it: `versions`, the editor plugin's name mapped to its installed version
  (`@6728429` `get(Hn).read("versions",r.name)`), and `oauth`, the OAuth sessions of an
  authentication provider keyed by provider id, `https://github.com/login/oauth` for GitHub
  (`@6928563` `static{this.providerId="https://github.com/login/oauth"}`, `@6928865`
  `read("oauth",this.id)`).

#### Cache directory

The server also has a cache root, `$XDG_CACHE_HOME/github-copilot` when that is absolute, else
`%USERPROFILE%\AppData\Local\Temp\github-copilot` on Windows and `~/.cache/github-copilot` elsewhere
(`@6043578` `a(wEt,"getXdgCachePath")`). Under it
`project-index/<folder name>.<first 8 hex of SHA-256 of the folder path>/` holds the workspace index
databases `local-index.db` and `workspace-chunks.db` (`@6181541` `"project-index"`, `@6181570`
`a(x8e,"getWorkspaceCachePath")`, `@6208330` `"local-index.db"`, `@6182984`
`"workspace-chunks.db"`). It is a rebuildable code index, not collected; its directory names give a
workspace's folder name but not its path.

#### Other surfaces

The language server also creates `$TMPDIR/github-copilot-<rand>` temp dirs on non-Windows (same
code), outside home. Logs go to the editor (Vim `:Copilot log`), not to this directory. The same
store is read by Copilot plugins for Neovim, JetBrains, Xcode and Vim because they embed the same
language server. VS Code's Copilot Chat state lives in `User/globalStorage` (covered by the vscode
entries) and its token in the VS Code secret store, not here.

The terminal, edit and MCP (Model Context Protocol, the tool-server interface) auto-approve rules
reach the server as editor configuration (`tools.terminal.autoApprove`, `tools.edit.autoApprove`,
`tools.mcp.autoApprove`; `copilot-language-server 1.408.0 dist/main.js`
`applyTerminalAutoApproveConfiguration`, `applyEditAutoApproveConfiguration`,
`applyMCPAutoApproveConfiguration`). The server does not write them; they live in each editor's
own settings.

### Credentials

`apps.json` and `hosts.json` hold the GitHub OAuth token in plaintext (`oauth_token` field) with the
GitHub login (`user`). `oauth.json` holds an authentication provider's OAuth sessions ([Token store
files](#token-store-files)). All four files, `versions.json` included, match the directory's
`*.json` secret glob. No keychain use in this path. No config file embeds API keys.

### Exclusions

None; the directory holds only small JSON files.

### Project-local files

Read only: `.github/copilot-instructions.md`, `.github/git-commit-instructions.md`,
`.github/instructions/`
([`main.js`](https://github.com/github/copilot.vim/blob/a12fd5672110c8aa7e3c8419e28c96943ca179be/copilot-language-server/dist/main.js)
constants `QNi`, `jNi`, `Ckr`). Nothing written.

### Where the project path is recorded

Not recorded; the store is per host, not per workspace.

## Parsing

`copilot` is `collected` only: there is nothing to parse. The directory is an OAuth token store
with a version setting beside it ([Token store files](#token-store-files)), and the language
server writes no conversation, history or log file there: no quoted `.jsonl` file name and no
conversation file occurs in the bundle (grepped for quoted `*.json`, `*.jsonl`, `*.db` and `*.log`
names, and for `persistConversation` and `saveConversation`). Chat history is the editor's; Copilot
Chat in VS Code is parsed as `vscode` ([Other surfaces](#other-surfaces)). The server records no
model, no permission decision (the auto-approve rules are editor settings) and no sync state, and
keeps no session, so a delete has nothing to leave behind.

## Confidence

- High: path resolution and the four token-store file names with their fields (readable code in
  the shipped bundle, version pinned).
- Medium: that JetBrains/Xcode plugins use the same store (same language server, but their bundles
  were not inspected).
- Not determined: whether a terms-acceptance file is written by other editor hosts; when the
  server builds its workspace index ([Cache directory](#cache-directory)).
