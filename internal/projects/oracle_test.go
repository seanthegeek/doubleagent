//go:build unix

package projects

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/users"
)

// systemPath is the PATH the sh collector runs under in these tests, so its
// grep, sed and awk are the system's and not a replacement earlier on the
// developer's PATH.
const systemPath = "/usr/bin:/bin:/usr/sbin:/sbin"

// shProjects runs the sh collector under dash in image mode over root, with
// each extra directory as a -p, and returns the projects its log names, in
// order, as Discover reports them.
func shProjects(t *testing.T, root string, extra []string) []Project {
	t.Helper()
	dash, err := exec.LookPath("dash")
	if err != nil {
		t.Skip("dash is not installed")
	}
	script, err := filepath.Abs("../../collectors/collect-agent-artifacts.sh")
	if err != nil {
		t.Fatal(err)
	}
	outDir := t.TempDir()
	args := []string{script, "-r", root, "-o", outDir, "-q", "--no-docker", "--max-file-size", "1"}
	for _, project := range extra {
		args = append(args, "-p", project)
	}
	command := exec.Command(dash, args...)
	command.Env = append(os.Environ(), "COLLECTOR_SH=dash", "PATH="+systemPath)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("sh collector: %v\n%s", err, output)
	}
	logs, err := filepath.Glob(filepath.Join(outDir, "*.log"))
	if err != nil || len(logs) != 1 {
		t.Fatalf("log files = %v, %v", logs, err)
	}
	return projectsLogged(t, logs[0])
}

// projectsLogged reads the `Project [owner] path` lines of a sh collector log.
func projectsLogged(t *testing.T, logPath string) []Project {
	t.Helper()
	handle, err := os.Open(logPath)
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()
	var logged []Project
	scanner := bufio.NewScanner(handle)
	for scanner.Scan() {
		_, text, _ := strings.Cut(scanner.Text(), " ")
		rest, found := strings.CutPrefix(text, "Project ")
		if !found {
			continue
		}
		project := Project{Path: rest}
		if owned, ok := strings.CutPrefix(rest, "["); ok {
			project.Owner, project.Path, _ = strings.Cut(owned, "] ")
		}
		logged = append(logged, project)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return logged
}

// compareWithSh runs Discover and the sh collector over root and reports
// every difference between the two project lists. It returns how many
// projects the sh collector found.
func compareWithSh(t *testing.T, root string, extra []string) int {
	t.Helper()
	homes := users.Enumerate(false, root, "", nil)
	got, err := Discover(context.Background(), homes, root, extra, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := shProjects(t, root, extra)
	if (len(got) != 0 || len(want) != 0) && !reflect.DeepEqual(got, want) {
		t.Errorf("Discover differs from the sh collector\n got: %q\nwant: %q", got, want)
	}
	return len(want)
}

func TestDiscoverAgreesWithShCollector(t *testing.T) {
	root := oracleTree(t)
	if compareWithSh(t, root, []string{root + "/srv/extra", root + "/home/alice/inside",
		root + "/srv/fromstate", root + "/srv/missing"}) == 0 {
		t.Error("the sh collector found no project")
	}
}

// oracleTree builds an image whose agent state exercises every reader and
// the sh quirks Discover reproduces, and returns its root.
func oracleTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeTree(t, root, oracleFiles(), "srv/p1", "srv/p2", "srv/p3", "srv/escaped", "srv/last",
		"srv/p4", "srv/p5", "srv/nul", "home/alice/Documents/Codex/x",
		"home/alice/.codex/worktrees/w", "home/alice/Documents/Codexy", "srv/codex",
		"srv/codexdot", "srv/codexnest", "srv/gem/merge", "srv/win", "srv/sp ace", "srv/plain",
		"srv/goose1", "srv/goose2", "srv/notlisted", "srv/cop", "srv/nocop",
		"home/alice/.copilot/x", "srv/claw", "srv/clawcwd", "srv/nano", "srv/nanocfg",
		"srv/tab by", "srv/twin/src", "srv/vs code", "srv/cline", "srv/a0", "srv/a0x", "srv/a0y",
		"srv/fromstate", "srv/extra", "home/alice/inside", "home/bob")
	return root
}

// oracleFiles returns the files of oracleTree, by path under the root.
func oracleFiles() map[string]string {
	alice := "home/alice/"
	vscode := alice + ".config/Code/User/"
	return map[string]string{
		alice + ".claude/history.jsonl": `{"project":"/srv/p1"}` + "\n" +
			`{"project":"/srv\/p2","x":1}{"project": "/srv/p3"}` + "\n" +
			`{"project":"\/srv\/escaped"}` + "\n" + `{"project":"/srv/last"}`,
		alice + ".claude.json":               `{"projects":{"/srv/p4": {"a":1},"/srv/p5":{}}}`,
		alice + ".claude-work/history.jsonl": `{"project":"/srv/nul"}` + "\n\x00\n",
		alice + ".codex/sessions/a/r.jsonl": codexCwds("/home/alice/Documents/Codex",
			"/home/alice/Documents/Codex/x", "/home/alice/.codex/worktrees/w",
			"/home/alice/Documents/Codexy", "/srv/codex"),
		alice + ".codex/sessions/b/.d.jsonl":            codexCwds("/srv/codexdot"),
		alice + ".codex/sessions/c.jsonl/r.jsonl":       codexCwds("/srv/codexnest"),
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

// codexCwds returns a rollout file's text naming each cwd on its own line.
func codexCwds(cwds ...string) string {
	var text strings.Builder
	for _, cwd := range cwds {
		text.WriteString(`{"type":"session_meta","payload":{"cwd":"` + cwd + `"}}` + "\n")
	}
	return text.String()
}
