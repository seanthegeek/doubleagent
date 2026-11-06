# atlarix

Atlarix is a closed-source coding agent from NorahLabs with a desktop app (macOS, Windows,
Linux), a CLI (`atlarix` on npm, Homebrew), an extension for VS Code, Cursor, Kiro and Windsurf
that bundles the CLI, an Agent Client Protocol agent for Zed and JetBrains IDEs (`atlarix acp`)
and a Chrome extension. The desktop app, the CLI, the editor extension and the ACP agent write
one shared tree, so they are one catalog agent. Its parser is in the backlog; no parser issue is
filed yet.

## Source and evidence

Closed source. Evidence:

1. Vendor statement by Amariah Abishai (NorahLabs), 2026-10-27, for 15.11.1:
   [issue 60, comment 6430492071](https://github.com/seanthegeek/doubleagent/issues/60#issuecomment-6430492071).
2. The vendor's check of the macOS directory names against an installed copy, 2026-10-27.

### Observed

No real install.

### Source reviews

None.

## Collection

### Per-user storage

#### Data directory

Shared by the desktop app, the CLI, the editor extension and the ACP agent:

- macOS: `~/Library/Application Support/Atlarix/`
- Windows: `%APPDATA%\Atlarix\` (`AppData/Roaming/Atlarix`), also for the Microsoft Store build
- Linux: `$XDG_CONFIG_HOME/Atlarix/`, default `~/.config/Atlarix/`

Unattended runs (`atlarix run`) use a separate folder named by `ATLARIX_USER_DATA_DIR`.

What it holds:

- `sqlite.db` with `sqlite.db-wal` and `sqlite.db-shm`: sessions and messages.
- `user-settings.json`: settings ([Credentials](#credentials)).
- `backups/`: copies of `sqlite.db` taken before an update or a repair.
- `shadow-snapshots/`: copies of project files taken before the agent edits them, for undo.
- `logs/`: the app and CLI logs.
- Chromium's profile files and caches (`Cache`, `Code Cache`, `GPUCache`, `DawnGraphiteCache`,
  `DawnWebGPUCache`, `blob_storage`, `Crashpad` and the rest of an Electron user-data folder).

#### Home directory `~/.atlarix`

On every OS. It holds:

- `projects/<slug>/memory/`: project memory notes, Markdown files with a `MEMORY.md` index.
- `tool-outputs/`: large tool results the agent saved.
- `logs/`, and small state files for the CLI.
- `bin/` (the CLI and helper binaries) and `skills/` (a cache of skills that ship with the app).

#### Chrome extension

Keeps its own sign-in, conversations and notes in the browser's extension storage, not in the
data directory.

#### Editor extension

Nothing of its own beyond two small flags in the editor's `globalState`: it runs its bundled
CLI, which uses the [data directory](#data-directory).

#### Scratch directories

`~/.atlarix/tool-outputs/`.

#### Relocation variables

| Variable | Moves | Replaces | Table agent |
| --- | --- | --- | --- |
| `XDG_CONFIG_HOME` | the parent of the Linux data directory `Atlarix/` ([vendor statement](https://github.com/seanthegeek/doubleagent/issues/60#issuecomment-6430492071)) | `.config` | `shared` |

Not confirmed: `ATLARIX_USER_DATA_DIR`, which the data directory section says names the folder of
unattended runs; the vendor statement does not mention it and no other source is cited.

### Credentials

- `supabase-auth.enc` and `supabase-auth-cli.enc` in the data directory: the sign-in sessions.
- `user-settings.json`: the user's own provider API keys, encrypted with the OS keychain
  (Electron safeStorage). On Linux without a keyring they can be stored unencrypted in the same
  file.
- `sqlite.db` and its copies in `backups/`: MCP server credentials, encrypted the same way.

### Exclusions

The Chromium caches in the data directory, `shadow-snapshots/` (copies of project files),
`~/.atlarix/bin` and `~/.atlarix/skills`.

### Project-local files

`.atlarix/` in the repository: rules, skills and hooks.

### Where the project path is recorded

`sqlite.db`, table `apps`.

## Parsing

Atlarix is `backlog`: its sessions and messages are in `sqlite.db`
([Per-user storage](#per-user-storage)), and no parser issue is filed yet.

## Confidence

- High (vendor statement): the data directory on each OS, the files in it, `~/.atlarix` and the
  credential files.
- Not verified on a real install: the Windows and Linux paths and what each surface writes step
  by step.
