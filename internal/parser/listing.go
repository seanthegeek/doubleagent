package parser

import "io/fs"

// Listing is a plain directory's entries by name, listed once through
// ReadDir, so a parser that looks up many names in one directory makes one
// listing rather than one look per name. A name is found only by the exact
// bytes the directory holds, which is stricter than a look through the file
// system on one that matches names in any case or normalisation: a parser
// uses it where the name comes from the agent's own writing, which spells
// it as the file does. A name the listing holds is looked at once more, as
// a look per name would, so the answer is that look's; a directory that
// could not be listed is answered by a look per name.
type Listing struct {
	directory Directory
	kinds     map[string]fs.FileMode
}

// ListingOf lists directory once.
func ListingOf(directory Directory) Listing {
	listing := Listing{directory: directory}
	entries, err := ReadDir(directory.Path())
	if err != nil {
		return listing
	}
	listing.kinds = make(map[string]fs.FileMode, len(entries))
	for _, entry := range entries {
		listing.kinds[entry.Name()] = entry.Type()
	}
	return listing
}

// held reports whether a look at name is worth making: the listing holds
// it with the kind want reports true of, or the directory was not listed.
func (l Listing) held(name string, want func(fs.FileMode) bool) bool {
	if l.kinds == nil {
		return true
	}
	kind, ok := l.kinds[name]
	return ok && want(kind)
}

// RegularFile returns the path of name in the directory and whether it is
// a regular file, as RegularFile of the directory's Child would say.
func (l Listing) RegularFile(name string) (string, bool) {
	path, ok := l.directory.Child(name)
	if !ok || !l.held(name, func(kind fs.FileMode) bool { return kind.IsRegular() }) {
		return "", false
	}
	return path, RegularFile(path)
}

// Subdirectory returns name in the directory as a Directory and whether it
// is a plain one, as DirectoryAt of the directory's Child would say.
func (l Listing) Subdirectory(name string) (Directory, bool) {
	path, ok := l.directory.Child(name)
	if !ok || !l.held(name, func(kind fs.FileMode) bool { return kind.IsDir() }) {
		return Directory{}, false
	}
	return DirectoryAt(path)
}
