// Package growthtests holds the tests that run over every registered parser
// of the analysis commands (parserregistry.All) rather than over one: that
// its output and work grow in step with its input (growth_test.go, with the
// cases of each parser in its growth_<package>_test.go), that it reports a
// JSON object's repeated member name once, on the file that holds it, and
// that it cuts a long value it carries into the text of many rows. Each
// file's comments say what its test builds and what it holds a parser to.
//
// The package holds only tests, since they belong to no one parser and
// none is code of the registry; this file gives the package its comment.
package growthtests
