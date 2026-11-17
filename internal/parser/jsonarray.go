package parser

import (
	"iter"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/pyjson"
)

// jsonWhitespace is the whitespace JSON allows between tokens.
const jsonWhitespace = " \t\r\n"

// skipJSONWhitespace returns the first offset at or after offset that is
// not JSON whitespace.
func skipJSONWhitespace(text string, offset int) int {
	for offset < len(text) && strings.IndexByte(jsonWhitespace, text[offset]) >= 0 {
		offset++
	}
	return offset
}

// ArrayAt yields (1-based position, element) from the JSON array that
// starts at the byte offset of text, after any whitespace, salvaging what it
// can of a damaged one. An element that does not decode stops the walk and
// adds "unparseable record <position> (<reason>)" at its position, with the
// text from the element to the end (TailText) as raw; the elements before it
// are kept. Text that is not an array adds "not a JSON array" at position 1,
// an array with no closing bracket "array cut off after record <n>". When
// end is not nil it is set to the offset after the closing bracket, or -1.
func ArrayAt(text string, offset int, problems *Problems, end *int) iter.Seq2[int, pyjson.Value] {
	return func(yield func(int, pyjson.Value) bool) {
		setEnd(end, -1)
		cursor := skipJSONWhitespace(text, max(offset, 0))
		if cursor >= len(text) || text[cursor] != '[' {
			problems.Add(1, "not a JSON array")
			return
		}
		cursor = skipJSONWhitespace(text, cursor+1)
		if cursor < len(text) && text[cursor] == ']' {
			setEnd(end, cursor+1)
			return
		}
		for position := 1; ; position++ {
			start := skipJSONWhitespace(text, cursor)
			value, next, err := pyjson.DecodePrefix(text, start)
			if err != nil {
				problems.AddRaw(position, UnparseableRecord(position, err), TailText(text, start))
				return
			}
			if !yield(position, value) {
				return
			}
			cursor = skipJSONWhitespace(text, next)
			switch {
			case cursor < len(text) && text[cursor] == ',':
				cursor++
				continue
			case cursor < len(text) && text[cursor] == ']':
				setEnd(end, cursor+1)
				return
			}
			problems.Add(position+1, "array cut off after record "+strconv.Itoa(position))
			return
		}
	}
}

// setEnd stores offset in end when the caller asked for it.
func setEnd(end *int, offset int) {
	if end != nil {
		*end = offset
	}
}

// JSONArray yields the elements of the file at path holding one JSON array,
// read as ReadText reads an optional file and walked as ArrayAt walks it. A
// file that cannot be read, a damaged array and data after the array are
// added to problems.
func JSONArray(path string, problems *Problems) iter.Seq2[int, pyjson.Value] {
	return func(yield func(int, pyjson.Value) bool) {
		text, ok := ReadText(path, problems)
		if !ok {
			return
		}
		end, count := -1, 0
		for position, value := range ArrayAt(text, 0, problems, &end) {
			count = position
			if !yield(position, value) {
				return
			}
		}
		if end >= 0 && strings.Trim(text[end:], jsonWhitespace) != "" {
			problems.Add(count+1, "unparseable data after the JSON array")
		}
	}
}

// ArrayElements returns the elements of the JSON array text holds, salvaging
// what it can of a damaged one, for stores read whole into memory. A bad
// element stops the walk with "unparseable record <position> (<reason>)" and
// the text from it to the end (TailText) as raw; text that is no array gives
// "not a JSON array" at 1; a missing closing bracket, a trailing comma at the
// end included, gives "array cut off after record <n>" after the last
// element; data after a non-empty array gives "unexpected data after the
// array". Data after an empty array is not reported.
func ArrayElements(text string, problems *Problems) []pyjson.Value {
	var elements []pyjson.Value
	cursor := skipJSONWhitespace(text, 0)
	if cursor >= len(text) || text[cursor] != '[' {
		problems.Add(1, "not a JSON array")
		return elements
	}
	cursor = skipJSONWhitespace(text, cursor+1)
	if cursor < len(text) && text[cursor] == ']' {
		return elements
	}
	for cursor < len(text) {
		position := len(elements) + 1
		value, next, err := pyjson.DecodePrefix(text, cursor)
		if err != nil {
			problems.AddRaw(position, UnparseableRecord(position, err), TailText(text, cursor))
			return elements
		}
		elements = append(elements, value)
		cursor = skipJSONWhitespace(text, next)
		if cursor < len(text) && text[cursor] == ',' {
			cursor = skipJSONWhitespace(text, cursor+1)
			continue
		}
		if cursor < len(text) && text[cursor] == ']' {
			if skipJSONWhitespace(text, cursor+1) < len(text) {
				problems.Add(position+1, "unexpected data after the array")
			}
			return elements
		}
		break
	}
	problems.Add(len(elements)+1, "array cut off after record "+strconv.Itoa(len(elements)))
	return elements
}
