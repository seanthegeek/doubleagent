package walk

import (
	"errors"
	"io/fs"
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/syspath"
	"golang.org/x/sys/windows"
)

// walkPath walks rest, a path relative to the resolved root that
// syspath.Opened has already rid of `.` and `..`, as Windows opens it, which
// the tests measure: a symbolic link's relative target is joined to the path
// as named up to the link, a junction's name included, and its `..` removed
// by name, and the open starts again with that name; a junction is looked
// through without its target taking the place of its name.
func (walker rootWalk) walkPath(rest string) rootScope {
	hops := 0
	found, reached := walker.walkNames(rest, &hops)
	if found.name == "" && walker.reached != nil {
		*walker.reached = reached
	}
	return found
}

// walkNames walks name, relative to the resolved root, and returns the
// decision and the physical path reached when it lets name through. hops
// counts the links followed across restarts and junction targets.
func (walker rootWalk) walkNames(name string, hops *int) (rootScope, string) {
	for {
		names := slices.DeleteFunc(strings.Split(name, "/"), func(part string) bool {
			return part == ""
		})
		found, reached, next := walker.walkOnce(names, hops)
		if next == "" {
			return found, reached
		}
		name = next
	}
}

// walkOnce walks names from the resolved root once. It returns the decision
// and the physical path reached, or, at a symbolic link, the name Windows
// opens next, relative to the root.
func (walker rootWalk) walkOnce(names []string, hops *int) (rootScope, string, string) {
	physical, lexical := walker.physical, walker.physical
	for i, component := range names {
		candidate := joinPath(physical, component)
		isLink, err := walker.probe.isLink(candidate)
		if err != nil && stopAllows(err, i < len(names)-1) {
			return walker.unreadable(candidate), "", ""
		}
		if err != nil {
			return walker.failedLink(candidate), "", ""
		}
		if !isLink {
			physical, lexical = candidate, joinPath(lexical, component)
			continue
		}
		*hops++
		target, junction, verdict := walker.linkTarget(physical, candidate, *hops)
		if verdict.name != "" {
			return verdict, "", ""
		}
		if junction {
			found, through := walker.throughJunction(target, candidate, hops)
			if found.name != "" {
				return found, "", ""
			}
			physical, lexical = through, joinPath(lexical, component)
			continue
		}
		next, verdict := walker.renamed(lexical, target, candidate, names[i+1:])
		return verdict, "", next
	}
	return rootScope{}, physical, ""
}

// linkTarget reads the reparse data of link, a link in the physical
// directory parent, and returns its target in canonical form (a `\x` target
// rooted on parent's drive or share) and whether it is a junction, or the
// decision when the step cannot be taken. Only a target acceptLink
// recognises is followed; every other is refused as unverified, so a form
// nobody thought of is never taken for a relative name.
func (walker rootWalk) linkTarget(parent, link string, hops int) (string, bool, rootScope) {
	if hops > maxLinkHops {
		return "", false, walker.failedLink(link)
	}
	data, err := walker.probe.reparse(link)
	if err != nil {
		return "", false, walker.failedLink(link)
	}
	form, ok := acceptLink(data)
	if !ok {
		return "", false, walker.failedLink(link)
	}
	target := form.target
	if form.kind == rootedTarget {
		volume := volumeOf(parent)
		if volume == "" {
			return "", false, walker.failedLink(link)
		}
		target = volume + target
	}
	return target, data.Tag == fsmeta.TagMountPoint, rootScope{}
}

// throughJunction walks the absolute target of the junction link, which
// acceptLink has checked holds no `.` or `..`, as a path of its own and
// returns where it physically leads.
func (walker rootWalk) throughJunction(target, link string, hops *int) (rootScope, string) {
	rest, inside := walker.relative(target)
	if !inside {
		return rootScope{name: walker.named(link)}, ""
	}
	return walker.walkNames(rest, hops)
}

// renamed returns the name Windows opens after the symbolic link link: its
// target, joined when relative to lexical, the path as named up to the link,
// then the names after the link, with `..` removed by name; or the decision
// when that name leads out of the root.
func (walker rootWalk) renamed(lexical, target, link string, after []string) (string, rootScope) {
	built := target
	if !syspath.IsAbs(target) {
		built = joinPath(lexical, target)
	}
	if len(after) > 0 {
		built = joinPath(built, strings.Join(after, "/"))
	}
	rest, inside := walker.relative(syspath.Opened(built))
	if !inside {
		return "", rootScope{name: walker.named(link)}
	}
	return rest, rootScope{}
}

// stopAllows reports whether a component the root check could not lstat
// ends the walk as the image's own answer. One that does not exist, or whose
// name no file can have (a `*`), does: the path has no `..` left, so every
// open of it fails. Any other failure (access refused) does only when
// nothing follows, since traverse rights may still let an open pass through
// a directory whose own attributes could not be read.
func stopAllows(err error, more bool) bool {
	return errors.Is(err, fs.ErrNotExist) || errors.Is(err, windows.ERROR_INVALID_NAME) || !more
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

// probes are the two calls the root check makes on the image: whether a
// path is a link (a junction included), and the reparse data a link stores.
type probes struct {
	isLink  func(path string) (bool, error)
	reparse func(path string) (fsmeta.Reparse, error)
	respell func(path string) string
}

// platformProbes returns the probes of the file system itself on Windows:
// links are told apart and read from their reparse data.
func platformProbes() probes {
	return probes{isLink: isLinkProbe, reparse: fsmeta.ReadReparse, respell: respellFromDrive}
}

// Reached returns, for a path that enters the run from outside a listing,
// the path Win32 opens for it: its `.` and `..` removed by name
// (syspath.Opened), which is also how the check judges it. The run then
// checks, opens, stages and logs that one path.
func (RootCheck) Reached(path string) string {
	return syspath.Opened(path)
}

// respellFromDrive returns path spelled as the disk holds it, from its
// drive or share (fsmeta.OnDiskPath): the file system's own comparison of
// names decides, so in a case-sensitive directory another case of a name is
// another name.
func respellFromDrive(path string) string {
	return fsmeta.OnDiskPath("", path)
}

// sameDirectory reports whether prefix, which matched root only when ASCII
// case is ignored, names root's own directory: spelled from the disk it is
// root's bytes. In a case-sensitive directory holding both spellings it is
// another directory, which Windows opens and the walk must not take for
// the root. Only the root's ancestors are listed to answer, never anything
// under prefix.
func (walker rootWalk) sameDirectory(prefix, root string) bool {
	return walker.probe.respell != nil && walker.probe.respell(prefix) == root
}
