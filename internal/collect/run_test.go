package collect

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/catalog"
)

func TestRun(t *testing.T) {
	cases := []struct {
		name       string
		options    Options
		action     Action
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{"list prints the catalog", Options{}, ActList, 0, string(catalog.Text()), ""},
		{"version prints the tool and version", Options{}, ActVersion, 0, "doubleagent 0.0.0-dev", ""},
		{"help prints the usage", Options{}, ActHelp, 0, UsageText, ""},
		{"a run is not implemented yet", Options{}, ActRun, 2, "", "not implemented yet"},
		{"a bad root is fatal", Options{Root: "/no/such/dir"}, ActRun, 2, "",
			"Root is not a directory: /no/such/dir\n"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run(context.Background(), testCase.options, testCase.action, &stdout, &stderr)
			if code != testCase.wantCode {
				t.Errorf("exit code %d, want %d", code, testCase.wantCode)
			}
			if !strings.HasPrefix(stdout.String(), testCase.wantStdout) {
				t.Errorf("stdout %q, want prefix %q", stdout.String(), testCase.wantStdout)
			}
			if !strings.Contains(stderr.String(), testCase.wantStderr) {
				t.Errorf("stderr %q, want it to contain %q", stderr.String(), testCase.wantStderr)
			}
		})
	}
}
