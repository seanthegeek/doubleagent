package pytext

import "strings"

// CodePointLen returns len(text) as Python counts it: the number of code
// points, a lone surrogate's three-byte form counting as one.
func CodePointLen(text string) int {
	count := 0
	for i := 0; i < len(text); count++ {
		_, width := NextCodePoint(text[i:])
		i += width
	}
	return count
}

// SliceCodePoints returns text[:n] as Python slices a str: the first n code
// points, or for a negative n all but the last -n, and the whole text or
// nothing when n is beyond either end.
func SliceCodePoints(text string, n int) string {
	if n < 0 {
		n += CodePointLen(text)
		if n <= 0 {
			return ""
		}
	}
	i := 0
	for count := 0; count < n && i < len(text); count++ {
		_, width := NextCodePoint(text[i:])
		i += width
	}
	return text[:i]
}

// PadRight returns text followed by spaces up to width code points, as
// Python's "%-14s" pads; text that long or longer is returned unchanged.
func PadRight(text string, width int) string {
	return text + padding(text, width)
}

// PadLeft returns text preceded by spaces up to width code points, as
// Python's "%14s" pads; text that long or longer is returned unchanged.
func PadLeft(text string, width int) string {
	return padding(text, width) + text
}

// padding returns the spaces that bring text to width code points.
func padding(text string, width int) string {
	missing := width - CodePointLen(text)
	if missing <= 0 {
		return ""
	}
	return strings.Repeat(" ", missing)
}
