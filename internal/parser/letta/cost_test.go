package letta

import (
	"cmp"
	"fmt"
	"math/bits"
	"math/rand/v2"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// linearSessionProject is sessionProject as a scan of every line: the
// first at or after the time, else the last before it, kept to check the
// binary search against.
func linearSessionProject(sessions []projectTime, timestamp string) string {
	if timestamp == "" {
		return ""
	}
	before := ""
	for _, session := range sessions {
		if session.timestamp >= timestamp {
			return session.project
		}
		before = session.project
	}
	return before
}

func TestSessionProjectAgreesWithAScanOfEveryLine(t *testing.T) {
	for seed := range uint64(500) {
		t.Run("sessions from seed "+strconv.FormatUint(seed, 10), func(t *testing.T) {
			source := rand.New(rand.NewPCG(seed, 527))
			sessions := make([]projectTime, source.IntN(8))
			for i := range sessions {
				sessions[i] = projectTime{fmt.Sprintf("2026-01-0%dT00:00:00.000Z", 1+source.IntN(4)),
					"/p" + strconv.Itoa(source.IntN(3))}
			}
			slices.SortFunc(sessions, func(left, right projectTime) int {
				return cmp.Or(strings.Compare(left.timestamp, right.timestamp), strings.Compare(left.project, right.project))
			})
			for day := range 6 {
				timestamp := fmt.Sprintf("2026-01-0%dT00:00:00.000Z", day)
				if day == 0 {
					timestamp = ""
				}
				if got, want := sessionProject(sessions, timestamp, parser.Work{}),
					linearSessionProject(sessions, timestamp); got != want {
					t.Fatalf("%v at %q: %q, want %q", sessions, timestamp, got, want)
				}
			}
		})
	}
}

// writeHome writes files, by home-relative path, under a new home and
// returns the home's directory.
func writeHome(t *testing.T, files map[string]string) string {
	t.Helper()
	home := t.TempDir()
	for name, text := range files {
		path := filepath.Join(home, filepath.FromSlash(name))
		if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := osfile.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return home
}

func TestTranscriptsReadTheSideFilesOnceAndSearchTheirSessions(t *testing.T) {
	const size = 512
	var sessions, agents []string
	for i := range size {
		sessions = append(sessions, fmt.Sprintf(`{"agent_id":"agent-1","timestamp":%d,"project":"/p%d"}`,
			1791000000000+i*1000, i))
		agents = append(agents, `{"agentId":"agent-`+strconv.Itoa(i)+`","baseUrl":"https://s`+strconv.Itoa(i)+`.example/"}`)
	}
	files := map[string]string{sessionsRel: strings.Join(sessions, "\n"),
		settingsRel: `{"agents":[` + strings.Join(agents, ",") + `]}`}
	var lines []string
	for i := range size {
		lines = append(lines, fmt.Sprintf(`{"captured_at":%d,"kind":"user","text":"t%d"}`, 1791000000000+i*1000-500, i))
	}
	const transcripts = 4
	var rels []string
	for i := range transcripts {
		rel := ".letta/transcripts/agent-1/conv-" + strconv.Itoa(i) + "/transcript.jsonl"
		files[rel] = strings.Join(lines, "\n")
		rels = append(rels, rel)
	}
	home := writeHome(t, files)
	subject := New()
	subject.Work = parser.CountingWork()
	var projects []string
	for _, rel := range rels {
		artifact := &input.Artifact{DiskPath: filepath.Join(home, filepath.FromSlash(rel)), Original: "/h/" + rel,
			Rel: rel, Agent: "letta", Home: &input.Home{DiskPath: home, Original: "/h"}, Manifest: jsonvalue.NewNull()}
		if err := subject.Parse(artifact, parser.Options{}, func(row timeline.Row) {
			if row.TurnType == timeline.TurnUser {
				projects = append(projects, row.ProjectPath)
			}
		}); err != nil {
			t.Fatal(err)
		}
	}
	if len(projects) != transcripts*size || projects[size-1] != "/p"+strconv.Itoa(size-1) {
		t.Fatalf("%d user rows, the last of the first transcript in %q", len(projects), projects[size-1])
	}
	limit := transcripts * size * (bits.Len(size) + 1)
	if compared := subject.Work.Count("session comparisons"); compared == 0 || compared > limit ||
		subject.Work.Count("sessions.jsonl reads") != 1 || subject.Work.Count("settings.json reads") != 1 {
		t.Errorf("%d comparisons (at most %d), %d reads of sessions.jsonl and %d of settings.json, want one each",
			compared, limit, subject.Work.Count("sessions.jsonl reads"), subject.Work.Count("settings.json reads"))
	}
}
