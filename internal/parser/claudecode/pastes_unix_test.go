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
	members := make([]jsonvalue.Member, placeholders)
	var display strings.Builder
	for i := range placeholders {
		number := strconv.Itoa(i)
		members[i] = jsonvalue.Member{Key: number, Value: decode(t, `{"contentHash": "abc"}`)}
		display.WriteString("[Pasted text #" + number + "] ")
	}
	pasted := jsonvalue.NewObject(members...)
	var problems parser.Problems
	got := ""
	allocations := testing.AllocsPerRun(1, func() {
		problems = nil
		got = expandPastes(home, display.String(), pasted, &problems)
	})
	if got != display.String() || len(problems) != placeholders {
		t.Fatalf("%d problems and the text changed: want the placeholders kept and one problem each",
			len(problems))
	}
	// The pattern's match list takes about one allocation per placeholder;
	// a read of the file takes about a dozen.
	if allocations > 2*placeholders {
		t.Errorf("%.0f allocations for %d placeholders of one unreadable file: read more than once",
			allocations, placeholders)
	}
}
