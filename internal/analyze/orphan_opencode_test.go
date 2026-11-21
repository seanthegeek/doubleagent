package analyze

import (
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parsers"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile/sqlitetest"
)

// opencodeOrphansV1 returns a 1.x database holding a session s1, a message
// of it with a part, a part naming a message no row holds, a part whose
// message_id is NULL, a message naming a session no row holds and a
// message whose session_id is NULL, each of those two with a part.
func opencodeOrphansV1(t *testing.T) string {
	t.Helper()
	user := func(messageID, sessionID any) []any {
		return []any{messageID, sessionID, opencodePart, opencodePart, `{"role":"user"}`}
	}
	text := func(partID string, messageID, sessionID any, offset int, body string) []any {
		return []any{partID, messageID, sessionID, opencodePart + offset, opencodePart + offset,
			`{"type":"text","text":"` + body + `"}`}
	}
	return buildDatabase(t,
		sqlitetest.Table{Name: "session", SQL: opencodeSessionSQL("session"), Rows: [][]any{
			{"s1", nil, "/proj", "T", opencodeNew, opencodeNew, nil}}},
		sqlitetest.Table{Name: "message", SQL: "CREATE TABLE message (id text, session_id text, " +
			"time_created integer, time_updated integer, data text)", Rows: [][]any{
			user("m1", "s1"), user("m2", "sX"), user("m3", nil)}},
		sqlitetest.Table{Name: "part", SQL: "CREATE TABLE part (id text, message_id text, session_id text, " +
			"time_created integer, time_updated integer, data text)", Rows: [][]any{
			text("p1", "m1", "s1", 0, "KEPT"), text("p2", "mX", "s1", 1000, "NO-MESSAGE"),
			text("p3", nil, "s1", 2000, "NULL-MESSAGE"), text("p4", "m2", "sX", 0, "NO-SESSION"),
			text("p5", "m3", nil, 0, "NULL-SESSION")}})
}

// opencodeOrphansV2 returns a 2.x database holding a session s1, an
// assistant record of it with one item, an item naming a record no row
// holds, an item whose message_id is NULL, an item of a user record, a
// record naming a session no row holds and a record whose session_id is
// NULL.
func opencodeOrphansV2(t *testing.T) string {
	t.Helper()
	record := func(messageID, sessionID any, kind string, seq int, data string) []any {
		return []any{messageID, sessionID, kind, seq, opencodePart + seq, opencodePart + seq, data}
	}
	item := func(partID string, messageID any, body string) []any {
		return []any{partID, messageID, "s1", "text", 0, `{"text":"` + body + `"}`}
	}
	return buildDatabase(t,
		sqlitetest.Table{Name: "session_v2", SQL: opencodeSessionSQL("session_v2"), Rows: [][]any{
			{"s1", nil, "/proj", "T", opencodeNew, opencodeNew, nil}}},
		sqlitetest.Table{Name: "session_message", SQL: "CREATE TABLE session_message (id text, session_id text, " +
			"type text, seq integer, time_created integer, time_updated integer, data text)", Rows: [][]any{
			record("m1", "s1", "assistant", 0, `{"content":[]}`), record("m2", "s1", "user", 1, `{"text":"ASKED"}`),
			record("m3", "sX", "user", 2, `{"text":"NO-SESSION"}`), record("m4", nil, "user", 3, `{"text":"NULL-SESSION"}`)}},
		sqlitetest.Table{Name: "session_part", SQL: "CREATE TABLE session_part (id text, message_id text, " +
			"session_id text, type text, ordinal integer, data text)", Rows: [][]any{
			item("p1", "m1", "KEPT"), item("p2", "mX", "NO-RECORD"), item("p3", nil, "NULL-RECORD"),
			item("p4", "m2", "USER-RECORD")}})
}

// orphanRun runs timeline over a collection holding database at rel for
// agent and returns every row as "time session project text".
func orphanRun(t *testing.T, agent, rel, database string) []string {
	t.Helper()
	work := t.TempDir()
	collection := writeCollection(t, filepath.Join(work, "col"),
		[]jsonvalue.Member{member("hostname", jsonvalue.NewString("h1"))},
		[]collectedFile{{"alice", "/home/alice", agent, rel, database}})
	env := Environment{Now: func() time.Time { return time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC) },
		Parsers: parsers.New, Version: "0.0.0-test"}
	output := filepath.Join(work, "out")
	args := []string{"timeline", "--agent", agent, collection, output}
	if code, _, stderr := runCommand(env, args...); code != ExitFound {
		t.Fatalf("%q exited %d: %s", args, code, stderr)
	}
	return recordsOf(t, filepath.Join(output, TimelineFile), "text", "timestamp_utc", "session_id",
		"project_path", "text")
}

// A part or item that no message or record read takes is evidence of its
// own: it is shown with its own session_id, that session's project and its
// own time, and one parser: row per table counts such rows; a message or
// record naming a session no row holds, or none, is shown with the session
// id it names and no project.
func TestAPartOfNoMessageIsShownWithItsOwnAttributes(t *testing.T) {
	const (
		part        = "2026-06-02T00:00:00.000Z "
		tail        = " naming no message read here; their rows are shown with their own session_id and no message"
		currentTail = " naming no assistant record read here; their rows are shown with their own session_id and no record"
	)
	legacy := []string{"2026-06-01T00:00:00.000Z s1 /proj session start: T directory=/proj",
		part + "s1 /proj KEPT", part + "sX  NO-SESSION", part + "  NULL-SESSION",
		"2026-06-02T00:00:01.000Z s1 /proj NO-MESSAGE", "2026-06-02T00:00:02.000Z s1 /proj NULL-MESSAGE",
		"   parser: part holds 2 rows" + tail}
	current := []string{"2026-06-01T00:00:00.000Z s1 /proj session start: T directory=/proj",
		part + "s1 /proj KEPT", "2026-06-02T00:00:00.001Z s1 /proj ASKED", "2026-06-02T00:00:00.002Z sX  NO-SESSION",
		"2026-06-02T00:00:00.003Z   NULL-SESSION", "   parser: session_part holds 3 rows" + currentTail,
		" s1 /proj NO-RECORD", " s1 /proj NULL-RECORD", " s1 /proj USER-RECORD"}
	for _, test := range []struct {
		name, agent, rel string
		build            func(*testing.T) string
		want             []string
	}{
		{"opencode 1.x", "opencode", ".local/share/opencode/opencode.db", opencodeOrphansV1, legacy},
		{"opencode 2.x", "opencode", ".local/share/opencode/opencode.db", opencodeOrphansV2, current},
		{"kilo code 1.x", "kilo-code", ".local/share/kilo/kilo.db", opencodeOrphansV1, legacy},
		{"kilo code 2.x", "kilo-code", ".local/share/kilo/kilo.db", opencodeOrphansV2, current},
	} {
		t.Run(test.name, func(t *testing.T) {
			if rows := orphanRun(t, test.agent, test.rel, test.build(t)); !slices.Equal(rows, test.want) {
				t.Errorf("rows\n%q\nwant\n%q", rows, test.want)
			}
		})
	}
}
