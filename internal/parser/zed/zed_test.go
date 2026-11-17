package zed

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
)

// The rows of every store are proved against the Python analyzer by the
// parser replay (internal/parity/replay), over the recorded fixture
// databases; these tests cover the helpers, with expected values taken from
// running the analyzer's own functions.

// thread is the JSON every frame below holds.
const threadJSON = `{"version":"0.3.0","messages":[]}`

func TestDecompressReadsAFrameWithOrWithoutItsSize(t *testing.T) {
	tests := []struct {
		name  string
		frame string
		want  string
	}{
		{"a frame recording its content size",
			"28b52ffd20210901007b2276657273696f6e223a22302e332e30222c226d65737361676573223a5b5d7d",
			threadJSON},
		{"a frame without its content size",
			"28b52ffd00000901007b2276657273696f6e223a22302e332e30222c226d65737361676573223a5b5d7d",
			threadJSON},
		{"a frame cut off inside its only block gives no content and no error",
			"28b52ffd00000901007b2276657273696f6e223a22302e332e30222c226d65737361676573223a", ""},
		{"no bytes give no content", "", ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			frame, err := hex.DecodeString(test.frame)
			if err != nil {
				t.Fatal(err)
			}
			got, err := Decompress(frame)
			if err != nil || string(got) != test.want {
				t.Errorf("got %q, %v; want %q", got, err, test.want)
			}
		})
	}
}

func TestDecompressRefusesDataThatIsNotZstd(t *testing.T) {
	got, err := Decompress([]byte("hello world"))
	if err == nil || got != nil || !strings.HasPrefix(err.Error(), "zstd: corrupt data (") {
		t.Errorf("got %q, %v; want a corrupt-data error", got, err)
	}
}

// decoded returns the JSON value of text, failing the test when it is not
// JSON.
func decoded(t *testing.T, text string) jsonvalue.Value {
	t.Helper()
	value, err := jsonvalue.DecodeString(text)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestHelpersGiveTheAnalyzersText(t *testing.T) {
	tests := []struct {
		name     string
		function func(jsonvalue.Value) string
		input    string
		want     string
	}{
		{"a null result content is empty", contentText, `null`, ""},
		{"a result content list joins its parts", contentText,
			`[{"Text": "a"}, {"Image": {}}, {"text": "b"}, {"x": 1}, 5, "s"]`, "a\n[image]\nb\n{\"x\":1}\ns"},
		{"a single result item is a list of one", contentText, `{"Text": 3, "text": "t"}`, "t"},
		{"a result content number is JSON", contentText, `7`, "7"},
		{"a draft's text block is its text", acpBlocksText, `{"type": "text", "text": "hi"}`, "hi"},
		{"a draft's other blocks are mentions, markers and JSON", acpBlocksText,
			`[{"type": "resource_link", "uri": "file:///a"}, {"type": "resource", "resource": {"uri": 5}},` +
				` {"type": "image"}, "raw", {"type": "audio"}, {"type": "other"}, 3]`,
			"[mention] file:///a\n[mention] 5\n[image]\nraw\n[audio]\n{\"type\":\"other\"}\n3"},
		{"a draft that is a string is JSON", acpBlocksText, `"str"`, `"str"`},
		{"a wrapped tool input is unwrapped", toolInputText,
			`{"input": {"type": "x", "value": {"b": 1, "a": 2}}}`, `{"b":1,"a":2}`},
		{"an empty tool input falls back to raw_input", toolInputText,
			`{"input": {}, "raw_input": "r"}`, "r"},
		{"a raw_input that is not a string is dropped", toolInputText,
			`{"input": "", "raw_input": 5}`, ""},
		{"a zero tool input is kept", toolInputText, `{"input": 0}`, "0"},
		{"an input with other keys stays wrapped", toolInputText,
			`{"input": {"value": "v", "other": 1}}`, `{"value":"v","other":1}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.function(decoded(t, test.input)); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestDenialTextReadsOnlyDeniedCalls(t *testing.T) {
	tests := []struct {
		name   string
		result string
		want   string
	}{
		{"the user denied the call", `{"is_error": true, "content": "Permission to run tool denied by user"}`,
			"permission decision: denied"},
		{"a disabled tool", `{"is_error": true, "content": [{"Text": "terminal tool is disabled"}]}`,
			"permission decision: auto_denied reason=terminal tool is disabled"},
		{"patterns that fail to compile, over two lines",
			`{"is_error": true, "content": "The edit tool cannot run because 2 regex patterns failed to compile.\nmore"}`,
			"permission decision: auto_denied reason=The edit tool cannot run because 2 regex patterns failed to compile.\nmore"},
		{"a shell without always-allow patterns",
			`{"is_error": true, "content": "The fish shell does not support \"always allow\" patterns x"}`,
			"permission decision: auto_denied reason=The fish shell does not support \"always allow\" patterns x"},
		{"the global default, padded", `{"is_error": true, "content": "  Blocked by global default: deny  "}`,
			"permission decision: auto_denied reason=Blocked by global default: deny"},
		{"another failure", `{"is_error": true, "content": "nope"}`, ""},
		{"a successful result", `{"is_error": false, "content": "Permission to run tool denied by user"}`, ""},
		{"a tool name holding a space",
			`{"is_error": true, "content": "Command blocked by security rule for my tool tool"}`, ""},
	}
	subject := New()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := subject.denialText(decoded(t, test.result)); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestThreadKeyFormatsARawUUID(t *testing.T) {
	tests := []struct {
		name  string
		value sqlitefile.Value
		want  string
	}{
		{"sixteen bytes are a UUID", sqlitefile.BlobValue([]byte{0x12, 0x34, 0x56, 0x78, 0x9a, 0xbc,
			0xde, 0xf0, 0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef}),
			"12345678-9abc-def0-0123-456789abcdef"},
		{"text is kept", sqlitefile.TextValue("t1"), "t1"},
		{"NULL is empty", sqlitefile.Value{}, ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := threadKey(test.value); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestWantsTheThreeKindsOfFile(t *testing.T) {
	tests := []struct {
		rel  string
		want bool
	}{
		{".local/share/zed/threads/threads.db", true},
		{"Library/Application Support/Zed/db/0-stable/db.sqlite", true},
		{".var/app/dev.zed.ZedPreview/data/zed/threads/threads.db", true},
		{".config/zed/settings.json", true},
		{"AppData/Roaming/Zed/global_settings.json", true},
		{".local/share/zed/threads/threads.db-wal", false},
		{".local/share/zed/db/0-stable/sub/db.sqlite", false},
		{"proj/.zed/settings.json", false},
	}
	for _, test := range tests {
		t.Run(test.rel, func(t *testing.T) {
			if got := New().Wants(&input.Artifact{Rel: test.rel}); got != test.want {
				t.Errorf("got %v, want %v", got, test.want)
			}
		})
	}
}

func FuzzDecompress(fuzz *testing.F) {
	for _, seed := range []string{
		"28b52ffd20210901007b2276657273696f6e223a22302e332e30222c226d65737361676573223a5b5d7d",
		"28b52ffd00000901007b2276657273696f6e223a22302e332e30222c226d65737361676573223a",
	} {
		frame, _ := hex.DecodeString(seed)
		fuzz.Add(frame)
	}
	fuzz.Fuzz(func(t *testing.T, data []byte) {
		out, err := Decompress(data)
		if err == nil && len(out) > MaxDecompressed {
			t.Errorf("%d bytes decoded, more than the limit", len(out))
		}
	})
}
