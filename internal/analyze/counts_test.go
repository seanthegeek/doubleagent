package analyze

import (
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

func TestAddBytesHoldsAtTheLargestInt64(t *testing.T) {
	tests := []struct {
		name               string
		total, size, wants int64
	}{
		{"small sizes add", 2, 3, 5},
		{"a sum reaching the largest value is exact", math.MaxInt64 - 1, 1, math.MaxInt64},
		{"a sum past the largest value holds there", math.MaxInt64, math.MaxInt64, math.MaxInt64},
		{"one past the largest value holds there", math.MaxInt64, 1, math.MaxInt64},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := addBytes(test.total, test.size); got != test.wants {
				t.Errorf("addBytes(%d, %d) = %d, want %d", test.total, test.size, got, test.wants)
			}
		})
	}
}

func TestFirstMetKeepsTheFirstOrderAndFindsEachKey(t *testing.T) {
	var counts agentCounts
	for _, agent := range []string{"b", "a", "b", "c", "a", "b"} {
		countAgent(&counts, agent, 1)
	}
	var order []string
	for _, entry := range counts.values {
		order = append(order, entry.agent+"="+strconv.FormatInt(entry.files, 10))
	}
	if got := strings.Join(order, " "); got != "b=3 a=2 c=1" {
		t.Errorf("counts %s, want b=3 a=2 c=1", got)
	}
	if entry, ok := counts.get("c"); !ok || entry.files != 1 {
		t.Errorf("get(c) = %v, %v", entry, ok)
	}
	if _, ok := counts.get("d"); ok {
		t.Error("get(d) found an agent never met")
	}
}

// sizedRow returns a collected file row of agent pi under /home/a with a
// size as JSON text.
func sizedRow(name, size string) string {
	path := "/home/a/.pi/" + name
	row := jsonvalue.NewObject(member("user", jsonvalue.NewString("a")),
		member("home", jsonvalue.NewString("/home/a")), member("agent", jsonvalue.NewString("pi")),
		member("path", jsonvalue.NewString(path)), member("archive_path", jsonvalue.NewString("fs"+path)),
		member("type", jsonvalue.NewString("file")), member("status", jsonvalue.NewString("collected")))
	text := row.JSON()
	return text[:len(text)-1] + `,"size":` + size + "}"
}

func TestDetectHoldsATotalSizeAtTheLargestValue(t *testing.T) {
	work := t.TempDir()
	collection := filepath.Join(work, "col")
	largest := strconv.FormatInt(math.MaxInt64, 10)
	writeFile(t, filepath.Join(collection, "manifest.jsonl"),
		sizedRow("s", largest)+"\n"+sizedRow("t", largest)+"\n")
	writeFile(t, filepath.Join(collection, "fs", "home", "a", ".pi", "s"), "x")
	writeFile(t, filepath.Join(collection, "fs", "home", "a", ".pi", "t"), "x")
	for _, args := range [][]string{{"detect", collection}, {"detect", "--json", collection}} {
		code, stdout, stderr := runCommand(testEnvironment(fakeParsers("pi")...), args...)
		if code != ExitFound || !strings.Contains(stdout, largest) || strings.Contains(stdout, " -") {
			t.Errorf("%v: exit %d, stdout %q, stderr %q; want the largest value %s", args, code, stdout,
				stderr, largest)
		}
	}
}

func TestDetectKeepsTheOrderOfManyAgents(t *testing.T) {
	t.Parallel()
	var files []collectedFile
	for i := range 2000 {
		files = append(files, collectedFile{"a", "/home/a", "agent" + strconv.Itoa(1999-i),
			".pi/f" + strconv.Itoa(i), "x"})
	}
	work := t.TempDir()
	collection := writeCollection(t, filepath.Join(work, "col"), nil, files)
	code, stdout, _ := runCommand(testEnvironment(), "detect", collection)
	lines := strings.Split(stdout, "\n")
	if code != ExitFound || !strings.Contains(lines[5], "agent1999 ") || !strings.Contains(lines[2004], "agent0 ") {
		t.Errorf("exit %d; the table does not list the agents in the order met:\n%s\n%s", code, lines[5],
			lines[2004])
	}
}
