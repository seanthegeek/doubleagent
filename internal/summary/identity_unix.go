//go:build unix

package summary

import "strconv"

// LiveRootText is the root collection.json and the log name in a live run.
const LiveRootText = "/"

// account is the account field of collection.json on Unix: run_as_uid, the
// effective user ID as a string.
type account struct {
	RunAsUID string `json:"run_as_uid"`
}

// accountOf returns the account field of identity.
func accountOf(identity Identity) account {
	return account{RunAsUID: strconv.Itoa(identity.UID)}
}
