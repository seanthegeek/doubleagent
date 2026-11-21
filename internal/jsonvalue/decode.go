package jsonvalue

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// Error is a document that is not JSON. Offset counts bytes from the start
// of the text given to the decoder; Line and Column count from 1, the
// column in characters. Err is the standard library's error, or
// io.ErrUnexpectedEOF when the text ends inside a value.
type Error struct {
	Offset int
	Line   int
	Column int
	Err    error
}

// Error returns the message with its position, for example "invalid JSON
// at line 1, column 4: invalid character ']' looking for beginning of
// value".
func (e *Error) Error() string {
	detail := e.Err.Error()
	if errors.Is(e.Err, io.ErrUnexpectedEOF) {
		detail = "unexpected end of JSON input"
	}
	// The position says where; the reader's note of what it expected
	// there is left out.
	detail, _, _ = strings.Cut(detail, " (expecting")
	return fmt.Sprintf("invalid JSON at line %d, column %d: %s", e.Line, e.Column, detail)
}

// Unwrap returns the standard library's error.
func (e *Error) Unwrap() error { return e.Err }

// errExtraData is the cause of an error for text after a whole document.
var errExtraData = errors.New("extra data after the value")

// MaxDepth is how deep arrays and objects of a decoded document may nest.
// A document deeper than this is refused, so the writer (JSON,
// IndentedJSON, MarshalJSON and Text) and Equal, which recurse once per
// level, stay within a bounded stack for any value decoded; a value built
// with NewArray and NewObject is not held to it. The number is the token
// reader's own limit where it has one, so both builds refuse the same
// documents with the same error.
const MaxDepth = 10_000

// errTooDeep is the cause of an error for an array or object that opens
// deeper than MaxDepth, worded as the token reader words its own.
var errTooDeep = errors.New("exceeded max depth")

// Document is a JSON text whose values are read one at a time with
// DecodePrefix, for a reader that walks the values of one text in order,
// such as the arrays of a damaged history. An error's line and column are
// counted on from where the last error's count ended, rather than from the
// start of the text, so a walk that meets an error in each of many values
// still reads each byte of the text a fixed number of times.
type Document struct {
	text string
	// counted is the place the last count of lines and columns reached;
	// scanned is how many bytes the counts have read.
	counted place
	scanned int
	// repeats holds what the values returned since the last TakeRepeats
	// repeated.
	repeats Repeats
}

// place is a byte offset of a text with its line and its column in
// characters, both counted from 1.
type place struct {
	offset, line, column int
}

// NewDocument returns text as a Document whose values are read in text
// order.
func NewDocument(text string) *Document {
	return &Document{text: text, counted: place{line: 1, column: 1}}
}

// Scanned returns how many bytes the document has read to count the lines
// and columns of its errors.
func (d *Document) Scanned() int { return d.scanned }

// String returns the document's text.
func (d *Document) String() string { return d.text }

// errorAt returns the error for reading the text up to byte offset. The
// line and column are counted on from the last count when the offset lies
// at or after it, and from the start of the text when it lies before; an
// offset outside the text is placed at its nearer end. A count kept for
// the next error ends at the start of a character, so an offset inside a
// character of several bytes, such as a DecodePrefix start, gives every
// later error the column a count from the start of the text gives.
func (d *Document) errorAt(offset int, cause error) *Error {
	within := min(max(offset, 0), len(d.text))
	from := d.counted
	if within < from.offset {
		from = place{line: 1, column: 1}
	}
	between := d.text[from.offset:within]
	d.scanned += len(between)
	reached := place{offset: within, line: from.line + strings.Count(between, "\n"), column: from.column}
	if lineStart := strings.LastIndexByte(between, '\n'); lineStart >= 0 {
		reached.column = 1
		between = between[lineStart+1:]
	}
	reached.column += utf8.RuneCountInString(between)
	d.counted = reached
	if start := characterStart(d.text, within); start < within {
		d.counted = place{offset: start, line: reached.line,
			column: reached.column - utf8.RuneCountInString(d.text[start:within])}
	}
	return &Error{Offset: offset, Line: reached.line, Column: reached.column, Err: cause}
}

// characterStart returns where the character of text that offset lies
// inside starts: the start of a valid UTF-8 sequence of several bytes that
// begins before offset and ends after it, or offset itself. A count of
// characters split there counts each byte before offset as one.
func characterStart(text string, offset int) int {
	for back := 1; back < utf8.UTFMax && back <= offset; back++ {
		if utf8.RuneStart(text[offset-back]) {
			if _, size := utf8.DecodeRuneInString(text[offset-back:]); size > back {
				return offset - back
			}
			return offset
		}
	}
	return offset
}

// Decode reads data, raw bytes such as a file's content, as one JSON
// document; whitespace may surround the value, anything else after it is an
// error. A leading byte-order mark is an error.
func Decode(data []byte) (Value, error) {
	return DecodeString(string(data))
}

// DecodeString reads text as one JSON document, as Decode does.
func DecodeString(text string) (Value, error) {
	return NewDocument(text).Decode()
}

// DecodePrefix reads the one value that starts at byte start of text,
// after any whitespace, and returns it with the byte offset just past it;
// what follows the value, such as another document, is left unread.
func DecodePrefix(text string, start int) (Value, int, error) {
	return NewDocument(text).DecodePrefix(start)
}

// DecodePrefix reads the one value that starts at byte start of the
// document, as the function DecodePrefix reads it from the document's text,
// noting the member names that repeat for TakeRepeats.
func (d *Document) DecodePrefix(start int) (Value, int, error) {
	value, end, found, err := d.decodeAt(start)
	if err != nil {
		return Value{}, 0, err
	}
	d.repeats.Merge(found)
	return value, end, nil
}

// decodeAt reads the one value that starts at byte start of the document
// and returns it with the offset just past it and the names it repeated.
func (d *Document) decodeAt(start int) (Value, int, Repeats, error) {
	if start < 0 || start > len(d.text) {
		return Value{}, 0, Repeats{}, d.errorAt(start, io.ErrUnexpectedEOF)
	}
	decoder := newDecoder(d.text[start:])
	var found Repeats
	value, err := readValue(decoder, d, start, &found)
	if err != nil {
		return Value{}, 0, Repeats{}, err
	}
	return value, start + int(decoder.InputOffset()), found, nil
}

// newDecoder returns a token reader over text that keeps each number's
// source text.
func newDecoder(text string) *json.Decoder {
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()
	return decoder
}

// frame is an array or object being read: its items, or its members and
// the key whose value comes next.
type frame struct {
	object Value
	items  []Value
	key    string
	hasKey bool
}

// readValue reads one value from decoder, which reads the document's text
// from byte start, building arrays and objects on a stack of its own rather than by
// recursion, and notes in found each member that repeats a name of its
// object. An array or object that opens below MaxDepth others is refused
// at its opening bracket.
func readValue(decoder *json.Decoder, document *Document, start int, found *Repeats) (Value, error) {
	var stack []*frame
	for {
		token, err := decoder.Token()
		if err != nil {
			return Value{}, readError(decoder, document, start, err)
		}
		var value Value
		switch typed := token.(type) {
		case json.Delim:
			if typed == '{' || typed == '[' {
				if len(stack) == MaxDepth {
					return Value{}, document.errorAt(start+int(decoder.InputOffset())-1, errTooDeep)
				}
				stack = append(stack, openFrame(typed))
				continue
			}
			top := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			value = top.close()
		case string:
			if top := topFrame(stack); top != nil && top.object.IsObject() && !top.hasKey {
				top.key, top.hasKey = typed, true
				continue
			}
			value = NewString(typed)
		default:
			value = scalar(typed)
		}
		top := topFrame(stack)
		if top == nil {
			return value, nil
		}
		top.add(value, found)
	}
}

// readError returns the error for a failed read: a syntax error at its own
// offset, and an end of text inside a value as an unexpected end.
func readError(decoder *json.Decoder, document *Document, start int, err error) error {
	text := document.text
	var syntax *json.SyntaxError
	switch {
	case errors.As(err, &syntax):
		// The reader's offset counts the character it stopped at.
		end := min(start+int(syntax.Offset), len(text))
		_, width := utf8.DecodeLastRuneInString(text[start:end])
		return document.errorAt(end-width, err)
	case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		return document.errorAt(len(text), io.ErrUnexpectedEOF)
	}
	return document.errorAt(start+int(decoder.InputOffset()), err)
}

// openFrame returns the frame of an array or object that delimiter opens.
func openFrame(delimiter json.Delim) *frame {
	if delimiter == '{' {
		return &frame{object: NewObject()}
	}
	return &frame{items: []Value{}}
}

// topFrame returns the innermost open frame, or nil.
func topFrame(stack []*frame) *frame {
	if len(stack) == 0 {
		return nil
	}
	return stack[len(stack)-1]
}

// add puts value into the frame: as the next item, or as the value of the
// key read before it, noting in found a key the object already holds.
func (f *frame) add(value Value, found *Repeats) {
	if f.object.IsObject() {
		if f.object.put(f.key, value) {
			found.note(f.key, 1)
		}
		f.key, f.hasKey = "", false
		return
	}
	f.items = append(f.items, value)
}

// close returns the value the frame read.
func (f *frame) close() Value {
	if f.object.IsObject() {
		return f.object
	}
	return Value{kind: Array, items: f.items}
}

// scalar returns the value of a token that is a number, a boolean or null.
func scalar(token json.Token) Value {
	switch typed := token.(type) {
	case json.Number:
		text := string(typed)
		return Value{kind: Number, flag: strings.ContainsAny(text, ".eE"), text: text}
	case bool:
		return NewBool(typed)
	}
	return Value{}
}
