package agentzero

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// toolFilesChat returns a chat.json whose history holds a tool message per
// name, t<i> naming messages/<name>, and whose log holds a tool item each.
func toolFilesChat(names []string) string {
	var messages, logs []string
	for i, name := range names {
		messageID := "t" + strconv.Itoa(i)
		messages = append(messages, `{"_cls":"Message","id":"`+messageID+`","ai":false,"content":{"tool_name":"x","file":"`+
			name+`"}}`)
		logs = append(logs, `{"type":"tool","id":"`+messageID+`","content":""}`)
	}
	history := `{"_cls":"History","current":{"_cls":"Topic","messages":[` + strings.Join(messages, ",") + `]}}`
	return `{"id":"ctx","agents":[{"number":0,"history":` + jsonvalue.NewString(history).JSON() +
		`}],"log":{"logs":[` + strings.Join(logs, ",") + `]}}`
}

// Tool messages naming one message file by spellings the disk opens it by
// (another case, or an accent written as a combining mark) read it once,
// and its text is in one row; two files are both read. A spelling the
// disk tells apart names no file, and the case says so.
func TestEverySpellingOfOneMessageFileReadsItOnce(t *testing.T) {
	tests := []struct {
		name    string
		named   []string
		files   []string
		anyCase bool
		reads   int
	}{
		{"four spellings of one file read it once", []string{"big.txt", "BIG.TXT", "Big.txt", "bIg.TxT"},
			[]string{"big.txt"}, true, 1},
		{"spellings in accented capitals read one file once", []string{"\u00e9t\u00e9.txt", "\u00c9T\u00c9.txt", "\u00c9t\u00e9.TXT"},
			[]string{"\u00e9t\u00e9.txt"}, true, 1},
		{"a spelling with combining accents reads one file once", []string{"\u00e9t\u00e9.txt", "e\u0301te\u0301.txt"},
			[]string{"\u00e9t\u00e9.txt"}, true, 1},
		{"two files are both read", []string{"big.txt", "other.txt"}, []string{"big.txt", "other.txt"}, false, 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			const rel = "agent-zero/usr/chats/ctx/chat.json"
			writeFile(t, home, rel, toolFilesChat(test.named))
			for _, name := range test.files {
				writeFile(t, home, "agent-zero/usr/chats/ctx/messages/"+name, "FILE TEXT OF "+name)
			}
			for _, name := range test.named[1:] {
				spelled := filepath.Join(home, "agent-zero", "usr", "chats", "ctx", "messages", name)
				if _, err := osfile.Lstat(spelled); test.anyCase && err != nil {
					t.Skip("this disk tells the spellings apart, so they open two names")
				}
			}
			artifact := &input.Artifact{DiskPath: filepath.Join(home, filepath.FromSlash(rel)), Original: "/h/" + rel,
				Rel: rel, Home: &input.Home{DiskPath: home}}
			subject := New()
			subject.Work = parser.CountingWork()
			shown := 0
			if err := subject.Parse(artifact, parser.Options{}, func(row timeline.Row) {
				shown += strings.Count(row.FullText(), "FILE TEXT OF ")
			}); err != nil {
				t.Fatal(err)
			}
			if reads := subject.Work.Count("message files read"); reads != test.reads || shown != test.reads {
				t.Errorf("read %d times and shown in %d rows, want %d of each", reads, shown, test.reads)
			}
		})
	}
}
