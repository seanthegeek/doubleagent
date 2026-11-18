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
// window for some rows (bounded by the next secondary row of the same
// text), or the time a row is matched at. The full rule is in
// analyzer/docs/output.md under "Secondary sources".
package dedup

import (
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
// kept that is not a system row is returned with Recovered "yes". The
// arguments are not changed.
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
	for _, index := range run.againstPrimary(primary, secondary, limits, forward) {
		dropped[index] = true
	}
	for _, index := range run.amongSecondary(secondary, dropped) {
		dropped[index] = true
	}
	counts := map[string]int{}
	kept := append(make([]timeline.Row, 0, len(primary)+len(secondary)-len(dropped)),
		primary...)
	for index, item := range secondary {
		if dropped[index] {
			counts[item.Row.Agent]++
			continue
		}
		row := item.Row
		if row.TurnType != timeline.TurnSystem {
			row.Recovered = "yes"
		}
		kept = append(kept, row)
	}
	return kept, counts, steps
}
