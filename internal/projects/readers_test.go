//go:build unix

package projects

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/users"
)

// readerTree builds an image whose agent state exercises every reader and
// the quirks of the shell's text handling that Discover reproduces, and
// returns its root.
func readerTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeTree(t, root, readerFiles(), "srv/p1", "srv/p2", "srv/p3", "srv/escaped", "srv/last",
		"srv/p4", "srv/p5", "srv/nul", "home/alice/Documents/Codex/x",
		"home/alice/.codex/worktrees/w", "home/alice/Documents/Codexy", "srv/codex",
		"srv/codexdot", "srv/codexnest", "srv/gem/merge", "srv/win", "srv/sp ace", "srv/plain",
		"srv/goose1", "srv/goose2", "srv/notlisted", "srv/cop", "srv/nocop",
		"home/alice/.copilot/x", "srv/claw", "srv/clawcwd", "srv/nano", "srv/nanocfg",
		"srv/tab by", "srv/twin/src", "srv/vs code", "srv/cline", "srv/a0", "srv/a0x", "srv/a0y",
		"srv/fromstate", "srv/extra", "home/alice/inside", "home/bob")
	return root
}

// readerFiles returns the files of readerTree, by path under the root.
func readerFiles() map[string]string {
	alice := "home/alice/"
	vscode := alice + ".config/Code/User/"
	return map[string]string{
		alice + ".claude/history.jsonl": `{"project":"/srv/p1"}` + "\n" +
			`{"project":"/srv\/p2","x":1}{"project": "/srv/p3"}` + "\n" +
			`{"project":"\/srv\/escaped"}` + "\n" + `{"project":"/srv/last"}`,
		alice + ".claude.json":               `{"projects":{"/srv/p4": {"a":1},"/srv/p5":{}}}`,
		alice + ".claude-work/history.jsonl": `{"project":"/srv/nul"}` + "\n\x00\n",
		alice + ".codex/sessions/a/r.jsonl": rolloutCwds("/home/alice/Documents/Codex",
			"/home/alice/Documents/Codex/x", "/home/alice/.codex/worktrees/w",
			"/home/alice/Documents/Codexy", "/srv/codex"),
		alice + ".codex/sessions/b/.d.jsonl":            rolloutCwds("/srv/codexdot"),
		alice + ".codex/sessions/c.jsonl/r.jsonl":       rolloutCwds("/srv/codexnest"),
		alice + ".gemini/tmp/x/.project_root":           "/srv/gem",
		alice + ".gemini/antigravity-cli/history.jsonl": `{"workspace":"/merge"}` + "\n",
		alice + ".gemini/antigravity-cli/settings.json": `{"a":"file:///C%3A/srv/win",` +
			`"b":"file:///srv/sp%20ace","c":"/srv/plain"}`,
		alice + ".local/share/goose/projects/p.md": "workingDirs:\r\n- '/srv/goose1'  \n-\n" +
			"- \"/srv/goose2\"\nname: x\n- /srv/notlisted\n",
		alice + ".copilot/session-state/s1/events.jsonl":   "{}\n",
		alice + ".copilot/session-state/s1/workspace.yaml": "cwd: '/srv/cop'\n",
		alice + ".copilot/session-state/s2/workspace.yaml": "cwd: /srv/nocop\n",
		alice + ".copilot/session-state/s3/events.jsonl":   "{}\n",
		alice + ".copilot/session-state/s3/workspace.yaml": "cwd: /home/alice/.copilot/x\n",
		alice + ".openclaw/openclaw.json": `{agents:{defaults:{workspace:'/srv/claw'}}, ` +
			`"cwd": "/srv/clawcwd"}`,
		alice + ".nanobot/sessions/s/.workspace": "/srv/nano",
		alice + ".nanobot/config.json":           `{"workspace":"/srv/nanocfg"}`,
		alice + ".tabby/config.toml":             "git_url = 'file:///srv/tab%20by'\n",
		alice + ".twinny/embeddings/t/manifest.json": `{"/srv/twin/src/a.ts":1,` +
			`"/srv/twin/b.ts":2}`,
		vscode + "workspaceStorage/h1/workspace.json": `{"folder": ` +
			`"file:///srv/vs%20code"}`,
		vscode + "globalStorage/saoudrizwan.claude-dev/state/taskHistory.json": `[{` +
			`"cwdOnTaskInitialization":"/srv/cline"}]`,
		alice + "agent-zero/usr/chats/c1/chat.json": `{"acp_cwd":"/srv/a0",` +
			`"acp_additional_directories": ["/srv/a0x", "rel", "/srv/a0y"]}`,
		"home/bob/.claude/history.jsonl": `{"project":"/srv/p1"}` + "\n" +
			`{"project":"/home/alice"}` + "\n" + `{"project":"/srv/fromstate"}` + "\n" +
			`{"project":"/srv/notadir"}` + "\n",
		"srv/notadir": "",
	}
}

// rolloutCwds returns a rollout file's text naming each cwd on its own line.
func rolloutCwds(cwds ...string) string {
	var text strings.Builder
	for _, cwd := range cwds {
		text.WriteString(`{"type":"session_meta","payload":{"cwd":"` + cwd + `"}}` + "\n")
	}
	return text.String()
}

// wantReaderProjects are the projects found in readerTree with the -p
// directories of TestDiscoverEveryReader, as owner and path under the root.
// The two projects of Codex session files in sibling directories come in the
// file system's listing order, so the test accepts either order for that pair.
var wantReaderProjects = []string{
	"alice /srv/p1", "alice /srv/p2", "alice /srv/p3", "alice /srv/last", "alice /srv/p4",
	"alice /srv/p5", "alice /home/alice/Documents/Codexy", "alice /srv/codex",
	"alice /srv/codexnest", "alice /srv/codexdot", "alice /srv/a0", "alice /srv/a0x",
	"alice /srv/a0y", "alice /srv/gem/merge", "alice /srv/win", "alice /srv/sp ace",
	"alice /srv/plain", "alice /srv/goose1", "alice /srv/goose2", "alice /srv/cop",
	"alice /srv/claw", "alice /srv/clawcwd", "alice /srv/nano", "alice /srv/nanocfg",
	"alice /srv/tab by", "alice /srv/twin", "alice /srv/vs code", "alice /srv/cline",
	"bob /srv/fromstate", " /srv/extra", "alice /home/alice/inside",
}

// inListingOrder puts the pair of Codex session projects, which come in the
// file system's listing order, in the order wantReaderProjects names.
func inListingOrder(projects []string) []string {
	nest := slices.Index(projects, "alice /srv/codexnest")
	dot := slices.Index(projects, "alice /srv/codexdot")
	if nest >= 0 && dot >= 0 && dot+1 == nest {
		projects[dot], projects[nest] = projects[nest], projects[dot]
	}
	return projects
}

func TestDiscoverEveryReader(t *testing.T) {
	root := readerTree(t)
	extra := []string{root + "/srv/extra", root + "/home/alice/inside",
		root + "/srv/fromstate", root + "/srv/missing"}
	homes := users.Enumerate(context.Background(), false, root, "", nil, nil, nil, nil)
	found, _, err := Discover(context.Background(), homes, root, extra, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, project := range found {
		got = append(got, project.Owner+" "+strings.TrimPrefix(project.Path, root))
	}
	if got = inListingOrder(got); !slices.Equal(got, wantReaderProjects) {
		t.Errorf("projects:\n%q\nwant:\n%q", got, wantReaderProjects)
	}
}
