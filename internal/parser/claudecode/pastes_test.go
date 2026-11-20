package claudecode

import (
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/dedup"
	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

func TestEachPasteCacheFileIsReadOncePerLine(t *testing.T) {
	const placeholders = 2000
	tests := []struct {
		name     string
		hashes   []string
		readable bool
		reads    int
	}{
		{"one file named by every placeholder", []string{"abc"}, true, 1},
		{"two files named in turn", []string{"abc", "def"}, true, 2},
		{"one file that cannot be read", []string{"abc"}, false, 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			members := make([]jsonvalue.Member, placeholders)
			var display, want strings.Builder
			for i := range placeholders {
				number, hash := strconv.Itoa(i), test.hashes[i%len(test.hashes)]
				members[i] = jsonvalue.Member{Key: number,
					Value: jsonvalue.NewObject(jsonvalue.Member{Key: "contentHash", Value: jsonvalue.NewString(hash)})}
				display.WriteString("[Pasted text #" + number + "] ")
				switch {
				case !test.readable:
					want.WriteString("[Pasted text #" + number + "] ")
				case i < len(test.hashes):
					want.WriteString("text of " + hash + " ")
				default:
					want.WriteString("[Pasted text #" + number + ", shown above] ")
				}
			}
			reads := 0
			var problems parser.Problems
			got := expandPastesFrom(display.String(), jsonvalue.NewObject(members...), &problems,
				func(hash string, read *parser.Problems) (string, bool) {
					reads++
					if !test.readable {
						read.Add(0, hash+".txt could not be read")
						return "", false
					}
					return "text of " + hash, true
				})
			wantProblems := 0
			if !test.readable {
				wantProblems = placeholders
			}
			if got != want.String() || reads != test.reads || len(problems) != wantProblems {
				t.Errorf("%d reads and %d problems, text as wanted %v; want %d reads and %d problems",
					reads, len(problems), got == want.String(), test.reads, wantProblems)
			}
		})
	}
}

// repeatedPaste is a line naming one paste size times, the paste size
// bytes long, kept inline or in paste-cache/.
type repeatedPaste struct {
	display string
	pasted  jsonvalue.Value
	paste   string
	// want is the exact length of the expanded text: the paste once and a
	// placeholder with its note at every later reference.
	want int
}

// inlinePaste returns a line whose one inline paste is named size times by
// its number.
func inlinePaste(size int) repeatedPaste {
	paste := strings.Repeat("x", size)
	entry := jsonvalue.NewObject(jsonvalue.Member{Key: "content", Value: jsonvalue.NewString(paste)})
	return repeatedPaste{display: strings.Repeat("[Pasted text #1]", size),
		pasted: jsonvalue.NewObject(jsonvalue.Member{Key: "1", Value: entry}), paste: paste,
		want: size + (size-1)*len("[Pasted text #1, shown above]")}
}

// hashedPaste returns a line of size placeholders, each its own number,
// whose entries all name one paste-cache file.
func hashedPaste(size int) repeatedPaste {
	paste := strings.Repeat("y", size)
	members := make([]jsonvalue.Member, size)
	var display strings.Builder
	want := size
	for i := range size {
		number := strconv.Itoa(i)
		members[i] = jsonvalue.Member{Key: number,
			Value: jsonvalue.NewObject(jsonvalue.Member{Key: "contentHash", Value: jsonvalue.NewString("h1")})}
		display.WriteString("[Pasted text #" + number + "]")
		if i > 0 {
			want += len("[Pasted text #" + number + ", shown above]")
		}
	}
	return repeatedPaste{display: display.String(), pasted: jsonvalue.NewObject(members...), paste: paste,
		want: want}
}

func TestAPasteNamedManyTimesInALineIsWrittenOnce(t *testing.T) {
	builders := []struct {
		name  string
		build func(size int) repeatedPaste
	}{{"inline", inlinePaste}, {"in the paste cache", hashedPaste}}
	for _, builder := range builders {
		for _, size := range []int{500, 2000} {
			line := builder.build(size)
			var problems parser.Problems
			got := expandPastesFrom(line.display, line.pasted, &problems,
				func(string, *parser.Problems) (string, bool) { return line.paste, true })
			if len(got) != line.want || strings.Count(got, line.paste) != 1 || !strings.HasPrefix(got, line.paste) {
				t.Errorf("%s, %d references: %d bytes holding the paste %d times, want %d bytes holding it once, first",
					builder.name, size, len(got), strings.Count(got, line.paste), line.want)
			}
		}
	}
}

func TestALaterPlaceholderKeepsItsLineCountAndAMissingPasteStaysAsWritten(t *testing.T) {
	pasted := jsonvalue.NewObject(
		jsonvalue.Member{Key: "1", Value: jsonvalue.NewObject(jsonvalue.Member{Key: "content",
			Value: jsonvalue.NewString("P")})},
		jsonvalue.Member{Key: "2", Value: jsonvalue.NewObject(jsonvalue.Member{Key: "contentHash",
			Value: jsonvalue.NewString("gone")})})
	display := "a [Pasted text #1 +3 lines] b [Pasted text #1 +3 lines] c [Pasted text #2] d [Pasted text #2]"
	want := "a P b [Pasted text #1 +3 lines, shown above] c [Pasted text #2] d [Pasted text #2]"
	var problems parser.Problems
	got := expandPastesFrom(display, pasted, &problems, func(string, *parser.Problems) (string, bool) { return "", false })
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// historyRows writes a history.jsonl of the given lines, with a paste-cache
// file h1.txt holding PASTE-B, and returns the rows the parser gives it.
func historyRows(t *testing.T, lines ...string) []timeline.Row {
	t.Helper()
	home := t.TempDir()
	const rel = ".claude/history.jsonl"
	cache := filepath.Join(home, ".claude", "paste-cache")
	if err := osfile.MkdirAll(cache, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(filepath.Join(cache, "h1.txt"), []byte("PASTE-B"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, filepath.FromSlash(rel))
	if err := osfile.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := &input.Artifact{DiskPath: path, Original: "/home/u/" + rel, Rel: rel, Agent: "claude-code",
		Home: &input.Home{DiskPath: home, Original: "/home/u", User: "u", Host: "h"}, Manifest: jsonvalue.NewNull()}
	var rows []timeline.Row
	if err := New().Parse(artifact, parser.Options{}, func(row timeline.Row) { rows = append(rows, row) }); err != nil {
		t.Fatal(err)
	}
	return rows
}

// A paste is written in full once per history line, so the row that holds
// a pointer always holds the text it points at, and deduplication, which
// keeps or drops whole rows, never keeps a pointer whose text it dropped.
func TestAKeptHistoryRowHoldsEveryPasteItsPointersName(t *testing.T) {
	rows := historyRows(t,
		`{"display": "[Pasted text #1] again [Pasted text #1]", "pastedContents": {"1": {"content": "PASTE-A"}}, `+
			`"timestamp": 1790000001000, "sessionId": "s", "project": "/p"}`,
		`{"display": "[Pasted text #1] then [Pasted text #2]", "pastedContents": {"1": {"contentHash": "h1"}, `+
			`"2": {"contentHash": "h1"}}, "timestamp": 1790000002000, "sessionId": "s", "project": "/p"}`,
		`{"display": "[Pasted text #1]", "pastedContents": {"1": {"contentHash": "h1"}}, `+
			`"timestamp": 1790000003000, "sessionId": "s", "project": "/p"}`)
	var texts []string
	for _, row := range rows {
		texts = append(texts, row.Text)
	}
	want := []string{"PASTE-A again [Pasted text #1, shown above]", "PASTE-B then [Pasted text #2, shown above]",
		"PASTE-B"}
	if !slices.Equal(texts, want) {
		t.Fatalf("got %q, want %q", texts, want)
	}
	var primary []timeline.Row
	secondary := make([]dedup.Secondary, len(rows))
	for i, row := range rows {
		record := row
		record.SourceFile = "/home/u/.claude/projects/p/s.jsonl"
		record.Text = []string{"PASTE-A again PASTE-A", "PASTE-B then PASTE-B", "PASTE-B"}[i]
		primary = append(primary, record)
		secondary[i] = dedup.Secondary{Row: row, Tolerance: 2}
	}
	kept, _ := dedup.Deduplicate(primary, secondary, dedup.Hooks{"claude-code": New().DedupHooks()})
	pointer := regexp.MustCompile(`\[Pasted text #[0-9]+[^\]]*, shown above\]`)
	for _, row := range kept[len(primary):] {
		before, _, found := strings.Cut(row.Text, ", shown above]")
		if found && !strings.Contains(before, "PASTE-") {
			t.Errorf("kept row %q points at a paste it does not hold", row.Text)
		}
		if found && !pointer.MatchString(row.Text) {
			t.Errorf("kept row %q has a pointer of another form", row.Text)
		}
	}
}
