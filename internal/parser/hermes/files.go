package hermes

import (
	"path"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parser/aider"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// pasteLifetime is how long a paste.rs upload lives, in seconds: its
// expire_at is its upload time plus this.
const pasteLifetime = 21600

// trajectoryTypes are the turn types of a ShareGPT entry's "from".
func trajectoryTypes() map[string]string {
	return map[string]string{"human": timeline.TurnUser, "gpt": timeline.TurnAssistant,
		"tool": timeline.TurnToolResult}
}

// setting returns a system row with no session at a line and time.
func (p *Parser) setting(artifact *input.Artifact, line int, text, timestamp string) timeline.Row {
	row := p.system(artifact, line, noSession(), text)
	row.TimestampUTC = timestamp
	return row
}

// readJSON returns the JSON document of the artifact, or false after
// adding a whole-file problem: its unreadable problem, or
// "<name> did not parse (<reason>)". For a file that holds secrets the
// reason goes through parser.ReaderErrorText, which withholds the reader's
// detail when the run withholds secrets.
func (p *Parser) readJSON(artifact *input.Artifact, name string, holdsSecrets bool,
	problems *parser.Problems) (jsonvalue.Value, bool) {
	content, ok := parser.FileBytes(artifact.DiskPath, problems)
	if !ok {
		return jsonvalue.Value{}, false
	}
	document, err := jsonvalue.Decode(content)
	if err != nil {
		reason := err.Error()
		if holdsSecrets {
			reason = parser.ReaderErrorText(artifact, err)
		}
		problems.Add(0, name+" did not parse ("+reason+")")
		return jsonvalue.Value{}, false
	}
	return document, true
}

// parsePastes emits one "debug report uploaded:" row per pastes/pending.json
// entry, at its expire_at less the paste lifetime. A file that is not a JSON
// list is one `parser:` row, an entry that is not an object with a url its
// own row.
func (p *Parser) parsePastes(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	document, ok := p.readJSON(artifact, "pastes/pending.json", false, &problems)
	if ok && !document.IsArray() {
		problems.Add(0, "pastes/pending.json is not a JSON list")
	}
	if !ok || !document.IsArray() {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	for i, entry := range document.Items() {
		position := i + 1
		if url := entry.Get("url"); !entry.IsObject() || url.IsEmpty() {
			problems.AddRaw(position, "unparseable record "+strconv.Itoa(position)+
				" (not an object with a url)", parser.JSONText(entry))
			continue
		}
		row := p.system(artifact, position, noSession(), "debug report uploaded: "+
			parser.URLText(artifact, entry.Get("url").Text())+" (from pastes/pending.json, expire_at minus 6 hours)")
		expire := entry.Get("expire_at")
		if seconds, ok := expire.Int(); ok && expire.IsInt() {
			row.TimestampUTC = timeutil.FromInt(seconds - pasteLifetime)
		} else if number, ok := expire.Float(); ok && expire.IsFloat() {
			row.TimestampUTC = timeutil.FromEpoch(number - pasteLifetime)
		}
		emit(row)
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// parseConfig emits one "permission setting: command_allowlist=<pattern>"
// row per entry of config.yaml's top-level command_allowlist block list;
// any other construct under the key is one whole-file `parser:` row after
// the entries before it.
func (p *Parser) parseConfig(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	text := ""
	if content, ok := parser.FileBytes(artifact.DiskPath, &problems); ok {
		text = textlines.ValidUTF8(content)
	}
	entries, err := p.readFlatList(textlines.Split(text), "command_allowlist")
	timestamp := ""
	if len(entries) > 0 {
		timestamp = parser.FileMtime(artifact)
	}
	for _, entry := range entries {
		emit(p.setting(artifact, entry.line, "permission setting: command_allowlist="+entry.value+
			" (from config.yaml, file mtime)", timestamp))
	}
	if err != nil {
		problems.Add(0, "config.yaml command_allowlist is not a flat list ("+err.Error()+")")
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// parsePairing emits one "permission setting: pairing.<platform>.<user id>"
// row per user the gateway's owner approved, at its approved_at.
func (p *Parser) parsePairing(artifact *input.Artifact, match []string, emit parser.Emit) {
	name := match[1] + "/" + match[2] + "-approved.json"
	var problems parser.Problems
	document, ok := p.readJSON(artifact, name, true, &problems)
	if ok && !document.IsObject() {
		problems.Add(0, name+" is not a JSON object")
	}
	for i, user := range document.Members() {
		position := i + 1
		if !user.Value.IsObject() {
			problems.AddRaw(position, "unparseable record "+strconv.Itoa(position)+" (not an object)",
				parser.JSONText(jsonvalue.NewObject(user)))
			continue
		}
		emit(p.setting(artifact, position, "permission setting: pairing."+match[2]+"."+user.Key+
			" user_name="+parser.SettingOrDash(user.Value.Get("user_name"))+" (from "+name+", approved_at)",
			timeutil.FromValue(user.Value.Get("approved_at"))))
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// parseHistory emits the prompt history of the classic CLI and the TUI,
// read as an Aider input history: one user row per entry at its time, or a
// "slash command:" row for an entry that starts with /.
func (p *Parser) parseHistory(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	for _, entry := range aider.ReadInputHistory(artifact.DiskPath, &problems) {
		row := p.BaseRow(artifact)
		row.SourceLine = entry.Line
		row.TimestampUTC = timeutil.FromText(entry.Timestamp)
		if strings.HasPrefix(entry.Text, "/") {
			row.TurnType, row.Text = timeline.TurnSystem, timeline.Compact("slash command: "+entry.Text)
		} else {
			row.TurnType, row.Text = timeline.TurnUser, timeline.Compact(entry.Text)
		}
		emit(row)
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// parseCron emits the rows of a cron run's document, all at the file
// name's time (host-local, written as if UTC): a "cron run: job" row, then
// the prompt as user and the response as assistant, or the whole document
// as a "cron run output:" row when it has no prompt and response sections.
func (p *Parser) parseCron(artifact *input.Artifact, match []string, emit parser.Emit) {
	timestamp := ""
	if clock := p.cronTime.FindStringSubmatch(match[2]); clock != nil {
		timestamp = timeutil.FromText(clock[1] + "T" + clock[2] + ":" + clock[3] + ":" + clock[4])
	}
	var problems parser.Problems
	document := ""
	if content, ok := parser.FileBytes(artifact.DiskPath, &problems); ok {
		document = textlines.ValidUTF8(content)
	}
	type turn struct{ turnType, text string }
	turns := []turn{{timeline.TurnSystem, "cron run: job " + match[1] +
		" (from cron/output, file name time, host-local)"}}
	if prompt, reply, ok := p.splitCronDocument(document); document != "" && ok {
		turns = append(turns, turn{timeline.TurnUser, prompt}, turn{timeline.TurnAssistant, reply})
	} else if document != "" {
		turns = append(turns, turn{timeline.TurnSystem, "cron run output: " + document})
	}
	for position, each := range turns {
		row := p.BaseRow(artifact)
		row.SourceLine = position
		row.TimestampUTC = timestamp
		row.TurnType = each.turnType
		row.Text = timeline.Compact(each.text)
		emit(row)
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// parseTrajectory emits the rows of a --save-trajectories file in the
// run's working directory: per line a "trajectory:" row at the line's
// timestamp (host-local, written as if UTC), then one row per ShareGPT
// entry of conversations with no time of its own. There is no session;
// the project is the collected directory.
func (p *Parser) parseTrajectory(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	project := ""
	if artifact.Home != nil && artifact.Home.Project {
		project = artifact.Home.Original
	}
	name := path.Base(artifact.Rel)
	for number, raw := range parser.Lines(artifact.DiskPath, &problems) {
		if len(strings.Trim(string(raw), textlines.ASCIIWhitespace)) == 0 {
			continue
		}
		record, err := jsonvalue.Decode(raw)
		reason := ""
		switch {
		case err != nil:
			reason = err.Error()
		case !record.IsObject():
			reason = "not a JSON object"
		case !record.Get("conversations").IsArray():
			reason = "conversations is not a list"
		}
		if reason != "" {
			problems.AddRaw(number, "unparseable line ("+reason+")", parser.LineText(raw))
			continue
		}
		p.trajectoryRows(artifact, number, project, name, record, emit)
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", project))
}

// trajectoryRows emits the rows of one trajectory line.
func (p *Parser) trajectoryRows(artifact *input.Artifact, number int, project, name string,
	record jsonvalue.Value, emit parser.Emit) {
	head := p.BaseRow(artifact)
	head.SourceLine, head.ProjectPath, head.TurnType = number, project, timeline.TurnSystem
	head.Model = parser.Text(record.Get("model"))
	head.TimestampUTC = timeutil.FromValue(record.Get("timestamp"))
	head.Text = "trajectory: model=" + parser.SettingOrDash(record.Get("model")) + " completed=" +
		parser.SettingOrDash(record.Get("completed")) + " (from " + name + ", timestamp host-local)"
	emit(head)
	for _, entry := range record.Get("conversations").Items() {
		row := p.BaseRow(artifact)
		row.SourceLine, row.ProjectPath, row.Model = number, project, head.Model
		sender, value := jsonvalue.NewNull(), ""
		if entry.IsObject() {
			sender, value = entry.Get("from"), parser.TextOf(entry.Get("value"))
		}
		turnType, known := trajectoryTypes()[sender.StringValue()]
		switch {
		case sender.IsString() && known:
			row.TurnType, row.Text = turnType, timeline.Compact(value)
		case parser.IsText(sender, "system"):
			row.TurnType = timeline.TurnSystem
			row.Text = timeline.Compact("trajectory system prompt: " + value)
		default:
			if value == "" {
				value = parser.JSONText(entry)
			}
			row.TurnType = timeline.TurnSystem
			row.Text = timeline.Compact(parser.SettingOrDash(sender) + ": " + value)
		}
		emit(row)
	}
}

// parseConnections emits the desktop app's remote backends and the entries
// it quarantined, at the file's mtime: one "remote backend:" row per
// connection that is not the app's own local runtime, with org=, name=,
// profile=, token= and headers= when recorded. A quarantined that is not an
// array adds a problem.
func (p *Parser) parseConnections(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	document, ok := p.readJSON(artifact, "connections.json", true, &problems)
	connections := document.Get("connections")
	if ok && !connections.IsArray() {
		problems.Add(0, "connections.json has no connections list")
	}
	timestamp := parser.FileMtime(artifact)
	const tail = " (from connections.json, file mtime)"
	for i, connection := range parser.Elements(connections) {
		position := i + 1
		if !connection.IsObject() {
			problems.AddRaw(position, "unparseable record "+strconv.Itoa(position)+" (not an object)",
				parser.JSONText(connection))
			continue
		}
		if parser.IsText(connection.Get("kind"), "local") {
			continue
		}
		emit(p.setting(artifact, position, connectionText(artifact, connection)+tail, timestamp))
	}
	for _, entry := range parser.ArrayMember(document.Get("quarantined"), "quarantined", 0, &problems) {
		emit(p.setting(artifact, 0, "remote backend quarantined by the app: "+
			parser.SettingOrDash(entry.Get("reason"))+tail, timestamp))
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// connectionText returns the "remote backend:" text of one connection: its
// url through parser.URLText, else [user@]host[:port], and the token and
// headers stored in plain text through parser.SecretText.
func connectionText(artifact *input.Artifact, connection jsonvalue.Value) string {
	target := connection.Get("url")
	if target.IsString() {
		target = jsonvalue.NewString(parser.URLText(artifact, target.StringValue()))
	}
	if host := connection.Get("host"); target.IsEmpty() && !host.IsEmpty() {
		text := host.Text()
		if user := connection.Get("user"); !user.IsEmpty() {
			text = user.Text() + "@" + text
		}
		if port := connection.Get("port"); !port.IsEmpty() {
			text += ":" + port.Text()
		}
		target = jsonvalue.NewString(text)
	}
	text := "remote backend: " + parser.SettingOrDash(connection.Get("kind")) + " " + parser.SettingOrDash(connection.Get("label")) +
		" " + parser.SettingOrDash(target)
	for _, pair := range [][2]string{{"org", "org"}, {"name", "name"}, {"remoteProfile", "profile"}} {
		if value := connection.Get(pair[0]); !value.IsEmpty() {
			text += " " + pair[1] + "=" + parser.SettingValue(value)
		}
	}
	for _, name := range []string{"token", "headers"} {
		if value, ok := plainSecret(connection.Get(name)); ok {
			text += " " + name + "=" + parser.SecretText(artifact, value)
		}
	}
	return text
}

// plainSecret returns the value of a connection's secret envelope that the
// app wrote in plain text, {"encoding": "plain", "value": <secret>}: a
// string as it is, JSON as compact text. An envelope of another encoding is
// ciphertext of the operating system's key store and is not read.
func plainSecret(envelope jsonvalue.Value) (string, bool) {
	value := envelope.Get("value")
	if !parser.IsText(envelope.Get("encoding"), "plain") || value.IsEmpty() {
		return "", false
	}
	return parser.Text(value), true
}
