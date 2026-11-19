package timeline

import (
	"reflect"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/secrets"
)

// fillStrings sets every string of value, a pointer to a struct, to text:
// each string field and each item of a list of strings.
func fillStrings(value any, text string) {
	fields := reflect.ValueOf(value).Elem()
	for i := range fields.NumField() {
		field := fields.Field(i)
		switch {
		case field.Kind() == reflect.String:
			field.SetString(text)
		case field.Kind() == reflect.Slice && field.Type().Elem().Kind() == reflect.String:
			field.Set(reflect.ValueOf([]string{text, text}))
		}
	}
}

// unreplaced returns the names of the written fields of value, a struct,
// that hold a string other than want.
func unreplaced(value any, want string) []string {
	fields := reflect.ValueOf(value)
	var names []string
	for i := range fields.NumField() {
		field, name := fields.Field(i), fields.Type().Field(i).Tag.Get("json")
		switch {
		case field.Kind() == reflect.String && field.String() != want:
			names = append(names, name)
		case field.Kind() == reflect.Slice && field.Type().Elem().Kind() == reflect.String:
			for _, item := range field.Interface().([]string) {
				if item != want {
					names = append(names, name)
				}
			}
		}
	}
	return names
}

func TestEveryStringOfARowAndASessionGoesThroughTheReplacement(t *testing.T) {
	recorder := secrets.NewRecorder(secrets.Setting{Withhold: true, Placeholder: "P"})
	found := recorder.Begin()
	recorder.Value("s3c")
	recorder.End()
	t.Run("a row", func(t *testing.T) {
		row := Row{Secrets: found}
		fillStrings(&row, "s3c")
		if names := unreplaced(row.Withheld(recorder), "P"); names != nil {
			t.Errorf("these fields keep the secret: %q", names)
		}
		if names := unreplaced(row.Withheld(nil), "s3c"); names != nil {
			t.Errorf("without a recorder these fields changed: %q", names)
		}
	})
	t.Run("a session", func(t *testing.T) {
		session := SessionSummary{Secrets: []*secrets.Found{found}}
		fillStrings(&session, "s3c")
		if names := unreplaced(session.Withheld(recorder), "P"); names != nil {
			t.Errorf("these fields keep the secret: %q", names)
		}
	})
	t.Run("a row of another parse keeps a short secret", func(t *testing.T) {
		row := Row{Secrets: &secrets.Found{}}
		fillStrings(&row, "s3c")
		if names := unreplaced(row.Withheld(recorder), "s3c"); names != nil {
			t.Errorf("these fields changed: %q", names)
		}
	})
}

func TestASessionCarriesTheRecordsOfItsRowsOnce(t *testing.T) {
	first, second := &secrets.Found{}, &secrets.Found{}
	rows := []Row{{SessionID: "s", Secrets: first}, {SessionID: "s", Secrets: second}, {SessionID: "s", Secrets: first},
		{SessionID: "s"}}
	summaries := Summarise(rows, MarkAgents{})
	if len(summaries) != 1 || !reflect.DeepEqual(summaries[0].Secrets, []*secrets.Found{first, second}) {
		t.Errorf("summaries %+v", summaries)
	}
}
