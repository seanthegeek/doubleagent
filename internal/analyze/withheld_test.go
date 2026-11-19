package analyze

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/secrets"
)

// fillValue sets every string reachable in value, which can be set, to
// text: each field encoding/json writes, one item of each list, one key and
// value of each map with string keys, and what each pointer points to.
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
	recorder := secrets.NewRecorder(secrets.Setting{Withhold: true, Placeholder: "P"})
	recorder.Value("s3c")
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
			fillValue(document, "s3c")
			var written bytes.Buffer
			if err := writeJSON(&written, out.document(document.Interface())); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(written.String(), "s3c") || !strings.Contains(written.String(), `"P"`) {
				t.Errorf("the secret is written:\n%s", written.String())
			}
		})
	}
}

func TestEachProjectObjectIsWithheldWithTheRecordsOfItsParses(t *testing.T) {
	recorder := secrets.NewRecorder(secrets.Setting{Withhold: true, Placeholder: "P"})
	found := recorder.Begin()
	recorder.Value("pw1")
	recorder.End()
	out := output{recorder}
	value := projectValueJSON{Agents: map[string]agentTallyJSON{}}
	entries := projectObjects{{path: "/srv/pw1-a", value: value, secrets: []*secrets.Found{found}},
		{path: "/srv/pw1-b", value: value}, {path: "/srv/pw1-a", value: value, secrets: []*secrets.Found{found}}}
	var written bytes.Buffer
	if err := writeJSON(&written, out.projects(entries)); err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"/srv/P-a\": {\n"
	if got := written.String(); !strings.HasPrefix(got, want) || strings.Count(got, `"/srv/P-a"`) != 2 ||
		!strings.Contains(got, `"/srv/pw1-b"`) {
		t.Errorf("got\n%s", got)
	}
}
