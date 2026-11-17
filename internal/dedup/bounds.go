package dedup

import (
	"slices"
	"sort"

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

// upper returns the earliest time after millis of a secondary row of the
// same text that could take candidate, and false when there is none.
func (b *bounds) upper(text identity, millis int64, candidate timeline.Row) (int64, bool) {
	lists := [][]int64{b.byNone[text]}
	if candidate.SessionID != "" {
		lists = append(lists, b.bySession[scopedText{text, candidate.SessionID}])
	}
	if candidate.ProjectPath != "" {
		lists = append(lists, b.byProjectOnly[scopedText{text, candidate.ProjectPath}])
	}
	later, found := int64(0), false
	for _, times := range lists {
		position := sort.Search(len(times), func(i int) bool { return times[i] > millis })
		if position < len(times) && (!found || times[position] < later) {
			later, found = times[position], true
		}
	}
	return later, found
}

// candidateBound returns the bound of a widened row by candidate index,
// or nil when the row is undated.
func (m matcher) candidateBound(limits *bounds, primary []timeline.Row,
	row timeline.Row) upperBound {
	millis, dated := m.matchTime(row, true)
	if !dated {
		return nil
	}
	text := m.byText(row)
	return func(index int) (int64, bool) {
		return limits.upper(text, millis, primary[index])
	}
}
