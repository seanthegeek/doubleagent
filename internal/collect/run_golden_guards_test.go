package collect_test

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

// archivePathsNamingNoMember returns each archive path of the run's
// manifest that names no member of its archive byte for byte, in row order.
// On an output directory that folds case the staging tree could spell a
// name another way than the row; the archive must still hold the row's. A
// path holding U+FFFD is passed over: the manifest writes that character
// for each byte of a name that is not valid UTF-8, where the member keeps
// the name's bytes.
func archivePathsNamingNoMember(test testing.TB, run collectorRun) []string {
	test.Helper()
	members := map[string]bool{}
	for _, member := range run.members {
		members[member.name] = true
	}
	var missing []string
	for _, line := range splitLines(run.files[".manifest.jsonl"]) {
		row, err := jsonvalue.DecodeString(line)
		if err != nil {
			test.Errorf("manifest row is not JSON: %v: %s", err, line)
			continue
		}
		archived := row.Get("archive_path").StringValue()
		if archived != "" && !strings.ContainsRune(archived, utf8.RuneError) && !members["./"+archived] {
			missing = append(missing, archived)
		}
	}
	return missing
}
