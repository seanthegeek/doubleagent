package continuedev

import (
	"cmp"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// devIndex returns the path of the session index of the .continue
// directory a dev_data file is in, .continue/dev_data/<file> or
// .continue/dev_data/<schema>/<file>, or "" when the kit refuses it.
func (p *Parser) devIndex(artifact *input.Artifact) string {
	levels := 2
	if parent, found := parserkit.Above(artifact, 1); found && textlines.EqualFoldASCII(filepath.Base(parent), ".continue") {
		levels = 1
	}
	index, _ := parserkit.Above(artifact, levels, "sessions", IndexName)
	return index
}

// recordModel returns a dev_data line's model title, else its model name.
func recordModel(record jsonvalue.Value) string {
	return parserkit.Text(parserkit.FirstNonEmpty(record.Get("modelTitle"), record.Get("modelName")))
}

// chatInteraction emits the user and assistant rows of each
// chatInteraction or chatFeedback line; a chatFeedback line adds "chat
// feedback: feedback=<true|false|-> model=<model|->".
func (p *Parser) chatInteraction(artifact *input.Artifact, feedback bool, emit parserkit.Emit) {
	var problems parserkit.Problems
	indexPath := p.devIndex(artifact)
	index := p.indexes.Index(indexPath)
	for line, record := range parserkit.JSONLines(artifact.DiskPath, &problems) {
		base := p.BaseRow(artifact)
		base.SourceLine = line
		base.TimestampUTC = timeutil.FromValue(record.Get("timestamp"))
		base.SessionID = parserkit.Text(record.Get("sessionId"))
		base.ProjectPath = parserkit.Text(index[base.SessionID].Get("workspaceDirectory"))
		p.indexes.Member(indexPath, base.SessionID, &base, timeline.TakeProject)
		model := recordModel(record)
		for _, turn := range [][2]string{{timeline.TurnUser, "prompt"}, {timeline.TurnAssistant, "completion"}} {
			text := record.Get(turn[1])
			if text.IsEmpty() {
				continue
			}
			row := base
			row.TurnType = turn[0]
			if turn[0] == timeline.TurnAssistant {
				row.Model = model
			}
			row.Text = timeline.Compact(parserkit.SettingValue(text))
			emit(row)
		}
		if feedback {
			row := base
			row.TurnType, row.Model = timeline.TurnSystem, model
			rating := "-"
			if value := record.Get("feedback"); !value.IsNull() {
				rating = parserkit.SettingValue(value)
			}
			row.Text = "chat feedback: feedback=" + rating + " model=" + cmp.Or(model, "-")
			emit(row)
		}
	}
	parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// editInteraction emits an edit request: an "edit interaction:" row, the
// prompt and the completion.
func (p *Parser) editInteraction(artifact *input.Artifact, emit parserkit.Emit) {
	var problems parserkit.Problems
	for line, record := range parserkit.JSONLines(artifact.DiskPath, &problems) {
		model := recordModel(record)
		file := cmp.Or(parserkit.Text(record.Get("filepath")), "-")
		parts := []struct {
			turnType string
			text     jsonvalue.Value
		}{
			{timeline.TurnSystem, jsonvalue.NewString("edit interaction: file=" + file)},
			{timeline.TurnUser, record.Get("prompt")},
			{timeline.TurnAssistant, record.Get("completion")},
		}
		for _, part := range parts {
			if part.text.IsEmpty() {
				continue
			}
			row := p.BaseRow(artifact)
			row.SourceLine = line
			row.TimestampUTC = timeutil.FromValue(record.Get("timestamp"))
			row.TurnType = part.turnType
			if part.turnType != timeline.TurnUser {
				row.Model = model
			}
			row.Text = timeline.Compact(parserkit.SettingValue(part.text))
			emit(row)
		}
	}
	parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// editOutcome emits the user's accept or reject of an edit as a
// "permission decision: edit" row. The prompt, completion and code are
// left out: the editInteraction line and the session already hold the
// request.
func (p *Parser) editOutcome(artifact *input.Artifact, emit parserkit.Emit) {
	var problems parserkit.Problems
	for line, record := range parserkit.JSONLines(artifact.DiskPath, &problems) {
		field := func(key string) string { return parserkit.SettingOrDash(record.Get(key)) }
		row := p.SystemRow(artifact, "permission decision: edit accepted="+field("accepted")+" file="+
			field("filepath")+" lines "+field("previousCodeLines")+" -> "+field("newCodeLines"))
		row.SourceLine = line
		row.TimestampUTC = timeutil.FromValue(record.Get("timestamp"))
		row.Model = recordModel(record)
		emit(row)
	}
	parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// toolUsage emits each toolUsage line's call, its result with the accepted
// and succeeded flags, and the user's decision when the line records one.
func (p *Parser) toolUsage(artifact *input.Artifact, emit parserkit.Emit) {
	var problems parserkit.Problems
	for line, record := range parserkit.JSONLines(artifact.DiskPath, &problems) {
		base := p.BaseRow(artifact)
		base.SourceLine = line
		base.TimestampUTC = timeutil.FromValue(record.Get("timestamp"))
		base.ToolName = parserkit.Text(record.Get("functionName"))
		base.ToolUseID = parserkit.Text(record.Get("toolCallId"))
		arguments := record.Get("toolCallArgs")
		if params := record.Get("functionParams"); arguments.IsEmpty() && !params.IsNull() {
			arguments = jsonvalue.NewString(parserkit.JSONText(params))
		}
		call := base
		call.TurnType = timeline.TurnToolUse
		call.Text = timeline.Compact(argumentsText(arguments))
		emit(call)
		result := base
		result.TurnType = timeline.TurnToolResult
		result.Text = timeline.Compact("accepted=" + parserkit.JSONText(record.Get("accepted")) +
			" succeeded=" + parserkit.JSONText(record.Get("succeeded")) + " " +
			outputText(record.Get("output")))
		emit(result)
		if accepted, found := record.Lookup("accepted"); found {
			decision := base
			decision.TurnType = timeline.TurnSystem
			decision.Text = "permission decision: accepted=" + parserkit.SettingValue(accepted)
			emit(decision)
		}
	}
	parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// inputHistory emits one user row per {text, timestamp} entry of the CLI's
// prompt history. The rows carry no session or project, since the file
// names none; source_line is the entry's 1-based position.
func (p *Parser) inputHistory(artifact *input.Artifact, emit parserkit.Emit) {
	var problems parserkit.Problems
	content, ok := parserkit.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	text := strings.TrimPrefix(textlines.ValidUTF8(content), byteOrderMark)
	for position, entry := range parserkit.ArrayText(text, &problems) {
		prompt := entry.Get("text")
		if !prompt.IsString() {
			problems.AddRaw(position, "unparseable record "+strconv.Itoa(position)+
				" (not an object with a text)", parserkit.JSONText(entry))
			continue
		}
		row := p.BaseRow(artifact)
		row.SourceLine = position
		row.TimestampUTC = timeutil.FromValue(entry.Get("timestamp"))
		row.TurnType = timeline.TurnUser
		row.Text = timeline.Compact(prompt.StringValue())
		emit(row)
	}
	parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}
