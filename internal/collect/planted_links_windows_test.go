package collect

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/syspath"
)

func TestAJunctionAtAnOutputNameIsNeverFollowed(t *testing.T) {
	name := runName()
	cases := []struct{ name, entry string }{
		{"a junction at the staging directory's name ends the run before it starts", ".stage-" + name},
		{"a junction at the log's name ends the run before it starts", name + ".log"},
		{"a junction at the manifest's name ends the run before it starts", name + ".manifest.jsonl"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			outDir := longTempDir(t)
			outside := newOutsideFiles(t)
			planted := filepath.Join(outDir, testCase.entry)
			// Every account may make a junction, unlike a symlink.
			makeJunction(t, syspath.FromNative(outside.directory), syspath.FromNative(planted))
			var stdout, stderr bytes.Buffer
			code := runCollection(context.Background(), testOptions(fixtureImage(t), outDir), testHost(t),
				&stdout, &stderr)
			outside.checkUntouched(t)
			if code != ExitFatal || !strings.Contains(stderr.String(), planted) {
				t.Errorf("exit code %d and stderr %q, want %d and a message naming %s", code,
					stderr.String(), ExitFatal, planted)
			}
		})
	}
}
