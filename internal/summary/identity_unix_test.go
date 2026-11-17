//go:build unix

package summary

import (
	"bytes"
	"strconv"
	"testing"
)

// accountLines are the lines of collection.json that name the account on
// Unix, as the document indents them.
func accountLines(identity Identity) string {
	return "  \"run_as_uid\": " + quoted(strconv.Itoa(identity.UID)) + ",\n"
}

func TestUnixWritesTheUserIDAsAString(t *testing.T) {
	var out bytes.Buffer
	summary := sampleSummary()
	summary.Identity = Identity{UID: 501, RunAs: "ignored", Admin: true}
	if err := WriteCollectionJSON(&out, summary); err != nil {
		t.Fatal(err)
	}
	want := "  \"platform\": \"Linux 6 x86_64\",\n  \"run_as_uid\": \"501\",\n  \"started\""
	if !bytes.Contains(out.Bytes(), []byte(want)) {
		t.Errorf("collection.json lacks %s:\n%s", want, out.String())
	}
	if bytes.Contains(out.Bytes(), []byte("run_as_admin")) {
		t.Error("a Unix collection.json holds the Windows account fields")
	}
}
