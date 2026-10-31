# Copyright 2026 Sean Whalen
# SPDX-License-Identifier: Apache-2.0
#
# Smoke test for Collect-AgentArtifacts.ps1: build a fake disk image, run the
# collector in image mode, and check the manifest and archive. Runs under
# Windows PowerShell 5.1 and PowerShell 7 on any OS:
#   powershell.exe -ExecutionPolicy Bypass -File tests\smoke.ps1
#   pwsh -File tests/smoke.ps1
[CmdletBinding()]
param([string]$Collector = '')
Set-StrictMode -Version 2
$ErrorActionPreference = 'Continue'
if ($Collector -eq '') { $Collector = Join-Path (Split-Path -Parent $PSScriptRoot) 'Collect-AgentArtifacts.ps1' }
$Separator = [string][System.IO.Path]::DirectorySeparatorChar
$Work = Join-Path ([System.IO.Path]::GetTempPath()) ("cac-smoke-" + [guid]::NewGuid().ToString('N').Substring(0, 8))
$Root = Join-Path $Work 'root'
$Out = Join-Path $Work 'out'
$script:fail = 0
function Pass([string]$message) { Write-Output "ok   $message" }
function FailCheck([string]$message) { Write-Output "FAIL $message"; $script:fail = 1 }
function Check([string]$name, [scriptblock]$condition) {
  $result = $false; $why = ''
  try { $result = [bool](& $condition) } catch { $result = $false; $why = " ($($_.Exception.Message))" }
  if ($result) { Pass $name } else { FailCheck ($name + $why) }
}
function ImagePath([string]$relativePath) { return (Join-Path $Root ($relativePath.Replace('/', $Separator))) }
# The extended-length form of an absolute Windows path (\\?\C:\... or
# \\?\UNC\server\share\...), which .NET and -LiteralPath open past 260
# characters without LongPathsEnabled; unchanged elsewhere. ImagePath stays
# plain because checks compare it with manifest paths; the fixture helpers
# write through ExtendedPath.
function ExtendedPath([string]$path) {
  if ($env:OS -ne 'Windows_NT' -or $path.StartsWith('\\?\')) { return $path }
  if ($path.StartsWith('\\')) { return '\\?\UNC\' + $path.Substring(2) }
  if ($path.Length -ge 3 -and $path[1] -eq ':') { return '\\?\' + $path }
  return $path
}
# Every file MakeFile and MakeSizedFile wrote, in extended form, until Test-Made
# checks and clears the list, so a fixture that could not be created is a FAIL.
$script:Made = New-Object System.Collections.ArrayList
function Test-Made {
  $missing = @($script:Made | Where-Object { -not [System.IO.File]::Exists($_) })
  $script:Made.Clear()
  return , $missing
}
function MakeFile([string]$relativePath, [string]$content) {
  $path = ExtendedPath (ImagePath $relativePath)
  [void][System.IO.Directory]::CreateDirectory([System.IO.Path]::GetDirectoryName($path))
  [System.IO.File]::WriteAllText($path, $content, (New-Object System.Text.UTF8Encoding $false))
  [void]$script:Made.Add($path)
}
function MakeSizedFile([string]$relativePath, [int]$bytes) {
  $path = ExtendedPath (ImagePath $relativePath)
  [void][System.IO.Directory]::CreateDirectory([System.IO.Path]::GetDirectoryName($path))
  $stream = [System.IO.File]::Create($path); $stream.SetLength($bytes); $stream.Close()
  [void]$script:Made.Add($path)
}

Write-Output "== $($PSVersionTable.PSEdition) $($PSVersionTable.PSVersion) on $([Environment]::OSVersion.Platform); work $Work"

# ---- fake image (a Windows profile tree plus a Linux-style home) -----------
MakeFile 'Users/alice/.claude/projects/-C-proj/s1.jsonl' '{"type":"user","message":{"role":"user","content":"hi"},"timestamp":"2026-10-03T00:00:00Z"}'
MakeFile 'Users/alice/.claude/history.jsonl' '{"display":"do things","timestamp":1759449600000,"project":"C:\\proj","sessionId":"s1"}'
MakeFile 'Users/alice/.claude/.credentials.json' '{"claudeAiOauth":{"accessToken":"sk-ant-oat-SECRET"}}'
MakeFile 'Users/alice/.claude.json' '{"projects":{"C:\\proj":{}}}'
MakeFile 'Users/alice/.claude/cache/junk.bin' 'cached junk'
MakeFile 'Users/alice/.claude/weird name with spaces & [brackets].txt' 'weird'
MakeFile 'proj/CLAUDE.md' '# proj'
MakeFile 'proj/.claude/settings.local.json' 'settings'
MakeFile 'Users/alice/.gemini/antigravity-cli/antigravity-oauth-token' 'token'
MakeFile 'Users/alice/.gemini/antigravity-cli/conversations/c1.db' 'db'
MakeSizedFile 'Users/alice/.gemini/antigravity-cli/bin/webm_encoder.exe' 300000
MakeFile 'Users/alice/.gemini/antigravity-cli/log/cli-1.log' 'log'
MakeFile 'Users/alice/.gemini/projects.json' '{"projects":{"C:\\gem":"gem"}}'
MakeFile 'gem/GEMINI.md' '# gem'
MakeFile 'Users/alice/.codex/sessions/2026/10/03/rollout-1.jsonl' '{"type":"session_meta","payload":{"cwd":"C:\\Users\\alice\\Documents\\app"}}'
MakeFile 'Users/alice/.codex/auth.json' '{"tokens":{"access_token":"x"}}'
MakeSizedFile 'Users/alice/.codex/packages/standalone/releases/1/codex.exe' 200000
MakeFile 'Users/alice/Documents/app/.cursorrules' 'rules'
MakeFile 'Users/alice/Documents/app/AGENTS.md' '# agents'
MakeSizedFile 'Users/alice/AppData/Roaming/Cursor/User/globalStorage/state.vscdb' 2100000
MakeFile 'Users/alice/AppData/Roaming/Cursor/User/globalStorage/ms-python.python/x' 'junk'
MakeFile 'Users/alice/AppData/Roaming/Cursor/User/workspaceStorage/abc/workspace.json' '{"folder":"file:///c%3A/Users/alice/Documents/app"}'
MakeFile 'Users/alice/AppData/Roaming/Code/User/globalStorage/saoudrizwan.claude-dev/state/taskHistory.json' '[{"id":"t1","cwdOnTaskInitialization":"C:\\cl"}]'
MakeFile 'Users/alice/AppData/Roaming/Code/User/globalStorage/saoudrizwan.claude-dev/checkpoints/x' 'ckpt'
MakeFile 'cl/.clinerules' 'rules'
MakeFile 'Users/alice/AppData/Roaming/Microsoft/Windows/PowerShell/PSReadLine/ConsoleHost_history.txt' 'Get-Process'
MakeFile 'Users/alice/AppData/Roaming/Claude/config.json' '{"oauth:tokenCache":"enc"}'
MakeFile 'Users/alice/AppData/Roaming/Claude/claude_desktop_config.json' '{"mcpServers":{}}'
MakeFile 'Users/alice/AppData/Roaming/Claude/Cache/index' 'cache'
MakeFile 'Users/alice/.continue/sessions/sessions.json' '[{"sessionId":"a","workspaceDirectory":"C:\\cont"}]'
MakeFile 'Users/alice/.continue/index/globalContext.json' '{"mcpOauthStorage":{}}'
MakeFile 'Users/alice/.continue/index/lancedb/data.lance' 'vec'
MakeFile 'cont/.continuerc.json' '{}'
MakeFile 'Users/alice/.local/share/crush/projects.json' '{"projects":[{"path":"C:\\crushproj","data_dir":"C:\\crushproj\\.crush"}]}'
MakeFile 'crushproj/.crush/crush.db' 'sqlite'
MakeFile 'Users/alice/.cline/data/settings/providers.json' '{"anthropic":{"apiKey":"sk-x"}}'
MakeFile 'Users/alice/.env' 'TOKEN=1'
MakeFile 'Users/Public/Desktop/readme.txt' 'public'
MakeFile 'Users/Default/NTUSER.DAT' 'default'
MakeFile 'home/bob/.ollama/models/blobs/sha256-abc' 'blob'
MakeFile 'home/bob/.ollama/history' 'hist'
# 2026-10-04 a profile with no agent state: listed in users, not counted on stdout
New-Item -ItemType Directory -Path (ImagePath 'Users/dave') -Force | Out-Null
MakeFile 'home/bob/.bash_history' 'export FOO=1'
# v1.3 catalog revision fixtures
MakeFile 'Users/alice/.factory/cache/session-index/index.db' 'sqlite'
MakeFile 'Users/alice/.factory/cache/sounds/done.wav' 'ding'
MakeFile 'Users/alice/.factory/sessions/-C-droid/s1.jsonl' '{"type":"session_start","cwd":"C:\\droid"}'
MakeFile 'droid/AGENTS.md' '# droid'
MakeFile 'Users/alice/.kiro/powers/installed/p1/mcp.json' '{"mcpServers":{}}'
MakeFile 'Users/alice/.kiro/sessions/cli/k1.json' '{"id":"k1","cwd":"C:\\kiroproj"}'
MakeFile 'kiroproj/.kiroignore' 'steer'
MakeFile 'Users/alice/.vscode-server/data/User/globalStorage/state.vscdb' 'sqlite'
MakeFile 'Users/alice/.vscode-server/data/User/globalStorage/saoudrizwan.claude-dev/state/taskHistory.json' '[{"id":"t2","cwdOnTaskInitialization":"C:\\remoteproj"}]'
MakeFile 'remoteproj/.clinerules' 'rules'
MakeFile 'Users/alice/.gemini/antigravity-ide/conversations/i1.db' 'db'
MakeFile 'Users/alice/.gemini/antigravity-cli/history.jsonl' '{"display":"hi","timestamp":1759449600000,"workspace":"C:\\agyproj"}'
MakeFile 'agyproj/.antigravityignore' 'ignore'
MakeFile 'Users/alice/.cursor/auth.json' '{"accessToken":"x"}'
MakeFile 'Users/alice/.copilot/pkg/copilot.exe' 'pkg'
MakeFile 'Users/alice/.copilot/config.json' '{"token":"x"}'
MakeFile 'Users/alice/.copilot/session-state/s1/workspace.yaml' "cwd: C:\\copproj`nbranch: main"
MakeFile 'copproj/AGENTS.md' '# cop'
MakeFile 'Users/alice/.claude/.device-keys.json' '{"k":"x"}'
MakeFile 'Users/alice/.local/share/amp/history.jsonl' '{"cwd":"C:\\ampproj","prompt":"hi"}'
MakeFile 'ampproj/.amp/settings.json' 'amp'
MakeFile 'kiloproj/.kilo/worktrees/w1/f' 'wt'
MakeFile 'Users/alice/.cline/data/sessions/s1/s2.json' '{"cwd":"C:\\kiloproj"}'
# 2026-10-04 catalog cases: the same list as smoke.sh, kind|agent|path relative to
# alice's profile.
$CasesCatalog = @'
excl||.pearai/extensions
excl||.pearai/index/lancedb
excl||.pearai/types
excl||.pearai/out
excl||.pearai/node_modules
excl||.pearai/.diffs
excl||.pearai/.migrations
excl||.config/pearai/User/globalStorage/pearai.pearai-roo-cline/checkpoints
excl||.config/pearai/User/globalStorage/pearai.pearai-roo-cline/tasks/t1/checkpoints
excl||.config/pearai/User/globalStorage/pearai.pearai-roo-cline/cache
exclf||.pearai/index/codebase.sqlite
exclf||.pearai/dev_data/devdata.sqlite
secret||.pearai/config.json
secret||.pearai/config.ts
secret||.pearai/.env
secret||.pearai/.configs/c1/config.json
secret||.config/pearai/User/globalStorage/pearai.pearai-roo-cline/settings/pearai_agent_mcp_settings.json
agent|pearai|.config/pearai/User/globalStorage/state.vscdb
agent|pearai|.pearai/sessions/s1.json
exclf||.config/Code/User/globalStorage/sourcegraph.cody-ai/symf/symf-v0.0.16
exclf||.local/share/Cody-nodejs/symf/symf-v0.0.16
exclf||Library/Application Support/Cody-nodejs/Data/symf/symf-v0.0.16
excl||.local/share/Cody-nodejs/dist
excl||AppData/Roaming/Cody-nodejs/Config/dist
secret||.config/Cody-nodejs/user-settings.json
secret||AppData/Roaming/Cody-nodejs/Config/user-settings.json
secret||.config/gh/hosts.yml
secret||AppData/Roaming/GitHub CLI/hosts.yml
agent|cody|.config/Code/User/globalStorage/sourcegraph.cody-ai/symf/indexroot/meta.json
agent|cody|.vscode/cody.json
agent|cody|.local/share/Cody-nodejs/JetBrains-globalState/cody-local-chatHistory-v2
excl||.twinny/embeddings/ws/chunks.lance
excl||.twinny/server/plugins/p1/repos/r1/checkout
excl||.twinny/server/plugins/p1/repos/r1/index
secret||.config/Code/User/globalStorage/rjmacarthy.twinny/twinny-providers.json
secret||.twinny/node/identity.json
secret||.twinny/server/license
secret||.twinny/server/plugins/p1/settings.json
agent|twinny|.config/Code/User/globalStorage/rjmacarthy.twinny/twinny-providers.json
agent|twinny|.vscode-server/data/User/globalStorage/rjmacarthy.twinny/state.json
excl||.tabby/models
excl||.tabby/index
excl||.tabby/repositories
secret||.tabby/config.toml
secret||.tabby-client/agent/config.toml
agent|tabby|.tabby/ee/db.sqlite
agent|tabby|.tabby/ee/db.sqlite-wal
agent|tabby|.tabby/ee/db.sqlite-shm
agent|tabby|.config/JetBrains/IntelliJIdea2025.2/options/intellij-tabby.xml
excl||.openinterpreter/packages
excl||.openinterpreter/cache
excl||.openinterpreter/.tmp
excl||.openinterpreter/tmp
excl||.openinterpreter/plugins/cache
excl||.openinterpreter/skills/.system
excl||.openinterpreter/worktrees
excl||.openinterpreter/.sandbox
excl||.openinterpreter/.sandbox-bin
excl||.openinterpreter/visualizations
excl||.openinterpreter/models-cache
secret||.openinterpreter/auth.json
secret||.openinterpreter/.credentials.json
secret||.openinterpreter/credentials/c1
secret||.openinterpreter/device_id
secret||.openinterpreter/secrets/s1
secret||.openinterpreter/.sandbox-secrets/s1
secret||.openinterpreter/.env
secret||.openinterpreter/config.toml
secret||.codex/.env
agent|open-interpreter|.openinterpreter/history.jsonl
excl||.openhands/cache
excl||.openhands/agent-canvas/workspaces
excl||.openhands/agent-canvas/tmux
secret||.openhands/agent-canvas/secret-key.txt
secret||.openhands/agent-canvas/api-key.txt
secret||.openhands/settings.json
secret||.openhands/secrets.json
secret||.openhands/profiles/p1.json
secret||.openhands/provider-connections/c1.json
secret||.openhands/auth/a1.json
secret||.openhands/runtime-control/r1
secret||.openhands/agent_settings.json
secret||.openhands/cloud/c1.json
secret||.openhands/mcp.json
secret||.openhands/.jwt_secret
secret||.openhands/.keys
agent|openhands|.config/OpenHands Agent Canvas/config.json
agent|openhands|.openhands/conversations/c1/events/event-00000-e1.json
excl||.pi/agent/bin
excl||.pi/agent/tmp
excl||.pi/agent/npm/node_modules
excl||.pi/agent/git/g1/node_modules
excl||.pi/agent/git/g1/.git
excl||.pi/npm/node_modules
excl||.pi/git/g1/node_modules
excl||.pi/git/g1/.git
exclf||.pi/agent/tools/fd
exclf||.pi/agent/tools/rg.exe
exclf||.pi/server/s1.sock
secret||.pi/agent/auth.json
secret||.pi/agent/auth.json.bak
secret||.pi/agent/mcp-auth.json
secret||.pi/agent/oauth.json
secret||.pi/agent/models.json
secret||.pi/agent/mcp.json
secret||.pi/mcp.json
secret||.config/little-coder/models.json
agent|pi|.pi/agent/settings.json
agent|little-coder|.pi/agent/little-coder-prompt-history.json
agent|little-coder|.little-coder/state.json
agent|little-coder|.cache/little-coder/log.txt
excl||.letta/bin
excl||.letta/mod-cache
excl||.letta/tool_execution_dir
excl||.letta/chroma
excl||.memgpt/chroma
secret||.letta/settings.json
secret||.letta/pg_uri
secret||.letta/credentials
secret||.letta/lc-local-backend/providers/auth.json
secret||.letta/channels/telegram/accounts.json
secret||.letta/channels/whatsapp/auth/creds.json
secret||.letta/agents/a1/memory/.git/config
secret||.letta/agents/a1/memory/.git/letta-credential-helper.cmd
secret||.config/letta/settings.json
secret||.memgpt/credentials
agent|letta|.letta/transcripts/a1/c1/transcript.jsonl
excl||.hermes/hermes-agent/hermes_cli
excl||.hermes/hermes-agent/.git
excl||AppData/Local/hermes/hermes-agent/hermes_cli
excl||AppData/Local/hermes/hermes-agent/.venv
excl||.hermes/models
excl||.hermes/runtimes
excl||.hermes/node
excl||.hermes/installs
excl||.hermes/tools
excl||.hermes/sandboxes
excl||.hermes/checkpoints/store/objects
excl||.hermes/checkpoints/store/indexes
excl||.hermes/checkpoints/legacy-1
excl||.hermes/backups
excl||.hermes/browser-profile
excl||.hermes/browser-profiles
excl||.hermes/browser_profiles
excl||.hermes/chrome-debug
excl||.hermes/bot-desktop/browser-profile
excl||.hermes/plugins/p1/node_modules
excl||.hermes/plugins/p1/.venv
excl||AppData/Local/hermes/models
excl||AppData/Local/hermes/runtimes
excl||AppData/Local/hermes/node
excl||AppData/Local/hermes/installs
excl||AppData/Local/hermes/tools
excl||AppData/Local/hermes/sandboxes
excl||AppData/Local/hermes/checkpoints/store/objects
excl||AppData/Local/hermes/checkpoints/store/indexes
excl||AppData/Local/hermes/checkpoints/legacy-1
excl||AppData/Local/hermes/backups
excl||AppData/Local/hermes/browser-profile
excl||AppData/Local/hermes/browser-profiles
excl||AppData/Local/hermes/browser_profiles
excl||AppData/Local/hermes/chrome-debug
excl||AppData/Local/hermes/bot-desktop/browser-profile
excl||AppData/Local/hermes/plugins/p1/node_modules
excl||AppData/Local/hermes/plugins/p1/.venv
excl||.hermes/profiles/work/models
excl||.hermes_dev/models
excl||.config/Hermes/Cache
excl||.config/Hermes/Code Cache
excl||.config/Hermes/GPUCache
excl||Library/Application Support/Hermes/Cache
excl||Library/Application Support/Hermes/Code Cache
excl||Library/Application Support/Hermes/GPUCache
excl||AppData/Roaming/Hermes/Cache
excl||AppData/Roaming/Hermes/Code Cache
excl||AppData/Roaming/Hermes/GPUCache
secret||.hermes/.env
secret||.hermes/.env.bak1
secret||.hermes/.op.env
secret||.hermes/npmrc
secret||.hermes/auth.json
secret||.hermes/auth.json.1
secret||.hermes/auth/google_oauth.json
secret||.hermes/.anthropic_oauth.json
secret||.hermes/.copilot_jwt.json
secret||.hermes/google_token.json
secret||.hermes/google_chat_user_tokens/u1.json
secret||.hermes/slack_tokens.json
secret||.hermes/honcho.json
secret||.hermes/mem0.json
secret||.hermes/webhook_subscriptions.json
secret||.hermes/teams_pipeline_store.json
secret||.hermes/mcp-tokens/t1.json
secret||.hermes/vault/vault.key
secret||.hermes/browser_auth/state.json
secret||.hermes/pairing/p1.json
secret||.hermes/platforms/pairing/p1.json
secret||.hermes/whatsapp/session/creds.json
secret||.hermes/platforms/whatsapp/session/creds.json
secret||.hermes/matrix/store/s1
secret||.hermes/platforms/matrix/store/s1
secret||.hermes/cache/bws_cache1.json
secret||.hermes/weixin/accounts/a1.json
secret||.hermes/runtime/photon-sidecar.json
secret||.hermes/proxy/ca.key
secret||.hermes/home/.gitconfig
secret||AppData/Local/hermes/.env
secret||AppData/Local/hermes/.env.bak1
secret||AppData/Local/hermes/.op.env
secret||AppData/Local/hermes/npmrc
secret||AppData/Local/hermes/auth.json
secret||AppData/Local/hermes/auth.json.1
secret||AppData/Local/hermes/auth/google_oauth.json
secret||AppData/Local/hermes/.anthropic_oauth.json
secret||AppData/Local/hermes/.copilot_jwt.json
secret||AppData/Local/hermes/google_token.json
secret||AppData/Local/hermes/google_chat_user_tokens/u1.json
secret||AppData/Local/hermes/slack_tokens.json
secret||AppData/Local/hermes/honcho.json
secret||AppData/Local/hermes/mem0.json
secret||AppData/Local/hermes/webhook_subscriptions.json
secret||AppData/Local/hermes/teams_pipeline_store.json
secret||AppData/Local/hermes/mcp-tokens/t1.json
secret||AppData/Local/hermes/vault/vault.key
secret||AppData/Local/hermes/browser_auth/state.json
secret||AppData/Local/hermes/pairing/p1.json
secret||AppData/Local/hermes/platforms/pairing/p1.json
secret||AppData/Local/hermes/whatsapp/session/creds.json
secret||AppData/Local/hermes/platforms/whatsapp/session/creds.json
secret||AppData/Local/hermes/matrix/store/s1
secret||AppData/Local/hermes/platforms/matrix/store/s1
secret||AppData/Local/hermes/cache/bws_cache1.json
secret||AppData/Local/hermes/weixin/accounts/a1.json
secret||AppData/Local/hermes/runtime/photon-sidecar.json
secret||AppData/Local/hermes/proxy/ca.key
secret||AppData/Local/hermes/home/.gitconfig
secret||.hermes/profiles/work/.env
secret||.hermes_dev/auth.json
secret||.hermes/hermes-agent/.env
secret||AppData/Local/hermes/hermes-agent/.env
secret||.hermes/config.yaml.bak-20261030t120000z
secret||.hermes/bot-desktop/lease.json
secret||.hermes/webapp/session-token
secret||.hermes/feishu_comment_pairing.json
secret||.hermes/workspace/meetings/node_token.json
secret||.hermes/.ssh/id_ed25519
secret||.hermes/.aws/credentials
secret||.hermes/.gnupg/pubring.kbx
secret||.hermes/.kube/config
secret||.hermes/.docker/config.json
secret||.hermes/.azure/msal_token_cache.json
secret||.hermes/.config/gh/hosts.yml
secret||.hermes/.config/gcloud/credentials.db
secret||.hermes/.envrc
secret||AppData/Local/hermes/config.yaml.bak-20261030t120000z
secret||AppData/Local/hermes/bot-desktop/lease.json
secret||AppData/Local/hermes/webapp/session-token
secret||AppData/Local/hermes/feishu_comment_pairing.json
secret||AppData/Local/hermes/workspace/meetings/node_token.json
secret||AppData/Local/hermes/.ssh/id_ed25519
secret||AppData/Local/hermes/.aws/credentials
secret||AppData/Local/hermes/.gnupg/pubring.kbx
secret||AppData/Local/hermes/.kube/config
secret||AppData/Local/hermes/.docker/config.json
secret||AppData/Local/hermes/.azure/msal_token_cache.json
secret||AppData/Local/hermes/.config/gh/hosts.yml
secret||AppData/Local/hermes/.config/gcloud/credentials.db
secret||AppData/Local/hermes/.envrc
agent|hermes|.hermes/state.db
agent|hermes|.hermes/state.db-wal
agent|hermes|.hermes/state.db-shm
agent|hermes|.hermes/cache/terminal/out1.txt
agent|hermes|.hermes/hermes-agent/.env.example
agent|hermes|.hermes_dev/config.yaml
agent|hermes|AppData/Local/hermes/cache/terminal/out1.txt
agent|hermes|.config/Hermes/connection.json
excl||.openclaw/dev
excl||.openclaw/git
excl||.openclaw/npm
excl||.openclaw/npm-runtime
excl||.openclaw/tmp
excl||.openclaw/tools
excl||.openclaw/worktrees
excl||.openclaw/plugin-skills
excl||.openclaw/extensions/e1/node_modules
excl||.openclaw/sandbox/skills-workspaces
excl||.openclaw/agents/main/agent/tmp
excl||.openclaw/agents/main/agent/.tmp
excl||.openclaw/browser/b1/user-data/Default/Cache
excl||.openclaw/browser/b1/user-data/Default/Code Cache
excl||.openclaw/browser/b1/user-data/Default/GPUCache
excl||.openclaw/browser/b1/user-data/Default/Service Worker
excl||.openclaw-work/tools
excl||.clawdbot/tools
excl||.clawdbot/npm
secret||.openclaw/openclaw.json
secret||.openclaw/clawdbot.json.bak
secret||.openclaw-work/.env
secret||.openclaw/credentials/c1.json
secret||.openclaw/service-env/gateway.env
secret||.openclaw/agents/main/agent/auth-profiles.json
secret||.openclaw/agents/main/agent/models.json
secret||.openclaw/browser/b1/user-data/Default/Cookies
secret||.clawdbot/clawdbot.json
secret||.clawdbot/.env
secret||.clawdbot/credentials/c1
secret||.moltbot/moltbot.json
secret||.moltbot/.env
secret||.moltbot/credentials/c1
secret||.config/openclaw/gateway.env
agent|openclaw|.openclaw/agents/main/agent/openclaw-agent.sqlite
agent|openclaw|.openclaw/agents/main/agent/openclaw-agent.sqlite-wal
agent|openclaw|.openclaw-work/openclaw.json
agent|openclaw|.config/systemd/user/openclaw-gateway.service
agent|openclaw|Library/LaunchAgents/ai.openclaw.gateway.plist
excl||.nanobot/bin
excl||.nanobot/run
excl||.nanobot-work/cache
secret||.nanobot/config.json
secret||.nanobot/auth/a1
secret||.nanobot-work/whatsapp-auth/a1
secret||.nanobot/matrix-store/s1
agent|nanobot|.nanobot-work/sessions/0123456789abcdef0123456789abcdef/Y2xpOmRpcmVjdA.jsonl
agent|nanobot|Library/LaunchAgents/ai.nanobot.gateway.plist
excl||agent-zero/.venv
excl||Desktop/agent-zero/.conda
excl||agent-zero/main/.git
excl||agent-zero/main/tmp/playwright
excl||agent-zero/main/tmp/memory/embeddings
excl||agent-zero/main/usr/.time_travel
excl||agent-zero/main/usr/workdir/w1/.venv
excl||agent-zero/main/usr/workdir/w1/node_modules
excl||.local/share/a0/browser-profiles
excl||Library/Application Support/A0/Browser Profiles
excl||AppData/Local/A0/Browser Profiles
excl||.config/Agent Zero Launcher/Cache
excl||.config/Agent Zero Launcher/Code Cache
excl||.config/Agent Zero Launcher/GPUCache
excl||.config/Agent Zero Launcher/docker_manager/cache
secret||agent-zero/main/usr/.env
secret||agent-zero/main/usr/secrets.env
secret||Desktop/agent-zero/usr/settings.json
secret||agent-zero/main/usr/projects/p1/.a0proj/secrets.env
secret||agent-zero/.env
secret||agent-zero/main/tmp/secrets.env
secret||agent-zero/main/tmp/settings.json
secret||agent-zero/main/usr/plugins/_desktop/profiles/d1/.ssh/id_ed25519
secret||agent-zero/main/usr/plugins/_desktop/profiles/d1/.gnupg/k1
secret||.agent-zero/.env
secret||.agent-zero/session_cookies.json
secret||.config/Agent Zero Launcher/docker_manager/state.json
secret||.config/Agent Zero Launcher/Local State
agent|agent-zero|agent-zero/main/usr/chats/c1/chat.json
agent|agent-zero|Desktop/agent-zero/usr/chats/c1/chat.json
agent|agent-zero|.local/share/a0/config.json
secret||.config/shell_gpt/.sgptrc
agent|shellgpt|AppData/Local/Temp/chat_cache/mychat
agent|shellgpt|.config/shell_gpt/roles/default.json
excl||.local/share/local-deep-research/cache
excl||.local/share/local-deep-research/models
excl||.local/share/local-deep-research/journal_data
excl||.local/share/local-deep-research/library
excl||Library/Application Support/local-deep-research/cache
excl||Library/Application Support/local-deep-research/models
excl||Library/Application Support/local-deep-research/journal_data
excl||Library/Application Support/local-deep-research/library
excl||AppData/Local/local-deep-research/local-deep-research/cache
excl||AppData/Local/local-deep-research/local-deep-research/models
excl||AppData/Local/local-deep-research/local-deep-research/journal_data
excl||AppData/Local/local-deep-research/local-deep-research/library
excl||Documents/LocalDeepResearch/Library
secret||.local/share/local-deep-research/.secret_key
secret||Library/Application Support/local-deep-research/.secret_key
secret||AppData/Local/local-deep-research/local-deep-research/.secret_key
agent|local-deep-research|.local/share/local-deep-research/encrypted_databases/u1.db
agent|local-deep-research|Documents/LocalDeepResearch/report.md
'@
$CasesCatalog = @($CasesCatalog -split "`r?`n" | Where-Object { $_ -ne '' })
foreach ($caseLine in $CasesCatalog) {
  $fields = @($caseLine -split '\|', 3)
  if ($fields[0] -eq 'excl') { MakeFile "Users/alice/$($fields[2])/f" 'x' } else { MakeFile "Users/alice/$($fields[2])" 'x' }
}
MakeFile 'Users/alice/.hermes/checkpoints/store/projects/0123456789abcdef.json' '{"workdir":"C:\\hermesproj","created_at":1759536000}'
MakeFile 'hermesproj/.hermes.md' '# hermes'
# 2026-10-30 hermes rebuild (#326)
$hermesProjectFiles = @('HERMES.md', '.hermes/environment.json', 'trajectory_samples.jsonl', 'failed_trajectories.jsonl')
MakeFile 'hermesproj/HERMES.md' '# hermes'
MakeFile 'hermesproj/.hermes/environment.json' '{}'
MakeFile 'hermesproj/trajectory_samples.jsonl' '{"conversations":[]}'
MakeFile 'hermesproj/failed_trajectories.jsonl' '{"conversations":[]}'
MakeFile 'Users/alice/.letta/sessions.jsonl' '{"type":"start","project":"C:\\lettaproj","agentId":"a1"}'
MakeFile 'lettaproj/.letta/settings.json' '{"hooks":{}}'
MakeFile 'Users/alice/.letta/lc-local-backend/conversations/c1/messages.jsonl' '{"type":"session","id":"c1","cwd":"C:\\lettaproj2"}'
MakeFile 'lettaproj2/AGENTS.md' '# letta'
MakeFile 'Users/alice/.pi/agent/sessions/--C--piproj--/2026-10-04T00-00-00-000Z_u1.jsonl' '{"type":"session","version":3,"id":"u1","cwd":"C:\\piproj"}'
MakeFile 'piproj/.pi/settings.json' '{}'
MakeFile 'Users/alice/.pi/agent/2026-10-01T00-00-00-000Z_u2.jsonl' '{"type":"session","version":1,"id":"u2","cwd":"C:\\piproj2"}'
MakeFile 'piproj2/AGENTS.md' '# pi'
MakeFile 'Users/alice/.pi/agent/trust.json' '{"C:\\pitrust":true}'
MakeFile 'pitrust/pi-session-2026-10-04.html' '<html></html>'
MakeFile 'Users/alice/.pi/agent/crashes.json' '[{"cwd":"C:\\picrash","sessionFile":"x.jsonl"}]'
MakeFile 'picrash/AGENTS.md' '# pi'
MakeFile 'Users/alice/.pi/agent/experimental/sessions/s1/meta.json' '{"id":"s1","cwd":"C:\\piexp"}'
MakeFile 'piexp/AGENTS.md' '# pi'
MakeFile 'Users/alice/.openinterpreter/sessions/2026/10/04/rollout-2026-10-04T00-00-00-u1.jsonl' '{"type":"session_meta","payload":{"cwd":"C:\\oiproj"}}'
MakeFile 'oiproj/.openinterpreter/rules/r.md' 'rule'
MakeFile 'Users/alice/.openinterpreter/archived_sessions/rollout-2026-10-01T00-00-00-u2.jsonl' '{"type":"session_meta","payload":{"cwd":"C:\\oiarch"}}'
MakeFile 'oiarch/.codewhale/notes.md' 'notes'
MakeFile 'Users/alice/.openhands/workspaces.json' '{"workspaces":[{"id":"w1","path":"C:\\ohproj"}],"workspaceParents":[]}'
MakeFile 'ohproj/.openhands/setup.sh' 'echo setup'
MakeFile 'Users/alice/.openhands/agent-canvas/dev_conversations/a1/meta.json' '{"workspace":{"working_dir":"C:\\ohdev"}}'
MakeFile 'Users/alice/.openhands/agent-canvas/conversations/a2/meta.json' '{"workspace":{"working_dir":"C:\\ohcanvas"}}'
MakeFile 'Users/alice/.openhands/conversations/a3/base_state.json' '{"workspace":{"working_dir":"C:\\ohcli"}}'
foreach ($project in @('ohdev', 'ohcanvas', 'ohcli')) { MakeFile "$project/AGENTS.md" '# oh' }
MakeFile 'Users/alice/.pearai/sessions/sessions.json' '[{"sessionId":"s1","title":"t","workspaceDirectory":"C:\\pearproj"}]'
MakeFile 'pearproj/.pearairc.json' '{}'
MakeFile 'pearproj/.pearai-agent/mcp.json' '{"mcpServers":{}}'
MakeFile 'Users/alice/.openclaw/openclaw.json' "{`n  // JSON5`n  agents: {`n    defaults: { workspace: 'C:\\clawws' },`n    entries: { main: { agentDir: `"C:\\clawagent`" } },`n  },`n}"
MakeFile 'Users/alice/.clawdbot/clawdbot.json' '{"agents":{"defaults":{"workspace":"C:\\clawlegacy"}}}'
MakeFile 'Users/alice/.openclaw-work/openclaw.json' '{agents:{defaults:{workspace:"C:\\clawprofile"}}}'
MakeFile 'clawws/SOUL.md' '# soul'
MakeFile 'clawagent/AGENTS.md' '# a'
MakeFile 'clawlegacy/IDENTITY.md' '# id'
MakeFile 'clawprofile/MEMORY.md' '# mem'
MakeFile 'home/bob/.openclaw/openclaw.json' '{}'
MakeFile 'Users/alice/.nanobot/sessions/0123456789abcdef0123456789abcdef/.workspace' 'C:\nanows'
MakeFile 'nanows/HEARTBEAT.md' '# hb'
MakeFile 'Users/alice/.nanobot-work/config.json' '{"agents":{"defaults":{"workspace":"C:\\nanocfg"}}}'
MakeFile 'nanocfg/memory/MEMORY.md' '# mem'
MakeFile 'Users/alice/.tabby/config.toml' "[[repositories]]`nname = `"p`"`ngit_url = `"file:///C:/tabbyproj`"`n[[repositories]]`nname = `"u`"`ngit_url = 'file:///srv/tabbyunix'`n"
MakeFile 'tabbyproj/AGENTS.md' '# tabby'
MakeFile 'srv/tabbyunix/AGENTS.md' '# tabby'
MakeFile 'Users/alice/.twinny/embeddings/twinproj/manifest.json' '{"files":{"C:\\twinproj\\src\\a.ts":{"hash":"h1"},"C:\\twinproj\\b.ts":{"hash":"h2"}}}'
MakeFile 'twinproj/AGENTS.md' '# twin'
# --- 2026-10-04 Docker volumes: a root volume matched to Agent Zero, an unmatched volume,
# and a rootless volume in alice's profile.
MakeFile 'var/lib/docker/volumes/a0_usr/_data/chats/ctx1/chat.json' '{"id":"ctx1","name":"chat","history":"[]"}'
MakeFile 'var/lib/docker/volumes/a0_usr/_data/.env' 'API_KEY_OPENAI=sk-x'
MakeFile 'var/lib/docker/volumes/a0_usr/_data/tmp/playwright/junk' 'junk'
MakeFile 'var/lib/docker/volumes/pgdata/_data/x' 'pg'
MakeFile 'var/lib/docker/volumes/metadata.db' 'metadata'
MakeFile 'Users/alice/.local/share/docker/volumes/tabby_data/_data/events/2026-10-01.json' '{"event":"completion"}'
# --- 2026-10-04 Muse Code
$CasesMuse = @'
excl||.local/share/muse/plugins/cache/builtin
excl||.local/share/muse/skills/bundled
excl||.local/share/muse/plugins/marketplaces
excl||.local/bin/.muse-update.Ab12Cd
excl||.muse/worktrees
exclf||AppData/Local/Programs/muse/muse-bin-1.4.2-R1.exe
exclf||AppData/Local/Programs/muse/.muse-update.4242.exe
secret||.config/muse/auth.json
secret||.config/muse/settings.json
agent|muse-code|.config/muse/trust.json
agent|muse-code|.local/share/muse/session-index.db
agent|muse-code|.local/share/muse/plugins/cache/local/p1/plugin.json
agent|muse-code|.local/bin/.muse-version
agent|muse-code|.local/bin/.muse-update-lock/pid
agent|muse-code|.muse/projects/slug/tracking.json
agent|muse-code|AppData/Local/Programs/muse/.muse-version
agent|muse-code|AppData/Local/Programs/muse/.muse-launcher.ps1
agent|muse-code|AppData/LocalLow/muse-shell-sandbox-01a0f000-0000-7000-8000-000000000001/tmp1.txt
'@
$CasesMuse = @($CasesMuse -split "`r?`n" | Where-Object { $_ -ne '' })
foreach ($caseLine in $CasesMuse) {
  $fields = @($caseLine -split '\|', 3)
  if ($fields[0] -eq 'excl') { MakeFile "Users/alice/$($fields[2])/f" 'x' } else { MakeFile "Users/alice/$($fields[2])" 'x' }
}
MakeFile 'Users/alice/.local/bin/othertool' 'x'
MakeFile 'Users/alice/.config/muse/trust.json' '{"schema_version":1,"projects":{"C:\\musetrust":{"decision":"trusted"}}}'
MakeFile 'Users/alice/.local/share/muse/sessions/2026/10/04/01a0f000-0000-7000-8000-000000000001/session.jsonl' '{"schema_version":1,"stream":{"kind":"session","id":"01a0f000-0000-7000-8000-000000000001"},"payload_type":"runtime.session.metadata","payload":{"kind":"metadata","record":{"model_id":"m","workspace_root":"C:\\museproj"}}}'
MakeFile 'museproj/.muse/hooks.json' '{}'
MakeFile 'museproj/.muse/worktrees/w1/f' 'x'
MakeFile 'musetrust/AGENTS.md' '# trusted'

# --- 2026-10-04 relocated Claude Code home
# ~/.claude-code-router is another tool's; the .claude-* lines must not claim it.
$ClaudeWorkSession = 'alice/.claude-work/projects/-srv-ccwork/0a0b0c0d-0000-4000-8000-000000000001.jsonl'
MakeFile "Users/$ClaudeWorkSession" '{"type":"user","cwd":"C:\\ccwork","message":{"role":"user","content":"hi"}}'
MakeFile 'Users/alice/.claude-work/history.jsonl' '{"display":"hi","project":"C:\\cchist","timestamp":1790848800000}'
MakeFile 'Users/alice/.claude-work/.claude.json' '{"projects":{"C:\\ccwork":{"allowedTools":[]}}}'
MakeFile 'Users/alice/.claude-work/.credentials.json' '{"claudeAiOauth":{"accessToken":"sk-ant-oat-SECRET"}}'
# 2026-10-23 (#256)
$ClaudeWorkKey = 'Users/alice/.claude-work/sessions/4343.0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef.key'
MakeFile $ClaudeWorkKey 'session key'
MakeFile 'Users/alice/.claude-work/daemon/control.key' '0123456789abcdef0123456789abcdef'
MakeFile 'Users/alice/.claude-work/daemon/roster.json' '{"workers":{}}'
MakeFile 'Users/alice/.claude-work/backups/.claude.json.backup.1792800000000' "{`"oauthAccount`":{}}`n"
MakeFile 'Users/alice/.claude-code-router/config.json' '{"PORT":3456}'
MakeFile 'ccwork/.claude/settings.json' '{}'
MakeFile 'cchist/.claude/settings.json' '{}'

# 2026-10-04 bug round: #29
# Catalog entries differing only by case are distinct directories under pwsh on
# Linux and one directory on NTFS and APFS.
MakeFile 'Users/alice/.config/PearAI/User/case29-upper.json' '{}'
MakeFile 'Users/alice/.config/pearai/User/case29-lower.json' '{}'
MakeFile 'Users/alice/.config/cursor/case29-cli.json' '{}'
MakeFile 'Users/alice/.config/Cursor/User/case29-editor.json' '{}'
$caseInsensitive = [System.IO.File]::Exists((ImagePath 'Users/alice/.config/pearai/User/case29-upper.json'))

# 2026-10-06 Ollama (#44)
# smoke.sh also checks a home listed only in etc/passwd, which this collector
# does not read.
$ollamaManifest = 'home/bob/.ollama/models/manifests/registry.ollama.ai/library/llama3.2/latest'
MakeFile $ollamaManifest '{"schemaVersion":2}'
[System.IO.File]::SetLastWriteTimeUtc((ImagePath $ollamaManifest), (New-Object DateTime 2026, 9, 14, 0, 0, 0, ([DateTimeKind]::Utc)))
# 2026-10-06 frameworks sweep (#64)
# CrewAI's Windows line is a case class, tested as CrewAI for alice and crewai
# for bob; a Camel JBang status file in ~/.camel must not be collected.
$CasesFrameworks = @'
agent|autogen-studio|.autogenstudio/autogen04203.db
secret||.autogenstudio/.env
secret||.autogenstudio/temp_env_vars.env
secret||.config/crewai/settings.json
secret||.local/share/crewai/credentials/tokens.enc
secret||.local/share/crewai/credentials/secret.key
secret||Library/Application Support/crewai/credentials/tokens.enc
secret||AppData/Local/CrewAI/credentials/secret.key
agent|crewai|.crewai/model_catalog_cache.json
agent|crewai|.local/share/myproj/latest_kickoff_task_outputs.db
agent|crewai|.local/share/myproj/latest_kickoff_task_outputs.db-wal
agent|crewai|.local/share/myproj/flow_states.db
agent|crewai|.local/share/myproj/.crewai_user.json
agent|crewai|Library/Application Support/myproj/latest_kickoff_task_outputs.db
agent|crewai|AppData/Local/CrewAI/myproj/latest_kickoff_task_outputs.db
secret||.camel/gmail_token.json
agent|camel-ai|.camel/gmail_token.json
agent|camel-ai|.camel/skills/s1/SKILL.md
agent|camel-ai|.config/camel/skills/s2/SKILL.md
secret||.config/difyctl/tokens.yml
secret||AppData/Roaming/difyctl/tokens.yml
secret||AppData/Local/difyctl/tokens.yml
agent|dify|.config/difyctl/config.yml
excl||.flowise/vectorstore
secret||.flowise/encryption.key
secret||.flowise/jwt_auth_token_secret.key
agent|flowise|.flowise/database.sqlite
excl||.langflow/knowledge_bases
excl||AppData/Roaming/com.LangflowDesktop/knowledge_bases
excl||Library/Application Support/com.LangflowDesktop/venv
excl||Library/Application Support/com.LangflowDesktop/python_env
excl||AppData/Local/com.LangflowDesktop/venv
excl||AppData/Local/com.LangflowDesktop/python_env
excl||AppData/Local/com.LangflowDesktop/cache
excl||.cache/langflow/profile_pictures
excl||Library/Caches/langflow/profile_pictures
excl||AppData/Local/langflow/langflow/Cache/profile_pictures
secret||.cache/langflow/secret_key
secret||.cache/langflow/cache_secret_key
secret||Library/Caches/langflow/secret_key
secret||AppData/Local/langflow/langflow/Cache/secret_key
secret||.langflow/data/.env
secret||AppData/Roaming/com.LangflowDesktop/data/.env
agent|langflow|.langflow/data/database.db
agent|langflow|Library/Logs/com.LangflowDesktop/langflow.log
secret||.metagpt/config2.yaml
secret||.metagpt/config2.bak
excl||.n8n/nodes/node_modules
excl||.n8n/custom/node_modules
excl||.n8n/node-definitions
excl||.n8n/n8n-sdk-templates
secret||.n8n/config
secret||.n8n/ssh/key
agent|n8n|.n8n/database.sqlite
agent|n8n|.n8n-local-gateway/log
agent|n8n|.n8n-gateway/settings.json
secret||.config/pydantic-clai2/credentials-default.enc
secret||.config/pydantic-clai2/logfire/logfire_credentials.json
agent|pydantic-clai|.config/pydantic-clai2/sessions.db
agent|pydantic-clai|.pydantic-ai/prompt-history.txt
excl||.config/open-interpreter/models
excl||Library/Application Support/open-interpreter/models
excl||AppData/Local/open-interpreter/open-interpreter/models
secret||.config/open-interpreter/profiles/default.yaml
secret||Library/Application Support/open-interpreter/profiles/default.yaml
secret||AppData/Local/open-interpreter/open-interpreter/profiles/default.yaml
secret||.config/Open Interpreter/config.yaml
secret||Library/Application Support/Open Interpreter Terminal/config.yaml
secret||AppData/Local/Open Interpreter/Open Interpreter/config.yaml
agent|open-interpreter|.config/open-interpreter/conversations/hello__October_06_2026_10-00-00.json
agent|open-interpreter|.config/Open Interpreter Terminal/conversations/old.json
agent|open-interpreter|.cache/open-interpreter/telemetry_user_id
'@
$CasesFrameworks = @($CasesFrameworks -split "`r?`n" | Where-Object { $_ -ne '' })
foreach ($caseLine in $CasesFrameworks) {
  $fields = @($caseLine -split '\|', 3)
  if ($fields[0] -eq 'excl') { MakeFile "Users/alice/$($fields[2])/f" 'x' } else { MakeFile "Users/alice/$($fields[2])" 'x' }
}
MakeFile 'Users/alice/.camel/4242-status.json' '{}'
MakeFile 'Users/alice/.local/share/myproj/unrelated.txt' 'x'
MakeFile 'home/bob/AppData/Local/crewai/myproj/latest_kickoff_task_outputs.db' 'x'
MakeFile 'home/bob/AppData/Local/crewai/credentials/tokens.enc' 'x'

$clawLinkOk = $false
try {
  New-Item -ItemType SymbolicLink -Path (ImagePath 'home/bob/.clawdbot') -Target '.openclaw' -ErrorAction Stop | Out-Null
  $clawLinkOk = $true
} catch { Write-Output "note: symlink creation not permitted here, .clawdbot symlink check skipped" }
$linkOk = $false
try {
  New-Item -ItemType SymbolicLink -Path (ImagePath 'Users/alice/.gemini/antigravity-cli/cli.log') -Target (Join-Path 'log' 'cli-1.log') -ErrorAction Stop | Out-Null
  $linkOk = $true
} catch { Write-Output "note: symlink creation not permitted here, symlink checks skipped" }

# 2026-10-07 (#71)
# Manifest rows are written by hand: names whose JSON needs escaping (a quote,
# a backslash and a tab on POSIX) or that 5.1's ConvertTo-Json wrote as \u
# escapes (an apostrophe, an ampersand). Built with .NET so the backslash
# stays part of the name.
$EscapedDir = [System.IO.Path]::Combine($Root, 'Users', 'alice', '.claude', 'projects', '-C-proj71')
[void][System.IO.Directory]::CreateDirectory($EscapedDir)
$EscapedNames = @("amp&apos's.jsonl")
if ($Separator -eq '/') { $EscapedNames += @('q"uote.jsonl', 'back\slash.jsonl', "tab`there.jsonl") }
foreach ($fileName in $EscapedNames) { [System.IO.File]::WriteAllText([System.IO.Path]::Combine($EscapedDir, $fileName), '{"type":"user"}', (New-Object System.Text.UTF8Encoding $false)) }

# 2026-10-10 long paths (#77)
# A transcript more than 260 characters below the image root, which must be
# collected without LongPathsEnabled on Windows.
$DeepTranscript = 'Users/alice/.claude/projects/-C-deep77/' + ((1..5 | ForEach-Object { 'd' + $_ + ('x' * 38) }) -join '/') + '/deep.jsonl'
$DeepBody = '{"type":"user","message":{"role":"user","content":"deep"},"timestamp":"2026-10-10T00:00:00Z"}'
$DeepSha = ([BitConverter]::ToString([System.Security.Cryptography.SHA256]::Create().ComputeHash((New-Object System.Text.UTF8Encoding $false).GetBytes($DeepBody))) -replace '-', '').ToLower()
MakeFile $DeepTranscript $DeepBody

# 2026-10-11 Claude Desktop Linux deb and Claude Code 2.1.295 session keys (#73, #76)
$claudeLinuxConfig = 'Users/alice/.config/Claude'
$spaceIds = '0a0a0a0a-0000-4000-8000-00000000000a/0b0b0b0b-0000-4000-8000-00000000000b'
$sessionKeyHex = '0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef'
$sessionKey = "Users/alice/.claude/sessions/4242.$sessionKeyHex.key"
$linkKey = "$claudeLinuxConfig/spaces-present/0a0a0a0a/0b0b0b0b/link-key"
$codeTabRecord = "$claudeLinuxConfig/claude-code-sessions/$spaceIds/local_76767676-7676-4676-8676-767676767676.json"
MakeFile $linkKey 'link-key ciphertext'
MakeFile "$claudeLinuxConfig/git-shadow/cli-temp-roots.json" '["/tmp/claude-1000"]'
MakeFile "$claudeLinuxConfig/logs/main.log" '2026-10-11 10:00:00 [info] [CCD] Installed'
MakeFile "$claudeLinuxConfig/claude-code/2.1.295/9b32b47ec4b3/claude" 'ELF'
MakeFile "$claudeLinuxConfig/claude-code/2.1.295/9b32b47ec4b3/.payload" ('{"sha256":"' + $sessionKeyHex + '","size":4}')
MakeFile "$claudeLinuxConfig/claude-code/2.1.295/9b32b47ec4b3/.verified" $sessionKeyHex
MakeFile $codeTabRecord '{"sessionId":"local_76767676-7676-4676-8676-767676767676","cliSessionId":"76767676-0000-4000-8000-000000000076","cwd":"/srv/proj","originCwd":"/srv/proj"}'
MakeFile 'Users/alice/.claude/sessions/4242.json' '{"pid":4242,"sessionId":"76767676-0000-4000-8000-000000000076","entrypoint":"claude-desktop","kind":"interactive"}'
MakeFile $sessionKey 'session key'

# 2026-10-11 Claude Code CLI cache directory (#81)
$cliCacheLog = 'Users/alice/Library/Caches/claude-cli-nodejs/-home-alice-proj/mcp-logs-terminal/2026-10-12T02-23-51-709Z.jsonl'
MakeFile $cliCacheLog '{"cwd":"/home/alice/proj","debug":"MCP server \"terminal\": connected","sessionId":"81818181-0000-4000-8000-000000000081","timestamp":"2026-10-12T02:23:51.709Z"}'
# The Windows layout puts the slug directories under Cache/, which the
# catalog names as Cache/* so the PowerShell exclusion */Claude*/Cache,
# matched without case, does not prune it.
$cliCacheLogWindows = 'Users/alice/AppData/Local/claude-cli-nodejs/Cache/C--Users-alice-proj/mcp-logs-ccd-host/2026-10-12T02-23-51-709Z.jsonl'
MakeFile $cliCacheLogWindows '{"cwd":"C:\\Users\\alice\\proj","debug":"MCP server \"ccd-host\": connected","sessionId":"81818181-0000-4000-8000-000000000081","timestamp":"2026-10-12T02:23:51.709Z"}'
# 2026-10-13 Antigravity desktop app (#83, #84)
# The Electron Cache is excluded by explicit bases: */Antigravity/Cache, matched
# without case here, would also prune .gemini/antigravity/cache.
$antigravityData = 'Users/alice/.gemini/antigravity'
$antigravityElectron = 'Users/alice/Library/Application Support/Antigravity'
$antigravityWindows = 'Users/alice/AppData/Roaming/Antigravity'
MakeFile "$antigravityData/conversations/a1.db" 'sqlite'
MakeFile "$antigravityData/crashes/crash_1.log" 'crash'
MakeFile "$antigravityData/plugin_data/gdocs/token.json" '{"token":{"access_token":"x","refresh_token":"y"}}'
MakeFile "$antigravityData/plugin_data/gdocs/bin/gdocs-1" 'bin'
MakeFile "$antigravityData/bin/webm_encoder" 'bin'
MakeFile "$antigravityData/cache/marketplaces/official/marketplace.json" '{"name":"official","plugins":[]}'
MakeFile "$antigravityData/cache/last_conversations.json" '{"x":"a1"}'
MakeFile "$antigravityElectron/app_storage.json" '{"lastCreatedProjectId":"p1"}'
MakeFile "$antigravityElectron/Cache/data_0" 'cache'
MakeFile "$antigravityElectron/Code Cache/js/x" 'js'
MakeFile 'Users/alice/Library/Logs/Antigravity/main.log' '[2026-10-13 18:28:00.000] [info]  Starting app (v2.23.0)'
MakeFile "$antigravityWindows/app_storage.json" '{"lastCreatedProjectId":"p1"}'
MakeFile "$antigravityWindows/Cache/data_0" 'cache'
# 2026-10-13 Antigravity Linux round (#83, #90, #91)
# In the Windows layout; plugin-secrets.json is in AppData/Roaming/Antigravity,
# the directory a case-insensitive filesystem shares with antigravity.
MakeFile "$antigravityWindows/plugin-secrets.json" '{"gdocs":"x"}'
MakeFile "$antigravityWindows/Dictionaries/en-US-10-1.bdic" 'bdic'
MakeFile "$antigravityData/lib/node_modules/sidecar_sdk/index.mjs" 'export {}'
MakeFile 'Users/alice/.gemini/config/node_modules/sidecar_sdk/index.mjs' 'export {}'
MakeFile 'Users/alice/.gemini/config/projects/91919191-0000-4000-8000-000000000091.json' (@('{', '  "id": "91919191-0000-4000-8000-000000000091",', '  "projectResources": {', '    "resources": [', '      {', '        "folderUri":  "file:///C:/agyapp"', '      }', '    ]', '  }', '}', '') -join "`n")
MakeFile 'Users/alice/.gemini/config/projects/91919191-0000-4000-8000-000000000092.json' (@('{', '  "id": "91919191-0000-4000-8000-000000000092",', '  "projectResources": {', '    "resources": [', '      {', '        "folderUri":  "file:///c%3A/agywin"', '      }', '    ]', '  }', '}', '') -join "`n")
MakeFile 'agyapp/.antigravityignore' 'ignore'
MakeFile 'agywin/.antigravityignore' 'ignore'
# 2026-10-14 home patterns and the working directory (#105)
# The cwd105 run's working directory: the image's homes must still be found.
$homesCwd = Join-Path $Work 'cwd105'
New-Item -ItemType Directory -Path (Join-Path (Join-Path $homesCwd 'home') 'analyst') -Force | Out-Null
New-Item -ItemType Directory -Path (Join-Path (Join-Path $homesCwd 'Users') 'analyst') -Force | Out-Null
# 2026-10-14 catalog match that is an excluded file (#107)
MakeFile 'Users/alice/.local/bin/.muse-update.107.exe' "1070`n"
# 2026-10-14 output directory under a file (#108)
$plainFile = Join-Path $Work 'file108'
[System.IO.File]::WriteAllText($plainFile, "x`n")
# 2026-10-14 exclusion globs and the working directory (#109)
# The cwd109 run's working directory holds another tree the glob matches.
MakeFile 'Users/alice/.claude/plugins/y/node_modules/f.js' "module.exports = 1`n"
$globCwd = Join-Path $Work 'cwd109'
New-Item -ItemType Directory -Path (Join-Path $globCwd '.claude/plugins/x/node_modules') -Force | Out-Null
# 2026-10-14 Antigravity on Windows (#83)
$aliceLocalAppData = 'Users/alice/AppData/Local'
MakeFile "$aliceLocalAppData/antigravity/staging/s1" "staged`n"
MakeFile "$aliceLocalAppData/antigravity-updater/installer.exe" "exe`n"
MakeFile "$aliceLocalAppData/Programs/antigravity/Antigravity.exe" "exe`n"

# 2026-10-15 ChatGPT desktop as the OpenAI.Codex package (#113)
$chatgptPackage = 'Users/alice/AppData/Local/Packages/OpenAI.Codex_2p2nqsd0c76g0'
$chatgptProfile = "$chatgptPackage/LocalCache/Roaming/Codex/web/Codex"
$chatgptPartition = "$chatgptProfile/Default/Partitions/codex-browser-app"
MakeFile "$chatgptProfile/Default/Local Storage/leveldb/000003.log" "ldb`n"
MakeFile "$chatgptProfile/Local State" "{`"os_crypt`":{}}`n"
MakeFile "$chatgptProfile/Default/Network/Cookies" "cookies`n"
MakeFile "$chatgptPartition/Network/Cookies" "cookies`n"
MakeFile "$chatgptProfile/Default/Login Data" "logins`n"
MakeFile "$chatgptProfile/Default/Cache/data_0" "cache`n"
MakeFile "$chatgptPartition/Cache/data_0" "cache`n"
MakeFile "$chatgptProfile/component_crx_cache/metadata.json" "{}`n"
MakeFile "$chatgptPackage/LocalCache/Local/Codex/Logs/2026/10/15/codex-desktop-x.log" "log`n"
MakeFile "$chatgptPackage/Settings/settings.dat" "hive`n"
MakeFile 'Users/alice/.codex/plugins/.plugin-appserver/codex.exe' "exe`n"
MakeFile 'Users/alice/.codex/.codex-global-state.json' "{}`n"

# 2026-10-15 ChatGPT desktop on macOS (#121)
$chatgptMac = 'Users/alice/Library/Application Support/Codex'
$chatgptMacPartition = "$chatgptMac/Default/Partitions/codex-browser-app"
$cuaGroupContainer = 'Users/alice/Library/Group Containers/2DC432GLL2.com.openai.sky.CUAService'
$computerUse = 'Users/alice/.codex/computer-use'
MakeFile "$chatgptMac/Default/Local Storage/leveldb/000003.log" "ldb`n"
MakeFile "$chatgptMac/Default/Cookies" "cookies`n"
MakeFile "$chatgptMacPartition/Cookies" "cookies`n"
MakeFile "$chatgptMac/Default/Login Data" "logins`n"
MakeFile "$chatgptMac/Default/Cache/data_0" "cache`n"
MakeFile "$chatgptMacPartition/Cache/data_0" "cache`n"
MakeFile "$chatgptMac/executor-plugins/codex-app-tools/server.mjs" "js`n"
MakeFile "$chatgptMac/executor-plugins/codex-app-tools/.mcp.json" "{}`n"
MakeFile "$chatgptMac/component_crx_cache/metadata.json" "{}`n"
MakeFile 'Users/alice/Library/Logs/com.openai.codex/2026/10/15/codex-desktop-x.log' "log`n"
MakeFile 'Users/alice/Library/Preferences/com.openai.codex.plist' "plist`n"
MakeFile 'Users/alice/Library/HTTPStorages/com.openai.codex.binarycookies' "cookies`n"
MakeFile "$cuaGroupContainer/IPC/computeruse.sock.lock" ''
MakeFile "$computerUse/Codex Computer Use.app/Contents/Info.plist" "plist`n"
MakeFile "$computerUse/config.json" "{}`n"

# 2026-10-16 GitHub Copilot Desktop (#147)
$copilot = 'Users/alice/.copilot'
$copilotDesktopData = 'Users/alice/.local/share/com.github.githubapp'
MakeFile "$copilot/data.db" "sqlite`n"
MakeFile "$copilot/data.db-wal" "wal`n"
MakeFile "$copilot/run/ws.token" ('A' * 49 + "`n")
MakeFile "$copilot/run/ws.port" "12345`n"
MakeFile "$copilot/chats/2026-10-16/x-5d404e14/.keep" ''
MakeFile "$copilot/repos/proj/README.md" "# proj`n"
$copilotCloneGit = "$copilot/repos/proj/.git"
$copilotCheckpointRef = "$copilotCloneGit/refs/copilot/checkpoints/5e55a0a1-0000-4000-8000-000000000001/00000000000000000001"
MakeFile "$copilotCloneGit/HEAD" "ref: refs/heads/main`n"
MakeFile "$copilotCheckpointRef/e0000000-0000-4000-8000-000000000020" "0123456789abcdef0123456789abcdef01234567`n"
MakeFile "$copilotCloneGit/objects/ab/cdef0123" "object`n"
MakeFile "$copilotCloneGit/index" "index`n"
MakeFile "$copilot/repos/copilot-worktrees/proj/alice-x/AGENTS.md" "# wt`n"
MakeFile "$copilot/logs/github-app.1.log" "log`n"
$copilotChatCwd = 'C:\Users\alice\.copilot\chats\2026-10-16\x-5d404e14'
MakeFile "$copilot/session-state/s2/workspace.yaml" "cwd: $copilotChatCwd`nclient_name: github/autopilot`n"
$copilotWorktree = 'C:\Users\alice\.copilot\repos\copilot-worktrees\proj\alice-x'
MakeFile "$copilot/session-state/s3/workspace.yaml" "cwd: $copilotWorktree`nclient_name: github/autopilot`n"
MakeFile "$copilotDesktopData/localstorage/tauri_localhost_0.localstorage" "sqlite`n"
MakeFile "$copilotDesktopData/app-skills/af/SKILL.md" "# af`n"
MakeFile 'Users/alice/.github-copilot-cli/logs/foundry.core20261016.log' "log`n"

# 2026-10-17 Copilot Desktop Windows round (#147)
$copilotWebView = 'Users/alice/AppData/Local/com.github.githubapp/EBWebView'
$copilotChatProject = "$copilot/chats/2026-10-17/x-90825c78/proj"
$copilotMarketplaces = 'Users/alice/AppData/Local/copilot/marketplaces'
MakeFile "$copilotWebView/Default/Local Storage/leveldb/000003.log" "leveldb`n"
MakeFile "$copilotWebView/Default/Network/Cookies" "cookies`n"
MakeFile "$copilotWebView/Local State" "{`"os_crypt`":{}}`n"
MakeFile "$copilotWebView/Default/Cache/Cache_Data/x" "cache`n"
MakeFile "$copilotWebView/component_crx_cache/x" "crx`n"
MakeFile "$copilotChatProject/package.json" "{`"name`":`"proj`"}`n"
MakeFile "$copilotChatProject/node_modules/m/index.js" "module`n"
MakeFile "$copilotMarketplaces/m/.git/HEAD" "ref: refs/heads/main`n"
foreach ($session in @('s1', 's2', 's3')) {
  MakeFile "$copilot/session-state/$session/events.jsonl" "{`"type`":`"session.start`"}`n"
}
MakeFile "$copilot/session-state/s4/workspace.yaml" "cwd: C:\\coppool`nclient_name: github/autopilot`n"
MakeFile 'coppool/AGENTS.md' '# pool'

# 2026-10-17 session sync round (#153)
$nanobotPairing = 'Users/alice/.nanobot/webui/remote-pairing/11111111-1111-4111-8111-111111111111'
$clineRemoteCache = 'Users/alice/.vscode-server/data/User/globalStorage/saoudrizwan.claude-dev/cache'
$agentZeroPluginConfig = 'plugins/_telegram_integration/config.json'
$agentZeroVolume = 'var/lib/docker/volumes/a0_usr/_data'
MakeFile "$nanobotPairing/identity" "key`n"
MakeFile "$nanobotPairing/exchange" "key`n"
MakeFile "$nanobotPairing/request" "{}`n"
MakeFile "Users/alice/agent-zero/main/usr/$agentZeroPluginConfig" '{"token":"x"}'
MakeFile "Users/alice/Desktop/agent-zero/usr/$agentZeroPluginConfig" '{"token":"x"}'
MakeFile "$agentZeroVolume/$agentZeroPluginConfig" '{"token":"x"}'
MakeFile "$agentZeroVolume/usr/$agentZeroPluginConfig" '{"token":"x"}'
MakeFile "$clineRemoteCache/remote_config_org1.json" "{`"promptUploading`":{}}`n"
MakeFile "$clineRemoteCache/sync-queue.json" "{`"items`":{}}`n"
MakeFile 'Users/alice/Documents/Cline/webhook_config.json' "{`"webhook_token`":`"x`"}`n"
MakeFile 'Users/alice/.gemini/tmp/gem/bug-memory-1792000000000.heapsnapshot' "heap`n"
MakeFile 'Users/alice/.little-coder/config.json' "{`"gist_token`":`"x`"}`n"
MakeFile 'Users/alice/.qwen/channels/qqbot-credentials.json' "{`"appId`":`"x`",`"appSecret`":`"x`"}`n"
MakeFile 'Users/alice/.qwen/channels/weixin/account.json' "{`"token`":`"x`"}`n"
MakeFile 'Users/alice/.qwen/channels/sessions.json' "{}`n"
MakeFile 'Users/alice/.qwen/channels/daemon/0123abcd/routes.json' "{}`n"
MakeFile 'Users/alice/.local/share/kilo/log/2026-10-17T120000.log' "INFO creating session`n"

# 2026-10-18 Codex shell snapshots (#160)
$codexShellSnapshot = 'Users/alice/.codex/shell_snapshots/22222222-2222-4222-8222-222222222222.1792000000000000000.sh'
MakeFile $codexShellSnapshot "# Snapshot file`nexport EXAMPLE_API_KEY=x`n"

# 2026-10-19 Codex projectless output directories (#149)
$codexOutputDir = 'home/bob/Documents/Codex/2026-10-18/w'
MakeFile 'home/bob/.codex/sessions/2026/10/03/rollout-2.jsonl' '{"type":"session_meta","payload":{"cwd":"/home/bob/Documents/Codex/2026-10-18/w"}}'
MakeFile "$codexOutputDir/outputs/report.md" "# report`n"
MakeFile "$codexOutputDir/work/notes.txt" "notes`n"

# 2026-10-20 Cursor IDE round on macOS (#6)
$cursorMac = 'Users/alice/Library/Application Support/Cursor'
$cursorMacWorker = "$cursorMac/User/globalStorage/anysphere.cursor-agent-worker"
$cursorMacUpdate = "$cursorMac/update-supervisor/33333333-3333-4333-8333-333333333333"
MakeFile "$cursorMac/AgentStores/cursor_agent_stores/u1/.sync/index.sqlite" "sqlite`n"
MakeFile "$cursorMac/AgentStores/cursor_agent_stores/.control/u1/pins/1-x.json" "{`"purpose`":`"foreground`"}`n"
MakeFile "$cursorMacUpdate/events.jsonl" "{`"type`":`"supervisor_started`"}`n"
MakeFile "$cursorMacUpdate/cursor-update-supervisor" "binary`n"
MakeFile "$cursorMacWorker/agent-cli/.local/share/x" "cli`n"
MakeFile "$cursorMacWorker/cursor-agent-worker-1.log" "worker log`n"
MakeFile 'Users/alice/.cursor/projects/p/canvases/node_modules/x.d.ts' "declare const x: number;`n"
MakeFile 'Users/alice/.cursor/projects/p/agent-transcripts/u/u.jsonl' "{`"type`":`"turn_ended`",`"status`":`"success`"}`n"
MakeFile 'Users/alice/.cursor/statsig-cache.json' "{}`n"

# 2026-10-20 Cursor IDE round on Ubuntu (#236)
$cursorLinuxStores = 'home/bob/.local/state/cursor/agent-stores/cursor_agent_stores'
MakeFile "$cursorLinuxStores/u1/.sync/index.sqlite" "sqlite`n"
MakeFile "$cursorLinuxStores/.control/u1/pins/1-x.json" "{`"purpose`":`"foreground`"}`n"

# 2026-10-21 Cursor IDE round on Windows (#236)
$cursorWindowsStores = 'Users/alice/AppData/Local/Cursor/AgentStores/cursor_agent_stores'
$cursorWindowsWorker = 'Users/alice/AppData/Roaming/Cursor/User/globalStorage/anysphere.cursor-agent-worker'
MakeFile "$cursorWindowsStores/u1/.sync/index.sqlite" "sqlite`n"
MakeFile "$cursorWindowsStores/.control/u1/pins/1-x.json" "{`"purpose`":`"foreground`"}`n"
MakeFile "$cursorWindowsWorker/agent-cli/x" "cli`n"
MakeFile "$cursorWindowsWorker/cursor-agent-worker-1.log" "worker log`n"
# An installed plugin's marketplace copy under .cursor/plugins/cache is
# excluded; plugins/local beside it is collected.
$cursorPlugins = 'Users/alice/.cursor/plugins'
MakeFile "$cursorPlugins/cache/cursor-public/x/0123abcd/mcp.json" "{`"mcpServers`":{}}`n"
MakeFile "$cursorPlugins/local/x.json" "{}`n"
# The updater's copy of inno_updater.exe under update-supervisor is excluded.
$cursorUpdateSupervisor = 'Users/alice/AppData/Roaming/Cursor/update-supervisor/win32/0000'
MakeFile "$cursorUpdateSupervisor/inno_updater.exe" "binary`n"
MakeFile "$cursorUpdateSupervisor/app-ready.json" "{`"currentVersion`":`"x`"}`n"
MakeFile "$cursorUpdateSupervisor/state.json" "{`"stage`":`"x`"}`n"
# 2026-10-21 OpenHands Agent Canvas credentials and workspaces (#218)
$canvasCredentials = @(
  'Users/alice/AppData/Roaming/OpenHands Agent Canvas/mars-credentials.json',
  'Users/alice/Library/Application Support/OpenHands Agent Canvas/mars-credentials.json',
  'Users/alice/.config/OpenHands Agent Canvas/mars-credentials.json'
)
foreach ($canvasFile in $canvasCredentials) { MakeFile $canvasFile "{`"version`":1,`"connections`":[]}`n" }
MakeFile 'Users/alice/.openhands/workspaces/run1/f' "x`n"

# 2026-10-21 Agent Zero source review (#217)
$whatsappCreds = 'tmp/whatsapp/session/creds.json'
$agentZeroCapture = 'Users/alice/AppData/Local/Temp/_a0_connector/computer_use/ctx1/0000.png'
MakeFile "Users/alice/agent-zero/main/$whatsappCreds" '{}'
MakeFile "Users/alice/Desktop/agent-zero/$whatsappCreds" '{}'
MakeFile "var/lib/docker/volumes/a0_usr/_data/$whatsappCreds" '{}'
MakeFile 'Users/alice/agent-zero/main/usr/chats/acp1/chat.json' ('{"id": "acp1", "data": ' +
  '{"acp_cwd": "/srv/a0acp", "acp_additional_directories": ["/srv/a0extra"]}}')
MakeFile 'srv/a0acp/AGENTS.md' '# acp'
MakeFile 'srv/a0extra/AGENTS.md' '# extra'
MakeFile $agentZeroCapture 'png'

# 2026-10-21 framework catalog gaps (#223, #224, #225)
MakeFile 'proj/.crewai/last_run.json' "{`"execution_id`":`"x`",`"tier`":`"ephemeral`"}`n"
foreach ($configDir in @('.config', 'AppData/Roaming', 'AppData/Local')) {
  MakeFile "Users/alice/$configDir/difyctl/login-pending.yml" "device_code: x`n"
}
$claiState = 'Users/alice/.local/state/pydantic-clai2'
MakeFile "$claiState/orchestrator.json" "{}`n"
MakeFile "$claiState/workspaces/registry/ws1/workspace.json" "{`"id`":`"ws1`"}`n"
MakeFile "$claiState/workspaces/registry/ws1/token" "token`n"
MakeFile "$claiState/workspaces/registry/ws1/session.log" "log`n"
MakeFile "$claiState/workspaces/clones/repo-0123456789/.git/HEAD" "ref: refs/heads/main`n"

# 2026-10-21 Codex worktree cwds and archived threads (#222)
MakeFile 'home/bob/.codex/archived_sessions/rollout-2026-10-01T00-00-00-a1.jsonl' '{"type":"session_meta","payload":{"cwd":"C:\\codexarch"}}'
MakeFile 'codexarch/AGENTS.md' "# agents`n"
MakeFile 'home/bob/.codex/sessions/2026/10/03/rollout-3.jsonl' '{"type":"session_meta","payload":{"cwd":"/home/bob/.codex/worktrees/a1b2/repo"}}'
MakeFile 'home/bob/.codex/worktrees/a1b2/repo/AGENTS.md' "# agents`n"
MakeFile 'Users/alice/.openinterpreter/sessions/2026/10/04/rollout-2026-10-04T00-00-00-u3.jsonl' '{"type":"session_meta","payload":{"cwd":"C:\\Users\\alice\\.openinterpreter\\worktrees\\c3d4\\repo"}}'
MakeFile 'Users/alice/.openinterpreter/worktrees/c3d4/repo/AGENTS.md' "# agents`n"

# 2026-10-21 Cody JetBrains plugin files (#214)
$jetbrainsOptions = 'Users/alice/AppData/Roaming/JetBrains/IntelliJIdea2025.2/options'
MakeFile "$jetbrainsOptions/cody.xml" "<application />`n"
MakeFile "$jetbrainsOptions/cody_accounts.xml" "<application />`n"
MakeFile "$jetbrainsOptions/cody_accounts_settings.xml" "<application />`n"
MakeFile "$jetbrainsOptions/cody_application_settings.xml" "<application />`n"
MakeFile 'Users/alice/.sourcegraph/cody_settings.json' "{}`n"
MakeFile 'proj/.idea/cody_history.xml' "<project version=`"4`" />`n"
MakeFile 'proj/.idea/.sourcegraph/cody_settings.json' "{}`n"

# 2026-10-21 agent worktrees inside a project (#216)
MakeFile 'gem/.gemini/worktrees/w1/f' "wt`n"
MakeFile 'gem/.qwen/worktrees/w1/f' "wt`n"
MakeFile 'gem/.gemini/settings.json' "{}`n"

# 2026-10-21 Hermes quick snapshots and desktop connections (#123, #208)
$hermesSnapshot = 'Users/alice/.hermes/state-snapshots/20261001-120004-pre-update'
$hermesSnapshotWindows = 'Users/alice/AppData/Local/hermes/state-snapshots/20261001-120004'
MakeFile "$hermesSnapshot/state.db" "sqlite`n"
MakeFile "$hermesSnapshot/manifest.json" "{`"id`":`"20261001-120004-pre-update`"}`n"
MakeFile "$hermesSnapshot/.env" "KEY=x`n"
MakeFile "$hermesSnapshot/auth.json" "{`"providers`":{}}`n"
MakeFile "$hermesSnapshot/pairing/telegram-approved.json" "{`"42`":{`"user_name`":`"u`"}}`n"
MakeFile "$hermesSnapshotWindows/state.db" "sqlite`n"
MakeFile 'Users/alice/.config/Hermes/connections.json' "{`"connections`":[]}`n"

# 2026-10-21 nanobot workspace cron and triggers (#220)
MakeFile 'nanows/cron/jobs.json' "{`"jobs`":[]}`n"
MakeFile 'nanows/cron/runs/j1_1_ab.json' "{`"job_id`":`"j1`",`"status`":`"ok`"}`n"
MakeFile 'nanows/triggers/triggers.json' "{`"triggers`":[]}`n"
MakeFile 'nanows/triggers/runs/d1.json' "{`"trigger_id`":`"t1`",`"status`":`"ok`"}`n"
MakeFile 'nanows/triggers/inbox/d2.json' "{`"id`":`"d2`"}`n"

# 2026-10-21 OpenClaw source review (#219)
$openclaw = 'Users/alice/.openclaw'
$openclawCloneId = '0123456789abcdef'
$openclawVolume = 'var/lib/docker/volumes/a0_usr/_data'
MakeFile "$openclaw/exec-approvals.json" "{`"socket`":{`"token`":`"x`"}}`n"
MakeFile 'Users/alice/.openclaw-auth-profile-secrets/gateway.env' "OPENAI_API_KEY=x`n"
MakeFile "$openclaw/projects/$openclawCloneId/repo/README.md" "clone`n"
MakeFile "$openclaw/projects/notes/todo.md" "notes`n"
MakeFile "$openclawVolume/exec-approvals.json" "{`"socket`":{`"token`":`"x`"}}`n"
MakeFile "$openclawVolume/projects/$openclawCloneId/repo/README.md" "clone`n"
MakeFile "$openclawVolume/projects/p219/notes.md" "project`n"
MakeFile "$openclawVolume/plugin-skills/s" "link`n"
MakeFile "$openclawVolume/agents/main/agent/tmp/t" "tmp`n"
MakeFile "$openclawVolume/agents/main/agent/.tmp/t" "tmp`n"

# 2026-10-21 OpenCode and Kilo Code source review (#213, #221)
$opencodeDesktop = 'Users/alice/Library/Application Support/ai.opencode.desktop'
$kiloSandboxPolicy = 'Users/alice/.local/state/kilo-sandbox-policy/aa'
MakeFile 'Users/alice/.config/opencode/service.json' "{`"password`":`"x`"}`n"
MakeFile 'Users/alice/.local/state/opencode/service-local.json' "{`"password`":`"x`"}`n"
MakeFile "$opencodeDesktop/drafts.sqlite" "sqlite`n"
MakeFile "$opencodeDesktop/compile-cache/x" "v8`n"
MakeFile 'Users/alice/.config/ai.opencode.desktop.beta/drafts.sqlite' "sqlite`n"
MakeFile 'Users/alice/AppData/Roaming/ai.opencode.desktop/drafts.sqlite' "sqlite`n"
MakeFile 'Users/alice/AppData/Local/Temp/opencode/notes.txt' "tmp`n"
MakeFile "$kiloSandboxPolicy/bb.json" "{`"enabled`":true,`"mode`":`"deny`",`"version`":1}`n"
MakeFile 'Users/alice/.local/state/kilo-sandbox-preference/bb.json' "false`n"

# 2026-10-21 Qwen Code session registry (#230)
MakeFile 'Users/alice/.qwen/sessions/4242.json' "{`"sessionId`":`"x`",`"ipcToken`":`"x`"}`n"
MakeFile 'Users/alice/.qwen/sessions/4243-0123abcd.json' "{`"sessionId`":`"x`",`"ipcToken`":`"x`"}`n"

# 2026-10-21 Roo Code CLI cloud credentials (#159)
MakeFile 'Users/alice/.roo/cli-credentials.json' "{`"token`":`"not-a-real-token`"}`n"

# 2026-10-21 Twinny workspace storage (#215)
$twinnyWorkspace = 'Users/alice/AppData/Roaming/Code/User/workspaceStorage/ws215'
MakeFile "$twinnyWorkspace/rjmacarthy.twinny/agent-changes/c1.json" "{`"id`":`"c1`",`"root`":`"C:\\twinproj`",`"createdAt`":1,`"files`":[]}`n"
MakeFile "$twinnyWorkspace/workspace.json" "{`"folder`":`"file:///c%3A/twinproj`"}`n"
# The twinny-server forge plugins keep access tokens in repos.json: flagged secret.
MakeFile 'Users/alice/.twinny/server/plugins/github/repos.json' "{`"version`":1,`"repos`":[],`"tokens`":[]}`n"

# 2026-10-21 project git metadata (#253)
$projectGit = 'proj/.git'
$commitId = '1111111111111111111111111111111111111111'
$tagId = '2222222222222222222222222222222222222222'
$zeroId = '0000000000000000000000000000000000000000'
$gitConfig = @(
  '[core]', "`trepositoryformatversion = 0", "`tbare = false",
  '[remote "origin"]', "`turl = https://git.example.com/acme/proj.git",
  "`tpushurl = git@git.example.com:acme/proj.git",
  "`tfetch = +refs/heads/*:refs/remotes/origin/*",
  '[remote "upstream"]', "`turl = https://git.example.org/upstream/proj.git",
  "`tfetch = +refs/heads/*:refs/remotes/upstream/*",
  '[branch "main"]', "`tremote = origin", "`tmerge = refs/heads/main"
)
MakeFile "$projectGit/HEAD" "ref: refs/heads/main`n"
MakeFile "$projectGit/config" (($gitConfig -join "`n") + "`n")
MakeFile "$projectGit/FETCH_HEAD" "$commitId`t`tbranch 'main' of https://git.example.com/acme/proj`n"
MakeFile "$projectGit/packed-refs" "# pack-refs with: peeled fully-peeled sorted`n$tagId refs/tags/v1`n"
MakeFile "$projectGit/refs/heads/main" "$commitId`n"
MakeFile "$projectGit/refs/remotes/origin/main" "$commitId`n"
MakeFile "$projectGit/logs/HEAD" "$zeroId $commitId A <a@example.com> 1759449600 +0000`tclone: from https://git.example.com/acme/proj.git`n"
MakeFile "$projectGit/logs/refs/remotes/origin/main" "$zeroId $commitId A <a@example.com> 1759449600 +0000`tfetch: fast-forward`n"
MakeFile "$projectGit/worktrees/wt1/HEAD" "ref: refs/heads/wt1`n"
MakeFile "$projectGit/worktrees/wt1/gitdir" "C:/proj-wt1/.git`n"
MakeFile "$projectGit/worktrees/wt1/index" "DIRC`n"
MakeFile "$projectGit/worktrees/wt1/index.lock" "DIRC`n"
MakeFile "$projectGit/worktrees/wt1/sharedindex.3333333333333333333333333333333333333333" "DIRC`n"
MakeFile "$projectGit/worktrees/wt1/modules/sub/HEAD" "ref: refs/heads/main`n"
MakeFile "$projectGit/worktrees/wt1/modules/sub/objects/ab/cdef" "object`n"
MakeFile "$projectGit/objects/ab/cdef" "object`n"
MakeFile "$projectGit/index" "DIRC`n"
MakeFile "$projectGit/hooks/pre-commit.sample" "#!/bin/sh`n"
MakeFile "$projectGit/lfs/objects/x" "lfs`n"
MakeFile "$projectGit/modules/sub/HEAD" "ref: refs/heads/main`n"

# 2026-10-23 claude-code Windows round (#256)
$claudeTempSession = 'Users/alice/AppData/Local/Temp/claude/C--proj/25625625-6256-4256-8256-256256256256'
MakeFile "$claudeTempSession/scratchpad/notes.txt" "scratch`n"
MakeFile "$claudeTempSession/tasks/b256b256b.output" "done`n"
MakeFile 'Users/alice/.claude.json.backup' "{`"oauthAccount`":{}}`n"
MakeFile 'Users/alice/.claude/daemon/control.key' '0123456789abcdef0123456789abcdef'
MakeFile 'Users/alice/.claude/daemon/pipe.key' '0123456789abcdef'
MakeFile 'Users/alice/.claude/daemon/roster.json' "{`"proto`":1,`"workers`":{}}`n"
MakeFile 'Users/alice/.claude/backups/.claude.json.backup.1792800000000' "{`"oauthAccount`":{}}`n"
MakeFile 'Users/alice/.claude/backups/.claude.json.corrupted.2026-10-23T17-00-00-000Z' "{`"oauthAccount`":{}}`n"
MakeFile 'Users/alice/.claude/paste-cache/0123456789abcdef.txt' "pasted text`n"
MakeFile 'Users/alice/.claude/plugins/plugin-directory-cache-v2.json' "{`"plugins`":[]}`n"
MakeFile "$projectGit/info/exclude" "# git ls-files --others --exclude-from=.git/info/exclude`n.claude/scheduled_tasks.lock`n"

# 2026-10-26 claude-code Linux round (#256)
MakeFile 'Users/alice/.local/state/claude/locks/2.1.310.lock' "{`"acquiredAt`":1792800000000,`"execPath`":`"/x`",`"pid`":1,`"version`":`"2.1.310`"}`n"
MakeFile 'Users/alice/.cache/claude/staging/entry' "staged`n"

# 2026-10-26 claude-code macOS round (#256)
$urlHandler = 'Users/alice/Applications/Claude Code URL Handler.app/Contents'
$bridgeSpawn = 'bridge-spawn/cse_0123456789abcdef0123'
MakeFile "$urlHandler/Info.plist" "<plist version=`"1.0`"><dict><key>CFBundleURLSchemes</key></dict></plist>`n"
foreach ($claudeDir in @('Users/alice/.claude', 'Users/alice/.claude-work')) {
  MakeFile "$claudeDir/ide/53256.lock" "{`"authToken`":`"00000000-0000-4000-8000-000000000256`",`"transport`":`"ws`"}`n"
  MakeFile "$claudeDir/$bridgeSpawn/mcp.json" "{`"mcpServers`":{`"claude-code-remote`":{`"headers`":{`"X-Session-UUID`":`"x`"}}}}`n"
}
MakeFile "Users/alice/.claude/$bridgeSpawn/append-system-prompt.txt" "appended system prompt`n"
$urlHandlerLinkOk = $false
try {
  New-Item -ItemType Directory -Path (ImagePath "$urlHandler/MacOS") -Force | Out-Null
  $urlHandlerTarget = [System.IO.Path]::Combine('..', '..', '..', '..', '.local', 'bin', 'claude')
  New-Item -ItemType SymbolicLink -Path (ImagePath "$urlHandler/MacOS/claude") -Target $urlHandlerTarget -ErrorAction Stop | Out-Null
  $urlHandlerLinkOk = $true
} catch { Write-Output 'note: symlink creation not permitted here, #256 URL handler symlink check skipped' }

# 2026-10-15 special files (#120)
# A Unix socket is a reparse point with an empty LinkType on Windows. Under
# 5.1, which lacks UnixDomainSocketEndPoint, a minimal EndPoint subclass builds
# the address. The socket is bound from inside its directory, so a long temp
# path cannot pass the socket path limit, and kept open until the checks after
# the default run; when none can be bound, the socket checks are skipped.
$socketPath = 'Users/alice/.codex/app-server-daemon/daemon-updater.sock'
$socket = $null
$endPointSource = @'
using System.Net;
using System.Net.Sockets;
using System.Text;
public class CacUnixEndPoint : EndPoint {
  private readonly string path;
  public CacUnixEndPoint(string path) { this.path = path; }
  public override AddressFamily AddressFamily { get { return AddressFamily.Unix; } }
  public override SocketAddress Serialize() {
    byte[] bytes = Encoding.UTF8.GetBytes(path);
    SocketAddress address = new SocketAddress(AddressFamily.Unix, 110);
    for (int i = 0; i < bytes.Length; i++) { address[2 + i] = bytes[i]; }
    return address;
  }
  public override EndPoint Create(SocketAddress address) { return new CacUnixEndPoint(path); }
}
'@
try {
  $socketDir = [System.IO.Path]::GetDirectoryName((ImagePath $socketPath))
  [void][System.IO.Directory]::CreateDirectory($socketDir)
  $endPointType = 'System.Net.Sockets.UnixDomainSocketEndPoint' -as [type]
  if (-not $endPointType) {
    if (-not ('CacUnixEndPoint' -as [type])) { Add-Type -TypeDefinition $endPointSource }
    $endPointType = 'CacUnixEndPoint' -as [type]
  }
  $socketArgs = @([System.Net.Sockets.AddressFamily]::Unix,
    [System.Net.Sockets.SocketType]::Stream, [System.Net.Sockets.ProtocolType]::Unspecified)
  $socket = New-Object System.Net.Sockets.Socket -ArgumentList $socketArgs
  $savedCwd = [Environment]::CurrentDirectory
  try {
    [Environment]::CurrentDirectory = $socketDir
    $socket.Bind((New-Object $endPointType.FullName -ArgumentList 'daemon-updater.sock'))
  } finally { [Environment]::CurrentDirectory = $savedCwd }
} catch {
  if ($socket) { $socket.Close() }
  $socket = $null
  Write-Output "note: no Unix domain socket could be created ($($_.Exception.Message)); socket checks skipped"
}

# 2026-10-29 drift sweep credential gaps (#248)
# Settings and MCP files that can hold MCP env, headers or OAuth values, one
# per secret glob, as path|agent entries relative to alice's home or to C:\proj.
$driftHomeSecrets = @(
  '.claude/settings.json|claude-code'
  '.claude/settings.local.json|claude-code'
  '.gemini/settings.json|gemini-cli'
  '.gemini/config/mcp_config.json|antigravity'
  '.cursor/mcp.json|cursor'
  '.copilot/settings.json|copilot-cli'
  '.copilot/mcp-secrets/index.json|copilot-cli'
  '.config/.copilot/config.json|copilot-cli'
  '.config/.copilot/mcp-config.json|copilot-cli'
  '.config/.copilot/mcp-oauth-config.json|copilot-cli'
  '.config/Code/User/globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json|cline'
  '.config/Code/User/globalStorage/rooveterinaryinc.roo-cline/settings/mcp_settings.json|roo-code'
  '.vscode-mock/global-storage/settings/mcp_settings.json|roo-code'
  '.config/amp/settings.json|amp'
  '.config/amp/settings.jsonc|amp'
  '.factory/settings.json|factory-droid'
  '.factory/settings.local.json|factory-droid'
  '.factory/mcp.json|factory-droid'
  '.aws/amazonq/cli-agents/default.json|kiro'
  '.openhands/conversations/a3/acp/codex/auth.json|openhands'
  '.little-coder/mcp.json|little-coder'
  '.hermes/config.yaml|hermes'
  '.cache/langflow/0f0e0d0c-1111-4000-8000-000000000248/_mcp_servers_0f0e0d0c-1111-4000-8000-000000000248.json|langflow'
  'Library/Caches/langflow/0f0e0d0c-1111-4000-8000-000000000248/_mcp_servers_0f0e0d0c-1111-4000-8000-000000000248.json|langflow'
  'AppData/Local/langflow/langflow/Cache/0f0e0d0c-1111-4000-8000-000000000248/_mcp_servers_0f0e0d0c-1111-4000-8000-000000000248.json|langflow'
  '.config/pydantic-clai2/mcp.json|pydantic-clai'
  '.ollama/id_ed25519|ollama'
  '.ollama/backup/codex/config.toml.1792800000|ollama'
  '.ollama/launch/muse-config/muse/settings.json|ollama'
)
$driftProjectSecrets = @(
  '.claude/settings.json|claude-code'
  '.claude/settings.local.json|claude-code'
  '.mcp.json|project'
  '.gemini/settings.json|gemini-cli'
  '.vscode/mcp.json|vscode'
  '.github/copilot/settings.json|copilot-cli'
  '.github/copilot/settings.local.json|copilot-cli'
  '.github/mcp.json|copilot-cli'
  'opencode.json|opencode'
  '.roo/mcp.json|roo-code'
  '.amazonq/mcp.json|kiro'
  '.amazonq/cli-agents/default.json|kiro'
  '.factory/settings.json|factory-droid'
  '.clai/mcp_servers.json|pydantic-clai'
  '.amp/settings.json|amp'
)
$driftHermesDesktop = @('.config/Hermes-canary', 'Library/Application Support/Hermes-canary', 'AppData/Roaming/Hermes-canary')
$driftHermesCaches = @('Cache', 'Code Cache', 'GPUCache')
$driftCrushFiles = @('crushrc', '.crushrc', 'crush.json', '.crush.json')
foreach ($driftEntry in $driftHomeSecrets) {
  MakeFile "Users/alice/$(($driftEntry -split '\|')[0])" "{`"mcpServers`":{`"x`":{`"env`":{`"TOKEN`":`"t`"}}}}`n"
}
foreach ($driftEntry in $driftProjectSecrets) {
  MakeFile "proj/$(($driftEntry -split '\|')[0])" "{`"mcpServers`":{`"x`":{`"headers`":{`"Authorization`":`"Bearer t`"}}}}`n"
}
foreach ($driftPath in $driftHermesDesktop) {
  MakeFile "Users/alice/$driftPath/connections.json" "{`"connections`":[]}`n"
  foreach ($driftLeaf in $driftHermesCaches) { MakeFile "Users/alice/$driftPath/$driftLeaf/data_0" "cache`n" }
}
$factoryEnvDump = 'Users/alice/.factory/temp/env/droid-env-s1-1.sh'
MakeFile $factoryEnvDump "export TOKEN=t`n"
MakeFile 'Users/alice/.factory/temp/run1/out.txt' "out`n"
MakeFile 'Users/alice/.cache/amp/runner-desktop/r1/state.json' "{}`n"
MakeFile 'Users/alice/.hermes-canary/config.yaml' "mcp_servers: {}`n"
MakeFile 'Users/alice/.hermes-canary/state.db' "sqlite`n"
MakeFile 'Users/alice/.hermes-canary/hermes-agent/hermes_cli/main.py' "x`n"
MakeFile 'Users/alice/.hermes-canary/hermes-agent/.env' "KEY=x`n"
MakeFile 'Users/alice/.hermes-canary/hermes-agent/.git/HEAD' "ref`n"
MakeFile 'Users/alice/AppData/Local/hermes-canary/hermes-agent/hermes_cli/main.py' "x`n"
MakeFile 'Users/alice/AppData/Local/hermes-canary/hermes-agent/.git/HEAD' "ref`n"
MakeFile 'Users/alice/AppData/Local/hermes-canary/hermes-agent/.env' "KEY=x`n"
MakeFile 'Users/alice/.cache/amp/desktop/state.json' "{}`n"
MakeFile 'Users/alice/.cache/amp/bin/amp' "ELF`n"
MakeFile 'Users/alice/.cache/amp/obelisk-runs/run.json' "{}`n"
MakeFile 'Users/alice/AppData/Local/hermes-canary/config.yaml' "mcp_servers: {}`n"
MakeFile 'Users/alice/AppData/Local/hermes-updater/pending/hermes-setup.exe' "MZ`n"
MakeFile 'Users/alice/AppData/Roaming/devin/mcp_config.json' "{`"mcpServers`":{}}`n"
MakeFile 'Users/alice/AppData/Roaming/devin/credentials.toml' "windsurf_api_key = `"k`"`n"
MakeFile 'Users/alice/AppData/Roaming/Devin/Cache/data_0' "cache`n"
MakeFile 'Users/alice/AppData/Roaming/Devin/User/settings.json' "{}`n"
MakeFile 'Users/alice/AppData/Local/devin/bin/devin.exe' "MZ`n"
MakeFile 'Users/alice/Library/Caches/ollama/data_0' "cache`n"
MakeFile 'Users/alice/Library/Caches/com.electron.ollama/data_0' "cache`n"
MakeFile 'Users/alice/AppData/Local/crush/projects.json' '{"projects":[{"path":"C:\\crushwin","data_dir":"C:\\crushwin\\.crush"}]}'
foreach ($driftPath in $driftCrushFiles) { MakeFile "crushwin/$driftPath" "export KEY=k`n" }
$missing = Test-Made
Check "every main-image fixture was created ($($missing.Count) missing$(if ($missing.Count) { ', first ' + $missing[0] }))" { $missing.Count -eq 0 }
Check '#77 deep fixture path is longer than 260 characters' { (ImagePath $DeepTranscript).Length -gt 260 }

# ---- runner ---------------------------------------------------------------
function Run([string]$name, [hashtable]$extra) {
  $outputDir = Join-Path $Out $name
  New-Item -ItemType Directory -Path $outputDir -Force | Out-Null
  $cargs = @{ Root = $Root; OutputDir = $outputDir; Quiet = $true; MaxFileSizeMB = 1 }
  foreach ($k in $extra.Keys) { $cargs[$k] = $extra[$k] }
  $global:LASTEXITCODE = 0
  $script:stdout = & $Collector @cargs 2>&1 | Out-String
  $script:ExitCode = $LASTEXITCODE
  $script:Manifest = Get-ChildItem -LiteralPath $outputDir -Filter '*.manifest.jsonl' | Select-Object -First 1
  $script:Archive = Get-ChildItem -LiteralPath $outputDir | Where-Object { $_.Name -match '\.(tar\.gz|zip)$' } | Select-Object -First 1
  $script:Summary = Get-ChildItem -LiteralPath $outputDir -Filter '*.collection.json' | Select-Object -First 1
  $script:Rows = @()
  if ($script:Manifest) { $script:Rows = @(Get-Content -LiteralPath $script:Manifest.FullName | ForEach-Object { $_ | ConvertFrom-Json }) }
}
function Row([string]$suffix) {
  $wanted = $suffix.Replace('\', '/')
  return ($script:Rows | Where-Object { ($_.path -replace '\\', '/').ToLowerInvariant().EndsWith($wanted.ToLowerInvariant()) } | Select-Object -First 1)
}
function StatusOf([string]$suffix) { $row = Row $suffix; if ($row) { return $row.status } else { return '' } }
function ArchiveEntries {
  if ($script:Archive.Name -like '*.tar.gz') { return @(& tar -tzf $script:Archive.FullName 2>$null) }
  Add-Type -AssemblyName System.IO.Compression.FileSystem
  $zip = [System.IO.Compression.ZipFile]::OpenRead($script:Archive.FullName)
  try { return @($zip.Entries | ForEach-Object { $_.FullName }) } finally { $zip.Dispose() }
}

# Reads a .tar.gz with .NET, so names outside the ANSI code page can be checked
# on Windows too, where tar.exe cannot list them. Returns the members (name,
# type, size, link and, for files, sha256) with pax path, linkpath and size
# records applied, how many headers had a wrong checksum, and whether the
# archive ends with two zero blocks.
function Read-TarGz([string]$file) {
  $fileStream = [System.IO.File]::OpenRead($file)
  $gzip = New-Object System.IO.Compression.GZipStream($fileStream, [System.IO.Compression.CompressionMode]::Decompress)
  $buffer = New-Object System.IO.MemoryStream
  try { $gzip.CopyTo($buffer) } finally { $gzip.Dispose() }
  $tarBytes = $buffer.ToArray()
  $utf8 = New-Object System.Text.UTF8Encoding $false
  $hasher = [System.Security.Cryptography.SHA256]::Create()
  $members = @(); $badSum = 0; $zeros = 0; $position = 0; $pax = @{}
  $readString = { param($offset, $length) $end = $offset; while ($end -lt $offset + $length -and $tarBytes[$end] -ne 0) { $end++ }; $utf8.GetString($tarBytes, $offset, $end - $offset) }
  while ($position + 512 -le $tarBytes.Length) {
    $header = New-Object byte[] 512; [Array]::Copy($tarBytes, $position, $header, 0, 512)
    if ([System.Linq.Enumerable]::Sum([int[]]$header) -eq 0) { $zeros++; $position += 512; if ($zeros -eq 2) { break }; continue }
    $zeros = 0
    $want = [Convert]::ToInt64((& $readString ($position + 148) 8).Trim(), 8)
    for ($i = 148; $i -lt 156; $i++) { $header[$i] = 32 }
    if ([System.Linq.Enumerable]::Sum([int[]]$header) -ne $want) { $badSum++ }
    $type = [string][char]$tarBytes[$position + 156]
    $size = [Convert]::ToInt64((& $readString ($position + 124) 12).Trim(), 8)
    if ($pax.ContainsKey('size')) { $size = [int64]$pax['size'] }
    $data = $position + 512
    if ($type -eq 'x') {
      # Records are "<len> key=value\n", len in bytes and counting itself.
      $cursor = $data
      while ($cursor -lt $data + $size) {
        $space = $cursor; while ($tarBytes[$space] -ne 32) { $space++ }
        $recordLength = [int]$utf8.GetString($tarBytes, $cursor, $space - $cursor)
        $record = $utf8.GetString($tarBytes, $space + 1, $cursor + $recordLength - $space - 2); $equals = $record.IndexOf('=')
        $pax[$record.Substring(0, $equals)] = $record.Substring($equals + 1)
        $cursor += $recordLength
      }
    } else {
      $name = & $readString $position 100; $prefix = & $readString ($position + 345) 155
      if ($prefix) { $name = $prefix + '/' + $name }
      $link = & $readString ($position + 157) 100
      if ($pax.ContainsKey('path')) { $name = $pax['path'] }
      if ($pax.ContainsKey('linkpath')) { $link = $pax['linkpath'] }
      $hash = ''
      if ($type -eq '0') { $hash = ([BitConverter]::ToString($hasher.ComputeHash($tarBytes, $data, [int]$size)) -replace '-', '').ToLower() }
      $members += [pscustomobject]@{ name = $name; type = $type; size = $size; link = $link; sha256 = $hash }
      $pax = @{}
    }
    $position = $data + [int]([Math]::Ceiling($size / 512) * 512)
  }
  return @{ members = $members; badsum = $badSum; end = ($zeros -eq 2) }
}
# A list as one string, sorted ordinally, so names that differ only by case
# compare in a fixed order.
function Join-Sorted($list) {
  $sorted = [string[]]@($list); [Array]::Sort($sorted, [StringComparer]::Ordinal)
  return ($sorted -join "`n")
}
# Every collected row's archived bytes, read from the extracted archive in
# $dir, match its sha256 and size; returns the number of files checked, or -1
# when one does not match.
function Test-ArchivedHashes([string]$dir) {
  $count = 0; $bad = 0
  foreach ($row in ($script:Rows | Where-Object { $_.status -eq 'collected' -and $_.archive_path })) {
    $archivedFile = ExtendedPath (Join-Path $dir ($row.archive_path.Replace('/', $Separator)))
    if (-not (Test-Path -LiteralPath $archivedFile)) { $bad++; continue }
    if ((Get-FileHash -LiteralPath $archivedFile -Algorithm SHA256).Hash.ToLower() -ne $row.sha256) { $bad++ }
    if ((Get-Item -LiteralPath $archivedFile -Force).Length -ne $row.size) { $bad++ }
    $count++
  }
  if ($bad -gt 0) { return -1 }
  return $count
}

# ---- default run ----------------------------------------------------------
Run 'default' @{}
$DefaultEntries = @(ArchiveEntries)
$defaultRunSummary = Get-Content -LiteralPath $script:Summary.FullName -Raw | ConvertFrom-Json
Check 'exit code 0' { $script:ExitCode -eq 0 }
Check 'archive exists' { $null -ne $script:Archive -and $script:Archive.Length -gt 0 }
Check 'sha256 sidecar matches' { (Get-Content -LiteralPath "$($script:Archive.FullName).sha256").Split(' ')[0] -eq (Get-FileHash -LiteralPath $script:Archive.FullName -Algorithm SHA256).Hash.ToLower() }
Check 'transcript collected' { (StatusOf 'Users/alice/.claude/projects/-C-proj/s1.jsonl') -eq 'collected' }
$row = Row 'Users/alice/.claude/projects/-C-proj/s1.jsonl'
Check 'transcript hash correct' { $row -and $row.sha256 -eq (Get-FileHash -LiteralPath (ImagePath 'Users/alice/.claude/projects/-C-proj/s1.jsonl') -Algorithm SHA256).Hash.ToLower() }
Check 'transcript has owner and attributes fields' { $row -and $null -ne $row.owner -and $row.attributes -ne '' }
$row = Row 'Users/alice/.claude/.credentials.json'
Check 'secret collected and flagged' { $row -and $row.secret -eq $true -and $row.status -eq 'collected' }
Check 'antigravity token flagged secret' { (Row 'antigravity-cli/antigravity-oauth-token').secret -eq $true }
$row = Row 'Users/alice/.gemini/antigravity-cli/conversations/c1.db'
Check 'nested entry attributed to its own agent' { $row -and $row.agent -eq 'antigravity' }
Check 'nested entry collected exactly once' { @($script:Rows | Where-Object { $_.path -like '*antigravity-cli*conversations*c1.db' }).Count -eq 1 }
$row = Row 'Users/alice/.gemini/projects.json'
Check 'enclosing entry still attributed to gemini-cli' { $row -and $row.agent -eq 'gemini-cli' }
$row = Row 'Users/alice/AppData/Roaming/Code/User/globalStorage/saoudrizwan.claude-dev/state/taskHistory.json'
Check 'cline globalStorage attributed to cline, not vscode' { $row -and $row.agent -eq 'cline' }
Check 'cline globalStorage collected exactly once' { @($script:Rows | Where-Object { $_.path -like '*Roaming*Code*User*globalStorage*saoudrizwan.claude-dev*taskHistory.json' }).Count -eq 1 }
$row = Row 'Users/alice/.claude/cache'
Check 'cache dir skipped_excluded with size' { $row -and $row.type -eq 'dir' -and $row.size -gt 0 -and $row.status -eq 'skipped_excluded' }
Check 'cache contents not collected' { $null -eq (Row 'cache/junk.bin') }
Check 'antigravity bin excluded' { (StatusOf 'antigravity-cli/bin') -eq 'skipped_excluded' }
Check 'codex packages excluded' { (StatusOf 'Users/alice/.codex/packages') -eq 'skipped_excluded' }
Check 'ollama model blobs excluded' { (StatusOf 'home/bob/.ollama/models/blobs') -eq 'skipped_excluded' }
Check 'ollama history collected (linux-style home in same image)' { (StatusOf 'home/bob/.ollama/history') -eq 'collected' }
Check 'bash history not collected' { -not ($script:Rows | Where-Object { $_.path -like '*.bash_history' }) }
Check 'PSReadLine history not collected' { -not ($script:Rows | Where-Object { $_.path -like '*ConsoleHost_history.txt' }) }
Check 'oversized file skipped_size' { (StatusOf 'Cursor/User/globalStorage/state.vscdb') -eq 'skipped_size' }
Check 'ms-python globalStorage excluded' { (StatusOf 'globalStorage/ms-python.python') -eq 'skipped_excluded' }
Check 'cline checkpoints excluded' { (StatusOf 'saoudrizwan.claude-dev/checkpoints') -eq 'skipped_excluded' }
Check 'weird filename collected' { (StatusOf 'weird name with spaces & [brackets].txt') -eq 'collected' }
if ($linkOk) {
  $row = Row 'antigravity-cli/cli.log'
  Check 'symlink recorded with target' { $row -and $row.type -eq 'symlink' -and $row.status -eq 'symlink' -and $row.target -like '*cli-1.log' }
}
Check 'project from history.jsonl (drive path mapped into image) collected' { (StatusOf 'proj/CLAUDE.md') -eq 'collected' }
Check 'project .claude settings collected' { (StatusOf 'proj/.claude/settings.local.json') -eq 'collected' }
Check 'project owner attributed' { (Row 'proj/CLAUDE.md').user -eq 'alice' }
# 2026-10-14 project files attributed to their agent (#104)
$row = Row 'proj/.claude/settings.local.json'
Check 'project .claude file attributed to claude-code' { $row -and ($row.home -replace '\\', '/').EndsWith('/proj') -and $row.agent -eq 'claude-code' -and $row.status -eq 'collected' }
$row = Row 'droid/AGENTS.md'
Check 'shared project AGENTS.md keeps agent project' { $row -and ($row.home -replace '\\', '/').EndsWith('/droid') -and $row.agent -eq 'project' -and $row.status -eq 'collected' }
Check 'project from workspace.json file URI collected' { (StatusOf 'Documents/app/.cursorrules') -eq 'collected' }
Check 'project from codex rollout cwd collected' { (StatusOf 'Documents/app/AGENTS.md') -eq 'collected' }
Check 'project from cline taskHistory collected' { (StatusOf 'cl/.clinerules') -eq 'collected' }
Check 'project from continue sessions.json collected' { (StatusOf 'cont/.continuerc.json') -eq 'collected' }
Check 'project from gemini projects.json collected' { (StatusOf 'gem/GEMINI.md') -eq 'collected' }
Check 'project-local crush.db from projects.json collected' { (StatusOf 'crushproj/.crush/crush.db') -eq 'collected' }
Check 'continue globalContext flagged secret' { (Row 'index/globalContext.json').secret -eq $true }
Check 'continue lancedb excluded' { (StatusOf 'index/lancedb') -eq 'skipped_excluded' }
Check 'cline providers.json flagged secret' { (Row 'settings/providers.json').secret -eq $true }
Check 'codex auth.json flagged secret' { (Row 'Users/alice/.codex/auth.json').secret -eq $true }
Check 'home .env not collected' { -not (Row 'Users/alice/.env') }
Check 'claude desktop config.json flagged secret' { (Row 'Roaming/Claude/config.json').secret -eq $true }
$row = Row 'Claude/claude_desktop_config.json'
Check 'factory session index not swallowed by cache exclusion' { (StatusOf 'Users/alice/.factory/cache/session-index/index.db') -eq 'collected' }
Check 'factory cache sounds excluded' { (StatusOf 'Users/alice/.factory/cache/sounds') -eq 'skipped_excluded' }
Check 'factory session cwd discovers project' { (StatusOf 'droid/AGENTS.md') -eq 'collected' }
$kiroRow = Row 'Users/alice/.kiro/powers/installed/p1/mcp.json'
Check 'kiro power mcp.json collected and flagged' { $kiroRow -and $kiroRow.secret -eq $true -and $kiroRow.status -eq 'collected' }
Check 'kiro cli session cwd discovers project' { (StatusOf 'kiroproj/.kiroignore') -eq 'collected' }
Check 'vscode-server User dir collected as vscode' { (Row 'Users/alice/.vscode-server/data/User/globalStorage/state.vscdb').agent -eq 'vscode' }
Check 'cline inside vscode-server attributed to cline' { (Row 'Users/alice/.vscode-server/data/User/globalStorage/saoudrizwan.claude-dev/state/taskHistory.json').agent -eq 'cline' }
Check 'vscode-server taskHistory discovers project' { (StatusOf 'remoteproj/.clinerules') -eq 'collected' }
Check 'antigravity-ide attributed to antigravity' { (Row 'Users/alice/.gemini/antigravity-ide/conversations/i1.db').agent -eq 'antigravity' }
Check 'antigravity history workspace discovers project' { (StatusOf 'agyproj/.antigravityignore') -eq 'collected' }
Check 'cursor auth.json flagged secret' { (Row 'Users/alice/.cursor/auth.json').secret -eq $true }
Check 'copilot pkg excluded' { (StatusOf 'Users/alice/.copilot/pkg') -eq 'skipped_excluded' }
Check 'copilot config.json flagged secret' { (Row 'Users/alice/.copilot/config.json').secret -eq $true }
Check 'copilot workspace.yaml cwd discovers project' { (StatusOf 'copproj/AGENTS.md') -eq 'collected' }
Check 'claude device keys flagged secret' { (Row 'Users/alice/.claude/.device-keys.json').secret -eq $true }
Check 'amp history cwd discovers project' { (StatusOf 'ampproj/.amp/settings.json') -eq 'collected' }
Check 'project-level kilo worktrees excluded' { (StatusOf 'kiloproj/.kilo/worktrees') -eq 'skipped_excluded' }
Check 'claude desktop mcp config flagged secret' { $row -and $row.secret -eq $true -and $row.status -eq 'collected' }
Check 'claude desktop Cache excluded' { (StatusOf 'Roaming/Claude/Cache') -eq 'skipped_excluded' }
# 2026-10-04 catalog cases
foreach ($caseLine in $CasesCatalog) {
  $fields = @($caseLine -split '\|', 3); $row = Row "Users/alice/$($fields[2])"
  switch ($fields[0]) {
    'excl' { Check "excluded: $($fields[2])" { $row -and $row.status -eq 'skipped_excluded' } }
    'exclf' { Check "excluded: $($fields[2])" { $row -and $row.status -eq 'skipped_excluded' } }
    'secret' { Check "secret: $($fields[2])" { $row -and $row.secret -eq $true -and $row.status -eq 'collected' } }
    'agent' { Check "$($fields[1]): $($fields[2])" { $row -and $row.agent -eq $fields[1] -and $row.status -eq 'collected' } }
  }
}
# On a case-insensitive filesystem .config/PearAI/User matches the same directory and is collected once
$pearaiRows = @($script:Rows | Where-Object { ($_.path -replace '\\', '/') -like '*/.config/pearai/User/globalStorage/state.vscdb' })
Check 'pearai lowercase .config/pearai collected as pearai' { $pearaiRows.Count -eq 1 -and @($pearaiRows | Where-Object { $_.agent -ne 'pearai' }).Count -eq 0 }
Check 'cody nested globalStorage collected exactly once' { @($script:Rows | Where-Object { ($_.path -replace '\\', '/') -like '*sourcegraph.cody-ai/symf/indexroot/meta.json' }).Count -eq 1 }
Check 'twinny nested globalStorage collected exactly once' { @($script:Rows | Where-Object { ($_.path -replace '\\', '/') -like '*globalStorage/rjmacarthy.twinny/twinny-providers.json' }).Count -eq 1 }
Check 'little-coder history nested in .pi collected exactly once' { @($script:Rows | Where-Object { $_.path -like '*little-coder-prompt-history.json' }).Count -eq 1 }
Check 'hermes installer checkout excluded but its .env kept' { (StatusOf 'Users/alice/.hermes/hermes-agent/hermes_cli') -eq 'skipped_excluded' -and (StatusOf 'Users/alice/.hermes/hermes-agent/.env') -eq 'collected' }
if ($clawLinkOk) {
  $row = Row 'home/bob/.clawdbot'
  Check 'openclaw .clawdbot rename symlink recorded' { $row -and $row.agent -eq 'openclaw' -and $row.type -eq 'symlink' -and $row.status -eq 'symlink' -and $row.target -like '*.openclaw' }
}
Check 'hermes checkpoint workdir discovers project' { (StatusOf 'hermesproj/.hermes.md') -eq 'collected' -and (Row 'hermesproj/.hermes.md').user -eq 'alice' }
# 2026-10-30 hermes rebuild (#326)
foreach ($hermesFile in $hermesProjectFiles) {
  $row = Row "hermesproj/$hermesFile"
  Check "#326 Hermes project file collected: $hermesFile" { $row -and $row.agent -eq 'hermes' -and $row.status -eq 'collected' }
}
Check 'letta sessions.jsonl project discovers project' { (StatusOf 'lettaproj/.letta/settings.json') -eq 'collected' }
Check 'project .letta/settings.json flagged secret (accepted overmatch)' { (Row 'lettaproj/.letta/settings.json').secret -eq $true }
Check 'letta local backend messages cwd discovers project' { (StatusOf 'lettaproj2/AGENTS.md') -eq 'collected' }
Check 'pi session cwd discovers project' { (StatusOf 'piproj/.pi/settings.json') -eq 'collected' }
Check 'pi misplaced session cwd discovers project' { (StatusOf 'piproj2/AGENTS.md') -eq 'collected' }
Check 'pi trust.json keys discover project' { (StatusOf 'pitrust/pi-session-2026-10-04.html') -eq 'collected' }
Check 'pi crashes.json cwd discovers project' { (StatusOf 'picrash/AGENTS.md') -eq 'collected' }
Check 'pi experimental meta cwd discovers project' { (StatusOf 'piexp/AGENTS.md') -eq 'collected' }
Check 'open-interpreter rollout cwd discovers project' { (StatusOf 'oiproj/.openinterpreter/rules/r.md') -eq 'collected' }
Check 'open-interpreter archived rollout cwd discovers project' { (StatusOf 'oiarch/.codewhale/notes.md') -eq 'collected' }
Check 'openhands workspaces.json path discovers project' { (StatusOf 'ohproj/.openhands/setup.sh') -eq 'collected' }
Check 'openhands canvas dev conversation discovers project' { (StatusOf 'ohdev/AGENTS.md') -eq 'collected' }
Check 'openhands canvas conversation discovers project' { (StatusOf 'ohcanvas/AGENTS.md') -eq 'collected' }
Check 'openhands cli base_state discovers project' { (StatusOf 'ohcli/AGENTS.md') -eq 'collected' }
Check 'pearai sessions.json discovers project' { (StatusOf 'pearproj/.pearairc.json') -eq 'collected' }
$row = Row 'pearproj/.pearai-agent/mcp.json'
Check 'project .pearai-agent/mcp.json flagged secret' { $row -and $row.secret -eq $true -and $row.status -eq 'collected' }
Check 'openclaw JSON5 workspace discovers project' { (StatusOf 'clawws/SOUL.md') -eq 'collected' }
Check 'openclaw JSON5 agentDir discovers project' { (StatusOf 'clawagent/AGENTS.md') -eq 'collected' }
Check 'openclaw legacy clawdbot.json discovers project' { (StatusOf 'clawlegacy/IDENTITY.md') -eq 'collected' }
Check 'openclaw profile config discovers project' { (StatusOf 'clawprofile/MEMORY.md') -eq 'collected' }
Check 'nanobot .workspace marker discovers project' { (StatusOf 'nanows/HEARTBEAT.md') -eq 'collected' }
Check 'nanobot instance config workspace discovers project' { (StatusOf 'nanocfg/memory/MEMORY.md') -eq 'collected' }
Check 'tabby config.toml file:// git_url (drive) discovers project' { (StatusOf 'tabbyproj/AGENTS.md') -eq 'collected' }
Check 'tabby config.toml file:// git_url (unix) discovers project' { (StatusOf 'srv/tabbyunix/AGENTS.md') -eq 'collected' }
Check 'twinny manifest common prefix discovers project' { (StatusOf 'twinproj/AGENTS.md') -eq 'collected' }
# 2026-10-04 Docker volumes
$row = Row 'var/lib/docker/volumes/a0_usr/_data/chats/ctx1/chat.json'
Check 'docker volume file collected as agent-zero, user docker' { $row -and $row.user -eq 'docker' -and $row.agent -eq 'agent-zero' -and $row.status -eq 'collected' -and ($row.home -replace '\\', '/').EndsWith('var/lib/docker/volumes/a0_usr/_data') }
$row = Row 'var/lib/docker/volumes/a0_usr/_data/.env'
Check 'docker volume .env flagged secret (volume-relative)' { $row -and $row.secret -eq $true -and $row.status -eq 'collected' }
Check 'docker volume tmp/playwright excluded (volume-relative)' { (StatusOf 'var/lib/docker/volumes/a0_usr/_data/tmp/playwright') -eq 'skipped_excluded' }
$row = Row 'var/lib/docker/volumes/pgdata/_data'
Check 'unmatched volume recorded skipped_unmatched_volume with size' { $row -and $row.user -eq 'docker' -and $row.type -eq 'dir' -and $row.size -gt 0 -and $row.status -eq 'skipped_unmatched_volume' }
Check 'unmatched volume contents not collected' { $null -eq (Row 'pgdata/_data/x') }
Check 'volumes dir metadata.db not collected' { $null -eq (Row 'volumes/metadata.db') }
$row = Row 'Users/alice/.local/share/docker/volumes/tabby_data/_data/events/2026-10-01.json'
Check 'rootless volume collected as tabby' { $row -and $row.user -eq 'docker' -and $row.agent -eq 'tabby' -and $row.status -eq 'collected' }
# 2026-10-04 Muse Code
foreach ($caseLine in $CasesMuse) {
  $fields = @($caseLine -split '\|', 3); $row = Row "Users/alice/$($fields[2])"
  switch ($fields[0]) {
    'excl' { Check "excluded: $($fields[2])" { $row -and $row.agent -eq 'muse-code' -and $row.status -eq 'skipped_excluded' } }
    'exclf' { Check "excluded: $($fields[2])" { $row -and $row.agent -eq 'muse-code' -and $row.status -eq 'skipped_excluded' } }
    'secret' { Check "secret: $($fields[2])" { $row -and $row.agent -eq 'muse-code' -and $row.secret -eq $true -and $row.status -eq 'collected' } }
    'agent' { Check "$($fields[1]): $($fields[2])" { $row -and $row.agent -eq $fields[1] -and $row.status -eq 'collected' } }
  }
}
$row = Row 'alice/AppData/Local/Programs/muse/muse-bin-1.4.2-R1.exe'
Check 'muse windows binary excluded with its size' { $row -and $row.type -eq 'file' -and $row.size -eq 1 -and $row.status -eq 'skipped_excluded' }
Check 'muse .local/bin glob leaves other tools alone' { $null -eq (Row 'alice/.local/bin/othertool') }
Check 'muse trust.json projects key discovers project' { (StatusOf 'musetrust/AGENTS.md') -eq 'collected' -and (Row 'musetrust/AGENTS.md').user -eq 'alice' }
$row = Row 'museproj/.muse/hooks.json'
Check 'muse session.jsonl workspace_root discovers project' { $row -and $row.user -eq 'alice' -and $row.agent -eq 'muse-code' -and $row.status -eq 'collected' }
Check 'muse project .muse/worktrees excluded' { (StatusOf 'museproj/.muse/worktrees') -eq 'skipped_excluded' }
# 2026-10-04 relocated Claude Code home
$row = Row $ClaudeWorkSession
Check 'relocated claude home session collected as claude-code' { $row -and $row.agent -eq 'claude-code' -and $row.status -eq 'collected' }
$row = Row 'alice/.claude-work/history.jsonl'
Check 'relocated claude home history collected as claude-code' { $row -and $row.agent -eq 'claude-code' -and $row.status -eq 'collected' }
$row = Row 'alice/.claude-work/.credentials.json'
Check 'relocated claude home credentials flagged secret' { $row -and $row.agent -eq 'claude-code' -and $row.secret -eq $true -and $row.status -eq 'collected' }
$row = Row 'alice/.claude-work/.claude.json'
Check 'relocated claude home .claude.json flagged secret' { $row -and $row.agent -eq 'claude-code' -and $row.secret -eq $true -and $row.status -eq 'collected' }
# 2026-10-23 (#256)
$row = Row $ClaudeWorkKey; $daemonKeyRow = Row 'alice/.claude-work/daemon/control.key'; $rosterRow = Row 'alice/.claude-work/daemon/roster.json'; $backupRow = Row 'alice/.claude-work/backups/.claude.json.backup.1792800000000'
Check '#256 relocated claude home session key, daemon files and backup flagged secret' { $row -and $daemonKeyRow -and $rosterRow -and $backupRow -and @(@($row, $daemonKeyRow, $rosterRow, $backupRow) | Where-Object { $_.agent -ne 'claude-code' -or $_.secret -ne $true -or $_.status -ne 'collected' }).Count -eq 0 }
Check 'claude-code-router not collected' { $null -eq (Row 'alice/.claude-code-router/config.json') }
$row = Row 'ccwork/.claude/settings.json'
Check 'relocated .claude.json projects key discovers project' { $row -and $row.user -eq 'alice' -and $row.agent -eq 'claude-code' -and $row.status -eq 'collected' }
$row = Row 'cchist/.claude/settings.json'
Check 'relocated history.jsonl discovers project' { $row -and $row.user -eq 'alice' -and $row.agent -eq 'claude-code' -and $row.status -eq 'collected' }
Check 'Public profile skipped' { $null -eq (Row 'Public/Desktop/readme.txt') }
Check 'Default profile skipped' { $null -eq (Row 'Default/NTUSER.DAT') }
$entries = ArchiveEntries
Check 'no live snapshot directory' { -not ($entries | Where-Object { $_ -match '(^|/)live/' }) }
Check 'manifest and summary inside archive' { ($entries | Where-Object { $_ -match 'manifest\.jsonl$' }) -and ($entries | Where-Object { $_ -match 'collection\.json$' }) }
Check 'archive layout is fs/<path relative to root>' { @($entries | Where-Object { $_ -match '(^|\./)fs/Users/alice/\.claude/history\.jsonl$' }).Count -eq 1 }
Check 'staging removed' { -not (Get-ChildItem -LiteralPath (Join-Path $Out 'default') -Filter '.stage-*' -Force) }
$sum = Get-Content -LiteralPath $script:Summary.FullName -Raw | ConvertFrom-Json
Check 'summary is valid JSON with zero copy errors' { $sum.counts.error_copy -eq 0 }
Check 'summary has no no_live option' { $null -eq $sum.options.PSObject.Properties['no_live'] }
Check 'summary counts match manifest' { $sum.counts.collected -eq @($script:Rows | Where-Object { $_.status -eq 'collected' }).Count }
Check 'manifest rows all parsed as JSON' { $script:Rows.Count -eq @(Get-Content -LiteralPath $script:Manifest.FullName).Count }
Check 'archive layout for a docker volume file is fs/<path relative to root>' { @($entries | Where-Object { $_ -match '(^|\./)fs/var/lib/docker/volumes/a0_usr/_data/chats/ctx1/chat\.json$' }).Count -eq 1 }
Check 'summary counts skipped_unmatched_volume' { $sum.counts.skipped_unmatched_volume -eq 1 }
Check 'summary docker counts' { $sum.docker.volumes_found -eq 3 -and $sum.docker.volumes_collected -eq 2 -and $sum.docker.unreadable -eq 0 -and $sum.docker.docker_desktop -eq $false }
Check 'summary no_docker option false and notes empty' { $sum.options.no_docker -eq $false -and @($sum.notes).Count -eq 0 }
Check 'stdout docker line' { $script:stdout -match '(?m)^docker:     3 volumes found, 2 collected, 0 unreadable\s*$' }
Check 'profile without artifacts still listed in users' { @($sum.users) -contains 'dave' }
Check 'profile without artifacts not in users_with_artifacts' { @($sum.users_with_artifacts).Count -gt 0 -and -not (@($sum.users_with_artifacts) -contains 'dave') }
Check 'stdout users counts users_with_artifacts' { $script:stdout -match "(?m)^users:      $(@($sum.users_with_artifacts).Count)\s*$" }
Check 'log has no progress line for a profile without artifacts' { $logText = Get-Content -LiteralPath (Get-ChildItem -LiteralPath (Join-Path $Out 'default') -Filter '*.log' | Select-Object -First 1).FullName -Raw; $logText -notmatch 'User dave' -and $logText -match 'User alice' }
Check 'stdout size is human-readable' { $script:stdout -match '(?m)^size:       \d+ \([0-9.]+ (B|KiB|MiB)\)\s*$' }
Check 'stdout collected bytes are human-readable' { $script:stdout -match '(?m)^collected:  \d+ files, \d+ bytes \([0-9.]+ (B|KiB|MiB)\)\s*$' }

# 2026-10-04 bug round: #29
function CountOf([string]$name) { return @($script:Rows | Where-Object { ($_.path -replace '\\', '/') -like "*/$name" }).Count }
function HasPath([string]$relativePath) { $wanted = (ImagePath $relativePath); return @($script:Rows | Where-Object { $_.path -ceq $wanted -and $_.status -eq 'collected' }).Count -eq 1 }
$defaultLog = Get-Content -LiteralPath (Get-ChildItem -LiteralPath (Join-Path $Out 'default') -Filter '*.log' | Select-Object -First 1).FullName -Raw
if ($caseInsensitive) {
  Write-Output "note: $Work is case-insensitive; checking the folded layout"
  Check '#29 case variants of one directory collected once' { (CountOf 'case29-upper.json') -eq 1 -and (CountOf 'case29-lower.json') -eq 1 }
  # PowerShell records the on-disk spelling (the catalog cases block above created .config/pearai first)
  Check '#29 path recorded in its on-disk spelling' { (HasPath 'Users/alice/.config/pearai/User/case29-upper.json') -and (HasPath 'Users/alice/.config/pearai/User/case29-lower.json') }
  Check '#29 nested case variant collected once under the nested entry' { (CountOf 'case29-editor.json') -eq 1 -and (HasPath 'Users/alice/.config/cursor/User/case29-editor.json') }
  # 5.1 wraps long log lines written through a redirected console, so whitespace is folded first
  Check '#29 dropped variant logged' { ($defaultLog -replace '\s+', ' ') -match '\[pearai\] \S*\.config[\\/]pearai[\\/]User: catalog line \.config/pearai/User names the same path as an earlier line \(case-insensitive filesystem\)' }
} else {
  Check '#29 PearAI and pearai are distinct here and both collected' { (HasPath 'Users/alice/.config/PearAI/User/case29-upper.json') -and (HasPath 'Users/alice/.config/pearai/User/case29-lower.json') -and (CountOf 'case29-upper.json') -eq 1 -and (CountOf 'case29-lower.json') -eq 1 }
  Check '#29 nested Cursor/User distinct from cursor and both collected' { (HasPath 'Users/alice/.config/cursor/case29-cli.json') -and (HasPath 'Users/alice/.config/Cursor/User/case29-editor.json') -and (CountOf 'case29-editor.json') -eq 1 -and (Row 'alice/.config/Cursor/User/case29-editor.json').agent -eq 'cursor' }
  Check '#29 nothing folded on a case-sensitive filesystem' { $defaultLog -notmatch 'case-insensitive filesystem' }
}

# 2026-10-06 Ollama (#44)
$row = Row $ollamaManifest
Check 'ollama model manifest collected' { $row -and $row.agent -eq 'ollama' -and $row.status -eq 'collected' }
Check 'ollama blob not collected' { $null -eq (Row 'blobs/sha256-abc') }

# 2026-10-06 frameworks sweep (#64)
foreach ($caseLine in $CasesFrameworks) {
  $fields = @($caseLine -split '\|', 3); $row = Row "Users/alice/$($fields[2])"
  switch ($fields[0]) {
    'excl' { Check "excluded: $($fields[2])" { $row -and $row.status -eq 'skipped_excluded' } }
    'exclf' { Check "excluded: $($fields[2])" { $row -and $row.status -eq 'skipped_excluded' } }
    'secret' { Check "secret: $($fields[2])" { $row -and $row.secret -eq $true -and $row.status -eq 'collected' } }
    'agent' { Check "$($fields[1]): $($fields[2])" { $row -and $row.agent -eq $fields[1] -and $row.status -eq 'collected' } }
  }
}
Check 'crewai mid-path wildcard leaves other files in the project dir alone' { $null -eq (Row 'alice/.local/share/myproj/unrelated.txt') }
$row = Row 'home/bob/AppData/Local/crewai/myproj/latest_kickoff_task_outputs.db'
Check 'crewai lowercase AppData/Local/crewai collected through the class' { $row -and $row.agent -eq 'crewai' -and $row.status -eq 'collected' }
$row = Row 'home/bob/AppData/Local/crewai/credentials/tokens.enc'
Check 'crewai lowercase credentials flagged secret through the class' { $row -and $row.secret -eq $true -and $row.status -eq 'collected' }
Check 'camel-ai narrow lines leave Apache Camel JBang files alone' { $null -eq (Row 'alice/.camel/4242-status.json') }
Check 'legacy open-interpreter space-named dir collected once' { (CountOf 'Open Interpreter Terminal/conversations/old.json') -eq 1 }

# 2026-10-11 Claude Desktop Linux deb and Claude Code 2.1.295 session keys (#73, #76)
function IsAgentRow([string]$relativePath, [string]$agent, [bool]$secret) { $row = Row $relativePath; return ($row -and $row.agent -eq $agent -and $row.secret -eq $secret -and $row.status -eq 'collected') }
Check '#73 Linux spaces-present link-key flagged secret' { IsAgentRow $linkKey 'claude-desktop' $true }
Check '#76 Claude Code sessions/<pid>.<hash>.key flagged secret' { IsAgentRow $sessionKey 'claude-code' $true }
Check '#76 Claude Code sessions/<pid>.json collected, not secret' { IsAgentRow 'Users/alice/.claude/sessions/4242.json' 'claude-code' $false }
Check '#73 Linux git-shadow/cli-temp-roots.json collected as claude-desktop' { IsAgentRow "$claudeLinuxConfig/git-shadow/cli-temp-roots.json" 'claude-desktop' $false }
Check '#73 Linux logs/main.log inside the data dir collected as claude-desktop' { IsAgentRow "$claudeLinuxConfig/logs/main.log" 'claude-desktop' $false }
Check '#73 Linux Code-tab session record collected as claude-desktop' { IsAgentRow $codeTabRecord 'claude-desktop' $false }
Check '#73 Linux claude-code excluded, downloaded CLI not in the manifest' { (StatusOf "$claudeLinuxConfig/claude-code") -eq 'skipped_excluded' -and @($script:Rows | Where-Object { ($_.path -replace '\\', '/') -like '*/9b32b47ec4b3/*' }).Count -eq 0 }

# 2026-10-11 Claude Code CLI cache directory (#81)
$row = Row $cliCacheLog
Check '#81 Claude Code CLI cache MCP log collected as claude-code' { $row -and $row.agent -eq 'claude-code' -and $row.status -eq 'collected' }
Check '#81 Claude Code CLI cache MCP log not secret' { $row -and $row.secret -eq $false -and $row.status -eq 'collected' }
$row = Row $cliCacheLogWindows
Check '#81 Claude Code CLI cache Windows Cache child collected as claude-code' { $row -and $row.agent -eq 'claude-code' -and $row.status -eq 'collected' }

# 2026-10-13 Antigravity desktop app (#83, #84)
function IsAntigravityRow([string]$relativePath, [bool]$secret) { $row = Row $relativePath; return ($row -and $row.agent -eq 'antigravity' -and $row.secret -eq $secret -and $row.status -eq 'collected') }
function HasNoRow([string]$tail) { return (@($script:Rows | Where-Object { ($_.path -replace '\\', '/') -like "*/$tail" }).Count -eq 0) }
Check '#83 Antigravity app conversation collected as antigravity' { IsAntigravityRow "$antigravityData/conversations/a1.db" $false }
Check '#83 Antigravity app crashes/ collected' { (StatusOf "$antigravityData/crashes/crash_1.log") -eq 'collected' }
Check '#84 Antigravity plugin token.json flagged secret' { IsAntigravityRow "$antigravityData/plugin_data/gdocs/token.json" $true }
Check '#83 Antigravity plugin binary excluded' { (StatusOf "$antigravityData/plugin_data/gdocs/bin") -eq 'skipped_excluded' -and (HasNoRow 'gdocs/bin/gdocs-1') }
Check '#83 Antigravity app bin excluded' { (StatusOf "$antigravityData/bin") -eq 'skipped_excluded' -and (HasNoRow '.gemini/antigravity/bin/webm_encoder') }
Check '#83 Antigravity marketplace cache excluded, other cache files collected' { (StatusOf "$antigravityData/cache/marketplaces") -eq 'skipped_excluded' -and (StatusOf "$antigravityData/cache/last_conversations.json") -eq 'collected' }
Check '#83 Antigravity Electron app_storage.json collected as antigravity' { IsAntigravityRow "$antigravityElectron/app_storage.json" $false }
Check '#83 Antigravity Electron Cache and Code Cache excluded' { (StatusOf "$antigravityElectron/Cache") -eq 'skipped_excluded' -and (StatusOf "$antigravityElectron/Code Cache") -eq 'skipped_excluded' -and (HasNoRow 'Application Support/Antigravity/Cache/data_0') -and (HasNoRow 'Antigravity/Code Cache/js/x') }
Check '#83 Antigravity Library/Logs main.log collected as antigravity' { IsAntigravityRow 'Users/alice/Library/Logs/Antigravity/main.log' $false }
Check '#83 Antigravity Windows app_storage.json collected as antigravity' { IsAntigravityRow "$antigravityWindows/app_storage.json" $false }
Check '#83 Antigravity Windows Electron Cache excluded' { (StatusOf "$antigravityWindows/Cache") -eq 'skipped_excluded' -and (HasNoRow 'Roaming/Antigravity/Cache/data_0') }
Check '#83 Antigravity data dir cache/ not pruned by the Electron Cache exclusion' { (StatusOf "$antigravityData/cache/last_conversations.json") -eq 'collected' -and (StatusOf "$antigravityData/cache") -ne 'skipped_excluded' }

# 2026-10-13 Antigravity Linux round (#83, #90, #91)
Check '#90 Antigravity CLI plugin-secrets.json flagged secret' { IsAntigravityRow "$antigravityWindows/plugin-secrets.json" $true }
Check '#90 Antigravity CLI plugin-secrets.json collected once' { (CountOf 'plugin-secrets.json') -eq 1 }
Check '#83 Antigravity Electron Dictionaries excluded' { (StatusOf "$antigravityWindows/Dictionaries") -eq 'skipped_excluded' -and (HasNoRow 'Dictionaries/en-US-10-1.bdic') }
Check '#83 Antigravity app sidecar SDK under lib/node_modules excluded' { (StatusOf "$antigravityData/lib/node_modules") -eq 'skipped_excluded' -and (HasNoRow 'antigravity/lib/node_modules/sidecar_sdk/index.mjs') }
Check '#83 Antigravity config/node_modules excluded' { (StatusOf 'Users/alice/.gemini/config/node_modules') -eq 'skipped_excluded' -and (HasNoRow 'config/node_modules/sidecar_sdk/index.mjs') }
Check '#91 Antigravity project folderUri discovers project' { (StatusOf 'agyapp/.antigravityignore') -eq 'collected' }
Check '#91 Antigravity drive-letter folderUri discovers project' { (StatusOf 'agywin/.antigravityignore') -eq 'collected' }
# 2026-10-14 catalog match that is an excluded file (#107)
$museUpdaterRow = Row 'Users/alice/.local/bin/.muse-update.107.exe'
Check '#107 excluded file match gets a skipped_excluded row with its size' { $museUpdaterRow -and $museUpdaterRow.user -eq 'alice' -and $museUpdaterRow.home -eq (ImagePath 'Users/alice') -and $museUpdaterRow.agent -eq 'muse-code' -and $museUpdaterRow.archive_path -eq '' -and $museUpdaterRow.type -eq 'file' -and $museUpdaterRow.size -eq 5 -and $museUpdaterRow.sha256 -eq '' -and $museUpdaterRow.secret -eq $false -and $museUpdaterRow.status -eq 'skipped_excluded' -and $museUpdaterRow.target -eq '' -and $museUpdaterRow.error -eq '' }
Check '#107 excluded file match not archived' { -not ($DefaultEntries | Where-Object { $_ -like '*muse-update.107.exe' }) }
# 2026-10-14 Antigravity on Windows (#83)
Check '#83 Antigravity Windows staging file collected as antigravity' { IsAntigravityRow "$aliceLocalAppData/antigravity/staging/s1" $false }
Check '#83 Antigravity installer and program directory not collected' { (HasNoRow 'antigravity-updater/installer.exe') -and (HasNoRow 'Programs/antigravity/Antigravity.exe') }
# 2026-10-15 ChatGPT desktop as the OpenAI.Codex package (#113)
Check '#113 ChatGPT profile Local Storage collected as chatgpt-desktop' { IsAgentRow "$chatgptProfile/Default/Local Storage/leveldb/000003.log" 'chatgpt-desktop' $false }
Check '#113 ChatGPT Local State flagged secret' { IsAgentRow "$chatgptProfile/Local State" 'chatgpt-desktop' $true }
Check '#113 ChatGPT profile and partition cookies flagged secret' { (IsAgentRow "$chatgptProfile/Default/Network/Cookies" 'chatgpt-desktop' $true) -and (IsAgentRow "$chatgptPartition/Network/Cookies" 'chatgpt-desktop' $true) }
Check '#113 ChatGPT Login Data flagged secret' { IsAgentRow "$chatgptProfile/Default/Login Data" 'chatgpt-desktop' $true }
Check '#113 ChatGPT profile and partition Cache excluded' { (StatusOf "$chatgptProfile/Default/Cache") -eq 'skipped_excluded' -and (StatusOf "$chatgptPartition/Cache") -eq 'skipped_excluded' -and (HasNoRow 'Cache/data_0') }
Check '#113 ChatGPT component cache beside the profile not in the manifest' { -not ($script:Rows | Where-Object { ($_.path -replace '\\', '/') -like '*/web/Codex/component_crx_cache*' }) }
Check '#113 ChatGPT app log collected as chatgpt-desktop' { IsAgentRow "$chatgptPackage/LocalCache/Local/Codex/Logs/2026/10/15/codex-desktop-x.log" 'chatgpt-desktop' $false }
Check '#113 ChatGPT MSIX settings hive collected' { IsAgentRow "$chatgptPackage/Settings/settings.dat" 'chatgpt-desktop' $false }
Check '#116 Codex plugin app-server binaries excluded' { (StatusOf 'Users/alice/.codex/plugins/.plugin-appserver') -eq 'skipped_excluded' -and (HasNoRow '.plugin-appserver/codex.exe') }
Check '#115 Codex desktop global state flagged secret' { IsAgentRow 'Users/alice/.codex/.codex-global-state.json' 'codex-cli' $true }
# 2026-10-15 ChatGPT desktop on macOS (#121)
Check '#121 ChatGPT macOS Local Storage collected as chatgpt-desktop' { IsAgentRow "$chatgptMac/Default/Local Storage/leveldb/000003.log" 'chatgpt-desktop' $false }
Check '#121 ChatGPT macOS profile and partition cookies flagged secret' { (IsAgentRow "$chatgptMac/Default/Cookies" 'chatgpt-desktop' $true) -and (IsAgentRow "$chatgptMacPartition/Cookies" 'chatgpt-desktop' $true) }
Check '#121 ChatGPT macOS Login Data flagged secret' { IsAgentRow "$chatgptMac/Default/Login Data" 'chatgpt-desktop' $true }
Check '#121 ChatGPT macOS profile and partition Cache excluded' { (StatusOf "$chatgptMac/Default/Cache") -eq 'skipped_excluded' -and (StatusOf "$chatgptMacPartition/Cache") -eq 'skipped_excluded' -and (HasNoRow 'Codex/Default/Cache/data_0') -and (HasNoRow 'codex-browser-app/Cache/data_0') }
Check '#121 ChatGPT macOS MCP server script excluded, manifest collected' { (StatusOf "$chatgptMac/executor-plugins/codex-app-tools/server.mjs") -eq 'skipped_excluded' -and (IsAgentRow "$chatgptMac/executor-plugins/codex-app-tools/.mcp.json" 'chatgpt-desktop' $false) }
Check '#121 ChatGPT macOS component cache not in the manifest' { -not ($script:Rows | Where-Object { ($_.path -replace '\\', '/') -like '*Application Support/Codex/component_crx_cache*' }) }
Check '#121 ChatGPT macOS app log collected as chatgpt-desktop' { IsAgentRow 'Users/alice/Library/Logs/com.openai.codex/2026/10/15/codex-desktop-x.log' 'chatgpt-desktop' $false }
Check '#121 ChatGPT macOS preferences collected' { IsAgentRow 'Users/alice/Library/Preferences/com.openai.codex.plist' 'chatgpt-desktop' $false }
Check '#121 ChatGPT macOS binary cookies flagged secret' { IsAgentRow 'Users/alice/Library/HTTPStorages/com.openai.codex.binarycookies' 'chatgpt-desktop' $true }
Check '#121 ChatGPT computer-use group container collected' { IsAgentRow "$cuaGroupContainer/IPC/computeruse.sock.lock" 'chatgpt-desktop' $false }
Check '#121 Codex computer-use app bundle excluded, config collected' { (StatusOf "$computerUse/Codex Computer Use.app") -eq 'skipped_excluded' -and (HasNoRow 'Computer Use.app/Contents/Info.plist') -and (IsAgentRow "$computerUse/config.json" 'codex-cli' $false) }
# 2026-10-15 special files (#120)
$specialCount = 0
if ($socket) {
  $specialCount = 1
  $socketType = 'socket'; if ($env:OS -eq 'Windows_NT') { $socketType = 'other' }
  $row = Row $socketPath
  Check '#120 socket recorded as skipped_special, not copied' { $row -and $row.archive_path -eq '' -and $row.type -eq $socketType -and $row.sha256 -eq '' -and $row.secret -eq $false -and $row.status -eq 'skipped_special' -and $row.target -eq '' -and $row.error -eq '' }
  $socket.Close()
}
Check '#120 special files not in the archive' { -not (ArchiveEntries | Where-Object { $_ -like '*ipc.fifo' -or $_ -like '*daemon-updater.sock' }) }
Check '#120 summary counts skipped_special' { $sum.counts.skipped_special -eq $specialCount }
Check '#120 stdout skipped line counts special files' { $script:stdout -match "(?m)^skipped:    \d+ excluded, \d+ too large, \d+ secret, $specialCount special\s*$" }
Check '#120 no symlink row targets its own path' { -not ($script:Rows | Where-Object { $_.status -eq 'symlink' -and ($_.target -replace '^\\\\\?\\', '' -replace '[\\/]+', '/') -eq ($_.path -replace '[\\/]+', '/') }) }

# 2026-10-16 GitHub Copilot Desktop (#147)
Check '#147 Copilot Desktop data.db collected as copilot-desktop' { (IsAgentRow "$copilot/data.db" 'copilot-desktop' $false) -and (IsAgentRow "$copilot/data.db-wal" 'copilot-desktop' $false) }
Check '#147 Copilot Desktop github-app log collected as copilot-desktop' { IsAgentRow "$copilot/logs/github-app.1.log" 'copilot-desktop' $false }
Check '#147 Copilot Desktop localstorage collected as copilot-desktop' { IsAgentRow "$copilotDesktopData/localstorage/tauri_localhost_0.localstorage" 'copilot-desktop' $false }
Check '#147 Foundry Local log collected as copilot-desktop' { IsAgentRow 'Users/alice/.github-copilot-cli/logs/foundry.core20261016.log' 'copilot-desktop' $false }
Check '#147 Copilot Desktop session workspace.yaml stays copilot-cli' { IsAgentRow "$copilot/session-state/s2/workspace.yaml" 'copilot-cli' $false }
Check '#147 Copilot Desktop ws.token flagged secret' { IsAgentRow "$copilot/run/ws.token" 'copilot-desktop' $true }
Check '#147 Copilot Desktop checkpoint ref and HEAD collected as copilot-desktop' { (IsAgentRow "$copilotCheckpointRef/e0000000-0000-4000-8000-000000000020" 'copilot-desktop' $false) -and (IsAgentRow "$copilotCloneGit/HEAD" 'copilot-desktop' $false) }
Check '#147 Copilot Desktop repos working tree, worktrees, objects and index excluded' { (StatusOf "$copilot/repos") -eq 'skipped_excluded' -and (StatusOf "$copilotCloneGit/objects") -eq 'skipped_excluded' -and (StatusOf "$copilotCloneGit/index") -eq 'skipped_excluded' -and $null -eq (Row 'repos/proj/README.md') -and $null -eq (Row 'alice-x/AGENTS.md') -and $null -eq (Row 'ab/cdef0123') }
Check '#147 Copilot Desktop app-skills excluded' { (StatusOf "$copilotDesktopData/app-skills") -eq 'skipped_excluded' -and $null -eq (Row 'app-skills/af/SKILL.md') }
$copilotLog = Get-Content -LiteralPath (Get-ChildItem -LiteralPath (Join-Path $Out 'default') -Filter '*.log' | Select-Object -First 1).FullName -Raw
$copilotHome = (ImagePath $copilot) + $Separator
Check '#147 Copilot Desktop chat directory and worktree not projects' { $copilotLog -notmatch 'Project .*[\\/]\.copilot[\\/]' -and -not ($script:Rows | Where-Object { $_.home.StartsWith($copilotHome) }) -and -not (@($sum.projects) | Where-Object { $_ -match '\.copilot[\\/](chats|repos)' }) }
Check '#147 Copilot Desktop chat directory file collected once as copilot-desktop' { @($script:Rows | Where-Object { ($_.path -replace '\\', '/') -like '*/x-5d404e14/.keep' }).Count -eq 1 -and (Row "$copilot/chats/2026-10-16/x-5d404e14/.keep").agent -eq 'copilot-desktop' }
# 2026-10-17 Copilot Desktop Windows round (#147)
Check '#147 WebView2 Local Storage collected as copilot-desktop' { IsAgentRow "$copilotWebView/Default/Local Storage/leveldb/000003.log" 'copilot-desktop' $false }
Check '#147 WebView2 cookies and Local State flagged secret' { (IsAgentRow "$copilotWebView/Default/Network/Cookies" 'copilot-desktop' $true) -and (IsAgentRow "$copilotWebView/Local State" 'copilot-desktop' $true) }
Check '#147 WebView2 Cache and component_crx_cache excluded' { (StatusOf "$copilotWebView/Default/Cache") -eq 'skipped_excluded' -and (StatusOf "$copilotWebView/component_crx_cache") -eq 'skipped_excluded' -and $null -eq (Row 'Cache_Data/x') -and $null -eq (Row 'component_crx_cache/x') }
Check '#147 Copilot Desktop chat project collected less node_modules' { (IsAgentRow "$copilotChatProject/package.json" 'copilot-desktop' $false) -and (StatusOf "$copilotChatProject/node_modules") -eq 'skipped_excluded' -and $null -eq (Row 'node_modules/m/index.js') }
Check '#131 Copilot CLI marketplaces excluded' { (StatusOf $copilotMarketplaces) -eq 'skipped_excluded' -and $null -eq (Row 'marketplaces/m/.git/HEAD') }
Check '#147 Copilot pool session cwd not a project' { (StatusOf 'coppool/AGENTS.md') -eq '' -and -not (@($sum.projects) | Where-Object { $_ -match 'coppool' }) }
# 2026-10-17 session sync round (#153)
Check '#153 nanobot remote pairing identity and exchange keys flagged secret' { (IsAgentRow "$nanobotPairing/identity" 'nanobot' $true) -and (IsAgentRow "$nanobotPairing/exchange" 'nanobot' $true) -and (IsAgentRow "$nanobotPairing/request" 'nanobot' $false) }
Check '#153 Agent Zero plugin config.json flagged secret' { (IsAgentRow "Users/alice/agent-zero/main/usr/$agentZeroPluginConfig" 'agent-zero' $true) -and (IsAgentRow "Users/alice/Desktop/agent-zero/usr/$agentZeroPluginConfig" 'agent-zero' $true) }
Check '#153 Agent Zero volume plugin config.json flagged secret' { (IsAgentRow "$agentZeroVolume/$agentZeroPluginConfig" 'agent-zero' $true) -and (Row "$agentZeroVolume/$agentZeroPluginConfig").user -eq 'docker' -and (IsAgentRow "$agentZeroVolume/usr/$agentZeroPluginConfig" 'agent-zero' $true) }
Check '#153 Cline cached remote config flagged secret' { IsAgentRow "$clineRemoteCache/remote_config_org1.json" 'cline' $true }
Check '#153 Cline globalStorage cache collected' { (IsAgentRow "$clineRemoteCache/sync-queue.json" 'cline' $false) -and (StatusOf $clineRemoteCache) -eq '' }
Check '#153 Cline webhook_config.json flagged secret' { IsAgentRow 'Users/alice/Documents/Cline/webhook_config.json' 'cline' $true }
Check '#153 Gemini CLI bug-memory heap snapshot flagged secret' { IsAgentRow 'Users/alice/.gemini/tmp/gem/bug-memory-1792000000000.heapsnapshot' 'gemini-cli' $true }
Check '#153 little-coder legacy config.json flagged secret' { IsAgentRow 'Users/alice/.little-coder/config.json' 'little-coder' $true }
Check '#153 Qwen Code channel credentials flagged secret' { (IsAgentRow 'Users/alice/.qwen/channels/qqbot-credentials.json' 'qwen-code' $true) -and (IsAgentRow 'Users/alice/.qwen/channels/weixin/account.json' 'qwen-code' $true) }
Check '#153 Qwen Code channel routing files not flagged' { (IsAgentRow 'Users/alice/.qwen/channels/sessions.json' 'qwen-code' $false) -and (IsAgentRow 'Users/alice/.qwen/channels/daemon/0123abcd/routes.json' 'qwen-code' $false) }
Check '#153 Kilo log directory collected' { IsAgentRow 'Users/alice/.local/share/kilo/log/2026-10-17T120000.log' 'kilo-code' $false }
# 2026-10-18 Codex shell snapshots (#160)
Check '#160 Codex shell snapshot flagged secret' { IsAgentRow $codexShellSnapshot 'codex-cli' $true }
# 2026-10-19 Codex projectless output directories (#149)
Check '#149 Codex projectless output collected' { (IsAgentRow "$codexOutputDir/outputs/report.md" 'codex-cli' $false) -and ((Row "$codexOutputDir/outputs/report.md").home -replace '\\', '/') -like '*/home/bob' }
Check '#149 Codex projectless dir not a project' { -not (@($sum.projects) | Where-Object { $_ -match 'Documents[\\/]Codex' }) -and -not ($script:Rows | Where-Object { $_.home -match 'Documents[\\/]Codex' }) -and $copilotLog -notmatch 'Project .*Documents[\\/]Codex' }
Check '#149 Codex rollout cwd still a project' { @($sum.projects | Where-Object { ($_ -replace '\\', '/') -like '*/Users/alice/Documents/app' }).Count -eq 1 }

# 2026-10-20 Cursor IDE round on macOS (#6)
function IsCursorRow([string]$relativePath) { $row = Row $relativePath; return ($row -and $row.agent -eq 'cursor' -and $row.status -eq 'collected') }
Check '#6 Cursor AgentStores sync index collected' { IsCursorRow "$cursorMac/AgentStores/cursor_agent_stores/u1/.sync/index.sqlite" }
Check '#6 Cursor AgentStores pin collected' { IsCursorRow "$cursorMac/AgentStores/cursor_agent_stores/.control/u1/pins/1-x.json" }
Check '#6 Cursor update-supervisor events collected' { IsCursorRow "$cursorMacUpdate/events.jsonl" }
Check '#6 Cursor update-supervisor binary excluded' { (StatusOf "$cursorMacUpdate/cursor-update-supervisor") -eq 'skipped_excluded' }
Check '#6 Cursor embedded agent CLI excluded' { (StatusOf "$cursorMacWorker/agent-cli") -eq 'skipped_excluded' -and (HasNoRow 'agent-cli/.local/share/x') }
Check '#6 Cursor agent worker log collected' { IsCursorRow "$cursorMacWorker/cursor-agent-worker-1.log" }
Check '#6 Cursor canvases node_modules excluded' { (StatusOf 'Users/alice/.cursor/projects/p/canvases/node_modules') -eq 'skipped_excluded' -and (HasNoRow 'node_modules/x.d.ts') }
Check '#6 Cursor agent transcript collected' { IsCursorRow 'Users/alice/.cursor/projects/p/agent-transcripts/u/u.jsonl' }
Check '#6 Cursor statsig cache excluded' { (StatusOf 'Users/alice/.cursor/statsig-cache.json') -eq 'skipped_excluded' }

# 2026-10-20 Cursor IDE round on Ubuntu (#236)
Check '#236 Cursor Linux agent-stores collected' { (IsCursorRow "$cursorLinuxStores/u1/.sync/index.sqlite") -and (IsCursorRow "$cursorLinuxStores/.control/u1/pins/1-x.json") }

# 2026-10-21 Cursor IDE round on Windows (#236)
Check '#236 Cursor Windows agent-stores collected' { (IsCursorRow "$cursorWindowsStores/u1/.sync/index.sqlite") -and (IsCursorRow "$cursorWindowsStores/.control/u1/pins/1-x.json") }
Check '#236 Cursor Windows embedded agent CLI excluded' { (StatusOf "$cursorWindowsWorker/agent-cli") -eq 'skipped_excluded' -and (HasNoRow 'anysphere.cursor-agent-worker/agent-cli/x') }
Check '#236 Cursor Windows agent worker log collected' { IsCursorRow "$cursorWindowsWorker/cursor-agent-worker-1.log" }
Check '#236 Cursor plugin cache excluded' { (StatusOf "$cursorPlugins/cache") -eq 'skipped_excluded' -and (HasNoRow 'cache/cursor-public/x/0123abcd/mcp.json') }
Check '#236 Cursor local plugins collected' { IsCursorRow "$cursorPlugins/local/x.json" }
Check '#236 Cursor Windows inno updater excluded' { (StatusOf "$cursorUpdateSupervisor/inno_updater.exe") -eq 'skipped_excluded' }
Check '#236 Cursor Windows update-supervisor state collected' { (IsCursorRow "$cursorUpdateSupervisor/app-ready.json") -and (IsCursorRow "$cursorUpdateSupervisor/state.json") }
# 2026-10-21 OpenHands Agent Canvas credentials and workspaces (#218)
Check '#218 OpenHands Agent Canvas mars-credentials.json flagged secret' {
  @($canvasCredentials | Where-Object {
      $row = Row $_
      -not ($row -and $row.agent -eq 'openhands' -and $row.secret -eq $true -and $row.status -eq 'collected')
    }).Count -eq 0
}
Check '#218 OpenHands automation workspaces excluded' { (StatusOf 'Users/alice/.openhands/workspaces') -eq 'skipped_excluded' -and -not (Row 'Users/alice/.openhands/workspaces/run1/f') }
Check '#218 OpenHands workspaces.json beside the excluded workspaces collected' { (StatusOf 'Users/alice/.openhands/workspaces.json') -eq 'collected' }

# 2026-10-21 Agent Zero source review (#217)
Check '#217 Agent Zero WhatsApp session flagged secret' { (IsAgentRow "Users/alice/agent-zero/main/$whatsappCreds" 'agent-zero' $true) -and (IsAgentRow "Users/alice/Desktop/agent-zero/$whatsappCreds" 'agent-zero' $true) -and (IsAgentRow "var/lib/docker/volumes/a0_usr/_data/$whatsappCreds" 'agent-zero' $true) -and (Row "var/lib/docker/volumes/a0_usr/_data/$whatsappCreds").user -eq 'docker' }
Check '#217 Agent Zero ACP workspace and extra folder discovered' { (StatusOf 'srv/a0acp/AGENTS.md') -eq 'collected' -and (StatusOf 'srv/a0extra/AGENTS.md') -eq 'collected' }
Check '#217 Agent Zero connector screen capture collected' { IsAgentRow $agentZeroCapture 'agent-zero' $false }

# 2026-10-21 framework catalog gaps (#223, #224, #225)
$row = Row 'proj/.crewai/last_run.json'
Check '#223 CrewAI project .crewai collected' { $row -and ($row.home -replace '\\', '/').EndsWith('/proj') -and $row.agent -eq 'crewai' -and $row.status -eq 'collected' }
function IsSecretRow([string]$relativePath) { $row = Row $relativePath; return ($row -and $row.secret -eq $true -and $row.status -eq 'collected') }
Check '#224 difyctl login-pending.yml flagged secret' { (IsSecretRow 'Users/alice/.config/difyctl/login-pending.yml') -and (IsSecretRow 'Users/alice/AppData/Roaming/difyctl/login-pending.yml') -and (IsSecretRow 'Users/alice/AppData/Local/difyctl/login-pending.yml') }
function IsClaiRow([string]$relativePath) { $row = Row $relativePath; return ($row -and $row.agent -eq 'pydantic-clai' -and $row.status -eq 'collected') }
Check '#225 CLAI 2.0 state directory collected' { (IsClaiRow "$claiState/orchestrator.json") -and (IsClaiRow "$claiState/workspaces/registry/ws1/workspace.json") -and (IsClaiRow "$claiState/workspaces/registry/ws1/session.log") }
Check '#225 CLAI 2.0 workspace token flagged secret' { (IsClaiRow "$claiState/workspaces/registry/ws1/token") -and (Row "$claiState/workspaces/registry/ws1/token").secret -eq $true }
Check '#225 CLAI 2.0 workspace clones excluded' { (StatusOf "$claiState/workspaces/clones") -eq 'skipped_excluded' -and @($script:Rows | Where-Object { ($_.path -replace '\\', '/') -like '*/repo-0123456789/.git/HEAD' }).Count -eq 0 }

# 2026-10-21 Codex worktree cwds and archived threads (#222)
Check '#222 Codex archived rollout cwd discovers project' { (StatusOf 'codexarch/AGENTS.md') -eq 'collected' -and @($sum.projects | Where-Object { ($_ -replace '\\', '/') -like '*/codexarch' }).Count -eq 1 }
Check '#222 Codex worktree cwd not a project' { -not (@($sum.projects) | Where-Object { $_ -match '\.codex[\\/]worktrees' }) -and (StatusOf 'home/bob/.codex/worktrees') -eq 'skipped_excluded' -and -not ($script:Rows | Where-Object { $_.path -match 'worktrees[\\/]a1b2[\\/]repo[\\/]AGENTS\.md' }) }
Check '#222 Open Interpreter worktree cwd not a project' { -not (@($sum.projects) | Where-Object { $_ -match '\.openinterpreter[\\/]worktrees' }) -and -not ($script:Rows | Where-Object { $_.path -match 'worktrees[\\/]c3d4[\\/]repo[\\/]AGENTS\.md' }) }

# 2026-10-21 Cody JetBrains plugin files (#214)
Check '#214 Cody JetBrains options files collected' { (IsAgentRow "$jetbrainsOptions/cody.xml" 'cody' $true) -and (IsAgentRow "$jetbrainsOptions/cody_application_settings.xml" 'cody' $false) }
Check '#214 Cody cody_accounts.xml flagged secret' { IsAgentRow "$jetbrainsOptions/cody_accounts.xml" 'cody' $true }
Check '#214 Cody cody_accounts_settings.xml not flagged' { IsAgentRow "$jetbrainsOptions/cody_accounts_settings.xml" 'cody' $false }
Check '#214 Cody home .sourcegraph collected' { IsAgentRow 'Users/alice/.sourcegraph/cody_settings.json' 'cody' $true }
Check '#214 Cody project .idea files collected' { (IsAgentRow 'proj/.idea/cody_history.xml' 'cody' $false) -and (IsAgentRow 'proj/.idea/.sourcegraph/cody_settings.json' 'cody' $true) }

# 2026-10-21 agent worktrees inside a project (#216)
Check '#216 Gemini CLI and Qwen Code project worktrees excluded' { (StatusOf 'gem/.gemini/worktrees') -eq 'skipped_excluded' -and (StatusOf 'gem/.qwen/worktrees') -eq 'skipped_excluded' -and $null -eq (Row 'gem/.gemini/worktrees/w1/f') -and $null -eq (Row 'gem/.qwen/worktrees/w1/f') }
Check '#216 Gemini CLI project settings beside the worktrees collected' { ($row = Row 'gem/.gemini/settings.json') -and $row.agent -eq 'gemini-cli' -and $row.status -eq 'collected' }

# 2026-10-21 Hermes quick snapshots and desktop connections (#123, #208)
Check '#123 Hermes quick snapshot state.db collected' { (IsAgentRow "$hermesSnapshot/state.db" 'hermes' $false) -and (IsAgentRow "$hermesSnapshot/manifest.json" 'hermes' $false) -and (IsAgentRow "$hermesSnapshotWindows/state.db" 'hermes' $false) -and (StatusOf 'Users/alice/.hermes/state-snapshots') -eq '' }
Check '#123 Hermes quick snapshot credentials flagged secret' { (IsAgentRow "$hermesSnapshot/.env" 'hermes' $true) -and (IsAgentRow "$hermesSnapshot/auth.json" 'hermes' $true) -and (IsAgentRow "$hermesSnapshot/pairing/telegram-approved.json" 'hermes' $true) }
Check '#208 Hermes desktop connection files flagged secret' { (IsAgentRow 'Users/alice/.config/Hermes/connections.json' 'hermes' $true) -and (IsAgentRow 'Users/alice/.config/Hermes/connection.json' 'hermes' $true) }

# 2026-10-21 nanobot workspace cron and triggers (#220)
function IsNanobotRow([string]$relativePath) { $row = Row $relativePath; return ($row -and $row.agent -eq 'nanobot' -and $row.status -eq 'collected') }
Check '#220 nanobot workspace cron jobs and runs collected' { (IsNanobotRow 'nanows/cron/jobs.json') -and (IsNanobotRow 'nanows/cron/runs/j1_1_ab.json') }
Check '#220 nanobot workspace triggers and runs collected' { (IsNanobotRow 'nanows/triggers/triggers.json') -and (IsNanobotRow 'nanows/triggers/runs/d1.json') }
Check '#220 nanobot trigger queue not collected' { @($script:Rows | Where-Object { ($_.path -replace '\\', '/') -like '*nanows/triggers/inbox*' }).Count -eq 0 }

# 2026-10-21 OpenClaw source review (#219)
Check '#219 OpenClaw exec-approvals.json flagged secret' { IsAgentRow "$openclaw/exec-approvals.json" 'openclaw' $true }
Check '#219 OpenClaw auth-profile secrets directory flagged secret' { IsAgentRow 'Users/alice/.openclaw-auth-profile-secrets/gateway.env' 'openclaw' $true }
Check '#219 OpenClaw volume exec-approvals.json flagged secret' { $row = Row "$openclawVolume/exec-approvals.json"; $row -and $row.user -eq 'docker' -and $row.secret -eq $true -and $row.status -eq 'collected' }
Check '#219 OpenClaw Gateway clone excluded' { (StatusOf "$openclaw/projects/$openclawCloneId") -eq 'skipped_excluded' -and (HasNoRow "$openclawCloneId/repo/README.md") }
Check '#219 OpenClaw projects entry that is no clone collected' { IsAgentRow "$openclaw/projects/notes/todo.md" 'openclaw' $false }
Check '#219 OpenClaw volume clone, plugin-skills and agent tmp excluded' { (StatusOf "$openclawVolume/projects/$openclawCloneId") -eq 'skipped_excluded' -and (StatusOf "$openclawVolume/plugin-skills") -eq 'skipped_excluded' -and (StatusOf "$openclawVolume/agents/main/agent/tmp") -eq 'skipped_excluded' -and (StatusOf "$openclawVolume/agents/main/agent/.tmp") -eq 'skipped_excluded' }
Check '#219 volume projects entry that is no clone collected' { $row = Row "$openclawVolume/projects/p219/notes.md"; $row -and $row.user -eq 'docker' -and $row.status -eq 'collected' }

# 2026-10-21 OpenCode and Kilo Code source review (#213, #221)
Check '#213 OpenCode service.json flagged secret' { (IsAgentRow 'Users/alice/.config/opencode/service.json' 'opencode' $true) -and (IsAgentRow 'Users/alice/.local/state/opencode/service-local.json' 'opencode' $true) }
Check '#213 OpenCode desktop data collected' { (IsAgentRow "$opencodeDesktop/drafts.sqlite" 'opencode' $false) -and (IsAgentRow 'Users/alice/.config/ai.opencode.desktop.beta/drafts.sqlite' 'opencode' $false) -and (IsAgentRow 'Users/alice/AppData/Roaming/ai.opencode.desktop/drafts.sqlite' 'opencode' $false) }
Check '#213 OpenCode desktop compile-cache excluded' { (StatusOf "$opencodeDesktop/compile-cache") -eq 'skipped_excluded' -and (HasNoRow 'compile-cache/x') }
Check '#213 OpenCode Windows temp root collected' { IsAgentRow 'Users/alice/AppData/Local/Temp/opencode/notes.txt' 'opencode' $false }
Check '#221 Kilo sandbox policy and preference collected' { (IsAgentRow "$kiloSandboxPolicy/bb.json" 'kilo-code' $false) -and (IsAgentRow 'Users/alice/.local/state/kilo-sandbox-preference/bb.json' 'kilo-code' $false) }

# 2026-10-21 Qwen Code session registry (#230)
Check '#230 Qwen Code session registry flagged secret' { (IsAgentRow 'Users/alice/.qwen/sessions/4242.json' 'qwen-code' $true) -and (IsAgentRow 'Users/alice/.qwen/sessions/4243-0123abcd.json' 'qwen-code' $true) }

# 2026-10-21 Roo Code CLI cloud credentials (#159)
$rooRow = Row 'Users/alice/.roo/cli-credentials.json'
Check '#159 roo cli-credentials.json flagged secret' { $rooRow -and $rooRow.agent -eq 'roo-code' -and $rooRow.secret -eq $true -and $rooRow.status -eq 'collected' }

# 2026-10-21 Twinny workspace storage (#215)
Check '#215 twinny workspaceStorage agent-changes attributed to twinny' { $row = Row "$twinnyWorkspace/rjmacarthy.twinny/agent-changes/c1.json"; $row -and $row.agent -eq 'twinny' -and $row.status -eq 'collected' }
Check '#215 twinny workspaceStorage neighbour stays vscode' { $row = Row "$twinnyWorkspace/workspace.json"; $row -and $row.agent -eq 'vscode' -and $row.status -eq 'collected' }
Check '#215 twinny-server forge repos.json flagged secret' { $row = Row 'Users/alice/.twinny/server/plugins/github/repos.json'; $row -and $row.agent -eq 'twinny' -and $row.status -eq 'collected' -and $row.secret -eq $true }

# 2026-10-21 project git metadata (#253)
# Test-GitFilesCollected FILE...: each file under the project's .git has a
# collected row with agent project and the project as its home.
# Test-NoGitRows PREFIX...: no manifest row of the project names a path under
# its .git that starts with it. Both look only at rows whose home is the
# project, because Row matches a path suffix and the Copilot Desktop clone at
# repos/proj/.git also ends in proj/.git.
function ProjectRows { return @($script:Rows | Where-Object { ($_.home -replace '\\', '/').EndsWith('/proj') }) }
function Test-GitFilesCollected([string[]]$files) {
  foreach ($file in $files) {
    $row = ProjectRows | Where-Object { ($_.path -replace '\\', '/').EndsWith("/$projectGit/$file") } | Select-Object -First 1
    if (-not ($row -and $row.agent -eq 'project' -and $row.status -eq 'collected')) { return $false }
  }
  return $true
}
function Test-NoGitRows([string[]]$prefixes) {
  foreach ($prefix in $prefixes) {
    if (@(ProjectRows | Where-Object { ($_.path -replace '\\', '/') -like "*/$projectGit/$prefix*" }).Count -ne 0) { return $false }
  }
  return $true
}
Check '#253 project git HEAD, config, FETCH_HEAD and packed-refs collected as project' { Test-GitFilesCollected @('HEAD', 'config', 'FETCH_HEAD', 'packed-refs') }
Check '#253 project git refs and logs collected as project' { Test-GitFilesCollected @('refs/heads/main', 'refs/remotes/origin/main', 'logs/HEAD', 'logs/refs/remotes/origin/main') }
Check '#253 project git worktrees collected as project' { Test-GitFilesCollected @('worktrees/wt1/HEAD', 'worktrees/wt1/gitdir') }
Check '#253 project git linked worktree index excluded' { $row = ProjectRows | Where-Object { ($_.path -replace '\\', '/').EndsWith("/$projectGit/worktrees/wt1/index") } | Select-Object -First 1; $row -and $row.status -eq 'skipped_excluded' }
Check '#253 project git linked worktree submodules excluded' { $row = ProjectRows | Where-Object { ($_.path -replace '\\', '/').EndsWith("/$projectGit/worktrees/wt1/modules") } | Select-Object -First 1; $row -and $row.status -eq 'skipped_excluded' -and (Test-NoGitRows @('worktrees/wt1/modules/sub')) }
Check '#253 project git linked worktree index.lock and sharedindex excluded' { $lockRow = ProjectRows | Where-Object { ($_.path -replace '\\', '/').EndsWith("/$projectGit/worktrees/wt1/index.lock") } | Select-Object -First 1; $sharedIndexRow = ProjectRows | Where-Object { ($_.path -replace '\\', '/').EndsWith("/$projectGit/worktrees/wt1/sharedindex.3333333333333333333333333333333333333333") } | Select-Object -First 1; $lockRow -and $lockRow.status -eq 'skipped_excluded' -and $sharedIndexRow -and $sharedIndexRow.status -eq 'skipped_excluded' }
Check '#253 project git objects, index, hooks, lfs and modules have no row' { Test-NoGitRows @('objects', 'index', 'hooks', 'lfs', 'modules') }
Check '#253 project git config not flagged secret' { $row = ProjectRows | Where-Object { ($_.path -replace '\\', '/').EndsWith("/$projectGit/config") } | Select-Object -First 1; $row -and $row.secret -eq $false -and $row.status -eq 'collected' }

# 2026-10-23 claude-code Windows round (#256)
Check '#256 Claude Code Windows temp root collected as claude-code' { (IsAgentRow "$claudeTempSession/scratchpad/notes.txt" 'claude-code' $false) -and (IsAgentRow "$claudeTempSession/tasks/b256b256b.output" 'claude-code' $false) }
Check '#256 home-level .claude.json.backup flagged secret' { IsAgentRow 'Users/alice/.claude.json.backup' 'claude-code' $true }
Check '#256 Claude Code daemon keys and roster flagged secret' { (IsAgentRow 'Users/alice/.claude/daemon/control.key' 'claude-code' $true) -and (IsAgentRow 'Users/alice/.claude/daemon/pipe.key' 'claude-code' $true) -and (IsAgentRow 'Users/alice/.claude/daemon/roster.json' 'claude-code' $true) }
Check '#256 Claude Code .claude.json backup flagged secret' { IsAgentRow 'Users/alice/.claude/backups/.claude.json.backup.1792800000000' 'claude-code' $true }
Check '#256 Claude Code corrupted .claude.json copy flagged secret' { IsAgentRow 'Users/alice/.claude/backups/.claude.json.corrupted.2026-10-23T17-00-00-000Z' 'claude-code' $true }
Check '#256 Claude Code paste-cache collected' { IsAgentRow 'Users/alice/.claude/paste-cache/0123456789abcdef.txt' 'claude-code' $false }
Check '#256 Claude Code plugin directory cache excluded' { (StatusOf 'Users/alice/.claude/plugins/plugin-directory-cache-v2.json') -eq 'skipped_excluded' }
Check '#256 project git info/exclude collected as project' { Test-GitFilesCollected @('info/exclude') }

# 2026-10-26 claude-code Linux round (#256)
Check '#256 Claude Code version lock collected as claude-code' { IsAgentRow 'Users/alice/.local/state/claude/locks/2.1.310.lock' 'claude-code' $false }
Check '#256 Claude Code cache staging collected as claude-code' { IsAgentRow 'Users/alice/.cache/claude/staging/entry' 'claude-code' $false }

# 2026-10-26 claude-code macOS round (#256)
Check '#256 Claude Code URL handler bundle collected as claude-code' { $row = Row "$urlHandler/MacOS/claude"; (IsAgentRow "$urlHandler/Info.plist" 'claude-code' $false) -and ((-not $urlHandlerLinkOk) -or ($row -and $row.agent -eq 'claude-code' -and $row.type -eq 'symlink' -and $row.status -eq 'symlink' -and ($row.target -replace '\\', '/') -like '*.local/bin/claude')) }
Check '#256 Claude Code IDE lock and bridge-spawn mcp.json flagged secret' { (IsAgentRow 'Users/alice/.claude/ide/53256.lock' 'claude-code' $true) -and (IsAgentRow "Users/alice/.claude/$bridgeSpawn/mcp.json" 'claude-code' $true) -and (IsAgentRow "Users/alice/.claude/$bridgeSpawn/append-system-prompt.txt" 'claude-code' $false) }
Check '#256 relocated claude home IDE lock and bridge-spawn mcp.json flagged secret' { (IsAgentRow 'Users/alice/.claude-work/ide/53256.lock' 'claude-code' $true) -and (IsAgentRow "Users/alice/.claude-work/$bridgeSpawn/mcp.json" 'claude-code' $true) }

# cross-check archived bytes against manifest hashes
$extracted = Join-Path $Work 'x'; New-Item -ItemType Directory -Path $extracted -Force | Out-Null
if ($script:Archive.Name -like '*.tar.gz') { & tar -xzf $script:Archive.FullName -C $extracted 2>$null } else { [System.IO.Compression.ZipFile]::ExtractToDirectory($script:Archive.FullName, $extracted) }
$hashedCount = Test-ArchivedHashes $extracted
Check "archived file bytes match manifest hashes ($hashedCount files)" { $hashedCount -gt 10 }
# 2026-10-14 manifest size of a collected file (#111)
# The size is the byte count the copy wrote, so it equals the archive member's
# length. CI cannot make NTFS report a stale length, so this guards the
# plumbing only.
$conversationRow = Row "$antigravityData/conversations/a1.db"
$archivedCopy = $null; if ($conversationRow) { $archivedCopy = ExtendedPath (Join-Path $extracted ($conversationRow.archive_path.Replace('/', $Separator))) }
Check '#111 collected size is the length of the archived copy' { $conversationRow -and $conversationRow.size -eq 6 -and (Get-Item -LiteralPath $archivedCopy -Force).Length -eq $conversationRow.size }
# 2026-10-10 long paths (#77)
$deepRow = Row $DeepTranscript
Check '#77 deep transcript collected as claude-code under its plain path' { $deepRow -and $deepRow.status -eq 'collected' -and $deepRow.agent -eq 'claude-code' -and $deepRow.path -eq (ImagePath $DeepTranscript) -and -not $deepRow.path.StartsWith('\\?\') -and $deepRow.error -eq '' }
Check '#77 deep transcript sha256 matches its content and the archived bytes' { $deepRow -and $deepRow.sha256 -eq $DeepSha -and (Get-FileHash -LiteralPath (ExtendedPath (Join-Path $extracted ($deepRow.archive_path.Replace('/', $Separator)))) -Algorithm SHA256).Hash.ToLower() -eq $DeepSha }
Check '#77 archive lists the deep transcript' { $deepRow -and $deepRow.archive_path -eq ('fs/' + $DeepTranscript) -and @($DefaultEntries | Where-Object { $_ -eq ('./' + $deepRow.archive_path) -or $_ -eq $deepRow.archive_path }).Count -eq 1 }

# 2026-10-07 (#71)
# Every line parses when read as UTF-8, and each name that needs escaping comes
# back unchanged in path; the hash cross-check above covers these files too.
function Read-ManifestLines([string]$file) {
  $raw = @([System.IO.File]::ReadAllLines($file, (New-Object System.Text.UTF8Encoding $false)))
  $rows = @(); $errs = 0
  foreach ($line in $raw) { try { $rows += ($line | ConvertFrom-Json) } catch { $errs++ } }
  return @{ count = $raw.Count; rows = $rows; errors = $errs }
}
$escapedManifest = Read-ManifestLines $script:Manifest.FullName
Check '#71 every manifest line parses as JSON' { $escapedManifest.errors -eq 0 -and $escapedManifest.rows.Count -eq $escapedManifest.count -and $escapedManifest.count -gt 10 }
foreach ($fileName in $EscapedNames) {
  $wantedPath = [System.IO.Path]::Combine($EscapedDir, $fileName)
  Check "#71 path round-trips through the manifest: $($fileName -replace '[^\x20-\x7e]', '?')" { @($escapedManifest.rows | Where-Object { $_.path -ceq $wantedPath -and $_.status -eq 'collected' -and $_.archive_path }).Count -eq 1 }
}
# Non-ASCII names, and on Windows a lone surrogate, which the UTF-8 writer
# turns into U+FFFD as ConvertTo-Json's output did. They are collected from an
# image of their own, whose archive Windows tar.exe cannot write (the checks
# below test the fallback). The sha256 is checked against the source, whose
# bytes are known.
$NonAsciiRoot = Join-Path $Work 'root71'
$NonAsciiDir = [System.IO.Path]::Combine($NonAsciiRoot, 'Users', 'alice', '.claude', 'projects', '-C-u71')
[void][System.IO.Directory]::CreateDirectory($NonAsciiDir)
$NonAsciiNames = @(('u' + [char]0x00FC + 'ml' + [char]0x00E9 + '.jsonl'), ([string][char]0x65E5 + [char]0x672C + ' ' + [char]0x8A9E + '.jsonl'), ([string][char]0x0416 + [char]0x0443 + [char]0x0440 + [char]0x043D + [char]0x0430 + [char]0x043B + '.jsonl'))
foreach ($fileName in $NonAsciiNames) { [System.IO.File]::WriteAllText([System.IO.Path]::Combine($NonAsciiDir, $fileName), '{"type":"user"}', (New-Object System.Text.UTF8Encoding $false)) }
# 2026-10-07 (#72)
# A symlink with a Cyrillic name and an absolute target with a CJK name, which
# goes in a pax linkpath record.
$NonAsciiLink = [string][char]0x0441 + [char]0x0441 + [char]0x044B + [char]0x043B + [char]0x043A + [char]0x0430 + '.jsonl'
$NonAsciiLinkTarget = [System.IO.Path]::Combine($NonAsciiDir, $NonAsciiNames[1])
try { New-Item -ItemType SymbolicLink -Path ([System.IO.Path]::Combine($NonAsciiDir, $NonAsciiLink)) -Target $NonAsciiLinkTarget -ErrorAction Stop | Out-Null }
catch { $NonAsciiLink = ''; Write-Output 'note: symlink creation not permitted here, #72 non-ASCII symlink check skipped' }
$LoneSurrogate = ''
if ($Separator -ne '/') {
  $LoneSurrogate = 'lone' + [char]0xD800 + '.jsonl'
  try { [System.IO.File]::WriteAllText([System.IO.Path]::Combine($NonAsciiDir, $LoneSurrogate), '{"type":"user"}', (New-Object System.Text.UTF8Encoding $false)) }
  catch { $LoneSurrogate = ''; Write-Output 'note: #71 lone-surrogate name not allowed here, check skipped' }
}
$nonAsciiOutput = Join-Path $Out 'u71'
New-Item -ItemType Directory -Path $nonAsciiOutput -Force | Out-Null
$nonAsciiArgs = @{ Root = $NonAsciiRoot; OutputDir = $nonAsciiOutput; Quiet = $true; NoDocker = $true }
$global:LASTEXITCODE = 0
& $Collector @nonAsciiArgs *> $null
$nonAsciiExit = $LASTEXITCODE
$nonAsciiManifest = Read-ManifestLines (Get-ChildItem -LiteralPath $nonAsciiOutput -Filter '*.manifest.jsonl' | Select-Object -First 1).FullName
$nonAsciiSha = (Get-FileHash -LiteralPath ([System.IO.Path]::Combine($NonAsciiDir, $NonAsciiNames[0])) -Algorithm SHA256).Hash.ToLower()
Check '#71 non-ASCII image: exit 0 and every manifest line parses' { $nonAsciiExit -eq 0 -and $nonAsciiManifest.errors -eq 0 -and $nonAsciiManifest.rows.Count -eq $nonAsciiManifest.count -and $nonAsciiManifest.count -ge $NonAsciiNames.Count }
foreach ($fileName in $NonAsciiNames) {
  $wantedPath = [System.IO.Path]::Combine($NonAsciiDir, $fileName)
  Check "#71 non-ASCII path round-trips with its sha256: $($fileName -replace '[^\x20-\x7e]', '?')" { @($nonAsciiManifest.rows | Where-Object { $_.path -ceq $wantedPath -and $_.status -eq 'collected' -and $_.sha256 -eq $nonAsciiSha }).Count -eq 1 }
}
if ($LoneSurrogate) {
  $wantedPath = [System.IO.Path]::Combine($NonAsciiDir, $LoneSurrogate.Replace([string][char]0xD800, [string][char]0xFFFD))
  Check '#71 lone surrogate in a name is written as U+FFFD' { @($nonAsciiManifest.rows | Where-Object { $_.path -ceq $wantedPath -and $_.status -eq 'collected' -and $_.sha256 -eq $nonAsciiSha }).Count -eq 1 }
}

# 2026-10-07 (#72)
# Some Windows tar.exe builds exit 0xC0000005 on a name outside the ANSI code
# page. The collector logs the exit code, notes the fallback, and writes the
# .tar.gz with its own tar writer, which keeps every name and the staged links.
# Elsewhere the system tar writes it. Read with .NET, since tar.exe cannot list
# these names either.
$nonAsciiArchive = Get-ChildItem -LiteralPath $nonAsciiOutput | Where-Object { $_.Name -match '\.(tar\.gz|zip)$' } | Select-Object -First 1
$nonAsciiSummary = Get-Content -LiteralPath (Get-ChildItem -LiteralPath $nonAsciiOutput -Filter '*.collection.json' | Select-Object -First 1).FullName -Raw | ConvertFrom-Json
$nonAsciiLog = [System.IO.File]::ReadAllText((Get-ChildItem -LiteralPath $nonAsciiOutput -Filter '*.log' | Select-Object -First 1).FullName)
Check '#72 non-ASCII image archived as a .tar.gz named in collection.json' { $nonAsciiArchive -and $nonAsciiArchive.Name -like '*.tar.gz' -and $nonAsciiSummary.archive -eq $nonAsciiArchive.Name }
# Not every tar.exe build crashes (the windows-latest runner's stores these
# names), so the fallback checks follow the archiver that ran; the forced tar
# failure further down tests the fallback on every host.
if ($env:OS -eq 'Windows_NT' -and (Get-Command tar.exe -ErrorAction SilentlyContinue)) {
  if ($nonAsciiSummary.capabilities.archiver -eq 'tar.exe') {
    Write-Output 'note: tar.exe stored the non-ASCII names here; #72 fallback checked through the forced tar failure only'
    Check '#72 Windows: tar.exe wrote it, so no fallback note' { @($nonAsciiSummary.notes | Where-Object { $_ -match '^tar\.exe failed' }).Count -eq 0 }
  } else {
    Check '#72 Windows: the PowerShell tar writer wrote it after tar.exe failed' { $nonAsciiSummary.capabilities.archiver -eq 'PowerShell tar writer' }
    Check '#72 Windows: notes give the tar.exe exit code and the fallback' { @($nonAsciiSummary.notes | Where-Object { $_ -match '^tar\.exe failed \(exit code 0x[0-9A-F]{8}\); fell back to PowerShell tar writer$' }).Count -eq 1 }
    Check '#72 Windows: the log gives the tar.exe exit code' { $nonAsciiLog -match '(?m)^tar\.exe failed: exit code 0x[0-9A-F]{8}\r?$' }
  }
}
# Checked for the default run and for one with the writer forced, so the pax
# records are tested on every platform; the forced one is also extracted with
# the platform tar where its tar can store these names (not Windows tar.exe).
$nonAsciiWriterOutput = Join-Path $Out 'u72w'
New-Item -ItemType Directory -Path $nonAsciiWriterOutput -Force | Out-Null
$env:CAC_ARCHIVER = 'writer'
$global:LASTEXITCODE = 0
try { & $Collector -Root $NonAsciiRoot -OutputDir $nonAsciiWriterOutput -Quiet -NoDocker *> $null } finally { Remove-Item Env:CAC_ARCHIVER }
$nonAsciiWriterExit = $LASTEXITCODE
$nonAsciiWriterArchive = Get-ChildItem -LiteralPath $nonAsciiWriterOutput | Where-Object { $_.Name -match '\.tar\.gz$' } | Select-Object -First 1
$nonAsciiWriterManifest = Read-ManifestLines (Get-ChildItem -LiteralPath $nonAsciiWriterOutput -Filter '*.manifest.jsonl' | Select-Object -First 1).FullName
Check '#72 non-ASCII image, writer forced: exit 0 and a .tar.gz' { $nonAsciiWriterExit -eq 0 -and $nonAsciiWriterArchive }
$wantedNames = @($NonAsciiNames); if ($LoneSurrogate) { $wantedNames += $LoneSurrogate.Replace([string][char]0xD800, [string][char]0xFFFD) }
# tar.exe rewrites a link target (\\?\ prefix, / separators, and on a runner
# a resolved short path; docs/windows.md), so for its archive only the target's
# file name is compared, and the stored target is printed for diagnosis.
foreach ($pair in @(@('', $nonAsciiArchive, $nonAsciiManifest, [string]$nonAsciiSummary.capabilities.archiver), @(' [writer]', $nonAsciiWriterArchive, $nonAsciiWriterManifest, 'PowerShell tar writer'))) {
  $tag = $pair[0]; $tarListing = Read-TarGz $pair[1].FullName; $pairRows = $pair[2].rows
  Check ('#72 non-ASCII archive: header checksums valid, two zero blocks at the end' + $tag) { $tarListing.badsum -eq 0 -and $tarListing.end -and $tarListing.members.Count -gt 3 }
  foreach ($fileName in $wantedNames) {
    $nameRow = @($pairRows | Where-Object { $_.path -ceq ([System.IO.Path]::Combine($NonAsciiDir, $fileName)) }) | Select-Object -First 1
    Check ("#72 non-ASCII name round-trips through the archive with its sha256: $($fileName -replace '[^\x20-\x7e]', '?')" + $tag) { $nameRow -and @($tarListing.members | Where-Object { $_.name -ceq ('./' + $nameRow.archive_path) -and $_.type -eq '0' -and $_.sha256 -eq $nameRow.sha256 -and $_.sha256 -eq $nonAsciiSha }).Count -eq 1 }
  }
  if ($NonAsciiLink) {
    $linkRow = @($pairRows | Where-Object { $_.path -ceq ([System.IO.Path]::Combine($NonAsciiDir, $NonAsciiLink)) }) | Select-Object -First 1
    if ($pair[3] -eq 'tar.exe' -and $linkRow) { $tarExeLink = @($tarListing.members | Where-Object { $_.name -ceq ('./' + $linkRow.archive_path) }) | Select-Object -First 1; if ($tarExeLink) { Write-Output ('note: tar.exe stored the #72 link target as ' + ([string]$tarExeLink.link -replace '[^\x20-\x7e]', '?')) } }
    Check ('#72 non-ASCII symlink kept in the archive with its target' + $tag) { $linkRow -and $linkRow.status -eq 'symlink' -and $linkRow.archive_path -and @($tarListing.members | Where-Object { $_.name -ceq ('./' + $linkRow.archive_path) -and $_.type -eq '2' -and ($_.link -ceq $NonAsciiLinkTarget -or ($pair[3] -eq 'tar.exe' -and $_.link.Replace('\', '/').EndsWith('/' + $NonAsciiNames[1], [System.StringComparison]::Ordinal))) }).Count -eq 1 }
  }
}
if ($Separator -eq '/') {
  $nonAsciiWriterExtracted = Join-Path $Work 'x72u'; New-Item -ItemType Directory -Path $nonAsciiWriterExtracted -Force | Out-Null
  & tar -xzf $nonAsciiWriterArchive.FullName -C $nonAsciiWriterExtracted 2>$null
  $nonAsciiExtractExit = $LASTEXITCODE
  $extractableRows = @($nonAsciiWriterManifest.rows | Where-Object { $_.status -eq 'collected' -and $_.archive_path })
  Check '#72 non-ASCII archive from the writer extracts with the platform tar' { $nonAsciiExtractExit -eq 0 -and $extractableRows.Count -ge $wantedNames.Count -and @($extractableRows | Where-Object { -not (Test-Path -LiteralPath ([System.IO.Path]::Combine($nonAsciiWriterExtracted, $_.archive_path))) }).Count -eq 0 }
}

# The writer forced with CAC_ARCHIVER on the main image, on every platform:
# the platform tar lists and extracts it, the members are the default run's,
# and the archived bytes match the manifest.
$env:CAC_ARCHIVER = 'writer'
try { Run 'writer72' @{} } finally { Remove-Item Env:CAC_ARCHIVER }
$forcedWriterSummary = Get-Content -LiteralPath $script:Summary.FullName -Raw | ConvertFrom-Json
Check '#72 forced writer: exit 0, .tar.gz, archiver PowerShell tar writer' { $script:ExitCode -eq 0 -and $script:Archive.Name -like '*.tar.gz' -and $forcedWriterSummary.archive -eq $script:Archive.Name -and $forcedWriterSummary.capabilities.archiver -eq 'PowerShell tar writer' -and @($forcedWriterSummary.notes).Count -eq @($defaultRunSummary.notes).Count }
$WriterEntries = @(ArchiveEntries)
Check '#72 forced writer: platform tar lists the same members as the default run' { $WriterEntries.Count -gt 10 -and ((Join-Sorted $WriterEntries) -ceq (Join-Sorted $DefaultEntries)) }
$forcedWriterExtracted = Join-Path $Work 'x72'; New-Item -ItemType Directory -Path $forcedWriterExtracted -Force | Out-Null
& tar -xzf $script:Archive.FullName -C $forcedWriterExtracted 2>$null
$forcedWriterExtractExit = $LASTEXITCODE
$forcedWriterHashed = Test-ArchivedHashes $forcedWriterExtracted
Check "#72 forced writer: platform tar extracts it and the bytes match the manifest ($forcedWriterHashed files)" { $forcedWriterExtractExit -eq 0 -and $forcedWriterHashed -gt 10 }
$forcedWriterTar = Read-TarGz $script:Archive.FullName
Check '#72 forced writer: header checksums valid, two zero blocks at the end' { $forcedWriterTar.badsum -eq 0 -and $forcedWriterTar.end }
if ($linkOk) { Check '#72 forced writer: the main image symlink is stored as a link' { @($forcedWriterTar.members | Where-Object { $_.type -eq '2' -and $_.name -like '*antigravity-cli/cli.log' }).Count -eq 1 } }
Check '#77 built-in tar writer stores the deep transcript' { $deepWriterRow = Row $DeepTranscript; $deepWriterRow -and @($forcedWriterTar.members | Where-Object { $_.type -eq '0' -and $_.name -eq ('./' + $deepWriterRow.archive_path) -and $_.sha256 -eq $DeepSha }).Count -eq 1 -and (Get-FileHash -LiteralPath (ExtendedPath (Join-Path $forcedWriterExtracted ($deepWriterRow.archive_path.Replace('/', $Separator)))) -Algorithm SHA256).Hash.ToLower() -eq $DeepSha }

# The zip forced with CAC_ARCHIVER: entry names use /, are the writer's file
# members without ./, and hold the manifest's bytes.
$env:CAC_ARCHIVER = 'zip'
try { Run 'zip72' @{} } finally { Remove-Item Env:CAC_ARCHIVER }
$forcedZipSummary = Get-Content -LiteralPath $script:Summary.FullName -Raw | ConvertFrom-Json
Check '#72 forced zip: exit 0, .zip, archiver ZipFile' { $script:ExitCode -eq 0 -and $script:Archive.Name -like '*.zip' -and $forcedZipSummary.archive -eq $script:Archive.Name -and $forcedZipSummary.capabilities.archiver -eq 'ZipFile' }
$ZipEntries = @(ArchiveEntries)
$wantZip = @($forcedWriterTar.members | Where-Object { $_.type -eq '0' } | ForEach-Object { $_.name.Substring(2) })
$gotZip = @($ZipEntries | Where-Object { -not $_.EndsWith('/') })
Check '#72 forced zip: entry names use / and match the tar members' { @($ZipEntries | Where-Object { $_ -like 'fs/*/*' }).Count -gt 10 -and ((Join-Sorted $gotZip) -ceq (Join-Sorted $wantZip)) }
if ($Separator -eq '\') { Check '#72 forced zip: no entry name has a backslash' { @($ZipEntries | Where-Object { $_.Contains('\') }).Count -eq 0 } }
Check '#72 forced zip: archived bytes match the manifest' {
  $zip = [System.IO.Compression.ZipFile]::OpenRead($script:Archive.FullName)
  $zipHashed = 0; $badZip = 0
  try {
    foreach ($row in ($script:Rows | Where-Object { $_.status -eq 'collected' -and $_.archive_path })) {
      $entry = $zip.GetEntry($row.archive_path)
      if ($null -eq $entry) { $badZip++; continue }
      $entryStream = $entry.Open()
      try { $zipHash = ([BitConverter]::ToString([System.Security.Cryptography.SHA256]::Create().ComputeHash($entryStream)) -replace '-', '').ToLower() } finally { $entryStream.Dispose() }
      if ($zipHash -ne $row.sha256) { $badZip++ }
      $zipHashed++
    }
  } finally { $zip.Dispose() }
  $badZip -eq 0 -and $zipHashed -gt 10
}
Check '#77 forced zip stores the deep transcript' { $deepZipRow = Row $DeepTranscript; $deepZipRow -and $deepZipRow.status -eq 'collected' -and @($ZipEntries | Where-Object { $_ -eq $deepZipRow.archive_path }).Count -eq 1 }

# 2026-10-29 drift sweep credential gaps (#248)
foreach ($driftEntry in $driftHomeSecrets) {
  $driftPath, $driftAgent = $driftEntry -split '\|'
  Check "#248 home file flagged secret: $driftPath" { IsAgentRow "Users/alice/$driftPath" $driftAgent $true }
}
foreach ($driftEntry in $driftProjectSecrets) {
  $driftPath, $driftAgent = $driftEntry -split '\|'
  Check "#248 project file flagged secret: $driftPath" { IsAgentRow "proj/$driftPath" $driftAgent $true }
}
Check '#322 Factory temp/env dump flagged secret' { IsAgentRow $factoryEnvDump 'factory-droid' $true }
Check '#322 Factory temp entries not starting with e excluded' { (StatusOf 'Users/alice/.factory/temp/run1') -eq 'skipped_excluded' -and (HasNoRow '.factory/temp/run1/out.txt') }
Check '#322 Amp cache outside the catalog not collected' { (HasNoRow '.cache/amp/runner-desktop/r1/state.json') -and (HasNoRow '.cache/amp/desktop/state.json') -and (HasNoRow '.cache/amp/bin/amp') -and (HasNoRow '.cache/amp/obelisk-runs/run.json') }
Check '#322 Ollama Library/Caches outside the catalog not collected' { (HasNoRow 'Library/Caches/ollama/data_0') -and (HasNoRow 'Library/Caches/com.electron.ollama/data_0') }
Check '#321 Windsurf AppData/Local/devin/bin outside the catalog not collected' { HasNoRow 'AppData/Local/devin/bin/devin.exe' }
Check '#324 Hermes suffixed home collected as hermes' { (IsAgentRow 'Users/alice/.hermes-canary/state.db' 'hermes' $false) -and (IsAgentRow 'Users/alice/.hermes-canary/config.yaml' 'hermes' $true) }
Check '#324 Hermes suffixed Windows home config.yaml flagged secret' { IsAgentRow 'Users/alice/AppData/Local/hermes-canary/config.yaml' 'hermes' $true }
Check '#324 Hermes suffixed home installer checkout excluded, its .env kept' { (StatusOf 'Users/alice/.hermes-canary/hermes-agent/hermes_cli') -eq 'skipped_excluded' -and (StatusOf 'Users/alice/.hermes-canary/hermes-agent/.git') -eq 'skipped_excluded' -and (IsSecretRow 'Users/alice/.hermes-canary/hermes-agent/.env') }
Check '#324 Hermes suffixed Windows home installer checkout excluded, its .env kept' { (StatusOf 'Users/alice/AppData/Local/hermes-canary/hermes-agent/hermes_cli') -eq 'skipped_excluded' -and (StatusOf 'Users/alice/AppData/Local/hermes-canary/hermes-agent/.git') -eq 'skipped_excluded' -and (IsSecretRow 'Users/alice/AppData/Local/hermes-canary/hermes-agent/.env') }
Check '#324 Hermes updater cache excluded' { (StatusOf 'Users/alice/AppData/Local/hermes-updater') -eq 'skipped_excluded' -and (StatusOf 'Users/alice/AppData/Local/hermes-updater/pending/hermes-setup.exe') -eq '' }
foreach ($driftPath in $driftHermesDesktop) {
  Check "#324 Hermes suffixed desktop connections flagged: $driftPath" { IsAgentRow "Users/alice/$driftPath/connections.json" 'hermes' $true }
  foreach ($driftLeaf in $driftHermesCaches) {
    Check "#324 Hermes suffixed desktop cache excluded: $driftPath/$driftLeaf" { (StatusOf "Users/alice/$driftPath/$driftLeaf") -eq 'skipped_excluded' }
  }
}
Check '#321 Devin CLI files collected as windsurf and flagged' { (IsAgentRow 'Users/alice/AppData/Roaming/devin/mcp_config.json' 'windsurf' $true) -and (IsAgentRow 'Users/alice/AppData/Roaming/devin/credentials.toml' 'windsurf' $true) }
Check '#321 Devin IDE cache not collected, its User directory collected' { (HasNoRow 'Devin/Cache/data_0') -and (HasNoRow 'Devin/Cache') -and (IsAgentRow 'Users/alice/AppData/Roaming/Devin/User/settings.json' 'windsurf' $false) }
Check '#323 Crush Windows projects.json discovers project' { (Row 'crushwin/crushrc').agent -eq 'crush' }
foreach ($driftPath in $driftCrushFiles) {
  Check "#323 Crush project file flagged secret: $driftPath" { IsAgentRow "crushwin/$driftPath" 'crush' $true }
}

# ---- -NoSecrets -----------------------------------------------------------
Run 'nosecrets' @{ NoSecrets = $true }
Check 'NoSecrets exit 0' { $script:ExitCode -eq 0 }
Check 'credentials skipped_secret' { (StatusOf 'Users/alice/.claude/.credentials.json') -eq 'skipped_secret' }
Check 'credentials absent from archive' { -not (ArchiveEntries | Where-Object { $_ -match 'credentials\.json$' }) }
# 2026-10-11 Claude Desktop Linux deb and Claude Code 2.1.295 session keys (#73, #76)
Check '#73 #76 NoSecrets skips link-key and the session key' { (StatusOf $linkKey) -eq 'skipped_secret' -and (StatusOf $sessionKey) -eq 'skipped_secret' }
# 2026-10-13 Antigravity desktop app (#83, #84)
Check '#84 Antigravity plugin token skipped without secrets' { (StatusOf "$antigravityData/plugin_data/gdocs/token.json") -eq 'skipped_secret' }
# 2026-10-13 Antigravity Linux round (#83, #90, #91)
Check '#90 Antigravity CLI plugin-secrets.json skipped without secrets' { (StatusOf "$antigravityWindows/plugin-secrets.json") -eq 'skipped_secret' }
# 2026-10-15 ChatGPT desktop as the OpenAI.Codex package (#113)
Check '#115 Codex desktop global state skipped without secrets' { (StatusOf 'Users/alice/.codex/.codex-global-state.json') -eq 'skipped_secret' }
# 2026-10-16 GitHub Copilot Desktop (#147)
Check '#147 Copilot Desktop ws.token skipped without secrets' { (StatusOf "$copilot/run/ws.token") -eq 'skipped_secret' }
# 2026-10-17 session sync round (#153)
Check '#153 nanobot remote pairing keys skipped without secrets' { (StatusOf "$nanobotPairing/identity") -eq 'skipped_secret' -and (StatusOf "$nanobotPairing/exchange") -eq 'skipped_secret' }
Check '#153 Agent Zero plugin config.json skipped without secrets' { (StatusOf "Users/alice/agent-zero/main/usr/$agentZeroPluginConfig") -eq 'skipped_secret' -and (StatusOf "Users/alice/Desktop/agent-zero/usr/$agentZeroPluginConfig") -eq 'skipped_secret' }
Check '#153 Agent Zero volume plugin config.json skipped without secrets' { (StatusOf "$agentZeroVolume/$agentZeroPluginConfig") -eq 'skipped_secret' -and (StatusOf "$agentZeroVolume/usr/$agentZeroPluginConfig") -eq 'skipped_secret' }
Check '#153 Cline cached remote config skipped without secrets' { (StatusOf "$clineRemoteCache/remote_config_org1.json") -eq 'skipped_secret' }
Check '#153 Cline webhook_config.json skipped without secrets' { (StatusOf 'Users/alice/Documents/Cline/webhook_config.json') -eq 'skipped_secret' }
Check '#153 Gemini CLI bug-memory heap snapshot skipped without secrets' { (StatusOf 'Users/alice/.gemini/tmp/gem/bug-memory-1792000000000.heapsnapshot') -eq 'skipped_secret' }
Check '#153 little-coder legacy config.json skipped without secrets' { (StatusOf 'Users/alice/.little-coder/config.json') -eq 'skipped_secret' }
Check '#153 Qwen Code channel credentials skipped without secrets' { (StatusOf 'Users/alice/.qwen/channels/qqbot-credentials.json') -eq 'skipped_secret' -and (StatusOf 'Users/alice/.qwen/channels/weixin/account.json') -eq 'skipped_secret' }
Check '#153 Qwen Code channel routing files kept without secrets' { (StatusOf 'Users/alice/.qwen/channels/sessions.json') -eq 'collected' -and (StatusOf 'Users/alice/.qwen/channels/daemon/0123abcd/routes.json') -eq 'collected' }
# 2026-10-18 Codex shell snapshots (#160)
Check '#160 Codex shell snapshot skipped without secrets' { (StatusOf $codexShellSnapshot) -eq 'skipped_secret' }
# 2026-10-21 framework catalog gaps (#223, #224, #225)
Check '#224 difyctl login-pending.yml skipped without secrets' { (StatusOf 'Users/alice/.config/difyctl/login-pending.yml') -eq 'skipped_secret' }
Check '#225 CLAI 2.0 workspace token skipped without secrets' { (StatusOf "$claiState/workspaces/registry/ws1/token") -eq 'skipped_secret' }

# 2026-10-21 Agent Zero source review (#217)
Check '#217 Agent Zero WhatsApp session skipped without secrets' { (StatusOf "Users/alice/agent-zero/main/$whatsappCreds") -eq 'skipped_secret' -and (StatusOf "Users/alice/Desktop/agent-zero/$whatsappCreds") -eq 'skipped_secret' -and (StatusOf "var/lib/docker/volumes/a0_usr/_data/$whatsappCreds") -eq 'skipped_secret' }

# 2026-10-21 Cody JetBrains plugin files (#214)
Check '#214 Cody cody_accounts.xml skipped without secrets' { (StatusOf "$jetbrainsOptions/cody_accounts.xml") -eq 'skipped_secret' }
Check '#214 Cody cody.xml skipped without secrets' { (StatusOf "$jetbrainsOptions/cody.xml") -eq 'skipped_secret' }
Check '#214 Cody cody_settings.json skipped without secrets' { ((StatusOf 'Users/alice/.sourcegraph/cody_settings.json') -eq 'skipped_secret') -and ((StatusOf 'proj/.idea/.sourcegraph/cody_settings.json') -eq 'skipped_secret') }

# 2026-10-21 Hermes quick snapshots and desktop connections (#123, #208)
Check '#123 Hermes quick snapshot credentials skipped without secrets' { (StatusOf "$hermesSnapshot/.env") -eq 'skipped_secret' -and (StatusOf "$hermesSnapshot/auth.json") -eq 'skipped_secret' -and (StatusOf "$hermesSnapshot/state.db") -eq 'collected' }
Check '#208 Hermes desktop connection files skipped without secrets' { (StatusOf 'Users/alice/.config/Hermes/connections.json') -eq 'skipped_secret' }

# 2026-10-21 OpenClaw source review (#219)
Check '#219 OpenClaw credentials skipped without secrets' { (StatusOf "$openclaw/exec-approvals.json") -eq 'skipped_secret' -and (StatusOf 'Users/alice/.openclaw-auth-profile-secrets/gateway.env') -eq 'skipped_secret' -and (StatusOf "$openclawVolume/exec-approvals.json") -eq 'skipped_secret' }

# 2026-10-21 OpenCode and Kilo Code source review (#213, #221)
Check '#213 OpenCode service.json skipped without secrets' { (StatusOf 'Users/alice/.config/opencode/service.json') -eq 'skipped_secret' -and (StatusOf 'Users/alice/.local/state/opencode/service-local.json') -eq 'skipped_secret' }

# 2026-10-21 Qwen Code session registry (#230)
Check '#230 Qwen Code session registry skipped without secrets' { (StatusOf 'Users/alice/.qwen/sessions/4242.json') -eq 'skipped_secret' -and (StatusOf 'Users/alice/.qwen/sessions/4243-0123abcd.json') -eq 'skipped_secret' }

# 2026-10-23 claude-code Windows round (#256)
Check '#256 Claude Code daemon keys and .claude.json backup skipped without secrets' { (StatusOf 'Users/alice/.claude/daemon/control.key') -eq 'skipped_secret' -and (StatusOf 'Users/alice/.claude/daemon/roster.json') -eq 'skipped_secret' -and (StatusOf 'Users/alice/.claude/backups/.claude.json.backup.1792800000000') -eq 'skipped_secret' }
Check '#256 relocated claude home session key, daemon key and backup skipped without secrets' { (StatusOf $ClaudeWorkKey) -eq 'skipped_secret' -and (StatusOf 'Users/alice/.claude-work/daemon/control.key') -eq 'skipped_secret' -and (StatusOf 'Users/alice/.claude-work/backups/.claude.json.backup.1792800000000') -eq 'skipped_secret' }

# 2026-10-26 claude-code macOS round (#256)
Check '#256 Claude Code IDE lock and bridge-spawn mcp.json skipped without secrets' { (StatusOf 'Users/alice/.claude/ide/53256.lock') -eq 'skipped_secret' -and (StatusOf "Users/alice/.claude/$bridgeSpawn/mcp.json") -eq 'skipped_secret' -and (StatusOf 'Users/alice/.claude-work/ide/53256.lock') -eq 'skipped_secret' -and (StatusOf "Users/alice/.claude-work/$bridgeSpawn/mcp.json") -eq 'skipped_secret' }

# 2026-10-29 drift sweep credential gaps (#248)
foreach ($driftEntry in $driftHomeSecrets) {
  $driftPath = ($driftEntry -split '\|')[0]
  Check "#248 home file skipped without secrets: $driftPath" { (StatusOf "Users/alice/$driftPath") -eq 'skipped_secret' }
}
foreach ($driftEntry in $driftProjectSecrets) {
  $driftPath = ($driftEntry -split '\|')[0]
  Check "#248 project file skipped without secrets: $driftPath" { (StatusOf "proj/$driftPath") -eq 'skipped_secret' }
}
Check '#322 Factory temp/env dump skipped without secrets' { (StatusOf $factoryEnvDump) -eq 'skipped_secret' }
Check '#324 Hermes suffixed homes skipped without secrets' { (StatusOf 'Users/alice/.hermes-canary/config.yaml') -eq 'skipped_secret' -and (StatusOf 'Users/alice/AppData/Local/hermes-canary/config.yaml') -eq 'skipped_secret' }
foreach ($driftPath in $driftHermesDesktop) {
  Check "#324 Hermes suffixed desktop connections skipped without secrets: $driftPath" { (StatusOf "Users/alice/$driftPath/connections.json") -eq 'skipped_secret' }
}
Check '#321 Devin CLI files skipped without secrets' { (StatusOf 'Users/alice/AppData/Roaming/devin/mcp_config.json') -eq 'skipped_secret' -and (StatusOf 'Users/alice/AppData/Roaming/devin/credentials.toml') -eq 'skipped_secret' }
foreach ($driftPath in $driftCrushFiles) {
  Check "#323 Crush project file skipped without secrets: $driftPath" { (StatusOf "crushwin/$driftPath") -eq 'skipped_secret' }
}

# 2026-10-04 catalog cases
foreach ($caseLine in $CasesCatalog) {
  $fields = @($caseLine -split '\|', 3); $row = Row "Users/alice/$($fields[2])"
  switch ($fields[0]) {
    'secret' { Check "no-secrets: $($fields[2])" { $row -and $row.status -eq 'skipped_secret' } }
  }
}
# 2026-10-04 Muse Code
foreach ($caseLine in $CasesMuse) {
  $fields = @($caseLine -split '\|', 3); $row = Row "Users/alice/$($fields[2])"
  switch ($fields[0]) {
    'secret' { Check "no-secrets: $($fields[2])" { $row -and $row.agent -eq 'muse-code' -and $row.status -eq 'skipped_secret' } }
  }
}
# 2026-10-06 frameworks sweep (#64)
foreach ($caseLine in $CasesFrameworks) {
  $fields = @($caseLine -split '\|', 3); $row = Row "Users/alice/$($fields[2])"
  switch ($fields[0]) {
    'secret' { Check "no-secrets: $($fields[2])" { $row -and $row.status -eq 'skipped_secret' } }
  }
}

# ---- -Full ----------------------------------------------------------------
Run 'full' @{ Full = $true }
Check 'Full exit 0' { $script:ExitCode -eq 0 }
Check 'cache contents collected with -Full' { (StatusOf 'cache/junk.bin') -eq 'collected' }
# 2026-10-14 catalog match that is an excluded file (#107)
Check '#107 excluded file match collected with --full' { (StatusOf 'Users/alice/.local/bin/.muse-update.107.exe') -eq 'collected' }

# ---- -Users ---------------------------------------------------------------
Run 'users' @{ Users = 'bob' }
Check 'Users exit 0' { $script:ExitCode -eq 0 }
Check 'only bob collected' { (-not ($script:Rows | Where-Object { $_.user -eq 'alice' })) -and ($script:Rows | Where-Object { $_.user -eq 'bob' }) }

# ---- -NoDocker -------------------------------------------------------------
Run 'nodocker' @{ NoDocker = $true }
Check 'NoDocker exit 0' { $script:ExitCode -eq 0 }
Check 'NoDocker collects no volume' { -not ($script:Rows | Where-Object { ($_.path -replace '\\', '/') -like '*docker/volumes*' }) }
$sum = Get-Content -LiteralPath $script:Summary.FullName -Raw | ConvertFrom-Json
Check 'NoDocker recorded in options' { $sum.options.no_docker -eq $true }
Check 'NoDocker prints no docker line' { $script:stdout -notmatch '(?m)^docker:' }

# 2026-10-14 home patterns and the working directory (#105)
Push-Location -LiteralPath $homesCwd
try { Run 'cwd105' @{ NoDocker = $true; NoProjects = $true } } finally { Pop-Location }
$homesUsers = ''
if ($script:Summary) { $homesUsers = @((Get-Content -LiteralPath $script:Summary.FullName -Raw | ConvertFrom-Json).users) -join ',' }
Check '#105 image homes found from a directory with home and Users subdirectories' { $script:ExitCode -eq 0 -and ($script:Rows | Where-Object { $_.user -eq 'bob' }) -and $homesUsers -ne '' -and $homesUsers -eq (@($defaultRunSummary.users) -join ',') }

# 2026-10-14 NESTED_CLAIMS from the environment (#106)
# An inherited value naming a subtree of a matched volume must prune nothing.
$env:NESTED_CLAIMS = ImagePath 'var/lib/docker/volumes/a0_usr/_data/chats'
try { Run 'claims106' @{ NoProjects = $true } } finally { Remove-Item Env:NESTED_CLAIMS }
Check '#106 NESTED_CLAIMS in the environment prunes nothing' { $script:ExitCode -eq 0 -and (StatusOf 'var/lib/docker/volumes/a0_usr/_data/chats/ctx1/chat.json') -eq 'collected' }

# ---- Docker Desktop note ----------------------------------------------------
MakeFile 'Users/alice/AppData/Local/Docker/wsl/disk/readme.txt' 'vhdx lives here'
Run 'desktop' @{}
Check 'Docker Desktop exit 0' { $script:ExitCode -eq 0 }
$sum = Get-Content -LiteralPath $script:Summary.FullName -Raw | ConvertFrom-Json
Check 'Docker Desktop noted in collection.json' { $sum.docker.docker_desktop -eq $true -and @($sum.notes | Where-Object { $_ -like 'docker: Docker Desktop data at *' }).Count -eq 1 }
Check 'Docker Desktop noted in stdout' { $script:stdout -match '(?m)^docker:     3 volumes found, 2 collected, 0 unreadable; Docker Desktop VM disk not collected\s*$' }
Remove-Item -LiteralPath (ImagePath 'Users/alice/AppData/Local/Docker') -Recurse -Force

# 2026-10-06 frameworks sweep (#64)
# Dify, Langflow and n8n volumes, in a run of their own so the counts above hold.
$n8nVolume = 'var/lib/docker/volumes/n8n_data/_data'
MakeFile "$n8nVolume/config" '{"encryptionKey":"k"}'
MakeFile "$n8nVolume/database.sqlite" 'db'
MakeFile "$n8nVolume/nodes/node_modules/pkg/index.js" 'x'
MakeFile "$n8nVolume/node-definitions/d.json" 'x'
MakeFile "$n8nVolume/n8n-sdk-templates/t.json" 'x'
MakeFile "$n8nVolume/ssh/key" 'key'
MakeFile 'var/lib/docker/volumes/n8n_n8n-data/_data/database.sqlite' 'db'
MakeFile 'var/lib/docker/volumes/langflow_langflow-data/_data/secret_key' 'k'
MakeFile 'var/lib/docker/volumes/langflow_langflow-data/_data/cache_secret_key' 'k'
MakeFile 'var/lib/docker/volumes/langflow_langflow-data/_data/profile_pictures/p.svg' 'x'
MakeFile 'var/lib/docker/volumes/docker_dify_agent_local_sandbox_home/_data/.local/share/shellctl/shellctl.db' 'db'
MakeFile 'var/lib/docker/volumes/docker_dify_agent_local_sandbox_workspace/_data/out.txt' 'x'
# 2026-10-21 Hermes quick snapshots and desktop connections (#123, #208)
# A Hermes home in a volume needs the volume-relative secret globs.
$hermesVolumeSnapshot = 'var/lib/docker/volumes/hermes_home/_data/state-snapshots/20261001-120004'
MakeFile "$hermesVolumeSnapshot/state.db" 'sqlite'
MakeFile "$hermesVolumeSnapshot/.env" 'KEY=x'
MakeFile "$hermesVolumeSnapshot/auth.json" '{"providers":{}}'
MakeFile "$hermesVolumeSnapshot/pairing/telegram-approved.json" '{}'
# 2026-10-29 drift sweep credential gaps (#248)
$script:Made.Clear()
$langflowVolumeMcp = 'var/lib/docker/volumes/langflow_langflow-data/_data/0f0e0d0c-1111-4000-8000-000000000248/_mcp_servers_0f0e0d0c-1111-4000-8000-000000000248.json'
$openhandsVolume = 'var/lib/docker/volumes/openhands_state/_data'
$ollamaVolume = 'var/lib/docker/volumes/ollama/_data'
MakeFile $langflowVolumeMcp "{`"mcpServers`":{}}`n"
MakeFile 'var/lib/docker/volumes/hermes_home/_data/config.yaml' "mcp_servers: {}`n"
MakeFile "$openhandsVolume/conversations/c1/acp/codex/auth.json" "{`"tokens`":{}}`n"
MakeFile "$openhandsVolume/agent-canvas/conversations/c2/acp/codex/auth.json" "{`"tokens`":{}}`n"
# 2026-10-30 hermes rebuild (#326)
MakeFile "$openhandsVolume/profiles/p1.json" "{`"model`":`"x`",`"api_key`":`"k`"}`n"
MakeFile "$ollamaVolume/backup/codex/config.toml.1792800000" "model = `"x`"`n"
MakeFile "$ollamaVolume/launch/muse-config/muse/settings.json" "{}`n"
# 2026-10-30 hermes rebuild (#326)
# One file under each volume-relative Hermes exclusion and secret glob, as kind|path below _data.
$hermesVolume = 'var/lib/docker/volumes/hermes_home/_data'
$hermesVolumeCases = @(
  'secret|.env',
  'secret|.env.bak1',
  'secret|.op.env',
  'secret|npmrc',
  'secret|auth.json',
  'secret|auth.json.1',
  'secret|auth/google_oauth.json',
  'secret|.anthropic_oauth.json',
  'secret|.copilot_jwt.json',
  'secret|google_token.json',
  'secret|google_chat_user_tokens/u1.json',
  'secret|slack_tokens.json',
  'secret|honcho.json',
  'secret|mem0.json',
  'secret|webhook_subscriptions.json',
  'secret|teams_pipeline_store.json',
  'secret|mcp-tokens/t1.json',
  'secret|vault/vault.key',
  'secret|browser_auth/state.json',
  'secret|bot-desktop/lease.json',
  'secret|webapp/session-token',
  'secret|webapp/native-sign-in-tokens.json',
  'secret|webapp/sign-in-registration.json',
  'secret|pairing/telegram-approved.json',
  'secret|platforms/pairing/telegram-approved.json',
  'secret|feishu_comment_pairing.json',
  'secret|whatsapp/session/creds.json',
  'secret|platforms/whatsapp/session/creds.json',
  'secret|matrix/store/crypto.db',
  'secret|platforms/matrix/store/crypto.db',
  'secret|cache/bws_cache.json',
  'secret|workspace/meetings/node_token.json',
  'secret|weixin/accounts/a1.json',
  'secret|runtime/photon-sidecar.json',
  'secret|proxy/ca.key',
  'secret|home/.gitconfig',
  'secret|config.yaml.bak.1',
  'secret|.ssh/id_ed25519',
  'secret|.aws/credentials',
  'secret|.aws/config',
  'secret|.gnupg/pubring.kbx',
  'secret|.kube/config',
  'secret|.docker/config.json',
  'secret|.azure/msal_token_cache.json',
  'secret|.config/gh/hosts.yml',
  'secret|.config/gcloud/credentials.db',
  'secret|.envrc',
  'secret|state-snapshots/20261001-120004/.env',
  'secret|state-snapshots/20261001-120004/auth.json',
  'secret|state-snapshots/20261001-120004/config.yaml',
  'secret|state-snapshots/20261001-120004/feishu_comment_pairing.json',
  'secret|state-snapshots/20261001-120004/platforms/pairing/telegram-approved.json',
  'excl|hermes-agent/hermes_cli',
  'excl|hermes-agent/.git',
  'excl|models',
  'excl|runtimes',
  'excl|node',
  'excl|installs',
  'excl|tools',
  'excl|sandboxes',
  'excl|checkpoints/store/objects',
  'excl|checkpoints/store/indexes',
  'excl|checkpoints/legacy-1',
  'excl|backups',
  'excl|browser-profile',
  'excl|browser-profiles',
  'excl|browser_profiles',
  'excl|chrome-debug',
  'excl|bot-desktop/browser-profile',
  'excl|plugins/p1/node_modules',
  'excl|plugins/p1/.venv',
  'secret|profiles/work/.env',
  'secret|profiles/work/.env.bak1',
  'secret|profiles/work/.op.env',
  'secret|profiles/work/npmrc',
  'secret|profiles/work/auth.json',
  'secret|profiles/work/auth.json.1',
  'secret|profiles/work/auth/google_oauth.json',
  'secret|profiles/work/.anthropic_oauth.json',
  'secret|profiles/work/.copilot_jwt.json',
  'secret|profiles/work/google_token.json',
  'secret|profiles/work/google_chat_user_tokens/u1.json',
  'secret|profiles/work/slack_tokens.json',
  'secret|profiles/work/honcho.json',
  'secret|profiles/work/mem0.json',
  'secret|profiles/work/webhook_subscriptions.json',
  'secret|profiles/work/teams_pipeline_store.json',
  'secret|profiles/work/mcp-tokens/t1.json',
  'secret|profiles/work/vault/vault.key',
  'secret|profiles/work/browser_auth/state.json',
  'secret|profiles/work/bot-desktop/lease.json',
  'secret|profiles/work/webapp/session-token',
  'secret|profiles/work/webapp/native-sign-in-tokens.json',
  'secret|profiles/work/webapp/sign-in-registration.json',
  'secret|profiles/work/pairing/telegram-approved.json',
  'secret|profiles/work/platforms/pairing/telegram-approved.json',
  'secret|profiles/work/feishu_comment_pairing.json',
  'secret|profiles/work/whatsapp/session/creds.json',
  'secret|profiles/work/platforms/whatsapp/session/creds.json',
  'secret|profiles/work/matrix/store/crypto.db',
  'secret|profiles/work/platforms/matrix/store/crypto.db',
  'secret|profiles/work/cache/bws_cache.json',
  'secret|profiles/work/workspace/meetings/node_token.json',
  'secret|profiles/work/weixin/accounts/a1.json',
  'secret|profiles/work/runtime/photon-sidecar.json',
  'secret|profiles/work/proxy/ca.key',
  'secret|profiles/work/home/.gitconfig',
  'secret|profiles/work/config.yaml.bak.1',
  'secret|profiles/work/.ssh/id_ed25519',
  'secret|profiles/work/.aws/credentials',
  'secret|profiles/work/.aws/config',
  'secret|profiles/work/.gnupg/pubring.kbx',
  'secret|profiles/work/.kube/config',
  'secret|profiles/work/.docker/config.json',
  'secret|profiles/work/.azure/msal_token_cache.json',
  'secret|profiles/work/.config/gh/hosts.yml',
  'secret|profiles/work/.config/gcloud/credentials.db',
  'secret|profiles/work/.envrc',
  'secret|profiles/work/config.yaml',
  'secret|.aws/cli/cache/x.json',
  'secret|profiles/work/.aws/sso/cache/x.json',
  'secret|profiles/work/.aws/cli/cache/x.json',
  'plain|.aws/amazonq/history/x.json',
  'plain|profiles/work/state.db'
)
foreach ($caseLine in $hermesVolumeCases) {
  $fields = @($caseLine -split '\|', 2)
  if ($fields[0] -eq 'excl') { MakeFile "$hermesVolume/$($fields[1])/f" 'x' } else { MakeFile "$hermesVolume/$($fields[1])" 'x' }
}
$missing = Test-Made
Check "#248 every frameworks-volume fixture was created ($($missing.Count) missing$(if ($missing.Count) { ', first ' + $missing[0] }))" { $missing.Count -eq 0 }
Run 'fwvolumes' @{}
Check 'frameworks volumes exit 0' { $script:ExitCode -eq 0 }
$row = Row "$n8nVolume/database.sqlite"
Check 'n8n_data volume collected as n8n' { $row -and $row.user -eq 'docker' -and $row.agent -eq 'n8n' -and $row.status -eq 'collected' -and ($row.home -replace '\\', '/').EndsWith($n8nVolume) }
$row = Row "$n8nVolume/config"
Check 'n8n volume config flagged secret (volume-relative)' { $row -and $row.secret -eq $true -and $row.status -eq 'collected' }
$row = Row "$n8nVolume/ssh/key"
Check 'n8n volume ssh key flagged secret (volume-relative)' { $row -and $row.secret -eq $true -and $row.status -eq 'collected' }
Check 'n8n volume nodes/node_modules excluded (volume-relative)' { (StatusOf "$n8nVolume/nodes/node_modules") -eq 'skipped_excluded' }
Check 'n8n volume node-definitions excluded (volume-relative)' { (StatusOf "$n8nVolume/node-definitions") -eq 'skipped_excluded' }
Check 'n8n volume n8n-sdk-templates excluded (volume-relative)' { (StatusOf "$n8nVolume/n8n-sdk-templates") -eq 'skipped_excluded' }
$row = Row 'var/lib/docker/volumes/n8n_n8n-data/_data/database.sqlite'
Check 'n8n_n8n-data volume collected as n8n' { $row -and $row.agent -eq 'n8n' -and $row.status -eq 'collected' }
$row = Row 'var/lib/docker/volumes/langflow_langflow-data/_data/secret_key'
Check 'langflow-data volume secret_key flagged (volume-relative)' { $row -and $row.agent -eq 'langflow' -and $row.secret -eq $true -and $row.status -eq 'collected' }
$row = Row 'var/lib/docker/volumes/langflow_langflow-data/_data/cache_secret_key'
Check 'langflow-data volume cache_secret_key flagged (volume-relative)' { $row -and $row.secret -eq $true -and $row.status -eq 'collected' }
Check 'langflow-data volume profile_pictures excluded (volume-relative)' { (StatusOf 'var/lib/docker/volumes/langflow_langflow-data/_data/profile_pictures') -eq 'skipped_excluded' }
$row = Row 'var/lib/docker/volumes/docker_dify_agent_local_sandbox_home/_data/.local/share/shellctl/shellctl.db'
Check 'dify sandbox home volume collected as dify' { $row -and $row.agent -eq 'dify' -and $row.status -eq 'collected' }
$row = Row 'var/lib/docker/volumes/docker_dify_agent_local_sandbox_workspace/_data/out.txt'
Check 'dify sandbox workspace volume collected as dify' { $row -and $row.agent -eq 'dify' -and $row.status -eq 'collected' }
# 2026-10-21 Hermes quick snapshots and desktop connections (#123, #208)
$row = Row "$hermesVolumeSnapshot/state.db"
Check '#123 Hermes volume quick snapshot collected, credentials flagged (volume-relative)' { $row -and $row.user -eq 'docker' -and $row.agent -eq 'hermes' -and $row.secret -eq $false -and $row.status -eq 'collected' -and (Row "$hermesVolumeSnapshot/.env").secret -eq $true -and (Row "$hermesVolumeSnapshot/auth.json").secret -eq $true -and (Row "$hermesVolumeSnapshot/pairing/telegram-approved.json").secret -eq $true }
# 2026-10-30 hermes rebuild (#326)
foreach ($caseLine in $hermesVolumeCases) {
  $fields = @($caseLine -split '\|', 2)
  if ($fields[0] -eq 'excl') {
    Check "#326 Hermes volume excluded: $($fields[1])" { (StatusOf "$hermesVolume/$($fields[1])") -eq 'skipped_excluded' }
  } elseif ($fields[0] -eq 'plain') {
    Check "#326 Hermes volume collected, not secret: $($fields[1])" { IsAgentRow "$hermesVolume/$($fields[1])" 'hermes' $false }
  } else {
    Check "#326 Hermes volume flagged: $($fields[1])" { IsAgentRow "$hermesVolume/$($fields[1])" 'hermes' $true }
  }
}
# 2026-10-29 drift sweep credential gaps (#248)
Check '#248 langflow-data volume _mcp_servers file flagged (volume-relative)' { IsAgentRow $langflowVolumeMcp 'langflow' $true }
Check '#248 Hermes volume config.yaml flagged (volume-relative)' { IsAgentRow 'var/lib/docker/volumes/hermes_home/_data/config.yaml' 'hermes' $true }
Check '#248 OpenHands volume conversation acp/codex/auth.json flagged (volume-relative)' { (IsAgentRow "$openhandsVolume/conversations/c1/acp/codex/auth.json" 'openhands' $true) -and (IsAgentRow "$openhandsVolume/agent-canvas/conversations/c2/acp/codex/auth.json" 'openhands' $true) }
# 2026-10-30 hermes rebuild (#326)
Check '#326 OpenHands volume LLM profile flagged (volume-relative)' { IsAgentRow "$openhandsVolume/profiles/p1.json" 'openhands' $true }
Check '#320 Ollama volume backup and Muse copy flagged (volume-relative)' { (IsAgentRow "$ollamaVolume/backup/codex/config.toml.1792800000" 'ollama' $true) -and (IsAgentRow "$ollamaVolume/launch/muse-config/muse/settings.json" 'ollama' $true) }
Run 'fwvolumesnosecrets' @{ NoSecrets = $true }
Check '#248 frameworks volumes no-secrets exit 0' { $script:ExitCode -eq 0 }
Check '#248 langflow-data volume _mcp_servers file skipped without secrets (volume-relative)' { (StatusOf $langflowVolumeMcp) -eq 'skipped_secret' }
Check '#248 Hermes volume config.yaml skipped without secrets (volume-relative)' { (StatusOf 'var/lib/docker/volumes/hermes_home/_data/config.yaml') -eq 'skipped_secret' }
# 2026-10-30 hermes rebuild (#326)
foreach ($caseLine in $hermesVolumeCases) {
  $fields = @($caseLine -split '\|', 2)
  if ($fields[0] -eq 'secret') {
    Check "#326 Hermes volume skipped without secrets: $($fields[1])" { (StatusOf "$hermesVolume/$($fields[1])") -eq 'skipped_secret' }
  }
}
Check '#248 OpenHands volume conversation acp/codex/auth.json skipped without secrets (volume-relative)' { (StatusOf "$openhandsVolume/conversations/c1/acp/codex/auth.json") -eq 'skipped_secret' -and (StatusOf "$openhandsVolume/agent-canvas/conversations/c2/acp/codex/auth.json") -eq 'skipped_secret' }
# 2026-10-30 hermes rebuild (#326)
Check '#326 OpenHands volume LLM profile skipped without secrets (volume-relative)' { (StatusOf "$openhandsVolume/profiles/p1.json") -eq 'skipped_secret' }
Check '#320 Ollama volume backup and Muse copy skipped without secrets (volume-relative)' { (StatusOf "$ollamaVolume/backup/codex/config.toml.1792800000") -eq 'skipped_secret' -and (StatusOf "$ollamaVolume/launch/muse-config/muse/settings.json") -eq 'skipped_secret' }
foreach ($volume in @('n8n_data', 'n8n_n8n-data', 'langflow_langflow-data', 'docker_dify_agent_local_sandbox_home', 'docker_dify_agent_local_sandbox_workspace', 'hermes_home', 'openhands_state', 'ollama')) {
  Remove-Item -LiteralPath (ImagePath "var/lib/docker/volumes/$volume") -Recurse -Force
}

# ---- -Inventory -----------------------------------------------------------
# Stat-only fleet inventory: JSON Lines on stdout, nothing written anywhere.
function InventoryRun([hashtable]$extra) {
  $cargs = @{ Root = $Root; Inventory = $true; Quiet = $true }
  foreach ($k in $extra.Keys) { $cargs[$k] = $extra[$k] }
  $global:LASTEXITCODE = 0
  $script:InventoryLines = @(& $Collector @cargs 2>$null | ForEach-Object { [string]$_ })
  $script:ExitCode = $LASTEXITCODE
  $script:Inventory = @($script:InventoryLines | ForEach-Object { $_ | ConvertFrom-Json })
}
function InventoryAgent([string]$user, [string]$agent) { return ($script:Inventory | Where-Object { $_.type -eq 'agent' } | Where-Object { $_.user -eq $user -and $_.agent -eq $agent } | Select-Object -First 1) }
# The raw line, for the timestamps: PowerShell 7's ConvertFrom-Json turns ISO strings into DateTime.
function InventoryLine([string]$user, [string]$agent) { return ($script:InventoryLines | Where-Object { $_.Contains('"user":"' + $user + '","agent":"' + $agent + '",') } | Select-Object -First 1) }
$hostKeys = 'type,host,collector,mode,at,users_scanned,users_unreadable,docker_volumes'
$agentKeys = 'type,host,user,agent,files,bytes,first,last,projects,evidence'
[System.IO.File]::SetLastWriteTimeUtc((ImagePath 'home/bob/.ollama/history'), (New-Object DateTime 2026, 9, 15, 1, 2, 3, ([DateTimeKind]::Utc)))
# bob's ~/.gemini holds only the nested antigravity-cli entry: no gemini-cli line.
MakeFile 'home/bob/.gemini/antigravity-cli/conversations/b1.db' 'db'
$inventoryCwd = Join-Path $Out 'invcwd'
New-Item -ItemType Directory -Path $inventoryCwd -Force | Out-Null
Push-Location -LiteralPath $inventoryCwd
InventoryRun @{}
Pop-Location
Check 'Inventory exit 0' { $script:ExitCode -eq 0 }
Check 'Inventory writes nothing in the current directory' { @(Get-ChildItem -LiteralPath $inventoryCwd -Force).Count -eq 0 }
Check 'Inventory first line is the host line, keys in order' { $script:Inventory[0].type -eq 'host' -and (@($script:Inventory[0].PSObject.Properties | ForEach-Object { $_.Name }) -join ',') -eq $hostKeys -and $script:Inventory[0].mode -eq 'image' -and $script:InventoryLines[0] -match '"at":"\d{4}-\d\d-\d\dT\d\d:\d\d:\d\dZ"' }
Check 'Inventory host line counts' { $script:Inventory[0].users_unreadable -eq 0 -and $script:Inventory[0].docker_volumes -eq 3 -and $script:Inventory[0].users_scanned -ge 2 }
Check 'Inventory every later line is an agent line, keys in order' { $script:Inventory.Count -gt 10 -and @($script:Inventory | Select-Object -Skip 1 | Where-Object { $_.type -ne 'agent' -or (@($_.PSObject.Properties | ForEach-Object { $_.Name }) -join ',') -ne $agentKeys }).Count -eq 0 }
$row = InventoryAgent 'bob' 'ollama'
Check 'Inventory ollama line: history and manifest, excluded blobs not counted' { $row -and $row.files -eq 2 -and $row.bytes -eq 23 -and (InventoryLine 'bob' 'ollama').Contains('"first":"2026-09-14T00:00:00Z","last":"2026-09-15T01:02:03Z"') -and $row.evidence -eq '.ollama' }
$row = InventoryAgent 'alice' 'claude-code'
Check 'Inventory claude-code evidence in catalog order' { $row -and $row.evidence -eq '.claude,.claude.json*,.claude-*/projects,.claude-*/history.jsonl,.claude-*/.claude.json*,.claude-*/.credentials.json,.claude-*/sessions/*.*.key,.claude-*/daemon/control.key,.claude-*/daemon/roster.json,.claude-*/backups/.claude.json.*,.claude-*/ide/*.lock,.claude-*/bridge-spawn/*/mcp.json,.local/state/claude,.cache/claude,Library/Caches/claude-cli-nodejs,AppData/Local/claude-cli-nodejs/Cache/*,AppData/Local/Temp/claude,Applications/Claude Code URL Handler.app' }
Check 'Inventory first and last set for alice claude-code' { (InventoryLine 'alice' 'claude-code') -match '"first":"20\d\d-\d\d-\d\dT\d\d:\d\d:\d\dZ","last":"20\d\d-\d\d-\d\dT\d\d:\d\d:\d\dZ"' }
Check 'Inventory: matched directory with no files of its own gives no line' { $null -eq (InventoryAgent 'bob' 'gemini-cli') -and ($bobLine = InventoryAgent 'bob' 'antigravity') -and $bobLine.files -eq 1 -and $bobLine.bytes -eq 2 -and $bobLine.evidence -eq '.gemini/antigravity-cli' }
Check 'Inventory: no agent line has zero files' { -not ($script:Inventory | Where-Object { $_.type -eq 'agent' } | Where-Object { $_.files -eq 0 }) }
Check 'Inventory projects counted per user' { $row.projects -ge 1 }
Check 'Inventory nested entry claimed from the enclosing agent' { (InventoryAgent 'alice' 'antigravity').evidence -like '*.gemini/antigravity-cli*' -and (InventoryAgent 'alice' 'gemini-cli').evidence -eq '.gemini' }
# 2026-10-14 catalog match that is an excluded file (#107)
# alice's Muse Code file count, compared with the -Full inventory's below.
$museFiles = -1
$row = InventoryAgent 'alice' 'muse-code'
if ($row) { $museFiles = $row.files }
$row = InventoryAgent 'docker' 'agent-zero'
Check 'Inventory docker volume line' { $row -and $row.files -eq 7 -and $row.bytes -eq 122 -and $row.projects -eq 0 -and $row.evidence -eq '*a0_usr' }
$row = InventoryAgent 'docker' 'tabby'
Check 'Inventory rootless docker volume line' { $row -and $row.files -eq 1 -and $row.bytes -eq 22 -and $row.evidence -eq '*tabby*' }
Check 'Inventory shared not inventoried' { -not ($script:Inventory | Where-Object { $_.type -eq 'agent' } | Where-Object { @('shared', 'project', 'live') -contains $_.agent }) }
# 2026-10-19 Codex projectless output directories (#149)
Check '#149 inventory codex-cli evidence names Documents/Codex' { (InventoryAgent 'bob' 'codex-cli').evidence -like '*Documents/Codex*' }
Check 'Inventory prints no path below a home' { -not ($script:InventoryLines | Where-Object { $_.Contains($Root) -or $_.Contains($Root.Replace('\', '\\')) }) }
$inventoryOutput = Join-Path $Out 'inv-o'
InventoryRun @{ OutputDir = $inventoryOutput }
Check 'Inventory with -OutputDir exit 0' { $script:ExitCode -eq 0 }
Check 'Inventory with -OutputDir creates no output directory' { -not (Test-Path -LiteralPath $inventoryOutput) }
InventoryRun @{ Full = $true }
Check 'Inventory -Full counts excluded files' { $row = InventoryAgent 'bob' 'ollama'; $row -and $row.files -eq 3 -and $row.bytes -eq 27 }
# 2026-10-14 catalog match that is an excluded file (#107)
# -Full adds one file for each excl and exclf case of $CasesMuse, plus the
# updater file, which the default inventory must not count either.
$museFull = InventoryAgent 'alice' 'muse-code'
Check '#107 inventory counts an excluded file match only with --full' { $museFiles -ge 0 -and $museFull -and ($museFull.files - $museFiles) -eq (@($CasesMuse | Where-Object { $_ -like 'excl*' }).Count + 1) }
InventoryRun @{ Users = 'bob'; NoDocker = $true }
Check 'Inventory -Users keeps one user' { $agentLines = @($script:Inventory | Where-Object { $_.type -eq 'agent' }); -not ($agentLines | Where-Object { $_.user -eq 'alice' }) -and ($agentLines | Where-Object { $_.user -eq 'bob' }) }
Check 'Inventory -NoDocker: no volumes, no docker lines' { $script:Inventory[0].users_scanned -eq 1 -and $script:Inventory[0].docker_volumes -eq 0 -and -not ($script:Inventory | Where-Object { $_.type -eq 'agent' } | Where-Object { $_.user -eq 'docker' }) }
# stderr is only observable from a child process: the collector logs through [Console]::Error.
$self = (Get-Process -Id $PID).Path
$errFile = Join-Path $Out 'inv-child.stderr'
$childOut = @(& $self -NoProfile -ExecutionPolicy Bypass -File $Collector -Root $Root -Inventory -OutputDir $inventoryOutput 2>$errFile)
Check 'Inventory without -Quiet: stdout is pure JSON Lines, log on stderr' { $childOut.Count -gt 10 -and @($childOut | Where-Object { $_ -notmatch '^\{"type":"(host|agent)",' }).Count -eq 0 -and (Get-Content -LiteralPath $errFile -Raw) -match 'User ' }
Check 'Inventory child stdout parses line by line' { @($childOut | Where-Object { $_ -ne '' } | ForEach-Object { try { $null = $_ | ConvertFrom-Json; $true } catch { $false } } | Where-Object { -not $_ }).Count -eq 0 }
# Windows PowerShell 5.1 wraps redirected error-stream text at the console width, so a long path splits across lines.
Check 'Inventory -OutputDir noted on stderr' { ((Get-Content -LiteralPath $errFile -Raw) -replace '\s+', ' ') -match 'NOTE: -Inventory writes nothing; -OutputDir .*inv-o ignored' }
$childQuiet = @(& $self -NoProfile -ExecutionPolicy Bypass -File $Collector -Root $Root -Inventory -Quiet 2>$errFile)
Check 'Inventory -Quiet leaves stderr empty' { $childQuiet.Count -gt 10 -and -not ((Get-Content -LiteralPath $errFile -Raw) -match '\S') }
# 2026-10-04 bug round: #29
# The -Users run above left alice out, so run once more.
InventoryRun @{ NoDocker = $true }
$row = InventoryAgent 'alice' 'pearai'
if ($caseInsensitive) {
  Check '#29 Inventory evidence keeps only the first spelling' { $row -and $row.evidence -clike '*.config/PearAI/User*' -and $row.evidence -cnotlike '*.config/pearai/User*' }
} else {
  Check '#29 Inventory evidence keeps both distinct spellings' { $row -and $row.evidence -clike '*.config/PearAI/User,.config/pearai/User*' }
}
Remove-Item -LiteralPath (ImagePath 'home/bob/.gemini') -Recurse -Force
if ($Separator -eq '/' -and (& id -u) -ne '0') {
  New-Item -ItemType Directory -Path (ImagePath 'home/carol/.claude') -Force | Out-Null
  & chmod 000 (ImagePath 'home/carol')
  InventoryRun @{}
  & chmod 700 (ImagePath 'home/carol')
  Check 'Inventory counts the unreadable home' { $script:Inventory[0].users_unreadable -eq 1 }
  Remove-Item -LiteralPath (ImagePath 'home/carol') -Recurse -Force
} else { Write-Output 'note: unreadable home inventory check needs a non-root POSIX host, skipped' }

# ---- live mode without the snapshot (1.6.0) --------------------------------
$liveOutput = Join-Path $Out 'livemode'
New-Item -ItemType Directory -Path $liveOutput -Force | Out-Null
$global:LASTEXITCODE = 0
$null = & $Collector -OutputDir $liveOutput -Quiet -Users 'no-such-user-cac' -NoDocker -NoProjects 2>&1
Check 'live mode exit 0' { $LASTEXITCODE -eq 0 }
$liveManifest = Get-ChildItem -LiteralPath $liveOutput -Filter '*.manifest.jsonl' | Select-Object -First 1
$liveSummary = Get-ChildItem -LiteralPath $liveOutput -Filter '*.collection.json' | Select-Object -First 1
Check 'live mode manifest has no live rows' { $liveManifest -and -not (Select-String -LiteralPath $liveManifest.FullName -SimpleMatch '"agent":"live"') }
Check 'live mode collection.json has hostname and no no_live' { $liveJson = Get-Content -LiteralPath $liveSummary.FullName -Raw | ConvertFrom-Json; $liveJson.hostname -ne '' -and $null -eq $liveJson.options.PSObject.Properties['no_live'] }
Check 'live mode writes no live directory' { -not (Test-Path -LiteralPath (Join-Path $liveOutput 'live')) -and -not (Get-ChildItem -LiteralPath $liveOutput -Filter '.stage-*' -Force) }
Check '-NoLive is no longer a parameter' { $threw = $false; try { & $Collector -NoLive -List | Out-Null } catch { $threw = $true }; $threw }

# ---- -List ----------------------------------------------------------------
Check '-List prints catalog' { @(& $Collector -List | Select-String -SimpleMatch 'claude-code|.claude').Count -ge 1 }
Check '-List prints docker volume table' { @(& $Collector -List | Where-Object { $_ -eq 'agent-zero|*a0_usr' }).Count -eq 1 }

# 2026-10-04 bug round: #31
# collection.json names the archive and archiver actually used, and records
# the project options. A tar that fails, first on PATH, forces a fallback: a
# shell script on POSIX, a copy of where.exe named tar.exe on Windows (it
# rejects -czf with exit 2). With CAC_ARCHIVER=tar,zip the fallback is the
# zip; by default it is the PowerShell tar writer.
$defaultSummaryText = Get-Content -LiteralPath (Get-ChildItem -LiteralPath (Join-Path $Out 'default') -Filter '*.collection.json' | Select-Object -First 1).FullName -Raw
$defaultSummary = $defaultSummaryText | ConvertFrom-Json
$defaultArchive = (Get-ChildItem -LiteralPath (Join-Path $Out 'default') | Where-Object { $_.Name -match '\.(tar\.gz|zip)$' } | Select-Object -First 1).Name
Check '#31 default summary names the archive written' { $defaultSummary.archive -eq $defaultArchive -and (($defaultArchive -like '*.tar.gz' -and @('tar.exe', 'PowerShell tar writer') -contains $defaultSummary.capabilities.archiver) -or ($defaultArchive -like '*.zip' -and $defaultSummary.capabilities.archiver -eq 'ZipFile')) }
Check '#31 default summary: no_projects false, projects empty' { $defaultSummary.options.no_projects -eq $false -and $defaultSummaryText -match '"no_projects":\s*false,\s*"projects":\s*\[\s*\]' }
$shim = Join-Path $Work 'shim31'
New-Item -ItemType Directory -Path $shim -Force | Out-Null
$shimOk = $false
if ($env:OS -eq 'Windows_NT') {
  $whereExe = Join-Path $env:SystemRoot 'System32\where.exe'
  if (Test-Path -LiteralPath $whereExe) { Copy-Item -LiteralPath $whereExe -Destination (Join-Path $shim 'tar.exe'); $shimOk = $true }
} else {
  [System.IO.File]::WriteAllText((Join-Path $shim 'tar'), "#!/bin/sh`necho 'tar: broken' >&2`nexit 2`n", (New-Object System.Text.UTF8Encoding $false))
  & chmod +x (Join-Path $shim 'tar'); $shimOk = $true
}
if ($shimOk) {
  $oldPath = $env:PATH
  $env:PATH = $shim + [System.IO.Path]::PathSeparator + $env:PATH
  $projectOne = ImagePath 'srv/p31-one'; $projectTwo = ImagePath 'srv/p31 two'
  $env:CAC_ARCHIVER = 'tar,zip'
  try { Run 'zip31' @{ Project = @($projectOne, $projectTwo) } } finally { Remove-Item Env:CAC_ARCHIVER }
  $env:PATH = $oldPath
  Check '#31 zip fallback exit 0, zip written' { $script:ExitCode -eq 0 -and $script:Archive -and $script:Archive.Name -like '*.zip' -and @(Get-ChildItem -LiteralPath (Join-Path $Out 'zip31') -Filter '*.tar.gz').Count -eq 0 }
  $zipFallbackSummary = Get-Content -LiteralPath $script:Summary.FullName -Raw | ConvertFrom-Json
  Check '#31 zip fallback: on-disk summary names the zip and ZipFile' { $zipFallbackSummary.archive -eq $script:Archive.Name -and $zipFallbackSummary.capabilities.archiver -eq 'ZipFile' }
  Check '#31 zip fallback: archived summary matches on-disk' {
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $zip = [System.IO.Compression.ZipFile]::OpenRead($script:Archive.FullName)
    try {
      $entry = $zip.Entries | Where-Object { $_.FullName -eq 'collection.json' } | Select-Object -First 1
      $reader = New-Object System.IO.StreamReader($entry.Open())
      try { $collectionText = $reader.ReadToEnd() } finally { $reader.Dispose() }
    } finally { $zip.Dispose() }
    $collectionText -eq [System.IO.File]::ReadAllText($script:Summary.FullName)
  }
  Check '#31 zip fallback: sidecar names the zip' { (Get-Content -LiteralPath "$($script:Archive.FullName).sha256").EndsWith('  ' + $script:Archive.Name) }
  Check '#31 -Project values recorded in options.projects' { $zipFallbackSummary.options.no_projects -eq $false -and @($zipFallbackSummary.options.projects).Count -eq 2 -and $zipFallbackSummary.options.projects[0] -eq $projectOne -and $zipFallbackSummary.options.projects[1] -eq $projectTwo }
  Check '#72 zip fallback: notes give the tar exit code' { @($zipFallbackSummary.notes | Where-Object { $_ -match '^tar(\.exe)? failed \(exit code 2\); fell back to ZipFile$' }).Count -eq 1 }
  # 2026-10-07 (#72)
  # The same failing tar with the default chain falls back to the PowerShell tar
  # writer, and the summary inside the archive says so.
  $env:PATH = $shim + [System.IO.Path]::PathSeparator + $env:PATH
  try { Run 'writer31' @{ NoDocker = $true } } finally { $env:PATH = $oldPath }
  $writerFallbackSummary = Get-Content -LiteralPath $script:Summary.FullName -Raw | ConvertFrom-Json
  $writerFallbackLog = [System.IO.File]::ReadAllText((Get-ChildItem -LiteralPath (Join-Path $Out 'writer31') -Filter '*.log' | Select-Object -First 1).FullName)
  Check '#72 tar fails: the PowerShell tar writer writes the .tar.gz' { $script:ExitCode -eq 0 -and $script:Archive.Name -like '*.tar.gz' -and $writerFallbackSummary.archive -eq $script:Archive.Name -and $writerFallbackSummary.capabilities.archiver -eq 'PowerShell tar writer' }
  Check '#72 tar fails: notes and log give the exit code' { @($writerFallbackSummary.notes | Where-Object { $_ -match '^tar(\.exe)? failed \(exit code 2\); fell back to PowerShell tar writer$' }).Count -eq 1 -and $writerFallbackLog -match '(?m)^tar(\.exe)? failed: exit code 2\r?$' }
  Check '#72 tar fails: the archived summary matches the one on disk' {
    $writerFallbackTar = Read-TarGz $script:Archive.FullName
    @($writerFallbackTar.members | Where-Object { $_.name -eq './collection.json' -and $_.sha256 -eq (Get-FileHash -LiteralPath $script:Summary.FullName -Algorithm SHA256).Hash.ToLower() }).Count -eq 1
  }
} else { Write-Output 'note: no where.exe to stand in for tar.exe, zip fallback checks skipped' }
Run 'noproj31' @{ NoProjects = $true }
$noProjectsSummary = Get-Content -LiteralPath $script:Summary.FullName -Raw
Check '#31 -NoProjects recorded' { ($noProjectsSummary | ConvertFrom-Json).options.no_projects -eq $true -and $noProjectsSummary -match '"no_projects":\s*true,\s*"projects":\s*\[\s*\]' }

# 2026-10-04 bug round: #30
# Symlinks are recreated in the archive with their target string verbatim
# under tar, and kept out of a zip, whose writer would follow them. A separate
# image so the default run's counts and notes stay as they are. On Windows a
# junction, which needs no privilege, gives a reparse point to recreate even
# where symlinks cannot be made; where links cannot be created in the stage
# the row must say so in error and claim no archive_path.
$LinkRoot = Join-Path $Work 'linkroot'
function LinkImagePath([string]$relativePath) { return (Join-Path $LinkRoot ($relativePath.Replace('/', $Separator))) }
function MakeLinkFile([string]$relativePath, [string]$content) {
  $path = ExtendedPath (LinkImagePath $relativePath)
  [void][System.IO.Directory]::CreateDirectory([System.IO.Path]::GetDirectoryName($path))
  [System.IO.File]::WriteAllText($path, $content, (New-Object System.Text.UTF8Encoding $false))
}
MakeLinkFile 'home/eve/.gemini/antigravity-cli/log/real.log' 'LINK-TARGET-BYTES'
MakeLinkFile 'outside/secret.txt' 'OUTSIDE-BYTES'
$onWindows = ($env:OS -eq 'Windows_NT')
$fileLinkOk = $false
# .NET 6+ only, so looked up by reflection; 5.1's New-Item would resolve the
# relative target and refuse the dangling one.
$fileLinkMethod = [System.IO.File].GetMethod('CreateSymbolicLink', [Type[]]@([string], [string]))
$dirLinkMethod = [System.IO.Directory].GetMethod('CreateSymbolicLink', [Type[]]@([string], [string]))
# Invoke rejects PSObject-wrapped strings, so the arguments are cast.
function MakeLink($method, [string]$relativePath, [string]$target) {
  $arguments = New-Object 'object[]' 2; $arguments[0] = [string](ExtendedPath (LinkImagePath $relativePath)); $arguments[1] = $target
  [void]$method.Invoke($null, $arguments)
}
try {
  if ($null -eq $fileLinkMethod -or $null -eq $dirLinkMethod) { throw 'no symlink API' }
  MakeLink $fileLinkMethod 'home/eve/.gemini/antigravity-cli/cli.log' 'log/real.log'
  MakeLink $fileLinkMethod 'home/eve/.gemini/antigravity-cli/dangling.log' 'log/missing.log'
  MakeLink $dirLinkMethod 'home/eve/.gemini/antigravity-cli/out' '../../../../outside'
  $fileLinkOk = $true
} catch { Write-Output 'note: symlink creation not permitted here, #30 symlink-in-archive checks skipped' }
$junctionOk = $false
if ($onWindows) {
  try { New-Item -ItemType Junction -Path (LinkImagePath 'home/eve/.gemini/antigravity-cli/jn') -Target (LinkImagePath 'outside') -ErrorAction Stop | Out-Null; $junctionOk = $true }
  catch { Write-Output 'note: junction creation failed, #30 junction checks skipped' }
  # A % in the name would be expanded by cmd.exe, so 5.1 must not run mklink for it.
  if ($junctionOk) { New-Item -ItemType Junction -Path (LinkImagePath 'home/eve/.gemini/antigravity-cli/jn%TEMP%x') -Target (LinkImagePath 'outside') | Out-Null }
}
$isDesktop = ($PSVersionTable.PSEdition -eq 'Desktop')
function RunLinkImage([string]$name) {
  $outputDir = Join-Path $Out $name
  New-Item -ItemType Directory -Path $outputDir -Force | Out-Null
  $global:LASTEXITCODE = 0
  $script:stdout = & $Collector -Root $LinkRoot -OutputDir $outputDir -Quiet -NoDocker 2>&1 | Out-String
  $script:ExitCode = $LASTEXITCODE
  $script:Manifest = Get-ChildItem -LiteralPath $outputDir -Filter '*.manifest.jsonl' | Select-Object -First 1
  $script:Archive = Get-ChildItem -LiteralPath $outputDir | Where-Object { $_.Name -match '\.(tar\.gz|zip)$' } | Select-Object -First 1
  $script:Summary = Get-ChildItem -LiteralPath $outputDir -Filter '*.collection.json' | Select-Object -First 1
  $script:Rows = @()
  if ($script:Manifest) { $script:Rows = @(Get-Content -LiteralPath $script:Manifest.FullName | ForEach-Object { $_ | ConvertFrom-Json }) }
  $script:LinkSummary = $null
  if ($script:Summary) { $script:LinkSummary = Get-Content -LiteralPath $script:Summary.FullName -Raw | ConvertFrom-Json }
}
# Bytes of every regular file in the archive, concatenated, to show that no
# link target's content was pulled in.
function ArchiveText {
  $extracted = Join-Path $Work ('lx-' + [guid]::NewGuid().ToString('N').Substring(0, 6))
  New-Item -ItemType Directory -Path $extracted -Force | Out-Null
  if ($script:Archive.Name -like '*.tar.gz') { & tar -xzf $script:Archive.FullName -C $extracted 2>$null } else { [System.IO.Compression.ZipFile]::ExtractToDirectory($script:Archive.FullName, $extracted) }
  $text = ''
  foreach ($file in @(Get-ChildItem -LiteralPath $extracted -Recurse -Force -File | Where-Object { ($_.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -eq 0 })) { $text += [System.IO.File]::ReadAllText($file.FullName) }
  return $text
}
function LinkRowsConsistent {
  $bad = @($script:Rows | Where-Object { $_.status -eq 'symlink' -and -not (($_.archive_path -ne '') -xor ($_.error -ne '')) })
  return ($bad.Count -eq 0)
}

if ($fileLinkOk -or $junctionOk) {
  # The same checks for tar and, forced with CAC_ARCHIVER, for the
  # PowerShell tar writer, which stores the target as staged.
  foreach ($linkRunName in @('links-tar', 'links-writer')) {
    $tag = ''
    if ($linkRunName -eq 'links-writer') { $tag = ' [writer]'; $env:CAC_ARCHIVER = 'writer' }
    try { RunLinkImage $linkRunName } finally { if ($linkRunName -eq 'links-writer') { Remove-Item Env:CAC_ARCHIVER } }
    Check ('#30 tar run exit 0' + $tag) { $script:ExitCode -eq 0 }
    Check ('#30 every symlink row has archive_path or error, not both' + $tag) { LinkRowsConsistent }
    $tarVerbose = @()
    if ($script:Archive.Name -like '*.tar.gz') { $tarVerbose = @(& tar -tvzf $script:Archive.FullName 2>$null) }
    if ($fileLinkOk) {
      $row = Row 'antigravity-cli/cli.log'
      Check ('#30 symlink row has archive_path and no error' + $tag) { $row -and $row.status -eq 'symlink' -and $row.archive_path -match '^fs/.*antigravity-cli/cli\.log$' -and $row.error -eq '' }
      Check ('#30 archive stores the link with its relative target verbatim' + $tag) { @($tarVerbose | Where-Object { $_ -match '^l' -and $_ -match 'antigravity-cli/cli\.log -> log/real\.log$' }).Count -eq 1 }
      Check ('#30 dangling symlink stored as a link' + $tag) { @($tarVerbose | Where-Object { $_ -match '^l' -and $_ -match 'antigravity-cli/dangling\.log -> log/missing\.log$' }).Count -eq 1 }
      Check ('#30 directory symlink stored as a link, not descended' + $tag) { @($tarVerbose | Where-Object { $_ -match '^l' -and $_ -match 'antigravity-cli/out -> \.\./\.\./\.\./\.\./outside$' }).Count -eq 1 -and -not ($tarVerbose | Where-Object { $_ -match 'antigravity-cli/out/' }) }
      Check ('#30 link target bytes outside the collection not archived' + $tag) { (ArchiveText) -notmatch 'OUTSIDE-BYTES' }
      Check ('#30 staging removed and link targets intact' + $tag) { -not (Get-ChildItem -LiteralPath (Join-Path $Out $linkRunName) -Filter '.stage-*' -Force) -and (Test-Path -LiteralPath (LinkImagePath 'outside/secret.txt')) -and (Test-Path -LiteralPath (LinkImagePath 'home/eve/.gemini/antigravity-cli/log/real.log')) }
    }
    if ($junctionOk) {
      $row = Row 'antigravity-cli/jn'
      Check ('#30 junction row has archive_path or a reason' + $tag) { $row -and $row.status -eq 'symlink' -and (($row.archive_path -ne '' -and $row.error -eq '') -or ($row.archive_path -eq '' -and $row.error -like 'not recreated in the archive:*')) }
      Check ('#30 junction failure noted in collection.json' + $tag) { $row.archive_path -ne '' -or @($script:LinkSummary.notes | Where-Object { $_ -like '*could not be recreated in the archive*' }).Count -eq 1 }
      Check ('#30 junction target bytes not archived' + $tag) { (ArchiveText) -notmatch 'OUTSIDE-BYTES' }
      Check ('#30 junction target intact after staging cleanup' + $tag) { Test-Path -LiteralPath (LinkImagePath 'outside/secret.txt') }
      if ($isDesktop) {
        Check ('#30 5.1: a failed mklink reports its message and exit code' + $tag) { $row.archive_path -ne '' -or $row.error -match '\(mklink exit \d+\)$' }
        $row = Row 'antigravity-cli/jn%TEMP%x'
        Check ('#30 5.1: a % in the link path is refused before cmd.exe' + $tag) { $row -and $row.archive_path -eq '' -and $row.error -like '*contains % or "*' }
      }
    }
    if ($linkRunName -eq 'links-writer') { Check '#72 links-writer: archiver PowerShell tar writer' { $script:LinkSummary.capabilities.archiver -eq 'PowerShell tar writer' -and $script:Archive.Name -like '*.tar.gz' } }
  }

  # zip from the start, forced with CAC_ARCHIVER (with no tar on the PATH the
  # PowerShell tar writer is used)
  $savedPath = $env:PATH
  $env:CAC_ARCHIVER = 'zip'
  try { RunLinkImage 'links-zip' } finally { Remove-Item Env:CAC_ARCHIVER }
  Check '#30 zip run exit 0 and wrote a zip' { $script:ExitCode -eq 0 -and $script:Archive.Name -like '*.zip' }
  Check '#30 zip: symlink rows have no archive_path and say zip' { $links = @($script:Rows | Where-Object { $_.status -eq 'symlink' }); $links.Count -ge 1 -and @($links | Where-Object { $_.archive_path -ne '' -or $_.error -notlike '*zip cannot store symlinks*' }).Count -eq 0 }
  Check '#30 zip: collection.json notes say symlinks are manifest-only' { @($script:LinkSummary.notes | Where-Object { $_ -like '*in the manifest only: the zip archive cannot store symlinks*' }).Count -eq 1 }
  Check '#30 zip: no link target content in the zip' { $text = ArchiveText; $text -notmatch 'OUTSIDE-BYTES' -and ([regex]::Matches($text, 'LINK-TARGET-BYTES')).Count -eq 1 }

  # tar fails late: a tar on PATH that exits 1, then the zip fall back,
  # with the PowerShell tar writer left out of the chain by CAC_ARCHIVER
  if (-not $onWindows) {
    $shim = Join-Path $Work 'badtar'
    New-Item -ItemType Directory -Path $shim -Force | Out-Null
    [System.IO.File]::WriteAllText((Join-Path $shim 'tar'), "#!/bin/sh`nexit 1`n")
    & chmod +x (Join-Path $shim 'tar')
    $env:PATH = $shim
    $env:CAC_ARCHIVER = 'tar,zip'
    try { RunLinkImage 'links-late' } finally { $env:PATH = $savedPath; Remove-Item Env:CAC_ARCHIVER }
    Check '#30 late fallback exit 0 and wrote a zip' { $script:ExitCode -eq 0 -and $script:Archive.Name -like '*.zip' }
    Check '#30 late fallback: symlink rows rewritten without archive_path' { $links = @($script:Rows | Where-Object { $_.status -eq 'symlink' }); $links.Count -ge 1 -and @($links | Where-Object { $_.archive_path -ne '' -or $_.error -notlike '*tar failed and zip cannot store symlinks*' }).Count -eq 0 }
    Check '#30 late fallback: manifest in the zip matches the one on disk' {
      $zip = [System.IO.Compression.ZipFile]::OpenRead($script:Archive.FullName)
      try { $entry = $zip.GetEntry('manifest.jsonl'); $reader = New-Object System.IO.StreamReader($entry.Open()); $inZip = $reader.ReadToEnd(); $reader.Close() } finally { $zip.Dispose() }
      $inZip -eq [System.IO.File]::ReadAllText($script:Manifest.FullName)
    }
    Check '#30 x #31 late fallback: collection.json in the zip matches the one on disk and names the zip' {
      $zip = [System.IO.Compression.ZipFile]::OpenRead($script:Archive.FullName)
      try { $entry = $zip.GetEntry('collection.json'); $reader = New-Object System.IO.StreamReader($entry.Open()); $inZip = $reader.ReadToEnd(); $reader.Close() } finally { $zip.Dispose() }
      $inZip -eq [System.IO.File]::ReadAllText($script:Summary.FullName) -and $script:LinkSummary.archive -eq $script:Archive.Name -and $script:LinkSummary.capabilities.archiver -eq 'ZipFile'
    }
    Check '#30 late fallback: collection.json notes the removed links' { @($script:LinkSummary.notes | Where-Object { $_ -like 'tar failed;*removed from the staging tree before zipping*' }).Count -eq 1 }
    Check '#30 late fallback: no link target content in the zip' { $text = ArchiveText; $text -notmatch 'OUTSIDE-BYTES' -and ([regex]::Matches($text, 'LINK-TARGET-BYTES')).Count -eq 1 }
    Check '#30 late fallback: link targets intact' { Test-Path -LiteralPath (LinkImagePath 'outside/secret.txt') }
  }
}

# 2026-10-04 bug round: #32, #38, #39
# A relative -Project is resolved against the current location (image mode
# too); a -Project that does not exist gets an error_read row and a log line.
MakeFile 'srv/relproj/AGENTS.md' '# rel'
$missingProject = Join-Path $Work 'nonexistent-cac'
$previousLocation = Get-Location
Set-Location -LiteralPath (ImagePath 'srv')
Run 'projargs' @{ Project = @('relproj', $missingProject) }
Set-Location -LiteralPath $previousLocation.Path
$sum = Get-Content -LiteralPath $script:Summary.FullName -Raw | ConvertFrom-Json
Check '-Project arguments exit 0' { $script:ExitCode -eq 0 }
Check 'relative -Project resolved against the current location' { (StatusOf 'srv/relproj/AGENTS.md') -eq 'collected' -and @($sum.projects | Where-Object { $_ -eq (ImagePath 'srv/relproj') }).Count -eq 1 }
Check '#31 x #39 options.projects records the resolved -Project values' { @($sum.options.projects).Count -eq 2 -and $sum.options.projects[0] -eq (ImagePath 'srv/relproj') -and $sum.options.projects[1] -eq $missingProject }
$missingRows = @($script:Rows | Where-Object { $_.path -eq $missingProject })
Check 'missing -Project gets one error_read dir row' { $missingRows.Count -eq 1 -and $missingRows[0].status -eq 'error_read' -and $missingRows[0].type -eq 'dir' -and $missingRows[0].home -eq $missingProject -and $missingRows[0].agent -eq '' -and $missingRows[0].archive_path -eq '' -and $missingRows[0].error -ne '' }
$projectArgsLog = Get-ChildItem -LiteralPath (Join-Path $Out 'projargs') -Filter '*.log' | Select-Object -First 1
Check 'missing -Project logged' { (Get-Content -LiteralPath $projectArgsLog.FullName -Raw).Contains("WARNING: project given with -Project is not a directory: ${missingProject}: ") }
Check 'missing -Project not listed in projects' { @($sum.projects | Where-Object { $_ -like '*nonexistent-cac*' }).Count -eq 0 }
Check 'missing -Project counted as error_read' { $sum.counts.error_read -eq 1 -and $script:stdout -match '(?m)^errors:     1 \(0 error_copy, 1 error_read\)\s*$' }
$defaultRunCounts = Get-Content -LiteralPath (Get-ChildItem -LiteralPath (Join-Path $Out 'default') -Filter '*.collection.json' | Select-Object -First 1).FullName -Raw | ConvertFrom-Json
Check 'default run has no error_read rows' { $defaultRunCounts.counts.error_read -eq 0 }
Remove-Item -LiteralPath (ImagePath 'srv/relproj') -Recurse -Force

# A read-only output directory exits 2 before any collection work: chmod on
# POSIX (not as root), a deny ACE for creating files on Windows.
$readOnly = Join-Path $Out 'readonly'
New-Item -ItemType Directory -Path $readOnly -Force | Out-Null
$readOnlySet = $false
if ($Separator -eq '/') {
  if ((& id -u) -ne '0') { & chmod 555 $readOnly; $readOnlySet = $true }
} else {
  $acl = Get-Acl -LiteralPath $readOnly
  $denyRule = New-Object System.Security.AccessControl.FileSystemAccessRule([Security.Principal.WindowsIdentity]::GetCurrent().User, 'CreateFiles, AppendData', 'Deny')
  $acl.AddAccessRule($denyRule); Set-Acl -LiteralPath $readOnly -AclObject $acl; $readOnlySet = $true
}
if ($readOnlySet) {
  # Start-Process writes the child's stderr to the file as is. With 2> the
  # 5.1 parent wraps each stderr line in a NativeCommandError record, and its
  # position and category lines land inside a message the child wrapped at
  # the console width. Whitespace is stripped before matching, so a
  # wrap anywhere in the message or the path still matches.
  $readOnlyError = Join-Path $Out 'readonly.stderr'
  $readOnlyArgs = '-NoProfile -ExecutionPolicy Bypass -File "{0}" -Root "{1}" -OutputDir "{2}" -Quiet -NoDocker' -f $Collector, $Root, $readOnly
  $readOnlyProcess = Start-Process -FilePath $self -ArgumentList $readOnlyArgs -NoNewWindow -PassThru -RedirectStandardError $readOnlyError -RedirectStandardOutput (Join-Path $Out 'readonly.stdout')
  $null = $readOnlyProcess.Handle; $readOnlyProcess.WaitForExit(); $readOnlyExit = $readOnlyProcess.ExitCode
  if ($Separator -eq '/') { & chmod 755 $readOnly } else { $acl = Get-Acl -LiteralPath $readOnly; [void]$acl.RemoveAccessRule($denyRule); Set-Acl -LiteralPath $readOnly -AclObject $acl }
  $readOnlyText = (Get-Content -LiteralPath $readOnlyError -Raw) -replace '\s', ''
  $readOnlyWant = ('Output dir not writable: ' + $readOnly) -replace '\s', ''
  Check 'read-only output dir exits 2 before collecting' { $readOnlyExit -eq 2 -and $readOnlyText.Contains($readOnlyWant) -and @(Get-ChildItem -LiteralPath $readOnly -Force).Count -eq 0 }
} else { Write-Output 'note: read-only output dir check needs a non-root user on POSIX, skipped' }
# 2026-10-14 output directory under a file (#108)
# The one error, written by Write-Error (whitespace stripped as above), with no
# Resolve-Path or StrictMode error and no writability message after it.
$outputUnderFile = Join-Path $plainFile 'out'
$outputUnderFileError = Join-Path $Out 'file108.stderr'
$outputUnderFileArgs = '-NoProfile -ExecutionPolicy Bypass -File "{0}" -Root "{1}" -OutputDir "{2}" -Quiet -NoDocker' -f $Collector, $Root, $outputUnderFile
$outputUnderFileProcess = Start-Process -FilePath $self -ArgumentList $outputUnderFileArgs -NoNewWindow -PassThru -RedirectStandardError $outputUnderFileError -RedirectStandardOutput (Join-Path $Out 'file108.stdout')
$null = $outputUnderFileProcess.Handle; $outputUnderFileProcess.WaitForExit()
$outputUnderFileText = [string](Get-Content -LiteralPath $outputUnderFileError -Raw) -replace '\s', ''
$outputUnderFileWant = ('Cannot create output dir: ' + $outputUnderFile) -replace '\s', ''
Check '#108 output dir under a file exits 2 with only the create error' { $outputUnderFileProcess.ExitCode -eq 2 -and $outputUnderFileText.Contains($outputUnderFileWant) -and ($outputUnderFileText.Length - $outputUnderFileText.Replace('Cannotcreateoutputdir', '').Length) -eq 'Cannotcreateoutputdir'.Length -and $outputUnderFileText -notmatch 'Resolve-Path|ProviderPath|Outputdirnotwritable' -and [System.IO.File]::Exists($plainFile) }
# 2026-10-14 exclusion globs and the working directory (#109)
Push-Location -LiteralPath $globCwd
try { Run 'cwd109' @{ NoDocker = $true; NoProjects = $true } } finally { Pop-Location }
Check '#109 wildcard exclusion holds from a directory with a matching tree' { $script:ExitCode -eq 0 -and (StatusOf 'Users/alice/.claude/plugins/y/node_modules') -eq 'skipped_excluded' -and -not ($script:Rows | Where-Object { ($_.path -replace '\\', '/') -like '*node_modules/f.js' }) }
# 2026-10-14 CDPATH in the environment (#110)
# sh only, as PowerShell has no CDPATH; the check is in smoke.sh.

# An unreadable home and an unlistable directory inside a walk: POSIX and not
# root only, since chmod does not deny on Windows or to root.
if ($Separator -eq '/' -and (& id -u) -ne '0') {
  New-Item -ItemType Directory -Path (ImagePath 'home/erin/.claude') -Force | Out-Null
  MakeFile 'Users/alice/.claude/locked/f' 'x'
  & chmod 000 (ImagePath 'home/erin') (ImagePath 'Users/alice/.claude/locked')
  Run 'readperm' @{}
  & chmod 700 (ImagePath 'home/erin') (ImagePath 'Users/alice/.claude/locked')
  $sum = Get-Content -LiteralPath $script:Summary.FullName -Raw | ConvertFrom-Json
  Check 'unreadable paths exit 0' { $script:ExitCode -eq 0 }
  $erinRows = @($script:Rows | Where-Object { $_.path -eq (ImagePath 'home/erin') })
  Check 'unreadable home gets one error_read row' { $erinRows.Count -eq 1 -and $erinRows[0].status -eq 'error_read' -and $erinRows[0].user -eq 'erin' -and $erinRows[0].agent -eq '' -and $erinRows[0].type -eq 'dir' -and $erinRows[0].size -eq 0 -and $erinRows[0].archive_path -eq '' -and $erinRows[0].error -match 'denied' }
  Check 'unreadable home stays in users, not in users_with_artifacts' { @($sum.users) -contains 'erin' -and -not (@($sum.users_with_artifacts) -contains 'erin') }
  $lockedRow = Row 'Users/alice/.claude/locked'
  Check 'unlistable directory in a walk gets an error_read row' { $lockedRow -and $lockedRow.status -eq 'error_read' -and $lockedRow.agent -eq 'claude-code' -and $lockedRow.type -eq 'dir' -and $lockedRow.error -match 'denied' }
  Check 'rest of the home still collected' { (StatusOf 'Users/alice/.claude/projects/-C-proj/s1.jsonl') -eq 'collected' }
  Check 'unreadable paths counted' { $sum.counts.error_read -eq 2 -and $script:stdout -match '(?m)^errors:     2 \(0 error_copy, 2 error_read\)\s*$' }
  Remove-Item -LiteralPath (ImagePath 'home/erin'), (ImagePath 'Users/alice/.claude/locked') -Recurse -Force
} else { Write-Output 'note: unreadable home and directory checks need a non-root POSIX host, skipped' }

# 2026-10-04 bug round: #29 x #32
# An unreadable catalog match gets one error_read row, and a case variant
# folded into it (case-insensitive filesystem) gets none of its own: chmod on
# POSIX (not as root), a deny ACE for listing on Windows.
$foldedDir = ImagePath 'Users/alice/.config/pearai/User'
$foldedSet = $false
# An enabled SeBackupPrivilege (an administrator's token in a Windows OpenSSH
# session) lets the listing through the deny ACE, so the block is skipped
# there, as it is for root on POSIX; on CI the privilege is present but
# disabled. Read from whoami /priv, whose last column is the state.
$backupPrivilege = $false
if ($Separator -ne '/') {
  try { $backupPrivilege = @(& whoami.exe /priv 2>$null | Where-Object { $_ -match '^\s*SeBackupPrivilege\s.*\sEnabled\s*$' }).Count -gt 0 } catch { }
}
if ($Separator -eq '/') {
  if ((& id -u) -ne '0') { & chmod 000 $foldedDir; $foldedSet = $true }
} elseif ($backupPrivilege) {
  Write-Output 'note: SeBackupPrivilege is enabled in this session and bypasses the deny ACE, #29 x #32 unreadable case-variant check skipped'
} else {
  $foldedAcl = Get-Acl -LiteralPath $foldedDir
  $foldedRule = New-Object System.Security.AccessControl.FileSystemAccessRule([Security.Principal.WindowsIdentity]::GetCurrent().User, 'ListDirectory', 'Deny')
  $foldedAcl.AddAccessRule($foldedRule); Set-Acl -LiteralPath $foldedDir -AclObject $foldedAcl; $foldedSet = $true
}
if ($foldedSet) {
  try { Run 'fold32' @{ NoDocker = $true } }
  finally {
    if ($Separator -eq '/') { & chmod 755 $foldedDir } else { $foldedAcl = Get-Acl -LiteralPath $foldedDir; [void]$foldedAcl.RemoveAccessRule($foldedRule); Set-Acl -LiteralPath $foldedDir -AclObject $foldedAcl }
  }
  $foldSummary = Get-Content -LiteralPath $script:Summary.FullName -Raw | ConvertFrom-Json
  $errorReadRows = @($script:Rows | Where-Object { $_.status -eq 'error_read' -and ($_.path -replace '\\', '/').ToLowerInvariant().EndsWith('/.config/pearai/user') })
  Check '#29 x #32 unreadable case-variant match: exactly one error_read row' { $script:ExitCode -eq 0 -and $errorReadRows.Count -eq 1 -and $foldSummary.counts.error_read -eq 1 }
  if ($caseInsensitive) {
    Check '#29 x #32 folded variant has no rows of its own' { @($script:Rows | Where-Object { ($_.path -replace '\\', '/').ToLowerInvariant() -like '*/.config/pearai/user*' }).Count -eq 1 }
  } else {
    Check '#29 x #32 the readable spelling is still collected' { (HasPath 'Users/alice/.config/PearAI/User/case29-upper.json') }
  }
} elseif ($Separator -eq '/') { Write-Output 'note: #29 x #32 unreadable case-variant check needs a non-root user on POSIX, skipped' }

# 2026-10-10 Claude Desktop MSIX build (#73)
# The MSIX build keeps userData in the package's virtualised
# LocalCache\Roaming\Claude, logs in AppData\Local\Claude\logs, and may use
# AppData\Local\Claude-Data\<family>. A separate image holds one Windows
# profile with those trees; the */Claude*/ secret and exclusion globs must
# apply under the new bases, and the Packages secret globs must flag the
# token cache and cookies of both package families.
$script:Made.Clear()
$savedRoot = $Root
$Root = Join-Path $Work 'root73'
$msixPackage = 'Users/alice/AppData/Local/Packages/Claude_pzs8sxrjxfjjc/LocalCache/Roaming/Claude'
$msixPbcPackage = 'Users/alice/AppData/Local/Packages/AnthropicPBC.Claude_fnn82j28hfe8t/LocalCache/Roaming/Claude'
$msixSpaceIds = '11111111-1111-4111-8111-111111111111/22222222-2222-4222-8222-222222222222'
$msixSession = "$msixPackage/claude-code-sessions/$msixSpaceIds/local_44444444-4444-4444-8444-444444444444.json"
$msixAuditKey = "$msixPackage/local-agent-mode-sessions/$msixSpaceIds/local_33333333-3333-4333-8333-333333333333/.audit-key"
$msixSpaces = "$msixPackage/local-agent-mode-sessions/$msixSpaceIds/remote-session-spaces.json"
$msixDataConfig = 'Users/alice/AppData/Local/Claude-Data/claude_pzs8sxrjxfjjc/config.json'
$msixLog = 'Users/alice/AppData/Local/Claude/logs/main.log'
MakeFile "$msixPackage/config.json" '{"oauth:tokenCache":"enc"}'
MakeFile "$msixPackage/Local State" '{"os_crypt":{}}'
MakeFile "$msixPackage/claude_desktop_config.json" '{"mcpServers":{}}'
MakeFile "$msixPackage/Network/Cookies" 'cookies'
MakeFile $msixSession '{"sessionId":"local_44444444-4444-4444-8444-444444444444","cwd":"C:\\proj"}'
MakeFile $msixSpaces '{"entries":[{"sessionId":"0123456789abcdef0123456789abcdef","folders":[]}]}'
MakeFile $msixAuditKey 'key'
MakeFile "$msixPackage/claude-code/2.1.295/abc/claude.exe" 'MZ'
MakeFile "$msixPackage/Cache/data_0" 'cache'
MakeFile "$msixPbcPackage/config.json" '{"oauth:tokenCache":"enc"}'
MakeFile $msixLog '[MSIX] Filesystem virtualization active'
MakeFile $msixDataConfig '{"oauth:tokenCache":"enc"}'
$missing = Test-Made
Check "#73 every MSIX build fixture was created ($($missing.Count) missing$(if ($missing.Count) { ', first ' + $missing[0] }))" { $missing.Count -eq 0 }
function IsDesktopSecretRow([string]$relativePath) { $row = Row $relativePath; return ($row -and $row.agent -eq 'claude-desktop' -and $row.secret -eq $true -and $row.status -eq 'collected') }
function IsDesktopPlainRow([string]$relativePath) { $row = Row $relativePath; return ($row -and $row.agent -eq 'claude-desktop' -and $row.secret -eq $false -and $row.status -eq 'collected') }
try {
  Run 'store73' @{ NoDocker = $true }
  Check '#73 MSIX build run exit 0' { $script:ExitCode -eq 0 }
  Check '#73 Packages Code-tab session record collected as claude-desktop' { IsDesktopPlainRow $msixSession }
  Check '#73 Packages session record collected exactly once' { @($script:Rows | Where-Object { $_.path -like '*local_44444444-4444-4444-8444-444444444444.json' }).Count -eq 1 }
  Check '#73 Packages remote-session-spaces.json collected' { IsDesktopPlainRow $msixSpaces }
  Check '#73 AppData/Local/Claude logs collected as claude-desktop' { IsDesktopPlainRow $msixLog }
  Check '#73 Packages config.json flagged secret (Packages/Claude_* glob)' { IsDesktopSecretRow "$msixPackage/config.json" }
  Check '#73 Packages Network/Cookies flagged secret (Packages/Claude_* glob)' { IsDesktopSecretRow "$msixPackage/Network/Cookies" }
  Check '#73 AnthropicPBC family config.json flagged secret' { IsDesktopSecretRow "$msixPbcPackage/config.json" }
  Check '#73 Claude-Data config.json flagged secret (AppData/Local/Claude* glob)' { IsDesktopSecretRow $msixDataConfig }
  Check '#73 Packages Local State flagged secret (*/Claude*/Local State)' { IsDesktopSecretRow "$msixPackage/Local State" }
  Check '#73 Packages claude_desktop_config.json flagged secret (*/Claude*/claude_desktop_config.json)' { IsDesktopSecretRow "$msixPackage/claude_desktop_config.json" }
  Check '#73 Packages .audit-key flagged secret (*/Claude*/local-agent-mode-sessions/*/.audit-key)' { IsDesktopSecretRow $msixAuditKey }
  Check '#73 Packages claude-code excluded (*/Claude*/claude-code)' { (StatusOf "$msixPackage/claude-code") -eq 'skipped_excluded' -and -not (Row "$msixPackage/claude-code/2.1.295/abc/claude.exe") }
  Check '#73 Packages Cache excluded (*/Claude*/Cache)' { (StatusOf "$msixPackage/Cache") -eq 'skipped_excluded' -and -not (Row "$msixPackage/Cache/data_0") }
  Run 'nosecrets73' @{ NoDocker = $true; NoSecrets = $true }
  Check '#73 NoSecrets run exit 0' { $script:ExitCode -eq 0 }
  Check '#73 NoSecrets skips the Packages and Claude-Data token caches' { (StatusOf "$msixPackage/config.json") -eq 'skipped_secret' -and (StatusOf "$msixPbcPackage/config.json") -eq 'skipped_secret' -and (StatusOf $msixDataConfig) -eq 'skipped_secret' }
  Check '#73 NoSecrets skips Local State, claude_desktop_config.json, .audit-key' { (StatusOf "$msixPackage/Local State") -eq 'skipped_secret' -and (StatusOf "$msixPackage/claude_desktop_config.json") -eq 'skipped_secret' -and (StatusOf $msixAuditKey) -eq 'skipped_secret' }
  Check '#73 NoSecrets still collects the session record' { (StatusOf $msixSession) -eq 'collected' }
  Run 'full73' @{ NoDocker = $true; Full = $true }
  Check '#73 Full run exit 0' { $script:ExitCode -eq 0 }
  Check '#73 Full collects the Packages claude.exe and Cache' { (StatusOf "$msixPackage/claude-code/2.1.295/abc/claude.exe") -eq 'collected' -and (StatusOf "$msixPackage/Cache/data_0") -eq 'collected' }
} finally { $Root = $savedRoot }

if ($script:fail -eq 0) { Write-Output "ALL PASSED ($($PSVersionTable.PSEdition) $($PSVersionTable.PSVersion))"; Remove-Item -LiteralPath (ExtendedPath $Work) -Recurse -Force -ErrorAction SilentlyContinue }
else { Write-Output "FAILURES; work dir kept: $Work" }
exit $script:fail
