//go:build !(noasm && nounsafe)

package zstdframe

// The zstd module reads compressed evidence with assembly and unchecked
// memory reads unless it is built with the tags noasm and nounsafe. This
// file is compiled only when one of them is missing, and its one line cannot
// compile: the compiler's "undefined" error names the tags to build with.
// Every go command of the project sets them (see "Go standards" in internal/AGENTS.md).
var _ = buildWithTagsNoasmAndNounsafeOnly
