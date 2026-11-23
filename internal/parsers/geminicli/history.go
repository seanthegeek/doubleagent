package geminicli

import (
	"bytes"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// historyObjectPattern is the start of a /chat save checkpoint as the CLI
// writes it, {"history": [...], "authType": ...}, for reading the history
// array of a file cut mid-write. Only JSON whitespace counts as space.
var historyObjectPattern = regexp.MustCompile(
	`^[ \t\n\r]*\{[ \t\n\r]*"history"[ \t\n\r]*:[ \t\n\r]*`)

// shellCommand is one command of a shell_history file and the line it
// starts on.
type shellCommand struct {
	line    int
	command string
}

// shellHistoryEntries returns the commands of a shell_history file as the
// CLI's own reader reads it: lines split at LF with a CR before it removed,
// blank lines skipped, and a line after one that ends in an odd number of
// backslashes joined to it with a space, that last backslash dropped. A
// line that is not valid UTF-8 adds a problem and is skipped.
func shellHistoryEntries(data []byte, problems *parserkit.Problems, work parserkit.Work) []shellCommand {
	var out []shellCommand
	var pieces []string
	first := 0
	for i, line := range bytes.Split(data, []byte("\n")) {
		line = bytes.TrimSuffix(line, []byte("\r"))
		if !utf8.Valid(line) {
			problems.AddRaw(i+1, "unparseable line (not valid UTF-8)", parserkit.LineText(line))
			continue
		}
		text := string(line)
		if strings.TrimSpace(text) == "" {
			continue
		}
		if last := len(pieces) - 1; last >= 0 && continues(pieces[last]) {
			pieces[last] = strings.TrimSuffix(pieces[last], `\`)
			pieces = append(pieces, text)
			continue
		}
		if len(pieces) > 0 {
			out = append(out, shellCommand{first, joinCommand(pieces, work)})
		}
		pieces, first = []string{text}, i+1
	}
	if len(pieces) > 0 {
		out = append(out, shellCommand{first, joinCommand(pieces, work)})
	}
	return out
}

// joinCommand joins the lines of one command with a space, counting the
// join and its bytes in work.
func joinCommand(pieces []string, work parserkit.Work) string {
	joined := strings.Join(pieces, " ")
	work.Add("joins", 1)
	work.Add("bytes joined", len(joined))
	return joined
}

// continues reports whether a shell_history line ends in an odd number of
// backslashes, which joins the next line to it.
func continues(line string) bool {
	return (len(line)-len(strings.TrimRight(line, `\`)))%2 == 1
}

// historyItems returns the Content items of a history copy and, for a
// /chat save checkpoint written as an object, that object (else null). A
// checkpoint is {history, authType} or a legacy bare array, a /bug history
// export a bare array. A file cut mid-write keeps its complete leading
// items; a file in neither form is one problem.
func historyItems(artifact *input.Artifact, text string, problems *parserkit.Problems) ([]jsonvalue.Value, jsonvalue.Value) {
	document, err := parserkit.DecodeJSON(artifact, text)
	if err != nil {
		if strings.HasPrefix(strings.TrimLeftFunc(text, unicode.IsSpace), "[") {
			return parserkit.ArrayElements(text, problems), jsonvalue.NewNull()
		}
		if found := historyObjectPattern.FindStringIndex(text); found != nil {
			return parserkit.ArrayElements(text[found[1]:], problems), jsonvalue.NewNull()
		}
		problems.Add(1, "unparseable chat history ("+err.Error()+")")
		return nil, jsonvalue.NewNull()
	}
	if document.IsArray() {
		return document.Items(), jsonvalue.NewNull()
	}
	if document.IsObject() && document.Get("history").IsArray() {
		return document.Get("history").Items(), document
	}
	problems.Add(1, "not a chat history: neither a JSON array nor an object with history")
	return nil, jsonvalue.NewNull()
}

// shellHistory emits one user row per command of a shell_history file, a
// secondary source.
func (p *Parser) shellHistory(artifact *input.Artifact, match pathMatch, emit parserkit.Emit) {
	var problems parserkit.Problems
	project := p.resolveProject(artifact, inBase(artifact, match), match["slug"], "", &problems)
	data, _ := parserkit.FileBytes(artifact.DiskPath, &problems)
	for _, entry := range shellHistoryEntries(data, &problems, p.Work) {
		row := p.BaseRow(artifact)
		row.SourceLine = entry.line
		row.ProjectPath = project
		row.TurnType = timeline.TurnUser
		row.Text = timeline.Compact(entry.command)
		emit(row)
	}
	parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", project))
}

// historyCopy emits the header row of a /chat save checkpoint or a /bug
// history export, then the rows of its items, which carry no time.
func (p *Parser) historyCopy(artifact *input.Artifact, options parserkit.Options, match pathMatch,
	emit parserkit.Emit) {
	var problems parserkit.Problems
	project := p.resolveProject(artifact, inBase(artifact, match), match["slug"], "", &problems)
	var items []jsonvalue.Value
	document := jsonvalue.NewNull()
	if data, ok := parserkit.FileBytes(artifact.DiskPath, &problems); ok && len(data) > 0 {
		items, document = historyItems(artifact, textlines.ValidUTF8(data), &problems)
	}
	var header timeline.Row
	if tag, ok := match["tag"]; ok {
		header = standingRow(p.Base, artifact, parserkit.FileMtime(artifact), project,
			"chat checkpoint: tag "+unquote(tag)+" authType="+parserkit.TextOr(document.Get("authType"), "-")+
				" "+fromFile(artifact))
	} else {
		header = standingRow(p.Base, artifact, timeutil.FromText(match["ms"]), project,
			"bug report history: exported (from "+artifact.Rel+", time in the file name)")
	}
	header.SourceLine = 0
	emit(header)
	p.itemRows(artifact, options, items, project, "", &problems, emit)
	parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", project))
}

// itemRows emits the rows of each Content item of a history, numbered from
// 1; an item that is not an object adds a problem at its number.
func (p *Parser) itemRows(artifact *input.Artifact, options parserkit.Options, items []jsonvalue.Value,
	project, sessionID string, problems *parserkit.Problems, emit parserkit.Emit) {
	for i, item := range items {
		if !item.IsObject() {
			problems.AddRaw(i+1, "unparseable record "+strconv.Itoa(i+1)+" (not a JSON object)",
				parserkit.JSONText(item))
			continue
		}
		p.contentRows(artifact, options, item, i+1, project, sessionID, emit)
	}
}

// contentRows emits the rows of one Content item ({role, parts}) of a
// history copy, with no time: thought parts as thinking rows when asked
// for; a model item's texts as assistant rows, a user item's as user rows
// or context rows; then a tool_use row per functionCall part and a
// tool_result row per functionResponse part with its media.
func (p *Parser) contentRows(artifact *input.Artifact, options parserkit.Options, item jsonvalue.Value,
	index int, project, sessionID string, emit parserkit.Emit) {
	row := func(turn, text string) timeline.Row {
		row := p.BaseRow(artifact)
		row.SourceLine, row.SessionID, row.ProjectPath = index, sessionID, project
		row.TurnType, row.Text = turn, timeline.Compact(text)
		return row
	}
	itemParts := parts(item.Get("parts"))
	thoughts, tools := callsAndThoughts(itemParts, row)
	if options.IncludeThinking {
		for _, thought := range thoughts {
			emit(row(timeline.TurnThinking, thought))
		}
	}
	user := parserkit.IsText(item.Get("role"), "user")
	runs, responses := contentRuns(jsonvalue.NewArray(itemParts...), user)
	for _, each := range responses {
		result := row(timeline.TurnToolResult, each.resultText())
		result.ToolName, result.ToolUseID = parserkit.Text(each.value.Get("name")),
			parserkit.Text(each.value.Get("id"))
		tools = append(tools, result)
	}
	for _, each := range runs {
		emit(runRow(row, each, user))
	}
	parserkit.EmitAll(emit, tools)
}

// callsAndThoughts returns the texts of an item's thought parts and a
// tool_use row per functionCall part, its arguments as compact JSON.
func callsAndThoughts(itemParts []jsonvalue.Value, row func(turn, text string) timeline.Row) ([]string,
	[]timeline.Row) {
	var thoughts []string
	var tools []timeline.Row
	for _, part := range itemParts {
		if !part.IsObject() {
			continue
		}
		if call := part.Get("functionCall"); call.IsObject() {
			text := ""
			if !call.Get("args").IsEmpty() {
				text = parserkit.JSONText(call.Get("args"))
			}
			use := row(timeline.TurnToolUse, text)
			use.ToolName, use.ToolUseID = parserkit.Text(call.Get("name")), parserkit.Text(call.Get("id"))
			tools = append(tools, use)
		} else if part.Get("text").IsString() && !part.Get("thought").IsEmpty() {
			thoughts = append(thoughts, part.Get("text").StringValue())
		}
	}
	return thoughts, tools
}

// runRow returns the row of one run of a message's text: an assistant row
// for a model's, else a user row, or a "context: <kind>:" system row for a
// text the CLI wrote itself.
func runRow(row func(turn, text string) timeline.Row, each run, user bool) timeline.Row {
	switch {
	case !user:
		return row(timeline.TurnAssistant, each.text)
	case each.kind != "":
		return row(timeline.TurnSystem, "context: "+each.kind+": "+strings.TrimSpace(each.text))
	}
	return row(timeline.TurnUser, each.text)
}
