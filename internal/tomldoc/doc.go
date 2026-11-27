// Package tomldoc reads a TOML 1.1 document into values that keep the
// document's order: a parser walks tables and keys in the order the file
// wrote them.
//
// The reading itself is done by github.com/BurntSushi/toml, whose Decode
// into a map gives the values but not their order. This package rebuilds
// the order from the key list Decode also returns: a table keeps the
// document's order where that list holds exactly its keys, and otherwise
// has its keys in byte order (docs/internals.md says which tables keep it).
//
// Parse refuses a document over a size limit, or whose keys and arrays nest
// deeper than a depth limit (maxDocumentBytes and maxDepth), before reading
// it, with an *Error that says which.
package tomldoc
