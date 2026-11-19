// Package dedup drops the rows of secondary sources that a primary store
// also records. A secondary source is a file a parser reads beside the
// agent's own transcript store that repeats some of its events: a
// per-prompt history, a mirror or a projection. Every source is parsed on
// every run; this package keeps each event once.
//
// A secondary row is dropped when a primary row has the same identity
// (host, user, agent, turn type and text, or the same tool_use_id for two
// tool rows that both carry one), the same scope (session, else project,
// else none) and a time within the source's tolerance. A secondary row with
// no time matches on identity and scope alone. Matching is one-to-one,
// timestamped rows first, each taking the nearest unused candidate, the
// earlier one on a tie.
//
// The secondary rows no primary row took are then matched against each
// other, looking under broader scopes too, so the several copies of one
// event that backups and mirrors keep are shown once. A row pairs only with
// a row from a different source file, and a parser: row is never dropped
// or taken. Every secondary row kept that is not a system row is marked
// Recovered "yes".
//
// An agent whose secondary source writes events differently from its
// primary store declares AgentHooks: another match form, a wider forward
// window for some rows, or the time a row is matched at. A widened row may
// not take a primary row at or after the next secondary row of the same
// text that could take that primary row, so the later line keeps the later
// record.
package dedup

import (
	"github.com/seanthegeek/doubleagent/internal/secrets"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// ToleranceSeconds is the default window, in seconds either side, within
// which a secondary row's time may differ from its primary row's: one
// agent writes its history in whole seconds beside the millisecond times
// of its transcript, and another's two records of one prompt differ by
// under a second.
const ToleranceSeconds = 2.0

// Secondary is one row of a secondary source with the tolerance, in
// seconds, of the source it came from.
type Secondary struct {
	Row       timeline.Row
	Tolerance float64
}

// AnchorKind says which time an Anchor hook matches a row at against the
// primary store.
type AnchorKind uint8

// The answers an Anchor hook gives.
const (
	// OwnTime matches the row at its own timestamp.
	OwnTime AnchorKind = iota
	// AtTime matches the row at the epoch milliseconds the hook returns.
	AtTime
	// NeverMatch lets no primary row take the row; among secondary copies
	// it matches at its own timestamp.
	NeverMatch
)

// AgentHooks are what one agent's parser declares when its secondary
// source records events in another form or at another moment than its
// primary store. A nil hook keeps the default.
type AgentHooks struct {
	// Form returns the turn type and text a row is matched under in place
	// of its own.
	Form func(row timeline.Row) (turnType, text string)
	// Window returns a forward window in seconds for a secondary row, and
	// false for the source's own tolerance; a window no wider than the
	// tolerance is ignored.
	Window func(row timeline.Row) (seconds float64, ok bool)
	// Anchor returns the time a row is matched at against the primary
	// store, with the epoch milliseconds when the kind is AtTime.
	Anchor func(row timeline.Row) (kind AnchorKind, millis int64)
}

// Hooks holds the AgentHooks of every agent that declares some, by agent
// name. The parser registry builds it from what each parser declares, so
// two parsers never import each other to supply their forms.
type Hooks map[string]AgentHooks

// Deduplicate returns the rows to keep and the number of secondary rows
// dropped per agent. The rows kept are every primary row, then the
// secondary rows with no match, each in the order given; a secondary row
// kept that is not a system row is returned with Recovered "yes". A row
// kept for a duplicate dropped carries the secrets records of both
// (timeline.Row.Secrets). The arguments are not changed.
func Deduplicate(primary []timeline.Row, secondary []Secondary, hooks Hooks) ([]timeline.Row,
	map[string]int) {
	kept, counts, _ := deduplicate(primary, secondary, hooks)
	return kept, counts
}

// deduplicate is Deduplicate, also returning the work its lookups did: the
// candidates they looked at, the jumps they made over candidates already
// refused, and the comparisons that sorted the candidates.
func deduplicate(primary []timeline.Row, secondary []Secondary, hooks Hooks) ([]timeline.Row,
	map[string]int, int) {
	steps := 0
	run := matcher{hooks: hooks, steps: &steps}
	forward := run.forwardWindows(secondary)
	var limits *bounds
	if len(forward) > 0 {
		limits = run.newBounds(secondary)
	}
	dropped := map[int]bool{}
	intoPrimary := run.againstPrimary(primary, secondary, limits, forward)
	for index := range intoPrimary {
		dropped[index] = true
	}
	intoSecondary := run.amongSecondary(secondary, dropped)
	for index := range intoSecondary {
		dropped[index] = true
	}
	counts := map[string]int{}
	kept := append(make([]timeline.Row, 0, len(primary)+len(secondary)-len(dropped)),
		primary...)
	keptAt := map[int]int{}
	for index, item := range secondary {
		if dropped[index] {
			counts[item.Row.Agent]++
			continue
		}
		row := item.Row
		if row.TurnType != timeline.TurnSystem {
			row.Recovered = "yes"
		}
		keptAt[index] = len(kept)
		kept = append(kept, row)
	}
	carryRecords(kept, secondary, intoPrimary, intoSecondary, keptAt)
	return kept, counts, steps
}

// carryRecords gives each kept row the secrets records of the rows dropped
// as its duplicates (timeline.Row.Secrets), so a secret only a dropped
// row's parse identified is replaced in the text that stands for both. A
// dropped row pairs with a primary row (intoPrimary) or with another
// secondary row (intoSecondary), itself kept (at keptAt) or dropped in turn;
// the chain is followed to the row kept, each link once.
func carryRecords(kept []timeline.Row, secondary []Secondary, intoPrimary, intoSecondary, keptAt map[int]int) {
	target := map[int]int{}
	merged := map[[2]*secrets.Found]*secrets.Found{}
	for index, item := range secondary {
		if _, isKept := keptAt[index]; isKept || item.Row.Secrets == nil {
			continue
		}
		row := &kept[keptPosition(index, intoPrimary, intoSecondary, keptAt, target)]
		pair := [2]*secrets.Found{row.Secrets, item.Row.Secrets}
		if merged[pair] == nil {
			merged[pair] = secrets.Merge(pair[0], pair[1])
		}
		row.Secrets = merged[pair]
	}
}

// keptPosition returns the position in the kept rows of the row that the
// secondary row at index stands as, following its chain of pairs to a
// primary or kept row, and notes in target the position of each row on the
// way, so each link is followed once over all calls.
func keptPosition(index int, intoPrimary, intoSecondary, keptAt, target map[int]int) int {
	var chain []int
	position := -1
	for position < 0 {
		if known, ok := target[index]; ok {
			position = known
			break
		}
		chain = append(chain, index)
		if primaryIndex, ok := intoPrimary[index]; ok {
			position = primaryIndex
		} else if keptIndex, ok := keptAt[index]; ok {
			position = keptIndex
		} else {
			index = intoSecondary[index]
		}
	}
	for _, each := range chain {
		target[each] = position
	}
	return position
}
