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
			artifact.Home.Secrets = mode.setting
			database, err := sqlitefile.OpenPath(artifact.DiskPath)
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			var got []string
			New().state(artifact, newStore(database, parser.CountingWork()), parser.Options{},
				func(row timeline.Row) { got = append(got, row.Text) })
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
				Home:     &input.Home{DiskPath: home, Original: "/home/u", User: "u", Host: "h", Secrets: mode.setting},
				Manifest: jsonvalue.NewNull()}
			var got []string
			err := New().Parse(artifact, parser.Options{}, func(row timeline.Row) { got = append(got, row.Text) })
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
			artifact.Home.Secrets = mode.setting
			var got []string
			var projects []string
			err := New().Parse(artifact, parser.Options{}, func(row timeline.Row) {
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
