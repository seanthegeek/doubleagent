package gitmeta

import (
	"reflect"
	"testing"
)

// gitCases are configs whose remotes were read with git 2.56's
// `git config -f <file> --list`; an empty list is a file git refuses as a
// whole, whose malformed line this package skips.
func gitCases() []configCase {
	one := []Remote{remote("p", list("one"))}
	return []configCase{
		{"a second header on the header's line takes the keys", "[remote \"o\"][remote \"p\"] url = one\n", one, nil},
		{"a second header after a blank takes the keys", "[remote \"o\"] [remote \"p\"]\nurl = one\n", one, nil},
		{"a header of another section after a remote's", "[remote \"o\"] [core] url = x\n", nil, nil},
		{"a blank before the section name is malformed", "[ remote \"o\"]\nurl = x\n", nil, nil},
		{"a blank after the subsection is malformed", "[remote \"o\" ]\nurl = x\n", nil, nil},
		{"a blank after a dotted name is malformed", "[remote.o ]\nurl = x\n", nil, nil},
		{"a tab between the name and the subsection", "[remote\t\"o\"]\nurl = x\n", []Remote{remote("o", list("x"))}, nil},
		{"a carriage return between the name and the subsection", "[remote\r\"o\"]\nurl = x\n",
			[]Remote{remote("o", list("x"))}, nil},
		{"a dotted name with a subsection names the remote by both",
			"[Remote.FOO \"Bar\"]\nurl = x\n", []Remote{remote("foo.Bar", list("x"))}, nil},
		{"a form feed ending a value is kept", "[remote \"o\"]\nurl = a\f\v\n", []Remote{remote("o", list("a\f\v"))}, nil},
		{"a form feed starting a value is kept", "[remote \"o\"]\nurl = \fa\n", []Remote{remote("o", list("\fa"))}, nil},
		{"a line starting with a form feed is malformed", "[remote \"o\"]\n\f url = x\n", nil, nil},
		{"a line starting with a carriage return is not", "[remote \"o\"]\n\r url = x\n", []Remote{remote("o", list("x"))}, nil},
		{"tabs and spaces inside a value are kept", "[remote \"o\"]\n\turl = a\tb  c\n", []Remote{remote("o", list("a\tb  c"))}, nil},
	}
}

func TestParseConfigReadsHeadersAndBlanksAsGitDoes(t *testing.T) {
	for _, test := range gitCases() {
		t.Run(test.name, func(t *testing.T) {
			remotes, problems := ParseConfig(test.text)
			if test.remotes == nil {
				test.remotes = []Remote{}
			}
			if !reflect.DeepEqual(remotes, test.remotes) || !reflect.DeepEqual(problems, test.problems) {
				t.Errorf("remotes %+v, problems %q; want %+v, %q", remotes, problems, test.remotes, test.problems)
			}
		})
	}
}
