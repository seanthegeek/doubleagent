// Package timeutil normalises timestamps to the one form the analyzer
// writes, YYYY-MM-DDTHH:MM:SS.mmmZ: UTC, milliseconds truncated, so rows
// from different agents sort together as plain strings. A value that does
// not read as a time gives "", never an error, so one bad record never
// stops a timeline.
//
// Parsers call FromValue on a decoded JSON field, FromText on text,
// FromEpoch on a float and FromInt on an integer. A number, or text of
// ASCII digits with at most one point, is seconds since the Unix epoch, or
// milliseconds when above 1e11. Other text is read as an ISO 8601 date and
// time in the forms agents write:
//
//	YYYY-MM-DD
//	YYYY-MM-DDTHH:MM   YYYY-MM-DD HH:MM:SS   YYYY-MM-DDTHH:MM:SS.fffffffff
//	any of those with a time, then Z, z, +HH:MM, +HHMM or +HH (none is UTC)
//
// Any single character other than a digit may stand between date and time.
// Week dates, ordinal dates, the basic form without separators, an hour
// alone, 24:00, a comma before the fraction, offsets with seconds or with
// minutes above 59, a space before the offset and non-ASCII digits give "".
package timeutil

import (
	"time"
)

// Clock returns the current time; a command takes one so that tests can
// pin it. Clock(time.Now) is the system's.
type Clock func() time.Time

// layout is the timeline form in Go's reference-time notation; Go writes a
// fraction by truncating it, as the analyzer does.
const layout = "2006-01-02T15:04:05.000Z"

// FormatUTC returns moment as a timeline timestamp: in UTC, milliseconds
// truncated, the year in four digits (years 1 to 9999).
func FormatUTC(moment time.Time) string {
	return moment.UTC().Format(layout)
}

// EpochMillis returns the milliseconds since the Unix epoch of a timeline
// timestamp, floored, and true; or 0 and false for "" and for text that is
// not exactly the form FormatUTC writes with a valid date and time.
func EpochMillis(value string) (int64, bool) {
	if len(value) != len(layout) || value[10] != 'T' || value[23] != 'Z' {
		return 0, false
	}
	fields, ok := readFields(value, []fieldSpec{{0, 4, '-'}, {5, 2, '-'}, {8, 2, 'T'},
		{11, 2, ':'}, {14, 2, ':'}, {17, 2, '.'}, {20, 3, 'Z'}})
	if !ok {
		return 0, false
	}
	moment, ok := civilTime(fields[0], fields[1], fields[2], fields[3], fields[4], fields[5])
	if !ok {
		return 0, false
	}
	return moment.UnixMilli() + int64(fields[6]), true
}

// FirstNonEmpty returns the first of values that is not "", or "".
func FirstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// fieldSpec is a run of digits at offset start, width digits long,
// followed by the byte after unless the run ends the text; an after of 0
// lets any byte follow.
type fieldSpec struct {
	start, width int
	after        byte
}

// readFields reads the digit runs specs name from text and checks the
// byte after each run that lies inside text.
func readFields(text string, specs []fieldSpec) ([]int, bool) {
	values := make([]int, len(specs))
	for i, spec := range specs {
		value, ok := digits(text, spec.start, spec.width)
		end := spec.start + spec.width
		if !ok || (spec.after != 0 && end < len(text) && text[end] != spec.after) {
			return nil, false
		}
		values[i] = value
	}
	return values, true
}

// digits returns the number the width ASCII digits at offset start of text
// spell, and false when text is too short or holds another byte there.
func digits(text string, start, width int) (int, bool) {
	if start < 0 || start+width > len(text) {
		return 0, false
	}
	value := 0
	for _, char := range []byte(text[start : start+width]) {
		if char < '0' || char > '9' {
			return 0, false
		}
		value = value*10 + int(char-'0')
	}
	return value, true
}

// civilTime returns the UTC time of a calendar date and time of day, and
// false when a field is out of its range (a year outside 1 to 9999, a day
// past the end of its month, a second of 60).
func civilTime(year, month, day, hour, minute, second int) (time.Time, bool) {
	if year < 1 || year > 9999 || month < 1 || month > 12 || day < 1 ||
		hour > 23 || minute > 59 || second > 59 {
		return time.Time{}, false
	}
	moment := time.Date(year, time.Month(month), day, hour, minute, second, 0, time.UTC)
	if moment.Day() != day {
		return time.Time{}, false
	}
	return moment, true
}
