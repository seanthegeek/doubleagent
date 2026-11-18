package claudecode

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// parsedTexts writes text at rel under a new home and returns the texts of
// the rows the parser gives it.
func parsedTexts(t *testing.T, rel, text string) []string {
	t.Helper()
	home := t.TempDir()
	path := filepath.Join(home, filepath.FromSlash(rel))
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := &input.Artifact{DiskPath: path, Original: "/h/" + rel, Rel: rel, Home: &input.Home{DiskPath: home}}
	var texts []string
	if err := New().Parse(artifact, parser.Options{}, func(row timeline.Row) { texts = append(texts, row.Text) }); err != nil {
		t.Fatal(err)
	}
	return texts
}

func TestASettingsPermissionsMemberOfTheWrongKindIsReported(t *testing.T) {
	const rel = ".claude/settings.json"
	const allowed = "permission setting: permissions.allow=Bash(ls) (from .claude/settings.json, file mtime)"
	tests := []struct {
		name, permissions string
		want              []string
	}{
		{"permissions as a string", `"x"`, []string{"parser: permissions is not a JSON object"}},
		{"permissions as a number", `3`, []string{"parser: permissions is not a JSON object"}},
		{"permissions as an array", `["Bash(ls)"]`, []string{"parser: permissions is not a JSON object"}},
		{"permissions null", `null`, nil},
		{"permissions empty", `{}`, nil},
		{"allow as a string", `{"allow": "Bash(ls)"}`, []string{"parser: permissions.allow is not a JSON array"}},
		{"deny as a number", `{"deny": 3}`, []string{"parser: permissions.deny is not a JSON array"}},
		{"ask as an object", `{"ask": {"a": 1}}`, []string{"parser: permissions.ask is not a JSON array"}},
		{"additionalDirectories as a string", `{"allow": ["Bash(ls)"], "additionalDirectories": "/q"}`,
			[]string{allowed, "parser: permissions.additionalDirectories is not a JSON array"}},
		{"lists null and empty", `{"allow": null, "deny": [], "ask": null, "additionalDirectories": []}`, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := parsedTexts(t, rel, `{"permissions": `+test.permissions+`}`); !slices.Equal(got, test.want) {
				t.Errorf("rows %q, want %q", got, test.want)
			}
		})
	}
	t.Run("permissions absent", func(t *testing.T) {
		if got := parsedTexts(t, rel, `{}`); len(got) != 0 {
			t.Errorf("rows %q, want none", got)
		}
	})
}

func TestAGlobalConfigProjectMemberOfTheWrongKindIsReported(t *testing.T) {
	const rel = ".claude.json"
	tests := []struct {
		name, projects string
		want           []string
	}{
		{"projects as a string", `"x"`, []string{"parser: projects is not a JSON object"}},
		{"projects as a number", `3`, []string{"parser: projects is not a JSON object"}},
		{"projects as an array", `[{"allowedTools": ["Bash"]}]`, []string{"parser: projects is not a JSON object"}},
		{"projects null", `null`, nil},
		{"projects empty", `{}`, nil},
		{"a project as a string", `{"/p": "x"}`, []string{"parser: projects./p is not a JSON object"}},
		{"a project as an array", `{"/p": []}`, []string{"parser: projects./p is not a JSON object"}},
		{"a project null", `{"/p": null}`, nil},
		{"allowedTools as a string", `{"/p": {"allowedTools": "Bash"}}`,
			[]string{"parser: projects./p.allowedTools is not a JSON array"}},
		{"allowedTools as a number", `{"/p": {"allowedTools": 3}}`,
			[]string{"parser: projects./p.allowedTools is not a JSON array"}},
		{"allowedTools as an object", `{"/p": {"allowedTools": {"a": 1}}}`,
			[]string{"parser: projects./p.allowedTools is not a JSON array"}},
		{"allowedTools null", `{"/p": {"allowedTools": null}}`, nil},
		{"allowedTools empty", `{"/p": {"allowedTools": []}}`, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := parsedTexts(t, rel, `{"projects": `+test.projects+`}`); !slices.Equal(got, test.want) {
				t.Errorf("rows %q, want %q", got, test.want)
			}
		})
	}
	t.Run("projects absent", func(t *testing.T) {
		if got := parsedTexts(t, rel, `{}`); len(got) != 0 {
			t.Errorf("rows %q, want none", got)
		}
	})
}
