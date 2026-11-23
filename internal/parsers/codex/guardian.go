package codex

import (
	"cmp"
	"slices"
	"sort"
	"unicode/utf8"

	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// guardianLog is one log row holding a guardian review span: its ts and
// how the database compares the ts column with a value, whether its body
// is text a query could return, and the thread_id and tool_call_id of
// each of its spans, in order.
type guardianLog struct {
	time     sqlitefile.Value
	compare  func(sqlitefile.Value) (int, bool)
	readable bool
	spans    []logSpan
}

// logSpan is the thread_id and tool_call_id of one guardian review span.
type logSpan struct {
	thread, call string
}

// guardianIndex holds the guardian rows of one log database in ts order,
// so the rows of a time window are found by binary search. Only rows whose
// ts can fall in a window of whole seconds are kept: a number, or text in a
// TEXT column, which compares the bound as text.
type guardianIndex struct {
	logs []guardianLog
	// unreadable[i] counts the rows of logs[:i] whose body is not text a
	// query could return.
	unreadable []int
	// parents holds, per thread a span names, its calls in ts order,
	// gathered in one pass over the rows when the index is built.
	parents map[string]*parentCalls
	work    parserkit.Work
}

// parentCalls is the calls of one reviewed thread's spans, in the order of
// the index's rows, with the row of each; other[i] is the first position
// at or after i whose call differs from calls[i], len(calls) when none.
type parentCalls struct {
	rows, other []int
	calls       []string
}

// newGuardianIndex returns the index of a database's guardian rows, those
// whose ts can fall in a window that textTimes says how to read, counting
// its lookups in work.
func newGuardianIndex(logs []guardianLog, textTimes bool, work parserkit.Work) *guardianIndex {
	kept := slices.DeleteFunc(logs, func(each guardianLog) bool {
		if textTimes {
			return each.time.Type != sqlitefile.Text
		}
		return each.time.Type != sqlitefile.Integer && each.time.Type != sqlitefile.Float
	})
	slices.SortStableFunc(kept, func(left, right guardianLog) int {
		order, _ := left.compare(right.time)
		return order
	})
	index := &guardianIndex{logs: kept, unreadable: make([]int, len(kept)+1), parents: callsByThread(kept, work),
		work: work}
	for i, each := range kept {
		index.unreadable[i+1] = index.unreadable[i]
		if !each.readable {
			index.unreadable[i+1]++
		}
	}
	return index
}

// window returns the positions of the rows whose ts lies from low to high
// seconds, both included, as the database compares them.
func (x *guardianIndex) window(low, high int64) (int, int) {
	from := sort.Search(len(x.logs), func(i int) bool {
		x.work.Add("guardian rows looked at", 1)
		order, ok := x.logs[i].compare(sqlitefile.IntValue(low))
		return ok && order >= 0
	})
	until := sort.Search(len(x.logs), func(i int) bool {
		x.work.Add("guardian rows looked at", 1)
		order, ok := x.logs[i].compare(sqlitefile.IntValue(high))
		return ok && order > 0
	})
	return from, max(from, until)
}

// callsByThread returns the calls of the spans of each thread the rows of
// logs name, each thread's in the order of the rows, in one pass over the
// rows, counting each row looked at in work.
func callsByThread(logs []guardianLog, work parserkit.Work) map[string]*parentCalls {
	parents := map[string]*parentCalls{}
	for position, each := range logs {
		work.Add("guardian rows looked at", 1)
		for _, span := range each.spans {
			if span.call == "" {
				continue
			}
			found, ok := parents[span.thread]
			if !ok {
				found = &parentCalls{}
				parents[span.thread] = found
			}
			found.rows = append(found.rows, position)
			found.calls = append(found.calls, span.call)
		}
	}
	for _, found := range parents {
		found.other = otherCalls(found.calls)
	}
	return parents
}

// otherCalls returns, for each call, the first position at or after it
// whose call differs, len(calls) when none.
func otherCalls(calls []string) []int {
	other := make([]int, len(calls))
	for i := len(calls) - 1; i >= 0; i-- {
		other[i] = len(calls)
		if i+1 < len(calls) {
			other[i] = i + 1
			if calls[i+1] == calls[i] {
				other[i] = other[i+1]
			}
		}
	}
	return other
}

// parent returns the calls of the reviewed thread's spans, none for a
// thread no span names.
func (x *guardianIndex) parent(thread string) *parentCalls {
	if found, ok := x.parents[thread]; ok {
		return found
	}
	return &parentCalls{}
}

// windowCalls returns the distinct calls of the thread's spans in the rows
// from low to high seconds: none, the one, or two of them when there are
// more; false when a row of the window has a body no query could return,
// as such a query fails.
func (x *guardianIndex) windowCalls(thread string, low, high int64) ([]string, bool) {
	from, until := x.window(low, high)
	if x.unreadable[until] > x.unreadable[from] {
		return nil, false
	}
	found := x.parent(thread)
	first := sort.SearchInts(found.rows, from)
	last := sort.SearchInts(found.rows, until)
	switch {
	case first == last:
		return nil, true
	case found.other[first] < last:
		return []string{found.calls[first], found.calls[found.other[first]]}, true
	}
	return found.calls[first : first+1], true
}

// guardianCalls returns the distinct tool_call_id values of the reviewed
// thread's guardian spans in the rows logged from 2 seconds before start
// to 2 seconds after end (both timeline timestamps), the ts compared as
// the database compares it: none, the one, or two of them when there are
// more. A database with a row in that time whose body is not valid text
// gives none, as a query of those rows fails. None when either time
// cannot be read, without reading the logs.
func guardianCalls(logs func() []*guardianIndex, thread, start, end string) []string {
	startMillis, startOK := timeutil.EpochMillis(start)
	endMillis, endOK := timeutil.EpochMillis(end)
	if !startOK || !endOK {
		return nil
	}
	low, high := floorDiv(startMillis, 1000)-2, floorDiv(endMillis, 1000)+2
	var calls []string
	for _, database := range logs() {
		found, ok := database.windowCalls(thread, low, high)
		if !ok {
			continue
		}
		for _, call := range found {
			if !slices.Contains(calls, call) && len(calls) < 2 {
				calls = append(calls, call)
			}
		}
	}
	return calls
}

// guardianRows returns the guardian review rows of the database at
// logPath, in rowid order, and whether its ts column has TEXT affinity;
// none when the database cannot be read. The rows are taken as the scan
// meets them, unprojected, so a body that is not valid text fails only
// the times that would have returned it.
func guardianRows(logPath string) ([]guardianLog, bool) {
	database, err := parserkit.OpenDatabase(logPath)
	if err != nil {
		return nil, false
	}
	defer database.Close()
	table, body, err := logsTableOf(database)
	if table == nil || err != nil {
		return nil, false
	}
	var rows []sqlitefile.Row
	err = table.Select().Where(func(row sqlitefile.Row) bool {
		if sqlitefile.Instr(row.Get(body), sqlitefile.TextValue(guardianMark)) > 0 {
			rows = append(rows, row)
		}
		return false
	}).Each(func(sqlitefile.Row) error { return nil })
	if err != nil {
		return nil, false
	}
	slices.SortStableFunc(rows, func(left, right sqlitefile.Row) int { return cmp.Compare(left.Rowid, right.Rowid) })
	logs := make([]guardianLog, 0, len(rows))
	for _, row := range rows {
		text := row.Get(body)
		each := guardianLog{time: row.Get("ts"), compare: func(operand sqlitefile.Value) (int, bool) {
			return row.Cmp("ts", operand)
		}, readable: text.Type != sqlitefile.Text || utf8.Valid(text.Bytes)}
		for _, match := range guardianSpan.FindAllStringSubmatch(parserkit.NonEmptyColumn(text), -1) {
			fields := logFields(match[1])
			each.spans = append(each.spans, logSpan{thread: fields["thread_id"], call: fields["tool_call_id"]})
		}
		logs = append(logs, each)
	}
	column, found := parserkit.ColumnOf(table, "ts")
	return logs, found && column.Affinity == sqlitefile.TextAffinity
}

// guardianIndexes returns the indexes of the logs_<N>.sqlite databases in
// home, in name order. The home is listed and each database read once for
// the parser's life, which is one run, since every guardian review thread
// of the home looks them up.
func (p *Parser) guardianIndexes(home string) []*guardianIndex {
	if indexes, ok := p.guardians[home]; ok {
		return indexes
	}
	if p.guardians == nil {
		p.guardians = map[string][]*guardianIndex{}
	}
	p.Work.Add("codex home listings", 1)
	indexes := []*guardianIndex{}
	for _, logPath := range logDatabases(home) {
		p.Work.Add("guardian log databases read", 1)
		logs, textTimes := guardianRows(logPath)
		indexes = append(indexes, newGuardianIndex(logs, textTimes, p.Work))
	}
	p.guardians[home] = indexes
	return indexes
}
