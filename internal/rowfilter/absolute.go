package rowfilter

import (
	"fmt"
	"strings"
	"time"

	"github.com/seanthegeek/doubleagent/internal/textlines"
)

// precision is the last unit an absolute value was written to, which sets
// the period it names.
type precision uint8

// The precisions, from the coarsest; an instant names no period.
const (
	inYear precision = iota
	inMonth
	inDay
	inHour
	inMinute
	inSecond
	instant
)

// dateTime is an absolute value as written: its fields, offset in minutes
// east of UTC, and precision.
type dateTime struct {
	year, month, day, hour, minute, second, microsecond int
	offset                                              int
	precision                                           precision
}

// setDay sets d's date to the calendar day of moment, with day precision.
func (d *dateTime) setDay(moment time.Time) {
	d.year, d.month, d.day = moment.Year(), int(moment.Month()), moment.Day()
	d.precision = inDay
}

// setWeekDate sets the date of a week and weekday of d's ISO year, whose
// first week holds 4 January, and returns a detail for a field out of
// range, or "". Week 53 exists only in a year whose last week holds its
// last Thursday, so in another year it is refused rather than read as the
// next year's first week.
func (d *dateTime) setWeekDate(week, weekday int) string {
	switch {
	case week < 1 || week > 53:
		return fmt.Sprintf("week must be in 1..53, not %d", week)
	case weekday < 1 || weekday > 7:
		return fmt.Sprintf("weekday must be in 1..7, not %d", weekday)
	case d.year < 1:
		return fmt.Sprintf("year must be in 1..9999, not %d", d.year)
	}
	fourth := time.Date(d.year, time.January, 4, 0, 0, 0, 0, time.UTC)
	monday := fourth.AddDate(0, 0, -((int(fourth.Weekday()) + 6) % 7))
	if year, _ := monday.AddDate(0, 0, 52*7).ISOWeek(); week == 53 && year != d.year {
		return fmt.Sprintf("week must be in 1..52 for %04d, not %d", d.year, week)
	}
	d.setDay(monday.AddDate(0, 0, (week-1)*7+weekday-1))
	return ""
}

// setOrdinalDate sets the date of a day of d's year, counted from 1, and
// returns a detail for a day the year does not have, or "".
func (d *dateTime) setOrdinalDate(ordinal int) string {
	days := 365
	if daysIn(d.year, time.February) == 29 {
		days = 366
	}
	switch {
	case d.year < 1:
		return fmt.Sprintf("year must be in 1..9999, not %d", d.year)
	case ordinal < 1 || ordinal > days:
		return fmt.Sprintf("ordinal day must be in 1..%d for %04d, not %d", days, d.year,
			ordinal)
	}
	d.setDay(time.Date(d.year, time.January, ordinal, 0, 0, 0, 0, time.UTC))
	return ""
}

// isDigit reports whether char is an ASCII digit.
func isDigit(char byte) bool {
	return char >= '0' && char <= '9'
}

// digitsAt returns the number the width ASCII digits at offset start of
// text spell, and false when text is too short or holds another byte.
func digitsAt(text string, start, width int) (int, bool) {
	if start+width > len(text) {
		return 0, false
	}
	value := 0
	for i := start; i < start+width; i++ {
		if !isDigit(text[i]) {
			return 0, false
		}
		value = value*10 + int(text[i]-'0')
	}
	return value, true
}

// readWeekOrOrdinal reads an ISO week date (YYYY-Www-D, YYYYWwwD, or
// YYYY-Www and YYYYWww for the week's Monday) or an ordinal date
// (YYYY-DDD, YYYYDDD) at the start of value, after the year d already
// holds, and returns the text after it. A week date without its day ends
// the value; the dash is used throughout or not at all.
func (d *dateTime) readWeekOrOrdinal(value string) (string, string, bool) {
	dashed := value[4] == '-'
	rest := value[4:]
	if dashed {
		rest = rest[1:]
	}
	if week, ok := digitsAt(rest, 1, 2); ok && rest[0] == 'W' {
		rest = rest[3:]
		weekday := 1
		if rest != "" {
			if dashed {
				if rest[0] != '-' {
					return "", "", false
				}
				rest = rest[1:]
			}
			if weekday, ok = digitsAt(rest, 0, 1); !ok {
				return "", "", false
			}
			rest = rest[1:]
		}
		detail := d.setWeekDate(week, weekday)
		return rest, detail, detail == ""
	}
	ordinal, ok := digitsAt(rest, 0, 3)
	// A basic ordinal date before a space is refused: seven digits there
	// read as well as a year, a month and a day, so the form is ambiguous.
	if !ok || (len(rest) > 3 && (isDigit(rest[3]) || (!dashed && rest[3] == ' '))) {
		return "", "", false
	}
	detail := d.setOrdinalDate(ordinal)
	return rest[3:], detail, detail == ""
}

// readDate reads YYYY, YYYY-MM, YYYY-MM-DD or YYYYMMDD, or an ISO week
// date or ordinal date (readWeekOrOrdinal), at the start of value and
// returns the text after it, with a detail when a field is out of range.
func (d *dateTime) readDate(value string) (string, string, bool) {
	year, ok := digitsAt(value, 0, 4)
	if !ok {
		return "", "", false
	}
	d.year, d.month, d.day = year, 1, 1
	if len(value) == 4 {
		d.precision = inYear
		return "", "", true
	}
	if month, ok := digitsAt(value, 5, 2); ok && value[4] == '-' {
		d.month = month
		if len(value) == 7 {
			d.precision = inMonth
			return "", "", true
		}
		if day, ok := digitsAt(value, 8, 2); ok && value[7] == '-' {
			d.day, d.precision = day, inDay
			return value[10:], "", true
		}
	}
	if month, ok := digitsAt(value, 4, 2); ok {
		if day, ok := digitsAt(value, 6, 2); ok && (len(value) == 8 || !isDigit(value[8])) {
			d.month, d.day, d.precision = month, day, inDay
			return value[8:], "", true
		}
	}
	return d.readWeekOrOrdinal(value)
}

// readClock reads hh, hh:mm, hhmm, hh:mm:ss or hhmmss, the seconds
// optionally followed by a point and a fraction, at the start of text and
// returns the text after it. The colons are used throughout or not at all.
func (d *dateTime) readClock(text string) (string, bool) {
	hour, ok := digitsAt(text, 0, 2)
	if !ok {
		return "", false
	}
	d.hour, d.precision, text = hour, inHour, text[2:]
	colon := strings.HasPrefix(text, ":")
	for _, field := range []*int{&d.minute, &d.second} {
		next := text
		if colon {
			if !strings.HasPrefix(next, ":") {
				return text, true
			}
			next = next[1:]
		}
		value, ok := digitsAt(next, 0, 2)
		if !ok {
			return text, !colon
		}
		*field, d.precision, text = value, d.precision+1, next[2:]
	}
	fraction, ok := strings.CutPrefix(text, ".")
	if !ok {
		return text, true
	}
	end := 0
	for end < len(fraction) && isDigit(fraction[end]) {
		end++
	}
	if end == 0 {
		return "", false
	}
	d.microsecond, _ = digitsAt((fraction[:end] + "00000")[:6], 0, 6)
	d.precision = instant
	return fraction[end:], true
}

// readOffset reads what follows the time of day: nothing (UTC), or Z, z,
// +HH, +HHMM or +HH:MM directly after it or after one space, where UTC in
// any case also means Z. Out-of-range offset fields give a detail.
func (d *dateTime) readOffset(text string) (string, bool) {
	spaced, hasSpace := strings.CutPrefix(text, " ")
	if hasSpace && textlines.LowerASCII(spaced) == "utc" {
		return "", true
	}
	if hasSpace {
		text = spaced
	}
	switch {
	case text == "":
		return "", !hasSpace
	case text == "Z" || text == "z":
		return "", true
	case text[0] != '+' && text[0] != '-':
		return "", false
	}
	hours, ok := digitsAt(text, 1, 2)
	minutes := 0
	switch rest := text[min(3, len(text)):]; {
	case !ok:
		return "", false
	case rest == "":
	case len(rest) == 2 || (len(rest) == 3 && rest[0] == ':'):
		if minutes, ok = digitsAt(rest, len(rest)-2, 2); !ok {
			return "", false
		}
	default:
		return "", false
	}
	if minutes > 59 {
		return fmt.Sprintf("offset minutes must be in 0..59, not %d", minutes), false
	}
	if hours > 23 {
		return fmt.Sprintf("offset hours must be in 0..23, not %d", hours), false
	}
	d.offset = hours*60 + minutes
	if text[0] == '-' {
		d.offset = -d.offset
	}
	return "", true
}

// readDateTime reads value as a date, optionally followed by T or a space
// and a time of day with an optional offset. It returns the fields, or
// false with a detail for the message when value is not in such a form.
func readDateTime(value string) (dateTime, string, bool) {
	var fields dateTime
	rest, detail, ok := fields.readDate(value)
	if !ok {
		return fields, detail, false
	}
	if rest == "" {
		return fields, "", true
	}
	if fields.precision != inDay || (rest[0] != 'T' && rest[0] != ' ') {
		return fields, "", false
	}
	rest, ok = fields.readClock(rest[1:])
	if !ok {
		return fields, "", false
	}
	detail, ok = fields.readOffset(rest)
	return fields, detail, ok
}

// rangeProblem returns the first field outside its range, in the order
// year, month, day, hour, minute, second, as a detail for the message;
// or "".
func (d *dateTime) rangeProblem() string {
	switch {
	case d.year < 1 || d.year > 9999:
		return fmt.Sprintf("year must be in 1..9999, not %d", d.year)
	case d.month < 1 || d.month > 12:
		return fmt.Sprintf("month must be in 1..12, not %d", d.month)
	case d.day < 1 || d.day > daysIn(d.year, time.Month(d.month)):
		return fmt.Sprintf("day must be in 1..%d for %04d-%02d, not %d",
			daysIn(d.year, time.Month(d.month)), d.year, d.month, d.day)
	case d.hour > 23:
		return fmt.Sprintf("hour must be in 0..23, not %d", d.hour)
	case d.minute > 59:
		return fmt.Sprintf("minute must be in 0..59, not %d", d.minute)
	case d.second > 59:
		return fmt.Sprintf("second must be in 0..59, not %d", d.second)
	}
	return ""
}

// parseAbsolute returns the bound an absolute value names, or the
// BoundError quoting text, the value as given, when it is not one.
func parseAbsolute(text, value string, upper bool) (string, error) {
	fields, detail, ok := readDateTime(value)
	if !ok {
		return "", cannotRead(text, detail)
	}
	if detail := fields.rangeProblem(); detail != "" {
		return "", cannotRead(text, detail)
	}
	start := time.Date(fields.year, time.Month(fields.month), fields.day, fields.hour,
		fields.minute, fields.second, fields.microsecond*1000, time.UTC).Add(
		-time.Duration(fields.offset) * time.Minute)
	if start.Year() < 1 || start.Year() > 9999 {
		return "", cannotRead(text, "the time is out of range")
	}
	var next time.Time
	switch fields.precision {
	case instant:
		return periodBound(start, start, false), nil
	case inYear:
		next = start.AddDate(1, 0, 0)
	case inMonth:
		next = start.AddDate(0, 1, 0)
	case inDay:
		next = start.AddDate(0, 0, 1)
	case inHour:
		next = start.Add(time.Hour)
	case inMinute:
		next = start.Add(time.Minute)
	default:
		next = start.Add(time.Second)
	}
	if upper && next.Add(-time.Millisecond).Year() > 9999 {
		return "", cannotRead(text, "the time is out of range")
	}
	return periodBound(start, next, upper), nil
}
