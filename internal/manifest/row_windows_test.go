package manifest

import (
	"strings"
	"testing"
)

func TestWindowsRowCarriesFieldsAfterMode(t *testing.T) {
	line := string(appendRow(nil, Row{Mode: "", Owner: "O", Attributes: "Hidden"}))
	want := `"mode":"","owner":"O","attributes":"Hidden","sha256"`
	if !strings.Contains(line, want) {
		t.Errorf("row %s lacks %s", line, want)
	}
}

func TestWriterWritesNativePaths(t *testing.T) {
	var out strings.Builder
	writer := NewWriter(&out)
	if err := writer.Write(Row{Home: "C:/Users/a", Path: "C:/Users/a/x"}); err != nil {
		t.Fatal(err)
	}
	if want := `"home":"C:\\Users\\a","agent":"","path":"C:\\Users\\a\\x"`; !strings.Contains(out.String(), want) {
		t.Errorf("row %s lacks %s", out.String(), want)
	}
	if !writer.HasPath("c:/users/A/X") {
		t.Error("HasPath did not fold")
	}
}
