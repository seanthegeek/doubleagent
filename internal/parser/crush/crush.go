// Package crush reads Crush's per-project SQLite database, its project
// registry and its permission settings.
//
// It reads <project>/.crush/crush.db (WAL mode; the artifact's rel is
// .crush/crush.db under a project or a home, and the project path is the
// directory holding .crush, since the database records no working
// directory), projects.json under .local/share/crush or AppData/Local/crush,
// every crush.json scope and the crushrc shell configs.
//
// The database's tables read are sessions, messages (whose parts column is
// a JSON array of {type, data}), mcp_enabled_servers, mcp_disabled_servers,
// files (the content's length only) and read_files. Parts carry no time:
// they take the message's created_at, except tool calls and results, at
// finished_at or the finish part's time. A finish part is a row only when
// the turn ended abnormally or carries a message. projects.json gives one
// row per project; crush.json and the crushrc "permissions allow|deny"
// lines give "permission setting:" rows at the file's mtime. No git branch
// is recorded. See research/crush.md.
package crush

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/pyjson"
	"github.com/seanthegeek/doubleagent/internal/pytext"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// The patterns over the artifact's rel and original path, and over a
// relayed message's text.
const (
	databasePattern = `(?:\A|/)\.crush/crush\.db\z`
	projectsPattern = `\A(?:\.local/share/crush|AppData/Local/crush)/projects\.json\z`
	originalPattern = `\A(.*)[\\/]\.crush[\\/]crush\.db\z`
	// Every crush.json scope the catalog collects: the global config, the
	// global data file, the workspace file in a data directory, and a
	// project's own.
	configPattern = `\A(?:\.config/crush/crush\.json|\.local/share/crush/crush\.json` +
		`|AppData/Local/crush/crush\.json|\.crush/crush\.json|\.?crush\.json)\z`
	// The global shell config beside crush.json, and a project's crushrc or
	// .crushrc.
	crushrcPattern        = `\A(?:\.config/crush/crushrc|\.?crushrc)\z`
	configOriginalPattern = `\A(.*)[\\/]\.crush[\\/]crush\.json\z`
	channelPattern        = `\A<channel[ \t\r\n]([^>]*)>`
	attributePattern      = `([A-Za-z_][\p{L}\p{N}_.-]*)="([^"]*)"`
)

// Parser reads Crush's databases, project registry and settings.
type Parser struct {
	parser.Base
	database, projects, original, config, crushrc, configOriginal *regexp.Regexp
	channel, attribute                                            *regexp.Regexp
}

// New returns the parser.
func New() *Parser {
	return &Parser{
		Base:           parser.NewBase("crush", "crush"),
		database:       regexp.MustCompile(databasePattern),
		projects:       regexp.MustCompile(projectsPattern),
		original:       regexp.MustCompile(originalPattern),
		config:         regexp.MustCompile(configPattern),
		crushrc:        regexp.MustCompile(crushrcPattern),
		configOriginal: regexp.MustCompile(configOriginalPattern),
		channel:        regexp.MustCompile(channelPattern),
		attribute:      regexp.MustCompile(attributePattern),
	}
}

// Marks says the parser records sessions relayed through an MCP channel.
func (p *Parser) Marks() parser.Marks {
	return parser.Marks{Relay: true}
}

// Wants reports whether the artifact is one of the files the parser reads.
func (p *Parser) Wants(artifact *input.Artifact) bool {
	rel := artifact.Rel
	return p.database.MatchString(rel) || p.projects.MatchString(rel) ||
		p.config.MatchString(rel) || p.crushrc.MatchString(rel)
}

// Parse emits the rows of one artifact, by the file it is.
func (p *Parser) Parse(artifact *input.Artifact, options parser.Options, emit parser.Emit) error {
	switch rel := artifact.Rel; {
	case p.projects.MatchString(rel):
		p.parseProjects(artifact, emit)
	case p.config.MatchString(rel):
		p.parseConfig(artifact, emit)
	case p.crushrc.MatchString(rel):
		p.parseCrushrc(artifact, emit)
	default:
		p.parseDatabase(artifact, options, emit)
	}
	return nil
}

// projectOf returns the project directory of a crush.db from its original
// path, or "".
func (p *Parser) projectOf(original string) string {
	if match := p.original.FindStringSubmatch(original); match != nil {
		return match[1]
	}
	return ""
}

// system returns a system row of the artifact at a source line.
func (p *Parser) system(artifact *input.Artifact, text string, line int) timeline.Row {
	row := p.SystemRow(artifact, text)
	row.SourceLine = line
	return row
}

// joinParts joins the non-empty parts with " | ".
func joinParts(parts ...string) string {
	var kept []string
	for _, part := range parts {
		if part != "" {
			kept = append(kept, part)
		}
	}
	return strings.Join(kept, " | ")
}

// textOnly returns a value's text when it is a string, else "".
func textOnly(value pyjson.Value) string {
	if value.IsString() {
		return value.Text()
	}
	return ""
}

// parseProjects emits one row per project of projects.json.
func (p *Parser) parseProjects(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	content, ok := parser.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	document, err := pyjson.Decode(content)
	if err != nil {
		emit(p.system(artifact, "parser: projects.json did not parse: "+err.Error(), 0))
		return
	}
	projects := document.Get("projects")
	if !projects.IsArray() {
		return
	}
	for position, project := range projects.Items() {
		if !project.IsObject() {
			continue
		}
		row := p.system(artifact, "", position+1)
		row.TimestampUTC = timeutil.FromValue(project.Get("last_accessed"))
		row.ProjectPath = parser.Str(project.Get("path"))
		dataDir := ""
		if value := project.Get("data_dir"); value.Truthy() {
			dataDir = "data_dir=" + value.Str()
		}
		row.Text = timeline.Compact(joinParts("crush project last accessed", dataDir))
		emit(row)
	}
}

// configProject returns the project a config file belongs to, or "" for a
// global file: a workspace crush.json belongs to the directory holding its
// .crush, a project-catalog file to its project directory.
func (p *Parser) configProject(artifact *input.Artifact) string {
	if match := p.configOriginal.FindStringSubmatch(artifact.Original); match != nil {
		return match[1]
	}
	if artifact.Home != nil && artifact.Home.Project {
		return artifact.Home.Original
	}
	return ""
}

// settingRow returns a "permission setting:" row of a config file.
func (p *Parser) settingRow(artifact *input.Artifact, timestamp, key string, value pyjson.Value,
	where string, line int) timeline.Row {
	row := p.system(artifact, "", line)
	row.TimestampUTC = timestamp
	row.ProjectPath = p.configProject(artifact)
	row.Text = timeline.Compact("permission setting: " + key + "=" + parser.SettingValue(value) +
		" (from " + where + ", file mtime)")
	return row
}

// settingList is one list of standing entries of a crush.json, with the
// key its rows name.
type settingList struct {
	key     string
	entries pyjson.Value
}

// configLists returns the lists of standing entries of a crush.json in the
// analyzer's order: permissions.allowed_tools, options.disabled_tools, each
// MCP server's disabled_tools and enabled_tools, and each hook event.
func configLists(document pyjson.Value) []settingList {
	var lists []settingList
	for _, pair := range [][2]string{{"permissions", "allowed_tools"}, {"options", "disabled_tools"}} {
		if block := document.Get(pair[0]); block.IsObject() {
			lists = append(lists, settingList{pair[0] + "." + pair[1], block.Get(pair[1])})
		}
	}
	if mcp := document.Get("mcp"); mcp.IsObject() {
		for _, server := range mcp.Members() {
			if !server.Value.IsObject() {
				continue
			}
			for _, key := range []string{"disabled_tools", "enabled_tools"} {
				lists = append(lists, settingList{"mcp." + server.Key + "." + key, server.Value.Get(key)})
			}
		}
	}
	if hooks := document.Get("hooks"); hooks.IsObject() {
		for _, event := range hooks.Members() {
			lists = append(lists, settingList{"hooks." + event.Key, event.Value})
		}
	}
	return lists
}

// parseConfig emits one "permission setting:" row per standing entry of a
// crush.json; nothing else in the file is read.
func (p *Parser) parseConfig(artifact *input.Artifact, emit parser.Emit) {
	timestamp := parser.FileMtime(artifact)
	var problems parser.Problems
	content, ok := parser.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", p.configProject(artifact)))
		return
	}
	document, err := pyjson.Decode(content)
	if err != nil || !document.IsObject() {
		row := p.system(artifact, "parser: "+artifact.Rel+" is not a JSON object", 1)
		row.TimestampUTC = timestamp
		row.ProjectPath = p.configProject(artifact)
		emit(row)
		return
	}
	for _, list := range configLists(document) {
		if !list.entries.IsArray() {
			continue
		}
		for _, entry := range list.entries.Items() {
			emit(p.settingRow(artifact, timestamp, list.key, entry, artifact.Rel, 1))
		}
	}
}

// parseCrushrc emits one "permission setting:" row per tool on a crushrc
// "permissions allow" or "permissions deny" line: allow adds to
// permissions.allowed_tools, deny to options.disabled_tools. Such a line is
// read as written, whatever condition is around it; one that does not
// split into shell words is a bad line.
func (p *Parser) parseCrushrc(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	content, ok := parser.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", p.configProject(artifact)))
		return
	}
	timestamp := parser.FileMtime(artifact)
	keys := map[string]string{"allow": "permissions.allowed_tools", "deny": "options.disabled_tools"}
	for i, line := range strings.Split(pytext.DecodeUTF8(content), "\n") {
		number := i + 1
		line = strings.TrimSuffix(line, "\r")
		if !strings.HasPrefix(pytext.StripLeft(line), "permissions") {
			continue
		}
		words, problem := splitWords(line)
		if problem != "" {
			problems.AddRaw(number, "unparseable line ("+problem+")", line)
			continue
		}
		if len(words) < 3 || words[0] != "permissions" || keys[words[1]] == "" {
			continue
		}
		where := artifact.Rel + " line " + strconv.Itoa(number)
		for _, tool := range words[2:] {
			emit(p.settingRow(artifact, timestamp, keys[words[1]], pyjson.NewString(tool), where, number))
		}
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", p.configProject(artifact)))
}

// The interfaces the registry looks for, checked when the package builds.
var (
	_ parser.Parser = (*Parser)(nil)
	_ parser.Marker = (*Parser)(nil)
)
