// Package protowire decodes protocol buffer messages from hand-written
// schemas. A parser names only the fields it reads, as a table of field
// number, name and kind per message, and needs no generated code. Unknown
// fields, groups and fields of another wire type than their kind are
// skipped. A field that cannot be read in its declared kind, a fixed-width
// field cut short and a packed field holding a damaged varint are left out
// of the result, and a malformed nested message is left out whole, so one
// damaged field costs only itself; only a top-level buffer that cannot be
// followed, or a message of more than 1,048,576 values (every scalar,
// packed value and nested message, repeated or not, at any depth), is an
// error.
//
// google.golang.org/protobuf can decode from a schema built in code, but
// it refuses a whole message for one damaged field, such as a malformed
// nested message, where this package keeps the rest; and it would be one
// more dependency for one parser.
package protowire
