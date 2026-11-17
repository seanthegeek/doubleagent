//go:build parity && unix

package parity

import (
	"slices"
	"strings"
	"testing"
)

func TestUnescapeManifestReadsAJSONStringBody(t *testing.T) {
	cases := []struct{ name, body, want string }{
		{"plain", "fs/home/a/x", "fs/home/a/x"},
		{"quote and backslash", `fs/home/a/quote\"and\\back`, `fs/home/a/quote"and\back`},
		{"named and numbered controls", `tab\tline\nbell\u0007`, "tab\tline\nbell\x07"},
		{"non-ASCII text", "caf\u00e9", "caf\u00e9"},
		{"a body that is not a JSON string", `bad\q`, `bad\q`},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := unescapeManifest(test.body); got != test.want {
				t.Errorf("unescapeManifest(%q) = %q, want %q", test.body, got, test.want)
			}
		})
	}
}

func TestTheGuardFindsRowsNamingNoMember(t *testing.T) {
	rows := []string{
		`{"path":"/a","archive_path":"fs/home/a/x","status":"collected"}`,
		`{"path":"/b","archive_path":"fs/home/a/Y","status":"collected"}`,
		`{"path":"/c","archive_path":"","status":"error_copy"}`,
		`{"path":"/d","archive_path":"fs/home/a/q\"uote","status":"collected"}`,
	}
	run := collectorRun{files: map[string]string{".manifest.jsonl": strings.Join(rows, "\n")},
		members: map[string]string{"./fs/home/a/x": "h", "./fs/home/a/y": "h",
			"./fs/home/a/q\"uote": "h"}}
	if got := archivePathsNamingNoMember(run); !slices.Equal(got, []string{"fs/home/a/Y"}) {
		t.Errorf("archivePathsNamingNoMember = %q; want only fs/home/a/Y, whose member is "+
			"spelled otherwise, and no row without an archive path", got)
	}
}

func TestNormalSummaryIgnoresOnlyLayoutAndTheNamedFields(t *testing.T) {
	shText := `{
  "tool": "collect-agent-artifacts",
  "mode": "image",
  "options": {"full": false, "projects": ["/p"]},
  "capabilities": {"hash_tool": "sha256sum"},
  "relocation": {"hints": [
    {"user": "a", "line": 1}
  ], "registry": {"read": false, "reason": "sh collector"}},
  "archive": "a.tar.gz"
}
`
	binaryText := `{
  "tool": "doubleagent",
  "mode": "image",
  "options": {
    "full": false,
    "projects": [
      "/p"
    ],
    "no_preserve_atime": false
  },
  "capabilities": {
    "hash_tool": "crypto/sha256"
  },
  "relocation": {
    "hints": [
      {
        "user": "a",
        "line": 1
      }
    ],
    "registry": {
      "read": false,
      "reason": "sh collector"
    }
  },
  "archive": "b.tar.gz"
}
`
	want := `"mode":"image"` + "\n" +
		`"options":{"full":false,"projects":["/p"]}` + "\n" +
		`"relocation":{"hints":[{"user":"a","line":1}],"registry":{"read":false,"reason":R}}`
	if got := normalSummary(shText); got != want {
		t.Errorf("normalSummary(sh) = %q, want %q", got, want)
	}
	if got := normalSummary(binaryText); got != want {
		t.Errorf("normalSummary(binary) = %q, want %q", got, want)
	}
	cases := []struct{ name, text string }{
		{"a changed value", strings.Replace(binaryText, `"line": 1`, `"line": 2`, 1)},
		{"an added field", strings.Replace(binaryText, `"mode": "image",`, `"mode": "image", "mode2": 1,`, 1)},
		{"a dropped field", strings.Replace(binaryText, `"line": 1`, `"line2": 1`, 1)},
	}
	for _, test := range cases {
		t.Run(test.name+" still differs", func(t *testing.T) {
			if normalSummary(test.text) == want {
				t.Errorf("normalSummary hid %s", test.name)
			}
		})
	}
}
