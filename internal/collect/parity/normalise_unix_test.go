//go:build parity && unix

package parity

import (
	"maps"
	"regexp"
	"slices"
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
	archivePathField       = regexp.MustCompile(`"archive_path":"(\\.|[^"\\])*"`)
	// foldingNote is the binary's note that the output directory takes two
	// names for one: letter case on the default macOS volume, Unicode form on
	// every APFS volume; neither script writes it.
	foldingNote = regexp.MustCompile(
		`^(?:[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9:]{8}Z )?NOTE: output directory .* ignores letter case or Unicode form in names;`)
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
		if foldingNote.MatchString(line) {
			continue
		}
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
		if !foldingNote.MatchString(line) {
			lines = append(lines, normalLine(line))
		}
	}
	return strings.Join(lines, "\n")
}

// normalise returns the run with what differs by design dropped. binary says
// the run is the Go binary's, whose log must hold no untimestamped line;
// logOnStderr says stderr carries the log, as it does without -q.
func normalise(run collectorRun, binary, logOnStderr bool) collectorRun {
	if !binary {
		run = reachDottedHome(run)
	}
	archived := dropVolatileManifest
	if run.foldsCase {
		archived = func(text string) string { return caseFreeArchivePaths(dropVolatileManifest(text)) }
	}
	manifest := archived(run.files[".manifest.jsonl"])
	files := map[string]string{
		".manifest.jsonl": manifest,
		".collection.json": summaryNoPreserveAtime.ReplaceAllString(registryReason.ReplaceAllString(
			summaryVolatile.ReplaceAllString(run.files[".collection.json"], ""), `"reason": R`), ""),
		".log":     normalLog(run.files[".log"], binary),
		".archive": normalMembers(run.members, archived),
	}
	stdout := inventoryVolatile.ReplaceAllString(stdoutVolatile.ReplaceAllString(run.stdout, ""),
		`"$1":X`)
	stderr := run.stderr
	if logOnStderr {
		stderr = normalStderr(stderr)
	}
	return collectorRun{code: run.code, stdout: stdout, stderr: stderr, files: files}
}

// caseFreeArchivePaths returns manifest text with every archive_path value
// lower-cased. On an output directory that folds case the binary names a
// row's member as the staging tree spells it (Devin/User/... staged in the
// directory devin made), where the sh collector writes the source's
// spelling; only that field is compared without regard to case, and only
// for such a run, while the archive's member names still compare exactly.
func caseFreeArchivePaths(manifest string) string {
	return archivePathField.ReplaceAllStringFunc(manifest, strings.ToLower)
}

// dropVolatileManifest returns the manifest text without its volatile fields.
func dropVolatileManifest(manifest string) string {
	return manifestVolatile.ReplaceAllString(manifest, "")
}

// The smoke image's passwd home with a `..` after a link (exception 47):
// frank's /srv/dotlink/../frank, srv/dotlink a link to ../opt/escape, is
// opt/frank, which the binary records, logs and archives under that path.
// The sh collector keeps the dotted spelling in every text and stages the
// copy at fs/srv/dotlink/../frank, which mkdir -p makes fs/srv/frank beside
// an empty fs/srv/dotlink directory, and fs/srv/ holds nothing else.
const (
	dottedHome  = "/srv/dotlink/../frank"
	reachedHome = "/opt/frank"
)

// reachDottedHome returns the sh collector's run with frank's dotted home
// spelled as the path the kernel reaches, in its texts and archive members,
// and nothing else changed.
func reachDottedHome(run collectorRun) collectorRun {
	reach := func(text string) string { return strings.ReplaceAll(text, dottedHome, reachedHome) }
	files := map[string]string{}
	for name, text := range run.files {
		files[name] = reach(text)
	}
	members := map[string]string{}
	for name, hash := range run.members {
		switch {
		case name == "./fs/srv/dotlink/":
			continue
		case strings.HasPrefix(name, "./fs/srv/frank/"):
			name = "./fs" + reachedHome + "/" + strings.TrimPrefix(name, "./fs/srv/frank/")
		case name == "./manifest.jsonl":
			hash = reach(hash)
		}
		members[name] = hash
	}
	// fs/srv/ is left a directory member of its own when frank's were its
	// only entries, as in the binary's archive it is not there at all.
	if _, present := members["./fs/srv/"]; present && !slices.ContainsFunc(slices.Collect(
		maps.Keys(members)), func(name string) bool {
		return name != "./fs/srv/" && strings.HasPrefix(name, "./fs/srv/")
	}) {
		delete(members, "./fs/srv/")
	}
	run.files, run.members = files, members
	run.stdout, run.stderr = reach(run.stdout), reach(run.stderr)
	return run
}
