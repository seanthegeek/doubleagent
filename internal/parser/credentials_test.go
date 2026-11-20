package parser

import (
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/secrets"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

func TestTheKitReturnsTheTextAsWrittenAndTheOutputReplacesWhatItRecords(t *testing.T) {
	_, jsonError := jsonvalue.DecodeString(`{"api_key": sk-1}`)
	withheld := secrets.Setting{Withhold: true, Placeholder: "[x]"}
	tests := []struct {
		name     string
		home     *input.Home
		recorded []string
		texts    []string
	}{
		{"an artifact without a home records nothing", nil, nil,
			[]string{"sk-1", "https://u:pw7@h/", jsonError.Error()}},
		{"a run that shows secrets records nothing", &input.Home{}, nil,
			[]string{"sk-1", "https://u:pw7@h/", jsonError.Error()}},
		{"a run that withholds records each and the output replaces it",
			&input.Home{Secrets: secrets.NewRecorder(withheld)},
			[]string{"invalid character 's' looking for beginning of value", "sk-1"},
			[]string{"[x]", "https://[x]@h/", "invalid JSON at line 1, column 13: [x]"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			artifact := &input.Artifact{Home: test.home}
			var rows []timeline.Row
			found, err := Within(artifact, func(emit Emit) error {
				for _, text := range []string{SecretText(artifact, "sk-1"), "https://u:pw7@h/",
					ReaderErrorText(artifact, jsonError)} {
					emit(timeline.Row{Text: text})
				}
				return nil
			}, func(row timeline.Row) { rows = append(rows, row) })
			if err != nil {
				t.Fatal(err)
			}
			raw := []string{"sk-1", "https://u:pw7@h/", jsonError.Error()}
			var recorder *secrets.Recorder
			if test.home != nil {
				recorder = test.home.Secrets
			}
			var texts []string
			for i, row := range rows {
				if row.Text != raw[i] || row.Secrets != found {
					t.Errorf("row %d: %q with record %p, want %q with %p", i, row.Text, row.Secrets, raw[i], found)
				}
				texts = append(texts, row.Withheld(recorder).Text)
			}
			if !slices.Equal(found.Secrets(), test.recorded) || !slices.Equal(texts, test.texts) {
				t.Errorf("recorded %q and wrote %q, want %q and %q", found.Secrets(), texts, test.recorded, test.texts)
			}
		})
	}
}

func TestEveryFormOfASecretAParserWritesIsRecorded(t *testing.T) {
	headers, err := jsonvalue.DecodeString(`{"Authorization": "Bearer fake-header-secret-0001", ` +
		`"X-API-Key": "fake-\"quoted\"-0002"}`)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name     string
		identify func(artifact *input.Artifact) string
		recorded []string
	}{
		{"a value with white space around it", func(artifact *input.Artifact) string {
			return SecretText(artifact, "  fake-secret-value-0001 ")
		}, []string{"  fake-secret-value-0001 ", "fake-secret-value-0001"}},
		{"an authorization value and its credentials", func(artifact *input.Artifact) string {
			return SecretText(artifact, "Bearer fake-secret-value-0001")
		}, []string{"Bearer fake-secret-value-0001", "fake-secret-value-0001"}},
		{"a basic authorization value in another case", func(artifact *input.Artifact) string {
			return SecretText(artifact, "basic ZmFrZTpzZWNyZXQ=")
		}, []string{"ZmFrZTpzZWNyZXQ=", "basic ZmFrZTpzZWNyZXQ="}},
		{"a headers object, each header and its JSON form", func(artifact *input.Artifact) string {
			return SecretJSONText(artifact, headers)
		}, []string{"Bearer fake-header-secret-0001", headers.JSON(), quotedInJSON(headers.JSON()),
			`fake-"quoted"-0002`, `fake-\"quoted\"-0002`, "fake-header-secret-0001"}},
		{"a value as compact JSON escapes it, in each form", func(artifact *input.Artifact) string {
			return SecretText(artifact, "Bearer fake-\"sec\\ret\u2028-0003 ")
		}, []string{"Bearer fake-\"sec\\ret\u2028-0003 ", "Bearer fake-\"sec\\ret\u2028-0003",
			"fake-\"sec\\ret\u2028-0003", `Bearer fake-\"sec\\ret\u2028-0003 `, `Bearer fake-\"sec\\ret\u2028-0003`,
			`fake-\"sec\\ret\u2028-0003`}},
		{"a value of white space alone is none", func(artifact *input.Artifact) string {
			return SecretText(artifact, " \t ")
		}, nil},
		{"a string from JSON as it is", func(artifact *input.Artifact) string {
			return SecretJSONText(artifact, jsonvalue.NewString("fake-secret-value-0001"))
		}, []string{"fake-secret-value-0001"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			artifact := &input.Artifact{Home: &input.Home{Secrets: secrets.NewRecorder(
				secrets.Setting{Withhold: true, Placeholder: "P"})}}
			found, _ := Within(artifact, func(Emit) error { test.identify(artifact); return nil }, func(timeline.Row) {})
			if got, want := found.Secrets(), slices.Sorted(slices.Values(test.recorded)); !slices.Equal(got, want) {
				t.Errorf("recorded %q, want %q", got, want)
			}
		})
	}
}

// quotedInJSON returns text as compact JSON writes it inside a string's
// quotes.
func quotedInJSON(text string) string {
	quoted := jsonvalue.NewString(text).JSON()
	return quoted[1 : len(quoted)-1]
}

func TestAHeadersObjectRecordsOnlyCredentialHeadersOnTheirOwn(t *testing.T) {
	headers, err := jsonvalue.DecodeString(`{"Authorization": "Bearer fake-header-secret-0001", ` +
		`"cookie": ["sid=fake-cookie-0004"], "Content-Type": "application/json", "X-Trace": "fake-trace-id-0005", ` +
		`"Access_Token": "fake-token-0006", "n": 12345678, "b": true}`)
	if err != nil {
		t.Fatal(err)
	}
	recorder := secrets.NewRecorder(secrets.Setting{Withhold: true, Placeholder: "P"})
	artifact := &input.Artifact{Home: &input.Home{Secrets: recorder}}
	found, _ := Within(artifact, func(Emit) error { SecretJSONText(artifact, headers); return nil },
		func(timeline.Row) {})
	everywhere, recorded := recorder.EverywhereTexts(), found.Secrets()
	for _, text := range []string{"fake-header-secret-0001", "sid=fake-cookie-0004", "fake-token-0006"} {
		if !slices.Contains(everywhere, text) {
			t.Errorf("%q, a credential header's value, is not replaced in every string", text)
		}
	}
	for _, text := range []string{"application/json", "fake-trace-id-0005"} {
		if slices.Contains(recorded, text) {
			t.Errorf("%q, a header that names no credential, is recorded on its own", text)
		}
		if got := recorder.RowText("port "+text, found); got != "port "+text {
			t.Errorf("the parse's row text gives %q", got)
		}
	}
	for _, text := range []string{"12345678", "true"} {
		if slices.Contains(recorded, text) {
			t.Errorf("the number or boolean %q is recorded", text)
		}
	}
}

func TestTheSecretItselfDecidesWhereEachOfItsFormsIsReplaced(t *testing.T) {
	tests := []struct {
		name, value string
		everywhere  []string
	}{
		{"a short value whose JSON form is long", `a"b"c"d`, nil},
		{"short control characters whose JSON form is long", "\x01\x02", nil},
		{"a short value with white space around it", "  abcd   ", nil},
		{"a short credential after a scheme", "Bearer abc1234", nil},
		{"a long credential after a scheme", "Bearer fake-0001", []string{"Bearer fake-0001", "fake-0001"}},
		{"a long value and its JSON form", `fake"q-01`, []string{`fake"q-01`, `fake\"q-01`}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := secrets.NewRecorder(secrets.Setting{Withhold: true, Placeholder: "P"})
			artifact := &input.Artifact{Home: &input.Home{Secrets: recorder}}
			found, _ := Within(artifact, func(Emit) error { SecretText(artifact, test.value); return nil },
				func(timeline.Row) {})
			if got := recorder.EverywhereTexts(); !slices.Equal(got, test.everywhere) {
				t.Errorf("replaced in every string: %q, want %q", got, test.everywhere)
			}
			row := `{"note":` + jsonvalue.NewString(test.value).JSON() + `}`
			if got := recorder.RowText(row, found); got != `{"note":"P"}` {
				t.Errorf("the parse's row %q is written %q", row, got)
			}
		})
	}
}

func TestAValueIsRecordedAsEachJSONWriterSpellsIt(t *testing.T) {
	tests := []struct {
		name, value string
		recorded    []string
	}{
		{"a slash", "fake/key-0001", []string{"fake/key-0001", `fake\/key-0001`}},
		{"the characters & < >", "fake&<key>-1", []string{"fake&<key>-1", `fake\u0026\u003Ckey\u003E-1`,
			`fake\u0026\u003ckey\u003e-1`}},
		{"a slash and a quote and <", `a/"b<cdefg`, []string{`a/"b<cdefg`, `a/\"b<cdefg`, `a\/\"b<cdefg`,
			`a/\"b\u003ccdefg`, `a/\"b\u003Ccdefg`, `a\/\"b\u003ccdefg`, `a\/\"b\u003Ccdefg`}},
		{"a backslash before a slash", `a\/b-fake-01`, []string{`a\/b-fake-01`, `a\\/b-fake-01`, `a\\\/b-fake-01`}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			artifact := &input.Artifact{Home: &input.Home{Secrets: secrets.NewRecorder(
				secrets.Setting{Withhold: true, Placeholder: "P"})}}
			found, _ := Within(artifact, func(Emit) error { SecretText(artifact, test.value); return nil },
				func(timeline.Row) {})
			if got, want := found.Secrets(), slices.Sorted(slices.Values(test.recorded)); !slices.Equal(got, want) {
				t.Errorf("recorded %q, want %q", got, want)
			}
		})
	}
}

func TestEverySpellingOfASecretReachesAsFarAsTheSecretItself(t *testing.T) {
	recorder := secrets.NewRecorder(secrets.Setting{Withhold: true, Placeholder: "P"})
	artifact := &input.Artifact{Home: &input.Home{Secrets: recorder}}
	// Short as written; its escaped spellings are longer than 8 bytes.
	found, _ := Within(artifact, func(Emit) error { SecretText(artifact, "a/b&c"); return nil }, func(timeline.Row) {})
	if got := recorder.EverywhereTexts(); got != nil {
		t.Errorf("replaced in every string: %q, want none", got)
	}
	if got := recorder.RowText(`a\/b\u0026c`, found); got != "P" {
		t.Errorf("the parse's row text gives %q", got)
	}
}

func TestAtMostTwentyOneTextsAreRecordedForOneValue(t *testing.T) {
	artifact := &input.Artifact{Home: &input.Home{Secrets: secrets.NewRecorder(
		secrets.Setting{Withhold: true, Placeholder: "P"})}}
	found, _ := Within(artifact, func(Emit) error {
		SecretText(artifact, ` Bearer fake/<&>"key-0001 `)
		return nil
	}, func(timeline.Row) {})
	if got := len(found.Secrets()); got > 21 {
		t.Errorf("%d texts recorded, want at most 21: %q", got, found.Secrets())
	}
}
