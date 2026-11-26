package analyze

import (
	"fmt"
	"maps"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/evidencepath"
	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// testFinder returns the scratch finder of the embedded catalog.
func testFinder(t *testing.T) *scratchFinder {
	t.Helper()
	tables, err := catalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	matcher, err := catalog.NewMatcher(tables)
	if err != nil {
		t.Fatal(err)
	}
	return newScratchFinder(matcher)
}

// scanningFinder is the scratch finder as it was before its entries were
// indexed by their first segment: every entry is tried at every component
// of every path, and a path's answer is not kept. It is the reference the
// indexed finder is compared with.
type scanningFinder struct {
	finder  *scratchFinder
	entries [][]string
}

// newScanningFinder lists the segments of the project entries that name an
// agent's directory as the scanning finder tries them.
func newScanningFinder(finder *scratchFinder) *scanningFinder {
	scanning := &scanningFinder{finder: finder}
	for _, pattern := range finder.found.Project {
		if pattern.Agent != input.ProjectAgent {
			scanning.entries = append(scanning.entries, pattern.Segments)
		}
	}
	return scanning
}

// underProjectEntry is the scanning form of scratchFinder.underProjectEntry.
func (s *scanningFinder) underProjectEntry(key string) bool {
	windows := timeline.IsWindowsPath(key)
	parts := slices.DeleteFunc(evidencepath.Elements(key, windows), func(part string) bool { return part == "." })
	for _, segments := range s.entries {
		for start := 1; start < len(parts)-len(segments); start++ {
			if s.finder.matchesAll(segments, parts[start:start+len(segments)]) {
				return true
			}
		}
	}
	return false
}

// scratchKeys is the scanning form of scratchFinder.scratchKeys.
func (s *scanningFinder) scratchKeys(collection *input.Collection, rows []timeline.Row) map[string]bool {
	keys := map[string]bool{}
	for _, row := range rows {
		if row.TurnType == timeline.TurnSystem && row.ProjectPath != "" &&
			strings.HasPrefix(row.Text, timeline.ScratchPrefix) {
			keys[timeline.ProjectKey(row.ProjectPath)] = true
		}
	}
	var homes []string
	for _, home := range collection.Homes {
		if home.Original != "" && !home.Project {
			homes = append(homes, stripImageRoot(home.Original, collection.Summary))
		}
	}
	paths := collectedProjects(collection)
	for _, row := range rows {
		paths = append(paths, row.ProjectPath)
	}
	for _, path := range paths {
		key := timeline.ProjectKey(path)
		if path != "" && !keys[key] && (s.underProjectEntry(key) || s.underHomeClaim(key, homes)) {
			keys[key] = true
		}
	}
	return keys
}

// underHomeClaim is scratchFinder.underHomeClaim as it was before the homes
// were looked up by key: every home is compared with every path.
func (s *scanningFinder) underHomeClaim(key string, homes []string) bool {
	for _, home := range homes {
		windows := timeline.IsWindowsPath(home)
		separator := "/"
		if windows {
			separator = `\`
		}
		homeKey := timeline.ProjectKey(home)
		prefix := homeKey
		if !strings.HasSuffix(prefix, separator) {
			prefix += separator
		}
		if key != homeKey && !strings.HasPrefix(key, prefix) {
			continue
		}
		if s.finder.found.ClaimsHomePath(key[len(homeKey):], separator) != "" {
			return true
		}
	}
	return false
}

// pathComponents returns the components generated paths are made of: the
// project entries' names in several cases, names their globs match, names
// that only fold to an entry's (a Kelvin sign for a k, a long s for an
// s), a byte that is not UTF-8, and ordinary names.
func pathComponents() []string {
	return []string{"src", "home", "alice", "a", "proj", ".", "", ".claude", ".CLAUDE", ".Claude",
		".git", ".GIT", "HEAD", "head", "config", ".github", ".GitHub", "copilot", "Copilot", "hooks",
		".vscode", "cody.json", "CODY.JSON", "memory", "MEMORY.md", "history.jsonl", "kilo.json5",
		"KILO.JSON", "opencode.jsonc", "diagnostics_1_x.json", "diagnostics_x_1.json", "pi-session-1.html",
		"session-2025-a.jsonl", ".roorules-code", ".\u212Ailo", ".\u017Fkills", ".kilo", ".skills",
		"\xff", ".claude\xff", ".pi", "approved-plan.md", "cron", "runs", "AGENTS.md", "\uFFFD"}
}

// randomPath returns a path of up to eight components: a POSIX path, a
// drive path, a share or an extended-length path, at times with a trailing
// separator.
func randomPath(random *rand.Rand) string {
	components := pathComponents()
	var parts []string
	for range random.IntN(9) {
		parts = append(parts, components[random.IntN(len(components))])
	}
	forms := []struct{ start, separator string }{{"/", "/"}, {"C:\\", "\\"}, {"c:/", "/"}, {`\\server\share\`, "\\"},
		{`\\?\D:\`, "\\"}, {"", "/"}}
	form := forms[random.IntN(len(forms))]
	path := form.start + strings.Join(parts, form.separator)
	if random.IntN(4) == 0 {
		path += form.separator
	}
	return path
}

// The indexed finder gives every path the answer of trying every entry at
// every component.
func TestTheIndexedFinderAnswersAsTryingEveryEntryDoes(t *testing.T) {
	t.Parallel()
	finder := testFinder(t)
	scanning := newScanningFinder(finder)
	cases := []string{"/home/alice/.claude/x", "/.claude/x", "/a/.claude", "/a/.claude/", "C:\\Users\\a\\.CLAUDE\\x",
		"/a/.git/HEAD/x", "/a/.git/HEAD", "/a/.github/copilot/.claude/x", "/a/.\u212Ailo/x", "/a/.\u017Fkills/x",
		"C:\\a\\.\u212Ailo\\x", "C:\\a\\.\u017Fkills\\x", "/a/kilo.json5/x", "C:\\a\\KILO.JSON5\\x", "/a/\xff/x",
		"/a/./.claude/x", "//a//.claude//x"}
	random := rand.New(rand.NewPCG(1, 2))
	for range 20000 {
		cases = append(cases, randomPath(random))
	}
	differ := 0
	for _, path := range cases {
		key := timeline.ProjectKey(path)
		if got, want := finder.underProjectEntry(key), scanning.underProjectEntry(key); got != want {
			differ++
			t.Errorf("%q: the indexed finder says %v, trying every entry %v", path, got, want)
		}
		if differ > 10 {
			t.Fatal("more than ten paths differ")
		}
	}
}

// A project path is a scratch directory by the rule collect matches the
// catalog with: ASCII case ignored on every platform, no other fold.
func TestAScratchDirectoryIsMatchedAsCollectMatchesTheCatalog(t *testing.T) {
	t.Parallel()
	finder := testFinder(t)
	cases := []struct {
		path string
		want bool
	}{
		{"/home/alice/proj/.CLAUDE/x", true},
		{`C:\Users\a\proj\.KILO\x`, true},
		{"/home/alice/proj/.\u212Ailo/x", false},
		{`C:\Users\a\proj\.` + "\u212A" + `ilo\x`, false},
		{"/home/alice/proj/.\u017Fkills/x", false},
	}
	for _, test := range cases {
		if got := finder.underProjectEntry(timeline.ProjectKey(test.path)); got != test.want {
			t.Errorf("%q: scratch directory %v, want %v", test.path, got, test.want)
		}
	}
}

// randomCollection returns rows and a collection of homes and projects
// drawn from a few generated paths, so paths repeat across rows.
func randomCollection(random *rand.Rand) (*input.Collection, []timeline.Row) {
	paths := []string{"/home/alice", "C:\\Users\\Bob", "/home/alice/.claude/projects/x", ""}
	for range 6 {
		paths = append(paths, randomPath(random))
	}
	pick := func() string { return paths[random.IntN(len(paths))] }
	collection := &input.Collection{Homes: []*input.Home{{Original: "/home/alice"}, {Original: "C:\\Users\\Bob"},
		{Original: pick(), Project: true}}}
	var rows []timeline.Row
	for range 30 {
		row := timeline.Row{ProjectPath: pick(), TurnType: timeline.TurnUser}
		if random.IntN(10) == 0 {
			row.TurnType, row.Text = timeline.TurnSystem, timeline.ScratchPrefix+" x"
		}
		rows = append(rows, row)
	}
	return collection, rows
}

// The scratch keys of a run are the same whether each path's answer is
// kept for the run or found again for each row.
func TestScratchKeysAreTheSameWithEachAnswerKept(t *testing.T) {
	t.Parallel()
	finder := testFinder(t)
	scanning := newScanningFinder(finder)
	random := rand.New(rand.NewPCG(3, 4))
	for i := range 2000 {
		collection, rows := randomCollection(random)
		if got, want := finder.scratchKeys(collection, rows), scanning.scratchKeys(collection, rows); !maps.Equal(got, want) {
			t.Fatalf("run %d: scratch keys %v, answering each row %v", i, got, want)
		}
	}
}

// longPathRows returns n rows naming one path of components components,
// none of them an entry's name.
func longPathRows(n, components int) []timeline.Row {
	path := strings.Repeat("/a", components)
	rows := make([]timeline.Row, n)
	for i := range rows {
		rows[i] = timeline.Row{ProjectPath: path, TurnType: timeline.TurnUser}
	}
	return rows
}

// A path is answered once however many rows name it, and its answer tries
// a number of segments that grows with its components, not with the
// components times the entries: one for each entry whose first segment is
// a glob, at each component.
func TestEachPathIsAnsweredOnceInWorkThatGrowsWithItsLength(t *testing.T) {
	tests := map[[2]int]int{}
	globbed := len(testFinder(t).globbed)
	for _, size := range [][2]int{{1000, 2000}, {2000, 2000}, {1000, 4000}} {
		t.Run(fmt.Sprintf("%d rows of %d components", size[0], size[1]), func(t *testing.T) {
			finder := testFinder(t)
			if keys := finder.scratchKeys(&input.Collection{}, longPathRows(size[0], size[1])); len(keys) != 0 {
				t.Fatalf("scratch keys %v, want none", keys)
			}
			tests[size] = finder.tests
		})
	}
	if tests[[2]int{2000, 2000}] != tests[[2]int{1000, 2000}] {
		t.Errorf("twice the rows tried %d segments, want the %d of one answer", tests[[2]int{2000, 2000}],
			tests[[2]int{1000, 2000}])
	}
	if once := tests[[2]int{1000, 2000}]; once > 2000*globbed || tests[[2]int{1000, 4000}] > 2*once+2*globbed {
		t.Errorf("2000 components tried %d segments and 4000 %d, want at most %d a component",
			once, tests[[2]int{1000, 4000}], globbed)
	}
}

// Looking the homes up by key answers every path as comparing it with
// every home does: homes nested in one another, Windows and POSIX homes,
// homes with a trailing separator, and paths in, under and beside them.
func TestTheHomeLookupAnswersAsComparingEveryHomeDoes(t *testing.T) {
	t.Parallel()
	finder := testFinder(t)
	scanning := newScanningFinder(finder)
	random := rand.New(rand.NewPCG(5, 6))
	claimed := 0
	for range 3000 {
		var homes []string
		indexed := newHomeKeys()
		for range 1 + random.IntN(4) {
			home := randomPath(random)
			if cut := strings.LastIndexAny(home, `/\`); cut > 0 && random.IntN(3) == 0 {
				home = home[:cut]
			}
			homes = append(homes, home)
			indexed.add(home)
		}
		for range 5 {
			path := homes[random.IntN(len(homes))] + randomPath(random)
			if random.IntN(4) == 0 {
				path = randomPath(random)
			}
			key := timeline.ProjectKey(path)
			got, want := finder.underHomeClaim(key, indexed), scanning.underHomeClaim(key, homes)
			if got != want {
				t.Fatalf("%q in homes %q: the lookup says %v, comparing every home %v", path, homes, got, want)
			}
			if got {
				claimed++
			}
		}
	}
	if claimed < 100 {
		t.Errorf("only %d paths were claimed, too few to compare", claimed)
	}
}

// A path is compared with the homes in a number of lookups that grows with
// its components, whatever the number of homes, so n homes and n paths
// cost at most two lookups a component, not n times n.
func TestTheHomeLookupsGrowWithThePathsNotTheHomes(t *testing.T) {
	lookups := map[int]int{}
	for _, count := range []int{500, 1000} {
		finder := testFinder(t)
		prefix := strings.Repeat("/d", 100)
		collection := &input.Collection{}
		var rows []timeline.Row
		for i := range count {
			collection.Homes = append(collection.Homes, &input.Home{Original: prefix + "/home" + strconv.Itoa(i)})
			rows = append(rows, timeline.Row{ProjectPath: prefix + "/work" + strconv.Itoa(i) + "/src",
				TurnType: timeline.TurnUser})
		}
		if keys := finder.scratchKeys(collection, rows); len(keys) != 0 {
			t.Fatalf("scratch keys %v, want none", keys)
		}
		lookups[count] = finder.homeLookups
	}
	if lookups[1000] > 2*lookups[500]+10 || lookups[500] > 500*2*103 {
		t.Errorf("500 homes and paths made %d lookups and 1000 made %d, want about two a component of each path",
			lookups[500], lookups[1000])
	}
}

// A project path collection.json names is answered in a number of bytes
// hashed and compared that grows with its length, not with its square,
// however many homes there are: a path twice as long costs about twice
// the bytes.
func TestTheHomeLookupOfALongPathGrowsWithItsLength(t *testing.T) {
	work := map[int]int{}
	for _, length := range []int{64 << 10, 128 << 10} {
		finder := testFinder(t)
		collection := &input.Collection{Summary: jsonvalue.NewObject(jsonvalue.Member{Key: "projects",
			Value: jsonvalue.NewArray(jsonvalue.NewString(strings.Repeat("/a", length/2)))})}
		for i := range 20 {
			collection.Homes = append(collection.Homes, &input.Home{Original: "/home/user" + strconv.Itoa(i)})
		}
		if keys := finder.scratchKeys(collection, nil); len(keys) != 0 {
			t.Fatalf("scratch keys %v, want none", keys)
		}
		work[length] = finder.homeBytes
	}
	if small, large := work[64<<10], work[128<<10]; small < 64<<10 || large > 2*small+64 {
		t.Errorf("a path of 64 KiB cost %d bytes and one of 128 KiB %d, want about twice", small, large)
	}
}
