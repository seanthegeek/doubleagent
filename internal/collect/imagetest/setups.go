package imagetest

import (
	"strings"
	"testing"
)

// Fixture builders both platforms share. Each takes the test it builds for
// and removes what it made, or puts back what it changed, when that test
// ends.

// volumeFile is a file the framework volumes case writes, with its path
// below the image's Docker volumes directory.
type volumeFile struct {
	path    string
	content string
}

// frameworkVolumeFiles returns the Dify, Langflow, n8n, Hermes, OpenHands and
// Ollama volume files of the frameworks sweep, besides the Hermes list.
func frameworkVolumeFiles() []volumeFile {
	return []volumeFile{
		{"n8n_data/_data/config", "{\"encryptionKey\":\"k\"}\n"},
		{"n8n_data/_data/database.sqlite", "db\n"},
		{"n8n_data/_data/nodes/node_modules/pkg/index.js", "x\n"},
		{"n8n_data/_data/node-definitions/d.json", "x\n"},
		{"n8n_data/_data/n8n-sdk-templates/t.json", "x\n"},
		{"n8n_data/_data/ssh/key", "key\n"},
		{"n8n_n8n-data/_data/database.sqlite", "db\n"},
		{"langflow_langflow-data/_data/secret_key", "k\n"},
		{"langflow_langflow-data/_data/cache_secret_key", "k\n"},
		{"langflow_langflow-data/_data/profile_pictures/p.svg", "x\n"},
		{"docker_dify_agent_local_sandbox_home/_data/.local/share/shellctl/shellctl.db", "db\n"},
		{"docker_dify_agent_local_sandbox_workspace/_data/out.txt", "x\n"},
		{"hermes_home/_data/state-snapshots/20261001-120004/state.db", "sqlite\n"},
		{"hermes_home/_data/state-snapshots/20261001-120004/.env", "KEY=x\n"},
		{"hermes_home/_data/state-snapshots/20261001-120004/auth.json", "{\"providers\":{}}\n"},
		{"hermes_home/_data/state-snapshots/20261001-120004/pairing/telegram-approved.json", "{}\n"},
		{"langflow_langflow-data/_data/0f0e0d0c-1111-4000-8000-000000000248/" +
			"_mcp_servers_0f0e0d0c-1111-4000-8000-000000000248.json", "{\"mcpServers\":{}}\n"},
		{"hermes_home/_data/config.yaml", "mcp_servers: {}\n"},
		{"openhands_state/_data/conversations/c1/acp/codex/auth.json", "{\"tokens\":{}}\n"},
		{"openhands_state/_data/agent-canvas/conversations/c2/acp/codex/auth.json", "{\"tokens\":{}}\n"},
		{"openhands_state/_data/profiles/p1.json", "{\"model\":\"x\",\"api_key\":\"k\"}\n"},
		{"openhands_state/_data/profiles/tmpabc123.tmp", "{\"model\":\"x\",\"api_key\":\"k\"}\n"},
		{"hermes_home/_data/profiles/work/pastes/pending.json", "[]\n"},
		{"ollama/_data/backup/codex/config.toml.1792800000", "model = \"x\"\n"},
		{"ollama/_data/launch/muse-config/muse/settings.json", "{}\n"},
		{"local-deep-research_ollama_data/_data/history", "hello\n"},
	}
}

// FrameworkVolumes builds the Dify, Langflow, n8n, Hermes, OpenHands and
// Ollama Docker volumes (the fwvolumes cases), in a run of their own so the
// other cases' counts hold.
func (img Image) FrameworkVolumes(test testing.TB) {
	build := newFixture(test, img.Work)
	defer build.done()
	volumes := join(img.Root, "var/lib/docker/volumes")
	build.dir(join(volumes, "n8n_n8n-data/_data"), join(volumes, "docker_dify_agent_local_sandbox_workspace/_data"))
	for _, file := range frameworkVolumeFiles() {
		build.file(join(volumes, file.path), file.content)
	}
	hermes := join(volumes, "hermes_home/_data")
	for _, line := range strings.Split(strings.TrimSpace(hermesVolumeCases), "\n") {
		kind, path, _ := strings.Cut(line, "|")
		if kind == "excl" {
			build.file(join(hermes, path, "f"), "x\n")
		} else {
			build.file(join(hermes, path), "x\n")
		}
	}
}

// PatternRoot builds an image root whose name holds "[" (the root486 case),
// which finds its homes through home/* with no passwd file, and returns it.
func (img Image) PatternRoot(test testing.TB) string {
	build := newFixture(test, img.Work)
	defer build.done()
	root := join(img.Work, "r[1]")
	build.file(join(root, "home/carol/.gemini/projects.json"), "{}\n")
	return root
}

// hermesVolumeCases lists one path below the Hermes volume's data directory
// for each volume-relative exclusion and secret glob, as kind|path: excl is a
// directory holding one file, anything else a file.
const hermesVolumeCases = `
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
`

// MsixRoot builds an image root holding one Windows profile with the Claude
// Desktop MSIX build's virtualised data, its logs and its Claude-Data
// directory (the store73, nosecrets73 and full73 cases), and returns it.
func (img Image) MsixRoot(test testing.TB) string {
	build := newFixture(test, img.Work)
	defer build.done()
	root := join(img.Work, "root73")
	spaces := "11111111-1111-4111-8111-111111111111/22222222-2222-4222-8222-222222222222"
	local := join(root, "Users/bob/AppData/Local")
	claude := join(local, "Packages/Claude_pzs8sxrjxfjjc/LocalCache/Roaming/Claude")
	sessions := join(claude, "local-agent-mode-sessions", spaces)
	other := join(local, "Packages/AnthropicPBC.Claude_fnn82j28hfe8t/LocalCache/Roaming/Claude")
	build.dir(join(claude, "claude-code/2.1.295/abc"), join(claude, "Cache"), join(claude, "Network"),
		join(sessions, "local_33333333-3333-4333-8333-333333333333"), join(local, "Claude/logs"),
		join(local, "Claude-Data/claude_pzs8sxrjxfjjc"), other, join(claude, "claude-code-sessions", spaces))
	for path, content := range map[string]string{
		join(claude, "config.json"):                "{\"oauth:tokenCache\":\"enc\"}\n",
		join(claude, "Local State"):                "{\"os_crypt\":{}}\n",
		join(claude, "claude_desktop_config.json"): "{\"mcpServers\":{}}\n",
		join(claude, "Network/Cookies"):            "cookies\n",
		join(claude, "claude-code-sessions", spaces, "local_44444444-4444-4444-8444-444444444444.json"): "{\"sessionId\":" +
			"\"local_44444444-4444-4444-8444-444444444444\",\"cwd\":\"C:\\\\proj\"}\n",
		join(sessions, "remote-session-spaces.json"): "{\"entries\":[{\"sessionId\":" +
			"\"0123456789abcdef0123456789abcdef\",\"folders\":[]}]}\n",
		join(sessions, "local_33333333-3333-4333-8333-333333333333/.audit-key"): "key\n",
		join(claude, "claude-code/2.1.295/abc/claude.exe"):                      "MZ\n",
		join(claude, "Cache/data_0"):                                            "cache\n",
		join(other, "config.json"):                                              "{\"oauth:tokenCache\":\"enc\"}\n",
		join(local, "Claude/logs/main.log"):                                     "[MSIX] Filesystem virtualization active\n",
		join(local, "Claude-Data/claude_pzs8sxrjxfjjc/config.json"):             "{\"oauth:tokenCache\":\"enc\"}\n",
	} {
		build.write(path, content)
	}
	return root
}
