package relocation

import (
	"context"
	"fmt"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/glob"
	"github.com/seanthegeek/doubleagent/internal/syspath"
	"github.com/seanthegeek/doubleagent/internal/users"
)

// homeRoots returns the home parents the image mode lists, as sh case
// patterns over a home less the image root; a home elsewhere (a daemon
// account's) is walked by the catalog but not swept.
func homeRoots() []string {
	return []string{"/home/*", "/Users/*", "/root", "/var/root", "/usr/home/*", "/export/home/*"}
}

// sweepPass sweeps every home, then every project, each under its header.
func (run *search) sweepPass(ctx context.Context) error {
	for _, home := range run.deps.Homes {
		if err := run.sweepHome(ctx, home); err != nil {
			return err
		}
	}
	for _, project := range run.deps.Projects {
		if err := run.sweepProject(ctx, project); err != nil {
			return err
		}
	}
	return nil
}

// sweepHome sweeps a home with the catalog's matches left out, or logs that
// it lies outside the home roots and is not swept.
func (run *search) sweepHome(ctx context.Context, home users.Home) error {
	if home.Path == "" {
		return nil
	}
	run.header = homeHeader(home)
	if !run.underHomeRoot(home.Path) {
		return run.logSkip("sweep skipped: home " + syspath.ToNative(home.Path) +
			" lies outside the user home roots")
	}
	return run.sweep(ctx, home.User, home.Path, run.deps.Tables.Catalog, "home")
}

// sweepProject sweeps a project with the project catalog's matches left out,
// or logs that it holds a home and is not swept.
func (run *search) sweepProject(ctx context.Context, project users.Home) error {
	if project.Path == "" {
		return nil
	}
	run.header = projectHeader(project)
	if run.holdsHome(project.Path) {
		return run.logSkip("sweep skipped: project " + syspath.ToNative(project.Path) +
			" holds a home directory")
	}
	return run.sweep(ctx, project.User, project.Path, run.deps.Tables.ProjectCatalog, "project")
}

// logSkip logs a skipped sweep. The base's header stays unwritten, as it
// heads hints only.
func (run *search) logSkip(text string) error {
	if err := run.deps.Log.Line(text); err != nil {
		return fmt.Errorf("log skipped sweep: %w", err)
	}
	return nil
}

// underHomeRoot reports whether home, less the system scope's base (the
// image root, or the system drive on a live Windows host), lies under one of
// the home roots, compared as the platform compares paths: Windows ignores
// case, as Test-UserHomeRoot's -match does.
func (run *search) underHomeRoot(home string) bool {
	relative, _ := syspath.CutPrefix(home, run.system)
	for _, pattern := range homeRoots() {
		if glob.MatchPath(syspath.Fold(pattern), syspath.Fold(relative)) {
			return true
		}
	}
	return false
}

// holdsHome reports whether a home of the user list is base or lies under it:
// a project at / or /home, which a sweep would read whole.
func (run *search) holdsHome(base string) bool {
	prefix := strings.TrimSuffix(base, "/") + "/"
	for _, home := range run.deps.Homes {
		if syspath.Equal(home.Path, base) || syspath.HasPrefix(home.Path, prefix) {
			return true
		}
	}
	return false
}

// sweep searches every regular file under base that the sweep's prune rules
// leave and that is within the size limit, counting the others as skipped
// for their size.
func (run *search) sweep(ctx context.Context, user, base string, table []catalog.Entry,
	scope string) error {
	walk := sweepWalk{ctx: ctx, opener: run.deps.Opener, pruned: run.pruneTest(base, table, scope),
		limit: run.deps.EnvScanMaxBytes}
	if err := walk.visit(base); err != nil {
		return err
	}
	run.result.FilesSkippedSize += walk.skipped
	return run.searchFiles(ctx, user, base, base, walk.files)
}

// pruneTest returns the test of a path the sweep leaves out with what is
// under it: the run's staging directory and output files, a match of the
// base's catalog table, a startup file the first pass searched, another home
// or project (swept on its own) and an active exclusion under the base. Paths
// and patterns are compared as the platform compares paths, through
// syspath.Fold.
func (run *search) pruneTest(base string, table []catalog.Entry, scope string) func(string) bool {
	literal := map[string]bool{}
	for _, claim := range run.deps.Claims(base, table) {
		for _, line := range claimLines(claim) {
			literal[syspath.Fold(line)] = true
		}
	}
	// The startup pass logged any link these globs stop at.
	files, _ := ListStartupFiles(base, scope, run.deps.Tables.StartupFiles, run.deps.Expansion,
		run.deps.Opener)
	for _, file := range files {
		literal[syspath.Fold(file)] = true
	}
	for _, other := range append(append([]string{}, pathsOf(run.deps.Homes)...),
		pathsOf(run.deps.Projects)...) {
		if !syspath.Equal(other, base) {
			literal[syspath.Fold(other)] = true
		}
	}
	patterns := run.prunePatterns(base)
	return func(path string) bool {
		folded := syspath.Fold(path)
		if literal[folded] {
			return true
		}
		for _, pattern := range patterns {
			if glob.MatchPath(pattern, folded) {
				return true
			}
		}
		return false
	}
}

// prunePatterns returns the glob patterns of the paths a sweep of base leaves
// out, folded: the run's staging directory and output files, and each active
// exclusion under base.
func (run *search) prunePatterns(base string) []string {
	outDir := glob.QuoteMeta(syspath.Fold(run.deps.OutDir))
	patterns := []string{outDir + "/.stage-" + syspath.Fold(run.deps.RunName),
		outDir + "/" + syspath.Fold(run.deps.RunName) + ".*"}
	for _, exclude := range run.deps.Excludes {
		if exclude != "" {
			patterns = append(patterns, glob.QuoteMeta(syspath.Fold(base))+"/"+syspath.Fold(exclude))
		}
	}
	return patterns
}
