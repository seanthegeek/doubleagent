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
// timestamp and true; or 0 and false for any other text. The rule is a
// round trip: value is accepted only when FormatUTC, given the time read
// from it, writes value back byte for byte, so the accepted texts are
// exactly those FormatUTC writes for the years 1 to 9999. time.Parse alone
// is wider (a comma or a sign before the fraction digits).
func EpochMillis(value string) (int64, bool) {
	moment, err := time.Parse(layout, value)
	if err != nil || moment.Year() < 1 || FormatUTC(moment) != value {
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
