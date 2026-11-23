// Package jsonvalue holds a decoded JSON document as an ordered value, read
// through encoding/json's token reader and written with its string quoting.
//
// The value keeps what the evidence wrote: an object's keys stay in document
// order (a repeated key keeps its first position and takes its last value,
// as JavaScript's JSON.parse reads it), and a number keeps its source text,
// so "1.50" stays "1.50" and an integer of any size stays exact. Strings are
// valid UTF-8: the token reader replaces each byte of invalid UTF-8, and
// each lone UTF-16 surrogate escape, with U+FFFD. NaN and Infinity are not
// JSON and do not decode.
//
// Decode reads a whole document, DecodePrefix one value and a Document the
// values of one text in text order. A Document also notes each member that
// repeats a name of its object (TakeRepeats), a count and the first few
// names, so a reader can report the value it replaced. Arrays and objects
// nest at most 10,000 deep: the decoder counts the levels itself and
// refuses a deeper document with an error ("exceeded max depth" at its
// opening bracket), and it builds the value without recursion, so reading
// never overflows the stack. Errors wrap the standard library's and give
// the byte offset, line and column where reading stopped.
//
// Text gives a value as row text (an array or object as compact JSON).
// IsEmpty is DoubleAgent's rule for a value that is left out of a row: null,
// false, zero, an empty string, array or object. Equal compares two values
// with numbers by value and objects without regard to key order.
//
// encoding/json can decode into any (map[string]any, float64): a map loses
// the evidence's key order, which rows and embedded JSON keep, and a float
// loses a number's text (1.50, a 20-digit id); UseNumber keeps the text but
// not the order. This package reads tokens to keep both.
package jsonvalue
