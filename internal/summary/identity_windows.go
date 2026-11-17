package summary

// LiveRootText is the root collection.json and the log name in a live run,
// as the PowerShell collector writes it.
const LiveRootText = `\`

// identityLines are the account lines of collection.json on Windows.
func identityLines(identity Identity) string { return windowsIdentityLines(identity) }
