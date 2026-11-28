package dedup

import (
	"math/big"
	"math/rand/v2"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/timeline"
)

func TestMulModIsTheProductModuloThePrime(t *testing.T) {
	random := rand.New(rand.NewPCG(7, 8))
	prime := big.NewInt(modulus)
	values := []uint64{0, 1, 2, modulus - 1, modulus - 2, 1 << 60, 1<<61 - 3}
	for range 1000 {
		values = append(values, random.Uint64N(modulus))
	}
	for i, left := range values {
		right := values[(i*7+3)%len(values)]
		want := new(big.Int).Mul(new(big.Int).SetUint64(left), new(big.Int).SetUint64(right))
		if got := mulMod(left, right); got != want.Mod(want, prime).Uint64() {
			t.Fatalf("mulMod(%d, %d) = %d, want %d", left, right, got, want.Uint64())
		}
	}
}

// randomBytes returns up to limit random bytes.
func randomBytes(random *rand.Rand, limit int) string {
	text := make([]byte, random.IntN(limit+1))
	for i := range text {
		text[i] = byte(random.UintN(256))
	}
	return string(text)
}

func TestTheDigestOfTwoTextsJoinedIsTheDigestOfTheirBytes(t *testing.T) {
	random := rand.New(rand.NewPCG(9, 10))
	for _, base := range []uint64{1, 2, 256, randomBase()} {
		digests := hasher{base}
		for range 500 {
			left, right := randomBytes(random, 40), randomBytes(random, 40)
			joined := digests.join(digests.extend(digest{}, left), digests.extend(digest{}, right))
			if whole := digests.extend(digest{}, left+right); joined != whole {
				t.Fatalf("base %d: digest of %q and %q joined is %v, of the bytes %v", base, left, right,
					joined, whole)
			}
		}
	}
}

// builtRow returns a row of pieces: "#" starts a reference to the shared
// text of that name in named, made on first use, and any other piece is
// literal.
func builtRow(named map[string]*timeline.SharedText, pieces ...string) timeline.Row {
	row := timeline.Row{TimestampUTC: "2026-10-01T10:00:00.000Z", Host: "h1", User: "alice", Agent: "plain",
		TurnType: timeline.TurnUser, SourceFile: "/h/history", SourceLine: 1}
	var out timeline.TextBuilder
	for _, piece := range pieces {
		name, isShared := strings.CutPrefix(piece, "#")
		if !isShared {
			out.Write(piece)
			continue
		}
		if named[name] == nil {
			named[name] = timeline.NewSharedText(name)
		}
		out.Share(named[name], "[shared]", func(*timeline.Row, bool) string { return "[pointer]" })
	}
	out.Set(&row)
	return row
}

func TestAReferenceIsKeyedByTheBytesItHoldsInFull(t *testing.T) {
	named := map[string]*timeline.SharedText{}
	var refs []timeline.SharedRef
	for _, text := range []string{" a b ", " a ", "a", " ", "\t a"} {
		for _, pieces := range [][]string{{"#" + text}, {"#" + text, "x"}, {"x", "#" + text},
			{"x", "#" + text, "y"}, {"#" + text, "#" + text}} {
			row := builtRow(named, pieces...)
			if row.Shared != nil {
				refs = append(refs, *row.Shared...)
			}
		}
	}
	for _, left := range refs {
		for _, right := range refs {
			same := left.Shared == right.Shared && left.Full == right.Full
			if got := keyOf(left) == keyOf(right); got != same {
				t.Errorf("references to %q holding %q and %q: one key %t, want %t", left.Shared.Text(),
					left.Full, right.Full, got, same)
			}
		}
	}
}

// classesOf returns the class of each row's text as it reads in full in
// one run with base.
func classesOf(rows []timeline.Row, base uint64) []int {
	texts := newTexts(base)
	classes := make([]int, len(rows))
	for i, row := range rows {
		text := texts.NewText()
		text.WriteRow(row)
		classes[i] = texts.classOf(text)
	}
	return classes
}

func TestTwoRowsShareAClassExactlyWhenTheirFullTextsAreEqual(t *testing.T) {
	random := rand.New(rand.NewPCG(11, 12))
	named := map[string]*timeline.SharedText{}
	words := []string{"ab", "ba", "a", "b", "abab", "x y", " ", "aab", " "}
	var rows []timeline.Row
	for range 3000 {
		pieces := make([]string, random.IntN(4)+1)
		for i := range pieces {
			pieces[i] = words[random.IntN(len(words))]
			if random.IntN(2) == 0 {
				pieces[i] = "#" + pieces[i]
			}
		}
		rows = append(rows, builtRow(named, pieces...))
	}
	// With a base of 1 a digest is the sum of the bytes, so texts that
	// hold the same bytes in another order share one.
	for _, base := range []uint64{1, randomBase()} {
		classes := classesOf(rows, base)
		for i := range rows {
			for j := range i {
				if want := rows[i].FullText() == rows[j].FullText(); (classes[i] == classes[j]) != want {
					t.Fatalf("base %d: %q and %q share a class: %t, want %t", base, rows[i].FullText(),
						rows[j].FullText(), classes[i] == classes[j], want)
				}
			}
		}
	}
}

func TestARowPairsOnlyWithARecordOfTheSameBytesWhenTheirDigestsCollide(t *testing.T) {
	long := strings.Repeat("0123456789", 8)
	tests := []struct {
		name    string
		record  string
		pieces  []string
		dropped bool
	}{
		{"the same bytes", "x " + long + " y", []string{"x ", "#" + long, " y"}, true},
		{"two bytes swapped outside the shared text", "x " + long + " y", []string{" x", "#" + long, " y"}, false},
		{"two bytes swapped inside the shared text", "x " + long + " y",
			[]string{"x ", "#1023456789" + long[10:], " y"}, false},
		{"one byte changed inside the shared text, its sum kept by one outside", "x " + long + " z",
			[]string{"x ", "#" + long[:79] + "8", " {"}, false},
		{"the same bytes in another arrangement", "x " + long + " y", []string{"x " + long[:5], "#" + long[5:] + " y"},
			true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			secondary := builtRow(map[string]*timeline.SharedText{}, test.pieces...)
			primary := secondary
			primary.Text, primary.Shared, primary.SourceFile = test.record, nil, "/t/transcript"
			kept, counts, _ := deduplicate([]timeline.Row{primary}, []Secondary{{Row: secondary, Tolerance: 2}},
				nil, newTexts(1))
			if got := counts["plain"] == 1; got != test.dropped || len(kept) != 2-counts["plain"] {
				t.Errorf("dropped %t (%d rows kept), want %t", got, len(kept), test.dropped)
			}
		})
	}
}

// namingRows returns a transcript record and 2n history rows that name
// one shared text of size bytes: n with a prompt of their own, each in its
// own session, then the same n again from another file, the record equal
// to the first. The record takes the first row and each copy the row it
// repeats.
func namingRows(n, size int) ([]timeline.Row, []Secondary) {
	named := map[string]*timeline.SharedText{strings.Repeat("p", size): nil}
	shared := strings.Repeat("p", size)
	secondary := make([]Secondary, 0, 2*n)
	for i := range 2 * n {
		row := builtRow(named, "prompt "+strconv.Itoa(i%n)+" ", "#"+shared, " end")
		row.SessionID, row.SourceFile = "s"+strconv.Itoa(i%n), "/h/history"+strconv.Itoa(i/n)
		secondary = append(secondary, Secondary{Row: row, Tolerance: 2})
	}
	record := secondary[0].Row
	record.Text, record.Shared, record.SourceFile = record.FullText(), nil, "/t/transcript"
	return []timeline.Row{record}, secondary
}

func TestDeduplicationReadsASharedTextOnceHoweverManyRowsNameIt(t *testing.T) {
	read := map[int]int{}
	for _, n := range []int{500, 1000} {
		primary, secondary := namingRows(n, 1<<16)
		texts := newTexts(randomBase())
		_, counts, _ := deduplicate(primary, secondary, nil, texts)
		if counts["plain"] != n+1 {
			t.Fatalf("%d rows dropped of %d, want %d", counts["plain"], 2*n, n+1)
		}
		read[n] = texts.SharedRead()
	}
	if read[500] != read[1000] || read[500] > 4<<16 {
		t.Errorf("deduplicating 1000 and 2000 rows read %d and %d bytes of their shared text, want one"+
			" same number under %d", read[500], read[1000], 4<<16)
	}
}
