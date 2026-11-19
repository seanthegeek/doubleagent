package cody

import (
	"slices"
	"strings"
	"testing"
	"time"
)

// damagedChats returns a JetBrains history cut mid-write holding n chats,
// each under its own date and holding one interaction that does not decode.
func damagedChats(n int) string {
	var text strings.Builder
	text.WriteString(`{"https://sourcegraph.com/-alice": {"chat": {`)
	start := time.Date(2026, time.January, 5, 0, 0, 0, 0, time.UTC)
	for i := range n {
		date := start.Add(time.Duration(i) * time.Minute).Format("Mon, 02 Jan 2006 15:04:05 GMT")
		text.WriteString(`"` + date + `": {"interactions": [x]},`)
	}
	return text.String()
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
		{"an interaction holding a date key, read past the chat's end", `{"a": {"chat": {"` + first +
			`": {"interactions": [{"` + second + `": {}}, {"y": ]}}}`,
			[]string{"chat:  | account=a | interactions=1", "chat:  | account=a | interactions=0",
				`parser: unparseable record 2 (invalid JSON at line 1, column 113: invalid character ']' after object key:value pair)`}},
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
