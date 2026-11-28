package cody

import (
	"cmp"
	"regexp"
	"slices"
	"sort"
	"strconv"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
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
	problem       parserkit.Problem
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

// newRecovery returns the walk of a damaged history's text.
func newRecovery(text string) *recovery {
	return &recovery{text: text, document: jsonvalue.NewDocument(text), marks: historyMarks(text)}
}

// recovery is the walk of a damaged history. Each chat's interactions are
// read as far as they decode, which can be past the next key when an
// interaction holds a key shaped like a chat's or an account's; a key whose
// object starts inside text an earlier chat's walk has read belongs to that
// chat's interactions and starts no chat or account of its own. The walks
// therefore read each byte of the text once whatever keys it holds; walked
// and searched count the bytes they decode and search, for the tests.
type recovery struct {
	// artifact is the parse the walks note repeated member names in, once
	// for the file however many chats repeat one.
	artifact         *input.Artifact
	text             string
	document         *jsonvalue.Document
	marks            []historyMark
	walked, searched int
}

// search returns the offsets of pattern's first match in the text from
// start to end, as FindStringSubmatchIndex gives them but counted from the
// start of the text, or nil; it counts the bytes it searched.
func (r *recovery) search(pattern *regexp.Regexp, start, end int) []int {
	if start >= end {
		return nil
	}
	found := pattern.FindStringSubmatchIndex(r.text[start:end])
	if found == nil {
		r.searched += end - start
		return nil
	}
	r.searched += found[1]
	for j := range found {
		if found[j] >= 0 {
			found[j] += start
		}
	}
	return found
}

// next returns the first mark after mark i whose object starts after the
// offset read, or a mark at the end of the text when there is none.
func (r *recovery) next(i, read int) historyMark {
	later := r.marks[i+1:]
	j := sort.Search(len(later), func(j int) bool { return later[j].offset > read })
	if j == len(later) {
		return historyMark{offset: len(r.text), keyStart: len(r.text)}
	}
	return later[j]
}

// chat returns the chat of mark i: its id, the interactions that decode and
// its title when it decodes, the problems of its damaged interactions, and
// the offset its walk read up to. A damaged interaction's record runs to
// where the next key outside the walk starts, so each part of the text is
// shown once however many chats are damaged; the title is looked for
// outside the interactions the walk read.
func (r *recovery) chat(i int) (jsonvalue.Value, parserkit.Problems, int) {
	mark := r.marks[i]
	var problems parserkit.Problems
	interactions := []jsonvalue.Value{}
	walkStart, stopped := mark.offset, mark.offset
	if found := r.search(interactionsKey, mark.offset, r.next(i, mark.offset).keyStart); found != nil {
		walkStart = found[1]
		walk := &parserkit.ArrayWalk{Document: r.document, Artifact: r.artifact,
			ShownEnd: func(stopped int) int { return r.next(i, stopped).keyStart }}
		for _, value := range walk.Array(walkStart, &problems, nil) {
			if value.IsObject() {
				interactions = append(interactions, value)
			}
		}
		stopped = walk.Stopped
		r.walked += stopped - walkStart
	}
	chat := jsonvalue.NewObject(jsonvalue.Member{Key: "id", Value: jsonvalue.NewString(mark.name)},
		jsonvalue.Member{Key: "interactions", Value: jsonvalue.NewArray(interactions...)})
	found := r.search(titleKey, mark.offset, walkStart)
	if found == nil {
		found = r.search(titleKey, stopped, r.next(i, stopped).keyStart)
	}
	if found != nil {
		if title, err := jsonvalue.DecodeString(r.text[found[2]:found[3]]); err == nil {
			chat.Set("chatTitle", title)
		}
	}
	return chat, problems, stopped
}

// chats returns the history the walk recovers, how many chats it holds and
// the damaged interactions, each with its account and chat.
func (r *recovery) chats() (jsonvalue.Value, int, []recoveredError) {
	history := jsonvalue.NewObject()
	var found []recoveredError
	account, chats, stopped := "", 0, -1
	for i, mark := range r.marks {
		switch {
		case mark.offset <= stopped: // inside an earlier chat's interactions
			continue
		case !mark.isChat:
			account = mark.name
			continue
		}
		chat, problems, read := r.chat(i)
		stopped = read
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
	return history, chats, found
}

// recoveredSessions gives each damaged interaction the session id of its
// chat: the chat key, or "<account>/<chat key>" when the key is under more
// than one account.
func recoveredSessions(history jsonvalue.Value, found []recoveredError) []recoveredError {
	counts := chatKeyCounts(history)
	for i, entry := range found {
		found[i].sessionID = entry.chat
		if counts[entry.chat] > 1 {
			found[i].sessionID = entry.account + "/" + entry.chat
		}
	}
	return found
}

// recoverHistory returns the chats that can be read from a damaged
// history's text, with the interactions that decode, a message saying how
// many, and the damaged interactions. A chat is one whose key is a
// Date.toUTCString() date; it belongs to the account key before it, and
// runs to the next account or chat key outside its interactions. The
// interactions' repeated member names are noted in the parse of artifact.
func recoverHistory(artifact *input.Artifact, text string) (jsonvalue.Value, string, []recoveredError) {
	walk := newRecovery(text)
	walk.artifact = artifact
	history, chats, found := walk.chats()
	return history, "chat history did not parse; recovered " + strconv.Itoa(chats) + " chat(s)",
		recoveredSessions(history, found)
}
