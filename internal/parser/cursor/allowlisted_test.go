package cursor

import (
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

func TestAnAppliedAllowlistOptionIsNamedAtItsFirstReferenceOnly(t *testing.T) {
	tests := []struct {
		name, applied, want string
	}{
		{"each option named once is named", `["a","b"]`, " allowlisted=curl,head"},
		{"an id with no option is written as itself", `["a","z"]`, " allowlisted=curl,z"},
		{"a repeated id is written as itself after its first time", `["a","b","a","a"]`,
			" allowlisted=curl,head,a,a"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			review, err := jsonvalue.DecodeString(`{"status":"Done","allowlistOptions":[{"id":"a","commandName":"curl"},` +
				`{"id":"b","commandName":"head"}],"appliedAllowlistOptionIds":` + test.applied + `}`)
			if err != nil {
				t.Fatal(err)
			}
			got := permissionText(jsonvalue.NewNull(), review)
			if !strings.HasSuffix(got, test.want) {
				t.Errorf("text %q, want it to end %q", got, test.want)
			}
		})
	}
}
