// Package sqlitetest writes SQLite database files for tests, in the file
// format's own layout (https://www.sqlite.org/fileformat2.html): page 1
// holds the header and the schema table, each table is a b-tree of leaf
// pages under as many levels of interior pages as its rows need, and a row
// too large for its page spills onto a chain of overflow pages. It writes
// tables only (no index, view or trigger), with 4096-byte pages, UTF-8
// text, and the rowid of each row its position from 1. A test builds a
// database of any size this way without a SQLite library. A varint past
// 56 bits, which the format writes in nine bytes and no rowid or size here
// reaches, is refused with a panic. Only tests import it.
package sqlitetest
