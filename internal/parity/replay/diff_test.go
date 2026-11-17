//go:build parity

package replay

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// shownDifferences is how many differing rows or lines a report lists before
// it only counts the rest.
const shownDifferences = 5

// DiffRows reports, under the case's pytest node id, every row where got
// differs from want: the row's index, each differing field with both values,
// and rows missing or extra at the end. It returns whether the two agree.
func DiffRows(t testing.TB, node string, want, got []Row) bool {
	t.Helper()
	var report []string
	differing := 0
	for i := 0; i < max(len(want), len(got)); i++ {
		var line string
		switch {
		case i >= len(got):
			line = fmt.Sprintf("row %d missing: %s", i, rowSummary(want[i]))
		case i >= len(want):
			line = fmt.Sprintf("row %d extra: %s", i, rowSummary(got[i]))
		default:
			line = rowFieldDifferences(i, want[i], got[i])
		}
		if line == "" {
			continue
		}
		differing++
		if differing <= shownDifferences {
			report = append(report, line)
		}
	}
	if differing == 0 {
		return true
	}
	if differing > shownDifferences {
		report = append(report, fmt.Sprintf("and %d more rows", differing-shownDifferences))
	}
	t.Errorf("%s: %d of %d rows differ (got %d):\n%s", node, differing, len(want), len(got),
		strings.Join(report, "\n"))
	return false
}

// rowFieldDifferences returns one line per field where two rows differ, under
// the row's index, or "" when they are equal.
func rowFieldDifferences(index int, want, got Row) string {
	wanted, gotten := reflect.ValueOf(want), reflect.ValueOf(got)
	var fields []string
	for i := range wanted.NumField() {
		left, right := wanted.Field(i).Interface(), gotten.Field(i).Interface()
		if left != right {
			name := wanted.Type().Field(i).Tag.Get("json")
			fields = append(fields, fmt.Sprintf("  %s: want %q, got %q", name, left, right))
		}
	}
	if len(fields) == 0 {
		return ""
	}
	return fmt.Sprintf("row %d (line %d):\n%s", index, want.SourceLine, strings.Join(fields, "\n"))
}

// rowSummary returns the fields that identify a row in a report.
func rowSummary(row Row) string {
	return fmt.Sprintf("%s:%d %s %q", row.SourceFile, row.SourceLine, row.TurnType, row.Text)
}

// DiffText reports, under the case's node id and a label naming the text
// (stdout, timeline.jsonl), the lines where got differs from want, and
// returns whether the two are equal.
func DiffText(t testing.TB, node, label, want, got string) bool {
	t.Helper()
	if want == got {
		return true
	}
	wanted, gotten := strings.SplitAfter(want, "\n"), strings.SplitAfter(got, "\n")
	var report []string
	differing := 0
	for i := 0; i < max(len(wanted), len(gotten)); i++ {
		left, right := lineAt(wanted, i), lineAt(gotten, i)
		if left == right {
			continue
		}
		differing++
		if differing <= shownDifferences {
			report = append(report, fmt.Sprintf("line %d:\n  want %q\n  got  %q", i+1, left, right))
		}
	}
	if differing > shownDifferences {
		report = append(report, fmt.Sprintf("and %d more lines", differing-shownDifferences))
	}
	t.Errorf("%s: %s differs in %d lines:\n%s", node, label, differing, strings.Join(report, "\n"))
	return false
}

// lineAt returns the line at index, or "" past the end.
func lineAt(lines []string, index int) string {
	if index < len(lines) {
		return lines[index]
	}
	return ""
}
