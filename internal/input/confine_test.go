package input

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// plantedText is what a file outside every test collection holds; no path
// the collection gives may lead to it.
const plantedText = "PLANTED-OUTSIDE-THE-COLLECTION"

// plantOutside writes the planted file at rel below dir/outside and
// returns dir/outside.
func plantOutside(t *testing.T, dir, rel string) string {
	t.Helper()
	outside := filepath.Join(dir, "outside")
	writeFile(t, filepath.Join(outside, filepath.FromSlash(rel)), plantedText)
	return outside
}

// wantConfined fails the test when a path the collection gives, a home's,
// a staged location or an artifact's, does not lie below the root, or when
// an artifact's file holds the planted text.
func wantConfined(t *testing.T, collection *Collection) {
	t.Helper()
	under := func(what, path string) {
		t.Helper()
		if path != collection.Root && !strings.HasPrefix(path, collection.Root+string(filepath.Separator)) {
			t.Errorf("%s %s lies outside the collection %s", what, path, collection.Root)
		}
	}
	for _, home := range collection.Homes {
		under("home", home.DiskPath)
		for rel, staged := range home.StagedPaths {
			under("staged path of "+rel, staged)
		}
	}
	for _, artifact := range collection.Artifacts {
		under("artifact", artifact.DiskPath)
		if data, err := os.ReadFile(artifact.DiskPath); err == nil && strings.Contains(string(data), plantedText) {
			t.Errorf("artifact %s reads the planted file", artifact.DiskPath)
		}
	}
}

// wantProblem fails the test unless a problem line holds text.
func wantProblem(t *testing.T, collection *Collection, text string) {
	t.Helper()
	for _, problem := range collection.Problems {
		if strings.Contains(problem, text) {
			return
		}
	}
	t.Errorf("no problem holds %q: %q", text, collection.Problems)
}

// geminiFile is the one real file of the confinement cases' collections.
func geminiFile(t *testing.T, root string) string {
	t.Helper()
	writeFile(t, filepath.Join(root, "fs/home/a/.gemini/tmp/slug/shell_history"), "ls -la\n")
	return collectedRow("a", "/home/a", "gemini-cli", "/home/a/.gemini/tmp/slug/shell_history",
		"fs/home/a/.gemini/tmp/slug/shell_history")
}

func TestManifestTextNeverNamesAPlaceOutsideTheCollection(t *testing.T) {
	climbing := strings.Repeat("../", 30)
	cases := []struct {
		name, row, problem string
	}{
		{"a link row whose archive path climbs out maps no staged path",
			jsonRow("user", "a", "home", "/home/a", "agent", "gemini-cli", "path", "/home/a/.gemini",
				"archive_path", "../outside/.gemini", "type", "symlink", "status", "collected", "size", 0),
			"/home/a/.gemini: outside the collection"},
		{"a link row whose archive path climbs to an absolute place maps no staged path",
			jsonRow("user", "a", "home", "/home/a", "agent", "gemini-cli", "path", "/home/a/.gemini",
				"archive_path", climbing+"etc/.gemini", "type", "symlink", "status", "collected", "size", 0),
			"/home/a/.gemini: outside the collection"},
		{"a home field that climbs out makes no home",
			collectedRow("b", "/../../outside", "gemini-cli", "/../../outside/.gemini/tmp/slug/shell_history",
				"fs/x/shell_history"),
			"home /../../outside: outside the collection"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			plantOutside(t, dir, ".gemini/tmp/slug/.project_root")
			root := filepath.Join(dir, "collection")
			writeFile(t, filepath.Join(root, "fs/x/shell_history"), "x\n")
			writeManifest(t, root, geminiFile(t, root), test.row)
			collection, err := Open(t.Context(), root, Options{Matcher: matcherOf(t, "")})
			opened(t, err)
			wantConfined(t, collection)
			wantProblem(t, collection, test.problem)
			for _, home := range collection.Homes {
				if path, ok := home.PathOf(".gemini/tmp/slug/.project_root"); ok &&
					!strings.HasPrefix(path, root+string(filepath.Separator)) {
					t.Errorf("PathOf gave %s", path)
				}
			}
		})
	}
}

func TestAnArchivedLinkRowClimbingOutMapsNothing(t *testing.T) {
	dir := t.TempDir()
	staging := filepath.Join(dir, "staging")
	writeFile(t, filepath.Join(staging, "fs/home/a/.gemini/tmp/slug/shell_history"), "ls -la\n")
	writeManifest(t, staging,
		collectedRow("a", "/home/a", "gemini-cli", "/home/a/.gemini/tmp/slug/shell_history",
			"fs/home/a/.gemini/tmp/slug/shell_history"),
		jsonRow("user", "a", "home", "/home/a", "agent", "gemini-cli", "path", "/home/a/.gemini",
			"archive_path", strings.Repeat("../", 30)+strings.TrimPrefix(filepath.ToSlash(dir), "/")+
				"/outside/.gemini", "type", "symlink", "status", "collected", "size", 0))
	plantOutside(t, dir, ".gemini/tmp/slug/.project_root")
	archive := filepath.Join(dir, "h_agent-artifacts.tar.gz")
	writeFile(t, archive, string(tarBytes(t, true, treeMembers(t, staging)...)))
	collection, err := Open(t.Context(), archive, Options{Matcher: matcherOf(t, "")})
	opened(t, err)
	defer collection.Cleanup()
	wantConfined(t, collection)
	wantProblem(t, collection, "/home/a/.gemini: outside the collection")
	if len(collection.Homes[0].StagedPaths) != 0 {
		t.Errorf("staged paths %q", collection.Homes[0].StagedPaths)
	}
}

func TestPathOfRefusesARelativePathThatLeavesTheHome(t *testing.T) {
	home := &Home{DiskPath: t.TempDir()}
	for _, rel := range []string{"..", "a/../..", "../outside/x", `a\..\..\x`, "a/\x00/b"} {
		if path, ok := home.PathOf(rel); ok {
			t.Errorf("PathOf(%q) = %s", rel, path)
		}
	}
	if path, ok := home.PathOf("./a//b/"); !ok || path != filepath.Join(home.DiskPath, "a", "b") {
		t.Errorf("PathOf of empty and dot elements = %s, %v", path, ok)
	}
}

func TestBelowRefusesAnEmptyBase(t *testing.T) {
	if path, ok := Below("", "x"); ok {
		t.Errorf("Below of an empty base gave %q", path)
	}
}
