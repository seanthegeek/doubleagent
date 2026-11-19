package crush

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// parseCrushrcFile writes text as a crushrc under a new home and returns the
// text of each row the parser gives it, cut where the file name starts.
func parseCrushrcFile(t *testing.T, text string) []string {
	t.Helper()
	const rel = ".config/crush/crushrc"
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
	emit := func(row timeline.Row) {
		before, _, _ := strings.Cut(row.Text, " (from ")
		texts = append(texts, before)
	}
	if err := New().Parse(artifact, parser.Options{}, emit); err != nil {
		t.Fatal(err)
	}
	return texts
}

func TestACrushrcLineIsReadWhenItStartsWithTheTextPermissions(t *testing.T) {
	tests := []struct {
		name, text string
		want       []string
	}{
		{"a first word permissions", "permissions allow view edit\n", []string{
			"permission setting: permissions.allowed_tools=view",
			"permission setting: permissions.allowed_tools=edit"}},
		{"a longer first word with an unclosed quote is a bad line", "permissions_note='multi\nline'\n", []string{
			"parser: unparseable line (a quote is not closed): permissions_note='multi"}},
		{"a longer first word that splits gives no row", "permissions_note=x allow view\n", nil},
		{"a first word that does not start with the text", "export permissions allow view\n", nil},
		{"leading blanks", "  \tpermissions deny view\n", []string{
			"permission setting: options.disabled_tools=view"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := parseCrushrcFile(t, test.text); !slices.Equal(got, test.want) {
				t.Errorf("rows %q, want %q", got, test.want)
			}
		})
	}
}

func TestAHashInsideACrushrcWordEndsTheLine(t *testing.T) {
	tests := []struct {
		name, text string
		want       []string
	}{
		{"inside a word", "permissions allow view#1 edit\n", []string{"permission setting: permissions.allowed_tools=view"}},
		{"after a blank", "permissions allow view #1 edit\n", []string{"permission setting: permissions.allowed_tools=view"}},
		{"inside quotes", "permissions allow 'view#1' edit\n", []string{
			"permission setting: permissions.allowed_tools=view#1",
			"permission setting: permissions.allowed_tools=edit"}},
		{"after a backslash", "permissions allow view\\#1 edit\n", []string{
			"permission setting: permissions.allowed_tools=view#1",
			"permission setting: permissions.allowed_tools=edit"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := parseCrushrcFile(t, test.text); !slices.Equal(got, test.want) {
				t.Errorf("rows %q, want %q", got, test.want)
			}
		})
	}
}

func TestABackslashInsideDoubleQuotesEscapesOnlyAQuoteOrABackslash(t *testing.T) {
	tests := []struct {
		name, text string
		want       []string
	}{
		{"before a dollar it is kept", "permissions allow \"a\\$b\"\n",
			[]string{"permission setting: permissions.allowed_tools=a\\$b"}},
		{"before a backquote it is kept", "permissions allow \"c\\`d\"\n",
			[]string{"permission setting: permissions.allowed_tools=c\\`d"}},
		{"before a double quote it escapes it", "permissions allow \"e\\\"f\"\n",
			[]string{"permission setting: permissions.allowed_tools=e\"f"}},
		{"before a backslash it escapes it", "permissions allow \"g\\\\h\"\n",
			[]string{"permission setting: permissions.allowed_tools=g\\h"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := parseCrushrcFile(t, test.text); !slices.Equal(got, test.want) {
				t.Errorf("rows %q, want %q", got, test.want)
			}
		})
	}
}
