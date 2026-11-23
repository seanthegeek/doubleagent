// Package parserkit is the kit every agent parser is written with: the Parser
// interface and the optional interfaces only some parsers implement, the
// options a timeline run passes, and the helpers parsers share for reading
// evidence (files, lines, JSON lines, JSON arrays, JSONC, SQLite databases)
// and for writing rows (the base row, error rows, the text of a message's
// content, settings and tracked changes).
//
// A parser owns one agent name from the catalog, says which of that agent's
// artifacts it reads (Wants), and emits timeline rows from each (Parse). The
// parsers themselves live in one package each under internal/parsers, and
// the registry that lists them in output order is internal/parsers itself.
//
// Every reader here opens only a regular file and never follows a symbolic
// link: it looks at the path's own entry first, so a link, a directory, a
// named pipe or a device is reported as unreadable and never opened. The
// directories above that entry were looked at when the path was built: a
// parser package builds no disk path itself (it calls no filepath.Join,
// Dir or Clean) and takes every path it opens from the artifact, from
// Beside, Above, Under or Home.PathOf, which refuse a link on the way down
// and a name that climbs, or from the Child of a Directory, a directory
// DirectoryAt found not to be a link, which refuses a name that is not one
// component. A problem met while reading is collected in a Problems list,
// which a parser turns into `parser:` rows (ErrorRows) where its output
// puts them.
package parserkit
