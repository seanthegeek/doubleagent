//go:build parity

package parity

import "testing"

// Fixture builders both parity tables share.

// setupPatternRoot builds an image root whose name holds "[", which finds its
// homes through home/* with no passwd file.
func setupPatternRoot(t *testing.T, img image) {
	fixture{t}.file(img.work+"/r[1]/home/carol/.gemini/projects.json", "{}\n")
}

// setupMsixRoot builds one Windows profile holding the Claude Desktop MSIX
// build's virtualised data, its logs and its Claude-Data directory.
func setupMsixRoot(t *testing.T, img image) {
	build := fixture{t}
	root := img.work + "/root73"
	spaces := "11111111-1111-4111-8111-111111111111/22222222-2222-4222-8222-222222222222"
	local := root + "/Users/bob/AppData/Local"
	pkg := local + "/Packages/Claude_pzs8sxrjxfjjc/LocalCache/Roaming/Claude"
	sessions := pkg + "/local-agent-mode-sessions/" + spaces
	build.dir(pkg+"/claude-code/2.1.295/abc", pkg+"/Cache", pkg+"/Network", sessions+"/local_33333333-3333-4333-8333-333333333333",
		local+"/Claude/logs", local+"/Claude-Data/claude_pzs8sxrjxfjjc",
		local+"/Packages/AnthropicPBC.Claude_fnn82j28hfe8t/LocalCache/Roaming/Claude")
	build.write(pkg+"/config.json", "{\"oauth:tokenCache\":\"enc\"}\n")
	build.write(pkg+"/Local State", "{\"os_crypt\":{}}\n")
	build.write(pkg+"/claude_desktop_config.json", "{\"mcpServers\":{}}\n")
	build.write(pkg+"/Network/Cookies", "cookies\n")
	build.dir(pkg + "/claude-code-sessions/" + spaces)
	build.write(pkg+"/claude-code-sessions/"+spaces+"/local_44444444-4444-4444-8444-444444444444.json",
		"{\"sessionId\":\"local_44444444-4444-4444-8444-444444444444\",\"cwd\":\"C:\\\\proj\"}\n")
	build.write(sessions+"/remote-session-spaces.json",
		"{\"entries\":[{\"sessionId\":\"0123456789abcdef0123456789abcdef\",\"folders\":[]}]}\n")
	build.write(sessions+"/local_33333333-3333-4333-8333-333333333333/.audit-key", "key\n")
	build.write(pkg+"/claude-code/2.1.295/abc/claude.exe", "MZ\n")
	build.write(pkg+"/Cache/data_0", "cache\n")
	build.write(local+"/Packages/AnthropicPBC.Claude_fnn82j28hfe8t/LocalCache/Roaming/Claude/config.json",
		"{\"oauth:tokenCache\":\"enc\"}\n")
	build.write(local+"/Claude/logs/main.log", "[MSIX] Filesystem virtualization active\n")
	build.write(local+"/Claude-Data/claude_pzs8sxrjxfjjc/config.json", "{\"oauth:tokenCache\":\"enc\"}\n")
}
