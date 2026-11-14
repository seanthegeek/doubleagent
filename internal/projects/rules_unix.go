//go:build unix

package projects

// platformRules returns the sh collector's rules: only a value starting with
// / is a path, as on the hosts it runs on.
func platformRules() pathRules { return pathRules{} }
