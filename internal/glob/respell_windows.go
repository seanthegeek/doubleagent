package glob

import (
	"strings"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
)

// respell returns literal, a slash-separated path below directory, with
// each segment that exists spelled as its directory holds it, as the
// PowerShell collector's Expand-Glob respells every literal catalog segment
// through Get-OnDiskName; a segment that is missing, ambiguous or holds a
// search wildcard stays as written. Empty segments are kept, so a trailing
// slash survives.
func respell(directory, literal string) string {
	directory = strings.TrimSuffix(directory, "/")
	segments := strings.Split(literal, "/")
	for i, segment := range segments {
		if segment == "" {
			continue
		}
		if name, ok := fsmeta.OnDiskName(directory, segment); ok {
			segments[i] = name
		}
		directory += "/" + segments[i]
	}
	return strings.Join(segments, "/")
}
