package timeutil

import (
	"regexp"
	"time"
)

// isoShape is the shape of the date and time forms the package comment
// lists, checked before time.Parse, which would also take a one-digit hour
// and an offset of 24 hours.
var isoShape = regexp.MustCompile(`\A[0-9]{4}-[0-9]{2}-[0-9]{2}` +
	`(?:T[0-9]{2}:[0-9]{2}(?::[0-9]{2}(?:[.,][0-9]+)?)?` +
	`(?:Z|[+-](?:[01][0-9]|2[0-3])(?::?[0-5][0-9])?)?)?\z`)

// isoLayouts are the time.Parse layouts of the accepted forms; a seconds
// layout also reads a fraction after the seconds.
func isoLayouts() []string {
	return []string{"2006-01-02", "2006-01-02T15:04", "2006-01-02T15:04:05",
		"2006-01-02T15:04Z07:00", "2006-01-02T15:04Z0700", "2006-01-02T15:04Z07",
		"2006-01-02T15:04:05Z07:00", "2006-01-02T15:04:05Z0700", "2006-01-02T15:04:05Z07"}
}

// fromISO returns the timestamp of ASCII text in one of the date and time
// forms the package comment lists, or "".
func fromISO(text string) string {
	data := []byte(text)
	if len(data) > 10 && (data[10] == 't' || data[10] == ' ') {
		data[10] = 'T'
	}
	if data[len(data)-1] == 'z' {
		data[len(data)-1] = 'Z'
	}
	if !isoShape.Match(data) {
		return ""
	}
	for _, form := range isoLayouts() {
		moment, err := time.Parse(form, string(data))
		if err != nil {
			continue
		}
		if year := moment.UTC().Year(); year < 1 || year > 9999 {
			return ""
		}
		return FormatUTC(moment)
	}
	return ""
}
