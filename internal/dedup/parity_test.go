//go:build parity

package dedup

import (
	"bytes"
	"fmt"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/pyjson"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// oracleScript reads one JSON case per line, {"primary": [row...],
// "secondary": [[row, tolerance]...]}, runs the analyzer's deduplicate on
// it with the hooks of the agent "hooked" registered, and prints the kept
// rows as source_line:recovered, then " | " and the drops per agent. The
// hooks mirror sampleHooks. The analyzer's directory is the first argument.
const oracleScript = `import json, sys
sys.path.insert(0, sys.argv[1])
from doubleagent import dedup
from doubleagent.model import Row
from doubleagent.timeutil import epoch_ms

def form(row):
    if row.turn_type == "system" and row.text.startswith("cmd: "):
        return "user", "/" + row.text[5:]
    return row.turn_type, row.text

def window(row):
    return 30.0 if row.turn_type == "user" and row.text.startswith("/") else None

def anchor(row):
    if row.model == "never":
        return dedup.NO_MATCH
    if row.model.startswith("at "):
        return epoch_ms(row.model[3:])
    return None

dedup.MATCH_FORMS["hooked"] = form
dedup.MATCH_TOLERANCES["hooked"] = window
dedup.MATCH_TIMES["hooked"] = anchor
for line in sys.stdin:
    case = json.loads(line)
    primary = [Row(**fields) for fields in case["primary"]]
    secondary = [(Row(**fields), tolerance) for fields, tolerance in case["secondary"]]
    kept, counts = dedup.deduplicate(primary, secondary)
    print(" ".join("%d:%s" % (row.source_line, row.recovered) for row in kept), "|",
          " ".join("%s=%d" % (agent, counts[agent]) for agent in sorted(counts) if counts[agent]))
`

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

// rowValue returns a row as the JSON object the oracle builds a Row from.
func rowValue(row timeline.Row) pyjson.Value {
	text := pyjson.NewString
	return pyjson.NewObject(pyjson.Member{Key: "timestamp_utc", Value: text(row.TimestampUTC)},
		pyjson.Member{Key: "host", Value: text(row.Host)},
		pyjson.Member{Key: "user", Value: text(row.User)},
		pyjson.Member{Key: "agent", Value: text(row.Agent)},
		pyjson.Member{Key: "session_id", Value: text(row.SessionID)},
		pyjson.Member{Key: "project_path", Value: text(row.ProjectPath)},
		pyjson.Member{Key: "turn_type", Value: text(row.TurnType)},
		pyjson.Member{Key: "model", Value: text(row.Model)},
		pyjson.Member{Key: "tool_use_id", Value: text(row.ToolUseID)},
		pyjson.Member{Key: "text", Value: text(row.Text)},
		pyjson.Member{Key: "source_file", Value: text(row.SourceFile)},
		pyjson.Member{Key: "source_line", Value: pyjson.NewInt(int64(row.SourceLine))})
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

// encodeCase returns one case as the JSON line the oracle reads.
func encodeCase(primary []timeline.Row, secondary []Secondary) string {
	primaryValues := []pyjson.Value{}
	for _, row := range primary {
		primaryValues = append(primaryValues, rowValue(row))
	}
	secondaryValues := []pyjson.Value{}
	for _, item := range secondary {
		secondaryValues = append(secondaryValues, pyjson.NewArray(rowValue(item.Row),
			pyjson.NewFloat(item.Tolerance)))
	}
	return pyjson.Compact(pyjson.NewObject(
		pyjson.Member{Key: "primary", Value: pyjson.NewArray(primaryValues...)},
		pyjson.Member{Key: "secondary", Value: pyjson.NewArray(secondaryValues...)}))
}

// summary writes a result as the oracle prints it.
func summary(kept []timeline.Row, counts map[string]int) string {
	parts := make([]string, len(kept))
	for i, row := range kept {
		parts[i] = strconv.Itoa(row.SourceLine) + ":" + row.Recovered
	}
	agents := slices.Sorted(func(yield func(string) bool) {
		for agent := range counts {
			if !yield(agent) {
				return
			}
		}
	})
	var drops []string
	for _, agent := range agents {
		drops = append(drops, fmt.Sprintf("%s=%d", agent, counts[agent]))
	}
	return strings.Join(parts, " ") + " | " + strings.Join(drops, " ")
}

func TestRandomRowSetsMatchPython(t *testing.T) {
	python := os.Getenv("PARITY_PYTHON")
	if python == "" {
		t.Skip("PARITY_PYTHON is not set; it names the Python 3 interpreter the oracle runs on")
	}
	analyzer, err := filepath.Abs(filepath.Join("..", "..", "analyzer"))
	if err != nil {
		t.Fatal(err)
	}
	source := &generator{random: rand.New(rand.NewPCG(19, 23))}
	var input bytes.Buffer
	var want []string
	for range 10000 {
		primary, secondary := source.randomCase()
		input.WriteString(encodeCase(primary, secondary) + "\n")
		want = append(want, summary(Deduplicate(primary, secondary, sampleHooks())))
	}
	lines := strings.Split(input.String(), "\n")
	command := exec.Command(python, "-I", "-c", oracleScript, analyzer)
	command.Stdin = &input
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		t.Fatalf("oracle: %v\n%s", err, stderr.String())
	}
	answers := strings.Split(strings.TrimSuffix(string(output), "\n"), "\n")
	if len(answers) != len(want) {
		t.Fatalf("oracle gave %d answers for %d cases", len(answers), len(want))
	}
	for i, answer := range answers {
		if strings.TrimRight(answer, " ") != strings.TrimRight(want[i], " ") {
			t.Errorf("case %d: Go keeps %q, Python %q\n%s", i, want[i], answer, lines[i])
		}
	}
}
