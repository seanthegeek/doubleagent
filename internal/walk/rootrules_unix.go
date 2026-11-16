//go:build unix

package walk

import (
	"strings"

	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// walkPath walks rest, a path relative to the resolved root, as the kernel
// resolves it: each component, a link's target included, one at a time from
// where the walk has physically got to, so a `..` goes up from the link's
// real directory.
func (walker rootWalk) walkPath(rest string) rootScope {
	return walker.walk(steps(rest, ""))
}

// stopAllows reports whether a component the root check could not lstat
// ends the walk as the image's own answer. On Unix it always does: the
// kernel resolves the same component first and fails the whole path with
// the same error, whatever follows it, so nothing behind it can be reached.
func stopAllows(error, bool) bool { return true }

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
		if err != nil && stopAllows(err, hasNames(components)) {
			return walker.unreadable(candidate)
		}
		if err != nil {
			return walker.failedLink(candidate)
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
	if walker.reached != nil {
		*walker.reached = current
	}
	return rootScope{}
}

// follow reads link, a symlink in the physical directory parent, and returns
// where its target's walk starts (parent for a relative target, the resolved
// root for an absolute one) and the target's components, or the decision
// when the target leads out of the root or the link cannot be verified. The
// target is never cleaned: a ".." after a link inside it goes up from where
// that link leads, so the walk takes it one component at a time. readlink
// returns the bytes the link stores, unchanged.
func (walker rootWalk) follow(parent, link string, hops int) (string, []step, rootScope) {
	if hops > maxLinkHops {
		return "", nil, walker.failedLink(link)
	}
	target, err := walker.probe.readlink(link)
	if err != nil {
		return "", nil, walker.failedLink(link)
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

// hasNames reports whether components hold a name still to be walked, not
// only empty or `.` components.
func hasNames(components []step) bool {
	for _, component := range components {
		if component.name != "" && component.name != "." {
			return true
		}
	}
	return false
}

// above is the decision for a `..` that climbs above the resolved root:
// via, the link whose target holds it, leads out of the root; a `..` of the
// path itself climbs above it, and the decision names that path.
func (walker rootWalk) above(via string) rootScope {
	if via == "" {
		return rootScope{name: walker.path, unverified: true, climbs: true}
	}
	return rootScope{name: walker.named(via)}
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

// probes are the two calls the root check makes on the image: whether a
// path is a link, and a link's target, the bytes it stores.
type probes struct {
	isLink   func(path string) (bool, error)
	readlink func(path string) (string, error)
}

// platformProbes returns the probes of the file system itself on Unix.
func platformProbes() probes {
	return probes{isLink: isLinkProbe, readlink: osfile.Readlink}
}
