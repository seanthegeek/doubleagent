package dedup

import (
	"testing"

	"github.com/seanthegeek/doubleagent/internal/secrets"
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
	if primary.Secrets != primaryRecord || len(primaryRecord.Secrets()) != 0 {
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
