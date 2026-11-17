package ollama

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// The rows of the database, the history and the settings files are proved
// against the Python analyzer by the parser replay
// (internal/parity/replay); these tests port what the Python tests call
// directly. Expected values come from running the analyzer.

func TestWantsOnlyTheFilesOllamaWrites(t *testing.T) {
	tests := []struct {
		rel  string
		want bool
	}{
		{"Library/Application Support/Ollama/db.sqlite", true},
		{"AppData/Local/Ollama/db.sqlite", true},
		{"AppData/Local/Ollama/db.sqlite-wal", false},
		{".ollama/history", true},
		{".ollama/config.json", true},
		{".ollama/id_ed25519", false},
		{"Library/Application Support/Claude-3p/configLibrary/" +
			"00000000-0000-4000-8000-000000000114.json", true},
		{"AppData/Local/Claude-3p/configLibrary/00000000-0000-4000-8000-000000000114.json", true},
		{"Library/Application Support/Claude-3p/configLibrary/" +
			"00000000-0000-4000-8000-000000000115.json", false},
		{"proj/.ollama/history", false},
	}
	for _, test := range tests {
		t.Run(test.rel, func(t *testing.T) {
			if got := New().Wants(&input.Artifact{Rel: test.rel}); got != test.want {
				t.Errorf("got %v, want %v", got, test.want)
			}
		})
	}
}

func TestCallTextPrefersTheURLThenTheQuery(t *testing.T) {
	tests := []struct {
		name      string
		arguments sqlitefile.Value
		want      string
	}{
		{"text that is not JSON stays", sqlitefile.TextValue("not json"), "not json"},
		{"an empty url keeps the arguments", sqlitefile.TextValue(`{"url":""}`), `{"url":""}`},
		{"a url is the text", sqlitefile.TextValue(`{"url":"https://x"}`), "https://x"},
		{"the url wins over the query", sqlitefile.TextValue(`{"query":"q","url":"u"}`), "u"},
		{"other arguments are sorted compact JSON",
			sqlitefile.TextValue(`{"query":5,"b":[1,2.0],"a":"é"}`), `{"a":"é","b":[1,2.0],"query":5}`},
		{"empty url and query are sorted", sqlitefile.TextValue(`{"url":"","query":""}`),
			`{"query":"","url":""}`},
		{"an array stays as stored", sqlitefile.TextValue("[1, 2]"), "[1, 2]"},
		{"a JSON string stays as stored", sqlitefile.TextValue(`"s"`), `"s"`},
		{"empty text stays empty", sqlitefile.TextValue(""), ""},
		{"padding around JSON is read", sqlitefile.TextValue(` {"query":"x"} `), "x"},
		{"NULL is null", sqlitefile.Value{}, "null"},
		{"an integer is its digits", sqlitefile.IntValue(5), "5"},
		{"a float is Python's repr", sqlitefile.FloatValue(1.5), "1.5"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := callText(test.arguments); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestResultFallbackGivesTheStoredResultAsCompactJSON(t *testing.T) {
	tests := []struct {
		name  string
		value sqlitefile.Value
		want  string
	}{
		{"NULL is nothing", sqlitefile.Value{}, ""},
		{"empty text is nothing", sqlitefile.TextValue(""), ""},
		{"JSON null is nothing", sqlitefile.TextValue("null"), ""},
		{"an object is sorted", sqlitefile.TextValue(`{"b":1,"a":[1.0]}`), `{"a":[1.0],"b":1}`},
		{"text that is not JSON stays", sqlitefile.TextValue("nope"), "nope"},
		{"an integer is its digits", sqlitefile.IntValue(5), "5"},
		{"a float is Python's repr", sqlitefile.FloatValue(2.5), "2.5"},
		{"zero as text is JSON zero", sqlitefile.TextValue("0"), "0"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := resultFallback(test.value); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestInvalidUTF8IsReportedAtTheOffsetOfItsFirstBadSequence(t *testing.T) {
	tests := []struct {
		name, data, want string
	}{
		{"a sequence cut at the end", "ab\xe2\x82", "not valid UTF-8 at byte offset 2"},
		{"a lone continuation byte", "\x80", "not valid UTF-8 at byte offset 0"},
		{"an overlong form", "\xc1\x80", "not valid UTF-8 at byte offset 0"},
		{"an encoded surrogate", "ok\xed\xa0\x80", "not valid UTF-8 at byte offset 2"},
		{"a code point past U+10FFFF", "\xf4\x90\x80\x80", "not valid UTF-8 at byte offset 0"},
		{"a lead byte no sequence starts with", "caf\xc3\xa9 \xff", "not valid UTF-8 at byte offset 6"},
		{"only the first bad sequence is named", "\xc3x\xff", "not valid UTF-8 at byte offset 0"},
		{"valid text with U+FFFD itself", "caf\xc3\xa9 \xef\xbf\xbd \xf0\x9f\x98\x80", ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := utf8Problem([]byte(test.data)); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestHistoryLinesAreTrimmedOfASCIIWhitespaceOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history")
	data := "  hello \t\n kept \n\n/bye\n\r\nbad \xe9\n"
	if err := osfile.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := &input.Artifact{DiskPath: path, Original: "/alice/.ollama/history",
		Rel: ".ollama/history", Home: &input.Home{User: "alice"}}
	var texts []string
	err := New().Parse(artifact, parser.Options{}, func(row timeline.Row) {
		texts = append(texts, row.TurnType+"|"+row.Text)
	})
	want := []string{"user|hello", "user| kept ", "system|slash command: /bye",
		"system|parser: unparseable line (not valid UTF-8 at byte offset 4): bad �"}
	if err != nil || !slices.Equal(texts, want) {
		t.Errorf("got %q, %v; want %q", texts, err, want)
	}
}
