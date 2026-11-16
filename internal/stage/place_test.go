//go:build windows || darwin

package stage

import "testing"

func TestOccupantAtComparesTheReportedPlaceByBytes(t *testing.T) {
	stager := Stager{Dir: "/out/.stage", Physical: "/private/out/.stage"}
	cases := []struct {
		name, final, dest, occupant string
		other                       bool
	}{
		{"the copy sits where it was named", "/private/out/.stage/fs/Users/Alice/x",
			"/out/.stage/fs/Users/Alice/x", "/out/.stage/fs/Users/Alice/x", false},
		{"another case of a directory is another place",
			"/private/out/.stage/fs/Users/Alice/x", "/out/.stage/fs/Users/alice/x",
			"/out/.stage/fs/Users/Alice/x", true},
		{"a decomposed name the volume stores composed is another place",
			"/private/out/.stage/fs/café", "/out/.stage/fs/café",
			"/out/.stage/fs/café", true},
		{"a place outside the staging directory proves nothing", "/elsewhere/x",
			"/out/.stage/fs/x", "", false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			occupant, other := stager.occupantAt(testCase.final, testCase.dest)
			if other != testCase.other || (other && occupant != testCase.occupant) {
				t.Errorf("occupantAt(%q, %q) = %q, %v; want %q, %v", testCase.final,
					testCase.dest, occupant, other, testCase.occupant, testCase.other)
			}
		})
	}
}
