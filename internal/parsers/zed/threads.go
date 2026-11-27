package zed

import (
	"cmp"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeline"
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
	row.TimestampUTC = cmp.Or(timestamp, context.updated)
	row.Model = context.model
	row.TurnType = turnType
	row.Text = timeline.Compact(text)
	return row
}

// threadRecords returns the rows of the threads table by updated_at, none
// when there is no such table. The rows are read in storage order and
// sorted here, so the rows read before damage come back with the error,
// which a sorted query, reading every row first, would not return.
func threadRecords(database *sqlitefile.Database) ([]sqlitefile.Row, error) {
	names, err := parserkit.TablesOf(database)
	if err != nil || !names.Has("threads") {
		return nil, err
	}
	table, err := database.Table("threads")
	if err != nil {
		return nil, err
	}
	rows, err := table.Select().All()
	// A binary comparison is the column's own order: research/zed.md gives
	// "updated_at TEXT NOT NULL" with no COLLATE clause, so it differs from
	// an ORDER BY only for a schema the agent never writes.
	slices.SortStableFunc(rows, func(left, right sqlitefile.Row) int {
		return sqlitefile.Compare(left.Get("updated_at"), right.Get("updated_at"))
	})
	return rows, err
}

// decodeThread returns the thread a row's data holds and the raw value to
// show, or an error saying why it holds none: the raw value is the text of
// a json row, decoded with replacement, or the stored bytes of a zstd row
// as lowercase hex, since they have no faithful text form. A zstd row is
// decompressed within the database's budget.
func decodeThread(artifact *input.Artifact, record sqlitefile.Row, budget *Budget) (jsonvalue.Value, string, error) {
	data := record.Get("data")
	if data.IsNull() {
		return jsonvalue.Value{}, "", errors.New("no data")
	}
	stored := data.Bytes
	if data.Type != sqlitefile.Blob && data.Type != sqlitefile.Text {
		stored = []byte(data.String())
	}
	isZstd := strings.ToLower(parserkit.NonEmptyColumn(record.Get("data_type"))) == "zstd"
	raw := stored
	var err error
	if isZstd {
		raw, err = budget.Decompress(stored)
	}
	value := jsonvalue.Value{}
	if err == nil {
		value, err = parserkit.DecodeJSONBytes(artifact, raw)
		if err == nil && !value.IsObject() {
			err = errors.New("the thread is not a JSON object")
		}
	}
	switch {
	case err == nil:
		return value, "", nil
	case isZstd:
		return jsonvalue.Value{}, hex.EncodeToString(stored), fmt.Errorf("%w; stored zstd blob as hex", err)
	}
	return jsonvalue.Value{}, textlines.ValidUTF8(stored), err
}

// threadRows emits the rows of one thread: its start, remote, model and
// grant rows, its scratch directory, its messages, its draft, then the
// problems met in its blob.
func (p *Parser) threadRows(artifact *input.Artifact, record sqlitefile.Row, budget *Budget,
	options parserkit.Options, emit parserkit.Emit) {
	context := &thread{artifact: artifact, rowid: record.Rowid,
		sessionID: parserkit.NonEmptyColumn(record.Get("id")), project: firstFolder(record.Get("folder_paths")),
		updated: parserkit.ColumnTime(record.Get("updated_at"))}
	value, raw, err := decodeThread(artifact, record, budget)
	if err != nil {
		var problems parserkit.Problems
		problems.AddRaw(int(record.Rowid), fmt.Sprintf("unparseable record %d (%v)", record.Rowid, err), raw)
		parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, context.sessionID, context.project))
		return
	}
	var problems parserkit.Problems
	p.threadState(context, record, value, &problems, emit)
	if tempDir := value.Get("sandboxed_terminal_temp_dir"); tempDir.IsString() && tempDir.StringValue() != "" {
		// Its own project path and no session id, so the thread stays
		// counted under its project.
		text := "scratch directory: " + tempDir.StringValue() +
			" (sandboxed terminal temp directory of thread " + context.sessionID +
			") (thread blob, as of updated_at)"
		emit(p.stateRow(artifact, text, context.updated, record.Rowid, "", tempDir.StringValue()))
	}
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
	parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, context.sessionID, context.project))
}

// parseThreads emits the rows of every thread of threads.db, by updated_at.
// A database error ends the file with its problem row, after the threads
// read before it; a thread past the database's decompression budget is
// one problem row.
func (p *Parser) parseThreads(artifact *input.Artifact, database *sqlitefile.Database,
	options parserkit.Options, emit parserkit.Emit) {
	records, err := threadRecords(database)
	budget := NewBudget(int64(database.PageCount())*int64(database.PageSize()), p.budgetFloor)
	for _, record := range records {
		p.threadRows(artifact, record, budget, options, emit)
	}
	if err != nil {
		emit(p.ProblemRow(artifact, parserkit.DatabaseError(err)))
	}
}
