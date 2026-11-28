package chatgptdesktop

import (
	"cmp"
	"errors"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/reason"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
)

// The database's path relative to the home, and the table read.
const (
	databaseRel  = ".codex/sqlite/codex-dev.db"
	catalogTable = "local_thread_catalog"
)

// Parser reads the conversation list of the ChatGPT desktop app.
type Parser struct {
	parserkit.Base
}

// New returns the parser.
func New() *Parser {
	return &Parser{Base: parserkit.NewBase("chatgpt-desktop", "chatgpt-desktop")}
}

// ReadsAgents names codex-cli, the agent the catalog gives .codex to.
func (p *Parser) ReadsAgents() []string {
	return []string{"codex-cli"}
}

// Marks says the parser records synced sessions and sessions of remote
// origin.
func (p *Parser) Marks() parserkit.Marks {
	return parserkit.Marks{Sync: true, Origin: true}
}

// Kinds returns the app database.
func (p *Parser) Kinds() []parserkit.Kind {
	return []parserkit.Kind{parserkit.RelKind("app database", databaseRel)}
}

// Wants reports whether the artifact is .codex/sqlite/codex-dev.db.
func (p *Parser) Wants(artifact *input.Artifact) bool {
	return parserkit.WantsAny(p.Kinds(), artifact)
}

// readCatalog returns every row of the catalog table in rowid order, or an
// error whose text is the message of the problem row that replaces them.
func readCatalog(artifact *input.Artifact, path string) ([]sqlitefile.Row, error) {
	database, err := parserkit.OpenDatabase(artifact, path)
	if err != nil {
		return nil, reason.Wrap(parserkit.DatabaseError(err), err)
	}
	defer database.Close()
	names, err := parserkit.TablesOf(database)
	if err != nil {
		return nil, reason.Wrap(parserkit.DatabaseError(err), err)
	}
	if !names.Has(catalogTable) {
		return nil, errors.New("no " + catalogTable + " table")
	}
	table, err := database.Table(catalogTable)
	if err != nil {
		return nil, reason.Wrap(parserkit.DatabaseError(err), err)
	}
	records, err := table.Select().OrderBy("rowid").All()
	if err != nil {
		return nil, reason.Wrap(parserkit.DatabaseError(err), err)
	}
	return records, nil
}

// shown returns a column as row text, "-" for NULL or empty text.
func shown(value sqlitefile.Value) string {
	if value.IsNull() || (value.Type == sqlitefile.Text && len(value.Bytes) == 0) {
		return "-"
	}
	return value.String()
}

// conversationRows emits the three rows of one conversation.
func (p *Parser) conversationRows(artifact *input.Artifact, record sqlitefile.Row, emit parserkit.Emit) {
	threadID := ""
	if thread := record.Get("thread_id"); !thread.IsEmpty() {
		threadID = thread.String()
	}
	created := parserkit.ColumnTime(record.Get("source_created_at"))
	updated := parserkit.ColumnTime(record.Get("source_updated_at"))
	texts := []string{
		"account conversation: " + shown(record.Get("display_title")) +
			" origin=" + shown(record.Get("conversation_origin")) +
			" created=" + cmp.Or(created, "-") + " updated=" + cmp.Or(updated, "-") +
			" host=" + shown(record.Get("host_id")) +
			" (from " + databaseRel + " " + catalogTable + ")",
		"session origin: remote chatgpt.com conversation " + cmp.Or(threadID, "-") +
			" (listed at sign-in; its transcript is server-side)",
		"session synced: chatgpt.com holds conversation " + cmp.Or(threadID, "-") +
			", no local transcript",
	}
	for _, text := range texts {
		row := p.SystemRow(artifact, text)
		row.SessionID = threadID
		row.TimestampUTC = cmp.Or(updated, created)
		emit(row)
	}
}

// Parse emits three system rows per ChatGPT conversation of the table.
func (p *Parser) Parse(artifact *input.Artifact, _ parserkit.Options, emit parserkit.Emit) error {
	records, err := readCatalog(artifact, artifact.DiskPath)
	if err != nil {
		emit(p.ProblemRow(artifact, err.Error()))
		return nil
	}
	for _, record := range records {
		if kind := record.Get("source_kind"); kind.Type == sqlitefile.Text && kind.Text() == "chatgpt" {
			p.conversationRows(artifact, record, emit)
		}
	}
	return nil
}

// The interfaces the registry looks for, checked when the package builds.
var (
	_ parserkit.Parser      = (*Parser)(nil)
	_ parserkit.AgentReader = (*Parser)(nil)
	_ parserkit.Marker      = (*Parser)(nil)
)
