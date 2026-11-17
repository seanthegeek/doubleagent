// Package timeutil normalises timestamps to the one form the timeline
// writes, YYYY-MM-DDTHH:MM:SS.mmmZ: UTC, milliseconds cut (never rounded),
// so rows from different agents sort together as plain strings. A value
// that does not read as a time gives "", never an error, so one bad record
// never stops a timeline. Years outside 1 to 9999 give "".
//
// Parsers call FromValue on a decoded JSON field, FromText on text,
// FromEpoch on a float and FromInt on an integer. A number, or text of
// ASCII digits with at most one point, is an epoch: milliseconds when it
// is above 1e11, else seconds. An epoch is read from its decimal digits,
// exactly, and cut to the millisecond. Other text is a date, or a date and
// time, in one of these forms (time.Parse layouts):
//
//	2006-01-02
//	2006-01-02T15:04    2006-01-02T15:04:05    2006-01-02T15:04:05.999999999
//	either time form followed by Z, -07:00, -0700 or -07
//
// The fraction may have any number of digits, after a point or a comma;
// T, a lower-case t or one space separates date and time; a lower-case z
// is Z; no zone is UTC. The hour has two digits and an offset's hours run
// to 23. Anything else gives "": week and ordinal dates, the basic form
// without separators, any other separator, a date with an offset but no
// time, 24:00, a second of 60, an offset with seconds, non-ASCII digits.
package timeutil

import (
	"time"
)

// Clock returns the current time; a command takes one so that tests can
// pin it. Clock(time.Now) is the system's.
type Clock func() time.Time

// layout is the timeline form in Go's reference-time notation; Format cuts
// the fraction to the digits the layout names.
const layout = "2006-01-02T15:04:05.000Z"

// FormatUTC returns moment as a timeline timestamp: in UTC, milliseconds
// cut, the year in four digits.
func FormatUTC(moment time.Time) string {
	return moment.UTC().Format(layout)
}

// FromTime returns moment as a timeline timestamp, or "" when it falls
// outside the years 1 to 9999.
func FromTime(moment time.Time) string {
	if year := moment.UTC().Year(); year < 1 || year > 9999 {
		return ""
	}
	return FormatUTC(moment)
}

// EpochMillis returns the milliseconds since the Unix epoch of a timeline
// timestamp and true; or 0 and false for "" and for text that is not
// exactly the form FormatUTC writes with a valid date and time in the
// years 1 to 9999.
func EpochMillis(value string) (int64, bool) {
	if len(value) != len(layout) {
		return 0, false
	}
	moment, err := time.Parse(layout, value)
	if err != nil || moment.Year() < 1 {
		return 0, false
	}
	return moment.UnixMilli(), true
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
