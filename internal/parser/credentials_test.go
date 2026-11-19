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
		`"X-Key": "fake-\"quoted\"-0002"}`)
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
		}, []string{"Bearer fake-header-secret-0001", headers.JSON(), `fake-"quoted"-0002`,
			`fake-\"quoted\"-0002`, "fake-header-secret-0001"}},
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
