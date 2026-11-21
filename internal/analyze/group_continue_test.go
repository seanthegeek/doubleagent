package analyze

import (
	"strings"
	"testing"
)

// continueRepeatIndex is a sessions.json holding the session s1 twice: an
// old entry titled TITLE-A in /proj/a and a new one titled TITLE-B in
// /proj/b.
const continueRepeatIndex = `[{"sessionId":"s1","title":"TITLE-A","dateCreated":"1767225600000",` +
	`"workspaceDirectory":"/proj/a"},{"sessionId":"s1","title":"TITLE-B","dateCreated":"1780272000000",` +
	`"workspaceDirectory":"/proj/b"}]`

// continueRepeatFiles returns the index above under the directory given,
// the session's file, which names no title or workspace of its own, and,
// under Continue's directory, a dev_data line of the session a day after
// the new entry.
func continueRepeatFiles(agent, directory string) func(*testing.T) []collectedFile {
	return func(*testing.T) []collectedFile {
		files := []collectedFile{
			{"alice", "/home/alice", agent, directory + "sessions/sessions.json", continueRepeatIndex},
			{"alice", "/home/alice", agent, directory + "sessions/s1.json", `{"sessionId":"s1","history":[` +
				`{"message":{"role":"user","content":"USER-TEXT"}},` +
				`{"message":{"role":"assistant","content":"REPLY-TEXT"}}]}`}}
		if agent == "continue" {
			files = append(files, collectedFile{"alice", "/home/alice", agent,
				directory + "dev_data/0.2.0/chatInteraction.jsonl", `{"eventName":"chatInteraction",` +
					`"timestamp":"2026-06-02T00:00:00.000Z","sessionId":"s1","prompt":"DEV-TEXT"}` + "\n"})
		}
		return files
	}
}

// continueNotice is the text of the `parser:` row of each entry above.
const continueNotice = "parser: sessions.json holds 2 entries of this entry's sessionId; its session's rows are " +
	"shown once, with "

// continueMembers returns the session's rows with the time, title and
// project of one entry: prefix is the row prefix of that entry's time and
// project, start the start row's text with %s for the title, dev whether
// the dev_data line, at its own time, is read.
func continueMembers(prefix, project, title, start string, dev bool) []string {
	rows := []string{prefix + strings.Replace(start, "%s", title, 1), prefix + "USER-TEXT", prefix + "REPLY-TEXT"}
	if dev {
		rows = append(rows, "2026-06-02T00:00:00.000Z s1 "+project+"  DEV-TEXT")
	}
	return rows
}

// continueCases returns the runs over the files above and the rows each
// gives; start is the session start row's text with %s for the title, dev
// whether the dev_data line is read.
func continueCases(start string, dev bool) []repeatCase {
	const (
		atA = "2026-01-01T00:00:00.000Z s1 /proj/a  "
		atB = "2026-06-01T00:00:00.000Z s1 /proj/b  "
	)
	first := continueMembers(atA, "/proj/a", "TITLE-A", start, dev)
	second := continueMembers(atB, "/proj/b", "TITLE-B", start, dev)
	return []repeatCase{
		{"without a filter they go with the first entry of the id", nil,
			append([]string{atA + continueNotice + "this entry", atB + continueNotice + "another of them"}, first...)},
		{"a window past the first entry moves them to the second", []string{"--since", "2026-03-01"},
			append([]string{atB + continueNotice + "this entry"}, second...)},
		{"a pattern only the second entry and the rows match moves them to the second",
			[]string{"--match", "TITLE-B|-TEXT"}, second},
		{"a pattern only the first entry and the rows match keeps them with the first",
			[]string{"--match", "TITLE-A|-TEXT"}, first},
		{"a pattern neither entry matches keeps them with the first", []string{"--match", "-TEXT"},
			first[1:]},
	}
}

func TestTheRowsOfARepeatedContinueSessionGoWithTheFirstIndexEntryTheFilterKeeps(t *testing.T) {
	checkRepeatCases(t, "continue", continueRepeatFiles("continue", ".continue/"),
		continueCases("session start: %s items=2", true))
}

func TestTheRowsOfARepeatedPearAIChatSessionGoWithTheFirstIndexEntryTheFilterKeeps(t *testing.T) {
	checkRepeatCases(t, "pearai", continueRepeatFiles("pearai", ".pearai/"),
		continueCases("session start: %s | history=2 perplexityHistory=0", false))
}
