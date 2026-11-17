package dedup

import (
	"math"
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// noMatch is the match time of a row no primary row can take: below any
// time EpochMillis returns, since the year 1 is about -6.2e13 milliseconds.
const noMatch = math.MinInt64

// windowLimit bounds a window in milliseconds, so that a time plus or minus
// a window never overflows; it is longer than the years 1 to 9999.
const windowLimit = 1 << 62

// matcher runs one deduplication with the hooks the agents declared.
type matcher struct {
	hooks Hooks
}

// identity is what two rows must share to pair, apart from scope: host,
// user, agent and turn type, with either the text (byID false) or the
// tool_use_id (byID true) as value.
type identity struct {
	host, user, agent, turnType string
	byID                        bool
	value                       string
}

// scopeKind is the scope a bucket key is filed under.
type scopeKind uint8

// The scopes, from the narrowest.
const (
	scopeSession scopeKind = iota
	scopeProject
	scopeAny
)

// scope is a scope kind with the session id or project path it names.
type scope struct {
	kind  scopeKind
	value string
}

// bucketKey is an identity under a scope: the key candidates are filed
// under and looked up by.
type bucketKey struct {
	identity
	scope
}

// stamp is a dated candidate: its match time and its row index.
type stamp struct {
	millis int64
	index  int
}

// bucket holds the candidates of one key: the dated ones sorted by time
// then index, the undated ones in the order they were filed.
type bucket struct {
	dated   []stamp
	undated []int
}

// isUnavailable reports whether the candidate at a row index may not be
// taken.
type isUnavailable func(index int) bool

// upperBound returns the time at or after which the candidate at a row
// index may not be taken, and false when there is none.
type upperBound func(index int) (int64, bool)

// matchTime returns the epoch milliseconds a row is matched at and true,
// noMatch and true for a row its agent's Anchor hook keeps from every
// primary row, or false when the row is undated. noMatch is returned only
// againstPrimary; otherwise the row's own time stands in.
func (m matcher) matchTime(row timeline.Row, againstPrimary bool) (int64, bool) {
	if hook := m.hooks[row.Agent].Anchor; hook != nil {
		switch kind, millis := hook(row); {
		case kind == AtTime && (millis != noMatch || againstPrimary):
			return millis, true
		case kind == NeverMatch && againstPrimary:
			return noMatch, true
		}
	}
	return timeutil.EpochMillis(row.TimestampUTC)
}

// byText returns a row's identity by text, in its agent's match form.
func (m matcher) byText(row timeline.Row) identity {
	turnType, text := row.TurnType, row.Text
	if form := m.hooks[row.Agent].Form; form != nil {
		turnType, text = form(row)
	}
	return identity{host: row.Host, user: row.User, agent: row.Agent, turnType: turnType,
		value: text}
}

// byID returns a row's identity by tool_use_id.
func byID(row timeline.Row) identity {
	return identity{host: row.Host, user: row.User, agent: row.Agent, turnType: row.TurnType,
		byID: true, value: row.ToolUseID}
}

// hasID reports whether a row is a tool call or result with a tool_use_id.
func hasID(row timeline.Row) bool {
	return (row.TurnType == timeline.TurnToolUse || row.TurnType == timeline.TurnToolResult) &&
		row.ToolUseID != ""
}

// isParserRow reports whether a row is a parser: row, which never pairs,
// since it describes its own file.
func isParserRow(row timeline.Row) bool {
	return row.TurnType == timeline.TurnSystem && strings.HasPrefix(row.Text, "parser:")
}

// scopeRank returns 0 for a row with a session id, 1 for one with only a
// project, else 2.
func scopeRank(row timeline.Row) int {
	switch {
	case row.SessionID != "":
		return 0
	case row.ProjectPath != "":
		return 1
	}
	return 2
}

// add files a candidate row under its text and id keys at every scope,
// keeping each bucket's dated candidates sorted. A row at noMatch is filed
// nowhere, so no row pairs with it.
func (m matcher) add(buckets map[bucketKey]*bucket, row timeline.Row, index int,
	millis int64, dated bool) {
	if dated && millis == noMatch {
		return
	}
	identities := []identity{m.byText(row)}
	if hasID(row) {
		identities = append(identities, byID(row))
	}
	scopes := []scope{{scopeSession, row.SessionID}, {scopeProject, row.ProjectPath},
		{kind: scopeAny}}
	for _, ident := range identities {
		for _, where := range scopes {
			key := bucketKey{ident, where}
			found := buckets[key]
			if found == nil {
				found = &bucket{}
				buckets[key] = found
			}
			if !dated {
				found.undated = append(found.undated, index)
				continue
			}
			entry := stamp{millis, index}
			position, _ := slices.BinarySearchFunc(found.dated, entry, compareStamps)
			found.dated = slices.Insert(found.dated, position, entry)
		}
	}
}

// compareStamps orders stamps by time, then row index.
func compareStamps(left, right stamp) int {
	switch {
	case left.millis != right.millis:
		return compareInts(left.millis, right.millis)
	case left.index < right.index:
		return -1
	case left.index > right.index:
		return 1
	}
	return 0
}

// compareInts returns -1, 0 or 1 as left is below, equal to or above right.
func compareInts(left, right int64) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	}
	return 0
}

// nearest returns the row index of the available dated candidate nearest
// in time, at most back milliseconds before millis or forward after it,
// the earlier on a tie, and before the bound upper gives for it.
func nearest(dated []stamp, millis, back, forward int64, unavailable isUnavailable,
	upper upperBound) (int, bool) {
	position, _ := slices.BinarySearchFunc(dated, millis-back,
		func(entry stamp, target int64) int { return compareInts(entry.millis, target) })
	best, bestDistance := -1, int64(0)
	for ; position < len(dated) && dated[position].millis <= millis+forward; position++ {
		candidate := dated[position]
		if unavailable(candidate.index) {
			continue
		}
		if upper != nil {
			if limit, ok := upper(candidate.index); ok && candidate.millis >= limit {
				continue
			}
		}
		distance := candidate.millis - millis
		if distance < 0 {
			distance = -distance
		}
		if best < 0 || distance < bestDistance {
			best, bestDistance = candidate.index, distance
		}
	}
	return best, best >= 0
}

// earliest returns the earliest available candidate of a bucket, the
// undated ones last.
func earliest(found *bucket, unavailable isUnavailable) (int, bool) {
	for _, entry := range found.dated {
		if !unavailable(entry.index) {
			return entry.index, true
		}
	}
	for _, index := range found.undated {
		if !unavailable(index) {
			return index, true
		}
	}
	return -1, false
}

// millisOf returns a window in seconds as whole milliseconds, truncated
// toward zero, within plus or minus windowLimit.
func millisOf(seconds float64) int64 {
	switch value := seconds * 1000; {
	case math.IsNaN(value):
		return 0
	case value >= windowLimit:
		return windowLimit
	case value <= -windowLimit:
		return -windowLimit
	default:
		return int64(value)
	}
}

// window is a forward window in seconds, set only when a hook widened it.
type window struct {
	seconds float64
	set     bool
}

// pair returns the available candidate in a bucket that a secondary row
// pairs with under the time rule.
func (m matcher) pair(found *bucket, row timeline.Row, tolerance float64,
	unavailable isUnavailable, upper upperBound, forward window, againstPrimary bool) (int, bool) {
	millis, dated := m.matchTime(row, againstPrimary)
	if found == nil || (dated && millis == noMatch) {
		return -1, false
	}
	if !dated {
		return earliest(found, unavailable)
	}
	back := millisOf(tolerance)
	ahead := back
	if forward.set {
		ahead = millisOf(forward.seconds)
	}
	return nearest(found.dated, millis, back, ahead, unavailable, upper)
}
