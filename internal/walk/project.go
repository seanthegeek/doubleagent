package walk

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
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
// under an image root a project whose path leads out of the root, or cannot
// be verified inside it, is not walked (ProjectOutOfRoot). The sh collector
// stages a project's files under their recorded path instead: a deviation.
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
// image root, the path that cannot be verified inside it (unverified), or
// the project itself when it is not under the root as given (notUnder).
type place struct {
	outOfRoot  string
	unverified bool
	notUnder   bool
	links      Links
	outside    bool
	physical   string
	stager     stage.Stager
}

// projectPlace resolves the links of project's own path, its last component
// included, against home. Links inside home stage the files where home holds
// them; past a link leading out of home, the project's resolved path does.
// Without a home, or when the path cannot be resolved, the files are staged
// under their recorded path, since no home's walk recreated a link on it.
// Whether the project is out of the image root is decided first, as
// ProjectOutOfRoot decides it, before anything else of its path is stat'ed.
func projectPlace(stager stage.Stager, home, project string) place {
	scope := projectScope(stager.Root, project, imageProbes())
	if scope.name != "" {
		return place{outOfRoot: scope.name, unverified: scope.unverified, notUnder: scope.outside,
			stager: stager}
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
// a live host, where nothing is out of it), may lie outside the root, and
// returns the path that decides it: the first symlinked directory of its
// path, the project itself included, whose target leads out of the root, the
// first that cannot be verified, or the project itself when it is not under
// the root as given. Such a target resolves on the analyst's
// own host. The links are read one by one from the resolved root and nothing
// outside it is ever stat'ed; a path that cannot be verified (a component
// that cannot be stat'ed, a link loop, a root that does not resolve) counts
// as outside.
func ProjectOutOfRoot(root, project string) (string, bool) {
	found := projectScope(root, project, imageProbes())
	return found.name, found.name != ""
}

// ProjectLeavesRoot reports whether project, under the image root root (""
// on a live host), lies outside the root as given or leads out of it, so
// that whether it is a directory is the analyst's host's answer rather than
// the image's. A project ProjectOutOfRoot flags only because a path inside
// the root cannot be read (missing, unreadable, a link loop) does not count:
// that answer is the image's own, whatever the host holds.
func ProjectLeavesRoot(root, project string) bool {
	found := projectScope(root, project, imageProbes())
	return found.name != "" && !found.inImage
}

// imageProbes returns the probes of the file system itself.
func imageProbes() probes {
	return probes{lstat: os.Lstat, readlink: os.Readlink}
}

// maxLinkHops bounds the links a project's path expands through, as the
// kernel bounds a path's links, so a loop ends as unverifiable.
const maxLinkHops = 40

// probes are the two calls the root check makes on the image.
type probes struct {
	lstat    func(path string) (fs.FileInfo, error)
	readlink func(path string) (string, error)
}

// rootScope is ProjectOutOfRoot's decision: name is "" for a project inside
// the root, else the path that decides it, spelled under the root as given;
// unverified is set when that path cannot be verified rather than leads out,
// and inImage as well when the reason is a probe inside the root that
// failed. outside is set for a project not under the root as given, which
// names itself.
type rootScope struct {
	name       string
	unverified bool
	inImage    bool
	outside    bool
}

// projectScope decides ProjectOutOfRoot's question with probe. A project
// whose path does not start with the root is outside it and is not probed
// at all.
func projectScope(root, project string, probe probes) rootScope {
	if root == "" {
		return rootScope{}
	}
	physicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return rootScope{name: root, unverified: true}
	}
	walker := rootWalk{given: root, physical: physicalRoot, probe: probe}
	rest, inside := walker.relative(project)
	if !inside {
		return rootScope{name: project, outside: true}
	}
	return walker.walk(strings.Split(rest, "/"))
}

// rootWalk walks a path's components from the resolved image root physical,
// the root as the run gives it in given.
type rootWalk struct {
	given    string
	physical string
	probe    probes
}

// walk follows components from the resolved root, expanding each link whose
// target stays inside the root into the target's own components, so every
// path it probes is the resolved root or under it.
func (walker rootWalk) walk(components []string) rootScope {
	current, hops := walker.physical, 0
	for len(components) > 0 {
		component := components[0]
		components = components[1:]
		switch component {
		case "", ".":
			continue
		case "..":
			// current is a physical path, so its parent is lexical.
			if current == walker.physical {
				return rootScope{name: walker.given, unverified: true}
			}
			current = filepath.Dir(current)
			continue
		}
		candidate := joinPath(current, component)
		info, err := walker.probe.lstat(candidate)
		if err != nil {
			return walker.unreadable(candidate)
		}
		if info.Mode()&fs.ModeSymlink == 0 {
			current = candidate
			continue
		}
		hops++
		start, target, verdict := walker.follow(current, candidate, hops)
		if verdict.name != "" {
			return verdict
		}
		components = append(target, components...)
		current = start
	}
	return rootScope{}
}

// follow reads link, a symlink in the physical directory parent, and returns
// where its target's walk starts (parent for a relative target, the resolved
// root for an absolute one) and the target's components, or the decision
// when the target leads out of the root or the link cannot be verified. The
// target is never cleaned: a ".." after a link inside it goes up from where
// that link leads, so the walk takes it one component at a time.
func (walker rootWalk) follow(parent, link string, hops int) (string, []string, rootScope) {
	if hops > maxLinkHops {
		return "", nil, walker.unreadable(link)
	}
	target, err := walker.probe.readlink(link)
	if err != nil {
		return "", nil, walker.unreadable(link)
	}
	if !filepath.IsAbs(target) {
		return parent, strings.Split(target, "/"), rootScope{}
	}
	rest, inside := walker.relative(target)
	if !inside {
		return "", nil, rootScope{name: walker.named(link)}
	}
	return walker.physical, strings.Split(rest, "/"), rootScope{}
}

// unreadable is the decision for path, under the resolved root, that a
// probe could not read or a link loop runs through.
func (walker rootWalk) unreadable(path string) rootScope {
	return rootScope{name: walker.named(path), unverified: true, inImage: true}
}

// relative returns path less the resolved root or the root as given (which
// resolves to it), and whether path lies under either.
func (walker rootWalk) relative(path string) (string, bool) {
	for _, root := range []string{walker.physical, filepath.Clean(walker.given)} {
		if rest, inside := pathUnder(path, root); inside {
			return rest, true
		}
	}
	return "", false
}

// named returns path, under the resolved root, spelled under the root as
// given, as the project's own path spells it.
func (walker rootWalk) named(path string) string {
	rest, _ := pathUnder(path, walker.physical)
	if rest == "" {
		return walker.given
	}
	return joinPath(walker.given, rest)
}

// pathUnder returns path less dir and true when path is dir or lies under
// it, compared lexically.
func pathUnder(path, dir string) (string, bool) {
	if path == dir {
		return "", true
	}
	if !strings.HasSuffix(dir, "/") {
		dir += "/"
	}
	return strings.CutPrefix(path, dir)
}

// joinPath returns name joined to dir with one slash.
func joinPath(dir, name string) string {
	if strings.HasSuffix(dir, "/") {
		return dir + name
	}
	return dir + "/" + name
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
	case found.notUnder:
		return line + ": is outside the image root; not walked"
	case found.unverified:
		return line + ": under " + found.outOfRoot +
			", which cannot be verified to lie inside the image root; not walked"
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
