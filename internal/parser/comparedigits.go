package parser

import "strings"

// CompareDigits orders two strings of ASCII digits by the numbers they
// write, however long, as strings.Compare orders text: negative, zero or
// positive. Leading zeros do not count, so "007" and "7" are equal. An
// agent numbers its chunk and event files this way, and a name that
// overflows an int64 still sorts (strconv.ParseInt would refuse it).
func CompareDigits(left, right string) int {
	left, right = strings.TrimLeft(left, "0"), strings.TrimLeft(right, "0")
	if len(left) != len(right) {
		return len(left) - len(right)
	}
	return strings.Compare(left, right)
}
