package gitmeta

import (
	"reflect"
	"strings"
	"testing"
)

// The /srv/proj repository of the analysis fixtures.
const (
	projectRemote  = "git@github.com:alice/proj.git"
	projectMirror  = "https://mirror.example.com/alice/proj.git"
	projectPushURL = "https://alice:FIXTURE-TOKEN@github.com/alice/proj.git"
	projectUpper   = "https://github.com/upstream/proj.git"
	projectConfig  = "[core]\n" +
		"\trepositoryformatversion = 0\n" +
		"\tbare = false\n" +
		"[remote \"origin\"]\n" +
		"\turl = " + projectRemote + "\n" +
		"\turl = " + projectMirror + "\n" +
		"\tpushurl = " + projectPushURL + " ; the push goes through HTTPS\n" +
		"\tfetch = +refs/heads/*:refs/remotes/origin/*\n" +
		"[remote \"upstream\"]\n" +
		"\turl = \"" + projectUpper + "\"\n" +
		"\tfetch = +refs/heads/*:refs/remotes/upstream/*\n" +
		"[branch \"main\"]\n" +
		"\tremote = origin\n" +
		"\tmerge = refs/heads/main\n"
)

// remote returns a Remote with its URLs and push URLs.
func remote(name string, urls []string, pushURLs ...string) Remote {
	if urls == nil {
		urls = []string{}
	}
	if pushURLs == nil {
		pushURLs = []string{}
	}
	return Remote{Name: name, URLs: urls, PushURLs: pushURLs}
}

// list is shorthand for a slice of strings.
func list(values ...string) []string { return values }

// configCase is one .git/config text with the remotes and problems read
// from it.
type configCase struct {
	name     string
	text     string
	remotes  []Remote
	problems []string
}

func TestParseConfig(t *testing.T) {
	for _, test := range append(configCases(), valueCases()...) {
		t.Run(test.name, func(t *testing.T) {
			remotes, problems := ParseConfig(test.text)
			if test.remotes == nil {
				test.remotes = []Remote{}
			}
			if !reflect.DeepEqual(remotes, test.remotes) {
				t.Errorf("remotes:\n%+v\nwant:\n%+v", remotes, test.remotes)
			}
			if !reflect.DeepEqual(problems, test.problems) {
				t.Errorf("problems %q, want %q", problems, test.problems)
			}
		})
	}
}

// configCases are the tests of sections, keys, comments and lines.
func configCases() []configCase {
	return []configCase{
		{"the fixture config", projectConfig, []Remote{
			remote("origin", list(projectRemote, projectMirror), projectPushURL),
			remote("upstream", list(projectUpper))}, nil},
		{"a pushurl is only what the file says",
			"[remote \"origin\"]\n\turl = https://h/r.git\n",
			[]Remote{remote("origin", list("https://h/r.git"))}, nil},
		{"case, comments, quotes and continuation", "# a comment\n; another\n" +
			"[Remote \"Fork\"]\n  URL = \"https://h/a;b#c.git\"  # trailing comment\n" +
			"  pushUrl = https://h/\\\npush.git\n[remote \"second\"]\n" +
			"\turl = https://h/one.git\n\tpushurl = https://h/push-a.git\n" +
			"\turl = https://h/two.git\n\tpushurl = https://h/push-b.git\n\turl =\n" +
			"[remote.legacy]\n\turl = /srv/mirror.git\n[branch \"main\"]\n" +
			"\turl = not-a-remote\n", []Remote{
			remote("Fork", list("https://h/a;b#c.git"), "https://h/push.git"),
			remote("legacy", list("/srv/mirror.git")),
			remote("second", list("https://h/one.git", "https://h/two.git"),
				"https://h/push-a.git", "https://h/push-b.git")}, nil},
		{"a backslash ending a comment does not continue", "[remote \"origin\"]\n" +
			"\turl = https://h/origin.git  # cloned from C:\\git\\\n[remote \"backup\"]\n" +
			"\turl = https://h/backup.git\n# mirrors live under D:\\git\\\n" +
			"[remote \"mirror\"]\n\turl = https://h/\\\n" +
			"mirror.git ; comment with a backslash \\\n[remote \"last\"]\n" +
			"\turl = https://h/last.git\n", []Remote{
			remote("backup", list("https://h/backup.git")),
			remote("last", list("https://h/last.git")),
			remote("mirror", list("https://h/mirror.git")),
			remote("origin", list("https://h/origin.git"))}, nil},
		{"the dotted header lowers its subsection", "[remote.Origin]\n" +
			"\turl = https://h/origin.git\n[remote \"origin\"]\n" +
			"\tpushurl = https://h/origin-push.git\n[remote \"Origin\"]\n" +
			"\turl = https://h/capital.git\n", []Remote{
			remote("Origin", list("https://h/capital.git")),
			remote("origin", list("https://h/origin.git"), "https://h/origin-push.git")}, nil},
		{"a byte-order mark, CRLF and Unicode line separators", "\ufeff[remote \"origin\"]\r\n" +
			"\turl = https://h/origin.git\r\n[remote \"odd\"]\n" +
			"\turl = \"/srv/team\u2028archive.git\"\n\tpushurl = \"/srv/team\u0085archive.git\"\n",
			[]Remote{remote("odd", list("/srv/team\u2028archive.git"), "/srv/team\u0085archive.git"),
				remote("origin", list("https://h/origin.git"))}, nil},
		{"a pair on the header line", "[remote \"origin\"] url = https://h/origin.git\n" +
			"[remote \"note\"] # just a comment\n\turl = https://h/note.git\n" +
			"[remote \"two\"]\turl = https://h/\\\ntwo.git ; pushurl = not-a-key\n", []Remote{
			remote("note", list("https://h/note.git")),
			remote("origin", list("https://h/origin.git")),
			remote("two", list("https://h/two.git"))}, nil},
		{"malformed lines are skipped", "[remote \"origin\"]\n" +
			"\tthis line has no equals sign\n\turl = https://h/origin.git\n" +
			"[remote \"broken\"\n\turl = https://h/broken.git\n= no key\n" +
			"[remote \"upstream\"]\n\turl = https://h/upstream.git\n\x00\u00ff garbage\n",
			[]Remote{remote("origin", list("https://h/origin.git")),
				remote("upstream", list("https://h/upstream.git"))}, nil},
		{"a subsection needs a blank before its quote", "[remote\"x\"]\n\turl = /srv/x\n",
			nil, nil},
		{"an empty config has no remotes", "", nil, nil},
		{"a remote without a url is not listed", "[remote \"x\"]\n\tfetch = +refs/*\n", nil, nil},
		{"a remote with only a push URL is listed",
			"[remote \"p\"]\n\tpushurl = https://h/p.git\n",
			[]Remote{remote("p", nil, "https://h/p.git")}, nil},
	}
}

// valueCases are the tests of value quoting, escapes and trimming.
func valueCases() []configCase {
	pad := strings.Repeat(" ", 200_000)
	return []configCase{
		{"quoted and escaped whitespace is kept", "[remote \"a\"]\n" +
			"\turl =   \"/srv/mirror.git \"  \n[remote \"b\"]\n" +
			"\turl = \" /srv/lead.git\"\t; note\n[remote \"c\"]\n" +
			"\turl = /srv/plain.git\\t \n[remote \"d\"]\n\turl = \"/srv/in ner.git\"\n",
			[]Remote{remote("a", list("/srv/mirror.git ")), remote("b", list(" /srv/lead.git")),
				remote("c", list("/srv/plain.git\t")), remote("d", list("/srv/in ner.git"))},
			nil},
		{"an unterminated quote is dropped and reported", "[remote \"origin\"]\n" +
			"\turl = https://h/origin.git\n[remote \"cut\"]\n" +
			"\tpushurl = https://h/cut-push.git\n\turl = \"https://example.com/team/part",
			[]Remote{remote("cut", nil, "https://h/cut-push.git"),
				remote("origin", list("https://h/origin.git"))},
			list("line 5: unterminated quoted value")},
		{"a quote closed on a continued line is fine",
			"[remote \"a\"]\n\turl = \"https://h/\\\na.git\"\n",
			[]Remote{remote("a", list("https://h/a.git"))}, nil},
		{"an unsupported escape is dropped and reported", "[remote \"origin\"]\n" +
			"\turl = https://h/origin.git\n[remote \"bad\"]\n" +
			"\turl = https://host/team\\q.git\n\tpushurl = https://h/bad-push.git\n" +
			"[remote \"spaced\"]\n\turl = /srv/with\\ space.git\n",
			[]Remote{remote("bad", nil, "https://h/bad-push.git"),
				remote("origin", list("https://h/origin.git"))},
			list("line 4: unsupported escape \\q", "line 7: unsupported escape \\ ")},
		{"a value padded with many spaces is trimmed",
			"[remote \"p\"]\n\turl =" + pad + "\"https://h/p.git \"" + pad + "\n",
			[]Remote{remote("p", list("https://h/p.git "))}, nil},
		{"the line of a rejected value is its first line", "[remote \"m\"]\n" +
			"\turl = https://h/\\\nmore\\\n\"cut\n\turl = https://h/ok.git\n",
			[]Remote{remote("m", list("https://h/ok.git"))},
			list("line 2: unterminated quoted value")},
		{"a non-ASCII character after a backslash is named whole",
			"[remote \"u\"]\n\turl = /srv/\\\u00e9.git\n", nil,
			list("line 2: unsupported escape \\\u00e9")},
		{"a backslash ending the file is dropped", "[remote \"e\"]\n\turl = /srv/end\\",
			[]Remote{remote("e", list("/srv/end"))}, nil},
		{"an escaped subsection loses its backslashes",
			"[remote \"a\\\"b\\\\c\\d\"]\n\turl = /srv/x\n",
			[]Remote{remote("a\"b\\cd", list("/srv/x"))}, nil},
	}
}

func TestHeaderAndKeySeparatorsAreGitWhiteSpaceOnly(t *testing.T) {
	for _, space := range []string{"\u00a0", "\u2028", "\u0085", "\u3000"} {
		for _, text := range []string{
			"[" + space + "remote \"origin\"]\n\turl = https://h/r.git\n",
			"[remote" + space + "\"origin\"]\n\turl = https://h/r.git\n",
			"[remote \"origin\"" + space + "]\n\turl = https://h/r.git\n",
			"[remote \"origin\"]\n\turl" + space + "= https://h/r.git\n",
		} {
			if remotes, _ := ParseConfig(text); len(remotes) != 0 {
				t.Errorf("ParseConfig(%q) = %+v, want no remote", text, remotes)
			}
		}
	}
	for _, space := range []string{" ", "\t", " \t"} {
		text := "[remote" + space + "\"origin\"]\n\turl" + space + "= https://h/r.git\n"
		want := []Remote{remote("origin", list("https://h/r.git"))}
		if remotes, _ := ParseConfig(text); !reflect.DeepEqual(remotes, want) {
			t.Errorf("ParseConfig(%q) = %+v", text, remotes)
		}
	}
}

func TestParseHead(t *testing.T) {
	sha := "0123456789abcdef0123456789abcdef01234567"
	tests := []struct{ name, text, want string }{
		{"a branch may hold Unicode whitespace", "ref: refs/heads/team\u2028release\u00a0\n",
			"team\u2028release\u00a0"},
		{"a CRLF line end is removed", "ref: refs/heads/main\r\n", "main"},
		{"blank lines and ASCII padding are skipped", "\n  ref: refs/heads/second \t\n",
			"second"},
		{"the fixture HEAD names its branch", "ref: refs/heads/main\n", "main"},
		{"a branch may hold a slash", "ref: refs/heads/feature/x", "feature/x"},
		{"a detached HEAD gives its commit", sha + "\n", sha},
		{"another ref is given in full", "ref: refs/remotes/origin/main\n",
			"refs/remotes/origin/main"},
		{"empty text gives nothing", "", ""},
		{"blank lines give nothing", "\n\n", ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ParseHead(test.text); got != test.want {
				t.Errorf("ParseHead(%q) = %q, want %q", test.text, got, test.want)
			}
		})
	}
}

func FuzzParseConfig(fuzz *testing.F) {
	fuzz.Add(projectConfig)
	fuzz.Add("[remote \"a\\\"b\"] url = \"x\\\n\\ty\" ; c\n[remote.B]\npushurl=\\q\n")
	for _, test := range loadMixedConfigs(fuzz) {
		fuzz.Add(test.Config)
	}
	fuzz.Fuzz(func(t *testing.T, text string) {
		remotes, _ := ParseConfig(text)
		for _, found := range remotes {
			if found.URLs == nil || found.PushURLs == nil {
				t.Fatalf("remote %q has a nil list", found.Name)
			}
		}
		ParseHead(text)
	})
}
