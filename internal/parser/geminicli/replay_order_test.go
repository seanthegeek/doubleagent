package geminicli

import (
	"math/rand/v2"
	"slices"
	"strconv"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
)

// referenceLog keeps the replay's order as a slice of ids rewritten by
// each change: the direct reading of the recording service's rules, kept
// to check the place numbers of messageLog against. visited counts the
// ids it reads.
type referenceLog struct {
	order   []string
	byID    map[string]*stored
	visited int
}

// put adds a new id last and gives a known one the new message and line.
func (m *referenceLog) put(message jsonvalue.Value, line int, sessionID string) {
	messageID := message.Get("id").StringValue()
	if entry, ok := m.byID[messageID]; ok {
		entry.message, entry.line = message, line
		return
	}
	m.order = append(m.order, messageID)
	m.byID[messageID] = &stored{message: message, line: line, sessionID: sessionID}
}

// drop removes the ids present and returns how many it removed.
func (m *referenceLog) drop(ids []string) int {
	removed := map[string]bool{}
	for _, messageID := range ids {
		if _, ok := m.byID[messageID]; ok {
			delete(m.byID, messageID)
			removed[messageID] = true
		}
	}
	if len(removed) > 0 {
		m.visited += len(m.order)
		m.order = slices.DeleteFunc(m.order, func(messageID string) bool { return removed[messageID] })
	}
	return len(removed)
}

// dropFrom removes the target and every id after it, or every id when the
// target is not present.
func (m *referenceLog) dropFrom(target string) int {
	m.visited += len(m.order)
	cut := max(slices.Index(m.order, target), 0)
	return m.drop(slices.Clone(m.order[cut:]))
}

// moveToEnd moves the ids present to the end in the order of their last
// mention.
func (m *referenceLog) moveToEnd(ids []string) {
	last := map[string]int{}
	for i, messageID := range ids {
		if _, ok := m.byID[messageID]; ok {
			last[messageID] = i
		}
	}
	if len(last) == 0 {
		return
	}
	moved := make([]string, 0, len(last))
	for messageID := range last {
		moved = append(moved, messageID)
	}
	slices.SortFunc(moved, func(left, right string) int { return last[left] - last[right] })
	m.visited += len(m.order)
	m.order = append(slices.DeleteFunc(m.order, func(messageID string) bool {
		_, ok := last[messageID]
		return ok
	}), moved...)
}

// orderChange is one change a replay makes to its messages.
type orderChange struct {
	kind string // "put", "drop", "rewind" or "move"
	ids  []string
	line int
}

// randomChanges returns count changes over a few ids, so that ids come
// back, move, go and are named when absent.
func randomChanges(source *rand.Rand, count int) []orderChange {
	someIDs := func() []string {
		ids := make([]string, source.IntN(4))
		for i := range ids {
			ids[i] = "m" + strconv.Itoa(source.IntN(8))
		}
		return ids
	}
	changes := make([]orderChange, 0, count)
	for line := 1; line <= count; line++ {
		change := orderChange{line: line, ids: someIDs()}
		switch roll := source.IntN(10); {
		case roll < 5:
			change.kind, change.ids = "put", []string{"m" + strconv.Itoa(source.IntN(8))}
		case roll < 7:
			change.kind = "move"
		case roll < 9:
			change.kind = "drop"
		default:
			change.kind, change.ids = "rewind", []string{"m" + strconv.Itoa(source.IntN(9))}
		}
		changes = append(changes, change)
	}
	return changes
}

// message returns a message with an id whose content names its line.
func message(messageID string, line int) jsonvalue.Value {
	return jsonvalue.NewObject(jsonvalue.Member{Key: "id", Value: jsonvalue.NewString(messageID)},
		jsonvalue.Member{Key: "line", Value: jsonvalue.NewInt(int64(line))})
}

// stateText returns the messages of a log in order with their lines and
// sessions, for comparing two logs.
func stateText(order []string, byID map[string]*stored) []string {
	out := make([]string, 0, len(order))
	for _, messageID := range order {
		entry := byID[messageID]
		out = append(out, messageID+"@"+strconv.Itoa(entry.line)+"/"+entry.sessionID+" "+entry.message.JSON())
	}
	return out
}

func TestThePlacesOfTheReplayKeepTheOrderOfTheRewrittenList(t *testing.T) {
	for seed := range uint64(300) {
		t.Run("changes from seed "+strconv.FormatUint(seed, 10), func(t *testing.T) {
			source := rand.New(rand.NewPCG(seed, 527))
			placed, reference := newMessageLog(parser.Work{}), referenceLog{byID: map[string]*stored{}}
			for _, change := range randomChanges(source, 200) {
				session := "s" + strconv.Itoa(change.line%3)
				var got, want int
				switch change.kind {
				case "put":
					placed.put(message(change.ids[0], change.line), change.line, session)
					reference.put(message(change.ids[0], change.line), change.line, session)
				case "move":
					placed.moveToEnd(change.ids)
					reference.moveToEnd(change.ids)
				case "drop":
					got, want = placed.drop(change.ids), reference.drop(change.ids)
				case "rewind":
					got, want = placed.dropFrom(change.ids[0]), reference.dropFrom(change.ids[0])
				}
				gotState := stateText(placed.order(), placed.byID)
				wantState := stateText(reference.order, reference.byID)
				if got != want || !slices.Equal(gotState, wantState) {
					t.Fatalf("after %s %q at line %d: %d removed and %q, want %d and %q", change.kind,
						change.ids, change.line, got, gotState, want, wantState)
				}
			}
		})
	}
}

func TestTheReplaysOrderReadsEachPlacementOnce(t *testing.T) {
	const size = 2000
	placed := newMessageLog(parser.CountingWork())
	for i := range size {
		placed.put(message("m"+strconv.Itoa(i), i), i, "s")
	}
	for range size {
		placed.moveToEnd([]string{"m0"})
	}
	for i := range size {
		extra := "x" + strconv.Itoa(i)
		placed.put(message(extra, i), i, "s")
		placed.dropFrom(extra)
	}
	order := placed.order()
	// Each rewind reads only its own placement, and the order reads the
	// size messages' placements and the size moves'.
	if len(order) != size || order[size-1] != "m0" || placed.work.Count("placements read") != 3*size {
		t.Errorf("%d placements read for %d messages, %d moves and %d rewinds, want %d", placed.work.Count("placements read"),
			size, size, size, 3*size)
	}
}
