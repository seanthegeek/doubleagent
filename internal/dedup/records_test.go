package dedup

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/secrets"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

func TestARowKeptForADuplicateCarriesTheDuplicatesRecordToo(t *testing.T) {
	recorder := secrets.NewRecorder(secrets.Setting{Withhold: true, Placeholder: "P"})
	primaryRecord := recorder.Begin()
	recorder.End()
	firstCopy := recorder.Begin()
	recorder.Value("pw1")
	recorder.End()
	secondCopy := recorder.Begin()
	recorder.Value("pw2")
	recorder.End()
	text := set("text", "use pw1 and pw2")
	primary := primaryRow(baseTime, text)
	primary.Secrets = primaryRecord
	// The first copy pairs with the primary row; the second, from another
	// backup, pairs with the first copy among the secondary rows.
	copied := historyRow(baseTime, text, set("file", backupA))
	copied.Secrets = firstCopy
	again := historyRow(baseTime, text, set("file", backupB))
	again.Secrets = secondCopy
	kept, _ := Deduplicate(rows(primary), withTolerance(2, copied, again), nil)
	if len(kept) != 1 {
		t.Fatalf("kept %d rows", len(kept))
	}
	if got := recorder.RowText(kept[0].Text, kept[0].Secrets); got != "use P and P" {
		t.Errorf("the kept row's text is written %q", got)
	}
	want := []*secrets.Found{primaryRecord, firstCopy, secondCopy}
	if got := kept[0].Secrets.Records(); !slices.Equal(got, want) {
		t.Errorf("the kept row lists the records %p, want its own and each copy's, %p", got, want)
	}
	if len(primaryRecord.Secrets()) != 0 {
		t.Error("the primary row's own record changed")
	}
}

func TestARowKeptWithoutDuplicatesKeepsItsOwnRecord(t *testing.T) {
	record := &secrets.Found{}
	primary := primaryRow(baseTime)
	primary.Secrets = record
	other := historyRow(baseTime, set("text", "something else"))
	kept, _ := Deduplicate(rows(primary), withTolerance(2, other), nil)
	if len(kept) != 2 || kept[0].Secrets != record || kept[1].Secrets != nil {
		t.Errorf("kept %+v", kept)
	}
}

// counter is a textset.Counter that keeps its counts by name.
type counter map[string]int

// Add counts n units of the named work.
func (c counter) Add(name string, n int) { c[name] += n }

// carryWork deduplicates n primary rows, each of its own parse, against n
// copies of them from one parse that recorded n values, one in each copy's
// text, and writes the kept rows. It returns the records the kept rows
// list, the bytes the replacement indexed and searched writing them, and
// whether each kept row's text was written with its value replaced.
func carryWork(n int) (int, int, bool) {
	recorder := secrets.NewRecorder(secrets.Setting{Withhold: true, Placeholder: "P"})
	primary := make([]timeline.Row, n)
	for i := range primary {
		primary[i] = primaryRow(baseTime, set("text", fmt.Sprintf("message %06d uses k%05d", i, i)))
		primary[i].Secrets = recorder.Begin()
		recorder.End()
	}
	copied := recorder.Begin()
	secondary := make([]timeline.Row, n)
	for i := range secondary {
		recorder.Value(fmt.Sprintf("k%05d", i))
		secondary[i] = historyRow(baseTime, set("text", primary[i].Text))
		secondary[i].Secrets = copied
	}
	recorder.End()
	kept, _ := Deduplicate(primary, withTolerance(2, secondary...), nil)
	listed, replaced := 0, len(kept) == n
	work := counter{}
	recorder.CountWork(work)
	for _, row := range kept {
		listed += len(row.Secrets.Records())
		written := recorder.RowText(row.Text, row.Secrets)
		replaced = replaced && strings.HasSuffix(written, " uses P")
	}
	return listed, work["bytes indexed"] + work["bytes searched"], replaced
}

func TestCarryingAndWritingDuplicatesRecordsGrowsWithTheRowsNotWithWhatTheRecordsHold(t *testing.T) {
	smallListed, smallWork, smallReplaced := carryWork(500)
	listed, work, replaced := carryWork(1000)
	if !smallReplaced || !replaced {
		t.Fatal("a kept row's text keeps the value its dropped copy's parse recorded")
	}
	// Each kept row lists its own record and the copies' one, and every
	// text is the same length, so twice the rows is exactly twice the work.
	if smallListed != 2*500 || listed != 2*smallListed {
		t.Errorf("the kept rows list %d and %d records for 500 and 1000 rows, want 1000 and 2000", smallListed,
			listed)
	}
	if work != 2*smallWork || smallWork <= 0 {
		t.Errorf("writing the rows indexed and searched %d and %d bytes for 500 and 1000 rows: not linear",
			smallWork, work)
	}
}

// formWork deduplicates one primary row of 100*n bytes against n short
// copies, each in its own file and of its own parse that recorded one short
// value, which a Form hook pairs with the long row, and writes the kept
// rows. It returns the rows kept, the bytes the replacement indexed and
// searched writing them, and whether every value was replaced.
func formWork(n int) (int, int, bool) {
	recorder := secrets.NewRecorder(secrets.Setting{Withhold: true, Placeholder: "P"})
	var long strings.Builder
	for i := range n {
		chunk := fmt.Sprintf("/x v%04d ", i)
		long.WriteString(chunk + strings.Repeat("a", 100-len(chunk)))
	}
	primary := primaryRow(baseTime, set("text", long.String()))
	primary.Secrets = recorder.Begin()
	recorder.End()
	copies := make([]timeline.Row, n)
	for i := range copies {
		copies[i] = historyRow(baseTime, set("text", "/x"), set("file", fmt.Sprintf("/b/%d/history.jsonl", i)))
		copies[i].Secrets = recorder.Begin()
		recorder.Value(fmt.Sprintf("v%04d", i))
		recorder.End()
	}
	hooks := Hooks{"claude-code": AgentHooks{Form: func(row timeline.Row, text *MatchText) string {
		if strings.HasPrefix(row.Text, "/x v0000") {
			text.Write("/x")
		} else {
			text.WriteRow(row)
		}
		return row.TurnType
	}}}
	kept, _ := Deduplicate(rows(primary), withTolerance(2, copies...), hooks)
	work := counter{}
	recorder.CountWork(work)
	replaced := true
	for _, row := range kept {
		written := recorder.RowText(row.Text, row.Secrets)
		replaced = replaced && strings.Count(written, "/x P ") == n
	}
	return len(kept), work["bytes indexed"] + work["bytes searched"], replaced
}

func TestWritingARowKeptForManyShortCopiesCostsItsLengthPlusTheirRecords(t *testing.T) {
	smallKept, small, smallReplaced := formWork(500)
	kept, work, replaced := formWork(1000)
	if smallKept != 1 || kept != 1 || !smallReplaced || !replaced {
		t.Fatalf("kept %d and %d rows, every value replaced: %t and %t", smallKept, kept, smallReplaced, replaced)
	}
	// The row is searched once for all its copies' records, which are
	// indexed once: 100 bytes of text and 5 of values per copy.
	if small != 105*500 || work != 105*1000 {
		t.Errorf("writing the row indexed and searched %d and %d bytes for 500 and 1000 copies, want %d and %d",
			small, work, 105*500, 105*1000)
	}
}
