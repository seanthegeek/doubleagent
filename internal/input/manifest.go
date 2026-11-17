package input

import (
	"bytes"
	"errors"
	"io/fs"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/textlines"
)

// jsonSpace is the whitespace JSON allows between tokens, the only padding
// a manifest line may carry.
const jsonSpace = " \t\n\r"

// homeKey identifies a home by the user and home path its rows name.
type homeKey struct {
	user, home string
}

// archivedRow is one row of a home that names a member or a link: its
// home-relative path, its archive path and whether it is a symlink row.
type archivedRow struct {
	rel, archivePath string
	link             bool
}

// manifestLoad is the state of reading one manifest into a collection.
type manifestLoad struct {
	collection *Collection
	matcher    *catalog.Matcher
	projects   map[string]bool
	imageRoot  string
	homes      map[homeKey]*Home
	order      []homeKey
	// provisional holds the homes whose directory came from the fallback,
	// until a row names it.
	provisional map[homeKey]bool
	archived    map[homeKey][]archivedRow
}

// loadCollected reads collection.json and manifest.jsonl under the root and
// attributes every collected file row to its home.
func (c *Collection) loadCollected(matcher *catalog.Matcher) error {
	c.readSummary()
	if c.Host == "" && !c.Summary.IsEmpty() {
		c.Host = textOf(c.Summary.Get("hostname"))
	}
	data, err := osfile.ReadFile(joinPath(c.Root, "manifest.jsonl"))
	if err != nil {
		return err
	}
	rows, bad := parseManifest(data)
	if bad > 0 {
		c.Notes = append(c.Notes, strconv.Itoa(bad)+" manifest rows did not parse")
	}
	c.Manifest = rows
	load := newManifestLoad(c, matcher)
	for _, row := range rows {
		load.add(row)
	}
	load.finish()
	return nil
}

// parseManifest splits manifest bytes into rows: each LF-ended line decoded
// as UTF-8 with replacement and trimmed of JSON whitespace, blank lines
// skipped. A line that is not a JSON object, or a collected file row whose
// size int() would refuse, is counted as bad instead of kept.
func parseManifest(data []byte) (rows []jsonvalue.Value, bad int) {
	rows = []jsonvalue.Value{}
	for len(data) > 0 {
		line := data
		if end := bytes.IndexByte(data, '\n'); end >= 0 {
			line, data = data[:end], data[end+1:]
		} else {
			data = nil
		}
		text := strings.Trim(textlines.ValidUTF8(line), jsonSpace)
		if text == "" {
			continue
		}
		row, err := jsonvalue.DecodeString(text)
		if err != nil || !row.IsObject() || !sizeReadable(row) {
			bad++
			continue
		}
		rows = append(rows, row)
	}
	return rows, bad
}

// sizeReadable reports whether a row that becomes an artifact has a size
// int() accepts; other rows are not asked.
func sizeReadable(row jsonvalue.Value) bool {
	if !isText(row.Get("status"), "collected") || !isText(row.Get("type"), "file") {
		return true
	}
	_, ok := parseSize(row.Get("size"))
	return ok
}

// readSummary reads collection.json when it is a regular file. A file that
// is not strict UTF-8, not JSON, or a JSON value that is true but not an
// object leaves a note and no summary; a false value (null, [], {}) is no
// summary and no note.
func (c *Collection) readSummary() {
	path := joinPath(c.Root, "collection.json")
	if !lstatMode(path).IsRegular() {
		return
	}
	data, err := osfile.ReadFile(path)
	if err != nil {
		c.Notes = append(c.Notes, "collection.json unreadable: "+errorReason(err))
		return
	}
	summary, err := decodeSummary(data)
	if err != nil {
		c.Notes = append(c.Notes, "collection.json unreadable: "+err.Error())
		return
	}
	if summary.IsObject() {
		c.Summary = summary
	}
}

// decodeSummary decodes collection.json bytes as strict UTF-8 JSON.
func decodeSummary(data []byte) (jsonvalue.Value, error) {
	if !utf8.Valid(data) {
		return jsonvalue.Value{}, errors.New("not valid UTF-8")
	}
	summary, err := jsonvalue.DecodeString(string(data))
	if err != nil {
		return jsonvalue.Value{}, err
	}
	if !summary.IsEmpty() && !summary.IsObject() {
		return jsonvalue.Value{}, errors.New("not a JSON object")
	}
	return summary, nil
}

// newManifestLoad returns the state for collection, with the projects and the image
// root collection.json names.
func newManifestLoad(collection *Collection, matcher *catalog.Matcher) *manifestLoad {
	load := &manifestLoad{
		collection:  collection,
		matcher:     matcher,
		projects:    map[string]bool{},
		imageRoot:   textOf(collection.Summary.Get("root")),
		homes:       map[homeKey]*Home{},
		provisional: map[homeKey]bool{},
		archived:    map[homeKey][]archivedRow{},
	}
	if projects := collection.Summary.Get("projects"); projects.IsArray() {
		for _, project := range projects.Items() {
			if !project.IsEmpty() {
				load.projects[project.Text()] = true
			}
		}
	}
	return load
}

// add takes one manifest row: it notes the row's member for the staged
// paths, and makes a collected file row an artifact of its home, or a
// problem when its file is not in the collection.
func (load *manifestLoad) add(row jsonvalue.Value) {
	key := homeKey{user: textOf(row.Get("user")), home: textOf(row.Get("home"))}
	original := textOf(row.Get("path"))
	archivePath := textOf(row.Get("archive_path"))
	link := isText(row.Get("type"), "symlink")
	if archivePath != "" || link {
		load.archived[key] = append(load.archived[key],
			archivedRow{rel: relativeToHome(original, key.home), archivePath: archivePath, link: link})
	}
	if !isText(row.Get("status"), "collected") || !isText(row.Get("type"), "file") {
		return
	}
	home := load.homeOf(key, row)
	rel := relativeToHome(original, key.home)
	agent := textOf(row.Get("agent"))
	if agent == ProjectAgent {
		// Collectors before 1.12.0 recorded every project-catalog file as
		// project; the project catalog names its agent.
		home.Project = true
		if agent = load.matcher.ProjectAgentFor(rel); agent == "" {
			agent = ProjectAgent
		}
	}
	collection := load.collection
	diskPath, reason := memberPath(collection.Root, archivePath)
	if reason != "" {
		label := original
		if label == "" {
			label = archivePath
		}
		collection.Problems = append(collection.Problems, label+": "+reason)
		return
	}
	collection.Artifacts = append(collection.Artifacts, &Artifact{DiskPath: diskPath, Original: original,
		Rel: rel, Agent: agent, Home: home, Manifest: row})
}

// homeOf returns the home of key, made from row when it is the home's first
// collected file. A home whose directory no row has shown takes the
// collector's layout as a fallback until a later row shows it.
func (load *manifestLoad) homeOf(key homeKey, row jsonvalue.Value) *Home {
	root := load.collection.Root
	archiveDir := archiveDirFromRow(row)
	home := load.homes[key]
	if home == nil {
		if archiveDir == "" {
			archiveDir = archiveDirFallback(key.home, load.imageRoot)
			load.provisional[key] = true
		}
		home = &Home{DiskPath: joinPath(root, archiveDir), Original: key.home, User: key.user,
			Host: load.collection.Host, Project: load.projects[key.home], StagedPaths: map[string]string{}}
		load.homes[key] = home
		load.order = append(load.order, key)
	} else if load.provisional[key] && archiveDir != "" {
		home.DiskPath = joinPath(root, archiveDir)
		delete(load.provisional, key)
	}
	return home
}

// memberPath returns where the collection holds archivePath, or the reason
// it cannot be an artifact: absent, not a regular file, or a path that
// would leave the collection: one starting with a slash, a backslash or a
// drive, or with a ".." element between either kind of slash, on every
// host.
func memberPath(root, archivePath string) (string, string) {
	if archivePath == "" {
		return "", "missing from the collection"
	}
	if strings.HasPrefix(archivePath, "/") || strings.HasPrefix(archivePath, `\`) ||
		windowsDrive(archivePath) != "" || containsPart(archiveParts(archivePath), "..") {
		return "", "outside the collection"
	}
	diskPath := joinPath(root, archivePath)
	info, err := osfile.Lstat(diskPath)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "", "missing from the collection"
	case err != nil:
		return "", errorReason(err)
	case !info.Mode().IsRegular():
		return "", "missing from the collection"
	}
	return diskPath, ""
}

// containsPart reports whether parts holds part.
func containsPart(parts []string, part string) bool {
	for _, candidate := range parts {
		if candidate == part {
			return true
		}
	}
	return false
}

// finish fills each home's staged paths from its rows and stores the homes
// in the order they were made.
func (load *manifestLoad) finish() {
	root := load.collection.Root
	for _, key := range load.order {
		home := load.homes[key]
		rows := load.archived[key]
		links := map[string]bool{}
		inPlace := map[string]bool{}
		for _, row := range rows {
			if row.link {
				links[row.rel] = true
			}
			// A row archived in place shows every directory above it real.
			if row.archivePath != "" && archivedInPlace(home, root, row.rel, row.archivePath) {
				parts := splitSlash(row.rel)
				for depth := 1; depth < len(parts); depth++ {
					inPlace[joinSlash(parts[:depth])] = true
				}
			}
		}
		// Files first: a directory a file's row maps takes precedence over a
		// nested link's own member at that path.
		for _, wantLink := range []bool{false, true} {
			for _, row := range rows {
				if row.link == wantLink && row.archivePath != "" {
					mapStagedPath(home, root, row, links, inPlace)
				}
			}
		}
		load.collection.Homes = append(load.collection.Homes, home)
	}
}
