package parser

import (
	"math"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

// NumberOrder returns the number a value sorts by: a JSON number's nearest
// float64, a number beyond the float64 range as +Inf or -Inf by its sign so
// it sorts after or before every number that has a float64, and 0 for any
// other value.
func NumberOrder(value jsonvalue.Value) float64 {
	number, ok := value.Float()
	switch {
	case ok:
		return number
	case !value.IsNumber():
		return 0
	case strings.HasPrefix(value.Text(), "-"):
		return math.Inf(-1)
	}
	return math.Inf(1)
}
