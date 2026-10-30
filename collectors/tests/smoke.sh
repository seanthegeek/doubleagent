#!/bin/sh
# Copyright 2026 Sean Whalen
# SPDX-License-Identifier: Apache-2.0
# Smoke test: build a fake disk image, run the collector in image mode under a
# given shell, and check the manifest and archive. Usage:
#   tests/smoke.sh [SHELL]     (SHELL defaults to sh; used for both the script
#                               and the find -exec workers via COLLECTOR_SH)
set -u
SHELL_UNDER_TEST=${1:-sh}
HERE=$(cd "$(dirname "$0")" && pwd)
SCRIPT="$HERE/../collect-agent-artifacts.sh"
# macOS sets TMPDIR with a trailing slash and under the /var -> /private/var
# symlink; strip the slash and resolve the directory so the paths the checks
# build match the manifest's.
_tmpdir=${TMPDIR:-/tmp}
WORK=$(mktemp -d "${_tmpdir%/}/cac-smoke.XXXXXX") && WORK=$(cd "$WORK" && pwd -P) || exit 1
ROOT="$WORK/root"
OUT="$WORK/out"
fail=0
pass()       { printf 'ok   %s\n' "$1"; }
fail_check() { printf 'FAIL %s\n' "$1"; fail=1; }
check() { if eval "$2"; then pass "$1"; else fail_check "$1"; fi; }

# ---- fake image -----------------------------------------------------------
mkdir -p "$ROOT/etc" "$ROOT/home/alice/.claude/projects/-srv-proj" "$ROOT/home/alice/.claude/cache" \
  "$ROOT/home/alice/.gemini/antigravity-cli/conversations" "$ROOT/home/alice/.gemini/antigravity-cli/bin" \
  "$ROOT/Users/bob/Library/Application Support/Cursor/User/globalStorage/ms-python.python" \
  "$ROOT/Users/bob/Library/Application Support/Cursor/User/workspaceStorage/abc" \
  "$ROOT/Users/bob/.codex/sessions/2026/10/03" "$ROOT/srv/proj/.claude" "$ROOT/Users/bob/Documents/app" \
  "$ROOT/home/ollama/.ollama/models/blobs" "$ROOT/nonexistent" "$ROOT/home/dave"
cat >"$ROOT/etc/passwd" <<EOF
root:x:0:0:root:/root:/bin/sh
alice:x:1000:1000:Alice:/home/alice:/bin/bash
ollama:x:999:999::/home/ollama:/usr/sbin/nologin
nobody:x:65534:65534::/nonexistent:/usr/sbin/nologin
dave:x:1001:1001:Dave:/home/dave:/bin/sh
EOF
printf '{"type":"user","message":{"role":"user","content":"hi"},"timestamp":"2026-10-03T00:00:00Z"}\n' >"$ROOT/home/alice/.claude/projects/-srv-proj/s1.jsonl"
printf '{"display":"do things","timestamp":1759449600000,"project":"/srv/proj","sessionId":"s1"}\n' >"$ROOT/home/alice/.claude/history.jsonl"
printf '{"claudeAiOauth":{"accessToken":"sk-ant-oat-SECRET"}}\n' >"$ROOT/home/alice/.claude/.credentials.json"
printf '{"projects":{"/srv/proj":{}}}\n' >"$ROOT/home/alice/.claude.json"
printf 'cached junk\n' >"$ROOT/home/alice/.claude/cache/junk.bin"
printf 'weird "name" with spaces and a \\ backslash\n' >"$ROOT/home/alice/.claude/weird \"name\" with spaces \\ bs.txt"
printf 'token\n' >"$ROOT/home/alice/.gemini/antigravity-cli/antigravity-oauth-token"
printf 'db\n' >"$ROOT/home/alice/.gemini/antigravity-cli/conversations/c1.db"
head -c 300000 /dev/zero >"$ROOT/home/alice/.gemini/antigravity-cli/bin/webm_encoder"
ln -s log/cli-1.log "$ROOT/home/alice/.gemini/antigravity-cli/cli.log"
printf 'export FOO=1\n' >"$ROOT/home/alice/.bash_history"
printf 'settings\n' >"$ROOT/srv/proj/.claude/settings.local.json"
printf '# proj\n' >"$ROOT/srv/proj/CLAUDE.md"
head -c 2100000 /dev/zero >"$ROOT/Users/bob/Library/Application Support/Cursor/User/globalStorage/state.vscdb"
printf 'junk\n' >"$ROOT/Users/bob/Library/Application Support/Cursor/User/globalStorage/ms-python.python/x"
printf '{"folder":"file:///Users/bob/Documents/app"}\n' >"$ROOT/Users/bob/Library/Application Support/Cursor/User/workspaceStorage/abc/workspace.json"
printf 'rules\n' >"$ROOT/Users/bob/Documents/app/.cursorrules"
printf '{"cwd":"/Users/bob/Documents/app"}\n' >"$ROOT/Users/bob/.codex/sessions/2026/10/03/rollout-1.jsonl"
printf 'blob\n' >"$ROOT/home/ollama/.ollama/models/blobs/sha256-abc"
printf 'hist\n' >"$ROOT/home/ollama/.ollama/history"
# --- source-verified layouts for the per-platform catalog entries ---
mkdir -p "$ROOT/home/alice/.continue/sessions" "$ROOT/home/alice/.continue/index/lancedb" "$ROOT/srv/cont" \
  "$ROOT/home/alice/.local/share/crush" "$ROOT/srv/crushproj/.crush" "$ROOT/srv/gem" "$ROOT/home/alice/.gemini/tmp/gem" \
  "$ROOT/home/alice/.cline/data/settings" "$ROOT/home/alice/.cline/data/sessions/s1" "$ROOT/srv/clineproj" \
  "$ROOT/home/alice/.local/share/opencode" \
  "$ROOT/Users/bob/Library/Application Support/Claude/Cache" "$ROOT/Users/bob/Library/Application Support/Zed/extensions" \
  "$ROOT/Users/bob/Library/Application Support/Zed/threads" "$ROOT/Users/bob/.config/zed" \
  "$ROOT/Users/bob/Library/Application Support/Cursor/User/globalStorage/saoudrizwan.claude-dev/state" \
  "$ROOT/Users/bob/Library/Application Support/Cursor/User/globalStorage/saoudrizwan.claude-dev/checkpoints" "$ROOT/Users/bob/clineproj"
printf '[{"sessionId":"a","workspaceDirectory":"/srv/cont"}]\n' >"$ROOT/home/alice/.continue/sessions/sessions.json"
printf '{"mcpOauthStorage":{}}\n' >"$ROOT/home/alice/.continue/index/globalContext.json"
printf 'vec\n' >"$ROOT/home/alice/.continue/index/lancedb/data.lance"
printf '{}\n' >"$ROOT/srv/cont/.continuerc.json"
printf '{"projects":[{"path":"/srv/crushproj","data_dir":"/srv/crushproj/.crush"}]}\n' >"$ROOT/home/alice/.local/share/crush/projects.json"
printf 'sqlite\n' >"$ROOT/srv/crushproj/.crush/crush.db"
printf '{"projects":{"/srv/gem":"gem"}}\n' >"$ROOT/home/alice/.gemini/projects.json"
printf '/srv/gem\n' >"$ROOT/home/alice/.gemini/tmp/gem/.project_root"
printf '# gem\n' >"$ROOT/srv/gem/GEMINI.md"
printf '{"anthropic":{"apiKey":"sk-x"}}\n' >"$ROOT/home/alice/.cline/data/settings/providers.json"
printf '{"id":"s1","cwd":"/srv/clineproj","workspace_root":"/srv/clineproj"}\n' >"$ROOT/home/alice/.cline/data/sessions/s1/s1.json"
printf 'rules\n' >"$ROOT/srv/clineproj/.clinerules"
printf '{"openai":{"type":"api","key":"x"}}\n' >"$ROOT/home/alice/.local/share/opencode/auth.json"
printf 'TOKEN=1\n' >"$ROOT/home/alice/.env"
printf '{"oauth:tokenCache":"enc"}\n' >"$ROOT/Users/bob/Library/Application Support/Claude/config.json"
printf '{"mcpServers":{}}\n' >"$ROOT/Users/bob/Library/Application Support/Claude/claude_desktop_config.json"
printf 'cache\n' >"$ROOT/Users/bob/Library/Application Support/Claude/Cache/index"
printf 'ext\n' >"$ROOT/Users/bob/Library/Application Support/Zed/extensions/foo.wasm"
printf 'db\n' >"$ROOT/Users/bob/Library/Application Support/Zed/threads/threads.db"
printf '{"language_models":{}}\n' >"$ROOT/Users/bob/.config/zed/settings.json"
printf '[{"id":"t1","cwdOnTaskInitialization":"/Users/bob/clineproj"}]\n' >"$ROOT/Users/bob/Library/Application Support/Cursor/User/globalStorage/saoudrizwan.claude-dev/state/taskHistory.json"
printf 'ckpt\n' >"$ROOT/Users/bob/Library/Application Support/Cursor/User/globalStorage/saoudrizwan.claude-dev/checkpoints/x"
printf 'rules\n' >"$ROOT/Users/bob/clineproj/.clinerules"
# --- v1.3 catalog revision: exclusions that must not swallow evidence, new secrets, remote server dirs, discovery sources ---
mkdir -p "$ROOT/home/alice/.factory/cache/session-index" "$ROOT/home/alice/.factory/cache/sounds" "$ROOT/home/alice/.factory/sessions/-srv-droid" "$ROOT/srv/droid" \
  "$ROOT/home/alice/.kiro/powers/installed/p1" "$ROOT/home/alice/.kiro/sessions/cli" "$ROOT/srv/kiroproj" \
  "$ROOT/home/alice/.vscode-server/data/User/globalStorage/saoudrizwan.claude-dev/state" "$ROOT/srv/remoteproj" \
  "$ROOT/home/alice/.gemini/antigravity-ide/conversations" "$ROOT/home/alice/.gemini/config" "$ROOT/srv/agyproj" \
  "$ROOT/home/alice/.cursor/projects/-srv-proj" "$ROOT/home/alice/.copilot/pkg" "$ROOT/home/alice/.copilot/session-state/s1" "$ROOT/srv/copproj" \
  "$ROOT/home/alice/.claude/image-cache" "$ROOT/home/alice/.local/share/amp" "$ROOT/srv/ampproj/.amp" \
  "$ROOT/home/alice/.windsurf/transcripts" "$ROOT/srv/kiloproj/.kilo/worktrees/w1" "$ROOT/Users/bob/Library/Application Support/Ollama"
printf 'sqlite\n' >"$ROOT/home/alice/.factory/cache/session-index/index.db"
printf 'ding\n' >"$ROOT/home/alice/.factory/cache/sounds/done.wav"
printf '{"type":"session_start","cwd":"/srv/droid"}\n' >"$ROOT/home/alice/.factory/sessions/-srv-droid/s1.jsonl"
printf '# droid\n' >"$ROOT/srv/droid/AGENTS.md"
printf '{"mcpServers":{"x":{"env":{"TOKEN":"t"}}}}\n' >"$ROOT/home/alice/.kiro/powers/installed/p1/mcp.json"
printf '{"id":"k1","cwd":"/srv/kiroproj"}\n' >"$ROOT/home/alice/.kiro/sessions/cli/k1.json"
printf 'steer\n' >"$ROOT/srv/kiroproj/.kiroignore"
printf 'sqlite\n' >"$ROOT/home/alice/.vscode-server/data/User/globalStorage/state.vscdb"
printf '[{"id":"t2","cwdOnTaskInitialization":"/srv/remoteproj"}]\n' >"$ROOT/home/alice/.vscode-server/data/User/globalStorage/saoudrizwan.claude-dev/state/taskHistory.json"
printf 'rules\n' >"$ROOT/srv/remoteproj/.clinerules"
printf 'db\n' >"$ROOT/home/alice/.gemini/antigravity-ide/conversations/i1.db"
printf '{"mcpServers":{}}\n' >"$ROOT/home/alice/.gemini/config/mcp_config.json"
printf '{"display":"hi","timestamp":1759449600000,"workspace":"/srv/agyproj"}\n' >"$ROOT/home/alice/.gemini/antigravity-cli/history.jsonl"
printf 'ignore\n' >"$ROOT/srv/agyproj/.antigravityignore"
printf '{"accessToken":"x"}\n' >"$ROOT/home/alice/.cursor/auth.json"
printf '{"tok":"x"}\n' >"$ROOT/home/alice/.cursor/projects/-srv-proj/mcp-auth.json"
printf 'pkg\n' >"$ROOT/home/alice/.copilot/pkg/copilot"
printf '{"token":"x"}\n' >"$ROOT/home/alice/.copilot/config.json"
printf 'cwd: /srv/copproj\nbranch: main\n' >"$ROOT/home/alice/.copilot/session-state/s1/workspace.yaml"
printf '# cop\n' >"$ROOT/srv/copproj/AGENTS.md"
printf '{"k":"x"}\n' >"$ROOT/home/alice/.claude/.device-keys.json"
printf 'img\n' >"$ROOT/home/alice/.claude/image-cache/a.png"
printf '{"cwd":"/srv/ampproj","prompt":"hi"}\n' >"$ROOT/home/alice/.local/share/amp/history.jsonl"
printf 'amp\n' >"$ROOT/srv/ampproj/.amp/settings.json"
printf '{"step":1}\n' >"$ROOT/home/alice/.windsurf/transcripts/t1.jsonl"
printf 'wt\n' >"$ROOT/srv/kiloproj/.kilo/worktrees/w1/f"
printf '{"cwd":"/srv/kiloproj"}\n' >"$ROOT/home/alice/.cline/data/sessions/s1/s2.json"
printf 'db\n' >"$ROOT/Users/bob/Library/Application Support/Ollama/db.sqlite"

# --- 2026-10-04 catalog cases: one per exclusion, secret glob and attributed agent, as
# kind|agent|path relative to alice's home. excl (a directory) and exclf (a
# file) must be skipped_excluded, secret collected and flagged, agent collected
# under that agent. The same list is in smoke.ps1.
CASES_CATALOG='
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
excl||.openclaw/projects/0123456789abcdef
excl||.openclaw/plugin-skills
excl||.openclaw/extensions/e1/node_modules
excl||.openclaw/sandbox/skills-workspaces
excl||.openclaw/cache/control-ui-assets
excl||.openclaw/agents/main/agent/tmp
excl||.openclaw/agents/main/agent/.tmp
excl||.openclaw/agents/main/agent/runs/tmp
excl||.openclaw/agents/main/agent/runs/.tmp
excl||.openclaw/browser/b1/user-data/Default/Cache
excl||.openclaw/browser/b1/user-data/Default/Code Cache
excl||.openclaw/browser/b1/user-data/Default/GPUCache
excl||.openclaw/browser/b1/user-data/Default/Service Worker
excl||.openclaw-work/dev
excl||.openclaw-work/git
excl||.openclaw-work/npm
excl||.openclaw-work/npm-runtime
excl||.openclaw-work/tmp
excl||.openclaw-work/tools
excl||.openclaw-work/worktrees
excl||.openclaw-work/projects/0123456789abcdef
excl||.openclaw-work/plugin-skills
excl||.openclaw-work/extensions/e1/node_modules
excl||.openclaw-work/sandbox/skills-workspaces
excl||.openclaw-work/cache/control-ui-assets
excl||.openclaw-work/agents/main/agent/tmp
excl||.openclaw-work/agents/main/agent/.tmp
excl||.openclaw-work/agents/main/agent/runs/tmp
excl||.openclaw-work/agents/main/agent/runs/.tmp
excl||.openclaw-work/browser/b1/user-data/Default/Cache
excl||.openclaw-work/browser/b1/user-data/Default/Code Cache
excl||.openclaw-work/browser/b1/user-data/Default/GPUCache
excl||.openclaw-work/browser/b1/user-data/Default/Service Worker
excl||.clawdbot/dev
excl||.clawdbot/git
excl||.clawdbot/npm
excl||.clawdbot/npm-runtime
excl||.clawdbot/tmp
excl||.clawdbot/tools
excl||.clawdbot/worktrees
excl||.clawdbot/projects/0123456789abcdef
excl||.clawdbot/plugin-skills
excl||.clawdbot/extensions/e1/node_modules
excl||.clawdbot/sandbox/skills-workspaces
excl||.clawdbot/cache/control-ui-assets
excl||.clawdbot/agents/main/agent/tmp
excl||.clawdbot/agents/main/agent/.tmp
excl||.clawdbot/agents/main/agent/runs/tmp
excl||.clawdbot/agents/main/agent/runs/.tmp
excl||.clawdbot/browser/b1/user-data/Default/Cache
excl||.clawdbot/browser/b1/user-data/Default/Code Cache
excl||.clawdbot/browser/b1/user-data/Default/GPUCache
excl||.clawdbot/browser/b1/user-data/Default/Service Worker
excl||.moltbot/dev
excl||.moltbot/git
excl||.moltbot/npm
excl||.moltbot/npm-runtime
excl||.moltbot/tmp
excl||.moltbot/tools
excl||.moltbot/worktrees
excl||.moltbot/projects/0123456789abcdef
excl||.moltbot/plugin-skills
excl||.moltbot/extensions/e1/node_modules
excl||.moltbot/sandbox/skills-workspaces
excl||.moltbot/cache/control-ui-assets
excl||.moltbot/agents/main/agent/tmp
excl||.moltbot/agents/main/agent/.tmp
excl||.moltbot/agents/main/agent/runs/tmp
excl||.moltbot/agents/main/agent/runs/.tmp
excl||.moltbot/browser/b1/user-data/Default/Cache
excl||.moltbot/browser/b1/user-data/Default/Code Cache
excl||.moltbot/browser/b1/user-data/Default/GPUCache
excl||.moltbot/browser/b1/user-data/Default/Service Worker
secret||.openclaw/openclaw.json
secret||.openclaw/clawdbot.json
secret||.openclaw/moltbot.json
secret||.openclaw/.env
secret||.openclaw/credentials/oauth.json
secret||.openclaw/service-env/gateway.env
secret||.openclaw/client/credentials.sqlite
secret||.openclaw/identity/device.json
secret||.openclaw/exec-approvals.json
secret||.openclaw/agents/main/agent/auth-profiles.json
secret||.openclaw/agents/main/agent/auth-state.json
secret||.openclaw/agents/main/agent/auth.json
secret||.openclaw/agents/main/agent/models.json
secret||.openclaw/browser/b1/user-data/Default/Cookies
secret||.clawdbot/openclaw.json
secret||.clawdbot/clawdbot.json
secret||.clawdbot/moltbot.json
secret||.clawdbot/.env
secret||.clawdbot/credentials/oauth.json
secret||.clawdbot/service-env/gateway.env
secret||.clawdbot/client/credentials.sqlite
secret||.clawdbot/identity/device.json
secret||.clawdbot/exec-approvals.json
secret||.clawdbot/agents/main/agent/auth-profiles.json
secret||.clawdbot/agents/main/agent/auth-state.json
secret||.clawdbot/agents/main/agent/auth.json
secret||.clawdbot/agents/main/agent/models.json
secret||.clawdbot/browser/b1/user-data/Default/Cookies
secret||.moltbot/openclaw.json
secret||.moltbot/clawdbot.json
secret||.moltbot/moltbot.json
secret||.moltbot/.env
secret||.moltbot/credentials/oauth.json
secret||.moltbot/service-env/gateway.env
secret||.moltbot/client/credentials.sqlite
secret||.moltbot/identity/device.json
secret||.moltbot/exec-approvals.json
secret||.moltbot/agents/main/agent/auth-profiles.json
secret||.moltbot/agents/main/agent/auth-state.json
secret||.moltbot/agents/main/agent/auth.json
secret||.moltbot/agents/main/agent/models.json
secret||.moltbot/browser/b1/user-data/Default/Cookies
secret||.openclaw-work/.env
secret||.openclaw/identity/share-device-auth.json
secret||.openclaw/exec-approvals.json.migrated.0123456789abcdef.a1
secret||.openclaw-auth-profile-secrets/gateway.env
secret||.config/openclaw/gateway.env
agent|openclaw|.openclaw/agents/main/agent/openclaw-agent.sqlite
agent|openclaw|.openclaw/agents/main/agent/openclaw-agent.sqlite-wal
agent|openclaw|.openclaw/workspace/tmp/AGENTS.md
agent|openclaw|.openclaw/workspace/tools/TOOLS.md
agent|openclaw|.openclaw-work/openclaw.json
agent|openclaw|.config/systemd/user/openclaw-gateway.service
agent|openclaw|.config/systemd/user/clawdbot-gateway.service
agent|openclaw|Library/LaunchAgents/ai.openclaw.gateway.plist
agent|openclaw|Library/Application Support/OpenClaw/databases/gateway-cache.sqlite
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
'
ALICE="$ROOT/home/alice"; BOB="$ROOT/Users/bob"
printf '%s\n' "$CASES_CATALOG" | while IFS='|' read -r _kind _agent _relpath; do
  [ -n "$_relpath" ] || continue
  case "$_kind" in
    excl) mkdir -p "$ALICE/$_relpath"; printf 'x\n' >"$ALICE/$_relpath/f" ;;
    *) mkdir -p "$ALICE/$(dirname "$_relpath")"; printf 'x\n' >"$ALICE/$_relpath" ;;
  esac
done
# discovery sources, one project each
mkdir -p "$ALICE/.hermes/checkpoints/store/projects" "$ROOT/srv/hermesproj" "$ALICE/.letta/lc-local-backend/conversations/c1" "$ROOT/srv/lettaproj/.letta" "$ROOT/srv/lettaproj2" \
  "$ALICE/.pi/agent/sessions/--srv-piproj--" "$ALICE/.pi/agent/experimental/sessions/s1" "$ROOT/srv/piproj/.pi" "$ROOT/srv/piproj2" "$ROOT/srv/pitrust" "$ROOT/srv/picrash" "$ROOT/srv/piexp" \
  "$ALICE/.openinterpreter/sessions/2026/10/04" "$ALICE/.openinterpreter/archived_sessions/2026/10/01" "$ROOT/srv/oiproj/.openinterpreter/rules" "$ROOT/srv/oiarch/.codewhale" \
  "$ALICE/.openhands/agent-canvas/dev_conversations/a1" "$ALICE/.openhands/agent-canvas/conversations/a2" "$ALICE/.openhands/conversations/a3" \
  "$ROOT/srv/ohproj/.openhands" "$ROOT/srv/ohdev" "$ROOT/srv/ohcanvas" "$ROOT/srv/ohcli" "$ALICE/.pearai/sessions" "$ROOT/srv/pearproj/.pearai-agent" \
  "$ROOT/srv/clawws" "$ROOT/srv/clawagent" "$ROOT/srv/clawlegacy" "$ROOT/srv/clawprofile" "$BOB/.openclaw" \
  "$ALICE/.nanobot/sessions/0123456789abcdef0123456789abcdef" "$ROOT/srv/nanows" "$ROOT/srv/nanocfg/memory" "$ROOT/srv/tabbyproj" \
  "$ALICE/.twinny/embeddings/twinproj" "$ROOT/srv/twinproj/src"
printf '{"workdir":"/srv/hermesproj","created_at":1759536000}\n' >"$ALICE/.hermes/checkpoints/store/projects/0123456789abcdef.json"
printf '# hermes\n' >"$ROOT/srv/hermesproj/.hermes.md"
# 2026-10-30 hermes rebuild (#326)
mkdir -p "$ROOT/srv/hermesproj/.hermes"
printf '# hermes\n' >"$ROOT/srv/hermesproj/HERMES.md"
printf '{}\n' >"$ROOT/srv/hermesproj/.hermes/environment.json"
printf '{"conversations":[]}\n' >"$ROOT/srv/hermesproj/trajectory_samples.jsonl"
printf '{"conversations":[]}\n' >"$ROOT/srv/hermesproj/failed_trajectories.jsonl"
printf '{"type":"start","project":"/srv/lettaproj","agentId":"a1"}\n' >"$ALICE/.letta/sessions.jsonl"
printf '{"hooks":{}}\n' >"$ROOT/srv/lettaproj/.letta/settings.json"
printf '{"type":"session","id":"c1","cwd":"/srv/lettaproj2"}\n' >"$ALICE/.letta/lc-local-backend/conversations/c1/messages.jsonl"
printf '# letta\n' >"$ROOT/srv/lettaproj2/AGENTS.md"
printf '{"type":"session","version":3,"id":"u1","cwd":"/srv/piproj"}\n' >"$ALICE/.pi/agent/sessions/--srv-piproj--/2026-10-04T00-00-00-000Z_u1.jsonl"
printf '{}\n' >"$ROOT/srv/piproj/.pi/settings.json"
printf '{"type":"session","version":1,"id":"u2","cwd":"/srv/piproj2"}\n' >"$ALICE/.pi/agent/2026-10-01T00-00-00-000Z_u2.jsonl"
printf '# pi\n' >"$ROOT/srv/piproj2/AGENTS.md"
printf '{"/srv/pitrust":true}\n' >"$ALICE/.pi/agent/trust.json"
printf '<html></html>\n' >"$ROOT/srv/pitrust/pi-session-2026-10-04.html"
printf '[{"cwd":"/srv/picrash","sessionFile":"x.jsonl"}]\n' >"$ALICE/.pi/agent/crashes.json"
printf '# pi\n' >"$ROOT/srv/picrash/AGENTS.md"
printf '{"id":"s1","cwd":"/srv/piexp"}\n' >"$ALICE/.pi/agent/experimental/sessions/s1/meta.json"
printf '# pi\n' >"$ROOT/srv/piexp/AGENTS.md"
printf '{"type":"session_meta","payload":{"cwd":"/srv/oiproj"}}\n' >"$ALICE/.openinterpreter/sessions/2026/10/04/rollout-2026-10-04T00-00-00-u1.jsonl"
printf 'rule\n' >"$ROOT/srv/oiproj/.openinterpreter/rules/r.md"
printf '{"type":"session_meta","payload":{"cwd":"/srv/oiarch"}}\n' >"$ALICE/.openinterpreter/archived_sessions/2026/10/01/rollout-2026-10-01T00-00-00-u2.jsonl"
printf 'notes\n' >"$ROOT/srv/oiarch/.codewhale/notes.md"
printf '{"workspaces":[{"id":"w1","path":"/srv/ohproj"}],"workspaceParents":[]}\n' >"$ALICE/.openhands/workspaces.json"
printf 'echo setup\n' >"$ROOT/srv/ohproj/.openhands/setup.sh"
printf '{"workspace":{"working_dir":"/srv/ohdev"}}\n' >"$ALICE/.openhands/agent-canvas/dev_conversations/a1/meta.json"
printf '{"workspace":{"working_dir":"/srv/ohcanvas"}}\n' >"$ALICE/.openhands/agent-canvas/conversations/a2/meta.json"
printf '{"workspace":{"working_dir":"/srv/ohcli"}}\n' >"$ALICE/.openhands/conversations/a3/base_state.json"
for _dir in ohdev ohcanvas ohcli; do printf '# oh\n' >"$ROOT/srv/$_dir/AGENTS.md"; done
printf '[{"sessionId":"s1","title":"t","workspaceDirectory":"/srv/pearproj"}]\n' >"$ALICE/.pearai/sessions/sessions.json"
printf '{}\n' >"$ROOT/srv/pearproj/.pearairc.json"
printf '{"mcpServers":{}}\n' >"$ROOT/srv/pearproj/.pearai-agent/mcp.json"
printf "{\n  // JSON5\n  agents: {\n    defaults: { workspace: '/srv/clawws' },\n    entries: { main: { agentDir: \"/srv/clawagent\" } },\n  },\n}\n" >"$ALICE/.openclaw/openclaw.json"
printf '{"agents":{"defaults":{"workspace":"/srv/clawlegacy"}}}\n' >"$ALICE/.clawdbot/clawdbot.json"
printf '{agents:{defaults:{workspace:"/srv/clawprofile"}}}\n' >"$ALICE/.openclaw-work/openclaw.json"
printf '# soul\n' >"$ROOT/srv/clawws/SOUL.md"
printf '# a\n' >"$ROOT/srv/clawagent/AGENTS.md"
printf '# id\n' >"$ROOT/srv/clawlegacy/IDENTITY.md"
printf '# mem\n' >"$ROOT/srv/clawprofile/MEMORY.md"
printf '{}\n' >"$BOB/.openclaw/openclaw.json"
ln -s .openclaw "$BOB/.clawdbot"
printf '/srv/nanows' >"$ALICE/.nanobot/sessions/0123456789abcdef0123456789abcdef/.workspace"
printf '# hb\n' >"$ROOT/srv/nanows/HEARTBEAT.md"
printf '{"agents":{"defaults":{"workspace":"/srv/nanocfg"}}}\n' >"$ALICE/.nanobot-work/config.json"
printf '# mem\n' >"$ROOT/srv/nanocfg/memory/MEMORY.md"
printf '[[repositories]]\nname = "p"\ngit_url = "file:///srv/tabbyproj"\n' >"$ALICE/.tabby/config.toml"
printf '# tabby\n' >"$ROOT/srv/tabbyproj/AGENTS.md"
printf '{"files":{"/srv/twinproj/src/a.ts":{"hash":"h1"},"/srv/twinproj/b.ts":{"hash":"h2"}}}\n' >"$ALICE/.twinny/embeddings/twinproj/manifest.json"
printf '# twin\n' >"$ROOT/srv/twinproj/AGENTS.md"

# --- 2026-10-04 Docker volumes: a root volume matched to Agent Zero, an unmatched volume,
# and a rootless volume in alice's home.
VOLUMES="$ROOT/var/lib/docker/volumes"
mkdir -p "$VOLUMES/a0_usr/_data/chats/ctx1" "$VOLUMES/a0_usr/_data/tmp/playwright" "$VOLUMES/pgdata/_data" \
  "$ALICE/.local/share/docker/volumes/tabby_data/_data/events"
printf '{"id":"ctx1","name":"chat","history":"[]"}\n' >"$VOLUMES/a0_usr/_data/chats/ctx1/chat.json"
printf 'API_KEY_OPENAI=sk-x\n' >"$VOLUMES/a0_usr/_data/.env"
printf 'junk\n' >"$VOLUMES/a0_usr/_data/tmp/playwright/junk"
printf 'pg\n' >"$VOLUMES/pgdata/_data/x"
printf '{"event":"completion"}\n' >"$ALICE/.local/share/docker/volumes/tabby_data/_data/events/2026-10-01.json"
printf 'metadata\n' >"$VOLUMES/metadata.db"

# --- 2026-10-04 Muse Code
CASES_MUSE='
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
'
printf '%s\n' "$CASES_MUSE" | while IFS='|' read -r _kind _agent _relpath; do
  [ -n "$_relpath" ] || continue
  case "$_kind" in
    excl) mkdir -p "$ALICE/$_relpath"; printf 'x\n' >"$ALICE/$_relpath/f" ;;
    *) mkdir -p "$ALICE/$(dirname "$_relpath")"; printf 'x\n' >"$ALICE/$_relpath" ;;
  esac
done
printf 'x\n' >"$ALICE/.local/bin/othertool"
MUSE_SESSION="$ALICE/.local/share/muse/sessions/2026/10/04/01a0f000-0000-7000-8000-000000000001"
mkdir -p "$MUSE_SESSION" "$ROOT/srv/museproj/.muse/worktrees/w1" "$ROOT/srv/musetrust"
printf '{"schema_version":1,"projects":{"/srv/musetrust":{"decision":"trusted"}}}\n' >"$ALICE/.config/muse/trust.json"
printf '{"schema_version":1,"stream":{"kind":"session","id":"01a0f000-0000-7000-8000-000000000001"},"payload_type":"runtime.session.metadata","payload":{"kind":"metadata","record":{"model_id":"m","workspace_root":"/srv/museproj"}}}\n' >"$MUSE_SESSION/session.jsonl"
printf '{}\n' >"$ROOT/srv/museproj/.muse/hooks.json"
printf 'x\n' >"$ROOT/srv/museproj/.muse/worktrees/w1/f"
printf '# trusted\n' >"$ROOT/srv/musetrust/AGENTS.md"

# --- 2026-10-04 relocated Claude Code home
# ~/.claude-code-router is another tool's; the .claude-* lines must not claim it.
CLAUDE_WORK="$ALICE/.claude-work"
CLAUDE_WORK_SESSION="$CLAUDE_WORK/projects/-srv-ccwork/0a0b0c0d-0000-4000-8000-000000000001.jsonl"
mkdir -p "$CLAUDE_WORK/projects/-srv-ccwork" "$ALICE/.claude-code-router" "$ROOT/srv/ccwork/.claude" "$ROOT/srv/cchist/.claude"
printf '{"type":"user","cwd":"/srv/ccwork","message":{"role":"user","content":"hi"}}\n' >"$CLAUDE_WORK_SESSION"
printf '{"display":"hi","project":"/srv/cchist","timestamp":1790848800000}\n' >"$CLAUDE_WORK/history.jsonl"
printf '{"projects":{"/srv/ccwork":{"allowedTools":[]}}}\n' >"$CLAUDE_WORK/.claude.json"
printf '{"claudeAiOauth":{"accessToken":"sk-ant-oat-SECRET"}}\n' >"$CLAUDE_WORK/.credentials.json"
# 2026-10-23 (#256)
CLAUDE_WORK_KEY="$CLAUDE_WORK/sessions/4343.0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef.key"
mkdir -p "$CLAUDE_WORK/sessions" "$CLAUDE_WORK/daemon" "$CLAUDE_WORK/backups"
printf 'session key\n' >"$CLAUDE_WORK_KEY"
printf '0123456789abcdef0123456789abcdef' >"$CLAUDE_WORK/daemon/control.key"
printf '{"workers":{}}\n' >"$CLAUDE_WORK/daemon/roster.json"
printf '{"oauthAccount":{}}\n' >"$CLAUDE_WORK/backups/.claude.json.backup.1792800000000"
printf '{"PORT":3456}\n' >"$ALICE/.claude-code-router/config.json"
printf '{}\n' >"$ROOT/srv/ccwork/.claude/settings.json"
printf '{}\n' >"$ROOT/srv/cchist/.claude/settings.json"

# 2026-10-04 bug round: #29
# Catalog entries differing only by case are distinct directories on a
# case-sensitive filesystem and one directory on a case-insensitive one.
mkdir -p "$ALICE/.config/PearAI/User" "$ALICE/.config/pearai/User" "$ALICE/.config/cursor" "$ALICE/.config/Cursor/User"
printf '{}\n' >"$ALICE/.config/PearAI/User/case29-upper.json"
printf '{}\n' >"$ALICE/.config/pearai/User/case29-lower.json"
printf '{}\n' >"$ALICE/.config/cursor/case29-cli.json"
printf '{}\n' >"$ALICE/.config/Cursor/User/case29-editor.json"
CASE_INSENSITIVE=0
[ -e "$ALICE/.config/pearai/User/case29-upper.json" ] && CASE_INSENSITIVE=1

# 2026-10-06 Ollama (#44)
# The service account's home is found through etc/passwd, which smoke.ps1 lacks.
OLLAMA_MANIFEST="$ROOT/home/ollama/.ollama/models/manifests/registry.ollama.ai/library/llama3.2/latest"
mkdir -p "$(dirname "$OLLAMA_MANIFEST")" "$ROOT/usr/share/ollama/.ollama/models/blobs"
printf '{"schemaVersion":2}\n' >"$OLLAMA_MANIFEST"
TZ=UTC0 touch -t 202609140000.00 "$OLLAMA_MANIFEST"
printf 'svc-ollama:x:998:998::/usr/share/ollama:/usr/sbin/nologin\n' >>"$ROOT/etc/passwd"
printf 'hist\n' >"$ROOT/usr/share/ollama/.ollama/history"
printf 'blob\n' >"$ROOT/usr/share/ollama/.ollama/models/blobs/sha256-def"

# 2026-10-06 frameworks sweep (#64)
# CrewAI's Windows line is a case class, tested as CrewAI for alice and crewai
# for bob; a Camel JBang status file in ~/.camel must not be collected.
CASES_FW='
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
'
printf '%s\n' "$CASES_FW" | while IFS='|' read -r _kind _agent _relpath; do
  [ -n "$_relpath" ] || continue
  case "$_kind" in
    excl) mkdir -p "$ALICE/$_relpath"; printf 'x\n' >"$ALICE/$_relpath/f" ;;
    *) mkdir -p "$ALICE/$(dirname "$_relpath")"; printf 'x\n' >"$ALICE/$_relpath" ;;
  esac
done
printf '{}\n' >"$ALICE/.camel/4242-status.json"
printf 'x\n' >"$ALICE/.local/share/myproj/unrelated.txt"
mkdir -p "$BOB/AppData/Local/crewai/myproj" "$BOB/AppData/Local/crewai/credentials"
printf 'x\n' >"$BOB/AppData/Local/crewai/myproj/latest_kickoff_task_outputs.db"
printf 'x\n' >"$BOB/AppData/Local/crewai/credentials/tokens.enc"

# 2026-10-11 Claude Desktop Linux deb and Claude Code 2.1.295 session keys (#73, #76)
CLAUDE_LINUX_CONFIG="$ALICE/.config/Claude"; SPACE_IDS="0a0a0a0a-0000-4000-8000-00000000000a/0b0b0b0b-0000-4000-8000-00000000000b"
SESSION_KEY_HEX=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
SESSION_KEY="$ALICE/.claude/sessions/4242.$SESSION_KEY_HEX.key"
mkdir -p "$CLAUDE_LINUX_CONFIG/spaces-present/0a0a0a0a/0b0b0b0b" "$CLAUDE_LINUX_CONFIG/git-shadow" "$CLAUDE_LINUX_CONFIG/logs" "$CLAUDE_LINUX_CONFIG/claude-code/2.1.295/9b32b47ec4b3" \
  "$CLAUDE_LINUX_CONFIG/claude-code-sessions/$SPACE_IDS" "$ALICE/.claude/sessions"
printf 'link-key ciphertext\n' >"$CLAUDE_LINUX_CONFIG/spaces-present/0a0a0a0a/0b0b0b0b/link-key"
printf '["/tmp/claude-1000"]\n' >"$CLAUDE_LINUX_CONFIG/git-shadow/cli-temp-roots.json"
printf '2026-10-11 10:00:00 [info] [CCD] Installed\n' >"$CLAUDE_LINUX_CONFIG/logs/main.log"
printf 'ELF\n' >"$CLAUDE_LINUX_CONFIG/claude-code/2.1.295/9b32b47ec4b3/claude"
printf '{"sha256":"%s","size":4}\n' "$SESSION_KEY_HEX" >"$CLAUDE_LINUX_CONFIG/claude-code/2.1.295/9b32b47ec4b3/.payload"
printf '%s' "$SESSION_KEY_HEX" >"$CLAUDE_LINUX_CONFIG/claude-code/2.1.295/9b32b47ec4b3/.verified"
printf '{"sessionId":"local_76767676-7676-4676-8676-767676767676","cliSessionId":"76767676-0000-4000-8000-000000000076","cwd":"/srv/proj","originCwd":"/srv/proj"}\n' >"$CLAUDE_LINUX_CONFIG/claude-code-sessions/$SPACE_IDS/local_76767676-7676-4676-8676-767676767676.json"
printf '{"pid":4242,"sessionId":"76767676-0000-4000-8000-000000000076","entrypoint":"claude-desktop","kind":"interactive"}\n' >"$ALICE/.claude/sessions/4242.json"
printf 'session key\n' >"$SESSION_KEY"

# 2026-10-11 Claude Code CLI cache directory (#81)
CLI_CACHE_LOG="$ALICE/Library/Caches/claude-cli-nodejs/-home-alice-proj/mcp-logs-terminal/2026-10-12T02-23-51-709Z.jsonl"
mkdir -p "$(dirname "$CLI_CACHE_LOG")"
printf '%s\n' '{"cwd":"/home/alice/proj","debug":"MCP server \"terminal\": connected","sessionId":"81818181-0000-4000-8000-000000000081","timestamp":"2026-10-12T02:23:51.709Z"}' >"$CLI_CACHE_LOG"
# The Windows layout puts the slug directories under Cache/, which the
# catalog names as Cache/* so the PowerShell exclusion */Claude*/Cache,
# matched without case, does not prune it.
CLI_CACHE_LOG_WINDOWS="$ALICE/AppData/Local/claude-cli-nodejs/Cache/C--Users-alice-proj/mcp-logs-ccd-host/2026-10-12T02-23-51-709Z.jsonl"
mkdir -p "$(dirname "$CLI_CACHE_LOG_WINDOWS")"
printf '%s\n' '{"cwd":"C:\\Users\\alice\\proj","debug":"MCP server \"ccd-host\": connected","sessionId":"81818181-0000-4000-8000-000000000081","timestamp":"2026-10-12T02:23:51.709Z"}' >"$CLI_CACHE_LOG_WINDOWS"

# 2026-10-13 Antigravity desktop app (#83, #84)
# The Electron Cache is excluded by explicit bases: */Antigravity/Cache, matched
# without case in PowerShell, would also prune .gemini/antigravity/cache.
ANTIGRAVITY_DATA="$ALICE/.gemini/antigravity"; ANTIGRAVITY_ELECTRON="$ALICE/Library/Application Support/Antigravity"; ANTIGRAVITY_WINDOWS="$ALICE/AppData/Roaming/Antigravity"
mkdir -p "$ANTIGRAVITY_DATA/conversations" "$ANTIGRAVITY_DATA/crashes" "$ANTIGRAVITY_DATA/plugin_data/gdocs/bin" "$ANTIGRAVITY_DATA/bin" "$ANTIGRAVITY_DATA/cache/marketplaces/official" \
  "$ANTIGRAVITY_ELECTRON/Cache" "$ANTIGRAVITY_ELECTRON/Code Cache/js" "$ALICE/Library/Logs/Antigravity" "$ANTIGRAVITY_WINDOWS/Cache"
printf 'sqlite\n' >"$ANTIGRAVITY_DATA/conversations/a1.db"
printf 'crash\n' >"$ANTIGRAVITY_DATA/crashes/crash_1.log"
printf '{"token":{"access_token":"x","refresh_token":"y"}}\n' >"$ANTIGRAVITY_DATA/plugin_data/gdocs/token.json"
printf 'bin\n' >"$ANTIGRAVITY_DATA/plugin_data/gdocs/bin/gdocs-1"
printf 'bin\n' >"$ANTIGRAVITY_DATA/bin/webm_encoder"
printf '{"name":"official","plugins":[]}\n' >"$ANTIGRAVITY_DATA/cache/marketplaces/official/marketplace.json"
printf '{"x":"a1"}\n' >"$ANTIGRAVITY_DATA/cache/last_conversations.json"
printf '{"lastCreatedProjectId":"p1"}\n' >"$ANTIGRAVITY_ELECTRON/app_storage.json"
printf 'cache\n' >"$ANTIGRAVITY_ELECTRON/Cache/data_0"
printf 'js\n' >"$ANTIGRAVITY_ELECTRON/Code Cache/js/x"
printf '[2026-10-13 18:28:00.000] [info]  Starting app (v2.23.0)\n' >"$ALICE/Library/Logs/Antigravity/main.log"
printf '{"lastCreatedProjectId":"p1"}\n' >"$ANTIGRAVITY_WINDOWS/app_storage.json"
printf 'cache\n' >"$ANTIGRAVITY_WINDOWS/Cache/data_0"

# 2026-10-13 Antigravity Linux round (#83, #90, #91)
# The Dictionaries fixture is in bob's macOS directory: alice's
# .config/Antigravity is .config/antigravity on a case-insensitive filesystem.
mkdir -p "$ALICE/.config/antigravity" "$ANTIGRAVITY_ELECTRON/Dictionaries" "$ANTIGRAVITY_DATA/lib/node_modules/sidecar_sdk" \
  "$ALICE/.gemini/config/node_modules/sidecar_sdk" "$ALICE/.gemini/config/projects" "$ROOT/srv/agyapp" "$ROOT/srv/agywin"
printf '{"gdocs":"x"}\n' >"$ALICE/.config/antigravity/plugin-secrets.json"
printf 'bdic\n' >"$ANTIGRAVITY_ELECTRON/Dictionaries/en-US-10-1.bdic"
printf 'export {}\n' >"$ANTIGRAVITY_DATA/lib/node_modules/sidecar_sdk/index.mjs"
printf 'export {}\n' >"$ALICE/.gemini/config/node_modules/sidecar_sdk/index.mjs"
printf '{\n  "id": "91919191-0000-4000-8000-000000000091",\n  "projectResources": {\n    "resources": [\n      {\n        "folderUri":  "file:///srv/agyapp"\n      }\n    ]\n  }\n}\n' >"$ALICE/.gemini/config/projects/91919191-0000-4000-8000-000000000091.json"
printf '{\n  "id": "91919191-0000-4000-8000-000000000092",\n  "projectResources": {\n    "resources": [\n      {\n        "folderUri":  "file:///c%%3A/srv/agywin"\n      }\n    ]\n  }\n}\n' >"$ALICE/.gemini/config/projects/91919191-0000-4000-8000-000000000092.json"
printf 'ignore\n' >"$ROOT/srv/agyapp/.antigravityignore"
printf 'ignore\n' >"$ROOT/srv/agywin/.antigravityignore"

# 2026-10-14 home patterns and the working directory (#105)
# The cwd105 run's working directory: home patterns expand under the image root.
HOMES_CWD="$WORK/cwd105"
mkdir -p "$HOMES_CWD/home/analyst" "$HOMES_CWD/Users/analyst"

# 2026-10-14 catalog match that is an excluded file (#107)
printf '1070\n' >"$ALICE/.local/bin/.muse-update.107.exe"

# 2026-10-14 output directory under a file (#108)
PLAIN_FILE="$WORK/file108"
printf 'x\n' >"$PLAIN_FILE"

# 2026-10-14 exclusion globs and the working directory (#109)
# The cwd109 run's working directory: the glob must reach find unexpanded.
mkdir -p "$ALICE/.claude/plugins/y/node_modules" "$WORK/cwd109/.claude/plugins/x/node_modules"
printf 'module.exports = 1\n' >"$ALICE/.claude/plugins/y/node_modules/f.js"

# 2026-10-14 CDPATH in the environment (#110)
# sh only, as PowerShell has no CDPATH: a decoy root for the cdpath110 run.
mkdir -p "$WORK/cdp110/root/home"

# 2026-10-14 Antigravity on Windows (#83)
ALICE_LOCAL_APPDATA="$ALICE/AppData/Local"
mkdir -p "$ALICE_LOCAL_APPDATA/antigravity/staging" "$ALICE_LOCAL_APPDATA/antigravity-updater" "$ALICE_LOCAL_APPDATA/Programs/antigravity"
printf 'staged\n' >"$ALICE_LOCAL_APPDATA/antigravity/staging/s1"
printf 'exe\n' >"$ALICE_LOCAL_APPDATA/antigravity-updater/installer.exe"
printf 'exe\n' >"$ALICE_LOCAL_APPDATA/Programs/antigravity/Antigravity.exe"

# 2026-10-15 ChatGPT desktop as the OpenAI.Codex package (#113)
CHATGPT_PACKAGE="$ALICE/AppData/Local/Packages/OpenAI.Codex_2p2nqsd0c76g0"
CHATGPT_PROFILE="$CHATGPT_PACKAGE/LocalCache/Roaming/Codex/web/Codex"
CHATGPT_PARTITION="$CHATGPT_PROFILE/Default/Partitions/codex-browser-app"
mkdir -p "$CHATGPT_PROFILE/Default/Local Storage/leveldb" "$CHATGPT_PROFILE/Default/Network" "$CHATGPT_PROFILE/Default/Cache" \
  "$CHATGPT_PARTITION/Network" "$CHATGPT_PARTITION/Cache" "$CHATGPT_PROFILE/component_crx_cache" \
  "$CHATGPT_PACKAGE/LocalCache/Local/Codex/Logs/2026/10/15" "$CHATGPT_PACKAGE/Settings" \
  "$BOB/.codex/plugins/.plugin-appserver"
printf 'ldb\n' >"$CHATGPT_PROFILE/Default/Local Storage/leveldb/000003.log"
printf '{"os_crypt":{}}\n' >"$CHATGPT_PROFILE/Local State"
printf 'cookies\n' >"$CHATGPT_PROFILE/Default/Network/Cookies"
printf 'cookies\n' >"$CHATGPT_PARTITION/Network/Cookies"
printf 'logins\n' >"$CHATGPT_PROFILE/Default/Login Data"
printf 'cache\n' >"$CHATGPT_PROFILE/Default/Cache/data_0"
printf 'cache\n' >"$CHATGPT_PARTITION/Cache/data_0"
printf '{}\n' >"$CHATGPT_PROFILE/component_crx_cache/metadata.json"
printf 'log\n' >"$CHATGPT_PACKAGE/LocalCache/Local/Codex/Logs/2026/10/15/codex-desktop-x.log"
printf 'hive\n' >"$CHATGPT_PACKAGE/Settings/settings.dat"
printf 'exe\n' >"$BOB/.codex/plugins/.plugin-appserver/codex.exe"
printf '{}\n' >"$BOB/.codex/.codex-global-state.json"

# 2026-10-15 ChatGPT desktop on macOS (#121)
CHATGPT_MAC="$BOB/Library/Application Support/Codex"
CHATGPT_MAC_PARTITION="$CHATGPT_MAC/Default/Partitions/codex-browser-app"
CUA_GROUP_CONTAINER="$BOB/Library/Group Containers/2DC432GLL2.com.openai.sky.CUAService"
COMPUTER_USE="$BOB/.codex/computer-use"
mkdir -p "$CHATGPT_MAC/Default/Local Storage/leveldb" "$CHATGPT_MAC/Default/Cache" "$CHATGPT_MAC_PARTITION/Cache" \
  "$CHATGPT_MAC/executor-plugins/codex-app-tools" "$CHATGPT_MAC/component_crx_cache" \
  "$BOB/Library/Logs/com.openai.codex/2026/10/15" "$BOB/Library/Preferences" \
  "$BOB/Library/HTTPStorages" "$CUA_GROUP_CONTAINER/IPC" "$COMPUTER_USE/Codex Computer Use.app/Contents"
printf 'ldb\n' >"$CHATGPT_MAC/Default/Local Storage/leveldb/000003.log"
printf 'cookies\n' >"$CHATGPT_MAC/Default/Cookies"
printf 'cookies\n' >"$CHATGPT_MAC_PARTITION/Cookies"
printf 'logins\n' >"$CHATGPT_MAC/Default/Login Data"
printf 'cache\n' >"$CHATGPT_MAC/Default/Cache/data_0"
printf 'cache\n' >"$CHATGPT_MAC_PARTITION/Cache/data_0"
printf 'js\n' >"$CHATGPT_MAC/executor-plugins/codex-app-tools/server.mjs"
printf '{}\n' >"$CHATGPT_MAC/executor-plugins/codex-app-tools/.mcp.json"
printf '{}\n' >"$CHATGPT_MAC/component_crx_cache/metadata.json"
printf 'log\n' >"$BOB/Library/Logs/com.openai.codex/2026/10/15/codex-desktop-x.log"
printf 'plist\n' >"$BOB/Library/Preferences/com.openai.codex.plist"
printf 'cookies\n' >"$BOB/Library/HTTPStorages/com.openai.codex.binarycookies"
: >"$CUA_GROUP_CONTAINER/IPC/computeruse.sock.lock"
printf 'plist\n' >"$COMPUTER_USE/Codex Computer Use.app/Contents/Info.plist"
printf '{}\n' >"$COMPUTER_USE/config.json"

# 2026-10-15 special files (#120)
# The FIFO is sh only. python3 binds the socket from inside its directory, as a
# full path under a long TMPDIR can pass the socket path length limit.
SPECIAL_DIR="$ROOT/home/alice/.codex"
mkdir -p "$SPECIAL_DIR/ipc" "$SPECIAL_DIR/app-server-daemon"
mkfifo "$SPECIAL_DIR/ipc/ipc.fifo"
HAVE_SOCKET=0
if command -v python3 >/dev/null 2>&1 && python3 -c 'import os,socket,sys
os.chdir(os.path.dirname(sys.argv[1]))
socket.socket(socket.AF_UNIX).bind(os.path.basename(sys.argv[1]))' \
  "$SPECIAL_DIR/app-server-daemon/daemon-updater.sock" 2>/dev/null &&
  [ -S "$SPECIAL_DIR/app-server-daemon/daemon-updater.sock" ]; then
  HAVE_SOCKET=1
else
  printf 'note: no Unix domain socket could be created; socket checks skipped\n'
fi

# 2026-10-16 GitHub Copilot Desktop (#147)
COPILOT="$ROOT/home/alice/.copilot"
COPILOT_DESKTOP_DATA="$ROOT/home/alice/.local/share/com.github.githubapp"
mkdir -p "$COPILOT/run" "$COPILOT/chats/2026-10-16/x-5d404e14" "$COPILOT/repos/proj" \
  "$COPILOT/repos/copilot-worktrees/proj/alice-x" "$COPILOT/logs" "$COPILOT/session-state/s2" \
  "$COPILOT/session-state/s3" "$COPILOT_DESKTOP_DATA/localstorage" "$COPILOT_DESKTOP_DATA/app-skills/af" \
  "$ROOT/home/alice/.github-copilot-cli/logs"
printf 'sqlite\n' >"$COPILOT/data.db"
printf 'wal\n' >"$COPILOT/data.db-wal"
printf '%s\n' 'AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA' >"$COPILOT/run/ws.token"
printf '12345\n' >"$COPILOT/run/ws.port"
: >"$COPILOT/chats/2026-10-16/x-5d404e14/.keep"
printf '# proj\n' >"$COPILOT/repos/proj/README.md"
COPILOT_CLONE_GIT="$COPILOT/repos/proj/.git"
COPILOT_CHECKPOINT_REF="$COPILOT_CLONE_GIT/refs/copilot/checkpoints/5e55a0a1-0000-4000-8000-000000000001/00000000000000000001"
mkdir -p "$COPILOT_CHECKPOINT_REF" "$COPILOT_CLONE_GIT/objects/ab"
printf 'ref: refs/heads/main\n' >"$COPILOT_CLONE_GIT/HEAD"
printf '0123456789abcdef0123456789abcdef01234567\n' \
  >"$COPILOT_CHECKPOINT_REF/e0000000-0000-4000-8000-000000000020"
printf 'object\n' >"$COPILOT_CLONE_GIT/objects/ab/cdef0123"
printf 'index\n' >"$COPILOT_CLONE_GIT/index"
printf '# wt\n' >"$COPILOT/repos/copilot-worktrees/proj/alice-x/AGENTS.md"
printf 'log\n' >"$COPILOT/logs/github-app.1.log"
printf 'cwd: /home/alice/.copilot/chats/2026-10-16/x-5d404e14\nclient_name: github/autopilot\n' \
  >"$COPILOT/session-state/s2/workspace.yaml"
printf 'cwd: /home/alice/.copilot/repos/copilot-worktrees/proj/alice-x\nclient_name: github/autopilot\n' \
  >"$COPILOT/session-state/s3/workspace.yaml"
printf 'sqlite\n' >"$COPILOT_DESKTOP_DATA/localstorage/tauri_localhost_0.localstorage"
printf '# af\n' >"$COPILOT_DESKTOP_DATA/app-skills/af/SKILL.md"
printf 'log\n' >"$ROOT/home/alice/.github-copilot-cli/logs/foundry.core20261016.log"

# 2026-10-17 Copilot Desktop Windows round (#147)
COPILOT_WEBVIEW="$ROOT/home/alice/AppData/Local/com.github.githubapp/EBWebView"
COPILOT_CHAT_PROJECT="$COPILOT/chats/2026-10-17/x-90825c78/proj"
COPILOT_MARKETPLACES="$ROOT/home/alice/AppData/Local/copilot/marketplaces"
mkdir -p "$COPILOT_WEBVIEW/Default/Local Storage/leveldb" "$COPILOT_WEBVIEW/Default/Cache/Cache_Data" "$COPILOT_WEBVIEW/Default/Network" \
  "$COPILOT_WEBVIEW/component_crx_cache" "$COPILOT_CHAT_PROJECT/node_modules/m" "$COPILOT_MARKETPLACES/m/.git" \
  "$COPILOT/session-state/s4" "$ROOT/srv/coppool"
printf 'leveldb\n' >"$COPILOT_WEBVIEW/Default/Local Storage/leveldb/000003.log"
printf 'cookies\n' >"$COPILOT_WEBVIEW/Default/Network/Cookies"
printf '{"os_crypt":{}}\n' >"$COPILOT_WEBVIEW/Local State"
printf 'cache\n' >"$COPILOT_WEBVIEW/Default/Cache/Cache_Data/x"
printf 'crx\n' >"$COPILOT_WEBVIEW/component_crx_cache/x"
printf '{"name":"proj"}\n' >"$COPILOT_CHAT_PROJECT/package.json"
printf 'module\n' >"$COPILOT_CHAT_PROJECT/node_modules/m/index.js"
printf 'ref: refs/heads/main\n' >"$COPILOT_MARKETPLACES/m/.git/HEAD"
for _session in s1 s2 s3; do
  printf '{"type":"session.start"}\n' >"$COPILOT/session-state/$_session/events.jsonl"
done
printf 'cwd: /srv/coppool\nclient_name: github/autopilot\n' >"$COPILOT/session-state/s4/workspace.yaml"
printf '# pool\n' >"$ROOT/srv/coppool/AGENTS.md"

# 2026-10-17 session sync round (#153)
NANOBOT_PAIRING="$ALICE/.nanobot/webui/remote-pairing/11111111-1111-4111-8111-111111111111"
CLINE_REMOTE_CACHE="$ALICE/.vscode-server/data/User/globalStorage/saoudrizwan.claude-dev/cache"
AGENT_ZERO_PLUGIN_CONFIG=plugins/_telegram_integration/config.json
mkdir -p "$NANOBOT_PAIRING" "$CLINE_REMOTE_CACHE" "$ALICE/agent-zero/main/usr/${AGENT_ZERO_PLUGIN_CONFIG%/*}" "$ALICE/Desktop/agent-zero/usr/${AGENT_ZERO_PLUGIN_CONFIG%/*}" \
  "$VOLUMES/a0_usr/_data/${AGENT_ZERO_PLUGIN_CONFIG%/*}" "$VOLUMES/a0_usr/_data/usr/${AGENT_ZERO_PLUGIN_CONFIG%/*}" "$ALICE/Documents/Cline" \
  "$ALICE/.little-coder" "$ALICE/.qwen/channels/weixin" "$ALICE/.qwen/channels/daemon/0123abcd" \
  "$ALICE/.local/share/kilo/log"
printf 'key\n' >"$NANOBOT_PAIRING/identity"
printf 'key\n' >"$NANOBOT_PAIRING/exchange"
printf '{}\n' >"$NANOBOT_PAIRING/request"
printf '{"token":"x"}\n' >"$ALICE/agent-zero/main/usr/$AGENT_ZERO_PLUGIN_CONFIG"
printf '{"token":"x"}\n' >"$ALICE/Desktop/agent-zero/usr/$AGENT_ZERO_PLUGIN_CONFIG"
printf '{"token":"x"}\n' >"$VOLUMES/a0_usr/_data/$AGENT_ZERO_PLUGIN_CONFIG"
printf '{"token":"x"}\n' >"$VOLUMES/a0_usr/_data/usr/$AGENT_ZERO_PLUGIN_CONFIG"
printf '{"promptUploading":{}}\n' >"$CLINE_REMOTE_CACHE/remote_config_org1.json"
printf '{"items":{}}\n' >"$CLINE_REMOTE_CACHE/sync-queue.json"
printf '{"webhook_token":"x"}\n' >"$ALICE/Documents/Cline/webhook_config.json"
printf 'heap\n' >"$ALICE/.gemini/tmp/gem/bug-memory-1792000000000.heapsnapshot"
printf '{"gist_token":"x"}\n' >"$ALICE/.little-coder/config.json"
printf '{"appId":"x","appSecret":"x"}\n' >"$ALICE/.qwen/channels/qqbot-credentials.json"
printf '{"token":"x"}\n' >"$ALICE/.qwen/channels/weixin/account.json"
printf '{}\n' >"$ALICE/.qwen/channels/sessions.json"
printf '{}\n' >"$ALICE/.qwen/channels/daemon/0123abcd/routes.json"
printf 'INFO creating session\n' >"$ALICE/.local/share/kilo/log/2026-10-17T120000.log"

# 2026-10-18 Codex shell snapshots (#160)
CODEX_SHELL_SNAPSHOT="$ALICE/.codex/shell_snapshots/22222222-2222-4222-8222-222222222222.1792000000000000000.sh"
mkdir -p "${CODEX_SHELL_SNAPSHOT%/*}"
printf '# Snapshot file\nexport EXAMPLE_API_KEY=x\n' >"$CODEX_SHELL_SNAPSHOT"

# 2026-10-19 Codex projectless output directories (#149)
CODEX_OUTPUT_DIR="$BOB/Documents/Codex/2026-10-18/w"
mkdir -p "$CODEX_OUTPUT_DIR/outputs" "$CODEX_OUTPUT_DIR/work"
printf '{"cwd":"/Users/bob/Documents/Codex/2026-10-18/w"}\n' \
  >"$BOB/.codex/sessions/2026/10/03/rollout-2.jsonl"
printf '# report\n' >"$CODEX_OUTPUT_DIR/outputs/report.md"
printf 'notes\n' >"$CODEX_OUTPUT_DIR/work/notes.txt"

# 2026-10-20 Cursor IDE round on macOS (#6)
CURSOR_MAC="$BOB/Library/Application Support/Cursor"
CURSOR_MAC_WORKER="$CURSOR_MAC/User/globalStorage/anysphere.cursor-agent-worker"
mkdir -p "$CURSOR_MAC/AgentStores/cursor_agent_stores/u1/.sync" \
  "$CURSOR_MAC/AgentStores/cursor_agent_stores/.control/u1/pins" \
  "$CURSOR_MAC/update-supervisor/33333333-3333-4333-8333-333333333333" \
  "$CURSOR_MAC_WORKER/agent-cli/.local/share" "$BOB/.cursor/projects/p/canvases/node_modules" \
  "$BOB/.cursor/projects/p/agent-transcripts/u"
printf 'sqlite\n' >"$CURSOR_MAC/AgentStores/cursor_agent_stores/u1/.sync/index.sqlite"
printf '{"purpose":"foreground"}\n' >"$CURSOR_MAC/AgentStores/cursor_agent_stores/.control/u1/pins/1-x.json"
printf '{"type":"supervisor_started"}\n' \
  >"$CURSOR_MAC/update-supervisor/33333333-3333-4333-8333-333333333333/events.jsonl"
printf 'binary\n' \
  >"$CURSOR_MAC/update-supervisor/33333333-3333-4333-8333-333333333333/cursor-update-supervisor"
printf 'cli\n' >"$CURSOR_MAC_WORKER/agent-cli/.local/share/x"
printf 'worker log\n' >"$CURSOR_MAC_WORKER/cursor-agent-worker-1.log"
printf 'declare const x: number;\n' >"$BOB/.cursor/projects/p/canvases/node_modules/x.d.ts"
printf '{"type":"turn_ended","status":"success"}\n' \
  >"$BOB/.cursor/projects/p/agent-transcripts/u/u.jsonl"
printf '{}\n' >"$BOB/.cursor/statsig-cache.json"

# 2026-10-20 Cursor IDE round on Ubuntu (#236)
CURSOR_LINUX_STORES="$ALICE/.local/state/cursor/agent-stores/cursor_agent_stores"
mkdir -p "$CURSOR_LINUX_STORES/u1/.sync" "$CURSOR_LINUX_STORES/.control/u1/pins"
printf 'sqlite\n' >"$CURSOR_LINUX_STORES/u1/.sync/index.sqlite"
printf '{"purpose":"foreground"}\n' >"$CURSOR_LINUX_STORES/.control/u1/pins/1-x.json"

# 2026-10-21 Cursor IDE round on Windows (#236)
CURSOR_WINDOWS_STORES="$BOB/AppData/Local/Cursor/AgentStores/cursor_agent_stores"
CURSOR_WINDOWS_WORKER="$BOB/AppData/Roaming/Cursor/User/globalStorage/anysphere.cursor-agent-worker"
mkdir -p "$CURSOR_WINDOWS_STORES/u1/.sync" "$CURSOR_WINDOWS_STORES/.control/u1/pins" "$CURSOR_WINDOWS_WORKER/agent-cli"
printf 'sqlite\n' >"$CURSOR_WINDOWS_STORES/u1/.sync/index.sqlite"
printf '{"purpose":"foreground"}\n' >"$CURSOR_WINDOWS_STORES/.control/u1/pins/1-x.json"
printf 'cli\n' >"$CURSOR_WINDOWS_WORKER/agent-cli/x"
printf 'worker log\n' >"$CURSOR_WINDOWS_WORKER/cursor-agent-worker-1.log"
# An installed plugin's marketplace copy under .cursor/plugins/cache is
# excluded; plugins/local beside it is collected.
CURSOR_PLUGINS="$BOB/.cursor/plugins"
mkdir -p "$CURSOR_PLUGINS/cache/cursor-public/x/0123abcd" "$CURSOR_PLUGINS/local"
printf '{"mcpServers":{}}\n' >"$CURSOR_PLUGINS/cache/cursor-public/x/0123abcd/mcp.json"
printf '{}\n' >"$CURSOR_PLUGINS/local/x.json"
# The updater's copy of inno_updater.exe under update-supervisor is excluded.
CURSOR_UPDATE_SUPERVISOR="$BOB/AppData/Roaming/Cursor/update-supervisor/win32/0000"
mkdir -p "$CURSOR_UPDATE_SUPERVISOR"
printf 'binary\n' >"$CURSOR_UPDATE_SUPERVISOR/inno_updater.exe"
printf '{"currentVersion":"x"}\n' >"$CURSOR_UPDATE_SUPERVISOR/app-ready.json"
printf '{"stage":"x"}\n' >"$CURSOR_UPDATE_SUPERVISOR/state.json"
# 2026-10-21 OpenHands Agent Canvas credentials and workspaces (#218)
CANVAS_CREDENTIALS_LINUX="$ALICE/.config/OpenHands Agent Canvas/mars-credentials.json"
CANVAS_CREDENTIALS_MAC="$BOB/Library/Application Support/OpenHands Agent Canvas/mars-credentials.json"
CANVAS_CREDENTIALS_WINDOWS="$BOB/AppData/Roaming/OpenHands Agent Canvas/mars-credentials.json"
for _file in "$CANVAS_CREDENTIALS_LINUX" "$CANVAS_CREDENTIALS_MAC" "$CANVAS_CREDENTIALS_WINDOWS"; do
  mkdir -p "$(dirname "$_file")"
  printf '{"version":1,"connections":[]}\n' >"$_file"
done
mkdir -p "$ALICE/.openhands/workspaces/run1"
printf 'x\n' >"$ALICE/.openhands/workspaces/run1/f"

# 2026-10-21 Agent Zero source review (#217)
WHATSAPP_CREDS=tmp/whatsapp/session/creds.json
AGENT_ZERO_ACP_CHAT="$ALICE/agent-zero/main/usr/chats/acp1"
AGENT_ZERO_CAPTURE="$BOB/AppData/Local/Temp/_a0_connector/computer_use/ctx1"
mkdir -p "$ALICE/agent-zero/main/${WHATSAPP_CREDS%/*}" "$ALICE/Desktop/agent-zero/${WHATSAPP_CREDS%/*}" \
  "$VOLUMES/a0_usr/_data/${WHATSAPP_CREDS%/*}" "$AGENT_ZERO_ACP_CHAT" "$ROOT/srv/a0acp" "$ROOT/srv/a0extra" "$AGENT_ZERO_CAPTURE"
printf '{}\n' >"$ALICE/agent-zero/main/$WHATSAPP_CREDS"
printf '{}\n' >"$ALICE/Desktop/agent-zero/$WHATSAPP_CREDS"
printf '{}\n' >"$VOLUMES/a0_usr/_data/$WHATSAPP_CREDS"
printf '{"id": "acp1", "data": {"acp_cwd": "/srv/a0acp", %s}}\n' \
  '"acp_additional_directories": ["/srv/a0extra"]' >"$AGENT_ZERO_ACP_CHAT/chat.json"
printf '# acp\n' >"$ROOT/srv/a0acp/AGENTS.md"
printf '# extra\n' >"$ROOT/srv/a0extra/AGENTS.md"
printf 'png\n' >"$AGENT_ZERO_CAPTURE/0000.png"

# 2026-10-21 framework catalog gaps (#223, #224, #225)
mkdir -p "$ROOT/srv/proj/.crewai"
printf '{"execution_id":"x","tier":"ephemeral"}\n' >"$ROOT/srv/proj/.crewai/last_run.json"
for _config_dir in .config AppData/Roaming AppData/Local; do
  mkdir -p "$ALICE/$_config_dir/difyctl"
  printf 'device_code: x\n' >"$ALICE/$_config_dir/difyctl/login-pending.yml"
done
CLAI_STATE="$ALICE/.local/state/pydantic-clai2"
mkdir -p "$CLAI_STATE/workspaces/registry/ws1" "$CLAI_STATE/workspaces/clones/repo-0123456789/.git"
printf '{}\n' >"$CLAI_STATE/orchestrator.json"
printf '{"id":"ws1"}\n' >"$CLAI_STATE/workspaces/registry/ws1/workspace.json"
printf 'token\n' >"$CLAI_STATE/workspaces/registry/ws1/token"
printf 'log\n' >"$CLAI_STATE/workspaces/registry/ws1/session.log"
printf 'ref: refs/heads/main\n' >"$CLAI_STATE/workspaces/clones/repo-0123456789/.git/HEAD"

# 2026-10-21 Codex worktree cwds and archived threads (#222)
mkdir -p "$BOB/.codex/archived_sessions" "$ROOT/srv/codexarch" "$BOB/.codex/worktrees/a1b2/repo" \
  "$ALICE/.openinterpreter/worktrees/c3d4/repo"
printf '{"type":"session_meta","payload":{"cwd":"/srv/codexarch"}}\n' \
  >"$BOB/.codex/archived_sessions/rollout-2026-10-01T00-00-00-a1.jsonl"
printf '# agents\n' >"$ROOT/srv/codexarch/AGENTS.md"
printf '{"type":"session_meta","payload":{"cwd":"/Users/bob/.codex/worktrees/a1b2/repo"}}\n' \
  >"$BOB/.codex/sessions/2026/10/03/rollout-3.jsonl"
printf '# agents\n' >"$BOB/.codex/worktrees/a1b2/repo/AGENTS.md"
printf '{"type":"session_meta","payload":{"cwd":"/home/alice/.openinterpreter/worktrees/c3d4/repo"}}\n' \
  >"$ALICE/.openinterpreter/sessions/2026/10/04/rollout-2026-10-04T00-00-00-u3.jsonl"
printf '# agents\n' >"$ALICE/.openinterpreter/worktrees/c3d4/repo/AGENTS.md"

# 2026-10-21 Cody JetBrains plugin files (#214)
JETBRAINS_OPTIONS="$ALICE/.config/JetBrains/IntelliJIdea2025.2/options"
mkdir -p "$JETBRAINS_OPTIONS" "$ALICE/.sourcegraph" "$ROOT/srv/proj/.idea/.sourcegraph"
printf '<application />\n' >"$JETBRAINS_OPTIONS/cody.xml"
printf '<application />\n' >"$JETBRAINS_OPTIONS/cody_accounts.xml"
printf '<application />\n' >"$JETBRAINS_OPTIONS/cody_accounts_settings.xml"
printf '<application />\n' >"$JETBRAINS_OPTIONS/cody_application_settings.xml"
printf '{}\n' >"$ALICE/.sourcegraph/cody_settings.json"
printf '<project version="4" />\n' >"$ROOT/srv/proj/.idea/cody_history.xml"
printf '{}\n' >"$ROOT/srv/proj/.idea/.sourcegraph/cody_settings.json"

# 2026-10-21 agent worktrees inside a project (#216)
GEMINI_PROJECT="$ROOT/srv/gem"
mkdir -p "$GEMINI_PROJECT/.gemini/worktrees/w1" "$GEMINI_PROJECT/.qwen/worktrees/w1"
printf 'wt\n' >"$GEMINI_PROJECT/.gemini/worktrees/w1/f"
printf 'wt\n' >"$GEMINI_PROJECT/.qwen/worktrees/w1/f"
printf '{}\n' >"$GEMINI_PROJECT/.gemini/settings.json"

# 2026-10-21 Hermes quick snapshots and desktop connections (#123, #208)
HERMES_SNAPSHOT="$ALICE/.hermes/state-snapshots/20261001-120004-pre-update"
HERMES_SNAPSHOT_WINDOWS="$ALICE/AppData/Local/hermes/state-snapshots/20261001-120004"
mkdir -p "$HERMES_SNAPSHOT/pairing" "$HERMES_SNAPSHOT_WINDOWS" "$ALICE/.config/Hermes"
printf 'sqlite\n' >"$HERMES_SNAPSHOT/state.db"
printf '{"id":"20261001-120004-pre-update"}\n' >"$HERMES_SNAPSHOT/manifest.json"
printf 'KEY=x\n' >"$HERMES_SNAPSHOT/.env"
printf '{"providers":{}}\n' >"$HERMES_SNAPSHOT/auth.json"
printf '{"42":{"user_name":"u"}}\n' >"$HERMES_SNAPSHOT/pairing/telegram-approved.json"
printf 'sqlite\n' >"$HERMES_SNAPSHOT_WINDOWS/state.db"
printf '{"connections":[]}\n' >"$ALICE/.config/Hermes/connections.json"

# 2026-10-21 nanobot workspace cron and triggers (#220)
NANOBOT_WORKSPACE="$ROOT/srv/nanows"
mkdir -p "$NANOBOT_WORKSPACE/cron/runs" "$NANOBOT_WORKSPACE/triggers/runs" "$NANOBOT_WORKSPACE/triggers/inbox"
printf '{"jobs":[]}\n' >"$NANOBOT_WORKSPACE/cron/jobs.json"
printf '{"job_id":"j1","status":"ok"}\n' >"$NANOBOT_WORKSPACE/cron/runs/j1_1_ab.json"
printf '{"triggers":[]}\n' >"$NANOBOT_WORKSPACE/triggers/triggers.json"
printf '{"trigger_id":"t1","status":"ok"}\n' >"$NANOBOT_WORKSPACE/triggers/runs/d1.json"
printf '{"id":"d2"}\n' >"$NANOBOT_WORKSPACE/triggers/inbox/d2.json"

# 2026-10-21 OpenClaw source review (#219)
OPENCLAW="$ALICE/.openclaw"
OPENCLAW_CLONE_ID=0123456789abcdef
mkdir -p "$OPENCLAW/projects/$OPENCLAW_CLONE_ID/repo" "$OPENCLAW/projects/notes" "$ALICE/.openclaw-auth-profile-secrets"
printf '{"socket":{"token":"x"}}\n' >"$OPENCLAW/exec-approvals.json"
printf 'OPENAI_API_KEY=x\n' >"$ALICE/.openclaw-auth-profile-secrets/gateway.env"
printf 'clone\n' >"$OPENCLAW/projects/$OPENCLAW_CLONE_ID/repo/README.md"
printf 'notes\n' >"$OPENCLAW/projects/notes/todo.md"
# 2026-10-30 openclaw rebuild (#326)
mkdir -p "$ROOT/srv/clawcwd" "$ROOT/srv/clawold" "$ROOT/srv/moltws/.openclaw" "$ROOT/srv/moltold"
printf '{agents:{defaults:{workspace:"/srv/clawprofile"},entries:{ops:{cwd:"/srv/clawcwd"}}}}\n' >"$ALICE/.openclaw-work/openclaw.json"
printf '{"agents":{"defaults":{"workspace":"/srv/clawold"}}}\n' >"$ALICE/.clawdbot/openclaw.json"
printf '{"agents":{"defaults":{"workspace":"/srv/moltws"}}}\n' >"$ALICE/.moltbot/moltbot.json"
printf '{"agents":{"defaults":{"workspace":"/srv/moltold"}}}\n' >"$ALICE/.clawdbot/moltbot.json"
printf '# tools\n' >"$ROOT/srv/clawcwd/TOOLS.md"
printf '# bootstrap\n' >"$ROOT/srv/clawold/BOOTSTRAP.md"
printf '#!/bin/sh\n' >"$ROOT/srv/moltws/.openclaw/worktree-setup.sh"
printf '# soul\n' >"$ROOT/srv/moltold/SOUL.md"

# 2026-10-21 OpenCode and Kilo Code source review (#213, #221)
OPENCODE_DESKTOP="$BOB/Library/Application Support/ai.opencode.desktop"
KILO_SANDBOX_POLICY="$ALICE/.local/state/kilo-sandbox-policy/aa"
mkdir -p "$ALICE/.config/opencode" "$ALICE/.local/state/opencode" "$OPENCODE_DESKTOP/compile-cache" \
  "$ALICE/.config/ai.opencode.desktop.beta" "$BOB/AppData/Roaming/ai.opencode.desktop" \
  "$BOB/AppData/Local/Temp/opencode" "$KILO_SANDBOX_POLICY" "$ALICE/.local/state/kilo-sandbox-preference"
printf '{"password":"x"}\n' >"$ALICE/.config/opencode/service.json"
printf '{"password":"x"}\n' >"$ALICE/.local/state/opencode/service-local.json"
printf 'sqlite\n' >"$OPENCODE_DESKTOP/drafts.sqlite"
printf 'v8\n' >"$OPENCODE_DESKTOP/compile-cache/x"
printf 'sqlite\n' >"$ALICE/.config/ai.opencode.desktop.beta/drafts.sqlite"
printf 'sqlite\n' >"$BOB/AppData/Roaming/ai.opencode.desktop/drafts.sqlite"
printf 'tmp\n' >"$BOB/AppData/Local/Temp/opencode/notes.txt"
printf '{"enabled":true,"mode":"deny","version":1}\n' >"$KILO_SANDBOX_POLICY/bb.json"
printf 'false\n' >"$ALICE/.local/state/kilo-sandbox-preference/bb.json"

# 2026-10-21 Qwen Code session registry (#230)
mkdir -p "$ALICE/.qwen/sessions"
printf '{"sessionId":"x","ipcToken":"x"}\n' >"$ALICE/.qwen/sessions/4242.json"
printf '{"sessionId":"x","ipcToken":"x"}\n' >"$ALICE/.qwen/sessions/4243-0123abcd.json"

# 2026-10-21 Roo Code CLI cloud credentials (#159)
mkdir -p "$ALICE/.roo"
printf '{"token":"not-a-real-token"}\n' >"$ALICE/.roo/cli-credentials.json"

# 2026-10-21 Twinny workspace storage (#215)
TWINNY_WORKSPACE="$ALICE/.config/Code/User/workspaceStorage/ws215"
mkdir -p "$TWINNY_WORKSPACE/rjmacarthy.twinny/agent-changes"
printf '{"id":"c1","root":"/srv/twinproj","createdAt":1,"files":[]}\n' \
  >"$TWINNY_WORKSPACE/rjmacarthy.twinny/agent-changes/c1.json"
printf '{"folder":"file:///srv/twinproj"}\n' >"$TWINNY_WORKSPACE/workspace.json"
# The twinny-server forge plugins keep access tokens in repos.json: flagged secret.
mkdir -p "$ALICE/.twinny/server/plugins/github"
printf '{"version":1,"repos":[],"tokens":[]}\n' >"$ALICE/.twinny/server/plugins/github/repos.json"

# 2026-10-21 project git metadata (#253)
PROJECT_GIT="$ROOT/srv/proj/.git"
mkdir -p "$PROJECT_GIT/refs/heads" "$PROJECT_GIT/refs/remotes/origin" "$PROJECT_GIT/logs/refs/remotes/origin" \
  "$PROJECT_GIT/worktrees/wt1" "$PROJECT_GIT/objects/ab" "$PROJECT_GIT/hooks" "$PROJECT_GIT/lfs/objects" "$PROJECT_GIT/modules/sub"
printf 'ref: refs/heads/main\n' >"$PROJECT_GIT/HEAD"
printf '%s\n' '[core]' '	repositoryformatversion = 0' '	bare = false' \
  '[remote "origin"]' '	url = https://git.example.com/acme/proj.git' \
  '	pushurl = git@git.example.com:acme/proj.git' \
  '	fetch = +refs/heads/*:refs/remotes/origin/*' \
  '[remote "upstream"]' '	url = https://git.example.org/upstream/proj.git' \
  '	fetch = +refs/heads/*:refs/remotes/upstream/*' \
  '[branch "main"]' '	remote = origin' '	merge = refs/heads/main' >"$PROJECT_GIT/config"
printf '%s\t\t%s\n' 1111111111111111111111111111111111111111 \
  "branch 'main' of https://git.example.com/acme/proj" >"$PROJECT_GIT/FETCH_HEAD"
printf '# pack-refs with: peeled fully-peeled sorted\n%s refs/tags/v1\n' \
  2222222222222222222222222222222222222222 >"$PROJECT_GIT/packed-refs"
printf '1111111111111111111111111111111111111111\n' >"$PROJECT_GIT/refs/heads/main"
printf '1111111111111111111111111111111111111111\n' >"$PROJECT_GIT/refs/remotes/origin/main"
printf '%s %s A <a@example.com> 1759449600 +0000\tclone: from %s\n' \
  0000000000000000000000000000000000000000 1111111111111111111111111111111111111111 \
  https://git.example.com/acme/proj.git >"$PROJECT_GIT/logs/HEAD"
printf '%s %s A <a@example.com> 1759449600 +0000\tfetch: fast-forward\n' \
  0000000000000000000000000000000000000000 1111111111111111111111111111111111111111 \
  >"$PROJECT_GIT/logs/refs/remotes/origin/main"
printf 'ref: refs/heads/wt1\n' >"$PROJECT_GIT/worktrees/wt1/HEAD"
printf '/srv/proj-wt1/.git\n' >"$PROJECT_GIT/worktrees/wt1/gitdir"
printf 'DIRC\n' >"$PROJECT_GIT/worktrees/wt1/index"
printf 'DIRC\n' >"$PROJECT_GIT/worktrees/wt1/index.lock"
printf 'DIRC\n' >"$PROJECT_GIT/worktrees/wt1/sharedindex.3333333333333333333333333333333333333333"
mkdir -p "$PROJECT_GIT/worktrees/wt1/modules/sub/objects/ab"
printf 'ref: refs/heads/main\n' >"$PROJECT_GIT/worktrees/wt1/modules/sub/HEAD"
printf 'object\n' >"$PROJECT_GIT/worktrees/wt1/modules/sub/objects/ab/cdef"
printf 'object\n' >"$PROJECT_GIT/objects/ab/cdef"
printf 'DIRC\n' >"$PROJECT_GIT/index"
printf '#!/bin/sh\n' >"$PROJECT_GIT/hooks/pre-commit.sample"
printf 'lfs\n' >"$PROJECT_GIT/lfs/objects/x"
printf 'ref: refs/heads/main\n' >"$PROJECT_GIT/modules/sub/HEAD"

# 2026-10-23 claude-code Windows round (#256)
CLAUDE_TEMP_SESSION="$ALICE/AppData/Local/Temp/claude/C--srv-proj/25625625-6256-4256-8256-256256256256"
mkdir -p "$CLAUDE_TEMP_SESSION/scratchpad" "$CLAUDE_TEMP_SESSION/tasks" "$ALICE/.claude/daemon" \
  "$ALICE/.claude/backups" "$ALICE/.claude/paste-cache" "$ALICE/.claude/plugins" "$PROJECT_GIT/info"
printf 'scratch\n' >"$CLAUDE_TEMP_SESSION/scratchpad/notes.txt"
printf 'done\n' >"$CLAUDE_TEMP_SESSION/tasks/b256b256b.output"
printf '{"oauthAccount":{}}\n' >"$ALICE/.claude.json.backup"
printf '0123456789abcdef0123456789abcdef' >"$ALICE/.claude/daemon/control.key"
printf '0123456789abcdef' >"$ALICE/.claude/daemon/pipe.key"
printf '{"proto":1,"workers":{}}\n' >"$ALICE/.claude/daemon/roster.json"
printf '{"oauthAccount":{}}\n' >"$ALICE/.claude/backups/.claude.json.backup.1792800000000"
printf '{"oauthAccount":{}}\n' >"$ALICE/.claude/backups/.claude.json.corrupted.2026-10-23T17-00-00-000Z"
printf 'pasted text\n' >"$ALICE/.claude/paste-cache/0123456789abcdef.txt"
printf '{"plugins":[]}\n' >"$ALICE/.claude/plugins/plugin-directory-cache-v2.json"
printf '# git ls-files --others --exclude-from=.git/info/exclude\n.claude/scheduled_tasks.lock\n' \
  >"$PROJECT_GIT/info/exclude"

# 2026-10-26 claude-code Linux round (#256)
mkdir -p "$ALICE/.local/state/claude/locks" "$ALICE/.cache/claude/staging"
printf '{"acquiredAt":1792800000000,"execPath":"/x","pid":1,"version":"2.1.310"}\n' \
  >"$ALICE/.local/state/claude/locks/2.1.310.lock"
printf 'staged\n' >"$ALICE/.cache/claude/staging/entry"

# 2026-10-26 claude-code macOS round (#256)
URL_HANDLER="$ALICE/Applications/Claude Code URL Handler.app/Contents"
BRIDGE_SPAWN="bridge-spawn/cse_0123456789abcdef0123"
mkdir -p "$URL_HANDLER/MacOS" "$ALICE/.claude/ide" "$ALICE/.claude/$BRIDGE_SPAWN" "$CLAUDE_WORK/ide" "$CLAUDE_WORK/$BRIDGE_SPAWN"
printf '<plist version="1.0"><dict><key>CFBundleURLSchemes</key></dict></plist>\n' \
  >"$URL_HANDLER/Info.plist"
ln -s /home/alice/.local/bin/claude "$URL_HANDLER/MacOS/claude"
for _claude_dir in "$ALICE/.claude" "$CLAUDE_WORK"; do
  printf '{"authToken":"00000000-0000-4000-8000-000000000256","transport":"ws"}\n' \
    >"$_claude_dir/ide/53256.lock"
  printf '{"mcpServers":{"claude-code-remote":{"headers":{"X-Session-UUID":"x"}}}}\n' \
    >"$_claude_dir/$BRIDGE_SPAWN/mcp.json"
done
printf 'appended system prompt\n' >"$ALICE/.claude/$BRIDGE_SPAWN/append-system-prompt.txt"

# 2026-10-29 drift sweep credential gaps (#248)
# Settings and MCP files that can hold MCP env, headers or OAuth values, one
# per secret glob, as path|agent lines relative to alice's home or to
# /srv/proj, read from files so the check loops run in this shell.
DRIFT_HOME_SECRETS="$WORK/drift-home-secrets"
DRIFT_PROJECT_SECRETS="$WORK/drift-project-secrets"
DRIFT_HERMES_DESKTOP="$WORK/drift-hermes-desktop"
cat >"$DRIFT_HOME_SECRETS" <<'EOF_DRIFT_HOME_SECRETS'
.claude/settings.json|claude-code
.claude/settings.local.json|claude-code
.gemini/settings.json|gemini-cli
.gemini/config/mcp_config.json|antigravity
.cursor/mcp.json|cursor
.copilot/settings.json|copilot-cli
.copilot/mcp-secrets/index.json|copilot-cli
.config/.copilot/config.json|copilot-cli
.config/.copilot/mcp-config.json|copilot-cli
.config/.copilot/mcp-oauth-config.json|copilot-cli
.config/Code/User/globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json|cline
.config/Code/User/globalStorage/rooveterinaryinc.roo-cline/settings/mcp_settings.json|roo-code
.vscode-mock/global-storage/settings/mcp_settings.json|roo-code
.config/amp/settings.json|amp
.config/amp/settings.jsonc|amp
.factory/settings.json|factory-droid
.factory/settings.local.json|factory-droid
.factory/mcp.json|factory-droid
.aws/amazonq/cli-agents/default.json|kiro
.openhands/conversations/a3/acp/codex/auth.json|openhands
.little-coder/mcp.json|little-coder
.hermes/config.yaml|hermes
.cache/langflow/0f0e0d0c-1111-4000-8000-000000000248/_mcp_servers_0f0e0d0c-1111-4000-8000-000000000248.json|langflow
Library/Caches/langflow/0f0e0d0c-1111-4000-8000-000000000248/_mcp_servers_0f0e0d0c-1111-4000-8000-000000000248.json|langflow
AppData/Local/langflow/langflow/Cache/0f0e0d0c-1111-4000-8000-000000000248/_mcp_servers_0f0e0d0c-1111-4000-8000-000000000248.json|langflow
.config/pydantic-clai2/mcp.json|pydantic-clai
.ollama/id_ed25519|ollama
.ollama/backup/codex/config.toml.1792800000|ollama
.ollama/launch/muse-config/muse/settings.json|ollama
EOF_DRIFT_HOME_SECRETS
cat >"$DRIFT_PROJECT_SECRETS" <<'EOF_DRIFT_PROJECT_SECRETS'
.claude/settings.json|claude-code
.claude/settings.local.json|claude-code
.mcp.json|project
.gemini/settings.json|gemini-cli
.vscode/mcp.json|vscode
.github/copilot/settings.json|copilot-cli
.github/copilot/settings.local.json|copilot-cli
.github/mcp.json|copilot-cli
opencode.json|opencode
.roo/mcp.json|roo-code
.amazonq/mcp.json|kiro
.amazonq/cli-agents/default.json|kiro
.factory/settings.json|factory-droid
.clai/mcp_servers.json|pydantic-clai
.amp/settings.json|amp
EOF_DRIFT_PROJECT_SECRETS
cat >"$DRIFT_HERMES_DESKTOP" <<'EOF_DRIFT_HERMES_DESKTOP'
.config/Hermes-canary
Library/Application Support/Hermes-canary
AppData/Roaming/Hermes-canary
EOF_DRIFT_HERMES_DESKTOP
while IFS='|' read -r _drift_path _drift_agent; do
  mkdir -p "$(dirname "$ALICE/$_drift_path")"
  printf '{"mcpServers":{"x":{"env":{"TOKEN":"t"}}}}\n' >"$ALICE/$_drift_path"
done <"$DRIFT_HOME_SECRETS"
while IFS='|' read -r _drift_path _drift_agent; do
  mkdir -p "$(dirname "$ROOT/srv/proj/$_drift_path")"
  printf '{"mcpServers":{"x":{"headers":{"Authorization":"Bearer t"}}}}\n' >"$ROOT/srv/proj/$_drift_path"
done <"$DRIFT_PROJECT_SECRETS"
while IFS= read -r _drift_path; do
  for _drift_leaf in Cache "Code Cache" GPUCache; do
    mkdir -p "$ALICE/$_drift_path/$_drift_leaf"; printf 'cache\n' >"$ALICE/$_drift_path/$_drift_leaf/data_0"
  done
  printf '{"connections":[]}\n' >"$ALICE/$_drift_path/connections.json"
done <"$DRIFT_HERMES_DESKTOP"
FACTORY_ENV_DUMP="$ALICE/.factory/temp/env/droid-env-s1-1.sh"
mkdir -p "$(dirname "$FACTORY_ENV_DUMP")" "$ALICE/.factory/temp/run1" "$ALICE/.cache/amp/runner-desktop/r1" \
  "$ALICE/.hermes-canary/hermes-agent/hermes_cli" "$ALICE/.hermes-canary/hermes-agent/.git" \
  "$ALICE/AppData/Local/hermes-canary/hermes-agent/hermes_cli" \
  "$ALICE/AppData/Local/hermes-canary/hermes-agent/.git" "$ALICE/AppData/Local/hermes-updater/pending" \
  "$ALICE/.cache/amp/desktop" "$ALICE/.cache/amp/bin" "$ALICE/.cache/amp/obelisk-runs" \
  "$ALICE/AppData/Roaming/devin" "$ALICE/AppData/Roaming/Devin/Cache" "$ALICE/AppData/Roaming/Devin/User" \
  "$ALICE/AppData/Local/devin/bin" "$ALICE/Library/Caches/ollama" "$ALICE/Library/Caches/com.electron.ollama" \
  "$ALICE/AppData/Local/crush" "$ROOT/srv/crushwin"
printf 'export TOKEN=t\n' >"$FACTORY_ENV_DUMP"
printf 'out\n' >"$ALICE/.factory/temp/run1/out.txt"
printf '{}\n' >"$ALICE/.cache/amp/runner-desktop/r1/state.json"
printf 'mcp_servers: {}\n' >"$ALICE/.hermes-canary/config.yaml"
printf 'sqlite\n' >"$ALICE/.hermes-canary/state.db"
printf 'x\n' >"$ALICE/.hermes-canary/hermes-agent/hermes_cli/main.py"
printf 'KEY=x\n' >"$ALICE/.hermes-canary/hermes-agent/.env"
printf 'ref\n' >"$ALICE/.hermes-canary/hermes-agent/.git/HEAD"
printf 'x\n' >"$ALICE/AppData/Local/hermes-canary/hermes-agent/hermes_cli/main.py"
printf 'ref\n' >"$ALICE/AppData/Local/hermes-canary/hermes-agent/.git/HEAD"
printf 'KEY=x\n' >"$ALICE/AppData/Local/hermes-canary/hermes-agent/.env"
printf '{}\n' >"$ALICE/.cache/amp/desktop/state.json"
printf 'ELF\n' >"$ALICE/.cache/amp/bin/amp"
printf '{}\n' >"$ALICE/.cache/amp/obelisk-runs/run.json"
printf 'mcp_servers: {}\n' >"$ALICE/AppData/Local/hermes-canary/config.yaml"
printf 'MZ\n' >"$ALICE/AppData/Local/hermes-updater/pending/hermes-setup.exe"
printf '{"mcpServers":{}}\n' >"$ALICE/AppData/Roaming/devin/mcp_config.json"
printf 'windsurf_api_key = "k"\n' >"$ALICE/AppData/Roaming/devin/credentials.toml"
printf 'cache\n' >"$ALICE/AppData/Roaming/Devin/Cache/data_0"
printf '{}\n' >"$ALICE/AppData/Roaming/Devin/User/settings.json"
printf 'MZ\n' >"$ALICE/AppData/Local/devin/bin/devin.exe"
printf 'cache\n' >"$ALICE/Library/Caches/ollama/data_0"
printf 'cache\n' >"$ALICE/Library/Caches/com.electron.ollama/data_0"
printf '{"projects":[{"path":"/srv/crushwin","data_dir":"/srv/crushwin/.crush"}]}\n' \
  >"$ALICE/AppData/Local/crush/projects.json"
for _drift_path in crushrc .crushrc crush.json .crush.json; do
  printf 'export KEY=k\n' >"$ROOT/srv/crushwin/$_drift_path"
done

run() { # run NAME ARGS...
  _name=$1; shift
  mkdir -p "$OUT/$_name"
  COLLECTOR_SH="$SHELL_UNDER_TEST" "$SHELL_UNDER_TEST" "$SCRIPT" -r "$ROOT" -o "$OUT/$_name" -q --max-file-size 1 "$@" >"$OUT/$_name.stdout" 2>"$OUT/$_name.stderr"
  _exit_code=$?
  MANIFEST=$(ls "$OUT/$_name"/*.manifest.jsonl 2>/dev/null)
  # shellcheck disable=SC2034 # ARCHIVE and SUMMARY are read by the eval'd check strings
  ARCHIVE=$(ls "$OUT/$_name"/*.tar.gz 2>/dev/null)
  # shellcheck disable=SC2034
  SUMMARY=$(ls "$OUT/$_name"/*.collection.json 2>/dev/null)
  return "$_exit_code"
}
# On a case-insensitive filesystem a path is recorded in its first catalog
# spelling, so the lookup ignores case there.
row() {
  if [ "$CASE_INSENSITIVE" = 1 ]; then grep -iF "\"path\":\"$1\"" "$MANIFEST"; else grep -F "\"path\":\"$1\"" "$MANIFEST"; fi | head -n1
}
# shellcheck disable=SC2317,SC2329 # called from the eval'd check strings
status_of() { row "$1" | sed 's/.*"status":"\([^"]*\)".*/\1/'; }

printf '== shell: %s (%s)\n' "$SHELL_UNDER_TEST" "$(command -v "$SHELL_UNDER_TEST")"

# ---- default run ----------------------------------------------------------
run default; check "exit code 0" "[ $? -eq 0 ]"
check "archive exists" "[ -s \"\$ARCHIVE\" ]"
check "sha256 sidecar matches" "[ \"\$(cut -d' ' -f1 \"\$ARCHIVE.sha256\")\" = \"\$(sha256sum \"\$ARCHIVE\" | cut -d' ' -f1)\" ]"
check "transcript collected" "[ \"\$(status_of \"$ROOT/home/alice/.claude/projects/-srv-proj/s1.jsonl\")\" = collected ]"
check "transcript hash correct" "[ \"\$(row \"$ROOT/home/alice/.claude/projects/-srv-proj/s1.jsonl\" | sed 's/.*\"sha256\":\"\([^\"]*\)\".*/\1/')\" = \"\$(sha256sum \"$ROOT/home/alice/.claude/projects/-srv-proj/s1.jsonl\" | cut -d' ' -f1)\" ]"
check "secret collected and flagged" "row \"$ROOT/home/alice/.claude/.credentials.json\" | grep -q '\"secret\":true,\"status\":\"collected\"'"
check "antigravity token flagged secret" "row \"$ROOT/home/alice/.gemini/antigravity-cli/antigravity-oauth-token\" | grep -q '\"secret\":true'"
check "cache dir skipped_excluded with size" "row \"$ROOT/home/alice/.claude/cache\" | grep -q '\"type\":\"dir\",\"size\":[1-9][0-9]*.*\"status\":\"skipped_excluded\"'"
check "cache contents not collected" "! grep -q 'cache/junk.bin' \"\$MANIFEST\""
check "nested entry attributed to its own agent" "row \"$ROOT/home/alice/.gemini/antigravity-cli/conversations/c1.db\" | grep -q '\"agent\":\"antigravity\"'"
check "nested entry collected exactly once" "[ \"\$(grep -c 'antigravity-cli/conversations/c1.db' \"\$MANIFEST\")\" = 1 ]"
check "enclosing entry still attributed to gemini-cli" "row \"$ROOT/home/alice/.gemini/projects.json\" | grep -q '\"agent\":\"gemini-cli\"'"
check "antigravity bin excluded" "[ \"\$(status_of \"$ROOT/home/alice/.gemini/antigravity-cli/bin\")\" = skipped_excluded ]"
check "ollama model blobs excluded" "[ \"\$(status_of \"$ROOT/home/ollama/.ollama/models/blobs\")\" = skipped_excluded ]"
check "ollama history collected (service account home)" "[ \"\$(status_of \"$ROOT/home/ollama/.ollama/history\")\" = collected ]"
check "oversized file skipped_size" "[ \"\$(status_of \"$ROOT/Users/bob/Library/Application Support/Cursor/User/globalStorage/state.vscdb\")\" = skipped_size ]"
check "ms-python globalStorage excluded" "[ \"\$(status_of \"$ROOT/Users/bob/Library/Application Support/Cursor/User/globalStorage/ms-python.python\")\" = skipped_excluded ]"
check "symlink recorded with target" "row \"$ROOT/home/alice/.gemini/antigravity-cli/cli.log\" | grep -q '\"type\":\"symlink\".*\"status\":\"symlink\",\"target\":\"log/cli-1.log\"'"
check "symlink present in archive" "tar -tzf \"\$ARCHIVE\" | grep -q 'antigravity-cli/cli.log'"
check "weird filename collected" "grep -q 'weird \\\\\"name\\\\\" with spaces \\\\\\\\ bs.txt.*\"status\":\"collected\"' \"\$MANIFEST\""
check "project from history.jsonl collected" "[ \"\$(status_of \"$ROOT/srv/proj/CLAUDE.md\")\" = collected ]"
check "project .claude settings collected" "[ \"\$(status_of \"$ROOT/srv/proj/.claude/settings.local.json\")\" = collected ]"
check "project owner attributed" "row \"$ROOT/srv/proj/CLAUDE.md\" | grep -q '\"user\":\"alice\"'"
# 2026-10-14 project files attributed to their agent (#104)
check "project .claude file attributed to claude-code" "row \"$ROOT/srv/proj/.claude/settings.local.json\" | grep -q '\"home\":\"$ROOT/srv/proj\",\"agent\":\"claude-code\".*\"status\":\"collected\"'"
check "shared project AGENTS.md keeps agent project" "row \"$ROOT/srv/droid/AGENTS.md\" | grep -q '\"home\":\"$ROOT/srv/droid\",\"agent\":\"project\".*\"status\":\"collected\"'"
check "project from cursor workspace.json collected" "[ \"\$(status_of \"$ROOT/Users/bob/Documents/app/.cursorrules\")\" = collected ]"
check "bob (no passwd entry) found via /Users glob" "grep -q '\"user\":\"bob\"' \"\$MANIFEST\""
check "nobody home skipped" "! grep -q '\"home\":\"$ROOT/nonexistent\"' \"\$MANIFEST\""
check "shell history not collected" "! grep -q '.bash_history' \"\$MANIFEST\""
check "continue sessions.json discovers project" "[ \"\$(status_of \"$ROOT/srv/cont/.continuerc.json\")\" = collected ]"
check "continue globalContext flagged secret" "row \"$ROOT/home/alice/.continue/index/globalContext.json\" | grep -q '\"secret\":true,\"status\":\"collected\"'"
check "continue lancedb excluded" "[ \"\$(status_of \"$ROOT/home/alice/.continue/index/lancedb\")\" = skipped_excluded ]"
check "crush projects.json discovers project-local crush.db" "[ \"\$(status_of \"$ROOT/srv/crushproj/.crush/crush.db\")\" = collected ]"
check "gemini projects.json / .project_root discovers project" "[ \"\$(status_of \"$ROOT/srv/gem/GEMINI.md\")\" = collected ]"
check "cline session cwd discovers project" "[ \"\$(status_of \"$ROOT/srv/clineproj/.clinerules\")\" = collected ]"
check "cline providers.json flagged secret" "row \"$ROOT/home/alice/.cline/data/settings/providers.json\" | grep -q '\"secret\":true'"
check "opencode auth.json flagged secret" "row \"$ROOT/home/alice/.local/share/opencode/auth.json\" | grep -q '\"secret\":true'"
check "home .env not collected" "[ -z \"\$(row \"$ROOT/home/alice/.env\")\" ]"
check "claude desktop config.json flagged secret" "row \"$ROOT/Users/bob/Library/Application Support/Claude/config.json\" | grep -q '\"secret\":true'"
check "claude desktop mcp config flagged secret" "row \"$ROOT/Users/bob/Library/Application Support/Claude/claude_desktop_config.json\" | grep -q '\"secret\":true,\"status\":\"collected\"'"
check "claude desktop Cache excluded" "[ \"\$(status_of \"$ROOT/Users/bob/Library/Application Support/Claude/Cache\")\" = skipped_excluded ]"
check "zed extensions excluded via [Zz]ed glob" "[ \"\$(status_of \"$ROOT/Users/bob/Library/Application Support/Zed/extensions\")\" = skipped_excluded ]"
check "zed threads.db collected" "[ \"\$(status_of \"$ROOT/Users/bob/Library/Application Support/Zed/threads/threads.db\")\" = collected ]"
check "zed settings.json flagged secret" "row \"$ROOT/Users/bob/.config/zed/settings.json\" | grep -q '\"secret\":true'"
check "cline globalStorage attributed to cline, not cursor" "row \"$ROOT/Users/bob/Library/Application Support/Cursor/User/globalStorage/saoudrizwan.claude-dev/state/taskHistory.json\" | grep -q '\"agent\":\"cline\"'"
check "cline globalStorage collected exactly once" "[ \"\$(grep -c 'Cursor/User/globalStorage/saoudrizwan.claude-dev/state/taskHistory.json' \"\$MANIFEST\")\" = 1 ]"
check "cursor state.vscdb still attributed to cursor" "row \"$ROOT/Users/bob/Library/Application Support/Cursor/User/globalStorage/state.vscdb\" | grep -q '\"agent\":\"cursor\"'"
check "factory session index not swallowed by cache exclusion" "[ \"\$(status_of \"$ROOT/home/alice/.factory/cache/session-index/index.db\")\" = collected ]"
check "factory cache sounds excluded" "[ \"\$(status_of \"$ROOT/home/alice/.factory/cache/sounds\")\" = skipped_excluded ]"
check "factory session cwd discovers project" "[ \"\$(status_of \"$ROOT/srv/droid/AGENTS.md\")\" = collected ]"
check "kiro power mcp.json collected and flagged" "row \"$ROOT/home/alice/.kiro/powers/installed/p1/mcp.json\" | grep -q '\"secret\":true,\"status\":\"collected\"'"
check "kiro cli session cwd discovers project" "[ \"\$(status_of \"$ROOT/srv/kiroproj/.kiroignore\")\" = collected ]"
check "vscode-server User dir collected as vscode" "row \"$ROOT/home/alice/.vscode-server/data/User/globalStorage/state.vscdb\" | grep -q '\"agent\":\"vscode\"'"
check "cline inside vscode-server attributed to cline" "row \"$ROOT/home/alice/.vscode-server/data/User/globalStorage/saoudrizwan.claude-dev/state/taskHistory.json\" | grep -q '\"agent\":\"cline\"'"
check "vscode-server taskHistory discovers project" "[ \"\$(status_of \"$ROOT/srv/remoteproj/.clinerules\")\" = collected ]"
check "antigravity-ide attributed to antigravity" "row \"$ROOT/home/alice/.gemini/antigravity-ide/conversations/i1.db\" | grep -q '\"agent\":\"antigravity\"'"
check "gemini config dir attributed to antigravity" "row \"$ROOT/home/alice/.gemini/config/mcp_config.json\" | grep -q '\"agent\":\"antigravity\"'"
check "antigravity history workspace discovers project" "[ \"\$(status_of \"$ROOT/srv/agyproj/.antigravityignore\")\" = collected ]"
check "cursor auth.json flagged secret" "row \"$ROOT/home/alice/.cursor/auth.json\" | grep -q '\"secret\":true'"
check "cursor project mcp-auth.json flagged secret" "row \"$ROOT/home/alice/.cursor/projects/-srv-proj/mcp-auth.json\" | grep -q '\"secret\":true'"
check "copilot pkg excluded" "[ \"\$(status_of \"$ROOT/home/alice/.copilot/pkg\")\" = skipped_excluded ]"
check "copilot config.json flagged secret" "row \"$ROOT/home/alice/.copilot/config.json\" | grep -q '\"secret\":true'"
check "copilot workspace.yaml cwd discovers project" "[ \"\$(status_of \"$ROOT/srv/copproj/AGENTS.md\")\" = collected ]"
check "claude device keys flagged secret" "row \"$ROOT/home/alice/.claude/.device-keys.json\" | grep -q '\"secret\":true'"
check "claude image-cache excluded" "[ \"\$(status_of \"$ROOT/home/alice/.claude/image-cache\")\" = skipped_excluded ]"
check "amp history cwd discovers project" "[ \"\$(status_of \"$ROOT/srv/ampproj/.amp/settings.json\")\" = collected ]"
check "windsurf transcripts collected" "row \"$ROOT/home/alice/.windsurf/transcripts/t1.jsonl\" | grep -q '\"agent\":\"windsurf\".*\"status\":\"collected\"'"
check "project-level kilo worktrees excluded" "[ \"\$(status_of \"$ROOT/srv/kiloproj/.kilo/worktrees\")\" = skipped_excluded ]"
check "ollama desktop app dir collected" "[ \"\$(status_of \"$ROOT/Users/bob/Library/Application Support/Ollama/db.sqlite\")\" = collected ]"
check "cline taskHistory in globalStorage discovers project" "[ \"\$(status_of \"$ROOT/Users/bob/clineproj/.clinerules\")\" = collected ]"
check "cline checkpoints in globalStorage excluded" "[ \"\$(status_of \"$ROOT/Users/bob/Library/Application Support/Cursor/User/globalStorage/saoudrizwan.claude-dev/checkpoints\")\" = skipped_excluded ]"
# 2026-10-04 catalog cases
printf '%s\n' "$CASES_CATALOG" >"$WORK/cases14"
while IFS='|' read -r _kind _agent _relpath; do
  [ -n "$_relpath" ] || continue
  _row=$(row "$ALICE/$_relpath")
  case "$_kind" in
    excl|exclf) case "$_row" in *'"status":"skipped_excluded"'*) pass "excluded: $_relpath" ;; *) fail_check "excluded: $_relpath" ;; esac ;;
    secret) case "$_row" in *'"secret":true,"status":"collected"'*) pass "secret: $_relpath" ;; *) fail_check "secret: $_relpath" ;; esac ;;
    agent) case "$_row" in *"\"agent\":\"$_agent\""*'"status":"collected"'*) pass "$_agent: $_relpath" ;; *) fail_check "$_agent: $_relpath" ;; esac ;;
  esac
done <"$WORK/cases14"
check "pearai lowercase .config/pearai collected" "[ \"\$(grep -ci '/.config/pearai/User/globalStorage/state.vscdb\"' \"\$MANIFEST\")\" = 1 ]"
check "cody nested globalStorage collected exactly once" "[ \"\$(grep -c 'sourcegraph.cody-ai/symf/indexroot/meta.json' \"\$MANIFEST\")\" = 1 ]"
check "twinny nested globalStorage collected exactly once" "[ \"\$(grep -c 'globalStorage/rjmacarthy.twinny/twinny-providers.json' \"\$MANIFEST\")\" = 1 ]"
check "little-coder history nested in .pi collected exactly once" "[ \"\$(grep -c 'little-coder-prompt-history.json' \"\$MANIFEST\")\" = 1 ]"
check "hermes installer checkout excluded but its .env kept" "[ \"\$(status_of \"$ALICE/.hermes/hermes-agent/hermes_cli\")\" = skipped_excluded ] && [ \"\$(status_of \"$ALICE/.hermes/hermes-agent/.env\")\" = collected ]"
check "openclaw .clawdbot rename symlink recorded" "row \"$BOB/.clawdbot\" | grep -q '\"agent\":\"openclaw\".*\"type\":\"symlink\".*\"status\":\"symlink\",\"target\":\".openclaw\"'"
check "hermes checkpoint workdir discovers project" "row \"$ROOT/srv/hermesproj/.hermes.md\" | grep -q '\"user\":\"alice\".*\"status\":\"collected\"'"
# 2026-10-30 hermes rebuild (#326)
for _hermes_file in HERMES.md .hermes/environment.json trajectory_samples.jsonl failed_trajectories.jsonl; do
  check "#326 Hermes project file collected: $_hermes_file" "row \"$ROOT/srv/hermesproj/$_hermes_file\" | grep -q '\"agent\":\"hermes\".*\"status\":\"collected\"'"
done
check "letta sessions.jsonl project discovers project" "[ \"\$(status_of \"$ROOT/srv/lettaproj/.letta/settings.json\")\" = collected ]"
check "project .letta/settings.json flagged secret (accepted overmatch)" "row \"$ROOT/srv/lettaproj/.letta/settings.json\" | grep -q '\"secret\":true'"
check "letta local backend messages cwd discovers project" "[ \"\$(status_of \"$ROOT/srv/lettaproj2/AGENTS.md\")\" = collected ]"
check "pi session cwd discovers project" "[ \"\$(status_of \"$ROOT/srv/piproj/.pi/settings.json\")\" = collected ]"
check "pi misplaced session cwd discovers project" "[ \"\$(status_of \"$ROOT/srv/piproj2/AGENTS.md\")\" = collected ]"
check "pi trust.json keys discover project" "[ \"\$(status_of \"$ROOT/srv/pitrust/pi-session-2026-10-04.html\")\" = collected ]"
check "pi crashes.json cwd discovers project" "[ \"\$(status_of \"$ROOT/srv/picrash/AGENTS.md\")\" = collected ]"
check "pi experimental meta cwd discovers project" "[ \"\$(status_of \"$ROOT/srv/piexp/AGENTS.md\")\" = collected ]"
check "open-interpreter rollout cwd discovers project" "[ \"\$(status_of \"$ROOT/srv/oiproj/.openinterpreter/rules/r.md\")\" = collected ]"
check "open-interpreter archived rollout cwd discovers project" "[ \"\$(status_of \"$ROOT/srv/oiarch/.codewhale/notes.md\")\" = collected ]"
check "openhands workspaces.json path discovers project" "[ \"\$(status_of \"$ROOT/srv/ohproj/.openhands/setup.sh\")\" = collected ]"
check "openhands canvas dev conversation discovers project" "[ \"\$(status_of \"$ROOT/srv/ohdev/AGENTS.md\")\" = collected ]"
check "openhands canvas conversation discovers project" "[ \"\$(status_of \"$ROOT/srv/ohcanvas/AGENTS.md\")\" = collected ]"
check "openhands cli base_state discovers project" "[ \"\$(status_of \"$ROOT/srv/ohcli/AGENTS.md\")\" = collected ]"
check "pearai sessions.json discovers project" "[ \"\$(status_of \"$ROOT/srv/pearproj/.pearairc.json\")\" = collected ]"
check "project .pearai-agent/mcp.json flagged secret" "row \"$ROOT/srv/pearproj/.pearai-agent/mcp.json\" | grep -q '\"secret\":true,\"status\":\"collected\"'"
check "openclaw JSON5 workspace discovers project" "[ \"\$(status_of \"$ROOT/srv/clawws/SOUL.md\")\" = collected ]"
check "openclaw JSON5 agentDir discovers project" "[ \"\$(status_of \"$ROOT/srv/clawagent/AGENTS.md\")\" = collected ]"
check "openclaw legacy clawdbot.json discovers project" "[ \"\$(status_of \"$ROOT/srv/clawlegacy/IDENTITY.md\")\" = collected ]"
check "openclaw profile config discovers project" "[ \"\$(status_of \"$ROOT/srv/clawprofile/MEMORY.md\")\" = collected ]"
check "nanobot .workspace marker discovers project" "[ \"\$(status_of \"$ROOT/srv/nanows/HEARTBEAT.md\")\" = collected ]"
check "nanobot instance config workspace discovers project" "[ \"\$(status_of \"$ROOT/srv/nanocfg/memory/MEMORY.md\")\" = collected ]"
check "tabby config.toml file:// git_url discovers project" "[ \"\$(status_of \"$ROOT/srv/tabbyproj/AGENTS.md\")\" = collected ]"
check "twinny manifest common prefix discovers project" "[ \"\$(status_of \"$ROOT/srv/twinproj/AGENTS.md\")\" = collected ]"
# 2026-10-04 Docker volumes
AGENT_ZERO_VOLUME="$VOLUMES/a0_usr/_data"
check "docker volume file collected as agent-zero, user docker" "row \"$AGENT_ZERO_VOLUME/chats/ctx1/chat.json\" | grep -q '\"user\":\"docker\",\"home\":\"$AGENT_ZERO_VOLUME\",\"agent\":\"agent-zero\".*\"status\":\"collected\"'"
check "docker volume file archived under fs/<original path>" "tar -tzf \"\$ARCHIVE\" | grep -q '^./fs/var/lib/docker/volumes/a0_usr/_data/chats/ctx1/chat.json\$'"
check "docker volume .env flagged secret (volume-relative)" "row \"$AGENT_ZERO_VOLUME/.env\" | grep -q '\"secret\":true,\"status\":\"collected\"'"
check "docker volume tmp/playwright excluded (volume-relative)" "[ \"\$(status_of \"$AGENT_ZERO_VOLUME/tmp/playwright\")\" = skipped_excluded ]"
check "unmatched volume recorded skipped_unmatched_volume with size" "row \"$VOLUMES/pgdata/_data\" | grep -q '\"user\":\"docker\".*\"type\":\"dir\",\"size\":[1-9][0-9]*.*\"status\":\"skipped_unmatched_volume\"'"
check "unmatched volume contents not collected" "! grep -q 'pgdata/_data/x' \"\$MANIFEST\""
check "volumes dir metadata.db not collected" "! grep -q 'volumes/metadata.db' \"\$MANIFEST\""
check "rootless volume collected as tabby" "row \"$ALICE/.local/share/docker/volumes/tabby_data/_data/events/2026-10-01.json\" | grep -q '\"user\":\"docker\".*\"agent\":\"tabby\".*\"status\":\"collected\"'"
check "summary counts skipped_unmatched_volume" "grep -q '\"skipped_unmatched_volume\": 1' \"\$SUMMARY\""
check "summary docker counts" "grep -q '\"docker\": {\"volumes_found\": 3, \"volumes_collected\": 2, \"unreadable\": 0' \"\$SUMMARY\""
check "summary no_docker option false" "grep -q '\"no_docker\": false' \"\$SUMMARY\""
check "stdout docker line" "grep -q '^docker:     3 volumes found, 2 collected, 0 unreadable\$' \"$OUT/default.stdout\""
# 2026-10-04 summary: only homes with manifest rows are counted and logged; sizes are also human-readable
check "home without artifacts still listed in users" "tr -d '\n' <\"\$SUMMARY\" | grep -q '\"users\": \\[[^]]*\"dave\"'"
check "home without artifacts not in users_with_artifacts" "grep -q '\"users_with_artifacts\": \\[\"' \"\$SUMMARY\" && ! grep '\"users_with_artifacts\"' \"\$SUMMARY\" | grep -q dave"
check "stdout users counts users_with_artifacts" "[ \"\$(sed -n 's/^users: *//p' \"$OUT/default.stdout\")\" = \"\$(grep '\"users_with_artifacts\"' \"\$SUMMARY\" | tr ',' '\\n' | grep -c '\"')\" ]"
check "log has no progress line for a home without artifacts" "! grep -q 'User dave' \"$OUT\"/default/*.log && grep -q 'User alice' \"$OUT\"/default/*.log"
check "stdout size is human-readable" "grep -q '^size:       [0-9]* ([0-9.]* \\(B\\|KiB\\|MiB\\))\$' \"$OUT/default.stdout\""
check "stdout collected bytes are human-readable" "grep -q '^collected:  [0-9]* files, [0-9]* bytes ([0-9.]* \\(B\\|KiB\\|MiB\\))\$' \"$OUT/default.stdout\""
check "log done line keeps size and sha256" "grep -q ' done: .* ([0-9]* bytes, sha256 [0-9a-f]\\{64\\})\$' \"$OUT\"/default/*.log"
# 2026-10-04 Muse Code
printf '%s\n' "$CASES_MUSE" >"$WORK/casesmuse"
while IFS='|' read -r _kind _agent _relpath; do
  [ -n "$_relpath" ] || continue
  _row=$(row "$ALICE/$_relpath")
  case "$_kind" in
    excl|exclf) case "$_row" in *'"agent":"muse-code"'*'"status":"skipped_excluded"'*) pass "excluded: $_relpath" ;; *) fail_check "excluded: $_relpath" ;; esac ;;
    secret) case "$_row" in *'"agent":"muse-code"'*'"secret":true,"status":"collected"'*) pass "secret: $_relpath" ;; *) fail_check "secret: $_relpath" ;; esac ;;
    agent) case "$_row" in *"\"agent\":\"$_agent\""*'"status":"collected"'*) pass "$_agent: $_relpath" ;; *) fail_check "$_agent: $_relpath" ;; esac ;;
  esac
done <"$WORK/casesmuse"
check "muse windows binary excluded with its size" "row \"$ALICE/AppData/Local/Programs/muse/muse-bin-1.4.2-R1.exe\" | grep -q '\"type\":\"file\",\"size\":2,.*\"status\":\"skipped_excluded\"'"
check "muse .local/bin glob leaves other tools alone" "! grep -q '/.local/bin/othertool\"' \"\$MANIFEST\""
check "muse trust.json projects key discovers project" "row \"$ROOT/srv/musetrust/AGENTS.md\" | grep -q '\"user\":\"alice\".*\"status\":\"collected\"'"
check "muse session.jsonl workspace_root discovers project" "row \"$ROOT/srv/museproj/.muse/hooks.json\" | grep -q '\"user\":\"alice\",\"home\":\"$ROOT/srv/museproj\",\"agent\":\"muse-code\".*\"status\":\"collected\"'"
check "muse project .muse/worktrees excluded" "[ \"\$(status_of \"$ROOT/srv/museproj/.muse/worktrees\")\" = skipped_excluded ]"
# 2026-10-04 relocated Claude Code home
check "relocated claude home session collected as claude-code" "row \"$CLAUDE_WORK_SESSION\" | grep -q '\"agent\":\"claude-code\".*\"status\":\"collected\"'"
check "relocated claude home history collected as claude-code" "row \"$CLAUDE_WORK/history.jsonl\" | grep -q '\"agent\":\"claude-code\".*\"status\":\"collected\"'"
check "relocated claude home credentials flagged secret" "row \"$CLAUDE_WORK/.credentials.json\" | grep -q '\"agent\":\"claude-code\".*\"secret\":true,\"status\":\"collected\"'"
check "relocated claude home .claude.json flagged secret" "row \"$CLAUDE_WORK/.claude.json\" | grep -q '\"agent\":\"claude-code\".*\"secret\":true,\"status\":\"collected\"'"
# 2026-10-23 (#256)
check "#256 relocated claude home session key, daemon files and backup flagged secret" "row \"$CLAUDE_WORK_KEY\" | grep -q '\"agent\":\"claude-code\".*\"secret\":true,\"status\":\"collected\"' && row \"$CLAUDE_WORK/daemon/control.key\" | grep -q '\"agent\":\"claude-code\".*\"secret\":true,\"status\":\"collected\"' && row \"$CLAUDE_WORK/daemon/roster.json\" | grep -q '\"agent\":\"claude-code\".*\"secret\":true,\"status\":\"collected\"' && row \"$CLAUDE_WORK/backups/.claude.json.backup.1792800000000\" | grep -q '\"agent\":\"claude-code\".*\"secret\":true,\"status\":\"collected\"'"
check "claude-code-router not collected" "! grep -q '/.claude-code-router/' \"\$MANIFEST\""
check "relocated .claude.json projects key discovers project" "row \"$ROOT/srv/ccwork/.claude/settings.json\" | grep -q '\"user\":\"alice\".*\"agent\":\"claude-code\".*\"status\":\"collected\"'"
check "relocated history.jsonl discovers project" "row \"$ROOT/srv/cchist/.claude/settings.json\" | grep -q '\"user\":\"alice\".*\"agent\":\"claude-code\".*\"status\":\"collected\"'"
check "no live snapshot directory" "! tar -tzf \"\$ARCHIVE\" | grep -q '^./live/'"
check "summary has no no_live option" "! grep -q no_live \"\$SUMMARY\""
check "manifest and summary inside archive" "tar -tzf \"\$ARCHIVE\" | grep -q '^./manifest.jsonl' && tar -tzf \"\$ARCHIVE\" | grep -q '^./collection.json'"
check "staging removed" "[ -z \"\$(ls -d \"$OUT/default\"/.stage-* 2>/dev/null)\" ]"
check "no copy errors" "grep -q '\"error_copy\": 0' \"\$SUMMARY\""
if command -v python3 >/dev/null 2>&1; then
  check "manifest is valid JSONL" "python3 -c 'import json,sys; [json.loads(line) for line in open(sys.argv[1])]' \"\$MANIFEST\""
  check "summary is valid JSON" "python3 -c 'import json,sys; json.load(open(sys.argv[1]))' \"\$SUMMARY\""
  check "archived file bytes match staged hash" "cd \"\$WORK\" && mkdir -p x && tar -xzf \"\$ARCHIVE\" -C x && python3 - \"\$MANIFEST\" x <<'PY'
import json,sys,hashlib,os
manifest,root=sys.argv[1],sys.argv[2]; count=0
for line in open(manifest):
    row=json.loads(line)
    if row['status']!='collected' or not row['archive_path']: continue
    path=os.path.join(root,row['archive_path'])
    assert hashlib.sha256(open(path,'rb').read()).hexdigest()==row['sha256'], path
    assert os.path.getsize(path)==row['size'], path
    count+=1
assert count>5
PY"
fi

# 2026-10-04 bug round: #29
# shellcheck disable=SC2317,SC2329 # called from the eval'd check strings
count_of() { grep -ci "/$1\"" "$MANIFEST"; }
if [ "$CASE_INSENSITIVE" = 1 ]; then
  printf 'note: %s is case-insensitive; checking the folded layout\n' "$WORK"
  check "#29 case variants of one directory collected once" "[ \"\$(count_of case29-upper.json)\" = 1 ] && [ \"\$(count_of case29-lower.json)\" = 1 ]"
  check "#29 first catalog spelling wins" "grep -qF '\"path\":\"$ALICE/.config/PearAI/User/case29-lower.json\"' \"\$MANIFEST\""
  check "#29 nested case variant collected once under the nested entry" "[ \"\$(count_of case29-editor.json)\" = 1 ] && grep -qF '\"path\":\"$ALICE/.config/cursor/User/case29-editor.json\"' \"\$MANIFEST\""
  check "#29 dropped variant logged" "grep -q '/.config/pearai/User: same file as .*/.config/PearAI/User (case-insensitive filesystem)' \"$OUT\"/default/*.log"
else
  check "#29 PearAI and pearai are distinct here and both collected" "[ \"\$(status_of \"$ALICE/.config/PearAI/User/case29-upper.json\")\" = collected ] && [ \"\$(status_of \"$ALICE/.config/pearai/User/case29-lower.json\")\" = collected ] && [ \"\$(count_of case29-upper.json)\" = 1 ] && [ \"\$(count_of case29-lower.json)\" = 1 ]"
  check "#29 nested Cursor/User distinct from cursor and both collected" "row \"$ALICE/.config/cursor/case29-cli.json\" | grep -q '\"agent\":\"cursor\".*\"status\":\"collected\"' && row \"$ALICE/.config/Cursor/User/case29-editor.json\" | grep -q '\"agent\":\"cursor\".*\"status\":\"collected\"' && [ \"\$(count_of case29-editor.json)\" = 1 ]"
  check "#29 nothing folded on a case-sensitive filesystem" "! grep -q 'case-insensitive filesystem' \"$OUT\"/default/*.log"
fi

# 2026-10-06 Ollama (#44)
check "ollama model manifest collected" "row \"$OLLAMA_MANIFEST\" | grep -q '\"agent\":\"ollama\".*\"status\":\"collected\"'"
check "ollama blob not collected" "! grep -q 'blobs/sha256-abc' \"\$MANIFEST\""
check "ollama /usr/share/ollama home from passwd collected" "row \"$ROOT/usr/share/ollama/.ollama/history\" | grep -q '\"user\":\"svc-ollama\",\"home\":\"$ROOT/usr/share/ollama\",\"agent\":\"ollama\".*\"status\":\"collected\"'"
check "ollama /usr/share/ollama blobs excluded" "[ \"\$(status_of \"$ROOT/usr/share/ollama/.ollama/models/blobs\")\" = skipped_excluded ]"

# 2026-10-06 frameworks sweep (#64)
printf '%s\n' "$CASES_FW" >"$WORK/casesfw"
while IFS='|' read -r _kind _agent _relpath; do
  [ -n "$_relpath" ] || continue
  _row=$(row "$ALICE/$_relpath")
  case "$_kind" in
    excl|exclf) case "$_row" in *'"status":"skipped_excluded"'*) pass "excluded: $_relpath" ;; *) fail_check "excluded: $_relpath" ;; esac ;;
    secret) case "$_row" in *'"secret":true,"status":"collected"'*) pass "secret: $_relpath" ;; *) fail_check "secret: $_relpath" ;; esac ;;
    agent) case "$_row" in *"\"agent\":\"$_agent\""*'"status":"collected"'*) pass "$_agent: $_relpath" ;; *) fail_check "$_agent: $_relpath" ;; esac ;;
  esac
done <"$WORK/casesfw"
check "crewai mid-path wildcard leaves other files in the project dir alone" "! grep -q '/.local/share/myproj/unrelated.txt\"' \"\$MANIFEST\""
check "crewai lowercase AppData/Local/crewai collected through the class" "row \"$BOB/AppData/Local/crewai/myproj/latest_kickoff_task_outputs.db\" | grep -q '\"agent\":\"crewai\".*\"status\":\"collected\"'"
check "crewai lowercase credentials flagged secret through the class" "row \"$BOB/AppData/Local/crewai/credentials/tokens.enc\" | grep -q '\"secret\":true,\"status\":\"collected\"'"
check "camel-ai narrow lines leave Apache Camel JBang files alone" "! grep -q '/.camel/4242-status.json\"' \"\$MANIFEST\""
check "legacy open-interpreter space-named dir collected once" "[ \"\$(grep -c '/.config/Open Interpreter Terminal/conversations/old.json\"' \"\$MANIFEST\")\" = 1 ]"

# 2026-10-11 Claude Desktop Linux deb and Claude Code 2.1.295 session keys (#73, #76)
check "#73 Linux spaces-present link-key flagged secret" "row \"$CLAUDE_LINUX_CONFIG/spaces-present/0a0a0a0a/0b0b0b0b/link-key\" | grep -q '\"agent\":\"claude-desktop\".*\"secret\":true,\"status\":\"collected\"'"
check "#76 Claude Code sessions/<pid>.<hash>.key flagged secret" "row \"$SESSION_KEY\" | grep -q '\"agent\":\"claude-code\".*\"secret\":true,\"status\":\"collected\"'"
check "#76 Claude Code sessions/<pid>.json collected, not secret" "row \"$ALICE/.claude/sessions/4242.json\" | grep -q '\"agent\":\"claude-code\".*\"secret\":false,\"status\":\"collected\"'"
check "#73 Linux git-shadow/cli-temp-roots.json collected as claude-desktop" "row \"$CLAUDE_LINUX_CONFIG/git-shadow/cli-temp-roots.json\" | grep -q '\"agent\":\"claude-desktop\".*\"secret\":false,\"status\":\"collected\"'"
check "#73 Linux logs/main.log inside the data dir collected as claude-desktop" "row \"$CLAUDE_LINUX_CONFIG/logs/main.log\" | grep -q '\"agent\":\"claude-desktop\".*\"secret\":false,\"status\":\"collected\"'"
check "#73 Linux Code-tab session record collected as claude-desktop" "row \"$CLAUDE_LINUX_CONFIG/claude-code-sessions/$SPACE_IDS/local_76767676-7676-4676-8676-767676767676.json\" | grep -q '\"agent\":\"claude-desktop\".*\"secret\":false,\"status\":\"collected\"'"
check "#73 Linux claude-code excluded, downloaded CLI not in the manifest" "[ \"\$(status_of \"$CLAUDE_LINUX_CONFIG/claude-code\")\" = skipped_excluded ] && ! grep -q '/9b32b47ec4b3/' \"\$MANIFEST\""

# 2026-10-11 Claude Code CLI cache directory (#81)
check "#81 Claude Code CLI cache MCP log collected as claude-code" "row \"$CLI_CACHE_LOG\" | grep -q '\"agent\":\"claude-code\".*\"status\":\"collected\"'"
check "#81 Claude Code CLI cache MCP log not secret" "row \"$CLI_CACHE_LOG\" | grep -q '\"secret\":false,\"status\":\"collected\"'"
check "#81 Claude Code CLI cache Windows Cache child collected as claude-code" "row \"$CLI_CACHE_LOG_WINDOWS\" | grep -q '\"agent\":\"claude-code\".*\"status\":\"collected\"'"

# 2026-10-13 Antigravity desktop app (#83, #84)
check "#83 Antigravity app conversation collected as antigravity" "row \"$ANTIGRAVITY_DATA/conversations/a1.db\" | grep -q '\"agent\":\"antigravity\".*\"status\":\"collected\"'"
check "#83 Antigravity app crashes/ collected" "[ \"\$(status_of \"$ANTIGRAVITY_DATA/crashes/crash_1.log\")\" = collected ]"
check "#84 Antigravity plugin token.json flagged secret" "row \"$ANTIGRAVITY_DATA/plugin_data/gdocs/token.json\" | grep -q '\"agent\":\"antigravity\".*\"secret\":true,\"status\":\"collected\"'"
check "#83 Antigravity plugin binary excluded" "[ \"\$(status_of \"$ANTIGRAVITY_DATA/plugin_data/gdocs/bin\")\" = skipped_excluded ] && ! grep -q 'gdocs/bin/gdocs-1\"' \"\$MANIFEST\""
check "#83 Antigravity app bin excluded" "[ \"\$(status_of \"$ANTIGRAVITY_DATA/bin\")\" = skipped_excluded ] && ! grep -q 'antigravity/bin/webm_encoder\"' \"\$MANIFEST\""
check "#83 Antigravity marketplace cache excluded, other cache files collected" "[ \"\$(status_of \"$ANTIGRAVITY_DATA/cache/marketplaces\")\" = skipped_excluded ] && [ \"\$(status_of \"$ANTIGRAVITY_DATA/cache/last_conversations.json\")\" = collected ]"
check "#83 Antigravity Electron app_storage.json collected as antigravity" "row \"$ANTIGRAVITY_ELECTRON/app_storage.json\" | grep -q '\"agent\":\"antigravity\".*\"status\":\"collected\"'"
check "#83 Antigravity Electron Cache and Code Cache excluded" "[ \"\$(status_of \"$ANTIGRAVITY_ELECTRON/Cache\")\" = skipped_excluded ] && [ \"\$(status_of \"$ANTIGRAVITY_ELECTRON/Code Cache\")\" = skipped_excluded ] && ! grep -q 'Antigravity/Cache/data_0\"' \"\$MANIFEST\" && ! grep -q 'Antigravity/Code Cache/js/x\"' \"\$MANIFEST\""
check "#83 Antigravity Library/Logs main.log collected as antigravity" "row \"$ALICE/Library/Logs/Antigravity/main.log\" | grep -q '\"agent\":\"antigravity\".*\"status\":\"collected\"'"
check "#83 Antigravity Windows app_storage.json collected as antigravity" "row \"$ANTIGRAVITY_WINDOWS/app_storage.json\" | grep -q '\"agent\":\"antigravity\".*\"status\":\"collected\"'"
check "#83 Antigravity Windows Electron Cache excluded" "[ \"\$(status_of \"$ANTIGRAVITY_WINDOWS/Cache\")\" = skipped_excluded ]"
check "#83 Antigravity data dir cache/ not pruned by the Electron Cache exclusion" "[ \"\$(status_of \"$ANTIGRAVITY_DATA/cache/last_conversations.json\")\" = collected ] && [ \"\$(status_of \"$ANTIGRAVITY_DATA/cache\")\" != skipped_excluded ]"

# 2026-10-13 Antigravity Linux round (#83, #90, #91)
check "#90 Antigravity CLI plugin-secrets.json flagged secret" "row \"$ALICE/.config/antigravity/plugin-secrets.json\" | grep -q '\"agent\":\"antigravity\".*\"secret\":true,\"status\":\"collected\"'"
check "#90 Antigravity CLI plugin-secrets.json collected once" "[ \"\$(grep -c 'plugin-secrets.json\"' \"\$MANIFEST\")\" = 1 ]"
check "#83 Antigravity Electron Dictionaries excluded" "[ \"\$(status_of \"$ANTIGRAVITY_ELECTRON/Dictionaries\")\" = skipped_excluded ] && ! grep -q 'en-US-10-1.bdic\"' \"\$MANIFEST\""
check "#83 Antigravity app sidecar SDK under lib/node_modules excluded" "[ \"\$(status_of \"$ANTIGRAVITY_DATA/lib/node_modules\")\" = skipped_excluded ] && ! grep -q 'antigravity/lib/node_modules/sidecar_sdk/index.mjs\"' \"\$MANIFEST\""
check "#83 Antigravity config/node_modules excluded" "[ \"\$(status_of \"$ALICE/.gemini/config/node_modules\")\" = skipped_excluded ] && ! grep -q 'config/node_modules/sidecar_sdk/index.mjs\"' \"\$MANIFEST\""
check "#91 Antigravity project folderUri discovers project" "[ \"\$(status_of \"$ROOT/srv/agyapp/.antigravityignore\")\" = collected ]"
check "#91 Antigravity drive-letter folderUri discovers project" "[ \"\$(status_of \"$ROOT/srv/agywin/.antigravityignore\")\" = collected ]"
# 2026-10-14 catalog match that is an excluded file (#107)
check "#107 excluded file match gets a skipped_excluded row with its size" "row \"$ALICE/.local/bin/.muse-update.107.exe\" | grep -q '^{\"user\":\"alice\",\"home\":\"$ALICE\",\"agent\":\"muse-code\",\"path\":\"[^\"]*\",\"archive_path\":\"\",\"type\":\"file\",\"size\":5,.*\"sha256\":\"\",\"secret\":false,\"status\":\"skipped_excluded\",\"target\":\"\",\"error\":\"\"}\$'"
check "#107 excluded file match not archived" "! tar -tzf \"\$ARCHIVE\" | grep -q 'muse-update.107.exe'"
# 2026-10-14 Antigravity on Windows (#83)
check "#83 Antigravity Windows staging file collected as antigravity" "row \"$ALICE_LOCAL_APPDATA/antigravity/staging/s1\" | grep -q '\"agent\":\"antigravity\".*\"status\":\"collected\"'"
check "#83 Antigravity installer and program directory not collected" "! grep -q 'antigravity-updater/installer.exe\"' \"\$MANIFEST\" && ! grep -q 'Programs/antigravity/Antigravity.exe\"' \"\$MANIFEST\""
# 2026-10-15 ChatGPT desktop as the OpenAI.Codex package (#113)
check "#113 ChatGPT profile Local Storage collected as chatgpt-desktop" "row \"$CHATGPT_PROFILE/Default/Local Storage/leveldb/000003.log\" | grep -q '\"agent\":\"chatgpt-desktop\".*\"secret\":false,\"status\":\"collected\"'"
check "#113 ChatGPT Local State flagged secret" "row \"$CHATGPT_PROFILE/Local State\" | grep -q '\"agent\":\"chatgpt-desktop\".*\"secret\":true,\"status\":\"collected\"'"
check "#113 ChatGPT profile and partition cookies flagged secret" "row \"$CHATGPT_PROFILE/Default/Network/Cookies\" | grep -q '\"secret\":true,\"status\":\"collected\"' && row \"$CHATGPT_PARTITION/Network/Cookies\" | grep -q '\"secret\":true,\"status\":\"collected\"'"
check "#113 ChatGPT Login Data flagged secret" "row \"$CHATGPT_PROFILE/Default/Login Data\" | grep -q '\"secret\":true,\"status\":\"collected\"'"
check "#113 ChatGPT profile and partition Cache excluded" "[ \"\$(status_of \"$CHATGPT_PROFILE/Default/Cache\")\" = skipped_excluded ] && [ \"\$(status_of \"$CHATGPT_PARTITION/Cache\")\" = skipped_excluded ] && ! grep -q 'Cache/data_0\"' \"\$MANIFEST\""
check "#113 ChatGPT component cache beside the profile not in the manifest" "! grep -q 'web/Codex/component_crx_cache' \"\$MANIFEST\""
check "#113 ChatGPT app log collected as chatgpt-desktop" "row \"$CHATGPT_PACKAGE/LocalCache/Local/Codex/Logs/2026/10/15/codex-desktop-x.log\" | grep -q '\"agent\":\"chatgpt-desktop\".*\"status\":\"collected\"'"
check "#113 ChatGPT MSIX settings hive collected" "row \"$CHATGPT_PACKAGE/Settings/settings.dat\" | grep -q '\"agent\":\"chatgpt-desktop\".*\"status\":\"collected\"'"
check "#116 Codex plugin app-server binaries excluded" "[ \"\$(status_of \"$BOB/.codex/plugins/.plugin-appserver\")\" = skipped_excluded ] && ! grep -q '.plugin-appserver/codex.exe\"' \"\$MANIFEST\""
check "#115 Codex desktop global state flagged secret" "row \"$BOB/.codex/.codex-global-state.json\" | grep -q '\"agent\":\"codex-cli\".*\"secret\":true,\"status\":\"collected\"'"
# 2026-10-15 ChatGPT desktop on macOS (#121)
check "#121 ChatGPT macOS Local Storage collected as chatgpt-desktop" "row \"$CHATGPT_MAC/Default/Local Storage/leveldb/000003.log\" | grep -q '\"agent\":\"chatgpt-desktop\".*\"secret\":false,\"status\":\"collected\"'"
check "#121 ChatGPT macOS profile and partition cookies flagged secret" "row \"$CHATGPT_MAC/Default/Cookies\" | grep -q '\"secret\":true,\"status\":\"collected\"' && row \"$CHATGPT_MAC_PARTITION/Cookies\" | grep -q '\"secret\":true,\"status\":\"collected\"'"
check "#121 ChatGPT macOS Login Data flagged secret" "row \"$CHATGPT_MAC/Default/Login Data\" | grep -q '\"secret\":true,\"status\":\"collected\"'"
check "#121 ChatGPT macOS profile and partition Cache excluded" "[ \"\$(status_of \"$CHATGPT_MAC/Default/Cache\")\" = skipped_excluded ] && [ \"\$(status_of \"$CHATGPT_MAC_PARTITION/Cache\")\" = skipped_excluded ] && ! grep -q 'Codex/Default/Cache/data_0\"' \"\$MANIFEST\" && ! grep -q 'codex-browser-app/Cache/data_0\"' \"\$MANIFEST\""
check "#121 ChatGPT macOS MCP server script excluded, manifest collected" "[ \"\$(status_of \"$CHATGPT_MAC/executor-plugins/codex-app-tools/server.mjs\")\" = skipped_excluded ] && row \"$CHATGPT_MAC/executor-plugins/codex-app-tools/.mcp.json\" | grep -q '\"agent\":\"chatgpt-desktop\".*\"status\":\"collected\"'"
check "#121 ChatGPT macOS component cache not in the manifest" "! grep -q 'Application Support/Codex/component_crx_cache' \"\$MANIFEST\""
check "#121 ChatGPT macOS app log collected as chatgpt-desktop" "row \"$BOB/Library/Logs/com.openai.codex/2026/10/15/codex-desktop-x.log\" | grep -q '\"agent\":\"chatgpt-desktop\".*\"status\":\"collected\"'"
check "#121 ChatGPT macOS preferences collected" "row \"$BOB/Library/Preferences/com.openai.codex.plist\" | grep -q '\"agent\":\"chatgpt-desktop\".*\"status\":\"collected\"'"
check "#121 ChatGPT macOS binary cookies flagged secret" "row \"$BOB/Library/HTTPStorages/com.openai.codex.binarycookies\" | grep -q '\"agent\":\"chatgpt-desktop\".*\"secret\":true,\"status\":\"collected\"'"
check "#121 ChatGPT computer-use group container collected" "row \"$CUA_GROUP_CONTAINER/IPC/computeruse.sock.lock\" | grep -q '\"agent\":\"chatgpt-desktop\".*\"status\":\"collected\"'"
check "#121 Codex computer-use app bundle excluded, config collected" "[ \"\$(status_of \"$COMPUTER_USE/Codex Computer Use.app\")\" = skipped_excluded ] && ! grep -q 'Computer Use.app/Contents/Info.plist\"' \"\$MANIFEST\" && row \"$COMPUTER_USE/config.json\" | grep -q '\"agent\":\"codex-cli\".*\"status\":\"collected\"'"
# 2026-10-15 special files (#120)
# shellcheck disable=SC2034 # read by the eval'd check strings
SPECIAL_COUNT=$((1 + HAVE_SOCKET))
check "#120 FIFO recorded as skipped_special, not copied" "row \"$SPECIAL_DIR/ipc/ipc.fifo\" | grep -q '\"archive_path\":\"\",\"type\":\"fifo\".*\"sha256\":\"\",\"secret\":false,\"status\":\"skipped_special\",\"target\":\"\",\"error\":\"\"'"
if [ "$HAVE_SOCKET" = 1 ]; then
  check "#120 socket recorded as skipped_special, not copied" "row \"$SPECIAL_DIR/app-server-daemon/daemon-updater.sock\" | grep -q '\"archive_path\":\"\",\"type\":\"socket\".*\"sha256\":\"\",\"secret\":false,\"status\":\"skipped_special\",\"target\":\"\",\"error\":\"\"'"
fi
check "#120 special files not in the archive" "! tar -tzf \"\$ARCHIVE\" | grep -q 'ipc.fifo\\|daemon-updater.sock'"
check "#120 summary counts skipped_special" "grep -q '\"skipped_special\": $SPECIAL_COUNT,' \"\$SUMMARY\""
check "#120 stdout skipped line counts special files" "grep -q '^skipped:    [0-9]* excluded, [0-9]* too large, [0-9]* secret, $SPECIAL_COUNT special\$' \"$OUT/default.stdout\""
check "#120 no symlink row targets its own path" "! grep '\"status\":\"symlink\"' \"\$MANIFEST\" | sed 's/.*\"path\":\"\([^\"]*\)\".*\"target\":\"\([^\"]*\)\".*/\1|\2/' | grep -q '^\\(.*\\)|\\1\$'"

# 2026-10-16 GitHub Copilot Desktop (#147)
check "#147 Copilot Desktop data.db collected as copilot-desktop" "row \"$COPILOT/data.db\" | grep -q '\"agent\":\"copilot-desktop\".*\"secret\":false,\"status\":\"collected\"' && row \"$COPILOT/data.db-wal\" | grep -q '\"agent\":\"copilot-desktop\".*\"status\":\"collected\"'"
check "#147 Copilot Desktop github-app log collected as copilot-desktop" "row \"$COPILOT/logs/github-app.1.log\" | grep -q '\"agent\":\"copilot-desktop\".*\"status\":\"collected\"'"
check "#147 Copilot Desktop localstorage collected as copilot-desktop" "row \"$COPILOT_DESKTOP_DATA/localstorage/tauri_localhost_0.localstorage\" | grep -q '\"agent\":\"copilot-desktop\".*\"status\":\"collected\"'"
check "#147 Foundry Local log collected as copilot-desktop" "row \"$ROOT/home/alice/.github-copilot-cli/logs/foundry.core20261016.log\" | grep -q '\"agent\":\"copilot-desktop\".*\"status\":\"collected\"'"
check "#147 Copilot Desktop session workspace.yaml stays copilot-cli" "row \"$COPILOT/session-state/s2/workspace.yaml\" | grep -q '\"agent\":\"copilot-cli\".*\"status\":\"collected\"'"
check "#147 Copilot Desktop ws.token flagged secret" "row \"$COPILOT/run/ws.token\" | grep -q '\"agent\":\"copilot-desktop\".*\"secret\":true,\"status\":\"collected\"'"
check "#147 Copilot Desktop checkpoint ref and HEAD collected as copilot-desktop" "row \"$COPILOT_CHECKPOINT_REF/e0000000-0000-4000-8000-000000000020\" | grep -q '\"agent\":\"copilot-desktop\".*\"status\":\"collected\"' && row \"$COPILOT_CLONE_GIT/HEAD\" | grep -q '\"agent\":\"copilot-desktop\".*\"status\":\"collected\"'"
check "#147 Copilot Desktop repos working tree, worktrees, objects and index excluded" "[ \"\$(status_of \"$COPILOT/repos\")\" = skipped_excluded ] && [ \"\$(status_of \"$COPILOT_CLONE_GIT/objects\")\" = skipped_excluded ] && [ \"\$(status_of \"$COPILOT_CLONE_GIT/index\")\" = skipped_excluded ] && ! grep -q 'repos/proj/README.md\"' \"\$MANIFEST\" && ! grep -q 'alice-x/AGENTS.md\"' \"\$MANIFEST\" && ! grep -q 'cdef0123\"' \"\$MANIFEST\""
check "#147 Copilot Desktop app-skills excluded" "[ \"\$(status_of \"$COPILOT_DESKTOP_DATA/app-skills\")\" = skipped_excluded ] && ! grep -q 'app-skills/af/SKILL.md\"' \"\$MANIFEST\""
check "#147 Copilot Desktop chat directory and worktree not projects" "! grep -q 'Project .*/\.copilot/' \"$OUT\"/default/*.log && ! grep -q '\"home\":\"$COPILOT/' \"\$MANIFEST\" && ! grep -q '\.copilot/chats' \"\$SUMMARY\" && ! grep -q 'copilot-worktrees' \"\$SUMMARY\""
check "#147 Copilot Desktop chat directory file collected once as copilot-desktop" "[ \"\$(grep -c 'x-5d404e14/.keep\"' \"\$MANIFEST\")\" = 1 ] && row \"$COPILOT/chats/2026-10-16/x-5d404e14/.keep\" | grep -q '\"agent\":\"copilot-desktop\"'"
# 2026-10-17 Copilot Desktop Windows round (#147)
check "#147 WebView2 Local Storage collected as copilot-desktop" "row \"$COPILOT_WEBVIEW/Default/Local Storage/leveldb/000003.log\" | grep -q '\"agent\":\"copilot-desktop\".*\"status\":\"collected\"'"
check "#147 WebView2 cookies and Local State flagged secret" "row \"$COPILOT_WEBVIEW/Default/Network/Cookies\" | grep -q '\"agent\":\"copilot-desktop\".*\"secret\":true,\"status\":\"collected\"' && row \"$COPILOT_WEBVIEW/Local State\" | grep -q '\"agent\":\"copilot-desktop\".*\"secret\":true,\"status\":\"collected\"'"
check "#147 WebView2 Cache and component_crx_cache excluded" "[ \"\$(status_of \"$COPILOT_WEBVIEW/Default/Cache\")\" = skipped_excluded ] && [ \"\$(status_of \"$COPILOT_WEBVIEW/component_crx_cache\")\" = skipped_excluded ] && ! grep -q 'Cache_Data/x\"' \"\$MANIFEST\" && ! grep -q 'component_crx_cache/x\"' \"\$MANIFEST\""
check "#147 Copilot Desktop chat project collected less node_modules" "row \"$COPILOT_CHAT_PROJECT/package.json\" | grep -q '\"agent\":\"copilot-desktop\".*\"status\":\"collected\"' && [ \"\$(status_of \"$COPILOT_CHAT_PROJECT/node_modules\")\" = skipped_excluded ] && ! grep -q 'node_modules/m/index.js\"' \"\$MANIFEST\""
check "#131 Copilot CLI marketplaces excluded" "[ \"\$(status_of \"$COPILOT_MARKETPLACES\")\" = skipped_excluded ] && ! grep -q 'marketplaces/m/.git/HEAD\"' \"\$MANIFEST\""
check "#147 Copilot pool session cwd not a project" "[ -z \"\$(status_of \"$ROOT/srv/coppool/AGENTS.md\")\" ] && ! grep -q 'coppool' \"\$SUMMARY\""

# 2026-10-17 session sync round (#153)
SECRET_COLLECTED='"secret":true,"status":"collected"'
check "#153 nanobot remote pairing identity and exchange keys flagged secret" "row \"$NANOBOT_PAIRING/identity\" | grep -q '\"agent\":\"nanobot\".*$SECRET_COLLECTED' && row \"$NANOBOT_PAIRING/exchange\" | grep -q '$SECRET_COLLECTED' && row \"$NANOBOT_PAIRING/request\" | grep -q '\"secret\":false,\"status\":\"collected\"'"
check "#153 Agent Zero plugin config.json flagged secret" "row \"$ALICE/agent-zero/main/usr/$AGENT_ZERO_PLUGIN_CONFIG\" | grep -q '\"agent\":\"agent-zero\".*$SECRET_COLLECTED' && row \"$ALICE/Desktop/agent-zero/usr/$AGENT_ZERO_PLUGIN_CONFIG\" | grep -q '$SECRET_COLLECTED'"
check "#153 Agent Zero volume plugin config.json flagged secret" "row \"$VOLUMES/a0_usr/_data/$AGENT_ZERO_PLUGIN_CONFIG\" | grep -q '\"user\":\"docker\".*$SECRET_COLLECTED' && row \"$VOLUMES/a0_usr/_data/usr/$AGENT_ZERO_PLUGIN_CONFIG\" | grep -q '$SECRET_COLLECTED'"
check "#153 Cline cached remote config flagged secret" "row \"$CLINE_REMOTE_CACHE/remote_config_org1.json\" | grep -q '\"agent\":\"cline\".*$SECRET_COLLECTED'"
check "#153 Cline globalStorage cache collected" "row \"$CLINE_REMOTE_CACHE/sync-queue.json\" | grep -q '\"agent\":\"cline\".*\"secret\":false,\"status\":\"collected\"' && [ -z \"\$(status_of \"$CLINE_REMOTE_CACHE\")\" ]"
check "#153 Cline webhook_config.json flagged secret" "row \"$ALICE/Documents/Cline/webhook_config.json\" | grep -q '\"agent\":\"cline\".*$SECRET_COLLECTED'"
check "#153 Gemini CLI bug-memory heap snapshot flagged secret" "row \"$ALICE/.gemini/tmp/gem/bug-memory-1792000000000.heapsnapshot\" | grep -q '\"agent\":\"gemini-cli\".*$SECRET_COLLECTED'"
check "#153 little-coder legacy config.json flagged secret" "row \"$ALICE/.little-coder/config.json\" | grep -q '\"agent\":\"little-coder\".*$SECRET_COLLECTED'"
check "#153 Qwen Code channel credentials flagged secret" "row \"$ALICE/.qwen/channels/qqbot-credentials.json\" | grep -q '\"agent\":\"qwen-code\".*$SECRET_COLLECTED' && row \"$ALICE/.qwen/channels/weixin/account.json\" | grep -q '$SECRET_COLLECTED'"
check "#153 Qwen Code channel routing files not flagged" "row \"$ALICE/.qwen/channels/sessions.json\" | grep -q '\"secret\":false,\"status\":\"collected\"' && row \"$ALICE/.qwen/channels/daemon/0123abcd/routes.json\" | grep -q '\"secret\":false,\"status\":\"collected\"'"
check "#153 Kilo log directory collected" "row \"$ALICE/.local/share/kilo/log/2026-10-17T120000.log\" | grep -q '\"agent\":\"kilo-code\".*\"status\":\"collected\"'"
# 2026-10-18 Codex shell snapshots (#160)
check "#160 Codex shell snapshot flagged secret" "row \"$CODEX_SHELL_SNAPSHOT\" | grep -q '\"agent\":\"codex-cli\".*$SECRET_COLLECTED'"
# 2026-10-19 Codex projectless output directories (#149)
check "#149 Codex projectless output collected" "row \"$CODEX_OUTPUT_DIR/outputs/report.md\" | grep -q '\"home\":\"$BOB\",\"agent\":\"codex-cli\".*\"status\":\"collected\"'"
check "#149 Codex projectless dir not a project" "! grep '^  \"projects\"' \"\$SUMMARY\" | grep -q 'Documents/Codex' && ! grep -q '\"home\":\"$BOB/Documents/Codex' \"\$MANIFEST\" && ! grep -q 'Project .*/Documents/Codex' \"$OUT\"/default/*.log"
check "#149 Codex rollout cwd still a project" "grep '^  \"projects\"' \"\$SUMMARY\" | grep -qF '\"$BOB/Documents/app\"'"

# 2026-10-20 Cursor IDE round on macOS (#6)
check "#6 Cursor AgentStores sync index collected" "row \"$CURSOR_MAC/AgentStores/cursor_agent_stores/u1/.sync/index.sqlite\" | grep -q '\"agent\":\"cursor\".*\"status\":\"collected\"'"
check "#6 Cursor AgentStores pin collected" "row \"$CURSOR_MAC/AgentStores/cursor_agent_stores/.control/u1/pins/1-x.json\" | grep -q '\"agent\":\"cursor\".*\"status\":\"collected\"'"
check "#6 Cursor update-supervisor events collected" "row \"$CURSOR_MAC/update-supervisor/33333333-3333-4333-8333-333333333333/events.jsonl\" | grep -q '\"agent\":\"cursor\".*\"status\":\"collected\"'"
check "#6 Cursor update-supervisor binary excluded" "[ \"\$(status_of \"$CURSOR_MAC/update-supervisor/33333333-3333-4333-8333-333333333333/cursor-update-supervisor\")\" = skipped_excluded ]"
check "#6 Cursor embedded agent CLI excluded" "[ \"\$(status_of \"$CURSOR_MAC_WORKER/agent-cli\")\" = skipped_excluded ] && ! grep -qF 'agent-cli/.local/share/x' \"\$MANIFEST\""
check "#6 Cursor agent worker log collected" "row \"$CURSOR_MAC_WORKER/cursor-agent-worker-1.log\" | grep -q '\"agent\":\"cursor\".*\"status\":\"collected\"'"
check "#6 Cursor canvases node_modules excluded" "[ \"\$(status_of \"$BOB/.cursor/projects/p/canvases/node_modules\")\" = skipped_excluded ] && ! grep -qF 'node_modules/x.d.ts' \"\$MANIFEST\""
check "#6 Cursor agent transcript collected" "row \"$BOB/.cursor/projects/p/agent-transcripts/u/u.jsonl\" | grep -q '\"agent\":\"cursor\".*\"status\":\"collected\"'"
check "#6 Cursor statsig cache excluded" "[ \"\$(status_of \"$BOB/.cursor/statsig-cache.json\")\" = skipped_excluded ]"

# 2026-10-20 Cursor IDE round on Ubuntu (#236)
check "#236 Cursor Linux agent-stores collected" "row \"$CURSOR_LINUX_STORES/u1/.sync/index.sqlite\" | grep -q '\"agent\":\"cursor\".*\"status\":\"collected\"' && row \"$CURSOR_LINUX_STORES/.control/u1/pins/1-x.json\" | grep -q '\"agent\":\"cursor\".*\"status\":\"collected\"'"

# 2026-10-21 Cursor IDE round on Windows (#236)
check "#236 Cursor Windows agent-stores collected" "row \"$CURSOR_WINDOWS_STORES/u1/.sync/index.sqlite\" | grep -q '\"agent\":\"cursor\".*\"status\":\"collected\"' && row \"$CURSOR_WINDOWS_STORES/.control/u1/pins/1-x.json\" | grep -q '\"agent\":\"cursor\".*\"status\":\"collected\"'"
check "#236 Cursor Windows embedded agent CLI excluded" "[ \"\$(status_of \"$CURSOR_WINDOWS_WORKER/agent-cli\")\" = skipped_excluded ] && ! grep -qF 'anysphere.cursor-agent-worker/agent-cli/x' \"\$MANIFEST\""
check "#236 Cursor Windows agent worker log collected" "row \"$CURSOR_WINDOWS_WORKER/cursor-agent-worker-1.log\" | grep -q '\"agent\":\"cursor\".*\"status\":\"collected\"'"
check "#236 Cursor plugin cache excluded" "[ \"\$(status_of \"$CURSOR_PLUGINS/cache\")\" = skipped_excluded ] && ! grep -qF 'cache/cursor-public/x/0123abcd/mcp.json' \"\$MANIFEST\""
check "#236 Cursor local plugins collected" "row \"$CURSOR_PLUGINS/local/x.json\" | grep -q '\"agent\":\"cursor\".*\"status\":\"collected\"'"
check "#236 Cursor Windows inno updater excluded" "[ \"\$(status_of \"$CURSOR_UPDATE_SUPERVISOR/inno_updater.exe\")\" = skipped_excluded ]"
check "#236 Cursor Windows update-supervisor state collected" "row \"$CURSOR_UPDATE_SUPERVISOR/app-ready.json\" | grep -q '\"agent\":\"cursor\".*\"status\":\"collected\"' && row \"$CURSOR_UPDATE_SUPERVISOR/state.json\" | grep -q '\"agent\":\"cursor\".*\"status\":\"collected\"'"
# 2026-10-21 OpenHands Agent Canvas credentials and workspaces (#218)
OPENHANDS_SECRET='"agent":"openhands".*"secret":true,"status":"collected"'
check "#218 OpenHands Agent Canvas mars-credentials.json flagged secret" "row \"$CANVAS_CREDENTIALS_LINUX\" | grep -q '$OPENHANDS_SECRET' && row \"$CANVAS_CREDENTIALS_MAC\" | grep -q '$OPENHANDS_SECRET' && row \"$CANVAS_CREDENTIALS_WINDOWS\" | grep -q '$OPENHANDS_SECRET'"
check "#218 OpenHands automation workspaces excluded" "[ \"\$(status_of \"$ALICE/.openhands/workspaces\")\" = skipped_excluded ] && ! grep -qF '.openhands/workspaces/run1/f' \"\$MANIFEST\""
check "#218 OpenHands workspaces.json beside the excluded workspaces collected" "[ \"\$(status_of \"$ALICE/.openhands/workspaces.json\")\" = collected ]"

# 2026-10-21 Agent Zero source review (#217)
check "#217 Agent Zero WhatsApp session flagged secret" "row \"$ALICE/agent-zero/main/$WHATSAPP_CREDS\" | grep -q '\"agent\":\"agent-zero\".*$SECRET_COLLECTED' && row \"$ALICE/Desktop/agent-zero/$WHATSAPP_CREDS\" | grep -q '$SECRET_COLLECTED' && row \"$VOLUMES/a0_usr/_data/$WHATSAPP_CREDS\" | grep -q '\"user\":\"docker\".*$SECRET_COLLECTED'"
check "#217 Agent Zero ACP workspace and extra folder discovered" "[ \"\$(status_of \"$ROOT/srv/a0acp/AGENTS.md\")\" = collected ] && [ \"\$(status_of \"$ROOT/srv/a0extra/AGENTS.md\")\" = collected ]"
check "#217 Agent Zero connector screen capture collected" "row \"$AGENT_ZERO_CAPTURE/0000.png\" | grep -q '\"agent\":\"agent-zero\".*\"status\":\"collected\"'"

# 2026-10-21 framework catalog gaps (#223, #224, #225)
check "#223 CrewAI project .crewai collected" "row \"$ROOT/srv/proj/.crewai/last_run.json\" | grep -q '\"home\":\"$ROOT/srv/proj\",\"agent\":\"crewai\".*\"status\":\"collected\"'"
check "#224 difyctl login-pending.yml flagged secret" "row \"$ALICE/.config/difyctl/login-pending.yml\" | grep -q '\"secret\":true,\"status\":\"collected\"' && row \"$ALICE/AppData/Roaming/difyctl/login-pending.yml\" | grep -q '\"secret\":true,\"status\":\"collected\"' && row \"$ALICE/AppData/Local/difyctl/login-pending.yml\" | grep -q '\"secret\":true,\"status\":\"collected\"'"
check "#225 CLAI 2.0 state directory collected" "row \"$CLAI_STATE/orchestrator.json\" | grep -q '\"agent\":\"pydantic-clai\".*\"status\":\"collected\"' && row \"$CLAI_STATE/workspaces/registry/ws1/workspace.json\" | grep -q '\"agent\":\"pydantic-clai\".*\"status\":\"collected\"' && row \"$CLAI_STATE/workspaces/registry/ws1/session.log\" | grep -q '\"agent\":\"pydantic-clai\".*\"status\":\"collected\"'"
check "#225 CLAI 2.0 workspace token flagged secret" "row \"$CLAI_STATE/workspaces/registry/ws1/token\" | grep -q '\"agent\":\"pydantic-clai\".*\"secret\":true,\"status\":\"collected\"'"
check "#225 CLAI 2.0 workspace clones excluded" "[ \"\$(status_of \"$CLAI_STATE/workspaces/clones\")\" = skipped_excluded ] && ! grep -qF 'repo-0123456789/.git/HEAD' \"\$MANIFEST\""

# 2026-10-21 Codex worktree cwds and archived threads (#222)
check "#222 Codex archived rollout cwd discovers project" "[ \"\$(status_of \"$ROOT/srv/codexarch/AGENTS.md\")\" = collected ] && grep '^  \"projects\"' \"\$SUMMARY\" | grep -qF '\"$ROOT/srv/codexarch\"'"
check "#222 Codex worktree cwd not a project" "! grep '^  \"projects\"' \"\$SUMMARY\" | grep -q '\.codex/worktrees' && [ \"\$(status_of \"$BOB/.codex/worktrees\")\" = skipped_excluded ] && ! grep -qF 'worktrees/a1b2/repo/AGENTS.md' \"\$MANIFEST\""
check "#222 Open Interpreter worktree cwd not a project" "! grep '^  \"projects\"' \"\$SUMMARY\" | grep -q '\.openinterpreter/worktrees' && ! grep -qF 'worktrees/c3d4/repo/AGENTS.md' \"\$MANIFEST\""

# 2026-10-21 Cody JetBrains plugin files (#214)
check "#214 Cody JetBrains options files collected" "row \"$JETBRAINS_OPTIONS/cody.xml\" | grep -q '\"agent\":\"cody\".*\"secret\":true,\"status\":\"collected\"' && row \"$JETBRAINS_OPTIONS/cody_application_settings.xml\" | grep -q '\"agent\":\"cody\".*\"status\":\"collected\"'"
check "#214 Cody cody_accounts.xml flagged secret" "row \"$JETBRAINS_OPTIONS/cody_accounts.xml\" | grep -q '\"agent\":\"cody\".*\"secret\":true,\"status\":\"collected\"'"
check "#214 Cody cody_accounts_settings.xml not flagged" "row \"$JETBRAINS_OPTIONS/cody_accounts_settings.xml\" | grep -q '\"agent\":\"cody\".*\"secret\":false,\"status\":\"collected\"'"
check "#214 Cody home .sourcegraph collected" "row \"$ALICE/.sourcegraph/cody_settings.json\" | grep -q '\"agent\":\"cody\".*\"secret\":true,\"status\":\"collected\"'"
check "#214 Cody project .idea files collected" "row \"$ROOT/srv/proj/.idea/cody_history.xml\" | grep -q '\"agent\":\"cody\".*\"status\":\"collected\"' && row \"$ROOT/srv/proj/.idea/.sourcegraph/cody_settings.json\" | grep -q '\"agent\":\"cody\".*\"secret\":true,\"status\":\"collected\"'"

# 2026-10-21 agent worktrees inside a project (#216)
check "#216 Gemini CLI and Qwen Code project worktrees excluded" "[ \"\$(status_of \"$GEMINI_PROJECT/.gemini/worktrees\")\" = skipped_excluded ] && [ \"\$(status_of \"$GEMINI_PROJECT/.qwen/worktrees\")\" = skipped_excluded ] && ! grep -qF 'gem/.gemini/worktrees/w1' \"\$MANIFEST\" && ! grep -qF 'gem/.qwen/worktrees/w1' \"\$MANIFEST\""
check "#216 Gemini CLI project settings beside the worktrees collected" "row \"$GEMINI_PROJECT/.gemini/settings.json\" | grep -q '\"agent\":\"gemini-cli\".*\"status\":\"collected\"'"

# 2026-10-21 Hermes quick snapshots and desktop connections (#123, #208)
check "#123 Hermes quick snapshot state.db collected" "row \"$HERMES_SNAPSHOT/state.db\" | grep -q '\"agent\":\"hermes\".*\"secret\":false,\"status\":\"collected\"' && row \"$HERMES_SNAPSHOT/manifest.json\" | grep -q '\"secret\":false,\"status\":\"collected\"' && row \"$HERMES_SNAPSHOT_WINDOWS/state.db\" | grep -q '\"agent\":\"hermes\".*\"secret\":false,\"status\":\"collected\"' && [ -z \"\$(status_of \"$ALICE/.hermes/state-snapshots\")\" ]"
check "#123 Hermes quick snapshot credentials flagged secret" "row \"$HERMES_SNAPSHOT/.env\" | grep -q '$SECRET_COLLECTED' && row \"$HERMES_SNAPSHOT/auth.json\" | grep -q '$SECRET_COLLECTED' && row \"$HERMES_SNAPSHOT/pairing/telegram-approved.json\" | grep -q '$SECRET_COLLECTED'"
check "#208 Hermes desktop connection files flagged secret" "row \"$ALICE/.config/Hermes/connections.json\" | grep -q '\"agent\":\"hermes\".*$SECRET_COLLECTED' && row \"$ALICE/.config/Hermes/connection.json\" | grep -q '$SECRET_COLLECTED'"

# 2026-10-21 nanobot workspace cron and triggers (#220)
check "#220 nanobot workspace cron jobs and runs collected" "row \"$NANOBOT_WORKSPACE/cron/jobs.json\" | grep -q '\"agent\":\"nanobot\".*\"status\":\"collected\"' && row \"$NANOBOT_WORKSPACE/cron/runs/j1_1_ab.json\" | grep -q '\"agent\":\"nanobot\".*\"status\":\"collected\"'"
check "#220 nanobot workspace triggers and runs collected" "row \"$NANOBOT_WORKSPACE/triggers/triggers.json\" | grep -q '\"agent\":\"nanobot\".*\"status\":\"collected\"' && row \"$NANOBOT_WORKSPACE/triggers/runs/d1.json\" | grep -q '\"agent\":\"nanobot\".*\"status\":\"collected\"'"
check "#220 nanobot trigger queue not collected" "! grep -qF 'nanows/triggers/inbox' \"\$MANIFEST\""

# 2026-10-21 OpenClaw source review (#219)
check "#219 OpenClaw exec-approvals.json flagged secret" "row \"$OPENCLAW/exec-approvals.json\" | grep -q '\"agent\":\"openclaw\".*$SECRET_COLLECTED'"
check "#219 OpenClaw auth-profile secrets directory flagged secret" "row \"$ALICE/.openclaw-auth-profile-secrets/gateway.env\" | grep -q '\"agent\":\"openclaw\".*$SECRET_COLLECTED'"
check "#219 OpenClaw Gateway clone excluded" "[ \"\$(status_of \"$OPENCLAW/projects/$OPENCLAW_CLONE_ID\")\" = skipped_excluded ] && ! grep -qF '$OPENCLAW_CLONE_ID/repo/README.md' \"\$MANIFEST\""
check "#219 OpenClaw projects entry that is no clone collected" "row \"$OPENCLAW/projects/notes/todo.md\" | grep -q '\"agent\":\"openclaw\".*\"status\":\"collected\"'"
# 2026-10-30 openclaw rebuild (#326)
check "openclaw JSON5 agent cwd discovers project" "[ \"\$(status_of \"$ROOT/srv/clawcwd/TOOLS.md\")\" = collected ]"
check "openclaw legacy .clawdbot/openclaw.json discovers project" "[ \"\$(status_of \"$ROOT/srv/clawold/BOOTSTRAP.md\")\" = collected ]"
check "openclaw Moltbot .moltbot/moltbot.json discovers project" "[ \"\$(status_of \"$ROOT/srv/moltws/.openclaw/worktree-setup.sh\")\" = collected ]"
check "openclaw Moltbot .clawdbot/moltbot.json discovers project" "[ \"\$(status_of \"$ROOT/srv/moltold/SOUL.md\")\" = collected ]"

# 2026-10-21 OpenCode and Kilo Code source review (#213, #221)
check "#213 OpenCode service.json flagged secret" "row \"$ALICE/.config/opencode/service.json\" | grep -q '\"agent\":\"opencode\".*$SECRET_COLLECTED' && row \"$ALICE/.local/state/opencode/service-local.json\" | grep -q '\"agent\":\"opencode\".*$SECRET_COLLECTED'"
check "#213 OpenCode desktop data collected" "row \"$OPENCODE_DESKTOP/drafts.sqlite\" | grep -q '\"agent\":\"opencode\".*\"status\":\"collected\"' && row \"$ALICE/.config/ai.opencode.desktop.beta/drafts.sqlite\" | grep -q '\"agent\":\"opencode\".*\"status\":\"collected\"' && row \"$BOB/AppData/Roaming/ai.opencode.desktop/drafts.sqlite\" | grep -q '\"agent\":\"opencode\".*\"status\":\"collected\"'"
check "#213 OpenCode desktop compile-cache excluded" "[ \"\$(status_of \"$OPENCODE_DESKTOP/compile-cache\")\" = skipped_excluded ] && ! grep -qF 'compile-cache/x' \"\$MANIFEST\""
check "#213 OpenCode Windows temp root collected" "row \"$BOB/AppData/Local/Temp/opencode/notes.txt\" | grep -q '\"agent\":\"opencode\".*\"status\":\"collected\"'"
check "#221 Kilo sandbox policy and preference collected" "row \"$KILO_SANDBOX_POLICY/bb.json\" | grep -q '\"agent\":\"kilo-code\".*\"status\":\"collected\"' && row \"$ALICE/.local/state/kilo-sandbox-preference/bb.json\" | grep -q '\"agent\":\"kilo-code\".*\"status\":\"collected\"'"

# 2026-10-21 Qwen Code session registry (#230)
check "#230 Qwen Code session registry flagged secret" "row \"$ALICE/.qwen/sessions/4242.json\" | grep -q '\"agent\":\"qwen-code\".*\"secret\":true,\"status\":\"collected\"' && row \"$ALICE/.qwen/sessions/4243-0123abcd.json\" | grep -q '\"secret\":true,\"status\":\"collected\"'"

# 2026-10-21 Roo Code CLI cloud credentials (#159)
check "#159 roo cli-credentials.json flagged secret" "row \"$ALICE/.roo/cli-credentials.json\" | grep -q '\"agent\":\"roo-code\".*\"secret\":true,\"status\":\"collected\"'"

# 2026-10-21 Twinny workspace storage (#215)
check "#215 twinny workspaceStorage agent-changes attributed to twinny" "row \"$TWINNY_WORKSPACE/rjmacarthy.twinny/agent-changes/c1.json\" | grep -q '\"agent\":\"twinny\".*\"status\":\"collected\"'"
check "#215 twinny workspaceStorage neighbour stays vscode" "row \"$TWINNY_WORKSPACE/workspace.json\" | grep -q '\"agent\":\"vscode\".*\"status\":\"collected\"'"
check "#215 twinny-server forge repos.json flagged secret" "row \"$ALICE/.twinny/server/plugins/github/repos.json\" | grep -q '\"agent\":\"twinny\".*\"secret\":true,\"status\":\"collected\"'"

# 2026-10-21 project git metadata (#253)
# git_files_collected FILE...: each file under the project's .git has a collected row
# with agent project and the project as its home. no_git_rows PREFIX...: no
# manifest row names a path under the project's .git that starts with it.
# shellcheck disable=SC2317,SC2329 # called from the eval'd check strings
git_files_collected() {
  for _git_path in "$@"; do
    row "$PROJECT_GIT/$_git_path" | grep -q "\"home\":\"$ROOT/srv/proj\",\"agent\":\"project\".*\"status\":\"collected\"" || return 1
  done
}
# shellcheck disable=SC2317,SC2329
no_git_rows() {
  for _git_path in "$@"; do
    if grep -qF "\"path\":\"$PROJECT_GIT/$_git_path" "$MANIFEST"; then return 1; fi
  done
}
check "#253 project git HEAD, config, FETCH_HEAD and packed-refs collected as project" "git_files_collected HEAD config FETCH_HEAD packed-refs"
check "#253 project git refs and logs collected as project" "git_files_collected refs/heads/main refs/remotes/origin/main logs/HEAD logs/refs/remotes/origin/main"
check "#253 project git worktrees collected as project" "git_files_collected worktrees/wt1/HEAD worktrees/wt1/gitdir"
check "#253 project git linked worktree index excluded" "[ \"\$(status_of \"$PROJECT_GIT/worktrees/wt1/index\")\" = skipped_excluded ]"
check "#253 project git linked worktree submodules excluded" "[ \"\$(status_of \"$PROJECT_GIT/worktrees/wt1/modules\")\" = skipped_excluded ] && no_git_rows worktrees/wt1/modules/sub"
check "#253 project git linked worktree index.lock and sharedindex excluded" "[ \"\$(status_of \"$PROJECT_GIT/worktrees/wt1/index.lock\")\" = skipped_excluded ] && [ \"\$(status_of \"$PROJECT_GIT/worktrees/wt1/sharedindex.3333333333333333333333333333333333333333\")\" = skipped_excluded ]"
check "#253 project git objects, index, hooks, lfs and modules have no row" "no_git_rows objects index hooks lfs modules"
check "#253 project git config not flagged secret" "row \"$PROJECT_GIT/config\" | grep -q '\"secret\":false,\"status\":\"collected\"'"

# 2026-10-23 claude-code Windows round (#256)
check "#256 Claude Code Windows temp root collected as claude-code" "row \"$CLAUDE_TEMP_SESSION/scratchpad/notes.txt\" | grep -q '\"agent\":\"claude-code\".*\"secret\":false,\"status\":\"collected\"' && row \"$CLAUDE_TEMP_SESSION/tasks/b256b256b.output\" | grep -q '\"agent\":\"claude-code\".*\"status\":\"collected\"'"
check "#256 home-level .claude.json.backup flagged secret" "row \"$ALICE/.claude.json.backup\" | grep -q '\"agent\":\"claude-code\".*\"secret\":true,\"status\":\"collected\"'"
check "#256 Claude Code daemon keys and roster flagged secret" "row \"$ALICE/.claude/daemon/control.key\" | grep -q '\"agent\":\"claude-code\".*\"secret\":true,\"status\":\"collected\"' && row \"$ALICE/.claude/daemon/pipe.key\" | grep -q '\"secret\":true,\"status\":\"collected\"' && row \"$ALICE/.claude/daemon/roster.json\" | grep -q '\"secret\":true,\"status\":\"collected\"'"
check "#256 Claude Code .claude.json backup flagged secret" "row \"$ALICE/.claude/backups/.claude.json.backup.1792800000000\" | grep -q '\"agent\":\"claude-code\".*\"secret\":true,\"status\":\"collected\"'"
check "#256 Claude Code corrupted .claude.json copy flagged secret" "row \"$ALICE/.claude/backups/.claude.json.corrupted.2026-10-23T17-00-00-000Z\" | grep -q '\"agent\":\"claude-code\".*\"secret\":true,\"status\":\"collected\"'"
check "#256 Claude Code paste-cache collected" "row \"$ALICE/.claude/paste-cache/0123456789abcdef.txt\" | grep -q '\"agent\":\"claude-code\".*\"secret\":false,\"status\":\"collected\"'"
check "#256 Claude Code plugin directory cache excluded" "[ \"\$(status_of \"$ALICE/.claude/plugins/plugin-directory-cache-v2.json\")\" = skipped_excluded ]"
check "#256 project git info/exclude collected as project" "git_files_collected info/exclude"

# 2026-10-26 claude-code Linux round (#256)
check "#256 Claude Code version lock collected as claude-code" "row \"$ALICE/.local/state/claude/locks/2.1.310.lock\" | grep -q '\"agent\":\"claude-code\".*\"secret\":false,\"status\":\"collected\"'"
check "#256 Claude Code cache staging collected as claude-code" "row \"$ALICE/.cache/claude/staging/entry\" | grep -q '\"agent\":\"claude-code\".*\"secret\":false,\"status\":\"collected\"'"

# 2026-10-26 claude-code macOS round (#256)
check "#256 Claude Code URL handler bundle collected as claude-code" "row \"$URL_HANDLER/Info.plist\" | grep -q '\"agent\":\"claude-code\".*\"secret\":false,\"status\":\"collected\"' && row \"$URL_HANDLER/MacOS/claude\" | grep -q '\"agent\":\"claude-code\".*\"type\":\"symlink\".*\"status\":\"symlink\",\"target\":\"/home/alice/.local/bin/claude\"'"
check "#256 Claude Code IDE lock and bridge-spawn mcp.json flagged secret" "row \"$ALICE/.claude/ide/53256.lock\" | grep -q '\"agent\":\"claude-code\".*\"secret\":true,\"status\":\"collected\"' && row \"$ALICE/.claude/$BRIDGE_SPAWN/mcp.json\" | grep -q '\"agent\":\"claude-code\".*\"secret\":true,\"status\":\"collected\"' && row \"$ALICE/.claude/$BRIDGE_SPAWN/append-system-prompt.txt\" | grep -q '\"secret\":false,\"status\":\"collected\"'"
check "#256 relocated claude home IDE lock and bridge-spawn mcp.json flagged secret" "row \"$CLAUDE_WORK/ide/53256.lock\" | grep -q '\"agent\":\"claude-code\".*\"secret\":true,\"status\":\"collected\"' && row \"$CLAUDE_WORK/$BRIDGE_SPAWN/mcp.json\" | grep -q '\"agent\":\"claude-code\".*\"secret\":true,\"status\":\"collected\"'"

# 2026-10-29 drift sweep credential gaps (#248)
while IFS='|' read -r _drift_path _drift_agent; do
  check "#248 home file flagged secret: $_drift_path" "row \"$ALICE/$_drift_path\" | grep -q '\"agent\":\"$_drift_agent\".*$SECRET_COLLECTED'"
done <"$DRIFT_HOME_SECRETS"
while IFS='|' read -r _drift_path _drift_agent; do
  check "#248 project file flagged secret: $_drift_path" "row \"$ROOT/srv/proj/$_drift_path\" | grep -q '\"agent\":\"$_drift_agent\".*$SECRET_COLLECTED'"
done <"$DRIFT_PROJECT_SECRETS"
check "#322 Factory temp/env dump flagged secret" "row \"$FACTORY_ENV_DUMP\" | grep -q '\"agent\":\"factory-droid\".*$SECRET_COLLECTED'"
check "#322 Factory temp entries not starting with e excluded" "[ \"\$(status_of \"$ALICE/.factory/temp/run1\")\" = skipped_excluded ] && ! grep -qF 'temp/run1/out.txt' \"\$MANIFEST\""
check "#322 Amp cache outside the catalog not collected" "! grep -qE '\.cache/amp/(runner-desktop|desktop|bin|obelisk-runs)/' \"\$MANIFEST\""
check "#322 Ollama Library/Caches outside the catalog not collected" "! grep -qF '/Library/Caches/ollama' \"\$MANIFEST\" && ! grep -qF '/Library/Caches/com.electron.ollama' \"\$MANIFEST\""
check "#321 Windsurf AppData/Local/devin/bin outside the catalog not collected" "! grep -qiF 'AppData/Local/devin' \"\$MANIFEST\""
check "#324 Hermes suffixed home collected as hermes" "row \"$ALICE/.hermes-canary/state.db\" | grep -q '\"agent\":\"hermes\".*\"status\":\"collected\"' && row \"$ALICE/.hermes-canary/config.yaml\" | grep -q '\"agent\":\"hermes\".*$SECRET_COLLECTED'"
check "#324 Hermes suffixed Windows home config.yaml flagged secret" "row \"$ALICE/AppData/Local/hermes-canary/config.yaml\" | grep -q '\"agent\":\"hermes\".*$SECRET_COLLECTED'"
check "#324 Hermes suffixed home installer checkout excluded, its .env kept" "[ \"\$(status_of \"$ALICE/.hermes-canary/hermes-agent/hermes_cli\")\" = skipped_excluded ] && [ \"\$(status_of \"$ALICE/.hermes-canary/hermes-agent/.git\")\" = skipped_excluded ] && row \"$ALICE/.hermes-canary/hermes-agent/.env\" | grep -q '$SECRET_COLLECTED'"
check "#324 Hermes suffixed Windows home installer checkout excluded, its .env kept" "[ \"\$(status_of \"$ALICE/AppData/Local/hermes-canary/hermes-agent/hermes_cli\")\" = skipped_excluded ] && [ \"\$(status_of \"$ALICE/AppData/Local/hermes-canary/hermes-agent/.git\")\" = skipped_excluded ] && row \"$ALICE/AppData/Local/hermes-canary/hermes-agent/.env\" | grep -q '$SECRET_COLLECTED'"
check "#324 Hermes updater cache excluded" "[ \"\$(status_of \"$ALICE/AppData/Local/hermes-updater\")\" = skipped_excluded ] && ! grep -qF 'hermes-setup.exe' \"\$MANIFEST\""
while IFS= read -r _drift_path; do
  check "#324 Hermes suffixed desktop connections flagged: $_drift_path" "row \"$ALICE/$_drift_path/connections.json\" | grep -q '\"agent\":\"hermes\".*$SECRET_COLLECTED'"
  for _drift_leaf in Cache "Code Cache" GPUCache; do
    check "#324 Hermes suffixed desktop cache excluded: $_drift_path/$_drift_leaf" "[ \"\$(status_of \"$ALICE/$_drift_path/$_drift_leaf\")\" = skipped_excluded ]"
  done
done <"$DRIFT_HERMES_DESKTOP"
check "#321 Devin CLI files collected as windsurf and flagged" "row \"$ALICE/AppData/Roaming/devin/mcp_config.json\" | grep -q '\"agent\":\"windsurf\".*$SECRET_COLLECTED' && row \"$ALICE/AppData/Roaming/devin/credentials.toml\" | grep -q '\"agent\":\"windsurf\".*$SECRET_COLLECTED'"
check "#321 Devin IDE cache not collected, its User directory collected" "! grep -qiF 'Roaming/Devin/Cache' \"\$MANIFEST\" && row \"$ALICE/AppData/Roaming/Devin/User/settings.json\" | grep -q '\"agent\":\"windsurf\".*\"status\":\"collected\"'"
check "#323 Crush Windows projects.json discovers project" "row \"$ROOT/srv/crushwin/crushrc\" | grep -q '\"agent\":\"crush\"'"
for _drift_path in crushrc .crushrc crush.json .crush.json; do
  check "#323 Crush project file flagged secret: $_drift_path" "row \"$ROOT/srv/crushwin/$_drift_path\" | grep -q '\"agent\":\"crush\".*$SECRET_COLLECTED'"
done

# ---- --no-secrets ---------------------------------------------------------
run nosecrets --no-secrets; check "no-secrets exit 0" "[ $? -eq 0 ]"
check "credentials skipped_secret" "[ \"\$(status_of \"$ROOT/home/alice/.claude/.credentials.json\")\" = skipped_secret ]"
check "credentials absent from archive" "! tar -tzf \"\$ARCHIVE\" | grep -q credentials.json"
# 2026-10-11 Claude Desktop Linux deb and Claude Code 2.1.295 session keys (#73, #76)
check "#73 #76 --no-secrets skips link-key and the session key" "[ \"\$(status_of \"$CLAUDE_LINUX_CONFIG/spaces-present/0a0a0a0a/0b0b0b0b/link-key\")\" = skipped_secret ] && [ \"\$(status_of \"$SESSION_KEY\")\" = skipped_secret ]"
# 2026-10-13 Antigravity desktop app (#83, #84)
check "#84 Antigravity plugin token skipped without secrets" "[ \"\$(status_of \"$ANTIGRAVITY_DATA/plugin_data/gdocs/token.json\")\" = skipped_secret ]"
# 2026-10-13 Antigravity Linux round (#83, #90, #91)
check "#90 Antigravity CLI plugin-secrets.json skipped without secrets" "[ \"\$(status_of \"$ALICE/.config/antigravity/plugin-secrets.json\")\" = skipped_secret ]"
# 2026-10-15 ChatGPT desktop as the OpenAI.Codex package (#113)
check "#115 Codex desktop global state skipped without secrets" "[ \"\$(status_of \"$BOB/.codex/.codex-global-state.json\")\" = skipped_secret ]"
# 2026-10-16 GitHub Copilot Desktop (#147)
check "#147 Copilot Desktop ws.token skipped without secrets" "[ \"\$(status_of \"$COPILOT/run/ws.token\")\" = skipped_secret ]"
# 2026-10-17 session sync round (#153)
check "#153 nanobot remote pairing keys skipped without secrets" "[ \"\$(status_of \"$NANOBOT_PAIRING/identity\")\" = skipped_secret ] && [ \"\$(status_of \"$NANOBOT_PAIRING/exchange\")\" = skipped_secret ]"
check "#153 Agent Zero plugin config.json skipped without secrets" "[ \"\$(status_of \"$ALICE/agent-zero/main/usr/$AGENT_ZERO_PLUGIN_CONFIG\")\" = skipped_secret ] && [ \"\$(status_of \"$ALICE/Desktop/agent-zero/usr/$AGENT_ZERO_PLUGIN_CONFIG\")\" = skipped_secret ]"
check "#153 Agent Zero volume plugin config.json skipped without secrets" "[ \"\$(status_of \"$VOLUMES/a0_usr/_data/$AGENT_ZERO_PLUGIN_CONFIG\")\" = skipped_secret ] && [ \"\$(status_of \"$VOLUMES/a0_usr/_data/usr/$AGENT_ZERO_PLUGIN_CONFIG\")\" = skipped_secret ]"
check "#153 Cline cached remote config skipped without secrets" "[ \"\$(status_of \"$CLINE_REMOTE_CACHE/remote_config_org1.json\")\" = skipped_secret ]"
check "#153 Cline webhook_config.json skipped without secrets" "[ \"\$(status_of \"$ALICE/Documents/Cline/webhook_config.json\")\" = skipped_secret ]"
check "#153 Gemini CLI bug-memory heap snapshot skipped without secrets" "[ \"\$(status_of \"$ALICE/.gemini/tmp/gem/bug-memory-1792000000000.heapsnapshot\")\" = skipped_secret ]"
check "#153 little-coder legacy config.json skipped without secrets" "[ \"\$(status_of \"$ALICE/.little-coder/config.json\")\" = skipped_secret ]"
check "#153 Qwen Code channel credentials skipped without secrets" "[ \"\$(status_of \"$ALICE/.qwen/channels/qqbot-credentials.json\")\" = skipped_secret ] && [ \"\$(status_of \"$ALICE/.qwen/channels/weixin/account.json\")\" = skipped_secret ]"
check "#153 Qwen Code channel routing files kept without secrets" "[ \"\$(status_of \"$ALICE/.qwen/channels/sessions.json\")\" = collected ] && [ \"\$(status_of \"$ALICE/.qwen/channels/daemon/0123abcd/routes.json\")\" = collected ]"
# 2026-10-18 Codex shell snapshots (#160)
check "#160 Codex shell snapshot skipped without secrets" "[ \"\$(status_of \"$CODEX_SHELL_SNAPSHOT\")\" = skipped_secret ]"
# 2026-10-21 framework catalog gaps (#223, #224, #225)
check "#224 difyctl login-pending.yml skipped without secrets" "[ \"\$(status_of \"$ALICE/.config/difyctl/login-pending.yml\")\" = skipped_secret ]"
check "#225 CLAI 2.0 workspace token skipped without secrets" "[ \"\$(status_of \"$CLAI_STATE/workspaces/registry/ws1/token\")\" = skipped_secret ]"

# 2026-10-21 Agent Zero source review (#217)
check "#217 Agent Zero WhatsApp session skipped without secrets" "[ \"\$(status_of \"$ALICE/agent-zero/main/$WHATSAPP_CREDS\")\" = skipped_secret ] && [ \"\$(status_of \"$ALICE/Desktop/agent-zero/$WHATSAPP_CREDS\")\" = skipped_secret ] && [ \"\$(status_of \"$VOLUMES/a0_usr/_data/$WHATSAPP_CREDS\")\" = skipped_secret ]"

# 2026-10-21 Cody JetBrains plugin files (#214)
check "#214 Cody cody_accounts.xml skipped without secrets" "[ \"\$(status_of \"$JETBRAINS_OPTIONS/cody_accounts.xml\")\" = skipped_secret ]"
check "#214 Cody cody.xml skipped without secrets" "[ \"\$(status_of \"$JETBRAINS_OPTIONS/cody.xml\")\" = skipped_secret ]"
check "#214 Cody cody_settings.json skipped without secrets" "[ \"\$(status_of \"$ALICE/.sourcegraph/cody_settings.json\")\" = skipped_secret ] && [ \"\$(status_of \"$ROOT/srv/proj/.idea/.sourcegraph/cody_settings.json\")\" = skipped_secret ]"

# 2026-10-21 Hermes quick snapshots and desktop connections (#123, #208)
check "#123 Hermes quick snapshot credentials skipped without secrets" "[ \"\$(status_of \"$HERMES_SNAPSHOT/.env\")\" = skipped_secret ] && [ \"\$(status_of \"$HERMES_SNAPSHOT/auth.json\")\" = skipped_secret ] && [ \"\$(status_of \"$HERMES_SNAPSHOT/state.db\")\" = collected ]"
check "#208 Hermes desktop connection files skipped without secrets" "[ \"\$(status_of \"$ALICE/.config/Hermes/connections.json\")\" = skipped_secret ]"

# 2026-10-21 OpenClaw source review (#219)
check "#219 OpenClaw credentials skipped without secrets" "[ \"\$(status_of \"$OPENCLAW/exec-approvals.json\")\" = skipped_secret ] && [ \"\$(status_of \"$ALICE/.openclaw-auth-profile-secrets/gateway.env\")\" = skipped_secret ]"

# 2026-10-21 OpenCode and Kilo Code source review (#213, #221)
check "#213 OpenCode service.json skipped without secrets" "[ \"\$(status_of \"$ALICE/.config/opencode/service.json\")\" = skipped_secret ] && [ \"\$(status_of \"$ALICE/.local/state/opencode/service-local.json\")\" = skipped_secret ]"

# 2026-10-21 Qwen Code session registry (#230)
check "#230 Qwen Code session registry skipped without secrets" "[ \"\$(status_of \"$ALICE/.qwen/sessions/4242.json\")\" = skipped_secret ] && [ \"\$(status_of \"$ALICE/.qwen/sessions/4243-0123abcd.json\")\" = skipped_secret ]"

# 2026-10-23 claude-code Windows round (#256)
check "#256 Claude Code daemon keys and .claude.json backup skipped without secrets" "[ \"\$(status_of \"$ALICE/.claude/daemon/control.key\")\" = skipped_secret ] && [ \"\$(status_of \"$ALICE/.claude/daemon/roster.json\")\" = skipped_secret ] && [ \"\$(status_of \"$ALICE/.claude/backups/.claude.json.backup.1792800000000\")\" = skipped_secret ]"
check "#256 relocated claude home session key, daemon key and backup skipped without secrets" "[ \"\$(status_of \"$CLAUDE_WORK_KEY\")\" = skipped_secret ] && [ \"\$(status_of \"$CLAUDE_WORK/daemon/control.key\")\" = skipped_secret ] && [ \"\$(status_of \"$CLAUDE_WORK/backups/.claude.json.backup.1792800000000\")\" = skipped_secret ]"

# 2026-10-26 claude-code macOS round (#256)
check "#256 Claude Code IDE lock and bridge-spawn mcp.json skipped without secrets" "[ \"\$(status_of \"$ALICE/.claude/ide/53256.lock\")\" = skipped_secret ] && [ \"\$(status_of \"$ALICE/.claude/$BRIDGE_SPAWN/mcp.json\")\" = skipped_secret ] && [ \"\$(status_of \"$CLAUDE_WORK/ide/53256.lock\")\" = skipped_secret ] && [ \"\$(status_of \"$CLAUDE_WORK/$BRIDGE_SPAWN/mcp.json\")\" = skipped_secret ]"

# 2026-10-29 drift sweep credential gaps (#248)
while IFS='|' read -r _drift_path _drift_agent; do
  check "#248 home file skipped without secrets: $_drift_path" "[ \"\$(status_of \"$ALICE/$_drift_path\")\" = skipped_secret ]"
done <"$DRIFT_HOME_SECRETS"
while IFS='|' read -r _drift_path _drift_agent; do
  check "#248 project file skipped without secrets: $_drift_path" "[ \"\$(status_of \"$ROOT/srv/proj/$_drift_path\")\" = skipped_secret ]"
done <"$DRIFT_PROJECT_SECRETS"
check "#322 Factory temp/env dump skipped without secrets" "[ \"\$(status_of \"$FACTORY_ENV_DUMP\")\" = skipped_secret ]"
check "#324 Hermes suffixed homes skipped without secrets" "[ \"\$(status_of \"$ALICE/.hermes-canary/config.yaml\")\" = skipped_secret ] && [ \"\$(status_of \"$ALICE/AppData/Local/hermes-canary/config.yaml\")\" = skipped_secret ]"
while IFS= read -r _drift_path; do
  check "#324 Hermes suffixed desktop connections skipped without secrets: $_drift_path" "[ \"\$(status_of \"$ALICE/$_drift_path/connections.json\")\" = skipped_secret ]"
done <"$DRIFT_HERMES_DESKTOP"
check "#321 Devin CLI files skipped without secrets" "[ \"\$(status_of \"$ALICE/AppData/Roaming/devin/mcp_config.json\")\" = skipped_secret ] && [ \"\$(status_of \"$ALICE/AppData/Roaming/devin/credentials.toml\")\" = skipped_secret ]"
for _drift_path in crushrc .crushrc crush.json .crush.json; do
  check "#323 Crush project file skipped without secrets: $_drift_path" "[ \"\$(status_of \"$ROOT/srv/crushwin/$_drift_path\")\" = skipped_secret ]"
done

# 2026-10-04 catalog cases
printf '%s\n' "$CASES_CATALOG" >"$WORK/nosecrets14"
while IFS='|' read -r _kind _agent _relpath; do
  [ -n "$_relpath" ] || continue
  _row=$(row "$ALICE/$_relpath")
  case "$_kind" in
    secret) case "$_row" in *'"status":"skipped_secret"'*) pass "no-secrets: $_relpath" ;; *) fail_check "no-secrets: $_relpath" ;; esac ;;
  esac
done <"$WORK/nosecrets14"
# 2026-10-04 Muse Code
printf '%s\n' "$CASES_MUSE" >"$WORK/nosecretsmuse"
while IFS='|' read -r _kind _agent _relpath; do
  [ -n "$_relpath" ] || continue
  _row=$(row "$ALICE/$_relpath")
  case "$_kind" in
    secret) case "$_row" in *'"agent":"muse-code"'*'"status":"skipped_secret"'*) pass "no-secrets: $_relpath" ;; *) fail_check "no-secrets: $_relpath" ;; esac ;;
  esac
done <"$WORK/nosecretsmuse"
# 2026-10-06 frameworks sweep (#64)
printf '%s\n' "$CASES_FW" >"$WORK/nosecretsfw"
while IFS='|' read -r _kind _agent _relpath; do
  [ -n "$_relpath" ] || continue
  _row=$(row "$ALICE/$_relpath")
  case "$_kind" in
    secret) case "$_row" in *'"status":"skipped_secret"'*) pass "no-secrets: $_relpath" ;; *) fail_check "no-secrets: $_relpath" ;; esac ;;
  esac
done <"$WORK/nosecretsfw"

# ---- --full ---------------------------------------------------------------
run full --full; check "full exit 0" "[ $? -eq 0 ]"
check "cache contents collected with --full" "[ \"\$(status_of \"$ROOT/home/alice/.claude/cache/junk.bin\")\" = collected ]"
# 2026-10-14 catalog match that is an excluded file (#107)
check "#107 excluded file match collected with --full" "[ \"\$(status_of \"$ALICE/.local/bin/.muse-update.107.exe\")\" = collected ]"

# ---- -u filter ------------------------------------------------------------
run users -u bob; check "users exit 0" "[ $? -eq 0 ]"
check "only bob collected" "! grep -q '\"user\":\"alice\"' \"\$MANIFEST\" && grep -q '\"user\":\"bob\"' \"\$MANIFEST\""

# ---- --no-docker ----------------------------------------------------------
run nodocker --no-docker; check "no-docker exit 0" "[ $? -eq 0 ]"
check "no-docker collects no volume" "! grep -q 'docker/volumes' \"\$MANIFEST\""
check "no-docker recorded in options" "grep -q '\"no_docker\": true' \"\$SUMMARY\""
check "no-docker prints no docker line" "! grep -q '^docker:' \"$OUT/nodocker.stdout\""

# 2026-10-14 home patterns and the working directory (#105)
(cd "$HOMES_CWD" && run cwd105 --no-docker --no-projects); _exit_homes_cwd=$?
MANIFEST_HOMES_CWD=$(ls "$OUT/cwd105"/*.manifest.jsonl 2>/dev/null)
SUMMARY_HOMES_CWD=$(ls "$OUT/cwd105"/*.collection.json 2>/dev/null)
check "#105 image homes found from a directory with home and Users subdirectories" "[ $_exit_homes_cwd -eq 0 ] && grep -q '\"user\":\"bob\"' \"$MANIFEST_HOMES_CWD\" && [ \"\$(grep '^  \"users\":' \"$SUMMARY_HOMES_CWD\")\" = \"\$(grep '^  \"users\":' \"$OUT\"/default/*.collection.json)\" ]"

# 2026-10-14 NESTED_CLAIMS from the environment (#106)
# An inherited value naming a subtree of a matched volume must prune nothing.
NESTED_CLAIMS="$AGENT_ZERO_VOLUME/chats"; export NESTED_CLAIMS
run claims106 --no-projects; _exit_claims=$?
unset NESTED_CLAIMS
check "#106 NESTED_CLAIMS in the environment prunes nothing" "[ $_exit_claims -eq 0 ] && [ \"\$(status_of \"$AGENT_ZERO_VOLUME/chats/ctx1/chat.json\")\" = collected ]"

# ---- unreadable volume (needs a non-root user; chmod 000 does not stop root)
if [ "$(id -u)" != 0 ]; then
  mkdir -p "$VOLUMES/hermes_data/_data/sessions"
  printf '{}\n' >"$VOLUMES/hermes_data/_data/sessions/s.json"
  chmod 000 "$VOLUMES/hermes_data/_data"
  # a root-only data root, as /var/lib/containers is for a non-root responder
  mkdir -p "$ROOT/var/lib/containers/storage/volumes/x_ollama/_data"
  chmod 000 "$ROOT/var/lib/containers"
  run dockerperm; check "unreadable volume exit 0" "[ $? -eq 0 ]"
  chmod 700 "$VOLUMES/hermes_data/_data" "$ROOT/var/lib/containers"
  check "unreadable volume and data root counted in stdout" "grep -q '^docker:     4 volumes found, 2 collected, 2 unreadable (run as root to collect)\$' \"$OUT/dockerperm.stdout\""
  check "unreadable volume noted in collection.json" "grep -q 'docker: volume hermes_data (hermes) at .* is not readable' \"\$SUMMARY\""
  check "unreadable data root noted in collection.json" "grep -q 'docker: [^\"]*/var/lib/containers is not readable' \"\$SUMMARY\""
  check "other volumes still collected" "[ \"\$(status_of \"$AGENT_ZERO_VOLUME/chats/ctx1/chat.json\")\" = collected ]"
  rm -rf "$VOLUMES/hermes_data" "$ROOT/var/lib/containers"
else
  printf 'skip unreadable volume check (running as root)\n'
fi

# ---- Docker Desktop note (macOS layout) -----------------------------------
mkdir -p "$BOB/Library/Containers/com.docker.docker/Data"
run desktop; check "docker desktop exit 0" "[ $? -eq 0 ]"
check "docker desktop noted in stdout" "grep -q '^docker:     3 volumes found, 2 collected, 0 unreadable; Docker Desktop VM disk not collected\$' \"$OUT/desktop.stdout\""
check "docker desktop noted in collection.json" "grep -q '\"docker: Docker Desktop data at .*com.docker.docker; volumes inside its virtual machine disk are not collected\"' \"\$SUMMARY\" && grep -q '\"docker_desktop\": true' \"\$SUMMARY\""
rm -rf "$BOB/Library/Containers"

# 2026-10-06 frameworks sweep (#64)
# Dify, Langflow and n8n volumes, in a run of their own so the counts above hold.
N8N_VOLUME="$VOLUMES/n8n_data/_data"
mkdir -p "$N8N_VOLUME/nodes/node_modules/pkg" "$N8N_VOLUME/node-definitions" "$N8N_VOLUME/n8n-sdk-templates" "$N8N_VOLUME/ssh" \
  "$VOLUMES/n8n_n8n-data/_data" "$VOLUMES/langflow_langflow-data/_data/profile_pictures" \
  "$VOLUMES/docker_dify_agent_local_sandbox_home/_data/.local/share/shellctl" "$VOLUMES/docker_dify_agent_local_sandbox_workspace/_data"
printf '{"encryptionKey":"k"}\n' >"$N8N_VOLUME/config"
printf 'db\n' >"$N8N_VOLUME/database.sqlite"
printf 'x\n' >"$N8N_VOLUME/nodes/node_modules/pkg/index.js"
printf 'x\n' >"$N8N_VOLUME/node-definitions/d.json"
printf 'x\n' >"$N8N_VOLUME/n8n-sdk-templates/t.json"
printf 'key\n' >"$N8N_VOLUME/ssh/key"
printf 'db\n' >"$VOLUMES/n8n_n8n-data/_data/database.sqlite"
printf 'k\n' >"$VOLUMES/langflow_langflow-data/_data/secret_key"
printf 'k\n' >"$VOLUMES/langflow_langflow-data/_data/cache_secret_key"
printf 'x\n' >"$VOLUMES/langflow_langflow-data/_data/profile_pictures/p.svg"
printf 'db\n' >"$VOLUMES/docker_dify_agent_local_sandbox_home/_data/.local/share/shellctl/shellctl.db"
printf 'x\n' >"$VOLUMES/docker_dify_agent_local_sandbox_workspace/_data/out.txt"
# 2026-10-21 Hermes quick snapshots and desktop connections (#123, #208)
# A Hermes home in a volume needs the volume-relative secret globs.
HERMES_VOLUME_SNAPSHOT="$VOLUMES/hermes_home/_data/state-snapshots/20261001-120004"
mkdir -p "$HERMES_VOLUME_SNAPSHOT/pairing"
printf 'sqlite\n' >"$HERMES_VOLUME_SNAPSHOT/state.db"
printf 'KEY=x\n' >"$HERMES_VOLUME_SNAPSHOT/.env"
printf '{"providers":{}}\n' >"$HERMES_VOLUME_SNAPSHOT/auth.json"
printf '{}\n' >"$HERMES_VOLUME_SNAPSHOT/pairing/telegram-approved.json"
# 2026-10-30 hermes rebuild (#326)
# One file under each volume-relative Hermes exclusion and secret glob, as kind|path below _data.
HERMES_VOLUME="$VOLUMES/hermes_home/_data"
HERMES_VOLUME_CASES="$WORK/hermes-volume-cases"
cat >"$HERMES_VOLUME_CASES" <<'EOF_HERMES_VOLUME_CASES'
secret|.env
secret|.env.bak1
secret|.op.env
secret|npmrc
secret|auth.json
secret|auth.json.1
secret|auth/google_oauth.json
secret|.anthropic_oauth.json
secret|.copilot_jwt.json
secret|google_token.json
secret|google_chat_user_tokens/u1.json
secret|slack_tokens.json
secret|honcho.json
secret|mem0.json
secret|webhook_subscriptions.json
secret|teams_pipeline_store.json
secret|mcp-tokens/t1.json
secret|vault/vault.key
secret|browser_auth/state.json
secret|bot-desktop/lease.json
secret|webapp/session-token
secret|webapp/native-sign-in-tokens.json
secret|webapp/sign-in-registration.json
secret|pairing/telegram-approved.json
secret|platforms/pairing/telegram-approved.json
secret|feishu_comment_pairing.json
secret|whatsapp/session/creds.json
secret|platforms/whatsapp/session/creds.json
secret|matrix/store/crypto.db
secret|platforms/matrix/store/crypto.db
secret|cache/bws_cache.json
secret|workspace/meetings/node_token.json
secret|weixin/accounts/a1.json
secret|runtime/photon-sidecar.json
secret|proxy/ca.key
secret|home/.gitconfig
secret|config.yaml.bak.1
secret|.ssh/id_ed25519
secret|.aws/credentials
secret|.aws/config
secret|.gnupg/pubring.kbx
secret|.kube/config
secret|.docker/config.json
secret|.azure/msal_token_cache.json
secret|.config/gh/hosts.yml
secret|.config/gcloud/credentials.db
secret|.envrc
secret|state-snapshots/20261001-120004/.env
secret|state-snapshots/20261001-120004/auth.json
secret|state-snapshots/20261001-120004/config.yaml
secret|state-snapshots/20261001-120004/feishu_comment_pairing.json
secret|state-snapshots/20261001-120004/platforms/pairing/telegram-approved.json
excl|hermes-agent/hermes_cli
excl|hermes-agent/.git
excl|models
excl|runtimes
excl|node
excl|installs
excl|tools
excl|sandboxes
excl|checkpoints/store/objects
excl|checkpoints/store/indexes
excl|checkpoints/legacy-1
excl|backups
excl|browser-profile
excl|browser-profiles
excl|browser_profiles
excl|chrome-debug
excl|bot-desktop/browser-profile
excl|plugins/p1/node_modules
excl|plugins/p1/.venv
secret|profiles/work/.env
secret|profiles/work/.env.bak1
secret|profiles/work/.op.env
secret|profiles/work/npmrc
secret|profiles/work/auth.json
secret|profiles/work/auth.json.1
secret|profiles/work/auth/google_oauth.json
secret|profiles/work/.anthropic_oauth.json
secret|profiles/work/.copilot_jwt.json
secret|profiles/work/google_token.json
secret|profiles/work/google_chat_user_tokens/u1.json
secret|profiles/work/slack_tokens.json
secret|profiles/work/honcho.json
secret|profiles/work/mem0.json
secret|profiles/work/webhook_subscriptions.json
secret|profiles/work/teams_pipeline_store.json
secret|profiles/work/mcp-tokens/t1.json
secret|profiles/work/vault/vault.key
secret|profiles/work/browser_auth/state.json
secret|profiles/work/bot-desktop/lease.json
secret|profiles/work/webapp/session-token
secret|profiles/work/webapp/native-sign-in-tokens.json
secret|profiles/work/webapp/sign-in-registration.json
secret|profiles/work/pairing/telegram-approved.json
secret|profiles/work/platforms/pairing/telegram-approved.json
secret|profiles/work/feishu_comment_pairing.json
secret|profiles/work/whatsapp/session/creds.json
secret|profiles/work/platforms/whatsapp/session/creds.json
secret|profiles/work/matrix/store/crypto.db
secret|profiles/work/platforms/matrix/store/crypto.db
secret|profiles/work/cache/bws_cache.json
secret|profiles/work/workspace/meetings/node_token.json
secret|profiles/work/weixin/accounts/a1.json
secret|profiles/work/runtime/photon-sidecar.json
secret|profiles/work/proxy/ca.key
secret|profiles/work/home/.gitconfig
secret|profiles/work/config.yaml.bak.1
secret|profiles/work/.ssh/id_ed25519
secret|profiles/work/.aws/credentials
secret|profiles/work/.aws/config
secret|profiles/work/.gnupg/pubring.kbx
secret|profiles/work/.kube/config
secret|profiles/work/.docker/config.json
secret|profiles/work/.azure/msal_token_cache.json
secret|profiles/work/.config/gh/hosts.yml
secret|profiles/work/.config/gcloud/credentials.db
secret|profiles/work/.envrc
secret|profiles/work/config.yaml
secret|.aws/cli/cache/x.json
secret|profiles/work/.aws/sso/cache/x.json
secret|profiles/work/.aws/cli/cache/x.json
plain|.aws/amazonq/history/x.json
plain|profiles/work/state.db
EOF_HERMES_VOLUME_CASES
while IFS='|' read -r _hermes_kind _hermes_path; do
  case "$_hermes_kind" in
    excl) mkdir -p "$HERMES_VOLUME/$_hermes_path"; printf 'x\n' >"$HERMES_VOLUME/$_hermes_path/f" ;;
    *) mkdir -p "$(dirname "$HERMES_VOLUME/$_hermes_path")"; printf 'x\n' >"$HERMES_VOLUME/$_hermes_path" ;;
  esac
done <"$HERMES_VOLUME_CASES"
# 2026-10-29 drift sweep credential gaps (#248)
LANGFLOW_VOLUME_MCP="$VOLUMES/langflow_langflow-data/_data/0f0e0d0c-1111-4000-8000-000000000248/_mcp_servers_0f0e0d0c-1111-4000-8000-000000000248.json"
OPENHANDS_VOLUME="$VOLUMES/openhands_state/_data"
OLLAMA_VOLUME="$VOLUMES/ollama/_data"
mkdir -p "$(dirname "$LANGFLOW_VOLUME_MCP")" "$OPENHANDS_VOLUME/conversations/c1/acp/codex" \
  "$OPENHANDS_VOLUME/agent-canvas/conversations/c2/acp/codex" "$OLLAMA_VOLUME/backup/codex" \
  "$OLLAMA_VOLUME/launch/muse-config/muse"
printf '{"mcpServers":{}}\n' >"$LANGFLOW_VOLUME_MCP"
printf 'mcp_servers: {}\n' >"$VOLUMES/hermes_home/_data/config.yaml"
printf '{"tokens":{}}\n' >"$OPENHANDS_VOLUME/conversations/c1/acp/codex/auth.json"
printf '{"tokens":{}}\n' >"$OPENHANDS_VOLUME/agent-canvas/conversations/c2/acp/codex/auth.json"
# 2026-10-30 hermes rebuild (#326)
mkdir -p "$OPENHANDS_VOLUME/profiles"
printf '{"model":"x","api_key":"k"}\n' >"$OPENHANDS_VOLUME/profiles/p1.json"
printf '{"model":"x","api_key":"k"}\n' >"$OPENHANDS_VOLUME/profiles/tmpabc123.tmp"
mkdir -p "$VOLUMES/hermes_home/_data/profiles/work/pastes"
printf '[]\n' >"$VOLUMES/hermes_home/_data/profiles/work/pastes/pending.json"
printf 'model = "x"\n' >"$OLLAMA_VOLUME/backup/codex/config.toml.1792800000"
printf '{}\n' >"$OLLAMA_VOLUME/launch/muse-config/muse/settings.json"
run fwvolumes; check "frameworks volumes exit 0" "[ $? -eq 0 ]"
check "n8n_data volume collected as n8n" "row \"$N8N_VOLUME/database.sqlite\" | grep -q '\"user\":\"docker\",\"home\":\"$N8N_VOLUME\",\"agent\":\"n8n\".*\"status\":\"collected\"'"
check "n8n volume config flagged secret (volume-relative)" "row \"$N8N_VOLUME/config\" | grep -q '\"secret\":true,\"status\":\"collected\"'"
check "n8n volume ssh key flagged secret (volume-relative)" "row \"$N8N_VOLUME/ssh/key\" | grep -q '\"secret\":true,\"status\":\"collected\"'"
check "n8n volume nodes/node_modules excluded (volume-relative)" "[ \"\$(status_of \"$N8N_VOLUME/nodes/node_modules\")\" = skipped_excluded ]"
check "n8n volume node-definitions excluded (volume-relative)" "[ \"\$(status_of \"$N8N_VOLUME/node-definitions\")\" = skipped_excluded ]"
check "n8n volume n8n-sdk-templates excluded (volume-relative)" "[ \"\$(status_of \"$N8N_VOLUME/n8n-sdk-templates\")\" = skipped_excluded ]"
check "n8n_n8n-data volume collected as n8n" "row \"$VOLUMES/n8n_n8n-data/_data/database.sqlite\" | grep -q '\"agent\":\"n8n\".*\"status\":\"collected\"'"
check "langflow-data volume secret_key flagged (volume-relative)" "row \"$VOLUMES/langflow_langflow-data/_data/secret_key\" | grep -q '\"agent\":\"langflow\".*\"secret\":true,\"status\":\"collected\"'"
check "langflow-data volume cache_secret_key flagged (volume-relative)" "row \"$VOLUMES/langflow_langflow-data/_data/cache_secret_key\" | grep -q '\"secret\":true,\"status\":\"collected\"'"
check "langflow-data volume profile_pictures excluded (volume-relative)" "[ \"\$(status_of \"$VOLUMES/langflow_langflow-data/_data/profile_pictures\")\" = skipped_excluded ]"
check "dify sandbox home volume collected as dify" "row \"$VOLUMES/docker_dify_agent_local_sandbox_home/_data/.local/share/shellctl/shellctl.db\" | grep -q '\"agent\":\"dify\".*\"status\":\"collected\"'"
check "dify sandbox workspace volume collected as dify" "row \"$VOLUMES/docker_dify_agent_local_sandbox_workspace/_data/out.txt\" | grep -q '\"agent\":\"dify\".*\"status\":\"collected\"'"
# 2026-10-21 Hermes quick snapshots and desktop connections (#123, #208)
check "#123 Hermes volume quick snapshot collected, credentials flagged (volume-relative)" "row \"$HERMES_VOLUME_SNAPSHOT/state.db\" | grep -q '\"user\":\"docker\".*\"agent\":\"hermes\".*\"secret\":false,\"status\":\"collected\"' && row \"$HERMES_VOLUME_SNAPSHOT/.env\" | grep -q '\"secret\":true,\"status\":\"collected\"' && row \"$HERMES_VOLUME_SNAPSHOT/auth.json\" | grep -q '\"secret\":true,\"status\":\"collected\"' && row \"$HERMES_VOLUME_SNAPSHOT/pairing/telegram-approved.json\" | grep -q '\"secret\":true,\"status\":\"collected\"'"
# 2026-10-30 hermes rebuild (#326)
while IFS='|' read -r _hermes_kind _hermes_path; do
  case "$_hermes_kind" in
    excl) check "#326 Hermes volume excluded: $_hermes_path" "[ \"\$(status_of \"$HERMES_VOLUME/$_hermes_path\")\" = skipped_excluded ]" ;;
    plain) check "#326 Hermes volume collected, not secret: $_hermes_path" "row \"$HERMES_VOLUME/$_hermes_path\" | grep -q '\"agent\":\"hermes\".*\"secret\":false,\"status\":\"collected\"'" ;;
    *) check "#326 Hermes volume flagged: $_hermes_path" "row \"$HERMES_VOLUME/$_hermes_path\" | grep -q '\"agent\":\"hermes\".*$SECRET_COLLECTED'" ;;
  esac
done <"$HERMES_VOLUME_CASES"
# 2026-10-29 drift sweep credential gaps (#248)
check "#248 langflow-data volume _mcp_servers file flagged (volume-relative)" "row \"$LANGFLOW_VOLUME_MCP\" | grep -q '\"agent\":\"langflow\".*$SECRET_COLLECTED'"
check "#248 Hermes volume config.yaml flagged (volume-relative)" "row \"$VOLUMES/hermes_home/_data/config.yaml\" | grep -q '\"agent\":\"hermes\".*$SECRET_COLLECTED'"
check "#248 OpenHands volume conversation acp/codex/auth.json flagged (volume-relative)" "row \"$OPENHANDS_VOLUME/conversations/c1/acp/codex/auth.json\" | grep -q '\"agent\":\"openhands\".*$SECRET_COLLECTED' && row \"$OPENHANDS_VOLUME/agent-canvas/conversations/c2/acp/codex/auth.json\" | grep -q '\"agent\":\"openhands\".*$SECRET_COLLECTED'"
# 2026-10-30 hermes rebuild (#326)
check "#326 OpenHands volume LLM profile flagged (volume-relative)" "row \"$OPENHANDS_VOLUME/profiles/p1.json\" | grep -q '\"agent\":\"openhands\".*$SECRET_COLLECTED'"
check "#326 OpenHands volume interrupted profile save flagged (volume-relative)" "row \"$OPENHANDS_VOLUME/profiles/tmpabc123.tmp\" | grep -q '\"agent\":\"openhands\".*$SECRET_COLLECTED'"
check "#326 Hermes volume profile pastes/pending.json flagged by profiles/*.json (accepted overmatch)" "row \"$VOLUMES/hermes_home/_data/profiles/work/pastes/pending.json\" | grep -q '\"agent\":\"hermes\".*$SECRET_COLLECTED'"
check "#320 Ollama volume backup and Muse copy flagged (volume-relative)" "row \"$OLLAMA_VOLUME/backup/codex/config.toml.1792800000\" | grep -q '\"agent\":\"ollama\".*$SECRET_COLLECTED' && row \"$OLLAMA_VOLUME/launch/muse-config/muse/settings.json\" | grep -q '\"agent\":\"ollama\".*$SECRET_COLLECTED'"
run fwvolumesnosecrets --no-secrets; check "#248 frameworks volumes no-secrets exit 0" "[ $? -eq 0 ]"
check "#248 langflow-data volume _mcp_servers file skipped without secrets (volume-relative)" "[ \"\$(status_of \"$LANGFLOW_VOLUME_MCP\")\" = skipped_secret ]"
check "#248 Hermes volume config.yaml skipped without secrets (volume-relative)" "[ \"\$(status_of \"$VOLUMES/hermes_home/_data/config.yaml\")\" = skipped_secret ]"
# 2026-10-30 hermes rebuild (#326)
while IFS='|' read -r _hermes_kind _hermes_path; do
  case "$_hermes_kind" in
    secret) check "#326 Hermes volume skipped without secrets: $_hermes_path" "[ \"\$(status_of \"$HERMES_VOLUME/$_hermes_path\")\" = skipped_secret ]" ;;
  esac
done <"$HERMES_VOLUME_CASES"
check "#248 OpenHands volume conversation acp/codex/auth.json skipped without secrets (volume-relative)" "[ \"\$(status_of \"$OPENHANDS_VOLUME/conversations/c1/acp/codex/auth.json\")\" = skipped_secret ] && [ \"\$(status_of \"$OPENHANDS_VOLUME/agent-canvas/conversations/c2/acp/codex/auth.json\")\" = skipped_secret ]"
# 2026-10-30 hermes rebuild (#326)
check "#326 OpenHands volume LLM profile skipped without secrets (volume-relative)" "[ \"\$(status_of \"$OPENHANDS_VOLUME/profiles/p1.json\")\" = skipped_secret ]"
check "#326 OpenHands volume interrupted profile save skipped without secrets (volume-relative)" "[ \"\$(status_of \"$OPENHANDS_VOLUME/profiles/tmpabc123.tmp\")\" = skipped_secret ]"
check "#320 Ollama volume backup and Muse copy skipped without secrets (volume-relative)" "[ \"\$(status_of \"$OLLAMA_VOLUME/backup/codex/config.toml.1792800000\")\" = skipped_secret ] && [ \"\$(status_of \"$OLLAMA_VOLUME/launch/muse-config/muse/settings.json\")\" = skipped_secret ]"
rm -rf "$VOLUMES/n8n_data" "$VOLUMES/n8n_n8n-data" "$VOLUMES/langflow_langflow-data" "$VOLUMES/docker_dify_agent_local_sandbox_home" "$VOLUMES/docker_dify_agent_local_sandbox_workspace" "$VOLUMES/hermes_home" "$VOLUMES/openhands_state" "$VOLUMES/ollama"

# ---- --inventory ----------------------------------------------------------
# Stat-only fleet inventory: JSON Lines on stdout, nothing written anywhere.
inventory_run() { # inventory_run NAME ARGS... -> $OUT/NAME.stdout and $OUT/NAME.stderr
  _name=$1; shift
  COLLECTOR_SH="$SHELL_UNDER_TEST" "$SHELL_UNDER_TEST" "$SCRIPT" -r "$ROOT" --inventory "$@" >"$OUT/$_name.stdout" 2>"$OUT/$_name.stderr"
}
# shellcheck disable=SC2317,SC2329 # called from the eval'd check strings
agent_line() { grep -F "\"user\":\"$2\",\"agent\":\"$3\"," "$OUT/$1.stdout" | head -n1; }
INVENTORY_HOST_PATTERN='^{"type":"host","host":"[^"]*","collector":"[0-9.]*","mode":"image","at":"[0-9-]*T[0-9:]*Z","users_scanned":[0-9]*,"users_unreadable":[0-9]*,"docker_volumes":[0-9]*}$'
INVENTORY_AGENT_PATTERN='^{"type":"agent","host":"[^"]*","user":"[^"]*","agent":"[^"]*","files":[0-9]*,"bytes":[0-9]*,"first":"[^"]*","last":"[^"]*","projects":[0-9]*,"evidence":"[^"]*"}$'
TZ=UTC0 touch -t 202609150102.03 "$ROOT/home/ollama/.ollama/history"
mkdir -p "$OUT/invcwd" "$BOB/.gemini/antigravity-cli/conversations"
# bob's ~/.gemini holds only the nested antigravity-cli entry: no gemini-cli line.
printf 'db\n' >"$BOB/.gemini/antigravity-cli/conversations/b1.db"
(cd "$OUT/invcwd" && inventory_run inventory -q); check "inventory exit 0" "[ $? -eq 0 ]"
check "inventory writes nothing in the current directory" "[ -z \"\$(ls -A \"$OUT/invcwd\")\" ]"
check "inventory -q leaves stderr empty" "[ ! -s \"$OUT/inventory.stderr\" ]"
check "inventory first line is the host line, keys in order" "head -n1 \"$OUT/inventory.stdout\" | grep -q '$INVENTORY_HOST_PATTERN'"
check "inventory host line counts" "head -n1 \"$OUT/inventory.stdout\" | grep -q '\"users_unreadable\":0,\"docker_volumes\":3}\$'"
check "inventory every later line is an agent line, keys in order" "[ \"\$(sed 1d \"$OUT/inventory.stdout\" | grep -vc '$INVENTORY_AGENT_PATTERN')\" = 0 ] && [ \"\$(grep -c . \"$OUT/inventory.stdout\")\" -gt 10 ]"
check "inventory ollama line: history and manifest, excluded blobs not counted" "agent_line inventory ollama ollama | grep -q '\"files\":2,\"bytes\":25,\"first\":\"2026-09-14T00:00:00Z\",\"last\":\"2026-09-15T01:02:03Z\",\"projects\":0,\"evidence\":\".ollama\"}\$'"
check "inventory claude-code evidence in catalog order" "agent_line inventory alice claude-code | grep -q '\"evidence\":\".claude,.claude.json\\*,.claude-\\*/projects,.claude-\\*/history.jsonl,.claude-\\*/.claude.json\\*,.claude-\\*/.credentials.json,.claude-\\*/sessions/\\*.\\*.key,.claude-\\*/daemon/control.key,.claude-\\*/daemon/roster.json,.claude-\\*/backups/.claude.json.\\*,.claude-\\*/ide/\\*.lock,.claude-\\*/bridge-spawn/\\*/mcp.json,.local/state/claude,.cache/claude,Library/Caches/claude-cli-nodejs,AppData/Local/claude-cli-nodejs/Cache/\\*,AppData/Local/Temp/claude,Applications/Claude Code URL Handler.app\"}\$'"
check "inventory first and last set for alice claude-code" "agent_line inventory alice claude-code | grep -q '\"first\":\"20[0-9-]*T[0-9:]*Z\",\"last\":\"20[0-9-]*T[0-9:]*Z\"'"
check "inventory nested entry claimed from the enclosing agent" "agent_line inventory alice antigravity | grep -q '\"evidence\":\"[^\"]*.gemini/antigravity-cli' && agent_line inventory alice gemini-cli | grep -q '\"evidence\":\".gemini\"'"
check "inventory: matched directory with no files of its own gives no line" "[ -z \"\$(agent_line inventory bob gemini-cli)\" ] && agent_line inventory bob antigravity | grep -q '\"files\":1,\"bytes\":3,.*\"evidence\":\".gemini/antigravity-cli\"}\$'"
check "inventory: no agent line has zero files" "! grep -q '\"files\":0,' \"$OUT/inventory.stdout\""
check "inventory projects counted per user" "agent_line inventory alice claude-code | grep -q '\"projects\":[1-9]'"
check "inventory docker volume line" "agent_line inventory docker agent-zero | grep -q '\"files\":7,\"bytes\":127,.*\"projects\":0,\"evidence\":\"\\*a0_usr\"}\$'"
check "inventory rootless docker volume line" "agent_line inventory docker tabby | grep -q '\"files\":1,\"bytes\":23,.*\"evidence\":\"\\*tabby\\*\"}\$'"
check "inventory shared not inventoried" "! grep -q '\"agent\":\"\\(shared\\|project\\|live\\)\"' \"$OUT/inventory.stdout\""
check "inventory prints no path below a home" "! grep -q '$ROOT' \"$OUT/inventory.stdout\""
# 2026-10-19 Codex projectless output directories (#149)
check "#149 inventory codex-cli evidence names Documents/Codex" "agent_line inventory bob codex-cli | grep -q '\"evidence\":\"[^\"]*Documents/Codex'"
if command -v python3 >/dev/null 2>&1; then
  check "inventory stdout is valid JSON Lines" "python3 -c 'import json,sys; [json.loads(line) for line in open(sys.argv[1])]' \"$OUT/inventory.stdout\""
fi
inventory_run inventory-o -o "$OUT/inv-o"; check "inventory with -o exit 0" "[ $? -eq 0 ]"
check "inventory with -o creates no output directory" "[ ! -e \"$OUT/inv-o\" ]"
check "inventory with -o notes it on stderr" "grep -q 'NOTE: --inventory writes nothing; -o .*inv-o ignored' \"$OUT/inventory-o.stderr\""
check "inventory without -q logs to stderr only" "[ -s \"$OUT/inventory-o.stderr\" ] && [ \"\$(grep -vc '$INVENTORY_HOST_PATTERN' \"$OUT/inventory-o.stdout\" | head -n1)\" = \"\$(grep -c '$INVENTORY_AGENT_PATTERN' \"$OUT/inventory-o.stdout\")\" ]"
inventory_run inventory-full -q --full
check "inventory --full counts excluded files" "agent_line inventory-full ollama ollama | grep -q '\"files\":3,\"bytes\":30,'"
# 2026-10-14 catalog match that is an excluded file (#107)
# --full adds one file for each excl and exclf case of CASES_MUSE, plus the
# updater file, which the default inventory must not count either.
# shellcheck disable=SC2317,SC2329 # called from the eval'd check string
inventory_files() { agent_line "$1" alice muse-code | sed 's/.*"files":\([0-9]*\),.*/\1/'; }
check "#107 inventory counts an excluded file match only with --full" "[ \"\$(( \$(inventory_files inventory-full) - \$(inventory_files inventory) ))\" = \"\$(( \$(printf '%s\\n' \"\$CASES_MUSE\" | grep -c '^excl') + 1 ))\" ]"
inventory_run inventory-u -q -u bob --no-docker
check "inventory -u keeps one user" "! grep -q '\"user\":\"alice\"' \"$OUT/inventory-u.stdout\" && grep -q '\"user\":\"bob\"' \"$OUT/inventory-u.stdout\""
check "inventory --no-docker: no volumes, no docker lines" "head -n1 \"$OUT/inventory-u.stdout\" | grep -q '\"users_scanned\":1,\"users_unreadable\":0,\"docker_volumes\":0}' && ! grep -q '\"user\":\"docker\"' \"$OUT/inventory-u.stdout\""
# 2026-10-04 bug round: #29
if [ "$CASE_INSENSITIVE" = 1 ]; then
  check "#29 inventory evidence keeps only the first spelling" "agent_line inventory alice pearai | grep -q '\"evidence\":\"[^\"]*.config/PearAI/User' && ! agent_line inventory alice pearai | grep -q '.config/pearai/User'"
else
  check "#29 inventory evidence keeps both distinct spellings" "agent_line inventory alice pearai | grep -q '.config/PearAI/User,.config/pearai/User'"
fi
rm -rf "$BOB/.gemini"
if [ "$(id -u)" != 0 ]; then
  mkdir -p "$ROOT/home/carol/.claude"
  chmod 000 "$ROOT/home/carol"
  inventory_run inventory-perm -q; check "inventory unreadable home exit 0" "[ $? -eq 0 ]"
  chmod 700 "$ROOT/home/carol"
  check "inventory counts the unreadable home" "head -n1 \"$OUT/inventory-perm.stdout\" | grep -q '\"users_unreadable\":1,'"
  check "inventory scanned count includes it" "[ \"\$(head -n1 \"$OUT/inventory-perm.stdout\" | sed 's/.*\"users_scanned\":\\([0-9]*\\).*/\\1/')\" = \"\$(( \$(head -n1 \"$OUT/inventory.stdout\" | sed 's/.*\"users_scanned\":\\([0-9]*\\).*/\\1/') + 1 ))\" ]"
  rm -rf "$ROOT/home/carol"
else
  printf 'skip unreadable home inventory check (running as root)\n'
fi

# ---- live mode without the snapshot ---------------------------------------
mkdir -p "$OUT/livehost"
COLLECTOR_SH="$SHELL_UNDER_TEST" "$SHELL_UNDER_TEST" "$SCRIPT" -o "$OUT/livehost" -q -u no-such-user-cac --no-docker --no-projects >/dev/null 2>&1
check "live mode exit 0" "[ $? -eq 0 ]"
LIVE_ARCHIVE=$(ls "$OUT/livehost"/*.tar.gz 2>/dev/null)
check "live mode writes no live directory" "! tar -tzf \"$LIVE_ARCHIVE\" | grep -q '^./live/'"
check "live mode manifest has no live rows" "! tar -xzOf \"$LIVE_ARCHIVE\" ./manifest.jsonl | grep -q '\"agent\":\"live\"'"
check "live mode collection.json has hostname and no no_live" "tar -xzOf \"$LIVE_ARCHIVE\" ./collection.json | grep -q '\"hostname\": \"[^\"]' && ! tar -xzOf \"$LIVE_ARCHIVE\" ./collection.json | grep -q no_live"
check "--no-live is no longer an option" "! \"$SHELL_UNDER_TEST\" \"$SCRIPT\" --no-live -o \"$OUT/nolive\" -q >/dev/null 2>&1"

# ---- --list ---------------------------------------------------------------
check "--list prints catalog" "\"$SHELL_UNDER_TEST\" \"$SCRIPT\" --list | grep -q '^claude-code|.claude$'"
check "--list prints docker volume table" "\"$SHELL_UNDER_TEST\" \"$SCRIPT\" --list | grep -q '^agent-zero|\\*a0_usr$'"

# 2026-10-04 bug round: #31
# collection.json names the archive and archiver actually used, and records
# the project options. A PATH shim tar that rejects -z forces the tar | gzip
# fallback; a shim gzip that fails as well forces the plain .tar fallback. The
# collector puts the system directories first on PATH, so these runs use a
# copy of it whose PATH line puts the shim directory first. A busybox built
# with the standalone shell (busybox-static, as CI installs) runs its own tar
# and gzip applets before searching PATH, so the shims never run; the probe
# below detects that and skips only the checks that need a forced fallback.
DEFAULT_SUMMARY=$(ls "$OUT/default"/*.collection.json)
check "#31 default summary: archiver tar -z, .tar.gz" "grep -q '\"archiver\": \"tar -z\"' \"$DEFAULT_SUMMARY\" && grep -q '\"archive\": \"[^\"]*\\.tar\\.gz\"' \"$DEFAULT_SUMMARY\""
check "#31 default summary: no_projects false, projects empty" "grep -q '\"no_projects\": false, \"projects\": \\[\\]' \"$DEFAULT_SUMMARY\""
SHIM_DIR="$WORK/shim31"; mkdir -p "$SHIM_DIR"
REAL_TAR=$(command -v tar)
# shellcheck disable=SC2016 # $1 and $@ belong to the shim, not to this script
printf '#!/bin/sh\ncase "$1" in *z*) echo "tar: -z not supported" >&2; exit 2 ;; esac\nexec "%s" "$@"\n' "$REAL_TAR" >"$SHIM_DIR/tar"
chmod +x "$SHIM_DIR/tar"
mkdir -p "$WORK/probe31"
printf '#!/bin/sh\necho cac-shim-ran\n' >"$WORK/probe31/tar"
chmod +x "$WORK/probe31/tar"
SHIMS_RUN=no
case $(PATH="$WORK/probe31:$PATH" "$SHELL_UNDER_TEST" -c tar 2>/dev/null) in
  (cac-shim-ran) SHIMS_RUN=yes ;;
esac
[ "$SHIMS_RUN" = yes ] || printf 'skip #31 forced tar | gzip and plain tar fallback checks (the shell runs its own tar applet before PATH)\n'
sed "s|^PATH=\"/usr/bin:|PATH=\"$SHIM_DIR:/usr/bin:|" "$SCRIPT" >"$SHIM_DIR/collector.sh"
check "#31 shimmed collector copy puts the shim first" "grep -q '^PATH=\"$SHIM_DIR:/usr/bin:' \"$SHIM_DIR/collector.sh\""
_saved_script_shim=$SCRIPT; SCRIPT="$SHIM_DIR/collector.sh"
run tgz31 -p "$ROOT/srv/ccwork" -p "$ROOT/srv/cchist"; _exit_shim=$?
check "#31 tar | gzip fallback exit 0" "[ $_exit_shim -eq 0 ] && [ -s \"\$ARCHIVE\" ]"
if [ "$SHIMS_RUN" = yes ]; then
  check "#31 tar | gzip: on-disk summary names tar | gzip" "grep -q '\"archiver\": \"tar | gzip\"' \"\$SUMMARY\" && grep -q \"\\\"archive\\\": \\\"\${ARCHIVE##*/}\\\"\" \"\$SUMMARY\""
fi
check "#31 tar | gzip: archived summary matches on-disk" "tar -xzOf \"\$ARCHIVE\" ./collection.json | cmp -s - \"\$SUMMARY\""
check "#31 -p values recorded in options.projects" "grep -q '\"no_projects\": false, \"projects\": \\[\"$ROOT/srv/ccwork\",\"$ROOT/srv/cchist\"\\]' \"\$SUMMARY\""
printf '#!/bin/sh\ncat >/dev/null; echo "gzip: broken" >&2; exit 1\n' >"$SHIM_DIR/gzip"
chmod +x "$SHIM_DIR/gzip"
run tar31 --no-projects; _exit_shim=$?
SCRIPT=$_saved_script_shim
PLAIN_TAR=$(ls "$OUT/tar31"/*.tar 2>/dev/null)
if [ "$SHIMS_RUN" = yes ]; then
  check "#31 plain tar fallback exit 0, no .tar.gz left" "[ $_exit_shim -eq 0 ] && [ -s \"$PLAIN_TAR\" ] && [ -z \"\$ARCHIVE\" ]"
  check "#31 plain tar: on-disk summary names .tar and tar" "grep -q '\"archiver\": \"tar\"' \"\$SUMMARY\" && grep -q \"\\\"archive\\\": \\\"\${PLAIN_TAR##*/}\\\"\" \"\$SUMMARY\""
  check "#31 plain tar: archived summary matches on-disk" "tar -xOf \"$PLAIN_TAR\" ./collection.json | cmp -s - \"\$SUMMARY\""
  check "#31 plain tar: sidecar names .tar" "grep -q \"  \${PLAIN_TAR##*/}\\\$\" \"$PLAIN_TAR.sha256\""
fi
check "#31 --no-projects recorded" "grep -q '\"no_projects\": true, \"projects\": \\[\\]' \"\$SUMMARY\""
if command -v python3 >/dev/null 2>&1; then
  check "#31 fallback summaries are valid JSON" "python3 -c 'import json,sys; [json.load(open(path)) for path in sys.argv[1:]]' \"\$SUMMARY\" \"$OUT\"/tgz31/*.collection.json"
fi

# 2026-10-05 bug round: #40
# A tar that fails inside tar -cf - | gzip must not leave a truncated .tar.gz
# behind: the collector falls through to the plain .tar. The shim rejects -z,
# writes some bytes and exits 1 for -cf -, and runs the real tar otherwise;
# gzip is the real one. The forced-failure checks reuse the probe above.
check "#40 tar | gzip success leaves no tar status file" "! ls -a \"$OUT/tgz31\" | grep -q 'tar-status'"
if [ "$SHIMS_RUN" = yes ]; then
  FAILING_TAR_SHIM="$WORK/shim40"; mkdir -p "$FAILING_TAR_SHIM"
  # shellcheck disable=SC2016 # $1, $2 and $@ belong to the shim, not to this script
  printf '#!/bin/sh\ncase "$1" in *z*) echo "tar: -z not supported" >&2; exit 2 ;; esac\nif [ "$1" = -cf ] && [ "$2" = - ]; then printf "partial tar bytes"; echo "tar: forced failure" >&2; exit 1; fi\nexec "%s" "$@"\n' "$REAL_TAR" >"$FAILING_TAR_SHIM/tar"
  chmod +x "$FAILING_TAR_SHIM/tar"
  sed "s|^PATH=\"/usr/bin:|PATH=\"$FAILING_TAR_SHIM:/usr/bin:|" "$_saved_script_shim" >"$FAILING_TAR_SHIM/collector.sh"
  SCRIPT="$FAILING_TAR_SHIM/collector.sh"
  run tarfail40; _exit_tar_fail=$?
  SCRIPT=$_saved_script_shim
  FAILED_TAR=$(ls "$OUT/tarfail40"/*.tar 2>/dev/null)
  check "#40 tar failure in the pipe: exit 0, plain .tar, no .tar.gz left" "[ $_exit_tar_fail -eq 0 ] && [ -s \"$FAILED_TAR\" ] && [ -z \"\$ARCHIVE\" ]"
  check "#40 tar failure in the pipe: summary names .tar and tar" "grep -q '\"archiver\": \"tar\"' \"\$SUMMARY\" && grep -q \"\\\"archive\\\": \\\"\${FAILED_TAR##*/}\\\"\" \"\$SUMMARY\""
  check "#40 tar failure in the pipe: archived summary matches on-disk" "tar -xOf \"$FAILED_TAR\" ./collection.json | cmp -s - \"\$SUMMARY\""
  check "#40 tar failure in the pipe: logged" "grep -q 'tar | gzip: tar exited 1\$' \"$OUT\"/tarfail40/*.log"
  check "#40 tar status file neither left nor archived" "! ls -a \"$OUT/tarfail40\" | grep -q 'tar-status' && ! tar -tf \"$FAILED_TAR\" | grep -q 'tar-status'"
else
  printf 'skip #40 forced tar failure in tar | gzip checks (the shell runs its own tar applet before PATH)\n'
fi

# 2026-10-04 bug round: #32, #38, #39
# A relative -p is resolved against the current directory (image mode too); a
# -p that does not exist gets an error_read row and a log line. The output
# directory check is the existing [ -w ]: a read-only -o exits 2.
mkdir -p "$ROOT/srv/relproj"
printf '# rel\n' >"$ROOT/srv/relproj/AGENTS.md"
_cwd=$(pwd)
cd "$ROOT/srv" || exit 1
run projargs -p relproj -p /nonexistent-cac/proj; _exit_code=$?
cd "$_cwd" || exit 1
check "-p arguments exit 0" "[ $_exit_code -eq 0 ]"
check "relative -p resolved against the current directory" "[ \"\$(status_of \"$ROOT/srv/relproj/AGENTS.md\")\" = collected ] && grep -q '\"projects\": \\[.*\"$ROOT/srv/relproj\"' \"\$SUMMARY\""
check "#31 x #39 options.projects records the resolved -p values" "grep -q '\"no_projects\": false, \"projects\": \\[\"$ROOT/srv/relproj\",\"/nonexistent-cac/proj\"\\]' \"\$SUMMARY\""
check "missing -p gets one error_read dir row" "[ \"\$(grep -c '\"path\":\"/nonexistent-cac/proj\"' \"\$MANIFEST\")\" = 1 ] && row /nonexistent-cac/proj | grep -q '\"home\":\"/nonexistent-cac/proj\",\"agent\":\"\",.*\"archive_path\":\"\",\"type\":\"dir\".*\"status\":\"error_read\",\"target\":\"\",\"error\":\"[^\"]*No such file or directory\"'"
check "missing -p logged" "grep -q 'WARNING: project given with -p is not a directory: /nonexistent-cac/proj: ' \"$OUT\"/projargs/*.log"
check "missing -p not listed in projects" "grep -q '^  \"projects\": \\[\"' \"\$SUMMARY\" && ! grep '^  \"projects\"' \"\$SUMMARY\" | grep -q nonexistent-cac"
check "missing -p counted as error_read" "grep -q '\"error_read\": 1,' \"\$SUMMARY\" && grep -q '^errors:     1 (0 error_copy, 1 error_read)\$' \"$OUT/projargs.stdout\""
check "default run has no error_read rows" "grep -q '\"error_read\": 0,' \"$OUT\"/default/*.collection.json && grep -q '^errors:     0 (0 error_copy, 0 error_read)\$' \"$OUT/default.stdout\""
rm -rf "$ROOT/srv/relproj"
if [ "$(id -u)" != 0 ]; then
  mkdir -p "$OUT/readonly"
  chmod 555 "$OUT/readonly"
  COLLECTOR_SH="$SHELL_UNDER_TEST" "$SHELL_UNDER_TEST" "$SCRIPT" -r "$ROOT" -o "$OUT/readonly" -q --no-docker >/dev/null 2>"$OUT/readonly.stderr"
  check "read-only output dir exits 2 before collecting" "[ $? -eq 2 ] && grep -q '^Output dir not writable: ' \"$OUT/readonly.stderr\" && [ -z \"\$(ls -A \"$OUT/readonly\")\" ]"
  chmod 755 "$OUT/readonly"
  # an unreadable home, an unlistable directory and one that cannot be entered
  mkdir -p "$ROOT/home/erin/.claude" "$ALICE/.claude/locked" "$ALICE/.claude/noenter"
  printf 'x\n' >"$ALICE/.claude/locked/f"
  chmod 000 "$ROOT/home/erin" "$ALICE/.claude/locked"
  chmod 600 "$ALICE/.claude/noenter"
  run readperm; _exit_code=$?
  chmod 700 "$ROOT/home/erin" "$ALICE/.claude/locked" "$ALICE/.claude/noenter"
  check "unreadable paths exit 0" "[ $_exit_code -eq 0 ]"
  check "unreadable home gets one error_read row" "[ \"\$(grep -c '\"path\":\"$ROOT/home/erin\"' \"\$MANIFEST\")\" = 1 ] && row \"$ROOT/home/erin\" | grep -q '\"user\":\"erin\",\"home\":\"$ROOT/home/erin\",\"agent\":\"\",.*\"archive_path\":\"\",\"type\":\"dir\",\"size\":0,.*\"status\":\"error_read\",\"target\":\"\",\"error\":\"[^\"]*Permission denied\"'"
  check "unreadable home stays in users, not in users_with_artifacts" "tr -d '\n' <\"\$SUMMARY\" | grep -q '\"users\": \\[[^]]*\"erin\"' && ! grep '\"users_with_artifacts\"' \"\$SUMMARY\" | grep -q erin"
  check "unlistable directory in a walk gets an error_read row" "row \"$ALICE/.claude/locked\" | grep -q '\"user\":\"alice\",\"home\":\"$ALICE\",\"agent\":\"claude-code\",.*\"type\":\"dir\",.*\"status\":\"error_read\",.*\"error\":\"[^\"]*Permission denied\"'"
  check "directory that cannot be entered gets an error_read row" "[ \"\$(status_of \"$ALICE/.claude/noenter\")\" = error_read ]"
  check "rest of the home still collected" "[ \"\$(status_of \"$ROOT/home/alice/.claude/projects/-srv-proj/s1.jsonl\")\" = collected ]"
  check "unreadable paths counted" "grep -q '\"error_read\": 3,' \"\$SUMMARY\" && grep -q '^errors:     3 (0 error_copy, 3 error_read)\$' \"$OUT/readperm.stdout\""
  check "unreadable paths logged" "[ \"\$(grep -c ' read failed: ' \"$OUT\"/readperm/*.log)\" = 3 ]"
  rm -rf "$ROOT/home/erin" "$ALICE/.claude/locked" "$ALICE/.claude/noenter"
else
  printf 'skip unreadable home, directory and output dir checks (running as root)\n'
fi
# 2026-10-14 output directory under a file (#108)
COLLECTOR_SH="$SHELL_UNDER_TEST" "$SHELL_UNDER_TEST" "$SCRIPT" -r "$ROOT" -o "$PLAIN_FILE/out" -q --no-docker >"$OUT/file108.stdout" 2>"$OUT/file108.stderr"
check "#108 output dir under a file exits 2 with only the create error" "[ $? -eq 2 ] && [ \"\$(cat \"$OUT/file108.stderr\")\" = \"Cannot create output dir: $PLAIN_FILE/out\" ] && [ ! -s \"$OUT/file108.stdout\" ] && [ -f \"$PLAIN_FILE\" ]"

# 2026-10-14 exclusion globs and the working directory (#109)
(cd "$WORK/cwd109" && run cwd109 --no-docker --no-projects); _exit_glob_cwd=$?
MANIFEST_GLOB_CWD=$(ls "$OUT/cwd109"/*.manifest.jsonl 2>/dev/null)
check "#109 wildcard exclusion holds from a directory with a matching tree" "[ $_exit_glob_cwd -eq 0 ] && grep -F '\"path\":\"$ALICE/.claude/plugins/y/node_modules\"' \"$MANIFEST_GLOB_CWD\" | grep -q '\"status\":\"skipped_excluded\"' && ! grep -q 'node_modules/f.js' \"$MANIFEST_GLOB_CWD\""

# 2026-10-14 CDPATH in the environment (#110)
# sh only: a relative -r with CDPATH naming the directory that holds the decoy
# root, run from the directory that holds the image root.
(cd "$WORK" && CDPATH="$WORK/cdp110" COLLECTOR_SH="$SHELL_UNDER_TEST" "$SHELL_UNDER_TEST" "$SCRIPT" -r root -o "$OUT/cdpath110" -q --no-docker --no-projects >"$OUT/cdpath110.stdout" 2>"$OUT/cdpath110.stderr"); _exit_cdpath=$?
check "#110 relative -r with CDPATH set collects the image in the working directory" "[ $_exit_cdpath -eq 0 ] && grep -q '^  \"root\": \"$ROOT\",\$' \"$OUT\"/cdpath110/*.collection.json && grep -q '\"user\":\"alice\"' \"$OUT\"/cdpath110/*.manifest.jsonl"

# 2026-10-04 bug round: #29 x #32
# An unreadable catalog match gets one error_read row, and a case variant
# folded into it (case-insensitive filesystem) gets none of its own.
if [ "$(id -u)" != 0 ]; then
  chmod 000 "$ALICE/.config/pearai/User"
  run fold32; _exit_code=$?
  chmod 755 "$ALICE/.config/pearai/User"
  check "#29 x #32 unreadable case-variant match: exactly one error_read row" "[ $_exit_code -eq 0 ] && [ \"\$(grep '\"status\":\"error_read\"' \"\$MANIFEST\" | grep -ciF '/.config/pearai/User\"')\" = 1 ] && grep -q '\"error_read\": 1,' \"\$SUMMARY\""
  if [ "$CASE_INSENSITIVE" = 1 ]; then
    check "#29 x #32 folded variant has no rows of its own" "! grep -qF '\"path\":\"$ALICE/.config/pearai/User' \"\$MANIFEST\""
  else
    check "#29 x #32 the readable spelling is still collected" "[ \"\$(status_of \"$ALICE/.config/PearAI/User/case29-upper.json\")\" = collected ]"
  fi
else
  printf 'skip #29 x #32 unreadable case-variant check (running as root)\n'
fi

# 2026-10-07 batched stage worker (#71)
# The stage worker lstats, copies and hashes a chunk of files with one stat,
# one cp per directory and one hasher call. A separate image holds one
# directory with names the hashers escape (newline, backslash, carriage
# return), an unreadable file between readable ones, and 300 files, more than
# one 256-file chunk. CAC_NO_BATCH=1 takes the per-file path; the two
# manifests must match apart from atime, which the first run's reads may move.
# The shasum run uses a copy of the collector whose sha256sum probe looks for
# a name that does not exist, the same sed-copy approach as the tar shims; a
# PATH shim cannot hide sha256sum, because the collector puts /usr/bin first.
BATCH_ROOT="$WORK/root71"; BATCH_DIR="$BATCH_ROOT/home/carol/.claude/projects/-srv-p71"; NEWLINE='
'
CARRIAGE_RETURN=$(printf '\r')
mkdir -p "$BATCH_ROOT/etc" "$BATCH_DIR" "$BATCH_ROOT/home/carol/.claude/file-history/s1"
printf 'carol:x:1002:1002::/home/carol:/bin/sh\n' >"$BATCH_ROOT/etc/passwd"
printf 'first\n' >"$BATCH_DIR/a.jsonl"
printf 'newline name\n' >"$BATCH_DIR/nl${NEWLINE}name.jsonl"
printf 'backslash name\n' >"$BATCH_DIR/back\\slash.jsonl"
printf 'cr name\n' >"$BATCH_DIR/cr${CARRIAGE_RETURN}name.jsonl"
printf 'locked\n' >"$BATCH_DIR/m-locked.jsonl"
printf 'last\n' >"$BATCH_DIR/z.jsonl"
_index=0
while [ "$_index" -lt 300 ]; do printf 'v%s\n' "$_index" >"$BATCH_ROOT/home/carol/.claude/file-history/s1/f$_index@v1"; _index=$((_index + 1)); done
BATCH_LOCKED=no
if [ "$(id -u)" != 0 ]; then chmod 000 "$BATCH_DIR/m-locked.jsonl"; BATCH_LOCKED=yes; fi
# shellcheck disable=SC2317,SC2329 # called from the eval'd check strings
hash_of_text() { printf '%b' "$1" | sha256sum | cut -d' ' -f1; }
# shellcheck disable=SC2317,SC2329
sha256_of_row() { row "$1" | sed 's/.*"sha256":"\([^"]*\)".*/\1/'; }
# shellcheck disable=SC2317,SC2329
without_atime() { sed 's/"atime":[0-9]*,//' "$1" | sort; }
_saved_root_batch=$ROOT; ROOT=$BATCH_ROOT
run batch71 --no-docker; _exit_batch=$?; BATCH_MANIFEST=$MANIFEST
# bsdtar reads a backslash in a member name as an escape, so the backslash
# file is hashed from a full extraction rather than with tar -O.
BATCH_EXTRACT="$WORK/x71"; mkdir -p "$BATCH_EXTRACT"; tar -xzf "$ARCHIVE" -C "$BATCH_EXTRACT" 2>/dev/null
# shellcheck disable=SC2034 # read by the eval'd check expression below
BACKSLASH_FILE="$BATCH_EXTRACT/fs/home/carol/.claude/projects/-srv-p71/back\\slash.jsonl"
check "#71 batched run exit 0, 306 rows" "[ $_exit_batch -eq 0 ] && [ \"\$(wc -l <\"\$MANIFEST\")\" -eq 306 ]"
check "#71 plain name in the batch hashed" "[ \"\$(sha256_of_row \"$BATCH_DIR/a.jsonl\")\" = \"\$(hash_of_text 'first\\n')\" ]"
check "#71 newline name hashed" "[ \"\$(sha256_of_row \"$BATCH_DIR/nl\\\\nname.jsonl\")\" = \"\$(hash_of_text 'newline name\\n')\" ]"
check "#71 newline name: hash matches the archived bytes" "[ \"\$(tar -xzOf \"\$ARCHIVE\" \"./fs/home/carol/.claude/projects/-srv-p71/nl${NEWLINE}name.jsonl\" | sha256sum | cut -d' ' -f1)\" = \"\$(sha256_of_row \"$BATCH_DIR/nl\\\\nname.jsonl\")\" ]"
check "#71 backslash name hashed" "[ \"\$(sha256_of_row \"$BATCH_DIR/back\\\\\\\\slash.jsonl\")\" = \"\$(hash_of_text 'backslash name\\n')\" ]"
check "#71 backslash name: hash matches the archived bytes" "[ \"\$(sha256sum <\"\$BACKSLASH_FILE\" | cut -d' ' -f1)\" = \"\$(sha256_of_row \"$BATCH_DIR/back\\\\\\\\slash.jsonl\")\" ]"
check "#71 carriage return name hashed" "[ \"\$(sha256_of_row \"$BATCH_DIR/cr\\\\rname.jsonl\")\" = \"\$(hash_of_text 'cr name\\n')\" ]"
check "#71 file after the odd names hashed" "[ \"\$(sha256_of_row \"$BATCH_DIR/z.jsonl\")\" = \"\$(hash_of_text 'last\\n')\" ]"
check "#71 files across two chunks all collected and hashed" "[ \"\$(grep -c '/file-history/s1/f[0-9]*@v1\",.*\"sha256\":\"[0-9a-f]\\{64\\}\",\"secret\":false,\"status\":\"collected\"' \"\$MANIFEST\")\" -eq 300 ] && [ \"\$(sha256_of_row \"$BATCH_ROOT/home/carol/.claude/file-history/s1/f299@v1\")\" = \"\$(hash_of_text 'v299\\n')\" ]"
if [ "$BATCH_LOCKED" = yes ]; then
  check "#71 unreadable file in the batch: error_copy row with its error" "row \"$BATCH_DIR/m-locked.jsonl\" | grep -q '\"archive_path\":\"\",\"type\":\"file\",\"size\":7,.*\"sha256\":\"\",\"secret\":false,\"status\":\"error_copy\",\"target\":\"\",\"error\":\"[^\"]*Permission denied\"'"
  check "#71 unreadable file in the batch: logged once" "[ \"\$(grep -c 'copy failed: .*/m-locked.jsonl: ' \"$OUT\"/batch71/*.log)\" = 1 ]"
  check "#71 unreadable file in the batch: the rest collected" "[ \"\$(grep -c '\"status\":\"collected\"' \"\$MANIFEST\")\" -eq 305 ] && ! tar -tzf \"\$ARCHIVE\" | grep -q m-locked"
else
  printf 'skip #71 unreadable file in a batch (running as root)\n'
fi
export CAC_NO_BATCH=1
run nobatch71 --no-docker; _exit_batch=$?
unset CAC_NO_BATCH
check "#71 per-file run exit 0" "[ $_exit_batch -eq 0 ]"
check "#71 batched and per-file manifests match" "without_atime \"\$MANIFEST\" >\"$WORK/nb71.txt\" && without_atime \"$BATCH_MANIFEST\" | cmp -s - \"$WORK/nb71.txt\""
check "#71 batched and per-file manifests in the same order" "sed 's/\"atime\":[0-9]*,//' \"\$MANIFEST\" >\"$WORK/nb71o.txt\" && sed 's/\"atime\":[0-9]*,//' \"$BATCH_MANIFEST\" | cmp -s - \"$WORK/nb71o.txt\""
if shasum -a 256 /dev/null >/dev/null 2>&1; then
  mkdir -p "$WORK/shasum71"
  sed 's/command -v sha256sum /command -v cac-no-sha256sum /' "$SCRIPT" >"$WORK/shasum71/collector.sh"
  _saved_script_shasum=$SCRIPT; SCRIPT="$WORK/shasum71/collector.sh"
  run shasum71 --no-docker; _exit_batch=$?
  SCRIPT=$_saved_script_shasum
  check "#71 shasum run: exit 0, hash_tool shasum" "[ $_exit_batch -eq 0 ] && grep -q '\"hash_tool\": \"shasum\"' \"\$SUMMARY\""
  check "#71 shasum run: manifest matches the sha256sum run" "without_atime \"\$MANIFEST\" | cmp -s - \"$WORK/nb71.txt\""
else
  printf 'skip #71 shasum path (no shasum)\n'
fi
ROOT=$_saved_root_batch
[ "$BATCH_LOCKED" = no ] || chmod 600 "$BATCH_DIR/m-locked.jsonl"

# 2026-10-10 Claude Desktop MSIX build (#73)
# The MSIX build keeps userData in the package's virtualised
# LocalCache/Roaming/Claude, logs in AppData/Local/Claude/logs, and may use
# AppData/Local/Claude-Data/<family>. A separate image holds one Windows
# profile with those trees; the */Claude*/ secret and exclusion globs must
# apply under the new bases, and the Packages secret globs must flag the
# token cache and cookies of both package families.
MSIX_ROOT="$WORK/root73"; MSIX_PACKAGE="$MSIX_ROOT/Users/bob/AppData/Local/Packages/Claude_pzs8sxrjxfjjc/LocalCache/Roaming/Claude"
MSIX_SPACE_IDS="11111111-1111-4111-8111-111111111111/22222222-2222-4222-8222-222222222222"
MSIX_PBC_PACKAGE="$MSIX_ROOT/Users/bob/AppData/Local/Packages/AnthropicPBC.Claude_fnn82j28hfe8t/LocalCache/Roaming/Claude"
MSIX_LOGS="$MSIX_ROOT/Users/bob/AppData/Local/Claude"; MSIX_DATA="$MSIX_ROOT/Users/bob/AppData/Local/Claude-Data/claude_pzs8sxrjxfjjc"
mkdir -p "$MSIX_PACKAGE/claude-code-sessions/$MSIX_SPACE_IDS" "$MSIX_PACKAGE/local-agent-mode-sessions/$MSIX_SPACE_IDS/local_33333333-3333-4333-8333-333333333333" \
  "$MSIX_PACKAGE/claude-code/2.1.295/abc" "$MSIX_PACKAGE/Cache" "$MSIX_PACKAGE/Network" "$MSIX_PBC_PACKAGE" "$MSIX_LOGS/logs" "$MSIX_DATA"
printf '{"oauth:tokenCache":"enc"}\n' >"$MSIX_PACKAGE/config.json"
printf '{"os_crypt":{}}\n' >"$MSIX_PACKAGE/Local State"
printf '{"mcpServers":{}}\n' >"$MSIX_PACKAGE/claude_desktop_config.json"
printf 'cookies\n' >"$MSIX_PACKAGE/Network/Cookies"
printf '{"sessionId":"local_44444444-4444-4444-8444-444444444444","cwd":"C:\\\\proj"}\n' >"$MSIX_PACKAGE/claude-code-sessions/$MSIX_SPACE_IDS/local_44444444-4444-4444-8444-444444444444.json"
printf '{"entries":[{"sessionId":"0123456789abcdef0123456789abcdef","folders":[]}]}\n' >"$MSIX_PACKAGE/local-agent-mode-sessions/$MSIX_SPACE_IDS/remote-session-spaces.json"
printf 'key\n' >"$MSIX_PACKAGE/local-agent-mode-sessions/$MSIX_SPACE_IDS/local_33333333-3333-4333-8333-333333333333/.audit-key"
printf 'MZ\n' >"$MSIX_PACKAGE/claude-code/2.1.295/abc/claude.exe"
printf 'cache\n' >"$MSIX_PACKAGE/Cache/data_0"
printf '{"oauth:tokenCache":"enc"}\n' >"$MSIX_PBC_PACKAGE/config.json"
printf '[MSIX] Filesystem virtualization active\n' >"$MSIX_LOGS/logs/main.log"
printf '{"oauth:tokenCache":"enc"}\n' >"$MSIX_DATA/config.json"
_saved_root_msix=$ROOT; ROOT=$MSIX_ROOT
run store73 --no-docker; _exit_msix=$?
check "#73 MSIX build run exit 0" "[ $_exit_msix -eq 0 ]"
check "#73 Packages Code-tab session record collected as claude-desktop" "row \"$MSIX_PACKAGE/claude-code-sessions/$MSIX_SPACE_IDS/local_44444444-4444-4444-8444-444444444444.json\" | grep -q '\"agent\":\"claude-desktop\".*\"secret\":false,\"status\":\"collected\"'"
check "#73 Packages session record collected exactly once" "[ \"\$(grep -c 'local_44444444-4444-4444-8444-444444444444.json' \"\$MANIFEST\")\" = 1 ]"
check "#73 Packages remote-session-spaces.json collected" "row \"$MSIX_PACKAGE/local-agent-mode-sessions/$MSIX_SPACE_IDS/remote-session-spaces.json\" | grep -q '\"agent\":\"claude-desktop\".*\"secret\":false,\"status\":\"collected\"'"
check "#73 AppData/Local/Claude logs collected as claude-desktop" "row \"$MSIX_LOGS/logs/main.log\" | grep -q '\"agent\":\"claude-desktop\".*\"status\":\"collected\"'"
check "#73 Packages config.json flagged secret (Packages/Claude_* glob)" "row \"$MSIX_PACKAGE/config.json\" | grep -q '\"agent\":\"claude-desktop\".*\"secret\":true,\"status\":\"collected\"'"
check "#73 Packages Network/Cookies flagged secret (Packages/Claude_* glob)" "row \"$MSIX_PACKAGE/Network/Cookies\" | grep -q '\"secret\":true,\"status\":\"collected\"'"
check "#73 AnthropicPBC family config.json flagged secret" "row \"$MSIX_PBC_PACKAGE/config.json\" | grep -q '\"agent\":\"claude-desktop\".*\"secret\":true,\"status\":\"collected\"'"
check "#73 Claude-Data config.json flagged secret (AppData/Local/Claude* glob)" "row \"$MSIX_DATA/config.json\" | grep -q '\"agent\":\"claude-desktop\".*\"secret\":true,\"status\":\"collected\"'"
check "#73 Packages Local State flagged secret (*/Claude*/Local State)" "row \"$MSIX_PACKAGE/Local State\" | grep -q '\"secret\":true,\"status\":\"collected\"'"
check "#73 Packages claude_desktop_config.json flagged secret (*/Claude*/claude_desktop_config.json)" "row \"$MSIX_PACKAGE/claude_desktop_config.json\" | grep -q '\"secret\":true,\"status\":\"collected\"'"
check "#73 Packages .audit-key flagged secret (*/Claude*/local-agent-mode-sessions/*/.audit-key)" "row \"$MSIX_PACKAGE/local-agent-mode-sessions/$MSIX_SPACE_IDS/local_33333333-3333-4333-8333-333333333333/.audit-key\" | grep -q '\"secret\":true,\"status\":\"collected\"'"
check "#73 Packages claude-code excluded (*/Claude*/claude-code)" "[ \"\$(status_of \"$MSIX_PACKAGE/claude-code\")\" = skipped_excluded ] && ! grep -q 'claude.exe' \"\$MANIFEST\""
check "#73 Packages Cache excluded (*/Claude*/Cache)" "[ \"\$(status_of \"$MSIX_PACKAGE/Cache\")\" = skipped_excluded ] && ! grep -q 'Cache/data_0' \"\$MANIFEST\""
run nosecrets73 --no-docker --no-secrets; _exit_msix=$?
check "#73 --no-secrets run exit 0" "[ $_exit_msix -eq 0 ]"
check "#73 --no-secrets skips the Packages and Claude-Data token caches" "[ \"\$(status_of \"$MSIX_PACKAGE/config.json\")\" = skipped_secret ] && [ \"\$(status_of \"$MSIX_PBC_PACKAGE/config.json\")\" = skipped_secret ] && [ \"\$(status_of \"$MSIX_DATA/config.json\")\" = skipped_secret ]"
check "#73 --no-secrets skips Local State, claude_desktop_config.json, .audit-key" "[ \"\$(status_of \"$MSIX_PACKAGE/Local State\")\" = skipped_secret ] && [ \"\$(status_of \"$MSIX_PACKAGE/claude_desktop_config.json\")\" = skipped_secret ] && [ \"\$(status_of \"$MSIX_PACKAGE/local-agent-mode-sessions/$MSIX_SPACE_IDS/local_33333333-3333-4333-8333-333333333333/.audit-key\")\" = skipped_secret ]"
check "#73 --no-secrets still collects the session record" "[ \"\$(status_of \"$MSIX_PACKAGE/claude-code-sessions/$MSIX_SPACE_IDS/local_44444444-4444-4444-8444-444444444444.json\")\" = collected ]"
run full73 --no-docker --full; _exit_msix=$?
check "#73 --full run exit 0" "[ $_exit_msix -eq 0 ]"
check "#73 --full collects the Packages claude.exe and Cache" "[ \"\$(status_of \"$MSIX_PACKAGE/claude-code/2.1.295/abc/claude.exe\")\" = collected ] && [ \"\$(status_of \"$MSIX_PACKAGE/Cache/data_0\")\" = collected ]"
ROOT=$_saved_root_msix

if [ "$fail" = 0 ]; then printf 'ALL PASSED (%s)\n' "$SHELL_UNDER_TEST"; rm -rf "$WORK"; else printf 'FAILURES (%s); work dir kept: %s\n' "$SHELL_UNDER_TEST" "$WORK"; fi
exit "$fail"
