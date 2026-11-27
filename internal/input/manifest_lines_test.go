package input

import (
	"bytes"
	"strings"
	"testing"
)

func TestAManifestLineTooLongToBeARowIsOneProblem(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir+"/fs/home/a/.claude/x", "x")
	row := collectedRow("a", "/home/a", "claude-code", "/home/a/.claude/x", "fs/home/a/.claude/x")
	writeManifest(t, dir, string(bytes.Repeat([]byte(" "), maxManifestLine+1)), row)
	collection, err := Open(t.Context(), dir, Options{Matcher: matcherOf(t, "")})
	opened(t, err)
	wantSlice(t, "problems", collection.Problems, []string{"manifest.jsonl line 1: longer than 1048576 bytes, not read"})
	wantEqual(t, "artifacts", len(collection.Artifacts), 1)
}

func TestACollectionSummaryLargerThanItsBoundIsANote(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir)
	writeFile(t, dir+"/collection.json", `{"hostname": "`+strings.Repeat("h", maxSummary)+`"}`)
	collection, err := Open(t.Context(), dir, Options{Matcher: matcherOf(t, "")})
	opened(t, err)
	wantSlice(t, "notes", collection.Notes, []string{"collection.json unreadable: larger than doubleagent reads"})
}

func TestAManifestThatIsADirectoryLeavesANote(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir+"/fs/home/a/.claude/x", "x")
	writeFile(t, dir+"/manifest.jsonl/inside", "x")
	collection, err := Open(t.Context(), dir, Options{Matcher: matcherOf(t, "")})
	opened(t, err)
	wantEqual(t, "kind", collection.Kind, KindLoose)
	wantSlice(t, "notes", collection.Notes,
		[]string{"manifest.jsonl is not a regular file; reading the input as a loose tree"})
}
