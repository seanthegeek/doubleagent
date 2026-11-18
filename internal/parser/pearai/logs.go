package pearai

import (
	"errors"
	"regexp"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// Each write to the output channel: two lines of 74 "=" (VsCodeExtension.ts:147-155),
// then "Settings:" ... a rule of 44 "#" ... the prompt, or "Completion:"
// (core/llm/index.ts:247-261, :586-587).
var logSeparator = regexp.MustCompile(`={74}\n={74}\n`)

// The parts of a write to the output channel.
const (
	separatorLength = 2 * 75
	logRule         = "\n\n" + ruleHashes + "\n\n"
	ruleHashes      = "############################################"
	settingsHead    = "Settings:\n"
	completionHead  = "Completion:\n"
)

// outputLog emits the rows of the PearAI output channel log: per write, a
// model row on each change and the prompt (its last user block, else the
// whole request) or the completion.
func (p *Parser) outputLog(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	raw, ok := parser.ReadText(artifact.DiskPath, &problems)
	if !ok {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	var starts []int
	for _, location := range logSeparator.FindAllStringIndex(raw, -1) {
		starts = append(starts, location[1])
	}
	preamble := raw
	if len(starts) > 0 {
		preamble = raw[:starts[0]-separatorLength]
	}
	if strings.Trim(preamble, textlines.ASCIIWhitespace) != "" {
		problems.AddRaw(1, "unparseable record (before the first separator)", strings.TrimRight(preamble, "\n"))
	}
	writes := &logWrites{owner: p, artifact: artifact, emit: emit, models: parser.Tracker{}}
	for i, start := range starts {
		end := len(raw)
		if i+1 < len(starts) {
			end = starts[i+1] - separatorLength
		}
		line := strings.Count(raw[:start], "\n") + 1
		if err := writes.record(raw[start:end], line); err != nil {
			problems.AddRaw(line, err.Error(), strings.TrimRight(raw[start:end], "\n"))
		}
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// logWrites is the state of an output log's read: the model of the last
// settings, the models seen, and whether the last prompt was a chat turn.
type logWrites struct {
	owner    *Parser
	artifact *input.Artifact
	emit     parser.Emit
	models   parser.Tracker
	model    string
	chat     bool
}

// row emits a row of the log at a line.
func (w *logWrites) row(line int, turnType, text, model string) {
	row := w.owner.BaseRow(w.artifact)
	row.SourceLine, row.TurnType, row.Model = line, turnType, model
	row.Text = timeline.Compact(text)
	w.emit(row)
}

// The reasons a write of the output log cannot be read.
var (
	errNoPromptRule = errors.New("unparseable record (no prompt rule)")
	errUnknownWrite = errors.New("unparseable record (not a Settings: or Completion: write)")
)

// record emits the rows of one write, or returns the error that says why
// it cannot be read.
func (w *logWrites) record(record string, line int) error {
	switch {
	case strings.HasPrefix(record, settingsHead):
		header, prompt, found := strings.Cut(record, logRule)
		if !found {
			return errNoPromptRule
		}
		w.model = settingsModel(header)
		if change := w.models.Change("model", "", w.model); change != "" {
			w.row(line, timeline.TurnSystem, change, w.model)
		}
		turnType, text := promptTurn(prompt)
		w.chat = turnType == timeline.TurnUser
		w.row(line, turnType, text, w.model)
	case strings.HasPrefix(record, completionHead):
		reply := strings.TrimPrefix(record, completionHead)
		if w.chat {
			w.row(line, timeline.TurnAssistant, reply, w.model)
		} else {
			w.row(line, timeline.TurnSystem, "model completion: "+timeline.Compact(reply), w.model)
		}
	default:
		return errUnknownWrite
	}
	return nil
}

// settingsModel returns the model a write's settings name: the last
// "model: <name>" line after the "Settings:" line, else "".
func settingsModel(header string) string {
	model := ""
	for _, line := range textlines.Split(header)[1:] {
		if key, value, found := strings.Cut(line, ": "); found && key == "model" {
			model = value
		}
	}
	return model
}

// devChat emits the rows of each line of dev_data/chat.jsonl: the prompt
// as promptTurn reads it, the completion, and the user's feedback.
func (p *Parser) devChat(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	var index map[string]jsonvalue.Value
	if sessions, found := parser.Above(artifact, 1, "sessions"); found {
		index = p.indexes.Index(sessions)
	}
	for line, record := range parser.JSONLines(artifact.DiskPath, &problems) {
		base := p.BaseRow(artifact)
		base.SourceLine = line
		base.SessionID = parser.Text(record.Get("sessionId"))
		base.ProjectPath = parser.Text(index[base.SessionID].Get("workspaceDirectory"))
		base.Model = parser.Text(record.Get("completionOptions").Get("model"))
		var turns [][2]string
		if prompt := record.Get("prompt"); prompt.IsString() {
			turnType, text := promptTurn(prompt.StringValue())
			turns = append(turns, [2]string{turnType, text})
		}
		if completion := record.Get("completion"); completion.IsString() {
			turns = append(turns, [2]string{timeline.TurnAssistant, completion.StringValue()})
		}
		if feedback := record.Get("feedback"); feedback.IsBool() {
			turns = append(turns, [2]string{timeline.TurnSystem,
				"user feedback: " + parser.SettingValue(feedback) + " (from dev_data/chat.jsonl)"})
		}
		for _, turn := range turns {
			row := base
			row.TurnType, row.Text = turn[0], timeline.Compact(turn[1])
			if row.Text != "" {
				emit(row)
			}
		}
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// tokens emits one "tokens generated:" row per line of
// dev_data/tokens_generated.jsonl.
func (p *Parser) tokens(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	for line, record := range parser.JSONLines(artifact.DiskPath, &problems) {
		parts := make([]string, 0, 4)
		for _, key := range []string{"model", "provider", "promptTokens", "generatedTokens"} {
			value := "-"
			if found, ok := record.Lookup(key); ok {
				value = parser.SettingValue(found)
			}
			parts = append(parts, key+"="+value)
		}
		row := p.SystemRow(artifact, "tokens generated: "+strings.Join(parts, " "))
		row.SourceLine = line
		row.Model = parser.Text(record.Get("model"))
		emit(row)
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}
