package ollama

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"unicode/utf8"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// Parser reads Ollama's database, history and settings files.
type Parser struct {
	parserkit.Base
	database, history, config, profile *parserkit.RelPattern
}

// New returns the parser.
func New() *Parser {
	return &Parser{
		Base:     parserkit.NewBase("ollama", "ollama"),
		database: parserkit.MustRelPattern(`\A(?:Library/Application Support/Ollama|AppData/Local/Ollama)/db\.sqlite\z`),
		history:  parserkit.MustRelPattern(`\A\.ollama/history\z`),
		config:   parserkit.MustRelPattern(`\A\.ollama/config\.json\z`),
		// The profile id is Ollama's own (claudeDesktopProfileID).
		profile: parserkit.MustRelPattern(`\A(?:Library/Application Support|AppData/Local)/Claude-3p/` +
			`configLibrary/00000000-0000-4000-8000-000000000114\.json\z`),
	}
}

// ReadsAgents names claude-desktop: the profile Ollama writes is under a
// Claude Desktop base.
func (p *Parser) ReadsAgents() []string {
	return []string{"claude-desktop"}
}

// Kinds returns the app database, the CLI history, the config and the
// shell profile.
func (p *Parser) Kinds() []parserkit.Kind {
	return []parserkit.Kind{parserkit.PatternKind("database", p.database), parserkit.PatternKind("history", p.history),
		parserkit.PatternKind("config", p.config), parserkit.PatternKind("profile", p.profile)}
}

// Wants reports whether the artifact is one of the files the parser reads.
func (p *Parser) Wants(artifact *input.Artifact) bool {
	return parserkit.WantsAny(p.Kinds(), artifact)
}

// row returns a row of the artifact at a source line, session and time.
func (p *Parser) row(artifact *input.Artifact, line int, sessionID, timestamp string) timeline.Row {
	row := p.BaseRow(artifact)
	row.SourceLine = line
	row.SessionID = sessionID
	row.TimestampUTC = timestamp
	return row
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

// parseHistory emits one user or "slash command:" row per line of
// .ollama/history, then a bad-line row per line that is not UTF-8.
func (p *Parser) parseHistory(artifact *input.Artifact, emit parserkit.Emit) {
	var problems parserkit.Problems
	for number, raw := range parserkit.Lines(artifact.DiskPath, &problems) {
		if len(bytes.Trim(raw, textlines.ASCIIWhitespace)) == 0 {
			continue
		}
		if err := utf8Error(raw); err != nil {
			problems.AddRaw(number, "unparseable line ("+err.Error()+")", parserkit.LineText(raw))
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
	parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// readObject returns the JSON object of a settings file, or an error whose
// text is that of the `parser:` row that replaces its rows.
func (p *Parser) readObject(artifact *input.Artifact) (jsonvalue.Value, error) {
	var problems parserkit.Problems
	content, ok := parserkit.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		return jsonvalue.Value{}, errors.New(parserkit.ProblemPrefix + problems[0].Message)
	}
	err := utf8Error(content)
	var document jsonvalue.Value
	if err == nil {
		document, err = parserkit.DecodeJSONBytes(artifact, content)
	}
	if err != nil {
		return jsonvalue.Value{}, fmt.Errorf("%s%s is not readable JSON (%w)", parserkit.ProblemPrefix, artifact.Rel, err)
	}
	if !document.IsObject() {
		return jsonvalue.Value{}, errors.New(parserkit.ProblemPrefix + artifact.Rel + " is not a JSON object")
	}
	return document, nil
}

// setting returns a "permission setting:" row at the file's mtime.
func (p *Parser) setting(artifact *input.Artifact, text string) timeline.Row {
	row := p.row(artifact, 0, "", parserkit.FileMtime(artifact))
	row.TurnType = timeline.TurnSystem
	row.Text = text
	return row
}

// parseConfig emits one row per integration of .ollama/config.json that
// sets automode; an integrations that is not an object adds a problem row.
func (p *Parser) parseConfig(artifact *input.Artifact, emit parserkit.Emit) {
	document, err := p.readObject(artifact)
	if err != nil {
		emit(p.SystemRow(artifact, err.Error()))
		return
	}
	var problems parserkit.Problems
	for name, integration := range parserkit.MembersOfKind(document.Get("integrations"), "integrations", jsonvalue.Object, 0,
		&problems) {
		automode, ok := integration.Lookup("automode")
		if !ok {
			continue
		}
		emit(p.setting(artifact, "permission setting: integrations."+name+".automode="+
			parserkit.SettingValue(automode)+" (from "+artifact.Rel+", file mtime)"))
	}
	for _, found := range problems {
		emit(p.SystemRow(artifact, parserkit.ProblemPrefix+found.Message))
	}
}

// parseProfile emits the autoModeEnabled row of the Claude Desktop profile.
func (p *Parser) parseProfile(artifact *input.Artifact, emit parserkit.Emit) {
	document, err := p.readObject(artifact)
	if err != nil {
		emit(p.SystemRow(artifact, err.Error()))
		return
	}
	if enabled, ok := document.Lookup("autoModeEnabled"); ok {
		emit(p.setting(artifact, "permission setting: autoModeEnabled="+
			parserkit.SettingValue(enabled)+" (set by Ollama's Claude Desktop launcher, from "+
			artifact.Rel+", file mtime)"))
	}
}

// Parse emits the rows of one Ollama file.
func (p *Parser) Parse(artifact *input.Artifact, options parserkit.Options, emit parserkit.Emit) error {
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
