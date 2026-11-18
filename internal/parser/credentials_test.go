package parser

import (
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/secrets"
)

func TestTheKitReadsTheSettingFromTheArtifactsHome(t *testing.T) {
	_, jsonError := jsonvalue.DecodeString(`{"api_key": sk-1}`)
	withheld := &input.Home{Secrets: secrets.Setting{Withhold: true, Placeholder: "[x]"}}
	tests := []struct {
		name                    string
		artifact                *input.Artifact
		value, url, readerError string
	}{
		{"an artifact without a home shows", &input.Artifact{}, "sk-1", "https://u:p@h/", jsonError.Error()},
		{"a home of the zero setting shows", &input.Artifact{Home: &input.Home{}}, "sk-1", "https://u:p@h/",
			jsonError.Error()},
		{"a home that withholds gives its placeholder", &input.Artifact{Home: withheld}, "[x]", "https://[x]@h/",
			"invalid JSON at line 1, column 13: [x]"},
		{"no artifact shows", nil, "sk-1", "https://u:p@h/", jsonError.Error()},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := SecretText(test.artifact, "sk-1"); got != test.value {
				t.Errorf("SecretText gave %q, want %q", got, test.value)
			}
			if got := URLText(test.artifact, "https://u:p@h/"); got != test.url {
				t.Errorf("URLText gave %q, want %q", got, test.url)
			}
			if got := ReaderErrorText(test.artifact, jsonError); got != test.readerError {
				t.Errorf("ReaderErrorText gave %q, want %q", got, test.readerError)
			}
		})
	}
}
