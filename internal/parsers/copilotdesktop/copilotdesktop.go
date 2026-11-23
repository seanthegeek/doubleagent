package copilotdesktop

import (
	"cmp"
	"regexp"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// Parser reads Copilot Desktop's data.db and its checkpoint refs.
type Parser struct {
	parserkit.Base
	database, ref *regexp.Regexp
}

// New returns the parser.
func New() *Parser {
	return &Parser{
		Base:     parserkit.NewBase("copilot-desktop", "copilot-desktop"),
		database: regexp.MustCompile(`\A\.copilot/data\.db\z`),
		ref: regexp.MustCompile(`\A\.copilot/repos/[^/]+/\.git/refs/copilot/checkpoints/` +
			`([0-9a-f-]{36})/([0-9]+)/([0-9a-f-]{36})\z`),
	}
}

// Wants reports whether the artifact is data.db or a checkpoint ref.
func (p *Parser) Wants(artifact *input.Artifact) bool {
	return parserkit.WantsAny(p.Kinds(), artifact)
}

// Kinds returns the app database and the checkpoint refs.
func (p *Parser) Kinds() []parserkit.Kind {
	return []parserkit.Kind{parserkit.PatternKind("data.db", p.database), parserkit.PatternKind("checkpoint ref", p.ref)}
}

// Parse emits the rows of data.db, or the one row of a checkpoint ref.
func (p *Parser) Parse(artifact *input.Artifact, _ parserkit.Options, emit parserkit.Emit) error {
	if ref := p.ref.FindStringSubmatch(artifact.Rel); ref != nil {
		emit(p.refRow(artifact, ref[1], ref[2], ref[3]))
		return nil
	}
	database, err := parserkit.OpenDatabase(artifact.DiskPath)
	if err != nil {
		emit(p.row(artifact, parserkit.ProblemPrefix+parserkit.DatabaseError(err), ""))
		return nil
	}
	defer database.Close()
	reader := &store{database: database}
	var rows []timeline.Row
	p.databaseRows(artifact, reader, func(row timeline.Row) { rows = append(rows, row) })
	if reader.err != nil {
		emit(p.row(artifact, "parser: "+parserkit.DatabaseError(reader.err), ""))
		return nil
	}
	parserkit.EmitAll(emit, rows)
	return nil
}

// refRow returns the row of one checkpoint ref file.
func (p *Parser) refRow(artifact *input.Artifact, sessionID, sequence, eventID string) timeline.Row {
	var problems parserkit.Problems
	data, ok := parserkit.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		return p.ErrorRows(artifact, problems, sessionID, "")[0]
	}
	number := strings.TrimLeft(sequence, "0")
	if number == "" {
		number = "0"
	}
	row := p.row(artifact, "checkpoint ref "+number+": commit "+
		strings.TrimSpace(textlines.ValidUTF8(data))+" event "+eventID+" (file mtime)",
		parserkit.FileMtime(artifact))
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
	tables, err := parserkit.TablesOf(s.database)
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

// keyed returns the rows of table ordered by created_at, keyed by column
// under the collation the table declares for it.
func (s *store) keyed(table, column string) keyed {
	found := keyed{rows: s.created(table), column: column}
	if len(found.rows) > 0 {
		handle, err := s.database.Table(table)
		if err != nil {
			s.err = err
			return found
		}
		found.collation = parserkit.KeyCollation(handle, column)
	}
	return found
}

// dash returns a column as text (parserkit.ColumnText), "-" when that is
// empty.
func dash(value sqlitefile.Value) string {
	return cmp.Or(parserkit.ColumnText(value), "-")
}

// token returns a token column as a row shows it: <name>=<token> through
// parserkit.SecretText when one is stored, <name>_present=false when the
// column is NULL or empty text.
func token(artifact *input.Artifact, name string, value sqlitefile.Value) string {
	if flag(value) == "false" {
		return name + "_present=false"
	}
	return name + "=" + parserkit.SecretText(artifact, parserkit.ColumnText(value))
}

// flag returns "false" for NULL or empty text and "true" for anything else.
func flag(value sqlitefile.Value) string {
	if value.IsNull() || (value.Type == sqlitefile.Text && len(value.Bytes) == 0) {
		return "false"
	}
	return "true"
}

// projectIndex is the stored rows of the projects table in reading order,
// with each row's main repository path, made into a string once, since
// every workspace, worktree and session row of the project carries it and
// the rows are held until the parse ends. An id names its first stored
// row; an id more than one row holds has a repeat.
type projectIndex struct {
	stored  keyed
	paths   []string
	first   map[string]int
	repeats map[string]*repeat
}

// newProjectIndex indexes the project rows by id.
func newProjectIndex(projects keyed) projectIndex {
	index := projectIndex{stored: projects, paths: make([]string, len(projects.rows)), first: map[string]int{},
		repeats: repeatsOf("projects", "worktrees, workspaces and sessions", projects)}
	for i, row := range projects.rows {
		key := projects.key(row.Get("id"))
		if _, met := index.first[key]; !met && key != "" {
			index.first[key] = i
		}
		index.paths[i] = parserkit.ColumnText(row.Get("main_repo_path"))
	}
	return index
}

// pathOf returns the main repository path of a project id's first stored
// row, "" when no row holds the id or it is NULL.
func (x projectIndex) pathOf(projectID sqlitefile.Value) string {
	if i, ok := x.first[x.stored.key(projectID)]; ok {
		return x.paths[i]
	}
	return ""
}

// databaseRows emits the rows of every table, in this order: accounts,
// settings, model providers, projects, workspaces, sessions and schedules.
func (p *Parser) databaseRows(artifact *input.Artifact, reader *store, emit parserkit.Emit) {
	projects := newProjectIndex(reader.keyed("projects", "id"))
	joined := newLinks(projects, reader.keyed("workspaces", "id"),
		reader.keyed("workspace_session_aliases", "session_id"))
	p.accountRows(artifact, reader, emit)
	p.settingsRows(artifact, reader, emit)
	for _, provider := range reader.created("model_providers") {
		emit(p.row(artifact, "model provider: "+parserkit.ColumnText(provider.Get("name"))+" type="+
			parserkit.ColumnText(provider.Get("type")), parserkit.ColumnTime(provider.Get("created_at"))))
	}
	p.projectRows(artifact, projects, emit)
	p.workspaceRows(artifact, reader, joined, emit)
	p.aliasRows(artifact, joined, emit)
	p.sessionRows(artifact, reader, joined, emit)
	p.scheduleRows(artifact, reader, emit)
}

// accountRows emits one row per account.
func (p *Parser) accountRows(artifact *input.Artifact, reader *store, emit parserkit.Emit) {
	for _, account := range reader.created("accounts") {
		emit(p.row(artifact, "account: "+parserkit.ColumnText(account.Get("login"))+" host="+
			parserkit.ColumnText(account.Get("host"))+" kind="+parserkit.ColumnText(account.Get("kind"))+" default="+
			parserkit.ColumnText(account.Get("is_default"))+" "+token(artifact, "token", account.Get("access_token")),
			parserkit.ColumnTime(account.Get("created_at"))))
	}
}

// settingsRows emits one row per settings row, at the file's mtime.
func (p *Parser) settingsRows(artifact *input.Artifact, reader *store, emit parserkit.Emit) {
	mtime := parserkit.FileMtime(artifact)
	for _, settings := range reader.rows("settings", "id") {
		emit(p.row(artifact, "desktop settings: storage_location="+
			parserkit.ColumnText(settings.Get("storage_location"))+" instructions="+
			dash(settings.Get("instructions"))+" "+token(artifact, "github_token", settings.Get("github_access_token"))+
			" github_token_type="+
			dash(settings.Get("github_token_type"))+" (from "+artifact.Rel+", file mtime)", mtime))
	}
}

// projectRows emits one row per stored project row, each row of an id
// more than one row holds followed by its `parser:` row.
func (p *Parser) projectRows(artifact *input.Artifact, projects projectIndex, emit parserkit.Emit) {
	for i, project := range projects.stored.rows {
		path := projects.paths[i]
		line := "project added: " + parserkit.ColumnText(project.Get("name")) + " kind=" +
			parserkit.ColumnText(project.Get("container_kind")) + " path=" + path
		if !project.Get("github_owner").IsNull() || !project.Get("github_repo").IsNull() {
			line += " github=" + parserkit.ColumnText(project.Get("github_owner")) + "/" +
				parserkit.ColumnText(project.Get("github_repo"))
		}
		line += " default_branch=" + parserkit.ColumnText(project.Get("default_branch"))
		row := p.row(artifact, line, parserkit.ColumnTime(project.Get("created_at")))
		row.ProjectPath = path
		notices := projects.repeats[projects.stored.key(project.Get("id"))].start(&row)
		emit(row)
		parserkit.EmitAll(emit, notices)
	}
}

// workspaceRows emits the worktree rows, then the workspace rows. A row
// naming a project id more than one row holds is a member of its group; a
// stored row of a workspace id more than one row holds is a parent of the
// workspace's group instead, followed by its `parser:` row.
func (p *Parser) workspaceRows(artifact *input.Artifact, reader *store, joined links, emit parserkit.Emit) {
	for _, worktree := range reader.created("worktrees") {
		row := p.row(artifact, "worktree: "+parserkit.ColumnText(worktree.Get("path"))+" branch="+
			parserkit.ColumnText(worktree.Get("branch"))+" base="+dash(worktree.Get("base_branch")),
			parserkit.ColumnTime(worktree.Get("created_at")))
		path, project := joined.projectPlace(worktree.Get("project_id"))
		row.ProjectPath = path
		project.member(&row)
		emit(row)
	}
	for _, workspace := range joined.workspaces.rows {
		line := "workspace: " + parserkit.ColumnText(workspace.Get("name")) + " type=" +
			parserkit.ColumnText(workspace.Get("workspace_type")) + " branch=" + parserkit.ColumnText(workspace.Get("branch"))
		for _, label := range []string{"source_pr", "created_pr"} {
			repo, number := workspace.Get(label+"_repo_full_name"), workspace.Get(label+"_number")
			if !repo.IsNull() || !number.IsNull() {
				line += " " + label + "=" + parserkit.ColumnText(repo) + "#" + parserkit.ColumnText(number)
			}
		}
		row := p.row(artifact, line, parserkit.ColumnTime(workspace.Get("created_at")))
		path, project := joined.projectPlace(workspace.Get("project_id"))
		row.ProjectPath = path
		var notices []timeline.Row
		if repeated := joined.workspaceRepeat(workspace); repeated != nil {
			notices = repeated.start(&row)
		} else {
			project.member(&row)
		}
		emit(row)
		parserkit.EmitAll(emit, notices)
	}
}

// sessionRows emits the session rows, the activity items and the
// checkpoints. A session row whose project path is joined through a
// repeated key is a member of that key's group (links.sessionPlace).
func (p *Parser) sessionRows(artifact *input.Artifact, reader *store, joined links, emit parserkit.Emit) {
	for _, session := range reader.created("sessions") {
		sessionID := parserkit.ColumnText(session.Get("id"))
		created := parserkit.ColumnTime(session.Get("created_at"))
		build := func(line string) timeline.Row {
			row := p.row(artifact, line, created)
			row.SessionID = sessionID
			row.Model = parserkit.ColumnText(session.Get("model"))
			path, place := joined.sessionPlace(session.Get("id"))
			row.ProjectPath = path
			place.member(&row)
			return row
		}
		emit(build("desktop session: " + dash(session.Get("title")) + " type=" +
			parserkit.ColumnText(session.Get("session_type")) + " model=" + dash(session.Get("model")) +
			" auto_approve=" + parserkit.ColumnText(session.Get("auto_approve")) + " permission_mode=" +
			dash(session.Get("permission_mode")) + " remote_control=" +
			parserkit.ColumnText(session.Get("remote_control_enabled")) + " archived=" +
			flag(session.Get("archived_at"))))
		if fork := session.Get("forked_from_session_id"); !fork.IsNull() &&
			!(fork.Type == sqlitefile.Text && len(fork.Bytes) == 0) {
			emit(build("forked from session " + parserkit.ColumnText(fork)))
		}
	}
	for _, activity := range reader.created("activity_items") {
		row := p.row(artifact, "activity "+parserkit.ColumnText(activity.Get("activity_type"))+": "+
			parserkit.ColumnText(activity.Get("preview")), parserkit.ColumnTime(activity.Get("created_at")))
		row.SessionID = parserkit.ColumnText(activity.Get("session_id"))
		emit(row)
	}
	for _, checkpoint := range reader.created("session_checkpoints") {
		row := p.row(artifact, "checkpoint commit "+parserkit.ColumnText(checkpoint.Get("commit_sha")),
			parserkit.ColumnTime(checkpoint.Get("created_at")))
		row.SessionID = parserkit.ColumnText(checkpoint.Get("session_id"))
		emit(row)
	}
}

// scheduleRows emits the rows of the scheduled-task tables, the session
// automations and the deletion intents.
func (p *Parser) scheduleRows(artifact *input.Artifact, reader *store, emit parserkit.Emit) {
	for _, task := range reader.created("my_copilot_scheduled_tasks") {
		emit(p.row(artifact, "scheduled task: "+parserkit.ColumnText(task.Get("title"))+" enabled="+
			parserkit.ColumnText(task.Get("enabled"))+" mode="+parserkit.ColumnText(task.Get("execution_mode"))+" agent="+
			parserkit.ColumnText(task.Get("background_agent_type"))+" prompt: "+parserkit.ColumnText(task.Get("prompt")),
			parserkit.ColumnTime(task.Get("created_at"))))
	}
	for _, trigger := range reader.created("my_copilot_scheduled_task_triggers") {
		emit(p.row(artifact, "scheduled task trigger: task="+parserkit.ColumnText(trigger.Get("task_id"))+
			" kind="+parserkit.ColumnText(trigger.Get("kind"))+" enabled="+parserkit.ColumnText(trigger.Get("enabled"))+
			" next_run_at="+dash(trigger.Get("next_run_at"))+" config: "+
			parserkit.ColumnText(trigger.Get("config_json")), parserkit.ColumnTime(trigger.Get("created_at"))))
	}
	for _, run := range reader.rows("my_copilot_scheduled_task_invocations", "claimed_at") {
		row := p.row(artifact, "scheduled task run: "+parserkit.ColumnText(run.Get("task_title"))+" status="+
			parserkit.ColumnText(run.Get("status"))+" source="+parserkit.ColumnText(run.Get("source"))+" scheduled_for="+
			dash(run.Get("scheduled_for"))+" prompt: "+parserkit.ColumnText(run.Get("prompt_snapshot")),
			parserkit.ColumnTime(run.Get("claimed_at")))
		row.SessionID = parserkit.ColumnText(run.Get("parent_session_id"))
		emit(row)
	}
	p.automationRows(artifact, reader, emit)
}

// automationRows emits the session automation rows and the deletion
// intents.
func (p *Parser) automationRows(artifact *input.Artifact, reader *store, emit parserkit.Emit) {
	for _, automation := range reader.rows("session_automations", "updated_at") {
		row := p.row(artifact, "session automation: "+dash(automation.Get("name"))+
			" interval="+parserkit.ColumnText(automation.Get("interval"))+" every_minutes="+
			dash(automation.Get("every_minutes"))+" next_run_at="+
			dash(automation.Get("next_run_at"))+" prompt: "+parserkit.ColumnText(automation.Get("prompt")),
			parserkit.ColumnTime(automation.Get("updated_at")))
		row.SessionID = parserkit.ColumnText(automation.Get("session_id"))
		emit(row)
	}
	for _, intent := range reader.created("session_deletion_intents") {
		row := p.row(artifact, "session deletion requested",
			parserkit.ColumnTime(intent.Get("created_at")))
		row.SessionID = parserkit.ColumnText(intent.Get("session_id"))
		emit(row)
	}
}

// The interfaces the registry looks for, checked when the package builds.
var _ parserkit.Parser = (*Parser)(nil)
