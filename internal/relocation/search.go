package relocation

import (
	"context"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/follow"
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/glob"
	"github.com/seanthegeek/doubleagent/internal/syspath"
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
// sweep left unread for their size, the number of hints with a value, and what
// the registry pass read.
type Result struct {
	Hints            []Hint
	Values           []Value
	FilesSearched    int
	FilesSkippedSize int
	Resolved         int
	Registry         RegistryOutcome
}

// Logger writes one line of the run log; runlog.Log is the one in use.
type Logger interface {
	Line(text string) error
}

// Deps is what a search reads and the work it hands back to the caller.
// Homes and Projects are the run's user list and project list (a project's
// User is its owner, possibly empty); Root is the image root, "" in live mode.
// Excluded tests a path under a base against the active EXCLUDES (nil: none);
// RunName names the run's own outputs, passed over wherever they lie.
// Claims returns the paths the catalog table matches under a base, as the walk
// expands them; Collect collects a hit file as agent shared, flagged secret,
// given the file's metadata from before the search first read it.
// Registry reads the Windows registry, nil off Windows; Profiles lists the
// ProfileList entries in registry order, whose hives the registry pass reads.
type Deps struct {
	Tables *catalog.Tables
	Root   string
	// SystemBase is the base of the system scope: Root in image mode, the
	// system drive on a live Windows host, "" on a live Unix one.
	SystemBase      string
	Homes           []users.Home
	Projects        []users.Home
	EnvScanHome     bool
	EnvScanMaxBytes int64
	Excluded        func(base, path string) bool
	RunName         string
	Plutil          PlistReader
	Log             Logger
	Claims          func(base string, table []catalog.Entry) []string
	Collect         func(user, home, path string, before fsmeta.Meta) error
	Registry        Registry
	Profiles        []users.Profile
	// Opener reads every file the search reads, as the run decided (nil
	// asks to hold access times wherever the platform can).
	Opener *fsmeta.Opener
	// Expansion is the image root's check the startup-file globs are expanded
	// through (nil on a live run, which has no root to leave).
	Expansion glob.Check
}

// search is the state of one Search: the hint records already held, the
// links out of the image root already logged, the header waiting for the
// first hint of a base, and the result so far.
type search struct {
	deps      Deps
	names     []string
	seen      map[string]bool
	outOfRoot map[string]bool
	header    string
	result    Result
}

// Search searches the startup files of every home and of the system, reads the
// registry, searches the startup files of every project, then, under
// --env-scan-home, sweeps every other file of each home and project, logging
// and recording each hint and collecting each hit file through deps.Collect.
// It stops with ctx's error when ctx is cancelled.
func Search(ctx context.Context, deps Deps) (Result, error) {
	run := &search{deps: deps, names: variableNames(deps.Tables), seen: map[string]bool{},
		outOfRoot: map[string]bool{}}
	err := run.startupPass(ctx)
	if err == nil && deps.EnvScanHome {
		err = run.sweepPass(ctx)
	}
	return run.result, err
}

// variableNames returns each RELOCATION_VARS variable once, in table order:
// by agent, then variable, for a table catalog.Parse read.
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

// startupPass searches the startup files of each home and the system, reads
// the registry, then searches each project's, under the header naming the base.
func (run *search) startupPass(ctx context.Context) error {
	for _, home := range run.deps.Homes {
		if home.Path == "" {
			continue
		}
		run.header = homeHeader(home)
		if err := run.searchStartup(ctx, home.User, home.Path, home.Path, "home"); err != nil {
			return err
		}
	}
	run.header = "Relocation hints in system files"
	if err := run.searchStartup(ctx, "", "", run.deps.SystemBase, "system"); err != nil {
		return err
	}
	if err := run.registryPass(); err != nil {
		return err
	}
	for _, project := range run.deps.Projects {
		if project.Path == "" {
			continue
		}
		run.header = projectHeader(project)
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
	files, outOfRoot := listStartupFiles(base, scope, run.deps.Tables.StartupFiles,
		run.deps.Expansion, run.deps.Opener, run.output())
	if err := run.logOutOfRoot(outOfRoot); err != nil {
		return err
	}
	return run.searchFiles(ctx, user, home, base, files)
}

// logOutOfRoot logs each path the root check refused a startup-file glob,
// once per run, worded by the refusal's kind, as the skipped sweeps are
// logged, without the base's header, which heads hints only.
func (run *search) logOutOfRoot(refusals []follow.Refusal) error {
	for _, refusal := range refusals {
		if run.outOfRoot[refusal.Link] {
			continue
		}
		run.outOfRoot[refusal.Link] = true
		reason := follow.Reason(refusal, " leads out of the image root; nothing behind it was listed")
		err := run.logSkip("startup files: " + syspath.ToNative(refusal.Link) + reason)
		if err != nil {
			return err
		}
	}
	return nil
}

// searchFiles counts files as searched and records the hints of each, in the
// order given.
func (run *search) searchFiles(ctx context.Context, user, home, base string, files []string) error {
	run.result.FilesSearched += len(files)
	prefix := strings.TrimSuffix(base, "/") + "/"
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !syspath.HasPrefix(file, prefix) {
			continue
		}
		if err := run.recordFileHints(ctx, user, home, file); err != nil {
			return err
		}
	}
	return nil
}

// recordFileHints records every name the file holds as a word, with the value
// its line assigns where Kind reads the file, then a hint on line 0 for each
// value plutil read from a plist, and has the file collected. A system plist
// under LaunchDaemons gives its hints the plist's UserName as their user.
// The file's lstat is taken before it is read, so its row records the access
// time the search's own read may move; a file lstat cannot read cannot be
// read either, and has no hits.
func (run *search) recordFileHints(ctx context.Context, user, home, path string) error {
	before, err := fsmeta.Lstat(path)
	if err != nil {
		return nil
	}
	hits := run.boundedNamesOf(path)
	if len(hits) == 0 {
		return nil
	}
	hintUser, values := user, fileValues{}
	if kind := Kind(path); kind != "" {
		values, err = run.resolve(ctx, path, kind, run.resolveHome(user, home), hits)
		if err != nil {
			return err
		}
		if user == "" && home == "" && values.User != "" && isDaemonPath(path) {
			hintUser = values.User
		}
	}
	index := values.index()
	for _, hit := range hits {
		value, reason := index.selectValue(hit.Line, hit.Name, run.deps.Root)
		if err := run.recordFile(hintUser, path, hit.Line, hit.Name, value, reason); err != nil {
			return err
		}
	}
	for _, assignment := range values.Assignments {
		if assignment.Line != 0 {
			continue
		}
		value, reason := index.selectValue(0, assignment.Name, run.deps.Root)
		if err := run.recordFile(hintUser, path, 0, assignment.Name, value, reason); err != nil {
			return err
		}
	}
	return run.deps.Collect(user, home, path, before)
}

// boundedNamesOf returns the hits of the file at path, a UTF-16 or UTF-32
// text decoded first; a file that cannot be read has none.
func (run *search) boundedNamesOf(path string) []lineName {
	file, err := run.deps.Opener.OpenRegular(path)
	if err != nil {
		return nil
	}
	defer file.Close()
	hits, _ := decodedBoundedNames(file, run.names, isPowerShell(path))
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
// open the file reads nothing. Only a cancelled ctx is an error.
func (run *search) resolve(ctx context.Context, path, kind, home string, hits []lineName,
) (fileValues, error) {
	if kind == "launchd" {
		return resolvePlist(ctx, run.deps.Plutil, path, hits)
	}
	file, err := run.deps.Opener.OpenRegular(path)
	if err != nil {
		return fileValues{}, nil
	}
	defer file.Close()
	var assignments []assignment
	if kind == "environment" {
		assignments, _ = resolveEnvironment(file, run.names)
	} else {
		assignments, _ = resolveEnvironmentD(file, run.names, home)
	}
	return fileValues{Assignments: assignments}, nil
}
