// Package collect_test runs the doubleagent binary's collect over the
// fixture image imagetest builds and compares what it writes with expected
// files (golden_test.go), and checks that every catalog line leaves its
// evidence there (coverage_test.go). The comparison normalises only what
// differs from run to run or machine to machine, and asserts the shape of
// each value it normalises:
//
//   - {OUT}, {WORK} and {NAME} for the output directory (unless it lies in
//     the image), the image's work directory and the run name, raw and
//     JSON-escaped and in each spelling pathSpellings gives, in every text;
//   - the log's leading times (present, never decreasing), its first line's
//     version, host, system and account, the done line's size and hash, and
//     on Windows the line saying whether the system updates last-access
//     times (one of its three wordings);
//   - collection.json's version, hostname, platform, account, started and
//     finished; stdout's archive size and hash; the inventory host;
//   - in the manifest, atime, ctime, btime (not after ctime, and on macOS
//     not after mtime), uid, gid and a Windows owner, each but atime first
//     checked against a fresh lstat of the row's path (checkIdentity),
//     a symlink's size where
//     its target holds a replaced path (its length), and on Linux and macOS
//     a directory row's size, du's disk usage (whole KiB);
//   - directory order: manifest rows sorted, and runs of project blocks,
//     relocation hints and a walk's failure lines in the log, and
//     collection.json's projects and hints.
//
// Modes, modification times, sizes, hashes, archive members (written
// sorted) and every other text are compared as written.
package collect_test

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

// The patterns normalise reads and rewrites.
var (
	logTime       = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z `)
	startLine     = regexp.MustCompile(`^doubleagent (\S+) starting on (\S+) \(([^)]*)\) (mode=.*) (uid=[0-9]+|run_as=.* admin=[01])$`)
	doneLine      = regexp.MustCompile(`^(done: .*) \([0-9]+ bytes, sha256 [0-9a-f]{64}\)$`)
	stdoutArchive = regexp.MustCompile(`(?m)^(size: +)[0-9]+ \([0-9.]+ [KMG]?i?B\)$|^(sha256: +)[0-9a-f]{64}$`)
	runNameShape  = regexp.MustCompile(`^\S+_[0-9]{8}T[0-9]{6}Z_agent-artifacts$`)
	timeShape     = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$`)
)

// volatileRowFields are the manifest fields normalise replaces after
// checking each is a number, and all but atime against the path's lstat.
var volatileRowFields = []string{"atime", "ctime", "btime", "uid", "gid"}

// normaliser holds the replacements of one run.
type normaliser struct {
	test    testing.TB
	paths   *strings.Replacer
	escaped *strings.Replacer
}

// newNormaliser returns the normaliser of a run whose output directory is
// outDir and run name name, over the image whose work directory is work.
func newNormaliser(test testing.TB, work, outDir, name string) normaliser {
	var raw, escaped []string
	add := func(text, placeholder string) {
		if text == "" {
			return
		}
		for _, spelling := range pathSpellings(text) {
			raw = append(raw, spelling, placeholder)
			escaped = append(escaped, jsonEscape(spelling), placeholder)
		}
	}
	// An output directory inside the image keeps its path there, which
	// does not vary.
	add(name, "{NAME}")
	if !strings.HasPrefix(outDir, work+string(filepath.Separator)) {
		add(outDir, "{OUT}")
	}
	add(work, "{WORK}")
	return normaliser{test: test, paths: strings.NewReplacer(raw...), escaped: strings.NewReplacer(escaped...)}
}

// jsonEscape returns text as encoding/json writes it inside a string,
// without the quotes and without escaping HTML characters.
func jsonEscape(text string) string {
	var out strings.Builder
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(text)
	return strings.TrimSuffix(strings.TrimPrefix(strings.TrimSuffix(out.String(), "\n"), `"`), `"`)
}

// normalise returns the golden files of a run: each output as the test
// stores it, keyed by file name; an output the run did not write, or an
// empty stream, has no entry. logOnStderr says stderr carries the log, as it
// does without -q, with no times.
func (n normaliser) normalise(run collectorRun, logOnStderr bool) map[string]string {
	n.test.Helper()
	files := map[string]string{"exit-code.txt": strconv.Itoa(run.code) + "\n"}
	put := func(name, text string) {
		if text != "" {
			files[name] = text
		}
	}
	put("stdout.txt", n.stdout(run.stdout))
	if logOnStderr {
		put("stderr.txt", n.log(run.stderr, false))
	} else {
		put("stderr.txt", n.paths.Replace(run.stderr))
	}
	if run.name == "" {
		return files
	}
	if !runNameShape.MatchString(run.name) {
		n.test.Errorf("run name %q is not <host>_<time>_agent-artifacts", run.name)
	}
	put("manifest.jsonl", n.manifest(run.files[".manifest.jsonl"]))
	put("log.txt", n.log(run.files[".log"], true))
	put("collection.json", n.summary(run.files[".collection.json"]))
	put("archive.txt", n.archive(run))
	return files
}

// stdout returns a run's standard output with its paths, the archive's size
// and hash and the inventory lines' volatile fields replaced.
func (n normaliser) stdout(text string) string {
	text = stdoutArchive.ReplaceAllStringFunc(n.paths.Replace(text), func(line string) string {
		label, _, _ := strings.Cut(line, ":")
		if label == "size" {
			return "size:       {SIZE}"
		}
		return "sha256:     {SHA256}"
	})
	var lines []string
	for _, line := range splitLines(text) {
		lines = append(lines, n.inventoryLine(line))
	}
	return joinLines(lines)
}

// inventoryLine returns an inventory line with its host replaced, and in
// the host line the version in its collector field and the time too; any
// other line is returned as it is.
func (n normaliser) inventoryLine(line string) string {
	if !strings.HasPrefix(line, `{"type":`) {
		return line
	}
	value, err := jsonvalue.DecodeString(line)
	if err != nil {
		n.test.Errorf("inventory line is not JSON: %v: %s", err, line)
		return line
	}
	n.requireText("inventory host", value.Get("host").StringValue() != "")
	value.Set("host", jsonvalue.NewString("{HOST}"))
	if value.Get("type").StringValue() == "host" {
		n.requireText("inventory collector", value.Get("collector").StringValue() != "")
		n.requireText("inventory time", timeShape.MatchString(value.Get("at").StringValue()))
		value.Set("collector", jsonvalue.NewString("{VERSION}"))
		value.Set("at", jsonvalue.NewString("{TIME}"))
	}
	return value.JSON()
}

// requireText fails the test when a normalised value has not its shape.
func (n normaliser) requireText(what string, ok bool) {
	if !ok {
		n.test.Helper()
		n.test.Errorf("%s has not the shape the normalisation expects", what)
	}
}

// manifest returns the manifest's rows sorted, with the volatile fields
// replaced.
func (n normaliser) manifest(text string) string {
	var rows []string
	for _, line := range splitLines(text) {
		rows = append(rows, n.row(line))
	}
	slices.Sort(rows)
	return joinLines(rows)
}

// rowField matches one numeric manifest field and its value, and sizeField
// the size field.
var (
	rowField  = regexp.MustCompile(`"(atime|ctime|btime|uid|gid)":[0-9]+`)
	sizeField = regexp.MustCompile(`"size":[0-9]+`)
)

// row returns one manifest row with its paths and volatile fields replaced,
// after checking the shapes of what it replaces.
func (n normaliser) row(line string) string {
	value, err := jsonvalue.DecodeString(line)
	if err != nil || value.Kind() != jsonvalue.Object {
		n.test.Errorf("manifest row is not a JSON object: %v: %s", err, line)
		return line
	}
	numbers := map[string]int64{}
	for _, field := range append([]string{"mtime"}, volatileRowFields...) {
		number, ok := value.Get(field).Int()
		if !ok || number < 0 {
			n.test.Errorf("manifest row's %s is not a whole number: %s", field, line)
		}
		numbers[field] = number
	}
	birth, change, modified := numbers["btime"], numbers["ctime"], numbers["mtime"]
	if birth != 0 && change != 0 && birth > change {
		n.test.Errorf("manifest row's btime %d is after its ctime %d: %s", birth, change, line)
	}
	if runtime.GOOS == "darwin" && birth > modified {
		n.test.Errorf("manifest row's btime %d is after its mtime %d: %s", birth, modified, line)
	}
	n.checkIdentity(value, numbers, line)
	line = rowField.ReplaceAllStringFunc(n.escaped.Replace(line), func(field string) string {
		name, _, _ := strings.Cut(strings.Trim(field, `"`), `"`)
		return `"` + name + `":"{` + strings.ToUpper(name) + `}"`
	})
	if target := value.Get("target").StringValue(); value.Get("type").StringValue() == "symlink" &&
		n.paths.Replace(target) != target {
		line = n.linkSize(value, target, line)
	}
	if directoryUsageVaries && value.Get("type").StringValue() == "dir" {
		size, _ := value.Get("size").Int()
		if size%1024 != 0 {
			n.test.Errorf("a directory row's size %d is no whole number of KiB: %s", size, line)
		}
		line = sizeField.ReplaceAllString(line, `"size":"{DISK_USAGE}"`)
	}
	return replaceOwner(line)
}

// linkSize returns a symlink row whose target holds a path that varies
// with its size replaced, since the size is the target's length, after
// checking that it is.
func (n normaliser) linkSize(value jsonvalue.Value, target, line string) string {
	size, _ := value.Get("size").Int()
	if linkSizeIsTargetLength && size != int64(len(target)) {
		n.test.Errorf("a symlink row's size %d is not its target's length %d: %s", size, len(target), line)
	}
	return sizeField.ReplaceAllString(line, `"size":"{TARGET_LENGTH}"`)
}

// log returns the log without its times, its first line and done line
// reduced to what does not vary, and the runs of lines whose order is the
// file system's directory order sorted. timed says each line must start
// with a time, as in the log file; the log on stderr has none.
func (n normaliser) log(text string, timed bool) string {
	var lines []string
	previous := ""
	for index, line := range splitLines(text) {
		if timed {
			stamp := logTime.FindString(line)
			if stamp == "" {
				n.test.Errorf("log line %d has no time: %q", index+1, line)
			} else if stamp < previous {
				n.test.Errorf("log line %d's time %s is before the line above's %s", index+1, stamp, previous)
			}
			previous = stamp
			line = strings.TrimPrefix(line, stamp)
		}
		line = n.paths.Replace(line)
		if index == 0 && (timed || startLine.MatchString(line)) {
			line = n.logStart(line)
		}
		lines = append(lines, machineSetting(doneLine.ReplaceAllString(line, "$1 ({SIZE} bytes, sha256 {SHA256})")))
	}
	return joinLines(sortDirectoryOrderedBlocks(lines))
}

// logStart returns the log's first line with the version, host, operating
// system and account replaced.
func (n normaliser) logStart(line string) string {
	match := startLine.FindStringSubmatch(line)
	if match == nil {
		n.test.Errorf("the log's first line has not the shape of a start line: %q", line)
		return line
	}
	account := "uid={UID}"
	if strings.HasPrefix(match[5], "run_as=") {
		account = "run_as={ACCOUNT} admin={ADMIN}"
	}
	return "doubleagent {VERSION} starting on {HOST} ({SYSTEM}) " + match[4] + " " + account
}

// sortDirectoryOrderedBlocks returns the log lines with each run of
// consecutive blocks of one kind whose order follows a directory listing
// sorted: project blocks (a "Project " line and the indented lines below
// it), relocation hint lines, and the failure lines of the files one walk
// reached.
func sortDirectoryOrderedBlocks(lines []string) []string {
	var sorted, run []string
	runKind := ""
	flush := func() {
		slices.Sort(run)
		for _, block := range run {
			sorted = append(sorted, strings.Split(block, "\n")...)
		}
		run, runKind = nil, ""
	}
	for index := 0; index < len(lines); index++ {
		kind := directoryOrderedKind(lines[index])
		if kind != runKind {
			flush()
		}
		if kind == "" {
			sorted = append(sorted, lines[index])
			continue
		}
		block := lines[index]
		for kind == "project" && index+1 < len(lines) && strings.HasPrefix(lines[index+1], "  ") {
			index++
			block += "\n" + lines[index]
		}
		run, runKind = append(run, block), kind
	}
	flush()
	return sorted
}

// directoryOrderedKind names the kind of a block whose place in the log
// follows a directory listing, or "" for any other line.
func directoryOrderedKind(line string) string {
	switch {
	case strings.HasPrefix(line, "Project "):
		return "project"
	case strings.HasPrefix(line, "relocation hint: "):
		return "hint"
	case strings.HasPrefix(line, "read failed: "), strings.HasPrefix(line, "copy failed: "):
		return "failure"
	}
	return ""
}

// summary returns collection.json with its volatile keys replaced and its
// directory-ordered lists sorted, indented as the binary writes it.
func (n normaliser) summary(text string) string {
	value, err := jsonvalue.DecodeString(n.escaped.Replace(text))
	if err != nil || value.Kind() != jsonvalue.Object {
		n.test.Errorf("collection.json is not a JSON object: %v", err)
		return text
	}
	n.requireText("collection.json version", value.Get("version").StringValue() != "")
	n.requireText("collection.json hostname", value.Get("hostname").StringValue() != "")
	n.requireText("collection.json platform", strings.HasPrefix(value.Get("platform").StringValue(), systemName()+" "))
	started, finished := value.Get("started").StringValue(), value.Get("finished").StringValue()
	n.requireText("collection.json started", timeShape.MatchString(started))
	n.requireText("collection.json finished", timeShape.MatchString(finished) && finished >= started)
	for key, placeholder := range map[string]string{"version": "{VERSION}", "hostname": "{HOST}",
		"platform": "{PLATFORM}", "started": "{TIME}", "finished": "{TIME}"} {
		value.Set(key, jsonvalue.NewString(placeholder))
	}
	n.replaceAccount(value)
	value.Set("projects", sortedItems(value.Get("projects")))
	if relocation := value.Get("relocation"); relocation.Kind() == jsonvalue.Object {
		relocation.Set("hints", sortedItems(relocation.Get("hints")))
	}
	return value.IndentedJSON("  ") + "\n"
}

// sortedItems returns an array with its items sorted by their compact JSON,
// or the value as it is when it is not an array.
func sortedItems(list jsonvalue.Value) jsonvalue.Value {
	if list.Kind() != jsonvalue.Array {
		return list
	}
	items := slices.Clone(list.Items())
	slices.SortStableFunc(items, func(first, second jsonvalue.Value) int {
		return strings.Compare(first.JSON(), second.JSON())
	})
	return jsonvalue.NewArray(items...)
}

// archive returns the archive's members, one line each in archive order:
// the quoted name, then the content's SHA-256, "link" and the quoted
// target, or "type" and the tar type. The manifest and collection.json the
// archive holds must equal the files beside it, and its log must begin as
// the log beside it does; each is listed as "beside" once checked.
func (n normaliser) archive(run collectorRun) string {
	var lines []string
	for _, member := range run.members {
		description := member.description
		if besideSuffix, holdsCopy := archivedCopies[member.name]; holdsCopy {
			description = n.archivedCopy(member, run.files[besideSuffix], besideSuffix == ".log")
		} else if target, isLink := strings.CutPrefix(description, "link "); isLink {
			description = "link " + strconv.Quote(n.paths.Replace(target))
		}
		lines = append(lines, strconv.Quote(member.name)+" "+description)
	}
	return joinLines(lines)
}

// archivedCopies names each archive member that copies a file written beside
// the archive, by that file's suffix after the run name.
var archivedCopies = map[string]string{"./manifest.jsonl": ".manifest.jsonl",
	"./collection.json": ".collection.json", "./collector.log": ".log"}

// archivedCopy checks an archived copy against the file beside the archive:
// equal, or for the log a beginning of it, since the log goes on after the
// archive is written. It returns the member's description.
func (n normaliser) archivedCopy(member archiveMember, beside string, prefix bool) string {
	if member.text == beside || (prefix && member.text != "" && strings.HasPrefix(beside, member.text)) {
		return "beside"
	}
	n.test.Errorf("the archive's %s is not the file beside the archive", member.name)
	return member.description
}

// splitLines returns the lines of text, without the newline after the last.
func splitLines(text string) []string {
	if text == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(text, "\n"), "\n")
}

// joinLines returns the lines each ended with a newline, or "" for none.
func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}
