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

	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// Patterns of the Windows normalisation: the log's times, the two start
// lines, the lines only one of the collectors writes, the two wordings it
// spells its own way, the reasons the comparison cuts, and stdout's archive
// lines, resolved count and inventory volatiles.
var (
	windowsLogTime   = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9:]{8}Z `)
	windowsStartLine = regexp.MustCompile(`^\S+ \S+ starting on .* (mode=\S+ root=.*?)(?: (?:uid|run_as)=.*)?$`)
	windowsOptions   = regexp.MustCompile(`^(?:archive|hash)=.*? (max_file_size=.*)$`)
	windowsFlag      = regexp.MustCompile(`=(True|False)\b`)
	powerShellOnly   = regexp.MustCompile(`^(?:discovery source unreadable: |stat failed: |(?:NOTE: )?tar(?:\.exe)? failed)`)
	windowsReason    = regexp.MustCompile(`^((?:read failed|copy failed|symlink not recreated): .+?: |.*? not readable: |` +
		`WARNING: project given with -p is not a directory: .+?: ).*$`)
	windowsStdout    = regexp.MustCompile(`(?m)^(size|sha256): .*\n`)
	windowsResolved  = regexp.MustCompile(`(?m)^(relocation: .*?), [0-9]+ resolved`)
	windowsInventory = regexp.MustCompile(`"(at|collector)":"[^"]*"`)
)

// newWindowsNormaliser returns the normaliser of one case. compare calls it
// on the PowerShell collector's run first and on the binary's second (Go
// evaluates call arguments left to right), and the stderr rule needs both:
// a failed run's error record, kept without its whitespace by the oracle,
// must contain the message the binary prints, whitespace dropped too.
func newWindowsNormaliser() normaliser {
	var reference string
	return func(run collectorRun, binary, logOnStderr bool) collectorRun {
		stderr := withoutCarriageReturns(run.stderr)
		switch {
		case run.code != 0 && !binary:
			reference = stderr
		case run.code != 0:
			stderr = withoutSpace(stderr)
			if stderr != "" && strings.Contains(reference, stderr) {
				stderr = reference
			}
		case logOnStderr:
			stderr = sortedLines(normalWindowsLog(stderr, binary))
		}
		files := map[string]string{
			".manifest.jsonl":  normalManifest(run.files[".manifest.jsonl"]),
			".collection.json": normalSummary(run.files[".collection.json"], binary),
			".log":             normalWindowsLog(run.files[".log"], binary),
			".archive":         normalWindowsArchive(run.members),
		}
		stdout := windowsStdout.ReplaceAllString(withoutCarriageReturns(run.stdout), "")
		stdout = windowsResolved.ReplaceAllString(stdout, "$1")
		stdout = windowsInventory.ReplaceAllString(stdout, `"$1":X`)
		return collectorRun{code: run.code, stdout: stdout, stderr: stderr, files: files}
	}
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
// writes dropped (the binary's sh-style find: line for a directory it cannot
// list; the PowerShell collector's kinds in powerShellOnly), its two
// wordings mapped onto the binary's, and the reasons of failure lines cut.
// binary says the log is the Go binary's.
func normalWindowsLog(text string, binary bool) string {
	var lines []string
	for _, line := range strings.Split(strings.TrimSuffix(withoutCarriageReturns(text), "\n"), "\n") {
		line = windowsLogTime.ReplaceAllString(line, "")
		onlyOne := (binary && strings.HasPrefix(line, "find: ")) || (!binary && powerShellOnly.MatchString(line))
		if onlyOne || strings.HasPrefix(line, "profile not accessible (skipped): ") {
			continue
		}
		lines = append(lines, normalWindowsLine(line))
	}
	return strings.Join(lines, "\n")
}

// normalWindowsLine returns one log line as the comparison sees it.
func normalWindowsLine(line string) string {
	if match := windowsStartLine.FindStringSubmatch(line); match != nil {
		return match[1]
	}
	if match := windowsOptions.FindStringSubmatch(line); match != nil {
		return windowsFlag.ReplaceAllStringFunc(match[1], func(flag string) string {
			if flag == "=True" {
				return "=1"
			}
			return "=0"
		})
	}
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
	if binary {
		blankFileHintValues(summary)
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
// them, each link target without the \\?\ prefix and with / separators.
func normalWindowsArchive(members map[string]string) string {
	normal := make(map[string]string, len(members))
	for name, description := range members {
		if quoted, ok := strings.CutPrefix(description, "link "); ok {
			if target, err := strconv.Unquote(quoted); err == nil {
				target = strings.ReplaceAll(strings.TrimPrefix(target, `\\?\`), `\`, "/")
				description = fmt.Sprintf("link %q", target)
			}
		}
		normal[name] = description
	}
	return normalMembers(normal, normalManifest)
}
