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
//     scheduling tables. A token is never written, only whether one is
//     present.
//   - .copilot/repos/<repo>/.git/refs/copilot/checkpoints/<session>/<seq>/<event>,
//     the loose git ref the app writes into its clone of a project per
//     checkpoint: one row at the file's mtime.
//
// The model is sessions.model per session; data.db keeps no per-switch
// history. See research/copilot-desktop.md.
package copilotdesktop

import (
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
	database, message := parser.OpenDatabase(artifact.DiskPath)
	if message != "" {
		emit(p.row(artifact, "parser: "+message, ""))
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
	tables   map[string]bool
	err      error
}

// rows returns every row of table ordered by the column, then by rowid; a
// table the database lacks gives none. After a failure it gives none.
func (s *store) rows(table, order string) []sqlitefile.Row {
	if s.err != nil {
		return nil
	}
	if s.tables == nil {
		s.tables, s.err = s.database.TableNames()
		if s.err != nil {
			return nil
		}
	}
	if !s.tables[table] {
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

// text returns a column as row text (parser.ColumnText): "" for NULL.
func text(value sqlitefile.Value) string {
	return parser.ColumnText(value)
}

// dash returns a column as text, "-" when that is empty.
func dash(value sqlitefile.Value) string {
	if shown := text(value); shown != "" {
		return shown
	}
	return "-"
}

// flag returns "false" for NULL or empty text and "true" for anything else.
func flag(value sqlitefile.Value) string {
	if value.IsNull() || (value.Type == sqlitefile.Text && len(value.Bytes) == 0) {
		return "false"
	}
	return "true"
}

// projectIndex is the projects of a database by id, in the order each id
// was first met; an id that two rows share holds the later row.
type projectIndex struct {
	order []string
	byID  map[string]sqlitefile.Row
}

// newProjectIndex indexes the project rows by id.
func newProjectIndex(rows []sqlitefile.Row) projectIndex {
	index := projectIndex{byID: map[string]sqlitefile.Row{}}
	for _, row := range rows {
		projectID := text(row.Get("id"))
		if _, seen := index.byID[projectID]; !seen {
			index.order = append(index.order, projectID)
		}
		index.byID[projectID] = row
	}
	return index
}

// pathOf returns the main repository path of a project id, "" when unknown.
func (x projectIndex) pathOf(projectID string) string {
	return text(x.byID[projectID].Get("main_repo_path"))
}

// databaseRows emits the rows of every table, in this order: accounts,
// settings, model providers, projects, workspaces, sessions and schedules.
func (p *Parser) databaseRows(artifact *input.Artifact, reader *store, emit parser.Emit) {
	projects := newProjectIndex(reader.created("projects"))
	workspaces := reader.created("workspaces")
	workspaceProject := map[string]string{}
	for _, workspace := range workspaces {
		workspaceProject[text(workspace.Get("id"))] = text(workspace.Get("project_id"))
	}
	aliases := map[string]string{}
	for _, alias := range reader.created("workspace_session_aliases") {
		aliases[text(alias.Get("session_id"))] = text(alias.Get("workspace_id"))
	}
	p.accountRows(artifact, reader, emit)
	p.settingsRows(artifact, reader, emit)
	for _, provider := range reader.created("model_providers") {
		emit(p.row(artifact, "model provider: "+text(provider.Get("name"))+" type="+
			text(provider.Get("type")), parser.ColumnTime(provider.Get("created_at"))))
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
		emit(p.row(artifact, "account: "+text(account.Get("login"))+" host="+
			text(account.Get("host"))+" kind="+text(account.Get("kind"))+" default="+
			text(account.Get("is_default"))+" token_present="+flag(account.Get("access_token")),
			parser.ColumnTime(account.Get("created_at"))))
	}
}

// settingsRows emits one row per settings row, at the file's mtime.
func (p *Parser) settingsRows(artifact *input.Artifact, reader *store, emit parser.Emit) {
	mtime := parser.FileMtime(artifact)
	for _, settings := range reader.rows("settings", "id") {
		emit(p.row(artifact, "desktop settings: storage_location="+
			text(settings.Get("storage_location"))+" instructions="+
			dash(settings.Get("instructions"))+" github_token_present="+
			flag(settings.Get("github_access_token"))+" github_token_type="+
			dash(settings.Get("github_token_type"))+" (from "+artifact.Rel+", file mtime)", mtime))
	}
}

// projectRows emits one row per project.
func (p *Parser) projectRows(artifact *input.Artifact, projects projectIndex, emit parser.Emit) {
	for _, projectID := range projects.order {
		project := projects.byID[projectID]
		line := "project added: " + text(project.Get("name")) + " kind=" +
			text(project.Get("container_kind")) + " path=" + text(project.Get("main_repo_path"))
		if !project.Get("github_owner").IsNull() || !project.Get("github_repo").IsNull() {
			line += " github=" + text(project.Get("github_owner")) + "/" +
				text(project.Get("github_repo"))
		}
		line += " default_branch=" + text(project.Get("default_branch"))
		row := p.row(artifact, line, parser.ColumnTime(project.Get("created_at")))
		row.ProjectPath = text(project.Get("main_repo_path"))
		emit(row)
	}
}

// workspaceRows emits the worktree rows, then the workspace rows.
func (p *Parser) workspaceRows(artifact *input.Artifact, reader *store,
	workspaces []sqlitefile.Row, projects projectIndex, emit parser.Emit) {
	for _, worktree := range reader.created("worktrees") {
		row := p.row(artifact, "worktree: "+text(worktree.Get("path"))+" branch="+
			text(worktree.Get("branch"))+" base="+dash(worktree.Get("base_branch")),
			parser.ColumnTime(worktree.Get("created_at")))
		row.ProjectPath = projects.pathOf(text(worktree.Get("project_id")))
		emit(row)
	}
	for _, workspace := range workspaces {
		line := "workspace: " + text(workspace.Get("name")) + " type=" +
			text(workspace.Get("workspace_type")) + " branch=" + text(workspace.Get("branch"))
		for _, label := range []string{"source_pr", "created_pr"} {
			repo, number := workspace.Get(label+"_repo_full_name"), workspace.Get(label+"_number")
			if !repo.IsNull() || !number.IsNull() {
				line += " " + label + "=" + text(repo) + "#" + text(number)
			}
		}
		row := p.row(artifact, line, parser.ColumnTime(workspace.Get("created_at")))
		row.ProjectPath = projects.pathOf(text(workspace.Get("project_id")))
		emit(row)
	}
}

// sessionRows emits the session rows, the activity items and the
// checkpoints.
func (p *Parser) sessionRows(artifact *input.Artifact, reader *store,
	pathOf func(sessionID string) string, emit parser.Emit) {
	for _, session := range reader.created("sessions") {
		sessionID := text(session.Get("id"))
		created := parser.ColumnTime(session.Get("created_at"))
		build := func(line string) timeline.Row {
			row := p.row(artifact, line, created)
			row.SessionID = sessionID
			row.Model = text(session.Get("model"))
			row.ProjectPath = pathOf(sessionID)
			return row
		}
		emit(build("desktop session: " + dash(session.Get("title")) + " type=" +
			text(session.Get("session_type")) + " model=" + dash(session.Get("model")) +
			" auto_approve=" + text(session.Get("auto_approve")) + " permission_mode=" +
			dash(session.Get("permission_mode")) + " remote_control=" +
			text(session.Get("remote_control_enabled")) + " archived=" +
			flag(session.Get("archived_at"))))
		if fork := session.Get("forked_from_session_id"); !fork.IsNull() &&
			!(fork.Type == sqlitefile.Text && len(fork.Bytes) == 0) {
			emit(build("forked from session " + text(fork)))
		}
	}
	for _, activity := range reader.created("activity_items") {
		row := p.row(artifact, "activity "+text(activity.Get("activity_type"))+": "+
			text(activity.Get("preview")), parser.ColumnTime(activity.Get("created_at")))
		row.SessionID = text(activity.Get("session_id"))
		emit(row)
	}
	for _, checkpoint := range reader.created("session_checkpoints") {
		row := p.row(artifact, "checkpoint commit "+text(checkpoint.Get("commit_sha")),
			parser.ColumnTime(checkpoint.Get("created_at")))
		row.SessionID = text(checkpoint.Get("session_id"))
		emit(row)
	}
}

// scheduleRows emits the rows of the scheduled-task tables, the session
// automations and the deletion intents.
func (p *Parser) scheduleRows(artifact *input.Artifact, reader *store, emit parser.Emit) {
	for _, task := range reader.created("my_copilot_scheduled_tasks") {
		emit(p.row(artifact, "scheduled task: "+text(task.Get("title"))+" enabled="+
			text(task.Get("enabled"))+" mode="+text(task.Get("execution_mode"))+" agent="+
			text(task.Get("background_agent_type"))+" prompt: "+text(task.Get("prompt")),
			parser.ColumnTime(task.Get("created_at"))))
	}
	for _, trigger := range reader.created("my_copilot_scheduled_task_triggers") {
		emit(p.row(artifact, "scheduled task trigger: task="+text(trigger.Get("task_id"))+
			" kind="+text(trigger.Get("kind"))+" enabled="+text(trigger.Get("enabled"))+
			" next_run_at="+dash(trigger.Get("next_run_at"))+" config: "+
			text(trigger.Get("config_json")), parser.ColumnTime(trigger.Get("created_at"))))
	}
	for _, run := range reader.rows("my_copilot_scheduled_task_invocations", "claimed_at") {
		row := p.row(artifact, "scheduled task run: "+text(run.Get("task_title"))+" status="+
			text(run.Get("status"))+" source="+text(run.Get("source"))+" scheduled_for="+
			dash(run.Get("scheduled_for"))+" prompt: "+text(run.Get("prompt_snapshot")),
			parser.ColumnTime(run.Get("claimed_at")))
		row.SessionID = text(run.Get("parent_session_id"))
		emit(row)
	}
	p.automationRows(artifact, reader, emit)
}

// automationRows emits the session automation rows and the deletion
// intents.
func (p *Parser) automationRows(artifact *input.Artifact, reader *store, emit parser.Emit) {
	for _, automation := range reader.rows("session_automations", "updated_at") {
		row := p.row(artifact, "session automation: "+dash(automation.Get("name"))+
			" interval="+text(automation.Get("interval"))+" every_minutes="+
			dash(automation.Get("every_minutes"))+" next_run_at="+
			dash(automation.Get("next_run_at"))+" prompt: "+text(automation.Get("prompt")),
			parser.ColumnTime(automation.Get("updated_at")))
		row.SessionID = text(automation.Get("session_id"))
		emit(row)
	}
	for _, intent := range reader.created("session_deletion_intents") {
		row := p.row(artifact, "session deletion requested",
			parser.ColumnTime(intent.Get("created_at")))
		row.SessionID = text(intent.Get("session_id"))
		emit(row)
	}
}

// The interfaces the registry looks for, checked when the package builds.
var _ parser.Parser = (*Parser)(nil)
