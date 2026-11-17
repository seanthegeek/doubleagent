package dedup

import (
	"slices"

	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// lookup is one key a secondary row looks under, with whether it is a
// broader scope than the row's own and whether only candidates without a
// tool_use_id may be taken there.
type lookup struct {
	key           bucketKey
	broad, idless bool
}

// lookups returns the keys a secondary row looks under, in order. A row
// with a tool_use_id looks under its id, then its text; the scope is the
// row's own and, with broaden, then its project and its identity alone.
func (m matcher) lookups(row timeline.Row, broaden bool) []lookup {
	var scopes []scope
	if row.SessionID != "" {
		scopes = append(scopes, scope{scopeSession, row.SessionID})
	}
	if row.ProjectPath != "" {
		scopes = append(scopes, scope{scopeProject, row.ProjectPath})
	}
	scopes = append(scopes, scope{kind: scopeAny})
	if !broaden {
		scopes = scopes[:1]
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
	for depth, where := range scopes {
		for _, entry := range identities {
			out = append(out, lookup{bucketKey{entry.ident, where}, depth > 0, entry.idless})
		}
	}
	return out
}

// unavailable decides which candidates a row may not pair with: those
// already taken; with sameFile, those from the row's own file; under an
// id-less key, those with a tool_use_id; and under a broader scope, those
// whose session or project differs from the row's.
type unavailable struct {
	rows          []timeline.Row
	used          map[int]bool
	row           timeline.Row
	sameFile      bool
	broad, idless bool
}

// contains reports whether the candidate at a row index may not be taken.
func (u *unavailable) contains(index int) bool {
	if u.used[index] {
		return true
	}
	candidate, row := u.rows[index], u.row
	if u.sameFile && candidate.SourceFile == row.SourceFile {
		return true
	}
	if u.idless && hasID(candidate) {
		return true
	}
	if !u.broad {
		return false
	}
	sessionDiffers := row.SessionID != "" && candidate.SessionID != "" &&
		candidate.SessionID != row.SessionID
	projectDiffers := row.ProjectPath != "" && candidate.ProjectPath != "" &&
		candidate.ProjectPath != row.ProjectPath
	return sessionDiffers || projectDiffers
}

// find returns the candidate a row pairs with, trying its lookups in
// order. With broaden the row is matched among secondary copies, at its
// own time where the primary join would give it none.
func (m matcher) find(buckets map[bucketKey]*bucket, row timeline.Row, tolerance float64,
	rule *unavailable, broaden bool, upper upperBound, forward window) (int, bool) {
	for _, entry := range m.lookups(row, broaden) {
		rule.broad, rule.idless = entry.broad, entry.idless
		index, ok := m.pair(buckets[entry.key], row, tolerance, rule.contains, upper, forward,
			!broaden)
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
	buckets := map[bucketKey]*bucket{}
	for index, row := range primary {
		if !isParserRow(row) {
			millis, dated := m.matchTime(row, true)
			m.add(buckets, row, index, millis, dated)
		}
	}
	used := map[int]bool{}
	var matched []int
	for _, secondaryIndex := range m.processingOrder(secondary, true) {
		item := secondary[secondaryIndex]
		seconds, widened := forward[secondaryIndex]
		var upper upperBound
		if limits != nil && widened {
			upper = m.candidateBound(limits, primary, item.Row)
		}
		rule := &unavailable{rows: primary, used: used, row: item.Row}
		index, ok := m.find(buckets, item.Row, item.Tolerance, rule, false, upper,
			window{seconds, widened})
		if ok {
			used[index] = true
			matched = append(matched, secondaryIndex)
		}
	}
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
	buckets := map[bucketKey]*bucket{}
	taken := make([]int, 0, len(matched))
	for index := range matched {
		taken = append(taken, index)
	}
	slices.Sort(taken)
	for _, index := range taken {
		millis, dated := m.matchTime(rows[index], false)
		m.add(buckets, rows[index], index, millis, dated)
	}
	used := map[int]bool{}
	var copies []int
	for _, index := range m.processingOrder(secondary, false) {
		if matched[index] {
			continue
		}
		item := secondary[index]
		rule := &unavailable{rows: rows, used: used, row: item.Row, sameFile: true}
		if candidate, ok := m.find(buckets, item.Row, item.Tolerance, rule, true, nil,
			window{}); ok {
			used[candidate] = true
			copies = append(copies, index)
		}
		millis, dated := m.matchTime(item.Row, false)
		m.add(buckets, item.Row, index, millis, dated)
	}
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
