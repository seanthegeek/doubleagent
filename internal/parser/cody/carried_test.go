package cody

import (
	"fmt"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/parser"
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

// An account key goes into the heading row of every chat under it, so a
// key over the limit is cut there, with one problem for the history
// counting the cuts: no row holds it whole, and twice the chats cost twice
// the bytes, never the key's length again.
func TestAnAccountKeyIsCutInTheHeadingOfEveryChatUnderIt(t *testing.T) {
	const rel = "Cody-nodejs/JetBrains-globalState/cody-local-chatHistory-v2"
	account := "endpoint-" + strings.Repeat("A", 50_000)
	var sizes []int
	for _, n := range []int{20, 40} {
		texts := parseFile(t, rel, []byte(accountHistory(account, n)))
		size, whole, headings := 0, 0, 0
		for _, text := range texts {
			size += len(text)
			if strings.Contains(text, account) {
				whole++
			}
			if strings.HasPrefix(text, "chat: t | account=endpoint-AAA") {
				headings++
			}
		}
		sizes = append(sizes, size)
		want := []string{parser.ProblemPrefix + parser.CarriedProblem("account", n)}
		if got := parserRows(texts); whole != 0 || headings != n || fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("%d chats gave %d whole keys, %d headings and problems %q; want none, %d and %q", n,
				whole, headings, got, n, want)
		}
	}
	if growth := sizes[1] - sizes[0]; growth > 20*(parser.CarriedLimit+1024) {
		t.Errorf("20 more chats cost %d bytes", growth)
	}
}
