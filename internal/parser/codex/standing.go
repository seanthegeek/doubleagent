package codex

import (
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
	"github.com/seanthegeek/doubleagent/internal/tomldoc"
)

// systemRow returns a system row of the artifact for the session at the
// line, with text as it is.
func (p *Parser) systemRow(artifact *input.Artifact, text, sessionID string, line int) timeline.Row {
	row := p.SystemRow(artifact, text)
	row.SessionID = sessionID
	row.SourceLine = line
	return row
}

// homeProject returns the project a project's own file belongs to: the
// home's original path when the home is a project, else "".
func homeProject(artifact *input.Artifact) string {
	if artifact.Home != nil && artifact.Home.Project {
		return artifact.Home.Original
	}
	return ""
}

// parseHistory emits a user row per line of the prompt history.
func (p *Parser) parseHistory(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	for line, record := range parser.JSONLines(artifact.DiskPath, &problems) {
		row := p.BaseRow(artifact)
		row.SourceLine = line
		row.TimestampUTC = timeutil.FromValue(record.Get("ts"))
		row.SessionID = parser.Str(record.Get("session_id"))
		row.TurnType = timeline.TurnUser
		row.Text = timeline.CompactValue(record.Get("text"))
		emit(row)
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// parseLedger emits a system row per record of a fork's import ledger, at
// the import time, under the imported thread. The row's source line is the
// record's position in the list, not a file line.
func (p *Parser) parseLedger(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	content, ok := parser.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	data, err := jsonvalue.Decode(content)
	if err != nil {
		emit(p.ProblemRow(artifact, "unreadable import ledger: "+err.Error()))
		return
	}
	records := data.Get("records")
	if !records.IsArray() {
		emit(p.ProblemRow(artifact, "import ledger has no records list"))
		return
	}
	for i, record := range records.Items() {
		if !record.IsObject() {
			continue
		}
		text := "imported session from " + parser.Str(record.Get("source_path")) + " sha256=" +
			parser.Str(record.Get("content_sha256"))
		if modified := timeutil.FromValue(record.Get("source_modified_at")); modified != "" {
			text += " source_modified=" + modified
		}
		row := p.systemRow(artifact, timeline.Compact(text),
			parser.Str(record.Get("imported_thread_id")), i+1)
		row.TimestampUTC = timeutil.FromValue(record.Get("imported_at"))
		emit(row)
	}
}

// parseAutomation emits the "scheduled task:" row of a desktop app
// automation at the file's modification time, under its target thread.
func (p *Parser) parseAutomation(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	content, ok := parser.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	document, err := tomldoc.Parse(textlines.ValidUTF8(content))
	if err != nil {
		emit(p.ProblemRow(artifact, "unreadable automation.toml: "+err.Error()))
		return
	}
	field := func(key string) string {
		value, ok := document.Get(key)
		if !ok {
			return ""
		}
		return tomlStr(value)
	}
	text := "scheduled task: " + field("id") + " name=" + field("name") + " kind=" +
		field("kind") + " status=" + field("status") + " rrule=" + field("rrule") +
		" target_thread=" + field("target_thread_id") + " created=" +
		tomlTime(document, "created_at") + " updated=" + tomlTime(document, "updated_at") +
		" (from " + artifact.Rel + ", file mtime) prompt: " + field("prompt")
	row := p.systemRow(artifact, text, field("target_thread_id"), 1)
	row.TimestampUTC = parser.FileMtime(artifact)
	emit(row)
}

// tomlStr returns a TOML value as Python's str() writes the value tomllib
// gives: a string as it is, a number, True or False, a date or time as
// Python writes it, and an array or table as Python writes a list or dict.
func tomlStr(value tomldoc.Value) string {
	switch value.Kind() {
	case tomldoc.KindInteger:
		return strconv.FormatInt(value.Int(), 10)
	case tomldoc.KindFloat:
		return strconv.FormatFloat(value.Float(), 'g', -1, 64)
	case tomldoc.KindBoolean:
		return strconv.FormatBool(value.Bool())
	case tomldoc.KindArray, tomldoc.KindTable:
		return tomlJSON(value).JSON()
	}
	return value.Text()
}

// tomlJSON returns a TOML value as the decoded JSON value of the same
// shape, a date or time as its text.
func tomlJSON(value tomldoc.Value) jsonvalue.Value {
	switch value.Kind() {
	case tomldoc.KindInteger:
		return jsonvalue.NewInt(value.Int())
	case tomldoc.KindFloat:
		return jsonvalue.NewFloat(value.Float())
	case tomldoc.KindBoolean:
		return jsonvalue.NewBool(value.Bool())
	case tomldoc.KindArray:
		items := make([]jsonvalue.Value, 0, len(value.Items()))
		for _, item := range value.Items() {
			items = append(items, tomlJSON(item))
		}
		return jsonvalue.NewArray(items...)
	case tomldoc.KindTable:
		object := jsonvalue.NewObject()
		for _, entry := range value.Table().Entries() {
			object.Set(entry.Key, tomlJSON(entry.Value))
		}
		return object
	}
	return jsonvalue.NewString(value.Text())
}

// tomlTime returns the timestamp of a table's key: an integer or float as
// an epoch number, a string or a date and time as text, else "".
func tomlTime(table *tomldoc.Table, key string) string {
	value, ok := table.Get(key)
	if !ok {
		return ""
	}
	switch value.Kind() {
	case tomldoc.KindInteger:
		return timeutil.FromInt(value.Int())
	case tomldoc.KindFloat:
		return timeutil.FromEpoch(value.Float())
	case tomldoc.KindBoolean, tomldoc.KindArray, tomldoc.KindTable:
		return ""
	}
	return timeutil.FromText(value.Text())
}

// setting returns a standing "permission setting:" row at the file's
// modification time.
func (p *Parser) setting(artifact *input.Artifact, text string, line int, project string) timeline.Row {
	row := p.systemRow(artifact, timeline.Compact("permission setting: "+text+" (from "+
		artifact.Rel+", file mtime)"), "", line)
	row.TimestampUTC = parser.FileMtime(artifact)
	row.ProjectPath = project
	return row
}

// parseConfig emits a "permission setting:" row per standing permission key
// of config.toml. A project's trust level carries the project, and a
// project's own .codex/config.toml the project.
func (p *Parser) parseConfig(artifact *input.Artifact, emit parser.Emit) {
	base := homeProject(artifact)
	var problems parser.Problems
	content, ok := parser.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", base))
		return
	}
	document, err := tomldoc.Parse(textlines.ValidUTF8(content))
	if err != nil {
		emit(p.ProblemRow(artifact, "unreadable config.toml: "+err.Error()))
		return
	}
	for _, found := range configSettings(document) {
		project := base
		if found.parts[0] == "projects" {
			project = found.parts[1]
		}
		text := strings.Join(found.parts, ".") + "=" + tomldoc.SettingText(found.value)
		emit(p.setting(artifact, text, 0, project))
	}
}

// configPermissionPaths are the standing permission keys of config.toml,
// as dotted paths with "*" for any one table name (research/codex-cli.md,
// Standing grants and approval records): the policy defaults, a project's
// trust level, and the approval_mode an "approve for good" answer writes
// for a tool.
var configPermissionPaths = [][]string{
	{"approval_policy"},
	{"approvals_reviewer"},
	{"sandbox_mode"},
	{"default_permissions"},
	{"projects", "*", "trust_level"},
	{"mcp_servers", "*", "tools", "*", "approval_mode"},
	{"plugins", "*", "mcp_servers", "*", "tools", "*", "approval_mode"},
	{"apps", "*", "tools", "*", "approval_mode"},
}

// configSetting is one standing key found in config.toml: its key parts
// and its value.
type configSetting struct {
	parts []string
	value tomldoc.Value
}

// configSettings returns every configPermissionPaths entry the document
// holds, path by path in that order, tables in document order. A key that
// holds a table is not a setting.
func configSettings(document *tomldoc.Table) []configSetting {
	var found []configSetting
	for _, path := range configPermissionPaths {
		found = walkSettings(found, document, path, nil)
	}
	return found
}

// walkSettings adds to found the settings under table that path names
// from it, parts being the keys walked so far.
func walkSettings(found []configSetting, table *tomldoc.Table, path, parts []string) []configSetting {
	visit := func(key string, node tomldoc.Value) {
		keys := append(parts[:len(parts):len(parts)], key)
		switch {
		case len(path) == 1 && node.Kind() != tomldoc.KindTable:
			found = append(found, configSetting{parts: keys, value: node})
		case len(path) > 1 && node.Kind() == tomldoc.KindTable:
			found = walkSettings(found, node.Table(), path[1:], keys)
		}
	}
	if path[0] != "*" {
		if node, ok := table.Get(path[0]); ok {
			visit(path[0], node)
		}
		return found
	}
	for _, entry := range table.Entries() {
		visit(entry.Key, entry.Value)
	}
	return found
}

// parseRules emits a "permission setting:" row per statement of a rules/
// exec policy file, then a problem row per statement left open.
func (p *Parser) parseRules(artifact *input.Artifact, emit parser.Emit) {
	base := homeProject(artifact)
	var problems parser.Problems
	content, ok := parser.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", base))
		return
	}
	for _, statement := range ruleStatements(textlines.ValidUTF8(content), &problems) {
		emit(p.setting(artifact, statement.text, statement.line, base))
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", base))
}

// statement is one top-level statement of an exec policy file: its first
// line and its text joined onto one line.
type statement struct {
	line int
	text string
}

// ruleStatements returns each top-level statement of a Starlark exec
// policy file (prefix_rule(...), network_rule(...) or any other call),
// joined onto one line without its comments. A statement still open at the
// end of the file (an unclosed bracket or string) adds the problem
// "unterminated rule" with its lines as written.
func ruleStatements(text string, problems *parser.Problems) []statement {
	var out []statement
	var scan ruleScanner
	start := 0
	var parts, raw []string
	for i, line := range textlines.Split(text) {
		kept := strings.Trim(scan.line(line), textlines.ASCIISpace)
		if kept != "" || len(parts) > 0 {
			if len(parts) == 0 {
				start = i + 1
			}
			parts = append(parts, kept)
			raw = append(raw, line)
		}
		if len(parts) > 0 && scan.depth <= 0 && scan.quote == 0 {
			out = append(out, statement{line: start, text: joinNonEmpty(parts)})
			parts, raw, scan.depth = nil, nil, 0
		}
	}
	if len(parts) > 0 {
		problems.AddRaw(start, "unterminated rule", strings.Join(raw, "\n"))
	}
	return out
}

// joinNonEmpty joins the parts that are not empty with spaces.
func joinNonEmpty(parts []string) string {
	var kept []string
	for _, part := range parts {
		if part != "" {
			kept = append(kept, part)
		}
	}
	return strings.Join(kept, " ")
}

// ruleScanner follows the brackets and strings of a policy file from line
// to line.
type ruleScanner struct {
	depth int
	quote byte
}

// line returns a line less its comment, and updates the bracket depth and
// the open string. Inside a string a backslash keeps the character after it.
func (s *ruleScanner) line(line string) string {
	var kept strings.Builder
	for i := 0; i < len(line); i++ {
		character := line[i]
		switch {
		case s.quote != 0 && character == '\\':
			end := min(i+2, len(line))
			kept.WriteString(line[i:end])
			i = end - 1
			continue
		case s.quote != 0:
			if character == s.quote {
				s.quote = 0
			}
		case character == '#':
			return kept.String()
		case character == '"' || character == '\'':
			s.quote = character
		case strings.IndexByte("([{", character) >= 0:
			s.depth++
		case strings.IndexByte(")]}", character) >= 0:
			s.depth--
		}
		kept.WriteByte(character)
	}
	return kept.String()
}

// memoryHeaderKeys are the header keys of a memory summary file.
var memoryHeaderKeys = []string{"thread_id", "updated_at", "rollout_path", "cwd", "git_branch"}

// memorySummary returns the header and the summary of a
// memories/rollout_summaries/<stem>.md file. The header (thread_id:,
// updated_at:, rollout_path:, cwd:, git_branch:) runs up to the first blank
// line; a line that is no header line starts the summary.
func memorySummary(text string) (map[string]string, string) {
	header := map[string]string{}
	lines := textlines.Split(text)
	for i, line := range lines {
		if strings.Trim(line, textlines.ASCIISpace) == "" {
			return header, strings.Join(lines[i+1:], "\n")
		}
		key, value, found := strings.Cut(line, ":")
		key = strings.Trim(key, textlines.ASCIISpace)
		if !found || !isMemoryKey(key) {
			return header, strings.Join(lines[i:], "\n")
		}
		header[key] = strings.Trim(value, textlines.ASCIISpace)
	}
	return header, ""
}

// isMemoryKey reports whether key is a memory summary header key.
func isMemoryKey(key string) bool {
	for _, known := range memoryHeaderKeys {
		if key == known {
			return true
		}
	}
	return false
}

// parseMemorySummary emits the "memory summary:" row of a thread's summary
// file, at its updated_at.
func (p *Parser) parseMemorySummary(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	content, ok := parser.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	header, body := memorySummary(textlines.ValidUTF8(content))
	threadID := header["thread_id"]
	if threadID == "" {
		emit(p.ProblemRow(artifact, "memory summary has no thread_id header"))
		return
	}
	text := "memory summary: thread " + threadID + " rollout=" + orDash(header["rollout_path"]) +
		" (from " + artifact.Rel + ", updated_at): " + strings.TrimSpace(body)
	row := p.systemRow(artifact, timeline.Compact(text), threadID, 1)
	row.TimestampUTC = timeutil.FromText(header["updated_at"])
	row.ProjectPath = header["cwd"]
	row.GitBranch = header["git_branch"]
	emit(row)
}
