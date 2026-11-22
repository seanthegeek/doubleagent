package crush

import (
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
)

// The rows of the database, the registry and the settings files are checked
// by the golden output of the analysis commands (internal/analyze), over
// inputs that hold every parsed agent; these tests cover the shell word
// splitter of the permissions file, the channel attributes' unescaping and
// the project path.

func TestShellWordsSplitAsPOSIXQuotingWithComments(t *testing.T) {
	tests := []struct {
		line    string
		want    []string
		wantErr string
	}{
		{"permissions allow bash edit", []string{"permissions", "allow", "bash", "edit"}, ""},
		{"permissions allow 'a b' \"c d\"", []string{"permissions", "allow", "a b", "c d"}, ""},
		{"permissions allow a#b c", []string{"permissions", "allow", "a"}, ""},
		{"permissions allow a # comment", []string{"permissions", "allow", "a"}, ""},
		{"# permissions allow x", nil, ""},
		{"permissions allow \"a#b\" 'c#d'", []string{"permissions", "allow", "a#b", "c#d"}, ""},
		{"permissions allow a\\ b", []string{"permissions", "allow", "a b"}, ""},
		{"permissions allow \"a\\\"b\" \"a\\\\b\" \"a\\nb\"", []string{"permissions", "allow", "a\"b", "a\\b", "a\\nb"}, ""},
		{"permissions allow 'a\\b'", []string{"permissions", "allow", "a\\b"}, ""},
		{"permissions allow \"unterminated", nil, "a quote is not closed"},
		{"permissions allow 'unterminated", nil, "a quote is not closed"},
		{"permissions allow trailing\\", nil, "a backslash ends the line"},
		{"permissions allow \"\"", []string{"permissions", "allow", ""}, ""},
		{"permissions allow ''", []string{"permissions", "allow", ""}, ""},
		{"permissions allow a\"\"b", []string{"permissions", "allow", "ab"}, ""},
		{"permissions\x09allow\x09 x\x0d", []string{"permissions", "allow", "x"}, ""},
		{"permissions allow é ü", []string{"permissions", "allow", "é", "ü"}, ""},
		{"permissions allow \"x\"y'z'", []string{"permissions", "allow", "xyz"}, ""},
		{"permissions allow a\\#b", []string{"permissions", "allow", "a#b"}, ""},
		{"  permissions deny  x  ", []string{"permissions", "deny", "x"}, ""},
		{"permissions allow \"a b\"#c d", []string{"permissions", "allow", "a b"}, ""},
		{"permissions allow x\x0by", []string{"permissions", "allow", "x\x0by"}, ""},
		{"", nil, ""},
		{"#", nil, ""},
		{"\"\"", []string{""}, ""},
		{"a \"\" b", []string{"a", "", "b"}, ""},
		{"permissions allow \"\\$x\" \"\\`\"", []string{"permissions", "allow", "\\$x", "\\`"}, ""},
	}
	for _, test := range tests {
		t.Run(test.line, func(t *testing.T) {
			got, err := splitWords(test.line)
			gotErr := ""
			if err != nil {
				gotErr = err.Error()
			}
			if !slices.Equal(got, test.want) || gotErr != test.wantErr {
				t.Errorf("got %q, %q; want %q, %q", got, gotErr, test.want, test.wantErr)
			}
		})
	}
}

func TestChannelAttributesAreUnescapedAsCrushEscapesThem(t *testing.T) {
	tests := []struct{ name, text, want string }{
		{"the five escaped characters", `&amp;&#39;&lt;&gt;&#34;`, `&'<>"`},
		{"an escaped ampersand stays one", "&amp;amp;", "&amp;"},
		{"named references", "&quot;&apos;&lt;", `"'<`},
		{"text without a reference", "telegram", "telegram"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			text := `<channel source="` + test.text + `">hello</channel>`
			reader := &messageReader{store: &store{parser: New()}}
			got, _ := reader.relayText(text, "m1")
			want := "session relayed: channel " + test.want + " (from crush.db message m1)"
			if got != want {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	}
}

func TestTheProjectIsTheDirectoryHoldingDotCrush(t *testing.T) {
	tests := []struct {
		original string
		want     string
	}{
		{"/srv/proj/.crush/crush.db", "/srv/proj"},
		{`C:\Users\a\repo\.crush\crush.db`, `C:\Users\a\repo`},
		{".crush/crush.db", ""},
	}
	for _, test := range tests {
		t.Run(test.original, func(t *testing.T) {
			if got := New().projectOf(test.original); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestTheChannelTagTakesXMLWhitespaceOnly(t *testing.T) {
	subject := New()
	for _, space := range []string{" ", "\t", "\n", "\u00a0", "\u2028", "\u0085"} {
		want := space == " " || space == "\t" || space == "\n"
		t.Run(space, func(t *testing.T) {
			if got := subject.channel.MatchString("<channel" + space + `source="x">`); got != want {
				t.Errorf("got %v, want %v", got, want)
			}
		})
	}
}

func TestWantsTheDatabaseRegistryAndEverySettingsScope(t *testing.T) {
	tests := []struct {
		rel  string
		want bool
	}{
		{".crush/crush.db", true},
		{"proj/.crush/crush.db", true},
		{".crush/crush.db-wal", false},
		{".local/share/crush/projects.json", true},
		{"AppData/Local/crush/projects.json", true},
		{".config/crush/crush.json", true},
		{".local/share/crush/crush.json", true},
		{".crush/crush.json", true},
		{"crush.json", true},
		{".crush.json", true},
		{"proj/crush.json", false},
		{".config/crush/crushrc", true},
		{"crushrc", true},
		{".crushrc", true},
		{".crush/crush.lock", false},
	}
	for _, test := range tests {
		t.Run(test.rel, func(t *testing.T) {
			if got := New().Wants(&input.Artifact{Rel: test.rel}); got != test.want {
				t.Errorf("got %v, want %v", got, test.want)
			}
		})
	}
}
