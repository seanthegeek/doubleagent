package timeutil

import (
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/textlines"
)

// The first and last millisecond of the years 1 to 9999 since the epoch.
const (
	firstMillisecond = -62135596800000 // 0001-01-01T00:00:00.000Z
	lastMillisecond  = 253402300799999 // 9999-12-31T23:59:59.999Z
)

// millisecondsAbove is the epoch number above which a value counts
// milliseconds rather than seconds (about the year 5138 in seconds).
const millisecondsAbove = 100_000_000_000

// fromMilliseconds returns the timestamp of a count of milliseconds since
// the epoch, or "" outside the years 1 to 9999.
func fromMilliseconds(milliseconds int64) string {
	if milliseconds < firstMillisecond || milliseconds > lastMillisecond {
		return ""
	}
	return FormatUTC(time.UnixMilli(milliseconds))
}

// fromDecimal returns the timestamp of an epoch written as an optional
// minus, digits and an optional point and digits: its whole milliseconds,
// the fraction of a millisecond dropped toward the earlier time.
func fromDecimal(text string) string {
	negative := strings.HasPrefix(text, "-")
	whole, fraction, _ := strings.Cut(strings.TrimPrefix(text, "-"), ".")
	units, err := strconv.ParseInt("0"+whole, 10, 64)
	if err != nil {
		return ""
	}
	digits := 3
	if !negative && (units > millisecondsAbove ||
		units == millisecondsAbove && strings.Trim(fraction, "0") != "") {
		digits = 0
	}
	kept := (fraction + "000")[:digits]
	if units > math.MaxInt64/1000 {
		return ""
	}
	milliseconds := units
	if digits == 3 {
		milliseconds = units * 1000
	}
	if part, _ := strconv.ParseInt("0"+kept, 10, 64); negative {
		milliseconds = -milliseconds - part
		if strings.Trim(fraction[min(len(fraction), digits):], "0") != "" {
			milliseconds--
		}
	} else {
		milliseconds += part
	}
	return fromMilliseconds(milliseconds)
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

// FromText returns the timestamp text spells, or "": text trimmed of ASCII
// whitespace is an epoch when it is ASCII digits with at most one point,
// else a date and time in a form the package comment lists. Text holding a
// non-ASCII character gives "".
func FromText(text string) string {
	text = strings.Trim(text, textlines.ASCIIWhitespace)
	if text == "" || !isASCII(text) {
		return ""
	}
	if isEpochText(text) {
		return fromDecimal(text)
	}
	return fromISO(text)
}

// FromEpoch returns the timestamp of a float epoch: milliseconds when it is
// above 1e11, else seconds, read from the shortest decimal that gives the
// float back and cut to the millisecond. NaN and an infinity give "".
func FromEpoch(epoch float64) string {
	if math.IsNaN(epoch) || math.IsInf(epoch, 0) {
		return ""
	}
	return fromDecimal(strconv.FormatFloat(epoch, 'f', -1, 64))
}

// FromValue returns the timestamp of a decoded JSON value: a string as
// FromText reads it, a number as an epoch read from its source text, and
// "" for null, a boolean, an array and an object.
func FromValue(value jsonvalue.Value) string {
	switch {
	case value.IsString():
		return FromText(value.StringValue())
	case value.IsNumber():
		text := string(value.Number())
		if strings.ContainsAny(text, "eE") {
			number, ok := value.Float()
			if !ok {
				return ""
			}
			return FromEpoch(number)
		}
		return fromDecimal(text)
	}
	return ""
}

// FromInt returns the timestamp of an integer epoch: milliseconds when it
// is above 1e11, else seconds.
func FromInt(epoch int64) string {
	if epoch > millisecondsAbove {
		return fromMilliseconds(epoch)
	}
	if epoch < firstMillisecond/1000 || epoch > lastMillisecond/1000 {
		return ""
	}
	return fromMilliseconds(epoch * 1000)
}
