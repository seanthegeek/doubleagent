package cursor

import (
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// composer is what the store has about one conversation: its first
// composerData, its first composerHeaders row and that row's decoded
// value, with every stored row of each (stored, headers) and the group of
// an id either table repeats (repeat.go). Its copies share its problems.
type composer struct {
	ident     string
	data      jsonvalue.Value // null when there is no composerData row
	hasData   bool
	dataRowid int64
	header    *sqlitefile.Row
	head      jsonvalue.Value // the header row's value, an object
	problems  *parserkit.Problems
	stored    []storedData
	headers   []storedHeader
	group     *timeline.Group
}

// storedData is one composerData row of a composer: its value, its rowid
// and the problems of the row, the composer's own unless the key repeats.
type storedData struct {
	value    jsonvalue.Value
	rowid    int64
	problems *parserkit.Problems
}

// storedHeader is one composerHeaders row of a composer, its value and
// the problems of the row, the composer's own unless the id repeats.
type storedHeader struct {
	row      *sqlitefile.Row
	value    jsonvalue.Value
	problems *parserkit.Problems
}

// newComposer returns a composer known only by its id.
func newComposer(ident string) *composer {
	return &composer{ident: ident, data: jsonvalue.NewNull(), head: jsonvalue.NewObject(), problems: &parserkit.Problems{}}
}

// documents returns the header value and the composerData, the order a
// composer's project is looked for in.
func (c *composer) documents() []jsonvalue.Value {
	return []jsonvalue.Value{c.head, c.data}
}

// project returns the composer's project path and git branch: the
// workspace's folder, else the first tracked repository's path, and the
// first branch of the tracked repository at that path.
func (c *composer) project() (string, string) {
	path := ""
	for _, document := range c.documents() {
		if path = parserkit.Text(document.Get("workspaceIdentifier").Get("uri").Get("fsPath")); path != "" {
			break
		}
	}
	var repos []jsonvalue.Value
	for _, document := range c.documents() {
		for _, tracked := range parserkit.Elements(document.Get("trackedGitRepos")) {
			if tracked.IsObject() {
				repos = append(repos, tracked)
			}
		}
	}
	if path == "" && len(repos) > 0 {
		path = parserkit.Text(repos[0].Get("repoPath"))
	}
	for _, repo := range repos {
		if parserkit.Text(repo.Get("repoPath")) == path {
			if branches := parserkit.Elements(repo.Get("branches")); len(branches) > 0 && branches[0].IsString() {
				return path, branches[0].StringValue()
			}
			break
		}
	}
	return path, ""
}

// headerValue returns a column of the header row, NULL without one.
func (c *composer) headerValue(column string) sqlitefile.Value {
	if c.header == nil {
		return sqlitefile.Value{}
	}
	return c.header.Get(column)
}

// headerAbsent reports whether a header column is NULL or "".
func headerAbsent(value sqlitefile.Value) bool {
	return value.IsNull() || (value.Type == sqlitefile.Text && len(value.Bytes) == 0)
}

// created returns the composer's creation time: the composerData's, the
// header value's, else the header row's.
func (c *composer) created() string {
	for _, document := range []jsonvalue.Value{c.data, c.head} {
		if value := document.Get("createdAt"); parserkit.Present(value) {
			return timeutil.FromValue(value)
		}
	}
	if value := c.headerValue("createdAt"); !headerAbsent(value) {
		return parserkit.ColumnTime(value)
	}
	return ""
}

// headerUpdated returns the header row's lastUpdatedAt, else the header
// value's.
func (c *composer) headerUpdated() string {
	if value := c.headerValue("lastUpdatedAt"); !headerAbsent(value) {
		return parserkit.ColumnTime(value)
	}
	return timeutil.FromValue(c.head.Get("lastUpdatedAt"))
}

// isTrue reports whether value is JSON true.
func isTrue(value jsonvalue.Value) bool {
	flag, ok := value.Bool()
	return ok && flag
}

// archived reports whether the composer is archived (a delete sets it too).
func (c *composer) archived() bool {
	return !c.headerValue("isArchived").IsEmpty() || isTrue(c.head.Get("isArchived")) ||
		isTrue(c.data.Get("isArchived"))
}

// composers holds the composers of a store in the order they were met.
type composers struct {
	list []*composer
	byID map[string]*composer
}

// get returns the composer of id, adding it when it is new.
func (c *composers) get(ident string) *composer {
	if found, ok := c.byID[ident]; ok {
		return found
	}
	created := newComposer(ident)
	c.byID[ident] = created
	c.list = append(c.list, created)
	return created
}

// readComposers returns every composer of the store: the composerData rows
// in rowid order, then the composers with only a header. A composerData
// value that is not a JSON object is that composer's problem, or that
// stored row's when the key repeats.
func readComposers(store *store) *composers {
	found := &composers{byID: map[string]*composer{}}
	if keyValues := store.table("cursorDiskKV"); keyValues != nil {
		rows := store.rows(prefixed(keyValues, "composerData:"))
		sortByRowid(rows)
		counts := map[string]int{}
		for _, row := range rows {
			counts[row.key]++
		}
		for _, row := range rows {
			current := found.get(row.key[len("composerData:"):])
			problems := current.problems
			if counts[row.key] > 1 {
				problems = &parserkit.Problems{}
			}
			value := objectRecord(store.artifact, row, problems)
			current.stored = append(current.stored, storedData{value: value, rowid: row.rowid, problems: problems})
			if !current.hasData {
				current.data, current.hasData, current.dataRowid = value, true, row.rowid
			}
		}
	}
	readHeaders(store, found)
	return found
}

// readHeaders attaches each composerHeaders row and its decoded value to
// its composer; a value that does not decode, or decodes to anything but
// an object, is that composer's problem.
func readHeaders(store *store, found *composers) {
	table := store.table("composerHeaders")
	if table == nil {
		return
	}
	wanted := []string{"composerId", "createdAt", "lastUpdatedAt", "isArchived", "value"}
	rows, err := selectPresent(table, wanted)
	if store.keep(err) {
		return
	}
	counts := map[string]int{}
	for i := range rows {
		if ident := rows[i].Get("composerId"); !ident.IsEmpty() {
			counts[ident.String()]++
		}
	}
	for i := range rows {
		ident := rows[i].Get("composerId")
		if ident.IsEmpty() {
			continue
		}
		current := found.get(ident.String())
		stored := storedHeader{row: &rows[i], value: jsonvalue.NewObject(), problems: current.problems}
		if counts[current.ident] > 1 {
			stored.problems = &parserkit.Problems{}
		}
		if value := rows[i].Get("value"); !value.IsNull() {
			stored.value = objectRecord(store.artifact, keyed{rowid: rows[i].Rowid, key: "composerHeaders " + current.ident,
				value: value}, stored.problems)
		}
		if current.header == nil {
			current.header, current.head = stored.row, stored.value
		}
		current.headers = append(current.headers, stored)
	}
}
