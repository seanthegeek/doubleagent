package syspath

// SameEntry reports whether two paths that differ in their bytes but not in
// their Fold keys name one file system entry (follow.SameEntry asks the file
// system): a directory Windows marks case-sensitive holds Alice and alice as
// two entries, an ordinary one only one. A nil SameEntry proves nothing.
type SameEntry func(first, second string) bool

// Entries maps paths to values, one value per file system entry. Two paths
// with equal bytes are one entry. Two whose Fold keys are equal and whose
// bytes differ are one entry only when same proves it; otherwise each keeps
// its own value, since merging two entries would lose one of them while
// keeping two spellings of one apart costs at most a second copy. On Unix,
// where Fold changes nothing, it is a map keyed by the bytes and never calls
// same. Same-key spellings after the first are rare and kept in more.
type Entries[V any] struct {
	fold  func(string) string
	same  SameEntry
	first map[string]spelled[V]
	more  map[string][]spelled[V]
}

// spelled is one stored spelling of a key and its value.
type spelled[V any] struct {
	path  string
	value V
}

// NewEntries returns an empty Entries that asks same about two spellings
// whose keys are equal.
func NewEntries[V any](same SameEntry) *Entries[V] {
	return newEntries[V](Fold, same)
}

// newEntries is NewEntries with the key function given, so every platform
// tests the Windows fold.
func newEntries[V any](fold func(string) string, same SameEntry) *Entries[V] {
	return &Entries[V]{fold: fold, same: same, first: map[string]spelled[V]{},
		more: map[string][]spelled[V]{}}
}

// Find returns the value stored for the entry path names and the spelling
// it was stored under: the spelling with path's own bytes, else the first
// one same proves to be the same entry. found is false when there is none,
// and on a nil Entries, which holds nothing.
func (entries *Entries[V]) Find(path string) (value V, stored string, found bool) {
	if entries == nil {
		return value, "", false
	}
	key := entries.fold(path)
	first, exists := entries.first[key]
	if !exists {
		return value, "", false
	}
	if first.path == path {
		return first.value, first.path, true
	}
	candidates := append([]spelled[V]{first}, entries.more[key]...)
	for _, candidate := range candidates[1:] {
		if candidate.path == path {
			return candidate.value, candidate.path, true
		}
	}
	for _, candidate := range candidates {
		if entries.same != nil && entries.same(candidate.path, path) {
			return candidate.value, candidate.path, true
		}
	}
	return value, "", false
}

// Has reports whether Find finds path.
func (entries *Entries[V]) Has(path string) bool {
	_, _, found := entries.Find(path)
	return found
}

// Put stores value for the entry path names, replacing the value of the
// spelling Find returns, or under path's own spelling when Find finds none.
func (entries *Entries[V]) Put(path string, value V) {
	key := entries.fold(path)
	_, stored, found := entries.Find(path)
	first, hasKey := entries.first[key]
	switch {
	case !hasKey:
		entries.first[key] = spelled[V]{path, value}
	case !found:
		entries.more[key] = append(entries.more[key], spelled[V]{path, value})
	case first.path == stored:
		entries.first[key] = spelled[V]{stored, value}
	default:
		spellings := entries.more[key]
		for i := range spellings {
			if spellings[i].path == stored {
				spellings[i].value = value
			}
		}
	}
}

// Add stores value for path and reports true when Find finds no entry for
// it; otherwise it changes nothing and reports false.
func (entries *Entries[V]) Add(path string, value V) bool {
	if entries.Has(path) {
		return false
	}
	entries.Put(path, value)
	return true
}

// CutUnder returns path less dir and its separator, and true, when path lies
// under dir: when it starts with dir's bytes and a slash, or, where the two
// spell dir differently but with one Fold key, when same proves path's
// spelling of it the entry dir names. A wrong prefix match would give a path
// to a home or base it does not lie under.
func CutUnder(path, dir string, same SameEntry) (rest string, found bool) {
	return cutUnder(Fold, path, dir, same)
}

// cutUnder is CutUnder with the key function given.
func cutUnder(fold func(string) string, path, dir string, same SameEntry,
) (rest string, found bool) {
	if len(path) <= len(dir) || path[len(dir)] != '/' {
		return path, false
	}
	prefix := path[:len(dir)]
	if prefix == dir || (fold(prefix) == fold(dir) && same != nil && same(prefix, dir)) {
		return path[len(dir)+1:], true
	}
	return path, false
}

// SameAs reports whether first and second name one entry: equal bytes, or
// equal Fold keys with same proving it.
func SameAs(first, second string, same SameEntry) bool {
	return first == second || (Fold(first) == Fold(second) && same != nil && same(first, second))
}
