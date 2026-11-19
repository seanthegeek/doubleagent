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
