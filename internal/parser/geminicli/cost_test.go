package geminicli

import (
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
)

// allocatedBytes returns how many bytes work allocates on the heap, a count
// that does not depend on the machine's speed.
func allocatedBytes(work func()) uint64 {
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	work()
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}

// growsLinearly reports whether the bytes work allocates for twice the size
// stay under three times those for the size: a reader that rebuilds its
// text per line allocates four times as much.
func growsLinearly(t *testing.T, size int, work func(size int)) {
	t.Helper()
	small := allocatedBytes(func() { work(size) })
	large := allocatedBytes(func() { work(2 * size) })
	if large > 3*small {
		t.Errorf("%d bytes for size %d against %d for size %d: not linear", large, 2*size, small, size)
	}
}

func TestShellHistoryJoinsContinuedLines(t *testing.T) {
	tests := []struct {
		name string
		data string
		want []shellCommand
	}{
		{"one command per line", "ls\npwd\n", []shellCommand{{1, "ls"}, {2, "pwd"}}},
		{"a continued line joins the next with a space", "ls \\\n-la\nx", []shellCommand{{1, "ls  -la"}, {3, "x"}}},
		{"two continuations in a row", "a\\\nb\\\nc", []shellCommand{{1, "a b c"}}},
		{"an even run of backslashes does not continue", "a\\\\\nb", []shellCommand{{1, "a\\\\"}, {2, "b"}}},
		{"a continuation across a blank line", "a\\\n\n  \nb", []shellCommand{{1, "a b"}}},
		{"a continued last line keeps its backslash", "a\\", []shellCommand{{1, "a\\"}}},
		{"a CR before the LF goes", "a\\\r\nb\r\n", []shellCommand{{1, "a b"}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var problems parser.Problems
			got := shellHistoryEntries([]byte(test.data), &problems)
			if !slices.Equal(got, test.want) || len(problems) != 0 {
				t.Errorf("got %+v and %v, want %+v", got, problems, test.want)
			}
		})
	}
}

func TestAShellHistoryOfContinuedLinesCostsLinearWork(t *testing.T) {
	growsLinearly(t, 4000, func(size int) {
		var problems parser.Problems
		shellHistoryEntries([]byte(strings.Repeat("abcdefgh\\\n", size)), &problems)
	})
}

func TestAMessageOfManyTextPartsCostsLinearWork(t *testing.T) {
	growsLinearly(t, 4000, func(size int) {
		items := make([]jsonvalue.Value, size)
		for i := range items {
			items[i] = jsonvalue.NewString("abcdefgh")
		}
		contentRuns(jsonvalue.NewArray(items...), true)
	})
}

func TestJoinRunsMergesNeighboursOfOneKind(t *testing.T) {
	tests := []struct {
		name   string
		pieces []run
		want   []run
	}{
		{"nothing", nil, nil},
		{"neighbours of one kind", []run{{"", "a"}, {"", "b"}}, []run{{"", "a\nb"}}},
		{"an empty piece between them", []run{{"", "a"}, {"", ""}, {"", "b"}}, []run{{"", "a\nb"}}},
		{"a kind between two of another", []run{{"", "a"}, {"hook_context", "h"}, {"", "b"}},
			[]run{{"", "a"}, {"hook_context", "h"}, {"", "b"}}},
		{"runs of two kinds", []run{{"k", "a"}, {"k", "b"}, {"", "c"}, {"", "d"}},
			[]run{{"k", "a\nb"}, {"", "c\nd"}}},
		{"only empty pieces", []run{{"", ""}, {"k", ""}}, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := joinRuns(test.pieces); !slices.Equal(got, test.want) {
				t.Errorf("got %+v, want %+v", got, test.want)
			}
		})
	}
}
