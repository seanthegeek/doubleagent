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

// Patterns of the totals a physical-project exception moves from error_copy
// to collected: the counts line of collection.json and two summary lines.
var (
	countsLine   = regexp.MustCompile(`(?m)^  "counts": .*$`)
	countField   = regexp.MustCompile(`"(collected|error_copy|collected_bytes)": ([0-9]+)`)
	errorField   = regexp.MustCompile(`"error":"(\\.|[^"\\])*"`)
	stdoutTotals = regexp.MustCompile(`(?m)^collected:  ([0-9]+) files?, ([0-9]+) bytes? \([^)]*\)$`)
	stdoutErrors = regexp.MustCompile(`(?m)^errors:     [0-9]+ \(([0-9]+) error_copy, ([0-9]+) error_read\)$`)
)

// physicalProject is a project whose own path runs through a symlinked
// directory, which the binary collects where its files physically are. The
// sh collector stages the files under their recorded path: through a link
// recreated in the staging tree, where the copy fails, or, when no walk
// recreated the link, at the recorded path itself. Paths are relative to the
// image root.
type physicalProject struct {
	owner    string   // the project's owner
	project  string   // the project as -p or an agent's state names it
	link     string   // the symlinked directory the project's log line names
	physical string   // where the project's files physically are
	names    []string // the files' paths inside the project, in manifest order
}

// expect rewrites the sh collector's run into the one the binary writes by
// design: each file's row records its physical archive path, an error_copy
// row becoming a collected one whose copy failed line goes and whose totals
// move by one file; the project's log line names the link; and the archive
// holds each file at its physical path, a file the sh collector archived at
// its recorded path moved there with the directories it alone held. Each
// rewritten text must hold its sh text exactly once, so a change on the sh
// side fails the case instead of passing it silently.
func (c physicalProject) expect(t *testing.T, img image, want *collectorRun) {
	t.Helper()
	failed := 0
	for _, name := range c.names {
		if c.rewriteFile(t, img, want, name) {
			failed++
		}
	}
	want.files[".log"] = c.logThroughLink(t, img, want.files[".log"], failed)
}

// rewriteFile rewrites the run for one file of the project, as expect says,
// and reports whether the sh collector failed to copy it.
func (c physicalProject) rewriteFile(t *testing.T, img image, want *collectorRun, name string) bool {
	t.Helper()
	content, err := os.ReadFile(img.root + "/" + c.physical + "/" + name)
	if err != nil {
		t.Fatal(err)
	}
	archivePath := "fs/" + c.physical + "/" + name
	hash := hashText(string(content))
	file := img.root + "/" + c.project + "/" + name
	manifest, recorded := rowAtPhysicalPath(t, want.files[".manifest.jsonl"], file, archivePath, hash)
	want.files[".manifest.jsonl"] = manifest
	want.members["./manifest.jsonl"], _ = rowAtPhysicalPath(t, want.members["./manifest.jsonl"], file,
		archivePath, hash)
	if recorded == "" {
		size := int64(len(content))
		want.files[".collection.json"] = moveCounts(want.files[".collection.json"], size)
		want.stdout = moveTotals(t, want.stdout, size)
	} else {
		removeMember(want.members, "./"+recorded)
	}
	addMember(want.members, "./"+archivePath, hash)
	return recorded == ""
}

// rowAtPhysicalPath returns manifest with the row of file archived at
// archivePath with hash: an error_copy row made the row of a collected copy,
// or a collected row's archive path replaced. It also returns the archive
// path the collected row had, or "" for an error_copy row.
func rowAtPhysicalPath(t *testing.T, manifest, file, archivePath, hash string) (string, string) {
	t.Helper()
	lines := strings.Split(manifest, "\n")
	found, recorded := 0, ""
	for i, line := range lines {
		if !strings.Contains(line, `"path":"`+file+`","archive_path":"`) {
			continue
		}
		found++
		if strings.Contains(line, `"status":"error_copy"`) {
			lines[i] = rowCollected(line, file, archivePath, hash)
			continue
		}
		recorded = archivePathOf.FindStringSubmatch(line)[1]
		lines[i] = strings.Replace(line, `"archive_path":"`+recorded+`"`,
			`"archive_path":"`+archivePath+`"`, 1)
	}
	if found != 1 {
		t.Fatalf("the sh manifest holds %d rows of %s, want 1", found, file)
	}
	return strings.Join(lines, "\n"), recorded
}

// archivePathOf finds the archive path of a manifest row.
var archivePathOf = regexp.MustCompile(`"archive_path":"((?:\\.|[^"\\])*)"`)

// rowCollected returns line, the error_copy row of file, made the row of a
// collected copy at archivePath with hash.
func rowCollected(line, file, archivePath, hash string) string {
	key := `"path":"` + file + `","archive_path":""`
	line = strings.Replace(line, key, `"path":"`+file+`","archive_path":"`+archivePath+`"`, 1)
	line = strings.Replace(line, `"sha256":""`, `"sha256":"`+hash+`"`, 1)
	line = strings.Replace(line, `"status":"error_copy"`, `"status":"collected"`, 1)
	return errorField.ReplaceAllString(line, `"error":""`)
}

// addMember adds the archive member of a file with hash and its directories.
func addMember(members map[string]string, member, hash string) {
	for end := 2; end < len(member); end++ {
		if member[end] != '/' {
			continue
		}
		if _, found := members[member[:end+1]]; !found {
			members[member[:end+1]] = "type 5"
		}
	}
	members[member] = hash
}

// removeMember removes the archive member of a file and each directory above
// it that then holds nothing.
func removeMember(members map[string]string, member string) {
	delete(members, member)
	for dir := member; ; {
		position := strings.LastIndexByte(strings.TrimSuffix(dir, "/"), '/')
		if position <= 1 {
			return
		}
		dir = dir[:position+1]
		for name := range members {
			if name != dir && strings.HasPrefix(name, dir) {
				return
			}
		}
		delete(members, dir)
	}
}

// logThroughLink returns the log with the project's line naming the link and
// the failed copy lines of the project's files removed, of which there must
// be failed.
func (c physicalProject) logThroughLink(t *testing.T, img image, log string, failed int) string {
	t.Helper()
	project, link := img.root+"/"+c.project, img.root+"/"+c.link
	header := "Project [" + c.owner + "] " + project
	physical := img.root + "/" + c.physical
	var kept []string
	headers, failures := 0, 0
	for _, line := range strings.Split(log, "\n") {
		body := logTime.ReplaceAllString(line, "")
		switch {
		case body == header:
			headers++
			line += " (through the symlinked directory " + link + ", archived under " + physical + ")"
		case strings.HasPrefix(body, "copy failed: "+project+"/"):
			failures++
			continue
		}
		kept = append(kept, line)
	}
	if headers != 1 || failures != failed {
		t.Fatalf("the sh log holds %d lines %q and %d copy failed lines, want 1 and %d",
			headers, header, failures, failed)
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

// hostProject is a project under an image root whose own path leads out of
// the root, through an absolute link a home's walk recorded and recreated:
// the binary does not walk it, since its files are on the analyst's host;
// the sh collector stages them through the recreated link, whose target
// leads out of the staging directory, and writes error_copy rows. Paths are
// relative to the image root.
type hostProject struct {
	owner   string   // the project's owner
	project string   // the project as -p names it
	link    string   // the symlinked directory that leads out of the root
	names   []string // the files the sh collector fails to copy
}

// expect rewrites the sh collector's run into the one the binary writes by
// design: the files' error_copy rows, match lines and copy failed lines go,
// the totals lose those errors, and the project's log line says it is not
// walked.
func (c hostProject) expect(t *testing.T, img image, want *collectorRun) {
	t.Helper()
	for _, name := range c.names {
		file := img.root + "/" + c.project + "/" + name
		want.files[".manifest.jsonl"] = dropErrorRow(t, want.files[".manifest.jsonl"], file)
		want.members["./manifest.jsonl"] = dropErrorRow(t, want.members["./manifest.jsonl"], file)
		want.files[".collection.json"] = countsLine.ReplaceAllStringFunc(want.files[".collection.json"],
			dropCopyError)
		want.stdout = dropTotalsError(t, want.stdout)
	}
	project := img.root + "/" + c.project
	header := "Project [" + c.owner + "] " + project
	var kept []string
	headers, failures, matches := 0, 0, 0
	for _, line := range strings.Split(want.files[".log"], "\n") {
		body := logTime.ReplaceAllString(line, "")
		switch {
		case body == header:
			headers++
			line += ": under the symlinked directory " + img.root + "/" + c.link +
				", which leads out of the image root; not walked"
		case strings.HasPrefix(body, "copy failed: "+project+"/"):
			failures++
			continue
		case strings.HasPrefix(strings.TrimSpace(body), "[") &&
			strings.Contains(body, "] "+project+"/"):
			matches++
			continue
		}
		kept = append(kept, line)
	}
	if headers != 1 || failures != len(c.names) || matches != len(c.names) {
		t.Fatalf("the sh log holds %d lines %q, %d copy failed and %d match lines, want 1, %d and %d",
			headers, header, failures, matches, len(c.names), len(c.names))
	}
	want.files[".log"] = strings.Join(kept, "\n")
}

// dropErrorRow returns manifest without the one error_copy row of file.
func dropErrorRow(t *testing.T, manifest, file string) string {
	t.Helper()
	var kept []string
	found := 0
	for _, line := range strings.Split(manifest, "\n") {
		if strings.Contains(line, `"path":"`+file+`","archive_path":""`) &&
			strings.Contains(line, `"status":"error_copy"`) {
			found++
			continue
		}
		kept = append(kept, line)
	}
	if found != 1 {
		t.Fatalf("the sh manifest holds %d error_copy rows of %s, want 1", found, file)
	}
	return strings.Join(kept, "\n")
}

// dropCopyError returns the counts line of collection.json with one
// error_copy fewer.
func dropCopyError(line string) string {
	return countField.ReplaceAllStringFunc(line, func(field string) string {
		parts := countField.FindStringSubmatch(field)
		if parts[1] != "error_copy" {
			return field
		}
		value, _ := strconv.ParseInt(parts[2], 10, 64)
		return `"error_copy": ` + strconv.FormatInt(value-1, 10)
	})
}

// dropTotalsError returns the summary with one error_copy fewer, in
// print_result's words.
func dropTotalsError(t *testing.T, stdout string) string {
	t.Helper()
	errorLine := stdoutErrors.FindStringSubmatch(stdout)
	if errorLine == nil {
		t.Fatalf("the sh summary lacks its errors line:\n%s", stdout)
	}
	copyErrors, _ := strconv.ParseInt(errorLine[1], 10, 64)
	readErrors, _ := strconv.ParseInt(errorLine[2], 10, 64)
	copyErrors--
	return strings.Replace(stdout, errorLine[0], "errors:     "+
		strconv.FormatInt(copyErrors+readErrors, 10)+" ("+strconv.FormatInt(copyErrors, 10)+
		" error_copy, "+errorLine[2]+" error_read)", 1)
}
