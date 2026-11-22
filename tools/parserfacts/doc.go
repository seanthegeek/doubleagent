// Package parserfacts finds what each registered parser of the timeline
// command tells the reader of its research page: the record keys it reads,
// the constant start of each row text and problem message it writes, and
// the file patterns of its kinds and of its secondary sources.
//
// It reads the parser packages' source with go/types, so a string counts
// only where the type checker places it: an argument of a kit function or
// method whose parameter is a key, text or message (the lookups of
// internal/jsonvalue, internal/sqlitefile, internal/tomldoc,
// internal/protowire and internal/vscodestate, a row's Text field, a
// problem's Message), or of a parser's own function whose parameter
// reaches one. Comments and strings passed elsewhere never count.
//
// The code is test code, so the binary's import guard (internal/follow)
// does not apply to a tool that parses the module's own source and never
// reads evidence. tools/check_parser_research.py runs
// TestWriteTheRegisteredParsersFacts with PARSERFACTS_OUT naming the file
// for the JSON report, and compares the report with the research pages;
// without the variable the test is skipped.
package parserfacts
