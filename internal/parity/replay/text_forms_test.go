//go:build parity

package replay

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"slices"
	"strings"
	"time"
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
// under the enabled differences: each JSON span as one encoding (116), and,
// outside prose, each formatted boolean in lower case (113).
func rowTextForm(turnType, text string, enabled enabledSet) string {
	text = structuresForm(text, enabled)
	if !isProse(turnType) && enabled[differenceBoolean] {
		text = formattedBoolean.ReplaceAllStringFunc(text, func(match string) string {
			parts := formattedBoolean.FindStringSubmatch(match)
			return parts[1] + strings.ToLower(parts[2]) + parts[3]
		})
	}
	return text
}

// wholeValueForm returns a formatted field compared as one value: True and
// False in lower case (113).
func wholeValueForm(value string, enabled enabledSet) string {
	if (value == "True" || value == "False") && enabled[differenceBoolean] {
		return strings.ToLower(value)
	}
	return value
}

// formattedBoolean is a boolean in the place a parser formats a value into:
// after "=" or ": ", up to a space, a punctuation mark that ends a value,
// or the end.
var formattedBoolean = regexp.MustCompile(`(=|: )(True|False)([\s,;|)\]}]|\z)`)

// structuresForm returns text with each JSON object or array span as one
// encoding (116).
func structuresForm(text string, enabled enabledSet) string {
	if !enabled[differenceEmbeddedJSON] {
		return text
	}
	var form strings.Builder
	for i := 0; i < len(text); {
		if text[i] == '{' || text[i] == '[' {
			if value, end, found := jsonSpan(text[i:]); found && value.container != 0 {
				form.WriteString(value.canonical())
				i += end
				continue
			}
		}
		form.WriteByte(text[i])
		i++
	}
	return form.String()
}

// jsonValue is a decoded JSON value that keeps what decoding into a map
// loses: an object's members in a list, so a key written twice is kept
// twice, and a number's text as written.
type jsonValue struct {
	// container is '{' for an object, '[' for an array and 0 for a scalar.
	container byte
	// scalar is a scalar's value: a string, a json.Number, a bool or nil.
	scalar  any
	members []jsonMember
	items   []jsonValue
}

// jsonMember is one member of a JSON object.
type jsonMember struct {
	key   string
	value jsonValue
}

// jsonSpan reads the JSON value text starts with, and returns it and its
// length.
func jsonSpan(text string) (jsonValue, int, bool) {
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()
	value, err := readJSONValue(decoder)
	if err != nil {
		return jsonValue{}, 0, false
	}
	return value, int(decoder.InputOffset()), true
}

// readWholeJSON reads text as one JSON value with nothing but JSON
// whitespace after it.
func readWholeJSON(text string) (jsonValue, bool) {
	value, end, ok := jsonSpan(text)
	return value, ok && strings.Trim(text[end:], " \t\r\n") == ""
}

// readJSONValue reads one value from decoder, whose tokens encoding/json
// checks for syntax and nesting.
func readJSONValue(decoder *json.Decoder) (jsonValue, error) {
	token, err := decoder.Token()
	if err != nil {
		return jsonValue{}, err
	}
	delimiter, isDelimiter := token.(json.Delim)
	if !isDelimiter {
		return jsonValue{scalar: token}, nil
	}
	value := jsonValue{container: byte(delimiter)}
	for decoder.More() {
		var key json.Token
		if delimiter == '{' {
			if key, err = decoder.Token(); err != nil {
				return jsonValue{}, err
			}
		}
		item, err := readJSONValue(decoder)
		if err != nil {
			return jsonValue{}, err
		}
		if name, isKey := key.(string); isKey {
			value.members = append(value.members, jsonMember{name, item})
		} else {
			value.items = append(value.items, item)
		}
	}
	if _, err := decoder.Token(); err != nil {
		return jsonValue{}, err
	}
	return value, nil
}

// canonical returns the value as compact JSON with each object's members
// sorted by key, a key written twice kept twice in its order, numbers as
// written and no HTML escaping.
func (v jsonValue) canonical() string {
	var parts []string
	switch v.container {
	case '{':
		members := slices.Clone(v.members)
		slices.SortStableFunc(members, func(left, right jsonMember) int {
			return strings.Compare(left.key, right.key)
		})
		for _, member := range members {
			parts = append(parts, encodeJSON(member.key, "")+":"+member.value.canonical())
		}
		return "{" + strings.Join(parts, ",") + "}"
	case '[':
		for _, item := range v.items {
			parts = append(parts, item.canonical())
		}
		return "[" + strings.Join(parts, ",") + "]"
	}
	return encodeJSON(v.scalar, "")
}

// indented returns compact JSON text indented by indent.
func indented(compact, indent string) string {
	var text bytes.Buffer
	if err := json.Indent(&text, []byte(compact), "", indent); err != nil {
		return compact
	}
	return text.String()
}

// decodeWhole decodes text as one JSON value with nothing but whitespace
// after it, into the types encoding/json gives.
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

// encodeJSON returns a decoded value as encoding/json writes it, keys
// sorted and no HTML escaping, indented by indent when it is not empty.
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

// Timestamps that are values of their own, not a time quoted inside a
// text: in a line of JSON, a whole string after a key or in an array; in
// any other line, a whole word.
var (
	jsonTimestamp = regexp.MustCompile(`[:,\[] ?"(` + timestampText.String() + `)"`)
	wordTimestamp = regexp.MustCompile(`(?:^|\s)(` + timestampText.String() + `)(?:\s|$)`)
)

// rolledOver returns want when got equals it but for whole timestamps each
// of which is want's or one millisecond earlier, where the analyzer's
// rounding of the microseconds carried into the next millisecond;
// otherwise got.
func rolledOver(want, got string) string {
	if want == got {
		return got
	}
	whole := wordTimestamp
	if trimmed := strings.TrimLeft(want, " "); strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, `"`) {
		whole = jsonTimestamp
	}
	wantAt, gotAt := whole.FindAllStringSubmatchIndex(want, -1), whole.FindAllStringSubmatchIndex(got, -1)
	if len(wantAt) == 0 || len(wantAt) != len(gotAt) {
		return got
	}
	wantEnd, gotEnd := 0, 0
	for i := range wantAt {
		if want[wantEnd:wantAt[i][2]] != got[gotEnd:gotAt[i][2]] ||
			!sameOrOneLater(want[wantAt[i][2]:wantAt[i][3]], got[gotAt[i][2]:gotAt[i][3]]) {
			return got
		}
		wantEnd, gotEnd = wantAt[i][3], gotAt[i][3]
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

// rolledOverLines returns got with each line that equals want's line at
// the same index but for timestamps rolled over (119) given want's form.
func rolledOverLines(want, got string) string {
	wantLines, gotLines := strings.Split(want, "\n"), strings.Split(got, "\n")
	for i := range min(len(wantLines), len(gotLines)) {
		gotLines[i] = rolledOver(wantLines[i], gotLines[i])
	}
	return strings.Join(gotLines, "\n")
}
