package tabby

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/parserkit/parsertest"
	"github.com/seanthegeek/doubleagent/internal/secrets"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// secretMode is one way a run shows secrets: by default, or withheld as
// the default placeholder, another, or an empty one, with the text a URL
// whose credentials are user:s3cret starts with under it.
type secretMode struct {
	name    string
	setting secrets.Setting
	scheme  string
}

// secretModes returns the default and the three ways of withholding.
func secretModes() []secretMode {
	return []secretMode{
		{"shown by default", secrets.Setting{}, "https://ci:s3cret@"},
		{"withheld as REDACTED", secrets.Setting{Withhold: true, Placeholder: secrets.DefaultPlaceholder},
			"https://REDACTED@"},
		{"withheld as another placeholder", secrets.Setting{Withhold: true, Placeholder: "[withheld]"},
			"https://[withheld]@"},
		{"withheld as an empty placeholder", secrets.Setting{Withhold: true}, "https://"},
	}
}

// checkWithheld fails the test when a run that withholds secrets gave a
// row holding the secret.
func checkWithheld(t *testing.T, mode secretMode, texts []string, secret string) {
	t.Helper()
	if mode.setting.Withhold && strings.Contains(strings.Join(texts, "\n"), secret) {
		t.Errorf("a withheld run shows %q: %q", secret, texts)
	}
}

func TestAModelsEndpointShowsItsCredentialsUnlessWithheld(t *testing.T) {
	const config = "[model.chat.http]\nmodel_name = \"m\"\napi_endpoint = \"https://ci:s3cret@e/v1\"\n"
	for _, mode := range secretModes() {
		t.Run(mode.name, func(t *testing.T) {
			got := parseWith(t, mode.setting, ".tabby/config.toml", config)
			want := []string{"model configured: chat m via http endpoint=" + mode.scheme + "e/v1 " +
				"(from config.toml [model.chat.http], file mtime)"}
			if !slices.Equal(got, want) {
				t.Errorf("got %q, want %q", got, want)
			}
			checkWithheld(t, mode, got, "s3cret")
		})
	}
}

func TestAnIntegrationsAPIBaseAndAccessTokenShowTheirCredentialsUnlessWithheld(t *testing.T) {
	data, err := osfile.ReadFile(filepath.Join("testdata", "integrations.db"))
	if err != nil {
		t.Fatal(err)
	}
	tokens := []string{"glpat-TOKEN", "REDACTED", "[withheld]", ""}
	for i, mode := range secretModes() {
		t.Run(mode.name, func(t *testing.T) {
			got := parseWith(t, mode.setting, ".tabby/ee/db.sqlite", string(data))
			want := []string{"source host: gitlab.example integration work kind=gitlab_self_hosted " +
				"api_base=" + mode.scheme + "gitlab.example/api access_token=" + tokens[i] +
				" (account not recorded; from integrations table, created_at)"}
			if !slices.Equal(got, want) {
				t.Errorf("got %q, want %q", got, want)
			}
			checkWithheld(t, mode, got, "s3cret")
			checkWithheld(t, mode, got, "glpat-TOKEN")
		})
	}
}

func TestAConfigThatIsNotTOMLGivesTheReadersDetailUnlessWithheld(t *testing.T) {
	const position = "parser: config.toml is not valid TOML (invalid TOML at line 2, column 11"
	details := []string{`: expected value but found "sk" instead)`, ": REDACTED)", ": [withheld])", ")"}
	for i, mode := range secretModes() {
		t.Run(mode.name, func(t *testing.T) {
			got := parseWith(t, mode.setting, ".tabby/config.toml", "[model.chat.http]\napi_key = sk-SECRET\n")
			if want := []string{position + details[i]}; !slices.Equal(got, want) {
				t.Errorf("got %q, want %q", got, want)
			}
			checkWithheld(t, mode, got, `"sk"`)
		})
	}
}

func TestAConfigOverTheReadersLimitCouldNotBeReadWhateverTheSetting(t *testing.T) {
	config := "a = " + strings.Repeat("[", 40) + strings.Repeat("]", 40) + "\n"
	want := []string{"parser: config.toml could not be read (TOML document nests keys, tables and arrays more " +
		"than 32 levels deep at line 1)"}
	for _, mode := range secretModes() {
		t.Run(mode.name, func(t *testing.T) {
			got := parseAsBy(t, parsertest.ParseWithoutSecrets, mode.setting, "tabby", ".tabby/config.toml", config)
			if !slices.Equal(got, want) {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	}
}

func TestAUsersAuthTokenAndAServerRecordsTokenAreShownUnlessWithheld(t *testing.T) {
	tokens := []string{"auth_FIXTURE", "REDACTED", "[withheld]", ""}
	users, err := osfile.ReadFile(filepath.Join("testdata", "users.db"))
	if err != nil {
		t.Fatal(err)
	}
	state, err := osfile.ReadFile(filepath.Join("testdata", "server-records.vscdb"))
	if err != nil {
		t.Fatal(err)
	}
	for i, mode := range secretModes() {
		t.Run(mode.name, func(t *testing.T) {
			got := parseWith(t, mode.setting, ".tabby/ee/db.sqlite", string(users))
			want := []string{"tabby user: ana@example.com (Ana) id=1 auth_token=" + tokens[i] +
				" (from users table, created_at)"}
			if !slices.Equal(got, want) {
				t.Errorf("got %q, want %q", got, want)
			}
			got = parseAs(t, mode.setting, "vscode", ".config/Code/User/globalStorage/state.vscdb", string(state))
			want = []string{"tabby server: " + mode.scheme + "tabby.example/ token=" + tokens[i] +
				" (from state.vscdb TabbyML.vscode-tabby server.serverRecords, no times)"}
			if !slices.Equal(got, want) {
				t.Errorf("got %q, want %q", got, want)
			}
			checkWithheld(t, mode, got, "auth_FIXTURE")
			checkWithheld(t, mode, got, "s3cret")
		})
	}
}

func TestAModelsAPIKeyIsShownUnlessWithheld(t *testing.T) {
	const config = "[model.chat.http]\nmodel_name = \"m\"\napi_key = \"sk-FIXTURE\"\n"
	keys := []string{"sk-FIXTURE", "REDACTED", "[withheld]", ""}
	for i, mode := range secretModes() {
		t.Run(mode.name, func(t *testing.T) {
			got := parseWith(t, mode.setting, ".tabby/config.toml", config)
			want := []string{"model configured: chat m via http api_key=" + keys[i] +
				" (from config.toml [model.chat.http], file mtime)"}
			if !slices.Equal(got, want) {
				t.Errorf("got %q, want %q", got, want)
			}
			checkWithheld(t, mode, got, "sk-FIXTURE")
		})
	}
}

func TestACompletionsAndAnAttachmentsRepositoryURLShowTheirCredentialsUnlessWithheld(t *testing.T) {
	const event = `{"user":"1","ts":1759312800123,"event":{"completion":{"completion_id":"c1","language":"go",` +
		`"prompt":"x","segments":{"git_url":"https://ci:s3cret@github.com/o/r","filepath":"a.go"},"choices":[]}}}` +
		"\n"
	attachment := `{"code": [{"git_url": "https://ci:s3cret@github.com/o/r", "filepath": "a.go"}], ` +
		`"doc": [{"title": "d", "link": "https://ci:s3cret@docs.example/d"}]}`
	for _, mode := range secretModes() {
		t.Run(mode.name, func(t *testing.T) {
			home := t.TempDir()
			rel := ".tabby/events/2026-10-01.json"
			path := filepath.Join(home, filepath.FromSlash(rel))
			if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := osfile.WriteFile(path, []byte(event), 0o600); err != nil {
				t.Fatal(err)
			}
			artifact := &input.Artifact{DiskPath: path, Original: "/home/u/" + rel, Rel: rel, Agent: "tabby",
				Home:     &input.Home{DiskPath: home, Original: "/home/u", User: "u", Host: "h", Secrets: secrets.NewRecorder(mode.setting)},
				Manifest: jsonvalue.NewNull()}
			var projects []string
			err := parsertest.Parse(t, New(), artifact, parserkit.Options{}, func(row timeline.Row) {
				projects = append(projects, row.ProjectPath)
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(projects) == 0 || projects[0] != mode.scheme+"github.com/o/r" {
				t.Errorf("projects %q, want the first %q", projects, mode.scheme+"github.com/o/r")
			}
			value, err := jsonvalue.DecodeString(attachment)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			_ = parsertest.Run(t, artifact, func(emit parserkit.Emit) error {
				for _, part := range attachmentParts(artifact, value) {
					emit(timeline.Row{Text: part})
				}
				return nil
			}, func(row timeline.Row) { got = append(got, row.Text) })
			want := []string{"code " + mode.scheme + "github.com/o/r a.go", "doc d " + mode.scheme + "docs.example/d"}
			if !slices.Equal(got, want) {
				t.Errorf("got %q, want %q", got, want)
			}
			checkWithheld(t, mode, slices.Concat(got, projects), "s3cret")
		})
	}
}
