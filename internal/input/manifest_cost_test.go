package input

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

func TestARepeatedProblemIsFoundWithoutSearchingTheList(t *testing.T) {
	collection := &Collection{}
	for i := range 1000 {
		collection.recordProblem("line " + strconv.Itoa(i))
	}
	collection.recordProblem("line 7")
	wantEqual(t, "problems after a repeat", len(collection.Problems), 1000)
	// A line placed in the list by hand is unknown to the set: recording it
	// again adds it, which shows the list itself is never searched.
	collection.Problems = append(collection.Problems, "by hand")
	collection.recordProblem("by hand")
	wantEqual(t, "problems after a line the set does not hold", len(collection.Problems), 1002)
}

func TestManyRefusedRowsEachGiveOneProblem(t *testing.T) {
	dir := t.TempDir()
	rows := []string{collectedRow("bob", "/home/bob", "pi", "/home/bob/.pi/a", "fs/home/bob/.pi/a")}
	for i := range 200 {
		rows = append(rows, jsonRow("user", "bob", "home", "/home/bob", "agent", "pi",
			"path", "/home/bob/.pi/l"+strconv.Itoa(i), "archive_path", "/out/"+strconv.Itoa(i),
			"type", "symlink", "status", "symlink"))
	}
	writeManifest(t, dir, rows...)
	writeFile(t, filepath.Join(dir, "fs", "home", "bob", ".pi", "a"), "[]")
	collection, err := Open(t.Context(), dir, Options{Matcher: matcherOf(t, "")})
	opened(t, err)
	wantEqual(t, "problems", len(collection.Problems), 200)
}

// deepRows returns a home's file row and one row whose path lies depth
// directories below the home, archived in place, or at a staged location
// when staged is set.
func deepRows(depth int, staged bool) []string {
	rel := strings.Repeat("x/", depth) + "f"
	archivePath := "fs/home/a/" + rel
	kind, status := "file", "collected"
	if staged {
		archivePath, kind, status = "fs/staged/"+rel, "symlink", "symlink"
	}
	return []string{collectedRow("a", "/home/a", "pi", "/home/a/.pi/s", "fs/home/a/.pi/s"),
		jsonRow("user", "a", "home", "/home/a", "agent", "pi", "path", "/home/a/"+rel,
			"archive_path", archivePath, "type", kind, "status", status, "size", 2)}
}

// loadBytes returns the bytes allocated while rows are attributed to their
// homes and the homes' staged paths are filled.
func loadBytes(t *testing.T, rows []string) uint64 {
	t.Helper()
	collection := &Collection{Root: t.TempDir()}
	if err := os.Mkdir(filepath.Join(collection.Root, "fs"), 0o777); err != nil {
		t.Fatal(err)
	}
	values := make([]jsonvalue.Value, len(rows))
	for i, row := range rows {
		values[i], _ = jsonvalue.DecodeString(row)
	}
	load := newManifestLoad(collection, matcherOf(t, ""))
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	for _, row := range values {
		load.add(row)
	}
	load.finish()
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}

func TestADeepRowCostsMemoryInProportionToItsLength(t *testing.T) {
	for _, staged := range []bool{false, true} {
		t.Run("staged "+strconv.FormatBool(staged), func(t *testing.T) {
			small := loadBytes(t, deepRows(1000, staged))
			large := loadBytes(t, deepRows(2000, staged))
			// Twice the depth may cost twice the bytes, with room for a
			// map's growth; a cost growing with the square costs four times.
			if large > 3*small {
				t.Errorf("depth 1000 allocated %d bytes, depth 2000 %d: more than three times", small, large)
			}
		})
	}
}

func TestARowDeeperThanTheLimitIsAProblem(t *testing.T) {
	for _, test := range []struct {
		name    string
		depth   int
		refused bool
	}{
		{"a path of exactly the limit's components is read", maxPathComponents - 3, false},
		{"a path one component past the limit is a problem", maxPathComponents - 2, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			// /home/a/ adds two components and the file name one.
			writeManifest(t, dir, deepRows(test.depth, true)...)
			writeFile(t, filepath.Join(dir, "fs", "home", "a", ".pi", "s"), "[]")
			collection, err := Open(t.Context(), dir, Options{Matcher: matcherOf(t, "")})
			opened(t, err)
			refused := slices.ContainsFunc(collection.Problems, func(line string) bool {
				return strings.HasSuffix(line, ": "+errTooDeep.Error())
			})
			wantEqual(t, "refused", refused, test.refused)
			wantEqual(t, "staged keys", len(collection.Homes[0].StagedPaths) > 0, !test.refused)
		})
	}
}

func TestAStagedRowWhosePathClimbsIsAProblemAndMapsNothing(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, collectedRow("a", "/home/a", "pi", "/home/a/.pi/s", "fs/home/a/.pi/s"),
		jsonRow("user", "a", "home", "/home/a", "agent", "pi", "path", "/home/a/x/../y/s/f",
			"archive_path", "fs/staged/s/f", "type", "symlink", "status", "symlink"))
	writeFile(t, filepath.Join(dir, "fs", "home", "a", ".pi", "s"), "[]")
	writeFile(t, filepath.Join(dir, "fs", "staged", "s", "g"), "[]")
	collection, err := Open(t.Context(), dir, Options{Matcher: matcherOf(t, "")})
	opened(t, err)
	wantProblem(t, collection, "/home/a/x/../y/s/f: "+errOutside.Error())
	wantEqual(t, "staged keys", len(collection.Homes[0].StagedPaths), 0)
	if path, ok := collection.Homes[0].PathOf("x/../y/s/g"); ok {
		t.Errorf("PathOf gave %s for a path holding ..", path)
	}
}

func TestACollectedRowWithANegativeSizeIsAProblem(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, collectedRow("a", "/home/a", "pi", "/home/a/.pi/s", "fs/home/a/.pi/s"),
		jsonRow("user", "a", "home", "/home/a", "agent", "pi", "path", "/home/a/.pi/t",
			"archive_path", "fs/home/a/.pi/t", "type", "file", "status", "collected", "size", -5))
	writeFile(t, filepath.Join(dir, "fs", "home", "a", ".pi", "s"), "[]")
	writeFile(t, filepath.Join(dir, "fs", "home", "a", ".pi", "t"), "[]")
	collection, err := Open(t.Context(), dir, Options{Matcher: matcherOf(t, "")})
	opened(t, err)
	wantProblem(t, collection, "/home/a/.pi/t: "+errNegativeSize.Error())
	wantSlice(t, "artifacts", rels(collection), []string{".pi/s"})
}

// The directories above a row are its components', however many slashes
// repeat between them, in the form the staged paths look them up: the
// bytes of the keys made grow with the components, not with the square of
// the path's length.
func TestTheDirectoriesAboveARowAreOnePerComponent(t *testing.T) {
	tests := []struct {
		name, rel string
		want      []string
	}{
		{"a plain path gives each directory", "a/b/c", []string{"a", "a/b"}},
		{"repeated slashes give the same directories", "a//b///c", []string{"a", "a/b"}},
		{"a file at the home gives none", "f", nil},
		{"leading and trailing slashes add none", "/a/b/", []string{"a"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			wantSlice(t, "directories", directoriesAbove(test.rel), test.want)
		})
	}
	keyBytes := func(slashes int) int {
		total := 0
		for _, directory := range directoriesAbove(".pi/x" + strings.Repeat("/", slashes) + "f") {
			total += len(directory)
		}
		return total
	}
	if small, large := keyBytes(1<<15), keyBytes(1<<16); large != small {
		t.Errorf("keys of %d and %d bytes for 32,768 and 65,536 slashes, want the same", small, large)
	}
}
