package analyze

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parsers"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile/sqlitetest"
)

// opencodeRecordOf returns a session_message data value with the text and
// the model pA/mA.
func opencodeRecordOf(text string) string {
	return `{"content":[{"type":"text","text":"` + text + `"}],"model":{"providerID":"pA","id":"mA"}}`
}

// opencodeMessages returns a 2.x OpenCode database of the session ses_c
// whose session_message holds the rows given, of the message msg_v, and
// whose session_part holds the message's item when parts is set.
func opencodeMessages(t *testing.T, records [][]any, parts bool) string {
	t.Helper()
	tables := []sqlitetest.Table{
		{Name: "session_v2", SQL: "CREATE TABLE session_v2 (id text PRIMARY KEY, project_id text, " +
			"directory text, title text, time_created integer, time_updated integer)", Rows: [][]any{
			{"ses_c", nil, "/proj/c", "C", opencodeOld, opencodeOld},
			{"ses_d", nil, "/proj/d", "D", opencodeNew, opencodeNew}}},
		{Name: "session_message", SQL: "CREATE TABLE session_message (id text, session_id text, " +
			"type text, seq integer, time_created integer, time_updated integer, data text)", Rows: records}}
	if parts {
		tables = append(tables, sqlitetest.Table{Name: "session_part", SQL: "CREATE TABLE session_part (id text, " +
			"message_id text, session_id text, type text, ordinal integer, data text)", Rows: [][]any{
			{"prt_v", "msg_v", "ses_c", "text", 0, `{"text":"ITEM-TEXT"}`}}})
	}
	return buildDatabase(t, tables...)
}

// opencodeOwnOld is msg_v's record in ses_c holding OWN-OLD.
func opencodeOwnOld() []any {
	return []any{"msg_v", "ses_c", "assistant", 0, opencodeOld, opencodeOld, opencodeRecordOf("OWN-OLD")}
}

// dedupRun runs timeline over OpenCode's primary database and its next
// release's, a secondary source, and returns the dropped line of the
// summary and each row holding OWN or ITEM as "recovered text".
func dedupRun(t *testing.T, primary, next string) (string, []string) {
	t.Helper()
	work := t.TempDir()
	collection := writeCollection(t, filepath.Join(work, "col"),
		[]jsonvalue.Member{member("hostname", jsonvalue.NewString("h1"))}, []collectedFile{
			{"alice", "/home/alice", "opencode", ".local/share/opencode/opencode-dev.db", primary},
			{"alice", "/home/alice", "opencode", ".local/share/opencode/opencode-next.db", next}})
	env := Environment{Now: func() time.Time { return time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC) },
		Parsers: parsers.New, Version: "0.0.0-test"}
	output := filepath.Join(work, "out")
	code, stdout, stderr := runCommand(env, "timeline", "--agent", "opencode", collection, output)
	if code != ExitFound {
		t.Fatal(stderr)
	}
	dropped := ""
	for _, line := range strings.Split(stdout, "\n") {
		if strings.HasPrefix(line, "dropped:") {
			dropped = strings.Join(strings.Fields(line), " ")
		}
	}
	var rows []string
	for _, row := range recordsOf(t, filepath.Join(output, TimelineFile), "text", "recovered", "text") {
		if strings.Contains(row, "OWN") || strings.Contains(row, "ITEM") {
			rows = append(rows, strings.TrimSpace(row))
		}
	}
	return dropped, rows
}

// Deduplication compares the rows the output shows and no other: a row a
// repeated id's group leaves out neither takes a secondary copy of its
// text, which is then shown as recovered and not counted as dropped, nor,
// as a secondary row, takes the primary row a shown secondary copy would,
// which would then be shown beside it. The second case is the first's
// message stored once, for comparison.
func TestDeduplicationSeesOnlyTheRowsTheOutputShows(t *testing.T) {
	ownNew := []any{"msg_v", "ses_d", "assistant", 0, opencodeNew, opencodeNew, opencodeRecordOf("OWN-NEW")}
	tests := []struct {
		name, primary, next string
		dropped             string
		want                []string
	}{
		{"a primary row its group hides leaves the secondary copy of its text recovered",
			opencodeRepeatV2(t), opencodeMessages(t, [][]any{opencodeOwnOld()}, false),
			"dropped: 3 duplicate rows from secondary sources (opencode 3)",
			[]string{"ITEM-TEXT", "OWN-NEW", "yes OWN-OLD"}},
		{"the same message stored once in the primary database is the secondary copy's",
			opencodeMessages(t, [][]any{opencodeOwnOld()}, true), opencodeMessages(t, [][]any{opencodeOwnOld()}, false),
			"dropped: 3 duplicate rows from secondary sources (opencode 3)", []string{"ITEM-TEXT", "yes OWN-OLD"}},
		{"a secondary row its group hides takes no primary row from a shown copy",
			opencodeMessages(t, [][]any{opencodeOwnOld()}, false),
			opencodeMessages(t, [][]any{opencodeOwnOld(), opencodeOwnOld(), ownNew}, true),
			"dropped: 4 duplicate rows from secondary sources (opencode 4)",
			[]string{"OWN-OLD", "yes ITEM-TEXT", "yes OWN-NEW"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dropped, rows := dedupRun(t, test.primary, test.next)
			slices.Sort(rows)
			if dropped != test.dropped || !slices.Equal(rows, test.want) {
				t.Errorf("%s, rows %q\nwant %s, rows %q", dropped, rows, test.dropped, test.want)
			}
		})
	}
}
