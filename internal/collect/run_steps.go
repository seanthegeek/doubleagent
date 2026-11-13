package collect

import (
	"bytes"
	"context"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/inventory"
	"github.com/seanthegeek/doubleagent/internal/manifest"
	"github.com/seanthegeek/doubleagent/internal/projects"
	"github.com/seanthegeek/doubleagent/internal/relocation"
	"github.com/seanthegeek/doubleagent/internal/runlog"
	"github.com/seanthegeek/doubleagent/internal/users"
	"github.com/seanthegeek/doubleagent/internal/version"
	"github.com/seanthegeek/doubleagent/internal/volumes"
	"github.com/seanthegeek/doubleagent/internal/walk"
)

// sharedAgent is the agent a relocation hint file is collected under.
const sharedAgent = "shared"

// findHomes lists the homes to collect, as get_user_homes does, with a
// warning when there is none.
func (state *runState) findHomes(context.Context) error {
	live := state.options.Root == ""
	state.homes = users.Enumerate(live, state.options.Root, state.options.UsersFilter,
		state.host.runner.Output)
	if len(state.homes) > 0 {
		return nil
	}
	return state.log.Line("WARNING: no user home directories found")
}

// collectHomes walks the catalog in every home; --inventory walks them later,
// in writeInventory, after project discovery.
func (state *runState) collectHomes(ctx context.Context) error {
	if state.options.Inventory {
		return nil
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
	counts, err := volumes.Collect(ctx, state.collector, state.options.Root, state.homes,
		&state.notes)
	state.docker = counts
	return err
}

// collectProjects discovers the projects unless --no-projects, reports the
// -p values that are not directories and, outside --inventory, collects the
// project catalog in each project, as collect_projects does.
func (state *runState) collectProjects(ctx context.Context) error {
	if state.options.NoProjects {
		return nil
	}
	var dropSecret func(string) bool
	if state.options.Inventory {
		dropSecret = walk.MatchAny(state.tables.SecretGlobs)
	}
	found, err := projects.Discover(ctx, state.homes, state.options.Root, state.options.Projects,
		dropSecret)
	if err != nil {
		return err
	}
	state.projects = found
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

// collectProject logs one project, writes its error_read row when it cannot
// be listed or entered, and collects the project catalog in it.
func (state *runState) collectProject(ctx context.Context, project projects.Project) error {
	header := "Project " + project.Path
	if project.Owner != "" {
		header = "Project [" + project.Owner + "] " + project.Path
	}
	if err := state.log.Line(header); err != nil {
		return err
	}
	if reason, failed := fsmeta.DirReadError(project.Path); failed {
		err := state.collector.WriteReadError(project.Owner, project.Path, project.Path, reason)
		if err != nil {
			return err
		}
	}
	return state.collector.CollectCatalog(ctx, project.Owner, project.Path,
		state.tables.ProjectCatalog, "")
}

// reportMissingProjects logs every -p value that is not a directory and,
// outside --inventory, writes its error_read row with zero metadata, as
// report_missing_projects does.
func (state *runState) reportMissingProjects() error {
	for _, path := range state.options.Projects {
		if path == "" || isDirectory(path) {
			continue
		}
		reason, failed := fsmeta.DirReadError(path)
		if !failed {
			reason = ""
		}
		line := "WARNING: project given with -p is not a directory: " + path + ": " + reason
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

// projectOwner returns the user of the first home that holds path, as
// get_project_owner does, or "".
func projectOwner(homes []users.Home, path string) string {
	for _, home := range homes {
		if strings.HasPrefix(path, home.Path+"/") {
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
		projectHomes = append(projectHomes, users.Home{User: project.Owner, Path: project.Path})
	}
	result, err := relocation.Search(ctx, relocation.Deps{
		Tables: state.tables, Root: state.options.Root, Homes: state.homes,
		Projects: projectHomes, EnvScanHome: state.options.EnvScanHome,
		EnvScanMaxBytes: state.options.EnvScanMaxBytes, Excludes: state.collector.Excludes,
		OutDir: state.outDir, RunName: state.name, Plutil: state.plutil, Log: state.log,
		Claims: catalogClaims, Collect: state.hintFileCollector(ctx),
	})
	state.relocation = result
	return err
}

// catalogClaims returns the paths table matches under base as the walk
// expands them, case variants folded, which the sweep leaves to the walk.
func catalogClaims(base string, table []catalog.Entry) []string {
	var claims []string
	for _, match := range walk.ExpandCatalog(base, table) {
		if match.Note == "" {
			claims = append(claims, match.Path)
		}
	}
	return claims
}

// hintFileCollector returns the relocation step's collect callback, as
// collect_hint_file does: a file that already has a row is left alone; a
// file reached through symlinked directories inside the base gets their rows
// and is staged where the base holds it; the file is flagged secret.
func (state *runState) hintFileCollector(ctx context.Context) func(user, home, path string) error {
	return func(user, home, path string) error {
		if state.manifest.HasPath(path) {
			return nil
		}
		base := home
		if base == "" {
			base = state.options.Root
		}
		stager := state.collector.Stager
		if links, outside := walk.LinkedParent(base, path); !outside {
			stager = stager.Through(links.From, links.To)
			if len(links.Passed) > 0 {
				err := state.collector.RecordPassedLinks(ctx, user, base, home, sharedAgent,
					links.Passed)
				if err != nil {
					return err
				}
			}
		}
		return state.collector.StageFile(user, home, sharedAgent, stager, path, true)
	}
}

// writeInventory is the --inventory step, as write_inventory does: it counts
// the homes, walks each one's catalog, then prints the host line, the agent
// lines of every user and those of the Docker volumes.
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
	header := "User " + home.User + " (" + home.Path + ")"
	err := state.collector.CollectCatalog(ctx, home.User, home.Path, state.tables.Catalog, header)
	if err != nil {
		return err
	}
	owned := 0
	for _, project := range state.projects {
		if project.Owner == home.User && project.Path != "" {
			owned++
		}
	}
	return records.WriteAgentLines(out, state.hostname, home.User, owned)
}

// countInventoryHomes counts the homes and those the run cannot both read and
// enter, logging a warning for each of those, as count_inventory_homes does.
func (state *runState) countInventoryHomes() (scanned, unreadable int, err error) {
	for _, home := range state.homes {
		if home.Path == "" {
			continue
		}
		scanned++
		if canAccess(home.Path, accessRead) && canAccess(home.Path, accessEnter) {
			continue
		}
		unreadable++
		line := "WARNING: home of " + home.User + " is not readable: " + home.Path
		if err := state.log.Line(line); err != nil {
			return 0, 0, err
		}
	}
	return scanned, unreadable, nil
}
