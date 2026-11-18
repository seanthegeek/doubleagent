package clinelegacy

import "github.com/seanthegeek/doubleagent/internal/jsonvalue"

// The two helpers below are kept for the Continue and PearAI parsers, which
// still call them; this package's own code uses parser.FirstNonEmpty and
// cmp.Compare. The file goes when those parsers stop calling them.

// FirstTruthy returns the first of values that is not empty
// (jsonvalue.Value.IsEmpty), else the last value.
func FirstTruthy(values ...jsonvalue.Value) jsonvalue.Value {
	for _, value := range values[:len(values)-1] {
		if !value.IsEmpty() {
			return value
		}
	}
	return values[len(values)-1]
}

// CompareNumbers returns -1, 0 or 1 as left is below, equal to or above
// right, the order a sort by number gives.
func CompareNumbers(left, right float64) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	}
	return 0
}
