package agentzero

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parser/parsertest"
	"github.com/seanthegeek/doubleagent/internal/secrets"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// secretMode is one way a run shows secrets: by default, or withheld as
// the default placeholder, another, or an empty one, with the text a
// secret is shown as under it.
type secretMode struct {
	name    string
	setting secrets.Setting
	shown   func(secret string) string
}

// secretModes returns the default and the three ways of withholding.
func secretModes() []secretMode {
	return []secretMode{
		{"shown by default", secrets.Setting{}, func(secret string) string { return secret }},
		{"withheld as REDACTED", secrets.Setting{Withhold: true, Placeholder: secrets.DefaultPlaceholder},
			func(string) string { return "REDACTED" }},
		{"withheld as another placeholder", secrets.Setting{Withhold: true, Placeholder: "[withheld]"},
			func(string) string { return "[withheld]" }},
		{"withheld as an empty placeholder", secrets.Setting{Withhold: true}, func(string) string { return "" }},
	}
}

// parseWith writes text at rel under a new home and returns the text of
// every row the parser gives for it in a run that shows secrets as setting
// says.
func parseWith(t *testing.T, setting secrets.Setting, rel, text string) []string {
	t.Helper()
	home := t.TempDir()
	path := filepath.Join(home, filepath.FromSlash(rel))
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := &input.Artifact{DiskPath: path, Original: "/home/u/" + rel, Rel: rel, Agent: "agent-zero",
		Home:     &input.Home{DiskPath: home, Original: "/home/u", User: "u", Host: "h", Secrets: secrets.NewRecorder(setting)},
		Manifest: jsonvalue.NewNull()}
	var texts []string
	if err := parsertest.Parse(t, New(), artifact, parser.Options{}, func(row timeline.Row) { texts = append(texts, row.Text) }); err != nil {
		t.Fatal(err)
	}
	return texts
}

// checkWithheld fails the test when a run that withholds secrets gave a
// row holding a secret of the fixtures, each of which holds FIXTURE.
func checkWithheld(t *testing.T, mode secretMode, texts []string) {
	t.Helper()
	if mode.setting.Withhold && strings.Contains(strings.Join(texts, "\n"), "FIXTURE") {
		t.Errorf("a withheld run shows a secret: %q", texts)
	}
}

func TestEveryConnectorSettingIsShownUnlessWithheld(t *testing.T) {
	const env = "AGENT_ZERO_HOST=http://localhost:50080\n" +
		"AGENT_ZERO_COMPUTER_USE_RESTORE_TOKEN=FIXTURE-TOKEN\n" +
		"not a setting\n" +
		"AGENT_ZERO_LAST_CONTEXT_HOST=https://ci:FIXTURE-PASSWORD@a0.example.net\n" +
		"AGENT_ZERO_LAST_CONTEXT_ID='RemCtx01'\n"
	for _, mode := range secretModes() {
		t.Run(mode.name, func(t *testing.T) {
			got := parseWith(t, mode.setting, ".agent-zero/.env", env)
			want := []string{
				"connector settings: AGENT_ZERO_HOST=" + mode.shown("http://localhost:50080") +
					" AGENT_ZERO_COMPUTER_USE_RESTORE_TOKEN=" + mode.shown("FIXTURE-TOKEN") +
					" AGENT_ZERO_LAST_CONTEXT_HOST=" + mode.shown("https://ci:FIXTURE-PASSWORD@a0.example.net") +
					" AGENT_ZERO_LAST_CONTEXT_ID=" + mode.shown("RemCtx01") + " (from .agent-zero/.env, file mtime)",
				"session origin: remote agent-zero instance " + mode.shown("https://ci:FIXTURE-PASSWORD@a0.example.net") + " context " +
					mode.shown("RemCtx01") + " (from .agent-zero/.env, file mtime)",
			}
			if !slices.Equal(got, want) {
				t.Errorf("got %q, want %q", got, want)
			}
			checkWithheld(t, mode, got)
		})
	}
}

func TestAChatsBotTokenAndModelKeyAreShownUnlessWithheld(t *testing.T) {
	const chat = `{"id": "ctx", "name": "c", "created_at": "2026-01-01T00:00:00Z", "agents": [], "logs": [], ` +
		`"data": {"telegram_bot": "opsbot", "telegram_chat_id": 7, "telegram_user_id": 8, ` +
		`"telegram_username": "ana", "telegram_bot_cfg": {"name": "opsbot", "token": "FIXTURE-BOT-TOKEN", ` +
		`"webhook_secret": "FIXTURE-WEBHOOK"}, "chat_model_override": {"chat": {"provider": "openrouter", ` +
		`"name": "m", "api_key": "sk-FIXTURE", "api_base": "https://ci:FIXTURE-PASSWORD@llm.example/v1"}, ` +
		`"utility": {"provider": "ollama", "name": "q"}}}}`
	bases := []string{"https://ci:FIXTURE-PASSWORD@", "https://REDACTED@", "https://[withheld]@", "https://"}
	for i, mode := range secretModes() {
		t.Run(mode.name, func(t *testing.T) {
			var got []string
			for _, text := range parseWith(t, mode.setting, "agent-zero/usr/chats/ctx/chat.json", chat) {
				if strings.HasPrefix(text, "session relayed:") || strings.HasPrefix(text, "model:") {
					got = append(got, text)
				}
			}
			want := []string{
				"session relayed: telegram bot opsbot chat 7 user 8 @ana token=" + mode.shown("FIXTURE-BOT-TOKEN") +
					" webhook_secret=" + mode.shown("FIXTURE-WEBHOOK") + " (from chat.json data, file mtime)",
				"model: requested chat=openrouter/m chat.api_key=" + mode.shown("sk-FIXTURE") + " chat.api_base=" +
					bases[i] + "llm.example/v1 utility=ollama/q (chat override, as of file mtime)",
			}
			if !slices.Equal(got, want) {
				t.Errorf("got %q, want %q", got, want)
			}
			checkWithheld(t, mode, got)
		})
	}
}

func TestATopLevelModelConfigurationShowsItsKeyAndBase(t *testing.T) {
	override := `{"provider": "openai", "name": "gpt", "api_key": "sk-FIXTURE", "api_base": "https://h/v1"}`
	want := "openai/gpt api_key=sk-FIXTURE api_base=https://h/v1"
	if got := overrideText(nil, decoded(t, override)); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
