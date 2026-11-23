package parserkit

import (
	"regexp"

	"github.com/seanthegeek/doubleagent/internal/input"
)

// Kind is one kind of file a parser reads: a name its tests use and the
// test of an artifact that decides whether the artifact is of the kind.
// A parser lists its kinds (Kinds) and its Wants is WantsAny of them, so
// the list is the whole of what it reads and a test can enumerate it.
type Kind struct {
	// Name says which file the kind is, as the parser's research page
	// names it.
	Name string
	// Wants reports whether the artifact is of the kind.
	Wants func(artifact *input.Artifact) bool
}

// KindLister is implemented by every registered parser.
type KindLister interface {
	// Kinds returns the kinds of file the parser reads, in the order its
	// Wants tries them.
	Kinds() []Kind
}

// PatternKind returns the kind of the artifacts whose rel matches pattern.
func PatternKind(name string, pattern *regexp.Regexp) Kind {
	return Kind{Name: name, Wants: func(artifact *input.Artifact) bool { return pattern.MatchString(artifact.Rel) }}
}

// RelKind returns the kind of the artifact whose rel is rel.
func RelKind(name, rel string) Kind {
	return Kind{Name: name, Wants: func(artifact *input.Artifact) bool { return artifact.Rel == rel }}
}

// RelFuncKind returns the kind of the artifacts whose rel the test holds
// for.
func RelFuncKind(name string, test func(rel string) bool) Kind {
	return Kind{Name: name, Wants: func(artifact *input.Artifact) bool { return test(artifact.Rel) }}
}

// WantsAny reports whether any of kinds wants the artifact, trying them in
// order and stopping at the first that does, so a kind whose test reads
// the file is tried only when the ones before it said no.
func WantsAny(kinds []Kind, artifact *input.Artifact) bool {
	for _, kind := range kinds {
		if kind.Wants(artifact) {
			return true
		}
	}
	return false
}

// KindsOf returns the kinds a parser lists, none when it does not
// implement KindLister.
func KindsOf(parser Parser) []Kind {
	if lister, ok := parser.(KindLister); ok {
		return lister.Kinds()
	}
	return nil
}
