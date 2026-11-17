package stage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/shortname"
	"github.com/seanthegeek/doubleagent/internal/syspath"
)

func TestMeasureShortNamesInACaseSensitiveDirectory(t *testing.T) {
	var report []string
	for _, marked := range []bool{false, true} {
		parent := filepath.Join(t.TempDir(), "fs")
		if err := os.Mkdir(parent, 0o755); err != nil {
			t.Fatal(err)
		}
		if marked {
			report = append(report, fmt.Sprintf("mark: %v", MarkCaseSensitive(syspath.FromNative(parent))))
		}
		long := filepath.Join(parent, ".claude-backup")
		if err := os.Mkdir(long, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(long, "x"), []byte("through"), 0o600); err != nil {
			t.Fatal(err)
		}
		short := shortname.ShortPath(t, long)
		alias := short[strings.LastIndexByte(short, '\\')+1:]
		data, err := os.ReadFile(filepath.Join(parent, alias, "x"))
		report = append(report, fmt.Sprintf("marked %v: short %q alias %q; read through alias: %q %v",
			marked, short, alias, data, err))
	}
	t.Errorf("measurement:\n%s", strings.Join(report, "\n"))
}
