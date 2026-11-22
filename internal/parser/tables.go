package parser

import "github.com/seanthegeek/doubleagent/internal/sqlitefile"

// Tables answers which tables a database holds, matching a name as SQL
// matches a table name: ASCII letters in any case, so a table the schema
// spells Messages is the messages table. The zero Tables holds none.
type Tables struct {
	database *sqlitefile.Database
}

// TablesOf reads the schema of database and returns its tables, or the
// error reading the schema met.
func TablesOf(database *sqlitefile.Database) (Tables, error) {
	if _, err := database.HasTable(""); err != nil {
		return Tables{}, err
	}
	return Tables{database: database}, nil
}

// Has reports whether the database holds a table called name. The schema
// was read without error when the Tables was made, and the database keeps
// what it read, so asking again meets no error.
func (t Tables) Has(name string) bool {
	if t.database == nil {
		return false
	}
	found, err := t.database.HasTable(name)
	return err == nil && found
}
