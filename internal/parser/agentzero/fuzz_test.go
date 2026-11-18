package agentzero

import "testing"

func FuzzRecoverLogs(fuzz *testing.F) {
	for _, seed := range []string{``, `{"logs": [{"no": 0}, {"no": 1`, `"logs" : [ {} , 3`, "\xff"} {
		fuzz.Add(seed)
	}
	subject := New()
	fuzz.Fuzz(func(t *testing.T, raw string) {
		subject.recoverLogs(raw)
	})
}
