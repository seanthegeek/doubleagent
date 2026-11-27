package crush

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/textlines"
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
	parserkit.Base
	database, projects, original, config, crushrc, configOriginal *parserkit.RelPattern
	channel, attribute                                            *regexp.Regexp
}

// New returns the parser.
func New() *Parser {
	return &Parser{
		Base:           parserkit.NewBase("crush", "crush"),
		database:       parserkit.MustRelPattern(databasePattern),
		projects:       parserkit.MustRelPattern(projectsPattern),
		original:       parserkit.MustRelPattern(originalPattern),
		config:         parserkit.MustRelPattern(configPattern),
		crushrc:        parserkit.MustRelPattern(crushrcPattern),
		configOriginal: parserkit.MustRelPattern(configOriginalPattern),
		channel:        regexp.MustCompile(channelPattern),
		attribute:      regexp.MustCompile(attributePattern),
	}
}

// Marks says the parser records sessions relayed through an MCP channel.
func (p *Parser) Marks() parserkit.Marks {
	return parserkit.Marks{Relay: true}
}

// Wants reports whether the artifact is one of the files the parser reads.
func (p *Parser) Wants(artifact *input.Artifact) bool {
	return parserkit.WantsAny(p.Kinds(), artifact)
}

// Kinds returns the database, the projects list, the config and crushrc.
func (p *Parser) Kinds() []parserkit.Kind {
	return []parserkit.Kind{parserkit.PatternKind("crush.db", p.database), parserkit.PatternKind("projects.json", p.projects),
		parserkit.PatternKind("crush.json", p.config), parserkit.PatternKind("crushrc", p.crushrc)}
}

// Parse emits the rows of one artifact, by the file it is.
func (p *Parser) Parse(artifact *input.Artifact, options parserkit.Options, emit parserkit.Emit) error {
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

// textOnly returns a value's text when it is a string, else "".
func textOnly(value jsonvalue.Value) string {
	if value.IsString() {
		return value.StringValue()
	}
	return ""
}

// parseProjects emits one row per project of projects.json; a document that
// is not an object, or a projects that is not an array, adds a problem row.
func (p *Parser) parseProjects(artifact *input.Artifact, emit parserkit.Emit) {
	var problems parserkit.Problems
	content, ok := parserkit.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	document, err := parserkit.DecodeJSONBytes(artifact, content)
	if err != nil {
		emit(p.system(artifact, parserkit.ProblemPrefix+"projects.json did not parse: "+err.Error(), 0))
		return
	}
	if !document.IsObject() {
		emit(p.system(artifact, parserkit.ProblemPrefix+parserkit.WrongKind("projects.json", jsonvalue.Object), 0))
		return
	}
	var found parserkit.Problems
	projects := parserkit.EntriesOfKind(document.Get("projects"), "projects", jsonvalue.Object, 0, &found)
	parserkit.EmitAll(emit, p.ErrorRows(artifact, found, "", ""))
	for position, project := range projects {
		row := p.system(artifact, "", position+1)
		row.TimestampUTC = timeutil.FromValue(project.Get("last_accessed"))
		row.ProjectPath = parserkit.Text(project.Get("path"))
		dataDir := ""
		if value := project.Get("data_dir"); !value.IsEmpty() {
			dataDir = "data_dir=" + value.Text()
		}
		row.Text = timeline.Compact(parserkit.JoinNonEmpty(" | ", "crush project last accessed", dataDir))
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
func (p *Parser) settingRow(artifact *input.Artifact, timestamp, key string, value jsonvalue.Value,
	where string, line int) timeline.Row {
	row := p.system(artifact, "", line)
	row.TimestampUTC = timestamp
	row.ProjectPath = p.configProject(artifact)
	row.Text = timeline.Compact("permission setting: " + key + "=" + parserkit.SettingValue(value) +
		" (from " + where + ", file mtime)")
	return row
}

// settingList is one list of standing entries of a crush.json, with the
// key its rows name.
type settingList struct {
	key     string
	entries jsonvalue.Value
}

// configLists returns the lists of standing entries of a crush.json in
// this order: permissions.allowed_tools, options.disabled_tools, each
// MCP server's disabled_tools and enabled_tools, and each hook event. A
// permissions, options, mcp or hooks that is not an object, and MCP servers
// that are not objects, add a problem at line 1.
func configLists(document jsonvalue.Value, problems *parserkit.Problems) []settingList {
	var lists []settingList
	for _, pair := range [][2]string{{"permissions", "allowed_tools"}, {"options", "disabled_tools"}} {
		block := document.Get(pair[0])
		if parserkit.ObjectMember(block, pair[0], 1, problems); block.IsObject() {
			lists = append(lists, settingList{pair[0] + "." + pair[1], block.Get(pair[1])})
		}
	}
	for name, server := range parserkit.MembersOfKind(document.Get("mcp"), "mcp", jsonvalue.Object, 1, problems) {
		for _, key := range []string{"disabled_tools", "enabled_tools"} {
			lists = append(lists, settingList{"mcp." + name + "." + key, server.Get(key)})
		}
	}
	for _, event := range parserkit.ObjectMember(document.Get("hooks"), "hooks", 1, problems) {
		lists = append(lists, settingList{"hooks." + event.Key, event.Value})
	}
	return lists
}

// parseConfig emits one "permission setting:" row per standing entry of a
// crush.json, then one "stored credentials:" row of each provider's
// api_key through parserkit.SecretText; nothing else in the file is read. A
// file that is not JSON gives the reader's error through
// parserkit.ReaderErrorText, since the file holds API keys; a member of the
// wrong kind on the way to those rows gives a problem row after them.
func (p *Parser) parseConfig(artifact *input.Artifact, emit parserkit.Emit) {
	timestamp := parserkit.FileMtime(artifact)
	var problems parserkit.Problems
	content, ok := parserkit.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", p.configProject(artifact)))
		return
	}
	document, err := parserkit.DecodeJSONBytes(artifact, content)
	if err != nil || !document.IsObject() {
		text := parserkit.ProblemPrefix + artifact.Rel + " is not a JSON object"
		if err != nil {
			text += " (" + parserkit.ReaderErrorText(artifact, err) + ")"
		}
		row := p.system(artifact, text, 1)
		row.TimestampUTC = timestamp
		row.ProjectPath = p.configProject(artifact)
		emit(row)
		return
	}
	var wrong parserkit.Problems
	carried := parserkit.NewCarried(artifact)
	for _, list := range configLists(document, &wrong) {
		// Each entry's row repeats its list's key, a server's or hook's name in it.
		key := carried.Cut("settings key", list.key)
		for _, entry := range parserkit.ArrayMember(list.entries, list.key, 1, &wrong) {
			emit(p.settingRow(artifact, timestamp, key, entry, artifact.Rel, 1))
		}
	}
	carried.Report(1, &wrong)
	var keys []string
	for name, provider := range parserkit.MembersOfKind(document.Get("providers"), "providers", jsonvalue.Object, 1,
		&wrong) {
		if key := parserkit.Text(provider.Get("api_key")); key != "" {
			keys = append(keys, "providers."+name+".api_key="+parserkit.SecretText(artifact, key))
		}
	}
	if len(keys) > 0 {
		row := p.system(artifact, "stored credentials: "+strings.Join(keys, " ")+" (from "+artifact.Rel+
			", file mtime)", 1)
		row.TimestampUTC = timestamp
		row.ProjectPath = p.configProject(artifact)
		emit(row)
	}
	for _, problem := range wrong {
		row := p.system(artifact, parserkit.ProblemPrefix+problem.Message, problem.Line)
		row.TimestampUTC = timestamp
		row.ProjectPath = p.configProject(artifact)
		emit(row)
	}
}

// parseCrushrc emits one "permission setting:" row per tool on a crushrc
// "permissions allow" or "permissions deny" line: allow adds to
// permissions.allowed_tools, deny to options.disabled_tools. Such a line is
// read as written, whatever condition is around it. A line is split into
// words when it starts, after blanks, with the text "permissions" (so
// permissions_note='x also is), and one that does not split into shell
// words is a bad line; only a first word of exactly "permissions" gives rows.
func (p *Parser) parseCrushrc(artifact *input.Artifact, emit parserkit.Emit) {
	var problems parserkit.Problems
	content, ok := parserkit.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", p.configProject(artifact)))
		return
	}
	timestamp := parserkit.FileMtime(artifact)
	keys := map[string]string{"allow": "permissions.allowed_tools", "deny": "options.disabled_tools"}
	for i, line := range strings.Split(textlines.ValidUTF8(content), "\n") {
		number := i + 1
		line = strings.TrimSuffix(line, "\r")
		if !strings.HasPrefix(strings.TrimLeftFunc(line, unicode.IsSpace), "permissions") {
			continue
		}
		words, err := splitWords(line)
		if err != nil {
			problems.AddRaw(number, "unparseable line ("+err.Error()+")", line)
			continue
		}
		if len(words) < 3 || words[0] != "permissions" || keys[words[1]] == "" {
			continue
		}
		where := artifact.Rel + " line " + strconv.Itoa(number)
		for _, tool := range words[2:] {
			emit(p.settingRow(artifact, timestamp, keys[words[1]], jsonvalue.NewString(tool), where, number))
		}
	}
	parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", p.configProject(artifact)))
}

// The interfaces the registry looks for, checked when the package builds.
var (
	_ parserkit.Parser = (*Parser)(nil)
	_ parserkit.Marker = (*Parser)(nil)
)
