package claudecode

import (
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/dedup"
	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// expandedLine returns a history line's text as the output writes it, the
// paste-cache file of each hash read through readCache once.
func expandedLine(display string, pasted jsonvalue.Value, problems *parserkit.Problems,
	readCache func(hash string, read *parserkit.Problems) (string, bool)) string {
	files := map[string]*paste{}
	var out timeline.TextBuilder
	expandPastesFrom(&out, display, pasted, problems, func(hash string) *paste {
		each, ok := files[hash]
		if !ok {
			each = &paste{}
			var text string
			text, each.found = readCache(hash, &each.problems)
			each.shared = timeline.NewSharedText(text)
			files[hash] = each
		}
		return each
	})
	rows := make([]timeline.Row, 1)
	out.Set(&rows[0])
	timeline.Resolve(rows)
	return rows[0].Text
}

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
			var problems parserkit.Problems
			got := expandedLine(display.String(), jsonvalue.NewObject(members...), &problems,
				func(hash string, read *parserkit.Problems) (string, bool) {
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
			if got != strings.TrimSpace(want.String()) || reads != test.reads || len(problems) != wantProblems {
				t.Errorf("%d reads and %d problems, text as wanted %v; want %d reads and %d problems",
					reads, len(problems), got == strings.TrimSpace(want.String()), test.reads, wantProblems)
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
			var problems parserkit.Problems
			got := expandedLine(line.display, line.pasted, &problems,
				func(string, *parserkit.Problems) (string, bool) { return line.paste, true })
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
	var problems parserkit.Problems
	got := expandedLine(display, pasted, &problems, func(string, *parserkit.Problems) (string, bool) { return "", false })
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// historyRowsWith is historyRows with h1.txt holding paste.
func historyRowsWith(t *testing.T, paste string, lines ...string) []timeline.Row {
	t.Helper()
	home := t.TempDir()
	const rel = ".claude/history.jsonl"
	cache := filepath.Join(home, ".claude", "paste-cache")
	if err := osfile.MkdirAll(cache, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(filepath.Join(cache, "h1.txt"), []byte(paste), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, filepath.FromSlash(rel))
	if err := osfile.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := &input.Artifact{DiskPath: path, Original: "/home/u/" + rel, Rel: rel, Agent: "claude-code",
		Home: &input.Home{DiskPath: home, Original: "/home/u", User: "u", Host: "h"}, Manifest: jsonvalue.NewNull()}
	var rows []timeline.Row
	if err := New().Parse(artifact, parserkit.Options{}, func(row timeline.Row) { rows = append(rows, row) }); err != nil {
		t.Fatal(err)
	}
	return rows
}

// historyRows writes a history.jsonl of the given lines, with a paste-cache
// file h1.txt holding PASTE-B, and returns the rows the parser gives it.
func historyRows(t *testing.T, lines ...string) []timeline.Row {
	t.Helper()
	return historyRowsWith(t, "PASTE-B", lines...)
}

// deduplicatedHistory runs the history rows through deduplication against
// the transcript records given, by index of the row they record, and
// returns the texts of the rows kept as the output writes them.
func deduplicatedHistory(rows []timeline.Row, records map[int]string) []string {
	var primary []timeline.Row
	secondary := make([]dedup.Secondary, len(rows))
	for i, row := range rows {
		secondary[i] = dedup.Secondary{Row: row, Tolerance: 2}
		if text, ok := records[i]; ok {
			record := row
			record.SourceFile, record.Text, record.Shared = "/home/u/.claude/projects/p/s.jsonl", text, nil
			primary = append(primary, record)
		}
	}
	kept, _ := dedup.Deduplicate(primary, secondary, dedup.Hooks{"claude-code": New().DedupHooks()})
	kept = kept[len(primary):]
	timeline.Resolve(kept)
	texts := make([]string, len(kept))
	for i, row := range kept {
		texts[i] = row.Text
	}
	return texts
}

// A history line matches its transcript record with its pastes in full,
// and the line that then holds a paste is the first one kept.
func TestTheFirstKeptHistoryLineHoldsAPasteWhoseFirstLineIsADuplicate(t *testing.T) {
	line := func(display string, second int) string {
		return `{"display": "` + display + `", "pastedContents": {"1": {"contentHash": "h1"}}, ` +
			`"timestamp": 179000000` + strconv.Itoa(second) + `000, "sessionId": "s", "project": "/p"}`
	}
	rows := historyRows(t, line("one [Pasted text #1] and [Pasted text #1]", 1),
		line("two [Pasted text #1]", 2), line("three [Pasted text #1]", 3))
	got := deduplicatedHistory(rows[:3], map[int]string{0: "one PASTE-B and PASTE-B"})
	want := []string{"two PASTE-B",
		"three [Pasted text #1, shown above in /home/u/.claude/history.jsonl line 2]"}
	if !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestHistoryLinesNamingOnePasteGrowWithThePasteNotWithTheirProduct(t *testing.T) {
	size := func(n int) int {
		lines := make([]string, n)
		for i := range lines {
			lines[i] = `{"display": "line ` + strconv.Itoa(i) + ` [Pasted text #1]", ` +
				`"pastedContents": {"1": {"contentHash": "h1"}}, "timestamp": 1790000001000}`
		}
		rows := historyRowsWith(t, strings.Repeat("p", 10*n), lines...)
		timeline.Resolve(rows)
		total := 0
		for _, row := range rows {
			total += len(row.Text)
		}
		return total
	}
	// Twice the lines naming a paste twice as long gives about twice the
	// bytes; the paste in every line would give four times as many.
	small, large := size(500), size(1000)
	if small < 10*500 || large > 5*small/2 {
		t.Errorf("%d bytes for 500 lines, %d for 1000", small, large)
	}
}
