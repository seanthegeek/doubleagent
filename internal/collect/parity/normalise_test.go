//go:build parity

package parity

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Patterns of what the comparison drops: the volatile manifest fields, the
// collection.json keys that differ by design, the log's times, the start
// line's tool and capabilities and the error texts of failure lines.
var (
	manifestVolatile = regexp.MustCompile(`"atime":[0-9]+,|"error":"(\\.|[^"\\])*"`)
	summaryVolatile  = regexp.MustCompile(`(?m)^  "(started|finished|archive|tool|version|capabilities)": .*\n`)
	registryReason   = regexp.MustCompile(`"reason": "[^"]*"`)
	logTime          = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9:]{8}Z `)
	logStart         = regexp.MustCompile(`^\S+ \S+ starting on `)
	logCapability    = regexp.MustCompile(`(hash|stat|worker_sh)=\S+ ?`)
	logDone          = regexp.MustCompile(`^(done: \S+) .*`)
	logReason        = regexp.MustCompile(`^((?:find|read failed|copy failed): [^:]*): .*?` +
		`(; staged path resolves outside the staging directory, not removed)?$`)
	findQuotes        = regexp.MustCompile(`^find: '(.*)': `)
	logProjectError   = regexp.MustCompile(`^(WARNING: project given with -p is not a directory: [^:]*): .*`)
	stdoutVolatile    = regexp.MustCompile(`(?m)^(size|sha256): .*\n`)
	inventoryVolatile = regexp.MustCompile(`"(at|collector)":"[^"]*"`)
)

// normalLine returns a log line with what differs by design dropped: the tool
// and capabilities of the start line, the text after the path of a done line
// and the error text of a failure line.
func normalLine(line string) string {
	line = logStart.ReplaceAllString(line, "TOOL starting on ")
	line = strings.TrimSpace(logCapability.ReplaceAllString(line, ""))
	line = logDone.ReplaceAllString(line, "$1")
	line = logProjectError.ReplaceAllString(line, "$1")
	return logReason.ReplaceAllString(line, "$1$2")
}

// normalLog returns the log's timestamped lines without their times and with
// what differs by design dropped. An untimestamped line is a program's own
// error output, which the sh collector lets into the log; the binary has none,
// so one there is kept to fail the comparison. Of the sh collector's, find's
// lines are kept, without the quotes GNU find puts around the path, since the
// binary writes the same lines with a time; the others are dropped.
func normalLog(text string, keepUntimed bool) string {
	var lines []string
	for _, line := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
		if !logTime.MatchString(line) {
			switch {
			case keepUntimed && line != "":
				lines = append(lines, "untimed: "+line)
			case !keepUntimed && strings.HasPrefix(line, "find: "):
				lines = append(lines, normalLine(findQuotes.ReplaceAllString(line, "find: $1: ")))
			}
			continue
		}
		lines = append(lines, normalLine(logTime.ReplaceAllString(line, "")))
	}
	return strings.Join(lines, "\n")
}

// normalStderr returns the log lines a run without -q writes to stderr, which
// carry no times in either collector.
func normalStderr(text string) string {
	var lines []string
	for _, line := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
		lines = append(lines, normalLine(line))
	}
	return strings.Join(lines, "\n")
}

// normalise returns the run with what differs by design dropped. binary says
// the run is the Go binary's, whose log must hold no untimestamped line;
// logOnStderr says stderr carries the log, as it does without -q.
func normalise(run collectorRun, binary, logOnStderr bool) collectorRun {
	manifest := manifestVolatile.ReplaceAllString(run.files[".manifest.jsonl"], "")
	files := map[string]string{
		".manifest.jsonl": manifest,
		".collection.json": registryReason.ReplaceAllString(
			summaryVolatile.ReplaceAllString(run.files[".collection.json"], ""), `"reason": R`),
		".log":     normalLog(run.files[".log"], binary),
		".archive": normalMembers(run.members),
	}
	stdout := inventoryVolatile.ReplaceAllString(stdoutVolatile.ReplaceAllString(run.stdout, ""),
		`"$1":X`)
	stderr := run.stderr
	if logOnStderr {
		stderr = normalStderr(stderr)
	}
	return collectorRun{code: run.code, stdout: stdout, stderr: stderr, files: files}
}

// normalMembers returns the archive's members as sorted "name hash" lines.
// The archived manifest is hashed after the manifest normalisation, since its
// atime and error texts differ by design, and the archived collection.json
// and collector.log are listed without a hash: the files beside the archive
// are the ones compared.
func normalMembers(members map[string]string) string {
	names := make([]string, 0, len(members))
	for name := range members {
		names = append(names, name)
	}
	sort.Strings(names)
	var lines []string
	for _, name := range names {
		hash := members[name]
		switch name {
		case "./collection.json", "./collector.log":
			hash = "-"
		case "./manifest.jsonl":
			hash = hashText(manifestVolatile.ReplaceAllString(hash, ""))
		}
		lines = append(lines, fmt.Sprintf("%q %s", name, hash))
	}
	return strings.Join(lines, "\n")
}

// compareRuns reports each part of the two normalised runs that differs, with
// the first differing line.
func compareRuns(t *testing.T, want, got collectorRun) {
	t.Helper()
	if want.code != got.code {
		t.Errorf("exit code %d, sh collector %d", got.code, want.code)
	}
	parts := map[string][2]string{"stdout": {want.stdout, got.stdout},
		"stderr": {want.stderr, got.stderr}}
	for suffix, text := range want.files {
		parts[suffix] = [2]string{text, got.files[suffix]}
	}
	for part, texts := range parts {
		for _, line := range differingLines(texts[0], texts[1], 3) {
			t.Errorf("%s differs at line %d:\n sh: %q\ngot: %q", part, line.number, line.want, line.got)
		}
	}
}

// differingLine is one line where two texts differ.
type differingLine struct {
	number    int
	want, got string
}

// differingLines returns the first limit lines where the texts differ.
func differingLines(want, got string, limit int) []differingLine {
	if want == got {
		return nil
	}
	var found []differingLine
	wantLines, gotLines := strings.Split(want, "\n"), strings.Split(got, "\n")
	for i := 0; i < max(len(wantLines), len(gotLines)) && len(found) < limit; i++ {
		if lineAt(wantLines, i) != lineAt(gotLines, i) {
			found = append(found, differingLine{i + 1, lineAt(wantLines, i), lineAt(gotLines, i)})
		}
	}
	return found
}

// lineAt returns line i of lines, or a marker past the end.
func lineAt(lines []string, i int) string {
	if i < len(lines) {
		return lines[i]
	}
	return "<end>"
}
