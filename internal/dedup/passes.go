package dedup

import (
	"slices"

	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// lookup is one place a secondary row looks: the keys whose candidates it
// may take, together, and whether only candidates without a tool_use_id may
// be taken there.
type lookup struct {
	keys   []bucketKey
	idless bool
}

// lookups returns the places a secondary row looks, in order. A row with a
// tool_use_id looks under its id, then its text; the scope is the row's
// own and, with broaden, then its project and its identity alone. Under a
// broader scope a row may take only candidates of its own session or none,
// and of its own project or none, so it looks under exactly those keys.
func (m matcher) lookups(row timeline.Row, broaden bool) []lookup {
	places := scopePlaces(row.SessionID, row.ProjectPath)
	if !broaden {
		places = places[:1]
	}
	type keyed struct {
		ident  identity
		idless bool
	}
	identities := []keyed{{m.byText(row), false}}
	if hasID(row) {
		identities = []keyed{{byID(row), false}, {m.byText(row), true}}
	}
	var out []lookup
	for _, place := range places {
		for _, entry := range identities {
			keys := make([]bucketKey, len(place))
			for i, where := range place {
				keys[i] = bucketKey{entry.ident, where}
			}
			out = append(out, lookup{keys, entry.idless})
		}
	}
	return out
}

// scopePlaces returns the scopes a row of a session and project looks
// under, from its own: its session, else its project, else none; then its
// project with its own session or none; then no scope, with its own session
// or none and its own project or none.
func scopePlaces(session, project string) [][]scope {
	pair := func(sessionID, projectPath string) scope {
		return scope{kind: scopePair, session: sessionID, project: projectPath}
	}
	switch {
	case session != "" && project != "":
		return [][]scope{{{kind: scopeSession, session: session}},
			{pair(session, project), pair("", project)},
			{pair(session, project), pair(session, ""), pair("", project), pair("", "")}}
	case session != "":
		return [][]scope{{{kind: scopeSession, session: session}},
			{{kind: scopeSession, session: session}, {kind: scopeSession}}}
	case project != "":
		return [][]scope{{{kind: scopeProject, project: project}},
			{{kind: scopeProject, project: project}, {kind: scopeProject}}}
	}
	return [][]scope{{{kind: scopeAny}}}
}

// pass is one matching pass: the candidates filed by key, the rows their
// indices name, which of them are taken, how many rows were filed, the
// blockTrees widened rows have needed, and the work its lookups did.
type pass struct {
	buckets map[bucketKey]*bucket
	rows    []timeline.Row
	used    []bool
	filed   int
	trees   map[treeKey]*blockTree
	steps   int
}

// newPass returns an empty pass over candidate rows.
func newPass(rows []timeline.Row) *pass {
	return &pass{buckets: map[bucketKey]*bucket{}, rows: rows, used: make([]bool, len(rows)),
		trees: map[treeKey]*blockTree{}}
}

// seal sorts the settled candidates once every one is filed.
func (p *pass) seal() {
	for _, found := range p.buckets {
		for i := range found.parts {
			slices.SortFunc(found.parts[i].settled.stamps, func(left, right stamp) int {
				p.steps++
				return compareStamps(left, right)
			})
		}
	}
}

// partsOf returns the candidate parts under a lookup's keys.
func (p *pass) partsOf(entry lookup) []*bucketPart {
	var parts []*bucketPart
	for _, key := range entry.keys {
		found := p.buckets[key]
		if found == nil {
			continue
		}
		parts = append(parts, &found.parts[0])
		if !entry.idless {
			parts = append(parts, &found.parts[1])
		}
	}
	return parts
}

// find returns the candidate a row pairs with, trying its lookups in
// order. With broaden the row is matched among secondary copies, at its
// own time where the primary join would give it none, and never with a
// candidate from its own file.
func (m matcher) find(state *pass, row timeline.Row, tolerance float64, broaden bool,
	bound *forwardBound, forward window) (int, bool) {
	query := &search{state: state, bound: bound}
	if broaden {
		query.refused = &refusal{row.SourceFile}
	}
	for _, entry := range m.lookups(row, broaden) {
		index, ok := m.pair(query, state.partsOf(entry), row, tolerance, forward, !broaden)
		if ok {
			return index, true
		}
	}
	return -1, false
}

// processingOrder returns the secondary row indices in the order the rule
// tests them: timestamped rows by time, then undated rows in input order,
// grouped by scope with byScope. The primary pass (byScope) orders by match
// time, the pass among copies by each row's own time. parser: rows are
// left out.
func (m matcher) processingOrder(secondary []Secondary, byScope bool) []int {
	var stamped []stamp
	var undated []int
	for index, item := range secondary {
		if isParserRow(item.Row) {
			continue
		}
		if millis, dated := m.matchTime(item.Row, byScope); dated {
			stamped = append(stamped, stamp{millis, index})
		} else {
			undated = append(undated, index)
		}
	}
	slices.SortFunc(stamped, compareStamps)
	if byScope {
		slices.SortStableFunc(undated, func(left, right int) int {
			return scopeRank(secondary[left].Row) - scopeRank(secondary[right].Row)
		})
	}
	order := make([]int, 0, len(stamped)+len(undated))
	for _, entry := range stamped {
		order = append(order, entry.index)
	}
	return append(order, undated...)
}

// againstPrimary returns the indices of the secondary rows that pair with
// a primary row. Each row looks only under its own scope; a row in forward
// looks up to that many seconds after it, before the bound limits gives.
func (m matcher) againstPrimary(primary []timeline.Row, secondary []Secondary, limits *bounds,
	forward map[int]float64) []int {
	state := newPass(primary)
	for index, row := range primary {
		if !isParserRow(row) {
			millis, dated := m.matchTime(row, true)
			m.add(state, row, index, millis, dated, false)
		}
	}
	state.seal()
	var matched []int
	for _, secondaryIndex := range m.processingOrder(secondary, true) {
		item := secondary[secondaryIndex]
		seconds, widened := forward[secondaryIndex]
		var bound *forwardBound
		if limits != nil && widened {
			bound = m.candidateBound(limits, item.Row)
		}
		index, ok := m.find(state, item.Row, item.Tolerance, false, bound,
			window{seconds, widened})
		if ok {
			state.used[index] = true
			matched = append(matched, secondaryIndex)
		}
	}
	*m.steps += state.steps
	return matched
}

// amongSecondary returns the indices of the secondary rows not in matched
// that pair with another secondary row. A candidate is a row a primary row
// took or one tested before, from another file; each is taken once, and a
// row also looks under broader scopes.
func (m matcher) amongSecondary(secondary []Secondary, matched map[int]bool) []int {
	rows := make([]timeline.Row, len(secondary))
	for index, item := range secondary {
		rows[index] = item.Row
	}
	state := newPass(rows)
	taken := make([]int, 0, len(matched))
	for index := range matched {
		taken = append(taken, index)
	}
	slices.Sort(taken)
	for _, index := range taken {
		millis, dated := m.matchTime(rows[index], false)
		m.add(state, rows[index], index, millis, dated, false)
	}
	state.seal()
	var copies []int
	for _, index := range m.processingOrder(secondary, false) {
		if matched[index] {
			continue
		}
		item := secondary[index]
		if candidate, ok := m.find(state, item.Row, item.Tolerance, true, nil, window{}); ok {
			state.used[candidate] = true
			copies = append(copies, index)
		}
		millis, dated := m.matchTime(item.Row, false)
		m.add(state, item.Row, index, millis, dated, true)
	}
	*m.steps += state.steps
	return copies
}

// forwardWindows returns, by index, the secondary rows whose agent's Window
// hook gives a window wider than the row's tolerance, with that window.
func (m matcher) forwardWindows(secondary []Secondary) map[int]float64 {
	windows := map[int]float64{}
	for index, item := range secondary {
		hook := m.hooks[item.Row.Agent].Window
		if hook == nil {
			continue
		}
		if wider, ok := hook(item.Row); ok && wider > item.Tolerance {
			windows[index] = wider
		}
	}
	return windows
}
