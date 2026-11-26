package glob

// Names returns the entry names of directory in listing order, from the
// listing the expander keeps of it, and whether the whole listing was read.
// A directory check refuses is not listed and gives no names, read whole,
// so nothing is looked up in it either.
func (expander *Expander) Names(directory string) (names []string, complete bool) {
	listed := (&expansion{Expander: expander}).listingOf(directory)
	return listed.names, listed.complete || listed.refused
}
