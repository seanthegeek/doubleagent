package opencode

import (
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/sqlitefile/sqlitetest"
)

// The first stored row of a repeated project id is the oldest by
// time_created, not the first the table holds: a session naming no
// directory of its own takes its worktree without a filter, though the
// table holds the newer row first.
func TestTheFirstStoredProjectRowIsTheOldest(t *testing.T) {
	rows := keysRows(t,
		sqlitetest.Table{Name: "project", SQL: "CREATE TABLE project (id TEXT, worktree TEXT, time_created INTEGER)",
			Rows: [][]any{{"prj_1", "/proj/newer", 1700000002000}, {"prj_1", "/proj/older", 1700000001000}}},
		sqlitetest.Table{Name: "session", SQL: "CREATE TABLE session (id TEXT, project_id TEXT, directory TEXT, " +
			"title TEXT, time_created INTEGER)", Rows: [][]any{{"s1", "prj_1", nil, "T", 1700000003000}}})
	const notice = " parser: project holds 2 rows of this row's id; its sessions and saved grants are shown once, with "
	want := []string{" /proj/older" + notice + "this row", " /proj/newer" + notice + "another of them",
		"s1 /proj/older session start: T"}
	if !slices.Equal(rows, want) {
		t.Errorf("rows %q, want %q", rows, want)
	}
}

// The first stored row of a repeated session id is the oldest by
// time_created, not the first the table holds.
func TestTheFirstStoredSessionRowIsTheOldest(t *testing.T) {
	rows := keysRows(t, keysSessionTable("", []any{"s1", "/proj/newer", "N", 1700000002000},
		[]any{"s1", "/proj/older", "O", 1700000001000}))
	const notice = " parser: session holds 2 rows of this row's id; its messages and shares are shown once, with "
	want := []string{"s1 /proj/older session start: O directory=/proj/older", "s1 /proj/older" + notice + "this row",
		"s1 /proj/newer session start: N directory=/proj/newer", "s1 /proj/newer" + notice + "another of them"}
	if !slices.Equal(rows, want) {
		t.Errorf("rows %q, want %q", rows, want)
	}
}
