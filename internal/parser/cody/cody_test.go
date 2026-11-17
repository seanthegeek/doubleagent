package cody

import (
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

// The expected values in this file come from running the Python helpers
// of the same names on the same inputs, except where a comment says the
// binary refuses a form Python reads.

func TestChatTimeReadsMailDatesAsPythonDoes(t *testing.T) {
	for _, test := range []struct{ text, want string }{
		{"Mon, 03 Mar 2025 10:00:00 GMT", "2025-03-03T10:00:00.000Z"},
		{"Mon,03 Mar 2025 10:00:00 GMT", "2025-03-03T10:00:00.000Z"},
		{"03 Mar 2025 10:00:00 GMT", "2025-03-03T10:00:00.000Z"},
		{"Mon, 3 Mar 2025 10:00 GMT", "2025-03-03T10:00:00.000Z"},
		{"Mon, 03 Mar 2025 10:00:00 +0530", "2025-03-03T04:30:00.000Z"},
		{"Mon, 03 Mar 2025 10:00:00 -0500", "2025-03-03T15:00:00.000Z"},
		{"Mon, 03 Mar 2025 10:00:00 EST", "2025-03-03T15:00:00.000Z"},
		{"Mon, 03 Mar 2025 10:00:00 PDT", "2025-03-03T17:00:00.000Z"},
		{"Mon, 03 Mar 2025 10:00:00 -0000", "2025-03-03T10:00:00.000Z"},
		{"Mon, 03 Mar 2025 10:00:00 XYZ", "2025-03-03T10:00:00.000Z"},
		{"Mon, 03 Mar 2025 10:00:00", "2025-03-03T10:00:00.000Z"},
		{"Mon, 03 Mar 25 10:00:00 GMT", "2025-03-03T10:00:00.000Z"},
		{"Mon, 03 Mar 69 10:00:00 GMT", "1969-03-03T10:00:00.000Z"},
		{"Mon, 03 Mar 68 10:00:00 GMT", "2068-03-03T10:00:00.000Z"},
		{"Monday, 03-Mar-25 10:00:00 GMT", "2025-03-03T10:00:00.000Z"},
		{"Mar 03 2025 10:00:00 GMT", "2025-03-03T10:00:00.000Z"},
		{"03 March 2025 10:00:00 GMT", "2025-03-03T10:00:00.000Z"},
		{"03 Mar 2025 10.00.05 GMT", "2025-03-03T10:00:05.000Z"},
		{"03 Mar 2025 10.00 GMT", "2025-03-03T10:00:00.000Z"},
		{"03 Mar 10:00:00 2025 GMT", "2025-03-03T10:00:00.000Z"},
		{"03 Mar 2025 10:00:00+0200", "2025-03-03T08:00:00.000Z"},
		{"03 Mar 2025 25:00:00 GMT", ""},
		{"31 Feb 2025 10:00:00 GMT", ""},
		{"03 Mar 2025 10:00:00 +9999", ""},
		{"03 Mar 2025 10:00:00 +2359", "2025-03-02T10:01:00.000Z"},
		{"03 Mar 2025 10:00:00 +0199", "2025-03-03T07:21:00.000Z"},
		{"03 Mar 2025 10:00:00 est", "2025-03-03T15:00:00.000Z"},
		{"03 mar 2025 10:00:00 gmt", "2025-03-03T10:00:00.000Z"},
		{"2025-03-03T10:00:00Z", "2025-03-03T10:00:00.000Z"},
		{"1741000000000", "2025-03-03T11:06:40.000Z"},
		{"", ""},
		{"   ", ""},
		{"Mon, 03 Mar 2025", ""},
		{"x, 03 Mar 2025 10:00:00 GMT", "2025-03-03T10:00:00.000Z"},
		{"03 Mar 2025, 10:00:00 GMT", "2025-03-03T10:00:00.000Z"},
		{"03 Mar 2025 10:00:00, GMT", "2025-03-03T10:00:00.000Z"},
		{"03 Xyz 2025 10:00:00 GMT", ""},
		{"03 Mar -5 10:00:00 GMT", ""},
		{"03 Mar 0 10:00:00 GMT", "2000-03-03T10:00:00.000Z"},
		{"03 Mar 2025 1:2:3 GMT", "2025-03-03T01:02:03.000Z"},
		{"03 Mar 2025 10:00:00:00 GMT", ""},
		{"Mon, 03 Mar 2025 10:00:00 GMT extra", "2025-03-03T10:00:00.000Z"},
		{"03 Mar 2025 10:00:00 +99999999999999999999", ""},
		{"03 Mar 99999999999999999999 10:00:00 GMT", ""},
		{"03 Mar 2025 10:00:00 UT", "2025-03-03T10:00:00.000Z"},
		{"Mon, 03 Mar 2025 10:00:00 Z", "2025-03-03T10:00:00.000Z"},
		{"03 Mar 0099 10:00:00 GMT", "1999-03-03T10:00:00.000Z"},
		{"03 Mar 0001 00:00:00 +0100", "2001-03-02T23:00:00.000Z"},
		{"03 Mar 2025 10:00:00 +05", "2025-03-03T09:55:00.000Z"},
		{"03 Mar 2025 10:00:00 05:00", "2025-03-03T10:00:00.000Z"},
		{"Mon, 03 Mar 2025 10:00:00 -0500 (EST)", "2025-03-03T15:00:00.000Z"},
		{"03-Mar-2025 10:00:00 GMT", "2025-03-03T10:00:00.000Z"},
		{"a-b-c 10:00:00 GMT", ""},
		// Refused: Python upper-cases the ligature U+FB06 to "ST" and reads the
		// zone as EST; only ASCII names are read here, so the zone is unknown
		// and the time is UTC.
		{"03 Mar 2025 10:00:00 E\ufb06", "2025-03-03T10:00:00.000Z"},
		// Refused: Python's int() reads "1_0" as 10.
		{"03 Mar 2025 1_0:00:00 GMT", ""},
		// Refused: Python's int() reads non-ASCII digits.
		{"03 Mar 2025 \u0661\u0660:00:00 GMT", ""},
	} {
		t.Run(test.text, func(t *testing.T) {
			if got := chatTime(test.text, jsonvalue.NewNull()); got != test.want {
				t.Errorf("chatTime(%q) = %q, want %q", test.text, got, test.want)
			}
		})
	}
}

func TestChatTimeFallsBackToTheLastInteraction(t *testing.T) {
	for _, test := range []struct {
		name, chatID string
		fallback     jsonvalue.Value
		want         string
	}{
		{"an ISO fallback", "", jsonvalue.NewString("2025-01-01T00:00:00Z"), "2025-01-01T00:00:00.000Z"},
		{"a number is not read", "junk", jsonvalue.NewInt(5), ""},
		{"a mail date fallback", "junk", jsonvalue.NewString("Mon, 03 Mar 2025 10:00:00 GMT"), "2025-03-03T10:00:00.000Z"},
		{"a chat id wins", "Sat, 03 Oct 2026 10:00:00 GMT", jsonvalue.NewString("Sat, 03 Oct 2026 11:00:00 GMT"),
			"2026-10-03T10:00:00.000Z"},
		{"a uuid chat id", "0f0e-uuid", jsonvalue.NewNull(), ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := chatTime(test.chatID, test.fallback); got != test.want {
				t.Errorf("chatTime(%q, %v) = %q, want %q", test.chatID, test.fallback, got, test.want)
			}
		})
	}
}

func TestRecoveredKeysTakeJSONWhitespaceAndASCIIDigitsOnly(t *testing.T) {
	date := "Wed, 01 Oct 2026 08:00:00 GMT"
	template := `{"acct": {"chat": {"%DATE%"%SPACE%:%AFTER%{"interactions": [], "chatTitle": "t"}, `
	build := func(date, space, after string) string {
		return strings.NewReplacer("%DATE%", date, "%SPACE%", space, "%AFTER%", after).Replace(template)
	}
	chats, _, _ := recoverHistory(build(date, " \n", "\t"))
	if keys := chats.Get("acct").Get("chat").Keys(); len(keys) != 1 || keys[0] != date {
		t.Errorf("recovered chats = %v, want [%q]", keys, date)
	}
	for _, space := range []string{"\u00a0", "\u2028", "\u0085"} {
		if chats, _, _ := recoverHistory(build(date, space, "")); chats.Len() != 0 {
			t.Errorf("a key followed by %q recovers %s", space, chats.JSON())
		}
	}
	if chats, _, _ := recoverHistory(build(strings.Replace(date, "01 Oct", "0\u0661 Oct", 1), "", "")); chats.Len() != 0 {
		t.Errorf("a key with a non-ASCII digit recovers %s", chats.JSON())
	}
}

func TestRecoverHistoryKeepsTheInteractionsBeforeTheDamage(t *testing.T) {
	text := `{"a": {"chat": {"Wed, 01 Oct 2026 08:00:00 GMT": {"chatTitle": "one", "interactions": [{"x": 1}, {"y": ` +
		`}, "b": {"chat": {"Wed, 01 Oct 2026 08:00:00 GMT": {"interactions": [1, {"z": 2}]}`
	history, message, errors := recoverHistory(text)
	want := `{"a":{"chat":{"Wed, 01 Oct 2026 08:00:00 GMT":{"id":"Wed, 01 Oct 2026 08:00:00 GMT",` +
		`"interactions":[{"x":1}],"chatTitle":"one"}}},"b":{"chat":{"Wed, 01 Oct 2026 08:00:00 GMT":` +
		`{"id":"Wed, 01 Oct 2026 08:00:00 GMT","interactions":[{"z":2}]}}}}`
	if got := history.JSON(); got != want {
		t.Errorf("recovered history = %s\nwant %s", got, want)
	}
	if message != "chat history did not parse; recovered 2 chat(s)" {
		t.Errorf("message = %q", message)
	}
	if len(errors) != 1 || errors[0].sessionID != "a/Wed, 01 Oct 2026 08:00:00 GMT" || errors[0].problem.Line != 2 {
		t.Errorf("errors = %+v", errors)
	}
}

func TestLegacyReaderStopsAtTheFirstDamage(t *testing.T) {
	for _, test := range []struct {
		name, document string
		chats          int
		fails          bool
	}{
		{"two chats", "<a><chat/><chat/></a>", 2, false},
		{"a second element", "<a><chat/></a><b/>", 1, true},
		{"an empty file", "", 0, true},
		{"a comment alone", "<!-- c -->", 0, true},
		{"an unclosed element", "<a><chat/>", 1, true},
		{"a mismatched tag", "<a><chat/></b>", 1, true},
		{"a byte-order mark", "\ufeff<a><chat/></a>", 1, false},
		{"trailing whitespace", "<a><chat/></a>\n  ", 1, false},
		{"text before the element", "x<a/>", 0, true},
		{"an undefined entity", "<a>&bogus;<chat/></a>", 0, true},
		{"a declaration", `<?xml version="1.0" encoding="UTF-8"?><a><chat/></a>`, 1, false},
		{"a control character", "<a><chat/>\x01</a>", 1, true},
		{"nested chats", "<a><chat><chat/></chat></a>", 2, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			reader := &legacyReader{onChat: func(*element, string) {}}
			err := reader.read([]byte(test.document))
			if reader.chats != test.chats || (err != nil) != test.fails {
				t.Errorf("read = %d chats, %v; want %d chats, failing %v", reader.chats, err, test.chats, test.fails)
			}
		})
	}
}

func TestLegacyReaderWordsTheDamageInPlainWords(t *testing.T) {
	for _, test := range []struct {
		name, document, want string
	}{
		{"an empty file has no root element", "", "no root element: line 1, column 1"},
		{"a second element is outside the root", "<a/><b/>", "content outside the root element: line 1, column 9"},
		{"text is outside the root", "x<a/>", "content outside the root element: line 1, column 2"},
	} {
		t.Run(test.name, func(t *testing.T) {
			reader := &legacyReader{onChat: func(*element, string) {}}
			if err := reader.read([]byte(test.document)); err == nil || err.Error() != test.want {
				t.Errorf("got %v, want %q", err, test.want)
			}
		})
	}
}

func FuzzLegacyReader(fuzz *testing.F) {
	fuzz.Add([]byte(`<history><AccountData><accountId value="a"/><chats><chat><messages><message>` +
		`<speaker value="HUMAN"/></message></messages></chat></chats></AccountData></history>`))
	fuzz.Fuzz(func(t *testing.T, data []byte) {
		reader := &legacyReader{onChat: func(*element, string) {}}
		_ = reader.read(data)
	})
}

func FuzzRecoverHistory(fuzz *testing.F) {
	fuzz.Add(`{"a": {"chat": {"Wed, 01 Oct 2026 08:00:00 GMT": {"interactions": [{"x": 1}, {"y": `)
	fuzz.Fuzz(func(t *testing.T, text string) {
		_, _, _ = recoverHistory(text)
	})
}
