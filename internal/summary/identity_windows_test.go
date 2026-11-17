package summary

import (
	"bytes"
	"testing"
)

// accountLines are the lines of collection.json that name the account on
// Windows, as the document indents them.
func accountLines(identity Identity) string {
	return "  \"run_as\": " + quoted(identity.RunAs) + ",\n  \"run_as_admin\": false,\n"
}

func TestWindowsWritesTheEscapedAccountAndTheAdministratorsFlag(t *testing.T) {
	cases := []struct {
		name     string
		identity Identity
		want     string
	}{
		{"an escaped account in Administrators", Identity{RunAs: `CORP\a"b`, Admin: true},
			"  \"run_as\": \"CORP\\\\a\\\"b\",\n  \"run_as_admin\": true,\n"},
		{"an account outside Administrators", Identity{RunAs: `H\u`},
			"  \"run_as\": \"H\\\\u\",\n  \"run_as_admin\": false,\n"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			var out bytes.Buffer
			summary := sampleSummary()
			summary.Identity = test.identity
			if err := WriteCollectionJSON(&out, summary); err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(out.Bytes(), []byte(test.want)) {
				t.Errorf("collection.json lacks %q:\n%s", test.want, out.String())
			}
			if bytes.Contains(out.Bytes(), []byte("run_as_uid")) {
				t.Error("a Windows collection.json holds run_as_uid")
			}
		})
	}
}
