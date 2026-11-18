// Package parser is the kit every agent parser is written with: the Parser
// interface and the optional interfaces only some parsers implement, the
// options a timeline run passes, and the helpers parsers share for reading
// evidence (files, lines, JSON lines, JSON arrays, JSONC, SQLite databases)
// and for writing rows (the base row, error rows, the text of a message's
// content, settings and tracked changes).
//
// A parser owns one agent name from the catalog, says which of that agent's
// artifacts it reads (Wants), and emits timeline rows from each (Parse). The
// parsers themselves live in one package each under this one, and the
// registry that lists them in output order is internal/parsers.
//
// Every reader here opens only a regular file and never follows a symbolic
// link: it looks at the path's own entry first, so a link, a directory, a
// named pipe or a device is reported as unreadable and never opened. The
// directories above that entry were looked at when the path was built: a
// parser package builds no disk path itself (it calls no filepath.Join,
// Dir or Clean) and takes every path it opens from the artifact, from
// Beside, Above, Under or Home.PathOf, which refuse a link on the way down
// and a name that climbs, or from the Child of a Directory, a directory
// DirectoryAt found not to be a link, which refuses a name that is not one
// component. A problem met while reading is collected in a Problems list,
// which a parser turns into `parser:` rows (ErrorRows) where its output
// puts them.
package parser

import (
	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// Options are the timeline options a parser honours.
type Options struct {
	// IncludeThinking keeps the model's reasoning blocks as thinking rows.
	IncludeThinking bool
}

// Emit receives each row a parser yields, in output order.
type Emit func(row timeline.Row)

// Parser reads the artifacts of one agent.
//
// Parse emits the rows of one artifact. Rows emitted before Parse returns an
// error are kept; the error becomes the problem line
// "<original path>: <reason>" of the run, so Parse returns one only for a
// failure that stops the whole artifact (an operating-system error), and
// reports anything smaller as a `parser:` row.
type Parser interface {
	// Agent returns the catalog agent name the parser's rows carry.
	Agent() string
	// Name returns the parser's name.
	Name() string
	// Wants reports whether the parser reads the artifact, chosen by its
	// path relative to the home.
	Wants(artifact *input.Artifact) bool
	// Parse emits the timeline rows of one artifact.
	Parse(artifact *input.Artifact, options Options, emit Emit) error
}

// AgentReader is implemented by a parser that is also offered the artifacts
// of other catalog agents: an extension whose chats are rows of an editor's
// own database, which the catalog attributes to the editor.
type AgentReader interface {
	// ReadsAgents returns the other agents whose artifacts the parser is
	// offered.
	ReadsAgents() []string
}

// SecondaryReader is implemented by a parser that reads at least one
// secondary source: a per-prompt history or a mirror that repeats events of
// a store the parser also reads. A secondary source's rows are kept only
// where the primary store does not yield the same event. A parser that
// implements it has a secondary source for the session marks, whatever
// Secondary answers for a given artifact.
type SecondaryReader interface {
	// Secondary reports whether the artifact is a secondary source.
	Secondary(artifact *input.Artifact) bool
}

// ToleranceSetter is implemented by a secondary reader whose source stamps
// an event at another moment than the primary store does, and so needs a
// wider matching window than DefaultTolerance.
type ToleranceSetter interface {
	// SecondaryTolerance returns how many seconds a secondary row's time
	// may differ from its primary row's.
	SecondaryTolerance(artifact *input.Artifact) float64
}

// DefaultTolerance is the window, in seconds either side, within which a
// secondary row's time may differ from its primary row's unless the parser
// sets another; it is the deduplicator's default window.
const DefaultTolerance = 2.0

// Marks are the session marks a parser's format records. Each one set
// tells the session summary that a session of this agent without the
// mark's row is "no" ("local" for Origin) rather than unknown.
type Marks struct {
	// Sync: the parser yields a "session synced:" row for a session synced
	// to a remote account.
	Sync bool
	// Relay: the parser yields a "session relayed:" row for a session
	// carried through a messaging platform.
	Relay bool
	// Origin: the parser yields a "session origin: remote" row for a session
	// that began on the vendor's servers.
	Origin bool
}

// Marker is implemented by a parser whose format records some session
// marks.
type Marker interface {
	// Marks returns the marks the parser records.
	Marks() Marks
}

// ReadsAgentsOf returns the other agents a parser is offered, none when it
// does not implement AgentReader.
func ReadsAgentsOf(parser Parser) []string {
	if reader, ok := parser.(AgentReader); ok {
		return reader.ReadsAgents()
	}
	return nil
}

// IsSecondary reports whether the artifact is a secondary source of the
// parser; false when the parser has none.
func IsSecondary(parser Parser, artifact *input.Artifact) bool {
	reader, ok := parser.(SecondaryReader)
	return ok && reader.Secondary(artifact)
}

// HasSecondary reports whether the parser reads any secondary source.
func HasSecondary(parser Parser) bool {
	_, ok := parser.(SecondaryReader)
	return ok
}

// Tolerance returns the matching window of the parser's secondary rows from
// the artifact, DefaultTolerance unless the parser sets another.
func Tolerance(parser Parser, artifact *input.Artifact) float64 {
	if setter, ok := parser.(ToleranceSetter); ok {
		return setter.SecondaryTolerance(artifact)
	}
	return DefaultTolerance
}

// MarksOf returns the marks a parser records, none when it does not
// implement Marker.
func MarksOf(parser Parser) Marks {
	if marker, ok := parser.(Marker); ok {
		return marker.Marks()
	}
	return Marks{}
}

// Base holds a parser's agent and name and gives it the rows every parser
// starts from. A parser embeds it, so Agent and Name come with it. Work
// counts the parser's work for a test, and counts nothing unless one sets
// it.
type Base struct {
	agent string
	name  string
	Work  Work
}

// NewBase returns the Base of a parser for agent, called name.
func NewBase(agent, name string) Base {
	return Base{agent: agent, name: name}
}

// Agent returns the catalog agent name the parser's rows carry.
func (b Base) Agent() string {
	return b.agent
}

// Name returns the parser's name.
func (b Base) Name() string {
	return b.name
}

// BaseRow returns a row with the host, user, agent and source file of the
// artifact filled in.
func (b Base) BaseRow(artifact *input.Artifact) timeline.Row {
	row := timeline.Row{Agent: b.agent, SourceFile: artifact.Original}
	if artifact.Home != nil {
		row.Host = artifact.Home.Host
		row.User = artifact.Home.User
	}
	return row
}

// SystemRow returns a system row of the artifact with the given text and
// nothing else set beyond the base row.
func (b Base) SystemRow(artifact *input.Artifact, text string) timeline.Row {
	row := b.BaseRow(artifact)
	row.TurnType = timeline.TurnSystem
	row.Text = text
	return row
}

// ProblemRow returns the system row "parser: <message>" for a problem that
// concerns the whole artifact.
func (b Base) ProblemRow(artifact *input.Artifact, message string) timeline.Row {
	return b.SystemRow(artifact, ProblemPrefix+message)
}
