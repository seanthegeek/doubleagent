package analyze

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/reason"
)

// temporaryLeftovers returns the names in directory that fleet's temporary
// files are given.
func temporaryLeftovers(t *testing.T, directory string) []string {
	t.Helper()
	entries, err := osfile.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	var found []string
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), fleetTemporaryPrefix) {
			found = append(found, entry.Name())
		}
	}
	return found
}

func TestFleetReplacesAnExistingOutputWithTheNewCSV(t *testing.T) {
	work := t.TempDir()
	writeFile(t, filepath.Join(work, "inv", "a.json"), hostLine)
	output := filepath.Join(work, "out.csv")
	writeFile(t, output, strings.Repeat("an older and longer CSV\n", 20))
	code, _, stderr := runCommand(testEnvironment(), "fleet", "-o", output, filepath.Join(work, "inv"))
	want := strings.Join(FleetColumns(), ",") + "\n" + "h,,,,,,,,,,,\n"
	if data, err := osfile.ReadFile(output); code != ExitFound || err != nil || string(data) != want {
		t.Errorf("exit %d, stderr %q, CSV %q (%v)", code, stderr, data, err)
	}
	if leftovers := temporaryLeftovers(t, work); leftovers != nil {
		t.Errorf("temporary files left: %q", leftovers)
	}
}

func TestFleetLeavesNoTemporaryFileWhenTheOutputIsADirectory(t *testing.T) {
	work := t.TempDir()
	writeFile(t, filepath.Join(work, "inv", "a.json"), hostLine)
	output := filepath.Join(work, "out.csv")
	writeFile(t, filepath.Join(output, "kept"), "kept\n")
	code, stdout, stderr := runCommand(testEnvironment(), "fleet", "-o", output, filepath.Join(work, "inv"))
	if code != ExitError || stdout != "" || stderr != "error: "+output+": "+reason.IsDirectory+"\n" {
		t.Errorf("exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	if leftovers := temporaryLeftovers(t, work); leftovers != nil {
		t.Errorf("temporary files left: %q", leftovers)
	}
	if data, err := osfile.ReadFile(filepath.Join(output, "kept")); err != nil || string(data) != "kept\n" {
		t.Errorf("the directory's file now holds %q (%v)", data, err)
	}
}
