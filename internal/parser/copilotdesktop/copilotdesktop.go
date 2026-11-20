// Package copilotdesktop reads the app store of GitHub Copilot Desktop.
//
// The app runs its sessions through a bundled Copilot CLI that writes them
// into .copilot/session-state/<id>/ in the CLI's own format, which the
// copilot-cli parser reads. This parser reads the app's own store only, none
// of it a secondary source, and yields system rows:
//
//   - .copilot/data.db, SQLite in WAL mode, every column possibly NULL and
//     every time ISO 8601 UTC; a missing table gives no rows. The tables are
//     accounts, settings, model_providers, projects, worktrees, workspaces,
//     sessions (the project through workspace_session_aliases and
//     workspaces.project_id), activity_items, session_checkpoints and the
//     scheduling tables. A stored GitHub token is shown through
//     parser.SecretText; a missing one is written as absent.
//   - .copilot/repos/<repo>/.git/refs/copilot/checkpoints/<session>/<seq>/<event>,
//     the loose git ref the app writes into its clone of a project per
//     checkpoint: one row at the file's mtime.
//
// The model is sessions.model per session; data.db keeps no per-switch
// history. See research/copilot-desktop.md.
package copilotdesktop

import (
	"cmp"
	"regexp"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// Parser reads Copilot Desktop's data.db and its checkpoint refs.
type Parser struct {
	parser.Base
	database, ref *regexp.Regexp
}

// New returns the parser.
func New() *Parser {
	return &Parser{
		Base:     parser.NewBase("copilot-desktop", "copilot-desktop"),
		database: regexp.MustCompile(`\A\.copilot/data\.db\z`),
		ref: regexp.MustCompile(`\A\.copilot/repos/[^/]+/\.git/refs/copilot/checkpoints/` +
			`([0-9a-f-]{36})/([0-9]+)/([0-9a-f-]{36})\z`),
	}
}

// Wants reports whether the artifact is data.db or a checkpoint ref.
func (p *Parser) Wants(artifact *input.Artifact) bool {
	return p.database.MatchString(artifact.Rel) || p.ref.MatchString(artifact.Rel)
}

// Parse emits the rows of data.db, or the one row of a checkpoint ref.
func (p *Parser) Parse(artifact *input.Artifact, _ parser.Options, emit parser.Emit) error {
	if ref := p.ref.FindStringSubmatch(artifact.Rel); ref != nil {
		emit(p.refRow(artifact, ref[1], ref[2], ref[3]))
		return nil
	}
	database, err := parser.OpenDatabase(artifact.DiskPath)
	if err != nil {
		emit(p.row(artifact, parser.ProblemPrefix+parser.DatabaseError(err), ""))
		return nil
	}
	defer database.Close()
	reader := &store{database: database}
	var rows []timeline.Row
	p.databaseRows(artifact, reader, func(row timeline.Row) { rows = append(rows, row) })
	if reader.err != nil {
		emit(p.row(artifact, "parser: "+parser.DatabaseError(reader.err), ""))
		return nil
	}
	parser.EmitAll(emit, rows)
	return nil
}

// refRow returns the row of one checkpoint ref file.
func (p *Parser) refRow(artifact *input.Artifact, sessionID, sequence, eventID string) timeline.Row {
	var problems parser.Problems
	data, ok := parser.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		return p.ErrorRows(artifact, problems, sessionID, "")[0]
	}
	number := strings.TrimLeft(sequence, "0")
	if number == "" {
		number = "0"
	}
	row := p.row(artifact, "checkpoint ref "+number+": commit "+
		strings.TrimSpace(textlines.ValidUTF8(data))+" event "+eventID+" (file mtime)",
		parser.FileMtime(artifact))
	row.SessionID = sessionID
	return row
}

// row returns a system row with the compacted text at the timestamp.
func (p *Parser) row(artifact *input.Artifact, text, timestamp string) timeline.Row {
	row := p.SystemRow(artifact, timeline.Compact(text))
	if timestamp != "" {
		row.TimestampUTC = timeutil.FromText(timestamp)
	}
	return row
}

// store reads the tables of one database, keeping the first error.
type store struct {
	database *sqlitefile.Database
	err      error
}

// rows returns every row of table ordered by the column, then by rowid; a
// table the database lacks gives none. After a failure it gives none.
func (s *store) rows(table, order string) []sqlitefile.Row {
	if s.err != nil {
		return nil
	}
	tables, err := parser.TablesOf(s.database)
	if err != nil {
		s.err = err
		return nil
	}
	if !tables.Has(table) {
		return nil
	}
	handle, err := s.database.Table(table)
	if err != nil {
		s.err = err
		return nil
	}
	rows, err := handle.Select().OrderBy(order, "rowid").All()
	if err != nil {
		s.err = err
		return nil
	}
	return rows
}

// created returns the rows of table ordered by created_at.
func (s *store) created(table string) []sqlitefile.Row {
	return s.rows(table, "created_at")
}

// dash returns a column as text (parser.ColumnText), "-" when that is
// empty.
func dash(value sqlitefile.Value) string {
	return cmp.Or(parser.ColumnText(value), "-")
}

// token returns a token column as a row shows it: <name>=<token> through
// parser.SecretText when one is stored, <name>_present=false when the
// column is NULL or empty text.
func token(artifact *input.Artifact, name string, value sqlitefile.Value) string {
	if flag(value) == "false" {
		return name + "_present=false"
	}
	return name + "=" + parser.SecretText(artifact, parser.ColumnText(value))
}

// flag returns "false" for NULL or empty text and "true" for anything else.
func flag(value sqlitefile.Value) string {
	if value.IsNull() || (value.Type == sqlitefile.Text && len(value.Bytes) == 0) {
		return "false"
	}
	return "true"
}

// projectIndex is the projects of a database by id, in the order each id
// was first met; an id that two rows share holds the later row. Each
// project's main repository path is made into a string once, since every
// workspace, worktree and session row of the project carries it and the
// rows are held until the parse ends.
type projectIndex struct {
	order []string
	byID  map[string]sqlitefile.Row
	paths map[string]string
}

// newProjectIndex indexes the project rows by id.
func newProjectIndex(rows []sqlitefile.Row) projectIndex {
	index := projectIndex{byID: map[string]sqlitefile.Row{}, paths: map[string]string{}}
	for _, row := range rows {
		projectID := parser.ColumnText(row.Get("id"))
		if _, seen := index.byID[projectID]; !seen {
			index.order = append(index.order, projectID)
		}
		index.byID[projectID] = row
		index.paths[projectID] = parser.ColumnText(row.Get("main_repo_path"))
	}
	return index
}

// pathOf returns the main repository path of a project id, "" when unknown.
func (x projectIndex) pathOf(projectID string) string {
	return x.paths[projectID]
}

// databaseRows emits the rows of every table, in this order: accounts,
// settings, model providers, projects, workspaces, sessions and schedules.
func (p *Parser) databaseRows(artifact *input.Artifact, reader *store, emit parser.Emit) {
	projects := newProjectIndex(reader.created("projects"))
	workspaces := reader.created("workspaces")
	workspaceProject := map[string]string{}
	for _, workspace := range workspaces {
		workspaceProject[parser.ColumnText(workspace.Get("id"))] = parser.ColumnText(workspace.Get("project_id"))
	}
	aliases := map[string]string{}
	for _, alias := range reader.created("workspace_session_aliases") {
		aliases[parser.ColumnText(alias.Get("session_id"))] = parser.ColumnText(alias.Get("workspace_id"))
	}
	p.accountRows(artifact, reader, emit)
	p.settingsRows(artifact, reader, emit)
	for _, provider := range reader.created("model_providers") {
		emit(p.row(artifact, "model provider: "+parser.ColumnText(provider.Get("name"))+" type="+
			parser.ColumnText(provider.Get("type")), parser.ColumnTime(provider.Get("created_at"))))
	}
	p.projectRows(artifact, projects, emit)
	p.workspaceRows(artifact, reader, workspaces, projects, emit)
	sessionPath := func(sessionID string) string {
		return projects.pathOf(workspaceProject[aliases[sessionID]])
	}
	p.sessionRows(artifact, reader, sessionPath, emit)
	p.scheduleRows(artifact, reader, emit)
}

// accountRows emits one row per account.
func (p *Parser) accountRows(artifact *input.Artifact, reader *store, emit parser.Emit) {
	for _, account := range reader.created("accounts") {
		emit(p.row(artifact, "account: "+parser.ColumnText(account.Get("login"))+" host="+
			parser.ColumnText(account.Get("host"))+" kind="+parser.ColumnText(account.Get("kind"))+" default="+
			parser.ColumnText(account.Get("is_default"))+" "+token(artifact, "token", account.Get("access_token")),
			parser.ColumnTime(account.Get("created_at"))))
	}
}

// settingsRows emits one row per settings row, at the file's mtime.
func (p *Parser) settingsRows(artifact *input.Artifact, reader *store, emit parser.Emit) {
	mtime := parser.FileMtime(artifact)
	for _, settings := range reader.rows("settings", "id") {
		emit(p.row(artifact, "desktop settings: storage_location="+
			parser.ColumnText(settings.Get("storage_location"))+" instructions="+
			dash(settings.Get("instructions"))+" "+token(artifact, "github_token", settings.Get("github_access_token"))+
			" github_token_type="+
			dash(settings.Get("github_token_type"))+" (from "+artifact.Rel+", file mtime)", mtime))
	}
}

// projectRows emits one row per project.
func (p *Parser) projectRows(artifact *input.Artifact, projects projectIndex, emit parser.Emit) {
	for _, projectID := range projects.order {
		project, path := projects.byID[projectID], projects.pathOf(projectID)
		line := "project added: " + parser.ColumnText(project.Get("name")) + " kind=" +
			parser.ColumnText(project.Get("container_kind")) + " path=" + path
		if !project.Get("github_owner").IsNull() || !project.Get("github_repo").IsNull() {
			line += " github=" + parser.ColumnText(project.Get("github_owner")) + "/" +
				parser.ColumnText(project.Get("github_repo"))
		}
		line += " default_branch=" + parser.ColumnText(project.Get("default_branch"))
		row := p.row(artifact, line, parser.ColumnTime(project.Get("created_at")))
		row.ProjectPath = path
		emit(row)
	}
}

// workspaceRows emits the worktree rows, then the workspace rows.
func (p *Parser) workspaceRows(artifact *input.Artifact, reader *store,
	workspaces []sqlitefile.Row, projects projectIndex, emit parser.Emit) {
	for _, worktree := range reader.created("worktrees") {
		row := p.row(artifact, "worktree: "+parser.ColumnText(worktree.Get("path"))+" branch="+
			parser.ColumnText(worktree.Get("branch"))+" base="+dash(worktree.Get("base_branch")),
			parser.ColumnTime(worktree.Get("created_at")))
		row.ProjectPath = projects.pathOf(parser.ColumnText(worktree.Get("project_id")))
		emit(row)
	}
	for _, workspace := range workspaces {
		line := "workspace: " + parser.ColumnText(workspace.Get("name")) + " type=" +
			parser.ColumnText(workspace.Get("workspace_type")) + " branch=" + parser.ColumnText(workspace.Get("branch"))
		for _, label := range []string{"source_pr", "created_pr"} {
			repo, number := workspace.Get(label+"_repo_full_name"), workspace.Get(label+"_number")
			if !repo.IsNull() || !number.IsNull() {
				line += " " + label + "=" + parser.ColumnText(repo) + "#" + parser.ColumnText(number)
			}
		}
		row := p.row(artifact, line, parser.ColumnTime(workspace.Get("created_at")))
		row.ProjectPath = projects.pathOf(parser.ColumnText(workspace.Get("project_id")))
		emit(row)
	}
}

// sessionRows emits the session rows, the activity items and the
// checkpoints.
func (p *Parser) sessionRows(artifact *input.Artifact, reader *store,
	pathOf func(sessionID string) string, emit parser.Emit) {
	for _, session := range reader.created("sessions") {
		sessionID := parser.ColumnText(session.Get("id"))
		created := parser.ColumnTime(session.Get("created_at"))
		build := func(line string) timeline.Row {
			row := p.row(artifact, line, created)
			row.SessionID = sessionID
			row.Model = parser.ColumnText(session.Get("model"))
			row.ProjectPath = pathOf(sessionID)
			return row
		}
		emit(build("desktop session: " + dash(session.Get("title")) + " type=" +
			parser.ColumnText(session.Get("session_type")) + " model=" + dash(session.Get("model")) +
			" auto_approve=" + parser.ColumnText(session.Get("auto_approve")) + " permission_mode=" +
			dash(session.Get("permission_mode")) + " remote_control=" +
			parser.ColumnText(session.Get("remote_control_enabled")) + " archived=" +
			flag(session.Get("archived_at"))))
		if fork := session.Get("forked_from_session_id"); !fork.IsNull() &&
			!(fork.Type == sqlitefile.Text && len(fork.Bytes) == 0) {
			emit(build("forked from session " + parser.ColumnText(fork)))
		}
	}
	for _, activity := range reader.created("activity_items") {
		row := p.row(artifact, "activity "+parser.ColumnText(activity.Get("activity_type"))+": "+
			parser.ColumnText(activity.Get("preview")), parser.ColumnTime(activity.Get("created_at")))
		row.SessionID = parser.ColumnText(activity.Get("session_id"))
		emit(row)
	}
	for _, checkpoint := range reader.created("session_checkpoints") {
		row := p.row(artifact, "checkpoint commit "+parser.ColumnText(checkpoint.Get("commit_sha")),
			parser.ColumnTime(checkpoint.Get("created_at")))
		row.SessionID = parser.ColumnText(checkpoint.Get("session_id"))
		emit(row)
	}
}

// scheduleRows emits the rows of the scheduled-task tables, the session
// automations and the deletion intents.
func (p *Parser) scheduleRows(artifact *input.Artifact, reader *store, emit parser.Emit) {
	for _, task := range reader.created("my_copilot_scheduled_tasks") {
		emit(p.row(artifact, "scheduled task: "+parser.ColumnText(task.Get("title"))+" enabled="+
			parser.ColumnText(task.Get("enabled"))+" mode="+parser.ColumnText(task.Get("execution_mode"))+" agent="+
			parser.ColumnText(task.Get("background_agent_type"))+" prompt: "+parser.ColumnText(task.Get("prompt")),
			parser.ColumnTime(task.Get("created_at"))))
	}
	for _, trigger := range reader.created("my_copilot_scheduled_task_triggers") {
		emit(p.row(artifact, "scheduled task trigger: task="+parser.ColumnText(trigger.Get("task_id"))+
			" kind="+parser.ColumnText(trigger.Get("kind"))+" enabled="+parser.ColumnText(trigger.Get("enabled"))+
			" next_run_at="+dash(trigger.Get("next_run_at"))+" config: "+
			parser.ColumnText(trigger.Get("config_json")), parser.ColumnTime(trigger.Get("created_at"))))
	}
	for _, run := range reader.rows("my_copilot_scheduled_task_invocations", "claimed_at") {
		row := p.row(artifact, "scheduled task run: "+parser.ColumnText(run.Get("task_title"))+" status="+
			parser.ColumnText(run.Get("status"))+" source="+parser.ColumnText(run.Get("source"))+" scheduled_for="+
			dash(run.Get("scheduled_for"))+" prompt: "+parser.ColumnText(run.Get("prompt_snapshot")),
			parser.ColumnTime(run.Get("claimed_at")))
		row.SessionID = parser.ColumnText(run.Get("parent_session_id"))
		emit(row)
	}
	p.automationRows(artifact, reader, emit)
}

// automationRows emits the session automation rows and the deletion
// intents.
func (p *Parser) automationRows(artifact *input.Artifact, reader *store, emit parser.Emit) {
	for _, automation := range reader.rows("session_automations", "updated_at") {
		row := p.row(artifact, "session automation: "+dash(automation.Get("name"))+
			" interval="+parser.ColumnText(automation.Get("interval"))+" every_minutes="+
			dash(automation.Get("every_minutes"))+" next_run_at="+
			dash(automation.Get("next_run_at"))+" prompt: "+parser.ColumnText(automation.Get("prompt")),
			parser.ColumnTime(automation.Get("updated_at")))
		row.SessionID = parser.ColumnText(automation.Get("session_id"))
		emit(row)
	}
	for _, intent := range reader.created("session_deletion_intents") {
		row := p.row(artifact, "session deletion requested",
			parser.ColumnTime(intent.Get("created_at")))
		row.SessionID = parser.ColumnText(intent.Get("session_id"))
		emit(row)
	}
}

// The interfaces the registry looks for, checked when the package builds.
var _ parser.Parser = (*Parser)(nil)
