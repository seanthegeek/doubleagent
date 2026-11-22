package vscode

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
)

func TestALogLineThatIsNotAnObjectIsAProblem(t *testing.T) {
	log := []string{`{"kind":0,"v":{"requests":[]}}`, `[1,2]`, `42`, `"text"`, ``, `null`}
	path := filepath.Join(t.TempDir(), "session.jsonl")
	if err := osfile.WriteFile(path, []byte(strings.Join(log, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var problems parser.Problems
	state := replay(nil, path, requestLines{}, &problems, parser.Work{})
	const reason = "unparseable line (not a JSON object)"
	want := parser.Problems{{Line: 2, Message: reason, Raw: "[1,2]"}, {Line: 3, Message: reason, Raw: "42"},
		{Line: 4, Message: reason, Raw: `"text"`}, {Line: 6, Message: reason, Raw: "null"}}
	if !slices.Equal(problems, want) {
		t.Errorf("problems %+v, want %+v", problems, want)
	}
	if got := state.JSON(); got != `{"requests":[]}` {
		t.Errorf("state %s, want the session the first line made", got)
	}
}
