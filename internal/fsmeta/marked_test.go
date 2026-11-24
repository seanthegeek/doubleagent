package fsmeta

import "testing"

func TestMarkedDirectory(t *testing.T) {
	cases := []struct {
		name string
		meta Meta
		want bool
	}{
		{"a directory is marked", Meta{Kind: Dir}, true},
		{"a file is not", Meta{Kind: File}, false},
		{"a symlink without the attribute is not", Meta{Kind: Symlink}, false},
		{"a directory symlink is", Meta{Kind: Symlink, Attributes: directoryAttribute}, true},
		{"a junction is",
			Meta{Kind: Symlink, Attributes: directoryAttribute | reparsePointAttribute}, true},
		{"another reparse point made as a directory is",
			Meta{Kind: Other, Attributes: directoryAttribute | reparsePointAttribute}, true},
		{"a reparse point made as a file is not", Meta{Kind: File, Attributes: reparsePointAttribute},
			false},
		{"the zero metadata of a path that cannot be read is not", ZeroMeta(), false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if got := testCase.meta.MarkedDirectory(); got != testCase.want {
				t.Errorf("MarkedDirectory(%+v) = %v, want %v", testCase.meta, got, testCase.want)
			}
		})
	}
}
