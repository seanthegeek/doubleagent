package analyze

import (
	"archive/tar"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeHomeArchive writes a tar of one home with a Claude history at path.
func writeHomeArchive(t *testing.T, path string) {
	t.Helper()
	var archive bytes.Buffer
	writer := tar.NewWriter(&archive)
	body := "{}\n"
	if err := writer.WriteHeader(&tar.Header{Name: "alice/.claude/history.jsonl", Mode: 0o644,
		Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte(body)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, archive.String())
}

func TestAnInterruptedCommandExits130AndLeavesNothingExtracted(t *testing.T) {
	cases := []struct {
		name string
		args func(dir, archive string) []string
	}{
		{"detect", func(dir, archive string) []string {
			return []string{"detect", "--work-dir", filepath.Join(dir, "work"), "--keep-extracted", archive}
		}},
		{"timeline", func(dir, archive string) []string {
			return []string{"timeline", "--work-dir", filepath.Join(dir, "work"), archive,
				filepath.Join(dir, "out")}
		}},
		{"fleet", func(dir, _ string) []string {
			inventory := filepath.Join(dir, "inventory.jsonl")
			writeFile(t, inventory, `{"type":"host","host":"h"}`+"\n")
			return []string{"fleet", "-o", filepath.Join(dir, "fleet.csv"), inventory}
		}},
	}
	for _, test := range cases {
		t.Run("an interrupted "+test.name+" exits 130", func(t *testing.T) {
			dir := t.TempDir()
			archive := filepath.Join(dir, "home.tar")
			writeHomeArchive(t, archive)
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			var stdout, stderr bytes.Buffer
			code := Run(ctx, test.args(dir, archive), System(), &stdout, &stderr)
			if code != ExitInterrupted || strings.TrimSpace(stderr.String()) != "interrupted" {
				t.Fatalf("exit %d, stderr %q", code, stderr.String())
			}
			entries, _ := os.ReadDir(filepath.Join(dir, "work"))
			if len(entries) != 0 {
				t.Errorf("left in the work directory: %v", entries)
			}
			for _, name := range []string{"out", "fleet.csv"} {
				if _, err := os.Lstat(filepath.Join(dir, name)); err == nil {
					t.Errorf("an interrupted run wrote %s", name)
				}
			}
		})
	}
}
