//go:build parity && unix

package parity

import (
	"regexp"
	"strings"
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
	// The binary's --no-preserve-atime, which the sh collector does not
	// have, is recorded in collection.json's options and the log's second
	// start line.
	summaryNoPreserveAtime = regexp.MustCompile(`, "no_preserve_atime": (true|false)`)
	logNoPreserveAtime     = regexp.MustCompile(` no_preserve_atime=[01]$`)
)

// normalLine returns a log line with what differs by design dropped: the tool
// and capabilities of the start line, the text after the path of a done line
// and the error text of a failure line.
func normalLine(line string) string {
	line = logStart.ReplaceAllString(line, "TOOL starting on ")
	line = logNoPreserveAtime.ReplaceAllString(line, "")
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
		".collection.json": summaryNoPreserveAtime.ReplaceAllString(registryReason.ReplaceAllString(
			summaryVolatile.ReplaceAllString(run.files[".collection.json"], ""), `"reason": R`), ""),
		".log":     normalLog(run.files[".log"], binary),
		".archive": normalMembers(run.members, dropVolatileManifest),
	}
	stdout := inventoryVolatile.ReplaceAllString(stdoutVolatile.ReplaceAllString(run.stdout, ""),
		`"$1":X`)
	stderr := run.stderr
	if logOnStderr {
		stderr = normalStderr(stderr)
	}
	return collectorRun{code: run.code, stdout: stdout, stderr: stderr, files: files}
}

// dropVolatileManifest returns the manifest text without its volatile fields.
func dropVolatileManifest(manifest string) string {
	return manifestVolatile.ReplaceAllString(manifest, "")
}
