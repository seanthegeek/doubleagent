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
// to its cleaned form, so one input gives one set of paths however its
// path was spelt. A path built from the evidence's text (a manifest field,
// a name a parser asks Home.PathOf or Below for) is made by one function,
// below: each element must be one plain component, and each directory on
// the way down is looked at with lstat, a link (on Windows any reparse
// point) refused. os.Root would also keep a path below a directory, but it
// follows a link that stays inside and bypasses internal/osfile's Windows
// path form. A loose tree's walk enters only real directories. Extraction
// creates no link, hard link or device, checks each component of a
// member's name the same way (checkElement), looks at each directory it
// makes with lstat and creates every file new; a member whose name cannot
// be used is skipped with a problem naming the archive and the member.
package input

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"slices"

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
	// ownDirectory is set when DiskPath is a directory the input's walk
	// found, or the input itself, so a lookup below it need not look at it.
	ownDirectory bool
}

// PathOf returns where the collection holds rel, a path relative to the
// home with slashes (empty and "." elements, which name no further place,
// are dropped), and true: the mapped
// location of rel or of its longest mapped ancestor with the rest of rel
// below it, else DiskPath/rel. It is false when an element of rel is not
// one plain component (such as "..", or a name holding a backslash), when
// rel has more than maxPathComponents elements, or when the way down passes
// through a link, so every path it gives lies below the home or a staged
// location; the last element is left for the reader.
func (h *Home) PathOf(rel string) (string, bool) {
	parts := slices.DeleteFunc(splitSlash(rel), func(part string) bool { return part == "" || part == "." })
	if len(parts) > maxPathComponents || !plainParts(parts) {
		return "", false
	}
	if len(h.StagedPaths) > 0 {
		joined, ends := joinedPrefixes(parts)
		for depth := len(parts); depth > 0; depth-- {
			if mapped, ok := h.StagedPaths[joined[:ends[depth]]]; ok {
				path, err := below(mapped, parts[depth:], true)
				return path, err == nil
			}
		}
	}
	path, err := below(h.DiskPath, parts, !h.ownDirectory)
	return path, err == nil
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

// Size returns the manifest row's size as parseSize reads it, never
// negative: a row whose size is refused or negative never becomes an
// artifact. Loose input gives 0.
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
	// problemLines holds every line of Problems, so a repeated line is
	// found without searching the list, whose length grows with the
	// evidence.
	problemLines map[string]bool
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
// naming the path), or as an *Error, and ctx's error once it is cancelled,
// which is looked at for each archive member, manifest row and directory.
// On error nothing extracted is left.
func Open(ctx context.Context, path string, options Options) (*Collection, error) {
	if options.Matcher == nil {
		return nil, errors.New("input: Open needs a catalog matcher")
	}
	source := filepath.Clean(path)
	info, err := osfile.Stat(source)
	if err != nil {
		return nil, err
	}
	var collection *Collection
	switch {
	case info.IsDir():
		collection, err = openDirectory(source)
	case info.Mode().IsRegular():
		collection, err = openArchive(ctx, source, options.WorkDir)
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
		err = collection.discoverLoose(ctx, options.Matcher)
	} else {
		err = collection.loadCollected(ctx, options.Matcher)
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
