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

func TestStrictUTF8ErrorsAreWordedAsTheAnalyzerWordsThem(t *testing.T) {
	tests := []struct {
		data string
		want string
	}{
		{"ab\xe2\x82", "'utf-8' codec can't decode bytes in position 2-3: unexpected end of data"},
		{"ab\xe2", "'utf-8' codec can't decode byte 0xe2 in position 2: unexpected end of data"},
		{"\x80", "'utf-8' codec can't decode byte 0x80 in position 0: invalid start byte"},
		{"\xc1\x80", "'utf-8' codec can't decode byte 0xc1 in position 0: invalid start byte"},
		{"\xe0\x80\x80", "'utf-8' codec can't decode byte 0xe0 in position 0: invalid continuation byte"},
		{"\xe0\xa0", "'utf-8' codec can't decode bytes in position 0-1: unexpected end of data"},
		{"\xe0\xa0x", "'utf-8' codec can't decode bytes in position 0-1: invalid continuation byte"},
		{"\xed\xa0\x80", "'utf-8' codec can't decode byte 0xed in position 0: invalid continuation byte"},
		{"\xf0\x90\x80", "'utf-8' codec can't decode bytes in position 0-2: unexpected end of data"},
		{"\xf0\x90\x80x", "'utf-8' codec can't decode bytes in position 0-2: invalid continuation byte"},
		{"\xf0\x90x", "'utf-8' codec can't decode bytes in position 0-1: invalid continuation byte"},
		{"\xf0\x80", "'utf-8' codec can't decode byte 0xf0 in position 0: invalid continuation byte"},
		{"\xf4\x90", "'utf-8' codec can't decode byte 0xf4 in position 0: invalid continuation byte"},
		{"\xf5", "'utf-8' codec can't decode byte 0xf5 in position 0: invalid start byte"},
		{"\xff", "'utf-8' codec can't decode byte 0xff in position 0: invalid start byte"},
		{"\xc3x", "'utf-8' codec can't decode byte 0xc3 in position 0: invalid continuation byte"},
		{"\xc3", "'utf-8' codec can't decode byte 0xc3 in position 0: unexpected end of data"},
		{"ok\xf0\x9f\x98", "'utf-8' codec can't decode bytes in position 2-4: unexpected end of data"},
		{"\xe2\x82\n", "'utf-8' codec can't decode bytes in position 0-1: invalid continuation byte"},
		{"caf\xc3\xa9 \xf0\x9f\x98\x80", ""},
	}
	for _, test := range tests {
		t.Run(test.want, func(t *testing.T) {
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
		"system|parser: unparseable line ('utf-8' codec can't decode byte 0xe9 in position 4: " +
			"invalid continuation byte): bad �"}
	if err != nil || !slices.Equal(texts, want) {
		t.Errorf("got %q, %v; want %q", texts, err, want)
	}
}
