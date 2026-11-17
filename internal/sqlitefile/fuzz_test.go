package sqlitefile

import "testing"

// Each fuzz target opens what it is given and walks the schema and every
// tree: reading damaged input must return, never panic or hang.

func FuzzMainFile(fuzz *testing.F) {
	fuzz.Add([]byte{})
	for _, name := range []string{"page_512.db", "page_1024.db", "shapes.db", "autovacuum.db",
		"log_fresh.db", "utf16le.db"} {
		fuzz.Add(testdataBytes(fuzz, name))
	}
	fuzz.Fuzz(func(_ *testing.T, main []byte) {
		walkEverything(OpenBytes(main, nil, nil))
	})
}

func FuzzLog(fuzz *testing.F) {
	for _, name := range []string{"log_rows.db", "log_big_endian.db", "log_reset.db", "log_fresh.db"} {
		fuzz.Add(testdataBytes(fuzz, name+"-wal"))
	}
	main := testdataBytes(fuzz, "log_rows.db")
	fuzz.Fuzz(func(_ *testing.T, log []byte) {
		walkEverything(OpenBytes(main, log, nil))
	})
}

func FuzzJournal(fuzz *testing.F) {
	for _, name := range []string{"journal_hot.db", "journal_super.db", "journal_count_all.db"} {
		fuzz.Add(testdataBytes(fuzz, name+"-journal"))
	}
	main := testdataBytes(fuzz, "journal_hot.db")
	fuzz.Fuzz(func(_ *testing.T, journal []byte) {
		walkEverything(OpenBytes(main, nil, journal))
	})
}
