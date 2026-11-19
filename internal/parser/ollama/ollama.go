// Package ollama reads Ollama's desktop chats, the REPL prompt history and
// the launcher's auto-mode settings.
//
// It reads, relative to the home:
//
//   - Library/Application Support/Ollama/db.sqlite and
//     AppData/Local/Ollama/db.sqlite, the desktop app's database: messages,
//     tool_calls and attachments (of which only the byte length of data is
//     read). The users and settings tables are never read. created_at is
//     local time with an offset.
//   - .ollama/history, the `ollama run` REPL's line history, one entry per
//     line with no time, session or model, a line starting with / a
//     "slash command:" row; a line that is not UTF-8 is a bad line. The
//     file keeps only the last 100 entries and splits a pasted multi-line
//     prompt across lines, one entry per line.
//   - .ollama/config.json (integrations.<name>.automode) and the Claude
//     Desktop third-party profile Ollama's launcher writes
//     (autoModeEnabled), as "permission setting:" rows at the file's mtime.
//
// There is no call id: the n-th tool message after an assistant message
// pairs with that message's n-th call, and the tool_use_id is
// "<chat_id>:<tool_calls.id>"; an extra tool message takes the last call's
// name and no id, as the app's reload does. The session id is the chat id.
// See research/ollama.md.
package ollama

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"unicode/utf8"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// Parser reads Ollama's database, history and settings files.
type Parser struct {
	parser.Base
	database, history, config, profile *regexp.Regexp
}

// New returns the parser.
func New() *Parser {
	return &Parser{
		Base:     parser.NewBase("ollama", "ollama"),
		database: regexp.MustCompile(`\A(?:Library/Application Support/Ollama|AppData/Local/Ollama)/db\.sqlite\z`),
		history:  regexp.MustCompile(`\A\.ollama/history\z`),
		config:   regexp.MustCompile(`\A\.ollama/config\.json\z`),
		// The profile id is Ollama's own (claudeDesktopProfileID).
		profile: regexp.MustCompile(`\A(?:Library/Application Support|AppData/Local)/Claude-3p/` +
			`configLibrary/00000000-0000-4000-8000-000000000114\.json\z`),
	}
}

// ReadsAgents names claude-desktop: the profile Ollama writes is under a
// Claude Desktop base.
func (p *Parser) ReadsAgents() []string {
	return []string{"claude-desktop"}
}

// Wants reports whether the artifact is one of the files the parser reads.
func (p *Parser) Wants(artifact *input.Artifact) bool {
	for _, pattern := range []*regexp.Regexp{p.database, p.history, p.config, p.profile} {
		if pattern.MatchString(artifact.Rel) {
			return true
		}
	}
	return false
}

// Parse emits the rows of one Ollama file.
func (p *Parser) Parse(artifact *input.Artifact, options parser.Options, emit parser.Emit) error {
	switch {
	case p.database.MatchString(artifact.Rel):
		p.parseDatabase(artifact, options, emit)
	case p.config.MatchString(artifact.Rel):
		p.parseConfig(artifact, emit)
	case p.profile.MatchString(artifact.Rel):
		p.parseProfile(artifact, emit)
	default:
		p.parseHistory(artifact, emit)
	}
	return nil
}

// row returns a row of the artifact at a source line, session and time.
func (p *Parser) row(artifact *input.Artifact, line int, sessionID, timestamp string) timeline.Row {
	row := p.BaseRow(artifact)
	row.SourceLine = line
	row.SessionID = sessionID
	row.TimestampUTC = timestamp
	return row
}

// parseHistory emits one user or "slash command:" row per line of
// .ollama/history, then a bad-line row per line that is not UTF-8.
func (p *Parser) parseHistory(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	for number, raw := range parser.Lines(artifact.DiskPath, &problems) {
		if len(bytes.Trim(raw, textlines.ASCIIWhitespace)) == 0 {
			continue
		}
		if err := utf8Error(raw); err != nil {
			problems.AddRaw(number, "unparseable line ("+err.Error()+")", parser.LineText(raw))
			continue
		}
		line := string(bytes.Trim(raw, textlines.ASCIIWhitespace))
		row := p.row(artifact, number, "", "")
		if line[0] == '/' {
			row.TurnType = timeline.TurnSystem
			row.Text = "slash command: " + line
		} else {
			row.TurnType = timeline.TurnUser
			row.Text = line
		}
		emit(row)
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// readObject returns the JSON object of a settings file, or an error whose
// text is that of the `parser:` row that replaces its rows.
func (p *Parser) readObject(artifact *input.Artifact) (jsonvalue.Value, error) {
	var problems parser.Problems
	content, ok := parser.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		return jsonvalue.Value{}, errors.New(parser.ProblemPrefix + problems[0].Message)
	}
	err := utf8Error(content)
	var document jsonvalue.Value
	if err == nil {
		document, err = jsonvalue.Decode(content)
	}
	if err != nil {
		return jsonvalue.Value{}, fmt.Errorf("%s%s is not readable JSON (%w)", parser.ProblemPrefix, artifact.Rel, err)
	}
	if !document.IsObject() {
		return jsonvalue.Value{}, errors.New(parser.ProblemPrefix + artifact.Rel + " is not a JSON object")
	}
	return document, nil
}

// setting returns a "permission setting:" row at the file's mtime.
func (p *Parser) setting(artifact *input.Artifact, text string) timeline.Row {
	row := p.row(artifact, 0, "", parser.FileMtime(artifact))
	row.TurnType = timeline.TurnSystem
	row.Text = text
	return row
}

// parseConfig emits one row per integration of .ollama/config.json that
// sets automode; an integrations that is not an object adds a problem row.
func (p *Parser) parseConfig(artifact *input.Artifact, emit parser.Emit) {
	document, err := p.readObject(artifact)
	if err != nil {
		emit(p.SystemRow(artifact, err.Error()))
		return
	}
	var problems parser.Problems
	for name, integration := range parser.MembersOfKind(document.Get("integrations"), "integrations", jsonvalue.Object, 0,
		&problems) {
		automode, ok := integration.Lookup("automode")
		if !ok {
			continue
		}
		emit(p.setting(artifact, "permission setting: integrations."+name+".automode="+
			parser.SettingValue(automode)+" (from "+artifact.Rel+", file mtime)"))
	}
	for _, found := range problems {
		emit(p.SystemRow(artifact, parser.ProblemPrefix+found.Message))
	}
}

// parseProfile emits the autoModeEnabled row of the Claude Desktop profile.
func (p *Parser) parseProfile(artifact *input.Artifact, emit parser.Emit) {
	document, err := p.readObject(artifact)
	if err != nil {
		emit(p.SystemRow(artifact, err.Error()))
		return
	}
	if enabled, ok := document.Lookup("autoModeEnabled"); ok {
		emit(p.setting(artifact, "permission setting: autoModeEnabled="+
			parser.SettingValue(enabled)+" (set by Ollama's Claude Desktop launcher, from "+
			artifact.Rel+", file mtime)"))
	}
}

// utf8Error returns, for data that is not valid UTF-8, an error naming
// where its first invalid sequence starts: "not valid UTF-8 at byte offset
// <n>", counted from 0. It returns nil for valid UTF-8.
func utf8Error(data []byte) error {
	for start := 0; start < len(data); {
		char, size := utf8.DecodeRune(data[start:])
		if char == utf8.RuneError && size <= 1 {
			return errors.New("not valid UTF-8 at byte offset " + strconv.Itoa(start))
		}
		start += size
	}
	return nil
}
