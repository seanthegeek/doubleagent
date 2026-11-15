package summary

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/manifest"
	"github.com/seanthegeek/doubleagent/internal/volumes"
)

// shDocument is collection.json as the sh collector writes it.
type shDocument struct {
	Version  string `json:"version"`
	Hostname string `json:"hostname"`
	Mode     string `json:"mode"`
	Root     string `json:"root"`
	Platform string `json:"platform"`
	UID      string `json:"run_as_uid"`
	Started  string `json:"started"`
	Finished string `json:"finished"`
	Options  struct {
		Full           bool     `json:"full"`
		NoSecrets      bool     `json:"no_secrets"`
		MaxFileBytes   int64    `json:"max_file_size_bytes"`
		UsersFilter    string   `json:"users_filter"`
		NoDocker       bool     `json:"no_docker"`
		NoProjects     bool     `json:"no_projects"`
		Projects       []string `json:"projects"`
		EnvScanHome    bool     `json:"env_scan_home"`
		EnvScanMaxSize int64    `json:"env_scan_max_size_bytes"`
	} `json:"options"`
	Capabilities struct {
		Plutil bool `json:"plutil"`
	} `json:"capabilities"`
	Users      []string                  `json:"users"`
	Homes      []string                  `json:"homes"`
	Active     []string                  `json:"users_with_artifacts"`
	Projects   []string                  `json:"projects"`
	Counts     map[manifest.Status]int64 `json:"counts"`
	DockerRaw  json.RawMessage           `json:"docker"`
	Relocation shRelocation              `json:"relocation"`
	Notes      []string                  `json:"notes"`
	Archive    string                    `json:"archive"`
}

type shRelocation struct {
	Hints []struct {
		User     string `json:"user"`
		Source   string `json:"source"`
		Line     int    `json:"line"`
		Variable string `json:"variable"`
		Agent    string `json:"agent"`
		Value    string `json:"value"`
	} `json:"hints"`
	Registry struct {
		Read   bool   `json:"read"`
		Reason string `json:"reason"`
	} `json:"registry"`
	FilesSearched    int `json:"files_searched"`
	FilesSkippedSize int `json:"files_skipped_size"`
}

// fixtureTree builds an image with a home, docker volumes, a hint and a secret.
func fixtureTree(t *testing.T) (root, projectDir string) {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root, projectDir = filepath.Join(base, "root"), filepath.Join(base, "proj")
	files := map[string]string{
		"etc/passwd":                                        "alice:x:1000:1000::/home/alice:/bin/sh\nbob:x:1001:1001::/home/bob:/bin/sh\n",
		"home/alice/.claude/settings.json":                  "{}",
		"home/alice/.claude/.credentials.json":              "secret",
		"home/alice/.claude/big.bin":                        strings.Repeat("x", 2<<20),
		"home/alice/.bashrc":                                "export CLAUDE_CONFIG_DIR=/home/alice/cc\nexport CODEX_HOME=\"$HOME/cx\"\n",
		"home/bob/.zshrc":                                   "# nothing\n",
		"var/lib/docker/volumes/a0_usr/_data/f":             "hi",
		"var/lib/docker/volumes/postgres/_data/PG":          "16",
		"home/alice/Library/Containers/com.docker.docker/x": "d",
	}
	for relative, content := range files {
		full := filepath.Join(root, relative)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	return root, projectDir
}

// runSh runs the sh collector in image mode and returns the text of
// collection.json and of its standard output.
func runSh(t *testing.T, args ...string) (document, stdout string) {
	t.Helper()
	shell, err := exec.LookPath("dash")
	if err != nil {
		t.Skip("dash is not installed")
	}
	script, err := filepath.Abs("../../collectors/collect-agent-artifacts.sh")
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	command := exec.Command(shell, append([]string{script, "-o", out, "-q"}, args...)...)
	command.Env = append(os.Environ(), "COLLECTOR_SH=dash", "LC_ALL=C")
	output, err := command.Output()
	if err != nil {
		t.Fatalf("sh collector: %v", err)
	}
	names, _ := filepath.Glob(filepath.Join(out, "*.collection.json"))
	if len(names) != 1 {
		t.Fatalf("collection.json files: %v", names)
	}
	content, err := os.ReadFile(names[0])
	if err != nil {
		t.Fatal(err)
	}
	return string(content), string(output)
}

// dropLines removes the lines that start with any of the quoted keys.
func dropLines(text string, keys ...string) string {
	var kept []string
	for _, line := range strings.Split(text, "\n") {
		drop := false
		for _, key := range keys {
			drop = drop || strings.HasPrefix(strings.TrimSpace(line), key)
		}
		if !drop {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
}

// summaryFrom converts the sh collector's own report into a Summary, so the
// writers' layout is compared over real values.
func summaryFrom(t *testing.T, document, stdout string) Summary {
	t.Helper()
	var parsed shDocument
	if err := json.Unmarshal([]byte(document), &parsed); err != nil {
		t.Fatal(err)
	}
	var docker struct {
		Found      int  `json:"volumes_found"`
		Collected  int  `json:"volumes_collected"`
		Unreadable int  `json:"unreadable"`
		Desktop    bool `json:"docker_desktop"`
	}
	if err := json.Unmarshal(parsed.DockerRaw, &docker); err != nil {
		t.Fatal(err)
	}
	uid, _ := strconv.Atoi(parsed.UID)
	byStatus := map[manifest.Status]int{}
	for status, count := range parsed.Counts {
		byStatus[status] = int(count)
	}
	summary := Summary{
		Version: parsed.Version, Hostname: parsed.Hostname, Mode: parsed.Mode, Root: parsed.Root,
		Platform: parsed.Platform, UID: uid, Started: parsed.Started, Finished: parsed.Finished,
		Options: Options(parsed.Options), Plutil: parsed.Capabilities.Plutil, Users: parsed.Users,
		Homes: parsed.Homes, ActiveUsers: parsed.Active, Projects: parsed.Projects,
		Counts: manifest.Counts{ByStatus: byStatus, CollectedBytes: parsed.Counts["collected_bytes"]},
		Docker: volumes.Counts{
			Found: docker.Found, Collected: docker.Collected,
			Unreadable: docker.Unreadable, Desktop: docker.Desktop,
		},
		Notes: parsed.Notes, Archive: parsed.Archive,
		Relocation: Relocation{RegistryRead: parsed.Relocation.Registry.Read, RegistryReason: parsed.Relocation.Registry.Reason, FilesSearched: parsed.Relocation.FilesSearched, FilesSkippedSize: parsed.Relocation.FilesSkippedSize},
	}
	for _, hint := range parsed.Relocation.Hints {
		summary.Relocation.Hints = append(summary.Relocation.Hints, Hint(hint))
	}
	summary.Docker.Seen = strings.Contains(stdout, "\ndocker:")
	return fillFromStdout(summary, stdout)
}

var resolvedCount = regexp.MustCompile(`, (\d+) resolved`)

// fillFromStdout reads the paths, size and checksum from the sh summary.
func fillFromStdout(summary Summary, stdout string) Summary {
	for _, line := range strings.Split(stdout, "\n") {
		name, value, _ := strings.Cut(line, ":")
		value = strings.TrimSpace(value)
		switch name {
		case "archive":
			summary.ArchivePath = value
		case "size":
			summary.ArchiveSize, _ = strconv.ParseInt(strings.Fields(value)[0], 10, 64)
		case "sha256":
			summary.ArchiveSHA = value
		case "manifest":
			summary.ManifestPath = value
		case "summary":
			summary.SummaryPath = value
		case "log":
			summary.LogPath = value
		case "relocation":
			if match := resolvedCount.FindStringSubmatch(value); match != nil {
				summary.Relocation.Resolved, _ = strconv.Atoi(match[1])
			}
		}
	}
	return summary
}

func TestWritersMatchShCollector(t *testing.T) {
	root, projectDir := fixtureTree(t)
	tests := []struct {
		name string
		args []string
	}{
		{"every home and the docker volumes", []string{"-r", root, "--max-file-size", "1"}},
		{"one user, no docker, an extra project", []string{
			"-r", root, "-u", "alice", "--no-docker", "-p", projectDir, "--no-secrets",
			"--max-file-size", "0", "--full", "--env-scan-home",
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			document, stdout := runSh(t, test.args...)
			summary := summaryFrom(t, document, stdout)
			var got strings.Builder
			if err := WriteCollectionJSON(&got, summary); err != nil {
				t.Fatal(err)
			}
			drop := []string{`"started"`, `"finished"`, `"archive"`, `"tool"`, `"version"`, `"capabilities"`}
			if dropLines(got.String(), drop...) != dropLines(document, drop...) {
				t.Errorf("collection.json differs\ngo:\n%s\nsh:\n%s", got.String(), document)
			}
			var result strings.Builder
			if err := PrintResult(&result, summary); err != nil {
				t.Fatal(err)
			}
			if dropLines(result.String(), "size:", "sha256:") != dropLines(stdout, "size:", "sha256:") {
				t.Errorf("stdout differs\ngo:\n%s\nsh:\n%s", result.String(), stdout)
			}
		})
	}
}
