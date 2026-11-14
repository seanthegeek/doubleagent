package walk

import (
	"context"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/stage"
)

// CollectProject collects the project catalog in project for user, as
// collect_projects does for one project; home is the first home of any user
// that holds the project, or "". A project the run lists, discovered from an
// agent's state or given with -p, is a target in its own right: the links in
// its own path are resolved to find where its files physically are, never to
// decide whether to collect it. The home walk's "leads out of the base; not
// walked" rule is for catalog matches, not projects, but scope outranks it:
// under an image root a project whose path leads out of the root is not
// walked, as it resolves on the analyst's own host. The sh collector stages a
// project's files under their recorded path instead: a deviation.
func (collector *Collector) CollectProject(ctx context.Context, user, home, project string) error {
	// The links of the path inside home, the project itself included, get
	// their rows; one leading out of home gets none. The files keep their
	// recorded path and are staged where they physically are, never through
	// a link recreated in the staging tree.
	place := projectPlace(collector.Stager, home, project)
	if err := collector.Log.Line(projectLine(user, project, place)); err != nil {
		return err
	}
	if place.outOfRoot != "" {
		return nil
	}
	if reason, failed := fsmeta.DirReadError(project); failed {
		if err := collector.WriteReadError(user, project, project, reason); err != nil {
			return err
		}
	}
	if len(place.links.Passed) > 0 && !collector.Inventory {
		owner := scope{user: user, home: home, agent: sharedAgent, stager: collector.Stager}
		err := collector.recordPassedLinks(ctx, owner, home, place.links.Passed,
			map[string]struct{}{})
		if err != nil {
			return err
		}
	}
	return collector.collectCatalog(ctx, user, project, collector.Tables.ProjectCatalog, "",
		place.stager)
}

// PathStagers are the stagers of a file under a base: Base stages the
// symlinked directories in Passed, those between the base and the file that
// stay inside it, and File stages the file where it physically is.
// Skip is set for a project CollectProject does not walk, whose files are
// not staged either.
type PathStagers struct {
	Base   stage.Stager
	File   stage.Stager
	Passed []string
	Skip   bool
}

// ProjectStagers returns the stagers of path, a file under project, a project
// the run lists, with home the first home of any user that holds it, or "".
// The links of the project's own path are resolved as CollectProject resolves
// them, and those between project and path inside the project are composed on
// top. A link between project and path leading out of the project leaves path
// staged at its project-level place, as linked_parent leaves such a file.
func ProjectStagers(stager stage.Stager, home, project, path string) PathStagers {
	found := projectPlace(stager, home, project)
	if found.outOfRoot != "" {
		return PathStagers{Skip: true}
	}
	base := found.stager
	links, outside := LinkedParent(project, path)
	if outside {
		return PathStagers{Base: base, File: base}
	}
	return PathStagers{Base: base, File: throughLinks(base, links.From, links.Into),
		Passed: links.Passed}
}

// place is where a project's files physically are: the links of its path
// inside its home, the first one leading out of it (in links.Outside, with
// physical the project's resolved path), and the stager of its files.
// outOfRoot is the symlinked directory by which the path leads out of the
// image root, when it does.
type place struct {
	outOfRoot string
	links     Links
	outside   bool
	physical  string
	stager    stage.Stager
}

// projectPlace resolves the links of project's own path, its last component
// included, against home. Links inside home stage the files where home holds
// them; past a link leading out of home, the project's resolved path does.
// Without a home, or when the path cannot be resolved, the files are staged
// under their recorded path, since no home's walk recreated a link on it.
func projectPlace(stager stage.Stager, home, project string) place {
	if link, out := ProjectOutOfRoot(stager.Root, project); out {
		return place{outOfRoot: link, stager: stager}
	}
	if home == "" {
		return place{stager: stager}
	}
	links, outside := LinkedParent(home, project+"/")
	if !outside {
		return place{links: links, stager: stager.Through(links.From, links.Into)}
	}
	found := place{links: links, outside: true, stager: stager}
	if physical, err := stage.PhysicalDir(project); err == nil {
		found.physical = underRoot(stager.Root, physical)
		found.stager = stager.Through(project, found.physical)
	}
	return found
}

// ProjectOutOfRoot reports whether project, under the image root root ("" on
// a live host, where nothing is out of it), resolves to a place outside the
// root, and returns the first symlinked directory of its path, the project
// itself included, that leads there. Such a target resolves on the analyst's
// own host, so following it would collect the analyst's files as evidence.
func ProjectOutOfRoot(root, project string) (string, bool) {
	if root == "" {
		return "", false
	}
	physical, err := stage.PhysicalDir(project)
	if err != nil {
		return "", false
	}
	physicalRoot, err := stage.PhysicalDir(root)
	if err != nil || physical == physicalRoot || strings.HasPrefix(physical, physicalRoot+"/") {
		return "", false
	}
	if links, _ := LinkedParent(root, project+"/"); links.Outside != "" {
		return links.Outside, true
	}
	return project, true
}

// underRoot returns physical, a resolved path, with the resolved image root
// it starts with replaced by root as given, so the archive path is taken
// from it as from any path under the root; a path outside the root is
// returned unchanged.
func underRoot(root, physical string) string {
	physicalRoot, err := stage.PhysicalDir(baseOrRoot(root))
	if err != nil {
		return physical
	}
	if physicalRoot == "/" {
		return root + physical
	}
	if rest, found := strings.CutPrefix(physical, physicalRoot+"/"); found {
		return root + "/" + rest
	}
	return physical
}

// projectLine returns the log line that opens a project, with the words a
// catalog match's line ends in when its path runs through a symlinked
// directory: the deepest inside its home, or the first leading out of it.
func projectLine(user, project string, found place) string {
	line := "Project " + project
	if user != "" {
		line = "Project [" + user + "] " + project
	}
	switch {
	case found.outOfRoot != "":
		return line + ": under the symlinked directory " + found.outOfRoot +
			", which leads out of the image root; not walked"
	case found.outside && found.physical != "":
		return line + " (through the symlinked directory " + found.links.Outside +
			", archived under " + found.physical + ")"
	case !found.outside && found.links.From != "":
		return line + " (through the symlinked directory " + found.links.From + ", archived under " +
			physicalPath(project, found.links.From, found.links.Into) + ")"
	}
	return line
}
