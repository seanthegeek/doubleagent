// Package textlines splits text into lines and names the character sets
// the analysis commands trim, as the formats they read specify them.
//
// A reader of a configuration or log format ends a line at LF or CRLF
// only and trims only ASCII whitespace. Unicode-aware splitting would also
// end a line at U+2028, U+0085 or a form feed, and a Unicode trim would
// take U+00A0 and other Unicode spaces, so either would cut or shorten a
// value that holds one. A lone CR stays inside its line; Split also takes
// one CR that ends the text, as a line end whose LF is missing. The sets are
// meant for strings.Trim, strings.TrimLeft and strings.TrimRight, which
// read them as single bytes here since every character is ASCII.
//
// A bufio.Scanner with ScanLines ends lines by the same rule but stops at a
// line longer than its buffer (64 KiB unless told otherwise); Split takes
// text already in memory and has no such limit.
package textlines
