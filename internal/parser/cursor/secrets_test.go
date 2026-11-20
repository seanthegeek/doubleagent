package cursor

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
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// secretMode is one way a run shows secrets, with the text its rows give
// for a URL whose credentials are tok:s3cret.
type secretMode struct {
	name    string
	setting secrets.Setting
	url     string
}

// secretModes returns the default, which shows the credentials, and the
// three ways of withholding them: the default placeholder, another, and an
// empty one, which takes the @ with it.
func secretModes() []secretMode {
	return []secretMode{
		{"shown by default", secrets.Setting{}, "https://tok:s3cret@mcp.example.invalid/"},
		{"withheld as REDACTED", secrets.Setting{Withhold: true, Placeholder: secrets.DefaultPlaceholder},
			"https://REDACTED@mcp.example.invalid/"},
		{"withheld as another placeholder", secrets.Setting{Withhold: true, Placeholder: "[withheld]"},
			"https://[withheld]@mcp.example.invalid/"},
		{"withheld as an empty placeholder", secrets.Setting{Withhold: true}, "https://mcp.example.invalid/"},
	}
}

// checkWithheld fails the test when a run that withholds secrets gave a
// row holding the password.
func checkWithheld(t *testing.T, mode secretMode, texts []string) {
	t.Helper()
	for _, text := range texts {
		if mode.setting.Withhold && strings.Contains(text, "s3cret") {
			t.Errorf("a withheld run shows the password: %q", text)
		}
	}
}

func TestAnMCPServersURLShowsItsCredentialsUnlessWithheld(t *testing.T) {
	for _, mode := range secretModes() {
		t.Run(mode.name, func(t *testing.T) {
			artifact := stateArtifact(t, "mcp_secret.db")
			artifact.Home.Secrets = secrets.NewRecorder(mode.setting)
			database, err := sqlitefile.OpenPath(artifact.DiskPath)
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			var got []string
			_ = parsertest.Run(t, artifact, func(emit parser.Emit) error {
				New().state(artifact, newStore(database, parser.CountingWork()), parser.Options{}, emit)
				return nil
			}, func(row timeline.Row) { got = append(got, row.Text) })
			want := []string{"mcp server: srv url=" + mode.url + " (from state.vscdb mcpOAuth.global, file mtime)" +
				mcpTokens}
			if !slices.Equal(got, want) {
				t.Errorf("got %q, want %q", got, want)
			}
			checkWithheld(t, mode, got)
		})
	}
}

func TestAnMCPOAuthAttemptsServerShowsItsCredentialsUnlessWithheld(t *testing.T) {
	const rel = ".config/Cursor/User/globalStorage/mcp-oauth-attempts/a.json"
	document := `{"serverUrl": "https://tok:s3cret@mcp.example.invalid/", "createdAtMs": 1790845200000}`
	for _, mode := range secretModes() {
		t.Run(mode.name, func(t *testing.T) {
			home := t.TempDir()
			path := filepath.Join(home, filepath.FromSlash(rel))
			if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := osfile.WriteFile(path, []byte(document), 0o600); err != nil {
				t.Fatal(err)
			}
			artifact := &input.Artifact{DiskPath: path, Original: "/home/u/" + rel, Rel: rel, Agent: "cursor",
				Home:     &input.Home{DiskPath: home, Original: "/home/u", User: "u", Host: "h", Secrets: secrets.NewRecorder(mode.setting)},
				Manifest: jsonvalue.NewNull()}
			var got []string
			err := parsertest.Parse(t, New(), artifact, parser.Options{}, func(row timeline.Row) { got = append(got, row.Text) })
			if err != nil {
				t.Fatal(err)
			}
			want := []string{"mcp oauth attempt: server=" + mode.url + " client_id=- workspace=- " +
				"(from mcp-oauth-attempts/a.json)"}
			if !slices.Equal(got, want) {
				t.Errorf("got %q, want %q", got, want)
			}
			checkWithheld(t, mode, got)
		})
	}
}

func TestACloudAgentsRepositoryURLShowsItsCredentialsUnlessWithheld(t *testing.T) {
	schemes := []string{"https://ci:s3cret@", "https://REDACTED@", "https://[withheld]@", "https://"}
	for i, mode := range secretModes() {
		t.Run(mode.name, func(t *testing.T) {
			artifact := stateArtifact(t, "cloud_repository.db")
			artifact.Home.Secrets = secrets.NewRecorder(mode.setting)
			var got []string
			var projects []string
			err := parsertest.Parse(t, New(), artifact, parser.Options{}, func(row timeline.Row) {
				got = append(got, row.Text)
				projects = append(projects, row.ProjectPath)
			})
			if err != nil {
				t.Fatal(err)
			}
			repository := schemes[i] + "github.com/o/r"
			want := []string{
				cloudOrigin,
				"session synced: cursor cloud agent bc-1 (vendor copy; from state.vscdb cloudAgentRepository.agents)",
				"source host: github.com o/r (cloud agent)",
				"cloud agent: fix repo=" + repository + " status=done workflowStatus=- archived=- killed=- " +
					"vmStarted=- " + cloudSource,
				"permission decision: approved mcp auth for cloud agent bc-1 tool_call=t1 " + mcpAuthSource,
				"source host repository: " + repository + " type=github " + reposSource,
			}
			if !slices.Equal(got, want) {
				t.Errorf("got %q, want %q", got, want)
			}
			// The decision joins its agent on the agent's id, so it takes
			// the agent's project as every row of the agent shows it.
			wantProjects := []string{repository, repository, repository, repository, repository, ""}
			if !slices.Equal(projects, wantProjects) {
				t.Errorf("projects %q, want %q", projects, wantProjects)
			}
			checkWithheld(t, mode, slices.Concat(got, projects))
		})
	}
}

// pluginRows returns the plugin rows of the plugin_scopes.db fixture under a
// secrets setting, each project path given its one spelling as the run
// gives every row's, and every row as timeline.jsonl writes it.
func pluginRows(t *testing.T, setting secrets.Setting) []timeline.Row {
	t.Helper()
	artifact := stateArtifact(t, "plugin_scopes.db")
	artifact.Home.Secrets = secrets.NewRecorder(setting)
	database, err := sqlitefile.OpenPath(artifact.DiskPath)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	var rows []timeline.Row
	_ = parsertest.Run(t, artifact, func(emit parser.Emit) error {
		New().state(artifact, newStore(database, parser.CountingWork()), parser.Options{}, func(row timeline.Row) {
			row.ProjectPath = timeline.CanonicalProjectPath(row.ProjectPath)
			emit(row)
		})
		return nil
	}, func(row timeline.Row) {
		if strings.HasPrefix(row.Text, "plugin installed:") {
			rows = append(rows, row)
		}
	})
	return rows
}

// A plugin scope's file: path is the row's project path, as the run spells
// every path, so a URL inside the path keeps its slashes and the output
// finds its credentials there as in the row's text; only the encoded form
// of a percent-encoded URL is left in the text, where it is no URL.
func TestAURLCredentialInAPluginScopesPathIsWithheldInEveryField(t *testing.T) {
	want := map[string]string{
		"C:\\w\\https://ef:gh@example.org/r":      "C:\\w\\https://REDACTED@example.org/r",
		"C:\\w\\https://ab:cd@example.com/r":      "C:\\w\\https://REDACTED@example.com/r",
		"\\\\server\\https://ij:kl@example.net/r": "\\\\server\\https://REDACTED@example.net/r",
		"/srv/https://mn:op@example.com/r":        "/srv/https://REDACTED@example.com/r",
	}
	shown := pluginRows(t, secrets.Setting{})
	withheld := pluginRows(t, secrets.Setting{Withhold: true, Placeholder: secrets.DefaultPlaceholder})
	if len(shown) != len(want) || len(withheld) != len(want) {
		t.Fatalf("%d and %d plugin rows, want %d", len(shown), len(withheld), len(want))
	}
	credentials := []string{"ef:gh", "ab:cd", "ij:kl", "mn:op"}
	for i, row := range shown {
		written := withheld[i]
		if got, ok := want[row.ProjectPath]; !ok || written.ProjectPath != got {
			t.Errorf("project path %q is written %q, want %q", row.ProjectPath, written.ProjectPath, got)
		}
		for _, text := range written.Strings() {
			for _, credential := range credentials {
				if strings.Contains(text, credential) {
					t.Errorf("a withheld run shows %q in %q", credential, text)
				}
			}
		}
	}
}
