// Package protowire decodes protocol buffer messages from hand-written
// schemas. A parser names only the fields it reads, as a table of field
// number, name and kind per message, and needs no generated code. Unknown
// fields, and fields of another wire type than their kind, are skipped. A
// field that cannot be read in its declared kind, a fixed-width field cut
// short and a packed field holding a damaged varint are left out of the
// result, and a malformed nested message is left out whole, so one damaged
// field costs only itself; only a top-level buffer that cannot be
// followed, or a message of more than 1,048,576 values (every scalar,
// packed value and nested message, repeated or not, at any depth), is an
// error.
//
// google.golang.org/protobuf decodes through code generated from .proto
// files; the agents' schemas are unpublished and change, so a parser names
// the few fields it reads in a table instead, and a damaged or unknown
// field never fails the rest of the message.
package protowire
