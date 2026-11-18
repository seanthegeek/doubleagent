package tabby

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/secrets"
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

func TestAnIntegrationsAPIBaseShowsItsCredentialsUnlessWithheld(t *testing.T) {
	data, err := osfile.ReadFile(filepath.Join("testdata", "integrations.db"))
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range secretModes() {
		t.Run(mode.name, func(t *testing.T) {
			got := parseWith(t, mode.setting, ".tabby/ee/db.sqlite", string(data))
			want := []string{"source host: gitlab.example integration work kind=gitlab_self_hosted " +
				"api_base=" + mode.scheme + "gitlab.example/api (account not recorded; from integrations table, " +
				"created_at)"}
			if !slices.Equal(got, want) {
				t.Errorf("got %q, want %q", got, want)
			}
			checkWithheld(t, mode, got, "s3cret")
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
			if got := parseWith(t, mode.setting, ".tabby/config.toml", config); !slices.Equal(got, want) {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	}
}
