package parser

import (
	"errors"
	"fmt"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

// JSONPosition returns where a JSON document stopped decoding, "invalid
// JSON at line <n>, column <n>", without the reader's detail, which can
// quote the text there: the reason a row gives for a file that can hold a
// secret, such as an API key. An error without a position is "invalid
// JSON".
func JSONPosition(err error) string {
	var position *jsonvalue.Error
	if errors.As(err, &position) {
		return fmt.Sprintf("invalid JSON at line %d, column %d", position.Line, position.Column)
	}
	return "invalid JSON"
}
