// Package parsers is the registry of the agent parsers: the list in the
// order that decides ties in the output, the routing of an artifact to the
// parsers offered it, and the run of every parser over a collection. Each
// parser is a package of its own in a subdirectory of this one, named for
// its agent.
//
// A parser is a value created for one run, since some keep a cache of the
// files they have read; All returns new values each time, and a Registry
// holds one run's values so that every artifact of the run reaches the same
// ones.
package parsers
