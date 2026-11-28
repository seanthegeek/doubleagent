package cody

import (
	"strconv"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// typedClaims holds the human messages of one scope's chats, in chat
// order, that an entry of the scope's input list can be a copy of.
type typedClaims struct {
	// waiting maps a text to the messages whose text or displayText it is,
	// so finding an entry's message costs the same however many there are.
	waiting map[string][]int
	taken   []bool
	work    parserkit.Work // counts each message looked at, for the tests of cost
}

// add records a human message under its text and, when it differs, its
// displayText, the two forms in which a chat keeps what was typed.
func (c *typedClaims) add(message jsonvalue.Value) {
	if c.waiting == nil {
		c.waiting = map[string][]int{}
	}
	index := len(c.taken)
	c.taken = append(c.taken, false)
	text, display := message.Get("text"), message.Get("displayText")
	if text.IsString() {
		c.waiting[text.StringValue()] = append(c.waiting[text.StringValue()], index)
	}
	if display.IsString() && (!text.IsString() || display.StringValue() != text.StringValue()) {
		c.waiting[display.StringValue()] = append(c.waiting[display.StringValue()], index)
	}
}

// take reports whether a message not yet taken holds text, taking the first
// in chat order; a message passed over was taken under its other form.
func (c *typedClaims) take(text string) bool {
	queue := c.waiting[text]
	for len(queue) > 0 {
		index := queue[0]
		queue = queue[1:]
		c.work.Add("typed claims looked at", 1)
		if !c.taken[index] {
			c.taken[index] = true
			c.waiting[text] = queue
			return true
		}
	}
	delete(c.waiting, text)
	return false
}

// typedEntry returns the text of an input entry and its row text: a string
// as it is, or an object's inputText with the files of its
// inputContextFiles (withContext); ok is false for an entry of another kind.
func typedEntry(entry jsonvalue.Value, tally *parserkit.EntryTally) (text, rowText string, ok bool) {
	switch {
	case entry.IsString():
		return entry.StringValue(), entry.StringValue(), true
	case entry.IsObject():
		inputText := entry.Get("inputText")
		tally.Check(inputText, "inputText", jsonvalue.String)
		text = inputText.StringValue()
		return text, withContext(text, tally.ArrayMember(entry.Get("inputContextFiles"), "inputContextFiles")), true
	}
	return "", "", false
}

// typedRows emits the rows of a scope's input list, the prompts typed into
// the chat box, after the scope's chats: a heading, then a recovered user
// row with no time or session for each entry with text that is no copy of
// a chat's message. An entry is a copy when a human message of a chat in the
// same scope, not yet taken by an earlier entry of the list, has a text or
// displayText equal to the entry's text, and it takes the first such
// message in chat order. An entry of another kind is counted in the tally.
func (p *Parser) typedRows(artifact *input.Artifact, list jsonvalue.Value, keyed bool, account string,
	claims *typedClaims, line int, emit parserkit.Emit, tally *parserkit.EntryTally) {
	entries := tally.ArrayMember(list, "input")
	if len(entries) == 0 {
		return
	}
	heading := "typed prompts: entries=" + strconv.Itoa(len(entries))
	if keyed {
		heading = "typed prompts: account=" + account + " | entries=" + strconv.Itoa(len(entries))
	}
	emit(p.systemRow(artifact, heading, line))
	var others []jsonvalue.Value
	for _, entry := range entries {
		text, rowText, ok := typedEntry(entry, tally)
		if !ok {
			others = append(others, entry)
			continue
		}
		if timeline.Compact(text) == "" || claims.take(text) {
			continue
		}
		row := p.BaseRow(artifact)
		row.SourceLine = line
		row.TurnType = timeline.TurnUser
		row.Text = timeline.Compact(rowText)
		row.Recovered = "yes"
		emit(row)
	}
	if len(others) > 0 { // every one of them is the wrong kind, so the tally counts them all
		tally.EntriesOfKind(jsonvalue.NewArray(others...), "input", jsonvalue.String)
	}
}
