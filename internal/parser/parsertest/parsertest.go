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
// their URLs replaced, each where it reaches (internal/secrets), as a
// separate, plain search gives them, with any placeholder, the empty one
// included, and in every string of a row; no recorded secret of
// secrets.EverywhereLength bytes or more may remain in them; and a parse
// run by Parse or Run must identify a secret, one run by ParseWithoutSecrets
// or RunWithoutSecrets none. Only tests import it.
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
	return run(test, artifact, func(inner parser.Emit) error { return each.Parse(artifact, options, inner) }, emit,
		true)
}

// ParseWithoutSecrets runs one parse of artifact by each as RunWithoutSecrets
// does.
func ParseWithoutSecrets(test testing.TB, each parser.Parser, artifact *input.Artifact, options parser.Options,
	emit parser.Emit) error {
	test.Helper()
	return run(test, artifact, func(inner parser.Emit) error { return each.Parse(artifact, options, inner) }, emit,
		false)
}

// Run runs produce as one parse of a run whose recorder is the artifact's
// home's (none shows secrets), then passes emit each row produce emitted as
// the output writes it, once the parse has ended and so every secret it
// identifies is recorded. It returns produce's error. Under a recorder it
// checks the rule of the package comment, and that the parse identifies a
// secret or its rows hold a URL credential, and fails test where either
// breaks.
func Run(test testing.TB, artifact *input.Artifact, produce func(emit parser.Emit) error, emit parser.Emit) error {
	test.Helper()
	return run(test, artifact, produce, emit, true)
}

// RunWithoutSecrets is Run for a fixture that holds no secret: under a
// recorder the parse must identify nothing and its rows hold no URL
// credential.
func RunWithoutSecrets(test testing.TB, artifact *input.Artifact, produce func(emit parser.Emit) error,
	emit parser.Emit) error {
	test.Helper()
	return run(test, artifact, produce, emit, false)
}

// run is Run, checking that the parse finds a secret when holdsSecrets
// and none when not.
func run(test testing.TB, artifact *input.Artifact, produce func(emit parser.Emit) error, emit parser.Emit,
	holdsSecrets bool) error {
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
		broken := slices.Concat(bothModes(shown, rows), written(recorder, found, rows))
		if identified := len(found.Secrets()) > 0 || holdsURLCredential(rows); identified != holdsSecrets {
			broken = append(broken, fmt.Sprintf("the parse identified a secret: %v, want %v (recorded %q)",
				identified, holdsSecrets, found.Secrets()))
		}
		for _, message := range broken {
			test.Error(message)
		}
	}
	for _, row := range rows {
		emit(row.Withheld(recorder))
	}
	return err
}

// holdsURLCredential reports whether a string of rows holds a URL's
// credential.
func holdsURLCredential(rows []timeline.Row) bool {
	for _, row := range rows {
		for _, field := range row.Strings() {
			if len(secrets.URLCredentials(field)) > 0 {
				return true
			}
		}
	}
	return false
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
// the credentials of its URLs where they stand and the texts the run
// replaces everywhere (the secrets of secrets.EverywhereLength bytes or
// more: this parse's values, the URL credentials of its rows and those
// other parses recorded); in the text alone, the parse's shorter values and
// its readers' details too. No long secret of this parse may remain.
func written(recorder *secrets.Recorder, found *secrets.Found, rows []timeline.Row) []string {
	var broken, long []string
	placeholder := recorder.Setting().Placeholder
	empty := placeholder == ""
	for _, value := range found.Values() {
		if len(value) >= secrets.EverywhereLength {
			long = append(long, value)
		}
	}
	everywhere := recorder.EverywhereTexts()
	for _, row := range rows {
		for _, field := range row.Strings() {
			long, everywhere = urlCredentialTexts(field, empty, long, everywhere)
		}
	}
	everywhere = slices.Concat(everywhere, long)
	inText := slices.Concat(everywhere, found.Values(), found.ReaderDetails())
	if empty {
		for _, detail := range found.ReaderDetails() {
			inText = append(inText, ": "+detail)
		}
	}
	for _, row := range rows {
		raw, output := row.Strings(), row.Withheld(recorder).Strings()
		for i, field := range raw {
			searched := everywhere
			if i == len(raw)-1 {
				searched = inText
			}
			if want := replaced(field, searched, secrets.URLCredentialRanges(field, empty), placeholder); output[i] !=
				want {
				broken = append(broken, fmt.Sprintf("%q is written %q, want %q (searched %q)", field, output[i], want,
					searched))
			}
			for _, secret := range long {
				if strings.Contains(output[i], secret) && !strings.Contains(placeholder, secret) {
					broken = append(broken, fmt.Sprintf("%q is written %q, which holds %q", field, output[i], secret))
				}
			}
		}
	}
	return broken
}

// urlCredentialTexts adds to long the URL credentials of text of
// secrets.EverywhereLength bytes or more, and to everywhere each of them as
// an empty placeholder matches it: user information with its @ too, told
// from another credential by the range an empty placeholder replaces.
func urlCredentialTexts(text string, empty bool, long, everywhere []string) ([]string, []string) {
	plain, withParts := secrets.URLCredentialRanges(text, false), secrets.URLCredentialRanges(text, true)
	for i, each := range plain {
		if each[1]-each[0] < secrets.EverywhereLength {
			continue
		}
		long = append(long, text[each[0]:each[1]])
		if empty && withParts[i][1] != each[1] {
			everywhere = append(everywhere, text[withParts[i][0]:withParts[i][1]])
		}
	}
	return long, everywhere
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
