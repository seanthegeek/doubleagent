package parsertest

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/secrets"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

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

// cutTails returns the tails of the values the kit cut that the output
// withholds, by a plain search of each whole value: where the first text
// the output replaces there (everywhere, or inText for a value in the text
// of this parse's rows) that runs past the kept bytes starts.
func cutTails(recorder *secrets.Recorder, found *secrets.Found, everywhere, inText []string) []cutTail {
	var tails []cutTail
	for _, each := range recorder.CutValues() {
		searched := everywhere
		if each.Parse != nil && each.Parse == found {
			searched = inText
		}
		for start := range each.Kept {
			if slices.ContainsFunc(searched, func(secret string) bool {
				return secret != "" && start+len(secret) > each.Kept && strings.HasPrefix(each.Value[start:], secret)
			}) {
				tails = append(tails, cutTail{each.Written[start:], each.Kept - start})
				break
			}
		}
	}
	return tails
}

// tailRanges returns the byte ranges of text, [start, end), that hold a
// tail's secret.
func tailRanges(text string, tails []cutTail) [][2]int {
	var ranges [][2]int
	for _, tail := range tails {
		for i := range len(text) {
			if strings.HasPrefix(text[i:], tail.text) {
				ranges = append(ranges, [2]int{i, i + tail.secret})
			}
		}
	}
	return ranges
}

// longURLCredentials adds to long the URL credentials of text that the
// output replaces in every string (secrets.URLCredentialEverywhere).
func longURLCredentials(text string, long []string) []string {
	for _, each := range secrets.URLCredentialRanges(text, false) {
		if credential := text[each[0]:each[1]]; secrets.URLCredentialEverywhere(credential) {
			long = append(long, credential)
		}
	}
	return long
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

// written returns what breaks the rule that a written row is its raw row
// with exactly these replaced, found by a plain search: in every string,
// the credentials of its URLs where they stand and the texts the run
// replaces everywhere (Recorder.EverywhereTexts), with this parse's values
// the run replaces everywhere (Found.EverywhereValues) and the URL
// credentials of its rows that reach every string
// (secrets.URLCredentialEverywhere), and the kept part of each secret a
// cut value ran through (cutTails); in the text alone, the parse's other
// values and its readers' details too. Those long ones may not remain.
func written(recorder *secrets.Recorder, found *secrets.Found, rows []timeline.Row) []string {
	var broken []string
	placeholder := recorder.Setting().Placeholder
	empty := placeholder == ""
	long := found.EverywhereValues()
	for _, row := range rows {
		for _, field := range row.Strings() {
			long = longURLCredentials(field, long)
		}
	}
	everywhere := slices.Concat(recorder.EverywhereTexts(), long)
	inText := slices.Concat(everywhere, found.Values(), found.ReaderDetails())
	if empty {
		for _, detail := range found.ReaderDetails() {
			inText = append(inText, ": "+detail)
		}
	}
	tails := cutTails(recorder, found, everywhere, inText)
	for _, row := range rows {
		raw, output := row.Strings(), row.Withheld(recorder).Strings()
		for i, field := range raw {
			searched := everywhere
			if i == len(raw)-1 {
				searched = inText
			}
			ranges := slices.Concat(secrets.URLCredentialRanges(field, empty), tailRanges(field, tails))
			if want := replaced(field, searched, ranges, placeholder); output[i] != want {
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

// cutTail is the end of a cut value's written text from the start of a
// secret the cut ran through, and how many of its bytes are that secret.
type cutTail struct {
	text   string
	secret int
}

// run is Run, checking that the parse finds a secret when holdsSecrets
// and none when not.
func run(test testing.TB, artifact *input.Artifact, produce func(emit parserkit.Emit) error, emit parserkit.Emit,
	holdsSecrets bool) error {
	test.Helper()
	var recorder *secrets.Recorder
	if artifact.Home != nil {
		recorder = artifact.Home.Secrets
	}
	var shown []timeline.Row
	if recorder != nil {
		artifact.Home.Secrets = nil
		_, _ = parserkit.Within(artifact, produce, func(row timeline.Row) { shown = append(shown, row) })
		artifact.Home.Secrets = recorder
		timeline.Resolve(shown)
	}
	var rows []timeline.Row
	found, err := parserkit.Within(artifact, produce, func(row timeline.Row) { rows = append(rows, row) })
	timeline.Resolve(rows)
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

// Parse runs one parse of artifact by each as Run does.
func Parse(test testing.TB, each parserkit.Parser, artifact *input.Artifact, options parserkit.Options,
	emit parserkit.Emit) error {
	test.Helper()
	return run(test, artifact, func(inner parserkit.Emit) error { return each.Parse(artifact, options, inner) }, emit,
		true)
}

// ParseWithoutSecrets runs one parse of artifact by each as Parse does, for
// a fixture that holds no secret: under a recorder the parse must identify
// nothing and its rows hold no URL credential.
func ParseWithoutSecrets(test testing.TB, each parserkit.Parser, artifact *input.Artifact, options parserkit.Options,
	emit parserkit.Emit) error {
	test.Helper()
	return run(test, artifact, func(inner parserkit.Emit) error { return each.Parse(artifact, options, inner) }, emit,
		false)
}

// Run runs produce as one parse of a run whose recorder is the artifact's
// home's (none shows secrets), then passes emit each row produce emitted as
// the output writes it, once the parse has ended and so every secret it
// identifies is recorded. It returns produce's error. Under a recorder it
// checks the rule of the package comment, and that the parse identifies a
// secret or its rows hold a URL credential, and fails test where either
// breaks.
func Run(test testing.TB, artifact *input.Artifact, produce func(emit parserkit.Emit) error, emit parserkit.Emit) error {
	test.Helper()
	return run(test, artifact, produce, emit, true)
}
