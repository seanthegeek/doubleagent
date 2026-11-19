//go:build parity

package replay

import (
	"fmt"
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parser/agentzero"
	"github.com/seanthegeek/doubleagent/internal/parser/tabby"
	"github.com/seanthegeek/doubleagent/internal/parsers"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// A command's outputs count every row a parse gives (inventory.json, the
// summary on stdout), so a row of withheldRows that the recording never
// gave would change every count of a case whose tree holds its file. The
// command replay therefore takes such a row at the parse that gives it,
// under difference 231's own rule, before the run counts and writes its
// rows; the parser replay compares the row itself.

// addedRowFilter takes, from each parse of a command run, the first row of
// withheldRows that the parse gives, and counts the parses it took one
// from.
type addedRowFilter struct {
	taken int
}

// emit returns the emit of one parse of artifact by agent's parser: it
// passes each row on but, for each entry of withheldRows for agent, the
// first system row of the artifact whose text is the entry's.
func (f *addedRowFilter) emit(agent string, artifact *input.Artifact, emit parser.Emit) parser.Emit {
	waiting := map[string]bool{}
	for _, entry := range withheldRows() {
		if entry.agent == agent {
			waiting[entry.text] = true
		}
	}
	return func(row timeline.Row) {
		if row.TurnType == timeline.TurnSystem && row.SourceFile == artifact.Original && waiting[row.Text] {
			delete(waiting, row.Text)
			f.taken++
			return
		}
		emit(row)
	}
}

// withoutAddedRows returns list with the parser of each agent that has an
// entry of withheldRows in place of one whose parses pass through filter.
// Each stand-in embeds the parser's own type, so every optional interface
// the parser implements is still there.
func withoutAddedRows(list []parser.Parser, filter *addedRowFilter) []parser.Parser {
	if filter == nil {
		return list
	}
	out := slices.Clone(list)
	for i, each := range out {
		switch concrete := each.(type) {
		case *agentzero.Parser:
			out[i] = agentZeroRows{concrete, filter}
		case *tabby.Parser:
			out[i] = tabbyRows{concrete, filter}
		}
	}
	return out
}

// agentZeroRows is the Agent Zero parser with its parses passed through an
// addedRowFilter.
type agentZeroRows struct {
	*agentzero.Parser
	filter *addedRowFilter
}

// Parse parses the artifact through the filter.
func (p agentZeroRows) Parse(artifact *input.Artifact, options parser.Options, emit parser.Emit) error {
	return p.Parser.Parse(artifact, options, p.filter.emit(p.Agent(), artifact, emit))
}

// tabbyRows is the Tabby parser with its parses passed through an
// addedRowFilter.
type tabbyRows struct {
	*tabby.Parser
	filter *addedRowFilter
}

// Parse parses the artifact through the filter.
func (p tabbyRows) Parse(artifact *input.Artifact, options parser.Options, emit parser.Emit) error {
	return p.Parser.Parse(artifact, options, p.filter.emit(p.Agent(), artifact, emit))
}

func TestEveryAgentOfAWithheldRowHasAStandInThatTakesIt(t *testing.T) {
	registered := parsers.ByAgent(parsers.All())
	for _, entry := range withheldRows() {
		t.Run(entry.label(), func(t *testing.T) {
			filter := &addedRowFilter{}
			standIns := parsers.ByAgent(withoutAddedRows(parsers.All(), filter))[entry.agent]
			if len(standIns) != 1 || fmt.Sprintf("%T", standIns[0]) == fmt.Sprintf("%T", registered[entry.agent][0]) {
				t.Fatalf("the agent's parsers are %T, not one stand-in", standIns)
			}
			artifact := &input.Artifact{Original: "/h/f"}
			var kept []timeline.Row
			emit := filter.emit(entry.agent, artifact, func(row timeline.Row) { kept = append(kept, row) })
			row := timeline.Row{TurnType: timeline.TurnSystem, SourceFile: "/h/f", Text: entry.text}
			other := row
			other.SourceFile = "/h/g"
			for _, each := range []timeline.Row{other, row, row} {
				emit(each)
			}
			if !slices.Equal(kept, []timeline.Row{other, row}) || filter.taken != 1 {
				t.Errorf("kept %v and took %d, want the other file's row and the second", kept, filter.taken)
			}
		})
	}
}
