//go:build parity

package replay

import (
	"slices"
	"strings"
	"testing"
)

func TestTheTableNumbersEachDifferenceOnceFrom51(t *testing.T) {
	seen := map[int]bool{}
	for i, entry := range Differences() {
		if entry.Number != 51+i || seen[entry.Number] {
			t.Errorf("entry %d is numbered %d", i, entry.Number)
		}
		seen[entry.Number] = true
		if (entry.Text != nil) == (entry.Excludes != nil) && entry.Rows == nil {
			t.Errorf("difference %d needs exactly one kind of normaliser", entry.Number)
		}
	}
}

func TestTextDifferencesRewriteOnlyTheirShape(t *testing.T) {
	tests := []struct {
		name   string
		number int
		text   string
		want   string
		found  int
	}{
		{"51 replaces a usage message to the end", 51,
			"usage: doubleagent [-h]\ndoubleagent: error: bad\n", "{usage}\n", 1},
		{"51 leaves a text without usage alone", 51, "error: no such file\n",
			"error: no such file\n", 0},
		{"52 rewrites a JSON decoder detail in a parser row", 52,
			"parser: unparseable line (Expecting ',' delimiter: line 1 column 5 (char 4)): {\"a\" 1",
			"parser: unparseable line ({json error}): {\"a\" 1", 1},
		{"52 rewrites a SQLite error up to a JSON quote", 52,
			`{"text":"parser: SQLite error: file is not a database","source_line":0}`,
			`{"text":"parser: SQLite error: {sqlite error}","source_line":0}`, 1},
		{"52 rewrites a system error of a problem line", 52,
			"problem:   /x/y: could not be read (Permission denied)\n",
			"problem:   /x/y: could not be read ({system error})\n", 1},
		{"52 rewrites an XML parse error", 52,
			"parser: cody_history.xml did not parse; read 0 chat(s) (not well-formed (invalid token): line 1, column 5)",
			"parser: cody_history.xml did not parse; read 0 chat(s) ({xml error})", 1},
		{"52 rewrites a TOML error", 52,
			"parser: unreadable config.toml: Invalid value (at line 2, column 7)",
			"parser: unreadable config.toml: {toml error}", 1},
		{"52 leaves the analyzer's own wording alone", 52,
			"parser: unparseable line (not a JSON object): [1]",
			"parser: unparseable line (not a JSON object): [1]", 0},
		{"52 leaves an assistant row that quotes an error alone", 52,
			"SQLite error: file is not a database", "SQLite error: file is not a database", 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, found := difference(t, test.number).Text(test.text)
			if got != test.want || found != test.found {
				t.Errorf("got %q (%d places), want %q (%d)", got, found, test.want, test.found)
			}
		})
	}
}

func TestExcludingDifferencesSeeOnlyTheirShape(t *testing.T) {
	tests := []struct {
		name   string
		number int
		text   string
		want   bool
	}{
		{"53 sees a lookahead", 53, "timeline\n/in\n--match\nfoo(?=bar)", true},
		{"53 sees a backreference in the joined form", 53, "--match=(a)\\1", true},
		{"53 sees a pattern Go cannot compile", 53, "--match\n[z-a]", true},
		{"53 passes a pattern both read alike", 53, "--match\nfoo|bar", false},
		{"54 sees an ISO week date", 54, "--since\n2026-W40", true},
		{"54 sees an ordinal date", 54, "--until\n2026-274", true},
		{"54 sees a non-ASCII digit", 54, "--since\n٢٠٢٦-10-01", true},
		{"54 passes a calendar date", 54, "--since\n2026-10-01", false},
		{"56 sees SQLite without FTS5", 56, "parser: SQLite error: no such module: fts5", true},
		{"56 passes another SQLite error", 56, "parser: SQLite error: disk I/O error", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := difference(t, test.number).Excludes(test.text); got != test.want {
				t.Errorf("got %v, want %v", got, test.want)
			}
		})
	}
}

func TestPlannerOrderSortsOnlyRunsOfItsRows(t *testing.T) {
	rows := []Row{{Text: "user prompt"}, {Text: "archived worktree: b"},
		{Text: "archived worktree: a"}, {Text: "assistant"}, {Text: "archived worktree: c"}}
	got, found := difference(t, 55).Rows(rows)
	texts := []string{}
	for _, row := range got {
		texts = append(texts, string(row.Text))
	}
	want := []string{"user prompt", "archived worktree: a", "archived worktree: b", "assistant",
		"archived worktree: c"}
	if !slices.Equal(texts, want) || found != 3 {
		t.Errorf("got %q (%d rows), want %q (3)", texts, found, want)
	}
	if rows[1].Text != "archived worktree: b" {
		t.Error("the input rows were reordered in place")
	}
}

func TestANamedDifferenceThatFindsNothingFailsTheCase(t *testing.T) {
	tests := []struct {
		name  string
		check func(reporter *recordingTB)
		fails bool
	}{
		{"a text difference with its shape passes", func(reporter *recordingTB) {
			NormaliseTexts(reporter, "n", []int{51}, "usage: x\n", "usage: y\n")
		}, false},
		{"a text difference without its shape fails", func(reporter *recordingTB) {
			NormaliseTexts(reporter, "n", []int{51}, "x\n", "x\n")
		}, true},
		{"a row difference without its shape fails", func(reporter *recordingTB) {
			NormaliseRows(reporter, "n", []int{52}, []Row{{Text: "hello"}}, nil)
		}, true},
		{"an excluding difference that shows passes", func(reporter *recordingTB) {
			Excluded(reporter, "n", []int{54}, "--since\n2026-W01")
		}, false},
		{"an excluding difference that does not show fails", func(reporter *recordingTB) {
			Excluded(reporter, "n", []int{54}, "--since\n2026-01-01")
		}, true},
		{"a difference of another kind is left to its own call", func(reporter *recordingTB) {
			NormaliseTexts(reporter, "n", []int{54}, "x", "x")
		}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reporter := &recordingTB{TB: t}
			test.check(reporter)
			failed := len(reporter.errors) > 0
			if failed != test.fails {
				t.Errorf("failed %v, want %v: %s", failed, test.fails,
					strings.Join(reporter.errors, "; "))
			}
		})
	}
}

func TestNormaliseRowsRewritesBothSides(t *testing.T) {
	want := []Row{{Text: "parser: SQLite error: file is not a database"}}
	got := []Row{{Text: "parser: SQLite error: malformed database: bad header"}}
	reporter := &recordingTB{TB: t}
	normalWant, normalGot := NormaliseRows(reporter, "n", []int{52}, want, got)
	if !DiffRows(reporter, "n", normalWant, normalGot) || len(reporter.errors) > 0 {
		t.Errorf("rows still differ: %v", reporter.errors)
	}
	if want[0].Text != "parser: SQLite error: file is not a database" {
		t.Error("the caller's rows were changed")
	}
}
