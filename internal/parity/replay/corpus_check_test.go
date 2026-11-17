//go:build parity

package replay

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// caseFiles returns the case files of the corpus by group, the name each
// loader takes: agents for parsers, commands for commands.
func caseFiles(t *testing.T, corpus *Corpus, group string) []string {
	t.Helper()
	entries, err := osfile.ReadDir(filepath.Join(corpus.Directory, group))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		if name, ok := strings.CutSuffix(entry.Name(), ".jsonl"); ok {
			names = append(names, name)
		}
	}
	return names
}

func TestEveryCaseOfTheCorpusDecodes(t *testing.T) {
	corpus := Open(t)
	total := 0
	for _, agent := range caseFiles(t, corpus, "parsers") {
		total += len(corpus.ParserCases(t, agent))
	}
	for _, command := range caseFiles(t, corpus, "commands") {
		total += len(corpus.CommandCases(t, command))
	}
	total += len(corpus.InputCases(t))
	if total == 0 {
		t.Error("the corpus holds no case")
	}
	t.Logf("%d cases decoded", total)
}

func TestTheFirstTreeOfEachCaseFileIsMaterialisedByteForByte(t *testing.T) {
	corpus := Open(t)
	trees := map[string][2]string{}
	for _, agent := range caseFiles(t, corpus, "parsers") {
		for _, parserCase := range corpus.ParserCases(t, agent) {
			if parserCase.Tree != "" && parserCase.Method == "parse" {
				trees["parsers/"+agent] = [2]string{parserCase.Tree, parserCase.Times}
				break
			}
		}
	}
	for _, command := range caseFiles(t, corpus, "commands") {
		for _, commandCase := range corpus.CommandCases(t, command) {
			if commandCase.Tree != "" {
				trees["commands/"+command] = [2]string{commandCase.Tree, commandCase.Times}
				break
			}
		}
	}
	for name, snapshot := range trees {
		t.Run(name, func(t *testing.T) {
			root := corpus.Materialise(t, snapshot[0], snapshot[1])
			checkTree(t, corpus, root, root, snapshot[0])
		})
	}
}

// checkTree compares every readable file under directory with its blob,
// hashing the file's bytes with root put back as the placeholder.
func checkTree(t *testing.T, corpus *Corpus, root, directory, tree string) {
	t.Helper()
	entries, err := corpus.ReadTree(tree)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		path := filepath.Join(directory, string(entry.Name))
		switch {
		case entry.Kind == "dir" && entry.Mode&0o500 == 0o500:
			checkTree(t, corpus, root, path, entry.Tree)
		case entry.Kind == "file" && entry.Blob != "" && entry.Mode&0o400 != 0:
			data, err := osfile.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			data = bytes.ReplaceAll(data, []byte(root), []byte(corpus.Header.RootPlaceholder))
			sum := sha256.Sum256(data)
			if hex.EncodeToString(sum[:]) != entry.Blob {
				t.Errorf("%s does not hash to its blob %s", path, entry.Blob)
			}
		}
	}
}
