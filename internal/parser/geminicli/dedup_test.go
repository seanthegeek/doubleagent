package geminicli

import (
	"bytes"
	"slices"
	"strconv"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/dedup"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/pyjson"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// The deduplications the analyzer's Gemini CLI tests run over the parser's
// rows, recorded with Python's results by testdata/dedup_cases.py, run
// again through the Go deduplicator with the hooks the parser declares
// (none). The secondary rows kept, in order, and the counts dropped must
// match.
func TestDeduplicationOfTheSecondarySourcesMatchesPython(t *testing.T) {
	data, err := osfile.ReadFile("testdata/dedup_cases.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	hooks := dedup.Hooks{}
	if declarer, ok := any(New()).(parser.HooksDeclarer); ok {
		hooks["gemini-cli"] = declarer.DedupHooks()
	}
	for i, line := range bytes.Split(bytes.TrimSpace(data), []byte("\n")) {
		recorded, err := pyjson.Decode(line)
		if err != nil {
			t.Fatal(err)
		}
		name := strconv.Itoa(i+1) + " " + recorded.Get("test").Text()
		t.Run(name, func(t *testing.T) { replayDeduplication(t, recorded, hooks) })
	}
}

// replayDeduplication runs one recorded call through the Go deduplicator
// and compares the secondary rows kept and the counts dropped.
func replayDeduplication(t *testing.T, recorded pyjson.Value, hooks dedup.Hooks) {
	var primary []timeline.Row
	for _, row := range recorded.Get("primary").Items() {
		primary = append(primary, recordedRow(row))
	}
	var secondary []dedup.Secondary
	for _, row := range recorded.Get("secondary").Items() {
		tolerance, _ := row.Get("tolerance").Float()
		secondary = append(secondary, dedup.Secondary{Row: recordedRow(row), Tolerance: tolerance})
	}
	kept, dropped := dedup.Deduplicate(primary, secondary, hooks)
	var want []timeline.Row
	for _, index := range recorded.Get("kept").Items() {
		position, _ := index.Int()
		row := secondary[position].Row
		if row.TurnType != timeline.TurnSystem {
			row.Recovered = "yes"
		}
		want = append(want, row)
	}
	if got := kept[len(primary):]; !slices.Equal(got, want) {
		t.Errorf("kept %d secondary rows, want %d:\n got %+v\nwant %+v", len(got), len(want), got, want)
	}
	for agent, count := range dropped {
		if want, _ := recorded.Get("dropped").Get(agent).Int(); int64(count) != want {
			t.Errorf("dropped %d rows of %s, want %d", count, agent, want)
		}
	}
	if len(dropped) != recorded.Get("dropped").Len() {
		t.Errorf("dropped rows of %v, want %s", dropped, pyjson.Compact(recorded.Get("dropped")))
	}
}

// recordedRow returns the row a recorded object holds, its fields named as
// timeline.jsonl names them.
func recordedRow(object pyjson.Value) timeline.Row {
	line, _ := object.Get("source_line").Int()
	text := func(key string) string { return object.Get(key).Text() }
	return timeline.Row{TimestampUTC: text("timestamp_utc"), Host: text("host"), User: text("user"),
		Agent: text("agent"), SessionID: text("session_id"), ProjectPath: text("project_path"),
		GitBranch: text("git_branch"), TurnType: text("turn_type"), Model: text("model"),
		ToolName: text("tool_name"), ToolUseID: text("tool_use_id"), Text: text("text"),
		SourceFile: text("source_file"), SourceLine: int(line), Recovered: text("recovered")}
}
