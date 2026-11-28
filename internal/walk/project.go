package walk

import (
	"context"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/follow"
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/glob"
	"github.com/seanthegeek/doubleagent/internal/stage"
	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// kind is the kind of refusal found records.
func (found rootScope) kind() follow.Kind {
	switch {
	case found.outside:
		return follow.Outside
	case found.climbs:
		return follow.Climbs
	case found.reparse:
		return follow.OtherReparse
	case found.unverified:
		return follow.Unverified
	}
	return follow.LeadsOut
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

// imageProbes returns the probes of the file system itself.
func imageProbes() probes {
	return platformProbes()
}

// place is where a project's files physically are: the links of its path
// inside its home, the first one leading out of it (in links.Outside, with
// physical the project's resolved path), and the stager of its files.
// refusal is the root check's, when the project is not walked: its Link is
// the symlinked directory by which the path leads out of the image root, the
// path that cannot be verified inside it or climbs above it, or the project
// itself when it is not under the root as given.
type place struct {
	refusal  follow.Refusal
	links    Links
	outside  bool
	physical string
	stager   stage.Stager
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
		rest, inside := pathUnder(name, root)
		if !inside {
			continue
		}
		// The part of name that matched the root, without its separator
		// (name may be the root with a trailing slash).
		prefix := strings.TrimSuffix(name[:len(name)-len(rest)], "/")
		if prefix == "" {
			prefix = "/"
		}
		// pathUnder ignores ASCII case on Windows; a prefix spelled
		// otherwise than the root must be the root's own directory, which
		// the disk decides (sameDirectory). One that is not may still lie
		// under the other spelling of the root.
		if prefix == root || walker.sameDirectory(prefix, root) {
			return rest, true
		}
	}
	return "", false
}

// RootCheck answers, for one image root resolved once, whether a path under
// it leads out of it (Exit) or may lie outside it (OutOfRoot). The zero
// RootCheck, a live run's, has no root to leave.
type RootCheck struct {
	given    string
	physical string
	failed   bool
}

// resolve is scope that also returns the physical path the walk reached for
// a project it lets through: where the operating system's open lands, which
// the tests compare with what the system itself reports.
func (check RootCheck) resolve(project string, probe probes) (rootScope, string) {
	if check.given == "" {
		return rootScope{}, ""
	}
	if check.failed {
		return rootScope{name: check.given, unverified: true}, ""
	}
	reached := ""
	walker := rootWalk{given: check.given, physical: check.physical, probe: probe,
		reached: &reached, path: project}
	// The check judges the path the operating system will open: on Windows
	// its `.` and `..` removed by name, as every open removes them.
	rest, inside := walker.relative(syspath.Opened(project))
	if !inside {
		return rootScope{name: project, outside: true}, ""
	}
	found := walker.walkPath(rest)
	return found, reached
}

// scope decides RootCheck.OutOfRoot's question for check's root with probe. A
// project whose path does not start with the root is outside it and is not
// probed at all.
func (check RootCheck) scope(project string, probe probes) rootScope {
	found, _ := check.resolve(project, probe)
	return found
}

// Exit is the root check's decision for path under check's root: the
// refusal and true when the path leads out of the root, cannot be verified
// to lie inside it, climbs above it or is not under it, carrying its kind so
// a log line can word it without walking the path again.
func (check RootCheck) Exit(path string) (follow.Refusal, bool) {
	found := check.scope(path, imageProbes())
	if found.name == "" || found.inImage {
		return follow.Refusal{}, false
	}
	return follow.Refusal{Link: found.name, Kind: found.kind()}, true
}

// Expansion returns the check the glob expander asks before it lists a
// directory or looks a name up in one, nil for a live run.
func (check RootCheck) Expansion() glob.Check {
	if check.given == "" {
		return nil
	}
	return check.Exit
}

// underRoot returns physical, a resolved path, with the resolved image root
// it starts with replaced by the root as given (resolved once, with the
// check), so the archive path is taken
// from it as from any path under the root; a path outside the root, and
// every path on a live run (no root, where the physical path keeps its
// drive), is returned unchanged.
func (check RootCheck) underRoot(physical string) string {
	root := check.given
	if root == "" || check.failed {
		return physical
	}
	// ResolveRoot cleans the root a run gives; a root given here with a
	// trailing separator still joins with one.
	if len(root) > 1 {
		root = strings.TrimSuffix(root, "/")
	}
	if check.physical == "/" {
		return root + physical
	}
	rest, found := stage.Under(physical, check.physical)
	switch {
	case found && rest == "":
		return root
	case found:
		return root + "/" + rest
	}
	return physical
}

// projectPlace resolves the links of project's own path, its last component
// included, against home. Links inside home stage the files where home holds
// them; past a link leading out of home, the project's resolved path does.
// Without a home, or when the path cannot be resolved, the files are staged
// under their recorded path, since no home's walk recreated a link on it.
// Whether the project is out of the image root is decided first, as
// RootCheck.OutOfRoot decides it, before anything else of its path is stat'ed.
func projectPlace(roots RootCheck, stager stage.Stager, home, project string) place {
	scope := roots.scope(project, imageProbes())
	if scope.name != "" {
		return place{refusal: follow.Refusal{Link: scope.name, Kind: scope.kind()}, stager: stager}
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
		found.physical = roots.underRoot(physical)
		found.stager = stager.Through(project, found.physical)
	}
	return found
}

// maxLinkHops bounds the links a project's path expands through, as the
// kernel bounds a path's links, so a loop ends; a path that reaches it counts
// as leading out of the root.
const maxLinkHops = 40

// rootScope is RootCheck.OutOfRoot's decision: name is "" for a project inside
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
	climbs     bool
	reparse    bool
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
	refusal := found.refusal
	switch {
	case refusal.Kind == follow.Outside:
		return line + ": is outside the image root; not walked"
	case refusal.Kind == follow.LeadsOut:
		return line + ": under the symlinked directory " + native(refusal.Link) +
			", which leads out of the image root; not walked"
	case refusal.Link != "":
		return line + ": " + native(refusal.Link) + follow.Reason(refusal, "") + "; not walked"
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

// CollectProject collects the project catalog in project for user; home is the first home of any user
// that holds the project, or "". A project the run lists, discovered from an
// agent's state or given with -p, is a target in its own right: the links in
// its own path are resolved to find where its files physically are, never to
// decide whether to collect it. The home walk's "leads out of the base; not
// walked" rule is for catalog matches, not projects, but scope outranks it:
// under an image root a project whose path leads out of the root, or cannot
// be verified inside it, is not walked (RootCheck.OutOfRoot). Exception 15
// says how this staging differs from staging under the recorded path.
func (collector *Collector) CollectProject(ctx context.Context, user, home, project string) error {
	// The links of the path inside home, the project itself included, get
	// their rows; one leading out of home gets none. The files keep their
	// recorded path and are staged where they physically are, never through
	// a link recreated in the staging tree.
	place := projectPlace(collector.RootCheck, collector.Stager, home, project)
	if err := collector.Log.Line(projectLine(user, project, place)); err != nil {
		return err
	}
	if place.refusal.Link != "" {
		return nil
	}
	// place.refusal returned above, so the project is inside the root.
	reason, failed, err := follow.DirReadError(collector.check(), collector.Stager.Opener, project)
	if follow.IsOpenedOutside(err) {
		return nil
	}
	if failed {
		if err := collector.WriteReadError(user, project, project, reason); err != nil {
			return err
		}
	}
	if len(place.links.Passed) > 0 && !collector.Inventory {
		owner := scope{user: user, home: home, agent: sharedAgent, stager: collector.Stager}
		err := collector.recordPassedLinks(ctx, owner, home, place.links.Passed,
			collector.pathSet(), nil)
		if err != nil {
			return err
		}
	}
	return collector.collectCatalog(ctx, user, project, collector.Tables.ProjectCatalog, "",
		place.stager)
}

// rootWalk walks a path's components from the resolved image root physical,
// the root as the run gives it in given.
type rootWalk struct {
	given    string
	physical string
	probe    probes
	reached  *string
	path     string
}

// ProjectStagers returns the stagers of path, a file under project, a project
// the run lists, with home the first home of any user that holds it, or "".
// The links of the project's own path are resolved as CollectProject resolves
// them, and those between project and path inside the project are composed on
// top. A link between project and path leading out of the project leaves path
// staged at its project-level place.
// roots is the run's root check (NewRootCheck of stager.Root).
func ProjectStagers(roots RootCheck, stager stage.Stager, home, project, path string,
) PathStagers {
	found := projectPlace(roots, stager, home, project)
	if found.refusal.Link != "" {
		return PathStagers{Skip: true}
	}
	base := found.stager
	links, outside := LinkedParent(project, path, roots.Expansion())
	if outside {
		return PathStagers{Base: base, File: base}
	}
	return PathStagers{Base: base, File: throughLinks(base, links.From, links.Into),
		Passed: links.Passed}
}

// OutOfRoot reports whether project, under check's root, may lie outside
// it, and returns the path that decides it: the first symlinked directory of
// its path, the project itself included, whose target leads out of the root,
// the first that cannot be verified (a component that cannot be stat'ed, a
// link that cannot be read, more links than maxLinkHops, a root that does
// not resolve), or the project itself when it is not under the root as
// given. The links are read one by one from the resolved root and nothing
// outside it is ever stat'ed.
func (check RootCheck) OutOfRoot(project string) (string, bool) {
	found := check.scope(project, imageProbes())
	return found.name, found.name != ""
}

// NewRootCheck resolves root, "" for a live run, for a RootCheck.
func NewRootCheck(root string) RootCheck {
	if root == "" {
		return RootCheck{}
	}
	physical, err := stage.PhysicalDir(root)
	return RootCheck{given: root, physical: physical, failed: err != nil}
}

// isLinkProbe reports whether path itself is a symlink or, on Windows, any
// reparse point, or the error when it cannot be stat'ed.
func isLinkProbe(path string) (bool, error) {
	meta, err := fsmeta.Lstat(path)
	return err == nil && meta.IsLink(), err
}

// isASCIILetter reports whether char is an ASCII letter, a drive letter.
func isASCIILetter(char byte) bool {
	return ('a' <= char && char <= 'z') || ('A' <= char && char <= 'Z')
}

// joinPath returns name joined to dir with one slash.
func joinPath(dir, name string) string {
	if strings.HasSuffix(dir, "/") {
		return dir + name
	}
	return dir + "/" + name
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
	meta, err := fsmeta.Lstat(link)
	reparse := err == nil && meta.IsOtherReparsePoint()
	return rootScope{name: walker.named(link), unverified: true, reparse: reparse}
}
