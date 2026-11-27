package parserkit

import (
	"encoding/binary"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/vscodestate"
)

// databaseMessages returns every text a parser can put in a row for the
// database at path: the kit's message, the opener's and the reader's errors
// as the parsers word them, and the editor state reader's problems.
func databaseMessages(t *testing.T, path string) []string {
	t.Helper()
	var messages []string
	database, err := OpenDatabase(path)
	if err != nil {
		messages = append(messages, DatabaseError(err))
	}
	if database != nil {
		messages = append(messages, readError(t, database)...)
	}
	if database, err := sqlitefile.OpenPath(path); err != nil {
		messages = append(messages, err.Error(), DatabaseError(err))
	} else {
		messages = append(messages, readError(t, database)...)
	}
	_, problems, _ := vscodestate.ReadItems(path, []string{"key"})
	return append(messages, problems...)
}

// readError reads the schema of an open database and the rows of the
// table on its second page, which damagedPageTwo damages, and returns the
// read error's text and the kit's message for it, or nothing when both
// reads work.
func readError(t *testing.T, database *sqlitefile.Database) []string {
	t.Helper()
	defer func() {
		if err := database.Close(); err != nil {
			t.Error(err)
		}
	}()
	found, err := database.HasTable("local_thread_catalog")
	if err == nil && found {
		var table *sqlitefile.Table
		if table, err = database.Table("local_thread_catalog"); err == nil {
			_, err = table.Select().All()
		}
	}
	if err == nil {
		return nil
	}
	return []string{err.Error(), DatabaseError(err)}
}

// damagedPageTwo returns a copy of a test database whose second page has a
// page type no b-tree page has, so it opens and fails when read.
func damagedPageTwo(t *testing.T) []byte {
	t.Helper()
	data, err := osfile.ReadFile(filepath.Join("..", "sqlitefile", "testdata", "sites.db"))
	if err != nil {
		t.Fatal(err)
	}
	pageSize := int(binary.BigEndian.Uint16(data[16:]))
	data[pageSize] = 0xff
	return data
}

func TestDatabaseMessagesNeverNameTheDirectory(t *testing.T) {
	directory := t.TempDir()
	cases := []struct {
		name string
		data []byte
	}{
		{"a missing database", nil},
		{"a damaged header", append([]byte("SQLite format 3\x00"), make([]byte, 200)...)},
		{"a database cut short", damagedPageTwo(t)[:100]},
		{"a damaged page", damagedPageTwo(t)},
	}
	for _, test := range cases {
		t.Run(test.name+" is reported without its directory", func(t *testing.T) {
			path := filepath.Join(directory, "state.vscdb")
			if test.data != nil {
				if err := osfile.WriteFile(path, test.data, 0o600); err != nil {
					t.Fatal(err)
				}
				defer osfile.Remove(path)
			}
			assertNoDirectory(t, directory, databaseMessages(t, path))
		})
	}
}

// assertNoDirectory fails the test when any message holds the directory's
// path, and when none says anything, since then nothing was checked.
func assertNoDirectory(t *testing.T, directory string, messages []string) {
	t.Helper()
	said := false
	for _, message := range messages {
		said = said || message != ""
		if strings.Contains(message, directory) {
			t.Errorf("message %q names the directory %s", message, directory)
		}
	}
	if !said {
		t.Error("no message was given")
	}
}
