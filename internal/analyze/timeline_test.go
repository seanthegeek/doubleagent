package analyze

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parsers"
	"github.com/seanthegeek/doubleagent/internal/pyjson"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// specParser is a parser that emits the rows testdata/timeline_spec.json
// gives for the files it names, as the analyzer's fake parsers in
// testdata/timeline_oracle.py do.
type specParser struct {
	parser.Base
	spec pyjson.Value
}

// Wants reports whether the spec names the artifact's file.
func (p specParser) Wants(artifact *input.Artifact) bool {
	return p.spec.Get("rows").Has(artifact.Rel)
}

// secondarySpecParser is a spec parser whose spec names a secondary file.
type secondarySpecParser struct {
	specParser
}

// Secondary reports whether the artifact is the spec's secondary file.
func (p secondarySpecParser) Secondary(artifact *input.Artifact) bool {
	return p.spec.Get("secondary").Text() == artifact.Rel
}

// SecondaryTolerance returns the spec's tolerance.
func (p secondarySpecParser) SecondaryTolerance(*input.Artifact) float64 {
	tolerance, _ := p.spec.Get("tolerance").Float()
	return tolerance
}

// Marks returns the marks the spec says the parser records.
func (p specParser) Marks() parser.Marks {
	return parser.Marks{Sync: p.spec.Get("sync").Truthy(), Relay: p.spec.Get("relay").Truthy(),
		Origin: p.spec.Get("origin").Truthy()}
}

// Parse emits the spec's rows of the artifact's file.
func (p specParser) Parse(artifact *input.Artifact, _ parser.Options, emit parser.Emit) error {
	for _, fields := range p.spec.Get("rows").Get(artifact.Rel).Items() {
		row := p.BaseRow(artifact)
		for _, field := range fields.Members() {
			setRowField(&row, field)
		}
		emit(row)
	}
	return nil
}

// setRowField sets the row field a spec member names.
func setRowField(row *timeline.Row, field pyjson.Member) {
	text := field.Value.Text()
	targets := map[string]*string{"timestamp_utc": &row.TimestampUTC, "session_id": &row.SessionID,
		"project_path": &row.ProjectPath, "turn_type": &row.TurnType, "model": &row.Model,
		"tool_name": &row.ToolName, "tool_use_id": &row.ToolUseID, "text": &row.Text}
	if target, ok := targets[field.Key]; ok {
		*target = text
		return
	}
	line, _ := field.Value.Int()
	row.SourceLine = int(line)
}

// readTestData decodes one JSON file of testdata.
func readTestData(t *testing.T, name string) pyjson.Value {
	t.Helper()
	data, err := osfile.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	value, err := pyjson.Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

// timelineFiles is the collection of testdata/timeline_oracle.py.
var timelineFiles = []collectedFile{
	{"alice", "/home/alice", "codex-cli", ".codex/sessions/a.jsonl", "x\n"},
	{"alice", "/home/alice", "codex-cli", ".codex/history.jsonl", "y\n"},
	{"alice", "/home/alice", "claude-code", ".claude/settings.json", "{}\n"},
	{"bob", "/home/bob", "aider", ".aider.chat.history.md", "# hi\n"},
	{"bob", "/home/bob", "windsurf", ".codeium/windsurf/x.pb", "pb"},
	{"bob", "/home/bob", "copilot", ".config/github-copilot/apps.json", "{}"},
	{"alice", "/srv/proj", "project", ".git/config",
		"[remote \"origin\"]\n\turl = git@example.com:a/p.git\n"},
	{"alice", "/srv/proj", "project", ".git/HEAD", "ref: refs/heads/main\n"},
	{"alice", "/srv/proj", "claude-code", ".claude/settings.local.json", "{}\n"},
	{"alice", "/srv/empty", "project", "AGENTS.md", "# x\n"},
}

// timelineRuns are the runs of testdata/timeline_oracle.py: a name and the
// arguments after the output directory.
var timelineRuns = []struct {
	name string
	args []string
}{
	{"all", nil},
	{"filtered", []string{"--since", "2026-10-01T09:00:02Z", "--until", "2026-10-03", "--match",
		"hello|ls", "-i"}},
	{"agent", []string{"--agent", "claude-code", "--keep-undated", "--since", "2026-10-02"}},
	{"none", []string{"--agent", "aider"}},
}

func TestTimelineMatchesTheAnalyzerOverFakeParsers(t *testing.T) {
	spec, expected := readTestData(t, "timeline_spec.json"), readTestData(t, "timeline_expected.json")
	work := t.TempDir()
	collection := writeCollection(t, filepath.Join(work, "col"), []pyjson.Member{
		member("hostname", pyjson.NewString("h1")),
		member("projects", stringArray([]string{"/srv/proj", "/srv/empty", `c:\users\a\demo`}))},
		timelineFiles)
	env := testEnvironment()
	env.Version = "0.24.0"
	env.Parsers = func() *parsers.Registry { return specRegistry(spec) }
	for _, run := range timelineRuns {
		t.Run(run.name, func(t *testing.T) {
			output := filepath.Join(work, "out-"+run.name)
			args := append([]string{"timeline", collection, "-o", output}, run.args...)
			code, stdout, stderr := runCommand(env, args...)
			want := expected.Get(run.name)
			if wantCode, _ := want.Get("exit").Int(); int64(code) != wantCode {
				t.Errorf("exit %d, want %d", code, wantCode)
			}
			compareOutput(t, "stdout", want.Get("stdout").Text(), slashRoot(showRoot(stdout, work)))
			compareOutput(t, "stderr", want.Get("stderr").Text(), stderr)
			for _, name := range []string{TimelineFile, SessionsFile, InventoryFile} {
				data, err := osfile.ReadFile(filepath.Join(output, name))
				if err != nil {
					t.Fatal(err)
				}
				compareOutput(t, name, want.Get(name).Text(), slashRoot(showRoot(string(data), work)))
			}
		})
	}
}

// rootedPath finds a path the expected texts write under {ROOT}.
var rootedPath = regexp.MustCompile(`\{ROOT\}/[^\s"]*`)

// slashRoot returns text with the separators of each path under {ROOT}
// written as slashes, as the expected texts, recorded on a POSIX system,
// write them.
func slashRoot(text string) string {
	return rootedPath.ReplaceAllStringFunc(text, func(path string) string {
		return strings.ReplaceAll(strings.ReplaceAll(path, `\\`, "/"), `\`, "/")
	})
}

// compareOutput reports the first line where got differs from want.
func compareOutput(t *testing.T, label, want, got string) {
	t.Helper()
	if want == got {
		return
	}
	wanted, gotten := strings.Split(want, "\n"), strings.Split(got, "\n")
	for i := range max(len(wanted), len(gotten)) {
		left, right := "", ""
		if i < len(wanted) {
			left = wanted[i]
		}
		if i < len(gotten) {
			right = gotten[i]
		}
		if left != right {
			t.Errorf("%s line %d:\n want %q\n got  %q", label, i+1, left, right)
			return
		}
	}
}

// specRegistry returns a registry of one spec parser per entry of the spec,
// in its order.
func specRegistry(spec pyjson.Value) *parsers.Registry {
	var list []parser.Parser
	for _, entry := range spec.Get("parsers").Items() {
		each := specParser{Base: parser.NewBase(entry.Get("agent").Text(), "fake"), spec: entry}
		if entry.Has("secondary") {
			list = append(list, secondarySpecParser{each})
		} else {
			list = append(list, each)
		}
	}
	return parsers.NewOf(list)
}

func TestTimelineRefusesBadFiltersBeforeOpeningTheInput(t *testing.T) {
	work := t.TempDir()
	missing := filepath.Join(work, "missing")
	tests := []struct {
		name   string
		args   []string
		stderr string
	}{
		{"a since after the until",
			[]string{"--since", "2026-10-02", "--until", "2026-10-01"},
			"error: --since 2026-10-02T00:00:00.000Z is after --until 2026-10-01T23:59:59.999Z\n"},
		{"a value that is no time", []string{"--since", "soon"},
			"error: cannot read 'soon' as a time; accepted: "},
		{"a pattern that does not compile", []string{"--match", "("}, "error: --match '(': "},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args := append([]string{"timeline", missing, "-o", filepath.Join(work, "out")}, test.args...)
			code, stdout, stderr := runCommand(testEnvironment(), args...)
			if code != ExitError || stdout != "" || !strings.HasPrefix(stderr, test.stderr) {
				t.Errorf("exit %d, stdout %q, stderr %q", code, stdout, stderr)
			}
		})
	}
	if _, err := osfile.Lstat(filepath.Join(work, "out")); err == nil {
		t.Error("the output directory was made")
	}
}

func TestTimelineReportsAnOutputItCannotMake(t *testing.T) {
	work := t.TempDir()
	writeFile(t, filepath.Join(work, "file"), "x")
	if err := osfile.MkdirAll(filepath.Join(work, "in"), 0o755); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := runCommand(testEnvironment(), "timeline", filepath.Join(work, "in"), "-o",
		filepath.Join(work, "file", "out"))
	if code != ExitError || !strings.HasPrefix(showRoot(stderr, work), "error: {ROOT}/file") {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
}
