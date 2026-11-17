package opencode

import (
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/pyjson"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// scope holds what every row of one session shares: its id and project
// path, and the model and agent last seen in it.
type scope struct {
	sessionID   string
	projectPath string
	models      parser.Tracker
	modes       parser.Tracker
}

// newScope returns the scope of a session.
func newScope(sessionID, projectPath string) *scope {
	return &scope{sessionID: sessionID, projectPath: projectPath, models: parser.Tracker{},
		modes: parser.Tracker{}}
}

// row returns a row of the session in scope, from sourceFile ("" for the
// artifact itself) at line.
func (p *Parser) row(artifact *input.Artifact, scoped *scope, sourceFile string, line int,
	timestamp, model string) timeline.Row {
	row := p.BaseRow(artifact)
	if sourceFile != "" {
		row.SourceFile = sourceFile
	}
	row.SourceLine = line
	row.SessionID = scoped.sessionID
	row.ProjectPath = scoped.projectPath
	row.TimestampUTC = timestamp
	row.Model = model
	return row
}

// systemRow returns a system row of the artifact itself, its text compacted.
func (p *Parser) systemRow(artifact *input.Artifact, scoped *scope, line int, timestamp,
	text string) timeline.Row {
	row := p.row(artifact, scoped, "", line, timestamp, "")
	row.TurnType = timeline.TurnSystem
	row.Text = timeline.Compact(text)
	return row
}

// modelRowText returns the "model:" row text of a model reference:
// "model: <id> provider=<provider>" for a session's first model and
// "model: <old id> -> <new id> provider=<provider>" when the id or the
// provider changes; "" when it is unchanged or names no model id. Only
// string values count.
func modelRowText(seen parser.Tracker, key string, provider, model pyjson.Value) string {
	providerText, modelText := textOf(provider), textOf(model)
	if modelText == "" {
		return ""
	}
	old := seen[key]
	if seen.Change("model", key, providerText+"\n"+modelText) == "" {
		return ""
	}
	text := "model: " + modelText
	if old != "" {
		_, oldModel, _ := strings.Cut(old, "\n")
		text = "model: " + oldModel + " -> " + modelText
	}
	if providerText != "" {
		text += " provider=" + providerText
	}
	return text
}

// agentRowText returns the "permission mode: agent=<name>" text when a
// session's agent is new or changed: an OpenCode agent (build, plan, a
// custom one) is a mode with its own permission set.
func agentRowText(seen parser.Tracker, key string, agent pyjson.Value) string {
	if !isFilled(agent) {
		return ""
	}
	return seen.Change("permission mode", key, "agent="+agent.Text())
}

// lookup reads one field of a session record, a database row or a JSON
// object: the value's text as Python's str() writes it and whether Python
// counts the value as true.
type lookup func(key string) (string, bool)

// jsonLookup reads the fields of a JSON object.
func jsonLookup(record pyjson.Value) lookup {
	return func(key string) (string, bool) {
		value := record.Get(key)
		return value.Str(), value.Truthy()
	}
}

// sessionText returns the "session start:" text of a session record, with
// note at its end.
func sessionText(field lookup, note string) string {
	get := func(key string) string {
		if text, ok := field(key); ok {
			return text
		}
		return ""
	}
	tagged := func(label, key string) string {
		if text := get(key); text != "" {
			return label + text
		}
		return ""
	}
	parent := get("parent_id")
	if parent == "" {
		parent = get("parentID")
	}
	if parent != "" {
		parent = "parent=" + parent
	}
	return timeline.Compact(join("session start:", get("title"), tagged("slug=", "slug"),
		tagged("version=", "version"), tagged("agent=", "agent"), parent,
		tagged("fork=", "fork_session_id"), tagged("directory=", "directory"), note))
}

// sessionRow returns the "session start:" row of a session record.
func (p *Parser) sessionRow(artifact *input.Artifact, scoped *scope, field lookup, created string,
	line int, note string) timeline.Row {
	row := p.row(artifact, scoped, "", line, created, "")
	row.TurnType = timeline.TurnSystem
	row.Text = sessionText(field, note)
	return row
}

// failure is a problem met while reading a store, with the file it
// concerns ("" for the artifact) or the session it belongs to.
type failure struct {
	line       int
	message    string
	raw        string
	sourceFile string
	sessionID  string
}

// failureRows returns the `parser:` rows of failures, with the given
// session id and project path, each from its own source file when it
// names one.
func (p *Parser) failureRows(artifact *input.Artifact, failures []failure, sessionID,
	projectPath string) []timeline.Row {
	problems := make(parser.Problems, len(failures))
	for i, each := range failures {
		problems[i] = parser.Problem{Line: each.line, Message: each.message, Raw: each.raw}
	}
	rows := p.ErrorRows(artifact, problems, sessionID, projectPath)
	for i, each := range failures {
		if each.sourceFile != "" {
			rows[i].SourceFile = each.sourceFile
		}
	}
	return rows
}
