package dedup

import (
	"github.com/seanthegeek/doubleagent/internal/secrets"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

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
	// Form writes the text a row is matched under to text, in place of its
	// own, and returns the turn type it is matched under. A shared text the
	// row names goes in as a segment (MatchText.Segment), so it is read
	// once however many rows name it.
	Form func(row timeline.Row, text *MatchText) (turnType string)
	// Window returns a forward window in seconds for a secondary row, and
	// false for the source's own tolerance; a window no wider than the
	// tolerance is ignored. What it learns from a shared text it keeps in
	// texts (Texts.Memo).
	Window func(row timeline.Row, texts *Texts) (seconds float64, ok bool)
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
	kept, counts, _ := deduplicate(primary, secondary, hooks, newTexts(randomBase()))
	return kept, counts
}

// deduplicate is Deduplicate with the run's Texts, also returning the work
// its lookups did: the candidates they looked at, the jumps they made over
// candidates already refused, and the comparisons that sorted the
// candidates.
func deduplicate(primary []timeline.Row, secondary []Secondary, hooks Hooks,
	texts *Texts) ([]timeline.Row, map[string]int, int) {
	steps := 0
	run := matcher{hooks: hooks, steps: &steps, texts: texts}
	primaryText := make([]identity, len(primary))
	for index, row := range primary {
		primaryText[index] = run.byText(row)
	}
	run.secondaryText = make([]identity, len(secondary))
	for index, item := range secondary {
		run.secondaryText[index] = run.byText(item.Row)
	}
	forward := run.forwardWindows(secondary)
	var limits *bounds
	if len(forward) > 0 {
		limits = run.newBounds(secondary)
	}
	dropped := map[int]bool{}
	intoPrimary := run.againstPrimary(primary, primaryText, secondary, limits, forward)
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
// the chain is followed to the row kept, each link once. A kept row lists
// the records (secrets.WithDuplicates) rather than holding a merged copy,
// so carrying costs the number of rows dropped, not what their records hold;
// writing the row costs its length a fixed number of times plus, per record
// listed, at most the smaller of the record's size and the row's length.
func carryRecords(kept []timeline.Row, secondary []Secondary, intoPrimary, intoSecondary, keptAt map[int]int) {
	target := map[int]int{}
	carried := map[int][]*secrets.Found{}
	for index, item := range secondary {
		if _, isKept := keptAt[index]; isKept || item.Row.Secrets == nil {
			continue
		}
		position := keptPosition(index, intoPrimary, intoSecondary, keptAt, target)
		carried[position] = append(carried[position], item.Row.Secrets)
	}
	for position, records := range carried {
		kept[position].Secrets = secrets.WithDuplicates(kept[position].Secrets, records)
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
