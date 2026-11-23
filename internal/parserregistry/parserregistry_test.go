package parserregistry

import (
	"context"
	"errors"
	"io/fs"
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
)

// stub is a parser of the tests: it wants artifacts whose rel it names,
// emits one row per artifact, then fails when told to.
type stub struct {
	parserkit.Base
	wants     string
	reads     []string
	secondary bool
	fail      error
	project   string
}

// Wants reports whether the artifact's rel is the one the stub wants.
func (s *stub) Wants(artifact *input.Artifact) bool { return artifact.Rel == s.wants }

// Parse emits one row naming the parser, then returns the stub's error.
func (s *stub) Parse(artifact *input.Artifact, _ parserkit.Options, emit parserkit.Emit) error {
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
func (s *readerStub) Marks() parserkit.Marks { return parserkit.Marks{Sync: true} }

// names returns the parsers' names.
func names(list []parserkit.Parser) []string {
	var found []string
	for _, each := range list {
		found = append(found, each.Name())
	}
	return found
}

func TestRegistryOffersAnArtifactToItsAgentsParsersThenItsReaders(t *testing.T) {
	codex := &stub{Base: parserkit.NewBase("codex-cli", "codex")}
	desktop := &readerStub{stub{Base: parserkit.NewBase("chatgpt-desktop", "desktop"),
		reads: []string{"codex-cli", "codex-cli"}}}
	aider := &stub{Base: parserkit.NewBase("aider", "aider")}
	registry := NewOf([]parserkit.Parser{codex, desktop, aider})
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

// sliceParser is a parser held by value whose slice field makes it a type
// Go cannot compare with ==: comparing two such values in interfaces panics.
type sliceParser struct {
	parserkit.Base
	reads []string
}

// Wants takes nothing.
func (s sliceParser) Wants(*input.Artifact) bool { return false }

// Parse emits nothing.
func (s sliceParser) Parse(*input.Artifact, parserkit.Options, parserkit.Emit) error { return nil }

// ReadsAgents returns the agents the parser names.
func (s sliceParser) ReadsAgents() []string { return s.reads }

func TestRegistryRoutesParsersOfATypeGoCannotCompare(t *testing.T) {
	editor := sliceParser{Base: parserkit.NewBase("editor", "editor")}
	extension := sliceParser{Base: parserkit.NewBase("extension", "extension"), reads: []string{"editor"}}
	both := sliceParser{Base: parserkit.NewBase("editor", "self"), reads: []string{"editor"}}
	registry := NewOf([]parserkit.Parser{editor, extension, both})
	tests := []struct {
		agent string
		want  []string
	}{
		{"editor", []string{"editor", "self", "extension"}},
		{"extension", []string{"extension"}},
	}
	for _, test := range tests {
		t.Run("the "+test.agent+" artifacts reach their parsers once each", func(t *testing.T) {
			if got := names(registry.For(test.agent)); !slices.Equal(got, test.want) {
				t.Errorf("got %v, want %v", got, test.want)
			}
		})
	}
}

func TestCollectRunsEveryWantingParserInArtifactThenRegistryOrder(t *testing.T) {
	denied := &fs.PathError{Op: "open", Path: "/x", Err: fs.ErrPermission}
	first := &stub{Base: parserkit.NewBase("codex-cli", "first"), wants: "a", project: `c:\Work`}
	failing := &stub{Base: parserkit.NewBase("codex-cli", "failing"), wants: "b", fail: denied}
	history := &readerStub{stub{Base: parserkit.NewBase("chatgpt-desktop", "history"), wants: "a",
		reads: []string{"codex-cli"}, secondary: true}}
	home := &input.Home{Host: "h1", User: "alice"}
	collection := &input.Collection{Problems: []string{"/x: earlier"}, Artifacts: []*input.Artifact{
		{Rel: "a", Agent: "codex-cli", Original: "/alice/a", Home: home},
		{Rel: "b", Agent: "codex-cli", Original: "/alice/b", Home: home},
	}}
	registry := NewOf([]parserkit.Parser{first, failing, history})
	result, err := registry.Collect(context.Background(), collection, parserkit.Options{}, nil)
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
	filtered, _ := registry.Collect(context.Background(), collection, parserkit.Options{},
		[]string{"chatgpt-desktop"})
	if len(filtered.Primary) != 0 || len(filtered.Secondary) != 1 {
		t.Errorf("the agent filter kept %+v", filtered)
	}
	cancelled, stop := context.WithCancel(context.Background())
	stop()
	if _, err := registry.Collect(cancelled, collection, parserkit.Options{}, nil); !errors.Is(err,
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
	if got := names(first); !slices.Contains(got, "chatgpt-desktop") {
		t.Errorf("registry %v", got)
	}
	if got := NoTranscripts(); !slices.Equal(got, []string{"camel-ai", "copilot", "metagpt", "shared"}) {
		t.Errorf("no transcripts %v", got)
	}
}
