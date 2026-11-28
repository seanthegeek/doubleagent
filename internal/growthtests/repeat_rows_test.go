package growthtests

import (
	"maps"
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/parsers"
)

// parsedAsOwn reports whether a parser that a run offers the artifact to,
// its agent's or one reading that agent's files, wants it.
func parsedAsOwn(artifact *input.Artifact) bool {
	for _, each := range parsers.All() {
		offered := each.Agent() == artifact.Agent || slices.Contains(parserkit.ReadsAgentsOf(each), artifact.Agent)
		if offered && each.Wants(artifact) {
			return true
		}
	}
	return false
}

// repeatRowsAndOwners returns the repeat rows by source file of a parse of
// the files of agent, or of the parser's own agent when agent is empty,
// and the source files a parser takes as its own artifact.
func repeatRowsAndOwners(t *testing.T, name, agent string, files []growthFile) (map[string]int, map[string]bool) {
	t.Helper()
	each := newParser(t, name)
	artifacts, _ := growthHome(t, cmpAgent(agent, each.Agent()), files)
	pinTimes(t, artifacts)
	own := map[string]bool{}
	for _, artifact := range artifacts {
		own[artifact.Original] = parsedAsOwn(artifact)
	}
	counts := map[string]int{}
	for _, row := range rowsOf(t, each, artifacts) {
		if isRepeatRow(row) {
			counts[row.SourceFile]++
		}
	}
	return counts, own
}

// repeatRowsByFileOf returns the repeat rows by source file of a parse of
// the files of agent, or of the parser's own agent when agent is empty.
func repeatRowsByFileOf(t *testing.T, name, agent string, files []growthFile) map[string]int {
	t.Helper()
	counts, _ := repeatRowsAndOwners(t, name, agent, files)
	return counts
}

func TestATaskMetadataRepeatIsReportedOnceOnItsOwnFile(t *testing.T) {
	const dir = ".config/Code/User/globalStorage/saoudrizwan.claude-dev/tasks/1767225600000/"
	files := []growthFile{
		{dir + "ui_messages.json", []byte(`[{"ts":1767225600100,"type":"say","say":"text","text":"hi"}]`)},
		{dir + "api_conversation_history.json",
			[]byte(`[{"role":"user","ts":1767225600150,"content":[{"type":"text","text":"hi"}]}]`)},
		{dir + "task_metadata.json", []byte(`{"files_in_context":[],"files_in_context":[]}`)},
	}
	got := repeatRowsByFileOf(t, "cline", "", files)
	want := map[string]int{"/home/u/" + dir + "task_metadata.json": 1}
	if !maps.Equal(got, want) {
		t.Errorf("repeat rows by file %v, want %v", got, want)
	}
}
