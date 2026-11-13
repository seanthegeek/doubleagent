package inventory

import (
	"strings"
	"testing"
)

// agentCase feeds an aggregator and names the agent lines it should write.
type agentCase struct {
	name  string
	feed  func(aggregator *Aggregator)
	lines string
}

// agentCases is the table of TestAgentLines.
func agentCases() []agentCase {
	return []agentCase{
		{"agents in order of first match with evidence", func(aggregator *Aggregator) {
			aggregator.Match("b-agent", ".b")
			aggregator.Match("a-agent", ".a")
			aggregator.Match("b-agent", ".b2")
			aggregator.Match("b-agent", ".b")
			aggregator.File("a-agent", 10, 86400)
			aggregator.File("b-agent", 5, 1700000000)
			aggregator.File("b-agent", 7, 1600000000)
		}, `{"type":"agent","host":"h","user":"u","agent":"b-agent","files":2,"bytes":12,` +
			`"first":"2020-09-13T12:26:40Z","last":"2023-11-14T22:13:20Z","projects":3,"evidence":".b,.b2"}` + "\n" +
			`{"type":"agent","host":"h","user":"u","agent":"a-agent","files":1,"bytes":10,` +
			`"first":"1970-01-02T00:00:00Z","last":"1970-01-02T00:00:00Z","projects":3,"evidence":".a"}` + "\n"},
		{"a match without files gives no line", func(aggregator *Aggregator) {
			aggregator.Match("empty", ".e")
		}, ""},
		{"a file without a time is counted but has no dates", func(aggregator *Aggregator) {
			aggregator.Match("a", ".a")
			aggregator.File("a", 4, 0)
		}, `{"type":"agent","host":"h","user":"u","agent":"a","files":1,"bytes":4,` +
			`"first":"","last":"","projects":3,"evidence":".a"}` + "\n"},
		{"a file of an agent never matched is not printed", func(aggregator *Aggregator) {
			aggregator.File("ghost", 4, 5)
		}, ""},
		{"a name with a quote or control byte is escaped", func(aggregator *Aggregator) {
			aggregator.Match("a\"b", "x\ty")
			aggregator.File("a\"b", 1, 1)
		}, `{"type":"agent","host":"h","user":"u","agent":"a\"b","files":1,"bytes":1,` +
			`"first":"1970-01-01T00:00:01Z","last":"1970-01-01T00:00:01Z","projects":3,"evidence":"x\ty"}` + "\n"},
	}
}

func TestAgentLines(t *testing.T) {
	for _, test := range agentCases() {
		t.Run(test.name, func(t *testing.T) {
			aggregator := NewAggregator()
			test.feed(aggregator)
			var out strings.Builder
			if err := aggregator.WriteAgentLines(&out, "h", "u", 3); err != nil {
				t.Fatal(err)
			}
			if out.String() != test.lines {
				t.Errorf("got:\n%s\nwant:\n%s", out.String(), test.lines)
			}
		})
	}
}

func TestWriteHostLine(t *testing.T) {
	var out strings.Builder
	host := Host{
		Host: "host01", Collector: "1.2.3", Mode: "image", At: "2026-10-04T16:31:54Z",
		UsersScanned: 3, UsersUnreadable: 1, DockerVolumes: 2,
	}
	if err := WriteHostLine(&out, host); err != nil {
		t.Fatal(err)
	}
	want := `{"type":"host","host":"host01","collector":"1.2.3","mode":"image",` +
		`"at":"2026-10-04T16:31:54Z","users_scanned":3,"users_unreadable":1,"docker_volumes":2}` + "\n"
	if out.String() != want {
		t.Errorf("got %s want %s", out.String(), want)
	}
}

func TestMatchResetsEarlierFileCounts(t *testing.T) {
	aggregator := NewAggregator()
	aggregator.File("late", 100, 50)
	aggregator.Match("late", ".l")
	aggregator.File("late", 1, 60)
	var out strings.Builder
	if err := aggregator.WriteAgentLines(&out, "h", "u", 0); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"files":1,"bytes":1,"first":"1970-01-01T00:00:50Z"`) {
		t.Errorf("got %s", out.String())
	}
}
