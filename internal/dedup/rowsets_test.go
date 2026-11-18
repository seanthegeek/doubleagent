package dedup

import (
	"encoding/json"
	"maps"
	"math/rand/v2"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// generator builds random row sets whose fields collide often.
type generator struct {
	random *rand.Rand
	line   int
}

// pick returns one of options at random.
func (g *generator) pick(options ...string) string {
	return options[g.random.IntN(len(options))]
}

// moment returns a timestamp on a coarse grid around one time, or "".
func (g *generator) moment() string {
	if g.random.IntN(6) == 0 {
		return ""
	}
	millis := int64(1_790_000_000_000) + int64(g.random.IntN(12))*500
	if g.random.IntN(8) == 0 {
		millis += int64(g.random.IntN(40_000))
	}
	return timeutil.FormatUTC(time.UnixMilli(millis))
}

// row returns a random row with a fresh source_line, which identifies it.
func (g *generator) row(files ...string) timeline.Row {
	g.line++
	row := timeline.Row{TimestampUTC: g.moment(), Host: "h1", User: g.pick("alice", "alice",
		"alice", "bob"), Agent: g.pick("claude-code", "hooked", "hooked"),
		SessionID: g.pick("", "s1", "s1", "s1", "s2"), ProjectPath: g.pick("", "/p1", "/p1",
			"/p2"), TurnType: g.pick("user", "user", "user", "tool_use", "system"),
		Text: g.pick("a", "a", "/x", "cmd: x", "parser: bad"), SourceFile: g.pick(files...),
		SourceLine: g.line}
	if row.TurnType == timeline.TurnToolUse {
		row.ToolUseID = g.pick("", "call_1", "call_2")
	}
	switch g.random.IntN(8) {
	case 0:
		row.Model = "never"
	case 1:
		row.Model = "at " + g.moment()
	}
	return row
}

// copyOf returns a secondary copy of a row: another file, a fresh
// source_line, its time often moved and its scope sometimes dropped.
func (g *generator) copyOf(row timeline.Row, files ...string) timeline.Row {
	g.line++
	row.SourceFile, row.SourceLine = g.pick(files...), g.line
	if moment, ok := timeutil.EpochMillis(row.TimestampUTC); ok && g.random.IntN(3) > 0 {
		row.TimestampUTC = timeutil.FormatUTC(time.UnixMilli(moment +
			int64(g.random.IntN(9)-4)*500))
	}
	switch g.random.IntN(7) {
	case 0:
		row.SessionID = ""
	case 3:
		row.ProjectPath = g.pick("/p1", "/p2")
	case 1:
		row.ProjectPath = ""
	case 2:
		row.TimestampUTC = ""
	}
	return row
}

// randomCase returns a random primary and secondary row set, many
// secondary rows copies of others.
func (g *generator) randomCase() ([]timeline.Row, []Secondary) {
	var primary []timeline.Row
	for range g.random.IntN(8) {
		primary = append(primary, g.row("/t/one.jsonl", "/t/two.jsonl"))
	}
	var secondary []Secondary
	files := []string{"/h/a.jsonl", "/h/b.jsonl", "/h/c.jsonl"}
	for range 1 + g.random.IntN(12) {
		tolerance := []float64{2, 2, 0.5, 2.5, 600}[g.random.IntN(5)]
		row := g.row(files...)
		switch pool := len(primary) + len(secondary); {
		case pool > 0 && g.random.IntN(3) > 0:
			source := g.random.IntN(pool)
			if source < len(primary) {
				row = g.copyOf(primary[source], files...)
			} else {
				row = g.copyOf(secondary[source-len(primary)].Row, files...)
			}
		}
		secondary = append(secondary, Secondary{row, tolerance})
	}
	return primary, secondary
}

// mixedRowSet is one case of testdata/rowsets.json: primary and secondary
// rows built by generator, deduplicated with sampleHooks, with the source
// lines kept in output order, those of them recovered from a secondary
// source, and the secondary rows dropped per agent.
type mixedRowSet struct {
	Name      string         `json:"name"`
	Primary   []timeline.Row `json:"primary"`
	Secondary []struct {
		Row       timeline.Row `json:"row"`
		Tolerance float64      `json:"tolerance"`
	} `json:"secondary"`
	Kept      []int          `json:"kept"`
	Recovered []int          `json:"recovered"`
	Dropped   map[string]int `json:"dropped"`
}

func TestMixedRowSets(t *testing.T) {
	data, err := os.ReadFile("testdata/rowsets.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []mixedRowSet
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, test := range cases {
		t.Run(test.Name, func(t *testing.T) {
			secondary := make([]Secondary, len(test.Secondary))
			for i, item := range test.Secondary {
				secondary[i] = Secondary{item.Row, item.Tolerance}
			}
			kept, dropped := Deduplicate(test.Primary, secondary, sampleHooks())
			lines, recovered := []int{}, []int{}
			for _, row := range kept {
				lines = append(lines, row.SourceLine)
				if row.Recovered == "yes" {
					recovered = append(recovered, row.SourceLine)
				}
			}
			if !slices.Equal(lines, test.Kept) || !slices.Equal(recovered, test.Recovered) {
				t.Errorf("kept %v, recovered %v; want %v, %v", lines, recovered, test.Kept,
					test.Recovered)
			}
			if !maps.Equal(dropped, test.Dropped) {
				t.Errorf("dropped %v, want %v", dropped, test.Dropped)
			}
		})
	}
}

func TestEveryRowIsKeptOrCountedAsDropped(t *testing.T) {
	source := &generator{random: rand.New(rand.NewPCG(19, 23))}
	for range 3000 {
		primary, secondary := source.randomCase()
		kept, dropped := Deduplicate(primary, secondary, sampleHooks())
		total := len(kept)
		for _, count := range dropped {
			total += count
		}
		if total != len(primary)+len(secondary) {
			t.Fatalf("%d rows in, %d kept and %v dropped", len(primary)+len(secondary),
				len(kept), dropped)
		}
		if len(kept) < len(primary) || !slices.Equal(kept[:len(primary)], primary) {
			t.Fatalf("the primary rows are not kept first and unchanged: %v", kept)
		}
		lastIndex := -1
		for _, row := range kept[len(primary):] {
			index := slices.IndexFunc(secondary, func(item Secondary) bool {
				return item.Row.SourceLine == row.SourceLine
			})
			if index <= lastIndex {
				t.Fatalf("secondary source line %d is out of order or not a secondary row",
					row.SourceLine)
			}
			lastIndex = index
			want := "yes"
			if row.TurnType == timeline.TurnSystem {
				want = ""
			}
			if row.Recovered != want {
				t.Fatalf("source line %d recovered %q, want %q", row.SourceLine, row.Recovered,
					want)
			}
		}
	}
}
