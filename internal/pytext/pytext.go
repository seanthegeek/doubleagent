// Package pytext turns values into text exactly as the Python analyzer's
// interpreter does, so that rows built in Go equal the rows Python built:
// floats as repr(float) writes them, True, False and None as %s writes
// them, bytes decoded as UTF-8 with Python's replacement rule, the trims of
// str.strip() and bytes.strip(), and slicing and padding counted in code
// points.
//
// A Python str is held in a Go string. Valid UTF-8 in it encodes code
// points, and a lone surrogate (U+D800 to U+DFFF, which a JSON \ud83d
// escape can produce and which UTF-8 cannot encode) is held as the
// three-byte form UTF-8 would give it (ED A0 80 for U+D800), the encoding
// known as WTF-8. Strings from DecodeUTF8 never hold that form, since
// Python's decoder replaces those bytes; only a decoded JSON escape puts
// one there. Every function of this package that walks code points reads
// both forms, and counts any other invalid byte as one code point.
package pytext
