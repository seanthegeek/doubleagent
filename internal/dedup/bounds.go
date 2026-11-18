package dedup

import (
	"slices"

	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// scopedText is a match identity by text with the session id or project
// path of the rows filed under it.
type scopedText struct {
	text  identity
	where string
}

// bounds holds the match times of the secondary rows by identity and
// scope, each list sorted. A widened row may not take a primary row
// recorded at or after the next later secondary row of the same text that
// could take that same primary row, so the later line keeps the later
// record. Who could take it follows the candidate's scope: a row of
// another session never counts, and a copy at the same time does not bound.
type bounds struct {
	bySession     map[scopedText][]int64
	byProjectOnly map[scopedText][]int64
	byNone        map[identity][]int64
}

// newBounds files the dated secondary rows that can match a primary row.
func (m matcher) newBounds(secondary []Secondary) *bounds {
	limits := &bounds{bySession: map[scopedText][]int64{},
		byProjectOnly: map[scopedText][]int64{}, byNone: map[identity][]int64{}}
	for _, item := range secondary {
		row := item.Row
		millis, dated := m.matchTime(row, true)
		if !dated || millis == noMatch || isParserRow(row) {
			continue
		}
		text := m.byText(row)
		switch {
		case row.SessionID != "":
			key := scopedText{text, row.SessionID}
			limits.bySession[key] = append(limits.bySession[key], millis)
		case row.ProjectPath != "":
			key := scopedText{text, row.ProjectPath}
			limits.byProjectOnly[key] = append(limits.byProjectOnly[key], millis)
		default:
			limits.byNone[text] = append(limits.byNone[text], millis)
		}
	}
	for _, times := range limits.bySession {
		slices.Sort(times)
	}
	for _, times := range limits.byProjectOnly {
		slices.Sort(times)
	}
	for _, times := range limits.byNone {
		slices.Sort(times)
	}
	return limits
}

// after returns the earliest time after millis of a secondary row of the
// same text that could take a candidate of a session and project, and
// false when there is none.
func (b *bounds) after(text identity, millis int64, session, project string) (int64, bool) {
	lists := [][]int64{b.byNone[text]}
	if session != "" {
		lists = append(lists, b.bySession[scopedText{text, session}])
	}
	if project != "" {
		lists = append(lists, b.byProjectOnly[scopedText{text, project}])
	}
	later, found := int64(0), false
	for _, times := range lists {
		position, _ := slices.BinarySearchFunc(times, millis, func(moment, target int64) int {
			if moment <= target {
				return -1
			}
			return 1
		})
		if position < len(times) && (!found || times[position] < later) {
			later, found = times[position], true
		}
	}
	return later, found
}

// forwardBound is the bound of one widened row at millis: the time from
// which it may not take a candidate.
type forwardBound struct {
	limits  *bounds
	text    identity
	millis  int64
	primary []timeline.Row
}

// forCandidate returns the bound for the candidate at a primary row index.
func (f *forwardBound) forCandidate(index int) (int64, bool) {
	candidate := f.primary[index]
	return f.limits.after(f.text, f.millis, candidate.SessionID, candidate.ProjectPath)
}

// shared returns a bound that holds for every candidate filed under where:
// each one's own bound is at most this.
func (f *forwardBound) shared(where scope) (int64, bool) {
	return f.limits.after(f.text, f.millis, where.session, where.project)
}

// candidateBound returns the bound of a widened row, or nil when the row is
// undated.
func (m matcher) candidateBound(limits *bounds, primary []timeline.Row,
	row timeline.Row) *forwardBound {
	millis, dated := m.matchTime(row, true)
	if !dated {
		return nil
	}
	return &forwardBound{limits, m.byText(row), millis, primary}
}
