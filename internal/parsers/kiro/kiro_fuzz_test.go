package kiro

import "testing"

func FuzzSalvage(fuzz *testing.F) {
	for _, seed := range []string{`{"conversation_id": "c", "history": [{"x": 1}, {"y": `,
		`{"history": [1, , {"a":1}`, `"history":[`, "garbage"} {
		fuzz.Add(seed)
	}
	subject := New()
	fuzz.Fuzz(func(t *testing.T, text string) {
		_, _, _ = subject.salvage(nil, text)
		_ = unescapeHistory(text)
	})
}
