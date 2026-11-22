//go:build unix

package walk

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// updateGolden is the environment variable that rewrites the golden files
// instead of comparing with them.
const updateGolden = "DOUBLEAGENT_UPDATE_GOLDEN"

// rowTimeField matches a modification time, which is zero only on a row for
// an item that was never read; the other time fields and the owner ids can be
// zero on some file systems and are matched with rowNumberField.
var (
	rowTimeField   = regexp.MustCompile(`"mtime":[1-9][0-9]*`)
	rowNumberField = regexp.MustCompile(`"(atime|ctime|btime|uid|gid)":[0-9]+`)
)

// normaliseRows returns the rows with the image root written as <root>, every
// modification time of a row that was read written as N, and every other
// time and owner field written as N, sorted: the order of a directory's
// entries is the file system's. It fails the test unless each read row had its
// six fields replaced, so each one was present and a number.
func normaliseRows(t *testing.T, rows []string, root string) []string {
	t.Helper()
	var normal []string
	for _, row := range rows {
		row = strings.ReplaceAll(row, root, "<root>")
		wantFields := 6
		if strings.Contains(row, `"mtime":0,`) {
			wantFields = 5
		}
		replaced := 0
		row = rowTimeField.ReplaceAllStringFunc(row, func(string) string {
			replaced++
			return `"mtime":N`
		})
		row = rowNumberField.ReplaceAllStringFunc(row, func(field string) string {
			replaced++
			return field[:strings.Index(field, ":")+1] + "N"
		})
		if replaced != wantFields {
			t.Errorf("row has %d time and owner fields, want %d: %s", replaced, wantFields, row)
		}
		normal = append(normal, row)
	}
	slices.Sort(normal)
	return normal
}

// normaliseLog returns the log lines with the image root written as <root>.
// A line follows the match line it belongs to, so the groups keep their
// order; the lines of one group come in the file system's directory order and
// are sorted. The note that two spellings name one file exists only on a file
// system that folds case, so it is returned apart.
func normaliseLog(lines []string, root string) (log, foldNotes []string) {
	var body []string
	flush := func() {
		slices.Sort(body)
		log = append(log, body...)
		body = nil
	}
	for _, line := range lines {
		line = strings.ReplaceAll(line, root, "<root>")
		switch {
		case strings.Contains(line, "(case-insensitive filesystem)"):
			foldNotes = append(foldNotes, line)
		case strings.HasPrefix(line, "  ["), strings.HasPrefix(line, "User "):
			flush()
			log = append(log, line)
		default:
			body = append(body, line)
		}
	}
	flush()
	return log, foldNotes
}

// listStage returns one line per entry of the staging directory, sorted by
// path: a directory, a file with its content hash, or a link with its target.
// Each path is quoted, since names hold newlines and quotes.
func listStage(t *testing.T, stageDir string) []string {
	t.Helper()
	var entries []string
	err := filepath.WalkDir(stageDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative := strings.TrimPrefix(path, stageDir)
		if relative == "" {
			return nil
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		switch {
		case info.Mode()&fs.ModeSymlink != 0:
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			entries = append(entries, "link "+strconv.Quote(relative)+" -> "+strconv.Quote(target))
		case info.IsDir():
			entries = append(entries, "dir "+strconv.Quote(relative))
		default:
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			sum := sha256.Sum256(content)
			entries = append(entries, "file "+strconv.Quote(relative)+" "+hex.EncodeToString(sum[:]))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return entries
}

// checkGolden compares got with the lines of testdata/name, or rewrites the
// file when the update variable is set.
func checkGolden(t *testing.T, name string, got []string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	text := strings.Join(got, "\n") + "\n"
	if os.Getenv(updateGolden) == "1" {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if text != string(want) {
		t.Errorf("%s differs from the collected output\nwant:\n%s\ngot:\n%s", path, want, text)
	}
}

func TestEdgeImageRowsLogAndStagingTree(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("the image holds directories no one may read, which the superuser can")
	}
	root := edgeImage(t)
	for _, settings := range edgeRuns {
		t.Run(settings.name, func(t *testing.T) {
			run := collectEdgeImage(t, root, settings)
			name := strings.NewReplacer(" ", "_").Replace(settings.name)
			checkGolden(t, "edge_"+name+".rows", normaliseRows(t, run.rows, root))
			log, foldNotes := normaliseLog(run.log, root)
			checkGolden(t, "edge_"+name+".log", log)
			checkGolden(t, "edge_"+name+".stage", listStage(t, run.stage))
			if folds := foldsCase(t); folds != (len(foldNotes) > 0) {
				t.Errorf("file system folds case = %v, notes of two spellings for one file = %q",
					folds, foldNotes)
			}
		})
	}
}
