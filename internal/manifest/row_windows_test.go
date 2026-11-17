package manifest

import (
	"bytes"
	"strings"
	"testing"
)

// platformFieldsText is what a row holds between mode and sha256 on this
// platform when owner and attributes are empty.
const platformFieldsText = `"owner":"","attributes":"",`

func TestWindowsRowCarriesFieldsAfterMode(t *testing.T) {
	cases := []struct {
		name string
		row  Row
		want string
	}{
		{"both set", Row{Owner: `HOST\alice`, Attributes: "Archive, ReparsePoint"},
			`"mode":"","owner":"HOST\\alice","attributes":"Archive, ReparsePoint","sha256"`},
		{"both empty are still written", Row{}, `"mode":"",` + platformFieldsText + `"sha256"`},
		{"escaped", Row{Owner: "a\"b"}, `"owner":"a\"b","attributes":"","sha256"`},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := lineOf(t, testCase.row); !strings.Contains(got, testCase.want) {
				t.Errorf("row %s lacks %s", got, testCase.want)
			}
		})
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
	if writer.HasPath("c:/users/A/X") {
		t.Error("HasPath matched another spelling the file system was not asked about")
	}
	writer.UseSameEntry(func(string, string) bool { return true })
	if !writer.HasPath("c:/users/A/X") {
		t.Error("HasPath did not find another spelling the file system calls the same entry")
	}
}

func TestWriterKeepsApartNamesNTFSKeepsApart(t *testing.T) {
	var out strings.Builder
	writer := NewWriter(&out)
	writer.UseSameEntry(func(string, string) bool { return true })
	for _, home := range []string{"C:/Users/img", "C:/Users/\u0131mg"} {
		if err := writer.Write(Row{Home: home, Path: home + "/x", Status: Collected}); err != nil {
			t.Fatal(err)
		}
	}
	if !writer.HasPath("c:/users/IMG/X") {
		t.Error("HasPath in another ASCII case = false, want true")
	}
	if !writer.HasHomeRows("C:/Users/img") || !writer.HasHomeRows("C:/Users/\u0131mg") {
		t.Error("HasHomeRows lost img or dotless-i img")
	}
	if writer.HasPath("C:/Users/\u0131mg/y") || !writer.HasPath("C:/Users/\u0131MG/X") {
		t.Error("HasPath does not tell dotless-i img's rows from img's")
	}
}

func TestWriterKeepsApartCaseVariantsTheFileSystemKeepsApart(t *testing.T) {
	var out strings.Builder
	writer := NewWriter(&out)
	asked := 0
	writer.UseSameEntry(func(first, second string) bool {
		asked++
		return strings.EqualFold(first, "C:/Users/Alice/x") && strings.EqualFold(second, first) &&
			second != "C:/Users/alice/x"
	})
	if err := writer.Write(Row{Home: "C:/Users/Alice", Path: "C:/Users/Alice/x"}); err != nil {
		t.Fatal(err)
	}
	if writer.HasPath("C:/Users/alice/x") || !writer.HasPath("C:/Users/ALICE/x") ||
		writer.HasHomeRows("C:/Users/alice") || !writer.HasHomeRows("C:/Users/Alice") {
		t.Error("the writer merged alice, a separate entry, with Alice, or lost ALICE, a second " +
			"spelling of Alice")
	}
	if asked == 0 {
		t.Error("the writer never asked the file system about another spelling")
	}
}

func TestWriterLayout(t *testing.T) {
	row := Row{
		User: "alice", Home: "C:/Users/alice", Agent: "claude-code",
		Path:        "C:/Users/alice/.claude/a\"b\\c\td",
		ArchivePath: "fs/C/Users/alice/.claude/a",
		Type:        "file", Size: 97, Mtime: 3, Atime: 4, Ctime: 5, Btime: 6,
		Mode: "", Owner: `HOST\alice`, Attributes: "Archive",
		SHA256: "abc", Secret: true, Status: Collected,
	}
	want := `{"user":"alice","home":"C:\\Users\\alice","agent":"claude-code",` +
		`"path":"C:\\Users\\alice\\.claude\\a\"b\\c\td","archive_path":"fs/C/Users/alice/.claude/a",` +
		`"type":"file","size":97,"mtime":3,"atime":4,"ctime":5,"btime":6,"uid":0,` +
		`"gid":0,"mode":"","owner":"HOST\\alice","attributes":"Archive","sha256":"abc",` +
		`"secret":true,"status":"collected","target":"","error":""}` + "\n"
	var out bytes.Buffer
	if err := NewWriter(&out).Write(row); err != nil {
		t.Fatal(err)
	}
	if out.String() != want {
		t.Errorf("row =\n%s want\n%s", out.String(), want)
	}
}
