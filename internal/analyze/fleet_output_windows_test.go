package analyze

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

func TestFleetReplacesAnExistingOutputOnWindows(t *testing.T) {
	// MoveFileEx with MOVEFILE_REPLACE_EXISTING gives the output's name to
	// the new CSV: an ordinary file there is replaced, and a second name of
	// a file below the input (an NTFS hard link) no longer names it, the
	// file keeping its content.
	work := t.TempDir()
	writeFile(t, filepath.Join(work, "inv", "a.json"), hostLine)
	evidence := filepath.Join(work, "inv", "sub", "evidence.txt")
	writeFile(t, evidence, "evidence that must not change\r\n")
	plain := filepath.Join(work, "plain.csv")
	writeFile(t, plain, strings.Repeat("an older and longer CSV\r\n", 20))
	linked := filepath.Join(work, "linked.csv")
	if err := os.Link(evidence, linked); err != nil {
		t.Fatal(err)
	}
	want := strings.Join(FleetColumns(), ",") + "\n" + "h,,,,,,,,,,,\n"
	for _, output := range []string{plain, linked} {
		code, _, stderr := runCommand(testEnvironment(), "fleet", "-o", output, filepath.Join(work, "inv"))
		if data, err := osfile.ReadFile(output); code != ExitFound || err != nil || string(data) != want {
			t.Errorf("%s: exit %d, stderr %q, CSV %q (%v)", output, code, stderr, data, err)
		}
	}
	if data, err := osfile.ReadFile(evidence); err != nil || string(data) != "evidence that must not change\r\n" {
		t.Errorf("the evidence file now holds %q (%v)", data, err)
	}
	if leftovers := temporaryLeftovers(t, work); leftovers != nil {
		t.Errorf("temporary files left: %q", leftovers)
	}
}
