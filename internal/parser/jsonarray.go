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
// can of a damaged one; the elements before the damage are kept. Text that
// is not an array adds "not a JSON array" at 1. An array whose text ends
// before its closing bracket, or that lacks a comma between two elements,
// adds "array cut off after record <n>" at n+1, n the elements read whole.
// An element that starts but does not decode adds "unparseable record
// <position> (<reason>)" at its position, with the text from there to the
// end (TailText) as raw. When end is not nil it is set to the offset after
// the closing bracket, or -1.
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
			if start == len(text) {
				problems.Add(position, cutOff(position-1))
				return
			}
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
			problems.Add(position+1, cutOff(position))
			return
		}
	}
}

// cutOff is the message of an array cut off after count whole elements.
func cutOff(count int) string {
	return "array cut off after record " + strconv.Itoa(count)
}

// setEnd stores offset in end when the caller asked for it.
func setEnd(end *int, offset int) {
	if end != nil {
		*end = offset
	}
}

// ArrayText yields the elements of the JSON array text holds, walked as
// ArrayAt walks it from the start of text. Text after the closing bracket
// other than JSON whitespace, after an empty array too, adds "unexpected
// data after the array" after the last element.
func ArrayText(text string, problems *Problems) iter.Seq2[int, jsonvalue.Value] {
	return func(yield func(int, jsonvalue.Value) bool) {
		end, count := -1, 0
		for position, value := range ArrayAt(text, 0, problems, &end) {
			count = position
			if !yield(position, value) {
				return
			}
		}
		if end >= 0 && skipJSONWhitespace(text, end) < len(text) {
			problems.Add(count+1, "unexpected data after the array")
		}
	}
}

// JSONArray yields the elements of the file at path holding one JSON array,
// read as ReadText reads an optional file and walked as ArrayText walks it,
// with the same problems; a file that cannot be read is one more.
func JSONArray(path string, problems *Problems) iter.Seq2[int, jsonvalue.Value] {
	return func(yield func(int, jsonvalue.Value) bool) {
		text, ok := ReadText(path, problems)
		if !ok {
			return
		}
		for position, value := range ArrayText(text, problems) {
			if !yield(position, value) {
				return
			}
		}
	}
}

// ArrayElements returns the elements ArrayText yields from text, with its
// problems, for stores read whole into memory.
func ArrayElements(text string, problems *Problems) []jsonvalue.Value {
	var elements []jsonvalue.Value
	for _, value := range ArrayText(text, problems) {
		elements = append(elements, value)
	}
	return elements
}
