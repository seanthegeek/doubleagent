package input

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
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
// home-relative path, its archive path, the label a problem with it names
// and whether it is a symlink row.
type archivedRow struct {
	rel, archivePath, label string
	link                    bool
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
func (c *Collection) loadCollected(ctx context.Context, matcher *catalog.Matcher) error {
	c.readSummary()
	if c.Host == "" && !c.Summary.IsEmpty() {
		c.Host = textOf(c.Summary.Get("hostname"))
	}
	rows, bad, err := c.readManifest(ctx)
	if err != nil {
		return err
	}
	if bad > 0 {
		c.Notes = append(c.Notes, strconv.Itoa(bad)+" manifest "+plural(bad, "row")+" did not parse")
	}
	c.Manifest = rows
	load := newManifestLoad(c, matcher)
	for _, row := range rows {
		if err := ctx.Err(); err != nil {
			return err
		}
		load.add(row)
	}
	load.finish()
	return nil
}

// readManifest reads manifest.jsonl under the root as a stream through
// parseManifest; a line too long to be a row is a problem naming its
// number.
func (c *Collection) readManifest(ctx context.Context) ([]jsonvalue.Value, int, error) {
	path := joinPath(c.Root, "manifest.jsonl")
	file, err := (&fsmeta.Opener{ReadOnly: true}).OpenRegular(path)
	if err != nil {
		return nil, 0, err
	}
	defer file.Close()
	var repeats jsonvalue.Repeats
	rows, bad, long, err := parseManifest(ctx, file, &repeats)
	if err != nil {
		return nil, 0, &fs.PathError{Op: "read", Path: path, Err: err}
	}
	if repeats.Count() > 0 {
		c.addProblem("manifest.jsonl: " + repeats.Describe(MaxPathLength))
	}
	for _, number := range long {
		c.addProblem("manifest.jsonl line " + strconv.Itoa(number) + ": longer than " +
			strconv.Itoa(maxManifestLine) + " bytes, not read")
	}
	return rows, bad, nil
}

// plural returns noun, with an s unless count is one.
func plural(count int, noun string) string {
	if count == 1 {
		return noun
	}
	return noun + "s"
}

// parseManifest reads manifest rows from r: each line decoded as UTF-8 with
// replacement and trimmed of JSON whitespace, blank lines skipped. A line
// that is not a JSON object, or a collected file row whose size parseSize
// refuses, is counted as bad instead of kept; the numbers of the lines
// longer than maxManifestLine are returned apart. The member names the
// rows repeat within an object are noted in repeats.
func parseManifest(ctx context.Context, r io.Reader, repeats *jsonvalue.Repeats) (rows []jsonvalue.Value, bad int,
	long []int, err error) {
	rows = []jsonvalue.Value{}
	err = readLines(ctx, r, maxManifestLine, func(number int, line []byte, tooLong bool) {
		if tooLong {
			long = append(long, number)
			return
		}
		text := strings.Trim(textlines.ValidUTF8(line), jsonSpace)
		if text == "" {
			return
		}
		document := jsonvalue.NewDocument(text)
		row, err := document.Decode()
		if err != nil || !row.IsObject() || !sizeReadable(row) {
			bad++
			return
		}
		repeats.Merge(document.TakeRepeats())
		rows = append(rows, row)
	})
	return rows, bad, long, err
}

// sizeReadable reports whether a row that becomes an artifact has a size
// parseSize reads; other rows are not asked.
func sizeReadable(row jsonvalue.Value) bool {
	if !isText(row.Get("status"), "collected") || !isText(row.Get("type"), "file") {
		return true
	}
	_, ok := parseSize(row.Get("size"))
	return ok
}

// maxSummary is the largest collection.json read; the one `collect` writes
// is a few kilobytes, its project list included.
const maxSummary = 16 << 20

// readSummary reads collection.json when it is a regular file. A file that
// is larger than maxSummary, not strict UTF-8, not JSON, or a JSON value
// that is true but not an object leaves a note and no summary; a false
// value (null, [], {}) is no summary and no note.
func (c *Collection) readSummary() {
	path := joinPath(c.Root, "collection.json")
	if !lstatMode(path).IsRegular() {
		return
	}
	data, err := readBounded(path, maxSummary)
	if err != nil {
		c.Notes = append(c.Notes, "collection.json unreadable: "+errorReason(err))
		return
	}
	summary, repeats, err := decodeSummary(data)
	if err != nil {
		c.Notes = append(c.Notes, "collection.json unreadable: "+err.Error())
		return
	}
	if repeats.Count() > 0 {
		c.addProblem("collection.json: " + repeats.Describe(MaxPathLength))
	}
	if summary.IsObject() {
		c.Summary = summary
	}
}

// errTooLarge is the reason a file larger than its bound is not read.
var errTooLarge = errors.New("larger than doubleagent reads")

// readBounded returns the content of the regular file at path, or
// errTooLarge, after reading at most limit+1 bytes, when it holds more
// than limit.
func readBounded(path string, limit int64) ([]byte, error) {
	file, err := (&fsmeta.Opener{ReadOnly: true}).OpenRegular(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, &fs.PathError{Op: "read", Path: path, Err: err}
	}
	if int64(len(data)) > limit {
		return nil, errTooLarge
	}
	return data, nil
}

// decodeSummary decodes collection.json bytes as strict UTF-8 JSON, with
// the member names it repeats within an object.
func decodeSummary(data []byte) (jsonvalue.Value, jsonvalue.Repeats, error) {
	if !utf8.Valid(data) {
		return jsonvalue.Value{}, jsonvalue.Repeats{}, errors.New("not valid UTF-8")
	}
	document := jsonvalue.NewDocument(string(data))
	summary, err := document.Decode()
	if err != nil {
		return jsonvalue.Value{}, jsonvalue.Repeats{}, err
	}
	if !summary.IsEmpty() && !summary.IsObject() {
		return jsonvalue.Value{}, jsonvalue.Repeats{}, errors.New("not a JSON object")
	}
	return summary, document.TakeRepeats(), nil
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
	label := original
	if label == "" {
		label = archivePath
	}
	if tooDeep(original) || tooDeep(archivePath) {
		load.collection.addProblem(label + ": " + errTooDeep.Error())
		return
	}
	link := isText(row.Get("type"), "symlink")
	if archivePath != "" || link {
		load.archived[key] = append(load.archived[key], archivedRow{rel: relativeToHome(original, key.home),
			archivePath: archivePath, label: label, link: link})
	}
	if !isText(row.Get("status"), "collected") || !isText(row.Get("type"), "file") {
		return
	}
	collection := load.collection
	if len(original) > MaxPathLength {
		collection.addProblem(label + ": " + errTooLong.Error())
		return
	}
	if size, _ := parseSize(row.Get("size")); size < 0 {
		collection.addProblem(label + ": " + errNegativeSize.Error())
		return
	}
	home, err := load.homeOf(key, row)
	if err != nil {
		collection.recordProblem(label + ": home " + key.home + ": " + err.Error())
		return
	}
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
	diskPath, reason := load.memberPath(archivePath)
	if reason != "" {
		collection.addProblem(label + ": " + reason)
		return
	}
	collection.Artifacts = append(collection.Artifacts, &Artifact{DiskPath: diskPath, Original: original,
		Rel: rel, Agent: agent, Home: home, Manifest: row})
}

// maxPathComponents is the most components a manifest path may have. A
// path a system's file calls take whole is at most 4095 bytes on Linux and
// 1023 on macOS (PATH_MAX, 4096 and 1024, counts the NUL that ends it), and
// holds at most one component for every two bytes; the limit is twice the
// most such a path holds, room for any real file, while a row's staged
// paths, one per component, stay few.
const maxPathComponents = 4096

// MaxPathLength is the longest path text, in bytes, of a file the input
// makes an artifact of: the manifest's path, where each byte of a name that
// is not UTF-8 is U+FFFD, three bytes, or a loose tree's home path joined
// to the file's part below it. It is Linux's PATH_MAX, which counts the
// NUL that ends a path: one byte more than the longest path Linux's file
// calls take whole. A file's path and its part below the home are written
// in every row the file gives (source_file) and in the text of many, so a
// longer one, which a manifest's text or Windows' extended-length paths
// allow, would cost its length once per row; it is a problem instead.
const MaxPathLength = 4096

// errTooLong is the reason a file whose path is longer than MaxPathLength
// is not read.
var errTooLong = errors.New("a path longer than " + strconv.Itoa(MaxPathLength) + " bytes, not read")

// errTooDeep is the reason a row with a path deeper than maxPathComponents
// is not read.
var errTooDeep = errors.New("more than " + strconv.Itoa(maxPathComponents) +
	" path components, not read")

// errNegativeSize is the reason a collected file row whose size is below
// zero is not read: no file has one, and a total of sizes must not fall.
var errNegativeSize = errors.New("a negative size, not read")

// tooDeep reports whether a manifest path, split at slashes or backslashes,
// has more than maxPathComponents non-empty components.
func tooDeep(path string) bool {
	components := 0
	for i := range len(path) {
		separator := path[i] == '/' || path[i] == '\\'
		if !separator && (i == 0 || path[i-1] == '/' || path[i-1] == '\\') {
			components++
		}
	}
	return components > maxPathComponents
}

// homeOf returns the home of key, made from row when it is the home's first
// collected file. A home whose directory no row has shown takes the
// layout `collect` writes as a fallback until a later row shows it. A directory
// that the row's text would place outside the collection, or reach through
// a link, is an error and makes no home.
func (load *manifestLoad) homeOf(key homeKey, row jsonvalue.Value) (*Home, error) {
	archiveDir := archiveDirFromRow(row)
	home := load.homes[key]
	if home == nil {
		provisional := archiveDir == ""
		if provisional {
			archiveDir = archiveDirFallback(key.home, load.imageRoot)
		}
		diskPath, err := load.collection.inside(archiveParts(archiveDir)...)
		if err != nil {
			return nil, err
		}
		home = &Home{DiskPath: diskPath, Original: key.home, User: key.user, Host: load.collection.Host,
			Project: load.projects[key.home], StagedPaths: map[string]string{},
			names: load.collection.names}
		load.homes[key] = home
		load.order = append(load.order, key)
		load.provisional[key] = provisional
	} else if load.provisional[key] && archiveDir != "" {
		if diskPath, err := load.collection.inside(archiveParts(archiveDir)...); err == nil {
			home.DiskPath = diskPath
			delete(load.provisional, key)
		}
	}
	return home, nil
}

// memberPath returns where the collection holds archivePath, or the reason
// it cannot be an artifact: absent (for an archive, no file the extraction
// made under that name), not a regular file, or refused by archivedPath.
func (load *manifestLoad) memberPath(archivePath string) (string, string) {
	extracted := load.collection.extractedFiles
	if archivePath == "" || extracted != nil && !extracted[joinSlash(archiveParts(archivePath))] {
		return "", "missing from the collection"
	}
	diskPath, err := load.collection.archivedPath(archivePath)
	if err != nil {
		return "", err.Error()
	}
	info, err := osfile.Lstat(diskPath)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "", "missing from the collection"
	case err != nil:
		return "", errorReason(err)
	case !info.Mode().IsRegular() || isLink(info):
		return "", "missing from the collection"
	}
	return diskPath, ""
}

// finish fills each home's staged paths from its rows and stores the homes
// in the order they were made.
func (load *manifestLoad) finish() {
	for _, key := range load.order {
		home := load.homes[key]
		rows := load.archived[key]
		links := map[string]bool{}
		inPlace := map[string]bool{}
		for _, row := range rows {
			// Keyed as the staged paths look a directory up, so a link's
			// row stops the walk however its path repeats a separator.
			if row.link {
				links[strings.Join(splitNonEmpty(row.rel), "/")] = true
			}
			// A row archived in place shows every directory above it real.
			if row.archivePath != "" && archivedInPlace(home, load.collection.Root, row.rel, row.archivePath) {
				for _, directory := range directoriesAbove(row.rel) {
					inPlace[directory] = true
				}
			}
		}
		// Files first: a directory a file's row maps takes precedence over a
		// nested link's own member at that path.
		for _, wantLink := range []bool{false, true} {
			for _, row := range rows {
				if row.link == wantLink && row.archivePath != "" {
					load.mapStagedPath(home, row, links, inPlace)
				}
			}
		}
		load.collection.Homes = append(load.collection.Homes, home)
	}
}

// directoriesAbove returns each directory above rel as the staged paths
// look it up: its non-empty components up to it, joined by single slashes
// (joinedPrefixes). There is one per component, which tooDeep bounds,
// however many separators repeat, and each is a prefix of one string.
func directoriesAbove(rel string) []string {
	parts := splitNonEmpty(rel)
	if len(parts) < 2 {
		return nil
	}
	joined, ends := joinedPrefixes(parts)
	directories := make([]string, len(parts)-1)
	for i := range directories {
		directories[i] = joined[:ends[i+1]]
	}
	return directories
}

// archivedPath returns where the collection holds an archive path from the
// manifest, its elements split at slashes or backslashes: one that starts at
// a slash, a backslash or a drive names a place outside the collection on
// some host and is refused on every host, as is one that inside refuses.
func (c *Collection) archivedPath(archivePath string) (string, error) {
	if startsOutside(archivePath) {
		return "", errOutside
	}
	return c.inside(archiveParts(archivePath)...)
}

// startsOutside reports whether an archive path starts at a slash, a
// backslash or a drive.
func startsOutside(archivePath string) bool {
	return strings.HasPrefix(archivePath, "/") || strings.HasPrefix(archivePath, `\`) ||
		windowsDrive(archivePath) != ""
}
