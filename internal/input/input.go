package input

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/secrets"
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
	// Whatever order it gives, the entries are taken in name order.
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
	// Project is set when the base is a project directory the collection
	// was made from, not a user's home.
	Project bool
	// StagedPaths maps a home-relative path (slashes) that the collection
	// archived away from DiskPath/rel, at a symlinked directory's physical
	// location, to where the collection holds it; empty for loose input.
	StagedPaths map[string]string
	// Secrets records the secrets the parser kit identifies in the home's
	// files, for the output to replace; every home of a run shares the
	// run's recorder, and nil (a run that shows secrets) records nothing.
	Secrets *secrets.Recorder
	// Repeats gathers the member names repeated within a JSON object that
	// the running parse reads outside the parser kit's readers; nil between
	// parses (parserkit.Within).
	Repeats *jsonvalue.Repeats
	// ownDirectory is set when DiskPath is a directory the input's walk
	// found, or the input itself, so a lookup below it need not look at it.
	ownDirectory bool
	// tree, when set, is the name of the agent directory given as the
	// input, whose parent stands as the home: every path PathOf gives
	// starts with it, since the rest of the parent is not the input.
	tree string
	// names gives DiskPath the name ListedPath prints it by.
	names *inputNames
}

// ListedPath returns the path the commands list the home under: its path
// on the source host, else where it is on disk, named as the analyst named
// the input (Collection.ShownPath).
func (h *Home) ListedPath() string {
	if h.Original != "" {
		return h.Original
	}
	return h.names.path(h.DiskPath)
}

// PathOf returns where the collection holds rel, a path relative to the
// home with slashes (empty and "." elements are dropped), and true: the
// mapped location of rel or of its longest mapped ancestor with the rest of
// rel below it, else DiskPath/rel. It is false when below or plainParts
// refuses rel, when rel is deeper than maxPathComponents, or when the input
// is one agent directory and rel does not start with its name, so every
// path it gives lies inside the input; the last element is left for the
// reader.
func (h *Home) PathOf(rel string) (string, bool) {
	parts := slices.DeleteFunc(strings.Split(rel, "/"), func(part string) bool { return part == "" || part == "." })
	if len(parts) > maxPathComponents || !plainParts(parts) ||
		h.tree != "" && (len(parts) == 0 || parts[0] != h.tree) {
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
	// Directory is the input directory's entry as Open reached it, a link
	// followed as the analyst chose it, so a caller can tell another path
	// that names the same directory (os.SameFile); nil for an archive.
	Directory fs.FileInfo
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
	// found without searching the list.
	problemLines map[string]bool
	// extractedFiles holds the slash key of each file the extraction of an
	// archive made, under the member's own name; nil for a directory
	// input. A manifest row names a file only by such a key, so a member
	// skipped as a collision is not read under the other member's content.
	extractedFiles map[string]bool
	// contested maps the key of each extracted file where members of
	// different names met to the kept member's name (slashOnlyName); only a
	// row whose archive path has that name reaches the file (notKeptAt).
	contested map[string]string
	// fromZip is set for a collection extracted from a zip (archiveParts).
	fromZip bool
	// extracted is the directory an archive was extracted to, until
	// Cleanup removes it.
	extracted string
	// readDir lists a directory of a loose tree (Options.ReadDir).
	readDir func(path string) ([]fs.DirEntry, error)
	// names gives the paths below the input, and below the extraction
	// directory, the names the commands print them by.
	names *inputNames
}

// ShownPath returns path, a path below the input or the extraction
// directory, as the commands print it: under the input as the analyst
// named it, cleaned, and under the extraction directory as --work-dir
// named it. Every other path is returned as it is.
func (c *Collection) ShownPath(path string) string { return c.names.path(path) }

// describe returns the problem line for err at path, the path named as
// ShownPath gives it.
func (c *Collection) describe(path string, err error) string {
	return describeError(c.names.path(path), err)
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
	return osfile.RemoveAll(directory)
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

// Error is an input that cannot be opened for a reason of its own: it is
// neither a file nor a directory, or not an archive that can be extracted.
type Error struct {
	Message string
}

// Error returns the message.
func (e *Error) Error() string { return e.Message }

// Open opens path as an archive, an extracted collection or a loose tree.
// Unreadable directories and files inside a loose tree are recorded in
// Problems, not returned. An error is returned when the input itself, an
// archive member or the manifest cannot be read, as an *Error, or as ctx's
// error once it is cancelled (looked at for each member, row and
// directory). On error nothing extracted is left.
func Open(ctx context.Context, path string, options Options) (*Collection, error) {
	if options.Matcher == nil {
		return nil, errors.New("input: Open needs a catalog matcher")
	}
	source, err := absolute(path)
	if err != nil {
		return nil, err
	}
	names := &inputNames{}
	names.add(source, filepath.Clean(path))
	info, err := osfile.Stat(source)
	if err != nil {
		return nil, names.error(err)
	}
	var collection *Collection
	switch {
	case info.IsDir():
		collection, err = openDirectory(source)
	case info.Mode().IsRegular():
		collection, err = openArchive(ctx, source, options.WorkDir, names)
	default:
		err = &Error{Message: path + " is neither a regular file nor a directory"}
	}
	if err != nil {
		return nil, names.error(err)
	}
	collection.Source = path
	collection.names = names
	if info.IsDir() {
		collection.Directory = info
	}
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
		return nil, names.error(err)
	}
	return collection, nil
}

// Absolute places a path the analyst typed by the rule the input is read
// by (absolute, per platform). The outside-the-input check calls it on the
// output too, so on Windows the output is placed where the commands write
// it.
func Absolute(path string) (string, error) { return absolute(path) }

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
