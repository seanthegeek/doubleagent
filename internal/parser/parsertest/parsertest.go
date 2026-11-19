// Package parsertest runs a parser in a test as an analysis run does under
// a secrets setting: the parse records what the kit identifies in the
// recorder of the artifact's home, and the test receives the rows as
// timeline.jsonl writes them (timeline.Row.Withheld), so a parser's test
// of a withheld secret checks the text the output holds.
//
// A run that withholds also proves, for every parse a test makes this way,
// the rule the design rests on: the parse is run a second time showing
// secrets, and the rows must be the same in both modes, field for field
// and in order, and so must the sessions built from them; the rows written
// must be the rows with exactly the recorded secrets and the credentials of
// their URLs replaced, as a separate, plain search gives them; and no recorded secret of
// secrets.EverywhereLength bytes or more may remain in them. Only tests
// import it.
package parsertest

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/secrets"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// Parse runs one parse of artifact by each as Run does.
func Parse(test testing.TB, each parser.Parser, artifact *input.Artifact, options parser.Options,
	emit parser.Emit) error {
	test.Helper()
	return Run(test, artifact, func(inner parser.Emit) error { return each.Parse(artifact, options, inner) }, emit)
}

// Run runs produce as one parse of a run whose recorder is the artifact's
// home's (none shows secrets), then passes emit each row produce emitted as
// the output writes it, once the parse has ended and so every secret it
// identifies is recorded. It returns produce's error. Under a recorder it
// checks the rule of the package comment and fails test where it breaks.
func Run(test testing.TB, artifact *input.Artifact, produce func(emit parser.Emit) error, emit parser.Emit) error {
	test.Helper()
	var recorder *secrets.Recorder
	if artifact.Home != nil {
		recorder = artifact.Home.Secrets
	}
	var shown []timeline.Row
	if recorder != nil {
		artifact.Home.Secrets = nil
		_, _ = parser.Within(artifact, produce, func(row timeline.Row) { shown = append(shown, row) })
		artifact.Home.Secrets = recorder
	}
	var rows []timeline.Row
	found, err := parser.Within(artifact, produce, func(row timeline.Row) { rows = append(rows, row) })
	for _, row := range rows {
		row.Collect(recorder)
	}
	if recorder != nil {
		for _, message := range slices.Concat(bothModes(shown, rows), written(recorder, found, rows)) {
			test.Error(message)
		}
	}
	for _, row := range rows {
		emit(row.Withheld(recorder))
	}
	return err
}

// bothModes returns what breaks the rule that the rows of a withheld run
// are the rows of the run that shows secrets, and so are their sessions.
func bothModes(shown, rows []timeline.Row) []string {
	var broken []string
	raw := make([]timeline.Row, len(rows))
	for i, row := range rows {
		row.Secrets = nil
		raw[i] = row
	}
	if !slices.Equal(raw, shown) {
		broken = append(broken, fmt.Sprintf("the rows differ between the modes:\nshown    %+v\nwithheld %+v",
			shown, raw))
	}
	withoutRecords := func(sessions []timeline.SessionSummary) []timeline.SessionSummary {
		for i := range sessions {
			sessions[i].Secrets = nil
		}
		return sessions
	}
	marks := timeline.MarkAgents{}
	if left, right := withoutRecords(timeline.Summarise(shown, marks)),
		withoutRecords(timeline.Summarise(rows, marks)); !reflect.DeepEqual(left, right) {
		broken = append(broken, fmt.Sprintf("the sessions differ between the modes:\nshown    %+v\nwithheld %+v",
			left, right))
	}
	return broken
}

// written returns what breaks the rule that a written row is its raw row
// with exactly the parse's recorded secrets and the credentials of URLs
// replaced (those in the row, and the long ones of every row), holding no
// long one.
func written(recorder *secrets.Recorder, found *secrets.Found, rows []timeline.Row) []string {
	var broken []string
	recorded := found.Secrets()
	for _, row := range rows {
		for _, field := range []string{row.Text, row.SessionID, row.ProjectPath, row.SourceFile} {
			for _, credential := range secrets.URLCredentials(field) {
				if len(credential) >= secrets.EverywhereLength {
					recorded = append(recorded, credential)
				}
			}
		}
	}
	var everywhere []string
	for _, secret := range recorded {
		if len(secret) >= secrets.EverywhereLength {
			everywhere = append(everywhere, secret)
		}
	}
	placeholder := recorder.Setting().Placeholder
	for _, row := range rows {
		output := row.Withheld(recorder)
		for i, raw := range []string{row.Text, row.SessionID, row.ProjectPath, row.SourceFile} {
			got := []string{output.Text, output.SessionID, output.ProjectPath, output.SourceFile}[i]
			want := replaced(raw, slices.Concat(recorded, secrets.URLCredentials(raw)), placeholder)
			if placeholder != "" && got != want {
				broken = append(broken, fmt.Sprintf("%q is written %q, want %q (recorded %q)", raw, got, want,
					recorded))
			}
			for _, secret := range everywhere {
				if strings.Contains(got, secret) && !strings.Contains(placeholder, secret) {
					broken = append(broken, fmt.Sprintf("%q is written %q, which holds %q", raw, got, secret))
				}
			}
		}
	}
	return broken
}

// replaced returns text with each of secrets replaced by placeholder, the
// longest at the leftmost position first, found by comparing each in turn.
func replaced(text string, secrets []string, placeholder string) string {
	var out strings.Builder
	for i := 0; i < len(text); {
		longest := 0
		for _, secret := range secrets {
			if strings.HasPrefix(text[i:], secret) && len(secret) > longest {
				longest = len(secret)
			}
		}
		if longest == 0 {
			out.WriteByte(text[i])
			i++
			continue
		}
		out.WriteString(placeholder)
		i += longest
	}
	return out.String()
}
