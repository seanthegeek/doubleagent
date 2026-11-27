package jsonvalue

import (
	"bytes"
	"encoding/json"
)

// JSON returns v as compact JSON with no HTML escaping. It is written
// straight from the value, so it never fails: a value nests at most as
// deep as the decoder allows, plus the levels a caller wraps around it,
// and the writer recurses once per level.
func (v Value) JSON() string {
	return string(newWriter().appendValue(nil, v))
}

// writer writes values as JSON, quoting strings through one encoder and
// buffer reused for every string of a value.
type writer struct {
	quoted  bytes.Buffer
	encoder *json.Encoder
}

// newWriter returns a writer with HTML escaping off.
func newWriter() *writer {
	result := &writer{}
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

// appendValue appends value to out.
func (w *writer) appendValue(out []byte, value Value) []byte {
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
		return w.appendArray(out, value.items)
	case Object:
		return w.appendObject(out, value.members.list)
	}
	return append(out, "null"...)
}

// appendArray appends an array's items to out.
func (w *writer) appendArray(out []byte, items []Value) []byte {
	out = append(out, '[')
	for i, item := range items {
		if i > 0 {
			out = append(out, ',')
		}
		out = w.appendValue(out, item)
	}
	return append(out, ']')
}

// appendObject appends an object's members in order to out.
func (w *writer) appendObject(out []byte, list []Member) []byte {
	out = append(out, '{')
	for i, member := range list {
		if i > 0 {
			out = append(out, ',')
		}
		out = append(w.appendString(out, member.Key), ':')
		out = w.appendValue(out, member.Value)
	}
	return append(out, '}')
}
