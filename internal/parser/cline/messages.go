package cline

import (
	"iter"
	"path"
	"regexp"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parser/clinelegacy"
	"github.com/seanthegeek/doubleagent/internal/pyjson"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// rejectionSuffix ends every rejected approval's reason the runtime writes
// (sdk/packages/shared/src/llms/tools.ts:104-105).
const rejectionSuffix = " -- NOT a tool or system failure. Clarify with user before proceeding."

// blockedPattern matches the two fixed reasons of a call the runtime skips
// without asking (sdk/packages/agents/src/agent-runtime.ts:2469, 2481).
var blockedPattern = regexp.MustCompile(
	`^(?:Tool "[^"\n]*" is disabled by policy|Tool [^\n]* was blocked by a runtime hook)\z`)

// transcript is what a messages file holds: its messages and the time of a
// message without its own.
type transcript struct {
	messages iter.Seq2[int, pyjson.Value]
	fallback string
}

// readTranscript reads a messages file: a whole object's messages array,
// or, for a file cut mid-write, its messages array walked as far as it
// parses.
func readTranscript(artifact *input.Artifact, manifest pyjson.Value, problems *parser.Problems) transcript {
	rawText, ok := parser.ReadText(artifact.DiskPath, problems)
	if !ok {
		return transcript{messages: func(func(int, pyjson.Value) bool) {}}
	}
	payload, err := pyjson.DecodeString(rawText)
	if err == nil && payload.IsObject() {
		return transcript{messages: positions(payload.Get("messages").Items()),
			fallback: timeutil.FirstNonEmpty(timeutil.FromValue(manifest.Get("started_at")),
				timeutil.FromValue(payload.Get("updated_at")))}
	}
	result := transcript{fallback: timeutil.FromValue(manifest.Get("started_at"))}
	if start, found := clinelegacy.ArrayAfterKey(rawText, "messages"); found {
		result.messages = parser.ArrayAt(rawText, start, problems, nil)
	} else {
		problems.Add(0, "messages file is not a JSON object")
		result.messages = positions(nil)
	}
	return result
}

// positions yields the items of a list with their 1-based positions.
func positions(items []pyjson.Value) iter.Seq2[int, pyjson.Value] {
	return func(yield func(int, pyjson.Value) bool) {
		for i, item := range items {
			if !yield(i+1, item) {
				return
			}
		}
	}
}

// messages emits the rows of an SDK session's messages file: a model row
// on each change, the content rows, and a permission decision after each
// result of a call the runtime skipped.
func (p *Parser) messages(artifact *input.Artifact, options parser.Options, emit parser.Emit) {
	directory := path.Dir(artifact.Rel)
	sessionID := path.Base(directory)
	var ignored, problems parser.Problems
	manifest, _ := parser.LoadJSON(artifact.Home.PathOf(directory+"/"+sessionID+".json"), &ignored)
	project := parser.Str(clinelegacy.FirstTruthy(manifest.Get("cwd"), manifest.Get("workspace_root")))
	read := readTranscript(artifact, manifest, &problems)
	seen := parser.Tracker{}
	if path.Base(artifact.Rel) == sessionID+".messages.json" && manifest.Get("model").Truthy() {
		seen[sessionID] = manifest.Get("model").Str()
	}
	fallback := read.fallback
	for position, message := range read.messages {
		if !message.IsObject() {
			continue
		}
		timestamp := timeutil.FirstNonEmpty(timeutil.FromValue(message.Get("ts")), fallback)
		fallback = timestamp
		row := p.BaseRow(artifact)
		row.SourceLine, row.TimestampUTC = position, timestamp
		row.SessionID, row.ProjectPath = sessionID, project
		p.messageRows(row, message, manifest, seen, options, emit)
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, sessionID, project))
}

// messageRows emits the rows of one message, each built from base.
func (p *Parser) messageRows(base timeline.Row, message, manifest pyjson.Value, seen parser.Tracker,
	options parser.Options, emit parser.Emit) {
	info := message.Get("modelInfo")
	base.Model = parser.Str(clinelegacy.FirstTruthy(info.Get("id"), manifest.Get("model")))
	if change := seen.Change("model", base.SessionID, parser.Str(info.Get("id"))); change != "" {
		row := base
		row.TurnType = timeline.TurnSystem
		if provider := parser.Str(info.Get("provider")); provider != "" {
			change += " provider=" + provider
		}
		row.Text = timeline.Compact(change)
		emit(row)
	}
	content := message.Get("content")
	decided := decisions(content)
	baseTurn := clinelegacy.BaseTurn(message.Get("role"))
	for _, block := range clinelegacy.ContentRows(content, baseTurn, options.IncludeThinking) {
		row := base
		row.TurnType, row.ToolName, row.ToolUseID = block.TurnType, block.ToolName, block.ToolUseID
		row.Text = timeline.Compact(block.Text)
		emit(row)
		if text, ok := decided[block.ToolUseID]; ok && block.TurnType == timeline.TurnToolResult {
			delete(decided, block.ToolUseID)
			row.TurnType, row.Text = timeline.TurnSystem, text
			emit(row)
		}
	}
}

// decisions returns, by tool call id, the "permission decision:" text of
// each skipped call in a message's content: an errored tool_result whose
// reason is a rejection or a block.
func decisions(content pyjson.Value) map[string]string {
	decided := map[string]string{}
	for _, block := range content.Items() {
		if !isText(block.Get("type"), "tool_result") || !block.Get("is_error").Truthy() {
			continue
		}
		if text := denial(block.Get("content")); text != "" {
			decided[parser.Str(block.Get("tool_use_id"))] = timeline.Compact(text)
		}
	}
	return decided
}

// denial returns the "permission decision:" text of a skipped call's
// result, whose content is {"error": <reason>}, possibly as a JSON string;
// "" for any other result.
func denial(content pyjson.Value) string {
	if content.IsString() {
		decoded, err := pyjson.DecodeString(content.Text())
		if err != nil {
			return ""
		}
		content = decoded
	}
	reason := content.Get("error")
	if !reason.IsString() {
		return ""
	}
	if text, ok := strings.CutSuffix(reason.Text(), rejectionSuffix); ok {
		return "permission decision: rejected reason=" + text
	}
	if blockedPattern.MatchString(reason.Text()) {
		return "permission decision: blocked reason=" + reason.Text()
	}
	return ""
}

// isText reports whether a value is the string text.
func isText(value pyjson.Value, text string) bool {
	return value.IsString() && value.Text() == text
}
