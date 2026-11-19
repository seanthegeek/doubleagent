package timeline

import (
	"reflect"
	"slices"
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

// withoutText returns the names of the fields unreplaced finds other than
// the row's text.
func withoutText(names []string) []string {
	var others []string
	for _, name := range names {
		if name != "text" {
			others = append(others, name)
		}
	}
	return others
}

func TestEveryStringOfARowAndASessionGoesThroughTheReplacement(t *testing.T) {
	const long, short = "fake-secret-value-0001", "s3c"
	recorder := secrets.NewRecorder(secrets.Setting{Withhold: true, Placeholder: "P"})
	found := recorder.Begin()
	recorder.Value(long)
	recorder.Value(short)
	recorder.End()
	t.Run("a long secret in every field of a row", func(t *testing.T) {
		row := Row{Secrets: found}
		fillStrings(&row, long)
		if names := unreplaced(row.Withheld(recorder), "P"); names != nil {
			t.Errorf("these fields keep the secret: %q", names)
		}
		if names := unreplaced(row.Withheld(nil), long); names != nil {
			t.Errorf("without a recorder these fields changed: %q", names)
		}
	})
	t.Run("a long secret in every field of a session", func(t *testing.T) {
		var session SessionSummary
		fillStrings(&session, long)
		if names := unreplaced(session.Withheld(recorder), "P"); names != nil {
			t.Errorf("these fields keep the secret: %q", names)
		}
	})
	t.Run("a short secret only in the text of a row of its parse", func(t *testing.T) {
		row := Row{Secrets: found}
		fillStrings(&row, short)
		withheld := row.Withheld(recorder)
		if names := withoutText(unreplaced(withheld, short)); names != nil || withheld.Text != "P" {
			t.Errorf("these fields changed: %q; the text is %q", names, withheld.Text)
		}
	})
	t.Run("a short secret in no field of a session", func(t *testing.T) {
		var session SessionSummary
		fillStrings(&session, short)
		if names := unreplaced(session.Withheld(recorder), short); names != nil {
			t.Errorf("these fields changed: %q", names)
		}
	})
	t.Run("a row of another parse keeps a short secret", func(t *testing.T) {
		row := Row{Secrets: &secrets.Found{}}
		fillStrings(&row, short)
		if names := unreplaced(row.Withheld(recorder), short); names != nil {
			t.Errorf("these fields changed: %q", names)
		}
	})
}

func TestStringsListsEveryStringFieldOfARow(t *testing.T) {
	var row Row
	fields := reflect.ValueOf(&row).Elem()
	var want []string
	for field, value := range fields.Fields() {
		if value.Kind() == reflect.String {
			value.SetString(field.Name)
			if field.Name != "Text" {
				want = append(want, field.Name)
			}
		}
	}
	// Each field holds its own name, so a field listed twice in place of
	// another shows; the text comes last.
	if want = append(want, "Text"); !slices.Equal(row.Strings(), want) {
		t.Errorf("Strings gives %q, want %q", row.Strings(), want)
	}
}
