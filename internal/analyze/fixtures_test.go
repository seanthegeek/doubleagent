package analyze

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/pyjson"
)

// fakeParser stands for an agent's parser in a test: it wants nothing.
type fakeParser struct {
	parser.Base
}

// Wants reports that the fake reads no artifact.
func (fakeParser) Wants(*input.Artifact) bool { return false }

// Parse emits nothing.
func (fakeParser) Parse(*input.Artifact, parser.Options, parser.Emit) error { return nil }

// fakeParsers returns constructors of fake parsers for the agents.
func fakeParsers(agents ...string) []func() parser.Parser {
	makers := make([]func() parser.Parser, len(agents))
	for i, agent := range agents {
		makers[i] = func() parser.Parser { return fakeParser{parser.NewBase(agent, "fake")} }
	}
	return makers
}

// collectedFile is one file of a test collection: who it belongs to, its
// home or project, its agent, its path under that base and its text.
type collectedFile struct {
	user, home, agent, rel, text string
}

// writeCollection writes an extracted collection under directory: a
// collection.json with the summary members, each file under fs/ and its
// collected manifest row, as the analyzer's tests write one.
func writeCollection(t *testing.T, directory string, summary []pyjson.Member,
	files []collectedFile) string {
	t.Helper()
	var manifest strings.Builder
	for _, file := range files {
		path := strings.TrimRight(file.home, "/") + "/" + file.rel
		writeFile(t, filepath.Join(directory, "fs"+filepath.FromSlash(path)), file.text)
		row := pyjson.NewObject(member("user", pyjson.NewString(file.user)),
			member("home", pyjson.NewString(file.home)), member("agent", pyjson.NewString(file.agent)),
			member("path", pyjson.NewString(path)), member("archive_path", pyjson.NewString("fs"+path)),
			member("type", pyjson.NewString("file")), member("status", pyjson.NewString("collected")),
			member("size", pyjson.NewInt(int64(len(file.text)))))
		manifest.WriteString(pyjson.Spaced(row) + "\n")
	}
	writeFile(t, filepath.Join(directory, "manifest.jsonl"), manifest.String())
	writeFile(t, filepath.Join(directory, "collection.json"), pyjson.Spaced(pyjson.NewObject(summary...)))
	return directory
}

// writeFile writes text to path, creating its directories.
func writeFile(t *testing.T, path, text string) {
	t.Helper()
	if err := osfile.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// showRoot returns output with the test's directory work, followed by a
// separator, written as "{ROOT}/" as the expected texts write it, in its raw
// and its JSON-escaped form.
func showRoot(output, work string) string {
	prefix := work + string(filepath.Separator)
	escaped := pyjson.Compact(pyjson.NewString(prefix))
	output = strings.ReplaceAll(output, escaped[1:len(escaped)-1], "{ROOT}/")
	return strings.ReplaceAll(output, prefix, "{ROOT}/")
}
