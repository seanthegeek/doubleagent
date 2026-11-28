package collect_test

import (
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// On Linux and macOS, linux/<case>/ and darwin/<case>/ hold only where that
// platform's output differs from the case in unix/: a file as a delta from
// unix/'s file (name.unix-delta) or, in a case over the main image, from
// the platform's own view of the default case's file (name.delta), or
// whole, and a file the platform does not write as name.absent. Updating on one platform keeps
// what the other expects: it writes unix/ when the other platform holds the
// case on its own or nothing is there yet, else only its own differences,
// and folds both into unix/ when they write the same.

// The suffixes of an overlay's files.
const (
	overlaySuffix = ".unix-delta"
	absentSuffix  = ".absent"
)

// caseTexts is a case's outputs with every delta applied: file name to text.
type caseTexts map[string]string

// goldenViews is what the expected files say each platform of the family
// writes: the whole cases of familyDirectory (shared), and on Linux and
// macOS each platform's own view where it holds an overlay (own).
type goldenViews struct {
	shared map[string]caseTexts
	own    map[string]map[string]caseTexts
}

// view returns what the expected files say platform writes for a case, and
// false when they hold nothing for it.
func (views goldenViews) view(platform, name string) (caseTexts, bool) {
	if texts, found := views.own[platform][name]; found {
		return texts, true
	}
	texts, found := views.shared[name]
	return texts, found
}

// familyMembers names the platforms that read unix/ with their own
// overlays, or none outside Linux and macOS.
func familyMembers() []string {
	if otherUnixPlatform() == "" {
		return nil
	}
	return []string{"darwin", "linux"}
}

// caseNames returns the names of the case directories under directory.
func caseNames(test testing.TB, directory string) []string {
	test.Helper()
	entries, err := os.ReadDir(filepath.Join(goldenRoot, directory))
	if err != nil && !os.IsNotExist(err) {
		test.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

// applyOverlay returns the shared case's texts changed as the overlay says;
// defaults is the platform's view of the default case.
func applyOverlay(test testing.TB, shared caseTexts, overlay caseFiles, defaults caseTexts) caseTexts {
	test.Helper()
	texts := maps.Clone(shared)
	if texts == nil {
		texts = caseTexts{}
	}
	for name, content := range overlay {
		switch {
		case strings.HasSuffix(name, absentSuffix):
			delete(texts, strings.TrimSuffix(name, absentSuffix))
		case strings.HasSuffix(name, overlaySuffix):
			plain := strings.TrimSuffix(name, overlaySuffix)
			text, err := applyDelta(shared[plain], content)
			if err != nil {
				test.Errorf("%s: %v", name, err)
			}
			texts[plain] = text
		case strings.HasSuffix(name, deltaSuffix):
			plain := strings.TrimSuffix(name, deltaSuffix)
			text, err := applyDelta(defaults[plain], content)
			if err != nil {
				test.Errorf("%s: %v", name, err)
			}
			texts[plain] = text
		default:
			texts[name] = content
		}
	}
	return texts
}

// loadViews reads the expected files of the family this platform belongs
// to.
func loadViews(test testing.TB) goldenViews {
	test.Helper()
	views := goldenViews{shared: map[string]caseTexts{}, own: map[string]map[string]caseTexts{}}
	family := familyDirectory()
	if stored, found := readCase(test, caseDirectory(family, "default")); found {
		views.shared["default"] = expandCase(test, stored, nil)
	}
	for _, name := range caseNames(test, family) {
		if stored, found := readCase(test, caseDirectory(family, name)); found && name != "default" {
			views.shared[name] = expandCase(test, stored, views.shared["default"])
		}
	}
	for _, platform := range familyMembers() {
		views.own[platform] = map[string]caseTexts{}
		if overlay, found := readCase(test, caseDirectory(platform, "default")); found {
			views.own[platform]["default"] = applyOverlay(test, views.shared["default"], overlay, nil)
		}
		defaults, _ := views.view(platform, "default")
		for _, name := range caseNames(test, platform) {
			if overlay, _ := readCase(test, caseDirectory(platform, name)); name != "default" {
				views.own[platform][name] = applyOverlay(test, views.shared[name], overlay, defaults)
			}
		}
	}
	return views
}

// makeOverlay returns the overlay that turns the shared case's texts into
// texts, or nil when they are equal. Each file is kept in the shortest of
// its forms that applies back: a delta from shared's file, a delta from
// defaults' (the platform's view of the default case), or whole; defaults
// is nil for a case that keeps no delta from the default.
func makeOverlay(shared, texts, defaults caseTexts) caseFiles {
	if maps.Equal(shared, texts) {
		return nil
	}
	overlay := caseFiles{}
	for name, text := range texts {
		if base, inShared := shared[name]; inShared && base == text {
			continue
		}
		stored, content := name, text
		for _, form := range []struct {
			suffix string
			bases  caseTexts
		}{{overlaySuffix, shared}, {deltaSuffix, defaults}} {
			base, found := form.bases[name]
			if !found {
				continue
			}
			delta := makeDelta(base, text)
			if applied, err := applyDelta(base, delta); err == nil && applied == text && len(delta) < len(content) {
				stored, content = name+form.suffix, delta
			}
		}
		overlay[stored] = content
	}
	for name := range shared {
		if _, written := texts[name]; !written {
			overlay[name+absentSuffix] = ""
		}
	}
	return overlay
}

// decideShared returns the shared texts of a case this platform wrote
// texts for, and this platform's own view where it differs from them: the
// other platform's own view folds into unix/ when it is the same, unix/ is
// kept when it holds the case, so the other platform's view and files stay
// as they are, and otherwise this platform's texts become unix/'s.
func decideShared(old goldenViews, name string, texts caseTexts) (shared, mine caseTexts) {
	other := otherUnixPlatform()
	otherView, otherOwns := old.own[other][name]
	existing, found := old.shared[name]
	switch {
	case otherOwns && maps.Equal(otherView, texts):
		return texts, nil
	case other != "" && found:
		return existing, texts
	}
	return texts, nil
}

// writeShared writes a case's whole texts to familyDirectory, with deltas
// from the default case's shared texts, or removes the case there when it
// has none.
func writeShared(test testing.TB, name string, shared map[string]caseTexts, overMain bool) {
	test.Helper()
	directory := caseDirectory(familyDirectory(), name)
	if shared[name] == nil {
		removeCase(test, directory)
		return
	}
	var defaults caseTexts
	if name != "default" {
		defaults = shared["default"]
	}
	writeCase(test, directory, storedForm(shared[name], defaults, overMain && defaults != nil))
}

// writeOverlay writes platform's overlay of a case on the shared texts,
// given the platform's own views (nil where it reads the shared one), or
// removes the platform's directory of the case when it has no overlay.
func writeOverlay(test testing.TB, platform, name string, shared, views map[string]caseTexts, overMain bool) {
	test.Helper()
	directory := caseDirectory(platform, name)
	var defaults caseTexts
	if overMain {
		defaults = views["default"]
		if defaults == nil {
			defaults = shared["default"]
		}
	}
	overlay := makeOverlay(shared[name], views[name], defaults)
	if views[name] == nil || overlay == nil {
		removeCase(test, directory)
		return
	}
	writeCase(test, directory, overlay)
}

// storeViews rewrites the expected files so that this platform's view of
// each case in got is what it wrote, keeping the other platform's view of
// every case. overMain says which cases may keep deltas from the default.
func storeViews(test testing.TB, got map[string]caseTexts, overMain map[string]bool) {
	test.Helper()
	old := loadViews(test)
	own, other := platformDirectory(), otherUnixPlatform()
	shared := map[string]caseTexts{}
	mine := map[string]caseTexts{}
	for name := range overMain {
		shared[name], mine[name] = old.shared[name], nil
		if texts, ran := got[name]; ran {
			shared[name], mine[name] = decideShared(old, name, texts)
		} else if texts, found := old.own[own][name]; found {
			mine[name] = texts
		}
	}
	otherViews := old.own[other]
	for name := range overMain {
		writeShared(test, name, shared, overMain[name])
		if other == "" {
			continue
		}
		writeOverlay(test, own, name, shared, mine, overMain[name])
		if _, found := otherViews[name]; found {
			writeOverlay(test, other, name, shared, otherViews, overMain[name])
		}
	}
}

func TestAnOverlayOnItsSharedCaseGivesThePlatformsView(t *testing.T) {
	long := strings.Repeat("row\n", 50)
	shared := caseTexts{"a.txt": long + "shared\n", "gone.txt": "x\n", "same.txt": "s\n"}
	defaults := caseTexts{"a.txt": long + "default\n", "b.txt": long + "b\n"}
	cases := []struct {
		name  string
		texts caseTexts
	}{
		{"a changed file", caseTexts{"a.txt": long + "mine\n", "gone.txt": "x\n", "same.txt": "s\n"}},
		{"a file the platform does not write", caseTexts{"a.txt": long + "shared\n", "same.txt": "s\n"}},
		{"a file only the platform writes", caseTexts{"a.txt": long + "shared\n", "gone.txt": "x\n",
			"same.txt": "s\n", "b.txt": long + "b2\n"}},
		{"a file closer to the default's", caseTexts{"a.txt": long + "default\n", "gone.txt": "x\n", "same.txt": "s\n"}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			overlay := makeOverlay(shared, test.texts, defaults)
			if got := applyOverlay(t, shared, overlay, defaults); !maps.Equal(got, test.texts) {
				t.Errorf("overlay %q applied gives %q, want %q", overlay, got, test.texts)
			}
			if _, kept := overlay["same.txt"]; kept {
				t.Errorf("the overlay holds a file the shared case already gives: %q", overlay)
			}
		})
	}
	if overlay := makeOverlay(shared, maps.Clone(shared), defaults); overlay != nil {
		t.Errorf("the overlay of the shared view itself is %q, want none", overlay)
	}
}
