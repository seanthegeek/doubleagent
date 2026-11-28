package cursor

import (
	"strconv"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// A composer id whose composerData key cursorDiskKV holds in more than one
// row, or that composerHeaders holds in more than one row, is one only
// damage or an edit writes: the key column is UNIQUE and composerId the
// header table's PRIMARY KEY. Its rows are a group (timeline.Group): each
// stored row is a parent, giving its own rows (its parser: row, session
// name, origin and archive or deletion rows) with the project it names,
// and the composer's own rows (its bubbles, draft, blobs and problems,
// and its rows in the side files of the home) are members, shown once
// with the project and branch of the stored row the group goes with.

// modelNotLent ends the parser: rows of a repeated bubble key whose stored
// rows name another model than the one in force before them.
const modelNotLent = ", and none gives its model to the rows after them"

// repeatedKeyText returns the text of the parser: row each stored row of a
// key table holds in n rows gives when each stored row gives its own rows
// and nothing else takes from it.
func repeatedKeyText(table string, n int) string {
	return parserkit.ProblemPrefix + table + " holds " + strconv.Itoa(n) + " rows of this row's key; each gives its own rows"
}

// repeated reports whether either table holds the composer's id twice.
func (c *composer) repeated() bool {
	return len(c.stored) > 1 || len(c.headers) > 1
}

// assignGroups gives each repeated composer of found its group: the
// parser's own for the composer in the state database at path, so the
// database's parse and the parses of the side files reading it share it,
// or a new one when path is "".
func (p *Parser) assignGroups(path string, found *composers) {
	for _, current := range found.list {
		if !current.repeated() {
			continue
		}
		if path == "" {
			current.group = timeline.NewGroup()
			continue
		}
		key := [2]string{path, current.ident}
		if p.groups == nil {
			p.groups = map[[2]string]*timeline.Group{}
		}
		if p.groups[key] == nil {
			p.groups[key] = timeline.NewGroup()
		}
		current.group = p.groups[key]
	}
}

// sharedState returns the path the side files of the artifact's home read
// their state from when it is the artifact's own, else "".
func sharedState(artifact *input.Artifact) string {
	if path := homeState(artifact); path != "" && path == artifact.DiskPath {
		return path
	}
	return ""
}

// variant returns a copy of the composer as its composerData row data and
// its composerHeaders row header show it; an index past the stored rows
// keeps the first's.
func (c *composer) variant(data, header int) *composer {
	shown := *c
	if data < len(c.stored) {
		shown.data, shown.dataRowid = c.stored[data].value, c.stored[data].rowid
	}
	if header < len(c.headers) {
		shown.header, shown.head = c.headers[header].row, c.headers[header].value
	}
	return &shown
}

// view is one parent of a composer: the composer as its stored rows show
// it, the stamper of the parent's own rows and the problems of those rows.
type view struct {
	composer *composer
	stamp    *stamper
	problems []*parserkit.Problems
}

// view registers the parent of a composer that its composerData row data
// and its composerHeaders row header show, emitting a parser: row for each
// of the two the parent stands for (byData, byHeader), and returns it.
func (r *stateReader) view(current *composer, data, header int, byData, byHeader bool, emit parserkit.Emit) view {
	shown := current.variant(data, header)
	stamp := r.owner.newStamper(r.artifact, shown, emit)
	found := view{composer: shown, stamp: stamp}
	var parent *timeline.Parent
	notice := func(table string, count int, line int64, problems *parserkit.Problems) {
		row := stamp.row(timeline.TurnSystem, "", shown.created(), line, "")
		if parent == nil {
			parent = current.group.Parent(&row)
		}
		prefix := parserkit.ProblemPrefix + table + " holds " + strconv.Itoa(count) + " rows of this row's id; " +
			"its conversation's messages, draft and rows in other files are shown once, with "
		parent.Texts(&row, timeline.RowText{Text: prefix + "this row"},
			timeline.RowText{Text: prefix + "another of them"})
		emit(row)
		found.problems = append(found.problems, problems)
	}
	if byData {
		notice("cursorDiskKV", len(current.stored), shown.dataRowid, current.stored[data].problems)
	}
	if byHeader {
		notice("composerHeaders", len(current.headers), shown.header.Rowid, current.headers[header].problems)
	}
	stamp.emit = func(row timeline.Row) {
		parent.Row(&row, timeline.Always)
		emit(row)
	}
	return found
}

// views returns the parents of a composer and emits each one's parser:
// rows: one per composerData row, with the first header, then one per
// later composerHeaders row, with the first composerData. A composer
// whose id neither table repeats is its one view, with no group.
func (r *stateReader) views(current *composer, emit parserkit.Emit) []view {
	if current.group == nil {
		return []view{{composer: current, stamp: r.owner.newStamper(r.artifact, current, emit)}}
	}
	var views []view
	for i := range max(1, len(current.stored)) {
		views = append(views, r.view(current, i, 0, len(current.stored) > 1, len(current.headers) > 1 && i == 0, emit))
	}
	for i := 1; i < len(current.headers); i++ {
		views = append(views, r.view(current, 0, i, false, true, emit))
	}
	return views
}

// ownProblems emits the problem rows of a view's stored rows, rows of its
// parent.
func (r *stateReader) ownProblems(found view) {
	for _, problems := range found.problems {
		parserkit.EmitAll(found.stamp.emit, r.owner.ErrorRows(r.artifact, *problems, found.composer.ident,
			found.stamp.path))
	}
}

// memberStamp returns the stamper of a composer's own rows: for a
// repeated composer each is a member of its group, taking what takes
// names from the stored row the group goes with.
func (p *Parser) memberStamp(artifact *input.Artifact, current *composer, emit parserkit.Emit,
	takes timeline.Takes) *stamper {
	stamp := p.newStamper(artifact, current, emit)
	if current.group != nil {
		stamp.emit = func(row timeline.Row) {
			current.group.Member(&row, takes)
			emit(row)
		}
	}
	return stamp
}

// member makes row, a row of the composer sessionID in a side file, a
// member of the composer's group when the home's state repeats its id.
func (index *stateIndex) member(sessionID string, row *timeline.Row, takes timeline.Takes) {
	if group := index.groups[sessionID]; group != nil {
		group.Member(row, takes)
	}
}

// cloudMember makes row, a row of the cloud agent sessionID in a side
// file, a member of the agent's group when the home's state repeats its
// bcId and no composer of that id has a group.
func (index *stateIndex) cloudMember(sessionID string, row *timeline.Row, takes timeline.Takes) {
	if repeat := index.cloud[sessionID]; repeat != nil && index.groups[sessionID] == nil {
		repeat.group.Member(row, takes)
	}
}
