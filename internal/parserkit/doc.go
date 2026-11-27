// Package parserkit is the kit every agent parser is written with: the Parser
// interface and the optional interfaces only some parsers implement, the
// options a timeline run passes, and the helpers parsers share for reading
// evidence (files, lines, JSON lines, JSON arrays, JSONC, SQLite databases)
// and for writing rows (the base row, error rows, the text of a message's
// content, settings and tracked changes).
//
// A parser owns one agent name from the catalog, says which of that agent's
// artifacts it reads (Wants), and emits timeline rows from each (Parse). The
// parsers live in one package each under internal/parsers, whose registry
// lists them in output order.
//
// No link is followed on the way to a file. The path builders (Beside,
// Above, Under, Home.PathOf and a Directory's Child) refuse a link on the
// way down and a name that climbs, and leave the last element to the
// reader (Open, ReadDir, DirectoryAt), which looks at that entry itself and
// opens only a regular file. A problem met while reading is
// collected in a Problems list, which a parser turns into `parser:` rows
// (ErrorRows) where its output puts them.
package parserkit
