//go:build parity

package parity

import (
	"strings"
	"testing"
)

// volumeFile is a file the framework volumes case writes, with its path
// below the image's Docker volumes directory.
type volumeFile struct {
	path    string
	content string
}

// frameworkVolumeFiles are the Dify, Langflow, n8n, Hermes, OpenHands and
// Ollama volume files the smoke test's frameworks sweep builds, besides the
// Hermes list.
var frameworkVolumeFiles = []volumeFile{
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
}

// setupFrameworkVolumes builds the Dify, Langflow, n8n, Hermes, OpenHands and
// Ollama volumes, in a run of their own so the other cases' counts hold.
func setupFrameworkVolumes(t *testing.T, img image) {
	build := fixture{t}
	volumes := img.root + "/var/lib/docker/volumes"
	build.dir(volumes+"/n8n_n8n-data/_data", volumes+"/docker_dify_agent_local_sandbox_workspace/_data")
	for _, file := range frameworkVolumeFiles {
		build.file(join(volumes, file.path), file.content)
	}
	hermes := volumes + "/hermes_home/_data"
	for _, line := range nonEmptyLines(hermesVolumeCases) {
		kind, path, _ := strings.Cut(line, "|")
		if kind == "excl" {
			build.file(join(hermes, path, "f"), "x\n")
		} else {
			build.file(join(hermes, path), "x\n")
		}
	}
}
