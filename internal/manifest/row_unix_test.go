//go:build unix

package manifest

import (
	"strings"
	"testing"
)

func TestUnixRowsCarryNoWindowsFields(t *testing.T) {
	line := string(appendRow(nil, Row{Owner: "x", Attributes: "Hidden"}))
	for _, field := range []string{`"owner"`, `"attributes"`} {
		if strings.Contains(line, field) {
			t.Errorf("row %s holds %s", line, field)
		}
	}
}
