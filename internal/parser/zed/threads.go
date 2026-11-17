package zed

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// sandboxFlags are the sandbox escapes of a thread's grants, each a row
// when it is set.
func sandboxFlags() []string {
	return []string{"network_any_host", "allow_fs_write_all", "unsandboxed", "sandbox_fallback"}
}

// thread is the context every row of one thread shares.
type thread struct {
	artifact  *input.Artifact
	rowid     int64
	sessionID string
	project   string
	updated   string
	model     string
	branch    string
}

// row returns a row of the thread with the text compacted, at timestamp or
// else the thread's updated_at.
func (p *Parser) threadRow(context *thread, turnType, text, timestamp string) timeline.Row {
	row := p.BaseRow(context.artifact)
	row.SourceLine = int(context.rowid)
	row.SessionID = context.sessionID
	row.ProjectPath = context.project
	row.GitBranch = context.branch
	row.TimestampUTC = timeutil.FirstNonEmpty(timestamp, context.updated)
	row.Model = context.model
	row.TurnType = turnType
	row.Text = timeline.Compact(text)
	return row
}

// parseThreads emits the rows of every thread of threads.db, by updated_at.
// A database error after some threads ends the file with its problem row.
func (p *Parser) parseThreads(artifact *input.Artifact, database *sqlitefile.Database,
	options parser.Options, emit parser.Emit) {
	records, err := threadRecords(database)
	for _, record := range records {
		p.threadRows(artifact, record, options, emit)
	}
	if err != nil {
		emit(p.ProblemRow(artifact, parser.DatabaseError(err)))
	}
}

// threadRecords returns the rows of the threads table by updated_at, none
// when there is no such table.
func threadRecords(database *sqlitefile.Database) ([]sqlitefile.Row, error) {
	names, err := database.TableNames()
	if err != nil || !names["threads"] {
		return nil, err
	}
	table, err := database.Table("threads")
	if err != nil {
		return nil, err
	}
	return table.Select().OrderBy("updated_at").All()
}

// decodeThread returns the thread a row's data holds, or the reason it
// holds none and the raw value to show: the text of a json row, decoded
// with replacement, or the stored bytes of a zstd row as lowercase hex,
// since they have no faithful text form.
func decodeThread(record sqlitefile.Row) (jsonvalue.Value, string, string) {
	data := record.Get("data")
	if data.IsNull() {
		return jsonvalue.Value{}, "no data", ""
	}
	stored := data.Bytes
	if data.Type != sqlitefile.Blob && data.Type != sqlitefile.Text {
		stored = []byte(data.String())
	}
	isZstd := strings.ToLower(columnStr(record.Get("data_type"))) == "zstd"
	raw, failure := stored, ""
	if isZstd {
		var err error
		if raw, err = Decompress(stored); err != nil {
			failure = err.Error()
		}
	}
	value := jsonvalue.Value{}
	if failure == "" {
		var err error
		value, err = jsonvalue.Decode(raw)
		switch {
		case err != nil:
			failure = err.Error()
		case !value.IsObject():
			failure = "the thread is not a JSON object"
		}
	}
	switch {
	case failure == "":
		return value, "", ""
	case isZstd:
		return jsonvalue.Value{}, failure + "; stored zstd blob as hex", hex.EncodeToString(stored)
	}
	return jsonvalue.Value{}, failure, textlines.ValidUTF8(stored)
}

// threadRows emits the rows of one thread: its start, remote, model and
// grant rows, its scratch directory, its messages, its draft, then the
// problems met in its blob.
func (p *Parser) threadRows(artifact *input.Artifact, record sqlitefile.Row,
	options parser.Options, emit parser.Emit) {
	context := &thread{artifact: artifact, rowid: record.Rowid,
		sessionID: columnStr(record.Get("id")), project: firstFolder(record.Get("folder_paths")),
		updated: parser.ColumnTime(record.Get("updated_at"))}
	value, reason, raw := decodeThread(record)
	if reason != "" {
		var problems parser.Problems
		problems.AddRaw(int(record.Rowid), fmt.Sprintf("unparseable record %d (%s)", record.Rowid,
			reason), raw)
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, context.sessionID, context.project))
		return
	}
	p.threadState(context, record, value, emit)
	if tempDir := value.Get("sandboxed_terminal_temp_dir"); tempDir.IsString() && tempDir.StringValue() != "" {
		// Its own project path and no session id, so the thread stays
		// counted under its project.
		text := fmt.Sprintf("scratch directory: %s (sandboxed terminal temp directory of thread %s)"+
			" (thread blob, as of updated_at)", tempDir.StringValue(), context.sessionID)
		emit(p.stateRow(artifact, text, context.updated, record.Rowid, "", tempDir.StringValue()))
	}
	var problems parser.Problems
	for _, item := range p.messages(value, record.Rowid, &problems) {
		if item.turnType == timeline.TurnThinking && !options.IncludeThinking {
			continue
		}
		if item.text == "" && item.turnType != timeline.TurnToolUse &&
			item.turnType != timeline.TurnToolResult {
			continue
		}
		row := p.threadRow(context, item.turnType, item.text, "")
		row.ToolName, row.ToolUseID = item.toolName, item.toolUseID
		emit(row)
	}
	if draft := value.Get("draft_prompt"); !draft.IsEmpty() {
		if text := acpBlocksText(draft); strings.TrimSpace(text) != "" {
			emit(p.threadRow(context, timeline.TurnSystem,
				"unsent draft: "+text+" (thread blob, as of updated_at)", ""))
		}
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, context.sessionID, context.project))
}
