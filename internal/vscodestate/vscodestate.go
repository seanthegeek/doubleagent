// Package vscodestate reads extension global state out of a VS Code family
// state.vscdb.
//
// <editor User dir>/globalStorage/state.vscdb is SQLite with one table,
// ItemTable(key TEXT, value BLOB). An extension's whole globalState is one
// row whose key is the extension id (sourcegraph.cody-ai, rjmacarthy.twinny)
// and whose value is UTF-8 JSON text; see microsoft/vscode at d7622a529314,
// src/vs/platform/extensionManagement/common/extensionStorage.ts:159-176.
// SecretStorage rows have keys starting secret:// and hold encrypted
// credentials; they are never read here, whatever key a caller asks for.
//
// The database is read in place through sqlitefile, its -wal and -journal
// sidecars applied; the -shm file is never read, so it never stands in the
// way. A file that cannot be read, or whose sidecar cannot, a file that
// is not SQLite, a damaged SQLite file and a value that is not JSON are each
// reported in the returned problem list; the rows are read in key order, so
// the problems come in key order too.
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

// SecretPrefix starts the keys of SecretStorage rows, which are never read.
const SecretPrefix = "secret://"

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
// start of rel or after a slash.
func IsStateDatabase(rel string) bool {
	return rel == stateDatabase || strings.HasSuffix(rel, "/"+stateDatabase)
}

// ReadItems returns the decoded JSON values of the requested ItemTable keys
// that are present, and the problems met. A value that is not JSON is
// returned as its text, a JSON string, and noted in the problems, and so is
// a database sqlitefile.Problem rejects, under its own file name. An absent
// file, a symbolic link and a file that is not regular give nothing.
// secret:// keys are never read.
func ReadItems(path string, keys []string) (map[string]jsonvalue.Value, []string) {
	values := map[string]jsonvalue.Value{}
	var wanted []sqlitefile.Value
	for _, key := range keys {
		if !strings.HasPrefix(key, SecretPrefix) {
			wanted = append(wanted, sqlitefile.TextValue(key))
		}
	}
	if len(wanted) == 0 || !regularFile(path) {
		return values, nil
	}
	if err := sqlitefile.Problem(path); err != nil {
		if errors.Is(err, sqlitefile.ErrNotDatabase) {
			return values, []string{filepath.Base(path) + " is not a SQLite database"}
		}
		return values, []string{err.Error()}
	}
	rows, err := itemRows(path, wanted)
	if err != nil {
		return values, []string{"state.vscdb unreadable: " + err.Error()}
	}
	var problems []string
	for _, row := range rows {
		key := row.Get("key")
		if key.Type != sqlitefile.Text || strings.HasPrefix(key.Text(), SecretPrefix) {
			continue
		}
		text := valueText(row.Get("value"))
		decoded, err := jsonvalue.DecodeString(text)
		if err != nil {
			decoded = jsonvalue.NewString(text)
			problems = append(problems, ValueProblem+key.Text()+" is not valid JSON ("+err.Error()+")")
		}
		values[key.Text()] = decoded
	}
	return values, problems
}

// regularFile reports whether path's own entry is a regular file.
func regularFile(path string) bool {
	info, err := osfile.Lstat(path)
	return err == nil && info.Mode()&fs.ModeType == 0
}

// itemRows returns the ItemTable rows whose key is one of wanted, in key
// order, or none when the database has no ItemTable.
func itemRows(path string, wanted []sqlitefile.Value) ([]sqlitefile.Row, error) {
	database, err := sqlitefile.OpenPath(path)
	if err != nil {
		return nil, err
	}
	defer database.Close()
	tables, err := database.TableNames()
	if err != nil || !tables["ItemTable"] {
		return nil, err
	}
	table, err := database.Table("ItemTable")
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

// valueText returns a value as the text its JSON is read from: a blob
// decoded as UTF-8 with each invalid byte replaced, and any other value as
// its row text (sqlitefile.Value.String).
func valueText(value sqlitefile.Value) string {
	switch value.Type {
	case sqlitefile.Blob:
		return textlines.ValidUTF8(value.Bytes)
	}
	return value.String()
}

// ReadItem returns the decoded JSON value of one ItemTable key, JSON null
// when it is absent (the two read alike), and the problems.
func ReadItem(path, key string) (jsonvalue.Value, []string) {
	values, problems := ReadItems(path, []string{key})
	return values[key], problems
}

// ShownProblem returns the problem a row shows: a value problem without the
// JSON decoder's detail, which may quote the value, and any other as it is.
func ShownProblem(problem string) string {
	if strings.HasPrefix(problem, ValueProblem) {
		shown, _, _ := strings.Cut(problem, " (")
		return shown
	}
	return problem
}
