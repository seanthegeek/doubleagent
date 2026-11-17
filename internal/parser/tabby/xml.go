package tabby

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"strconv"
	"strings"
)

// element is one element of an XML document: its tag (`{namespace}name`
// for a name in a namespace), its attributes by name, and its children in
// document order.
type element struct {
	tag        string
	attributes map[string]string
	children   []*element
}

// attribute returns the value of the attribute called name.
func (e *element) attribute(name string) (string, bool) {
	value, ok := e.attributes[name]
	return value, ok
}

// iter returns the element and every element below it, in document order,
// that have the tag; "" takes every element.
func (e *element) iter(tag string) []*element {
	var found []*element
	pending := []*element{e}
	for len(pending) > 0 {
		next := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if tag == "" || next.tag == tag {
			found = append(found, next)
		}
		for i := len(next.children) - 1; i >= 0; i-- {
			pending = append(pending, next.children[i])
		}
	}
	return found
}

// qualifiedName returns a name as an element tree writes it.
func qualifiedName(name xml.Name) string {
	if name.Space == "" {
		return name.Local
	}
	return "{" + name.Space + "}" + name.Local
}

// parseXML reads a well-formed XML document of one root element into its
// tree. Text outside the root other than white space, a second root, no
// root at all or an attribute given twice is an error, worded "<what>: line <n>, column <n>".
func parseXML(data []byte) (*element, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var root *element
	var open []*element
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, xmlError(decoder, err.Error())
		}
		switch token := token.(type) {
		case xml.StartElement:
			if root != nil && len(open) == 0 {
				return nil, xmlError(decoder, "junk after document element")
			}
			node := &element{tag: qualifiedName(token.Name), attributes: map[string]string{}}
			for _, attribute := range token.Attr {
				name := qualifiedName(attribute.Name)
				if _, seen := node.attributes[name]; seen {
					return nil, xmlError(decoder, "duplicate attribute")
				}
				if attribute.Name.Space != "xmlns" && attribute.Name.Local != "xmlns" {
					node.attributes[name] = attribute.Value
				}
			}
			if len(open) == 0 {
				root = node
			} else {
				parent := open[len(open)-1]
				parent.children = append(parent.children, node)
			}
			open = append(open, node)
		case xml.EndElement:
			open = open[:len(open)-1]
		case xml.CharData:
			if len(open) == 0 && len(bytes.Trim(token, " \t\r\n")) > 0 {
				return nil, xmlError(decoder, "text outside the document element")
			}
		}
	}
	if root == nil {
		return nil, xmlError(decoder, "no element found")
	}
	return root, nil
}

// xmlError words a reading failure with the line and column the decoder
// stopped at. The decoder's own message loses its line prefix, quotes and
// brackets, so the whole stays one plain phrase.
func xmlError(decoder *xml.Decoder, message string) error {
	if _, rest, found := strings.Cut(message, "XML syntax error on line "); found {
		if _, detail, ok := strings.Cut(rest, ": "); ok {
			message = detail
		}
	}
	message = strings.Map(func(char rune) rune {
		if strings.ContainsRune(`()"`, char) {
			return '\''
		}
		return char
	}, message)
	line, column := decoder.InputPos()
	return errors.New(message + ": line " + strconv.Itoa(line) + ", column " + strconv.Itoa(column))
}
