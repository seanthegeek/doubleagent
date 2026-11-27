package antigravity

import (
	"math"
	"math/big"
	"path"
	"strconv"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/protowire"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// stepSourceUserImplicit is exa.cortex_pb.CortexStepSource's
// USER_IMPLICIT: a step the user did not type.
const stepSourceUserImplicit = 3

// enumText returns the name a column value stands for in an enum table, or
// the value as row text.
func enumText(value sqlitefile.Value, names map[int64]string) string {
	number, ok := int64(0), false
	switch {
	case value.Type == sqlitefile.Integer:
		number, ok = value.Int, true
	case value.Type == sqlitefile.Float && value.Float == math.Trunc(value.Float) &&
		math.Abs(value.Float) < 1<<62:
		number, ok = int64(value.Float), true
	}
	if name, known := names[number]; ok && known {
		return name
	}
	return value.String()
}

// blobOf returns the bytes of a blob or text column, nil for any other.
func blobOf(value sqlitefile.Value) []byte {
	if value.Type == sqlitefile.Blob || value.Type == sqlitefile.Text {
		return value.Bytes
	}
	return nil
}

// decodeMessage decodes data as the named message; false when it is
// malformed.
func (p *Parser) decodeMessage(data []byte, name string) (message, bool) {
	decoded, err := p.schema.Decode(data, name)
	if err != nil {
		return message{}, false
	}
	return message{decoded}, true
}

// openDatabase opens an artifact's database, or emits the problem row
// with the session id and returns nil.
func (p *Parser) openDatabase(artifact *input.Artifact, sessionID string, emit parserkit.Emit) *sqlitefile.Database {
	database, err := parserkit.OpenDatabase(artifact, artifact.DiskPath)
	if err != nil {
		parserkit.EmitAll(emit, p.ErrorRows(artifact, parserkit.Problems{{Message: parserkit.DatabaseError(err)}}, sessionID, ""))
		return nil
	}
	return database
}

// conversation emits the rows of a conversations/<id>.db. Its rows are
// gathered first: a database error met while reading them is one problem
// row in their place.
func (p *Parser) conversation(artifact *input.Artifact, options parserkit.Options, emit parserkit.Emit) {
	// The kind's pattern holds the .db suffix, in any ASCII case.
	base := path.Base(artifact.Rel)
	name := base[:len(base)-len(".db")]
	database := p.openDatabase(artifact, name, emit)
	if database == nil {
		return
	}
	defer database.Close()
	reader := &conversationReader{owner: p, artifact: artifact, database: database, options: options,
		sessionID: name}
	if err := reader.read(); err != nil {
		problem := parserkit.Problems{{Message: parserkit.DatabaseError(err)}}
		parserkit.EmitAll(emit, p.ErrorRows(artifact, problem, name, ""))
		return
	}
	parserkit.EmitAll(emit, reader.rows)
}

// conversationReader reads one conversation database into rows.
type conversationReader struct {
	owner     *Parser
	artifact  *input.Artifact
	database  *sqlitefile.Database
	options   parserkit.Options
	tables    parserkit.Tables
	sessionID string
	project   string
	branch    string
	source    string
	models    map[int64]string
	rows      []timeline.Row
}

// row returns a row of the conversation at a step's index, its project a
// workspace URI when it is not a file URI.
func (r *conversationReader) row(index int64) timeline.Row {
	row := r.owner.BaseRow(r.artifact)
	row.SourceLine = int(index)
	row.SessionID, row.ProjectPath, row.GitBranch = r.sessionID, r.project, r.branch
	return row
}

// add keeps a row.
func (r *conversationReader) add(row timeline.Row) {
	r.rows = append(r.rows, row)
}

// firstRow returns the first row of a table in storage order with every
// column, and whether there is one.
func (r *conversationReader) firstRow(table string, columns ...string) (sqlitefile.Row, bool, error) {
	if !r.tables.Has(table) {
		return sqlitefile.Row{}, false, nil
	}
	handle, err := r.database.Table(table)
	if err != nil {
		return sqlitefile.Row{}, false, err
	}
	return handle.Select(columns...).First()
}

// read reads the conversation: its trajectory, the models of its steps,
// then its steps.
func (r *conversationReader) read() error {
	tables, err := parserkit.TablesOf(r.database)
	if err != nil || !tables.Has("steps") {
		return err
	}
	r.tables = tables
	if err := r.trajectory(); err != nil {
		return err
	}
	if err := r.metadata(); err != nil {
		return err
	}
	if err := r.generators(); err != nil {
		return err
	}
	return r.steps()
}

// trajectory reads the conversation's id and the surface that started it.
func (r *conversationReader) trajectory() error {
	trajectory, ok, err := r.firstRow("trajectory_meta")
	if err != nil || !ok {
		return err
	}
	if cascade := trajectory.Get("cascade_id"); !cascade.IsEmpty() {
		r.sessionID = cascade.String()
	}
	if source := trajectory.Get("source"); !source.IsNull() {
		r.source = enumText(source, trajectorySources())
	}
	return nil
}

// metadata reads the conversation's workspace and branch, and keeps its
// session start row.
func (r *conversationReader) metadata() error {
	blob, ok, err := r.firstRow("trajectory_metadata_blob", "data")
	if err != nil || !ok || blob.Get("data").IsEmpty() {
		return err
	}
	meta, ok := r.owner.decodeMessage(blobOf(blob.Get("data")), "CortexTrajectoryMetadata")
	if !ok {
		r.problem("parser: the trajectory metadata could not be decoded")
	}
	workspace := message{}
	if workspaces := meta.children("workspaces"); len(workspaces) > 0 {
		workspace = workspaces[0]
	}
	folder := workspace.first("workspace_folder_absolute_uri")
	if uris := meta.list("workspace_uris"); folder == "" && len(uris) > 0 {
		folder = valueText(uris[0])
	}
	r.project, r.branch = uriPath(folder), workspace.text("branch_name")
	if !meta.set("created_at") {
		return nil
	}
	match := conversationPattern.FindStringSubmatch(r.artifact.Rel)
	row := r.row(0)
	row.TimestampUTC = meta.text("created_at")
	row.TurnType = timeline.TurnSystem
	row.Text = timeline.Compact(parserkit.JoinNonEmpty(" | ", "session start", prefixed("source=", r.source),
		prefixed("data_dir=", match[1]), prefixed("project_id=", meta.first("project_id")),
		prefixed("parent=", meta.first("parent_conversation_id"))))
	r.add(row)
	return nil
}

// prefixed returns prefix and text, or "" when text is empty.
func prefixed(prefix, text string) string {
	if text == "" {
		return ""
	}
	return prefix + text
}

// generators reads the model that generated each step.
func (r *conversationReader) generators() error {
	r.models = map[int64]string{}
	if !r.tables.Has("gen_metadata") {
		return nil
	}
	table, err := r.database.Table("gen_metadata")
	if err != nil {
		return err
	}
	bad := 0
	err = table.Select("data").Each(func(row sqlitefile.Row) error {
		generator, ok := r.owner.decodeMessage(blobOf(row.Get("data")), "CortexStepGeneratorMetadata")
		if !ok {
			bad++
			return nil
		}
		model := generator.child("chat_model").first("response_model", "model_display_name")
		for _, index := range generator.list("step_indices") {
			if number, ok := index.(*big.Int); ok && number.IsInt64() {
				r.models[number.Int64()] = model
			}
		}
		return nil
	})
	if err == nil && bad > 0 {
		r.problem("parser: the generator metadata of " + strconv.Itoa(bad) + " row(s) could not be decoded")
	}
	return err
}

// problem keeps a problem row of the conversation, at line 0.
func (r *conversationReader) problem(text string) {
	row := r.row(0)
	row.TurnType = timeline.TurnSystem
	row.Text = text
	r.add(row)
}

// steps reads every step in index order, then says how many could not be
// decoded.
func (r *conversationReader) steps() error {
	table, err := r.database.Table("steps")
	if err != nil {
		return err
	}
	columns := []string{"idx", "step_type", "status", "metadata", "step_payload"}
	if table.Has("permissions") {
		columns = append(columns, "permissions")
	}
	tracked, bad := parserkit.Tracker{}, 0
	err = table.Select(columns...).OrderBy("idx").Each(func(row sqlitefile.Row) error {
		if !r.step(row, tracked) {
			bad++
		}
		return nil
	})
	if err != nil {
		return err
	}
	if bad > 0 {
		r.problem("parser: " + strconv.Itoa(bad) + " step(s) could not be decoded")
	}
	return nil
}

// step keeps the rows of one step: a model change, the step's own rows and
// its permission grants. It returns false for a step that does not decode.
func (r *conversationReader) step(stored sqlitefile.Row, tracked parserkit.Tracker) bool {
	index := stored.Get("idx").Int
	step := message{&protowire.Message{}}
	if payload := stored.Get("step_payload"); !payload.IsEmpty() {
		decoded, ok := r.owner.decodeMessage(blobOf(payload), "Step")
		if !ok {
			return false
		}
		step = decoded
	}
	metadata := step.child("metadata")
	if !step.has("metadata") && !stored.Get("metadata").IsEmpty() {
		decoded, ok := r.owner.decodeMessage(blobOf(stored.Get("metadata")), "CortexStepMetadata")
		if !ok {
			return false
		}
		metadata = decoded
	}
	model := r.models[index]
	if change := tracked.Change("model", r.sessionID, model); change != "" {
		row := r.row(index)
		row.TimestampUTC = metadata.first("created_at", "completed_at")
		row.TurnType, row.Model, row.Text = timeline.TurnSystem, model, change
		r.add(row)
	}
	r.stepRows(index, stored, step, metadata, model)
	r.permissionRows(index, stored.Get("permissions"), step, metadata)
	return true
}
