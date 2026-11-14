//go:build parity

package parity

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/summary"
)

// Patterns of the totals a collected-instead exception moves from error_copy
// to collected: the counts line of collection.json and two summary lines.
var (
	countsLine   = regexp.MustCompile(`(?m)^  "counts": .*$`)
	countField   = regexp.MustCompile(`"(collected|error_copy|collected_bytes)": ([0-9]+)`)
	errorField   = regexp.MustCompile(`"error":"(\\.|[^"\\])*"`)
	stdoutTotals = regexp.MustCompile(`(?m)^collected:  ([0-9]+) files?, ([0-9]+) bytes? \([^)]*\)$`)
	stdoutErrors = regexp.MustCompile(`(?m)^errors:     [0-9]+ \(([0-9]+) error_copy, ([0-9]+) error_read\)$`)
)

// collectedInstead is a file the sh collector fails to copy and the binary
// collects by design: a project reached through a link inside its owner's
// home, which the binary stages where the home physically holds it. Paths
// are relative to the image root.
type collectedInstead struct {
	owner    string // the project's owner
	project  string // the project as -p names it, through the link
	link     string // the symlinked directory the project's path runs through
	physical string // where the home physically holds the project
	name     string // the file's path inside the project
}

// expect rewrites the sh collector's run into the one the binary writes by
// design: the file's error_copy row becomes a collected row archived at its
// physical path, the project's log line names the link, the copy failed
// line goes, the totals move by one file, and the archive gains the file and
// its directories. Each rewritten text must hold its sh text exactly once,
// so a change on the sh side fails the case instead of passing it silently.
func (c collectedInstead) expect(t *testing.T, img image, want *collectorRun) {
	t.Helper()
	content, err := os.ReadFile(img.root + "/" + c.physical + "/" + c.name)
	if err != nil {
		t.Fatal(err)
	}
	archivePath := "fs/" + c.physical + "/" + c.name
	want.files[".manifest.jsonl"] = c.rowCollected(t, img, want.files[".manifest.jsonl"],
		archivePath, hashText(string(content)))
	want.members["./manifest.jsonl"] = c.rowCollected(t, img, want.members["./manifest.jsonl"],
		archivePath, hashText(string(content)))
	want.files[".log"] = c.logThroughLink(t, img, want.files[".log"])
	size := int64(len(content))
	want.files[".collection.json"] = moveCounts(want.files[".collection.json"], size)
	want.stdout = moveTotals(t, want.stdout, size)
	member := "./" + archivePath
	for end := 2; end < len(member); end++ {
		if member[end] != '/' {
			continue
		}
		if _, found := want.members[member[:end+1]]; !found {
			want.members[member[:end+1]] = "type 5"
		}
	}
	want.members["./"+archivePath] = hashText(string(content))
}

// rowCollected returns manifest with the file's error_copy row made the row
// of a collected copy at archivePath with hash.
func (c collectedInstead) rowCollected(t *testing.T, img image, manifest, archivePath, hash string,
) string {
	t.Helper()
	file := img.root + "/" + c.project + "/" + c.name
	key := `"path":"` + file + `","archive_path":""`
	lines := strings.Split(manifest, "\n")
	found := 0
	for i, line := range lines {
		if !strings.Contains(line, key) || !strings.Contains(line, `"status":"error_copy"`) {
			continue
		}
		found++
		line = strings.Replace(line, key, `"path":"`+file+`","archive_path":"`+archivePath+`"`, 1)
		line = strings.Replace(line, `"sha256":""`, `"sha256":"`+hash+`"`, 1)
		line = strings.Replace(line, `"status":"error_copy"`, `"status":"collected"`, 1)
		lines[i] = errorField.ReplaceAllString(line, `"error":""`)
	}
	if found != 1 {
		t.Fatalf("the sh manifest holds %d error_copy rows of %s, want 1", found, file)
	}
	return strings.Join(lines, "\n")
}

// logThroughLink returns the log with the project's line naming the link and
// the file's copy failed line removed.
func (c collectedInstead) logThroughLink(t *testing.T, img image, log string) string {
	t.Helper()
	project, link := img.root+"/"+c.project, img.root+"/"+c.link
	header := "Project [" + c.owner + "] " + project
	physical := img.root + "/" + c.physical
	failed := "copy failed: " + project + "/" + c.name + ": "
	var kept []string
	headers, failures := 0, 0
	for _, line := range strings.Split(log, "\n") {
		body := logTime.ReplaceAllString(line, "")
		switch {
		case body == header:
			headers++
			line += " (through the symlinked directory " + link + ", archived under " + physical + ")"
		case strings.HasPrefix(body, failed):
			failures++
			continue
		}
		kept = append(kept, line)
	}
	if headers != 1 || failures != 1 {
		t.Fatalf("the sh log holds %d lines %q and %d copy failed lines, want 1 each",
			headers, header, failures)
	}
	return strings.Join(kept, "\n")
}

// moveCounts returns collection.json with one error_copy counted as a
// collected file of size bytes.
func moveCounts(text string, size int64) string {
	return countsLine.ReplaceAllStringFunc(text, func(line string) string {
		return countField.ReplaceAllStringFunc(line, func(field string) string {
			return moveCount(field, size)
		})
	})
}

// moveCount returns one field of the counts line moved as moveCounts says.
func moveCount(field string, size int64) string {
	parts := countField.FindStringSubmatch(field)
	value, _ := strconv.ParseInt(parts[2], 10, 64)
	switch parts[1] {
	case "collected":
		value++
	case "error_copy":
		value--
	default:
		value += size
	}
	return `"` + parts[1] + `": ` + strconv.FormatInt(value, 10)
}

// moveTotals returns the summary with one error_copy counted as a collected
// file of size bytes, in print_result's words.
func moveTotals(t *testing.T, stdout string, size int64) string {
	t.Helper()
	totals, errorLine := stdoutTotals.FindStringSubmatch(stdout), stdoutErrors.FindStringSubmatch(stdout)
	if totals == nil || errorLine == nil {
		t.Fatalf("the sh summary lacks its collected or errors line:\n%s", stdout)
	}
	files, _ := strconv.ParseInt(totals[1], 10, 64)
	bytes, _ := strconv.ParseInt(totals[2], 10, 64)
	copyErrors, _ := strconv.ParseInt(errorLine[1], 10, 64)
	readErrors, _ := strconv.ParseInt(errorLine[2], 10, 64)
	files, bytes, copyErrors = files+1, bytes+size, copyErrors-1
	stdout = strings.Replace(stdout, totals[0], "collected:  "+summary.CountNoun(files, "file")+", "+
		summary.CountNoun(bytes, "byte")+" ("+summary.HumanSize(bytes)+")", 1)
	return strings.Replace(stdout, errorLine[0], "errors:     "+
		strconv.FormatInt(copyErrors+readErrors, 10)+" ("+strconv.FormatInt(copyErrors, 10)+
		" error_copy, "+errorLine[2]+" error_read)", 1)
}
