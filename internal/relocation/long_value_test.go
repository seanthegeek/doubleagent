package relocation

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/syspath"
	"github.com/seanthegeek/doubleagent/internal/users"
)

// longReason is the reason of a raw value longer than maxValueBytes.
const longReason = "longer than 4096 bytes"

// padded returns prefix and suffix around the letter a repeated until the
// whole is size bytes long.
func padded(prefix, suffix string, size int) string {
	return prefix + strings.Repeat("a", size-len(prefix)-len(suffix)) + suffix
}

// longCase is one value of a reader's test: the text after `=` as the reader
// sees it, and the outcome wanted.
type longCase struct {
	name, raw, value, reason string
}

// checkLong runs read on one assignment of each case and compares the one
// assignment it returns.
func checkLong(t *testing.T, cases []longCase, read func(raw string) []assignment) {
	t.Helper()
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got := read(test.raw)
			if len(got) != 1 || got[0].Value != test.value || got[0].Reason != test.reason {
				t.Fatalf("got %.200v, want value %.200q reason %.200q", got, test.value, test.reason)
			}
		})
	}
}

func TestEnvironmentFileRefusesALongValueBeforeAnyOtherReason(t *testing.T) {
	cases := []longCase{
		{"4096 bytes are kept", padded("/", "", 4096), padded("/", "", 4096), ""},
		{"4096 bytes with a hash are judged by the hash", padded("/", "#", 4096), "", "holds a #"},
		{"4096 bytes quoted are judged as quoted", padded("/", `"`, 4096), "", "quoted"},
		{"4097 bytes", padded("/", "", 4097), "", longReason},
		{"4097 bytes with a hash", padded("/", "#", 4097), "", longReason},
		{"4097 bytes with a quote", padded("/", `"`, 4097), "", longReason},
		{"4097 bytes in quotes", padded(`"/`, `"`, 4097), "", longReason},
		{"4097 bytes ending in a backslash", padded("/", `\`, 4097), "", longReason},
		{"4097 bytes with a dollar name", padded("$", "", 4097), "", longReason},
	}
	checkLong(t, cases, func(raw string) []assignment {
		got, _ := resolveEnvironment(strings.NewReader("CODEX_HOME="+raw+"\n"), testNames())
		return got
	})
	t.Run("a NUL byte before a long value", func(t *testing.T) {
		text := "A=\x00\nCODEX_HOME=" + padded("/", "", 4097) + "\n"
		got, _ := resolveEnvironment(strings.NewReader(text), testNames())
		if len(got) != 1 || got[0].Reason != longReason {
			t.Errorf("got %.200v", got)
		}
	})
	t.Run("a long value on a continued line", func(t *testing.T) {
		text := "A=1\\ \nCODEX_HOME=" + padded("/", "", 4097) + "\n"
		got, _ := resolveEnvironment(strings.NewReader(text), testNames())
		if len(got) != 1 || got[0].Reason != longReason {
			t.Errorf("got %.200v", got)
		}
	})
}

func TestEnvironmentDRefusesALongValueBeforeAnyOtherReason(t *testing.T) {
	reference := func(size int) string { return "${" + strings.Repeat("A", size-3) + "}" }
	cases := []longCase{
		{"4096 bytes are kept", padded("/", "", 4096), padded("/", "", 4096), ""},
		{"4096 bytes of a braced reference name that quotes it",
			reference(4096), "", "references $" + strings.Repeat("A", 4093)},
		{"4096 bytes of a plain reference", "$" + strings.Repeat("A", 4095), "",
			"references $" + strings.Repeat("A", 4095)},
		{"4096 bytes quoted are judged as quoted", padded("'/", "", 4096), "", "quoted"},
		{"4097 bytes", padded("/", "", 4097), "", longReason},
		{"4097 bytes of a braced reference", reference(4097), "", longReason},
		{"4097 bytes of a plain reference", "$" + strings.Repeat("A", 4096), "", longReason},
		{"4097 bytes in double quotes", `"` + reference(4095) + `"`, "", longReason},
		{"4097 bytes in single quotes", padded("'/", "'", 4097), "", longReason},
		{"4097 bytes with a stray quote", padded("/", `"`, 4097), "", longReason},
		{"4097 bytes with a dollar and a digit", padded("/$1", "", 4097), "", longReason},
		{"20000 bytes of a braced reference", reference(20000), "", longReason},
		{"4097 bytes that are an unclosed quote", padded(`"/`, "", 4097), "", longReason},
	}
	checkLong(t, cases, func(raw string) []assignment {
		got, _ := resolveEnvironmentD(strings.NewReader("CODEX_HOME="+raw+"\n"), testNames(), "/h")
		return got
	})
	t.Run("a long value on a continued line", func(t *testing.T) {
		text := "CODEX_HOME=" + padded("/", "", 4000) + "\\\n" + padded("", "", 200) + "\n"
		got, _ := resolveEnvironmentD(strings.NewReader(text), testNames(), "")
		if len(got) != 1 || got[0].Reason != longReason {
			t.Errorf("got %.200v", got)
		}
	})
	t.Run("a long value after a NUL byte", func(t *testing.T) {
		text := "A=\x00\nCODEX_HOME=" + padded("/", "", 4097) + "\n"
		got, _ := resolveEnvironmentD(strings.NewReader(text), testNames(), "")
		if len(got) != 1 || got[0].Reason != longReason {
			t.Errorf("got %.200v", got)
		}
	})
}

func TestSelectValueRefusesALongValueWhateverTheReader(t *testing.T) {
	long := padded("/", "\n", 5000)
	values := fileValues{Assignments: []assignment{
		{Line: 0, Name: "CODEX_HOME", Value: long, Reason: "holds a newline"}}}
	got, reason := values.index().selectValue(0, "CODEX_HOME", "")
	if got != "" || reason != longReason {
		t.Errorf("selectValue = %.100q, %q; want %q", got, reason, longReason)
	}
}

func TestRegistryHintRefusesALongRawValueBeforeAnyOtherReason(t *testing.T) {
	shrinking := strings.Repeat("%A%", 1400)
	cases := []struct {
		name, text string
		expandable bool
		want       string
	}{
		{"4096 bytes are kept", padded(`C:\`, "", 4096), false, ""},
		{"4097 bytes", padded(`C:\`, "", 4097), false, longReason},
		{"4097 bytes with a percent name", padded("%", "%", 4097), false, longReason},
		{"4097 bytes with a quote", padded(`C:\`, `"`, 4097), false, longReason},
		{"4097 bytes that expand to fewer", shrinking, true, longReason},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			name := "CODEX_HOME"
			if test.expandable {
				name += "*"
			}
			fake := fakeRegistry{keys: map[registryKey]RegistryStrings{
				{localMachine, systemEnvironmentKey}: stringValues("A", "x", name, test.text)}}
			log := &lineLog{}
			search := registrySearch(t, fake, nil, nil, log)
			if err := search.registryPass(); err != nil {
				t.Fatal(err)
			}
			hint := search.result.Hints[len(search.result.Hints)-1]
			line := log.lines[len(log.lines)-1]
			if test.want == "" && hint.Value != padded(`C:\`, "", 4096) {
				t.Errorf("value %.100q, want it kept", hint.Value)
			}
			if test.want != "" && (hint.Value != "" || !strings.HasSuffix(line, test.want)) {
				t.Errorf("hint %.100q, line %.200q; want reason %q", hint.Value, line, test.want)
			}
			if test.want != "" && len(line) > 300 {
				t.Errorf("a log line of %d bytes", len(line))
			}
		})
	}
}

func TestSearchLogsNoLongValue(t *testing.T) {
	root := syspath.FromNative(t.TempDir())
	long := strings.Repeat("A", 20000)
	writeTree(t, root, map[string]string{
		"etc/environment.d/10.conf":                   "CODEX_HOME=${" + long + "}\n",
		"etc/environment":                             "CODEX_HOME=/" + long + "#\n",
		"home/alice/.config/environment.d/20.conf":    "CODEX_HOME=$" + long + "\n",
		"home/alice/.config/environment.d/30.conf":    `CODEX_HOME="/` + long + "\n",
		"home/alice/.config/environment.d/40.conf":    "CODEX_HOME=/" + long + "\\\nx\n",
		"home/alice/.config/environment.d/short.conf": "CODEX_HOME=/opt/short\n",
	})
	log := &lineLog{}
	deps := searchDeps(t, root, log, &[]string{})
	deps.Homes = []users.Home{{User: "alice", Path: root + "/home/alice"}}
	result, err := Search(context.Background(), deps)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range log.lines {
		if len(line) > 400 {
			t.Errorf("a log line of %d bytes: %.120q", len(line), line)
		}
	}
	for _, hint := range result.Hints {
		if len(hint.Source)+len(hint.Variable)+len(hint.Value) > 1000 {
			t.Errorf("a hint of %d bytes", len(hint.Source)+len(hint.Variable)+len(hint.Value))
		}
	}
	longReasons := 0
	for _, line := range log.lines {
		if strings.HasSuffix(line, longReason) {
			longReasons++
		}
	}
	if longReasons < 4 || !slices.ContainsFunc(log.lines, func(line string) bool {
		return strings.HasSuffix(line, "= "+syspath.ToNative(root+"/opt/short"))
	}) {
		t.Errorf("log:\n%.2000s", strings.Join(log.lines, "\n"))
	}
}
