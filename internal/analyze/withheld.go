package analyze

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"reflect"

	"github.com/seanthegeek/doubleagent/internal/secrets"
)

// output is how a command writes the strings of its own output (the
// summary it prints, inventory.json, detect's report) under the run's
// secrets setting: each through the recorder's Text, and a problem line
// through its ProblemText.
type output struct {
	recorder *secrets.Recorder
}

// text returns text as the command's output writes it.
func (o output) text(text string) string {
	return o.recorder.Text(text)
}

// shown returns text as the command prints it for a person: withheld as
// text gives it, then shown.
func (o output) shown(text string) string {
	return shown(o.text(text))
}

// printError writes an error message as the command's output writes it
// (printError), so a value the analyst gave, a --match pattern or an
// --agent name holding a URL's credentials, is replaced like any other.
func (o output) printError(ctx context.Context, stderr io.Writer, message string) int {
	return printError(ctx, stderr, o.text(message))
}

// shownAll returns each text of texts as shown gives it.
func (o output) shownAll(texts []string) []string {
	out := make([]string, len(texts))
	for i, text := range texts {
		out[i] = o.shown(text)
	}
	return out
}

// document returns a copy of document, a value encoding/json writes, with
// every string in it (a field, an item of a list, a key or value of a map,
// behind a pointer) through text, so no string of a JSON output misses the
// replacement. A field encoding/json skips is copied as it is.
func (o output) document(document any) any {
	if o.recorder == nil {
		return document
	}
	return withheldValue(reflect.ValueOf(document), o.text).Interface()
}

// collect reads every string in document, a value encoding/json writes,
// for the credentials of the URLs in it (secrets.Recorder.Collect).
func (o output) collect(document any) {
	if o.recorder == nil {
		return
	}
	withheldValue(reflect.ValueOf(document), func(text string) string {
		o.recorder.Collect(text)
		return text
	})
}

// withheldValue returns a copy of value with every string through replace.
func withheldValue(value reflect.Value, replace func(string) string) reflect.Value {
	switch value.Kind() {
	case reflect.String:
		return reflect.ValueOf(replace(value.String())).Convert(value.Type())
	case reflect.Pointer, reflect.Interface:
		if value.IsNil() {
			return value
		}
		if value.Kind() == reflect.Interface {
			copied := reflect.New(value.Type()).Elem()
			copied.Set(withheldValue(value.Elem(), replace))
			return copied
		}
		copied := reflect.New(value.Type().Elem())
		copied.Elem().Set(withheldValue(value.Elem(), replace))
		return copied
	case reflect.Slice:
		if value.IsNil() {
			return value
		}
		copied := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		for i := range value.Len() {
			copied.Index(i).Set(withheldValue(value.Index(i), replace))
		}
		return copied
	case reflect.Map:
		return withheldMap(value, replace)
	case reflect.Struct:
		return withheldStruct(value, replace)
	}
	return value
}

// withheldMap returns a copy of a map with its keys and values through
// replace. Two keys that become one would lose an entry, so a map whose
// keys can hold evidence is written as an ordered object instead
// (projectObjects).
func withheldMap(value reflect.Value, replace func(string) string) reflect.Value {
	if value.IsNil() {
		return value
	}
	copied := reflect.MakeMapWithSize(value.Type(), value.Len())
	iterator := value.MapRange()
	for iterator.Next() {
		copied.SetMapIndex(withheldValue(iterator.Key(), replace), withheldValue(iterator.Value(), replace))
	}
	return copied
}

// withheldStruct returns a copy of a struct with each field encoding/json
// writes through replace; a field it does not write is copied as it is.
func withheldStruct(value reflect.Value, replace func(string) string) reflect.Value {
	copied := reflect.New(value.Type()).Elem()
	copied.Set(value)
	withholdFields(copied, replace)
	return copied
}

// withholdFields puts each field encoding/json writes of fields, a struct
// that can be set, through replace in place: an exported field, and the
// exported fields of an embedded struct whose type is unexported, whose
// fields encoding/json writes as the outer struct's.
func withholdFields(fields reflect.Value, replace func(string) string) {
	for i := range fields.NumField() {
		field := fields.Type().Field(i)
		switch {
		case field.Tag.Get("json") == "-":
		case field.IsExported():
			fields.Field(i).Set(withheldValue(fields.Field(i), replace))
		case field.Anonymous && field.Type.Kind() == reflect.Struct:
			withholdFields(fields.Field(i), replace)
		}
	}
}

// projectObjects is a JSON object of project objects by display path, in
// the order given: the byte order of the paths as the evidence spells them
// (projectInventory.entries), so withholding secrets, which changes the
// paths written, changes no key's place. Two paths that become one when
// their secrets are withheld are both written, under that one key, so no
// project's counts are lost.
type projectObjects []projectEntryJSON

// MarshalJSON writes the object, its keys and values as encoding/json
// writes them with HTML characters not escaped.
func (p projectObjects) MarshalJSON() ([]byte, error) {
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	out.WriteByte('{')
	for i, entry := range p {
		if i > 0 {
			out.WriteByte(',')
		}
		if err := encoder.Encode(entry.path); err != nil {
			return nil, err
		}
		out.WriteByte(':')
		if err := encoder.Encode(entry.value); err != nil {
			return nil, err
		}
	}
	out.WriteByte('}')
	return out.Bytes(), nil
}
