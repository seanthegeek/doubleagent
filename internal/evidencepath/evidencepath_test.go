package evidencepath

import (
	"slices"
	"testing"
)

func TestIsDrivePathNeedsALetterAColonAndASeparator(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{`C:\x`, true}, {"c:/x", true}, {`z:\`, true}, {"C:", false}, {"C:x", false},
		{`1:\x`, false}, {`\\server\share`, false}, {"/c/x", false}, {"é:/x", false},
	}
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			if got := IsDrivePath(test.path); got != test.want {
				t.Errorf("got %v, want %v", got, test.want)
			}
		})
	}
}

func TestElementsSplitByTheEvidencesRules(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		windows bool
		want    []string
	}{
		{"a Windows path splits at both slashes", `C:\a/b\\c\`, true, []string{"C:", "a", "b", "c"}},
		{"a POSIX path keeps its backslashes", `/a\b//c/`, false, []string{`a\b`, "c"}},
		{"dot elements stay", "/a/./b", false, []string{"a", ".", "b"}},
		{"nothing gives no element", "", true, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := Elements(test.path, test.windows); !slices.Equal(got, test.want) {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}
