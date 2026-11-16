package stage

import "testing"

// caseSensitiveVolume skips the test: no Linux runner offers a volume that
// keeps letter case apart and takes Unicode forms for one.
func caseSensitiveVolume(t *testing.T) string {
	t.Helper()
	t.Skip("no case-sensitive volume that merges Unicode forms here")
	return ""
}
