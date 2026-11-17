package parsers

import (
	"context"
	"errors"
	"io/fs"
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
)

// stub is a parser of the tests: it wants artifacts whose rel it names,
// emits one row per artifact, then fails when told to.
type stub struct {
	parser.Base
	wants     string
	reads     []string
	secondary bool
	fail      error
	project   string
}

// Wants reports whether the artifact's rel is the one the stub wants.
func (s *stub) Wants(artifact *input.Artifact) bool { return artifact.Rel == s.wants }

// Parse emits one row naming the parser, then returns the stub's error.
func (s *stub) Parse(artifact *input.Artifact, _ parser.Options, emit parser.Emit) error {
	row := s.SystemRow(artifact, s.Name()+" read "+artifact.Rel)
	row.ProjectPath = s.project
	emit(row)
	return s.fail
}

// readerStub is a stub that is offered other agents' artifacts and reads
// a secondary source.
type readerStub struct{ stub }

// ReadsAgents returns the agents the stub names.
func (s *readerStub) ReadsAgents() []string { return s.reads }

// Secondary reports the stub's setting.
func (s *readerStub) Secondary(*input.Artifact) bool { return s.secondary }

// SecondaryTolerance is a window of five seconds.
func (s *readerStub) SecondaryTolerance(*input.Artifact) float64 { return 5 }

// Marks records synced sessions.
func (s *readerStub) Marks() parser.Marks { return parser.Marks{Sync: true} }

// names returns the parsers' names.
func names(list []parser.Parser) []string {
	var found []string
	for _, each := range list {
		found = append(found, each.Name())
	}
	return found
}

func TestRegistryRoutesAnArtifactAsTheAnalyzerDoes(t *testing.T) {
	codex := &stub{Base: parser.NewBase("codex-cli", "codex")}
	desktop := &readerStub{stub{Base: parser.NewBase("chatgpt-desktop", "desktop"),
		reads: []string{"codex-cli", "codex-cli"}}}
	aider := &stub{Base: parser.NewBase("aider", "aider")}
	registry := NewOf([]parser.Parser{codex, desktop, aider})
	tests := []struct {
		agent string
		want  []string
	}{
		{"codex-cli", []string{"codex", "desktop"}},
		{"chatgpt-desktop", []string{"desktop"}},
		{input.ProjectAgent, []string{"codex", "desktop", "aider"}},
		{"cursor", nil},
	}
	for _, test := range tests {
		t.Run(test.agent, func(t *testing.T) {
			if got := names(registry.For(test.agent)); !slices.Equal(got, test.want) {
				t.Errorf("got %v, want %v", got, test.want)
			}
		})
	}
	marks := registry.Marks()
	if !marks.Synced["chatgpt-desktop"] || marks.Synced["codex-cli"] ||
		!marks.Secondary["chatgpt-desktop"] || len(marks.Relayed) != 0 {
		t.Errorf("marks %+v", marks)
	}
	if hooks := registry.Hooks(); len(hooks) != 0 {
		t.Errorf("hooks %v", hooks)
	}
}

func TestCollectRunsEveryWantingParserInArtifactThenRegistryOrder(t *testing.T) {
	denied := &fs.PathError{Op: "open", Path: "/x", Err: fs.ErrPermission}
	first := &stub{Base: parser.NewBase("codex-cli", "first"), wants: "a", project: `c:\Work`}
	failing := &stub{Base: parser.NewBase("codex-cli", "failing"), wants: "b", fail: denied}
	history := &readerStub{stub{Base: parser.NewBase("chatgpt-desktop", "history"), wants: "a",
		reads: []string{"codex-cli"}, secondary: true}}
	home := &input.Home{Host: "h1", User: "alice"}
	collection := &input.Collection{Problems: []string{"/x: earlier"}, Artifacts: []*input.Artifact{
		{Rel: "a", Agent: "codex-cli", Original: "/alice/a", Home: home},
		{Rel: "b", Agent: "codex-cli", Original: "/alice/b", Home: home},
	}}
	registry := NewOf([]parser.Parser{first, failing, history})
	result, err := registry.Collect(context.Background(), collection, parser.Options{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var primary []string
	for _, row := range result.Primary {
		primary = append(primary, row.Text+" "+row.ProjectPath)
	}
	if want := []string{`first read a C:\Work`, "failing read b "}; !slices.Equal(primary, want) {
		t.Errorf("primary %q, want %q", primary, want)
	}
	if len(result.Secondary) != 1 || result.Secondary[0].Row.Text != "history read a" ||
		result.Secondary[0].Tolerance != 5 {
		t.Errorf("secondary %+v", result.Secondary)
	}
	if want := []string{"/x: earlier", "/alice/b: permission denied"}; !slices.Equal(result.Problems, want) {
		t.Errorf("problems %q, want %q", result.Problems, want)
	}
	filtered, _ := registry.Collect(context.Background(), collection, parser.Options{},
		[]string{"chatgpt-desktop"})
	if len(filtered.Primary) != 0 || len(filtered.Secondary) != 1 {
		t.Errorf("the agent filter kept %+v", filtered)
	}
	cancelled, stop := context.WithCancel(context.Background())
	stop()
	if _, err := registry.Collect(cancelled, collection, parser.Options{}, nil); !errors.Is(err,
		context.Canceled) {
		t.Errorf("a cancelled run gave %v", err)
	}
}

func TestAllReturnsNewValuesInRegistryOrder(t *testing.T) {
	first, second := All(), All()
	if len(first) == 0 || len(first) != len(second) {
		t.Fatalf("All gave %d and %d parsers", len(first), len(second))
	}
	for i := range first {
		if first[i] == second[i] {
			t.Errorf("parser %d is the same value twice", i)
		}
	}
	if got := names(first); got[0] != "claude-code" {
		t.Errorf("registry %v", got)
	}
	if got := NoTranscripts(); !slices.Equal(got, []string{"camel-ai", "copilot", "metagpt", "shared"}) {
		t.Errorf("no transcripts %v", got)
	}
}
