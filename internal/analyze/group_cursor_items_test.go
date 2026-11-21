package analyze

import (
	"encoding/base64"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parsers"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile/sqlitetest"
)

// cursorItemsOf returns a state.vscdb whose ItemTable holds the rows given,
// without the table's key constraint.
func cursorItemsOf(t *testing.T, rows [][]any) string {
	t.Helper()
	return buildDatabase(t, sqlitetest.Table{Name: "ItemTable", SQL: "CREATE TABLE ItemTable (key TEXT, value BLOB)",
		Rows: rows})
}

// cursorTimedTexts runs timeline over a home holding the state given, with
// the options given, and returns its rows as "time text", sorted.
func cursorTimedTexts(t *testing.T, state string, options ...string) []string {
	t.Helper()
	work := t.TempDir()
	collection := writeCollection(t, filepath.Join(work, "col"),
		[]jsonvalue.Member{member("hostname", jsonvalue.NewString("h1"))},
		[]collectedFile{{"alice", "/home/alice", "cursor", cursorStateRel, state}})
	env := Environment{Now: func() time.Time { return time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC) },
		Parsers: parsers.New, Version: "0.0.0-test"}
	output := filepath.Join(work, "out")
	args := append(append([]string{"timeline", "--agent", "cursor"}, options...), collection, output)
	if code, _, stderr := runCommand(env, args...); code != ExitFound {
		t.Fatalf("%q exited %d: %s", args, code, stderr)
	}
	rows := recordsOf(t, filepath.Join(output, TimelineFile), "text", "timestamp_utc", "text")
	slices.Sort(rows)
	return rows
}

// withoutTimes returns the rows given with their times taken off: the
// rows at the file's modification time have none a test can know.
func withoutTimes(rows []string) []string {
	texts := make([]string, len(rows))
	for i, row := range rows {
		_, texts[i], _ = strings.Cut(row, " ")
	}
	slices.Sort(texts)
	return texts
}

// cursorRepeatedIntegrations returns a state.vscdb whose ItemTable holds,
// each twice, the GitHub account alice, its usages (repo in January, then
// gist in June), the enrolment of host u1 and its display name (OLD-NAME,
// then NEW-NAME).
func cursorRepeatedIntegrations(t *testing.T) string {
	t.Helper()
	return cursorItemsOf(t, [][]any{
		{"github-alice", `[{"id":"x"}]`}, {"github-alice", `[{"id":"x"}]`},
		{"github-alice-usages", `[{"scopes":["repo"],"lastUsed":1767225600000}]`},
		{"github-alice-usages", `[{"scopes":["gist"],"lastUsed":1780272000000}]`},
		{"remoteAgentHost.enrolled.u1", "true"}, {"remoteAgentHost.enrolled.u1", "true"},
		{"remoteAgentHost.displayName.u1", `"OLD-NAME"`}, {"remoteAgentHost.displayName.u1", `"NEW-NAME"`}})
}

// The rows of the integrations above.
const (
	cursorAccountRepo = "2026-01-01T00:00:00.000Z source host: github account alice scopes=repo " +
		"(from state.vscdb key github-alice)"
	cursorAccountGist = "2026-06-01T00:00:00.000Z source host: github account alice scopes=gist " +
		"(from state.vscdb key github-alice)"
	cursorEnrolled = "remote agent host enrolled: %s (from state.vscdb remoteAgentHost.enrolled.u1, file mtime; " +
		"on by default, a paired device leaves no local record)"
)

// cursorShownOnce returns the parser: row of a stored row of an ItemTable
// key held twice whose rows are shown once, with it or another of them.
func cursorShownOnce(rows string, chosen bool) string {
	which := "another of them"
	if chosen {
		which = "this row"
	}
	return "parser: ItemTable holds 2 rows of this row's key; its " + rows + " shown once, with " + which
}

func TestTheRowsARepeatedCursorIntegrationKeyFeedsGoWithTheStoredRowTheFilterKeeps(t *testing.T) {
	enrolled := func(name string) string { return strings.Replace(cursorEnrolled, "%s", name, 1) }
	tests := []struct {
		name    string
		options []string
		timed   []string
		texts   []string
	}{
		{"without a filter the account takes the first usages and the host the first name", nil,
			[]string{cursorAccountRepo}, []string{enrolled("OLD-NAME")}},
		{"a window past the first usages moves the account to the second", []string{"--since", "2026-03-01"},
			[]string{cursorAccountGist}, []string{enrolled("OLD-NAME")}},
		{"a pattern only the second name matches moves the host to it", []string{"--match", "NEW-NAME"},
			nil, []string{enrolled("NEW-NAME")}},
		{"a pattern only the second scopes match moves the account to them", []string{"--match", "scopes=gist"},
			[]string{cursorAccountGist}, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rows := cursorTimedTexts(t, cursorRepeatedIntegrations(t), test.options...)
			texts := withoutTimes(rows)
			for _, want := range test.timed {
				if !slices.Contains(rows, want) {
					t.Errorf("no row %q in\n%s", want, strings.Join(rows, "\n"))
				}
			}
			for _, want := range test.texts {
				if !slices.Contains(texts, want) {
					t.Errorf("no row %q in\n%s", want, strings.Join(texts, "\n"))
				}
			}
			if count := countContaining(texts, "source host: github account") +
				countContaining(texts, "remote agent host enrolled:"); count != len(test.timed)+len(test.texts) {
				t.Errorf("%d account and host rows, want %d:\n%s", count, len(test.timed)+len(test.texts),
					strings.Join(texts, "\n"))
			}
		})
	}
	texts := withoutTimes(cursorTimedTexts(t, cursorRepeatedIntegrations(t)))
	for _, rows := range []string{"account row is", "enrolment row is"} {
		for _, chosen := range []bool{true, false} {
			if count := countContaining(texts, cursorShownOnce(rows, chosen)); count != 2 {
				t.Errorf("%d rows %q, want 2 (one per stored row of the account or host key and one "+
					"per stored row of the usages or name key):\n%s", count, cursorShownOnce(rows, chosen),
					strings.Join(texts, "\n"))
			}
		}
	}
}

// cursorMCPKey returns the ItemTable key of an MCP OAuth field of a scope:
// mcpOAuth.global. and URL-safe base64 of "[scope] field".
func cursorMCPKey(scope, field string) string {
	return "mcpOAuth.global." + base64.RawURLEncoding.EncodeToString([]byte("["+scope+"] "+field))
}

// cursorRepeatedMCP returns a state.vscdb whose ItemTable holds the server
// URL of the MCP scope srv twice, each with a credential, and its update
// time as each of updates.
func cursorRepeatedMCP(t *testing.T, updates ...string) string {
	t.Helper()
	url := cursorMCPKey("srv", "mcp_server_url")
	rows := [][]any{{url, `"https://tok:s3cret@one.invalid/"`}, {url, `"https://tok:s3cret@two.invalid/"`}}
	for _, update := range updates {
		rows = append(rows, []any{cursorMCPKey("srv", "mcp_oauth_updated_at_ms"), update})
	}
	return cursorItemsOf(t, rows)
}

// cursorServer returns the "mcp server:" row of the host given.
func cursorServer(when, credentials, host string) string {
	return when + "mcp server: srv url=https://" + credentials + host +
		".invalid/ (from state.vscdb mcpOAuth.global, mcp_oauth_updated_at_ms)"
}

func TestARepeatedCursorMCPServerGivesEveryURLAtTheUpdateTimeTheFilterKeeps(t *testing.T) {
	january, june := "2026-01-01T00:00:00.000Z ", "2026-06-01T00:00:00.000Z "
	urlNotice := "parser: ItemTable holds 2 rows of this row's key; each gives its own rows"
	timeNotice := "parser: ItemTable holds 2 rows of this row's key; its server rows are shown once, with "
	state := cursorRepeatedMCPTimes
	for _, test := range []repeatCase{
		{"without a filter both URLs take the first time", nil, []string{
			cursorServer(january, "tok:s3cret@", "one"), cursorServer(january, "tok:s3cret@", "two"),
			january + timeNotice + "this row", june + timeNotice + "another of them"}},
		{"a window past the first time moves both URLs to the second", []string{"--since", "2026-03-01"}, []string{
			cursorServer(june, "tok:s3cret@", "one"), cursorServer(june, "tok:s3cret@", "two"),
			june + timeNotice + "this row"}},
		{"withheld secrets leave neither URL's credentials", []string{"--no-secrets"}, []string{
			cursorServer(january, "REDACTED@", "one"), cursorServer(january, "REDACTED@", "two"),
			january + timeNotice + "this row", june + timeNotice + "another of them"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			rows := cursorTimedTexts(t, state(t), test.options...)
			var timed []string
			for _, row := range rows {
				if !strings.Contains(row, urlNotice) {
					timed = append(timed, row)
				}
			}
			if !slices.Equal(timed, test.want) || countContaining(rows, urlNotice) != 2 {
				t.Errorf("rows\n%s\nwant two %q and\n%s", strings.Join(rows, "\n"), urlNotice,
					strings.Join(test.want, "\n"))
			}
		})
	}
}

// A stored update time that is not a number is no time, so the server
// rows take the one that is, and the other says so.
func TestARepeatedCursorMCPUpdateTimeThatIsNotANumberIsShownAndNotTaken(t *testing.T) {
	rows := cursorTimedTexts(t, cursorRepeatedMCP(t, `"soon"`, "1780272000000"))
	june := "2026-06-01T00:00:00.000Z "
	for _, want := range []string{cursorServer(june, "tok:s3cret@", "one"), cursorServer(june, "tok:s3cret@", "two")} {
		if !slices.Contains(rows, want) {
			t.Errorf("no row %q in\n%s", want, strings.Join(rows, "\n"))
		}
	}
	if count := countContaining(rows, "rows of this row's key; it holds no time"); count != 1 {
		t.Errorf("%d rows say a stored update time holds no time, want 1:\n%s", count, strings.Join(rows, "\n"))
	}
}

// cursorRepeatedMCPTimes returns the MCP state above with the update time
// stored twice, in January and in June.
func cursorRepeatedMCPTimes(t *testing.T) string {
	return cursorRepeatedMCP(t, "1767225600000", "1780272000000")
}
