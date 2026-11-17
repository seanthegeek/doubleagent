package timeutil

import (
	"strings"
	"time"
)

// fromISO returns the timestamp of ASCII text in one of the ISO 8601 forms
// the package comment lists, or "".
//
// The text is first rewritten as the Python analyzer rewrote it before
// handing it to its parser: a final Z or z becomes +00:00, and the first
// point followed by digits anywhere in the text has those digits cut or
// padded to six. The strict parse that follows then sees what that parser
// saw, so every form accepted here gives the time the analyzer gave.
func fromISO(text string) string {
	if strings.HasSuffix(text, "Z") || strings.HasSuffix(text, "z") {
		text = text[:len(text)-1] + "+00:00"
	}
	moment, ok := parseISO(sixDigitFraction(text))
	if !ok {
		return ""
	}
	return FormatUTC(moment)
}

// sixDigitFraction returns text with the digits after its first point that
// has digits after it cut or padded with zeros to six.
func sixDigitFraction(text string) string {
	for i := 0; i+1 < len(text); i++ {
		if text[i] != '.' || !isDigit(text[i+1]) {
			continue
		}
		end := i + 1
		for end < len(text) && isDigit(text[end]) {
			end++
		}
		fraction := (text[i+1:end] + "000000")[:6]
		return text[:i+1] + fraction + text[end:]
	}
	return text
}

// parseISO reads text, rewritten by fromISO, as a date, optionally followed
// by a separator, a time of day and an offset, and returns the moment in
// UTC; false when text is in no accepted form or the moment falls outside
// the years 1 to 9999.
func parseISO(text string) (time.Time, bool) {
	fields, ok := readFields(text, []fieldSpec{{0, 4, '-'}, {5, 2, '-'}, {8, 2, 0}})
	if !ok || text[4] != '-' || text[7] != '-' {
		return time.Time{}, false
	}
	if len(text) == 10 {
		return civilTime(fields[0], fields[1], fields[2], 0, 0, 0)
	}
	if isDigit(text[10]) {
		return time.Time{}, false
	}
	clock, rest, ok := parseClock(text[11:])
	if !ok {
		return time.Time{}, false
	}
	offset, ok := parseOffset(rest)
	if !ok {
		return time.Time{}, false
	}
	local, ok := civilTime(fields[0], fields[1], fields[2], clock.hour, clock.minute,
		clock.second)
	if !ok {
		return time.Time{}, false
	}
	moment := local.Add(time.Duration(clock.microsecond)*time.Microsecond - offset)
	if year := moment.Year(); year < 1 || year > 9999 {
		return time.Time{}, false
	}
	return moment, true
}

// timeOfDay is the time of day a timestamp names.
type timeOfDay struct {
	hour, minute, second, microsecond int
}

// parseClock reads HH:MM, HH:MM:SS or HH:MM:SS.ffffff at the start of text
// and returns it with the text after it.
func parseClock(text string) (timeOfDay, string, bool) {
	var clock timeOfDay
	fields, ok := readFields(text, []fieldSpec{{0, 2, ':'}, {3, 2, 0}})
	if !ok || len(text) < 3 || text[2] != ':' {
		return clock, "", false
	}
	clock.hour, clock.minute = fields[0], fields[1]
	rest := text[5:]
	if !strings.HasPrefix(rest, ":") {
		return clock, rest, true
	}
	if clock.second, ok = digits(rest, 1, 2); !ok {
		return clock, "", false
	}
	rest = rest[3:]
	if !strings.HasPrefix(rest, ".") {
		return clock, rest, true
	}
	if clock.microsecond, ok = digits(rest, 1, 6); !ok {
		return clock, "", false
	}
	return clock, rest[7:], true
}

// parseOffset reads what follows the time of day: nothing (UTC), or a sign
// and HH, HHMM or HH:MM, the hours below 24 and the minutes below 60.
func parseOffset(text string) (time.Duration, bool) {
	if text == "" {
		return 0, true
	}
	if text[0] != '+' && text[0] != '-' {
		return 0, false
	}
	hours, ok := digits(text, 1, 2)
	rest := text[min(3, len(text)):]
	minutes := 0
	if ok && rest != "" {
		rest = strings.TrimPrefix(rest, ":")
		minutes, ok = digits(rest, 0, 2)
		ok = ok && len(rest) == 2
	}
	if !ok || hours > 23 || minutes > 59 {
		return 0, false
	}
	offset := time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute
	if text[0] == '-' {
		offset = -offset
	}
	return offset, true
}

// isDigit reports whether char is an ASCII digit.
func isDigit(char byte) bool {
	return char >= '0' && char <= '9'
}
