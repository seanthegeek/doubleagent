package musecode

import (
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

func FuzzResultAndArguments(fuzz *testing.F) {
	for _, seed := range []string{``, `{"output":"o","exit_code":2}`, `{"command":"x"}`, `{bad`, `[1]`, `"q"`} {
		fuzz.Add(seed)
	}
	fuzz.Fuzz(func(t *testing.T, text string) {
		value := jsonvalue.NewString(text)
		resultText(nil, value)
		argumentsSummary(nil, value)
		microsToUTC(value)
	})
}
