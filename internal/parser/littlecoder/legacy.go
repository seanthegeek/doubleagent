package littlecoder

import (
	"fmt"
	"iter"
	"unicode/utf8"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parser/pi"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// numbered yields (1-based position, item) for each item of a list, and
// nothing for any other value: the messages of a saved session, whose
// other shapes hold no message object.
func numbered(list jsonvalue.Value) iter.Seq2[int, jsonvalue.Value] {
	return func(yield func(int, jsonvalue.Value) bool) {
		for i, item := range list.Items() {
			if !yield(i+1, item) {
				return
			}
		}
	}
}

// parseLegacyHistory emits the rows of sessions/history.json, every session
// saved at exit. A file cut mid-write is walked as far as its sessions
// array parses.
func (p *Parser) parseLegacyHistory(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	text, ok := parser.ReadText(artifact.DiskPath, &problems)
	var sessions iter.Seq2[int, jsonvalue.Value]
	switch document, err := jsonvalue.DecodeString(text); {
	case !ok:
	case err == nil && document.IsObject() && document.Get("sessions").IsArray():
		sessions = numbered(document.Get("sessions"))
	default:
		if found := p.sessionsKey.FindStringIndex(text); found != nil {
			sessions = parser.ArrayAt(text, found[1], &problems, nil)
		} else {
			problems.Add(0, "history.json has no sessions array")
		}
	}
	if sessions != nil {
		for position, session := range sessions {
			if session.IsObject() {
				p.legacyRows(artifact, session, numbered(session.Get("messages")),
					parser.Text(session.Get("session_id")), parser.Text(session.Get("saved_at")), position, emit)
			}
		}
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// parseLegacySession emits the rows of one saved session file (daily,
// /save, or session_latest.json). A file cut mid-write gives the keys
// before its messages and the messages as far as the array parses.
func (p *Parser) parseLegacySession(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	text, ok := parser.ReadText(artifact.DiskPath, &problems)
	if ok {
		document, err := jsonvalue.DecodeString(text)
		switch found := p.messagesKey.FindStringIndex(text); {
		case err == nil && document.IsObject():
			p.legacyRows(artifact, document, numbered(document.Get("messages")),
				parser.Text(document.Get("session_id")), parser.Text(document.Get("saved_at")), 0, emit)
		case found != nil:
			head := text[:found[0]]
			p.legacyRows(artifact, jsonvalue.NewNull(), parser.ArrayAt(text, found[1], &problems, nil),
				firstGroup(p.sessionID.FindStringSubmatch(head)), firstGroup(p.savedAt.FindStringSubmatch(head)),
				0, emit)
		default:
			problems.Add(0, "session file is not a JSON object")
		}
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// firstGroup returns a match's first group, or "" when there is no match.
func firstGroup(match []string) string {
	if match == nil {
		return ""
	}
	return match[1]
}

// legacyRows emits a `session saved:` row, then each message's rows, all at
// the session's saved_at. sessionPosition is the session's position in
// history.json, which every row takes as its line, or 0 for a session file,
// whose rows take their message's position.
func (p *Parser) legacyRows(artifact *input.Artifact, session jsonvalue.Value,
	messages iter.Seq2[int, jsonvalue.Value], sessionID, savedAt string, sessionPosition int, emit parser.Emit) {
	timestamp := timeutil.FromText(savedAt)
	put := func(line int, turn, text, toolName, toolUseID string) {
		row := p.BaseRow(artifact)
		row.SourceLine = line
		if sessionPosition != 0 {
			row.SourceLine = sessionPosition
		}
		row.TimestampUTC, row.SessionID, row.TurnType = timestamp, sessionID, turn
		row.ToolName, row.ToolUseID, row.Text = toolName, toolUseID, timeline.Compact(text)
		emit(row)
	}
	dash := parser.SettingOrDash
	put(0, timeline.TurnSystem, "session saved: little-coder v0.0.x session "+
		dash(jsonvalue.NewString(sessionID))+" turn_count="+dash(session.Get("turn_count"))+
		" input_tokens="+dash(session.Get("total_input_tokens"))+" output_tokens="+
		dash(session.Get("total_output_tokens"))+" (saved_at "+dash(jsonvalue.NewString(savedAt))+
		", host local time)", "", "")
	for position, message := range messages {
		if message.IsObject() {
			messageRows(message, func(turn, text, toolName, toolUseID string) {
				put(position, turn, text, toolName, toolUseID)
			})
		}
	}
}

// messageRows writes, through put, the rows of one saved message by its
// role.
func messageRows(message jsonvalue.Value, put func(turn, text, toolName, toolUseID string)) {
	text := parser.TextOf(message.Get("content"))
	switch message.Get("role").StringValue() {
	case "user":
		if count, ok := imageCount(message.Get("images")); ok && count > 0 {
			if text != "" {
				text += "\n"
			}
			text += fmt.Sprintf("[%d image(s)]", count)
		}
		if text != "" {
			put(timeline.TurnUser, text, "", "")
		}
	case "assistant":
		if text != "" {
			put(timeline.TurnAssistant, text, "", "")
		}
		for _, call := range message.Get("tool_calls").Items() {
			if call.IsObject() {
				put(timeline.TurnToolUse, pi.ArgsSummary(call.Get("input")), parser.Text(call.Get("name")),
					parser.Text(call.Get("id")))
			}
		}
	case "tool":
		name, callID := parser.Text(message.Get("name")), parser.Text(message.Get("tool_call_id"))
		put(timeline.TurnToolResult, text, name, callID)
		if text == legacyDenied {
			put(timeline.TurnSystem, "permission decision: denied ("+text+")", name, callID)
		}
	}
}

// imageCount returns how many images a saved user message's images field
// counts, and false when the field is not a list, an object or a string.
// little-coder writes a list of images (research/little-coder.md); an
// object counts its members and a string its characters (runes), so a field
// of another shape still says how much it held.
func imageCount(value jsonvalue.Value) (int, bool) {
	switch {
	case value.IsString():
		return utf8.RuneCountInString(value.StringValue()), true
	case value.IsArray(), value.IsObject():
		return value.Len(), true
	}
	return 0, false
}
