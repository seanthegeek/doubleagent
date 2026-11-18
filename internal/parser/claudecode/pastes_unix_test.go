//go:build !windows

package claudecode

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
)

func TestAPasteCacheFileNamedManyTimesIsReadOnce(t *testing.T) {
	home := t.TempDir()
	cacheFile := filepath.Join(home, "paste-cache", "abc.txt")
	if err := osfile.MkdirAll(filepath.Dir(cacheFile), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(cacheFile, []byte("text"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(cacheFile, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := parser.ReadFile(cacheFile); err == nil {
		t.Skip("this user reads a file without read permission")
	}
	const placeholders = 2000
	allocations := func(hash string) (float64, string, int) {
		members := make([]jsonvalue.Member, placeholders)
		var display strings.Builder
		for i := range placeholders {
			number := strconv.Itoa(i)
			members[i] = jsonvalue.Member{Key: number, Value: decode(t, `{"contentHash": "`+hash+`"}`)}
			display.WriteString("[Pasted text #" + number + "] ")
		}
		pasted := jsonvalue.NewObject(members...)
		var problems parser.Problems
		got := ""
		count := testing.AllocsPerRun(1, func() {
			problems = nil
			got = expandPastes(home, display.String(), pasted, &problems)
		})
		if got != display.String() {
			t.Fatalf("the placeholders of %q changed: %q", hash, got)
		}
		return count, got, len(problems)
	}
	// A name that is not a plain hash is never looked up, so the difference
	// between the two is the cost of reading the file: about a dozen
	// allocations a read.
	unread, _, unreadProblems := allocations("not/a/hash")
	read, _, problems := allocations("abc")
	if unreadProblems != 0 || problems != placeholders {
		t.Fatalf("%d and %d problems, want none and one per placeholder", unreadProblems, problems)
	}
	if read-unread > placeholders/2 {
		t.Errorf("%.0f allocations more for %d placeholders of one unreadable file than for none: "+
			"read more than once", read-unread, placeholders)
	}
}
