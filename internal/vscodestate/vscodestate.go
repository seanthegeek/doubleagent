package vscodestate

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/textlines"
)

// secretPrefix starts the keys of SecretStorage rows, which are never read.
const secretPrefix = "secret://"

// ValueProblem starts the problem reported for a value that is not JSON.
const ValueProblem = "ItemTable value for "

// stateDatabase is the path, relative to a home, that ends in an editor's
// global state database.
const stateDatabase = "User/globalStorage/state.vscdb"

// EditorAgents returns the catalog agents whose User directory holds a VS
// Code family state.vscdb. An extension installed in a fork writes its
// global state to the fork's database, which the catalog attributes to the
// fork, so parsers for such extensions read the files of all of these.
func EditorAgents() []string {
	return []string{"vscode", "cursor", "windsurf", "pearai", "kiro", "antigravity"}
}

// IsStateDatabase reports whether rel, a path relative to a home, is an
// editor's global state database: User/globalStorage/state.vscdb at the
// start of rel or after a slash, in any ASCII case, as the catalog matches.
func IsStateDatabase(rel string) bool {
	return textlines.EqualFoldASCII(rel, stateDatabase) || textlines.HasSuffixFoldASCII(rel, "/"+stateDatabase)
}

// regularFile reports whether path's own entry is a regular file, a copy
// of parserkit.RegularFile: the kit's own tests (package parserkit) import
// this package, so it cannot import the kit.
func regularFile(path string) bool {
	info, err := osfile.Lstat(path)
	return err == nil && info.Mode()&fs.ModeType == 0
}

// openState opens the state database at path, or returns nil and the
// problem that says why it cannot: "<name> is not a SQLite database", the
// unreadable message of the file or a sidecar, or "state.vscdb unreadable:
// <reason>". It sorts the error as parserkit.OpenDatabase does, which it
// cannot call for the reason regularFile gives.
func openState(listing func(directory string) osfile.Spellings, path string,
) (*sqlitefile.Database, string) {
	database, err := sqlitefile.OpenPath(path, listing)
	var unreadable *sqlitefile.UnreadableError
	switch {
	case err == nil:
		return database, ""
	case errors.Is(err, sqlitefile.ErrNotDatabase):
		return nil, filepath.Base(path) + " is not a SQLite database"
	case errors.As(err, &unreadable):
		return nil, unreadable.Error()
	}
	return nil, "state.vscdb unreadable: " + err.Error()
}

// itemRows returns the ItemTable rows of database whose key is one of
// wanted, in key order, or none when the database has no ItemTable, its
// name matched as SQL matches a table name, in any ASCII case.
func itemRows(database *sqlitefile.Database, wanted []sqlitefile.Value) ([]sqlitefile.Row, error) {
	table, err := database.Table("ItemTable")
	if errors.Is(err, sqlitefile.ErrNoTable) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return table.Select("key", "value").Where(func(row sqlitefile.Row) bool {
		for _, key := range wanted {
			if row.Equals("key", key) {
				return true
			}
		}
		return false
	}).OrderBy("key").All()
}

// ReadItems returns the decoded JSON values of the requested ItemTable keys
// that are present, and the problems met. A value that is not JSON is
// returned as its text, a JSON string, and noted with the JSON reader's
// reason as readerError words it (parserkit.ReaderErrors), since it can
// quote the value; so is a database openState cannot open. An absent file,
// a symbolic link and a file that is not regular give nothing, and
// secret:// keys are never read. The member names the values repeat come
// last, for the parse of the database to report (parserkit.NoteRepeats).
// listing gives a directory's listing, where a sidecar absent as built is
// looked for (sqlitefile.OpenPath).
func ReadItems(listing func(directory string) osfile.Spellings, path string, keys []string,
	readerError func(error) string) (map[string]jsonvalue.Value, []string, jsonvalue.Repeats) {
	var repeats jsonvalue.Repeats
	values := map[string]jsonvalue.Value{}
	var wanted []sqlitefile.Value
	for _, key := range keys {
		if !strings.HasPrefix(key, secretPrefix) {
			wanted = append(wanted, sqlitefile.TextValue(key))
		}
	}
	if len(wanted) == 0 || !regularFile(path) {
		return values, nil, repeats
	}
	database, problem := openState(listing, path)
	if database == nil {
		return values, []string{problem}, repeats
	}
	rows, err := itemRows(database, wanted)
	database.Close()
	if err != nil {
		return values, []string{"state.vscdb unreadable: " + err.Error()}, repeats
	}
	var problems []string
	for _, row := range rows {
		key := row.Get("key")
		if key.Type != sqlitefile.Text || strings.HasPrefix(key.Text(), secretPrefix) {
			continue
		}
		text := row.Get("value").UTF8Text()
		document := jsonvalue.NewDocument(text)
		decoded, err := document.Decode()
		repeats.Merge(document.TakeRepeats())
		if err != nil {
			decoded = jsonvalue.NewString(text)
			problems = append(problems, ValueProblem+key.Text()+" is not valid JSON ("+readerError(err)+")")
		}
		values[key.Text()] = decoded
	}
	return values, problems, repeats
}

// ReadItem returns the decoded JSON value of one ItemTable key, JSON null
// when it is absent (the two read alike), the problems and the member
// names the value repeats, as ReadItems does.
func ReadItem(listing func(directory string) osfile.Spellings, path, key string,
	readerError func(error) string) (jsonvalue.Value, []string, jsonvalue.Repeats) {
	values, problems, repeats := ReadItems(listing, path, []string{key}, readerError)
	return values[key], problems, repeats
}
