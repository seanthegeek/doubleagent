package parser

import (
	"strconv"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/reason"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// ProblemPrefix starts the text of every row that reports a problem with
// the evidence rather than an event of the agent.
const ProblemPrefix = "parser: "

// Problem is one problem met while reading an artifact: the line or record
// it concerns (0 for the whole file), the message, and the raw text of the
// line or record when there is one to show.
type Problem struct {
	Line    int
	Message string
	Raw     string
}

// Problems collects the problems of one parse in the order they are met.
type Problems []Problem

// Add records a problem without raw text.
func (p *Problems) Add(line int, message string) {
	*p = append(*p, Problem{Line: line, Message: message})
}

// AddRaw records a problem with the raw text of its line or record.
func (p *Problems) AddRaw(line int, message, raw string) {
	*p = append(*p, Problem{Line: line, Message: message, Raw: raw})
}

// AddUnreadable records, at line, that the file at path could not be read
// because of err: "<file name> could not be read (<reason>)".
func (p *Problems) AddUnreadable(line int, path string, err error) {
	p.Add(line, reason.Unreadable(path, err))
}

// UnparseableLine returns the message of a line that is not JSON:
// "unparseable line (<reason>)".
func UnparseableLine(err error) string {
	return "unparseable line (" + err.Error() + ")"
}

// UnparseableRecord returns the message of the record at a 1-based position
// that is not JSON: "unparseable record <position> (<reason>)".
func UnparseableRecord(position int, err error) string {
	return "unparseable record " + strconv.Itoa(position) + " (" + err.Error() + ")"
}

// ErrorRows returns one system row per problem, in list order, with the
// given session id and project path. The text is "parser: <message>: <raw>",
// or "parser: <message>" when the problem has no raw text, and the row's
// source line is the problem's line.
func (b Base) ErrorRows(artifact *input.Artifact, problems Problems, sessionID,
	projectPath string) []timeline.Row {
	rows := make([]timeline.Row, 0, len(problems))
	for _, problem := range problems {
		row := b.BaseRow(artifact)
		row.SessionID = sessionID
		row.ProjectPath = projectPath
		row.TurnType = timeline.TurnSystem
		row.SourceLine = problem.Line
		row.Text = ProblemPrefix + problem.Message
		if problem.Raw != "" {
			row.Text += ": " + problem.Raw
		}
		rows = append(rows, row)
	}
	return rows
}

// EmitAll passes each row to emit, in order.
func EmitAll(emit Emit, rows []timeline.Row) {
	for _, row := range rows {
		emit(row)
	}
}
