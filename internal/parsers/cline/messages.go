package cline

import (
	"cmp"
	"iter"
	"path"
	"regexp"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/parsers/clinelegacy"
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

// messagesArray finds the messages array of a messages file cut mid-write.
var messagesArray = clinelegacy.KeyedArray("messages")

// transcript is what a messages file holds: its messages and the time of a
// message without its own.
type transcript struct {
	messages iter.Seq2[int, jsonvalue.Value]
	fallback string
}

// readTranscript reads a messages file: a whole object's messages array,
// or, for a file cut mid-write, its messages array walked as far as it
// parses. A messages member that is not an array adds a problem.
func readTranscript(artifact *input.Artifact, manifest sessionManifest, problems *parserkit.Problems) transcript {
	rawText, ok := parserkit.ReadText(artifact.DiskPath, problems)
	if !ok {
		return transcript{messages: func(func(int, jsonvalue.Value) bool) {}}
	}
	payload, err := parserkit.DecodeJSON(artifact, rawText)
	if err == nil && payload.IsObject() {
		return transcript{messages: positions(parserkit.ArrayMember(payload.Get("messages"), "messages", 0, problems)),
			fallback: cmp.Or(timeutil.FromValue(manifest.started),
				timeutil.FromValue(payload.Get("updated_at")))}
	}
	result := transcript{fallback: timeutil.FromValue(manifest.started)}
	if start, found := clinelegacy.ArrayAfterKey(rawText, messagesArray); found {
		result.messages = parserkit.ArrayAt(rawText, start, problems, nil)
	} else {
		problems.Add(0, "messages file is not a JSON object")
		result.messages = positions(nil)
	}
	return result
}

// positions yields the items of a list with their 1-based positions.
func positions(items []jsonvalue.Value) iter.Seq2[int, jsonvalue.Value] {
	return func(yield func(int, jsonvalue.Value) bool) {
		for i, item := range items {
			if !yield(i+1, item) {
				return
			}
		}
	}
}

// sessionManifest is what a session's manifest gives each of its messages
// files: the project, the start time and the model; the zero value is a
// manifest that gives nothing.
type sessionManifest struct {
	project        string
	started, model jsonvalue.Value
}

// sessionManifest returns what the manifest of the session sessionID,
// beside the messages file, gives, read once per home by the entry its
// path opens however many messages files the session has; a manifest that
// is missing or does not parse gives nothing, and that is kept too.
func (p *Parser) sessionManifest(artifact *input.Artifact, sessionID string) sessionManifest {
	manifestPath, ok := parserkit.Beside(artifact, sessionID+".json")
	if !ok {
		return sessionManifest{}
	}
	read := p.manifests.For(artifact.Home)
	key := parserkit.KeyOf(manifestPath)
	if known, seen := read[key]; seen {
		return known
	}
	p.Work.Add("session manifests read", 1)
	var ignored parserkit.Problems
	// The manifest is parsed as its own artifact, whose parse reports its
	// repeated member names.
	manifest, _ := parserkit.LoadJSON(nil, manifestPath, &ignored)
	known := sessionManifest{project: parserkit.Text(parserkit.FirstNonEmpty(manifest.Get("cwd"),
		manifest.Get("workspace_root"))), started: manifest.Get("started_at"), model: manifest.Get("model")}
	read[key] = known
	return known
}

// messages emits the rows of an SDK session's messages file: a model row
// on each change, the content rows, and a permission decision after each
// result of a call the runtime skipped.
func (p *Parser) messages(artifact *input.Artifact, options parserkit.Options, emit parserkit.Emit) {
	directory := path.Dir(artifact.Rel)
	sessionID := path.Base(directory)
	var problems parserkit.Problems
	manifest := p.sessionManifest(artifact, sessionID)
	project := manifest.project
	read := readTranscript(artifact, manifest, &problems)
	seen := parserkit.Tracker{}
	if path.Base(artifact.Rel) == sessionID+".messages.json" && !manifest.model.IsEmpty() {
		seen[sessionID] = manifest.model.Text()
	}
	fallback := read.fallback
	for position, message := range parserkit.EntriesOfKindFrom(read.messages, "messages", jsonvalue.Object, 0, &problems) {
		timestamp := cmp.Or(timeutil.FromValue(message.Get("ts")), fallback)
		fallback = timestamp
		row := p.BaseRow(artifact)
		row.SourceLine, row.TimestampUTC = position, timestamp
		row.SessionID, row.ProjectPath = sessionID, project
		p.messageRows(artifact, row, message, manifest, seen, options, emit)
	}
	parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, sessionID, project))
}

// messageRows emits the rows of one message of artifact, each built from
// base.
func (p *Parser) messageRows(artifact *input.Artifact, base timeline.Row, message jsonvalue.Value,
	manifest sessionManifest, seen parserkit.Tracker, options parserkit.Options, emit parserkit.Emit) {
	info := message.Get("modelInfo")
	base.Model = parserkit.Text(parserkit.FirstNonEmpty(info.Get("id"), manifest.model))
	if change := seen.Change("model", base.SessionID, parserkit.Text(info.Get("id"))); change != "" {
		row := base
		row.TurnType = timeline.TurnSystem
		if provider := parserkit.Text(info.Get("provider")); provider != "" {
			change += " provider=" + provider
		}
		row.Text = timeline.Compact(change)
		emit(row)
	}
	content := message.Get("content")
	decided := decisions(artifact, content)
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
// reason is a rejection or a block. A result's repeated member names are
// noted in the parse of artifact.
func decisions(artifact *input.Artifact, content jsonvalue.Value) map[string]string {
	decided := map[string]string{}
	for _, block := range content.Items() {
		if !parserkit.IsText(block.Get("type"), "tool_result") || block.Get("is_error").IsEmpty() {
			continue
		}
		if text := denial(artifact, block.Get("content")); text != "" {
			decided[parserkit.Text(block.Get("tool_use_id"))] = timeline.Compact(text)
		}
	}
	return decided
}

// denial returns the "permission decision:" text of a skipped call's
// result, whose content is {"error": <reason>}, possibly as a JSON string;
// "" for any other result.
func denial(artifact *input.Artifact, content jsonvalue.Value) string {
	if content.IsString() {
		decoded, err := parserkit.DecodeJSON(artifact, content.StringValue())
		if err != nil {
			return ""
		}
		content = decoded
	}
	reason := content.Get("error")
	if !reason.IsString() {
		return ""
	}
	if text, ok := strings.CutSuffix(reason.StringValue(), rejectionSuffix); ok {
		return "permission decision: rejected reason=" + text
	}
	if blockedPattern.MatchString(reason.StringValue()) {
		return "permission decision: blocked reason=" + reason.StringValue()
	}
	return ""
}
