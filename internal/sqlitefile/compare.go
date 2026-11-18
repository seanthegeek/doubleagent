package sqlitefile

import (
	"bytes"
	"cmp"
	"math"
)

// Compare orders two values as SQL's ORDER BY does with the BINARY
// collation ("Sort Order" in https://www.sqlite.org/datatype3.html): NULL
// first, then integers and floats by numeric value, then text by its
// bytes, then blobs by their bytes. It returns -1, 0 or 1.
func Compare(left, right Value) int {
	return compareCollated(left, right, "")
}

// Equal reports whether SQL's "left = right" holds for two values that carry
// no affinity, such as the results of functions: equal numbers of either
// class, or the same text or the same blob. NULL equals nothing.
func Equal(left, right Value) bool {
	return left.Type != Null && right.Type != Null && Compare(left, right) == 0
}

// storageRank places a value's storage class in the sort order.
func storageRank(value Value) int {
	switch value.Type {
	case Null:
		return 0
	case Integer, Float:
		return 1
	case Text:
		return 2
	}
	return 3
}

// compareCollated orders two values, comparing text under collation:
// "NOCASE" folds ASCII letters, "RTRIM" ignores trailing spaces, and any
// other name, "" and "BINARY" included, compares bytes. A collation an
// application registers itself is not available here.
func compareCollated(left, right Value, collation string) int {
	if leftRank, rightRank := storageRank(left), storageRank(right); leftRank != rightRank {
		return cmp.Compare(leftRank, rightRank)
	}
	switch left.Type {
	case Null:
		return 0
	case Integer, Float:
		return compareNumbers(left, right)
	case Text:
		return compareText(left.Bytes, right.Bytes, collation)
	}
	return bytes.Compare(left.Bytes, right.Bytes)
}

// compareNumbers compares two numbers exactly, an integer with a float
// included, as SQLite does without rounding the integer to a float.
func compareNumbers(left, right Value) int {
	switch {
	case left.Type == Integer && right.Type == Integer:
		return cmp.Compare(left.Int, right.Int)
	case left.Type == Float && right.Type == Float:
		return cmp.Compare(left.Float, right.Float)
	case left.Type == Integer:
		return compareIntFloat(left.Int, right.Float)
	}
	return -compareIntFloat(right.Int, left.Float)
}

// compareIntFloat compares an integer with a float exactly.
func compareIntFloat(integer int64, number float64) int {
	switch {
	case number < -9223372036854775808:
		return 1
	case number >= 9223372036854775808:
		return -1
	}
	whole := math.Trunc(number)
	if order := cmp.Compare(integer, int64(whole)); order != 0 {
		return order
	}
	return cmp.Compare(whole, number)
}

// compareText compares two texts under a collation.
func compareText(left, right []byte, collation string) int {
	switch collation {
	case "NOCASE":
		return bytes.Compare(foldBytes(left), foldBytes(right))
	case "RTRIM":
		return bytes.Compare(bytes.TrimRight(left, " "), bytes.TrimRight(right, " "))
	}
	return bytes.Compare(left, right)
}

// foldBytes returns text with its ASCII capitals lowered, the NOCASE rule.
func foldBytes(text []byte) []byte {
	folded := make([]byte, len(text))
	for i, octet := range text {
		folded[i] = lowerByte(octet)
	}
	return folded
}
