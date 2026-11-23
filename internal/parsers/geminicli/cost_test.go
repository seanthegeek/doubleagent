package geminicli

import (
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/parserkit"
)

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
			var problems parserkit.Problems
			got := shellHistoryEntries([]byte(test.data), &problems, parserkit.Work{})
			if !slices.Equal(got, test.want) || len(problems) != 0 {
				t.Errorf("got %+v and %v, want %+v", got, problems, test.want)
			}
		})
	}
}

func TestAShellHistoryOfContinuedLinesIsJoinedOnce(t *testing.T) {
	const lines = 4000
	work := parserkit.CountingWork()
	var problems parserkit.Problems
	got := shellHistoryEntries([]byte(strings.Repeat("abcdefgh\\\n", lines)+"end"), &problems, work)
	calls, written := work.Count("joins"), work.Count("bytes joined")
	if len(got) != 1 || calls != 1 || written != len(got[0].command) || written != lines*9+3 {
		t.Errorf("%d commands, %d joins writing %d bytes; want one join of %d bytes", len(got), calls,
			written, lines*9+3)
	}
}

func TestAMessageOfManyTextPartsIsJoinedOnce(t *testing.T) {
	const parts = 4000
	pieces := make([]run, parts)
	for i := range pieces {
		pieces[i] = run{"", "abcdefgh"}
	}
	work := parserkit.CountingWork()
	got := joinRuns(pieces, work)
	calls, written := work.Count("joins"), work.Count("bytes joined")
	if len(got) != 1 || calls != 1 || written != parts*9-1 {
		t.Errorf("%d runs, %d joins writing %d bytes; want one join of %d bytes", len(got), calls, written,
			parts*9-1)
	}
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
			if got := joinRuns(test.pieces, parserkit.Work{}); !slices.Equal(got, test.want) {
				t.Errorf("got %+v, want %+v", got, test.want)
			}
		})
	}
}
