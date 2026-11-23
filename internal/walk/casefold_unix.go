//go:build unix

package walk

import (
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/glob"
)

// caseFolding answers, for the exclusion and secret tests, whether the
// directory holding each component of a path folds letter case, so a byte
// is compared ignoring ASCII case only where the disk itself ignores it. It
// asks the image root's check before it looks at a name, reads through
// lstat and listing (fsmeta.Lstat and glob.Names unless a test gives
// others) and keeps each directory's answer for the run.
type caseFolding struct {
	check   glob.Check
	lstat   func(path string) (fsmeta.Meta, error)
	listing func(directory string) []string
	folds   map[string]bool
}

// newCaseFolding returns an empty cache that reads under check and opener.
func newCaseFolding(check glob.Check, opener *fsmeta.Opener) *caseFolding {
	return &caseFolding{check: check, lstat: fsmeta.Lstat,
		listing: func(directory string) []string { return glob.Names(directory, check, opener) },
		folds:   map[string]bool{}}
}

// matchUnder reports whether path[from:] matches one of patterns, each byte
// from foldFrom on compared ignoring ASCII case when its component's
// directory folds. A pattern that cannot match under any folding, or that
// matches byte for byte, is settled without looking at the disk; only a
// path matching some other way has its directories probed, once per call.
func (folding *caseFolding) matchUnder(patterns []pathPattern, path string, from, foldFrom int,
) bool {
	subject := path[from:]
	var mask []bool
	for _, pattern := range patterns {
		if !pattern.folding.MatchEitherCase(subject) {
			continue
		}
		if pattern.folding.MatchBytes(subject) {
			return true
		}
		if mask == nil {
			mask = folding.componentMask(path, foldFrom)
		}
		if pattern.folding.Match(subject, func(index int) bool { return mask[from+index] }) {
			return true
		}
	}
	return false
}

// componentMask returns, for each byte of path, whether it lies in a
// component from foldFrom on whose directory folds case; a component with
// no ASCII letter is never asked about, since folding cannot change it.
func (folding *caseFolding) componentMask(path string, foldFrom int) []bool {
	mask := make([]bool, len(path))
	for start := foldFrom; start < len(path); {
		end := start
		for end < len(path) && path[end] != '/' {
			end++
		}
		name := path[start:end]
		if hasASCIILetter(name) && folding.directoryFolds(parentOf(path, start), name) {
			for i := start; i < end; i++ {
				mask[i] = true
			}
		}
		start = end + 1
	}
	return mask
}

// directoryFolds reports whether directory folds case, probed through its
// entry name the first time the directory is asked about.
func (folding *caseFolding) directoryFolds(directory, name string) bool {
	if answer, known := folding.folds[directory]; known {
		return answer
	}
	answer := folding.probe(directory, name)
	folding.folds[directory] = answer
	return answer
}

// probe reports whether directory folds case: its entry name and the
// spelling with each ASCII letter's case swapped are one entry by device and
// inode. A file with more than one link could be two names of one inode, so
// it also needs the listing to hold one name equal to it under ASCII case.
// A refused check, an error, a missing name or no identity answers false;
// only the last component is respelled, and lstat follows nothing.
func (folding *caseFolding) probe(directory, name string) bool {
	spelled, swapped := joinName(directory, name), joinName(directory, swapCase(name))
	if folding.check != nil {
		if _, refused := folding.check(spelled); refused {
			return false
		}
		if _, refused := folding.check(swapped); refused {
			return false
		}
	}
	one, err := folding.lstat(spelled)
	if err != nil || one.Ino == 0 {
		return false
	}
	other, err := folding.lstat(swapped)
	if err != nil || other.Dev != one.Dev || other.Ino != one.Ino {
		return false
	}
	if one.Kind == fsmeta.Dir || one.Nlink == 1 {
		return true
	}
	count := 0
	for _, entry := range folding.listing(directory) {
		if asciiLower(entry) == asciiLower(name) {
			count++
		}
	}
	return count == 1
}

// parentOf returns the directory holding the component of path that starts
// at start: the path before its separator, or / for a component of the root.
func parentOf(path string, start int) string {
	if start <= 1 {
		return "/"
	}
	return path[:start-1]
}

// joinName returns name inside directory.
func joinName(directory, name string) string {
	if directory == "/" {
		return "/" + name
	}
	return directory + "/" + name
}

// swapCase returns name with each ASCII letter's case swapped.
func swapCase(name string) string {
	swapped := []byte(name)
	for i, char := range swapped {
		switch {
		case 'a' <= char && char <= 'z':
			swapped[i] = char - 'a' + 'A'
		case 'A' <= char && char <= 'Z':
			swapped[i] = char - 'A' + 'a'
		}
	}
	return string(swapped)
}

// hasASCIILetter reports whether name holds an ASCII letter.
func hasASCIILetter(name string) bool {
	for i := 0; i < len(name); i++ {
		if lowered := name[i] | 0x20; 'a' <= lowered && lowered <= 'z' {
			return true
		}
	}
	return false
}
