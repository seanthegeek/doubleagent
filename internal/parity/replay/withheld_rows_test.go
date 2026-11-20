//go:build parity

package replay

import (
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parser/agentzero"
	"github.com/seanthegeek/doubleagent/internal/parser/crush"
	"github.com/seanthegeek/doubleagent/internal/parser/letta"
	"github.com/seanthegeek/doubleagent/internal/parser/littlecoder"
	"github.com/seanthegeek/doubleagent/internal/parser/nanobot"
	"github.com/seanthegeek/doubleagent/internal/parser/openclaw"
	"github.com/seanthegeek/doubleagent/internal/parser/shellgpt"
	"github.com/seanthegeek/doubleagent/internal/parser/tabby"
	"github.com/seanthegeek/doubleagent/internal/parser/twinny"
	"github.com/seanthegeek/doubleagent/internal/parsers"
	"github.com/seanthegeek/doubleagent/internal/secrets"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// A command's outputs count every row a parse gives (inventory.json, the
// summary on stdout), so a row of withheldRows that the recording never
// gave would change every count of a case whose tree holds its file. The
// command replay therefore takes such a row at the parse that gives it,
// under difference 231's own rule (parsedFile.matches, the whole row),
// before the run counts and writes its rows; the parser replay compares
// the row itself.

// addedRowFilter takes, from each parse of a command run, the first row of
// withheldRows that the parse gives and the rows of difference 250
// (socketTokenRows), and counts the rows it took of each.
type addedRowFilter struct {
	taken, socketTokens int
}

// emit returns the emit of one parse of artifact by agent's parser: it
// passes each row on but, for each entry of withheldRows for agent and
// each row of socketTokenRows, the first row that is the whole row the
// entry lets the parse add.
func (f *addedRowFilter) emit(agent string, artifact *input.Artifact, emit parser.Emit) parser.Emit {
	parse := parsedFile{agent: agent, file: artifact.Original, disk: artifact.DiskPath, rel: artifact.Rel,
		home: artifact.Home, mtime: parser.FileMtime(artifact)}
	var waiting []func(Row) bool
	for _, entry := range withheldRows() {
		if entry.agent == agent {
			waiting = append(waiting, parse.matches(entry.text, entry.row, nil))
		}
	}
	withheld := len(waiting)
	waiting = append(waiting, socketTokenRows(parse)...)
	recorder := recorderOf(artifact)
	return func(row timeline.Row) {
		// The row is compared as the output writes it (timeline.Row.Withheld:
		// its text by RowText with the parse's record so far, every other
		// field by Text).
		written := row
		written.Secrets = recorder.Current()
		candidate := replayRow(written.Withheld(recorder))
		for i, matches := range waiting {
			if matches(candidate) {
				waiting = slices.Delete(waiting, i, i+1)
				if i < withheld {
					withheld--
					f.taken++
				} else {
					f.socketTokens++
				}
				return
			}
		}
		emit(row)
	}
}

// recorderOf returns the recorder of the artifact's home, nil for none.
func recorderOf(artifact *input.Artifact) *secrets.Recorder {
	if artifact.Home == nil {
		return nil
	}
	return artifact.Home.Secrets
}

// withoutAddedRows returns list with the parser of each agent that has an
// entry of withheldRows, and OpenClaw's for difference 250, in place of one
// whose parses pass through filter.
// Each stand-in embeds the parser's own type, so every optional interface
// the parser implements is still there.
func withoutAddedRows(list []parser.Parser, filter *addedRowFilter) []parser.Parser {
	if filter == nil {
		return list
	}
	out := slices.Clone(list)
	for i, each := range out {
		switch concrete := each.(type) {
		case *agentzero.Parser:
			out[i] = agentZeroRows{concrete, filter}
		case *tabby.Parser:
			out[i] = tabbyRows{concrete, filter}
		case *twinny.Parser:
			out[i] = twinnyRows{concrete, filter}
		case *letta.Parser:
			out[i] = lettaRows{concrete, filter}
		case *shellgpt.Parser:
			out[i] = shellgptRows{concrete, filter}
		case *crush.Parser:
			out[i] = crushRows{concrete, filter}
		case *nanobot.Parser:
			out[i] = nanobotRows{concrete, filter}
		case *littlecoder.Parser:
			out[i] = littleCoderRows{concrete, filter}
		case *openclaw.Parser:
			out[i] = openClawRows{concrete, filter}
		}
	}
	return out
}

// agentZeroRows is the Agent Zero parser with its parses passed through an
// addedRowFilter.
type agentZeroRows struct {
	*agentzero.Parser
	filter *addedRowFilter
}

// Parse parses the artifact through the filter.
func (p agentZeroRows) Parse(artifact *input.Artifact, options parser.Options, emit parser.Emit) error {
	return p.Parser.Parse(artifact, options, p.filter.emit(p.Agent(), artifact, emit))
}

// tabbyRows is the Tabby parser with its parses passed through an
// addedRowFilter.
type tabbyRows struct {
	*tabby.Parser
	filter *addedRowFilter
}

// Parse parses the artifact through the filter.
func (p tabbyRows) Parse(artifact *input.Artifact, options parser.Options, emit parser.Emit) error {
	return p.Parser.Parse(artifact, options, p.filter.emit(p.Agent(), artifact, emit))
}

// twinnyRows is the Twinny parser with its parses passed through an
// addedRowFilter.
type twinnyRows struct {
	*twinny.Parser
	filter *addedRowFilter
}

// Parse parses the artifact through the filter.
func (p twinnyRows) Parse(artifact *input.Artifact, options parser.Options, emit parser.Emit) error {
	return p.Parser.Parse(artifact, options, p.filter.emit(p.Agent(), artifact, emit))
}

// lettaRows is the Letta parser with its parses passed through an
// addedRowFilter.
type lettaRows struct {
	*letta.Parser
	filter *addedRowFilter
}

// Parse parses the artifact through the filter.
func (p lettaRows) Parse(artifact *input.Artifact, options parser.Options, emit parser.Emit) error {
	return p.Parser.Parse(artifact, options, p.filter.emit(p.Agent(), artifact, emit))
}

// shellgptRows is the ShellGPT parser with its parses passed through an
// addedRowFilter.
type shellgptRows struct {
	*shellgpt.Parser
	filter *addedRowFilter
}

// Parse parses the artifact through the filter.
func (p shellgptRows) Parse(artifact *input.Artifact, options parser.Options, emit parser.Emit) error {
	return p.Parser.Parse(artifact, options, p.filter.emit(p.Agent(), artifact, emit))
}

// crushRows is the Crush parser with its parses passed through an
// addedRowFilter.
type crushRows struct {
	*crush.Parser
	filter *addedRowFilter
}

// Parse parses the artifact through the filter.
func (p crushRows) Parse(artifact *input.Artifact, options parser.Options, emit parser.Emit) error {
	return p.Parser.Parse(artifact, options, p.filter.emit(p.Agent(), artifact, emit))
}

// nanobotRows is the nanobot parser with its parses passed through an
// addedRowFilter.
type nanobotRows struct {
	*nanobot.Parser
	filter *addedRowFilter
}

// Parse parses the artifact through the filter.
func (p nanobotRows) Parse(artifact *input.Artifact, options parser.Options, emit parser.Emit) error {
	return p.Parser.Parse(artifact, options, p.filter.emit(p.Agent(), artifact, emit))
}

// littleCoderRows is the little-coder parser with its parses passed through an
// addedRowFilter.
type littleCoderRows struct {
	*littlecoder.Parser
	filter *addedRowFilter
}

// Parse parses the artifact through the filter.
func (p littleCoderRows) Parse(artifact *input.Artifact, options parser.Options, emit parser.Emit) error {
	return p.Parser.Parse(artifact, options, p.filter.emit(p.Agent(), artifact, emit))
}

// openClawRows is the OpenClaw parser with its parses passed through an
// addedRowFilter, for the socket token rows of difference 250.
type openClawRows struct {
	*openclaw.Parser
	filter *addedRowFilter
}

// Parse parses the artifact through the filter.
func (p openClawRows) Parse(artifact *input.Artifact, options parser.Options, emit parser.Emit) error {
	return p.Parser.Parse(artifact, options, p.filter.emit(p.Agent(), artifact, emit))
}

// wholeAddedRow returns the row an entry of withheldRows lets a parse of
// artifact add, as its parser emits it: the artifact's file is the users
// database under testdata, whose first user a table time names.
func wholeAddedRow(entry withheldRow, artifact *input.Artifact) timeline.Row {
	row := timeline.Row{TurnType: timeline.TurnSystem, Agent: entry.agent, SourceFile: artifact.Original,
		Host: artifact.Home.Host, User: artifact.Home.User, Text: entry.text, SourceLine: entry.row.line,
		TimestampUTC: entry.row.time, SessionID: entry.row.session, ProjectPath: entry.row.project}
	switch {
	case entry.row.cell != tableCell{}:
		row.TimestampUTC = "2026-09-01T09:00:00.000Z"
	case entry.row.time == fileTime:
		row.TimestampUTC = parser.FileMtime(artifact)
	}
	if entry.row.project == homePath {
		row.ProjectPath = artifact.Home.Original
	}
	return row
}

// usersArtifact returns an artifact at original whose file on disk is the
// users database under testdata, in a home of bob on mini.local.
func usersArtifact(original string) *input.Artifact {
	return &input.Artifact{Original: original, DiskPath: filepath.Join("testdata", "tabby", "users.db"),
		Rel: strings.TrimPrefix(original, "/h/"), Manifest: jsonvalue.NewNull(),
		Home: &input.Home{Original: "/h", User: "bob", Host: "mini.local"}}
}

func TestEveryAgentOfAWithheldRowHasAStandInThatTakesIt(t *testing.T) {
	registered := parsers.ByAgent(parsers.All())
	for _, entry := range withheldRows() {
		t.Run(entry.label(), func(t *testing.T) {
			filter := &addedRowFilter{}
			standIns := parsers.ByAgent(withoutAddedRows(parsers.All(), filter))[entry.agent]
			if len(standIns) != 1 || fmt.Sprintf("%T", standIns[0]) == fmt.Sprintf("%T", registered[entry.agent][0]) {
				t.Fatalf("the agent's parsers are %T, not one stand-in", standIns)
			}
			artifact := usersArtifact("/h/f")
			var kept []timeline.Row
			emit := filter.emit(entry.agent, artifact, func(row timeline.Row) { kept = append(kept, row) })
			row := wholeAddedRow(entry, artifact)
			other := row
			other.SourceFile = "/h/g"
			for _, each := range []timeline.Row{other, row, row} {
				emit(each)
			}
			if !slices.Equal(kept, []timeline.Row{other, row}) || filter.taken != 1 {
				t.Errorf("kept %v and took %d, want the other file's row and the second", kept, filter.taken)
			}
		})
	}
}

func TestACommandRunKeepsAnAddedRowThatDiffersInAnyField(t *testing.T) {
	var entry withheldRow
	for _, each := range withheldRows() {
		if each.label() == "crush provider API key" {
			entry = each
		}
	}
	artifact := usersArtifact("/h/.crush/crush.json")
	whole := wholeAddedRow(entry, artifact)
	type rowCase struct {
		name  string
		row   timeline.Row
		taken bool
	}
	tests := []rowCase{{"the whole row is taken", whole, true}}
	// Every column of a row, each field the output writes, is changed in
	// turn, so a column added to the row type is changed too.
	columns := reflect.TypeFor[timeline.Row]()
	for i := range columns.NumField() {
		if columns.Field(i).Tag.Get("json") == "-" {
			continue
		}
		row := whole
		value := reflect.ValueOf(&row).Elem().Field(i)
		switch value.Kind() {
		case reflect.String:
			value.SetString(value.String() + "-changed")
		case reflect.Int:
			value.SetInt(value.Int() + 99)
		default:
			t.Fatalf("no change is written for the column %s of kind %s", columns.Field(i).Name, value.Kind())
		}
		tests = append(tests, rowCase{"a row with another " + columns.Field(i).Name + " is kept", row, false})
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			filter := &addedRowFilter{}
			var kept []timeline.Row
			filter.emit(entry.agent, artifact, func(row timeline.Row) { kept = append(kept, row) })(test.row)
			if taken := filter.taken == 1 && len(kept) == 0; taken != test.taken || filter.taken+len(kept) != 1 {
				t.Errorf("took %d and kept %d, want taken %v", filter.taken, len(kept), test.taken)
			}
		})
	}
}
