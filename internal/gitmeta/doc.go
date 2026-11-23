// Package gitmeta reads the remotes and checked-out branch of a collected
// project's repository from the text of two of its metadata files:
//
//   - .git/config, git's INI-like configuration: a [remote "<name>"]
//     section (or the deprecated [remote.<name>], whose name git
//     lower-cases) per remote, with every url and pushurl value kept in
//     file order. Section and key names match without regard to case. A
//     value may be quoted, holds the escapes \\, \", \n, \t and \b, ends at
//     a # or ; outside quotes, loses only its unprotected surrounding
//     whitespace, and continues after a backslash that ends the line
//     outside a comment. A pair, or another header, may follow a header on
//     one line. White space is git's: space, tab, CR and LF. A line
//     that is neither a header nor a pair, and the keys after a malformed
//     header, are skipped; a value git rejects (an unterminated quote, an
//     unknown escape) is dropped and reported.
//   - .git/HEAD: "ref: refs/heads/<branch>" while a branch is checked out,
//     else the commit hash of a detached HEAD.
//
// Both readers take the file's text decoded as UTF-8 with invalid bytes
// replaced (textlines.ValidUTF8), split lines at LF or CRLF only, as git
// does, and drop a leading byte-order mark; HEAD's ref is trimmed of ASCII
// white space.
// Finding the files among a collection's artifacts is the caller's job.
//
// Running git, or a Git library, would open a repository found in
// evidence and could run its hooks or configured programs; this package
// only reads the two text files and never runs anything.
package gitmeta
