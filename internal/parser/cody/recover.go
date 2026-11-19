package cody

import (
	"cmp"
	"regexp"
	"slices"
	"strconv"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
)

// jsonSpace is the whitespace JSON allows between tokens, and keyColon a
// key's colon with it.
const (
	jsonSpace = `[ \t\n\r]*`
	keyColon  = jsonSpace + ":" + jsonSpace
)

// The keys a damaged history is walked by: a chat's key, a date as
// Date.toUTCString() writes it; an account's key followed by its "chat"
// object; and a chat's interactions and title.
var (
	dateKey = regexp.MustCompile(`"((?:Mon|Tue|Wed|Thu|Fri|Sat|Sun), [0-9]{2} [A-Z][a-z]{2} [0-9]{4} ` +
		`[0-9]{2}:[0-9]{2}:[0-9]{2} GMT)"` + keyColon + `\{`)
	accountKey      = regexp.MustCompile(`"([^"\\]+)"` + keyColon + `\{` + jsonSpace + `"chat"` + keyColon + `\{`)
	interactionsKey = regexp.MustCompile(`"interactions"` + keyColon)
	titleKey        = regexp.MustCompile(`"chatTitle"` + keyColon + `("(?:[^"\\]|\\.)*")`)
)

// recoveredError is a damaged interaction of a recovered history: its
// problem, the account and key of its chat, and the session id the chat's
// rows carry.
type recoveredError struct {
	problem       parser.Problem
	account, chat string
	sessionID     string
}

// historyMark is where an account or a chat starts in a damaged history,
// and where its key starts.
type historyMark struct {
	offset, keyStart int
	isChat           bool
	name             string
}

// historyMarks returns the accounts and chats of a history's text, in text
// order: an account from its key, a chat from the end of its key.
func historyMarks(text string) []historyMark {
	var marks []historyMark
	for _, found := range accountKey.FindAllStringSubmatchIndex(text, -1) {
		marks = append(marks, historyMark{offset: found[0], keyStart: found[0], name: text[found[2]:found[3]]})
	}
	for _, found := range dateKey.FindAllStringSubmatchIndex(text, -1) {
		marks = append(marks, historyMark{offset: found[1], keyStart: found[0], isChat: true,
			name: text[found[2]:found[3]]})
	}
	slices.SortStableFunc(marks, func(left, right historyMark) int {
		if order := cmp.Compare(left.offset, right.offset); order != 0 {
			return order
		}
		if left.isChat != right.isChat { // an account sorts before a chat at the same offset
			if left.isChat {
				return 1
			}
			return -1
		}
		return cmp.Compare(left.name, right.name)
	})
	return marks
}

// recoverHistory returns the chats that can be read from a damaged
// history's text, with the interactions that decode, a message saying how
// many, and the damaged interactions. A chat is one whose key is a
// Date.toUTCString() date; it belongs to the account key before it, and
// runs to the next account or chat key.
func recoverHistory(text string) (jsonvalue.Value, string, []recoveredError) {
	history := jsonvalue.NewObject()
	var found []recoveredError
	account := ""
	chats := 0
	marks := historyMarks(text)
	for i, mark := range marks {
		if !mark.isChat {
			account = mark.name
			continue
		}
		end, shownEnd := len(text), len(text)
		if i+1 < len(marks) {
			end, shownEnd = marks[i+1].offset, marks[i+1].keyStart
		}
		chat, problems := recoverChat(text, mark, end, shownEnd)
		for _, problem := range problems {
			found = append(found, recoveredError{problem: problem, account: account, chat: mark.name})
		}
		holder, ok := history.Lookup(account)
		if !ok {
			holder = jsonvalue.NewObject(jsonvalue.Member{Key: "chat", Value: jsonvalue.NewObject()})
			history.Set(account, holder)
		}
		holder.Get("chat").Set(mark.name, chat)
		chats++
	}
	return history, "chat history did not parse; recovered " + strconv.Itoa(chats) + " chat(s)",
		recoveredSessions(history, found)
}

// recoverChat returns the chat whose key ends at mark, read up to end: its
// id, the interactions that decode and its title when it decodes, and the
// problems of its damaged interactions. The interactions array is walked
// past end when an interaction holds a key that looks like a chat's, but a
// damaged interaction's record is shown only up to shownEnd, where the next
// key starts, and not at all when it starts after that: each part of the
// text is shown once however many chats are damaged.
func recoverChat(text string, mark historyMark, end, shownEnd int) (jsonvalue.Value, parser.Problems) {
	var problems parser.Problems
	interactions := []jsonvalue.Value{}
	if found := interactionsKey.FindStringIndex(text[mark.offset:end]); found != nil {
		for _, value := range parser.ArrayAtShownTo(text, mark.offset+found[1], shownEnd, &problems, nil) {
			if value.IsObject() {
				interactions = append(interactions, value)
			}
		}
	}
	chat := jsonvalue.NewObject(jsonvalue.Member{Key: "id", Value: jsonvalue.NewString(mark.name)},
		jsonvalue.Member{Key: "interactions", Value: jsonvalue.NewArray(interactions...)})
	if found := titleKey.FindStringSubmatch(text[mark.offset:end]); found != nil {
		if title, err := jsonvalue.DecodeString(found[1]); err == nil {
			chat.Set("chatTitle", title)
		}
	}
	return chat, problems
}

// recoveredSessions gives each damaged interaction the session id of its
// chat: the chat key, or "<account>/<chat key>" when the key is under more
// than one account.
func recoveredSessions(history jsonvalue.Value, found []recoveredError) []recoveredError {
	counts := map[string]int{}
	for _, holder := range history.Members() {
		for _, key := range holder.Value.Get("chat").Keys() {
			counts[key]++
		}
	}
	for i, entry := range found {
		found[i].sessionID = entry.chat
		if counts[entry.chat] > 1 {
			found[i].sessionID = entry.account + "/" + entry.chat
		}
	}
	return found
}
