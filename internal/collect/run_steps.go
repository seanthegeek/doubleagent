package collect

import (
	"bytes"
	"context"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/follow"
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/inventory"
	"github.com/seanthegeek/doubleagent/internal/manifest"
	"github.com/seanthegeek/doubleagent/internal/projects"
	"github.com/seanthegeek/doubleagent/internal/relocation"
	"github.com/seanthegeek/doubleagent/internal/runlog"
	"github.com/seanthegeek/doubleagent/internal/syspath"
	"github.com/seanthegeek/doubleagent/internal/users"
	"github.com/seanthegeek/doubleagent/internal/version"
	"github.com/seanthegeek/doubleagent/internal/volumes"
	"github.com/seanthegeek/doubleagent/internal/walk"
)

// sharedAgent is the agent a relocation hint file is collected under.
const sharedAgent = "shared"

// findHomes lists the homes to collect, with a warning when there is none
// and a log line for each candidate that could not be seen as a directory.
// It keeps the
// ProfileList entries for the registry pass. A cancelled ctx stops the run
// before the report is used: getent and dscl are not run, or are stopped.
func (state *runState) findHomes(ctx context.Context) error {
	live := state.options.Root == ""
	report := state.host.findUsers(ctx, live, state.options.Root, state.options.UsersFilter,
		state.host.runner.Output, state.roots.Expansion(), state.opener, state.roots.Reached)
	if err := ctx.Err(); err != nil {
		return err
	}
	for _, refusal := range report.OutOfRoot {
		reason := follow.Reason(refusal,
			" leads out of the image root; nothing behind it was listed or taken as a home")
		line := "user homes: " + syspath.ToNative(refusal.Link) + reason
		if err := state.log.Line(line); err != nil {
			return err
		}
	}
	state.homes, state.skippedHomes = report.Homes, report.Skipped
	for _, merge := range report.Merged {
		line := "user homes: " + syspath.ToNative(merge.Path) + " is the home of " + merge.Kept +
			" and of " + merge.Dropped + "; walked once, as " + merge.Kept + "'s"
		if err := state.log.Line(line); err != nil {
			return err
		}
	}
	state.profiles = report.Profiles
	if len(state.homes) == 0 {
		if err := state.log.Line("WARNING: no user home directories found"); err != nil {
			return err
		}
	}
	for _, skipped := range state.skippedHomes {
		line := "profile not accessible (skipped): " + syspath.ToNative(skipped.Path)
		if err := state.log.Line(line); err != nil {
			return err
		}
	}
	return state.logUnmatchedUsers(report)
}

// reachedProjects returns the -p paths, each holding a `.` or `..` component
// replaced by the path the system reaches for it (walk.RootCheck.Reached),
// each path once, first spelling first: two -p values reaching one entry
// (syspath.Entries; on Windows also two spellings the file system calls one
// directory) are one project, warned about and recorded once.
func reachedProjects(projects []string, roots walk.RootCheck) []string {
	var kept []string
	seen := syspath.NewEntries[struct{}](follow.SameEntry(roots.Expansion()))
	for _, project := range projects {
		project = roots.Reached(project)
		if seen.Add(project, struct{}{}) {
			kept = append(kept, project)
		}
	}
	return kept
}

// logUnmatchedUsers writes a warning for each -u name that matched no user
// home, with the users whose homes were found, so a mistyped name does not
// pass for a user without files, and one for a -u list that names nobody
// (",").
func (state *runState) logUnmatchedUsers(report users.Report) error {
	filter := state.options.UsersFilter
	if filter != "" && strings.Trim(filter, ",") == "" {
		return state.log.Line("WARNING: -u lists no names")
	}
	found := "none"
	if len(report.Found) > 0 {
		found = strings.Join(report.Found, ", ")
	}
	for _, name := range report.Unmatched {
		line := "WARNING: -u " + name + " matches no user home; homes found for: " + found
		if err := state.log.Line(line); err != nil {
			return err
		}
	}
	return nil
}

// collectHomes writes the error_read row of each home its parent lists but
// the run cannot see, then walks the catalog in every
// home; --inventory walks them later, in writeInventory, after project
// discovery.
func (state *runState) collectHomes(ctx context.Context) error {
	if state.options.Inventory {
		return nil
	}
	for _, skipped := range state.skippedHomes {
		if !skipped.Denied {
			continue
		}
		leaf := skipped.Path[strings.LastIndexByte(skipped.Path, '/')+1:]
		err := state.collector.WriteReadError(leaf, skipped.Path, skipped.Path, skipped.Reason)
		if err != nil {
			return err
		}
	}
	return state.collector.CollectUserHomes(ctx, state.homes)
}

// collectVolumes collects the Docker and Podman volumes unless --no-docker;
// under --inventory their files go to the run's Docker records.
func (state *runState) collectVolumes(ctx context.Context) error {
	if state.options.NoDocker {
		return nil
	}
	state.collector.Records = state.dockerRecords
	counts, err := volumes.Collect(ctx, state.collector, systemBases(state.options.Root),
		state.homes, &state.notes)
	state.docker = counts
	return err
}

// collectProjects discovers the projects unless --no-projects, reports the
// -p values that are not directories and, outside --inventory, collects the
// project catalog in each project.
func (state *runState) collectProjects(ctx context.Context) error {
	if state.options.NoProjects {
		return nil
	}
	var dropSecret func(home, relative string) bool
	if state.options.Inventory {
		dropSecret = state.collector.IsSecret
	}
	// Live, there is no image root for a path to lead out of.
	var leavesRoot projects.RootExit
	if state.options.Root != "" {
		leavesRoot = state.rootExit
	}
	state.options.Projects = reachedProjects(state.options.Projects, state.roots)
	found, refused, err := projects.Discover(ctx, state.homes, state.options.Root,
		state.options.Projects, dropSecret, leavesRoot, state.opener, state.roots.Reached,
		state.output().Holds)
	if err != nil {
		return err
	}
	state.projects = found
	for _, refusal := range refused {
		if err := state.log.Line(refusedSourceLine(refusal)); err != nil {
			return err
		}
	}
	if err := state.reportMissingProjects(); err != nil || state.options.Inventory {
		return err
	}
	for _, project := range found {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := state.collectProject(ctx, project); err != nil {
			return err
		}
	}
	return nil
}

// collectProject collects one project, with the
// first home of any user that holds it as the base its path's links are
// resolved in.
func (state *runState) collectProject(ctx context.Context, project projects.Project) error {
	home := enclosingHome(state.homes, project.Path, follow.SameEntry(state.roots.Expansion()))
	return state.collector.CollectProject(ctx, project.Owner, home, project.Path)
}

// enclosingHome returns the first home of any user that holds path, or ""
// when it lies under none: the home whose walk may have recorded and
// recreated a link of path's, whoever owns the project. A home spelling
// path's start byte for byte is chosen first; one spelled otherwise only in
// ASCII case holds path only where same says so, since the wrong home would
// stage the project's files through another home's links.
func enclosingHome(homes []users.Home, path string, same syspath.SameEntry) string {
	for _, home := range homes {
		if home.Path != "" && strings.HasPrefix(path, home.Path+"/") {
			return home.Path
		}
	}
	for _, home := range homes {
		if _, inside := syspath.CutUnder(path, home.Path, same); home.Path != "" && inside {
			return home.Path
		}
	}
	return ""
}

// projectLeavesRoot reports whether path, a project, lies outside the image
// root or leads out of it (walk.ProjectLeavesRoot): whether it is a
// directory is then the analyst's host's answer, so it is never asked, and
// the project step logs the project as not walked instead.
func (state *runState) projectLeavesRoot(path string) bool {
	_, leaves := state.roots.Exit(path)
	return leaves
}

// rootExit is the run's root check, resolved once, for project discovery.
func (state *runState) rootExit(path string) (follow.Refusal, bool) {
	return state.roots.Exit(path)
}

// refusedSourceLine is the log line of a link out of the image root on the
// path of one or more discovery sources of a home: nothing behind it was
// listed, stat'ed or read. It says nothing of whether any source exists
// behind it, which only the analyst's host could answer.
func refusedSourceLine(refusal projects.Refusal) string {
	return "Discovery [" + refusal.User + "] " + syspath.ToNative(refusal.Link) +
		follow.Reason(refusal.Refusal,
			": the symlink leads out of the image root; nothing behind it was listed or read")
}

// projectNotWalked reports whether the project step leaves path, a project,
// unwalked: out of the image root or not verifiable inside it.
func (state *runState) projectNotWalked(path string) bool {
	_, out := state.roots.OutOfRoot(path)
	return out
}

// reportMissingProjects logs every -p value that is not a directory and,
// outside --inventory, writes its error_read row with zero metadata. A value that leaves the image root is
// skipped untested, whatever the analyst's host holds there.
func (state *runState) reportMissingProjects() error {
	check := state.roots.Expansion()
	for _, path := range state.options.Projects {
		if path == "" || state.projectLeavesRoot(path) {
			continue
		}
		if isDir, err := follow.IsDirectory(check, path); isDir || err != nil {
			continue
		}
		// projectLeavesRoot above passed path, so this is never a refusal.
		reason, failed, err := follow.DirReadError(check, state.opener, path)
		if follow.IsOpenedOutside(err) {
			continue
		}
		if !failed {
			reason = ""
		}
		line := "WARNING: project given with -p is not a directory: " + syspath.ToNative(path) +
			": " + reason
		if err := state.log.Line(line); err != nil {
			return err
		}
		if state.options.Inventory {
			continue
		}
		row := manifest.Row{User: projectOwner(state.homes, path), Home: path, Path: path,
			Type: "dir", Status: manifest.ErrorRead, Error: reason}
		if err := state.manifest.Write(row); err != nil {
			return err
		}
	}
	return nil
}

// projectOwner returns the user of the first home that holds path, or "": a home spelling path's start byte for byte
// first, then on Windows the first matching it ignoring ASCII case, since the
// owner only labels a row and a wrong one loses nothing.
func projectOwner(homes []users.Home, path string) string {
	for _, home := range homes {
		if strings.HasPrefix(path, home.Path+"/") {
			return home.User
		}
	}
	for _, home := range homes {
		if syspath.HasPrefix(path, home.Path+"/") {
			return home.User
		}
	}
	return ""
}

// searchRelocation searches the startup files for relocation hints and
// collects each file that names one, outside --inventory.
func (state *runState) searchRelocation(ctx context.Context) error {
	if state.options.Inventory {
		return nil
	}
	projectHomes := make([]users.Home, 0, len(state.projects))
	for _, project := range state.projects {
		// A project the project step leaves unwalked, out of the image root or
		// not verifiable inside it, is not searched either: its files may be
		// on the analyst's host.
		if !state.projectNotWalked(project.Path) {
			projectHomes = append(projectHomes, users.Home{User: project.Owner, Path: project.Path})
		}
	}
	result, err := relocation.Search(ctx, relocation.Deps{
		Tables: state.tables, Root: state.options.Root, Homes: state.homes,
		Projects: projectHomes, EnvScanHome: state.options.EnvScanHome,
		EnvScanMaxBytes: state.options.EnvScanMaxBytes, Excluded: state.collector.Excluded,
		RunName: state.name, Plutil: state.plutil, Log: state.log,
		Claims: state.catalogClaims, Collect: state.hintFileCollector(ctx),
		Registry: registryReader(), SystemBase: syspath.SystemBase(state.options.Root),
		Profiles: state.profiles, Opener: state.opener, Expansion: state.roots.Expansion(),
	})
	state.relocation = result
	return err
}

// catalogClaims returns the paths table matches under base as the walk
// expands them, each path once and nothing behind a link out of the
// image root looked at, which the sweep leaves to the walk.
func (state *runState) catalogClaims(base string, table []catalog.Entry) []string {
	var claims []string
	for _, match := range walk.ExpandCatalog(base, table, state.roots.Expansion(), state.opener) {
		if match.OutOfRoot.Link == "" {
			claims = append(claims, match.Path)
		}
	}
	return claims
}

// hintFileCollector returns the relocation step's collect callback: a file that already has a row is left alone; a
// file reached through symlinked directories inside the base gets their rows
// and is staged where the base holds it; the file is flagged secret. A
// project's files are staged where the project physically is, as the project
// step stages them. The row takes before, the file's lstat from before the
// search read it.
func (state *runState) hintFileCollector(ctx context.Context,
) func(user, home, path string, before fsmeta.Meta) error {
	return func(user, home, path string, before fsmeta.Meta) error {
		if state.manifest.HasPath(path) {
			return nil
		}
		base := home
		if base == "" {
			base = state.options.Root
		}
		stagers := state.hintFileStagers(base, path)
		if stagers.Skip {
			return nil
		}
		if len(stagers.Passed) > 0 {
			err := state.collector.RecordPassedLinks(ctx, user, base, home, sharedAgent,
				stagers.Base, stagers.Passed)
			if err != nil {
				return err
			}
		}
		return state.collector.StageFile(user, home, sharedAgent, stagers.File, path, true, &before)
	}
}

// hintFileStagers returns the stagers of a hint file at path under base: a
// project base's come from the project's own path, any other base's from
// the run's stager and the links between base and path.
func (state *runState) hintFileStagers(base, path string) walk.PathStagers {
	for _, project := range state.projects {
		if project.Path == base {
			home := enclosingHome(state.homes, base, follow.SameEntry(state.roots.Expansion()))
			return walk.ProjectStagers(state.roots, state.collector.Stager, home, base, path)
		}
	}
	stagers := walk.PathStagers{Base: state.collector.Stager, File: state.collector.Stager}
	if links, outside := walk.LinkedParent(base, path, state.roots.Expansion()); !outside {
		stagers.File = stagers.Base.Through(links.From, links.Into)
		stagers.Passed = links.Passed
	}
	return stagers
}

// writeInventory is the --inventory step: it counts
// the homes, walks each one's catalog, then prints the host line, the agent
// lines of every user and those of the Docker volumes. A stop request before
// the first line returns ctx's error and prints nothing; after it a stop
// request no longer stops the inventory.
func (state *runState) writeInventory(ctx context.Context) error {
	scanned, unreadable, err := state.countInventoryHomes()
	if err != nil {
		return err
	}
	var agentLines bytes.Buffer
	for _, home := range state.homes {
		if home.Path == "" {
			continue
		}
		if err := state.inventoryHome(ctx, home, &agentLines); err != nil {
			return err
		}
	}
	host := inventory.Host{Host: state.hostname, Collector: version.String(), Mode: state.mode(),
		Timestamp: state.started.Format(runlog.TimeFormat), UsersScanned: scanned,
		UsersUnreadable: unreadable, DockerVolumes: state.docker.Found}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := inventory.WriteHostLine(state.stdout, host); err != nil {
		return err
	}
	if _, err := state.stdout.Write(agentLines.Bytes()); err != nil {
		return err
	}
	return state.dockerRecords.WriteAgentLines(state.stdout, state.hostname, "docker", 0)
}

// inventoryHome walks one home's catalog into a fresh aggregator and writes
// its agent lines to out, with the number of projects the user owns.
func (state *runState) inventoryHome(ctx context.Context, home users.Home,
	out *bytes.Buffer,
) error {
	records := inventory.NewAggregator()
	state.collector.Records = records
	header := "User " + home.User + " (" + syspath.ToNative(home.Path) + ")"
	err := state.collector.CollectCatalog(ctx, home.User, home.Path, state.tables.Catalog, header)
	if err != nil {
		return err
	}
	// A project that leaves the image root was not walked and is not counted,
	// as the summary leaves it out of its project list.
	owned := 0
	for _, project := range state.projects {
		if project.Owner == home.User && project.Path != "" && !state.projectNotWalked(project.Path) {
			owned++
		}
	}
	return records.WriteAgentLines(out, state.hostname, home.User, owned)
}

// countInventoryHomes counts the homes and those the run cannot both list and
// enter (fsmeta.DirReadError), logging a warning for each of those.
func (state *runState) countInventoryHomes() (scanned, unreadable int, err error) {
	for _, home := range state.homes {
		if home.Path == "" {
			continue
		}
		scanned++
		// The users step kept no home that leads out of the root.
		_, failed, err := follow.DirReadError(state.roots.Expansion(), state.opener, home.Path)
		if !failed || follow.IsOpenedOutside(err) {
			continue
		}
		unreadable++
		line := "WARNING: home of " + home.User + " is not readable: " + syspath.ToNative(home.Path)
		if err := state.log.Line(line); err != nil {
			return 0, 0, err
		}
	}
	return scanned, unreadable, nil
}
