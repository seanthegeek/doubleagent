//go:build parity

package replay

// repeatDifferences returns the differences numbered 250 onward: evidence
// the binary writes once where the recording wrote it at every row that
// named it.
func repeatDifferences() []Difference {
	return []Difference{socketTokenDifference()}
}

// repeatLabels returns the entries the Added rules of repeatDifferences
// report a taken row under, which no per-entry tally of another difference
// counts.
func repeatLabels() map[string]bool {
	return map[string]bool{socketTokenLabel: true}
}
