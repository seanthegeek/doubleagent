package parsertest

import (
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/secrets"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// rowsOf runs produce once showing secrets and once withholding them as
// REDACTED, and returns the rows of each, the second raw, and its record.
func rowsOf(produce func(artifact *input.Artifact, emit parser.Emit)) ([]timeline.Row, []timeline.Row,
	*secrets.Recorder, *secrets.Found) {
	artifact := &input.Artifact{Home: &input.Home{}}
	var shown, rows []timeline.Row
	produceOf := func(emit parser.Emit) error { produce(artifact, emit); return nil }
	_, _ = parser.Within(artifact, produceOf, func(row timeline.Row) { shown = append(shown, row) })
	recorder := secrets.NewRecorder(secrets.Setting{Withhold: true, Placeholder: secrets.DefaultPlaceholder})
	artifact.Home.Secrets = recorder
	found, _ := parser.Within(artifact, produceOf, func(row timeline.Row) { rows = append(rows, row) })
	return shown, rows, recorder, found
}

func TestTheRuleOfBothModesRefusesAParserThatTellsThemApart(t *testing.T) {
	tests := []struct {
		name    string
		produce func(artifact *input.Artifact, emit parser.Emit)
		broken  bool
	}{
		{"a parser that identifies its secret passes", func(artifact *input.Artifact, emit parser.Emit) {
			emit(timeline.Row{SessionID: "s", Text: "key=" + parser.SecretText(artifact, "sk-FIXTURE")})
		}, false},
		{"a parser that hides a value itself fails", func(artifact *input.Artifact, emit parser.Emit) {
			text := "key=sk-FIXTURE"
			if artifact.Home.Secrets != nil {
				text = "key=REDACTED"
			}
			emit(timeline.Row{SessionID: "s", Text: text})
		}, true},
		{"a parser that drops a row when withholding fails", func(artifact *input.Artifact, emit parser.Emit) {
			if artifact.Home.Secrets == nil {
				emit(timeline.Row{SessionID: "s", Text: "x"})
			}
		}, true},
		{"a parser whose session id changes with the mode fails", func(artifact *input.Artifact, emit parser.Emit) {
			session := "s"
			if artifact.Home.Secrets != nil {
				session = "t"
			}
			emit(timeline.Row{SessionID: session, Text: "x"})
		}, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			shown, rows, recorder, found := rowsOf(test.produce)
			broken := append(bothModes(shown, rows), written(recorder, found, rows)...)
			if (len(broken) > 0) != test.broken {
				t.Errorf("broken %q, want broken %v", broken, test.broken)
			}
		})
	}
}

func TestTheWrittenRowIsCheckedAgainstAPlainSearch(t *testing.T) {
	recorder := secrets.NewRecorder(secrets.Setting{Withhold: true, Placeholder: "P"})
	found := recorder.Begin()
	recorder.Value("user:password")
	recorder.Value("password")
	recorder.End()
	rows := []timeline.Row{{Text: "user:password password", Secrets: found}}
	if broken := written(recorder, found, rows); broken != nil {
		t.Errorf("broken %q", broken)
	}
	if got := replaced("user:password password", found.Secrets(), "P"); got != "P P" {
		t.Errorf("the plain search gave %q", got)
	}
	unrecorded := []timeline.Row{{Text: "sk-FIXTURE-LONG", Secrets: &secrets.Found{}}}
	if broken := written(recorder, &secrets.Found{}, unrecorded); broken != nil {
		t.Errorf("a row of nothing recorded is broken: %q", broken)
	}
}
