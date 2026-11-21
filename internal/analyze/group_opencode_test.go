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

// The times of the OpenCode stores below, in milliseconds: an old stored
// row of a repeated message id, a new one five months later, and a part
// stored a day after the new row.
const (
	opencodeOld  = 1767225600000
	opencodeNew  = 1780272000000
	opencodePart = 1780358400000
)

// opencodeRepeatV1 returns a 1.x database whose message table holds a
// user message id and an assistant message id each in two rows, one in an
// old session and one in a new one, each row with its own model, and the
// parts of each id: a text part dated a day after the new row.
func opencodeRepeatV1(t *testing.T) string {
	t.Helper()
	return buildDatabase(t,
		sqlitetest.Table{Name: "session", SQL: "CREATE TABLE session (id text PRIMARY KEY, project_id text, " +
			"directory text, title text, time_created integer, time_updated integer)", Rows: [][]any{
			{"ses_a", nil, "/proj/a", "A", opencodeOld, opencodeOld},
			{"ses_b", nil, "/proj/b", "B", opencodeNew, opencodeNew}}},
		sqlitetest.Table{Name: "message", SQL: "CREATE TABLE message (id text, session_id text, " +
			"time_created integer, time_updated integer, data text)", Rows: [][]any{
			{"msg_u", "ses_a", opencodeOld, opencodeOld, `{"role":"user"}`},
			{"msg_u", "ses_b", opencodeNew, opencodeNew, `{"role":"user"}`},
			{"msg_a", "ses_a", opencodeOld + 1000, opencodeOld + 1000,
				`{"role":"assistant","modelID":"mA","providerID":"pA"}`},
			{"msg_a", "ses_b", opencodeNew + 1000, opencodeNew + 1000,
				`{"role":"assistant","modelID":"mB","providerID":"pB"}`}}},
		sqlitetest.Table{Name: "part", SQL: "CREATE TABLE part (id text, message_id text, session_id text, " +
			"time_created integer, time_updated integer, data text)", Rows: [][]any{
			{"prt_u", "msg_u", "ses_a", opencodePart, opencodePart, `{"type":"text","text":"USER-PART-TEXT"}`},
			{"prt_a", "msg_a", "ses_a", opencodePart + 1000, opencodePart + 1000,
				`{"type":"text","text":"ASSISTANT-PART-TEXT"}`}}})
}

// opencodeRepeatV2 returns a 2.x database whose session_message holds an
// assistant message id in two records, an old one and a new one in
// another session, each with its own model and its own content, and the
// id's one session_part item, a text the record's time dates.
func opencodeRepeatV2(t *testing.T) string {
	t.Helper()
	record := func(text, provider, model string) string {
		return `{"content":[{"type":"text","text":"` + text + `"}],"model":{"providerID":"` + provider +
			`","id":"` + model + `"}}`
	}
	return buildDatabase(t,
		sqlitetest.Table{Name: "session_v2", SQL: "CREATE TABLE session_v2 (id text PRIMARY KEY, project_id text, " +
			"directory text, title text, time_created integer, time_updated integer)", Rows: [][]any{
			{"ses_c", nil, "/proj/c", "C", opencodeOld, opencodeOld},
			{"ses_d", nil, "/proj/d", "D", opencodeNew, opencodeNew}}},
		sqlitetest.Table{Name: "session_message", SQL: "CREATE TABLE session_message (id text, session_id text, " +
			"type text, seq integer, time_created integer, time_updated integer, data text)", Rows: [][]any{
			{"msg_v", "ses_c", "assistant", 0, opencodeOld, opencodeOld, record("OWN-OLD", "pA", "mA")},
			{"msg_v", "ses_d", "assistant", 0, opencodeNew, opencodeNew, record("OWN-NEW", "pB", "mB")}}},
		sqlitetest.Table{Name: "session_part", SQL: "CREATE TABLE session_part (id text, message_id text, " +
			"session_id text, type text, ordinal integer, data text)", Rows: [][]any{
			{"prt_v", "msg_v", "ses_c", "text", 0, `{"text":"ITEM-TEXT"}`}}})
}

// buildDatabase returns the bytes of a database holding the tables.
func buildDatabase(t *testing.T, tables ...sqlitetest.Table) string {
	t.Helper()
	data, err := sqlitetest.Build(tables...)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// opencodeRepeatRun runs timeline over a collection holding the 1.x
// database at legacyRel and the 2.x one at currentRel, for the agent
// given, with the options given, and returns each row of timeline.jsonl
// that has a session as "time session project model text".
func opencodeRepeatRun(t *testing.T, agent, legacyRel, currentRel string, options ...string) []string {
	t.Helper()
	work := t.TempDir()
	collection := writeCollection(t, filepath.Join(work, "col"),
		[]jsonvalue.Member{member("hostname", jsonvalue.NewString("h1"))}, []collectedFile{
			{"alice", "/home/alice", agent, legacyRel, opencodeRepeatV1(t)},
			{"alice", "/home/alice", agent, currentRel, opencodeRepeatV2(t)}})
	env := Environment{Now: func() time.Time { return time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC) },
		Parsers: parsers.New, Version: "0.0.0-test"}
	output := filepath.Join(work, "out")
	args := append(append([]string{"timeline", "--agent", agent}, options...), collection, output)
	code, _, stderr := runCommand(env, args...)
	if code != ExitFound {
		t.Fatalf("%q exited %d: %s", args, code, stderr)
	}
	return recordsOf(t, filepath.Join(output, TimelineFile), "session_id", "timestamp_utc", "session_id",
		"project_path", "model", "text")
}

// The rows of the OpenCode stores above: each stored row's own rows, and
// the rows of its id's parts with the attributes of one stored row.
const (
	opencodeShown    = "parser: message holds 2 rows of this row's id; its parts are shown once, with "
	opencodeShownV2  = "parser: session_message holds 2 rows of this row's id; its parts are shown once, with "
	opencodeStartA   = "2026-01-01T00:00:00.000Z ses_a /proj/a  session start: A directory=/proj/a"
	opencodeStartB   = "2026-06-01T00:00:00.000Z ses_b /proj/b  session start: B directory=/proj/b"
	opencodeStartC   = "2026-01-01T00:00:00.000Z ses_c /proj/c  session start: C directory=/proj/c"
	opencodeStartD   = "2026-06-01T00:00:00.000Z ses_d /proj/d  session start: D directory=/proj/d"
	opencodeUserA    = "2026-01-01T00:00:00.000Z ses_a /proj/a  "
	opencodeUserB    = "2026-06-01T00:00:00.000Z ses_b /proj/b  "
	opencodeModelA   = "2026-01-01T00:00:01.000Z ses_a /proj/a pA/mA "
	opencodeModelB   = "2026-06-01T00:00:01.000Z ses_b /proj/b pB/mB "
	opencodeRecordC  = "2026-01-01T00:00:00.000Z ses_c /proj/c pA/mA "
	opencodeRecordD  = "2026-06-01T00:00:00.000Z ses_d /proj/d pB/mB "
	opencodePartWith = "2026-06-02T00:00:01.000Z "
)

// opencodeRepeatCase is one run over the OpenCode stores above: the
// filter's options and the rows it gives.
type opencodeRepeatCase struct {
	name    string
	options []string
	want    []string
}

// opencodeRepeatCases returns the runs over the OpenCode stores above:
// without a filter, with a window or a pattern that keeps only the later
// stored rows of each id, only the earlier, or (a pattern) neither.
func opencodeRepeatCases() []opencodeRepeatCase {
	assistantA, assistantB := opencodePartWith+"ses_a /proj/a pA/mA ASSISTANT-PART-TEXT",
		opencodePartWith+"ses_b /proj/b pB/mB ASSISTANT-PART-TEXT"
	return []opencodeRepeatCase{
		{"without a filter they go with the first row of the id", nil, []string{
			opencodeStartC, opencodeRecordC + opencodeShownV2 + "this row", opencodeRecordC + "model: mA provider=pA",
			opencodeRecordC + "ITEM-TEXT", opencodeStartA, opencodeUserA + opencodeShown + "this row",
			opencodeUserA + "USER-PART-TEXT", opencodeModelA + opencodeShown + "this row",
			opencodeModelA + "model: mA provider=pA", opencodeStartD,
			opencodeRecordD + opencodeShownV2 + "another of them", opencodeRecordD + "model: mB provider=pB",
			opencodeRecordD + "OWN-NEW", opencodeStartB, opencodeUserB + opencodeShown + "another of them",
			opencodeModelB + opencodeShown + "another of them", opencodeModelB + "model: mB provider=pB", assistantA}},
		{"a window past the first row moves them to the second", []string{"--since", "2026-03-01"}, []string{
			opencodeRecordD + "ITEM-TEXT", opencodeStartD, opencodeRecordD + opencodeShownV2 + "this row",
			opencodeRecordD + "model: mB provider=pB", opencodeUserB + "USER-PART-TEXT", opencodeStartB,
			opencodeUserB + opencodeShown + "this row", opencodeModelB + opencodeShown + "this row",
			opencodeModelB + "model: mB provider=pB", assistantB}},
		{"a window before the second row keeps them with the first", []string{"--until", "2026-03-01"}, []string{
			opencodeStartC, opencodeRecordC + opencodeShownV2 + "this row", opencodeRecordC + "model: mA provider=pA",
			opencodeRecordC + "ITEM-TEXT", opencodeStartA, opencodeUserA + opencodeShown + "this row",
			opencodeUserA + "USER-PART-TEXT", opencodeModelA + opencodeShown + "this row",
			opencodeModelA + "model: mA provider=pA"}},
		{"a pattern only the second row's rows match moves them to the second", []string{"--match", "mB|TEXT"},
			[]string{opencodeUserA + "USER-PART-TEXT", opencodeRecordD + "ITEM-TEXT",
				opencodeRecordD + "model: mB provider=pB", opencodeModelB + "model: mB provider=pB", assistantB}},
		{"a pattern only the first row's rows match keeps them with the first", []string{"--match", "mA|TEXT"},
			[]string{opencodeRecordC + "model: mA provider=pA", opencodeRecordC + "ITEM-TEXT",
				opencodeUserA + "USER-PART-TEXT", opencodeModelA + "model: mA provider=pA", assistantA}},
		{"a pattern only a later record's own content matches shows that content", []string{"--match", "OWN-NEW"},
			[]string{opencodeRecordD + "OWN-NEW"}},
		{"a pattern matching a later record's content and model keeps the content shown",
			[]string{"--match", "OWN-NEW|mB"}, []string{opencodeRecordD + "model: mB provider=pB",
				opencodeRecordD + "OWN-NEW", opencodeModelB + "model: mB provider=pB"}},
		{"a pattern neither row's rows match keeps them with the first", []string{"--match", "PART-TEXT|ITEM"},
			[]string{opencodeRecordC + "ITEM-TEXT", opencodeUserA + "USER-PART-TEXT", assistantA}},
	}
}

func TestTheRowsOfARepeatedOpenCodeMessageGoWithTheFirstRowTheFilterKeeps(t *testing.T) {
	for _, test := range opencodeRepeatCases() {
		t.Run(test.name, func(t *testing.T) {
			rows := opencodeRepeatRun(t, "opencode", ".local/share/opencode/opencode.db",
				".local/share/opencode/opencode-dev.db", test.options...)
			if !slices.Equal(rows, test.want) {
				t.Errorf("rows\n%s\nwant\n%s", strings.Join(rows, "\n"), strings.Join(test.want, "\n"))
			}
		})
	}
}

// Kilo Code's CLI keeps OpenCode's stores under its own directory, read
// by the same reader, so a repeated message id there is shown the same way.
func TestTheRowsOfARepeatedKiloCodeMessageGoWithTheFirstRowTheFilterKeeps(t *testing.T) {
	for _, test := range opencodeRepeatCases() {
		t.Run(test.name, func(t *testing.T) {
			rows := opencodeRepeatRun(t, "kilo-code", ".local/share/kilo/kilo.db",
				".local/share/kilo/kilo-dev.db", test.options...)
			if !slices.Equal(rows, test.want) {
				t.Errorf("rows\n%s\nwant\n%s", strings.Join(rows, "\n"), strings.Join(test.want, "\n"))
			}
		})
	}
}
