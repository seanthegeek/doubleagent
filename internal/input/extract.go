package input

import (
	"archive/zip"
	"compress/bzip2"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// The signatures a zip archive starts with: a local file header, or the
// end record of an archive with no member (the format's specification,
// APPNOTE.TXT, sections 4.3.7 and 4.3.16).
const (
	zipLocalSignature = "PK\x03\x04"
	zipEmptySignature = "PK\x05\x06"
)

// extractArchive extracts the zip or tar archive at path, which messages
// name as shown, into destination, a new empty directory, and returns a
// problem line for each member it skipped and the entries it made. The kind
// is told from the first bytes, never the end: a file that starts as a zip
// and that archive/zip reads is a zip, any other is tried as a tar. A file
// that is neither, or a damaged archive, is an *Error.
func extractArchive(ctx context.Context, path, shown, destination string) (*extraction, error) {
	file, err := osfile.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, &fs.PathError{Op: "stat", Path: shown, Err: err}
	}
	out := newExtraction(destination, shown)
	magic := make([]byte, 4)
	n, _ := file.ReadAt(magic, 0)
	if start := string(magic[:n]); start == zipLocalSignature || start == zipEmptySignature {
		archive, err := zip.NewReader(file, info.Size())
		if err == nil {
			archive.RegisterDecompressor(zipBzip2Method, func(r io.Reader) io.ReadCloser {
				return io.NopCloser(bzip2.NewReader(r))
			})
			out.zip = true
			return out, extractZip(ctx, archive.File, shown, out)
		}
		if !errors.Is(err, zip.ErrFormat) {
			return nil, cannotExtract(shown, zipReason(err))
		}
	}
	return out, extractTar(ctx, file, shown, out)
}

// notAnArchive returns the error for a file that is neither archive.
func notAnArchive(path string) error {
	return &Error{Message: path + " is neither a tar nor a zip archive"}
}

// cannotExtract returns the error for an archive that is damaged or uses a
// feature the extractor refuses.
func cannotExtract(path, reason string) error {
	return &Error{Message: path + ": cannot extract: " + reason}
}

// extraction writes the members of one archive below its root. Every
// directory it makes is checked with lstat before a member goes into it, and
// every file is created new (O_EXCL), so no member is written through a
// link. The root starts empty, so an entry found under a name the extraction
// never made is one the file system holds for two names (it folds case or
// Unicode normalisation), and the later member is skipped.
type extraction struct {
	root string
	// archive is the archive's path, which a problem names.
	archive string
	// made maps the slash key of each entry made to whether it is a
	// directory.
	made map[string]bool
	// children holds the names made in each directory, by its key ("" for
	// the root).
	children map[string][]string
	// identities maps, for each directory where two members met, the
	// identity of each child made in it to its name, built at the first one.
	identities map[string]map[[2]uint64]string
	// problems holds a line for each member skipped.
	problems []string
	// zip is set for a zip archive, whose names extractZip splits at
	// backslashes.
	zip bool
	// ownNames maps the key of each file made to the name, as the archive
	// gives it, of the member that made it, only where that name read with
	// slashes alone (slashOnlyName) is not the key.
	ownNames map[string]string
	// contested maps the key of each file where members of different names
	// met (createNew) to the name, read with slashes alone, of the member
	// kept there.
	contested map[string]string
}

// newExtraction returns an extraction of archive into root.
func newExtraction(root, archive string) *extraction {
	return &extraction{root: root, archive: archive, made: map[string]bool{},
		children: map[string][]string{}, identities: map[string]map[[2]uint64]string{},
		ownNames: map[string]string{}, contested: map[string]string{}}
}

// skip records that the member called name was not extracted, and why.
func (e *extraction) skip(name, reason string) {
	e.problems = append(e.problems, e.archive+": member "+name+" not extracted: "+reason)
}

// collisionError is a member whose place holds an entry made for another
// member: kept is that member's key, or "" when no entry made has the
// place's identity; sameName is set when that entry has the member's own
// name but the other kind, a file where a directory is due or the reverse;
// otherName is set when that entry is a file whose member has another name
// (slashOnlyName); kept is then that member's name.
type collisionError struct {
	kept      string
	sameName  bool
	otherName bool
}

// Error returns the reason the member is skipped.
func (e *collisionError) Error() string {
	switch {
	case e.sameName:
		return "member " + e.kept + " holds its place as another kind of entry"
	case e.otherName:
		return "its name and that of member " + e.kept + " name one place when a backslash separates " +
			"folders; " + e.kept + " is kept"
	case e.kept == "":
		return "this file system holds its name as an entry already there"
	}
	return "this file system holds its name and that of member " + e.kept + " as one entry; " +
		e.kept + " is kept"
}

// skipped reports whether err, from writing the member called name, means
// the member is skipped rather than the archive refused (a name the system
// cannot hold, or a place another member holds), and records the skip.
func (e *extraction) skipped(name string, err error) bool {
	var collision *collisionError
	switch {
	case errors.As(err, &collision):
		e.skip(name, collision.Error())
	case nameTooLong(err):
		e.skip(name, reasonTooLong)
	case nameRefused(err):
		e.skip(name, reasonNotAName)
	default:
		return false
	}
	return true
}

// The reasons a member is skipped for its name.
const (
	reasonLeavesDirectory = "its name leads out of the extraction directory"
	reasonNoFile          = "its name names no file"
	reasonNotAName        = "its name holds a part this system cannot use as a file name"
	reasonTooLong         = "its name, or a part of it, is longer than this system takes"
)

// memberParts returns the components a tar or zip member name stands for
// below the extraction directory, with this platform's separators read as
// slashes and leading slashes and empty and "." components dropped. A name
// that then starts with a drive letter or holds a ".." component
// (filepath.IsLocal's rule), or a component checkElement refuses, gives the
// reason it is skipped; a name the system cannot store fails at the write.
func memberParts(name string) ([]string, string) {
	name = strings.TrimLeft(memberSeparators(name), "/")
	if windowsDrive(name) != "" || slices.Contains(strings.Split(name, "/"), "..") {
		return nil, reasonLeavesDirectory
	}
	parts := memberElements(name)
	if !plainParts(parts) {
		return nil, reasonNotAName
	}
	return parts, ""
}

// memberElements returns the elements of a member name that holds no ".."
// element: path.Clean drops the empty and "." ones.
func memberElements(name string) []string {
	cleaned := path.Clean(name)
	if cleaned == "." {
		return nil
	}
	return strings.Split(cleaned, "/")
}

// slashOnlyName returns a member name, or a manifest row's archive path,
// read with slashes alone as separators: leading slashes and empty and "."
// parts dropped, the rest joined by single slashes. Two names that differ
// here are two members, even where a backslash puts them in one place.
func slashOnlyName(name string) string {
	return strings.Join(memberElements(strings.TrimLeft(name, "/")), "/")
}

// place is where a member's components go: the slash key and the native
// path of each prefix, as slices of one string each, so a member's work
// grows with its name's length.
type place struct {
	parts             []string
	key, path         string
	keyEnds, pathEnds []int
}

// placeOf returns the place of the member components parts.
func (e *extraction) placeOf(parts []string) place {
	key, keyEnds := joinedPrefixes(parts)
	var path strings.Builder
	path.WriteString(e.root)
	pathEnds := make([]int, len(parts)+1)
	pathEnds[0] = len(e.root)
	for i, part := range parts {
		path.WriteByte(filepath.Separator)
		path.WriteString(part)
		pathEnds[i+1] = path.Len()
	}
	return place{parts: parts, key: key, path: path.String(), keyEnds: keyEnds, pathEnds: pathEnds}
}

// prefix returns the key and path of the first depth components.
func (p place) prefix(depth int) (string, string) {
	return p.key[:p.keyEnds[depth]], p.path[:p.pathEnds[depth]]
}

// parentKey returns the key of the directory holding the entry at depth.
func (p place) parentKey(depth int) string {
	return p.key[:p.keyEnds[depth-1]]
}

// makeDirectories makes each directory of the first depth components of
// the place that does not exist yet. A component that is a file this
// archive wrote, or an entry it never made under that name, is a
// *collisionError; one that cannot be looked at is the system's error.
func (e *extraction) makeDirectories(member place, depth int) error {
	for level := 1; level <= depth; level++ {
		key, path := member.prefix(level)
		if directory, ok := e.made[key]; ok {
			if !directory {
				return &collisionError{kept: key, sameName: true}
			}
			continue
		}
		_, err := osfile.Lstat(path)
		switch {
		case err == nil:
			return &collisionError{kept: e.keptAt(member.parentKey(level), path)}
		case !errors.Is(err, fs.ErrNotExist):
			return err
		}
		if err := osfile.MkdirAll(path, 0o777); err != nil {
			return err
		}
		info, err := osfile.Lstat(path)
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return &fs.PathError{Op: "mkdir", Path: path, Err: errNotDirectory}
		}
		e.record(member, level, true)
	}
	return nil
}

// record notes the entry made for the member's first depth components.
func (e *extraction) record(member place, depth int, directory bool) {
	key, path := member.prefix(depth)
	parentKey, name := member.parentKey(depth), member.parts[depth-1]
	e.made[key] = directory
	e.children[parentKey] = append(e.children[parentKey], name)
	if index := e.identities[parentKey]; index != nil {
		if identity, ok := identityOf(path); ok {
			index[identity] = name
		}
	}
}

// keptAt returns the key of the entry made in the directory parentKey
// that the file system holds at path, or "" when none is.
func (e *extraction) keptAt(parentKey, path string) string {
	index := e.identities[parentKey]
	directory := filepath.Dir(path)
	if index == nil {
		index = map[[2]uint64]string{}
		for _, name := range e.children[parentKey] {
			if identity, ok := identityOf(directory + string(filepath.Separator) + name); ok {
				index[identity] = name
			}
		}
		e.identities[parentKey] = index
	}
	identity, ok := identityOf(path)
	name, found := index[identity]
	if !ok || !found {
		return ""
	}
	if parentKey == "" {
		return name
	}
	return parentKey + "/" + name
}

// identityOf returns the device and file number of the entry at path, not
// following a link, and false when the system gives none.
func identityOf(path string) ([2]uint64, bool) {
	meta, err := fsmeta.Lstat(path)
	if err != nil || meta.Ino == 0 {
		return [2]uint64{}, false
	}
	return [2]uint64{meta.Dev, meta.Ino}, true
}

// writeFile creates the file parts names for the member called name, as
// the archive gives it, its directories first, and copies contents into
// it, with modified, when not zero, as its access and modification time.
// Which member keeps a place two names reach is createNew's rule; a
// directory there, or an entry made for another name, is a
// *collisionError.
func (e *extraction) writeFile(parts []string, name string, perm fs.FileMode, contents io.Reader,
	modified time.Time) error {
	if len(parts) == 0 {
		return &fs.PathError{Op: "open", Path: e.root, Err: errIsDirectory}
	}
	member := e.placeOf(parts)
	if err := e.makeDirectories(member, len(parts)-1); err != nil {
		return err
	}
	out, err := e.createNew(member, len(parts), name, perm)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, contents)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return &fs.PathError{Op: "close", Path: member.path, Err: closeErr}
	}
	if !modified.IsZero() {
		return osfile.Chtimes(member.path, modified, modified)
	}
	return nil
}

// createNew creates the file at the place of the member called name, as
// the archive gives it, exclusively. The member that made the file holds
// the place: a later member of its name read with slashes alone
// (slashOnlyName) replaces the file, and one of any other name is a
// *collisionError, its place recorded as contested. Members whose names
// hold no backslash are extracted first (extractZip, extractTar), so such
// a member holds its place against one that reaches it only through a
// backslash.
func (e *extraction) createNew(member place, depth int, name string, perm fs.FileMode) (*os.File, error) {
	key, _ := member.prefix(depth)
	directory, made := e.made[key]
	slashName := slashOnlyName(name)
	switch {
	case made && directory:
		return nil, &collisionError{kept: key, sameName: true}
	case made && slashName != e.ownerOf(key):
		e.contested[key] = e.ownerOf(key)
		return nil, &collisionError{kept: e.ownerOf(key), otherName: true}
	}
	out, err := e.openOver(member, depth, made, perm)
	if err != nil {
		return nil, err
	}
	if slashName != key {
		e.ownNames[key] = name
	}
	return out, nil
}

// openOver opens the file at the member's place exclusively, first
// removing the regular file an earlier member wrote there when made says
// one did, and records a place it makes; an entry there that no member
// made is a *collisionError.
func (e *extraction) openOver(member place, depth int, made bool, perm fs.FileMode) (*os.File, error) {
	_, path := member.prefix(depth)
	flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL
	out, err := osfile.OpenFile(path, flags, perm)
	if err == nil {
		if !made {
			e.record(member, depth, false)
		}
		return out, nil
	}
	info, statErr := osfile.Lstat(path)
	if statErr != nil {
		return nil, err
	}
	if !made || !info.Mode().IsRegular() {
		return nil, &collisionError{kept: e.keptAt(member.parentKey(depth), path)}
	}
	if err := osfile.Remove(path); err != nil {
		return nil, err
	}
	return osfile.OpenFile(path, flags, perm)
}

// ownerOf returns the name, read with slashes alone, of the member that
// made the file at key.
func (e *extraction) ownerOf(key string) string {
	if name, ok := e.ownNames[key]; ok {
		return slashOnlyName(name)
	}
	return key
}

// setTime gives the directory parts the access and modification time
// modified.
func (e *extraction) setTime(parts []string, modified time.Time) error {
	return osfile.Chtimes(e.placeOf(parts).path, modified, modified)
}
