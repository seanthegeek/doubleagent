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
	marks := timeline.MarkAgents{}
	if left, right := timeline.Summarise(shown, marks), timeline.Summarise(rows, marks); !reflect.DeepEqual(left,
		right) {
		broken = append(broken, fmt.Sprintf("the sessions differ between the modes:\nshown    %+v\nwithheld %+v",
			left, right))
	}
	return broken
}

// written returns what breaks the rule that a written row is its raw row
// with exactly these replaced, found by a plain search: in every string,
// the credentials of its URLs where they stand and the secrets of
// secrets.EverywhereLength bytes or more (the parse's values and the URL
// credentials of every row); in the text alone, the parse's shorter values
// and its readers' details too. No long secret may remain.
func written(recorder *secrets.Recorder, found *secrets.Found, rows []timeline.Row) []string {
	var broken, everywhere []string
	for _, value := range found.Values() {
		if len(value) >= secrets.EverywhereLength {
			everywhere = append(everywhere, value)
		}
	}
	for _, row := range rows {
		for _, field := range row.Strings() {
			for _, credential := range secrets.URLCredentials(field) {
				if len(credential) >= secrets.EverywhereLength {
					everywhere = append(everywhere, credential)
				}
			}
		}
	}
	inText := slices.Concat(everywhere, found.Values(), found.ReaderDetails())
	placeholder := recorder.Setting().Placeholder
	for _, row := range rows {
		raw, output := row.Strings(), row.Withheld(recorder).Strings()
		for i, field := range raw {
			searched := everywhere
			if i == len(raw)-1 {
				searched = inText
			}
			want := replaced(field, searched, secrets.URLCredentialRanges(field, false), placeholder)
			if placeholder != "" && output[i] != want {
				broken = append(broken, fmt.Sprintf("%q is written %q, want %q (searched %q)", field, output[i], want,
					searched))
			}
			for _, secret := range everywhere {
				if strings.Contains(output[i], secret) && !strings.Contains(placeholder, secret) {
					broken = append(broken, fmt.Sprintf("%q is written %q, which holds %q", field, output[i], secret))
				}
			}
		}
	}
	return broken
}

// replaced returns text with every occurrence of each of secrets and each
// of ranges ([start, end) byte ranges) replaced by placeholder, the
// occurrences found by comparing each secret at each position: ranges that
// share a byte are one, replaced once, and two that only touch are two.
func replaced(text string, secrets []string, ranges [][2]int, placeholder string) string {
	for i := range len(text) {
		for _, secret := range secrets {
			if secret != "" && strings.HasPrefix(text[i:], secret) {
				ranges = append(ranges, [2]int{i, i + len(secret)})
			}
		}
	}
	slices.SortFunc(ranges, func(left, right [2]int) int { return left[0] - right[0] })
	var out strings.Builder
	written, end := 0, -1
	for _, each := range ranges {
		if each[0] >= end {
			out.WriteString(text[written:each[0]] + placeholder)
		}
		end = max(end, each[1])
		written = end
	}
	out.WriteString(text[written:])
	return out.String()
}
