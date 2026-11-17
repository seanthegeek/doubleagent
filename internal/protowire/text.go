package protowire

import (
	"errors"
	"math"
	"strings"
	"time"
	"unicode/utf8"
)

func isMalformed(err error) bool {
	return errors.Is(err, ErrMalformed)
}

// continuationRange gives, per lead byte, the range its first continuation
// byte may take; the other continuation bytes are 0x80 to 0xBF. It is the
// table of well-formed UTF-8 in the Unicode standard (Table 3-7).
func continuationRange(lead byte, position int) (low, high byte) {
	if position == 0 {
		switch lead {
		case 0xE0:
			return 0xA0, 0xBF
		case 0xED:
			return 0x80, 0x9F
		case 0xF0:
			return 0x90, 0xBF
		case 0xF4:
			return 0x80, 0x8F
		}
	}
	return 0x80, 0xBF
}

// sequenceLength returns how many bytes a lead byte promises, 0 for a byte
// that can never start a sequence.
func sequenceLength(lead byte) int {
	switch {
	case lead >= 0xC2 && lead <= 0xDF:
		return 2
	case lead >= 0xE0 && lead <= 0xEF:
		return 3
	case lead >= 0xF0 && lead <= 0xF4:
		return 4
	}
	return 0
}

// invalidRun returns how many bytes at the start of data make up one
// ill-formed sequence: the lead byte and every continuation byte that is
// still valid, at least one byte.
func invalidRun(data []byte) int {
	want := sequenceLength(data[0])
	used := 1
	for used < want && used < len(data) {
		low, high := continuationRange(data[0], used-1)
		if data[used] < low || data[used] > high {
			break
		}
		used++
	}
	return used
}

// decodeUTF8 turns bytes into a string, writing one U+FFFD for each maximal
// ill-formed sequence, as Python's `bytes.decode("utf-8", "replace")` does.
func decodeUTF8(data []byte) string {
	if utf8.Valid(data) {
		return string(data)
	}
	var text strings.Builder
	for len(data) > 0 {
		character, size := utf8.DecodeRune(data)
		if character == utf8.RuneError && size <= 1 {
			text.WriteRune(utf8.RuneError)
			data = data[invalidRun(data):]
			continue
		}
		text.Write(data[:size])
		data = data[size:]
	}
	return text.String()
}

const (
	earliestEpoch = -62135596800 // 0001-01-01T00:00:00Z
	latestEpoch   = 253402300799 // 9999-12-31T23:59:59Z
)

// epochText formats an epoch number as the analyzer's timestamp: a number
// above 1e11 is milliseconds, any other seconds. The fraction is rounded to
// microseconds, half to even, then cut to milliseconds. A time outside years
// 1 to 9999 gives "".
func epochText(epoch float64) string {
	if epoch > 1e11 {
		epoch /= 1000.0
	}
	whole, fraction := math.Modf(epoch)
	micros := math.RoundToEven(fraction * 1e6)
	if micros >= 1e6 {
		micros -= 1e6
		whole++
	} else if micros < 0 {
		micros += 1e6
		whole--
	}
	if math.IsNaN(whole) || whole < earliestEpoch || whole > latestEpoch {
		return ""
	}
	return formatSeconds(int64(whole), int64(micros)/1000)
}

// formatSeconds writes epoch seconds and milliseconds as UTC text.
func formatSeconds(seconds, millis int64) string {
	moment := time.Unix(seconds, millis*int64(time.Millisecond)).UTC()
	return moment.Format("2006-01-02T15:04:05.000Z")
}
