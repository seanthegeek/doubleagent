//go:build parity && windows

package parity

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// Patterns of the Windows normalisation: the log's times, the two start
// lines, the done line's archive size and hash, the lines only one of the
// collectors writes, the wordings that name an option in each collector's
// own spelling, the reasons the comparison cuts, the PowerShell collector's
// archiver fallback notes, and stdout's archive lines, resolved count and
// inventory volatiles.
var (
	windowsLogTime = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9:]{8}Z `)
	// windowsFoldingNote is the binary's note that the output directory
	// ignores letter case, matched once the time is cut.
	windowsFoldingNote = regexp.MustCompile(
		`^NOTE: output directory .* ignores letter case or Unicode form in names;`)
	windowsStartLine = regexp.MustCompile(`^\S+ \S+ starting on .* (mode=\S+ root=.*?)(?: (?:uid|run_as)=.*)?$`)
	windowsOptions   = regexp.MustCompile(`^(?:archive|hash)=.*? (max_file_size=.*)$`)
	windowsFlag      = regexp.MustCompile(`=(True|False)\b`)
	windowsDone      = regexp.MustCompile(`^(done: .*?) \([0-9]+ bytes, sha256 [0-9a-f]+\)$`)
	inventoryNote    = regexp.MustCompile(`^NOTE: -Inventory writes nothing; -OutputDir (.*) ignored$`)
	invalidSize      = regexp.MustCompile(`(?m)^Invalid (--max-file-size|--env-scan-max-size): .*$`)
	archiverNote     = regexp.MustCompile(`; fell back to |^tar failed; `)
	hintOutcome      = regexp.MustCompile(`^(relocation hint: (.+?) names \S+ \([^)]*\))(?: = .*|; value not resolved: .*)$`)
	powerShellOnly   = regexp.MustCompile(`^(?:discovery source unreadable: |stat failed: |(?:NOTE: )?tar(?:\.exe)? failed)`)
	windowsReason    = regexp.MustCompile(`^((?:read failed|copy failed|symlink not recreated): .+?: |.*? not readable: |` +
		`WARNING: project given with -p is not a directory: .+?: ).*$`)
	windowsStdout    = regexp.MustCompile(`(?m)^(size|sha256): .*\n`)
	windowsResolved  = regexp.MustCompile(`(?m)^(relocation: .*?), [0-9]+ resolved`)
	windowsInventory = regexp.MustCompile(`"(at|collector)":"[^"]*"`)
	// The binary's --no-preserve-atime, which the PowerShell collector does
	// not have, ends the binary's second start line.
	windowsNoPreserveAtime = regexp.MustCompile(` no_preserve_atime=[01]$`)
	// The binary's line saying how it opens files (whether it asks for the
	// right to hold the access time), which the PowerShell collector does
	// not decide.
	accessDecision = regexp.MustCompile(`^(?:last-access updates |--no-preserve-atime: files are opened)`)
	// The binary's lines for a file read without the access-time hold and
	// their count, which the PowerShell collector does not ask for.
	accessNotHeld = regexp.MustCompile(`^access times? not held: `)
)

// newWindowsNormaliser returns the normaliser of one case. compare calls it
// on the PowerShell collector's run first and on the binary's second (Go
// evaluates call arguments left to right), and the stderr rule needs both:
// a failed run's error record, kept without its whitespace by the oracle,
// must contain the message the binary prints, whitespace dropped too. The
// archive rule needs both as well: the binary's target is blanked for each
// link member the PowerShell collector's archive stores without one.
func newWindowsNormaliser() normaliser {
	var reference string
	var emptyTargets map[string]bool
	return func(run collectorRun, binary, logOnStderr bool) collectorRun {
		stderr := withoutCarriageReturns(run.stderr)
		switch {
		case run.code != 0 && !binary:
			reference = stderr
		case run.code != 0:
			stderr = withoutSpace(invalidSize.ReplaceAllStringFunc(stderr, powerShellUsage))
			if stderr != "" && strings.Contains(reference, stderr) {
				stderr = reference
			}
		case logOnStderr:
			stderr = sortedLines(normalWindowsLog(stderr, binary, false))
		}
		manifest, members := run.files[".manifest.jsonl"], run.members
		if binary {
			manifest, members = replaceSurrogates(manifest), replaceMemberSurrogates(members)
			members = blankLinkTargets(members, emptyTargets)
		} else {
			emptyTargets = emptyLinkTargets(members)
		}
		files := map[string]string{
			".manifest.jsonl":  normalManifest(manifest),
			".collection.json": normalSummary(run.files[".collection.json"], binary),
			".log":             normalWindowsLog(run.files[".log"], binary, true),
			".archive":         normalWindowsArchive(members),
		}
		stdout := windowsStdout.ReplaceAllString(withoutCarriageReturns(run.stdout), "")
		stdout = windowsResolved.ReplaceAllString(stdout, "$1")
		stdout = windowsInventory.ReplaceAllString(stdout, `"$1":X`)
		return collectorRun{code: run.code, stdout: stdout, stderr: stderr, files: files}
	}
}

// powerShellUsage returns the PowerShell collector's text for the binary's
// usage error about a size option: the binary names its sh-style option and
// the value, the collector its parameter alone.
func powerShellUsage(message string) string {
	if strings.Contains(message, "--env-scan-max-size") {
		return "Invalid -EnvScanMaxSizeMB"
	}
	return "Invalid -MaxFileSizeMB"
}

// withoutCarriageReturns returns text with CRLF line ends turned into LF.
func withoutCarriageReturns(text string) string {
	return strings.ReplaceAll(text, "\r\n", "\n")
}

// sortedLines returns the lines of text sorted, for a log whose order
// differs by design: the PowerShell collector's inventory walks the homes
// before the volumes.
func sortedLines(text string) string {
	lines := strings.Split(text, "\n")
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

// normalWindowsLog returns the log's lines without their times, each start
// line reduced to what both collectors print, the lines only one collector
// writes dropped (the binary's find:, WARNING: -u, output-directory case
// note, file-open and not-held lines; powerShellOnly), its two wordings mapped onto the binary's, and
// failure reasons cut. binary says the log is the Go binary's; timed says it
// is the log file, whose lines carry times. Every untimed line of the
// PowerShell collector's log file comes after the log is closed, from its
// archiver step (tar's own error output, the fallback notes, staged-link
// cleanup), which the binary has no counterpart for, so those are dropped.
func normalWindowsLog(text string, binary, timed bool) string {
	var lines []string
	for _, line := range strings.Split(strings.TrimSuffix(withoutCarriageReturns(text), "\n"), "\n") {
		archiverLine := timed && !binary && !windowsLogTime.MatchString(line)
		line = windowsLogTime.ReplaceAllString(line, "")
		binaryOnly := strings.HasPrefix(line, "find: ") || strings.HasPrefix(line, "WARNING: -u ") ||
			windowsFoldingNote.MatchString(line) ||
			accessDecision.MatchString(line) || accessNotHeld.MatchString(line)
		onlyOne := (binary && binaryOnly) || (!binary && powerShellOnly.MatchString(line))
		if archiverLine || onlyOne {
			continue
		}
		if binary {
			line = withoutFileHintOutcome(line)
		}
		lines = append(lines, normalWindowsLine(line))
	}
	return strings.Join(lines, "\n")
}

// withoutFileHintOutcome returns a hint log line of the binary without the
// resolved value or the reason it was not resolved when the hint's source is
// a file: the PowerShell collector resolves only registry values and writes
// neither for a file.
func withoutFileHintOutcome(line string) string {
	match := hintOutcome.FindStringSubmatch(line)
	if match == nil || strings.HasPrefix(match[2], `HKLM\`) || strings.HasPrefix(match[2], `HKU\`) {
		return line
	}
	return match[1]
}

// replaceSurrogates returns text with each lone surrogate the binary writes
// as its three WTF-8 bytes (a file name Windows allows but UTF-8 cannot
// hold) replaced by U+FFFD, as the PowerShell collector's UTF-8 writer
// writes it.
func replaceSurrogates(text string) string {
	var out strings.Builder
	for i := 0; i < len(text); i++ {
		if i+2 < len(text) && text[i] == 0xed && text[i+1] >= 0xa0 && text[i+1] <= 0xbf &&
			text[i+2] >= 0x80 && text[i+2] <= 0xbf {
			out.WriteString("\uFFFD")
			i += 2
			continue
		}
		out.WriteByte(text[i])
	}
	return out.String()
}

// replaceMemberSurrogates returns the archive's members with replaceSurrogates
// applied to each name and to the archived manifest's text.
func replaceMemberSurrogates(members map[string]string) map[string]string {
	replaced := make(map[string]string, len(members))
	for name, description := range members {
		if name == "./manifest.jsonl" {
			description = replaceSurrogates(description)
		}
		replaced[replaceSurrogates(name)] = description
	}
	return replaced
}

// normalWindowsLine returns one log line as the comparison sees it.
func normalWindowsLine(line string) string {
	if match := windowsStartLine.FindStringSubmatch(line); match != nil {
		return match[1]
	}
	line = windowsNoPreserveAtime.ReplaceAllString(line, "")
	if match := windowsOptions.FindStringSubmatch(line); match != nil {
		return windowsFlag.ReplaceAllStringFunc(match[1], func(flag string) string {
			if flag == "=True" {
				return "=1"
			}
			return "=0"
		})
	}
	line = windowsDone.ReplaceAllString(line, "$1")
	line = inventoryNote.ReplaceAllString(line, "NOTE: --inventory writes nothing; -o $1 ignored")
	line = strings.Replace(line, "WARNING: no user profile directories found", "WARNING: no user home directories found", 1)
	line = strings.Replace(line, "WARNING: project given with -Project is not a directory: ",
		"WARNING: project given with -p is not a directory: ", 1)
	return windowsReason.ReplaceAllString(line, "$1")
}

// normalManifest returns each manifest row parsed and written back in one
// form, its fields in their order and without atime and error, so escapes
// that differ but mean the same do not count.
func normalManifest(text string) string {
	var rows []string
	for _, line := range strings.Split(strings.TrimSuffix(withoutCarriageReturns(text), "\n"), "\n") {
		if line == "" {
			continue
		}
		row, err := normalRow(line)
		if err != nil {
			row = fmt.Sprintf("unparsable row (%v): %s", err, line)
		}
		rows = append(rows, row)
	}
	return strings.Join(rows, "\n")
}

// normalRow returns one manifest row as key=value pairs in the row's order,
// each value written as Go's JSON encoder writes it.
func normalRow(line string) (string, error) {
	decoder := json.NewDecoder(strings.NewReader(line))
	decoder.UseNumber()
	if token, err := decoder.Token(); err != nil || token != json.Delim('{') {
		return "", fmt.Errorf("not an object: %v", err)
	}
	var fields []string
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return "", err
		}
		var value any
		if err := decoder.Decode(&value); err != nil {
			return "", err
		}
		if key == "atime" || key == "error" {
			continue
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			return "", err
		}
		fields = append(fields, fmt.Sprintf("%v=%s", key, encoded))
	}
	return strings.Join(fields, ","), nil
}

// normalSummary returns collection.json parsed and written back with sorted
// keys, without the keys that differ by design, its root in the canonical
// form and, in the binary's, the values of hints read from files blanked,
// since the PowerShell collector resolves only registry values.
func normalSummary(text string, binary bool) string {
	if text == "" {
		return ""
	}
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()
	var summary map[string]any
	if err := decoder.Decode(&summary); err != nil {
		return fmt.Sprintf("unparsable collection.json (%v): %s", err, text)
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return "collection.json holds more than one value: " + text
	}
	for _, key := range []string{"started", "finished", "archive", "tool", "version", "capabilities", "platform"} {
		delete(summary, key)
	}
	if root, ok := summary["root"].(string); ok {
		summary["root"] = syspath.FromNative(root)
	}
	// The binary's --no-preserve-atime, which the PowerShell collector does
	// not have, is recorded in its options.
	if options, ok := summary["options"].(map[string]any); ok {
		delete(options, "no_preserve_atime")
	}
	if binary {
		blankFileHintValues(summary)
	} else {
		dropArchiverNotes(summary)
	}
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(summary); err != nil {
		return fmt.Sprintf("collection.json not encodable (%v)", err)
	}
	return out.String()
}

// dropArchiverNotes removes the notes the PowerShell collector writes when
// an archiver fails and it falls back to the next one; the binary has one
// archiver.
func dropArchiverNotes(summary map[string]any) {
	notes, ok := summary["notes"].([]any)
	if !ok {
		return
	}
	kept := []any{}
	for _, note := range notes {
		if text, isText := note.(string); !isText || !archiverNote.MatchString(text) {
			kept = append(kept, note)
		}
	}
	summary["notes"] = kept
}

// blankFileHintValues sets the value of every relocation hint whose source
// is not a registry value to "".
func blankFileHintValues(summary map[string]any) {
	relocation, _ := summary["relocation"].(map[string]any)
	hints, _ := relocation["hints"].([]any)
	for _, entry := range hints {
		hint, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		source, _ := hint["source"].(string)
		if !strings.HasPrefix(source, `HKLM\`) && !strings.HasPrefix(source, `HKU\`) {
			hint["value"] = ""
		}
	}
}

// normalWindowsArchive returns the archive's members as normalMembers lists
// them, each link target as normalLinkTarget gives it, and without directory
// members: the binary stages a planned path's directory before deciding its
// status, so a skipped file can leave an empty one, where the PowerShell
// collector creates a directory only when it copies into it. The files imply
// their directories.
func normalWindowsArchive(members map[string]string) string {
	normal := make(map[string]string, len(members))
	for name, description := range members {
		if description == "type 5" {
			continue
		}
		if quoted, ok := strings.CutPrefix(description, "link "); ok {
			if target, err := strconv.Unquote(quoted); err == nil {
				description = fmt.Sprintf("link %q", normalLinkTarget(target))
			}
		}
		normal[name] = description
	}
	return normalMembers(normal, normalManifest)
}

func TestTheWindowsLogDropsTheBinarysAccessTimeNotHeldLines(t *testing.T) {
	lines := []string{
		`2026-01-02T03:04:05Z access time not held: C:\img\Users\a\.claude\x.json: right to hold refused`,
		"2026-01-02T03:04:05Z access times not held: 1 file(s)",
		"2026-01-02T03:04:05Z Archiving",
	}
	cases := []struct {
		name   string
		binary bool
		want   string
	}{
		{"the binary's lines are dropped", true, "Archiving"},
		{"the same text in the PowerShell collector's log is kept", false,
			strings.Join([]string{
				`access time not held: C:\img\Users\a\.claude\x.json: right to hold refused`,
				"access times not held: 1 file(s)", "Archiving"}, "\n")},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got := normalWindowsLog(strings.Join(lines, "\n")+"\n", test.binary, true)
			if got != test.want {
				t.Errorf("normalWindowsLog = %q, want %q", got, test.want)
			}
		})
	}
}
