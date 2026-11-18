//go:build darwin || linux

package vscodestate

import (
	"os"
	"slices"
	"testing"
)

func TestReadItemsReportsAnUnreadableDatabaseOrSidecar(t *testing.T) {
	for _, suffix := range []string{"", "-wal"} {
		t.Run("an unreadable state.vscdb"+suffix+" is the problem", func(t *testing.T) {
			path := stateCopy(t)
			if err := os.WriteFile(path+"-wal", nil, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(path+suffix, 0); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(path+suffix, 0o600) })
			if file, err := os.Open(path + suffix); err == nil {
				file.Close()
				t.Skip("a file with mode 0 is still readable here")
			}
			values, problems := ReadItems(path, []string{"cloudAgentRepository.agents.u1"})
			want := []string{"state.vscdb" + suffix + " could not be read (permission denied)"}
			if len(values) != 0 || !slices.Equal(problems, want) {
				t.Errorf("ReadItems() = %v, %q; want nothing and %q", values, problems, want)
			}
		})
	}
	shared := stateCopy(t)
	if err := os.Symlink(shared, shared+"-shm"); err != nil {
		t.Fatal(err)
	}
	if values, problems := ReadItems(shared, []string{"cloudAgentRepository.agents.u1"}); len(values) != 1 ||
		problems != nil {
		t.Errorf("beside a linked -shm file: %v, %q; want the value read", values, problems)
	}
	path := stateCopy(t)
	link := path + ".link"
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if values, problems := ReadItems(link, []string{"cloudAgentRepository.agents.u1"}); len(values) != 0 ||
		problems != nil {
		t.Errorf("a linked database gave %v, %q; want nothing read", values, problems)
	}
}
