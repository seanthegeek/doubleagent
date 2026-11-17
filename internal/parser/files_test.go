package parser

import (
	"path/filepath"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// The expected timestamps are what the Python analyzer's file_mtime gives
// for the same manifest values and file times.
func TestFileMtimePrefersAPositiveManifestNumber(t *testing.T) {
	path := writeFile(t, "settings.json", []byte("{}"))
	moment := time.Unix(1790848800, 123456789)
	if err := osfile.Chtimes(path, moment, moment); err != nil {
		t.Fatal(err)
	}
	absent := filepath.Join(t.TempDir(), "absent")
	tests := []struct {
		name, manifest, path, want string
	}{
		{"an integer mtime", `{"mtime":1790848800}`, absent, "2026-10-01T10:00:00.000Z"},
		{"a float mtime", `{"mtime":1790848800.25}`, absent, "2026-10-01T10:00:00.250Z"},
		{"an mtime in milliseconds", `{"mtime":1790848800123}`, absent, "2026-10-01T10:00:00.123Z"},
		{"zero falls back to the file", `{"mtime":0}`, path, "2026-10-01T10:00:00.123Z"},
		{"a negative mtime falls back to the file", `{"mtime":-5}`, path, "2026-10-01T10:00:00.123Z"},
		{"a boolean falls back to the file", `{"mtime":true}`, path, "2026-10-01T10:00:00.123Z"},
		{"a string falls back to the file", `{"mtime":"1790848800"}`, path, "2026-10-01T10:00:00.123Z"},
		{"an integer too large for a float gives nothing", `{"mtime":1` + zeros(400) + `}`, path, ""},
		{"no manifest and no file give nothing", `null`, absent, ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			artifact := &input.Artifact{DiskPath: test.path, Manifest: decoded(t, test.manifest)}
			if got := FileMtime(artifact); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

// zeros returns n zero digits.
func zeros(n int) string {
	digits := make([]byte, n)
	for i := range digits {
		digits[i] = '0'
	}
	return string(digits)
}

func TestFileMtimeCutsTheFileTimeToTheMillisecond(t *testing.T) {
	tests := []struct {
		nanoseconds int64
		want        string
	}{
		{1790848800_123456789, "2026-10-01T10:00:00.123Z"},
		{1790848800_999999500, "2026-10-01T10:00:00.999Z"},
		{1790848800_000500500, "2026-10-01T10:00:00.000Z"},
		{1790848799_999999999, "2026-10-01T09:59:59.999Z"},
	}
	for _, test := range tests {
		t.Run(test.want, func(t *testing.T) {
			path := writeFile(t, "file", nil)
			moment := time.Unix(0, test.nanoseconds)
			if err := osfile.Chtimes(path, moment, moment); err != nil {
				t.Fatal(err)
			}
			artifact := &input.Artifact{DiskPath: path, Manifest: jsonvalue.NewNull()}
			if got := FileMtime(artifact); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

// The times are whole multiples of 100 ns, the finest a Windows file time
// holds.
func TestMtimeSecondsIsThePythonStatFloat(t *testing.T) {
	tests := []struct {
		nanoseconds int64
		want        float64
	}{
		{1790848800_123456789, 1790848800.1234567},
		{1790848800_999999500, 1790848800.9999995},
		{1790848799_999999999, 1790848800.0},
		{1_000000100, 1.0000001},
	}
	for _, test := range tests {
		t.Run(strconv.FormatInt(test.nanoseconds, 10), func(t *testing.T) {
			path := writeFile(t, "file", nil)
			moment := time.Unix(0, test.nanoseconds)
			if err := osfile.Chtimes(path, moment, moment); err != nil {
				t.Fatal(err)
			}
			info, err := osfile.Lstat(path)
			if err != nil {
				t.Fatal(err)
			}
			if got := MtimeSeconds(info); got != test.want {
				t.Errorf("got %v, want %v", got, test.want)
			}
		})
	}
}

func TestReadTextAndLoadJSONTreatASideFileAsOptional(t *testing.T) {
	directory := t.TempDir()
	withMark := writeFile(t, "mark.json", []byte("\xef\xbb\xbf{\"a\":1}"))
	notJSON := writeFile(t, "bad.json", []byte("{"))
	tests := []struct {
		name, path, text string
		ok               bool
	}{
		{"a byte-order mark is dropped", withMark, `{"a":1}`, true},
		{"a missing file gives nothing", filepath.Join(directory, "absent.json"), "", false},
		{"a directory gives nothing", directory, "", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var problems Problems
			text, ok := ReadText(test.path, &problems)
			if text != test.text || ok != test.ok || len(problems) > 0 {
				t.Errorf("got %q, %v, %v", text, ok, problems)
			}
		})
	}
	var problems Problems
	if value, ok := LoadJSON(withMark, &problems); !ok || value.JSON() != `{"a":1}` {
		t.Errorf("LoadJSON gave %v, %v", value.JSON(), ok)
	}
	if _, ok := LoadJSON(notJSON, &problems); ok || len(problems) > 0 {
		t.Errorf("text that is not JSON gave %v and %v", ok, problems)
	}
}

func TestOpenAndReadDirRefuseWhatIsNotTheirKind(t *testing.T) {
	directory := t.TempDir()
	file := writeFile(t, "file.txt", []byte("x"))
	_, openErr := Open(directory)
	_, listErr := ReadDir(file)
	tests := []struct {
		name, path string
		err        error
		want       string
	}{
		{"a directory is not opened as a file", directory, openErr,
			filepath.Base(directory) + " could not be read (is a directory)"},
		{"a file is not listed as a directory", file, listErr,
			"file.txt could not be read (not a directory)"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var problems Problems
			problems.AddUnreadable(0, test.path, test.err)
			if got := problems[0].Message; got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
	var problems Problems
	if data, ok := FileBytes(file, &problems); !ok || string(data) != "x" || len(problems) > 0 {
		t.Errorf("FileBytes gave %q, %v, %v", data, ok, problems)
	}
	entries, err := ReadDir(filepath.Dir(file))
	if err != nil || len(entries) != 1 || entries[0].Name() != "file.txt" {
		t.Errorf("ReadDir gave %v, %v", entries, err)
	}
}

func TestReadDirListsInNameOrderAsBytes(t *testing.T) {
	directory := t.TempDir()
	for _, name := range []string{"c", "_x", "a", "B", "é", "a.b", "10", "9"} {
		if err := osfile.WriteFile(filepath.Join(directory, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, entry := range entries {
		got = append(got, entry.Name())
	}
	want := []string{"10", "9", "B", "_x", "a", "a.b", "c", "é"}
	if !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}
