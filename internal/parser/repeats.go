package parser

import (
	"cmp"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// RepeatedMembers returns the problem text of the members that repeated a
// name of their JSON object, whose values replaced the earlier ones:
// "1 member name repeats within its JSON object (<name>); the last value
// is used", or "<n> member names repeat within their JSON objects (<name>
// <k> times, ...); the last value of each is used", naming the first
// names noted, with "..." when others repeated. A name is written as a
// JSON string, or as "a name of <length> bytes" when longer than
// CarriedLimit, so the text stays a fixed size and holds no part of a
// name. The value a repeat replaced is never written.
func RepeatedMembers(repeats jsonvalue.Repeats) string {
	names := repeats.Names()
	if repeats.Count() == 1 && len(names) == 1 {
		return "1 member name repeats within its JSON object (" + repeatedName(names[0].Name) +
			"); the last value is used"
	}
	shown := make([]string, 0, len(names)+1)
	listed := 0
	for _, name := range names {
		times := " times"
		if name.Times == 1 {
			times = " time"
		}
		shown = append(shown, repeatedName(name.Name)+" "+strconv.Itoa(name.Times)+times)
		listed += name.Times
	}
	if listed < repeats.Count() {
		shown = append(shown, "...")
	}
	return strconv.Itoa(repeats.Count()) + " member names repeat within their JSON objects (" +
		strings.Join(shown, ", ") + "); the last value of each is used"
}

// repeatedName returns a repeated member name as RepeatedMembers writes it.
func repeatedName(name string) string {
	if len(name) > CarriedLimit {
		return "a name of " + strconv.Itoa(len(name)) + " bytes"
	}
	return jsonvalue.NewString(name).JSON()
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
// all it was given. A parser that reads JSON outside the kit's line and
// array readers, which report their own, notes each value here.
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
