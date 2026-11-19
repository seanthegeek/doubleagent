package hermes

import (
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/secrets"
)

// secretMode is one way a run shows secrets: by default, or withheld as
// the default placeholder, another, or an empty one.
type secretMode struct {
	name    string
	setting secrets.Setting
}

// secretModes returns the default and the three ways of withholding.
func secretModes() []secretMode {
	return []secretMode{
		{"shown by default", secrets.Setting{}},
		{"withheld as REDACTED", secrets.Setting{Withhold: true, Placeholder: secrets.DefaultPlaceholder}},
		{"withheld as another placeholder", secrets.Setting{Withhold: true, Placeholder: "[withheld]"}},
		{"withheld as an empty placeholder", secrets.Setting{Withhold: true}},
	}
}

func TestAFileHoldingSecretsThatIsNotJSONGivesTheReadersDetailUnlessWithheld(t *testing.T) {
	tests := []struct {
		name, rel, text, position, detail string
	}{
		{"the desktop app's connections", ".config/Hermes/connections.json",
			`{"connections": [{"kind": "remote", "token": sk-SECRET}]}`,
			"parser: connections.json did not parse (invalid JSON at line 1, column 46",
			": invalid character 's' looking for beginning of value"},
		{"a platform's approved users", ".hermes/pairing/telegram-approved.json",
			`{"12345": {"code": 0123456789}}`,
			"parser: pairing/telegram-approved.json did not parse (invalid JSON at line 1, column 21",
			": invalid character '1' after object key:value pair"},
	}
	tails := []string{"", ": REDACTED", ": [withheld]", ""}
	for _, test := range tests {
		for i, mode := range secretModes() {
			t.Run(test.name+" "+mode.name, func(t *testing.T) {
				tail := tails[i]
				if i == 0 {
					tail = test.detail
				}
				want := []string{test.position + tail + ")"}
				if got := parseWith(t, mode.setting, test.rel, test.text); !slices.Equal(got, want) {
					t.Errorf("got %q, want %q", got, want)
				}
			})
		}
	}
}

func TestARemoteBackendsURLShowsItsCredentialsUnlessWithheld(t *testing.T) {
	document := `{"connections": [` +
		`{"kind": "remote", "label": "work", "url": "https://alice:s3cret@hermes.example:9119/"},` +
		`{"kind": "ssh", "label": "box", "host": "box.example", "user": "alice", "port": 22}]}`
	urls := []string{"https://alice:s3cret@hermes.example:9119/", "https://REDACTED@hermes.example:9119/",
		"https://[withheld]@hermes.example:9119/", "https://hermes.example:9119/"}
	for i, mode := range secretModes() {
		t.Run(mode.name, func(t *testing.T) {
			got := parseWith(t, mode.setting, ".config/Hermes/connections.json", document)
			want := []string{
				"remote backend: remote work " + urls[i] + " (from connections.json, file mtime)",
				"remote backend: ssh box alice@box.example:22 (from connections.json, file mtime)",
			}
			if !slices.Equal(got, want) {
				t.Errorf("got %q, want %q", got, want)
			}
			if mode.setting.Withhold && strings.Contains(strings.Join(got, "\n"), "s3cret") {
				t.Errorf("a withheld run shows the password: %q", got)
			}
		})
	}
}

func TestAConnectionsPlainTokenAndHeadersAreShownUnlessWithheld(t *testing.T) {
	document := `{"connections": [{"kind": "remote", "label": "work", "url": "https://hermes.example/", ` +
		`"token": {"encoding": "plain", "value": "FIXTURE-TOKEN"}, ` +
		`"headers": {"encoding": "plain", "value": "{\"X-Key\": \"FIXTURE-HEADER\"}"}}, ` +
		`{"kind": "cloud", "label": "cloud", "url": "https://c.example/", ` +
		`"token": {"encoding": "safeStorage", "value": "Y2lwaGVydGV4dA=="}}]}`
	shown := []string{"token=FIXTURE-TOKEN headers={\"X-Key\": \"FIXTURE-HEADER\"}", "token=REDACTED headers=REDACTED",
		"token=[withheld] headers=[withheld]", "token= headers="}
	for i, mode := range secretModes() {
		t.Run(mode.name, func(t *testing.T) {
			got := parseWith(t, mode.setting, ".config/Hermes/connections.json", document)
			want := []string{
				"remote backend: remote work https://hermes.example/ " + shown[i] + " (from connections.json, file mtime)",
				"remote backend: cloud cloud https://c.example/ (from connections.json, file mtime)",
			}
			if !slices.Equal(got, want) {
				t.Errorf("got %q, want %q", got, want)
			}
			if mode.setting.Withhold && strings.Contains(strings.Join(got, "\n"), "FIXTURE") {
				t.Errorf("a withheld run shows a secret: %q", got)
			}
		})
	}
}

func TestAHeaderThatNamesNoCredentialIsReplacedOnlyWithTheWholeHeaders(t *testing.T) {
	document := `{"connections": [{"kind": "remote", "label": "work", "url": "https://hermes.example:18642/", ` +
		`"headers": {"encoding": "plain", "value": {"X-Version": "1"}}}]}`
	got := parseWith(t, secrets.Setting{Withhold: true, Placeholder: "P"}, ".config/Hermes/connections.json", document)
	want := []string{"remote backend: remote work https://hermes.example:18642/ headers=P " +
		"(from connections.json, file mtime)"}
	if !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestAnUploadedDebugReportsURLShowsItsCredentialsUnlessWithheld(t *testing.T) {
	document := `[{"url": "https://paste.example/r/1?token=FIXTURE-TOKEN&x=1", "expire_at": 1790845200}]`
	urls := []string{"FIXTURE-TOKEN", "REDACTED", "[withheld]", ""}
	for i, mode := range secretModes() {
		t.Run(mode.name, func(t *testing.T) {
			got := parseWith(t, mode.setting, ".hermes/pastes/pending.json", document)
			want := []string{"debug report uploaded: https://paste.example/r/1?token=" + urls[i] +
				"&x=1 (from pastes/pending.json, expire_at minus 6 hours)"}
			if !slices.Equal(got, want) {
				t.Errorf("got %q, want %q", got, want)
			}
			if mode.setting.Withhold && strings.Contains(strings.Join(got, "\n"), "FIXTURE") {
				t.Errorf("a withheld run shows a secret: %q", got)
			}
		})
	}
}
