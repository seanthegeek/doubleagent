package kilocode

import (
	"path/filepath"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

func TestAFileThatIsNotWhatKiloWritesIsAProblemWithNoTime(t *testing.T) {
	const mtime = "2024-01-02T03:04:05.000Z"
	share := ".local/share/kilo/storage/session_share/ses_1.json"
	tests := []struct {
		name, rel, text string
		want            [4]string
	}{
		{"a config file that is not an object", ".config/kilo/kilo.json", `[1]`,
			[4]string{"", "", "1", "parser: .config/kilo/kilo.json is not a JSON object"}},
		{"a config file that does not decode", ".config/kilo/kilo.json", `{`,
			[4]string{"", "", "1", "parser: .config/kilo/kilo.json could not be decoded (invalid JSON at line 1, column 2: unexpected end of JSON input)"}},
		{"a sandbox preference that is not true or false", ".local/state/kilo-sandbox-preference/a.json", `1`,
			[4]string{"", "", "1", "parser: .local/state/kilo-sandbox-preference/a.json is not true or false"}},
		{"a sandbox policy that is not an object", ".local/state/kilo-sandbox-policy/a/b.json", `1`,
			[4]string{"", "", "1", "parser: .local/state/kilo-sandbox-policy/a/b.json is not a JSON object"}},
		{"a retention state that is not an object", ".local/share/kilo/retention/state.json", `1`,
			[4]string{"", "", "1", "parser: retention/state.json is not a JSON object"}},
		{"a cloud session marker that is not an object", share, `1`,
			[4]string{"", "ses_1", "1", "parser: storage/session_share/ses_1.json is not a JSON object"}},
		{"a sandbox preference keeps the file's time", ".local/state/kilo-sandbox-preference/a.json", `true`,
			[4]string{mtime, "", "1", "permission setting: sandbox.enabled=true " +
				"(from .local/state/kilo-sandbox-preference/a.json, file mtime)"}},
	}
	moment := time.Date(2024, time.January, 2, 3, 4, 5, 0, time.UTC)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			path := filepath.Join(home, filepath.FromSlash(test.rel))
			write(t, path, test.text)
			if err := osfile.Chtimes(path, moment, moment); err != nil {
				t.Fatal(err)
			}
			var got [][4]string
			for _, row := range parsed(t, home, test.rel) {
				got = append(got, [4]string{row.TimestampUTC, row.SessionID, strconv.Itoa(row.SourceLine), row.Text})
			}
			if want := [][4]string{test.want}; !slices.Equal(got, want) {
				t.Errorf("rows %q, want %q", got, want)
			}
		})
	}
}
