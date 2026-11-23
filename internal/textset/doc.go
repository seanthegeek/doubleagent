// Package textset finds a fixed list of texts in other texts in one pass
// over each, whatever the number of listed texts. strings.Contains or
// strings.Index searched once per listed text costs the length of the text
// once per listed text; strings.Replacer replaces, but tells neither which
// texts occur nor the longest one that starts at each position, which a
// caller needs to choose among texts that overlap.
//
// A Set is an Aho-Corasick automaton: a tree of the texts' prefixes where
// each node also links to the longest proper suffix of its prefix that is a
// node too, so a byte that leaves the tree falls back along those links
// instead of starting over. A Set built backward holds each text reversed
// and reads a searched text from its end, so the texts it reports at a
// position are those that start there.
package textset
