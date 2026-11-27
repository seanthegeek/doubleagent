package cody

import (
	"fmt"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/parserkit"
)

// accountHistory returns a JetBrains chat history of one account, keyed
// account, holding n chats of no interactions.
func accountHistory(account string, n int) string {
	chats := make([]string, 0, n)
	for i := range n {
		chats = append(chats, fmt.Sprintf(`"c%d": {"id": "c%d", "chatTitle": "t", "interactions": []}`, i, i))
	}
	return `{"` + account + `": {"chat": {` + strings.Join(chats, ", ") + `}}}`
}

// legacyHistory returns a legacy JetBrains history of n chats under one
// AccountData whose accountId is account, or, when own, n chats in the
// deprecated top-level list, each naming account as its own accountId.
func legacyHistory(account string, own bool, n int) string {
	if own {
		chat := `<chat><internalId value="i"/><accountId value="` + account + `"/></chat>`
		return `<application><component name="ChatHistory"><chats><list>` + strings.Repeat(chat, n) +
			`</list></chats></component></application>`
	}
	chats := strings.Repeat(`<chat><internalId value="i"/></chat>`, n)
	return `<application><component name="ChatHistory"><accountData><list><AccountData><accountId value="` +
		account + `"/><chats><list>` + chats + `</list></chats></AccountData></list></accountData></component></application>`
}

// An account key or id goes into the heading row of every chat under it,
// so one over the limit is cut there, with one problem for the history
// counting the cuts: no row holds it whole, and twice the chats cost twice
// the bytes, never the key's length again. A legacy chat's own accountId
// is what the chat itself says, written whole in its one heading.
func TestAnAccountIsCutInTheHeadingOfEveryChatUnderIt(t *testing.T) {
	account := "endpoint-" + strings.Repeat("A", 50_000)
	tests := []struct {
		name, rel, heading string
		history            func(n int) string
		cut                bool
	}{
		{"an account key of a JetBrains chat history", "Cody-nodejs/JetBrains-globalState/cody-local-chatHistory-v2",
			"chat: t | account=endpoint-AAA", func(n int) string { return accountHistory(account, n) }, true},
		{"the accountId of a legacy AccountData", ".idea/cody_history.xml", "chat: i | account=endpoint-AAA",
			func(n int) string { return legacyHistory(account, false, n) }, true},
		{"a legacy chat's own accountId", ".idea/cody_history.xml", "chat: i | account=endpoint-AAA",
			func(n int) string { return legacyHistory(account, true, n) }, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var sizes []int
			for _, n := range []int{20, 40} {
				texts := parseFile(t, test.rel, []byte(test.history(n)))
				size, whole, headings := 0, 0, 0
				for _, text := range texts {
					size += len(text)
					if strings.Contains(text, account) {
						whole++
					}
					if strings.HasPrefix(text, test.heading) {
						headings++
					}
				}
				sizes = append(sizes, size)
				want, wantWhole := []string{parserkit.ProblemPrefix + parserkit.CarriedProblem("account", n)}, 0
				if !test.cut {
					want, wantWhole = nil, n
				}
				if got := parserRows(texts); whole != wantWhole || headings != n || fmt.Sprint(got) != fmt.Sprint(want) {
					t.Errorf("%d chats gave %d whole keys, %d headings and problems %q; want %d, %d and %q", n,
						whole, headings, got, wantWhole, n, want)
				}
			}
			if growth := sizes[1] - sizes[0]; test.cut && growth > 20*(parserkit.CarriedLimit+1024) {
				t.Errorf("20 more chats cost %d bytes", growth)
			}
		})
	}
}
