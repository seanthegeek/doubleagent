package parser

import (
	"cmp"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// RepeatedMembers returns the problem text of the members that repeated a
// name of their JSON object, whose values replaced the earlier ones
// (jsonvalue.Repeats.Describe), a name longer than CarriedLimit given by
// its length only.
func RepeatedMembers(repeats jsonvalue.Repeats) string {
	return repeats.Describe(CarriedLimit)
}

// AddRepeats adds at line the problem RepeatedMembers(repeats), a note
// (Damaged leaves it out), when a member repeated a name, and nothing
// otherwise.
func (p *Problems) AddRepeats(line int, repeats jsonvalue.Repeats) {
	if repeats.Count() > 0 {
		*p = append(*p, Problem{Line: line, Message: RepeatedMembers(repeats), note: true})
	}
}

// repeated gathers the repeated member names of the JSON values a reader
// decodes from one file (its lines or records), so the file gives one
// problem however many of its values repeat one: at the line of the first
// value that did, counting them all.
type repeated struct {
	line    int
	repeats jsonvalue.Repeats
}

// note adds the repeats of the value at line.
func (r *repeated) note(line int, repeats jsonvalue.Repeats) {
	if repeats.Count() == 0 {
		return
	}
	if r.repeats.Count() == 0 {
		r.line = line
	}
	r.repeats.Merge(repeats)
}

// report adds the problem of the repeats noted, if any, at the line of the
// first.
func (r *repeated) report(problems *Problems) {
	problems.AddRepeats(r.line, r.repeats)
}

// NoteRepeats adds repeats, what a parser's own reading of JSON in the
// evidence noted (jsonvalue.Document.TakeRepeats), to the parse of
// artifact that is running (Within), which ends with one problem row for
// all it was given. The rule every parser follows: a repeat is reported
// once, by the parse of the file that holds it. A read of another file
// during a parse (a cached lookup, a neighbour) passes no artifact when a
// parser takes that file as its own artifact, and the artifact of the
// first parse that reads it only when none does.
func NoteRepeats(artifact *input.Artifact, repeats jsonvalue.Repeats) {
	if repeats.Count() > 0 && artifact != nil && artifact.Home != nil && artifact.Home.Repeats != nil {
		artifact.Home.Repeats.Merge(repeats)
	}
}

// DecodeJSON reads text, JSON from the evidence of a parse of artifact, as
// one JSON document (jsonvalue.DecodeString) and notes the member names it
// repeats in the parse (NoteRepeats); a text that does not decode notes
// none.
func DecodeJSON(artifact *input.Artifact, text string) (jsonvalue.Value, error) {
	document := jsonvalue.NewDocument(text)
	value, err := document.Decode()
	NoteRepeats(artifact, document.TakeRepeats())
	return value, err
}

// DecodeJSONBytes reads data, raw bytes of the evidence, as DecodeJSON
// reads text.
func DecodeJSONBytes(artifact *input.Artifact, data []byte) (jsonvalue.Value, error) {
	return DecodeJSON(artifact, string(data))
}

// parseOutline is what a parse's rows have in common, for the problem row
// of the repeats it noted: the agent of its first row, and the session id
// and project path when every row gives the same, else empty.
type parseOutline struct {
	rows                        int
	agent, sessionID, project   string
	mixedSession, mixedProjects bool
}

// add takes in one row of the parse.
func (o *parseOutline) add(row timeline.Row) {
	if o.rows == 0 {
		o.agent, o.sessionID, o.project = row.Agent, row.SessionID, row.ProjectPath
	}
	o.rows++
	o.mixedSession = o.mixedSession || row.SessionID != o.sessionID
	o.mixedProjects = o.mixedProjects || row.ProjectPath != o.project
}

// repeatsRow returns the problem row (RepeatedMembers) of the repeats a
// parse of artifact noted: the parse's agent, or the artifact's when it
// gave no row, and the session id and project path all its rows share.
func (o *parseOutline) repeatsRow(artifact *input.Artifact, repeats jsonvalue.Repeats) timeline.Row {
	row := timeline.Row{Agent: cmp.Or(o.agent, artifact.Agent), SourceFile: artifact.Original,
		TurnType: timeline.TurnSystem, Text: ProblemPrefix + RepeatedMembers(repeats)}
	if artifact.Home != nil {
		row.Host, row.User = artifact.Home.Host, artifact.Home.User
	}
	if !o.mixedSession {
		row.SessionID = o.sessionID
	}
	if !o.mixedProjects {
		row.ProjectPath = o.project
	}
	return row
}
