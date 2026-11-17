package cody

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/pyjson"
	"github.com/seanthegeek/doubleagent/internal/pytext"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// monthNames are the month names a mail date may use, short then long.
func monthNames() []string {
	return []string{"jan", "feb", "mar", "apr", "may", "jun", "jul", "aug", "sep", "oct", "nov", "dec",
		"january", "february", "march", "april", "may", "june", "july", "august", "september",
		"october", "november", "december"}
}

// dayNames are the short day names a mail date may open with.
func dayNames() []string {
	return []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}
}

// zoneOffsets are the zone names a mail date may end with, as an offset
// written hhmm with a sign.
func zoneOffsets() map[string]int64 {
	return map[string]int64{"UT": 0, "UTC": 0, "GMT": 0, "Z": 0, "AST": -400, "ADT": -300,
		"EST": -500, "EDT": -400, "CST": -600, "CDT": -500, "MST": -700, "MDT": -600,
		"PST": -800, "PDT": -700}
}

// chatTime returns the UTC time of a chat id written by JavaScript's
// Date.toUTCString(), else of fallback; each is tried first as a mail date
// and then as any time the timeline reads, and only when it is a non-empty
// string.
func chatTime(chatID string, fallback pyjson.Value) string {
	candidates := []string{chatID}
	if fallback.IsString() {
		candidates = append(candidates, fallback.Text())
	}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if iso, ok := mailDate(candidate); ok {
			return timeutil.FromText(iso)
		}
		if timestamp := timeutil.FromText(candidate); timestamp != "" {
			return timestamp
		}
	}
	return ""
}

// mailDate reads a date in the forms RFC 2822 and its predecessors allow
// (a day name or not, two- and four-digit years, a time with or without
// seconds or with dots, a numeric offset or a North American zone name; an
// unknown zone, or -0000, is UTC) and returns it as ISO 8601 text with its
// offset, or false. The fields are the ones Python's email date reader
// takes; it differs in reading only ASCII digits, letters and signs.
func mailDate(text string) (string, bool) {
	fields, ok := dateFields(text)
	if !ok {
		return "", false
	}
	day, month, year, clock, zone := fields[0], fields[1], fields[2], fields[3], fields[4]
	monthNumber, ok := monthOf(&day, &month)
	if !ok {
		return "", false
	}
	day = strings.TrimSuffix(day, ",")
	if strings.Index(year, ":") > 0 {
		year, clock = clock, year
	}
	if strings.HasSuffix(year, ",") {
		if year = year[:len(year)-1]; year == "" {
			return "", false
		}
	}
	if year == "" || year[0] < '0' || year[0] > '9' {
		year, zone = zone, year
	}
	clock = strings.TrimSuffix(clock, ",")
	parts, ok := clockParts(clock)
	if !ok {
		return "", false
	}
	numbers, ok := integers(year, day, parts[0], parts[1], parts[2])
	if !ok {
		return "", false
	}
	return isoText(numbers, monthNumber, zoneSeconds(zone))
}

// dateFields splits a mail date into its day, month, year, time and zone
// fields, dropping a day name and splitting an offset written against the
// time, or returns false when there are fewer than five.
func dateFields(text string) ([]string, bool) {
	fields := pytext.Fields(text)
	if len(fields) == 0 {
		return nil, false
	}
	if strings.HasSuffix(fields[0], ",") || slices.Contains(dayNames(), strings.ToLower(asciiOnly(fields[0]))) {
		fields = fields[1:]
	} else if i := strings.LastIndex(fields[0], ","); i >= 0 {
		fields[0] = fields[0][i+1:]
	}
	if len(fields) == 3 {
		if parts := strings.Split(fields[0], "-"); len(parts) == 3 {
			fields = append(parts, fields[1:]...)
		}
	}
	if len(fields) == 4 {
		last := fields[3]
		i := strings.Index(last, "+")
		if i < 0 {
			i = strings.Index(last, "-")
		}
		if i > 0 {
			fields = append(fields[:3:3], last[:i], last[i:])
		} else {
			fields = append(fields, "")
		}
	}
	if len(fields) < 5 || fields[0] == "" || fields[1] == "" || fields[2] == "" {
		return nil, false
	}
	return fields[:5], true
}

// asciiOnly returns text when it is ASCII and "" otherwise, so a non-ASCII
// word never matches a name.
func asciiOnly(text string) string {
	for i := range len(text) {
		if text[i] >= 0x80 {
			return ""
		}
	}
	return text
}

// monthOf returns the month number of the month field, or of the day field
// when the two are the other way round, swapping them back.
func monthOf(day, month *string) (int, bool) {
	names := monthNames()
	index := slices.Index(names, strings.ToLower(asciiOnly(*month)))
	if index < 0 {
		*day, *month = *month, strings.ToLower(*day)
		if index = slices.Index(names, asciiOnly(*month)); index < 0 {
			return 0, false
		}
	}
	return index%12 + 1, true
}

// clockParts returns the hour, minute and second of a time written with
// colons, or with dots, the second "0" when it is missing.
func clockParts(clock string) ([3]string, bool) {
	parts := strings.Split(clock, ":")
	if len(parts) == 1 && strings.Contains(clock, ".") {
		parts = strings.Split(clock, ".")
	}
	switch len(parts) {
	case 2:
		return [3]string{parts[0], parts[1], "0"}, true
	case 3:
		return [3]string{parts[0], parts[1], parts[2]}, true
	}
	return [3]string{}, false
}

// integerLimit bounds the integers a date field is read as: anything
// larger is out of every range checked later, as Python's unbounded
// integer is.
const integerLimit = 1_000_000_000_000_000

// integer reads a field as an integer with an optional sign and ASCII
// digits; one too large is held at integerLimit with its sign.
func integer(text string) (int64, bool) {
	digits := strings.TrimLeft(text, "+-")
	if len(text)-len(digits) > 1 || digits == "" || strings.Trim(digits, "0123456789") != "" {
		return 0, false
	}
	value, err := strconv.ParseInt(digits, 10, 64)
	if err != nil || value > integerLimit {
		value = integerLimit
	}
	if text[0] == '-' {
		value = -value
	}
	return value, true
}

// integers reads each field as an integer, or returns false.
func integers(fields ...string) ([]int64, bool) {
	values := make([]int64, len(fields))
	for i, field := range fields {
		value, ok := integer(field)
		if !ok {
			return nil, false
		}
		values[i] = value
	}
	return values, true
}

// zoneSeconds returns the offset of a zone field in seconds, or nil for
// an unknown zone and for -0000, which say nothing of the local zone.
func zoneSeconds(zone string) *int64 {
	zone = strings.ToUpper(asciiOnly(zone))
	offset, known := zoneOffsets()[zone]
	if !known {
		value, ok := integer(zone)
		if !ok || (value == 0 && strings.HasPrefix(zone, "-")) {
			return nil
		}
		offset = value
	}
	sign := int64(1)
	if offset < 0 {
		sign, offset = -1, -offset
	}
	seconds := sign * (offset/100*3600 + offset%100*60)
	return &seconds
}

// isoText returns year, day, hour, minute and second with the month and
// offset as ISO 8601 text, or false for a date or offset out of range. A
// year under 100 is POSIX's two-digit year: 69 to 99 in the twentieth
// century, the rest in the twenty-first.
func isoText(numbers []int64, month int, offset *int64) (string, bool) {
	year, day, hour, minute, second := numbers[0], numbers[1], numbers[2], numbers[3], numbers[4]
	if year < 100 {
		if year > 68 {
			year += 1900
		} else {
			year += 2000
		}
	}
	if year < 1 || year > 9999 || day < 1 || day > daysIn(year, month) || hour < 0 || hour > 23 ||
		minute < 0 || minute > 59 || second < 0 || second > 59 {
		return "", false
	}
	text := fmt.Sprintf("%04d-%02d-%02dT%02d:%02d:%02d", year, month, day, hour, minute, second)
	if offset == nil {
		return text, true
	}
	seconds, sign := *offset, "+"
	if seconds < 0 {
		seconds, sign = -seconds, "-"
	}
	if seconds >= 24*3600 {
		return "", false
	}
	return text + fmt.Sprintf("%s%02d:%02d", sign, seconds/3600, seconds%3600/60), true
}

// daysIn returns the number of days of a month of the Gregorian calendar.
func daysIn(year int64, month int) int64 {
	switch month {
	case 2:
		if year%4 == 0 && (year%100 != 0 || year%400 == 0) {
			return 29
		}
		return 28
	case 4, 6, 9, 11:
		return 30
	}
	return 31
}
