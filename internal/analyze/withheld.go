package analyze

import (
	"bytes"
	"encoding/json"
	"reflect"
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/secrets"
)

// output is how a command writes the strings of its own output (the
// summary it prints, inventory.json, detect's report) under the run's
// secrets setting: each through the recorder's Text, with the command's own
// record and the records of the parses the string comes from.
type output struct {
	recorder *secrets.Recorder
}

// text returns text as the command's output writes it, with the secrets of
// scopes (the parses it comes from) and the command's own replaced.
func (o output) text(text string, scopes ...*secrets.Found) string {
	if o.recorder == nil {
		return text
	}
	return o.recorder.Text(text, append(slices.Clone(scopes), o.recorder.Command())...)
}

// shown returns text as the command prints it for a person: withheld as
// text gives it, then shown.
func (o output) shown(text string, scopes ...*secrets.Found) string {
	return shown(o.text(text, scopes...))
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
// behind a pointer) through text with scopes, so no string of a JSON output
// misses the replacement. A field encoding/json skips is copied as it is.
func (o output) document(document any, scopes ...*secrets.Found) any {
	if o.recorder == nil {
		return document
	}
	replace := func(text string) string { return o.text(text, scopes...) }
	return withheldValue(reflect.ValueOf(document), replace).Interface()
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
// the byte order of the paths. Two paths that become one when their
// secrets are withheld are both written, under that one key, so no
// project's counts are lost.
type projectObjects []projectEntryJSON

// MarshalJSON writes the object, its keys and values as encoding/json
// writes them with HTML characters not escaped.
func (p projectObjects) MarshalJSON() ([]byte, error) {
	sorted := slices.Clone(p)
	slices.SortStableFunc(sorted, func(left, right projectEntryJSON) int {
		return strings.Compare(left.path, right.path)
	})
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	out.WriteByte('{')
	for i, entry := range sorted {
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
