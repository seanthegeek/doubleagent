// Package chatgptdesktop reads the ChatGPT desktop app's list of account
// conversations.
//
// The app keeps .codex/sqlite/codex-dev.db (SQLite in WAL mode, its recent
// rows in the -wal sidecar), whose table local_thread_catalog lists the
// account's conversations. A ChatGPT-mode conversation leaves no transcript
// on disk, only the row the app writes at sign-in, whose source_kind is
// "chatgpt"; the other rows are the app's Codex threads, whose rollouts the
// codex-cli parser reads, and are skipped. The catalog attributes .codex to
// codex-cli, so this parser reads that agent's artifacts too.
//
// Columns read, each possibly NULL: thread_id, display_title,
// source_created_at, source_updated_at (REAL epoch seconds),
// conversation_origin, host_id and source_kind. Each conversation gives three
// system rows at source_updated_at, else source_created_at: the "account
// conversation:" row, a "session origin: remote" row and a "session synced:"
// row. A file that is not SQLite, a SQLite error, or a database without the
// table is one `parser:` row. See research/chatgpt-desktop.md.
package chatgptdesktop

import (
	"fmt"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// The database's path relative to the home, and the table read.
const (
	databaseRel  = ".codex/sqlite/codex-dev.db"
	catalogTable = "local_thread_catalog"
)

// Parser reads the conversation list of the ChatGPT desktop app.
type Parser struct {
	parser.Base
}

// New returns the parser.
func New() *Parser {
	return &Parser{Base: parser.NewBase("chatgpt-desktop", "chatgpt-desktop")}
}

// ReadsAgents names codex-cli, the agent the catalog gives .codex to.
func (p *Parser) ReadsAgents() []string {
	return []string{"codex-cli"}
}

// Marks says the parser records synced sessions and sessions of remote
// origin.
func (p *Parser) Marks() parser.Marks {
	return parser.Marks{Sync: true, Origin: true}
}

// Wants reports whether the artifact is .codex/sqlite/codex-dev.db.
func (p *Parser) Wants(artifact *input.Artifact) bool {
	return artifact.Rel == databaseRel
}

// Parse emits three system rows per ChatGPT conversation of the table.
func (p *Parser) Parse(artifact *input.Artifact, _ parser.Options, emit parser.Emit) error {
	records, message := readCatalog(artifact.DiskPath)
	if message != "" {
		emit(p.ProblemRow(artifact, message))
		return nil
	}
	for _, record := range records {
		if kind := record.Get("source_kind"); kind.Type == sqlitefile.Text && kind.Text() == "chatgpt" {
			p.conversationRows(artifact, record, emit)
		}
	}
	return nil
}

// readCatalog returns every row of the catalog table in rowid order, or the
// message of the problem row that replaces them.
func readCatalog(path string) ([]sqlitefile.Row, string) {
	database, message := parser.OpenDatabase(path)
	if message != "" {
		return nil, message
	}
	defer database.Close()
	names, err := database.TableNames()
	if err != nil {
		return nil, parser.DatabaseError(err)
	}
	if !names[catalogTable] {
		return nil, "no " + catalogTable + " table"
	}
	table, err := database.Table(catalogTable)
	if err != nil {
		return nil, parser.DatabaseError(err)
	}
	records, err := table.Select().OrderBy("rowid").All()
	if err != nil {
		return nil, parser.DatabaseError(err)
	}
	return records, ""
}

// conversationRows emits the three rows of one conversation.
func (p *Parser) conversationRows(artifact *input.Artifact, record sqlitefile.Row, emit parser.Emit) {
	threadID := ""
	if thread := record.Get("thread_id"); thread.Truthy() {
		threadID = thread.Str()
	}
	created := parser.ColumnTime(record.Get("source_created_at"))
	updated := parser.ColumnTime(record.Get("source_updated_at"))
	texts := []string{
		fmt.Sprintf("account conversation: %s origin=%s created=%s updated=%s host=%s (from %s %s)",
			shown(record.Get("display_title")), shown(record.Get("conversation_origin")),
			orDash(created), orDash(updated), shown(record.Get("host_id")), databaseRel,
			catalogTable),
		"session origin: remote chatgpt.com conversation " + orDash(threadID) +
			" (listed at sign-in; its transcript is server-side)",
		"session synced: chatgpt.com holds conversation " + orDash(threadID) +
			", no local transcript",
	}
	for _, text := range texts {
		row := p.SystemRow(artifact, text)
		row.SessionID = threadID
		row.TimestampUTC = timeutil.FirstNonEmpty(updated, created)
		emit(row)
	}
}

// shown returns a column as row text, "-" for NULL or empty text.
func shown(value sqlitefile.Value) string {
	if value.IsNull() || (value.Type == sqlitefile.Text && len(value.Bytes) == 0) {
		return "-"
	}
	return value.Str()
}

// orDash returns text, or "-" when it is empty.
func orDash(text string) string {
	if text == "" {
		return "-"
	}
	return text
}

// The interfaces the registry looks for, checked when the package builds.
var (
	_ parser.Parser      = (*Parser)(nil)
	_ parser.AgentReader = (*Parser)(nil)
	_ parser.Marker      = (*Parser)(nil)
)
