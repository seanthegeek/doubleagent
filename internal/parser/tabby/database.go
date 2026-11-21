package tabby

import (
	"cmp"
	"path"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// tableColumns are the only columns ever read. Of the secret-bearing
// columns, a user's auth_token and an integration's access_token are read
// and shown through parser.SecretText; the tables holding the others are
// not read.
func tableColumns() map[string][]string {
	return map[string][]string{
		"users":   {"id", "email", "name", "auth_token", "created_at"},
		"threads": {"id", "is_ephemeral", "user_id", "created_at", "updated_at", "relevant_questions"},
		"thread_messages": {"id", "thread_id", "role", "content", "code_source_id", "attachment",
			"code_attachments", "client_code_attachments", "doc_attachments", "created_at"},
		"user_events":           {"id", "user_id", "kind", "created_at", "payload"},
		"pages":                 {"id", "author_id", "title", "content", "code_source_id", "created_at"},
		"page_sections":         {"id", "page_id", "title", "content", "position", "attachment", "created_at"},
		"integrations":          {"id", "kind", "display_name", "api_base", "access_token", "created_at"},
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
	tables   parser.Tables
	// users holds each user's label by the key of its id, under
	// userCollation, the collation users.id declares.
	users         map[string]string
	userCollation string
	emit          parser.Emit
	// carried cuts what a join copies onto many rows: a user's label and
	// an integration's name and kind.
	carried *parser.Carried
	// problems are the database's own problems, emitted after its rows.
	problems parser.Problems
	// pageGroups holds the group (timeline.Group) of each page id the
	// pages table repeats with sections, by id.
	pageGroups map[string]*timeline.Group
	// projects holds the project of each thread and page id whose rows
	// name one, by table and id, for every stored row of a repeated id.
	projects map[[2]string]string
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
	database, err := parser.OpenDatabase(artifact.DiskPath)
	if err != nil {
		emit(p.row(artifact, rowSpec{text: parser.ProblemPrefix + parser.DatabaseError(err)}))
		return
	}
	defer database.Close()
	reader := &store{parser: p, artifact: artifact, database: database, emit: emit,
		carried: parser.NewCarried(artifact), projects: map[[2]string]string{}}
	if err := reader.rows(); err != nil {
		emit(p.row(artifact, rowSpec{text: parser.ProblemPrefix + parser.DatabaseError(err)}))
	}
	problems := reader.problems
	reader.carried.Report(0, &problems)
	for _, problem := range problems {
		emit(p.row(artifact, rowSpec{text: parser.ProblemPrefix + problem.Message}))
	}
}

// rows emits the database's rows in order, stopping at the first error.
func (s *store) rows() error {
	var err error
	if s.tables, err = parser.TablesOf(s.database); err != nil {
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
		if s.tables.Has(step.table) {
			if err := step.run(); err != nil {
				return err
			}
		}
	}
	return nil
}

// each runs visit over the rows of table restricted to its whitelisted
// columns, ordered by the comma-separated order columns the table has. A
// table with none of the columns gives no rows. A column counts as present
// when "PRAGMA table_info" would list it under that name in any ASCII case,
// as SQL finds it (sqlitefile.Table.Has).
func (s *store) each(table, order string, visit func(sqlitefile.Row)) error {
	handle, err := s.database.Table(table)
	if err != nil {
		return err
	}
	var chosen []string
	for _, column := range tableColumns()[table] {
		if handle.Has(column) {
			chosen = append(chosen, column)
		}
	}
	if len(chosen) == 0 {
		return nil
	}
	var keys []string
	for _, column := range strings.Split(order, ",") {
		if handle.Has(strings.TrimSpace(column)) {
			keys = append(keys, column)
		}
	}
	return handle.Select(chosen...).OrderBy(keys...).Each(func(row sqlitefile.Row) error {
		visit(row)
		return nil
	})
}

// readUsers returns each user's label, "<email> (<name>)", by id, and
// emits a "tabby user:" row with the auth_token of each user that has one,
// the token through parser.SecretText. An id is the table's key, so a
// second row of one is damage: the label is text inside the rows of the
// threads, pages and events naming the user, rows of other ids, so no
// stored row of such an id lends its label to them (they name the user by
// id), and each stored row gives a `parser:` row naming its own label.
func (s *store) readUsers() (map[string]string, error) {
	users := map[string]string{}
	if !s.tables.Has("users") {
		return users, nil
	}
	var stored []sqlitefile.Row
	counts := map[string]int{}
	s.userCollation = s.idCollation("users")
	err := s.each("users", "id", func(user sqlitefile.Row) {
		stored = append(stored, user)
		counts[parser.CollatedKey(user.Get("id"), s.userCollation)]++
	})
	for _, user := range stored {
		key := parser.CollatedKey(user.Get("id"), s.userCollation)
		userID, label := user.Get("id").String(), userLabel(user)
		spec := rowSpec{line: parser.ColumnLine(user.Get("id")), timestamp: parser.ColumnTime(user.Get("created_at"))}
		if token := parser.NonEmptyColumn(user.Get("auth_token")); token != "" {
			spec.text = "tabby user: " + cmp.Or(label, "-") + " id=" + userID + " auth_token=" +
				parser.SecretText(s.artifact, token) + " (from users table, created_at)"
			s.emit(s.parser.row(s.artifact, spec))
		}
		if key == "" {
			continue
		}
		if counts[key] == 1 {
			users[key] = label
			continue
		}
		spec.text = parser.ProblemPrefix + "users holds " + strconv.Itoa(counts[key]) + " rows of this row's id; " +
			"the rows of its threads, pages and events name the user by id alone, and this row's label is " +
			cmp.Or(s.carried.Cut("user label", label), "-")
		s.emit(s.parser.row(s.artifact, spec))
	}
	return users, err
}

// userLabel returns a user's label, "<email> (<name>)", or whichever of
// the two the row holds.
func userLabel(user sqlitefile.Row) string {
	label := parser.NonEmptyColumn(user.Get("email"))
	if name := user.Get("name"); !name.IsEmpty() {
		if label != "" {
			label += " (" + name.String() + ")"
		} else {
			label = name.String()
		}
	}
	return label
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

// nullKey files the children whose parent id is NULL: no key equals it,
// since every key ColumnKey gives starts with a letter, so no parent row
// takes them and they are shown under a stand-in of their own.
const nullKey = "\x00"

// grouped is rows grouped by the key of the parent id they name, the keys
// in the order first met, with the text of the id as the first row of
// each key names it.
type grouped struct {
	order []string
	rows  map[string][]sqlitefile.Row
	text  map[string]string
	// collation is the one the parent table's id column declares, under
	// which a child's parent id is compared with the parents' ids.
	collation string
}

// newGrouped returns an empty grouping under the collation of the parent
// table's id column.
func newGrouped(collation string) *grouped {
	return &grouped{rows: map[string][]sqlitefile.Row{}, text: map[string]string{}, collation: collation}
}

// add files a row under the parent id it names.
func (g *grouped) add(parentID sqlitefile.Value, row sqlitefile.Row) {
	key := parser.CollatedKey(parentID, g.collation)
	if key == "" {
		key = nullKey
	}
	if _, seen := g.rows[key]; !seen {
		g.order = append(g.order, key)
		g.text[key] = parentID.String()
	}
	g.rows[key] = append(g.rows[key], row)
}

// parent is one thread or page: its row, or none (missing) when only its
// children name it; key is its id as the id column compares it ("" for
// a NULL id, which no child's parent id equals) and parentID its text.
type parent struct {
	key      string
	parentID string
	row      sqlitefile.Row
	missing  bool
}

// parents returns the rows of table in order, then a stand-in for each key
// of children no row has, whose children still get read under that id.
func (s *store) parents(table, order string, children *grouped) ([]parent, error) {
	var list []parent
	err := s.each(table, order, func(row sqlitefile.Row) {
		list = append(list, parent{key: parser.CollatedKey(row.Get("id"), children.collation),
			parentID: row.Get("id").String(), row: row})
	})
	if err != nil {
		return nil, err
	}
	known := map[string]bool{}
	for _, each := range list {
		known[each.key] = true
	}
	for _, key := range children.order {
		if !known[key] {
			list = append(list, parent{key: key, parentID: children.text[key], missing: true})
		}
	}
	return list, nil
}

// idCollation returns the collation the id column of table declares, ""
// when the table is missing or cannot be opened, as each then reads none.
func (s *store) idCollation(table string) string {
	if !s.tables.Has(table) {
		return ""
	}
	handle, err := s.database.Table(table)
	if err != nil {
		return ""
	}
	return parser.KeyCollation(handle, "id")
}

// eachParent runs visit over each parent with the children filed under
// its id. An id is its table's key, so a second row of one is damage: the
// children go with the first row of the id, in the order the rows are
// read, each later row is visited with none, and the table gives one
// problem counting those rows, so the rows grow with the table rather
// than with the square of it.
func (s *store) eachParent(table, noun string, parents []parent, children *grouped,
	visit func(parent, []sqlitefile.Row)) {
	shown, repeats := map[string]bool{}, 0
	for _, each := range parents {
		group := children.rows[each.key]
		if len(group) > 0 && shown[each.key] {
			group = nil
			repeats++
		}
		if len(group) > 0 {
			shown[each.key] = true
		}
		visit(each, group)
	}
	if repeats > 0 {
		rows := "1 row repeating"
		if repeats > 1 {
			rows = strconv.Itoa(repeats) + " rows repeating"
		}
		s.problems = append(s.problems, parser.Problem{Message: table + " holds " + rows +
			" an earlier row's id; its " + noun + " are shown once"})
	}
}

// projectOfID returns the project of a thread or page id: the first git
// URL its rows attach code from. Those rows go with the first stored row
// of the id (eachParent), so the project is kept for the id's later rows,
// whose start rows show it too: it is the id's, not one stored row's.
func (s *store) projectOfID(table, parentKey string, attachments []jsonvalue.Value) string {
	key := [2]string{table, parentKey}
	if len(attachments) == 0 {
		return s.projects[key]
	}
	project := projectOf(s.artifact, attachments)
	if project != "" {
		s.projects[key] = project
	}
	return project
}

// owner returns a user's label by the id column, "" when unknown; a
// missing column reads as NULL. The label is cut by the store's carried,
// since each of the user's rows repeats it.
func (s *store) owner(value sqlitefile.Value) string {
	return s.carried.Cut("user label", s.users[parser.CollatedKey(value, s.userCollation)])
}

// threads emits each thread's start row and its messages.
func (s *store) threads() error {
	messages := newGrouped(s.idCollation("threads"))
	if s.tables.Has("thread_messages") {
		err := s.each("thread_messages", "thread_id,created_at,id", func(row sqlitefile.Row) {
			messages.add(row.Get("thread_id"), row)
		})
		if err != nil {
			return err
		}
	}
	threads, err := s.parents("threads", "created_at,id", messages)
	if err != nil {
		return err
	}
	s.eachParent("threads", "messages", threads, messages, s.thread)
	return nil
}

// thread emits one thread's start row and its message rows.
func (s *store) thread(thread parent, messages []sqlitefile.Row) {
	attachments := make([]jsonvalue.Value, len(messages))
	for i, message := range messages {
		attachments[i] = messageAttachment(message)
	}
	project := s.projectOfID("threads", thread.key, attachments)
	var questions []string
	if value, ok := jsonColumn(thread.row.Get("relevant_questions")); ok {
		for _, question := range parser.Elements(value) {
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
	if thread.missing {
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
	role, text := parser.NonEmptyColumn(message.Get("role")), parser.NonEmptyColumn(message.Get("content"))
	parts := attachmentParts(s.artifact, attachment)
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
	return s.parser.row(s.artifact, rowSpec{line: parser.ColumnLine(message.Get("id")), sessionID: threadID,
		projectPath: project, timestamp: parser.ColumnTime(message.Get("created_at")),
		turnType: turnType, text: text})
}

// pages emits one system row per AI-written page, then its content and
// each section, in position order, as assistant rows.
func (s *store) pages() error {
	sections := newGrouped(s.idCollation("pages"))
	if s.tables.Has("page_sections") {
		err := s.each("page_sections", "page_id,position,id", func(row sqlitefile.Row) {
			sections.add(row.Get("page_id"), row)
		})
		if err != nil {
			return err
		}
	}
	pages, err := s.parents("pages", "created_at,id", sections)
	if err != nil {
		return err
	}
	s.pageGroups = repeatedGroups(pages, sections)
	s.eachParent("pages", "sections", pages, sections, s.page)
	return nil
}

// repeatedGroups returns a group for each id that more than one row of
// parents has and that children are filed under: the rows of the id are
// its parents, and the children, shown once, its members.
func repeatedGroups(parents []parent, children *grouped) map[string]*timeline.Group {
	counts := map[string]int{}
	for _, each := range parents {
		if each.key != "" && !each.missing {
			counts[each.key]++
		}
	}
	groups := map[string]*timeline.Group{}
	for key, count := range counts {
		if count > 1 && len(children.rows[key]) > 0 {
			groups[key] = timeline.NewGroup()
		}
	}
	return groups
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
		projectPath: s.projectOfID("pages", page.key, attachments),
		timestamp:   parser.ColumnTime(page.row.Get("created_at"))}
	authorID, author := page.row.Get("author_id"), ""
	if owner := s.owner(authorID); owner != "" {
		author = "author " + owner
	} else if !authorID.IsNull() {
		author = "author user_id=" + authorID.String()
	}
	missing := ""
	if page.missing {
		missing = "page row missing"
	}
	start := spec
	start.text = parser.JoinNonEmpty(" | ", "page start", labelledColumn("title ", page.row.Get("title")), author,
		labelledColumn("code_source ", page.row.Get("code_source_id")), missing)
	startRow := s.parser.row(s.artifact, start)
	// Each row of a repeated id is a parent of the id's group, and its
	// sections, shown once, members that take the time of the row the
	// group goes with when they have none of their own.
	group := s.pageGroups[page.key]
	var handle *timeline.Parent
	if group != nil {
		handle = group.Parent(&startRow)
	}
	s.emit(startRow)
	if content := page.row.Get("content"); !content.IsEmpty() {
		body := spec
		body.turnType, body.text = timeline.TurnAssistant, content.String()
		bodyRow := s.parser.row(s.artifact, body)
		if handle != nil {
			handle.Row(&bodyRow, timeline.Always)
		}
		s.emit(bodyRow)
	}
	s.sectionRows(spec, sections, attachments, group)
}

// sectionRows emits a page's sections as assistant rows: the title and
// content, then the attachments, each at its created_at, else the page's,
// or, for the sections of a repeated id, those of group, the time of the
// page row the group goes with.
func (s *store) sectionRows(spec rowSpec, sections []sqlitefile.Row, attachments []jsonvalue.Value,
	group *timeline.Group) {
	for i, section := range sections {
		var pieces []string
		for _, value := range []sqlitefile.Value{section.Get("title"), section.Get("content")} {
			if !value.IsNull() && !(value.Type == sqlitefile.Text && len(value.Bytes) == 0) {
				pieces = append(pieces, value.String())
			}
		}
		text := strings.Join(pieces, "\n\n")
		if parts := attachmentParts(s.artifact, attachments[i]); len(parts) > 0 {
			text += " [attachments: " + strings.Join(parts, "; ") + "]"
		}
		row := spec
		row.line, row.turnType, row.text = parser.ColumnLine(section.Get("id")), timeline.TurnAssistant, text
		created := parser.ColumnTime(section.Get("created_at"))
		if created != "" {
			row.timestamp = created
		}
		sectionRow := s.parser.row(s.artifact, row)
		if group != nil && created == "" {
			group.Member(&sectionRow, timeline.TakeTime)
		}
		s.emit(sectionRow)
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
	return s.each("user_events", "created_at,id", func(event sqlitefile.Row) {
		spec := rowSpec{line: parser.ColumnLine(event.Get("id")), timestamp: parser.ColumnTime(event.Get("created_at"))}
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
		s.parser.eventRows(s.artifact, spec, "user="+owner, payload, parser.NonEmptyColumn(event.Get("kind")), s.emit)
	})
}

// integrations emits the source-host integrations, then the repositories
// cached from them. An integration's access_token goes through
// parser.SecretText.
func (s *store) integrations() error {
	type named struct{ name, kind string }
	names, collation := map[string]named{}, s.idCollation("integrations")
	err := s.each("integrations", "created_at,id", func(integration sqlitefile.Row) {
		kind, apiBase := parser.NonEmptyColumn(integration.Get("kind")), parser.NonEmptyColumn(integration.Get("api_base"))
		host := integrationHosts()[kind]
		if host == "" {
			host = urlHostname(apiBase)
		}
		name := parser.NonEmptyColumn(integration.Get("display_name"))
		// Each repository's row repeats its integration's name and kind.
		if key := parser.CollatedKey(integration.Get("id"), collation); key != "" {
			names[key] = named{s.carried.Cut("integration name", name), s.carried.Cut("integration kind", kind)}
		}
		s.emit(s.parser.row(s.artifact, rowSpec{line: parser.ColumnLine(integration.Get("id")),
			timestamp: parser.ColumnTime(integration.Get("created_at")),
			text: "source host: " + cmp.Or(host, "-") + " integration " + cmp.Or(name, "-") + " kind=" + cmp.Or(kind, "-") +
				" api_base=" + cmp.Or(apiBase, "-") + accessToken(s.artifact, integration.Get("access_token")) +
				" (account not recorded; from integrations table, created_at)"}))
	})
	if err != nil || !s.tables.Has("provided_repositories") {
		return err
	}
	return s.each("provided_repositories", "integration_id,id", func(repository sqlitefile.Row) {
		integration := names[parser.CollatedKey(repository.Get("integration_id"), collation)]
		active := "no"
		if !repository.Get("active").IsEmpty() {
			active = "yes"
		}
		owner := integration.name
		if owner == "" {
			owner = "integration_id=" + repository.Get("integration_id").String()
		}
		s.emit(s.parser.row(s.artifact, rowSpec{line: parser.ColumnLine(repository.Get("id")),
			timestamp: parser.ColumnTime(repository.Get("updated_at")),
			text: "source host repository: " + cmp.Or(parser.NonEmptyColumn(repository.Get("git_url")), "-") + " type=" +
				cmp.Or(integration.kind, "-") + " name=" + cmp.Or(parser.NonEmptyColumn(repository.Get("name")), "-") + " active=" +
				active + " (integration " + owner + "; from provided_repositories table, updated_at)"}))
	})
}

// accessToken returns " access_token=<token>" of an integration's token
// column through parser.SecretText, "" when it is NULL or empty.
func accessToken(artifact *input.Artifact, value sqlitefile.Value) string {
	if token := parser.NonEmptyColumn(value); token != "" {
		return " access_token=" + parser.SecretText(artifact, token)
	}
	return ""
}
