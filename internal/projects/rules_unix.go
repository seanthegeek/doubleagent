//go:build unix

package projects

// platformRules returns the POSIX rules: only a value starting with / is a
// path.
func platformRules() pathRules { return pathRules{} }
