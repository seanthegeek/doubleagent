// Package catalogtest rewrites a catalog text for tests: the same table lines
// under the same section headers, with the tables and the lines inside each
// in another order, and with the comments and blank lines left out. Tests use
// it to show that the order of a catalog file's lines changes nothing the
// program does. No production code imports it.
package catalogtest
