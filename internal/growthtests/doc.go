// Package growthtests holds tests of the registered parsers
// (parserregistry.All) that are the code of neither the registry nor one
// parser: the growth test, which holds the output and work of every parser
// to growing in step with its input (growth_test.go, with each parser's
// cases in its growth_<package>_test.go), and the tests of repeated member
// names and of carried values that share its helpers.
//
// The package holds only tests; this file gives the package its comment.
package growthtests
