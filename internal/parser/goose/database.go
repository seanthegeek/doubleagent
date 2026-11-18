package goose

import (
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// storedJSON is a decoded JSON column of sessions.db: the value (JSON null
// for a NULL or empty column), and for one that does not decode, or
// decodes to another kind than Goose writes there, the reason and the
// stored text in full.
type storedJSON struct {
	value       jsonvalue.Value
	reason, raw string
}

// readStored decodes a JSON column that should hold an array when array is
// set, else an object; a blob is read as UTF-8 with replacement, and JSON
// null is accepted.
func readStored(column sqlitefile.Value, array bool) storedJSON {
	var text string
	switch column.Type {
	case sqlitefile.Null:
		return storedJSON{}
	case sqlitefile.Integer, sqlitefile.Float:
		return storedJSON{reason: "not JSON text", raw: column.String()}
	case sqlitefile.Blob:
		text = textlines.ValidUTF8(column.Bytes)
	default:
		text = column.Text()
	}
	if len(column.Bytes) == 0 {
		return storedJSON{}
	}
	decoded, err := jsonvalue.DecodeString(text)
	switch {
	case err != nil:
		return storedJSON{reason: err.Error(), raw: text}
	case decoded.IsNull():
	case array && !decoded.IsArray():
		return storedJSON{reason: "not a JSON array", raw: text}
	case !array && !decoded.IsObject():
		return storedJSON{reason: "not a JSON object", raw: text}
	}
	return storedJSON{value: decoded, raw: text}
}

// orText returns a column as row text, "" for an empty value (NULL, zero,
// or empty text or blob).
func orText(value sqlitefile.Value) string {
	if value.IsEmpty() {
		return ""
	}
	return value.String()
}

// sourceLine returns an id column as a row's source line, 0 when it is not
// an integer.
func sourceLine(value sqlitefile.Value) int {
	if value.Type == sqlitefile.Integer {
		return int(value.Int)
	}
	return 0
}

// epochColumn returns a created_timestamp column as epochNumber reads a
// number, text as timeutil reads it.
func epochColumn(value sqlitefile.Value) string {
	switch value.Type {
	case sqlitefile.Integer:
		if value.Int > 10_000_000_000 {
			return timeutil.FromEpoch(float64(value.Int) / 1000.0)
		}
	case sqlitefile.Float:
		return epochNumber(value.Float)
	}
	return parser.ColumnTime(value)
}

// database reads one sessions.db: the sessions by id, the model tracked per
// session across sessions and messages, and each session's model.
type database struct {
	parser   *Parser
	artifact *input.Artifact
	options  parser.Options
	handle   *sqlitefile.Database
	sessions map[string]sqlitefile.Row
	models   parser.Tracker
	modes    parser.Tracker
	byModel  map[string]string
	emit     parser.Emit
}

// parseDatabase emits the rows of sessions.db: each session's rows, then
// each message's. A database error ends the rows with one `parser:` row;
// the rows before it stay.
func (p *Parser) parseDatabase(artifact *input.Artifact, options parser.Options, emit parser.Emit) {
	problem := func(message string) {
		row := p.row(artifact, 0, "", "", "", "")
		row.TurnType, row.Text = timeline.TurnSystem, parser.ProblemPrefix+message
		emit(row)
	}
	handle, message := parser.OpenDatabase(artifact.DiskPath)
	if message != "" {
		problem(message)
		return
	}
	defer handle.Close()
	reader := &database{parser: p, artifact: artifact, options: options, handle: handle,
		sessions: map[string]sqlitefile.Row{}, models: parser.Tracker{}, modes: parser.Tracker{},
		byModel: map[string]string{}, emit: emit}
	if err := reader.rows(); err != nil {
		problem(parser.DatabaseError(err))
	}
}

// rows emits the database's rows, stopping at the first error.
func (d *database) rows() error {
	tables, err := d.handle.TableNames()
	if err != nil || !tables["messages"] {
		return err
	}
	if tables["sessions"] {
		table, err := d.handle.Table("sessions")
		if err != nil {
			return err
		}
		err = table.Select(allColumns(table)...).OrderBy("created_at", "rowid").
			Each(func(session sqlitefile.Row) error {
				d.session(session)
				return nil
			})
		if err != nil {
			return err
		}
	}
	answered, err := d.answered()
	if err != nil {
		return err
	}
	table, err := d.handle.Table("messages")
	if err != nil {
		return err
	}
	reader := &blockReader{parser: d.parser, artifact: d.artifact, options: d.options,
		toolNames: map[string]string{}}
	return table.Select().OrderBy("session_id", "created_timestamp", "id").
		Each(func(message sqlitefile.Row) error {
			d.message(message, reader, answered)
			return nil
		})
}

// allColumns returns "rowid" and every column of the table, the columns of
// "select rowid as _rowid, *".
func allColumns(table *sqlitefile.Table) []string {
	columns := []string{"rowid"}
	for _, column := range table.Columns() {
		columns = append(columns, column.Name)
	}
	return columns
}

// session emits a session's start row, its bad-config row, and its relay,
// model and permission mode rows.
func (d *database) session(session sqlitefile.Row) {
	rowid := int(session.Rowid)
	config := readStored(session.Get("model_config_json"), false)
	var problems parser.Problems
	if config.reason != "" {
		problems.AddRaw(rowid, "unparseable record "+itoa(rowid)+" (sessions.model_config_json: "+
			config.reason+")", config.raw)
	}
	modelValue := ""
	if config.value.IsObject() {
		modelValue = parser.Text(config.value.Get("model_name"))
	}
	sessionID := orText(session.Get("id"))
	provider := orText(session.Get("provider_name"))
	d.byModel[sessionID] = modelName(provider, modelValue)
	d.sessions[sessionID] = session
	workingDir := orText(session.Get("working_dir"))
	row := d.parser.row(d.artifact, rowid, sessionID, workingDir, d.byModel[sessionID],
		parser.ColumnTime(session.Get("created_at")))
	row.TurnType = timeline.TurnSystem
	row.Text = timeline.Compact(parser.JoinNonEmpty(" | ", "session start", orText(session.Get("name")),
		orText(session.Get("description")), labelled("type=", session.Get("session_type")),
		labelled("parent=", session.Get("parent_session_id")),
		labelled("schedule=", session.Get("schedule_id")),
		labelled("archived=", session.Get("archived_at"))))
	d.emit(row)
	parser.EmitAll(d.emit, d.parser.ErrorRows(d.artifact, problems, sessionID, workingDir))
	if text := d.parser.gatewayText(session); text != "" {
		d.emit(systemCopy(row, text))
	}
	if text := d.models.Change("model", sessionID, modelValue); text != "" {
		if provider != "" {
			text += " provider=" + provider
		}
		d.emit(systemCopy(row, text))
	}
	if text := d.modes.Change("permission mode", sessionID, orText(session.Get("goose_mode"))); text != "" {
		d.emit(systemCopy(row, text))
	}
}

// labelled returns label and the column's text when the column is not
// empty, else "".
func labelled(label string, value sqlitefile.Value) string {
	if value.IsEmpty() {
		return ""
	}
	return label + value.String()
}

// itoa returns n in decimal.
func itoa(n int) string {
	return strconv.Itoa(n)
}

// gatewayText returns the "session relayed:" text of a gateway session, ""
// for any other. The platform and user id come from working_dir
// (<config>/gateway/<platform>/<user_id>), else the platform from name; the
// display name comes from name (<platform>/<display name or user id>), "-"
// when it is the user id or absent.
func (p *Parser) gatewayText(session sqlitefile.Row) string {
	if kind := session.Get("session_type"); kind.Type != sqlitefile.Text || kind.Text() != "gateway" {
		return ""
	}
	platform, userID := "", ""
	if match := p.gateway.FindStringSubmatch(orText(session.Get("working_dir"))); match != nil {
		platform, userID = match[1], match[2]
	}
	namePlatform, display, _ := strings.Cut(orText(session.Get("name")), "/")
	if platform == "" {
		platform = namePlatform
	}
	if display == userID {
		display = ""
	}
	return "session relayed: gateway " + dash(platform) + " user " + dash(userID) + " display " +
		dash(display) + " (from sessions.db session_type gateway, created_at)"
}

// dash returns text, or "-" when it is empty.
func dash(text string) string {
	if text == "" {
		return "-"
	}
	return text
}

// answered maps each session id to the tool request ids its
// toolConfirmationResponse blocks answer. A content_json that does not
// decode is skipped here; the full read of the same row reports it.
func (d *database) answered() (map[string]map[string]bool, error) {
	answered := map[string]map[string]bool{}
	table, err := d.handle.Table("messages")
	if err != nil {
		return nil, err
	}
	err = table.Select("session_id", "content_json").Where(func(row sqlitefile.Row) bool {
		return sqlitefile.Like(row.Get("content_json"), "%toolConfirmationResponse%")
	}).Each(func(row sqlitefile.Row) error {
		content := readStored(row.Get("content_json"), true).value
		for _, block := range arrayItems(content) {
			data := block.Get("data")
			if data.IsObject() && isText(data.Get("actionType"), "toolConfirmationResponse") {
				sessionID := orText(row.Get("session_id"))
				if answered[sessionID] == nil {
					answered[sessionID] = map[string]bool{}
				}
				answered[sessionID][parser.Text(data.Get("id"))] = true
			}
		}
		return nil
	})
	return answered, err
}

// arrayItems returns the items of an array, nothing for any other value.
func arrayItems(value jsonvalue.Value) []jsonvalue.Value {
	if value.IsArray() {
		return value.Items()
	}
	return nil
}

// message emits a message's model change, its bad-value rows and the rows
// of its content blocks.
func (d *database) message(message sqlitefile.Row, reader *blockReader,
	answered map[string]map[string]bool) {
	sessionID := orText(message.Get("session_id"))
	projectPath := orText(d.sessions[sessionID].Get("working_dir"))
	record := message.Get("id")
	if record.IsEmpty() {
		record = sqlitefile.IntValue(0)
	}
	line := sourceLine(record)
	var problems parser.Problems
	stored := readStored(message.Get("metadata_json"), false)
	if stored.reason != "" {
		problems.AddRaw(line, "unparseable record "+record.String()+" (messages.metadata_json: "+
			stored.reason+")", stored.raw)
	}
	meta := stored.value
	if !meta.IsObject() {
		meta = jsonvalue.NewObject()
	}
	base := d.parser.row(d.artifact, line, sessionID, projectPath, "",
		timeutil.FirstNonEmpty(epochColumn(message.Get("created_timestamp")),
			parser.ColumnTime(message.Get("timestamp"))))
	base.Model = d.messageModel(base, orObject(meta.Get("inference")))
	parser.EmitAll(d.emit, d.parser.ErrorRows(d.artifact, problems, sessionID, projectPath))
	content := readStored(message.Get("content_json"), true)
	if content.value.IsNull() && content.reason == "" {
		content.reason = "empty"
		if content.raw != "" {
			content.reason = "not a JSON array"
		}
	}
	if content.reason != "" {
		var bad parser.Problems
		bad.AddRaw(line, "unparseable record "+record.String()+" (messages.content_json: "+
			content.reason+")", content.raw)
		parser.EmitAll(d.emit, d.parser.ErrorRows(d.artifact, bad, sessionID, projectPath))
		return
	}
	reader.blocks(base, orText(message.Get("role")), content.value, userVisible(meta),
		answeredOf(answered, sessionID), d.emit)
}

// messageModel returns the model of a message from its inference
// metadata, else its session's, and emits the model row of a change.
func (d *database) messageModel(base timeline.Row, inference jsonvalue.Value) string {
	resolved := parser.Text(inference.Get("resolvedModel"))
	if resolved == "" {
		resolved = parser.Text(inference.Get("requestedModel"))
	}
	provider := parser.Text(inference.Get("provider"))
	base.Model = modelName(provider, resolved)
	if base.Model == "" {
		base.Model = d.byModel[base.SessionID]
	}
	if text := d.models.Change("model", base.SessionID, resolved); text != "" {
		if provider != "" {
			text += " provider=" + provider
		}
		if requested := parser.Text(inference.Get("requestedModel")); requested != "" && requested != resolved {
			text += " requested=" + requested
		}
		d.emit(systemCopy(base, text))
	}
	return base.Model
}

// answeredOf returns the answered ids of a session, an empty set when it
// has none.
func answeredOf(answered map[string]map[string]bool, sessionID string) map[string]bool {
	if ids := answered[sessionID]; ids != nil {
		return ids
	}
	return map[string]bool{}
}
