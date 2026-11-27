package parserkit

import (
	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/textlines"
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

// PatternKind returns the kind of the artifacts whose rel matches pattern,
// ignoring the case of ASCII letters as the catalog does.
func PatternKind(name string, pattern *RelPattern) Kind {
	return Kind{Name: name, Wants: func(artifact *input.Artifact) bool { return pattern.MatchString(artifact.Rel) }}
}

// RelKind returns the kind of the artifact whose rel is rel, ignoring the
// case of ASCII letters as the catalog does.
func RelKind(name, rel string) Kind {
	return Kind{Name: name, Wants: func(artifact *input.Artifact) bool { return textlines.EqualFoldASCII(artifact.Rel, rel) }}
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
