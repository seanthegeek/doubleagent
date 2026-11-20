package analyze

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parsers"
)

// projectPathOutputs runs detect, detect --json and timeline with options
// over a collection that holds a Claude Code transcript whose working
// directory is project, and lists it as a project when listed, and returns
// everything they printed and wrote, joined.
func projectPathOutputs(t *testing.T, project string, listed bool, options ...string) string {
	t.Helper()
	work := t.TempDir()
	line := jsonvalue.NewObject(member("type", jsonvalue.NewString("user")),
		member("sessionId", jsonvalue.NewString("s1")), member("uuid", jsonvalue.NewString("u1")),
		member("timestamp", jsonvalue.NewString("2026-10-01T00:00:00Z")), member("cwd", jsonvalue.NewString(project)),
		member("message", jsonvalue.NewObject(member("role", jsonvalue.NewString("user")),
			member("content", jsonvalue.NewString("hello")))))
	summary := []jsonvalue.Member{member("hostname", jsonvalue.NewString("h1"))}
	if listed {
		summary = append(summary, member("projects", stringArray([]string{project})))
	}
	collection := writeCollection(t, filepath.Join(work, "col"), summary,
		[]collectedFile{{"alice", "/home/alice", "claude-code",
			".claude/projects/C--proj/11111111-2222-4333-8444-555555555555.jsonl", line.JSON() + "\n"}})
	env := Environment{Now: func() time.Time { return time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC) },
		Parsers: parsers.New, Version: "0.0.0-test"}
	output := filepath.Join(work, "out")
	var printed []string
	for _, args := range [][]string{{"detect"}, {"detect", "--json"}, {"timeline"}} {
		command := append(append(args, options...), collection)
		if args[0] == "timeline" {
			command = append(command, output)
		}
		code, stdout, stderr := runCommand(env, command...)
		if code > ExitNothing || stderr != "" {
			t.Fatalf("%q exited %d: %s", command, code, stderr)
		}
		printed = append(printed, stdout)
	}
	for _, name := range []string{TimelineFile, SessionsFile, InventoryFile} {
		data, err := osfile.ReadFile(filepath.Join(output, name))
		if err != nil {
			t.Fatal(err)
		}
		printed = append(printed, string(data))
	}
	return strings.Join(printed, "\n")
}

func TestAURLCredentialInAWindowsProjectPathIsWithheldInEveryOutput(t *testing.T) {
	tests := []struct {
		name, project string
		listed        bool
	}{
		{"a long password", "C:/proj/https://QZ:QZJWZJWQ@h/x", false},
		{"a long password in a listed project", "C:/proj/https://QZ:QZJWZJWQ@h/x", true},
		{"a short password", "C:/proj/https://QZ:QZJ@h/x", false},
		{"a short password in a listed project", "C:/proj/https://QZ:QZJ@h/x", true},
		{"a short password after doubled separators", `c:\proj\\https://QZ:QZJ@h//x`, true},
		{"a query token holding a slash", "C:/proj/https://h/?token=QZJW/QZJW", true},
		{"a URL whose scheme is the drive letter", "C://QZ:QZJ@h/x", true},
		{"a UNC path", `\\srv/share/https://QZ:QZJ@h/x`, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if shown := projectPathOutputs(t, test.project, test.listed); !strings.Contains(shown, "QZJ") {
				t.Fatal("the run that shows secrets does not show the project path's credential")
			}
			withheld := projectPathOutputs(t, test.project, test.listed, "--no-secrets")
			if strings.Contains(withheld, "QZJ") {
				i := strings.Index(withheld, "QZJ")
				t.Errorf("--no-secrets shows the credential: %q", withheld[max(0, i-60):min(len(withheld), i+20)])
			}
		})
	}
}
