// Package growthtests holds the tests that run over every registered parser
// of the timeline command (parserregistry.All) rather than over one: that
// its output and work grow in step with its input (growth_test.go, with one
// growth_<parser>_test.go per parser building its cases), that it reports a
// JSON object's repeated member name once per file (repeats_test.go,
// planted_test.go), and that it cuts a long value it carries into the text
// of many rows (carried_test.go). Each file's comments say what its test
// builds and what it holds a parser to.
//
// The package holds only tests, since they belong to no one parser and
// none is code of the registry; this file gives the package its comment.
package growthtests
