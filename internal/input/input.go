// Package input opens what the analyst points the analyzer at and turns it
// into attributed artifacts. Three shapes are accepted:
//
//   - a collector archive (a tar, plain, gzip or bzip2 compressed, or a zip),
//     extracted to a new temporary directory first;
//   - an extracted collection: a directory holding manifest.jsonl, fs/ and,
//     usually, collection.json;
//   - a loose tree: any other directory, such as a copied home, a mounted
//     image or one agent directory copied on its own.
//
// With a manifest, attribution (host, user, home, agent) comes from its rows.
// Without one, homes are found by matching the catalog against the tree, the
// user is inferred from the path, and the host is left empty unless the
// caller names one. A loose tree's directories are read in name order, not
// the order the file system lists them in, so one tree gives one result on
// every file system.
//
// The input's own path is kept as given; the paths found in it are joined
// to its cleaned form (no doubled separator, no "." component, no trailing
// separator, ".." kept), so one input gives one set of paths however its
// path was spelt. Nothing here follows a
// symbolic link inside the input, and extraction never writes outside its
// directory: no link, hard link or device member is created, a member's name
// is confined to the directory, and every file is created new.
package input

import (
	"errors"
	"io/fs"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// The kinds of input a Collection reports.
const (
	KindArchive   = "archive"
	KindCollected = "collected"
	KindLoose     = "loose"
)

// ProjectAgent is the agent of a project file no single agent owns
// (AGENTS.md), and of every project-catalog file in collections made before
// collector 1.12.0. Such a file is offered to every parser.
const ProjectAgent = "project"

// sharedAgent is the catalog agent of files many tools read; a directory
// holding only such files is not a home.
const sharedAgent = "shared"

// Options are the choices of one Open call.
type Options struct {
	// Matcher is the compiled catalog the input is matched against.
	Matcher *catalog.Matcher
	// WorkDir is the directory an archive is extracted under, created when
	// missing; empty means the system's temporary directory.
	WorkDir string
	// Host, when set, is the host every home is attributed to.
	Host string
	// ReadDir lists a directory of a loose tree; nil means osfile.ReadDir.
	// Whatever order it gives, the entries are taken in name order, so a
	// test can deliver them in another order to show the result does not
	// depend on it.
	ReadDir func(path string) ([]fs.DirEntry, error)
}

// Home is a base the artifacts are relative to: a user's home, a volume or
// a project directory.
type Home struct {
	// DiskPath is where the home is on the analyst's disk.
	DiskPath string
	// Original is the home's path on the source host, best effort.
	Original string
	User     string
	Host     string
	// Inferred is set when User and Original came from the path, not a
	// manifest.
	Inferred bool
	// Project is set when the base is a project directory the collector
	// collected from, not a user's home.
	Project bool
	// StagedPaths maps a home-relative path (slashes) that the collector
	// archived away from DiskPath/rel, at a symlinked directory's physical
	// location, to where the collection holds it; empty for loose input.
	StagedPaths map[string]string
}

// PathOf returns where the collection holds rel, a path relative to the
// home with slashes and no leading slash: the mapped location of rel or of
// its longest mapped ancestor with the rest of rel below it, else
// DiskPath/rel.
func (h *Home) PathOf(rel string) string {
	if len(h.StagedPaths) == 0 {
		return joinPath(h.DiskPath, rel)
	}
	parts := splitSlash(rel)
	for depth := len(parts); depth > 0; depth-- {
		if mapped, ok := h.StagedPaths[joinSlash(parts[:depth])]; ok {
			return joinPath(mapped, parts[depth:]...)
		}
	}
	return joinPath(h.DiskPath, rel)
}

// Artifact is one attributed file of the input.
type Artifact struct {
	DiskPath string
	// Original is the file's path on the source host.
	Original string
	// Rel is the path relative to the home, with slashes.
	Rel   string
	Agent string
	Home  *Home
	// Manifest is the file's manifest row, an object, or null for loose
	// input.
	Manifest jsonvalue.Value
}

// Mtime returns the manifest row's mtime, or null when there is none.
func (a *Artifact) Mtime() jsonvalue.Value { return a.Manifest.Get("mtime") }

// Size returns the manifest row's size as parseSize reads it; a row whose
// size is refused never becomes an artifact. Loose input gives 0.
func (a *Artifact) Size() int64 {
	size, _ := parseSize(a.Manifest.Get("size"))
	return size
}

// Collection is an opened input.
type Collection struct {
	// Source is the input path as given.
	Source string
	// Kind is KindArchive, KindCollected or KindLoose.
	Kind string
	// Root is the directory that holds fs/ (an archive or collection) or
	// the tree itself (loose input).
	Root string
	Host string
	// Manifest holds the manifest rows that parsed, in file order; nil for
	// loose input.
	Manifest []jsonvalue.Value
	// Summary is collection.json, an object, or null when it is absent or
	// unreadable.
	Summary   jsonvalue.Value
	Homes     []*Home
	Artifacts []*Artifact
	Notes     []string
	// Problems holds "<path>: <reason>" for each directory or file inside
	// the input that could not be read while it was opened.
	Problems []string
	// extracted is the directory an archive was extracted to, until
	// Cleanup removes it.
	extracted string
	// readDir lists a directory of a loose tree before listDirectory sorts
	// it.
	readDir func(path string) ([]fs.DirEntry, error)
}

// ExtractedDir returns the directory the archive was extracted to, or ""
// for a directory input or after Cleanup.
func (c *Collection) ExtractedDir() string { return c.extracted }

// Cleanup removes the directory an archive was extracted to, if any.
func (c *Collection) Cleanup() error {
	if c.extracted == "" {
		return nil
	}
	directory := c.extracted
	c.extracted = ""
	if info, err := osfile.Lstat(directory); err != nil || !info.IsDir() {
		return nil
	}
	return osfile.RemoveAll(directory)
}

// ArtifactsFor returns the artifacts attributed to agent, in order.
func (c *Collection) ArtifactsFor(agent string) []*Artifact {
	var found []*Artifact
	for _, artifact := range c.Artifacts {
		if artifact.Agent == agent {
			found = append(found, artifact)
		}
	}
	return found
}

// Agents returns the agents of the artifacts in the order first met.
func (c *Collection) Agents() []string {
	var agents []string
	seen := map[string]bool{}
	for _, artifact := range c.Artifacts {
		if !seen[artifact.Agent] {
			seen[artifact.Agent] = true
			agents = append(agents, artifact.Agent)
		}
	}
	return agents
}

// Error is an input that cannot be opened for a reason of its own rather
// than an operating-system error: it is neither a file nor a directory, or
// not an archive that can be extracted.
type Error struct {
	Message string
}

// Error returns the message.
func (e *Error) Error() string { return e.Message }

// Open opens path as an archive, an extracted collection or a loose tree.
// Unreadable directories and files inside a loose tree are not errors; they
// are recorded in Problems. An error is returned when the input itself, an
// archive member or the manifest cannot be read (an operating-system error
// naming the path), or as an *Error. On error nothing extracted is left.
func Open(path string, options Options) (*Collection, error) {
	if options.Matcher == nil {
		return nil, errors.New("input: Open needs a catalog matcher")
	}
	source := cleanPath(path)
	info, err := osfile.Stat(source)
	if err != nil {
		return nil, err
	}
	var collection *Collection
	switch {
	case info.IsDir():
		collection, err = openDirectory(source)
	case info.Mode().IsRegular():
		collection, err = openArchive(source, options.WorkDir)
	default:
		err = &Error{Message: path + " is neither a regular file nor a directory"}
	}
	if err != nil {
		return nil, err
	}
	collection.Source = path
	if options.Host != "" {
		collection.Host = options.Host
	}
	collection.readDir = options.ReadDir
	if collection.readDir == nil {
		collection.readDir = osfile.ReadDir
	}
	if collection.Kind == KindLoose {
		err = collection.discoverLoose(options.Matcher)
	} else {
		err = collection.loadCollected(options.Matcher)
	}
	if err != nil {
		_ = collection.Cleanup()
		return nil, err
	}
	return collection, nil
}

// openDirectory returns the collection of a directory input, after a
// listing shows it readable: an unlistable input is an error, not an empty
// tree.
func openDirectory(source string) (*Collection, error) {
	if _, err := osfile.ReadDir(source); err != nil {
		return nil, err
	}
	kind := KindLoose
	if isCollectionLayout(source) {
		kind = KindCollected
	}
	return &Collection{Source: source, Kind: kind, Root: source}, nil
}

// isCollectionLayout reports whether directory holds a regular
// manifest.jsonl and a directory fs, neither a link.
func isCollectionLayout(directory string) bool {
	return lstatMode(joinPath(directory, "manifest.jsonl")).IsRegular() &&
		lstatMode(joinPath(directory, "fs")).IsDir()
}

// lstatMode returns the type and mode bits of path itself, or an irregular
// mode when it cannot be described, which is neither a file nor a
// directory.
func lstatMode(path string) fs.FileMode {
	info, err := osfile.Lstat(path)
	if err != nil {
		return fs.ModeIrregular
	}
	return info.Mode()
}
