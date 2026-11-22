package tomldoc

import (
	"slices"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

// definitionKind is how the document first defined a key path.
type definitionKind uint8

// What a later statement may do to a path of each kind, from the Keys,
// Table, Inline Table and Array of Tables sections of TOML 1.1; all else,
// and `path = value` on a path already defined, is invalid.
//
//	kind of path     [path]       [[path]]     path.x = value     [path.x]
//	implicitTable    defines it   no           makes it dotted    yes
//	headerTable      no           no           no                 yes
//	dottedTable      no           no           yes                yes
//	tableArray       no           new table    no                 in its last table
//	valueDefinition  no           no           no                 no
const (
	// implicitTable is a table a longer header made, such as a by [a.b].
	implicitTable definitionKind = iota
	// headerTable is a table a [table] header defined.
	headerTable
	// dottedTable is a table dotted keys made, such as a by a.b = 1.
	dottedTable
	// tableArray is an array of tables; its children are those of its last
	// table.
	tableArray
	// valueDefinition is a key given any value, an inline table or an
	// array included.
	valueDefinition
)

// definition is one key path the document has defined, and the paths
// below it, by their last part.
type definition struct {
	kind     definitionKind
	children map[string]*definition
}

// add defines a child of the given kind and returns it.
func (d *definition) add(part string, kind definitionKind) *definition {
	if d.children == nil {
		d.children = map[string]*definition{}
	}
	child := &definition{kind: kind}
	d.children[part] = child
	return child
}

// definitions holds what the document has defined so far, from the root,
// and the tables key/value pairs go into: first the table the last header
// opened, then each inline table being read, a table of its own that
// nothing outside its braces can reach. visit, when set, is given the full
// path of every header and key, as the reader's key list names them.
type definitions struct {
	root   *definition
	scopes []scope
	visit  func(path []string)
}

// scope is a table key/value pairs go into, and its full path.
type scope struct {
	node *definition
	path []string
}

// newDefinitions starts with nothing defined, in the root table.
func newDefinitions() definitions {
	root := &definition{kind: headerTable}
	return definitions{root: root, scopes: []scope{{node: root}}}
}

// fullPath returns the full path of a key in the innermost scope.
func (d *definitions) fullPath(parts []string) []string {
	return append(slices.Clip(d.scopes[len(d.scopes)-1].path), parts...)
}

// defineHeader applies a [path] or [[path]] header as the table of the
// kinds of definition says, a part on the way to the last one taking the
// [path.x] column, and opens its table. It returns why the header is
// invalid, or "".
func (d *definitions) defineHeader(parts []string, array bool) string {
	if d.visit != nil {
		d.visit(parts)
	}
	node := d.root
	for i, part := range parts[:len(parts)-1] {
		next := node.children[part]
		switch {
		case next == nil:
			next = node.add(part, implicitTable)
		case next.kind == valueDefinition:
			return "key " + toml.Key(parts[:i+1]).String() +
				" already holds a value, so no table can be defined in it"
		}
		node = next
	}
	last := node.children[parts[len(parts)-1]]
	switch {
	case array && last == nil:
		last = node.add(parts[len(parts)-1], tableArray)
	case array && last.kind == tableArray:
		last.children = nil
	case array:
		return "key " + toml.Key(parts).String() + " is already defined, and not as an array of tables"
	case last == nil:
		last = node.add(parts[len(parts)-1], headerTable)
	case last.kind == implicitTable:
		last.kind = headerTable
	default:
		return "table " + toml.Key(parts).String() + " is already defined"
	}
	d.scopes[0] = scope{node: last, path: parts}
	return ""
}

// defineKey applies a key/value line, or a key/value pair of an inline
// table, to the innermost scope, as the table of the kinds of definition
// says: each part before the last takes the path.x = value column, and
// the last part must be new. It returns why the key is invalid, or "".
func (d *definitions) defineKey(parts []string) string {
	if d.visit != nil {
		d.visit(d.fullPath(parts))
	}
	node := d.scopes[len(d.scopes)-1].node
	for i, part := range parts[:len(parts)-1] {
		next := node.children[part]
		switch {
		case next == nil:
			next = node.add(part, dottedTable)
		case next.kind == implicitTable:
			next.kind = dottedTable
		case next.kind != dottedTable:
			return "dotted key " + toml.Key(d.fullPath(parts)).String() + " adds to " +
				toml.Key(d.fullPath(parts[:i+1])).String() + ", which is defined as " + next.kind.words()
		}
		node = next
	}
	if node.children[parts[len(parts)-1]] != nil {
		return "key " + toml.Key(d.fullPath(parts)).String() + " is already defined"
	}
	node.add(parts[len(parts)-1], valueDefinition)
	return ""
}

// words names a kind of definition in a message.
func (k definitionKind) words() string {
	switch k {
	case headerTable:
		return "a [table]"
	case tableArray:
		return "an array of tables"
	}
	return "a value"
}

// checkOffset returns why a date-time's offset is out of range, or "": as
// RFC 3339 section 5.6 gives it, time-numoffset = ("+" / "-") time-hour
// ":" time-minute, with an hour of 00 to 23 and a minute of 00 to 59. The
// text is a value as written, which the reader has read as a date-time
// when it is one; the reader moves an offset minute of 60 into the hour.
func checkOffset(text string) string {
	if len(text) < 16 || !strings.ContainsRune("Tt ", rune(text[10])) {
		return ""
	}
	offset := text[len(text)-6:]
	if offset[0] != '+' && offset[0] != '-' || offset[3] != ':' {
		return ""
	}
	hour, hourErr := strconv.Atoi(offset[1:3])
	minute, minuteErr := strconv.Atoi(offset[4:6])
	if hourErr != nil || minuteErr != nil || hour <= 23 && minute <= 59 {
		return ""
	}
	return "the offset " + offset + " of the date-time " + text +
		" is not an hour of 00 to 23 and a minute of 00 to 59"
}

// decodeBasicKey returns the text of a quoted key in a basic string, its
// escapes replaced as the String section of the specification gives them.
// The reader has checked every escape, so one it would refuse is kept as
// written.
func decodeBasicKey(body string) string {
	if !strings.Contains(body, `\`) {
		return body
	}
	var text strings.Builder
	for i := 0; i < len(body); i++ {
		if body[i] != '\\' || i+1 == len(body) {
			text.WriteByte(body[i])
			continue
		}
		i++
		if digits := hexDigitsAfter(body[i]); digits > 0 && i+digits < len(body) {
			code, err := strconv.ParseUint(body[i+1:i+1+digits], 16, 32)
			if err == nil {
				text.WriteRune(rune(code))
				i += digits
				continue
			}
		} else if replacement, ok := shortEscape(body[i]); ok {
			text.WriteByte(replacement)
			continue
		}
		text.WriteString(body[i-1 : i+1])
	}
	return text.String()
}

// hexDigitsAfter returns how many hexadecimal digits follow the escape
// letter x, u or U, the escapes of a code point, and 0 for any other.
func hexDigitsAfter(letter byte) int {
	switch letter {
	case 'x':
		return 2
	case 'u':
		return 4
	case 'U':
		return 8
	}
	return 0
}

// shortEscape returns the character a one-letter escape stands for, and
// false for a letter that is not one.
func shortEscape(letter byte) (byte, bool) {
	switch letter {
	case 'b':
		return '\b', true
	case 't':
		return '\t', true
	case 'n':
		return '\n', true
	case 'f':
		return '\f', true
	case 'r':
		return '\r', true
	case 'e':
		return 0x1b, true
	case '"', '\\':
		return letter, true
	}
	return 0, false
}
