package parserkit

import "testing"

func TestCompareDigitsOrdersDigitStringsByValue(t *testing.T) {
	tests := []struct {
		name, left, right string
		want              int
	}{
		{"leading zeros do not count", "00001", "1", 0},
		{"a shorter number is smaller", "00002", "00010", -1},
		{"a longer number is larger", "100000", "99999", 1},
		{"numbers beyond an int64 compare digit by digit",
			"123456789012345678901234567890", "123456789012345678901234567891", -1},
		{"zero and the empty string are equal", "0", "", 0},
		{"a larger digit is larger", "2", "1", 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := CompareDigits(test.left, test.right)
			if (got > 0) != (test.want > 0) || (got < 0) != (test.want < 0) {
				t.Errorf("CompareDigits(%q, %q) = %d, want the sign of %d", test.left, test.right, got, test.want)
			}
		})
	}
}
