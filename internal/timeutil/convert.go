package timeutil

import (
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/textlines"
)

// The range of whole seconds since the epoch that falls in the years 1 to
// 9999, the years a timestamp can hold.
const (
	firstSecond = -62135596800 // 0001-01-01T00:00:00Z
	lastSecond  = 253402300799 // 9999-12-31T23:59:59Z
)

// millisecondsAbove is the epoch number above which a value counts
// milliseconds rather than seconds (about the year 5138 in seconds).
const millisecondsAbove = 1e11

// FromValue returns the timestamp of a decoded JSON value: a string as
// FromText reads it, a number as FromEpoch reads it, and "" for null, a
// boolean, an array, an object and an integer too large for a float.
func FromValue(value jsonvalue.Value) string {
	switch {
	case value.IsString():
		return FromText(value.StringValue())
	case value.IsNumber():
		epoch, ok := value.Float()
		if !ok {
			return ""
		}
		return FromEpoch(epoch)
	}
	return ""
}

// FromText returns the timestamp text spells, or "": text trimmed of ASCII
// whitespace is an epoch number when it is ASCII digits with at most one
// point, else an ISO 8601 date and time in a form the package comment
// lists. Text holding a non-ASCII character gives "".
func FromText(text string) string {
	text = strings.Trim(text, textlines.ASCIIWhitespace)
	if text == "" || !isASCII(text) {
		return ""
	}
	if isEpochText(text) {
		// A value past the float range reads as infinity, which FromEpoch refuses.
		epoch, _ := strconv.ParseFloat(text, 64)
		return FromEpoch(epoch)
	}
	return fromISO(text)
}

// FromInt returns the timestamp of an integer epoch number, as FromEpoch
// reads the nearest float.
func FromInt(epoch int64) string {
	return FromEpoch(float64(epoch))
}

// FromEpoch returns the timestamp of an epoch number: seconds, or
// milliseconds when above 1e11. The time is first rounded to microseconds,
// halves to even, then written with milliseconds truncated; a time outside
// the years 1 to 9999, NaN and an infinity give "".
func FromEpoch(epoch float64) string {
	if epoch > millisecondsAbove {
		epoch /= 1000
	}
	if math.IsNaN(epoch) || math.IsInf(epoch, 0) {
		return ""
	}
	whole, fraction := math.Modf(epoch)
	microseconds := math.RoundToEven(fraction * 1e6)
	switch {
	case microseconds >= 1e6:
		microseconds -= 1e6
		whole++
	case microseconds < 0:
		microseconds += 1e6
		whole--
	}
	if whole < firstSecond || whole > lastSecond {
		return ""
	}
	return FormatUTC(time.Unix(int64(whole), int64(microseconds)*int64(time.Microsecond)))
}

// isASCII reports whether every byte of text is ASCII.
func isASCII(text string) bool {
	for i := 0; i < len(text); i++ {
		if text[i] >= 0x80 {
			return false
		}
	}
	return true
}

// isEpochText reports whether text is ASCII digits with at most one point
// among them, and at least one digit.
func isEpochText(text string) bool {
	points, digitCount := 0, 0
	for i := 0; i < len(text); i++ {
		switch char := text[i]; {
		case char == '.':
			points++
		case char >= '0' && char <= '9':
			digitCount++
		default:
			return false
		}
	}
	return points <= 1 && digitCount > 0
}
