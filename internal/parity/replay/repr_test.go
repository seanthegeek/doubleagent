//go:build parity

package replay

import (
	"encoding/json"
	"strconv"
	"strings"
	"unicode/utf8"
)

// reprDepth is the deepest nesting a repr span is read to.
const reprDepth = 64

// reprReader reads a Python list or dict repr: lists, dicts, quoted
// strings with Python's escapes, numbers, True, False and None.
type reprReader struct {
	text   string
	offset int
	depth  int
}

// reprSpan reads the list or dict repr text starts with, and returns its
// value as decoded JSON would hold it, and its length.
func reprSpan(text string) (any, int, bool) {
	reader := &reprReader{text: text}
	value, ok := reader.value()
	if !ok {
		return nil, 0, false
	}
	if _, container := value.([]any); !container {
		if _, container = value.(map[string]any); !container {
			return nil, 0, false
		}
	}
	return value, reader.offset, true
}

// skipSpace moves past spaces.
func (r *reprReader) skipSpace() {
	for r.offset < len(r.text) && r.text[r.offset] == ' ' {
		r.offset++
	}
}

// value reads one value.
func (r *reprReader) value() (any, bool) {
	r.skipSpace()
	if r.offset >= len(r.text) || r.depth > reprDepth {
		return nil, false
	}
	switch r.text[r.offset] {
	case '[':
		return r.list()
	case '{':
		return r.dict()
	case '\'', '"':
		return r.quoted()
	}
	return r.word()
}

// list reads a list after its opening bracket.
func (r *reprReader) list() (any, bool) {
	r.offset++
	r.depth++
	defer func() { r.depth-- }()
	items := []any{}
	for {
		if r.closes(']') {
			return items, true
		}
		item, ok := r.value()
		if !ok || !r.separated(']') {
			return nil, false
		}
		items = append(items, item)
	}
}

// dict reads a dict after its opening brace; a key that is not a string
// becomes its JSON text.
func (r *reprReader) dict() (any, bool) {
	r.offset++
	r.depth++
	defer func() { r.depth-- }()
	members := map[string]any{}
	for {
		if r.closes('}') {
			return members, true
		}
		key, ok := r.value()
		r.skipSpace()
		if !ok || r.offset >= len(r.text) || r.text[r.offset] != ':' {
			return nil, false
		}
		r.offset++
		item, ok := r.value()
		if !ok || !r.separated('}') {
			return nil, false
		}
		name, isText := key.(string)
		if !isText {
			name = encodeJSON(key, "")
		}
		members[name] = item
	}
}

// closes reports whether the container ends here, and moves past its end.
func (r *reprReader) closes(end byte) bool {
	r.skipSpace()
	if r.offset < len(r.text) && r.text[r.offset] == end {
		r.offset++
		return true
	}
	return false
}

// separated moves past the comma after an item, and reports whether a
// comma or the container's end follows it.
func (r *reprReader) separated(end byte) bool {
	r.skipSpace()
	switch {
	case r.offset < len(r.text) && r.text[r.offset] == ',':
		r.offset++
		return true
	case r.offset < len(r.text) && r.text[r.offset] == end:
		return true
	}
	return false
}

// word reads True, False, None or a number.
func (r *reprReader) word() (any, bool) {
	end := r.offset
	for end < len(r.text) && strings.IndexByte("+-.0123456789eEaFlNnorsTu", r.text[end]) >= 0 {
		end++
	}
	word := r.text[r.offset:end]
	r.offset = end
	switch {
	case word == "True" || word == "False":
		return word == "True", true
	case word == "None":
		return nil, true
	case numberText.MatchString(word):
		return json.Number(word), true
	}
	return nil, false
}

// quoted reads a quoted string with Python's escapes.
func (r *reprReader) quoted() (any, bool) {
	quote := r.text[r.offset]
	var text strings.Builder
	for r.offset++; r.offset < len(r.text); {
		character := r.text[r.offset]
		switch {
		case character == quote:
			r.offset++
			return text.String(), true
		case character == '\\':
			if !r.escape(&text) {
				return nil, false
			}
		default:
			text.WriteByte(character)
			r.offset++
		}
	}
	return nil, false
}

// reprEscapes are the one-letter escapes of Python's repr.
var reprEscapes = map[byte]string{'\\': "\\", '\'': "'", '"': "\"", 'n': "\n", 'r': "\r", 't': "\t"}

// escape reads one escape at the reader's backslash.
func (r *reprReader) escape(text *strings.Builder) bool {
	if r.offset+1 >= len(r.text) {
		return false
	}
	letter := r.text[r.offset+1]
	if plain, ok := reprEscapes[letter]; ok {
		text.WriteString(plain)
		r.offset += 2
		return true
	}
	width := map[byte]int{'x': 2, 'u': 4, 'U': 8}[letter]
	if width == 0 || r.offset+2+width > len(r.text) {
		return false
	}
	code, err := strconv.ParseUint(r.text[r.offset+2:r.offset+2+width], 16, 32)
	if err != nil {
		return false
	}
	if !utf8.ValidRune(rune(code)) {
		code = utf8.RuneError
	}
	text.WriteRune(rune(code))
	r.offset += 2 + width
	return true
}
