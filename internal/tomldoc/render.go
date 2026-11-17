package tomldoc

import (
	"math"
	"sort"
	"strconv"
	"strings"
)

// SettingText returns a value as the text of a setting row, as Python's
// `setting_value` writes it: `true` or `false` for a boolean, a string as it
// is, a date or time as Python writes it, and anything else as compact JSON
// with the keys of every table sorted.
func SettingText(value Value) string {
	switch value.kind {
	case KindBoolean:
		if value.boolean {
			return "true"
		}
		return "false"
	case KindString, KindDatetime, KindLocalDatetime, KindLocalDate, KindLocalTime:
		return value.text
	}
	return CompactJSON(value)
}

// CompactJSON returns a value as JSON with no spaces, the keys of every table
// sorted and non-ASCII characters kept, as Python's
// `json.dumps(value, ensure_ascii=False, separators=(",", ":"), sort_keys=True)`
// writes it. A date or time inside an array or table is written as a JSON
// string of its text.
func CompactJSON(value Value) string {
	var out strings.Builder
	writeJSON(&out, value)
	return out.String()
}

func writeJSON(out *strings.Builder, value Value) {
	switch value.kind {
	case KindInteger:
		out.WriteString(strconv.FormatInt(value.integer, 10))
	case KindFloat:
		out.WriteString(jsonFloat(value.float))
	case KindBoolean:
		out.WriteString(strconv.FormatBool(value.boolean))
	case KindArray:
		out.WriteByte('[')
		for i, item := range value.items {
			if i > 0 {
				out.WriteByte(',')
			}
			writeJSON(out, item)
		}
		out.WriteByte(']')
	case KindTable:
		writeJSONTable(out, value.table)
	default:
		writeJSONString(out, value.text)
	}
}

func writeJSONTable(out *strings.Builder, table *Table) {
	entries := table.Entries()
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].Key < entries[j].Key })
	out.WriteByte('{')
	for i, entry := range entries {
		if i > 0 {
			out.WriteByte(',')
		}
		writeJSONString(out, entry.Key)
		out.WriteByte(':')
		writeJSON(out, entry.Value)
	}
	out.WriteByte('}')
}

// writeJSONString writes a JSON string with only the escapes Python's
// encoder makes when it keeps non-ASCII characters.
func writeJSONString(out *strings.Builder, text string) {
	out.WriteByte('"')
	for _, character := range text {
		switch {
		case character == '"':
			out.WriteString(`\"`)
		case character == '\\':
			out.WriteString(`\\`)
		case character == '\n':
			out.WriteString(`\n`)
		case character == '\r':
			out.WriteString(`\r`)
		case character == '\t':
			out.WriteString(`\t`)
		case character == '\b':
			out.WriteString(`\b`)
		case character == '\f':
			out.WriteString(`\f`)
		case character < 0x20:
			out.WriteString(`\u00`)
			out.WriteByte("0123456789abcdef"[character>>4])
			out.WriteByte("0123456789abcdef"[character&15])
		default:
			out.WriteRune(character)
		}
	}
	out.WriteByte('"')
}

// jsonFloat writes a float as Python's `json.dumps` does: the shortest text
// that reads back as the same float, `1.0` for a whole number, an exponent
// from 1e16 up and below 1e-4, and NaN and Infinity as words.
func jsonFloat(number float64) string {
	switch {
	case math.IsNaN(number):
		return "NaN"
	case math.IsInf(number, 1):
		return "Infinity"
	case math.IsInf(number, -1):
		return "-Infinity"
	}
	scientific := strconv.FormatFloat(number, 'e', -1, 64)
	mantissa, exponentText, _ := strings.Cut(scientific, "e")
	exponent, _ := strconv.Atoi(exponentText)
	negative := strings.HasPrefix(mantissa, "-")
	digits := strings.Replace(strings.TrimPrefix(mantissa, "-"), ".", "", 1)
	sign := ""
	if negative {
		sign = "-"
	}
	decimalPoint := exponent + 1
	if decimalPoint <= -4 || decimalPoint > 16 {
		return sign + exponentForm(digits, exponent)
	}
	return sign + fixedForm(digits, decimalPoint)
}

// exponentForm writes digits as `d.ddde+XX`.
func exponentForm(digits string, exponent int) string {
	text := digits[:1]
	if len(digits) > 1 {
		text += "." + digits[1:]
	}
	sign := "+"
	if exponent < 0 {
		sign, exponent = "-", -exponent
	}
	return text + "e" + sign + leftPad(strconv.Itoa(exponent), 2)
}

// fixedForm writes digits with the decimal point placed decimalPoint digits in.
func fixedForm(digits string, decimalPoint int) string {
	switch {
	case decimalPoint <= 0:
		return "0." + strings.Repeat("0", -decimalPoint) + digits
	case decimalPoint >= len(digits):
		return digits + strings.Repeat("0", decimalPoint-len(digits)) + ".0"
	}
	return digits[:decimalPoint] + "." + digits[decimalPoint:]
}

func leftPad(text string, width int) string {
	if len(text) >= width {
		return text
	}
	return strings.Repeat("0", width-len(text)) + text
}
