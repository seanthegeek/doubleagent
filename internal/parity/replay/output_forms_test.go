//go:build parity

package replay

import (
	"encoding/csv"
	"regexp"
	"slices"
	"strings"
)

// outputForms turns the texts of a command case into the form they are
// compared in under the enabled differences of format: arguments are the
// case's command line, root expanded, whose paths 112 reads.
type outputForms struct {
	enabled   enabledSet
	arguments []string
}

// form returns a text of the run, named by label (stdout, stderr or an
// output's path), in its compared form; want is the recorded text of the
// same label, which decides whether a stdout is a JSON document or a table.
func (f outputForms) form(label, want, text string) string {
	switch {
	case label == "stderr":
	case label == "stdout" && strings.HasPrefix(want, "{"),
		strings.HasSuffix(label, ".json"):
		text = f.jsonDocument(text)
	case label == "stdout":
		text = f.table(text)
	case strings.HasSuffix(label, ".jsonl"):
		text = f.jsonLines(text)
	case strings.HasSuffix(label, ".csv"):
		text = f.csvRecords(text)
	}
	return f.paths(text)
}

// jsonDocument returns a JSON document as its decoded value, indented with
// keys sorted and numbers by value (108); a text that does not decode is
// kept as it is.
func (f outputForms) jsonDocument(text string) string {
	if !f.enabled[differenceJSONValues] {
		return text
	}
	value, ok := decodeWhole(f.surrogates(text))
	if !ok {
		return text
	}
	return encodeJSON(numbersByValue(value, true), "  ") + "\n"
}

// jsonLines returns each line of a JSON Lines text as its decoded record,
// compact with keys sorted and numbers by value (108), its text and
// formatted fields in their compared form; a line that does not decode is
// kept as it is.
func (f outputForms) jsonLines(text string) string {
	if !f.enabled[differenceJSONValues] {
		return text
	}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		value, ok := decodeWhole(f.surrogates(line))
		if !ok {
			continue
		}
		if record, isObject := value.(map[string]any); isObject {
			f.recordFields(record)
		}
		lines[i] = encodeJSON(numbersByValue(value, true), "")
	}
	return strings.Join(lines, "\n")
}

// recordFields puts a timeline row's text, and the formatted fields of a
// row or session, in their compared form.
func (f outputForms) recordFields(record map[string]any) {
	turnType, _ := record["turn_type"].(string)
	if text, ok := record["text"].(string); ok {
		record["text"] = rowTextForm(turnType, text, f.enabled)
	}
	for _, field := range formattedFields() {
		if value, ok := record[field].(string); ok {
			record[field] = wholeValueForm(value, f.enabled)
		}
	}
}

// surrogateEscape is a \u escape of a UTF-16 surrogate, or a backslash
// escaped by another, which is taken first so its u is not read as an
// escape.
var surrogateEscape = regexp.MustCompile(`\\\\|\\u[dD][89abAB][0-9a-fA-F]{2}\\u[dD][c-fC-F][0-9a-fA-F]{2}|` +
	`\\u[dD][89a-fA-F][0-9a-fA-F]{2}`)

// surrogates returns JSON text with each escape of a lone surrogate, which
// Python writes and encoding/json reads as U+FFFD, written as U+FFFD under
// 109 and otherwise as its escape's own text, so that it differs from a
// U+FFFD the binary wrote.
func (f outputForms) surrogates(text string) string {
	if !strings.Contains(text, `\u`) {
		return text
	}
	return surrogateEscape.ReplaceAllStringFunc(text, func(escape string) string {
		if escape == `\\` || len(escape) == 12 {
			return escape
		}
		if f.enabled[differenceLoneSurrogate] {
			return `�`
		}
		return `\` + escape
	})
}

// csvRecords returns a CSV text as the records encoding/csv reads from it,
// one JSON array a line (110); a text it cannot read is kept as it is.
func (f outputForms) csvRecords(text string) string {
	if !f.enabled[differenceCSVRecords] {
		return text
	}
	reader := csv.NewReader(strings.NewReader(text))
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		return text
	}
	lines := make([]string, len(records))
	for i, record := range records {
		lines[i] = encodeJSON(record, "")
	}
	return strings.Join(lines, "\n") + "\n"
}

// columnGap is the padding between two columns of a table.
var columnGap = regexp.MustCompile(`  +`)

// table returns a table with each line's trailing spaces removed and each
// run of two or more spaces as one column separator (111).
func (f outputForms) table(text string) string {
	if !f.enabled[differenceTableColumns] {
		return text
	}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = columnGap.ReplaceAllLiteralString(strings.TrimRight(line, " "), " | ")
	}
	return strings.Join(lines, "\n")
}

// paths returns text with each path of the command line that Python's
// pathlib prints in another form written in that form (112), where it ends
// a word: the binary prints a path as it was given.
func (f outputForms) paths(text string) string {
	if !f.enabled[differencePathAsGiven] {
		return text
	}
	for _, given := range f.givenPaths() {
		shape := regexp.MustCompile(regexp.QuoteMeta(given) + `([\s":,)]|\z)`)
		form := pathlibForm(given)
		text = shape.ReplaceAllStringFunc(text, func(match string) string {
			return form + match[len(given):]
		})
	}
	return text
}

// givenPaths returns the values of the command line, an option's joined
// value included, whose pathlib form differs from them, longest first.
func (f outputForms) givenPaths() []string {
	var given []string
	for _, argument := range f.arguments {
		if option, value, joined := strings.Cut(argument, "="); joined && strings.HasPrefix(option, "--") {
			argument = value
		}
		if argument != "" && pathlibForm(argument) != argument {
			given = append(given, argument)
		}
	}
	slices.SortFunc(given, func(left, right string) int { return len(right) - len(left) })
	return given
}

// pathlibForm returns a POSIX path as Python's pathlib prints it: repeated
// slashes and "." components dropped, two leading slashes kept, no
// trailing slash, "." for nothing.
func pathlibForm(path string) string {
	leading := ""
	switch {
	case strings.HasPrefix(path, "//") && !strings.HasPrefix(path, "///"):
		leading = "//"
	case strings.HasPrefix(path, "/"):
		leading = "/"
	}
	var parts []string
	for _, part := range strings.Split(path, "/") {
		if part != "" && part != "." {
			parts = append(parts, part)
		}
	}
	if form := leading + strings.Join(parts, "/"); form != "" {
		return form
	}
	return "."
}

// rolledOverLines returns got with each line that equals want's line at
// the same index but for timestamps rolled over (119) given want's form.
func rolledOverLines(want, got string) string {
	wantLines, gotLines := strings.Split(want, "\n"), strings.Split(got, "\n")
	for i := range min(len(wantLines), len(gotLines)) {
		gotLines[i] = rolledOver(wantLines[i], gotLines[i])
	}
	return strings.Join(gotLines, "\n")
}
