package dedup

import (
	"cmp"
	"crypto/sha256"
	"iter"
	"math"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// noMatch is the match time of a row no primary row can take: below any
// time EpochMillis returns, since the year 1 is about -6.2e13 milliseconds.
const noMatch = math.MinInt64

// digestMark starts the match value of a text longer than sha256.Size
// bytes, which makes that value one byte longer than any text compared as
// it is.
const digestMark = "#"

// windowLimit bounds a window in milliseconds, so that a time plus or minus
// a window never overflows; it is longer than the years 1 to 9999.
const windowLimit = 1 << 62

// matcher runs one deduplication with the hooks the agents declared, adding
// the work its lookups do to steps.
type matcher struct {
	hooks Hooks
	steps *int
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

// The scopes: a row's session, its project, none, and its session and
// project together, which lets a lookup under a broader scope ask for
// exactly the sessions and projects it may take.
const (
	scopeSession scopeKind = iota
	scopeProject
	scopeAny
	scopePair
)

// scope is a scope kind with the session id and project path it names.
type scope struct {
	kind             scopeKind
	session, project string
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

// bucket holds the candidates of one key, apart by whether they carry a
// tool_use_id, since a lookup by text for a row that carries one may take
// only those without.
type bucket struct {
	parts [2]bucketPart
}

// bucketPart holds candidates of one key: the dated ones filed before a
// pass's lookups began (settled, sorted once by time then row index), those
// filed during the lookups (arriving, already in that order), and the
// undated ones by when they were filed.
type bucketPart struct {
	settled, arriving, undated *candidateList
}

// newBucket returns an empty bucket.
func newBucket() *bucket {
	found := &bucket{}
	for i := range found.parts {
		found.parts[i] = bucketPart{newCandidateList(), newCandidateList(), newCandidateList()}
	}
	return found
}

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

// byText returns a row's identity by text, in its agent's match form: the
// row's text as it reads in full when the form leaves it as it is.
func (m matcher) byText(row timeline.Row) identity {
	turnType, text := row.TurnType, row.Text
	if form := m.hooks[row.Agent].Form; form != nil {
		turnType, text = form(row)
	}
	value := ""
	if row.Shared != nil && text == row.Text {
		value = matchValue(row.Pieces())
	} else {
		value = matchValue(func(yield func(string) bool) { yield(text) })
	}
	return identity{host: row.Host, user: row.User, agent: row.Agent, turnType: turnType,
		value: value}
}

// matchValue returns what two texts given in pieces are compared by: the
// text itself up to sha256.Size bytes, else digestMark and its SHA-256
// digest, so a long text is held in a fixed size however many rows name it,
// and a text never equals the value of a longer one.
func matchValue(pieces iter.Seq[string]) string {
	length := 0
	for piece := range pieces {
		length += len(piece)
	}
	if length <= sha256.Size {
		var short strings.Builder
		for piece := range pieces {
			short.WriteString(piece)
		}
		return short.String()
	}
	digest := sha256.New()
	// Each piece is copied through one small buffer, never whole.
	var chunk [4096]byte
	for piece := range pieces {
		for piece != "" {
			n := copy(chunk[:], piece)
			digest.Write(chunk[:n])
			piece = piece[n:]
		}
	}
	return digestMark + string(digest.Sum(nil))
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

// add files a candidate row under its text and id keys at every scope. A
// candidate filed before the pass's lookups begin is settled; one filed
// during them arrives after every dated candidate already filed, since the
// pass tests rows in the order of their match times. A row at noMatch is
// filed nowhere, so no row pairs with it.
func (m matcher) add(state *pass, row timeline.Row, index int, millis int64, dated,
	arriving bool) {
	if dated && millis == noMatch {
		return
	}
	identities := []identity{m.byText(row)}
	if hasID(row) {
		identities = append(identities, byID(row))
	}
	scopes := []scope{{kind: scopeSession, session: row.SessionID},
		{kind: scopeProject, project: row.ProjectPath}, {kind: scopeAny},
		{kind: scopePair, session: row.SessionID, project: row.ProjectPath}}
	state.filed++
	for _, ident := range identities {
		for _, where := range scopes {
			key := bucketKey{ident, where}
			found := state.buckets[key]
			if found == nil {
				found = newBucket()
				state.buckets[key] = found
			}
			part := &found.parts[boolInt(hasID(row))]
			switch {
			case !dated:
				part.undated.push(stamp{int64(state.filed), index})
			case arriving:
				part.arriving.push(stamp{millis, index})
			default:
				part.settled.push(stamp{millis, index})
			}
		}
	}
}

// compareStamps orders stamps by time, then row index.
func compareStamps(left, right stamp) int {
	return cmp.Or(cmp.Compare(left.millis, right.millis), cmp.Compare(left.index, right.index))
}

// boolInt returns 1 for true and 0 for false.
func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
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

// pair returns the candidate of the parts that a secondary row pairs with
// under the time rule.
func (m matcher) pair(query *search, parts []*bucketPart, row timeline.Row, tolerance float64,
	forward window, againstPrimary bool) (int, bool) {
	millis, dated := m.matchTime(row, againstPrimary)
	if len(parts) == 0 || (dated && millis == noMatch) {
		return -1, false
	}
	if !dated {
		return query.earliest(parts)
	}
	back := millisOf(tolerance)
	ahead := back
	if forward.set {
		ahead = millisOf(forward.seconds)
	}
	return query.nearest(parts, millis, millis-back, millis+ahead)
}
