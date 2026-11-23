package walk

import "testing"

func TestGlobSpellsComparesEveryByteOnEveryPlatform(t *testing.T) {
	cases := []struct {
		name    string
		base    string
		pattern string
		path    string
		want    bool
	}{
		{"a lower-case name spells the lower-case path", "/home/u", ".config/goose",
			"/home/u/.config/goose", true},
		{"a lower-case name does not spell the capitalised path", "/home/u", ".config/goose",
			"/home/u/.config/Goose", false},
		{"a capitalised name spells the capitalised path", "/home/u", ".config/Goose",
			"/home/u/.config/Goose", true},
		{"a capitalised name does not spell the lower-case path", "/home/u", ".config/Goose",
			"/home/u/.config/goose", false},
		{"a wildcard segment spells either case it matches", "/home/u", ".config/*oose",
			"/home/u/.config/Goose", true},
		{"a wildcard segment still compares its literal letters by byte", "/home/u",
			".config/goo*", "/home/u/.config/Goose", false},
		{"a base holding glob metacharacters is compared as text", "/home/a[1]*?",
			".config/goose", "/home/a[1]*?/.config/goose", true},
		{"a path under another base is not spelled", "/home/u", ".config/goose",
			"/home/v/.config/goose", false},
		{"any word of a several-word glob may spell the path", "/home/u",
			".config/Goose\n.config/goose", "/home/u/.config/goose", true},
		{"a literal word holding a bracket spells the name it found as typed", "/home/u",
			"a[1]", "/home/u/a[1]", true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := globSpells(test.base, test.pattern, test.path); got != test.want {
				t.Errorf("globSpells(%s, %s, %s) = %v; want %v", test.base, test.pattern,
					test.path, got, test.want)
			}
		})
	}
}

func TestAMatchAlreadySpelledFromTheDiskIsRepresentedByTheLineThatSpellsIt(t *testing.T) {
	// The Windows expander respells each literal segment from the disk, so
	// two lines can reach the fold with one path already spelled alike.
	found := []Match{{Agent: "first", Glob: ".config/Goose", Path: "/home/u/.config/goose"},
		{Agent: "second", Glob: ".config/goose", Path: "/home/u/.config/goose"}}
	merged := foldCaseMatches("/home/u", found, nil, nil)
	if len(merged) != 1 || merged[0] != found[1] {
		t.Errorf("merged %v; want the one match of the line .config/goose", merged)
	}
}
