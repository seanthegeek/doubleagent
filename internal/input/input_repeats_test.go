package input

import "testing"

func TestManifestRowsThatRepeatAMemberNameGiveOneProblem(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir+"/fs/home/a/.claude/x", "x")
	writeFile(t, dir+"/fs/home/a/.claude/y", "y")
	first := collectedRow("a", "/home/a", "claude-code", "/home/a/.claude/x", "fs/home/a/.claude/x")
	second := collectedRow("a", "/home/a", "claude-code", "/home/a/.claude/y", "fs/home/a/.claude/y")
	writeManifest(t, dir, `{"note":1,"note":2,`+first[1:], `{"note":1,"note":2,`+second[1:])
	writeFile(t, dir+"/collection.json", `{"hostname":"a","hostname":"b"}`)
	collection, err := Open(t.Context(), dir, Options{Matcher: matcherOf(t, "")})
	opened(t, err)
	wantSlice(t, "problems", collection.Problems, []string{
		`collection.json: 1 member name repeats within its JSON object ("hostname"); the last value is used`,
		`manifest.jsonl: 2 member names repeat within their JSON objects ("note" 2 times); ` +
			`the last value of each is used`,
	})
	wantEqual(t, "artifacts", len(collection.Artifacts), 2)
	wantEqual(t, "host", collection.Summary.Get("hostname").Text(), "b")
}

func TestACollectionWithoutARepeatGivesNoProblem(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir)
	writeFile(t, dir+"/collection.json", `{"hostname":"a","projects":[{"hostname":"b"}]}`)
	collection, err := Open(t.Context(), dir, Options{Matcher: matcherOf(t, "")})
	opened(t, err)
	wantSlice(t, "problems", collection.Problems, nil)
}
