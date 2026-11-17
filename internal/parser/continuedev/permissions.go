package continuedev

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/pyjson"
	"github.com/seanthegeek/doubleagent/internal/pytext"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// The lines of the permissions.yaml shape the CLI reads and writes
// (permissionsYamlLoader.ts:15-19, :162-171; policyWriter.ts:57-66): the
// three list keys, each empty ([], null, ~ or nothing) or followed by
// "- <rule>" lines; and the scalar forms of a rule.
var (
	yamlKey          = regexp.MustCompile(`^(allow|ask|exclude):[ \t]*(\[[ \t]*\]|null|~)?[ \t]*(?:#.*)?\z`)
	yamlItem         = regexp.MustCompile(`^[ \t]*-[ \t]+(.*?)[ \t]*\z`)
	yamlDoubleQuoted = regexp.MustCompile(`^("(?:[^"\\]|\\.)*")[ \t]*(?:#.*)?\z`)
	yamlSingleQuoted = regexp.MustCompile(`^'((?:[^']|'')*)'[ \t]*(?:#.*)?\z`)
	yamlComment      = regexp.MustCompile(`[ \t]+#.*\z`)
	yamlMapping      = regexp.MustCompile(`:[ \t]`)
)

// yamlScalar returns a YAML sequence item as a string, or false for
// anything but a scalar on its line. A double-quoted item is decoded as
// JSON, whose escapes it shares; in a single-quoted one ” is one quote; a
// plain one loses its " #" comment. A nested sequence or mapping, a flow
// collection, an alias, a block scalar or an unclosed quote is no scalar.
func yamlScalar(text string) (string, bool) {
	switch {
	case strings.HasPrefix(text, `"`):
		match := yamlDoubleQuoted.FindStringSubmatch(text)
		if match == nil {
			return "", false
		}
		value, err := pyjson.DecodeString(match[1])
		return value.Text(), err == nil && value.IsString()
	case strings.HasPrefix(text, "'"):
		match := yamlSingleQuoted.FindStringSubmatch(text)
		if match == nil {
			return "", false
		}
		return strings.ReplaceAll(match[1], "''", "'"), true
	case text == "" || strings.IndexByte("[]{}&*!|>%@`-?:,#", text[0]) >= 0:
		return "", false
	}
	value := yamlComment.ReplaceAllLiteralString(text, "")
	// A mapping value starts at ":" before a space or tab.
	if yamlMapping.MatchString(value) || strings.HasSuffix(value, ":") {
		return "", false
	}
	return value, true
}

// rule is one rule of permissions.yaml: its line, list and text.
type rule struct {
	line       int
	list, text string
}

// readPermissions returns the rules of a permissions.yaml, or, with no
// rules, why the file is not the flat shape the CLI writes: a reader of
// that shape alone cannot tell what any other YAML construct grants.
func readPermissions(text string) ([]rule, string) {
	var rules []rule
	current := ""
	for i, line := range textlines.Split(text) {
		if i == 0 {
			line = strings.TrimPrefix(line, byteOrderMark)
		}
		stripped := strings.Trim(line, textlines.SpaceOrTab)
		if stripped == "" || strings.HasPrefix(stripped, "#") || stripped == "---" {
			continue
		}
		if key := yamlKey.FindStringSubmatch(line); key != nil {
			current = key[1]
			if key[2] != "" {
				current = ""
			}
			continue
		}
		item := yamlItem.FindStringSubmatch(line)
		value, ok := "", false
		if item != nil && current != "" {
			value, ok = yamlScalar(item[1])
		}
		if !ok {
			return nil, "unsupported YAML at line " + strconv.Itoa(i+1)
		}
		rules = append(rules, rule{line: i + 1, list: current, text: value})
	}
	return rules, ""
}

// permissions emits one "permission setting:" row per rule of
// permissions.yaml, at the file's modification time; a file outside the
// shape readPermissions reads is one problem row.
func (p *Parser) permissions(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	content, ok := parser.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	rules, problem := readPermissions(pytext.DecodeUTF8(content))
	if problem != "" {
		emit(p.ProblemRow(artifact, problem+" (only allow, ask and exclude lists of strings are read)"))
		return
	}
	timestamp := parser.FileMtime(artifact)
	for _, each := range rules {
		row := p.SystemRow(artifact, timeline.Compact("permission setting: "+each.list+"="+each.text+
			" (from "+artifact.Rel+", file mtime)"))
		row.SourceLine = each.line
		row.TimestampUTC = timestamp
		emit(row)
	}
}
