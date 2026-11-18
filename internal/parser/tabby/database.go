package tabby

import (
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// tableColumns are the only columns ever read. No secret-bearing column is
// listed: of the tables holding one, users and integrations are read
// without it.
func tableColumns() map[string][]string {
	return map[string][]string{
		"users":   {"id", "email", "name"},
		"threads": {"id", "is_ephemeral", "user_id", "created_at", "updated_at", "relevant_questions"},
		"thread_messages": {"id", "thread_id", "role", "content", "code_source_id", "attachment",
			"code_attachments", "client_code_attachments", "doc_attachments", "created_at"},
		"user_events":   {"id", "user_id", "kind", "created_at", "payload"},
		"pages":         {"id", "author_id", "title", "content", "code_source_id", "created_at"},
		"page_sections": {"id", "page_id", "title", "content", "position", "attachment", "created_at"},
		// integrations.access_token is never selected.
		"integrations":          {"id", "kind", "display_name", "api_base", "created_at"},
		"provided_repositories": {"id", "integration_id", "name", "git_url", "active", "updated_at"},
	}
}

// integrationHosts are the hosts the source-host integrations of each kind
// talk to; self-hosted kinds take the host of their api_base.
func integrationHosts() map[string]string {
	return map[string]string{"github": "github.com", "gitlab": "gitlab.com"}
}

// store reads one database for the rows of one artifact.
type store struct {
	parser   *Parser
	artifact *input.Artifact
	database *sqlitefile.Database
	tables   map[string]bool
	users    map[string]string
	emit     parser.Emit
}

// parseDatabase emits the rows of a database or a backup of one: a backup
// first says what it is; then threads, pages, user events and
// integrations. A database error ends the rows with one `parser:` row; the
// rows before it stay.
func (p *Parser) parseDatabase(artifact *input.Artifact, backup bool, emit parser.Emit) {
	if err := sqlitefile.Problem(artifact.DiskPath); err != nil {
		emit(p.row(artifact, rowSpec{text: parser.ProblemPrefix + parser.DatabaseError(err)}))
		return
	}
	if backup {
		emit(p.row(artifact, rowSpec{text: "backup database " + path.Base(artifact.Rel) +
			" copied before a schema migration; it can hold threads and events since deleted " +
			"from the live database"}))
	}
	database, message := parser.OpenDatabase(artifact.DiskPath)
	if message != "" {
		emit(p.row(artifact, rowSpec{text: parser.ProblemPrefix + message}))
		return
	}
	defer database.Close()
	reader := &store{parser: p, artifact: artifact, database: database, emit: emit}
	if err := reader.rows(); err != nil {
		emit(p.row(artifact, rowSpec{text: parser.ProblemPrefix + parser.DatabaseError(err)}))
	}
}

// rows emits the database's rows in order, stopping at the first error.
func (s *store) rows() error {
	var err error
	if s.tables, err = s.database.TableNames(); err != nil {
		return err
	}
	if s.users, err = s.readUsers(); err != nil {
		return err
	}
	for _, step := range []struct {
		table string
		run   func() error
	}{{"threads", s.threads}, {"pages", s.pages}, {"user_events", s.userEvents},
		{"integrations", s.integrations}} {
		if s.tables[step.table] {
			if err := step.run(); err != nil {
				return err
			}
		}
	}
	return nil
}

// selection is a table's whitelisted columns that it has, in whitelist
// order: the keys each row of a select of them holds.
type selection struct {
	columns []string
}

// has reports whether the selection holds the column.
func (s selection) has(column string) bool {
	return slices.Contains(s.columns, column)
}

// each runs visit over the rows of table restricted to its whitelisted
// columns, ordered by the comma-separated order columns the table has. A
// table with none of the columns gives no rows. A column counts as present
// when "PRAGMA table_info" would list it under exactly that name.
func (s *store) each(table, order string, visit func(sqlitefile.Row, selection)) error {
	handle, err := s.database.Table(table)
	if err != nil {
		return err
	}
	have := map[string]bool{}
	for _, column := range handle.Columns() {
		if column.Generated == sqlitefile.NotGenerated {
			have[column.Name] = true
		}
	}
	var chosen selection
	for _, column := range tableColumns()[table] {
		if have[column] {
			chosen.columns = append(chosen.columns, column)
		}
	}
	if len(chosen.columns) == 0 {
		return nil
	}
	var keys []string
	for _, column := range strings.Split(order, ",") {
		if have[strings.TrimSpace(column)] {
			keys = append(keys, column)
		}
	}
	return handle.Select(chosen.columns...).OrderBy(keys...).Each(func(row sqlitefile.Row) error {
		visit(row, chosen)
		return nil
	})
}

// readUsers returns each user's label, "<email> (<name>)", by id.
func (s *store) readUsers() (map[string]string, error) {
	users := map[string]string{}
	if !s.tables["users"] {
		return users, nil
	}
	err := s.each("users", "id", func(user sqlitefile.Row, _ selection) {
		label := orText(user.Get("email"))
		if name := user.Get("name"); !name.IsEmpty() {
			if label != "" {
				label += " (" + name.String() + ")"
			} else {
				label = name.String()
			}
		}
		users[user.Get("id").String()] = label
	})
	return users, err
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

// digitsLine returns an id read as text as a source line when it is all
// ASCII digits, else 0.
func digitsLine(text string) int {
	number, err := strconv.Atoi(text)
	if err != nil || text == "" || strings.Trim(text, "0123456789") != "" {
		return 0
	}
	return number
}

// grouped is rows grouped by a key, the keys in the order first met.
type grouped struct {
	order []string
	rows  map[string][]sqlitefile.Row
}

// add files a row under its key.
func (g *grouped) add(key string, row sqlitefile.Row) {
	if _, seen := g.rows[key]; !seen {
		g.order = append(g.order, key)
	}
	g.rows[key] = append(g.rows[key], row)
}

// parent is one thread or page: its row, or none when only its children
// name it, and the columns its row holds.
type parent struct {
	parentID string
	row      sqlitefile.Row
	missing  bool
	from     selection
}

// parents returns the rows of table in order, then a stand-in for each key
// of children no row has, whose children still get read under that id.
func (s *store) parents(table, order string, children *grouped) ([]parent, error) {
	var list []parent
	err := s.each(table, order, func(row sqlitefile.Row, from selection) {
		list = append(list, parent{parentID: row.Get("id").String(), row: row, from: from})
	})
	if err != nil {
		return nil, err
	}
	known := map[string]bool{}
	for _, each := range list {
		known[each.parentID] = true
	}
	for _, key := range children.order {
		if !known[key] {
			list = append(list, parent{parentID: key, missing: true})
		}
	}
	return list, nil
}

// owner returns a user's label by the id column, "" when unknown; a
// missing column reads as NULL.
func (s *store) owner(value sqlitefile.Value) string {
	return s.users[value.String()]
}

// threads emits each thread's start row and its messages.
func (s *store) threads() error {
	messages := &grouped{rows: map[string][]sqlitefile.Row{}}
	if s.tables["thread_messages"] {
		err := s.each("thread_messages", "thread_id,created_at,id", func(row sqlitefile.Row, _ selection) {
			messages.add(row.Get("thread_id").String(), row)
		})
		if err != nil {
			return err
		}
	}
	threads, err := s.parents("threads", "created_at,id", messages)
	if err != nil {
		return err
	}
	for _, thread := range threads {
		s.thread(thread, messages.rows[thread.parentID])
	}
	return nil
}

// thread emits one thread's start row and its message rows.
func (s *store) thread(thread parent, messages []sqlitefile.Row) {
	attachments := make([]jsonvalue.Value, len(messages))
	for i, message := range messages {
		attachments[i] = messageAttachment(message)
	}
	project := projectOf(attachments)
	var questions []string
	if value, ok := jsonColumn(thread.row.Get("relevant_questions")); ok {
		for _, question := range arrayItems(value) {
			questions = append(questions, question.Text())
		}
	}
	userID := thread.row.Get("user_id")
	ownerText := ""
	if owner := s.owner(userID); owner != "" {
		ownerText = "owner " + owner
	} else if !userID.IsNull() {
		ownerText = "owner user_id=" + userID.String()
	}
	parts := []string{"thread start", ownerText}
	if !thread.row.Get("is_ephemeral").IsEmpty() {
		parts = append(parts, "ephemeral")
	}
	if !thread.from.has("user_id") {
		parts = append(parts, "thread row missing")
	}
	if len(questions) > 0 {
		parts = append(parts, "relevant questions: "+strings.Join(questions, "; "))
	}
	s.emit(s.parser.row(s.artifact, rowSpec{line: digitsLine(thread.parentID), sessionID: thread.parentID,
		projectPath: project, timestamp: parser.ColumnTime(thread.row.Get("created_at")),
		text: parser.JoinNonEmpty(" | ", parts...)}))
	for i, message := range messages {
		s.emit(s.messageRow(message, attachments[i], thread.parentID, project))
	}
}

// messageRow returns the row of one thread message.
func (s *store) messageRow(message sqlitefile.Row, attachment jsonvalue.Value, threadID,
	project string) timeline.Row {
	role, text := orText(message.Get("role")), orText(message.Get("content"))
	parts := attachmentParts(attachment)
	if source := message.Get("code_source_id"); !source.IsEmpty() {
		parts = append([]string{"code_source " + source.String()}, parts...)
	}
	if len(parts) > 0 {
		text += " [attachments: " + strings.Join(parts, "; ") + "]"
	}
	turnType := role
	if role != timeline.TurnUser && role != timeline.TurnAssistant {
		text, turnType = role+": "+text, timeline.TurnSystem
	}
	return s.parser.row(s.artifact, rowSpec{line: sourceLine(message.Get("id")), sessionID: threadID,
		projectPath: project, timestamp: parser.ColumnTime(message.Get("created_at")),
		turnType: turnType, text: text})
}

// pages emits one system row per AI-written page, then its content and
// each section, in position order, as assistant rows.
func (s *store) pages() error {
	sections := &grouped{rows: map[string][]sqlitefile.Row{}}
	if s.tables["page_sections"] {
		err := s.each("page_sections", "page_id,position,id", func(row sqlitefile.Row, _ selection) {
			sections.add(row.Get("page_id").String(), row)
		})
		if err != nil {
			return err
		}
	}
	pages, err := s.parents("pages", "created_at,id", sections)
	if err != nil {
		return err
	}
	for _, page := range pages {
		s.page(page, sections.rows[page.parentID])
	}
	return nil
}

// page emits one page's rows.
func (s *store) page(page parent, sections []sqlitefile.Row) {
	attachments := make([]jsonvalue.Value, len(sections))
	for i, section := range sections {
		attachment, ok := jsonColumn(section.Get("attachment"))
		if !ok || !attachment.IsObject() {
			attachment = jsonvalue.NewObject()
		}
		attachments[i] = attachment
	}
	spec := rowSpec{line: digitsLine(page.parentID), sessionID: "page:" + page.parentID,
		projectPath: projectOf(attachments), timestamp: parser.ColumnTime(page.row.Get("created_at"))}
	authorID, author := page.row.Get("author_id"), ""
	if owner := s.owner(authorID); owner != "" {
		author = "author " + owner
	} else if !authorID.IsNull() {
		author = "author user_id=" + authorID.String()
	}
	missing := ""
	if !page.from.has("author_id") {
		missing = "page row missing"
	}
	start := spec
	start.text = parser.JoinNonEmpty(" | ", "page start", labelledColumn("title ", page.row.Get("title")), author,
		labelledColumn("code_source ", page.row.Get("code_source_id")), missing)
	s.emit(s.parser.row(s.artifact, start))
	if content := page.row.Get("content"); !content.IsEmpty() {
		body := spec
		body.turnType, body.text = timeline.TurnAssistant, content.String()
		s.emit(s.parser.row(s.artifact, body))
	}
	s.sectionRows(spec, sections, attachments)
}

// sectionRows emits a page's sections as assistant rows: the title and
// content, then the attachments, each at its created_at, else the page's.
func (s *store) sectionRows(spec rowSpec, sections []sqlitefile.Row, attachments []jsonvalue.Value) {
	for i, section := range sections {
		var pieces []string
		for _, value := range []sqlitefile.Value{section.Get("title"), section.Get("content")} {
			if !value.IsNull() && !(value.Type == sqlitefile.Text && len(value.Bytes) == 0) {
				pieces = append(pieces, value.String())
			}
		}
		text := strings.Join(pieces, "\n\n")
		if parts := attachmentParts(attachments[i]); len(parts) > 0 {
			text += " [attachments: " + strings.Join(parts, "; ") + "]"
		}
		row := spec
		row.line, row.turnType, row.text = sourceLine(section.Get("id")), timeline.TurnAssistant, text
		if created := parser.ColumnTime(section.Get("created_at")); created != "" {
			row.timestamp = created
		}
		s.emit(s.parser.row(s.artifact, row))
	}
}

// labelledColumn returns label and the column's text when the column is
// not empty, else "".
func labelledColumn(label string, value sqlitefile.Value) string {
	if value.IsEmpty() {
		return ""
	}
	return label + value.String()
}

// userEvents emits the rows of each user event, or a `parser:` row for a
// payload that is not a JSON object.
func (s *store) userEvents() error {
	return s.each("user_events", "created_at,id", func(event sqlitefile.Row, _ selection) {
		spec := rowSpec{line: sourceLine(event.Get("id")), timestamp: parser.ColumnTime(event.Get("created_at"))}
		userID := event.Get("user_id")
		owner := s.owner(userID)
		if owner == "" {
			owner = userID.String()
		}
		payload, ok := jsonColumn(event.Get("payload"))
		if !ok || !payload.IsObject() {
			spec.text = "parser: user_event " + event.Get("id").String() + " payload did not parse"
			s.emit(s.parser.row(s.artifact, spec))
			return
		}
		s.parser.eventRows(s.artifact, spec, "user="+owner, payload, orText(event.Get("kind")), s.emit)
	})
}

// integrations emits the source-host integrations, then the repositories
// cached from them. The token column is never selected.
func (s *store) integrations() error {
	type named struct{ name, kind string }
	names := map[string]named{}
	err := s.each("integrations", "created_at,id", func(integration sqlitefile.Row, _ selection) {
		kind, apiBase := orText(integration.Get("kind")), orText(integration.Get("api_base"))
		host := integrationHosts()[kind]
		if host == "" {
			host = urlHostname(apiBase)
		}
		name := orText(integration.Get("display_name"))
		names[integration.Get("id").String()] = named{name, kind}
		s.emit(s.parser.row(s.artifact, rowSpec{line: sourceLine(integration.Get("id")),
			timestamp: parser.ColumnTime(integration.Get("created_at")),
			text: "source host: " + dash(host) + " integration " + dash(name) + " kind=" + dash(kind) +
				" api_base=" + dash(apiBase) + " (account not recorded; from integrations table, " +
				"created_at)"}))
	})
	if err != nil || !s.tables["provided_repositories"] {
		return err
	}
	return s.each("provided_repositories", "integration_id,id", func(repository sqlitefile.Row, _ selection) {
		integration := names[repository.Get("integration_id").String()]
		active := "no"
		if !repository.Get("active").IsEmpty() {
			active = "yes"
		}
		owner := integration.name
		if owner == "" {
			owner = "integration_id=" + repository.Get("integration_id").String()
		}
		s.emit(s.parser.row(s.artifact, rowSpec{line: sourceLine(repository.Get("id")),
			timestamp: parser.ColumnTime(repository.Get("updated_at")),
			text: "source host repository: " + dash(orText(repository.Get("git_url"))) + " type=" +
				dash(integration.kind) + " name=" + dash(orText(repository.Get("name"))) + " active=" +
				active + " (integration " + owner + "; from provided_repositories table, updated_at)"}))
	})
}

// dash returns text, or "-" when it is empty.
func dash(text string) string {
	if text == "" {
		return "-"
	}
	return text
}
