package jsonvalue

import (
	"bytes"
	"encoding/json"
	"strings"
)

// MarshalJSON writes v as compact JSON for encoding/json: object keys in
// the value's order, numbers as written, strings through encoding/json's
// string encoder without HTML escaping.
func (v Value) MarshalJSON() ([]byte, error) {
	quoter := newQuoter()
	return quoter.appendValue(nil, v)
}

// JSON returns v as compact JSON with no HTML escaping.
func (v Value) JSON() string {
	return encode(v, "")
}

// IndentedJSON returns v as JSON indented by indent per level, with no
// HTML escaping and no final newline.
func (v Value) IndentedJSON(indent string) string {
	return encode(v, indent)
}

// encode writes value through an encoder with HTML escaping off, indented
// by indent when it is not empty. A Value always encodes, so an error, which
// would be a defect here, gives "".
func encode(value Value, indent string) string {
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", indent)
	if err := encoder.Encode(value); err != nil {
		return ""
	}
	return strings.TrimSuffix(out.String(), "\n")
}

// quoter writes strings as JSON through one encoder and buffer reused for
// every string of a value.
type quoter struct {
	out     bytes.Buffer
	encoder *json.Encoder
}

// newQuoter returns a quoter with HTML escaping off.
func newQuoter() *quoter {
	writer := &quoter{}
	writer.encoder = json.NewEncoder(&writer.out)
	writer.encoder.SetEscapeHTML(false)
	return writer
}

// appendString appends text as a JSON string to out.
func (q *quoter) appendString(out []byte, text string) ([]byte, error) {
	q.out.Reset()
	if err := q.encoder.Encode(text); err != nil {
		return nil, err
	}
	return append(out, bytes.TrimSuffix(q.out.Bytes(), []byte("\n"))...), nil
}

// appendValue appends value as compact JSON to out.
func (q *quoter) appendValue(out []byte, value Value) ([]byte, error) {
	switch value.kind {
	case Bool:
		if value.flag {
			return append(out, "true"...), nil
		}
		return append(out, "false"...), nil
	case Number:
		return append(out, value.text...), nil
	case String:
		return q.appendString(out, value.text)
	case Array:
		return q.appendArray(out, value.items)
	case Object:
		return q.appendObject(out, value.members.list)
	}
	return append(out, "null"...), nil
}

// appendArray appends an array's items as compact JSON to out.
func (q *quoter) appendArray(out []byte, items []Value) ([]byte, error) {
	out = append(out, '[')
	for i, item := range items {
		if i > 0 {
			out = append(out, ',')
		}
		var err error
		if out, err = q.appendValue(out, item); err != nil {
			return nil, err
		}
	}
	return append(out, ']'), nil
}

// appendObject appends an object's members in order as compact JSON to out.
func (q *quoter) appendObject(out []byte, list []Member) ([]byte, error) {
	out = append(out, '{')
	for i, member := range list {
		if i > 0 {
			out = append(out, ',')
		}
		var err error
		if out, err = q.appendString(out, member.Key); err != nil {
			return nil, err
		}
		out = append(out, ':')
		if out, err = q.appendValue(out, member.Value); err != nil {
			return nil, err
		}
	}
	return append(out, '}'), nil
}
