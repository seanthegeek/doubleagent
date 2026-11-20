package timeline

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// piece is one piece of a text a test builds: literal text, or a shared
// text with its stand-in.
type piece struct {
	text    string
	shared  *SharedText
	standIn string
}

// built returns the row of the pieces, built by a TextBuilder, each shared
// text's pointer naming the holder's source line.
func built(pieces ...piece) Row {
	var out TextBuilder
	for _, each := range pieces {
		if each.shared == nil {
			out.Write(each.text)
			continue
		}
		out.Share(each.shared, each.standIn, func(holder *Row, sameRow bool) string {
			if sameRow {
				return "<" + each.standIn + " above>"
			}
			return "<" + each.standIn + " at line " + strconv.Itoa(holder.SourceLine) + ">"
		})
	}
	var row Row
	out.Set(&row)
	return row
}

func TestABuiltTextReadsInFullAsTheCompactedTextOfItsPieces(t *testing.T) {
	spaced, blank := NewSharedText("  inner text \n"), NewSharedText(" \t ")
	tests := []struct {
		name   string
		pieces []piece
		want   string
	}{
		{"literal text alone", []piece{{text: "  a b  "}}, "a b"},
		{"a shared text at each end is trimmed there", []piece{{shared: spaced, standIn: "S"}, {text: " - "},
			{shared: spaced, standIn: "S"}}, "inner text \n - " + "  inner text"},
		{"a shared text of white space at an end names nothing", []piece{{text: " x "}, {shared: blank, standIn: "B"}},
			"x"},
		{"white space before a shared text goes", []piece{{text: "\n "}, {shared: spaced, standIn: "S"}, {text: "!"}},
			"inner text \n!"},
		{"nothing at all", []piece{{text: " "}, {shared: blank, standIn: "B"}}, ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var whole strings.Builder
			for _, each := range test.pieces {
				whole.WriteString(each.text)
				if each.shared != nil {
					whole.WriteString(each.shared.Text())
				}
			}
			row := built(test.pieces...)
			if got := row.FullText(); got != test.want || got != Compact(whole.String()) {
				t.Errorf("full text %q, want %q", got, test.want)
			}
		})
	}
}

func TestResolveWritesEachSharedTextInFullAtItsFirstRowOnly(t *testing.T) {
	first, second := NewSharedText("FIRST"), NewSharedText("SECOND")
	rows := []Row{
		built(piece{text: "plain"}),
		built(piece{text: "a "}, piece{shared: first, standIn: "f"}, piece{text: " "}, piece{shared: first, standIn: "f"}),
		built(piece{shared: second, standIn: "s"}, piece{text: " b "}, piece{shared: first, standIn: "f"}),
		built(piece{text: "c "}, piece{shared: second, standIn: "s"}),
	}
	for i := range rows {
		rows[i].SourceLine = i + 1
	}
	Resolve(rows)
	var texts []string
	for _, row := range rows {
		texts = append(texts, row.Text)
		if row.Shared != nil {
			t.Errorf("row %d still names shared texts", row.SourceLine)
		}
	}
	want := []string{"plain", "a FIRST <f above>", "SECOND b <f at line 2>", "c <s at line 3>"}
	if !slices.Equal(texts, want) {
		t.Errorf("got %q, want %q", texts, want)
	}
}

func TestTheRowsGrowWithTheSharedTextNotWithTheRowsNamingIt(t *testing.T) {
	size := func(n int) int {
		shared := NewSharedText(strings.Repeat("x", 10*n))
		rows := make([]Row, n)
		for i := range rows {
			rows[i] = built(piece{text: "row " + strconv.Itoa(i) + " "}, piece{shared: shared, standIn: "s"})
			rows[i].SourceLine = i + 1
		}
		Resolve(rows)
		total := 0
		for _, row := range rows {
			total += len(row.Text)
		}
		return total
	}
	// Twice the rows naming a text twice as long gives about twice the
	// bytes; the text in every row would give four times as many.
	small, large := size(500), size(1000)
	if small < 10*500 || large > 5*small/2 {
		t.Errorf("%d bytes for 500 rows, %d for 1000", small, large)
	}
}

func TestARowMatchesAPatternAsItsTextReadsInFull(t *testing.T) {
	shared := NewSharedText("middle part")
	row := built(piece{text: "start "}, piece{shared: shared, standIn: "[stand-in]"}, piece{text: " end"})
	for _, expression := range []string{"start middle", `^start middle part end\z`, `\bpart\b`, "t m", "stand-in",
		"^middle", `part end$`, "é", ""} {
		t.Run(expression, func(t *testing.T) {
			pattern := regexp.MustCompile(expression)
			if got, want := row.Matches(pattern), pattern.MatchString(row.FullText()); got != want {
				t.Errorf("Matches = %v, want %v", got, want)
			}
		})
	}
}
