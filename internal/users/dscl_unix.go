//go:build unix

package users

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"io"
)

// The two record attributes dsclHomes asks dscl for, as its property list
// form names them.
const (
	dsclHomeKey = "dsAttrTypeStandard:NFSHomeDirectory"
	dsclNameKey = "dsAttrTypeStandard:RecordName"
)

// isRecordPlace reports whether open, the elements a dictionary starts
// inside, is the top-level array of the property list.
func isRecordPlace(open []string) bool {
	return len(open) == 2 && open[0] == "plist" && open[1] == "array"
}

// plistNode is one element of a property list with its text and the
// elements inside it, in order.
type plistNode struct {
	XMLName xml.Name
	Text    string      `xml:",chardata"`
	Nodes   []plistNode `xml:",any"`
}

// plistStrings returns the strings of value when it is an array, in order,
// passing over any other element inside it.
func plistStrings(value plistNode) []string {
	if value.XMLName.Local != "array" {
		return nil
	}
	var texts []string
	for _, node := range value.Nodes {
		if node.XMLName.Local == "string" {
			texts = append(texts, node.Text)
		}
	}
	return texts
}

// recordHomes returns the homes of one record dictionary, whose children
// alternate a key and its value.
func recordHomes(record plistNode) []Home {
	var names, paths []string
	for i := 0; i+1 < len(record.Nodes); i += 2 {
		key, value := record.Nodes[i], record.Nodes[i+1]
		if key.XMLName.Local != "key" {
			return nil
		}
		switch key.Text {
		case dsclNameKey:
			names = plistStrings(value)
		case dsclHomeKey:
			paths = plistStrings(value)
		}
	}
	user := ""
	if len(names) > 0 {
		user = names[0]
	}
	var homes []Home
	for _, path := range paths {
		if path != "" {
			homes = append(homes, Home{user, withoutEmptySegments(path)})
		}
	}
	return homes
}

// dsclRecordHomes reads the property list dscl prints, a top-level array
// of one dictionary per record, each attribute a key followed by an array
// of strings, and returns one home for each NFSHomeDirectory string that is
// not empty, under the record's first RecordName ("" when it has none), the
// home without its empty segments (withoutEmptySegments) as a passwd home
// is. Other keys and their values, and any element that is not a record,
// are passed over; a record whose children do not alternate a key and a
// value gives none. A text that is not well-formed XML ends the reading:
// the homes of the records read before it are returned with the error.
func dsclRecordHomes(r io.Reader) ([]Home, error) {
	decoder := xml.NewDecoder(r)
	var homes []Home
	var open []string
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			return homes, nil
		}
		if err != nil {
			return homes, err
		}
		switch element := token.(type) {
		case xml.StartElement:
			if element.Name.Local != "dict" || !isRecordPlace(open) {
				open = append(open, element.Name.Local)
				continue
			}
			var record plistNode
			if err := decoder.DecodeElement(&record, &element); err != nil {
				return homes, err
			}
			homes = append(homes, recordHomes(record)...)
		case xml.EndElement:
			open = open[:max(len(open)-1, 0)]
		}
	}
}

// dsclHomes returns the homes of `dscl -plist . -readall /Users
// NFSHomeDirectory RecordName` (dsclRecordHomes). The property list form is
// asked for because the plain listing pads a name to a column and puts one
// space between two values of a record, so a home holding a space cannot be
// told from two homes there.
func dsclHomes(ctx context.Context, run Runner) []Home {
	if run == nil || ctx.Err() != nil {
		return nil
	}
	output, err := run(ctx, "dscl", "-plist", ".", "-readall", "/Users",
		"NFSHomeDirectory", "RecordName")
	if err != nil {
		return nil
	}
	homes, _ := dsclRecordHomes(bytes.NewReader(output))
	return homes
}
