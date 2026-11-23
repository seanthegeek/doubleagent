package imagetest

import (
	"os"
	"slices"
	"strings"
	"testing"
)

// homeSystems maps each home of a committed manifest, a directory directly
// under root/home or root/Users, to the operating system whose paths it
// holds: linux, darwin or windows.
var homeSystems = map[string]map[string]string{
	"testdata/image_unix.txt": {
		"root/home/alice":  "linux",
		"root/home/dave":   "linux",
		"root/home/dot":    "linux",
		"root/home/grace":  "linux",
		"root/home/nl":     "linux",
		"root/home/nl\n":   "linux",
		"root/home/ollama": "linux",
		"root/home/u1":     "linux",
		"root/home/u[1]":   "linux",
		"root/Users/bob":   "darwin",
		"root/Users/heidi": "windows",
		"root/Users/ivan":  "windows",
	},
	"testdata/image_windows.txt": {
		"root/Users/Default": "windows",
		"root/Users/Public":  "windows",
		"root/Users/alice":   "windows",
		"root/Users/dave":    "windows",
		"root/Users/ivan":    "windows",
		"root/Users/judy":    "darwin",
		"root/home/bob":      "linux",
		"root/home/dot":      "linux",
		"root/home/grace":    "linux",
		"root/home/u1":       "linux",
		"root/home/u[1]":     "linux",
	},
}

// foldASCII returns path with its ASCII capitals lowered, the key under
// which a volume that ignores letter case takes two names for one.
func foldASCII(path string) string {
	return strings.Map(func(character rune) rune {
		if character >= 'A' && character <= 'Z' {
			return character + 'a' - 'A'
		}
		return character
	}, path)
}

// caseTwins returns a line for each entry whose path equals an earlier
// entry's once ASCII capitals are lowered.
func caseTwins(entries []Entry) []string {
	seen := map[string]string{}
	var lines []string
	for _, entry := range entries {
		key := foldASCII(entry.Path)
		if earlier, found := seen[key]; found {
			lines = append(lines, earlier+" and "+entry.Path+" differ only in letter case")
			continue
		}
		seen[key] = entry.Path
	}
	return lines
}

// systemOf returns the one system a path below a home belongs to, "" when
// any may hold it: darwin for a Library or Applications directory or an app
// bundle, windows for AppData, the PowerShell profile directories and a
// program ending .exe or .cmd.
func systemOf(below string) string {
	names := strings.Split(below, "/")
	lowered := foldASCII(names[len(names)-1])
	switch {
	case names[0] == "Library" || names[0] == "Applications" ||
		slices.ContainsFunc(names, func(name string) bool { return strings.HasSuffix(name, ".app") }):
		return "darwin"
	case names[0] == "AppData" || strings.HasPrefix(below+"/", "Documents/PowerShell/") ||
		strings.HasPrefix(below+"/", "Documents/WindowsPowerShell/") ||
		strings.HasSuffix(lowered, ".exe") || strings.HasSuffix(lowered, ".cmd"):
		return "windows"
	}
	return ""
}

// misplacedPaths returns a line for each home of entries missing from
// systems, each home of systems missing from entries, and each path below
// a home that belongs to a system other than the home's.
func misplacedPaths(entries []Entry, systems map[string]string) []string {
	var lines []string
	homes := map[string]bool{}
	for _, entry := range entries {
		parts := strings.SplitN(entry.Path, "/", 4)
		if len(parts) < 3 || parts[0] != "root" || (parts[1] != "home" && parts[1] != "Users") {
			continue
		}
		home := strings.Join(parts[:3], "/")
		system, known := systems[home]
		if !homes[home] && !known {
			lines = append(lines, home+" is a home the table gives no system")
		}
		homes[home] = true
		if len(parts) == 4 {
			if wanted := systemOf(parts[3]); wanted != "" && known && wanted != system {
				lines = append(lines, entry.Path+" is a "+wanted+" path in a "+system+" home")
			}
		}
	}
	for home := range systems {
		if !homes[home] {
			lines = append(lines, home+" is in the table but not in the manifest")
		}
	}
	slices.Sort(lines)
	return lines
}

// committedEntries returns the entries of the committed manifest name.
func committedEntries(t *testing.T, name string) []Entry {
	t.Helper()
	file, err := os.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	entries, err := Parse(file)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return entries
}

func TestNoTwoManifestEntriesDifferOnlyInCase(t *testing.T) {
	t.Parallel()
	for name := range homeSystems {
		t.Run(name, func(t *testing.T) {
			for _, line := range caseTwins(committedEntries(t, name)) {
				t.Errorf("%s: %s", name, line)
			}
		})
	}
}

func TestEachHomeHoldsOnlyItsSystemsPaths(t *testing.T) {
	t.Parallel()
	for name, systems := range homeSystems {
		t.Run(name, func(t *testing.T) {
			for _, line := range misplacedPaths(committedEntries(t, name), systems) {
				t.Errorf("%s: %s", name, line)
			}
		})
	}
}

func TestTheLayoutChecksFindEachBreach(t *testing.T) {
	t.Parallel()
	manifest := `"root" dir
"root/Users/mac" dir
"root/Users/mac/AppData/Roaming/x" file ""
"root/Users/mac/Library/Logs/x" file ""
"root/Users/stray" dir
"root/Users/win" dir
"root/Users/win/Documents/PowerShell/profile.ps1" file ""
"root/Users/win/x.app/Contents/x" file ""
"root/home/lin" dir
"root/home/lin/.config/Goose" dir
"root/home/lin/.config/Goose/settings.json" file ""
"root/home/lin/.config/goose" dir
"root/home/lin/.config/goose/config.yaml" file ""
"root/home/lin/.local/bin/tool.EXE" file ""
"root/home/lin/Applications" dir
"root/home/lin/Documents/PowerShellish" file ""
"root/home/lin/notes.cmdline" file ""
"root/srv/Library/x" file ""
`
	entries, err := Parse(strings.NewReader(manifest))
	if err != nil {
		t.Fatal(err)
	}
	systems := map[string]string{"root/Users/mac": "darwin", "root/Users/win": "windows",
		"root/home/lin": "linux", "root/home/gone": "linux"}
	want := []string{
		"root/Users/mac/AppData/Roaming/x is a windows path in a darwin home",
		"root/Users/stray is a home the table gives no system",
		"root/Users/win/x.app/Contents/x is a darwin path in a windows home",
		"root/home/gone is in the table but not in the manifest",
		"root/home/lin/.local/bin/tool.EXE is a windows path in a linux home",
		"root/home/lin/Applications is a darwin path in a linux home",
	}
	if got := misplacedPaths(entries, systems); !slices.Equal(got, want) {
		t.Errorf("misplacedPaths =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	twins := []string{"root/home/lin/.config/Goose and root/home/lin/.config/goose differ only in letter case"}
	sorted := slices.Clone(entries)
	slices.SortFunc(sorted, func(first, second Entry) int { return strings.Compare(first.Path, second.Path) })
	if got := caseTwins(sorted); !slices.Equal(got, twins) {
		t.Errorf("caseTwins = %q, want %q", got, twins)
	}
}
