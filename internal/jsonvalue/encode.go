package jsonvalue

import (
	"bytes"
	"encoding/json"
)

// MarshalJSON writes v as compact JSON for encoding/json: object keys in
// the value's order, numbers as written, strings through encoding/json's
// string encoder without HTML escaping. encoding/json checks the text a
// marshaller returns and refuses a document nested more than 10,000 levels
// in all, so a value at the decoder's full depth marshals alone but not
// inside another value; JSON, IndentedJSON and Text have no such limit.
func (v Value) MarshalJSON() ([]byte, error) {
	return newWriter("").appendValue(nil, v, 0), nil
}

// JSON returns v as compact JSON with no HTML escaping. It is written
// straight from the value, so it never fails: a value nests at most as
// deep as the decoder allows, plus the levels a caller wraps around it,
// and the writer recurses once per level.
func (v Value) JSON() string {
	return string(newWriter("").appendValue(nil, v, 0))
}

// IndentedJSON returns v as JSON indented by indent per level, as
// encoding/json's Indent lays it out, with no HTML escaping and no final
// newline. Like JSON it is written straight from the value.
func (v Value) IndentedJSON(indent string) string {
	return string(newWriter(indent).appendValue(nil, v, 0))
}

// writer writes values as JSON, quoting strings through one encoder and
// buffer reused for every string of a value, and starting a line indented
// by indent per level before each item when indent is not empty.
type writer struct {
	quoted  bytes.Buffer
	encoder *json.Encoder
	indent  string
}

// newWriter returns a writer with HTML escaping off.
func newWriter(indent string) *writer {
	result := &writer{indent: indent}
	result.encoder = json.NewEncoder(&result.quoted)
	result.encoder.SetEscapeHTML(false)
	return result
}

// appendString appends text as a JSON string to out. Encoding a string
// cannot fail (invalid UTF-8 becomes U+FFFD) and a bytes.Buffer never
// refuses a write, so the encoder's error is always nil.
func (w *writer) appendString(out []byte, text string) []byte {
	w.quoted.Reset()
	_ = w.encoder.Encode(text)
	return append(out, bytes.TrimSuffix(w.quoted.Bytes(), []byte("\n"))...)
}

// newline starts a line at a level when the writer indents.
func (w *writer) newline(out []byte, level int) []byte {
	if w.indent == "" {
		return out
	}
	out = append(out, '\n')
	for range level {
		out = append(out, w.indent...)
	}
	return out
}

// appendValue appends value, at a level of nesting, to out.
func (w *writer) appendValue(out []byte, value Value, level int) []byte {
	switch value.kind {
	case Bool:
		if value.flag {
			return append(out, "true"...)
		}
		return append(out, "false"...)
	case Number:
		return append(out, value.text...)
	case String:
		return w.appendString(out, value.text)
	case Array:
		return w.appendArray(out, value.items, level)
	case Object:
		return w.appendObject(out, value.members.list, level)
	}
	return append(out, "null"...)
}

// appendArray appends an array's items to out.
func (w *writer) appendArray(out []byte, items []Value, level int) []byte {
	if len(items) == 0 {
		return append(out, "[]"...)
	}
	out = append(out, '[')
	for i, item := range items {
		if i > 0 {
			out = append(out, ',')
		}
		out = w.appendValue(w.newline(out, level+1), item, level+1)
	}
	return append(w.newline(out, level), ']')
}

// appendObject appends an object's members in order to out.
func (w *writer) appendObject(out []byte, list []Member, level int) []byte {
	if len(list) == 0 {
		return append(out, "{}"...)
	}
	out = append(out, '{')
	for i, member := range list {
		if i > 0 {
			out = append(out, ',')
		}
		out = append(w.appendString(w.newline(out, level+1), member.Key), ':')
		if w.indent != "" {
			out = append(out, ' ')
		}
		out = w.appendValue(out, member.Value, level+1)
	}
	return append(w.newline(out, level), '}')
}
