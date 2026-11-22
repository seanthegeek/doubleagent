package collect_test

import (
	"regexp"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

// directoryUsageVaries is false: a directory row's size is the sum of its
// files' sizes, which the image fixes.
const directoryUsageVaries = false

// linkSizeIsTargetLength is false: a link's size is what the reparse point
// reports, not checked against its target here.
const linkSizeIsTargetLength = false

// ownerField matches a manifest row's non-empty owner, the account that
// owns the file, which is the test's own.
var ownerField = regexp.MustCompile(`"owner":"(\\.|[^"\\])+"`)

// replaceOwner returns a manifest row with a non-empty owner replaced.
func replaceOwner(line string) string {
	return ownerField.ReplaceAllString(line, `"owner":"{OWNER}"`)
}

// replaceAccount replaces collection.json's run_as and run_as_admin, after
// checking the account is named and the membership is a boolean.
func (n normaliser) replaceAccount(summary jsonvalue.Value) {
	n.requireText("collection.json run_as", summary.Get("run_as").StringValue() != "")
	n.requireText("collection.json run_as_admin", summary.Get("run_as_admin").IsBool())
	summary.Set("run_as", jsonvalue.NewString("{ACCOUNT}"))
	summary.Set("run_as_admin", jsonvalue.NewString("{ADMIN}"))
}

// systemName returns the operating system's name, which collection.json's
// platform starts with.
func systemName() string { return "Windows" }

// pinUmask does nothing: Windows has no umask, and the image's modes are
// not compared there.
func pinUmask(testing.TB) {}
