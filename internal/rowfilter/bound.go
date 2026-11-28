package rowfilter

import (
	"strconv"
	"strings"
	"time"

	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// FormsHelp lists the accepted WHEN forms; every BoundError ends with it,
// and the timeline command's help for --until quotes it.
const FormsHelp = "accepted: ISO 8601 such as 2026-10-01, 2026-10-01T09:30[:SS][Z|+HH:MM] or " +
	"2026-10-01 09:30[:SS] [+HH:MM|UTC]; 2026-10 or 2026; epoch seconds or milliseconds; " +
	"today, yesterday, now; or a relative time such as 45m, 36h, 3d, 2w, 6mo, 1y, " +
	"optionally followed by 'ago'"

// BoundError is a --since or --until value that is not one of the
// accepted forms. Its message is the reason, then the accepted forms.
type BoundError struct {
	Reason string
}

// Error returns the reason followed by FormsHelp.
func (e *BoundError) Error() string {
	return e.Reason + "; " + FormsHelp
}

// cannotRead returns the BoundError of a value that is not a time, with a
// detail in parentheses when one is given.
func cannotRead(text, detail string) *BoundError {
	reason := "cannot read " + strconv.Quote(text) + " as a time"
	if detail != "" {
		reason += " (" + detail + ")"
	}
	return &BoundError{Reason: reason}
}

// outOfRange returns the BoundError of a value whose time falls outside the
// years 1 to 9999.
func outOfRange(text string) *BoundError {
	return &BoundError{Reason: strconv.Quote(text) + " is out of range"}
}

// periodBound returns the first millisecond of the period from start to
// next, or with upper its last.
func periodBound(start, next time.Time, upper bool) string {
	if upper {
		return timeutil.FormatUTC(next.Add(-time.Millisecond))
	}
	return timeutil.FormatUTC(start)
}

// allDigits reports whether text is all ASCII digits; empty text is.
func allDigits(text string) bool {
	for i := 0; i < len(text); i++ {
		if text[i] < '0' || text[i] > '9' {
			return false
		}
	}
	return true
}

// isEpoch reports whether value is nine or more ASCII digits, optionally
// followed by a point and more digits.
func isEpoch(value string) bool {
	whole, fraction, hasPoint := strings.Cut(value, ".")
	if len(whole) < 9 || !allDigits(whole) {
		return false
	}
	return !hasPoint || (fraction != "" && allDigits(fraction))
}

// ParseBound returns the timeline timestamp a --since value (upper false)
// or an --until value (upper true) names, relative forms counted back from
// now. Surrounding whitespace is removed and each inner run of it read as
// one space.
func ParseBound(text string, upper bool, now time.Time) (string, error) {
	value := strings.Join(strings.Fields(text), " ")
	lowered := textlines.LowerASCII(value)
	now = now.UTC()
	switch lowered {
	case "now":
		return timeutil.FormatUTC(now), nil
	case "today", "yesterday":
		day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		if lowered == "yesterday" {
			day = day.AddDate(0, 0, -1)
		}
		if day.Year() < 1 {
			return "", outOfRange(text)
		}
		return periodBound(day, day.AddDate(0, 0, 1), upper), nil
	}
	if amount, unit, ok := matchRelative(lowered); ok {
		moment, ok := before(now, amount, unit)
		if !ok {
			return "", outOfRange(text)
		}
		return timeutil.FormatUTC(moment), nil
	}
	if isEpoch(value) {
		if bound := timeutil.FromText(value); bound != "" {
			return bound, nil
		}
		return "", &BoundError{Reason: strconv.Quote(text) +
			" is out of range as epoch seconds or milliseconds"}
	}
	return parseAbsolute(text, value, upper)
}

// number returns the value of a run of ASCII digits, false when it is
// empty, holds another byte or exceeds the range of an int64.
func number(digits string) (int64, bool) {
	if digits == "" || !allDigits(digits) {
		return 0, false
	}
	value, err := strconv.ParseInt(digits, 10, 64)
	return value, err == nil
}
