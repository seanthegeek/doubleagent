package twinny

import (
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parser/parsertest"
	"github.com/seanthegeek/doubleagent/internal/secrets"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

func TestAProvidersAPIKeyIsShownUnlessWithheld(t *testing.T) {
	const global = `{"twinny.inference-providers": {"p1": {"id": "p1", "label": "Ollama", "provider": "ollama", ` +
		`"type": "chat", "modelName": "qwen", "apiKey": "sk-FIXTURE"}, "p2": {"id": "p2", "label": "Local"}}, ` +
		`"twinny.active-chat-provider": {"id": "p1", "apiKey": "sk-FIXTURE"}}`
	tests := []struct {
		name    string
		setting secrets.Setting
		key     string
	}{
		{"shown by default", secrets.Setting{}, "sk-FIXTURE"},
		{"withheld as REDACTED", secrets.Setting{Withhold: true, Placeholder: secrets.DefaultPlaceholder}, "REDACTED"},
		{"withheld as another placeholder", secrets.Setting{Withhold: true, Placeholder: "[withheld]"}, "[withheld]"},
		{"withheld as an empty placeholder", secrets.Setting{Withhold: true}, ""},
	}
	state, err := jsonvalue.DecodeString(global)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			artifact := writeArtifact(t, "User/globalStorage/state.vscdb", "")
			artifact.Home.Secrets = secrets.NewRecorder(test.setting)
			var got []string
			_ = parsertest.Run(t, artifact, func(emit parser.Emit) error {
				New().globalRows(artifact, state, parser.Options{}, emit)
				return nil
			}, func(row timeline.Row) { got = append(got, row.Text) })
			want := []string{
				"model provider: p1 label=Ollama provider=ollama type=chat model=qwen apiKey=" + test.key +
					" (from rjmacarthy.twinny twinny.inference-providers, file mtime)",
				"model provider: active chat p1 apiKey=" + test.key +
					" (from rjmacarthy.twinny twinny.active-chat-provider, file mtime)",
			}
			if !slices.Equal(got, want) {
				t.Errorf("got %q, want %q", got, want)
			}
			if test.setting.Withhold && strings.Contains(strings.Join(got, "\n"), "FIXTURE") {
				t.Errorf("a withheld run shows the key: %q", got)
			}
		})
	}
}
