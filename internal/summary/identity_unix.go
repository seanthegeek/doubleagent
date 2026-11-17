//go:build unix

package summary

// LiveRootText is the root collection.json and the log name in a live run,
// as the sh collector writes it.
const LiveRootText = "/"

// identityLines are the account lines of collection.json on Unix.
func identityLines(identity Identity) string { return unixIdentityLines(identity) }
