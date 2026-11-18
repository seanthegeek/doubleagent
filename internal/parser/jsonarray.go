package parser

import (
	"iter"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
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
// can. Text that is not an array adds "not a JSON array" at 1. Where an
// element is due, after "[" or a comma, one that does not decode, the end
// of the text included, stops the walk with "unparseable record <position>
// (<reason>)" and the text from there to the end (TailText) as raw. Where a
// comma or "]" is due after an element, anything else adds "array cut off
// after record <n>". The elements before the damage are kept. When end is
// not nil it is set to the offset after the closing bracket, or -1.
func ArrayAt(text string, offset int, problems *Problems, end *int) iter.Seq2[int, jsonvalue.Value] {
	return func(yield func(int, jsonvalue.Value) bool) {
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
			value, next, err := jsonvalue.DecodePrefix(text, start)
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
// file that cannot be read, a damaged array and data after the array, empty
// or not, as "unparseable data after the JSON array", are added to
// problems.
func JSONArray(path string, problems *Problems) iter.Seq2[int, jsonvalue.Value] {
	return func(yield func(int, jsonvalue.Value) bool) {
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

// ArrayElements returns the elements of the JSON array text holds, for
// stores read whole into memory (Gemini CLI histories, Kilo Code tasks),
// walked as ArrayAt walks it except for two conditions, named in the words
// those stores' rows hold: text that ends where an element is due gives
// "array cut off after record <n>" after the last element, not an
// unparseable record; data after a non-empty array gives "unexpected data
// after the array", and data after an empty one nothing.
func ArrayElements(text string, problems *Problems) []jsonvalue.Value {
	var elements []jsonvalue.Value
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
		value, next, err := jsonvalue.DecodePrefix(text, cursor)
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
