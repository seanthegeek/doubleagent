// Package sqlitefile reads SQLite database files without SQL and without
// writing anything: the header, table and index b-trees (the page trees
// that hold rows and index keys), overflow pages and records, as the file
// format document (https://www.sqlite.org/fileformat2.html) defines them,
// with what SQLite sees in a copy's sidecars (the -shm file it rebuilds is
// ignored). A rollback journal is played back when it has the journal
// magic, at least 512 bytes, sector and page sizes that are powers of two
// in range and no super-journal, until a short read, a page number of 0 or
// of the lock page, or a checksum mismatch. A write-ahead log is used when
// its header has a known magic, a valid page size and a correct checksum;
// frames are read while their salts and running checksum hold, and the
// pages up to the last commit frame, with its database size, win.
//
// OpenPath, Open and OpenBytes open a database; Schema lists the schema
// table, TableRows and IndexRows walk one b-tree in key order, and Table
// and Select read a table's rows as SQL's SELECT returns them. Damage
// gives an error wrapping ErrMalformed, never a panic.
//
// database/sql would need a SQLite driver. The pure-Go ones are SQLite's C
// source translated to Go, keeping C's memory model through the unsafe
// package, so Go's bounds checks do not protect them and they cannot build
// with the project's nounsafe tag; a cgo one would end the static,
// cross-compiled build; and either runs the database's own schema text
// through a SQL engine, where an evidence file is hostile input. This
// package reads pages and records with bounds-checked Go, executes no SQL
// (CREATE TABLE text is read only for the columns and their defaults) and
// never writes, so a database is read in place with its -wal and journal.
package sqlitefile
