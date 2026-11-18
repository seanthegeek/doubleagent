package opencode

import (
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// history emits one row per prompt of prompt-history.jsonl, the terminal
// UI's last 50, with no time, session or project, since a line records
// none: a user row, or for a shell command (mode "shell") a tool_use row
// of tool "shell" with no id, which pairs on text with the transcript's
// command. 2.0 writes the prompt as text, 1.x and Kilo Code as input.
func (p *Parser) history(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	for line, record := range parser.JSONLines(artifact.DiskPath, &problems) {
		text := record.Get("text")
		if !text.IsString() {
			text = record.Get("input")
		}
		if !text.IsString() || strings.TrimSpace(text.StringValue()) == "" {
			continue
		}
		row := p.row(artifact, newScope("", ""), "", line, "", "")
		row.TurnType = timeline.TurnUser
		if parser.IsText(record.Get("mode"), "shell") {
			row.TurnType, row.ToolName = timeline.TurnToolUse, "shell"
		}
		row.Text = timeline.Compact(text.StringValue())
		emit(row)
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// stash emits one "prompt stashed (unsent):" row per prompt set aside in
// prompt-stash.jsonl, at the line's timestamp (epoch milliseconds). 2.0
// writes {prompt: {text, ...}, timestamp}, 1.x and Kilo Code {input, parts,
// timestamp}.
func (p *Parser) stash(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	for line, record := range parser.JSONLines(artifact.DiskPath, &problems) {
		text := record.Get("input")
		if prompt := record.Get("prompt"); prompt.IsObject() {
			text = prompt.Get("text")
		}
		stripped := strings.TrimSpace(textOf(text))
		if !text.IsString() || stripped == "" {
			continue
		}
		emit(p.systemRow(artifact, newScope("", ""), line,
			timeutil.FromValue(record.Get("timestamp")),
			"prompt stashed (unsent): "+stripped+" (from "+artifact.Rel+", timestamp)"))
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// pinned emits one "session pinned:" row per id of session.json's pinned
// list, at the file's modification time. The id stays after the session is
// deleted, so a pinned session with no other row is a trace of a deleted
// one. A file that is not a JSON object is one `parser:` row.
func (p *Parser) pinned(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	document, ok := readObject(artifact.DiskPath, &problems)
	if len(problems) > 0 {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	timestamp := parser.FileMtime(artifact)
	if !ok {
		emit(p.systemRow(artifact, newScope("", ""), 1, timestamp,
			"parser: "+artifact.Rel+" is not a JSON object"))
		return
	}
	for _, identifier := range document.Get("pinned").Items() {
		if !isFilled(identifier) {
			continue
		}
		emit(p.systemRow(artifact, newScope(identifier.StringValue(), ""), 1, timestamp,
			"session pinned: "+identifier.StringValue()+" (from "+artifact.Rel+
				" pinned, file mtime)"))
	}
}

// readObject returns the file at path decoded as a JSON object, or false
// when it cannot be read (its problem added) or is not one.
func readObject(path string, problems *parser.Problems) (jsonvalue.Value, bool) {
	data, ok := parser.FileBytes(path, problems)
	if !ok {
		return jsonvalue.NewNull(), false
	}
	value, err := jsonvalue.Decode(data)
	if err != nil || !value.IsObject() {
		return jsonvalue.NewNull(), false
	}
	return value, true
}
