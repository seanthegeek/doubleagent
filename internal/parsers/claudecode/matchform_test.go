package claudecode

import (
	"math/rand/v2"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/dedup"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// The prompt form and typed-command test as regular expressions over the
// text read in full, which the piecewise forms must agree with.
var (
	wrapperTagPattern   = regexp.MustCompile(`</?pasted_content(?:[ \t\r\n][^>]*)?>`)
	typedCommandPattern = regexp.MustCompile(`^/[A-Za-z][A-Za-z0-9:_-]*(?:[ \t]|\z)`)
)

// promptByPattern returns a prompt's match form computed over its text in
// full with the regular expressions.
func promptByPattern(text string) string {
	return squash(wrapperTagPattern.ReplaceAllLiteralString(text, " "))
}

// formTokens are the pieces generated texts are made of: parts of wrapper
// tags, brackets, ASCII and other white space, and letters.
var formTokens = []string{"<pasted_content", "</pasted_content", "<pasted_", "content", "<", "</",
	"p", ">", " ", "\t", "\n", "\r", "\f", "\v", "  ", " ", "x", "/", "id=\"1\"", "/m", "a:b-c_d",
	"<pasted_content>", "</pasted_content>", "<pasted_content id=\"0\">", "\xff"}

// generatedText returns up to limit tokens drawn by random.
func generatedText(random *rand.Rand, limit int) string {
	var text strings.Builder
	for range random.IntN(limit + 1) {
		text.WriteString(formTokens[random.IntN(len(formTokens))])
	}
	return text.String()
}

// generatedRow returns a user row of literal texts and references to the
// shared texts, drawn by random; a shared text may be named twice.
func generatedRow(random *rand.Rand, shared []*timeline.SharedText) timeline.Row {
	row := timeline.Row{TurnType: timeline.TurnUser}
	var out timeline.TextBuilder
	for range random.IntN(5) + 1 {
		if random.IntN(2) == 0 {
			out.Write(generatedText(random, 4))
			continue
		}
		out.Share(shared[random.IntN(len(shared))], "[Pasted text #1]", pastePointer("[Pasted text #1]"))
	}
	out.Set(&row)
	return row
}

// generatedShared returns n shared texts drawn by random.
func generatedShared(random *rand.Rand, n int) []*timeline.SharedText {
	shared := make([]*timeline.SharedText, n)
	for i := range shared {
		shared[i] = timeline.NewSharedText(generatedText(random, 8))
	}
	return shared
}

func TestThePromptFormOfARowInPiecesEqualsItsFormReadInFull(t *testing.T) {
	random := rand.New(rand.NewPCG(1, 2))
	shared := generatedShared(random, 12)
	texts := dedup.NewTexts()
	for i := range 20000 {
		row := generatedRow(random, shared)
		text := texts.NewText()
		turn := MatchForm(row, text)
		if want := promptByPattern(row.FullText()); turn != timeline.TurnUser || text.String() != want {
			t.Fatalf("row %d %q: form %q, want %q", i, row.FullText(), text.String(), want)
		}
	}
}

func TestATypedCommandIsFoundAcrossPastes(t *testing.T) {
	tests := []struct {
		name   string
		pieces []string
		want   bool
	}{
		{"a name in a paste after the slash", []string{"/", "#model"}, true},
		{"a slash and part of a name in a paste", []string{"#/mo", "del x"}, true},
		{"a name across two pastes", []string{"#/m", "#od", "el"}, true},
		{"a name ending in a paste at a tab", []string{"/m", "#od\tx"}, true},
		{"a name ending in a paste at a dot", []string{"/m", "#o.d"}, false},
		{"a paste of a slash alone", []string{"#/", "9"}, false},
		{"a paste of a name with no slash", []string{"#model"}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			row := timeline.Row{TurnType: timeline.TurnUser}
			var out timeline.TextBuilder
			for _, piece := range test.pieces {
				if paste, ok := strings.CutPrefix(piece, "#"); ok {
					out.Share(timeline.NewSharedText(paste), "[Pasted text #1]", pastePointer("[Pasted text #1]"))
				} else {
					out.Write(piece)
				}
			}
			out.Set(&row)
			texts := dedup.NewTexts()
			for range 2 {
				if _, typed := MatchWindow(row, texts); typed != test.want {
					t.Errorf("typed %t, want %t", typed, test.want)
				}
			}
		})
	}
}

func TestTheTypedCommandTestOfARowInPiecesEqualsItsTestReadInFull(t *testing.T) {
	random := rand.New(rand.NewPCG(3, 4))
	shared := generatedShared(random, 200)
	texts := dedup.NewTexts()
	for i := range 20000 {
		row := generatedRow(random, shared)
		_, typed := MatchWindow(row, texts)
		if want := typedCommandPattern.MatchString(row.FullText()); typed != want {
			t.Fatalf("row %d %q: typed %t, want %t", i, row.FullText(), typed, want)
		}
	}
}

// pairedRows returns, for each of n generated history rows, its own
// session's transcript record: one whose text is the history row's in full
// (when exact) or that text with one byte changed, inside or outside the
// shared texts the row names.
func pairedRows(random *rand.Rand, n int) ([]timeline.Row, []dedup.Secondary) {
	shared := generatedShared(random, 6)
	primary := make([]timeline.Row, 0, n)
	secondary := make([]dedup.Secondary, 0, n)
	for i := range n {
		history := generatedRow(random, shared)
		history.Agent, history.SessionID = "claude-code", "s"+strconv.Itoa(i)
		history.TimestampUTC = "2026-10-01T10:00:00.000Z"
		history.SourceFile = "/h/history.jsonl"
		record := history
		record.Text, record.Shared, record.SourceFile = history.FullText(), nil, "/p/s.jsonl"
		if record.Text != "" && random.IntN(2) == 0 {
			changed := []byte(record.Text)
			changed[random.IntN(len(changed))] = formTokens[random.IntN(len(formTokens))][0]
			record.Text = string(changed)
		}
		primary = append(primary, record)
		secondary = append(secondary, dedup.Secondary{Row: history, Tolerance: dedup.ToleranceSeconds})
	}
	return primary, secondary
}

func TestAHistoryRowPairsWithItsRecordExactlyWhenTheirFormsReadInFullAreEqual(t *testing.T) {
	random := rand.New(rand.NewPCG(5, 6))
	primary, secondary := pairedRows(random, 4000)
	hooks := dedup.Hooks{"claude-code": New().DedupHooks()}
	kept, _ := dedup.Deduplicate(primary, secondary, hooks)
	keptSessions := map[string]bool{}
	for _, row := range kept[len(primary):] {
		keptSessions[row.SessionID] = true
	}
	paired, apart := 0, 0
	for i, item := range secondary {
		want := promptByPattern(primary[i].Text) == promptByPattern(item.Row.FullText())
		if got := !keptSessions[item.Row.SessionID]; got != want {
			t.Errorf("history %q and record %q: paired %t, want %t", item.Row.FullText(), primary[i].Text,
				got, want)
		}
		if want {
			paired++
		} else {
			apart++
		}
	}
	if paired < 1000 || apart < 1000 {
		t.Errorf("the mix has %d pairs and %d near misses, want 1000 of each", paired, apart)
	}
}

// pasteRows returns n history rows that each name one paste of size bytes
// after a prompt of their own, and as many again that are the same line
// typed again, with the transcript record of the first.
func pasteRows(n, size int) ([]timeline.Row, []timeline.Row) {
	paste := timeline.NewSharedText("<pasted_content id=\"1\">" + strings.Repeat("line of a paste\n", size/16) + ">")
	history := make([]timeline.Row, 0, 2*n)
	for i := range 2 * n {
		prompt := "/review " + strconv.Itoa(i%n) + " "
		row := timeline.Row{TurnType: timeline.TurnUser, Agent: "claude-code"}
		var out timeline.TextBuilder
		out.Write(prompt)
		out.Share(paste, "[Pasted text #1]", pastePointer("[Pasted text #1]"))
		out.Write(" and </pasted_content")
		out.Set(&row)
		history = append(history, row)
	}
	record := history[0]
	record.Text, record.Shared = history[0].FullText(), nil
	return []timeline.Row{record}, history
}

func TestThePromptFormReadsAPasteOnceHoweverManyLinesNameIt(t *testing.T) {
	read := map[int]int{}
	for _, n := range []int{500, 1000} {
		records, history := pasteRows(n, 1<<16)
		texts := dedup.NewTexts()
		for _, row := range append(records, history...) {
			MatchForm(row, texts.NewText())
			MatchWindow(row, texts)
		}
		read[n] = texts.SharedRead()
	}
	if read[500] != read[1000] || read[500] > 8<<16 {
		t.Errorf("the forms of 1000 and 2000 lines read %d and %d bytes of their paste, want one same"+
			" number under %d", read[500], read[1000], 8<<16)
	}
}

// A prompt is evidence: whatever its text and pastes, its match form and
// typed-command test read in pieces must equal those of its text in full.
func FuzzThePromptFormInPiecesEqualsItsFormReadInFull(fuzz *testing.F) {
	fuzz.Add("say <pasted_", "content id=1", "> x </pasted_content>")
	fuzz.Add("/mo", "del", "\tx")
	fuzz.Add(" <pasted_content\n", " a ", "b")
	fuzz.Fuzz(func(t *testing.T, before, paste, after string) {
		shared := timeline.NewSharedText(paste)
		row := timeline.Row{TurnType: timeline.TurnUser}
		var out timeline.TextBuilder
		out.Write(before)
		out.Share(shared, "[Pasted text #1]", pastePointer("[Pasted text #1]"))
		out.Write(after)
		out.Share(shared, "[Pasted text #1]", pastePointer("[Pasted text #1]"))
		out.Set(&row)
		texts := dedup.NewTexts()
		text := texts.NewText()
		MatchForm(row, text)
		if want := promptByPattern(row.FullText()); text.String() != want {
			t.Errorf("form %q, want %q", text.String(), want)
		}
		if _, typed := MatchWindow(row, texts); typed != typedCommandPattern.MatchString(row.FullText()) {
			t.Errorf("typed %t for %q", typed, row.FullText())
		}
	})
}
