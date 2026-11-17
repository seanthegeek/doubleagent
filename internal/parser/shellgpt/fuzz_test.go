package shellgpt

import "testing"

func FuzzLoadMessages(fuzz *testing.F) {
	for _, seed := range []string{``, `[]`, `[{"role":"user"}`, `[{"role":"a"}, 3,`, `{"a":1}`, "[\xff"} {
		fuzz.Add(seed)
	}
	fuzz.Fuzz(func(t *testing.T, raw string) {
		messages, failure := loadMessages(raw)
		if len(messages) > 0 && !messages[0].IsObject() {
			t.Errorf("a message that is not an object: %v", messages[0])
		}
		_ = failure
	})
}
