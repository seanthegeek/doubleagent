//go:build parity

package replay

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parser/openclaw"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
)

// socketTokenDifference returns difference 250.
func socketTokenDifference() Difference {
	return Difference{Number: 250, Summary: "the OpenClaw exec approvals socket token is one row of its own, " +
		"not part of each setting's row: a parse of state/openclaw.sqlite may add, for each " +
		"exec_approvals_config row whose raw_json holds socket.token, the one row " +
		socketTokenText + " at that row's rowid and updated_at_ms, which the recording, which never read " +
		"the token, lacks", Added: socketTokenAdded}
}

// socketTokenText is the row difference 250 lets a parse add, the token
// withheld as the replay withholds every secret.
const socketTokenText = "permission setting: socket.token=" + replayPlaceholder +
	" (from state/openclaw.sqlite exec_approvals_config, updated_at_ms)"

// socketTokenLabel is the entry difference 250's added row is counted
// under.
const socketTokenLabel = "openclaw exec approvals socket token row"

// socketTokenRows returns, for a parse of an OpenClaw state database, a
// match for the whole row the binary writes for each exec_approvals_config
// row whose raw_json decodes and holds a socket token as a non-empty
// string, and nothing for any other parse. The database is read with the
// project's own reader, as cellTime reads one.
func socketTokenRows(parse parsedFile) []func(Row) bool {
	if parse.agent != "openclaw" || !strings.HasSuffix(parse.rel, "/state/openclaw.sqlite") {
		return nil
	}
	database, err := sqlitefile.OpenPath(parse.disk)
	if err != nil {
		return nil
	}
	defer func() { _ = database.Close() }()
	table, err := database.Table("exec_approvals_config")
	if err != nil || !table.Has("raw_json") || !table.Has("updated_at_ms") {
		return nil
	}
	rows, err := table.Select("raw_json", "updated_at_ms").All()
	if err != nil {
		return nil
	}
	var matches []func(Row) bool
	for _, row := range rows {
		document, err := jsonvalue.DecodeString(row.Get("raw_json").String())
		if token := document.Get("socket").Get("token"); err != nil || !token.IsString() || token.StringValue() == "" {
			continue
		}
		rule := addedRow{time: parser.ColumnTime(row.Get("updated_at_ms")), line: int(row.Rowid)}
		matches = append(matches, parse.matches(socketTokenText, rule, nil))
	}
	return matches
}

// socketTokenAdded takes from got, for each row of socketTokenRows, the
// first row that is that whole row and that want does not hold, and
// returns the rest and socketTokenLabel when it took one, else "". A
// binary that leaves the token out takes nothing, so no case uses the
// difference and a run of every case fails.
func socketTokenAdded(parse parsedFile, want, got []Row) ([]Row, string) {
	took := false
	for _, matches := range socketTokenRows(parse) {
		var taken bool
		if got, taken = takeRow(matches, want, got); taken {
			took = true
		}
	}
	if !took {
		return got, ""
	}
	return got, socketTokenLabel
}

// approvalsParse copies testdata/openclaw/approvals.db into a new home as
// state/openclaw.sqlite and returns the parse and the rows the binary
// gives it with secrets withheld as the replay withholds them.
func approvalsParse(t *testing.T) (parsedFile, []Row) {
	t.Helper()
	const rel = ".openclaw/state/openclaw.sqlite"
	data, err := osfile.ReadFile(filepath.Join("testdata", "openclaw", "approvals.db"))
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	path := filepath.Join(home, filepath.FromSlash(rel))
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := &input.Artifact{DiskPath: path, Original: "/h/" + rel, Rel: rel, Agent: "openclaw",
		Manifest: jsonvalue.NewNull(), Home: &input.Home{DiskPath: home, Original: "/h", User: "bob", Host: "mini"}}
	got, err := parseWithheld(openclaw.New(), artifact, parser.Options{})
	if err != nil {
		t.Fatal(err)
	}
	return parsedFile{agent: "openclaw", file: artifact.Original, disk: path, rel: rel, home: artifact.Home}, got
}

// socketTokenCase is one binary output difference 250 is given beside the
// recording's rows: whether it takes a row, and the rows it leaves.
type socketTokenCase struct {
	name  string
	got   func(binary []Row) []Row
	taken bool
	// left is the number of the binary's rows left after the take that
	// the recording does not hold.
	left int
}

// socketTokenCases returns the outputs difference 250 is tried on: the
// binary's own, and each way a wrong binary could write the token.
func socketTokenCases() []socketTokenCase {
	edit := func(change func(rows []Row) []Row) func([]Row) []Row {
		return func(binary []Row) []Row { return change(slices.Clone(binary)) }
	}
	tokenRows := func(rows []Row) []int {
		var indexes []int
		for i, row := range rows {
			if string(row.Text) == socketTokenText {
				indexes = append(indexes, i)
			}
		}
		return indexes
	}
	rewrite := func(old, new string) func([]Row) []Row {
		return edit(func(rows []Row) []Row {
			for _, i := range tokenRows(rows) {
				rows[i].Text = Text(strings.Replace(string(rows[i].Text), old, new, 1))
			}
			return rows
		})
	}
	return []socketTokenCase{
		{"the binary's token rows are taken", func(binary []Row) []Row { return binary }, true, 0},
		{"a binary that drops the token takes nothing", edit(func(rows []Row) []Row {
			return slices.DeleteFunc(rows, func(row Row) bool { return string(row.Text) == socketTokenText })
		}), false, 0},
		{"a token shown in place of the placeholder is left", rewrite(replayPlaceholder, "FIXTURE-SOCKET-TOKEN"),
			false, 2},
		{"a token row of another wording is left", rewrite("socket.token=", "socket token="), false, 2},
		{"a token row at another line is left", edit(func(rows []Row) []Row {
			for _, i := range tokenRows(rows) {
				rows[i].SourceLine += 7
			}
			return rows
		}), false, 2},
		{"a token row of another time is left", edit(func(rows []Row) []Row {
			for _, i := range tokenRows(rows) {
				rows[i].TimestampUTC = "2026-01-01T00:00:00.000Z"
			}
			return rows
		}), false, 2},
		{"a second row of one token is left", edit(func(rows []Row) []Row {
			return append(rows, rows[tokenRows(rows)[0]])
		}), true, 1},
		{"a token on the row of the settings without one is left", edit(func(rows []Row) []Row {
			extra := rows[tokenRows(rows)[0]]
			extra.SourceLine = 3
			return append(rows, extra)
		}), true, 1},
		{"the token appended to a setting's row is left", edit(func(rows []Row) []Row {
			rows[0].Text = Text(strings.Replace(string(rows[0].Text), " (from", " socket.token="+replayPlaceholder+" (from", 1))
			return rows
		}), true, 1},
	}
}

func TestTheSocketTokenRowIsTheOnlyRowDifference250LetsAParseAdd(t *testing.T) {
	parse, binary := approvalsParse(t)
	var recorded []Row
	for _, row := range binary {
		if !strings.Contains(string(row.Text), "socket.token") {
			recorded = append(recorded, row)
		}
	}
	if len(binary) != len(recorded)+2 || len(recorded) != 3 {
		t.Fatalf("the binary gave %d rows, %d without the token; want 5 and 3: %v", len(binary), len(recorded), binary)
	}
	for _, test := range socketTokenCases() {
		t.Run(test.name, func(t *testing.T) {
			left, label := difference(t, 250).Added(parse, recorded, test.got(binary))
			if (label == socketTokenLabel) != test.taken || label != "" && label != socketTokenLabel {
				t.Errorf("label %q, want taken %v", label, test.taken)
			}
			extra := slices.DeleteFunc(slices.Clone(left), func(row Row) bool { return slices.Contains(recorded, row) })
			if len(extra) != test.left {
				t.Errorf("left %d rows the recording lacks, want %d: %v", len(extra), test.left, extra)
			}
		})
	}
}

func TestTheSocketTokenRowIsTakenOnlyFromAnOpenClawStateDatabase(t *testing.T) {
	parse, binary := approvalsParse(t)
	for _, other := range []parsedFile{
		{agent: "hermes", file: parse.file, disk: parse.disk, rel: parse.rel, home: parse.home},
		{agent: "openclaw", file: parse.file, disk: parse.disk, rel: ".openclaw/agents/a/agent/openclaw-agent.sqlite",
			home: parse.home},
	} {
		if _, label := socketTokenAdded(other, nil, binary); label != "" {
			t.Errorf("a parse of %s by %s took a row", other.rel, other.agent)
		}
	}
}
