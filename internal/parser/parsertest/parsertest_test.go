package parsertest

import (
	"fmt"
	"strconv"
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
	if got := replaced("user:password password", found.Secrets(), nil, "P"); got != "P P" {
		t.Errorf("the plain search gave %q", got)
	}
	unrecorded := []timeline.Row{{Text: "sk-FIXTURE-LONG", Secrets: &secrets.Found{}}}
	if broken := written(recorder, &secrets.Found{}, unrecorded); broken != nil {
		t.Errorf("a row of nothing recorded is broken: %q", broken)
	}
}

func TestAWrittenRowIsComparedExactlyWithEveryPlaceholder(t *testing.T) {
	for _, placeholder := range []string{"P", ""} {
		t.Run("placeholder "+strconv.Quote(placeholder), func(t *testing.T) {
			recorder := secrets.NewRecorder(secrets.Setting{Withhold: true, Placeholder: placeholder})
			own := recorder.Begin()
			recorder.Value("pw7")
			recorder.End()
			other := recorder.Begin()
			recorder.End()
			row := timeline.Row{Text: "password pw7 at https://ci:pw8@h/", Model: "pw7", Secrets: own}
			if broken := written(recorder, own, []timeline.Row{row}); broken != nil {
				t.Errorf("a row written by the rule is broken: %q", broken)
			}
			// The same row carrying another parse's record is written with
			// the short secret kept, which the check refuses.
			row.Secrets = other
			if broken := written(recorder, own, []timeline.Row{row}); len(broken) != 1 {
				t.Errorf("a row that keeps a short secret gave %q, want one break", broken)
			}
		})
	}
}

// recording is a testing.TB that keeps the errors reported to it.
type recording struct {
	testing.TB
	errors []string
}

// Error keeps the error.
func (r *recording) Error(args ...any) { r.errors = append(r.errors, fmt.Sprint(args...)) }

// Helper marks nothing.
func (r *recording) Helper() {}

func TestAParseIsHeldToWhetherItsFixtureHoldsASecret(t *testing.T) {
	identifies := func(artifact *input.Artifact) func(parser.Emit) error {
		return func(emit parser.Emit) error {
			emit(timeline.Row{SessionID: "s", Text: "key=" + parser.SecretText(artifact, "fake-secret-value-0001")})
			return nil
		}
	}
	plain := func(*input.Artifact) func(parser.Emit) error {
		return func(emit parser.Emit) error { emit(timeline.Row{SessionID: "s", Text: "no secret"}); return nil }
	}
	tests := []struct {
		name    string
		run     func(testing.TB, *input.Artifact, func(parser.Emit) error, parser.Emit) error
		produce func(*input.Artifact) func(parser.Emit) error
		broken  bool
	}{
		{"a parse that identifies a secret passes Run", Run, identifies, false},
		{"a parse that identifies nothing fails Run", Run, plain, true},
		{"a parse that identifies nothing passes RunWithoutSecrets", RunWithoutSecrets, plain, false},
		{"a parse that identifies a secret fails RunWithoutSecrets", RunWithoutSecrets, identifies, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := secrets.NewRecorder(secrets.Setting{Withhold: true, Placeholder: "P"})
			artifact := &input.Artifact{Home: &input.Home{Secrets: recorder}}
			reporter := &recording{TB: t}
			_ = test.run(reporter, artifact, test.produce(artifact), func(timeline.Row) {})
			if (len(reporter.errors) > 0) != test.broken {
				t.Errorf("errors %q, want broken %v", reporter.errors, test.broken)
			}
		})
	}
}

func TestAValueOneParseIdentifiesIsReplacedInAnotherParsesRows(t *testing.T) {
	recorder := secrets.NewRecorder(secrets.Setting{Withhold: true, Placeholder: "P"})
	first := &input.Artifact{Home: &input.Home{User: "alice", Secrets: recorder}}
	second := &input.Artifact{Home: &input.Home{User: "bob", Secrets: recorder}}
	_ = Run(t, first, func(emit parser.Emit) error {
		emit(timeline.Row{SessionID: "s", Text: "key=" + parser.SecretText(first, "fake-secret-value-0001") +
			" pin=" + parser.SecretText(first, "pw7")})
		return nil
	}, func(timeline.Row) {})
	var written []timeline.Row
	_ = RunWithoutSecrets(t, second, func(emit parser.Emit) error {
		emit(timeline.Row{SessionID: "fake-secret-value-0001", Text: "saw fake-secret-value-0001 and pw7"})
		return nil
	}, func(row timeline.Row) { written = append(written, row) })
	if len(written) != 1 || written[0].Text != "saw P and pw7" || written[0].SessionID != "P" {
		t.Errorf("the other parse's row is written %+v", written)
	}
}
