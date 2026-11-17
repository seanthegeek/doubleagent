//go:build unix

package summary

import "strconv"

// LiveRootText is the root collection.json and the log name in a live run,
// as the sh collector writes it.
const LiveRootText = "/"

// account is the account field of collection.json on Unix: run_as_uid, the
// effective user ID as a string, as the sh collector writes it.
type account struct {
	RunAsUID string `json:"run_as_uid"`
}

// accountOf returns the account field of identity.
func accountOf(identity Identity) account {
	return account{RunAsUID: strconv.Itoa(identity.UID)}
}
