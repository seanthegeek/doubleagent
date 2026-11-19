package analyze

import (
	"bytes"
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/secrets"
)

// fillValue sets every string reachable in value, which can be set, to
// text: each field encoding/json writes, one item of each list, each item
// of an array, one key and value of each map with string keys, and what
// each pointer points to.
func fillValue(value reflect.Value, text string) {
	switch value.Kind() {
	case reflect.String:
		value.SetString(text)
	case reflect.Pointer:
		value.Set(reflect.New(value.Type().Elem()))
		fillValue(value.Elem(), text)
	case reflect.Slice:
		value.Set(reflect.MakeSlice(value.Type(), 1, 1))
		fillValue(value.Index(0), text)
	case reflect.Array:
		for i := range value.Len() {
			fillValue(value.Index(i), text)
		}
	case reflect.Map:
		value.Set(reflect.MakeMap(value.Type()))
		key, item := reflect.New(value.Type().Key()).Elem(), reflect.New(value.Type().Elem()).Elem()
		fillValue(key, text)
		fillValue(item, text)
		value.SetMapIndex(key, item)
	case reflect.Struct:
		for i := range value.NumField() {
			field := value.Type().Field(i)
			if field.IsExported() || field.Anonymous {
				fillValue(value.Field(i), text)
			}
		}
	}
}

func TestEveryStringOfAJSONOutputGoesThroughTheReplacement(t *testing.T) {
	const secret = "fake-secret-value-0001"
	recorder := secrets.NewRecorder(secrets.Setting{Withhold: true, Placeholder: "P"})
	recorder.Value(secret)
	out := output{recorder}
	tests := []struct {
		name     string
		document any
	}{
		{"detect --json", &detectJSON{}},
		{"inventory.json", &inventoryJSON{}},
		{"a project object", &projectValueJSON{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			document := reflect.ValueOf(test.document).Elem()
			fillValue(document, secret)
			var written bytes.Buffer
			if err := writeJSON(&written, out.document(document.Interface())); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(written.String(), secret) || !strings.Contains(written.String(), `"P"`) {
				t.Errorf("the secret is written:\n%s", written.String())
			}
		})
	}
}

func TestAProjectObjectReplacesOnlyTheSecretsThatReachEveryString(t *testing.T) {
	recorder := secrets.NewRecorder(secrets.Setting{Withhold: true, Placeholder: "P"})
	recorder.Begin()
	recorder.Value("pw1")
	recorder.Value("fake-secret-value-0001")
	recorder.End()
	out := output{recorder}
	value := projectValueJSON{Agents: map[string]agentTallyJSON{}}
	entries := projectObjects{{path: "/srv/fake-secret-value-0001/a", value: value}, {path: "/srv/pw1-b", value: value},
		{path: "/srv/fake-secret-value-0001/a", value: value}}
	var written bytes.Buffer
	if err := writeJSON(&written, out.projects(entries)); err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"/srv/P/a\": {\n"
	if got := written.String(); !strings.HasPrefix(got, want) || strings.Count(got, `"/srv/P/a"`) != 2 ||
		!strings.Contains(got, `"/srv/pw1-b"`) {
		t.Errorf("got\n%s", got)
	}
}

// unreachable returns where in typ, a type an output is written from
// (named place), a string lies that the copy of output.document cannot
// reach: behind an
// embedded pointer of an unexported type, or inside a type that writes
// itself (json.Marshaler) other than allowed; seen holds the types walked.
func unreachable(typ reflect.Type, place string, allowed []reflect.Type, seen map[reflect.Type]bool) []string {
	if seen[typ] {
		return nil
	}
	seen[typ] = true
	marshaler := reflect.TypeFor[json.Marshaler]()
	if typ.Implements(marshaler) || reflect.PointerTo(typ).Implements(marshaler) {
		if slices.Contains(allowed, typ) {
			return nil
		}
		return []string{place + " (" + typ.String() + " writes itself)"}
	}
	var found []string
	switch typ.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Array:
		found = unreachable(typ.Elem(), place+"[]", allowed, seen)
	case reflect.Map:
		found = slices.Concat(unreachable(typ.Key(), place+"{key}", allowed, seen),
			unreachable(typ.Elem(), place+"{}", allowed, seen))
	case reflect.Interface:
		found = []string{place + " (an interface, whose value the type does not say)"}
	case reflect.Struct:
		for field := range typ.Fields() {
			switch {
			case field.Tag.Get("json") == "-" || !field.IsExported() && !field.Anonymous:
			case field.Anonymous && !field.IsExported() && field.Type.Kind() == reflect.Pointer:
				found = append(found, place+"."+field.Name+" (an embedded pointer of an unexported type)")
			default:
				found = append(found, unreachable(field.Type, place+"."+field.Name, allowed, seen)...)
			}
		}
	}
	return found
}

// projectObjectsType is the one type that writes itself in an output,
// whose paths and objects output.projects replaces.
var projectObjectsType = reflect.TypeFor[projectObjects]()

func TestEveryOutputTypeHoldsOnlyKindsTheCopyReaches(t *testing.T) {
	for _, typ := range []reflect.Type{reflect.TypeFor[detectJSON](), reflect.TypeFor[inventoryJSON](),
		reflect.TypeFor[projectValueJSON]()} {
		if found := unreachable(typ, typ.Name(), []reflect.Type{projectObjectsType}, map[reflect.Type]bool{}); found != nil {
			t.Errorf("the copy cannot reach %q", found)
		}
	}
}

// hidden is an unexported type embedded by pointer.
type hidden struct{ B string }

// selfWriting is a type that writes itself.
type selfWriting struct{ C string }

// MarshalJSON writes the type as an empty object.
func (selfWriting) MarshalJSON() ([]byte, error) { return []byte("{}"), nil }

func TestTheTypeCheckFindsEachKindTheCopyCannotReach(t *testing.T) {
	tests := []struct {
		name string
		typ  reflect.Type
	}{
		{"an embedded pointer of an unexported type", reflect.TypeFor[struct{ *hidden }]()},
		{"a type that writes itself", reflect.TypeFor[struct{ S selfWriting }]()},
		{"an interface", reflect.TypeFor[struct{ I any }]()},
		{"one inside an array", reflect.TypeFor[struct{ A [2]selfWriting }]()},
		{"one inside a map", reflect.TypeFor[struct{ M map[string]selfWriting }]()},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if found := unreachable(test.typ, "x", nil, map[reflect.Type]bool{}); len(found) != 1 {
				t.Errorf("found %q, want one place", found)
			}
		})
	}
}

func TestTheCopyReplacesEveryKindItReaches(t *testing.T) {
	const secret = "fake-secret-value-0001"
	type inner struct{ Deep string }
	type document struct {
		Array   [2]string
		Pointer *string
		List    []string
		Map     map[string]string
		inner
		Nested struct{ Field string } `json:"nested"`
	}
	recorder := secrets.NewRecorder(secrets.Setting{Withhold: true, Placeholder: "P"})
	recorder.Value(secret)
	var value document
	fillValue(reflect.ValueOf(&value).Elem(), secret)
	value.Deep = secret
	var written bytes.Buffer
	if err := writeJSON(&written, output{recorder}.document(value)); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(written.String(), secret) || strings.Count(written.String(), `"P"`) < 8 {
		t.Errorf("the copy wrote\n%s", written.String())
	}
	if value.Array[0] != secret {
		t.Error("the copy changed the document it copied")
	}
}
