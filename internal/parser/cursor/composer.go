package cursor

import (
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// composer is what the store has about one conversation: its
// composerData, its composerHeaders row and that row's decoded value.
type composer struct {
	ident     string
	data      jsonvalue.Value // null when there is no composerData row
	hasData   bool
	dataRowid int64
	header    *sqlitefile.Row
	head      jsonvalue.Value // the header row's value, an object
	problems  parser.Problems
}

// newComposer returns a composer known only by its id.
func newComposer(ident string) *composer {
	return &composer{ident: ident, data: jsonvalue.NewNull(), head: jsonvalue.NewObject()}
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
		if path = parser.Str(document.Get("workspaceIdentifier").Get("uri").Get("fsPath")); path != "" {
			break
		}
	}
	var repos []jsonvalue.Value
	for _, document := range c.documents() {
		for _, tracked := range parser.Elements(document.Get("trackedGitRepos")) {
			if tracked.IsObject() {
				repos = append(repos, tracked)
			}
		}
	}
	if path == "" && len(repos) > 0 {
		path = parser.Str(repos[0].Get("repoPath"))
	}
	for _, repo := range repos {
		if parser.Str(repo.Get("repoPath")) == path {
			if branches := parser.Elements(repo.Get("branches")); len(branches) > 0 && branches[0].IsString() {
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
		if value := document.Get("createdAt"); !absent(value) {
			return timeutil.FromValue(value)
		}
	}
	if value := c.headerValue("createdAt"); !headerAbsent(value) {
		return parser.ColumnTime(value)
	}
	return ""
}

// headerUpdated returns the header row's lastUpdatedAt, else the header
// value's.
func (c *composer) headerUpdated() string {
	if value := c.headerValue("lastUpdatedAt"); !headerAbsent(value) {
		return parser.ColumnTime(value)
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
	return c.headerValue("isArchived").Truthy() || isTrue(c.head.Get("isArchived")) ||
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
// in rowid order, then the composers with only a header.
func readComposers(store *store) *composers {
	found := &composers{byID: map[string]*composer{}}
	if keyValues := store.table("cursorDiskKV"); keyValues != nil {
		rows := store.rows(prefixed(keyValues, "composerData:"))
		sortByRowid(rows)
		for _, row := range rows {
			current := found.get(row.key[len("composerData:"):])
			current.data = objectOf(decodeRecord(row.rowid, row.key, row.value, &current.problems))
			current.hasData, current.dataRowid = true, row.rowid
		}
	}
	readHeaders(store, found)
	return found
}

// objectOf returns value when it is an object, else an empty object.
func objectOf(value jsonvalue.Value) jsonvalue.Value {
	if value.IsObject() {
		return value
	}
	return jsonvalue.NewObject()
}

// readHeaders attaches each composerHeaders row and its decoded value to
// its composer; a value that does not decode is that composer's problem.
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
	for i := range rows {
		ident := rows[i].Get("composerId")
		if !ident.Truthy() {
			continue
		}
		current := found.get(ident.Str())
		current.header = &rows[i]
		if value := rows[i].Get("value"); !value.IsNull() {
			current.head = objectOf(decodeRecord(rows[i].Rowid, "composerHeaders "+current.ident, value,
				&current.problems))
		}
	}
}
