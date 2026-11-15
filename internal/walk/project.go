package walk

import (
	"context"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/follow"
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/glob"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/stage"
	"github.com/seanthegeek/doubleagent/internal/syspath"
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
	if reason, failed, _ := follow.DirReadError(collector.check(), project); failed {
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
	links, outside := LinkedParent(project, path, NewRootCheck(stager.Root).Expansion())
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
	roots := NewRootCheck(stager.Root)
	scope := roots.scope(project, imageProbes())
	if scope.name != "" {
		return place{outOfRoot: scope.name, unverified: scope.unverified, notUnder: scope.outside,
			stager: stager}
	}
	if home == "" {
		return place{stager: stager}
	}
	links, outside := LinkedParent(home, project+"/", roots.Expansion())
	if !outside {
		return place{links: links, stager: stager.Through(links.From, links.Into)}
	}
	found := place{links: links, outside: true, stager: stager}
	if physical, err := follow.PhysicalDir(roots.Expansion(), project); err == nil {
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
// that cannot be stat'ed, a link that cannot be read, a chain of more links
// than maxLinkHops, a root that does not resolve) counts as outside.
func ProjectOutOfRoot(root, project string) (string, bool) {
	found := projectScope(root, project, imageProbes())
	return found.name, found.name != ""
}

// ProjectLeavesRoot reports whether project, under the image root root (""
// on a live host), lies outside the root as given, leads out of it through
// a link, climbs above it with `..`, or sits under a root that cannot be
// resolved, so that whether it is a directory is the analyst's host's answer
// rather than the image's. A link step that fails (a target that cannot be
// read, more links than maxLinkHops) counts too, since the host may resolve
// what the check could not. Only a component inside the root that is missing
// or cannot be stat'ed does not count: that answer is the image's own.
func ProjectLeavesRoot(root, project string) bool {
	_, leaves := LeavesRootAt(root, project)
	return leaves
}

// LeavesRootAt reports what ProjectLeavesRoot reports for path, any path
// under the image root root, and the path that decides it, spelled under the
// root as given: the first symlinked directory (or, on Windows, junction) of
// path, path itself included, whose target leads out of the root, or the
// link whose target climbs above the root with `..`. Project discovery asks
// it before reading a source.
func LeavesRootAt(root, path string) (string, bool) {
	return NewRootCheck(root).Exit(path)
}

// RootCheck answers LeavesRootAt's question for one image root, resolved
// once, so a caller that asks of many paths does not resolve the root each
// time. The zero RootCheck, a live run's, has no root to leave.
type RootCheck struct {
	given    string
	physical string
	failed   bool
}

// NewRootCheck resolves root, "" for a live run, for a RootCheck.
func NewRootCheck(root string) RootCheck {
	if root == "" {
		return RootCheck{}
	}
	physical, err := stage.PhysicalDir(root)
	return RootCheck{given: root, physical: physical, failed: err != nil}
}

// Exit is LeavesRootAt for path under check's root.
func (check RootCheck) Exit(path string) (string, bool) {
	found := check.scope(path, imageProbes())
	if found.name == "" || found.inImage {
		return "", false
	}
	return found.name, true
}

// Expansion returns the check the glob expander asks before it lists a
// directory or looks a name up in one, nil for a live run.
func (check RootCheck) Expansion() glob.Check {
	if check.given == "" {
		return nil
	}
	return check.Exit
}

// imageProbes returns the probes of the file system itself.
func imageProbes() probes {
	return probes{isLink: isLinkProbe, readlink: osfile.Readlink}
}

// isLinkProbe reports whether path itself is a symlink or, on Windows, any
// reparse point, or the error when it cannot be stat'ed.
func isLinkProbe(path string) (bool, error) {
	meta, err := fsmeta.Lstat(path)
	return err == nil && isLinkMeta(meta), err
}

// maxLinkHops bounds the links a project's path expands through, as the
// kernel bounds a path's links, so a loop ends; a path that reaches it counts
// as leading out of the root.
const maxLinkHops = 40

// probes are the two calls the root check makes on the image: whether a
// path is a link (a junction included), and a link's target.
type probes struct {
	isLink   func(path string) (bool, error)
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

// projectScope decides ProjectOutOfRoot's question with probe.
func projectScope(root, project string, probe probes) rootScope {
	return NewRootCheck(root).scope(project, probe)
}

// scope decides ProjectOutOfRoot's question for check's root with probe. A
// project whose path does not start with the root is outside it and is not
// probed at all.
func (check RootCheck) scope(project string, probe probes) rootScope {
	if check.given == "" {
		return rootScope{}
	}
	if check.failed {
		return rootScope{name: check.given, unverified: true}
	}
	walker := rootWalk{given: check.given, physical: check.physical, probe: probe}
	rest, inside := walker.relative(project)
	if !inside {
		return rootScope{name: project, outside: true}
	}
	return walker.walk(steps(rest, ""))
}

// step is one component of the path the root check walks, with via the link
// whose target it came from ("" for a component of the path as given), so a
// `..` that climbs above the root names that link.
type step struct {
	name string
	via  string
}

// steps returns the components of path, each marked as coming from via.
func steps(path, via string) []step {
	var found []step
	for _, name := range strings.Split(path, "/") {
		found = append(found, step{name: name, via: via})
	}
	return found
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
func (walker rootWalk) walk(components []step) rootScope {
	current, hops := walker.physical, 0
	for len(components) > 0 {
		component := components[0]
		components = components[1:]
		switch component.name {
		case "", ".":
			continue
		case "..":
			// current is a physical path, so its parent is lexical.
			if current == walker.physical {
				return walker.above(component.via)
			}
			current = parentDir(current)
			continue
		}
		candidate := joinPath(current, component.name)
		isLink, err := walker.probe.isLink(candidate)
		if err != nil {
			return walker.unreadable(candidate)
		}
		if !isLink {
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
// that link leads, so the walk takes it one component at a time. A Windows
// target is read with backslashes, so it is made canonical first; one with
// a single leading separator (`\x`) is rooted on the drive or share of the
// link's own directory, as Windows resolves it, and a link whose directory
// names neither cannot be verified.
func (walker rootWalk) follow(parent, link string, hops int) (string, []step, rootScope) {
	if hops > maxLinkHops {
		return "", nil, walker.failedLink(link)
	}
	target, err := walker.probe.readlink(link)
	if err != nil {
		return "", nil, walker.failedLink(link)
	}
	target = syspath.FromNative(target)
	if strings.HasPrefix(target, "/") && !syspath.IsAbs(target) {
		volume := volumeOf(parent)
		if volume == "" {
			return "", nil, walker.failedLink(link)
		}
		target = volume + target
	}
	if !syspath.IsAbs(target) {
		return parent, steps(target, link), rootScope{}
	}
	// The target is compared as text, ignoring ASCII case only: respelling
	// it from the disk would look at paths outside the root, which nothing
	// here may stat. A target that matches the root only through another
	// case of a non-ASCII letter counts as outside and is not walked.
	rest, inside := walker.relative(target)
	if !inside {
		return "", nil, rootScope{name: walker.named(link)}
	}
	return walker.physical, steps(rest, link), rootScope{}
}

// volumeOf returns the drive (`C:`) or UNC share (`//server/share`) a
// canonical Windows path starts with, or "" when it starts with neither.
func volumeOf(path string) string {
	if len(path) >= 2 && path[1] == ':' && isASCIILetter(path[0]) {
		return path[:2]
	}
	server, rest, found := strings.Cut(strings.TrimPrefix(path, "//"), "/")
	if !strings.HasPrefix(path, "//") || !found || server == "" {
		return ""
	}
	share, _, _ := strings.Cut(rest, "/")
	if share == "" {
		return ""
	}
	return "//" + server + "/" + share
}

// isASCIILetter reports whether char is an ASCII letter, a drive letter.
func isASCIILetter(char byte) bool {
	return ('a' <= char && char <= 'z') || ('A' <= char && char <= 'Z')
}

// above is the decision for a `..` that climbs above the resolved root: it
// names via, the link whose target holds it, or the root as given when the
// path itself holds it.
func (walker rootWalk) above(via string) rootScope {
	if via == "" {
		return rootScope{name: walker.given, unverified: true}
	}
	return rootScope{name: walker.named(via), unverified: true}
}

// unreadable is the decision for path, under the resolved root, that a
// probe could not stat: missing or unreadable, which is the image's own
// answer.
func (walker rootWalk) unreadable(path string) rootScope {
	return rootScope{name: walker.named(path), unverified: true, inImage: true}
}

// failedLink is the decision for link, under the resolved root, when its
// step cannot be taken: its target cannot be read or placed, or the path has
// run through more than maxLinkHops links. The operating system may still
// resolve it (Windows allows more reparse points per path than the bound),
// so it counts as leading out of the root.
func (walker rootWalk) failedLink(link string) rootScope {
	return rootScope{name: walker.named(link), unverified: true}
}

// relative returns name less the resolved root or the root as given (which
// resolves to it), and whether name lies under either. The given root loses
// only a trailing slash: cleaning it would turn a UNC root //server/share
// into /server/share, which nothing lies under.
func (walker rootWalk) relative(name string) (string, bool) {
	given := walker.given
	if len(given) > 1 {
		given = strings.TrimSuffix(given, "/")
	}
	for _, root := range []string{walker.physical, given} {
		if rest, inside := pathUnder(name, root); inside {
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
// it, compared lexically, ignoring case on Windows.
func pathUnder(path, dir string) (string, bool) {
	if syspath.Equal(path, dir) {
		return "", true
	}
	if !strings.HasSuffix(dir, "/") {
		dir += "/"
	}
	return syspath.CutPrefix(path, dir)
}

// parentDir returns the directory holding path, a physical path under the
// resolved root: everything before its last slash, or / for a path directly
// under /.
func parentDir(path string) string {
	position := strings.LastIndexByte(path, '/')
	if position <= 0 {
		return "/"
	}
	return path[:position]
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
// from it as from any path under the root; a path outside the root, and
// every path on a live run (no root, where the physical path keeps its
// drive), is returned unchanged.
func underRoot(root, physical string) string {
	if root == "" {
		return physical
	}
	physicalRoot, err := stage.PhysicalDir(root)
	if err != nil {
		return physical
	}
	if physicalRoot == "/" {
		return root + physical
	}
	if rest, found := stage.Under(physical, physicalRoot); found && rest != "" {
		return root + "/" + rest
	}
	return physical
}

// projectLine returns the log line that opens a project, with the words a
// catalog match's line ends in when its path runs through a symlinked
// directory: the deepest inside its home, or the first leading out of it.
func projectLine(user, project string, found place) string {
	native := syspath.ToNative
	line := "Project " + native(project)
	if user != "" {
		line = "Project [" + user + "] " + native(project)
	}
	switch {
	case found.notUnder:
		return line + ": is outside the image root; not walked"
	case found.unverified:
		return line + ": under " + native(found.outOfRoot) +
			", which cannot be verified to lie inside the image root; not walked"
	case found.outOfRoot != "":
		return line + ": under the symlinked directory " + native(found.outOfRoot) +
			", which leads out of the image root; not walked"
	case found.outside && found.physical != "":
		return line + " (through the symlinked directory " + native(found.links.Outside) +
			", archived under " + native(found.physical) + ")"
	case !found.outside && found.links.From != "":
		return line + " (through the symlinked directory " + native(found.links.From) +
			", archived under " +
			native(physicalPath(project, found.links.From, found.links.Into)) + ")"
	}
	return line
}
