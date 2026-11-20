package cody

import (
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// chatDate returns the i-th of a run of chat keys a minute apart.
func chatDate(i int) string {
	start := time.Date(2026, time.January, 5, 0, 0, 0, 0, time.UTC)
	return start.Add(time.Duration(i) * time.Minute).Format("Mon, 02 Jan 2006 15:04:05 GMT")
}

// damagedChats returns a JetBrains history cut mid-write holding n chats,
// each under its own date and holding one interaction that does not decode.
func damagedChats(n int) string {
	var text strings.Builder
	text.WriteString(`{"https://sourcegraph.com/-alice": {"chat": {`)
	for i := range n {
		text.WriteString(`"` + chatDate(i) + `": {"interactions": [x]},`)
	}
	return text.String()
}

// nestedChats returns a history of n chats whose first interaction holds
// the next chat's key, each padded with a 200-byte string: cut off after
// the last when closed is false, else closed and followed by other text.
func nestedChats(n int, closed bool) string {
	var text strings.Builder
	text.WriteString(`{"a": {"chat": {`)
	for i := range n {
		text.WriteString(`"` + chatDate(i) + `": {"interactions": [{"pad": "` + strings.Repeat("p", 200) + `", `)
	}
	text.WriteString(`"z": 1`)
	if closed {
		text.WriteString(strings.Repeat(`}]}`, n) + `}}} other text`)
	}
	return text.String()
}

// damagedAccounts returns a history of n accounts, each with a whole chat
// and a last chat whose interaction does not decode.
func damagedAccounts(n int) string {
	var text strings.Builder
	text.WriteString(`{`)
	for i := range n {
		text.WriteString(`"account` + strconv.Itoa(i) + `": {"chat": {"` + chatDate(2*i) + `": {"interactions": [{"ok": 1}]}, "` +
			chatDate(2*i+1) + `": {"interactions": [{"bad": ]}}},`)
	}
	return text.String()
}

// keysInsideInteractions returns a history of n chats, each holding an
// interaction with an account's key and one with a chat's key, then an
// interaction that does not decode.
func keysInsideInteractions(n int) string {
	var text strings.Builder
	text.WriteString(`{"a": {"chat": {`)
	for i := range n {
		text.WriteString(`"` + chatDate(2*i) + `": {"interactions": [{"b": {"chat": {}}}, {"` + chatDate(2*i+1) +
			`": {"interactions": [{"bad": ]}}, {"bad": ]}, `)
	}
	return text.String()
}

func TestRecoveryReadsEachByteOfTheHistoryOnce(t *testing.T) {
	for _, test := range []struct {
		name  string
		build func(n int) string
	}{
		{"chats each holding the next chat's key, cut off", func(n int) string { return nestedChats(n, false) }},
		{"chats each holding the next chat's key, closed", func(n int) string { return nestedChats(n, true) }},
		{"chats each with a damaged interaction", damagedChats},
		{"accounts each with a damaged last chat", damagedAccounts},
		{"chats with keys inside their interactions, then damage", keysInsideInteractions},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, n := range []int{100, 200} {
				text := test.build(n)
				walk := newRecovery(text)
				_, _, found := walk.chats()
				shown := 0
				for _, entry := range found {
					shown += len(entry.problem.Raw)
				}
				// Two patterns, the interactions key and the title, each search
				// a chat's text once.
				if walk.walked > len(text) || walk.searched > 2*len(text) || walk.document.Scanned() > len(text) ||
					shown > len(text) {
					t.Errorf("%d chats: a %d-byte text was decoded for %d bytes, searched for %d, counted for "+
						"%d and shown for %d; want each at most the text, searched at most twice", n, len(text),
						walk.walked, walk.searched, walk.document.Scanned(), shown)
				}
			}
		})
	}
}

// recordText returns the record text of the unparseable-record rows the
// parser gives a JetBrains history, and how many there are.
func recordText(t *testing.T, text string) (string, int) {
	t.Helper()
	var shown strings.Builder
	count := 0
	for _, row := range parseFile(t, "Cody-nodejs/JetBrains-globalState/cody-local-chatHistory-v2", []byte(text)) {
		if strings.HasPrefix(row, "parser: unparseable record") {
			count++
			shown.WriteString(row[strings.LastIndex(row, "): ")+len("): "):])
		}
	}
	return shown.String(), count
}

func TestADamagedInteractionShowsItsRecordOnlyToTheEndOfItsChat(t *testing.T) {
	for _, n := range []int{200, 400} {
		shown, count := recordText(t, damagedChats(n))
		if want := strings.Repeat("x]},", n); count != n || shown != want {
			t.Errorf("%d damaged chats give %d rows showing %d bytes, want %d rows showing %d bytes (4 per chat)",
				n, count, len(shown), n, len(want))
		}
	}
}

func TestTheRecordOfADamagedInteractionEndsWhereTheNextKeyStarts(t *testing.T) {
	const first, second = "Mon, 05 Jan 2026 00:00:00 GMT", "Tue, 06 Jan 2026 00:00:00 GMT"
	for _, test := range []struct {
		name, text string
		want       []string
	}{
		{"a chat before another chat", `{"a": {"chat": {"` + first + `": {"interactions": [{"x": 1}, {"y": ]}, ` +
			`"` + second + `": {"interactions": []}}}`,
			[]string{"chat:  | account=a | interactions=1", "chat:  | account=a | interactions=0",
				`parser: unparseable record 2 (invalid JSON at line 1, column 84: invalid character ']' after object key:value pair): {"y": ]}, `}},
		{"the last chat", `{"a": {"chat": {"` + first + `": {"interactions": [{"y": ]}}` + "\n",
			[]string{"chat:  | account=a | interactions=0",
				`parser: unparseable record 1 (invalid JSON at line 1, column 74: invalid character ']' after object key:value pair): {"y": ]}}`}},
		{"a chat before an account", `{"a": {"chat": {"` + first + `": {"interactions": [{"y": ]}}}, ` +
			`"b": {"chat": {}}}`,
			[]string{"chat:  | account=a | interactions=0",
				`parser: unparseable record 1 (invalid JSON at line 1, column 74: invalid character ']' after object key:value pair): {"y": ]}}}, `}},
		{"an interaction holding a date key keeps it, in a damaged last chat", `{"a": {"chat": {"` + first +
			`": {"interactions": [{"` + second + `": {}}, {"y": ]}}}`,
			[]string{"chat:  | account=a | interactions=1",
				`parser: unparseable record 2 (invalid JSON at line 1, column 113: invalid character ']' after object key:value pair): {"y": ]}}}`}},
		{"an interaction holding a date key keeps it, before another chat", `{"a": {"chat": {"` + first +
			`": {"interactions": [{"` + second + `": {"interactions": []}}]}, "Wed, 07 Jan 2026 00:00:00 GMT": ` +
			`{"interactions": [{"y": ]}}}`,
			[]string{"chat:  | account=a | interactions=1", "chat:  | account=a | interactions=0",
				`parser: unparseable record 1 (invalid JSON at line 1, column 184: invalid character ']' after object key:value pair): {"y": ]}}}`}},
		{"a chat after a damaged string still starts at its key", `{"a": {"chat": {"` + first +
			`": {"interactions": [{"y": "cut}, "` + second + `": {"interactions": [{"x": 1}]}}}`,
			[]string{"chat:  | account=a | interactions=0", "chat:  | account=a | interactions=1",
				`parser: unparseable record 1 (invalid JSON at line 1, column 82: invalid character 'T' after object key:value pair): {"y": "cut}, `}},
		{"each damaged chat names its own line and column", "{\"a\": {\"chat\": {\"" + first +
			"\": {\"interactions\": [{\"é\": ]}, \n\"" + second + "\": {\"interactions\": [{\"é\": ]}}}",
			[]string{"chat:  | account=a | interactions=0", "chat:  | account=a | interactions=0",
				"parser: unparseable record 1 (invalid JSON at line 1, column 74: invalid character ']' after object key:value pair): {\"é\": ]}, ",
				"parser: unparseable record 1 (invalid JSON at line 2, column 58: invalid character ']' after object key:value pair): {\"é\": ]}}}"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var got []string
			for _, row := range parseFile(t, "Cody-nodejs/JetBrains-globalState/cody-local-chatHistory-v2", []byte(test.text)) {
				if strings.HasPrefix(row, "parser: unparseable record") || strings.HasPrefix(row, "chat: ") {
					got = append(got, row)
				}
			}
			if !slices.Equal(got, test.want) {
				t.Errorf("rows %q\nwant %q", got, test.want)
			}
		})
	}
}
