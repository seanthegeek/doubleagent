//go:build unix

package manifest

import (
	"strings"
	"testing"
)

// platformFieldsText is what a row holds between mode and sha256 on this
// platform: nothing.
const platformFieldsText = ""

func TestUnixRowsCarryNoWindowsFields(t *testing.T) {
	line := lineOf(t, Row{Owner: "x", Attributes: "Hidden"})
	for _, field := range []string{`"owner"`, `"attributes"`} {
		if strings.Contains(line, field) {
			t.Errorf("row %s holds %s", line, field)
		}
	}
}
