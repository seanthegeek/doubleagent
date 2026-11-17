// Package sqlitefile reads SQLite database files without SQL and without
// writing anything: the header, table and index b-trees (the page trees
// that hold rows and index keys), overflow pages and records, as the file
// format document (https://www.sqlite.org/fileformat2.html) defines them.
// It sees what SQLite sees when it opens a copy of a database and its
// sidecars; the -shm file is ignored, since SQLite rebuilds it.
//
// A rollback journal (name-journal) is played back, its saved pages read in
// place of the main file's, when the main file is not empty, the journal's
// first byte is not zero, the journal is at least 512 bytes long and names
// no super-journal (SQLite plays such a journal only when the named file
// exists, which a copy never has), and its first header has the journal
// magic and a sector size and page size (0 meaning the database's) that are
// powers of two in range. The main file is then cut or grown to the size
// that header records; records are restored segment by segment, pages past
// that size skipped, until a short read, a page number of 0 or of the lock
// page, or a checksum mismatch.
//
// A write-ahead log (name-wal) is used when the main file, after any
// rollback, is not empty and the log's header has a known magic, a page
// size that is a power of two from 512 to 65536 and a correct checksum; a
// valid header of another format version is an error, as it is to SQLite.
// Frames are read in order while their salts match the header's, their page
// number is not 0 and the running checksum holds; the pages of the frames up
// to the last commit frame among them, and its database size, win.
//
// OpenPath, Open and OpenBytes open a database; Schema lists the schema
// table, TableRows and IndexRows walk one b-tree in key order, and Table
// and Select read a table's rows as SQL's SELECT returns them. Damage
// gives an error wrapping ErrMalformed, never a panic.
package sqlitefile
