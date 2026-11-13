// Package relocation finds the startup files that name an environment
// variable able to move an agent's files (a relocation hint), reads the value
// a line assigns where the file's syntax needs no shell, and records each hint
// with the log lines and collection.json entries the sh collector writes.
// collectors/docs/relocation.md and the "Relocation hints" section of
// collectors/docs/internals.md describe the step. Collecting a hit file and
// listing the catalog's matches are the caller's, through Deps.
package relocation

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/users"
)

// Hint is one relocation hint: a line of Source naming Variable, a variable
// of Agent's, with the path the line assigns in Value when it was resolved.
// Line is 0 for a value plutil read from a whole plist.
type Hint struct {
	User     string
	Source   string
	Line     int
	Variable string
	Agent    string
	Value    string
}

// Value is a resolved value with the home-relative path it stands in for, the
// third field of the variable's RELOCATION_VARS line.
type Value struct {
	User     string
	Variable string
	Agent    string
	Replaces string
	Value    string
}

// Result is what a search found: the hints in the order they were recorded,
// the resolved values, the number of files searched, the number of files the
// sweep left unread for their size, and the number of hints with a value.
type Result struct {
	Hints            []Hint
	Values           []Value
	FilesSearched    int
	FilesSkippedSize int
	Resolved         int
}

// Logger writes one line of the run log; runlog.Log is the one in use.
type Logger interface {
	Line(text string) error
}

// Deps is what a search reads and the work it hands back to the caller.
// Homes and Projects are the run's user list and project list (a project's
// User is its owner, possibly empty); Root is the image root, "" in live mode.
// Excludes is the run's active EXCLUDES list, empty under --full. OutDir and
// RunName name the run's own output files, which the sweep never reads.
// Claims returns the paths the catalog table matches under a base, as the walk
// expands them; Collect collects a hit file as agent shared, flagged secret.
type Deps struct {
	Tables          *catalog.Tables
	Root            string
	Homes           []users.Home
	Projects        []users.Home
	EnvScanHome     bool
	EnvScanMaxBytes int64
	Excludes        []string
	OutDir          string
	RunName         string
	Plutil          PlistReader
	Log             Logger
	Claims          func(base string, table []catalog.Entry) []string
	Collect         func(user, home, path string) error
}

// search is the state of one Search: the hint records already held, the
// header waiting for the first hint of a base, and the result so far.
type search struct {
	deps   Deps
	names  []string
	seen   map[string]bool
	header string
	result Result
}

// Search searches the startup files of every home, of the system and of every
// project, then, under --env-scan-home, sweeps every other file of each home
// and project, logging and recording each hint and collecting each hit file
// through deps.Collect. It stops with ctx's error when ctx is cancelled.
func Search(ctx context.Context, deps Deps) (Result, error) {
	run := &search{deps: deps, names: variableNames(deps.Tables), seen: map[string]bool{}}
	err := run.startupPass(ctx)
	if err == nil && deps.EnvScanHome {
		err = run.sweepPass(ctx)
	}
	return run.result, err
}

// variableNames returns each RELOCATION_VARS variable once, in table order.
func variableNames(tables *catalog.Tables) []string {
	var names []string
	seen := map[string]bool{}
	for _, line := range tables.RelocationVars {
		if !seen[line.Variable] {
			seen[line.Variable] = true
			names = append(names, line.Variable)
		}
	}
	return names
}

// startupPass searches the startup files of each home, the system and each
// project, under the header naming the base.
func (run *search) startupPass(ctx context.Context) error {
	for _, home := range run.deps.Homes {
		if home.Path == "" {
			continue
		}
		run.header = fmt.Sprintf("Relocation hints for %s (%s)", home.User, home.Path)
		if err := run.searchStartup(ctx, home.User, home.Path, home.Path, "home"); err != nil {
			return err
		}
	}
	run.header = "Relocation hints in system files"
	if err := run.searchStartup(ctx, "", "", run.deps.Root, "system"); err != nil {
		return err
	}
	for _, project := range run.deps.Projects {
		if project.Path == "" {
			continue
		}
		run.header = "Relocation hints in project " + project.Path
		err := run.searchStartup(ctx, project.User, project.Path, project.Path, "project")
		if err != nil {
			return err
		}
	}
	return nil
}

// searchStartup searches the scope's startup files under base for the hints
// of user and home (both empty for the system).
func (run *search) searchStartup(ctx context.Context, user, home, base, scope string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	files := ListStartupFiles(base, scope, run.deps.Tables.StartupFiles)
	return run.searchFiles(ctx, user, home, base, files)
}

// searchFiles counts files as searched and records the hints of each, the
// files that are not PowerShell first and then the PowerShell ones, the order
// in which the sh collector's search worker lists them. A swept file under a
// directory whose name holds a newline is counted but not recorded: the sh
// worker's one-path-per-line output splits its name, so no hint names it.
func (run *search) searchFiles(ctx context.Context, user, home, base string, files []string) error {
	run.result.FilesSearched += len(files)
	var others, scripts []string
	for _, file := range files {
		if IsPowerShell(file) {
			scripts = append(scripts, file)
		} else {
			others = append(others, file)
		}
	}
	prefix := strings.TrimSuffix(base, "/") + "/"
	for _, file := range append(others, scripts...) {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !strings.HasPrefix(file, prefix) || strings.Contains(file, "\n") {
			continue
		}
		if err := run.recordFileHints(user, home, file); err != nil {
			return err
		}
	}
	return nil
}

// recordFileHints records every name the file holds as a word, with the value
// its line assigns where Kind reads the file, then a hint on line 0 for each
// value plutil read from a plist, and has the file collected. A system plist
// under LaunchDaemons gives its hints the plist's UserName as their user.
func (run *search) recordFileHints(user, home, path string) error {
	hits := run.boundedNamesOf(path)
	if len(hits) == 0 {
		return nil
	}
	hintUser, values := user, FileValues{}
	if kind := Kind(path); kind != "" {
		values = run.resolve(path, kind, run.resolveHome(user, home), hits)
		if user == "" && home == "" && values.User != "" && isDaemonPath(path) {
			hintUser = values.User
		}
	}
	for _, hit := range hits {
		value, reason := values.selectValue(hit.Line, hit.Name, run.deps.Root)
		if err := run.record(hintUser, path, hit.Line, hit.Name, value, reason); err != nil {
			return err
		}
	}
	for _, assignment := range values.Assignments {
		if assignment.Line != 0 {
			continue
		}
		value, reason := values.selectValue(0, assignment.Name, run.deps.Root)
		if err := run.record(hintUser, path, 0, assignment.Name, value, reason); err != nil {
			return err
		}
	}
	return run.deps.Collect(user, home, path)
}

// boundedNamesOf returns the hits of the file at path; a file that cannot be
// read has none, as grep lists no file it cannot open.
func (run *search) boundedNamesOf(path string) []LineName {
	file, err := openNoFollow(path)
	if err != nil {
		return nil
	}
	defer file.Close()
	hits, _ := BoundedNames(file, run.names, IsPowerShell(path))
	return hits
}

// resolveHome returns what `$HOME` stands for in the file's environment.d
// syntax: the home, less the image root, when user and home are a pair of the
// run's user list, else "".
func (run *search) resolveHome(user, home string) string {
	if home == "" {
		return ""
	}
	for _, candidate := range run.deps.Homes {
		if candidate.User == user && candidate.Path == home {
			return strings.TrimPrefix(home, run.deps.Root)
		}
	}
	return ""
}

// resolve reads the values of a hit file of a kind: a value reader that cannot
// open the file reads nothing.
func (run *search) resolve(path, kind, home string, hits []LineName) FileValues {
	if kind == "launchd" {
		return ResolvePlist(run.deps.Plutil, path, hits)
	}
	file, err := openNoFollow(path)
	if err != nil {
		return FileValues{}
	}
	defer file.Close()
	var assignments []Assignment
	if kind == "environment" {
		assignments, _ = ResolveEnvironment(file, run.names)
	} else {
		assignments, _ = ResolveEnvironmentD(file, run.names, home)
	}
	return FileValues{Assignments: assignments}
}

// openNoFollow opens a regular file for reading without following a symlink
// in its last component.
func openNoFollow(path string) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_RDONLY|noFollow, 0)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	return file, nil
}
