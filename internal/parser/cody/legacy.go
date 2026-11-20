package cody

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// element is an XML element read so far: its tag, its value attribute and
// its child elements. Text is never read.
type element struct {
	tag      string
	value    string
	children []*element
}

// child returns the first direct child element with the tag, or nil.
func (e *element) child(tag string) *element {
	for _, each := range e.children {
		if each.tag == tag {
			return each
		}
	}
	return nil
}

// childValue returns the value attribute of the first direct child with
// the tag, or "".
func (e *element) childValue(tag string) string {
	if found := e.child(tag); found != nil {
		return found.value
	}
	return ""
}

// descendants returns the elements with the tag under e, e included, in
// document order.
func (e *element) descendants(tag string, out []*element) []*element {
	if e.tag == tag {
		out = append(out, e)
	}
	for _, each := range e.children {
		out = each.descendants(tag, out)
	}
	return out
}

// tagName returns an element name as the tag it is compared by: the local
// name, after its namespace in braces when it has one.
func tagName(name xml.Name) string {
	if name.Space == "" {
		return name.Local
	}
	return "{" + name.Space + "}" + name.Local
}

// newElement returns the element a start tag opens.
func newElement(start xml.StartElement) *element {
	created := &element{tag: tagName(start.Name)}
	for _, attribute := range start.Attr {
		if attribute.Name.Space == "" && attribute.Name.Local == "value" {
			created.value = attribute.Value
		}
	}
	return created
}

// legacyReader walks the older plugin's history as a stream: the open
// elements and the account the chats being read belong to.
type legacyReader struct {
	open    []*element
	account string
	chats   int
	onChat  func(chat *element, account string)
}

// start opens an element; an AccountData starts a new account.
func (r *legacyReader) start(token xml.StartElement) {
	opened := newElement(token)
	if len(r.open) > 0 {
		parent := r.open[len(r.open)-1]
		parent.children = append(parent.children, opened)
	}
	r.open = append(r.open, opened)
	if opened.tag == "AccountData" {
		r.account = ""
	}
}

// end closes the innermost element: an accountId directly in AccountData
// names the account, the end of AccountData forgets it, and a chat is
// passed on, then emptied.
func (r *legacyReader) end() {
	closed := r.open[len(r.open)-1]
	r.open = r.open[:len(r.open)-1]
	inAccount := len(r.open) > 0 && r.open[len(r.open)-1].tag == "AccountData"
	switch {
	case closed.tag == "accountId" && inAccount:
		r.account = closed.value
	case closed.tag == "AccountData":
		r.account = ""
	case closed.tag == "chat":
		r.chats++
		r.onChat(closed, r.account)
		closed.children = nil
	}
}

// errNoRoot and errJunk are the errors of a document with no element and
// of one with more after its element.
var (
	errNoRoot = errors.New("no root element")
	errJunk   = errors.New("content outside the root element")
)

// read walks the document, passing each chat on as it ends, and returns
// the error that stopped it, nil at a well-formed end.
func (r *legacyReader) read(data []byte) error {
	decoder := xml.NewDecoder(bytes.NewReader(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))))
	rooted := false
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) && rooted {
			return nil
		}
		if err == nil && len(r.open) == 0 {
			err = r.outside(token, rooted)
		}
		if err != nil {
			return xmlError(decoder, err)
		}
		switch token := token.(type) {
		case xml.StartElement:
			rooted = true
			r.start(token)
		case xml.EndElement:
			r.end()
		}
	}
}

// outside returns the error of a token outside the document element: a
// second element, or text that is not whitespace.
func (r *legacyReader) outside(token xml.Token, rooted bool) error {
	switch token := token.(type) {
	case xml.StartElement:
		if rooted {
			return errJunk
		}
	case xml.CharData:
		if len(bytes.Trim(token, " \t\r\n")) > 0 {
			return errJunk
		}
	}
	return nil
}

// xmlError returns the error that stopped a document's walk with the line
// and column where the decoder stood: "<message>: line <n>, column <m>".
func xmlError(decoder *xml.Decoder, err error) error {
	line, column := decoder.InputPos()
	message := err.Error()
	var syntax *xml.SyntaxError
	switch {
	case errors.As(err, &syntax):
		message = syntax.Msg
	case errors.Is(err, io.EOF):
		message = errNoRoot.Error()
	}
	return errors.New(message + ": line " + strconv.Itoa(line) + ", column " + strconv.Itoa(column))
}

// legacy emits the rows of a JetBrains project's .idea/cody_history.xml.
// The file is read as a stream, so the chats before a damaged part are
// kept, and the damage is one row saying how many were read. The account
// id an AccountData gives every chat under it is cut (parser.Carried).
func (p *Parser) legacy(artifact *input.Artifact, emit parser.Emit) {
	project := ""
	if artifact.Home != nil && artifact.Home.Project {
		project = artifact.Home.Original
	}
	var problems parser.Problems
	data, ok := parser.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", project))
		return
	}
	carried := parser.NewCarried(artifact)
	reader := &legacyReader{onChat: func(chat *element, account string) {
		p.legacyChat(artifact, chat, carried.Cut("account", account), project, emit)
	}}
	if err := reader.read(data); err != nil {
		row := p.systemRow(artifact, "parser: cody_history.xml did not parse; read "+
			strconv.Itoa(reader.chats)+" chat(s) ("+err.Error()+")", 0)
		row.ProjectPath = project
		emit(row)
	}
	carried.Report(0, &problems)
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", project))
}

// legacyChat emits the rows of one chat of the older plugin's history.
func (p *Parser) legacyChat(artifact *input.Artifact, chat *element, account, project string,
	emit parser.Emit) {
	internal := chat.childValue("internalId")
	updated := chat.childValue("updatedAt")
	if account == "" {
		account = chat.childValue("accountId")
	}
	model := legacyModel(chat)
	var messages []*element
	if holder := chat.child("messages"); holder != nil {
		messages = holder.descendants("message", nil)
	}
	sessionID := updated
	if sessionID == "" {
		sessionID = internal
	}
	timestamp := chatTime(updated, jsonvalue.NewNull())
	row := func(turnType, text, modelID string) {
		made := p.BaseRow(artifact)
		made.TimestampUTC, made.SessionID, made.ProjectPath = timestamp, sessionID, project
		made.TurnType, made.Model, made.Text = turnType, modelID, timeline.Compact(text)
		emit(made)
	}
	shownAccount := account
	if shownAccount == "" {
		shownAccount = "-"
	}
	row(timeline.TurnSystem, "chat: "+internal+" | account="+shownAccount+" | messages="+
		strconv.Itoa(len(messages))+" (legacy JetBrains history)", "")
	legacyMessages(messages, model, row)
}

// legacyModel returns a legacy chat's model: the first model under its llm
// element with a value, else its own model element's.
func legacyModel(chat *element) string {
	if llm := chat.child("llm"); llm != nil {
		for _, each := range llm.descendants("model", nil) {
			if each.value != "" {
				return each.value
			}
		}
	}
	return chat.childValue("model")
}

// legacyMessages emits the rows of a legacy chat's messages through row,
// the chat's model once before its first reply.
func legacyMessages(messages []*element, model string, row func(turnType, text, modelID string)) {
	modelShown := false
	for _, message := range messages {
		speaker, text := message.childValue("speaker"), message.childValue("text")
		switch {
		case speaker == "ASSISTANT":
			if model != "" && !modelShown {
				modelShown = true
				row(timeline.TurnSystem, "model: "+model, model)
			}
			if text != "" {
				row(timeline.TurnAssistant, text, model)
			}
		case speaker == "HUMAN":
			if text != "" {
				row(timeline.TurnUser, text, "")
			}
		case text != "":
			shown := speaker
			if shown == "" {
				shown = "-"
			}
			row(timeline.TurnSystem, strings.Join([]string{"message speaker=", shown, ": ", text}, ""), "")
		}
	}
}
