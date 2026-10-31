<#
.SYNOPSIS
  Forensic collector for AI coding agent artifacts on Windows.

.DESCRIPTION
  Windows counterpart of collect-agent-artifacts.sh. Collects the on-disk state
  of AI coding agents (Claude Code, Gemini CLI, Antigravity, Codex CLI, Copilot
  CLI, Cursor, Windsurf, Continue, Aider, Ollama, ...) for every user profile on
  a host, from Docker and Podman named volumes it can reach on disk, into a
  single tar.gz (via the built-in tar.exe on Windows 10 1803+) or zip, with a JSONL
  manifest. Same catalog, manifest schema and archive layout as the sh script.
  Host state (processes, users, network, shell histories) is left to the EDR
  it supplements.

  Windows PowerShell 5.1 compatible. No modules, no prompts, nothing from stdin.
  Run as:  powershell.exe -ExecutionPolicy Bypass -File Collect-AgentArtifacts.ps1 -OutputDir C:\ir

.PARAMETER OutputDir
  Directory for the archive (default: current directory).
.PARAMETER Root
  Alternate root such as a mounted disk image (image mode).
.PARAMETER Users
  Comma-separated profile names to collect (default: all).
.PARAMETER Project
  Extra project directory to collect (repeatable). A relative path is
  resolved against the current location, and the path is never prefixed
  with -Root.
.PARAMETER Full
  Disable default size exclusions.
.PARAMETER NoSecrets
  Skip credential files instead of collecting them.
.PARAMETER NoProjects
  Skip project-level artifact discovery.
.PARAMETER NoDocker
  Skip Docker and Podman volume enumeration. Docker Desktop keeps Linux volumes
  inside a WSL virtual disk that this script cannot reach; it only notes that.
.PARAMETER MaxFileSizeMB
  Skip files larger than this (default 256, 0 = none).
.PARAMETER KeepStaging
  Keep the staging directory after archiving.
.PARAMETER Inventory
  Write nothing: walk the catalog with file metadata only and print one JSON
  line per user and agent found (file count, bytes, first and last
  modification time) to stdout. -OutputDir is ignored.
.PARAMETER List
  Print the artifact catalog and exit.
.PARAMETER Quiet
  Only print the final summary.
.PARAMETER Version
  Print version and exit.

.NOTES
  Copyright 2026 Sean Whalen
  SPDX-License-Identifier: Apache-2.0
  Exit codes: 0 archive written (per-file errors are in the manifest), or,
                with -Inventory, the walk ran,
              1 usage error, 2 fatal.
#>
[CmdletBinding()]
param(
  [Alias('o')][string]$OutputDir = '.',
  [Alias('r')][string]$Root = '',
  [Alias('u')][string]$Users = '',
  [Alias('p')][string[]]$Project = @(),
  [switch]$Full,
  [switch]$NoSecrets,
  [switch]$NoProjects,
  [switch]$NoDocker,
  [int]$MaxFileSizeMB = 256,
  [Alias('k')][switch]$KeepStaging,
  [switch]$Inventory,
  [switch]$List,
  [Alias('q')][switch]$Quiet,
  [switch]$Version
)

Set-StrictMode -Version 2
$ErrorActionPreference = 'Continue'
$ToolVersion = '1.21.0'
$TOOL = 'collect-agent-artifacts'

# ===========================================================================
# Reader's guide
# ===========================================================================
# The collector is one file so that an EDR console can upload it and run
# it once. PowerShell requires the comment-based help and param() above to
# come first; after this preamble the file reads top to bottom in twelve
# sections, the same sections in the same order as the sh collector,
# collect-agent-artifacts.sh:
#
#    1. Reader's guide             this block
#    2. Data tables                CATALOG, PROJECT_CATALOG, EXCLUDES,
#                                  SECRET_GLOBS and DOCKER_VOLUMES
#    3. Shared helpers             logging, notes, JSON escaping, glob to
#                                  regex, extended paths, owner, epoch, hash
#    4. Manifest writer            the row writer and the error_read row
#    5. Staging files              one file: metadata first, copy, hash the
#                                  copy, row; symlinks; excluded paths
#    6. Catalog walk               glob expansion, claimed paths, the walk
#    7. Users                      enumerating and filtering home directories
#    8. Docker and Podman volumes  named volumes as further bases
#    9. Project discovery          project paths read from agent state
#   10. Inventory output           the -Inventory counters and JSON lines
#   11. Summary and archive        collection.json, the archivers, stdout
#   12. Main program               setup, then one call per step, in order
#
# Data flow. The tables are data, not code: an agent is added by adding
# lines to them. Get-UserHomes lists the home directories. For each home,
# Expand-Catalog expands every CATALOG line before anything is walked and
# claims each match, so a nested entry (.gemini/antigravity-cli inside
# .gemini) keeps its subtree and the walk of the enclosing entry skips it.
# Add-CollectedPath and Add-CollectedTree walk each match without following
# reparse points; a path matching EXCLUDES is pruned with one
# skipped_excluded row, and a file matching SECRET_GLOBS is flagged secret.
# Add-CollectedFile reads a file's metadata before copying it, copies it
# with shared access into the staging tree under fs/<original path>, hashes
# the staged copy rather than the source, and writes its manifest row
# through Write-ManifestRow; it recreates a symbolic link or junction
# instead of following it, and records a socket, FIFO, device or other
# reparse point without opening it. Then collection.json summarises the run and
# the staging tree is archived (tar.exe, else the PowerShell tar writer,
# else a zip). Docker and Podman named volumes (DOCKER_VOLUMES) are
# collected whole, each as one match; the discovered project directories
# are further bases walked with PROJECT_CATALOG. -Inventory is the same
# walk with no writes: each file feeds per-agent counters, and
# Write-Inventory prints JSON lines.
#
# Naming. Script state shared between functions is $script:PascalCase and
# is written and read with that prefix; the parameters (-Root, -Inventory,
# ...), the five tables, $TOOL and $ToolVersion are read by their plain
# names. Parameters of functions and locals are $camelCase and never take
# an automatic variable's name ($home, $host, $args, $input, $error,
# $matches, $pid, $profile). Functions use approved verbs, and a function
# that does the same step as one in the sh collector has the matching name
# (Add-CollectedPath and collect_path, Write-ManifestRow and
# write_manifest_row). Every function has a comment above it: what it
# takes, what it returns or sets, and the quirk it works around when there
# is one. There are no worker programs on this side; every step runs in
# this process.
#
# collectors/docs/internals.md describes the architecture and lists the sh
# and PowerShell function pairs.

# ===========================================================================
# Data tables
# ===========================================================================
# CATALOG (agent|glob relative to each home), PROJECT_CATALOG (agent|glob
# relative to each discovered project), EXCLUDES and SECRET_GLOBS (globs
# relative to the collection base, where * crosses /) and DOCKER_VOLUMES
# (agent|volume name glob). Byte-identical to the tables in
# collect-agent-artifacts.sh; tests/catalog-sync.sh fails if they drift.
$CATALOG = @'
# Anthropic
claude-code|.claude
claude-code|.claude.json*
# Claude Code config homes moved with CLAUDE_CONFIG_DIR (~/.claude-work):
# anchored on Claude-only names, never a bare .claude-*
claude-code|.claude-*/projects
claude-code|.claude-*/file-history
claude-code|.claude-*/history.jsonl
claude-code|.claude-*/.claude.json*
claude-code|.claude-*/.credentials.json
claude-code|.claude-*/hfi-auth.json
claude-code|.claude-*/.session_ingress_token
claude-code|.claude-*/remote/.oauth_token
claude-code|.claude-*/remote/.api_key
claude-code|.claude-*/sessions/*.*.key
claude-code|.claude-*/daemon/control.key
claude-code|.claude-*/daemon/pipe.key
claude-code|.claude-*/daemon/roster.json
claude-code|.claude-*/backups/.claude.json.*
# Running-process credentials in a relocated config home: the IDE bridge
# lock (authToken) and the Remote Control spawn MCP definition
claude-code|.claude-*/ide/*.lock
claude-code|.claude-*/bridge-spawn/*/mcp.json
claude-code|.local/share/claude
# Claude Code version locks and its staging/ directory, home-relative on Linux
# and Windows alike
claude-code|.local/state/claude
claude-code|.cache/claude
# Claude Code CLI cache: MCP server logs per project. The Windows
# entry names the children of Cache/: the PowerShell exclusion match ignores
# case, so */Claude*/Cache would prune claude-cli-nodejs/Cache itself
claude-code|Library/Caches/claude-cli-nodejs
claude-code|.cache/claude-cli-nodejs
claude-code|AppData/Local/claude-cli-nodejs/Cache/*
# Claude Code Windows temp root, under the home (scratchpads and tasks); the
# Linux and macOS roots are outside every home (research/claude-code.md)
claude-code|AppData/Local/Temp/claude
# Claude Code macOS URL handler: the bundle registering the claude-cli: URL
# scheme, install evidence that outlives ~/.local
claude-code|Applications/Claude Code URL Handler.app
claude-desktop|Library/Application Support/Claude
claude-desktop|Library/Application Support/Claude-3p
claude-desktop|Library/Logs/Claude
claude-desktop|Library/Logs/Claude-3p
claude-desktop|.config/Claude
claude-desktop|.config/Claude-3p
claude-desktop|AppData/Roaming/Claude
claude-desktop|AppData/Roaming/Claude-3p
claude-desktop|AppData/Local/Claude-3p
# Claude Desktop MSIX build: userData is virtualised under Packages,
# logs stay in AppData/Local/Claude, Claude-Data is the unpackaged fallback
claude-desktop|AppData/Local/Claude
claude-desktop|AppData/Local/Claude-Data
claude-desktop|AppData/Local/Packages/Claude_*/LocalCache/Roaming/Claude
claude-desktop|AppData/Local/Packages/AnthropicPBC.Claude_*/LocalCache/Roaming/Claude
claude-desktop|Claude
claude-desktop|Documents/Claude
# Google: Gemini CLI and Antigravity (research/gemini-cli.md,
# research/antigravity.md). The antigravity entries nest inside ~/.gemini and
# claim their subtrees. The Antigravity Cache exclusion names its three bases:
# the PowerShell match ignores case, so */Antigravity/Cache would also prune
# .gemini/antigravity/cache. AppData/Local/antigravity names one whole
# component, so it matches neither antigravity-updater nor the installed app.
gemini-cli|.gemini
gemini-cli|.cache/.gemini
antigravity|.gemini/antigravity-cli
antigravity|.gemini/antigravity-ide
antigravity|.gemini/antigravity
antigravity|.gemini/antigravity-backup
antigravity|.gemini/config
antigravity|.antigravity
antigravity|.cache/antigravity
antigravity|AppData/Local/antigravity
antigravity|.config/Antigravity
antigravity|Library/Application Support/Antigravity
antigravity|Library/Logs/Antigravity
antigravity|AppData/Roaming/Antigravity
antigravity|.config/antigravity/plugin-secrets.json
antigravity|Library/Application Support/antigravity/plugin-secrets.json
antigravity|AppData/Roaming/antigravity/plugin-secrets.json
antigravity|.config/Antigravity IDE/User
antigravity|.config/Antigravity IDE/logs
antigravity|Library/Application Support/Antigravity IDE/User
antigravity|Library/Application Support/Antigravity IDE/logs
antigravity|AppData/Roaming/Antigravity IDE/User
antigravity|AppData/Roaming/Antigravity IDE/logs
# OpenAI
codex-cli|.codex
# Codex: the working directory the ChatGPT desktop app gives a thread with no
# project (research/codex-cli.md)
codex-cli|Documents/Codex
chatgpt-desktop|Library/Application Support/com.openai.chat
chatgpt-desktop|AppData/Local/Packages/OpenAI.ChatGPT-Desktop_*/LocalCache/Roaming/ChatGPT
chatgpt-desktop|AppData/Local/Packages/OpenAI.ChatGPT-Desktop_*/LocalState
# ChatGPT desktop on Windows, Store package OpenAI.Codex: the profile and the
# files beside it in the Chromium user-data directory are named rather than
# the directory, so the Chromium component data is not walked
# (research/chatgpt-desktop.md)
chatgpt-desktop|AppData/Local/Packages/OpenAI.Codex_*/LocalCache/Roaming/Codex/web/Codex/Default
chatgpt-desktop|AppData/Local/Packages/OpenAI.Codex_*/LocalCache/Roaming/Codex/web/Codex/Local State
chatgpt-desktop|AppData/Local/Packages/OpenAI.Codex_*/LocalCache/Roaming/Codex/web/Codex/Last Version
chatgpt-desktop|AppData/Local/Packages/OpenAI.Codex_*/LocalCache/Roaming/Codex/web/Codex/statsig-state.json
chatgpt-desktop|AppData/Local/Packages/OpenAI.Codex_*/LocalCache/Roaming/Codex/web/Codex/.codex-tpp-local-executor-installation-id
chatgpt-desktop|AppData/Local/Packages/OpenAI.Codex_*/LocalCache/Local/Codex
chatgpt-desktop|AppData/Local/Packages/OpenAI.Codex_*/Settings
# ChatGPT desktop on macOS, bundle id com.openai.codex: the profile and the
# files beside it in Application Support/Codex are named, as on Windows. The
# com.openai.chat line above is the former native app.
chatgpt-desktop|Library/Application Support/com.openai.codex
chatgpt-desktop|Library/Application Support/Codex/Default
chatgpt-desktop|Library/Application Support/Codex/Local State
chatgpt-desktop|Library/Application Support/Codex/Last Version
chatgpt-desktop|Library/Application Support/Codex/statsig-state.json
chatgpt-desktop|Library/Application Support/Codex/.codex-tpp-local-executor-installation-id
chatgpt-desktop|Library/Application Support/Codex/browser-sidebar-page-states.json
chatgpt-desktop|Library/Application Support/Codex/executor-plugins
chatgpt-desktop|Library/Application Support/Codex/artifact-sessions
chatgpt-desktop|Library/Application Support/com.openai.codex.installer
chatgpt-desktop|Library/Logs/com.openai.codex
chatgpt-desktop|Library/Preferences/com.openai.codex.plist
chatgpt-desktop|Library/Preferences/com.openai.codex.installer.plist
chatgpt-desktop|Library/Preferences/com.openai.sky.CUAService.plist
chatgpt-desktop|Library/HTTPStorages/com.openai.codex
chatgpt-desktop|Library/HTTPStorages/com.openai.codex.binarycookies
chatgpt-desktop|Library/HTTPStorages/com.openai.sky.CUAService
chatgpt-desktop|Library/HTTPStorages/com.openai.sky.CUAService.binarycookies
chatgpt-desktop|Library/Group Containers/2DC432GLL2.com.openai.sky.CUAService
# GitHub Copilot (CLI, and the OAuth token store used by Copilot plugins)
copilot-cli|.copilot
copilot-cli|.config/.copilot
copilot-cli|.local/state/.copilot
copilot-cli|.cache/copilot
copilot-cli|Library/Caches/copilot
copilot-cli|AppData/Local/copilot
copilot|.config/github-copilot
copilot|AppData/Local/github-copilot
# GitHub Copilot Desktop (Tauri app, package "github"): runs its sessions through a bundled
# Copilot CLI inside ~/.copilot (session-state stays copilot-cli); these nested entries claim
# the store, chat and repository directories, run files and log of the app itself
copilot-desktop|.copilot/data.db*
copilot-desktop|.copilot/repo-metadata-cache.db*
copilot-desktop|.copilot/chats
copilot-desktop|.copilot/repos
copilot-desktop|.copilot/repos/*/.git
copilot-desktop|.copilot/run
copilot-desktop|.copilot/media-cache
copilot-desktop|.copilot/canvas-catalog-probe
copilot-desktop|.copilot/logs/github-app.*.log
copilot-desktop|.local/share/com.github.githubapp
copilot-desktop|Library/Application Support/com.github.githubapp
copilot-desktop|AppData/Roaming/com.github.githubapp
copilot-desktop|AppData/Local/com.github.githubapp
copilot-desktop|.github-copilot-cli
# Cursor
cursor|.cursor
cursor|.config/cursor
cursor|AppData/Roaming/Cursor/auth.json
cursor|.cursor-server/data/User
cursor|Library/Application Support/Cursor/User
cursor|Library/Application Support/Cursor/logs
cursor|Library/Application Support/Cursor/AgentStores
cursor|Library/Application Support/Cursor/update-supervisor
cursor|.config/Cursor/User
cursor|.config/Cursor/logs
cursor|.local/state/cursor/agent-stores
cursor|.config/Cursor/update-supervisor
cursor|AppData/Roaming/Cursor/User
cursor|AppData/Roaming/Cursor/logs
cursor|AppData/Local/Cursor/AgentStores
cursor|AppData/Roaming/Cursor/update-supervisor
# VS Code and VSCodium: globalStorage holds Copilot Chat, Cline, Roo, Kilo, Continue, Augment state
vscode|.vscode*/extensions/extensions.json
vscode|Library/Application Support/Code*/User
vscode|Library/Application Support/Code*/logs
vscode|.config/Code*/User
vscode|.config/Code*/logs
vscode|AppData/Roaming/Code*/User
vscode|AppData/Roaming/Code*/logs
vscode|Library/Application Support/VSCodium*/User
vscode|Library/Application Support/VSCodium*/logs
vscode|.config/VSCodium*/User
vscode|.config/VSCodium*/logs
vscode|AppData/Roaming/VSCodium*/User
vscode|AppData/Roaming/VSCodium*/logs
vscode|Library/Application Support/Positron/User
vscode|.config/Positron/User
vscode|AppData/Roaming/Positron/User
vscode|Library/Application Support/Trae*/User
vscode|.config/Trae*/User
vscode|AppData/Roaming/Trae*/User
vscode|.vscode*/argv.json
vscode|.vscode*/cli/token*.json
# Remote development servers (WSL, SSH, tunnels) keep the same User layout on the remote host
vscode|.vscode-server*/data/User
vscode|.vscode-server*/data/logs
vscode|.vscodium-server*/data/User
vscode|.positron-server/data/User
# VS Code family extension state. Each editor User directory is collected
# above; these nested entries claim the agent extension globalStorage from
# that walk so Cline, Roo, Kilo, Continue, Cody and Twinny files are
# attributed to them. Twinny also keeps per-workspace extension storage
# (agent-changes/) under workspaceStorage/<id>/rjmacarthy.twinny.
cline|.config/*/User/globalStorage/saoudrizwan.claude-dev
cline|Library/Application Support/*/User/globalStorage/saoudrizwan.claude-dev
cline|AppData/Roaming/*/User/globalStorage/saoudrizwan.claude-dev
roo-code|.config/*/User/globalStorage/rooveterinaryinc.roo-cline
roo-code|Library/Application Support/*/User/globalStorage/rooveterinaryinc.roo-cline
roo-code|AppData/Roaming/*/User/globalStorage/rooveterinaryinc.roo-cline
kilo-code|.config/*/User/globalStorage/kilocode.kilo-code
kilo-code|Library/Application Support/*/User/globalStorage/kilocode.kilo-code
kilo-code|AppData/Roaming/*/User/globalStorage/kilocode.kilo-code
continue|.config/*/User/globalStorage/continue.continue
continue|Library/Application Support/*/User/globalStorage/continue.continue
continue|AppData/Roaming/*/User/globalStorage/continue.continue
cody|.config/*/User/globalStorage/sourcegraph.cody-ai
cody|Library/Application Support/*/User/globalStorage/sourcegraph.cody-ai
cody|AppData/Roaming/*/User/globalStorage/sourcegraph.cody-ai
twinny|.config/*/User/globalStorage/rjmacarthy.twinny
twinny|Library/Application Support/*/User/globalStorage/rjmacarthy.twinny
twinny|AppData/Roaming/*/User/globalStorage/rjmacarthy.twinny
twinny|.config/*/User/workspaceStorage/*/rjmacarthy.twinny
twinny|Library/Application Support/*/User/workspaceStorage/*/rjmacarthy.twinny
twinny|AppData/Roaming/*/User/workspaceStorage/*/rjmacarthy.twinny
cline|.*-server*/data/User/globalStorage/saoudrizwan.claude-dev
roo-code|.*-server*/data/User/globalStorage/rooveterinaryinc.roo-cline
kilo-code|.*-server*/data/User/globalStorage/kilocode.kilo-code
continue|.*-server*/data/User/globalStorage/continue.continue
cody|.*-server*/data/User/globalStorage/sourcegraph.cody-ai
twinny|.*-server*/data/User/globalStorage/rjmacarthy.twinny
twinny|.*-server*/data/User/workspaceStorage/*/rjmacarthy.twinny
# Windsurf (rebranding to Devin)
windsurf|.codeium
windsurf|.windsurf
windsurf|.devin
windsurf|.config/devin
windsurf|.local/share/devin
# On a case-insensitive disk AppData/Roaming/devin (the Devin CLI) is the same
# directory as AppData/Roaming/Devin (the Devin IDE), so only the CLI files are named
windsurf|AppData/Roaming/devin/mcp_config.json
windsurf|AppData/Roaming/devin/credentials.toml
windsurf|Library/Application Support/Windsurf/User
windsurf|Library/Application Support/Windsurf/logs
windsurf|.config/Windsurf/User
windsurf|.config/Windsurf/logs
windsurf|AppData/Roaming/Windsurf/User
windsurf|AppData/Roaming/Windsurf/logs
windsurf|Library/Application Support/Devin/User
windsurf|Library/Application Support/Devin/logs
windsurf|.config/Devin/User
windsurf|.config/Devin/logs
windsurf|AppData/Roaming/Devin/User
windsurf|AppData/Roaming/Devin/logs
windsurf|.windsurf-server/data/User
windsurf|.devin-server/data/User
# Cline, Roo Code, Kilo Code
cline|.cline
cline|Documents/Cline
cline|Cline/Rules
roo-code|.roo
roo-code|.roo-code
roo-code|.vscode-mock
roo-code|.local/share/Roo-Code
roo-code|AppData/Roaming/Roo-Code
kilo-code|.kilo
kilo-code|.kilocode
kilo-code|.kilocodemodes
kilo-code|.config/kilo
kilo-code|.local/share/kilo
kilo-code|.local/state/kilo
kilo-code|.local/state/kilo-sandbox-policy
kilo-code|.local/state/kilo-sandbox-preference
# Kilo Code JetBrains plugin: its kilo folder in the IDE system directory, its settings files
kilo-code|Library/Caches/JetBrains/*/kilo
kilo-code|AppData/Local/JetBrains/*/kilo
kilo-code|.cache/JetBrains/*/kilo
kilo-code|Library/Application Support/JetBrains/*/options/kilo*.xml
kilo-code|AppData/Roaming/JetBrains/*/options/kilo*.xml
kilo-code|.config/JetBrains/*/options/kilo*.xml
# Continue
continue|.continue
# Aider (history files live in the repo; these are the home-level ones)
aider|.aider
aider|.aider.conf.yml
aider|.aider.model.settings.yml
aider|.aider.model.metadata.json
aider|.aider.chat.history.md
aider|.aider.input.history
aider|.aider.llm.history
# OpenCode (XDG roots with no platform branch: same paths on macOS and Windows)
opencode|.config/opencode
opencode|.opencode
opencode|.local/share/opencode
opencode|.local/state/opencode
opencode|Library/Application Support/ai.opencode.desktop*
opencode|AppData/Roaming/ai.opencode.desktop*
opencode|.config/ai.opencode.desktop*
opencode|AppData/Local/Temp/opencode
# Crush (~/.config on every OS; sessions live in each project .crush/crush.db)
crush|.config/crush
crush|.local/share/crush
crush|AppData/Local/crush
crush|.crush
crush|.cache/crush
# Goose (XDG on macOS too; Electron app dir is separate). On Windows Goose
# writes only the config, data and cache directories under Block/goose. The
# cache entry names the children of cache/: the PowerShell exclusion match
# ignores case, so */Goose/Cache would prune Block/goose/cache itself
goose|.config/goose
goose|.local/share/goose
goose|.local/state/goose
goose|.cache/goose
goose|.config/Goose
goose|Library/Application Support/Goose
goose|AppData/Roaming/Block/goose/config
goose|AppData/Roaming/Block/goose/data
goose|AppData/Local/Block/goose/cache/*
goose|Library/Application Support/Block/goose
goose|AppData/Roaming/Goose
goose|.goose
# Zed (config in ~/.config/zed on macOS too; data under Library)
zed|.config/zed
zed|.local/share/zed
zed|Library/Application Support/Zed
zed|Library/Logs/Zed
zed|AppData/Roaming/Zed
zed|AppData/Local/Zed
zed|.var/app/dev.zed.Zed/config/zed
zed|.var/app/dev.zed.Zed/data/zed
zed|.var/app/dev.zed.ZedPreview/config/zed
zed|.var/app/dev.zed.ZedPreview/data/zed
zed|.var/app/dev.zed.ZedNightly/config/zed
zed|.var/app/dev.zed.ZedNightly/data/zed
# Qwen Code
qwen-code|.qwen
# Amp (data dir is ~/.local/share/amp on every OS; threads are server-side)
amp|.config/amp
amp|.local/share/amp
amp|.cache/amp/logs
amp|.cache/amp/terminal
amp|.amp
amp|AppData/Local/amp/logs
# Factory Droid
factory-droid|.factory
# Meta Muse Code (XDG ~/.config/muse and ~/.local/share/muse on every OS; launcher state in its install dir)
muse-code|.config/muse
muse-code|.local/share/muse
muse-code|.muse
muse-code|.local/bin/.muse-*
muse-code|AppData/Local/Programs/muse
muse-code|AppData/LocalLow/muse-shell-sandbox-*
# Augment (auggie CLI; the VS Code extension state is under the vscode entries)
augment|.augment
# Kiro (CLI, legacy Amazon Q CLI, and the Kiro IDE which is a VS Code fork)
kiro|.kiro
kiro|.kiro-server/data/User
kiro|.aws/amazonq
kiro|.aws/sso/cache
kiro|.local/share/kiro-cli
kiro|.local/share/amazon-q
kiro|Library/Application Support/kiro-cli
kiro|Library/Application Support/amazon-q
kiro|AppData/Local/kiro-cli
kiro|AppData/Local/amazon-q
kiro|.config/Kiro/User
kiro|.config/Kiro/logs
kiro|Library/Application Support/Kiro/User
kiro|Library/Application Support/Kiro/logs
kiro|AppData/Roaming/Kiro/User
kiro|AppData/Roaming/Kiro/logs
# PearAI (VS Code fork; ~/.pearai is both the editor extension dir and the Continue fork home)
pearai|.pearai
pearai|.config/PearAI/User
pearai|.config/PearAI/logs
pearai|.config/pearai/User
pearai|Library/Application Support/PearAI/User
pearai|Library/Application Support/PearAI/logs
pearai|AppData/Roaming/PearAI/User
pearai|AppData/Roaming/PearAI/logs
pearai|.pearai-server/data/User
# Sourcegraph Cody (VS Code chat history and token are rows in state.vscdb; JetBrains and agent state under Cody-nodejs)
cody|.vscode/cody.json
cody|.cody
cody|.local/share/Cody-nodejs
cody|.config/Cody-nodejs
cody|.local/state/Cody-nodejs
cody|Library/Application Support/Cody-nodejs
cody|Library/Preferences/Cody-nodejs
cody|Library/Logs/Cody-nodejs
cody|AppData/Local/Cody-nodejs
cody|AppData/Roaming/Cody-nodejs
cody|.sourcegraph
cody|.config/JetBrains/*/options/cody*.xml
cody|Library/Application Support/JetBrains/*/options/cody*.xml
cody|AppData/Roaming/JetBrains/*/options/cody*.xml
# Twinny (chats are rows in state.vscdb; embeddings, node and server state under ~/.twinny)
twinny|.twinny
# Tabby (self-hosted server state, editor agent config, JetBrains plugin settings)
tabby|.tabby
tabby|.tabby-client
tabby|.config/JetBrains/*/options/intellij-tabby*.xml
tabby|Library/Application Support/JetBrains/*/options/intellij-tabby*.xml
tabby|AppData/Roaming/JetBrains/*/options/intellij-tabby*.xml
# Open Interpreter (a Codex CLI fork with the Codex layout under ~/.openinterpreter)
open-interpreter|.openinterpreter
open-interpreter|.config/open-interpreter
open-interpreter|.config/Open Interpreter*
open-interpreter|Library/Application Support/open-interpreter
open-interpreter|Library/Application Support/Open Interpreter*
open-interpreter|AppData/Local/open-interpreter
open-interpreter|AppData/Local/Open Interpreter*
open-interpreter|.cache/open-interpreter
# OpenHands (CLI and Agent Server under ~/.openhands; Agent Canvas desktop app)
openhands|.openhands
openhands|Library/Application Support/OpenHands Agent Canvas
openhands|.config/OpenHands Agent Canvas
openhands|AppData/Roaming/OpenHands Agent Canvas
# pi (same path on every OS); little-coder runs pi and its prompt history is a nested entry
pi|.pi
little-coder|.little-coder
little-coder|.config/little-coder
little-coder|.cache/little-coder
little-coder|.pi/agent/little-coder-prompt-history.json
# Letta Code, and the retired Letta (MemGPT) server
letta|.letta
letta|.config/letta
letta|.memgpt
# Hermes Agent (Nous Research; AppData/Local/hermes on Windows, Electron app dir Hermes)
# HERMES_DATA_DIR_SUFFIX is appended to the home and the Electron dir with no
# separator (.hermes-channel-build-canary), so ?* takes any suffix
hermes|.hermes
hermes|.hermes?*
hermes|AppData/Local/hermes
hermes|AppData/Local/hermes?*
hermes|.config/Hermes
hermes|.config/Hermes?*
hermes|Library/Application Support/Hermes
hermes|Library/Application Support/Hermes?*
hermes|AppData/Roaming/Hermes
hermes|AppData/Roaming/Hermes?*
# OpenClaw (formerly Clawdbot and Moltbot; profiles are ~/.openclaw-<profile>; ~/.clawdbot and ~/.moltbot are legacy state directories)
openclaw|.openclaw
openclaw|.openclaw-*
openclaw|.clawdbot
openclaw|.moltbot
openclaw|.config/openclaw
openclaw|Library/Application Support/OpenClaw
openclaw|Library/LaunchAgents/ai.openclaw.*
openclaw|.config/systemd/user/openclaw-*
openclaw|.config/systemd/user/clawdbot-*
# nanobot (instances are ~/.nanobot-<name>)
nanobot|.nanobot
nanobot|.nanobot-*
nanobot|Library/LaunchAgents/ai.nanobot.*
nanobot|.config/systemd/user/nanobot-*
# Agent Zero (state is under the install dir, not a dot dir: installer and launcher
# default ~/agent-zero/<instance>/usr, or a clone in ~/agent-zero or ~/Desktop/agent-zero)
agent-zero|agent-zero
agent-zero|Desktop/agent-zero
agent-zero|.agent-zero
agent-zero|.local/share/a0
agent-zero|Library/Application Support/A0
agent-zero|AppData/Local/A0
agent-zero|.config/Agent Zero Launcher
agent-zero|Library/Application Support/Agent Zero Launcher
agent-zero|AppData/Roaming/Agent Zero Launcher
# computer-use screen captures of the A0 CLI connector in the system temp
# directory, inside the home only on Windows
agent-zero|AppData/Local/Temp/_a0_connector
# ShellGPT (~/.config on every OS; chats default to the temp dir, under the home only on Windows)
shellgpt|.config/shell_gpt
shellgpt|AppData/Local/Temp/chat_cache
shellgpt|AppData/Local/Temp/shell_gpt
# Local Deep Research (platformdirs data dir; history is in an encrypted per-user database)
local-deep-research|.local/share/local-deep-research
local-deep-research|Library/Application Support/local-deep-research
local-deep-research|AppData/Local/local-deep-research
local-deep-research|Documents/LocalDeepResearch
# AutoGen Studio (~/.autogenstudio on every OS; the SQLite db settings table can hold API keys)
autogen-studio|.autogenstudio
# CrewAI (appdirs; run data dir is named after the project folder, so file-anchored lines)
crewai|.config/crewai
crewai|.crewai
crewai|.local/share/crewai
crewai|.local/share/*/latest_kickoff_task_outputs.db*
crewai|.local/share/*/flow_states.db*
crewai|.local/share/*/.crewai_user.json
crewai|Library/Application Support/crewai
crewai|Library/Application Support/*/latest_kickoff_task_outputs.db*
crewai|Library/Application Support/*/flow_states.db*
crewai|Library/Application Support/*/.crewai_user.json
crewai|AppData/Local/[Cc]rew[Aa][Ii]
# CAMEL (camel-ai; ~/.camel is shared with Apache Camel JBang, so only CAMEL-specific names)
camel-ai|.camel/gmail_token.json
camel-ai|.camel/skills
camel-ai|.config/camel
# Dify (server state is in bind mounts beside the compose file; difyctl client config)
dify|.config/difyctl
dify|AppData/Roaming/difyctl
dify|AppData/Local/difyctl
# Flowise (~/.flowise on every OS; the README compose bind-mounts it)
flowise|.flowise
# Langflow (platformdirs cache dir; Desktop paths from docs)
langflow|.cache/langflow
langflow|Library/Caches/langflow
langflow|AppData/Local/langflow/langflow/Cache
langflow|.langflow
langflow|Library/Application Support/com.LangflowDesktop
langflow|Library/Logs/com.LangflowDesktop
langflow|AppData/Roaming/com.LangflowDesktop
langflow|AppData/Local/com.LangflowDesktop
# MetaGPT (~/.metagpt on every OS; config2.yaml holds the LLM API key)
metagpt|.metagpt
# n8n (~/.n8n on every OS; computer-use gateway logs every tool call)
n8n|.n8n
n8n|.n8n-local-gateway
n8n|.n8n-gateway
# Pydantic AI CLIs: clai prompt history and the CLAI 2.0 coding agent (~/.config and ~/.local/state on every OS)
pydantic-clai|.pydantic-ai
pydantic-clai|.config/pydantic-clai2
pydantic-clai|.local/state/pydantic-clai2
# Shared cross-agent directories (skills and instructions read by several agents)
shared|.agents
shared|.config/AGENTS.md
shared|.config/agents
# GitHub CLI login that agents use for source-host access (gh auth login):
# the token in plaintext when the login keychain is unreachable, a token-less
# host entry when it is in the keychain
shared|.config/gh/hosts.yml
shared|AppData/Roaming/GitHub CLI/hosts.yml
# Ollama
ollama|.ollama
ollama|Library/Application Support/Ollama
ollama|AppData/Local/Ollama
'@

$PROJECT_CATALOG = @'
claude-code|.claude
claude-code|CLAUDE.md
claude-code|CLAUDE.local.md
project|.mcp.json
project|AGENTS.md
project|AGENTS.override.md
project|AGENT.md
project|.agents
project|.git/HEAD
project|.git/config
project|.git/FETCH_HEAD
project|.git/packed-refs
project|.git/refs
project|.git/logs
project|.git/worktrees
project|.git/info/exclude
gemini-cli|GEMINI.md
gemini-cli|.gemini
gemini-cli|.geminiignore
qwen-code|QWEN.md
qwen-code|.qwen
qwen-code|.qwenignore
codex-cli|.codex
cursor|.cursor
cursor|.cursorrules
cursor|.cursorignore
cursor|.cursorindexingignore
windsurf|.windsurf
windsurf|.windsurfrules
windsurf|.windsurfignore
windsurf|.codeiumignore
windsurf|.devin
windsurf|.devinignore
antigravity|.antigravityignore
cline|.clinerules
cline|.clineignore
cline|.cline
roo-code|.roo
roo-code|.roomodes
roo-code|.rooignore
roo-code|.roorules*
roo-code|.rooprotected
kilo-code|.kilo
kilo-code|.kilocode
kilo-code|.kilocodemodes
kilo-code|.kilocodeignore
kilo-code|.kilocoderules
kilo-code|kilo.json*
continue|.continue
continue|.continuerc.json
continue|.continueignore
aider|.aider.chat.history.md
aider|.aider.input.history
aider|.aider.llm.history
aider|.aider.conf.yml
aider|.aider.model.settings.yml
aider|.aider.model.metadata.json
aider|.aiderignore
opencode|.opencode
opencode|opencode.json*
crush|.crush
crush|crush.json
crush|.crush.json
crush|crushrc
crush|.crushrc
crush|.crushignore
crush|CRUSH.md
crush|CRUSH.local.md
crush|crush.md
crush|crush.local.md
crush|Crush.md
crush|Crush.local.md
goose|.goosehints
goose|.goose
goose|diagnostics_*.json
zed|.zed
zed|.rules
project|.github/copilot-instructions.md
project|.github/git-commit-instructions.md
copilot-cli|.github/copilot
project|.github/instructions
copilot-cli|.github/mcp.json
copilot-cli|.github/hooks
copilot-cli|.github/skills
copilot-cli|.github/agents
copilot-cli|.github/lsp.json
vscode|.vscode/mcp.json
amp|.amp
factory-droid|.factory
factory-droid|.factory-plugin
factory-droid|.droid.yaml
muse-code|.muse
augment|.augment
augment|.augment-guidelines
augment|.augmentignore
augment|.augment-plugin
kiro|.kiro
kiro|.kiroignore
kiro|.amazonq
kiro|AmazonQ.md
pearai|.pearairc.json
pearai|.pearaiignore
pearai|.pearai-agent
pearai|.pearai-agent-ignore
pearai|.pearai-agent-modes
cody|.cody
cody|.vscode/cody.json
cody|.sourcegraph
cody|.idea/cody_history.xml
cody|.idea/.sourcegraph
open-interpreter|.openinterpreter
open-interpreter|.codewhale
openhands|.openhands
pi|.pi
little-coder|.pi/approved-plan.md
pi|pi-session-*.html
letta|.letta
pydantic-clai|.clai
letta|.skills
crewai|.crewai
hermes|.hermes.md
hermes|HERMES.md
hermes|.hermes
hermes|trajectory_samples.jsonl
hermes|failed_trajectories.jsonl
openclaw|.openclaw
project|SOUL.md
openclaw|IDENTITY.md
project|USER.md
openclaw|TOOLS.md
openclaw|BOOTSTRAP.md
openclaw|MEMORY.md
nanobot|.nanobot
nanobot|HEARTBEAT.md
nanobot|memory/MEMORY.md
nanobot|memory/history.jsonl
nanobot|memory/HISTORY.md
nanobot|cron/jobs.json
nanobot|cron/runs
nanobot|triggers/triggers.json
nanobot|triggers/runs
'@

$EXCLUDES = @'
.claude/cache
.claude/plugins/marketplaces
.claude/plugins/cache
.claude/plugins/plugin-directory-cache*
.claude/plugins/*/node_modules
.claude/local/node_modules
.claude/image-cache
.claude/chrome
.claude/worktrees
.claude/checkpoints
.local/share/claude/versions
*/Claude*/Cache
*/Claude*/Code Cache
*/Claude*/GPUCache
*/Claude*/DawnGraphiteCache
*/Claude*/DawnWebGPUCache
*/Claude*/claude-code
*/Claude*/claude-code-vm
*/Claude*/vm_bundles
*/Claude*/sentry
.gemini/whisper_models
.gemini/bin
.gemini/tmp/bin
.gemini/extensions/*/node_modules
.gemini/extensions/*/.git
.gemini/cli-browser-profile
.gemini/worktrees
.gemini/antigravity/bin
.gemini/antigravity-cli/bin
.gemini/antigravity-ide/bin
.gemini/antigravity-backup/bin
.gemini/antigravity*/plugin_data/*/bin
.gemini/antigravity*/cache/marketplaces
.gemini/antigravity*/brain/*/.system_generated/worktrees
.gemini/antigravity*/lib/node_modules
.gemini/config/node_modules
*/Antigravity/Code Cache
*/Antigravity/GPUCache
*/Antigravity/DawnGraphiteCache
*/Antigravity/DawnWebGPUCache
*/Antigravity/blob_storage
*/Antigravity/Shared Dictionary
*/Antigravity/CachedData
*/Antigravity/Crashpad
*/Antigravity/Dictionaries
.config/Antigravity/Cache
Library/Application Support/Antigravity/Cache
AppData/Roaming/Antigravity/Cache
.antigravity/extensions
AppData/Local/agy
.codex/packages
.codex/cache
.codex/.tmp
.codex/plugins/cache
.codex/plugins/.plugin-appserver
.codex/plugins/.remote-plugin-install-staging
.codex/computer-use/*.app
.codex/skills/.system
.codex/worktrees
.codex/.sandbox
.codex/.sandbox-bin
.codex/tmp
.codex/visualizations
*/Roaming/ChatGPT/Cache
*/Roaming/ChatGPT/Code Cache
*/Roaming/ChatGPT/GPUCache
*/Roaming/ChatGPT/DawnGraphiteCache
*/Roaming/ChatGPT/DawnWebGPUCache
*/Roaming/Codex/web/Codex/Default/Cache
*/Roaming/Codex/web/Codex/Default/Code Cache
*/Roaming/Codex/web/Codex/Default/GPUCache
*/Roaming/Codex/web/Codex/Default/DawnGraphiteCache
*/Roaming/Codex/web/Codex/Default/DawnWebGPUCache
*/Roaming/Codex/web/Codex/Default/blob_storage
*/Roaming/Codex/web/Codex/Default/Extensions
*/Roaming/Codex/web/Codex/Default/Service Worker/CacheStorage
*/Roaming/Codex/web/Codex/Default/Service Worker/ScriptCache
*/Roaming/Codex/web/Codex/Default/Partitions/*/Cache
*/Roaming/Codex/web/Codex/Default/Partitions/*/Code Cache
*/Roaming/Codex/web/Codex/Default/Partitions/*/GPUCache
*/Roaming/Codex/web/Codex/Default/Partitions/*/DawnGraphiteCache
*/Roaming/Codex/web/Codex/Default/Partitions/*/DawnWebGPUCache
*/Roaming/Codex/web/Codex/Default/Partitions/*/blob_storage
*/Roaming/Codex/web/Codex/Default/Partitions/*/Extensions
*/Roaming/Codex/web/Codex/Default/Partitions/*/Service Worker/CacheStorage
*/Roaming/Codex/web/Codex/Default/Partitions/*/Service Worker/ScriptCache
Library/Application Support/Codex/Default/Cache
Library/Application Support/Codex/Default/Code Cache
Library/Application Support/Codex/Default/GPUCache
Library/Application Support/Codex/Default/DawnGraphiteCache
Library/Application Support/Codex/Default/DawnWebGPUCache
Library/Application Support/Codex/Default/blob_storage
Library/Application Support/Codex/Default/Extensions
Library/Application Support/Codex/Default/Service Worker/CacheStorage
Library/Application Support/Codex/Default/Service Worker/ScriptCache
Library/Application Support/Codex/Default/Partitions/*/Cache
Library/Application Support/Codex/Default/Partitions/*/Code Cache
Library/Application Support/Codex/Default/Partitions/*/GPUCache
Library/Application Support/Codex/Default/Partitions/*/DawnGraphiteCache
Library/Application Support/Codex/Default/Partitions/*/DawnWebGPUCache
Library/Application Support/Codex/Default/Partitions/*/blob_storage
Library/Application Support/Codex/Default/Partitions/*/Extensions
Library/Application Support/Codex/Default/Partitions/*/Service Worker/CacheStorage
Library/Application Support/Codex/Default/Partitions/*/Service Worker/ScriptCache
Library/Application Support/Codex/executor-plugins/*/server.mjs
.copilot/pkg
.copilot/installed-plugins
.cache/copilot/pkg
Library/Caches/copilot/pkg
AppData/Local/copilot/pkg
.cache/copilot/marketplaces
Library/Caches/copilot/marketplaces
AppData/Local/copilot/marketplaces
.copilot/repos
.copilot/repos/*/.git/objects
.copilot/repos/*/.git/index
.copilot/chats/*/node_modules
.local/share/com.github.githubapp/app-skills
.local/share/com.github.githubapp/WebKitCache
.local/share/com.github.githubapp/CacheStorage
Library/Application Support/com.github.githubapp/app-skills
AppData/Roaming/com.github.githubapp/app-skills
AppData/Local/com.github.githubapp/EBWebView/Default/Cache
AppData/Local/com.github.githubapp/EBWebView/Default/Code Cache
AppData/Local/com.github.githubapp/EBWebView/Default/GPUCache
AppData/Local/com.github.githubapp/EBWebView/Default/DawnGraphiteCache
AppData/Local/com.github.githubapp/EBWebView/Default/DawnWebGPUCache
AppData/Local/com.github.githubapp/EBWebView/Default/blob_storage
AppData/Local/com.github.githubapp/EBWebView/Default/Shared Dictionary
AppData/Local/com.github.githubapp/EBWebView/GrShaderCache
AppData/Local/com.github.githubapp/EBWebView/ShaderCache
AppData/Local/com.github.githubapp/EBWebView/GPUPersistentCache
AppData/Local/com.github.githubapp/EBWebView/component_crx_cache
AppData/Local/com.github.githubapp/EBWebView/extensions_crx_cache
AppData/Local/com.github.githubapp/EBWebView/WidevineCdm
AppData/Local/com.github.githubapp/EBWebView/Subresource Filter
AppData/Local/com.github.githubapp/EBWebView/hyphen-data
AppData/Local/com.github.githubapp/EBWebView/PKIMetadata
AppData/Local/com.github.githubapp/EBWebView/CertificateRevocation
AppData/Local/com.github.githubapp/EBWebView/BrowserMetrics-spare.pma
.cursor/extensions
.cursor/worktrees
.cursor/plugins/marketplaces
.cursor/plugins/cache
.cursor-server/bin
.cursor-server/extensions
*/Cursor/User/globalStorage/state.vscdb.backup
*/User/globalStorage/anysphere.cursor-agent-worker/agent-cli
*/Cursor/update-supervisor/*/cursor-update-supervisor
*/Cursor/update-supervisor/*/inno_updater.exe
.cursor/projects/*/canvases/node_modules
.cursor/statsig-cache.json
.codeium/windsurf/implicit
.codeium/*/bin
.codeium/bin
.codeium/ws-browser*
.windsurf/extensions
.devin/extensions
*/User/globalStorage/saoudrizwan.claude-dev/checkpoints
*/User/globalStorage/rooveterinaryinc.roo-cline/checkpoints
*/User/globalStorage/rooveterinaryinc.roo-cline/tasks/*/checkpoints
*/User/globalStorage/rooveterinaryinc.roo-cline/cache
*/User/globalStorage/kilocode.kilo-code/checkpoints
*/User/globalStorage/kilocode.kilo-code/cache
*/User/globalStorage/ms-*
*/User/globalStorage/vscjava*
*/User/globalStorage/redhat*
*/User/globalStorage/eamodio*
*/User/globalStorage/golang*
*/User/globalStorage/rust-lang*
*/User/globalStorage/yzane.markdown-pdf
*/User/globalStorage/github.copilot-chat/logContextRecordings
*/User/globalStorage/github.copilot-chat/workspaceRecordings
*/User/workspaceStorage/*/GitHub.copilot-chat/codebase-external.sqlite*
*/User/workspaceStorage/*/ms-*
*/User/workspaceStorage/*/vscjava*
*/User/workspaceStorage/*/redhat*
*/User/workspaceStorage/*/rust-lang*
.cline/data/checkpoint-scratch
.cline/data/workspaces/chat
.cline/data/agent-plugins
.cline/plugins
.cline/worktrees
.vscode-mock/global-storage/tasks/*/checkpoints
.vscode-mock/global-storage/checkpoints
.roo/worktrees
.roo-code/mcp
.local/share/Roo-Code/MCP
AppData/Roaming/Roo-Code/MCP
.local/share/kilo/snapshot
.local/share/kilo/repos
.local/share/kilo/tool-output
.local/share/kilo/worktree
.local/state/kilo/indexing
.local/share/kilo/state/indexing
.kilo/worktrees
.kilo/bin
Library/Caches/JetBrains/*/kilo/cli
Library/Caches/JetBrains/*/kilo/repo-cli
AppData/Local/JetBrains/*/kilo/cli
AppData/Local/JetBrains/*/kilo/repo-cli
.cache/JetBrains/*/kilo/cli
.cache/JetBrains/*/kilo/repo-cli
.continue/index/lancedb
.continue/index/*.sqlite
.continue/.utils
.continue/node_modules
.continue/out
.continue/types
.continue/.diffs
.continue/.migrations
.continue/dev_data/devdata.sqlite
.aider/caches
.aider.tags.cache*
.local/share/opencode/bin
.local/share/opencode/snapshot
.local/share/opencode/worktree
.local/share/opencode/repos
.local/share/opencode/tool-output
.config/opencode/node_modules
.opencode/bin
.opencode/node_modules
Library/Application Support/ai.opencode.desktop*/compile-cache
AppData/Roaming/ai.opencode.desktop*/compile-cache
.config/ai.opencode.desktop*/compile-cache
Library/Application Support/ai.opencode.desktop*/cli
AppData/Roaming/ai.opencode.desktop*/cli
.config/ai.opencode.desktop*/cli
.local/share/goose/models
.local/share/goose/model_catalog
.local/state/goose/codex
.config/goose/mcp-apps-cache
AppData/Roaming/Block/goose/data/models
AppData/Roaming/Block/goose/data/model_catalog
AppData/Roaming/Block/goose/data/codex
AppData/Roaming/Block/goose/config/mcp-apps-cache
*/Goose/Cache
*/Goose/Code Cache
*/Goose/GPUCache
*/Goose/DawnGraphiteCache
*/Goose/DawnWebGPUCache
*/Goose/blob_storage
*/[Zz]ed/extensions
*/[Zz]ed/remote_extensions
*/[Zz]ed/node
*/[Zz]ed/languages
*/[Zz]ed/copilot
*/[Zz]ed/debug_adapters
*/[Zz]ed/external_agents
*/[Zz]ed/prettier
*/[Zz]ed/remote_servers
*/[Zz]ed/devcontainer
*/[Zz]ed/embeddings
*/[Zz]ed/server_state
*/[Zz]ed/hang_traces
*/[Zz]ed/build_timings
.qwen/updates
.qwen/extension-store
.qwen/extensions/*/node_modules
.qwen/bin
.qwen/sandbox
.qwen/scratch-workspaces
.qwen/locales
.qwen/arena
.qwen/artifacts
.qwen/resources
.qwen/startup-perf
.qwen/worktrees
.amp/bin
.factory/updates
.factory/cache/sounds
.factory/bin
.factory/tools
.factory/software-factory/*/*/repos
.factory/software-factory/*/*/worktrees
.factory/temp/[!e]*
.factory/worktrees
.factory/snapshots
.factory/generated-images
.local/share/muse/plugins/cache/builtin
.local/share/muse/skills/bundled
.local/share/muse/plugins/marketplaces
.local/bin/.muse-update.*
AppData/Local/Programs/muse/muse-bin-*.exe
AppData/Local/Programs/muse/.muse-update.*.exe
.muse/worktrees
.augment/binaries
.augment/vfs
.augment/knowledgebase
.augment/uploads
.augment/worktrees
.augment/plugins/marketplaces
.aws/amazonq/cli-checkouts
.aws/amazonq/knowledge_bases
.kiro/extensions
.kiro/cloud-cache
.kiro/sandbox-state
.kiro/sandbox-curl
*/User/globalStorage/kiro.kiroagent/*lance*
*/User/globalStorage/kiro.kiroagent/index
.pearai/extensions
.pearai/index/lancedb
.pearai/index/*.sqlite
.pearai/types
.pearai/out
.pearai/node_modules
.pearai/.diffs
.pearai/.migrations
.pearai/dev_data/devdata.sqlite
*/User/globalStorage/pearai.pearai-roo-cline/checkpoints
*/User/globalStorage/pearai.pearai-roo-cline/tasks/*/checkpoints
*/User/globalStorage/pearai.pearai-roo-cline/cache
*/globalStorage/sourcegraph.cody-ai/symf/symf-*
*/Cody-nodejs/symf/symf-*
*/Cody-nodejs/Data/symf/symf-*
*/Cody-nodejs/dist
*/Cody-nodejs/Config/dist
.twinny/embeddings/*/chunks.lance
.twinny/server/plugins/*/repos/*/checkout
.twinny/server/plugins/*/repos/*/index
.tabby/models
.tabby/index
.tabby/repositories
.openinterpreter/packages
.openinterpreter/cache
.openinterpreter/.tmp
.openinterpreter/tmp
.openinterpreter/plugins/cache
.openinterpreter/skills/.system
.openinterpreter/worktrees
.openinterpreter/.sandbox
.openinterpreter/.sandbox-bin
.openinterpreter/visualizations
.openinterpreter/models-cache
.config/open-interpreter/models
Library/Application Support/open-interpreter/models
AppData/Local/open-interpreter/open-interpreter/models
.openhands/cache
.openhands/agent-canvas/workspaces
.openhands/agent-canvas/tmux
.openhands/workspaces
.pi/agent/bin
.pi/agent/tools/fd*
.pi/agent/tools/rg*
.pi/agent/tmp
.pi/agent/npm/node_modules
.pi/agent/git/*/node_modules
.pi/agent/git/*/.git
.pi/npm/node_modules
.pi/git/*/node_modules
.pi/git/*/.git
.pi/server/*.sock
.letta/bin
.letta/mod-cache
.letta/tool_execution_dir
.letta/chroma
.memgpt/chroma
.hermes*/hermes-agent/[!.]*
.hermes*/hermes-agent/.[!e]*
AppData/Local/hermes*/hermes-agent/[!.]*
AppData/Local/hermes*/hermes-agent/.[!e]*
.hermes*/models
.hermes*/runtimes
.hermes*/node
.hermes*/installs
.hermes*/tools
.hermes*/sandboxes
.hermes*/checkpoints/store/objects
.hermes*/checkpoints/store/indexes
.hermes*/checkpoints/legacy-*
.hermes*/backups
.hermes*/browser-profile
.hermes*/browser-profiles
.hermes*/browser_profiles
.hermes*/chrome-debug
.hermes*/bot-desktop/browser-profile
.hermes*/plugins/*/node_modules
.hermes*/plugins/*/.venv
AppData/Local/hermes*/models
AppData/Local/hermes*/runtimes
AppData/Local/hermes*/node
AppData/Local/hermes*/installs
AppData/Local/hermes*/tools
AppData/Local/hermes*/sandboxes
AppData/Local/hermes*/checkpoints/store/objects
AppData/Local/hermes*/checkpoints/store/indexes
AppData/Local/hermes*/checkpoints/legacy-*
AppData/Local/hermes*/backups
AppData/Local/hermes*/browser-profile
AppData/Local/hermes*/browser-profiles
AppData/Local/hermes*/browser_profiles
AppData/Local/hermes*/chrome-debug
AppData/Local/hermes*/bot-desktop/browser-profile
AppData/Local/hermes*/plugins/*/node_modules
AppData/Local/hermes*/plugins/*/.venv
AppData/Local/hermes-updater
.config/Hermes*/Cache
.config/Hermes*/Code Cache
.config/Hermes*/GPUCache
Library/Application Support/Hermes*/Cache
Library/Application Support/Hermes*/Code Cache
Library/Application Support/Hermes*/GPUCache
AppData/Roaming/Hermes*/Cache
AppData/Roaming/Hermes*/Code Cache
AppData/Roaming/Hermes*/GPUCache
.openclaw/dev
.openclaw/git
.openclaw/npm
.openclaw/npm-runtime
.openclaw/tmp
.openclaw/tools
.openclaw/worktrees
.openclaw/projects/[0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f]
.openclaw/plugin-skills
.openclaw/extensions/*/node_modules
.openclaw/sandbox/skills-workspaces
.openclaw/cache/control-ui-assets
.openclaw/agents/*/agent/tmp
.openclaw/agents/*/agent/.tmp
.openclaw/agents/*/agent/*/tmp
.openclaw/agents/*/agent/*/.tmp
.openclaw/browser/*/user-data/*/Cache
.openclaw/browser/*/user-data/*/Code Cache
.openclaw/browser/*/user-data/*/GPUCache
.openclaw/browser/*/user-data/*/Service Worker
.openclaw-*/projects/[0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f]
.openclaw-*/plugin-skills
.openclaw-*/extensions/*/node_modules
.openclaw-*/sandbox/skills-workspaces
.openclaw-*/cache/control-ui-assets
.openclaw-*/agents/*/agent/tmp
.openclaw-*/agents/*/agent/.tmp
.openclaw-*/agents/*/agent/*/tmp
.openclaw-*/agents/*/agent/*/.tmp
.openclaw-*/browser/*/user-data/*/Cache
.openclaw-*/browser/*/user-data/*/Code Cache
.openclaw-*/browser/*/user-data/*/GPUCache
.openclaw-*/browser/*/user-data/*/Service Worker
.clawdbot/dev
.clawdbot/git
.clawdbot/npm
.clawdbot/npm-runtime
.clawdbot/tmp
.clawdbot/tools
.clawdbot/worktrees
.clawdbot/projects/[0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f]
.clawdbot/plugin-skills
.clawdbot/extensions/*/node_modules
.clawdbot/sandbox/skills-workspaces
.clawdbot/cache/control-ui-assets
.clawdbot/agents/*/agent/tmp
.clawdbot/agents/*/agent/.tmp
.clawdbot/agents/*/agent/*/tmp
.clawdbot/agents/*/agent/*/.tmp
.clawdbot/browser/*/user-data/*/Cache
.clawdbot/browser/*/user-data/*/Code Cache
.clawdbot/browser/*/user-data/*/GPUCache
.clawdbot/browser/*/user-data/*/Service Worker
.moltbot/dev
.moltbot/git
.moltbot/npm
.moltbot/npm-runtime
.moltbot/tmp
.moltbot/tools
.moltbot/worktrees
.moltbot/projects/[0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f]
.moltbot/plugin-skills
.moltbot/extensions/*/node_modules
.moltbot/sandbox/skills-workspaces
.moltbot/cache/control-ui-assets
.moltbot/agents/*/agent/tmp
.moltbot/agents/*/agent/.tmp
.moltbot/agents/*/agent/*/tmp
.moltbot/agents/*/agent/*/.tmp
.moltbot/browser/*/user-data/*/Cache
.moltbot/browser/*/user-data/*/Code Cache
.moltbot/browser/*/user-data/*/GPUCache
.moltbot/browser/*/user-data/*/Service Worker
.nanobot*/bin
.nanobot*/run
.nanobot*/cache
agent-zero*/.venv
Desktop/agent-zero*/.venv
agent-zero*/.conda
Desktop/agent-zero*/.conda
agent-zero*/.git
Desktop/agent-zero*/.git
agent-zero*/tmp/playwright
Desktop/agent-zero*/tmp/playwright
agent-zero*/tmp/memory/embeddings
Desktop/agent-zero*/tmp/memory/embeddings
agent-zero*/usr/.time_travel
Desktop/agent-zero*/usr/.time_travel
agent-zero*/usr/workdir/*/.venv
Desktop/agent-zero*/usr/workdir/*/.venv
agent-zero*/usr/workdir/*/node_modules
Desktop/agent-zero*/usr/workdir/*/node_modules
.local/share/a0/browser-profiles
Library/Application Support/A0/Browser Profiles
AppData/Local/A0/Browser Profiles
*/Agent Zero Launcher/Cache
*/Agent Zero Launcher/Code Cache
*/Agent Zero Launcher/GPUCache
*/Agent Zero Launcher/docker_manager/cache
.local/share/local-deep-research/cache
.local/share/local-deep-research/models
.local/share/local-deep-research/journal_data
.local/share/local-deep-research/library
Library/Application Support/local-deep-research/cache
Library/Application Support/local-deep-research/models
Library/Application Support/local-deep-research/journal_data
Library/Application Support/local-deep-research/library
AppData/Local/local-deep-research/local-deep-research/cache
AppData/Local/local-deep-research/local-deep-research/models
AppData/Local/local-deep-research/local-deep-research/journal_data
AppData/Local/local-deep-research/local-deep-research/library
Documents/LocalDeepResearch/Library
.flowise/vectorstore
.langflow/knowledge_bases
AppData/Roaming/com.LangflowDesktop/knowledge_bases
Library/Application Support/com.LangflowDesktop/venv
Library/Application Support/com.LangflowDesktop/python_env
AppData/Local/com.LangflowDesktop/venv
AppData/Local/com.LangflowDesktop/python_env
AppData/Local/com.LangflowDesktop/cache
.cache/langflow/profile_pictures
Library/Caches/langflow/profile_pictures
AppData/Local/langflow/langflow/Cache/profile_pictures
.n8n/nodes/node_modules
.n8n/custom/node_modules
.n8n/node-definitions
.n8n/n8n-sdk-templates
.local/state/pydantic-clai2/workspaces/clones
.ollama/models/blobs
AppData/Local/Ollama/updates
.git/worktrees/*/index
.git/worktrees/*/modules
.git/worktrees/*/index.lock
.git/worktrees/*/sharedindex.*
.time_travel
usr/.time_travel
workdir/*/.venv
workdir/*/node_modules
usr/workdir/*/.venv
usr/workdir/*/node_modules
tmp/playwright
tmp/memory/embeddings
models
index
repositories
journal_data
library
agent-canvas/workspaces
agent-canvas/tmux
hermes-agent/[!.]*
hermes-agent/.[!e]*
runtimes
node
installs
tools
sandboxes
checkpoints/store/objects
checkpoints/store/indexes
checkpoints/legacy-*
backups
browser-profile
browser-profiles
browser_profiles
chrome-debug
bot-desktop/browser-profile
plugins/*/node_modules
plugins/*/.venv
bin
mod-cache
tool_execution_dir
chroma
profile_pictures
nodes/node_modules
node-definitions
n8n-sdk-templates
'@

$SECRET_GLOBS = @'
.claude/.credentials.json
.claude/.device-keys.json
.claude/hfi-auth.json
.claude/.session_ingress_token
.claude/remote/.oauth_token
.claude/remote/.api_key
.claude/sessions/*.key
.claude/daemon/control.key
.claude/daemon/pipe.key
.claude/daemon/roster.json
.claude/backups/.claude.json.*
.claude/ide/*.lock
.claude/bridge-spawn/*/mcp.json
.claude.json
.claude.json.backup*
.claude/settings.json
.claude/settings.local.json
.mcp.json
.claude-*/.credentials.json
.claude-*/hfi-auth.json
.claude-*/.session_ingress_token
.claude-*/remote/.oauth_token
.claude-*/remote/.api_key
.claude-*/sessions/*.key
.claude-*/daemon/control.key
.claude-*/daemon/pipe.key
.claude-*/daemon/roster.json
.claude-*/backups/.claude.json.*
.claude-*/ide/*.lock
.claude-*/bridge-spawn/*/mcp.json
.claude-*/.claude.json
.claude-*/.claude.json.backup*
Library/Application Support/Claude*/config.json
Library/Application Support/Claude*/Cookies*
Library/Application Support/Claude*/host-creds-*.json
Library/Application Support/Claude*/ccd-session-secrets/*
.config/Claude*/config.json
.config/Claude*/Cookies*
.config/Claude*/host-creds-*.json
.config/Claude*/ccd-session-secrets/*
AppData/Roaming/Claude*/config.json
AppData/Roaming/Claude*/Cookies*
AppData/Roaming/Claude*/host-creds-*.json
AppData/Roaming/Claude*/ccd-session-secrets/*
AppData/Local/Claude*/config.json
AppData/Local/Claude*/Cookies*
AppData/Local/Claude*/host-creds-*.json
AppData/Local/Claude*/ccd-session-secrets/*
AppData/Local/Packages/Claude_*/LocalCache/Roaming/Claude*/config.json
AppData/Local/Packages/Claude_*/LocalCache/Roaming/Claude*/Cookies*
AppData/Local/Packages/Claude_*/LocalCache/Roaming/Claude*/host-creds-*.json
AppData/Local/Packages/Claude_*/LocalCache/Roaming/Claude*/ccd-session-secrets/*
AppData/Local/Packages/AnthropicPBC.Claude_*/LocalCache/Roaming/Claude*/config.json
AppData/Local/Packages/AnthropicPBC.Claude_*/LocalCache/Roaming/Claude*/Cookies*
AppData/Local/Packages/AnthropicPBC.Claude_*/LocalCache/Roaming/Claude*/host-creds-*.json
AppData/Local/Packages/AnthropicPBC.Claude_*/LocalCache/Roaming/Claude*/ccd-session-secrets/*
*/Claude*/Local State
*/Claude*/claude_desktop_config.json
*/Claude*/local-agent-mode-sessions/*/.audit-key
*/Claude*/spaces-present/*/link-key
*.gemini/oauth_creds.json
*.gemini/gemini-credentials.json
*.gemini/mcp-oauth-tokens.json
*.gemini/a2a-oauth-tokens.json
*.gemini/.env
*.gemini/settings.json
*.gemini/extensions/*/.env
*.gemini/tmp/*/bug-memory-*.heapsnapshot
*.gemini/antigravity-cli/antigravity-oauth-token
*.gemini/antigravity*/plugin_data/*/token.json
.gemini/config/mcp_config.json
.config/[Aa]ntigravity/plugin-secrets.json
Library/Application Support/[Aa]ntigravity/plugin-secrets.json
AppData/Roaming/[Aa]ntigravity/plugin-secrets.json
.qwen/oauth_creds.json
.qwen/mcp-oauth-tokens*.json
.qwen/extension-secrets-v1.json
.qwen/.env
.qwen/settings.json
.qwen/extensions/*/.qwen-extension-git-credentials.json
.qwen/channels/*-credentials.json
.qwen/channels/weixin/account.json
.qwen/sessions/*.json
.codex/auth.json
.codex/.credentials.json
.codex/secrets/*
.codex/.sandbox-secrets/*
.codex/config.toml
.codex/.env
.codex/.codex-global-state.json*
.codex/shell_snapshots/*
*/Roaming/ChatGPT/Network/Cookies*
*/Roaming/ChatGPT/Local State
*/Roaming/Codex/web/Codex/Default/Network/Cookies*
*/Roaming/Codex/web/Codex/Default/Partitions/*/Network/Cookies*
*/Roaming/Codex/web/Codex/Default/Login Data*
*/Roaming/Codex/web/Codex/Default/Partitions/*/Login Data*
*/Roaming/Codex/web/Codex/Local State
Library/Application Support/Codex/Default/Cookies*
Library/Application Support/Codex/Default/Partitions/*/Cookies*
Library/Application Support/Codex/Default/Login Data*
Library/Application Support/Codex/Default/Partitions/*/Login Data*
Library/HTTPStorages/com.openai.codex.binarycookies
Library/HTTPStorages/com.openai.sky.CUAService.binarycookies
.config/github-copilot/*.json
AppData/Local/github-copilot/*.json
.copilot/config.json
.copilot/mcp-oauth-config*
.copilot/mcp-config.json
.copilot/run/*.token
.copilot/settings.json
.copilot/mcp-secrets/*
.config/.copilot/config.json
.config/.copilot/mcp-config*
.config/.copilot/mcp-oauth-config*
.github/copilot/settings.json
.github/copilot/settings.local.json
.github/mcp.json
AppData/Local/com.github.githubapp/EBWebView/Default/Network/Cookies*
AppData/Local/com.github.githubapp/EBWebView/Default/Login Data*
AppData/Local/com.github.githubapp/EBWebView/Local State
.cursor/auth.json
.config/cursor/auth.json
AppData/Roaming/Cursor/auth.json
.cursor/mcp.json
.cursor/projects/*/mcp-auth.json
.vscode*/cli/token*.json
*/User/mcp.json
*/User/profiles/*/mcp.json
.vscode/mcp.json
.codeium/config.json
.codeium/mcp_config.json
.codeium/windsurf/mcp_config.json
.config/devin/mcp_config.json
AppData/Roaming/devin/mcp_config.json
.local/share/devin/credentials.toml
AppData/Roaming/devin/credentials.toml
.cline/data/settings/providers.json
.cline/data/settings/cline_mcp_settings.json
.cline/data/settings/composio/*.json
.cline/data/secrets.json
.cline/data/connectors/settings.json
.cline/data/db/connectors.db*
*/User/globalStorage/saoudrizwan.claude-dev/cache/remote_config_*.json
*/User/globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json
.config/gh/hosts.yml
AppData/Roaming/GitHub CLI/hosts.yml
Documents/Cline/webhook_config.json
.vscode-mock/global-storage/secrets.json
.roo/cli-credentials.json
*/User/globalStorage/rooveterinaryinc.roo-cline/settings/*mcp_settings.json
.vscode-mock/global-storage/settings/*mcp_settings.json
.roo/mcp.json
.local/share/kilo/auth.json
.local/share/kilo/mcp-auth.json
.local/share/kilo/kilo*.db*
.local/share/kilo/opencode*.db*
.config/kilo/kilo.json*
.config/kilo/opencode.json*
.config/kilo/config.json
.kilocode/cli/config.json
.local/state/kilo/daemon.json
*/User/globalStorage/kilocode.kilo-code/settings/mcp_settings.json
kilo.json*
.kilo/kilo.json*
.kilo/opencode.json*
.kilocode/kilo.json*
.kilocode/opencode.json*
.continue/auth*.json
.continue/.env
.continue/config.yaml
.continue/config.json
.continue/config.ts
.continue/.configs/*/config.js*
.continue/mcpServers/*
.continue/index/globalContext.json
*/globalStorage/continue.continue/*.bin
.aider/oauth-keys.env
.aider.conf.yml
.aider.model.settings.yml
.local/share/opencode/auth.json
.local/share/opencode/mcp-auth.json
.local/share/opencode/opencode*.db*
.config/opencode/opencode.json*
.config/opencode/config.json
.opencode/opencode.json*
opencode.json*
.config/opencode/service*.json*
.config/opencode/cli.json*
.local/state/opencode/service*.json*
.local/state/opencode/pty/*/service.json
.local/share/crush/crush.json
AppData/Local/crush/crush.json
.config/crush/crush.json
.config/crush/crushrc
.crush/crush.json
crush.json
.crush.json
crushrc
.crushrc
.config/goose/secrets.yaml
.config/goose/config.yaml
.config/goose/config.tmp
.config/goose/githubcopilot/info.json
.config/goose/githubcopilot/*/info.json
.config/goose/chatgpt_codex/tokens.json
.config/goose/gemini_oauth/tokens.json
.config/goose/kimicode/token.json
.config/goose/muse_code/token.json
.config/goose/huggingface/oauth/tokens.json
.config/goose/databricks/oauth/*.json
.config/goose/xai_oauth/tokens.json
.config/goose/.tmp??????
.config/goose/githubcopilot/.tmp??????
.config/goose/githubcopilot/*/.tmp??????
.config/goose/chatgpt_codex/.tmp??????
.config/goose/gemini_oauth/.tmp??????
.config/goose/muse_code/.tmp??????
.config/goose/databricks/oauth/.tmp??????
.config/goose/xai_oauth/.tmp??????
.config/goose/custom_providers/*.json
.config/goose/custom_providers/.tmp??????
.config/goose/roaming_node_key
.config/goose/tls/server.key
.local/state/goose/claude-code/mcp-config-*.json
AppData/Roaming/Block/goose/config/secrets.yaml
AppData/Roaming/Block/goose/config/config.yaml
AppData/Roaming/Block/goose/config/config.tmp
AppData/Roaming/Block/goose/config/githubcopilot/info.json
AppData/Roaming/Block/goose/config/githubcopilot/*/info.json
AppData/Roaming/Block/goose/config/chatgpt_codex/tokens.json
AppData/Roaming/Block/goose/config/gemini_oauth/tokens.json
AppData/Roaming/Block/goose/config/kimicode/token.json
AppData/Roaming/Block/goose/config/muse_code/token.json
AppData/Roaming/Block/goose/config/huggingface/oauth/tokens.json
AppData/Roaming/Block/goose/config/databricks/oauth/*.json
AppData/Roaming/Block/goose/config/xai_oauth/tokens.json
AppData/Roaming/Block/goose/config/.tmp??????
AppData/Roaming/Block/goose/config/githubcopilot/.tmp??????
AppData/Roaming/Block/goose/config/githubcopilot/*/.tmp??????
AppData/Roaming/Block/goose/config/chatgpt_codex/.tmp??????
AppData/Roaming/Block/goose/config/gemini_oauth/.tmp??????
AppData/Roaming/Block/goose/config/muse_code/.tmp??????
AppData/Roaming/Block/goose/config/databricks/oauth/.tmp??????
AppData/Roaming/Block/goose/config/xai_oauth/.tmp??????
AppData/Roaming/Block/goose/config/custom_providers/*.json
AppData/Roaming/Block/goose/config/custom_providers/.tmp??????
AppData/Roaming/Block/goose/config/roaming_node_key
AppData/Roaming/Block/goose/config/tls/server.key
AppData/Roaming/Block/goose/data/claude-code/mcp-config-*.json
.config/[Gg]oose/settings.json
Library/Application Support/Goose/settings.json
AppData/Roaming/Goose/settings.json
diagnostics_*.json
.config/zed/development_credentials
.config/zed/settings.json
.config/zed/settings_backup.json
.config/zed/global_settings.json
AppData/Roaming/Zed/development_credentials
AppData/Roaming/Zed/settings.json
AppData/Roaming/Zed/settings_backup.json
AppData/Roaming/Zed/global_settings.json
.var/app/dev.zed.Zed*/config/zed/settings.json
.var/app/dev.zed.Zed*/config/zed/development_credentials
.local/share/amp/secrets.json
.local/share/amp/accounts.json
.local/share/amp/accounts/*/secrets.json
.amp/oauth/*
.config/amp/settings.json*
.amp/settings.json*
.factory/auth.json
.factory/auth.v2.*
.factory/auth.encrypted
.factory/config.json
.factory/temp/env/*
.factory/prem-auth/*/credentials.*
.factory/mcp-oauth*
.factory/settings.json
.factory/settings.local.json
.factory/mcp.json
.config/muse/auth.json
.config/muse/settings.json
.augment/session.json
.augment/settings.json
.augment/settings.local.json
*/kiro-cli/data.sqlite3*
*/amazon-q/data.sqlite3*
.aws/sso/cache/*.json
.aws/amazonq/mcp.json
.kiro/settings/mcp.json
.kiro/powers/installed/*/mcp.json
.kiro/agents/*.json
.kiro/web-session/*
.kiro/secrets.json
.aws/amazonq/cli-agents/*.json
.amazonq/cli-agents/*.json
.amazonq/mcp.json
.pearai/config.json
.pearai/config.ts
.pearai/.env
.pearai/.configs/*/config.js*
*/globalStorage/pearai.pearai-roo-cline/settings/pearai_agent_mcp_settings.json
.pearai-agent/mcp.json
*/Cody-nodejs/user-settings.json
*/Cody-nodejs/Config/user-settings.json
.config/JetBrains/*/options/cody_accounts.xml
Library/Application Support/JetBrains/*/options/cody_accounts.xml
AppData/Roaming/JetBrains/*/options/cody_accounts.xml
.config/JetBrains/*/options/cody.xml
Library/Application Support/JetBrains/*/options/cody.xml
AppData/Roaming/JetBrains/*/options/cody.xml
.sourcegraph/cody_settings.json
.idea/.sourcegraph/cody_settings.json
*/User/globalStorage/rjmacarthy.twinny/twinny-providers.json
.twinny/node/identity.json
.twinny/server/license
.twinny/server/plugins/*/settings.json
.twinny/server/plugins/*/repos.json
.tabby/config.toml
.tabby-client/agent/config.toml
.openinterpreter/auth.json
.openinterpreter/.credentials.json
.openinterpreter/credentials/*
.openinterpreter/device_id
.openinterpreter/secrets/*
.openinterpreter/.sandbox-secrets/*
.openinterpreter/.env
.openinterpreter/config.toml
.config/open-interpreter/profiles/*
Library/Application Support/open-interpreter/profiles/*
AppData/Local/open-interpreter/open-interpreter/profiles/*
.config/Open Interpreter*/config.yaml
Library/Application Support/Open Interpreter*/config.yaml
AppData/Local/Open Interpreter*/Open Interpreter*/config.yaml
.openhands/agent-canvas/secret-key.txt
.openhands/agent-canvas/api-key.txt
.openhands/settings.json
.openhands/secrets.json
.openhands/profiles/*
.openhands/provider-connections/*
.openhands/auth/*
.openhands/runtime-control/*
.openhands/agent_settings.json
.openhands/cloud/*
.openhands/mcp.json
.openhands/.jwt_secret
.openhands/.keys
.openhands/*/acp/codex/auth.json
Library/Application Support/OpenHands Agent Canvas/mars-credentials.json
.config/OpenHands Agent Canvas/mars-credentials.json
AppData/Roaming/OpenHands Agent Canvas/mars-credentials.json
.pi/agent/auth.json
.pi/agent/auth.json.*
.pi/agent/mcp-auth.json
.pi/agent/oauth.json*
.pi/agent/models.json
.pi/agent/mcp.json
.pi/mcp.json
.config/little-coder/models.json
.little-coder/config.json
.little-coder/mcp.json
.letta/settings.json
.letta/pg_uri
.letta/credentials
.letta/lc-local-backend/providers/auth.json
.letta/channels/*/accounts.json
.letta/channels/whatsapp/auth/*
.letta/agents/*/memory/.git/config
.letta/agents/*/memory/.git/letta-credential-helper.cmd
.config/letta/settings.json
.memgpt/credentials
.hermes*/.env
.hermes*/.env.bak*
.hermes*/.op.env
.hermes*/npmrc
.hermes*/auth.json
.hermes*/auth.json.*
.hermes*/auth/*
.hermes*/.anthropic_oauth.json
.hermes*/.copilot_jwt.json
.hermes*/google_*.json
.hermes*/google_chat_user_tokens/*
.hermes*/slack_tokens.json
.hermes*/honcho.json
.hermes*/mem0.json
.hermes*/webhook_subscriptions.json
.hermes*/teams_pipeline_store.json
.hermes*/mcp-tokens/*
.hermes*/vault/*
.hermes*/browser_auth/*
.hermes*/pairing/*
.hermes*/platforms/pairing/*
.hermes*/whatsapp/session/*
.hermes*/platforms/whatsapp/session/*
.hermes*/matrix/store/*
.hermes*/platforms/matrix/store/*
.hermes*/cache/bws_cache*.json
.hermes*/weixin/accounts/*
.hermes*/runtime/photon-sidecar.json
.hermes*/proxy/*
.hermes*/home/*
.hermes*/config.yaml
.hermes*/config.yaml.*
.hermes*/bot-desktop/*
.hermes*/webapp/*
.hermes*/feishu_comment_pairing.json
.hermes*/workspace/meetings/node_token.json
.hermes*/.ssh/*
.hermes*/.aws/*
.hermes*/.gnupg/*
.hermes*/.kube/*
.hermes*/.docker/*
.hermes*/.azure/*
.hermes*/.config/gh/*
.hermes*/.config/gcloud/*
.hermes*/.envrc
AppData/Local/hermes*/.env
AppData/Local/hermes*/.env.bak*
AppData/Local/hermes*/.op.env
AppData/Local/hermes*/npmrc
AppData/Local/hermes*/auth.json
AppData/Local/hermes*/auth.json.*
AppData/Local/hermes*/auth/*
AppData/Local/hermes*/.anthropic_oauth.json
AppData/Local/hermes*/.copilot_jwt.json
AppData/Local/hermes*/google_*.json
AppData/Local/hermes*/google_chat_user_tokens/*
AppData/Local/hermes*/slack_tokens.json
AppData/Local/hermes*/honcho.json
AppData/Local/hermes*/mem0.json
AppData/Local/hermes*/webhook_subscriptions.json
AppData/Local/hermes*/teams_pipeline_store.json
AppData/Local/hermes*/mcp-tokens/*
AppData/Local/hermes*/vault/*
AppData/Local/hermes*/browser_auth/*
AppData/Local/hermes*/pairing/*
AppData/Local/hermes*/platforms/pairing/*
AppData/Local/hermes*/whatsapp/session/*
AppData/Local/hermes*/platforms/whatsapp/session/*
AppData/Local/hermes*/matrix/store/*
AppData/Local/hermes*/platforms/matrix/store/*
AppData/Local/hermes*/cache/bws_cache*.json
AppData/Local/hermes*/weixin/accounts/*
AppData/Local/hermes*/runtime/photon-sidecar.json
AppData/Local/hermes*/proxy/*
AppData/Local/hermes*/home/*
AppData/Local/hermes*/config.yaml
AppData/Local/hermes*/config.yaml.*
AppData/Local/hermes*/bot-desktop/*
AppData/Local/hermes*/webapp/*
AppData/Local/hermes*/feishu_comment_pairing.json
AppData/Local/hermes*/workspace/meetings/node_token.json
AppData/Local/hermes*/.ssh/*
AppData/Local/hermes*/.aws/*
AppData/Local/hermes*/.gnupg/*
AppData/Local/hermes*/.kube/*
AppData/Local/hermes*/.docker/*
AppData/Local/hermes*/.azure/*
AppData/Local/hermes*/.config/gh/*
AppData/Local/hermes*/.config/gcloud/*
AppData/Local/hermes*/.envrc
.config/Hermes*/connection*.json*
Library/Application Support/Hermes*/connection*.json*
AppData/Roaming/Hermes*/connection*.json*
.openclaw*/openclaw.json*
.openclaw*/clawdbot.json*
.openclaw*/moltbot.json*
.openclaw*/.env
.openclaw*/credentials/*
.openclaw*/service-env/*
.openclaw*/client/credentials.sqlite*
.openclaw*/identity/*
.openclaw*/exec-approvals.json*
.openclaw*/agents/*/agent/auth-profiles.json*
.openclaw*/agents/*/agent/auth-state.json*
.openclaw*/agents/*/agent/auth.json*
.openclaw*/agents/*/agent/models.json
.openclaw*/browser/*/user-data/*/Cookies*
.clawdbot/openclaw.json*
.clawdbot/clawdbot.json*
.clawdbot/moltbot.json*
.clawdbot/.env
.clawdbot/credentials/*
.clawdbot/service-env/*
.clawdbot/client/credentials.sqlite*
.clawdbot/identity/*
.clawdbot/exec-approvals.json*
.clawdbot/agents/*/agent/auth-profiles.json*
.clawdbot/agents/*/agent/auth-state.json*
.clawdbot/agents/*/agent/auth.json*
.clawdbot/agents/*/agent/models.json
.clawdbot/browser/*/user-data/*/Cookies*
.moltbot/openclaw.json*
.moltbot/clawdbot.json*
.moltbot/moltbot.json*
.moltbot/.env
.moltbot/credentials/*
.moltbot/service-env/*
.moltbot/client/credentials.sqlite*
.moltbot/identity/*
.moltbot/exec-approvals.json*
.moltbot/agents/*/agent/auth-profiles.json*
.moltbot/agents/*/agent/auth-state.json*
.moltbot/agents/*/agent/auth.json*
.moltbot/agents/*/agent/models.json
.moltbot/browser/*/user-data/*/Cookies*
.openclaw-auth-profile-secrets/*
.config/openclaw/gateway.env
.nanobot*/config.json
.nanobot*/auth/*
.nanobot*/whatsapp-auth/*
.nanobot*/matrix-store/*
.nanobot*/webui/remote-pairing/*/identity
.nanobot*/webui/remote-pairing/*/exchange
agent-zero*/usr/.env
Desktop/agent-zero*/usr/.env
agent-zero*/usr/secrets.env
Desktop/agent-zero*/usr/secrets.env
agent-zero*/usr/settings.json
Desktop/agent-zero*/usr/settings.json
agent-zero*/usr/projects/*/.a0proj/secrets.env
Desktop/agent-zero*/usr/projects/*/.a0proj/secrets.env
agent-zero*/.env
Desktop/agent-zero*/.env
agent-zero*/tmp/secrets.env
Desktop/agent-zero*/tmp/secrets.env
agent-zero*/tmp/settings.json
Desktop/agent-zero*/tmp/settings.json
agent-zero*/usr/plugins/_desktop/profiles/*/.ssh/*
Desktop/agent-zero*/usr/plugins/_desktop/profiles/*/.ssh/*
agent-zero*/usr/plugins/_desktop/profiles/*/.gnupg/*
Desktop/agent-zero*/usr/plugins/_desktop/profiles/*/.gnupg/*
agent-zero*/usr/plugins/*/config.json
Desktop/agent-zero*/usr/plugins/*/config.json
agent-zero*/tmp/whatsapp/session/*
Desktop/agent-zero*/tmp/whatsapp/session/*
.agent-zero/.env
.agent-zero/session_cookies.json
*/Agent Zero Launcher/docker_manager/state.json
*/Agent Zero Launcher/Local State
.config/shell_gpt/.sgptrc
.local/share/local-deep-research/.secret_key
Library/Application Support/local-deep-research/.secret_key
AppData/Local/local-deep-research/local-deep-research/.secret_key
.autogenstudio/.env
.autogenstudio/temp_env_vars.env
.config/crewai/settings.json
.local/share/crewai/credentials/*
Library/Application Support/crewai/credentials/*
AppData/Local/[Cc]rew[Aa][Ii]/credentials/*
.camel/gmail_token.json
.config/difyctl/tokens.yml
AppData/Roaming/difyctl/tokens.yml
AppData/Local/difyctl/tokens.yml
.config/difyctl/login-pending.yml
AppData/Roaming/difyctl/login-pending.yml
AppData/Local/difyctl/login-pending.yml
.flowise/encryption.key
.flowise/*_secret.key
.cache/langflow/*secret_key
Library/Caches/langflow/*secret_key
AppData/Local/langflow/langflow/Cache/*secret_key
.cache/langflow/*/_mcp_servers_*.json
Library/Caches/langflow/*/_mcp_servers_*.json
AppData/Local/langflow/langflow/Cache/*/_mcp_servers_*.json
.langflow/data/.env
AppData/Roaming/com.LangflowDesktop/data/.env
.metagpt/config2.yaml
.metagpt/config2.bak
.n8n/config
.n8n/ssh/*
.config/pydantic-clai2/credentials-*
.config/pydantic-clai2/logfire/logfire_credentials.json
.local/state/pydantic-clai2/workspaces/registry/*/token
.config/pydantic-clai2/mcp.json
.clai/mcp_servers.json
.ollama/id_ed25519
.ollama/backup/*
.ollama/launch/muse-config/muse/settings.json
.env
secrets.env
settings.json
projects/*/.a0proj/secrets.env
plugins/_desktop/profiles/*/.ssh/*
plugins/_desktop/profiles/*/.gnupg/*
plugins/*/config.json
usr/.env
usr/secrets.env
usr/settings.json
usr/projects/*/.a0proj/secrets.env
usr/plugins/_desktop/profiles/*/.ssh/*
usr/plugins/_desktop/profiles/*/.gnupg/*
usr/plugins/*/config.json
tmp/secrets.env
tmp/settings.json
tmp/whatsapp/session/*
config.toml
id_ed25519
backup/*
launch/muse-config/muse/settings.json
.secret_key
agent-canvas/secret-key.txt
agent-canvas/api-key.txt
secrets.json
profiles/*.json
profiles/tmp*.tmp
provider-connections/*
runtime-control/*
agent_settings.json
cloud/*
mcp.json
.jwt_secret
.keys
conversations/*/acp/codex/auth.json
agent-canvas/*/acp/codex/auth.json
pg_uri
credentials
lc-local-backend/providers/auth.json
channels/*/accounts.json
channels/whatsapp/auth/*
agents/*/memory/.git/config
agents/*/memory/.git/letta-credential-helper.cmd
.env.bak*
.op.env
npmrc
auth.json
auth.json.*
auth/*
.anthropic_oauth.json
.copilot_jwt.json
google_*.json
google_chat_user_tokens/*
slack_tokens.json
honcho.json
mem0.json
webhook_subscriptions.json
teams_pipeline_store.json
mcp-tokens/*
vault/*
browser_auth/*
pairing/*
platforms/pairing/*
whatsapp/session/*
platforms/whatsapp/session/*
matrix/store/*
platforms/matrix/store/*
cache/bws_cache*.json
weixin/accounts/*
runtime/photon-sidecar.json
proxy/*
home/*
config.yaml
config.yaml.*
bot-desktop/*
webapp/*
feishu_comment_pairing.json
workspace/meetings/node_token.json
.ssh/*
.aws/credentials
.aws/config
.aws/cli/cache/*.json
.gnupg/*
.kube/*
.docker/*
.azure/*
.config/gh/*
.config/gcloud/*
.envrc
state-snapshots/*/.env
state-snapshots/*/auth.json
state-snapshots/*/config.yaml
state-snapshots/*/feishu_comment_pairing.json
state-snapshots/*/pairing/*
state-snapshots/*/platforms/pairing/*
profiles/*/.env
profiles/*/.env.bak*
profiles/*/.op.env
profiles/*/npmrc
profiles/*/auth.json
profiles/*/auth.json.*
profiles/*/auth/*
profiles/*/.anthropic_oauth.json
profiles/*/.copilot_jwt.json
profiles/*/google_*.json
profiles/*/google_chat_user_tokens/*
profiles/*/slack_tokens.json
profiles/*/honcho.json
profiles/*/mem0.json
profiles/*/webhook_subscriptions.json
profiles/*/teams_pipeline_store.json
profiles/*/mcp-tokens/*
profiles/*/vault/*
profiles/*/browser_auth/*
profiles/*/pairing/*
profiles/*/platforms/pairing/*
profiles/*/whatsapp/session/*
profiles/*/platforms/whatsapp/session/*
profiles/*/matrix/store/*
profiles/*/platforms/matrix/store/*
profiles/*/cache/bws_cache*.json
profiles/*/weixin/accounts/*
profiles/*/runtime/photon-sidecar.json
profiles/*/proxy/*
profiles/*/home/*
profiles/*/config.yaml
profiles/*/config.yaml.*
profiles/*/bot-desktop/*
profiles/*/webapp/*
profiles/*/feishu_comment_pairing.json
profiles/*/workspace/meetings/node_token.json
profiles/*/.ssh/*
profiles/*/.aws/credentials
profiles/*/.aws/config
profiles/*/.aws/sso/cache/*.json
profiles/*/.aws/cli/cache/*.json
profiles/*/.gnupg/*
profiles/*/.kube/*
profiles/*/.docker/*
profiles/*/.azure/*
profiles/*/.config/gh/*
profiles/*/.config/gcloud/*
profiles/*/.envrc
config.json
whatsapp-auth/*
matrix-store/*
secret_key
cache_secret_key
[0-9a-f]*-*-*-*-*/_mcp_servers_*.json
config
ssh/*
'@

# Docker and Podman named volumes, agent|glob matched against the volume name.
$DOCKER_VOLUMES = @'
# Matched against each volume name; the first matching line wins. A matched
# volume is collected whole as user "docker" with its _data directory as the
# home, so EXCLUDES and SECRET_GLOBS apply relative to the volume root.
# Agent Zero: a0_usr in the README docker run (Compose prefixes it as
# <project>_a0_usr) and a0-launcher-<slug>-usr in the A0 Launcher named-volume
# mode; both are mounted at /a0/usr
agent-zero|*a0_usr
agent-zero|a0-launcher-*-usr
# Local Deep Research: ldr_data in docker-compose.yml, mounted at /data
local-deep-research|*ldr_data
# Ollama: ollama in docs/docker.mdx, ollama_data in the Local Deep Research
# compose file; mounted at /root/.ollama
ollama|ollama
ollama|*ollama*
# Dify: dify_agent_local_sandbox_home at /home/dify (shellctl job database and
# output logs under .local/share/shellctl) and dify_agent_local_sandbox_workspace
# at /workspace in docker/docker-compose.yaml; Compose prefixes them (docker_)
dify|*dify_agent_local_sandbox_home
dify|*dify_agent_local_sandbox_workspace
# Langflow: langflow-data in docker_example/docker-compose.yml, mounted at
# /app/langflow, the config dir (LANGFLOW_CONFIG_DIR)
langflow|*langflow-data
# n8n: n8n_data in the README docker run and n8n-data in the get-n8n.sh compose
# file (n8n_n8n-data); both mounted at /home/node/.n8n
n8n|*n8n_data
n8n|*n8n-data
# The OpenClaw Docker setup: OPENCLAW_HOME_VOLUME (openclaw_home in its guide),
# mounted at /home/node, the home of a Claude Code install made in the container
claude-code|*openclaw_home
# Inferred names: the documentation of these tools bind-mounts the home directory
# instead of a named volume; a volume replacing it has the same layout (~/.tabby
# at /data, ~/.hermes at /opt/data, ...)
tabby|*tabby*
local-deep-research|*local-deep-research*
openhands|*openhands*
letta|*letta*
hermes|*hermes*
nanobot|*nanobot*
'@

# ===========================================================================
# Shared helpers
# ===========================================================================
# Constants and small functions every later section uses.

# UTF-8 without a byte order mark, for every file the collector writes
# (Add-Content on 5.1 defaults to ASCII and mangles non-ASCII paths).
$script:Utf8NoBom = New-Object System.Text.UTF8Encoding $false
# \ on Windows, / under PowerShell 7 on Linux or macOS.
$script:Sep = [string][System.IO.Path]::DirectorySeparatorChar
# Windows, as opposed to PowerShell 7 on Linux or macOS.
$script:IsWindowsHost = ($env:OS -eq 'Windows_NT')

# Takes a table here-string; returns its lines that are neither empty nor a
# # comment.
function Get-TableLines([string]$table) {
  $table -split "`r?`n" | Where-Object { $_ -ne '' -and -not $_.StartsWith('#') }
}

# Takes a message; appends "<UTC time> <message>" to the run log through
# $script:LogWriter when one is open (-Inventory opens none) and writes the
# message alone to stderr unless -Quiet.
$script:LogWriter = $null
function Write-CollectorLog([string]$message) {
  $timestamp = (Get-Date).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ')
  if ($script:LogWriter) { $script:LogWriter.WriteLine("$timestamp $message") }
  if (-not $Quiet) { [Console]::Error.WriteLine($message) }
}

# Takes a message; appends it to $script:Notes, the notes list of
# collection.json, and logs it as "NOTE: <message>".
$script:Notes = @()
function Add-Note([string]$message) {
  $script:Notes += $message
  Write-CollectorLog "NOTE: $message"
}

# Takes a string; returns its JSON string body (no surrounding quotes) the
# way PowerShell 7's ConvertTo-Json writes it (" and \, control characters,
# \u0085, \u2028 and \u2029 escaped). Write-ManifestRow calls it only for a row
# holding one of those rare characters ($script:JsonRareRx).
$script:JsonRareRx = New-Object System.Text.RegularExpressions.Regex(
  '[\x00-\x1f\u0085\u2028\u2029]')
$script:JsonInv = [System.Globalization.CultureInfo]::InvariantCulture
function ConvertTo-JsonEscaped([string]$text) {
  $builder = New-Object System.Text.StringBuilder ($text.Length + 16)
  foreach ($char in $text.ToCharArray()) {
    $code = [int]$char
    if ($char -eq '"') { [void]$builder.Append('\"') }
    elseif ($char -eq '\') { [void]$builder.Append('\\') }
    elseif ($code -eq 8) { [void]$builder.Append('\b') }
    elseif ($code -eq 9) { [void]$builder.Append('\t') }
    elseif ($code -eq 10) { [void]$builder.Append('\n') }
    elseif ($code -eq 12) { [void]$builder.Append('\f') }
    elseif ($code -eq 13) { [void]$builder.Append('\r') }
    elseif ($code -lt 0x20 -or $code -eq 0x85 -or $code -eq 0x2028 -or $code -eq 0x2029) {
      [void]$builder.Append('\u').Append($code.ToString('x4', $script:JsonInv))
    }
    else { [void]$builder.Append($char) }
  }
  return $builder.ToString()
}

# Takes a glob and whether * and ? may cross /; returns an anchored regex.
# Catalog globs match one path segment at a time; exclusion and secret globs a
# whole relative path.
function Convert-GlobToRegex([string]$glob, [bool]$starCrossesSeparator) {
  $builder = New-Object System.Text.StringBuilder
  [void]$builder.Append('^')
  $star = '[^/]*'; $question = '[^/]'
  if ($starCrossesSeparator) { $star = '.*'; $question = '.' }
  $i = 0
  while ($i -lt $glob.Length) {
    $char = $glob[$i]
    switch ($char) {
      '*' { [void]$builder.Append($star) }
      '?' { [void]$builder.Append($question) }
      '[' {
        $close = $glob.IndexOf(']', $i + 1)
        if ($close -gt $i) {
          $class = $glob.Substring($i, $close - $i + 1)
          if ($class.StartsWith('[!')) { $class = '[^' + $class.Substring(2) }
          [void]$builder.Append($class); $i = $close
        }
        else { [void]$builder.Append('\[') }
      }
      default { [void]$builder.Append([regex]::Escape([string]$char)) }
    }
    $i++
  }
  [void]$builder.Append('$')
  $builder.ToString()
}

# Takes the globs of one table; returns one case-insensitive Regex matching a
# relative path when any glob does, so each path costs one IsMatch. An empty
# table matches nothing; RegexOptions.Compiled is off, being slower on 5.1.
function ConvertTo-TableRegex([string[]]$globs) {
  $bodies = @(foreach ($glob in $globs) {
      $globRegex = Convert-GlobToRegex $glob $true
      $globRegex.Substring(1, $globRegex.Length - 2)
    })
  $pattern = '(?!)'
  if ($bodies.Count -gt 0) { $pattern = '^(?:' + ($bodies -join '|') + ')$' }
  $options = [System.Text.RegularExpressions.RegexOptions]::IgnoreCase
  return (New-Object System.Text.RegularExpressions.Regex($pattern, $options))
}

# On Windows every file system call takes the extended-length form (\\?\C:\...),
# which passes 260 characters without LongPathsEnabled; everything written out
# keeps the plain form. See "Locked files and long paths" in windows.md.
$script:UseExtendedPaths = ($env:OS -eq 'Windows_NT')
# A . or .. segment or a doubled separator, which the prefix stops Win32 from
# resolving, so ConvertTo-ExtendedPath resolves it first.
$script:ExtendedNormRx = New-Object System.Text.RegularExpressions.Regex(
  '\\\.{1,2}(?:\\|$)|\\\\')

# Takes a path; returns its extended-length form on Windows, with . and ..
# segments resolved, or the path unchanged elsewhere or when relative.
function ConvertTo-ExtendedPath([string]$path) {
  $ordinal = [System.StringComparison]::Ordinal
  if (-not $script:UseExtendedPaths -or -not $path -or $path.StartsWith('\\?\', $ordinal)) {
    return $path
  }
  $backslashed = $path.Replace('/', '\')
  if ($backslashed.Length -ge 3 -and $backslashed[1] -eq ':' -and $backslashed[2] -eq '\') {
    $prefix = '\\?\'; $body = $backslashed; $keepSegments = 1
  }
  elseif ($backslashed.Length -gt 2 -and $backslashed.StartsWith('\\', $ordinal) -and
    $backslashed[2] -ne '?' -and $backslashed[2] -ne '.') {
    $prefix = '\\?\UNC\'; $body = $backslashed.Substring(2); $keepSegments = 2
  }
  else { return $path }
  if ($script:ExtendedNormRx.IsMatch($body)) {
    # $keepSegments leading segments (the drive, or server and share) are
    # never popped.
    $segments = New-Object System.Collections.ArrayList
    foreach ($segment in $body.Split([char]'\')) {
      if ($segments.Count -lt $keepSegments) { [void]$segments.Add($segment); continue }
      if ($segment -eq '' -or $segment -eq '.') { continue }
      if ($segment -eq '..') {
        if ($segments.Count -gt $keepSegments) { $segments.RemoveAt($segments.Count - 1) }
        continue
      }
      [void]$segments.Add($segment)
    }
    $body = [string]::Join('\', [string[]]$segments.ToArray())
    if ($segments.Count -le $keepSegments) { $body += '\' }
  }
  return $prefix + $body
}

# Takes a path; returns its plain form (\\?\UNC\ becomes \\, \\?\ is
# dropped), or the path unchanged when it is not extended.
function ConvertFrom-ExtendedPath([string]$path) {
  if (-not $path) { return $path }
  if ($path.StartsWith('\\?\UNC\', [System.StringComparison]::OrdinalIgnoreCase)) {
    return '\\' + $path.Substring(8)
  }
  if ($path.StartsWith('\\?\', [System.StringComparison]::Ordinal)) { return $path.Substring(4) }
  return $path
}

# Takes an exception message; returns it with every extended path turned back
# into the plain form, for rows and the log.
function ConvertFrom-ExtendedText([string]$text) {
  if (-not $text -or $text.IndexOf('\\?\', [System.StringComparison]::Ordinal) -lt 0) {
    return $text
  }
  return $text.Replace('\\?\UNC\', '\\').Replace('\\?\', '')
}

# Takes a path and a kind (Leaf, Container, or anything else for either);
# returns whether it exists as that kind. Test-Path throws on protected
# profiles even with -ErrorAction, so this probes through .NET.
function Test-PathQuiet([string]$path, [string]$kind) {
  if (-not $path) { return $false }
  $path = ConvertTo-ExtendedPath $path
  try {
    if ($kind -eq 'Leaf') { return [System.IO.File]::Exists($path) }
    if ($kind -eq 'Container') { return [System.IO.Directory]::Exists($path) }
    return ([System.IO.File]::Exists($path) -or [System.IO.Directory]::Exists($path))
  } catch { return $false }
}

# Takes a parent path and a child that may use / or \; returns them joined with
# the platform separator. Join-Path resolves PowerShell drives.
function Join-PathSafe([string]$parent, [string]$child) {
  $child = $child.Replace('\', $script:Sep).Replace('/', $script:Sep)
  if ($parent.EndsWith('\') -or $parent.EndsWith('/')) { return $parent + $child }
  return $parent + $script:Sep + $child
}

# Takes a base directory and a full path; returns the path relative to the base
# (or whole) with / separators, the form the globs match. Ignores case.
function Get-RelPath([string]$base, [string]$full) {
  $trimmedBase = $base.TrimEnd('\', '/')
  $ignoreCase = [System.StringComparison]::OrdinalIgnoreCase
  if ($full.Length -gt $trimmedBase.Length -and $full.StartsWith($trimmedBase, $ignoreCase)) {
    return $full.Substring($trimmedBase.Length).TrimStart('\', '/').Replace('\', '/')
  }
  return $full.Replace('\', '/')
}

# Takes a DateTime; returns Unix epoch seconds, or 0 when it cannot be
# converted.
function Get-Epoch($dateTime) {
  try { return [int64]([DateTimeOffset]::new([DateTime]$dateTime).ToUnixTimeSeconds()) }
  catch { return [int64]0 }
}

# Takes a path; returns its owner as an account name, or its SID when the
# name cannot be resolved, or '' when the security descriptor cannot be read
# (always under PowerShell 7, whose File class has no GetAccessControl). Only
# the owner section is read, and each SID is translated once per run
# ($script:OwnerNames caches SID -> name).
$script:OwnerNames = @{}
function Get-OwnerName([string]$path) {
  try {
    $sections = [System.Security.AccessControl.AccessControlSections]::Owner
    $security = [System.IO.File]::GetAccessControl((ConvertTo-ExtendedPath $path), $sections)
    $ownerSid = $security.GetOwner([System.Security.Principal.SecurityIdentifier])
    $key = $ownerSid.Value
  } catch { return '' }
  if (-not $script:OwnerNames.ContainsKey($key)) {
    $accountName = $key
    try {
      $accountName = $ownerSid.Translate([System.Security.Principal.NTAccount]).Value
    } catch { }
    $script:OwnerNames[$key] = $accountName
  }
  return $script:OwnerNames[$key]
}

# Takes a path; returns the lowercase hex SHA-256 of the file, or '' when it
# cannot be read. One SHA256 object serves the run instead of a Get-FileHash
# call per file. Used for the archive; Copy-FileShared hashes each staged
# copy itself.
$script:Sha256 = [System.Security.Cryptography.SHA256]::Create()
$script:ShareReadWriteDelete = [System.IO.FileShare]::ReadWrite -bor [System.IO.FileShare]::Delete
function Get-FileSha256([string]$path) {
  $stream = $null
  try {
    $stream = New-Object System.IO.FileStream((ConvertTo-ExtendedPath $path),
      [System.IO.FileMode]::Open, [System.IO.FileAccess]::Read, $script:ShareReadWriteDelete)
    $hash = $script:Sha256.ComputeHash($stream)
    return [System.BitConverter]::ToString($hash).Replace('-', '').ToLowerInvariant()
  } catch { return '' }
  finally { if ($stream) { $stream.Dispose() } }
}

# Takes a directory; returns the total size of the files under it, or 0 when it
# cannot be listed.
function Get-DirSize([string]$dir) {
  try {
    $listing = @{ LiteralPath = (ConvertTo-ExtendedPath $dir); Recurse = $true; Force = $true }
    $measure = Get-ChildItem @listing -File -ErrorAction SilentlyContinue |
      Measure-Object -Property Length -Sum
    if ($measure.Sum) { return [int64]$measure.Sum } else { return [int64]0 }
  } catch { return [int64]0 }
}

# Takes a FileSystemInfo; returns its link target (several joined with ;),
# or '' when it has none or it cannot be read.
function Get-LinkTarget($item) {
  try {
    $targetProperty = $item.PSObject.Properties['Target']
    if ($targetProperty -and $targetProperty.Value) {
      return ([string[]]$targetProperty.Value) -join ';'
    }
  } catch { }
  return ''
}

# Takes a path; returns whether it is a reparse point (a symlink or
# junction), $false when it cannot be read.
function Test-IsLink([string]$path) {
  try {
    $item = Get-Item -LiteralPath (ConvertTo-ExtendedPath $path) -Force -ErrorAction Stop
    return (($item.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0)
  } catch { return $false }
}

# Takes a FileSystemInfo; returns its manifest type (symlink, file, socket, fifo,
# device or other). PowerShell 5.1 exposes no reparse tag, so a reparse point
# that is neither a symbolic link nor a junction is 'other'.
function Get-ManifestType($item) {
  if (($item.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0) {
    $linkType = $null
    try {
      $linkProperty = $item.PSObject.Properties['LinkType']
      if ($linkProperty) { $linkType = [string]$linkProperty.Value }
    } catch { }
    if ($null -eq $linkType -or $linkType -eq 'SymbolicLink' -or $linkType -eq 'Junction') {
      return 'symlink'
    }
    return 'other'
  }
  $unixMode = ''
  try {
    $modeProperty = $item.PSObject.Properties['UnixMode']
    if ($modeProperty) { $unixMode = [string]$modeProperty.Value }
  } catch { }
  if ($unixMode.StartsWith('s')) { return 'socket' }
  if ($unixMode.StartsWith('p')) { return 'fifo' }
  if ($unixMode.StartsWith('b') -or $unixMode.StartsWith('c')) { return 'device' }
  return 'file'
}

# Takes a directory; returns whether its entries can be listed.
function Test-DirReadable([string]$path) {
  try {
    [void][System.IO.Directory]::GetFileSystemEntries((ConvertTo-ExtendedPath $path))
    return $true
  } catch { return $false }
}

# ===========================================================================
# Manifest writer
# ===========================================================================
# Every row of manifest.jsonl goes through Write-ManifestRow, which also
# keeps the per-status counts for collection.json and stdout. The fields and
# status values are an interface (collectors/docs/output.md).

# Rows per status, and the bytes of collected files.
$script:Counts = @{
  collected = 0; symlink = 0; skipped_excluded = 0; skipped_size = 0; skipped_secret = 0
  skipped_special = 0; error_copy = 0; skipped_unmatched_volume = 0; error_read = 0
  bytes = [int64]0
}
# Homes (and other bases) that gave at least one manifest row.
$script:RowHomes = @{}
# The manifest.jsonl writer, opened by Open-RunFiles unless -Inventory.
$script:ManifestWriter = $null

# Takes the manifest fields in order, without uid, gid and mode; writes one JSON
# line, counts it in $script:Counts and marks its home in $script:RowHomes. Rows
# are written by hand: ConvertTo-Json is slower per row and differs between 5.1
# and 7.
function Write-ManifestRow {
  param([string]$user, [string]$homeDir, [string]$agent, [string]$path, [string]$archivePath,
    [string]$type, [int64]$size, [int64]$mtime, [int64]$atime, [int64]$ctime, [int64]$btime,
    [string]$owner, [string]$attributes, [string]$sha256, [bool]$secret, [string]$status,
    [string]$target, [string]$errorText)
  $fields = [string[]]@($user, $homeDir, $agent, $path, $archivePath, $type, $owner, $attributes,
    $sha256, $status, $target, $errorText)
  if ($script:JsonRareRx.IsMatch([string]::Concat($fields))) {
    for ($i = 0; $i -lt $fields.Length; $i++) { $fields[$i] = ConvertTo-JsonEscaped $fields[$i] }
  } else {
    # No field holds a control character, so U+0000 can separate them while
    # " and \ are escaped in one pass.
    $joined = [string]::Join([string][char]0, $fields)
    $fields = $joined.Replace('\', '\\').Replace('"', '\"').Split([char]0)
  }
  $secretJson = 'false'; if ($secret) { $secretJson = 'true' }
  $culture = $script:JsonInv
  $script:ManifestWriter.WriteLine([string]::Concat([string[]]@(
        '{"user":"', $fields[0], '","home":"', $fields[1], '","agent":"', $fields[2],
        '","path":"', $fields[3], '","archive_path":"', $fields[4], '","type":"', $fields[5],
        '","size":', $size.ToString($culture), ',"mtime":', $mtime.ToString($culture),
        ',"atime":', $atime.ToString($culture), ',"ctime":', $ctime.ToString($culture),
        ',"btime":', $btime.ToString($culture), ',"uid":0,"gid":0,"mode":"","owner":"', $fields[6],
        '","attributes":"', $fields[7], '","sha256":"', $fields[8], '","secret":', $secretJson,
        ',"status":"', $fields[9], '","target":"', $fields[10], '","error":"', $fields[11], '"}')))
  if ($script:Counts.ContainsKey($status)) { $script:Counts[$status]++ }
  # The error_read row for an unreadable home itself does not make it a home
  # with artifacts.
  if (-not ($status -eq 'error_read' -and $path -eq $homeDir)) {
    $script:RowHomes[$homeDir] = $true
  }
  if ($status -eq 'collected') { $script:Counts['bytes'] += $size }
}

# Takes a directory; returns why it cannot be walked, or '' when it can. Off
# Windows a directory that can be listed but not entered also fails, matching
# the sh collector's -r and -x tests.
function Get-DirReadError([string]$dir) {
  try {
    [void]@(Get-ChildItem -LiteralPath (ConvertTo-ExtendedPath $dir) -Force -ErrorAction Stop)
    return ''
  }
  catch { return (ConvertFrom-ExtendedText $_.Exception.Message) }
}

# Takes the user, home, agent, a directory that could not be walked and the
# reason; logs it and, unless -Inventory, writes its error_read row.
function Write-ReadErrorRow {
  param([string]$user, [string]$homeDir, [string]$agent, [string]$dir, [string]$errorText)
  Write-CollectorLog "read failed: ${dir}: $errorText"
  if ($Inventory) { return }
  $mtime = 0; $atime = 0; $btime = 0; $attributes = ''
  try {
    $dirItem = Get-Item -LiteralPath (ConvertTo-ExtendedPath $dir) -Force -ErrorAction Stop
    $mtime = Get-Epoch $dirItem.LastWriteTimeUtc
    $atime = Get-Epoch $dirItem.LastAccessTimeUtc
    $btime = Get-Epoch $dirItem.CreationTimeUtc
    $attributes = [string]$dirItem.Attributes
  } catch { }
  $owner = ''; if (Test-PathQuiet $dir 'Container') { $owner = Get-OwnerName $dir }
  $row = @($user, $homeDir, $agent, $dir, '', 'dir', 0, $mtime, $atime, 0, $btime, $owner,
    $attributes, '', $false, 'error_read', '', $errorText)
  Write-ManifestRow @row
}

# ===========================================================================
# Staging files
# ===========================================================================
# PowerShell has no worker programs: Add-CollectedTree calls Add-CollectedFile
# in this process for every file.

# Takes a full source path; returns its fs/... archive path.
function Get-ArchivePath([string]$full) {
  if ($script:Mode -eq 'image') { return 'fs/' + (Get-RelPath $Root $full) }
  if ($full.Length -ge 3 -and $full[1] -eq ':') {
    return 'fs/' + $full.Substring(0, 1).ToUpper() + '/' + $full.Substring(3).Replace('\', '/')
  }
  return 'fs/' + $full.TrimStart('\').Replace('\', '/')
}

# Staging directories already created or found, so each file does not test
# its parent again.
$script:StagedDirs = New-Object 'System.Collections.Generic.HashSet[string]'

# Takes the source path, the staging path and the source's pre-copy Get-Item;
# copies the file with shared access and returns @(sha256 of the copy or '',
# bytes copied). The hash reads the copy through the handle that wrote it,
# because reopening waits for the antivirus scan; "Locked files and long
# paths" in collectors/docs/windows.md has the rest. Throws on failure.
function Copy-FileShared([string]$source, [string]$destination, $item) {
  $inStream = $null; $outStream = $null; $sha256 = ''; $copied = [int64]0
  $source = ConvertTo-ExtendedPath $source; $destination = ConvertTo-ExtendedPath $destination
  $stageDir = [System.IO.Path]::GetDirectoryName($destination)
  if (-not $script:StagedDirs.Contains($stageDir)) {
    if (-not (Test-PathQuiet $stageDir 'Any')) {
      [void][System.IO.Directory]::CreateDirectory($stageDir)
    }
    [void]$script:StagedDirs.Add($stageDir)
  }
  try {
    $inStream = New-Object System.IO.FileStream($source, [System.IO.FileMode]::Open,
      [System.IO.FileAccess]::Read, $script:ShareReadWriteDelete)
    $outStream = New-Object System.IO.FileStream($destination, [System.IO.FileMode]::Create,
      [System.IO.FileAccess]::ReadWrite, [System.IO.FileShare]::None)
    $inStream.CopyTo($outStream)
    $copied = [int64]$outStream.Length
    $inStream.Dispose(); $inStream = $null
    try {
      $outStream.Flush(); $outStream.Position = 0
      $hash = $script:Sha256.ComputeHash($outStream)
      $sha256 = [System.BitConverter]::ToString($hash).Replace('-', '').ToLowerInvariant()
    } catch { $sha256 = '' }
  } finally {
    if ($outStream) { $outStream.Dispose() }
    if ($inStream) { $inStream.Dispose() }
  }
  try {
    [System.IO.File]::SetCreationTimeUtc($destination, $item.CreationTimeUtc)
    [System.IO.File]::SetLastWriteTimeUtc($destination, $item.LastWriteTimeUtc)
    [System.IO.File]::SetLastAccessTimeUtc($destination, $item.LastAccessTimeUtc)
  } catch { }
  return @($sha256, $copied)
}

# ---------------------------------------------------------------------------
# Links are recreated with their target string as stored, never with New-Item,
# which on 5.1 resolves a relative target and refuses a missing one; windows.md,
# "Symlinks and reparse points", has the details.
# ---------------------------------------------------------------------------
$script:LinkApi = $null
$script:LinkApiError = ''
$script:LinkFileMethod = $null
$script:LinkDirMethod = $null
$script:CmdExe = ''
# Each staged link as @(plain staging path, is a directory link, target).
$script:StagedLinks = New-Object System.Collections.ArrayList
# Links left out because the archive is a zip, and links that failed.
$script:LinkCounts = @{ zip = 0; failed = 0 }

# Returns the link API, detected once: 'dotnet' (PowerShell 7), 'mklink'
# (Windows PowerShell 5.1), or '' with the reason in $script:LinkApiError.
function Get-LinkApi {
  if ($null -ne $script:LinkApi) { return $script:LinkApi }
  $script:LinkApi = ''
  $twoStrings = [Type[]]@([string], [string])
  $script:LinkFileMethod = [System.IO.File].GetMethod('CreateSymbolicLink', $twoStrings)
  $script:LinkDirMethod = [System.IO.Directory].GetMethod('CreateSymbolicLink', $twoStrings)
  if ($null -ne $script:LinkFileMethod -and $null -ne $script:LinkDirMethod) {
    $script:LinkApi = 'dotnet'
  }
  elseif ($env:OS -eq 'Windows_NT') {
    $cmdPath = ''
    if ($env:SystemRoot) { $cmdPath = Join-PathSafe $env:SystemRoot 'System32\cmd.exe' }
    if ($cmdPath -ne '' -and (Test-PathQuiet $cmdPath 'Leaf')) {
      $script:CmdExe = $cmdPath; $script:LinkApi = 'mklink'
    }
    else { $script:LinkApiError = 'cmd.exe not found for mklink' }
  } else { $script:LinkApiError = 'no symlink API in this PowerShell' }
  return $script:LinkApi
}

# Takes the link path, its target and whether it is a directory link; runs
# mklink with an argument string built here, so PowerShell's native-argument
# quoting is not involved. Returns '' or mklink's message and exit code.
function Invoke-Mklink([string]$linkPath, [string]$target, [bool]$isDir) {
  # cmd expands %NAME% even inside quotes, and a quote would end the argument.
  if ($linkPath.Contains('%') -or $linkPath.Contains('"') -or $target.Contains('%') -or
    $target.Contains('"')) {
    return 'link path or target contains % or ", which cmd.exe mklink cannot take verbatim'
  }
  $dirSwitch = ''; if ($isDir) { $dirSwitch = '/D ' }
  $startInfo = New-Object System.Diagnostics.ProcessStartInfo
  $startInfo.FileName = $script:CmdExe
  $quotedArgs = '"' + $linkPath + '" "' + $target + '"'
  $startInfo.Arguments = '/d /v:off /c mklink ' + $dirSwitch + $quotedArgs
  # The link path is absolute; a UNC current directory would add a cmd.exe
  # warning to stderr.
  $startInfo.WorkingDirectory = $env:SystemRoot
  $startInfo.UseShellExecute = $false
  $startInfo.CreateNoWindow = $true
  $startInfo.RedirectStandardOutput = $true
  $startInfo.RedirectStandardError = $true
  $process = [System.Diagnostics.Process]::Start($startInfo)
  # mklink's messages are short, so reading one stream to the end cannot
  # block on the other.
  $errText = $process.StandardError.ReadToEnd()
  [void]$process.StandardOutput.ReadToEnd()
  $process.WaitForExit()
  if ($process.ExitCode -eq 0) { return '' }
  $message = ($errText -replace '\s+', ' ').Trim()
  if ($message -eq '') { $message = 'mklink failed' }
  return "$message (mklink exit $($process.ExitCode))"
}

# Takes the plain staging path of a link, its target and whether it is a
# directory link; makes the link and records it in $script:StagedLinks. Returns
# '' or the reason.
function Invoke-CreateSymlink([string]$linkPath, [string]$target, [bool]$isDir) {
  $api = Get-LinkApi
  $extendedLinkPath = ConvertTo-ExtendedPath $linkPath
  try {
    if ($api -eq 'dotnet') {
      $method = $script:LinkFileMethod; if ($isDir) { $method = $script:LinkDirMethod }
      # Plain strings: Invoke rejects PSObject-wrapped arguments.
      $methodArgs = New-Object 'object[]' 2
      $methodArgs[0] = [string]$extendedLinkPath; $methodArgs[1] = [string]$target
      [void]$method.Invoke($null, $methodArgs)
    } elseif ($api -eq 'mklink') {
      $why = Invoke-Mklink $extendedLinkPath $target $isDir
      if ($why -ne '') { return (ConvertFrom-ExtendedText $why) }
    } else { return $script:LinkApiError }
  } catch {
    # Invoke wraps the IOException; report the innermost message.
    $exception = $_.Exception
    while ($null -ne $exception.InnerException) { $exception = $exception.InnerException }
    return (ConvertFrom-ExtendedText $exception.Message)
  }
  [void]$script:StagedLinks.Add(@($linkPath, $isDir, $target))
  return ''
}

# Takes a reparse point's Get-Item and its plain staging path; recreates the
# link and returns '' or the row's error, counting it in $script:LinkCounts.
function Add-StagedLink($item, [string]$stagePath) {
  if (-not $script:LinksInArchive) {
    $script:LinkCounts.zip++
    return 'not recreated in the archive: zip cannot store symlinks'
  }
  $targets = @()
  try {
    $targetProperty = $item.PSObject.Properties['Target']
    if ($targetProperty -and $targetProperty.Value) {
      $targets = @([string[]]$targetProperty.Value)
    }
  } catch { }
  if ($targets.Count -ne 1 -or $targets[0] -eq '') {
    $script:LinkCounts.failed++
    return 'not recreated in the archive: reparse point has no single link target'
  }
  # Only Windows distinguishes file and directory links.
  $isDir = ($env:OS -eq 'Windows_NT') -and
    (($item.Attributes -band [System.IO.FileAttributes]::Directory) -ne 0)
  $why = ''
  try {
    $parentDir = [System.IO.Path]::GetDirectoryName($stagePath)
    if (-not (Test-PathQuiet $parentDir 'Any')) {
      [void][System.IO.Directory]::CreateDirectory((ConvertTo-ExtendedPath $parentDir))
    }
    $why = Invoke-CreateSymlink $stagePath $targets[0] $isDir
  } catch { $why = ConvertFrom-ExtendedText $_.Exception.Message }
  if ($why -eq '') { return '' }
  $script:LinkCounts.failed++
  Write-CollectorLog "symlink not recreated: $(ConvertFrom-ExtendedPath $item.FullName): $why"
  return "not recreated in the archive: $why"
}

# Takes the user, home, agent and the file's Get-Item; stages it and writes its
# manifest row, or under -Inventory feeds Add-InventoryFile. Runs once per
# file, so Join-PathSafe and Get-Epoch are inlined and nothing here starts a
# pipeline or another process.
function Add-CollectedFile([string]$user, [string]$homeDir, [string]$agent, $item) {
  if ($Inventory) {
    # Regular files only: links and special files are neither followed nor counted.
    if ((Get-ManifestType $item) -eq 'file' -and -not $item.PSIsContainer) {
      Add-InventoryFile $agent $item
    }
    return
  }
  # $extendedPath opens past 260 characters and $path is what the row records.
  $extendedPath = $item.FullName
  $path = ConvertFrom-ExtendedPath $extendedPath
  $homeRel = Get-RelPath $homeDir $path
  $archiveRel = Get-ArchivePath $path
  $stageRel = $script:Sep + $archiveRel.Replace('\', $script:Sep).Replace('/', $script:Sep)
  $stagePath = $script:Stage + $stageRel; $extendedStagePath = $script:XStage + $stageRel
  $size = [int64]0; if (-not $item.PSIsContainer) { try { $size = [int64]$item.Length } catch { } }
  try {
    $mtime = [DateTimeOffset]::new($item.LastWriteTimeUtc).ToUnixTimeSeconds()
    $atime = [DateTimeOffset]::new($item.LastAccessTimeUtc).ToUnixTimeSeconds()
    $btime = [DateTimeOffset]::new($item.CreationTimeUtc).ToUnixTimeSeconds()
  } catch {
    $mtime = Get-Epoch $item.LastWriteTimeUtc; $atime = Get-Epoch $item.LastAccessTimeUtc
    $btime = Get-Epoch $item.CreationTimeUtc
  }
  $owner = Get-OwnerName $path
  $attributes = [string]$item.Attributes
  $secret = $script:SecretRegex.IsMatch($homeRel)
  $type = Get-ManifestType $item
  $status = 'collected'; $sha256 = ''; $target = ''; $errorText = ''
  if ($type -eq 'symlink') {
    $status = 'symlink'; $target = Get-LinkTarget $item
    $errorText = Add-StagedLink $item $stagePath
  } elseif ($type -ne 'file') {
    $status = 'skipped_special'
  } elseif ($secret -and $NoSecrets) {
    $status = 'skipped_secret'
  } elseif ($script:MaxSize -gt 0 -and $size -gt $script:MaxSize) {
    $status = 'skipped_size'
  } else {
    try {
      $copy = Copy-FileShared $extendedPath $extendedStagePath $item
      $sha256 = [string]$copy[0]; $size = [int64]$copy[1]
    } catch {
      $status = 'error_copy'; $errorText = ConvertFrom-ExtendedText $_.Exception.Message
      Write-CollectorLog "copy failed: ${path}: $errorText"
      try {
        if (Test-PathQuiet $extendedStagePath 'Any') {
          Remove-Item -LiteralPath $extendedStagePath -Force
        }
      } catch { }
    }
  }
  $archivePath = ''
  if ($status -eq 'collected' -or ($status -eq 'symlink' -and $errorText -eq '')) {
    $archivePath = $archiveRel
  }
  $row = @($user, $homeDir, $agent, $path, $archivePath, $type, $size, $mtime, $atime, 0, $btime,
    $owner, $attributes, $sha256, $secret, $status, $target, $errorText)
  Write-ManifestRow @row
}

# Takes the user, home, agent and an excluded path's Get-Item; unless
# -Inventory, writes its skipped_excluded row.
function Add-ExcludedPath([string]$user, [string]$homeDir, [string]$agent, $item) {
  if ($Inventory) { return }
  $path = ConvertFrom-ExtendedPath $item.FullName
  if ($item.PSIsContainer) { $type = 'dir'; $size = Get-DirSize $path }
  else { $type = 'file'; $size = [int64]0; try { $size = [int64]$item.Length } catch { } }
  $mtime = Get-Epoch $item.LastWriteTimeUtc
  $atime = Get-Epoch $item.LastAccessTimeUtc
  $btime = Get-Epoch $item.CreationTimeUtc
  $row = @($user, $homeDir, $agent, $path, '', $type, $size, $mtime, $atime, 0, $btime, '',
    ([string]$item.Attributes), '', $false, 'skipped_excluded', '', '')
  Write-ManifestRow @row
}

# ===========================================================================
# Catalog walk
# ===========================================================================
# A table is expanded in full before anything is walked, so a nested entry
# (.gemini/antigravity-cli in .gemini) claims its subtree from the outer walk.

# Returns a new table for $script:ClaimedPaths: case-insensitive on Windows,
# ordinal elsewhere, so pwsh on Linux keeps .config/Cursor and .config/cursor
# apart.
function Get-ClaimTable {
  if ($env:OS -eq 'Windows_NT') { return @{} }
  return (New-Object 'System.Collections.Generic.Dictionary[string,bool]')
}
$script:ClaimedPaths = Get-ClaimTable
# Paths that could not be walked since the counter was last reset; the
# Docker volume step reads it per volume.
$script:WalkErrors = 0
# Directories a wildcard segment could not list (path -> message), filled by
# Expand-Glob and read by Expand-Catalog.
$script:GlobReadErrors = [ordered]@{}

# Takes a directory and a literal catalog segment; returns the segment's on-disk
# spelling, so two entries that differ only by case expand to one path on a
# case-insensitive filesystem; the segment as written when that is ambiguous.
function Get-OnDiskName([string]$dir, [string]$segment) {
  try {
    $extendedDir = ConvertTo-ExtendedPath $dir
    $found = @([System.IO.Directory]::GetFileSystemEntries($extendedDir, $segment))
    if ($found.Count -eq 1) {
      $name = [System.IO.Path]::GetFileName($found[0])
      $ignoreCase = [System.StringComparison]::OrdinalIgnoreCase
      if ([string]::Equals($name, $segment, $ignoreCase)) { return $name }
    }
  } catch { }
  return $segment
}

# Takes a base directory and a catalog glob; returns the full paths it matches,
# in on-disk spelling. A directory a wildcard segment cannot list is recorded in
# $script:GlobReadErrors.
function Expand-Glob([string]$base, [string]$pattern) {
  $current = @($base)
  foreach ($segment in ($pattern -split '/')) {
    if ($segment -eq '') { continue }
    $next = @()
    $isWild = ($segment -match '[\*\?\[]')
    foreach ($dir in $current) {
      if ($isWild) {
        $segmentRegex = Convert-GlobToRegex $segment $false
        $extendedDir = ConvertTo-ExtendedPath $dir
        try { $children = Get-ChildItem -LiteralPath $extendedDir -Force -ErrorAction Stop }
        catch {
          $children = @()
          if (-not $script:GlobReadErrors.Contains($dir)) {
            $script:GlobReadErrors[$dir] = (ConvertFrom-ExtendedText $_.Exception.Message)
          }
        }
        foreach ($child in $children) {
          if ($child.Name -match $segmentRegex) {
            $next += (ConvertFrom-ExtendedPath $child.FullName)
          }
        }
      } else {
        $candidate = Join-PathSafe $dir $segment
        if (Test-PathQuiet $candidate 'Any') {
          $next += (Join-PathSafe $dir (Get-OnDiskName $dir $segment))
        }
      }
    }
    $current = $next
    if ($current.Count -eq 0) { break }
  }
  if ($current.Count -gt 0 -and $current[0] -eq $base) { return @() }
  return $current
}

# Takes a base directory and a table; returns @{ Entries; GlobErrors }, every
# match as @(agent, path, glob), claimed in $script:ClaimedPaths. A path an
# earlier line matched is listed with an empty agent, so the caller skips it.
function Expand-Catalog([string]$base, [string]$table) {
  $entries = New-Object System.Collections.ArrayList
  $script:ClaimedPaths = Get-ClaimTable
  $globErrors = [ordered]@{}
  foreach ($line in (Get-TableLines $table)) {
    $parts = @($line -split '\|', 2)
    if ($parts.Count -lt 2) { continue }
    $agent = $parts[0]; $pattern = $parts[1]
    $script:GlobReadErrors = [ordered]@{}
    foreach ($matchPath in @(Expand-Glob $base $pattern)) {
      if ($script:ClaimedPaths.ContainsKey($matchPath)) {
        [void]$entries.Add(@('', $matchPath, "$agent|$pattern")); continue
      }
      [void]$entries.Add(@($agent, $matchPath, $pattern))
      $script:ClaimedPaths[$matchPath] = $true
    }
    foreach ($errorDir in @($script:GlobReadErrors.Keys)) {
      if (-not $globErrors.Contains($errorDir)) {
        $globErrors[$errorDir] = @($agent, $script:GlobReadErrors[$errorDir])
      }
    }
  }
  $script:GlobReadErrors = [ordered]@{}
  return @{ Entries = $entries; GlobErrors = $globErrors }
}

# Takes the user, base and the GlobErrors of Expand-Catalog; writes an
# error_read row for each directory not already covered by the base or a match.
# Counts $script:WalkErrors.
function Write-GlobErrorRows([string]$user, [string]$base, $globErrors) {
  $ignoreCase = [System.StringComparison]::OrdinalIgnoreCase
  foreach ($errorDir in @($globErrors.Keys)) {
    if ($errorDir.TrimEnd('\', '/') -eq $base.TrimEnd('\', '/')) { continue }
    $underMatch = $false
    foreach ($claimedPath in $script:ClaimedPaths.Keys) {
      $isUnder = $errorDir.StartsWith($claimedPath + $script:Sep, $ignoreCase)
      if ($errorDir -eq $claimedPath -or $isUnder) { $underMatch = $true; break }
    }
    if ($underMatch) { continue }
    $script:WalkErrors++
    Write-ReadErrorRow $user $base $globErrors[$errorDir][0] $errorDir $globErrors[$errorDir][1]
  }
}

# Takes the user, a base directory, a table and an optional progress header;
# collects every match of the table under the base, logging the header before
# the first one.
function Add-CatalogMatches([string]$user, [string]$base, [string]$table, [string]$header = '') {
  $expansion = Expand-Catalog $base $table
  Write-GlobErrorRows $user $base $expansion.GlobErrors
  foreach ($entry in $expansion.Entries) {
    $agent = $entry[0]; $matchPath = $entry[1]
    if (-not $agent) {
      if ($header) { Write-CollectorLog $header; $header = '' }
      $duplicate = @($entry[2] -split '\|', 2)
      $lineText = "catalog line $($duplicate[1]) names the same path as an earlier line"
      Write-CollectorLog ("  [$($duplicate[0])] ${matchPath}: $lineText" +
        ' (case-insensitive filesystem), collected there')
      continue
    }
    if ($Inventory) {
      if ($agent -eq 'shared') { continue }
      Add-InventoryEvidence $agent $entry[2]
    }
    if ($header) { Write-CollectorLog $header; $header = '' }
    Write-CollectorLog "  [$agent] $matchPath"
    Add-CollectedPath $user $base $agent $matchPath
  }
  $script:ClaimedPaths = Get-ClaimTable
}

# Takes the user, home, agent and a matched path; collects it, counting a path
# that cannot be read in $script:WalkErrors.
function Add-CollectedPath([string]$user, [string]$homeDir, [string]$agent, [string]$path) {
  $item = $null
  try { $item = Get-Item -LiteralPath (ConvertTo-ExtendedPath $path) -Force -ErrorAction Stop }
  catch {
    $script:WalkErrors++
    Write-CollectorLog "stat failed: ${path}: $(ConvertFrom-ExtendedText $_.Exception.Message)"
    return
  }
  $homeRel = Get-RelPath $homeDir $path
  if ($script:ExcludeRegex.IsMatch($homeRel)) {
    Add-ExcludedPath $user $homeDir $agent $item; return
  }
  $isLink = (($item.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0)
  if ($item.PSIsContainer -and -not $isLink) { Add-CollectedTree $user $homeDir $agent $path }
  else { Add-CollectedFile $user $homeDir $agent $item }
}

# Takes the user, home, agent and a directory; walks it depth first, skipping
# claimed children and pruning excluded ones. It lists the extended form, so a
# tree past 260 characters is walked without LongPathsEnabled.
function Add-CollectedTree([string]$user, [string]$homeDir, [string]$agent, [string]$dir) {
  $children = @()
  $extendedDir = ConvertTo-ExtendedPath $dir
  try { $children = @(Get-ChildItem -LiteralPath $extendedDir -Force -ErrorAction Stop) }
  catch {
    $script:WalkErrors++
    Write-ReadErrorRow $user $homeDir $agent $dir (ConvertFrom-ExtendedText $_.Exception.Message)
    return
  }
  foreach ($child in $children) {
    $childPath = ConvertFrom-ExtendedPath $child.FullName
    if ($script:ClaimedPaths.ContainsKey($childPath)) { continue }
    $homeRel = Get-RelPath $homeDir $childPath
    if ($script:ExcludeRegex.IsMatch($homeRel)) {
      Add-ExcludedPath $user $homeDir $agent $child; continue
    }
    $isLink = (($child.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0)
    if ($child.PSIsContainer -and -not $isLink) {
      Add-CollectedTree $user $homeDir $agent $childPath
    }
    else { Add-CollectedFile $user $homeDir $agent $child }
  }
}

# ===========================================================================
# Users
# ===========================================================================
# Every profile directory is a base for CATALOG. Each home is a hashtable
# @{ user; home }; $script:UserList holds them in enumeration order.

# Profile names that are never a person's home.
$script:SkipHomes = @('Public', 'Default', 'Default User', 'All Users', 'Shared', 'defaultuser0')
# Transient Windows service profiles: font driver hosts, desktop window
# manager, TEMP.
$script:SkipHomeRe = '^(UMFD-\d+|DWM-\d+|TEMP)(\..*)?$'
# Homes that were listed but could not be entered, in the plain form.
$script:InaccessibleHomes = @()
# The inaccessible homes whose parent lists them (access denied), set by
# Add-UserHomes.
$script:DeniedHomes = @()
$script:UserList = @()

# Returns every candidate home as @{ user; home }, duplicates included, from
# the registry's ProfileList and the usual home roots.
function Get-CandidateHomes {
  $candidates = @()
  if ($script:Mode -eq 'live') {
    $profileList = 'HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion\ProfileList'
    try {
      foreach ($key in (Get-ChildItem $profileList -ErrorAction Stop)) {
        $properties = Get-ItemProperty -LiteralPath $key.PSPath -ErrorAction SilentlyContinue
        $profilePath = $properties.ProfileImagePath
        if ($profilePath) {
          $profilePath = [Environment]::ExpandEnvironmentVariables($profilePath)
          $candidates += @{ user = (Split-Path -Leaf $profilePath); home = $profilePath }
        }
      }
    } catch { }
    $drive = $env:SystemDrive; if (-not $drive) { $drive = 'C:' }
    $driveRoot = $drive + $script:Sep; if ($script:Sep -eq '/') { $driveRoot = '/' }
    $roots = @('Users/*', 'home/*'); $base = $driveRoot
  } else {
    $roots = @('Users/*', 'home/*', 'root', 'var/root', 'usr/home/*', 'export/home/*')
    $base = $Root
  }
  foreach ($glob in $roots) {
    foreach ($homeDir in (Expand-Glob $base $glob)) {
      $candidates += @{ user = (Split-Path -Leaf $homeDir); home = $homeDir }
    }
  }
  return $candidates
}

# Returns the homes to collect as @{ user; home }, in enumeration order, each
# once; a home .NET cannot see is added to $script:InaccessibleHomes.
function Get-UserHomes {
  $wanted = @(); if ($Users -ne '') { $wanted = @($Users -split ',' | Where-Object { $_ -ne '' }) }
  $seen = @{}; $selected = @()
  foreach ($candidate in (Get-CandidateHomes)) {
    $homePath = $candidate.home.TrimEnd('\', '/')
    if ($script:SkipHomes -contains $candidate.user -or $candidate.user -match $script:SkipHomeRe) {
      continue
    }
    if ([System.IO.File]::Exists((ConvertTo-ExtendedPath $homePath))) { continue }
    if ($wanted.Count -gt 0 -and -not ($wanted -contains $candidate.user)) { continue }
    if (-not (Test-PathQuiet $homePath 'Container')) {
      if (-not ($script:InaccessibleHomes -contains $homePath)) {
        $script:InaccessibleHomes += $homePath
      }
      continue
    }
    $key = $homePath.ToLowerInvariant()
    if ($seen.ContainsKey($key)) { continue }
    $seen[$key] = $true
    $selected += @{ user = $candidate.user; home = $homePath }
  }
  return $selected
}

# Returns the inaccessible homes that are denied rather than gone: their parent
# directory lists them.
function Get-DeniedHomes {
  foreach ($homePath in $script:InaccessibleHomes) {
    $leaf = Split-Path -Leaf $homePath; $parent = Split-Path -Parent $homePath
    try {
      $listed = @([System.IO.Directory]::GetDirectories((ConvertTo-ExtendedPath $parent), $leaf))
      if ($listed.Count -gt 0) { $homePath }
    } catch { }
  }
}

# Main step: collects CATALOG under every home in $script:UserList, with an
# error_read row for each home that cannot be walked. Sets $script:DeniedHomes
# and, under -Inventory, $script:InventoryUsers.
function Add-UserHomes {
  $script:DeniedHomes = @(Get-DeniedHomes)
  if (-not $Inventory) {
    foreach ($deniedHome in $script:DeniedHomes) {
      $why = 'access denied'
      $extendedHome = ConvertTo-ExtendedPath $deniedHome
      try { [void](Get-Item -LiteralPath $extendedHome -Force -ErrorAction Stop) }
      catch { $why = ConvertFrom-ExtendedText $_.Exception.Message }
      Write-ReadErrorRow (Split-Path -Leaf $deniedHome) $deniedHome '' $deniedHome $why
    }
  }
  foreach ($userHome in $script:UserList) {
    $script:InventoryAgents = [ordered]@{}
    if (-not $Inventory) {
      $why = Get-DirReadError $userHome.home
      if ($why) { Write-ReadErrorRow $userHome.user $userHome.home '' $userHome.home $why }
    }
    $header = "User $($userHome.user) ($($userHome.home))"
    Add-CatalogMatches $userHome.user $userHome.home $CATALOG $header
    if ($Inventory) { $script:InventoryUsers += , @($userHome, $script:InventoryAgents) }
  }
}

# ===========================================================================
# Docker and Podman volumes
# ===========================================================================
# Volumes are found on disk, as in the sh collector; internals.md, "Docker and
# Podman volumes", has the flow.

# What the volume step saw, for collection.json and the stdout docker line.
$script:Docker = @{ seen = $false; found = 0; collected = 0; unreadable = 0; desktop = $false }
# DOCKER_VOLUMES as @(agent, name regex, glob), in table order.
$script:DockerVolumeRules = @()
foreach ($line in (Get-TableLines $DOCKER_VOLUMES)) {
  $parts = @($line -split '\|', 2)
  if ($parts.Count -lt 2) { continue }
  $script:DockerVolumeRules += , @($parts[0], (Convert-GlobToRegex $parts[1] $false), $parts[1])
}
# The -Inventory accumulator for matched volumes, reported as user docker.
$script:InventoryDocker = [ordered]@{}

# Takes a volume name; returns the first DOCKER_VOLUMES rule (agent, regex, glob)
# matching it case-sensitively, as the sh case(1) does, or $null.
function Get-DockerVolumeRule([string]$name) {
  foreach ($rule in $script:DockerVolumeRules) { if ($name -cmatch $rule[1]) { return , $rule } }
  return $null
}

# Takes a base directory and a relative volumes path; returns the volumes
# directory when it can be listed, else '', noting a symlinked or unreadable
# component in $script:Docker.
function Test-VolumeRoot([string]$base, [string]$rel) {
  $volumesDir = $base
  $segments = @($rel -split '/')
  for ($i = 0; $i -lt $segments.Count; $i++) {
    $volumesDir = Join-PathSafe $volumesDir $segments[$i]
    if (Test-IsLink $volumesDir) {
      $script:Docker.seen = $true
      Add-Note "docker: $volumesDir is a symlink and was not followed; collect its target by hand"
      return ''
    }
    if (-not (Test-PathQuiet $volumesDir 'Container')) { return '' }
    $last = ($i -eq $segments.Count - 1)
    $nextVisible = (-not $last) -and
      (Test-PathQuiet (Join-PathSafe $volumesDir $segments[$i + 1]) 'Container')
    if (-not (Test-DirReadable $volumesDir) -and -not $nextVisible) {
      $script:Docker.seen = $true; $script:Docker.unreadable++
      Add-Note "docker: $volumesDir is not readable; run as root to collect the volumes under it"
      return ''
    }
  }
  return $volumesDir
}

# Takes a base directory and a relative volumes path; gives every volume
# directory in it to Add-Volume, counting each in $script:Docker.found.
function Add-VolumeDir([string]$base, [string]$rel) {
  $volumesDir = Test-VolumeRoot $base $rel
  if (-not $volumesDir) { return }
  $script:Docker.seen = $true
  $listing = @{ LiteralPath = (ConvertTo-ExtendedPath $volumesDir); Force = $true }
  $children = @(Get-ChildItem @listing -ErrorAction SilentlyContinue)
  Write-CollectorLog "Docker volumes in $volumesDir"
  foreach ($volume in $children) {
    if (-not $volume.PSIsContainer) { continue }
    if (($volume.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0) { continue }
    $script:Docker.found++
    Add-Volume $volume
  }
}

# Takes the _data directory of an unmatched volume; writes its
# skipped_unmatched_volume row.
function Write-UnmatchedVolumeRow([string]$data) {
  $attributes = ''
  try {
    $dataItem = Get-Item -LiteralPath (ConvertTo-ExtendedPath $data) -Force -ErrorAction Stop
    $attributes = [string]$dataItem.Attributes
  } catch { }
  $row = @('docker', $data, '', $data, '', 'dir', (Get-DirSize $data), 0, 0, 0, 0, '',
    $attributes, '', $false, 'skipped_unmatched_volume', '', '')
  Write-ManifestRow @row
}

# Takes one volume directory's entry; collects a matched volume's _data, or
# records an unmatched or unreadable one.
function Add-Volume($volume) {
  $name = $volume.Name
  $volumePath = ConvertFrom-ExtendedPath $volume.FullName
  $data = Join-PathSafe $volumePath '_data'
  $rule = Get-DockerVolumeRule $name
  $agent = ''; if ($rule) { $agent = $rule[0] }
  $label = $agent; if (-not $label) { $label = 'unmatched' }
  $dataIsDir = Test-PathQuiet $data 'Container'
  if (-not (Test-DirReadable $volumePath) -or ($dataIsDir -and -not (Test-DirReadable $data))) {
    $script:Docker.unreadable++
    Add-Note ("docker: volume $name ($label) at $volumePath is not readable; " +
      'run as root to collect it')
    return
  }
  if ((Test-IsLink $data) -or -not $dataIsDir) {
    Write-CollectorLog "  volume $name has no _data directory, skipped"
    return
  }
  if (-not $agent) {
    if (-not $Inventory) { Write-UnmatchedVolumeRow $data }
    return
  }
  Write-CollectorLog "  [$agent] $data"
  $script:WalkErrors = 0
  $script:ClaimedPaths = Get-ClaimTable
  if ($Inventory) {
    $savedAgents = $script:InventoryAgents; $script:InventoryAgents = $script:InventoryDocker
    Add-InventoryEvidence $agent $rule[2]
    Add-CollectedPath 'docker' $data $agent $data
    $script:InventoryAgents = $savedAgents
  } else {
    Add-CollectedPath 'docker' $data $agent $data
  }
  $script:Docker.collected++
  if ($script:WalkErrors -gt 0) {
    $script:Docker.unreadable++
    Add-Note ("docker: volume $name ($agent) could not be walked completely " +
      "($($script:WalkErrors) unreadable paths, see collector.log); run as root to collect it")
  }
}

# Takes the Docker Desktop data directories; notes each that exists, since its
# volumes are inside a VM disk, and sets $script:Docker.desktop.
function Find-DockerDesktop([string[]]$desktopDirs) {
  foreach ($desktopDir in $desktopDirs) {
    if ((Test-PathQuiet $desktopDir 'Container') -and -not (Test-IsLink $desktopDir)) {
      $script:Docker.seen = $true; $script:Docker.desktop = $true
      Add-Note ("docker: Docker Desktop data at $desktopDir; volumes inside its virtual " +
        'machine disk are not collected')
    }
  }
}

# Main step: collects the volumes of the system and per-user Docker and Podman
# data roots, then notes Docker Desktop data.
function Add-DockerVolumes {
  if ($script:Mode -eq 'image') { $dockerBase = $Root }
  elseif ($script:Sep -eq '/') { $dockerBase = '/' }
  else {
    $dockerBase = $env:SystemDrive; if (-not $dockerBase) { $dockerBase = 'C:' }
    $dockerBase += $script:Sep
  }
  $programData = Join-PathSafe $dockerBase 'ProgramData'
  if ($script:Mode -eq 'live' -and $env:ProgramData) { $programData = $env:ProgramData }
  $varLib = Join-PathSafe $dockerBase 'var/lib'
  Add-VolumeDir $varLib 'docker/volumes'
  Add-VolumeDir $varLib 'containers/storage/volumes'
  Add-VolumeDir $programData 'Docker/volumes'
  $desktopDirs = @(Join-PathSafe $programData 'DockerDesktop')
  foreach ($userHome in $script:UserList) {
    $localShare = Join-PathSafe $userHome.home '.local/share'
    Add-VolumeDir $localShare 'docker/volumes'
    Add-VolumeDir $localShare 'containers/storage/volumes'
    $desktopDirs += (Join-PathSafe $userHome.home 'Library/Containers/com.docker.docker')
    $desktopDirs += (Join-PathSafe $userHome.home 'AppData/Local/Docker')
  }
  Find-DockerDesktop $desktopDirs
}

# ===========================================================================
# Project discovery
# ===========================================================================
# Project paths are read from the agents' plain-text state with a regex per
# format, in the sh collector's order; internals.md, "Project discovery", lists
# the groups.

$script:ProjectList = @()
# The home whose sources are being read, for Test-InventorySecret.
$script:DiscoveryHome = ''

# Takes a value read from a discovery source; returns it as a local directory
# path (file:// URIs unescaped, under -Root in image mode), or $null when it is
# not an absolute path this host or image can hold.
function ConvertTo-LocalPath([string]$value) {
  $value = $value.Trim()
  if ($value -eq '') { return $null }
  if ($value.StartsWith('file://')) {
    $value = $value.Substring(7); try { $value = [uri]::UnescapeDataString($value) } catch { }
    if ($value -match '^/[A-Za-z]:') { $value = $value.Substring(1) }
  }
  $value = $value.Replace('\\', '\').Replace('\/', '/')
  if ($value -match '^[A-Za-z]:[\\/]') {
    $rest = $value.Substring(1).Replace('/', $script:Sep).Replace('\', $script:Sep)
    $value = $value.Substring(0, 1).ToUpper() + $rest
    if ($script:Mode -eq 'image') { return $Root + $value.Substring(2) }
    return $value
  }
  if ($value.StartsWith('/')) {
    if ($script:Mode -eq 'image') { return $Root + $value.Replace('/', $script:Sep) }
    if ($script:Sep -eq '/') { return $value }
    return $null
  }
  return $null
}

# Takes a discovery source path; returns whether -Inventory must skip it as a
# credential file.
function Test-InventorySecret([string]$file) {
  if (-not $Inventory -or -not $script:DiscoveryHome) { return $false }
  return $script:SecretRegex.IsMatch((Get-RelPath $script:DiscoveryHome $file))
}

# Takes a discovery source path; returns its text, or $null. It opens the file
# with shared access, since File.ReadAllText refuses a file a running agent
# holds open for writing.
function Read-DiscoverySource([string]$file) {
  if (-not $file -or -not (Test-PathQuiet $file 'Leaf')) { return $null }
  if (Test-InventorySecret $file) { return $null }
  $stream = $null; $reader = $null
  try {
    $stream = New-Object System.IO.FileStream((ConvertTo-ExtendedPath $file),
      [System.IO.FileMode]::Open, [System.IO.FileAccess]::Read, $script:ShareReadWriteDelete)
    $reader = New-Object System.IO.StreamReader($stream, [System.Text.Encoding]::UTF8, $true)
    return $reader.ReadToEnd()
  } catch {
    $errorText = ConvertFrom-ExtendedText $_.Exception.Message
    Write-CollectorLog "discovery source unreadable: ${file}: $errorText"
    return $null
  } finally {
    if ($reader) { $reader.Dispose() } elseif ($stream) { $stream.Dispose() }
  }
}

# Takes a regex and source files; returns capture group 1 of every match, in
# file order.
function Get-RegexCaptures([string]$regex, [string[]]$files) {
  foreach ($file in $files) {
    $text = Read-DiscoverySource $file
    if ($null -eq $text) { continue }
    foreach ($match in [regex]::Matches($text, $regex)) { $match.Groups[1].Value }
  }
}

# Takes a JSON key and source files; returns every string value of that key.
function Get-JsonValues([string]$key, [string[]]$files) {
  Get-RegexCaptures ('"' + [regex]::Escape($key) + '"\s*:\s*"((?:[^"\\]|\\.)*)"') $files
}

# Takes a regex for what follows the colon and source files; returns every
# JSON key that is an absolute or drive-letter path (projects keyed by path).
function Get-JsonKeys([string]$suffix, [string[]]$files) {
  Get-RegexCaptures ('"([A-Za-z]:(?:[^"\\]|\\.)*|/(?:[^"\\]|\\.)*)"\s*:\s*' + $suffix) $files
}

# Takes source files; returns every quoted absolute path, drive-letter path
# or file:// URI in them.
function Get-PathStrings([string[]]$files) {
  Get-RegexCaptures '"((?:file:///|/|[A-Za-z]:\\\\)(?:[^"\\]|\\.)*)"' $files
}

# Takes a YAML key and source files; returns the value of every top-level
# "key: value" line, without quotes.
function Get-YamlValues([string]$key, [string[]]$files) {
  $regex = '(?m)^' + [regex]::Escape($key) + ':[ \t]*["'']?([^"''\r\n]+)["'']?[ \t]*$'
  Get-RegexCaptures $regex $files
}

# Takes a key and source files; returns every string value of that key in
# JSON5, whose keys may be unquoted and strings single-quoted.
function Get-Json5Values([string]$key, [string[]]$files) {
  $regex = '(?<![\w$])["'']?' + [regex]::Escape($key) +
    '["'']?\s*:\s*(?:"((?:[^"\\]|\\.)*)"|''([^'']*)'')'
  foreach ($file in $files) {
    $text = Read-DiscoverySource $file
    if ($null -eq $text) { continue }
    foreach ($match in [regex]::Matches($text, $regex)) {
      if ($match.Groups[1].Success) { $match.Groups[1].Value } else { $match.Groups[2].Value }
    }
  }
}

# Takes a TOML key and source files; returns every key = "file://..." value
# (Tabby repositories).
function Get-TomlFileUrls([string]$key, [string[]]$files) {
  $regex = '(?m)^[ \t]*' + [regex]::Escape($key) + '[ \t]*=[ \t]*["''](file://[^"''\r\n]*)["'']'
  Get-RegexCaptures $regex $files
}

# Takes source files that each hold one path; returns the text of each readable
# one, under -Inventory too.
function Get-FileTexts([string[]]$files) {
  foreach ($file in $files) {
    try { [System.IO.File]::ReadAllText((ConvertTo-ExtendedPath $file)) } catch { }
  }
}

# Takes absolute file paths; returns the deepest directory they share, or $null
# for fewer than two shared segments.
function Get-CommonDir([string[]]$paths) {
  $common = $null
  foreach ($value in $paths) {
    $path = ([string]$value).Replace('\\', '\').Replace('\/', '/')
    if ($path.StartsWith('file://')) { continue }
    $parts = @($path -split '[\\/]')
    if ($parts.Count -lt 3) { continue }
    $parts = @($parts[0..($parts.Count - 2)])
    if ($null -eq $common) { $common = $parts; continue }
    $limit = [Math]::Min($common.Count, $parts.Count); $shared = 0
    while ($shared -lt $limit -and $common[$shared] -eq $parts[$shared]) { $shared++ }
    if ($shared -eq 0) { $common = @() } else { $common = @($common[0..($shared - 1)]) }
  }
  if ($null -eq $common -or $common.Count -lt 2) { return $null }
  return ($common -join '/')
}

# Takes a -Project directory; returns the user whose home contains it, or ''.
function Get-ProjectOwner([string]$projectDir) {
  $ignoreCase = [System.StringComparison]::OrdinalIgnoreCase
  foreach ($userHome in $script:UserList) {
    if ($projectDir.StartsWith($userHome.home + $script:Sep, $ignoreCase)) { return $userHome.user }
  }
  return ''
}

# The six source groups below each take a home and return the raw values
# they read, in source order; Find-Projects calls them in this order.

# Claude Code: the prompt history and .claude.json, also in config homes
# moved with CLAUDE_CONFIG_DIR (.claude-*), which keep .claude.json inside.
function Find-ClaudeCodeProjects([string]$homeDir) {
  Get-JsonValues 'project' @((Join-PathSafe $homeDir '.claude/history.jsonl'))
  Get-JsonKeys '\{' @((Join-PathSafe $homeDir '.claude.json'))
  Get-JsonValues 'project' @(Expand-Glob $homeDir '.claude-*/history.jsonl')
  Get-JsonKeys '\{' @(Expand-Glob $homeDir '.claude-*/.claude.json')
}

# Group 2 of the internals.md discovery table: the CLI agents' session files.
function Find-CliSessionProjects([string]$homeDir) {
  # The ChatGPT desktop app's projectless directories (Documents/Codex, collected whole) and
  # Codex thread worktrees (excluded) are not projects.
  $codexOutputDir = Join-PathSafe $homeDir 'Documents/Codex'
  $codexWorktrees = (Join-PathSafe $homeDir '.codex/worktrees') + $script:Sep
  $ignoreCase = [System.StringComparison]::OrdinalIgnoreCase
  $codexRollouts = @(
    Expand-Glob $homeDir '.codex/sessions/*/*/*/*.jsonl'
    Expand-Glob $homeDir '.codex/archived_sessions/*.jsonl'
    Expand-Glob $homeDir '.codex/archived_sessions/*/*/*/*.jsonl'
  )
  Get-JsonValues 'cwd' $codexRollouts |
    Where-Object {
      $cwdPath = [string](ConvertTo-LocalPath $_)
      -not ($cwdPath.Equals($codexOutputDir, $ignoreCase) -or
        $cwdPath.StartsWith($codexOutputDir + $script:Sep, $ignoreCase) -or
        $cwdPath.StartsWith($codexWorktrees, $ignoreCase))
    }
  Get-JsonValues 'cwd' @(Expand-Glob $homeDir '.qwen/projects/*/chats/*.jsonl')
  Get-JsonValues 'cwd' @(Expand-Glob $homeDir '.cline/data/sessions/*/*.json')
  Get-JsonValues 'workspace_root' @(Expand-Glob $homeDir '.cline/data/sessions/*/*.json')
  Get-JsonValues 'cwd' @(Expand-Glob $homeDir '.cursor/chats/*/*/meta.json')
  Get-JsonValues 'workspaceDirectory' @((Join-PathSafe $homeDir '.continue/sessions/sessions.json'))
  Get-JsonValues 'working_dir' @(
    Expand-Glob $homeDir '.local/share/goose/sessions/*.jsonl'
    Expand-Glob $homeDir 'AppData/Roaming/Block/goose/data/sessions/*.jsonl'
  )
  Get-JsonValues 'worktree' @(
    Expand-Glob $homeDir '.local/share/opencode/storage/project/*.json'
    Expand-Glob $homeDir '.local/share/kilo/storage/project/*.json'
  )
  Get-JsonValues 'canonical' @(Expand-Glob $homeDir '.local/share/kilo/memory/*/manifest.json')
  $crushRegistries = @('.local/share/crush', 'AppData/Local/crush') |
    ForEach-Object { Join-PathSafe $homeDir "$_/projects.json" }
  Get-JsonValues 'path' @($crushRegistries)
  $a0Chats = @(@('agent-zero/usr', 'agent-zero/*/usr', 'Desktop/agent-zero/usr') |
    ForEach-Object { Expand-Glob $homeDir "$_/chats/*/chat.json" })
  Get-JsonValues 'acp_cwd' $a0Chats
  $a0Lists = Get-RegexCaptures '"acp_additional_directories"\s*:\s*\[([^\]]*)\]' $a0Chats
  foreach ($list in $a0Lists) {
    foreach ($match in [regex]::Matches($list, '"((?:[^"\\]|\\.)*)"')) { $match.Groups[1].Value }
  }
}

# Gemini CLI (projects, trusted folders, per-project .project_root files)
# and Antigravity (CLI history, settings and project caches).
function Find-GeminiProjects([string]$homeDir) {
  Get-JsonKeys '"' @((Join-PathSafe $homeDir '.gemini/projects.json'))
  Get-JsonKeys '"' @((Join-PathSafe $homeDir '.gemini/trustedFolders.json'))
  Get-FileTexts @(
    Expand-Glob $homeDir '.gemini/tmp/*/.project_root'
    Expand-Glob $homeDir '.gemini/history/*/.project_root'
  )
  Get-JsonValues 'workspace' @((Join-PathSafe $homeDir '.gemini/antigravity-cli/history.jsonl'))
  Get-PathStrings @(
    Join-PathSafe $homeDir '.gemini/antigravity-cli/settings.json'
    Join-PathSafe $homeDir '.gemini/antigravity-cli/cache/projects.json'
    Expand-Glob $homeDir '.gemini/config/projects/*.json'
  )
}

# Part of group 4 of the internals.md discovery table: Goose's desktop recent
# directories, scheduled jobs' recipe directories and the workingDirs block list
# of each project file, one pair of surrounding quotes and trailing blanks
# removed.
function Find-GooseProjects([string]$homeDir) {
  Get-PathStrings @(
    Join-PathSafe $homeDir '.config/Goose/recent-dirs.json'
    Join-PathSafe $homeDir 'Library/Application Support/Goose/recent-dirs.json'
    Join-PathSafe $homeDir 'AppData/Roaming/Goose/recent-dirs.json'
  )
  Get-JsonValues 'recipe_base_dir' @(
    Join-PathSafe $homeDir '.local/share/goose/schedule.json'
    Join-PathSafe $homeDir 'AppData/Roaming/Block/goose/data/schedule.json'
  )
  $projectFiles = @(
    Expand-Glob $homeDir '.local/share/goose/projects/*.md'
    Expand-Glob $homeDir 'AppData/Roaming/Block/goose/data/projects/*.md'
  )
  $listRegex = '(?m)^workingDirs:[ \t]*\r?\n((?:- [^\r\n]*(?:\r?\n|$))+)'
  foreach ($list in (Get-RegexCaptures $listRegex $projectFiles)) {
    foreach ($match in [regex]::Matches($list, '(?m)^- (.*?)[ \t]*\r?$')) {
      $item = $match.Groups[1].Value
      $last = $item.Length - 1
      if ($last -ge 1 -and ($item[0] -eq [char]'"' -or $item[0] -eq [char]"'") -and
        $item[$last] -eq $item[0]) {
        $item = $item.Substring(1, $last - 1)
      }
      $item
    }
  }
}

# Group 4 of the internals.md discovery table: the other agents' session and
# state files.
function Find-AgentSessionProjects([string]$homeDir) {
  Get-JsonValues 'cwd' @(
    Join-PathSafe $homeDir '.local/share/amp/history.jsonl'
    Expand-Glob $homeDir '.factory/sessions/*/*.jsonl'
    Expand-Glob $homeDir '.factory/sessions/*.jsonl'
    Expand-Glob $homeDir '.kiro/sessions/cli/*.json'
    Expand-Glob $homeDir '.cursor/acp-sessions/*/meta.json'
  )
  Get-JsonValues 'root' @(Expand-Glob $homeDir '.kiro/workspace-roots/*/.trust-migration.json')
  Get-PathStrings @(
    Expand-Glob $homeDir '.kiro/sessions/*/sess_*/session.json'
  )
  Find-GooseProjects $homeDir
  # A cwd under .copilot (Copilot Desktop chats and worktrees) is not a project, nor is that
  # of a session without events.jsonl, which Copilot Desktop pre-created and never prompted.
  $copilotDir = (Join-PathSafe $homeDir '.copilot') + $script:Sep
  $ignoreCase = [System.StringComparison]::OrdinalIgnoreCase
  $promptedSessions = @(Expand-Glob $homeDir '.copilot/session-state/*/events.jsonl' |
    ForEach-Object { Join-PathSafe ([System.IO.Path]::GetDirectoryName($_)) 'workspace.yaml' })
  Get-YamlValues 'cwd' $promptedSessions |
    Where-Object { -not ([string](ConvertTo-LocalPath $_)).StartsWith($copilotDir, $ignoreCase) }
  Get-JsonValues 'workdir' @(
    Expand-Glob $homeDir '.hermes/checkpoints/store/projects/*.json'
    Expand-Glob $homeDir '.hermes/profiles/*/checkpoints/store/projects/*.json'
    Expand-Glob $homeDir 'AppData/Local/hermes/checkpoints/store/projects/*.json'
    Expand-Glob $homeDir 'AppData/Local/hermes/profiles/*/checkpoints/store/projects/*.json'
  )
  Get-JsonValues 'project' @((Join-PathSafe $homeDir '.letta/sessions.jsonl'))
  Get-JsonValues 'cwd' @(
    Expand-Glob $homeDir '.letta/lc-local-backend/conversations/*/messages.jsonl'
    Expand-Glob $homeDir '.pi/agent/sessions/*/*.jsonl'
    Expand-Glob $homeDir '.pi/agent/*.jsonl'
    Join-PathSafe $homeDir '.pi/agent/crashes.json'
    Expand-Glob $homeDir '.pi/agent/experimental/sessions/*/meta.json'
  )
  Get-JsonKeys '(?:true|false)' @((Join-PathSafe $homeDir '.pi/agent/trust.json'))
  Get-JsonKeys '\{' @((Join-PathSafe $homeDir '.config/muse/trust.json'))
  $museSessions = '.local/share/muse/sessions/*/*/*/*/session.jsonl'
  Get-JsonValues 'workspace_root' @(Expand-Glob $homeDir $museSessions)
  # Open Interpreter thread worktrees (excluded) are not projects.
  $oiWorktrees = (Join-PathSafe $homeDir '.openinterpreter/worktrees') + $script:Sep
  Get-JsonValues 'cwd' @(
    Expand-Glob $homeDir '.openinterpreter/sessions/*/*/*/rollout-*.jsonl'
    Expand-Glob $homeDir '.openinterpreter/archived_sessions/rollout-*.jsonl'
    Expand-Glob $homeDir '.openinterpreter/archived_sessions/*/*/*/rollout-*.jsonl'
  ) | Where-Object {
    -not ([string](ConvertTo-LocalPath $_)).StartsWith($oiWorktrees, $ignoreCase)
  }
  Get-JsonValues 'path' @((Join-PathSafe $homeDir '.openhands/workspaces.json'))
  Get-JsonValues 'working_dir' @(
    Expand-Glob $homeDir '.openhands/agent-canvas/dev_conversations/*/meta.json'
    Expand-Glob $homeDir '.openhands/agent-canvas/conversations/*/meta.json'
    Expand-Glob $homeDir '.openhands/conversations/*/base_state.json'
  )
  Get-JsonValues 'workspaceDirectory' @((Join-PathSafe $homeDir '.pearai/sessions/sessions.json'))
}

# Configuration files that name a workspace: OpenClaw (and its Clawdbot and
# Moltbot configs), nanobot, Tabby repositories and Twinny embedding manifests.
function Find-ConfigFileProjects([string]$homeDir) {
  $openClawConfigs = @(
    Expand-Glob $homeDir '.openclaw*/openclaw.json'
    Join-PathSafe $homeDir '.clawdbot/openclaw.json'
    Join-PathSafe $homeDir '.clawdbot/clawdbot.json'
    Join-PathSafe $homeDir '.clawdbot/moltbot.json'
    Join-PathSafe $homeDir '.moltbot/moltbot.json'
  )
  foreach ($key in @('workspace', 'cwd', 'agentDir')) { Get-Json5Values $key $openClawConfigs }
  Get-FileTexts @(Expand-Glob $homeDir '.nanobot*/sessions/*/.workspace')
  Get-JsonValues 'workspace' @(Expand-Glob $homeDir '.nanobot*/config.json')
  Get-TomlFileUrls 'git_url' @((Join-PathSafe $homeDir '.tabby/config.toml'))
  foreach ($manifest in @(Expand-Glob $homeDir '.twinny/embeddings/*/manifest.json')) {
    $commonDir = Get-CommonDir @(Get-PathStrings @($manifest))
    if ($commonDir) { $commonDir }
  }
}

# VS Code-family workspaces and the Cline, Roo Code and Kilo Code task indexes.
function Find-EditorProjects([string]$homeDir) {
  $userDirs = @(
    Expand-Glob $homeDir 'AppData/Roaming/*/User'
    Expand-Glob $homeDir '.config/*/User'
    Expand-Glob $homeDir 'Library/Application Support/*/User'
    Expand-Glob $homeDir '.*-server*/data/User'
  )
  foreach ($userDir in $userDirs) {
    Get-JsonValues 'folder' @(Expand-Glob $userDir 'workspaceStorage/*/workspace.json')
    Get-JsonValues 'cwdOnTaskInitialization' @(
      Expand-Glob $userDir 'globalStorage/saoudrizwan.claude-dev/state/taskHistory.json'
      Expand-Glob $userDir 'globalStorage/saoudrizwan.claude-dev/tasks/*/task_metadata.json'
    )
    Get-JsonValues 'workspace' @(
      Expand-Glob $userDir 'globalStorage/rooveterinaryinc.roo-cline/tasks/_index.json'
      Expand-Glob $userDir 'globalStorage/kilocode.kilo-code/tasks/_index.json'
    )
  }
}

# Takes the discovered @{ user; path } entries; returns each existing directory
# that is not a home once, without regard to case, with the first user found.
function Select-UniqueProjects($found) {
  $seen = @{}; $unique = @()
  $homeKeys = @{}
  foreach ($userHome in $script:UserList) { $homeKeys[$userHome.home.ToLowerInvariant()] = $true }
  foreach ($entry in $found) {
    $key = $entry.path.ToLowerInvariant()
    if ($seen.ContainsKey($key)) {
      if ($seen[$key].user -eq '' -and $entry.user -ne '') { $seen[$key].user = $entry.user }
      continue
    }
    if ($homeKeys.ContainsKey($key)) { continue }
    if (-not (Test-PathQuiet $entry.path 'Container')) { continue }
    $seen[$key] = $entry; $unique += $entry
  }
  return $unique
}

# Returns every project directory to collect as @{ user; path }.
function Find-Projects {
  $found = @()
  foreach ($userHome in $script:UserList) {
    $homeDir = $userHome.home
    $script:DiscoveryHome = $homeDir
    $values = @(
      Find-ClaudeCodeProjects $homeDir
      Find-CliSessionProjects $homeDir
      Find-GeminiProjects $homeDir
      Find-AgentSessionProjects $homeDir
      Find-ConfigFileProjects $homeDir
      Find-EditorProjects $homeDir
    )
    $script:DiscoveryHome = ''
    foreach ($value in $values) {
      $localPath = ConvertTo-LocalPath ([string]$value)
      if ($localPath) { $found += @{ user = $userHome.user; path = $localPath.TrimEnd('\', '/') } }
    }
  }
  foreach ($projectArg in $Project) {
    $found += @{ user = (Get-ProjectOwner $projectArg); path = $projectArg.TrimEnd('\', '/') }
  }
  return (Select-UniqueProjects $found)
}

# Main step: collects PROJECT_CATALOG under every directory in
# $script:ProjectList, with an error_read row for a -Project that is not a
# directory.
function Add-Projects {
  foreach ($projectArg in $Project) {
    if (Test-PathQuiet $projectArg 'Container') { continue }
    if (Test-PathQuiet $projectArg 'Leaf') { $why = 'Not a directory' }
    else {
      $why = "Cannot find path '$projectArg' because it does not exist."
      $extendedArg = ConvertTo-ExtendedPath $projectArg
      try { [void](Get-Item -LiteralPath $extendedArg -Force -ErrorAction Stop) }
      catch { $why = ConvertFrom-ExtendedText $_.Exception.Message }
    }
    $warning = 'WARNING: project given with -Project is not a directory'
    Write-CollectorLog "${warning}: ${projectArg}: $why"
    if (-not $Inventory) {
      $owner = Get-ProjectOwner $projectArg
      $row = @($owner, $projectArg, '', $projectArg, '', 'dir', 0, 0, 0, 0, 0, '', '', '', $false,
        'error_read', '', $why)
      Write-ManifestRow @row
    }
  }
  if ($Inventory) { return }
  foreach ($projectDir in $script:ProjectList) {
    $userLabel = ''; if ($projectDir.user) { $userLabel = '[' + $projectDir.user + '] ' }
    Write-CollectorLog "Project $userLabel$($projectDir.path)"
    $why = Get-DirReadError $projectDir.path
    if ($why) { Write-ReadErrorRow $projectDir.user $projectDir.path '' $projectDir.path $why }
    Add-CatalogMatches $projectDir.user $projectDir.path $PROJECT_CATALOG
  }
}

# ===========================================================================
# Inventory output
# ===========================================================================
# Under -Inventory Add-CollectedFile feeds these accumulators instead of
# copying, and Write-Inventory prints them as JSON lines.

# The accumulator being filled: agent -> files, bytes, first and last mtime and
# the matched globs; one per user in $script:InventoryUsers, and
# $script:InventoryDocker for the volumes.
$script:InventoryAgents = [ordered]@{}
$script:InventoryUsers = @()

# Takes an agent; returns its entry in $script:InventoryAgents, creating it empty.
function Get-InventoryEntry([string]$agent) {
  if (-not $script:InventoryAgents.Contains($agent)) {
    $script:InventoryAgents[$agent] = @{
      files = 0; bytes = [int64]0; first = [int64]0; last = [int64]0
      evidence = (New-Object System.Collections.ArrayList)
    }
  }
  return $script:InventoryAgents[$agent]
}

# Takes an agent and the catalog glob that matched; adds the glob to the
# agent's evidence once.
function Add-InventoryEvidence([string]$agent, [string]$glob) {
  $entry = Get-InventoryEntry $agent
  if (-not $entry.evidence.Contains($glob)) { [void]$entry.evidence.Add($glob) }
}

# Takes an agent and a regular file's Get-Item; counts the file, its bytes
# and its modification time in the agent's entry.
function Add-InventoryFile([string]$agent, $item) {
  $entry = Get-InventoryEntry $agent
  $entry.files++
  try { $entry.bytes += [int64]$item.Length } catch { }
  $mtime = Get-Epoch $item.LastWriteTimeUtc
  if ($mtime -gt 0) {
    if ($entry.first -eq 0 -or $mtime -lt $entry.first) { $entry.first = $mtime }
    if ($mtime -gt $entry.last) { $entry.last = $mtime }
  }
}

# Takes epoch seconds; returns UTC ISO 8601 without fractions, or '' for 0.
function Format-InventoryTime([int64]$epoch) {
  if ($epoch -le 0) { return '' }
  $culture = [Globalization.CultureInfo]::InvariantCulture
  $utc = [DateTimeOffset]::FromUnixTimeSeconds($epoch).UtcDateTime
  return $utc.ToString('yyyy-MM-ddTHH:mm:ssZ', $culture)
}

# Takes a user, one accumulator and the user's project count; returns its agent
# lines as compressed JSON.
function Write-InventoryLines([string]$user, $accumulator, [int]$projects) {
  foreach ($agent in @($accumulator.Keys)) {
    $entry = $accumulator[$agent]
    if ($entry.files -eq 0) { continue }
    $line = [ordered]@{
      type = 'agent'; host = $script:HostName; user = $user; agent = $agent
      files = [int64]$entry.files; bytes = [int64]$entry.bytes
      first = (Format-InventoryTime $entry.first); last = (Format-InventoryTime $entry.last)
      projects = $projects; evidence = (@($entry.evidence) -join ',')
    }
    $line | ConvertTo-Json -Compress
  }
}

# Main step under -Inventory: returns the host line, each user's agent lines and
# the Docker volume agents.
function Write-Inventory {
  $denied = $script:DeniedHomes
  $unreadable = $denied.Count
  foreach ($userHome in $script:UserList) {
    if (-not (Test-DirReadable $userHome.home)) {
      $unreadable++
      Write-CollectorLog "WARNING: home of $($userHome.user) is not readable: $($userHome.home)"
    }
  }
  $hostLine = [ordered]@{
    type = 'host'; host = $script:HostName; collector = $ToolVersion; mode = $script:Mode
    at = $script:StartTs; users_scanned = ($script:UserList.Count + $denied.Count)
    users_unreadable = $unreadable; docker_volumes = $script:Docker.found
  }
  Write-Output ($hostLine | ConvertTo-Json -Compress)
  foreach ($pair in $script:InventoryUsers) {
    $userHome = $pair[0]
    $projectCount = @($script:ProjectList | Where-Object { $_.user -eq $userHome.user }).Count
    Write-InventoryLines $userHome.user $pair[1] $projectCount
  }
  Write-InventoryLines 'docker' $script:InventoryDocker 0
}

# ===========================================================================
# Summary and archive
# ===========================================================================
# Windows tar.exe crashes on a name outside the ANSI code page, so the PowerShell
# tar writer follows it and a zip is the last resort; CAC_ARCHIVER, a test
# hook, sets the chain.
$script:ArchiverLabels = @{ tar = 'tar.exe'; writer = 'PowerShell tar writer'; zip = 'ZipFile' }

# Takes a line; appends it to the run log with no timestamp. Used once the
# log writer is closed, for archiving and cleanup.
function Add-LogLine([string]$text) {
  [System.IO.File]::AppendAllText($script:XLogPath, "$text`n", $script:Utf8NoBom)
}

# Takes the staging directory; returns its entries as @(name, FileSystemInfo,
# kind), a directory before its contents, a staged link an entry of its own and
# never descended.
function Get-StageEntries([string]$stageDir) {
  $entries = New-Object System.Collections.ArrayList
  $stack = New-Object System.Collections.Stack
  $stack.Push(@('', (New-Object System.IO.DirectoryInfo((ConvertTo-ExtendedPath $stageDir)))))
  $reparse = [System.IO.FileAttributes]::ReparsePoint
  $directory = [System.IO.FileAttributes]::Directory
  while ($stack.Count -gt 0) {
    $top = $stack.Pop(); $prefix = $top[0]
    foreach ($child in $top[1].GetFileSystemInfos()) {
      $attributes = $child.Attributes
      if (($attributes -band $reparse) -ne 0) {
        [void]$entries.Add(@(($prefix + $child.Name), $child, 'l'))
      }
      elseif (($attributes -band $directory) -ne 0) {
        $name = $prefix + $child.Name + '/'
        [void]$entries.Add(@($name, $child, 'd')); $stack.Push(@($name, $child))
      }
      else { [void]$entries.Add(@(($prefix + $child.Name), $child, 'f')) }
    }
  }
  return , $entries
}

# Takes a FileSystemInfo; returns the epoch seconds of its own last write
# time, 0 before 1970.
function Get-UnixTime($info) {
  $seconds = 0
  try {
    $seconds = [int64][Math]::Floor(($info.LastWriteTimeUtc.Ticks - 621355968000000000) / 10000000)
  } catch { }
  if ($seconds -lt 0) { $seconds = 0 }
  return $seconds
}

$script:TarAscii = [System.Text.Encoding]::ASCII
$script:TarUtf8 = New-Object System.Text.UTF8Encoding $false
$script:TarZero = New-Object byte[] 1024
$script:TarNonAscii = New-Object System.Text.RegularExpressions.Regex('[^\x00-\x7f]')
# The fixed fields of a ustar header (uid and gid 0, the ustar magic and
# version 00, the checksum field as eight spaces) and their byte sum.
$script:TarTemplate = New-Object byte[] 512
[void]$script:TarAscii.GetBytes('0000000', 0, 7, $script:TarTemplate, 108)
[void]$script:TarAscii.GetBytes('0000000', 0, 7, $script:TarTemplate, 116)
[void]$script:TarAscii.GetBytes('ustar', 0, 5, $script:TarTemplate, 257)
$script:TarTemplate[263] = 48; $script:TarTemplate[264] = 48
for ($offset = 148; $offset -lt 156; $offset++) { $script:TarTemplate[$offset] = 32 }
$script:TarTemplateSum = 0
foreach ($templateByte in $script:TarTemplate) { $script:TarTemplateSum += $templateByte }

# Takes a member's name, ustar prefix, type flag, mode, size, mtime and link
# target; returns its 512-byte ustar header. The checksum is the template's sum
# plus the bytes written here, which is faster under 5.1 than summing all 512.
function Get-TarHeader {
  param([string]$name, [string]$prefix, [char]$type, [string]$tarMode, [int64]$size,
    [int64]$mtime, [string]$link)
  $header = [byte[]]$script:TarTemplate.Clone()
  $ascii = $script:TarAscii
  $sizeOctal = [Convert]::ToString($size, 8).PadLeft(11, '0')
  $mtimeOctal = [Convert]::ToString($mtime, 8).PadLeft(11, '0')
  [void]$ascii.GetBytes($name, 0, $name.Length, $header, 0)
  [void]$ascii.GetBytes($tarMode, 0, 7, $header, 100)
  [void]$ascii.GetBytes($sizeOctal, 0, 11, $header, 124)
  [void]$ascii.GetBytes($mtimeOctal, 0, 11, $header, 136)
  $header[156] = [byte]$type
  if ($link) { [void]$ascii.GetBytes($link, 0, $link.Length, $header, 157) }
  if ($prefix) { [void]$ascii.GetBytes($prefix, 0, $prefix.Length, $header, 345) }
  $fields = $name + $tarMode + $sizeOctal + $mtimeOctal + $link + $prefix
  $fieldBytes = [int[]]$ascii.GetBytes($fields)
  $sum = $script:TarTemplateSum + [int]$header[156] + [System.Linq.Enumerable]::Sum($fieldBytes)
  $sumOctal = [Convert]::ToString($sum, 8).PadLeft(6, '0')
  [void]$ascii.GetBytes($sumOctal, 0, 6, $header, 148)
  $header[154] = 0
  return , $header
}

# Takes a pax key and value; returns one pax record, "<len> key=value\n",
# where len counts the whole record, its own digits included.
function Get-PaxRecord([string]$key, [string]$value) {
  $body = ' ' + $key + '=' + $value + "`n"
  $bodyLength = $script:TarUtf8.GetByteCount($body)
  $length = $bodyLength + ([string]$bodyLength).Length
  if (([string]$length).Length -ne ([string]$bodyLength).Length) {
    $length = $bodyLength + ([string]$length).Length
  }
  return ([string]$length) + $body
}

# Takes a name and a field size; returns an ASCII stand-in for readers that
# ignore pax headers.
function Get-TarAsciiName([string]$name, [int]$max) {
  $asciiName = $script:TarNonAscii.Replace($name, '_')
  if ($asciiName.Length -gt $max) { $asciiName = $asciiName.Substring($asciiName.Length - $max) }
  return $asciiName
}

# Takes the output stream and a member's fields; writes its header, preceded by
# a pax header when a field does not fit ustar.
function Write-TarMember {
  param($out, [string]$path, [char]$type, [string]$tarMode, [int64]$size, [int64]$mtime,
    [string]$link)
  $name = $path; $prefix = ''; $pax = ''
  if ($script:TarNonAscii.IsMatch($path)) {
    $pax += Get-PaxRecord 'path' $path; $name = Get-TarAsciiName $path 100
  }
  elseif ($path.Length -gt 100) {
    # ustar splits a long name at a / into a prefix of up to 155 bytes and a
    # name of up to 100: the last / at or before byte 155 that leaves a
    # non-empty name.
    $cut = -1
    if ($path.Length -le 256) {
      $cut = $path.LastIndexOf([char]'/', [Math]::Min(155, $path.Length - 2))
    }
    if ($cut -gt 0 -and $cut -ge ($path.Length - 101)) {
      $prefix = $path.Substring(0, $cut); $name = $path.Substring($cut + 1)
    }
    else { $pax += Get-PaxRecord 'path' $path; $name = Get-TarAsciiName $path 100 }
  }
  if ($link -and ($link.Length -gt 100 -or $script:TarNonAscii.IsMatch($link))) {
    $pax += Get-PaxRecord 'linkpath' $link; $link = Get-TarAsciiName $link 100
  }
  if ($size -gt 8589934591) { $pax += Get-PaxRecord 'size' ([string]$size); $headerSize = 0 }
  else { $headerSize = $size }
  if ($pax) {
    $paxBytes = $script:TarUtf8.GetBytes($pax)
    $paxName = './PaxHeaders/' + (Get-TarAsciiName ($path.TrimEnd('/') -replace '^.*/', '') 87)
    $paxHeader = Get-TarHeader $paxName '' 'x' '0000644' $paxBytes.Length $mtime ''
    $out.Write($paxHeader, 0, 512); $out.Write($paxBytes, 0, $paxBytes.Length)
    $pad = (512 - $paxBytes.Length % 512) % 512; if ($pad) { $out.Write($script:TarZero, 0, $pad) }
  }
  $header = Get-TarHeader $name $prefix $type $tarMode $headerSize $mtime $link
  $out.Write($header, 0, 512)
}

# Takes the staging directory; returns each staged link's member name mapped to
# its staged target.
function Get-StagedLinkTargets([string]$stageDir) {
  $comparer = [StringComparer]::Ordinal
  if ($script:IsWindowsHost) { $comparer = [StringComparer]::OrdinalIgnoreCase }
  $targets = New-Object 'System.Collections.Generic.Dictionary[string,string]' ($comparer)
  foreach ($stagedLink in @($script:StagedLinks)) {
    if ($stagedLink[0].Length -gt $stageDir.Length + 1) {
      $memberName = $stagedLink[0].Substring($stageDir.Length + 1).Replace($script:Sep, '/')
      $targets[$memberName] = $stagedLink[2]
    }
  }
  return , $targets
}

# Takes a link entry and the staged-link targets; returns its target. Throws
# when it has none.
function Get-StagedLinkTarget($entry, $linkTargets) {
  $info = $entry[1]; $link = ''
  if ($linkTargets.ContainsKey($entry[0])) { $link = $linkTargets[$entry[0]] }
  if (-not $link) {
    $targetProperty = $info.PSObject.Properties['LinkTarget']
    if ($targetProperty -and $targetProperty.Value) { $link = [string]$targetProperty.Value }
    else {
      try {
        $targets = @((Get-Item -LiteralPath $info.FullName -Force -ErrorAction Stop).Target)
        if ($targets.Count -eq 1) { $link = [string]$targets[0] }
      } catch { }
    }
  }
  if (-not $link) { throw "staged link has no target: $(ConvertFrom-ExtendedPath $info.FullName)" }
  return $link
}

# Takes the output stream, the stage entries and the staged-link targets; writes
# each member. Throws when a staged file shrinks while it is read.
function Write-TarEntries($out, $entries, $linkTargets) {
  $buffer = New-Object byte[] 1048576
  $fileMode = [System.IO.FileMode]::Open; $readAccess = [System.IO.FileAccess]::Read
  $share = $script:ShareReadWriteDelete
  $nonAscii = $script:TarNonAscii
  foreach ($entry in $entries) {
    $info = $entry[1]; $kind = $entry[2]; $path = './' + $entry[0]
    $link = ''; $size = 0; $source = $null
    if ($kind -eq 'd') { $type = '5'; $tarMode = '0000755' }
    elseif ($kind -eq 'l') {
      $type = '2'; $tarMode = '0000777'; $link = Get-StagedLinkTarget $entry $linkTargets
    } else {
      $type = '0'; $tarMode = '0000644'
      $source = [System.IO.File]::Open($info.FullName, $fileMode, $readAccess, $share)
    }
    try {
      if ($source) { $size = $source.Length }
      # Inline rather than Get-UnixTime: a function call per member is
      # measurable under 5.1.
      $mtime = [int64][Math]::Floor(($info.LastWriteTimeUtc.Ticks - 621355968000000000) / 10000000)
      if ($mtime -lt 0) { $mtime = 0 }
      # Most members are short ASCII names; Write-TarMember handles the rest.
      $plainHeader = $path.Length -le 100 -and $link.Length -le 100 -and
        -not $nonAscii.IsMatch($path) -and -not ($link -and $nonAscii.IsMatch($link)) -and
        $size -le 8589934591
      if ($plainHeader) {
        $header = Get-TarHeader $path '' $type $tarMode $size $mtime $link
        $out.Write($header, 0, 512)
      } else { Write-TarMember $out $path $type $tarMode $size $mtime $link }
      if ($source) {
        $left = $size
        while ($left -gt 0) {
          $want = $buffer.Length; if ($left -lt $want) { $want = [int]$left }
          $read = $source.Read($buffer, 0, $want)
          if ($read -le 0) {
            throw "staged file shrank while archiving: $(ConvertFrom-ExtendedPath $info.FullName)"
          }
          $out.Write($buffer, 0, $read); $left -= $read
        }
        $pad = (512 - $size % 512) % 512; if ($pad) { $out.Write($script:TarZero, 0, $pad) }
      }
    } finally { if ($source) { $source.Dispose() } }
  }
}

# Takes the staging directory and the archive path; writes a gzip-compressed
# POSIX tar of the stage with the member names tar -czf gives. Throws on
# failure.
function Write-TarGz([string]$stageDir, [string]$archivePath) {
  $entries = Get-StageEntries $stageDir
  $linkTargets = Get-StagedLinkTargets $stageDir
  $fileStream = $null; $out = $null
  try {
    $fileStream = New-Object System.IO.FileStream((ConvertTo-ExtendedPath $archivePath),
      [System.IO.FileMode]::Create, [System.IO.FileAccess]::Write, [System.IO.FileShare]::None,
      65536)
    $gzip = New-Object System.IO.Compression.GZipStream($fileStream,
      [System.IO.Compression.CompressionLevel]::Optimal)
    # Headers are 512-byte writes; the buffer hands the compressor larger blocks.
    $out = New-Object System.IO.BufferedStream($gzip, 1048576)
    $stageInfo = New-Object System.IO.DirectoryInfo((ConvertTo-ExtendedPath $stageDir))
    Write-TarMember $out './' '5' '0000755' 0 (Get-UnixTime $stageInfo) ''
    Write-TarEntries $out $entries $linkTargets
    $out.Write($script:TarZero, 0, 1024)
  } finally {
    if ($null -ne $out) { $out.Dispose() } elseif ($null -ne $fileStream) { $fileStream.Dispose() }
  }
}

# Takes the staging directory and the archive path; writes a zip of the stage
# with / separators, which ZipFile.CreateFromDirectory on .NET Framework does
# not.
function Write-StageZip([string]$stageDir, [string]$archivePath) {
  Add-Type -AssemblyName System.IO.Compression.FileSystem
  $entries = Get-StageEntries $stageDir
  $hasChild = New-Object 'System.Collections.Generic.HashSet[string]' ([StringComparer]::Ordinal)
  foreach ($entry in $entries) {
    $trimmed = $entry[0].TrimEnd('/'); $slash = $trimmed.LastIndexOf('/')
    if ($slash -ge 0) { [void]$hasChild.Add($trimmed.Substring(0, $slash + 1)) }
  }
  $level = [System.IO.Compression.CompressionLevel]::Optimal
  $zip = $null
  try {
    $zip = [System.IO.Compression.ZipFile]::Open((ConvertTo-ExtendedPath $archivePath), 'Create')
    foreach ($entry in $entries) {
      $info = $entry[1]; $kind = $entry[2]
      if ($kind -eq 'l') {
        Add-LogLine "zip: staged link left out: $(ConvertFrom-ExtendedPath $info.FullName)"
        continue
      }
      if ($kind -eq 'd' -and $hasChild.Contains($entry[0])) { continue }
      $zipEntry = $zip.CreateEntry($entry[0], $level)
      # A zip time holds 1980 to 2107 only; outside that the entry keeps the default.
      try { $zipEntry.LastWriteTime = [DateTimeOffset]$info.LastWriteTime } catch { }
      if ($kind -eq 'd') { continue }
      $source = [System.IO.File]::Open($info.FullName, [System.IO.FileMode]::Open,
        [System.IO.FileAccess]::Read, $script:ShareReadWriteDelete)
      try {
        $destination = $zipEntry.Open()
        try { $source.CopyTo($destination, 1048576) } finally { $destination.Dispose() }
      } finally { $source.Dispose() }
    }
  } finally { if ($null -ne $zip) { $zip.Dispose() } }
}

# Deletes the staged links themselves, never their targets; returns how many
# could not be removed.
function Clear-StagedLinks {
  $left = 0
  foreach ($stagedLink in @($script:StagedLinks)) {
    $extendedLink = ConvertTo-ExtendedPath $stagedLink[0]
    try {
      if ($stagedLink[1]) { [System.IO.Directory]::Delete($extendedLink) }
      else { [System.IO.File]::Delete($extendedLink) }
    }
    catch {
      $left++
      $why = ConvertFrom-ExtendedText $_.Exception.Message
      Add-LogLine "could not remove staged link $($stagedLink[0]): $why"
    }
  }
  $script:StagedLinks.Clear()
  return $left
}

# Takes the reason; rewrites the symlink rows of the manifest without
# archive_path, after a fall back to zip.
function Clear-LinkArchivePaths([string]$reason) {
  $lines = [System.IO.File]::ReadAllLines($script:XManifestPath, $script:Utf8NoBom)
  $writer = New-Object System.IO.StreamWriter($script:XManifestPath, $false, $script:Utf8NoBom)
  try {
    foreach ($line in $lines) {
      if ($line.Contains('"status":"symlink"')) {
        $row = $line | ConvertFrom-Json
        if ($row.status -eq 'symlink' -and $row.archive_path) {
          $row.archive_path = ''; $row.error = $reason
          $line = $row | ConvertTo-Json -Compress -Depth 2
        }
      }
      $writer.WriteLine($line)
    }
  } finally { $writer.Close() }
}

# Takes a byte count; returns it human-readable: 512 -> "512 B",
# 21868755 -> "20.9 MiB".
function Format-HumanSize([int64]$bytes) {
  $units = @('B', 'KiB', 'MiB', 'GiB', 'TiB')
  $value = [double]$bytes; $i = 0
  while ($value -ge 1024 -and $i -lt 4) { $value /= 1024; $i++ }
  if ($i -eq 0) { return "$bytes B" }
  return $value.ToString('0.0', [Globalization.CultureInfo]::InvariantCulture) + ' ' + $units[$i]
}

# Notes the symlinks that are in the manifest only.
function Add-LinkNotes {
  if ($script:LinkCounts.zip -gt 0) {
    Add-Note ("$($script:LinkCounts.zip) symlinks are in the manifest only: " +
      'the zip archive cannot store symlinks')
  }
  if ($script:LinkCounts.failed -gt 0) {
    Add-Note ("$($script:LinkCounts.failed) symlinks could not be recreated in the archive; " +
      "each row's error says why")
  }
}

# Main step after the walk: sets $script:EndTs and $script:ActiveUsers and builds
# $script:Summary, the content of collection.json. The body is one literal.
function Initialize-Summary {
  $script:EndTs = (Get-Date).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ')
  $script:ActiveUsers = @($script:UserList | Where-Object { $script:RowHomes.ContainsKey($_.home) })
  Add-LinkNotes
  $rootText = '\'; if ($Root) { $rootText = $Root }
  $platform = "Windows $([Environment]::OSVersion.Version) $env:PROCESSOR_ARCHITECTURE"
  $platform += " PowerShell $($PSVersionTable.PSVersion)"
  try { $runAs = [Security.Principal.WindowsIdentity]::GetCurrent().Name }
  catch { $runAs = [Environment]::UserDomainName + '\' + [Environment]::UserName }
  $counts = $script:Counts
  $script:Summary = [ordered]@{
    tool = $TOOL; version = $ToolVersion; hostname = $script:HostName; mode = $script:Mode
    root = $rootText
    platform = $platform; run_as = $runAs; run_as_admin = $script:IsAdmin
    started = $script:StartTs; finished = $script:EndTs
    options = [ordered]@{
      full = $Full.IsPresent; no_secrets = $NoSecrets.IsPresent
      max_file_size_bytes = $script:MaxSize
      users_filter = $Users; no_docker = $NoDocker.IsPresent; no_projects = $NoProjects.IsPresent
      projects = @($Project)
    }
    capabilities = [ordered]@{
      hash_tool = 'System.Security.Cryptography.SHA256'
      archiver = $script:ArchiverLabels[$script:Archivers[0]]
    }
    users = @($script:UserList | ForEach-Object { $_.user })
    homes = @($script:UserList | ForEach-Object { $_.home })
    users_with_artifacts = @($script:ActiveUsers | ForEach-Object { $_.user })
    projects = @($script:ProjectList | ForEach-Object { $_.path })
    counts = [ordered]@{
      collected = $counts.collected; symlink = $counts.symlink
      skipped_excluded = $counts.skipped_excluded; skipped_size = $counts.skipped_size
      skipped_secret = $counts.skipped_secret; skipped_special = $counts.skipped_special
      error_copy = $counts.error_copy
      skipped_unmatched_volume = $counts.skipped_unmatched_volume; error_read = $counts.error_read
      collected_bytes = $counts.bytes
    }
    docker = [ordered]@{
      volumes_found = $script:Docker.found; volumes_collected = $script:Docker.collected
      unreadable = $script:Docker.unreadable; docker_desktop = $script:Docker.desktop
    }
    notes = @($script:Notes)
    archive = (Split-Path -Leaf $script:Archive)
  }
}

# Writes collection.json beside the archive and stages a copy; called again
# before each fallback archiver.
function Write-Summary {
  $json = $script:Summary | ConvertTo-Json -Depth 4
  [System.IO.File]::WriteAllText($script:XSummaryPath, $json, $script:Utf8NoBom)
  $stagedSummary = Join-PathSafe $script:XStage 'collection.json'
  Copy-Item -LiteralPath $script:XSummaryPath -Destination $stagedSummary -Force
}

# Main step: closes the manifest and log writers and stages copies of both;
# later log lines go through Add-LogLine.
function Close-RunFiles {
  Write-CollectorLog 'Archiving'
  $script:ManifestWriter.Close()
  $script:LogWriter.Close()
  $stagedManifest = Join-PathSafe $script:XStage 'manifest.jsonl'
  Copy-Item -LiteralPath $script:XManifestPath -Destination $stagedManifest -Force
  $stagedLog = Join-PathSafe $script:XStage 'collector.log'
  Copy-Item -LiteralPath $script:XLogPath -Destination $stagedLog -Force
}

# Takes a native exit code; returns it as text, in hex for a negative
# (NTSTATUS) code, or 'unknown'.
function Format-ExitCode($code) {
  if ($null -eq $code) { return 'unknown' }
  if ([int]$code -lt 0) { return '0x' + ([int]$code).ToString('X8') }
  return [string]$code
}

# Removes the archive file if one exists (a partial one from a failed
# archiver, or a leftover); logs when it cannot.
function Clear-PartialArchive {
  try {
    if (Test-PathQuiet $script:Archive 'Any') {
      [System.IO.File]::Delete((ConvertTo-ExtendedPath $script:Archive))
    }
  }
  catch {
    Add-LogLine "could not remove partial archive $($script:Archive): $($_.Exception.Message)"
  }
}

# Before a fall back to zip: removes the staged links and rewrites their rows.
# Throws when a link cannot be removed, so nothing is zipped through one.
function Clear-StagedLinksForZip {
  $linkCount = $script:StagedLinks.Count
  if ((Clear-StagedLinks) -gt 0) {
    throw 'staged symlinks could not be removed; not zipping through them'
  }
  Clear-LinkArchivePaths 'not in the archive: tar failed and zip cannot store symlinks'
  $script:Notes += ("tar failed; $linkCount symlinks were removed from the staging tree before " +
    'zipping, because zip cannot store symlinks')
  Add-LogLine "NOTE: tar failed; $linkCount staged symlinks removed before zipping"
  $stagedManifest = Join-PathSafe $script:XStage 'manifest.jsonl'
  Copy-Item -LiteralPath $script:XManifestPath -Destination $stagedManifest -Force
}

# Takes the next archiver and why the previous one failed; notes the fall back
# and rewrites and restages collection.json and the log. Sets $script:Archive.
function Switch-Archiver([string]$kind, [string]$failedWhy) {
  $label = $script:ArchiverLabels[$kind]
  if ($kind -eq 'zip' -and $script:StagedLinks.Count -gt 0) { Clear-StagedLinksForZip }
  $script:Notes += "$failedWhy; fell back to $label"
  Add-LogLine "NOTE: falling back to $label"
  if ($kind -eq 'zip') {
    $script:Archive = Join-PathSafe $script:OutputDir "$($script:RunName).zip"
    Clear-PartialArchive
  }
  $script:Summary.notes = @($script:Notes)
  $script:Summary.archive = (Split-Path -Leaf $script:Archive)
  $script:Summary.capabilities.archiver = $label
  Write-Summary
  $stagedLog = Join-PathSafe $script:XStage 'collector.log'
  Copy-Item -LiteralPath $script:XLogPath -Destination $stagedLog -Force
}

# Takes an archiver (tar, writer or zip); runs it and sets
# $script:ArchiveFailure. The tar line reads script variables without $script:
# and keeps its indentation, because 5.1 logs a native command's first stderr
# line with its source line and column.
function Invoke-Archiver([string]$kind) {
  $label = $script:ArchiverLabels[$kind]
  $script:ArchiveFailure = ''
  if ($kind -eq 'tar') {
    $tarName = Split-Path -Leaf $script:TarExe
    $global:LASTEXITCODE = 0
    & $TarExe -czf $Archive -C $Stage . 2>>$LogPath
    $tarExit = $LASTEXITCODE
    if ($tarExit -eq 0 -and (Test-PathQuiet $script:Archive 'Any')) { return }
    if ($tarExit -eq 0) {
      $script:ArchiveFailure = "$tarName failed (exit code 0 but no archive was written)"
    }
    else { $script:ArchiveFailure = "$tarName failed (exit code $(Format-ExitCode $tarExit))" }
    Add-LogLine "${tarName} failed: exit code $(Format-ExitCode $tarExit)"
    return
  }
  try {
    if ($kind -eq 'writer') { Write-TarGz $script:Stage $script:Archive }
    else { Write-StageZip $script:Stage $script:Archive }
    if (Test-PathQuiet $script:Archive 'Leaf') { return }
    $script:ArchiveFailure = "$label failed (no archive was written)"
  } catch {
    $inner = $_.Exception
    while ($null -ne $inner.InnerException) { $inner = $inner.InnerException }
    $script:ArchiveFailure = "$label failed ($($inner.Message))"
    Add-LogLine "$label failed: $($inner.Message)"
  }
}

# Main step: tries each archiver in $script:Archivers in turn. Sets
# $script:ArchiveWritten.
function Write-Archive {
  Clear-PartialArchive
  $script:ArchiveWritten = $false
  $script:ArchiveFailure = ''
  for ($i = 0; $i -lt $script:Archivers.Count; $i++) {
    $kind = $script:Archivers[$i]
    if ($i -gt 0) {
      try { Switch-Archiver $kind $script:ArchiveFailure }
      catch { Add-LogLine "FATAL: archive failed: $($_.Exception.Message)"; break }
    }
    Invoke-Archiver $kind
    if ($script:ArchiveFailure -eq '') { $script:ArchiveWritten = $true; break }
    Clear-PartialArchive
  }
}

# Main step: writes <archive>.sha256 and logs "done". Sets $script:ArchiveSha and
# $script:ArchiveSize.
function Write-ArchiveHash {
  $script:ArchiveSha = Get-FileSha256 $script:Archive
  $script:ArchiveSize = (Get-Item -LiteralPath (ConvertTo-ExtendedPath $script:Archive)).Length
  $archiveName = Split-Path -Leaf $script:Archive
  $hashFile = ConvertTo-ExtendedPath ($script:Archive + '.sha256')
  $hashLine = "$($script:ArchiveSha)  $archiveName`n"
  [System.IO.File]::WriteAllText($hashFile, $hashLine, $script:Utf8NoBom)
  $sizeAndHash = "$($script:ArchiveSize) bytes, sha256 $($script:ArchiveSha)"
  Add-LogLine "$($script:EndTs) done: $($script:Archive) ($sizeAndHash)"
  if (-not $Quiet) { [Console]::Error.WriteLine('done') }
}

# Main step unless -KeepStaging: removes the staged links, then the staging
# directory; a link that could not be deleted keeps the directory.
function Clear-Staging {
  if ((Clear-StagedLinks) -eq 0) {
    try { Remove-Item -LiteralPath $script:XStage -Recurse -Force -ErrorAction Stop } catch { }
  }
  else {
    $why = 'staging directory kept because a staged link could not be removed'
    Add-LogLine "${why}: $($script:Stage)"
  }
}

# Main step: prints the summary on stdout: the output files, the counts and,
# when the volume step saw Docker or Podman, the docker line.
function Write-Result {
  $counts = $script:Counts
  Write-Output "archive:    $($script:Archive)"
  Write-Output "size:       $($script:ArchiveSize) ($(Format-HumanSize $script:ArchiveSize))"
  Write-Output "sha256:     $($script:ArchiveSha)"
  Write-Output "manifest:   $($script:ManifestPath)"
  Write-Output "summary:    $($script:SummaryPath)"
  Write-Output "log:        $($script:LogPath)"
  Write-Output "users:      $($script:ActiveUsers.Count)"
  Write-Output "projects:   $($script:ProjectList.Count)"
  $collectedSize = Format-HumanSize $counts.bytes
  Write-Output "collected:  $($counts.collected) files, $($counts.bytes) bytes ($collectedSize)"
  $skipped = "$($counts.skipped_excluded) excluded, $($counts.skipped_size) too large"
  $skipped += ", $($counts.skipped_secret) secret, $($counts.skipped_special) special"
  Write-Output "skipped:    $skipped"
  $errorCount = $counts.error_copy + $counts.error_read
  $errorKinds = "$($counts.error_copy) error_copy, $($counts.error_read) error_read"
  Write-Output "errors:     $errorCount ($errorKinds)"
  if ($script:Docker.seen) {
    $docker = $script:Docker
    $dockerLine = "docker:     $($docker.found) volumes found, $($docker.collected) collected, "
    $dockerLine += "$($docker.unreadable) unreadable"
    if ($script:Docker.unreadable -gt 0) { $dockerLine += ' (run as root to collect)' }
    if ($script:Docker.desktop) { $dockerLine += '; Docker Desktop VM disk not collected' }
    Write-Output $dockerLine
  }
}

# ===========================================================================
# Main program
# ===========================================================================
# The options come from param() at the top of the file; the run list at the end
# calls one step per section, in run order.

# Takes a table here-string; returns all its non-empty lines, comments
# included (the form -List prints).
function Get-NonEmptyLines([string]$table) {
  $table -split "`r?`n" | Where-Object { $_ -ne '' }
}

# Prints the five tables for -List, each under its header line. The output
# is identical to the sh collector's --list (tests/catalog-sync.sh).
function Write-CatalogList {
  Write-Output '# home artifacts (agent|glob relative to home)'
  Get-NonEmptyLines $CATALOG
  Write-Output '# project artifacts (relative to each discovered project)'
  Get-NonEmptyLines $PROJECT_CATALOG
  Write-Output '# excluded unless --full'
  Get-NonEmptyLines $EXCLUDES
  Write-Output ''
  Write-Output '# credential files (flagged secret, skipped with --no-secrets)'
  Get-NonEmptyLines $SECRET_GLOBS
  Write-Output '# docker volumes (agent|volume name glob)'
  Get-NonEmptyLines $DOCKER_VOLUMES
}

# Sets $script:ExcludeRegex from EXCLUDES (a regex that matches nothing
# under -Full) and $script:SecretRegex from SECRET_GLOBS.
function Initialize-TableRegexes {
  $excludeGlobs = @(Get-TableLines $EXCLUDES); if ($Full) { $excludeGlobs = @() }
  $script:ExcludeRegex = ConvertTo-TableRegex $excludeGlobs
  $script:SecretRegex = ConvertTo-TableRegex @(Get-NonEmptyLines $SECRET_GLOBS)
}

# Takes the -Project values; returns each once, resolved against the current
# location and never prefixed with -Root, since -Project names a path on the
# machine running the collector.
function Resolve-ProjectArgs([string[]]$projectArgs) {
  $resolved = @()
  foreach ($projectArg in @($projectArgs)) {
    if (-not $projectArg) { continue }
    $path = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($projectArg)
    if ($path.Length -gt 1) { $path = $path.TrimEnd('\', '/') }
    if ($path.EndsWith(':')) { $path += '\' }
    if (-not ($resolved -contains $path)) { $resolved += $path }
  }
  return $resolved
}

# Sets $script:Mode and resolves -Root in image mode. Returns '' or the error.
function Resolve-RootArg {
  $script:Mode = 'live'
  if ($Root -eq '') { return '' }
  if (-not (Test-PathQuiet $Root 'Container')) { return "Root is not a directory: $Root" }
  # One statement, so -Root is left as given if Resolve-Path fails.
  $script:Root = (ConvertFrom-ExtendedPath (
      Resolve-Path -LiteralPath (ConvertTo-ExtendedPath $Root)).ProviderPath).TrimEnd('\', '/')
  $script:Mode = 'image'
  return ''
}

# Creates and resolves -OutputDir and checks it is writable with a probe file.
# Returns '' or the error. New-Item -Force neither throws nor returns an item
# under a file, so the missing item is the error too.
function Initialize-OutputDir {
  if (-not (Test-PathQuiet $OutputDir 'Container')) {
    $directory = @{ ItemType = 'Directory'; Path = (ConvertTo-ExtendedPath $OutputDir) }
    $created = $null
    try { $created = New-Item @directory -Force -ErrorAction Stop } catch { }
    if ($null -eq $created) { return "Cannot create output dir: $OutputDir" }
  }
  # One statement, so -OutputDir is left as given if Resolve-Path fails
  # (the error is written and the probe below reports the directory).
  $script:OutputDir = (ConvertFrom-ExtendedPath (
      Resolve-Path -LiteralPath (ConvertTo-ExtendedPath $OutputDir)).ProviderPath).TrimEnd('\', '/')
  if ($script:OutputDir -eq '') { $script:OutputDir = $script:Sep }
  $probe = Join-PathSafe $script:OutputDir ".write-probe-$([guid]::NewGuid().ToString('N'))"
  try {
    $probeStream = [System.IO.File]::Open((ConvertTo-ExtendedPath $probe),
      [System.IO.FileMode]::CreateNew, [System.IO.FileAccess]::Write)
    $probeStream.Close()
  }
  catch { return "Output dir not writable: $($script:OutputDir)" }
  try { [System.IO.File]::Delete((ConvertTo-ExtendedPath $probe)) } catch { }
  return ''
}

# Sets $script:StartTs, $script:HostName, $script:RunName and the output paths,
# plain and extended.
function Initialize-RunNames {
  $script:StartTs = (Get-Date).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ')
  $stamp = (Get-Date).ToUniversalTime().ToString('yyyyMMddTHHmmssZ')
  $script:HostName = $env:COMPUTERNAME
  if (-not $script:HostName) { $script:HostName = [System.Net.Dns]::GetHostName() }
  $hostSafe = ($script:HostName -replace '[^A-Za-z0-9._-]', '-')
  $script:RunName = "${hostSafe}_${stamp}_agent-artifacts"
  $script:Stage = Join-PathSafe $OutputDir ".stage-$($script:RunName)"
  $script:LogPath = Join-PathSafe $OutputDir "$($script:RunName).log"
  $script:ManifestPath = Join-PathSafe $OutputDir "$($script:RunName).manifest.jsonl"
  $script:SummaryPath = Join-PathSafe $OutputDir "$($script:RunName).collection.json"
  $script:XStage = ConvertTo-ExtendedPath $script:Stage
  $script:XLogPath = ConvertTo-ExtendedPath $script:LogPath
  $script:XManifestPath = ConvertTo-ExtendedPath $script:ManifestPath
  $script:XSummaryPath = ConvertTo-ExtendedPath $script:SummaryPath
}

# Sets $script:TarExe, $script:Archivers, $script:ArchiverWhy,
# $script:LinksInArchive and $script:Archive; internals.md, "Capability
# detection", has the chain. Returns '' or the CAC_ARCHIVER error.
function Initialize-Archivers {
  $script:TarExe = $null
  $tarCommand = $null
  if ($Inventory) { }
  elseif ($script:IsWindowsHost) { $tarCommand = Get-Command tar.exe -ErrorAction SilentlyContinue }
  else { $tarCommand = Get-Command tar -ErrorAction SilentlyContinue }
  if ($tarCommand) { $script:TarExe = $tarCommand.Source }
  $script:Archivers = @()
  $script:ArchiverWhy = ''
  if (-not $Inventory) {
    $spec = 'tar,writer,zip'
    if ($env:CAC_ARCHIVER) { $spec = $env:CAC_ARCHIVER; $script:ArchiverWhy = "CAC_ARCHIVER=$spec" }
    foreach ($kind in @($spec -split ',')) {
      $kind = $kind.Trim()
      if (-not $script:ArchiverLabels.ContainsKey($kind)) {
        return "CAC_ARCHIVER: unknown archiver '$kind' (use tar, writer or zip, comma-separated)"
      }
      if ($kind -eq 'tar' -and -not $script:TarExe) {
        if (-not $script:ArchiverWhy) { $script:ArchiverWhy = 'tar.exe not found' }
        continue
      }
      if (-not ($script:Archivers -contains $kind)) { $script:Archivers += $kind }
    }
    if ($script:Archivers.Count -eq 0) {
      return "CAC_ARCHIVER: no archiver left to try ($spec, $($script:ArchiverWhy))"
    }
    if (-not ($script:Archivers -contains 'tar')) { $script:TarExe = $null }
  }
  $zipFirst = ($script:Archivers.Count -gt 0 -and $script:Archivers[0] -eq 'zip')
  $script:LinksInArchive = ($script:Archivers.Count -gt 0 -and $script:Archivers[0] -ne 'zip')
  $extension = 'tar.gz'; if ($zipFirst) { $extension = 'zip' }
  $script:Archive = Join-PathSafe $OutputDir "$($script:RunName).$extension"
  return ''
}

# Unless -Inventory, creates the staging tree and opens $script:LogWriter and
# $script:ManifestWriter.
function Open-RunFiles {
  $script:LogWriter = $null
  $script:ManifestWriter = $null
  if ($Inventory) { return }
  [void][System.IO.Directory]::CreateDirectory((Join-PathSafe $script:XStage 'fs'))
  $utf8 = $script:Utf8NoBom
  $script:LogWriter = New-Object System.IO.StreamWriter($script:XLogPath, $false, $utf8)
  $script:ManifestWriter = New-Object System.IO.StreamWriter($script:XManifestPath, $false, $utf8)
  $script:LogWriter.AutoFlush = $true
}

# Returns whether the current identity is in the Administrators role, or
# $false when that cannot be determined (always off Windows).
function Test-Administrator {
  try {
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = [Security.Principal.WindowsPrincipal]$identity
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
  } catch { return $false }
}

# Logs the start lines: version, host, mode, archiver, options and warnings.
function Write-StartLog {
  $osVersion = [Environment]::OSVersion.VersionString
  $rootText = '\'; if ($Root) { $rootText = $Root }
  Write-CollectorLog ("$TOOL $ToolVersion starting on $($script:HostName) ($osVersion, " +
    "PowerShell $($PSVersionTable.PSVersion)) mode=$script:Mode root=$rootText")
  $archiveVia = 'none'
  if ($script:Archivers.Count -eq 0) { }
  elseif ($script:Archivers[0] -eq 'zip') { $archiveVia = 'zip' }
  elseif ($script:Archivers[0] -eq 'tar') { $archiveVia = 'tar.gz via ' + $script:TarExe }
  elseif ($script:Archivers[0] -eq 'writer') { $archiveVia = 'tar.gz via PowerShell tar writer' }
  if ($script:ArchiverWhy -and $script:Archivers.Count -gt 0 -and $script:Archivers[0] -ne 'tar') {
    $archiveVia += " ($($script:ArchiverWhy))"
  }
  Write-CollectorLog ("archive=$archiveVia max_file_size=${MaxFileSizeMB}MB " +
    "full=$($Full.IsPresent) no_secrets=$($NoSecrets.IsPresent) no_docker=$($NoDocker.IsPresent)")
  if ($Inventory -and $script:OutputDirGiven) {
    Write-CollectorLog "NOTE: -Inventory writes nothing; -OutputDir $OutputDir ignored"
  }
  if ($script:Mode -eq 'live' -and -not $script:IsAdmin) {
    Write-CollectorLog ('WARNING: not running as Administrator; ' +
      "other users' profiles will probably be unreadable")
  }
  if ($script:UserList.Count -eq 0) {
    Write-CollectorLog 'WARNING: no user profile directories found'
  }
  foreach ($inaccessible in $script:InaccessibleHomes) {
    Write-CollectorLog "profile not accessible (skipped): $inaccessible"
  }
}

# ---------------------------------------------------------------------------
# Run. Each step is defined in the section named beside it.
# ---------------------------------------------------------------------------
if ($Version) { Write-Output "$TOOL $ToolVersion"; exit 0 }
if ($List) { Write-CatalogList; exit 0 }
if ($MaxFileSizeMB -lt 0) { Write-Error 'Invalid -MaxFileSizeMB'; exit 1 }
$script:MaxSize = [int64]$MaxFileSizeMB * 1024 * 1024
# $PSBoundParameters inside a function is the function's own, so it is read here.
$script:OutputDirGiven = $PSBoundParameters.ContainsKey('OutputDir')

# Setup: resolve the options, name the outputs, open the log and manifest.
Initialize-TableRegexes
$Project = @(Resolve-ProjectArgs $Project)
$setupError = Resolve-RootArg
if ($setupError) { Write-Error $setupError; exit 2 }
# -Inventory writes nothing: no output directory, staging, log or archive.
if (-not $Inventory) {
  $setupError = Initialize-OutputDir
  if ($setupError) { Write-Error $setupError; exit 2 }
}
Initialize-RunNames
$setupError = Initialize-Archivers
if ($setupError) { Write-Error $setupError; exit 2 }
Open-RunFiles

$script:UserList = @(Get-UserHomes)                     # 7. Users
$script:IsAdmin = Test-Administrator
Write-StartLog
Add-UserHomes                                           # 7. Users (and the -Inventory walk)
if (-not $NoDocker) { Add-DockerVolumes }               # 8. Docker and Podman volumes
if (-not $NoProjects) {
  $script:ProjectList = @(Find-Projects)                # 9. Project discovery
  Add-Projects
}
if ($Inventory) {
  Write-Inventory                                       # 10. Inventory output
  exit 0
}
Initialize-Summary                                      # 11. Summary and archive
Write-Summary
Close-RunFiles
Write-Archive
if (-not $script:ArchiveWritten) { Write-Error 'FATAL: archive not written'; exit 2 }
Write-ArchiveHash
if (-not $KeepStaging) { Clear-Staging }
Write-Result
exit 0
