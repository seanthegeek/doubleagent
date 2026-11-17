package rowfilter

import (
	"strings"
	"time"
)

// unitSeconds returns the length in seconds of a fixed unit, or 0 for a
// calendar unit (months, years) and for text that names no unit.
func unitSeconds(unit string) int64 {
	switch unit {
	case "m", "min", "mins", "minute", "minutes":
		return 60
	case "h", "hour", "hours":
		return 3600
	case "d", "day", "days":
		return 86400
	case "w", "week", "weeks":
		return 7 * 86400
	}
	return 0
}

// isCalendarUnit reports whether unit names months or years, and which.
func isCalendarUnit(unit string) (months, years bool) {
	switch unit {
	case "mo", "month", "months":
		return true, false
	case "y", "year", "years":
		return false, true
	}
	return false, false
}

// spanLimit is a number of seconds longer than the years 1 to 9999, so a
// count of fixed units past it is out of range whatever the clock says.
const spanLimit = 10_000 * 366 * 86400

// matchRelative returns the count and unit of a lowered relative time: a
// run of ASCII digits, an optional space, a unit and an optional " ago".
func matchRelative(lowered string) (count, unit string, ok bool) {
	end := 0
	for end < len(lowered) && lowered[end] >= '0' && lowered[end] <= '9' {
		end++
	}
	unit = strings.TrimSuffix(strings.TrimPrefix(lowered[end:], " "), " ago")
	months, years := isCalendarUnit(unit)
	if end == 0 || (unitSeconds(unit) == 0 && !months && !years) {
		return "", "", false
	}
	return lowered[:end], unit, true
}

// before returns the time a count of units before now, and false when the
// count or the result falls outside the years 1 to 9999. Months and years
// move the calendar date and keep the time of day; a day the target month
// lacks becomes its last day.
func before(now time.Time, count, unit string) (time.Time, bool) {
	amount, ok := number(count)
	if !ok {
		return time.Time{}, false
	}
	var moment time.Time
	switch months, years := isCalendarUnit(unit); {
	case years:
		if amount > 10_000 {
			return time.Time{}, false
		}
		moment, ok = monthsBefore(now, amount*12)
	case months:
		if amount > 120_000 {
			return time.Time{}, false
		}
		moment, ok = monthsBefore(now, amount)
	default:
		seconds := unitSeconds(unit)
		if amount > spanLimit/seconds {
			return time.Time{}, false
		}
		moment = time.Unix(now.Unix()-amount*seconds, int64(now.Nanosecond())).UTC()
		ok = true
	}
	if !ok || moment.Year() < 1 || moment.Year() > 9999 {
		return time.Time{}, false
	}
	return moment, true
}

// monthsBefore returns the time a number of calendar months before now,
// the day kept or cut to the end of the target month, and false before the
// year 1.
func monthsBefore(now time.Time, months int64) (time.Time, bool) {
	total := int64(now.Year())*12 + int64(now.Month()) - 1 - months
	if total < 12 {
		return time.Time{}, false
	}
	year, month := int(total/12), time.Month(total%12+1)
	day := min(now.Day(), daysIn(year, month))
	return time.Date(year, month, day, now.Hour(), now.Minute(), now.Second(),
		now.Nanosecond(), time.UTC), true
}

// daysIn returns the number of days in a month of a year.
func daysIn(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}
