package follow

import (
	"os"
	"slices"
	"strings"
	"testing"
)

// The second check's coverage, kept in step with the guard's counted list
// and with internals.md: every counted call on an image path either opens
// through the opener's Admit or is named, as a whole code span, in the
// sentence of internals.md that lists what the root check alone covers.
// The test checks that the lists agree; which class each call belongs to is
// reviewed by hand when a call is added.
const (
	admitted      = "admitted"
	rootCheckOnly = "root check alone"
	notImage      = "not an image path"
)

// siteClasses classifies every counted call of allowedCalls by file and name.
func siteClasses() map[string]string {
	return map[string]string{
		"internal/glob/expand.go fsmeta.OpenDir":               admitted,
		"internal/projects/files.go fsmeta.OpenDir":            admitted,
		"internal/relocation/walk.go fsmeta.OpenDir":           admitted,
		"internal/walk/tree.go fsmeta.OpenDir":                 admitted,
		"internal/walk/files.go fsmeta.DiskUsageBytes":         admitted,
		"internal/volumes/volumes.go fsmeta.DiskUsageBytes":    admitted,
		"internal/collect/options.go fsmeta.OnDiskPath":        rootCheckOnly,
		"internal/projects/paths.go fsmeta.OnDiskPath":         rootCheckOnly,
		"internal/walk/rootrules_windows.go fsmeta.OnDiskPath": rootCheckOnly,
		"internal/users/users.go osfile.ReadDir":               rootCheckOnly,
		"internal/walk/project.go stage.PhysicalDir":           rootCheckOnly,
		// The operator's own -r and -o, live ProfileList and ProgramData
		// paths, the run's output and staging files, and host programs.
		"internal/collect/options_windows.go fsmeta.OnDiskPath":            notImage,
		"internal/collect/run_windows.go fsmeta.OnDiskPath":                notImage,
		"internal/users/users_windows.go fsmeta.OnDiskPath":                notImage,
		"internal/collect/options.go osfile.Stat":                          notImage,
		"internal/collect/run_unix.go unix.Faccessat":                      notImage,
		"internal/archive/archive.go osfile.Remove":                        notImage,
		"internal/archive/archive.go osfile.OpenFile":                      notImage,
		"internal/collect/run_setup.go osfile.MkdirAll":                    notImage,
		"internal/collect/run_setup.go stage.MarkCaseSensitive":            notImage,
		"internal/collect/probe_windows.go windows.CreateFile":             notImage,
		"internal/analyze/probe_windows.go windows.CreateFile":             notImage,
		"internal/collect/run_setup.go stage.CreateTree":                   notImage,
		"internal/collect/probe_unix.go osfile.OpenRoot":                   notImage,
		"internal/collect/run_finish.go osfile.Remove":                     notImage,
		"internal/collect/run_finish.go osfile.Rename":                     notImage,
		"internal/collect/run.go osfile.Remove":                            notImage,
		"internal/collect/run_finish.go osfile.Open":                       notImage,
		"internal/collect/run_finish.go osfile.OpenFile":                   notImage,
		"internal/runlog/runlog.go osfile.OpenFile":                        notImage,
		"internal/hostcmd/hostcmd.go exec.CommandContext":                  notImage,
		"internal/hostcmd/hostcmd.go exec.LookPath":                        notImage,
		"internal/hostcmd/hostcmd.go exec.Error":                           notImage,
		"internal/shortname/shortname_windows.go windows.GetShortPathName": notImage,
		"internal/shortname/shortname_windows.go windows.GetLongPathName":  notImage,
		"internal/shortname/shortname_windows.go osfile.MkdirAll":          notImage,
		// A database the analysis commands read from a collection they were given.
		"internal/sqlitefile/path.go osfile.Open": notImage,
		// The files and directories a parser reads from that collection.
		"internal/parserkit/files.go osfile.ReadDir": notImage,
		// The analysis commands' input: the analyst's own archive or directory, read
		// with no image root, and the extraction directory it makes.
		"internal/input/input.go osfile.Stat":        notImage,
		"internal/input/input.go osfile.ReadDir":     notImage,
		"internal/input/input.go osfile.RemoveAll":   notImage,
		"internal/input/archive.go osfile.Open":      notImage,
		"internal/input/archive.go osfile.Stat":      notImage,
		"internal/input/archive.go osfile.ReadFile":  notImage,
		"internal/input/archive.go osfile.MkdirAll":  notImage,
		"internal/input/archive.go osfile.MkdirTemp": notImage,
		"internal/input/extract.go osfile.Open":      notImage,
		"internal/input/extract.go osfile.MkdirAll":  notImage,
		"internal/input/extract.go osfile.OpenFile":  notImage,
		"internal/input/extract.go osfile.Remove":    notImage,
		"internal/input/extract.go osfile.Chtimes":   notImage,
		"internal/input/loose.go stage.PhysicalDir":  notImage,
		// The analysis commands' outputs, where the analyst names them.
		"internal/analyze/fleet.go osfile.MkdirAll":      notImage,
		"internal/analyze/fleet.go osfile.Rename":        notImage,
		"internal/analyze/fleet.go osfile.Remove":        notImage,
		"internal/analyze/timeline.go osfile.ReadDir":    notImage,
		"internal/analyze/output.go osfile.OpenFile":     notImage,
		"internal/analyze/timeline.go osfile.Remove":     notImage,
		"internal/analyze/probe_unix.go osfile.OpenRoot": notImage,
		"internal/analyze/timeline.go osfile.MkdirAll":   notImage,
		// The analyst's output path, placed and followed to keep the output
		// out of the input directory, which is known by the entry its read
		// reached; on Windows the input is read, and the work directory
		// made, at the place the same call gives.
		"internal/analyze/outside_unix.go filepath.EvalSymlinks": notImage,
		"internal/input/paths_windows.go filepath.Abs":           notImage,
	}
}

// ownRootCheckOnly are this package's own following calls that open no
// handle the run keeps, or run only where there is no second check (ReadFile
// reads the Unix /etc/passwd), named in the same sentence of internals.md.
var ownRootCheckOnly = []string{"follow.Stat", "IsDirectory", "IsRegularFile", "follow.ReadFile",
	"follow.PhysicalDir", "CanEnter"}

func TestEveryImageSiteIsAdmittedOrDocumented(t *testing.T) {
	classes := siteClasses()
	for key := range allowedCalls() {
		if _, known := classes[key]; !known {
			t.Errorf("%s is counted but not classified in siteClasses: is it admitted, covered "+
				"by the root check alone (and named in internals.md), or not an image path?", key)
		}
	}
	allowed := allowedCalls()
	for key := range classes {
		if _, counted := allowed[key]; !counted {
			t.Errorf("%s is classified but no longer counted", key)
		}
	}
	text, err := os.ReadFile("../../docs/internals.md")
	if err != nil {
		t.Fatal(err)
	}
	// A Windows checkout may end its lines with CRLF.
	_, sentence, found := strings.Cut(strings.ReplaceAll(string(text), "\r\n", "\n"),
		"The root\ncheck alone covers")
	if !found {
		t.Fatal(`internals.md has no "The root check alone covers" sentence`)
	}
	sentence, _, _ = strings.Cut(sentence, "\n\n")
	// Each name must appear as a whole code span: the call itself, its last
	// name alone, or for osfile, which wraps package os, the os name.
	named := func(spellings ...string) bool {
		return slices.ContainsFunc(spellings, func(spelling string) bool {
			return strings.Contains(sentence, "`"+spelling+"`")
		})
	}
	for _, name := range ownRootCheckOnly {
		if !named(name) {
			t.Errorf("internals.md's root-check-alone sentence does not name %s", name)
		}
	}
	for key, class := range classes {
		if class != rootCheckOnly {
			continue
		}
		call := key[strings.LastIndexByte(key, ' ')+1:]
		last := call[strings.IndexByte(call, '.')+1:]
		if !named(call, last, "os."+strings.TrimPrefix(call, "osfile.")) {
			t.Errorf("internals.md's root-check-alone sentence does not name %s (%s)", call, key)
		}
	}
}
