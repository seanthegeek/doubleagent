package summary

// LiveRootText is the root collection.json and the log name in a live run,
// as the PowerShell collector writes it.
const LiveRootText = `\`

// account is the account fields of collection.json on Windows: the account
// as DOMAIN\user, and whether it is in the Administrators group.
type account struct {
	RunAs      string `json:"run_as"`
	RunAsAdmin bool   `json:"run_as_admin"`
}

// accountOf returns the account fields of identity.
func accountOf(identity Identity) account {
	return account{RunAs: identity.RunAs, RunAsAdmin: identity.Admin}
}
