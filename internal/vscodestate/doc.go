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
