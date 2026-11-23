package walk

import (
	"context"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/follow"
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/stage"
	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// Links is what LinkedParent finds between a base and a path. Outside is the
// first symlinked directory whose target leads out of the base or cannot be
// resolved. Passed lists the symlinked directories met before it whose
// targets stay inside the base, From is the deepest of them and Into where
// the base physically holds it; all are empty when there is none.
type Links struct {
	Outside string
	Passed  []string
	From    string
	Into    string
}

// LinkedParent reports true when a symlinked directory between base ("" for
// the root) and path leads out of base once resolved, or cannot be resolved,
// glob expansion follows such a link, the walk never does. Only the components of path past as many as base has are tested, and
// path itself is not, unless it ends in "/", as a project's own path is given
// so that the project directory is tested too; the components are counted,
// never compared. check is the image root's (nil live): a link it refuses
// counts as leading out, and is not resolved.
func LinkedParent(base, path string, check follow.Check) (Links, bool) {
	var links Links
	relative := path
	for range strings.Count(base, "/") + 1 {
		if position := strings.IndexByte(relative, '/'); position >= 0 {
			relative = relative[position+1:]
		}
	}
	directory, root := base, ""
	for {
		component, rest, found := strings.Cut(relative, "/")
		if !found {
			return Links{Passed: links.Passed, From: links.From, Into: links.Into}, false
		}
		directory, relative = directory+"/"+component, rest
		if !isSymlink(directory) {
			continue
		}
		links.Outside = directory
		if root == "" {
			physicalBase, err := follow.PhysicalDir(check, baseOrRoot(base))
			if err != nil {
				return links, true
			}
			root = physicalBase
		}
		physical, err := follow.PhysicalDir(check, directory)
		if err != nil {
			return links, true
		}
		inside, found := stage.Under(physical, root)
		if !found {
			return links, true
		}
		links.From = directory
		links.Into = strings.TrimSuffix(base+"/"+inside, "/")
		links.Passed = append(links.Passed, directory)
	}
}

// baseOrRoot returns base, or / for the empty base of the live root.
func baseOrRoot(base string) string {
	if base == "" {
		return "/"
	}
	return base
}

// isSymlink reports whether path itself is a link the walk never enters: a
// symlink or, on Windows, any reparse point (a junction, a cloud placeholder
// directory), told by the ReparsePoint attribute.
func isSymlink(path string) bool {
	meta, err := fsmeta.Lstat(path)
	return err == nil && meta.IsLink()
}

// physicalPath returns path with its leading from replaced by to, the path
// the log names for a match staged through an in-base link.
func physicalPath(path, from, into string) string {
	if from == "" {
		return path
	}
	return into + path[len(from):]
}

// throughLinks returns stager set to stage paths under from at into, where
// into is first mapped by the link stager already stages through: the links a
// match passes inside a project compose with the link the project's own path
// passes inside its home. Every path staged with the result lies under from,
// the deepest link before it, so one pair suffices.
func throughLinks(stager stage.Stager, from, into string) stage.Stager {
	if from == "" {
		return stager
	}
	if stager.From != "" && (into == stager.From || strings.HasPrefix(into, stager.From+"/")) {
		into = physicalPath(into, stager.From, stager.Into)
	}
	return stager.Through(from, into)
}

// claimed reports whether path, under base, is one of claims or lies under
// one through real directories only: the claim's
// own walk then records it, and a walk never descends a symlinked directory.
func claimed(path, base string, claims *syspath.Entries[struct{}]) bool {
	for len(path) > len(base) {
		if claims.Has(path) {
			return true
		}
		position := strings.LastIndexByte(path, '/')
		if position < 0 {
			return false
		}
		path = path[:position]
		if isSymlink(path) {
			return false
		}
	}
	return false
}

// recordPassedLinks collects each link of links (symlinked directories under
// base) once per seen set, as a symlink row of home under the agent agents
// holds for it, or the owner's agent when agents is nil or has none, unless it
// is no longer a link, is claimed, or already has a manifest row; each is
// staged where base physically holds it, through the owner's stager.
func (collector *Collector) recordPassedLinks(ctx context.Context, owner scope, base string,
	links []string, seen *syspath.Entries[struct{}], agents *syspath.Entries[string],
) error {
	baseStager, ownerAgent := owner.stager, owner.agent
	for _, link := range links {
		if !seen.Add(link, struct{}{}) {
			continue
		}
		if !isSymlink(link) || claimed(link, base, owner.claims) ||
			collector.Manifest.HasPath(link) {
			continue
		}
		owner.agent = ownerAgent
		if agent, _, found := agents.Find(link); found {
			owner.agent = agent
		}
		found, _ := LinkedParent(base, link, collector.check())
		owner.stager = throughLinks(baseStager, found.From, found.Into)
		if _, err := collector.collectPath(ctx, owner, link); err != nil {
			return err
		}
	}
	return nil
}

// RecordPassedLinks collects the symlinked directories of links that a path
// under base ran through, as the relocation step does for a startup file: no
// path is claimed, and each link is considered once in this call. stager is
// the base's, which a link's own place in base is composed with.
func (collector *Collector) RecordPassedLinks(ctx context.Context, user, base, home, agent string,
	stager stage.Stager, links []string,
) error {
	owner := scope{user: user, home: home, agent: agent, stager: stager}
	return collector.recordPassedLinks(ctx, owner, base, links, collector.pathSet(), nil)
}

// readField returns a match path as a field read from an agent|glob|path
// line: one `|` ending a path that holds no other is dropped, as a shell's
// read drops a final delimiter.
func readField(path string) string {
	trimmed, found := strings.CutSuffix(path, "|")
	if found && !strings.Contains(trimmed, "|") {
		return trimmed
	}
	return path
}
