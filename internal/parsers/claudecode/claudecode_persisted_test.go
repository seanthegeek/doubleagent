package claudecode

import (
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// persistedStub is the tool result a transcript keeps for an output saved
// to tool-results/<name>.
func persistedStub(name string) string {
	return "<persisted-output>\nOutput too large. Full output saved to: /x/s1/tool-results/" + name +
		"\n</persisted-output>"
}

// toolResultRows writes a session file s1.jsonl of the given tool result
// stubs, one line each, beside s1/tool-results/ holding files, and returns
// the texts of the tool_result rows the parser gives it, each shared text
// written out in the order of the rows.
func toolResultRows(t *testing.T, stubs []string, files map[string]string) []string {
	t.Helper()
	home := t.TempDir()
	const rel = ".claude/projects/p/s1.jsonl"
	results := filepath.Join(home, ".claude", "projects", "p", "s1", "tool-results")
	if err := osfile.MkdirAll(results, 0o700); err != nil {
		t.Fatal(err)
	}
	for name, text := range files {
		if err := osfile.WriteFile(filepath.Join(results, name), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	lines := make([]string, len(stubs))
	for i, stub := range stubs {
		block := jsonvalue.NewObject(
			jsonvalue.Member{Key: "type", Value: jsonvalue.NewString("tool_result")},
			jsonvalue.Member{Key: "tool_use_id", Value: jsonvalue.NewString("t" + strconv.Itoa(i))},
			jsonvalue.Member{Key: "content", Value: jsonvalue.NewString(stub)})
		lines[i] = `{"type": "user", "sessionId": "s1", "timestamp": "2026-01-02T03:04:05.000Z", ` +
			`"message": {"role": "user", "content": [` + block.JSON() + `]}}`
	}
	path := filepath.Join(home, filepath.FromSlash(rel))
	if err := osfile.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := &input.Artifact{DiskPath: path, Original: "/home/u/" + rel, Rel: rel, Agent: "claude-code",
		Home: &input.Home{DiskPath: home, Original: "/home/u", User: "u", Host: "h"}, Manifest: jsonvalue.NewNull()}
	var rows []timeline.Row
	emit := func(row timeline.Row) {
		if row.TurnType == timeline.TurnToolResult {
			rows = append(rows, row)
		}
	}
	if err := New().Parse(artifact, parserkit.Options{}, emit); err != nil {
		t.Fatal(err)
	}
	timeline.Resolve(rows)
	texts := make([]string, len(rows))
	for i, row := range rows {
		texts[i] = row.Text
	}
	return texts
}

// laterReference is the row of a later reference to tool-results/<name>
// whose text line of s1.jsonl shows.
func laterReference(name string, line int) string {
	return persistedStub(name) + "\n[persisted output shown in full above " +
		"(/home/u/.claude/projects/p/s1.jsonl line " + strconv.Itoa(line) + "): tool-results/" + name + "]"
}

func TestAPersistedOutputNamedByManyStubsIsWrittenOnce(t *testing.T) {
	for _, size := range []int{100, 400} {
		output := strings.Repeat("z", 10*size)
		stubs := make([]string, size)
		for i := range stubs {
			stubs[i] = persistedStub("a.txt")
		}
		texts := toolResultRows(t, stubs, map[string]string{"a.txt": output})
		got := 0
		for _, text := range texts {
			got += len(text)
		}
		want := len(output) + (size-1)*len(laterReference("a.txt", 1))
		if got != want || len(texts) != size || texts[0] != output || texts[size-1] != laterReference("a.txt", 1) {
			t.Errorf("%d stubs naming one file of %d bytes: %d rows of %d bytes, want %d rows of %d bytes",
				size, len(output), len(texts), got, size, want)
		}
	}
}

func TestEachPersistedOutputIsShownInFullAtItsOwnFirstReference(t *testing.T) {
	stubs := []string{persistedStub("a.txt"), persistedStub("b.txt"), persistedStub("a.txt"),
		persistedStub("gone.txt"), persistedStub("gone.txt"), persistedStub("b.txt")}
	texts := toolResultRows(t, stubs, map[string]string{"a.txt": "output A", "b.txt": "output B"})
	missing := persistedStub("gone.txt") + "\n[persisted output not found: tool-results/gone.txt]"
	want := []string{"output A", "output B", laterReference("a.txt", 1), missing, missing,
		laterReference("b.txt", 2)}
	if strings.Join(texts, "\n|\n") != strings.Join(want, "\n|\n") {
		t.Errorf("got %q, want %q", texts, want)
	}
}

// sessionFiles is one session's transcripts, by path below its project
// directory p/ in the order they are parsed, each holding the given tool
// result stubs, and its tool-results/ files.
type sessionFiles struct {
	transcripts [][2]string
	outputs     map[string]string
}

// shownRow is the source file, line and text of a tool_result row.
type shownRow struct {
	file string
	line int
	text string
}

// writeTranscriptFile writes text at rel under home and returns its path.
func writeTranscriptFile(t *testing.T, home, rel, text string) string {
	t.Helper()
	path := filepath.Join(home, filepath.FromSlash(rel))
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// stubLine returns a transcript line of the session whose one tool result
// is the stub of tool-results/<name> of that session.
func stubLine(sessionID, name string) string {
	stub := "<persisted-output>\nOutput too large. Full output saved to: /x/" + sessionID + "/tool-results/" + name +
		"\n</persisted-output>"
	block := jsonvalue.NewObject(
		jsonvalue.Member{Key: "type", Value: jsonvalue.NewString("tool_result")},
		jsonvalue.Member{Key: "tool_use_id", Value: jsonvalue.NewString("t")},
		jsonvalue.Member{Key: "content", Value: jsonvalue.NewString(stub)})
	return `{"type": "user", "sessionId": "` + sessionID + `", "timestamp": "2026-01-02T03:04:05.000Z", ` +
		`"message": {"role": "user", "content": [` + block.JSON() + `]}}`
}

// sessionRows writes the sessions into one home and parses every
// transcript with one parser, session by session in order, and returns the
// tool_result rows, each shared text written out in the order of the
// rows, and the persisted outputs the parser read.
func sessionRows(t *testing.T, sessions map[string]sessionFiles, order []string) ([]shownRow, int) {
	t.Helper()
	home := t.TempDir()
	subject := New()
	subject.Work = parserkit.CountingWork()
	// One home value, as one load of an input gives all its files.
	homeValue := &input.Home{DiskPath: home, Original: "/home/u"}
	var parsed []timeline.Row
	for _, sessionID := range order {
		files := sessions[sessionID]
		for name, text := range files.outputs {
			writeTranscriptFile(t, home, ".claude/projects/p/"+sessionID+"/tool-results/"+name, text)
		}
		for _, transcript := range files.transcripts {
			rel := ".claude/projects/p/" + transcript[0]
			path := writeTranscriptFile(t, home, rel, stubLine(sessionID, transcript[1])+"\n")
			artifact := &input.Artifact{DiskPath: path, Original: "/home/u/" + rel, Rel: rel, Agent: "claude-code",
				Home: homeValue, Manifest: jsonvalue.NewNull()}
			if err := subject.Parse(artifact, parserkit.Options{}, func(row timeline.Row) {
				if row.TurnType == timeline.TurnToolResult {
					parsed = append(parsed, row)
				}
			}); err != nil {
				t.Fatal(err)
			}
		}
	}
	timeline.Resolve(parsed)
	rows := make([]shownRow, len(parsed))
	for i, row := range parsed {
		rows[i] = shownRow{row.SourceFile, row.SourceLine, row.Text}
	}
	return rows, subject.Work.Count("persisted outputs read")
}

// pointerTo is the row of a reference to tool-results/<name> of the session
// whose text a row of file at line shows.
func pointerTo(sessionID, name, file string, line int) string {
	return "<persisted-output>\nOutput too large. Full output saved to: /x/" + sessionID + "/tool-results/" + name +
		"\n</persisted-output>\n[persisted output shown in full above (" + file + " line " +
		strconv.Itoa(line) + "): tool-results/" + name + "]"
}

func TestAPersistedOutputIsWrittenOnceForAllTheSessionsTranscripts(t *testing.T) {
	for _, n := range []int{100, 200} {
		t.Run(strconv.Itoa(n)+" subagent transcripts naming one output", func(t *testing.T) {
			output := strings.Repeat("z", 10*n)
			files := sessionFiles{transcripts: [][2]string{{"s1.jsonl", "a.txt"}},
				outputs: map[string]string{"a.txt": output}}
			for i := range n {
				files.transcripts = append(files.transcripts,
					[2]string{"s1/subagents/agent-a" + strconv.Itoa(i) + ".jsonl", "a.txt"})
			}
			rows, reads := sessionRows(t, map[string]sessionFiles{"s1": files}, []string{"s1"})
			pointer := pointerTo("s1", "a.txt", "/home/u/.claude/projects/p/s1.jsonl", 1)
			size := 0
			for _, row := range rows {
				size += len(row.text)
			}
			t.Logf("%d rows of %d bytes", len(rows), size)
			if want := len(output) + n*len(pointer); size != want || rows[0].text != output ||
				rows[n].text != pointer || reads != 1 {
				t.Errorf("%d rows of %d bytes after %d reads, want %d bytes after one read", len(rows), size,
					reads, want)
			}
		})
	}
}

func TestAPointerNamesTheTranscriptAndLineThatShowTheOutput(t *testing.T) {
	files := sessionFiles{transcripts: [][2]string{{"s1/subagents/agent-a.jsonl", "a.txt"},
		{"s1.jsonl", "a.txt"}, {"s1/subagents/agent-b.jsonl", "a.txt"}}, outputs: map[string]string{"a.txt": "output A"}}
	rows, _ := sessionRows(t, map[string]sessionFiles{"s1": files}, []string{"s1"})
	first := "/home/u/.claude/projects/p/s1/subagents/agent-a.jsonl"
	want := []shownRow{{first, 1, "output A"},
		{"/home/u/.claude/projects/p/s1.jsonl", 1, pointerTo("s1", "a.txt", first, 1)},
		{"/home/u/.claude/projects/p/s1/subagents/agent-b.jsonl", 1, pointerTo("s1", "a.txt", first, 1)}}
	if !slices.Equal(rows, want) {
		t.Errorf("got %+v, want %+v", rows, want)
	}
}

func TestEachSessionShowsItsOwnPersistedOutputOnce(t *testing.T) {
	sessions := map[string]sessionFiles{}
	for _, sessionID := range []string{"s1", "s2"} {
		sessions[sessionID] = sessionFiles{transcripts: [][2]string{{sessionID + ".jsonl", "a.txt"},
			{sessionID + "/subagents/agent-a.jsonl", "a.txt"}}, outputs: map[string]string{"a.txt": "output of " + sessionID}}
	}
	rows, reads := sessionRows(t, sessions, []string{"s1", "s2"})
	want := []shownRow{{"/home/u/.claude/projects/p/s1.jsonl", 1, "output of s1"},
		{"/home/u/.claude/projects/p/s1/subagents/agent-a.jsonl", 1,
			pointerTo("s1", "a.txt", "/home/u/.claude/projects/p/s1.jsonl", 1)},
		{"/home/u/.claude/projects/p/s2.jsonl", 1, "output of s2"},
		{"/home/u/.claude/projects/p/s2/subagents/agent-a.jsonl", 1,
			pointerTo("s2", "a.txt", "/home/u/.claude/projects/p/s2.jsonl", 1)}}
	if !slices.Equal(rows, want) || reads != 2 {
		t.Errorf("got %+v after %d reads, want %+v after 2", rows, reads, want)
	}
}

func TestASecondLoadOfTheSameFilesShowsTheOutputAgain(t *testing.T) {
	home := t.TempDir()
	writeTranscriptFile(t, home, ".claude/projects/p/s1/tool-results/a.txt", "output A")
	const rel = ".claude/projects/p/s1.jsonl"
	path := writeTranscriptFile(t, home, rel, stubLine("s1", "a.txt")+"\n")
	subject := New()
	for load := range 2 {
		artifact := &input.Artifact{DiskPath: path, Original: "/home/u/" + rel, Rel: rel, Agent: "claude-code",
			Home: &input.Home{DiskPath: home, Original: "/home/u"}, Manifest: jsonvalue.NewNull()}
		var rows []timeline.Row
		if err := subject.Parse(artifact, parserkit.Options{}, func(row timeline.Row) {
			if row.TurnType == timeline.TurnToolResult {
				rows = append(rows, row)
			}
		}); err != nil {
			t.Fatal(err)
		}
		timeline.Resolve(rows)
		var texts []string
		for _, row := range rows {
			texts = append(texts, row.Text)
		}
		if !slices.Equal(texts, []string{"output A"}) {
			t.Errorf("load %d: got %q, want the output in full", load, texts)
		}
	}
}
