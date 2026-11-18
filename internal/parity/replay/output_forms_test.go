//go:build parity

package replay

import (
	"encoding/csv"
	"errors"
	"io"
	"slices"
	"strings"
)

// outputForms turns the texts of a command case into the form they are
// compared in under the enabled differences of format.
type outputForms struct {
	enabled enabledSet
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
	case strings.HasSuffix(label, ".jsonl"):
		text = f.jsonLines(text)
	case strings.HasSuffix(label, ".csv"):
		text = f.csvRecords(text)
	}
	return text
}

// jsonDocument returns a JSON document in its canonical form (108),
// indented; a text that does not decode is kept as it is.
func (f outputForms) jsonDocument(text string) string {
	if !f.enabled[differenceJSONValues] {
		return text
	}
	value, ok := readWholeJSON(text)
	if !ok {
		return text
	}
	return indented(value.canonical(), "  ") + "\n"
}

// jsonLines returns each line of a JSON Lines text in its canonical form
// (108), its text and formatted fields in their compared form; a line that
// does not decode is kept as it is.
func (f outputForms) jsonLines(text string) string {
	if !f.enabled[differenceJSONValues] {
		return text
	}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		value, ok := readWholeJSON(line)
		if !ok {
			continue
		}
		f.recordFields(value.members)
		lines[i] = value.canonical()
	}
	return strings.Join(lines, "\n")
}

// recordFields puts a timeline row's text, and the formatted fields of a
// row or session, in their compared form, in place.
func (f outputForms) recordFields(members []jsonMember) {
	turnType := ""
	for _, member := range members {
		if text, ok := member.value.scalar.(string); ok && member.key == "turn_type" {
			turnType = text
		}
	}
	for i, member := range members {
		text, ok := member.value.scalar.(string)
		switch {
		case !ok:
		case member.key == "text":
			members[i].value.scalar = rowTextForm(turnType, text, f.enabled)
		case slices.Contains(formattedFields(), member.key):
			members[i].value.scalar = wholeValueForm(text, f.enabled)
		}
	}
}

// csvRecords returns a CSV text as the records encoding/csv reads from it,
// one JSON array a line, with a line in place of each blank line, which
// encoding/csv skips (110); a text it cannot read is kept as it is.
func (f outputForms) csvRecords(text string) string {
	if !f.enabled[differenceCSVRecords] {
		return text
	}
	reader := csv.NewReader(strings.NewReader(text))
	reader.FieldsPerRecord = -1
	var lines []string
	ended := 0
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return text
		}
		start, _ := reader.FieldPos(0)
		lines = append(lines, blankLines(start-ended-1)...)
		last, _ := reader.FieldPos(len(record) - 1)
		ended = last + strings.Count(record[len(record)-1], "\n")
		lines = append(lines, encodeJSON(record, ""))
	}
	physical := strings.Count(text, "\n")
	if text != "" && !strings.HasSuffix(text, "\n") {
		physical++
	}
	lines = append(lines, blankLines(physical-ended)...)
	return strings.Join(lines, "\n") + "\n"
}

// blankLines returns the compared form of count blank lines of a CSV.
func blankLines(count int) []string {
	return slices.Repeat([]string{"(a blank line)"}, max(count, 0))
}
