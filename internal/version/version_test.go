package version

import (
	"runtime/debug"
	"testing"
)

func TestStringReturnsAStampedVersionUnchanged(t *testing.T) {
	saved := Version
	t.Cleanup(func() { Version = saved })
	Version = "0.1.0"
	if got := String(); got != "0.1.0" {
		t.Fatalf("String() = %q, want 0.1.0", got)
	}
}

func TestCommitSuffix(t *testing.T) {
	cases := []struct {
		name     string
		settings []debug.BuildSetting
		want     string
	}{
		{"no revision gives no suffix", nil, ""},
		{"a revision is cut to twelve digits",
			[]debug.BuildSetting{{Key: "vcs.revision", Value: "0123456789abcdef0123"}},
			"+0123456789ab"},
		{"a dirty tree is marked",
			[]debug.BuildSetting{
				{Key: "vcs.revision", Value: "0123456789ab"},
				{Key: "vcs.modified", Value: "true"},
			},
			"+0123456789ab-modified"},
		{"a clean tree is not",
			[]debug.BuildSetting{
				{Key: "vcs.revision", Value: "0123456789ab"},
				{Key: "vcs.modified", Value: "false"},
			},
			"+0123456789ab"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := commitSuffix(testCase.settings); got != testCase.want {
				t.Fatalf("commitSuffix() = %q, want %q", got, testCase.want)
			}
		})
	}
}
