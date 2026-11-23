package parserkit

import (
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// plain is a parser with none of the optional interfaces.
type plain struct{ Base }

// Wants wants everything.
func (plain) Wants(*input.Artifact) bool { return true }

// Parse emits nothing.
func (plain) Parse(*input.Artifact, Options, Emit) error { return nil }

// rich is a parser with every optional interface.
type rich struct{ plain }

// ReadsAgents names one other agent.
func (rich) ReadsAgents() []string { return []string{"vscode"} }

// Secondary says every artifact named history is secondary.
func (rich) Secondary(artifact *input.Artifact) bool { return artifact.Rel == "history" }

// SecondaryTolerance widens the window.
func (rich) SecondaryTolerance(*input.Artifact) float64 { return 30 }

// Marks records every mark.
func (rich) Marks() Marks { return Marks{Sync: true, Relay: true, Origin: true} }

func TestOptionalInterfacesHaveTheBaseDefaults(t *testing.T) {
	history := &input.Artifact{Rel: "history"}
	without := plain{NewBase("a", "a")}
	with := rich{plain{NewBase("b", "b")}}
	tests := []struct {
		name      string
		subject   Parser
		agents    []string
		secondary bool
		tolerance float64
		marks     Marks
	}{
		{"a parser without them reads no other agent and has no secondary source", without, nil,
			false, DefaultTolerance, Marks{}},
		{"a parser with them answers for itself", with, []string{"vscode"}, true, 30,
			Marks{Sync: true, Relay: true, Origin: true}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ReadsAgentsOf(test.subject); !slices.Equal(got, test.agents) {
				t.Errorf("reads agents %v", got)
			}
			if got := IsSecondary(test.subject, history); got != test.secondary ||
				HasSecondary(test.subject) != test.secondary {
				t.Errorf("secondary %v", got)
			}
			if got := Tolerance(test.subject, history); got != test.tolerance {
				t.Errorf("tolerance %v", got)
			}
			if got := MarksOf(test.subject); got != test.marks {
				t.Errorf("marks %+v", got)
			}
		})
	}
}

func TestErrorRowsCarryTheProblemLineAndText(t *testing.T) {
	base := NewBase("aider", "aider")
	artifact := &input.Artifact{Original: "/alice/.aider.chat.history.md",
		Home: &input.Home{Host: "h1", User: "alice"}}
	var problems Problems
	problems.AddRaw(4, "unparseable line (not a JSON object)", "[1,2]")
	problems.Add(0, "history.md could not be read (permission denied)")
	rows := base.ErrorRows(artifact, problems, "s1", "/srv/proj")
	want := []timeline.Row{
		{Host: "h1", User: "alice", Agent: "aider", SessionID: "s1", ProjectPath: "/srv/proj",
			TurnType: "system", Text: "parser: unparseable line (not a JSON object): [1,2]",
			SourceFile: "/alice/.aider.chat.history.md", SourceLine: 4},
		{Host: "h1", User: "alice", Agent: "aider", SessionID: "s1", ProjectPath: "/srv/proj",
			TurnType: "system", Text: "parser: history.md could not be read (permission denied)",
			SourceFile: "/alice/.aider.chat.history.md"},
	}
	if !slices.Equal(rows, want) {
		t.Errorf("got %+v, want %+v", rows, want)
	}
	var emitted []timeline.Row
	EmitAll(func(row timeline.Row) { emitted = append(emitted, row) }, rows)
	if !slices.Equal(emitted, want) {
		t.Errorf("EmitAll gave %+v", emitted)
	}
	if got := base.ProblemRow(artifact, "no table"); got.Text != "parser: no table" ||
		got.TurnType != timeline.TurnSystem || got.Agent != "aider" || got.User != "alice" {
		t.Errorf("ProblemRow gave %+v", got)
	}
}
