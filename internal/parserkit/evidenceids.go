package parserkit

import "github.com/seanthegeek/doubleagent/internal/textlines"

// EvidenceIDs is the set of ids a store of the evidence holds (an index's
// entries, a database's keys), which finds the store's spelling of an id
// that a file or directory name gives, since a disk that ignores case may
// spell the name otherwise. A parser that looks a disk name up among such
// ids asks Spelling and puts the answer in its rows. The zero value is an
// empty set.
type EvidenceIDs struct {
	held map[string]bool
	// folded maps an id's ASCII lower case to the id; clash marks a lower
	// case that two ids share.
	folded map[string]string
	clash  map[string]bool
}

// Add puts an id the evidence holds into the set.
func (ids *EvidenceIDs) Add(held string) {
	if ids.held == nil {
		ids.held, ids.folded, ids.clash = map[string]bool{}, map[string]string{}, map[string]bool{}
	}
	ids.held[held] = true
	lower := textlines.LowerASCII(held)
	if earlier, ok := ids.folded[lower]; ok && earlier != held {
		ids.clash[lower] = true
	}
	ids.folded[lower] = held
}

// Spelling returns the name itself when the set holds it, else the one id
// equal to it ignoring ASCII case, else "" and false, two such ids naming
// none since which one the name means cannot be told.
func (ids *EvidenceIDs) Spelling(name string) (string, bool) {
	if ids.held[name] {
		return name, true
	}
	lower := textlines.LowerASCII(name)
	found, ok := ids.folded[lower]
	if !ok || ids.clash[lower] {
		return "", false
	}
	return found, true
}
