#!/bin/sh
# collect-agent-artifacts.sh - forensic collector for AI coding agent artifacts
#
# Copyright 2026 Sean Whalen
# SPDX-License-Identifier: Apache-2.0
# Licensed under the Apache License, Version 2.0; see the LICENSE file.
#
# Collects the on-disk state of AI coding agents (Claude Code, Gemini CLI,
# Antigravity, Codex CLI, Copilot CLI, Cursor, Windsurf, Continue, Aider,
# Ollama, ...) for every user on a host, from Docker and Podman named
# volumes, into a single tar.gz with a JSONL manifest. Host state (processes,
# users, network, shell histories) is left to the EDR it supplements.
#
# Portability: POSIX sh only. Runs under bash 3.2 (macOS /bin/sh), dash,
# ash/busybox, FreeBSD/OpenBSD sh and zsh in sh emulation. External tools used:
# find, cp, ls, tar, gzip, du, awk, grep, sed, sort, cut, readlink, mkdir, rm,
# and one of sha256sum / shasum / sha256 / openssl.
#
# Modes:
#   live   (default)   collect from the running system
#   image  (-r ROOT)   collect from a mounted disk image / alternate root
# Either mode with --inventory writes nothing: it walks the catalog with lstat
# only and prints one JSON line per (user, agent) found to stdout.
#
# Exit codes: 0 archive written (per-file errors are recorded in the manifest),
#               or, with --inventory, the walk ran,
#             1 usage error, 2 fatal (no output dir, no tar, ...).

# ===========================================================================
# Reader's guide
# ===========================================================================
# The file is in twelve sections, in this order (the PowerShell collector,
# Collect-AgentArtifacts.ps1, has the same sections under the same titles):
#    1. Reader's guide       this map
#    2. Data tables          CATALOG, PROJECT_CATALOG, EXCLUDES, SECRET_GLOBS
#                            and DOCKER_VOLUMES: what is collected, as data
#    3. Shared helpers       logging, notes, JSON escaping, number checks and
#                            the stat, hash and file id wrappers (mostly
#                            COMMON; add_note and file_id run in the main
#                            shell only)
#    4. Manifest writer      the row writer, the directory-read check and the
#                            error_read row (COMMON)
#    5. Staging files        the worker programs that find -exec runs
#    6. Catalog walk         glob expansion, catalog expansion with case
#                            folding and claimed paths, one match, the walk
#    7. Users                enumerating and filtering home directories
#    8. Docker and Podman volumes
#    9. Project discovery    project paths read from agent state, and -p
#   10. Inventory output     the --inventory JSON lines
#   11. Summary and archive  collection.json, the archivers, the stdout summary
#   12. Main program         options, setup, then one call per step
#
# Data flow: the tables are applied to every home the user enumeration lists.
# For each home the catalog walk expands every CATALOG entry first and prunes
# each match that lies inside another match from the enclosing walk (a claimed
# subtree, collected once, under the more specific agent), then walks each
# match with one find(1). find prunes the EXCLUDES globs with one
# skipped_excluded row each and hands the files to the stage worker, which
# tests SECRET_GLOBS, lstats every file before anything is copied, recreates a
# symlink instead of following it, records a socket, FIFO or device without
# opening it, copies the file into the staging directory, hashes the staged
# copy (not the source, which a running agent may still be appending to) and
# writes one manifest row per file. collection.json and the stdout summary are
# built from the manifest, and the staging directory, with the manifest, the
# log and collection.json, becomes the archive. Docker and Podman volumes
# matched by DOCKER_VOLUMES are collected whole, each as one match; discovered
# projects are further bases walked with PROJECT_CATALOG. --inventory is the
# same walk with no writes: the workers only stat, and section 10 prints one
# JSON line per user and agent.
#
# Naming: globals that functions share (configuration, counters, file paths,
# the tables) are UPPER_CASE. POSIX sh has no local, so a function's own
# temporaries are _<prefix>_<name>, the prefix an abbreviation of the
# function's name stated in its comment (_cp_ for collect_path), which keeps
# a callee from clobbering its caller's variables. A function that assigns
# its result instead of printing it, to save a fork, sets _ret_<what>
# (_ret_json, _ret_number) and its comment says so. A worker program is held
# in <NAME>_WORKER and read from the here-document EOF_<NAME>_WORKER; its
# variables need no prefix. A name is at least three characters after its
# prefix, except a short loop index (i, j, k).
#
# Maintainer documentation, with the table of sh and PowerShell function
# pairs: collectors/docs/internals.md.

VERSION="1.21.0"
TOOL="collect-agent-artifacts"

# Settings every later line relies on: the C locale for byte-wise matching
# and sorting, the base system's tools first on PATH, and output that only
# the user running the collector can read. CDPATH is unset because cd
# searches it for a relative -r or -o and then prints the directory, which
# would resolve the wrong tree into a two-line path.
LC_ALL=C
export LC_ALL
unset CDPATH
PATH="/usr/bin:/bin:/usr/sbin:/sbin:/usr/local/bin:$PATH"
export PATH
umask 077

# ===========================================================================
# Data tables
# ===========================================================================
# Artifact catalog. One entry per line: agent|path-glob (relative to each home
# directory). Every entry is tried for every home, so platform-specific paths
# are harmless on other platforms and a Windows image mounted on Linux works.
# ---------------------------------------------------------------------------
CATALOG='
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
# OpenCode (xdg-basedir: same paths on macOS and Windows)
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
# Goose (XDG on macOS too; Electron app dir is separate)
goose|.config/goose
goose|.local/share/goose
goose|.local/state/goose
goose|.config/Goose
goose|Library/Application Support/Goose
goose|AppData/Roaming/Block/goose
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
'

# Project-level artifacts, relative to each discovered project directory.
PROJECT_CATALOG='
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
'

# Paths (globs relative to home, project dir or Docker volume _data) skipped
# unless --full. Volume-relative forms of a tool's exclusions follow the
# home-relative ones at the end of the table. They are
# recorded in the manifest as skipped_excluded with their size. find(1) -path
# semantics: * also matches /.
EXCLUDES='
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
.local/share/goose/models
.local/share/goose/model_catalog
.local/state/goose/codex
.config/goose/mcp-apps-cache
AppData/Roaming/Block/goose/data/models
AppData/Roaming/Block/goose/data/model_catalog
AppData/Roaming/Block/goose/data/codex
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
'

# Files holding credentials. Collected by default and flagged secret=true in
# the manifest; skipped with --no-secrets. Matched with case(1) globs against
# the path relative to the home, project or Docker volume _data directory;
# * also matches /. Volume-relative forms come last.
SECRET_GLOBS='.claude/.credentials.json
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
.config/opencode/service*.json
.local/state/opencode/service*.json
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
.config/goose/githubcopilot/*
.config/goose/chatgpt_codex/*
.config/goose/gemini_oauth/*
.config/goose/kimicode/*
.config/goose/muse_code/*
.config/goose/huggingface/*
.config/goose/databricks/*
.config/goose/xai_oauth/*
.config/goose/roaming_node_key
.config/goose/tls/*
AppData/Roaming/Block/goose/config/secrets.yaml
AppData/Roaming/Block/goose/config/config.yaml
AppData/Roaming/Block/goose/config/githubcopilot/*
AppData/Roaming/Block/goose/config/chatgpt_codex/*
AppData/Roaming/Block/goose/config/gemini_oauth/*
AppData/Roaming/Block/goose/config/kimicode/*
AppData/Roaming/Block/goose/config/muse_code/*
AppData/Roaming/Block/goose/config/huggingface/*
AppData/Roaming/Block/goose/config/databricks/*
AppData/Roaming/Block/goose/config/xai_oauth/*
AppData/Roaming/Block/goose/config/roaming_node_key
AppData/Roaming/Block/goose/config/tls/*
.local/state/goose/logs/llm_request.*
AppData/Roaming/Block/goose/data/logs/llm_request.*
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
ssh/*'

# Docker and Podman named volumes, agent|glob matched against the volume name.
DOCKER_VOLUMES='
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
# Inferred names: the documentation of these tools bind-mounts the home directory
# instead of a named volume; a volume replacing it has the same layout (~/.tabby
# at /data, ~/.hermes at /opt/data, ...)
tabby|*tabby*
local-deep-research|*local-deep-research*
openhands|*openhands*
letta|*letta*
hermes|*hermes*
nanobot|*nanobot*
'


# ===========================================================================
# Shared helpers
# ===========================================================================
# The helpers the workers also need are text in COMMON, eval'd here and
# prepended to every worker; the ones after COMMON run in the main shell only.

# heredoc_var: reads a here-document on stdin into _ret_heredoc_text without its
# final newline. VAR=$(cat <<'EOF' ...) cannot be used: posh ends a $(...) at the
# first `)`, even a case pattern's inside a here-document.
heredoc_var() {
  _ret_heredoc_text=
  while IFS= read -r _hv_line; do _ret_heredoc_text="$_ret_heredoc_text$_hv_line
"; done
  _ret_heredoc_text=${_ret_heredoc_text%?}
}

heredoc_var <<'EOF_COMMON_HELPERS'
# utc_timestamp -> the current time in UTC, ISO 8601 to the second
utc_timestamp() { date -u +%Y-%m-%dT%H:%M:%SZ; }
# log_line TEXT...: appends "TIME TEXT" to $LOG, and TEXT to stderr unless
# QUIET=1. Reads LOG and QUIET.
log_line() {
  printf '%s %s\n' "$(utc_timestamp)" "$*" >>"$LOG"
  [ "${QUIET:-0}" = 1 ] || printf '%s\n' "$*" >&2
}
# hash_file FILE -> the sha256 of FILE in lower-case hex, or "" when it cannot be
# read or HASH_TOOL is none. Reads HASH_TOOL. FILE goes on stdin, so its name is
# never escaped in the hasher's output.
hash_file() {
  case "$HASH_TOOL" in
    sha256sum) sha256sum <"$1" 2>/dev/null | cut -d' ' -f1 ;;
    shasum)    shasum -a 256 <"$1" 2>/dev/null | cut -d' ' -f1 ;;
    sha256)    sha256 -q <"$1" 2>/dev/null ;;
    openssl)   openssl dgst -sha256 -r <"$1" 2>/dev/null | cut -d' ' -f1 ;;
    *)         printf '' ;;
  esac
}
# stat_file FILE -> "size mtime atime ctime btime uid gid mode" of FILE itself
# (lstat), or nothing when stat fails. Reads STAT_MODE.
stat_file() {
  case "$STAT_MODE" in
    gnu)  stat -c '%s %Y %X %Z %W %u %g %a' "$1" 2>/dev/null ;;
    gnu0) stat -c '%s %Y %X %Z 0 %u %g %a' "$1" 2>/dev/null ;;
    bsd)  stat -f '%z %m %a %c %B %u %g %Lp' "$1" 2>/dev/null ;;
    *)    printf '%s 0 0 0 0 0 0 0\n' "$(wc -c <"$1" 2>/dev/null | tr -d ' ')" ;;
  esac
}
# stat_files FILE... -> one stat_file line per FILE from a single stat call, or
# return 1 when there is no stat. Reads STAT_MODE. A file stat cannot read prints
# no line, so callers check the line count.
stat_files() {
  case "$STAT_MODE" in
    gnu)  stat -c '%s %Y %X %Z %W %u %g %a' "$@" 2>/dev/null ;;
    gnu0) stat -c '%s %Y %X %Z 0 %u %g %a' "$@" 2>/dev/null ;;
    bsd)  stat -f '%z %m %a %c %B %u %g %Lp' "$@" 2>/dev/null ;;
    *)    return 1 ;;
  esac
}
# hash_files FILE... -> one "hash name" line per readable FILE from a single
# hasher call, or return 1 when HASH_TOOL is none. The hashers escape a name
# holding a backslash, newline or carriage return, so such names use hash_file.
hash_files() {
  case "$HASH_TOOL" in
    sha256sum) sha256sum "$@" 2>/dev/null ;;
    shasum)    shasum -a 256 "$@" 2>/dev/null ;;
    sha256)    sha256 -r "$@" 2>/dev/null ;;
    openssl)   openssl dgst -sha256 -r "$@" 2>/dev/null ;;
    *)         return 1 ;;
  esac
}
# hash_match LINE FILE -> 0, with the hash in _ret_hash, when LINE is hash_files'
# line for FILE. Reads HASH_TOOL (sha256 -r puts one space before the name, the
# others two or " *"). Temporaries: _hm_.
hash_match() {
  _ret_hash=${1%% *}
  case "$_ret_hash" in '' | *[!0-9a-f]*) return 1 ;; esac
  [ "${#_ret_hash}" -eq 64 ] || return 1
  _hm_name=${1#* }
  [ "$HASH_TOOL" = sha256 ] || _hm_name=${_hm_name#?}
  [ "$_hm_name" = "$2" ]
}
# The characters json_escape handles, as globals, since POSIX sh has no $'\t'.
TAB=$(printf '\t'); CARRIAGE_RETURN=$(printf '\r'); NEWLINE='
'
# json_escape TEXT -> TEXT as a JSON string body (backslash, double quote, tab,
# carriage return and newline escaped); awk starts only when one is present.
json_escape() {
  case "$1" in
    *\\*|*'"'*|*"$TAB"*|*"$CARRIAGE_RETURN"*|*"$NEWLINE"*) ;;
    *) printf '%s' "$1"; return ;;
  esac
  printf '%s' "$1" | awk 'BEGIN{ORS=""} NR>1{printf "\\n"} {
    count=length($0)
    for(i=1;i<=count;i++){ char=substr($0,i,1)
      if(char=="\\") printf "\\\\"; else if(char=="\"") printf "\\\"";
      else if(char=="\t") printf "\\t"; else if(char=="\r") printf "\\r";
      else printf "%s", char } }'
}
# to_number VALUE -> VALUE when it is a non-negative integer, else 0
to_number() { case "$1" in ''|*[!0-9]*) printf '0' ;; *) printf '%s' "$1" ;; esac; }
# json_escape_var VALUE / to_number_var VALUE: json_escape and to_number,
# assigned to _ret_json and _ret_number, so the common case forks nothing.
json_escape_var() {
  case "$1" in
    *\\*|*'"'*|*"$TAB"*|*"$CARRIAGE_RETURN"*|*"$NEWLINE"*) _ret_json=$(json_escape "$1") ;;
    *) _ret_json=$1 ;;
  esac
}
to_number_var() { case "$1" in ''|*[!0-9]*) _ret_number=0 ;; *) _ret_number=$1 ;; esac; }
# define_secret_test: defines is_secret REL, true when REL matches a secret glob.
# Reads SECRET_PATTERN. One case pattern runs in the shell's own matcher, much
# faster under bash and posh than a loop of one case per glob.
define_secret_test() {
  eval "is_secret() { case \"\$1\" in $SECRET_PATTERN) return 0 ;; esac; return 1; }"
}
EOF_COMMON_HELPERS
COMMON=$_ret_heredoc_text
eval "$COMMON"

# add_note TEXT: appends TEXT to NOTES (collection.json "notes") and logs it.
# Sets NOTES, so call it in the main shell, never in a pipeline or $(...).
add_note() { NOTES="$NOTES
$1"; log_line "NOTE: $1"; }

# compile_secret_pattern: sets SECRET_PATTERN to the lines of SECRET_GLOBS joined
# with |, spaces escaped, for define_secret_test. Reads SECRET_GLOBS.
# tests/catalog-sync.sh checks the globs hold nothing else the pattern would read.
compile_secret_pattern() {
  SECRET_PATTERN=$(printf '%s\n' "$SECRET_GLOBS" | sed 's/ /\\ /g' | tr '\n' '|')
  SECRET_PATTERN=${SECRET_PATTERN%|}
}

# file_id PATH -> device:inode of PATH itself, or the inode alone from ls -di
# without stat; empty when PATH cannot be stat'ed. Reads STAT_MODE.
# shellcheck disable=SC2012 # ls -di is the POSIX inode; only its first field is read
file_id() {
  case "$STAT_MODE" in
    gnu|gnu0) stat -c '%d:%i' "$1" 2>/dev/null ;;
    bsd)      stat -f '%d:%i' "$1" 2>/dev/null ;;
    *)        ls -di "$1" 2>/dev/null | awk 'NR==1 {print $1}' ;;
  esac
}

# ===========================================================================
# Manifest writer
# ===========================================================================
# Also part of COMMON: the workers write most manifest rows.
heredoc_var <<'EOF_COMMON_MANIFEST'
# write_manifest_row MANIFEST user home agent path archive_path type size
#   mtime atime ctime btime uid gid mode sha256 secret status target error
# Appends one JSON line to MANIFEST; a numeric field that is not a number is 0.
# Temporaries: _wmr_.
write_manifest_row() {
  json_escape_var "$2"; _wmr_user=$_ret_json
  json_escape_var "$3"; _wmr_home=$_ret_json
  json_escape_var "$4"; _wmr_agent=$_ret_json
  json_escape_var "$5"; _wmr_path=$_ret_json
  json_escape_var "$6"; _wmr_archive_path=$_ret_json
  json_escape_var "${19}"; _wmr_target=$_ret_json
  json_escape_var "${20}"; _wmr_error=$_ret_json
  to_number_var "$8"; _wmr_size=$_ret_number
  to_number_var "$9"; _wmr_mtime=$_ret_number
  to_number_var "${10}"; _wmr_atime=$_ret_number
  to_number_var "${11}"; _wmr_ctime=$_ret_number
  to_number_var "${12}"; _wmr_btime=$_ret_number
  to_number_var "${13}"; _wmr_uid=$_ret_number
  to_number_var "${14}"; _wmr_gid=$_ret_number
  printf '{"user":"%s","home":"%s","agent":"%s","path":"%s","archive_path":"%s",'\
'"type":"%s","size":%s,"mtime":%s,"atime":%s,"ctime":%s,"btime":%s,"uid":%s,'\
'"gid":%s,"mode":"%s","sha256":"%s","secret":%s,"status":"%s","target":"%s",'\
'"error":"%s"}\n' \
    "$_wmr_user" "$_wmr_home" "$_wmr_agent" "$_wmr_path" "$_wmr_archive_path" "$7" \
    "$_wmr_size" "$_wmr_mtime" "$_wmr_atime" "$_wmr_ctime" "$_wmr_btime" "$_wmr_uid" \
    "$_wmr_gid" "${15}" "${16}" "${17}" "${18}" "$_wmr_target" "$_wmr_error" >>"$1"
}
# get_dir_read_error DIR -> prints why DIR cannot be listed or entered and
# returns 0, else returns 1. test(1) decides; the text is ls(1)'s OS error.
get_dir_read_error() {
  get_dir_read_error_var "$1" || return 1
  printf '%s' "$_ret_dir_error"
}
# get_dir_read_error_var DIR: get_dir_read_error, assigned to _ret_dir_error, so
# the per-directory check forks only when it fails.
get_dir_read_error_var() {
  if [ ! -d "$1" ]; then
    if [ -e "$1" ] || [ -L "$1" ]; then
      _ret_dir_error='Not a directory'
    else
      _ret_dir_error=$(ls -ld "$1" 2>&1 >/dev/null)
      [ -n "$_ret_dir_error" ] || _ret_dir_error='No such file or directory'
    fi
  elif [ ! -r "$1" ]; then
    _ret_dir_error=$(ls "$1" 2>&1 >/dev/null)
    [ -n "$_ret_dir_error" ] || _ret_dir_error='cannot be listed (no read permission)'
  elif [ ! -x "$1" ]; then
    _ret_dir_error=$(ls -ld "$1/." 2>&1 >/dev/null)
    [ -n "$_ret_dir_error" ] || _ret_dir_error='cannot be entered (no search permission)'
  else
    return 1
  fi
  _ret_dir_error=${_ret_dir_error%%"$NEWLINE"*}
}
# write_read_error_row MANIFEST USER HOME AGENT DIR ERROR: one error_read row for
# DIR, with its lstat metadata and size 0, and a log line. Temporaries: _wrer_.
write_read_error_row() {
  _wrer_stat=$(stat_file "$5"); [ -n "$_wrer_stat" ] || _wrer_stat='0 0 0 0 0 0 0 0'
  case "$_wrer_stat" in ' '*) _wrer_stat="0$_wrer_stat" ;; esac
  # shellcheck disable=SC2086 # split the stat fields
  set -- "$@" $_wrer_stat
  write_manifest_row "$1" "$2" "$3" "$4" "$5" "" dir 0 "$8" "$9" "${10}" "${11}" \
    "${12}" "${13}" "${14}" "" false error_read "" "$6"
  log_line "read failed: $5: $6"
}
EOF_COMMON_MANIFEST
eval "$_ret_heredoc_text"
COMMON="$COMMON
$_ret_heredoc_text"

# ===========================================================================
# Staging files
# ===========================================================================
# find -exec cannot call a shell function, so the per-file work runs as
# programs held in <NAME>_WORKER: the text of COMMON followed by the worker's
# own, started as find ... -exec "$WORKER_SH" -c "$PROGRAM" sh ARGS... {} +.
# A worker's variables need no prefix; its first line names its arguments.
# collectors/docs/internals.md, "Worker programs (sh)", has the design.

# STAGE_FILES_WORKER: stages every non-directory path find gives it and writes
# its manifest row. Arguments: stage_dir root user home agent manifest files...
# POSIX sh has no arrays, so each chunk of files is held in numbered variables
# (path_N, stat_N, ...); internals.md, "Worker programs (sh)", has the passes.
heredoc_var <<'EOF_STAGE_FILES_WORKER'
stage_dir=$1; root=$2; user=$3; home=$4; agent=$5; manifest=$6; shift 6
set -f
define_secret_test
if [ "${CAC_NO_BATCH:-0}" = 1 ]; then batch=0; else batch=1; fi
batch_stat=$batch; batch_copy=$batch; batch_hash=$batch
last_mkdir_dir=
# lstat_chunk: pass 1, before anything is copied. Reads path_N; sets stat_N.
lstat_chunk() {
  stat_batch_ok=0
  if [ "$batch_stat" = 1 ]; then
    set --
    i=1; while [ "$i" -le "$chunk_size" ]; do
      eval "set -- \"\$@\" \"\$path_$i\""; i=$((i + 1))
    done
    stat_lines=$(stat_files "$@")
    saved_ifs=$IFS; IFS=$NEWLINE
    set -- $stat_lines
    IFS=$saved_ifs
    if [ "$#" -eq "$chunk_size" ]; then
      i=1; for stat_line in "$@"; do eval "stat_$i=\$stat_line"; i=$((i + 1)); done
      stat_batch_ok=1
    fi
  fi
  if [ "$stat_batch_ok" = 0 ]; then
    i=1; while [ "$i" -le "$chunk_size" ]; do
      eval "path=\$path_$i"; stat_line=$(stat_file "$path"); eval "stat_$i=\$stat_line"
      i=$((i + 1))
    done
  fi
}
# plan_chunk: pass 2; makes the staging directories and symlinks and sets status_N,
# type_N, secret_N and target_N. Reads path_N, stat_N, NO_SECRETS, MAX_SIZE. A
# special file is never copied, since cp would block on a FIFO.
plan_chunk() {
  set --
  i=1; while [ "$i" -le "$chunk_size" ]; do
    eval "path=\$path_$i"; dest_dir="$stage_dir/fs${path#"$root"}"; dest_dir=${dest_dir%/*}
    if [ "$dest_dir" != "$last_mkdir_dir" ]; then
      set -- "$@" "$dest_dir"; last_mkdir_dir=$dest_dir
    fi
    i=$((i + 1))
  done
  [ "$#" -eq 0 ] || mkdir -p "$@" 2>>"$LOG"
  i=1; while [ "$i" -le "$chunk_size" ]; do
    eval "path=\$path_$i; stat_line=\$stat_$i"
    [ -n "$stat_line" ] || stat_line='0 0 0 0 0 0 0 0'
    set -- $stat_line
    home_rel=${path#"$home"/}
    secret=false
    if is_secret "$home_rel"; then secret=true; fi
    target=; type=file
    if [ -L "$path" ]; then
      status=symlink; type=symlink
      target=$(readlink "$path" 2>/dev/null)
      ln -s "$target" "$stage_dir/fs${path#"$root"}" 2>/dev/null
    elif [ -e "$path" ] && [ ! -f "$path" ]; then
      status=skipped_special
      if [ -S "$path" ]; then type=socket
      elif [ -p "$path" ]; then type=fifo
      elif [ -b "$path" ] || [ -c "$path" ]; then type=device
      else type=other; fi
    elif [ "$secret" = true ] && [ "${NO_SECRETS:-0}" = 1 ]; then
      status=skipped_secret
    elif to_number_var "$1" && [ "${MAX_SIZE:-0}" -gt 0 ] && [ "$_ret_number" -gt "$MAX_SIZE" ]
    then
      status=skipped_size
    else
      status=copy
    fi
    eval "status_$i=\$status; type_$i=\$type; secret_$i=\$secret; target_$i=\$target"
    eval "copy_error_$i=; hash_$i="
    i=$((i + 1))
  done
}
# copy_run FIRST LAST DIR: copies the "copy" entries FIRST..LAST of one directory
# into DIR and sets their status_N and copy_error_N. A cp -p of the whole run is
# redone per file when it fails, so each failing file gets its own error.
copy_run() {
  copy_first=$1; copy_last=$2; copy_dir=$3; copy_batch_ok=0
  set --
  j=$copy_first; while [ "$j" -le "$copy_last" ]; do
    eval "status=\$status_$j"
    if [ "$status" = copy ]; then eval "set -- \"\$@\" \"\$path_$j\""; fi
    j=$((j + 1))
  done
  if [ "$batch_copy" = 1 ] && [ "$#" -gt 1 ]; then
    if copy_batch_error=$(cp -p "$@" "$copy_dir/" 2>&1) && [ -z "$copy_batch_error" ]; then
      copy_batch_ok=1
    fi
  fi
  j=$copy_first; while [ "$j" -le "$copy_last" ]; do
    eval "status=\$status_$j"
    if [ "$status" = copy ] && [ "$copy_batch_ok" = 1 ]; then
      eval "status_$j=collected"
    elif [ "$status" = copy ]; then
      eval "path=\$path_$j"; dest="$stage_dir/fs${path#"$root"}"
      if copy_error=$(cp -p "$path" "$dest" 2>&1); then
        status=collected
      else
        status=error_copy
        rm -f "$dest" 2>/dev/null
        log_line "copy failed: $path: $copy_error"
      fi
      eval "status_$j=\$status; copy_error_$j=\$copy_error"
    fi
    j=$((j + 1))
  done
}
# copy_chunk: pass 3, one copy_run per run of consecutive "copy" entries whose
# staging directory is the same. Reads status_N and path_N.
copy_chunk() {
  run_start=0; run_end=0; run_dir=
  i=1; while [ "$i" -le "$chunk_size" ]; do
    eval "status=\$status_$i"
    if [ "$status" = copy ]; then
      eval "path=\$path_$i"; dest_dir="$stage_dir/fs${path#"$root"}"; dest_dir=${dest_dir%/*}
      if [ "$run_start" -gt 0 ] && [ "$dest_dir" != "$run_dir" ]; then
        copy_run "$run_start" "$run_end" "$run_dir"; run_start=0
      fi
      [ "$run_start" -gt 0 ] || { run_start=$i; run_dir=$dest_dir; }
      run_end=$i
    fi
    i=$((i + 1))
  done
  [ "$run_start" -eq 0 ] || copy_run "$run_start" "$run_end" "$run_dir"
}
# hash_chunk: pass 4; sets hash_N for every collected entry, with one hasher call
# for the chunk. Reads status_N, path_N, HASH_TOOL.
hash_chunk() {
  [ "$HASH_TOOL" != none ] || return 0
  set --
  if [ "$batch_hash" = 1 ]; then
    i=1; while [ "$i" -le "$chunk_size" ]; do
      eval "status=\$status_$i; path=\$path_$i"
      if [ "$status" = collected ]; then
        dest="$stage_dir/fs${path#"$root"}"
        case "$dest" in
          *\\* | *"$NEWLINE"* | *"$CARRIAGE_RETURN"*) ;;
          *) set -- "$@" "$dest" ;;
        esac
      fi
      i=$((i + 1))
    done
  fi
  hash_lines=
  [ "$#" -eq 0 ] || hash_lines=$(hash_files "$@")
  saved_ifs=$IFS; IFS=$NEWLINE
  set -- $hash_lines
  IFS=$saved_ifs
  i=1; while [ "$i" -le "$chunk_size" ]; do
    eval "status=\$status_$i; path=\$path_$i"
    if [ "$status" = collected ]; then
      dest="$stage_dir/fs${path#"$root"}"
      if [ "$#" -gt 0 ] && hash_match "$1" "$dest"; then
        sha256=$_ret_hash; shift
      else
        sha256=$(hash_file "$dest")
      fi
      eval "hash_$i=\$sha256"
    fi
    i=$((i + 1))
  done
}
# write_chunk_rows: pass 5, one manifest row per entry, in find's order.
# Reads every numbered variable.
write_chunk_rows() {
  i=1; while [ "$i" -le "$chunk_size" ]; do
    eval "path=\$path_$i; stat_line=\$stat_$i; status=\$status_$i; secret=\$secret_$i"
    eval "type=\$type_$i; target=\$target_$i; copy_error=\$copy_error_$i; sha256=\$hash_$i"
    [ -n "$stat_line" ] || stat_line='0 0 0 0 0 0 0 0'
    set -- $stat_line
    case "$status" in
      collected | symlink) archive_path="fs${path#"$root"}" ;;
      *) archive_path= ;;
    esac
    write_manifest_row "$manifest" "$user" "$home" "$agent" "$path" "$archive_path" "$type" \
      "$1" "$2" "$3" "$4" "$5" "$6" "$7" "$8" "$sha256" "$secret" "$status" "$target" \
      "$copy_error"
    i=$((i + 1))
  done
}
while [ "$#" -gt 0 ]; do
  chunk_size=0; chunk_bytes=0
  while [ "$#" -gt 0 ] && [ "$chunk_size" -lt 256 ] && [ "$chunk_bytes" -lt 65536 ]; do
    chunk_size=$((chunk_size + 1)); eval "path_$chunk_size=\$1"
    chunk_bytes=$((chunk_bytes + ${#1})); shift
  done
  lstat_chunk
  plan_chunk
  copy_chunk
  hash_chunk
  write_chunk_rows
done
EOF_STAGE_FILES_WORKER
STAGE_FILES_WORKER="$COMMON
$_ret_heredoc_text"

# EXCLUDED_PATHS_WORKER: one skipped_excluded row per excluded path, a
# directory's size from du -sk. Arguments: root user home agent manifest paths...
heredoc_var <<'EOF_EXCLUDED_PATHS_WORKER'
root=$1; user=$2; home=$3; agent=$4; manifest=$5; shift 5
for path in "$@"; do
  if [ -d "$path" ]; then
    type=dir; size_kib=$(du -sk "$path" 2>/dev/null | cut -f1)
    to_number_var "$size_kib"; size=$((_ret_number * 1024))
    stat_line='0 0 0 0 0 0 0 0'
  else
    type=file; stat_line=$(stat_file "$path"); [ -n "$stat_line" ] || stat_line='0 0 0 0 0 0 0 0'
    size=${stat_line%% *}
  fi
  set -- $stat_line
  write_manifest_row "$manifest" "$user" "$home" "$agent" "$path" "" "$type" "$size" \
    "$2" "$3" "$4" "$5" "$6" "$7" "$8" "" false skipped_excluded "" ""
done
EOF_EXCLUDED_PATHS_WORKER
EXCLUDED_PATHS_WORKER="$COMMON
$_ret_heredoc_text"

# UNREADABLE_DIRS_WORKER: one error_read row per directory that cannot be listed
# or entered, decided by test(1), since find's message is localised.
# Arguments: user home agent manifest dirs...
heredoc_var <<'EOF_UNREADABLE_DIRS_WORKER'
user=$1; home=$2; agent=$3; manifest=$4; shift 4
for dir in "$@"; do
  if get_dir_read_error_var "$dir"; then
    write_read_error_row "$manifest" "$user" "$home" "$agent" "$dir" "$_ret_dir_error"
  fi
done
EOF_UNREADABLE_DIRS_WORKER
UNREADABLE_DIRS_WORKER="$COMMON
$_ret_heredoc_text"

# INVENTORY_FILES_WORKER: prints F|agent|size|mtime per regular file under
# --inventory and writes nothing. Arguments: agent files...
heredoc_var <<'EOF_INVENTORY_FILES_WORKER'
agent=$1; shift
set -f
# print_file_rows COUNT LINES: the F rows for the stat LINES when there are
# COUNT of them; returns 1 without output otherwise.
print_file_rows() {
  expected_count=$1; saved_ifs=$IFS; IFS=$NEWLINE
  set -- $2
  IFS=$saved_ifs
  [ "$#" -eq "$expected_count" ] || return 1
  for stat_line in "$@"; do
    to_number_var "${stat_line%% *}"; size=$_ret_number
    stat_line=${stat_line#* }; to_number_var "${stat_line%% *}"
    printf 'F|%s|%s|%s\n' "$agent" "$size" "$_ret_number"
  done
}
stat_lines=
[ "${CAC_NO_BATCH:-0}" = 1 ] || stat_lines=$(stat_files "$@")
if ! print_file_rows "$#" "$stat_lines"; then
  for path in "$@"; do
    stat_line=$(stat_file "$path")
    set -- $stat_line
    to_number_var "${1:-}"; size=$_ret_number; to_number_var "${2:-}"
    printf 'F|%s|%s|%s\n' "$agent" "$size" "$_ret_number"
  done
fi
EOF_INVENTORY_FILES_WORKER
INVENTORY_FILES_WORKER="$COMMON
$_ret_heredoc_text"

# ===========================================================================
# Catalog walk
# ===========================================================================
# A table is expanded in full before anything is walked, so a match nested in
# another (.gemini/antigravity-cli in .gemini) is pruned from the outer walk.

# expand_glob BASE PATTERN -> the paths matching BASE/PATTERN that exist or are
# symlinks, one per line. IFS is newline only, so a BASE with spaces stays one
# word. Temporaries: _eg_.
expand_glob() {
  _eg_saved_ifs=$IFS
  IFS='
'
  for _eg_match in $1/$2; do
    if [ -e "$_eg_match" ] || [ -L "$_eg_match" ]; then printf '%s\n' "$_eg_match"; fi
  done
  IFS=$_eg_saved_ifs
}

# classify_case_matches: agent|glob|path lines on stdin -> the K, D and R
# instruction lines fold_case_matches reads; collectors/docs/internals.md,
# "Claimed paths and case variants", lists them.
classify_case_matches() {
  LC_ALL=C awk -F'|' '
    NF >= 3 { count++; line[count] = $0; agent[count] = $1; glob[count] = $2
      path[count] = substr($0, length($1) + length($2) + 3); lower[count] = tolower(path[count]) }
    END { for (i = 1; i <= count; i++) {
      same = 0
      for (j = 1; j < i; j++) if (lower[j] == lower[i]) { same = j; break }
      if (same) { print "@|" path[same]; print "D|" line[i]; continue }
      parent = 0
      for (j = 1; j <= count; j++) {
        if (j == i) continue; parent_len = length(lower[j])
        if (length(lower[i]) > parent_len + 1 &&
            substr(lower[i], 1, parent_len + 1) == lower[j] "/" &&
            substr(path[i], 1, parent_len) != path[j]) {
          if (!parent || parent_len < length(lower[parent])) parent = j }
      }
      if (parent) { parent_len = length(path[parent])
        print "@|" path[parent]; print "^|" substr(path[i], 1, parent_len)
        print "R|" agent[i] "|" glob[i] "|" substr(path[i], parent_len + 1); continue }
      print "K|" line[i] } }'
}

# fold_case_matches: classify_case_matches lines on stdin -> agent|glob|path lines
# with the spellings of one on-disk path folded together, plus #|message lines
# to log. Only colliding paths are stat'ed. Temporaries: _fcm_.
fold_case_matches() {
  while IFS= read -r _fcm_line; do
    case "$_fcm_line" in
      ('@|'*) _fcm_ref=${_fcm_line#??} ;;
      ('^|'*) _fcm_prefix=${_fcm_line#??} ;;
      ('K|'*) printf '%s\n' "${_fcm_line#??}" ;;
      ('D|'*) _fcm_line=${_fcm_line#??}; _fcm_match=${_fcm_line#*|}; _fcm_match=${_fcm_match#*|}
        _fcm_file_id=$(file_id "$_fcm_match")
        if [ "$_fcm_match" = "$_fcm_ref" ] ||
          { [ -n "$_fcm_file_id" ] && [ "$_fcm_file_id" = "$(file_id "$_fcm_ref")" ]; }; then
          printf '#|  [%s] %s: same file as %s (case-insensitive filesystem), collected there\n' \
            "${_fcm_line%%|*}" "$_fcm_match" "$_fcm_ref"
        else printf '%s\n' "$_fcm_line"; fi ;;
      ('R|'*) _fcm_line=${_fcm_line#??}; _fcm_agent=${_fcm_line%%|*}; _fcm_rest=${_fcm_line#*|}
        _fcm_glob=${_fcm_rest%%|*}; _fcm_rest=${_fcm_rest#*|}
        _fcm_file_id=$(file_id "$_fcm_prefix")
        if [ -n "$_fcm_file_id" ] && [ "$_fcm_file_id" = "$(file_id "$_fcm_ref")" ]; then
          printf '%s|%s|%s%s\n' "$_fcm_agent" "$_fcm_glob" "$_fcm_ref" "$_fcm_rest"
        else printf '%s|%s|%s%s\n' "$_fcm_agent" "$_fcm_glob" "$_fcm_prefix" "$_fcm_rest"; fi ;;
    esac
  done
}

# expand_catalog BASE TABLE -> agent|glob|path for every match of TABLE under
# BASE, in table order, case variants folded, plus #|message lines to log.
# Temporaries: _ec_.
expand_catalog() {
  printf '%s\n' "$2" | while IFS='|' read -r _ec_agent _ec_glob; do
    case "$_ec_agent" in (''|'#'*) continue ;; esac
    expand_glob "$1" "$_ec_glob" | while IFS= read -r _ec_match; do
      printf '%s|%s|%s\n' "$_ec_agent" "$_ec_glob" "$_ec_match"
    done
  done | classify_case_matches | fold_case_matches
}

# collect_catalog USER BASE TABLE [HEADER]: collects every match of TABLE under
# BASE, logging HEADER before the first one. Reads INVENTORY; sets NESTED_CLAIMS
# while it runs, so a match inside another is collected once, under its own
# agent. Temporaries: _cc_.
collect_catalog() {
  _cc_header=${4:-}
  _cc_matches=$(expand_catalog "$2" "$3")
  NESTED_CLAIMS=$(printf '%s\n' "$_cc_matches" | grep -v '^#|' | cut -d'|' -f3-)
  printf '%s\n' "$_cc_matches" | while IFS='|' read -r _cc_agent _cc_glob _cc_match; do
    [ -n "$_cc_agent" ] || continue
    if [ "$_cc_agent" = '#' ]; then
      if [ -n "$_cc_header" ]; then log_line "$_cc_header"; _cc_header=; fi
      if [ -n "$_cc_match" ]; then log_line "$_cc_glob|$_cc_match"; else log_line "$_cc_glob"; fi
      continue
    fi
    if [ "$INVENTORY" = 1 ]; then
      case "$_cc_agent" in shared) continue ;; esac
      printf 'M|%s|%s\n' "$_cc_agent" "$_cc_glob"
    fi
    if [ -n "$_cc_header" ]; then log_line "$_cc_header"; _cc_header=; fi
    log_line "  [$_cc_agent] $_cc_match"
    collect_path "$1" "$2" "$_cc_agent" "$_cc_match" </dev/null
  done
  NESTED_CLAIMS=
}

# test_excluded HOME PATH: true when PATH matches HOME/GLOB for a glob of
# ACTIVE_EXCLUDES, as find -path does. Reads ACTIVE_EXCLUDES. Temporaries: _te_.
test_excluded() {
  _te_saved_ifs=$IFS
  IFS='
'
  set -f
  for _te_glob in $ACTIVE_EXCLUDES; do
    [ -n "$_te_glob" ] || continue
    # shellcheck disable=SC2254 # the table entry is a glob
    case "$2" in "$1"/$_te_glob) set +f; IFS=$_te_saved_ifs; return 0 ;; esac
  done
  set +f
  IFS=$_te_saved_ifs
  return 1
}

# collect_path USER HOME AGENT PATH: collects one catalog match, a directory
# through collect_tree, anything else through the workers, since find never
# tests a starting point against EXCLUDES. Reads INVENTORY, ACTIVE_EXCLUDES,
# MANIFEST, STAGE, ROOT, WORKER_SH. Temporaries: _cp_.
collect_path() {
  _cp_user=$1; _cp_home=$2; _cp_agent=$3; _cp_match=$4
  if [ -d "$_cp_match" ] && [ ! -L "$_cp_match" ]; then
    # A match that cannot be listed or entered gets its error_read row here,
    # not from the walk: BSD find applies no expression to a starting point
    # it cannot read and GNU find does, so the pre-check gives both one row
    # and never two.
    if [ "$INVENTORY" != 1 ] && _cp_dir_error=$(get_dir_read_error "$_cp_match"); then
      write_read_error_row "$MANIFEST" "$_cp_user" "$_cp_home" "$_cp_agent" "$_cp_match" \
        "$_cp_dir_error"
      return 0
    fi
    collect_tree "$_cp_user" "$_cp_home" "$_cp_agent" "$_cp_match"
  elif [ -n "$ACTIVE_EXCLUDES" ] && test_excluded "$_cp_home" "$_cp_match"; then
    [ "$INVENTORY" = 1 ] || "$WORKER_SH" -c "$EXCLUDED_PATHS_WORKER" sh "$ROOT" "$_cp_user" \
      "$_cp_home" "$_cp_agent" "$MANIFEST" "$_cp_match"
  elif [ "$INVENTORY" = 1 ]; then
    if [ -f "$_cp_match" ] && [ ! -L "$_cp_match" ]; then
      "$WORKER_SH" -c "$INVENTORY_FILES_WORKER" sh "$_cp_agent" "$_cp_match"
    fi
  else
    "$WORKER_SH" -c "$STAGE_FILES_WORKER" sh "$STAGE" "$ROOT" "$_cp_user" "$_cp_home" \
      "$_cp_agent" "$MANIFEST" "$_cp_match"
  fi
}

# collect_tree USER HOME AGENT DIR: walks DIR with one find(1) that prunes the
# claimed and excluded paths and hands the rest to the workers. Reads
# NESTED_CLAIMS, ACTIVE_EXCLUDES, INVENTORY, MANIFEST, STAGE, ROOT, WORKER_SH,
# LOG. Temporaries: _ct_.
collect_tree() {
  _ct_user=$1; _ct_home=$2; _ct_agent=$3
  set -- "$4"
  _ct_saved_ifs=$IFS
  IFS='
'
  set -f
  # A path another catalog entry claimed is left to that entry.
  _ct_first=1
  for _ct_claim in $NESTED_CLAIMS; do
    case "$_ct_claim" in "$1"/*) ;; *) continue ;; esac
    if [ "$_ct_first" = 1 ]; then set -- "$@" '(' -path "$_ct_claim"; _ct_first=0
    else set -- "$@" -o -path "$_ct_claim"; fi
  done
  if [ "$_ct_first" = 0 ]; then set -- "$@" ')' -prune -o; fi
  _ct_first=1
  for _ct_exclude in $ACTIVE_EXCLUDES; do
    [ -n "$_ct_exclude" ] || continue
    if [ "$_ct_first" = 1 ]; then set -- "$@" '(' -path "$_ct_home/$_ct_exclude"; _ct_first=0
    else set -- "$@" -o -path "$_ct_home/$_ct_exclude"; fi
  done
  IFS=$_ct_saved_ifs; set +f
  if [ "$_ct_first" = 0 ] && [ "$INVENTORY" = 1 ]; then
    set -- "$@" ')' -prune -o
  elif [ "$_ct_first" = 0 ]; then
    set -- "$@" ')' -prune -exec "$WORKER_SH" -c "$EXCLUDED_PATHS_WORKER" sh "$ROOT" \
      "$_ct_user" "$_ct_home" "$_ct_agent" "$MANIFEST" '{}' + -o
  fi
  if [ "$INVENTORY" = 1 ]; then
    set -- "$@" -type f -exec "$WORKER_SH" -c "$INVENTORY_FILES_WORKER" sh "$_ct_agent" '{}' +
  else
    set -- "$@" -type d -exec "$WORKER_SH" -c "$UNREADABLE_DIRS_WORKER" sh \
      "$_ct_user" "$_ct_home" "$_ct_agent" "$MANIFEST" '{}' + -o
    set -- "$@" ! -type d -exec "$WORKER_SH" -c "$STAGE_FILES_WORKER" sh \
      "$STAGE" "$ROOT" "$_ct_user" "$_ct_home" "$_ct_agent" "$MANIFEST" '{}' +
  fi
  find "$@" 2>>"$LOG"
}

# ===========================================================================
# Users
# ===========================================================================

# enumerate_users -> user:home lines from passwd, dscl and the usual home
# parents, duplicates included. The home patterns stay quoted, so the shell never
# expands them against the current directory. Reads MODE and ROOT. Temporaries:
# _eu_.
enumerate_users() {
  {
    if [ "$MODE" = live ] && command -v getent >/dev/null 2>&1; then
      getent passwd 2>/dev/null
    elif [ -f "$ROOT/etc/passwd" ]; then
      cat "$ROOT/etc/passwd"
    fi
  } | awk -F: -v root="$ROOT" 'NF>=6 && $6!="" {print $1":"root$6}'
  if [ "$MODE" = live ] && command -v dscl >/dev/null 2>&1; then
    dscl . -list /Users NFSHomeDirectory 2>/dev/null | awk 'NF>=2 {print $1":"$2}'
  fi
  for _eu_glob in 'home/*' 'Users/*' root var/root 'usr/home/*' 'export/home/*'; do
    expand_glob "$ROOT" "$_eu_glob" | while IFS= read -r _eu_home; do
      [ -d "$_eu_home" ] && printf '%s:%s\n' "${_eu_home##*/}" "$_eu_home"
    done
  done
}

# filter_users: user:home lines on stdin -> the same lines without system and
# shared profile homes, users -u did not name, and repeated homes. Reads ROOT and
# USERS.
filter_users() {
  awk -F: -v root="$ROOT" -v want="$USERS" '
    BEGIN { wanted_count=split(want, wanted, ",")
      for(i=1;i<=wanted_count;i++) if (wanted[i]!="") selected[wanted[i]]=1 }
    {
      user=$1; home=substr($0, index($0, ":")+1)
      rel=home; if (root!="" && index(home, root)==1) rel=substr(home, length(root)+1)
      if (rel=="" || rel=="/" || rel=="/bin" || rel=="/sbin" || rel=="/usr" ||
          rel=="/usr/bin" || rel=="/usr/sbin" || rel=="/dev" || rel=="/dev/null" ||
          rel=="/proc" || rel=="/sys" || rel=="/nonexistent" || rel=="/var/empty" ||
          rel=="/Users/Shared" || rel=="/Users/Public" || rel=="/Users/Default" ||
          rel=="/Users/All Users" || rel=="/Users/Default User") next
      if (wanted_count>0 && !(user in selected)) next
      if (seen[home]++) next
      print user":"home
    }'
}

# get_user_homes -> user:home lines for the homes to collect, in enumeration
# order. Temporaries: _guh_.
get_user_homes() {
  enumerate_users | filter_users | while IFS=: read -r _guh_user _guh_home; do
    [ -d "$_guh_home" ] && printf '%s:%s\n' "$_guh_user" "$_guh_home"
  done
}

# collect_user_homes: collects CATALOG in every home of USER_LIST, with an
# error_read row for a home that cannot be read. The catalog is still tried, since
# a home mode 711 yields its non-glob entries. Reads USER_LIST, CATALOG,
# MANIFEST. Temporaries: _cuh_.
collect_user_homes() {
  printf '%s\n' "$USER_LIST" | while IFS=: read -r _cuh_user _cuh_home; do
    [ -n "$_cuh_home" ] || continue
    if _cuh_dir_error=$(get_dir_read_error "$_cuh_home"); then
      write_read_error_row "$MANIFEST" "$_cuh_user" "$_cuh_home" "" "$_cuh_home" \
        "$_cuh_dir_error"
    fi
    collect_catalog "$_cuh_user" "$_cuh_home" "$CATALOG" "User $_cuh_user ($_cuh_home)"
  done
}

# ===========================================================================
# Docker and Podman volumes
# ===========================================================================
# Volumes are found on disk, never with the docker binary. The functions that
# set the DOCKER_* globals run in the main shell, never in a pipeline or $(...).

# get_docker_volume_rule NAME -> the first DOCKER_VOLUMES line whose glob matches
# NAME, or nothing. Temporaries: _gdvr_.
get_docker_volume_rule() {
  _gdvr_saved_ifs=$IFS
  IFS='
'
  set -f
  for _gdvr_line in $DOCKER_VOLUMES; do
    case "$_gdvr_line" in ''|'#'*) continue ;; esac
    _gdvr_glob=${_gdvr_line#*|}
    # shellcheck disable=SC2254 # the table entry is a glob
    case "$1" in
      $_gdvr_glob) set +f; IFS=$_gdvr_saved_ifs; printf '%s' "$_gdvr_line"; return 0 ;;
    esac
  done
  set +f
  IFS=$_gdvr_saved_ifs
}

# find_errors_since LINE -> number of find(1) error lines appended to LOG
# after its line LINE
find_errors_since() { tail -n "+$(( $1 + 1 ))" "$LOG" 2>/dev/null | grep -c '^find: '; }

# test_volume_root BASE REL: returns 0 and sets _ret_volume_dir when every
# component of BASE/REL can be entered and the last listed; else returns 1, with a
# note for a symlink or unreadable component. Sets DOCKER_SEEN and
# DOCKER_UNREADABLE. Temporaries: _tvr_.
test_volume_root() {
  _tvr_dir=$1; _tvr_rest=$2/
  while [ -n "$_tvr_rest" ]; do
    _tvr_dir="$_tvr_dir/${_tvr_rest%%/*}"; _tvr_rest=${_tvr_rest#*/}
    if [ -L "$_tvr_dir" ]; then
      DOCKER_SEEN=1
      add_note "docker: $_tvr_dir is a symlink and was not followed; collect its target by hand"
      return 1
    fi
    [ -d "$_tvr_dir" ] || return 1
    if [ ! -x "$_tvr_dir" ] || { [ -z "$_tvr_rest" ] && [ ! -r "$_tvr_dir" ]; }; then
      DOCKER_SEEN=1; DOCKER_UNREADABLE=$(( DOCKER_UNREADABLE + 1 ))
      add_note "docker: $_tvr_dir is not readable; run as root to collect the volumes under it"
      return 1
    fi
  done
  _ret_volume_dir=$_tvr_dir
  return 0
}

# collect_volume VOLUME: collects one matched volume's _data as user docker, or
# records an unmatched or unreadable one. Sets the DOCKER_* counters, DOCKER_INV
# and NOTES. Temporaries: _cv_.
collect_volume() {
  _cv_name=${1##*/}; _cv_data="$1/_data"
  _cv_rule=$(get_docker_volume_rule "$_cv_name")
  _cv_agent=${_cv_rule%%|*}
  if [ ! -r "$1" ] || [ ! -x "$1" ] ||
    { [ -d "$_cv_data" ] && { [ ! -r "$_cv_data" ] || [ ! -x "$_cv_data" ]; }; }; then
    DOCKER_UNREADABLE=$(( DOCKER_UNREADABLE + 1 ))
    _cv_volume="volume $_cv_name (${_cv_agent:-unmatched})"
    add_note "docker: $_cv_volume at $1 is not readable; run as root to collect it"
  elif [ -L "$_cv_data" ] || [ ! -d "$_cv_data" ]; then
    log_line "  volume $_cv_name has no _data directory, skipped"
  elif [ -z "$_cv_agent" ] && [ "$INVENTORY" = 1 ]; then
    :
  elif [ -z "$_cv_agent" ]; then
    _cv_size_kib=$(du -sk "$_cv_data" 2>/dev/null | cut -f1)
    write_manifest_row "$MANIFEST" docker "$_cv_data" "" "$_cv_data" "" dir \
      "$(( $(to_number "$_cv_size_kib") * 1024 ))" 0 0 0 0 0 0 0 "" false \
      skipped_unmatched_volume "" ""
  elif [ "$INVENTORY" = 1 ]; then
    log_line "  [$_cv_agent] $_cv_data"
    DOCKER_INV="$DOCKER_INV
M|$_cv_rule
$(collect_path docker "$_cv_data" "$_cv_agent" "$_cv_data" </dev/null)"
    DOCKER_COLLECTED=$(( DOCKER_COLLECTED + 1 ))
  else
    log_line "  [$_cv_agent] $_cv_data"
    _cv_log_lines=$(wc -l <"$LOG" | tr -d ' ')
    collect_path docker "$_cv_data" "$_cv_agent" "$_cv_data" </dev/null
    _cv_find_errors=$(to_number "$(find_errors_since "$(to_number "$_cv_log_lines")")")
    DOCKER_COLLECTED=$(( DOCKER_COLLECTED + 1 ))
    if [ "$_cv_find_errors" -gt 0 ]; then
      DOCKER_UNREADABLE=$(( DOCKER_UNREADABLE + 1 ))
      _cv_note="docker: volume $_cv_name ($_cv_agent) could not be walked completely"
      _cv_note="$_cv_note ($_cv_find_errors unreadable paths, see collector.log)"
      add_note "$_cv_note; run as root to collect it"
    fi
  fi
}

# collect_volume_dir BASE REL: collect_volume for every volume under BASE/REL.
# Sets DOCKER_SEEN and DOCKER_FOUND. Temporaries: _cvd_.
collect_volume_dir() {
  test_volume_root "$1" "$2" || return
  _cvd_dir=$_ret_volume_dir
  DOCKER_SEEN=1
  log_line "Docker volumes in $_cvd_dir"
  _cvd_list=$(expand_glob "$_cvd_dir" '*')
  while IFS= read -r _cvd_volume; do
    [ -n "$_cvd_volume" ] || continue
    if [ -L "$_cvd_volume" ] || [ ! -d "$_cvd_volume" ]; then continue; fi
    DOCKER_FOUND=$(( DOCKER_FOUND + 1 ))
    collect_volume "$_cvd_volume"
  done <<EOF_VOLUME_LIST
$_cvd_list
EOF_VOLUME_LIST
}

# collect_docker_volumes: collects the volumes of the system and per-user Docker
# and Podman data roots and notes Docker Desktop data, whose volumes are inside a
# VM disk. Reads USER_LIST; sets the DOCKER_* counters and NOTES. Temporaries:
# _cdv_.
collect_docker_volumes() {
  collect_volume_dir "$ROOT/var/lib" docker/volumes </dev/null
  collect_volume_dir "$ROOT/var/lib" containers/storage/volumes </dev/null
  collect_volume_dir "$ROOT/ProgramData" Docker/volumes </dev/null
  _cdv_desktop_dirs="$ROOT/ProgramData/DockerDesktop"
  while IFS= read -r _cdv_line; do
    [ -n "$_cdv_line" ] || continue
    _cdv_home=${_cdv_line#*:}
    collect_volume_dir "$_cdv_home/.local/share" docker/volumes </dev/null
    collect_volume_dir "$_cdv_home/.local/share" containers/storage/volumes </dev/null
    _cdv_desktop_dirs="$_cdv_desktop_dirs
$_cdv_home/Library/Containers/com.docker.docker
$_cdv_home/AppData/Local/Docker"
  done <<EOF_USER_LIST
$USER_LIST
EOF_USER_LIST
  while IFS= read -r _cdv_dir; do
    if [ -n "$_cdv_dir" ] && [ -d "$_cdv_dir" ] && [ ! -L "$_cdv_dir" ]; then
      DOCKER_SEEN=1; DOCKER_DESKTOP=1
      _cdv_note="docker: Docker Desktop data at $_cdv_dir;"
      add_note "$_cdv_note volumes inside its virtual machine disk are not collected"
    fi
  done <<EOF_DOCKER_DESKTOP_DIRS
$_cdv_desktop_dirs
EOF_DOCKER_DESKTOP_DIRS
}

# ===========================================================================
# Project discovery
# ===========================================================================
# Project paths read with grep from each home's plain-text agent state. The
# groups keep their order, which sets the manifest row order; internals.md,
# "Project discovery", lists them.

# test_inventory_secret FILE: true under --inventory when FILE matches a secret
# glob, so inventory never reads a credential file. Reads INVENTORY and
# DISCOVERY_HOME. Temporaries: _tis_.
test_inventory_secret() {
  [ "$INVENTORY" = 1 ] || return 1
  _tis_rel=${1#"$DISCOVERY_HOME"/}
  is_secret "$_tis_rel"
}
# Eval'd by each discovery helper to drop the files test_inventory_secret rejects
# from "$@", which a function could not change. Temporaries: _ids_.
# shellcheck disable=SC2016 # expanded where it is eval'd, not here
INVENTORY_DROP_SECRETS='if [ "$INVENTORY" = 1 ]; then
  _ids_count=$#
  for _ids_file do test_inventory_secret "$_ids_file" || set -- "$@" "$_ids_file"; done
  shift "$_ids_count"
fi'
# json_vals KEY FILE... -> values of "KEY":"..." across the given files.
# Temporaries: _jv_.
json_vals() {
  _jv_key=$1; shift
  eval "$INVENTORY_DROP_SECRETS"
  [ $# -gt 0 ] || return 0
  grep -ho "\"$_jv_key\": *\"[^\"]*\"" "$@" 2>/dev/null | sed "s/^\"$_jv_key\": *\"//; s/\"\$//"
}
# json_path_keys FILE NEXT -> the keys of FILE that are absolute paths followed by
# ": NEXT" (a basic regular expression), unless test_inventory_secret rejects
# FILE.
json_path_keys() {
  [ -f "$1" ] && ! test_inventory_secret "$1" &&
    grep -o "\"/[^\"]*\": *$2" "$1" 2>/dev/null | sed "s/^\"//; s/\": *$2\$//"
}
# abs_strings FILE... -> every quoted absolute path or file:// URI in the files.
# A Windows file URI keeps only its drive-relative path, which image mode
# prefixes with the root, as the PowerShell collector does.
abs_strings() {
  eval "$INVENTORY_DROP_SECRETS"
  [ $# -gt 0 ] || return 0
  grep -ho '"\(file:\/\/\)\{0,1\}/[^"]*"' "$@" 2>/dev/null |
    sed 's/^"//; s/"$//; s/^\(file:\/\/\/[A-Za-z]\)%3[Aa]\//\1:\//;'\
' s/^file:\/\/\/[A-Za-z]:\//\//; s/^file:\/\///; s/%20/ /g'
}
# yaml_vals KEY FILE... -> values of top-level "KEY: value" lines.
# Temporaries: _yv_.
yaml_vals() {
  _yv_key=$1; shift
  eval "$INVENTORY_DROP_SECRETS"
  [ $# -gt 0 ] || return 0
  grep -h "^$_yv_key: *" "$@" 2>/dev/null | sed "s/^$_yv_key: *//; s/^[\"']//; s/[\"']\$//"
}
# json5_vals KEY FILE... -> string values of KEY in JSON5 files, where the key may
# be unquoted and the value single-quoted. Temporaries: _j5v_.
json5_vals() {
  _j5v_key=$1; shift
  eval "$INVENTORY_DROP_SECRETS"
  [ $# -gt 0 ] || return 0
  grep -Eho "(^|[^A-Za-z0-9_\$])[\"']?${_j5v_key}[\"']? *: *(\"[^\"]*\"|'[^']*')" "$@" \
    2>/dev/null | sed "s/^.*${_j5v_key}[\"']\{0,1\} *: *[\"']//; s/[\"']\$//"
}
# toml_file_urls KEY FILE... -> local paths of KEY = "file://..." lines
# (Tabby). Temporaries: _tfu_.
toml_file_urls() {
  _tfu_key=$1; shift
  eval "$INVENTORY_DROP_SECRETS"
  [ $# -gt 0 ] || return 0
  grep -ho "^ *$_tfu_key *= *[\"']file://[^\"']*[\"']" "$@" 2>/dev/null |
    sed "s/^ *$_tfu_key *= *[\"']file:\/\///; s/[\"']\$//; s/%20/ /g"
}
# common_dir -> the deepest directory shared by the absolute file paths on
# stdin (Twinny records indexed files, not the workspace root)
common_dir() {
  awk '{ depth=split($0, parts, "/") - 1
    if (NR==1) { common_depth=depth; for (i=1;i<=common_depth;i++) common[i]=parts[i]; next }
    if (depth<common_depth) common_depth=depth
    for (i=1;i<=common_depth;i++) if (parts[i]!=common[i]) { common_depth=i-1; break } }
    END { if (NR>0 && common_depth>=2) { dir=common[1]
      for (i=2;i<=common_depth;i++) dir=dir "/" common[i]; print dir } }'
}
# session_cwds DIR NAME -> the "cwd" values in the files called NAME under DIR
# (Codex and Open Interpreter rollouts), when DIR is a directory
session_cwds() {
  [ -d "$1" ] && find "$1" -name "$2" -exec grep -ho '"cwd":"[^"]*"' {} + 2>/dev/null |
    sed 's/^"cwd":"//; s/"$//'
}

# find_claude_code_projects HOME: Claude Code projects, also in CLAUDE_CONFIG_DIR
# homes. Temporaries: _fccp_.
find_claude_code_projects() {
  json_vals project "$1/.claude/history.jsonl"
  json_path_keys "$1/.claude.json" '{'
  json_vals project "$1"/.claude-*/history.jsonl
  for _fccp_config in "$1"/.claude-*/.claude.json; do
    json_path_keys "$_fccp_config" '{'
  done
}
# find_cli_session_projects HOME: projects of the CLI agents' session files
# (group 2 of the internals.md discovery table). Temporaries: _fcsp_.
find_cli_session_projects() {
  # The ChatGPT desktop app's projectless directories (Documents/Codex, collected whole) and
  # Codex thread worktrees (excluded) are not projects.
  for _fcsp_dir in "$1/.codex/sessions" "$1/.codex/archived_sessions"; do
    session_cwds "$_fcsp_dir" '*.jsonl'
  done | while IFS= read -r _fcsp_cwd; do
    case $ROOT$_fcsp_cwd in
      ("$1/Documents/Codex" | "$1/Documents/Codex/"* | "$1/.codex/worktrees/"*) ;;
      (*) printf '%s\n' "$_fcsp_cwd" ;;
    esac
  done
  json_vals cwd "$1"/.qwen/projects/*/chats/*.jsonl "$1"/.cline/data/sessions/*/*.json \
    "$1"/.cursor/chats/*/*/meta.json
  json_vals workspace_root "$1"/.cline/data/sessions/*/*.json
  json_vals workspaceDirectory "$1/.continue/sessions/sessions.json"
  json_vals working_dir "$1"/.local/share/goose/sessions/*.jsonl
  json_vals worktree "$1"/.local/share/opencode/storage/project/*.json
  json_vals path "$1/.local/share/crush/projects.json" "$1/AppData/Local/crush/projects.json"
  set -- "$1"/agent-zero/usr/chats/*/chat.json "$1"/agent-zero/*/usr/chats/*/chat.json \
    "$1"/Desktop/agent-zero/usr/chats/*/chat.json
  json_vals acp_cwd "$@"
  grep -ho '"acp_additional_directories": *\[[^]]*\]' "$@" 2>/dev/null |
    grep -o '"/[^"]*"' | sed 's/^"//; s/"$//'
}
# find_gemini_projects HOME: Gemini CLI and Antigravity CLI projects.
find_gemini_projects() {
  json_path_keys "$1/.gemini/projects.json" '"'
  json_path_keys "$1/.gemini/trustedFolders.json" '"'
  cat "$1"/.gemini/tmp/*/.project_root "$1"/.gemini/history/*/.project_root 2>/dev/null
  json_vals workspace "$1/.gemini/antigravity-cli/history.jsonl"
  abs_strings "$1/.gemini/antigravity-cli/settings.json" \
    "$1/.gemini/antigravity-cli/cache/projects.json" "$1"/.gemini/config/projects/*.json
}
# find_agent_session_projects HOME: projects of the other agents' session and
# state files (group 4 of the internals.md discovery table). Temporaries: _fasp_.
find_agent_session_projects() {
  json_vals cwd "$1/.local/share/amp/history.jsonl" "$1"/.factory/sessions/*/*.jsonl \
    "$1"/.factory/sessions/*.jsonl "$1"/.kiro/sessions/cli/*.json \
    "$1"/.cursor/acp-sessions/*/meta.json
  json_vals root "$1"/.kiro/workspace-roots/*/.trust-migration.json
  abs_strings "$1"/.kiro/sessions/*/sess_*/session.json "$1/.config/Goose/recent-dirs.json" \
    "$1/Library/Application Support/Goose/recent-dirs.json" \
    "$1/AppData/Roaming/Goose/recent-dirs.json"
  # A cwd under .copilot (Copilot Desktop chats and worktrees) is not a project, nor is that
  # of a session without events.jsonl, which Copilot Desktop pre-created and never prompted.
  for _fasp_events in "$1"/.copilot/session-state/*/events.jsonl; do
    [ -f "$_fasp_events" ] && yaml_vals cwd "${_fasp_events%/events.jsonl}/workspace.yaml"
  done | while IFS= read -r _fasp_cwd; do
    case $ROOT$_fasp_cwd in ("$1/.copilot/"*) ;; (*) printf '%s\n' "$_fasp_cwd" ;; esac
  done
  json_vals workdir "$1"/.hermes/checkpoints/store/projects/*.json \
    "$1"/.hermes/profiles/*/checkpoints/store/projects/*.json \
    "$1"/AppData/Local/hermes/checkpoints/store/projects/*.json \
    "$1"/AppData/Local/hermes/profiles/*/checkpoints/store/projects/*.json
  json_vals project "$1/.letta/sessions.jsonl"
  json_vals cwd "$1"/.letta/lc-local-backend/conversations/*/messages.jsonl \
    "$1"/.pi/agent/sessions/*/*.jsonl "$1"/.pi/agent/*.jsonl \
    "$1/.pi/agent/crashes.json" "$1"/.pi/agent/experimental/sessions/*/meta.json
  json_path_keys "$1/.pi/agent/trust.json" '[tf]'
  json_path_keys "$1/.config/muse/trust.json" '{'
  json_vals workspace_root "$1"/.local/share/muse/sessions/*/*/*/*/session.jsonl
  # Open Interpreter thread worktrees (excluded) are not projects.
  for _fasp_dir in "$1/.openinterpreter/sessions" "$1/.openinterpreter/archived_sessions"; do
    session_cwds "$_fasp_dir" 'rollout-*.jsonl'
  done | while IFS= read -r _fasp_cwd; do
    case $ROOT$_fasp_cwd in
      ("$1/.openinterpreter/worktrees/"*) ;;
      (*) printf '%s\n' "$_fasp_cwd" ;;
    esac
  done
  json_vals path "$1/.openhands/workspaces.json"
  json_vals working_dir "$1"/.openhands/agent-canvas/dev_conversations/*/meta.json \
    "$1"/.openhands/agent-canvas/conversations/*/meta.json \
    "$1"/.openhands/conversations/*/base_state.json
  json_vals workspaceDirectory "$1/.pearai/sessions/sessions.json"
}
# find_config_file_projects HOME: OpenClaw (and its Clawdbot and Moltbot configs),
# nanobot, Tabby and Twinny. Temporaries: _fcfp_.
find_config_file_projects() {
  for _fcfp_key in workspace cwd agentDir; do
    json5_vals "$_fcfp_key" "$1"/.openclaw*/openclaw.json "$1/.clawdbot/openclaw.json" \
      "$1/.clawdbot/clawdbot.json" "$1/.clawdbot/moltbot.json" "$1/.moltbot/moltbot.json"
  done
  for _fcfp_file in "$1"/.nanobot*/sessions/*/.workspace; do
    [ -f "$_fcfp_file" ] && awk 1 "$_fcfp_file"
  done
  json_vals workspace "$1"/.nanobot*/config.json
  toml_file_urls git_url "$1/.tabby/config.toml"
  for _fcfp_file in "$1"/.twinny/embeddings/*/manifest.json; do
    [ -f "$_fcfp_file" ] && abs_strings "$_fcfp_file" | common_dir
  done
}
# find_editor_projects HOME: VS Code-family workspaces and the Cline, Roo Code and
# Kilo Code task workspaces. Temporaries: _fep_.
find_editor_projects() {
  for _fep_user_dir in "$1"/Library/Application\ Support/*/User "$1"/.config/*/User \
    "$1"/AppData/Roaming/*/User "$1"/.*-server*/data/User; do
    [ -d "$_fep_user_dir" ] || continue
    [ -d "$_fep_user_dir/workspaceStorage" ] &&
      find "$_fep_user_dir/workspaceStorage" -name workspace.json \
        -exec grep -ho '"folder": *"file://[^"]*"' {} + 2>/dev/null |
      sed 's/^"folder": *"file:\/\///; s/"$//; s/%20/ /g'
    _fep_global="$_fep_user_dir/globalStorage"
    json_vals cwdOnTaskInitialization "$_fep_global"/saoudrizwan.claude-dev/state/taskHistory.json \
      "$_fep_global"/saoudrizwan.claude-dev/tasks/*/task_metadata.json
    json_vals workspace "$_fep_global"/rooveterinaryinc.roo-cline/tasks/_index.json \
      "$_fep_global"/kilocode.kilo-code/tasks/_index.json
  done
}

# get_project_owner DIR -> the user whose home in USER_LIST contains DIR, or
# nothing
get_project_owner() {
  printf '%s\n' "$USER_LIST" | awk -F: -v project="$1" '{ home=substr($0,index($0,":")+1)
    if (index(project, home"/")==1) { print $1; exit } }'
}

# discover_projects -> user:path for every project path in the state of each home
# in USER_LIST, then owner:path for every -p directory, with ROOT prefixed. Sets
# DISCOVERY_HOME in its own subshell. Temporaries: _dp_.
discover_projects() {
  _dp_saved_ifs=$IFS
  IFS='
'
  printf '%s\n' "$USER_LIST" | while IFS=: read -r _dp_user DISCOVERY_HOME; do
    [ -n "$DISCOVERY_HOME" ] || continue
    {
      find_claude_code_projects "$DISCOVERY_HOME"
      find_cli_session_projects "$DISCOVERY_HOME"
      find_gemini_projects "$DISCOVERY_HOME"
      find_agent_session_projects "$DISCOVERY_HOME"
      find_config_file_projects "$DISCOVERY_HOME"
      find_editor_projects "$DISCOVERY_HOME"
    } | grep '^/' | sed 's/\\\//\//g' |
      awk -v root="$ROOT" -v user="$_dp_user" '{print user":"root$0}'
  done
  IFS=$_dp_saved_ifs
  printf '%s\n' "$EXTRA_PROJECTS" | grep '^/' | while IFS= read -r _dp_project; do
    printf '%s:%s\n' "$(get_project_owner "$_dp_project")" "$_dp_project"
  done
}

# merge_project_owners: user:path lines on stdin -> one line per path, in
# order of first appearance, owned by the first non-empty user given for it
merge_project_owners() {
  awk '
    { sep=index($0,":"); user=substr($0,1,sep-1); project=substr($0,sep+1)
      if (project=="") next
      if (!(project in owner)) { order[++count]=project; owner[project]=user }
      else if (owner[project]=="") owner[project]=user }
    END { for (k=1;k<=count;k++) print owner[order[k]]":"order[k] }'
}

# keep_project_dirs: user:path lines on stdin -> the lines whose path is a
# directory and is not itself a home in USER_LIST. Temporaries: _kpd_.
keep_project_dirs() {
  while IFS= read -r _kpd_line; do
    _kpd_project=${_kpd_line#*:}
    [ -d "$_kpd_project" ] || continue
    case "
$USER_LIST" in (*":$_kpd_project
"*|*":$_kpd_project") continue ;; esac
    printf '%s\n' "$_kpd_line"
  done
}

# report_missing_projects: logs every -p value that is not a directory and,
# outside --inventory, writes its error_read row. Reads EXTRA_PROJECTS, INVENTORY,
# MANIFEST. Temporaries: _rmp_.
report_missing_projects() {
  printf '%s\n' "$EXTRA_PROJECTS" | while IFS= read -r _rmp_project; do
    if [ -z "$_rmp_project" ] || [ -d "$_rmp_project" ]; then continue; fi
    _rmp_error=$(get_dir_read_error "$_rmp_project")
    log_line "WARNING: project given with -p is not a directory: $_rmp_project: $_rmp_error"
    [ "$INVENTORY" = 1 ] || write_manifest_row "$MANIFEST" \
      "$(get_project_owner "$_rmp_project")" "$_rmp_project" "" "$_rmp_project" "" dir \
      0 0 0 0 0 0 0 "" "" false error_read "" "$_rmp_error"
  done
}

# collect_projects: sets PROJECT_LIST and, outside --inventory, collects
# PROJECT_CATALOG in every project. Call it in the main shell. Temporaries: _cpr_.
collect_projects() {
  PROJECT_LIST=$(discover_projects | merge_project_owners | keep_project_dirs)
  report_missing_projects
  [ "$INVENTORY" = 1 ] || printf '%s\n' "$PROJECT_LIST" | while IFS= read -r _cpr_line; do
    [ -n "$_cpr_line" ] || continue
    _cpr_owner=${_cpr_line%%:*}; _cpr_project=${_cpr_line#*:}
    log_line "Project ${_cpr_owner:+[$_cpr_owner] }$_cpr_project"
    if _cpr_error=$(get_dir_read_error "$_cpr_project"); then
      write_read_error_row "$MANIFEST" "$_cpr_owner" "$_cpr_project" "" "$_cpr_project" \
        "$_cpr_error"
    fi
    collect_catalog "$_cpr_owner" "$_cpr_project" "$PROJECT_CATALOG"
  done
}

# ===========================================================================
# Inventory output
# ===========================================================================
# The host line comes first but its counts are known only after the walk, so
# the agent lines are held until then.

# aggregate_inventory: M| and F| lines on stdin -> agent|files|bytes|first|last|
# evidence per agent with at least one file, in order of first match. Dates come
# from plain arithmetic (Howard Hinnant's civil-from-days algorithm), since POSIX
# has no date -d.
aggregate_inventory() {
  awk -F'|' '
    function iso(epoch,   days, secs, shifted, era, day_of_era, year_of_era, year,
                 day_of_year, month_index, day, month) {
      if (epoch <= 0) return ""
      days = int(epoch / 86400); secs = epoch - days * 86400
      shifted = days + 719468; era = int(shifted / 146097)
      day_of_era = shifted - era * 146097
      year_of_era = day_of_era - int(day_of_era / 1460) + int(day_of_era / 36524)
      year_of_era = int((year_of_era - int(day_of_era / 146096)) / 365)
      year = year_of_era + era * 400
      day_of_year = 365 * year_of_era + int(year_of_era / 4) - int(year_of_era / 100)
      day_of_year = day_of_era - day_of_year
      month_index = int((5 * day_of_year + 2) / 153)
      day = day_of_year - int((153 * month_index + 2) / 5) + 1
      month = (month_index < 10) ? month_index + 3 : month_index - 9; if (month <= 2) year++
      return sprintf("%04d-%02d-%02dT%02d:%02d:%02dZ", year, month, day,
        int(secs / 3600), int((secs % 3600) / 60), secs % 60)
    }
    $1 == "M" { agent = $2
      if (!(agent in seen)) {
        seen[agent] = 1; order[++count] = agent; files[agent] = 0; bytes[agent] = 0
        evidence[agent] = "" }
      if (!((agent, $3) in globs_seen)) { globs_seen[agent, $3] = 1
        evidence[agent] = evidence[agent] (evidence[agent] == "" ? "" : ",") $3 } }
    $1 == "F" { agent = $2; files[agent]++; bytes[agent] += $3
      if ($4 > 0) {
        if (!(agent in first) || $4 < first[agent]) first[agent] = $4
        if (!(agent in last) || $4 > last[agent]) last[agent] = $4 } }
    END { for (k = 1; k <= count; k++) { agent = order[k]; if (files[agent] == 0) continue
      printf "%s|%d|%.0f|%s|%s|%s\n", agent, files[agent], bytes[agent],
        iso(first[agent] + 0), iso(last[agent] + 0), evidence[agent] } }'
}

# write_inventory_lines USER PROJECTS: aggregate_inventory lines on stdin ->
# one agent JSON line each for USER, with PROJECTS as its project count.
# Reads HOST. Temporaries: _wil_.
write_inventory_lines() {
  while IFS='|' read -r _wil_agent _wil_files _wil_bytes _wil_first _wil_last _wil_evidence; do
    [ -n "$_wil_agent" ] || continue
    printf '{"type":"agent","host":"%s","user":"%s","agent":"%s","files":%s,"bytes":%s,'\
'"first":"%s","last":"%s","projects":%s,"evidence":"%s"}\n' \
      "$(json_escape "$HOST")" "$(json_escape "$1")" "$(json_escape "$_wil_agent")" \
      "$(to_number "$_wil_files")" "$(to_number "$_wil_bytes")" \
      "$_wil_first" "$_wil_last" "$(to_number "$2")" "$(json_escape "$_wil_evidence")"
  done
}

# count_inventory_homes: sets USERS_SCANNED and USERS_UNREADABLE from USER_LIST,
# warning for each unreadable home. Call it in the main shell. Temporaries: _cih_.
count_inventory_homes() {
  USERS_SCANNED=0; USERS_UNREADABLE=0
  while IFS=: read -r _cih_user _cih_home; do
    [ -n "$_cih_home" ] || continue
    USERS_SCANNED=$(( USERS_SCANNED + 1 ))
    if [ ! -r "$_cih_home" ] || [ ! -x "$_cih_home" ]; then
      USERS_UNREADABLE=$(( USERS_UNREADABLE + 1 ))
      log_line "WARNING: home of $_cih_user is not readable: $_cih_home"
    fi
  done <<EOF_USER_LIST
$USER_LIST
EOF_USER_LIST
}

# write_inventory: the --inventory step; walks every home and prints the host line
# and the agent lines. Reads USER_LIST, PROJECT_LIST, CATALOG, DOCKER_INV,
# DOCKER_FOUND, HOST, VERSION, MODE, START_TIME. Temporaries: _wi_.
write_inventory() {
  count_inventory_homes
  _wi_agent_lines=$(printf '%s\n' "$USER_LIST" | while IFS=: read -r _wi_user _wi_home; do
    [ -n "$_wi_home" ] || continue
    _wi_projects=$(printf '%s\n' "$PROJECT_LIST" | awk -F: -v user="$_wi_user" '
      $1 == user && length($0) > length(user) + 1 { count++ } END { print count + 0 }')
    collect_catalog "$_wi_user" "$_wi_home" "$CATALOG" "User $_wi_user ($_wi_home)" |
      aggregate_inventory | write_inventory_lines "$_wi_user" "$_wi_projects"
  done)
  printf '{"type":"host","host":"%s","collector":"%s","mode":"%s","at":"%s",'\
'"users_scanned":%s,"users_unreadable":%s,"docker_volumes":%s}\n' \
    "$(json_escape "$HOST")" "$VERSION" "$MODE" "$START_TIME" \
    "$USERS_SCANNED" "$USERS_UNREADABLE" "$DOCKER_FOUND"
  [ -z "$_wi_agent_lines" ] || printf '%s\n' "$_wi_agent_lines"
  printf '%s\n' "$DOCKER_INV" | aggregate_inventory | write_inventory_lines docker 0
}

# ===========================================================================
# Summary and archive
# ===========================================================================

# count_status STATUS -> the number of manifest rows with that status.
# Reads MANIFEST.
count_status() { to_number "$(grep -c "\"status\":\"$1\"" "$MANIFEST" 2>/dev/null)"; }

# list_active_users -> user:home for the users whose home gave a manifest row
# other than its own error_read. Reads USER_LIST, MANIFEST. Temporaries: _lau_.
list_active_users() {
  printf '%s\n' "$USER_LIST" | while IFS=: read -r _lau_user _lau_home; do
    [ -n "$_lau_home" ] || continue
    _lau_home_json=$(json_escape "$_lau_home")
    grep -F "\"home\":\"$_lau_home_json\"" "$MANIFEST" 2>/dev/null |
      grep -F -v -q "\"path\":\"$_lau_home_json\",\"archive_path\":\"\",\"type\":\"dir\"" &&
      printf '%s:%s\n' "$_lau_user" "$_lau_home"
  done
}

# sum_collected_bytes -> the total size of the collected rows. Reads MANIFEST.
sum_collected_bytes() {
  awk -F'"size":' '/"status":"collected"/ { split($2, fields, ","); total += fields[1] }
    END { printf "%d", total }' "$MANIFEST"
}

# format_human_size BYTES -> "512 B", "20.9 MiB"
format_human_size() {
  awk -v bytes="$(to_number "$1")" 'BEGIN { split("B KiB MiB GiB TiB", units, " "); i = 1
    while (bytes >= 1024 && i < 5) { bytes /= 1024; i++ }
    if (i == 1) printf "%d B", bytes; else printf "%.1f %s", bytes, units[i] }'
}

# json_list LINES -> the non-empty lines of LINES as a JSON array of strings.
# Temporaries: _jl_.
json_list() {
  _jl_started=0
  printf '['
  printf '%s\n' "$1" | while IFS= read -r _jl_item; do
    [ -n "$_jl_item" ] || continue
    [ "$_jl_started" = 0 ] || printf ','
    printf '"%s"' "$(json_escape "$_jl_item")"
    _jl_started=1
  done
  printf ']'
}

# json_bool FLAG -> true when FLAG is 1, else false
json_bool() { if [ "$1" = 1 ]; then printf true; else printf false; fi; }

# write_summary ARCHIVE_NAME ARCHIVER: writes collection.json (SUMMARY) and stages
# a copy; called again before each archive fallback. Reads the run globals and
# counters. Temporaries: _ws_.
write_summary() {
  _ws_platform="$(uname -s 2>/dev/null) $(uname -r 2>/dev/null) $(uname -m 2>/dev/null)"
  cat >"$SUMMARY" <<EOF_SUMMARY
{
  "tool": "$TOOL",
  "version": "$VERSION",
  "hostname": "$(json_escape "$HOST")",
  "mode": "$MODE",
  "root": "$(json_escape "${ROOT:-/}")",
  "platform": "$(json_escape "$_ws_platform")",
  "run_as_uid": "$(id -u 2>/dev/null)",
  "started": "$START_TIME",
  "finished": "$END_TIME",
  "options": {"full": $(json_bool "$FULL"), "no_secrets": $(json_bool "$NO_SECRETS"), \
"max_file_size_bytes": $MAX_SIZE, "users_filter": "$(json_escape "$USERS")", \
"no_docker": $(json_bool "$NO_DOCKER"), "no_projects": $(json_bool "$NO_PROJECTS"), \
"projects": $(json_list "$EXTRA_PROJECTS")},
  "capabilities": {"hash_tool": "$HASH_TOOL", "stat_mode": "$STAT_MODE", \
"worker_shell": "$(json_escape "$WORKER_SH")", "archiver": "$(json_escape "$2")"},
  "users": $(json_list "$(printf '%s\n' "$USER_LIST" | cut -d: -f1)"),
  "homes": $(json_list "$(printf '%s\n' "$USER_LIST" | sed 's/^[^:]*://')"),
  "users_with_artifacts": $(json_list "$(printf '%s\n' "$ACTIVE_USERS" | cut -d: -f1)"),
  "projects": $(json_list "$(printf '%s\n' "$PROJECT_LIST" | sed 's/^[^:]*://')"),
  "counts": {"collected": $(count_status collected), "symlink": $(count_status symlink), \
"skipped_excluded": $(count_status skipped_excluded), \
"skipped_size": $(count_status skipped_size), \
"skipped_secret": $(count_status skipped_secret), \
"skipped_special": $(count_status skipped_special), "error_copy": $(count_status error_copy), \
"skipped_unmatched_volume": $(count_status skipped_unmatched_volume), \
"error_read": $(count_status error_read), "collected_bytes": $BYTES},
  "docker": {"volumes_found": $DOCKER_FOUND, "volumes_collected": $DOCKER_COLLECTED, \
"unreadable": $DOCKER_UNREADABLE, "docker_desktop": $(json_bool "$DOCKER_DESKTOP")},
  "notes": $(json_list "$NOTES"),
  "archive": "$(json_escape "$1")"
}
EOF_SUMMARY
  cp "$SUMMARY" "$STAGE/collection.json"
}

# tar_gzip: writes ARCHIVE with tar -cf - | gzip, true only when both succeed.
# POSIX sh has no pipefail, so tar's status goes through TAR_STATUS_FILE.
# Temporaries: _tg_.
tar_gzip() {
  [ -z "$TAR_STATUS_FILE" ] || rm -f "$TAR_STATUS_FILE"
  { tar -cf - -C "$STAGE" . 2>>"$LOG"; echo "$?" >"$TAR_STATUS_FILE"; } | gzip >"$ARCHIVE"
  _tg_gzip_status=$?
  _tg_tar_status=$(cat "$TAR_STATUS_FILE" 2>/dev/null)
  [ -z "$TAR_STATUS_FILE" ] || rm -f "$TAR_STATUS_FILE"
  [ "$_tg_gzip_status" = 0 ] && [ "$_tg_tar_status" = 0 ] && return 0
  [ "$_tg_tar_status" = 0 ] || log_line "tar | gzip: tar exited ${_tg_tar_status:-without a status}"
  return 1
}

# write_archive: stages collection.json, the manifest and the log, then archives
# the staging directory with tar -czf, tar | gzip or plain tar. Exits 2 when no
# archive was written.
write_archive() {
  write_summary "$NAME.tar.gz" 'tar -z'
  log_line "Archiving"
  cp "$MANIFEST" "$STAGE/manifest.jsonl"
  cp "$LOG" "$STAGE/collector.log"
  rm -f "$ARCHIVE"
  if ! tar -czf "$ARCHIVE" -C "$STAGE" . 2>>"$LOG"; then
    rm -f "$ARCHIVE"
    if command -v gzip >/dev/null 2>&1 && write_summary "$NAME.tar.gz" 'tar | gzip' &&
      tar_gzip; then :
    else
      rm -f "$ARCHIVE"; ARCHIVE="$OUTDIR/$NAME.tar"
      write_summary "$NAME.tar" 'tar'
      tar -cf "$ARCHIVE" -C "$STAGE" . 2>>"$LOG" || { log_line "FATAL: tar failed"; exit 2; }
    fi
  fi
  [ -s "$ARCHIVE" ] || { log_line "FATAL: archive not written"; exit 2; }
}

# hash_archive: sets ARCHIVE_SHA and ARCHIVE_SIZE, writes ARCHIVE.sha256 and logs
# "done".
hash_archive() {
  ARCHIVE_SHA=$(hash_file "$ARCHIVE")
  ARCHIVE_SIZE=$(stat_file "$ARCHIVE"); ARCHIVE_SIZE=${ARCHIVE_SIZE%% *}
  printf '%s  %s\n' "$ARCHIVE_SHA" "${ARCHIVE##*/}" >"$ARCHIVE.sha256"
  printf '%s done: %s (%s bytes, sha256 %s)\n' "$(utc_timestamp)" "$ARCHIVE" "$ARCHIVE_SIZE" \
    "$ARCHIVE_SHA" >>"$LOG"
  [ "$QUIET" = 1 ] || printf 'done\n' >&2
}

# print_result: the stdout summary, one "name: value" line each, and a docker
# line when any Docker or Podman data was seen. Temporaries: _pr_.
print_result() {
  printf 'archive:    %s\n' "$ARCHIVE"
  printf 'size:       %s (%s)\n' "$ARCHIVE_SIZE" "$(format_human_size "$ARCHIVE_SIZE")"
  printf 'sha256:     %s\n' "$ARCHIVE_SHA"
  printf 'manifest:   %s\n' "$MANIFEST"
  printf 'summary:    %s\n' "$SUMMARY"
  printf 'log:        %s\n' "$LOG"
  printf 'users:      %s\n' "$(printf '%s\n' "$ACTIVE_USERS" | grep -c .)"
  printf 'projects:   %s\n' "$(printf '%s\n' "$PROJECT_LIST" | grep -c .)"
  printf 'collected:  %s files, %s bytes (%s)\n' "$(count_status collected)" "$BYTES" \
    "$(format_human_size "$BYTES")"
  printf 'skipped:    %s excluded, %s too large, %s secret, %s special\n' \
    "$(count_status skipped_excluded)" "$(count_status skipped_size)" \
    "$(count_status skipped_secret)" "$(count_status skipped_special)"
  _pr_copy_errors=$(count_status error_copy); _pr_read_errors=$(count_status error_read)
  printf 'errors:     %s (%s error_copy, %s error_read)\n' \
    "$(( _pr_copy_errors + _pr_read_errors ))" "$_pr_copy_errors" "$_pr_read_errors"
  if [ "$DOCKER_SEEN" = 1 ]; then
    printf 'docker:     %s volumes found, %s collected, %s unreadable' \
      "$DOCKER_FOUND" "$DOCKER_COLLECTED" "$DOCKER_UNREADABLE"
    [ "$DOCKER_UNREADABLE" -gt 0 ] && printf ' (run as root to collect)'
    [ "$DOCKER_DESKTOP" = 1 ] && printf '; Docker Desktop VM disk not collected'
    printf '\n'
  fi
}

# ===========================================================================
# Main program
# ===========================================================================
# The setup and step functions set globals, so each is called in the main
# shell, never in a pipeline or $(...).

# usage: prints the help text to stdout
usage() {
  cat <<EOF_USAGE
Usage: $0 [options]

  -o, --output DIR        Directory for the archive (default: current dir)
  -r, --root DIR          Alternate root, e.g. a mounted disk image (image mode)
  -u, --users LIST        Comma-separated usernames to collect (default: all)
  -p, --project DIR       Extra project directory to collect (repeatable)
      --full              Disable default size exclusions (model blobs, caches,
                          extension binaries)
      --no-secrets        Skip credential files instead of collecting them
      --no-projects       Skip project-level artifact discovery
      --no-docker         Skip Docker and Podman volume enumeration
      --max-file-size MB  Skip files larger than this (default 256, 0 = none)
  -k, --keep-staging      Keep the staging directory after archiving
      --inventory         Write nothing; print one JSON line per user and agent
                          found (file count, bytes, first and last mtime) to
                          stdout. -o is ignored
      --list              Print the artifact catalog and exit
  -q, --quiet             Only print the final summary
  -V, --version           Print version and exit
  -h, --help              This help

Environment: COLLECTOR_SH overrides the shell used for per-file workers
(default: dash when it is on PATH, otherwise sh). Run as root to collect
every user's home.
EOF_USAGE
}

# need_value COUNT: exits 1 with the usage when an option's value is missing
# (COUNT, the arguments left including the option, is below 2).
need_value() { [ "$1" -ge 2 ] || { usage >&2; exit 1; }; }

# parse_options ARGS...: sets the option globals from ARGS. Exits 0 after
# --version or --help, and 1 on an unknown option.
parse_options() {
  OUTDIR=.; OUTDIR_SET=0; ROOT=; USERS=; EXTRA_PROJECTS=
  FULL=0; NO_SECRETS=0; NO_PROJECTS=0; NO_DOCKER=0; MAX_MB=256
  KEEP=0; QUIET=0; LIST=0; INVENTORY=0
  while [ $# -gt 0 ]; do
    case "$1" in
      -o|--output)        need_value $#; OUTDIR=$2; OUTDIR_SET=1; shift ;;
      -r|--root)          need_value $#; ROOT=$2; shift ;;
      -u|--users)         need_value $#; USERS=$2; shift ;;
      -p|--project)       need_value $#; EXTRA_PROJECTS="$EXTRA_PROJECTS
$2"; shift ;;
      --full)             FULL=1 ;;
      --no-secrets)       NO_SECRETS=1 ;;
      --no-projects)      NO_PROJECTS=1 ;;
      --no-docker)        NO_DOCKER=1 ;;
      --max-file-size)    need_value $#; MAX_MB=$2; shift ;;
      -k|--keep-staging)  KEEP=1 ;;
      --list)             LIST=1 ;;
      --inventory)        INVENTORY=1 ;;
      -q|--quiet)         QUIET=1 ;;
      -V|--version)       printf '%s %s\n' "$TOOL" "$VERSION"; exit 0 ;;
      -h|--help)          usage; exit 0 ;;
      *)                  printf 'Unknown option: %s\n' "$1" >&2; usage >&2; exit 1 ;;
    esac
    shift
  done
}

# print_catalog: the --list output, the five tables under a heading each,
# without blank lines (SECRET_GLOBS has none)
print_catalog() {
  printf '# home artifacts (agent|glob relative to home)\n%s\n' "$CATALOG" | grep -v '^$'
  printf '\n# project artifacts (relative to each discovered project)\n%s\n' \
    "$PROJECT_CATALOG" | grep -v '^$'
  printf '\n# excluded unless --full\n%s\n' "$EXCLUDES" | grep -v '^$'
  printf '\n# credential files (flagged secret, skipped with --no-secrets)\n%s\n' "$SECRET_GLOBS"
  printf '\n# docker volumes (agent|volume name glob)\n%s\n' "$DOCKER_VOLUMES" | grep -v '^$'
}

# set_max_size: exits 1 when --max-file-size is not a whole number of MiB,
# else sets MAX_SIZE in bytes (0, no limit)
set_max_size() {
  case "$MAX_MB" in
    ''|*[!0-9]*) printf 'Invalid --max-file-size: %s\n' "$MAX_MB" >&2; exit 1 ;;
  esac
  MAX_SIZE=$(( MAX_MB * 1024 * 1024 ))
}

# resolve_extra_projects: rewrites EXTRA_PROJECTS as absolute paths, each once,
# resolved against the current directory and never prefixed with the root, since
# -p names a path on the machine running the collector. Temporaries: _rep_.
resolve_extra_projects() {
  _rep_resolved=
  while IFS= read -r _rep_project; do
    [ -n "$_rep_project" ] || continue
    case "$_rep_project" in
      /*) ;;
      *) _rep_absolute=$(cd "./$_rep_project" 2>/dev/null && pwd) || _rep_absolute=
         [ -n "$_rep_absolute" ] || _rep_absolute="$(pwd)/${_rep_project#./}"
         _rep_project=$_rep_absolute ;;
    esac
    case "$_rep_resolved
" in (*"
$_rep_project
"*) continue ;; esac
    _rep_resolved="$_rep_resolved
$_rep_project"
  done <<EOF_EXTRA_PROJECTS
$EXTRA_PROJECTS
EOF_EXTRA_PROJECTS
  EXTRA_PROJECTS=$_rep_resolved
}

# resolve_root: sets ROOT ("" or an absolute path; exits 2 when it is not a
# directory) and MODE (live or image).
resolve_root() {
  if [ -n "$ROOT" ]; then
    [ -d "$ROOT" ] || { printf 'Root is not a directory: %s\n' "$ROOT" >&2; exit 2; }
    ROOT=$(cd "$ROOT" && pwd)
    [ "$ROOT" = / ] && ROOT=
  fi
  if [ -n "$ROOT" ]; then MODE=image; else MODE=live; fi
}

# check_prerequisites: exits 2 without find, or, outside --inventory, without
# tar or a writable output directory. Sets OUTDIR to its absolute path.
check_prerequisites() {
  if [ "$INVENTORY" != 1 ]; then
    [ -d "$OUTDIR" ] || mkdir -p "$OUTDIR" 2>/dev/null ||
      { printf 'Cannot create output dir: %s\n' "$OUTDIR" >&2; exit 2; }
    OUTDIR=$(cd "$OUTDIR" && pwd) || exit 2
    [ -w "$OUTDIR" ] || { printf 'Output dir not writable: %s\n' "$OUTDIR" >&2; exit 2; }
    command -v tar >/dev/null 2>&1 || { printf 'tar not found\n' >&2; exit 2; }
  fi
  command -v find >/dev/null 2>&1 || { printf 'find not found\n' >&2; exit 2; }
}

# detect_capabilities: sets HASH_TOOL, STAT_MODE and WORKER_SH once. The workers
# run under dash when it is installed, as it starts and loops faster than bash;
# COLLECTOR_SH overrides it. Temporaries: _dc_.
detect_capabilities() {
  if command -v sha256sum >/dev/null 2>&1; then HASH_TOOL=sha256sum
  elif command -v shasum >/dev/null 2>&1 && shasum -a 256 /dev/null >/dev/null 2>&1; then
    HASH_TOOL=shasum
  elif command -v sha256 >/dev/null 2>&1; then HASH_TOOL=sha256
  elif command -v openssl >/dev/null 2>&1; then HASH_TOOL=openssl
  else HASH_TOOL=none; fi
  _dc_stat_probe=$(stat -c '%s %W' / 2>/dev/null)
  case "$_dc_stat_probe" in
    '') if stat -f '%z' / >/dev/null 2>&1; then STAT_MODE=bsd; else STAT_MODE=none; fi ;;
    *[!0-9\ ]*) STAT_MODE=gnu0 ;;
    *) STAT_MODE=gnu ;;
  esac
  if [ -n "${COLLECTOR_SH:-}" ]; then WORKER_SH=$COLLECTOR_SH
  elif command -v dash >/dev/null 2>&1; then WORKER_SH=dash
  else WORKER_SH='sh'; fi
}

# set_output_names: sets START_TIME, HOST, NAME and the output paths under
# OUTDIR; under --inventory STAGE and TAR_STATUS_FILE are empty and LOG and
# MANIFEST are /dev/null.
set_output_names() {
  START_TIME=$(utc_timestamp)
  STAMP=$(date -u +%Y%m%dT%H%M%SZ)
  HOST=$(hostname 2>/dev/null || uname -n 2>/dev/null || printf unknown)
  HOST_SAFE=$(printf '%s' "$HOST" | tr -c 'A-Za-z0-9._-' '-')
  NAME="${HOST_SAFE}_${STAMP}_agent-artifacts"
  STAGE="$OUTDIR/.stage-$NAME"
  ARCHIVE="$OUTDIR/$NAME.tar.gz"
  LOG="$OUTDIR/$NAME.log"
  MANIFEST="$OUTDIR/$NAME.manifest.jsonl"
  SUMMARY="$OUTDIR/$NAME.collection.json"
  TAR_STATUS_FILE="$OUTDIR/.tar-status-$NAME"
  if [ "$INVENTORY" = 1 ]; then
    STAGE=; LOG=/dev/null; MANIFEST=/dev/null; TAR_STATUS_FILE=
  fi
}

# cleanup: removes the staging directory (unless -k) and tar_gzip's status
# file. Reads KEEP, STAGE, TAR_STATUS_FILE.
# shellcheck disable=SC2317,SC2329 # invoked from the traps in the main program
cleanup() {
  if [ "$KEEP" != 1 ] && [ -n "$STAGE" ] && [ -d "$STAGE" ]; then
    rm -rf "$STAGE"
  fi
  [ -z "$TAR_STATUS_FILE" ] || rm -f "$TAR_STATUS_FILE"
}

# create_output_files: outside --inventory, creates the staging directory
# (exits 2 when it cannot) and empty LOG and MANIFEST files
create_output_files() {
  if [ "$INVENTORY" != 1 ]; then
    mkdir -p "$STAGE/fs" || { printf 'Cannot create staging dir\n' >&2; exit 2; }
    : >"$LOG"; : >"$MANIFEST"
  fi
}

# log_start: writes the first log lines: version, host, mode, capabilities,
# options and warnings. Temporaries: _ls_.
log_start() {
  _ls_system="$(uname -s 2>/dev/null) $(uname -r 2>/dev/null)"
  _ls_mode="mode=$MODE root=${ROOT:-/} uid=$(id -u 2>/dev/null)"
  log_line "$TOOL $VERSION starting on $HOST ($_ls_system) $_ls_mode"
  _ls_options="max_file_size=${MAX_MB}MB full=$FULL no_secrets=$NO_SECRETS no_docker=$NO_DOCKER"
  log_line "hash=$HASH_TOOL stat=$STAT_MODE $_ls_options worker_sh=$WORKER_SH"
  if [ "$MODE" = live ] && [ "$(id -u 2>/dev/null)" != 0 ]; then
    log_line "WARNING: not running as root; other users' homes will probably be unreadable"
  fi
  if [ "$INVENTORY" = 1 ] && [ "$OUTDIR_SET" = 1 ]; then
    log_line "NOTE: --inventory writes nothing; -o $OUTDIR ignored"
  fi
}

# The steps, in run order.
parse_options "$@"
if [ "$LIST" = 1 ]; then
  print_catalog
  exit 0
fi
set_max_size
resolve_extra_projects
resolve_root
check_prerequisites
detect_capabilities
compile_secret_pattern
define_secret_test
set_output_names
# exported for the worker programs
export HASH_TOOL STAT_MODE MAX_SIZE SECRET_PATTERN NO_SECRETS LOG QUIET
trap 'cleanup' EXIT
trap 'log_line "interrupted"; cleanup; trap - EXIT; exit 130' INT TERM
create_output_files
if [ "$FULL" = 1 ]; then ACTIVE_EXCLUDES=; else ACTIVE_EXCLUDES=$EXCLUDES; fi
log_start

# Globals the steps fill. NESTED_CLAIMS is cleared so the volume walks never
# prune paths an inherited environment variable named.
NOTES=
DOCKER_SEEN=0; DOCKER_FOUND=0; DOCKER_COLLECTED=0; DOCKER_UNREADABLE=0; DOCKER_DESKTOP=0
DOCKER_INV=
PROJECT_LIST=
NESTED_CLAIMS=

USER_LIST=$(get_user_homes)
[ -n "$USER_LIST" ] || log_line "WARNING: no user home directories found"
# --inventory walks the homes in write_inventory, after project discovery
[ "$INVENTORY" = 1 ] || collect_user_homes
[ "$NO_DOCKER" = 1 ] || collect_docker_volumes
[ "$NO_PROJECTS" = 1 ] || collect_projects
if [ "$INVENTORY" = 1 ]; then
  write_inventory
  exit 0
fi

END_TIME=$(utc_timestamp)
ACTIVE_USERS=$(list_active_users)
BYTES=$(sum_collected_bytes)
write_archive
hash_archive
print_result
exit 0
