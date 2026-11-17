//go:build parity && unix

package parity

import (
	"os"
	"runtime"
	"strings"
	"testing"
)

// badName is a file name that is not valid UTF-8, the bytes after "lone" being
// 0xff and 0xfe, and what the binary's manifest writes for it: encoding/json
// puts one U+FFFD character in place of each byte that is not part of a valid
// character (exception 51).
const (
	badName      = "lone\xff\xfe.jsonl"
	badNameShown = "lone��.jsonl"
)

// setupBadName adds a transcript whose name is not valid UTF-8 to alice's
// Claude Code projects. It skips the case where the file system refuses such a
// name, as macOS's does. Linux file systems take any bytes, so a refusal there
// fails the case: the case then cannot pass on every runner without running.
func setupBadName(t *testing.T, img image) {
	build := fixture{t}
	directory := img.root + "/home/alice/.claude/projects/-srv-bad51"
	build.dir(directory)
	if err := os.WriteFile(directory+"/"+badName, []byte("{\"type\":\"user\"}\n"), 0o644); err != nil {
		if runtime.GOOS == "linux" {
			t.Fatalf("Linux refused a name that is not valid UTF-8: %v", err)
		}
		t.Skipf("this file system refuses a name that is not valid UTF-8: %v", err)
	}
}

// shownBadName rewrites the sh collector's run into the one the binary writes
// by design: in the manifest, and in the manifest the archive holds, the
// file's path and archive path carry U+FFFD for each byte of the name that is
// not valid UTF-8. The archive member's name, and every text outside the
// manifest, keep the real bytes. The case fails when the sh manifest does not
// name the file exactly twice (path and archive_path), so a change on the sh
// side cannot pass silently.
func shownBadName(t *testing.T, _ image, want *collectorRun) {
	t.Helper()
	show := func(label, manifest string) string {
		if got := strings.Count(manifest, badName); got != 2 {
			t.Errorf("the sh %s names the file %d times, want 2 (path and archive_path)", label, got)
		}
		return strings.ReplaceAll(manifest, badName, badNameShown)
	}
	want.files[".manifest.jsonl"] = show("manifest", want.files[".manifest.jsonl"])
	want.members["./manifest.jsonl"] = show("archived manifest", want.members["./manifest.jsonl"])
}

func TestShownBadNameRewritesOnlyTheManifestTexts(t *testing.T) {
	row := `{"path":"/h/` + badName + `","archive_path":"fs/h/` + badName + `"}`
	want := collectorRun{
		files:   map[string]string{".manifest.jsonl": row, ".log": "read " + badName},
		members: map[string]string{"./manifest.jsonl": row, "./fs/h/" + badName: "hash"},
	}
	shownBadName(t, image{}, &want)
	shown := `{"path":"/h/` + badNameShown + `","archive_path":"fs/h/` + badNameShown + `"}`
	if want.files[".manifest.jsonl"] != shown || want.members["./manifest.jsonl"] != shown {
		t.Errorf("manifests = %q and %q, want %q", want.files[".manifest.jsonl"],
			want.members["./manifest.jsonl"], shown)
	}
	if want.files[".log"] != "read "+badName || want.members["./fs/h/"+badName] != "hash" {
		t.Errorf("the log or the archive member's name was rewritten: %q, %q", want.files[".log"], want.members)
	}
}
