//go:build windows || darwin

package stage

import "testing"

func TestPlacedAtComparesTheReportedPlaceByBytes(t *testing.T) {
	stager := Stager{Dir: "/out/.stage", Physical: "/private/out/.stage"}
	cases := []struct {
		name, final, dest string
		want              bool
	}{
		{"the copy sits where it was named", "/private/out/.stage/fs/Users/Alice/x",
			"/out/.stage/fs/Users/Alice/x", true},
		{"another case of a directory is another entry's place",
			"/private/out/.stage/fs/Users/Alice/x", "/out/.stage/fs/Users/alice/x", false},
		{"a decomposed name the volume stores composed is another entry's place",
			"/private/out/.stage/fs/café", "/out/.stage/fs/café", false},
		{"a place outside the staging directory proves nothing", "/elsewhere/x",
			"/out/.stage/fs/x", true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := stager.placedAt(testCase.final, testCase.dest); got != testCase.want {
				t.Errorf("placedAt(%q, %q) = %v, want %v", testCase.final, testCase.dest, got,
					testCase.want)
			}
		})
	}
}
