//go:build parity

package timeline

import (
	"bytes"
	"encoding/hex"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/pytext"
)

// oracleScript reads commands, one per line, each a word and hex fields
// (UTF-8 with surrogates passed through). "row" adds a Row of the fifteen
// fields to the batch and prints its JSONL line; "sessions" prints the
// sessions.jsonl lines of the batch for four comma-separated agent sets;
// "order" prints the batch's indices in timeline order and empties it;
// "path" prints the canonical form, key and Windows test of a path.
// The analyzer's directory is the first argument, since -I ignores
// PYTHONPATH.
const oracleScript = `import sys
sys.path.insert(0, sys.argv[1])
from doubleagent.model import Row, TIMELINE_COLUMNS, summarise
from doubleagent.model import canonical_project_path, is_windows_path, project_key
from doubleagent.cli import jsonl_line
def text(field):
    return bytes.fromhex(field).decode("utf-8", "surrogatepass")
def out(value):
    print(value.encode("utf-8", "surrogatepass").hex())
batch = []
for line in sys.stdin:
    word, *fields = line.rstrip("\n").split(" ")
    if word == "row":
        values = [text(field) for field in fields]
        values[13] = int(values[13])
        row = Row(**dict(zip(TIMELINE_COLUMNS, values)))
        batch.append(row)
        out(jsonl_line(row.as_dict()))
    elif word == "path":
        path = text(fields[0])
        print(" ".join(part.encode("utf-8", "surrogatepass").hex() for part in (
            canonical_project_path(path), project_key(path), str(is_windows_path(path)))))
    elif word == "sessions":
        sets = [frozenset(a for a in text(field).split(",") if a) for field in fields]
        out("".join(jsonl_line(s.as_dict()) + "\n" for s in summarise(batch, *sets)))
    else:
        order = sorted(range(len(batch)), key=lambda i: (batch[i].timestamp_utc == "",
            batch[i].timestamp_utc, batch[i].source_file, batch[i].source_line))
        print(",".join(map(str, order)) or "-")
        batch = []
`

// askPython runs oracleScript under the interpreter PARITY_PYTHON names
// over commands, skipping the test when the variable is unset, and
// returns one answer per command.
func askPython(t *testing.T, commands []string) []string {
	t.Helper()
	python := os.Getenv("PARITY_PYTHON")
	if python == "" {
		t.Skip("PARITY_PYTHON is not set; it names the Python 3 interpreter the oracle runs on")
	}
	analyzer, err := filepath.Abs(filepath.Join("..", "..", "analyzer"))
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(python, "-I", "-c", oracleScript, analyzer)
	command.Stdin = strings.NewReader(strings.Join(commands, "\n") + "\n")
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		t.Fatalf("oracle: %v\n%s", err, stderr.String())
	}
	answers := strings.Split(strings.TrimSuffix(string(output), "\n"), "\n")
	if len(answers) != len(commands) {
		t.Fatalf("oracle gave %d answers for %d commands", len(answers), len(commands))
	}
	return answers
}

// rowCommand returns the "row" command for row.
func rowCommand(row Row) string {
	fields := []string{row.TimestampUTC, row.Host, row.User, row.Agent, row.SessionID,
		row.ProjectPath, row.GitBranch, row.TurnType, row.Model, row.ToolName, row.ToolUseID,
		row.Text, row.SourceFile, strconv.Itoa(row.SourceLine), row.Recovered}
	for i, field := range fields {
		fields[i] = hex.EncodeToString([]byte(field))
	}
	return "row " + strings.Join(fields, " ")
}

func TestRowLinesMatchPython(t *testing.T) {
	random := rand.New(rand.NewPCG(19, 23))
	rows := make([]Row, 3000)
	commands := make([]string, len(rows))
	for i := range rows {
		rows[i] = Row{TimestampUTC: oddText(random), Host: oddText(random),
			Agent: oddText(random), Text: oddText(random), Model: oddText(random),
			SourceFile: oddText(random), SourceLine: random.IntN(1 << 40), Recovered: "yes"}
		commands[i] = rowCommand(rows[i])
	}
	commands = append(commands, "order")
	answers := askPython(t, commands)
	for i := range rows {
		want, err := hex.DecodeString(answers[i])
		if err != nil {
			t.Fatal(err)
		}
		if got := AppendRow(nil, &rows[i]); string(got) != string(want)+"\n" {
			t.Errorf("AppendRow =\n%q\nPython writes\n%q", got, want)
		}
	}
}

func TestSessionsAndOrderMatchPython(t *testing.T) {
	random := rand.New(rand.NewPCG(29, 31))
	agentSets := []string{"", "claude-code", "copilot-cli,aider", "claude-code,copilot-cli,aider"}
	var commands []string
	var batches [][]Row
	var marks []MarkAgents
	for range 300 {
		rows := make([]Row, 1+random.IntN(40))
		for i := range rows {
			rows[i] = poolRow(random)
			commands = append(commands, rowCommand(rows[i]))
		}
		sets := make([]string, 4)
		for i := range sets {
			sets[i] = agentSets[random.IntN(len(agentSets))]
		}
		mark := MarkAgents{agentMap(sets[0]), agentMap(sets[1]), agentMap(sets[2]),
			agentMap(sets[3])}
		for i, set := range sets {
			sets[i] = hex.EncodeToString([]byte(set))
		}
		commands = append(commands, "sessions "+strings.Join(sets, " "), "order")
		batches, marks = append(batches, rows), append(marks, mark)
	}
	answers := askPython(t, commands)
	position := 0
	for i, rows := range batches {
		position += len(rows)
		compareBatch(t, rows, marks[i], answers[position], answers[position+1])
		position += 2
	}
}

// compareBatch compares Summarise and SortRows over rows with the oracle's
// answers for them.
func compareBatch(t *testing.T, rows []Row, marks MarkAgents, sessions, order string) {
	t.Helper()
	var out bytes.Buffer
	if err := WriteSessions(&out, Summarise(rows, marks)); err != nil {
		t.Fatal(err)
	}
	if want, _ := hex.DecodeString(sessions); out.String() != string(want) {
		t.Errorf("sessions of %d rows =\n%s\nPython gives\n%s", len(rows), out.String(), want)
	}
	indexed := make([]Row, len(rows))
	for i, row := range rows {
		row.ToolUseID = strconv.Itoa(i)
		indexed[i] = row
	}
	SortRows(indexed)
	positions := make([]string, len(indexed))
	for i, row := range indexed {
		positions[i] = row.ToolUseID
	}
	if got := strings.Join(positions, ","); got != order {
		t.Errorf("order = %s, Python gives %s", got, order)
	}
}

func TestProjectPathsOfGeneratedSpellingsMatchPython(t *testing.T) {
	random := rand.New(rand.NewPCG(37, 41))
	const alphabet = `\\/?:CcUuNnx. `
	paths := make([]string, 5000)
	commands := make([]string, len(paths))
	for i := range paths {
		path := []byte{}
		if random.IntN(2) == 0 {
			path = append(path, pick(random, `\\?\`, `\\?\UNC\`, `\\?\unc/`, `C:\`, `\\`)...)
		}
		for range random.IntN(10) {
			path = append(path, alphabet[random.IntN(len(alphabet))])
		}
		paths[i] = string(path)
		commands[i] = "path " + hex.EncodeToString(path)
	}
	for i, answer := range askPython(t, commands) {
		windows := pytext.Bool(IsWindowsPath(paths[i]))
		got := hex.EncodeToString([]byte(CanonicalProjectPath(paths[i]))) + " " +
			hex.EncodeToString([]byte(ProjectKey(paths[i]))) + " " +
			hex.EncodeToString([]byte(windows))
		if got != answer {
			t.Errorf("path %q gives %s, Python gives %s", paths[i], got, answer)
		}
	}
}

// pick returns one of options at random.
func pick(random *rand.Rand, options ...string) string {
	return options[random.IntN(len(options))]
}

// agentMap returns the agent set a comma-separated list names.
func agentMap(list string) map[string]bool {
	set := map[string]bool{}
	for _, agent := range strings.Split(list, ",") {
		if agent != "" {
			set[agent] = true
		}
	}
	return set
}

// poolRow returns a row whose fields come from small pools, so sessions,
// sources, times and marks collide.
func poolRow(random *rand.Rand) Row {
	pick := func(options ...string) string { return options[random.IntN(len(options))] }
	return Row{
		TimestampUTC: pick("", "", "2026-10-01T08:00:00.000Z", "2026-10-01T08:00:00.001Z",
			"2026-09-30T23:59:59.999Z", "0999-01-01T00:00:00.000Z"),
		Host:        pick("h", "g"),
		User:        pick("u", "v"),
		Agent:       pick("claude-code", "copilot-cli", "aider"),
		SessionID:   pick("", "a", "b", "c"),
		ProjectPath: pick("", "", "/srv/p", "/srv/q", "C:\\x"),
		TurnType:    pick("user", "assistant", "tool_use", "tool_result", "system", "thinking"),
		Model:       pick("", "", "m1", "m2", "M1"),
		Text: pick("", "hello", "session synced: acct", "session relayed: chat",
			"session origin: remote cloud", "scratch directory: tmp", "parser: bad"),
		SourceFile: pick("", "/t.jsonl", "/h.jsonl", "/a"),
		SourceLine: random.IntN(4),
		Recovered:  pick("", "yes"),
	}
}

// oddText returns short text of characters a writer must handle: controls,
// quotes, separators, non-ASCII and emoji, and now and then a lone
// surrogate.
func oddText(random *rand.Rand) string {
	alphabet := []rune{'a', '"', '\\', '/', '\n', '\r', '\t', '\b', '\f', 0, 0x1f, 0x7f, 0x80,
		0x85, 0xa0, 0xe9, 0x2028, 0x2029, 0xfeff, 0xfffd, 0x4e2d, 0x1f600, '<', '&'}
	var out []byte
	for range random.IntN(12) {
		out = pytext.AppendCodePoint(out, alphabet[random.IntN(len(alphabet))])
	}
	if random.IntN(10) == 0 {
		out = pytext.AppendCodePoint(out, rune(0xd800+random.IntN(0x800)))
	}
	return string(out)
}
