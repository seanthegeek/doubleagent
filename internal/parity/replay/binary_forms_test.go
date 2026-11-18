//go:build parity

package replay

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"strings"
)

// The functions of this file write a recorded text the way the binary is
// to write it after its formatting changes, so a test can show that the
// information comparison accepts each change.

// binaryJSON returns a JSON document as encoding/json writes its decoded
// value: keys sorted, UTF-8, HTML characters escaped, a final newline.
func binaryJSON(text string, indent string) string {
	value, ok := decodeWhole(text)
	if !ok {
		return text
	}
	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	encoder.SetIndent("", indent)
	if err := encoder.Encode(value); err != nil {
		return text
	}
	return encoded.String()
}

// binaryJSONLines returns each line of a JSON Lines text as encoding/json
// writes its decoded value.
func binaryJSONLines(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = strings.TrimSuffix(binaryJSON(line, ""), "\n")
		}
	}
	return strings.Join(lines, "\n")
}

// binaryCSV returns a CSV text as encoding/csv writes the records it reads
// from it.
func binaryCSV(text string) string {
	reader := csv.NewReader(strings.NewReader(text))
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		return text
	}
	var written bytes.Buffer
	writer := csv.NewWriter(&written)
	if writer.WriteAll(records) != nil {
		return text
	}
	return written.String()
}

// binaryRowText returns a row's text as the binary is to write it: each
// JSON span compact in its own key order with numbers as written, and,
// outside prose, a formatted True or False in lower case.
func binaryRowText(turnType, text string) string {
	var form strings.Builder
	for i := 0; i < len(text); {
		if text[i] == '{' || text[i] == '[' {
			if _, end, ok := jsonSpan(text[i:]); ok {
				var compact bytes.Buffer
				if json.Compact(&compact, []byte(text[i:i+end])) == nil {
					form.Write(compact.Bytes())
					i += end
					continue
				}
			}
		}
		form.WriteByte(text[i])
		i++
	}
	text = form.String()
	if isProse(turnType) {
		return text
	}
	return formattedBoolean.ReplaceAllStringFunc(text, func(match string) string {
		parts := formattedBoolean.FindStringSubmatch(match)
		return parts[1] + strings.ToLower(parts[2]) + parts[3]
	})
}
