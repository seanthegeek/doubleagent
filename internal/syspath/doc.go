// Package syspath converts between the one path form the binary uses inside
// and the forms the operating system takes and prints. The canonical form has
// forward slashes, the drive letter as spelled and no trailing slash
// (C:/Users/alice; the drive root C:; the UNC, or network share, root
// //server/share), never the \\?\ extended-length prefix. On Unix that is
// the native form and every conversion is the identity. On Windows FromNative
// takes what the OS hands the binary, ToNative gives the output form, Extended
// the form for the binary's own Win32 calls, and Fold the ASCII-lowered key
// paths are compared by. The Windows algorithms are plain functions here so
// every platform tests them; the platform files bind the exported names.
//
// path/filepath has Clean, ToSlash, FromSlash and VolumeName, but only for the
// OS's own form; it keeps no second form, and os adds the \\?\ prefix only to
// a path of 248 bytes or more. This package keeps the canonical form beside
// the native one, gives every absolute path the prefix and resolves . and ..
// by name before it (never above a drive or share), compares paths by an
// ASCII-only key and by file identity (Entries), and rewrites the paths in an
// error chain to native form.
package syspath
