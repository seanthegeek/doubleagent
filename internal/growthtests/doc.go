// Package growthtests holds the tests of the registered parsers
// (parserregistry.All) that belong to no one parser's package: that the
// output and work of every parser grow in step with its input
// (growth_test.go, with each parser's cases in its
// growth_<package>_test.go), that a JSON object's repeated member name is
// reported once, by the parse of the file that holds it, and that the
// parsers that write one carried value into the text of many rows cut it
// in each. Each file's comments say what its test builds and what it holds
// a parser to.
//
// The package holds only tests, since none is code of the registry; this
// file gives the package its comment.
package growthtests
