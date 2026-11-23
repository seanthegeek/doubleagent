// Package tomldoc reads a TOML 1.1 document into values that keep the
// document's order: a parser walks tables and keys in the order the file
// wrote them.
//
// The reading itself is done by github.com/BurntSushi/toml, which returns
// tables as unordered maps; every value, and every array's length, is the
// module's own. The order comes from the list of keys that module reports
// in document order: walking it rebuilds each table, and a table keeps the
// walk's order when the walk holds exactly its keys, and otherwise has its
// keys in byte order, as does every table under it. The walk holds a
// table's exact keys for the tables of headers, arrays of tables and
// dotted keys, for inline tables outside arrays, and for the inline tables
// of an array that is a key's own value when nothing in the array holds a
// table except those inline tables, whose values hold none. Other inline
// tables in arrays are in byte order, unless the module returns the array
// as an array of tables (one of its inline tables holds the empty key):
// the walk then keeps a table's document order wherever it holds exactly
// that table's keys.
//
// Parse refuses a document over a size limit, or whose keys and arrays nest
// deeper than a depth limit (maxDocumentBytes and maxDepth), before reading
// it, with an *Error that says which.
//
// The module's Decode into a map gives the values but not their order;
// rows list settings in the file's order, so this package rebuilds it from
// the key list Decode also returns.
package tomldoc
