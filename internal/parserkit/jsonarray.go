package parserkit

import (
	"errors"
	"iter"
	"strconv"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

// ArrayAt yields (1-based position, element) from the JSON array that
// starts at the byte offset of text, after any whitespace, keeping the
// elements before any damage. Text that is not an array adds "not a JSON
// array" at 1. An array that ends before its closing bracket, or lacks a
// comma between two elements, adds "array cut off after record <n>" at n+1,
// n the elements read whole. An element that does not decode adds
// "unparseable record <position> (<reason>)" at its position, with the text
// from there to the end (TailText) as raw. Elements that repeat a member
// name give one problem (repeatedMembers) at the first one's position when
// the walk ends. A non-nil end is set past the closing bracket, or to -1.
func ArrayAt(text string, offset int, problems *Problems, end *int) iter.Seq2[int, jsonvalue.Value] {
	walk := &ArrayWalk{Document: jsonvalue.NewDocument(text)}
	return walk.Array(offset, problems, end)
}

// ArrayWalk walks the JSON arrays of one document as ArrayAt walks one, for
// a caller that reads several arrays of a text in text order and reads no
// part of it twice. After each walk, Stopped is the byte offset it read up
// to: past the closing bracket, where an element was not followed by a
// comma or bracket, or where an element that does not decode failed.
// ShownEnd, when not nil, is given that offset and returns where the raw
// text of the element that does not decode ends; one that starts at or
// after it has none.
type ArrayWalk struct {
	Document *jsonvalue.Document
	// Artifact, when set, is the parse the elements' repeated member names
	// are noted in (NoteRepeats) in place of a problem, for a reader that
	// walks many arrays of one file, which then gives one row.
	Artifact *input.Artifact
	ShownEnd func(stopped int) int
	Stopped  int
}

// Array yields (1-based position, element) from the array that starts at the
// byte offset of the walk's document, with the problems ArrayAt adds.
func (w *ArrayWalk) Array(offset int, problems *Problems, end *int) iter.Seq2[int, jsonvalue.Value] {
	return func(yield func(int, jsonvalue.Value) bool) {
		var noted repeated
		defer func() {
			if w.Artifact != nil {
				NoteRepeats(w.Artifact, noted.repeats)
			} else {
				noted.report(problems)
			}
		}()
		text := w.Document.String()
		setEnd(end, -1)
		cursor := SkipJSONSpace(text, max(offset, 0))
		w.Stopped = cursor
		if cursor >= len(text) || text[cursor] != '[' {
			problems.Add(1, "not a JSON array")
			return
		}
		cursor = SkipJSONSpace(text, cursor+1)
		w.Stopped = cursor
		if cursor < len(text) && text[cursor] == ']' {
			w.Stopped = cursor + 1
			setEnd(end, cursor+1)
			return
		}
		for position := 1; ; position++ {
			start := SkipJSONSpace(text, cursor)
			w.Stopped = start
			if start == len(text) {
				problems.Add(position, cutOff(position-1))
				return
			}
			value, next, err := w.Document.DecodePrefix(start)
			if err != nil {
				w.failed(start, position, err, problems)
				return
			}
			noted.note(position, w.Document.TakeRepeats())
			if !yield(position, value) {
				return
			}
			cursor = SkipJSONSpace(text, next)
			w.Stopped = cursor
			switch {
			case cursor < len(text) && text[cursor] == ',':
				cursor++
				continue
			case cursor < len(text) && text[cursor] == ']':
				w.Stopped = cursor + 1
				setEnd(end, cursor+1)
				return
			}
			problems.Add(position+1, cutOff(position))
			return
		}
	}
}

// failed records the element at start that does not decode: where the
// reader stopped, and its problem with the raw text up to ShownEnd.
func (w *ArrayWalk) failed(start, position int, err error, problems *Problems) {
	text := w.Document.String()
	var decodeError *jsonvalue.Error
	if errors.As(err, &decodeError) {
		w.Stopped = min(max(decodeError.Offset, start), len(text))
	}
	shownEnd := len(text)
	if w.ShownEnd != nil {
		shownEnd = min(max(w.ShownEnd(w.Stopped), 0), len(text))
	}
	raw := ""
	if start < shownEnd {
		raw = TailText(text[:shownEnd], start)
	}
	problems.AddRaw(position, UnparseableRecord(position, err), raw)
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
		if end >= 0 && SkipJSONSpace(text, end) < len(text) {
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
