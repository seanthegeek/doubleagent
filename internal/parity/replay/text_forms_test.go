//go:build parity

package replay

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math"
	"math/big"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// proseTurns are the turn types whose text a person or a tool wrote as it
// stands: a word such as True or None there is the text itself, never a
// formatted value, so it is compared exactly. In the other turn types
// (system, tool_use) the parser formats values into its own lead-in.
func proseTurns() []string {
	return []string{"user", "assistant", "thinking", "tool_result"}
}

// isProse reports whether a turn type's text is prose.
func isProse(turnType string) bool {
	return slices.Contains(proseTurns(), turnType)
}

// rowTextForm returns a row's text in the form both sides are compared in,
// under the enabled differences: a lone surrogate as U+FFFD (109), the
// information separators U+001C to U+001F trimmed with the whitespace
// around them (118), each JSON span and list or object repr as one JSON
// encoding (116, 117), and, outside prose, each formatted scalar as one
// token (113, 114, 115).
func rowTextForm(turnType, text string, enabled enabledSet) string {
	text = surrogateForm(text, enabled)
	if enabled[differenceSeparators] {
		text = separatorsTrimmed(text)
	}
	text = structuresForm(text, enabled)
	if !isProse(turnType) {
		text = scalarsForm(text, enabled)
	}
	return text
}

// wholeValueForm returns a formatted field compared as one value: True and
// False in lower case (113), None as empty (114), a number by value (115).
func wholeValueForm(value string, enabled enabledSet) string {
	value = surrogateForm(value, enabled)
	switch {
	case (value == "True" || value == "False") && enabled[differenceBoolean]:
		return strings.ToLower(value)
	case value == "None" && enabled[differenceNull]:
		return ""
	case enabled[differenceNumber] && numberText.MatchString(value):
		return canonicalNumber(value)
	}
	return value
}

// surrogateForm returns text with each lone surrogate, which the corpus
// keeps in its three-byte UTF-8 form, as U+FFFD, when 109 is enabled.
func surrogateForm(text string, enabled enabledSet) string {
	if !enabled[differenceLoneSurrogate] || !strings.Contains(text, "\xed") {
		return text
	}
	var form strings.Builder
	for i := 0; i < len(text); i++ {
		if i+2 < len(text) && text[i] == 0xed && text[i+1] >= 0xa0 && text[i+1] <= 0xbf &&
			text[i+2] >= 0x80 && text[i+2] <= 0xbf {
			form.WriteRune(utf8.RuneError)
			i += 2
			continue
		}
		form.WriteByte(text[i])
	}
	return form.String()
}

// isSeparator reports whether character is an information separator,
// U+001C to U+001F, which Python's strip removes and unicode.IsSpace does
// not.
func isSeparator(character rune) bool {
	return character >= 0x1c && character <= 0x1f
}

// separatorsTrimmed returns text with each run of whitespace that holds an
// information separator removed where it starts or ends the text or a
// line: what Python's strip removed and strings.TrimSpace keeps.
func separatorsTrimmed(text string) string {
	lines := strings.Split(separatorRunTrimmed(text), "\n")
	for i, line := range lines {
		lines[i] = separatorRunTrimmed(line)
	}
	return strings.Join(lines, "\n")
}

// separatorRunTrimmed trims the leading and the trailing run of whitespace
// of text, each only when it holds an information separator.
func separatorRunTrimmed(text string) string {
	space := func(character rune) bool { return unicode.IsSpace(character) || isSeparator(character) }
	if left := strings.TrimLeftFunc(text, space); strings.ContainsFunc(text[:len(text)-len(left)],
		isSeparator) {
		text = left
	}
	if right := strings.TrimRightFunc(text, space); strings.ContainsFunc(text[len(right):], isSeparator) {
		text = right
	}
	return text
}

// numberText is a number as Python's repr or the evidence spells one.
var numberText = regexp.MustCompile(`\A-?[0-9]+(?:\.[0-9]+)?(?:[eE][-+]?[0-9]+)?\z`)

// formattedScalar is a scalar in the place a parser formats a value into:
// after "=" or ": ", up to a space, a punctuation mark that ends a value,
// or the end.
var formattedScalar = regexp.MustCompile(`(=|: )(True|False|None|-?[0-9]+(?:\.[0-9]+)?(?:[eE][-+]?[0-9]+)?)` +
	`([\s,;|)\]}]|\z)`)

// scalarsForm returns text with each formatted scalar in one form, under
// the enabled differences among 113, 114 and 115.
func scalarsForm(text string, enabled enabledSet) string {
	if !enabled[differenceBoolean] && !enabled[differenceNull] && !enabled[differenceNumber] {
		return text
	}
	return formattedScalar.ReplaceAllStringFunc(text, func(match string) string {
		parts := formattedScalar.FindStringSubmatch(match)
		return parts[1] + wholeValueForm(parts[2], enabled) + parts[3]
	})
}

// canonicalNumber returns a number's text by its value: an integer exactly,
// a fraction or exponent as the nearest double, which an integral double
// gives as an integer. A number no double holds keeps its text.
func canonicalNumber(text string) string {
	if !strings.ContainsAny(text, ".eE") {
		var value big.Int
		if _, ok := value.SetString(text, 10); ok {
			return value.String()
		}
		return text
	}
	value, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return strings.ToLower(text)
	}
	if value == math.Trunc(value) {
		integer, _ := big.NewFloat(value).Int(nil)
		return integer.String()
	}
	return strconv.FormatFloat(value, 'g', -1, 64)
}

// structuresForm returns text with each JSON object or array span as one
// encoding (116), and each span that is not JSON but a Python list or dict
// repr as the same encoding of its value (117).
func structuresForm(text string, enabled enabledSet) string {
	if !enabled[differenceEmbeddedJSON] && !enabled[differenceListRepr] {
		return text
	}
	var form strings.Builder
	for i := 0; i < len(text); {
		if text[i] == '{' || text[i] == '[' {
			value, end, found := jsonSpan(text[i:])
			switch {
			case !found && enabled[differenceListRepr]:
				value, end, found = reprSpan(text[i:])
			case found && !enabled[differenceEmbeddedJSON]:
				found = false
			}
			if found {
				form.WriteString(encodeJSON(numbersByValue(value, enabled[differenceNumber]), ""))
				i += end
				continue
			}
		}
		form.WriteByte(text[i])
		i++
	}
	return form.String()
}

// jsonSpan decodes the JSON value text starts with, and returns it and its
// length.
func jsonSpan(text string) (any, int, bool) {
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()
	var value any
	if decoder.Decode(&value) != nil {
		return nil, 0, false
	}
	return value, int(decoder.InputOffset()), true
}

// decodeWhole decodes text as one JSON value with nothing but whitespace
// after it.
func decodeWhole(text string) (any, bool) {
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()
	var value any
	if decoder.Decode(&value) != nil {
		return nil, false
	}
	_, err := decoder.Token()
	return value, errors.Is(err, io.EOF)
}

// numbersByValue returns a decoded value with each number in its
// canonical text when byValue is set.
func numbersByValue(value any, byValue bool) any {
	if !byValue {
		return value
	}
	switch typed := value.(type) {
	case json.Number:
		return json.Number(canonicalNumber(string(typed)))
	case []any:
		items := make([]any, len(typed))
		for i, item := range typed {
			items[i] = numbersByValue(item, true)
		}
		return items
	case map[string]any:
		members := make(map[string]any, len(typed))
		for key, item := range typed {
			members[key] = numbersByValue(item, true)
		}
		return members
	}
	return value
}

// encodeJSON returns a decoded value as JSON with keys sorted and no HTML
// escaping, indented by indent when it is not empty.
func encodeJSON(value any, indent string) string {
	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", indent)
	if err := encoder.Encode(value); err != nil {
		return "(not encodable: " + err.Error() + ")"
	}
	return strings.TrimSuffix(encoded.String(), "\n")
}

// timestampText is a timestamp as the outputs write one.
var timestampText = regexp.MustCompile(`[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}\.[0-9]{3}Z`)

// rolledOver returns want when got equals it but for timestamps each of
// which is want's or one millisecond earlier, where Python's rounding of
// the microseconds carried into the next millisecond; otherwise got.
func rolledOver(want, got string) string {
	if want == got {
		return got
	}
	wantAt, gotAt := timestampText.FindAllStringIndex(want, -1), timestampText.FindAllStringIndex(got, -1)
	if len(wantAt) == 0 || len(wantAt) != len(gotAt) {
		return got
	}
	wantEnd, gotEnd := 0, 0
	for i := range wantAt {
		if want[wantEnd:wantAt[i][0]] != got[gotEnd:gotAt[i][0]] ||
			!sameOrOneLater(want[wantAt[i][0]:wantAt[i][1]], got[gotAt[i][0]:gotAt[i][1]]) {
			return got
		}
		wantEnd, gotEnd = wantAt[i][1], gotAt[i][1]
	}
	if want[wantEnd:] != got[gotEnd:] {
		return got
	}
	return want
}

// sameOrOneLater reports whether the timestamp want equals got or is one
// millisecond later.
func sameOrOneLater(want, got string) bool {
	if want == got {
		return true
	}
	wantTime, wantErr := time.Parse(time.RFC3339Nano, want)
	gotTime, gotErr := time.Parse(time.RFC3339Nano, got)
	return wantErr == nil && gotErr == nil && wantTime.Sub(gotTime) == time.Millisecond
}
